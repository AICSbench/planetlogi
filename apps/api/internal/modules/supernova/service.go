package supernova

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"sync"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AICSbench/planetlogi/apps/api/internal/mathengine"
	"github.com/AICSbench/planetlogi/apps/api/internal/modules/auth"
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/economy"
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/reqguard"
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/state"
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/timewindow"
)

const (
	SuperSightDiffCount  = 5
	SuperSightMaxWrong   = 5
	SuperSightPointsEach = 10
	SuperSightTimeLimitS = 300
	WarpDurationS        = 120
	// WarpFinishGraceS lets slow network submissions land just past the
	// server-side deadline before the session expires.
	WarpFinishGraceS   = 3
	HubHistoryPageSize = 20
	HistoryCap         = 100
)

var (
	ErrSuperNovaClosed = errors.New("super nova challenge is open Mon-Sat 06:00-22:00 local time")
	ErrSessionEnded    = errors.New("super sight session already ended")
	// ErrRewardPending signals the ledger could not be reached, so the score
	// was not recorded; the client can safely retry.
	ErrRewardPending = errors.New("super nova score could not be recorded; try again")
)

// supernovaProgressDoc is the durable weekly score + score history document
// per student, so diamond S totals survive restarts.
type supernovaProgressDoc struct {
	Weekly  map[string]int      `json:"weekly"`
	History []ScoreHistoryEntry `json:"history"`
}

const supernovaProgressModule = "supernova_progress_v1"

type DiffTarget struct {
	ID     int     `json:"id"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Radius float64 `json:"radius"`
	Found  bool    `json:"found"`
}

type SuperSightSession struct {
	SessionID   string       `json:"sessionId"`
	PuzzleID    string       `json:"puzzleId"`
	Title       string       `json:"title"`
	FoundCount  int          `json:"foundCount"`
	WrongCount  int          `json:"wrongCount"`
	MaxWrong    int          `json:"maxWrong"`
	Score       int          `json:"score"`
	Completed   bool         `json:"completed"`
	FoundPoints []DiffTarget `json:"foundPoints"`
	TimeLimitS  int          `json:"timeLimitSec"`
}

type ScoreHistoryEntry struct {
	ID         string    `json:"id"`
	Mode       string    `json:"mode"` // SUPER_SIGHT | SUPER_WARP
	IsPractice bool      `json:"isPractice"`
	Score      int       `json:"score"`
	RecordedAt time.Time `json:"recordedAt"`
}

type HubSummary struct {
	IsOpen             bool                `json:"isOpen"`
	WindowNotice       string              `json:"windowNotice,omitempty"`
	WeeklyDiamondScore int                 `json:"weeklyDiamondScore"`
	History            []ScoreHistoryEntry `json:"history"`
	HistoryPage        int                 `json:"historyPage"`
	HistoryPages       int                 `json:"historyPages"`
	PageSize           int                 `json:"historyPageSize"`
}

type WarpQuestion struct {
	Index    int                         `json:"index"`
	Question mathengine.RenderedQuestion `json:"question"`
}

type SuperWarpStart struct {
	SessionID  string         `json:"sessionId"`
	IsPractice bool           `json:"isPractice"`
	DurationS  int            `json:"durationSec"`
	AutoSkipS  int            `json:"autoSkipSec"`
	Questions  []WarpQuestion `json:"questions"`
}

type sightState struct {
	id         string
	studentID  string
	puzzleID   string
	title      mathengine.LocaleText
	targets    []DiffTarget
	found      int
	wrong      int
	score      int
	completed  bool
	isPractice bool
	deadline   time.Time
}

type warpState struct {
	id         string
	studentID  string
	isPractice bool
	templates  []string
	seeds      []int64
	score      int
	finished   bool
	deadline   time.Time
}

type Service struct {
	mu             sync.Mutex
	ledger         economy.Ledger
	pool           *pgxpool.Pool
	store          *state.PGStore
	weeklyScores   map[string]int // studentID:weekKey -> diamond S score
	history        map[string][]ScoreHistoryEntry
	progressLoaded map[string]bool
	sightSess      map[string]*sightState
	warpSess       map[string]*warpState
	seq            int
	nowClock       func() time.Time
	enforceOpen    bool
}

func NewService(ledger economy.Ledger) *Service {
	return &Service{
		ledger:         ledger,
		weeklyScores:   make(map[string]int),
		history:        make(map[string][]ScoreHistoryEntry),
		progressLoaded: make(map[string]bool),
		sightSess:      make(map[string]*sightState),
		warpSess:       make(map[string]*warpState),
		seq:            100,
		nowClock:       time.Now,
		// Ranked windows are enforced by default in production; tests and
		// demos can relax it via config (SUPERNOVA_ENFORCE_OPEN) or the
		// SetEnforceWindow setter. Per-question auto-skip stays client-side.
		enforceOpen: true,
	}
}

// NewServiceWithPG wires the durable phase2_state store, so weekly diamond S
// scores and history survive restarts.
func NewServiceWithPG(pool *pgxpool.Pool, ledger economy.Ledger) *Service {
	s := NewService(ledger)
	s.pool = pool
	s.store = state.NewPGStore(pool)
	return s
}

// SetEnforceWindow toggles whether the ranked schedule window is enforced
// server-side (true in production; false only for local demos/tests).
func (s *Service) SetEnforceWindow(enforce bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.enforceOpen = enforce
}

func (s *Service) SetClockForTest(fn func() time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nowClock = fn
}

// newSessionID returns a random session ID so reward ledger keys are unique
// across process restarts (CR-10).
func newSessionID(prefix string) string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	}
	return prefix + "-" + hex.EncodeToString(b[:])
}

// randSeed returns a random positive question seed.
func randSeed() int64 {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return time.Now().UnixNano()
	}
	v := int64(binary.BigEndian.Uint64(b[:]))
	if v < 0 {
		v = -v
	}
	if v == 0 {
		return 1
	}
	return v
}

func (s *Service) weekKey() string {
	yr, wk := s.nowClock().UTC().ISOWeek()
	return fmt.Sprintf("%04d-W%02d", yr, wk)
}

func (s *Service) GetHub(ctx context.Context, studentID, tz string, page int) HubSummary {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureProgressLocked(ctx, studentID)
	eval := timewindow.Evaluate(s.nowClock(), tz, timewindow.SuperNovaWindow)
	hist := s.history[studentID]
	total := len(hist)
	pages := (total + HubHistoryPageSize - 1) / HubHistoryPageSize
	if page < 1 {
		page = 1
	}
	if pages > 0 && page > pages {
		page = pages
	}
	start := (page - 1) * HubHistoryPageSize
	if start > total {
		start = total
	}
	end := start + HubHistoryPageSize
	if end > total {
		end = total
	}
	if hist == nil {
		hist = []ScoreHistoryEntry{}
	}
	return HubSummary{
		IsOpen:             !s.enforceOpen || eval.IsOpen,
		WindowNotice:       eval.Reason,
		WeeklyDiamondScore: s.weeklyScores[studentID+":"+s.weekKey()],
		History:            hist[start:end],
		HistoryPage:        page,
		HistoryPages:       pages,
		PageSize:           HubHistoryPageSize,
	}
}

// ensureProgressLocked lazily loads the student's durable score document on
// first access after a restart (PG mode).
func (s *Service) ensureProgressLocked(ctx context.Context, studentID string) {
	if s.store == nil || s.progressLoaded[studentID] {
		return
	}
	s.progressLoaded[studentID] = true
	var doc supernovaProgressDoc
	found, err := s.store.Load(ctx, studentID, supernovaProgressModule, &doc)
	if err != nil || !found {
		return
	}
	week := s.weekKey()
	if doc.Weekly != nil {
		s.weeklyScores[studentID+":"+week] = doc.Weekly[week]
	}
	s.history[studentID] = doc.History
}

func (s *Service) StartSuperSight(studentID, tz, locale string, isPractice bool) (SuperSightSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	eval := timewindow.Evaluate(s.nowClock(), tz, timewindow.SuperNovaWindow)
	if !isPractice && s.enforceOpen && !eval.IsOpen {
		return SuperSightSession{}, ErrSuperNovaClosed
	}
	s.seq++
	id := newSessionID("sight")
	st := &sightState{
		id:         id,
		studentID:  studentID,
		puzzleID:   "ss-puzzle-01",
		title:      mathengine.LocaleText{EN: "Cosmic Observatory #01", ID: "Observatorium Kosmik #01"},
		isPractice: isPractice,
		deadline:   s.nowClock().Add(SuperSightTimeLimitS * time.Second),
		targets: []DiffTarget{
			{ID: 1, X: 0.18, Y: 0.25, Radius: 0.08},
			{ID: 2, X: 0.50, Y: 0.22, Radius: 0.08},
			{ID: 3, X: 0.82, Y: 0.28, Radius: 0.08},
			{ID: 4, X: 0.30, Y: 0.72, Radius: 0.08},
			{ID: 5, X: 0.74, Y: 0.75, Radius: 0.08},
		},
	}
	s.sightSess[id] = st
	return renderSight(st, locale), nil
}

func (s *Service) TapSuperSight(ctx context.Context, studentID, sessionID string, x, y float64, locale string) (bool, SuperSightSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.sightSess[sessionID]
	if !ok || st.studentID != studentID {
		return false, SuperSightSession{}, fmt.Errorf("super sight session not found")
	}
	if st.completed {
		return false, renderSight(st, locale), ErrSessionEnded
	}

	// Server-side 5-minute deadline: a tap after the deadline completes the
	// session with whatever score was reached, and no further taps count.
	if now := s.nowClock(); now.After(st.deadline) {
		if err := s.recordScore(ctx, studentID, "SUPER_SIGHT", st.isPractice, st.score, st.id); err != nil {
			// The score was not recorded; leave the session open so a retry
			// (or the next tap) can record it.
			return false, renderSight(st, locale), err
		}
		st.completed = true
		return false, renderSight(st, locale), ErrSessionEnded
	}

	hit := false
	for i := range st.targets {
		if st.targets[i].Found {
			continue
		}
		dist := math.Hypot(st.targets[i].X-x, st.targets[i].Y-y)
		if dist <= st.targets[i].Radius {
			st.targets[i].Found = true
			st.found++
			st.score += SuperSightPointsEach
			hit = true
			break
		}
	}
	if !hit {
		st.wrong++
	}

	if st.found >= SuperSightDiffCount || st.wrong >= SuperSightMaxWrong {
		if err := s.recordScore(ctx, studentID, "SUPER_SIGHT", st.isPractice, st.score, st.id); err != nil {
			// The score was not recorded; leave the session open so a retry
			// can still record it.
			return false, renderSight(st, locale), err
		}
		st.completed = true
	}

	return hit, renderSight(st, locale), nil
}

func (s *Service) StartSuperWarp(studentID, tz, locale string, grade int, isPractice bool) (SuperWarpStart, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	eval := timewindow.Evaluate(s.nowClock(), tz, timewindow.SuperNovaWindow)
	if !isPractice && s.enforceOpen && !eval.IsOpen {
		return SuperWarpStart{}, ErrSuperNovaClosed
	}
	s.seq++
	id := newSessionID("warp")
	tmpls := mathengine.TemplatesForGrade(grade)
	qs := make([]WarpQuestion, 15)
	codes := make([]string, 15)
	seeds := make([]int64, 15)
	for i := 0; i < 15; i++ {
		t := tmpls[i%len(tmpls)]
		seed := randSeed()
		codes[i] = t.Code
		seeds[i] = seed
		prob, _ := mathengine.GenerateByCode(t.Code, seed, locale)
		qs[i] = WarpQuestion{Index: i + 1, Question: prob.Question}
	}
	s.warpSess[id] = &warpState{
		id:         id,
		studentID:  studentID,
		isPractice: isPractice,
		templates:  codes,
		seeds:      seeds,
		deadline:   s.nowClock().Add((WarpDurationS + WarpFinishGraceS) * time.Second),
	}
	return SuperWarpStart{
		SessionID:  id,
		IsPractice: isPractice,
		DurationS:  WarpDurationS,
		AutoSkipS:  10,
		Questions:  qs,
	}, nil
}

func (s *Service) FinishSuperWarp(ctx context.Context, studentID, sessionID string, locale string, answers map[int]map[string]string) (int, HubSummary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ws, ok := s.warpSess[sessionID]
	if !ok || ws.studentID != studentID {
		return 0, HubSummary{}, fmt.Errorf("super warp session not found")
	}
	if !ws.finished {
		// Server-side 2-minute deadline (plus a small network grace): a
		// submission arriving past it expires the session with score 0 and
		// records nothing.
		if now := s.nowClock(); now.After(ws.deadline) {
			ws.finished = true
			ws.score = 0
			hist := s.history[studentID]
			if hist == nil {
				hist = []ScoreHistoryEntry{}
			}
			return 0, HubSummary{
				IsOpen:             true,
				WeeklyDiamondScore: s.weeklyScores[studentID+":"+s.weekKey()],
				History:            hist,
			}, nil
		}
		score := 0
		for i, code := range ws.templates {
			prob, _ := mathengine.GenerateByCode(code, ws.seeds[i], locale)
			if mathengine.Grade(prob, answers[i+1]).IsCorrect {
				score++
			}
		}
		if err := s.recordScore(ctx, studentID, "SUPER_WARP", ws.isPractice, score, ws.id); err != nil {
			// The score was not recorded; leave the session unfinished so a
			// retry can still record it.
			return 0, HubSummary{}, err
		}
		ws.score = score
		ws.finished = true
	}
	hist := s.history[studentID]
	if hist == nil {
		hist = []ScoreHistoryEntry{}
	}
	return ws.score, HubSummary{
		IsOpen:             true,
		WeeklyDiamondScore: s.weeklyScores[studentID+":"+s.weekKey()],
		History:            hist,
	}, nil
}

// recordScore applies the ledger FIRST (a failure propagates and leaves the
// session unrecorded so the student can retry), then persists weekly score +
// history durably (PG mode) and mirrors them in memory. The session ID is a
// random UUID, so the ledger idempotency key stays unique across restarts.
func (s *Service) recordScore(ctx context.Context, studentID, mode string, isPractice bool, score int, sessionID string) error {
	// Practice mode NEVER alters weekly diamond S score or the ledger!
	if !isPractice && score > 0 && s.ledger != nil {
		if _, err := s.ledger.Apply(ctx, economy.MutationRequest{
			StudentID:      studentID,
			Currency:       economy.CurrencySuperS,
			Delta:          int64(score),
			SourceType:     mode,
			SourceID:       sessionID,
			IdempotencyKey: fmt.Sprintf("supernova:%s:%s", studentID, sessionID),
		}); err != nil {
			return fmt.Errorf("%w: %v", ErrRewardPending, err)
		}
	}

	entry := ScoreHistoryEntry{
		ID:         sessionID,
		Mode:       mode,
		IsPractice: isPractice,
		Score:      score,
		RecordedAt: s.nowClock().UTC(),
	}
	week := s.weekKey()

	if s.store != nil {
		var weeklyValue int
		if err := s.store.SaveMerged(ctx, studentID, supernovaProgressModule, func(current []byte) (any, error) {
			var doc supernovaProgressDoc
			if len(current) > 0 {
				if err := json.Unmarshal(current, &doc); err != nil {
					return nil, err
				}
			}
			if doc.Weekly == nil {
				doc.Weekly = map[string]int{}
			}
			if !isPractice {
				doc.Weekly[week] += score
			}
			doc.History = append([]ScoreHistoryEntry{entry}, doc.History...)
			if len(doc.History) > HistoryCap {
				doc.History = doc.History[:HistoryCap]
			}
			weeklyValue = doc.Weekly[week]
			return doc, nil
		}); err != nil {
			return err
		}
		s.weeklyScores[studentID+":"+week] = weeklyValue
		// Reload the merged history so the in-memory mirror matches the doc.
		var doc supernovaProgressDoc
		if _, err := s.store.Load(ctx, studentID, supernovaProgressModule, &doc); err == nil {
			s.history[studentID] = doc.History
		}
		return nil
	}

	s.history[studentID] = append([]ScoreHistoryEntry{entry}, s.history[studentID]...)
	if len(s.history[studentID]) > HistoryCap {
		s.history[studentID] = s.history[studentID][:HistoryCap]
	}
	if !isPractice && score > 0 {
		s.weeklyScores[studentID+":"+week] += score
	}
	return nil
}

func renderSight(st *sightState, locale string) SuperSightSession {
	foundOnly := make([]DiffTarget, 0, st.found)
	for _, t := range st.targets {
		if t.Found {
			foundOnly = append(foundOnly, t)
		}
	}
	return SuperSightSession{
		SessionID:   st.id,
		PuzzleID:    st.puzzleID,
		Title:       st.title.For(locale),
		FoundCount:  st.found,
		WrongCount:  st.wrong,
		MaxWrong:    SuperSightMaxWrong,
		Score:       st.score,
		Completed:   st.completed,
		FoundPoints: foundOnly,
		TimeLimitS:  SuperSightTimeLimitS,
	}
}

type Handler struct {
	svc     *Service
	authSvc *auth.Service
}

func NewHandler(svc *Service, authSvc *auth.Service) *Handler {
	return &Handler{svc: svc, authSvc: authSvc}
}

type authIn struct {
	CookieAccess  string `cookie:"pl_access"`
	Authorization string `header:"Authorization"`
	Locale        string `query:"locale" default:"en"`
}

// resolve returns the authenticated student identity. Grade and timezone come
// from the trusted principal — never from client query strings — and missing,
// invalid, or non-student credentials are rejected.
func (h *Handler) resolve(ctx context.Context, in authIn) (auth.StudentIdentity, string, error) {
	locale := in.Locale
	if locale != "id" {
		locale = "en"
	}
	tok := reqguard.Token(in.CookieAccess, in.Authorization)
	ident, err := h.authSvc.ResolveStudent(ctx, tok)
	if err != nil {
		return auth.StudentIdentity{}, locale, auth.StudentAuthHTTPError(err)
	}
	return ident, locale, nil
}

func (h *Handler) RegisterRoutes(_ chi.Router, api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "supernova-hub",
		Method:      http.MethodGet,
		Path:        "/api/v1/super-nova/hub",
		Summary:     "Get Super Nova weekly diamond S score and score history",
		Tags:        []string{"SuperNova"},
	}, func(ctx context.Context, in *struct {
		authIn
		Page int `query:"page" default:"1"`
	}) (*struct{ Body HubSummary }, error) {
		ident, _, err := h.resolve(ctx, in.authIn)
		if err != nil {
			return nil, err
		}
		return &struct{ Body HubSummary }{Body: h.svc.GetHub(ctx, ident.ID, ident.Timezone, in.Page)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "supernova-sight-start",
		Method:      http.MethodPost,
		Path:        "/api/v1/super-nova/super-sight/start",
		Summary:     "Start a Super Sight spot-the-difference puzzle (5 diffs, max 5 wrong, 5 min)",
		Tags:        []string{"SuperNova"},
	}, func(ctx context.Context, in *struct {
		authIn
		Body struct {
			IsPractice bool `json:"isPractice"`
		}
	}) (*struct{ Body SuperSightSession }, error) {
		ident, locale, err := h.resolve(ctx, in.authIn)
		if err != nil {
			return nil, err
		}
		sess, err := h.svc.StartSuperSight(ident.ID, ident.Timezone, locale, in.Body.IsPractice)
		if err != nil {
			return nil, huma.Error403Forbidden(err.Error())
		}
		return &struct{ Body SuperSightSession }{Body: sess}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "supernova-sight-tap",
		Method:      http.MethodPost,
		Path:        "/api/v1/super-nova/super-sight/tap",
		Summary:     "Submit a normalized (0..1) tap coordinate on the Super Sight canvas",
		Tags:        []string{"SuperNova"},
	}, func(ctx context.Context, in *struct {
		authIn
		Body struct {
			SessionID string  `json:"sessionId"`
			X         float64 `json:"x"`
			Y         float64 `json:"y"`
		}
	}) (*struct {
		Body struct {
			Hit     bool              `json:"hit"`
			Session SuperSightSession `json:"session"`
		}
	}, error) {
		ident, locale, err := h.resolve(ctx, in.authIn)
		if err != nil {
			return nil, err
		}
		hit, sess, err := h.svc.TapSuperSight(ctx, ident.ID, in.Body.SessionID, in.Body.X, in.Body.Y, locale)
		if err != nil && !errors.Is(err, ErrSessionEnded) {
			if errors.Is(err, ErrRewardPending) {
				return nil, huma.Error503ServiceUnavailable(err.Error())
			}
			return nil, huma.Error400BadRequest(err.Error())
		}
		out := &struct {
			Body struct {
				Hit     bool              `json:"hit"`
				Session SuperSightSession `json:"session"`
			}
		}{}
		out.Body.Hit = hit
		out.Body.Session = sess
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "supernova-warp-start",
		Method:      http.MethodPost,
		Path:        "/api/v1/super-nova/super-warp/start",
		Summary:     "Start a 2-minute Super Warp rapid-fire math sprint",
		Tags:        []string{"SuperNova"},
	}, func(ctx context.Context, in *struct {
		authIn
		Body struct {
			IsPractice bool `json:"isPractice"`
		}
	}) (*struct{ Body SuperWarpStart }, error) {
		ident, locale, err := h.resolve(ctx, in.authIn)
		if err != nil {
			return nil, err
		}
		start, err := h.svc.StartSuperWarp(ident.ID, ident.Timezone, locale, ident.Grade, in.Body.IsPractice)
		if err != nil {
			return nil, huma.Error403Forbidden(err.Error())
		}
		return &struct{ Body SuperWarpStart }{Body: start}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "supernova-warp-finish",
		Method:      http.MethodPost,
		Path:        "/api/v1/super-nova/super-warp/finish",
		Summary:     "Grade Super Warp answers and update weekly S score if Challenge mode",
		Tags:        []string{"SuperNova"},
	}, func(ctx context.Context, in *struct {
		authIn
		Body struct {
			SessionID string                    `json:"sessionId"`
			Answers   map[int]map[string]string `json:"answers"`
		}
	}) (*struct {
		Body struct {
			Score int        `json:"score"`
			Hub   HubSummary `json:"hub"`
		}
	}, error) {
		ident, locale, err := h.resolve(ctx, in.authIn)
		if err != nil {
			return nil, err
		}
		score, hub, err := h.svc.FinishSuperWarp(ctx, ident.ID, in.Body.SessionID, locale, in.Body.Answers)
		if err != nil {
			if errors.Is(err, ErrRewardPending) {
				return nil, huma.Error503ServiceUnavailable(err.Error())
			}
			return nil, huma.Error400BadRequest(err.Error())
		}
		out := &struct {
			Body struct {
				Score int        `json:"score"`
				Hub   HubSummary `json:"hub"`
			}
		}{}
		out.Body.Score = score
		out.Body.Hub = hub
		return out, nil
	})
}

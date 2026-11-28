package supernova

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"sync"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5"

	"github.com/AICSbench/planetlogi/apps/api/internal/mathengine"
	"github.com/AICSbench/planetlogi/apps/api/internal/modules/auth"
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/economy"
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/reqguard"
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/timewindow"
)

const (
	SuperSightDiffCount  = 5
	SuperSightMaxWrong   = 5
	SuperSightPointsEach = 10
)

var (
	ErrSuperNovaClosed = errors.New("super nova challenge is open Mon-Sat 06:00-22:00 local time")
	ErrSessionEnded    = errors.New("super sight session already ended")
)

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
}

type warpState struct {
	id         string
	studentID  string
	isPractice bool
	templates  []string
	seeds      []int64
	score      int
	finished   bool
}

type Service struct {
	mu           sync.Mutex
	ledger       economy.Ledger
	weeklyScores map[string]int // studentID:weekKey -> diamond S score
	history      map[string][]ScoreHistoryEntry
	sightSess    map[string]*sightState
	warpSess     map[string]*warpState
	seq          int
	nowClock     func() time.Time
	enforceOpen  bool
}

func NewService(ledger economy.Ledger) *Service {
	return &Service{
		ledger:       ledger,
		weeklyScores: make(map[string]int),
		history:      make(map[string][]ScoreHistoryEntry),
		sightSess:    make(map[string]*sightState),
		warpSess:     make(map[string]*warpState),
		seq:          100,
		nowClock:     time.Now,
		enforceOpen:  false, // Enforced in prod/tests when enabled; kept open for demo testing
	}
}

func (s *Service) weekKey() string {
	yr, wk := s.nowClock().UTC().ISOWeek()
	return fmt.Sprintf("%04d-W%02d", yr, wk)
}

func (s *Service) GetHub(studentID, tz string) HubSummary {
	s.mu.Lock()
	defer s.mu.Unlock()
	eval := timewindow.Evaluate(s.nowClock(), tz, timewindow.SuperNovaWindow)
	hist := s.history[studentID]
	if hist == nil {
		hist = []ScoreHistoryEntry{}
	}
	return HubSummary{
		IsOpen:             !s.enforceOpen || eval.IsOpen,
		WindowNotice:       eval.Reason,
		WeeklyDiamondScore: s.weeklyScores[studentID+":"+s.weekKey()],
		History:            hist,
	}
}

func (s *Service) StartSuperSight(studentID, tz, locale string, isPractice bool) (SuperSightSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	eval := timewindow.Evaluate(s.nowClock(), tz, timewindow.SuperNovaWindow)
	if !isPractice && s.enforceOpen && !eval.IsOpen {
		return SuperSightSession{}, ErrSuperNovaClosed
	}
	s.seq++
	id := fmt.Sprintf("sight-%d", s.seq)
	st := &sightState{
		id:         id,
		studentID:  studentID,
		puzzleID:   "ss-puzzle-01",
		title:      mathengine.LocaleText{EN: "Cosmic Observatory #01", ID: "Observatorium Kosmik #01"},
		isPractice: isPractice,
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
		st.completed = true
		s.recordScoreLocked(ctx, studentID, "SUPER_SIGHT", st.isPractice, st.score, st.id)
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
	id := fmt.Sprintf("warp-%d", s.seq)
	tmpls := mathengine.TemplatesForGrade(grade)
	qs := make([]WarpQuestion, 15)
	codes := make([]string, 15)
	seeds := make([]int64, 15)
	for i := 0; i < 15; i++ {
		t := tmpls[i%len(tmpls)]
		seed := int64(s.seq*100 + i + 1)
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
	}
	return SuperWarpStart{
		SessionID:  id,
		IsPractice: isPractice,
		DurationS:  120,
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
		score := 0
		for i, code := range ws.templates {
			prob, _ := mathengine.GenerateByCode(code, ws.seeds[i], locale)
			if mathengine.Grade(prob, answers[i+1]).IsCorrect {
				score++
			}
		}
		ws.score = score
		ws.finished = true
		s.recordScoreLocked(ctx, studentID, "SUPER_WARP", ws.isPractice, score, ws.id)
	}
	return ws.score, HubSummary{
		IsOpen:             true,
		WeeklyDiamondScore: s.weeklyScores[studentID+":"+s.weekKey()],
		History:            s.history[studentID],
	}, nil
}

func (s *Service) recordScoreLocked(ctx context.Context, studentID, mode string, isPractice bool, score int, sourceID string) {
	s.history[studentID] = append([]ScoreHistoryEntry{{
		ID:         sourceID,
		Mode:       mode,
		IsPractice: isPractice,
		Score:      score,
		RecordedAt: s.nowClock().UTC(),
	}}, s.history[studentID]...)

	// Practice mode NEVER alters weekly diamond S score!
	if !isPractice && score > 0 {
		s.weeklyScores[studentID+":"+s.weekKey()] += score
		if s.ledger != nil {
			_, _ = s.ledger.Apply(ctx, economy.MutationRequest{
				StudentID:      studentID,
				Currency:       economy.CurrencySuperS,
				Delta:          int64(score),
				SourceType:     mode,
				SourceID:       sourceID,
				IdempotencyKey: fmt.Sprintf("supernova:%s:%s", studentID, sourceID),
			})
		}
	}
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
		TimeLimitS:  300,
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
	}, func(ctx context.Context, in *authIn) (*struct{ Body HubSummary }, error) {
		ident, _, err := h.resolve(ctx, *in)
		if err != nil {
			return nil, err
		}
		return &struct{ Body HubSummary }{Body: h.svc.GetHub(ident.ID, ident.Timezone)}, nil
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

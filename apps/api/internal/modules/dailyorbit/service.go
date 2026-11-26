package dailyorbit

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5"

	"github.com/AICSbench/planetlogi/apps/api/internal/mathengine"
	"github.com/AICSbench/planetlogi/apps/api/internal/modules/auth"
	"github.com/AICSbench/planetlogi/apps/api/internal/modules/streak"
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/economy"
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/timewindow"
)

const (
	MaxChangeQuestionsPerDay = 3
	CompletionBonusAC        = 5
)

var (
	ErrOutsideOperatingHours = errors.New("daily orbit is open 06:00-22:00 local time")
	ErrAlreadyAnswered       = errors.New("question already locked after submission or skip")
	ErrChangeLimitReached    = errors.New("daily change-question limit (3/day) reached")
	ErrTierLocked            = errors.New("difficulty tier requires more star points")
	ErrInvalidSlot           = errors.New("question slot must be 1..10")
)

type Tier string

const (
	TierElementary   Tier = "ELEMENTARY"
	TierIntermediate Tier = "INTERMEDIATE"
	TierAdvanced     Tier = "ADVANCED"
	TierMaster       Tier = "MASTER"
)

var TierUnlockSP = map[Tier]int64{
	TierElementary:   0,
	TierIntermediate: 15,
	TierAdvanced:     50,
	TierMaster:       100,
}

type SlotStatus string

const (
	SlotPending SlotStatus = "PENDING"
	SlotCorrect SlotStatus = "CORRECT"
	SlotWrong   SlotStatus = "WRONG"
	SlotSkipped SlotStatus = "SKIPPED"
)

type SlotView struct {
	Index           int                         `json:"index"`
	StageName       string                      `json:"stageName"`
	NodeShape       string                      `json:"nodeShape"` // circle | hexagon | star
	RewardSP        int                         `json:"rewardSp"`
	Status          SlotStatus                  `json:"status"`
	EarnedSP        int                         `json:"earnedSp"`
	Question        mathengine.RenderedQuestion `json:"question"`
	SubmittedAnswer map[string]string           `json:"submittedAnswer,omitempty"`
	WorkedSolution  *mathengine.WorkedSolution  `json:"workedSolution,omitempty"`
}

type SPBreakdown struct {
	DailyOrbit   int `json:"dailyOrbit"`
	StarDuels    int `json:"starDuels"`
	SundayMeteor int `json:"sundayMeteor"`
	TotalToday   int `json:"totalToday"`
}

type SessionView struct {
	LocalDate            string      `json:"localDate"`
	IsOpen               bool        `json:"isOpen"`
	WindowNotice         string      `json:"windowNotice,omitempty"`
	Tier                 Tier        `json:"tier"`
	UnlockedTiers        []Tier      `json:"unlockedTiers"`
	ActiveCurricula      []string    `json:"activeCurricula"`
	ChangeQuestionUsed   int         `json:"changeQuestionUsed"`
	ChangeQuestionLimit  int         `json:"changeQuestionLimit"`
	CorrectCount         int         `json:"correctCount"`
	CompletedCount       int         `json:"completedCount"`
	TotalSPEarned        int         `json:"totalSpEarned"`
	CompletionBonusAward bool        `json:"completionBonusAwarded"`
	TodaySPBreakdown     SPBreakdown `json:"todaySpBreakdown"`
	Slots                []SlotView  `json:"slots"`
}

type internalSlot struct {
	index        int
	stageName    string
	nodeShape    string
	rewardSP     int
	templateCode string
	seed         int64
	status       SlotStatus
	earnedSP     int
	submitted    map[string]string
}

type studentPrefs struct {
	grade           int
	tier            Tier
	activeCurricula []string
	totalSP         int64
}

type internalSession struct {
	localDate            string
	tier                 Tier
	slots                []*internalSlot
	changeQuestionUsed   int
	correctCount         int
	completedCount       int
	totalSPEarned        int
	completionBonusAward bool
}

type Service struct {
	mu          sync.Mutex
	ledger      economy.Ledger
	streakSvc   *streak.Service
	sessions    map[string]*internalSession // key: studentID + ":" + localDate
	prefs       map[string]*studentPrefs
	nowClock    func() time.Time
	enforceOpen bool
}

func NewService(ledger economy.Ledger, streakSvc *streak.Service) *Service {
	return &Service{
		ledger:      ledger,
		streakSvc:   streakSvc,
		sessions:    make(map[string]*internalSession),
		prefs:       make(map[string]*studentPrefs),
		nowClock:    time.Now,
		enforceOpen: true,
	}
}

func stageForIndex(idx int) (name, shape string, sp int) {
	switch {
	case idx <= 3:
		return "Launchpad Warmup", "circle", 1
	case idx <= 5:
		return "Asteroid Hop", "circle", 1
	case idx <= 8:
		return "Nebula Push", "hexagon", 2
	default:
		return "Supernova Finale", "star", 3
	}
}

func (s *Service) getPrefsLocked(studentID string, grade int) *studentPrefs {
	p, ok := s.prefs[studentID]
	if !ok {
		if grade < 1 || grade > 6 {
			grade = 1
		}
		p = &studentPrefs{
			grade:           grade,
			tier:            TierElementary,
			activeCurricula: []string{"SINGAPORE_MOE", "HOTS_WHEEL"},
			totalSP:         12,
		}
		s.prefs[studentID] = p
	}
	return p
}

func (s *Service) getOrCreateSessionLocked(studentID string, grade int, localDate string) *internalSession {
	key := studentID + ":" + localDate
	if sess, ok := s.sessions[key]; ok {
		return sess
	}
	prefs := s.getPrefsLocked(studentID, grade)
	templates := mathengine.TemplatesForGrade(prefs.grade)
	allTemplates := mathengine.Registry()

	slots := make([]*internalSlot, 10)
	baseSeed := int64(20261126)
	for i := 0; i < 10; i++ {
		idx := i + 1
		stage, shape, sp := stageForIndex(idx)
		var tmpl mathengine.TemplateMeta
		if i < len(templates) {
			tmpl = templates[i]
		} else {
			tmpl = allTemplates[(prefs.grade*3+i)%len(allTemplates)]
		}
		slots[i] = &internalSlot{
			index:        idx,
			stageName:    stage,
			nodeShape:    shape,
			rewardSP:     sp,
			templateCode: tmpl.Code,
			seed:         baseSeed + int64(idx*97+prefs.grade*13),
			status:       SlotPending,
		}
	}

	sess := &internalSession{
		localDate: localDate,
		tier:      prefs.tier,
		slots:     slots,
	}
	s.sessions[key] = sess
	return sess
}

func (s *Service) GetToday(studentID string, grade int, tz, locale string) SessionView {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.nowClock()
	eval := timewindow.Evaluate(now, tz, timewindow.DailyOrbitWindow)
	prefs := s.getPrefsLocked(studentID, grade)
	sess := s.getOrCreateSessionLocked(studentID, prefs.grade, eval.LocalDate)
	return s.renderViewLocked(sess, prefs, eval, locale)
}

func (s *Service) SubmitAnswer(ctx context.Context, studentID string, grade int, tz, locale string, slotIndex int, submitted map[string]string) (SlotView, SessionView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.nowClock()
	eval := timewindow.Evaluate(now, tz, timewindow.DailyOrbitWindow)
	if s.enforceOpen && !eval.IsOpen {
		return SlotView{}, SessionView{}, ErrOutsideOperatingHours
	}
	if slotIndex < 1 || slotIndex > 10 {
		return SlotView{}, SessionView{}, ErrInvalidSlot
	}

	prefs := s.getPrefsLocked(studentID, grade)
	sess := s.getOrCreateSessionLocked(studentID, prefs.grade, eval.LocalDate)
	slot := sess.slots[slotIndex-1]
	if slot.status != SlotPending {
		return SlotView{}, SessionView{}, ErrAlreadyAnswered
	}

	prob, _ := mathengine.GenerateByCode(slot.templateCode, slot.seed, locale)
	gradeRes := mathengine.Grade(prob, submitted)
	slot.submitted = submitted
	sess.completedCount++

	xpDelta := int64(1)
	if gradeRes.IsCorrect {
		slot.status = SlotCorrect
		slot.earnedSP = slot.rewardSP
		sess.correctCount++
		sess.totalSPEarned += slot.rewardSP
		prefs.totalSP += int64(slot.rewardSP)
		xpDelta = 3

		if s.ledger != nil {
			_, _ = s.ledger.Apply(ctx, economy.MutationRequest{
				StudentID:      studentID,
				Currency:       economy.CurrencySP,
				Delta:          int64(slot.rewardSP),
				SourceType:     "DAILY_ORBIT",
				SourceID:       fmt.Sprintf("%s:q%d", sess.localDate, slot.index),
				IdempotencyKey: fmt.Sprintf("daily_orbit:%s:%s:q%d:sp", sess.localDate, studentID, slot.index),
			})
		}
		if s.streakSvc != nil {
			s.streakSvc.RecordDailyOrbitProgress(studentID, tz, now, sess.correctCount)
		}
	} else {
		slot.status = SlotWrong
	}

	if s.ledger != nil {
		_, _ = s.ledger.Apply(ctx, economy.MutationRequest{
			StudentID:      studentID,
			Currency:       economy.CurrencyXP,
			Delta:          xpDelta,
			SourceType:     "DAILY_ORBIT",
			SourceID:       fmt.Sprintf("%s:q%d", sess.localDate, slot.index),
			IdempotencyKey: fmt.Sprintf("daily_orbit:%s:%s:q%d:xp", sess.localDate, studentID, slot.index),
		})
	}

	if sess.completedCount == 10 && !sess.completionBonusAward {
		sess.completionBonusAward = true
		if s.ledger != nil {
			_, _ = s.ledger.Apply(ctx, economy.MutationRequest{
				StudentID:      studentID,
				Currency:       economy.CurrencyAC,
				Delta:          CompletionBonusAC,
				SourceType:     "DAILY_ORBIT_BONUS",
				SourceID:       sess.localDate,
				IdempotencyKey: fmt.Sprintf("daily_orbit:%s:%s:bonus_ac", sess.localDate, studentID),
			})
		}
	}

	view := s.renderViewLocked(sess, prefs, eval, locale)
	return view.Slots[slotIndex-1], view, nil
}

func (s *Service) SkipSlot(ctx context.Context, studentID string, grade int, tz, locale string, slotIndex int) (SlotView, SessionView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.nowClock()
	eval := timewindow.Evaluate(now, tz, timewindow.DailyOrbitWindow)
	if s.enforceOpen && !eval.IsOpen {
		return SlotView{}, SessionView{}, ErrOutsideOperatingHours
	}
	if slotIndex < 1 || slotIndex > 10 {
		return SlotView{}, SessionView{}, ErrInvalidSlot
	}
	prefs := s.getPrefsLocked(studentID, grade)
	sess := s.getOrCreateSessionLocked(studentID, prefs.grade, eval.LocalDate)
	slot := sess.slots[slotIndex-1]
	if slot.status != SlotPending {
		return SlotView{}, SessionView{}, ErrAlreadyAnswered
	}

	slot.status = SlotSkipped
	slot.earnedSP = 0
	sess.completedCount++

	if sess.completedCount == 10 && !sess.completionBonusAward {
		sess.completionBonusAward = true
		if s.ledger != nil {
			_, _ = s.ledger.Apply(ctx, economy.MutationRequest{
				StudentID:      studentID,
				Currency:       economy.CurrencyAC,
				Delta:          CompletionBonusAC,
				SourceType:     "DAILY_ORBIT_BONUS",
				SourceID:       sess.localDate,
				IdempotencyKey: fmt.Sprintf("daily_orbit:%s:%s:bonus_ac", sess.localDate, studentID),
			})
		}
	}

	view := s.renderViewLocked(sess, prefs, eval, locale)
	return view.Slots[slotIndex-1], view, nil
}

func (s *Service) ChangeQuestion(studentID string, grade int, tz, locale string, slotIndex int, mode string) (SlotView, SessionView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.nowClock()
	eval := timewindow.Evaluate(now, tz, timewindow.DailyOrbitWindow)
	if s.enforceOpen && !eval.IsOpen {
		return SlotView{}, SessionView{}, ErrOutsideOperatingHours
	}
	if slotIndex < 1 || slotIndex > 10 {
		return SlotView{}, SessionView{}, ErrInvalidSlot
	}
	prefs := s.getPrefsLocked(studentID, grade)
	sess := s.getOrCreateSessionLocked(studentID, prefs.grade, eval.LocalDate)
	if sess.changeQuestionUsed >= MaxChangeQuestionsPerDay {
		return SlotView{}, SessionView{}, ErrChangeLimitReached
	}
	slot := sess.slots[slotIndex-1]
	if slot.status != SlotPending {
		return SlotView{}, SessionView{}, ErrAlreadyAnswered
	}

	all := mathengine.Registry()
	picked := all[0]
	for _, tmpl := range all {
		if tmpl.Code == slot.templateCode {
			continue
		}
		if mode == "easier" && tmpl.Difficulty <= 2 {
			picked = tmpl
			break
		}
		if mode == "harder" && tmpl.Difficulty >= 3 {
			picked = tmpl
			break
		}
		if mode == "topic" && tmpl.Grade == prefs.grade {
			picked = tmpl
			break
		}
	}
	slot.templateCode = picked.Code
	slot.seed += 313
	sess.changeQuestionUsed++

	view := s.renderViewLocked(sess, prefs, eval, locale)
	return view.Slots[slotIndex-1], view, nil
}

func (s *Service) SetTier(studentID string, grade int, tz, locale string, tier Tier) (SessionView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	prefs := s.getPrefsLocked(studentID, grade)
	reqSP, ok := TierUnlockSP[tier]
	if !ok || prefs.totalSP < reqSP {
		return SessionView{}, ErrTierLocked
	}
	prefs.tier = tier
	eval := timewindow.Evaluate(s.nowClock(), tz, timewindow.DailyOrbitWindow)
	sess := s.getOrCreateSessionLocked(studentID, prefs.grade, eval.LocalDate)
	sess.tier = tier
	return s.renderViewLocked(sess, prefs, eval, locale), nil
}

func (s *Service) SetCurricula(studentID string, grade int, tz, locale string, curricula []string) SessionView {
	s.mu.Lock()
	defer s.mu.Unlock()
	prefs := s.getPrefsLocked(studentID, grade)
	if len(curricula) > 0 {
		prefs.activeCurricula = curricula
	}
	eval := timewindow.Evaluate(s.nowClock(), tz, timewindow.DailyOrbitWindow)
	sess := s.getOrCreateSessionLocked(studentID, prefs.grade, eval.LocalDate)
	return s.renderViewLocked(sess, prefs, eval, locale)
}

func (s *Service) renderViewLocked(sess *internalSession, prefs *studentPrefs, eval timewindow.Evaluation, locale string) SessionView {
	unlocked := make([]Tier, 0, 4)
	for _, t := range []Tier{TierElementary, TierIntermediate, TierAdvanced, TierMaster} {
		if prefs.totalSP >= TierUnlockSP[t] {
			unlocked = append(unlocked, t)
		}
	}

	slots := make([]SlotView, len(sess.slots))
	for i, sl := range sess.slots {
		prob, _ := mathengine.GenerateByCode(sl.templateCode, sl.seed, locale)
		sv := SlotView{
			Index:           sl.index,
			StageName:       sl.stageName,
			NodeShape:       sl.nodeShape,
			RewardSP:        sl.rewardSP,
			Status:          sl.status,
			EarnedSP:        sl.earnedSP,
			Question:        prob.Question,
			SubmittedAnswer: sl.submitted,
		}
		// Never expose WorkedSolution while slot is still PENDING!
		if sl.status != SlotPending {
			sol := prob.Solution
			sv.WorkedSolution = &sol
		}
		slots[i] = sv
	}

	return SessionView{
		LocalDate:            sess.localDate,
		IsOpen:               !s.enforceOpen || eval.IsOpen,
		WindowNotice:         eval.Reason,
		Tier:                 sess.tier,
		UnlockedTiers:        unlocked,
		ActiveCurricula:      prefs.activeCurricula,
		ChangeQuestionUsed:   sess.changeQuestionUsed,
		ChangeQuestionLimit:  MaxChangeQuestionsPerDay,
		CorrectCount:         sess.correctCount,
		CompletedCount:       sess.completedCount,
		TotalSPEarned:        sess.totalSPEarned,
		CompletionBonusAward: sess.completionBonusAward,
		TodaySPBreakdown: SPBreakdown{
			DailyOrbit:   sess.totalSPEarned,
			StarDuels:    0,
			SundayMeteor: 0,
			TotalToday:   sess.totalSPEarned,
		},
		Slots: slots,
	}
}

type Handler struct {
	svc     *Service
	authSvc *auth.Service
}

func NewHandler(svc *Service, authSvc *auth.Service) *Handler {
	return &Handler{svc: svc, authSvc: authSvc}
}

type commonIn struct {
	CookieAccess  string `cookie:"pl_access"`
	Authorization string `header:"Authorization"`
	Locale        string `query:"locale" default:"en"`
	Grade         int    `query:"grade" default:"1"`
}

func (h *Handler) resolveUser(ctx context.Context, in commonIn) (string, int, string, string) {
	locale := in.Locale
	if locale != "id" {
		locale = "en"
	}
	grade := in.Grade
	if grade < 1 || grade > 6 {
		grade = 1
	}
	if h.authSvc != nil {
		tok := in.CookieAccess
		if tok == "" && len(in.Authorization) > 7 {
			tok = in.Authorization[7:]
		}
		if tok != "" {
			if p, err := h.authSvc.ResolvePrincipal(ctx, tok); err == nil {
				g := p.Grade
				if g < 1 || g > 6 {
					g = grade
				}
				return p.ID, g, p.Timezone, locale
			}
		}
	}
	return "demo-pilot1", grade, "Asia/Jakarta", locale
}

func (h *Handler) RegisterRoutes(_ chi.Router, api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "daily-orbit-today",
		Method:      http.MethodGet,
		Path:        "/api/v1/daily-orbit/today",
		Summary:     "Get today's 10-question Daily Orbit progression path",
		Tags:        []string{"DailyOrbit"},
	}, func(ctx context.Context, in *commonIn) (*struct{ Body SessionView }, error) {
		id, grade, tz, locale := h.resolveUser(ctx, *in)
		return &struct{ Body SessionView }{Body: h.svc.GetToday(id, grade, tz, locale)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "daily-orbit-submit",
		Method:      http.MethodPost,
		Path:        "/api/v1/daily-orbit/submit",
		Summary:     "Submit a single-attempt answer for a Daily Orbit slot",
		Tags:        []string{"DailyOrbit"},
	}, func(ctx context.Context, in *struct {
		commonIn
		Body struct {
			SlotIndex int               `json:"slotIndex" minimum:"1" maximum:"10"`
			Answers   map[string]string `json:"answers"`
		}
	}) (*struct {
		Body struct {
			Slot    SlotView    `json:"slot"`
			Session SessionView `json:"session"`
		}
	}, error) {
		id, grade, tz, locale := h.resolveUser(ctx, in.commonIn)
		slot, sess, err := h.svc.SubmitAnswer(ctx, id, grade, tz, locale, in.Body.SlotIndex, in.Body.Answers)
		if err != nil {
			switch {
			case errors.Is(err, ErrAlreadyAnswered):
				return nil, huma.Error409Conflict(err.Error())
			case errors.Is(err, ErrOutsideOperatingHours):
				return nil, huma.Error403Forbidden(err.Error())
			default:
				return nil, huma.Error400BadRequest(err.Error())
			}
		}
		out := &struct {
			Body struct {
				Slot    SlotView    `json:"slot"`
				Session SessionView `json:"session"`
			}
		}{}
		out.Body.Slot = slot
		out.Body.Session = sess
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "daily-orbit-skip",
		Method:      http.MethodPost,
		Path:        "/api/v1/daily-orbit/skip",
		Summary:     "Skip a Daily Orbit question (0 SP) and reveal its worked solution",
		Tags:        []string{"DailyOrbit"},
	}, func(ctx context.Context, in *struct {
		commonIn
		Body struct {
			SlotIndex int `json:"slotIndex" minimum:"1" maximum:"10"`
		}
	}) (*struct {
		Body struct {
			Slot    SlotView    `json:"slot"`
			Session SessionView `json:"session"`
		}
	}, error) {
		id, grade, tz, locale := h.resolveUser(ctx, in.commonIn)
		slot, sess, err := h.svc.SkipSlot(ctx, id, grade, tz, locale, in.Body.SlotIndex)
		if err != nil {
			if errors.Is(err, ErrAlreadyAnswered) {
				return nil, huma.Error409Conflict(err.Error())
			}
			return nil, huma.Error400BadRequest(err.Error())
		}
		out := &struct {
			Body struct {
				Slot    SlotView    `json:"slot"`
				Session SessionView `json:"session"`
			}
		}{}
		out.Body.Slot = slot
		out.Body.Session = sess
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "daily-orbit-change-question",
		Method:      http.MethodPost,
		Path:        "/api/v1/daily-orbit/change-question",
		Summary:     "Replace a pending Daily Orbit question (max 3 edits/day)",
		Tags:        []string{"DailyOrbit"},
	}, func(ctx context.Context, in *struct {
		commonIn
		Body struct {
			SlotIndex int    `json:"slotIndex" minimum:"1" maximum:"10"`
			Mode      string `json:"mode" enum:"easier,harder,topic"`
		}
	}) (*struct {
		Body struct {
			Slot    SlotView    `json:"slot"`
			Session SessionView `json:"session"`
		}
	}, error) {
		id, grade, tz, locale := h.resolveUser(ctx, in.commonIn)
		slot, sess, err := h.svc.ChangeQuestion(id, grade, tz, locale, in.Body.SlotIndex, in.Body.Mode)
		if err != nil {
			return nil, huma.Error400BadRequest(err.Error())
		}
		out := &struct {
			Body struct {
				Slot    SlotView    `json:"slot"`
				Session SessionView `json:"session"`
			}
		}{}
		out.Body.Slot = slot
		out.Body.Session = sess
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "daily-orbit-set-tier",
		Method:      http.MethodPatch,
		Path:        "/api/v1/daily-orbit/tier",
		Summary:     "Switch unlocked Daily Orbit difficulty tier",
		Tags:        []string{"DailyOrbit"},
	}, func(ctx context.Context, in *struct {
		commonIn
		Body struct {
			Tier Tier `json:"tier" enum:"ELEMENTARY,INTERMEDIATE,ADVANCED,MASTER"`
		}
	}) (*struct{ Body SessionView }, error) {
		id, grade, tz, locale := h.resolveUser(ctx, in.commonIn)
		sess, err := h.svc.SetTier(id, grade, tz, locale, in.Body.Tier)
		if err != nil {
			return nil, huma.Error403Forbidden(err.Error())
		}
		return &struct{ Body SessionView }{Body: sess}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "daily-orbit-set-curricula",
		Method:      http.MethodPatch,
		Path:        "/api/v1/daily-orbit/curricula",
		Summary:     "Update active Daily Orbit curriculum pools",
		Tags:        []string{"DailyOrbit"},
	}, func(ctx context.Context, in *struct {
		commonIn
		Body struct {
			Curricula []string `json:"curricula" minItems:"1"`
		}
	}) (*struct{ Body SessionView }, error) {
		id, grade, tz, locale := h.resolveUser(ctx, in.commonIn)
		return &struct{ Body SessionView }{Body: h.svc.SetCurricula(id, grade, tz, locale, in.Body.Curricula)}, nil
	})
}

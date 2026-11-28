package dailyorbit

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"net/http"
	"sync"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

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
	ErrOutsideOperatingHours  = errors.New("daily orbit is open 06:00-22:00 local time")
	ErrAlreadyAnswered        = errors.New("question already locked after submission or skip")
	ErrChangeLimitReached     = errors.New("daily change-question limit (3/day) reached")
	ErrTierLocked             = errors.New("difficulty tier requires more star points")
	ErrInvalidSlot            = errors.New("question slot must be 1..10")
	ErrQuestionReplaced       = errors.New("question was replaced; please refresh")
	ErrNoReplacementAvailable = errors.New("no replacement question available for this grade")
	ErrInvalidCurricula       = errors.New("at least one valid curriculum code is required")
)

var validCurriculaSet = map[string]bool{
	"KURIKULUM_MERDEKA": true,
	"SINGAPORE_MOE":     true,
	"CAMBRIDGE_0096":    true,
	"CHALLENGING_MATHS": true,
	"HOTS_WHEEL":        true,
	"EXPLORER_SERIES":   true,
}

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

type SkillAttemptRecorder interface {
	RecordSkillAttempt(studentID, skillCode string, isCorrect bool) error
}

type Service struct {
	mu          sync.Mutex
	ledger      economy.Ledger
	streakSvc   *streak.Service
	attemptRec  SkillAttemptRecorder
	pool        *pgxpool.Pool
	subjectID   string
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

func NewServiceWithPG(pool *pgxpool.Pool, ledger economy.Ledger, streakSvc *streak.Service) *Service {
	svc := NewService(ledger, streakSvc)
	svc.pool = pool
	return svc
}

func (s *Service) SetAttemptRecorder(rec SkillAttemptRecorder) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.attemptRec = rec
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

func (s *Service) getPrefsLocked(studentID string, grade int) (*studentPrefs, error) {
	if s.pool != nil && isUUID(studentID) {
		pgPrefs, err := s.loadPrefsPG(context.Background(), studentID)
		if err != nil {
			return nil, err
		}
		if pgPrefs != nil {
			if grade >= 1 && grade <= 6 {
				pgPrefs.grade = grade
			}
			s.prefs[studentID] = pgPrefs
			return pgPrefs, nil
		}
	}
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
		if err := s.savePrefsPG(context.Background(), studentID, p); err != nil {
			return nil, err
		}
		s.prefs[studentID] = p
	}
	return p, nil
}

func deriveSlotSeed(studentID, localDate string, slotIdx int, tier Tier) int64 {
	h := fnv.New64a()
	_, _ = fmt.Fprintf(h, "%s|%s|%d|%s", studentID, localDate, slotIdx, tier)
	// Keep seed within IEEE-754 53-bit safe integer range for JSON round-tripping in JS clients.
	return int64(h.Sum64()&0x000fffffffffffff) + 1
}

func filterTemplatesByTierAndCurricula(pool []mathengine.TemplateMeta, tier Tier, curricula []string) []mathengine.TemplateMeta {
	minDiff, maxDiff := 1, 5
	switch tier {
	case TierElementary:
		maxDiff = 3
	case TierIntermediate:
		minDiff, maxDiff = 2, 4
	case TierAdvanced:
		minDiff, maxDiff = 3, 5
	case TierMaster:
		minDiff, maxDiff = 4, 5
	}
	currSet := make(map[string]bool, len(curricula))
	for _, c := range curricula {
		currSet[c] = true
	}

	byTierAndCurr := make([]mathengine.TemplateMeta, 0, len(pool))
	byCurrOnly := make([]mathengine.TemplateMeta, 0, len(pool))
	byTierOnly := make([]mathengine.TemplateMeta, 0, len(pool))
	for _, t := range pool {
		inTier := t.Difficulty >= minDiff && t.Difficulty <= maxDiff
		inCurr := len(currSet) == 0 || t.CurriculumCode == "" || currSet[t.CurriculumCode]
		if inTier && inCurr {
			byTierAndCurr = append(byTierAndCurr, t)
		}
		if inCurr {
			byCurrOnly = append(byCurrOnly, t)
		}
		if inTier {
			byTierOnly = append(byTierOnly, t)
		}
	}
	if len(byTierAndCurr) > 0 {
		return byTierAndCurr
	}
	if len(byCurrOnly) > 0 {
		return byCurrOnly
	}
	if len(byTierOnly) > 0 {
		return byTierOnly
	}
	return pool
}

func (s *Service) getOrCreateSessionLocked(studentID string, prefs *studentPrefs, localDate string) (*internalSession, error) {
	key := studentID + ":" + localDate
	if s.pool != nil && isUUID(studentID) {
		pgSess, err := s.loadSessionPG(context.Background(), studentID, localDate)
		if err != nil {
			return nil, err
		}
		if pgSess != nil {
			s.sessions[key] = pgSess
			return pgSess, nil
		}
	}
	if sess, ok := s.sessions[key]; ok {
		return sess, nil
	}
	gradePool := mathengine.TemplatesForGrade(prefs.grade)
	if len(gradePool) == 0 {
		gradePool = mathengine.Registry()
	}
	templates := filterTemplatesByTierAndCurricula(gradePool, prefs.tier, prefs.activeCurricula)

	slots := make([]*internalSlot, 10)
	for i := 0; i < 10; i++ {
		idx := i + 1
		stage, shape, sp := stageForIndex(idx)
		tmpl := templates[i%len(templates)]
		slots[i] = &internalSlot{
			index:        idx,
			stageName:    stage,
			nodeShape:    shape,
			rewardSP:     sp,
			templateCode: tmpl.Code,
			seed:         deriveSlotSeed(studentID, localDate, idx, prefs.tier),
			status:       SlotPending,
		}
	}

	sess := &internalSession{
		localDate: localDate,
		tier:      prefs.tier,
		slots:     slots,
	}
	if err := s.saveSessionPG(context.Background(), studentID, sess); err != nil {
		return nil, err
	}
	s.sessions[key] = sess
	return sess, nil
}

func (s *Service) GetToday(studentID string, grade int, tz, locale string) (SessionView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.nowClock()
	eval := timewindow.Evaluate(now, tz, timewindow.DailyOrbitWindow)
	prefs, err := s.getPrefsLocked(studentID, grade)
	if err != nil {
		return SessionView{}, err
	}
	sess, err := s.getOrCreateSessionLocked(studentID, prefs, eval.LocalDate)
	if err != nil {
		return SessionView{}, err
	}
	return s.renderViewLocked(sess, prefs, eval, locale), nil
}

func (s *Service) SubmitAnswer(ctx context.Context, studentID string, grade int, tz, locale string, slotIndex int, submitted map[string]string, templateCode string, seed int64) (SlotView, SessionView, error) {
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

	prefs, err := s.getPrefsLocked(studentID, grade)
	if err != nil {
		return SlotView{}, SessionView{}, err
	}
	sess, err := s.getOrCreateSessionLocked(studentID, prefs, eval.LocalDate)
	if err != nil {
		return SlotView{}, SessionView{}, err
	}
	slot := sess.slots[slotIndex-1]
	if slot.status != SlotPending {
		return SlotView{}, SessionView{}, ErrAlreadyAnswered
	}
	if templateCode == "" || seed == 0 || slot.templateCode != templateCode || slot.seed != seed {
		return SlotView{}, SessionView{}, ErrQuestionReplaced
	}

	prob, _ := mathengine.GenerateByCode(slot.templateCode, slot.seed, locale)
	gradeRes := mathengine.Grade(prob, submitted)

	xpDelta := int64(1)
	if gradeRes.IsCorrect {
		xpDelta = 3
	}
	awardCompletionBonus := sess.completedCount+1 == 10 && !sess.completionBonusAward

	if s.ledger != nil {
		ledgerReqs := make([]economy.MutationRequest, 0, 3)
		if gradeRes.IsCorrect {
			ledgerReqs = append(ledgerReqs, economy.MutationRequest{
				StudentID:      studentID,
				Currency:       economy.CurrencySP,
				Delta:          int64(slot.rewardSP),
				SourceType:     "DAILY_ORBIT",
				SourceID:       fmt.Sprintf("%s:q%d", sess.localDate, slot.index),
				IdempotencyKey: fmt.Sprintf("daily_orbit:%s:%s:q%d:sp", sess.localDate, studentID, slot.index),
			})
		}
		ledgerReqs = append(ledgerReqs, economy.MutationRequest{
			StudentID:      studentID,
			Currency:       economy.CurrencyXP,
			Delta:          xpDelta,
			SourceType:     "DAILY_ORBIT",
			SourceID:       fmt.Sprintf("%s:q%d", sess.localDate, slot.index),
			IdempotencyKey: fmt.Sprintf("daily_orbit:%s:%s:q%d:xp", sess.localDate, studentID, slot.index),
		})
		if awardCompletionBonus {
			ledgerReqs = append(ledgerReqs, economy.MutationRequest{
				StudentID:      studentID,
				Currency:       economy.CurrencyAC,
				Delta:          CompletionBonusAC,
				SourceType:     "DAILY_ORBIT_BONUS",
				SourceID:       sess.localDate,
				IdempotencyKey: fmt.Sprintf("daily_orbit:%s:%s:bonus_ac", sess.localDate, studentID),
			})
		}
		if _, err := s.ledger.ApplyBatch(ctx, ledgerReqs); err != nil {
			return SlotView{}, SessionView{}, fmt.Errorf("award daily orbit rewards: %w", err)
		}
	}
	if awardCompletionBonus {
		sess.completionBonusAward = true
	}

	slot.submitted = submitted
	sess.completedCount++
	if gradeRes.IsCorrect {
		slot.status = SlotCorrect
		slot.earnedSP = slot.rewardSP
		sess.correctCount++
		sess.totalSPEarned += slot.rewardSP
		prefs.totalSP += int64(slot.rewardSP)
		if s.streakSvc != nil {
			if _, err := s.streakSvc.RecordDailyOrbitProgress(studentID, tz, now, sess.correctCount); err != nil {
				return SlotView{}, SessionView{}, fmt.Errorf("record streak progress: %w", err)
			}
		}
	} else {
		slot.status = SlotWrong
	}
	if err := s.saveSessionPG(ctx, studentID, sess); err != nil {
		return SlotView{}, SessionView{}, fmt.Errorf("persist daily orbit session: %w", err)
	}
	if err := s.savePrefsPG(ctx, studentID, prefs); err != nil {
		return SlotView{}, SessionView{}, fmt.Errorf("persist daily orbit prefs: %w", err)
	}
	if s.attemptRec != nil && prob.Question.SkillCode != "" {
		if err := s.attemptRec.RecordSkillAttempt(studentID, prob.Question.SkillCode, gradeRes.IsCorrect); err != nil {
			return SlotView{}, SessionView{}, fmt.Errorf("record skill attempt: %w", err)
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
	prefs, err := s.getPrefsLocked(studentID, grade)
	if err != nil {
		return SlotView{}, SessionView{}, err
	}
	sess, err := s.getOrCreateSessionLocked(studentID, prefs, eval.LocalDate)
	if err != nil {
		return SlotView{}, SessionView{}, err
	}
	slot := sess.slots[slotIndex-1]
	if slot.status != SlotPending {
		return SlotView{}, SessionView{}, ErrAlreadyAnswered
	}

	if sess.completedCount+1 == 10 && !sess.completionBonusAward {
		if s.ledger != nil {
			if _, err := s.ledger.Apply(ctx, economy.MutationRequest{
				StudentID:      studentID,
				Currency:       economy.CurrencyAC,
				Delta:          CompletionBonusAC,
				SourceType:     "DAILY_ORBIT_BONUS",
				SourceID:       sess.localDate,
				IdempotencyKey: fmt.Sprintf("daily_orbit:%s:%s:bonus_ac", sess.localDate, studentID),
			}); err != nil {
				return SlotView{}, SessionView{}, fmt.Errorf("award daily orbit bonus: %w", err)
			}
		}
		sess.completionBonusAward = true
	}
	slot.status = SlotSkipped
	slot.earnedSP = 0
	sess.completedCount++
	if err := s.saveSessionPG(ctx, studentID, sess); err != nil {
		return SlotView{}, SessionView{}, fmt.Errorf("persist daily orbit session: %w", err)
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
	prefs, err := s.getPrefsLocked(studentID, grade)
	if err != nil {
		return SlotView{}, SessionView{}, err
	}
	sess, err := s.getOrCreateSessionLocked(studentID, prefs, eval.LocalDate)
	if err != nil {
		return SlotView{}, SessionView{}, err
	}
	if sess.changeQuestionUsed >= MaxChangeQuestionsPerDay {
		return SlotView{}, SessionView{}, ErrChangeLimitReached
	}
	slot := sess.slots[slotIndex-1]
	if slot.status != SlotPending {
		return SlotView{}, SessionView{}, ErrAlreadyAnswered
	}

	gradePool := mathengine.TemplatesForGrade(prefs.grade)
	var picked *mathengine.TemplateMeta
	for i := range gradePool {
		tmpl := gradePool[i]
		if tmpl.Code == slot.templateCode {
			continue
		}
		if mode == "easier" && tmpl.Difficulty <= 2 {
			picked = &tmpl
			break
		}
		if mode == "harder" && tmpl.Difficulty >= 3 {
			picked = &tmpl
			break
		}
		if mode == "topic" {
			picked = &tmpl
			break
		}
	}
	if picked == nil {
		for i := range gradePool {
			tmpl := gradePool[i]
			if tmpl.Code != slot.templateCode {
				picked = &tmpl
				break
			}
		}
	}
	if picked == nil {
		return SlotView{}, SessionView{}, ErrNoReplacementAvailable
	}
	slot.templateCode = picked.Code
	slot.seed += 313
	sess.changeQuestionUsed++
	if err := s.saveSessionPG(context.Background(), studentID, sess); err != nil {
		return SlotView{}, SessionView{}, fmt.Errorf("persist daily orbit session: %w", err)
	}

	view := s.renderViewLocked(sess, prefs, eval, locale)
	return view.Slots[slotIndex-1], view, nil
}

func (s *Service) SetTier(studentID string, grade int, tz, locale string, tier Tier) (SessionView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	prefs, err := s.getPrefsLocked(studentID, grade)
	if err != nil {
		return SessionView{}, err
	}
	reqSP, ok := TierUnlockSP[tier]
	if !ok || prefs.totalSP < reqSP {
		return SessionView{}, ErrTierLocked
	}
	eval := timewindow.Evaluate(s.nowClock(), tz, timewindow.DailyOrbitWindow)
	sess, err := s.getOrCreateSessionLocked(studentID, prefs, eval.LocalDate)
	if err != nil {
		return SessionView{}, err
	}
	prefs.tier = tier
	sess.tier = tier
	gradePool := mathengine.TemplatesForGrade(prefs.grade)
	if len(gradePool) > 0 {
		filtered := filterTemplatesByTierAndCurricula(gradePool, tier, prefs.activeCurricula)
		for i, sl := range sess.slots {
			if sl.status == SlotPending {
				sl.templateCode = filtered[i%len(filtered)].Code
			}
		}
	}
	if err := s.savePrefsPG(context.Background(), studentID, prefs); err != nil {
		return SessionView{}, fmt.Errorf("persist daily orbit prefs: %w", err)
	}
	if err := s.saveSessionPG(context.Background(), studentID, sess); err != nil {
		return SessionView{}, fmt.Errorf("persist daily orbit session: %w", err)
	}
	return s.renderViewLocked(sess, prefs, eval, locale), nil
}

func (s *Service) SetCurricula(studentID string, grade int, tz, locale string, curricula []string) (SessionView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	valid := make([]string, 0, len(curricula))
	for _, c := range curricula {
		if validCurriculaSet[c] {
			valid = append(valid, c)
		}
	}
	if len(valid) == 0 {
		return SessionView{}, ErrInvalidCurricula
	}
	prefs, err := s.getPrefsLocked(studentID, grade)
	if err != nil {
		return SessionView{}, err
	}
	eval := timewindow.Evaluate(s.nowClock(), tz, timewindow.DailyOrbitWindow)
	sess, err := s.getOrCreateSessionLocked(studentID, prefs, eval.LocalDate)
	if err != nil {
		return SessionView{}, err
	}
	prefs.activeCurricula = valid
	gradePool := mathengine.TemplatesForGrade(prefs.grade)
	if len(gradePool) > 0 {
		filtered := filterTemplatesByTierAndCurricula(gradePool, prefs.tier, prefs.activeCurricula)
		for i, sl := range sess.slots {
			if sl.status == SlotPending {
				sl.templateCode = filtered[i%len(filtered)].Code
			}
		}
	}
	if err := s.savePrefsPG(context.Background(), studentID, prefs); err != nil {
		return SessionView{}, fmt.Errorf("persist daily orbit prefs: %w", err)
	}
	if err := s.saveSessionPG(context.Background(), studentID, sess); err != nil {
		return SessionView{}, fmt.Errorf("persist daily orbit session: %w", err)
	}
	return s.renderViewLocked(sess, prefs, eval, locale), nil
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

type CommonIn struct {
	CookieAccess  string `cookie:"pl_access"`
	Authorization string `header:"Authorization"`
	Locale        string `query:"locale" default:"en"`
	Grade         int    `query:"grade" default:"1"`
}

func (h *Handler) resolveUser(ctx context.Context, in CommonIn, requireAuth bool) (string, int, string, string, error) {
	locale := in.Locale
	if locale != "id" {
		locale = "en"
	}
	grade := in.Grade
	if grade < 1 || grade > 6 {
		grade = 1
	}
	tok := in.CookieAccess
	if tok == "" && len(in.Authorization) > 7 {
		tok = in.Authorization[7:]
	}
	if h.authSvc != nil {
		if tok != "" {
			p, err := h.authSvc.ResolvePrincipal(ctx, tok)
			if err != nil {
				return "", 0, "", "", auth.ErrInvalidCredentials
			}
			g := p.Grade
			if g < 1 || g > 6 {
				g = grade
			}
			return p.ID, g, p.Timezone, locale, nil
		}
		if requireAuth {
			return "", 0, "", "", auth.ErrInvalidCredentials
		}
	}
	return "demo-pilot1", grade, "Asia/Jakarta", locale, nil
}

func (h *Handler) RegisterRoutes(_ chi.Router, api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "daily-orbit-today",
		Method:      http.MethodGet,
		Path:        "/api/v1/daily-orbit/today",
		Summary:     "Get today's 10-question Daily Orbit progression path",
		Tags:        []string{"DailyOrbit"},
	}, func(ctx context.Context, in *CommonIn) (*struct{ Body SessionView }, error) {
		id, grade, tz, locale, err := h.resolveUser(ctx, *in, false)
		if err != nil {
			return nil, huma.Error401Unauthorized("unauthenticated")
		}
		session, err := h.svc.GetToday(id, grade, tz, locale)
		if err != nil {
			return nil, huma.Error503ServiceUnavailable("daily orbit unavailable")
		}
		return &struct{ Body SessionView }{Body: session}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "daily-orbit-submit",
		Method:      http.MethodPost,
		Path:        "/api/v1/daily-orbit/submit",
		Summary:     "Submit a single-attempt answer for a Daily Orbit slot",
		Tags:        []string{"DailyOrbit"},
		Security:    []map[string][]string{{"accessCookie": {}}, {"bearerAuth": {}}},
	}, func(ctx context.Context, in *struct {
		CommonIn
		Body struct {
			SlotIndex    int               `json:"slotIndex" minimum:"1" maximum:"10"`
			Answers      map[string]string `json:"answers"`
			TemplateCode string            `json:"templateCode,omitempty"`
			Seed         int64             `json:"seed,omitempty"`
		}
	}) (*struct {
		Body struct {
			Slot    SlotView    `json:"slot"`
			Session SessionView `json:"session"`
		}
	}, error) {
		id, grade, tz, locale, err := h.resolveUser(ctx, in.CommonIn, true)
		if err != nil {
			return nil, huma.Error401Unauthorized("authentication required")
		}
		slot, sess, err := h.svc.SubmitAnswer(ctx, id, grade, tz, locale, in.Body.SlotIndex, in.Body.Answers, in.Body.TemplateCode, in.Body.Seed)
		if err != nil {
			switch {
			case errors.Is(err, ErrAlreadyAnswered), errors.Is(err, ErrQuestionReplaced):
				return nil, huma.Error409Conflict(err.Error())
			case errors.Is(err, ErrOutsideOperatingHours):
				return nil, huma.Error403Forbidden(err.Error())
			case errors.Is(err, ErrInvalidSlot):
				return nil, huma.Error400BadRequest(err.Error())
			default:
				return nil, huma.Error503ServiceUnavailable("daily orbit unavailable")
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
		Security:    []map[string][]string{{"accessCookie": {}}, {"bearerAuth": {}}},
	}, func(ctx context.Context, in *struct {
		CommonIn
		Body struct {
			SlotIndex int `json:"slotIndex" minimum:"1" maximum:"10"`
		}
	}) (*struct {
		Body struct {
			Slot    SlotView    `json:"slot"`
			Session SessionView `json:"session"`
		}
	}, error) {
		id, grade, tz, locale, err := h.resolveUser(ctx, in.CommonIn, true)
		if err != nil {
			return nil, huma.Error401Unauthorized("authentication required")
		}
		slot, sess, err := h.svc.SkipSlot(ctx, id, grade, tz, locale, in.Body.SlotIndex)
		if err != nil {
			if errors.Is(err, ErrAlreadyAnswered) {
				return nil, huma.Error409Conflict(err.Error())
			}
			if errors.Is(err, ErrInvalidSlot) || errors.Is(err, ErrOutsideOperatingHours) {
				return nil, huma.Error400BadRequest(err.Error())
			}
			return nil, huma.Error503ServiceUnavailable("daily orbit unavailable")
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
		Security:    []map[string][]string{{"accessCookie": {}}, {"bearerAuth": {}}},
	}, func(ctx context.Context, in *struct {
		CommonIn
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
		id, grade, tz, locale, err := h.resolveUser(ctx, in.CommonIn, true)
		if err != nil {
			return nil, huma.Error401Unauthorized("authentication required")
		}
		slot, sess, err := h.svc.ChangeQuestion(id, grade, tz, locale, in.Body.SlotIndex, in.Body.Mode)
		if err != nil {
			switch err {
			case ErrOutsideOperatingHours, ErrInvalidSlot, ErrChangeLimitReached, ErrAlreadyAnswered, ErrNoReplacementAvailable:
				return nil, huma.Error400BadRequest(err.Error())
			default:
				return nil, huma.Error503ServiceUnavailable("daily orbit unavailable")
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
		OperationID: "daily-orbit-set-tier",
		Method:      http.MethodPatch,
		Path:        "/api/v1/daily-orbit/tier",
		Summary:     "Switch unlocked Daily Orbit difficulty tier",
		Tags:        []string{"DailyOrbit"},
		Security:    []map[string][]string{{"accessCookie": {}}, {"bearerAuth": {}}},
	}, func(ctx context.Context, in *struct {
		CommonIn
		Body struct {
			Tier Tier `json:"tier" enum:"ELEMENTARY,INTERMEDIATE,ADVANCED,MASTER"`
		}
	}) (*struct{ Body SessionView }, error) {
		id, grade, tz, locale, err := h.resolveUser(ctx, in.CommonIn, true)
		if err != nil {
			return nil, huma.Error401Unauthorized("authentication required")
		}
		sess, err := h.svc.SetTier(id, grade, tz, locale, in.Body.Tier)
		if err != nil {
			if errors.Is(err, ErrTierLocked) {
				return nil, huma.Error403Forbidden(err.Error())
			}
			return nil, huma.Error503ServiceUnavailable("daily orbit unavailable")
		}
		return &struct{ Body SessionView }{Body: sess}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "daily-orbit-set-curricula",
		Method:      http.MethodPatch,
		Path:        "/api/v1/daily-orbit/curricula",
		Summary:     "Update active Daily Orbit curriculum pools",
		Tags:        []string{"DailyOrbit"},
		Security:    []map[string][]string{{"accessCookie": {}}, {"bearerAuth": {}}},
	}, func(ctx context.Context, in *struct {
		CommonIn
		Body struct {
			Curricula []string `json:"curricula" minItems:"1"`
		}
	}) (*struct{ Body SessionView }, error) {
		id, grade, tz, locale, err := h.resolveUser(ctx, in.CommonIn, true)
		if err != nil {
			return nil, huma.Error401Unauthorized("authentication required")
		}
		sess, err := h.svc.SetCurricula(id, grade, tz, locale, in.Body.Curricula)
		if err != nil {
			if errors.Is(err, ErrInvalidCurricula) {
				return nil, huma.Error400BadRequest(err.Error())
			}
			return nil, huma.Error503ServiceUnavailable("daily orbit unavailable")
		}
		return &struct{ Body SessionView }{Body: sess}, nil
	})
}

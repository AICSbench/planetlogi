package astroquiz

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5"

	"github.com/AICSbench/planetlogi/apps/api/internal/mathengine"
	"github.com/AICSbench/planetlogi/apps/api/internal/modules/auth"
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/economy"
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/reqguard"
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/state"
)

const WeeklySupplyPodRewardAC = 5

// maxElapsedSec bounds the server-measured paper timer (24h).
const maxElapsedSec = 24 * 60 * 60

const (
	paperQuestionCount = 10
	// maxTickGapSec caps how much wall-clock time between two server ticks (open/save/submit)
	// counts toward the paper timer, so a tab left open overnight does not add hours.
	maxTickGapSec = 15 * 60
	// DefaultTimezone is used when the caller has no configured timezone.
	DefaultTimezone = "Asia/Jakarta"
)

var (
	ErrPaperNotFound = errors.New("paper not found")
	// ErrAnswerKeyLocked means the caller may not see this paper's answer key yet.
	ErrAnswerKeyLocked = errors.New("answer key is available only after submission")
	// ErrRewardUnavailable means the Supply Pod credit failed; re-submitting retries it.
	ErrRewardUnavailable = errors.New("supply pod could not be credited; please retry")
)

type tzKey struct{}

// WithTimezone attaches the caller's configured IANA timezone to ctx. The weekly Supply Pod
// window and deadline follow it (AGENTS.md guardrail #5).
func WithTimezone(ctx context.Context, tz string) context.Context {
	return context.WithValue(ctx, tzKey{}, tz)
}

func locationFrom(ctx context.Context) *time.Location {
	if tz, _ := ctx.Value(tzKey{}).(string); tz != "" {
		if loc, err := time.LoadLocation(tz); err == nil {
			return loc
		}
	}
	return time.UTC
}

type Category string

const (
	CategoryEssential Category = "ESSENTIAL"
	CategoryTopSchool Category = "TOP_SCHOOL"
	CategoryHOTS      Category = "HOTS"
)

type PaperSummary struct {
	Code           string   `json:"code"`
	Grade          int      `json:"grade"`
	Category       Category `json:"category"`
	Title          string   `json:"title"`
	QuestionCount  int      `json:"questionCount"`
	Status         string   `json:"status"` // NOT_STARTED | IN_PROGRESS | SUBMITTED
	BestScore      int      `json:"bestScore"`
	WeeklyDeadline string   `json:"weeklyDeadline"`
}

type PaperQuestion struct {
	Index          int                         `json:"index"`
	Question       mathengine.RenderedQuestion `json:"question"`
	DraftAnswer    map[string]string           `json:"draftAnswer,omitempty"`
	IsCorrect      *bool                       `json:"isCorrect,omitempty"`
	WorkedSolution *mathengine.WorkedSolution  `json:"workedSolution,omitempty"`
}

type PaperAttemptView struct {
	PaperCode          string          `json:"paperCode"`
	Title              string          `json:"title"`
	Grade              int             `json:"grade"`
	Category           Category        `json:"category"`
	Status             string          `json:"status"` // IN_PROGRESS | SUBMITTED
	ElapsedSec         int             `json:"elapsedSec"`
	Score              int             `json:"score"`
	TotalQuestions     int             `json:"totalQuestions"`
	SupplyPodAwarded   bool            `json:"supplyPodAwarded"`
	SupplyPodClaimedWk bool            `json:"supplyPodClaimedThisWeek"`
	SupplyPodRetryable bool            `json:"supplyPodRetryable"`
	Questions          []PaperQuestion `json:"questions"`
}

type PrintablePaper struct {
	PaperCode string                        `json:"paperCode"`
	Title     string                        `json:"title"`
	Grade     int                           `json:"grade"`
	Category  Category                      `json:"category"`
	Questions []mathengine.RenderedQuestion `json:"questions"`
	// AnswerKeyAvailable tells the client whether requesting includeKey=true will succeed.
	AnswerKeyAvailable bool                `json:"answerKeyAvailable"`
	AnswerKeys         []map[string]string `json:"answerKeys,omitempty"`
}

type paperDef struct {
	code      string
	grade     int
	category  Category
	title     mathengine.LocaleText
	templates []string
}

type attemptState struct {
	status           string
	elapsedSec       int       // server-measured, see tickLocked
	lastTick         time.Time // last open/save/submit while IN_PROGRESS
	drafts           map[int]map[string]string
	draftUpdatedAt   map[int]int64
	score            int
	supplyPodAwarded bool
	rewardClaimed    bool
	claimWeek        string
}

type Service struct {
	mu              sync.Mutex
	ledger          economy.Ledger
	store           state.Store
	loaded          map[string]bool
	papers          []paperDef
	attempts        map[string]map[string]*attemptState // studentID -> paperCode -> state
	weeklySupplyPod map[string]bool                     // studentID:weekKey -> claimed
	weeklyPaper     map[string]string                   // studentID:weekKey -> paper that earned it
	nowClock        func() time.Time
}

func NewService(ledger economy.Ledger) *Service {
	return &Service{
		ledger:          ledger,
		loaded:          make(map[string]bool),
		papers:          buildPapers(),
		attempts:        make(map[string]map[string]*attemptState),
		weeklySupplyPod: make(map[string]bool),
		weeklyPaper:     make(map[string]string),
		nowClock:        time.Now,
	}
}

func NewServiceWithStore(store state.Store, ledger economy.Ledger) *Service {
	svc := NewService(ledger)
	svc.store = store
	return svc
}

type quizState struct {
	Attempts    map[string]attemptStateDTO `json:"attempts"`
	Weekly      map[string]bool            `json:"weekly"`
	WeeklyPaper map[string]string          `json:"weeklyPaper,omitempty"`
}

type attemptStateDTO struct {
	Status           string                    `json:"status"`
	ElapsedSec       int                       `json:"elapsedSec"`
	LastTick         time.Time                 `json:"lastTick,omitempty"`
	Drafts           map[int]map[string]string `json:"drafts"`
	DraftUpdatedAt   map[int]int64             `json:"draftUpdatedAt,omitempty"`
	Score            int                       `json:"score"`
	SupplyPodAwarded bool                      `json:"supplyPodAwarded"`
	RewardClaimed    bool                      `json:"rewardClaimed"`
	ClaimWeek        string                    `json:"claimWeek,omitempty"`
}

func (st *attemptState) toDTO() attemptStateDTO {
	drafts := make(map[int]map[string]string, len(st.drafts))
	for idx, answers := range st.drafts {
		copied := make(map[string]string, len(answers))
		for key, value := range answers {
			copied[key] = value
		}
		drafts[idx] = copied
	}
	draftUpdatedAt := make(map[int]int64, len(st.draftUpdatedAt))
	for idx, at := range st.draftUpdatedAt {
		draftUpdatedAt[idx] = at
	}
	return attemptStateDTO{
		Status: st.status, ElapsedSec: st.elapsedSec, LastTick: st.lastTick, Drafts: drafts,
		DraftUpdatedAt: draftUpdatedAt,
		Score:          st.score, SupplyPodAwarded: st.supplyPodAwarded, RewardClaimed: st.rewardClaimed,
		ClaimWeek: st.claimWeek,
	}
}

func (d attemptStateDTO) toState() *attemptState {
	st := &attemptState{
		status: d.Status, elapsedSec: d.ElapsedSec, lastTick: d.LastTick, drafts: d.Drafts,
		draftUpdatedAt: d.DraftUpdatedAt,
		score:          d.Score, supplyPodAwarded: d.SupplyPodAwarded, rewardClaimed: d.RewardClaimed,
		claimWeek: d.ClaimWeek,
	}
	if st.status == "" {
		st.status = "IN_PROGRESS"
	}
	if st.drafts == nil {
		st.drafts = make(map[int]map[string]string)
	}
	if st.draftUpdatedAt == nil {
		st.draftUpdatedAt = make(map[int]int64)
	}
	return st
}

// isUUID reports whether studentID is a real (PostgreSQL-backed) ID.
func isUUID(s string) bool { return reqguard.IsUUID(s) }

func (s *Service) ensureStudentLoaded(ctx context.Context, studentID string) error {
	if s.store == nil || !isUUID(studentID) {
		return nil
	}
	s.mu.Lock()
	if s.loaded[studentID] {
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()
	var st quizState
	if _, err := s.store.Load(ctx, studentID, "astroquiz", &st); err != nil {
		return err
	}
	s.mu.Lock()
	if s.loaded[studentID] {
		s.mu.Unlock()
		return nil
	}
	if s.attempts[studentID] == nil {
		s.attempts[studentID] = make(map[string]*attemptState)
	}
	for code, dto := range st.Attempts {
		s.attempts[studentID][code] = dto.toState()
	}
	for key, claimed := range st.Weekly {
		if claimed {
			s.weeklySupplyPod[key] = true
		}
	}
	for key, paper := range st.WeeklyPaper {
		s.weeklyPaper[key] = paper
	}
	s.loaded[studentID] = true
	s.mu.Unlock()
	return nil
}

func mergeQuizState(currentRaw []byte, snapshot quizState) (any, error) {
	current := quizState{
		Attempts: make(map[string]attemptStateDTO),
		Weekly:   make(map[string]bool),
	}
	if len(currentRaw) > 0 {
		if err := json.Unmarshal(currentRaw, &current); err != nil {
			return nil, err
		}
	}
	if current.Attempts == nil {
		current.Attempts = make(map[string]attemptStateDTO)
	}
	if current.Weekly == nil {
		current.Weekly = make(map[string]bool)
	}
	if current.WeeklyPaper == nil {
		current.WeeklyPaper = make(map[string]string)
	}
	for key, paper := range snapshot.WeeklyPaper {
		if current.WeeklyPaper[key] == "" {
			current.WeeklyPaper[key] = paper
		}
	}
	for code, snap := range snapshot.Attempts {
		existing, ok := current.Attempts[code]
		if !ok {
			current.Attempts[code] = snap
			continue
		}
		merged := snap
		if existing.Status == "SUBMITTED" {
			// A submitted attempt is one immutable unit: keep its answers,
			// score, and status together even if the snapshot is stale.
			merged.Status = "SUBMITTED"
			merged.Score = existing.Score
			merged.Drafts = existing.Drafts
			if existing.ElapsedSec > merged.ElapsedSec {
				merged.ElapsedSec = existing.ElapsedSec
			}
		} else if snap.Status == "SUBMITTED" {
			// The incoming submission is one immutable unit: keep its
			// answers, score, and status together instead of merging stale
			// in-progress drafts over them.
			if existing.ElapsedSec > merged.ElapsedSec {
				merged.ElapsedSec = existing.ElapsedSec
			}
		} else {
			if snap.Score > merged.Score {
				merged.Score = snap.Score
			}
			if existing.ElapsedSec > merged.ElapsedSec {
				merged.ElapsedSec = existing.ElapsedSec
			}
			drafts := make(map[int]map[string]string, len(existing.Drafts)+len(snap.Drafts))
			draftUpdatedAt := make(map[int]int64, len(existing.DraftUpdatedAt)+len(snap.DraftUpdatedAt))
			for idx, answers := range existing.Drafts {
				drafts[idx] = answers
				draftUpdatedAt[idx] = existing.DraftUpdatedAt[idx]
			}
			for idx, answers := range snap.Drafts {
				if at, ok := existing.DraftUpdatedAt[idx]; !ok || snap.DraftUpdatedAt[idx] >= at {
					drafts[idx] = answers
					draftUpdatedAt[idx] = snap.DraftUpdatedAt[idx]
				}
			}
			merged.Drafts = drafts
			merged.DraftUpdatedAt = draftUpdatedAt
		}
		merged.SupplyPodAwarded = existing.SupplyPodAwarded || snap.SupplyPodAwarded
		merged.RewardClaimed = existing.RewardClaimed || snap.RewardClaimed
		if existing.ClaimWeek != "" {
			merged.ClaimWeek = existing.ClaimWeek
		}
		current.Attempts[code] = merged
	}
	for key, claimed := range snapshot.Weekly {
		current.Weekly[key] = current.Weekly[key] || claimed
	}
	return current, nil
}

func (s *Service) persistStudent(ctx context.Context, studentID string) error {
	if s.store == nil || !isUUID(studentID) {
		return nil
	}
	s.mu.Lock()
	st := quizState{
		Attempts:    make(map[string]attemptStateDTO),
		Weekly:      make(map[string]bool),
		WeeklyPaper: make(map[string]string),
	}
	for code, attempt := range s.attempts[studentID] {
		st.Attempts[code] = attempt.toDTO()
	}
	prefix := studentID + ":"
	for key, claimed := range s.weeklySupplyPod {
		if claimed && strings.HasPrefix(key, prefix) {
			st.Weekly[key] = true
		}
	}
	for key, paper := range s.weeklyPaper {
		if strings.HasPrefix(key, prefix) {
			st.WeeklyPaper[key] = paper
		}
	}
	s.mu.Unlock()
	if ms, ok := s.store.(state.MergeStore); ok {
		return ms.SaveMerged(ctx, studentID, "astroquiz", func(current []byte) (any, error) {
			return mergeQuizState(current, st)
		})
	}
	return s.store.Save(ctx, studentID, "astroquiz", st)
}

func (s *Service) currentWeekKey(ctx context.Context) string {
	yr, wk := s.nowClock().In(locationFrom(ctx)).ISOWeek()
	return fmt.Sprintf("%04d-W%02d", yr, wk)
}

func (s *Service) weeklyDeadlineStr(ctx context.Context) string {
	now := s.nowClock().In(locationFrom(ctx))
	daysUntilSun := (7 - int(now.Weekday())) % 7
	return now.AddDate(0, 0, daysUntilSun).Format("02 Jan 2006")
}

func (s *Service) findPaper(code string) (paperDef, bool) {
	for _, p := range s.papers {
		if p.code == code {
			return p, true
		}
	}
	return paperDef{}, false
}

// mergeDrafts copies client answers into the attempt and drops keys outside
// the 1..10 question range so ungraded data never accumulates.
func mergeDrafts(dst map[int]map[string]string, updatedAt map[int]int64, drafts map[int]map[string]string) {
	mergeDraftsAt(dst, updatedAt, drafts, time.Now().UnixNano())
}

func mergeDraftsAt(dst map[int]map[string]string, updatedAt map[int]int64, drafts map[int]map[string]string, at int64) {
	for k, v := range drafts {
		if k < 1 || k > 10 {
			continue
		}
		copied := make(map[string]string, len(v))
		for key, val := range v {
			copied[key] = val
		}
		dst[k] = copied
		updatedAt[k] = at
	}
}

func gradePaperDrafts(p paperDef, drafts map[int]map[string]string, locale string) int {
	score := 0
	for i := 0; i < 10; i++ {
		idx := i + 1
		tmpl := p.templates[i%len(p.templates)]
		prob, _ := mathengine.GenerateByCode(tmpl, seedForPaperQ(p.code, idx), locale)
		if mathengine.Grade(prob, drafts[idx]).IsCorrect {
			score++
		}
	}
	return score
}

// mergeSubmissionAttempt grades the current durable drafts and freezes them as a
// submitted unit. It runs inside the state store's row lock so a stale instance
// cannot grade an older draft set and overwrite a newer save.
func mergeSubmissionAttempt(dto attemptStateDTO, p paperDef, drafts map[int]map[string]string, locale, claimWeek string, now time.Time) attemptStateDTO {
	if dto.Drafts == nil {
		dto.Drafts = make(map[int]map[string]string)
	}
	if dto.DraftUpdatedAt == nil {
		dto.DraftUpdatedAt = make(map[int]int64)
	}
	if dto.Status != "SUBMITTED" {
		if !dto.LastTick.IsZero() {
			gap := int(now.Sub(dto.LastTick).Seconds())
			if gap > maxTickGapSec {
				gap = maxTickGapSec
			}
			if gap > 0 {
				dto.ElapsedSec += gap
			}
			if dto.ElapsedSec > maxElapsedSec {
				dto.ElapsedSec = maxElapsedSec
			}
		}
		dto.LastTick = now
		mergeDraftsAt(dto.Drafts, dto.DraftUpdatedAt, drafts, now.UnixNano())
		dto.Score = gradePaperDrafts(p, dto.Drafts, locale)
		dto.Status = "SUBMITTED"
	}
	if dto.ClaimWeek == "" {
		dto.ClaimWeek = claimWeek
	}
	return dto
}

func (s *Service) submitAtomic(ctx context.Context, studentID string, p paperDef, drafts map[int]map[string]string, locale, claimWeek string) (attemptStateDTO, map[string]bool, map[string]string, error) {
	ms, ok := s.store.(state.MergeStore)
	if !ok {
		return attemptStateDTO{}, nil, nil, fmt.Errorf("astroquiz store does not support atomic merges")
	}
	s.mu.Lock()
	snapshotWeekly := make(map[string]bool)
	prefix := studentID + ":"
	for key, claimed := range s.weeklySupplyPod {
		if claimed && strings.HasPrefix(key, prefix) {
			snapshotWeekly[key] = true
		}
	}
	snapshotWeeklyPaper := make(map[string]string)
	for key, paper := range s.weeklyPaper {
		if strings.HasPrefix(key, prefix) {
			snapshotWeeklyPaper[key] = paper
		}
	}
	var localAwarded bool
	var localClaimWeek string
	if attempts := s.attempts[studentID]; attempts != nil {
		if local := attempts[p.code]; local != nil {
			localAwarded = local.supplyPodAwarded
			localClaimWeek = local.claimWeek
		}
	}
	s.mu.Unlock()

	now := s.nowClock()
	var submitted attemptStateDTO
	var mergedWeekly map[string]bool
	var mergedWeeklyPaper map[string]string
	err := ms.SaveMerged(ctx, studentID, "astroquiz", func(currentRaw []byte) (any, error) {
		current := quizState{
			Attempts:    make(map[string]attemptStateDTO),
			Weekly:      make(map[string]bool),
			WeeklyPaper: make(map[string]string),
		}
		if len(currentRaw) > 0 {
			if err := json.Unmarshal(currentRaw, &current); err != nil {
				return nil, err
			}
		}
		if current.Attempts == nil {
			current.Attempts = make(map[string]attemptStateDTO)
		}
		if current.Weekly == nil {
			current.Weekly = make(map[string]bool)
		}
		if current.WeeklyPaper == nil {
			current.WeeklyPaper = make(map[string]string)
		}
		for key, claimed := range snapshotWeekly {
			current.Weekly[key] = current.Weekly[key] || claimed
		}
		for key, paper := range snapshotWeeklyPaper {
			if current.WeeklyPaper[key] == "" {
				current.WeeklyPaper[key] = paper
			}
		}
		submitted = mergeSubmissionAttempt(current.Attempts[p.code], p, drafts, locale, claimWeek, now)
		if localAwarded && localClaimWeek != "" && submitted.ClaimWeek == localClaimWeek {
			// The ledger already credited this paper before a failed final
			// write; keep its badge and weekly claim on retry.
			submitted.SupplyPodAwarded = true
			submitted.RewardClaimed = true
			current.Weekly[studentID+":"+localClaimWeek] = true
			current.WeeklyPaper[studentID+":"+localClaimWeek] = p.code
		}
		if submitted.ClaimWeek != "" {
			wkKey := studentID + ":" + submitted.ClaimWeek
			if !current.Weekly[wkKey] && current.WeeklyPaper[wkKey] == "" {
				current.WeeklyPaper[wkKey] = p.code
			}
		}
		current.Attempts[p.code] = submitted
		mergedWeekly = make(map[string]bool, len(current.Weekly))
		for key, claimed := range current.Weekly {
			mergedWeekly[key] = claimed
		}
		mergedWeeklyPaper = make(map[string]string, len(current.WeeklyPaper))
		for key, paper := range current.WeeklyPaper {
			mergedWeeklyPaper[key] = paper
		}
		return current, nil
	})
	if err != nil {
		return attemptStateDTO{}, nil, nil, err
	}
	return submitted, mergedWeekly, mergedWeeklyPaper, nil
}

func (s *Service) submitLocal(ctx context.Context, studentID string, p paperDef, drafts map[int]map[string]string, locale, claimWeek string) (attemptStateDTO, error) {
	s.mu.Lock()
	st := s.getOrCreateAttemptLocked(studentID, p.code)
	if st.status != "SUBMITTED" {
		s.tickLocked(st)
		mergeDrafts(st.drafts, st.draftUpdatedAt, drafts)
		st.score = gradePaperDrafts(p, st.drafts, locale)
		st.status = "SUBMITTED"
	}
	if st.claimWeek == "" {
		st.claimWeek = claimWeek
	}
	dto := st.toDTO()
	s.mu.Unlock()
	if err := s.persistStudent(ctx, studentID); err != nil {
		return attemptStateDTO{}, err
	}
	return dto, nil
}

func seedForPaperQ(code string, idx int) int64 {
	var h int64 = 900
	for _, ch := range code {
		h = h*37 + int64(ch)
	}
	if h < 0 {
		h = -h
	}
	return h%100000 + int64(idx*29)
}

func (s *Service) ListPapers(ctx context.Context, studentID string, grade int, category, locale string) ([]PaperSummary, bool, error) {
	if err := s.ensureStudentLoaded(ctx, studentID); err != nil {
		return nil, false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	wkKey := studentID + ":" + s.currentWeekKey(ctx)
	claimed := s.weeklySupplyPod[wkKey]
	deadline := s.weeklyDeadlineStr(ctx)

	out := make([]PaperSummary, 0)
	for _, p := range s.papers {
		if grade > 0 && p.grade != grade {
			continue
		}
		if category != "" && category != "ALL" && string(p.category) != category {
			continue
		}
		status := "NOT_STARTED"
		score := 0
		if stMap := s.attempts[studentID]; stMap != nil {
			if st := stMap[p.code]; st != nil {
				status = st.status
				score = st.score
			}
		}
		out = append(out, PaperSummary{
			Code:           p.code,
			Grade:          p.grade,
			Category:       p.category,
			Title:          p.title.For(locale),
			QuestionCount:  10,
			Status:         status,
			BestScore:      score,
			WeeklyDeadline: deadline,
		})
	}
	return out, claimed, nil
}

func (s *Service) OpenPaper(ctx context.Context, studentID, paperCode, locale string) (PaperAttemptView, error) {
	if err := s.ensureStudentLoaded(ctx, studentID); err != nil {
		return PaperAttemptView{}, err
	}
	s.mu.Lock()
	p, ok := s.findPaper(paperCode)
	if !ok {
		s.mu.Unlock()
		return PaperAttemptView{}, fmt.Errorf("%w: %q", ErrPaperNotFound, paperCode)
	}
	st := s.getOrCreateAttemptLocked(studentID, paperCode)
	if st.status == "IN_PROGRESS" {
		// Opening/resuming starts a new timing window; time away is not counted.
		st.lastTick = s.nowClock()
	}
	view := s.renderAttemptLocked(ctx, studentID, p, st, locale)
	s.mu.Unlock()
	if err := s.persistStudent(ctx, studentID); err != nil {
		return PaperAttemptView{}, err
	}
	return view, nil
}

func (s *Service) SavePaper(ctx context.Context, studentID, paperCode, locale string, elapsedSec int, drafts map[int]map[string]string) (PaperAttemptView, error) {
	_ = elapsedSec // display-only on the client; the server measures elapsed time
	if err := reqguard.ValidateDrafts(drafts); err != nil {
		return PaperAttemptView{}, err
	}
	if err := s.ensureStudentLoaded(ctx, studentID); err != nil {
		return PaperAttemptView{}, err
	}
	s.mu.Lock()
	p, ok := s.findPaper(paperCode)
	if !ok {
		s.mu.Unlock()
		return PaperAttemptView{}, fmt.Errorf("%w: %q", ErrPaperNotFound, paperCode)
	}
	st := s.getOrCreateAttemptLocked(studentID, paperCode)
	if st.status == "SUBMITTED" {
		view := s.renderAttemptLocked(ctx, studentID, p, st, locale)
		s.mu.Unlock()
		return view, nil
	}
	s.tickLocked(st)
	mergeDrafts(st.drafts, st.draftUpdatedAt, drafts)
	view := s.renderAttemptLocked(ctx, studentID, p, st, locale)
	s.mu.Unlock()
	if err := s.persistStudent(ctx, studentID); err != nil {
		return PaperAttemptView{}, err
	}
	return view, nil
}

// SubmitPaper grades all 10 questions at once, reveals worked solutions, and awards
// 1 Weekly Supply Pod (+5 AC) max once per ISO week.
func (s *Service) SubmitPaper(ctx context.Context, studentID, paperCode, locale string, elapsedSec int, drafts map[int]map[string]string) (PaperAttemptView, error) {
	_ = elapsedSec // display-only on the client; the server measures elapsed time
	if err := reqguard.ValidateDrafts(drafts); err != nil {
		return PaperAttemptView{}, err
	}
	if err := s.ensureStudentLoaded(ctx, studentID); err != nil {
		return PaperAttemptView{}, err
	}
	p, ok := s.findPaper(paperCode)
	if !ok {
		return PaperAttemptView{}, fmt.Errorf("%w: %q", ErrPaperNotFound, paperCode)
	}
	claimWeek := s.currentWeekKey(ctx)

	var submitted attemptStateDTO
	if s.store != nil && isUUID(studentID) {
		if _, ok := s.store.(state.MergeStore); ok {
			var mergedWeekly map[string]bool
			var mergedWeeklyPaper map[string]string
			var err error
			submitted, mergedWeekly, mergedWeeklyPaper, err = s.submitAtomic(ctx, studentID, p, drafts, locale, claimWeek)
			if err != nil {
				return PaperAttemptView{}, err
			}
			s.mu.Lock()
			if s.attempts[studentID] == nil {
				s.attempts[studentID] = make(map[string]*attemptState)
			}
			s.attempts[studentID][p.code] = submitted.toState()
			for key, claimed := range mergedWeekly {
				if claimed {
					s.weeklySupplyPod[key] = true
				}
			}
			for key, paper := range mergedWeeklyPaper {
				s.weeklyPaper[key] = paper
			}
			s.mu.Unlock()
		} else if _, err := s.submitLocal(ctx, studentID, p, drafts, locale, claimWeek); err != nil {
			return PaperAttemptView{}, err
		}
	} else if _, err := s.submitLocal(ctx, studentID, p, drafts, locale, claimWeek); err != nil {
		return PaperAttemptView{}, err
	}

	// The submitted attempt is durable now. Settle the weekly pod from the
	// canonical claim week and the paper that reserved it.
	s.mu.Lock()
	st := s.getOrCreateAttemptLocked(studentID, p.code)
	claimWeek = st.claimWeek
	wkKey := studentID + ":" + claimWeek
	weeklyClaimed := s.weeklySupplyPod[wkKey]
	weeklyPaper := s.weeklyPaper[wkKey]
	needsReward := !st.rewardClaimed || (weeklyPaper == p.code && !st.supplyPodAwarded)
	view := s.renderAttemptLocked(ctx, studentID, p, st, locale)
	s.mu.Unlock()

	if needsReward {
		switch {
		case weeklyPaper != "" && weeklyPaper != p.code:
			// Another paper earned this week; this paper may submit but
			// must not receive the pod badge.
			s.mu.Lock()
			st = s.getOrCreateAttemptLocked(studentID, p.code)
			st.rewardClaimed = true
			view = s.renderAttemptLocked(ctx, studentID, p, st, locale)
			s.mu.Unlock()
			if err := s.persistStudent(ctx, studentID); err != nil {
				return PaperAttemptView{}, err
			}
		case weeklyClaimed:
			// The credit is already settled. Only the paper that reserved
			// the week may carry the badge.
			s.mu.Lock()
			st = s.getOrCreateAttemptLocked(studentID, p.code)
			st.rewardClaimed = true
			if weeklyPaper == p.code {
				st.supplyPodAwarded = true
			}
			view = s.renderAttemptLocked(ctx, studentID, p, st, locale)
			s.mu.Unlock()
			if err := s.persistStudent(ctx, studentID); err != nil {
				return PaperAttemptView{}, err
			}
		default:
			if s.ledger == nil {
				s.mu.Lock()
				st = s.getOrCreateAttemptLocked(studentID, p.code)
				s.weeklySupplyPod[wkKey] = true
				s.weeklyPaper[wkKey] = p.code
				st.rewardClaimed = true
				st.supplyPodAwarded = true
				view = s.renderAttemptLocked(ctx, studentID, p, st, locale)
				s.mu.Unlock()
				if err := s.persistStudent(ctx, studentID); err != nil {
					return PaperAttemptView{}, err
				}
			} else {
				if _, err := s.ledger.Apply(ctx, economy.MutationRequest{
					StudentID:      studentID,
					Currency:       economy.CurrencyAC,
					Delta:          WeeklySupplyPodRewardAC,
					SourceType:     "ASTRO_QUIZ_SUPPLY_POD",
					SourceID:       p.code,
					IdempotencyKey: fmt.Sprintf("supply_pod:%s:%s", studentID, claimWeek),
				}); err != nil {
					return PaperAttemptView{}, fmt.Errorf("%w: %v", ErrRewardUnavailable, err)
				}
				s.mu.Lock()
				st = s.getOrCreateAttemptLocked(studentID, p.code)
				s.weeklySupplyPod[wkKey] = true
				s.weeklyPaper[wkKey] = p.code
				st.rewardClaimed = true
				st.supplyPodAwarded = true
				view = s.renderAttemptLocked(ctx, studentID, p, st, locale)
				s.mu.Unlock()
				if err := s.persistStudent(ctx, studentID); err != nil {
					return PaperAttemptView{}, err
				}
			}
		}
	}
	return view, nil
}

// GetPrintablePaper returns the A4 worksheet. Answer keys are included only when requested and
// allowed: adults (adult=true, for marking printed worksheets) always, a student only after
// submitting this paper.
func (s *Service) GetPrintablePaper(ctx context.Context, studentID, paperCode, locale string, includeKey, adult bool) (PrintablePaper, error) {
	if err := s.ensureStudentLoaded(ctx, studentID); err != nil {
		return PrintablePaper{}, err
	}
	p, ok := s.findPaper(paperCode)
	if !ok {
		return PrintablePaper{}, fmt.Errorf("%w: %q", ErrPaperNotFound, paperCode)
	}
	available := adult
	if !available {
		s.mu.Lock()
		st := s.attempts[studentID][paperCode]
		available = st != nil && st.status == "SUBMITTED"
		s.mu.Unlock()
	}
	if includeKey && !available {
		return PrintablePaper{}, ErrAnswerKeyLocked
	}
	qs := make([]mathengine.RenderedQuestion, paperQuestionCount)
	var keys []map[string]string
	if includeKey {
		keys = make([]map[string]string, paperQuestionCount)
	}
	for i := 0; i < 10; i++ {
		idx := i + 1
		tmpl := p.templates[i%len(p.templates)]
		prob, _ := mathengine.GenerateByCode(tmpl, seedForPaperQ(p.code, idx), locale)
		qs[i] = prob.Question
		if includeKey {
			keys[i] = prob.Solution.CorrectAnswers
		}
	}
	return PrintablePaper{
		PaperCode:          p.code,
		Title:              p.title.For(locale),
		Grade:              p.grade,
		Category:           p.category,
		Questions:          qs,
		AnswerKeys:         keys,
		AnswerKeyAvailable: available,
	}, nil
}

func (s *Service) getOrCreateAttemptLocked(studentID, paperCode string) *attemptState {
	if s.attempts[studentID] == nil {
		s.attempts[studentID] = make(map[string]*attemptState)
	}
	st := s.attempts[studentID][paperCode]
	if st == nil {
		st = &attemptState{
			status:         "IN_PROGRESS",
			drafts:         make(map[int]map[string]string),
			draftUpdatedAt: make(map[int]int64),
		}
		s.attempts[studentID][paperCode] = st
	}
	return st
}

// tickLocked adds the server-measured time since the previous tick (capped per gap and in
// total) to the attempt. The client-reported elapsedSec is never trusted.
func (s *Service) tickLocked(st *attemptState) {
	now := s.nowClock()
	if st.status == "IN_PROGRESS" && !st.lastTick.IsZero() {
		gap := int(now.Sub(st.lastTick).Seconds())
		if gap > maxTickGapSec {
			gap = maxTickGapSec
		}
		if gap > 0 {
			st.elapsedSec += gap
		}
		if st.elapsedSec > maxElapsedSec {
			st.elapsedSec = maxElapsedSec
		}
	}
	st.lastTick = now
}

func (s *Service) renderAttemptLocked(ctx context.Context, studentID string, p paperDef, st *attemptState, locale string) PaperAttemptView {
	qs := make([]PaperQuestion, 10)
	for i := 0; i < 10; i++ {
		idx := i + 1
		tmpl := p.templates[i%len(p.templates)]
		prob, _ := mathengine.GenerateByCode(tmpl, seedForPaperQ(p.code, idx), locale)
		pq := PaperQuestion{
			Index:       idx,
			Question:    prob.Question,
			DraftAnswer: st.drafts[idx],
		}
		// Solutions stay strictly hidden until full paper SUBMIT!
		if st.status == "SUBMITTED" {
			ok := mathengine.Grade(prob, st.drafts[idx]).IsCorrect
			pq.IsCorrect = &ok
			sol := prob.Solution
			pq.WorkedSolution = &sol
		}
		qs[i] = pq
	}

	retryable := st.status == "SUBMITTED" && !st.supplyPodAwarded
	if retryable && st.claimWeek != "" {
		wkKey := studentID + ":" + st.claimWeek
		owner := s.weeklyPaper[wkKey]
		if owner != "" && owner != p.code {
			retryable = false
		} else if owner == "" && s.weeklySupplyPod[wkKey] {
			retryable = false
		}
	}
	return PaperAttemptView{
		PaperCode:          p.code,
		Title:              p.title.For(locale),
		Grade:              p.grade,
		Category:           p.category,
		Status:             st.status,
		ElapsedSec:         st.elapsedSec,
		Score:              st.score,
		TotalQuestions:     10,
		SupplyPodAwarded:   st.supplyPodAwarded,
		SupplyPodClaimedWk: s.weeklySupplyPod[studentID+":"+s.currentWeekKey(ctx)],
		SupplyPodRetryable: retryable,
		Questions:          qs,
	}
}

func buildPapers() []paperDef {
	return []paperDef{
		{code: "G1-ESS-01", grade: 1, category: CategoryEssential, title: mathengine.LocaleText{EN: "Grade 1 Essential Revision: Numbers & Operations", ID: "Revisi Esensial Kelas 1: Bilangan & Operasi"}, templates: []string{"g1_add_within_20", "g1_sub_within_20", "g1_place_value_tens_ones", "g1_pattern_hots"}},
		{code: "G1-TOP-01", grade: 1, category: CategoryTopSchool, title: mathengine.LocaleText{EN: "Grade 1 Top School Year-End Paper (Math-F1)", ID: "Ujian Akhir Tahun Sekolah Unggulan Kelas 1 (Math-F1)"}, templates: []string{"g1_add_within_20", "g1_place_value_tens_ones", "g1_sub_within_20", "g1_pattern_hots"}},
		{code: "G1-HOTS-01", grade: 1, category: CategoryHOTS, title: mathengine.LocaleText{EN: "Grade 1 HOTS Heuristic Paper (Ages 6–9)", ID: "Kertas HOTS Heuristik Kelas 1 (Usia 6–9)"}, templates: []string{"g1_pattern_hots", "g1_add_within_20", "g1_place_value_tens_ones"}},

		{code: "G2-ESS-01", grade: 2, category: CategoryEssential, title: mathengine.LocaleText{EN: "Grade 2 Essential Revision: Addition & Multiplication", ID: "Revisi Esensial Kelas 2: Penjumlahan & Perkalian"}, templates: []string{"g2_add_within_1000", "g2_mul_tables_2_5_10", "g2_unit_fraction_mcq", "g2_bar_model_compare"}},
		{code: "G2-TOP-01", grade: 2, category: CategoryTopSchool, title: mathengine.LocaleText{EN: "Grade 2 Top School Year-End Paper (Math-F2)", ID: "Ujian Akhir Tahun Sekolah Unggulan Kelas 2 (Math-F2)"}, templates: []string{"g2_add_within_1000", "g2_bar_model_compare", "g2_mul_tables_2_5_10", "g2_unit_fraction_mcq"}},
		{code: "G2-HOTS-01", grade: 2, category: CategoryHOTS, title: mathengine.LocaleText{EN: "Grade 2 HOTS Heuristic Paper: Draw Models", ID: "Kertas HOTS Kelas 2: Menggambar Model Batang"}, templates: []string{"g2_bar_model_compare", "g1_pattern_hots", "g2_mul_tables_2_5_10"}},

		{code: "G3-ESS-01", grade: 3, category: CategoryEssential, title: mathengine.LocaleText{EN: "Grade 3 Essential Revision: Area, Division & Fractions", ID: "Revisi Esensial Kelas 3: Luas, Pembagian & Pecahan"}, templates: []string{"g3_mul_6_to_9", "g3_div_with_remainder", "g3_area_perimeter_rect", "g3_equiv_fraction"}},
		{code: "G3-TOP-01", grade: 3, category: CategoryTopSchool, title: mathengine.LocaleText{EN: "Grade 3 Top School Year-End Paper (Math-I1)", ID: "Ujian Akhir Tahun Sekolah Unggulan Kelas 3 (Math-I1)"}, templates: []string{"g3_area_perimeter_rect", "g3_div_with_remainder", "g3_equiv_fraction", "g3_mul_6_to_9"}},
		{code: "G3-HOTS-01", grade: 3, category: CategoryHOTS, title: mathengine.LocaleText{EN: "Grade 3 HOTS Heuristic Paper (Ages 8–11)", ID: "Kertas HOTS Heuristik Kelas 3 (Usia 8–11)"}, templates: []string{"g3_div_with_remainder", "g3_area_perimeter_rect", "g2_bar_model_compare"}},

		{code: "G4-ESS-01", grade: 4, category: CategoryEssential, title: mathengine.LocaleText{EN: "Grade 4 Essential Revision: LCM, Decimals & Fractions", ID: "Revisi Esensial Kelas 4: KPK, Desimal & Pecahan"}, templates: []string{"g4_lcm_gcf", "g4_decimal_add_sub", "g4_fraction_of_set", "g4_work_backwards_hots"}},
		{code: "G4-TOP-01", grade: 4, category: CategoryTopSchool, title: mathengine.LocaleText{EN: "Grade 4 Top School Year-End Paper (Math-I2)", ID: "Ujian Akhir Tahun Sekolah Unggulan Kelas 4 (Math-I2)"}, templates: []string{"g4_decimal_add_sub", "g4_lcm_gcf", "g4_fraction_of_set", "g4_work_backwards_hots"}},
		{code: "G4-HOTS-01", grade: 4, category: CategoryHOTS, title: mathengine.LocaleText{EN: "Grade 4 HOTS Heuristic Paper: Work Backwards", ID: "Kertas HOTS Kelas 4: Strategi Bekerja Mundur"}, templates: []string{"g4_work_backwards_hots", "g4_lcm_gcf", "g4_fraction_of_set"}},

		{code: "G5-ESS-01", grade: 5, category: CategoryEssential, title: mathengine.LocaleText{EN: "Grade 5 Essential Revision: Percentage, Ratio & Volume", ID: "Revisi Esensial Kelas 5: Persentase, Rasio & Volume"}, templates: []string{"g5_percentage_discount", "g5_ratio_word_problem", "g5_volume_cuboid", "g5_average_mean"}},
		{code: "G5-TOP-01", grade: 5, category: CategoryTopSchool, title: mathengine.LocaleText{EN: "Grade 5 Top School Year-End Paper (Math-A1)", ID: "Ujian Akhir Tahun Sekolah Unggulan Kelas 5 (Math-A1)"}, templates: []string{"g5_ratio_word_problem", "g5_percentage_discount", "g5_volume_cuboid", "g5_average_mean"}},
		{code: "G5-HOTS-01", grade: 5, category: CategoryHOTS, title: mathengine.LocaleText{EN: "Grade 5 HOTS Heuristic Paper: Ratio & Models", ID: "Kertas HOTS Kelas 5: Rasio & Model Batang"}, templates: []string{"g5_ratio_word_problem", "g4_work_backwards_hots", "g5_percentage_discount"}},

		{code: "G6-ESS-01", grade: 6, category: CategoryEssential, title: mathengine.LocaleText{EN: "Grade 6 Essential Revision: Algebra, Speed & Circles", ID: "Revisi Esensial Kelas 6: Aljabar, Kecepatan & Lingkaran"}, templates: []string{"g6_algebra_linear", "g6_speed_distance_time", "g6_circle_circumference", "g6_supposition_hots"}},
		{code: "G6-TOP-01", grade: 6, category: CategoryTopSchool, title: mathengine.LocaleText{EN: "Grade 6 Final Exam Mock Paper (Math-M1)", ID: "Tryout Ujian Akhir SD Kelas 6 (Math-M1)"}, templates: []string{"g6_algebra_linear", "g6_speed_distance_time", "g6_circle_circumference", "g6_supposition_hots"}},
		{code: "G6-HOTS-01", grade: 6, category: CategoryHOTS, title: mathengine.LocaleText{EN: "Grade 6 HOTS Heuristic Paper: Supposition Method", ID: "Kertas HOTS Kelas 6: Metode Pengandaian"}, templates: []string{"g6_supposition_hots", "g6_algebra_linear", "g6_speed_distance_time"}},
	}
}

type Handler struct {
	svc     *Service
	authSvc *auth.Service
}

func NewHandler(svc *Service, authSvc *auth.Service) *Handler {
	return &Handler{svc: svc, authSvc: authSvc}
}

type AuthIn struct {
	CookieAccess  string `cookie:"pl_access"`
	Authorization string `header:"Authorization"`
	Locale        string `query:"locale" default:"en"`
}

var authSecurity = []map[string][]string{{"accessCookie": {}}, {"bearerAuth": {}}}

// resolve returns the caller, a ctx carrying their timezone, and the locale. Attempt-changing
// calls (open/save/submit) require a student account.
func (h *Handler) resolve(ctx context.Context, in AuthIn, mutating bool) (auth.Principal, context.Context, string, error) {
	locale := in.Locale
	if locale != "id" {
		locale = "en"
	}
	if h.authSvc == nil {
		p := auth.Principal{ID: "demo-pilot1", Role: auth.RoleStudent, Timezone: DefaultTimezone}
		return p, WithTimezone(ctx, p.Timezone), locale, nil
	}
	tok := reqguard.Token(in.CookieAccess, in.Authorization)
	if tok == "" {
		return auth.Principal{}, ctx, locale, auth.ErrInvalidCredentials
	}
	p, err := h.authSvc.ResolvePrincipal(ctx, tok)
	if err != nil {
		return auth.Principal{}, ctx, locale, auth.ErrInvalidCredentials
	}
	if mutating && p.Role != auth.RoleStudent {
		return auth.Principal{}, ctx, locale, auth.ErrForbidden
	}
	tz := p.Timezone
	if tz == "" {
		tz = DefaultTimezone
	}
	return *p, WithTimezone(ctx, tz), locale, nil
}

// toHTTPError maps service errors to responses without echoing internal (e.g. database) errors.
func toHTTPError(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, auth.ErrInvalidCredentials):
		return huma.Error401Unauthorized("authentication required")
	case errors.Is(err, auth.ErrForbidden):
		return huma.Error403Forbidden("student account required")
	case errors.Is(err, ErrAnswerKeyLocked):
		return huma.Error403Forbidden(err.Error())
	case errors.Is(err, ErrPaperNotFound):
		return huma.Error404NotFound(err.Error())
	case errors.Is(err, reqguard.ErrInvalidAnswers):
		return huma.Error400BadRequest(err.Error())
	case errors.Is(err, ErrRewardUnavailable):
		slog.WarnContext(ctx, "astroquiz reward credit failed", "err", err)
		return huma.Error503ServiceUnavailable(ErrRewardUnavailable.Error())
	default:
		slog.ErrorContext(ctx, "astroquiz request failed", "err", err)
		return huma.Error500InternalServerError("internal error")
	}
}

func (h *Handler) RegisterRoutes(_ chi.Router, api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "astroquiz-list",
		Method:      http.MethodGet,
		Path:        "/api/v1/astro-quiz/papers",
		Summary:     "List AstroQuiz papers across Essential, Top School, and HOTS categories",
		Tags:        []string{"AstroQuiz"},
		Security:    authSecurity,
	}, func(ctx context.Context, in *struct {
		AuthIn
		Grade    int    `query:"grade" default:"1"`
		Category string `query:"category" default:"ALL"`
	}) (*struct {
		Body struct {
			Papers                   []PaperSummary `json:"papers"`
			SupplyPodClaimedThisWeek bool           `json:"supplyPodClaimedThisWeek"`
		}
	}, error) {
		p, ctx, locale, err := h.resolve(ctx, in.AuthIn, false)
		if err != nil {
			return nil, toHTTPError(ctx, err)
		}
		id := p.ID
		papers, claimed, err := h.svc.ListPapers(ctx, id, in.Grade, in.Category, locale)
		if err != nil {
			return nil, toHTTPError(ctx, err)
		}
		out := &struct {
			Body struct {
				Papers                   []PaperSummary `json:"papers"`
				SupplyPodClaimedThisWeek bool           `json:"supplyPodClaimedThisWeek"`
			}
		}{}
		out.Body.Papers = papers
		out.Body.SupplyPodClaimedThisWeek = claimed
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "astroquiz-open",
		Method:      http.MethodGet,
		Path:        "/api/v1/astro-quiz/papers/{code}",
		Summary:     "Open or resume an AstroQuiz paper attempt",
		Tags:        []string{"AstroQuiz"},
		Security:    authSecurity,
	}, func(ctx context.Context, in *struct {
		AuthIn
		Code string `path:"code"`
	}) (*struct{ Body PaperAttemptView }, error) {
		p, ctx, locale, err := h.resolve(ctx, in.AuthIn, true)
		if err != nil {
			return nil, toHTTPError(ctx, err)
		}
		id := p.ID
		view, err := h.svc.OpenPaper(ctx, id, in.Code, locale)
		if err != nil {
			return nil, toHTTPError(ctx, err)
		}
		return &struct{ Body PaperAttemptView }{Body: view}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "astroquiz-save",
		Method:      http.MethodPost,
		Path:        "/api/v1/astro-quiz/papers/{code}/save",
		Summary:     "Save in-progress AstroQuiz answers and elapsed timer",
		Tags:        []string{"AstroQuiz"},
		Security:    authSecurity,
	}, func(ctx context.Context, in *struct {
		AuthIn
		Code string `path:"code"`
		Body struct {
			ElapsedSec int                       `json:"elapsedSec"`
			Drafts     map[int]map[string]string `json:"drafts"`
		}
	}) (*struct{ Body PaperAttemptView }, error) {
		p, ctx, locale, err := h.resolve(ctx, in.AuthIn, true)
		if err != nil {
			return nil, toHTTPError(ctx, err)
		}
		id := p.ID
		view, err := h.svc.SavePaper(ctx, id, in.Code, locale, in.Body.ElapsedSec, in.Body.Drafts)
		if err != nil {
			return nil, toHTTPError(ctx, err)
		}
		return &struct{ Body PaperAttemptView }{Body: view}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "astroquiz-submit",
		Method:      http.MethodPost,
		Path:        "/api/v1/astro-quiz/papers/{code}/submit",
		Summary:     "Submit full AstroQuiz paper to reveal solutions and claim Weekly Supply Pod",
		Tags:        []string{"AstroQuiz"},
		Security:    authSecurity,
	}, func(ctx context.Context, in *struct {
		AuthIn
		Code string `path:"code"`
		Body struct {
			ElapsedSec int                       `json:"elapsedSec"`
			Drafts     map[int]map[string]string `json:"drafts"`
		}
	}) (*struct{ Body PaperAttemptView }, error) {
		p, ctx, locale, err := h.resolve(ctx, in.AuthIn, true)
		if err != nil {
			return nil, toHTTPError(ctx, err)
		}
		id := p.ID
		view, err := h.svc.SubmitPaper(ctx, id, in.Code, locale, in.Body.ElapsedSec, in.Body.Drafts)
		if err != nil {
			return nil, toHTTPError(ctx, err)
		}
		return &struct{ Body PaperAttemptView }{Body: view}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "astroquiz-print",
		Method:      http.MethodGet,
		Path:        "/api/v1/astro-quiz/papers/{code}/print",
		Summary:     "Get ink-friendly A4 printable payload for an AstroQuiz paper",
		Tags:        []string{"AstroQuiz"},
		Security:    authSecurity,
	}, func(ctx context.Context, in *struct {
		AuthIn
		Code       string `path:"code"`
		IncludeKey bool   `query:"includeKey" default:"false"`
	}) (*struct{ Body PrintablePaper }, error) {
		p, ctx, locale, err := h.resolve(ctx, in.AuthIn, false)
		if err != nil {
			return nil, toHTTPError(ctx, err)
		}
		id := p.ID
		view, err := h.svc.GetPrintablePaper(ctx, id, in.Code, locale, in.IncludeKey,
			auth.HasAllowedRole(p.Role, auth.RoleParent, auth.RoleTeacher, auth.RoleSchoolAdmin, auth.RoleAdmin, auth.RoleContentEditor))
		if err != nil {
			return nil, toHTTPError(ctx, err)
		}
		return &struct{ Body PrintablePaper }{Body: view}, nil
	})
}

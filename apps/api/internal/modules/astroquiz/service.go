package astroquiz

import (
	"context"
	"fmt"
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

// maxElapsedSec bounds the client-reported paper timer (24h).
const maxElapsedSec = 24 * 60 * 60

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
	Questions          []PaperQuestion `json:"questions"`
}

type PrintablePaper struct {
	PaperCode  string                        `json:"paperCode"`
	Title      string                        `json:"title"`
	Grade      int                           `json:"grade"`
	Category   Category                      `json:"category"`
	Questions  []mathengine.RenderedQuestion `json:"questions"`
	AnswerKeys []map[string]string           `json:"answerKeys"`
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
	elapsedSec       int
	drafts           map[int]map[string]string
	score            int
	supplyPodAwarded bool
	rewardClaimed    bool
}

type Service struct {
	mu              sync.Mutex
	ledger          economy.Ledger
	store           state.Store
	loaded          map[string]bool
	papers          []paperDef
	attempts        map[string]map[string]*attemptState // studentID -> paperCode -> state
	weeklySupplyPod map[string]bool                     // studentID:weekKey -> claimed
	nowClock        func() time.Time
}

func NewService(ledger economy.Ledger) *Service {
	return &Service{
		ledger:          ledger,
		loaded:          make(map[string]bool),
		papers:          buildPapers(),
		attempts:        make(map[string]map[string]*attemptState),
		weeklySupplyPod: make(map[string]bool),
		nowClock:        time.Now,
	}
}

func NewServiceWithStore(store state.Store, ledger economy.Ledger) *Service {
	svc := NewService(ledger)
	svc.store = store
	return svc
}

type quizState struct {
	Attempts map[string]attemptStateDTO `json:"attempts"`
	Weekly   map[string]bool            `json:"weekly"`
}

type attemptStateDTO struct {
	Status           string                    `json:"status"`
	ElapsedSec       int                       `json:"elapsedSec"`
	Drafts           map[int]map[string]string `json:"drafts"`
	Score            int                       `json:"score"`
	SupplyPodAwarded bool                      `json:"supplyPodAwarded"`
	RewardClaimed    bool                      `json:"rewardClaimed"`
}

func (st *attemptState) toDTO() attemptStateDTO {
	return attemptStateDTO{
		Status: st.status, ElapsedSec: st.elapsedSec, Drafts: st.drafts,
		Score: st.score, SupplyPodAwarded: st.supplyPodAwarded, RewardClaimed: st.rewardClaimed,
	}
}

func (d attemptStateDTO) toState() *attemptState {
	st := &attemptState{
		status: d.Status, elapsedSec: d.ElapsedSec, drafts: d.Drafts,
		score: d.Score, supplyPodAwarded: d.SupplyPodAwarded, rewardClaimed: d.RewardClaimed,
	}
	if st.status == "" {
		st.status = "IN_PROGRESS"
	}
	if st.drafts == nil {
		st.drafts = make(map[int]map[string]string)
	}
	return st
}

func isUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i := 0; i < 36; i++ {
		c := s[i]
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				return false
			}
		}
	}
	return true
}

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
	s.loaded[studentID] = true
	s.mu.Unlock()
	return nil
}

func (s *Service) persistStudent(ctx context.Context, studentID string) error {
	if s.store == nil || !isUUID(studentID) {
		return nil
	}
	s.mu.Lock()
	st := quizState{
		Attempts: make(map[string]attemptStateDTO),
		Weekly:   make(map[string]bool),
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
	s.mu.Unlock()
	return s.store.Save(ctx, studentID, "astroquiz", st)
}

func (s *Service) currentWeekKey() string {
	yr, wk := s.nowClock().UTC().ISOWeek()
	return fmt.Sprintf("%04d-W%02d", yr, wk)
}

func (s *Service) weeklyDeadlineStr() string {
	now := s.nowClock().UTC()
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
func mergeDrafts(dst map[int]map[string]string, drafts map[int]map[string]string) {
	for k, v := range drafts {
		if k < 1 || k > 10 {
			continue
		}
		copied := make(map[string]string, len(v))
		for key, val := range v {
			copied[key] = val
		}
		dst[k] = copied
	}
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
	wkKey := studentID + ":" + s.currentWeekKey()
	claimed := s.weeklySupplyPod[wkKey]
	deadline := s.weeklyDeadlineStr()

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
		return PaperAttemptView{}, fmt.Errorf("paper %q not found", paperCode)
	}
	st := s.getOrCreateAttemptLocked(studentID, paperCode)
	view := s.renderAttemptLocked(studentID, p, st, locale)
	s.mu.Unlock()
	if err := s.persistStudent(ctx, studentID); err != nil {
		return PaperAttemptView{}, err
	}
	return view, nil
}

func (s *Service) SavePaper(ctx context.Context, studentID, paperCode, locale string, elapsedSec int, drafts map[int]map[string]string) (PaperAttemptView, error) {
	if elapsedSec < 0 || elapsedSec > maxElapsedSec {
		return PaperAttemptView{}, fmt.Errorf("elapsedSec must be 0..%d", maxElapsedSec)
	}
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
		return PaperAttemptView{}, fmt.Errorf("paper %q not found", paperCode)
	}
	st := s.getOrCreateAttemptLocked(studentID, paperCode)
	if st.status == "SUBMITTED" {
		view := s.renderAttemptLocked(studentID, p, st, locale)
		s.mu.Unlock()
		return view, nil
	}
	st.elapsedSec = elapsedSec
	mergeDrafts(st.drafts, drafts)
	view := s.renderAttemptLocked(studentID, p, st, locale)
	s.mu.Unlock()
	if err := s.persistStudent(ctx, studentID); err != nil {
		return PaperAttemptView{}, err
	}
	return view, nil
}

// SubmitPaper grades all 10 questions at once, reveals worked solutions, and awards
// 1 Weekly Supply Pod (+5 AC) max once per ISO week.
func (s *Service) SubmitPaper(ctx context.Context, studentID, paperCode, locale string, elapsedSec int, drafts map[int]map[string]string) (PaperAttemptView, error) {
	if elapsedSec < 0 || elapsedSec > maxElapsedSec {
		return PaperAttemptView{}, fmt.Errorf("elapsedSec must be 0..%d", maxElapsedSec)
	}
	if err := reqguard.ValidateDrafts(drafts); err != nil {
		return PaperAttemptView{}, err
	}
	if err := s.ensureStudentLoaded(ctx, studentID); err != nil {
		return PaperAttemptView{}, err
	}
	s.mu.Lock()
	var paper paperDef
	var view PaperAttemptView
	var claimWeek string
	view, err := func() (PaperAttemptView, error) {
		p, ok := s.findPaper(paperCode)
		if !ok {
			return PaperAttemptView{}, fmt.Errorf("paper %q not found", paperCode)
		}
		paper = p
		st := s.getOrCreateAttemptLocked(studentID, paperCode)
		if st.status != "SUBMITTED" {
			st.elapsedSec = elapsedSec
			mergeDrafts(st.drafts, drafts)
			score := 0
			for i := 0; i < 10; i++ {
				idx := i + 1
				tmpl := p.templates[i%len(p.templates)]
				prob, _ := mathengine.GenerateByCode(tmpl, seedForPaperQ(p.code, idx), locale)
				if mathengine.Grade(prob, st.drafts[idx]).IsCorrect {
					score++
				}
			}
			st.score = score
			st.status = "SUBMITTED"
		}
		wk := s.currentWeekKey()
		wkKey := studentID + ":" + wk
		if !s.weeklySupplyPod[wkKey] && !st.rewardClaimed {
			if s.ledger == nil {
				s.weeklySupplyPod[wkKey] = true
				st.rewardClaimed = true
				st.supplyPodAwarded = true
			} else {
				// Claim the reward outside the lock, after the attempt state
				// has been persisted. A failed ledger call stays retryable.
				claimWeek = wk
			}
		}
		return s.renderAttemptLocked(studentID, p, st, locale), nil
	}()
	s.mu.Unlock()
	if err != nil {
		return PaperAttemptView{}, err
	}

	if claimWeek != "" {
		if _, err := s.ledger.Apply(ctx, economy.MutationRequest{
			StudentID:      studentID,
			Currency:       economy.CurrencyAC,
			Delta:          WeeklySupplyPodRewardAC,
			SourceType:     "ASTRO_QUIZ_SUPPLY_POD",
			SourceID:       claimWeek,
			IdempotencyKey: fmt.Sprintf("supply_pod:%s:%s", studentID, claimWeek),
		}); err != nil {
			// Persist the submitted paper first so the retry only has to
			// claim the reward again.
			if persistErr := s.persistStudent(ctx, studentID); persistErr != nil {
				return PaperAttemptView{}, persistErr
			}
			return PaperAttemptView{}, fmt.Errorf("award weekly supply pod: %w", err)
		}
		s.mu.Lock()
		st := s.getOrCreateAttemptLocked(studentID, paperCode)
		s.weeklySupplyPod[studentID+":"+claimWeek] = true
		st.rewardClaimed = true
		st.supplyPodAwarded = true
		view = s.renderAttemptLocked(studentID, paper, st, locale)
		s.mu.Unlock()
	}

	if err := s.persistStudent(ctx, studentID); err != nil {
		return PaperAttemptView{}, err
	}
	return view, nil
}

func (s *Service) GetPrintablePaper(ctx context.Context, studentID, paperCode, locale string, includeKey bool) (PrintablePaper, error) {
	if err := s.ensureStudentLoaded(ctx, studentID); err != nil {
		return PrintablePaper{}, err
	}
	p, ok := s.findPaper(paperCode)
	if !ok {
		return PrintablePaper{}, fmt.Errorf("paper %q not found", paperCode)
	}
	if includeKey {
		s.mu.Lock()
		st := s.attempts[studentID][paperCode]
		s.mu.Unlock()
		if st == nil || st.status != "SUBMITTED" {
			return PrintablePaper{}, fmt.Errorf("answer key is available only after submission")
		}
	}
	qs := make([]mathengine.RenderedQuestion, 10)
	keys := make([]map[string]string, 10)
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
		PaperCode:  p.code,
		Title:      p.title.For(locale),
		Grade:      p.grade,
		Category:   p.category,
		Questions:  qs,
		AnswerKeys: keys,
	}, nil
}

func (s *Service) getOrCreateAttemptLocked(studentID, paperCode string) *attemptState {
	if s.attempts[studentID] == nil {
		s.attempts[studentID] = make(map[string]*attemptState)
	}
	st := s.attempts[studentID][paperCode]
	if st == nil {
		st = &attemptState{
			status: "IN_PROGRESS",
			drafts: make(map[int]map[string]string),
		}
		s.attempts[studentID][paperCode] = st
	}
	return st
}

func (s *Service) renderAttemptLocked(studentID string, p paperDef, st *attemptState, locale string) PaperAttemptView {
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
		SupplyPodClaimedWk: s.weeklySupplyPod[studentID+":"+s.currentWeekKey()],
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

func (h *Handler) resolveStudent(ctx context.Context, in AuthIn) (string, string, error) {
	locale := in.Locale
	if locale != "id" {
		locale = "en"
	}
	if h.authSvc == nil {
		return "demo-pilot1", locale, nil
	}
	tok := reqguard.Token(in.CookieAccess, in.Authorization)
	if tok != "" {
		if p, err := h.authSvc.ResolvePrincipal(ctx, tok); err == nil {
			return p.ID, locale, nil
		}
	}
	return "", locale, auth.ErrInvalidCredentials
}

func (h *Handler) RegisterRoutes(_ chi.Router, api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "astroquiz-list",
		Method:      http.MethodGet,
		Path:        "/api/v1/astro-quiz/papers",
		Summary:     "List AstroQuiz papers across Essential, Top School, and HOTS categories",
		Tags:        []string{"AstroQuiz"},
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
		id, locale, err := h.resolveStudent(ctx, in.AuthIn)
		if err != nil {
			return nil, huma.Error401Unauthorized("authentication required")
		}
		papers, claimed, err := h.svc.ListPapers(ctx, id, in.Grade, in.Category, locale)
		if err != nil {
			return nil, huma.Error503ServiceUnavailable("astro quiz unavailable")
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
	}, func(ctx context.Context, in *struct {
		AuthIn
		Code string `path:"code"`
	}) (*struct{ Body PaperAttemptView }, error) {
		id, locale, err := h.resolveStudent(ctx, in.AuthIn)
		if err != nil {
			return nil, huma.Error401Unauthorized("authentication required")
		}
		view, err := h.svc.OpenPaper(ctx, id, in.Code, locale)
		if err != nil {
			return nil, huma.Error404NotFound(err.Error())
		}
		return &struct{ Body PaperAttemptView }{Body: view}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "astroquiz-save",
		Method:      http.MethodPost,
		Path:        "/api/v1/astro-quiz/papers/{code}/save",
		Summary:     "Save in-progress AstroQuiz answers and elapsed timer",
		Tags:        []string{"AstroQuiz"},
	}, func(ctx context.Context, in *struct {
		AuthIn
		Code string `path:"code"`
		Body struct {
			ElapsedSec int                       `json:"elapsedSec"`
			Drafts     map[int]map[string]string `json:"drafts"`
		}
	}) (*struct{ Body PaperAttemptView }, error) {
		id, locale, err := h.resolveStudent(ctx, in.AuthIn)
		if err != nil {
			return nil, huma.Error401Unauthorized("authentication required")
		}
		view, err := h.svc.SavePaper(ctx, id, in.Code, locale, in.Body.ElapsedSec, in.Body.Drafts)
		if err != nil {
			return nil, huma.Error400BadRequest(err.Error())
		}
		return &struct{ Body PaperAttemptView }{Body: view}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "astroquiz-submit",
		Method:      http.MethodPost,
		Path:        "/api/v1/astro-quiz/papers/{code}/submit",
		Summary:     "Submit full AstroQuiz paper to reveal solutions and claim Weekly Supply Pod",
		Tags:        []string{"AstroQuiz"},
	}, func(ctx context.Context, in *struct {
		AuthIn
		Code string `path:"code"`
		Body struct {
			ElapsedSec int                       `json:"elapsedSec"`
			Drafts     map[int]map[string]string `json:"drafts"`
		}
	}) (*struct{ Body PaperAttemptView }, error) {
		id, locale, err := h.resolveStudent(ctx, in.AuthIn)
		if err != nil {
			return nil, huma.Error401Unauthorized("authentication required")
		}
		view, err := h.svc.SubmitPaper(ctx, id, in.Code, locale, in.Body.ElapsedSec, in.Body.Drafts)
		if err != nil {
			return nil, huma.Error400BadRequest(err.Error())
		}
		return &struct{ Body PaperAttemptView }{Body: view}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "astroquiz-print",
		Method:      http.MethodGet,
		Path:        "/api/v1/astro-quiz/papers/{code}/print",
		Summary:     "Get ink-friendly A4 printable payload for an AstroQuiz paper",
		Tags:        []string{"AstroQuiz"},
	}, func(ctx context.Context, in *struct {
		AuthIn
		Code       string `path:"code"`
		IncludeKey bool   `query:"includeKey" default:"false"`
	}) (*struct{ Body PrintablePaper }, error) {
		id, locale, err := h.resolveStudent(ctx, in.AuthIn)
		if err != nil {
			return nil, huma.Error401Unauthorized("authentication required")
		}
		view, err := h.svc.GetPrintablePaper(ctx, id, in.Code, locale, in.IncludeKey)
		if err != nil {
			return nil, huma.Error404NotFound(err.Error())
		}
		return &struct{ Body PrintablePaper }{Body: view}, nil
	})
}

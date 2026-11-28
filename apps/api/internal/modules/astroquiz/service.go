package astroquiz

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
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/economy"
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/reqguard"
)

const (
	WeeklySupplyPodRewardAC = 5
	paperQuestionCount      = 10
	// maxElapsedSec bounds the client-reported timer (24h); server-side timing is a follow-up.
	maxElapsedSec = 24 * 60 * 60
)

var (
	ErrPaperNotFound = errors.New("paper not found")
	ErrInvalidInput  = errors.New("invalid input")
	// ErrRewardUnavailable means the Supply Pod could not be credited; re-submitting retries it.
	ErrRewardUnavailable = errors.New("supply pod reward could not be credited; please retry")
)

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

// PrintablePaper is the A4 worksheet payload. AnswerKeys is only populated for adults
// (parent/teacher/staff) or for a student whose attempt on this paper is already SUBMITTED,
// so the print view can never be used to read answers before submitting.
type PrintablePaper struct {
	PaperCode          string                        `json:"paperCode"`
	Title              string                        `json:"title"`
	Grade              int                           `json:"grade"`
	Category           Category                      `json:"category"`
	Questions          []mathengine.RenderedQuestion `json:"questions"`
	AnswerKeyAvailable bool                          `json:"answerKeyAvailable"`
	AnswerKeys         []map[string]string           `json:"answerKeys,omitempty"`
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
	// podWeek is set when this submission claimed the week's Supply Pod; podPending stays
	// true until the ledger credit succeeds so a failed credit can be retried.
	podWeek    string
	podPending bool
}

type Service struct {
	mu              sync.Mutex
	ledger          economy.Ledger
	papers          []paperDef
	attempts        map[string]map[string]*attemptState // studentID -> paperCode -> state
	weeklySupplyPod map[string]bool                     // studentID:weekKey -> claimed
	nowClock        func() time.Time
}

func NewService(ledger economy.Ledger) *Service {
	return &Service{
		ledger:          ledger,
		papers:          buildPapers(),
		attempts:        make(map[string]map[string]*attemptState),
		weeklySupplyPod: make(map[string]bool),
		nowClock:        time.Now,
	}
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

func (s *Service) ListPapers(studentID string, grade int, category, locale string) ([]PaperSummary, bool) {
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
			QuestionCount:  paperQuestionCount,
			Status:         status,
			BestScore:      score,
			WeeklyDeadline: deadline,
		})
	}
	return out, claimed
}

func (s *Service) OpenPaper(studentID, paperCode, locale string) (PaperAttemptView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.findPaper(paperCode)
	if !ok {
		return PaperAttemptView{}, fmt.Errorf("%w: %q", ErrPaperNotFound, paperCode)
	}
	st := s.getOrCreateAttemptLocked(studentID, paperCode)
	return s.renderAttemptLocked(studentID, p, st, locale), nil
}

func (s *Service) SavePaper(studentID, paperCode, locale string, elapsedSec int, drafts map[int]map[string]string) (PaperAttemptView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.findPaper(paperCode)
	if !ok {
		return PaperAttemptView{}, fmt.Errorf("%w: %q", ErrPaperNotFound, paperCode)
	}
	clean, err := sanitizeDrafts(elapsedSec, drafts)
	if err != nil {
		return PaperAttemptView{}, err
	}
	st := s.getOrCreateAttemptLocked(studentID, paperCode)
	if st.status == "SUBMITTED" {
		return s.renderAttemptLocked(studentID, p, st, locale), nil
	}
	st.elapsedSec = elapsedSec
	for k, v := range clean {
		st.drafts[k] = v
	}
	return s.renderAttemptLocked(studentID, p, st, locale), nil
}

// sanitizeDrafts bounds the client payload: question indices 1..10 only, bounded answer maps,
// and a sane elapsed time.
func sanitizeDrafts(elapsedSec int, drafts map[int]map[string]string) (map[int]map[string]string, error) {
	if elapsedSec < 0 || elapsedSec > maxElapsedSec {
		return nil, fmt.Errorf("%w: elapsedSec must be 0..%d", ErrInvalidInput, maxElapsedSec)
	}
	clean := make(map[int]map[string]string, len(drafts))
	for idx, ans := range drafts {
		if idx < 1 || idx > paperQuestionCount {
			return nil, fmt.Errorf("%w: draft question index must be 1..%d", ErrInvalidInput, paperQuestionCount)
		}
		c, err := reqguard.CopyAnswers(ans)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidInput, err)
		}
		clean[idx] = c
	}
	return clean, nil
}

// SubmitPaper grades all 10 questions at once, reveals worked solutions, and awards
// 1 Weekly Supply Pod (+5 AC) max once per ISO week. The pod is only recorded as claimed after
// the ledger credit succeeds; if it fails, ErrRewardUnavailable is returned and re-submitting the
// same paper retries the credit (the idempotency key makes the retry safe).
func (s *Service) SubmitPaper(ctx context.Context, studentID, paperCode, locale string, elapsedSec int, drafts map[int]map[string]string) (PaperAttemptView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.findPaper(paperCode)
	if !ok {
		return PaperAttemptView{}, fmt.Errorf("%w: %q", ErrPaperNotFound, paperCode)
	}
	st := s.getOrCreateAttemptLocked(studentID, paperCode)
	if st.status != "SUBMITTED" {
		clean, err := sanitizeDrafts(elapsedSec, drafts)
		if err != nil {
			return PaperAttemptView{}, err
		}
		st.elapsedSec = elapsedSec
		for k, v := range clean {
			st.drafts[k] = v
		}
		score := 0
		for i := 0; i < paperQuestionCount; i++ {
			idx := i + 1
			tmpl := p.templates[i%len(p.templates)]
			prob, _ := mathengine.GenerateByCode(tmpl, seedForPaperQ(p.code, idx), locale)
			if mathengine.Grade(prob, st.drafts[idx]).IsCorrect {
				score++
			}
		}
		st.score = score
		st.status = "SUBMITTED"

		wk := s.currentWeekKey()
		if !s.weeklySupplyPod[studentID+":"+wk] && !s.podPendingLocked(studentID) {
			st.podWeek = wk
			st.podPending = true
		}
	}

	if st.podPending {
		if s.ledger != nil {
			if _, err := s.ledger.Apply(ctx, economy.MutationRequest{
				StudentID:      studentID,
				Currency:       economy.CurrencyAC,
				Delta:          WeeklySupplyPodRewardAC,
				SourceType:     "ASTRO_QUIZ_SUPPLY_POD",
				SourceID:       st.podWeek,
				IdempotencyKey: fmt.Sprintf("supply_pod:%s:%s", studentID, st.podWeek),
			}); err != nil {
				return PaperAttemptView{}, fmt.Errorf("%w: %v", ErrRewardUnavailable, err)
			}
		}
		s.weeklySupplyPod[studentID+":"+st.podWeek] = true
		st.supplyPodAwarded = true
		st.podPending = false
	}
	return s.renderAttemptLocked(studentID, p, st, locale), nil
}

// podPendingLocked reports whether another paper already holds an un-credited Supply Pod claim,
// so two submissions in the same week cannot both claim it while a credit is being retried.
func (s *Service) podPendingLocked(studentID string) bool {
	for _, st := range s.attempts[studentID] {
		if st.podPending {
			return true
		}
	}
	return false
}

// GetPrintablePaper builds the A4 worksheet. Answer keys are included only when
// includeAnswerKeys is true (decided by the caller's role and attempt state, see CanSeeAnswerKeys).
func (s *Service) GetPrintablePaper(paperCode, locale string, includeAnswerKeys bool) (PrintablePaper, error) {
	p, ok := s.findPaper(paperCode)
	if !ok {
		return PrintablePaper{}, fmt.Errorf("%w: %q", ErrPaperNotFound, paperCode)
	}
	qs := make([]mathengine.RenderedQuestion, paperQuestionCount)
	var keys []map[string]string
	if includeAnswerKeys {
		keys = make([]map[string]string, paperQuestionCount)
	}
	for i := 0; i < paperQuestionCount; i++ {
		idx := i + 1
		tmpl := p.templates[i%len(p.templates)]
		prob, _ := mathengine.GenerateByCode(tmpl, seedForPaperQ(p.code, idx), locale)
		qs[i] = prob.Question
		if includeAnswerKeys {
			keys[i] = prob.Solution.CorrectAnswers
		}
	}
	return PrintablePaper{
		PaperCode:          p.code,
		Title:              p.title.For(locale),
		Grade:              p.grade,
		Category:           p.category,
		Questions:          qs,
		AnswerKeyAvailable: includeAnswerKeys,
		AnswerKeys:         keys,
	}, nil
}

// CanSeeAnswerKeys: adults always may (for marking printed worksheets); a student only once
// their own attempt on this paper has been SUBMITTED.
func (s *Service) CanSeeAnswerKeys(principal auth.Principal, paperCode string) bool {
	if auth.HasAllowedRole(principal.Role, auth.RoleParent, auth.RoleTeacher, auth.RoleSchoolAdmin, auth.RoleAdmin, auth.RoleContentEditor) {
		return true
	}
	if principal.Role != auth.RoleStudent || principal.ID == "" || principal.ID == reqguard.DemoStudentID {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.attempts[principal.ID][paperCode]
	return st != nil && st.status == "SUBMITTED"
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
	qs := make([]PaperQuestion, paperQuestionCount)
	for i := 0; i < paperQuestionCount; i++ {
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
		TotalQuestions:     paperQuestionCount,
		SupplyPodAwarded:   st.supplyPodAwarded,
		SupplyPodClaimedWk: s.weeklySupplyPod[studentID+":"+s.currentWeekKey()],
		Questions:          qs,
	}
}

func buildPapers() []paperDef {
	return []paperDef{
		{code: "G1-ESS-01", grade: 1, category: CategoryEssential, title: mathengine.LocaleText{EN: "Grade 1 Essential Revision: Numbers & Operations", ID: "Revisi Esensial Kelas 1: Bilangan & Operasi"}, templates: []string{"g1_add_within_20", "g1_sub_within_20", "g1_place_value_tens_ones", "g1_pattern_hots"}},
		{code: "G1-TOP-01", grade: 1, category: CategoryTopSchool, title: mathengine.LocaleText{EN: "Grade 1 Top School Year-End Paper (Math-F1)", ID: "Ujian Akhir Tahun Kelas 1 (Math-F1)"}, templates: []string{"g1_add_within_20", "g1_place_value_tens_ones", "g1_sub_within_20", "g1_pattern_hots"}},
		{code: "G1-HOTS-01", grade: 1, category: CategoryHOTS, title: mathengine.LocaleText{EN: "Grade 1 HOTS Heuristic Paper (Ages 6–9)", ID: "Kertas HOTS Heuristik Kelas 1 (Usia 6–9)"}, templates: []string{"g1_pattern_hots", "g1_add_within_20", "g1_place_value_tens_ones"}},

		{code: "G2-ESS-01", grade: 2, category: CategoryEssential, title: mathengine.LocaleText{EN: "Grade 2 Essential Revision: Addition & Multiplication", ID: "Revisi Esensial Kelas 2: Penjumlahan & Perkalian"}, templates: []string{"g2_add_within_1000", "g2_mul_tables_2_5_10", "g2_unit_fraction_mcq", "g2_bar_model_compare"}},
		{code: "G2-TOP-01", grade: 2, category: CategoryTopSchool, title: mathengine.LocaleText{EN: "Grade 2 Top School Year-End Paper (Math-F2)", ID: "Ujian Akhir Tahun Kelas 2 (Math-F2)"}, templates: []string{"g2_add_within_1000", "g2_bar_model_compare", "g2_mul_tables_2_5_10", "g2_unit_fraction_mcq"}},
		{code: "G2-HOTS-01", grade: 2, category: CategoryHOTS, title: mathengine.LocaleText{EN: "Grade 2 HOTS Paper: Draw Models", ID: "Kertas HOTS Kelas 2: Menggambar Model Batang"}, templates: []string{"g2_bar_model_compare", "g1_pattern_hots", "g2_mul_tables_2_5_10"}},

		{code: "G3-ESS-01", grade: 3, category: CategoryEssential, title: mathengine.LocaleText{EN: "Grade 3 Essential Revision: Area, Division & Fractions", ID: "Revisi Esensial Kelas 3: Luas, Pembagian & Pecahan"}, templates: []string{"g3_mul_6_to_9", "g3_div_with_remainder", "g3_area_perimeter_rect", "g3_equiv_fraction"}},
		{code: "G3-TOP-01", grade: 3, category: CategoryTopSchool, title: mathengine.LocaleText{EN: "Grade 3 Top School Year-End Paper (Math-I1)", ID: "Ujian Akhir Tahun Kelas 3 (Math-I1)"}, templates: []string{"g3_area_perimeter_rect", "g3_div_with_remainder", "g3_equiv_fraction", "g3_mul_6_to_9"}},
		{code: "G3-HOTS-01", grade: 3, category: CategoryHOTS, title: mathengine.LocaleText{EN: "Grade 3 HOTS Heuristic Paper (Ages 8–11)", ID: "Kertas HOTS Heuristik Kelas 3 (Usia 8–11)"}, templates: []string{"g3_div_with_remainder", "g3_area_perimeter_rect", "g2_bar_model_compare"}},

		{code: "G4-ESS-01", grade: 4, category: CategoryEssential, title: mathengine.LocaleText{EN: "Grade 4 Essential Revision: LCM & Decimals", ID: "Revisi Esensial Kelas 4: KPK & Desimal"}, templates: []string{"g4_lcm_gcf", "g4_decimal_add_sub", "g4_fraction_of_set", "g4_work_backwards_hots"}},
		{code: "G4-TOP-01", grade: 4, category: CategoryTopSchool, title: mathengine.LocaleText{EN: "Grade 4 Top School Year-End Paper (Math-I2)", ID: "Ujian Akhir Tahun Kelas 4 (Math-I2)"}, templates: []string{"g4_decimal_add_sub", "g4_lcm_gcf", "g4_fraction_of_set", "g4_work_backwards_hots"}},
		{code: "G4-HOTS-01", grade: 4, category: CategoryHOTS, title: mathengine.LocaleText{EN: "Grade 4 HOTS Paper: Work Backwards", ID: "Kertas HOTS Kelas 4: Bekerja Mundur"}, templates: []string{"g4_work_backwards_hots", "g4_lcm_gcf", "g4_fraction_of_set"}},

		{code: "G5-ESS-01", grade: 5, category: CategoryEssential, title: mathengine.LocaleText{EN: "Grade 5 Essential Revision: Percentage & Ratio", ID: "Revisi Esensial Kelas 5: Persentase & Rasio"}, templates: []string{"g5_percentage_discount", "g5_ratio_word_problem", "g5_volume_cuboid", "g5_average_mean"}},
		{code: "G5-TOP-01", grade: 5, category: CategoryTopSchool, title: mathengine.LocaleText{EN: "Grade 5 Top School Year-End Paper (Math-A1)", ID: "Ujian Akhir Tahun Kelas 5 (Math-A1)"}, templates: []string{"g5_ratio_word_problem", "g5_percentage_discount", "g5_volume_cuboid", "g5_average_mean"}},
		{code: "G5-HOTS-01", grade: 5, category: CategoryHOTS, title: mathengine.LocaleText{EN: "Grade 5 HOTS Paper: Ratio & Models", ID: "Kertas HOTS Kelas 5: Rasio & Model Batang"}, templates: []string{"g5_ratio_word_problem", "g4_work_backwards_hots", "g5_percentage_discount"}},

		{code: "G6-ESS-01", grade: 6, category: CategoryEssential, title: mathengine.LocaleText{EN: "Grade 6 Essential Revision: Algebra, Speed & Circles", ID: "Revisi Esensial Kelas 6: Aljabar, Kecepatan & Lingkaran"}, templates: []string{"g6_algebra_linear", "g6_speed_distance_time", "g6_circle_circumference", "g6_supposition_hots"}},
		{code: "G6-TOP-01", grade: 6, category: CategoryTopSchool, title: mathengine.LocaleText{EN: "Grade 6 Final Exam Mock Paper (Math-M1)", ID: "Tryout Ujian Akhir SD Kelas 6 (Math-M1)"}, templates: []string{"g6_algebra_linear", "g6_speed_distance_time", "g6_circle_circumference", "g6_supposition_hots"}},
		{code: "G6-HOTS-01", grade: 6, category: CategoryHOTS, title: mathengine.LocaleText{EN: "Grade 6 HOTS Paper: Supposition Method", ID: "Kertas HOTS Kelas 6: Metode Pengandaian"}, templates: []string{"g6_supposition_hots", "g6_algebra_linear", "g6_speed_distance_time"}},
	}
}

type Handler struct {
	svc     *Service
	authSvc *auth.Service
}

func NewHandler(svc *Service, authSvc *auth.Service) *Handler {
	return &Handler{svc: svc, authSvc: authSvc}
}

var authSecurity = []map[string][]string{{"accessCookie": {}}, {"bearerAuth": {}}}

func (h *Handler) resolver() reqguard.PrincipalResolver {
	if h.authSvc == nil {
		return nil
	}
	return h.authSvc
}

// resolveStudent returns the caller's ID and locale. Attempt-changing calls require an
// authenticated student; anonymous reads fall back to the read-only guest identity.
func (h *Handler) resolveStudent(ctx context.Context, c reqguard.Credentials, requireStudent bool) (string, string, error) {
	p, err := reqguard.Resolve(ctx, h.resolver(), c, requireStudent)
	if err != nil {
		return "", "", err
	}
	if requireStudent && p.Role != auth.RoleStudent {
		return "", "", auth.ErrForbidden
	}
	return p.ID, c.NormalizedLocale(), nil
}

func toHTTPError(err error) error {
	switch {
	case errors.Is(err, reqguard.ErrUnauthenticated):
		return huma.Error401Unauthorized("authentication required")
	case errors.Is(err, auth.ErrForbidden):
		return huma.Error403Forbidden("student account required")
	case errors.Is(err, ErrPaperNotFound):
		return huma.Error404NotFound(err.Error())
	case errors.Is(err, ErrInvalidInput):
		return huma.Error400BadRequest(err.Error())
	case errors.Is(err, ErrRewardUnavailable):
		return huma.Error503ServiceUnavailable(ErrRewardUnavailable.Error())
	default:
		return huma.Error500InternalServerError("internal error")
	}
}

type paperProgressBody struct {
	ElapsedSec int                       `json:"elapsedSec" minimum:"0" maximum:"86400"`
	Drafts     map[int]map[string]string `json:"drafts"`
}

func (h *Handler) RegisterRoutes(_ chi.Router, api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "astroquiz-list",
		Method:      http.MethodGet,
		Path:        "/api/v1/astro-quiz/papers",
		Summary:     "List AstroQuiz papers across Essential, Top School, and HOTS categories",
		Tags:        []string{"AstroQuiz"},
	}, func(ctx context.Context, in *struct {
		reqguard.Credentials
		Grade    int    `query:"grade" default:"1" minimum:"0" maximum:"6"`
		Category string `query:"category" default:"ALL" enum:"ALL,ESSENTIAL,TOP_SCHOOL,HOTS"`
	}) (*struct {
		Body struct {
			Papers                   []PaperSummary `json:"papers"`
			SupplyPodClaimedThisWeek bool           `json:"supplyPodClaimedThisWeek"`
		}
	}, error) {
		id, locale, err := h.resolveStudent(ctx, in.Credentials, false)
		if err != nil {
			return nil, toHTTPError(err)
		}
		papers, claimed := h.svc.ListPapers(id, in.Grade, in.Category, locale)
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
		reqguard.Credentials
		Code string `path:"code" maxLength:"32"`
	}) (*struct{ Body PaperAttemptView }, error) {
		id, locale, err := h.resolveStudent(ctx, in.Credentials, true)
		if err != nil {
			return nil, toHTTPError(err)
		}
		view, err := h.svc.OpenPaper(id, in.Code, locale)
		if err != nil {
			return nil, toHTTPError(err)
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
		reqguard.Credentials
		Code string `path:"code" maxLength:"32"`
		Body paperProgressBody
	}) (*struct{ Body PaperAttemptView }, error) {
		id, locale, err := h.resolveStudent(ctx, in.Credentials, true)
		if err != nil {
			return nil, toHTTPError(err)
		}
		view, err := h.svc.SavePaper(id, in.Code, locale, in.Body.ElapsedSec, in.Body.Drafts)
		if err != nil {
			return nil, toHTTPError(err)
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
		reqguard.Credentials
		Code string `path:"code" maxLength:"32"`
		Body paperProgressBody
	}) (*struct{ Body PaperAttemptView }, error) {
		id, locale, err := h.resolveStudent(ctx, in.Credentials, true)
		if err != nil {
			return nil, toHTTPError(err)
		}
		view, err := h.svc.SubmitPaper(ctx, id, in.Code, locale, in.Body.ElapsedSec, in.Body.Drafts)
		if err != nil {
			return nil, toHTTPError(err)
		}
		return &struct{ Body PaperAttemptView }{Body: view}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "astroquiz-print",
		Method:      http.MethodGet,
		Path:        "/api/v1/astro-quiz/papers/{code}/print",
		Summary:     "Get ink-friendly A4 printable payload for an AstroQuiz paper (answer keys only for adults or after submission)",
		Tags:        []string{"AstroQuiz"},
	}, func(ctx context.Context, in *struct {
		reqguard.Credentials
		Code string `path:"code" maxLength:"32"`
	}) (*struct{ Body PrintablePaper }, error) {
		p, err := reqguard.Resolve(ctx, h.resolver(), in.Credentials, false)
		if err != nil {
			return nil, toHTTPError(err)
		}
		view, err := h.svc.GetPrintablePaper(in.Code, in.NormalizedLocale(), h.svc.CanSeeAnswerKeys(p, in.Code))
		if err != nil {
			return nil, toHTTPError(err)
		}
		return &struct{ Body PrintablePaper }{Body: view}, nil
	})
}

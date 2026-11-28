package assignments

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
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
	maxTitleLen       = 120
	maxTemplatesPerAs = 20
)

var (
	// ErrAssignmentNotFound is also returned when the caller is not allowed to see the record,
	// so assignment IDs cannot be probed.
	ErrAssignmentNotFound = errors.New("assignment not found")
	ErrNotLinkedChild     = errors.New("parent can only assign to linked children")
	ErrInvalidInput       = errors.New("invalid input")
	// ErrRewardUnavailable means the completion AC could not be credited; re-submitting retries it.
	ErrRewardUnavailable = errors.New("assignment reward could not be credited; please retry")
)

type AssignedByRole string

const (
	AssignedBySystem  AssignedByRole = "SYSTEM"
	AssignedByParent  AssignedByRole = "PARENT"
	AssignedByTeacher AssignedByRole = "TEACHER"
)

type AssignmentSummary struct {
	ID             string         `json:"id"`
	Title          string         `json:"title"`
	AssignedByRole AssignedByRole `json:"assignedByRole"`
	AssignedByName string         `json:"assignedByName"`
	AcademicYear   int            `json:"academicYear"`
	DueDate        string         `json:"dueDate"`
	Status         string         `json:"status"` // INCOMPLETE | COMPLETE
	QuestionCount  int            `json:"questionCount"`
	CompletedCount int            `json:"completedCount"`
	Score          int            `json:"score"`
	RewardAC       int            `json:"rewardAc"`
}

type AssignmentQuestionView struct {
	Index           int                         `json:"index"`
	Question        mathengine.RenderedQuestion `json:"question"`
	SubmittedAnswer map[string]string           `json:"submittedAnswer,omitempty"`
	IsCorrect       *bool                       `json:"isCorrect,omitempty"`
	WorkedSolution  *mathengine.WorkedSolution  `json:"workedSolution,omitempty"`
}

type AssignmentDetail struct {
	Summary   AssignmentSummary        `json:"summary"`
	Questions []AssignmentQuestionView `json:"questions"`
}

type itemSpec struct {
	templateCode string
	seed         int64
}

type assignmentRecord struct {
	id             string
	studentID      string
	title          string
	assignedByRole AssignedByRole
	assignedByName string
	academicYear   int
	dueDate        string
	rewardAC       int
	items          []itemSpec
	submitted      map[int]map[string]string
	correct        map[int]bool
	completed      bool
	rewardPaid     bool
}

type Service struct {
	mu          sync.Mutex
	ledger      economy.Ledger
	records     map[string]*assignmentRecord
	parentLinks map[string][]string // parentID -> []studentID
	nextID      int
}

func NewService(ledger economy.Ledger) *Service {
	svc := &Service{
		ledger:  ledger,
		records: make(map[string]*assignmentRecord),
		parentLinks: map[string][]string{
			"demo-parent": {"demo-pilot1"},
		},
		nextID: 100,
	}
	svc.seedDefaults("demo-pilot1")
	return svc
}

func (s *Service) seedDefaults(studentID string) {
	s.records["asgn-1"] = &assignmentRecord{
		id:             "asgn-1",
		studentID:      studentID,
		title:          "Weekly Orbit Warmup: Addition & Place Value",
		assignedByRole: AssignedBySystem,
		assignedByName: "PlanetLogi",
		academicYear:   2026,
		dueDate:        "2026-12-03",
		rewardAC:       5,
		items: []itemSpec{
			{templateCode: "g1_add_within_20", seed: 701},
			{templateCode: "g1_sub_within_20", seed: 702},
			{templateCode: "g1_place_value_tens_ones", seed: 703},
			{templateCode: "g1_pattern_hots", seed: 704},
		},
		submitted: make(map[int]map[string]string),
		correct:   make(map[int]bool),
	}
}

func (s *Service) LinkParentChild(parentID, studentID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.parentLinks[parentID] = append(s.parentLinks[parentID], studentID)
}

func (s *Service) isLinkedLocked(parentID, studentID string) bool {
	for _, cid := range s.parentLinks[parentID] {
		if cid == studentID {
			return true
		}
	}
	return false
}

func (s *Service) ListForStudent(studentID, statusFilter, assignedByFilter string, year int) []AssignmentSummary {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]AssignmentSummary, 0)
	for _, r := range s.records {
		if r.studentID != studentID {
			continue
		}
		st := "INCOMPLETE"
		if r.completed {
			st = "COMPLETE"
		}
		if statusFilter != "" && statusFilter != "ALL" && st != statusFilter {
			continue
		}
		if assignedByFilter != "" && assignedByFilter != "ALL" && string(r.assignedByRole) != assignedByFilter {
			continue
		}
		if year > 0 && r.academicYear != year {
			continue
		}
		out = append(out, toSummary(r))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].DueDate != out[j].DueDate {
			return out[i].DueDate < out[j].DueDate
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func (s *Service) CreateParentCustomAssignment(parentID, parentName, studentID, title, dueDate string, templateCodes []string) (AssignmentSummary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.isLinkedLocked(parentID, studentID) {
		return AssignmentSummary{}, ErrNotLinkedChild
	}
	if title == "" || len(title) > maxTitleLen {
		return AssignmentSummary{}, fmt.Errorf("%w: title must be 1..%d characters", ErrInvalidInput, maxTitleLen)
	}
	if len(templateCodes) == 0 || len(templateCodes) > maxTemplatesPerAs {
		return AssignmentSummary{}, fmt.Errorf("%w: between 1 and %d question templates are required", ErrInvalidInput, maxTemplatesPerAs)
	}
	if dueDate != "" {
		if _, err := time.Parse("2006-01-02", dueDate); err != nil {
			return AssignmentSummary{}, fmt.Errorf("%w: dueDate must be YYYY-MM-DD", ErrInvalidInput)
		}
	}
	for _, code := range templateCodes {
		if _, ok := mathengine.GenerateByCode(code, 1, "en"); !ok {
			return AssignmentSummary{}, fmt.Errorf("%w: unknown question template %q", ErrInvalidInput, code)
		}
	}
	s.nextID++
	id := fmt.Sprintf("asgn-%d", s.nextID)
	items := make([]itemSpec, 0, len(templateCodes))
	for i, code := range templateCodes {
		items = append(items, itemSpec{templateCode: code, seed: int64(s.nextID*100 + i + 1)})
	}
	if dueDate == "" {
		dueDate = time.Now().UTC().AddDate(0, 0, 7).Format("2006-01-02")
	}
	rec := &assignmentRecord{
		id:             id,
		studentID:      studentID,
		title:          title,
		assignedByRole: AssignedByParent,
		assignedByName: parentName,
		academicYear:   time.Now().UTC().Year(),
		dueDate:        dueDate,
		rewardAC:       5,
		items:          items,
		submitted:      make(map[int]map[string]string),
		correct:        make(map[int]bool),
	}
	s.records[id] = rec
	return toSummary(rec), nil
}

// GetDetail returns an assignment to its student or to a parent linked to that student.
func (s *Service) GetDetail(viewer auth.Principal, assignmentID, locale string) (AssignmentDetail, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.records[assignmentID]
	if !ok || !s.canViewLocked(viewer, rec) {
		return AssignmentDetail{}, ErrAssignmentNotFound
	}
	return renderDetail(rec, locale), nil
}

func (s *Service) canViewLocked(viewer auth.Principal, rec *assignmentRecord) bool {
	if viewer.ID == rec.studentID {
		return true
	}
	return viewer.Role == auth.RoleParent && s.isLinkedLocked(viewer.ID, rec.studentID)
}

// SubmitQuestion records the owning student's answer for one question. A question can only be
// answered once (its worked solution is revealed afterwards); re-submitting returns the stored
// result unchanged. The completion AC goes to the record owner and is only marked paid after the
// ledger credit succeeds, so a failed credit is retried by the next submit call.
func (s *Service) SubmitQuestion(ctx context.Context, studentID, assignmentID string, qIndex int, answers map[string]string, locale string) (mathengine.GradeResult, AssignmentDetail, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.records[assignmentID]
	if !ok || rec.studentID != studentID {
		return mathengine.GradeResult{}, AssignmentDetail{}, ErrAssignmentNotFound
	}
	if qIndex < 1 || qIndex > len(rec.items) {
		return mathengine.GradeResult{}, AssignmentDetail{}, fmt.Errorf("%w: invalid question index", ErrInvalidInput)
	}
	item := rec.items[qIndex-1]
	prob, ok := mathengine.GenerateByCode(item.templateCode, item.seed, locale)
	if !ok {
		return mathengine.GradeResult{}, AssignmentDetail{}, fmt.Errorf("assignment %q references unknown template %q", rec.id, item.templateCode)
	}

	var res mathengine.GradeResult
	if prev, answered := rec.submitted[qIndex]; answered {
		res = mathengine.Grade(prob, prev)
	} else {
		copied, err := reqguard.CopyAnswers(answers)
		if err != nil {
			return mathengine.GradeResult{}, AssignmentDetail{}, fmt.Errorf("%w: %v", ErrInvalidInput, err)
		}
		res = mathengine.Grade(prob, copied)
		rec.submitted[qIndex] = copied
		rec.correct[qIndex] = res.IsCorrect
	}

	if len(rec.submitted) == len(rec.items) {
		rec.completed = true
		if !rec.rewardPaid {
			if s.ledger != nil {
				if _, err := s.ledger.Apply(ctx, economy.MutationRequest{
					StudentID:      rec.studentID,
					Currency:       economy.CurrencyAC,
					Delta:          int64(rec.rewardAC),
					SourceType:     "ASSIGNMENT_COMPLETE",
					SourceID:       rec.id,
					IdempotencyKey: fmt.Sprintf("asgn:%s:%s", rec.id, rec.studentID),
				}); err != nil {
					return mathengine.GradeResult{}, AssignmentDetail{}, fmt.Errorf("%w: %v", ErrRewardUnavailable, err)
				}
			}
			rec.rewardPaid = true
		}
	}
	return res, renderDetail(rec, locale), nil
}

func toSummary(r *assignmentRecord) AssignmentSummary {
	score := 0
	for _, ok := range r.correct {
		if ok {
			score++
		}
	}
	status := "INCOMPLETE"
	if r.completed {
		status = "COMPLETE"
	}
	return AssignmentSummary{
		ID:             r.id,
		Title:          r.title,
		AssignedByRole: r.assignedByRole,
		AssignedByName: r.assignedByName,
		AcademicYear:   r.academicYear,
		DueDate:        r.dueDate,
		Status:         status,
		QuestionCount:  len(r.items),
		CompletedCount: len(r.submitted),
		Score:          score,
		RewardAC:       r.rewardAC,
	}
}

func renderDetail(r *assignmentRecord, locale string) AssignmentDetail {
	qs := make([]AssignmentQuestionView, len(r.items))
	for i, item := range r.items {
		idx := i + 1
		prob, _ := mathengine.GenerateByCode(item.templateCode, item.seed, locale)
		qv := AssignmentQuestionView{
			Index:           idx,
			Question:        prob.Question,
			SubmittedAnswer: r.submitted[idx],
		}
		if _, done := r.submitted[idx]; done {
			ok := r.correct[idx]
			qv.IsCorrect = &ok
			sol := prob.Solution
			qv.WorkedSolution = &sol
		}
		qs[i] = qv
	}
	return AssignmentDetail{Summary: toSummary(r), Questions: qs}
}

type Handler struct {
	svc     *Service
	authSvc *auth.Service
}

func NewHandler(svc *Service, authSvc *auth.Service) *Handler {
	return &Handler{svc: svc, authSvc: authSvc}
}

var authSecurity = []map[string][]string{{"accessCookie": {}}, {"bearerAuth": {}}}

// resolve returns the caller. With requireAuth=false an anonymous request becomes the
// read-only guest student, which only ever sees the guest's own records.
func (h *Handler) resolve(ctx context.Context, c reqguard.Credentials, requireAuth bool) (auth.Principal, error) {
	var r reqguard.PrincipalResolver
	if h.authSvc != nil {
		r = h.authSvc
	}
	return reqguard.Resolve(ctx, r, c, requireAuth)
}

func toHTTPError(err error) error {
	switch {
	case errors.Is(err, reqguard.ErrUnauthenticated):
		return huma.Error401Unauthorized("authentication required")
	case errors.Is(err, auth.ErrForbidden), errors.Is(err, ErrNotLinkedChild):
		return huma.Error403Forbidden(err.Error())
	case errors.Is(err, ErrAssignmentNotFound):
		return huma.Error404NotFound(err.Error())
	case errors.Is(err, ErrInvalidInput):
		return huma.Error400BadRequest(err.Error())
	case errors.Is(err, ErrRewardUnavailable):
		return huma.Error503ServiceUnavailable(ErrRewardUnavailable.Error())
	default:
		return huma.Error500InternalServerError("internal error")
	}
}

func (h *Handler) RegisterRoutes(_ chi.Router, api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "assignments-list",
		Method:      http.MethodGet,
		Path:        "/api/v1/assignments",
		Summary:     "List student assignments with status, year, and assigned-by filters",
		Tags:        []string{"Assignments"},
	}, func(ctx context.Context, in *struct {
		reqguard.Credentials
		Status     string `query:"status" default:"ALL" enum:"ALL,INCOMPLETE,COMPLETE"`
		AssignedBy string `query:"assignedBy" default:"ALL" enum:"ALL,SYSTEM,PARENT,TEACHER"`
		Year       int    `query:"year" default:"0" minimum:"0"`
	}) (*struct {
		Body struct {
			Assignments []AssignmentSummary `json:"assignments"`
		}
	}, error) {
		p, err := h.resolve(ctx, in.Credentials, false)
		if err != nil {
			return nil, toHTTPError(err)
		}
		out := &struct {
			Body struct {
				Assignments []AssignmentSummary `json:"assignments"`
			}
		}{}
		out.Body.Assignments = h.svc.ListForStudent(p.ID, in.Status, in.AssignedBy, in.Year)
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "assignments-create-custom",
		Method:      http.MethodPost,
		Path:        "/api/v1/assignments/custom",
		Summary:     "Parent Personalised Assignment Builder — create targeted homework for a linked child",
		Tags:        []string{"Assignments"},
		Security:    authSecurity,
	}, func(ctx context.Context, in *struct {
		reqguard.Credentials
		Body struct {
			StudentID     string   `json:"studentId" minLength:"1" maxLength:"64"`
			Title         string   `json:"title" minLength:"1" maxLength:"120"`
			DueDate       string   `json:"dueDate,omitempty" format:"date"`
			TemplateCodes []string `json:"templateCodes" minItems:"1" maxItems:"20"`
		}
	}) (*struct{ Body AssignmentSummary }, error) {
		p, err := h.resolve(ctx, in.Credentials, true)
		if err != nil {
			return nil, toHTTPError(err)
		}
		if p.Role != auth.RoleParent {
			return nil, huma.Error403Forbidden("parent account required")
		}
		summary, err := h.svc.CreateParentCustomAssignment(p.ID, p.DisplayName, in.Body.StudentID, in.Body.Title, in.Body.DueDate, in.Body.TemplateCodes)
		if err != nil {
			return nil, toHTTPError(err)
		}
		return &struct{ Body AssignmentSummary }{Body: summary}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "assignments-detail",
		Method:      http.MethodGet,
		Path:        "/api/v1/assignments/{id}",
		Summary:     "Open an assignment and inspect its questions",
		Tags:        []string{"Assignments"},
	}, func(ctx context.Context, in *struct {
		reqguard.Credentials
		ID string `path:"id" maxLength:"64"`
	}) (*struct{ Body AssignmentDetail }, error) {
		p, err := h.resolve(ctx, in.Credentials, false)
		if err != nil {
			return nil, toHTTPError(err)
		}
		detail, err := h.svc.GetDetail(p, in.ID, in.NormalizedLocale())
		if err != nil {
			return nil, toHTTPError(err)
		}
		return &struct{ Body AssignmentDetail }{Body: detail}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "assignments-submit",
		Method:      http.MethodPost,
		Path:        "/api/v1/assignments/{id}/submit",
		Summary:     "Submit an assignment question answer and award AC upon full completion",
		Tags:        []string{"Assignments"},
		Security:    authSecurity,
	}, func(ctx context.Context, in *struct {
		reqguard.Credentials
		ID   string `path:"id" maxLength:"64"`
		Body struct {
			QuestionIndex int               `json:"questionIndex" minimum:"1" maximum:"20"`
			Answers       map[string]string `json:"answers"`
		}
	}) (*struct {
		Body struct {
			Result     mathengine.GradeResult `json:"result"`
			Assignment AssignmentDetail       `json:"assignment"`
		}
	}, error) {
		p, err := h.resolve(ctx, in.Credentials, true)
		if err != nil {
			return nil, toHTTPError(err)
		}
		if p.Role != auth.RoleStudent {
			return nil, huma.Error403Forbidden("student account required")
		}
		res, detail, err := h.svc.SubmitQuestion(ctx, p.ID, in.ID, in.Body.QuestionIndex, in.Body.Answers, in.NormalizedLocale())
		if err != nil {
			return nil, toHTTPError(err)
		}
		out := &struct {
			Body struct {
				Result     mathengine.GradeResult `json:"result"`
				Assignment AssignmentDetail       `json:"assignment"`
			}
		}{}
		out.Body.Result = res
		out.Body.Assignment = detail
		return out, nil
	})
}

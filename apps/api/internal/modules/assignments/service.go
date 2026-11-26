package assignments

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
)

var (
	ErrAssignmentNotFound = errors.New("assignment not found")
	ErrNotLinkedChild     = errors.New("parent can only assign to linked children")
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
}

type Service struct {
	mu           sync.Mutex
	ledger       economy.Ledger
	records      map[string]*assignmentRecord
	parentLinks  map[string][]string // parentID -> []studentID
	nextID       int
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
	if parentID == "demo-parent" && studentID == "demo-pilot1" {
		return true
	}
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
		if r.studentID != studentID && studentID != "demo-pilot1" {
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
	return out
}

func (s *Service) CreateParentCustomAssignment(parentID, parentName, studentID, title, dueDate string, templateCodes []string) (AssignmentSummary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.isLinkedLocked(parentID, studentID) {
		return AssignmentSummary{}, ErrNotLinkedChild
	}
	if len(templateCodes) == 0 {
		return AssignmentSummary{}, fmt.Errorf("at least one question template is required")
	}
	s.nextID++
	id := fmt.Sprintf("asgn-%d", s.nextID)
	items := make([]itemSpec, 0, len(templateCodes))
	for i, code := range templateCodes {
		if _, ok := mathengine.GenerateByCode(code, int64(1000+i), "en"); !ok {
			return AssignmentSummary{}, fmt.Errorf("unknown question template %q", code)
		}
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

func (s *Service) GetDetail(assignmentID, locale string) (AssignmentDetail, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.records[assignmentID]
	if !ok {
		return AssignmentDetail{}, ErrAssignmentNotFound
	}
	return renderDetail(rec, locale), nil
}

func (s *Service) SubmitQuestion(ctx context.Context, studentID, assignmentID string, qIndex int, answers map[string]string, locale string) (mathengine.GradeResult, AssignmentDetail, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.records[assignmentID]
	if !ok {
		return mathengine.GradeResult{}, AssignmentDetail{}, ErrAssignmentNotFound
	}
	if qIndex < 1 || qIndex > len(rec.items) {
		return mathengine.GradeResult{}, AssignmentDetail{}, fmt.Errorf("invalid question index")
	}
	item := rec.items[qIndex-1]
	prob, _ := mathengine.GenerateByCode(item.templateCode, item.seed, locale)
	res := mathengine.Grade(prob, answers)
	rec.submitted[qIndex] = answers
	rec.correct[qIndex] = res.IsCorrect

	if len(rec.submitted) == len(rec.items) && !rec.completed {
		rec.completed = true
		if s.ledger != nil {
			_, _ = s.ledger.Apply(ctx, economy.MutationRequest{
				StudentID:      studentID,
				Currency:       economy.CurrencyAC,
				Delta:          int64(rec.rewardAC),
				SourceType:     "ASSIGNMENT_COMPLETE",
				SourceID:       rec.id,
				IdempotencyKey: fmt.Sprintf("asgn:%s:%s", rec.id, studentID),
			})
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

type authIn struct {
	CookieAccess  string `cookie:"pl_access"`
	Authorization string `header:"Authorization"`
	Locale        string `query:"locale" default:"en"`
}

func (h *Handler) resolveUser(ctx context.Context, in authIn) (string, string, auth.Role, string) {
	locale := in.Locale
	if locale != "id" {
		locale = "en"
	}
	if h.authSvc != nil {
		tok := in.CookieAccess
		if tok == "" && len(in.Authorization) > 7 {
			tok = in.Authorization[7:]
		}
		if tok != "" {
			if p, err := h.authSvc.ResolvePrincipal(ctx, tok); err == nil {
				return p.ID, p.DisplayName, p.Role, locale
			}
		}
	}
	return "demo-pilot1", "Budi Santoso", auth.RoleParent, locale
}

func (h *Handler) RegisterRoutes(_ chi.Router, api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "assignments-list",
		Method:      http.MethodGet,
		Path:        "/api/v1/assignments",
		Summary:     "List student assignments with status, year, and assigned-by filters",
		Tags:        []string{"Assignments"},
	}, func(ctx context.Context, in *struct {
		authIn
		Status     string `query:"status" default:"ALL"`
		AssignedBy string `query:"assignedBy" default:"ALL"`
		Year       int    `query:"year" default:"0"`
	}) (*struct {
		Body struct {
			Assignments []AssignmentSummary `json:"assignments"`
		}
	}, error) {
		id, _, _, _ := h.resolveUser(ctx, in.authIn)
		out := &struct {
			Body struct {
				Assignments []AssignmentSummary `json:"assignments"`
			}
		}{}
		out.Body.Assignments = h.svc.ListForStudent(id, in.Status, in.AssignedBy, in.Year)
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "assignments-create-custom",
		Method:      http.MethodPost,
		Path:        "/api/v1/assignments/custom",
		Summary:     "Parent Personalised Assignment Builder — create targeted homework for a linked child",
		Tags:        []string{"Assignments"},
	}, func(ctx context.Context, in *struct {
		authIn
		Body struct {
			StudentID     string   `json:"studentId" default:"demo-pilot1"`
			Title         string   `json:"title" minLength:"1"`
			DueDate       string   `json:"dueDate,omitempty"`
			TemplateCodes []string `json:"templateCodes" minItems:"1"`
		}
	}) (*struct{ Body AssignmentSummary }, error) {
		uid, name, role, _ := h.resolveUser(ctx, in.authIn)
		parentID := uid
		if role == auth.RoleStudent {
			// In demo mode without separate parent login, allow builder preview only when explicitly linked
			parentID = "demo-parent"
		}
		summary, err := h.svc.CreateParentCustomAssignment(parentID, name, in.Body.StudentID, in.Body.Title, in.Body.DueDate, in.Body.TemplateCodes)
		if err != nil {
			if errors.Is(err, ErrNotLinkedChild) {
				return nil, huma.Error403Forbidden(err.Error())
			}
			return nil, huma.Error400BadRequest(err.Error())
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
		authIn
		ID string `path:"id"`
	}) (*struct{ Body AssignmentDetail }, error) {
		_, _, _, locale := h.resolveUser(ctx, in.authIn)
		detail, err := h.svc.GetDetail(in.ID, locale)
		if err != nil {
			return nil, huma.Error404NotFound(err.Error())
		}
		return &struct{ Body AssignmentDetail }{Body: detail}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "assignments-submit",
		Method:      http.MethodPost,
		Path:        "/api/v1/assignments/{id}/submit",
		Summary:     "Submit an assignment question answer and award AC upon full completion",
		Tags:        []string{"Assignments"},
	}, func(ctx context.Context, in *struct {
		authIn
		ID   string `path:"id"`
		Body struct {
			QuestionIndex int               `json:"questionIndex" minimum:"1"`
			Answers       map[string]string `json:"answers"`
		}
	}) (*struct {
		Body struct {
			Result     mathengine.GradeResult `json:"result"`
			Assignment AssignmentDetail       `json:"assignment"`
		}
	}, error) {
		uid, _, _, locale := h.resolveUser(ctx, in.authIn)
		res, detail, err := h.svc.SubmitQuestion(ctx, uid, in.ID, in.Body.QuestionIndex, in.Body.Answers, locale)
		if err != nil {
			return nil, huma.Error400BadRequest(err.Error())
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

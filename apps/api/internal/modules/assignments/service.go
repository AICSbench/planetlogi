package assignments

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AICSbench/planetlogi/apps/api/internal/mathengine"
	"github.com/AICSbench/planetlogi/apps/api/internal/modules/auth"
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/economy"
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/reqguard"
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/state"
)

// Bounds for parent-built assignments; each item is rendered on every read.
const (
	MaxTitleLen  = 120
	MaxTemplates = 20
)

var (
	ErrAssignmentNotFound = errors.New("assignment not found")
	ErrNotLinkedChild     = errors.New("parent can only assign to linked children")
	ErrInvalidInput       = errors.New("invalid input")
	// ErrRewardUnavailable means the completion AC could not be credited; it can be retried.
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
	rewarded       bool
	persisted      bool
	requestID      string
}

type Service struct {
	mu          sync.Mutex
	ledger      economy.Ledger
	pool        *pgxpool.Pool
	store       state.Store
	loaded      map[string]bool
	records     map[string]*assignmentRecord
	parentLinks map[string][]string // parentID -> []studentID
}

func NewService(ledger economy.Ledger) *Service {
	svc := &Service{
		ledger:  ledger,
		records: make(map[string]*assignmentRecord),
		parentLinks: map[string][]string{
			"demo-parent": {"demo-pilot1"},
		},
		loaded: make(map[string]bool),
	}
	svc.seedDefaults("demo-pilot1")
	return svc
}

func NewServiceWithDeps(pool *pgxpool.Pool, store state.Store, ledger economy.Ledger) *Service {
	svc := NewService(ledger)
	svc.pool = pool
	svc.store = store
	return svc
}

type assignmentState struct {
	Records map[string]assignmentRecordDTO `json:"records"`
}

type assignmentRecordDTO struct {
	ID             string                    `json:"id"`
	StudentID      string                    `json:"studentId"`
	Title          string                    `json:"title"`
	AssignedByRole AssignedByRole            `json:"assignedByRole"`
	AssignedByName string                    `json:"assignedByName"`
	AcademicYear   int                       `json:"academicYear"`
	DueDate        string                    `json:"dueDate"`
	RewardAC       int                       `json:"rewardAc"`
	Items          []itemSpecDTO             `json:"items"`
	Submitted      map[int]map[string]string `json:"submitted"`
	Correct        map[int]bool              `json:"correct"`
	Completed      bool                      `json:"completed"`
	Rewarded       bool                      `json:"rewarded"`
	RequestID      string                    `json:"requestId,omitempty"`
}

type itemSpecDTO struct {
	TemplateCode string `json:"templateCode"`
	Seed         int64  `json:"seed"`
}

func toRecordDTO(r *assignmentRecord) assignmentRecordDTO {
	items := make([]itemSpecDTO, len(r.items))
	for i, item := range r.items {
		items[i] = itemSpecDTO{TemplateCode: item.templateCode, Seed: item.seed}
	}
	submitted := make(map[int]map[string]string, len(r.submitted))
	for q, answers := range r.submitted {
		copied := make(map[string]string, len(answers))
		for key, value := range answers {
			copied[key] = value
		}
		submitted[q] = copied
	}
	correct := make(map[int]bool, len(r.correct))
	for q, ok := range r.correct {
		correct[q] = ok
	}
	return assignmentRecordDTO{
		ID: r.id, StudentID: r.studentID, Title: r.title,
		AssignedByRole: r.assignedByRole, AssignedByName: r.assignedByName,
		AcademicYear: r.academicYear, DueDate: r.dueDate, RewardAC: r.rewardAC,
		Items: items, Submitted: submitted, Correct: correct,
		Completed: r.completed, Rewarded: r.rewarded, RequestID: r.requestID,
	}
}

func (d assignmentRecordDTO) toRecord() *assignmentRecord {
	items := make([]itemSpec, len(d.Items))
	for i, item := range d.Items {
		items[i] = itemSpec{templateCode: item.TemplateCode, seed: item.Seed}
	}
	submitted := d.Submitted
	if submitted == nil {
		submitted = make(map[int]map[string]string)
	}
	correct := d.Correct
	if correct == nil {
		correct = make(map[int]bool)
	}
	return &assignmentRecord{
		id: d.ID, studentID: d.StudentID, title: d.Title,
		assignedByRole: d.AssignedByRole, assignedByName: d.AssignedByName,
		academicYear: d.AcademicYear, dueDate: d.DueDate, rewardAC: d.RewardAC,
		items: items, submitted: submitted, correct: correct,
		completed: d.Completed, rewarded: d.Rewarded,
		persisted: true, requestID: d.RequestID,
	}
}

// ensureStudentLoaded reads the student's assignments once; the mutex is never
// held while the store performs a query.
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
	var st assignmentState
	if _, err := s.store.Load(ctx, studentID, "assignments", &st); err != nil {
		return err
	}
	s.mu.Lock()
	if s.loaded[studentID] {
		s.mu.Unlock()
		return nil
	}
	for _, dto := range st.Records {
		s.records[dto.ID] = dto.toRecord()
	}
	s.loaded[studentID] = true
	s.mu.Unlock()
	return nil
}

func mergeAssignmentState(currentRaw []byte, snapshot assignmentState) (any, error) {
	current := assignmentState{Records: make(map[string]assignmentRecordDTO)}
	if len(currentRaw) > 0 {
		if err := json.Unmarshal(currentRaw, &current); err != nil {
			return nil, err
		}
	}
	if current.Records == nil {
		current.Records = make(map[string]assignmentRecordDTO)
	}
	for id, snap := range snapshot.Records {
		if existing, ok := current.Records[id]; ok {
			merged := snap
			merged.Completed = existing.Completed || snap.Completed
			merged.Rewarded = existing.Rewarded || snap.Rewarded
			submitted := make(map[int]map[string]string, len(existing.Submitted)+len(snap.Submitted))
			for q, answers := range existing.Submitted {
				submitted[q] = answers
			}
			for q, answers := range snap.Submitted {
				submitted[q] = answers
			}
			correct := make(map[int]bool, len(existing.Correct)+len(snap.Correct))
			for q, ok := range existing.Correct {
				correct[q] = ok
			}
			for q, ok := range snap.Correct {
				correct[q] = ok
			}
			merged.Submitted = submitted
			merged.Correct = correct
			current.Records[id] = merged
		} else {
			current.Records[id] = snap
		}
	}
	return current, nil
}

func (s *Service) persistStudent(ctx context.Context, studentID string) error {
	if s.store == nil || !isUUID(studentID) {
		s.mu.Lock()
		for _, rec := range s.records {
			if rec.studentID == studentID {
				rec.persisted = true
			}
		}
		s.mu.Unlock()
		return nil
	}
	s.mu.Lock()
	st := assignmentState{Records: make(map[string]assignmentRecordDTO)}
	for id, rec := range s.records {
		if rec.studentID == studentID {
			st.Records[id] = toRecordDTO(rec)
		}
	}
	s.mu.Unlock()
	var err error
	if ms, ok := s.store.(state.MergeStore); ok {
		err = ms.SaveMerged(ctx, studentID, "assignments", func(current []byte) (any, error) {
			return mergeAssignmentState(current, st)
		})
	} else {
		err = s.store.Save(ctx, studentID, "assignments", st)
	}
	if err != nil {
		return err
	}
	s.mu.Lock()
	for _, rec := range s.records {
		if rec.studentID == studentID {
			rec.persisted = true
		}
	}
	s.mu.Unlock()
	return nil
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

func (s *Service) isLinked(ctx context.Context, parentID, studentID string) (bool, error) {
	if s.pool != nil {
		if !isUUID(parentID) || !isUUID(studentID) {
			return parentID == "demo-parent" && studentID == "demo-pilot1", nil
		}
		var ok bool
		err := s.pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM student_profiles
				WHERE id = $1::uuid AND parent_account_id = $2::uuid
			)
		`, studentID, parentID).Scan(&ok)
		return ok, err
	}
	if parentID == "demo-parent" && studentID == "demo-pilot1" {
		return true, nil
	}
	for _, cid := range s.parentLinks[parentID] {
		if cid == studentID {
			return true, nil
		}
	}
	return false, nil
}

func (s *Service) ListForStudent(ctx context.Context, studentID, statusFilter, assignedByFilter string, year int) ([]AssignmentSummary, error) {
	if err := s.ensureStudentLoaded(ctx, studentID); err != nil {
		return nil, err
	}
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
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].DueDate != out[j].DueDate {
			return out[i].DueDate < out[j].DueDate
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func (s *Service) CreateParentCustomAssignment(ctx context.Context, parentID, parentName, studentID, title, dueDate string, templateCodes []string, requestID string) (AssignmentSummary, error) {
	if title == "" || utf8.RuneCountInString(title) > MaxTitleLen {
		return AssignmentSummary{}, fmt.Errorf("%w: title must be 1..%d characters", ErrInvalidInput, MaxTitleLen)
	}
	if len(templateCodes) == 0 || len(templateCodes) > MaxTemplates {
		return AssignmentSummary{}, fmt.Errorf("%w: between 1 and %d question templates are required", ErrInvalidInput, MaxTemplates)
	}
	linked, err := s.isLinked(ctx, parentID, studentID)
	if err != nil {
		return AssignmentSummary{}, err
	}
	if !linked {
		return AssignmentSummary{}, ErrNotLinkedChild
	}
	if err := s.ensureStudentLoaded(ctx, studentID); err != nil {
		return AssignmentSummary{}, err
	}
	s.mu.Lock()
	summary, err := func() (AssignmentSummary, error) {
		if len(templateCodes) == 0 {
			return AssignmentSummary{}, fmt.Errorf("%w: at least one question template is required", ErrInvalidInput)
		}
		// A retried create request reuses its stable request ID, so an
		// ambiguous save outcome cannot produce a duplicate assignment.
		if requestID != "" {
			for _, rec := range s.records {
				if rec.studentID == studentID && rec.requestID == requestID {
					return toSummary(rec), nil
				}
			}
		}
		// UUIDs stay unique across restarts and replicas, so reward idempotency
		// keys never collide with a previous assignment.
		id := "asgn-" + uuid.NewString()
		items := make([]itemSpec, 0, len(templateCodes))
		for i, code := range templateCodes {
			if _, ok := mathengine.GenerateByCode(code, int64(1000+i), "en"); !ok {
				return AssignmentSummary{}, fmt.Errorf("%w: unknown question template %q", ErrInvalidInput, code)
			}
			items = append(items, itemSpec{templateCode: code, seed: time.Now().UnixNano() + int64(i*7919)})
		}
		if dueDate == "" {
			dueDate = time.Now().UTC().AddDate(0, 0, 7).Format("2006-01-02")
		} else if _, err := time.Parse("2006-01-02", dueDate); err != nil {
			return AssignmentSummary{}, fmt.Errorf("%w: dueDate must use YYYY-MM-DD", ErrInvalidInput)
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
			requestID:      requestID,
		}
		s.records[id] = rec
		return toSummary(rec), nil
	}()
	s.mu.Unlock()
	if err != nil {
		return AssignmentSummary{}, err
	}
	if err := s.persistStudent(ctx, studentID); err != nil {
		return AssignmentSummary{}, err
	}
	return summary, nil
}

func (s *Service) GetDetail(ctx context.Context, studentID, assignmentID, locale string) (AssignmentDetail, error) {
	if err := s.ensureStudentLoaded(ctx, studentID); err != nil {
		return AssignmentDetail{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.records[assignmentID]
	if !ok || rec.studentID != studentID {
		return AssignmentDetail{}, ErrAssignmentNotFound
	}
	return renderDetail(rec, locale), nil
}

func (s *Service) SubmitQuestion(ctx context.Context, studentID, assignmentID string, qIndex int, answers map[string]string, locale string) (mathengine.GradeResult, AssignmentDetail, error) {
	if err := reqguard.ValidateAnswers(answers); err != nil {
		return mathengine.GradeResult{}, AssignmentDetail{}, err
	}
	if err := s.ensureStudentLoaded(ctx, studentID); err != nil {
		return mathengine.GradeResult{}, AssignmentDetail{}, err
	}
	s.mu.Lock()
	var res mathengine.GradeResult
	var rewardAC int
	var rewardID string
	var needsReward bool
	detail, err := func() (AssignmentDetail, error) {
		rec, ok := s.records[assignmentID]
		if !ok || rec.studentID != studentID {
			return AssignmentDetail{}, ErrAssignmentNotFound
		}
		if qIndex < 1 || qIndex > len(rec.items) {
			return AssignmentDetail{}, fmt.Errorf("%w: invalid question index", ErrInvalidInput)
		}
		if _, done := rec.submitted[qIndex]; done {
			// Retry only flushes an unpersisted snapshot or an uncredited
			// reward; stored answers are never overwritten.
			if !rec.persisted || (rec.completed && !rec.rewarded && s.ledger != nil) {
				if rec.completed && !rec.rewarded && s.ledger != nil {
					needsReward = true
					rewardAC = rec.rewardAC
					rewardID = rec.id
				}
				return renderDetail(rec, locale), nil
			}
			return AssignmentDetail{}, fmt.Errorf("%w: question %d has already been submitted", ErrInvalidInput, qIndex)
		}
		item := rec.items[qIndex-1]
		prob, _ := mathengine.GenerateByCode(item.templateCode, item.seed, locale)
		res = mathengine.Grade(prob, answers)
		rec.submitted[qIndex] = answers
		rec.correct[qIndex] = res.IsCorrect
		rec.completed = len(rec.submitted) == len(rec.items)
		rec.persisted = false
		if rec.completed {
			if !rec.rewarded && s.ledger != nil {
				needsReward = true
				rewardAC = rec.rewardAC
				rewardID = rec.id
			} else if s.ledger == nil {
				rec.rewarded = true
			}
		}
		return renderDetail(rec, locale), nil
	}()
	s.mu.Unlock()
	if err != nil {
		return res, detail, err
	}

	var rewardErr error
	if needsReward {
		// The ledger can block on its own transaction; never hold the service
		// mutex across it. The idempotency key makes a retry safe.
		if _, err := s.ledger.Apply(ctx, economy.MutationRequest{
			StudentID:      studentID,
			Currency:       economy.CurrencyAC,
			Delta:          int64(rewardAC),
			SourceType:     "ASSIGNMENT_COMPLETE",
			SourceID:       rewardID,
			IdempotencyKey: fmt.Sprintf("asgn:%s:%s", rewardID, studentID),
		}); err != nil {
			rewardErr = fmt.Errorf("%w: %v", ErrRewardUnavailable, err)
		} else {
			s.mu.Lock()
			if rec, ok := s.records[rewardID]; ok {
				rec.rewarded = true
				detail = renderDetail(rec, locale)
			}
			s.mu.Unlock()
		}
	}

	// Persist submitted answers even when the reward failed so a retry can
	// award it without re-answering the questions.
	if err := s.persistStudent(ctx, studentID); err != nil {
		return res, detail, err
	}
	if rewardErr != nil {
		return res, detail, rewardErr
	}
	return res, detail, nil
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

type AuthIn struct {
	CookieAccess  string `cookie:"pl_access"`
	Authorization string `header:"Authorization"`
	Locale        string `query:"locale" default:"en"`
}

func (h *Handler) resolveUser(ctx context.Context, in AuthIn) (string, string, auth.Role, string, error) {
	locale := in.Locale
	if locale != "id" {
		locale = "en"
	}
	if h.authSvc == nil {
		return "demo-pilot1", "Budi Santoso", auth.RoleParent, locale, nil
	}
	tok := reqguard.Token(in.CookieAccess, in.Authorization)
	if tok != "" {
		if p, err := h.authSvc.ResolvePrincipal(ctx, tok); err == nil {
			return p.ID, p.DisplayName, p.Role, locale, nil
		}
	}
	return "", "", "", locale, auth.ErrInvalidCredentials
}

var authSecurity = []map[string][]string{{"accessCookie": {}}, {"bearerAuth": {}}}

// toHTTPError maps service errors to responses without echoing internal (e.g. database) errors.
func toHTTPError(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, ErrNotLinkedChild):
		return huma.Error403Forbidden(err.Error())
	case errors.Is(err, ErrAssignmentNotFound):
		return huma.Error404NotFound(err.Error())
	case errors.Is(err, ErrInvalidInput), errors.Is(err, reqguard.ErrInvalidAnswers):
		return huma.Error400BadRequest(err.Error())
	case errors.Is(err, ErrRewardUnavailable):
		slog.WarnContext(ctx, "assignment reward credit failed", "err", err)
		return huma.Error503ServiceUnavailable(ErrRewardUnavailable.Error())
	default:
		slog.ErrorContext(ctx, "assignments request failed", "err", err)
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
		Security:    authSecurity,
	}, func(ctx context.Context, in *struct {
		AuthIn
		Status     string `query:"status" default:"ALL"`
		AssignedBy string `query:"assignedBy" default:"ALL"`
		Year       int    `query:"year" default:"0"`
	}) (*struct {
		Body struct {
			Assignments []AssignmentSummary `json:"assignments"`
		}
	}, error) {
		id, _, _, _, err := h.resolveUser(ctx, in.AuthIn)
		if err != nil {
			return nil, huma.Error401Unauthorized("authentication required")
		}
		out := &struct {
			Body struct {
				Assignments []AssignmentSummary `json:"assignments"`
			}
		}{}
		assignments, err := h.svc.ListForStudent(ctx, id, in.Status, in.AssignedBy, in.Year)
		if err != nil {
			return nil, toHTTPError(ctx, err)
		}
		out.Body.Assignments = assignments
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
		AuthIn
		Body struct {
			StudentID     string   `json:"studentId" minLength:"1" maxLength:"64"`
			Title         string   `json:"title" minLength:"1" maxLength:"120"`
			DueDate       string   `json:"dueDate,omitempty"`
			TemplateCodes []string `json:"templateCodes" minItems:"1" maxItems:"20"`
			RequestID     string   `json:"requestId,omitempty"`
		}
	}) (*struct{ Body AssignmentSummary }, error) {
		uid, name, role, _, err := h.resolveUser(ctx, in.AuthIn)
		if err != nil {
			return nil, huma.Error401Unauthorized("authentication required")
		}
		if role != auth.RoleParent {
			return nil, huma.Error403Forbidden("parent access required")
		}
		summary, err := h.svc.CreateParentCustomAssignment(ctx, uid, name, in.Body.StudentID, in.Body.Title, in.Body.DueDate, in.Body.TemplateCodes, in.Body.RequestID)
		if err != nil {
			return nil, toHTTPError(ctx, err)
		}
		return &struct{ Body AssignmentSummary }{Body: summary}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "assignments-detail",
		Method:      http.MethodGet,
		Path:        "/api/v1/assignments/{id}",
		Summary:     "Open an assignment and inspect its questions",
		Tags:        []string{"Assignments"},
		Security:    authSecurity,
	}, func(ctx context.Context, in *struct {
		AuthIn
		ID string `path:"id"`
	}) (*struct{ Body AssignmentDetail }, error) {
		uid, _, _, locale, err := h.resolveUser(ctx, in.AuthIn)
		if err != nil {
			return nil, huma.Error401Unauthorized("authentication required")
		}
		detail, err := h.svc.GetDetail(ctx, uid, in.ID, locale)
		if err != nil {
			return nil, toHTTPError(ctx, err)
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
		AuthIn
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
		uid, _, _, locale, err := h.resolveUser(ctx, in.AuthIn)
		if err != nil {
			return nil, huma.Error401Unauthorized("authentication required")
		}
		res, detail, err := h.svc.SubmitQuestion(ctx, uid, in.ID, in.Body.QuestionIndex, in.Body.Answers, locale)
		if err != nil {
			return nil, toHTTPError(ctx, err)
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

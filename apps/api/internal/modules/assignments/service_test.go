package assignments

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/AICSbench/planetlogi/apps/api/internal/mathengine"
	"github.com/AICSbench/planetlogi/apps/api/internal/modules/auth"
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/economy"
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/reqguard/reqguardtest"
)

var (
	parent   = auth.Principal{ID: "par-1", Role: auth.RoleParent, DisplayName: "Budi"}
	student  = auth.Principal{ID: "stu-1", Role: auth.RoleStudent}
	intruder = auth.Principal{ID: "stu-2", Role: auth.RoleStudent}
)

func newTestService(t *testing.T) (*Service, *reqguardtest.FlakyLedger) {
	t.Helper()
	ledger := &reqguardtest.FlakyLedger{Inner: economy.NewMemoryLedger(
		economy.Wallet{StudentID: "stu-1", AstroCoins: 10, PilotLevel: 1},
		economy.Wallet{StudentID: "stu-2", AstroCoins: 10, PilotLevel: 1},
	)}
	svc := NewService(ledger)
	svc.LinkParentChild("par-1", "stu-1")
	return svc, ledger
}

func correctAnswers(t *testing.T, q AssignmentQuestionView) map[string]string {
	t.Helper()
	prob, ok := mathengine.GenerateByCode(q.Question.TemplateCode, q.Question.Seed, "en")
	if !ok {
		t.Fatalf("unknown template %s", q.Question.TemplateCode)
	}
	return prob.Solution.CorrectAnswers
}

func TestParentCustomAssignmentAndCompletionReward(t *testing.T) {
	svc, ledger := newTestService(t)

	// Unlinked parent MUST be blocked
	if _, err := svc.CreateParentCustomAssignment("par-other", "Other", "stu-1", "Test", "", []string{"g1_add_within_20"}); err != ErrNotLinkedChild {
		t.Fatalf("expected ErrNotLinkedChild, got %v", err)
	}

	created, err := svc.CreateParentCustomAssignment("par-1", "Budi", "stu-1", "Weekend Addition Drill", "2026-12-01", []string{"g1_add_within_20", "g1_sub_within_20"})
	if err != nil || created.QuestionCount != 2 || created.AssignedByRole != AssignedByParent {
		t.Fatalf("unexpected created assignment: %+v err=%v", created, err)
	}

	detail, err := svc.GetDetail(student, created.ID, "en")
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range detail.Questions {
		if _, detail, err = svc.SubmitQuestion(context.Background(), "stu-1", created.ID, q.Index, correctAnswers(t, q), "en"); err != nil {
			t.Fatal(err)
		}
	}
	if detail.Summary.Status != "COMPLETE" || detail.Summary.Score != 2 {
		t.Fatalf("expected COMPLETE with score 2/2, got %+v", detail.Summary)
	}
	if got := ledger.Credited("stu-1", economy.CurrencyAC); got != 5 {
		t.Fatalf("expected exactly +5 AC credited to the owner, got %d", got)
	}
}

func TestAssignmentOwnershipIsEnforced(t *testing.T) {
	svc, ledger := newTestService(t)
	created, err := svc.CreateParentCustomAssignment("par-1", "Budi", "stu-1", "Drill", "", []string{"g1_add_within_20"})
	if err != nil {
		t.Fatal(err)
	}

	// Another student can neither read nor answer it (and learns nothing about its existence).
	if _, err := svc.GetDetail(intruder, created.ID, "en"); !errors.Is(err, ErrAssignmentNotFound) {
		t.Fatalf("intruder read: expected ErrAssignmentNotFound, got %v", err)
	}
	if _, _, err := svc.SubmitQuestion(context.Background(), "stu-2", created.ID, 1, map[string]string{}, "en"); !errors.Is(err, ErrAssignmentNotFound) {
		t.Fatalf("intruder submit: expected ErrAssignmentNotFound, got %v", err)
	}
	if got := ledger.Credited("stu-2", economy.CurrencyAC); got != 0 {
		t.Fatalf("intruder must not earn AC, got %d", got)
	}

	// The linked parent may view; an unlinked parent may not.
	if _, err := svc.GetDetail(parent, created.ID, "en"); err != nil {
		t.Fatalf("linked parent should view: %v", err)
	}
	if _, err := svc.GetDetail(auth.Principal{ID: "par-2", Role: auth.RoleParent}, created.ID, "en"); !errors.Is(err, ErrAssignmentNotFound) {
		t.Fatalf("unlinked parent: expected ErrAssignmentNotFound, got %v", err)
	}

	// The guest identity only sees its own seeded record, never other students' work.
	for _, a := range svc.ListForStudent("demo-pilot1", "ALL", "ALL", 0) {
		if a.ID == created.ID {
			t.Fatal("guest listing leaked another student's assignment")
		}
	}
	if list := svc.ListForStudent("stu-1", "ALL", "ALL", 0); len(list) != 1 || list[0].ID != created.ID {
		t.Fatalf("owner listing wrong: %+v", list)
	}
}

func TestAnsweredQuestionCannotBeOverwritten(t *testing.T) {
	svc, _ := newTestService(t)
	created, _ := svc.CreateParentCustomAssignment("par-1", "Budi", "stu-1", "Drill", "", []string{"g1_add_within_20", "g1_sub_within_20"})
	detail, _ := svc.GetDetail(student, created.ID, "en")

	first, _, err := svc.SubmitQuestion(context.Background(), "stu-1", created.ID, 1, map[string]string{"answer": "-1"}, "en")
	if err != nil || first.IsCorrect {
		t.Fatalf("expected a wrong first answer, got %+v err=%v", first, err)
	}
	// After the worked solution is revealed, re-submitting the correct answer must not change the score.
	again, after, err := svc.SubmitQuestion(context.Background(), "stu-1", created.ID, 1, correctAnswers(t, detail.Questions[0]), "en")
	if err != nil || again.IsCorrect || after.Summary.Score != 0 || after.Questions[0].SubmittedAnswer["answer"] != "-1" {
		t.Fatalf("answered question was overwritten: res=%+v summary=%+v", again, after.Summary)
	}
}

func TestFailedRewardIsRetried(t *testing.T) {
	svc, ledger := newTestService(t)
	created, _ := svc.CreateParentCustomAssignment("par-1", "Budi", "stu-1", "Drill", "", []string{"g1_add_within_20"})

	ledger.Fail = true
	if _, _, err := svc.SubmitQuestion(context.Background(), "stu-1", created.ID, 1, map[string]string{"answer": "1"}, "en"); !errors.Is(err, ErrRewardUnavailable) {
		t.Fatalf("expected ErrRewardUnavailable, got %v", err)
	}
	ledger.Fail = false
	if _, _, err := svc.SubmitQuestion(context.Background(), "stu-1", created.ID, 1, map[string]string{"answer": "1"}, "en"); err != nil {
		t.Fatalf("retry failed: %v", err)
	}
	if got := ledger.Credited("stu-1", economy.CurrencyAC); got != 5 {
		t.Fatalf("expected +5 AC after retry, got %d", got)
	}
	// Further submits never pay twice.
	_, _, _ = svc.SubmitQuestion(context.Background(), "stu-1", created.ID, 1, map[string]string{"answer": "1"}, "en")
	if got := ledger.Credited("stu-1", economy.CurrencyAC); got != 5 {
		t.Fatalf("reward paid twice: %d", got)
	}
}

func TestCreateValidatesInput(t *testing.T) {
	svc, _ := newTestService(t)
	tooMany := make([]string, maxTemplatesPerAs+1)
	for i := range tooMany {
		tooMany[i] = "g1_add_within_20"
	}
	cases := map[string]func() error{
		"empty title": func() error {
			_, err := svc.CreateParentCustomAssignment("par-1", "B", "stu-1", "", "", []string{"g1_add_within_20"})
			return err
		},
		"long title": func() error {
			_, err := svc.CreateParentCustomAssignment("par-1", "B", "stu-1", strings.Repeat("x", maxTitleLen+1), "", []string{"g1_add_within_20"})
			return err
		},
		"too many templates": func() error {
			_, err := svc.CreateParentCustomAssignment("par-1", "B", "stu-1", "t", "", tooMany)
			return err
		},
		"bad due date": func() error {
			_, err := svc.CreateParentCustomAssignment("par-1", "B", "stu-1", "t", "tomorrow", []string{"g1_add_within_20"})
			return err
		},
		"unknown template": func() error {
			_, err := svc.CreateParentCustomAssignment("par-1", "B", "stu-1", "t", "", []string{"nope"})
			return err
		},
	}
	for name, fn := range cases {
		if err := fn(); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("%s: expected ErrInvalidInput, got %v", name, err)
		}
	}
}

// Regression for the AC-farming path: before the fix a student session was mapped to the
// demo parent, could create assignments, and was paid for submitting any (even empty) answers.
func TestHTTPRoleAndAuthChecks(t *testing.T) {
	authFx := reqguardtest.New(t, parent, student, intruder)
	svc, ledger := newTestService(t)
	_, api := humatest.New(t)
	NewHandler(svc, authFx.Service).RegisterRoutes(nil, api)

	createBody := map[string]any{"studentId": "stu-1", "title": "Drill", "templateCodes": []string{"g1_add_within_20"}}

	if resp := api.Post("/api/v1/assignments/custom", authFx.BearerHeader(t, "stu-2"), createBody); resp.Code != http.StatusForbidden {
		t.Fatalf("student create: expected 403, got %d", resp.Code)
	}
	if resp := api.Post("/api/v1/assignments/custom", createBody); resp.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous create: expected 401, got %d", resp.Code)
	}
	if resp := api.Post("/api/v1/assignments/custom", "Authorization: Bearer forged", createBody); resp.Code != http.StatusUnauthorized {
		t.Fatalf("invalid token create: expected 401, got %d", resp.Code)
	}

	resp := api.Post("/api/v1/assignments/custom", authFx.BearerHeader(t, "par-1"), createBody)
	if resp.Code != http.StatusOK {
		t.Fatalf("linked parent create: expected 200, got %d %s", resp.Code, resp.Body.String())
	}
	id := svc.ListForStudent("stu-1", "ALL", "ALL", 0)[0].ID

	submit := map[string]any{"questionIndex": 1, "answers": map[string]string{"answer": "1"}}
	if resp := api.Post("/api/v1/assignments/"+id+"/submit", authFx.BearerHeader(t, "stu-2"), submit); resp.Code != http.StatusNotFound {
		t.Fatalf("intruder submit: expected 404, got %d", resp.Code)
	}
	if resp := api.Get("/api/v1/assignments/"+id, authFx.BearerHeader(t, "stu-2")); resp.Code != http.StatusNotFound {
		t.Fatalf("intruder detail: expected 404, got %d", resp.Code)
	}
	if resp := api.Post("/api/v1/assignments/"+id+"/submit", submit); resp.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous submit: expected 401, got %d", resp.Code)
	}
	if resp := api.Post("/api/v1/assignments/"+id+"/submit", authFx.BearerHeader(t, "stu-1"), submit); resp.Code != http.StatusOK {
		t.Fatalf("owner submit: expected 200, got %d %s", resp.Code, resp.Body.String())
	}
	if ledger.Credited("stu-2", economy.CurrencyAC) != 0 || ledger.Credited("stu-1", economy.CurrencyAC) != 5 {
		t.Fatalf("AC went to the wrong account: %+v", ledger.Applied)
	}
}

package assignments

import (
	"context"
	"testing"

	"github.com/AICSbench/planetlogi/apps/api/internal/mathengine"
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/economy"
)

func TestParentCustomAssignmentAndCompletionReward(t *testing.T) {
	ledger := economy.NewMemoryLedger(economy.Wallet{StudentID: "stu-1", AstroCoins: 10, PilotLevel: 1})
	svc := NewService(ledger)
	svc.LinkParentChild("par-1", "stu-1")

	// Unlinked parent MUST be blocked
	if _, err := svc.CreateParentCustomAssignment("par-other", "Other", "stu-1", "Test", "", []string{"g1_add_within_20"}); err != ErrNotLinkedChild {
		t.Fatalf("expected ErrNotLinkedChild, got %v", err)
	}

	// Linked parent creates custom 2-question assignment
	created, err := svc.CreateParentCustomAssignment("par-1", "Budi", "stu-1", "Weekend Addition Drill", "2026-12-01", []string{"g1_add_within_20", "g1_sub_within_20"})
	if err != nil || created.QuestionCount != 2 || created.AssignedByRole != AssignedByParent {
		t.Fatalf("unexpected created assignment: %+v err=%v", created, err)
	}

	detail, err := svc.GetDetail(created.ID, "en")
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range detail.Questions {
		prob, _ := mathengine.GenerateByCode(q.Question.TemplateCode, q.Question.Seed, "en")
		if _, detail, err = svc.SubmitQuestion(context.Background(), "stu-1", created.ID, q.Index, prob.Solution.CorrectAnswers, "en"); err != nil {
			t.Fatal(err)
		}
	}
	if detail.Summary.Status != "COMPLETE" || detail.Summary.Score != 2 {
		t.Fatalf("expected COMPLETE with score 2/2, got %+v", detail.Summary)
	}
}

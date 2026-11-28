package assignments

import (
	"context"
	"testing"

	"github.com/AICSbench/planetlogi/apps/api/internal/mathengine"
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/economy"
)

const stuUUID = "66666666-6666-6666-6666-666666666666"

func acCredited(l *recordingLedger) int64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	var total int64
	for _, r := range l.applied {
		if r.Currency == economy.CurrencyAC {
			total += r.Delta
		}
	}
	return total
}

// Completion AC must never be credited before the final answer is durable.
func TestCompletionRewardCreditedOnlyAfterAnswerIsDurable(t *testing.T) {
	ctx := context.Background()
	store := newAssignmentMemoryStore()
	ledger := &recordingLedger{}
	svc := NewServiceWithDeps(nil, store, ledger)
	svc.LinkParentChild("par-1", stuUUID)

	created, err := svc.CreateParentCustomAssignment(ctx, "par-1", "Budi", stuUUID, "Drill", "", []string{"g1_add_within_20"}, "req-order")
	if err != nil {
		t.Fatal(err)
	}
	detail, err := svc.GetDetail(ctx, stuUUID, created.ID, "en")
	if err != nil {
		t.Fatal(err)
	}
	q := detail.Questions[0]
	prob, _ := mathengine.GenerateByCode(q.Question.TemplateCode, q.Question.Seed, "en")

	store.failNext = true
	if _, _, err := svc.SubmitQuestion(ctx, stuUUID, created.ID, 1, prob.Solution.CorrectAnswers, "en"); err == nil {
		t.Fatal("expected the failed save to surface")
	}
	if got := acCredited(ledger); got != 0 {
		t.Fatalf("AC credited before the final answer was durable: %d", got)
	}
	if _, _, err := svc.SubmitQuestion(ctx, stuUUID, created.ID, 1, prob.Solution.CorrectAnswers, "en"); err != nil {
		t.Fatalf("retry: %v", err)
	}
	if got := acCredited(ledger); got != 5 {
		t.Fatalf("expected exactly +5 AC after the retry, got %d", got)
	}
}

// Two instances creating the same request: the second must answer with the durable record's ID
// (not its own local, never-stored ID) and must be able to open it.
func TestConcurrentCreateReturnsDurableID(t *testing.T) {
	ctx := context.Background()
	store := newAssignmentMemoryStore()
	a := NewServiceWithDeps(nil, store, nil)
	b := NewServiceWithDeps(nil, store, nil)
	for _, svc := range []*Service{a, b} {
		svc.LinkParentChild("par-1", stuUUID)
	}
	// Instance B loads the student before A's create lands.
	if _, err := b.GetDetail(ctx, stuUUID, "asgn-none", "en"); err == nil {
		t.Fatal("expected not found")
	}

	first, err := a.CreateParentCustomAssignment(ctx, "par-1", "Budi", stuUUID, "Drill", "", []string{"g1_add_within_20"}, "req-same")
	if err != nil {
		t.Fatal(err)
	}
	second, err := b.CreateParentCustomAssignment(ctx, "par-1", "Budi", stuUUID, "Drill", "", []string{"g1_add_within_20"}, "req-same")
	if err != nil {
		t.Fatal(err)
	}
	if second.ID != first.ID {
		t.Fatalf("instance B returned a non-durable ID: %s, durable is %s", second.ID, first.ID)
	}
	if _, err := b.GetDetail(ctx, stuUUID, first.ID, "en"); err != nil {
		t.Fatalf("instance B cannot open the durable assignment: %v", err)
	}
	fresh := NewServiceWithDeps(nil, store, nil)
	list, err := fresh.ListForStudent(ctx, stuUUID, "ALL", "ALL", 0)
	if err != nil || len(list) != 1 || list[0].ID != first.ID {
		t.Fatalf("durable state should hold exactly the one assignment %s: %+v err=%v", first.ID, list, err)
	}
}

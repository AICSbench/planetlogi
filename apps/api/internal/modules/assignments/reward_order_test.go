package assignments

import (
	"context"
	"encoding/json"
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

// Load-time reconciliation must also persist completion before crediting.
func TestLoadReconciliationPersistsBeforeCredit(t *testing.T) {
	ctx := context.Background()
	store := newAssignmentMemoryStore()
	ledger := &recordingLedger{}
	svc := NewServiceWithDeps(nil, store, ledger)
	svc.LinkParentChild("par-1", stuUUID)

	created, err := svc.CreateParentCustomAssignment(ctx, "par-1", "Budi", stuUUID, "Drill", "", []string{"g1_add_within_20"}, "req-load-order")
	if err != nil {
		t.Fatal(err)
	}
	detail, err := svc.GetDetail(ctx, stuUUID, created.ID, "en")
	if err != nil {
		t.Fatal(err)
	}
	q := detail.Questions[0]
	prob, _ := mathengine.GenerateByCode(q.Question.TemplateCode, q.Question.Seed, "en")

	// The final answer reaches memory, but its save fails before the reward.
	store.failNext = true
	if _, _, err := svc.SubmitQuestion(ctx, stuUUID, created.ID, 1, prob.Solution.CorrectAnswers, "en"); err == nil {
		t.Fatal("expected the failed save to surface")
	}
	if got := acCredited(ledger); got != 0 {
		t.Fatalf("AC credited before durable completion: %d", got)
	}

	// A read merges the pending completion. While storage is down it must
	// neither fail nor credit the reward.
	store.failNext = true
	if _, err := svc.ListForStudent(ctx, stuUUID, "ALL", "ALL", 0); err != nil {
		t.Fatalf("read failed while completion was pending: %v", err)
	}
	if got := acCredited(ledger); got != 0 {
		t.Fatalf("AC credited before completion was durable: %d", got)
	}

	if _, err := svc.ListForStudent(ctx, stuUUID, "ALL", "ALL", 0); err != nil {
		t.Fatal(err)
	}
	if got := acCredited(ledger); got != 5 {
		t.Fatalf("expected exactly +5 AC after durable reconciliation, got %d", got)
	}
}

func TestMergeAssignmentStateAliasKeepsDurableIdentity(t *testing.T) {
	current := assignmentState{Records: map[string]assignmentRecordDTO{
		"durable-id": {
			ID: "durable-id", StudentID: stuUUID, RequestID: "req-alias", Title: "Durable",
			Items: []itemSpecDTO{{TemplateCode: "g1_add_within_20", Seed: 11}},
		},
	}}
	raw, _ := json.Marshal(current)
	snapshot := assignmentState{Records: map[string]assignmentRecordDTO{
		"local-id": {
			ID: "local-id", StudentID: stuUUID, RequestID: "req-alias", Title: "Local",
			Items: []itemSpecDTO{{TemplateCode: "g1_sub_within_20", Seed: 22}},
		},
	}}
	merged, aliases, err := mergeAssignmentStateAliases(raw, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if aliases["local-id"] != "durable-id" {
		t.Fatalf("expected local alias to durable ID, got %+v", aliases)
	}
	rec, ok := merged.Records["durable-id"]
	if !ok {
		t.Fatal("durable record key was lost")
	}
	if rec.ID != "durable-id" || rec.Title != "Durable" ||
		rec.Items[0].TemplateCode != "g1_add_within_20" || rec.Items[0].Seed != 11 {
		t.Fatalf("alias merge replaced immutable assignment fields: %+v", rec)
	}
	if _, exists := merged.Records["local-id"]; exists {
		t.Fatal("local duplicate survived the alias merge")
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
	firstDetail, err := a.GetDetail(ctx, stuUUID, first.ID, "en")
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
	after, err := b.GetDetail(ctx, stuUUID, first.ID, "en")
	if err != nil {
		t.Fatalf("instance B cannot open the durable assignment: %v", err)
	}
	if after.Questions[0].Question.TemplateCode != firstDetail.Questions[0].Question.TemplateCode ||
		after.Questions[0].Question.Seed != firstDetail.Questions[0].Question.Seed {
		t.Fatalf("durable question changed after alias merge: before=%+v after=%+v", firstDetail.Questions[0].Question, after.Questions[0].Question)
	}
	fresh := NewServiceWithDeps(nil, store, nil)
	list, err := fresh.ListForStudent(ctx, stuUUID, "ALL", "ALL", 0)
	if err != nil || len(list) != 1 || list[0].ID != first.ID {
		t.Fatalf("durable state should hold exactly the one assignment %s: %+v err=%v", first.ID, list, err)
	}
}

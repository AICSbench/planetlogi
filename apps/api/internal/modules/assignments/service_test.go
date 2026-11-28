package assignments

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/AICSbench/planetlogi/apps/api/internal/mathengine"
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/economy"
)

// recordingLedger captures reward mutations so tests can assert the wallet
// effect, not just an in-memory completion flag.
type recordingLedger struct {
	mu       sync.Mutex
	applied  []economy.MutationRequest
	failNext bool
}

func (l *recordingLedger) Apply(_ context.Context, req economy.MutationRequest) (economy.MutationResult, error) {
	return l.apply(req)
}

func (l *recordingLedger) ApplyBatch(_ context.Context, reqs []economy.MutationRequest) ([]economy.MutationResult, error) {
	out := make([]economy.MutationResult, 0, len(reqs))
	for _, req := range reqs {
		res, err := l.apply(req)
		if err != nil {
			return nil, err
		}
		out = append(out, res)
	}
	return out, nil
}

func (l *recordingLedger) apply(req economy.MutationRequest) (economy.MutationResult, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.failNext {
		l.failNext = false
		return economy.MutationResult{}, errors.New("ledger temporarily unavailable")
	}
	l.applied = append(l.applied, req)
	return economy.MutationResult{BalanceAfter: req.Delta}, nil
}

func (l *recordingLedger) totalAC() int64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	var total int64
	for _, req := range l.applied {
		if req.Currency == economy.CurrencyAC {
			total += req.Delta
		}
	}
	return total
}

func TestParentCustomAssignmentAndCompletionReward(t *testing.T) {
	ledger := &recordingLedger{}
	svc := NewService(ledger)
	svc.LinkParentChild("par-1", "stu-1")

	// Unlinked parent MUST be blocked
	if _, err := svc.CreateParentCustomAssignment(context.Background(), "par-other", "Other", "stu-1", "Test", "", []string{"g1_add_within_20"}); err != ErrNotLinkedChild {
		t.Fatalf("expected ErrNotLinkedChild, got %v", err)
	}

	// Invalid due dates are rejected
	if _, err := svc.CreateParentCustomAssignment(context.Background(), "par-1", "Budi", "stu-1", "Test", "tomorrow", []string{"g1_add_within_20"}); err == nil {
		t.Fatal("expected invalid dueDate to be rejected")
	}

	// Linked parent creates custom 2-question assignment
	created, err := svc.CreateParentCustomAssignment(context.Background(), "par-1", "Budi", "stu-1", "Weekend Addition Drill", "2026-12-01", []string{"g1_add_within_20", "g1_sub_within_20"})
	if err != nil || created.QuestionCount != 2 || created.AssignedByRole != AssignedByParent {
		t.Fatalf("unexpected created assignment: %+v err=%v", created, err)
	}
	if created.ID == "asgn-1" {
		t.Fatalf("custom assignment reused the seeded ID: %s", created.ID)
	}

	detail, err := svc.GetDetail(context.Background(), "stu-1", created.ID, "en")
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range detail.Questions {
		prob, _ := mathengine.GenerateByCode(q.Question.TemplateCode, q.Question.Seed, "en")
		if q.Index == len(detail.Questions) {
			ledger.failNext = true
			if _, _, err = svc.SubmitQuestion(context.Background(), "stu-1", created.ID, q.Index, prob.Solution.CorrectAnswers, "en"); err == nil {
				t.Fatal("expected the failed reward to surface as an error")
			}
			// Retrying the final question after a ledger failure must credit
			// the reward exactly once without overwriting answers.
			if _, detail, err = svc.SubmitQuestion(context.Background(), "stu-1", created.ID, q.Index, prob.Solution.CorrectAnswers, "en"); err != nil {
				t.Fatalf("retry after ledger failure: %v", err)
			}
			continue
		}
		if _, detail, err = svc.SubmitQuestion(context.Background(), "stu-1", created.ID, q.Index, prob.Solution.CorrectAnswers, "en"); err != nil {
			t.Fatal(err)
		}
	}
	if detail.Summary.Status != "COMPLETE" || detail.Summary.Score != 2 {
		t.Fatalf("expected COMPLETE with score 2/2, got %+v", detail.Summary)
	}
	if got := ledger.totalAC(); got != 5 {
		t.Fatalf("expected exactly +5 AC credited once, got %d", got)
	}

	// A second submission for an answered question is rejected.
	q := detail.Questions[0]
	prob, _ := mathengine.GenerateByCode(q.Question.TemplateCode, q.Question.Seed, "en")
	if _, _, err := svc.SubmitQuestion(context.Background(), "stu-1", created.ID, 1, prob.Solution.CorrectAnswers, "en"); err == nil {
		t.Fatal("expected duplicate question submission to be rejected")
	}
}

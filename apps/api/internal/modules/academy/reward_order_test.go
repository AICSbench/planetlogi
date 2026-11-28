package academy

import (
	"context"
	"errors"
	"testing"

	"github.com/AICSbench/planetlogi/apps/api/internal/mathengine"
)

// failingMergeStore fails the next SaveMerged when failNext is set.
type failingMergeStore struct {
	*memoryStore
	failNext bool
}

func (f *failingMergeStore) SaveMerged(ctx context.Context, studentID, module string, merge func([]byte) (any, error)) error {
	if f.failNext {
		f.failNext = false
		return errors.New("store temporarily unavailable")
	}
	return f.memoryStore.SaveMerged(ctx, studentID, module, merge)
}

// The Star Scroll must never be credited before the solved answers that earned it are durable:
// if that save fails, no credit happens and the next check credits exactly once.
func TestStarScrollCreditedOnlyAfterProgressIsDurable(t *testing.T) {
	const stu = "55555555-5555-5555-5555-555555555555"
	ctx := context.Background()
	store := &failingMergeStore{memoryStore: newMemoryStore()}
	ledger := &recordingLedger{}
	svc := NewServiceWithStore(store, ledger)

	lesson, err := svc.GetLesson(ctx, stu, "g1-wn-l1", "en")
	if err != nil {
		t.Fatal(err)
	}
	answers := map[int]map[string]string{}
	for _, item := range lesson.DoAndLearn {
		prob, _ := mathengine.GenerateByCode(item.Question.TemplateCode, item.Question.Seed, "en")
		answers[item.Index] = prob.Solution.CorrectAnswers
	}
	for idx := 1; idx <= 3; idx++ {
		if _, _, err := svc.CheckAnswer(ctx, stu, "g1-wn-l1", "en", idx, answers[idx]); err != nil {
			t.Fatal(err)
		}
	}

	store.failNext = true
	if _, _, err := svc.CheckAnswer(ctx, stu, "g1-wn-l1", "en", 4, answers[4]); err == nil {
		t.Fatal("expected the failed progress save to surface")
	}
	if got := ledger.scrolls(); got != 0 {
		t.Fatalf("Star Scroll credited before the solved answers were durable: %d", got)
	}

	if _, detail, err := svc.CheckAnswer(ctx, stu, "g1-wn-l1", "en", 4, answers[4]); err != nil || !detail.ScrollEarned {
		t.Fatalf("retry should credit the scroll: earned=%v err=%v", detail.ScrollEarned, err)
	}
	if got := ledger.scrolls(); got != 1 {
		t.Fatalf("expected exactly one Star Scroll, got %d", got)
	}
}

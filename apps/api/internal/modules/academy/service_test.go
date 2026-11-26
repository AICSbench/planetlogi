package academy

import (
	"context"
	"testing"

	"github.com/AICSbench/planetlogi/apps/api/internal/mathengine"
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/economy"
)

func TestSpaceAcademySaveTryAgainAndStarScroll(t *testing.T) {
	ledger := economy.NewMemoryLedger(economy.Wallet{StudentID: "stu-1", PilotLevel: 1})
	svc := NewService(ledger)

	topics := svc.ListTopics("stu-1", 0, "en")
	if len(topics) < 12 {
		t.Fatalf("expected >=12 topics across G1-G6, got %d", len(topics))
	}

	// Save draft persists across reloads
	saved, err := svc.SaveDraft("stu-1", "g1-wn-l1", "en", 1, map[string]string{"answer": "9"})
	if err != nil || saved.DoAndLearn[0].DraftAnswer["answer"] != "9" {
		t.Fatalf("draft did not persist: err=%v draft=%v", err, saved.DoAndLearn[0].DraftAnswer)
	}

	// Wrong check marks LastWrong=true and allows Try Again!
	wrongRes, afterWrong, err := svc.CheckAnswer(context.Background(), "stu-1", "g1-wn-l1", "en", 1, map[string]string{"answer": "-999"})
	if err != nil || wrongRes.IsCorrect || !afterWrong.DoAndLearn[0].LastWrong {
		t.Fatalf("expected wrong check to allow Try Again, got res=%+v", wrongRes)
	}

	// Solving all 4 questions awards exactly 1 Star Scroll
	for idx := 1; idx <= 4; idx++ {
		q := afterWrong.DoAndLearn[idx-1].Question
		prob, _ := mathengine.GenerateByCode(q.TemplateCode, q.Seed, "en")
		if _, afterWrong, err = svc.CheckAnswer(context.Background(), "stu-1", "g1-wn-l1", "en", idx, prob.Solution.CorrectAnswers); err != nil {
			t.Fatalf("check q%d failed: %v", idx, err)
		}
	}

	if !afterWrong.ScrollEarned {
		t.Fatalf("expected Star Scroll awarded after solving 4/4 Do & Learn questions")
	}

	// Toggle Self-Check pushes completion to 100%
	_, _ = svc.ToggleSelfCheck("stu-1", "g1-wn-l1", "count_20", true, "en")
	final, _ := svc.ToggleSelfCheck("stu-1", "g1-wn-l1", "numline_add", true, "en")
	if final.CompletionPct != 100 {
		t.Fatalf("expected 100%% completion, got %d%%", final.CompletionPct)
	}
}

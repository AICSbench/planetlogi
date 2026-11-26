package astroquiz

import (
	"context"
	"testing"

	"github.com/AICSbench/planetlogi/apps/api/internal/mathengine"
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/economy"
)

func TestAstroQuizHiddenSolutionsAndWeeklySupplyPodCap(t *testing.T) {
	ledger := economy.NewMemoryLedger(economy.Wallet{StudentID: "stu-1", AstroCoins: 10, PilotLevel: 1})
	svc := NewService(ledger)

	papers, claimed := svc.ListPapers("stu-1", 0, "ALL", "en")
	if len(papers) != 18 || claimed {
		t.Fatalf("expected 18 papers across 3 categories x 6 grades and unclaimed pod, got %d / %v", len(papers), claimed)
	}

	// Open paper -> solutions MUST stay hidden before full submit
	opened, err := svc.OpenPaper("stu-1", "G1-ESS-01", "en")
	if err != nil {
		t.Fatal(err)
	}
	if opened.Questions[0].WorkedSolution != nil {
		t.Fatalf("solutions must remain hidden while IN_PROGRESS")
	}

	// Prepare correct answers and submit paper 1 -> awards 1 Weekly Supply Pod (+5 AC)
	drafts := map[int]map[string]string{}
	for _, pq := range opened.Questions {
		prob, _ := mathengine.GenerateByCode(pq.Question.TemplateCode, pq.Question.Seed, "en")
		drafts[pq.Index] = prob.Solution.CorrectAnswers
	}
	sub1, err := svc.SubmitPaper(context.Background(), "stu-1", "G1-ESS-01", "en", 120, drafts)
	if err != nil {
		t.Fatal(err)
	}
	if sub1.Score != 10 || sub1.Questions[0].WorkedSolution == nil || !sub1.SupplyPodAwarded {
		t.Fatalf("expected 10/10, revealed solutions, and Supply Pod awarded: %+v", sub1)
	}

	// Submitting a 2nd paper in the same ISO week MUST NOT award a 2nd Supply Pod
	sub2, err := svc.SubmitPaper(context.Background(), "stu-1", "G1-TOP-01", "en", 90, map[int]map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	if sub2.SupplyPodAwarded {
		t.Fatalf("weekly Supply Pod cap (1/week) violated")
	}
}

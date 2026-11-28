package training

import (
	"context"
	"testing"

	"github.com/AICSbench/planetlogi/apps/api/internal/mathengine"
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/economy"
)

func TestTrainingCurriculaProficiencyAndStars(t *testing.T) {
	ledger := economy.NewMemoryLedger(economy.Wallet{StudentID: "stu-1", PilotLevel: 1})
	svc := NewService(ledger)

	if cur := svc.ListCurricula("id"); len(cur) != 6 {
		t.Fatalf("expected 6 curricula, got %d", len(cur))
	}
	if wheel, err := svc.GetHOTSWheel("stu-1", "ALL", "en"); err != nil || len(wheel) != 4 {
		t.Fatalf("expected 4 HOTS quadrants, got %d", len(wheel))
	}

	sess, err := svc.StartPracticeSession("stu-1", "num-add-within-20", "en")
	if err != nil || len(sess.Questions) != 10 {
		t.Fatalf("expected 10-question practice session, err=%v len=%d", err, len(sess.Questions))
	}

	// Answer 9/10 correctly -> 3 stars + Mastery (90%)
	ctx := context.Background()
	for i, q := range sess.Questions {
		prob, _ := mathengine.GenerateByCode(q.TemplateCode, q.Seed, "en")
		ans := prob.Solution.CorrectAnswers
		if i == 9 {
			ans = map[string]string{"answer": "-999"}
		}
		if _, _, err := svc.SubmitPracticeAnswer(ctx, "stu-1", sess.SessionID, i+1, "en", ans); err != nil {
			t.Fatalf("submit q%d failed: %v", i+1, err)
		}
	}

	skills, err := svc.ListSkills("stu-1", "SINGAPORE_MOE", 1, "en")
	if err != nil {
		t.Fatal(err)
	}
	var found *SkillView
	for i := range skills {
		if skills[i].Code == "num-add-within-20" {
			found = &skills[i]
			break
		}
	}
	if found == nil {
		t.Fatal("expected skill in Grade 1 Singapore MOE list")
	}
	if found.ProficiencyPct != 90 || found.ProficiencyTier != TierMastery || found.BestStars != 3 {
		t.Fatalf("expected 90%% Mastery 3 stars, got %+v", found)
	}
}

package reports

import (
	"context"
	"testing"

	"github.com/AICSbench/planetlogi/apps/api/internal/mathengine"
	"github.com/AICSbench/planetlogi/apps/api/internal/modules/training"
)

func TestReportsAggregation(t *testing.T) {
	trn := training.NewService(nil)
	h := NewHandler(trn, nil)

	sess, err := trn.StartPracticeSession("stu-1", "num-add-within-20", "en")
	if err != nil {
		t.Fatal(err)
	}
	for i, q := range sess.Questions {
		prob, _ := mathengine.GenerateByCode(q.TemplateCode, q.Seed, "en")
		if _, _, err := trn.SubmitPracticeAnswer(context.Background(), "stu-1", sess.SessionID, i+1, "en", prob.Solution.CorrectAnswers); err != nil {
			t.Fatal(err)
		}
	}

	prof := h.BuildProficiency("stu-1", "SINGAPORE_MOE", 1, "en")
	if prof.MasteryCount != 1 {
		t.Fatalf("expected 1 Mastery skill, got %+v", prof)
	}

	hs := h.BuildHighScores("stu-1", "SINGAPORE_MOE", 1, "en")
	if len(hs.Tier9To10) != 1 {
		t.Fatalf("expected 1 skill in 9-10 tier, got %d", len(hs.Tier9To10))
	}
}

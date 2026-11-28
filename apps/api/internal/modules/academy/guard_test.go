package academy

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/AICSbench/planetlogi/apps/api/internal/platform/reqguard"
)

func TestDoLearnAnswersAreBounded(t *testing.T) {
	svc := NewService(nil)
	huge := map[string]string{"answer": strings.Repeat("9", 1000)}
	if _, err := svc.SaveDraft(context.Background(), "stu-1", "g1-wn-l1", "en", 1, huge); !errors.Is(err, reqguard.ErrInvalidAnswers) {
		t.Fatalf("save: expected ErrInvalidAnswers, got %v", err)
	}
	if _, _, err := svc.CheckAnswer(context.Background(), "stu-1", "g1-wn-l1", "en", 1, huge); !errors.Is(err, reqguard.ErrInvalidAnswers) {
		t.Fatalf("check: expected ErrInvalidAnswers, got %v", err)
	}
}

package astroquiz

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/AICSbench/planetlogi/apps/api/internal/platform/reqguard"
)

func TestPaperPayloadIsBounded(t *testing.T) {
	svc := NewService(nil)
	ctx := context.Background()
	huge := map[int]map[string]string{1: {"answer": strings.Repeat("9", 1000)}}
	if _, err := svc.SavePaper(ctx, "stu-1", "G1-ESS-01", "en", 10, huge); !errors.Is(err, reqguard.ErrInvalidAnswers) {
		t.Fatalf("save: expected ErrInvalidAnswers, got %v", err)
	}
	if _, err := svc.SubmitPaper(ctx, "stu-1", "G1-ESS-01", "en", 10, huge); !errors.Is(err, reqguard.ErrInvalidAnswers) {
		t.Fatalf("submit: expected ErrInvalidAnswers, got %v", err)
	}
	if _, err := svc.SavePaper(ctx, "stu-1", "G1-ESS-01", "en", maxElapsedSec+1, nil); err == nil {
		t.Fatal("expected an error for elapsedSec beyond 24h")
	}
}

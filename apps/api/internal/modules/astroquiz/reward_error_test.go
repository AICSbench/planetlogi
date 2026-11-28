package astroquiz

import (
	"context"
	"errors"
	"testing"

	"github.com/AICSbench/planetlogi/apps/api/internal/platform/economy"
)

type downLedger struct{}

func (downLedger) Apply(context.Context, economy.MutationRequest) (economy.MutationResult, error) {
	return economy.MutationResult{}, errors.New("ledger down")
}

func (downLedger) ApplyBatch(context.Context, []economy.MutationRequest) ([]economy.MutationResult, error) {
	return nil, errors.New("ledger down")
}

// A failed Supply Pod credit is reported as retryable (503), not as an internal error.
func TestSupplyPodLedgerFailureIsRetryable(t *testing.T) {
	svc := NewService(downLedger{})
	_, err := svc.SubmitPaper(context.Background(), "stu-1", "G1-ESS-01", "en", 0, nil)
	if !errors.Is(err, ErrRewardUnavailable) {
		t.Fatalf("expected ErrRewardUnavailable, got %v", err)
	}
	var statusErr interface{ GetStatus() int }
	if !errors.As(toHTTPError(context.Background(), err), &statusErr) || statusErr.GetStatus() != 503 {
		t.Fatalf("expected HTTP 503 for a retryable reward failure, got %v", toHTTPError(context.Background(), err))
	}
}

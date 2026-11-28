package customization

import (
	"errors"
	"testing"
	"time"

	"github.com/AICSbench/planetlogi/apps/api/internal/platform/economy"
)

func TestBadgeRecoveryUsesLocalDateAndRejectsInvalidDatesWithoutDebit(t *testing.T) {
	ctx := t.Context()
	ledger := economy.NewMemoryLedger(economy.Wallet{StudentID: "pilot", AstroCoins: 20})
	svc := NewService(ledger)
	// December 1 in Jakarta, November 30 in UTC.
	svc.nowClock = func() time.Time { return time.Date(2026, 11, 30, 18, 0, 0, 0, time.UTC) }
	for _, tc := range []struct{ date, timezone string }{
		{"2026-11-31", "Asia/Jakarta"},
		{"2026-12-02", "Asia/Jakarta"},
		{"2026-12-01", "UTC"},
	} {
		if _, err := svc.RecoverMissedBadge(ctx, "pilot", tc.timezone, tc.date, "id"); err == nil {
			t.Fatalf("accepted invalid recovery date %s in %s", tc.date, tc.timezone)
		}
	}
	// A wallet with exactly 20 AC can still recover a valid card after the rejections.
	card, err := svc.RecoverMissedBadge(ctx, "pilot", "Asia/Jakarta", "2026-11-30", "id")
	if err != nil || card.Date != "2026-11-30" || card.Tier != 1 || card.TierName != "Bronze" {
		t.Fatalf("valid recovery: card=%+v err=%v", card, err)
	}
	if _, err := svc.RecoverMissedBadge(ctx, "pilot", "Asia/Jakarta", "2026-11-29", "id"); !errors.Is(err, economy.ErrInsufficientFunds) {
		t.Fatalf("second distinct recovery: err=%v; want insufficient funds", err)
	}
}

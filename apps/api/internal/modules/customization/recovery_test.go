package customization

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/AICSbench/planetlogi/apps/api/internal/platform/economy"
)

func TestRecoverBadgeValidatesDateAgainstStudentMidnight(t *testing.T) {
	ctx := context.Background()
	ledger := economy.NewMemoryLedger(economy.Wallet{StudentID: "pilot", AstroCoins: 20})
	svc := NewService(ledger)
	now := time.Date(2026, 11, 29, 16, 59, 59, 0, time.UTC)
	svc.nowClock = func() time.Time { return now }

	for _, date := range []string{"2026-02-30", "2026-11", "2026-11-30"} {
		if _, err := svc.RecoverMissedBadge(ctx, "pilot", "Asia/Jakarta", date, "en"); err == nil {
			t.Errorf("accepted invalid or future date %q", date)
		}
	}

	// Jakarta has reached Nov 30, while UTC still shows Nov 29.
	now = now.Add(time.Second)
	card, err := svc.RecoverMissedBadge(ctx, "pilot", "Asia/Jakarta", "2026-11-30", "en")
	if err != nil || card.Date != "2026-11-30" || card.Tier != 1 {
		t.Fatalf("same-day recovery with original 20 AC: %+v, err=%v", card, err)
	}
	if _, err := svc.RecoverMissedBadge(ctx, "pilot", "Asia/Jakarta", "2026-11-29", "en"); !errors.Is(err, economy.ErrInsufficientFunds) {
		t.Fatalf("second recovery: got %v, want insufficient funds", err)
	}
	cards := svc.GetMonthlyBadges(ctx, "pilot", "2026-11", "en")
	if len(cards) != 30 || cards[29].Tier != 1 || cards[28].Tier != 0 {
		t.Fatalf("calendar should show only Nov 30 recovered, got %+v", cards)
	}
}

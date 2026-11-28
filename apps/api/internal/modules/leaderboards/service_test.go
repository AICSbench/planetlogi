package leaderboards

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/AICSbench/planetlogi/apps/api/internal/platform/economy"
)

func TestLeaderboardsTenThousandPilotsUnder50msAndMaskedPII(t *testing.T) {
	svc := NewService(economy.NewMemoryLedger())
	svc.SeedPilotsForBenchmark(10000)

	challenges := svc.ListChallenges("en")
	if len(challenges) != 10 {
		t.Fatalf("expected 6 open + 4 seasonal = 10 challenges, got %d", len(challenges))
	}

	start := time.Now()
	board := svc.GetBoard("stu-1", "Raka Pratama", 3, "HERO_OF_DAY", "NATIONAL", 0, "TOP50", "en")
	elapsed := time.Since(start)
	if elapsed > 50*time.Millisecond {
		t.Fatalf("leaderboard query over 10k pilots exceeded 50ms: %v", elapsed)
	}
	if len(board.Entries) != 50 {
		t.Fatalf("expected Top 50 entries, got %d", len(board.Entries))
	}
	for _, e := range board.Entries {
		if strings.Contains(e.MaskedName, "@") || !strings.HasSuffix(e.MaskedName, ".") {
			t.Fatalf("PII masking violation: %q", e.MaskedName)
		}
	}

	nearby := svc.GetBoard("stu-1", "Raka Pratama", 3, "HERO_OF_DAY", "NATIONAL", 0, "NEARBY", "id")
	if len(nearby.Entries) == 0 || len(nearby.Entries) > 11 {
		t.Fatalf("expected nearby +/-5 window (<=11 entries), got %d", len(nearby.Entries))
	}
}

func TestBonusGiftIdempotencyAndEvents(t *testing.T) {
	ctx := context.Background()
	ledger := economy.NewMemoryLedger(economy.Wallet{StudentID: "stu-gift", AstroCoins: 10})
	svc := NewService(ledger)

	// Fix the clock inside the Sunday Meteor window (Sunday 10:00 Asia/Jakarta)
	// so the window check is deterministic regardless of the real weekday.
	svc.SetClockForTest(func() time.Time {
		return time.Date(2026, 11, 29, 10, 0, 0, 0, time.FixedZone("WIB", 7*3600))
	})

	status, err := svc.ClaimBonusGift(ctx, "stu-gift", "Asia/Jakarta")
	if err != nil || !status.Claimed {
		t.Fatalf("first ClaimBonusGift failed: %v %+v", err, status)
	}
	probe, _ := ledger.Apply(ctx, economy.MutationRequest{
		StudentID:      "stu-gift",
		Currency:       economy.CurrencyAC,
		Delta:          1,
		SourceType:     "PROBE",
		IdempotencyKey: "gift-probe-1",
	})
	if probe.Wallet.AstroCoins != 10+DailyBonusGiftAC+1 {
		t.Fatalf("expected +%d AC from bonus gift, wallet=%d", DailyBonusGiftAC, probe.Wallet.AstroCoins)
	}

	if _, err := svc.ClaimBonusGift(ctx, "stu-gift", "Asia/Jakarta"); err != ErrAlreadyClaimedToday {
		t.Fatalf("expected ErrAlreadyClaimedToday on repeat claim, got %v", err)
	}

	mStart, err := svc.StartSundayMeteor(ctx, "stu-gift", 2, "Asia/Jakarta", "id")
	if err != nil {
		t.Fatalf("StartSundayMeteor: %v", err)
	}
	if len(mStart.Questions) != SundayMeteorCount {
		t.Fatalf("expected %d meteor questions, got %d", SundayMeteorCount, len(mStart.Questions))
	}
	res, err := svc.SubmitSundayMeteor(ctx, "stu-gift", mStart.SessionID, "Asia/Jakarta", "id", map[int]map[string]string{})
	if err != nil || res.Total != SundayMeteorCount {
		t.Fatalf("SubmitSundayMeteor failed: %v %+v", err, res)
	}

	// The one-session-per-day limit must reject a second session the same day.
	if _, err := svc.StartSundayMeteor(ctx, "stu-gift", 2, "Asia/Jakarta", "id"); err != ErrMeteorAlreadyDone {
		t.Fatalf("expected ErrMeteorAlreadyDone on second meteor session, got %v", err)
	}

	cr, err := svc.SubmitCreatorChallenge(ctx, "stu-gift", "Raka Pratama", 2, "Orbit Cookies", "Share 12 space cookies equally among 4 astronauts!")
	if err != nil {
		t.Fatalf("SubmitCreatorChallenge: %v", err)
	}
	if cr.AuthorMask != "Raka P." {
		t.Fatalf("expected masked author 'Raka P.', got %q", cr.AuthorMask)
	}
	if cr.Status != "PENDING" {
		t.Fatalf("expected pending moderation status, got %q", cr.Status)
	}
	// Pending child submissions must never appear in the public events hub.
	hub := svc.GetEventsHub("Asia/Jakarta", "en")
	for _, e := range hub.CreatorEntries {
		if e.ID == cr.ID {
			t.Fatalf("pending creator submission leaked into public creator entries")
		}
	}
}

func TestSundayMeteorWindowEnforced(t *testing.T) {
	svc := NewService(economy.NewMemoryLedger())
	// Saturday 10:00 Asia/Jakarta — outside the Sunday 06:00-22:00 window.
	svc.SetClockForTest(func() time.Time {
		return time.Date(2026, 11, 28, 10, 0, 0, 0, time.FixedZone("WIB", 7*3600))
	})
	if _, err := svc.StartSundayMeteor(context.Background(), "stu-x", 2, "Asia/Jakarta", "en"); err != ErrEventClosed {
		t.Fatalf("expected ErrEventClosed outside the Sunday window, got %v", err)
	}
}

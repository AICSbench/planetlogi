package leaderboards

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/AICSbench/planetlogi/apps/api/internal/mathengine"
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/economy"
	"github.com/jackc/pgx/v5/pgxpool"
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

func TestMeteorAndGiftDurabilityPG(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	var student string
	err = pool.QueryRow(ctx, `INSERT INTO student_profiles (parent_account_id, login_id, password_hash, first_name, masked_display_name, grade, crew_code) SELECT parent_account_id, 'meteor-test-' || gen_random_uuid(), password_hash, 'Test', 'Test P.', 1, left(replace(gen_random_uuid()::text, '-', ''), 9) FROM student_profiles WHERE login_id='pilot1' RETURNING id::text`).Scan(&student)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(ctx, `DELETE FROM student_profiles WHERE id=$1::uuid`, student); err != nil {
			t.Error(err)
		}
	})
	svc := NewServiceWithPG(pool, economy.NewPGLedger(pool))
	clock := time.Date(2026, 11, 29, 10, 0, 0, 0, time.FixedZone("WIB", 7*3600))
	svc.SetClockForTest(func() time.Time { return clock })
	status, err := svc.GetBonusGift(ctx, student, "Asia/Jakarta")
	if err != nil || status.Claimed {
		t.Fatalf("missing gift: %+v %v", status, err)
	}
	start, err := svc.StartSundayMeteor(ctx, student, 1, "Asia/Jakarta", "en")
	if err != nil {
		t.Fatal(err)
	}
	result, err := svc.SubmitSundayMeteor(ctx, student, start.SessionID, "Asia/Jakarta", "en", nil)
	if err != nil || result.RewardSP != 0 {
		t.Fatalf("zero score: %+v %v", result, err)
	}
	reloaded := NewServiceWithPG(pool, economy.NewPGLedger(pool))
	reloaded.SetClockForTest(func() time.Time { return clock })
	if _, err := reloaded.StartSundayMeteor(ctx, student, 1, "Asia/Jakarta", "en"); !errors.Is(err, ErrMeteorAlreadyDone) {
		t.Fatalf("completion not durable: %v", err)
	}
	clock = clock.AddDate(0, 0, 7)
	start, err = reloaded.StartSundayMeteor(ctx, student, 1, "Asia/Jakarta", "en")
	if err != nil {
		t.Fatal(err)
	}
	sess := reloaded.meteorSess[start.SessionID]
	prob, ok := mathengine.GenerateByCode(sess.templates[0], sess.seeds[0], "en")
	if !ok {
		t.Fatal("template missing")
	}
	result, err = reloaded.SubmitSundayMeteor(ctx, student, start.SessionID, "Asia/Jakarta", "en", map[int]map[string]string{0: prob.Solution.CorrectAnswers})
	if err != nil || result.RewardSP <= 0 {
		t.Fatalf("positive score: %+v %v", result, err)
	}
	var balance int64
	if err := pool.QueryRow(ctx, `SELECT total_sp FROM student_wallets WHERE student_id=$1::uuid`, student).Scan(&balance); err != nil || balance != result.RewardSP {
		t.Fatalf("wallet=%d reward=%d err=%v", balance, result.RewardSP, err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := reloaded.GetBonusGift(canceled, student, "Asia/Jakarta"); !errors.Is(err, ErrLedgerUnavailable) {
		t.Fatalf("expected read error, got %v", err)
	}
}

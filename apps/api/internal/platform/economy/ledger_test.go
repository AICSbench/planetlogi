package economy

import (
	"context"
	"sync"
	"testing"
)

func TestPilotXPLevelCurve(t *testing.T) {
	if got := XPRequiredForLevel(1); got != 4 {
		t.Fatalf("Lvl 1->2 should require 4 XP, got %d", got)
	}
	p := ComputeLevelProgress(0)
	if p.Level != 1 || p.XPForNextLevel != 4 {
		t.Fatalf("unexpected Lvl 1 progress: %+v", p)
	}
	p2 := ComputeLevelProgress(4)
	if p2.Level != 2 || p2.XPIntoLevel != 0 {
		t.Fatalf("expected Lvl 2 at 4 XP, got %+v", p2)
	}
	pMax := ComputeLevelProgress(10_000_000)
	if pMax.Level != MaxPilotLevel {
		t.Fatalf("expected cap at Lvl %d, got %d", MaxPilotLevel, pMax.Level)
	}
}

func TestLedgerIdempotencyAndConcurrency(t *testing.T) {
	ledger := NewMemoryLedger(Wallet{StudentID: "stu-1", AstroCoins: 20, PilotLevel: 1})
	ctx := context.Background()

	// Concurrent duplicate reward grants with same idempotency key MUST credit only once
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := ledger.Apply(ctx, MutationRequest{
				StudentID:      "stu-1",
				Currency:       CurrencySP,
				Delta:          3,
				SourceType:     "DAILY_ORBIT",
				SourceID:       "q9",
				IdempotencyKey: "daily_orbit:2026-11-26:stu-1:q9",
			})
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()

	res, err := ledger.Apply(ctx, MutationRequest{
		StudentID:      "stu-1",
		Currency:       CurrencyXP,
		Delta:          4,
		SourceType:     "DAILY_ORBIT",
		SourceID:       "q9",
		IdempotencyKey: "daily_orbit:2026-11-26:stu-1:q9:xp",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Wallet.TotalSP != 3 {
		t.Fatalf("expected exactly 3 SP (no double credit), got %d", res.Wallet.TotalSP)
	}
	if res.Wallet.PilotLevel != 2 {
		t.Fatalf("expected automatic level up to Lvl 2, got %d", res.Wallet.PilotLevel)
	}

	// Spending 20 AC succeeds; spending another 1 AC fails with ErrInsufficientFunds
	if _, err := ledger.Apply(ctx, MutationRequest{
		StudentID:      "stu-1",
		Currency:       CurrencyAC,
		Delta:          -20,
		SourceType:     "STORY_UNLOCK",
		SourceID:       "ch-1",
		IdempotencyKey: "story:stu-1:ch-1",
	}); err != nil {
		t.Fatalf("expected 20 AC spend to succeed: %v", err)
	}
	if _, err := ledger.Apply(ctx, MutationRequest{
		StudentID:      "stu-1",
		Currency:       CurrencyAC,
		Delta:          -1,
		SourceType:     "BRAIN_GALAXY",
		SourceID:       "game-1",
		IdempotencyKey: "brain:stu-1:game-1",
	}); err != ErrInsufficientFunds {
		t.Fatalf("expected ErrInsufficientFunds, got %v", err)
	}
}

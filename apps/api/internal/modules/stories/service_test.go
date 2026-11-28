package stories

import (
	"context"
	"testing"

	"github.com/AICSbench/planetlogi/apps/api/internal/platform/economy"
)

func TestPlanetStoriesOpeningFreeAndAtomic20ACUnlock(t *testing.T) {
	ledger := economy.NewMemoryLedger(economy.Wallet{StudentID: "stu-1", AstroCoins: 25, PilotLevel: 1})
	svc := NewService(ledger)

	planets := svc.ListPlanets(context.Background(), "stu-1", "en")
	if len(planets) != 6 {
		t.Fatalf("expected 6 Grade Planets, got %d", len(planets))
	}

	// Opening chapter is free & has 20 pages
	openCh, err := svc.OpenChapter(context.Background(), "stu-1", "terra_pebble-ch0", "id")
	if err != nil || len(openCh.Pages) != 20 {
		t.Fatalf("expected 20-page free opening chapter, err=%v pages=%d", err, len(openCh.Pages))
	}

	// Unlock Chapter 1 deducts 20 AC (25 -> 5) and is idempotent on repeat call
	if _, err := svc.UnlockChapter(context.Background(), "stu-1", "terra_pebble-ch1", "en"); err != nil {
		t.Fatalf("unlock ch1 failed: %v", err)
	}
	if _, err := svc.UnlockChapter(context.Background(), "stu-1", "terra_pebble-ch1", "en"); err != nil {
		t.Fatalf("idempotent unlock ch1 failed: %v", err)
	}

	// Chapter 2 requires 20 AC but only 5 AC remains -> ErrInsufficientFunds
	if _, err := svc.UnlockChapter(context.Background(), "stu-1", "terra_pebble-ch2", "en"); err != economy.ErrInsufficientFunds {
		t.Fatalf("expected ErrInsufficientFunds for ch2, got %v", err)
	}
}

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

func TestChapterBookmarksStayWithinChapterAndBelongToReader(t *testing.T) {
	ctx := t.Context()
	svc := NewService(nil)
	const chapter = "terra_pebble-ch0"

	for _, tc := range []struct{ page, want int }{{10, 10}, {21, 20}, {0, 1}} {
		if err := svc.BookmarkPage(ctx, "reader", chapter, tc.page); err != nil {
			t.Fatal(err)
		}
		viewer, err := svc.OpenChapter(ctx, "reader", chapter, "id")
		if err != nil || viewer.LastPageRead != tc.want {
			t.Fatalf("bookmark %d: viewer page = %d, error = %v; want %d", tc.page, viewer.LastPageRead, err, tc.want)
		}
		other, err := svc.OpenChapter(ctx, "other-reader", chapter, "en")
		if err != nil || other.LastPageRead != 1 {
			t.Fatalf("other reader page = %d, error = %v; want 1", other.LastPageRead, err)
		}
		for _, planet := range svc.ListPlanets(ctx, "reader", "en") {
			for _, summary := range planet.Chapters {
				if summary.Code == chapter && summary.LastPageRead != tc.want {
					t.Fatalf("summary page = %d; want %d", summary.LastPageRead, tc.want)
				}
			}
		}
	}
	if err := svc.BookmarkPage(ctx, "reader", "unknown-chapter", 3); err == nil {
		t.Fatal("bookmark accepted for a chapter that does not exist")
	}
}

func TestUnlockChapterDoesNotGrantAccessToAnotherReader(t *testing.T) {
	ctx := t.Context()
	ledger := economy.NewMemoryLedger(
		economy.Wallet{StudentID: "funded-reader", AstroCoins: 20},
		economy.Wallet{StudentID: "other-reader", AstroCoins: 19},
	)
	svc := NewService(ledger)
	const chapter = "terra_pebble-ch1"

	if _, err := svc.UnlockChapter(ctx, "funded-reader", chapter, "en"); err != nil {
		t.Fatalf("unlock with exactly 20 AC: %v", err)
	}
	if _, err := svc.OpenChapter(ctx, "funded-reader", chapter, "en"); err != nil {
		t.Fatalf("funded reader could not open unlocked chapter: %v", err)
	}
	if _, err := svc.UnlockChapter(ctx, "other-reader", chapter, "en"); err != economy.ErrInsufficientFunds {
		t.Fatalf("unlock with 19 AC = %v; want insufficient funds", err)
	}
	if _, err := svc.OpenChapter(ctx, "other-reader", chapter, "en"); err != economy.ErrInsufficientFunds {
		t.Fatalf("failed unlock granted access: %v", err)
	}
}

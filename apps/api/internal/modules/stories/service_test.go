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

func TestBookmarkPageClampsAndIsolatesProgress(t *testing.T) {
	ctx := context.Background()
	svc := NewService(nil)
	const chapter = "terra_pebble-ch0"
	for _, tc := range []struct {
		name string
		page int
		want int
	}{
		{"above opening length", 100, 20},
		{"in range", 8, 8},
		{"below first page", 0, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := svc.BookmarkPage(ctx, "reader", chapter, tc.page); err != nil {
				t.Fatal(err)
			}
			view, err := svc.OpenChapter(ctx, "reader", chapter, "id")
			if err != nil || view.LastPageRead != tc.want {
				t.Fatalf("open after bookmark: page=%d err=%v, want %d", view.LastPageRead, err, tc.want)
			}
			summary := svc.ListPlanets(ctx, "reader", "id")[0].Chapters[0]
			if summary.Code != chapter || summary.LastPageRead != tc.want {
				t.Fatalf("planet listing disagrees with viewer: %+v", summary)
			}
			other, err := svc.OpenChapter(ctx, "other-reader", chapter, "en")
			if err != nil || other.LastPageRead != 1 {
				t.Fatalf("bookmark leaked to another reader: page=%d err=%v", other.LastPageRead, err)
			}
		})
	}
	if err := svc.BookmarkPage(ctx, "reader", "unknown-chapter", 3); err == nil {
		t.Fatal("expected unknown chapter to be rejected")
	}
}

func TestStoryUnlockIsScopedToPurchaser(t *testing.T) {
	ctx := context.Background()
	svc := NewService(economy.NewMemoryLedger(
		economy.Wallet{StudentID: "buyer", AstroCoins: 20},
		economy.Wallet{StudentID: "other", AstroCoins: 19},
	))
	const chapter = "terra_pebble-ch1"
	if _, err := svc.UnlockChapter(ctx, "buyer", chapter, "en"); err != nil {
		t.Fatalf("unlock with exact price: %v", err)
	}
	if _, err := svc.OpenChapter(ctx, "buyer", chapter, "en"); err != nil {
		t.Fatalf("purchaser cannot reopen chapter: %v", err)
	}
	if _, err := svc.UnlockChapter(ctx, "other", chapter, "en"); err != economy.ErrInsufficientFunds {
		t.Fatalf("unlock with 19 AC: got %v, want insufficient funds", err)
	}
	if _, err := svc.OpenChapter(ctx, "other", chapter, "en"); err != economy.ErrInsufficientFunds {
		t.Fatalf("other reader could open paid chapter: %v", err)
	}
}

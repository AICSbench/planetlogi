package customization

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AICSbench/planetlogi/apps/api/internal/platform/economy"
)

func TestAstroBuddyShowcaseAndBadgeRecovery(t *testing.T) {
	ledger := economy.NewMemoryLedger(economy.Wallet{StudentID: "stu-1", AstroCoins: 20, PilotLevel: 1})
	svc := NewService(ledger)

	// Showcase filters out 0-SP entries
	entries := svc.GetShowcase(context.Background(), 1)
	for _, e := range entries {
		if e.StarPoints < 1 {
			t.Fatalf("showcase must hide students with 0 SP: %+v", e)
		}
	}

	// Safe 2-part name prevents arbitrary free text
	saved := svc.SaveBuddy(context.Background(), "stu-1", "Raka", AstroBuddyConfig{Adjective: "BadFreeText", BodyShape: "star", ColorHex: "#22D3EE"})
	if saved.FullName != "Starry Raka" {
		t.Fatalf("expected fallback safe name 'Starry Raka', got %q", saved.FullName)
	}

	// Recover missed badge deducts 20 AC idempotently
	if _, err := svc.RecoverMissedBadge(context.Background(), "stu-1", "Asia/Jakarta", "2026-11-10", "en"); err != nil {
		t.Fatalf("expected recovery to succeed: %v", err)
	}
	if _, err := svc.RecoverMissedBadge(context.Background(), "stu-1", "Asia/Jakarta", "2026-11-10", "en"); err != nil {
		t.Fatalf("idempotent recovery failed: %v", err)
	}
	if _, err := svc.RecoverMissedBadge(context.Background(), "stu-1", "Asia/Jakarta", "2026-11-11", "en"); err != economy.ErrInsufficientFunds {
		t.Fatalf("expected ErrInsufficientFunds on 2nd recovery, got %v", err)
	}

	// Validate 365-day bilingual badge trivia seed
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "content", "badges", "daily-star-badges-365.json"))
	if err != nil {
		t.Fatalf("missing daily-star-badges-365.json: %v", err)
	}
	var doc struct {
		Days []struct {
			DayOfYear int `json:"dayOfYear"`
		} `json:"days"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil || len(doc.Days) != 365 {
		t.Fatalf("expected 365 daily trivia cards, err=%v len=%d", err, len(doc.Days))
	}
}

// TestShowcasePersistAcrossRestartPG verifies the Galaxy Showcase entry list
// is served from the durable shared store so restarts cannot drop it.
func TestShowcasePersistAcrossRestartPG(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping PostgreSQL showcase persistence test")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	t.Cleanup(pool.Close)

	svc1 := NewServiceWithPG(pool, nil)
	first := svc1.GetShowcase(ctx, 1)
	if len(first) == 0 {
		t.Fatalf("expected seeded showcase entries, got none")
	}
	// A fresh "process" must see the same durable entries.
	svc2 := NewServiceWithPG(pool, nil)
	second := svc2.GetShowcase(ctx, 1)
	if len(second) != len(first) {
		t.Fatalf("showcase entries lost across restart: %d -> %d", len(first), len(second))
	}
	for i := range first {
		if first[i].MaskedName != second[i].MaskedName || first[i].Rank != second[i].Rank {
			t.Fatalf("showcase entry mismatch after restart: %+v vs %+v", first[i], second[i])
		}
	}
}

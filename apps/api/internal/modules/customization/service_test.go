package customization

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5"
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
	saved, err := svc.SaveBuddy(context.Background(), "stu-1", "Raka", AstroBuddyConfig{Adjective: "BadFreeText", BodyShape: "star", ColorHex: "#22D3EE"})
	if err != nil {
		t.Fatalf("SaveBuddy: %v", err)
	}
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

	// This test uses the shared showcase key; preserve any existing fixture.
	var previous []byte
	err = pool.QueryRow(ctx, `SELECT payload FROM shared_state WHERE scope=$1 AND key=$2`, showcaseScope, showcaseKey).Scan(&previous)
	if err != nil && err != pgx.ErrNoRows {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM shared_state WHERE scope=$1 AND key=$2`, showcaseScope, showcaseKey); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, err := pool.Exec(ctx, `DELETE FROM shared_state WHERE scope=$1 AND key=$2`, showcaseScope, showcaseKey)
		if err == nil && previous != nil {
			_, err = pool.Exec(ctx, `INSERT INTO shared_state(scope,key,payload) VALUES($1,$2,$3::jsonb)`, showcaseScope, showcaseKey, string(previous))
		}
		if err != nil {
			t.Error(err)
		}
	})
	svc1 := NewServiceWithPG(pool, nil)
	first := svc1.GetShowcase(ctx, 1)
	if len(first) == 0 {
		t.Fatalf("expected seeded showcase entries, got none")
	}
	fixture := []ShowcaseEntry{{MaskedName: "Fixture P.", Grade: 1, StarPoints: 987}}
	if err := svc1.store.SaveShared(ctx, showcaseScope, showcaseKey, fixture); err != nil {
		t.Fatal(err)
	}
	// A fresh "process" must see the modified durable entry rather than seed.
	svc2 := NewServiceWithPG(pool, nil)
	second := svc2.GetShowcase(ctx, 1)
	if len(second) != 1 || second[0].MaskedName != "Fixture P." || second[0].StarPoints != 987 {
		t.Fatalf("durable showcase fixture lost: %+v", second)
	}
	if err := svc1.store.SaveShared(ctx, showcaseScope, showcaseKey, []ShowcaseEntry{}); err != nil {
		t.Fatal(err)
	}
	if got := svc2.GetShowcase(ctx, 1); len(got) != 0 {
		t.Fatalf("persisted empty showcase was replaced: %+v", got)
	}
}

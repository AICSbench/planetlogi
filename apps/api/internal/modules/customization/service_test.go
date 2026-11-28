package customization

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/AICSbench/planetlogi/apps/api/internal/platform/economy"
)

func TestAstroBuddyShowcaseAndBadgeRecovery(t *testing.T) {
	ledger := economy.NewMemoryLedger(economy.Wallet{StudentID: "stu-1", AstroCoins: 20, PilotLevel: 1})
	svc := NewService(ledger)

	// Showcase filters out 0-SP entries
	entries := svc.GetShowcase(1)
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

package social

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/AICSbench/planetlogi/apps/api/internal/platform/economy"
)

func TestStarDuelsEnergyLimitAndReset(t *testing.T) {
	ledger := economy.NewMemoryLedger()
	svc := NewService(ledger)

	day1 := time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC)
	svc.SetClockForTest(func() time.Time { return day1 })

	for i := 1; i <= DailyEnergyMax; i++ {
		duel, left, err := svc.StartDuel("stu-1", 1, "Asia/Jakarta", "en", "RANDOM", "")
		if err != nil {
			t.Fatalf("expected duel %d to start, got err: %v", i, err)
		}
		if left != DailyEnergyMax-i {
			t.Fatalf("expected %d energy left, got %d", DailyEnergyMax-i, left)
		}
		if duel.Opponent.Grade != 1 {
			t.Fatalf("random matchmaking violated grade isolation: got grade %d", duel.Opponent.Grade)
		}
		if strings.Contains(duel.Opponent.MaskedName, "@") || !strings.HasSuffix(duel.Opponent.MaskedName, ".") {
			t.Fatalf("opponent name not masked: %q", duel.Opponent.MaskedName)
		}
	}

	if _, _, err := svc.StartDuel("stu-1", 1, "Asia/Jakarta", "en", "RANDOM", ""); err != ErrNoEnergy {
		t.Fatalf("expected ErrNoEnergy on 4th duel, got %v", err)
	}

	day2 := day1.Add(24 * time.Hour)
	svc.SetClockForTest(func() time.Time { return day2 })
	_, left, err := svc.StartDuel("stu-1", 1, "Asia/Jakarta", "en", "CLASSMATE", "")
	if err != nil || left != 2 {
		t.Fatalf("expected energy reset on next day, err=%v left=%d", err, left)
	}
}

func TestStarDuelGradingAndIdempotentSP(t *testing.T) {
	ctx := context.Background()
	ledger := economy.NewMemoryLedger(economy.Wallet{StudentID: "stu-2", TotalSP: 10})
	svc := NewService(ledger)

	duel, _, err := svc.StartDuel("stu-2", 2, "Asia/Jakarta", "id", "CLASSMATE", "")
	if err != nil {
		t.Fatalf("StartDuel: %v", err)
	}
	res1, err := svc.SubmitDuel(ctx, "stu-2", duel.ID, "id", map[int]map[string]string{})
	if err != nil {
		t.Fatalf("SubmitDuel: %v", err)
	}
	if res1.Status != "COMPLETED" || res1.RewardSP <= 0 {
		t.Fatalf("expected completed duel with participation SP, got %+v", res1)
	}

	// Verify ledger SP updated via probe mutation
	probe1, _ := ledger.Apply(ctx, economy.MutationRequest{
		StudentID:      "stu-2",
		Currency:       economy.CurrencySP,
		Delta:          1,
		SourceType:     "PROBE",
		IdempotencyKey: "probe-1",
	})
	// Replay submit is idempotent
	_, err = svc.SubmitDuel(ctx, "stu-2", duel.ID, "id", map[int]map[string]string{})
	if err != nil {
		t.Fatalf("replay SubmitDuel: %v", err)
	}
	probe2, _ := ledger.Apply(ctx, economy.MutationRequest{
		StudentID:      "stu-2",
		Currency:       economy.CurrencySP,
		Delta:          1,
		SourceType:     "PROBE",
		IdempotencyKey: "probe-2",
	})
	if probe2.Wallet.TotalSP != probe1.Wallet.TotalSP+1 {
		t.Fatalf("expected idempotent SP award (only +1 from probe), got %d then %d", probe1.Wallet.TotalSP, probe2.Wallet.TotalSP)
	}
}

func TestStarCrewCodeUniquenessAndJoin(t *testing.T) {
	svc := NewService(economy.NewMemoryLedger())

	seen := make(map[string]bool)
	for i := 0; i < 20; i++ {
		c := svc.GetOrCreateCrew("pilot-"+string(rune('a'+i)), "Raka Pratama", 1)
		if len(c.CrewCode) != 9 {
			t.Fatalf("expected 9-digit crew code, got %q", c.CrewCode)
		}
		if seen[c.CrewCode] {
			t.Fatalf("duplicate crew code generated: %s", c.CrewCode)
		}
		seen[c.CrewCode] = true
	}

	joined, err := svc.JoinCrew("stu-join", "Siti Rahma", 1, "482-910-375")
	if err != nil {
		t.Fatalf("JoinCrew failed: %v", err)
	}
	if len(joined.Members) != 3 {
		t.Fatalf("expected 3 crew members after join, got %d", len(joined.Members))
	}
	if joined.WeeklyTotalSP != 68+52+35 {
		t.Fatalf("unexpected weekly crew SP aggregation: %d", joined.WeeklyTotalSP)
	}
	left := svc.LeaveCrew("stu-join", "Siti Rahma", 1)
	if left.CrewCode == "482910375" || len(left.Members) != 1 {
		t.Fatalf("expected fresh personal crew after leave, got %+v", left)
	}
}

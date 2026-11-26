package dailyorbit

import (
	"context"
	"testing"
	"time"

	"github.com/AICSbench/planetlogi/apps/api/internal/mathengine"
	"github.com/AICSbench/planetlogi/apps/api/internal/modules/streak"
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/economy"
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/timewindow"
)

func TestDailyOrbitScoringLimitsAndLocks(t *testing.T) {
	ledger := economy.NewMemoryLedger(economy.Wallet{StudentID: "stu-1", AstroCoins: 10, PilotLevel: 1})
	streakSvc := streak.NewService()
	svc := NewService(ledger, streakSvc)

	loc := timewindow.LoadLocationSafe("Asia/Jakarta")
	svc.nowClock = func() time.Time { return time.Date(2026, 11, 26, 10, 0, 0, 0, loc) }

	today := svc.GetToday("stu-1", 1, "Asia/Jakarta", "en")
	if len(today.Slots) != 10 {
		t.Fatalf("expected 10 Daily Orbit slots, got %d", len(today.Slots))
	}
	if today.Slots[0].WorkedSolution != nil {
		t.Fatalf("pending slot MUST NOT expose WorkedSolution")
	}

	// Change Question works 3 times and fails on the 4th
	for i := 0; i < 3; i++ {
		if _, _, err := svc.ChangeQuestion("stu-1", 1, "Asia/Jakarta", "en", 1, "easier"); err != nil {
			t.Fatalf("change question %d failed: %v", i+1, err)
		}
	}
	if _, _, err := svc.ChangeQuestion("stu-1", 1, "Asia/Jakarta", "en", 1, "harder"); err != ErrChangeLimitReached {
		t.Fatalf("expected ErrChangeLimitReached on 4th change, got %v", err)
	}

	// Answer all 10 slots correctly -> earns 17 SP + 5 AC completion bonus + advances streak!
	ctx := context.Background()
	refreshed := svc.GetToday("stu-1", 1, "Asia/Jakarta", "en")
	for _, sl := range refreshed.Slots {
		prob, _ := mathengine.GenerateByCode(sl.Question.TemplateCode, sl.Question.Seed, "en")
		resSlot, _, err := svc.SubmitAnswer(ctx, "stu-1", 1, "Asia/Jakarta", "en", sl.Index, prob.Solution.CorrectAnswers)
		if err != nil {
			t.Fatalf("slot %d submit error: %v", sl.Index, err)
		}
		if resSlot.Status != SlotCorrect {
			t.Fatalf("expected slot %d CORRECT, got %s", sl.Index, resSlot.Status)
		}
	}

	// Duplicate submission on locked slot 1 MUST return ErrAlreadyAnswered
	if _, _, err := svc.SubmitAnswer(ctx, "stu-1", 1, "Asia/Jakarta", "en", 1, map[string]string{"answer": "0"}); err != ErrAlreadyAnswered {
		t.Fatalf("expected ErrAlreadyAnswered, got %v", err)
	}

	final := svc.GetToday("stu-1", 1, "Asia/Jakarta", "en")
	if final.TotalSPEarned != 17 || !final.CompletionBonusAward {
		t.Fatalf("expected 17 SP & completion bonus, got SP=%d bonus=%v", final.TotalSPEarned, final.CompletionBonusAward)
	}
	if st := streakSvc.GetStatus("stu-1", "Asia/Jakarta"); st.CurrentStreak != 4 {
		t.Fatalf("expected streak to advance from 3 -> 4, got %d", st.CurrentStreak)
	}

	// Intermediate tier (15 SP) should now be unlockable; Master (100 SP) still locked
	if _, err := svc.SetTier("stu-1", 1, "Asia/Jakarta", "en", TierIntermediate); err != nil {
		t.Fatalf("expected Intermediate tier to unlock: %v", err)
	}
	if _, err := svc.SetTier("stu-1", 1, "Asia/Jakarta", "en", TierMaster); err != ErrTierLocked {
		t.Fatalf("expected ErrTierLocked for Master, got %v", err)
	}
}

package streak

import (
	"context"
	"testing"
	"time"

	"github.com/AICSbench/planetlogi/apps/api/internal/platform/timewindow"
)

func TestOrbitStreakAndCryoShieldTimeTravel(t *testing.T) {
	svc := NewService()
	tz := "Asia/Jakarta"
	loc := timewindow.LoadLocationSafe(tz)

	day1 := time.Date(2026, 11, 26, 10, 0, 0, 0, loc)

	// 4 correct answers does NOT increment streak yet
	st := svc.RecordDailyOrbitProgress("stu-1", tz, day1, 4)
	if st.CurrentStreak != 3 {
		t.Fatalf("expected streak to remain 3 at 4/5 correct, got %d", st.CurrentStreak)
	}

	// 5th correct answer increments streak to 4 (idempotent if called again on same day)
	st = svc.RecordDailyOrbitProgress("stu-1", tz, day1, 5)
	st = svc.RecordDailyOrbitProgress("stu-1", tz, day1, 6)
	if st.CurrentStreak != 4 {
		t.Fatalf("expected streak 4 after reaching 5 correct, got %d", st.CurrentStreak)
	}

	// Miss day 2 -> midnight on day 3 consumes 1 Cryo Shield (2 -> 1), preserves streak 4
	day3 := time.Date(2026, 11, 28, 0, 5, 0, 0, loc)
	st = svc.ReconcileMidnight("stu-1", tz, day3)
	if st.CurrentStreak != 4 || st.CryoShields != 1 {
		t.Fatalf("expected Cryo Shield protection (streak 4, shields 1), got %+v", st)
	}

	// Miss day 3 -> midnight on day 4 consumes last Cryo Shield (1 -> 0)
	day4 := time.Date(2026, 11, 29, 0, 5, 0, 0, loc)
	st = svc.ReconcileMidnight("stu-1", tz, day4)
	if st.CurrentStreak != 4 || st.CryoShields != 0 {
		t.Fatalf("expected 0 shields left & streak 4, got %+v", st)
	}

	// Miss day 4 with 0 shields -> midnight on day 5 resets streak to 0
	day5 := time.Date(2026, 11, 30, 0, 5, 0, 0, loc)
	st = svc.ReconcileMidnight("stu-1", tz, day5)
	if st.CurrentStreak != 0 {
		t.Fatalf("expected streak reset to 0 with 0 shields, got %d", st.CurrentStreak)
	}

	// Max-3 Cryo Shield cap enforced
	st = svc.GrantCryoShield("stu-1", 10)
	if st.CryoShields != MaxCryoShields {
		t.Fatalf("expected Cryo Shield cap %d, got %d", MaxCryoShields, st.CryoShields)
	}

	// ReconcileAllMidnight runs across all active student records
	n, err := svc.ReconcileAllMidnight(context.Background(), day5.AddDate(0, 0, 1))
	if err != nil || n != 1 {
		t.Fatalf("expected ReconcileAllMidnight to process 1 student, got n=%d err=%v", n, err)
	}
}

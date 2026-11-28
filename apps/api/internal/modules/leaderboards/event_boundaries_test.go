package leaderboards

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/AICSbench/planetlogi/apps/api/internal/platform/economy"
)

type retryGiftLedger struct {
	economy.Ledger
	unavailable bool
}

func (l *retryGiftLedger) Apply(ctx context.Context, req economy.MutationRequest) (economy.MutationResult, error) {
	if l.unavailable {
		return economy.MutationResult{}, errors.New("ledger unavailable")
	}
	return l.Ledger.Apply(ctx, req)
}

func TestBonusGiftRetryAndLocalMidnight(t *testing.T) {
	ctx := context.Background()
	ledger := &retryGiftLedger{Ledger: economy.NewMemoryLedger(), unavailable: true}
	svc := NewService(ledger)
	const student = "pilot"
	const tz = "Asia/Jakarta"
	now := time.Date(2026, 11, 29, 16, 59, 59, 0, time.UTC)
	svc.SetClockForTest(func() time.Time { return now })

	assertStatus := func(date string, claimed bool) {
		t.Helper()
		status, err := svc.GetBonusGift(ctx, student, tz)
		if err != nil || status.LocalDate != date || status.Claimed != claimed || status.RewardAC != 2 {
			t.Fatalf("gift status: %+v err=%v, want date=%s claimed=%v reward=2", status, err, date, claimed)
		}
	}
	if _, err := svc.ClaimBonusGift(ctx, student, tz); !errors.Is(err, ErrLedgerUnavailable) {
		t.Fatalf("failed claim: got %v, want ledger unavailable", err)
	}
	assertStatus("2026-11-29", false)
	ledger.unavailable = false
	if _, err := svc.ClaimBonusGift(ctx, student, tz); err != nil {
		t.Fatalf("retry claim: %v", err)
	}
	assertStatus("2026-11-29", true)

	now = now.Add(time.Second)
	assertStatus("2026-11-30", false)
	if _, err := svc.ClaimBonusGift(ctx, student, tz); err != nil {
		t.Fatalf("new local day's claim: %v", err)
	}
	assertStatus("2026-11-30", true)
	wallet, err := ledger.Apply(ctx, economy.MutationRequest{
		StudentID: student, Currency: economy.CurrencyAC, Delta: -4,
		SourceType: "TEST_SPEND", IdempotencyKey: "spend-two-gifts",
	})
	if err != nil || wallet.Wallet.AstroCoins != 0 {
		t.Fatalf("expected exactly 4 AC from two local dates, wallet=%+v err=%v", wallet.Wallet, err)
	}
}

func TestSundayMeteorWindowEdges(t *testing.T) {
	for _, tc := range []struct {
		name       string
		utcHour    int
		utcMinute  int
		localStart bool
	}{
		{"before opening", -2, 59, false}, // Saturday 22:59 UTC is Sunday 05:59 in Jakarta.
		{"at opening", -1, 0, true},
		{"before closing", 14, 59, true},
		{"at closing", 15, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := NewService(nil)
			svc.SetClockForTest(func() time.Time {
				return time.Date(2026, 11, 29, tc.utcHour, tc.utcMinute, 0, 0, time.UTC)
			})
			start, err := svc.StartSundayMeteor(context.Background(), "pilot", 3, "Asia/Jakarta", "en")
			if tc.localStart {
				if err != nil || start.SessionID == "" || len(start.Questions) != SundayMeteorCount {
					t.Fatalf("inside window: session=%q questions=%d err=%v", start.SessionID, len(start.Questions), err)
				}
			} else if !errors.Is(err, ErrEventClosed) {
				t.Fatalf("outside window: got %v, want event closed", err)
			}
		})
	}
}

func TestMarkAllNotificationsReadIsStudentScoped(t *testing.T) {
	svc := NewService(nil)
	alice := svc.ListNotifications("pilot-a", "en")
	bob := svc.ListNotifications("pilot-b", "en")
	if len(alice) == 0 || len(bob) == 0 || alice[0].Read || bob[0].Read {
		t.Fatal("expected both pilots to have an unread notification")
	}
	svc.MarkAllNotificationsRead("pilot-a")
	for _, item := range svc.ListNotifications("pilot-a", "en") {
		if !item.Read {
			t.Fatalf("notification %s is still unread", item.ID)
		}
	}
	if svc.ListNotifications("pilot-b", "en")[0].Read {
		t.Fatal("marking one pilot's notifications affected another")
	}
}

package social

import (
	"errors"
	"testing"
	"time"
)

func TestIncomingDuelRequiresOwnerAndAvailableEnergy(t *testing.T) {
	ctx := t.Context()
	svc := NewService(nil)
	now := time.Date(2026, 12, 1, 16, 59, 0, 0, time.UTC) // 23:59 in Jakarta
	svc.SetClockForTest(func() time.Time { return now })
	const tz = "Asia/Jakarta"
	lobby := func() DuelLobby { return svc.GetLobby(ctx, "pilot", 3, tz, "en") }
	incoming := lobby().Incoming
	if len(incoming) == 0 {
		t.Fatal("expected a pending incoming duel")
	}
	id := incoming[0].ID
	if _, err := svc.AcceptIncoming(ctx, "other-pilot", id, tz, "en"); !errors.Is(err, ErrDuelNotFound) {
		t.Fatalf("another pilot accepted duel: %v", err)
	}
	if _, err := svc.SubmitDuel(ctx, "pilot", id, "en", nil); !errors.Is(err, ErrDuelNotActive) {
		t.Fatalf("pending duel could be submitted: %v", err)
	}
	for range 3 {
		if _, _, err := svc.StartDuel(ctx, "pilot", 3, tz, "en", "RANDOM", ""); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := svc.AcceptIncoming(ctx, "pilot", id, tz, "en"); !errors.Is(err, ErrNoEnergy) {
		t.Fatalf("accept with exhausted energy = %v; want ErrNoEnergy", err)
	}
	now = now.Add(time.Minute)
	accepted, err := svc.AcceptIncoming(ctx, "pilot", id, tz, "en")
	if err != nil || accepted.Status != "IN_PROGRESS" || len(accepted.Questions) != 5 {
		t.Fatalf("accept after local midnight: duel=%+v err=%v", accepted, err)
	}
	if _, err := svc.AcceptIncoming(ctx, "pilot", id, tz, "en"); !errors.Is(err, ErrDuelNotActive) {
		t.Fatalf("duplicate acceptance = %v; want ErrDuelNotActive", err)
	}
	if left := lobby().EnergyLeft; left != 2 {
		t.Fatalf("energy after one acceptance and one retry = %d; want 2", left)
	}
}

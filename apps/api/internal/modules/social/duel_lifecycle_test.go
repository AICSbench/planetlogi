package social

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/AICSbench/planetlogi/apps/api/internal/platform/economy"
)

func TestIncomingDuelSharesEnergyAndRequiresAcceptance(t *testing.T) {
	ctx := context.Background()
	svc := NewService(economy.NewMemoryLedger())
	const student = "pilot"
	const tz = "Asia/Jakarta"
	now := time.Date(2026, 11, 29, 16, 59, 59, 0, time.UTC)
	svc.SetClockForTest(func() time.Time { return now })
	lobby := svc.GetLobby(ctx, student, 2, tz, "en")
	if len(lobby.Incoming) != 1 {
		t.Fatalf("expected one incoming challenge, got %d", len(lobby.Incoming))
	}
	id := lobby.Incoming[0].ID
	if _, err := svc.AcceptIncoming(ctx, "other-pilot", id, tz, "en"); !errors.Is(err, ErrDuelNotFound) {
		t.Fatalf("another pilot accepting duel: %v", err)
	}
	if _, err := svc.SubmitDuel(ctx, student, id, "en", nil); !errors.Is(err, ErrDuelNotActive) {
		t.Fatalf("submitting before acceptance: %v", err)
	}
	for i := 0; i < DailyEnergyMax; i++ {
		if _, _, err := svc.StartDuel(ctx, student, 2, tz, "en", "RANDOM", ""); err != nil {
			t.Fatalf("outgoing duel %d: %v", i+1, err)
		}
	}
	if _, err := svc.AcceptIncoming(ctx, student, id, tz, "en"); !errors.Is(err, ErrNoEnergy) {
		t.Fatalf("accept at exhausted quota: %v", err)
	}

	now = now.Add(time.Second)
	accepted, err := svc.AcceptIncoming(ctx, student, id, tz, "id")
	if err != nil || accepted.Status != "IN_PROGRESS" || len(accepted.Questions) != DuelQuestionCount {
		t.Fatalf("accept after local midnight: %+v, err=%v", accepted, err)
	}
	if _, err := svc.AcceptIncoming(ctx, student, id, tz, "en"); !errors.Is(err, ErrDuelNotActive) {
		t.Fatalf("accept again: %v", err)
	}
	if got := svc.GetLobby(ctx, student, 2, tz, "en"); got.LocalDate != "2026-11-30" || got.EnergyLeft != DailyEnergyMax-1 {
		t.Fatalf("accept should use exactly one cell on the new local date: date=%s left=%d", got.LocalDate, got.EnergyLeft)
	}
}

func TestJoinCrewRejectsBadCodesAndDuplicateMembership(t *testing.T) {
	ctx := context.Background()
	svc := NewService(nil)
	const owner = "crew-owner"
	original := svc.GetOrCreateCrew(ctx, owner, "Crew Owner", 2)
	for _, tc := range []struct {
		code string
		err  error
	}{
		{"12-34", ErrInvalidCode},
		{"12345678A", ErrInvalidCode},
		{"000000000", ErrCrewNotFound},
		{original.CrewCode, ErrAlreadyInCrew},
	} {
		if _, err := svc.JoinCrew(ctx, owner, "Crew Owner", 2, tc.code); !errors.Is(err, tc.err) {
			t.Errorf("JoinCrew(%q) error = %v, want %v", tc.code, err, tc.err)
		}
		got := svc.GetOrCreateCrew(ctx, owner, "Crew Owner", 2)
		if got.CrewCode != original.CrewCode || len(got.Members) != 1 || got.WeeklyTotalSP != original.WeeklyTotalSP {
			t.Fatalf("rejected join changed current roster: %+v", got)
		}
	}
}

package supernova

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/AICSbench/planetlogi/apps/api/internal/mathengine"
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/economy"
)

type retrySightLedger struct {
	*economy.MemoryLedger
	unavailable bool
}

func (l *retrySightLedger) Apply(ctx context.Context, req economy.MutationRequest) (economy.MutationResult, error) {
	if l.unavailable {
		return economy.MutationResult{}, errors.New("ledger unavailable")
	}
	return l.MemoryLedger.Apply(ctx, req)
}

func TestSuperSightRetriesFrozenScoreAfterRewardFailure(t *testing.T) {
	ctx := t.Context()
	ledger := &retrySightLedger{MemoryLedger: economy.NewMemoryLedger(), unavailable: true}
	svc := NewService(ledger)
	svc.SetClockForTest(func() time.Time { return time.Date(2026, 12, 1, 4, 0, 0, 0, time.UTC) })
	session, err := svc.StartSuperSight("pilot", "Asia/Jakarta", "en", false)
	if err != nil {
		t.Fatal(err)
	}
	tap := func(student string, x, y float64) (bool, SuperSightSession, error) {
		return svc.TapSuperSight(ctx, student, session.SessionID, x, y, "en")
	}
	if hit, _, err := tap("other-pilot", .18, .25); err == nil || hit {
		t.Fatalf("other student could tap: hit=%t err=%v", hit, err)
	}
	if hit, state, err := tap("pilot", .18, .25); err != nil || !hit || state.Score != 10 || state.WrongCount != 0 {
		t.Fatalf("first correct tap: hit=%t state=%+v err=%v", hit, state, err)
	}
	// Repeating an already found point must not award additional points.
	if hit, state, err := tap("pilot", .18, .25); err != nil || hit || state.Score != 10 || state.FoundCount != 1 {
		t.Fatalf("duplicate tap: hit=%t state=%+v err=%v", hit, state, err)
	}
	for wrong := 2; wrong < 5; wrong++ {
		if hit, state, err := tap("pilot", 0, 0); err != nil || hit || state.Completed || state.WrongCount != wrong {
			t.Fatalf("miss %d: hit=%t state=%+v err=%v", wrong, hit, state, err)
		}
	}
	if _, state, err := tap("pilot", 0, 0); !errors.Is(err, ErrRewardPending) || state.WrongCount != 5 {
		t.Fatalf("final miss: state=%+v err=%v; want pending reward", state, err)
	}
	if hub := svc.GetHub(ctx, "pilot", "Asia/Jakarta", 1); hub.WeeklyDiamondScore != 0 || len(hub.History) != 0 {
		t.Fatalf("failed reward recorded a score: %+v", hub)
	}
	ledger.unavailable = false
	for range 2 {
		if hit, state, err := tap("pilot", .50, .22); !errors.Is(err, ErrSessionEnded) || hit || !state.Completed || state.Score != 10 {
			t.Fatalf("tap after reaching limit: hit=%t state=%+v err=%v", hit, state, err)
		}
	}
	hub := svc.GetHub(ctx, "pilot", "Asia/Jakarta", 1)
	if hub.WeeklyDiamondScore != 10 || len(hub.History) != 1 || hub.History[0].Score != 10 {
		t.Fatalf("recovered score was not recorded exactly once: %+v", hub)
	}
}

func TestSuperNovaHistoryPaginatesNewestFirst(t *testing.T) {
	ctx := t.Context()
	svc := NewService(nil)
	now := time.Date(2026, 12, 1, 4, 0, 0, 0, time.UTC)
	svc.SetClockForTest(func() time.Time { return now })
	var oldest, newest string
	for i := 0; i <= HubHistoryPageSize; i++ {
		session, err := svc.StartSuperSight("pilot", "Asia/Jakarta", "en", true)
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			oldest = session.SessionID
		}
		newest = session.SessionID
		now = now.Add(6 * time.Minute)
		if _, _, err := svc.TapSuperSight(ctx, "pilot", session.SessionID, 0, 0, "en"); !errors.Is(err, ErrSessionEnded) {
			t.Fatalf("expire practice session %d: %v", i, err)
		}
	}
	first := svc.GetHub(ctx, "pilot", "Asia/Jakarta", 0)
	if first.HistoryPage != 1 || first.HistoryPages != 2 || len(first.History) != HubHistoryPageSize || first.History[0].ID != newest {
		t.Fatalf("unexpected first history page: %+v", first)
	}
	last := svc.GetHub(ctx, "pilot", "Asia/Jakarta", 99)
	if last.HistoryPage != 2 || len(last.History) != 1 || last.History[0].ID != oldest {
		t.Fatalf("unexpected last history page: %+v", last)
	}
	for i := 1; i < len(first.History); i++ {
		if !first.History[i-1].RecordedAt.After(first.History[i].RecordedAt) {
			t.Fatal("history is not newest first")
		}
	}
	if hub := svc.GetHub(ctx, "other-pilot", "Asia/Jakarta", 1); len(hub.History) != 0 {
		t.Fatalf("another student's history was visible: %+v", hub.History)
	}
}

func TestSuperWarpScoresSubmittedAnswersOnce(t *testing.T) {
	ctx := t.Context()
	svc := NewService(nil)
	now := time.Date(2026, 12, 1, 4, 0, 0, 0, time.UTC)
	svc.SetClockForTest(func() time.Time { return now })
	const tz = "Asia/Jakarta"
	session, err := svc.StartSuperWarp("pilot", tz, "id", 3, false)
	if err != nil {
		t.Fatal(err)
	}
	if session.DurationS != 120 || session.AutoSkipS != 10 || len(session.Questions) < 2 {
		t.Fatalf("unexpected sprint configuration: %+v", session)
	}
	// Use the generated problem's solution as an input fixture. Leave all
	// other questions unanswered and include an out-of-range answer.
	state := svc.warpSess[session.SessionID]
	problem, ok := mathengine.GenerateByCode(state.templates[0], state.seeds[0], "id")
	if !ok {
		t.Fatal("first question could not be regenerated")
	}
	answers := map[int]map[string]string{
		1: problem.Solution.CorrectAnswers, len(session.Questions) + 1: problem.Solution.CorrectAnswers,
	}
	if _, _, err := svc.FinishSuperWarp(ctx, "other-pilot", session.SessionID, "id", answers); err == nil {
		t.Fatal("another pilot could finish the session")
	}
	now = now.Add(120 * time.Second)
	score, _, err := svc.FinishSuperWarp(ctx, "pilot", session.SessionID, "id", answers)
	if err != nil || score != 1 {
		t.Fatalf("score=%d err=%v; want one point for the single correct answer", score, err)
	}
	now = now.Add(time.Minute)
	score, _, err = svc.FinishSuperWarp(ctx, "pilot", session.SessionID, "id", nil)
	if err != nil || score != 1 {
		t.Fatalf("retry score=%d err=%v; want the original result", score, err)
	}
	hub := svc.GetHub(ctx, "pilot", tz, 1)
	if hub.WeeklyDiamondScore != 1 || len(hub.History) != 1 || hub.History[0].ID != session.SessionID {
		t.Fatalf("ranked sprint not recorded exactly once: %+v", hub)
	}
}

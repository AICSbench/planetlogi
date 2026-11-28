package supernova

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/AICSbench/planetlogi/apps/api/internal/mathengine"
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/economy"
)

type retryScoreLedger struct {
	economy.Ledger
	fail     bool
	credited int64
}

func (l *retryScoreLedger) Apply(ctx context.Context, req economy.MutationRequest) (economy.MutationResult, error) {
	if l.fail {
		return economy.MutationResult{}, errors.New("ledger unavailable")
	}
	result, err := l.Ledger.Apply(ctx, req)
	if err == nil && !result.WasDuplicate && req.Currency == economy.CurrencySuperS {
		l.credited += req.Delta
	}
	return result, err
}

func TestSuperWarpRetryKeepsOnTimeScoreAfterDeadline(t *testing.T) {
	ctx := context.Background()
	ledger := &retryScoreLedger{Ledger: economy.NewMemoryLedger(), fail: true}
	svc := NewService(ledger)
	const student = "pilot"
	const tz = "Asia/Jakarta"
	now := time.Date(2026, 12, 1, 3, 0, 0, 0, time.UTC)
	svc.SetClockForTest(func() time.Time { return now })

	start, err := svc.StartSuperWarp(student, tz, "en", 3, false)
	if err != nil {
		t.Fatal(err)
	}
	// Recover the answer key from the server fixture; the client never gets it.
	session := svc.warpSess[start.SessionID]
	problem, ok := mathengine.GenerateByCode(session.templates[0], session.seeds[0], "en")
	if !ok {
		t.Fatal("first question template missing")
	}
	answers := map[int]map[string]string{1: problem.Solution.CorrectAnswers}
	if _, _, err := svc.FinishSuperWarp(ctx, "other-pilot", start.SessionID, "en", answers); err == nil {
		t.Fatal("another pilot could submit this session")
	}
	if _, _, err := svc.FinishSuperWarp(ctx, student, start.SessionID, "en", answers); !errors.Is(err, ErrRewardPending) {
		t.Fatalf("first finish: got %v, want pending reward", err)
	}
	if hub := svc.GetHub(ctx, student, tz, 1); hub.WeeklyDiamondScore != 0 || len(hub.History) != 0 {
		t.Fatalf("failed finish was recorded: %+v", hub)
	}

	now = now.Add((WarpDurationS + WarpFinishGraceS + 1) * time.Second)
	ledger.fail = false
	for _, attempt := range []string{"retry", "replay"} {
		score, hub, err := svc.FinishSuperWarp(ctx, student, start.SessionID, "en", nil)
		if err != nil || score != 1 || hub.WeeklyDiamondScore != 1 || ledger.credited != 1 {
			t.Fatalf("%s: score=%d weekly=%d credited=%d err=%v", attempt, score, hub.WeeklyDiamondScore, ledger.credited, err)
		}
		if len(hub.History) != 1 || hub.History[0].ID != start.SessionID || hub.History[0].Score != 1 {
			t.Fatalf("%s: expected one history entry for the original score, got %+v", attempt, hub.History)
		}
	}
}

func TestSuperSightRetryDoesNotAcceptNewHits(t *testing.T) {
	ctx := context.Background()
	ledger := &retryScoreLedger{Ledger: economy.NewMemoryLedger(), fail: true}
	svc := NewService(ledger)
	now := time.Date(2026, 12, 1, 3, 0, 0, 0, time.UTC)
	svc.SetClockForTest(func() time.Time { return now })
	start, err := svc.StartSuperSight("pilot", "Asia/Jakarta", "en", false)
	if err != nil {
		t.Fatal(err)
	}
	tap := func(x, y float64) (bool, SuperSightSession, error) {
		return svc.TapSuperSight(ctx, "pilot", start.SessionID, x, y, "en")
	}
	if hit, _, err := tap(0.18, 0.25); err != nil || !hit {
		t.Fatalf("initial hit: hit=%v err=%v", hit, err)
	}
	// The first four duplicate taps consume attempts without scoring again.
	for wrong := 1; wrong < SuperSightMaxWrong; wrong++ {
		if hit, view, err := tap(0.18, 0.25); err != nil || hit || view.WrongCount != wrong || view.Score != 10 {
			t.Fatalf("duplicate tap %d: hit=%v view=%+v err=%v", wrong, hit, view, err)
		}
	}
	if _, view, err := tap(0, 0); !errors.Is(err, ErrRewardPending) || view.Completed || view.WrongCount != SuperSightMaxWrong {
		t.Fatalf("final miss should leave a frozen session: view=%+v err=%v", view, err)
	}

	ledger.fail = false
	for _, attempt := range []string{"retry", "replay"} {
		hit, view, err := tap(0.50, 0.22)
		if !errors.Is(err, ErrSessionEnded) || hit || !view.Completed || view.FoundCount != 1 || view.Score != 10 || ledger.credited != 10 {
			t.Fatalf("%s changed frozen result: hit=%v view=%+v credited=%d err=%v", attempt, hit, view, ledger.credited, err)
		}
	}
}

package supernova

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSuperSightScoringPracticeAndPackValidation(t *testing.T) {
	svc := NewService(nil)
	// Fix the clock inside the ranked window (Tuesday 10:00 Asia/Jakarta) so
	// enforceOpen never makes the test flaky.
	svc.SetClockForTest(func() time.Time {
		return time.Date(2026, 12, 1, 10, 0, 0, 0, time.FixedZone("WIB", 7*3600))
	})
	ctx := context.Background()

	// Practice mode does NOT increment weekly diamond S score
	pSess, err := svc.StartSuperSight("stu-1", "Asia/Jakarta", "en", true)
	if err != nil {
		t.Fatal(err)
	}
	coords := [][2]float64{{0.18, 0.25}, {0.50, 0.22}, {0.82, 0.28}, {0.30, 0.72}, {0.74, 0.75}}
	for _, pt := range coords {
		hit, next, err := svc.TapSuperSight(context.Background(), "stu-1", pSess.SessionID, pt[0], pt[1], "en")
		if err != nil || !hit {
			t.Fatalf("expected hit at %v: hit=%v err=%v", pt, hit, err)
		}
		pSess = next
	}
	if !pSess.Completed || pSess.Score != 50 {
		t.Fatalf("expected 50 pts completed, got %+v", pSess)
	}
	if hub := svc.GetHub(ctx, "stu-1", "Asia/Jakarta", 1); hub.WeeklyDiamondScore != 0 {
		t.Fatalf("practice mode must not change weekly S score, got %d", hub.WeeklyDiamondScore)
	}

	// Ranked mode increments weekly diamond S score
	rSess, _ := svc.StartSuperSight("stu-1", "Asia/Jakarta", "en", false)
	for _, pt := range coords {
		_, rSess, _ = svc.TapSuperSight(context.Background(), "stu-1", rSess.SessionID, pt[0], pt[1], "en")
	}
	if hub := svc.GetHub(ctx, "stu-1", "Asia/Jakarta", 1); hub.WeeklyDiamondScore != 50 {
		t.Fatalf("expected weekly S score 50, got %d", hub.WeeklyDiamondScore)
	}

	// Validate 30 Super Sight puzzle pack JSON
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "content", "super-sight", "puzzles-30.json"))
	if err != nil {
		t.Fatalf("missing puzzles-30.json: %v", err)
	}
	var doc struct {
		Puzzles []struct {
			ID      string       `json:"id"`
			Targets []DiffTarget `json:"targets"`
		} `json:"puzzles"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil || len(doc.Puzzles) != 30 {
		t.Fatalf("expected 30 valid puzzles, err=%v len=%d", err, len(doc.Puzzles))
	}
	ids := make(map[string]bool)
	for _, pz := range doc.Puzzles {
		if pz.ID == "" || ids[pz.ID] {
			t.Fatalf("empty or duplicate puzzle ID %q", pz.ID)
		}
		ids[pz.ID] = true
		if len(pz.Targets) != 5 {
			t.Fatalf("puzzle %s must have 5 targets", pz.ID)
		}
		for i, target := range pz.Targets {
			if target.Radius <= 0 || target.X-target.Radius < 0 || target.X+target.Radius > 1 ||
				target.Y-target.Radius < 0 || target.Y+target.Radius > 1 {
				t.Errorf("puzzle %s target %+v falls outside the normalized scene", pz.ID, target)
			}
			for _, other := range pz.Targets[:i] {
				if target.ID == other.ID || math.Hypot(target.X-other.X, target.Y-other.Y) < target.Radius+other.Radius {
					t.Errorf("puzzle %s targets %d and %d conflict", pz.ID, target.ID, other.ID)
				}
			}
		}
	}
}

func TestSuperNovaWindowEnforcedByDefault(t *testing.T) {
	svc := NewService(nil)
	// Sunday — outside the Mon-Sat ranked window.
	svc.SetClockForTest(func() time.Time {
		return time.Date(2026, 11, 29, 10, 0, 0, 0, time.FixedZone("WIB", 7*3600))
	})
	if _, err := svc.StartSuperSight("stu-1", "Asia/Jakarta", "en", false); err != ErrSuperNovaClosed {
		t.Fatalf("expected ErrSuperNovaClosed outside ranked window, got %v", err)
	}
	if _, err := svc.StartSuperWarp("stu-1", "Asia/Jakarta", "en", 3, false); err != ErrSuperNovaClosed {
		t.Fatalf("expected ErrSuperNovaClosed outside ranked window, got %v", err)
	}
	// Practice mode stays open regardless of the window.
	if _, err := svc.StartSuperSight("stu-1", "Asia/Jakarta", "en", true); err != nil {
		t.Fatalf("practice must stay open outside the window, got %v", err)
	}
	// Demo deployments can relax the window via config.
	svc.SetEnforceWindow(false)
	if _, err := svc.StartSuperSight("stu-1", "Asia/Jakarta", "en", false); err != nil {
		t.Fatalf("expected ranked start with enforceOpen=false, got %v", err)
	}
}

func TestSuperWarpDeadlineExpiresScore(t *testing.T) {
	svc := NewService(nil)
	base := time.Date(2026, 12, 1, 10, 0, 0, 0, time.FixedZone("WIB", 7*3600))
	svc.SetClockForTest(func() time.Time { return base })
	ctx := context.Background()

	start, err := svc.StartSuperWarp("stu-1", "Asia/Jakarta", "en", 3, false)
	if err != nil {
		t.Fatal(err)
	}
	// Submit 3 seconds past the deadline (120s + 3s grace).
	svc.SetClockForTest(func() time.Time { return base.Add((WarpDurationS + WarpFinishGraceS + 3) * time.Second) })
	score, _, err := svc.FinishSuperWarp(ctx, "stu-1", start.SessionID, "en", map[int]map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	if score != 0 {
		t.Fatalf("expected expired warp to score 0, got %d", score)
	}
	if hub := svc.GetHub(ctx, "stu-1", "Asia/Jakarta", 1); hub.WeeklyDiamondScore != 0 {
		t.Fatalf("expired warp must not change weekly S score, got %d", hub.WeeklyDiamondScore)
	}
}

func TestSuperSightDeadlineEndsSession(t *testing.T) {
	svc := NewService(nil)
	base := time.Date(2026, 12, 1, 10, 0, 0, 0, time.FixedZone("WIB", 7*3600))
	svc.SetClockForTest(func() time.Time { return base })
	ctx := context.Background()

	sess, err := svc.StartSuperSight("stu-1", "Asia/Jakarta", "en", true)
	if err != nil {
		t.Fatal(err)
	}
	// One correct tap, then cross the 5-minute deadline.
	_, next, err := svc.TapSuperSight(ctx, "stu-1", sess.SessionID, 0.18, 0.25, "en")
	if err != nil {
		t.Fatal(err)
	}
	svc.SetClockForTest(func() time.Time { return base.Add((SuperSightTimeLimitS + 5) * time.Second) })
	_, next, err = svc.TapSuperSight(ctx, "stu-1", next.SessionID, 0.50, 0.22, "en")
	if err == nil || !errors.Is(err, ErrSessionEnded) {
		t.Fatalf("expected ErrSessionEnded after deadline, got %v", err)
	}
	if !next.Completed || next.Score != SuperSightPointsEach {
		t.Fatalf("expected completed session with 10 pts, got %+v", next)
	}
}

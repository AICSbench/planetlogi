package supernova

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSuperSightScoringPracticeAndPackValidation(t *testing.T) {
	svc := NewService(nil)

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
	if hub := svc.GetHub("stu-1", "Asia/Jakarta"); hub.WeeklyDiamondScore != 0 {
		t.Fatalf("practice mode must not change weekly S score, got %d", hub.WeeklyDiamondScore)
	}

	// Ranked mode increments weekly diamond S score
	rSess, _ := svc.StartSuperSight("stu-1", "Asia/Jakarta", "en", false)
	for _, pt := range coords {
		_, rSess, _ = svc.TapSuperSight(context.Background(), "stu-1", rSess.SessionID, pt[0], pt[1], "en")
	}
	if hub := svc.GetHub("stu-1", "Asia/Jakarta"); hub.WeeklyDiamondScore != 50 {
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
	for _, pz := range doc.Puzzles {
		if len(pz.Targets) != 5 {
			t.Fatalf("puzzle %s must have 5 targets", pz.ID)
		}
	}
}

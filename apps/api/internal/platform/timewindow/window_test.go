package timewindow

import (
	"testing"
	"time"
)

func TestEvaluateOperatingWindows(t *testing.T) {
	jakarta := LoadLocationSafe("Asia/Jakarta")

	// 05:59 WIB -> closed for Daily Orbit
	beforeOpen := time.Date(2026, 11, 26, 5, 59, 0, 0, jakarta)
	if eval := Evaluate(beforeOpen, "Asia/Jakarta", DailyOrbitWindow); eval.IsOpen {
		t.Fatalf("expected closed at 05:59 WIB")
	}

	// 06:00 WIB -> open for Daily Orbit
	atOpen := time.Date(2026, 11, 26, 6, 0, 0, 0, jakarta)
	if eval := Evaluate(atOpen, "Asia/Jakarta", DailyOrbitWindow); !eval.IsOpen || eval.LocalDate != "2026-11-26" {
		t.Fatalf("expected open at 06:00 WIB, got %+v", eval)
	}

	// 22:00 WIB -> closed for Daily Orbit
	atClose := time.Date(2026, 11, 26, 22, 0, 0, 0, jakarta)
	if eval := Evaluate(atClose, "Asia/Jakarta", DailyOrbitWindow); eval.IsOpen {
		t.Fatalf("expected closed at 22:00 WIB")
	}

	// Sunday check: 2026-11-29 is Sunday
	sundayNoon := time.Date(2026, 11, 29, 12, 0, 0, 0, jakarta)
	if eval := Evaluate(sundayNoon, "Asia/Jakarta", SundayMeteorWindow); !eval.IsOpen {
		t.Fatalf("expected Sunday Meteor open on Sunday noon")
	}
	if eval := Evaluate(sundayNoon, "Asia/Jakarta", SuperNovaWindow); eval.IsOpen {
		t.Fatalf("expected Super Nova closed on Sunday")
	}

	// DST timezone verification (America/New_York spring-forward day 2026-03-08)
	ny := LoadLocationSafe("America/New_York")
	dstDay := time.Date(2026, 3, 8, 9, 30, 0, 0, ny)
	if eval := Evaluate(dstDay, "America/New_York", BrainGalaxyDefaultWindow); !eval.IsOpen || eval.LocalDate != "2026-03-08" {
		t.Fatalf("expected Brain Galaxy open on NY DST morning, got %+v", eval)
	}
}

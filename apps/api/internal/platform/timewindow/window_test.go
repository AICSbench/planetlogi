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

	// Invalid timezone falls back to Asia/Jakarta (UTC+7): 23:30 UTC on Nov 25 is 06:30 WIB on Nov 26 (OPEN)
	utcInstant := time.Date(2026, 11, 25, 23, 30, 0, 0, time.UTC)
	fallbackEval := Evaluate(utcInstant, "Invalid/Timezone_Name", DailyOrbitWindow)
	if !fallbackEval.IsOpen || fallbackEval.LocalDate != "2026-11-26" {
		t.Fatalf("expected invalid tz to fall back to Asia/Jakarta (2026-11-26 open), got %+v", fallbackEval)
	}

	// Sunday check: 2026-11-29 is Sunday
	sundayNoon := time.Date(2026, 11, 29, 12, 0, 0, 0, jakarta)
	if eval := Evaluate(sundayNoon, "Asia/Jakarta", SundayMeteorWindow); !eval.IsOpen {
		t.Fatalf("expected Sunday Meteor open on Sunday noon")
	}
	if eval := Evaluate(sundayNoon, "Asia/Jakarta", SuperNovaWindow); eval.IsOpen {
		t.Fatalf("expected Super Nova closed on Sunday")
	}

	// Real UTC->America/New_York DST conversion check:
	// On 2026-03-08 (EDT = UTC-4 after 02:00 spring-forward), 12:30 UTC is 08:30 EDT (OPEN), whereas 11:30 UTC is 07:30 EDT (CLOSED).
	nyClosedUTC := time.Date(2026, 3, 8, 11, 30, 0, 0, time.UTC)
	if eval := Evaluate(nyClosedUTC, "America/New_York", BrainGalaxyDefaultWindow); eval.IsOpen {
		t.Fatalf("expected Brain Galaxy closed at 07:30 EDT (11:30 UTC), got %+v", eval)
	}
	nyOpenUTC := time.Date(2026, 3, 8, 12, 30, 0, 0, time.UTC)
	if eval := Evaluate(nyOpenUTC, "America/New_York", BrainGalaxyDefaultWindow); !eval.IsOpen || eval.LocalDate != "2026-03-08" {
		t.Fatalf("expected Brain Galaxy open at 08:30 EDT (12:30 UTC), got %+v", eval)
	}
}

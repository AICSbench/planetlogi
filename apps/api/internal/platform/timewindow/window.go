package timewindow

import (
	"fmt"
	"time"
)

// Window defines an authoritative daily operating window in the user's local timezone.
type Window struct {
	StartHour   int
	StartMinute int
	EndHour     int
	EndMinute   int
	AllowedDays []time.Weekday // Empty means all 7 days
}

var (
	DailyOrbitWindow         = Window{StartHour: 6, EndHour: 22}
	SuperNovaWindow          = Window{StartHour: 6, EndHour: 22, AllowedDays: []time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday, time.Saturday}}
	SundayMeteorWindow       = Window{StartHour: 6, EndHour: 22, AllowedDays: []time.Weekday{time.Sunday}}
	BrainGalaxySchoolWindow  = Window{StartHour: 8, EndHour: 18}
	BrainGalaxyDefaultWindow = Window{StartHour: 8, EndHour: 20}
)

// Evaluation describes whether a feature is currently open for a user.
type Evaluation struct {
	IsOpen    bool      `json:"isOpen"`
	LocalDate string    `json:"localDate"`
	LocalTime time.Time `json:"localTime"`
	Reason    string    `json:"reason,omitempty"`
}

// LoadLocationSafe resolves an IANA timezone name with Asia/Jakarta fallback.
func LoadLocationSafe(tz string) *time.Location {
	if tz == "" {
		tz = "Asia/Jakarta"
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc, err = time.LoadLocation("Asia/Jakarta")
		if err != nil {
			loc = time.FixedZone("WIB", 7*3600)
		}
	}
	return loc
}

// LocalDateString returns YYYY-MM-DD in the student's configured timezone.
func LocalDateString(now time.Time, tz string) string {
	return now.In(LoadLocationSafe(tz)).Format("2006-01-02")
}

// Evaluate checks if the given instant falls within the operating window in tz.
func Evaluate(now time.Time, tz string, w Window) Evaluation {
	local := now.In(LoadLocationSafe(tz))
	eval := Evaluation{
		LocalDate: local.Format("2006-01-02"),
		LocalTime: local,
	}

	if len(w.AllowedDays) > 0 {
		dayOK := false
		for _, d := range w.AllowedDays {
			if local.Weekday() == d {
				dayOK = true
				break
			}
		}
		if !dayOK {
			eval.IsOpen = false
			eval.Reason = fmt.Sprintf("closed on %s", local.Weekday().String())
			return eval
		}
	}

	minutesNow := local.Hour()*60 + local.Minute()
	startMinutes := w.StartHour*60 + w.StartMinute
	endMinutes := w.EndHour*60 + w.EndMinute

	if minutesNow >= startMinutes && minutesNow < endMinutes {
		eval.IsOpen = true
		return eval
	}

	eval.IsOpen = false
	eval.Reason = fmt.Sprintf("open %02d:%02d-%02d:%02d local time", w.StartHour, w.StartMinute, w.EndHour, w.EndMinute)
	return eval
}

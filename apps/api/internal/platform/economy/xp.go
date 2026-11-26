package economy

import "math"

const (
	MinPilotLevel = 1
	MaxPilotLevel = 100
)

// XPRequiredForLevel returns the XP delta required to advance from level L to L+1 (1 <= L < 100).
// Matches docs/05-gamification-social-and-safety.md §1.2.
func XPRequiredForLevel(level int) int64 {
	if level < MinPilotLevel {
		level = MinPilotLevel
	}
	if level >= MaxPilotLevel {
		return 0
	}
	if level == 1 {
		return 4
	}
	d := float64(level - 1)
	return int64(math.Round(4.0 + 6.0*d + 1.8*math.Pow(d, 1.55)))
}

type LevelProgress struct {
	Level          int   `json:"level"`
	XPIntoLevel    int64 `json:"xpIntoLevel"`
	XPForNextLevel int64 `json:"xpForNextLevel"`
}

// ComputeLevelProgress calculates Pilot Level (1..100) from cumulative Pilot XP.
func ComputeLevelProgress(totalXP int64) LevelProgress {
	if totalXP < 0 {
		totalXP = 0
	}
	remaining := totalXP
	level := MinPilotLevel
	for level < MaxPilotLevel {
		req := XPRequiredForLevel(level)
		if remaining < req {
			return LevelProgress{
				Level:          level,
				XPIntoLevel:    remaining,
				XPForNextLevel: req,
			}
		}
		remaining -= req
		level++
	}
	return LevelProgress{
		Level:          MaxPilotLevel,
		XPIntoLevel:    0,
		XPForNextLevel: 0,
	}
}

package streak

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type pgExecutor interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	QueryRow(context.Context, string, ...any) pgx.Row
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func isUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i := 0; i < 36; i++ {
		c := s[i]
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				return false
			}
		}
	}
	return true
}

func (s *Service) subjectIDPG(ctx context.Context, db pgExecutor) (string, error) {
	s.subjectMu.Lock()
	cached := s.subjectID
	s.subjectMu.Unlock()
	if cached != "" {
		return cached, nil
	}
	var id string
	err := db.QueryRow(ctx, `SELECT id::text FROM subjects WHERE code = 'math' LIMIT 1`).Scan(&id)
	if err != nil {
		return "", err
	}
	s.subjectMu.Lock()
	if s.subjectID == "" {
		s.subjectID = id
	}
	id = s.subjectID
	s.subjectMu.Unlock()
	return id, nil
}

func (s *Service) loadRecordPG(ctx context.Context, db pgExecutor, studentID string) (*record, error) {
	if s.pool == nil || !isUUID(studentID) {
		return nil, nil
	}
	subjID, err := s.subjectIDPG(ctx, db)
	if err != nil {
		return nil, err
	}

	var st Status
	var lastCompleted *time.Time
	var todayDate *time.Time
	var todayCorrect int
	var reminderTime string
	err = db.QueryRow(ctx, `
		SELECT current_streak, longest_streak, cryo_shields, nova_pet_stage,
		       last_completed_local_date, reminder_enabled, today_correct_count,
		       today_local_date, reminder_time_local
		FROM student_streaks
		WHERE student_id = $1::uuid AND subject_id = $2::uuid
	`, studentID, subjID).Scan(
		&st.CurrentStreak,
		&st.LongestStreak,
		&st.CryoShields,
		&st.NovaPetStage,
		&lastCompleted,
		&st.ReminderEnabled,
		&todayCorrect,
		&todayDate,
		&reminderTime,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		st = Status{
			CurrentStreak:        3,
			LongestStreak:        3,
			CryoShields:          2,
			MaxCryoShields:       MaxCryoShields,
			NovaPetStage:         NovaPetStageForStreak(3),
			DailyMilestoneTarget: DailyStreakMilestone,
			ReminderTimeLocal:    "19:00",
		}
		_, err = db.Exec(ctx, `
			INSERT INTO student_streaks
				(student_id, subject_id, current_streak, longest_streak, cryo_shields, nova_pet_stage, reminder_enabled, today_correct_count, reminder_time_local)
			VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, 0, $8)
			ON CONFLICT (student_id, subject_id) DO NOTHING
		`, studentID, subjID, st.CurrentStreak, st.LongestStreak, st.CryoShields, st.NovaPetStage, st.ReminderEnabled, st.ReminderTimeLocal)
		if err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	} else {
		st.MaxCryoShields = MaxCryoShields
		st.DailyMilestoneTarget = DailyStreakMilestone
		st.TodayCorrectCount = todayCorrect
		if strings.TrimSpace(reminderTime) != "" {
			st.ReminderTimeLocal = reminderTime
		} else {
			st.ReminderTimeLocal = "19:00"
		}
		if lastCompleted != nil {
			st.LastCompletedDate = lastCompleted.Format("2006-01-02")
		}
	}

	cal := make(map[string]DayStatus)
	rows, err := db.Query(ctx, `
		SELECT local_date, status
		FROM streak_calendar_days
		WHERE student_id = $1::uuid AND subject_id = $2::uuid
	`, studentID, subjID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var dt time.Time
		var dst string
		if err := rows.Scan(&dt, &dst); err != nil {
			return nil, err
		}
		cal[dt.Format("2006-01-02")] = DayStatus(dst)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	rec := &record{
		status:   st,
		calendar: cal,
	}
	if todayDate != nil {
		rec.todayDate = todayDate.Format("2006-01-02")
	}
	return rec, nil
}

func (s *Service) saveRecordPG(ctx context.Context, studentID string, rec *record, changedDate string) error {
	if s.pool == nil || !isUUID(studentID) {
		return nil
	}
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		return s.writeRecordPG(ctx, tx, studentID, rec, changedDate)
	})
}

func (s *Service) writeRecordPG(ctx context.Context, db pgExecutor, studentID string, rec *record, changedDate string) error {
	if s.pool == nil || !isUUID(studentID) {
		return nil
	}
	subjID, err := s.subjectIDPG(ctx, db)
	if err != nil {
		return err
	}

	var lastComp any
	if rec.status.LastCompletedDate != "" {
		lastComp = rec.status.LastCompletedDate
	}
	var todayDt any
	if rec.todayDate != "" {
		todayDt = rec.todayDate
	}

	if _, err := db.Exec(ctx, `
		INSERT INTO student_streaks
			(student_id, subject_id, current_streak, longest_streak, cryo_shields, nova_pet_stage,
			 last_completed_local_date, reminder_enabled, today_correct_count, today_local_date, reminder_time_local, updated_at)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7::date, $8, $9, $10::date, $11, NOW())
		ON CONFLICT (student_id, subject_id) DO UPDATE SET
			current_streak = EXCLUDED.current_streak,
			longest_streak = EXCLUDED.longest_streak,
			cryo_shields = EXCLUDED.cryo_shields,
			nova_pet_stage = EXCLUDED.nova_pet_stage,
			last_completed_local_date = EXCLUDED.last_completed_local_date,
			reminder_enabled = EXCLUDED.reminder_enabled,
			today_correct_count = EXCLUDED.today_correct_count,
			today_local_date = EXCLUDED.today_local_date,
			reminder_time_local = EXCLUDED.reminder_time_local,
			updated_at = NOW()
	`, studentID, subjID, rec.status.CurrentStreak, rec.status.LongestStreak, rec.status.CryoShields,
		rec.status.NovaPetStage, lastComp, rec.status.ReminderEnabled, rec.status.TodayCorrectCount,
		todayDt, rec.status.ReminderTimeLocal); err != nil {
		return err
	}

	if changedDate != "" {
		if st, ok := rec.calendar[changedDate]; ok {
			if _, err := db.Exec(ctx, `
				INSERT INTO streak_calendar_days (student_id, subject_id, local_date, status)
				VALUES ($1::uuid, $2::uuid, $3::date, $4)
				ON CONFLICT (student_id, subject_id, local_date) DO UPDATE SET status = EXCLUDED.status
			`, studentID, subjID, changedDate, string(st)); err != nil {
				return err
			}
		}
	}
	return nil
}

// resetTodayPG clears only the dated counter, so reminder settings and newer
// progress written by another instance are left untouched.
func (s *Service) resetTodayPG(ctx context.Context, db pgExecutor, studentID, localDate string) error {
	subjID, err := s.subjectIDPG(ctx, db)
	if err != nil {
		return err
	}
	_, err = db.Exec(ctx, `
		UPDATE student_streaks
		SET today_correct_count = 0, today_local_date = $3::date, updated_at = NOW()
		WHERE student_id = $1::uuid AND subject_id = $2::uuid
		  AND (today_local_date IS NULL OR today_local_date <> $3::date)
	`, studentID, subjID, localDate)
	return err
}

// ReconcileAllMidnight reconciles missed days for all students persisted in PostgreSQL
// (or in-memory records when running without a DB pool).
func (s *Service) ReconcileAllMidnight(ctx context.Context, now time.Time) (int, error) {
	type target struct {
		id string
		tz string
	}
	var targets []target

	if s.pool != nil {
		rows, err := s.pool.Query(ctx, `SELECT id::text, timezone FROM student_profiles`)
		if err != nil {
			return 0, err
		}
		defer rows.Close()
		for rows.Next() {
			var t target
			if err := rows.Scan(&t.id, &t.tz); err == nil {
				if t.tz == "" {
					t.tz = "Asia/Jakarta"
				}
				targets = append(targets, t)
			}
		}
		if err := rows.Err(); err != nil {
			return 0, err
		}
	} else {
		s.mu.Lock()
		for id := range s.records {
			targets = append(targets, target{id: id, tz: "Asia/Jakarta"})
		}
		s.mu.Unlock()
	}

	for _, t := range targets {
		if _, err := s.reconcileMidnightErr(ctx, t.id, t.tz, now); err != nil {
			return 0, err
		}
	}
	return len(targets), nil
}

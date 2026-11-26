package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"golang.org/x/crypto/bcrypt"

	dbmigrations "github.com/AICSbench/planetlogi/apps/api/db"
	"github.com/AICSbench/planetlogi/apps/api/internal/config"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg := config.Load()
	ctx := context.Background()

	sqlDB, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		logger.Error("open sql db failed", "error", err)
		os.Exit(1)
	}
	defer func() { _ = sqlDB.Close() }()

	if err := dbmigrations.Up(ctx, sqlDB); err != nil {
		logger.Error("migrations failed", "error", err)
		os.Exit(1)
	}

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("pgxpool init failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := seedCore(ctx, pool); err != nil {
		logger.Error("seed failed", "error", err)
		os.Exit(1)
	}
	logger.Info("planetlogi seed completed cleanly")
}

func seedCore(ctx context.Context, pool *pgxpool.Pool) error {
	// 1. Seed multi-subject catalog (only Math active at launch)
	subjects := []struct {
		code   string
		en     string
		id     string
		active bool
		order  int
	}{
		{"math", "Mathematics", "Matematika", true, 1},
		{"science", "Science", "IPA", false, 2},
		{"english", "English", "Bahasa Inggris", false, 3},
	}

	var mathSubjectID string
	for _, s := range subjects {
		var id string
		err := pool.QueryRow(ctx, `
			INSERT INTO subjects (code, translations, is_active, sort_order)
			VALUES ($1, jsonb_build_object('en', $2::text, 'id', $3::text), $4, $5)
			ON CONFLICT (code) DO UPDATE SET translations = EXCLUDED.translations
			RETURNING id
		`, s.code, s.en, s.id, s.active, s.order).Scan(&id)
		if err != nil {
			return fmt.Errorf("seed subject %s: %w", s.code, err)
		}
		if s.code == "math" {
			mathSubjectID = id
		}
	}

	// 2. Seed 6 elementary math curriculum shells
	curricula := []struct {
		code  string
		en    string
		id    string
		order int
	}{
		{"KURIKULUM_MERDEKA", "Kurikulum Merdeka (Indonesia)", "Kurikulum Merdeka (SD)", 1},
		{"SINGAPORE_MOE", "Singapore Math MOE", "Matematika Singapura (MOE)", 2},
		{"CAMBRIDGE_0096", "Cambridge Primary 0096", "Cambridge Primary 0096", 3},
		{"CHALLENGING_MATHS", "Challenging Maths", "Matematika Tantangan", 4},
		{"HOTS_WHEEL", "HOTS Constellation", "HOTS (Strategi Pemecahan Masalah)", 5},
		{"EXPLORER_SERIES", "Real-World Explorer Series", "Seri Eksplorasi Dunia Nyata", 6},
	}
	for _, c := range curricula {
		if _, err := pool.Exec(ctx, `
			INSERT INTO curricula (subject_id, code, translations, sort_order, is_active)
			VALUES ($1, $2, jsonb_build_object('en', $3::text, 'id', $4::text), $5, TRUE)
			ON CONFLICT (code) DO NOTHING
		`, mathSubjectID, c.code, c.en, c.id, c.order); err != nil {
			return fmt.Errorf("seed curriculum %s: %w", c.code, err)
		}
	}

	// 3. Seed demo school, parent, and Grade 1 student (pilot1 / planet123)
	var schoolID string
	err := pool.QueryRow(ctx, `
		INSERT INTO schools (name, country_code, academic_rollover_month, academic_rollover_day)
		VALUES ('SDN Bintang Galaksi 01', 'ID', 7, 1)
		RETURNING id
	`).Scan(&schoolID)
	if err != nil {
		return fmt.Errorf("seed school: %w", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte("planet123"), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash demo password: %w", err)
	}

	var parentID string
	err = pool.QueryRow(ctx, `
		INSERT INTO user_accounts (role, email, display_name, password_hash, otp_enabled, preferred_locale, timezone)
		VALUES ('PARENT', 'parent@planetlogi.local', 'Budi Santoso', $1, FALSE, 'id', 'Asia/Jakarta')
		ON CONFLICT (email) DO UPDATE SET display_name = EXCLUDED.display_name
		RETURNING id
	`, string(hash)).Scan(&parentID)
	if err != nil {
		return fmt.Errorf("seed parent: %w", err)
	}

	var studentID string
	err = pool.QueryRow(ctx, `
		INSERT INTO student_profiles (
			parent_account_id, school_id, login_id, password_hash, first_name, masked_display_name,
			grade, preferred_locale, timezone, crew_code, next_promotion_date
		)
		VALUES ($1, $2, 'pilot1', $3, 'Raka', 'Raka S.', 1, 'id', 'Asia/Jakarta', '100200300', '2027-07-01')
		ON CONFLICT (login_id) DO UPDATE SET first_name = EXCLUDED.first_name
		RETURNING id
	`, parentID, schoolID, string(hash)).Scan(&studentID)
	if err != nil {
		return fmt.Errorf("seed student: %w", err)
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO student_wallets (student_id, total_sp, astro_coins, pilot_xp, pilot_level, total_star_scrolls)
		VALUES ($1, 12, 25, 10, 2, 0)
		ON CONFLICT (student_id) DO NOTHING
	`, studentID); err != nil {
		return fmt.Errorf("seed wallet: %w", err)
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO student_streaks (student_id, subject_id, current_streak, longest_streak, cryo_shields, nova_pet_stage)
		VALUES ($1, $2, 3, 3, 2, 2)
		ON CONFLICT (student_id, subject_id) DO NOTHING
	`, studentID, mathSubjectID); err != nil {
		return fmt.Errorf("seed streak: %w", err)
	}

	return nil
}

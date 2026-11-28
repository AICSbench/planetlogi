package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"golang.org/x/crypto/bcrypt"

	dbmigrations "github.com/AICSbench/planetlogi/apps/api/db"
	"github.com/AICSbench/planetlogi/apps/api/internal/config"
	"github.com/AICSbench/planetlogi/apps/api/internal/mathengine"
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

	if err := seedCore(ctx, pool, logger); err != nil {
		logger.Error("seed failed", "error", err)
		os.Exit(1)
	}
	logger.Info("planetlogi seed completed cleanly")
}

func resolveContentFile(rel string) (string, bool) {
	candidates := []string{
		rel,
		filepath.Join("..", rel),
		filepath.Join("..", "..", rel),
		filepath.Join("/app", rel),
	}
	if dir := os.Getenv("CONTENT_DIR"); dir != "" {
		candidates = append([]string{filepath.Join(dir, rel)}, candidates...)
	}
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, true
		}
	}
	return "", false
}

func seedCore(ctx context.Context, pool *pgxpool.Pool, logger *slog.Logger) error {
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

	// 3. Seed demo school (stable UUID so repeated runs reuse the same school), parent, and Grade 1 student (pilot1 / planet123)
	const demoSchoolUUID = "00000000-0000-0000-0000-000000000001"
	var schoolID string
	err := pool.QueryRow(ctx, `
		INSERT INTO schools (id, name, country_code, academic_rollover_month, academic_rollover_day)
		VALUES ($1::uuid, 'SDN Bintang Galaksi 01', 'ID', 7, 1)
		ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name
		RETURNING id
	`, demoSchoolUUID).Scan(&schoolID)
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
		ON CONFLICT (LOWER(email)) WHERE email IS NOT NULL DO UPDATE SET display_name = EXCLUDED.display_name
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
		ON CONFLICT (LOWER(login_id)) DO UPDATE SET first_name = EXCLUDED.first_name
		RETURNING id
	`, parentID, schoolID, string(hash)).Scan(&studentID)
	if err != nil {
		return fmt.Errorf("seed student: %w", err)
	}

	walletTx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin wallet seed tx: %w", err)
	}
	defer func() { _ = walletTx.Rollback(ctx) }()

	walletTag, err := walletTx.Exec(ctx, `
		INSERT INTO student_wallets (student_id, total_sp, astro_coins, pilot_xp, pilot_level, total_star_scrolls)
		VALUES ($1, 12, 25, 10, 2, 0)
		ON CONFLICT (student_id) DO NOTHING
	`, studentID)
	if err != nil {
		return fmt.Errorf("seed wallet: %w", err)
	}

	// Record opening ledger entries only when this seed initializes the wallet row
	if walletTag.RowsAffected() > 0 {
		openingEntries := []struct {
			currency string
			amount   int64
			idemKey  string
		}{
			{"SP", 12, "seed:pilot1:opening:sp"},
			{"AC", 25, "seed:pilot1:opening:ac"},
			{"XP", 10, "seed:pilot1:opening:xp"},
		}
		for _, entry := range openingEntries {
			if _, err := walletTx.Exec(ctx, `
				INSERT INTO currency_transactions
					(student_id, subject_id, currency, delta, balance_after, source_type, source_id, idempotency_key)
				VALUES ($1::uuid, $2::uuid, $3, $4, $4, 'SEED_OPENING_BALANCE', 'seed', $5)
				ON CONFLICT (idempotency_key) DO NOTHING
			`, studentID, mathSubjectID, entry.currency, entry.amount, entry.idemKey); err != nil {
				return fmt.Errorf("seed ledger %s: %w", entry.currency, err)
			}
		}
	}

	if err := walletTx.Commit(ctx); err != nil {
		return fmt.Errorf("commit wallet seed tx: %w", err)
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO student_streaks (student_id, subject_id, current_streak, longest_streak, cryo_shields, nova_pet_stage)
		VALUES ($1, $2, 3, 3, 2, 2)
		ON CONFLICT (student_id, subject_id) DO NOTHING
	`, studentID, mathSubjectID); err != nil {
		return fmt.Errorf("seed streak: %w", err)
	}

	if err := seedContentPacks(ctx, pool, mathSubjectID, logger); err != nil {
		return fmt.Errorf("seed content packs: %w", err)
	}

	return nil
}

func seedContentPacks(ctx context.Context, pool *pgxpool.Pool, mathSubjectID string, logger *slog.Logger) error {
	currPath, ok := resolveContentFile(filepath.Join("content", "curricula", "g1-g6-curricula.json"))
	if !ok {
		logger.Warn("content packs not found on disk; skipping static JSON import")
		return nil
	}
	currBytes, err := os.ReadFile(currPath)
	if err != nil {
		return fmt.Errorf("read curricula pack: %w", err)
	}

	type seedSkill struct {
		Code          string            `json:"code"`
		Grade         int               `json:"grade"`
		Curricula     []string          `json:"curricula"`
		Category      string            `json:"category"`
		HeuristicCode string            `json:"heuristicCode"`
		AgeBand       string            `json:"ageBand"`
		Difficulty    int               `json:"difficulty"`
		Translations  map[string]string `json:"translations"`
	}
	var currDoc struct {
		Skills []seedSkill `json:"skills"`
	}
	if err := json.Unmarshal(currBytes, &currDoc); err != nil {
		return fmt.Errorf("parse curricula pack: %w", err)
	}
	templates := mathengine.Registry()
	knownSkills := make(map[string]bool)
	for _, sk := range currDoc.Skills {
		knownSkills[sk.Code] = true
	}
	for _, tmpl := range templates {
		if knownSkills[tmpl.SkillCode] {
			continue
		}
		currDoc.Skills = append(currDoc.Skills, seedSkill{
			Code: tmpl.SkillCode, Grade: tmpl.Grade, Curricula: []string{tmpl.CurriculumCode},
			Difficulty: tmpl.Difficulty, HeuristicCode: tmpl.HeuristicCode,
			Translations: map[string]string{"en": tmpl.SkillName.EN, "id": tmpl.SkillName.ID},
		})
		knownSkills[tmpl.SkillCode] = true
	}

	// Map curriculum code -> id
	currIDs := make(map[string]string)
	rows, err := pool.Query(ctx, `SELECT code, id::text FROM curricula WHERE subject_id = $1::uuid`, mathSubjectID)
	if err != nil {
		return fmt.Errorf("load curricula ids: %w", err)
	}
	for rows.Next() {
		var code, id string
		if scanErr := rows.Scan(&code, &id); scanErr == nil {
			currIDs[code] = id
		}
	}
	rows.Close()

	topicIDs := make(map[string]string)
	skillIDs := make(map[string]string)

	for _, sk := range currDoc.Skills {
		primaryCurr := "SINGAPORE_MOE"
		if len(sk.Curricula) > 0 {
			primaryCurr = sk.Curricula[0]
		}
		currID := currIDs[primaryCurr]
		if currID == "" {
			continue
		}
		cat := sk.Category
		switch cat {
		case "CORE", "WORD_PROBLEM", "CHALLENGING", "HOTS", "EXPLORER":
		default:
			cat = "CORE"
		}
		topicKey := fmt.Sprintf("%s:%d:%s", currID, sk.Grade, cat)
		topicID, hasTopic := topicIDs[topicKey]
		if !hasTopic {
			err := pool.QueryRow(ctx, `
				SELECT id::text FROM topics WHERE curriculum_id = $1::uuid AND grade = $2 AND category = $3 LIMIT 1
			`, currID, sk.Grade, cat).Scan(&topicID)
			if err != nil {
				err = pool.QueryRow(ctx, `
					INSERT INTO topics (curriculum_id, grade, category, translations)
					VALUES ($1::uuid, $2, $3, jsonb_build_object('en', $4::text, 'id', $5::text))
					RETURNING id::text
				`, currID, sk.Grade, cat, fmt.Sprintf("Grade %d %s", sk.Grade, cat), fmt.Sprintf("Kelas %d %s", sk.Grade, cat)).Scan(&topicID)
				if err != nil {
					return fmt.Errorf("insert topic %s: %w", topicKey, err)
				}
			}
			topicIDs[topicKey] = topicID
		}

		var skillID string
		err := pool.QueryRow(ctx, `SELECT id::text FROM skills WHERE canonical_code = $1 LIMIT 1`, sk.Code).Scan(&skillID)
		if err != nil {
			trRaw, _ := json.Marshal(sk.Translations)
			var heur, band any
			if sk.HeuristicCode != "" {
				heur = sk.HeuristicCode
			}
			if sk.AgeBand != "" {
				band = sk.AgeBand
			}
			err = pool.QueryRow(ctx, `
				INSERT INTO skills (canonical_code, topic_id, difficulty, heuristic_code, age_band, translations)
				VALUES ($1, $2::uuid, $3, $4, $5, $6::jsonb)
				RETURNING id::text
			`, sk.Code, topicID, sk.Difficulty, heur, band, trRaw).Scan(&skillID)
			if err != nil {
				return fmt.Errorf("insert skill %s: %w", sk.Code, err)
			}
		}
		skillIDs[sk.Code] = skillID
	}

	// Import 300 curated static questions
	qPath, ok := resolveContentFile(filepath.Join("content", "questions", "g1-g6-curated-pack.json"))
	if ok {
		qBytes, err := os.ReadFile(qPath)
		if err != nil {
			return fmt.Errorf("read curated questions: %w", err)
		}
		var qDoc struct {
			Questions []struct {
				Code         string            `json:"code"`
				Grade        int               `json:"grade"`
				SkillCode    string            `json:"skillCode"`
				Difficulty   int               `json:"difficulty"`
				Stem         map[string]string `json:"stem"`
				Answer       string            `json:"answer"`
				SolutionText map[string]string `json:"solutionText"`
			} `json:"questions"`
		}
		if err := json.Unmarshal(qBytes, &qDoc); err != nil {
			return fmt.Errorf("parse curated questions: %w", err)
		}
		for _, q := range qDoc.Questions {
			skID := skillIDs[q.SkillCode]
			if skID == "" {
				continue
			}
			trRaw, _ := json.Marshal(map[string]any{"stem": q.Stem, "solutionText": q.SolutionText})
			ruleRaw, _ := json.Marshal(map[string]any{"strategy": "exact_numeric", "answer": q.Answer})
			if _, err := pool.Exec(ctx, `
				INSERT INTO questions (code, subject_id, skill_id, kind, grade, difficulty, input_type, translations, grading_rule, status)
				VALUES ($1, $2::uuid, $3::uuid, 'STATIC', $4, $5, 'single_numeric', $6::jsonb, $7::jsonb, 'PUBLISHED')
				ON CONFLICT (code) DO UPDATE SET
					translations = EXCLUDED.translations,
					grading_rule = EXCLUDED.grading_rule,
					status = 'PUBLISHED',
					updated_at = NOW()
			`, q.Code, mathSubjectID, skID, q.Grade, q.Difficulty, trRaw, ruleRaw); err != nil {
				return fmt.Errorf("upsert curated question %s: %w", q.Code, err)
			}
		}
		logger.Info("imported curated questions", "count", len(qDoc.Questions))
	}

	// Import 60 procedural template descriptors
	for _, tmpl := range templates {
		skID := skillIDs[tmpl.SkillCode]
		if skID == "" {
			return fmt.Errorf("missing skill %s for template %s", tmpl.SkillCode, tmpl.Code)
		}
		trRaw, _ := json.Marshal(map[string]any{"skillName": map[string]string{"en": tmpl.SkillName.EN, "id": tmpl.SkillName.ID}})
		cfgRaw, _ := json.Marshal(map[string]any{"templateCode": tmpl.Code, "curriculumCode": tmpl.CurriculumCode})
		ruleRaw, _ := json.Marshal(map[string]any{"strategy": "procedural"})
		var heur any
		if tmpl.HeuristicCode != "" {
			heur = tmpl.HeuristicCode
		}
		if _, err := pool.Exec(ctx, `
			INSERT INTO questions (code, subject_id, skill_id, kind, grade, difficulty, input_type, heuristic_code, template_config, translations, grading_rule, status)
			VALUES ($1, $2::uuid, $3::uuid, 'PROCEDURAL', $4, $5, $6, $7, $8::jsonb, $9::jsonb, $10::jsonb, 'PUBLISHED')
			ON CONFLICT (code) DO UPDATE SET
				skill_id = EXCLUDED.skill_id,
				template_config = EXCLUDED.template_config,
				translations = EXCLUDED.translations,
				status = 'PUBLISHED',
				updated_at = NOW()
		`, "PL-TPL-"+tmpl.Code, mathSubjectID, skID, tmpl.Grade, tmpl.Difficulty, string(tmpl.InputType), heur, cfgRaw, trRaw, ruleRaw); err != nil {
			return fmt.Errorf("upsert procedural template %s: %w", tmpl.Code, err)
		}
	}
	logger.Info("imported procedural templates", "count", len(templates))

	return nil
}

package training

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AICSbench/planetlogi/apps/api/internal/platform/economy"
)

type pgPracticePayload struct {
	Seeds    []int64      `json:"seeds"`
	Answered map[int]bool `json:"answered"`
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

func (s *Service) ensureStatsLoadedPG(ctx context.Context, studentID string) (map[string]*skillStat, error) {
	if s.pool != nil && isUUID(studentID) {
		stMap := make(map[string]*skillStat)
		rows, err := s.pool.Query(ctx, `
			SELECT skill_code, recent_results, best_session_score, best_stars
			FROM training_skill_stats
			WHERE student_id = $1::uuid
		`, studentID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var code string
			var resultsRaw []byte
			var bestScore, bestStars int
			if err := rows.Scan(&code, &resultsRaw, &bestScore, &bestStars); err != nil {
				return nil, err
			}
			var win []bool
			if err := json.Unmarshal(resultsRaw, &win); err != nil {
				return nil, err
			}
			stMap[code] = &skillStat{recentWindow: win, bestSessionScore: bestScore, bestStars: bestStars}
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return stMap, nil
	}
	if stMap, ok := s.stats[studentID]; ok {
		return stMap, nil
	}
	stMap := make(map[string]*skillStat)
	s.stats[studentID] = stMap
	return stMap, nil
}

func (s *Service) recordSkillAttemptPG(ctx context.Context, studentID, attemptID, skillCode string, correct bool, sessionScore int) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if attemptID != "" {
			tag, err := tx.Exec(ctx, `INSERT INTO training_attempt_receipts (student_id, attempt_id)
				VALUES ($1::uuid, $2) ON CONFLICT DO NOTHING`, studentID, attemptID)
			if err != nil || tag.RowsAffected() == 0 {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `INSERT INTO training_skill_stats (student_id, skill_code)
			VALUES ($1::uuid, $2) ON CONFLICT DO NOTHING`, studentID, skillCode); err != nil {
			return err
		}
		var st skillStat
		var results []byte
		if err := tx.QueryRow(ctx, `SELECT recent_results, best_session_score, best_stars
			FROM training_skill_stats WHERE student_id = $1::uuid AND skill_code = $2 FOR UPDATE`,
			studentID, skillCode).Scan(&results, &st.bestSessionScore, &st.bestStars); err != nil {
			return err
		}
		if err := json.Unmarshal(results, &st.recentWindow); err != nil {
			return err
		}
		st.record(correct, sessionScore)
		return saveSkillStatPG(ctx, tx, studentID, skillCode, &st)
	})
}

func saveSkillStatPG(ctx context.Context, tx pgx.Tx, studentID, skillCode string, st *skillStat) error {
	resultsRaw, err := json.Marshal(st.recentWindow)
	if err != nil {
		return err
	}
	correct := 0
	for _, ok := range st.recentWindow {
		if ok {
			correct++
		}
	}
	pct, tier := ComputeProficiency(st.recentWindow)
	_, err = tx.Exec(ctx, `
		UPDATE training_skill_stats SET
			recent_results = $3::jsonb, recent_window_count = $4, recent_correct_count = $5,
			proficiency_pct = $6, proficiency_tier = $7, best_session_score = $8, best_stars = $9,
			updated_at = NOW()
		WHERE student_id = $1::uuid AND skill_code = $2
	`, studentID, skillCode, resultsRaw, len(st.recentWindow), correct, pct, string(tier), st.bestSessionScore, st.bestStars)
	return err
}

func (s *Service) loadSessionPG(ctx context.Context, sessionID string) (*activePractice, bool) {
	if s.pool == nil {
		return nil, false
	}
	var studentID, skillCode string
	var payloadRaw []byte
	var correct int
	err := s.pool.QueryRow(ctx, `
		SELECT student_id::text, skill_code, answered, correct_count
		FROM training_practice_sessions
		WHERE id = $1
	`, sessionID).Scan(&studentID, &skillCode, &payloadRaw, &correct)
	if err != nil {
		return nil, false
	}
	var payload pgPracticePayload
	if err := json.Unmarshal(payloadRaw, &payload); err != nil || len(payload.Seeds) == 0 {
		return nil, false
	}
	if payload.Answered == nil {
		payload.Answered = make(map[int]bool)
	}
	var tmplCode string
	for _, entry := range s.catalog {
		if entry.code == skillCode {
			tmplCode = entry.templateCode
			break
		}
	}
	return &activePractice{
		sessionID:    sessionID,
		studentID:    studentID,
		skillCode:    skillCode,
		templateCode: tmplCode,
		seeds:        payload.Seeds,
		answered:     payload.Answered,
		correct:      correct,
	}, true
}

func (s *Service) saveSessionPG(ctx context.Context, sess *activePractice, grade int) error {
	if s.pool == nil || sess == nil || !isUUID(sess.studentID) {
		return nil
	}
	raw, err := json.Marshal(pgPracticePayload{
		Seeds:    sess.seeds,
		Answered: sess.answered,
	})
	if err != nil {
		return err
	}
	var baseSeed int64
	if len(sess.seeds) > 0 {
		baseSeed = sess.seeds[0]
	}
	if grade < 1 || grade > 6 {
		grade = 1
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO training_practice_sessions
			(id, student_id, skill_code, grade, mode, seed, answered, correct_count)
		VALUES ($1, $2::uuid, $3, $4, 'PRACTICE', $5, $6::jsonb, $7)
		ON CONFLICT (id) DO UPDATE SET
			answered = EXCLUDED.answered,
			correct_count = EXCLUDED.correct_count
	`, sess.sessionID, sess.studentID, sess.skillCode, grade, baseSeed, raw, sess.correct)
	return err
}

func NewServiceWithPG(pool *pgxpool.Pool, ledger economy.Ledger) *Service {
	svc := NewService(ledger)
	svc.pool = pool
	return svc
}

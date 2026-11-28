package training

import (
	"context"
	"encoding/json"

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

func (s *Service) ensureStatsLoadedPG(ctx context.Context, studentID string) map[string]*skillStat {
	if stMap, ok := s.stats[studentID]; ok {
		return stMap
	}
	stMap := make(map[string]*skillStat)
	if s.pool != nil && isUUID(studentID) {
		rows, err := s.pool.Query(ctx, `
			SELECT skill_code, recent_results, best_session_score, best_stars
			FROM training_skill_stats
			WHERE student_id = $1::uuid
		`, studentID)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var code string
				var resultsRaw []byte
				var bestScore, bestStars int
				if scanErr := rows.Scan(&code, &resultsRaw, &bestScore, &bestStars); scanErr == nil {
					var win []bool
					_ = json.Unmarshal(resultsRaw, &win)
					stMap[code] = &skillStat{
						recentWindow:     win,
						bestSessionScore: bestScore,
						bestStars:        bestStars,
					}
				}
			}
		}
	}
	s.stats[studentID] = stMap
	return stMap
}

func (s *Service) saveSkillStatPG(ctx context.Context, studentID, skillCode string, st *skillStat) {
	if s.pool == nil || !isUUID(studentID) || st == nil {
		return
	}
	resultsRaw, err := json.Marshal(st.recentWindow)
	if err != nil {
		return
	}
	correct := 0
	for _, ok := range st.recentWindow {
		if ok {
			correct++
		}
	}
	pct, tier := ComputeProficiency(st.recentWindow)
	_, _ = s.pool.Exec(ctx, `
		INSERT INTO training_skill_stats
			(student_id, skill_code, recent_results, recent_window_count, recent_correct_count,
			 proficiency_pct, proficiency_tier, best_session_score, best_stars, updated_at)
		VALUES ($1::uuid, $2, $3::jsonb, $4, $5, $6, $7, $8, $9, NOW())
		ON CONFLICT (student_id, skill_code) DO UPDATE SET
			recent_results = EXCLUDED.recent_results,
			recent_window_count = EXCLUDED.recent_window_count,
			recent_correct_count = EXCLUDED.recent_correct_count,
			proficiency_pct = EXCLUDED.proficiency_pct,
			proficiency_tier = EXCLUDED.proficiency_tier,
			best_session_score = EXCLUDED.best_session_score,
			best_stars = EXCLUDED.best_stars,
			updated_at = NOW()
	`, studentID, skillCode, resultsRaw, len(st.recentWindow), correct, pct, string(tier), st.bestSessionScore, st.bestStars)
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

func (s *Service) saveSessionPG(ctx context.Context, sess *activePractice, grade int) {
	if s.pool == nil || sess == nil || !isUUID(sess.studentID) {
		return
	}
	raw, err := json.Marshal(pgPracticePayload{
		Seeds:    sess.seeds,
		Answered: sess.answered,
	})
	if err != nil {
		return
	}
	var baseSeed int64
	if len(sess.seeds) > 0 {
		baseSeed = sess.seeds[0]
	}
	if grade < 1 || grade > 6 {
		grade = 1
	}
	_, _ = s.pool.Exec(ctx, `
		INSERT INTO training_practice_sessions
			(id, student_id, skill_code, grade, mode, seed, answered, correct_count)
		VALUES ($1, $2::uuid, $3, $4, 'PRACTICE', $5, $6::jsonb, $7)
		ON CONFLICT (id) DO UPDATE SET
			answered = EXCLUDED.answered,
			correct_count = EXCLUDED.correct_count
	`, sess.sessionID, sess.studentID, sess.skillCode, grade, baseSeed, raw, sess.correct)
}

func NewServiceWithPG(pool *pgxpool.Pool, ledger economy.Ledger) *Service {
	svc := NewService(ledger)
	svc.pool = pool
	return svc
}

package dailyorbit

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
)

type pgSlotPayload struct {
	Index        int               `json:"index"`
	StageName    string            `json:"stageName"`
	NodeShape    string            `json:"nodeShape"`
	RewardSP     int               `json:"rewardSp"`
	TemplateCode string            `json:"templateCode"`
	Seed         int64             `json:"seed"`
	Status       SlotStatus        `json:"status"`
	EarnedSP     int               `json:"earnedSp"`
	Submitted    map[string]string `json:"submitted,omitempty"`
}

type pgSessionPayload struct {
	Slots                []pgSlotPayload `json:"slots"`
	CompletedCount       int             `json:"completedCount"`
	CompletionBonusAward bool            `json:"completionBonusAward"`
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

func (s *Service) subjectIDPG(ctx context.Context) (string, error) {
	if s.subjectID != "" {
		return s.subjectID, nil
	}
	var id string
	err := s.pool.QueryRow(ctx, `SELECT id::text FROM subjects WHERE code = 'math' LIMIT 1`).Scan(&id)
	if err != nil {
		return "", err
	}
	s.subjectID = id
	return id, nil
}

func (s *Service) loadPrefsPG(ctx context.Context, studentID string) (*studentPrefs, bool) {
	if s.pool == nil || !isUUID(studentID) {
		return nil, false
	}
	var grade int
	var tier string
	var curriculaRaw []byte
	var totalSP int64
	err := s.pool.QueryRow(ctx, `
		SELECT grade, tier, active_curricula, total_sp
		FROM daily_orbit_prefs
		WHERE student_id = $1::uuid
	`, studentID).Scan(&grade, &tier, &curriculaRaw, &totalSP)
	if errors.Is(err, pgx.ErrNoRows) || err != nil {
		return nil, false
	}
	var curricula []string
	if err := json.Unmarshal(curriculaRaw, &curricula); err != nil || len(curricula) == 0 {
		curricula = []string{"SINGAPORE_MOE", "HOTS_WHEEL"}
	}
	return &studentPrefs{
		grade:           grade,
		tier:            Tier(tier),
		activeCurricula: curricula,
		totalSP:         totalSP,
	}, true
}

func (s *Service) savePrefsPG(ctx context.Context, studentID string, p *studentPrefs) error {
	if s.pool == nil || !isUUID(studentID) || p == nil {
		return nil
	}
	curriculaRaw, err := json.Marshal(p.activeCurricula)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO daily_orbit_prefs (student_id, grade, tier, active_curricula, total_sp, updated_at)
		VALUES ($1::uuid, $2, $3, $4::jsonb, $5, NOW())
		ON CONFLICT (student_id) DO UPDATE SET
			grade = EXCLUDED.grade,
			tier = EXCLUDED.tier,
			active_curricula = EXCLUDED.active_curricula,
			total_sp = EXCLUDED.total_sp,
			updated_at = NOW()
	`, studentID, p.grade, string(p.tier), curriculaRaw, p.totalSP)
	return err
}

func (s *Service) loadSessionPG(ctx context.Context, studentID, localDate string) (*internalSession, bool) {
	if s.pool == nil || !isUUID(studentID) {
		return nil, false
	}
	subjID, err := s.subjectIDPG(ctx)
	if err != nil {
		return nil, false
	}

	var tier string
	var questionsRaw []byte
	var changeUsed, correctCount, totalSPEarned int
	err = s.pool.QueryRow(ctx, `
		SELECT tier, questions_json, change_question_used, correct_count, total_sp_earned
		FROM daily_orbit_sessions
		WHERE student_id = $1::uuid AND subject_id = $2::uuid AND local_date = $3::date
	`, studentID, subjID, localDate).Scan(&tier, &questionsRaw, &changeUsed, &correctCount, &totalSPEarned)
	if err != nil {
		return nil, false
	}

	var payload pgSessionPayload
	if err := json.Unmarshal(questionsRaw, &payload); err != nil || len(payload.Slots) != 10 {
		return nil, false
	}
	slots := make([]*internalSlot, 10)
	for i, ps := range payload.Slots {
		slots[i] = &internalSlot{
			index:        ps.Index,
			stageName:    ps.StageName,
			nodeShape:    ps.NodeShape,
			rewardSP:     ps.RewardSP,
			templateCode: ps.TemplateCode,
			seed:         ps.Seed,
			status:       ps.Status,
			earnedSP:     ps.EarnedSP,
			submitted:    ps.Submitted,
		}
	}
	return &internalSession{
		localDate:            localDate,
		tier:                 Tier(tier),
		slots:                slots,
		changeQuestionUsed:   changeUsed,
		correctCount:         correctCount,
		completedCount:       payload.CompletedCount,
		totalSPEarned:        totalSPEarned,
		completionBonusAward: payload.CompletionBonusAward,
	}, true
}

func (s *Service) saveSessionPG(ctx context.Context, studentID string, sess *internalSession) error {
	if s.pool == nil || !isUUID(studentID) || sess == nil {
		return nil
	}
	subjID, err := s.subjectIDPG(ctx)
	if err != nil {
		return err
	}

	payload := pgSessionPayload{
		Slots:                make([]pgSlotPayload, len(sess.slots)),
		CompletedCount:       sess.completedCount,
		CompletionBonusAward: sess.completionBonusAward,
	}
	for i, sl := range sess.slots {
		payload.Slots[i] = pgSlotPayload{
			Index:        sl.index,
			StageName:    sl.stageName,
			NodeShape:    sl.nodeShape,
			RewardSP:     sl.rewardSP,
			TemplateCode: sl.templateCode,
			Seed:         sl.seed,
			Status:       sl.status,
			EarnedSP:     sl.earnedSP,
			Submitted:    sl.submitted,
		}
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO daily_orbit_sessions
			(student_id, subject_id, local_date, tier, questions_json, change_question_used, correct_count, total_sp_earned, completed_at)
		VALUES ($1::uuid, $2::uuid, $3::date, $4, $5::jsonb, $6, $7, $8, CASE WHEN $9 >= 10 THEN NOW() ELSE NULL END)
		ON CONFLICT (student_id, subject_id, local_date) DO UPDATE SET
			tier = EXCLUDED.tier,
			questions_json = EXCLUDED.questions_json,
			change_question_used = EXCLUDED.change_question_used,
			correct_count = EXCLUDED.correct_count,
			total_sp_earned = EXCLUDED.total_sp_earned,
			completed_at = COALESCE(daily_orbit_sessions.completed_at, EXCLUDED.completed_at)
	`, studentID, subjID, sess.localDate, string(sess.tier), raw, sess.changeQuestionUsed, sess.correctCount, sess.totalSPEarned, sess.completedCount)
	return err
}

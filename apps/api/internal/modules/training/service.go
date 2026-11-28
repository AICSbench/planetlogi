package training

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AICSbench/planetlogi/apps/api/internal/mathengine"
	"github.com/AICSbench/planetlogi/apps/api/internal/modules/auth"
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/economy"
)

type ProficiencyTier string

const (
	TierIncomplete ProficiencyTier = "INCOMPLETE"
	TierBeginning  ProficiencyTier = "BEGINNING"
	TierCompetent  ProficiencyTier = "COMPETENT"
	TierMastery    ProficiencyTier = "MASTERY"
)

type CurriculumSummary struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
	SkillCount  int    `json:"skillCount"`
}

type SkillView struct {
	Code             string          `json:"code"`
	Name             string          `json:"name"`
	Grade            int             `json:"grade"`
	Curricula        []string        `json:"curricula"`
	Category         string          `json:"category"`
	Difficulty       int             `json:"difficulty"`
	HeuristicCode    string          `json:"heuristicCode,omitempty"`
	AgeBand          string          `json:"ageBand,omitempty"`
	TemplateCode     string          `json:"templateCode"`
	NationalAvgPct   int             `json:"nationalAvgPct"`
	AttemptsCount    int             `json:"attemptsCount"`
	CorrectCount     int             `json:"correctCount"`
	ProficiencyPct   int             `json:"proficiencyPct"`
	ProficiencyTier  ProficiencyTier `json:"proficiencyTier"`
	BestSessionScore int             `json:"bestSessionScore"`
	BestStars        int             `json:"bestStars"`
}

type HeuristicNode struct {
	Code       string      `json:"code"`
	Name       string      `json:"name"`
	SkillCount int         `json:"skillCount"`
	Skills     []SkillView `json:"skills"`
}

type HOTSQuadrant struct {
	Code       string          `json:"code"`
	Name       string          `json:"name"`
	Heuristics []HeuristicNode `json:"heuristics"`
}

type PracticeSession struct {
	SessionID   string                        `json:"sessionId"`
	SkillCode   string                        `json:"skillCode"`
	SkillName   string                        `json:"skillName"`
	Questions   []mathengine.RenderedQuestion `json:"questions"`
	Answered    int                           `json:"answered"`
	Correct     int                           `json:"correct"`
	StarsEarned int                           `json:"starsEarned"`
}

type skillCatalogEntry struct {
	code           string
	grade          int
	curricula      []string
	category       string
	difficulty     int
	heuristicCode  string
	ageBand        string
	nationalAvgPct int
	templateCode   string
	name           mathengine.LocaleText
}

type skillStat struct {
	recentWindow     []bool // max 20 recent valid attempts
	bestSessionScore int
	bestStars        int
}

type activePractice struct {
	sessionID    string
	studentID    string
	skillCode    string
	templateCode string
	seeds        []int64
	answered     map[int]bool
	correct      int
}

type Service struct {
	mu       sync.Mutex
	ledger   economy.Ledger
	pool     *pgxpool.Pool
	catalog  []skillCatalogEntry
	stats    map[string]map[string]*skillStat // studentID -> skillCode -> stat
	receipts map[[2]string]struct{}           // (studentID, attemptID), for in-memory students
	sessions map[string]*activePractice
	nextSeq  int64
}

func NewService(ledger economy.Ledger) *Service {
	return &Service{
		ledger:   ledger,
		catalog:  buildDefaultCatalog(),
		stats:    make(map[string]map[string]*skillStat),
		receipts: make(map[[2]string]struct{}),
		sessions: make(map[string]*activePractice),
		nextSeq:  1000,
	}
}

func StarsForScore(scoreOutOf10 int) int {
	switch {
	case scoreOutOf10 >= 9:
		return 3
	case scoreOutOf10 >= 6:
		return 2
	case scoreOutOf10 >= 4:
		return 1
	default:
		return 0
	}
}

func ComputeProficiency(window []bool) (pct int, tier ProficiencyTier) {
	if len(window) == 0 {
		return 0, TierIncomplete
	}
	correct := 0
	for _, ok := range window {
		if ok {
			correct++
		}
	}
	pct = (correct * 100) / len(window)
	if len(window) < 10 {
		return pct, TierIncomplete
	}
	switch {
	case pct >= 80:
		return pct, TierMastery
	case pct >= 60:
		return pct, TierCompetent
	default:
		return pct, TierBeginning
	}
}

func (s *Service) ListCurricula(locale string) []CurriculumSummary {
	defs := []struct {
		code string
		name mathengine.LocaleText
		desc mathengine.LocaleText
	}{
		{"KURIKULUM_MERDEKA", mathengine.LocaleText{EN: "Kurikulum Merdeka (Indonesia)", ID: "Kurikulum Merdeka (SD)"}, mathengine.LocaleText{EN: "National elementary curriculum Fase A, B, and C.", ID: "Kurikulum SD nasional Fase A, B, dan C."}},
		{"SINGAPORE_MOE", mathengine.LocaleText{EN: "Singapore Math MOE", ID: "Matematika Singapura (MOE)"}, mathengine.LocaleText{EN: "CPA & Bar Model mastery tracks across Grades 1–6.", ID: "Jalur penguasaan CPA & Model Batang SD 1–6."}},
		{"CAMBRIDGE_0096", mathengine.LocaleText{EN: "Cambridge Primary 0096", ID: "Cambridge Primary 0096"}, mathengine.LocaleText{EN: "International Cambridge Primary Mathematics Stages 1–6.", ID: "Kurikulum internasional Cambridge Primary Stage 1–6."}},
		{"CHALLENGING_MATHS", mathengine.LocaleText{EN: "Challenging Maths", ID: "Matematika Tantangan"}, mathengine.LocaleText{EN: "Olympiad-foundation & non-routine enrichment.", ID: "Soal pengayaan non-rutin dan dasar olimpiade."}},
		{"HOTS_WHEEL", mathengine.LocaleText{EN: "HOTS Constellation", ID: "HOTS (Strategi Heuristik)"}, mathengine.LocaleText{EN: "4-Quadrant / 9-Strategy heuristic wheel across 3 age bands.", ID: "Roda 4 Kuadran / 9 Strategi Heuristik untuk 3 rentang usia."}},
		{"EXPLORER_SERIES", mathengine.LocaleText{EN: "Real-World Explorer Series", ID: "Seri Eksplorasi Dunia Nyata"}, mathengine.LocaleText{EN: "Competition-style real-world problems (Junior & Senior).", ID: "Soal dunia nyata bergaya kompetisi (Junior & Senior)."}},
	}

	out := make([]CurriculumSummary, 0, len(defs))
	for _, d := range defs {
		count := 0
		for _, sk := range s.catalog {
			if containsStr(sk.curricula, d.code) {
				count++
			}
		}
		out = append(out, CurriculumSummary{
			Code:        d.code,
			Name:        d.name.For(locale),
			Description: d.desc.For(locale),
			SkillCount:  count,
		})
	}
	return out
}

func (s *Service) ListSkills(studentID, curriculumCode string, grade int, locale string) ([]SkillView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stMap, err := s.ensureStatsLoadedPG(context.Background(), studentID)
	if err != nil {
		return nil, err
	}

	out := make([]SkillView, 0)
	for _, entry := range s.catalog {
		if curriculumCode != "" && curriculumCode != "ALL" && !containsStr(entry.curricula, curriculumCode) {
			continue
		}
		if grade > 0 && entry.grade != grade {
			continue
		}
		out = append(out, s.toSkillView(entry, stMap, locale))
	}
	return out, nil
}

func (s *Service) GetHOTSWheel(studentID, ageBand, locale string) ([]HOTSQuadrant, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stMap, err := s.ensureStatsLoadedPG(context.Background(), studentID)
	if err != nil {
		return nil, err
	}

	type hDef struct {
		code string
		name mathengine.LocaleText
	}
	type qDef struct {
		code       string
		name       mathengine.LocaleText
		heuristics []hDef
	}
	quadrants := []qDef{
		{
			code: "REPRESENTATION", name: mathengine.LocaleText{EN: "Using a Representation", ID: "Menggunakan Representasi"},
			heuristics: []hDef{
				{"DRAW_DIAGRAMS", mathengine.LocaleText{EN: "Draw Diagrams", ID: "Menggambar Diagram"}},
				{"DRAW_MODELS", mathengine.LocaleText{EN: "Draw Models", ID: "Menggambar Model Batang"}},
				{"MAKE_A_LIST", mathengine.LocaleText{EN: "Make a List", ID: "Membuat Daftar Teratur"}},
			},
		},
		{
			code: "PROCESS", name: mathengine.LocaleText{EN: "Walking Through the Process", ID: "Menelusuri Proses"},
			heuristics: []hDef{
				{"ACT_IT_OUT", mathengine.LocaleText{EN: "Act It Out", ID: "Memperagakan Langkah"}},
				{"WORK_BACKWARDS", mathengine.LocaleText{EN: "Work Backwards", ID: "Bekerja Mundur"}},
			},
		},
		{
			code: "GUESSING", name: mathengine.LocaleText{EN: "Making a Guess", ID: "Membuat Perkiraan"},
			heuristics: []hDef{
				{"GUESS_AND_CHECK", mathengine.LocaleText{EN: "Guess & Check", ID: "Tebak & Periksa"}},
				{"LOOK_FOR_PATTERNS", mathengine.LocaleText{EN: "Look for Patterns", ID: "Mencari Pola"}},
			},
		},
		{
			code: "CHANGING_PROBLEM", name: mathengine.LocaleText{EN: "Changing the Problem", ID: "Mengubah Masalah"},
			heuristics: []hDef{
				{"MAKE_SUPPOSITION", mathengine.LocaleText{EN: "Make Supposition", ID: "Membuat Pengandaian"}},
				{"SIMPLIFY_PROBLEM", mathengine.LocaleText{EN: "Simplify the Problem", ID: "Menyederhanakan Masalah"}},
			},
		},
	}

	out := make([]HOTSQuadrant, 0, len(quadrants))
	for _, q := range quadrants {
		nodes := make([]HeuristicNode, 0, len(q.heuristics))
		for _, h := range q.heuristics {
			skills := make([]SkillView, 0)
			for _, sk := range s.catalog {
				if sk.heuristicCode != h.code {
					continue
				}
				if ageBand != "" && ageBand != "ALL" && sk.ageBand != "" && sk.ageBand != ageBand {
					continue
				}
				skills = append(skills, s.toSkillView(sk, stMap, locale))
			}
			nodes = append(nodes, HeuristicNode{
				Code:       h.code,
				Name:       h.name.For(locale),
				SkillCount: len(skills),
				Skills:     skills,
			})
		}
		out = append(out, HOTSQuadrant{
			Code:       q.code,
			Name:       q.name.For(locale),
			Heuristics: nodes,
		})
	}
	return out, nil
}

func (s *Service) StartPracticeSession(studentID, skillCode, locale string) (PracticeSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var target *skillCatalogEntry
	for i := range s.catalog {
		if s.catalog[i].code == skillCode {
			target = &s.catalog[i]
			break
		}
	}
	if target == nil {
		return PracticeSession{}, fmt.Errorf("skill %q not found", skillCode)
	}

	s.nextSeq++
	var rnd [4]byte
	_, _ = rand.Read(rnd[:])
	sessID := fmt.Sprintf("trn-%d-%s-%d", time.Now().UnixMilli(), hex.EncodeToString(rnd[:]), s.nextSeq)
	seeds := make([]int64, 10)
	questions := make([]mathengine.RenderedQuestion, 10)
	for i := 0; i < 10; i++ {
		seed := s.nextSeq*100 + int64(i+1)
		seeds[i] = seed
		prob, _ := mathengine.GenerateByCode(target.templateCode, seed, locale)
		questions[i] = prob.Question
	}

	sess := &activePractice{
		sessionID:    sessID,
		studentID:    studentID,
		skillCode:    target.code,
		templateCode: target.templateCode,
		seeds:        seeds,
		answered:     make(map[int]bool),
	}
	s.sessions[sessID] = sess
	if err := s.saveSessionPG(context.Background(), sess, target.grade); err != nil {
		return PracticeSession{}, fmt.Errorf("persist practice session: %w", err)
	}

	return PracticeSession{
		SessionID: sessID,
		SkillCode: target.code,
		SkillName: target.name.For(locale),
		Questions: questions,
	}, nil
}

func (s *Service) SubmitPracticeAnswer(ctx context.Context, studentID, sessionID string, questionIndex int, locale string, submitted map[string]string) (mathengine.GradeResult, PracticeSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sess, ok := s.sessions[sessionID]
	if !ok {
		if pgSess, loaded := s.loadSessionPG(ctx, sessionID); loaded {
			s.sessions[sessionID] = pgSess
			sess = pgSess
			ok = true
		}
	}
	if !ok || sess.studentID != studentID {
		return mathengine.GradeResult{}, PracticeSession{}, fmt.Errorf("practice session not found")
	}
	if questionIndex < 1 || questionIndex > len(sess.seeds) {
		return mathengine.GradeResult{}, PracticeSession{}, fmt.Errorf("question index must be 1..%d", len(sess.seeds))
	}

	seed := sess.seeds[questionIndex-1]
	prob, _ := mathengine.GenerateByCode(sess.templateCode, seed, locale)
	gradeRes := mathengine.Grade(prob, submitted)

	if !sess.answered[questionIndex] {
		if s.ledger != nil {
			xp := int64(1)
			if gradeRes.IsCorrect {
				xp = 2
			}
			if _, err := s.ledger.Apply(ctx, economy.MutationRequest{
				StudentID:      studentID,
				Currency:       economy.CurrencyXP,
				Delta:          xp,
				SourceType:     "GALAXY_TRAINING",
				SourceID:       fmt.Sprintf("%s:q%d", sessionID, questionIndex),
				IdempotencyKey: fmt.Sprintf("training:%s:%s:q%d", sessionID, studentID, questionIndex),
			}); err != nil {
				return mathengine.GradeResult{}, PracticeSession{}, fmt.Errorf("award training XP: %w", err)
			}
		}
		correct := sess.correct
		if gradeRes.IsCorrect {
			correct++
		}
		if err := s.recordSkillAttemptLocked(ctx, studentID, "", sess.skillCode, gradeRes.IsCorrect, correct); err != nil {
			return mathengine.GradeResult{}, PracticeSession{}, fmt.Errorf("persist skill stats: %w", err)
		}
		sess.answered[questionIndex] = true
		sess.correct = correct
		if err := s.saveSessionPG(ctx, sess, 0); err != nil {
			return mathengine.GradeResult{}, PracticeSession{}, fmt.Errorf("persist practice session: %w", err)
		}
	}

	view := PracticeSession{
		SessionID:   sess.sessionID,
		SkillCode:   sess.skillCode,
		Answered:    len(sess.answered),
		Correct:     sess.correct,
		StarsEarned: StarsForScore(sess.correct),
	}
	return gradeRes, view, nil
}

// RecordSkillAttempt adds an external attempt to Proficiency once per student and attempt ID.
func (s *Service) RecordSkillAttempt(ctx context.Context, studentID, attemptID, skillCode string, isCorrect bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.recordSkillAttemptLocked(ctx, studentID, attemptID, skillCode, isCorrect, 0)
}

func (s *Service) recordSkillAttemptLocked(ctx context.Context, studentID, attemptID, skillCode string, isCorrect bool, sessionScore int) error {
	if s.pool != nil && isUUID(studentID) {
		return s.recordSkillAttemptPG(ctx, studentID, attemptID, skillCode, isCorrect, sessionScore)
	}
	key := [2]string{studentID, attemptID}
	if _, seen := s.receipts[key]; attemptID != "" && seen {
		return nil
	}
	stMap, err := s.ensureStatsLoadedPG(ctx, studentID)
	if err != nil {
		return err
	}
	st := stMap[skillCode]
	if st == nil {
		st = &skillStat{}
		stMap[skillCode] = st
	}
	st.record(isCorrect, sessionScore)
	if attemptID != "" {
		s.receipts[key] = struct{}{}
	}
	return nil
}

func (st *skillStat) record(isCorrect bool, sessionScore int) {
	st.recentWindow = append(st.recentWindow, isCorrect)
	if len(st.recentWindow) > 20 {
		st.recentWindow = st.recentWindow[len(st.recentWindow)-20:]
	}
	if sessionScore > st.bestSessionScore {
		st.bestSessionScore = sessionScore
		st.bestStars = StarsForScore(sessionScore)
	}
}

func (s *Service) toSkillView(entry skillCatalogEntry, stMap map[string]*skillStat, locale string) SkillView {
	var attempts, correct, pct, bestScore, bestStars int
	tier := TierIncomplete
	if stMap != nil {
		if st, ok := stMap[entry.code]; ok {
			attempts = len(st.recentWindow)
			for _, v := range st.recentWindow {
				if v {
					correct++
				}
			}
			pct, tier = ComputeProficiency(st.recentWindow)
			bestScore = st.bestSessionScore
			bestStars = st.bestStars
		}
	}
	return SkillView{
		Code:             entry.code,
		Name:             entry.name.For(locale),
		Grade:            entry.grade,
		Curricula:        entry.curricula,
		Category:         entry.category,
		Difficulty:       entry.difficulty,
		HeuristicCode:    entry.heuristicCode,
		AgeBand:          entry.ageBand,
		TemplateCode:     entry.templateCode,
		NationalAvgPct:   entry.nationalAvgPct,
		AttemptsCount:    attempts,
		CorrectCount:     correct,
		ProficiencyPct:   pct,
		ProficiencyTier:  tier,
		BestSessionScore: bestScore,
		BestStars:        bestStars,
	}
}

func containsStr(slice []string, target string) bool {
	for _, s := range slice {
		if s == target {
			return true
		}
	}
	return false
}

func buildDefaultCatalog() []skillCatalogEntry {
	catalog := []skillCatalogEntry{
		{code: "num-add-within-20", grade: 1, curricula: []string{"SINGAPORE_MOE", "KURIKULUM_MERDEKA", "CAMBRIDGE_0096"}, category: "CORE", difficulty: 1, nationalAvgPct: 82, templateCode: "g1_add_within_20", name: mathengine.LocaleText{EN: "Addition within 20", ID: "Penjumlahan sampai 20"}},
		{code: "num-sub-within-20", grade: 1, curricula: []string{"KURIKULUM_MERDEKA", "SINGAPORE_MOE"}, category: "CORE", difficulty: 1, nationalAvgPct: 78, templateCode: "g1_sub_within_20", name: mathengine.LocaleText{EN: "Subtraction within 20", ID: "Pengurangan sampai 20"}},
		{code: "num-place-value-100", grade: 1, curricula: []string{"SINGAPORE_MOE", "CAMBRIDGE_0096", "KURIKULUM_MERDEKA"}, category: "CORE", difficulty: 2, nationalAvgPct: 74, templateCode: "g1_place_value_tens_ones", name: mathengine.LocaleText{EN: "Place Value (Tens & Ones)", ID: "Nilai Tempat (Puluhan & Satuan)"}},
		{code: "hots-look-for-patterns-g1", grade: 1, curricula: []string{"HOTS_WHEEL", "CHALLENGING_MATHS", "EXPLORER_SERIES"}, category: "HOTS", difficulty: 2, heuristicCode: "LOOK_FOR_PATTERNS", ageBand: "6-9", nationalAvgPct: 68, templateCode: "g1_pattern_hots", name: mathengine.LocaleText{EN: "Star Number Patterns", ID: "Pola Bilangan Bintang"}},

		{code: "num-add-1000", grade: 2, curricula: []string{"KURIKULUM_MERDEKA", "SINGAPORE_MOE", "CAMBRIDGE_0096"}, category: "CORE", difficulty: 2, nationalAvgPct: 76, templateCode: "g2_add_within_1000", name: mathengine.LocaleText{EN: "Addition within 1,000", ID: "Penjumlahan sampai 1.000"}},
		{code: "num-mul-2-3-4-5-10", grade: 2, curricula: []string{"SINGAPORE_MOE", "KURIKULUM_MERDEKA"}, category: "CORE", difficulty: 2, nationalAvgPct: 79, templateCode: "g2_mul_tables_2_5_10", name: mathengine.LocaleText{EN: "Multiplication Tables (2–5, 10)", ID: "Perkalian 2–5 & 10"}},
		{code: "wp-bar-model-compare", grade: 2, curricula: []string{"HOTS_WHEEL", "SINGAPORE_MOE", "EXPLORER_SERIES"}, category: "WORD_PROBLEM", difficulty: 3, heuristicCode: "DRAW_MODELS", ageBand: "6-9", nationalAvgPct: 66, templateCode: "g2_bar_model_compare", name: mathengine.LocaleText{EN: "Comparison Bar Models", ID: "Model Batang Perbandingan"}},
		{code: "frac-intro-compare", grade: 2, curricula: []string{"CAMBRIDGE_0096", "SINGAPORE_MOE"}, category: "CORE", difficulty: 2, nationalAvgPct: 71, templateCode: "g2_unit_fraction_mcq", name: mathengine.LocaleText{EN: "Comparing Unit Fractions", ID: "Membandingkan Pecahan Satuan"}},

		{code: "num-mul-6-7-8-9", grade: 3, curricula: []string{"SINGAPORE_MOE", "KURIKULUM_MERDEKA"}, category: "CORE", difficulty: 2, nationalAvgPct: 75, templateCode: "g3_mul_6_to_9", name: mathengine.LocaleText{EN: "Multiplication Tables (6–9)", ID: "Perkalian 6–9"}},
		{code: "num-div-remainder", grade: 3, curricula: []string{"KURIKULUM_MERDEKA", "SINGAPORE_MOE", "EXPLORER_SERIES"}, category: "CORE", difficulty: 3, nationalAvgPct: 64, templateCode: "g3_div_with_remainder", name: mathengine.LocaleText{EN: "Division with Remainders", ID: "Pembagian dengan Sisa"}},
		{code: "geo-area-perimeter-rect", grade: 3, curricula: []string{"SINGAPORE_MOE", "CAMBRIDGE_0096", "KURIKULUM_MERDEKA"}, category: "CORE", difficulty: 3, nationalAvgPct: 69, templateCode: "g3_area_perimeter_rect", name: mathengine.LocaleText{EN: "Area & Perimeter of Rectangles", ID: "Luas & Keliling Persegi Panjang"}},
		{code: "frac-add-like", grade: 3, curricula: []string{"CAMBRIDGE_0096", "SINGAPORE_MOE"}, category: "CORE", difficulty: 3, nationalAvgPct: 72, templateCode: "g3_equiv_fraction", name: mathengine.LocaleText{EN: "Adding Related Fractions", ID: "Penjumlahan Pecahan"}},

		{code: "num-factors-multiples", grade: 4, curricula: []string{"KURIKULUM_MERDEKA", "SINGAPORE_MOE", "CAMBRIDGE_0096"}, category: "CORE", difficulty: 3, nationalAvgPct: 67, templateCode: "g4_lcm_gcf", name: mathengine.LocaleText{EN: "Factors & Multiples (LCM/GCF)", ID: "Faktor & Kelipatan (KPK/FPB)"}},
		{code: "dec-add-sub", grade: 4, curricula: []string{"SINGAPORE_MOE", "KURIKULUM_MERDEKA"}, category: "CORE", difficulty: 3, nationalAvgPct: 74, templateCode: "g4_decimal_add_sub", name: mathengine.LocaleText{EN: "Adding Decimals", ID: "Penjumlahan Desimal"}},
		{code: "frac-of-set", grade: 4, curricula: []string{"SINGAPORE_MOE", "EXPLORER_SERIES"}, category: "WORD_PROBLEM", difficulty: 3, nationalAvgPct: 65, templateCode: "g4_fraction_of_set", name: mathengine.LocaleText{EN: "Fraction of a Set", ID: "Pecahan dari Sekumpulan Benda"}},
		{code: "hots-work-backwards", grade: 4, curricula: []string{"HOTS_WHEEL", "CHALLENGING_MATHS"}, category: "HOTS", difficulty: 4, heuristicCode: "WORK_BACKWARDS", ageBand: "8-11", nationalAvgPct: 58, templateCode: "g4_work_backwards_hots", name: mathengine.LocaleText{EN: "Working Backwards Strategy", ID: "Strategi Bekerja Mundur"}},

		{code: "pct-discount", grade: 5, curricula: []string{"SINGAPORE_MOE", "KURIKULUM_MERDEKA", "EXPLORER_SERIES"}, category: "WORD_PROBLEM", difficulty: 3, nationalAvgPct: 69, templateCode: "g5_percentage_discount", name: mathengine.LocaleText{EN: "Percentage & Discounts", ID: "Persentase & Diskon"}},
		{code: "ratio-two-quantities", grade: 5, curricula: []string{"SINGAPORE_MOE", "HOTS_WHEEL", "CHALLENGING_MATHS"}, category: "WORD_PROBLEM", difficulty: 4, heuristicCode: "DRAW_MODELS", ageBand: "8-11", nationalAvgPct: 61, templateCode: "g5_ratio_word_problem", name: mathengine.LocaleText{EN: "Ratio Bar-Model Problems", ID: "Soal Rasio Model Batang"}},
		{code: "geo-volume-cuboid", grade: 5, curricula: []string{"KURIKULUM_MERDEKA", "SINGAPORE_MOE", "CAMBRIDGE_0096"}, category: "CORE", difficulty: 3, nationalAvgPct: 73, templateCode: "g5_volume_cuboid", name: mathengine.LocaleText{EN: "Volume of Cuboids", ID: "Volume Balok"}},
		{code: "data-average", grade: 5, curricula: []string{"CAMBRIDGE_0096", "SINGAPORE_MOE"}, category: "CORE", difficulty: 3, nationalAvgPct: 76, templateCode: "g5_average_mean", name: mathengine.LocaleText{EN: "Average (Mean)", ID: "Rata-rata (Mean)"}},

		{code: "alg-linear-eq", grade: 6, curricula: []string{"SINGAPORE_MOE", "CAMBRIDGE_0096"}, category: "CORE", difficulty: 4, nationalAvgPct: 64, templateCode: "g6_algebra_linear", name: mathengine.LocaleText{EN: "Simple Linear Algebra", ID: "Aljabar Linear Dasar"}},
		{code: "rate-speed-dst", grade: 6, curricula: []string{"SINGAPORE_MOE", "KURIKULUM_MERDEKA", "EXPLORER_SERIES"}, category: "WORD_PROBLEM", difficulty: 4, nationalAvgPct: 60, templateCode: "g6_speed_distance_time", name: mathengine.LocaleText{EN: "Speed, Distance & Time", ID: "Kecepatan, Jarak & Waktu"}},
		{code: "geo-circles", grade: 6, curricula: []string{"KURIKULUM_MERDEKA", "SINGAPORE_MOE"}, category: "CORE", difficulty: 4, nationalAvgPct: 63, templateCode: "g6_circle_circumference", name: mathengine.LocaleText{EN: "Circumference of Circles", ID: "Keliling Lingkaran"}},
		{code: "hots-make-supposition", grade: 6, curricula: []string{"HOTS_WHEEL", "CHALLENGING_MATHS", "EXPLORER_SERIES"}, category: "CHALLENGING", difficulty: 5, heuristicCode: "MAKE_SUPPOSITION", ageBand: "10-13", nationalAvgPct: 52, templateCode: "g6_supposition_hots", name: mathengine.LocaleText{EN: "Supposition Method", ID: "Metode Pengandaian"}},
	}
	knownSkills := make(map[string]bool)
	for _, entry := range catalog {
		knownSkills[entry.code] = true
	}
	for _, tmpl := range mathengine.Registry() {
		if knownSkills[tmpl.SkillCode] {
			continue
		}
		entry := skillCatalogEntry{
			code: tmpl.SkillCode, grade: tmpl.Grade, curricula: []string{tmpl.CurriculumCode},
			category: "CORE", difficulty: tmpl.Difficulty, templateCode: tmpl.Code,
			name: tmpl.SkillName, heuristicCode: tmpl.HeuristicCode,
		}
		switch tmpl.CurriculumCode {
		case "HOTS_WHEEL":
			entry.category = "HOTS"
			entry.ageBand = "8-11"
			if tmpl.Grade >= 5 {
				entry.ageBand = "10-13"
			} else if tmpl.Grade <= 2 {
				entry.ageBand = "6-9"
			}
		case "CHALLENGING_MATHS":
			entry.category = "CHALLENGING"
		case "EXPLORER_SERIES":
			entry.category = "EXPLORER"
		}
		catalog = append(catalog, entry)
		knownSkills[entry.code] = true
	}
	return catalog
}

type Handler struct {
	svc     *Service
	authSvc *auth.Service
}

func NewHandler(svc *Service, authSvc *auth.Service) *Handler {
	return &Handler{svc: svc, authSvc: authSvc}
}

type AuthQuery struct {
	CookieAccess  string `cookie:"pl_access"`
	Authorization string `header:"Authorization"`
	Locale        string `query:"locale" default:"en"`
}

func (h *Handler) resolveStudent(ctx context.Context, in AuthQuery, requireAuth bool) (string, string, error) {
	locale := in.Locale
	if locale != "id" {
		locale = "en"
	}
	tok := in.CookieAccess
	if tok == "" && len(in.Authorization) > 7 {
		tok = in.Authorization[7:]
	}
	if h.authSvc != nil {
		if tok != "" {
			p, err := h.authSvc.ResolvePrincipal(ctx, tok)
			if err != nil {
				return "", "", auth.ErrInvalidCredentials
			}
			return p.ID, locale, nil
		}
		if requireAuth {
			return "", "", auth.ErrInvalidCredentials
		}
	}
	return "demo-pilot1", locale, nil
}

func (h *Handler) RegisterRoutes(_ chi.Router, api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "training-curricula",
		Method:      http.MethodGet,
		Path:        "/api/v1/training/curricula",
		Summary:     "List all 6 Galaxy Training curriculum tracks",
		Tags:        []string{"Training"},
	}, func(_ context.Context, in *struct {
		Locale string `query:"locale" default:"en"`
	}) (*struct {
		Body struct {
			Curricula []CurriculumSummary `json:"curricula"`
		}
	}, error) {
		out := &struct {
			Body struct {
				Curricula []CurriculumSummary `json:"curricula"`
			}
		}{}
		out.Body.Curricula = h.svc.ListCurricula(in.Locale)
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "training-skills",
		Method:      http.MethodGet,
		Path:        "/api/v1/training/skills",
		Summary:     "List skills for a curriculum and grade with High Score stars and Proficiency %",
		Tags:        []string{"Training"},
	}, func(ctx context.Context, in *struct {
		AuthQuery
		Curriculum string `query:"curriculum" default:"SINGAPORE_MOE"`
		Grade      int    `query:"grade" default:"1"`
	}) (*struct {
		Body struct {
			Skills []SkillView `json:"skills"`
		}
	}, error) {
		studentID, locale, err := h.resolveStudent(ctx, in.AuthQuery, false)
		if err != nil {
			return nil, huma.Error401Unauthorized("unauthenticated")
		}
		out := &struct {
			Body struct {
				Skills []SkillView `json:"skills"`
			}
		}{}
		out.Body.Skills, err = h.svc.ListSkills(studentID, in.Curriculum, in.Grade, locale)
		if err != nil {
			return nil, huma.Error503ServiceUnavailable("training statistics unavailable")
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "training-hots-wheel",
		Method:      http.MethodGet,
		Path:        "/api/v1/training/hots",
		Summary:     "Get the 4-Quadrant / 9-Heuristic HOTS Constellation Wheel",
		Tags:        []string{"Training"},
	}, func(ctx context.Context, in *struct {
		AuthQuery
		AgeBand string `query:"ageBand" default:"ALL"`
	}) (*struct {
		Body struct {
			Quadrants []HOTSQuadrant `json:"quadrants"`
		}
	}, error) {
		studentID, locale, err := h.resolveStudent(ctx, in.AuthQuery, false)
		if err != nil {
			return nil, huma.Error401Unauthorized("unauthenticated")
		}
		out := &struct {
			Body struct {
				Quadrants []HOTSQuadrant `json:"quadrants"`
			}
		}{}
		out.Body.Quadrants, err = h.svc.GetHOTSWheel(studentID, in.AgeBand, locale)
		if err != nil {
			return nil, huma.Error503ServiceUnavailable("training statistics unavailable")
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "training-start-session",
		Method:      http.MethodPost,
		Path:        "/api/v1/training/sessions",
		Summary:     "Start a 10-question Galaxy Training practice session for a skill",
		Tags:        []string{"Training"},
		Security:    []map[string][]string{{"accessCookie": {}}, {"bearerAuth": {}}},
	}, func(ctx context.Context, in *struct {
		AuthQuery
		Body struct {
			SkillCode string `json:"skillCode" minLength:"1"`
		}
	}) (*struct{ Body PracticeSession }, error) {
		studentID, locale, err := h.resolveStudent(ctx, in.AuthQuery, true)
		if err != nil {
			return nil, huma.Error401Unauthorized("authentication required")
		}
		sess, err := h.svc.StartPracticeSession(studentID, in.Body.SkillCode, locale)
		if err != nil {
			return nil, huma.Error404NotFound(err.Error())
		}
		return &struct{ Body PracticeSession }{Body: sess}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "training-submit-answer",
		Method:      http.MethodPost,
		Path:        "/api/v1/training/sessions/submit",
		Summary:     "Grade a Galaxy Training practice answer and update Proficiency % and stars",
		Tags:        []string{"Training"},
		Security:    []map[string][]string{{"accessCookie": {}}, {"bearerAuth": {}}},
	}, func(ctx context.Context, in *struct {
		AuthQuery
		Body struct {
			SessionID     string            `json:"sessionId" minLength:"1"`
			QuestionIndex int               `json:"questionIndex" minimum:"1" maximum:"10"`
			Answers       map[string]string `json:"answers"`
		}
	}) (*struct {
		Body struct {
			Result  mathengine.GradeResult `json:"result"`
			Session PracticeSession        `json:"session"`
		}
	}, error) {
		studentID, locale, err := h.resolveStudent(ctx, in.AuthQuery, true)
		if err != nil {
			return nil, huma.Error401Unauthorized("authentication required")
		}
		res, sess, err := h.svc.SubmitPracticeAnswer(ctx, studentID, in.Body.SessionID, in.Body.QuestionIndex, locale, in.Body.Answers)
		if err != nil {
			return nil, huma.Error400BadRequest(err.Error())
		}
		out := &struct {
			Body struct {
				Result  mathengine.GradeResult `json:"result"`
				Session PracticeSession        `json:"session"`
			}
		}{}
		out.Body.Result = res
		out.Body.Session = sess
		return out, nil
	})
}

package training

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AICSbench/planetlogi/apps/api/internal/mathengine"
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/economy"
)

func TestTrainingCurriculaProficiencyAndStars(t *testing.T) {
	ledger := economy.NewMemoryLedger(economy.Wallet{StudentID: "stu-1", PilotLevel: 1})
	svc := NewService(ledger)

	if cur := svc.ListCurricula("id"); len(cur) != 6 {
		t.Fatalf("expected 6 curricula, got %d", len(cur))
	}
	if wheel, err := svc.GetHOTSWheel("stu-1", "ALL", "en"); err != nil || len(wheel) != 4 {
		t.Fatalf("expected 4 HOTS quadrants, got %d", len(wheel))
	}

	sess, err := svc.StartPracticeSession("stu-1", "num-add-within-20", "en")
	if err != nil || len(sess.Questions) != 10 {
		t.Fatalf("expected 10-question practice session, err=%v len=%d", err, len(sess.Questions))
	}

	// Answer 9/10 correctly -> 3 stars + Mastery (90%)
	ctx := context.Background()
	for i, q := range sess.Questions {
		prob, _ := mathengine.GenerateByCode(q.TemplateCode, q.Seed, "en")
		ans := prob.Solution.CorrectAnswers
		if i == 9 {
			ans = map[string]string{"answer": "-999"}
		}
		if _, _, err := svc.SubmitPracticeAnswer(ctx, "stu-1", sess.SessionID, i+1, "en", ans); err != nil {
			t.Fatalf("submit q%d failed: %v", i+1, err)
		}
	}

	skills, err := svc.ListSkills("stu-1", "SINGAPORE_MOE", 1, "en")
	if err != nil {
		t.Fatal(err)
	}
	var found *SkillView
	for i := range skills {
		if skills[i].Code == "num-add-within-20" {
			found = &skills[i]
			break
		}
	}
	if found == nil {
		t.Fatal("expected skill in Grade 1 Singapore MOE list")
	}
	if found.ProficiencyPct != 90 || found.ProficiencyTier != TierMastery || found.BestStars != 3 {
		t.Fatalf("expected 90%% Mastery 3 stars, got %+v", found)
	}
}

func TestRemappedSkillsArePlayableInTraining(t *testing.T) {
	svc := NewService(nil)
	for code, template := range map[string]string{
		"num-add-4digit":      "g3_add_sub_4digit",
		"geo-triangle-angles": "g4_angles_triangle_mcq",
		"geo-triangle-area":   "g5_triangle_area",
		"frac-divide":         "g6_fraction_divide",
	} {
		for _, locale := range []string{"en", "id"} {
			practice, err := svc.StartPracticeSession("stu-1", code, locale)
			if err != nil {
				t.Fatal(err)
			}
			if len(practice.Questions) == 0 || practice.SkillName == "" {
				t.Fatalf("skill %s (%s): missing questions or translation", code, locale)
			}
			q := practice.Questions[0]
			if q.TemplateCode != template || q.SkillCode != code {
				t.Errorf("skill %s (%s): got template %s with skill %s, want %s", code, locale, q.TemplateCode, q.SkillCode, template)
			}
		}
	}
}

func TestTrainingPGConcurrentAnswers(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping PostgreSQL concurrency test")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	var studentID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO student_profiles (parent_account_id, login_id, password_hash, first_name, masked_display_name, grade, crew_code)
		SELECT parent_account_id, 'test-' || gen_random_uuid(), password_hash, 'Test Pilot', 'Test Pilot', 1,
			left(replace(gen_random_uuid()::text, '-', ''), 9)
		FROM student_profiles WHERE login_id = 'pilot1'
		RETURNING id::text
	`).Scan(&studentID); err != nil {
		t.Fatalf("create isolated test student (seed pilot1 first): %v", err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(ctx, `DELETE FROM student_profiles WHERE id = $1::uuid`, studentID); err != nil {
			t.Error(err)
		}
	})

	newSvc := func() *Service { return NewServiceWithPG(pool, economy.NewPGLedger(pool)) }
	sess, err := newSvc().StartPracticeSession(studentID, "num-add-within-20", "en")
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		q := sess.Questions[i]
		prob, _ := mathengine.GenerateByCode(q.TemplateCode, q.Seed, "en")
		svc := newSvc()
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, _, errs[idx] = svc.SubmitPracticeAnswer(ctx, studentID, sess.SessionID, idx+1, "en", prob.Solution.CorrectAnswers)
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("concurrent answer %d failed: %v", i+1, err)
		}
	}

	var correct int
	if err := pool.QueryRow(ctx, `SELECT correct_count FROM training_practice_sessions WHERE id = $1`, sess.SessionID).Scan(&correct); err != nil || correct != 2 {
		t.Fatalf("expected both answers saved, got correct=%d: %v", correct, err)
	}
	var answered map[string]bool
	if err := pool.QueryRow(ctx, `SELECT answered->'answered' FROM training_practice_sessions WHERE id = $1`, sess.SessionID).Scan(&answered); err != nil || !answered["1"] || !answered["2"] {
		t.Fatalf("expected both answered slots persisted, got %v: %v", answered, err)
	}
	var window, correctStats int
	if err := pool.QueryRow(ctx, `SELECT recent_window_count, recent_correct_count FROM training_skill_stats
		WHERE student_id = $1::uuid AND skill_code = 'num-add-within-20'`, studentID).Scan(&window, &correctStats); err != nil || window != 2 || correctStats != 2 {
		t.Fatalf("expected both attempts in proficiency, got %d/%d: %v", correctStats, window, err)
	}
	var xp int
	if err := pool.QueryRow(ctx, `SELECT pilot_xp FROM student_wallets WHERE student_id = $1::uuid`, studentID).Scan(&xp); err != nil || xp != 4 {
		t.Fatalf("expected exactly 4 XP, got %d: %v", xp, err)
	}
}

func TestIsParentLinkedDemoFallback(t *testing.T) {
	svc := NewService(nil)
	ok, err := svc.IsParentLinked(context.Background(), "demo-parent", "demo-pilot1")
	if err != nil || !ok {
		t.Fatalf("expected demo parent link, got ok=%v err=%v", ok, err)
	}
	ok, err = svc.IsParentLinked(context.Background(), "other-parent", "demo-pilot1")
	if err != nil || ok {
		t.Fatalf("expected unrelated parent to be rejected, got ok=%v err=%v", ok, err)
	}
}

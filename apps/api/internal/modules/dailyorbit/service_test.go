package dailyorbit

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AICSbench/planetlogi/apps/api/internal/mathengine"
	"github.com/AICSbench/planetlogi/apps/api/internal/modules/streak"
	"github.com/AICSbench/planetlogi/apps/api/internal/modules/training"
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/economy"
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/timewindow"
)

func TestDailyOrbitScoringLimitsAndLocks(t *testing.T) {
	ledger := economy.NewMemoryLedger(economy.Wallet{StudentID: "stu-1", AstroCoins: 10, PilotLevel: 1})
	streakSvc := streak.NewService()
	svc := NewService(ledger, streakSvc)

	loc := timewindow.LoadLocationSafe("Asia/Jakarta")
	svc.nowClock = func() time.Time { return time.Date(2026, 11, 26, 10, 0, 0, 0, loc) }

	today := getToday(t, svc, "stu-1")
	if len(today.Slots) != 10 {
		t.Fatalf("expected 10 Daily Orbit slots, got %d", len(today.Slots))
	}
	if today.Slots[0].WorkedSolution != nil {
		t.Fatalf("pending slot MUST NOT expose WorkedSolution")
	}

	// Change Question works 3 times and fails on the 4th
	for i := 0; i < 3; i++ {
		if _, _, err := svc.ChangeQuestion("stu-1", 1, "Asia/Jakarta", "en", 1, "easier"); err != nil {
			t.Fatalf("change question %d failed: %v", i+1, err)
		}
	}
	if _, _, err := svc.ChangeQuestion("stu-1", 1, "Asia/Jakarta", "en", 1, "harder"); err != ErrChangeLimitReached {
		t.Fatalf("expected ErrChangeLimitReached on 4th change, got %v", err)
	}

	// Answer all 10 slots correctly -> earns 17 SP + 5 AC completion bonus + advances streak!
	ctx := context.Background()
	refreshed := getToday(t, svc, "stu-1")
	for _, sl := range refreshed.Slots {
		if sl.Question.Seed <= 0 || sl.Question.Seed > 9007199254740991 {
			t.Fatalf("slot %d seed %d outside JS Number.MAX_SAFE_INTEGER", sl.Index, sl.Question.Seed)
		}
		prob, _ := mathengine.GenerateByCode(sl.Question.TemplateCode, sl.Question.Seed, "en")
		resSlot, _, err := svc.SubmitAnswer(ctx, "stu-1", 1, "Asia/Jakarta", "en", sl.Index, prob.Solution.CorrectAnswers, sl.Question.TemplateCode, sl.Question.Seed)
		if err != nil {
			t.Fatalf("slot %d submit error: %v", sl.Index, err)
		}
		if resSlot.Status != SlotCorrect {
			t.Fatalf("expected slot %d CORRECT, got %s", sl.Index, resSlot.Status)
		}
	}

	// Duplicate submission on locked slot 1 MUST return ErrAlreadyAnswered
	if _, _, err := svc.SubmitAnswer(ctx, "stu-1", 1, "Asia/Jakarta", "en", 1, map[string]string{"answer": "0"}, "", 0); err != ErrAlreadyAnswered {
		t.Fatalf("expected ErrAlreadyAnswered, got %v", err)
	}

	final := getToday(t, svc, "stu-1")
	if final.TotalSPEarned != 17 || !final.CompletionBonusAward {
		t.Fatalf("expected 17 SP & completion bonus, got SP=%d bonus=%v", final.TotalSPEarned, final.CompletionBonusAward)
	}
	if st, err := streakSvc.GetStatus("stu-1", "Asia/Jakarta"); err != nil || st.CurrentStreak != 4 {
		t.Fatalf("expected streak to advance from 3 -> 4, got %d", st.CurrentStreak)
	}

	// Intermediate tier (15 SP) should now be unlockable; Master (100 SP) still locked
	if _, err := svc.SetTier("stu-1", 1, "Asia/Jakarta", "en", TierIntermediate); err != nil {
		t.Fatalf("expected Intermediate tier to unlock: %v", err)
	}
	if _, err := svc.SetTier("stu-1", 1, "Asia/Jakarta", "en", TierMaster); err != ErrTierLocked {
		t.Fatalf("expected ErrTierLocked for Master, got %v", err)
	}
}

type mockAttemptRecorder struct {
	calls []string
	err   error
}

func (m *mockAttemptRecorder) RecordSkillAttempt(_ context.Context, studentID, _, skillCode string, isCorrect bool) error {
	if m.err != nil {
		return m.err
	}
	if isCorrect {
		m.calls = append(m.calls, studentID+":"+skillCode+":ok")
	} else {
		m.calls = append(m.calls, studentID+":"+skillCode+":err")
	}
	return nil
}

func TestDailyOrbitRecordsSkillAttemptsForReports(t *testing.T) {
	svc := NewService(economy.NewMemoryLedger(), streak.NewService())
	loc := timewindow.LoadLocationSafe("Asia/Jakarta")
	svc.nowClock = func() time.Time { return time.Date(2026, 11, 26, 10, 0, 0, 0, loc) }

	rec := &mockAttemptRecorder{}
	svc.SetAttemptRecorder(rec)

	today := getToday(t, svc, "stu-2")
	sl := today.Slots[0]
	prob, _ := mathengine.GenerateByCode(sl.Question.TemplateCode, sl.Question.Seed, "en")
	if _, _, err := svc.SubmitAnswer(context.Background(), "stu-2", 1, "Asia/Jakarta", "en", sl.Index, prob.Solution.CorrectAnswers, sl.Question.TemplateCode, sl.Question.Seed); err != nil {
		t.Fatalf("submit error: %v", err)
	}
	if len(rec.calls) != 1 || rec.calls[0] != "stu-2:"+sl.Question.SkillCode+":ok" {
		t.Fatalf("expected attempt recorder to receive stu-2:%s:ok, got %v", sl.Question.SkillCode, rec.calls)
	}

	// Verify persistence/recorder error is propagated instead of silently ignored
	rec.err = errors.New("db write failed")
	sl2 := today.Slots[1]
	prob2, _ := mathengine.GenerateByCode(sl2.Question.TemplateCode, sl2.Question.Seed, "en")
	if _, _, err := svc.SubmitAnswer(context.Background(), "stu-2", 1, "Asia/Jakarta", "en", sl2.Index, prob2.Solution.CorrectAnswers, sl2.Question.TemplateCode, sl2.Question.Seed); err == nil {
		t.Fatalf("expected SubmitAnswer to propagate attempt recorder error")
	}
}

type lostAttemptAck struct{ SkillAttemptRecorder }

var errLostAttemptAck = errors.New("lost acknowledgement after recording skill attempt")

func (r lostAttemptAck) RecordSkillAttempt(ctx context.Context, studentID, attemptID, skillCode string, correct bool) error {
	if err := r.SkillAttemptRecorder.RecordSkillAttempt(ctx, studentID, attemptID, skillCode, correct); err != nil {
		return err
	}
	return errLostAttemptAck
}

func TestDailyOrbitPGPersistenceRestart(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping PostgreSQL restart integration test")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
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

	loc := timewindow.LoadLocationSafe("Asia/Jakarta")
	fixedNow := time.Date(2099, 1, 2, 10, 0, 0, 0, loc)

	svc1 := NewServiceWithPG(pool, economy.NewPGLedger(pool), streak.NewServiceWithPG(pool))
	svc1.nowClock = func() time.Time { return fixedNow }
	recorder := &mockAttemptRecorder{err: errors.New("recorder unavailable")}
	today := getToday(t, svc1, studentID)
	for _, earlier := range today.Slots[:4] {
		p, _ := mathengine.GenerateByCode(earlier.Question.TemplateCode, earlier.Question.Seed, "en")
		if _, _, err := svc1.SubmitAnswer(ctx, studentID, 1, "Asia/Jakarta", "en", earlier.Index, p.Solution.CorrectAnswers, earlier.Question.TemplateCode, earlier.Question.Seed); err != nil {
			t.Fatal(err)
		}
	}
	svc1.SetAttemptRecorder(recorder)
	sl := today.Slots[4]
	prob, _ := mathengine.GenerateByCode(sl.Question.TemplateCode, sl.Question.Seed, "en")
	submit := func(svc *Service, answers map[string]string) (SlotView, SessionView, error) {
		return svc.SubmitAnswer(ctx, studentID, 1, "Asia/Jakarta", "en", sl.Index, answers, sl.Question.TemplateCode, sl.Question.Seed)
	}
	constraint := "test_streak_" + studentID
	name := pgx.Identifier{constraint}.Sanitize()
	if _, err := pool.Exec(ctx, `ALTER TABLE streak_calendar_days ADD CONSTRAINT `+name+
		` CHECK (student_id <> '`+studentID+`'::uuid)`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(ctx, `ALTER TABLE streak_calendar_days DROP CONSTRAINT IF EXISTS `+name); err != nil {
			t.Error(err)
		}
	})
	var pgErr *pgconn.PgError
	if _, _, err := submit(svc1, prob.Solution.CorrectAnswers); !errors.As(err, &pgErr) || pgErr.ConstraintName != constraint {
		t.Fatalf("expected streak write failure, got %v", err)
	}

	if _, err := pool.Exec(ctx, `ALTER TABLE streak_calendar_days DROP CONSTRAINT `+name); err != nil {
		t.Fatal(err)
	}
	streaks := streak.NewServiceWithPG(pool)
	svc2 := NewServiceWithPG(pool, economy.NewPGLedger(pool), streaks)
	svc2.nowClock = func() time.Time { return fixedNow }
	if _, _, err := svc2.SkipSlot(ctx, studentID, 1, "Asia/Jakarta", "en", sl.Index); !errors.Is(err, ErrAlreadyAnswered) {
		t.Fatalf("skip after a streak write failure must not replace the accepted answer, got %v", err)
	}
	svc2.SetAttemptRecorder(recorder)
	if _, _, err := submit(svc2, nil); !errors.Is(err, recorder.err) {
		t.Fatalf("expected proficiency failure after recovering streak, got %v", err)
	}
	if st, err := streaks.GetStatus(studentID, "Asia/Jakarta"); err != nil || st.TodayCorrectCount != 5 || st.CurrentStreak != 4 {
		t.Fatalf("expected streak to reach milestone once, got %+v: %v", st, err)
	}
	if days, err := streaks.GetCalendar(studentID, "2099-01"); err != nil || len(days) != 1 || days[0].Status != streak.DayCompleted {
		t.Fatalf("expected a completed calendar day after retry, got %v: %v", days, err)
	}

	svc2.SetAttemptRecorder(lostAttemptAck{training.NewServiceWithPG(pool, nil)})
	if _, _, err := submit(svc2, nil); !errors.Is(err, errLostAttemptAck) {
		t.Fatalf("retry after restart should reach recorder and lose its acknowledgement, got %v", err)
	}

	svc3 := NewServiceWithPG(pool, economy.NewPGLedger(pool), streak.NewServiceWithPG(pool))
	svc3.nowClock = func() time.Time { return fixedNow }
	stats := training.NewServiceWithPG(pool, nil)
	svc3.SetAttemptRecorder(stats)
	got, reloaded, err := submit(svc3, nil)
	if err != nil || got.Status != SlotCorrect || reloaded.CompletedCount != 5 || reloaded.TotalSPEarned != 5 {
		t.Fatalf("expected original correct answer and five completions after retry, got %s, completed=%d SP=%d: %v", got.Status, reloaded.CompletedCount, reloaded.TotalSPEarned, err)
	}
	if _, _, err := submit(svc3, nil); !errors.Is(err, ErrAlreadyAnswered) {
		t.Fatalf("expected completed submission to remain locked, got %v", err)
	}
	var sp, xp int
	if err := pool.QueryRow(ctx, `SELECT total_sp, pilot_xp FROM student_wallets WHERE student_id = $1::uuid`, studentID).Scan(&sp, &xp); err != nil || sp != 5 || xp != 15 {
		t.Fatalf("expected exactly 5 SP + 15 XP, got SP=%d XP=%d: %v", sp, xp, err)
	}
	skills, err := stats.ListSkills(studentID, "ALL", 1, "en")
	if err != nil {
		t.Fatal(err)
	}
	for _, skill := range skills {
		if skill.Code == prob.Question.SkillCode {
			if skill.AttemptsCount != 1 || skill.CorrectCount != 1 {
				t.Fatalf("expected exactly one correct proficiency attempt, got %d/%d", skill.CorrectCount, skill.AttemptsCount)
			}
			return
		}
	}
	t.Fatalf("skill %s not found in proficiency", prob.Question.SkillCode)
}

func getToday(t *testing.T, svc *Service, studentID string) SessionView {
	t.Helper()
	view, err := svc.GetToday(studentID, 1, "Asia/Jakarta", "en")
	if err != nil {
		t.Fatal(err)
	}
	return view
}

type bonusLedger struct {
	economy.Ledger
	err   error
	coins int64
}

func (l *bonusLedger) Apply(ctx context.Context, req economy.MutationRequest) (economy.MutationResult, error) {
	if l.err != nil {
		return economy.MutationResult{}, l.err
	}
	result, err := l.Ledger.Apply(ctx, req)
	l.coins = result.Wallet.AstroCoins
	return result, err
}

func TestSkipFinalSlotRetriesFailedBonus(t *testing.T) {
	failure := errors.New("ledger unavailable")
	ledger := &bonusLedger{Ledger: economy.NewMemoryLedger(), err: failure}
	svc := NewService(ledger, nil)
	svc.nowClock = func() time.Time { return time.Date(2026, 11, 26, 3, 0, 0, 0, time.UTC) }
	for i := 1; i < 10; i++ {
		if _, _, err := svc.SkipSlot(t.Context(), "student", 1, "Asia/Jakarta", "en", i); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := svc.SkipSlot(t.Context(), "student", 1, "Asia/Jakarta", "en", 10); !errors.Is(err, failure) {
		t.Fatalf("expected ledger error, got %v", err)
	}
	view := getToday(t, svc, "student")
	if view.Slots[9].Status != SlotPending || view.CompletedCount != 9 || view.CompletionBonusAward {
		t.Fatalf("session changed after failed bonus: %+v", view)
	}
	ledger.err = nil
	slot, view, err := svc.SkipSlot(t.Context(), "student", 1, "Asia/Jakarta", "en", 10)
	if err != nil || slot.Status != SlotSkipped || !view.CompletionBonusAward || ledger.coins != 5 {
		t.Fatalf("retry: slot=%s bonus=%v coins=%d err=%v", slot.Status, view.CompletionBonusAward, ledger.coins, err)
	}
}

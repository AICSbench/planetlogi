package state_test

import (
	"context"
	"encoding/json"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AICSbench/planetlogi/apps/api/internal/mathengine"
	"github.com/AICSbench/planetlogi/apps/api/internal/modules/academy"
	"github.com/AICSbench/planetlogi/apps/api/internal/modules/assignments"
	"github.com/AICSbench/planetlogi/apps/api/internal/modules/astroquiz"
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/state"
)

// TestPhase2StateSurvivesRestart simulates a process restart by constructing
// fresh services against the same PostgreSQL store.
func TestPhase2StateSurvivesRestart(t *testing.T) {
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

	var studentID, parentID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO student_profiles (parent_account_id, login_id, password_hash, first_name, masked_display_name, grade, crew_code)
		SELECT parent_account_id, 'state-test-' || gen_random_uuid(), password_hash, 'State Pilot', 'State Pilot', 3,
			left(replace(gen_random_uuid()::text, '-', ''), 9)
		FROM student_profiles WHERE login_id = 'pilot1'
		RETURNING id::text, parent_account_id::text
	`).Scan(&studentID, &parentID); err != nil {
		t.Fatalf("create isolated test student (seed pilot1 first): %v", err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), `DELETE FROM student_profiles WHERE id = $1::uuid`, studentID); err != nil {
			t.Error(err)
		}
	})

	store := state.NewPGStore(pool)

	// First "process": write one record for each module.
	academySvc := academy.NewServiceWithStore(store, nil)
	if _, err := academySvc.SaveDraft(ctx, studentID, "g1-wn-l1", "en", 1, map[string]string{"answer": "9"}); err != nil {
		t.Fatalf("save academy draft: %v", err)
	}

	quizSvc := astroquiz.NewServiceWithStore(store, nil)
	if _, err := quizSvc.SavePaper(ctx, studentID, "G1-ESS-01", "en", 42, map[int]map[string]string{1: {"answer": "7"}}); err != nil {
		t.Fatalf("save quiz draft: %v", err)
	}
	submitted, err := quizSvc.SubmitPaper(ctx, studentID, "G1-ESS-01", "en", 42, map[int]map[string]string{1: {"answer": "7"}})
	if err != nil {
		t.Fatalf("submit quiz paper: %v", err)
	}

	assignSvc := assignments.NewServiceWithDeps(pool, store, nil)
	created, err := assignSvc.CreateParentCustomAssignment(ctx, parentID, "State Parent", studentID, "Persistence Check", "2026-12-10", []string{"g1_add_within_20"}, "")
	if err != nil {
		t.Fatalf("create assignment: %v", err)
	}
	detail, err := assignSvc.GetDetail(ctx, studentID, created.ID, "en")
	if err != nil {
		t.Fatal(err)
	}
	q := detail.Questions[0]
	prob, ok := mathengine.GenerateByCode(q.Question.TemplateCode, q.Question.Seed, "en")
	if !ok {
		t.Fatal("question template not found")
	}
	if _, _, err := assignSvc.SubmitQuestion(ctx, studentID, created.ID, q.Index, prob.Solution.CorrectAnswers, "en"); err != nil {
		t.Fatalf("submit assignment answer: %v", err)
	}

	// Second "process": fresh services must reload every record.
	academyReloaded := academy.NewServiceWithStore(store, nil)
	lesson, err := academyReloaded.GetLesson(ctx, studentID, "g1-wn-l1", "en")
	if err != nil || lesson.DoAndLearn[0].DraftAnswer["answer"] != "9" {
		t.Fatalf("academy draft lost after restart: err=%v draft=%v", err, lesson.DoAndLearn[0].DraftAnswer)
	}

	quizReloaded := astroquiz.NewServiceWithStore(store, nil)
	paper, err := quizReloaded.OpenPaper(ctx, studentID, "G1-ESS-01", "en")
	if err != nil {
		t.Fatal(err)
	}
	// ElapsedSec is server-measured (the client's 42 is ignored); it must survive the restart.
	if paper.Status != "SUBMITTED" || paper.ElapsedSec != submitted.ElapsedSec || paper.Questions[0].DraftAnswer["answer"] != "7" {
		t.Fatalf("quiz attempt lost after restart: %+v", paper)
	}
	_, claimed, err := quizReloaded.ListPapers(ctx, studentID, 0, "ALL", "en")
	if err != nil || !claimed {
		t.Fatalf("weekly Supply Pod claim lost after restart: claimed=%v err=%v", claimed, err)
	}

	assignReloaded := assignments.NewServiceWithDeps(pool, store, nil)
	list, err := assignReloaded.ListForStudent(ctx, studentID, "ALL", "ALL", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != created.ID || list[0].Status != "COMPLETE" {
		t.Fatalf("assignment lost after restart: %+v", list)
	}
}

func TestSaveMergedPreservesConcurrentWrites(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping PostgreSQL merge integration test")
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
		SELECT parent_account_id, 'merge-test-' || gen_random_uuid(), password_hash, 'Merge Pilot', 'Merge Pilot', 1,
			left(replace(gen_random_uuid()::text, '-', ''), 9)
		FROM student_profiles WHERE login_id = 'pilot1'
		RETURNING id::text
	`).Scan(&studentID); err != nil {
		t.Fatalf("create isolated test student (seed pilot1 first): %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM student_profiles WHERE id = $1::uuid`, studentID)
	})

	store := state.NewPGStore(pool)
	merge := func(key string) func([]byte) (any, error) {
		return func(current []byte) (any, error) {
			out := map[string]string{}
			if len(current) > 0 {
				if err := json.Unmarshal(current, &out); err != nil {
					return nil, err
				}
			}
			out[key] = key
			return out, nil
		}
	}

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, key := range []string{"assignment-a", "assignment-b"} {
		wg.Add(1)
		go func(k string) {
			defer wg.Done()
			if err := store.SaveMerged(ctx, studentID, "assignments", merge(k)); err != nil {
				errs <- err
			}
		}(key)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}

	var got map[string]string
	if _, err := store.Load(ctx, studentID, "assignments", &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got["assignment-a"] != "assignment-a" || got["assignment-b"] != "assignment-b" {
		t.Fatalf("concurrent merged writes lost data: %+v", got)
	}
}

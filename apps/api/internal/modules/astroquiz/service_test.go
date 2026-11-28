package astroquiz

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/AICSbench/planetlogi/apps/api/internal/mathengine"
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/economy"
)

type recordingLedger struct {
	mu       sync.Mutex
	applied  []economy.MutationRequest
	failNext bool
}

func (l *recordingLedger) Apply(_ context.Context, req economy.MutationRequest) (economy.MutationResult, error) {
	return l.apply(req)
}

func (l *recordingLedger) ApplyBatch(_ context.Context, reqs []economy.MutationRequest) ([]economy.MutationResult, error) {
	out := make([]economy.MutationResult, 0, len(reqs))
	for _, req := range reqs {
		res, err := l.apply(req)
		if err != nil {
			return nil, err
		}
		out = append(out, res)
	}
	return out, nil
}

func (l *recordingLedger) apply(req economy.MutationRequest) (economy.MutationResult, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.failNext {
		l.failNext = false
		return economy.MutationResult{}, errors.New("ledger temporarily unavailable")
	}
	l.applied = append(l.applied, req)
	return economy.MutationResult{BalanceAfter: req.Delta}, nil
}

func (l *recordingLedger) totalAC() int64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	var total int64
	for _, req := range l.applied {
		if req.Currency == economy.CurrencyAC {
			total += req.Delta
		}
	}
	return total
}

func TestAstroQuizHiddenSolutionsAndWeeklySupplyPodCap(t *testing.T) {
	ledger := &recordingLedger{}
	svc := NewService(ledger)

	papers, claimed, err := svc.ListPapers(context.Background(), "stu-1", 0, "ALL", "en")
	if err != nil {
		t.Fatal(err)
	}
	if len(papers) != 18 || claimed {
		t.Fatalf("expected 18 papers across 3 categories x 6 grades and unclaimed pod, got %d / %v", len(papers), claimed)
	}

	// Open paper -> solutions MUST stay hidden before full submit
	opened, err := svc.OpenPaper(context.Background(), "stu-1", "G1-ESS-01", "en")
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range opened.Questions {
		if q.WorkedSolution != nil {
			t.Fatalf("solution for Q%d leaked while IN_PROGRESS", q.Index)
		}
	}

	// Negative elapsed time and out-of-range draft keys are rejected/dropped.
	// The client-reported elapsedSec is ignored: the server measures the timer.
	if saved, err := svc.SavePaper(context.Background(), "stu-1", "G1-ESS-01", "en", -1, nil); err != nil || saved.ElapsedSec < 0 {
		t.Fatalf("client elapsedSec must be ignored: elapsed=%d err=%v", saved.ElapsedSec, err)
	}
	if _, err := svc.SavePaper(context.Background(), "stu-1", "G1-ESS-01", "en", 5, map[int]map[string]string{
		0:  {"answer": "junk"},
		11: {"answer": "junk"},
	}); err != nil {
		t.Fatalf("out-of-range drafts should be ignored, got %v", err)
	}
	reopened, err := svc.OpenPaper(context.Background(), "stu-1", "G1-ESS-01", "en")
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range reopened.Questions {
		if _, ok := q.DraftAnswer["answer"]; ok {
			t.Fatalf("out-of-range draft was stored on Q%d", q.Index)
		}
	}

	// Prepare correct answers and submit paper 1 -> awards 1 Weekly Supply Pod (+5 AC)
	drafts := map[int]map[string]string{}
	for _, pq := range opened.Questions {
		prob, _ := mathengine.GenerateByCode(pq.Question.TemplateCode, pq.Question.Seed, "en")
		drafts[pq.Index] = prob.Solution.CorrectAnswers
	}
	sub1, err := svc.SubmitPaper(context.Background(), "stu-1", "G1-ESS-01", "en", 120, drafts)
	if err != nil {
		t.Fatal(err)
	}
	if sub1.Score != 10 || !sub1.SupplyPodAwarded {
		t.Fatalf("expected 10/10 and Supply Pod awarded: %+v", sub1)
	}
	for _, q := range sub1.Questions {
		if q.WorkedSolution == nil {
			t.Fatalf("solution for Q%d missing after submit", q.Index)
		}
	}
	if got := ledger.totalAC(); got != 5 {
		t.Fatalf("expected exactly +5 AC credited once, got %d", got)
	}

	// Submitting a 2nd paper in the same ISO week MUST NOT award a 2nd Supply Pod
	sub2, err := svc.SubmitPaper(context.Background(), "stu-1", "G1-TOP-01", "en", 90, map[int]map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	if sub2.SupplyPodAwarded {
		t.Fatalf("weekly Supply Pod cap (1/week) violated")
	}
	if got := ledger.totalAC(); got != 5 {
		t.Fatalf("expected no second Supply Pod credit, got %d", got)
	}
}

func TestAstroQuizSupplyPodRetriesAfterLedgerFailure(t *testing.T) {
	ledger := &recordingLedger{failNext: true}
	svc := NewService(ledger)

	opened, err := svc.OpenPaper(context.Background(), "stu-1", "G1-ESS-01", "en")
	if err != nil {
		t.Fatal(err)
	}
	drafts := map[int]map[string]string{}
	for _, pq := range opened.Questions {
		prob, _ := mathengine.GenerateByCode(pq.Question.TemplateCode, pq.Question.Seed, "en")
		drafts[pq.Index] = prob.Solution.CorrectAnswers
	}
	if _, err := svc.SubmitPaper(context.Background(), "stu-1", "G1-ESS-01", "en", 60, drafts); err == nil {
		t.Fatal("expected the failed Supply Pod credit to surface as an error")
	}

	// The paper stays submitted and a retry credits the pod exactly once.
	sub, err := svc.SubmitPaper(context.Background(), "stu-1", "G1-ESS-01", "en", 60, drafts)
	if err != nil {
		t.Fatalf("retry after ledger failure: %v", err)
	}
	if !sub.SupplyPodAwarded || sub.Score != 10 {
		t.Fatalf("expected retry to award the pod and keep the score, got %+v", sub)
	}
	if got := ledger.totalAC(); got != 5 {
		t.Fatalf("expected exactly +5 AC credited once, got %d", got)
	}
}

// The content manifest is the authoring contract for the compiled paper
// catalog; load it here so titles and templates cannot drift silently.
func TestPapersMatchContentManifest(t *testing.T) {
	raw, err := os.ReadFile("../../../../../content/quizzes/g1-g6-papers.json")
	if errors.Is(err, os.ErrNotExist) {
		t.Skip("content manifest not present in this checkout")
	}
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Papers []struct {
			Code         string            `json:"code"`
			Grade        int               `json:"grade"`
			Category     string            `json:"category"`
			Translations map[string]string `json:"translations"`
			Templates    []string          `json:"templates"`
		} `json:"papers"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	papers := buildPapers()
	if len(papers) != len(manifest.Papers) {
		t.Fatalf("paper count drift: catalog=%d manifest=%d", len(papers), len(manifest.Papers))
	}
	byCode := make(map[string]paperDef, len(papers))
	for _, p := range papers {
		byCode[p.code] = p
	}
	for _, mp := range manifest.Papers {
		p, ok := byCode[mp.Code]
		if !ok {
			t.Fatalf("paper %s missing from catalog", mp.Code)
		}
		if p.grade != mp.Grade || string(p.category) != mp.Category ||
			p.title.EN != mp.Translations["en"] || p.title.ID != mp.Translations["id"] {
			t.Fatalf("paper %s metadata drift", mp.Code)
		}
		if len(p.templates) != len(mp.Templates) {
			t.Fatalf("paper %s template count drift", mp.Code)
		}
		for i := range p.templates {
			if p.templates[i] != mp.Templates[i] {
				t.Fatalf("paper %s template %d drift", mp.Code, i)
			}
		}
	}
}

type quizMemoryStore struct {
	mu        sync.Mutex
	data      map[string][]byte
	failAfter int
}

func newQuizMemoryStore() *quizMemoryStore {
	return &quizMemoryStore{data: make(map[string][]byte)}
}

func (m *quizMemoryStore) key(studentID, module string) string { return studentID + ":" + module }

func (m *quizMemoryStore) Load(_ context.Context, studentID, module string, dest any) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	raw, ok := m.data[m.key(studentID, module)]
	if !ok {
		return false, nil
	}
	return true, json.Unmarshal(raw, dest)
}

func (m *quizMemoryStore) Save(_ context.Context, studentID, module string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.data[m.key(studentID, module)] = raw
	m.mu.Unlock()
	return nil
}

func (m *quizMemoryStore) SaveMerged(_ context.Context, studentID, module string, merge func([]byte) (any, error)) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failAfter > 0 {
		m.failAfter--
		if m.failAfter == 0 {
			return errors.New("store temporarily unavailable")
		}
	}
	key := m.key(studentID, module)
	merged, err := merge(m.data[key])
	if err != nil {
		return err
	}
	raw, err := json.Marshal(merged)
	if err != nil {
		return err
	}
	m.data[key] = raw
	return nil
}

func TestSupplyPodClaimWeekSurvivesWeekBoundary(t *testing.T) {
	const studentID = "22222222-2222-2222-2222-222222222222"
	store := newQuizMemoryStore()
	ledger := &recordingLedger{failNext: true}

	week1 := time.Date(2026, 11, 23, 10, 0, 0, 0, time.UTC)
	svc := NewServiceWithStore(store, ledger)
	svc.nowClock = func() time.Time { return week1 }

	opened, err := svc.OpenPaper(context.Background(), studentID, "G1-ESS-01", "en")
	if err != nil {
		t.Fatal(err)
	}
	drafts := map[int]map[string]string{}
	for _, pq := range opened.Questions {
		prob, _ := mathengine.GenerateByCode(pq.Question.TemplateCode, pq.Question.Seed, "en")
		drafts[pq.Index] = prob.Solution.CorrectAnswers
	}
	if _, err := svc.SubmitPaper(context.Background(), studentID, "G1-ESS-01", "en", 60, drafts); err == nil {
		t.Fatal("expected the first ledger attempt to fail")
	}

	// The retry happens a week later and after a service restart.
	week2 := week1.AddDate(0, 0, 8)
	reloaded := NewServiceWithStore(store, ledger)
	reloaded.nowClock = func() time.Time { return week2 }
	sub, err := reloaded.SubmitPaper(context.Background(), studentID, "G1-ESS-01", "en", 60, drafts)
	if err != nil {
		t.Fatalf("retry after week boundary: %v", err)
	}
	if !sub.SupplyPodAwarded {
		t.Fatal("expected the original Supply Pod to be awarded on retry")
	}
	ledger.mu.Lock()
	defer ledger.mu.Unlock()
	if len(ledger.applied) != 1 {
		t.Fatalf("expected one credit, got %d", len(ledger.applied))
	}
	year, week := week1.ISOWeek()
	want := fmt.Sprintf("supply_pod:%s:%04d-W%02d", studentID, year, week)
	if ledger.applied[0].IdempotencyKey != want {
		t.Fatalf("retry changed the claim key: got %q want %q", ledger.applied[0].IdempotencyKey, want)
	}
}

func TestConcurrentPrintAndSubmit(t *testing.T) {
	svc := NewService(nil)
	opened, err := svc.OpenPaper(context.Background(), "stu-1", "G1-ESS-01", "en")
	if err != nil {
		t.Fatal(err)
	}
	drafts := map[int]map[string]string{}
	for _, pq := range opened.Questions {
		prob, _ := mathengine.GenerateByCode(pq.Question.TemplateCode, pq.Question.Seed, "en")
		drafts[pq.Index] = prob.Solution.CorrectAnswers
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, _ = svc.GetPrintablePaper(context.Background(), "stu-1", "G1-ESS-01", "en", true, false)
		}()
		go func() {
			defer wg.Done()
			_, _ = svc.SubmitPaper(context.Background(), "stu-1", "G1-ESS-01", "en", 10, drafts)
		}()
	}
	wg.Wait()
}

func TestMergeQuizStateKeepsSubmittedAnswers(t *testing.T) {
	current := quizState{Attempts: map[string]attemptStateDTO{
		"G1-ESS-01": {
			Status: "SUBMITTED", Score: 9,
			Drafts: map[int]map[string]string{1: {"answer": "A"}},
		},
	}}
	raw, _ := json.Marshal(current)
	snapshot := quizState{Attempts: map[string]attemptStateDTO{
		"G1-ESS-01": {
			Status: "IN_PROGRESS",
			Drafts: map[int]map[string]string{1: {"answer": "B"}},
		},
	}}
	merged, err := mergeQuizState(raw, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	rec := merged.(quizState).Attempts["G1-ESS-01"]
	if rec.Status != "SUBMITTED" || rec.Score != 9 || rec.Drafts[1]["answer"] != "A" {
		t.Fatalf("stale snapshot replaced submitted answers/score: %+v", rec)
	}
}

func TestMergeQuizStateKeepsIncomingSubmittedUnit(t *testing.T) {
	current := quizState{Attempts: map[string]attemptStateDTO{
		"G1-ESS-01": {
			Status:         "IN_PROGRESS",
			Drafts:         map[int]map[string]string{1: {"answer": "stale-newer"}},
			DraftUpdatedAt: map[int]int64{1: 200},
		},
	}}
	raw, _ := json.Marshal(current)
	snapshot := quizState{Attempts: map[string]attemptStateDTO{
		"G1-ESS-01": {
			Status:         "SUBMITTED",
			Score:          9,
			Drafts:         map[int]map[string]string{1: {"answer": "submitted"}},
			DraftUpdatedAt: map[int]int64{1: 100},
		},
	}}
	merged, err := mergeQuizState(raw, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	rec := merged.(quizState).Attempts["G1-ESS-01"]
	if rec.Status != "SUBMITTED" || rec.Score != 9 || rec.Drafts[1]["answer"] != "submitted" {
		t.Fatalf("submitted snapshot was split by draft merge: %+v", rec)
	}
}

func TestSupplyPodRetryableUsesPaperClaimWeek(t *testing.T) {
	const studentID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	store := newQuizMemoryStore()
	ledger := &recordingLedger{failNext: true}
	svc := NewServiceWithStore(store, ledger)
	week1 := time.Date(2026, 11, 23, 10, 0, 0, 0, time.UTC)
	week2 := week1.AddDate(0, 0, 8)
	svc.nowClock = func() time.Time { return week1 }
	ctx := context.Background()

	a, err := svc.OpenPaper(ctx, studentID, "G1-ESS-01", "en")
	if err != nil {
		t.Fatal(err)
	}
	aDrafts := map[int]map[string]string{}
	for _, q := range a.Questions {
		prob, _ := mathengine.GenerateByCode(q.Question.TemplateCode, q.Question.Seed, "en")
		aDrafts[q.Index] = prob.Solution.CorrectAnswers
	}
	if _, err := svc.SubmitPaper(ctx, studentID, "G1-ESS-01", "en", 0, aDrafts); !errors.Is(err, ErrRewardUnavailable) {
		t.Fatalf("expected A's credit to fail, got %v", err)
	}
	if view, err := svc.OpenPaper(ctx, studentID, "G1-ESS-01", "en"); err != nil || !view.SupplyPodRetryable {
		t.Fatalf("paper A should be retryable in its own week: %+v err=%v", view, err)
	}

	svc.nowClock = func() time.Time { return week2 }
	b, err := svc.OpenPaper(ctx, studentID, "G1-TOP-01", "en")
	if err != nil {
		t.Fatal(err)
	}
	bDrafts := map[int]map[string]string{}
	for _, q := range b.Questions {
		prob, _ := mathengine.GenerateByCode(q.Question.TemplateCode, q.Question.Seed, "en")
		bDrafts[q.Index] = prob.Solution.CorrectAnswers
	}
	if _, err := svc.SubmitPaper(ctx, studentID, "G1-TOP-01", "en", 0, bDrafts); err != nil {
		t.Fatal(err)
	}

	view, err := svc.OpenPaper(ctx, studentID, "G1-ESS-01", "en")
	if err != nil {
		t.Fatal(err)
	}
	if !view.SupplyPodRetryable {
		t.Fatal("paper A's W1 claim is still pending, but the current-week flag hid Retry")
	}
	if !view.SupplyPodClaimedWk {
		t.Fatal("test expected the current week to be claimed by paper B")
	}
	if _, err := svc.SubmitPaper(ctx, studentID, "G1-ESS-01", "en", 0, map[int]map[string]string{}); err != nil {
		t.Fatal(err)
	}
	if got := ledger.totalAC(); got != 2*WeeklySupplyPodRewardAC {
		t.Fatalf("expected W1 and W2 credits, got %d", got)
	}
}

func TestSupplyPodAwardSurvivesFailedFinalPersist(t *testing.T) {
	const studentID = "99999999-9999-9999-9999-999999999999"
	store := newQuizMemoryStore()
	ledger := &recordingLedger{}
	svc := NewServiceWithStore(store, ledger)
	ctx := context.Background()

	opened, err := svc.OpenPaper(ctx, studentID, "G1-ESS-01", "en")
	if err != nil {
		t.Fatal(err)
	}
	drafts := map[int]map[string]string{}
	for _, q := range opened.Questions {
		prob, _ := mathengine.GenerateByCode(q.Question.TemplateCode, q.Question.Seed, "en")
		drafts[q.Index] = prob.Solution.CorrectAnswers
	}

	// Let the submission persist, then fail the post-ledger persist.
	store.failAfter = 2
	if _, err := svc.SubmitPaper(ctx, studentID, "G1-ESS-01", "en", 0, drafts); err == nil {
		t.Fatal("expected the final persist to fail")
	}
	if got := ledger.totalAC(); got != WeeklySupplyPodRewardAC {
		t.Fatalf("expected one ledger credit, got %d", got)
	}

	view, err := svc.SubmitPaper(ctx, studentID, "G1-ESS-01", "en", 0, map[int]map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	if !view.SupplyPodAwarded {
		t.Fatal("paper lost its Supply Pod badge after the failed final write")
	}
	if got := ledger.totalAC(); got != WeeklySupplyPodRewardAC {
		t.Fatalf("retry credited the pod twice: %d", got)
	}
	var stored quizState
	if _, err := store.Load(ctx, studentID, "astroquiz", &stored); err != nil {
		t.Fatal(err)
	}
	if !stored.Attempts["G1-ESS-01"].SupplyPodAwarded {
		t.Fatal("durable attempt lost the Supply Pod badge")
	}
}

func TestSubmitGradesDurableDraftsAcrossInstances(t *testing.T) {
	const studentID = "77777777-7777-7777-7777-777777777777"
	store := newQuizMemoryStore()
	a := NewServiceWithStore(store, nil)
	b := NewServiceWithStore(store, nil)
	ctx := context.Background()

	opened, err := a.OpenPaper(ctx, studentID, "G1-ESS-01", "en")
	if err != nil {
		t.Fatal(err)
	}
	// B caches an empty attempt before A saves the answer.
	if _, err := b.OpenPaper(ctx, studentID, "G1-ESS-01", "en"); err != nil {
		t.Fatal(err)
	}
	q1 := opened.Questions[0]
	prob, _ := mathengine.GenerateByCode(q1.Question.TemplateCode, q1.Question.Seed, "en")
	if _, err := a.SavePaper(ctx, studentID, "G1-ESS-01", "en", 0, map[int]map[string]string{1: prob.Solution.CorrectAnswers}); err != nil {
		t.Fatal(err)
	}

	submitted, err := b.SubmitPaper(ctx, studentID, "G1-ESS-01", "en", 0, map[int]map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	if submitted.Score != 1 {
		t.Fatalf("stale instance graded %d instead of the durable 1/10 answer", submitted.Score)
	}
	var stored quizState
	if _, err := store.Load(ctx, studentID, "astroquiz", &stored); err != nil {
		t.Fatal(err)
	}
	if got := stored.Attempts["G1-ESS-01"]; got.Status != "SUBMITTED" || got.Score != 1 {
		t.Fatalf("durable submission did not keep the graded answer: %+v", got)
	} else if got.Drafts[1]["answer"] != prob.Solution.CorrectAnswers["answer"] {
		t.Fatalf("durable submission lost the graded Q1 answer: %+v", got.Drafts[1])
	}
}

func TestStaleInstanceUsesPersistedClaimWeek(t *testing.T) {
	const studentID = "44444444-4444-4444-4444-444444444444"
	store := newQuizMemoryStore()
	ledger := &recordingLedger{failNext: true}
	week1 := time.Date(2026, 11, 23, 10, 0, 0, 0, time.UTC)
	week2 := week1.AddDate(0, 0, 8)

	a := NewServiceWithStore(store, ledger)
	a.nowClock = func() time.Time { return week1 }
	b := NewServiceWithStore(store, ledger)
	b.nowClock = func() time.Time { return week2 }

	opened, err := a.OpenPaper(context.Background(), studentID, "G1-ESS-01", "en")
	if err != nil {
		t.Fatal(err)
	}
	// Load B's cache before A persists the original claim week.
	if _, err := b.OpenPaper(context.Background(), studentID, "G1-ESS-01", "en"); err != nil {
		t.Fatal(err)
	}
	drafts := map[int]map[string]string{}
	for _, pq := range opened.Questions {
		prob, _ := mathengine.GenerateByCode(pq.Question.TemplateCode, pq.Question.Seed, "en")
		drafts[pq.Index] = prob.Solution.CorrectAnswers
	}
	if _, err := a.SubmitPaper(context.Background(), studentID, "G1-ESS-01", "en", 60, drafts); err == nil {
		t.Fatal("expected A's ledger attempt to fail")
	}
	if _, err := b.SubmitPaper(context.Background(), studentID, "G1-ESS-01", "en", 60, drafts); err != nil {
		t.Fatalf("B retry: %v", err)
	}
	ledger.mu.Lock()
	defer ledger.mu.Unlock()
	if len(ledger.applied) != 1 {
		t.Fatalf("expected one credit, got %d", len(ledger.applied))
	}
	year, week := week1.ISOWeek()
	want := fmt.Sprintf("supply_pod:%s:%04d-W%02d", studentID, year, week)
	if ledger.applied[0].IdempotencyKey != want {
		t.Fatalf("stale instance used a different claim week: got %q want %q", ledger.applied[0].IdempotencyKey, want)
	}
}

func TestMergeQuizStateKeepsNewerDraft(t *testing.T) {
	current := quizState{Attempts: map[string]attemptStateDTO{
		"G1-ESS-01": {
			Status:         "IN_PROGRESS",
			Drafts:         map[int]map[string]string{1: {"answer": "new"}},
			DraftUpdatedAt: map[int]int64{1: 200},
		},
	}}
	raw, _ := json.Marshal(current)
	snapshot := quizState{Attempts: map[string]attemptStateDTO{
		"G1-ESS-01": {
			Status:         "IN_PROGRESS",
			Drafts:         map[int]map[string]string{1: {"answer": "old"}},
			DraftUpdatedAt: map[int]int64{1: 100},
		},
	}}
	merged, err := mergeQuizState(raw, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	rec := merged.(quizState).Attempts["G1-ESS-01"]
	if rec.Drafts[1]["answer"] != "new" {
		t.Fatalf("older snapshot overwrote a newer draft: %+v", rec.Drafts)
	}
}

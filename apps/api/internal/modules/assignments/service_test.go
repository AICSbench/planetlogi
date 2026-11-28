package assignments

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/AICSbench/planetlogi/apps/api/internal/mathengine"
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/economy"
)

// recordingLedger captures reward mutations so tests can assert the wallet
// effect, not just an in-memory completion flag.
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

func TestParentCustomAssignmentAndCompletionReward(t *testing.T) {
	ledger := &recordingLedger{}
	svc := NewService(ledger)
	svc.LinkParentChild("par-1", "stu-1")

	// Unlinked parent MUST be blocked
	if _, err := svc.CreateParentCustomAssignment(context.Background(), "par-other", "Other", "stu-1", "Test", "", []string{"g1_add_within_20"}, ""); err != ErrNotLinkedChild {
		t.Fatalf("expected ErrNotLinkedChild, got %v", err)
	}

	// Invalid due dates are rejected
	if _, err := svc.CreateParentCustomAssignment(context.Background(), "par-1", "Budi", "stu-1", "Test", "tomorrow", []string{"g1_add_within_20"}, ""); err == nil {
		t.Fatal("expected invalid dueDate to be rejected")
	}

	// Linked parent creates custom 2-question assignment
	created, err := svc.CreateParentCustomAssignment(context.Background(), "par-1", "Budi", "stu-1", "Weekend Addition Drill", "2026-12-01", []string{"g1_add_within_20", "g1_sub_within_20"}, "")
	if err != nil || created.QuestionCount != 2 || created.AssignedByRole != AssignedByParent {
		t.Fatalf("unexpected created assignment: %+v err=%v", created, err)
	}
	if created.ID == "asgn-1" {
		t.Fatalf("custom assignment reused the seeded ID: %s", created.ID)
	}

	detail, err := svc.GetDetail(context.Background(), "stu-1", created.ID, "en")
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range detail.Questions {
		prob, _ := mathengine.GenerateByCode(q.Question.TemplateCode, q.Question.Seed, "en")
		if q.Index == len(detail.Questions) {
			ledger.failNext = true
			if _, _, err = svc.SubmitQuestion(context.Background(), "stu-1", created.ID, q.Index, prob.Solution.CorrectAnswers, "en"); err == nil {
				t.Fatal("expected the failed reward to surface as an error")
			}
			// Retrying the final question after a ledger failure must credit
			// the reward exactly once without overwriting answers.
			if _, detail, err = svc.SubmitQuestion(context.Background(), "stu-1", created.ID, q.Index, prob.Solution.CorrectAnswers, "en"); err != nil {
				t.Fatalf("retry after ledger failure: %v", err)
			}
			continue
		}
		if _, detail, err = svc.SubmitQuestion(context.Background(), "stu-1", created.ID, q.Index, prob.Solution.CorrectAnswers, "en"); err != nil {
			t.Fatal(err)
		}
	}
	if detail.Summary.Status != "COMPLETE" || detail.Summary.Score != 2 {
		t.Fatalf("expected COMPLETE with score 2/2, got %+v", detail.Summary)
	}
	if got := ledger.totalAC(); got != 5 {
		t.Fatalf("expected exactly +5 AC credited once, got %d", got)
	}

	// A second submission for an answered question is rejected.
	q := detail.Questions[0]
	prob, _ := mathengine.GenerateByCode(q.Question.TemplateCode, q.Question.Seed, "en")
	if _, _, err := svc.SubmitQuestion(context.Background(), "stu-1", created.ID, 1, prob.Solution.CorrectAnswers, "en"); err == nil {
		t.Fatal("expected duplicate question submission to be rejected")
	}
}

func TestCreateAssignmentRequestIDIsIdempotent(t *testing.T) {
	svc := NewService(nil)
	svc.LinkParentChild("par-1", "stu-1")
	ctx := context.Background()

	first, err := svc.CreateParentCustomAssignment(ctx, "par-1", "Budi", "stu-1", "Retry Safe", "2026-12-01", []string{"g1_add_within_20"}, "req-1")
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.CreateParentCustomAssignment(ctx, "par-1", "Budi", "stu-1", "Retry Safe", "2026-12-01", []string{"g1_add_within_20"}, "req-1")
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID {
		t.Fatalf("retried create produced a second assignment: %s vs %s", first.ID, second.ID)
	}
	list, err := svc.ListForStudent(ctx, "stu-1", "ALL", "ALL", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("expected one assignment, got %d", len(list))
	}
}

type assignmentMemoryStore struct {
	mu       sync.Mutex
	data     map[string][]byte
	failNext bool
	entered  chan struct{}
	release  chan struct{}
}

func newAssignmentMemoryStore() *assignmentMemoryStore {
	return &assignmentMemoryStore{data: make(map[string][]byte)}
}

func (m *assignmentMemoryStore) key(studentID, module string) string { return studentID + ":" + module }

func (m *assignmentMemoryStore) Load(_ context.Context, studentID, module string, dest any) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	raw, ok := m.data[m.key(studentID, module)]
	if !ok {
		return false, nil
	}
	return true, json.Unmarshal(raw, dest)
}

func (m *assignmentMemoryStore) Save(_ context.Context, studentID, module string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.data[m.key(studentID, module)] = raw
	m.mu.Unlock()
	return nil
}

func (m *assignmentMemoryStore) SaveMerged(_ context.Context, studentID, module string, merge func([]byte) (any, error)) error {
	if m.entered != nil {
		close(m.entered)
		<-m.release
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failNext {
		m.failNext = false
		return errors.New("store temporarily unavailable")
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

func TestAssignmentFinalSubmissionPersistsAfterRetry(t *testing.T) {
	const studentID = "33333333-3333-3333-3333-333333333333"
	store := newAssignmentMemoryStore()
	svc := NewServiceWithDeps(nil, store, nil)
	svc.LinkParentChild("par-1", studentID)
	ctx := context.Background()

	created, err := svc.CreateParentCustomAssignment(ctx, "par-1", "Budi", studentID, "Persist Retry", "2026-12-01", []string{"g1_add_within_20", "g1_sub_within_20"}, "req-persist")
	if err != nil {
		t.Fatal(err)
	}
	detail, err := svc.GetDetail(ctx, studentID, created.ID, "en")
	if err != nil {
		t.Fatal(err)
	}
	q1 := detail.Questions[0]
	prob1, _ := mathengine.GenerateByCode(q1.Question.TemplateCode, q1.Question.Seed, "en")
	if _, _, err := svc.SubmitQuestion(ctx, studentID, created.ID, q1.Index, prob1.Solution.CorrectAnswers, "en"); err != nil {
		t.Fatal(err)
	}

	q2 := detail.Questions[1]
	prob2, _ := mathengine.GenerateByCode(q2.Question.TemplateCode, q2.Question.Seed, "en")
	store.failNext = true
	if _, _, err := svc.SubmitQuestion(ctx, studentID, created.ID, q2.Index, prob2.Solution.CorrectAnswers, "en"); err == nil {
		t.Fatal("expected the failed save to surface")
	}
	// The same final submission flushes the completed snapshot after the store recovers.
	if _, detail, err = svc.SubmitQuestion(ctx, studentID, created.ID, q2.Index, prob2.Solution.CorrectAnswers, "en"); err != nil {
		t.Fatalf("retry after save failure: %v", err)
	}
	if detail.Summary.Status != "COMPLETE" {
		t.Fatalf("expected COMPLETE after retry, got %+v", detail.Summary)
	}

	reloaded := NewServiceWithDeps(nil, store, nil)
	list, err := reloaded.ListForStudent(ctx, studentID, "ALL", "ALL", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Status != "COMPLETE" {
		t.Fatalf("completed assignment was not durable: %+v", list)
	}
}

func TestAssignmentReadsSurvivePendingRewardFailure(t *testing.T) {
	const studentID = "55555555-5555-5555-5555-555555555555"
	store := newAssignmentMemoryStore()
	writerLedger := &recordingLedger{}
	writer := NewServiceWithDeps(nil, store, writerLedger)
	writer.LinkParentChild("par-1", studentID)
	ctx := context.Background()

	created, err := writer.CreateParentCustomAssignment(ctx, "par-1", "Budi", studentID, "Reward retry", "2026-12-01", []string{"g1_add_within_20"}, "req-reward-retry")
	if err != nil {
		t.Fatal(err)
	}
	detail, err := writer.GetDetail(ctx, studentID, created.ID, "en")
	if err != nil {
		t.Fatal(err)
	}
	prob, _ := mathengine.GenerateByCode(detail.Questions[0].Question.TemplateCode, detail.Questions[0].Question.Seed, "en")
	writerLedger.failNext = true
	if _, _, err := writer.SubmitQuestion(ctx, studentID, created.ID, 1, prob.Solution.CorrectAnswers, "en"); !errors.Is(err, ErrRewardUnavailable) {
		t.Fatalf("expected reward error, got %v", err)
	}

	readerLedger := &recordingLedger{failNext: true}
	reader := NewServiceWithDeps(nil, store, readerLedger)
	list, err := reader.ListForStudent(ctx, studentID, "ALL", "ALL", 0)
	if err != nil {
		t.Fatalf("read failed while reward was pending: %v", err)
	}
	if len(list) != 1 || list[0].Status != "COMPLETE" {
		t.Fatalf("completed assignment not visible: %+v", list)
	}
	if len(readerLedger.applied) != 0 {
		t.Fatal("reward was credited despite the injected ledger failure")
	}
	if _, err := reader.ListForStudent(ctx, studentID, "ALL", "ALL", 0); err != nil {
		t.Fatal(err)
	}
	if len(readerLedger.applied) != 1 {
		t.Fatalf("pending reward was not retried: %d credits", len(readerLedger.applied))
	}
}

func TestAssignmentListReloadsRecordsWrittenByAnotherInstance(t *testing.T) {
	const studentID = "44444444-4444-4444-4444-444444444444"
	store := newAssignmentMemoryStore()
	writer := NewServiceWithDeps(nil, store, nil)
	writer.LinkParentChild("par-1", studentID)
	reader := NewServiceWithDeps(nil, store, nil)
	ctx := context.Background()

	if list, err := reader.ListForStudent(ctx, studentID, "ALL", "ALL", 0); err != nil || len(list) != 0 {
		t.Fatalf("prime reader cache: list=%v err=%v", list, err)
	}
	if _, err := writer.CreateParentCustomAssignment(ctx, "par-1", "Budi", studentID, "From another instance", "2026-12-01", []string{"g1_add_within_20"}, "req-cross-instance"); err != nil {
		t.Fatal(err)
	}
	list, err := reader.ListForStudent(ctx, studentID, "ALL", "ALL", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Title != "From another instance" {
		t.Fatalf("reader did not reload the persisted assignment: %+v", list)
	}
}

func TestMergeAssignmentRecordKeepsCorrectnessWithWinningAnswer(t *testing.T) {
	existing := assignmentRecordDTO{
		Submitted:   map[int]map[string]string{1: {"answer": "new"}},
		SubmittedAt: map[int]int64{1: 200},
		Correct:     map[int]bool{1: true},
	}
	stale := assignmentRecordDTO{
		Submitted:   map[int]map[string]string{1: {"answer": "old"}},
		SubmittedAt: map[int]int64{1: 100},
		Correct:     map[int]bool{1: false},
	}
	merged := mergeAssignmentRecord(existing, stale)
	if merged.Submitted[1]["answer"] != "new" {
		t.Fatalf("older answer won the merge: %+v", merged.Submitted)
	}
	if !merged.Correct[1] {
		t.Fatal("stale correctness overwrote the stored answer's result")
	}
}

func TestMergeAssignmentStateDerivesCompletion(t *testing.T) {
	current := assignmentState{Records: map[string]assignmentRecordDTO{
		"asgn-x": {
			ID: "asgn-x", Items: []itemSpecDTO{{TemplateCode: "g1_add_within_20"}, {TemplateCode: "g1_sub_within_20"}},
			Submitted: map[int]map[string]string{1: {"answer": "1"}},
			Correct:   map[int]bool{1: true},
		},
	}}
	raw, _ := json.Marshal(current)
	snapshot := assignmentState{Records: map[string]assignmentRecordDTO{
		"asgn-x": {
			ID: "asgn-x", Items: current.Records["asgn-x"].Items,
			Submitted: map[int]map[string]string{2: {"answer": "2"}},
			Correct:   map[int]bool{2: true},
		},
	}}
	merged, err := mergeAssignmentState(raw, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	rec := merged.(assignmentState).Records["asgn-x"]
	if !rec.Completed {
		t.Fatal("merged answers covering every item must mark the assignment complete")
	}
	if len(rec.Submitted) != 2 || !rec.Correct[1] || !rec.Correct[2] {
		t.Fatalf("merged answers lost: %+v", rec)
	}
}

func TestMergeAssignmentStateDeduplicatesRequestID(t *testing.T) {
	items := []itemSpecDTO{{TemplateCode: "g1_add_within_20"}}
	current := assignmentState{Records: map[string]assignmentRecordDTO{
		"asgn-a": {ID: "asgn-a", StudentID: "stu-1", RequestID: "req-1", Items: items},
	}}
	raw, _ := json.Marshal(current)
	snapshot := assignmentState{Records: map[string]assignmentRecordDTO{
		"asgn-b": {
			ID: "asgn-b", StudentID: "stu-1", RequestID: "req-1", Items: items,
			Submitted: map[int]map[string]string{1: {"answer": "9"}},
		},
	}}
	merged, err := mergeAssignmentState(raw, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	records := merged.(assignmentState).Records
	if len(records) != 1 {
		t.Fatalf("request ID created a duplicate assignment: %+v", records)
	}
	if records["asgn-a"].Submitted[1]["answer"] != "9" {
		t.Fatal("deduplicated record lost the newer submission")
	}
}

func TestPersistAcknowledgesOnlySavedRevision(t *testing.T) {
	const studentID = "55555555-5555-5555-5555-555555555555"
	store := newAssignmentMemoryStore()
	svc := NewServiceWithDeps(nil, store, nil)
	svc.LinkParentChild("par-1", studentID)
	ctx := context.Background()
	created, err := svc.CreateParentCustomAssignment(ctx, "par-1", "Budi", studentID, "Revision", "2026-12-01", []string{"g1_add_within_20"}, "req-rev")
	if err != nil {
		t.Fatal(err)
	}

	store.entered = make(chan struct{})
	store.release = make(chan struct{})
	done := make(chan error, 1)
	go func() { done <- svc.persistStudent(ctx, studentID) }()
	<-store.entered
	// A newer change lands after the snapshot but before the old save acks.
	svc.mu.Lock()
	rec := svc.records[created.ID]
	rec.revision++
	rec.persisted = false
	svc.mu.Unlock()
	close(store.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	svc.mu.Lock()
	defer svc.mu.Unlock()
	if svc.records[created.ID].persisted {
		t.Fatal("an older save acknowledged a newer revision as persisted")
	}
}

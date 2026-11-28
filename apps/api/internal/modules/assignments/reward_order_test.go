package assignments

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/AICSbench/planetlogi/apps/api/internal/mathengine"
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/economy"
)

const stuUUID = "66666666-6666-6666-6666-666666666666"

func acCredited(l *recordingLedger) int64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	var total int64
	for _, r := range l.applied {
		if r.Currency == economy.CurrencyAC {
			total += r.Delta
		}
	}
	return total
}

// Completion AC must never be credited before the final answer is durable.
func TestCompletionRewardCreditedOnlyAfterAnswerIsDurable(t *testing.T) {
	ctx := context.Background()
	store := newAssignmentMemoryStore()
	ledger := &recordingLedger{}
	svc := NewServiceWithDeps(nil, store, ledger)
	svc.LinkParentChild("par-1", stuUUID)

	created, err := svc.CreateParentCustomAssignment(ctx, "par-1", "Budi", stuUUID, "Drill", "", []string{"g1_add_within_20"}, "req-order")
	if err != nil {
		t.Fatal(err)
	}
	detail, err := svc.GetDetail(ctx, stuUUID, created.ID, "en")
	if err != nil {
		t.Fatal(err)
	}
	q := detail.Questions[0]
	prob, _ := mathengine.GenerateByCode(q.Question.TemplateCode, q.Question.Seed, "en")

	store.failNext = true
	if _, _, err := svc.SubmitQuestion(ctx, stuUUID, created.ID, 1, prob.Solution.CorrectAnswers, "en"); err == nil {
		t.Fatal("expected the failed save to surface")
	}
	if got := acCredited(ledger); got != 0 {
		t.Fatalf("AC credited before the final answer was durable: %d", got)
	}
	if _, _, err := svc.SubmitQuestion(ctx, stuUUID, created.ID, 1, prob.Solution.CorrectAnswers, "en"); err != nil {
		t.Fatalf("retry: %v", err)
	}
	if got := acCredited(ledger); got != 5 {
		t.Fatalf("expected exactly +5 AC after the retry, got %d", got)
	}
}

type storeCheckingLedger struct {
	*recordingLedger
	store        *assignmentMemoryStore
	studentID    string
	assignmentID string
	t            *testing.T
}

func (l *storeCheckingLedger) Apply(ctx context.Context, req economy.MutationRequest) (economy.MutationResult, error) {
	var st assignmentState
	found, err := l.store.Load(ctx, l.studentID, "assignments", &st)
	if err != nil || !found {
		l.t.Fatalf("ledger called without durable assignment state: found=%v err=%v", found, err)
	}
	rec, ok := st.Records[l.assignmentID]
	if !ok || !rec.Completed {
		l.t.Fatalf("ledger called before completion was durable: %+v", rec)
	}
	return l.recordingLedger.Apply(ctx, req)
}

func (l *storeCheckingLedger) ApplyBatch(ctx context.Context, reqs []economy.MutationRequest) ([]economy.MutationResult, error) {
	return l.recordingLedger.ApplyBatch(ctx, reqs)
}

type blockingAssignmentStore struct {
	*assignmentMemoryStore
	mu          sync.Mutex
	armed       bool
	blocked     bool
	entered     chan struct{}
	release     chan struct{}
	releaseOnce sync.Once
}

func (b *blockingAssignmentStore) releaseNow() {
	b.releaseOnce.Do(func() { close(b.release) })
}

func (b *blockingAssignmentStore) SaveMerged(ctx context.Context, studentID, module string, merge func([]byte) (any, error)) error {
	b.mu.Lock()
	if b.armed && !b.blocked {
		b.blocked = true
		b.mu.Unlock()
		close(b.entered)
		<-b.release
	} else {
		b.mu.Unlock()
	}
	return b.assignmentMemoryStore.SaveMerged(ctx, studentID, module, merge)
}

// Load-time reconciliation must also persist completion before crediting.
func TestLoadReconciliationPersistsBeforeCredit(t *testing.T) {
	ctx := context.Background()
	store := newAssignmentMemoryStore()
	ledger := &storeCheckingLedger{recordingLedger: &recordingLedger{}, store: store, studentID: stuUUID, t: t}
	svc := NewServiceWithDeps(nil, store, ledger)
	svc.LinkParentChild("par-1", stuUUID)

	created, err := svc.CreateParentCustomAssignment(ctx, "par-1", "Budi", stuUUID, "Drill", "", []string{"g1_add_within_20"}, "req-load-order")
	if err != nil {
		t.Fatal(err)
	}
	ledger.assignmentID = created.ID
	detail, err := svc.GetDetail(ctx, stuUUID, created.ID, "en")
	if err != nil {
		t.Fatal(err)
	}
	q := detail.Questions[0]
	prob, _ := mathengine.GenerateByCode(q.Question.TemplateCode, q.Question.Seed, "en")

	// The final answer reaches memory, but its save fails before the reward.
	store.failNext = true
	if _, _, err := svc.SubmitQuestion(ctx, stuUUID, created.ID, 1, prob.Solution.CorrectAnswers, "en"); err == nil {
		t.Fatal("expected the failed save to surface")
	}
	if got := acCredited(ledger.recordingLedger); got != 0 {
		t.Fatalf("AC credited before durable completion: %d", got)
	}

	// A read merges the pending completion. While storage is down it must
	// neither fail nor credit the reward.
	store.failNext = true
	if _, err := svc.ListForStudent(ctx, stuUUID, "ALL", "ALL", 0); err != nil {
		t.Fatalf("read failed while completion was pending: %v", err)
	}
	if got := acCredited(ledger.recordingLedger); got != 0 {
		t.Fatalf("AC credited before completion was durable: %d", got)
	}

	if _, err := svc.ListForStudent(ctx, stuUUID, "ALL", "ALL", 0); err != nil {
		t.Fatal(err)
	}
	if got := acCredited(ledger.recordingLedger); got != 5 {
		t.Fatalf("expected exactly +5 AC after durable reconciliation, got %d", got)
	}
}

func TestFailedCreateAliasPreservesProgressWithoutDoubleReward(t *testing.T) {
	ctx := context.Background()
	store := newAssignmentMemoryStore()
	ledger := &storeCheckingLedger{recordingLedger: &recordingLedger{}, store: store, studentID: stuUUID, t: t}
	a := NewServiceWithDeps(nil, store, ledger)
	b := NewServiceWithDeps(nil, store, ledger)
	for _, svc := range []*Service{a, b} {
		svc.LinkParentChild("par-1", stuUUID)
	}

	store.failNext = true
	if _, err := a.CreateParentCustomAssignment(ctx, "par-1", "Budi", stuUUID, "Drill", "", []string{"g1_add_within_20"}, "req-failed-create"); err == nil {
		t.Fatal("expected create to fail")
	}
	var localID string
	a.mu.Lock()
	for id, rec := range a.records {
		if rec.requestID == "req-failed-create" {
			localID = id
		}
	}
	a.mu.Unlock()
	if localID == "" {
		t.Fatal("local assignment was not retained after the failed create")
	}
	local, err := a.GetDetail(ctx, stuUUID, localID, "en")
	if err != nil {
		t.Fatal(err)
	}
	prob, _ := mathengine.GenerateByCode(local.Questions[0].Question.TemplateCode, local.Questions[0].Question.Seed, "en")

	// The local answer stays in memory when its save fails; no reward yet.
	store.failNext = true
	if _, _, err := a.SubmitQuestion(ctx, stuUUID, localID, 1, prob.Solution.CorrectAnswers, "en"); err == nil {
		t.Fatal("expected local answer save to fail")
	}
	if got := acCredited(ledger.recordingLedger); got != 0 {
		t.Fatalf("local completion credited before the durable alias existed: %d", got)
	}

	durable, err := b.CreateParentCustomAssignment(ctx, "par-1", "Budi", stuUUID, "Drill", "", []string{"g1_add_within_20"}, "req-failed-create")
	if err != nil {
		t.Fatal(err)
	}
	if durable.ID == localID {
		t.Fatal("test expected the retry to use a different durable ID")
	}
	ledger.assignmentID = durable.ID
	list, err := a.ListForStudent(ctx, stuUUID, "ALL", "ALL", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != durable.ID || list[0].Status != "COMPLETE" {
		t.Fatalf("alias reconciliation lost progress or kept a duplicate: %+v", list)
	}
	if got := acCredited(ledger.recordingLedger); got != 5 {
		t.Fatalf("expected exactly one completion reward, got %d", got)
	}
	if _, err := a.GetDetail(ctx, stuUUID, localID, "en"); !errors.Is(err, ErrAssignmentNotFound) {
		t.Fatalf("local duplicate is still visible: %v", err)
	}
}

func TestSubmitQuestionUsesDurableIDWhenAliasCreatedDuringPersist(t *testing.T) {
	ctx := context.Background()
	base := newAssignmentMemoryStore()
	store := &blockingAssignmentStore{assignmentMemoryStore: base, entered: make(chan struct{}), release: make(chan struct{})}
	t.Cleanup(store.releaseNow)
	ledger := &recordingLedger{}
	a := NewServiceWithDeps(nil, store, ledger)
	b := NewServiceWithDeps(nil, store, ledger)
	for _, svc := range []*Service{a, b} {
		svc.LinkParentChild("par-1", stuUUID)
	}

	store.failNext = true
	if _, err := a.CreateParentCustomAssignment(ctx, "par-1", "Budi", stuUUID, "Drill", "", []string{"g1_add_within_20"}, "req-race-alias"); err == nil {
		t.Fatal("expected create to fail")
	}
	var localID string
	a.mu.Lock()
	for id, rec := range a.records {
		if rec.requestID == "req-race-alias" {
			localID = id
		}
	}
	a.mu.Unlock()
	store.mu.Lock()
	store.armed = true
	store.mu.Unlock()
	local, err := a.GetDetail(ctx, stuUUID, localID, "en")
	if err != nil {
		t.Fatal(err)
	}
	prob, _ := mathengine.GenerateByCode(local.Questions[0].Question.TemplateCode, local.Questions[0].Question.Seed, "en")

	result := make(chan error, 1)
	go func() {
		_, _, err := a.SubmitQuestion(ctx, stuUUID, localID, 1, prob.Solution.CorrectAnswers, "en")
		result <- err
	}()
	select {
	case <-store.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("SubmitQuestion did not reach persistence")
	}
	durable, err := b.CreateParentCustomAssignment(ctx, "par-1", "Budi", stuUUID, "Drill", "", []string{"g1_add_within_20"}, "req-race-alias")
	if err != nil {
		t.Fatal(err)
	}
	store.releaseNow()
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("SubmitQuestion stalled after release")
	}
	if got := acCredited(ledger); got != 5 {
		t.Fatalf("expected one credit, got %d", got)
	}
	ledger.mu.Lock()
	if len(ledger.applied) != 1 || ledger.applied[0].SourceID != durable.ID {
		t.Fatalf("reward used a non-durable assignment ID: %+v", ledger.applied)
	}
	ledger.mu.Unlock()
}

func TestLoadReconciliationDropsProgressForDifferentAliasItems(t *testing.T) {
	ctx := context.Background()
	store := newAssignmentMemoryStore()
	ledger := &recordingLedger{}
	a := NewServiceWithDeps(nil, store, ledger)
	b := NewServiceWithDeps(nil, store, ledger)
	for _, svc := range []*Service{a, b} {
		svc.LinkParentChild("par-1", stuUUID)
	}

	store.failNext = true
	if _, err := a.CreateParentCustomAssignment(ctx, "par-1", "Budi", stuUUID, "Drill", "", []string{"g1_add_within_20"}, "req-diff-alias"); err == nil {
		t.Fatal("expected create to fail")
	}
	var localID string
	a.mu.Lock()
	for id, rec := range a.records {
		if rec.requestID == "req-diff-alias" {
			localID = id
		}
	}
	a.mu.Unlock()
	local, err := a.GetDetail(ctx, stuUUID, localID, "en")
	if err != nil {
		t.Fatal(err)
	}
	prob, _ := mathengine.GenerateByCode(local.Questions[0].Question.TemplateCode, local.Questions[0].Question.Seed, "en")
	store.failNext = true
	if _, _, err := a.SubmitQuestion(ctx, stuUUID, localID, 1, prob.Solution.CorrectAnswers, "en"); err == nil {
		t.Fatal("expected local save to fail")
	}

	if _, err := b.CreateParentCustomAssignment(ctx, "par-1", "Budi", stuUUID, "Drill", "", []string{"g1_sub_within_20"}, "req-diff-alias"); err != nil {
		t.Fatal(err)
	}
	list, err := a.ListForStudent(ctx, stuUUID, "ALL", "ALL", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Status != "INCOMPLETE" {
		t.Fatalf("different-question alias kept discarded progress: %+v", list)
	}
	if got := acCredited(ledger); got != 0 {
		t.Fatalf("discarded local completion was rewarded: %d", got)
	}
}

func TestLoadReconciliationUsesDurableAliasIDForReward(t *testing.T) {
	ctx := context.Background()
	base := newAssignmentMemoryStore()
	store := &blockingAssignmentStore{assignmentMemoryStore: base, entered: make(chan struct{}), release: make(chan struct{})}
	t.Cleanup(store.releaseNow)
	ledger := &recordingLedger{}
	a := NewServiceWithDeps(nil, store, ledger)
	b := NewServiceWithDeps(nil, store, ledger)
	for _, svc := range []*Service{a, b} {
		svc.LinkParentChild("par-1", stuUUID)
	}

	store.failNext = true
	if _, err := a.CreateParentCustomAssignment(ctx, "par-1", "Budi", stuUUID, "Drill", "", []string{"g1_add_within_20"}, "req-load-race"); err == nil {
		t.Fatal("expected create to fail")
	}
	var localID string
	a.mu.Lock()
	for id, rec := range a.records {
		if rec.requestID == "req-load-race" {
			localID = id
		}
	}
	a.mu.Unlock()
	local, err := a.GetDetail(ctx, stuUUID, localID, "en")
	if err != nil {
		t.Fatal(err)
	}
	prob, _ := mathengine.GenerateByCode(local.Questions[0].Question.TemplateCode, local.Questions[0].Question.Seed, "en")
	store.failNext = true
	if _, _, err := a.SubmitQuestion(ctx, stuUUID, localID, 1, prob.Solution.CorrectAnswers, "en"); err == nil {
		t.Fatal("expected local save to fail")
	}

	store.mu.Lock()
	store.armed = true
	store.mu.Unlock()
	type listResult struct {
		list []AssignmentSummary
		err  error
	}
	result := make(chan listResult, 1)
	go func() {
		list, err := a.ListForStudent(ctx, stuUUID, "ALL", "ALL", 0)
		result <- listResult{list: list, err: err}
	}()
	select {
	case <-store.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("ListForStudent did not reach persistence")
	}
	durable, err := b.CreateParentCustomAssignment(ctx, "par-1", "Budi", stuUUID, "Drill", "", []string{"g1_add_within_20"}, "req-load-race")
	if err != nil {
		t.Fatal(err)
	}
	store.releaseNow()
	var res listResult
	select {
	case res = <-result:
	case <-time.After(2 * time.Second):
		t.Fatal("ListForStudent stalled after release")
	}
	if res.err != nil {
		t.Fatal(res.err)
	}
	if len(res.list) != 1 || res.list[0].ID != durable.ID || res.list[0].Status != "COMPLETE" {
		t.Fatalf("load reconciliation lost progress or used a local ID: %+v", res.list)
	}
	ledger.mu.Lock()
	if len(ledger.applied) != 1 || ledger.applied[0].SourceID != durable.ID {
		t.Fatalf("load reward used a non-durable ID: %+v", ledger.applied)
	}
	ledger.mu.Unlock()
}

func TestSubmitQuestionConflictsWhenAliasQuestionsDiffer(t *testing.T) {
	ctx := context.Background()
	base := newAssignmentMemoryStore()
	store := &blockingAssignmentStore{assignmentMemoryStore: base, entered: make(chan struct{}), release: make(chan struct{})}
	t.Cleanup(store.releaseNow)
	ledger := &recordingLedger{}
	a := NewServiceWithDeps(nil, store, ledger)
	b := NewServiceWithDeps(nil, store, ledger)
	for _, svc := range []*Service{a, b} {
		svc.LinkParentChild("par-1", stuUUID)
	}

	store.failNext = true
	if _, err := a.CreateParentCustomAssignment(ctx, "par-1", "Budi", stuUUID, "Drill", "", []string{"g1_add_within_20"}, "req-diff-race"); err == nil {
		t.Fatal("expected create to fail")
	}
	var localID string
	a.mu.Lock()
	for id, rec := range a.records {
		if rec.requestID == "req-diff-race" {
			localID = id
		}
	}
	a.mu.Unlock()
	local, err := a.GetDetail(ctx, stuUUID, localID, "en")
	if err != nil {
		t.Fatal(err)
	}
	prob, _ := mathengine.GenerateByCode(local.Questions[0].Question.TemplateCode, local.Questions[0].Question.Seed, "en")

	store.mu.Lock()
	store.armed = true
	store.mu.Unlock()
	result := make(chan error, 1)
	go func() {
		_, _, err := a.SubmitQuestion(ctx, stuUUID, localID, 1, prob.Solution.CorrectAnswers, "en")
		result <- err
	}()
	select {
	case <-store.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("SubmitQuestion did not reach persistence")
	}
	if _, err := b.CreateParentCustomAssignment(ctx, "par-1", "Budi", stuUUID, "Drill", "", []string{"g1_sub_within_20"}, "req-diff-race"); err != nil {
		t.Fatal(err)
	}
	store.releaseNow()
	select {
	case err := <-result:
		if !errors.Is(err, ErrSubmissionConflict) {
			t.Fatalf("expected a discarded-submission conflict, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("SubmitQuestion stalled after release")
	}
	if got := acCredited(ledger); got != 0 {
		t.Fatalf("discarded submission was rewarded: %d", got)
	}
}

func TestMergeAssignmentStateAliasKeepsDurableIdentity(t *testing.T) {
	current := assignmentState{Records: map[string]assignmentRecordDTO{
		"durable-id": {
			ID: "durable-id", StudentID: stuUUID, RequestID: "req-alias", Title: "Durable",
			Items: []itemSpecDTO{{TemplateCode: "g1_add_within_20", Seed: 11}},
		},
	}}
	raw, _ := json.Marshal(current)
	snapshot := assignmentState{Records: map[string]assignmentRecordDTO{
		"local-id": {
			ID: "local-id", StudentID: stuUUID, RequestID: "req-alias", Title: "Local",
			Items:     []itemSpecDTO{{TemplateCode: "g1_sub_within_20", Seed: 22}},
			Submitted: map[int]map[string]string{1: {"answer": "local"}},
			Correct:   map[int]bool{1: true},
			Completed: true,
		},
	}}
	merged, aliases, err := mergeAssignmentStateAliases(raw, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if aliases["local-id"] != "durable-id" {
		t.Fatalf("expected local alias to durable ID, got %+v", aliases)
	}
	rec, ok := merged.Records["durable-id"]
	if !ok {
		t.Fatal("durable record key was lost")
	}
	if rec.ID != "durable-id" || rec.Title != "Durable" ||
		rec.Items[0].TemplateCode != "g1_add_within_20" || rec.Items[0].Seed != 11 {
		t.Fatalf("alias merge replaced immutable assignment fields: %+v", rec)
	}
	if _, exists := merged.Records["local-id"]; exists {
		t.Fatal("local duplicate survived the alias merge")
	}
	if len(rec.Submitted) != 0 || len(rec.Correct) != 0 || rec.Completed {
		t.Fatalf("answers for different questions were copied onto the durable record: %+v", rec)
	}
}

// Two instances creating the same request: the second must answer with the durable record's ID
// (not its own local, never-stored ID) and must be able to open it.
func TestConcurrentCreateReturnsDurableID(t *testing.T) {
	ctx := context.Background()
	store := newAssignmentMemoryStore()
	a := NewServiceWithDeps(nil, store, nil)
	b := NewServiceWithDeps(nil, store, nil)
	for _, svc := range []*Service{a, b} {
		svc.LinkParentChild("par-1", stuUUID)
	}
	// Instance B loads the student before A's create lands.
	if _, err := b.GetDetail(ctx, stuUUID, "asgn-none", "en"); err == nil {
		t.Fatal("expected not found")
	}

	first, err := a.CreateParentCustomAssignment(ctx, "par-1", "Budi", stuUUID, "Drill", "", []string{"g1_add_within_20"}, "req-same")
	if err != nil {
		t.Fatal(err)
	}
	firstDetail, err := a.GetDetail(ctx, stuUUID, first.ID, "en")
	if err != nil {
		t.Fatal(err)
	}
	second, err := b.CreateParentCustomAssignment(ctx, "par-1", "Budi", stuUUID, "Drill", "", []string{"g1_add_within_20"}, "req-same")
	if err != nil {
		t.Fatal(err)
	}
	if second.ID != first.ID {
		t.Fatalf("instance B returned a non-durable ID: %s, durable is %s", second.ID, first.ID)
	}
	after, err := b.GetDetail(ctx, stuUUID, first.ID, "en")
	if err != nil {
		t.Fatalf("instance B cannot open the durable assignment: %v", err)
	}
	if after.Questions[0].Question.TemplateCode != firstDetail.Questions[0].Question.TemplateCode ||
		after.Questions[0].Question.Seed != firstDetail.Questions[0].Question.Seed {
		t.Fatalf("durable question changed after alias merge: before=%+v after=%+v", firstDetail.Questions[0].Question, after.Questions[0].Question)
	}
	fresh := NewServiceWithDeps(nil, store, nil)
	list, err := fresh.ListForStudent(ctx, stuUUID, "ALL", "ALL", 0)
	if err != nil || len(list) != 1 || list[0].ID != first.ID {
		t.Fatalf("durable state should hold exactly the one assignment %s: %+v err=%v", first.ID, list, err)
	}
}

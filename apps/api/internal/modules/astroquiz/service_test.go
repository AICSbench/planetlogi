package astroquiz

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sync"
	"testing"

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
	if _, err := svc.SavePaper(context.Background(), "stu-1", "G1-ESS-01", "en", -1, nil); err == nil {
		t.Fatal("expected negative elapsedSec to be rejected")
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

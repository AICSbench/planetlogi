package astroquiz

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/AICSbench/planetlogi/apps/api/internal/mathengine"
	"github.com/AICSbench/planetlogi/apps/api/internal/modules/auth"
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/economy"
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/reqguard/reqguardtest"
)

func newTestService() (*Service, *reqguardtest.FlakyLedger) {
	ledger := &reqguardtest.FlakyLedger{Inner: economy.NewMemoryLedger(economy.Wallet{StudentID: "stu-1", AstroCoins: 10, PilotLevel: 1})}
	return NewService(ledger), ledger
}

func correctDrafts(t *testing.T, view PaperAttemptView) map[int]map[string]string {
	t.Helper()
	drafts := map[int]map[string]string{}
	for _, pq := range view.Questions {
		prob, ok := mathengine.GenerateByCode(pq.Question.TemplateCode, pq.Question.Seed, "en")
		if !ok {
			t.Fatalf("unknown template %s", pq.Question.TemplateCode)
		}
		drafts[pq.Index] = prob.Solution.CorrectAnswers
	}
	return drafts
}

func TestAllPaperTemplatesResolve(t *testing.T) {
	for _, p := range buildPapers() {
		for _, tmpl := range p.templates {
			if _, ok := mathengine.GenerateByCode(tmpl, 1, "en"); !ok {
				t.Fatalf("paper %s references unknown template %q", p.code, tmpl)
			}
		}
	}
}

func TestAstroQuizHiddenSolutionsAndWeeklySupplyPodCap(t *testing.T) {
	svc, ledger := newTestService()

	papers, claimed := svc.ListPapers("stu-1", 0, "ALL", "en")
	if len(papers) != 18 || claimed {
		t.Fatalf("expected 18 papers across 3 categories x 6 grades and unclaimed pod, got %d / %v", len(papers), claimed)
	}

	// Open paper -> solutions MUST stay hidden for every question before full submit
	opened, err := svc.OpenPaper("stu-1", "G1-ESS-01", "en")
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range opened.Questions {
		if q.WorkedSolution != nil || q.IsCorrect != nil {
			t.Fatalf("Q%d: solutions must remain hidden while IN_PROGRESS", q.Index)
		}
	}

	sub1, err := svc.SubmitPaper(context.Background(), "stu-1", "G1-ESS-01", "en", 120, correctDrafts(t, opened))
	if err != nil {
		t.Fatal(err)
	}
	if sub1.Score != 10 || !sub1.SupplyPodAwarded {
		t.Fatalf("expected 10/10 and Supply Pod awarded: %+v", sub1)
	}
	for _, q := range sub1.Questions {
		if q.WorkedSolution == nil {
			t.Fatalf("Q%d: solution not revealed after submit", q.Index)
		}
	}

	// Submitting a 2nd paper in the same ISO week MUST NOT award a 2nd Supply Pod
	sub2, err := svc.SubmitPaper(context.Background(), "stu-1", "G1-TOP-01", "en", 90, map[int]map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	if sub2.SupplyPodAwarded {
		t.Fatalf("weekly Supply Pod cap (1/week) violated")
	}
	if got := ledger.Credited("stu-1", economy.CurrencyAC); got != WeeklySupplyPodRewardAC {
		t.Fatalf("expected exactly +%d AC, got %d", WeeklySupplyPodRewardAC, got)
	}
}

func TestFailedSupplyPodCreditIsRetried(t *testing.T) {
	svc, ledger := newTestService()
	ledger.Fail = true
	if _, err := svc.SubmitPaper(context.Background(), "stu-1", "G1-ESS-01", "en", 60, nil); !errors.Is(err, ErrRewardUnavailable) {
		t.Fatalf("expected ErrRewardUnavailable, got %v", err)
	}
	if _, claimed := svc.ListPapers("stu-1", 1, "ALL", "en"); claimed {
		t.Fatal("week must not be marked claimed while the credit failed")
	}

	// A different paper cannot grab a second claim while the first one is pending.
	ledger.Fail = false
	other, err := svc.SubmitPaper(context.Background(), "stu-1", "G1-TOP-01", "en", 60, nil)
	if err != nil || other.SupplyPodAwarded {
		t.Fatalf("second paper must not claim the pending pod: %+v err=%v", other.SupplyPodAwarded, err)
	}

	// Re-submitting the original paper retries the credit.
	retried, err := svc.SubmitPaper(context.Background(), "stu-1", "G1-ESS-01", "en", 60, nil)
	if err != nil || !retried.SupplyPodAwarded || !retried.SupplyPodClaimedWk {
		t.Fatalf("retry did not award the pod: %+v err=%v", retried, err)
	}
	if got := ledger.Credited("stu-1", economy.CurrencyAC); got != WeeklySupplyPodRewardAC {
		t.Fatalf("expected exactly +%d AC after retry, got %d", WeeklySupplyPodRewardAC, got)
	}
}

func TestDraftsAreBounded(t *testing.T) {
	svc, _ := newTestService()
	cases := map[string]struct {
		elapsed int
		drafts  map[int]map[string]string
	}{
		"index out of range": {10, map[int]map[string]string{999: {"answer": "1"}}},
		"index zero":         {10, map[int]map[string]string{0: {"answer": "1"}}},
		"negative elapsed":   {-1, nil},
		"oversized answer":   {10, map[int]map[string]string{1: {"answer": strings.Repeat("9", 65)}}},
	}
	for name, tc := range cases {
		if _, err := svc.SavePaper("stu-1", "G1-ESS-01", "en", tc.elapsed, tc.drafts); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("%s: expected ErrInvalidInput from save, got %v", name, err)
		}
	}
}

func TestPrintAnswerKeysFollowSubmissionGate(t *testing.T) {
	svc, _ := newTestService()
	stu := auth.Principal{ID: "stu-1", Role: auth.RoleStudent}

	if svc.CanSeeAnswerKeys(stu, "G1-ESS-01") {
		t.Fatal("student must not see answer keys before submitting")
	}
	if _, err := svc.OpenPaper("stu-1", "G1-ESS-01", "en"); err != nil {
		t.Fatal(err)
	}
	if svc.CanSeeAnswerKeys(stu, "G1-ESS-01") {
		t.Fatal("student must not see answer keys while IN_PROGRESS")
	}
	blank, err := svc.GetPrintablePaper("G1-ESS-01", "en", false)
	if err != nil || blank.AnswerKeyAvailable || blank.AnswerKeys != nil || len(blank.Questions) != paperQuestionCount {
		t.Fatalf("blank worksheet leaked keys: %+v err=%v", blank, err)
	}

	if _, err := svc.SubmitPaper(context.Background(), "stu-1", "G1-ESS-01", "en", 10, nil); err != nil {
		t.Fatal(err)
	}
	if !svc.CanSeeAnswerKeys(stu, "G1-ESS-01") {
		t.Fatal("student should see keys after submitting")
	}
	if svc.CanSeeAnswerKeys(stu, "G1-TOP-01") {
		t.Fatal("submitting one paper must not unlock keys for another")
	}
	if !svc.CanSeeAnswerKeys(auth.Principal{ID: "par-1", Role: auth.RoleParent}, "G1-TOP-01") {
		t.Fatal("parents may print answer keys for marking")
	}
	if svc.CanSeeAnswerKeys(auth.Principal{ID: "demo-pilot1", Role: auth.RoleStudent}, "G1-ESS-01") {
		t.Fatal("the guest identity must never see keys")
	}
}

func TestHTTPPrintAndAuth(t *testing.T) {
	authFx := reqguardtest.New(t,
		auth.Principal{ID: "stu-1", Role: auth.RoleStudent},
		auth.Principal{ID: "par-1", Role: auth.RoleParent},
	)
	svc, _ := newTestService()
	_, api := humatest.New(t)
	NewHandler(svc, authFx.Service).RegisterRoutes(nil, api)

	if resp := api.Get("/api/v1/astro-quiz/papers/G1-ESS-01/print"); resp.Code != http.StatusOK || strings.Contains(resp.Body.String(), "answerKeys") {
		t.Fatalf("anonymous print must be a blank worksheet: %d %s", resp.Code, resp.Body.String())
	}
	if resp := api.Get("/api/v1/astro-quiz/papers/G1-ESS-01/print", authFx.BearerHeader(t, "stu-1")); strings.Contains(resp.Body.String(), "answerKeys") {
		t.Fatal("student print leaked answer keys before submitting")
	}
	if resp := api.Get("/api/v1/astro-quiz/papers/G1-ESS-01/print", authFx.BearerHeader(t, "par-1")); !strings.Contains(resp.Body.String(), "answerKeys") {
		t.Fatal("parent print should include answer keys")
	}

	if resp := api.Post("/api/v1/astro-quiz/papers/G1-ESS-01/submit", map[string]any{"elapsedSec": 5, "drafts": map[string]any{}}); resp.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous submit: expected 401, got %d", resp.Code)
	}
	if resp := api.Post("/api/v1/astro-quiz/papers/G1-ESS-01/submit", authFx.BearerHeader(t, "par-1"), map[string]any{"elapsedSec": 5, "drafts": map[string]any{}}); resp.Code != http.StatusForbidden {
		t.Fatalf("parent submit: expected 403, got %d", resp.Code)
	}
	if resp := api.Post("/api/v1/astro-quiz/papers/G1-ESS-01/submit?locale=id", authFx.BearerHeader(t, "stu-1"), map[string]any{"elapsedSec": 5, "drafts": map[string]any{}}); resp.Code != http.StatusOK {
		t.Fatalf("student submit: expected 200, got %d %s", resp.Code, resp.Body.String())
	}
	if resp := api.Get("/api/v1/astro-quiz/papers/G1-ESS-01/print?locale=id", authFx.BearerHeader(t, "stu-1")); !strings.Contains(resp.Body.String(), "answerKeys") || !strings.Contains(resp.Body.String(), "Kelas 1") {
		t.Fatalf("submitted student should get Indonesian keys: %s", resp.Body.String())
	}
}

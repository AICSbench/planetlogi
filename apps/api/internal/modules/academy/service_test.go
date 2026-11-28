package academy

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

func TestSpaceAcademySaveTryAgainAndStarScroll(t *testing.T) {
	ledger := economy.NewMemoryLedger(economy.Wallet{StudentID: "stu-1", PilotLevel: 1})
	svc := NewService(ledger)

	topics := svc.ListTopics("stu-1", 0, "en")
	if len(topics) < 12 {
		t.Fatalf("expected >=12 topics across G1-G6, got %d", len(topics))
	}

	// Save draft persists across reloads
	if _, err := svc.SaveDraft("stu-1", "g1-wn-l1", "en", 1, map[string]string{"answer": "9"}); err != nil {
		t.Fatal(err)
	}
	reloaded, err := svc.GetLesson("stu-1", "g1-wn-l1", "en")
	if err != nil || reloaded.DoAndLearn[0].DraftAnswer["answer"] != "9" {
		t.Fatalf("draft did not persist: err=%v draft=%v", err, reloaded.DoAndLearn[0].DraftAnswer)
	}

	// Wrong check marks LastWrong=true and allows Try Again!
	wrongRes, afterWrong, err := svc.CheckAnswer(context.Background(), "stu-1", "g1-wn-l1", "en", 1, map[string]string{"answer": "-999"})
	if err != nil || wrongRes.IsCorrect || !afterWrong.DoAndLearn[0].LastWrong {
		t.Fatalf("expected wrong check to allow Try Again, got res=%+v", wrongRes)
	}

	// Solving all 4 questions awards exactly 1 Star Scroll
	for idx := 1; idx <= 4; idx++ {
		q := afterWrong.DoAndLearn[idx-1].Question
		prob, _ := mathengine.GenerateByCode(q.TemplateCode, q.Seed, "en")
		if _, afterWrong, err = svc.CheckAnswer(context.Background(), "stu-1", "g1-wn-l1", "en", idx, prob.Solution.CorrectAnswers); err != nil {
			t.Fatalf("check q%d failed: %v", idx, err)
		}
	}

	if !afterWrong.ScrollEarned {
		t.Fatalf("expected Star Scroll awarded after solving 4/4 Do & Learn questions")
	}

	// Toggle Self-Check pushes completion to 100%
	_, _ = svc.ToggleSelfCheck("stu-1", "g1-wn-l1", "count_20", true, "en")
	final, _ := svc.ToggleSelfCheck("stu-1", "g1-wn-l1", "numline_add", true, "en")
	if final.CompletionPct != 100 {
		t.Fatalf("expected 100%% completion, got %d%%", final.CompletionPct)
	}
}

func TestAllLessonTemplatesResolve(t *testing.T) {
	_, lessons := buildCatalog()
	for _, l := range lessons {
		if _, ok := mathengine.GenerateByCode(l.templateCode, 1, "en"); !ok {
			t.Fatalf("lesson %s references unknown template %q", l.code, l.templateCode)
		}
	}
}

func solveAll(t *testing.T, svc *Service, studentID, lesson string) (LessonDetail, error) {
	t.Helper()
	detail, err := svc.GetLesson(studentID, lesson, "en")
	if err != nil {
		t.Fatal(err)
	}
	var lastErr error
	for idx := 1; idx <= doLearnQuestionCount; idx++ {
		q := detail.DoAndLearn[idx-1].Question
		prob, _ := mathengine.GenerateByCode(q.TemplateCode, q.Seed, "en")
		var d LessonDetail
		_, d, lastErr = svc.CheckAnswer(context.Background(), studentID, lesson, "en", idx, prob.Solution.CorrectAnswers)
		if lastErr == nil {
			detail = d
		}
	}
	return detail, lastErr
}

func TestFailedStarScrollCreditIsRetried(t *testing.T) {
	ledger := &reqguardtest.FlakyLedger{Inner: economy.NewMemoryLedger(economy.Wallet{StudentID: "stu-1", PilotLevel: 1})}
	svc := NewService(ledger)

	ledger.Fail = true
	if _, err := solveAll(t, svc, "stu-1", "g1-wn-l1"); !errors.Is(err, ErrRewardUnavailable) {
		t.Fatalf("expected ErrRewardUnavailable on the 4th solve, got %v", err)
	}
	if d, _ := svc.GetLesson("stu-1", "g1-wn-l1", "en"); d.ScrollEarned {
		t.Fatal("scroll must not be marked earned when the credit failed")
	}

	ledger.Fail = false
	if _, err := solveAll(t, svc, "stu-1", "g1-wn-l1"); err != nil {
		t.Fatalf("retry failed: %v", err)
	}
	if got := ledger.Credited("stu-1", economy.CurrencyScroll); got != 1 {
		t.Fatalf("expected exactly 1 Star Scroll, got %d", got)
	}
	_, _ = solveAll(t, svc, "stu-1", "g1-wn-l1")
	if got := ledger.Credited("stu-1", economy.CurrencyScroll); got != 1 {
		t.Fatalf("Star Scroll credited twice: %d", got)
	}
}

func TestInputBoundsAndReadsDoNotAllocate(t *testing.T) {
	svc := NewService(nil)

	_ = svc.ListTopics("reader", 0, "en")
	if _, err := svc.GetLesson("reader", "g1-wn-l1", "en"); err != nil {
		t.Fatal(err)
	}
	if len(svc.progress) != 0 {
		t.Fatalf("read-only calls must not allocate progress, have %d entries", len(svc.progress))
	}

	if _, err := svc.ToggleSelfCheck("stu-1", "g1-wn-l1", "not-a-real-item", true, "en"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("unknown self-check key: expected ErrInvalidInput, got %v", err)
	}
	if _, err := svc.SaveDraft("stu-1", "g1-wn-l1", "en", 5, nil); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("question index 5: expected ErrInvalidInput, got %v", err)
	}
	if _, err := svc.SaveDraft("stu-1", "g1-wn-l1", "en", 1, map[string]string{"answer": strings.Repeat("9", 65)}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("oversized answer: expected ErrInvalidInput, got %v", err)
	}
	if _, err := svc.GetLesson("stu-1", "nope", "en"); !errors.Is(err, ErrLessonNotFound) {
		t.Fatalf("expected ErrLessonNotFound, got %v", err)
	}
}

func TestHTTPAuthAndLocaleBinding(t *testing.T) {
	authFx := reqguardtest.New(t,
		auth.Principal{ID: "stu-1", Role: auth.RoleStudent},
		auth.Principal{ID: "par-1", Role: auth.RoleParent},
	)
	svc := NewService(nil)
	_, api := humatest.New(t)
	NewHandler(svc, authFx.Service).RegisterRoutes(nil, api)

	// Locale is bound again: Indonesian titles are served.
	if resp := api.Get("/api/v1/academy/topics?grade=1&locale=id"); !strings.Contains(resp.Body.String(), "Bilangan Cacah") {
		t.Fatalf("locale=id not honoured: %s", resp.Body.String())
	}

	body := map[string]any{"lessonCode": "g1-wn-l1", "questionIndex": 1, "answers": map[string]string{"answer": "3"}}
	if resp := api.Post("/api/v1/academy/do-learn/save", body); resp.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous save: expected 401, got %d", resp.Code)
	}
	if resp := api.Post("/api/v1/academy/do-learn/save", authFx.BearerHeader(t, "par-1"), body); resp.Code != http.StatusForbidden {
		t.Fatalf("parent save: expected 403, got %d", resp.Code)
	}
	if resp := api.Post("/api/v1/academy/do-learn/save", authFx.BearerHeader(t, "stu-1"), body); resp.Code != http.StatusOK {
		t.Fatalf("student save: expected 200, got %d %s", resp.Code, resp.Body.String())
	}
	// The authenticated student's draft is stored under their own ID, not the shared guest.
	if d, _ := svc.GetLesson("stu-1", "g1-wn-l1", "en"); d.DoAndLearn[0].DraftAnswer["answer"] != "3" {
		t.Fatal("draft was not stored for the authenticated student")
	}
	if d, _ := svc.GetLesson("demo-pilot1", "g1-wn-l1", "en"); d.DoAndLearn[0].DraftAnswer != nil {
		t.Fatal("draft leaked into the guest identity")
	}
}

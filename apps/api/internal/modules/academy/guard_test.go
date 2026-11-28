package academy

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/AICSbench/planetlogi/apps/api/internal/modules/auth"
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/reqguard"
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/reqguard/reqguardtest"
)

func TestDoLearnAnswersAreBounded(t *testing.T) {
	svc := NewService(nil)
	huge := map[string]string{"answer": strings.Repeat("9", 1000)}
	if _, err := svc.SaveDraft(context.Background(), "stu-1", "g1-wn-l1", "en", 1, huge); !errors.Is(err, reqguard.ErrInvalidAnswers) {
		t.Fatalf("save: expected ErrInvalidAnswers, got %v", err)
	}
	if _, _, err := svc.CheckAnswer(context.Background(), "stu-1", "g1-wn-l1", "en", 1, huge); !errors.Is(err, reqguard.ErrInvalidAnswers) {
		t.Fatalf("check: expected ErrInvalidAnswers, got %v", err)
	}
}

func TestHTTPProgressWritesAreStudentOnly(t *testing.T) {
	fx := reqguardtest.New(t,
		auth.Principal{ID: "stu-1", Role: auth.RoleStudent},
		auth.Principal{ID: "par-1", Role: auth.RoleParent},
	)
	_, api := humatest.New(t)
	NewHandler(NewService(nil), fx.Service).RegisterRoutes(nil, api)
	body := map[string]any{"lessonCode": "g1-wn-l1", "questionIndex": 1, "answers": map[string]string{"answer": "3"}}

	if resp := api.Post("/api/v1/academy/do-learn/save", fx.BearerHeader(t, "par-1"), body); resp.Code != http.StatusForbidden {
		t.Fatalf("parent save: expected 403, got %d", resp.Code)
	}
	if resp := api.Post("/api/v1/academy/do-learn/save", fx.BearerHeader(t, "stu-1"), body); resp.Code != http.StatusOK {
		t.Fatalf("student save: expected 200, got %d", resp.Code)
	}
	if resp := api.Get("/api/v1/academy/topics?grade=1", fx.BearerHeader(t, "par-1")); resp.Code != http.StatusOK {
		t.Fatalf("parent may still browse topics, got %d", resp.Code)
	}
	if resp := api.Get("/api/v1/academy/lessons/nope", fx.BearerHeader(t, "stu-1")); resp.Code != http.StatusNotFound {
		t.Fatalf("unknown lesson: expected 404, got %d", resp.Code)
	}
	bad := map[string]any{"lessonCode": "g1-wn-l1", "itemKey": "nope", "checked": true}
	if resp := api.Patch("/api/v1/academy/self-check", fx.BearerHeader(t, "stu-1"), bad); resp.Code != http.StatusBadRequest {
		t.Fatalf("unknown self-check item: expected 400, got %d", resp.Code)
	}
}

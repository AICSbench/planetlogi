package assignments

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

// Builder payloads are persisted and re-rendered on every read, so they must be bounded.
func TestCustomAssignmentPayloadBounds(t *testing.T) {
	fx := reqguardtest.New(t,
		auth.Principal{ID: "par-1", Role: auth.RoleParent, DisplayName: "Budi"},
		auth.Principal{ID: "stu-1", Role: auth.RoleStudent},
	)
	svc := NewService(nil)
	svc.LinkParentChild("par-1", "stu-1")
	_, api := humatest.New(t)
	NewHandler(svc, fx.Service).RegisterRoutes(nil, api)
	parent := fx.BearerHeader(t, "par-1")

	many := make([]string, MaxTemplates+1)
	for i := range many {
		many[i] = "g1_add_within_20"
	}
	for name, body := range map[string]map[string]any{
		"too many templates": {"studentId": "stu-1", "title": "Drill", "templateCodes": many},
		"oversized title":    {"studentId": "stu-1", "title": strings.Repeat("x", MaxTitleLen+1), "templateCodes": []string{"g1_add_within_20"}},
	} {
		if resp := api.Post("/api/v1/assignments/custom", parent, body); resp.Code < 400 || resp.Code >= 500 {
			t.Fatalf("%s: expected a 4xx rejection, got %d", name, resp.Code)
		}
	}

	ctx := context.Background()
	if _, err := svc.CreateParentCustomAssignment(ctx, "par-1", "Budi", "stu-1", strings.Repeat("x", MaxTitleLen+1), "", []string{"g1_add_within_20"}); err == nil {
		t.Fatal("service accepted an oversized title")
	}
	if _, err := svc.CreateParentCustomAssignment(ctx, "par-1", "Budi", "stu-1", "Drill", "", many); err == nil {
		t.Fatal("service accepted too many templates")
	}

	created, err := svc.CreateParentCustomAssignment(ctx, "par-1", "Budi", "stu-1", "Drill", "", []string{"g1_add_within_20"})
	if err != nil {
		t.Fatal(err)
	}
	huge := map[string]any{"questionIndex": 1, "answers": map[string]string{"answer": strings.Repeat("9", 100000)}}
	if resp := api.Post("/api/v1/assignments/"+created.ID+"/submit", fx.BearerHeader(t, "stu-1"), huge); resp.Code != http.StatusBadRequest {
		t.Fatalf("oversized answer: expected 400, got %d", resp.Code)
	}
	if _, _, err := svc.SubmitQuestion(ctx, "stu-1", created.ID, 1, map[string]string{"answer": strings.Repeat("9", 1000)}, "en"); !errors.Is(err, reqguard.ErrInvalidAnswers) {
		t.Fatalf("expected ErrInvalidAnswers, got %v", err)
	}
}

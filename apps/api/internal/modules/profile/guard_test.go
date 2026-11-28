package profile

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/AICSbench/planetlogi/apps/api/internal/modules/auth"
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/reqguard/reqguardtest"
)

// Family settings and the parent overview are parent-sensitive (AGENTS.md guardrail #6):
// a student session must not change screen-time limits or read the parent dashboard.
func TestHTTPParentOnlyEndpoints(t *testing.T) {
	fx := reqguardtest.New(t,
		auth.Principal{ID: "stu-1", Role: auth.RoleStudent},
		auth.Principal{ID: "demo-parent", Role: auth.RoleParent, DisplayName: "Budi"},
	)
	svc := NewService(nil, nil, fx.Service, false)
	_, api := humatest.New(t)
	NewHandler(svc, fx.Service).RegisterRoutes(nil, api)
	student, parent := fx.BearerHeader(t, "stu-1"), fx.BearerHeader(t, "demo-parent")

	patch := map[string]any{"brainGalaxyDailyCapMin": 25}
	if resp := api.Patch("/api/v1/profile/settings", patch); resp.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous settings: expected 401, got %d", resp.Code)
	}
	if resp := api.Patch("/api/v1/profile/settings", student, patch); resp.Code != http.StatusForbidden {
		t.Fatalf("student settings: expected 403, got %d", resp.Code)
	}
	if resp := api.Patch("/api/v1/profile/settings", parent, patch); resp.Code != http.StatusOK {
		t.Fatalf("parent settings: expected 200, got %d %s", resp.Code, resp.Body.String())
	}
	if resp := api.Patch("/api/v1/profile/settings", parent, map[string]any{"brainGalaxyWindowStart": "garbage"}); resp.Code != http.StatusBadRequest {
		t.Fatalf("invalid window time: expected 400, got %d", resp.Code)
	}

	if resp := api.Get("/api/v1/parent/overview", student); resp.Code != http.StatusForbidden {
		t.Fatalf("student overview: expected 403, got %d", resp.Code)
	}
	if resp := api.Get("/api/v1/parent/overview", parent); resp.Code != http.StatusOK {
		t.Fatalf("parent overview: expected 200, got %d %s", resp.Code, resp.Body.String())
	}
}

func TestSettingsValidationAndFailClosedOTP(t *testing.T) {
	svc := NewService(nil, nil, nil, false)
	parent := auth.Principal{ID: "demo-parent", Role: auth.RoleParent}

	for name, in := range map[string]UpdateSettingsInput{
		"bad language":     {Language: "fr"},
		"bad window time":  {BrainGalaxyWindowEnd: "25:99"},
		"too many curric.": {DailyOrbitCurricula: []string{"a", "b", "c", "d", "e", "f", "g", "h", "i"}},
		"long curriculum":  {StarDuelCurricula: []string{strings.Repeat("x", 33)}},
	} {
		if _, err := svc.UpdateSettings(context.Background(), parent, in); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("%s: expected ErrInvalidInput, got %v", name, err)
		}
	}

	enable := true
	if _, err := svc.UpdateSettings(context.Background(), parent, UpdateSettingsInput{OTPEnabled: &enable}); err != nil {
		t.Fatal(err)
	}
	// Without an OTP verifier any code must be rejected (previously any non-empty code passed).
	if _, err := svc.UpdateSettings(context.Background(), parent, UpdateSettingsInput{OTPCode: "000000", Language: "id"}); !errors.Is(err, ErrOTPRequired) {
		t.Fatalf("expected fail-closed ErrOTPRequired, got %v", err)
	}
}

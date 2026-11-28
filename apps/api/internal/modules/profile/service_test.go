package profile

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/AICSbench/planetlogi/apps/api/internal/config"
	"github.com/AICSbench/planetlogi/apps/api/internal/modules/auth"
	"github.com/AICSbench/planetlogi/apps/api/internal/modules/reports"
	"github.com/AICSbench/planetlogi/apps/api/internal/modules/streak"
	"github.com/AICSbench/planetlogi/apps/api/internal/modules/training"
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/reqguard/reqguardtest"
)

func TestProfileSettingsOTPGateAndScreenTimeCap(t *testing.T) {
	cfg := config.Load()
	cfg.AppEnv = "development"
	cfg.OTPDevEcho = true
	authSvc := auth.NewService(cfg, nil, nil, nil)
	trn := training.NewService(nil)
	rep := reports.NewHandler(trn, nil)
	str := streak.NewService()
	svc := NewService(rep, str, authSvc, false)

	parent := auth.Principal{ID: "par-1", Role: auth.RoleParent, DisplayName: "Budi"}

	// Enable OTP and test Brain Galaxy cap clamped to max 25 min
	enable := true
	overCap := 60
	updated, err := svc.UpdateSettings(context.Background(), "stu-1", parent, UpdateSettingsInput{
		OTPEnabled:             &enable,
		Language:               "id",
		BrainGalaxyDailyCapMin: &overCap,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !updated.Settings.OTPEnabled || updated.Settings.Language != "id" || updated.Settings.BrainGalaxyDailyCapMin != 25 {
		t.Fatalf("unexpected settings after update: %+v", updated.Settings)
	}

	// Subsequent update WITHOUT OTP MUST fail with ErrOTPRequired
	if _, err := svc.UpdateSettings(context.Background(), "stu-1", parent, UpdateSettingsInput{Language: "en"}); err != ErrOTPRequired {
		t.Fatalf("expected ErrOTPRequired, got %v", err)
	}

	// Request real Parent OTP and submit it -> update succeeds
	code, err := authSvc.RequestParentOTP(context.Background(), parent)
	if err != nil {
		t.Fatal(err)
	}
	afterOTP, err := svc.UpdateSettings(context.Background(), "stu-1", parent, UpdateSettingsInput{
		OTPCode:  code,
		Language: "en",
	})
	if err != nil || afterOTP.Settings.Language != "en" {
		t.Fatalf("expected OTP-verified update to succeed: err=%v lang=%s", err, afterOTP.Settings.Language)
	}
}

func TestSettingsValidationAndFailClosedOTP(t *testing.T) {
	svc := NewService(nil, nil, nil, false)
	parent := auth.Principal{ID: "par-1", Role: auth.RoleParent}

	for name, in := range map[string]UpdateSettingsInput{
		"bad language":     {Language: "fr"},
		"bad window time":  {BrainGalaxyWindowStart: "25:99"},
		"too many curric.": {DailyOrbitCurricula: []string{"a", "b", "c", "d", "e", "f", "g", "h", "i"}},
		"long curriculum":  {StarDuelCurricula: []string{strings.Repeat("x", 33)}},
	} {
		if _, err := svc.UpdateSettings(context.Background(), "par-1", parent, in); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("%s: expected ErrInvalidInput, got %v", name, err)
		}
	}

	enable := true
	if _, err := svc.UpdateSettings(context.Background(), "par-1", parent, UpdateSettingsInput{OTPEnabled: &enable}); err != nil {
		t.Fatal(err)
	}
	// Without an OTP verifier any code must be rejected (previously any non-empty code passed).
	if _, err := svc.UpdateSettings(context.Background(), "par-1", parent, UpdateSettingsInput{OTPCode: "000000", Language: "id"}); !errors.Is(err, ErrOTPRequired) {
		t.Fatalf("expected fail-closed ErrOTPRequired, got %v", err)
	}
}

func TestHTTPParentOnlyEndpoints(t *testing.T) {
	authFx := reqguardtest.New(t,
		auth.Principal{ID: "stu-1", Role: auth.RoleStudent},
		auth.Principal{ID: "par-1", Role: auth.RoleParent, DisplayName: "Budi"},
	)
	svc := NewService(nil, nil, authFx.Service, false)
	_, api := humatest.New(t)
	NewHandler(svc, authFx.Service).RegisterRoutes(nil, api)

	patch := map[string]any{"brainGalaxyDailyCapMin": 0}
	if resp := api.Patch("/api/v1/profile/settings", patch); resp.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous settings: expected 401, got %d", resp.Code)
	}
	if resp := api.Patch("/api/v1/profile/settings", authFx.BearerHeader(t, "stu-1"), patch); resp.Code != http.StatusForbidden {
		t.Fatalf("student settings: expected 403, got %d", resp.Code)
	}
	if resp := api.Patch("/api/v1/profile/settings", authFx.BearerHeader(t, "par-1"), patch); resp.Code != http.StatusOK {
		t.Fatalf("parent settings: expected 200, got %d %s", resp.Code, resp.Body.String())
	}

	if resp := api.Get("/api/v1/parent/overview"); resp.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous overview: expected 401, got %d", resp.Code)
	}
	if resp := api.Get("/api/v1/parent/overview", authFx.BearerHeader(t, "stu-1")); resp.Code != http.StatusForbidden {
		t.Fatalf("student overview: expected 403, got %d", resp.Code)
	}
	if resp := api.Get("/api/v1/parent/overview", authFx.BearerHeader(t, "par-1")); resp.Code != http.StatusOK {
		t.Fatalf("parent overview: expected 200, got %d %s", resp.Code, resp.Body.String())
	}
}

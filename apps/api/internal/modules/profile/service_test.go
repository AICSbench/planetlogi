package profile

import (
	"context"
	"testing"

	"github.com/AICSbench/planetlogi/apps/api/internal/config"
	"github.com/AICSbench/planetlogi/apps/api/internal/modules/auth"
	"github.com/AICSbench/planetlogi/apps/api/internal/modules/reports"
	"github.com/AICSbench/planetlogi/apps/api/internal/modules/streak"
	"github.com/AICSbench/planetlogi/apps/api/internal/modules/training"
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
	updated, err := svc.UpdateSettings(context.Background(), parent, UpdateSettingsInput{
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
	if _, err := svc.UpdateSettings(context.Background(), parent, UpdateSettingsInput{Language: "en"}); err != ErrOTPRequired {
		t.Fatalf("expected ErrOTPRequired, got %v", err)
	}

	// Request real Parent OTP and submit it -> update succeeds
	code, err := authSvc.RequestParentOTP(context.Background(), parent)
	if err != nil {
		t.Fatal(err)
	}
	afterOTP, err := svc.UpdateSettings(context.Background(), parent, UpdateSettingsInput{
		OTPCode:  code,
		Language: "en",
	})
	if err != nil || afterOTP.Settings.Language != "en" {
		t.Fatalf("expected OTP-verified update to succeed: err=%v lang=%s", err, afterOTP.Settings.Language)
	}
}

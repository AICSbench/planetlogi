package profile

import (
	"context"
	"errors"
	"sync"
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

	parent := auth.Principal{ID: "demo-parent", Role: auth.RoleParent, DisplayName: "Budi"}

	// Enabling OTP without a verified code is refused, so a parent who cannot receive codes
	// can never lock themselves out of their settings.
	enable := true
	overCap := 60
	if _, err := svc.UpdateSettings(context.Background(), parent, UpdateSettingsInput{OTPEnabled: &enable}); err != ErrOTPRequired {
		t.Fatalf("enabling OTP without a code: expected ErrOTPRequired, got %v", err)
	}

	// Enable OTP with a verified code and test Brain Galaxy cap clamped to max 25 min
	enrollCode, err := authSvc.RequestParentOTP(context.Background(), parent)
	if err != nil {
		t.Fatal(err)
	}
	updated, err := svc.UpdateSettings(context.Background(), parent, UpdateSettingsInput{
		OTPEnabled:             &enable,
		OTPCode:                enrollCode,
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

func TestUpdateSettingsPreservesIndependentPatches(t *testing.T) {
	svc := NewService(nil, nil, nil, false)
	parent := auth.Principal{ID: "demo-parent", Role: auth.RoleParent}
	ctx := context.Background()

	if _, err := svc.UpdateSettings(ctx, parent, UpdateSettingsInput{Language: "id"}); err != nil {
		t.Fatal(err)
	}
	capMin := 10
	if _, err := svc.UpdateSettings(ctx, parent, UpdateSettingsInput{BrainGalaxyDailyCapMin: &capMin}); err != nil {
		t.Fatal(err)
	}
	bundle, err := svc.GetBundle(ctx, parent)
	if err != nil {
		t.Fatal(err)
	}
	if bundle.Settings.Language != "id" || bundle.Settings.BrainGalaxyDailyCapMin != 10 {
		t.Fatalf("concurrent patches overwrote each other: %+v", bundle.Settings)
	}
}

func TestUpdateSettingsRejectsStudentRole(t *testing.T) {
	svc := NewService(nil, nil, nil, false)
	student := auth.Principal{ID: "stu-1", Role: auth.RoleStudent}
	if _, err := svc.UpdateSettings(context.Background(), student, UpdateSettingsInput{Language: "id"}); err != ErrParentOnly {
		t.Fatalf("expected ErrParentOnly, got %v", err)
	}
}

func TestUpdateSettingsOverlappingPatches(t *testing.T) {
	svc := NewService(nil, nil, nil, false)
	parent := auth.Principal{ID: "demo-parent", Role: auth.RoleParent}
	ctx := context.Background()
	capMin := 10

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		if _, err := svc.UpdateSettings(ctx, parent, UpdateSettingsInput{Language: "id"}); err != nil {
			t.Errorf("language patch: %v", err)
		}
	}()
	go func() {
		defer wg.Done()
		if _, err := svc.UpdateSettings(ctx, parent, UpdateSettingsInput{BrainGalaxyDailyCapMin: &capMin}); err != nil {
			t.Errorf("cap patch: %v", err)
		}
	}()
	wg.Wait()

	bundle, err := svc.GetBundle(ctx, parent)
	if err != nil {
		t.Fatal(err)
	}
	if bundle.Settings.Language != "id" || bundle.Settings.BrainGalaxyDailyCapMin != 10 {
		t.Fatalf("overlapping patches overwrote each other: %+v", bundle.Settings)
	}
}

func TestListChildrenDemoFallbackIsRestricted(t *testing.T) {
	svc := NewService(nil, nil, nil, false)
	children, err := svc.ListChildren(context.Background(), auth.Principal{ID: "demo-parent", Role: auth.RoleParent})
	if err != nil || len(children) != 1 || children[0].StudentID != "demo-pilot1" {
		t.Fatalf("demo parent children: %+v err=%v", children, err)
	}
	if _, err := svc.ListChildren(context.Background(), auth.Principal{ID: "other-parent", Role: auth.RoleParent}); !errors.Is(err, ErrNoLinkedChild) {
		t.Fatalf("unrelated parent should have no children, got %v", err)
	}
}

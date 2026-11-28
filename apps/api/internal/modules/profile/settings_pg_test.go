package profile

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AICSbench/planetlogi/apps/api/internal/config"
	"github.com/AICSbench/planetlogi/apps/api/internal/modules/auth"
)

// TestSettingsPGPartialUpdateAndOTPEnrollment needs a migrated + seeded database.
func TestSettingsPGPartialUpdateAndOTPEnrollment(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping PostgreSQL settings test")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	var parentID string
	if err := pool.QueryRow(ctx, `SELECT parent_account_id::text FROM student_profiles ORDER BY created_at, id LIMIT 1`).Scan(&parentID); err != nil {
		t.Skipf("no seeded student profile: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `UPDATE user_accounts SET otp_enabled = false WHERE id = $1::uuid`, parentID)
	})
	_, _ = pool.Exec(ctx, `UPDATE user_accounts SET otp_enabled = false WHERE id = $1::uuid`, parentID)

	cfg := config.Load()
	cfg.AppEnv = "development"
	cfg.OTPDevEcho = true
	authSvc := auth.NewService(cfg, nil, nil, nil)
	svc := NewServiceWithPG(pool, nil, nil, authSvc, false)
	parent := auth.Principal{ID: parentID, Role: auth.RoleParent}

	// Two partial patches: the second must not restore the first one's field.
	lang, capMin := "id", 15
	if _, err := svc.UpdateSettings(ctx, parent, UpdateSettingsInput{Language: lang}); err != nil {
		t.Fatal(err)
	}
	got, err := svc.UpdateSettings(ctx, parent, UpdateSettingsInput{BrainGalaxyDailyCapMin: &capMin, BrainGalaxyWindowStart: "07:30"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Settings.Language != "id" || got.Settings.BrainGalaxyDailyCapMin != 15 || got.Settings.BrainGalaxyWindowStart != "07:30" {
		t.Fatalf("partial updates clobbered each other: %+v", got.Settings)
	}

	// Enabling OTP needs a verified code.
	on := true
	if _, err := svc.UpdateSettings(ctx, parent, UpdateSettingsInput{OTPEnabled: &on}); !errors.Is(err, ErrOTPRequired) {
		t.Fatalf("enable without code: expected ErrOTPRequired, got %v", err)
	}
	code, err := authSvc.RequestParentOTP(ctx, parent)
	if err != nil {
		t.Fatal(err)
	}
	enabled, err := svc.UpdateSettings(ctx, parent, UpdateSettingsInput{OTPEnabled: &on, OTPCode: code})
	if err != nil || !enabled.Settings.OTPEnabled {
		t.Fatalf("verified enrollment failed: %+v err=%v", enabled.Settings, err)
	}
	if _, err := svc.UpdateSettings(ctx, parent, UpdateSettingsInput{Language: "en"}); !errors.Is(err, ErrOTPRequired) {
		t.Fatalf("update after enrollment without code: expected ErrOTPRequired, got %v", err)
	}
}

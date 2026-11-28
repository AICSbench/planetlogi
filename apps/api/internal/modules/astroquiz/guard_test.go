package astroquiz

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/AICSbench/planetlogi/apps/api/internal/modules/auth"

	"github.com/AICSbench/planetlogi/apps/api/internal/platform/reqguard"
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/reqguard/reqguardtest"
)

func TestPaperPayloadIsBounded(t *testing.T) {
	svc := NewService(nil)
	ctx := context.Background()
	huge := map[int]map[string]string{1: {"answer": strings.Repeat("9", 1000)}}
	if _, err := svc.SavePaper(ctx, "stu-1", "G1-ESS-01", "en", 10, huge); !errors.Is(err, reqguard.ErrInvalidAnswers) {
		t.Fatalf("save: expected ErrInvalidAnswers, got %v", err)
	}
	if _, err := svc.SubmitPaper(ctx, "stu-1", "G1-ESS-01", "en", 10, huge); !errors.Is(err, reqguard.ErrInvalidAnswers) {
		t.Fatalf("submit: expected ErrInvalidAnswers, got %v", err)
	}
}

func TestTimerIsMeasuredByServer(t *testing.T) {
	svc := NewService(nil)
	ctx := context.Background()
	now := time.Date(2026, 11, 30, 9, 0, 0, 0, time.UTC)
	svc.nowClock = func() time.Time { return now }

	if _, err := svc.OpenPaper(ctx, "stu-1", "G1-ESS-01", "en"); err != nil {
		t.Fatal(err)
	}
	now = now.Add(90 * time.Second)
	saved, err := svc.SavePaper(ctx, "stu-1", "G1-ESS-01", "en", 999999, nil)
	if err != nil || saved.ElapsedSec != 90 {
		t.Fatalf("expected 90s measured by the server (client sent 999999), got %d err=%v", saved.ElapsedSec, err)
	}

	// Time away is capped per gap, and re-opening starts a fresh window.
	now = now.Add(6 * time.Hour)
	saved, _ = svc.SavePaper(ctx, "stu-1", "G1-ESS-01", "en", 0, nil)
	if saved.ElapsedSec != 90+maxTickGapSec {
		t.Fatalf("idle gap must be capped at %ds, got total %d", maxTickGapSec, saved.ElapsedSec)
	}
	now = now.Add(48 * time.Hour)
	if _, err := svc.OpenPaper(ctx, "stu-1", "G1-ESS-01", "en"); err != nil {
		t.Fatal(err)
	}
	now = now.Add(30 * time.Second)
	sub, _ := svc.SubmitPaper(ctx, "stu-1", "G1-ESS-01", "en", 0, nil)
	if sub.ElapsedSec != 90+maxTickGapSec+30 {
		t.Fatalf("resume must not count time away, got %d", sub.ElapsedSec)
	}
	now = now.Add(time.Hour)
	again, _ := svc.SubmitPaper(ctx, "stu-1", "G1-ESS-01", "en", 0, nil)
	if again.ElapsedSec != sub.ElapsedSec {
		t.Fatal("a submitted paper's timer must stop")
	}
}

func TestSupplyPodWeekFollowsUserTimezone(t *testing.T) {
	svc := NewService(nil)
	// Sunday 23:30 UTC is already Monday (next ISO week) in Jakarta (UTC+7).
	svc.nowClock = func() time.Time { return time.Date(2026, 11, 29, 23, 30, 0, 0, time.UTC) }
	jkt := WithTimezone(context.Background(), "Asia/Jakarta")
	utc := WithTimezone(context.Background(), "UTC")
	if svc.currentWeekKey(jkt) == svc.currentWeekKey(utc) {
		t.Fatalf("week key ignored the timezone: %s", svc.currentWeekKey(jkt))
	}
	if svc.currentWeekKey(WithTimezone(context.Background(), "Not/AZone")) != svc.currentWeekKey(utc) {
		t.Fatal("an invalid timezone must fall back to UTC")
	}
}

func TestHTTPAttemptsAreStudentOnly(t *testing.T) {
	fx := reqguardtest.New(t,
		auth.Principal{ID: "stu-1", Role: auth.RoleStudent},
		auth.Principal{ID: "par-1", Role: auth.RoleParent},
	)
	_, api := humatest.New(t)
	NewHandler(NewService(nil), fx.Service).RegisterRoutes(nil, api)

	if resp := api.Get("/api/v1/astro-quiz/papers/G1-ESS-01", fx.BearerHeader(t, "par-1")); resp.Code != http.StatusForbidden {
		t.Fatalf("parent open: expected 403, got %d", resp.Code)
	}
	if resp := api.Get("/api/v1/astro-quiz/papers/G1-ESS-01/print?includeKey=true", fx.BearerHeader(t, "par-1")); resp.Code != http.StatusOK {
		t.Fatalf("parent print with key: expected 200, got %d", resp.Code)
	}
	if resp := api.Get("/api/v1/astro-quiz/papers/G1-ESS-01/print?includeKey=true", fx.BearerHeader(t, "stu-1")); resp.Code != http.StatusForbidden {
		t.Fatalf("student key before submit: expected 403, got %d", resp.Code)
	}
	if resp := api.Get("/api/v1/astro-quiz/papers/NOPE", fx.BearerHeader(t, "stu-1")); resp.Code != http.StatusNotFound {
		t.Fatalf("unknown paper: expected 404, got %d", resp.Code)
	}
}

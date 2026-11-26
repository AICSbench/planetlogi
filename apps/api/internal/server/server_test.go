package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AICSbench/planetlogi/apps/api/internal/config"
)

type stubChecker struct{ err error }

func (s stubChecker) Ping(_ context.Context) error { return s.err }

func TestHealthzAndReadyz(t *testing.T) {
	cfg := config.Load()

	t.Run("healthz returns 200", func(t *testing.T) {
		app := New(Dependencies{Config: cfg})
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		rec := httptest.NewRecorder()
		app.Router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("readyz returns 200 when db and redis healthy", func(t *testing.T) {
		app := New(Dependencies{
			Config:     cfg,
			DBChecker:  stubChecker{},
			RedisCheck: stubChecker{},
		})
		req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
		rec := httptest.NewRecorder()
		app.Router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("readyz returns 503 when postgres fails", func(t *testing.T) {
		app := New(Dependencies{
			Config:     cfg,
			DBChecker:  stubChecker{err: errors.New("connection refused")},
			RedisCheck: stubChecker{},
		})
		req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
		rec := httptest.NewRecorder()
		app.Router.ServeHTTP(rec, req)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503, got %d", rec.Code)
		}
	})
}

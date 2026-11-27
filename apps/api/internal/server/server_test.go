package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5"

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

	t.Run("timeout middleware exempts SSE streams", func(t *testing.T) {
		var sseHasDeadline, jsonHasDeadline bool
		app := New(Dependencies{
			Config: cfg,
			Registrars: []ModuleRegistrar{
				func(r chi.Router, _ huma.API) {
					r.Get("/api/v1/duels/stream", func(w http.ResponseWriter, req *http.Request) {
						_, sseHasDeadline = req.Context().Deadline()
						w.WriteHeader(http.StatusOK)
					})
					r.Get("/api/v1/regular", func(w http.ResponseWriter, req *http.Request) {
						_, jsonHasDeadline = req.Context().Deadline()
						w.WriteHeader(http.StatusOK)
					})
				},
			},
		})

		sseReq := httptest.NewRequest(http.MethodGet, "/api/v1/duels/stream", nil)
		sseReq.Header.Set("Accept", "text/event-stream")
		app.Router.ServeHTTP(httptest.NewRecorder(), sseReq)
		if sseHasDeadline {
			t.Fatalf("expected SSE stream request to bypass 30s timeout deadline")
		}

		jsonReq := httptest.NewRequest(http.MethodGet, "/api/v1/regular", nil)
		app.Router.ServeHTTP(httptest.NewRecorder(), jsonReq)
		if !jsonHasDeadline {
			t.Fatalf("expected ordinary HTTP request to carry 30s timeout deadline")
		}
	})
}

package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/AICSbench/planetlogi/apps/api/internal/config"
)

// ReadinessChecker checks external dependencies (PostgreSQL & Redis) for GET /readyz.
type ReadinessChecker interface {
	Ping(ctx context.Context) error
}

// ModuleRegistrar allows each Go feature module to register routes without editing a monolith.
type ModuleRegistrar func(router chi.Router, api huma.API)

// Dependencies groups stateless infrastructure dependencies injected into the HTTP server.
type Dependencies struct {
	Config     config.Config
	Logger     *slog.Logger
	DBChecker  ReadinessChecker
	RedisCheck ReadinessChecker
	Registrars []ModuleRegistrar
}

// App wraps both the Chi router and the Huma OpenAPI 3.1 API instance.
type App struct {
	Router chi.Router
	API    huma.API
}

// New constructs the stateless PlanetLogi HTTP router + OpenAPI 3.1 specification.
func New(deps Dependencies) *App {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   deps.Config.AllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-Request-ID"},
		ExposedHeaders:   []string{"X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// HA Load Balancer Liveness Probe — always 200 while process is alive.
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "ok",
			"service": "planetlogi-api",
			"time":    time.Now().UTC().Format(time.RFC3339),
		})
	})

	// HA Load Balancer Readiness Probe — checks PostgreSQL & Redis; returns 503 on outage.
	r.Get("/readyz", func(w http.ResponseWriter, req *http.Request) {
		ctx, cancel := context.WithTimeout(req.Context(), 2*time.Second)
		defer cancel()

		checks := map[string]string{
			"postgres": "ok",
			"redis":    "ok",
		}
		healthy := true

		if deps.DBChecker != nil {
			if err := deps.DBChecker.Ping(ctx); err != nil {
				checks["postgres"] = "unavailable"
				healthy = false
			}
		}
		if deps.RedisCheck != nil {
			if err := deps.RedisCheck.Ping(ctx); err != nil {
				checks["redis"] = "unavailable"
				healthy = false
			}
		}

		status := http.StatusOK
		state := "ready"
		if !healthy {
			status = http.StatusServiceUnavailable
			state = "degraded"
		}
		writeJSON(w, status, map[string]any{
			"status": state,
			"checks": checks,
		})
	})

	humaCfg := huma.DefaultConfig("PlanetLogi API", "0.1.0")
	humaCfg.Info.Description = "PlanetLogi Elementary Mathematics Learning Platform REST API (Grades 1-6, Bilingual EN/ID)"
	api := humachi.New(r, humaCfg)

	for _, reg := range deps.Registrars {
		if reg != nil {
			reg(r, api)
		}
	}

	return &App{
		Router: r,
		API:    api,
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

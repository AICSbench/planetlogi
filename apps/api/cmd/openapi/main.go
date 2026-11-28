package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/AICSbench/planetlogi/apps/api/internal/config"
	"github.com/AICSbench/planetlogi/apps/api/internal/modules/auth"
	"github.com/AICSbench/planetlogi/apps/api/internal/modules/dailyorbit"
	"github.com/AICSbench/planetlogi/apps/api/internal/modules/reports"
	"github.com/AICSbench/planetlogi/apps/api/internal/modules/streak"
	"github.com/AICSbench/planetlogi/apps/api/internal/modules/training"
	"github.com/AICSbench/planetlogi/apps/api/internal/server"
)

// Exports the authoritative OpenAPI 3.1 specification from the Go Huma router
// so packages/api-client can generate strongly typed TypeScript contracts.
func main() {
	cfg := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))

	authSvc := auth.NewService(cfg, nil, nil, logger)
	streakSvc := streak.NewService()
	dailyOrbitSvc := dailyorbit.NewService(nil, streakSvc)
	trainingSvc := training.NewService(nil)

	app := server.New(server.Dependencies{
		Config: cfg,
		Logger: logger,
		Registrars: []server.ModuleRegistrar{
			auth.NewHandler(authSvc, cfg).RegisterRoutes,
			streak.NewHandler(streakSvc, authSvc).RegisterRoutes,
			dailyorbit.NewHandler(dailyOrbitSvc, authSvc).RegisterRoutes,
			training.NewHandler(trainingSvc, authSvc).RegisterRoutes,
			reports.NewHandler(trainingSvc, authSvc).RegisterRoutes,
		},
	})

	specBytes, err := json.MarshalIndent(app.API.OpenAPI(), "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to marshal openapi spec: %v\n", err)
		os.Exit(1)
	}
	_, _ = os.Stdout.Write(specBytes)
	_, _ = os.Stdout.Write([]byte("\n"))
}

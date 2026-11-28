package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/AICSbench/planetlogi/apps/api/internal/config"
	"github.com/AICSbench/planetlogi/apps/api/internal/modules/academy"
	"github.com/AICSbench/planetlogi/apps/api/internal/modules/assignments"
	"github.com/AICSbench/planetlogi/apps/api/internal/modules/astroquiz"
	"github.com/AICSbench/planetlogi/apps/api/internal/modules/auth"
	"github.com/AICSbench/planetlogi/apps/api/internal/modules/customization"
	"github.com/AICSbench/planetlogi/apps/api/internal/modules/dailyorbit"
	"github.com/AICSbench/planetlogi/apps/api/internal/modules/leaderboards"
	"github.com/AICSbench/planetlogi/apps/api/internal/modules/profile"
	"github.com/AICSbench/planetlogi/apps/api/internal/modules/reports"
	"github.com/AICSbench/planetlogi/apps/api/internal/modules/social"
	"github.com/AICSbench/planetlogi/apps/api/internal/modules/stories"
	"github.com/AICSbench/planetlogi/apps/api/internal/modules/streak"
	"github.com/AICSbench/planetlogi/apps/api/internal/modules/supernova"
	"github.com/AICSbench/planetlogi/apps/api/internal/modules/training"
	"github.com/AICSbench/planetlogi/apps/api/internal/server"
)

func main() {
	cfg := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))

	authSvc := auth.NewService(cfg, nil, nil, logger)
	streakSvc := streak.NewService()
	dailyOrbitSvc := dailyorbit.NewService(nil, streakSvc)
	trainingSvc := training.NewService(nil)
	reportsH := reports.NewHandler(trainingSvc, authSvc)
	academySvc := academy.NewService(nil)
	quizSvc := astroquiz.NewService(nil)
	assignSvc := assignments.NewService(nil)
	profileSvc := profile.NewService(reportsH, streakSvc, authSvc, cfg.BillingEnabled)
	supernovaSvc := supernova.NewService(nil)
	storiesSvc := stories.NewService(nil)
	customSvc := customization.NewService(nil)
	socialSvc := social.NewService(nil)
	lbSvc := leaderboards.NewService(nil)

	app := server.New(server.Dependencies{
		Config: cfg,
		Logger: logger,
		Registrars: []server.ModuleRegistrar{
			auth.NewHandler(authSvc, cfg).RegisterRoutes,
			streak.NewHandler(streakSvc, authSvc).RegisterRoutes,
			dailyorbit.NewHandler(dailyOrbitSvc, authSvc).RegisterRoutes,
			training.NewHandler(trainingSvc, authSvc).RegisterRoutes,
			reportsH.RegisterRoutes,
			academy.NewHandler(academySvc, authSvc).RegisterRoutes,
			astroquiz.NewHandler(quizSvc, authSvc).RegisterRoutes,
			assignments.NewHandler(assignSvc, authSvc).RegisterRoutes,
			profile.NewHandler(profileSvc, authSvc).RegisterRoutes,
			supernova.NewHandler(supernovaSvc, authSvc).RegisterRoutes,
			stories.NewHandler(storiesSvc, authSvc).RegisterRoutes,
			customization.NewHandler(customSvc, authSvc).RegisterRoutes,
			social.NewHandler(socialSvc, authSvc).RegisterRoutes,
			leaderboards.NewHandler(lbSvc, authSvc).RegisterRoutes,
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

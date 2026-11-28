package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

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
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/economy"
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/state"
	"github.com/AICSbench/planetlogi/apps/api/internal/server"
)

type redisPinger struct{ client *redis.Client }

func (r redisPinger) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	cfg := config.Load()
	if err := cfg.ValidateForServer(); err != nil {
		logger.Error("invalid production server configuration", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to initialize postgres pool", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	defer func() { _ = rdb.Close() }()

	ledger := economy.NewPGLedger(pool)
	authSvc := auth.NewService(cfg, auth.NewPGStore(pool), rdb, logger)
	streakSvc := streak.NewServiceWithPG(pool)
	dailyOrbitSvc := dailyorbit.NewServiceWithPG(pool, ledger, streakSvc)
	trainingSvc := training.NewServiceWithPG(pool, ledger)
	dailyOrbitSvc.SetAttemptRecorder(trainingSvc)
	reportsH := reports.NewHandler(trainingSvc, authSvc)
	stateStore := state.NewPGStore(pool)
	academySvc := academy.NewServiceWithStore(stateStore, ledger)
	quizSvc := astroquiz.NewServiceWithStore(stateStore, ledger)
	assignSvc := assignments.NewServiceWithDeps(pool, stateStore, ledger)
	profileSvc := profile.NewServiceWithPG(pool, reportsH, streakSvc, authSvc, cfg.BillingEnabled)
	supernovaSvc := supernova.NewServiceWithPG(pool, ledger)
	supernovaSvc.SetEnforceWindow(cfg.SuperNovaEnforceOpen)
	storiesSvc := stories.NewServiceWithPG(pool, ledger)
	customSvc := customization.NewServiceWithPG(pool, ledger)
	socialSvc := social.NewServiceWithPG(pool, ledger)
	lbSvc := leaderboards.NewServiceWithPG(pool, ledger)

	app := server.New(server.Dependencies{
		Config:     cfg,
		Logger:     logger,
		DBChecker:  pool,
		RedisCheck: redisPinger{client: rdb},
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

	httpSrv := &http.Server{
		Addr:              ":" + cfg.APIPort,
		Handler:           app.Router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("planetlogi api starting", "port", cfg.APIPort, "env", cfg.AppEnv)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		logger.Error("http server error", "error", err)
		os.Exit(1)
	case <-ctx.Done():
		logger.Info("shutdown signal received; draining connections")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
		defer cancel()
		if err := httpSrv.Shutdown(shutdownCtx); err != nil {
			logger.Error("graceful shutdown error", "error", err)
			os.Exit(1)
		}
		logger.Info("planetlogi api stopped cleanly")
	}
}

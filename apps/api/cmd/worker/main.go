package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AICSbench/planetlogi/apps/api/internal/config"
	"github.com/AICSbench/planetlogi/apps/api/internal/modules/streak"
)

const (
	TaskLeaderboardRefresh  = "leaderboard:refresh"
	TaskMidnightStreakCheck = "streak:midnight_reconcile"
	TaskAnnualGradeRollover = "school:annual_rollover"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	cfg := config.Load()

	pool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to initialize postgres pool", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	streakSvc := streak.NewServiceWithPG(pool)

	redisOpt := asynq.RedisClientOpt{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	}

	srv := asynq.NewServer(redisOpt, asynq.Config{
		Concurrency: 10,
		Queues: map[string]int{
			"critical": 6,
			"default":  3,
			"low":      1,
		},
	})

	mux := asynq.NewServeMux()
	mux.HandleFunc(TaskLeaderboardRefresh, func(_ context.Context, _ *asynq.Task) error {
		logger.Info("running leaderboard refresh job")
		return nil
	})
	mux.HandleFunc(TaskMidnightStreakCheck, func(ctx context.Context, _ *asynq.Task) error {
		count, err := streakSvc.ReconcileAllMidnight(ctx, time.Now())
		if err != nil {
			logger.Error("midnight orbit streak reconciliation failed", "error", err)
			return err
		}
		logger.Info("completed midnight orbit streak reconciliation job", "students", count)
		return nil
	})
	mux.HandleFunc(TaskAnnualGradeRollover, func(_ context.Context, _ *asynq.Task) error {
		logger.Info("running annual grade rollover check")
		return nil
	})

	scheduler := asynq.NewScheduler(redisOpt, nil)
	if _, err := scheduler.Register("@hourly", asynq.NewTask(TaskMidnightStreakCheck, nil), asynq.Queue("default")); err != nil {
		logger.Error("failed to register midnight streak scheduler", "error", err)
		os.Exit(1)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		logger.Info("shutdown signal received; stopping asynq worker gracefully")
		scheduler.Shutdown()
		srv.Shutdown()
	}()

	go func() {
		if err := scheduler.Run(); err != nil {
			logger.Error("asynq scheduler exited", "error", err)
		}
	}()

	logger.Info("planetlogi asynq worker starting", "redis", cfg.RedisAddr)
	if err := srv.Run(mux); err != nil {
		logger.Error("worker exited with error", "error", err)
		os.Exit(1)
	}
}

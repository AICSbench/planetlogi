package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/hibiken/asynq"

	"github.com/AICSbench/planetlogi/apps/api/internal/config"
)

const (
	TaskLeaderboardRefresh  = "leaderboard:refresh"
	TaskMidnightStreakCheck = "streak:midnight_reconcile"
	TaskAnnualGradeRollover = "school:annual_rollover"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	cfg := config.Load()

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
	mux.HandleFunc(TaskMidnightStreakCheck, func(_ context.Context, _ *asynq.Task) error {
		logger.Info("running midnight orbit streak reconciliation job")
		return nil
	})
	mux.HandleFunc(TaskAnnualGradeRollover, func(_ context.Context, _ *asynq.Task) error {
		logger.Info("running annual grade rollover check")
		return nil
	})

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		logger.Info("shutdown signal received; stopping asynq worker gracefully")
		srv.Shutdown()
	}()

	logger.Info("planetlogi asynq worker starting", "redis", cfg.RedisAddr)
	if err := srv.Run(mux); err != nil {
		logger.Error("worker exited with error", "error", err)
		os.Exit(1)
	}
}

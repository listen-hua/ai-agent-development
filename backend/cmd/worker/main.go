package main

import (
	"context"
	"internal-ai-agent/backend/internal/config"
	"internal-ai-agent/backend/internal/integration/feishu"
	"internal-ai-agent/backend/internal/model"
	"internal-ai-agent/backend/internal/service"
	"internal-ai-agent/backend/internal/store"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	cfg := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL is required by reminder worker")
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	repo, err := store.OpenPostgres(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer repo.Close()
	aliyun := model.NewAliyun(cfg.DashScopeAPIKey, cfg.DashScopeBaseURL)
	var provider model.Provider = aliyun
	if !aliyun.Available() {
		provider = model.Mock{}
		logger.Warn("DASHSCOPE_API_KEY is not configured; reminder natural-language fallback is limited")
	}
	feishuClient := feishu.New(cfg.FeishuAppID, cfg.FeishuAppSecret, cfg.FeishuRedirectURI)
	reminders := service.NewReminder(repo, provider, cfg.ReminderModel, cfg.ReminderTimezone, cfg.ReminderMaxActive)
	dispatcher := service.NewReminderDispatcher(repo, reminders, feishuClient, cfg.FeishuAppLink, time.Duration(cfg.ReminderGraceMinutes)*time.Minute, time.Duration(cfg.RetentionDays)*24*time.Hour)
	notifications := service.NewNotification(repo, feishuClient, provider)
	interval := time.Duration(cfg.ReminderPollSeconds) * time.Second
	if interval < time.Second {
		interval = 5 * time.Second
	}
	logger.Info("worker started", "retention_days", cfg.RetentionDays, "reminder_poll", interval, "reminder_grace_minutes", cfg.ReminderGraceMinutes)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	if err = dispatcher.Tick(ctx); err != nil {
		logger.Error("initial reminder tick failed", "error", err)
	}
	if err = notifications.DispatchDue(ctx); err != nil {
		logger.Error("initial scheduled notification tick failed", "error", err)
	}
	for {
		select {
		case <-ticker.C:
			if err = dispatcher.Tick(ctx); err != nil {
				logger.Error("reminder tick failed", "error", err)
			}
			if err = notifications.DispatchDue(ctx); err != nil {
				logger.Error("scheduled notification tick failed", "error", err)
			}
		case <-ctx.Done():
			logger.Info("worker stopped")
			return
		}
	}
}

package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"internal-ai-agent/backend/internal/blob"
	"internal-ai-agent/backend/internal/config"
	"internal-ai-agent/backend/internal/domain"
	"internal-ai-agent/backend/internal/httpapi"
	"internal-ai-agent/backend/internal/integration/feishu"
	"internal-ai-agent/backend/internal/model"
	"internal-ai-agent/backend/internal/parser"
	"internal-ai-agent/backend/internal/security"
	"internal-ai-agent/backend/internal/service"
	"internal-ai-agent/backend/internal/store"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg := config.Load()
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	rootCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	defaultConfig := domain.AgentConfig{GenerationModel: cfg.GenerationModel, EmbeddingModel: cfg.EmbeddingModel, RerankModel: cfg.RerankModel, Temperature: .1, MaxOutputTokens: 1600, TimeoutSeconds: 90, RetrievalTopK: 30, RerankTopN: 6, ScoreThreshold: .45, ContextBudget: 12000, SystemPrompt: "你是公司行政制度助手。只能依据提供的已发布制度回答，不得编造，不得使用外部常识补充公司规定。"}
	var repo store.Repository
	var postgres *store.Postgres
	if cfg.DatabaseURL != "" {
		var err error
		postgres, err = store.OpenPostgres(context.Background(), cfg.DatabaseURL)
		if err != nil {
			if cfg.Environment == "production" {
				log.Fatal(err)
			}
			slog.Warn("postgres unavailable; using in-memory repository", "error", err)
		} else if err = postgres.EnsureSeed(context.Background(), defaultConfig); err != nil {
			log.Fatal(err)
		}
	}
	if postgres != nil {
		repo = postgres
		defer postgres.Close()
	} else {
		repo = store.NewMemory(defaultConfig)
	}
	grantedAdmins, err := service.EnsureBootstrapSuperAdmins(rootCtx, repo, cfg.BootstrapSuperAdminOpenIDs)
	if err != nil {
		return err
	}
	if len(cfg.BootstrapSuperAdminOpenIDs) > 0 {
		slog.Info("bootstrap administrator reconciliation completed", "configured", len(cfg.BootstrapSuperAdminOpenIDs), "granted", grantedAdmins)
	}
	aliyun := model.NewAliyun(cfg.DashScopeAPIKey, cfg.DashScopeBaseURL)
	var provider model.Provider = aliyun
	if !aliyun.Available() {
		provider = model.Mock{}
		slog.Warn("DASHSCOPE_API_KEY is not configured; using deterministic demo provider")
	}
	feishuClient := feishu.New(cfg.FeishuAppID, cfg.FeishuAppSecret, cfg.FeishuRedirectURI)
	directory := service.NewDirectory(repo, feishuClient)
	agents, err := service.NewAgentRegistry(repo, cfg.AgentSecretEncryptionKey, cfg.DashScopeAPIKey, cfg.GeminiAPIKey, cfg.GenerationModel, cfg.GeminiImageModel)
	if err != nil {
		return err
	}
	if err = agents.EnsureDefaults(rootCtx); err != nil {
		return err
	}
	provider = model.NewAgentRoutedProvider(agents, "administrative_assistant", cfg.DashScopeBaseURL, provider)
	tika := parser.New(cfg.TikaURL)
	var blobStore blob.Store = blob.Noop{}
	if cfg.MinIOEndpoint != "" {
		value, err := blob.NewMinIO(context.Background(), cfg.MinIOEndpoint, cfg.MinIOAccessKey, cfg.MinIOSecretKey, cfg.MinIOBucket, cfg.MinIOSecure)
		if err != nil {
			if cfg.Environment == "production" {
				log.Fatal(err)
			}
			slog.Warn("minio unavailable; originals are not persisted", "error", err)
		} else {
			blobStore = value
		}
	}
	var scanner security.Scanner = security.NoopScanner{}
	if cfg.ClamAVAddr != "" {
		scanner = security.ClamAV{Addr: cfg.ClamAVAddr}
	}
	knowledge := service.NewKnowledge(repo, provider, tika, cfg.EmbeddingModel, blobStore, scanner, feishuClient)
	hub := service.NewRunHub()
	reminders := service.NewReminder(repo, provider, cfg.ReminderModel, cfg.ReminderTimezone, cfg.ReminderMaxActive)
	chat := service.NewChat(repo, provider, hub, reminders)
	notifications := service.NewNotification(repo, feishuClient, provider, scanner)
	feishuBot := service.NewFeishuBot(repo, chat, feishuClient, cfg.FeishuAppLink, reminders)
	longConnection := feishu.NewLongConnection(cfg.FeishuAppID, cfg.FeishuAppSecret, feishuBot, directory)
	sessions := security.NewSessions(cfg.SessionSecret)
	seed(context.Background(), repo, knowledge)
	go knowledge.RunSourceScheduler(rootCtx, 15*time.Minute)
	handler := httpapi.New(cfg, repo, sessions, chat, knowledge, notifications, reminders, feishuClient, directory, agents).Handler()
	server := &http.Server{Addr: cfg.HTTPAddr, Handler: handler, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 2 * time.Minute, MaxHeaderBytes: 1 << 20}

	var longConnectionErr <-chan error
	if longConnection.Configured() {
		result := make(chan error, 1)
		longConnectionErr = result
		go func() { result <- longConnection.Start(rootCtx) }()
	} else {
		slog.Warn("FEISHU_APP_ID or FEISHU_APP_SECRET is not configured; long connection disabled")
	}

	httpErr := make(chan error, 1)
	slog.Info("server listening", "addr", cfg.HTTPAddr)
	go func() {
		err := server.ListenAndServe()
		if err == http.ErrServerClosed {
			err = nil
		}
		httpErr <- err
	}()

	var runErr error
	select {
	case <-rootCtx.Done():
		slog.Info("server shutdown requested")
	case runErr = <-httpErr:
	case runErr = <-longConnectionErr:
		if runErr == nil {
			runErr = context.Canceled
		}
	}

	longConnection.Close()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil && runErr == nil {
		runErr = err
	}
	return runErr
}

func seed(ctx context.Context, repo store.Repository, knowledge *service.Knowledge) {
	admin, err := repo.GetUser(ctx, store.DemoAdminID)
	if err != nil {
		return
	}
	docs, _ := repo.ListDocuments(ctx)
	if len(docs) > 0 {
		return
	}
	content := []byte("员工休假制度\n\n年假：员工连续工作满一年后可享受带薪年假。申请年假应至少提前三个工作日在飞书提交申请，并经直属负责人审批。\n\n病假：病假一天以上需要提供合法医疗机构出具的证明。\n\n报销制度\n\n日常费用应在发生后三十个自然日内提交报销，超过期限需补充书面说明。")
	doc, err := knowledge.Ingest(ctx, admin, "员工休假与报销制度（演示）", "text/plain", content, domain.ACL{Scope: "all"})
	if err == nil {
		_, _ = knowledge.Publish(ctx, admin, doc.ID)
	}
}

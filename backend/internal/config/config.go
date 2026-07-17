package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Environment                string
	HTTPAddr                   string
	PublicURL                  string
	SessionSecret              string
	AgentSecretEncryptionKey   string
	DevAuthEnabled             bool
	DatabaseURL                string
	RedisAddr                  string
	MinIOEndpoint              string
	MinIOAccessKey             string
	MinIOSecretKey             string
	MinIOBucket                string
	MinIOSecure                bool
	TikaURL                    string
	ClamAVAddr                 string
	FeishuAppID                string
	FeishuAppSecret            string
	FeishuRedirectURI          string
	FeishuAppLink              string
	BootstrapSuperAdminOpenIDs []string
	DashScopeAPIKey            string
	DashScopeBaseURL           string
	GenerationModel            string
	EmbeddingModel             string
	RerankModel                string
	GeminiAPIKey               string
	GeminiImageModel           string
	ReminderModel              string
	ReminderTimezone           string
	ReminderPollSeconds        int
	ReminderGraceMinutes       int
	ReminderMaxActive          int
	RetentionDays              int
}

func Load() Config {
	config := Config{
		Environment:                env("APP_ENV", "development"),
		HTTPAddr:                   env("HTTP_ADDR", ":8080"),
		PublicURL:                  env("PUBLIC_URL", "http://localhost:5173"),
		SessionSecret:              env("SESSION_SECRET", "development-only-secret-change-me"),
		DevAuthEnabled:             envBool("DEV_AUTH_ENABLED", true),
		DatabaseURL:                os.Getenv("DATABASE_URL"),
		RedisAddr:                  env("REDIS_ADDR", "localhost:6379"),
		MinIOEndpoint:              os.Getenv("MINIO_ENDPOINT"),
		MinIOAccessKey:             os.Getenv("MINIO_ACCESS_KEY"),
		MinIOSecretKey:             os.Getenv("MINIO_SECRET_KEY"),
		MinIOBucket:                env("MINIO_BUCKET", "ai-agent-documents"),
		MinIOSecure:                envBool("MINIO_SECURE", false),
		TikaURL:                    os.Getenv("TIKA_URL"),
		ClamAVAddr:                 os.Getenv("CLAMAV_ADDR"),
		FeishuAppID:                os.Getenv("FEISHU_APP_ID"),
		FeishuAppSecret:            os.Getenv("FEISHU_APP_SECRET"),
		FeishuRedirectURI:          os.Getenv("FEISHU_REDIRECT_URI"),
		FeishuAppLink:              os.Getenv("FEISHU_APP_LINK"),
		BootstrapSuperAdminOpenIDs: csv(os.Getenv("BOOTSTRAP_SUPER_ADMIN_OPEN_IDS")),
		DashScopeAPIKey:            os.Getenv("DASHSCOPE_API_KEY"),
		DashScopeBaseURL:           env("DASHSCOPE_BASE_URL", "https://dashscope.aliyuncs.com/compatible-mode/v1"),
		GenerationModel:            env("DASHSCOPE_GENERATION_MODEL", "qwen-plus"),
		EmbeddingModel:             env("DASHSCOPE_EMBEDDING_MODEL", "text-embedding-v4"),
		RerankModel:                env("DASHSCOPE_RERANK_MODEL", "qwen3-rerank"),
		GeminiAPIKey:               os.Getenv("GEMINI_API_KEY"),
		GeminiImageModel:           env("GEMINI_IMAGE_MODEL", "gemini-3.1-flash-image"),
		ReminderModel:              env("DASHSCOPE_REMINDER_MODEL", "qwen-flash"),
		ReminderTimezone:           env("REMINDER_TIMEZONE", "Asia/Shanghai"),
		ReminderPollSeconds:        envInt("REMINDER_POLL_SECONDS", 5),
		ReminderGraceMinutes:       envInt("REMINDER_GRACE_MINUTES", 30),
		ReminderMaxActive:          envInt("REMINDER_MAX_ACTIVE_PER_USER", 100),
		RetentionDays:              envInt("RETENTION_DAYS", 90),
	}
	config.AgentSecretEncryptionKey = env("AGENT_SECRET_ENCRYPTION_KEY", config.SessionSecret)
	if config.FeishuAppLink == "" && config.FeishuAppID != "" {
		config.FeishuAppLink = fmt.Sprintf("https://applink.feishu.cn/client/web_app/open?appId=%s&mode=appCenter&path=/chat", config.FeishuAppID)
	}
	return config
}

func csv(value string) []string {
	values := []string{}
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			values = append(values, item)
		}
	}
	return values
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envInt(key string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(key))
	if err != nil {
		return fallback
	}
	return value
}

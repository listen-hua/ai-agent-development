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
	FeishuMeetingCalendarID    string
	BootstrapSuperAdminOpenIDs []string
	DashScopeAPIKey            string
	DashScopeBaseURL           string
	GenerationModel            string
	EmbeddingModel             string
	RerankModel                string
	ContextModel               string
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
	environment := strings.ToLower(strings.TrimSpace(env("APP_ENV", "development")))
	config := Config{
		Environment:                environment,
		HTTPAddr:                   env("HTTP_ADDR", ":8080"),
		PublicURL:                  env("PUBLIC_URL", "http://localhost:5173"),
		SessionSecret:              env("SESSION_SECRET", "development-only-secret-change-me"),
		DevAuthEnabled:             envBool("DEV_AUTH_ENABLED", environment != "production"),
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
		FeishuMeetingCalendarID:    os.Getenv("FEISHU_MEETING_CALENDAR_ID"),
		BootstrapSuperAdminOpenIDs: csv(os.Getenv("BOOTSTRAP_SUPER_ADMIN_OPEN_IDS")),
		DashScopeAPIKey:            os.Getenv("DASHSCOPE_API_KEY"),
		DashScopeBaseURL:           env("DASHSCOPE_BASE_URL", "https://dashscope.aliyuncs.com/compatible-mode/v1"),
		GenerationModel:            env("DASHSCOPE_GENERATION_MODEL", "qwen-plus"),
		EmbeddingModel:             env("DASHSCOPE_EMBEDDING_MODEL", "text-embedding-v4"),
		RerankModel:                env("DASHSCOPE_RERANK_MODEL", "qwen3-rerank"),
		ContextModel:               env("DASHSCOPE_CONTEXT_MODEL", "qwen-flash"),
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

func (c Config) Validate() error {
	if c.Environment != "production" {
		return nil
	}
	var problems []string
	if c.DevAuthEnabled {
		problems = append(problems, "DEV_AUTH_ENABLED must be false")
	}
	if weakSecret(c.SessionSecret) {
		problems = append(problems, "SESSION_SECRET must be a non-placeholder value with at least 32 characters")
	}
	if weakSecret(c.AgentSecretEncryptionKey) {
		problems = append(problems, "AGENT_SECRET_ENCRYPTION_KEY must be a non-placeholder value with at least 32 characters")
	}
	required := []struct {
		name  string
		value string
	}{
		{"DATABASE_URL", c.DatabaseURL},
		{"MINIO_ENDPOINT", c.MinIOEndpoint},
		{"MINIO_ACCESS_KEY", c.MinIOAccessKey},
		{"MINIO_SECRET_KEY", c.MinIOSecretKey},
		{"DASHSCOPE_API_KEY", c.DashScopeAPIKey},
		{"FEISHU_APP_ID", c.FeishuAppID},
		{"FEISHU_APP_SECRET", c.FeishuAppSecret},
	}
	for _, item := range required {
		if strings.TrimSpace(item.value) == "" {
			problems = append(problems, item.name+" is required")
		}
	}
	if strings.TrimSpace(c.MinIOSecretKey) != "" && weakSecret(c.MinIOSecretKey) {
		problems = append(problems, "MINIO_SECRET_KEY must be a non-placeholder value with at least 32 characters")
	}
	if len(problems) > 0 {
		return fmt.Errorf("invalid production configuration: %s", strings.Join(problems, "; "))
	}
	return nil
}

func weakSecret(value string) bool {
	value = strings.TrimSpace(value)
	lower := strings.ToLower(value)
	return len(value) < 32 || strings.Contains(lower, "development") || strings.Contains(lower, "change")
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

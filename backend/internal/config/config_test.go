package config

import (
	"strings"
	"testing"
)

func TestLoadBuildsDefaultFeishuAppLink(t *testing.T) {
	t.Setenv("FEISHU_APP_ID", "cli_test")
	t.Setenv("FEISHU_APP_LINK", "")
	value := Load()
	if !strings.Contains(value.FeishuAppLink, "appId=cli_test") || !strings.Contains(value.FeishuAppLink, "path=/chat") {
		t.Fatalf("unexpected default AppLink: %s", value.FeishuAppLink)
	}
}

func TestProductionDefaultsAreRejected(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("DEV_AUTH_ENABLED", "")
	t.Setenv("SESSION_SECRET", "")
	t.Setenv("AGENT_SECRET_ENCRYPTION_KEY", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("MINIO_ENDPOINT", "")
	t.Setenv("MINIO_ACCESS_KEY", "")
	t.Setenv("MINIO_SECRET_KEY", "")
	t.Setenv("DASHSCOPE_API_KEY", "")
	t.Setenv("FEISHU_APP_ID", "")
	t.Setenv("FEISHU_APP_SECRET", "")
	value := Load()
	if value.DevAuthEnabled {
		t.Fatal("development authentication must default to disabled in production")
	}
	if err := value.Validate(); err == nil {
		t.Fatal("expected insecure production defaults to be rejected")
	}
}

func TestSecureProductionConfigurationIsAccepted(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("DEV_AUTH_ENABLED", "false")
	t.Setenv("SESSION_SECRET", "session-secret-with-at-least-32-characters")
	t.Setenv("AGENT_SECRET_ENCRYPTION_KEY", "agent-secret-with-at-least-32-characters")
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("MINIO_ENDPOINT", "minio:9000")
	t.Setenv("MINIO_ACCESS_KEY", "access")
	t.Setenv("MINIO_SECRET_KEY", "minio-secret-with-at-least-32-characters")
	t.Setenv("DASHSCOPE_API_KEY", "dashscope")
	t.Setenv("FEISHU_APP_ID", "app")
	t.Setenv("FEISHU_APP_SECRET", "secret")
	if err := Load().Validate(); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestIAMProductionConfigurationRequiresAppCredentials(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("DEV_AUTH_ENABLED", "false")
	t.Setenv("SESSION_SECRET", "session-secret-with-at-least-32-characters")
	t.Setenv("AGENT_SECRET_ENCRYPTION_KEY", "agent-secret-with-at-least-32-characters")
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("MINIO_ENDPOINT", "minio:9000")
	t.Setenv("MINIO_ACCESS_KEY", "access")
	t.Setenv("MINIO_SECRET_KEY", "minio-secret-with-at-least-32-characters")
	t.Setenv("DASHSCOPE_API_KEY", "dashscope")
	t.Setenv("FEISHU_APP_ID", "app")
	t.Setenv("FEISHU_APP_SECRET", "secret")
	t.Setenv("IAM_ENABLED", "true")
	t.Setenv("IAM_APP_ID", "")
	t.Setenv("IAM_APP_SECRET", "")
	err := Load().Validate()
	if err == nil || !strings.Contains(err.Error(), "IAM_APP_ID") || !strings.Contains(err.Error(), "IAM_APP_SECRET") {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

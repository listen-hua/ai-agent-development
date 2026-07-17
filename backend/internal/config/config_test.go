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

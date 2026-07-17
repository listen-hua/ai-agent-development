package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"internal-ai-agent/backend/internal/config"
	"internal-ai-agent/backend/internal/integration/feishu"
)

func TestFeishuAuthConfigExposesOnlyPublicClientConfig(t *testing.T) {
	server := &Server{
		cfg:    config.Config{FeishuAppID: "cli_test", FeishuAppSecret: "do-not-expose"},
		feishu: feishu.New("cli_test", "do-not-expose", ""),
	}
	recorder := httptest.NewRecorder()
	server.feishuAuthConfig(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/auth/feishu/config", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	if strings.Contains(recorder.Body.String(), "do-not-expose") {
		t.Fatal("response exposed the app secret")
	}
	var response struct {
		AppID   string `json:"app_id"`
		Enabled bool   `json:"enabled"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.AppID != "cli_test" || !response.Enabled {
		t.Fatalf("unexpected response: %+v", response)
	}
}

func TestExchangeRejectsEmptyCode(t *testing.T) {
	server := &Server{}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/feishu/exchange", strings.NewReader(`{"code":"  "}`))
	request.Header.Set("Content-Type", "application/json")
	server.exchange(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
}

func TestFeishuClientDiagnosticsAcceptsSanitizedClientError(t *testing.T) {
	server := &Server{}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/feishu/client-diagnostics", strings.NewReader(`{"stage":"request_access_fail","errno":"103","message":"not supported","h5sdk":true,"request_access":true,"request_auth_code":true}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "Lark/7.67.9")
	server.feishuClientDiagnostics(recorder, request)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
}

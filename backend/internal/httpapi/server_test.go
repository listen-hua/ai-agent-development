package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"internal-ai-agent/backend/internal/config"
	"internal-ai-agent/backend/internal/domain"
	"internal-ai-agent/backend/internal/integration/feishu"
	iamintegration "internal-ai-agent/backend/internal/integration/iam"
	"internal-ai-agent/backend/internal/security"
	"internal-ai-agent/backend/internal/store"
)

func TestFeishuAuthConfigExposesOnlyPublicClientConfig(t *testing.T) {
	server := &Server{
		cfg:    config.Config{FeishuAppID: "cli_test", FeishuAppSecret: "do-not-expose", DevAuthEnabled: true},
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
		AppID          string `json:"app_id"`
		Enabled        bool   `json:"enabled"`
		DevAuthEnabled bool   `json:"dev_auth_enabled"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.AppID != "cli_test" || !response.Enabled || !response.DevAuthEnabled {
		t.Fatalf("unexpected response: %+v", response)
	}
}

func TestIAMAuthConfigDoesNotExposeSecret(t *testing.T) {
	iamClient := iamintegration.New("https://iam.example.com", "shimmer-ai", "do-not-expose", "", time.Second, time.Second)
	server := &Server{
		cfg:    config.Config{IAMEnabled: true, IAMAppID: "shimmer-ai", IAMAppSecret: "do-not-expose"},
		iam:    iamClient,
		feishu: feishu.New("feishu-app", "feishu-secret", ""),
	}
	recorder := httptest.NewRecorder()
	server.iamAuthConfig(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/auth/iam/config", nil))
	if recorder.Code != http.StatusOK || strings.Contains(recorder.Body.String(), "do-not-expose") {
		t.Fatalf("unexpected IAM config response: %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestIAMPermissionMiddlewareUsesIAMInsteadOfLocalAdminRole(t *testing.T) {
	iamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		permission := r.URL.Query().Get("permission_key")
		w.Header().Set("Content-Type", "application/json")
		if permission == permissionAgentUse {
			_, _ = w.Write([]byte(`{"code":0,"data":{"user_id":18}}`))
			return
		}
		_, _ = w.Write([]byte(`{"code":510001,"message":"权限不足"}`))
	}))
	defer iamServer.Close()
	iamClient := iamintegration.New(iamServer.URL, "shimmer-ai", "secret", "", time.Second, time.Second)
	repo := store.NewMemory(domain.AgentConfig{})
	iamUserID := int64(18)
	user, err := repo.UpsertIAMUser(context.Background(), domain.User{
		FeishuOpenID: "ou_iam_user",
		FeishuUserID: "iam-feishu-user",
		IAMUserID:    &iamUserID,
		Name:         "IAM User",
		Status:       "active",
		Roles:        []domain.Role{domain.RoleEmployee, domain.RoleSuperAdmin},
	})
	if err != nil {
		t.Fatal(err)
	}
	sessions := security.NewSessions("a-secret-long-enough-for-tests")
	sessionToken, err := sessions.IssueFor(user.ID, "iam", iamUserID, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	server := &Server{repo: repo, sessions: sessions, iam: iamClient}
	handler := server.auth(server.permission(permissionUserManage, domain.RoleSuperAdmin)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})))
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	request.AddCookie(&http.Cookie{Name: "ai_agent_session", Value: sessionToken})
	request.AddCookie(&http.Cookie{Name: "iam_user_token", Value: "iam-token"})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	var payload struct {
		Code int `json:"code"`
	}
	if err = json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if recorder.Code != http.StatusOK || payload.Code != iamintegration.CodePermissionDenied {
		t.Fatalf("IAM denial must override local super_admin: status=%d body=%s", recorder.Code, recorder.Body.String())
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

func TestBuildDirectoryDepartmentOptionsUsesReadableHierarchyPaths(t *testing.T) {
	options := buildDirectoryDepartmentOptions([]feishu.ContactDepartment{
		{OpenDepartmentID: "od_engineering", Name: "研发部", ParentDepartmentID: "od_product"},
		{OpenDepartmentID: "od_product", Name: "产品中心", ParentDepartmentID: "0"},
		{OpenDepartmentID: "od_finance", Name: "财务部", ParentDepartmentID: "0"},
	})
	if len(options) != 3 {
		t.Fatalf("unexpected options: %#v", options)
	}
	byID := make(map[string]directoryDepartmentOption, len(options))
	for _, option := range options {
		byID[option.ID] = option
	}
	if got := byID["od_engineering"]; got.Name != "研发部" || got.Path != "产品中心 / 研发部" || got.Depth != 1 {
		t.Fatalf("unexpected nested department option: %#v", got)
	}
	if got := byID["od_finance"]; got.Path != "财务部" || got.Depth != 0 {
		t.Fatalf("unexpected top-level department option: %#v", got)
	}
}

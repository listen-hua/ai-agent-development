package feishu

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExchangeLegacyCodeUsesMatchingTokenFlow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/open-apis/auth/v3/app_access_token/internal":
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "app_access_token": "app-token", "expire": 7200})
		case "/open-apis/authen/v1/access_token":
			if r.Header.Get("Authorization") != "Bearer app-token" {
				t.Errorf("unexpected app authorization header: %s", r.Header.Get("Authorization"))
				http.Error(w, "bad authorization", http.StatusUnauthorized)
				return
			}
			var input map[string]string
			_ = json.NewDecoder(r.Body).Decode(&input)
			if input["code"] != "legacy-code" || input["grant_type"] != "authorization_code" {
				t.Errorf("unexpected token request: %#v", input)
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]string{"access_token": "user-token"}})
		case "/open-apis/authen/v1/user_info":
			if r.Header.Get("Authorization") != "Bearer user-token" {
				t.Errorf("unexpected user authorization header: %s", r.Header.Get("Authorization"))
				http.Error(w, "bad authorization", http.StatusUnauthorized)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]string{"open_id": "ou_test", "name": "测试员工", "tenant_key": "tenant_test"}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := New("cli_test", "secret", "http://127.0.0.1:8088/")
	client.baseURL = server.URL
	client.http = server.Client()
	user, err := client.ExchangeLegacyCode(context.Background(), "legacy-code")
	if err != nil {
		t.Fatal(err)
	}
	if user.OpenID != "ou_test" || user.Name != "测试员工" || user.TenantKey != "tenant_test" {
		t.Fatalf("unexpected user: %#v", user)
	}
}

func TestGetContactUserReturnsAuthorizationAttributes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/open-apis/auth/v3/tenant_access_token/internal/":
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "tenant_access_token": "tenant-token", "expire": 7200})
		case "/open-apis/contact/v3/users/ou_test":
			if r.Header.Get("Authorization") != "Bearer tenant-token" || r.URL.Query().Get("department_id_type") != "open_department_id" {
				http.Error(w, "bad authorization", http.StatusUnauthorized)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{"user": map[string]any{"open_id": "ou_test", "name": "测试员工", "department_ids": []string{"od_finance"}, "job_title": "会计", "job_level_id": "jl_1", "job_family_id": "jf_finance", "employee_type": 1, "avatar": map[string]string{"avatar_72": "https://example/avatar"}, "status": map[string]bool{"is_activated": true}}}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := New("cli_test", "secret", "")
	client.baseURL = server.URL
	client.http = server.Client()
	user, err := client.GetContactUser(context.Background(), "ou_test")
	if err != nil {
		t.Fatal(err)
	}
	if user.JobTitle != "会计" || len(user.DepartmentIDs) != 1 || user.DepartmentIDs[0] != "od_finance" || user.Status != "active" {
		t.Fatalf("unexpected contact profile: %#v", user)
	}
}

func TestSendRichCardNormalizesLongMessageUUID(t *testing.T) {
	var receivedUUID string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/open-apis/auth/v3/tenant_access_token/internal/":
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "tenant_access_token": "tenant-token", "expire": 7200})
		case "/open-apis/im/v1/messages":
			if r.URL.Query().Get("receive_id_type") != "chat_id" {
				t.Errorf("unexpected receive_id_type: %s", r.URL.Query().Get("receive_id_type"))
			}
			var input map[string]any
			_ = json.NewDecoder(r.Body).Decode(&input)
			receivedUUID, _ = input["uuid"].(string)
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]string{"message_id": "om_test"}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := New("cli_test", "secret", "")
	client.baseURL = server.URL
	client.http = server.Client()
	longUUID := strings.Repeat("a", 36) + "_" + strings.Repeat("b", 36)
	messageID, err := client.SendRichCard(context.Background(), "chat_id", "oc_test", "测试通知", "正文", nil, longUUID)
	if err != nil {
		t.Fatal(err)
	}
	if messageID != "om_test" {
		t.Fatalf("unexpected message ID: %s", messageID)
	}
	if len(receivedUUID) != 50 || receivedUUID != normalizeMessageUUID(longUUID) {
		t.Fatalf("message uuid was not normalized: %q (%d)", receivedUUID, len(receivedUUID))
	}
	if normalizeMessageUUID("short-idempotency-key") != "short-idempotency-key" {
		t.Fatal("short message uuid should remain unchanged")
	}
}

func TestNormalizeCardMarkdownConvertsUnsupportedHeadings(t *testing.T) {
	input := "# 一级标题\n## 二级标题\n### 三级标题\n正文\n#"
	want := "**▌ 一级标题**\n**• 二级标题**\n**• 三级标题**\n正文\n#"
	if got := normalizeCardMarkdown(input); got != want {
		t.Fatalf("unexpected normalized markdown:\n%s", got)
	}
}

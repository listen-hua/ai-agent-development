package iam

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAuthenticateUsesBearerAndEncodesParameters(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer token-value" {
			t.Fatalf("unexpected authorization: %q", r.Header.Get("Authorization"))
		}
		if r.URL.Query().Get("app_id") != "shimmer-ai" || r.URL.Query().Get("app_secret") != "secret" {
			t.Fatalf("unexpected app credentials: %s", r.URL.RawQuery)
		}
		if r.URL.Query().Get("attrs") != "channel=cn|global,game=1" {
			t.Fatalf("unexpected attrs: %q", r.URL.Query().Get("attrs"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"data":{"user_id":18}}`))
	}))
	defer server.Close()

	client := New(server.URL, "shimmer-ai", "secret", "", time.Second, time.Second)
	userID, err := client.Authenticate(context.Background(), "token-value", "agent_use", map[string][]string{
		"game": {"1"}, "channel": {"cn", "global"},
	})
	if err != nil || userID != 18 {
		t.Fatalf("unexpected result: user=%d error=%v", userID, err)
	}
}

func TestAuthenticatePassesThroughIAMError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":510001,"message":"权限不足"}`))
	}))
	defer server.Close()
	client := New(server.URL, "shimmer-ai", "secret", "", time.Second, time.Second)
	_, err := client.Authenticate(context.Background(), "token", "knowledge_manage", nil)
	if code, ok := ErrorCode(err); !ok || code != CodePermissionDenied {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAuthenticateRejectsMissingTokenWithoutCallingIAM(t *testing.T) {
	client := New("https://iam.example.com", "app", "secret", "", time.Second, time.Second)
	_, err := client.Authenticate(context.Background(), " ", "agent_use", nil)
	if code, ok := ErrorCode(err); !ok || code != CodeTokenInvalid {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLookupUserFindsExpectedIdentity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Cookie"), "iam_user_token=token") {
			t.Fatalf("missing IAM cookie: %q", r.Header.Get("Cookie"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":17,"feishu_account_info":{"name":"A","avatar_url":"","user_id":"feishu-17"}},{"id":18,"feishu_account_info":{"name":"B","avatar_url":"","user_id":"feishu-18"}}]}`))
	}))
	defer server.Close()
	client := New(server.URL, "app", "secret", "", time.Second, time.Second)
	value, err := client.LookupUser(context.Background(), "token", 18)
	if err != nil || value.FeishuAccountInfo.UserID != "feishu-18" {
		t.Fatalf("unexpected result: %+v error=%v", value, err)
	}
}

package feishu

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListContactDepartmentsReturnsOrganizationStructure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/open-apis/auth/v3/tenant_access_token/internal/":
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "tenant_access_token": "tenant-token", "expire": 7200})
		case "/open-apis/contact/v3/departments":
			if r.URL.Query().Get("fetch_child") != "false" {
				t.Errorf("department roots must be discovered without recursion: %s", r.URL.RawQuery)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": 0,
				"data": map[string]any{
					"has_more": false,
					"items": []any{
						map[string]any{"name": "测试公司", "open_department_id": "0", "parent_department_id": ""},
					},
				},
			})
		case "/open-apis/contact/v3/departments/0/children":
			if r.Header.Get("Authorization") != "Bearer tenant-token" {
				t.Errorf("unexpected authorization header: %s", r.Header.Get("Authorization"))
			}
			query := r.URL.Query()
			if query.Get("department_id_type") != "open_department_id" || query.Get("fetch_child") != "true" || query.Get("page_size") != "50" {
				t.Errorf("unexpected department query: %s", r.URL.RawQuery)
			}
			if query.Get("page_token") == "" {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"code": 0,
					"data": map[string]any{
						"has_more":   true,
						"page_token": "next-page",
						"items": []any{
							map[string]any{"name": "产品中心", "open_department_id": "od_product", "parent_department_id": "0", "order": "100", "member_count": 20, "status": map[string]bool{"is_deleted": false}},
						},
					},
				})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": 0,
				"data": map[string]any{
					"has_more": false,
					"items": []any{
						map[string]any{"name": "研发部", "open_department_id": "od_engineering", "parent_department_id": "od_product", "order": "200", "member_count": 8, "status": map[string]bool{"is_deleted": false}},
						map[string]any{"name": "已删除部门", "open_department_id": "od_deleted", "parent_department_id": "0", "status": map[string]bool{"is_deleted": true}},
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := New("cli_test", "secret", "")
	client.baseURL = server.URL
	client.http = server.Client()
	departments, err := client.ListContactDepartments(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(departments) != 2 {
		t.Fatalf("expected two active departments, got %#v", departments)
	}
	if departments[0].OpenDepartmentID != "od_product" || departments[0].Name != "产品中心" {
		t.Fatalf("unexpected first department: %#v", departments[0])
	}
	if departments[1].ParentDepartmentID != "od_product" || departments[1].Name != "研发部" {
		t.Fatalf("unexpected nested department: %#v", departments[1])
	}
}

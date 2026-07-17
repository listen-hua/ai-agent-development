package feishu

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExtractFolderToken(t *testing.T) {
	for _, input := range []string{
		"fldcnFolderToken",
		"https://example.feishu.cn/drive/folder/fldcnFolderToken?from=from_copylink",
	} {
		token, err := ExtractFolderToken(input)
		if err != nil || token != "fldcnFolderToken" {
			t.Fatalf("ExtractFolderToken(%q) = %q, %v", input, token, err)
		}
	}
	if _, err := ExtractFolderToken("https://example.feishu.cn/wiki/wikcnToken"); err == nil {
		t.Fatal("expected wiki URL to be rejected")
	}
}

func TestDriveListAndFetchDocx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/open-apis/auth/v3/tenant_access_token/internal/":
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "tenant_access_token": "tenant-token", "expire": 7200})
		case "/open-apis/drive/v1/files":
			if r.Header.Get("Authorization") != "Bearer tenant-token" || r.URL.Query().Get("folder_token") != "fld_root" {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{"files": []map[string]string{{"token": "doccn1", "name": "考勤制度", "type": "docx", "url": "https://example/docx/doccn1"}}, "has_more": false}})
		case "/open-apis/docx/v1/documents/doccn1/raw_content":
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]string{"content": "迟到与请假制度正文"}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := New("cli_test", "secret", "")
	client.baseURL = server.URL
	client.http = server.Client()
	items, err := client.ListDriveFolder(context.Background(), "fld_root")
	if err != nil || len(items) != 1 {
		t.Fatalf("ListDriveFolder returned %#v, %v", items, err)
	}
	file, err := client.FetchDriveItem(context.Background(), items[0])
	if err != nil {
		t.Fatal(err)
	}
	if string(file.Data) != "迟到与请假制度正文" || file.MimeType != "text/plain; charset=utf-8" {
		t.Fatalf("unexpected file: %#v", file)
	}
}

func TestDriveExportSheet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/open-apis/auth/v3/tenant_access_token/internal/":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "tenant_access_token": "tenant-token", "expire": 7200})
		case "/open-apis/drive/v1/export_tasks":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]string{"ticket": "ticket-1"}})
		case "/open-apis/drive/v1/export_tasks/ticket-1":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{"result": map[string]any{"file_token": "exported-1", "job_status": 0, "job_error_msg": "success"}}})
		case "/open-apis/drive/v1/export_tasks/file/exported-1/download":
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write([]byte("xlsx-data"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := New("cli_test", "secret", "")
	client.baseURL = server.URL
	client.http = server.Client()
	file, err := client.FetchDriveItem(context.Background(), DriveItem{Token: "shtcn1", Name: "行政台账", Type: "sheet"})
	if err != nil {
		t.Fatal(err)
	}
	if string(file.Data) != "xlsx-data" || file.Name != "行政台账.xlsx" || file.MimeType != "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" {
		t.Fatalf("unexpected exported file: %#v", file)
	}
}

func TestExtractWikiNodeToken(t *testing.T) {
	for _, input := range []string{
		"wikcnNodeToken",
		"https://example.feishu.cn/wiki/wikcnNodeToken?from=from_copylink",
	} {
		token, err := ExtractWikiNodeToken(input)
		if err != nil || token != "wikcnNodeToken" {
			t.Fatalf("ExtractWikiNodeToken(%q) = %q, %v", input, token, err)
		}
	}
	if _, err := ExtractWikiNodeToken("https://example.feishu.cn/drive/folder/fldcnToken"); err == nil {
		t.Fatal("expected drive folder URL to be rejected")
	}
}

func TestWikiGetNodeAndListChildren(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/open-apis/auth/v3/tenant_access_token/internal/":
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "tenant_access_token": "tenant-token", "expire": 7200})
		case "/open-apis/wiki/v2/spaces/get_node":
			if r.Header.Get("Authorization") != "Bearer tenant-token" || r.URL.Query().Get("token") != "wik-root" || r.URL.Query().Get("obj_type") != "wiki" {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{"node": map[string]any{
				"space_id": "space-1", "node_token": "wik-root", "obj_token": "docx-root", "obj_type": "docx", "title": "行政制度", "has_child": true,
			}}})
		case "/open-apis/wiki/v2/spaces/space-1/nodes":
			if r.URL.Query().Get("parent_node_token") != "wik-root" || r.URL.Query().Get("page_size") != "50" {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{
				"items":    []map[string]any{{"space_id": "space-1", "node_token": "wik-child", "obj_token": "docx-child", "obj_type": "docx", "title": "请假制度"}},
				"has_more": false,
			}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := New("cli_test", "secret", "")
	client.baseURL = server.URL
	client.http = server.Client()
	root, err := client.GetWikiNode(context.Background(), "wik-root")
	if err != nil || root.ObjToken != "docx-root" || !root.HasChild {
		t.Fatalf("GetWikiNode returned %#v, %v", root, err)
	}
	children, err := client.ListWikiNodes(context.Background(), root.SpaceID, root.NodeToken)
	if err != nil || len(children) != 1 || children[0].NodeToken != "wik-child" {
		t.Fatalf("ListWikiNodes returned %#v, %v", children, err)
	}
}

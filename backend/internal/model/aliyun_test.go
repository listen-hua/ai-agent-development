package model

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAliyunStreamGenerateEmitsDeltas(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" || r.Header.Get("Authorization") != "Bearer test-key" {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload["stream"] != true {
			t.Fatalf("expected stream=true, got %#v", payload["stream"])
		}
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		for _, content := range []string{"你", "好", "，世界"} {
			fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":%q}}]}\n\n", content)
			flusher.Flush()
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer server.Close()

	provider := NewAliyun("test-key", server.URL)
	parts := make([]string, 0)
	answer, err := provider.StreamGenerate(context.Background(), GenerateRequest{Model: "qwen-plus", Messages: []Message{{Role: "user", Content: "你好"}}}, func(delta string) {
		parts = append(parts, delta)
	})
	if err != nil {
		t.Fatal(err)
	}
	if answer != "你好，世界" || strings.Join(parts, "") != answer || len(parts) != 3 {
		t.Fatalf("unexpected stream result: answer=%q parts=%#v", answer, parts)
	}
}

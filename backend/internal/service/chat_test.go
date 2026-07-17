package service

import (
	"context"
	"internal-ai-agent/backend/internal/domain"
	"internal-ai-agent/backend/internal/model"
	"internal-ai-agent/backend/internal/store"
	"testing"
	"time"
)

func TestChatReturnsGroundedCitation(t *testing.T) {
	ctx := context.Background()
	repo := store.NewMemory(domain.AgentConfig{GenerationModel: "mock", EmbeddingModel: "mock", RerankModel: "mock", RetrievalTopK: 10, RerankTopN: 5, MaxOutputTokens: 500, SystemPrompt: "制度助手"})
	admin, _ := repo.GetUser(ctx, store.DemoAdminID)
	now := time.Now()
	version := domain.DocumentVersion{ID: "version", Version: "1.0", Status: "published", CreatedAt: now}
	doc := domain.Document{ID: "document", Title: "休假制度", ACL: domain.ACL{Scope: "all"}, Status: "published", Versions: []domain.DocumentVersion{version}, CreatedAt: now, UpdatedAt: now}
	_ = repo.CreateDocument(ctx, doc, []domain.Chunk{{ID: "chunk", DocumentID: doc.ID, VersionID: version.ID, Content: "年假需要提前三个工作日申请"}})
	hub := NewRunHub()
	chat := NewChat(repo, model.Mock{}, hub)
	conv, err := chat.CreateConversation(ctx, admin)
	if err != nil {
		t.Fatal(err)
	}
	runID, err := chat.Start(ctx, admin, conv.ID, "年假提前多久申请")
	if err != nil {
		t.Fatal(err)
	}
	_, events, ok := chat.Subscribe(runID)
	if !ok {
		t.Fatal("run not found")
	}
	done := false
	deltaText := ""
	for event := range events {
		if event.Type == "delta" {
			deltaText += event.Delta
		}
		if event.Type == "done" {
			done = true
			if event.Message == nil || len(event.Message.Citations) != 1 {
				t.Fatalf("expected one citation: %#v", event.Message)
			}
			if deltaText != event.Message.Content {
				t.Fatalf("streamed text %q does not match final answer %q", deltaText, event.Message.Content)
			}
		}
	}
	if !done {
		t.Fatal("run did not complete")
	}
}

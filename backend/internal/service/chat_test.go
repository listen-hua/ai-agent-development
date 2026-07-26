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

func TestConversationContextResolvesMeetingClarificationFromPreviousTurn(t *testing.T) {
	ctx := context.Background()
	repo := store.NewMemory(domain.AgentConfig{})
	user, _ := repo.GetUser(ctx, store.DemoEmployeeID)
	engine := NewConversationContextEngine(repo, model.Mock{})
	chat := NewChat(repo, model.Mock{}, NewRunHub())
	conversation, err := chat.CreateConversation(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	original := "帮我看下1号会议室有没有人，没人帮我预约下，时间从16点30开始，预计17点结束"
	if err = repo.AddMessage(ctx, domain.Message{ID: "original", ConversationID: conversation.ID, Role: "user", Content: original, CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err = repo.AddMessage(ctx, domain.Message{ID: "clarification", ConversationID: conversation.ID, Role: "assistant", Content: "请告诉我要预约或操作哪一天的会议室，例如“明天下午 3 点”。", CreatedAt: now.Add(time.Second)}); err != nil {
		t.Fatal(err)
	}
	if err = repo.AddMessage(ctx, domain.Message{ID: "answer", ConversationID: conversation.ID, Role: "user", Content: "今天的", CreatedAt: now.Add(2 * time.Second)}); err != nil {
		t.Fatal(err)
	}
	turn, err := engine.Analyze(ctx, user, conversation.ID, "answer", "今天的", domain.AgentConfig{ContextBudget: 12000})
	if err != nil {
		t.Fatal(err)
	}
	expected := original + "；用户补充：今天的"
	if turn.Understanding.Intent != contextIntentMeeting {
		t.Fatalf("expected meeting intent, got %q", turn.Understanding.Intent)
	}
	if turn.Understanding.StandaloneQuery != expected {
		t.Fatalf("expected %q, got %q", expected, turn.Understanding.StandaloneQuery)
	}
}

func TestValidateCurrentCitationsRejectsInvalidAndRenumbersUsedEvidence(t *testing.T) {
	citations := []domain.Citation{{ID: "cite_1", Title: "制度一"}, {ID: "cite_2", Title: "制度二"}}
	answer, filtered := validateCurrentCitations("结论来自第二份制度。[2]", citations)
	if answer != "结论来自第二份制度。[1]" {
		t.Fatalf("expected citation to be renumbered, got %q", answer)
	}
	if len(filtered) != 1 || filtered[0].Title != "制度二" || filtered[0].ID != "cite_1" {
		t.Fatalf("unexpected filtered citations: %#v", filtered)
	}
	answer, filtered = validateCurrentCitations("无引用结论", citations)
	if answer != noAnswer || filtered != nil {
		t.Fatal("answer without a current citation must be rejected")
	}
	answer, filtered = validateCurrentCitations("错误引用。[3]", citations)
	if answer != noAnswer || filtered != nil {
		t.Fatal("out-of-range citation must be rejected")
	}
}

package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"internal-ai-agent/backend/internal/domain"
	"internal-ai-agent/backend/internal/integration/feishu"
	"internal-ai-agent/backend/internal/model"
	"internal-ai-agent/backend/internal/store"
)

type fakeFeishuSender struct {
	messages chan string
}

func (f *fakeFeishuSender) Configured() bool { return true }
func (f *fakeFeishuSender) SendText(_ context.Context, _, _, text, _ string) (string, error) {
	f.messages <- text
	return "om_answer", nil
}

func TestFeishuBotQueuesOnceAndAddsAppLink(t *testing.T) {
	ctx := context.Background()
	repo := store.NewMemory(domain.AgentConfig{GenerationModel: "mock", EmbeddingModel: "mock", RerankModel: "mock", RetrievalTopK: 10, RerankTopN: 5, MaxOutputTokens: 500, SystemPrompt: "制度助手"})
	now := time.Now()
	version := domain.DocumentVersion{ID: "version", Version: "1.0", Status: "published", CreatedAt: now}
	doc := domain.Document{ID: "document", Title: "休假制度", ACL: domain.ACL{Scope: "all"}, Status: "published", Versions: []domain.DocumentVersion{version}, CreatedAt: now, UpdatedAt: now}
	if err := repo.CreateDocument(ctx, doc, []domain.Chunk{{ID: "chunk", DocumentID: doc.ID, VersionID: version.ID, Content: "年假需要提前三个工作日申请"}}); err != nil {
		t.Fatal(err)
	}
	sender := &fakeFeishuSender{messages: make(chan string, 2)}
	bot := NewFeishuBot(repo, NewChat(repo, model.Mock{}, NewRunHub()), sender, "https://applink.example/chat")
	event := feishu.MessageEvent{EventID: "evt_once", OpenID: "ou_bot_test", ChatID: "oc_chat", MessageType: "text", Content: `{"text":"@_user_1 年假提前多久申请"}`, MentionKeys: []string{"@_user_1"}}
	if err := bot.HandleMessage(ctx, event); err != nil {
		t.Fatal(err)
	}
	select {
	case answer := <-sender.messages:
		if !strings.Contains(answer, "https://applink.example/chat") {
			t.Fatalf("answer does not include app link: %s", answer)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for feishu answer")
	}
	if err := bot.HandleMessage(ctx, event); err != nil {
		t.Fatal(err)
	}
	select {
	case duplicate := <-sender.messages:
		t.Fatalf("duplicate event sent another answer: %s", duplicate)
	case <-time.After(150 * time.Millisecond):
	}
}

func TestParseFeishuTextRejectsInvalidJSON(t *testing.T) {
	if _, err := parseFeishuText("not-json", nil); err == nil {
		t.Fatal("expected invalid content to fail")
	}
}

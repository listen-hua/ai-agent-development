package service

import (
	"context"
	"testing"
	"time"

	"internal-ai-agent/backend/internal/domain"
	"internal-ai-agent/backend/internal/model"
	"internal-ai-agent/backend/internal/store"
)

func TestConversationContextRewritesPolicyFollowUp(t *testing.T) {
	ctx := context.Background()
	repo := store.NewMemory(domain.AgentConfig{})
	user, _ := repo.GetUser(ctx, store.DemoEmployeeID)
	chat := NewChat(repo, model.Mock{}, NewRunHub())
	conversation, err := chat.CreateConversation(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	addContextTestMessage(t, repo, domain.Message{ID: "policy-question", ConversationID: conversation.ID, Role: "user", Content: "年假如何申请", CreatedAt: now})
	addContextTestMessage(t, repo, domain.Message{ID: "policy-answer", ConversationID: conversation.ID, Role: "assistant", Content: "请查看年假制度。", CreatedAt: now.Add(time.Second)})
	addContextTestMessage(t, repo, domain.Message{ID: "policy-follow-up", ConversationID: conversation.ID, Role: "user", Content: "那试用期员工呢", CreatedAt: now.Add(2 * time.Second)})

	engine := NewConversationContextEngine(repo, model.Mock{})
	turn, err := engine.Analyze(ctx, user, conversation.ID, "policy-follow-up", "那试用期员工呢", domain.AgentConfig{ContextBudget: 12000})
	if err != nil {
		t.Fatal(err)
	}
	if turn.Understanding.Intent != contextIntentPolicy {
		t.Fatalf("expected policy intent, got %q", turn.Understanding.Intent)
	}
	if turn.Understanding.StandaloneQuery != "年假如何申请；用户补充：那试用期员工呢" {
		t.Fatalf("unexpected standalone query: %q", turn.Understanding.StandaloneQuery)
	}
	if !turn.ContextFailed {
		t.Fatal("mock provider should exercise the safe context-model fallback")
	}
}

func TestConversationContextRemovesRevokedHistoricalAnswer(t *testing.T) {
	ctx := context.Background()
	repo := store.NewMemory(domain.AgentConfig{})
	user, _ := repo.GetUser(ctx, store.DemoEmployeeID)
	now := time.Now()
	version := domain.DocumentVersion{ID: "version-revoked", Version: "1", Status: "published", CreatedAt: now}
	document := domain.Document{
		ID: "document-revoked", Title: "受限制度", Status: "published",
		ACL:      domain.ACL{Scope: "restricted", Rules: []domain.ACLRule{{UserIDs: []string{user.ID}}}},
		Versions: []domain.DocumentVersion{version}, CreatedAt: now, UpdatedAt: now,
	}
	if err := repo.CreateDocument(ctx, document, nil); err != nil {
		t.Fatal(err)
	}
	message := domain.Message{
		ID: "cited-answer", Role: "assistant", Content: "这里包含受限制度内容。",
		Citations: []domain.Citation{{DocumentID: document.ID, VersionID: version.ID}},
	}
	document.ACL = domain.ACL{Scope: "restricted", Rules: []domain.ACLRule{{UserIDs: []string{"another-user"}}}}
	if err := repo.UpdateDocument(ctx, document); err != nil {
		t.Fatal(err)
	}

	engine := NewConversationContextEngine(repo, model.Mock{})
	sanitized := engine.sanitizeHistory(ctx, user, []domain.Message{message})
	if len(sanitized) != 1 || sanitized[0].Content != "[历史回答因当前权限变化不可用于上下文]" {
		t.Fatalf("revoked answer was not sanitized: %#v", sanitized)
	}
	if len(sanitized[0].Citations) != 0 {
		t.Fatal("revoked citations must not remain in model context")
	}
}

func TestConversationContextResetCreatesHistoryBoundary(t *testing.T) {
	ctx := context.Background()
	repo := store.NewMemory(domain.AgentConfig{})
	user, _ := repo.GetUser(ctx, store.DemoEmployeeID)
	chat := NewChat(repo, model.Mock{}, NewRunHub())
	conversation, err := chat.CreateConversation(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	addContextTestMessage(t, repo, domain.Message{ID: "before-reset-user", ConversationID: conversation.ID, Role: "user", Content: "年假如何申请", CreatedAt: now})
	addContextTestMessage(t, repo, domain.Message{ID: "before-reset-assistant", ConversationID: conversation.ID, Role: "assistant", Content: "旧会话回答", CreatedAt: now.Add(time.Second)})
	engine := NewConversationContextEngine(repo, model.Mock{})
	if err = engine.Reset(ctx, conversation.ID); err != nil {
		t.Fatal(err)
	}
	addContextTestMessage(t, repo, domain.Message{ID: "after-reset", ConversationID: conversation.ID, Role: "user", Content: "那试用期员工呢", CreatedAt: now.Add(2 * time.Second)})
	turn, err := engine.Analyze(ctx, user, conversation.ID, "after-reset", "那试用期员工呢", domain.AgentConfig{ContextBudget: 12000})
	if err != nil {
		t.Fatal(err)
	}
	if turn.Understanding.StandaloneQuery != "那试用期员工呢" {
		t.Fatalf("reset leaked old history into standalone query: %q", turn.Understanding.StandaloneQuery)
	}
}

func TestConversationTurnTicketsRunInReservationOrder(t *testing.T) {
	locks := newConversationLocks()
	first := locks.Reserve("conversation")
	second := locks.Reserve("conversation")
	order := make(chan string, 2)
	go func() {
		second.Wait()
		order <- "second"
		second.Done()
	}()
	go func() {
		first.Wait()
		order <- "first"
		time.Sleep(10 * time.Millisecond)
		first.Done()
	}()
	if got := <-order; got != "first" {
		t.Fatalf("expected first ticket first, got %q", got)
	}
	if got := <-order; got != "second" {
		t.Fatalf("expected second ticket second, got %q", got)
	}
}

func addContextTestMessage(t *testing.T, repo store.Repository, message domain.Message) {
	t.Helper()
	if err := repo.AddMessage(context.Background(), message); err != nil {
		t.Fatal(err)
	}
}

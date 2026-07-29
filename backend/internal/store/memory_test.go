package store

import (
	"context"
	"testing"
	"time"

	"internal-ai-agent/backend/internal/domain"
)

func TestUpsertUserPreservesExistingRoles(t *testing.T) {
	repo := NewMemory(domain.AgentConfig{})
	updated, err := repo.UpsertUser(context.Background(), domain.User{
		FeishuOpenID: "ou_demo_admin",
		Name:         "管理员新名称",
		Roles:        []domain.Role{domain.RoleEmployee},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !updated.HasRole(domain.RoleSuperAdmin) {
		t.Fatal("identity refresh removed the existing administrator role")
	}
	if len(updated.DepartmentIDs) == 0 {
		t.Fatal("identity refresh removed existing departments")
	}
}

func TestListMessagesReturnsEmptySlice(t *testing.T) {
	repo := NewMemory(domain.AgentConfig{})
	messages, err := repo.ListMessages(context.Background(), "empty")
	if err != nil {
		t.Fatal(err)
	}
	if messages == nil || len(messages) != 0 {
		t.Fatalf("expected a non-nil empty slice, got %#v", messages)
	}
}

func TestSearchChunksAppliesDepartmentAndJobTitleACL(t *testing.T) {
	repo := NewMemory(domain.AgentConfig{})
	now := time.Now()
	doc := domain.Document{ID: "doc-finance", Title: "会计制度", Status: "published", ACL: domain.ACL{Scope: "restricted", Rules: []domain.ACLRule{{DepartmentIDs: []string{"dept_finance"}, JobTitles: []string{"会计"}}}}, Versions: []domain.DocumentVersion{{ID: "ver-finance", Version: "1.0", Status: "published"}}, CreatedAt: now, UpdatedAt: now}
	chunk := domain.Chunk{ID: "chunk-finance", DocumentID: doc.ID, VersionID: "ver-finance", Content: "会计报销制度"}
	if err := repo.CreateDocument(context.Background(), doc, []domain.Chunk{chunk}); err != nil {
		t.Fatal(err)
	}
	allowed := domain.User{ID: "accountant", DepartmentIDs: []string{"dept_finance"}, JobTitle: "会计", Roles: []domain.Role{domain.RoleEmployee}}
	denied := domain.User{ID: "manager", DepartmentIDs: []string{"dept_finance"}, JobTitle: "财务经理", Roles: []domain.Role{domain.RoleEmployee}}
	chunks, _, err := repo.SearchChunks(context.Background(), allowed, "会计", nil, 8)
	if err != nil || len(chunks) != 1 {
		t.Fatalf("expected authorized chunk, got %d, error %v", len(chunks), err)
	}
	chunks, _, err = repo.SearchChunks(context.Background(), denied, "会计", nil, 8)
	if err != nil || len(chunks) != 0 {
		t.Fatalf("expected ACL to hide chunk, got %d, error %v", len(chunks), err)
	}
}

func TestUpdateUserRolesKeepsLastSuperAdmin(t *testing.T) {
	repo := NewMemory(domain.AgentConfig{})
	if _, err := repo.UpdateUserRoles(context.Background(), DemoAdminID, []domain.Role{domain.RoleEmployee}); err != ErrConflict {
		t.Fatalf("expected ErrConflict when removing the final super admin, got %v", err)
	}
}

func TestBoundConversationIsIsolatedAndExpires(t *testing.T) {
	ctx := context.Background()
	repo := NewMemory(domain.AgentConfig{})
	firstUser, _ := repo.GetUser(ctx, DemoEmployeeID)
	secondUser, _ := repo.GetUser(ctx, DemoAdminID)
	now := time.Now()
	ttl := 30 * time.Minute

	first, err := repo.GetOrCreateBoundConversation(ctx, firstUser, domain.AgentAdministrativeAssistant, domain.ConversationChannelFeishuBot, "chat-a", now, ttl)
	if err != nil {
		t.Fatal(err)
	}
	same, _ := repo.GetOrCreateBoundConversation(ctx, firstUser, domain.AgentAdministrativeAssistant, domain.ConversationChannelFeishuBot, "chat-a", now.Add(time.Minute), ttl)
	if same.ID != first.ID {
		t.Fatal("same user, chat and agent should reuse the active conversation")
	}
	otherChat, _ := repo.GetOrCreateBoundConversation(ctx, firstUser, domain.AgentAdministrativeAssistant, domain.ConversationChannelFeishuBot, "chat-b", now, ttl)
	otherUser, _ := repo.GetOrCreateBoundConversation(ctx, secondUser, domain.AgentAdministrativeAssistant, domain.ConversationChannelFeishuBot, "chat-a", now, ttl)
	otherAgent, _ := repo.GetOrCreateBoundConversation(ctx, firstUser, "image_agent", domain.ConversationChannelFeishuBot, "chat-a", now, ttl)
	if otherChat.ID == first.ID || otherUser.ID == first.ID || otherAgent.ID == first.ID {
		t.Fatal("bindings must be isolated by user, chat and agent")
	}
	expired, _ := repo.GetOrCreateBoundConversation(ctx, firstUser, domain.AgentAdministrativeAssistant, domain.ConversationChannelFeishuBot, "chat-a", now.Add(32*time.Minute), ttl)
	if expired.ID == first.ID {
		t.Fatal("an inactive binding should start a new conversation")
	}
}

func TestMessageFeedbackAndMetricsUseStoredData(t *testing.T) {
	ctx := context.Background()
	repo := NewMemory(domain.AgentConfig{})
	now := time.Now()
	conversation := domain.Conversation{ID: "conversation", UserID: DemoEmployeeID, Title: "测试", CreatedAt: now, UpdatedAt: now}
	if err := repo.CreateConversation(ctx, conversation); err != nil {
		t.Fatal(err)
	}
	if err := repo.AddMessage(ctx, domain.Message{ID: "question", ConversationID: conversation.ID, Role: "user", Content: "问题", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := repo.AddMessage(ctx, domain.Message{ID: "answer", ConversationID: conversation.ID, Role: "assistant", Content: "答案", Citations: []domain.Citation{{ID: "cite"}}, CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := repo.RecordMessageFeedback(ctx, "answer", DemoEmployeeID, true); err != nil {
		t.Fatal(err)
	}
	if err := repo.RecordMessageFeedback(ctx, "answer", DemoAdminID, false); err != ErrNotFound {
		t.Fatalf("another user must not be able to update feedback, got %v", err)
	}
	metrics := repo.Metrics(ctx)
	if metrics.QuestionsToday != 1 || metrics.PositiveRate != 1 || metrics.CitationCoverage != 1 || metrics.NoAnswerRate != 0 {
		t.Fatalf("unexpected metrics: %#v", metrics)
	}
}

func TestUpsertIAMUserMergesWithExistingFeishuIdentity(t *testing.T) {
	repo := NewMemory(domain.AgentConfig{})
	existing, err := repo.UpsertUser(context.Background(), domain.User{
		FeishuOpenID: "ou_existing",
		Name:         "Existing",
		Status:       "active",
		Roles:        []domain.Role{domain.RoleEmployee, domain.RoleKnowledgeAdmin},
	})
	if err != nil {
		t.Fatal(err)
	}
	iamUserID := int64(18)
	merged, err := repo.UpsertIAMUser(context.Background(), domain.User{
		FeishuOpenID: "ou_existing",
		FeishuUserID: "feishu-user-18",
		IAMUserID:    &iamUserID,
		Name:         "Updated",
		Status:       "active",
	})
	if err != nil {
		t.Fatal(err)
	}
	if merged.ID != existing.ID || !merged.HasRole(domain.RoleKnowledgeAdmin) {
		t.Fatalf("IAM identity did not preserve the Feishu account: %#v", merged)
	}
	byIAM, err := repo.GetUserByIAMID(context.Background(), iamUserID)
	if err != nil || byIAM.ID != existing.ID {
		t.Fatalf("IAM lookup failed: %#v error=%v", byIAM, err)
	}
}

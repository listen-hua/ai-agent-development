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

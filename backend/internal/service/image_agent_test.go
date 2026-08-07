package service

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"internal-ai-agent/backend/internal/blob"
	"internal-ai-agent/backend/internal/domain"
	"internal-ai-agent/backend/internal/security"
	"internal-ai-agent/backend/internal/store"
)

func TestEnsureDefaultsAllowsKnownComflyImageHosts(t *testing.T) {
	ctx := context.Background()
	repo := store.NewMemory(domain.AgentConfig{})
	agent, err := NewImageAgent(repo, repo, blob.Noop{}, security.NoopScanner{}, "test-image-secret")
	if err != nil {
		t.Fatal(err)
	}
	if err = agent.EnsureDefaults(ctx); err != nil {
		t.Fatal(err)
	}
	relays, err := repo.ListImageRelays(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, relay := range relays {
		if relay.RelayKey != "comfly" {
			continue
		}
		if !slices.Contains(relay.AllowedOutputHosts, "webstatic.apiproxy.vip") {
			t.Fatalf("Comfly output host allowlist was not initialized: %#v", relay.AllowedOutputHosts)
		}
		return
	}
	t.Fatal("Comfly relay was not initialized")
}

func TestValidateCartoonStrength(t *testing.T) {
	valid := []string{
		`portrait`,
		`{"cartoonization_strength": 0}`,
		`{"cartoonization_strength": 0.75}`,
	}
	for _, value := range valid {
		if err := ValidateCartoonStrength(value); err != nil {
			t.Fatalf("%s should be valid: %v", value, err)
		}
	}
	invalid := []string{
		`{"cartoonization_strength": 2}`,
		`{"cartoonization_strength": "0.5"}`,
		`{"cartoonization_strength": 0, "cartoonization_strength": 1}`,
	}
	for _, value := range invalid {
		if err := ValidateCartoonStrength(value); err == nil {
			t.Fatalf("%s should be invalid", value)
		}
	}
}

func TestSaveProjectReturnsSpecificValidationErrors(t *testing.T) {
	repo := store.NewMemory(domain.AgentConfig{})
	agent, err := NewImageAgent(repo, repo, blob.Noop{}, security.NoopScanner{}, "test-image-secret")
	if err != nil {
		t.Fatal(err)
	}
	_, err = agent.SaveProject(context.Background(), domain.User{}, "", ImageProjectInput{
		ProjectKey: "1invalid", Name: "活动图", ACL: domain.ACL{Scope: "all"}, Enabled: true,
	})
	if err == nil || !strings.Contains(err.Error(), "项目标识需以") {
		t.Fatalf("expected a specific project key error, got %v", err)
	}
	_, err = agent.SaveProject(context.Background(), domain.User{}, "", ImageProjectInput{
		ProjectKey: "campaign_2026", Name: " ", ACL: domain.ACL{Scope: "all"}, Enabled: true,
	})
	if err == nil || err.Error() != "项目名称不能为空" {
		t.Fatalf("expected a specific project name error, got %v", err)
	}
	value, err := agent.SaveProject(context.Background(), domain.User{}, "", ImageProjectInput{
		ProjectKey: "宣传图项目", Name: "宣传图生成", ACL: domain.ACL{Scope: "all"}, Enabled: true,
	})
	if err != nil {
		t.Fatalf("expected Chinese project key and name to be accepted, got %v", err)
	}
	if value.ProjectKey != "宣传图项目" || value.Name != "宣传图生成" {
		t.Fatalf("unexpected Chinese project values: %#v", value)
	}
}

func TestNodeDimensions(t *testing.T) {
	width, height := nodeDimensions("16:9")
	if width != 360 || height >= width {
		t.Fatalf("unexpected dimensions %v x %v", width, height)
	}
}

func TestPlaceholderNodesUseSelectedAnchorAndAvoidOccupiedSpace(t *testing.T) {
	canvas := domain.ImageCanvas{
		ID: "canvas-placement",
		Nodes: []domain.ImageCanvasNode{
			{ID: "anchor", X: 100, Y: 80, Width: 360, Height: 360, ZIndex: 1},
			{ID: "occupied", X: 484, Y: 80, Width: 360, Height: 360, ZIndex: 2},
		},
	}
	job := domain.ImageJob{ID: "job-placement", AspectRatio: "1:1", Count: 1}

	nodes := placeholderNodes(canvas, job, "anchor", 5000, 5000)

	if len(nodes) != 1 {
		t.Fatalf("expected one placeholder, got %d", len(nodes))
	}
	if nodes[0].X != 868 || nodes[0].Y != 80 {
		t.Fatalf("expected occupied anchor row to shift right with a 24px gap, got x=%v y=%v", nodes[0].X, nodes[0].Y)
	}
	if nodes[0].ZIndex != 3 {
		t.Fatalf("expected placeholder above existing nodes, got z-index %d", nodes[0].ZIndex)
	}
}

func TestPlaceholderNodesFallBackToNearestViewportNode(t *testing.T) {
	canvas := domain.ImageCanvas{
		ID: "canvas-nearest",
		Nodes: []domain.ImageCanvasNode{
			{ID: "far", X: 0, Y: 0, Width: 100, Height: 100},
			{ID: "near", X: 900, Y: 500, Width: 200, Height: 120},
		},
	}
	job := domain.ImageJob{ID: "job-nearest", AspectRatio: "16:9", Count: 1}

	nodes := placeholderNodes(canvas, job, "not-on-this-canvas", 1000, 560)

	if nodes[0].X != 1124 || nodes[0].Y != 500 {
		t.Fatalf("expected nearest node fallback with top alignment, got x=%v y=%v", nodes[0].X, nodes[0].Y)
	}
}

func TestPlaceholderNodesCenterTwoColumnGroupOnEmptyCanvas(t *testing.T) {
	canvas := domain.ImageCanvas{ID: "canvas-empty"}
	job := domain.ImageJob{ID: "job-grid", AspectRatio: "1:1", Count: 4}

	nodes := placeholderNodes(canvas, job, "", 1000, 600)

	expected := [][2]float64{{628, 228}, {1012, 228}, {628, 612}, {1012, 612}}
	if len(nodes) != len(expected) {
		t.Fatalf("expected %d placeholders, got %d", len(expected), len(nodes))
	}
	for index, point := range expected {
		if nodes[index].X != point[0] || nodes[index].Y != point[1] {
			t.Fatalf("placeholder %d: expected (%v,%v), got (%v,%v)", index, point[0], point[1], nodes[index].X, nodes[index].Y)
		}
	}
}

func TestPlaceholderNodesKeepTwoColumnGridForSupportedCounts(t *testing.T) {
	for _, count := range []int{1, 2, 4, 8} {
		t.Run(fmt.Sprintf("%d-images", count), func(t *testing.T) {
			nodes := placeholderNodes(
				domain.ImageCanvas{ID: "canvas-grid"},
				domain.ImageJob{ID: "job-grid", AspectRatio: "1:1", Count: count},
				"", 1000, 1000,
			)
			if len(nodes) != count {
				t.Fatalf("expected %d placeholders, got %d", count, len(nodes))
			}
			for index, current := range nodes {
				if index > 0 && index%2 == 1 && current.X-nodes[index-1].X != 384 {
					t.Fatalf("horizontal gap for node %d is not 24", index)
				}
				if index >= 2 && current.Y-nodes[index-2].Y != 384 {
					t.Fatalf("vertical gap for node %d is not 24", index)
				}
			}
		})
	}
}

func TestPromptActionNameDoesNotRequireSeparateEmployeeLabel(t *testing.T) {
	if got := promptActionName("", "cartoonize", ""); got != "cartoonize" {
		t.Fatalf("new actions should default to action_key, got %q", got)
	}
	if got := promptActionName("", "cartoonize-v2", "卡通化"); got != "卡通化" {
		t.Fatalf("editing should preserve the existing display name, got %q", got)
	}
	if got := promptActionName("  新名称  ", "cartoonize", "旧名称"); got != "新名称" {
		t.Fatalf("an explicitly provided name should still be honored, got %q", got)
	}
}

func TestSavePromptActionAcceptsChineseButtonKey(t *testing.T) {
	ctx := context.Background()
	repo := store.NewMemory(domain.AgentConfig{})
	admin, _ := repo.GetUser(ctx, store.DemoAdminID)
	agent, _ := NewImageAgent(repo, repo, newImageTestBlob(), security.NoopScanner{}, "test-image-secret")

	action, err := agent.SavePromptAction(ctx, admin, "", ImagePromptActionInput{
		ActionKey: "卡通化_2", PromptTemplate: "将参考图转换为卡通风格", Enabled: true,
	})
	if err != nil {
		t.Fatalf("Chinese button keys should be accepted: %v", err)
	}
	if action.ActionKey != "卡通化_2" || action.Name != "卡通化_2" {
		t.Fatalf("unexpected action identity: %+v", action)
	}

	if _, err = agent.SavePromptAction(ctx, admin, "", ImagePromptActionInput{
		ActionKey: "-invalid", PromptTemplate: "prompt", Enabled: true,
	}); err == nil {
		t.Fatal("button keys must start with a letter")
	}
}

func TestPromptActionPreviewLifecycleAndAuthorization(t *testing.T) {
	ctx := context.Background()
	repo := store.NewMemory(domain.AgentConfig{})
	admin, _ := repo.GetUser(ctx, store.DemoAdminID)
	admin.Permissions = append([]domain.PermissionKey(nil), domain.AllPermissionKeys...)
	employee, _ := repo.GetUser(ctx, store.DemoEmployeeID)
	project := domain.ImageProject{
		ID: "project-preview", ProjectKey: "preview", Name: "Preview", ACL: domain.ACL{Scope: "all"},
		Enabled: true, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := repo.UpsertImageProject(ctx, project); err != nil {
		t.Fatal(err)
	}
	blobs := newImageTestBlob()
	agent, err := NewImageAgent(repo, repo, blobs, security.NoopScanner{}, "test-image-secret")
	if err != nil {
		t.Fatal(err)
	}
	action, err := agent.SavePromptAction(ctx, admin, "", ImagePromptActionInput{
		ActionKey: "preview-action", PromptTemplate: "portrait", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	png, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAIAAAABCAYAAAD0In+KAAAADUlEQVR42mNk+M/wHwAEAQH/5agzWQAAAABJRU5ErkJggg==")

	withPreview, err := agent.SavePromptActionPreview(ctx, admin, action.ID, ImagePromptActionPreviewInput{
		FileName: "preview.png", DeclaredMIME: "image/png", Data: png,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !withPreview.HasPreview || withPreview.PreviewMIMEType != "image/png" || withPreview.PreviewWidth != 2 || withPreview.PreviewHeight != 1 {
		t.Fatalf("unexpected preview metadata: %+v", withPreview)
	}
	if _, ok := blobs.objects[withPreview.PreviewObjectKey]; !ok {
		t.Fatal("preview was not written to object storage")
	}
	loaded, content, err := agent.PromptActionPreviewContent(ctx, employee, action.ID)
	if err != nil || loaded.ID != action.ID || string(content) != string(png) {
		t.Fatalf("employee should read an enabled global preview: action=%+v err=%v", loaded, err)
	}

	oldObjectKey := withPreview.PreviewObjectKey
	replaced, err := agent.SavePromptActionPreview(ctx, admin, action.ID, ImagePromptActionPreviewInput{
		FileName: "replacement.png", DeclaredMIME: "image/png", Data: png,
	})
	if err != nil {
		t.Fatal(err)
	}
	if replaced.PreviewObjectKey == oldObjectKey || !blobs.deleted[oldObjectKey] {
		t.Fatal("replacing a preview must delete the old object")
	}

	_, err = agent.SavePromptAction(ctx, admin, action.ID, ImagePromptActionInput{
		ActionKey: action.ActionKey, PromptTemplate: action.PromptTemplate, Enabled: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = agent.PromptActionPreviewContent(ctx, employee, action.ID); !errors.Is(err, store.ErrForbidden) {
		t.Fatalf("employee must not read a disabled action preview, got %v", err)
	}
	if _, _, err = agent.PromptActionPreviewContent(ctx, admin, action.ID); err != nil {
		t.Fatalf("image administrators should preview disabled actions: %v", err)
	}

	removed, err := agent.RemovePromptActionPreview(ctx, admin, action.ID)
	if err != nil {
		t.Fatal(err)
	}
	if removed.HasPreview || removed.PreviewObjectKey != "" || !blobs.deleted[replaced.PreviewObjectKey] {
		t.Fatalf("preview was not removed cleanly: %+v", removed)
	}
}

func TestPromptActionPreviewRejectsInvalidFiles(t *testing.T) {
	ctx := context.Background()
	repo := store.NewMemory(domain.AgentConfig{})
	admin, _ := repo.GetUser(ctx, store.DemoAdminID)
	agent, _ := NewImageAgent(repo, repo, newImageTestBlob(), security.NoopScanner{}, "test-image-secret")
	action, err := agent.SavePromptAction(ctx, admin, "", ImagePromptActionInput{
		ActionKey: "invalid-preview", PromptTemplate: "portrait", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = agent.SavePromptActionPreview(ctx, admin, action.ID, ImagePromptActionPreviewInput{
		FileName: "fake.png", DeclaredMIME: "image/png", Data: []byte("not an image"),
	}); err == nil {
		t.Fatal("content detection must reject a fake image")
	}
	if _, err = agent.SavePromptActionPreview(ctx, admin, action.ID, ImagePromptActionPreviewInput{
		FileName: "large.png", DeclaredMIME: "image/png", Data: make([]byte, (5<<20)+1),
	}); err == nil {
		t.Fatal("oversized preview must be rejected")
	}
}

type imageTestBlob struct {
	objects map[string][]byte
	deleted map[string]bool
}

func newImageTestBlob() *imageTestBlob {
	return &imageTestBlob{objects: map[string][]byte{}, deleted: map[string]bool{}}
}

func (b *imageTestBlob) Put(_ context.Context, key string, data []byte, _ string) error {
	b.objects[key] = append([]byte(nil), data...)
	return nil
}

func (b *imageTestBlob) Get(_ context.Context, key string) ([]byte, error) {
	data, ok := b.objects[key]
	if !ok {
		return nil, store.ErrNotFound
	}
	return append([]byte(nil), data...), nil
}

func (b *imageTestBlob) Delete(_ context.Context, key string) error {
	delete(b.objects, key)
	b.deleted[key] = true
	return nil
}

func TestImportCanvasAssetCreatesReadyCenteredNode(t *testing.T) {
	ctx := context.Background()
	repo := store.NewMemory(domain.AgentConfig{})
	user, err := repo.GetUser(ctx, store.DemoEmployeeID)
	if err != nil {
		t.Fatal(err)
	}
	project := domain.ImageProject{
		ID: "project-import", ProjectKey: "import", Name: "Import", ACL: domain.ACL{Scope: "all"},
		Enabled: true, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err = repo.UpsertImageProject(ctx, project); err != nil {
		t.Fatal(err)
	}
	agent, err := NewImageAgent(repo, repo, blob.Noop{}, security.NoopScanner{}, "test-image-secret")
	if err != nil {
		t.Fatal(err)
	}
	canvas, err := agent.Canvas(ctx, user, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	png, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAIAAAABCAYAAAD0In+KAAAADUlEQVR42mNk+M/wHwAEAQH/5agzWQAAAABJRU5ErkJggg==")
	if err != nil {
		t.Fatal(err)
	}
	updated, err := agent.ImportCanvasAsset(ctx, user, project.ID, ImageCanvasImport{
		FileName: "clipboard.png", DeclaredMIME: "image/png", Data: png,
		X: 1000, Y: 600, Version: canvas.Version, Origin: "paste",
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Version != canvas.Version+1 || len(updated.Nodes) != 1 {
		t.Fatalf("unexpected canvas version/nodes: version=%d nodes=%d", updated.Version, len(updated.Nodes))
	}
	node := updated.Nodes[0]
	if node.Status != "ready" || node.AssetID == "" || node.Width != 360 || node.Height != 180 {
		t.Fatalf("unexpected imported node: %+v", node)
	}
	if node.X != 820 || node.Y != 510 {
		t.Fatalf("target must represent the node center, got x=%v y=%v", node.X, node.Y)
	}
	asset, err := repo.GetImageAsset(ctx, node.AssetID)
	if err != nil {
		t.Fatal(err)
	}
	if asset.Source != "upload" || asset.Width != 2 || asset.Height != 1 {
		t.Fatalf("unexpected imported asset: %+v", asset)
	}
}

func TestImportCanvasAssetRejectsStaleVersion(t *testing.T) {
	ctx := context.Background()
	repo := store.NewMemory(domain.AgentConfig{})
	user, _ := repo.GetUser(ctx, store.DemoEmployeeID)
	project := domain.ImageProject{
		ID: "project-conflict", ProjectKey: "conflict", Name: "Conflict", ACL: domain.ACL{Scope: "all"},
		Enabled: true, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	_ = repo.UpsertImageProject(ctx, project)
	agent, _ := NewImageAgent(repo, repo, blob.Noop{}, security.NoopScanner{}, "test-image-secret")
	canvas, _ := agent.Canvas(ctx, user, project.ID)
	png, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAIAAAABCAYAAAD0In+KAAAADUlEQVR42mNk+M/wHwAEAQH/5agzWQAAAABJRU5ErkJggg==")
	input := ImageCanvasImport{FileName: "drop.png", DeclaredMIME: "image/png", Data: png, X: 10, Y: 20, Version: canvas.Version, Origin: "drop"}
	if _, err := agent.ImportCanvasAsset(ctx, user, project.ID, input); err != nil {
		t.Fatal(err)
	}
	if _, err := agent.ImportCanvasAsset(ctx, user, project.ID, input); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("expected stale version conflict, got %v", err)
	}
}

func TestImageCanvasLifecycleAndOwnership(t *testing.T) {
	ctx := context.Background()
	repo := store.NewMemory(domain.AgentConfig{})
	owner, _ := repo.GetUser(ctx, store.DemoEmployeeID)
	other, _ := repo.GetUser(ctx, store.DemoAdminID)
	agent, err := NewImageAgent(repo, repo, blob.Noop{}, security.NoopScanner{}, "test-image-secret")
	if err != nil {
		t.Fatal(err)
	}

	canvas, err := agent.CreateCanvas(ctx, owner, "  市场宣传图  ")
	if err != nil {
		t.Fatal(err)
	}
	if canvas.Name != "市场宣传图" || canvas.ProjectID != "" {
		t.Fatalf("unexpected new canvas: %+v", canvas)
	}
	if _, err = agent.CanvasByID(ctx, other, canvas.ID, false); !errors.Is(err, store.ErrForbidden) {
		t.Fatalf("another user must not read this canvas, got %v", err)
	}
	if _, err = agent.CreateCanvas(ctx, owner, "市场宣传图"); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("duplicate active canvas names must conflict, got %v", err)
	}

	if err = agent.DeleteCanvas(ctx, owner, canvas.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = agent.CanvasByID(ctx, owner, canvas.ID, false); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("deleted canvas must reject normal access, got %v", err)
	}
	trash, err := agent.Canvases(ctx, owner, true)
	if err != nil || len(trash) != 1 || trash[0].ID != canvas.ID {
		t.Fatalf("unexpected trash list: %+v, err=%v", trash, err)
	}

	if _, err = agent.CreateCanvas(ctx, owner, "市场宣传图"); err != nil {
		t.Fatal(err)
	}
	restored, err := agent.RestoreCanvas(ctx, owner, canvas.ID)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Name == "市场宣传图" || restored.DeletedAt != nil {
		t.Fatalf("restore should resolve an active name conflict: %+v", restored)
	}
}

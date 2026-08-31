package service

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"testing"
	"time"

	"internal-ai-agent/backend/internal/blob"
	"internal-ai-agent/backend/internal/domain"
	"internal-ai-agent/backend/internal/security"
	"internal-ai-agent/backend/internal/store"
)

func TestEnsureDefaultsAllowsKnownRelayImageHosts(t *testing.T) {
	ctx := context.Background()
	repo := store.NewMemory(domain.AgentConfig{})
	existingXGAPI := domain.ImageRelay{
		ID: "existing-xgapi", RelayKey: "xgapi", Name: "Configured XGAPI",
		BaseURL: "https://api.xgapiproxy.win/v1", AllowedOutputHosts: []string{"api.xgapiproxy.win"},
		EncryptedAPIKey: "keep-encrypted-key", CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := repo.UpsertImageRelay(ctx, existingXGAPI); err != nil {
		t.Fatal(err)
	}
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
	expectedHosts := map[string][]string{
		"xgapi":  {"api.xgapiproxy.win", "image.xgapiproxy.win", "webstatic.apiproxy.vip"},
		"comfly": {"ai.comfly.org", "files.closeai.fans", "webstatic.apiproxy.vip", "webstatic.aiproxy.vip"},
	}
	for relayKey, hosts := range expectedHosts {
		var relay domain.ImageRelay
		for _, candidate := range relays {
			if candidate.RelayKey == relayKey {
				relay = candidate
				break
			}
		}
		if relay.ID == "" {
			t.Fatalf("%s relay was not initialized", relayKey)
		}
		for _, host := range hosts {
			if !slices.Contains(relay.AllowedOutputHosts, host) {
				t.Fatalf("%s output host %q was not initialized: %#v", relayKey, host, relay.AllowedOutputHosts)
			}
		}
		if relayKey == "xgapi" && relay.EncryptedAPIKey != existingXGAPI.EncryptedAPIKey {
			t.Fatal("merging required hosts must preserve the configured API key")
		}
	}
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

func TestValidateImageJobPromptAllowsFifteenThousandCharacters(t *testing.T) {
	if err := validateImageJobPrompt("generate", strings.Repeat("图", imagePromptMaxLength)); err != nil {
		t.Fatalf("a %d-character prompt should be accepted: %v", imagePromptMaxLength, err)
	}
	if err := validateImageJobPrompt("generate", strings.Repeat("图", imagePromptMaxLength+1)); err == nil || !strings.Contains(err.Error(), "15000") {
		t.Fatalf("a prompt above the limit should be rejected with the configured limit, got %v", err)
	}
}

func TestCreateImageJobSnapshotsRelayAndModelOnCanvasNodes(t *testing.T) {
	ctx := context.Background()
	repo := store.NewMemory(domain.AgentConfig{})
	user, _ := repo.GetUser(ctx, store.DemoEmployeeID)
	now := time.Now()
	project := domain.ImageProject{ID: "project-source", ProjectKey: "source", Name: "来源测试", ACL: domain.ACL{Scope: "all"}, Enabled: true, CreatedAt: now, UpdatedAt: now}
	relay := domain.ImageRelay{ID: "relay-source", RelayKey: "source-relay", Name: "Comfly", BaseURL: "https://example.com/v1", Enabled: true, EncryptedAPIKey: "configured", CreatedAt: now, UpdatedAt: now}
	model := domain.ImageModel{ID: "model-source", RelayID: relay.ID, ModelID: "gpt-image-2", DisplayName: "GPT Image 2", Protocol: "gpt_image_2", Enabled: true, SupportedSizes: []string{"1K"}, MaxCount: 1, CreatedAt: now, UpdatedAt: now}
	if err := repo.UpsertImageProject(ctx, project); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpsertImageRelay(ctx, relay); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpsertImageModels(ctx, []domain.ImageModel{model}); err != nil {
		t.Fatal(err)
	}
	agent, _ := NewImageAgent(repo, repo, blob.Noop{}, security.NoopScanner{}, "test-image-secret")
	canvas, err := agent.CreateCanvas(ctx, user, "来源测试画布")
	if err != nil {
		t.Fatal(err)
	}
	created, err := agent.CreateJob(ctx, user, ImageJobInput{CanvasID: canvas.ID, ProjectID: project.ID, RelayID: relay.ID,
		ModelID: model.ID, Kind: "generate", Prompt: "a cat", AspectRatio: "1:1", ImageSize: "1K", Count: 1, IdempotencyKey: "source-snapshot"})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := agent.CanvasByID(ctx, user, canvas.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(updated.Nodes) != 1 {
		t.Fatalf("expected one generated node, got %+v", updated.Nodes)
	}
	node := updated.Nodes[0]
	if node.GenerationRelayName != relay.Name || node.GenerationModelName != model.DisplayName || node.GenerationModelKey != model.ModelID {
		t.Fatalf("generation source snapshot was not preserved: %+v", node)
	}

	relay.Name, model.DisplayName = "Renamed relay", "Renamed model"
	if err = repo.UpsertImageRelay(ctx, relay); err != nil {
		t.Fatal(err)
	}
	if err = repo.UpsertImageModels(ctx, []domain.ImageModel{model}); err != nil {
		t.Fatal(err)
	}
	unchanged, _ := agent.CanvasByID(ctx, user, canvas.ID, false)
	if unchanged.Nodes[0].GenerationRelayName != "Comfly" || unchanged.Nodes[0].GenerationModelName != "GPT Image 2" {
		t.Fatalf("historical source names must remain snapshots: %+v", unchanged.Nodes[0])
	}

	output := created.Outputs[0]
	output.RequestedSize, output.ActualWidth, output.ActualHeight, output.UpdatedAt = "1K", 1024, 1024, time.Now()
	asset := domain.ImageAsset{ID: "generated-source-asset", OwnerID: user.ID, ProjectID: project.ID, ObjectKey: "images/source.png", MIMEType: "image/png", FileName: "source.png", Width: 1024, Height: 1024, SizeBytes: 1024, Source: "generated", CreatedAt: time.Now()}
	if err = repo.SaveImageJobOutput(ctx, output, asset, domain.ImageCanvasNode{Width: 360, Height: 360}); err != nil {
		t.Fatal(err)
	}
	if _, err = agent.SavePixianConfig(ctx, user, PixianConfigInput{Enabled: true, TestMode: true, APIID: "test-id", APISecret: "test-secret", TimeoutSeconds: 180, Concurrency: 2, MaxPixels: 25_000_000}); err != nil {
		t.Fatal(err)
	}
	readyCanvas, _ := agent.CanvasByID(ctx, user, canvas.ID, false)
	if _, err = agent.CreateBackgroundRemovalJob(ctx, user, canvas.ID, BackgroundRemovalJobInput{NodeIDs: []string{readyCanvas.Nodes[0].ID}, Version: readyCanvas.Version, IdempotencyKey: "source-cutout"}); err != nil {
		t.Fatal(err)
	}
	withCutout, _ := agent.CanvasByID(ctx, user, canvas.ID, false)
	cutout := withCutout.Nodes[1]
	if cutout.GenerationRelayName != "Comfly" || cutout.GenerationModelName != "GPT Image 2" || cutout.GenerationModelKey != "gpt-image-2" {
		t.Fatalf("Pixian result must inherit the original generation source: %+v", cutout)
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

func TestDeleteUnusedProjectCleansPromptActionScopes(t *testing.T) {
	ctx := context.Background()
	repo := store.NewMemory(domain.AgentConfig{})
	blobs := newImageTestBlob()
	agent, err := NewImageAgent(repo, repo, blobs, security.NoopScanner{}, "test-image-secret")
	if err != nil {
		t.Fatal(err)
	}
	actor, _ := repo.GetUser(ctx, store.DemoAdminID)
	first, err := agent.SaveProject(ctx, actor, "", ImageProjectInput{
		ProjectKey: "unused-a", Name: "待删除项目", ACL: domain.ACL{Scope: "all"}, Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := agent.SaveProject(ctx, actor, "", ImageProjectInput{
		ProjectKey: "unused-b", Name: "保留项目", ACL: domain.ACL{Scope: "all"}, Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	single, err := agent.SavePromptAction(ctx, actor, "", ImagePromptActionInput{
		ActionKey: "only-first", PromptTemplate: "只属于待删除项目", ProjectIDs: []string{first.ID}, Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	multiple, err := agent.SavePromptAction(ctx, actor, "", ImagePromptActionInput{
		ActionKey: "shared", PromptTemplate: "两个项目共享", ProjectIDs: []string{first.ID, second.ID}, Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = agent.DeleteProject(ctx, actor, first.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.GetImageProject(ctx, first.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("deleted project must be removed, got %v", err)
	}
	if _, err = repo.GetImagePromptAction(ctx, single.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("single-project action must be removed, got %v", err)
	}
	updated, err := repo.GetImagePromptAction(ctx, multiple.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(updated.ProjectIDs, []string{second.ID}) || updated.ProjectID != second.ID {
		t.Fatalf("shared action should retain only the remaining project: %+v", updated)
	}
}

func TestDeleteProjectRejectsExistingAssets(t *testing.T) {
	ctx := context.Background()
	repo := store.NewMemory(domain.AgentConfig{})
	agent, err := NewImageAgent(repo, repo, blob.Noop{}, security.NoopScanner{}, "test-image-secret")
	if err != nil {
		t.Fatal(err)
	}
	actor, _ := repo.GetUser(ctx, store.DemoAdminID)
	project, err := agent.SaveProject(ctx, actor, "", ImageProjectInput{
		ProjectKey: "used-project", Name: "已有数据项目", ACL: domain.ACL{Scope: "all"}, Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = repo.CreateImageAsset(ctx, domain.ImageAsset{ID: "asset-project-delete", OwnerID: actor.ID, ProjectID: project.ID}); err != nil {
		t.Fatal(err)
	}
	if err = agent.DeleteProject(ctx, actor, project.ID); err == nil || !errors.Is(err, store.ErrConflict) {
		t.Fatalf("used project deletion must be rejected, got %v", err)
	}
	if _, err = repo.GetImageProject(ctx, project.ID); err != nil {
		t.Fatalf("rejected deletion must keep the project: %v", err)
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

	nodes := placeholderNodes(canvas, job, "anchor", 5000, 5000, 0, 0)

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

	nodes := placeholderNodes(canvas, job, "not-on-this-canvas", 1000, 560, 0, 0)

	if nodes[0].X != 1124 || nodes[0].Y != 500 {
		t.Fatalf("expected nearest node fallback with top alignment, got x=%v y=%v", nodes[0].X, nodes[0].Y)
	}
}

func TestPlaceholderNodesCenterTwoColumnGroupOnEmptyCanvas(t *testing.T) {
	canvas := domain.ImageCanvas{ID: "canvas-empty"}
	job := domain.ImageJob{ID: "job-grid", AspectRatio: "1:1", Count: 4}

	nodes := placeholderNodes(canvas, job, "", 1000, 600, 0, 0)

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
				"", 1000, 1000, 0, 0,
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

func TestPlaceholderNodesUseFirstReferenceExactRatio(t *testing.T) {
	nodes := placeholderNodes(
		domain.ImageCanvas{ID: "canvas-original"},
		domain.ImageJob{ID: "job-original", AspectRatio: "original", Count: 1},
		"", 500, 500, 1000, 427,
	)
	if len(nodes) != 1 || nodes[0].Width != 360 || math.Abs(nodes[0].Height-153.72) > 0.001 {
		t.Fatalf("original placeholder should preserve the first reference ratio: %+v", nodes)
	}
}

func TestOriginalRatioRequiresReferenceCapableChatModel(t *testing.T) {
	valid := ImageJobInput{Kind: "generate", AspectRatio: "original", ReferenceAssetIDs: []string{"asset"}}
	chatModel := domain.ImageModel{Protocol: "chat_completions", SupportsReference: true}
	if err := validateOriginalAspectRatio(valid, chatModel); err != nil {
		t.Fatalf("reference-capable chat model should accept original ratio: %v", err)
	}
	invalid := []struct {
		name  string
		input ImageJobInput
		model domain.ImageModel
	}{
		{name: "no reference", input: ImageJobInput{Kind: "generate", AspectRatio: "original"}, model: chatModel},
		{name: "images protocol", input: valid, model: domain.ImageModel{Protocol: "images_generations", SupportsReference: true}},
		{name: "unsupported reference", input: valid, model: domain.ImageModel{Protocol: "chat_completions"}},
	}
	for _, test := range invalid {
		t.Run(test.name, func(t *testing.T) {
			if err := validateOriginalAspectRatio(test.input, test.model); err == nil {
				t.Fatal("invalid original ratio request should be rejected")
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

func TestPromptActionSupportsMultipleProjectScopes(t *testing.T) {
	ctx := context.Background()
	repo := store.NewMemory(domain.AgentConfig{})
	admin, _ := repo.GetUser(ctx, store.DemoAdminID)
	employee, _ := repo.GetUser(ctx, store.DemoEmployeeID)
	projects := []domain.ImageProject{
		{ID: "project-multi-a", ProjectKey: "multi-a", Name: "项目 A", ACL: domain.ACL{Scope: "all"}, Enabled: true},
		{ID: "project-multi-b", ProjectKey: "multi-b", Name: "项目 B", ACL: domain.ACL{Scope: "all"}, Enabled: true},
		{ID: "project-multi-c", ProjectKey: "multi-c", Name: "项目 C", ACL: domain.ACL{Scope: "all"}, Enabled: true},
	}
	for _, project := range projects {
		if err := repo.UpsertImageProject(ctx, project); err != nil {
			t.Fatal(err)
		}
	}
	agent, err := NewImageAgent(repo, repo, newImageTestBlob(), security.NoopScanner{}, "test-image-secret")
	if err != nil {
		t.Fatal(err)
	}
	scoped, err := agent.SavePromptAction(ctx, admin, "", ImagePromptActionInput{
		ActionKey: "shared-style", PromptTemplate: "项目专属风格", ProjectIDs: []string{projects[1].ID, projects[0].ID}, Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(scoped.ProjectIDs, []string{projects[0].ID, projects[1].ID}) || scoped.ProjectID != projects[0].ID {
		t.Fatalf("project scopes should be deduplicated, sorted and retain a legacy primary project: %+v", scoped)
	}
	for _, projectID := range []string{projects[0].ID, projects[1].ID} {
		actions, listErr := agent.PromptActions(ctx, employee, projectID)
		if listErr != nil || len(actions) != 1 || actions[0].ID != scoped.ID {
			t.Fatalf("scoped action should be available in %s: actions=%+v err=%v", projectID, actions, listErr)
		}
	}
	if actions, listErr := agent.PromptActions(ctx, employee, projects[2].ID); listErr != nil || len(actions) != 0 {
		t.Fatalf("scoped action must not leak into unrelated projects: actions=%+v err=%v", actions, listErr)
	}

	global, err := agent.SavePromptAction(ctx, admin, "", ImagePromptActionInput{
		ActionKey: "shared-style", PromptTemplate: "全局风格", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if actions, _ := agent.PromptActions(ctx, employee, projects[0].ID); len(actions) != 1 || actions[0].ID != scoped.ID {
		t.Fatalf("multi-project action should override the global action in its projects: %+v", actions)
	}
	if actions, _ := agent.PromptActions(ctx, employee, projects[2].ID); len(actions) != 1 || actions[0].ID != global.ID {
		t.Fatalf("global action should remain available outside the selected projects: %+v", actions)
	}
	if _, err = agent.SavePromptAction(ctx, admin, "", ImagePromptActionInput{
		ActionKey: "shared-style", PromptTemplate: "重复项目风格", ProjectIDs: []string{projects[1].ID}, Enabled: true,
	}); err == nil || !strings.Contains(err.Error(), "已存在相同按键标识") {
		t.Fatalf("overlapping project scopes with the same action key must be rejected, got %v", err)
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

func TestReferenceDataURLsAllowSameOwnerAssetsAcrossProjects(t *testing.T) {
	ctx := context.Background()
	repo := store.NewMemory(domain.AgentConfig{})
	user, err := repo.GetUser(ctx, store.DemoEmployeeID)
	if err != nil {
		t.Fatal(err)
	}
	blobs := newImageTestBlob()
	agent, err := NewImageAgent(repo, repo, blobs, security.NoopScanner{}, "test-image-secret")
	if err != nil {
		t.Fatal(err)
	}
	asset := domain.ImageAsset{
		ID: "asset-from-another-project", OwnerID: user.ID, ProjectID: "source-project",
		ObjectKey: "image-agent/reference.png", MIMEType: "image/png", SizeBytes: 3,
	}
	if err = repo.CreateImageAsset(ctx, asset); err != nil {
		t.Fatal(err)
	}
	if err = blobs.Put(ctx, asset.ObjectKey, []byte{1, 2, 3}, asset.MIMEType); err != nil {
		t.Fatal(err)
	}

	dispatcher := NewImageDispatcher(agent, time.Hour)
	references, err := dispatcher.referenceDataURLs(ctx, domain.ImageJob{
		ID: "job-cross-project", UserID: user.ID, ProjectID: "target-project",
		ReferenceAssetIDs: []string{asset.ID},
	})
	if err != nil {
		t.Fatalf("same-owner cross-project reference should be readable: %v", err)
	}
	if len(references) != 1 || references[0] != "data:image/png;base64,AQID" {
		t.Fatalf("unexpected reference data URL: %#v", references)
	}
}

func TestReferenceDataURLsRejectAssetsOwnedByAnotherUser(t *testing.T) {
	ctx := context.Background()
	repo := store.NewMemory(domain.AgentConfig{})
	user, _ := repo.GetUser(ctx, store.DemoEmployeeID)
	other, _ := repo.GetUser(ctx, store.DemoAdminID)
	blobs := newImageTestBlob()
	agent, err := NewImageAgent(repo, repo, blobs, security.NoopScanner{}, "test-image-secret")
	if err != nil {
		t.Fatal(err)
	}
	asset := domain.ImageAsset{
		ID: "asset-owned-by-other-user", OwnerID: other.ID, ProjectID: "target-project",
		ObjectKey: "image-agent/private.png", MIMEType: "image/png", SizeBytes: 3,
	}
	if err = repo.CreateImageAsset(ctx, asset); err != nil {
		t.Fatal(err)
	}

	dispatcher := NewImageDispatcher(agent, time.Hour)
	if _, err = dispatcher.referenceDataURLs(ctx, domain.ImageJob{
		ID: "job-wrong-owner", UserID: user.ID, ProjectID: "target-project",
		ReferenceAssetIDs: []string{asset.ID},
	}); err == nil || !strings.Contains(err.Error(), "no longer belongs to this user") {
		t.Fatalf("another user's reference asset must be rejected, got %v", err)
	}
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

func TestCreateBackgroundRemovalJobPlacesResultToRight(t *testing.T) {
	ctx := context.Background()
	repo := store.NewMemory(domain.AgentConfig{})
	user, _ := repo.GetUser(ctx, store.DemoEmployeeID)
	project := domain.ImageProject{ID: "project-bg", ProjectKey: "background", Name: "抠图", ACL: domain.ACL{Scope: "all"}, Enabled: true, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	_ = repo.UpsertImageProject(ctx, project)
	blobs := newImageTestBlob()
	agent, _ := NewImageAgent(repo, repo, blobs, security.NoopScanner{}, "test-image-secret")
	_, err := agent.SavePixianConfig(ctx, user, PixianConfigInput{Enabled: true, TestMode: true, APIID: "test-id", APISecret: "test-secret", TimeoutSeconds: 180, Concurrency: 2, MaxPixels: 25_000_000})
	if err != nil {
		t.Fatal(err)
	}
	canvas, _ := agent.CreateCanvas(ctx, user, "抠图画布")
	png, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAIAAAABCAYAAAD0In+KAAAADUlEQVR42mNk+M/wHwAEAQH/5agzWQAAAABJRU5ErkJggg==")
	canvas, err = agent.ImportCanvasAssetByID(ctx, user, canvas.ID, project.ID, ImageCanvasImport{FileName: "source.png", DeclaredMIME: "image/png", Data: png, X: 200, Y: 200, Version: canvas.Version, Origin: "drop"})
	if err != nil {
		t.Fatal(err)
	}
	source := canvas.Nodes[0]
	job, err := agent.CreateBackgroundRemovalJob(ctx, user, canvas.ID, BackgroundRemovalJobInput{NodeIDs: []string{source.ID}, Version: canvas.Version, IdempotencyKey: "remove-once"})
	if err != nil {
		t.Fatal(err)
	}
	updated, _ := agent.CanvasByID(ctx, user, canvas.ID, false)
	if len(updated.Nodes) != 2 {
		t.Fatalf("expected source plus placeholder, got %+v", updated.Nodes)
	}
	placeholder := updated.Nodes[1]
	if placeholder.X != source.X+source.Width+imageCanvasNodeGap || placeholder.SourceNodeID != source.ID || placeholder.BackgroundRemovalJobID != job.ID {
		t.Fatalf("unexpected placeholder placement: source=%+v placeholder=%+v", source, placeholder)
	}
}

package service

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"testing"
	"time"

	"internal-ai-agent/backend/internal/blob"
	"internal-ai-agent/backend/internal/domain"
	"internal-ai-agent/backend/internal/security"
	"internal-ai-agent/backend/internal/store"
)

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

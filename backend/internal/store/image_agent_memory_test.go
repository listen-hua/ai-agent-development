package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"internal-ai-agent/backend/internal/domain"
)

func TestCreateImageJobAtomicallyClaimsCanvasVersion(t *testing.T) {
	ctx := context.Background()
	repo := NewMemory(domain.AgentConfig{})
	canvas, err := repo.GetOrCreateImageCanvas(ctx, "user-placement", "project-placement")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	job := domain.ImageJob{
		ID: "job-one", UserID: canvas.UserID, ProjectID: canvas.ProjectID, CanvasID: canvas.ID,
		Count: 1, IdempotencyKey: "request-one", CreatedAt: now, UpdatedAt: now,
	}
	node := domain.ImageCanvasNode{
		ID: "node-one", CanvasID: canvas.ID, JobID: job.ID, Status: "pending",
		X: 100, Y: 100, Width: 360, Height: 360, CreatedAt: now, UpdatedAt: now,
	}

	if _, err = repo.CreateImageJob(ctx, job, []domain.ImageCanvasNode{node}, canvas.Version); err != nil {
		t.Fatal(err)
	}
	updated, err := repo.GetOrCreateImageCanvas(ctx, canvas.UserID, canvas.ProjectID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Version != canvas.Version+1 || len(updated.Nodes) != 1 {
		t.Fatalf("placeholder creation must atomically advance the canvas: version=%d nodes=%d", updated.Version, len(updated.Nodes))
	}

	staleJob := job
	staleJob.ID = "job-two"
	staleJob.IdempotencyKey = "request-two"
	staleNode := node
	staleNode.ID = "node-two"
	staleNode.JobID = staleJob.ID
	if _, err = repo.CreateImageJob(ctx, staleJob, []domain.ImageCanvasNode{staleNode}, canvas.Version); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected stale canvas version conflict, got %v", err)
	}
	if _, err = repo.GetImageJob(ctx, staleJob.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("conflicting job must not be persisted, got %v", err)
	}
}

func TestDeleteImageCanvasNodeAtomicallyClaimsCanvasVersion(t *testing.T) {
	ctx := context.Background()
	repo := NewMemory(domain.AgentConfig{})
	canvas, err := repo.GetOrCreateImageCanvas(ctx, "user-delete", "project-delete")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	asset := domain.ImageAsset{
		ID: "asset-delete", OwnerID: canvas.UserID, ProjectID: canvas.ProjectID,
		ObjectKey: "images/delete.png", MIMEType: "image/png", Source: "upload", CreatedAt: now,
	}
	node := domain.ImageCanvasNode{
		ID: "node-delete", CanvasID: canvas.ID, AssetID: asset.ID, Status: "ready",
		X: 10, Y: 20, Width: 360, Height: 240, CreatedAt: now, UpdatedAt: now,
	}
	imported, err := repo.ImportImageCanvasAsset(ctx, canvas, canvas.Version, asset, node)
	if err != nil {
		t.Fatal(err)
	}
	listed, err := repo.ListImageCanvases(ctx, canvas.UserID, false)
	if err != nil || len(listed) != 1 {
		t.Fatalf("unexpected canvas list: count=%d err=%v", len(listed), err)
	}
	if listed[0].PreviewAssetID != asset.ID || listed[0].NodeCount != 1 {
		t.Fatalf("canvas list must expose its first ready image as preview: %+v", listed[0])
	}

	if _, err = repo.DeleteImageCanvasNode(ctx, imported, node.ID, canvas.Version); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected stale version conflict, got %v", err)
	}
	unchanged, err := repo.GetOrCreateImageCanvas(ctx, canvas.UserID, canvas.ProjectID)
	if err != nil || len(unchanged.Nodes) != 1 {
		t.Fatalf("stale deletion must leave the node intact: nodes=%d err=%v", len(unchanged.Nodes), err)
	}

	deleted, err := repo.DeleteImageCanvasNode(ctx, imported, node.ID, imported.Version)
	if err != nil {
		t.Fatal(err)
	}
	if deleted.Version != imported.Version+1 || len(deleted.Nodes) != 0 {
		t.Fatalf("unexpected canvas after deletion: version=%d nodes=%d", deleted.Version, len(deleted.Nodes))
	}
	if _, err = repo.GetImageAsset(ctx, asset.ID); err != nil {
		t.Fatalf("canvas deletion should retain the asset for job history and audit: %v", err)
	}
}

func TestDeleteImageCanvasNodesRemovesAllNodesWithOneVersion(t *testing.T) {
	ctx := context.Background()
	repo := NewMemory(domain.AgentConfig{})
	canvas, err := repo.GetOrCreateImageCanvas(ctx, "user-batch-delete", "project-batch-delete")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	for index, id := range []string{"node-one", "node-two", "node-three"} {
		asset := domain.ImageAsset{ID: "asset-" + id, OwnerID: canvas.UserID, ProjectID: canvas.ProjectID, Source: "upload", CreatedAt: now}
		node := domain.ImageCanvasNode{ID: id, CanvasID: canvas.ID, AssetID: asset.ID, Status: "ready", ZIndex: index + 1, CreatedAt: now, UpdatedAt: now}
		canvas, err = repo.ImportImageCanvasAsset(ctx, canvas, canvas.Version, asset, node)
		if err != nil {
			t.Fatal(err)
		}
	}

	beforeVersion := canvas.Version
	updated, err := repo.DeleteImageCanvasNodes(ctx, canvas, []string{"node-one", "node-three"}, beforeVersion)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Version != beforeVersion+1 || len(updated.Nodes) != 1 || updated.Nodes[0].ID != "node-two" {
		t.Fatalf("unexpected batch deletion result: version=%d nodes=%+v", updated.Version, updated.Nodes)
	}
	if _, err = repo.DeleteImageCanvasNodes(ctx, updated, []string{"node-two", "missing"}, updated.Version); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing member must roll back the whole batch, got %v", err)
	}
	unchanged, _ := repo.GetOrCreateImageCanvas(ctx, canvas.UserID, canvas.ProjectID)
	if unchanged.Version != updated.Version || len(unchanged.Nodes) != 1 {
		t.Fatalf("failed batch must not change canvas: version=%d nodes=%d", unchanged.Version, len(unchanged.Nodes))
	}
}

func TestCreateBackgroundRemovalJobAtomicallyAddsPlaceholders(t *testing.T) {
	ctx := context.Background()
	repo := NewMemory(domain.AgentConfig{})
	canvas, err := repo.GetOrCreateImageCanvas(ctx, "user-bg", "project-bg")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	job := domain.BackgroundRemovalJob{ID: "bg-job", UserID: canvas.UserID, CanvasID: canvas.ID, Status: "pending", IdempotencyKey: "request-bg", NextAttemptAt: now, CreatedAt: now, UpdatedAt: now}
	node := domain.ImageCanvasNode{ID: "bg-placeholder", CanvasID: canvas.ID, BackgroundRemovalJobID: job.ID, SourceNodeID: "source-node", Status: "pending", Width: 360, Height: 240, CreatedAt: now, UpdatedAt: now}
	item := domain.BackgroundRemovalItem{ID: "bg-item", JobID: job.ID, SourceNodeID: "source-node", SourceAssetID: "source-asset", ProjectID: "project-bg", PlaceholderNodeID: node.ID, Status: "pending", CreatedAt: now, UpdatedAt: now}
	created, err := repo.CreateBackgroundRemovalJob(ctx, job, []domain.BackgroundRemovalItem{item}, []domain.ImageCanvasNode{node}, canvas.Version)
	if err != nil {
		t.Fatal(err)
	}
	if len(created.Items) != 1 {
		t.Fatalf("unexpected job items: %+v", created.Items)
	}
	updated, _ := repo.GetImageCanvas(ctx, canvas.ID)
	if updated.Version != canvas.Version+1 || len(updated.Nodes) != 1 || updated.Nodes[0].BackgroundRemovalJobID != job.ID {
		t.Fatalf("placeholder and version must be atomic: %+v", updated)
	}
	duplicate := job
	duplicate.ID = "bg-job-duplicate"
	again, err := repo.CreateBackgroundRemovalJob(ctx, duplicate, []domain.BackgroundRemovalItem{item}, []domain.ImageCanvasNode{node}, updated.Version)
	if err != nil || again.ID != job.ID {
		t.Fatalf("idempotent request must return existing job: %+v err=%v", again, err)
	}
}

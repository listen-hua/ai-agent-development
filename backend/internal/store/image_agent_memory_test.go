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

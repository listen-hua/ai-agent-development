package store

import (
	"context"
	"sort"
	"time"

	"internal-ai-agent/backend/internal/domain"
	"internal-ai-agent/backend/internal/ids"
)

func (m *Memory) ListImageRelays(context.Context) ([]domain.ImageRelay, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]domain.ImageRelay, 0, len(m.imageRelays))
	for _, value := range m.imageRelays {
		out = append(out, value)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (m *Memory) GetImageRelay(_ context.Context, id string) (domain.ImageRelay, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	value, ok := m.imageRelays[id]
	if !ok {
		return value, ErrNotFound
	}
	return value, nil
}

func (m *Memory) UpsertImageRelay(_ context.Context, value domain.ImageRelay) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.imageRelays[value.ID] = value
	return nil
}

func (m *Memory) ListImageModels(_ context.Context, relayID string) ([]domain.ImageModel, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []domain.ImageModel{}
	for _, value := range m.imageModels {
		if relayID == "" || value.RelayID == relayID {
			out = append(out, value)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DisplayName < out[j].DisplayName })
	return out, nil
}

func (m *Memory) GetImageModel(_ context.Context, id string) (domain.ImageModel, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	value, ok := m.imageModels[id]
	if !ok {
		return value, ErrNotFound
	}
	return value, nil
}

func (m *Memory) UpsertImageModels(_ context.Context, values []domain.ImageModel) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, value := range values {
		existingID := ""
		for id, existing := range m.imageModels {
			if existing.RelayID == value.RelayID && existing.ModelID == value.ModelID {
				existingID = id
				break
			}
		}
		if existingID != "" {
			existing := m.imageModels[existingID]
			existing.DisplayName = value.DisplayName
			existing.UpdatedAt = value.UpdatedAt
			m.imageModels[existingID] = existing
		} else {
			m.imageModels[value.ID] = value
		}
	}
	return nil
}

func (m *Memory) UpdateImageModel(_ context.Context, value domain.ImageModel) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.imageModels[value.ID]; !ok {
		return ErrNotFound
	}
	m.imageModels[value.ID] = value
	return nil
}

func (m *Memory) ListImageProjects(context.Context) ([]domain.ImageProject, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]domain.ImageProject, 0, len(m.imageProjects))
	for _, value := range m.imageProjects {
		out = append(out, value)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (m *Memory) GetImageProject(_ context.Context, id string) (domain.ImageProject, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	value, ok := m.imageProjects[id]
	if !ok {
		return value, ErrNotFound
	}
	return value, nil
}

func (m *Memory) UpsertImageProject(_ context.Context, value domain.ImageProject) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.imageProjects[value.ID] = value
	return nil
}

func (m *Memory) ListImagePromptActions(_ context.Context, projectID string) ([]domain.ImagePromptAction, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []domain.ImagePromptAction{}
	for _, value := range m.imagePromptActions {
		if projectID == "" || value.ProjectID == "" || value.ProjectID == projectID {
			out = append(out, value)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SortOrder < out[j].SortOrder })
	return out, nil
}

func (m *Memory) GetImagePromptAction(_ context.Context, id string) (domain.ImagePromptAction, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	value, ok := m.imagePromptActions[id]
	if !ok {
		return value, ErrNotFound
	}
	return value, nil
}

func (m *Memory) UpsertImagePromptAction(_ context.Context, value domain.ImagePromptAction) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	value.HasPreview = value.PreviewObjectKey != ""
	m.imagePromptActions[value.ID] = value
	return nil
}

func (m *Memory) UpdateImagePromptActionPreview(_ context.Context, value domain.ImagePromptAction) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	existing, ok := m.imagePromptActions[value.ID]
	if !ok {
		return ErrNotFound
	}
	existing.PreviewObjectKey = value.PreviewObjectKey
	existing.PreviewMIMEType = value.PreviewMIMEType
	existing.PreviewSizeBytes = value.PreviewSizeBytes
	existing.PreviewWidth = value.PreviewWidth
	existing.PreviewHeight = value.PreviewHeight
	existing.HasPreview = value.PreviewObjectKey != ""
	existing.UpdatedBy = value.UpdatedBy
	existing.UpdatedAt = value.UpdatedAt
	m.imagePromptActions[value.ID] = existing
	return nil
}

func (m *Memory) DeleteImagePromptAction(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.imagePromptActions[id]; !ok {
		return ErrNotFound
	}
	delete(m.imagePromptActions, id)
	return nil
}

func canvasMemoryKey(userID, projectID string) string { return userID + "\x00" + projectID }

func (m *Memory) GetOrCreateImageCanvas(_ context.Context, userID, projectID string) (domain.ImageCanvas, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := canvasMemoryKey(userID, projectID)
	value, ok := m.imageCanvases[key]
	if !ok {
		now := time.Now()
		value = domain.ImageCanvas{ID: ids.New("canvas"), UserID: userID, ProjectID: projectID,
			Viewport: domain.ImageViewport{Zoom: 1}, Version: 1, CreatedAt: now, UpdatedAt: now, Nodes: []domain.ImageCanvasNode{}}
		m.imageCanvases[key] = value
	}
	return value, nil
}

func (m *Memory) UpdateImageCanvas(_ context.Context, value domain.ImageCanvas, expectedVersion int64) (domain.ImageCanvas, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := canvasMemoryKey(value.UserID, value.ProjectID)
	existing, ok := m.imageCanvases[key]
	if !ok {
		return value, ErrNotFound
	}
	if existing.Version != expectedVersion {
		return value, ErrConflict
	}
	existing.Viewport = value.Viewport
	updates := map[string]domain.ImageCanvasNode{}
	for _, node := range value.Nodes {
		updates[node.ID] = node
	}
	for index, node := range existing.Nodes {
		if updated, ok := updates[node.ID]; ok {
			node.X, node.Y, node.Width, node.Height, node.ZIndex = updated.X, updated.Y, updated.Width, updated.Height, updated.ZIndex
			existing.Nodes[index] = node
		}
	}
	existing.Version++
	existing.UpdatedAt = time.Now()
	m.imageCanvases[key] = existing
	return existing, nil
}

func (m *Memory) ImportImageCanvasAsset(_ context.Context, canvas domain.ImageCanvas, expectedVersion int64, asset domain.ImageAsset, node domain.ImageCanvasNode) (domain.ImageCanvas, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := canvasMemoryKey(canvas.UserID, canvas.ProjectID)
	existing, ok := m.imageCanvases[key]
	if !ok {
		return canvas, ErrNotFound
	}
	if existing.ID != canvas.ID || existing.Version != expectedVersion {
		return canvas, ErrConflict
	}
	if _, exists := m.imageAssets[asset.ID]; exists {
		return canvas, ErrConflict
	}
	maxZ := 0
	for _, current := range existing.Nodes {
		if current.ID == node.ID {
			return canvas, ErrConflict
		}
		if current.ZIndex > maxZ {
			maxZ = current.ZIndex
		}
	}
	node.ZIndex = maxZ + 1
	existing.Nodes = append(existing.Nodes, node)
	existing.Version++
	existing.UpdatedAt = time.Now()
	m.imageAssets[asset.ID] = asset
	m.imageCanvases[key] = existing
	return existing, nil
}

func (m *Memory) DeleteImageCanvasNode(_ context.Context, canvas domain.ImageCanvas, nodeID string, expectedVersion int64) (domain.ImageCanvas, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := canvasMemoryKey(canvas.UserID, canvas.ProjectID)
	existing, ok := m.imageCanvases[key]
	if !ok {
		return canvas, ErrNotFound
	}
	if existing.ID != canvas.ID || existing.Version != expectedVersion {
		return canvas, ErrConflict
	}
	nodeIndex := -1
	for index, node := range existing.Nodes {
		if node.ID == nodeID {
			nodeIndex = index
			break
		}
	}
	if nodeIndex < 0 {
		return canvas, ErrNotFound
	}
	existing.Nodes = append(existing.Nodes[:nodeIndex], existing.Nodes[nodeIndex+1:]...)
	existing.Version++
	existing.UpdatedAt = time.Now()
	m.imageCanvases[key] = existing
	return existing, nil
}

func (m *Memory) CreateImageAsset(_ context.Context, value domain.ImageAsset) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.imageAssets[value.ID] = value
	return nil
}

func (m *Memory) GetImageAsset(_ context.Context, id string) (domain.ImageAsset, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	value, ok := m.imageAssets[id]
	if !ok {
		return value, ErrNotFound
	}
	return value, nil
}

func (m *Memory) CreateImageJob(_ context.Context, value domain.ImageJob, nodes []domain.ImageCanvasNode, expectedCanvasVersion int64) (domain.ImageJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, existing := range m.imageJobs {
		if existing.UserID == value.UserID && existing.IdempotencyKey == value.IdempotencyKey {
			return existing, nil
		}
	}
	key := canvasMemoryKey(value.UserID, value.ProjectID)
	canvas := m.imageCanvases[key]
	if len(nodes) > 0 {
		if canvas.ID != value.CanvasID || canvas.Version != expectedCanvasVersion {
			return value, ErrConflict
		}
		canvas.Version++
		canvas.UpdatedAt = time.Now()
	}
	value.Outputs = make([]domain.ImageJobOutput, value.Count)
	for index := range value.Outputs {
		value.Outputs[index] = domain.ImageJobOutput{ID: ids.New("output"), JobID: value.ID, OutputIndex: index,
			Status: "pending", CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
	}
	m.imageJobs[value.ID] = value
	canvas.Nodes = append(canvas.Nodes, nodes...)
	m.imageCanvases[key] = canvas
	return value, nil
}

func (m *Memory) GetImageJob(_ context.Context, id string) (domain.ImageJob, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	value, ok := m.imageJobs[id]
	if !ok {
		return value, ErrNotFound
	}
	return value, nil
}

func (m *Memory) ListImageJobs(_ context.Context, userID, projectID string, limit int) ([]domain.ImageJob, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []domain.ImageJob{}
	for _, value := range m.imageJobs {
		if value.UserID == userID && (projectID == "" || value.ProjectID == projectID) {
			out = append(out, value)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (m *Memory) ClaimImageJobs(_ context.Context, now time.Time, lease time.Duration, limit int) ([]domain.ImageJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []domain.ImageJob{}
	for id, value := range m.imageJobs {
		if len(out) >= limit {
			break
		}
		if (value.Status == "pending" || value.Status == "retry") && !value.NextAttemptAt.After(now) &&
			(value.LockedUntil == nil || value.LockedUntil.Before(now)) {
			until := now.Add(lease)
			value.Status, value.LockedUntil, value.Attempts, value.UpdatedAt = "running", &until, value.Attempts+1, now
			m.imageJobs[id] = value
			out = append(out, value)
		}
	}
	return out, nil
}

func (m *Memory) UpdateImageJob(_ context.Context, value domain.ImageJob) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.imageJobs[value.ID]; !ok {
		return ErrNotFound
	}
	m.imageJobs[value.ID] = value
	return nil
}

func (m *Memory) SaveImageJobOutput(_ context.Context, output domain.ImageJobOutput, asset domain.ImageAsset, node domain.ImageCanvasNode) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.imageAssets[asset.ID] = asset
	job := m.imageJobs[output.JobID]
	for index := range job.Outputs {
		if job.Outputs[index].ID == output.ID {
			output.AssetID, output.Status = asset.ID, "succeeded"
			job.Outputs[index] = output
		}
	}
	m.imageJobs[job.ID] = job
	key := canvasMemoryKey(job.UserID, job.ProjectID)
	canvas := m.imageCanvases[key]
	for index := range canvas.Nodes {
		if canvas.Nodes[index].JobID == output.JobID && canvas.Nodes[index].OutputIndex == output.OutputIndex {
			canvas.Nodes[index].AssetID, canvas.Nodes[index].Status = asset.ID, "ready"
			canvas.Nodes[index].Width, canvas.Nodes[index].Height = node.Width, node.Height
		}
	}
	m.imageCanvases[key] = canvas
	return nil
}

func (m *Memory) UpdateImageJobOutputFailure(_ context.Context, output domain.ImageJobOutput, node domain.ImageCanvasNode) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	job := m.imageJobs[output.JobID]
	for index := range job.Outputs {
		if job.Outputs[index].ID == output.ID {
			job.Outputs[index] = output
		}
	}
	m.imageJobs[job.ID] = job
	key := canvasMemoryKey(job.UserID, job.ProjectID)
	canvas := m.imageCanvases[key]
	for index := range canvas.Nodes {
		if canvas.Nodes[index].JobID == output.JobID && canvas.Nodes[index].OutputIndex == output.OutputIndex {
			canvas.Nodes[index].Status, canvas.Nodes[index].Error = "failed", node.Error
		}
	}
	m.imageCanvases[key] = canvas
	return nil
}

func (m *Memory) CleanupImageJobLogs(_ context.Context, before time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, value := range m.imageJobs {
		if value.CreatedAt.Before(before) {
			value.Prompt, value.Error, value.IdempotencyKey = "", "", "expired-"+value.ID
			m.imageJobs[id] = value
		}
	}
	return nil
}

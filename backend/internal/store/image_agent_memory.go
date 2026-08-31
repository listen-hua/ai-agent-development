package store

import (
	"context"
	"fmt"
	"sort"
	"strings"
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
			existing.RequestModelID = value.RequestModelID
			existing.RemoteEndpointTypes = append([]string(nil), value.RemoteEndpointTypes...)
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

func (m *Memory) DeleteImageProject(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.imageProjects[id]; !ok {
		return ErrNotFound
	}
	for _, value := range m.imageAssets {
		if value.ProjectID == id {
			return ErrConflict
		}
	}
	for _, value := range m.imageJobs {
		if value.ProjectID == id {
			return ErrConflict
		}
	}
	for _, value := range m.imageCanvases {
		if value.ProjectID == id {
			return ErrConflict
		}
	}
	for actionID, value := range m.imagePromptActions {
		projectIDs := value.ProjectIDs
		if len(projectIDs) == 0 && value.ProjectID != "" {
			projectIDs = []string{value.ProjectID}
		}
		if !containsStringValue(projectIDs, id) {
			continue
		}
		remaining := make([]string, 0, len(projectIDs)-1)
		for _, projectID := range projectIDs {
			if projectID != id {
				remaining = append(remaining, projectID)
			}
		}
		if len(remaining) == 0 {
			delete(m.imagePromptActions, actionID)
			continue
		}
		value.ProjectIDs = remaining
		value.ProjectID = remaining[0]
		m.imagePromptActions[actionID] = value
	}
	delete(m.imageProjects, id)
	return nil
}

func (m *Memory) ListImagePromptActions(_ context.Context, projectID string) ([]domain.ImagePromptAction, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []domain.ImagePromptAction{}
	for _, value := range m.imagePromptActions {
		projectIDs := value.ProjectIDs
		if len(projectIDs) == 0 && value.ProjectID != "" {
			projectIDs = []string{value.ProjectID}
		}
		if projectID == "" || len(projectIDs) == 0 || containsStringValue(projectIDs, projectID) {
			out = append(out, value)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SortOrder < out[j].SortOrder })
	return out, nil
}

func containsStringValue(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
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

func (m *Memory) ListImageCanvases(_ context.Context, userID string, deleted bool) ([]domain.ImageCanvas, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := []domain.ImageCanvas{}
	for _, value := range m.imageCanvases {
		if value.UserID != userID || (value.DeletedAt != nil) != deleted {
			continue
		}
		value.NodeCount = len(value.Nodes)
		value.PreviewAssetID = firstCanvasPreviewAssetID(value.Nodes)
		value.Nodes = []domain.ImageCanvasNode{}
		result = append(result, value)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].UpdatedAt.After(result[j].UpdatedAt) })
	return result, nil
}

func (m *Memory) CreateImageCanvas(_ context.Context, value domain.ImageCanvas) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, existing := range m.imageCanvases {
		if existing.UserID == value.UserID && existing.DeletedAt == nil && strings.EqualFold(existing.Name, value.Name) {
			return ErrConflict
		}
	}
	m.imageCanvases[value.ID] = value
	return nil
}

func (m *Memory) GetImageCanvas(_ context.Context, id string) (domain.ImageCanvas, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	value, ok := m.imageCanvases[id]
	if !ok {
		return value, ErrNotFound
	}
	value.NodeCount = len(value.Nodes)
	value.PreviewAssetID = firstCanvasPreviewAssetID(value.Nodes)
	return value, nil
}

func firstCanvasPreviewAssetID(nodes []domain.ImageCanvasNode) string {
	var selected domain.ImageCanvasNode
	found := false
	for _, node := range nodes {
		if node.Status != "ready" || node.AssetID == "" {
			continue
		}
		if !found || node.ZIndex < selected.ZIndex || (node.ZIndex == selected.ZIndex && node.CreatedAt.Before(selected.CreatedAt)) {
			selected, found = node, true
		}
	}
	if found {
		return selected.AssetID
	}
	return ""
}

func (m *Memory) SoftDeleteImageCanvas(_ context.Context, id, userID string, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	value, ok := m.imageCanvases[id]
	if !ok || value.UserID != userID || value.DeletedAt != nil {
		return ErrNotFound
	}
	value.DeletedAt = &now
	value.UpdatedAt = now
	m.imageCanvases[id] = value
	return nil
}

func (m *Memory) RestoreImageCanvas(_ context.Context, id, userID string, now time.Time) (domain.ImageCanvas, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	value, ok := m.imageCanvases[id]
	if !ok || value.UserID != userID || value.DeletedAt == nil {
		return value, ErrNotFound
	}
	baseRunes := []rune(strings.TrimSpace(value.Name))
	if len(baseRunes) > 65 {
		baseRunes = baseRunes[:65]
	}
	base, name := string(baseRunes), value.Name
	for suffix := 0; ; suffix++ {
		conflict := false
		for _, existing := range m.imageCanvases {
			if existing.ID != id && existing.UserID == userID && existing.DeletedAt == nil && strings.EqualFold(existing.Name, name) {
				conflict = true
				break
			}
		}
		if !conflict {
			break
		}
		name = base + "（恢复）"
		if suffix > 0 {
			name = fmt.Sprintf("%s（恢复%d）", base, suffix+1)
		}
	}
	value.Name, value.DeletedAt, value.UpdatedAt = name, nil, now
	m.imageCanvases[id] = value
	return value, nil
}

func (m *Memory) CleanupDeletedImageCanvases(_ context.Context, before time.Time) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var count int64
	for id, canvas := range m.imageCanvases {
		if canvas.DeletedAt == nil || !canvas.DeletedAt.Before(before) {
			continue
		}
		active := false
		for _, job := range m.imageJobs {
			if job.CanvasID == id && (job.Status == "pending" || job.Status == "running" || job.Status == "retry") {
				active = true
				break
			}
		}
		if !active {
			delete(m.imageCanvases, id)
			for jobID, job := range m.imageJobs {
				if job.CanvasID == id {
					job.CanvasID = ""
					m.imageJobs[jobID] = job
				}
			}
			count++
		}
	}
	return count, nil
}

func (m *Memory) GetOrCreateImageCanvas(_ context.Context, userID, projectID string) (domain.ImageCanvas, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, value := range m.imageCanvases {
		if value.UserID == userID && value.ProjectID == projectID && value.DeletedAt == nil {
			return value, nil
		}
	}
	now := time.Now()
	value := domain.ImageCanvas{ID: ids.New("canvas"), UserID: userID, ProjectID: projectID, Name: "项目画布-" + projectID,
		Viewport: domain.ImageViewport{Zoom: 1}, Version: 1, CreatedAt: now, UpdatedAt: now, Nodes: []domain.ImageCanvasNode{}}
	m.imageCanvases[value.ID] = value
	return value, nil
}

func (m *Memory) UpdateImageCanvas(_ context.Context, value domain.ImageCanvas, expectedVersion int64) (domain.ImageCanvas, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	existing, ok := m.imageCanvases[value.ID]
	if !ok {
		return value, ErrNotFound
	}
	if existing.Version != expectedVersion || existing.DeletedAt != nil {
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
	m.imageCanvases[value.ID] = existing
	return existing, nil
}

func (m *Memory) ImportImageCanvasAsset(_ context.Context, canvas domain.ImageCanvas, expectedVersion int64, asset domain.ImageAsset, node domain.ImageCanvasNode) (domain.ImageCanvas, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	existing, ok := m.imageCanvases[canvas.ID]
	if !ok {
		return canvas, ErrNotFound
	}
	if existing.Version != expectedVersion || existing.DeletedAt != nil {
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
	m.imageCanvases[canvas.ID] = existing
	return existing, nil
}

func (m *Memory) DeleteImageCanvasNode(ctx context.Context, canvas domain.ImageCanvas, nodeID string, expectedVersion int64) (domain.ImageCanvas, error) {
	return m.DeleteImageCanvasNodes(ctx, canvas, []string{nodeID}, expectedVersion)
}

func (m *Memory) DeleteImageCanvasNodes(_ context.Context, canvas domain.ImageCanvas, nodeIDs []string, expectedVersion int64) (domain.ImageCanvas, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	existing, ok := m.imageCanvases[canvas.ID]
	if !ok {
		return canvas, ErrNotFound
	}
	if existing.Version != expectedVersion || existing.DeletedAt != nil {
		return canvas, ErrConflict
	}
	deleting := make(map[string]bool, len(nodeIDs))
	for _, nodeID := range nodeIDs {
		deleting[nodeID] = true
	}
	found := 0
	remaining := make([]domain.ImageCanvasNode, 0, len(existing.Nodes))
	for _, node := range existing.Nodes {
		if deleting[node.ID] {
			found++
			continue
		}
		remaining = append(remaining, node)
	}
	if found != len(deleting) {
		return canvas, ErrNotFound
	}
	existing.Nodes = remaining
	existing.Version++
	existing.UpdatedAt = time.Now()
	m.imageCanvases[canvas.ID] = existing
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
	canvas := m.imageCanvases[value.CanvasID]
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
			Status: "pending", RequestedSize: value.ImageSize, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
	}
	m.imageJobs[value.ID] = value
	canvas.Nodes = append(canvas.Nodes, nodes...)
	m.imageCanvases[value.CanvasID] = canvas
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
	canvas := m.imageCanvases[job.CanvasID]
	for index := range canvas.Nodes {
		if canvas.Nodes[index].JobID == output.JobID && canvas.Nodes[index].OutputIndex == output.OutputIndex {
			canvas.Nodes[index].AssetID, canvas.Nodes[index].Status = asset.ID, "ready"
			canvas.Nodes[index].Width, canvas.Nodes[index].Height = node.Width, node.Height
			canvas.Nodes[index].RequestedSize = output.RequestedSize
			canvas.Nodes[index].ActualWidth, canvas.Nodes[index].ActualHeight = output.ActualWidth, output.ActualHeight
			canvas.Nodes[index].ResolutionWarning = output.ResolutionWarning
		}
	}
	m.imageCanvases[job.CanvasID] = canvas
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
	canvas := m.imageCanvases[job.CanvasID]
	for index := range canvas.Nodes {
		if canvas.Nodes[index].JobID == output.JobID && canvas.Nodes[index].OutputIndex == output.OutputIndex {
			canvas.Nodes[index].Status, canvas.Nodes[index].Error = "failed", node.Error
		}
	}
	m.imageCanvases[job.CanvasID] = canvas
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

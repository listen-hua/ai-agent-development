package store

import (
	"context"
	"sort"
	"time"

	"internal-ai-agent/backend/internal/domain"
)

func (m *Memory) GetPixianBackgroundRemovalConfig(context.Context) (domain.PixianBackgroundRemovalConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v := m.pixianConfig
	v.HasAPIID = v.EncryptedAPIID != ""
	v.HasAPISecret = v.EncryptedAPISecret != ""
	return v, nil
}
func (m *Memory) SavePixianBackgroundRemovalConfig(_ context.Context, v domain.PixianBackgroundRemovalConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pixianConfig = v
	return nil
}

func (m *Memory) CreateBackgroundRemovalJob(_ context.Context, job domain.BackgroundRemovalJob, items []domain.BackgroundRemovalItem, nodes []domain.ImageCanvasNode, version int64) (domain.BackgroundRemovalJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, v := range m.backgroundRemovalJobs {
		if v.UserID == job.UserID && v.IdempotencyKey == job.IdempotencyKey {
			return v, nil
		}
	}
	canvas, ok := m.imageCanvases[job.CanvasID]
	if !ok {
		return job, ErrNotFound
	}
	if canvas.Version != version || canvas.DeletedAt != nil {
		return job, ErrConflict
	}
	active := map[string]bool{}
	for _, v := range m.backgroundRemovalJobs {
		for _, item := range v.Items {
			if item.Status == "pending" || item.Status == "running" {
				active[item.SourceNodeID] = true
			}
		}
	}
	for _, item := range items {
		if active[item.SourceNodeID] {
			return job, ErrConflict
		}
	}
	canvas.Version++
	canvas.UpdatedAt = job.UpdatedAt
	canvas.Nodes = append(canvas.Nodes, nodes...)
	m.imageCanvases[canvas.ID] = canvas
	job.Items = append([]domain.BackgroundRemovalItem(nil), items...)
	m.backgroundRemovalJobs[job.ID] = job
	return job, nil
}
func (m *Memory) GetBackgroundRemovalJob(_ context.Context, id string) (domain.BackgroundRemovalJob, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.backgroundRemovalJobs[id]
	if !ok {
		return v, ErrNotFound
	}
	return v, nil
}
func (m *Memory) ListBackgroundRemovalJobs(_ context.Context, limit int) ([]domain.BackgroundRemovalJob, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []domain.BackgroundRemovalJob{}
	for _, v := range m.backgroundRemovalJobs {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
func (m *Memory) ClaimBackgroundRemovalJobs(_ context.Context, now time.Time, lease time.Duration, limit int) ([]domain.BackgroundRemovalJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []domain.BackgroundRemovalJob{}
	for id, v := range m.backgroundRemovalJobs {
		if len(out) >= limit {
			break
		}
		if (v.Status == "pending" || v.Status == "retry") && !v.NextAttemptAt.After(now) && (v.LockedUntil == nil || v.LockedUntil.Before(now)) {
			until := now.Add(lease)
			v.Status = "running"
			v.Attempts++
			v.LockedUntil = &until
			v.UpdatedAt = now
			m.backgroundRemovalJobs[id] = v
			out = append(out, v)
		}
	}
	return out, nil
}
func (m *Memory) UpdateBackgroundRemovalItem(_ context.Context, item domain.BackgroundRemovalItem, node domain.ImageCanvasNode) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	job, ok := m.backgroundRemovalJobs[item.JobID]
	if !ok {
		return ErrNotFound
	}
	for i := range job.Items {
		if job.Items[i].ID == item.ID {
			job.Items[i] = item
		}
	}
	m.backgroundRemovalJobs[job.ID] = job
	canvas := m.imageCanvases[job.CanvasID]
	for i := range canvas.Nodes {
		if canvas.Nodes[i].ID == item.PlaceholderNodeID {
			canvas.Nodes[i].Status = node.Status
			canvas.Nodes[i].Error = node.Error
			canvas.Nodes[i].UpdatedAt = node.UpdatedAt
		}
	}
	m.imageCanvases[canvas.ID] = canvas
	return nil
}
func (m *Memory) SaveBackgroundRemovalResult(_ context.Context, item domain.BackgroundRemovalItem, asset domain.ImageAsset, node domain.ImageCanvasNode) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	job, ok := m.backgroundRemovalJobs[item.JobID]
	if !ok {
		return ErrNotFound
	}
	item.ResultAssetID = asset.ID
	item.Status = "succeeded"
	for i := range job.Items {
		if job.Items[i].ID == item.ID {
			job.Items[i] = item
		}
	}
	m.backgroundRemovalJobs[job.ID] = job
	m.imageAssets[asset.ID] = asset
	canvas := m.imageCanvases[job.CanvasID]
	for i := range canvas.Nodes {
		if canvas.Nodes[i].ID == item.PlaceholderNodeID {
			canvas.Nodes[i].AssetID = asset.ID
			canvas.Nodes[i].Status = "ready"
			canvas.Nodes[i].Width = node.Width
			canvas.Nodes[i].Height = node.Height
			canvas.Nodes[i].Error = ""
			canvas.Nodes[i].UpdatedAt = node.UpdatedAt
		}
	}
	m.imageCanvases[canvas.ID] = canvas
	return nil
}
func (m *Memory) UpdateBackgroundRemovalJob(_ context.Context, v domain.BackgroundRemovalJob) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	old, ok := m.backgroundRemovalJobs[v.ID]
	if !ok {
		return ErrNotFound
	}
	v.Items = old.Items
	m.backgroundRemovalJobs[v.ID] = v
	return nil
}
func (m *Memory) BackgroundRemovalStatistics(_ context.Context, since time.Time) (domain.BackgroundRemovalStatistics, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out domain.BackgroundRemovalStatistics
	now := time.Now()
	for _, v := range m.backgroundRemovalJobs {
		if v.CreatedAt.Before(since) {
			continue
		}
		if v.CreatedAt.Year() == now.Year() && v.CreatedAt.YearDay() == now.YearDay() {
			out.TodayCalls++
		}
		if v.Status == "succeeded" || v.Status == "partial" {
			out.ThirtyDaySucceeded++
		}
		if v.Status == "failed" {
			out.ThirtyDayFailed++
		}
		out.ThirtyDayImages += v.CompletedCount + v.FailedCount
		out.CreditsCharged += v.CreditsCharged
		out.CreditsCalculated += v.CreditsCalculated
	}
	return out, nil
}

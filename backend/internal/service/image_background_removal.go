package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"internal-ai-agent/backend/internal/domain"
	"internal-ai-agent/backend/internal/ids"
	"internal-ai-agent/backend/internal/integration/pixian"
	"internal-ai-agent/backend/internal/store"
)

type PixianConfigInput struct {
	Enabled        bool
	TestMode       bool
	APIID          string
	APISecret      string
	TimeoutSeconds int
	Concurrency    int
	MaxPixels      int
}

type BackgroundRemovalJobInput struct {
	NodeIDs        []string
	Version        int64
	IdempotencyKey string
}

func (s *ImageAgent) PixianConfig(ctx context.Context) (domain.PixianBackgroundRemovalConfig, error) {
	v, err := s.repo.GetPixianBackgroundRemovalConfig(ctx)
	if err != nil {
		return v, err
	}
	return publicPixianConfig(v), nil
}

func publicPixianConfig(v domain.PixianBackgroundRemovalConfig) domain.PixianBackgroundRemovalConfig {
	v.HasAPIID = v.EncryptedAPIID != ""
	v.HasAPISecret = v.EncryptedAPISecret != ""
	v.EncryptedAPIID = ""
	v.EncryptedAPISecret = ""
	return v
}

func (s *ImageAgent) SavePixianConfig(ctx context.Context, actor domain.User, input PixianConfigInput) (domain.PixianBackgroundRemovalConfig, error) {
	v, err := s.repo.GetPixianBackgroundRemovalConfig(ctx)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return v, err
	}
	if input.TimeoutSeconds < 180 || input.TimeoutSeconds > 600 {
		return v, errors.New("抠图超时必须在 180 到 600 秒之间")
	}
	if input.Concurrency < 1 || input.Concurrency > 5 {
		return v, errors.New("抠图并发数必须在 1 到 5 之间")
	}
	if input.MaxPixels < 100 || input.MaxPixels > 25_000_000 {
		return v, errors.New("Pixian 最大输出像素必须在 100 到 25000000 之间")
	}
	if strings.TrimSpace(input.APIID) != "" {
		v.EncryptedAPIID, err = s.secrets.Encrypt(strings.TrimSpace(input.APIID))
		if err != nil {
			return v, err
		}
		v.APIIDHint = secretHint(strings.TrimSpace(input.APIID))
	}
	if strings.TrimSpace(input.APISecret) != "" {
		v.EncryptedAPISecret, err = s.secrets.Encrypt(strings.TrimSpace(input.APISecret))
		if err != nil {
			return v, err
		}
		v.APISecretHint = secretHint(strings.TrimSpace(input.APISecret))
	}
	if input.Enabled && (v.EncryptedAPIID == "" || v.EncryptedAPISecret == "") {
		return v, errors.New("启用抠图服务前必须配置 API ID 和 API Secret")
	}
	v.Enabled = input.Enabled
	v.TestMode = input.TestMode
	v.TimeoutSeconds = input.TimeoutSeconds
	v.Concurrency = input.Concurrency
	v.MaxPixels = input.MaxPixels
	v.UpdatedBy = actor.ID
	v.UpdatedAt = time.Now()
	if err = s.repo.SavePixianBackgroundRemovalConfig(ctx, v); err != nil {
		return v, err
	}
	s.appendAudit(ctx, actor, "image.background_removal.config.update", "pixian_background_removal_config", "pixian", map[string]any{"enabled": v.Enabled, "test_mode": v.TestMode, "timeout_seconds": v.TimeoutSeconds, "concurrency": v.Concurrency, "credentials_updated": strings.TrimSpace(input.APIID) != "" || strings.TrimSpace(input.APISecret) != ""})
	return publicPixianConfig(v), nil
}

func (s *ImageAgent) pixianClient(ctx context.Context) (domain.PixianBackgroundRemovalConfig, *pixian.Client, error) {
	v, err := s.repo.GetPixianBackgroundRemovalConfig(ctx)
	if err != nil {
		return v, nil, err
	}
	if v.EncryptedAPIID == "" || v.EncryptedAPISecret == "" {
		return v, nil, errors.New("Pixian API 凭证尚未配置")
	}
	id, err := s.secrets.Decrypt(v.EncryptedAPIID)
	if err != nil {
		return v, nil, err
	}
	secret, err := s.secrets.Decrypt(v.EncryptedAPISecret)
	if err != nil {
		return v, nil, err
	}
	return v, pixian.New(id, secret, time.Duration(v.TimeoutSeconds)*time.Second), nil
}

func (s *ImageAgent) TestPixian(ctx context.Context, actor domain.User, save bool) (domain.PixianBackgroundRemovalConfig, error) {
	v, client, err := s.pixianClient(ctx)
	if err != nil {
		return publicPixianConfig(v), err
	}
	account, err := client.Account(ctx)
	s.appendAudit(ctx, actor, "image.background_removal.connection.test", "pixian_background_removal_config", "pixian", map[string]any{"success": err == nil})
	if err != nil {
		return publicPixianConfig(v), err
	}
	v.AccountState = account.State
	v.AccountCredits = account.Credits
	now := time.Now()
	v.AccountCheckedAt = &now
	if save {
		v.UpdatedAt = now
		if err = s.repo.SavePixianBackgroundRemovalConfig(ctx, v); err != nil {
			return publicPixianConfig(v), err
		}
	}
	return publicPixianConfig(v), nil
}

func (s *ImageAgent) BackgroundRemovalStatistics(ctx context.Context) (domain.BackgroundRemovalStatistics, error) {
	return s.repo.BackgroundRemovalStatistics(ctx, time.Now().Add(-30*24*time.Hour))
}
func (s *ImageAgent) BackgroundRemovalJobs(ctx context.Context, limit int) ([]domain.BackgroundRemovalJob, error) {
	return s.repo.ListBackgroundRemovalJobs(ctx, limit)
}

func (s *ImageAgent) CreateBackgroundRemovalJob(ctx context.Context, user domain.User, canvasID string, input BackgroundRemovalJobInput) (domain.BackgroundRemovalJob, error) {
	if err := s.ensureAgentEnabled(ctx); err != nil {
		return domain.BackgroundRemovalJob{}, err
	}
	config, err := s.repo.GetPixianBackgroundRemovalConfig(ctx)
	if err != nil {
		return domain.BackgroundRemovalJob{}, err
	}
	if !config.Enabled {
		return domain.BackgroundRemovalJob{}, errors.New("智能抠图服务尚未启用")
	}
	if config.EncryptedAPIID == "" || config.EncryptedAPISecret == "" {
		return domain.BackgroundRemovalJob{}, errors.New("智能抠图服务凭证尚未配置")
	}
	canvas, err := s.CanvasByID(ctx, user, canvasID, false)
	if err != nil {
		return domain.BackgroundRemovalJob{}, err
	}
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if input.IdempotencyKey == "" {
		input.IdempotencyKey = ids.New("bg-remove")
	}
	seen := map[string]bool{}
	nodeIDs := make([]string, 0, len(input.NodeIDs))
	for _, id := range input.NodeIDs {
		id = strings.TrimSpace(id)
		if id != "" && !seen[id] {
			seen[id] = true
			nodeIDs = append(nodeIDs, id)
		}
	}
	if len(nodeIDs) == 0 {
		return domain.BackgroundRemovalJob{}, errors.New("请至少选择一张已完成图片")
	}
	if len(nodeIDs) > 20 {
		return domain.BackgroundRemovalJob{}, errors.New("每次最多抠图 20 张")
	}
	if input.Version < 1 {
		return domain.BackgroundRemovalJob{}, errors.New("画布版本无效")
	}
	byID := map[string]domain.ImageCanvasNode{}
	for _, n := range canvas.Nodes {
		byID[n.ID] = n
	}
	now := time.Now()
	job := domain.BackgroundRemovalJob{ID: ids.New("bgjob"), UserID: user.ID, CanvasID: canvas.ID, Status: "pending", TestMode: config.TestMode, NextAttemptAt: now, IdempotencyKey: input.IdempotencyKey, CreatedAt: now, UpdatedAt: now}
	items := make([]domain.BackgroundRemovalItem, 0, len(nodeIDs))
	nodes := make([]domain.ImageCanvasNode, 0, len(nodeIDs))
	occupied := append([]domain.ImageCanvasNode(nil), canvas.Nodes...)
	maxZ := 0
	for _, n := range occupied {
		if n.ZIndex > maxZ {
			maxZ = n.ZIndex
		}
	}
	for index, nodeID := range nodeIDs {
		source, ok := byID[nodeID]
		if !ok {
			return job, store.ErrNotFound
		}
		if source.Status != "ready" || source.AssetID == "" {
			return job, errors.New("所选内容包含尚未生成完成或没有图片资源的节点")
		}
		asset, getErr := s.repo.GetImageAsset(ctx, source.AssetID)
		if getErr != nil {
			return job, getErr
		}
		if asset.OwnerID != user.ID {
			return job, store.ErrForbidden
		}
		if _, getErr = s.authorizedProject(ctx, user, asset.ProjectID, true); getErr != nil {
			return job, getErr
		}
		if asset.MIMEType != "image/jpeg" && asset.MIMEType != "image/png" && asset.MIMEType != "image/webp" {
			return job, errors.New("Pixian 仅支持 JPG、PNG 和 WEBP 图片")
		}
		if int64(asset.Width)*int64(asset.Height) > 32_000_000 {
			return job, fmt.Errorf("图片 %s 超过 Pixian 3200 万像素限制", asset.FileName)
		}
		block := imageCanvasRect{x: source.X + source.Width + imageCanvasNodeGap, y: source.Y, width: source.Width, height: source.Height}
		block.x = firstFreeCanvasBlockX(block, occupied)
		placeholder := domain.ImageCanvasNode{ID: ids.New("node"), CanvasID: canvas.ID, BackgroundRemovalJobID: job.ID, SourceNodeID: source.ID, OutputIndex: index, Status: "pending", X: block.x, Y: block.y, Width: source.Width, Height: source.Height, ZIndex: maxZ + index + 1,
			GenerationRelayName: source.GenerationRelayName, GenerationModelName: source.GenerationModelName, GenerationModelKey: source.GenerationModelKey,
			CreatedAt: now, UpdatedAt: now}
		item := domain.BackgroundRemovalItem{ID: ids.New("bgitem"), JobID: job.ID, SourceNodeID: source.ID, SourceAssetID: asset.ID, ProjectID: asset.ProjectID, PlaceholderNodeID: placeholder.ID, Status: "pending", CreatedAt: now, UpdatedAt: now}
		nodes = append(nodes, placeholder)
		items = append(items, item)
		occupied = append(occupied, placeholder)
	}
	job.Items = items
	created, err := s.repo.CreateBackgroundRemovalJob(ctx, job, items, nodes, input.Version)
	if err != nil {
		return created, err
	}
	s.appendAudit(ctx, user, "image.background_removal.job.create", "background_removal_job", created.ID, map[string]any{"canvas_id": canvas.ID, "image_count": len(items), "test_mode": config.TestMode, "source_node_ids": nodeIDs})
	return created, nil
}

func (s *ImageAgent) BackgroundRemovalJob(ctx context.Context, user domain.User, id string) (domain.BackgroundRemovalJob, error) {
	v, err := s.repo.GetBackgroundRemovalJob(ctx, id)
	if err == nil && v.UserID != user.ID {
		return v, store.ErrForbidden
	}
	return v, err
}

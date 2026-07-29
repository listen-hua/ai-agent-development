package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	_ "golang.org/x/image/webp"
	"internal-ai-agent/backend/internal/blob"
	"internal-ai-agent/backend/internal/domain"
	"internal-ai-agent/backend/internal/ids"
	"internal-ai-agent/backend/internal/integration/imageproxy"
	"internal-ai-agent/backend/internal/security"
	"internal-ai-agent/backend/internal/store"
)

var (
	imageKeyPattern        = regexp.MustCompile(`^[a-z][a-z0-9_-]{1,49}$`)
	cartoonStrengthPattern = regexp.MustCompile(`(?m)("cartoonization_strength"\s*:\s*)(-?(?:\d+(?:\.\d*)?|\.\d+))`)
	validRatios            = map[string]bool{"1:1": true, "16:9": true, "9:16": true, "4:3": true, "3:4": true}
	validSizes             = map[string]bool{"1K": true, "2K": true, "4K": true}
	validCounts            = map[int]bool{1: true, 2: true, 4: true, 8: true}
)

type ImageRelayInput struct {
	RelayKey           string
	Name               string
	BaseURL            string
	APIKey             string
	Enabled            bool
	TimeoutSeconds     int
	AllowedOutputHosts []string
}

type ImageModelInput struct {
	DisplayName       string
	Protocol          string
	Enabled           bool
	SupportsReference bool
	SupportsReverse   bool
	SupportedSizes    []string
	MaxCount          int
}

type ImageProjectInput struct {
	ProjectKey  string
	Name        string
	Description string
	ACL         domain.ACL
	Enabled     bool
}

type ImagePromptActionInput struct {
	ActionKey      string
	Name           string
	PromptTemplate string
	ProjectID      string
	Enabled        bool
	SortOrder      int
}

type ImageCanvasPatch struct {
	Viewport domain.ImageViewport
	Nodes    []domain.ImageCanvasNode
	Version  int64
}

type ImageCanvasImport struct {
	FileName     string
	DeclaredMIME string
	Data         []byte
	X            float64
	Y            float64
	Version      int64
	Origin       string
}

type ImageJobInput struct {
	ProjectID         string
	RelayID           string
	ModelID           string
	Kind              string
	Prompt            string
	AspectRatio       string
	ImageSize         string
	Count             int
	ReferenceAssetIDs []string
	IdempotencyKey    string
	PlacementX        float64
	PlacementY        float64
	AnchorNodeID      string
}

type ImageAgent struct {
	repo    store.ImageRepository
	audit   store.Repository
	blobs   blob.Store
	scanner security.Scanner
	secrets *security.SecretBox
	agents  *AgentRegistry
}

func NewImageAgent(repo store.ImageRepository, audit store.Repository, blobs blob.Store, scanner security.Scanner, encryptionSecret string, registries ...*AgentRegistry) (*ImageAgent, error) {
	secrets, err := security.NewSecretBox(encryptionSecret)
	if err != nil {
		return nil, err
	}
	value := &ImageAgent{repo: repo, audit: audit, blobs: blobs, scanner: scanner, secrets: secrets}
	if len(registries) > 0 {
		value.agents = registries[0]
	}
	return value, nil
}

func (s *ImageAgent) EnsureDefaults(ctx context.Context) error {
	now := time.Now()
	relays, err := s.repo.ListImageRelays(ctx)
	if err != nil {
		return err
	}
	defaults := []domain.ImageRelay{
		{ID: "00000000-0000-4000-8000-000000000201", RelayKey: "xgapi", Name: "XGAPI", BaseURL: "https://api.xgapi.top/v1", Enabled: true, TimeoutSeconds: 120, AllowedOutputHosts: []string{"api.xgapi.top"}, CreatedAt: now, UpdatedAt: now},
		{ID: "00000000-0000-4000-8000-000000000202", RelayKey: "comfly", Name: "Comfly AI", BaseURL: "https://ai.comfly.org/v1", Enabled: true, TimeoutSeconds: 120, AllowedOutputHosts: []string{"ai.comfly.org"}, CreatedAt: now, UpdatedAt: now},
	}
	existing := map[string]bool{}
	for _, relay := range relays {
		existing[relay.RelayKey] = true
	}
	for _, value := range defaults {
		if !existing[value.RelayKey] {
			if err = s.repo.UpsertImageRelay(ctx, value); err != nil {
				return err
			}
		}
	}
	projects, err := s.repo.ListImageProjects(ctx)
	if err != nil {
		return err
	}
	if len(projects) == 0 {
		value := domain.ImageProject{ID: "00000000-0000-4000-8000-000000000301", ProjectKey: "general",
			Name: "通用创意", Description: "公司通用 AI 生图项目", ACL: domain.ACL{Scope: "all"}, Enabled: true,
			CreatedAt: now, UpdatedAt: now}
		return s.repo.UpsertImageProject(ctx, value)
	}
	return nil
}

func (s *ImageAgent) Options(ctx context.Context, user domain.User) (domain.ImageAgentOptions, error) {
	if err := s.ensureAgentEnabled(ctx); err != nil {
		return domain.ImageAgentOptions{}, err
	}
	relays, err := s.repo.ListImageRelays(ctx)
	if err != nil {
		return domain.ImageAgentOptions{}, err
	}
	models, err := s.repo.ListImageModels(ctx, "")
	if err != nil {
		return domain.ImageAgentOptions{}, err
	}
	projects, err := s.repo.ListImageProjects(ctx)
	if err != nil {
		return domain.ImageAgentOptions{}, err
	}
	result := domain.ImageAgentOptions{Relays: []domain.ImageRelay{}, Models: []domain.ImageModel{}, Projects: []domain.ImageProject{}, PromptActions: []domain.ImagePromptAction{}}
	relayVisible := map[string]bool{}
	for _, relay := range relays {
		if relay.Enabled && relay.EncryptedAPIKey != "" {
			relay.EncryptedAPIKey = ""
			relay.HasAPIKey = true
			result.Relays = append(result.Relays, relay)
			relayVisible[relay.ID] = true
		}
	}
	for _, model := range models {
		if model.Enabled && relayVisible[model.RelayID] {
			result.Models = append(result.Models, model)
		}
	}
	for _, project := range projects {
		if project.ACL.Allows(user) {
			result.Projects = append(result.Projects, project)
		}
	}
	if len(result.Projects) > 0 {
		actions, listErr := s.repo.ListImagePromptActions(ctx, result.Projects[0].ID)
		if listErr != nil {
			return result, listErr
		}
		for _, action := range actions {
			if action.Enabled {
				result.PromptActions = append(result.PromptActions, action)
			}
		}
	}
	return result, nil
}

func (s *ImageAgent) PromptActions(ctx context.Context, user domain.User, projectID string) ([]domain.ImagePromptAction, error) {
	if _, err := s.authorizedProject(ctx, user, projectID, false); err != nil {
		return nil, err
	}
	values, err := s.repo.ListImagePromptActions(ctx, projectID)
	if err != nil {
		return nil, err
	}
	overrides := map[string]domain.ImagePromptAction{}
	for _, value := range values {
		if !value.Enabled {
			continue
		}
		existing, exists := overrides[value.ActionKey]
		if !exists || (existing.ProjectID == "" && value.ProjectID == projectID) {
			overrides[value.ActionKey] = value
		}
	}
	result := make([]domain.ImagePromptAction, 0, len(overrides))
	for _, value := range overrides {
		result = append(result, value)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].SortOrder == result[j].SortOrder {
			return result[i].Name < result[j].Name
		}
		return result[i].SortOrder < result[j].SortOrder
	})
	return result, nil
}

func (s *ImageAgent) ListRelays(ctx context.Context) ([]domain.ImageRelay, error) {
	values, err := s.repo.ListImageRelays(ctx)
	for index := range values {
		values[index].HasAPIKey = values[index].EncryptedAPIKey != ""
		values[index].EncryptedAPIKey = ""
	}
	return values, err
}

func (s *ImageAgent) SaveRelay(ctx context.Context, actor domain.User, id string, input ImageRelayInput) (domain.ImageRelay, error) {
	input.RelayKey = strings.ToLower(strings.TrimSpace(input.RelayKey))
	input.Name = strings.TrimSpace(input.Name)
	input.BaseURL = strings.TrimRight(strings.TrimSpace(input.BaseURL), "/")
	input.APIKey = strings.TrimSpace(input.APIKey)
	if !imageKeyPattern.MatchString(input.RelayKey) || input.Name == "" {
		return domain.ImageRelay{}, errors.New("中转站标识和名称不能为空，标识只能包含小写字母、数字、横线或下划线")
	}
	parsed, err := url.Parse(input.BaseURL)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" {
		return domain.ImageRelay{}, errors.New("中转站 Base URL 必须是有效的 HTTPS 地址")
	}
	if input.TimeoutSeconds == 0 {
		input.TimeoutSeconds = 120
	}
	if input.TimeoutSeconds < 10 || input.TimeoutSeconds > 600 {
		return domain.ImageRelay{}, errors.New("超时时间必须在 10 到 600 秒之间")
	}
	hosts := normalizeHosts(input.AllowedOutputHosts)
	if len(hosts) == 0 {
		hosts = []string{parsed.Hostname()}
	}
	now := time.Now()
	value := domain.ImageRelay{ID: id, RelayKey: input.RelayKey, Name: input.Name, BaseURL: input.BaseURL,
		Enabled: input.Enabled, TimeoutSeconds: input.TimeoutSeconds, AllowedOutputHosts: hosts,
		UpdatedBy: actor.ID, UpdatedAt: now}
	if id == "" {
		value.ID, value.CreatedBy, value.CreatedAt = ids.New("relay"), actor.ID, now
	} else {
		existing, getErr := s.repo.GetImageRelay(ctx, id)
		if getErr != nil {
			return value, getErr
		}
		value.CreatedBy, value.CreatedAt = existing.CreatedBy, existing.CreatedAt
		value.EncryptedAPIKey, value.APIKeyHint = existing.EncryptedAPIKey, existing.APIKeyHint
	}
	if input.APIKey != "" {
		value.EncryptedAPIKey, err = s.secrets.Encrypt(input.APIKey)
		if err != nil {
			return value, err
		}
		value.APIKeyHint = secretHint(input.APIKey)
	}
	if err = s.repo.UpsertImageRelay(ctx, value); err != nil {
		return value, err
	}
	s.appendAudit(ctx, actor, "image.relay.save", "image_relay", value.ID, map[string]any{"relay_key": value.RelayKey, "api_key_updated": input.APIKey != ""})
	value.HasAPIKey, value.EncryptedAPIKey = value.EncryptedAPIKey != "", ""
	return value, nil
}

func (s *ImageAgent) TestRelay(ctx context.Context, actor domain.User, id string) error {
	relay, client, err := s.relayClient(ctx, id)
	if err != nil {
		return err
	}
	err = client.Test(ctx)
	s.appendAudit(ctx, actor, "image.relay.test", "image_relay", relay.ID, map[string]any{"success": err == nil})
	return err
}

func (s *ImageAgent) SyncModels(ctx context.Context, actor domain.User, relayID string) ([]domain.ImageModel, error) {
	relay, client, err := s.relayClient(ctx, relayID)
	if err != nil {
		return nil, err
	}
	remote, err := client.Models(ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	values := make([]domain.ImageModel, 0, len(remote))
	for _, modelID := range remote {
		values = append(values, domain.ImageModel{ID: ids.New("model"), RelayID: relay.ID, ModelID: modelID,
			DisplayName: modelID, Protocol: "chat_completions", Enabled: false, SupportsReverse: true,
			SupportedSizes: []string{"1K", "2K", "4K"}, MaxCount: 1, CreatedAt: now, UpdatedAt: now})
	}
	if err = s.repo.UpsertImageModels(ctx, values); err != nil {
		return nil, err
	}
	s.appendAudit(ctx, actor, "image.models.sync", "image_relay", relay.ID, map[string]any{"discovered": len(values)})
	return s.repo.ListImageModels(ctx, relay.ID)
}

func (s *ImageAgent) ListModels(ctx context.Context, relayID string) ([]domain.ImageModel, error) {
	return s.repo.ListImageModels(ctx, relayID)
}

func (s *ImageAgent) SaveModel(ctx context.Context, actor domain.User, id string, input ImageModelInput) (domain.ImageModel, error) {
	value, err := s.repo.GetImageModel(ctx, id)
	if err != nil {
		return value, err
	}
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	if input.DisplayName == "" {
		input.DisplayName = value.ModelID
	}
	if input.Protocol != "chat_completions" && input.Protocol != "images_generations" {
		return value, errors.New("无效的生图协议")
	}
	if !validCounts[input.MaxCount] {
		return value, errors.New("最大生成数量只能是 1、2、4 或 8")
	}
	sizes := normalizeSizes(input.SupportedSizes)
	if len(sizes) == 0 {
		return value, errors.New("至少需要开放一个分辨率档位")
	}
	value.DisplayName, value.Protocol, value.Enabled = input.DisplayName, input.Protocol, input.Enabled
	value.SupportsReference, value.SupportsReverse = input.SupportsReference, input.SupportsReverse
	value.SupportedSizes, value.MaxCount, value.UpdatedAt = sizes, input.MaxCount, time.Now()
	if err = s.repo.UpdateImageModel(ctx, value); err != nil {
		return value, err
	}
	s.appendAudit(ctx, actor, "image.model.update", "image_model", value.ID, map[string]any{"enabled": value.Enabled, "protocol": value.Protocol})
	return value, nil
}

func (s *ImageAgent) ListProjects(ctx context.Context) ([]domain.ImageProject, error) {
	return s.repo.ListImageProjects(ctx)
}

func (s *ImageAgent) SaveProject(ctx context.Context, actor domain.User, id string, input ImageProjectInput) (domain.ImageProject, error) {
	input.ProjectKey = strings.ToLower(strings.TrimSpace(input.ProjectKey))
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	if !imageKeyPattern.MatchString(input.ProjectKey) || input.Name == "" {
		return domain.ImageProject{}, errors.New("项目标识和名称不能为空")
	}
	if err := input.ACL.Validate(); err != nil {
		return domain.ImageProject{}, err
	}
	now := time.Now()
	value := domain.ImageProject{ID: id, ProjectKey: input.ProjectKey, Name: input.Name, Description: input.Description,
		ACL: input.ACL, Enabled: input.Enabled, UpdatedBy: actor.ID, UpdatedAt: now}
	if id == "" {
		value.ID, value.CreatedBy, value.CreatedAt = ids.New("project"), actor.ID, now
	} else {
		existing, err := s.repo.GetImageProject(ctx, id)
		if err != nil {
			return value, err
		}
		value.CreatedBy, value.CreatedAt = existing.CreatedBy, existing.CreatedAt
	}
	if err := s.repo.UpsertImageProject(ctx, value); err != nil {
		return value, err
	}
	s.appendAudit(ctx, actor, "image.project.save", "image_project", value.ID, map[string]any{"enabled": value.Enabled, "scope": value.ACL.Scope})
	return value, nil
}

func (s *ImageAgent) ListPromptActions(ctx context.Context, projectID string) ([]domain.ImagePromptAction, error) {
	return s.repo.ListImagePromptActions(ctx, projectID)
}

func (s *ImageAgent) SavePromptAction(ctx context.Context, actor domain.User, id string, input ImagePromptActionInput) (domain.ImagePromptAction, error) {
	input.ActionKey = strings.ToLower(strings.TrimSpace(input.ActionKey))
	input.PromptTemplate = strings.TrimSpace(input.PromptTemplate)
	if !imageKeyPattern.MatchString(input.ActionKey) || input.PromptTemplate == "" {
		return domain.ImagePromptAction{}, errors.New("快捷提示词标识格式不正确或模板为空")
	}
	if err := ValidateCartoonStrength(input.PromptTemplate); err != nil {
		return domain.ImagePromptAction{}, err
	}
	if input.ProjectID != "" {
		if _, err := s.repo.GetImageProject(ctx, input.ProjectID); err != nil {
			return domain.ImagePromptAction{}, err
		}
	}
	var existing domain.ImagePromptAction
	if id != "" {
		loaded, err := s.repo.GetImagePromptAction(ctx, id)
		if err != nil {
			return domain.ImagePromptAction{}, err
		}
		existing = loaded
	}
	input.Name = promptActionName(input.Name, input.ActionKey, existing.Name)
	now := time.Now()
	value := domain.ImagePromptAction{ID: id, ActionKey: input.ActionKey, Name: input.Name,
		PromptTemplate: input.PromptTemplate, ProjectID: input.ProjectID, Enabled: input.Enabled,
		SortOrder: input.SortOrder, UpdatedBy: actor.ID, UpdatedAt: now}
	if id == "" {
		value.ID, value.CreatedBy, value.CreatedAt = ids.New("action"), actor.ID, now
	} else {
		value.CreatedBy, value.CreatedAt = existing.CreatedBy, existing.CreatedAt
	}
	if err := s.repo.UpsertImagePromptAction(ctx, value); err != nil {
		return value, err
	}
	s.appendAudit(ctx, actor, "image.prompt_action.save", "image_prompt_action", value.ID, map[string]any{"action_key": value.ActionKey})
	return value, nil
}

func promptActionName(provided, actionKey, existing string) string {
	if value := strings.TrimSpace(provided); value != "" {
		return value
	}
	if value := strings.TrimSpace(existing); value != "" {
		return value
	}
	return actionKey
}

func (s *ImageAgent) DeletePromptAction(ctx context.Context, actor domain.User, id string) error {
	if err := s.repo.DeleteImagePromptAction(ctx, id); err != nil {
		return err
	}
	s.appendAudit(ctx, actor, "image.prompt_action.delete", "image_prompt_action", id, nil)
	return nil
}

func ValidateCartoonStrength(prompt string) error {
	matches := cartoonStrengthPattern.FindAllStringSubmatch(prompt, -1)
	if len(matches) > 1 {
		return errors.New("cartoonization_strength 字段只能出现一次")
	}
	if strings.Count(prompt, `"cartoonization_strength"`) != len(matches) {
		return errors.New("cartoonization_strength 必须是 0 到 1 之间的数字")
	}
	if len(matches) == 1 {
		var strength float64
		if _, err := fmt.Sscanf(matches[0][2], "%f", &strength); err != nil || strength < 0 || strength > 1 {
			return errors.New("cartoonization_strength 必须在 0 到 1 之间")
		}
	}
	return nil
}

func (s *ImageAgent) Canvas(ctx context.Context, user domain.User, projectID string) (domain.ImageCanvas, error) {
	if _, err := s.authorizedProject(ctx, user, projectID, false); err != nil {
		return domain.ImageCanvas{}, err
	}
	return s.repo.GetOrCreateImageCanvas(ctx, user.ID, projectID)
}

func (s *ImageAgent) UpdateCanvas(ctx context.Context, user domain.User, projectID string, input ImageCanvasPatch) (domain.ImageCanvas, error) {
	if _, err := s.authorizedProject(ctx, user, projectID, false); err != nil {
		return domain.ImageCanvas{}, err
	}
	canvas, err := s.repo.GetOrCreateImageCanvas(ctx, user.ID, projectID)
	if err != nil {
		return canvas, err
	}
	if input.Viewport.Zoom < .1 || input.Viewport.Zoom > 2 {
		return canvas, errors.New("画布缩放范围必须在 10% 到 200% 之间")
	}
	canvas.Viewport, canvas.Nodes = input.Viewport, input.Nodes
	return s.repo.UpdateImageCanvas(ctx, canvas, input.Version)
}

func (s *ImageAgent) UploadAsset(ctx context.Context, user domain.User, projectID, fileName, declaredMIME string, data []byte) (domain.ImageAsset, error) {
	if _, err := s.authorizedProject(ctx, user, projectID, true); err != nil {
		return domain.ImageAsset{}, err
	}
	value, err := s.prepareUploadedImage(ctx, user, projectID, fileName, declaredMIME, data)
	if err != nil {
		return domain.ImageAsset{}, err
	}
	if err = s.blobs.Put(ctx, value.ObjectKey, data, value.MIMEType); err != nil {
		return value, err
	}
	if err = s.repo.CreateImageAsset(ctx, value); err != nil {
		_ = s.blobs.Delete(ctx, value.ObjectKey)
		return value, err
	}
	s.appendAudit(ctx, user, "image.asset.upload", "image_asset", value.ID, map[string]any{"project_id": projectID, "size": len(data)})
	return value, nil
}

func (s *ImageAgent) ImportCanvasAsset(ctx context.Context, user domain.User, projectID string, input ImageCanvasImport) (domain.ImageCanvas, error) {
	if _, err := s.authorizedProject(ctx, user, projectID, true); err != nil {
		return domain.ImageCanvas{}, err
	}
	input.Origin = strings.TrimSpace(input.Origin)
	if input.Origin != "paste" && input.Origin != "drop" {
		return domain.ImageCanvas{}, errors.New("图片导入来源只能是 paste 或 drop")
	}
	if input.Version < 1 {
		return domain.ImageCanvas{}, errors.New("画布版本无效")
	}
	if math.IsNaN(input.X) || math.IsNaN(input.Y) || math.IsInf(input.X, 0) || math.IsInf(input.Y, 0) {
		return domain.ImageCanvas{}, errors.New("图片位置无效")
	}
	canvas, err := s.repo.GetOrCreateImageCanvas(ctx, user.ID, projectID)
	if err != nil {
		return canvas, err
	}
	asset, err := s.prepareUploadedImage(ctx, user, projectID, input.FileName, input.DeclaredMIME, input.Data)
	if err != nil {
		return canvas, err
	}
	width, height := importedNodeDimensions(asset.Width, asset.Height)
	now := time.Now()
	node := domain.ImageCanvasNode{
		ID:        ids.New("node"),
		CanvasID:  canvas.ID,
		AssetID:   asset.ID,
		Status:    "ready",
		X:         input.X - width/2,
		Y:         input.Y - height/2,
		Width:     width,
		Height:    height,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err = s.blobs.Put(ctx, asset.ObjectKey, input.Data, asset.MIMEType); err != nil {
		return canvas, err
	}
	updated, err := s.repo.ImportImageCanvasAsset(ctx, canvas, input.Version, asset, node)
	if err != nil {
		_ = s.blobs.Delete(ctx, asset.ObjectKey)
		return canvas, err
	}
	s.appendAudit(ctx, user, "image.asset.canvas_import", "image_asset", asset.ID, map[string]any{
		"project_id": projectID,
		"origin":     input.Origin,
		"size":       len(input.Data),
		"node_id":    node.ID,
	})
	return updated, nil
}

func (s *ImageAgent) prepareUploadedImage(ctx context.Context, user domain.User, projectID, fileName, declaredMIME string, data []byte) (domain.ImageAsset, error) {
	if len(data) == 0 || len(data) > 10<<20 {
		return domain.ImageAsset{}, errors.New("单张图片不能超过 10 MB")
	}
	detected := http.DetectContentType(data)
	if detected != "image/jpeg" && detected != "image/png" && detected != "image/webp" {
		return domain.ImageAsset{}, errors.New("只支持 JPG、PNG 和 WEBP 图片")
	}
	if declaredMIME != "" && !strings.HasPrefix(declaredMIME, "image/") {
		return domain.ImageAsset{}, errors.New("上传文件不是图片")
	}
	if err := s.scanner.Scan(ctx, data); err != nil {
		return domain.ImageAsset{}, fmt.Errorf("图片安全扫描失败: %w", err)
	}
	width, height := imageDimensions(data)
	if width <= 0 || height <= 0 {
		return domain.ImageAsset{}, errors.New("无法读取图片尺寸")
	}
	now := time.Now()
	extension, _ := mime.ExtensionsByType(detected)
	suffix := ".bin"
	if len(extension) > 0 {
		suffix = extension[0]
	}
	value := domain.ImageAsset{ID: ids.New("asset"), OwnerID: user.ID, ProjectID: projectID,
		ObjectKey: "image-agent/" + user.ID + "/" + ids.New("object") + suffix, MIMEType: detected,
		FileName: filepath.Base(fileName), Width: width, Height: height, SizeBytes: int64(len(data)),
		Source: "upload", CreatedAt: now}
	return value, nil
}

func (s *ImageAgent) AssetContent(ctx context.Context, user domain.User, id string) (domain.ImageAsset, []byte, error) {
	value, err := s.repo.GetImageAsset(ctx, id)
	if err != nil {
		return value, nil, err
	}
	if value.OwnerID != user.ID && !user.HasRole(domain.RoleSuperAdmin) {
		return value, nil, store.ErrForbidden
	}
	if _, err = s.authorizedProject(ctx, user, value.ProjectID, false); err != nil {
		return value, nil, err
	}
	data, err := s.blobs.Get(ctx, value.ObjectKey)
	return value, data, err
}

func (s *ImageAgent) CreateJob(ctx context.Context, user domain.User, input ImageJobInput) (domain.ImageJob, error) {
	if err := s.ensureAgentEnabled(ctx); err != nil {
		return domain.ImageJob{}, err
	}
	if _, err := s.authorizedProject(ctx, user, input.ProjectID, true); err != nil {
		return domain.ImageJob{}, err
	}
	relay, err := s.repo.GetImageRelay(ctx, input.RelayID)
	if err != nil {
		return domain.ImageJob{}, err
	}
	model, err := s.repo.GetImageModel(ctx, input.ModelID)
	if err != nil {
		return domain.ImageJob{}, err
	}
	if !relay.Enabled || relay.EncryptedAPIKey == "" || !model.Enabled || model.RelayID != relay.ID {
		return domain.ImageJob{}, errors.New("所选中转站或模型当前不可用")
	}
	if input.Kind == "" {
		input.Kind = "generate"
	}
	if input.Kind != "generate" && input.Kind != "reverse_prompt" {
		return domain.ImageJob{}, errors.New("无效的任务类型")
	}
	input.Prompt = strings.TrimSpace(input.Prompt)
	if input.Kind == "generate" && input.Prompt == "" {
		return domain.ImageJob{}, errors.New("请输入文本描述")
	}
	if input.Kind == "reverse_prompt" && (len(input.ReferenceAssetIDs) == 0 || !model.SupportsReverse) {
		return domain.ImageJob{}, errors.New("当前模型不支持图片反推或尚未上传参考图")
	}
	if len(input.ReferenceAssetIDs) > 5 {
		return domain.ImageJob{}, errors.New("参考图最多 5 张")
	}
	if len(input.ReferenceAssetIDs) > 0 && (!model.SupportsReference && input.Kind == "generate") {
		return domain.ImageJob{}, errors.New("当前模型不支持参考图")
	}
	if model.Protocol == "images_generations" && len(input.ReferenceAssetIDs) > 0 && input.Kind == "generate" {
		return domain.ImageJob{}, errors.New("当前 Images 协议未开放参考图编辑，请选择 Chat 生图模型")
	}
	if !validRatios[input.AspectRatio] || !validSizes[input.ImageSize] || !containsString(model.SupportedSizes, input.ImageSize) {
		return domain.ImageJob{}, errors.New("所选比例或分辨率不受支持")
	}
	if !validCounts[input.Count] || input.Count > model.MaxCount {
		return domain.ImageJob{}, errors.New("生成数量超过模型配置上限")
	}
	if input.Kind == "reverse_prompt" {
		input.Count = 1
	}
	if input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey); input.IdempotencyKey == "" {
		input.IdempotencyKey = ids.New("request")
	}
	var referenceBytes int64
	for _, assetID := range input.ReferenceAssetIDs {
		asset, getErr := s.repo.GetImageAsset(ctx, assetID)
		if getErr != nil {
			return domain.ImageJob{}, getErr
		}
		if asset.OwnerID != user.ID || asset.ProjectID != input.ProjectID {
			return domain.ImageJob{}, store.ErrForbidden
		}
		referenceBytes += asset.SizeBytes
	}
	if referenceBytes > 30<<20 {
		return domain.ImageJob{}, errors.New("参考图总大小不能超过 30 MB")
	}
	now := time.Now()
	job := domain.ImageJob{ID: ids.New("job"), UserID: user.ID, ProjectID: input.ProjectID,
		RelayID: relay.ID, ModelID: model.ID, Kind: input.Kind, Prompt: input.Prompt, AspectRatio: input.AspectRatio,
		ImageSize: input.ImageSize, Count: input.Count, ReferenceAssetIDs: input.ReferenceAssetIDs, Status: "pending",
		NextAttemptAt: now, IdempotencyKey: input.IdempotencyKey, CreatedAt: now, UpdatedAt: now}
	var created domain.ImageJob
	for attempt := 0; attempt < 2; attempt++ {
		var canvas domain.ImageCanvas
		canvas, err = s.repo.GetOrCreateImageCanvas(ctx, user.ID, input.ProjectID)
		if err != nil {
			return domain.ImageJob{}, err
		}
		job.CanvasID = canvas.ID
		nodes := []domain.ImageCanvasNode{}
		if input.Kind == "generate" {
			nodes = placeholderNodes(canvas, job, input.AnchorNodeID, input.PlacementX, input.PlacementY)
		}
		created, err = s.repo.CreateImageJob(ctx, job, nodes, canvas.Version)
		if !errors.Is(err, store.ErrConflict) || input.Kind != "generate" {
			break
		}
	}
	if err == nil {
		s.appendAudit(ctx, user, "image.job.create", "image_job", created.ID, map[string]any{
			"project_id": input.ProjectID, "relay_id": relay.ID, "model_id": model.ID, "kind": input.Kind, "count": input.Count,
		})
	}
	return created, err
}

func (s *ImageAgent) ensureAgentEnabled(ctx context.Context) error {
	if s.agents == nil {
		return nil
	}
	profile, err := s.agents.repo.GetAgentProfileByKey(ctx, "image_generator")
	if err != nil {
		return err
	}
	if !profile.Enabled {
		return errors.New("AI 生图 Agent 已停用")
	}
	return nil
}

func (s *ImageAgent) Job(ctx context.Context, user domain.User, id string) (domain.ImageJob, error) {
	value, err := s.repo.GetImageJob(ctx, id)
	if err != nil {
		return value, err
	}
	if value.UserID != user.ID && !user.HasRole(domain.RoleSuperAdmin) {
		return value, store.ErrForbidden
	}
	return value, nil
}

func (s *ImageAgent) Jobs(ctx context.Context, user domain.User, projectID string) ([]domain.ImageJob, error) {
	if projectID != "" {
		if _, err := s.authorizedProject(ctx, user, projectID, false); err != nil {
			return nil, err
		}
	}
	return s.repo.ListImageJobs(ctx, user.ID, projectID, 50)
}

func (s *ImageAgent) authorizedProject(ctx context.Context, user domain.User, id string, requireEnabled bool) (domain.ImageProject, error) {
	project, err := s.repo.GetImageProject(ctx, id)
	if err != nil {
		return project, err
	}
	if !project.ACL.Allows(user) {
		return project, store.ErrForbidden
	}
	if requireEnabled && !project.Enabled {
		return project, errors.New("项目已停用，历史画布仅可查看")
	}
	return project, nil
}

func (s *ImageAgent) relayClient(ctx context.Context, id string) (domain.ImageRelay, *imageproxy.Client, error) {
	relay, err := s.repo.GetImageRelay(ctx, id)
	if err != nil {
		return relay, nil, err
	}
	if relay.EncryptedAPIKey == "" {
		return relay, nil, errors.New("中转站 API Key 尚未配置")
	}
	key, err := s.secrets.Decrypt(relay.EncryptedAPIKey)
	if err != nil {
		return relay, nil, err
	}
	client, err := imageproxy.New(relay.BaseURL, key, time.Duration(relay.TimeoutSeconds)*time.Second, relay.AllowedOutputHosts)
	return relay, client, err
}

func (s *ImageAgent) appendAudit(ctx context.Context, actor domain.User, action, resourceType, resourceID string, metadata map[string]any) {
	if s.audit == nil {
		return
	}
	_ = s.audit.AppendAudit(ctx, domain.AuditEvent{ID: ids.New("audit"), ActorID: actor.ID, ActorName: actor.Name,
		Action: action, ResourceType: resourceType, ResourceID: resourceID, Metadata: metadata, CreatedAt: time.Now()})
}

const imageCanvasNodeGap = 24.0

type imageCanvasRect struct {
	x      float64
	y      float64
	width  float64
	height float64
}

func placeholderNodes(canvas domain.ImageCanvas, job domain.ImageJob, anchorNodeID string, x, y float64) []domain.ImageCanvasNode {
	if x == 0 && y == 0 {
		zoom := canvas.Viewport.Zoom
		if zoom <= 0 {
			zoom = 1
		}
		x, y = (600-canvas.Viewport.X)/zoom, (360-canvas.Viewport.Y)/zoom
	}
	width, height := nodeDimensions(job.AspectRatio)
	result := make([]domain.ImageCanvasNode, 0, job.Count)
	columns := 2
	if job.Count == 1 {
		columns = 1
	}
	rows := (job.Count + columns - 1) / columns
	block := imageCanvasRect{
		width:  float64(min(job.Count, columns))*width + float64(min(job.Count, columns)-1)*imageCanvasNodeGap,
		height: float64(rows)*height + float64(rows-1)*imageCanvasNodeGap,
	}
	if len(canvas.Nodes) == 0 {
		block.x, block.y = x-block.width/2, y-block.height/2
	} else {
		anchor := nearestImageCanvasNode(canvas.Nodes, anchorNodeID, x, y)
		block.x, block.y = anchor.X+anchor.Width+imageCanvasNodeGap, anchor.Y
		block.x = firstFreeCanvasBlockX(block, canvas.Nodes)
	}
	maxZ := 0
	for _, node := range canvas.Nodes {
		if node.ZIndex > maxZ {
			maxZ = node.ZIndex
		}
	}
	for index := 0; index < job.Count; index++ {
		column, row := index%columns, index/columns
		now := time.Now()
		result = append(result, domain.ImageCanvasNode{ID: ids.New("node"), CanvasID: canvas.ID, JobID: job.ID,
			OutputIndex: index, Status: "pending",
			X:     block.x + float64(column)*(width+imageCanvasNodeGap),
			Y:     block.y + float64(row)*(height+imageCanvasNodeGap),
			Width: width, Height: height, ZIndex: maxZ + index + 1, CreatedAt: now, UpdatedAt: now})
	}
	return result
}

func nearestImageCanvasNode(nodes []domain.ImageCanvasNode, preferredID string, x, y float64) domain.ImageCanvasNode {
	for _, node := range nodes {
		if preferredID != "" && node.ID == preferredID {
			return node
		}
	}
	nearest := nodes[0]
	nearestDistance := math.MaxFloat64
	for _, node := range nodes {
		centerX, centerY := node.X+node.Width/2, node.Y+node.Height/2
		distance := (centerX-x)*(centerX-x) + (centerY-y)*(centerY-y)
		if distance < nearestDistance {
			nearest, nearestDistance = node, distance
		}
	}
	return nearest
}

func firstFreeCanvasBlockX(block imageCanvasRect, nodes []domain.ImageCanvasNode) float64 {
	for attempt := 0; attempt <= len(nodes); attempt++ {
		nextX := block.x
		collided := false
		for _, node := range nodes {
			target := imageCanvasRect{x: node.X, y: node.Y, width: node.Width, height: node.Height}
			if canvasRectsRespectGap(block, target, imageCanvasNodeGap) {
				continue
			}
			collided = true
			if candidate := node.X + node.Width + imageCanvasNodeGap; candidate > nextX {
				nextX = candidate
			}
		}
		if !collided {
			return block.x
		}
		block.x = nextX
	}
	return block.x
}

func canvasRectsRespectGap(left, right imageCanvasRect, gap float64) bool {
	return left.x+left.width+gap <= right.x ||
		right.x+right.width+gap <= left.x ||
		left.y+left.height+gap <= right.y ||
		right.y+right.height+gap <= left.y
}

func nodeDimensions(ratio string) (float64, float64) {
	parts := strings.Split(ratio, ":")
	var widthRatio, heightRatio float64 = 1, 1
	if len(parts) == 2 {
		_, _ = fmt.Sscanf(parts[0], "%f", &widthRatio)
		_, _ = fmt.Sscanf(parts[1], "%f", &heightRatio)
	}
	if widthRatio >= heightRatio {
		return 360, 360 * heightRatio / widthRatio
	}
	return 360 * widthRatio / heightRatio, 360
}

func importedNodeDimensions(pixelWidth, pixelHeight int) (float64, float64) {
	if pixelWidth <= 0 || pixelHeight <= 0 {
		return 360, 360
	}
	width, height := float64(pixelWidth), float64(pixelHeight)
	scale := 360 / math.Max(width, height)
	return width * scale, height * scale
}

func imageDimensions(data []byte) (int, int) {
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return 0, 0
	}
	return config.Width, config.Height
}

func normalizeHosts(values []string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		value = strings.TrimPrefix(value, "https://")
		value = strings.Split(value, "/")[0]
		value = strings.Split(value, ":")[0]
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}

func normalizeSizes(values []string) []string {
	result := []string{}
	for _, candidate := range []string{"1K", "2K", "4K"} {
		if containsString(values, candidate) {
			result = append(result, candidate)
		}
	}
	return result
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func secretHint(value string) string {
	runes := []rune(value)
	if len(runes) <= 4 {
		return "••••"
	}
	return "••••" + string(runes[len(runes)-4:])
}

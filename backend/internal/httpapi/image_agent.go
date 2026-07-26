package httpapi

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"internal-ai-agent/backend/internal/domain"
	"internal-ai-agent/backend/internal/service"
	"internal-ai-agent/backend/internal/store"
)

type imageRelayPayload struct {
	RelayKey           string   `json:"relay_key"`
	Name               string   `json:"name"`
	BaseURL            string   `json:"base_url"`
	APIKey             string   `json:"api_key"`
	Enabled            bool     `json:"enabled"`
	TimeoutSeconds     int      `json:"timeout_seconds"`
	AllowedOutputHosts []string `json:"allowed_output_hosts"`
}

func (s *Server) imageAgentOptions(w http.ResponseWriter, r *http.Request) {
	value, err := s.imageAgent.Options(r.Context(), currentUser(r))
	if err == nil {
		if projectID := strings.TrimSpace(r.URL.Query().Get("project_id")); projectID != "" {
			value.PromptActions, err = s.imageAgent.PromptActions(r.Context(), currentUser(r), projectID)
		}
	}
	respondImage(w, value, err)
}

func (s *Server) imageCanvas(w http.ResponseWriter, r *http.Request) {
	value, err := s.imageAgent.Canvas(r.Context(), currentUser(r), r.PathValue("id"))
	respondImage(w, value, err)
}

func (s *Server) updateImageCanvas(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Viewport domain.ImageViewport     `json:"viewport"`
		Nodes    []domain.ImageCanvasNode `json:"nodes"`
		Version  int64                    `json:"version"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	value, err := s.imageAgent.UpdateCanvas(r.Context(), currentUser(r), r.PathValue("id"), service.ImageCanvasPatch{
		Viewport: input.Viewport, Nodes: input.Nodes, Version: input.Version,
	})
	respondImage(w, value, err)
}

func (s *Server) uploadImageAsset(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 31<<20)
	if err := r.ParseMultipartForm(31 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "上传内容不能超过 30 MB", err)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "请选择图片", err)
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, (10<<20)+1))
	if err != nil {
		writeError(w, http.StatusBadRequest, "读取图片失败", err)
		return
	}
	value, err := s.imageAgent.UploadAsset(r.Context(), currentUser(r), r.FormValue("project_id"), header.Filename, header.Header.Get("Content-Type"), data)
	respondImage(w, value, err)
}

func (s *Server) imageAssetContent(w http.ResponseWriter, r *http.Request) {
	asset, data, err := s.imageAgent.AssetContent(r.Context(), currentUser(r), r.PathValue("id"))
	if err != nil {
		respondImage(w, nil, err)
		return
	}
	w.Header().Set("Content-Type", asset.MIMEType)
	w.Header().Set("Content-Disposition", `inline; filename="`+strings.ReplaceAll(asset.FileName, `"`, "")+`"`)
	w.Header().Set("Cache-Control", "private, max-age=3600")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (s *Server) createImageJob(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ProjectID         string   `json:"project_id"`
		RelayID           string   `json:"relay_id"`
		ModelID           string   `json:"model_id"`
		Kind              string   `json:"kind"`
		Prompt            string   `json:"prompt"`
		AspectRatio       string   `json:"aspect_ratio"`
		ImageSize         string   `json:"image_size"`
		Count             int      `json:"count"`
		ReferenceAssetIDs []string `json:"reference_asset_ids"`
		IdempotencyKey    string   `json:"idempotency_key"`
		PlacementX        float64  `json:"placement_x"`
		PlacementY        float64  `json:"placement_y"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	value, err := s.imageAgent.CreateJob(r.Context(), currentUser(r), service.ImageJobInput{
		ProjectID: input.ProjectID, RelayID: input.RelayID, ModelID: input.ModelID, Kind: input.Kind,
		Prompt: input.Prompt, AspectRatio: input.AspectRatio, ImageSize: input.ImageSize, Count: input.Count,
		ReferenceAssetIDs: input.ReferenceAssetIDs, IdempotencyKey: input.IdempotencyKey,
		PlacementX: input.PlacementX, PlacementY: input.PlacementY,
	})
	if err != nil {
		respondImage(w, value, err)
		return
	}
	writeJSON(w, http.StatusAccepted, value)
}

func (s *Server) getImageJob(w http.ResponseWriter, r *http.Request) {
	value, err := s.imageAgent.Job(r.Context(), currentUser(r), r.PathValue("id"))
	respondImage(w, value, err)
}

func (s *Server) listImageJobs(w http.ResponseWriter, r *http.Request) {
	value, err := s.imageAgent.Jobs(r.Context(), currentUser(r), r.URL.Query().Get("project_id"))
	respondImage(w, value, err)
}

func (s *Server) listImageRelays(w http.ResponseWriter, r *http.Request) {
	value, err := s.imageAgent.ListRelays(r.Context())
	respondImage(w, value, err)
}

func (s *Server) createImageRelay(w http.ResponseWriter, r *http.Request) {
	var input imageRelayPayload
	if !decodeJSON(w, r, &input) {
		return
	}
	value, err := s.imageAgent.SaveRelay(r.Context(), currentUser(r), "", service.ImageRelayInput{
		RelayKey: input.RelayKey, Name: input.Name, BaseURL: input.BaseURL, APIKey: input.APIKey,
		Enabled: input.Enabled, TimeoutSeconds: input.TimeoutSeconds, AllowedOutputHosts: input.AllowedOutputHosts,
	})
	if err == nil {
		writeJSON(w, http.StatusCreated, value)
		return
	}
	respondImage(w, value, err)
}

func (s *Server) updateImageRelay(w http.ResponseWriter, r *http.Request) {
	var input imageRelayPayload
	if !decodeJSON(w, r, &input) {
		return
	}
	value, err := s.imageAgent.SaveRelay(r.Context(), currentUser(r), r.PathValue("id"), service.ImageRelayInput{
		RelayKey: input.RelayKey, Name: input.Name, BaseURL: input.BaseURL, APIKey: input.APIKey,
		Enabled: input.Enabled, TimeoutSeconds: input.TimeoutSeconds, AllowedOutputHosts: input.AllowedOutputHosts,
	})
	respondImage(w, value, err)
}

func (s *Server) testImageRelay(w http.ResponseWriter, r *http.Request) {
	err := s.imageAgent.TestRelay(r.Context(), currentUser(r), r.PathValue("id"))
	respondImage(w, map[string]bool{"ok": err == nil}, err)
}

func (s *Server) syncImageModels(w http.ResponseWriter, r *http.Request) {
	value, err := s.imageAgent.SyncModels(r.Context(), currentUser(r), r.PathValue("id"))
	respondImage(w, value, err)
}

func (s *Server) listImageModels(w http.ResponseWriter, r *http.Request) {
	value, err := s.imageAgent.ListModels(r.Context(), r.URL.Query().Get("relay_id"))
	respondImage(w, value, err)
}

func (s *Server) updateImageModel(w http.ResponseWriter, r *http.Request) {
	var input struct {
		DisplayName       string   `json:"display_name"`
		Protocol          string   `json:"protocol"`
		Enabled           bool     `json:"enabled"`
		SupportsReference bool     `json:"supports_reference"`
		SupportsReverse   bool     `json:"supports_reverse"`
		SupportedSizes    []string `json:"supported_sizes"`
		MaxCount          int      `json:"max_count"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	value, err := s.imageAgent.SaveModel(r.Context(), currentUser(r), r.PathValue("id"), service.ImageModelInput{
		DisplayName: input.DisplayName, Protocol: input.Protocol, Enabled: input.Enabled,
		SupportsReference: input.SupportsReference, SupportsReverse: input.SupportsReverse,
		SupportedSizes: input.SupportedSizes, MaxCount: input.MaxCount,
	})
	respondImage(w, value, err)
}

type imageProjectPayload struct {
	ProjectKey  string     `json:"project_key"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	ACL         domain.ACL `json:"acl"`
	Enabled     bool       `json:"enabled"`
}

func (s *Server) listImageProjects(w http.ResponseWriter, r *http.Request) {
	value, err := s.imageAgent.ListProjects(r.Context())
	respondImage(w, value, err)
}

func (s *Server) createImageProject(w http.ResponseWriter, r *http.Request) {
	var input imageProjectPayload
	if !decodeJSON(w, r, &input) {
		return
	}
	value, err := s.imageAgent.SaveProject(r.Context(), currentUser(r), "", service.ImageProjectInput{
		ProjectKey: input.ProjectKey, Name: input.Name, Description: input.Description, ACL: input.ACL, Enabled: input.Enabled,
	})
	if err == nil {
		writeJSON(w, http.StatusCreated, value)
		return
	}
	respondImage(w, value, err)
}

func (s *Server) updateImageProject(w http.ResponseWriter, r *http.Request) {
	var input imageProjectPayload
	if !decodeJSON(w, r, &input) {
		return
	}
	value, err := s.imageAgent.SaveProject(r.Context(), currentUser(r), r.PathValue("id"), service.ImageProjectInput{
		ProjectKey: input.ProjectKey, Name: input.Name, Description: input.Description, ACL: input.ACL, Enabled: input.Enabled,
	})
	respondImage(w, value, err)
}

type imagePromptActionPayload struct {
	ActionKey      string `json:"action_key"`
	Name           string `json:"name"`
	PromptTemplate string `json:"prompt_template"`
	ProjectID      string `json:"project_id"`
	Enabled        bool   `json:"enabled"`
	SortOrder      int    `json:"sort_order"`
}

func (s *Server) listImagePromptActions(w http.ResponseWriter, r *http.Request) {
	value, err := s.imageAgent.ListPromptActions(r.Context(), r.URL.Query().Get("project_id"))
	respondImage(w, value, err)
}

func (s *Server) createImagePromptAction(w http.ResponseWriter, r *http.Request) {
	var input imagePromptActionPayload
	if !decodeJSON(w, r, &input) {
		return
	}
	value, err := s.imageAgent.SavePromptAction(r.Context(), currentUser(r), "", service.ImagePromptActionInput{
		ActionKey: input.ActionKey, Name: input.Name, PromptTemplate: input.PromptTemplate,
		ProjectID: input.ProjectID, Enabled: input.Enabled, SortOrder: input.SortOrder,
	})
	if err == nil {
		writeJSON(w, http.StatusCreated, value)
		return
	}
	respondImage(w, value, err)
}

func (s *Server) updateImagePromptAction(w http.ResponseWriter, r *http.Request) {
	var input imagePromptActionPayload
	if !decodeJSON(w, r, &input) {
		return
	}
	value, err := s.imageAgent.SavePromptAction(r.Context(), currentUser(r), r.PathValue("id"), service.ImagePromptActionInput{
		ActionKey: input.ActionKey, Name: input.Name, PromptTemplate: input.PromptTemplate,
		ProjectID: input.ProjectID, Enabled: input.Enabled, SortOrder: input.SortOrder,
	})
	respondImage(w, value, err)
}

func (s *Server) deleteImagePromptAction(w http.ResponseWriter, r *http.Request) {
	err := s.imageAgent.DeletePromptAction(r.Context(), currentUser(r), r.PathValue("id"))
	if err != nil {
		respondImage(w, nil, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func respondImage(w http.ResponseWriter, value any, err error) {
	if err == nil {
		writeJSON(w, http.StatusOK, value)
		return
	}
	status := http.StatusBadRequest
	switch {
	case errors.Is(err, store.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, store.ErrForbidden):
		status = http.StatusForbidden
	case errors.Is(err, store.ErrConflict):
		status = http.StatusConflict
	}
	writeError(w, status, "请求失败", err)
}

func parseInt(value string, fallback int) int {
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

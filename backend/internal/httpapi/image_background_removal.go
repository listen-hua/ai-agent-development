package httpapi

import (
	"net/http"

	"internal-ai-agent/backend/internal/service"
)

func (s *Server) createBackgroundRemovalJob(w http.ResponseWriter, r *http.Request) {
	var input struct {
		NodeIDs        []string `json:"node_ids"`
		Version        int64    `json:"version"`
		IdempotencyKey string   `json:"idempotency_key"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	v, err := s.imageAgent.CreateBackgroundRemovalJob(r.Context(), currentUser(r), r.PathValue("canvasID"), service.BackgroundRemovalJobInput{NodeIDs: input.NodeIDs, Version: input.Version, IdempotencyKey: input.IdempotencyKey})
	if err == nil {
		writeJSON(w, http.StatusCreated, v)
		return
	}
	respondImage(w, v, err)
}
func (s *Server) getBackgroundRemovalJob(w http.ResponseWriter, r *http.Request) {
	v, err := s.imageAgent.BackgroundRemovalJob(r.Context(), currentUser(r), r.PathValue("jobID"))
	respondImage(w, v, err)
}

type pixianConfigPayload struct {
	Enabled        bool   `json:"enabled"`
	TestMode       bool   `json:"test_mode"`
	APIID          string `json:"api_id"`
	APISecret      string `json:"api_secret"`
	TimeoutSeconds int    `json:"timeout_seconds"`
	Concurrency    int    `json:"concurrency"`
	MaxPixels      int    `json:"max_pixels"`
}

func (s *Server) getPixianConfig(w http.ResponseWriter, r *http.Request) {
	v, err := s.imageAgent.PixianConfig(r.Context())
	respondImage(w, v, err)
}
func (s *Server) updatePixianConfig(w http.ResponseWriter, r *http.Request) {
	var input pixianConfigPayload
	if !decodeJSON(w, r, &input) {
		return
	}
	v, err := s.imageAgent.SavePixianConfig(r.Context(), currentUser(r), service.PixianConfigInput{Enabled: input.Enabled, TestMode: input.TestMode, APIID: input.APIID, APISecret: input.APISecret, TimeoutSeconds: input.TimeoutSeconds, Concurrency: input.Concurrency, MaxPixels: input.MaxPixels})
	respondImage(w, v, err)
}
func (s *Server) testPixianConnection(w http.ResponseWriter, r *http.Request) {
	v, err := s.imageAgent.TestPixian(r.Context(), currentUser(r), false)
	respondImage(w, v, err)
}
func (s *Server) refreshPixianAccount(w http.ResponseWriter, r *http.Request) {
	v, err := s.imageAgent.TestPixian(r.Context(), currentUser(r), true)
	respondImage(w, v, err)
}
func (s *Server) pixianStatistics(w http.ResponseWriter, r *http.Request) {
	v, err := s.imageAgent.BackgroundRemovalStatistics(r.Context())
	respondImage(w, v, err)
}
func (s *Server) listBackgroundRemovalJobs(w http.ResponseWriter, r *http.Request) {
	v, err := s.imageAgent.BackgroundRemovalJobs(r.Context(), parseInt(r.URL.Query().Get("limit"), 100))
	respondImage(w, v, err)
}

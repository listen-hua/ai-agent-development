package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"internal-ai-agent/backend/internal/domain"
	"internal-ai-agent/backend/internal/store"
)

func (s *Server) massageMe(w http.ResponseWriter, r *http.Request) {
	values, err := s.massage.MyCycles(r.Context(), currentUser(r))
	respond(w, values, err)
}
func (s *Server) massageResponse(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Action string `json:"action"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	value, err := s.massage.Respond(r.Context(), currentUser(r), r.PathValue("id"), strings.TrimSpace(input.Action))
	if err != nil {
		writeMassageError(w, "按摩排号操作失败", err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}
func (s *Server) listMassageCycles(w http.ResponseWriter, r *http.Request) {
	values, err := s.massage.ListCycles(r.Context())
	respond(w, values, err)
}
func (s *Server) getMassageCycle(w http.ResponseWriter, r *http.Request) {
	value, err := s.massage.GetCycle(r.Context(), r.PathValue("id"))
	respond(w, value, err)
}
func (s *Server) createMassageCycle(w http.ResponseWriter, r *http.Request) {
	var input domain.MassageCycle
	if !decodeJSON(w, r, &input) {
		return
	}
	value, err := s.massage.CreateCycle(r.Context(), currentUser(r), input)
	if err != nil {
		writeMassageError(w, "创建按摩批次失败", err)
		return
	}
	writeJSON(w, http.StatusCreated, value)
}
func (s *Server) updateMassageCycle(w http.ResponseWriter, r *http.Request) {
	var input domain.MassageCycle
	if !decodeJSON(w, r, &input) {
		return
	}
	input.ID = r.PathValue("id")
	value, err := s.massage.UpdateCycle(r.Context(), currentUser(r), input)
	if err != nil {
		writeMassageError(w, "更新按摩批次失败", err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}
func (s *Server) deleteMassageCycle(w http.ResponseWriter, r *http.Request) {
	if err := s.massage.DeleteCycle(r.Context(), currentUser(r), r.PathValue("id")); err != nil {
		writeMassageError(w, "删除按摩批次失败", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (s *Server) publishMassageCycle(w http.ResponseWriter, r *http.Request) {
	value, err := s.massage.Publish(r.Context(), currentUser(r), r.PathValue("id"))
	if err != nil {
		writeMassageError(w, "发布按摩报名失败", err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}
func (s *Server) resendMassageSignup(w http.ResponseWriter, r *http.Request) {
	count, err := s.massage.Resend(r.Context(), currentUser(r), r.PathValue("id"))
	if err != nil {
		writeMassageError(w, "补发报名通知失败", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"count": count})
}
func (s *Server) massageSessionAction(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Reason string `json:"reason"`
	}
	if r.ContentLength > 0 && !decodeJSON(w, r, &input) {
		return
	}
	value, err := s.massage.SessionAction(r.Context(), currentUser(r), r.PathValue("id"), r.PathValue("action"), input.Reason)
	if err != nil {
		writeMassageError(w, "场次操作失败", err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}
func (s *Server) massageCallAction(w http.ResponseWriter, r *http.Request) {
	action := r.PathValue("action")
	if action != "complete" && action != "no-show" {
		writeError(w, http.StatusNotFound, "操作不存在", nil)
		return
	}
	if action == "no-show" {
		action = "no_show"
	}
	value, err := s.massage.AdminCallAction(r.Context(), currentUser(r), r.PathValue("id"), action)
	if err != nil {
		writeMassageError(w, "叫号操作失败", err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}
func (s *Server) massageStatistics(w http.ResponseWriter, r *http.Request) {
	value, err := s.massage.Statistics(r.Context(), r.PathValue("id"))
	respond(w, value, err)
}
func (s *Server) massageStatisticsCSV(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="massage-statistics.csv"`)
	if err := s.massage.WriteCSV(r.Context(), r.PathValue("id"), w); err != nil {
		writeMassageError(w, "导出统计失败", err)
	}
}
func writeMassageError(w http.ResponseWriter, message string, err error) {
	status := http.StatusBadRequest
	if errors.Is(err, store.ErrForbidden) {
		status = http.StatusForbidden
	} else if errors.Is(err, store.ErrNotFound) {
		status = http.StatusNotFound
	} else if errors.Is(err, store.ErrConflict) {
		status = http.StatusConflict
	}
	writeError(w, status, message, err)
}

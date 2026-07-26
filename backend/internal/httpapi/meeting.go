package httpapi

import (
	"errors"
	"net/http"

	"internal-ai-agent/backend/internal/store"
)

func (s *Server) confirmMeetingBookingAction(w http.ResponseWriter, r *http.Request) {
	if s.meetings == nil {
		writeError(w, http.StatusServiceUnavailable, "会议室预约服务未启用", nil)
		return
	}
	var input struct {
		OptionID string `json:"option_id"`
	}
	if r.ContentLength > 0 && !decodeJSON(w, r, &input) {
		return
	}
	action, booking, err := s.meetings.Confirm(r.Context(), currentUser(r), r.PathValue("id"), input.OptionID)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, store.ErrForbidden) {
			status = http.StatusForbidden
		} else if errors.Is(err, store.ErrConflict) {
			status = http.StatusConflict
		}
		writeError(w, status, "确认会议室预约失败", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"action": action, "booking": booking})
}

func (s *Server) cancelMeetingBookingAction(w http.ResponseWriter, r *http.Request) {
	if s.meetings == nil {
		writeError(w, http.StatusServiceUnavailable, "会议室预约服务未启用", nil)
		return
	}
	action, err := s.meetings.CancelAction(r.Context(), currentUser(r), r.PathValue("id"))
	respond(w, action, err)
}

func (s *Server) listMeetingBookings(w http.ResponseWriter, r *http.Request) {
	if s.meetings == nil {
		writeError(w, http.StatusServiceUnavailable, "会议室预约服务未启用", nil)
		return
	}
	values, err := s.meetings.ListBookings(r.Context(), currentUser(r))
	respond(w, values, err)
}

func (s *Server) listMeetingRooms(w http.ResponseWriter, r *http.Request) {
	rooms, settings, err := s.meetings.Rooms(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取会议室失败", err)
		return
	}
	alerts, err := s.meetings.BookingAlerts(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取异常预约失败", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"rooms": rooms, "settings": settings, "booking_alerts": alerts})
}

func (s *Server) syncMeetingRooms(w http.ResponseWriter, r *http.Request) {
	actor := currentUser(r)
	rooms, err := s.meetings.SyncRooms(r.Context(), &actor)
	respond(w, rooms, err)
}

func (s *Server) initializeMeetingCalendar(w http.ResponseWriter, r *http.Request) {
	settings, err := s.meetings.InitializeCalendar(r.Context(), currentUser(r))
	respond(w, settings, err)
}

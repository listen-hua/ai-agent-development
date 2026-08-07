package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"internal-ai-agent/backend/internal/service"
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

func (s *Server) prepareMeetingBookingCancellation(w http.ResponseWriter, r *http.Request) {
	if s.meetings == nil {
		writeError(w, http.StatusServiceUnavailable, "会议室预约服务未启用", nil)
		return
	}
	action, err := s.meetings.PrepareCancellationForBooking(r.Context(), currentUser(r), r.PathValue("id"), "h5", "")
	if err != nil {
		writeMeetingError(w, "创建取消确认失败", err)
		return
	}
	writeJSON(w, http.StatusCreated, action)
}

func (s *Server) searchMeetingAttendees(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	values, err := s.meetings.SearchAttendees(r.Context(), currentUser(r), strings.TrimSpace(r.URL.Query().Get("q")), limit)
	if err != nil {
		writeMeetingError(w, "搜索飞书员工失败", err)
		return
	}
	if s.directory != nil {
		if departments, directoryErr := s.directory.Departments(r.Context()); directoryErr == nil {
			names := make(map[string]string, len(departments))
			for _, department := range departments {
				names[department.OpenDepartmentID] = department.Name
			}
			for index := range values {
				for _, departmentID := range values[index].DepartmentIDs {
					if name := names[departmentID]; name != "" {
						values[index].DepartmentNames = append(values[index].DepartmentNames, name)
					}
				}
			}
		}
	}
	writeJSON(w, http.StatusOK, values)
}

func (s *Server) updateMeetingBookingDraft(w http.ResponseWriter, r *http.Request) {
	var input service.MeetingBookingDraftUpdate
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := s.meetings.UpdateDraft(r.Context(), currentUser(r), r.PathValue("id"), input)
	if err != nil {
		writeMeetingError(w, "更新会议预约信息失败", err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func writeMeetingError(w http.ResponseWriter, message string, err error) {
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

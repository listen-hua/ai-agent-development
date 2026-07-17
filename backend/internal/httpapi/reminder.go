package httpapi

import (
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"internal-ai-agent/backend/internal/domain"
	"internal-ai-agent/backend/internal/ids"
)

func (s *Server) listReminders(w http.ResponseWriter, r *http.Request) {
	values, err := s.reminders.List(r.Context(), currentUser(r))
	respond(w, values, err)
}

func (s *Server) getReminder(w http.ResponseWriter, r *http.Request) {
	value, err := s.reminders.Get(r.Context(), currentUser(r), r.PathValue("id"))
	respond(w, value, err)
}

func (s *Server) listReminderDeliveries(w http.ResponseWriter, r *http.Request) {
	values, err := s.reminders.Deliveries(r.Context(), currentUser(r), r.PathValue("id"))
	respond(w, values, err)
}

func (s *Server) createNewReminderAction(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Content  string                  `json:"content"`
		Schedule domain.ReminderSchedule `json:"schedule"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	value, err := s.reminders.CreateAction(r.Context(), currentUser(r), "create", "", input.Content, &input.Schedule, "h5", "")
	if err != nil {
		writeError(w, http.StatusBadRequest, "创建提醒操作失败", err)
		return
	}
	writeJSON(w, http.StatusCreated, value)
}

func (s *Server) createReminderAction(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Action   string                   `json:"action"`
		Content  string                   `json:"content"`
		Schedule *domain.ReminderSchedule `json:"schedule"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.Action != "update" && input.Action != "pause" && input.Action != "resume" && input.Action != "delete" {
		writeError(w, http.StatusBadRequest, "不支持的提醒操作", nil)
		return
	}
	value, err := s.reminders.CreateAction(r.Context(), currentUser(r), input.Action, r.PathValue("id"), input.Content, input.Schedule, "h5", "")
	if err != nil {
		writeError(w, http.StatusBadRequest, "创建提醒操作失败", err)
		return
	}
	writeJSON(w, http.StatusCreated, value)
}

func (s *Server) confirmReminderAction(w http.ResponseWriter, r *http.Request) {
	action, reminder, err := s.reminders.Confirm(r.Context(), currentUser(r), r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusConflict, "提醒无法确认", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"action": action, "reminder": reminder})
}

func (s *Server) cancelReminderAction(w http.ResponseWriter, r *http.Request) {
	value, err := s.reminders.CancelAction(r.Context(), currentUser(r), r.PathValue("id"))
	respond(w, value, err)
}

func (s *Server) listWorkCalendar(w http.ResponseWriter, r *http.Request) {
	year := time.Now().Year()
	if value := r.URL.Query().Get("year"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 2020 || parsed > 2100 {
			writeError(w, http.StatusBadRequest, "年份无效", err)
			return
		}
		year = parsed
	}
	values, err := s.repo.ListWorkdayOverrides(r.Context(), year)
	respond(w, values, err)
}

func (s *Server) upsertWorkCalendar(w http.ResponseWriter, r *http.Request) {
	date := r.PathValue("date")
	if _, err := time.Parse("2006-01-02", date); err != nil {
		writeError(w, http.StatusBadRequest, "日期格式无效", err)
		return
	}
	var input struct {
		IsWorkday bool   `json:"is_workday"`
		Note      string `json:"note"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	user := currentUser(r)
	value := domain.WorkdayOverride{Date: date, IsWorkday: input.IsWorkday, Note: strings.TrimSpace(input.Note), UpdatedBy: user.ID, UpdatedAt: time.Now()}
	if err := s.repo.UpsertWorkdayOverride(r.Context(), value); err != nil {
		writeError(w, http.StatusBadRequest, "保存工作日失败", err)
		return
	}
	if err := s.reminders.RecalculateWorkdayReminders(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "重新计算工作日提醒失败", err)
		return
	}
	s.auditWorkCalendar(r, user, "work_calendar.update", date)
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) deleteWorkCalendar(w http.ResponseWriter, r *http.Request) {
	date := r.PathValue("date")
	if _, err := time.Parse("2006-01-02", date); err != nil {
		writeError(w, http.StatusBadRequest, "日期格式无效", err)
		return
	}
	if err := s.repo.DeleteWorkdayOverride(r.Context(), date); err != nil {
		writeError(w, http.StatusBadRequest, "删除工作日覆盖失败", err)
		return
	}
	if err := s.reminders.RecalculateWorkdayReminders(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "重新计算工作日提醒失败", err)
		return
	}
	s.auditWorkCalendar(r, currentUser(r), "work_calendar.delete", date)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) importWorkCalendar(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(2 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "CSV 文件无效", err)
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "缺少 CSV 文件", err)
		return
	}
	defer file.Close()
	reader := csv.NewReader(io.LimitReader(file, 2<<20))
	reader.TrimLeadingSpace = true
	user := currentUser(r)
	count := 0
	for rowNumber := 1; ; rowNumber++ {
		row, readErr := reader.Read()
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("CSV 第 %d 行无法读取", rowNumber), readErr)
			return
		}
		if rowNumber == 1 && len(row) > 0 && strings.EqualFold(strings.TrimSpace(row[0]), "date") {
			continue
		}
		if len(row) < 2 {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("CSV 第 %d 行字段不足", rowNumber), nil)
			return
		}
		date := strings.TrimSpace(row[0])
		if _, parseErr := time.Parse("2006-01-02", date); parseErr != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("CSV 第 %d 行日期无效", rowNumber), parseErr)
			return
		}
		isWorkday, parseErr := strconv.ParseBool(strings.TrimSpace(row[1]))
		if parseErr != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("CSV 第 %d 行 is_workday 必须为 true 或 false", rowNumber), parseErr)
			return
		}
		note := ""
		if len(row) > 2 {
			note = strings.TrimSpace(row[2])
		}
		value := domain.WorkdayOverride{Date: date, IsWorkday: isWorkday, Note: note, UpdatedBy: user.ID, UpdatedAt: time.Now()}
		if saveErr := s.repo.UpsertWorkdayOverride(r.Context(), value); saveErr != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("CSV 第 %d 行保存失败", rowNumber), saveErr)
			return
		}
		count++
	}
	if err := s.reminders.RecalculateWorkdayReminders(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "重新计算工作日提醒失败", err)
		return
	}
	s.auditWorkCalendar(r, user, "work_calendar.import", strconv.Itoa(count))
	writeJSON(w, http.StatusOK, map[string]int{"imported": count})
}

func (s *Server) auditWorkCalendar(r *http.Request, user domain.User, action, id string) {
	_ = s.repo.AppendAudit(r.Context(), domain.AuditEvent{ID: ids.New("aud"), ActorID: user.ID, ActorName: user.Name, Action: action, ResourceType: "work_calendar", ResourceID: id, Metadata: map[string]any{}, CreatedAt: time.Now()})
}

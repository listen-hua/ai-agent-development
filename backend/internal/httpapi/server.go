package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"sort"
	"strings"
	"time"

	"internal-ai-agent/backend/internal/config"
	"internal-ai-agent/backend/internal/domain"
	"internal-ai-agent/backend/internal/ids"
	"internal-ai-agent/backend/internal/integration/feishu"
	"internal-ai-agent/backend/internal/security"
	"internal-ai-agent/backend/internal/service"
	"internal-ai-agent/backend/internal/store"
)

type Server struct {
	cfg           config.Config
	repo          store.Repository
	sessions      *security.Sessions
	chat          *service.Chat
	knowledge     *service.Knowledge
	notifications *service.Notification
	reminders     *service.Reminder
	meetings      *service.Meeting
	directory     *service.Directory
	agents        *service.AgentRegistry
	imageAgent    *service.ImageAgent
	feishu        *feishu.Client
	mux           *http.ServeMux
}
type contextKey string

const userKey contextKey = "user"

func New(cfg config.Config, repo store.Repository, sessions *security.Sessions, chat *service.Chat, knowledge *service.Knowledge, notifications *service.Notification, reminders *service.Reminder, feishuClient *feishu.Client, directory *service.Directory, agents *service.AgentRegistry, features ...any) *Server {
	s := &Server{cfg: cfg, repo: repo, sessions: sessions, chat: chat, knowledge: knowledge, notifications: notifications, reminders: reminders, directory: directory, agents: agents, feishu: feishuClient, mux: http.NewServeMux()}
	for _, feature := range features {
		switch value := feature.(type) {
		case *service.Meeting:
			s.meetings = value
		case *service.ImageAgent:
			s.imageAgent = value
		}
	}
	s.routes()
	return s
}
func (s *Server) Handler() http.Handler { return s.recoverer(s.securityHeaders(s.logRequests(s.mux))) }

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	s.mux.HandleFunc("GET /api/v1/auth/feishu/config", s.feishuAuthConfig)
	s.mux.HandleFunc("POST /api/v1/auth/feishu/client-diagnostics", s.feishuClientDiagnostics)
	s.mux.HandleFunc("POST /api/v1/auth/feishu/exchange", s.exchange)
	s.mux.Handle("GET /api/v1/me", s.auth(http.HandlerFunc(s.me)))
	s.mux.Handle("GET /api/v1/agents", s.auth(http.HandlerFunc(s.listAvailableAgents)))
	s.mux.Handle("POST /api/v1/auth/logout", s.auth(http.HandlerFunc(s.logout)))
	s.mux.Handle("GET /api/v1/conversations", s.auth(http.HandlerFunc(s.listConversations)))
	s.mux.Handle("POST /api/v1/conversations", s.auth(http.HandlerFunc(s.createConversation)))
	s.mux.Handle("GET /api/v1/conversations/{id}/messages", s.auth(http.HandlerFunc(s.listMessages)))
	s.mux.Handle("DELETE /api/v1/conversations/{id}", s.auth(http.HandlerFunc(s.deleteConversation)))
	s.mux.Handle("POST /api/v1/conversations/{id}/context/reset", s.auth(http.HandlerFunc(s.resetConversationContext)))
	s.mux.Handle("POST /api/v1/conversations/{id}/messages", s.auth(http.HandlerFunc(s.startMessage)))
	s.mux.Handle("GET /api/v1/runs/{id}/events", s.auth(http.HandlerFunc(s.runEvents)))
	s.mux.Handle("GET /api/v1/reminders", s.auth(http.HandlerFunc(s.listReminders)))
	s.mux.Handle("GET /api/v1/reminders/{id}", s.auth(http.HandlerFunc(s.getReminder)))
	s.mux.Handle("GET /api/v1/reminders/{id}/deliveries", s.auth(http.HandlerFunc(s.listReminderDeliveries)))
	s.mux.Handle("POST /api/v1/reminder-actions", s.auth(http.HandlerFunc(s.createNewReminderAction)))
	s.mux.Handle("POST /api/v1/reminders/{id}/actions", s.auth(http.HandlerFunc(s.createReminderAction)))
	s.mux.Handle("POST /api/v1/reminder-actions/{id}/confirm", s.auth(http.HandlerFunc(s.confirmReminderAction)))
	s.mux.Handle("POST /api/v1/reminder-actions/{id}/cancel", s.auth(http.HandlerFunc(s.cancelReminderAction)))
	if s.meetings != nil {
		s.mux.Handle("POST /api/v1/meeting-booking-actions/{id}/confirm", s.auth(http.HandlerFunc(s.confirmMeetingBookingAction)))
		s.mux.Handle("POST /api/v1/meeting-booking-actions/{id}/cancel", s.auth(http.HandlerFunc(s.cancelMeetingBookingAction)))
		s.mux.Handle("GET /api/v1/meeting-bookings", s.auth(http.HandlerFunc(s.listMeetingBookings)))
	}
	if s.imageAgent != nil {
		s.mux.Handle("GET /api/v1/image-agent/options", s.auth(http.HandlerFunc(s.imageAgentOptions)))
		s.mux.Handle("GET /api/v1/image-agent/projects/{id}/canvas", s.auth(http.HandlerFunc(s.imageCanvas)))
		s.mux.Handle("PATCH /api/v1/image-agent/projects/{id}/canvas", s.auth(http.HandlerFunc(s.updateImageCanvas)))
		s.mux.Handle("POST /api/v1/image-agent/assets", s.auth(http.HandlerFunc(s.uploadImageAsset)))
		s.mux.Handle("GET /api/v1/image-agent/assets/{id}/content", s.auth(http.HandlerFunc(s.imageAssetContent)))
		s.mux.Handle("POST /api/v1/image-agent/jobs", s.auth(http.HandlerFunc(s.createImageJob)))
		s.mux.Handle("GET /api/v1/image-agent/jobs", s.auth(http.HandlerFunc(s.listImageJobs)))
		s.mux.Handle("GET /api/v1/image-agent/jobs/{id}", s.auth(http.HandlerFunc(s.getImageJob)))
	}

	knowledgeAdmin := s.roles(domain.RoleKnowledgeAdmin)
	directoryAdmin := s.roles(domain.RoleKnowledgeAdmin, domain.RoleImageAdmin)
	notificationAdmin := s.roles(domain.RoleNotificationAdmin)
	auditor := s.roles(domain.RoleAuditor, domain.RoleKnowledgeAdmin, domain.RoleNotificationAdmin)
	superAdmin := s.roles(domain.RoleSuperAdmin)
	s.mux.Handle("GET /api/v1/admin/knowledge/sources", s.auth(knowledgeAdmin(http.HandlerFunc(s.listSources))))
	s.mux.Handle("POST /api/v1/admin/knowledge/sources", s.auth(knowledgeAdmin(http.HandlerFunc(s.createSource))))
	s.mux.Handle("POST /api/v1/admin/knowledge/sources/{id}/sync", s.auth(knowledgeAdmin(http.HandlerFunc(s.syncSource))))
	s.mux.Handle("GET /api/v1/admin/knowledge/documents", s.auth(knowledgeAdmin(http.HandlerFunc(s.listDocuments))))
	s.mux.Handle("POST /api/v1/admin/knowledge/documents/upload", s.auth(knowledgeAdmin(http.HandlerFunc(s.uploadDocument))))
	s.mux.Handle("POST /api/v1/admin/knowledge/documents/{id}/publish", s.auth(knowledgeAdmin(http.HandlerFunc(s.publishDocument))))
	s.mux.Handle("PUT /api/v1/admin/knowledge/documents/{id}/acl", s.auth(knowledgeAdmin(http.HandlerFunc(s.updateDocumentACL))))
	s.mux.Handle("GET /api/v1/admin/directory/options", s.auth(directoryAdmin(http.HandlerFunc(s.directoryOptions))))
	s.mux.Handle("GET /api/v1/admin/agent/configs", s.auth(knowledgeAdmin(http.HandlerFunc(s.listConfigs))))
	s.mux.Handle("POST /api/v1/admin/agent/configs", s.auth(knowledgeAdmin(http.HandlerFunc(s.createConfig))))
	s.mux.Handle("POST /api/v1/admin/agent/configs/{id}/publish", s.auth(knowledgeAdmin(http.HandlerFunc(s.publishConfig))))
	s.mux.Handle("GET /api/v1/admin/agents", s.auth(knowledgeAdmin(http.HandlerFunc(s.listAgentProfiles))))
	s.mux.Handle("POST /api/v1/admin/agents", s.auth(knowledgeAdmin(http.HandlerFunc(s.createAgentProfile))))
	s.mux.Handle("PUT /api/v1/admin/agents/{id}", s.auth(knowledgeAdmin(http.HandlerFunc(s.updateAgentProfile))))
	s.mux.Handle("GET /api/v1/admin/notifications", s.auth(notificationAdmin(http.HandlerFunc(s.listNotifications))))
	s.mux.Handle("POST /api/v1/admin/notifications", s.auth(notificationAdmin(http.HandlerFunc(s.createNotification))))
	s.mux.Handle("GET /api/v1/admin/notifications/targets", s.auth(notificationAdmin(http.HandlerFunc(s.notificationTargets))))
	s.mux.Handle("POST /api/v1/admin/notifications/images", s.auth(notificationAdmin(http.HandlerFunc(s.uploadNotificationImage))))
	s.mux.Handle("POST /api/v1/admin/notifications/ai-draft", s.auth(notificationAdmin(http.HandlerFunc(s.aiDraftNotification))))
	s.mux.Handle("POST /api/v1/admin/notifications/{id}/approve", s.auth(notificationAdmin(http.HandlerFunc(s.approveNotification))))
	s.mux.Handle("POST /api/v1/admin/notifications/{id}/send", s.auth(notificationAdmin(http.HandlerFunc(s.sendNotification))))
	s.mux.Handle("POST /api/v1/admin/notifications/{id}/cancel", s.auth(notificationAdmin(http.HandlerFunc(s.cancelNotification))))
	s.mux.Handle("GET /api/v1/admin/work-calendar", s.auth(notificationAdmin(http.HandlerFunc(s.listWorkCalendar))))
	s.mux.Handle("PUT /api/v1/admin/work-calendar/{date}", s.auth(notificationAdmin(http.HandlerFunc(s.upsertWorkCalendar))))
	s.mux.Handle("DELETE /api/v1/admin/work-calendar/{date}", s.auth(notificationAdmin(http.HandlerFunc(s.deleteWorkCalendar))))
	s.mux.Handle("POST /api/v1/admin/work-calendar/import", s.auth(notificationAdmin(http.HandlerFunc(s.importWorkCalendar))))
	s.mux.Handle("GET /api/v1/admin/audit", s.auth(auditor(http.HandlerFunc(s.listAudit))))
	s.mux.Handle("GET /api/v1/admin/metrics", s.auth(auditor(http.HandlerFunc(s.metrics))))
	s.mux.Handle("GET /api/v1/admin/users", s.auth(superAdmin(http.HandlerFunc(s.listUsers))))
	s.mux.Handle("PUT /api/v1/admin/users/{id}/roles", s.auth(superAdmin(http.HandlerFunc(s.updateUserRoles))))
	s.mux.Handle("POST /api/v1/admin/users/sync", s.auth(superAdmin(http.HandlerFunc(s.syncUsers))))
	if s.meetings != nil {
		s.mux.Handle("GET /api/v1/admin/meeting-rooms", s.auth(notificationAdmin(http.HandlerFunc(s.listMeetingRooms))))
		s.mux.Handle("POST /api/v1/admin/meeting-rooms/sync", s.auth(notificationAdmin(http.HandlerFunc(s.syncMeetingRooms))))
		s.mux.Handle("POST /api/v1/admin/meeting-rooms/calendar", s.auth(notificationAdmin(http.HandlerFunc(s.initializeMeetingCalendar))))
	}
	if s.imageAgent != nil {
		imageAdmin := s.roles(domain.RoleImageAdmin)
		s.mux.Handle("GET /api/v1/admin/image-agent/relays", s.auth(imageAdmin(http.HandlerFunc(s.listImageRelays))))
		s.mux.Handle("POST /api/v1/admin/image-agent/relays", s.auth(imageAdmin(http.HandlerFunc(s.createImageRelay))))
		s.mux.Handle("PUT /api/v1/admin/image-agent/relays/{id}", s.auth(imageAdmin(http.HandlerFunc(s.updateImageRelay))))
		s.mux.Handle("POST /api/v1/admin/image-agent/relays/{id}/test", s.auth(imageAdmin(http.HandlerFunc(s.testImageRelay))))
		s.mux.Handle("POST /api/v1/admin/image-agent/relays/{id}/models/sync", s.auth(imageAdmin(http.HandlerFunc(s.syncImageModels))))
		s.mux.Handle("GET /api/v1/admin/image-agent/models", s.auth(imageAdmin(http.HandlerFunc(s.listImageModels))))
		s.mux.Handle("PUT /api/v1/admin/image-agent/models/{id}", s.auth(imageAdmin(http.HandlerFunc(s.updateImageModel))))
		s.mux.Handle("GET /api/v1/admin/image-agent/projects", s.auth(imageAdmin(http.HandlerFunc(s.listImageProjects))))
		s.mux.Handle("POST /api/v1/admin/image-agent/projects", s.auth(imageAdmin(http.HandlerFunc(s.createImageProject))))
		s.mux.Handle("PUT /api/v1/admin/image-agent/projects/{id}", s.auth(imageAdmin(http.HandlerFunc(s.updateImageProject))))
		s.mux.Handle("GET /api/v1/admin/image-agent/prompt-actions", s.auth(imageAdmin(http.HandlerFunc(s.listImagePromptActions))))
		s.mux.Handle("POST /api/v1/admin/image-agent/prompt-actions", s.auth(imageAdmin(http.HandlerFunc(s.createImagePromptAction))))
		s.mux.Handle("PUT /api/v1/admin/image-agent/prompt-actions/{id}", s.auth(imageAdmin(http.HandlerFunc(s.updateImagePromptAction))))
		s.mux.Handle("DELETE /api/v1/admin/image-agent/prompt-actions/{id}", s.auth(imageAdmin(http.HandlerFunc(s.deleteImagePromptAction))))
	}
}

func (s *Server) feishuAuthConfig(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{
		"app_id":  s.cfg.FeishuAppID,
		"enabled": s.feishu.Configured(),
	})
}

func (s *Server) feishuClientDiagnostics(w http.ResponseWriter, r *http.Request) {
	userAgent := r.UserAgent()
	lowerUserAgent := strings.ToLower(userAgent)
	if !strings.Contains(lowerUserAgent, "lark") && !strings.Contains(lowerUserAgent, "feishu") {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	var input struct {
		Stage           string `json:"stage"`
		Errno           string `json:"errno"`
		Message         string `json:"message"`
		H5SDK           bool   `json:"h5sdk"`
		RequestAccess   bool   `json:"request_access"`
		RequestAuthCode bool   `json:"request_auth_code"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	slog.Warn("feishu client auth failed",
		"stage", truncateDiagnostic(input.Stage, 64),
		"errno", truncateDiagnostic(input.Errno, 32),
		"message", truncateDiagnostic(input.Message, 256),
		"h5sdk", input.H5SDK,
		"request_access", input.RequestAccess,
		"request_auth_code", input.RequestAuthCode,
		"user_agent", truncateDiagnostic(userAgent, 512),
	)
	w.WriteHeader(http.StatusNoContent)
}

func truncateDiagnostic(value string, limit int) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}

func (s *Server) exchange(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Code       string `json:"code"`
		AuthMethod string `json:"auth_method"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Code = strings.TrimSpace(input.Code)
	if input.Code == "" {
		writeError(w, http.StatusBadRequest, "缺少飞书授权码", nil)
		return
	}
	var user domain.User
	var err error
	if s.cfg.DevAuthEnabled && strings.HasPrefix(input.Code, "dev:") {
		if input.Code == "dev:employee" {
			user, err = s.repo.GetUser(r.Context(), store.DemoEmployeeID)
		} else {
			user, err = s.repo.GetUser(r.Context(), store.DemoAdminID)
		}
	} else {
		var info feishu.UserInfo
		switch input.AuthMethod {
		case "", "request_access":
			info, err = s.feishu.ExchangeCode(r.Context(), input.Code)
		case "request_auth_code":
			info, err = s.feishu.ExchangeLegacyCode(r.Context(), input.Code)
		default:
			writeError(w, http.StatusBadRequest, "不支持的飞书授权方式", nil)
			return
		}
		if err == nil {
			roles := []domain.Role{domain.RoleEmployee}
			if containsString(s.cfg.BootstrapSuperAdminOpenIDs, info.OpenID) {
				roles = append(roles, domain.RoleSuperAdmin)
			}
			candidate := domain.User{FeishuOpenID: info.OpenID, Name: info.Name, AvatarURL: info.AvatarURL, Status: "active", Roles: roles}
			if s.directory != nil {
				if enriched, enrichErr := s.directory.Enrich(r.Context(), candidate); enrichErr == nil {
					candidate = enriched
				} else {
					slog.Warn("feishu contact profile unavailable; continuing with basic identity", "error", enrichErr)
				}
			}
			user, err = s.repo.UpsertUser(r.Context(), candidate)
		}
	}
	if err != nil {
		writeError(w, http.StatusUnauthorized, "登录失败", err)
		return
	}
	if user.Status != "" && user.Status != "active" {
		writeError(w, http.StatusForbidden, "账号已停用，请联系管理员", nil)
		return
	}
	token, err := s.sessions.Issue(user.ID, 12*time.Hour)
	if err != nil {
		writeError(w, 500, "创建会话失败", err)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "ai_agent_session", Value: token, Path: "/", HttpOnly: true, Secure: s.cfg.Environment == "production", SameSite: http.SameSiteLaxMode, MaxAge: 43200})
	_ = s.repo.AppendAudit(r.Context(), domain.AuditEvent{ID: ids.New("aud"), ActorID: user.ID, ActorName: user.Name, Action: "auth.login", ResourceType: "session", ResourceID: user.ID, Metadata: map[string]any{}, CreatedAt: time.Now()})
	writeJSON(w, http.StatusOK, user)
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
func (s *Server) me(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, currentUser(r)) }
func (s *Server) listAvailableAgents(w http.ResponseWriter, r *http.Request) {
	values, err := s.agents.List(r.Context(), false)
	respond(w, values, err)
}
func (s *Server) logout(w http.ResponseWriter, _ *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: "ai_agent_session", Value: "", Path: "/", HttpOnly: true, MaxAge: -1})
	w.WriteHeader(http.StatusNoContent)
}
func (s *Server) listConversations(w http.ResponseWriter, r *http.Request) {
	values, err := s.repo.ListConversations(r.Context(), currentUser(r).ID)
	respond(w, values, err)
}
func (s *Server) createConversation(w http.ResponseWriter, r *http.Request) {
	value, err := s.chat.CreateConversation(r.Context(), currentUser(r))
	if err != nil {
		writeError(w, 500, "创建会话失败", err)
		return
	}
	writeJSON(w, 201, value)
}
func (s *Server) listMessages(w http.ResponseWriter, r *http.Request) {
	conv, err := s.repo.GetConversation(r.Context(), r.PathValue("id"))
	if err != nil || conv.UserID != currentUser(r).ID {
		writeError(w, 404, "会话不存在", err)
		return
	}
	values, err := s.repo.ListMessages(r.Context(), conv.ID)
	respond(w, values, err)
}
func (s *Server) deleteConversation(w http.ResponseWriter, r *http.Request) {
	err := s.repo.DeleteConversation(r.Context(), r.PathValue("id"), currentUser(r).ID)
	if err != nil {
		writeError(w, 400, "删除会话失败", err)
		return
	}
	w.WriteHeader(204)
}
func (s *Server) resetConversationContext(w http.ResponseWriter, r *http.Request) {
	err := s.chat.ResetContext(r.Context(), currentUser(r), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "会话不存在", err)
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "清除上下文失败", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (s *Server) startMessage(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Content string `json:"content"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	runID, err := s.chat.Start(r.Context(), currentUser(r), r.PathValue("id"), input.Content)
	if err != nil {
		writeError(w, 400, "发送失败", err)
		return
	}
	writeJSON(w, 202, map[string]string{"run_id": runID})
}
func (s *Server) runEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, 500, "不支持流式响应", nil)
		return
	}
	history, ch, found := s.chat.Subscribe(r.PathValue("id"))
	if !found {
		writeError(w, 404, "运行不存在", nil)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	send := func(event domain.RunEvent) {
		body, _ := json.Marshal(event)
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, body)
		flusher.Flush()
	}
	for _, event := range history {
		send(event)
	}
	if ch == nil {
		return
	}
	for {
		select {
		case event, open := <-ch:
			if !open {
				return
			}
			send(event)
		case <-r.Context().Done():
			return
		case <-time.After(25 * time.Second):
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		}
	}
}

func (s *Server) listSources(w http.ResponseWriter, r *http.Request) {
	v, e := s.repo.ListSources(r.Context())
	respond(w, v, e)
}
func (s *Server) createSource(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name        string     `json:"name"`
		Type        string     `json:"type"`
		RemoteToken string     `json:"remote_token"`
		DefaultACL  domain.ACL `json:"default_acl"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := input.DefaultACL.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, "可见范围无效", err)
		return
	}
	v, e := s.knowledge.CreateSource(r.Context(), currentUser(r), input.Name, input.Type, input.RemoteToken, input.DefaultACL)
	if e != nil {
		slog.Warn("create knowledge source rejected", "source_type", input.Type, "error", e)
		writeError(w, 400, "创建资料源失败", e)
		return
	}
	writeJSON(w, 201, v)
}
func (s *Server) syncSource(w http.ResponseWriter, r *http.Request) {
	e := s.knowledge.Sync(r.Context(), currentUser(r), r.PathValue("id"))
	if e != nil {
		writeError(w, 400, "同步失败", e)
		return
	}
	writeJSON(w, 202, map[string]string{"status": "queued"})
}
func (s *Server) listDocuments(w http.ResponseWriter, r *http.Request) {
	v, e := s.repo.ListDocuments(r.Context())
	respond(w, v, e)
}
func (s *Server) uploadDocument(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, 400, "文件不能超过 32MB", err)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, 400, "缺少文件", err)
		return
	}
	defer file.Close()
	data, err := readMultipart(file, 32<<20)
	if err != nil {
		writeError(w, 400, "读取文件失败", err)
		return
	}
	acl := domain.ACL{}
	if raw := r.FormValue("acl"); raw != "" {
		if err = json.Unmarshal([]byte(raw), &acl); err != nil {
			writeError(w, 400, "可见范围格式无效", err)
			return
		}
	} else {
		acl = domain.ACL{Scope: r.FormValue("acl_scope")}
		if acl.Scope == "" {
			acl.Scope = "all"
		}
	}
	if err = acl.Validate(); err != nil {
		writeError(w, 400, "可见范围无效", err)
		return
	}
	mime := header.Header.Get("Content-Type")
	v, e := s.knowledge.Ingest(r.Context(), currentUser(r), header.Filename, mime, data, acl)
	if e != nil {
		writeError(w, 422, "文档解析失败", e)
		return
	}
	writeJSON(w, 201, v)
}
func (s *Server) publishDocument(w http.ResponseWriter, r *http.Request) {
	v, e := s.knowledge.Publish(r.Context(), currentUser(r), r.PathValue("id"))
	if e != nil {
		writeError(w, 400, "发布失败", e)
		return
	}
	writeJSON(w, 200, v)
}
func (s *Server) updateDocumentACL(w http.ResponseWriter, r *http.Request) {
	var acl domain.ACL
	if !decodeJSON(w, r, &acl) {
		return
	}
	if err := acl.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, "可见范围无效", err)
		return
	}
	value, err := s.knowledge.UpdateACL(r.Context(), currentUser(r), r.PathValue("id"), acl)
	if err != nil {
		writeError(w, http.StatusBadRequest, "更新可见范围失败", err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) directoryOptions(w http.ResponseWriter, r *http.Request) {
	users, err := s.repo.ListUsers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取组织信息失败", err)
		return
	}
	type userOption struct {
		ID            string   `json:"id"`
		Name          string   `json:"name"`
		DepartmentIDs []string `json:"department_ids"`
		JobTitle      string   `json:"job_title"`
	}
	if s.directory == nil || s.feishu == nil || !s.feishu.Configured() {
		writeError(w, http.StatusServiceUnavailable, "飞书通讯录未配置，无法读取公司组织架构", nil)
		return
	}
	departments, err := s.directory.Departments(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, "读取飞书公司组织架构失败，请检查应用通讯录权限及可见范围", err)
		return
	}
	if len(departments) == 0 {
		writeError(w, http.StatusBadGateway, "飞书未返回任何可见部门，请在开放平台配置部门读取权限，并将应用通讯录可见范围覆盖公司组织架构", nil)
		return
	}
	jobTitles := map[string]bool{}
	userOptions := []userOption{}
	for _, user := range users {
		if user.Status != "active" {
			continue
		}
		if user.JobTitle != "" {
			jobTitles[user.JobTitle] = true
		}
		userOptions = append(userOptions, userOption{ID: user.ID, Name: user.Name, DepartmentIDs: user.DepartmentIDs, JobTitle: user.JobTitle})
	}
	departmentOptions := buildDirectoryDepartmentOptions(departments)
	titles := make([]string, 0, len(jobTitles))
	for title := range jobTitles {
		titles = append(titles, title)
	}
	sort.Strings(titles)
	sort.Slice(userOptions, func(i, j int) bool { return userOptions[i].Name < userOptions[j].Name })
	writeJSON(w, http.StatusOK, map[string]any{"departments": departmentOptions, "job_titles": titles, "users": userOptions})
}

type directoryDepartmentOption struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	ParentID string `json:"parent_id,omitempty"`
	Path     string `json:"path"`
	Depth    int    `json:"depth"`
}

func buildDirectoryDepartmentOptions(departments []feishu.ContactDepartment) []directoryDepartmentOption {
	byID := make(map[string]feishu.ContactDepartment, len(departments))
	for _, department := range departments {
		if department.OpenDepartmentID != "" {
			byID[department.OpenDepartmentID] = department
		}
	}
	pathCache := make(map[string]string, len(byID))
	depthCache := make(map[string]int, len(byID))
	var resolvePath func(string, map[string]bool) (string, int)
	resolvePath = func(id string, visiting map[string]bool) (string, int) {
		if path, ok := pathCache[id]; ok {
			return path, depthCache[id]
		}
		department, ok := byID[id]
		if !ok {
			return "", 0
		}
		name := strings.TrimSpace(department.Name)
		if name == "" {
			name = department.OpenDepartmentID
		}
		if visiting[id] {
			return name, 0
		}
		visiting[id] = true
		path, depth := name, 0
		if parentID := department.ParentDepartmentID; parentID != "" && parentID != "0" {
			if parentPath, parentDepth := resolvePath(parentID, visiting); parentPath != "" {
				path = parentPath + " / " + name
				depth = parentDepth + 1
			}
		}
		delete(visiting, id)
		pathCache[id], depthCache[id] = path, depth
		return path, depth
	}
	options := make([]directoryDepartmentOption, 0, len(byID))
	for id, department := range byID {
		path, depth := resolvePath(id, map[string]bool{})
		options = append(options, directoryDepartmentOption{
			ID:       id,
			Name:     strings.TrimSpace(department.Name),
			ParentID: department.ParentDepartmentID,
			Path:     path,
			Depth:    depth,
		})
	}
	sort.Slice(options, func(i, j int) bool {
		if options[i].Path == options[j].Path {
			return options[i].ID < options[j].ID
		}
		return options[i].Path < options[j].Path
	})
	return options
}
func (s *Server) listConfigs(w http.ResponseWriter, r *http.Request) {
	v, e := s.repo.ListConfigs(r.Context())
	respond(w, v, e)
}
func (s *Server) createConfig(w http.ResponseWriter, r *http.Request) {
	var input domain.AgentConfig
	if !decodeJSON(w, r, &input) {
		return
	}
	configs, _ := s.repo.ListConfigs(r.Context())
	max := 0
	for _, v := range configs {
		if v.Version > max {
			max = v.Version
		}
	}
	value := domain.AgentConfigVersion{ID: ids.New("cfg"), Version: max + 1, Status: "draft", Config: input, CreatedBy: currentUser(r).ID, CreatedAt: time.Now()}
	if err := s.repo.SaveConfig(r.Context(), value); err != nil {
		writeError(w, 400, "保存配置失败", err)
		return
	}
	writeJSON(w, 201, value)
}
func (s *Server) publishConfig(w http.ResponseWriter, r *http.Request) {
	v, e := s.repo.PublishConfig(r.Context(), r.PathValue("id"))
	if e != nil {
		writeError(w, 400, "发布配置失败", e)
		return
	}
	writeJSON(w, 200, v)
}
func (s *Server) listAgentProfiles(w http.ResponseWriter, r *http.Request) {
	values, err := s.agents.List(r.Context(), true)
	respond(w, values, err)
}
func (s *Server) createAgentProfile(w http.ResponseWriter, r *http.Request) {
	s.saveAgentProfile(w, r, "")
}
func (s *Server) updateAgentProfile(w http.ResponseWriter, r *http.Request) {
	s.saveAgentProfile(w, r, r.PathValue("id"))
}
func (s *Server) saveAgentProfile(w http.ResponseWriter, r *http.Request, id string) {
	var input struct {
		AgentKey    string         `json:"agent_key"`
		Name        string         `json:"name"`
		Description string         `json:"description"`
		Kind        string         `json:"kind"`
		Provider    string         `json:"provider"`
		Model       string         `json:"model"`
		APIKey      string         `json:"api_key"`
		Enabled     bool           `json:"enabled"`
		Settings    map[string]any `json:"settings"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	value, err := s.agents.Save(r.Context(), currentUser(r), id, service.AgentProfileInput{AgentKey: input.AgentKey, Name: input.Name, Description: input.Description, Kind: input.Kind, Provider: input.Provider, Model: input.Model, APIKey: input.APIKey, Enabled: input.Enabled, Settings: input.Settings})
	if err != nil {
		writeError(w, http.StatusBadRequest, "保存 Agent 配置失败", err)
		return
	}
	status := http.StatusOK
	if id == "" {
		status = http.StatusCreated
	}
	writeJSON(w, status, value)
}
func (s *Server) listNotifications(w http.ResponseWriter, r *http.Request) {
	v, e := s.repo.ListNotifications(r.Context())
	respond(w, v, e)
}
func (s *Server) createNotification(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Title         string                     `json:"title"`
		Content       string                     `json:"content"`
		Audience      domain.ACL                 `json:"audience"`
		RecipientType string                     `json:"recipient_type"`
		RecipientIDs  []string                   `json:"recipient_ids"`
		Images        []domain.NotificationImage `json:"images"`
		ScheduledAt   *time.Time                 `json:"scheduled_at"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	var v domain.NotificationDraft
	var e error
	if input.RecipientType == "" {
		v, e = s.notifications.Create(r.Context(), currentUser(r), input.Title, input.Content, input.Audience, input.ScheduledAt)
	} else {
		v, e = s.notifications.CreateTargeted(r.Context(), currentUser(r), input.Title, input.Content, input.RecipientType, input.RecipientIDs, input.Images, input.ScheduledAt)
	}
	if e != nil {
		writeError(w, 400, "创建通知失败", e)
		return
	}
	writeJSON(w, 201, v)
}
func (s *Server) notificationTargets(w http.ResponseWriter, r *http.Request) {
	users, chats, chatErr := s.notifications.Targets(r.Context())
	response := map[string]any{"users": users, "chats": chats}
	if chatErr != nil {
		slog.Warn("feishu notification chat targets unavailable", "error", chatErr)
		response["chat_error"] = "应用无法读取群聊，请开通 im:chat:readonly 权限并确认机器人已加入目标群聊"
	}
	writeJSON(w, http.StatusOK, response)
}
func (s *Server) uploadNotificationImage(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 11*1024*1024)
	if err := r.ParseMultipartForm(11 * 1024 * 1024); err != nil {
		writeError(w, http.StatusBadRequest, "图片不能超过 10 MB", err)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "请选择图片", err)
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 10*1024*1024+1))
	if err != nil || len(data) > 10*1024*1024 {
		writeError(w, http.StatusBadRequest, "读取图片失败或图片超过 10 MB", err)
		return
	}
	contentType := http.DetectContentType(data)
	allowed := map[string]bool{"image/jpeg": true, "image/png": true, "image/gif": true, "image/webp": true, "image/bmp": true, "image/tiff": true, "image/x-icon": true, "image/vnd.microsoft.icon": true}
	if !allowed[contentType] {
		writeError(w, http.StatusBadRequest, "仅支持 JPG、PNG、GIF、WEBP、BMP、TIFF 或 ICO 图片", nil)
		return
	}
	image, err := s.notifications.UploadImage(r.Context(), header.Filename, data)
	if err != nil {
		writeError(w, http.StatusBadGateway, "上传图片到飞书失败", err)
		return
	}
	writeJSON(w, http.StatusCreated, image)
}
func (s *Server) aiDraftNotification(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Brief string `json:"brief"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	content, err := s.notifications.DraftWithAI(r.Context(), input.Brief)
	if err != nil {
		writeError(w, http.StatusBadGateway, "AI 起草失败", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"content": content})
}
func (s *Server) approveNotification(w http.ResponseWriter, r *http.Request) {
	v, e := s.notifications.Approve(r.Context(), currentUser(r), r.PathValue("id"))
	respond(w, v, e)
}
func (s *Server) sendNotification(w http.ResponseWriter, r *http.Request) {
	v, e := s.notifications.Send(r.Context(), currentUser(r), r.PathValue("id"))
	respond(w, v, e)
}
func (s *Server) cancelNotification(w http.ResponseWriter, r *http.Request) {
	v, e := s.notifications.Cancel(r.Context(), currentUser(r), r.PathValue("id"))
	respond(w, v, e)
}
func (s *Server) listAudit(w http.ResponseWriter, r *http.Request) {
	v, e := s.repo.ListAudit(r.Context(), 200)
	respond(w, v, e)
}
func (s *Server) metrics(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.repo.Metrics(r.Context()))
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.repo.ListUsers(r.Context())
	respond(w, users, err)
}
func (s *Server) updateUserRoles(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Roles []domain.Role `json:"roles"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	roles, err := domain.NormalizeRoles(input.Roles)
	if err != nil {
		writeError(w, http.StatusBadRequest, "角色配置无效", err)
		return
	}
	value, err := s.repo.UpdateUserRoles(r.Context(), r.PathValue("id"), roles)
	if errors.Is(err, store.ErrConflict) {
		writeError(w, http.StatusConflict, "必须至少保留一名超级管理员", err)
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "更新角色失败", err)
		return
	}
	actor := currentUser(r)
	_ = s.repo.AppendAudit(r.Context(), domain.AuditEvent{ID: ids.New("aud"), ActorID: actor.ID, ActorName: actor.Name, Action: "identity.roles.update", ResourceType: "user", ResourceID: value.ID, Metadata: map[string]any{"roles": value.Roles}, CreatedAt: time.Now()})
	writeJSON(w, http.StatusOK, value)
}
func (s *Server) syncUsers(w http.ResponseWriter, r *http.Request) {
	if s.directory == nil || s.feishu == nil || !s.feishu.Configured() {
		writeError(w, http.StatusServiceUnavailable, "飞书通讯录同步未配置", nil)
		return
	}
	succeeded, failed, err := s.directory.SyncAllUsers(r.Context(), currentUser(r))
	if err != nil {
		writeError(w, http.StatusBadGateway, "同步飞书通讯录失败", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"succeeded": succeeded, "failed": failed})
}

func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("ai_agent_session")
		if err != nil {
			writeError(w, 401, "请先登录", nil)
			return
		}
		claims, err := s.sessions.Verify(cookie.Value)
		if err != nil {
			writeError(w, 401, "登录已过期", err)
			return
		}
		user, err := s.repo.GetUser(r.Context(), claims.UserID)
		if err != nil {
			writeError(w, 401, "用户不存在", err)
			return
		}
		if user.Status != "" && user.Status != "active" {
			writeError(w, http.StatusForbidden, "账号已停用", nil)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey, user)))
	})
}
func (s *Server) roles(roles ...domain.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !currentUser(r).HasRole(roles...) {
				writeError(w, 403, "没有访问权限", nil)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
func currentUser(r *http.Request) domain.User {
	value, _ := r.Context().Value(userKey).(domain.User)
	return value
}
func readMultipart(file multipart.File, limit int64) ([]byte, error) {
	reader := http.MaxBytesReader(nil, file, limit)
	defer reader.Close()
	return ioReadAll(reader)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(w, 400, "请求格式错误", err)
		return false
	}
	return true
}
func respond(w http.ResponseWriter, value any, err error) {
	if err != nil {
		status := 500
		if errors.Is(err, store.ErrNotFound) {
			status = 404
		}
		if errors.Is(err, store.ErrForbidden) {
			status = 403
		}
		writeError(w, status, "请求失败", err)
		return
	}
	writeJSON(w, 200, value)
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, status int, message string, err error) {
	payload := map[string]any{"error": message}
	if err != nil {
		payload["detail"] = err.Error()
	}
	writeJSON(w, status, payload)
}
func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}
func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		slog.Info("request", "method", r.Method, "path", r.URL.Path, "duration", time.Since(start))
	})
}
func (s *Server) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if value := recover(); value != nil {
				slog.Error("panic", "value", value)
				writeError(w, 500, "服务内部错误", nil)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

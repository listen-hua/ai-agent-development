package store

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"internal-ai-agent/backend/internal/domain"
	"internal-ai-agent/backend/internal/ids"
)

type Memory struct {
	mu                     sync.RWMutex
	users                  map[string]domain.User
	openIDs                map[string]string
	conversations          map[string]domain.Conversation
	messages               map[string][]domain.Message
	messageFeedback        map[string]bool
	conversationContexts   map[string]domain.ConversationContext
	conversationBindings   map[string]domain.ConversationBinding
	sources                map[string]domain.KnowledgeSource
	documents              map[string]domain.Document
	chunks                 []domain.Chunk
	configs                []domain.AgentConfigVersion
	agentProfiles          map[string]domain.AgentProfile
	notifications          map[string]domain.NotificationDraft
	notificationDeliveries map[string]string
	reminders              map[string]domain.Reminder
	reminderActions        map[string]domain.ReminderActionDraft
	reminderDeliveries     map[string]domain.ReminderDelivery
	workdayOverrides       map[string]domain.WorkdayOverride
	reminderBotJobs        map[string]domain.ReminderBotJob
	meetingRooms           map[string]domain.MeetingRoom
	meetingSettings        domain.MeetingSettings
	meetingActions         map[string]domain.MeetingBookingAction
	meetingBookings        map[string]domain.MeetingBooking
	meetingDeliveries      map[string]domain.MeetingBookingDelivery
	imageRelays            map[string]domain.ImageRelay
	imageModels            map[string]domain.ImageModel
	imageProjects          map[string]domain.ImageProject
	imagePromptActions     map[string]domain.ImagePromptAction
	imageCanvases          map[string]domain.ImageCanvas
	imageAssets            map[string]domain.ImageAsset
	imageJobs              map[string]domain.ImageJob
	audits                 []domain.AuditEvent
	events                 map[string]struct{}
}

const DemoAdminID = "00000000-0000-4000-8000-000000000001"
const DemoEmployeeID = "00000000-0000-4000-8000-000000000002"

func NewMemory(defaultConfig domain.AgentConfig) *Memory {
	now := time.Now()
	admin := domain.User{ID: DemoAdminID, FeishuOpenID: "ou_demo_admin", Name: "演示管理员", DepartmentIDs: []string{"dept_admin"}, JobTitle: "系统管理员", Status: "active", Roles: []domain.Role{domain.RoleEmployee, domain.RoleSuperAdmin}}
	employee := domain.User{ID: DemoEmployeeID, FeishuOpenID: "ou_demo_employee", Name: "演示员工", DepartmentIDs: []string{"dept_product"}, JobTitle: "产品经理", Status: "active", Roles: []domain.Role{domain.RoleEmployee}}
	config := domain.AgentConfigVersion{ID: ids.New("cfg"), Version: 1, Status: "published", Config: defaultConfig, CreatedBy: admin.ID, CreatedAt: now, PublishedAt: &now}
	return &Memory{
		users: map[string]domain.User{admin.ID: admin, employee.ID: employee}, openIDs: map[string]string{admin.FeishuOpenID: admin.ID, employee.FeishuOpenID: employee.ID},
		conversations: map[string]domain.Conversation{}, messages: map[string][]domain.Message{}, messageFeedback: map[string]bool{}, conversationContexts: map[string]domain.ConversationContext{}, conversationBindings: map[string]domain.ConversationBinding{}, sources: map[string]domain.KnowledgeSource{}, documents: map[string]domain.Document{},
		configs: []domain.AgentConfigVersion{config}, agentProfiles: map[string]domain.AgentProfile{}, notifications: map[string]domain.NotificationDraft{}, notificationDeliveries: map[string]string{},
		reminders: map[string]domain.Reminder{}, reminderActions: map[string]domain.ReminderActionDraft{}, reminderDeliveries: map[string]domain.ReminderDelivery{},
		workdayOverrides: map[string]domain.WorkdayOverride{}, reminderBotJobs: map[string]domain.ReminderBotJob{},
		meetingRooms: map[string]domain.MeetingRoom{}, meetingSettings: domain.MeetingSettings{Timezone: "Asia/Shanghai", WorkdayStart: "09:00", WorkdayEnd: "18:00", SlotMinutes: 30, SyncIntervalMinute: 15, UpdatedAt: now},
		meetingActions: map[string]domain.MeetingBookingAction{}, meetingBookings: map[string]domain.MeetingBooking{}, meetingDeliveries: map[string]domain.MeetingBookingDelivery{},
		imageRelays: map[string]domain.ImageRelay{}, imageModels: map[string]domain.ImageModel{}, imageProjects: map[string]domain.ImageProject{},
		imagePromptActions: map[string]domain.ImagePromptAction{}, imageCanvases: map[string]domain.ImageCanvas{},
		imageAssets: map[string]domain.ImageAsset{}, imageJobs: map[string]domain.ImageJob{},
		audits: []domain.AuditEvent{}, events: map[string]struct{}{},
	}
}

func (m *Memory) UpsertUser(_ context.Context, user domain.User) (domain.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if id, ok := m.openIDs[user.FeishuOpenID]; ok {
		existing := m.users[id]
		existing.Name = user.Name
		existing.AvatarURL = user.AvatarURL
		if user.OrganizationSyncedAt != nil {
			existing.DepartmentIDs = user.DepartmentIDs
			existing.JobTitle = user.JobTitle
			existing.JobLevelID = user.JobLevelID
			existing.JobFamilyID = user.JobFamilyID
			existing.EmployeeType = user.EmployeeType
			existing.Status = user.Status
			existing.OrganizationSyncedAt = user.OrganizationSyncedAt
		}
		for _, role := range user.Roles {
			if !hasRole(existing.Roles, role) {
				existing.Roles = append(existing.Roles, role)
			}
		}
		user = existing
	}
	if user.ID == "" {
		user.ID = ids.New("usr")
	}
	if len(user.Roles) == 0 {
		user.Roles = []domain.Role{domain.RoleEmployee}
	}
	if user.Status == "" {
		user.Status = "active"
	}
	m.users[user.ID] = user
	m.openIDs[user.FeishuOpenID] = user.ID
	return user, nil
}

func hasRole(roles []domain.Role, expected domain.Role) bool {
	for _, role := range roles {
		if role == expected {
			return true
		}
	}
	return false
}
func (m *Memory) GetUser(_ context.Context, id string) (domain.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	value, ok := m.users[id]
	if !ok {
		return domain.User{}, ErrNotFound
	}
	return value, nil
}
func (m *Memory) GetUserByOpenID(_ context.Context, openID string) (domain.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	id, ok := m.openIDs[openID]
	if !ok {
		return domain.User{}, ErrNotFound
	}
	return m.users[id], nil
}
func (m *Memory) ListUsers(_ context.Context) ([]domain.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]domain.User, 0, len(m.users))
	for _, value := range m.users {
		out = append(out, value)
	}
	return out, nil
}
func (m *Memory) UpdateUserRoles(_ context.Context, userID string, roles []domain.Role) (domain.User, error) {
	normalized, err := domain.NormalizeRoles(roles)
	if err != nil {
		return domain.User{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	user, ok := m.users[userID]
	if !ok {
		return domain.User{}, ErrNotFound
	}
	if hasRole(user.Roles, domain.RoleSuperAdmin) && !hasRole(normalized, domain.RoleSuperAdmin) {
		count := 0
		for _, candidate := range m.users {
			if hasRole(candidate.Roles, domain.RoleSuperAdmin) {
				count++
			}
		}
		if count <= 1 {
			return domain.User{}, ErrConflict
		}
	}
	user.Roles = normalized
	m.users[userID] = user
	return user, nil
}
func (m *Memory) UpdateUserStatus(_ context.Context, openID, status string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	id, ok := m.openIDs[openID]
	if !ok {
		return ErrNotFound
	}
	user := m.users[id]
	user.Status = status
	m.users[id] = user
	return nil
}

func (m *Memory) UpsertMeetingRooms(_ context.Context, rooms []domain.MeetingRoom) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	seen := make(map[string]bool, len(rooms))
	for _, room := range rooms {
		seen[room.RoomID] = true
		m.meetingRooms[room.RoomID] = room
	}
	for id, room := range m.meetingRooms {
		if !seen[id] {
			room.Enabled = false
			room.ScheduleEnabled = false
			room.LastError = "会议室已从飞书同步范围移除"
			m.meetingRooms[id] = room
		}
	}
	return nil
}

func (m *Memory) ListMeetingRooms(_ context.Context) ([]domain.MeetingRoom, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	values := make([]domain.MeetingRoom, 0, len(m.meetingRooms))
	for _, room := range m.meetingRooms {
		values = append(values, room)
	}
	sort.Slice(values, func(i, j int) bool {
		if values[i].Capacity == values[j].Capacity {
			return values[i].Name < values[j].Name
		}
		return values[i].Capacity < values[j].Capacity
	})
	return values, nil
}

func (m *Memory) GetMeetingRoom(_ context.Context, roomID string) (domain.MeetingRoom, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	value, ok := m.meetingRooms[roomID]
	if !ok {
		return value, ErrNotFound
	}
	return value, nil
}

func (m *Memory) GetMeetingSettings(_ context.Context) (domain.MeetingSettings, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.meetingSettings, nil
}

func (m *Memory) SaveMeetingSettings(_ context.Context, value domain.MeetingSettings) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.meetingSettings = value
	return nil
}

func (m *Memory) CreateMeetingBookingAction(_ context.Context, value domain.MeetingBookingAction) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.meetingActions[value.ID]; exists {
		return ErrConflict
	}
	m.meetingActions[value.ID] = value
	return nil
}

func (m *Memory) GetMeetingBookingAction(_ context.Context, id string) (domain.MeetingBookingAction, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	value, ok := m.meetingActions[id]
	if !ok {
		return value, ErrNotFound
	}
	return value, nil
}

func (m *Memory) ClaimMeetingBookingAction(_ context.Context, id, userID, optionID string, now time.Time) (domain.MeetingBookingAction, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	value, ok := m.meetingActions[id]
	if !ok {
		return value, ErrNotFound
	}
	if value.UserID != userID {
		return value, ErrForbidden
	}
	if value.Status != domain.MeetingActionPending || now.After(value.ExpiresAt) {
		if value.Status == domain.MeetingActionPending {
			value.Status = domain.MeetingActionExpired
			m.meetingActions[id] = value
		}
		return value, ErrConflict
	}
	value.Status = domain.MeetingActionProcessing
	value.SelectedOptionID = optionID
	m.meetingActions[id] = value
	return value, nil
}

func (m *Memory) UpdateMeetingBookingAction(_ context.Context, value domain.MeetingBookingAction) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.meetingActions[value.ID]; !exists {
		return ErrNotFound
	}
	m.meetingActions[value.ID] = value
	return nil
}

func (m *Memory) CreateMeetingBooking(_ context.Context, value domain.MeetingBooking) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.meetingBookings[value.ID]; exists {
		return ErrConflict
	}
	m.meetingBookings[value.ID] = value
	return nil
}

func (m *Memory) GetMeetingBooking(_ context.Context, id string) (domain.MeetingBooking, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	value, ok := m.meetingBookings[id]
	if !ok {
		return value, ErrNotFound
	}
	return value, nil
}

func (m *Memory) ListMeetingBookings(_ context.Context, userID string) ([]domain.MeetingBooking, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	values := []domain.MeetingBooking{}
	for _, booking := range m.meetingBookings {
		if userID == "" || booking.UserID == userID {
			values = append(values, booking)
		}
	}
	sort.Slice(values, func(i, j int) bool { return values[i].StartAt.After(values[j].StartAt) })
	return values, nil
}

func (m *Memory) UpdateMeetingBooking(_ context.Context, value domain.MeetingBooking) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.meetingBookings[value.ID]; !exists {
		return ErrNotFound
	}
	m.meetingBookings[value.ID] = value
	for id, delivery := range m.meetingDeliveries {
		if delivery.BookingID == value.ID && value.Status != "active" && delivery.Status != "sent" {
			delivery.Status = "cancelled"
			delivery.UpdatedAt = value.UpdatedAt
			m.meetingDeliveries[id] = delivery
		}
	}
	return nil
}

func (m *Memory) CreateMeetingBookingDelivery(_ context.Context, value domain.MeetingBookingDelivery) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, existing := range m.meetingDeliveries {
		if existing.BookingID == value.BookingID && existing.ScheduledFor.Equal(value.ScheduledFor) {
			return nil
		}
	}
	m.meetingDeliveries[value.ID] = value
	return nil
}

func (m *Memory) ClaimMeetingBookingDeliveries(_ context.Context, now time.Time, lease time.Duration, limit int) ([]domain.MeetingBookingDelivery, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if limit <= 0 {
		limit = 50
	}
	values := []domain.MeetingBookingDelivery{}
	for id, delivery := range m.meetingDeliveries {
		if len(values) >= limit || (delivery.Status != "pending" && delivery.Status != "retry" && delivery.Status != "sending") || delivery.NextAttemptAt.After(now) || delivery.LockedUntil != nil && delivery.LockedUntil.After(now) {
			continue
		}
		lockedUntil := now.Add(lease)
		delivery.Status = "sending"
		delivery.Attempts++
		delivery.LockedUntil = &lockedUntil
		delivery.UpdatedAt = now
		m.meetingDeliveries[id] = delivery
		values = append(values, delivery)
	}
	return values, nil
}

func (m *Memory) UpdateMeetingBookingDelivery(_ context.Context, value domain.MeetingBookingDelivery) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.meetingDeliveries[value.ID]; !exists {
		return ErrNotFound
	}
	m.meetingDeliveries[value.ID] = value
	return nil
}
func (m *Memory) CreateConversation(_ context.Context, value domain.Conversation) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	normalizeConversation(&value)
	m.conversations[value.ID] = value
	return nil
}
func (m *Memory) ListConversations(_ context.Context, userID string) ([]domain.Conversation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []domain.Conversation{}
	for _, value := range m.conversations {
		if value.UserID == userID {
			out = append(out, value)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out, nil
}
func (m *Memory) GetConversation(_ context.Context, id string) (domain.Conversation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	value, ok := m.conversations[id]
	if !ok {
		return domain.Conversation{}, ErrNotFound
	}
	return value, nil
}
func (m *Memory) DeleteConversation(_ context.Context, id, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	value, ok := m.conversations[id]
	if !ok {
		return ErrNotFound
	}
	if value.UserID != userID {
		return ErrForbidden
	}
	delete(m.conversations, id)
	delete(m.messages, id)
	delete(m.conversationContexts, id)
	for key, binding := range m.conversationBindings {
		if binding.ConversationID == id {
			delete(m.conversationBindings, key)
		}
	}
	return nil
}
func (m *Memory) AddMessage(_ context.Context, value domain.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages[value.ConversationID] = append(m.messages[value.ConversationID], value)
	if conv, ok := m.conversations[value.ConversationID]; ok {
		conv.UpdatedAt = value.CreatedAt
		if conv.Title == "新会话" && value.Role == "user" {
			conv.Title = truncate(value.Content, 24)
		}
		m.conversations[conv.ID] = conv
	}
	return nil
}
func (m *Memory) ListMessages(_ context.Context, conversationID string) ([]domain.Message, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]domain.Message{}, m.messages[conversationID]...), nil
}

func (m *Memory) UpdateMessageAnalysis(_ context.Context, messageID, intent, standalone string, version, promptTokens, completionTokens int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for conversationID, values := range m.messages {
		for index := range values {
			if values[index].ID != messageID {
				continue
			}
			values[index].Intent = intent
			values[index].StandaloneQuery = standalone
			values[index].ContextVersion = version
			if promptTokens >= 0 {
				values[index].PromptTokens = promptTokens
			}
			if completionTokens >= 0 {
				values[index].CompletionTokens = completionTokens
			}
			m.messages[conversationID] = values
			return nil
		}
	}
	return ErrNotFound
}

func (m *Memory) GetConversationContext(_ context.Context, conversationID string) (domain.ConversationContext, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	value, ok := m.conversationContexts[conversationID]
	if !ok {
		return domain.ConversationContext{}, ErrNotFound
	}
	return value, nil
}

func (m *Memory) SaveConversationContext(_ context.Context, value domain.ConversationContext) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.conversations[value.ConversationID]; !ok {
		return ErrNotFound
	}
	m.conversationContexts[value.ConversationID] = value
	return nil
}

func (m *Memory) ResetConversationContext(_ context.Context, conversationID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.conversations[conversationID]; !ok {
		return ErrNotFound
	}
	version := 0
	if current, ok := m.conversationContexts[conversationID]; ok {
		version = current.Version + 1
	}
	resetThrough := ""
	if messages := m.messages[conversationID]; len(messages) > 0 {
		resetThrough = messages[len(messages)-1].ID
	}
	m.conversationContexts[conversationID] = domain.ConversationContext{
		ConversationID:        conversationID,
		ResetThroughMessageID: resetThrough,
		ActiveTask:            domain.ConversationTaskState{Slots: map[string]any{}},
		Version:               version,
		UpdatedAt:             time.Now(),
	}
	return nil
}

func (m *Memory) GetOrCreateBoundConversation(_ context.Context, user domain.User, agentKey, channel, scopeID string, now time.Time, ttl time.Duration) (domain.Conversation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := conversationBindingKey(user.ID, agentKey, channel, scopeID)
	if binding, ok := m.conversationBindings[key]; ok && binding.ExpiresAt.After(now) {
		if conversation, exists := m.conversations[binding.ConversationID]; exists {
			binding.ExpiresAt = now.Add(ttl)
			binding.UpdatedAt = now
			m.conversationBindings[key] = binding
			return conversation, nil
		}
	}
	conversation := domain.Conversation{ID: ids.New("conv"), UserID: user.ID, Title: "飞书行政助手", AgentKey: agentKey, Channel: channel, CreatedAt: now, UpdatedAt: now}
	normalizeConversation(&conversation)
	m.conversations[conversation.ID] = conversation
	m.conversationBindings[key] = domain.ConversationBinding{UserID: user.ID, AgentKey: agentKey, Channel: channel, ExternalScopeID: scopeID, ConversationID: conversation.ID, ExpiresAt: now.Add(ttl), UpdatedAt: now}
	return conversation, nil
}

func (m *Memory) ResetConversationBinding(_ context.Context, userID, agentKey, channel, scopeID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.conversationBindings, conversationBindingKey(userID, agentKey, channel, scopeID))
	return nil
}

func (m *Memory) CleanupConversationHistory(_ context.Context, before time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for conversationID, messages := range m.messages {
		for index := range messages {
			if messages[index].CreatedAt.Before(before) {
				messages[index].Content = "[内容已按留存策略清理]"
				messages[index].Citations = []domain.Citation{}
				messages[index].StandaloneQuery = ""
			}
		}
		m.messages[conversationID] = messages
	}
	for conversationID, value := range m.conversationContexts {
		if value.UpdatedAt.Before(before) {
			delete(m.conversationContexts, conversationID)
		}
	}
	for key, binding := range m.conversationBindings {
		if binding.UpdatedAt.Before(before) {
			delete(m.conversationBindings, key)
		}
	}
	return nil
}

func normalizeConversation(value *domain.Conversation) {
	if value.AgentKey == "" {
		value.AgentKey = domain.AgentAdministrativeAssistant
	}
	if value.Channel == "" {
		value.Channel = domain.ConversationChannelH5
	}
}

func conversationBindingKey(userID, agentKey, channel, scopeID string) string {
	return strings.Join([]string{userID, agentKey, channel, scopeID}, "\x00")
}
func (m *Memory) CreateSource(_ context.Context, value domain.KnowledgeSource) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sources[value.ID] = value
	return nil
}
func (m *Memory) GetSource(_ context.Context, id string) (domain.KnowledgeSource, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	value, ok := m.sources[id]
	if !ok {
		return domain.KnowledgeSource{}, ErrNotFound
	}
	return value, nil
}
func (m *Memory) ListSources(_ context.Context) ([]domain.KnowledgeSource, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []domain.KnowledgeSource{}
	for _, value := range m.sources {
		out = append(out, value)
	}
	return out, nil
}
func (m *Memory) UpdateSource(_ context.Context, value domain.KnowledgeSource) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.sources[value.ID]; !ok {
		return ErrNotFound
	}
	m.sources[value.ID] = value
	return nil
}
func (m *Memory) CreateDocument(_ context.Context, value domain.Document, chunks []domain.Chunk) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.documents[value.ID] = value
	m.chunks = append(m.chunks, chunks...)
	return nil
}
func (m *Memory) CreateDocumentVersion(_ context.Context, value domain.Document, version domain.DocumentVersion, chunks []domain.Chunk) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	current, ok := m.documents[value.ID]
	if !ok {
		return ErrNotFound
	}
	value.Versions = append([]domain.DocumentVersion{version}, current.Versions...)
	m.documents[value.ID] = value
	m.chunks = append(m.chunks, chunks...)
	return nil
}
func (m *Memory) UpdateDocument(_ context.Context, value domain.Document) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.documents[value.ID]; !ok {
		return ErrNotFound
	}
	m.documents[value.ID] = value
	return nil
}
func (m *Memory) GetDocument(_ context.Context, id string) (domain.Document, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	value, ok := m.documents[id]
	if !ok {
		return domain.Document{}, ErrNotFound
	}
	return value, nil
}
func (m *Memory) GetDocumentByRemoteToken(_ context.Context, sourceID, remoteToken string) (domain.Document, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, value := range m.documents {
		if value.SourceID == sourceID && value.RemoteToken == remoteToken {
			return value, nil
		}
	}
	return domain.Document{}, ErrNotFound
}
func (m *Memory) ListDocuments(_ context.Context) ([]domain.Document, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []domain.Document{}
	for _, value := range m.documents {
		out = append(out, value)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out, nil
}
func (m *Memory) ListDocumentsBySource(_ context.Context, sourceID string) ([]domain.Document, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []domain.Document{}
	for _, value := range m.documents {
		if value.SourceID == sourceID {
			out = append(out, value)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out, nil
}

func (m *Memory) SearchChunks(_ context.Context, user domain.User, query string, _ []float32, limit int) ([]domain.Chunk, map[string]domain.Document, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	type scored struct {
		chunk domain.Chunk
		score int
	}
	terms := tokenize(query)
	candidates := []scored{}
	docs := map[string]domain.Document{}
	for _, chunk := range m.chunks {
		doc := m.documents[chunk.DocumentID]
		if doc.Status != "published" || !doc.ACL.Allows(user) {
			continue
		}
		versionPublished := false
		for _, version := range doc.Versions {
			if version.ID == chunk.VersionID && version.Status == "published" {
				versionPublished = true
				break
			}
		}
		if !versionPublished {
			continue
		}
		score := 0
		lower := strings.ToLower(chunk.Content + " " + chunk.Heading + " " + doc.Title)
		for _, term := range terms {
			if strings.Contains(lower, term) {
				score += 2
			}
			score += strings.Count(lower, term)
		}
		if score > 0 {
			candidates = append(candidates, scored{chunk: chunk, score: score})
			docs[doc.ID] = doc
		}
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].score > candidates[j].score })
	if limit <= 0 {
		limit = 8
	}
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	out := make([]domain.Chunk, len(candidates))
	for i, item := range candidates {
		out[i] = item.chunk
	}
	return out, docs, nil
}

func (m *Memory) ListConfigs(_ context.Context) ([]domain.AgentConfigVersion, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := append([]domain.AgentConfigVersion(nil), m.configs...)
	sort.Slice(out, func(i, j int) bool { return out[i].Version > out[j].Version })
	return out, nil
}
func (m *Memory) SaveConfig(_ context.Context, value domain.AgentConfigVersion) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.configs = append(m.configs, value)
	return nil
}
func (m *Memory) PublishConfig(_ context.Context, id string) (domain.AgentConfigVersion, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for i := range m.configs {
		if m.configs[i].Status == "published" {
			m.configs[i].Status = "archived"
		}
		if m.configs[i].ID == id {
			m.configs[i].Status = "published"
			m.configs[i].PublishedAt = &now
		}
	}
	for _, v := range m.configs {
		if v.ID == id {
			return v, nil
		}
	}
	return domain.AgentConfigVersion{}, ErrNotFound
}
func (m *Memory) PublishedConfig(_ context.Context) (domain.AgentConfigVersion, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, v := range m.configs {
		if v.Status == "published" {
			return v, nil
		}
	}
	return domain.AgentConfigVersion{}, ErrNotFound
}
func (m *Memory) ListAgentProfiles(_ context.Context) ([]domain.AgentProfile, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	values := make([]domain.AgentProfile, 0, len(m.agentProfiles))
	for _, value := range m.agentProfiles {
		values = append(values, value)
	}
	sort.Slice(values, func(i, j int) bool { return values[i].CreatedAt.Before(values[j].CreatedAt) })
	return values, nil
}
func (m *Memory) GetAgentProfile(_ context.Context, id string) (domain.AgentProfile, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	value, ok := m.agentProfiles[id]
	if !ok {
		return value, ErrNotFound
	}
	return value, nil
}
func (m *Memory) GetAgentProfileByKey(_ context.Context, key string) (domain.AgentProfile, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, value := range m.agentProfiles {
		if value.AgentKey == key {
			return value, nil
		}
	}
	return domain.AgentProfile{}, ErrNotFound
}
func (m *Memory) UpsertAgentProfile(_ context.Context, value domain.AgentProfile) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, existing := range m.agentProfiles {
		if existing.AgentKey == value.AgentKey && id != value.ID {
			return ErrConflict
		}
	}
	m.agentProfiles[value.ID] = value
	return nil
}
func (m *Memory) CreateNotification(_ context.Context, value domain.NotificationDraft) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notifications[value.ID] = value
	return nil
}
func (m *Memory) UpdateNotification(_ context.Context, value domain.NotificationDraft) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.notifications[value.ID]; !ok {
		return ErrNotFound
	}
	m.notifications[value.ID] = value
	return nil
}
func (m *Memory) GetNotification(_ context.Context, id string) (domain.NotificationDraft, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.notifications[id]
	if !ok {
		return domain.NotificationDraft{}, ErrNotFound
	}
	return v, nil
}
func (m *Memory) ListNotifications(_ context.Context) ([]domain.NotificationDraft, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []domain.NotificationDraft{}
	for _, v := range m.notifications {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out, nil
}
func (m *Memory) ClaimDueNotifications(_ context.Context, now time.Time, limit int) ([]domain.NotificationDraft, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if limit <= 0 {
		limit = 20
	}
	out := make([]domain.NotificationDraft, 0, limit)
	for id, value := range m.notifications {
		dueScheduled := value.Status == "scheduled" && value.ScheduledAt != nil && !value.ScheduledAt.After(now)
		staleSending := value.Status == "sending" && !value.UpdatedAt.After(now.Add(-10*time.Minute))
		if len(out) >= limit || (!dueScheduled && !staleSending) {
			continue
		}
		value.Status = "sending"
		value.UpdatedAt = now
		m.notifications[id] = value
		out = append(out, value)
	}
	return out, nil
}
func (m *Memory) RecordNotificationDelivery(_ context.Context, notificationID, receiverID, _ string, _ string, status, _ string, _ string, _ int, _ time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notificationDeliveries[notificationID+"\x00"+receiverID] = status
	return nil
}
func (m *Memory) CreateReminderAction(_ context.Context, value domain.ReminderActionDraft) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.reminderActions[value.ID]; exists {
		return ErrConflict
	}
	m.reminderActions[value.ID] = value
	return nil
}

func (m *Memory) CountReminderActionsSince(_ context.Context, userID string, since time.Time) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	count := 0
	for _, value := range m.reminderActions {
		if value.UserID == userID && !value.CreatedAt.Before(since) {
			count++
		}
	}
	return count, nil
}
func (m *Memory) GetReminderAction(_ context.Context, id string) (domain.ReminderActionDraft, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	value, ok := m.reminderActions[id]
	if !ok {
		return domain.ReminderActionDraft{}, ErrNotFound
	}
	return value, nil
}
func (m *Memory) LatestPendingReminderAction(_ context.Context, userID, conversationID string) (domain.ReminderActionDraft, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var found domain.ReminderActionDraft
	for _, value := range m.reminderActions {
		if value.UserID != userID || value.Status != "pending" || (conversationID != "" && value.SourceConversationID != conversationID) {
			continue
		}
		if found.ID == "" || value.CreatedAt.After(found.CreatedAt) {
			found = value
		}
	}
	if found.ID == "" {
		return found, ErrNotFound
	}
	return found, nil
}
func (m *Memory) ConfirmReminderAction(_ context.Context, id, userID string, nextFireAt *time.Time, now time.Time) (domain.ReminderActionDraft, domain.Reminder, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	action, ok := m.reminderActions[id]
	if !ok {
		return action, domain.Reminder{}, ErrNotFound
	}
	if action.UserID != userID {
		return action, domain.Reminder{}, ErrForbidden
	}
	if action.Status == "confirmed" && action.ResultReminderID != "" {
		return action, m.reminders[action.ResultReminderID], nil
	}
	if action.Status != "pending" || now.After(action.ExpiresAt) {
		if action.Status == "pending" {
			action.Status = "expired"
			m.reminderActions[id] = action
		}
		return action, domain.Reminder{}, ErrConflict
	}
	var reminder domain.Reminder
	switch action.Action {
	case "create":
		if action.Schedule == nil {
			return action, reminder, ErrConflict
		}
		reminder = domain.Reminder{ID: ids.New("rem"), UserID: userID, Content: action.Content, Status: "active", Schedule: *action.Schedule, NextFireAt: nextFireAt, SourceChannel: action.SourceChannel, SourceConversationID: action.SourceConversationID, Version: 1, CreatedAt: now, UpdatedAt: now}
	case "update", "pause", "resume", "delete":
		reminder, ok = m.reminders[action.ReminderID]
		if !ok {
			return action, reminder, ErrNotFound
		}
		if reminder.UserID != userID {
			return action, reminder, ErrForbidden
		}
		if action.Action == "update" {
			if action.Content != "" {
				reminder.Content = action.Content
			}
			if action.Schedule != nil {
				reminder.Schedule = *action.Schedule
			}
			reminder.NextFireAt = nextFireAt
			reminder.Status = "active"
		} else if action.Action == "pause" {
			reminder.Status = "paused"
			reminder.NextFireAt = nil
		} else if action.Action == "resume" {
			reminder.Status = "active"
			reminder.NextFireAt = nextFireAt
		} else {
			reminder.Status = "cancelled"
			reminder.NextFireAt = nil
		}
		reminder.Version++
		reminder.UpdatedAt = now
	default:
		return action, reminder, ErrConflict
	}
	m.reminders[reminder.ID] = reminder
	confirmed := now
	action.Status = "confirmed"
	action.ResultReminderID = reminder.ID
	action.ConfirmedAt = &confirmed
	m.reminderActions[id] = action
	return action, reminder, nil
}
func (m *Memory) CancelReminderAction(_ context.Context, id, userID string, now time.Time) (domain.ReminderActionDraft, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	action, ok := m.reminderActions[id]
	if !ok {
		return action, ErrNotFound
	}
	if action.UserID != userID {
		return action, ErrForbidden
	}
	if action.Status == "cancelled" {
		return action, nil
	}
	if action.Status != "pending" {
		return action, ErrConflict
	}
	action.Status = "cancelled"
	action.ConfirmedAt = &now
	m.reminderActions[id] = action
	return action, nil
}
func (m *Memory) ListReminders(_ context.Context, userID string) ([]domain.Reminder, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []domain.Reminder{}
	for _, value := range m.reminders {
		if value.UserID == userID {
			out = append(out, value)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out, nil
}
func (m *Memory) GetReminder(_ context.Context, id string) (domain.Reminder, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	value, ok := m.reminders[id]
	if !ok {
		return value, ErrNotFound
	}
	return value, nil
}
func (m *Memory) ListReminderDeliveries(_ context.Context, reminderID string, limit int) ([]domain.ReminderDelivery, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []domain.ReminderDelivery{}
	for _, value := range m.reminderDeliveries {
		if value.ReminderID == reminderID {
			out = append(out, value)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ScheduledFor.After(out[j].ScheduledFor) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
func (m *Memory) ListDueReminders(_ context.Context, now time.Time, limit int) ([]domain.Reminder, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []domain.Reminder{}
	for _, value := range m.reminders {
		if value.Status == "active" && value.NextFireAt != nil && !value.NextFireAt.After(now) {
			out = append(out, value)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].NextFireAt.Before(*out[j].NextFireAt) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (m *Memory) ListActiveWorkdayReminders(_ context.Context) ([]domain.Reminder, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	values := make([]domain.Reminder, 0)
	for _, value := range m.reminders {
		if value.Status == "active" && value.Schedule.Type == domain.ReminderWorkday {
			values = append(values, value)
		}
	}
	return values, nil
}
func (m *Memory) CreateReminderDeliveryAndAdvance(_ context.Context, reminderID string, scheduledFor time.Time, nextFireAt *time.Time, now time.Time) (domain.ReminderDelivery, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	reminder, ok := m.reminders[reminderID]
	if !ok {
		return domain.ReminderDelivery{}, false, ErrNotFound
	}
	if reminder.Status != "active" || reminder.NextFireAt == nil || !reminder.NextFireAt.Equal(scheduledFor) {
		return domain.ReminderDelivery{}, false, nil
	}
	for _, value := range m.reminderDeliveries {
		if value.ReminderID == reminderID && value.ScheduledFor.Equal(scheduledFor) {
			return value, false, nil
		}
	}
	delivery := domain.ReminderDelivery{ID: ids.New("del"), ReminderID: reminderID, ScheduledFor: scheduledFor, Status: "pending", NextAttemptAt: now, IdempotencyKey: ids.New("idem"), CreatedAt: now, UpdatedAt: now}
	m.reminderDeliveries[delivery.ID] = delivery
	reminder.NextFireAt = nextFireAt
	reminder.UpdatedAt = now
	m.reminders[reminderID] = reminder
	return delivery, true, nil
}
func (m *Memory) ClaimReminderDeliveries(_ context.Context, now time.Time, lease time.Duration, limit int) ([]domain.ReminderDelivery, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []domain.ReminderDelivery{}
	for id, value := range m.reminderDeliveries {
		claimable := (value.Status == "pending" || value.Status == "retry" || value.Status == "sending") && !value.NextAttemptAt.After(now) && (value.LockedUntil == nil || value.LockedUntil.Before(now))
		if !claimable {
			continue
		}
		locked := now.Add(lease)
		value.Status = "sending"
		value.LockedUntil = &locked
		value.Attempts++
		value.UpdatedAt = now
		m.reminderDeliveries[id] = value
		out = append(out, value)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}
func (m *Memory) UpdateReminderDelivery(_ context.Context, value domain.ReminderDelivery) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.reminderDeliveries[value.ID]; !ok {
		return ErrNotFound
	}
	m.reminderDeliveries[value.ID] = value
	return nil
}
func (m *Memory) UpdateReminderRuntime(_ context.Context, id, status string, lastFiredAt *time.Time, lastError string, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	value, ok := m.reminders[id]
	if !ok {
		return ErrNotFound
	}
	if status != "" {
		value.Status = status
	}
	if lastFiredAt != nil {
		value.LastFiredAt = lastFiredAt
	}
	value.LastError = lastError
	value.UpdatedAt = now
	m.reminders[id] = value
	return nil
}

func (m *Memory) UpdateReminderNextFire(_ context.Context, id string, nextFireAt *time.Time, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	value, ok := m.reminders[id]
	if !ok {
		return ErrNotFound
	}
	value.NextFireAt = nextFireAt
	value.UpdatedAt = now
	m.reminders[id] = value
	return nil
}

func (m *Memory) CleanupReminderHistory(_ context.Context, before time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, value := range m.reminders {
		if (value.Status == "completed" || value.Status == "cancelled" || value.Status == "failed") && value.UpdatedAt.Before(before) {
			delete(m.reminders, id)
			for deliveryID, delivery := range m.reminderDeliveries {
				if delivery.ReminderID == id {
					delete(m.reminderDeliveries, deliveryID)
				}
			}
		}
	}
	for id, value := range m.reminderActions {
		if value.Status != "pending" && value.CreatedAt.Before(before) {
			delete(m.reminderActions, id)
		}
	}
	return nil
}
func (m *Memory) ListWorkdayOverrides(_ context.Context, year int) ([]domain.WorkdayOverride, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	prefix := fmt.Sprintf("%04d-", year)
	out := []domain.WorkdayOverride{}
	for key, value := range m.workdayOverrides {
		if strings.HasPrefix(key, prefix) {
			out = append(out, value)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date < out[j].Date })
	return out, nil
}
func (m *Memory) GetWorkdayOverride(_ context.Context, date string) (domain.WorkdayOverride, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	value, ok := m.workdayOverrides[date]
	if !ok {
		return value, ErrNotFound
	}
	return value, nil
}
func (m *Memory) UpsertWorkdayOverride(_ context.Context, value domain.WorkdayOverride) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.workdayOverrides[value.Date] = value
	return nil
}
func (m *Memory) DeleteWorkdayOverride(_ context.Context, date string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.workdayOverrides, date)
	return nil
}
func (m *Memory) CreateReminderBotJob(_ context.Context, value domain.ReminderBotJob) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, existing := range m.reminderBotJobs {
		if existing.EventID == value.EventID {
			return false, nil
		}
	}
	m.reminderBotJobs[value.ID] = value
	return true, nil
}
func (m *Memory) ClaimReminderBotJobs(_ context.Context, now time.Time, lease time.Duration, limit int) ([]domain.ReminderBotJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []domain.ReminderBotJob{}
	for id, value := range m.reminderBotJobs {
		if (value.Status != "pending" && value.Status != "retry" && value.Status != "processing") || value.AvailableAt.After(now) {
			continue
		}
		value.Status = "processing"
		value.Attempts++
		value.AvailableAt = now.Add(lease)
		value.UpdatedAt = now
		m.reminderBotJobs[id] = value
		out = append(out, value)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}
func (m *Memory) UpdateReminderBotJob(_ context.Context, value domain.ReminderBotJob) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.reminderBotJobs[value.ID]; !ok {
		return ErrNotFound
	}
	m.reminderBotJobs[value.ID] = value
	return nil
}
func (m *Memory) AppendAudit(_ context.Context, value domain.AuditEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.audits = append(m.audits, value)
	return nil
}
func (m *Memory) ListAudit(_ context.Context, limit int) ([]domain.AuditEvent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	start := 0
	if limit > 0 && len(m.audits) > limit {
		start = len(m.audits) - limit
	}
	out := append([]domain.AuditEvent(nil), m.audits[start:]...)
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}
func (m *Memory) MarkEventProcessed(_ context.Context, id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.events[id]; ok {
		return false
	}
	m.events[id] = struct{}{}
	return true
}
func (m *Memory) ForgetProcessedEvent(_ context.Context, id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.events, id)
}

func (m *Memory) RecordMessageFeedback(_ context.Context, messageID, userID string, positive bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for conversationID, messages := range m.messages {
		conversation, ok := m.conversations[conversationID]
		if !ok || conversation.UserID != userID {
			continue
		}
		for _, message := range messages {
			if message.ID == messageID && message.Role == "assistant" {
				m.messageFeedback[messageID] = positive
				return nil
			}
		}
	}
	return ErrNotFound
}

func (m *Memory) Metrics(_ context.Context) domain.DashboardMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()
	now := time.Now()
	year, day := now.Year(), now.YearDay()
	questionsToday := 0
	assistantMessages := 0
	assistantWithCitations := 0
	for _, messages := range m.messages {
		for _, message := range messages {
			if message.Role == "user" && message.CreatedAt.Year() == year && message.CreatedAt.YearDay() == day {
				questionsToday++
			}
			if message.Role == "assistant" {
				assistantMessages++
				if len(message.Citations) > 0 {
					assistantWithCitations++
				}
			}
		}
	}
	positive := 0
	for _, value := range m.messageFeedback {
		if value {
			positive++
		}
	}
	published := 0
	for _, d := range m.documents {
		if d.Status == "published" {
			published++
		}
	}
	syncBacklog := 0
	for _, source := range m.sources {
		if source.SyncStatus == "queued" || source.SyncStatus == "pending" || source.SyncStatus == "syncing" {
			syncBacklog++
		}
	}
	deliveries, sent := 0, 0
	for _, status := range m.notificationDeliveries {
		if status == "sent" || status == "failed" {
			deliveries++
			if status == "sent" {
				sent++
			}
		}
	}
	return domain.DashboardMetrics{
		QuestionsToday:     questionsToday,
		PositiveRate:       ratio(positive, len(m.messageFeedback)),
		NoAnswerRate:       ratio(assistantMessages-assistantWithCitations, assistantMessages),
		CitationCoverage:   ratio(assistantWithCitations, assistantMessages),
		DocumentsPublished: published,
		SyncBacklog:        syncBacklog,
		DeliverySuccess:    ratio(sent, deliveries),
	}
}

func ratio(numerator, denominator int) float64 {
	if denominator == 0 {
		return 0
	}
	return float64(numerator) / float64(denominator)
}

func tokenize(value string) []string {
	value = strings.ToLower(strings.TrimSpace(value))
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ' ' || r == '，' || r == ',' || r == '。' || r == '？' || r == '?' || r == '、' || r == '：' || r == ':'
	})
	result := append([]string(nil), fields...)
	for _, field := range fields {
		runes := []rune(field)
		if len(runes) >= 4 && containsHan(runes) {
			for i := 0; i < len(runes)-1; i++ {
				result = append(result, string(runes[i:i+2]))
			}
		}
	}
	if len(result) == 0 && value != "" {
		return []string{value}
	}
	return result
}
func containsHan(values []rune) bool {
	for _, value := range values {
		if value >= 0x4E00 && value <= 0x9FFF {
			return true
		}
	}
	return false
}
func truncate(value string, max int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= max {
		return string(runes)
	}
	return string(runes[:max]) + "…"
}

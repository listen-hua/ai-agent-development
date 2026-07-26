package store

import (
	"context"
	"errors"
	"internal-ai-agent/backend/internal/domain"
	"time"
)

var ErrNotFound = errors.New("not found")
var ErrForbidden = errors.New("forbidden")
var ErrConflict = errors.New("conflict")

type Repository interface {
	UpsertUser(context.Context, domain.User) (domain.User, error)
	GetUser(context.Context, string) (domain.User, error)
	GetUserByOpenID(context.Context, string) (domain.User, error)
	ListUsers(context.Context) ([]domain.User, error)
	UpdateUserRoles(context.Context, string, []domain.Role) (domain.User, error)
	UpdateUserStatus(context.Context, string, string) error
	UpsertMeetingRooms(context.Context, []domain.MeetingRoom) error
	ListMeetingRooms(context.Context) ([]domain.MeetingRoom, error)
	GetMeetingRoom(context.Context, string) (domain.MeetingRoom, error)
	GetMeetingSettings(context.Context) (domain.MeetingSettings, error)
	SaveMeetingSettings(context.Context, domain.MeetingSettings) error
	CreateMeetingBookingAction(context.Context, domain.MeetingBookingAction) error
	GetMeetingBookingAction(context.Context, string) (domain.MeetingBookingAction, error)
	ClaimMeetingBookingAction(context.Context, string, string, string, time.Time) (domain.MeetingBookingAction, error)
	UpdateMeetingBookingAction(context.Context, domain.MeetingBookingAction) error
	CreateMeetingBooking(context.Context, domain.MeetingBooking) error
	GetMeetingBooking(context.Context, string) (domain.MeetingBooking, error)
	ListMeetingBookings(context.Context, string) ([]domain.MeetingBooking, error)
	UpdateMeetingBooking(context.Context, domain.MeetingBooking) error
	CreateMeetingBookingDelivery(context.Context, domain.MeetingBookingDelivery) error
	ClaimMeetingBookingDeliveries(context.Context, time.Time, time.Duration, int) ([]domain.MeetingBookingDelivery, error)
	UpdateMeetingBookingDelivery(context.Context, domain.MeetingBookingDelivery) error
	CreateConversation(context.Context, domain.Conversation) error
	ListConversations(context.Context, string) ([]domain.Conversation, error)
	GetConversation(context.Context, string) (domain.Conversation, error)
	DeleteConversation(context.Context, string, string) error
	AddMessage(context.Context, domain.Message) error
	ListMessages(context.Context, string) ([]domain.Message, error)
	UpdateMessageAnalysis(context.Context, string, string, string, int, int, int) error
	GetConversationContext(context.Context, string) (domain.ConversationContext, error)
	SaveConversationContext(context.Context, domain.ConversationContext) error
	ResetConversationContext(context.Context, string) error
	GetOrCreateBoundConversation(context.Context, domain.User, string, string, string, time.Time, time.Duration) (domain.Conversation, error)
	ResetConversationBinding(context.Context, string, string, string, string) error
	CleanupConversationHistory(context.Context, time.Time) error
	CreateSource(context.Context, domain.KnowledgeSource) error
	GetSource(context.Context, string) (domain.KnowledgeSource, error)
	ListSources(context.Context) ([]domain.KnowledgeSource, error)
	UpdateSource(context.Context, domain.KnowledgeSource) error
	CreateDocument(context.Context, domain.Document, []domain.Chunk) error
	CreateDocumentVersion(context.Context, domain.Document, domain.DocumentVersion, []domain.Chunk) error
	UpdateDocument(context.Context, domain.Document) error
	GetDocument(context.Context, string) (domain.Document, error)
	GetDocumentByRemoteToken(context.Context, string, string) (domain.Document, error)
	ListDocuments(context.Context) ([]domain.Document, error)
	ListDocumentsBySource(context.Context, string) ([]domain.Document, error)
	SearchChunks(context.Context, domain.User, string, []float32, int) ([]domain.Chunk, map[string]domain.Document, error)
	ListConfigs(context.Context) ([]domain.AgentConfigVersion, error)
	SaveConfig(context.Context, domain.AgentConfigVersion) error
	PublishConfig(context.Context, string) (domain.AgentConfigVersion, error)
	PublishedConfig(context.Context) (domain.AgentConfigVersion, error)
	ListAgentProfiles(context.Context) ([]domain.AgentProfile, error)
	GetAgentProfile(context.Context, string) (domain.AgentProfile, error)
	GetAgentProfileByKey(context.Context, string) (domain.AgentProfile, error)
	UpsertAgentProfile(context.Context, domain.AgentProfile) error
	CreateNotification(context.Context, domain.NotificationDraft) error
	UpdateNotification(context.Context, domain.NotificationDraft) error
	GetNotification(context.Context, string) (domain.NotificationDraft, error)
	ListNotifications(context.Context) ([]domain.NotificationDraft, error)
	ClaimDueNotifications(context.Context, time.Time, int) ([]domain.NotificationDraft, error)
	RecordNotificationDelivery(context.Context, string, string, string, string, string, string, string, int, time.Time) error
	CreateReminderAction(context.Context, domain.ReminderActionDraft) error
	CountReminderActionsSince(context.Context, string, time.Time) (int, error)
	GetReminderAction(context.Context, string) (domain.ReminderActionDraft, error)
	LatestPendingReminderAction(context.Context, string, string) (domain.ReminderActionDraft, error)
	ConfirmReminderAction(context.Context, string, string, *time.Time, time.Time) (domain.ReminderActionDraft, domain.Reminder, error)
	CancelReminderAction(context.Context, string, string, time.Time) (domain.ReminderActionDraft, error)
	ListReminders(context.Context, string) ([]domain.Reminder, error)
	GetReminder(context.Context, string) (domain.Reminder, error)
	ListReminderDeliveries(context.Context, string, int) ([]domain.ReminderDelivery, error)
	ListDueReminders(context.Context, time.Time, int) ([]domain.Reminder, error)
	ListActiveWorkdayReminders(context.Context) ([]domain.Reminder, error)
	CreateReminderDeliveryAndAdvance(context.Context, string, time.Time, *time.Time, time.Time) (domain.ReminderDelivery, bool, error)
	ClaimReminderDeliveries(context.Context, time.Time, time.Duration, int) ([]domain.ReminderDelivery, error)
	UpdateReminderDelivery(context.Context, domain.ReminderDelivery) error
	UpdateReminderRuntime(context.Context, string, string, *time.Time, string, time.Time) error
	UpdateReminderNextFire(context.Context, string, *time.Time, time.Time) error
	CleanupReminderHistory(context.Context, time.Time) error
	ListWorkdayOverrides(context.Context, int) ([]domain.WorkdayOverride, error)
	GetWorkdayOverride(context.Context, string) (domain.WorkdayOverride, error)
	UpsertWorkdayOverride(context.Context, domain.WorkdayOverride) error
	DeleteWorkdayOverride(context.Context, string) error
	CreateReminderBotJob(context.Context, domain.ReminderBotJob) (bool, error)
	ClaimReminderBotJobs(context.Context, time.Time, time.Duration, int) ([]domain.ReminderBotJob, error)
	UpdateReminderBotJob(context.Context, domain.ReminderBotJob) error
	AppendAudit(context.Context, domain.AuditEvent) error
	ListAudit(context.Context, int) ([]domain.AuditEvent, error)
	MarkEventProcessed(context.Context, string) bool
	ForgetProcessedEvent(context.Context, string)
	Metrics(context.Context) domain.DashboardMetrics
}

// ImageRepository is intentionally separate from Repository so the image agent
// remains an optional module for lightweight deployments and isolated tests.
type ImageRepository interface {
	ListImageRelays(context.Context) ([]domain.ImageRelay, error)
	GetImageRelay(context.Context, string) (domain.ImageRelay, error)
	UpsertImageRelay(context.Context, domain.ImageRelay) error
	ListImageModels(context.Context, string) ([]domain.ImageModel, error)
	GetImageModel(context.Context, string) (domain.ImageModel, error)
	UpsertImageModels(context.Context, []domain.ImageModel) error
	UpdateImageModel(context.Context, domain.ImageModel) error
	ListImageProjects(context.Context) ([]domain.ImageProject, error)
	GetImageProject(context.Context, string) (domain.ImageProject, error)
	UpsertImageProject(context.Context, domain.ImageProject) error
	ListImagePromptActions(context.Context, string) ([]domain.ImagePromptAction, error)
	GetImagePromptAction(context.Context, string) (domain.ImagePromptAction, error)
	UpsertImagePromptAction(context.Context, domain.ImagePromptAction) error
	DeleteImagePromptAction(context.Context, string) error
	GetOrCreateImageCanvas(context.Context, string, string) (domain.ImageCanvas, error)
	UpdateImageCanvas(context.Context, domain.ImageCanvas, int64) (domain.ImageCanvas, error)
	CreateImageAsset(context.Context, domain.ImageAsset) error
	GetImageAsset(context.Context, string) (domain.ImageAsset, error)
	CreateImageJob(context.Context, domain.ImageJob, []domain.ImageCanvasNode) (domain.ImageJob, error)
	GetImageJob(context.Context, string) (domain.ImageJob, error)
	ListImageJobs(context.Context, string, string, int) ([]domain.ImageJob, error)
	ClaimImageJobs(context.Context, time.Time, time.Duration, int) ([]domain.ImageJob, error)
	UpdateImageJob(context.Context, domain.ImageJob) error
	SaveImageJobOutput(context.Context, domain.ImageJobOutput, domain.ImageAsset, domain.ImageCanvasNode) error
	UpdateImageJobOutputFailure(context.Context, domain.ImageJobOutput, domain.ImageCanvasNode) error
	CleanupImageJobLogs(context.Context, time.Time) error
}

package domain

import "time"

const (
	MeetingActionPending    = "pending"
	MeetingActionProcessing = "processing"
	MeetingActionConfirmed  = "confirmed"
	MeetingActionCancelled  = "cancelled"
	MeetingActionExpired    = "expired"
)

type MeetingRoom struct {
	ID                      string     `json:"id"`
	RoomID                  string     `json:"room_id"`
	Name                    string     `json:"name"`
	Capacity                int        `json:"capacity"`
	RoomLevelID             string     `json:"room_level_id"`
	Path                    []string   `json:"path"`
	Enabled                 bool       `json:"enabled"`
	ScheduleEnabled         bool       `json:"schedule_enabled"`
	DisabledFrom            *time.Time `json:"disabled_from,omitempty"`
	DisabledUntil           *time.Time `json:"disabled_until,omitempty"`
	DisableReason           string     `json:"disable_reason,omitempty"`
	ApprovalSwitch          int        `json:"approval_switch"`
	ApprovalCondition       int        `json:"approval_condition"`
	ApprovalDurationHours   float64    `json:"approval_duration_hours"`
	ReservationStartSeconds int        `json:"reservation_start_seconds"`
	ReservationEndSeconds   int        `json:"reservation_end_seconds"`
	MaxDurationHours        int        `json:"max_duration_hours"`
	LastError               string     `json:"last_error,omitempty"`
	SyncedAt                time.Time  `json:"synced_at"`
}

func (r MeetingRoom) AvailableAt(start, end time.Time) bool {
	if !r.Enabled || !r.ScheduleEnabled || !end.After(start) {
		return false
	}
	if r.DisabledFrom == nil {
		return true
	}
	until := r.DisabledUntil
	if until == nil || until.IsZero() {
		return end.Before(*r.DisabledFrom) || end.Equal(*r.DisabledFrom)
	}
	return !start.Before(*until) || !end.After(*r.DisabledFrom)
}

func (r MeetingRoom) RequiresApproval(duration time.Duration) bool {
	if r.ApprovalSwitch != 1 {
		return false
	}
	if r.ApprovalCondition == 0 {
		return true
	}
	return r.ApprovalDurationHours <= 0 || duration.Hours() > r.ApprovalDurationHours
}

type MeetingBookingOption struct {
	ID       string    `json:"id"`
	RoomID   string    `json:"room_id"`
	RoomName string    `json:"room_name"`
	Capacity int       `json:"capacity"`
	StartAt  time.Time `json:"start_at"`
	EndAt    time.Time `json:"end_at"`
}

type MeetingAttendee struct {
	UserID string `json:"user_id"`
	OpenID string `json:"open_id"`
	Name   string `json:"name"`
}

type MeetingBookingDraftSlots struct {
	Date                string   `json:"date,omitempty"`
	Hour                int      `json:"hour,omitempty"`
	Minute              int      `json:"minute,omitempty"`
	DurationMinutes     int      `json:"duration_minutes,omitempty"`
	Title               string   `json:"title,omitempty"`
	AttendeeUserIDs     []string `json:"attendee_user_ids"`
	AttendeesConfirmed  bool     `json:"attendees_confirmed"`
	Capacity            int      `json:"capacity,omitempty"`
	OriginalQuery       string   `json:"original_query,omitempty"`
	RequestedRoomName   string   `json:"requested_room_name,omitempty"`
	CandidateBookingIDs []string `json:"candidate_booking_ids,omitempty"`
	SelectedBookingID   string   `json:"selected_booking_id,omitempty"`
}

type MeetingBookingDraft struct {
	ID              string                   `json:"id"`
	UserID          string                   `json:"user_id"`
	ConversationID  string                   `json:"conversation_id,omitempty"`
	Channel         string                   `json:"channel"`
	Intent          string                   `json:"intent"`
	Slots           MeetingBookingDraftSlots `json:"slots"`
	MissingFields   []string                 `json:"missing_fields"`
	Status          string                   `json:"status"`
	Version         int64                    `json:"version"`
	ResultActionID  string                   `json:"result_action_id,omitempty"`
	ResultAction    *MeetingBookingAction    `json:"result_action,omitempty"`
	BookingChoices  []MeetingBooking         `json:"booking_choices,omitempty"`
	AttendeeChoices []MeetingAttendeeOption  `json:"attendee_choices,omitempty"`
	ExpiresAt       time.Time                `json:"expires_at"`
	CreatedAt       time.Time                `json:"created_at"`
	UpdatedAt       time.Time                `json:"updated_at"`
}

type MeetingAttendeeOption struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	AvatarURL       string   `json:"avatar_url,omitempty"`
	DepartmentIDs   []string `json:"department_ids"`
	DepartmentNames []string `json:"department_names,omitempty"`
	JobTitle        string   `json:"job_title,omitempty"`
}

type MeetingBookingAction struct {
	ID                   string                 `json:"id"`
	UserID               string                 `json:"user_id"`
	Intent               string                 `json:"intent"`
	Title                string                 `json:"title"`
	Attendees            []MeetingAttendee      `json:"attendees"`
	Capacity             int                    `json:"capacity"`
	RequestedRoomName    string                 `json:"requested_room_name,omitempty"`
	Options              []MeetingBookingOption `json:"options"`
	SelectedOptionID     string                 `json:"selected_option_id,omitempty"`
	BookingID            string                 `json:"booking_id,omitempty"`
	ResultBookingID      string                 `json:"result_booking_id,omitempty"`
	Status               string                 `json:"status"`
	SourceChannel        string                 `json:"source_channel"`
	SourceConversationID string                 `json:"source_conversation_id,omitempty"`
	ExpiresAt            time.Time              `json:"expires_at"`
	CreatedAt            time.Time              `json:"created_at"`
	ConfirmedAt          *time.Time             `json:"confirmed_at,omitempty"`
}

type MeetingBooking struct {
	ID                   string            `json:"id"`
	UserID               string            `json:"user_id"`
	CalendarID           string            `json:"calendar_id"`
	EventID              string            `json:"event_id"`
	RoomID               string            `json:"room_id"`
	RoomName             string            `json:"room_name"`
	Title                string            `json:"title"`
	StartAt              time.Time         `json:"start_at"`
	EndAt                time.Time         `json:"end_at"`
	Attendees            []MeetingAttendee `json:"attendees"`
	Status               string            `json:"status"`
	ReplacesBookingID    string            `json:"replaces_booking_id,omitempty"`
	ReplacedByBookingID  string            `json:"replaced_by_booking_id,omitempty"`
	LastError            string            `json:"last_error,omitempty"`
	SourceChannel        string            `json:"source_channel"`
	SourceConversationID string            `json:"source_conversation_id,omitempty"`
	CreatedAt            time.Time         `json:"created_at"`
	UpdatedAt            time.Time         `json:"updated_at"`
}

type MeetingBookingDelivery struct {
	ID             string     `json:"id"`
	BookingID      string     `json:"booking_id"`
	ScheduledFor   time.Time  `json:"scheduled_for"`
	Status         string     `json:"status"`
	Attempts       int        `json:"attempts"`
	NextAttemptAt  time.Time  `json:"next_attempt_at"`
	LockedUntil    *time.Time `json:"locked_until,omitempty"`
	MessageID      string     `json:"message_id,omitempty"`
	Error          string     `json:"error,omitempty"`
	IdempotencyKey string     `json:"idempotency_key"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type MeetingSettings struct {
	CalendarID         string     `json:"calendar_id"`
	Timezone           string     `json:"timezone"`
	WorkdayStart       string     `json:"workday_start"`
	WorkdayEnd         string     `json:"workday_end"`
	SlotMinutes        int        `json:"slot_minutes"`
	SyncIntervalMinute int        `json:"sync_interval_minutes"`
	LastSyncedAt       *time.Time `json:"last_synced_at,omitempty"`
	LastSyncError      string     `json:"last_sync_error,omitempty"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

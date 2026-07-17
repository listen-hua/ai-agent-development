package domain

import (
	"fmt"
	"strings"
	"time"
)

const (
	ReminderOnce    = "once"
	ReminderDaily   = "daily"
	ReminderWorkday = "workday"
	ReminderWeekly  = "weekly"
)

type ReminderSchedule struct {
	Type      string     `json:"type"`
	Timezone  string     `json:"timezone"`
	LocalTime string     `json:"local_time,omitempty"`
	Weekdays  []int      `json:"weekdays,omitempty"`
	OnceAt    *time.Time `json:"once_at,omitempty"`
}

func (s ReminderSchedule) Validate() error {
	if s.Timezone != "Asia/Shanghai" {
		return fmt.Errorf("unsupported timezone")
	}
	switch s.Type {
	case ReminderOnce:
		if s.OnceAt == nil {
			return fmt.Errorf("once_at is required")
		}
	case ReminderDaily, ReminderWorkday:
		if !validLocalTime(s.LocalTime) {
			return fmt.Errorf("local_time is invalid")
		}
	case ReminderWeekly:
		if !validLocalTime(s.LocalTime) || len(s.Weekdays) == 0 {
			return fmt.Errorf("weekly schedule requires local_time and weekdays")
		}
		for _, weekday := range s.Weekdays {
			if weekday < 1 || weekday > 7 {
				return fmt.Errorf("weekday is invalid")
			}
		}
	default:
		return fmt.Errorf("schedule type is invalid")
	}
	return nil
}

func validLocalTime(value string) bool {
	_, err := time.Parse("15:04", value)
	return err == nil
}

type Reminder struct {
	ID                   string           `json:"id"`
	UserID               string           `json:"user_id"`
	Content              string           `json:"content"`
	Status               string           `json:"status"`
	Schedule             ReminderSchedule `json:"schedule"`
	NextFireAt           *time.Time       `json:"next_fire_at,omitempty"`
	LastFiredAt          *time.Time       `json:"last_fired_at,omitempty"`
	LastError            string           `json:"last_error,omitempty"`
	SourceChannel        string           `json:"source_channel"`
	SourceConversationID string           `json:"source_conversation_id,omitempty"`
	Version              int              `json:"version"`
	CreatedAt            time.Time        `json:"created_at"`
	UpdatedAt            time.Time        `json:"updated_at"`
}

type ReminderActionDraft struct {
	ID                   string            `json:"id"`
	UserID               string            `json:"user_id"`
	Action               string            `json:"action"`
	ReminderID           string            `json:"reminder_id,omitempty"`
	Content              string            `json:"content,omitempty"`
	Schedule             *ReminderSchedule `json:"schedule,omitempty"`
	Status               string            `json:"status"`
	SourceChannel        string            `json:"source_channel"`
	SourceConversationID string            `json:"source_conversation_id,omitempty"`
	ResultReminderID     string            `json:"result_reminder_id,omitempty"`
	ExpiresAt            time.Time         `json:"expires_at"`
	CreatedAt            time.Time         `json:"created_at"`
	ConfirmedAt          *time.Time        `json:"confirmed_at,omitempty"`
}

type ReminderDelivery struct {
	ID             string     `json:"id"`
	ReminderID     string     `json:"reminder_id"`
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

type WorkdayOverride struct {
	Date      string    `json:"date"`
	IsWorkday bool      `json:"is_workday"`
	Note      string    `json:"note,omitempty"`
	UpdatedBy string    `json:"updated_by,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ReminderBotJob struct {
	ID          string    `json:"id"`
	EventID     string    `json:"event_id"`
	OpenID      string    `json:"open_id"`
	ChatID      string    `json:"chat_id"`
	MessageID   string    `json:"message_id"`
	Content     string    `json:"content"`
	Status      string    `json:"status"`
	Attempts    int       `json:"attempts"`
	LastError   string    `json:"last_error,omitempty"`
	AvailableAt time.Time `json:"available_at"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func NormalizeReminderContent(value string) (string, error) {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) == 0 {
		return "", fmt.Errorf("reminder content is required")
	}
	if len(runes) > 500 {
		return "", fmt.Errorf("reminder content is too long")
	}
	return value, nil
}

package domain

import (
	"fmt"
	"strings"
	"time"
)

const (
	MassageCycleDraft      = "draft"
	MassageCycleScheduled  = "scheduled"
	MassageCycleSignupOpen = "signup_open"
	MassageCycleInProgress = "in_progress"
	MassageCycleCompleted  = "completed"
	MassageCycleCancelled  = "cancelled"

	MassageSessionScheduled = "scheduled"
	MassageSessionRunning   = "running"
	MassageSessionPaused    = "paused"
	MassageSessionCompleted = "completed"
	MassageSessionClosed    = "closed"
)

type MassageAudience struct {
	Scope           string   `json:"scope"`
	DepartmentIDs   []string `json:"department_ids,omitempty"`
	UserIDs         []string `json:"user_ids,omitempty"`
	ExcludedUserIDs []string `json:"excluded_user_ids,omitempty"`
}

func (a MassageAudience) Allows(user User) bool {
	for _, id := range a.ExcludedUserIDs {
		if id == user.ID {
			return false
		}
	}
	if user.Status != "active" || user.FeishuOpenID == "" {
		return false
	}
	if a.Scope == "all" || a.Scope == "" {
		return true
	}
	for _, id := range a.UserIDs {
		if id == user.ID {
			return true
		}
	}
	for _, wanted := range a.DepartmentIDs {
		for _, owned := range user.DepartmentIDs {
			if wanted == owned {
				return true
			}
		}
	}
	return false
}

type MassageCycle struct {
	ID             string           `json:"id"`
	ServiceMonth   string           `json:"service_month"`
	Title          string           `json:"title"`
	SignupNoticeAt time.Time        `json:"signup_notice_at"`
	SignupDeadline time.Time        `json:"signup_deadline"`
	Audience       MassageAudience  `json:"audience"`
	Status         string           `json:"status"`
	NextQueue      int              `json:"next_queue"`
	EligibleCount  int              `json:"eligible_count"`
	EnrolledCount  int              `json:"enrolled_count"`
	Sessions       []MassageSession `json:"sessions"`
	CreatedBy      string           `json:"created_by"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
}

type MassageSession struct {
	ID              string     `json:"id"`
	CycleID         string     `json:"cycle_id"`
	Sequence        int        `json:"sequence"`
	StartsAt        time.Time  `json:"starts_at"`
	Quota           int        `json:"quota"`
	ConcurrentSlots int        `json:"concurrent_slots"`
	Status          string     `json:"status"`
	CompletedCount  int        `json:"completed_count"`
	StartedAt       *time.Time `json:"started_at,omitempty"`
	ClosedAt        *time.Time `json:"closed_at,omitempty"`
	CloseReason     string     `json:"close_reason,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type MassageEnrollment struct {
	ID          string     `json:"id"`
	CycleID     string     `json:"cycle_id"`
	UserID      string     `json:"user_id"`
	UserName    string     `json:"user_name,omitempty"`
	QueueNumber int        `json:"queue_number"`
	Status      string     `json:"status"`
	EnrolledAt  *time.Time `json:"enrolled_at,omitempty"`
	WithdrawnAt *time.Time `json:"withdrawn_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type MassageCall struct {
	ID            string     `json:"id"`
	SessionID     string     `json:"session_id"`
	EnrollmentID  string     `json:"enrollment_id"`
	UserID        string     `json:"user_id"`
	UserName      string     `json:"user_name,omitempty"`
	QueueNumber   int        `json:"queue_number"`
	Status        string     `json:"status"`
	CalledAt      *time.Time `json:"called_at,omitempty"`
	ResponseDueAt *time.Time `json:"response_due_at,omitempty"`
	RespondedAt   *time.Time `json:"responded_at,omitempty"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
	MessageID     string     `json:"message_id,omitempty"`
	LastError     string     `json:"last_error,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type MassageDelivery struct {
	ID             string     `json:"id"`
	CycleID        string     `json:"cycle_id"`
	SessionID      string     `json:"session_id,omitempty"`
	CallID         string     `json:"call_id,omitempty"`
	UserID         string     `json:"user_id"`
	Kind           string     `json:"kind"`
	Status         string     `json:"status"`
	Attempts       int        `json:"attempts"`
	AvailableAt    time.Time  `json:"available_at"`
	LockedUntil    *time.Time `json:"locked_until,omitempty"`
	MessageID      string     `json:"message_id,omitempty"`
	LastError      string     `json:"last_error,omitempty"`
	IdempotencyKey string     `json:"idempotency_key"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type MassageStatistic struct {
	CycleID      string              `json:"cycle_id"`
	Eligible     int                 `json:"eligible"`
	Enrolled     int                 `json:"enrolled"`
	Completed    int                 `json:"completed"`
	Rejected     int                 `json:"rejected"`
	TimedOut     int                 `json:"timed_out"`
	NoShow       int                 `json:"no_show"`
	DeliveryFail int                 `json:"delivery_failed"`
	Unserved     int                 `json:"unserved"`
	Enrollments  []MassageEnrollment `json:"enrollments"`
	Calls        []MassageCall       `json:"calls"`
}

type MassageMe struct {
	Cycle              MassageCycle       `json:"cycle"`
	Enrollment         *MassageEnrollment `json:"enrollment,omitempty"`
	CurrentCall        *MassageCall       `json:"current_call,omitempty"`
	CurrentQueueNumber int                `json:"current_queue_number,omitempty"`
}

type MassageAction struct {
	Type    string `json:"type"`
	CycleID string `json:"cycle_id"`
	Label   string `json:"label"`
}

func ValidateMassageCycle(value MassageCycle) error {
	value.ServiceMonth = strings.TrimSpace(value.ServiceMonth)
	value.Title = strings.TrimSpace(value.Title)
	if _, err := time.Parse("2006-01", value.ServiceMonth); err != nil {
		return fmt.Errorf("服务月份格式无效，请使用 YYYY-MM")
	}
	if value.Title == "" || len([]rune(value.Title)) > 100 {
		return fmt.Errorf("批次名称不能为空且不能超过 100 个字符")
	}
	if !value.SignupNoticeAt.Before(value.SignupDeadline) {
		return fmt.Errorf("报名通知时间必须早于报名截止时间")
	}
	if len(value.Sessions) != 2 {
		return fmt.Errorf("每个按摩批次必须包含两场按摩")
	}
	if value.Sessions[0].Sequence != 1 || value.Sessions[1].Sequence != 2 || !value.Sessions[0].StartsAt.Before(value.Sessions[1].StartsAt) {
		return fmt.Errorf("第二场按摩时间必须晚于第一场")
	}
	if value.SignupDeadline.After(value.Sessions[0].StartsAt) {
		return fmt.Errorf("报名截止时间不能晚于第一场按摩时间")
	}
	for _, session := range value.Sessions {
		if session.Quota < 1 || session.Quota > 10000 || session.ConcurrentSlots < 1 || session.ConcurrentSlots > 20 {
			return fmt.Errorf("本场人数或并行服务人数超出允许范围")
		}
	}
	if value.Audience.Scope != "" && value.Audience.Scope != "all" && value.Audience.Scope != "restricted" {
		return fmt.Errorf("参与范围设置无效")
	}
	return nil
}

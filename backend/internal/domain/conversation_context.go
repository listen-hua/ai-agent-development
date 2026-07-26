package domain

import "time"

const (
	AgentAdministrativeAssistant = "administrative_assistant"
	ConversationChannelH5        = "h5"
	ConversationChannelFeishuBot = "feishu_bot"
)

type ConversationTaskState struct {
	Intent       string         `json:"intent,omitempty"`
	Status       string         `json:"status,omitempty"`
	Slots        map[string]any `json:"slots,omitempty"`
	MissingSlots []string       `json:"missing_slots,omitempty"`
}

type ConversationContext struct {
	ConversationID             string                `json:"conversation_id"`
	Summary                    string                `json:"summary,omitempty"`
	SummarizedThroughMessageID string                `json:"summarized_through_message_id,omitempty"`
	ResetThroughMessageID      string                `json:"reset_through_message_id,omitempty"`
	ActiveTask                 ConversationTaskState `json:"active_task"`
	Version                    int                   `json:"version"`
	ExpiresAt                  *time.Time            `json:"expires_at,omitempty"`
	UpdatedAt                  time.Time             `json:"updated_at"`
}

type ConversationBinding struct {
	UserID          string    `json:"user_id"`
	AgentKey        string    `json:"agent_key"`
	Channel         string    `json:"channel"`
	ExternalScopeID string    `json:"external_scope_id"`
	ConversationID  string    `json:"conversation_id"`
	ExpiresAt       time.Time `json:"expires_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type TurnUnderstanding struct {
	Intent          string                `json:"intent"`
	StandaloneQuery string                `json:"standalone_query"`
	NeedsContext    bool                  `json:"needs_context,omitempty"`
	ActiveTask      ConversationTaskState `json:"active_task"`
}

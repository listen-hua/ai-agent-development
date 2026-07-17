package domain

import (
	"fmt"
	"strings"
	"time"
)

type Role string

const (
	RoleEmployee          Role = "employee"
	RoleKnowledgeAdmin    Role = "knowledge_admin"
	RoleNotificationAdmin Role = "notification_admin"
	RoleAuditor           Role = "auditor"
	RoleSuperAdmin        Role = "super_admin"
)

type User struct {
	ID                   string     `json:"id"`
	FeishuOpenID         string     `json:"feishu_open_id"`
	Name                 string     `json:"name"`
	AvatarURL            string     `json:"avatar_url"`
	DepartmentIDs        []string   `json:"department_ids"`
	JobTitle             string     `json:"job_title"`
	JobLevelID           string     `json:"job_level_id"`
	JobFamilyID          string     `json:"job_family_id"`
	EmployeeType         int        `json:"employee_type"`
	Status               string     `json:"status"`
	OrganizationSyncedAt *time.Time `json:"organization_synced_at,omitempty"`
	Roles                []Role     `json:"roles"`
}

func (u User) HasRole(roles ...Role) bool {
	for _, owned := range u.Roles {
		if owned == RoleSuperAdmin {
			return true
		}
		for _, wanted := range roles {
			if owned == wanted {
				return true
			}
		}
	}
	return false
}

func ValidRole(role Role) bool {
	switch role {
	case RoleEmployee, RoleKnowledgeAdmin, RoleNotificationAdmin, RoleAuditor, RoleSuperAdmin:
		return true
	default:
		return false
	}
}

func NormalizeRoles(roles []Role) ([]Role, error) {
	seen := map[Role]bool{RoleEmployee: true}
	out := []Role{RoleEmployee}
	for _, role := range roles {
		if !ValidRole(role) {
			return nil, fmt.Errorf("invalid role: %s", role)
		}
		if !seen[role] {
			seen[role] = true
			out = append(out, role)
		}
	}
	return out, nil
}

type ACLRule struct {
	DepartmentIDs []string `json:"department_ids,omitempty"`
	JobTitles     []string `json:"job_titles,omitempty"`
	JobLevelIDs   []string `json:"job_level_ids,omitempty"`
	JobFamilyIDs  []string `json:"job_family_ids,omitempty"`
	EmployeeTypes []int    `json:"employee_types,omitempty"`
	UserIDs       []string `json:"user_ids,omitempty"`
}

type ACL struct {
	Scope         string    `json:"scope"`
	DepartmentIDs []string  `json:"department_ids,omitempty"`
	RoleNames     []Role    `json:"role_names,omitempty"`
	UserIDs       []string  `json:"user_ids,omitempty"`
	Rules         []ACLRule `json:"rules,omitempty"`
}

func (a ACL) Allows(user User) bool {
	if a.Scope == "all" || user.HasRole(RoleSuperAdmin) {
		return true
	}
	for _, rule := range a.Rules {
		if rule.matches(user) {
			return true
		}
	}
	// Backward compatibility for ACLs created before rule groups were introduced.
	for _, id := range a.UserIDs {
		if id == user.ID {
			return true
		}
	}
	for _, dept := range a.DepartmentIDs {
		for _, owned := range user.DepartmentIDs {
			if dept == owned {
				return true
			}
		}
	}
	for _, role := range a.RoleNames {
		if user.HasRole(role) {
			return true
		}
	}
	return false
}

func (r ACLRule) matches(user User) bool {
	if len(r.DepartmentIDs)+len(r.JobTitles)+len(r.JobLevelIDs)+len(r.JobFamilyIDs)+len(r.EmployeeTypes)+len(r.UserIDs) == 0 {
		return false
	}
	return (len(r.DepartmentIDs) == 0 || overlaps(r.DepartmentIDs, user.DepartmentIDs)) &&
		(len(r.JobTitles) == 0 || contains(r.JobTitles, user.JobTitle)) &&
		(len(r.JobLevelIDs) == 0 || contains(r.JobLevelIDs, user.JobLevelID)) &&
		(len(r.JobFamilyIDs) == 0 || contains(r.JobFamilyIDs, user.JobFamilyID)) &&
		(len(r.EmployeeTypes) == 0 || containsInt(r.EmployeeTypes, user.EmployeeType)) &&
		(len(r.UserIDs) == 0 || contains(r.UserIDs, user.ID))
}

func (a ACL) Validate() error {
	if a.Scope != "all" && a.Scope != "restricted" {
		return errorsNew("scope must be all or restricted")
	}
	if a.Scope == "all" {
		return nil
	}
	if len(a.Rules) == 0 && len(a.DepartmentIDs) == 0 && len(a.RoleNames) == 0 && len(a.UserIDs) == 0 {
		return errorsNew("restricted ACL must contain at least one rule")
	}
	for _, rule := range a.Rules {
		if len(rule.DepartmentIDs)+len(rule.JobTitles)+len(rule.JobLevelIDs)+len(rule.JobFamilyIDs)+len(rule.EmployeeTypes)+len(rule.UserIDs) == 0 {
			return errorsNew("ACL rule cannot be empty")
		}
	}
	return nil
}

func errorsNew(message string) error { return fmt.Errorf("%s", message) }

func contains(values []string, expected string) bool {
	expected = strings.TrimSpace(expected)
	for _, value := range values {
		if strings.TrimSpace(value) == expected {
			return true
		}
	}
	return false
}

func overlaps(left, right []string) bool {
	for _, value := range left {
		if contains(right, value) {
			return true
		}
	}
	return false
}

func containsInt(values []int, expected int) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

type KnowledgeSource struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	Type          string     `json:"type"`
	RemoteToken   string     `json:"remote_token,omitempty"`
	DefaultACL    ACL        `json:"default_acl"`
	SyncStatus    string     `json:"sync_status"`
	SyncError     string     `json:"sync_error,omitempty"`
	LastSyncStats SyncStats  `json:"last_sync_stats"`
	LastSyncedAt  *time.Time `json:"last_synced_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

type SyncStats struct {
	Discovered  int `json:"discovered"`
	Created     int `json:"created"`
	Updated     int `json:"updated"`
	Unchanged   int `json:"unchanged"`
	Unavailable int `json:"unavailable"`
	Skipped     int `json:"skipped"`
	Failed      int `json:"failed"`
}

type DocumentVersion struct {
	ID          string     `json:"id"`
	Version     string     `json:"version"`
	Checksum    string     `json:"checksum"`
	ObjectKey   string     `json:"object_key,omitempty"`
	MimeType    string     `json:"mime_type"`
	Status      string     `json:"status"`
	EffectiveAt *time.Time `json:"effective_at,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

type Document struct {
	ID          string            `json:"id"`
	SourceID    string            `json:"source_id,omitempty"`
	Title       string            `json:"title"`
	SourceURL   string            `json:"source_url,omitempty"`
	RemoteToken string            `json:"remote_token,omitempty"`
	ACL         ACL               `json:"acl"`
	Status      string            `json:"status"`
	Versions    []DocumentVersion `json:"versions"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

type Chunk struct {
	ID         string    `json:"id"`
	DocumentID string    `json:"document_id"`
	VersionID  string    `json:"version_id"`
	Ordinal    int       `json:"ordinal"`
	Heading    string    `json:"heading"`
	Page       int       `json:"page"`
	Content    string    `json:"content"`
	Embedding  []float32 `json:"-"`
}

type Citation struct {
	ID         string `json:"id"`
	DocumentID string `json:"document_id"`
	VersionID  string `json:"version_id"`
	Title      string `json:"title"`
	Version    string `json:"version"`
	Heading    string `json:"heading"`
	Page       int    `json:"page"`
	Excerpt    string `json:"excerpt"`
	SourceURL  string `json:"source_url,omitempty"`
}

type Conversation struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Message struct {
	ID             string               `json:"id"`
	ConversationID string               `json:"conversation_id"`
	Role           string               `json:"role"`
	Content        string               `json:"content"`
	Citations      []Citation           `json:"citations"`
	Model          string               `json:"model,omitempty"`
	ReminderAction *ReminderActionDraft `json:"reminder_action,omitempty"`
	CreatedAt      time.Time            `json:"created_at"`
}

type RunEvent struct {
	Type      string      `json:"type"`
	RunID     string      `json:"run_id"`
	Delta     string      `json:"delta,omitempty"`
	Citation  *Citation   `json:"citation,omitempty"`
	Message   *Message    `json:"message,omitempty"`
	Error     string      `json:"error,omitempty"`
	Metadata  interface{} `json:"metadata,omitempty"`
	CreatedAt time.Time   `json:"created_at"`
}

type AgentConfig struct {
	GenerationModel string  `json:"generation_model"`
	EmbeddingModel  string  `json:"embedding_model"`
	RerankModel     string  `json:"rerank_model"`
	Temperature     float64 `json:"temperature"`
	MaxOutputTokens int     `json:"max_output_tokens"`
	TimeoutSeconds  int     `json:"timeout_seconds"`
	RetrievalTopK   int     `json:"retrieval_top_k"`
	RerankTopN      int     `json:"rerank_top_n"`
	ScoreThreshold  float64 `json:"score_threshold"`
	ContextBudget   int     `json:"context_budget"`
	SystemPrompt    string  `json:"system_prompt"`
}

type AgentConfigVersion struct {
	ID          string      `json:"id"`
	Version     int         `json:"version"`
	Status      string      `json:"status"`
	Config      AgentConfig `json:"config"`
	CreatedBy   string      `json:"created_by"`
	CreatedAt   time.Time   `json:"created_at"`
	PublishedAt *time.Time  `json:"published_at,omitempty"`
}

type AgentProfile struct {
	ID               string         `json:"id"`
	AgentKey         string         `json:"agent_key"`
	Name             string         `json:"name"`
	Description      string         `json:"description"`
	Kind             string         `json:"kind"`
	Provider         string         `json:"provider"`
	Model            string         `json:"model"`
	Enabled          bool           `json:"enabled"`
	HasAPIKey        bool           `json:"has_api_key"`
	APIKeyHint       string         `json:"api_key_hint"`
	CredentialSource string         `json:"credential_source"`
	EncryptedAPIKey  string         `json:"-"`
	Settings         map[string]any `json:"settings"`
	CreatedBy        string         `json:"created_by,omitempty"`
	UpdatedBy        string         `json:"updated_by,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
}

type NotificationDraft struct {
	ID             string                  `json:"id"`
	Title          string                  `json:"title"`
	Content        string                  `json:"content"`
	ContentFormat  string                  `json:"content_format"`
	Images         []NotificationImage     `json:"images"`
	RecipientType  string                  `json:"recipient_type"`
	Recipients     []NotificationRecipient `json:"recipients"`
	Audience       ACL                     `json:"audience"`
	Status         string                  `json:"status"`
	ScheduledAt    *time.Time              `json:"scheduled_at,omitempty"`
	ApprovedBy     string                  `json:"approved_by,omitempty"`
	CreatedBy      string                  `json:"created_by"`
	IdempotencyKey string                  `json:"idempotency_key"`
	RecipientCount int                     `json:"recipient_count"`
	LastError      string                  `json:"last_error,omitempty"`
	CreatedAt      time.Time               `json:"created_at"`
	UpdatedAt      time.Time               `json:"updated_at"`
}

type NotificationImage struct {
	ImageKey string `json:"image_key"`
	Name     string `json:"name"`
	Alt      string `json:"alt"`
}

type NotificationRecipient struct {
	Type string `json:"type"`
	ID   string `json:"id"`
	Name string `json:"name"`
}

type NotificationTargetOption struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url,omitempty"`
}

type AuditEvent struct {
	ID           string         `json:"id"`
	ActorID      string         `json:"actor_id"`
	ActorName    string         `json:"actor_name"`
	Action       string         `json:"action"`
	ResourceType string         `json:"resource_type"`
	ResourceID   string         `json:"resource_id"`
	Metadata     map[string]any `json:"metadata"`
	CreatedAt    time.Time      `json:"created_at"`
}

type DashboardMetrics struct {
	QuestionsToday     int     `json:"questions_today"`
	PositiveRate       float64 `json:"positive_rate"`
	NoAnswerRate       float64 `json:"no_answer_rate"`
	CitationCoverage   float64 `json:"citation_coverage"`
	DocumentsPublished int     `json:"documents_published"`
	SyncBacklog        int     `json:"sync_backlog"`
	DeliverySuccess    float64 `json:"delivery_success"`
}

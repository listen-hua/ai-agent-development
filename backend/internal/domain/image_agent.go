package domain

import "time"

type ImageRelay struct {
	ID                 string    `json:"id"`
	RelayKey           string    `json:"relay_key"`
	Name               string    `json:"name"`
	BaseURL            string    `json:"base_url"`
	Enabled            bool      `json:"enabled"`
	TimeoutSeconds     int       `json:"timeout_seconds"`
	AllowedOutputHosts []string  `json:"allowed_output_hosts"`
	EncryptedAPIKey    string    `json:"-"`
	APIKeyHint         string    `json:"api_key_hint"`
	HasAPIKey          bool      `json:"has_api_key"`
	CreatedBy          string    `json:"created_by,omitempty"`
	UpdatedBy          string    `json:"updated_by,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type ImageModel struct {
	ID                  string    `json:"id"`
	RelayID             string    `json:"relay_id"`
	ModelID             string    `json:"model_id"`
	RequestModelID      string    `json:"request_model_id"`
	RemoteEndpointTypes []string  `json:"remote_endpoint_types"`
	DisplayName         string    `json:"display_name"`
	Protocol            string    `json:"protocol"`
	Enabled             bool      `json:"enabled"`
	SupportsReference   bool      `json:"supports_reference"`
	SupportsReverse     bool      `json:"supports_reverse"`
	SupportedSizes      []string  `json:"supported_sizes"`
	MaxCount            int       `json:"max_count"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type ImageProject struct {
	ID          string    `json:"id"`
	ProjectKey  string    `json:"project_key"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	ACL         ACL       `json:"acl"`
	Enabled     bool      `json:"enabled"`
	CreatedBy   string    `json:"created_by,omitempty"`
	UpdatedBy   string    `json:"updated_by,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type ImagePromptAction struct {
	ID               string    `json:"id"`
	ActionKey        string    `json:"action_key"`
	Name             string    `json:"name"`
	PromptTemplate   string    `json:"prompt_template"`
	ProjectID        string    `json:"project_id,omitempty"`
	ProjectIDs       []string  `json:"project_ids,omitempty"`
	Enabled          bool      `json:"enabled"`
	SortOrder        int       `json:"sort_order"`
	HasPreview       bool      `json:"has_preview"`
	PreviewObjectKey string    `json:"-"`
	PreviewMIMEType  string    `json:"preview_mime_type,omitempty"`
	PreviewSizeBytes int64     `json:"preview_size_bytes,omitempty"`
	PreviewWidth     int       `json:"preview_width,omitempty"`
	PreviewHeight    int       `json:"preview_height,omitempty"`
	CreatedBy        string    `json:"created_by,omitempty"`
	UpdatedBy        string    `json:"updated_by,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type ImageViewport struct {
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	Zoom float64 `json:"zoom"`
}

type ImageCanvas struct {
	ID             string            `json:"id"`
	UserID         string            `json:"user_id"`
	ProjectID      string            `json:"project_id,omitempty"`
	Name           string            `json:"name"`
	Viewport       ImageViewport     `json:"viewport"`
	Version        int64             `json:"version"`
	NodeCount      int               `json:"node_count"`
	PreviewAssetID string            `json:"preview_asset_id,omitempty"`
	DeletedAt      *time.Time        `json:"deleted_at,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
	Nodes          []ImageCanvasNode `json:"nodes"`
}

type ImageCanvasNode struct {
	ID                     string    `json:"id"`
	CanvasID               string    `json:"canvas_id"`
	AssetID                string    `json:"asset_id,omitempty"`
	JobID                  string    `json:"job_id,omitempty"`
	BackgroundRemovalJobID string    `json:"background_removal_job_id,omitempty"`
	SourceNodeID           string    `json:"source_node_id,omitempty"`
	OutputIndex            int       `json:"output_index"`
	Status                 string    `json:"status"`
	X                      float64   `json:"x"`
	Y                      float64   `json:"y"`
	Width                  float64   `json:"width"`
	Height                 float64   `json:"height"`
	ZIndex                 int       `json:"z_index"`
	Error                  string    `json:"error,omitempty"`
	RequestedSize          string    `json:"requested_size,omitempty"`
	ActualWidth            int       `json:"actual_width,omitempty"`
	ActualHeight           int       `json:"actual_height,omitempty"`
	ResolutionWarning      string    `json:"resolution_warning,omitempty"`
	GenerationRelayName    string    `json:"generation_relay_name,omitempty"`
	GenerationModelName    string    `json:"generation_model_name,omitempty"`
	GenerationModelKey     string    `json:"generation_model_key,omitempty"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}

type ImageAsset struct {
	ID            string    `json:"id"`
	OwnerID       string    `json:"owner_id"`
	ProjectID     string    `json:"project_id"`
	ObjectKey     string    `json:"-"`
	MIMEType      string    `json:"mime_type"`
	FileName      string    `json:"file_name"`
	Width         int       `json:"width"`
	Height        int       `json:"height"`
	SizeBytes     int64     `json:"size_bytes"`
	Source        string    `json:"source"`
	SourceAssetID string    `json:"source_asset_id,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

type ImageJob struct {
	ID                string           `json:"id"`
	UserID            string           `json:"user_id"`
	ProjectID         string           `json:"project_id"`
	CanvasID          string           `json:"canvas_id"`
	RelayID           string           `json:"relay_id"`
	ModelID           string           `json:"model_id"`
	Kind              string           `json:"kind"`
	Prompt            string           `json:"prompt"`
	AspectRatio       string           `json:"aspect_ratio"`
	ImageSize         string           `json:"image_size"`
	Count             int              `json:"count"`
	ReferenceAssetIDs []string         `json:"reference_asset_ids"`
	Status            string           `json:"status"`
	Attempts          int              `json:"attempts"`
	NextAttemptAt     time.Time        `json:"next_attempt_at"`
	LockedUntil       *time.Time       `json:"locked_until,omitempty"`
	CompletedCount    int              `json:"completed_count"`
	ReversedPrompt    string           `json:"reversed_prompt,omitempty"`
	Error             string           `json:"error,omitempty"`
	IdempotencyKey    string           `json:"-"`
	CreatedAt         time.Time        `json:"created_at"`
	UpdatedAt         time.Time        `json:"updated_at"`
	Outputs           []ImageJobOutput `json:"outputs,omitempty"`
}

type ImageJobOutput struct {
	ID                string    `json:"id"`
	JobID             string    `json:"job_id"`
	OutputIndex       int       `json:"output_index"`
	AssetID           string    `json:"asset_id,omitempty"`
	Status            string    `json:"status"`
	Error             string    `json:"error,omitempty"`
	Attempts          int       `json:"attempts"`
	RequestedSize     string    `json:"requested_size,omitempty"`
	ActualWidth       int       `json:"actual_width,omitempty"`
	ActualHeight      int       `json:"actual_height,omitempty"`
	ResolutionWarning string    `json:"resolution_warning,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type ImageAgentOptions struct {
	Relays                   []ImageRelay        `json:"relays"`
	Models                   []ImageModel        `json:"models"`
	Projects                 []ImageProject      `json:"projects"`
	PromptActions            []ImagePromptAction `json:"prompt_actions"`
	BackgroundRemovalEnabled bool                `json:"background_removal_enabled"`
}

type PixianBackgroundRemovalConfig struct {
	Enabled            bool       `json:"enabled"`
	TestMode           bool       `json:"test_mode"`
	EncryptedAPIID     string     `json:"-"`
	EncryptedAPISecret string     `json:"-"`
	APIIDHint          string     `json:"api_id_hint"`
	APISecretHint      string     `json:"api_secret_hint"`
	HasAPIID           bool       `json:"has_api_id"`
	HasAPISecret       bool       `json:"has_api_secret"`
	TimeoutSeconds     int        `json:"timeout_seconds"`
	Concurrency        int        `json:"concurrency"`
	MaxPixels          int        `json:"max_pixels"`
	AccountState       string     `json:"account_state,omitempty"`
	AccountCredits     float64    `json:"account_credits"`
	AccountCheckedAt   *time.Time `json:"account_checked_at,omitempty"`
	UpdatedBy          string     `json:"updated_by,omitempty"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

type BackgroundRemovalJob struct {
	ID                string                  `json:"id"`
	UserID            string                  `json:"user_id"`
	CanvasID          string                  `json:"canvas_id"`
	Status            string                  `json:"status"`
	TestMode          bool                    `json:"test_mode"`
	Attempts          int                     `json:"attempts"`
	NextAttemptAt     time.Time               `json:"next_attempt_at"`
	LockedUntil       *time.Time              `json:"locked_until,omitempty"`
	CompletedCount    int                     `json:"completed_count"`
	FailedCount       int                     `json:"failed_count"`
	CreditsCharged    float64                 `json:"credits_charged"`
	CreditsCalculated float64                 `json:"credits_calculated"`
	Error             string                  `json:"error,omitempty"`
	IdempotencyKey    string                  `json:"-"`
	CreatedAt         time.Time               `json:"created_at"`
	UpdatedAt         time.Time               `json:"updated_at"`
	Items             []BackgroundRemovalItem `json:"items"`
}

type BackgroundRemovalItem struct {
	ID                string    `json:"id"`
	JobID             string    `json:"job_id"`
	SourceNodeID      string    `json:"source_node_id"`
	SourceAssetID     string    `json:"source_asset_id"`
	ProjectID         string    `json:"project_id"`
	PlaceholderNodeID string    `json:"placeholder_node_id"`
	ResultAssetID     string    `json:"result_asset_id,omitempty"`
	Status            string    `json:"status"`
	Attempts          int       `json:"attempts"`
	CreditsCharged    float64   `json:"credits_charged"`
	CreditsCalculated float64   `json:"credits_calculated"`
	InputSize         string    `json:"input_size,omitempty"`
	ResultSize        string    `json:"result_size,omitempty"`
	Error             string    `json:"error,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type BackgroundRemovalStatistics struct {
	TodayCalls         int     `json:"today_calls"`
	ThirtyDaySucceeded int     `json:"thirty_day_succeeded"`
	ThirtyDayFailed    int     `json:"thirty_day_failed"`
	ThirtyDayImages    int     `json:"thirty_day_images"`
	CreditsCharged     float64 `json:"credits_charged"`
	CreditsCalculated  float64 `json:"credits_calculated"`
}

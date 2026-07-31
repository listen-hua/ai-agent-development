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
	ID                string    `json:"id"`
	RelayID           string    `json:"relay_id"`
	ModelID           string    `json:"model_id"`
	DisplayName       string    `json:"display_name"`
	Protocol          string    `json:"protocol"`
	Enabled           bool      `json:"enabled"`
	SupportsReference bool      `json:"supports_reference"`
	SupportsReverse   bool      `json:"supports_reverse"`
	SupportedSizes    []string  `json:"supported_sizes"`
	MaxCount          int       `json:"max_count"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
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
	ID        string            `json:"id"`
	UserID    string            `json:"user_id"`
	ProjectID string            `json:"project_id"`
	Viewport  ImageViewport     `json:"viewport"`
	Version   int64             `json:"version"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
	Nodes     []ImageCanvasNode `json:"nodes,omitempty"`
}

type ImageCanvasNode struct {
	ID          string    `json:"id"`
	CanvasID    string    `json:"canvas_id"`
	AssetID     string    `json:"asset_id,omitempty"`
	JobID       string    `json:"job_id,omitempty"`
	OutputIndex int       `json:"output_index"`
	Status      string    `json:"status"`
	X           float64   `json:"x"`
	Y           float64   `json:"y"`
	Width       float64   `json:"width"`
	Height      float64   `json:"height"`
	ZIndex      int       `json:"z_index"`
	Error       string    `json:"error,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type ImageAsset struct {
	ID        string    `json:"id"`
	OwnerID   string    `json:"owner_id"`
	ProjectID string    `json:"project_id"`
	ObjectKey string    `json:"-"`
	MIMEType  string    `json:"mime_type"`
	FileName  string    `json:"file_name"`
	Width     int       `json:"width"`
	Height    int       `json:"height"`
	SizeBytes int64     `json:"size_bytes"`
	Source    string    `json:"source"`
	CreatedAt time.Time `json:"created_at"`
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
	ID          string    `json:"id"`
	JobID       string    `json:"job_id"`
	OutputIndex int       `json:"output_index"`
	AssetID     string    `json:"asset_id,omitempty"`
	Status      string    `json:"status"`
	Error       string    `json:"error,omitempty"`
	Attempts    int       `json:"attempts"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type ImageAgentOptions struct {
	Relays        []ImageRelay        `json:"relays"`
	Models        []ImageModel        `json:"models"`
	Projects      []ImageProject      `json:"projects"`
	PromptActions []ImagePromptAction `json:"prompt_actions"`
}

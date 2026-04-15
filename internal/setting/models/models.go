package models

import "encoding/json"

type General struct {
	SiteName                    string   `json:"app.site_name"`
	Lang                        string   `json:"app.lang"`
	MaxFileUploadSize           int      `json:"app.max_file_upload_size"`
	FaviconURL                  string   `json:"app.favicon_url"`
	LogoURL                     string   `json:"app.logo_url"`
	RootURL                     string   `json:"app.root_url"`
	AllowedFileUploadExtensions []string `json:"app.allowed_file_upload_extensions"`
	Timezone                    string   `json:"app.timezone"`
	BusinessHoursID             string   `json:"app.business_hours_id"`
	TeamInboxShowAll            bool     `json:"app.team_inbox_show_all"`
}

type EmailNotification struct {
	Username      string `json:"notification.email.username" db:"notification.email.username"`
	Host          string `json:"notification.email.host" db:"notification.email.host"`
	Port          int    `json:"notification.email.port" db:"notification.email.port"`
	Password      string `json:"notification.email.password" db:"notification.email.password"`
	MaxConns      int    `json:"notification.email.max_conns" db:"notification.email.max_conns"`
	IdleTimeout   string `json:"notification.email.idle_timeout" db:"notification.email.idle_timeout"`
	WaitTimeout   string `json:"notification.email.wait_timeout" db:"notification.email.wait_timeout"`
	AuthProtocol  string `json:"notification.email.auth_protocol" db:"notification.email.auth_protocol"`
	EmailAddress  string `json:"notification.email.email_address" db:"notification.email.email_address"`
	MaxMsgRetries int    `json:"notification.email.max_msg_retries" db:"notification.email.max_msg_retries"`
	TLSType       string `json:"notification.email.tls_type" db:"notification.email.tls_type"`
	TLSSkipVerify bool   `json:"notification.email.tls_skip_verify" db:"notification.email.tls_skip_verify"`
	HelloHostname string `json:"notification.email.hello_hostname" db:"notification.email.hello_hostname"`
	Enabled       bool   `json:"notification.email.enabled" db:"notification.email.enabled"`
}

type Settings struct {
	EmailNotification
	General
}

// AISettings holds AI/RAG configuration.
type AISettings struct {
	Enabled                 bool    `json:"ai.enabled" db:"ai.enabled"`
	Provider                string  `json:"ai.provider" db:"ai.provider"`
	OpenAIAPIKey            string  `json:"ai.openai_api_key" db:"ai.openai_api_key"`
	OpenRouterAPIKey        string  `json:"ai.openrouter_api_key" db:"ai.openrouter_api_key"`
	OpenRouterModel         string  `json:"ai.openrouter_model" db:"ai.openrouter_model"`
	EmbeddingModel          string  `json:"ai.embedding_model" db:"ai.embedding_model"`
	SystemPrompt            string  `json:"ai.system_prompt" db:"ai.system_prompt"`
	MaxContextChunks        int     `json:"ai.max_context_chunks" db:"ai.max_context_chunks"`
	SimilarityThreshold     float64 `json:"ai.similarity_threshold" db:"ai.similarity_threshold"`
	ExternalSearchEnabled   bool    `json:"ai.external_search_enabled" db:"ai.external_search_enabled"`
	ExternalSearchURL       string  `json:"ai.external_search_url" db:"ai.external_search_url"`
	ExternalSearchMaxResults int    `json:"ai.external_search_max_results" db:"ai.external_search_max_results"`
	ExternalSearchEndpoints string  `json:"ai.external_search_endpoints" db:"ai.external_search_endpoints"`
	ExternalSearchHeaders   string  `json:"ai.external_search_headers" db:"ai.external_search_headers"`
	TranscriptionEnabled    bool    `json:"ai.transcription_enabled" db:"ai.transcription_enabled"`
	TranscriptionProvider   string  `json:"ai.transcription_provider" db:"ai.transcription_provider"`
}

// InboxAISettings holds per-inbox AI/RAG configuration.
type InboxAISettings struct {
	ID                       int             `db:"id" json:"id"`
	CreatedAt                string          `db:"created_at" json:"created_at"`
	UpdatedAt                string          `db:"updated_at" json:"updated_at"`
	InboxID                  int             `db:"inbox_id" json:"inbox_id"`
	SystemPrompt             string          `db:"system_prompt" json:"system_prompt"`
	MaxContextChunks         int             `db:"max_context_chunks" json:"max_context_chunks"`
	SimilarityThreshold      float64         `db:"similarity_threshold" json:"similarity_threshold"`
	ExternalSearchEnabled    bool            `db:"external_search_enabled" json:"external_search_enabled"`
	ExternalSearchURL        string          `db:"external_search_url" json:"external_search_url"`
	ExternalSearchMaxResults int             `db:"external_search_max_results" json:"external_search_max_results"`
	ExternalSearchEndpoints  string          `db:"external_search_endpoints" json:"external_search_endpoints"`
	ExternalSearchHeaders    string          `db:"external_search_headers" json:"external_search_headers"`
	KnowledgeSourceIDs       json.RawMessage `db:"knowledge_source_ids" json:"knowledge_source_ids"`
}

// TrashSettings holds trash/spam auto-cleanup configuration.
type TrashSettings struct {
	AutoTrashResolvedDays int `json:"trash.auto_trash_resolved_days" db:"trash.auto_trash_resolved_days"`
	AutoTrashSpamDays     int `json:"trash.auto_trash_spam_days" db:"trash.auto_trash_spam_days"`
	AutoDeleteDays        int `json:"trash.auto_delete_days" db:"trash.auto_delete_days"`
	ActivityPurgeDays     int `json:"trash.activity_purge_days" db:"trash.activity_purge_days"`
}

// PCISettings holds PCI data notification configuration.
type PCISettings struct {
	NotifyAgentID  int      `json:"pci.notify_agent_id" db:"pci.notify_agent_id"`
	NotifyMethod   string   `json:"pci.notify_method" db:"pci.notify_method"` // "in_app", "email", "both"
	IgnoreSubjects []string `json:"pci.ignore_subjects" db:"pci.ignore_subjects"`
}

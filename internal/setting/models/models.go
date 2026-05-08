package models

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

// TrashSettings holds trash/spam auto-cleanup retention windows in days.
type TrashSettings struct {
	AutoTrashResolvedDays int `json:"trash.auto_trash_resolved_days" db:"trash.auto_trash_resolved_days"`
	AutoTrashSpamDays     int `json:"trash.auto_trash_spam_days" db:"trash.auto_trash_spam_days"`
	AutoDeleteDays        int `json:"trash.auto_delete_days" db:"trash.auto_delete_days"`
	ActivityPurgeDays     int `json:"trash.activity_purge_days" db:"trash.activity_purge_days"`
}

// PCISettings holds PCI redaction notification configuration. Used by the
// auto-redact loop and manual redact handler to alert an admin when card
// data was successfully scrubbed but the original email could not be
// removed from the IMAP source mailbox (so it can be deleted manually).
//
// NotifyMethod is one of "in_app", "email", or "both"; empty defaults to
// "both" at the call site. NotifyAgentID = 0 disables notifications.
type PCISettings struct {
	NotifyAgentID int    `json:"pci.notify_agent_id" db:"pci.notify_agent_id"`
	NotifyMethod  string `json:"pci.notify_method" db:"pci.notify_method"`
}

// AISettings holds AI feature toggles. Currently scoped to T3v voicemail
// transcription — the rest of v1.0.3's AI surface (RAG, knowledge sources,
// external search, per-inbox prompts) lives behind T3a/T3b which haven't
// been ported yet. New AI fields land here as the relevant tier-3 units
// arrive.
//
// TranscriptionEnabled gates the on-ingest audio-attachment hook in the
// conversation package. TranscriptionProvider selects the backend:
//
//   - "openai" — synchronous Whisper API call via the AI manager's stored
//     OpenAI key. Costs ~$0.006/min of audio.
//   - "local"  — drops a job file into transcribeQueueDir for an external
//     whisper.cpp worker (see transcribe-worker.sh + docs/voice-
//     transcription.md). Free, runs offline, requires host setup.
//
// Empty TranscriptionProvider with TranscriptionEnabled=true is treated as
// "local" by the orchestrator (matches v1.0.3 default).
type AISettings struct {
	TranscriptionEnabled  bool   `json:"ai.transcription_enabled" db:"ai.transcription_enabled"`
	TranscriptionProvider string `json:"ai.transcription_provider" db:"ai.transcription_provider"`
}

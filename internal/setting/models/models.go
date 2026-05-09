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

// AISettings holds AI feature toggles + RAG (Retrieval-Augmented Generation)
// configuration. Originally scoped to T3v voicemail transcription; T3a
// extended it with RAG knobs for the "Generate Response" pipeline.
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
//
// T3a RAG fields:
//
//   - Enabled is a coarse toggle for the RAG feature surface (settings UI,
//     "Generate Response" button) — independent of transcription.
//   - EmbeddingModel selects the OpenAI embeddings model. Defaults to
//     "text-embedding-3-small" (1536 dims, the size the rag_documents.embedding
//     column was built for). Changing this without rebuilding the table will
//     fail at insert time.
//   - SystemPrompt is the template fed to the LLM. Empty string falls back
//     to a built-in default in cmd/rag.go. Supports {{site_name}}, {{context}},
//     {{macros}}, {{enquiry}} substitutions.
//   - MaxContextChunks bounds how many top-similarity rows feed the prompt.
//     0 falls back to 5 in the handler.
//   - SimilarityThreshold filters out low-quality matches (cosine similarity
//     0..1, higher = more similar). 0 or >0.5 falls back to 0.25 — the v1.0.3
//     production-tuned default that catches paraphrased queries while
//     rejecting obviously-unrelated chunks. Tightening past 0.5 risks empty
//     results on legitimate questions.
//
// T3d External-search fields. When ExternalSearchEnabled is true and
// ExternalSearchURL is non-empty, the RAG pipeline runs a two-step
// classify-then-search routine that feeds Meilisearch-compatible API
// results into the prompt as an additional context block via the
// {{external_search_results}} placeholder. All HTTP traffic is routed
// through the SS2 SSRF-guarded http.Client so admin-supplied URLs cannot
// reach private/loopback ranges.
//
//   - ExternalSearchURL is the Meilisearch (or compatible) base URL.
//     Endpoint paths from ExternalSearchEndpoints are appended.
//   - ExternalSearchMaxResults caps hits per endpoint. 0 falls back to 3.
//   - ExternalSearchEndpoints is a JSON object string mapping intent
//     types to endpoint paths, e.g.
//     `{"product": "/indexes/products/search", "category": "...", "faq": "..."}`.
//   - ExternalSearchHeaders is an optional JSON object string of custom
//     headers (e.g. Authorization, Referer, User-Agent overrides).
type AISettings struct {
	TranscriptionEnabled     bool    `json:"ai.transcription_enabled" db:"ai.transcription_enabled"`
	TranscriptionProvider    string  `json:"ai.transcription_provider" db:"ai.transcription_provider"`
	Enabled                  bool    `json:"ai.enabled" db:"ai.enabled"`
	EmbeddingModel           string  `json:"ai.embedding_model" db:"ai.embedding_model"`
	SystemPrompt             string  `json:"ai.system_prompt" db:"ai.system_prompt"`
	MaxContextChunks         int     `json:"ai.max_context_chunks" db:"ai.max_context_chunks"`
	SimilarityThreshold      float64 `json:"ai.similarity_threshold" db:"ai.similarity_threshold"`
	ExternalSearchEnabled    bool    `json:"ai.external_search_enabled" db:"ai.external_search_enabled"`
	ExternalSearchURL        string  `json:"ai.external_search_url" db:"ai.external_search_url"`
	ExternalSearchMaxResults int     `json:"ai.external_search_max_results" db:"ai.external_search_max_results"`
	ExternalSearchEndpoints  string  `json:"ai.external_search_endpoints" db:"ai.external_search_endpoints"`
	ExternalSearchHeaders    string  `json:"ai.external_search_headers" db:"ai.external_search_headers"`
}

// InboxAISettings holds per-inbox AI/RAG configuration (T3h). Mirrors the
// runtime-relevant subset of AISettings — system prompt, RAG tuning,
// external-search config — plus a JSON array of knowledge_source_ids that
// scopes RAG search to a subset of sources for this inbox. When a row
// exists for an inbox it overrides the global AISettings for that inbox's
// conversations; when absent, the RAG pipeline falls back to the global
// settings.
//
// Field-by-field semantics match AISettings; defaults applied at runtime
// (cmd/rag.go) follow the same fallbacks (threshold 0.25, max chunks 5)
// so an admin-saved zero value behaves identically whether scoped to an
// inbox or to the global config.
//
// KnowledgeSourceIDs is a JSON array of rag_sources.id values. Empty
// array (or null) means "search all sources" (matches global behaviour);
// a non-empty array filters the pgvector search via `source_id = ANY`.
// Stored as jsonb so the column can hold an array natively without a
// join table — admins typically pick 1-3 sources per inbox and the set
// is read once per generate-response call.
//
// Transcription fields are deliberately NOT included: voicemail
// transcription is invoked at message ingest before any inbox-scoped
// reasoning happens (it runs unconditionally per the global toggle), and
// the embedding model is chosen at the AI manager layer not the RAG
// handler. Per-inbox prompts/sources are the only knobs that benefit
// from inbox-level overrides today.
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

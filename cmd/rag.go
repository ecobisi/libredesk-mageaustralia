package main

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/abhinavxd/libredesk/internal/envelope"
	"github.com/abhinavxd/libredesk/internal/rag/models"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

// handleGetRAGSources returns all RAG knowledge sources, ordered by
// created_at desc. Surfaces under perm `ai:manage` — admin-only.
func handleGetRAGSources(r *fastglue.Request) error {
	app := r.Context.(*App)
	sources, err := app.rag.GetSources()
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(sources)
}

// handleGetRAGSource returns a single RAG source by ID.
func handleGetRAGSource(r *fastglue.Request) error {
	app := r.Context.(*App)
	id, err := strconv.Atoi(r.RequestCtx.UserValue("id").(string))
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.Ts("globals.messages.invalid", "name", "`id`"), nil, envelope.InputError)
	}
	source, err := app.rag.GetSource(id)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(source)
}

// handleCreateRAGSource creates a new knowledge source. SourceType is
// constrained to one of macro/webpage/custom/file at the SQL CHECK
// constraint; we surface a clean 400 here too so the UI gets a
// readable error envelope instead of a constraint-violation
// stacktrace.
func handleCreateRAGSource(r *fastglue.Request) error {
	app := r.Context.(*App)
	var req struct {
		Name       string          `json:"name"`
		SourceType string          `json:"source_type"`
		Config     json.RawMessage `json:"config"`
		Enabled    bool            `json:"enabled"`
	}
	if err := r.Decode(&req, "json"); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("globals.messages.badRequest"), nil, envelope.InputError)
	}
	if strings.TrimSpace(req.Name) == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.Ts("globals.messages.empty", "name", "name"), nil, envelope.InputError)
	}
	if req.SourceType != "macro" && req.SourceType != "webpage" && req.SourceType != "custom" && req.SourceType != "file" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.Ts("globals.messages.invalid", "name", "source_type"), nil, envelope.InputError)
	}
	source, err := app.rag.CreateSource(req.Name, req.SourceType, req.Config, req.Enabled)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(source)
}

// handleUpdateRAGSource updates a knowledge source. SourceType is
// immutable post-creation (different types use incompatible config
// shapes); the handler accepts only name/config/enabled.
func handleUpdateRAGSource(r *fastglue.Request) error {
	app := r.Context.(*App)
	id, err := strconv.Atoi(r.RequestCtx.UserValue("id").(string))
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.Ts("globals.messages.invalid", "name", "`id`"), nil, envelope.InputError)
	}
	var req struct {
		Name    string          `json:"name"`
		Config  json.RawMessage `json:"config"`
		Enabled bool            `json:"enabled"`
	}
	if err := r.Decode(&req, "json"); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("globals.messages.badRequest"), nil, envelope.InputError)
	}
	source, err := app.rag.UpdateSource(id, req.Name, req.Config, req.Enabled)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(source)
}

// handleDeleteRAGSource deletes a knowledge source. ON DELETE CASCADE
// reaps the indexed documents.
func handleDeleteRAGSource(r *fastglue.Request) error {
	app := r.Context.(*App)
	id, err := strconv.Atoi(r.RequestCtx.UserValue("id").(string))
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.Ts("globals.messages.invalid", "name", "`id`"), nil, envelope.InputError)
	}
	if err := app.rag.DeleteSource(id); err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(true)
}

// handleSyncRAGSource kicks an immediate background sync of one source.
// Returns 200 with `sync_started` immediately — sync may take several
// minutes for a large knowledge base, so we don't block the HTTP
// response. Failures are logged server-side; the UI polls
// last_synced_at to detect completion.
func handleSyncRAGSource(r *fastglue.Request) error {
	app := r.Context.(*App)
	id, err := strconv.Atoi(r.RequestCtx.UserValue("id").(string))
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.Ts("globals.messages.invalid", "name", "`id`"), nil, envelope.InputError)
	}
	go func() {
		if err := app.ragSync.SyncSourceByID(id); err != nil {
			app.lo.Error("error syncing source", "source_id", id, "error", err)
		}
	}()
	return r.SendEnvelope(map[string]string{"status": "sync_started"})
}

// handleRAGSearch is the admin "Test Knowledge Base" search endpoint.
// limit defaults to 5, threshold to 0.25 — values that the v1.0.3
// admin UI ships with. Both are clamped at the manager-call site.
func handleRAGSearch(r *fastglue.Request) error {
	app := r.Context.(*App)
	var req struct {
		Query     string  `json:"query"`
		Limit     int     `json:"limit"`
		Threshold float64 `json:"threshold"`
	}
	if err := r.Decode(&req, "json"); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("globals.messages.badRequest"), nil, envelope.InputError)
	}
	if strings.TrimSpace(req.Query) == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.Ts("globals.messages.empty", "name", "query"), nil, envelope.InputError)
	}
	if req.Limit <= 0 {
		req.Limit = 5
	}
	if req.Threshold <= 0 {
		req.Threshold = 0.25
	}
	results, err := app.rag.Search(req.Query, req.Limit, req.Threshold)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(results)
}

// defaultRAGSystemPrompt is used when ai.system_prompt is empty. Kept
// in cmd/rag.go (not the settings layer) so the admin-saved empty
// string round-trips cleanly. Supports the same {{site_name}} /
// {{context}} / {{macros}} / {{enquiry}} substitutions as a custom
// prompt.
const defaultRAGSystemPrompt = `You are a helpful customer support assistant for {{site_name}}. Use the following knowledge base content to answer questions accurately.

Knowledge Base Context:
{{context}}

Customer Question: {{enquiry}}

Provide a helpful, accurate response based on the context above. If the context doesn't contain relevant information, let the customer know you'll need to check and get back to them.`

// handleRAGGenerateResponse is the agent-facing "Generate Response"
// endpoint. Takes the customer's most recent message (or full
// conversation, the frontend is free to either) and produces an AI-
// drafted reply by:
//
//  1. Reading admin-tuned threshold/max-chunks from settings.
//  2. Embedding the question + cosine-search rag_documents.
//  3. Splitting hits into "context" docs and "macro" docs (macros are
//     formatted differently in the prompt — they're tone references,
//     not factual sources).
//  4. Substituting into the system-prompt template and sending to the
//     default LLM provider via CompletionWithSystemPrompt.
//
// Surfaced under `conversations:write` (T3i may relax further). RAG-
// search failure does NOT fail the whole request: we log + continue
// with empty context so the LLM can still draft a generic reply when
// the knowledge base is empty or pgvector is misconfigured.
func handleRAGGenerateResponse(r *fastglue.Request) error {
	app := r.Context.(*App)
	var req struct {
		ConversationID  int    `json:"conversation_id"`
		CustomerMessage string `json:"customer_message"`
	}
	if err := r.Decode(&req, "json"); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("globals.messages.badRequest"), nil, envelope.InputError)
	}
	if strings.TrimSpace(req.CustomerMessage) == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.Ts("globals.messages.empty", "name", "customer_message"), nil, envelope.InputError)
	}

	aiSettings, err := app.setting.GetAISettings()
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	// Defaults match the v1.0.3 production-tuned values. Threshold
	// is clamped to (0, 0.5] — settings outside that range usually
	// indicate stale data from a previous schema and are ignored.
	threshold := aiSettings.SimilarityThreshold
	if threshold <= 0 || threshold > 0.5 {
		threshold = 0.25
	}
	maxChunks := aiSettings.MaxContextChunks
	if maxChunks <= 0 {
		maxChunks = 5
	}

	results, err := app.rag.Search(req.CustomerMessage, maxChunks, threshold)
	if err != nil {
		app.lo.Warn("RAG search failed, continuing without context", "error", err)
		results = []models.SearchResult{}
	}
	app.lo.Info("RAG generate response", "results_count", len(results), "threshold", threshold)

	var contextParts, macroParts []string
	for _, res := range results {
		if strings.HasPrefix(res.SourceRef, "macro_") {
			macroParts = append(macroParts, "- "+res.Title+": "+res.Content)
		} else {
			contextParts = append(contextParts, "## "+res.Title+"\n"+res.Content)
		}
	}
	contextStr := strings.Join(contextParts, "\n\n")
	macrosStr := strings.Join(macroParts, "\n")

	systemPrompt := aiSettings.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = defaultRAGSystemPrompt
	}
	systemPrompt = strings.ReplaceAll(systemPrompt, "{{site_name}}", ko.String("app.site_name"))
	systemPrompt = strings.ReplaceAll(systemPrompt, "{{context}}", contextStr)
	systemPrompt = strings.ReplaceAll(systemPrompt, "{{macros}}", macrosStr)
	systemPrompt = strings.ReplaceAll(systemPrompt, "{{enquiry}}", req.CustomerMessage)

	response, err := app.ai.CompletionWithSystemPrompt(systemPrompt, req.CustomerMessage)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(map[string]interface{}{
		"response": response,
		"sources":  results,
	})
}

// handleRAGFileUpload handles file uploads for RAG knowledge sources.
// Supported extensions: .txt / .csv / .json. The full file body is
// captured into the source's config JSON (FileConfig.Content) at
// upload time so re-syncs are local — no need to keep the original
// file on disk. After creating the row, kicks an immediate background
// sync so the file is searchable as soon as the embed-calls finish.
func handleRAGFileUpload(r *fastglue.Request) error {
	app := r.Context.(*App)

	form, err := r.RequestCtx.MultipartForm()
	if err != nil {
		app.lo.Error("error parsing form data", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("globals.messages.badRequest"), nil, envelope.InputError)
	}
	files, ok := form.File["file"]
	if !ok || len(files) == 0 {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.Ts("globals.messages.empty", "name", "file"), nil, envelope.InputError)
	}

	fileHeader := files[0]
	file, err := fileHeader.Open()
	if err != nil {
		app.lo.Error("error opening file", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, app.i18n.T("globals.messages.somethingWentWrong"), nil, envelope.GeneralError)
	}
	defer file.Close()

	content := make([]byte, fileHeader.Size)
	if _, err := file.Read(content); err != nil {
		app.lo.Error("error reading file", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, app.i18n.T("globals.messages.somethingWentWrong"), nil, envelope.GeneralError)
	}

	filename := fileHeader.Filename
	var fileType string
	switch {
	case strings.HasSuffix(strings.ToLower(filename), ".txt"):
		fileType = "txt"
	case strings.HasSuffix(strings.ToLower(filename), ".csv"):
		fileType = "csv"
	case strings.HasSuffix(strings.ToLower(filename), ".json"):
		fileType = "json"
	default:
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.Ts("globals.messages.invalid", "name", "file type"), nil, envelope.InputError)
	}

	name := filename
	if names, ok := form.Value["name"]; ok && len(names) > 0 && strings.TrimSpace(names[0]) != "" {
		name = strings.TrimSpace(names[0])
	}

	enabled := true
	if enabledVals, ok := form.Value["enabled"]; ok && len(enabledVals) > 0 {
		enabled = enabledVals[0] == "true"
	}

	configJSON, err := json.Marshal(models.FileConfig{
		Filename: filename,
		FileType: fileType,
		Content:  string(content),
	})
	if err != nil {
		app.lo.Error("error marshaling config", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, app.i18n.T("globals.messages.somethingWentWrong"), nil, envelope.GeneralError)
	}

	source, err := app.rag.CreateSource(name, "file", configJSON, enabled)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	// Immediate sync — file content can be embedded right away (no
	// network fetch like webpages). Background goroutine because the
	// embed-calls themselves are network-bound and may take seconds.
	go func() {
		if err := app.ragSync.SyncSourceByID(source.ID); err != nil {
			app.lo.Error("error syncing file source", "source_id", source.ID, "error", err)
		}
	}()

	return r.SendEnvelope(source)
}

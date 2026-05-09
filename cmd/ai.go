package main

import (
	"strconv"

	"github.com/abhinavxd/libredesk/internal/ai"
	"github.com/abhinavxd/libredesk/internal/envelope"
	settingmodels "github.com/abhinavxd/libredesk/internal/setting/models"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

type aiCompletionReq struct {
	PromptKey string `json:"prompt_key"`
	Content   string `json:"content"`
}

type providerUpdateReq struct {
	Provider string `json:"provider"`
	APIKey   string `json:"api_key"`
	Model    string `json:"model"`
}

// T3b: setDefaultProviderReq is the body for PUT /api/v1/ai/provider/default.
type setDefaultProviderReq struct {
	Provider string `json:"provider"`
}

// T3b: testProviderReq is the body for POST /api/v1/ai/provider/test.
// An empty APIKey means "use the saved one" so admins don't have to
// re-type the key just to verify the connection still works.
type testProviderReq struct {
	Provider string `json:"provider"`
	APIKey   string `json:"api_key"`
	Model    string `json:"model"`
}

// handleAICompletion handles AI completion requests
func handleAICompletion(r *fastglue.Request) error {
	var (
		app = r.Context.(*App)
		req = aiCompletionReq{}
	)

	if err := r.Decode(&req, "json"); err != nil {
		return sendErrorEnvelope(r, envelope.NewError(envelope.InputError, app.i18n.T("errors.parsingRequest"), nil))
	}

	resp, err := app.ai.Completion(req.PromptKey, req.Content)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(resp)
}

// handleGetAIPrompts returns AI prompts
func handleGetAIPrompts(r *fastglue.Request) error {
	var (
		app = r.Context.(*App)
	)
	resp, err := app.ai.GetPrompts()
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(resp)
}

// handleUpdateAIProvider updates the AI provider
func handleUpdateAIProvider(r *fastglue.Request) error {
	var (
		app = r.Context.(*App)
		req providerUpdateReq
	)
	if err := r.Decode(&req, "json"); err != nil {
		return sendErrorEnvelope(r, envelope.NewError(envelope.InputError, app.i18n.T("errors.parsingRequest"), nil))
	}
	if err := app.ai.UpdateProvider(req.Provider, req.APIKey, req.Model); err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope("Provider updated successfully")
}

// T3b: handleGetAIProviders returns the configured AI providers (sans
// API keys). The frontend uses this to render the AISettings page —
// which providers are configured, which is the default, and which
// model OpenRouter is currently using.
func handleGetAIProviders(r *fastglue.Request) error {
	var (
		app = r.Context.(*App)
	)
	resp, err := app.ai.GetProviders()
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(resp)
}

// T3b: handleGetSupportedProviders returns the static catalogue of
// provider types the frontend knows how to render forms for.
func handleGetSupportedProviders(r *fastglue.Request) error {
	return r.SendEnvelope(ai.SupportedProviders)
}

// T3b: handleGetAvailableModels returns the curated OpenRouter model
// list for the AISettings dropdown. Cache-friendly: backed by a 24h
// in-memory cache that fetches from /api/v1/models lazily.
func handleGetAvailableModels(r *fastglue.Request) error {
	var (
		app = r.Context.(*App)
	)
	models := app.ai.GetAvailableModels()
	return r.SendEnvelope(models)
}

// T3b: handleSetDefaultAIProvider flips the is_default flag to the
// named provider. The partial unique index on is_default makes this
// atomic without a wrapper transaction.
func handleSetDefaultAIProvider(r *fastglue.Request) error {
	var (
		app = r.Context.(*App)
		req setDefaultProviderReq
	)
	if err := r.Decode(&req, "json"); err != nil {
		return sendErrorEnvelope(r, envelope.NewError(envelope.InputError, app.i18n.T("errors.parsingRequest"), nil))
	}
	if err := app.ai.SetDefaultProvider(req.Provider); err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope("Default provider updated successfully")
}

// T3b: handleTestAIProvider verifies a provider's credentials by
// sending a tiny prompt through the wire. Empty APIKey in the request
// means "use the saved key" so admins can verify an existing config
// without re-entering the (long) key.
func handleTestAIProvider(r *fastglue.Request) error {
	var (
		app = r.Context.(*App)
		req testProviderReq
	)
	if err := r.Decode(&req, "json"); err != nil {
		return sendErrorEnvelope(r, envelope.NewError(envelope.InputError, app.i18n.T("errors.parsingRequest"), nil))
	}
	if err := app.ai.TestProvider(req.Provider, req.APIKey, req.Model); err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope("Connection successful")
}

// T3h: handleGetInboxAISettings returns AI settings for a specific
// inbox. When the inbox has no override row, returns an empty
// InboxAISettings struct (with id=0, inbox_id set) so the frontend can
// distinguish "no override, show global defaults" from a real saved
// row (id > 0).
func handleGetInboxAISettings(r *fastglue.Request) error {
	app := r.Context.(*App)
	id, err := strconv.Atoi(r.RequestCtx.UserValue("id").(string))
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.Ts("globals.messages.invalid", "name", "`id`"), nil, envelope.InputError)
	}
	out, err := app.setting.GetInboxAISettings(id)
	if err != nil {
		// No row -> empty struct so the frontend renders the
		// global-defaults fallback state. We don't surface the
		// underlying sql.ErrNoRows as a 500 because "no override" is
		// the dominant case.
		return r.SendEnvelope(settingmodels.InboxAISettings{InboxID: id})
	}
	return r.SendEnvelope(out)
}

// T3h: handleUpdateInboxAISettings creates or updates AI settings for
// an inbox. Numeric ranges are validated to match handleUpdateAISettings
// (see settings.go) so the per-inbox surface is no laxer than the
// global form.
func handleUpdateInboxAISettings(r *fastglue.Request) error {
	app := r.Context.(*App)
	id, err := strconv.Atoi(r.RequestCtx.UserValue("id").(string))
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.Ts("globals.messages.invalid", "name", "`id`"), nil, envelope.InputError)
	}

	var req settingmodels.InboxAISettings
	if err := r.Decode(&req, "json"); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("globals.messages.badRequest"), nil, envelope.InputError)
	}
	req.InboxID = id

	// Mirror the global handleUpdateAISettings range checks. Per-inbox
	// rows feed the same RAG runtime path, so any value the global
	// form would reject is also nonsense here.
	if req.MaxContextChunks < 0 || req.MaxContextChunks > 50 {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "max_context_chunks must be between 0 and 50", nil, envelope.InputError)
	}
	if req.SimilarityThreshold < 0 || req.SimilarityThreshold > 1 {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "similarity_threshold must be between 0 and 1", nil, envelope.InputError)
	}
	if req.ExternalSearchMaxResults < 0 || req.ExternalSearchMaxResults > 10 {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "external_search_max_results must be between 0 and 10", nil, envelope.InputError)
	}

	out, err := app.setting.UpsertInboxAISettings(req)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(out)
}

// T3h: handleDeleteInboxAISettings removes a per-inbox AI settings row
// so the RAG pipeline falls back to global settings on the next
// generate call.
func handleDeleteInboxAISettings(r *fastglue.Request) error {
	app := r.Context.(*App)
	id, err := strconv.Atoi(r.RequestCtx.UserValue("id").(string))
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.Ts("globals.messages.invalid", "name", "`id`"), nil, envelope.InputError)
	}
	if err := app.setting.DeleteInboxAISettings(id); err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(true)
}

package main

import (
	"github.com/abhinavxd/libredesk/internal/ai"
	"github.com/abhinavxd/libredesk/internal/envelope"
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

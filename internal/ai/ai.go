// Package ai manages AI prompts and integrates with LLM providers.
package ai

import (
	"database/sql"
	"embed"
	"encoding/json"
	"errors"

	"github.com/abhinavxd/libredesk/internal/ai/models"
	"github.com/abhinavxd/libredesk/internal/crypto"
	"github.com/abhinavxd/libredesk/internal/dbutil"
	"github.com/abhinavxd/libredesk/internal/envelope"
	"github.com/jmoiron/sqlx"
	"github.com/knadh/go-i18n"
	"github.com/zerodha/logf"
)

var (
	//go:embed queries.sql
	efs embed.FS

	ErrInvalidAPIKey = errors.New("invalid API Key")
	ErrApiKeyNotSet  = errors.New("api Key not set")
)

type Manager struct {
	q             queries
	lo            *logf.Logger
	i18n          *i18n.I18n
	encryptionKey string
}

// Opts contains options for initializing the Manager.
type Opts struct {
	DB            *sqlx.DB
	I18n          *i18n.I18n
	Lo            *logf.Logger
	EncryptionKey string
}

// queries contains prepared SQL queries.
type queries struct {
	GetDefaultProvider  *sqlx.Stmt `query:"get-default-provider"`
	GetPrompt           *sqlx.Stmt `query:"get-prompt"`
	GetPrompts          *sqlx.Stmt `query:"get-prompts"`
	SetOpenAIKey        *sqlx.Stmt `query:"set-openai-key"`
	GetProvider         *sqlx.Stmt `query:"get-provider"`
	GetProviders        *sqlx.Stmt `query:"get-providers"`
	SetDefaultProvider  *sqlx.Stmt `query:"set-default-provider"`
	UpsertOpenRouter    *sqlx.Stmt `query:"upsert-openrouter"`
	SetOpenRouterConfig *sqlx.Stmt `query:"set-openrouter-config"`
}

// New creates and returns a new instance of the Manager.
func New(opts Opts) (*Manager, error) {
	var q queries
	if err := dbutil.ScanSQLFile("queries.sql", &q, opts.DB, efs); err != nil {
		return nil, err
	}
	return &Manager{
		q:             q,
		lo:            opts.Lo,
		i18n:          opts.I18n,
		encryptionKey: opts.EncryptionKey,
	}, nil
}

// Completion sends a prompt to the default provider and returns the response.
func (m *Manager) Completion(k string, prompt string) (string, error) {
	systemPrompt, err := m.getPrompt(k)
	if err != nil {
		return "", err
	}

	client, providerName, err := m.getDefaultProviderClient()
	if err != nil {
		m.lo.Error("error getting provider client", "error", err)
		return "", envelope.NewError(envelope.GeneralError, m.i18n.T("globals.messages.somethingWentWrong"), nil)
	}

	payload := PromptPayload{
		SystemPrompt: systemPrompt,
		UserPrompt:   prompt,
	}

	response, err := client.SendPrompt(payload)
	if err != nil {
		if errors.Is(err, ErrInvalidAPIKey) {
			m.lo.Error("error invalid API key", "error", err)
			return "", envelope.NewError(envelope.InputError, m.i18n.T("globals.messages.somethingWentWrong"), nil)
		}
		if errors.Is(err, ErrApiKeyNotSet) {
			m.lo.Error("error API key not set", "error", err)
			return "", envelope.NewError(envelope.InputError, m.i18n.Ts("ai.apiKeyNotSet", "provider", providerName), nil)
		}
		m.lo.Error("error sending prompt to provider", "error", err)
		return "", envelope.NewError(envelope.GeneralError, err.Error(), nil)
	}

	return response, nil
}

// GetPrompts returns a list of prompts from the database.
func (m *Manager) GetPrompts() ([]models.Prompt, error) {
	var prompts = make([]models.Prompt, 0)
	if err := m.q.GetPrompts.Select(&prompts); err != nil {
		m.lo.Error("error fetching prompts", "error", err)
		return nil, envelope.NewError(envelope.GeneralError, m.i18n.T("globals.messages.somethingWentWrong"), nil)
	}
	return prompts, nil
}

// GetProviders returns information about all configured providers for the
// admin UI. The api_key itself never leaves the server — the frontend gets
// only the boolean has_api_key + the (non-secret) model selection.
func (m *Manager) GetProviders() ([]ProviderInfo, error) {
	var providers = make([]models.Provider, 0)
	if err := m.q.GetProviders.Select(&providers); err != nil {
		m.lo.Error("error fetching providers", "error", err)
		return nil, envelope.NewError(envelope.GeneralError, m.i18n.T("globals.messages.somethingWentWrong"), nil)
	}

	result := make([]ProviderInfo, 0, len(providers))
	for _, p := range providers {
		info := ProviderInfo{
			Provider:  p.Provider,
			Name:      p.Name,
			IsDefault: p.IsDefault,
		}
		var config map[string]interface{}
		if err := json.Unmarshal([]byte(p.Config), &config); err == nil {
			if apiKey, ok := config["api_key"].(string); ok && apiKey != "" {
				info.HasAPIKey = true
			}
			if model, ok := config["model"].(string); ok {
				info.Model = model
			}
		}
		result = append(result, info)
	}
	return result, nil
}

// GetAvailableModels returns a curated list of OpenRouter models for the
// AISettings dropdown. Errors are swallowed at the lower level (network
// failure → fallback list), so callers always get something.
func (m *Manager) GetAvailableModels() []string {
	models, _ := FetchOpenRouterModels()
	return models
}

// UpdateProvider updates a provider's stored config.
//
// For OpenAI: only apiKey is consumed (model is fixed at gpt-4o-mini).
// For OpenRouter: both apiKey and model are persisted; an empty apiKey
// preserves the existing one so admins can change models without
// re-entering the key.
func (m *Manager) UpdateProvider(provider, apiKey, model string) error {
	switch ProviderType(provider) {
	case ProviderOpenAI:
		return m.setOpenAIAPIKey(apiKey)
	case ProviderOpenRouter:
		return m.setOpenRouterConfig(apiKey, model)
	default:
		m.lo.Error("unsupported provider type", "provider", provider)
		return envelope.NewError(envelope.GeneralError, m.i18n.T("validation.invalidProvider"), nil)
	}
}

// SetDefaultProvider marks the named provider as the default and clears
// the flag on the others. The partial unique index on is_default makes
// the multi-row UPDATE atomic without a wrapper transaction.
func (m *Manager) SetDefaultProvider(provider string) error {
	if _, err := m.q.SetDefaultProvider.Exec(provider); err != nil {
		m.lo.Error("error setting default provider", "error", err)
		return envelope.NewError(envelope.GeneralError, m.i18n.T("globals.messages.somethingWentWrong"), nil)
	}
	return nil
}

// TestProvider sends a tiny prompt through the named provider and
// returns whether the round-trip worked. Used by the AISettings "Test
// Connection" button.
//
// If apiKey is empty, the saved key is loaded from the database — lets
// the admin test an existing config without re-typing the (potentially
// long) key.
func (m *Manager) TestProvider(provider, apiKey, model string) error {
	if apiKey == "" {
		savedKey, savedModel := m.getSavedAPIKey(provider)
		apiKey = savedKey
		if model == "" {
			model = savedModel
		}
	}

	var client ProviderClient
	switch ProviderType(provider) {
	case ProviderOpenAI:
		client = NewOpenAIClient(apiKey, m.lo)
	case ProviderOpenRouter:
		client = NewOpenRouterClient(apiKey, model, m.lo)
	default:
		return envelope.NewError(envelope.InputError, m.i18n.T("validation.invalidProvider"), nil)
	}

	_, err := client.SendPrompt(PromptPayload{
		SystemPrompt: "You are a helpful assistant.",
		UserPrompt:   "Say OK to confirm the connection works.",
	})
	if err != nil {
		if errors.Is(err, ErrInvalidAPIKey) {
			return envelope.NewError(envelope.InputError, m.i18n.Ts("globals.messages.invalid", "name", "API Key"), nil)
		}
		if errors.Is(err, ErrApiKeyNotSet) {
			return envelope.NewError(envelope.InputError, m.i18n.Ts("ai.apiKeyNotSet", "provider", provider), nil)
		}
		return envelope.NewError(envelope.GeneralError, err.Error(), nil)
	}
	return nil
}

// getSavedAPIKey reads the currently-stored api_key + model for a
// provider from ai_providers. Used by TestProvider when the admin doesn't
// re-enter the key in the form. Returns ("", "") on any error — TestProvider
// then falls through to ErrApiKeyNotSet which the caller surfaces cleanly.
//
// Both OpenAI and OpenRouter store the api_key encrypted at rest; decryption
// happens here so the test reflects what a real call would see. Decryption
// failure (typically a changed app.encryption_key, or a legacy plaintext row
// from before T3j) bails out as ("", "") so the admin sees ErrApiKeyNotSet
// and re-saves the key, which writes a fresh encrypted value.
func (m *Manager) getSavedAPIKey(provider string) (string, string) {
	rows, err := m.q.GetProviders.Queryx()
	if err != nil {
		return "", ""
	}
	defer rows.Close()

	for rows.Next() {
		var p models.Provider
		if err := rows.StructScan(&p); err != nil {
			continue
		}
		if p.Provider != provider {
			continue
		}
		var config map[string]interface{}
		if err := json.Unmarshal([]byte(p.Config), &config); err != nil {
			return "", ""
		}
		apiKey, _ := config["api_key"].(string)
		model, _ := config["model"].(string)
		// Both providers store api_key encrypted at rest.
		if apiKey != "" {
			switch ProviderType(provider) {
			case ProviderOpenAI, ProviderOpenRouter:
				if dec, err := crypto.Decrypt(apiKey, m.encryptionKey); err == nil {
					apiKey = dec
				} else {
					return "", ""
				}
			}
		}
		return apiKey, model
	}
	return "", ""
}

// setOpenAIAPIKey sets the OpenAI API key in the database.
func (m *Manager) setOpenAIAPIKey(apiKey string) error {
	// Encrypt API key before storing.
	encryptedKey, err := crypto.Encrypt(apiKey, m.encryptionKey)
	if err != nil {
		m.lo.Error("error encrypting API key", "error", err)
		return envelope.NewError(envelope.GeneralError, m.i18n.T("globals.messages.somethingWentWrong"), nil)
	}

	if _, err := m.q.SetOpenAIKey.Exec(encryptedKey); err != nil {
		m.lo.Error("error setting OpenAI API key", "error", err)
		return envelope.NewError(envelope.GeneralError, m.i18n.T("globals.messages.somethingWentWrong"), nil)
	}
	return nil
}

// setOpenRouterConfig persists OpenRouter's API key + model selection.
//
// Two-step write because schema.sql + the v2.2.16 migration both seed an
// OpenRouter row, but extra-cautious idempotency: UpsertOpenRouter is a
// no-op when the row exists, then SetOpenRouterConfig actually writes
// the values. An empty apiKey preserves whatever's already stored — lets
// admins change just the model from the UI.
//
// API key is encrypted at rest via crypto.Encrypt (T3j) — same path as
// the OpenAI provider — so a stolen DB dump alone doesn't surface bearer
// tokens. The empty-key sentinel ("preserve existing") happens BEFORE
// encryption so the SQL CASE in set-openrouter-config still sees `''`.
func (m *Manager) setOpenRouterConfig(apiKey, model string) error {
	if model == "" {
		model = defaultOpenRouterModel
	}

	// Encrypt the API key before storing. Empty string is preserved as
	// the "keep existing key" sentinel that set-openrouter-config's
	// CASE WHEN $1::text = '' branch keys off — encrypting "" would
	// produce non-empty ciphertext and clobber the saved key.
	if apiKey != "" {
		encryptedKey, err := crypto.Encrypt(apiKey, m.encryptionKey)
		if err != nil {
			m.lo.Error("error encrypting OpenRouter API key", "error", err)
			return envelope.NewError(envelope.GeneralError, m.i18n.T("globals.messages.somethingWentWrong"), nil)
		}
		apiKey = encryptedKey
	}

	if _, err := m.q.UpsertOpenRouter.Exec(); err != nil {
		m.lo.Error("error upserting OpenRouter provider", "error", err)
		return envelope.NewError(envelope.GeneralError, m.i18n.T("globals.messages.somethingWentWrong"), nil)
	}

	if _, err := m.q.SetOpenRouterConfig.Exec(apiKey, model); err != nil {
		m.lo.Error("error setting OpenRouter config", "error", err)
		return envelope.NewError(envelope.GeneralError, m.i18n.T("globals.messages.somethingWentWrong"), nil)
	}
	return nil
}

// getPrompt returns a prompt from the database.
func (m *Manager) getPrompt(k string) (string, error) {
	var p models.Prompt
	if err := m.q.GetPrompt.Get(&p, k); err != nil {
		if err == sql.ErrNoRows {
			m.lo.Error("error prompt not found", "key", k)
			return "", envelope.NewError(envelope.InputError, m.i18n.T("validation.notFoundTemplate"), nil)
		}
		m.lo.Error("error fetching prompt", "error", err)
		return "", envelope.NewError(envelope.GeneralError, m.i18n.T("globals.messages.somethingWentWrong"), nil)
	}
	return p.Content, nil
}

// GetOpenAIClient returns an OpenAI client built from the stored provider
// config, or nil if OpenAI isn't configured (no row, no api_key set, or the
// stored key fails to decrypt). Used by T3v's voicemail-transcription
// pipeline, which needs the Whisper API endpoint specifically — Whisper
// only ships on OpenAI proper, not via OpenRouter — so the orchestration
// layer reaches for OpenAI directly rather than going through the default
// provider client. nil-return is intentionally unsurfaced to callers (no
// envelope error): the conversation manager treats a missing client as
// "transcription disabled" and logs once.
func (m *Manager) GetOpenAIClient() *OpenAIClient {
	var p models.Provider
	if err := m.q.GetDefaultProvider.Get(&p); err != nil {
		return nil
	}
	if ProviderType(p.Provider) != ProviderOpenAI {
		return nil
	}
	var config struct {
		APIKey string `json:"api_key"`
	}
	if err := json.Unmarshal([]byte(p.Config), &config); err != nil || config.APIKey == "" {
		return nil
	}
	decryptedKey, err := crypto.Decrypt(config.APIKey, m.encryptionKey)
	if err != nil {
		return nil
	}
	return NewOpenAIClient(decryptedKey, m.lo)
}

// GenerateEmbedding generates an OpenAI text-embedding-3-small embedding
// for the given text. Always reaches for the stored OpenAI provider —
// embeddings are an OpenAI-proper API, not available via OpenRouter or
// other LLM relays — so this bypasses the default-provider machinery.
//
// Wired in cmd/init.go as the rag.Manager's EmbeddingFunc. Returns a
// clean error envelope if no OpenAI provider row exists or its api_key
// is blank/un-decryptable, matching the error-surface conventions used
// by the rest of the AI package.
func (m *Manager) GenerateEmbedding(text string) ([]float32, error) {
	var p models.Provider
	if err := m.q.GetProvider.Get(&p, string(ProviderOpenAI)); err != nil {
		if err == sql.ErrNoRows {
			return nil, envelope.NewError(envelope.InputError, "OpenAI API key required for embeddings", nil)
		}
		m.lo.Error("error fetching OpenAI provider", "error", err)
		return nil, envelope.NewError(envelope.GeneralError, m.i18n.Ts("globals.messages.errorFetching", "name", "OpenAI provider"), nil)
	}

	var config struct {
		APIKey string `json:"api_key"`
	}
	if err := json.Unmarshal([]byte(p.Config), &config); err != nil {
		m.lo.Error("error parsing OpenAI config", "error", err)
		return nil, envelope.NewError(envelope.GeneralError, "Error parsing OpenAI config", nil)
	}
	if config.APIKey == "" {
		return nil, envelope.NewError(envelope.InputError, "OpenAI API key required for embeddings", nil)
	}

	// Decrypt — provider api_key is encrypted at rest (same path as
	// getDefaultProviderClient). Decryption failure usually means a
	// changed app.encryption_key; surface as a generic error so the
	// admin re-saves the key.
	decryptedKey, err := crypto.Decrypt(config.APIKey, m.encryptionKey)
	if err != nil {
		m.lo.Error("error decrypting OpenAI API key", "error", err)
		return nil, envelope.NewError(envelope.GeneralError, m.i18n.T("globals.messages.somethingWentWrong"), nil)
	}

	client := NewOpenAIClient(decryptedKey, m.lo)
	return client.GenerateEmbedding(text)
}

// CompletionWithSystemPrompt sends a prompt with a custom (caller-supplied)
// system prompt to the default provider. The base Completion method looks
// the system prompt up by key from ai_prompts; the RAG pipeline assembles
// the prompt at call time from the search results + user question + the
// admin-configured template, so it needs a way to bypass the prompt
// table.
func (m *Manager) CompletionWithSystemPrompt(systemPrompt, userPrompt string) (string, error) {
	return m.CompletionWithPayload(PromptPayload{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
	})
}

// CompletionWithPayload sends a fully-assembled PromptPayload (which
// may include multimodal Images) to the default provider. T3e — RAG
// generate calls this when conversation attachments yielded resized
// images so the LLM can examine the screenshots the customer sent.
//
// Error envelope shape mirrors CompletionWithSystemPrompt — the two
// share their failure modes (provider lookup, decryption, key, network)
// and an admin re-saving the API key fixes either path. Kept as the
// "real" implementation; CompletionWithSystemPrompt now delegates so
// the text-only call path remains a one-liner.
func (m *Manager) CompletionWithPayload(payload PromptPayload) (string, error) {
	client, providerName, err := m.getDefaultProviderClient()
	if err != nil {
		m.lo.Error("error getting provider client", "error", err)
		return "", err
	}

	response, err := client.SendPrompt(payload)
	if err != nil {
		if errors.Is(err, ErrInvalidAPIKey) {
			m.lo.Error("error invalid API key", "error", err)
			return "", envelope.NewError(envelope.InputError, m.i18n.T("globals.messages.somethingWentWrong"), nil)
		}
		if errors.Is(err, ErrApiKeyNotSet) {
			m.lo.Error("error API key not set", "error", err)
			return "", envelope.NewError(envelope.InputError, m.i18n.Ts("ai.apiKeyNotSet", "provider", providerName), nil)
		}
		m.lo.Error("error sending prompt to provider", "error", err)
		return "", envelope.NewError(envelope.GeneralError, err.Error(), nil)
	}
	return response, nil
}

// providerDisplayName returns the human-readable name for a provider key
// ("openai" → "OpenAI", "openrouter" → "OpenRouter"). Falls back to the
// generic "AI provider" label so error messages stay sensible if a new
// provider is added without updating this map.
func providerDisplayName(p ProviderType) string {
	for _, info := range SupportedProviders {
		if info.Provider == string(p) {
			return info.Name
		}
	}
	return "AI provider"
}

// getDefaultProviderClient returns a ProviderClient for the default provider
// along with its human-readable display name (so callers can surface an
// accurate error message when the provider's API key is missing/invalid).
func (m *Manager) getDefaultProviderClient() (ProviderClient, string, error) {
	var p models.Provider

	if err := m.q.GetDefaultProvider.Get(&p); err != nil {
		m.lo.Error("error fetching provider details", "error", err)
		return nil, "", envelope.NewError(envelope.GeneralError, m.i18n.T("globals.messages.somethingWentWrong"), nil)
	}

	displayName := providerDisplayName(ProviderType(p.Provider))

	switch ProviderType(p.Provider) {
	case ProviderOpenAI:
		config := struct {
			APIKey string `json:"api_key"`
		}{}
		if err := json.Unmarshal([]byte(p.Config), &config); err != nil {
			m.lo.Error("error parsing provider config", "error", err)
			return nil, displayName, envelope.NewError(envelope.GeneralError, m.i18n.T("globals.messages.somethingWentWrong"), nil)
		}
		// Decrypt API key.
		decryptedKey, err := crypto.Decrypt(config.APIKey, m.encryptionKey)
		if err != nil {
			m.lo.Error("error decrypting API key", "error", err)
			return nil, displayName, envelope.NewError(envelope.GeneralError, m.i18n.T("globals.messages.somethingWentWrong"), nil)
		}
		return NewOpenAIClient(decryptedKey, m.lo), displayName, nil
	case ProviderOpenRouter:
		config := struct {
			APIKey string `json:"api_key"`
			Model  string `json:"model"`
		}{}
		if err := json.Unmarshal([]byte(p.Config), &config); err != nil {
			m.lo.Error("error parsing provider config", "error", err)
			return nil, displayName, envelope.NewError(envelope.GeneralError, m.i18n.T("globals.messages.somethingWentWrong"), nil)
		}
		// Decrypt the API key (encrypted at rest since T3j, mirroring
		// the OpenAI path). An empty key isn't fatal: the client
		// returns ErrApiKeyNotSet on first SendPrompt which the manager
		// surfaces as a clean "configure AI in settings" error — so
		// skip the decrypt step entirely when the field is empty
		// (Decrypt would otherwise return a "ciphertext too short"
		// error that the admin can't act on).
		apiKey := config.APIKey
		if apiKey != "" {
			decryptedKey, err := crypto.Decrypt(apiKey, m.encryptionKey)
			if err != nil {
				m.lo.Error("error decrypting OpenRouter API key", "error", err)
				return nil, displayName, envelope.NewError(envelope.GeneralError, m.i18n.T("globals.messages.somethingWentWrong"), nil)
			}
			apiKey = decryptedKey
		}
		return NewOpenRouterClient(apiKey, config.Model, m.lo), displayName, nil
	default:
		m.lo.Error("unsupported provider type", "provider", p.Provider)
		return nil, displayName, envelope.NewError(envelope.GeneralError, m.i18n.T("validation.invalidProvider"), nil)
	}
}

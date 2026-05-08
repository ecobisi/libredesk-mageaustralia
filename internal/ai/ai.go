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
	GetDefaultProvider *sqlx.Stmt `query:"get-default-provider"`
	GetPrompt          *sqlx.Stmt `query:"get-prompt"`
	GetPrompts         *sqlx.Stmt `query:"get-prompts"`
	SetOpenAIKey       *sqlx.Stmt `query:"set-openai-key"`
	GetProvider        *sqlx.Stmt `query:"get-provider"`
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

	client, err := m.getDefaultProviderClient()
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
			return "", envelope.NewError(envelope.InputError, m.i18n.Ts("ai.apiKeyNotSet", "provider", "OpenAI"), nil)
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

// UpdateProvider updates a provider.
func (m *Manager) UpdateProvider(provider, apiKey string) error {
	switch ProviderType(provider) {
	case ProviderOpenAI:
		return m.setOpenAIAPIKey(apiKey)
	default:
		m.lo.Error("unsupported provider type", "provider", provider)
		return envelope.NewError(envelope.GeneralError, m.i18n.T("validation.invalidProvider"), nil)
	}
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
	client, err := m.getDefaultProviderClient()
	if err != nil {
		m.lo.Error("error getting provider client", "error", err)
		return "", err
	}

	response, err := client.SendPrompt(PromptPayload{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
	})
	if err != nil {
		if errors.Is(err, ErrInvalidAPIKey) {
			m.lo.Error("error invalid API key", "error", err)
			return "", envelope.NewError(envelope.InputError, m.i18n.T("globals.messages.somethingWentWrong"), nil)
		}
		if errors.Is(err, ErrApiKeyNotSet) {
			m.lo.Error("error API key not set", "error", err)
			return "", envelope.NewError(envelope.InputError, m.i18n.Ts("ai.apiKeyNotSet", "provider", "AI Provider"), nil)
		}
		m.lo.Error("error sending prompt to provider", "error", err)
		return "", envelope.NewError(envelope.GeneralError, err.Error(), nil)
	}
	return response, nil
}

// getDefaultProviderClient returns a ProviderClient for the default provider.
func (m *Manager) getDefaultProviderClient() (ProviderClient, error) {
	var p models.Provider

	if err := m.q.GetDefaultProvider.Get(&p); err != nil {
		m.lo.Error("error fetching provider details", "error", err)
		return nil, envelope.NewError(envelope.GeneralError, m.i18n.T("globals.messages.somethingWentWrong"), nil)
	}

	switch ProviderType(p.Provider) {
	case ProviderOpenAI:
		config := struct {
			APIKey string `json:"api_key"`
		}{}
		if err := json.Unmarshal([]byte(p.Config), &config); err != nil {
			m.lo.Error("error parsing provider config", "error", err)
			return nil, envelope.NewError(envelope.GeneralError, m.i18n.T("globals.messages.somethingWentWrong"), nil)
		}
		// Decrypt API key.
		decryptedKey, err := crypto.Decrypt(config.APIKey, m.encryptionKey)
		if err != nil {
			m.lo.Error("error decrypting API key", "error", err)
			return nil, envelope.NewError(envelope.GeneralError, m.i18n.T("globals.messages.somethingWentWrong"), nil)
		}
		return NewOpenAIClient(decryptedKey, m.lo), nil
	default:
		m.lo.Error("unsupported provider type", "provider", p.Provider)
		return nil, envelope.NewError(envelope.GeneralError, m.i18n.T("validation.invalidProvider"), nil)
	}
}

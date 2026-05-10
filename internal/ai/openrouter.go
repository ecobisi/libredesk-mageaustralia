// T3b: OpenRouter provider client.
//
// OpenRouter is a unified API gateway to 100+ models from Anthropic, OpenAI,
// Google, xAI, Meta, etc. The wire format is OpenAI-compatible with two
// extra headers (HTTP-Referer + X-Title) that OpenRouter uses for usage
// attribution on its dashboard.
//
// The client implements the same ProviderClient.SendPrompt interface as
// OpenAIClient so the rest of the AI/RAG pipeline doesn't care which
// provider is selected — getDefaultProviderClient swaps in the right one
// based on the ai_providers row that has is_default = true.
//
// API key is encrypted at rest via crypto.Encrypt/Decrypt (T3j).
package ai

import (
	"net/http"
	"time"

	"github.com/zerodha/logf"
)

// OpenRouterClient implements ProviderClient for the OpenRouter API.
type OpenRouterClient struct {
	apiKey string
	model  string
	lo     *logf.Logger
	client *http.Client
}

// NewOpenRouterClient creates a new OpenRouter client.
//
// 60s budget to mirror the OpenAI chat client (T3f) — assembled RAG prompts
// (knowledge base context + customer message) push the model well past the
// 10s-default ceiling under normal load.
func NewOpenRouterClient(apiKey, model string, lo *logf.Logger) *OpenRouterClient {
	if model == "" {
		model = defaultOpenRouterModel
	}
	return &OpenRouterClient{
		apiKey: apiKey,
		model:  model,
		lo:     lo,
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

// SendPrompt sends a prompt to OpenRouter and returns the response text.
// OpenRouter is OpenAI-compatible (T3b note (i)) so the request shape +
// response decoding lives in the shared doChatCompletion helper. Two
// extra headers (HTTP-Referer + X-Title) are OpenRouter-specific
// usage-attribution metadata — without them the install shows up as
// anonymous traffic on the OpenRouter dashboard.
func (o *OpenRouterClient) SendPrompt(payload PromptPayload) (string, error) {
	return doChatCompletion(o.client, o.lo, chatCompletionRequest{
		URL:         "https://openrouter.ai/api/v1/chat/completions",
		APIKey:      o.apiKey,
		Model:       o.model,
		Payload:     payload,
		MaxTokens:   1024,
		Temperature: 0.7,
		ExtraHeaders: map[string]string{
			"HTTP-Referer": "https://libredesk.io",
			"X-Title":      "Libredesk",
		},
		LogTag: "OpenRouter API",
	})
}

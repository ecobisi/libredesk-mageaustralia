package ai

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// ProviderClient is the interface all providers should implement.
type ProviderClient interface {
	SendPrompt(payload PromptPayload) (string, error)
}

// ProviderType is an enum-like type for different providers.
type ProviderType string

const (
	ProviderOpenAI     ProviderType = "openai"
	ProviderClaude     ProviderType = "claude"
	ProviderOpenRouter ProviderType = "openrouter"
)

// defaultOpenRouterModel is the fallback model when none is set on the
// ai_providers row. Claude Haiku 3 is the cheapest "good enough" choice
// for an out-of-the-box experience; admins pick their preferred model
// from the AISettings UI dropdown.
const defaultOpenRouterModel = "anthropic/claude-3-haiku"

// PromptPayload represents the structured input for an LLM provider.
type PromptPayload struct {
	SystemPrompt string `json:"system_prompt"`
	UserPrompt   string `json:"user_prompt"`
}

// ProviderInfo is the public-facing shape returned to the admin UI for
// the AISettings page. HasAPIKey is computed from the stored config so the
// frontend can surface a "Configured" badge without ever receiving the key
// itself.
type ProviderInfo struct {
	Provider  string `json:"provider"`
	Name      string `json:"name"`
	Model     string `json:"model,omitempty"`
	HasAPIKey bool   `json:"has_api_key"`
	IsDefault bool   `json:"is_default"`
}

// SupportedProviders is the static list of provider types the admin UI
// can offer. Surfaced via GET /api/v1/ai/providers/supported so the
// frontend doesn't need to hardcode the list.
var SupportedProviders = []ProviderInfo{
	{Provider: string(ProviderOpenAI), Name: "OpenAI", Model: "gpt-4o-mini"},
	{Provider: string(ProviderOpenRouter), Name: "OpenRouter", Model: defaultOpenRouterModel},
}

// Model cache for the OpenRouter /models endpoint. 24h TTL — model
// catalogues change slowly, and we'd rather serve a stale list than
// hit the network on every admin page load.
var (
	modelCache      []string
	modelCacheMutex sync.RWMutex
	modelCacheTime  time.Time
	modelCacheTTL   = 24 * time.Hour
)

// OpenRouterModel is one item of the OpenRouter /api/v1/models response.
type OpenRouterModel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// OpenRouterModelsResponse is the wrapper for the /models endpoint.
type OpenRouterModelsResponse struct {
	Data []OpenRouterModel `json:"data"`
}

// providerConfig defines how many models to take from each provider when
// balancing the OpenRouter catalogue.
type providerConfig struct {
	prefix   string
	maxCount int
}

// preferredProviders defines the order and quota for each provider in the
// balanced model list. The dropdown would otherwise be dominated by
// whichever provider has the most variants on OpenRouter at the moment.
var preferredProviders = []providerConfig{
	{"anthropic/", 8},
	{"openai/", 10},
	{"google/", 6},
	{"x-ai/", 4},
	{"moonshotai/", 3},
	{"z-ai/", 4},
	{"deepseek/", 5},
	{"qwen/", 5},
	{"meta-llama/", 5},
	{"mistralai/", 5},
}

// FetchOpenRouterModels returns a curated list of OpenRouter model IDs,
// fetched live from /api/v1/models with a 24h cache. Falls back to a
// static list if the network call fails so the admin UI still shows
// reasonable choices when offline.
func FetchOpenRouterModels() ([]string, error) {
	modelCacheMutex.RLock()
	if len(modelCache) > 0 && time.Since(modelCacheTime) < modelCacheTTL {
		defer modelCacheMutex.RUnlock()
		return modelCache, nil
	}
	modelCacheMutex.RUnlock()

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get("https://openrouter.ai/api/v1/models")
	if err != nil {
		return getFallbackModels(), nil
	}
	defer resp.Body.Close()

	var result OpenRouterModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return getFallbackModels(), nil
	}

	models := balanceModelsAcrossProviders(result.Data)

	modelCacheMutex.Lock()
	modelCache = models
	modelCacheTime = time.Now()
	modelCacheMutex.Unlock()

	return models, nil
}

// balanceModelsAcrossProviders takes a quota-limited slice of model IDs
// per provider so the dropdown isn't 80% one vendor.
func balanceModelsAcrossProviders(data []OpenRouterModel) []string {
	providerModels := make(map[string][]string)

	for _, model := range data {
		// Skip free tier and special variants — they confuse the
		// dropdown and the free tier has aggressive rate limits.
		if strings.HasSuffix(model.ID, ":free") || strings.HasSuffix(model.ID, ":exacto") {
			continue
		}

		for _, pc := range preferredProviders {
			if strings.HasPrefix(model.ID, pc.prefix) {
				providerModels[pc.prefix] = append(providerModels[pc.prefix], model.ID)
				break
			}
		}
	}

	// Sort each provider's models descending so newer versions land
	// first (lexically: gemini-2.5 > gemini-2.0).
	for prefix := range providerModels {
		models := providerModels[prefix]
		sort.Slice(models, func(i, j int) bool {
			return models[i] > models[j]
		})
	}

	var result []string
	for _, pc := range preferredProviders {
		models := providerModels[pc.prefix]
		count := pc.maxCount
		if len(models) < count {
			count = len(models)
		}
		result = append(result, models[:count]...)
	}

	return result
}

// getFallbackModels returns a static list when the OpenRouter API is
// unreachable. Kept up-to-date manually; mirrors v1.0.3's curated list.
func getFallbackModels() []string {
	return []string{
		// Anthropic
		"anthropic/claude-opus-4.5",
		"anthropic/claude-sonnet-4.5",
		"anthropic/claude-haiku-4.5",
		"anthropic/claude-opus-4",
		"anthropic/claude-sonnet-4",
		// OpenAI
		"openai/gpt-5.2-pro",
		"openai/gpt-5.1",
		"openai/o3-pro",
		"openai/o3",
		"openai/gpt-4o",
		// Google
		"google/gemini-3-pro-preview",
		"google/gemini-2.5-pro",
		"google/gemini-2.5-flash",
		// xAI
		"x-ai/grok-4",
		"x-ai/grok-3",
		// Moonshot
		"moonshotai/kimi-k2",
		// Zhipu
		"z-ai/glm-4.6",
		"z-ai/glm-4.5",
		// DeepSeek
		"deepseek/deepseek-v3.2",
		"deepseek/deepseek-r1",
		// Qwen
		"qwen/qwen3-max",
		"qwen/qwen3-235b-a22b",
		// Meta
		"meta-llama/llama-4-maverick",
		"meta-llama/llama-3.3-70b-instruct",
		// Mistral
		"mistralai/mistral-large-2512",
	}
}

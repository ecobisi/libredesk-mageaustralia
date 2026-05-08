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
// API key encryption-at-rest follows in T3j; for now the key is stored
// plaintext in ai_providers.config. (OpenAI predates this concern and is
// already encrypted via crypto.Encrypt/Decrypt.)
package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/valyala/fasthttp"
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
func (o *OpenRouterClient) SendPrompt(payload PromptPayload) (string, error) {
	if o.apiKey == "" {
		return "", ErrApiKeyNotSet
	}

	apiURL := "https://openrouter.ai/api/v1/chat/completions"

	// Multimodal-capable wire shape. OpenRouter pass-through is
	// OpenAI-compatible (T3b note (i)) so the image_url + detail:low
	// shape lands at whichever upstream the admin selected — Anthropic
	// Claude 3.x, Google Gemini, OpenAI vision models all accept it.
	// Models without vision ignore the image parts; the request still
	// completes against the text content.
	messages := []interface{}{
		map[string]string{"role": "system", "content": payload.SystemPrompt},
	}

	if len(payload.Images) > 0 {
		content := []map[string]interface{}{
			{"type": "text", "text": payload.UserPrompt},
		}
		for _, img := range payload.Images {
			content = append(content, map[string]interface{}{
				"type": "image_url",
				"image_url": map[string]string{
					"url":    img.URL,
					"detail": "low",
				},
			})
		}
		messages = append(messages, map[string]interface{}{
			"role":    "user",
			"content": content,
		})
	} else {
		messages = append(messages, map[string]string{
			"role":    "user",
			"content": payload.UserPrompt,
		})
	}

	requestBody := map[string]interface{}{
		"model":       o.model,
		"messages":    messages,
		"max_tokens":  1024,
		"temperature": 0.7,
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		o.lo.Error("error marshalling request body", "error", err)
		return "", fmt.Errorf("marshalling request body: %w", err)
	}

	req, err := http.NewRequest(fasthttp.MethodPost, apiURL, bytes.NewBuffer(bodyBytes))
	if err != nil {
		o.lo.Error("error creating request", "error", err)
		return "", fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+o.apiKey)
	req.Header.Set("Content-Type", "application/json")
	// OpenRouter usage-attribution headers — surface this install on the
	// OpenRouter dashboard rather than appearing as anonymous traffic.
	req.Header.Set("HTTP-Referer", "https://libredesk.io")
	req.Header.Set("X-Title", "Libredesk")

	resp, err := o.client.Do(req)
	if err != nil {
		o.lo.Error("error making HTTP request", "error", err)
		return "", fmt.Errorf("making HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return "", ErrInvalidAPIKey
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		o.lo.Error("non-ok response received from OpenRouter API", "status", resp.Status, "code", resp.StatusCode, "response_text", string(body))
		return "", fmt.Errorf("API error: %s, body: %s", resp.Status, body)
	}

	var responseBody struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&responseBody); err != nil {
		return "", fmt.Errorf("decoding response body: %w", err)
	}

	if len(responseBody.Choices) > 0 {
		return responseBody.Choices[0].Message.Content, nil
	}
	return "", fmt.Errorf("no response found")
}

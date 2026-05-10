package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/valyala/fasthttp"
	"github.com/zerodha/logf"
)

// chatCompletionRequest is the shared request shape for OpenAI-compatible
// /chat/completions endpoints (OpenAI proper + OpenRouter pass-through).
// Both providers accept the same wire format; the only differences are
// (a) which model name lands in the `model` field, and (b) which extra
// headers (like OpenRouter's HTTP-Referer + X-Title) get attached.
type chatCompletionRequest struct {
	URL         string
	APIKey      string
	Model       string
	Payload     PromptPayload
	MaxTokens   int
	Temperature float64

	// ExtraHeaders are merged on top of the standard
	// Authorization + Content-Type pair. Used by OpenRouter to surface
	// usage attribution; OpenAI proper sends nil.
	ExtraHeaders map[string]string

	// LogTag identifies the upstream provider in log lines —
	// "openai API" / "OpenRouter API" — so a non-200 in the logs
	// points the operator at the right dashboard.
	LogTag string
}

// chatCompletionResponse mirrors the choices[].message.content shape
// returned by both upstreams. Extra fields are ignored by the JSON
// decoder so we don't have to maintain a model-specific schema here.
type chatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// buildChatMessages assembles the messages array for an OpenAI-compatible
// chat/completions request. Text-only payloads keep the
// content-as-string shape (cheaper to parse, more readable in HTTP
// captures); multimodal payloads switch to the content-as-array shape
// since that's the only wire form that allows image_url parts to
// coexist with text in a single turn.
//
// Used by both OpenAIClient and OpenRouterClient — OpenRouter is OpenAI
// pass-through so the same shape lands on Anthropic / Google / xAI etc.
// upstream models. Vision-incapable models silently ignore the image
// parts.
func buildChatMessages(payload PromptPayload) []interface{} {
	messages := []interface{}{
		map[string]string{"role": "system", "content": payload.SystemPrompt},
	}

	if len(payload.Images) > 0 {
		// "low" detail tier — the images are already capped at 500px on
		// the long side (internal/image.MaxAIDimension), and "low" maps
		// to 85 tokens per image regardless of resolution; "high"
		// scales with the image and would burn token budget on each
		// screenshot in a multi-image support thread.
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
	return messages
}

// doChatCompletion executes an OpenAI-compatible /chat/completions
// request and returns the first choice's message content. The request +
// response plumbing is identical between OpenAI and OpenRouter — this
// helper is the single source of truth for the JSON marshal/unmarshal +
// auth header + 401 → ErrInvalidAPIKey shaping.
func doChatCompletion(client *http.Client, lo *logf.Logger, req chatCompletionRequest) (string, error) {
	if req.APIKey == "" {
		return "", ErrApiKeyNotSet
	}

	requestBody := map[string]interface{}{
		"model":       req.Model,
		"messages":    buildChatMessages(req.Payload),
		"max_tokens":  req.MaxTokens,
		"temperature": req.Temperature,
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		lo.Error("error marshalling request body", "error", err)
		return "", fmt.Errorf("marshalling request body: %w", err)
	}

	httpReq, err := http.NewRequest(fasthttp.MethodPost, req.URL, bytes.NewBuffer(bodyBytes))
	if err != nil {
		lo.Error("error creating request", "error", err)
		return "", fmt.Errorf("error creating request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+req.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")
	for k, v := range req.ExtraHeaders {
		httpReq.Header.Set(k, v)
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		lo.Error("error making HTTP request", "error", err)
		return "", fmt.Errorf("making HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return "", ErrInvalidAPIKey
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		lo.Error("non-ok response received from "+req.LogTag, "status", resp.Status, "code", resp.StatusCode, "response_text", string(body))
		return "", fmt.Errorf("API error: %s, body: %s", resp.Status, body)
	}

	var responseBody chatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&responseBody); err != nil {
		return "", fmt.Errorf("decoding response body: %w", err)
	}

	if len(responseBody.Choices) > 0 {
		return responseBody.Choices[0].Message.Content, nil
	}
	return "", fmt.Errorf("no response found")
}

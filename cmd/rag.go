package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/abhinavxd/libredesk/internal/ai"
	"github.com/abhinavxd/libredesk/internal/envelope"
	"github.com/abhinavxd/libredesk/internal/rag/models"
	settingmodels "github.com/abhinavxd/libredesk/internal/setting/models"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

// DefaultMaxRAGImages is the upper bound on conversation images the
// multimodal generate path includes (T3e). 3 is the v1.0.3-tuned value
// that keeps token-cost for "low" detail tier images bounded for typical
// support threads while still capturing the customer's most recent few
// screenshots.
const DefaultMaxRAGImages = 3

// T3d external-search-API integration types.
//
// SearchIntent is one classified bucket the LLM derived from the
// customer message. SearchClassification wraps the JSON the classifier
// returns; the type/query strings are echoed back into the search call.
//
// ExternalSearchHit and ExternalSearchResponse mirror the
// Meilisearch /indexes/<idx>/search response shape with optional
// product-/category-/FAQ-specific fields. Generic enough to absorb any
// Meilisearch-compatible search backend; non-Meilisearch APIs that
// expose a similarly-shaped JSON response (Typesense, Elasticsearch
// adapters) work as long as they return `hits` of objects with at
// minimum a `name`/`question` field.
type SearchIntent struct {
	Type  string `json:"type"`
	Query string `json:"query"`
}

type SearchClassification struct {
	Intents []SearchIntent `json:"intents"`
}

type ExternalSearchHit struct {
	Name            string                 `json:"name"`
	Question        string                 `json:"question"`
	Answer          string                 `json:"answer"`
	URL             string                 `json:"url"`
	Description     string                 `json:"description"`
	MetaDescription string                 `json:"meta_description"`
	BrandID         string                 `json:"brand_id"`
	InStock         int                    `json:"in_stock"`
	ProductCount    int                    `json:"product_count"`
	Price           map[string]interface{} `json:"price"`
	Categories      map[string]interface{} `json:"categories"`
}

type ExternalSearchResponse struct {
	Hits               []ExternalSearchHit `json:"hits"`
	Query              string              `json:"query"`
	EstimatedTotalHits int                 `json:"estimatedTotalHits"`
}

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
// {{today}} / {{context}} / {{macros}} / {{enquiry}} /
// {{external_search_results}} substitutions as a custom prompt.
// The customer question is wrapped in <customer_message> XML
// delimiters so the LLM treats it as opaque user data rather than
// executable instructions — mitigates "IGNORE ALL PREVIOUS
// INSTRUCTIONS"-style prompt injection (T3l, mirrors v1.0.3 7f73f8f5).
// Custom admin-set prompts are not auto-wrapped because admins are
// trusted to design their own template; the substitution only happens
// at the {{enquiry}} site.
const defaultRAGSystemPrompt = `You are a helpful customer support assistant for {{site_name}}. Use the following knowledge base content to answer questions accurately.

Today is {{today}}.

Knowledge Base Context:
{{context}}

Customer Question:
<customer_message>
{{enquiry}}
</customer_message>

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

	// Cap conversation context to bound prompt size, AI provider
	// timeouts, and token cost on long email threads or adversarial
	// input. The frontend ALSO limits to the last 10 messages — this
	// is the server-side ceiling regardless of caller. Truncation
	// keeps the tail (most recent) so the customer's latest question
	// is preserved; older context is replaced with a marker so the
	// LLM knows the message was clipped (T3f, mirrors v1.0.3
	// d986a684).
	const maxCustomerMessageLen = 6000
	if len(req.CustomerMessage) > maxCustomerMessageLen {
		original := len(req.CustomerMessage)
		req.CustomerMessage = "[Earlier messages truncated]\n\n" + req.CustomerMessage[original-maxCustomerMessageLen:]
		app.lo.Info("RAG generate: truncated customer message", "original_len", original, "truncated_to", len(req.CustomerMessage))
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

	// T3e: pull image attachments from the conversation, resize, and
	// shape into ImageContent slots for the multimodal payload.
	// Failure here does NOT fail the whole generate call — vision
	// support is additive context, the LLM can still draft a reply
	// from the text. Skipped entirely when conversation_id was not
	// supplied (admin "Test Knowledge Base" search hits this handler
	// with conversation_id=0).
	var aiImages []ai.ImageContent
	if req.ConversationID > 0 {
		images, err := app.rag.GetConversationImages(req.ConversationID, DefaultMaxRAGImages)
		if err != nil {
			app.lo.Warn("failed to get conversation images, continuing without", "conversation_id", req.ConversationID, "error", err)
		} else if len(images) > 0 {
			aiImages = make([]ai.ImageContent, 0, len(images))
			for _, img := range images {
				aiImages = append(aiImages, ai.ImageContent{
					URL:      img.DataURL,
					Filename: img.Filename,
				})
			}
			app.lo.Info("conversation images extracted for AI", "conversation_id", req.ConversationID, "count", len(aiImages))
		}
	}

	// T3d: optionally augment the prompt with results from an external
	// HTTP search API. Two-step pipeline: classify the customer message
	// into search intents (product / category / faq / …), then fan out
	// one HTTP POST per matched endpoint and format the hits as a
	// dedicated context block. Failure at any stage degrades gracefully
	// to "no external context" rather than failing the whole generate
	// call — the LLM still has the pgvector chunks to work with.
	var externalSearchContext string
	if aiSettings.ExternalSearchEnabled && aiSettings.ExternalSearchURL != "" {
		maxSearchResults := aiSettings.ExternalSearchMaxResults
		if maxSearchResults <= 0 {
			maxSearchResults = 3
		}
		intents, err := app.classifySearchIntent(req.CustomerMessage)
		if err != nil {
			app.lo.Warn("external search classification failed, continuing without", "error", err)
		} else {
			app.lo.Info("external search classification", "intents", intents)
			externalSearchContext = app.performExternalSearch(aiSettings, intents, maxSearchResults)
			if externalSearchContext != "" {
				app.lo.Info("external search results added to context", "length", len(externalSearchContext))
			}
		}
	}

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
	// Inject current date (AEST) so the LLM knows what "today" and
	// "recently" mean — prevents stale date references in responses
	// and lets the conversation timestamps (T3g frontend hunk) be
	// reasoned about relative to a fixed anchor. AEST chosen to match
	// v1.0.3 source; if other deployments need a different zone the
	// substitution site is the place to extend (T3g, mirrors v1.0.3
	// 30b5194c).
	now := time.Now().In(time.FixedZone("AEST", 10*60*60))
	systemPrompt = strings.ReplaceAll(systemPrompt, "{{today}}", fmt.Sprintf("%s %d %s %d", now.Weekday(), now.Day(), now.Month(), now.Year()))
	systemPrompt = strings.ReplaceAll(systemPrompt, "{{site_name}}", ko.String("app.site_name"))
	systemPrompt = strings.ReplaceAll(systemPrompt, "{{context}}", contextStr)
	systemPrompt = strings.ReplaceAll(systemPrompt, "{{macros}}", macrosStr)
	// T3d: empty-string when external search is disabled, unreachable,
	// or returned no hits — admins who don't include the placeholder in
	// their template are unaffected; admins who do see "" rather than
	// the literal token. Substituted before {{enquiry}} so a customer
	// message containing the literal "{{external_search_results}}"
	// can't be promoted into the substitution slot.
	systemPrompt = strings.ReplaceAll(systemPrompt, "{{external_search_results}}", externalSearchContext)
	systemPrompt = strings.ReplaceAll(systemPrompt, "{{enquiry}}", req.CustomerMessage)

	// T3e: nudge the LLM to actually look at the attached images. Models
	// that ignore image_url parts (no-vision models on OpenRouter) lose
	// nothing from this prompt addition; vision-capable models gain a
	// directive to reference what they see. Appended after all template
	// substitutions so admin-edited prompts still get the directive when
	// images are present.
	if len(aiImages) > 0 {
		systemPrompt += "\n\nNote: The customer has attached images to this conversation. Please examine them and reference relevant details in your response."
	}

	response, err := app.ai.CompletionWithPayload(ai.PromptPayload{
		SystemPrompt: systemPrompt,
		UserPrompt:   req.CustomerMessage,
		Images:       aiImages,
	})
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(map[string]interface{}{
		"response": response,
		"sources":  results,
	})
}

// classifySearchIntent asks the LLM to bucket the customer's message
// into zero or more search intents (T3d). The classifier is deliberately
// conservative — pure greetings/thanks return an empty list so we don't
// fan out HTTP calls on chit-chat. The customer message is wrapped in
// XML delimiters to mitigate "IGNORE ALL PREVIOUS INSTRUCTIONS"-style
// prompt-injection (T3l hardening, applied to this second prompt site
// too — v1.0.3 0da66067 had it on classifySearchIntent already).
//
// Output is hand-stripped of markdown code fences before json.Unmarshal
// because some providers wrap their JSON in ```json blocks even when
// asked not to. Failure (parse, network, etc.) is non-fatal — the caller
// logs and skips external search.
func (app *App) classifySearchIntent(message string) ([]SearchIntent, error) {
	classifyPrompt := `Analyze this customer support message and extract search intents.
Return JSON only, no other text.

Message:
<customer_message>
` + message + `
</customer_message>

Response format:
{"intents": [{"type": "product", "query": "concise search terms"}, {"type": "category", "query": "concise terms"}, {"type": "faq", "query": "concise terms"}]}

Rules:
- Only include intent types that are relevant to the message
- Keep queries to 2-5 words, suitable for search
- "product" = looking for a specific product, brand, or item
- "category" = browsing a type/category of items
- "faq" = asking about policies, shipping, returns, orders, delivery, etc.
- A message can have multiple intents
- If the message is purely conversational (greetings, thanks) or not related to products/policies, return empty intents: {"intents": []}
- Do NOT wrap in markdown code blocks`

	response, err := app.ai.CompletionWithSystemPrompt("You are a JSON-only classifier. Output valid JSON only, no markdown, no explanation.", classifyPrompt)
	if err != nil {
		return nil, fmt.Errorf("classification failed: %w", err)
	}

	response = strings.TrimSpace(response)
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	var classification SearchClassification
	if err := json.Unmarshal([]byte(response), &classification); err != nil {
		return nil, fmt.Errorf("failed to parse classification: %w (response: %s)", err, response)
	}
	return classification.Intents, nil
}

// queryExternalSearch issues one POST against a Meilisearch-compatible
// search endpoint. Routes through the SS2 SSRF-guarded http.Client on
// the App so admin-supplied URLs cannot pivot to private/loopback
// targets — the dialer rejects RFC1918/link-local/loopback/IPv6-
// reserved ranges unless ai.allowed_hosts CIDR-allowlists them. Returns
// a generic "blocked by SSRF guard" wrap on dial-deny so the caller's
// log makes the cause visible without leaking internal IPs.
func (app *App) queryExternalSearch(searchURL, query string, limit int, headers map[string]string) (*ExternalSearchResponse, error) {
	payload := fmt.Sprintf(`{"q":%q,"limit":%d}`, query, limit)
	req, err := http.NewRequest(http.MethodPost, searchURL, bytes.NewBufferString(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "Libredesk/2")
	}

	client := app.extSearchClient
	if client == nil {
		// Defensive — main.go always wires this. A nil client at runtime
		// would mean the wiring regressed; falling back to a guarded
		// no-op client is the safest behaviour.
		return nil, fmt.Errorf("external search HTTP client not initialised")
	}
	resp, err := client.Do(req)
	if err != nil {
		// ssrfguard surfaces a "blocked address" / dial error here.
		// Wrap once with a recognisable phrase so operators tracing
		// "blocked by SSRF guard" in logs land on the right call site.
		if strings.Contains(err.Error(), "blocked") || strings.Contains(err.Error(), "denied") {
			return nil, fmt.Errorf("blocked by SSRF guard: %w", err)
		}
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("external search returned status %d: %s", resp.StatusCode, string(body))
	}

	var result ExternalSearchResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// performExternalSearch fans out one HTTP call per matched intent and
// formats the hits into a Markdown-ish context block fed into the
// system prompt via {{external_search_results}}. Empty endpoint config,
// no matched intents, and unreachable URLs all return the empty string
// — the caller's substitution stays a literal empty token in the
// prompt, which the LLM ignores cleanly.
//
// Endpoint and header config are stored as JSON-string settings rather
// than typed structs because the v1.0.3 admin form lets operators wire
// in arbitrary key/value pairs (intent → path; header name → header
// value); a typed struct would force a schema migration on every new
// supported intent type.
func (app *App) performExternalSearch(aiSettings settingmodels.AISettings, intents []SearchIntent, maxResults int) string {
	endpoints := make(map[string]string)
	if aiSettings.ExternalSearchEndpoints != "" {
		if err := json.Unmarshal([]byte(aiSettings.ExternalSearchEndpoints), &endpoints); err != nil {
			app.lo.Warn("failed to parse external search endpoints config", "error", err)
			return ""
		}
	}
	if len(endpoints) == 0 {
		app.lo.Warn("no external search endpoints configured")
		return ""
	}

	headers := make(map[string]string)
	if aiSettings.ExternalSearchHeaders != "" {
		if err := json.Unmarshal([]byte(aiSettings.ExternalSearchHeaders), &headers); err != nil {
			app.lo.Warn("failed to parse external search headers config", "error", err)
		}
	}

	baseURL := strings.TrimRight(aiSettings.ExternalSearchURL, "/")

	var sections []string
	for _, intent := range intents {
		endpointPath, ok := endpoints[intent.Type]
		if !ok {
			continue
		}
		searchURL := baseURL + endpointPath
		results, err := app.queryExternalSearch(searchURL, intent.Query, maxResults, headers)
		if err != nil {
			app.lo.Warn("external search query failed", "type", intent.Type, "query", intent.Query, "error", err)
			continue
		}
		if len(results.Hits) == 0 {
			continue
		}

		switch intent.Type {
		case "product":
			var lines []string
			for i, hit := range results.Hits {
				price := ""
				if aud, ok := hit.Price["AUD"]; ok {
					if audMap, ok := aud.(map[string]interface{}); ok {
						if formatted, ok := audMap["default_formated"].(string); ok {
							price = formatted
						}
						if origFormatted, ok := audMap["default_original_formated"].(string); ok {
							price += " (was " + origFormatted + ")"
						}
					}
				}
				stock := "In Stock"
				if hit.InStock == 0 {
					stock = "Out of Stock"
				}
				line := fmt.Sprintf("%d. %s", i+1, hit.Name)
				if hit.BrandID != "" {
					line += " by " + hit.BrandID
				}
				if price != "" {
					line += " - " + price
				}
				line += " - " + stock
				line += "\n   URL: " + hit.URL
				desc := stripHTMLForExternalSearch(hit.Description)
				if len(desc) > 200 {
					desc = desc[:200] + "..."
				}
				if desc != "" {
					line += "\n   " + strings.TrimSpace(desc)
				}
				lines = append(lines, line)
			}
			sections = append(sections, "=== Product Results (from website) ===\n"+strings.Join(lines, "\n\n"))

		case "category":
			var lines []string
			for i, hit := range results.Hits {
				line := fmt.Sprintf("%d. %s (%d products)", i+1, hit.Name, hit.ProductCount)
				line += "\n   URL: " + hit.URL
				if hit.MetaDescription != "" {
					desc := hit.MetaDescription
					if len(desc) > 200 {
						desc = desc[:200] + "..."
					}
					line += "\n   " + desc
				}
				lines = append(lines, line)
			}
			sections = append(sections, "=== Category Results (from website) ===\n"+strings.Join(lines, "\n\n"))

		case "faq":
			var lines []string
			for i, hit := range results.Hits {
				line := fmt.Sprintf("%d. Q: %s\n   A: %s", i+1, hit.Question, hit.Answer)
				line += "\n   URL: " + hit.URL
				lines = append(lines, line)
			}
			sections = append(sections, "=== FAQ Results (from website) ===\n"+strings.Join(lines, "\n\n"))
		}
	}
	if len(sections) == 0 {
		return ""
	}
	return strings.Join(sections, "\n\n")
}

// stripHTMLForExternalSearch is a tiny tag-stripper used to flatten the
// product description before injecting into the prompt context. v1.0.3
// 0da66067 carried its own stripHTML in cmd/rag.go; v2 already has a
// regex-based version in internal/rag/sync/macro.go (T3l-hoisted), but
// it's package-internal there. Rather than exporting it just for this
// site, we keep a small purpose-built one — same behaviour for this
// callsite, scoped to ~200-char product blurbs.
func stripHTMLForExternalSearch(s string) string {
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	for strings.Contains(s, "<") {
		start := strings.Index(s, "<")
		end := strings.Index(s[start:], ">")
		if end == -1 {
			break
		}
		s = s[:start] + s[start+end+1:]
	}
	return strings.TrimSpace(s)
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

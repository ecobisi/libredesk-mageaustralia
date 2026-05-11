package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/abhinavxd/libredesk/internal/ai"
	"github.com/abhinavxd/libredesk/internal/envelope"
	"github.com/abhinavxd/libredesk/internal/rag"
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
	// T3u: shipping attributes pulled from the Meilisearch product index.
	// BulkyGoods is reserved for future product-level callouts (currently
	// unused in formatting). DisableFreeShip is the per-product override
	// that triggers "CUSTOM FREIGHT QUOTE REQUIRED" — checked
	// case-insensitively against literal "Yes" so admin-curated values
	// like "yes"/"YES"/"Yes" all match.
	BulkyGoods      string                 `json:"bulky_goods"`
	DisableFreeShip string                 `json:"disable_free_shipping"`
	Price           map[string]interface{} `json:"price"`
	Categories      map[string]interface{} `json:"categories"`
	// T3u: per-SKU stock map populated when the product has size/option
	// variants. Outer key = SKU; inner map carries `qty` (float64 from
	// JSON) and `in_stock` (float64, 0 = out, >0 = in). Optional — single-
	// SKU products omit this and the formatter skips the "Stock by
	// size/option" line.
	SkuStockData map[string]map[string]interface{} `json:"sku_stock_data"`
}

type ExternalSearchResponse struct {
	Hits               []ExternalSearchHit `json:"hits"`
	Query              string              `json:"query"`
	EstimatedTotalHits int                 `json:"estimatedTotalHits"`
}

// MultiSearchResponse wraps Meilisearch's `/multi-search` API response.
// T3u: enables endpoints to use a single Meilisearch instance to query
// multiple indexes (or a single index with a filter expression) by
// configuring the endpoint path as `multi-search:indexUid` or
// `multi-search:indexUid:filter_expression`. Each entry in `results`
// has the same shape as a per-index search response — `performExternalSearch`
// only consumes the first entry since one intent maps to one indexUid.
type MultiSearchResponse struct {
	Results []ExternalSearchResponse `json:"results"`
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

// defaultRAGSystemPrompt is used when ai.system_prompt is empty. The
// canonical template lives in internal/rag/default_prompt.go so the
// v2.2.19 migration can also seed it into the DB on fresh + upgrading
// installs (where empty rows get the example as a starting point).
// If the admin clears the prompt back to empty, the code falls back
// to this same constant.
var defaultRAGSystemPrompt = rag.DefaultSystemPrompt

// handleRAGGenerateResponse is the agent-facing "Generate Response"
// endpoint. Takes the customer's most recent message (or full
// conversation, the frontend is free to either) and produces an AI-
// drafted reply by:
//
//  1. Resolving the effective AI settings — per-inbox if an override
//     exists for the conversation's inbox, otherwise the global config
//     (T3h).
//  2. Embedding the question + cosine-search rag_documents (optionally
//     scoped to a per-inbox subset of knowledge sources).
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
		// T3h: explicit inbox_id override. When omitted (and a
		// ConversationID is supplied), the handler resolves it via
		// conversation.GetConversationInboxID. The admin Test
		// Knowledge Base path passes neither and resolves to the
		// global settings.
		InboxID int `json:"inbox_id"`
		// T3r: opt-in flag set by the "+ Orders" button. When true
		// AND the conversation has an ID AND ecommerce is configured,
		// the handler fans out to app.ecommerce.GatherFullContext to
		// produce the customer / recent orders / mentioned-orders
		// markdown block injected into the prompt. Default false so
		// the standard "Generate Response" path makes zero ecommerce
		// API calls — matters because Magento/Maho lookups can be
		// slow and the agent shouldn't pay that latency unless they
		// asked for it.
		IncludeEcommerce bool `json:"include_ecommerce"`
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

	// T3h: resolve inbox_id — explicit body value wins, otherwise look
	// up via conversation. Lookup failure is logged + ignored so the
	// global-fallback path still works.
	inboxID := req.InboxID
	if inboxID == 0 && req.ConversationID > 0 {
		if convInboxID, err := app.conversation.GetConversationInboxID(req.ConversationID); err == nil {
			inboxID = convInboxID
		} else {
			app.lo.Warn("RAG generate: failed to resolve inbox from conversation, using global", "conversation_id", req.ConversationID, "error", err)
		}
	}

	aiSettings, err := app.setting.GetEffectiveAISettings(inboxID)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	app.lo.Info("RAG generate using AI settings", "inbox_id", inboxID, "has_inbox_override", aiSettings.ID > 0)

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

	// T3h: parse per-inbox knowledge source ID filter. Empty array (or
	// parse failure) falls back to "search all sources" behaviour.
	var sourceIDs []int
	if len(aiSettings.KnowledgeSourceIDs) > 0 && string(aiSettings.KnowledgeSourceIDs) != "[]" {
		if err := json.Unmarshal(aiSettings.KnowledgeSourceIDs, &sourceIDs); err != nil {
			app.lo.Warn("failed to parse knowledge source IDs, falling back to all sources", "error", err)
			sourceIDs = nil
		}
	}

	results, err := app.rag.Search(req.CustomerMessage, maxChunks, threshold, sourceIDs...)
	if err != nil {
		app.lo.Warn("RAG search failed, continuing without context", "error", err)
		results = []models.SearchResult{}
	}
	app.lo.Info("RAG generate response", "results_count", len(results), "threshold", threshold, "source_ids", sourceIDs)

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

	// T3r: optional ecommerce context (customer + recent orders +
	// orders mentioned in the conversation). Gated on the explicit
	// IncludeEcommerce flag — the standard "Generate Response" button
	// stays free of Magento/Maho lookups. ConversationID 0 (admin
	// "Test Knowledge Base") and an unconfigured manager both short-
	// circuit to "" + nil. ecommerceWarnings is surfaced verbatim in
	// the envelope so the agent UI can toast auth failures instead
	// of silently degrading the prompt.
	var ecommerceContext string
	var ecommerceWarnings []string
	if req.IncludeEcommerce && req.ConversationID > 0 && app.ecommerce != nil && app.ecommerce.IsConfigured() {
		ecommerceContext, ecommerceWarnings = app.gatherEcommerceContext(r.RequestCtx, req.ConversationID)
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

	// T3u: filter knowledge-base chunks that mention bulky / oversized /
	// freight / courier shipping policy. These chunks tend to encode
	// site-wide "we don't ship X" rules that contradict the per-product
	// shipping tags emitted by performExternalSearch (FREE SHIPPING /
	// STANDARD SHIPPING / CUSTOM FREIGHT QUOTE REQUIRED). Without this
	// filter the LLM sometimes overrides the per-product label with the
	// generic policy from the KB, producing inconsistent shipping advice.
	// Macros are not filtered — they're tone references, not factual
	// sources, and admins curate them directly.
	shippingFilterWords := []string{"bulky", "oversized", "freight", "courier"}
	var contextParts, macroParts []string
	for _, res := range results {
		if strings.HasPrefix(res.SourceRef, "macro_") {
			macroParts = append(macroParts, "- "+res.Title+": "+res.Content)
		} else {
			lower := strings.ToLower(res.Content)
			skip := false
			for _, kw := range shippingFilterWords {
				if strings.Contains(lower, kw) {
					skip = true
					break
				}
			}
			if !skip {
				contextParts = append(contextParts, "## "+res.Title+"\n"+res.Content)
			}
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

	// T3r: append the ecommerce markdown block to the assembled prompt
	// so the LLM has order/customer details to draw from. Done after all
	// {{...}} substitutions to keep the block out of the admin-editable
	// template surface — the block format is fixed by FormatContextForPrompt
	// and isn't meant to be sliced by template authors.
	if ecommerceContext != "" {
		systemPrompt += "\n\n" + ecommerceContext
	}

	response, err := app.ai.CompletionWithPayload(ai.PromptPayload{
		SystemPrompt: systemPrompt,
		UserPrompt:   req.CustomerMessage,
		Images:       aiImages,
	})
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	// T3r: ecommerce_warnings is omitted when nil/empty so the response
	// shape stays minimal for the common no-ecommerce path. ReplyBox.vue
	// only iterates if the field is a non-empty array, so either shape
	// works on the wire — explicit nil keeps the success envelope tidy.
	resp := map[string]interface{}{
		"response": response,
		"sources":  results,
	}
	if len(ecommerceWarnings) > 0 {
		resp["ecommerce_warnings"] = ecommerceWarnings
	}
	return r.SendEnvelope(resp)
}

// gatherEcommerceContext fetches the conversation's contact email +
// last 50 messages, hands them to app.ecommerce.GatherFullContext (T3o
// multi-stage: customer-by-email + recent-orders-by-email + scan
// messages for order numbers + per-order Stage 3 lookups), and returns
// the FormatContextForPrompt markdown block plus the warnings slice.
//
// All failures along the way log + return ("" , nil) so the RAG flow
// degrades to "no ecommerce context" rather than failing the whole
// generate call. Warnings (cascading auth failures, network errors)
// are surfaced verbatim to the caller so the UI can toast them — the
// agent needs to know "your AI reply was generated without order data
// because Maho 401'd" rather than silently getting a generic answer.
//
// Mirrors v1.0.3 bb7895b9 + T3ae(d). Uses the v2 conversation manager
// signatures: GetConversationUUID(id), GetConversation(id, uuid, ""),
// GetConversationMessages(uuid, page, pageSize, private, msgTypes).
func (app *App) gatherEcommerceContext(ctx context.Context, conversationID int) (string, []string) {
	uuid, err := app.conversation.GetConversationUUID(conversationID)
	if err != nil {
		app.lo.Warn("ecommerce: failed to get conversation UUID", "conversation_id", conversationID, "error", err)
		return "", nil
	}

	conv, err := app.conversation.GetConversation(conversationID, uuid, "")
	if err != nil {
		app.lo.Warn("ecommerce: failed to get conversation", "conversation_id", conversationID, "error", err)
		return "", nil
	}

	customerEmail := ""
	if conv.Contact.Email.Valid {
		customerEmail = conv.Contact.Email.String
	}
	if customerEmail == "" {
		app.lo.Debug("ecommerce: no contact email on conversation", "conversation_id", conversationID)
		return "", nil
	}

	// Page 1, pageSize 50 — wide enough to catch order numbers buried
	// deep in long support threads, narrow enough to keep the manager's
	// regex scan fast. Failure is non-fatal: GatherFullContext still
	// has the email-based lookups to work with.
	messages, _, err := app.conversation.GetConversationMessages(uuid, 1, 50, nil, nil)
	if err != nil {
		app.lo.Warn("ecommerce: failed to fetch messages for order-number scan, continuing without", "conversation_id", conversationID, "error", err)
	}

	messageTexts := make([]string, 0, len(messages))
	for _, msg := range messages {
		if msg.Content == "" {
			continue
		}
		// HTML stripped because the order-number regex shouldn't match
		// anything inside attribute values or tag names. Reuses the
		// stripHTMLForExternalSearch helper already used by the T3d
		// external-search path — same regex strip is fine here.
		text := stripHTMLForExternalSearch(msg.Content)
		if text != "" {
			messageTexts = append(messageTexts, text)
		}
	}

	app.lo.Info("ecommerce: gathering context", "conversation_id", conversationID, "email", customerEmail, "messages_scanned", len(messageTexts))

	// maxOrders=5 matches v1.0.3 — 5 most-recent orders are enough to
	// give the LLM a sense of the customer's history without bloating
	// the prompt; per-order Stage 3 lookups bolt on full details for
	// orders specifically referenced in the conversation.
	eCtx, err := app.ecommerce.GatherFullContext(ctx, customerEmail, messageTexts, 5)
	if err != nil {
		app.lo.Warn("ecommerce: GatherFullContext failed", "email", customerEmail, "error", err)
		return "", nil
	}

	formatted := app.ecommerce.FormatContextForPrompt(eCtx)
	if formatted != "" {
		app.lo.Info("ecommerce: context added to prompt", "email", customerEmail, "context_length", len(formatted), "warnings", len(eCtx.Warnings))
	}

	return formatted, eCtx.Warnings
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

// queryExternalMultiSearch issues one POST against a Meilisearch
// `/multi-search` endpoint that bundles a single named-index query.
// Used when the admin endpoints config maps an intent to
// `multi-search:<indexUid>` or `multi-search:<indexUid>:<filter>` —
// useful for instances where one Meilisearch host indexes multiple
// stores and a per-store filter expression (e.g. `store_id = 1`) keeps
// results scoped. The same SSRF-guarded client used by
// queryExternalSearch is reused so admin-supplied URLs cannot pivot to
// private targets. The response is unwrapped to the first results
// entry so callers see the same `*ExternalSearchResponse` shape as the
// per-index path. Empty `results` from the API yields an empty
// response (not an error) so the caller's "no hits" branch handles it
// uniformly. Mirrors v1.0.3 cf97cb06 — adapted to take an App receiver
// so the SSRF client and UA header are shared with queryExternalSearch.
func (app *App) queryExternalMultiSearch(searchURL, indexUid, query string, limit int, filter string, headers map[string]string) (*ExternalSearchResponse, error) {
	qObj := map[string]interface{}{
		"indexUid": indexUid,
		"q":        query,
		"limit":    limit,
	}
	if filter != "" {
		qObj["filter"] = []string{filter}
	}
	payload, err := json.Marshal(map[string]interface{}{
		"queries": []interface{}{qObj},
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, searchURL, bytes.NewBuffer(payload))
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
		return nil, fmt.Errorf("external search HTTP client not initialised")
	}
	resp, err := client.Do(req)
	if err != nil {
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
		return nil, fmt.Errorf("multi-search returned status %d: %s", resp.StatusCode, string(body))
	}

	var multiResult MultiSearchResponse
	if err := json.Unmarshal(body, &multiResult); err != nil {
		return nil, fmt.Errorf("failed to parse multi-search response: %w", err)
	}
	if len(multiResult.Results) == 0 {
		return &ExternalSearchResponse{}, nil
	}
	return &multiResult.Results[0], nil
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
//
// T3h: takes InboxAISettings rather than the global AISettings — the
// per-inbox override path passes its own external-search config when
// configured, the global-fallback path passes the projected global
// values via setting.GetEffectiveAISettings. Either way the three fields
// read here (URL / Endpoints / Headers) have identical semantics.
func (app *App) performExternalSearch(aiSettings settingmodels.InboxAISettings, intents []SearchIntent, maxResults int) string {
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

		// T3u: dispatch on `multi-search:` prefix. This format selects
		// Meilisearch's `/multi-search` API instead of the per-index
		// `/indexes/<idx>/search` API, optionally with a Meilisearch
		// filter expression carried as the third colon-delimited
		// segment. Endpoints without the prefix continue to use the
		// existing per-index path so existing admin configs keep
		// working unchanged.
		var (
			results *ExternalSearchResponse
			err     error
		)
		if strings.HasPrefix(endpointPath, "multi-search:") {
			parts := strings.SplitN(endpointPath, ":", 3)
			indexUid := parts[1]
			filter := ""
			if len(parts) == 3 {
				filter = parts[2]
			}
			fullURL := baseURL + "/multi-search"
			results, err = app.queryExternalMultiSearch(fullURL, indexUid, intent.Query, maxResults, filter, headers)
		} else {
			searchURL := baseURL + endpointPath
			results, err = app.queryExternalSearch(searchURL, intent.Query, maxResults, headers)
		}
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
				// T3u: build per-SKU stock summary for products with size/
				// option variants. Each SKU's status is "<qty> in stock" or
				// "out of stock"; map iteration order is non-deterministic,
				// which matches v1.0.3 — the LLM doesn't rely on order, just
				// presence/absence of each SKU.
				skuStock := ""
				if len(hit.SkuStockData) > 0 {
					var skuParts []string
					for sku, data := range hit.SkuStockData {
						qty := 0
						if q, ok := data["qty"].(float64); ok {
							qty = int(q)
						}
						inStk := false
						if s, ok := data["in_stock"].(float64); ok && s > 0 {
							inStk = true
						}
						status := "out of stock"
						if inStk {
							status = fmt.Sprintf("%d in stock", qty)
						}
						skuParts = append(skuParts, fmt.Sprintf("%s: %s", sku, status))
					}
					skuStock = strings.Join(skuParts, ", ")
				}
				// T3u: shipping label per product. DisableFreeShip="Yes"
				// always wins (admin-curated override). Otherwise infer from
				// AUD default price: AUD$150 is the AU free-shipping
				// threshold on Mage Australia / Trabulium stores; below
				// that, "STANDARD SHIPPING ($5)". Non-AUD currencies and
				// missing-price products get no shipping line at all
				// (silent fallthrough) — better than misleading the LLM.
				if strings.EqualFold(hit.DisableFreeShip, "Yes") {
					line += " - CUSTOM FREIGHT QUOTE REQUIRED"
				} else {
					if aud, ok := hit.Price["AUD"]; ok {
						if audMap, ok := aud.(map[string]interface{}); ok {
							if defPrice, ok := audMap["default"].(float64); ok && defPrice >= 150 {
								line += " - FREE SHIPPING"
							} else {
								line += " - STANDARD SHIPPING ($5)"
							}
						}
					}
				}
				line += "\n   URL: " + hit.URL
				if skuStock != "" {
					line += "\n   Stock by size/option: " + skuStock
				}
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
			// T3u: same bulky/freight skip as the KB-context loop above —
			// FAQ entries about generic shipping policy contradict the
			// per-product shipping tags. Combined Q + A is lower-cased
			// once and substring-matched against the same word list.
			skipKeywords := []string{"bulky", "oversized", "freight", "courier"}
			var lines []string
			for i, hit := range results.Hits {
				combined := strings.ToLower(hit.Question + " " + hit.Answer)
				skip := false
				for _, kw := range skipKeywords {
					if strings.Contains(combined, kw) {
						skip = true
						break
					}
				}
				if skip {
					continue
				}
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

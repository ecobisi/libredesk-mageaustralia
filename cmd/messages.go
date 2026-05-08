package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/mail"
	"strings"
	"sync"
	"time"

	amodels "github.com/abhinavxd/libredesk/internal/auth/models"
	authzModels "github.com/abhinavxd/libredesk/internal/authz/models"
	cmodels "github.com/abhinavxd/libredesk/internal/conversation/models"
	"github.com/abhinavxd/libredesk/internal/envelope"
	imodels "github.com/abhinavxd/libredesk/internal/inbox/models"
	"github.com/abhinavxd/libredesk/internal/stringutil"
	umodels "github.com/abhinavxd/libredesk/internal/user/models"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

// messageDedupMap prevents duplicate message submissions within a short
// window. Live ops kept seeing customers receive two identical replies when
// an agent's "send failed" toast fired (transient timeout) and the agent
// retried — but the original send had actually succeeded server-side.
var messageDedupMap sync.Map

// messageDedupTTL bounds the dedup window. 60s covers the worst case we've
// seen in prod (#6043): an agent waiting ~11s on a perceived-failed send
// then retrying. Shorter windows let real duplicates slip through; longer
// risks blocking legitimate "I meant to send the same thing twice" cases.
const messageDedupTTL = 60 * time.Second

// checkMessageDedup returns true if this message was already sent recently.
// The key includes setStatus so a "Send" followed by "Send & Resolve" with
// the same body isn't conflated — they're different actions even though the
// content matches. Without this, EC1's Send-and-Set-Status dropdown could
// silently no-op the second click.
//
// `from` is also part of the key (EC14 follow-up): an agent who first sends
// as support@ and then re-selects orders@ to re-send the same body — e.g.
// the recipient asks them to use a different alias — must not be blocked
// as a duplicate. Different From = different intent.
func checkMessageDedup(userID int, convUUID, content, setStatus, from string) bool {
	h := sha256.Sum256([]byte(content))
	// Truncating to first 8 bytes (64 bits) is intentional — collision-safe
	// at this scale (single agent's per-conv keyspace within a 60s window)
	// and the per-user/per-conv namespace prevents cross-tenant collisions.
	key := fmt.Sprintf("%d:%s:%s:%s:%x", userID, convUUID, setStatus, from, h[:8])

	if _, loaded := messageDedupMap.LoadOrStore(key, time.Now()); loaded {
		return true
	}

	// Evict after TTL so the map doesn't grow unbounded over the day.
	go func() {
		time.Sleep(messageDedupTTL)
		messageDedupMap.Delete(key)
	}()
	return false
}

type messageReq struct {
	Attachments []int                  `json:"attachments"`
	Message     string                 `json:"message"`
	Private     bool                   `json:"private"`
	To          []string               `json:"to"`
	CC          []string               `json:"cc"`
	BCC         []string               `json:"bcc"`
	SenderType  string                 `json:"sender_type"`
	Mentions    []cmodels.MentionInput `json:"mentions"`
	EchoID      string                 `json:"echo_id"`
	// ForwardedTo, when non-empty, signals "forward this conversation to
	// the listed addresses". The original recipients (To) are ignored;
	// CC/BCC pass through unchanged so an agent can loop in teammates.
	ForwardedTo []string `json:"forwarded_to"`
	// SetStatus, when non-empty, transitions the conversation to that
	// status name after a successful send. Powers the "Send & Resolve" /
	// "Send & Close" dropdown in the reply box.
	SetStatus string `json:"set_status"`
	// From, when non-empty, overrides the inbox's primary From address
	// for this single send. Powers EC14's per-inbox From switcher
	// (e.g. send as "orders@" instead of "support@" from the same inbox).
	// Must match the inbox's configured primary From or one of the
	// configured aliases — validated at the handler before the send is
	// queued so a malicious client can't spoof an arbitrary address.
	From string `json:"from"`
}

// handleGetMessages returns messages for a conversation.
func handleGetMessages(r *fastglue.Request) error {
	var (
		app     = r.Context.(*App)
		uuid    = r.RequestCtx.UserValue("uuid").(string)
		auser   = r.RequestCtx.UserValue("user").(amodels.User)
		total   = 0
		private *bool
	)
	page, pageSize := getPagination(r)

	// Parse optional private filter (null = no filter)
	if r.RequestCtx.QueryArgs().Has("private") {
		p := r.RequestCtx.QueryArgs().GetBool("private")
		private = &p
	}

	// Parse repeated type params: ?type=incoming&type=outgoing
	var msgTypes []string
	for _, v := range r.RequestCtx.QueryArgs().PeekMulti("type") {
		msgTypes = append(msgTypes, string(v))
	}

	user, err := app.user.GetAgent(auser.ID, "")
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	// Check permission
	_, err = enforceConversationAccess(app, uuid, user)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	messages, pageSize, err := app.conversation.GetConversationMessages(uuid, page, pageSize, private, msgTypes)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	rootURL, _ := app.setting.GetAppRootURL()
	for i := range messages {
		total = messages[i].Total
		// Populate attachment URLs
		for j := range messages[i].Attachments {
			att := messages[i].Attachments[j]
			messages[i].Attachments[j].URL = app.media.GetURL(att.UUID, att.ContentType, att.Name)
		}
		resolveContentCIDs(&messages[i], rootURL)
	}

	// Process CSAT status for all messages (will only affect CSAT messages)
	app.conversation.ProcessCSATStatus(messages)

	// Strip CSAT UUID from agent sessions to prevent self-rating.
	if r.RequestCtx.UserValue("auth_method") != "api_key" {
		for i := range messages {
			if messages[i].HasCSAT() {
				messages[i].StripCSATUUID()
			}
		}
	}

	return r.SendEnvelope(envelope.PageResults{
		Total:      total,
		Results:    messages,
		Page:       page,
		PerPage:    pageSize,
		TotalPages: (total + pageSize - 1) / pageSize,
	})
}

// handleGetMessage fetches a single from DB using the uuid.
func handleGetMessage(r *fastglue.Request) error {
	var (
		app   = r.Context.(*App)
		uuid  = r.RequestCtx.UserValue("uuid").(string)
		cuuid = r.RequestCtx.UserValue("cuuid").(string)
		auser = r.RequestCtx.UserValue("user").(amodels.User)
	)
	user, err := app.user.GetAgent(auser.ID, "")
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	// Check permission
	_, err = enforceConversationAccess(app, cuuid, user)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	message, err := app.conversation.GetMessage(uuid)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	// Process CSAT status for the message (will only affect CSAT messages)
	messages := []cmodels.Message{message}
	app.conversation.ProcessCSATStatus(messages)
	message = messages[0]

	// Strip CSAT UUID from agent sessions to prevent self-rating.
	if r.RequestCtx.UserValue("auth_method") != "api_key" && message.HasCSAT() {
		message.StripCSATUUID()
	}

	rootURL, _ := app.setting.GetAppRootURL()
	for j := range message.Attachments {
		att := message.Attachments[j]
		message.Attachments[j].URL = app.media.GetURL(att.UUID, att.ContentType, att.Name)
	}
	resolveContentCIDs(&message, rootURL)

	return r.SendEnvelope(message)
}

// handleRetryMessage changes message status to `pending`, so it's enqueued for sending.
func handleRetryMessage(r *fastglue.Request) error {
	var (
		app   = r.Context.(*App)
		uuid  = r.RequestCtx.UserValue("uuid").(string)
		cuuid = r.RequestCtx.UserValue("cuuid").(string)
		auser = r.RequestCtx.UserValue("user").(amodels.User)
	)

	user, err := app.user.GetAgent(auser.ID, "")
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	// Check permission
	_, err = enforceConversationAccess(app, cuuid, user)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	// Only outgoing agent messages that have failed can be retried.
	msg, err := app.conversation.GetMessage(uuid)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	if msg.SenderType != cmodels.SenderTypeAgent || msg.Status != cmodels.MessageStatusFailed || msg.SenderID != user.ID || msg.ConversationUUID != cuuid {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("globals.messages.badRequest"), nil, envelope.InputError)
	}

	if err = app.conversation.MarkMessageAsPending(uuid); err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(true)
}

// loadOwnedPrivateNote enforces conversation access, fetches the message and
// confirms that it is a private note authored by `user`. Returns the message on
// success or an envelope-wrapped error suitable to bubble up from a handler.
// Centralised so the ownership rule lives in one place across edit/delete.
func loadOwnedPrivateNote(app *App, cuuid, msgUUID string, user umodels.User) (cmodels.Message, error) {
	if _, err := enforceConversationAccess(app, cuuid, user); err != nil {
		return cmodels.Message{}, err
	}
	msg, err := app.conversation.GetMessage(msgUUID)
	if err != nil {
		return cmodels.Message{}, err
	}
	if msg.ConversationUUID != cuuid || !msg.Private || msg.SenderType != cmodels.SenderTypeAgent || msg.SenderID != user.ID {
		return cmodels.Message{}, envelope.NewError(envelope.PermissionError, app.i18n.T("status.deniedPermission"), nil)
	}
	return msg, nil
}

// handleUpdatePrivateNote updates the body of a private note. Only the
// agent who authored the note can edit it.
func handleUpdatePrivateNote(r *fastglue.Request) error {
	var (
		app   = r.Context.(*App)
		uuid  = r.RequestCtx.UserValue("uuid").(string)
		cuuid = r.RequestCtx.UserValue("cuuid").(string)
		auser = r.RequestCtx.UserValue("user").(amodels.User)
		req   struct {
			Content string `json:"content"`
		}
	)

	user, err := app.user.GetAgent(auser.ID, "")
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	if err := r.Decode(&req, "json"); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("globals.messages.badRequest"), nil, envelope.InputError)
	}
	if strings.TrimSpace(req.Content) == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.Ts("globals.messages.empty", "name", "`content`"), nil, envelope.InputError)
	}

	if _, err := loadOwnedPrivateNote(app, cuuid, uuid, user); err != nil {
		return sendErrorEnvelope(r, err)
	}

	// Sanitize HTML to prevent stored XSS/HTML injection.
	req.Content = stringutil.SanitizeHTML(req.Content)
	if err := app.conversation.UpdatePrivateNote(uuid, req.Content); err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(true)
}

// handleDeletePrivateNote soft-deletes a private note, replacing the body
// with a tombstone that records who deleted it. Only the original author
// can delete their own note.
func handleDeletePrivateNote(r *fastglue.Request) error {
	var (
		app   = r.Context.(*App)
		uuid  = r.RequestCtx.UserValue("uuid").(string)
		cuuid = r.RequestCtx.UserValue("cuuid").(string)
		auser = r.RequestCtx.UserValue("user").(amodels.User)
	)

	user, err := app.user.GetAgent(auser.ID, "")
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	if _, err := loadOwnedPrivateNote(app, cuuid, uuid, user); err != nil {
		return sendErrorEnvelope(r, err)
	}

	actorName := strings.TrimSpace(user.FirstName + " " + user.LastName)
	if err := app.conversation.SoftDeletePrivateNote(uuid, actorName); err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(true)
}

// handleSendMessage sends a message in a conversation.
func handleSendMessage(r *fastglue.Request) error {
	var (
		app   = r.Context.(*App)
		auser = r.RequestCtx.UserValue("user").(amodels.User)
		cuuid = r.RequestCtx.UserValue("cuuid").(string)
		req   = messageReq{}
	)

	user, err := app.user.GetAgent(auser.ID, "")
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	// Check access to conversation.
	conv, err := enforceConversationAccess(app, cuuid, user)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	if err := r.Decode(&req, "json"); err != nil {
		app.lo.Error("error unmarshalling message request", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, app.i18n.T("errors.parsingRequest"), nil, envelope.InputError)
	}

	// Make sure the inbox is enabled.
	inbox, err := app.inbox.GetDBRecord(conv.InboxID)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	if !inbox.Enabled {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("status.disabledInbox"), nil, envelope.InputError)
	}

	if req.SenderType != umodels.UserTypeAgent && req.SenderType != umodels.UserTypeContact {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("globals.messages.somethingWentWrong"), nil, envelope.InputError)
	}

	// Reject duplicate submissions early. Only guard agent sends — contact
	// sends come from external integrations where idempotency is the
	// caller's responsibility, and private notes don't go to a customer
	// so a stray duplicate is harmless. The set_status field is part of
	// the dedup key so EC1's dropdown variants don't collide with a
	// preceding plain Send.
	if req.SenderType == umodels.UserTypeAgent && !req.Private && len(req.ForwardedTo) == 0 {
		if checkMessageDedup(user.ID, cuuid, req.Message, req.SetStatus, req.From) {
			app.lo.Warn("duplicate message rejected", "user_id", user.ID, "conversation_uuid", cuuid, "set_status", req.SetStatus)
			return r.SendErrorEnvelope(fasthttp.StatusConflict, app.i18n.T("replyBox.duplicateMessage"), nil, envelope.InputError)
		}
	}

	// Contacts cannot send private messages
	if req.SenderType == umodels.UserTypeContact && req.Private {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("globals.messages.badRequest"), nil, envelope.InputError)
	}

	// Check if user has permission to send messages as contact
	if req.SenderType == umodels.UserTypeContact {
		parts := strings.Split(authzModels.PermMessagesWriteAsContact, ":")
		if len(parts) != 2 {
			app.lo.Error("error parsing permission string", "permission", authzModels.PermMessagesWriteAsContact)
			return sendErrorEnvelope(r, envelope.NewError(envelope.InputError, app.i18n.T("globals.messages.somethingWentWrong"), nil))
		}
		ok, err := app.authz.Enforce(user, parts[0], parts[1])
		if err != nil {
			app.lo.Error("error checking permission", "error", err)
			return sendErrorEnvelope(r, envelope.NewError(envelope.InputError, app.i18n.T("globals.messages.somethingWentWrong"), nil))
		}
		if !ok {
			return r.SendErrorEnvelope(fasthttp.StatusForbidden, app.i18n.T("status.deniedPermission"), nil, envelope.PermissionError)
		}
	}

	// Get media for all attachments, skip any already associated with a model.
	media, err := getUnassociatedMedia(app, req.Attachments)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, app.i18n.T("globals.messages.somethingWentWrong"), nil, envelope.GeneralError)
	}

	// Create contact message.
	if req.SenderType == umodels.UserTypeContact {
		message, err := app.conversation.CreateContactMessage(media, int(conv.ContactID), cuuid, req.Message, cmodels.ContentTypeHTML, false)
		if err != nil {
			return sendErrorEnvelope(r, err)
		}
		return r.SendEnvelope(message)
	}

	// Send private note.
	if req.Private {
		message, err := app.conversation.SendPrivateNote(media, user.ID, cuuid, req.Message, req.Mentions)
		if err != nil {
			return sendErrorEnvelope(r, err)
		}
		return r.SendEnvelope(message)
	}

	meta := map[string]any{}
	if req.EchoID != "" {
		meta["echo_id"] = req.EchoID
	}

	// EC14: validate the per-message From override. Only allow values that
	// match either the inbox's primary From or one of the configured
	// aliases — otherwise an attacker who can post a reply could spoof
	// any From address. We compare against the bare email part so admins
	// can configure aliases either as bare addresses or with display
	// names; either form on either side matches.
	if req.From != "" {
		validFrom, err := validateInboxFromOverride(inbox, req.From)
		if err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, err.Error(), nil, envelope.InputError)
		}
		meta["from"] = validFrom
	}

	// Forward mode: route to the forwarded_to recipients instead of req.To.
	// CC/BCC pass through unchanged so an agent can loop teammates in.
	sendTo := req.To
	if len(req.ForwardedTo) > 0 {
		meta["forwarded"] = true
		meta["forwarded_to"] = req.ForwardedTo
		sendTo = req.ForwardedTo
	}

	message, err := app.conversation.QueueReply(media, conv.InboxID, user.ID, conv.ContactID, cuuid, req.Message, sendTo, req.CC, req.BCC, meta)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	// Log a forward activity note so the audit trail captures who forwarded
	// to whom. Best-effort: a logging failure shouldn't break the send.
	if len(req.ForwardedTo) > 0 {
		recipients := strings.Join(req.ForwardedTo, ", ")
		if err := app.conversation.InsertConversationActivity(cmodels.ActivityMessageForwarded, cuuid, recipients, user); err != nil {
			app.lo.Warn("failed to insert forward activity note", "error", err, "conversation_uuid", cuuid)
		}
	}

	// EC1: "Send & Set Status" dropdown. After the reply is queued,
	// transition the conversation status in the same request so the agent
	// gets a single-action send-and-resolve. Don't fail the response, since
	// the reply itself landed and the agent can flip status manually — but
	// log at Error (this is an unmet user-facing guarantee) and surface the
	// error string back via the envelope so the frontend can toast a warning
	// like "Reply sent, but couldn't set status to Resolved". Without this,
	// "Send & Resolve" silently degrades to "Send" and the agent never knows.
	var setStatusErr string
	if req.SetStatus != "" {
		if err := app.conversation.UpdateConversationStatus(cuuid, 0, req.SetStatus, "", user); err != nil {
			app.lo.Error("failed to set status after send", "error", err, "conversation_uuid", cuuid, "set_status", req.SetStatus)
			setStatusErr = err.Error()
		}
	}

	if setStatusErr != "" {
		// Inline the message fields plus a non-fatal set_status_error so
		// existing consumers reading the message (e.g. echo replacement) still
		// see what they expect — only the new field is additive.
		return r.SendEnvelope(map[string]any{
			"message":          message,
			"set_status_error": setStatusErr,
			"set_status":       req.SetStatus,
		})
	}
	return r.SendEnvelope(message)
}

// validateInboxFromOverride enforces that a per-message From override (EC14
// reply-box From switcher) matches one of the inbox's configured addresses —
// the primary `from` field or one of the `aliases` entries in the JSONB
// config. Comparison is by parsed bare email so admins can mix bare and
// display-name forms freely on either side. Returns the trusted From string
// to thread into the message meta, or an error envelope-friendly message.
func validateInboxFromOverride(inbox imodels.Inbox, requested string) (string, error) {
	requestedAddr, err := mail.ParseAddress(requested)
	if err != nil {
		return "", fmt.Errorf("invalid From address: %v", err)
	}
	// Don't lower-case here — the comparison below uses strings.EqualFold,
	// which is case-insensitive on its own. Keeping the bare TrimSpace
	// avoids a redundant allocation on the hot path.
	requestedEmail := strings.TrimSpace(requestedAddr.Address)

	// Build the allow-list: primary From + aliases. The primary From is on
	// the inbox row; aliases live inside the JSONB config column.
	candidates := []string{inbox.From}
	if len(inbox.Config) > 0 {
		var cfg imodels.Config
		if err := json.Unmarshal(inbox.Config, &cfg); err == nil {
			candidates = append(candidates, cfg.Aliases...)
		}
	}

	for _, c := range candidates {
		if c == "" {
			continue
		}
		addr, err := mail.ParseAddress(c)
		if err != nil {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(addr.Address), requestedEmail) {
			// Return the agent's chosen string verbatim — preserves any
			// display-name override they may have typed.
			return requested, nil
		}
	}
	return "", fmt.Errorf("From address not allowed for this inbox")
}

// resolveContentCIDs replaces inline image cid: references in email message content
// with actual attachment URLs and resolves relative /uploads/ paths to absolute URLs.
func resolveContentCIDs(msg *cmodels.Message, rootURL string) {
	for _, att := range msg.Attachments {
		if att.ContentID != "" && att.URL != "" {
			msg.Content = strings.ReplaceAll(msg.Content, "cid:"+att.ContentID, att.URL)
		}
	}
	if rootURL != "" {
		msg.Content = strings.ReplaceAll(msg.Content, `src="/uploads/`, `src="`+rootURL+`/uploads/`)
		msg.Content = strings.ReplaceAll(msg.Content, `src='/uploads/`, `src='`+rootURL+`/uploads/`)
	}
}

// handleRedactMessagePCI is the manual "Redact Now" path: agents click the
// banner button on a flagged message to scrub it immediately rather than
// waiting for the 7-day auto-redact safety net. The handler enforces
// conversation access, scrubs the DB row, then best-effort deletes the
// original from IMAP via the inbox manager. IMAP failure is logged and
// notified to the configured PCI admin agent (PCISettings) but does NOT
// roll back the in-DB redaction — the primary in-app exposure is closed
// the moment the row is rewritten.
//
// Activity note is inserted in either branch so the conversation audit
// trail records the redaction regardless of IMAP outcome.
func handleRedactMessagePCI(r *fastglue.Request) error {
	var (
		app   = r.Context.(*App)
		uuid  = r.RequestCtx.UserValue("uuid").(string)
		cuuid = r.RequestCtx.UserValue("cuuid").(string)
		auser = r.RequestCtx.UserValue("user").(amodels.User)
	)

	user, err := app.user.GetAgent(auser.ID, "")
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	if _, err := enforceConversationAccess(app, cuuid, user); err != nil {
		return sendErrorEnvelope(r, err)
	}

	msg, err := app.conversation.RedactMessagePCI(uuid)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	actorName := strings.TrimSpace(user.FirstName + " " + user.LastName)
	if actorName == "" {
		actorName = user.Email.String
	}

	if msg.SourceID.Valid && msg.SourceID.String != "" {
		if err := app.inbox.DeleteIMAPMessage(msg.InboxID, msg.SourceID.String); err != nil {
			app.lo.Error("failed to delete PCI email from IMAP after manual redact", "error", err, "message_uuid", uuid)
			app.conversation.InsertPCIRedactActivityNote(cuuid, actorName, false,
				"Card data was redacted but the original email could not be deleted from Gmail. Please delete manually.")
			app.conversation.NotifyPCIIMAPDeleteFailed(cuuid, uuid)
		} else {
			app.conversation.InsertPCIRedactActivityNote(cuuid, actorName, true, "")
		}
	} else {
		app.conversation.InsertPCIRedactActivityNote(cuuid, actorName, true, "")
	}

	return r.SendEnvelope(true)
}

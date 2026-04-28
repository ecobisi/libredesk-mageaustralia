package main

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	amodels "github.com/abhinavxd/libredesk/internal/auth/models"
	authzModels "github.com/abhinavxd/libredesk/internal/authz/models"
	cmodels "github.com/abhinavxd/libredesk/internal/conversation/models"
	"github.com/abhinavxd/libredesk/internal/envelope"
	"github.com/abhinavxd/libredesk/internal/stringutil"
	medModels "github.com/abhinavxd/libredesk/internal/media/models"
	umodels "github.com/abhinavxd/libredesk/internal/user/models"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

// messageDedupMap prevents duplicate message submissions within a short window.
var messageDedupMap sync.Map

const messageDedupTTL = 60 * time.Second

// checkMessageDedup returns true if this message was already sent recently (duplicate).
func checkMessageDedup(userID int, convUUID, content string) bool {
	h := sha256.Sum256([]byte(content))
	key := fmt.Sprintf("%d:%s:%x", userID, convUUID, h[:8])

	if _, loaded := messageDedupMap.LoadOrStore(key, time.Now()); loaded {
		return true // duplicate
	}

	// Clean up after TTL
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
	InboxID     int                    `json:"inbox_id"`
	ForwardedTo []string               `json:"forwarded_to"`
}

// refreshContentUploadURLs replaces expired signed upload URLs in message HTML content
// with freshly signed ones. This is needed because vue-letter renders content in an iframe
// (srcdoc) which doesn't share session cookies, so images need valid signed URLs.
func refreshContentUploadURLs(app *App, content string) string {
	if !strings.Contains(content, "/uploads/") {
		return content
	}
	// Match /uploads/UUID with optional query params, or full absolute URL with /uploads/UUID
	re := regexp.MustCompile(`(https?://[^"'\s]*/uploads/|/uploads/)([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})(?:\?[^"'\s>]*)?`)
	return re.ReplaceAllStringFunc(content, func(match string) string {
		subs := re.FindStringSubmatch(match)
		if len(subs) < 3 {
			return match
		}
		uuid := subs[2]
		// Generate a fresh signed URL
		return app.media.GetURL(uuid, "", "")
	})
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

	for i := range messages {
		total = messages[i].Total
		// Populate attachment URLs
		for j := range messages[i].Attachments {
			att := messages[i].Attachments[j]
			messages[i].Attachments[j].URL = app.media.GetURL(att.UUID, att.ContentType, att.Name)
		}
		// Refresh signed URLs in inline content (iframe can't use session cookies)
		messages[i].Content = refreshContentUploadURLs(app, messages[i].Content)
		// Redact CSAT survey link
		messages[i].CensorCSATContent()
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

	// Refresh signed URLs in inline content
	message.Content = refreshContentUploadURLs(app, message.Content)

	// Redact CSAT survey link
	message.CensorCSATContent()

	for j := range message.Attachments {
		att := message.Attachments[j]
		message.Attachments[j].URL = app.media.GetURL(att.UUID, att.ContentType, att.Name)
	}

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
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, app.i18n.Ts("globals.messages.errorParsing", "name", "{globals.terms.request}"), nil, envelope.InputError)
	}

	if req.SenderType != umodels.UserTypeAgent && req.SenderType != umodels.UserTypeContact {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.Ts("globals.messages.invalid", "name", "`sender_type`"), nil, envelope.InputError)
	}

	// Prevent duplicate message submissions (same user, same conversation, same content within 10s).
	if checkMessageDedup(user.ID, cuuid, req.Message) {
		app.lo.Warn("duplicate message rejected", "user_id", user.ID, "conversation_uuid", cuuid)
		return r.SendErrorEnvelope(fasthttp.StatusConflict, "Duplicate message", nil, envelope.InputError)
	}

	// Contacts cannot send private messages
	if req.SenderType == umodels.UserTypeContact && req.Private {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("globals.messages.badRequest"), nil, envelope.InputError)
	}

	// Check if user has permission to send messages as contact
	if req.SenderType == umodels.UserTypeContact {
		parts := strings.Split(authzModels.PermMessagesWriteAsContact, ":")
		if len(parts) != 2 {
			return sendErrorEnvelope(r, envelope.NewError(envelope.InputError, app.i18n.Ts("globals.messages.errorChecking", "name", "{globals.terms.permission}"), nil))
		}
		ok, err := app.authz.Enforce(user, parts[0], parts[1])
		if err != nil {
			return sendErrorEnvelope(r, envelope.NewError(envelope.InputError, app.i18n.Ts("globals.messages.errorChecking", "name", "{globals.terms.permission}"), nil))
		}
		if !ok {
			return r.SendErrorEnvelope(fasthttp.StatusForbidden, app.i18n.Ts("globals.messages.denied", "name", "{globals.terms.permission}"), nil, envelope.PermissionError)
		}
	}

	// Get media for all attachments.
	var media = make([]medModels.Media, 0, len(req.Attachments))
	for _, id := range req.Attachments {
		m, err := app.media.Get(id, "")
		if err != nil {
			app.lo.Error("error fetching media", "error", err)
			return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, app.i18n.Ts("globals.messages.errorFetching", "name", "{globals.terms.media}"), nil, envelope.GeneralError)
		}
		if m.ModelID.Int > 0 {
			// Attachment is already associated with another model. Skip it.
			app.lo.Warn("attachment already associated with another model, skipping", "media_id", m.ID, "model", m.Model.String, "model_id", m.ModelID.Int)
			continue
		}
		media = append(media, m)
	}

	// Create contact message.
	if req.SenderType == umodels.UserTypeContact {
		message, err := app.conversation.CreateContactMessage(media, int(conv.ContactID), cuuid, req.Message, cmodels.ContentTypeHTML)
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

	// Use the requested inbox ID if provided, otherwise default to conversation's inbox.
	inboxID := conv.InboxID
	if req.InboxID > 0 && req.InboxID != conv.InboxID {
		// Validate the requested inbox exists and is enabled.
		inboxRecord, err := app.inbox.GetDBRecord(req.InboxID)
		if err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid inbox", nil, envelope.InputError)
		}
		if !inboxRecord.Enabled {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Inbox is disabled", nil, envelope.InputError)
		}
		inboxID = req.InboxID
	}

	// Handle forward: set meta and override recipients
	meta := map[string]any{}
	sendTo := req.To
	sendCC := req.CC
	sendBCC := req.BCC
	if len(req.ForwardedTo) > 0 {
		meta["forwarded"] = true
		meta["forwarded_to"] = req.ForwardedTo
		sendTo = req.ForwardedTo
		// CC/BCC pass through unchanged — agents sometimes want to loop
		// internal teammates in on a forward; stripping was silently
		// dropping their chosen recipients.
	}

	// Queue reply.
	message, err := app.conversation.QueueReply(media, inboxID, user.ID, cuuid, req.Message, sendTo, sendCC, sendBCC, meta)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	// Insert activity note for forwarded messages.
	if len(req.ForwardedTo) > 0 {
		actorName := strings.TrimSpace(user.FirstName + " " + user.LastName)
		recipients := strings.Join(req.ForwardedTo, ", ")
		app.conversation.InsertForwardActivityNote(cuuid, actorName, recipients, user.ID)
	}

	return r.SendEnvelope(message)
}

// handleUpdatePrivateNote updates the content of a private note.
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

	if _, err = enforceConversationAccess(app, cuuid, user); err != nil {
		return sendErrorEnvelope(r, err)
	}

	if err := r.Decode(&req, "json"); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("globals.messages.badRequest"), nil, envelope.InputError)
	}

	if strings.TrimSpace(req.Content) == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.Ts("globals.messages.empty", "name", "`content`"), nil, envelope.InputError)
	}

	// Sanitize HTML to prevent stored XSS/HTML injection.
	req.Content = stringutil.SanitizeHTML(req.Content)
	if err := app.conversation.UpdatePrivateNote(uuid, req.Content); err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(true)
}

// handleDeletePrivateNote soft-deletes a private note.
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

	if _, err = enforceConversationAccess(app, cuuid, user); err != nil {
		return sendErrorEnvelope(r, err)
	}

	actorName := strings.TrimSpace(user.FirstName + " " + user.LastName)
	if err := app.conversation.SoftDeletePrivateNote(uuid, actorName); err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(true)
}

// handleRedactMessagePCI scrubs PCI (credit card) data from a message.
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

	// Check permission.
	_, err = enforceConversationAccess(app, cuuid, user)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	// Redact PCI data.
	msg, err := app.conversation.RedactMessagePCI(uuid)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	actorName := user.FirstName + " " + user.LastName

	// Try to delete from IMAP.
	if msg.SourceID.Valid && msg.SourceID.String != "" {
		if err := app.inbox.DeleteIMAPMessage(msg.InboxID, msg.SourceID.String); err != nil {
			app.lo.Error("failed to delete PCI email from IMAP after manual redact", "error", err)
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

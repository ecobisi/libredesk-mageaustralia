package main

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	amodels "github.com/abhinavxd/libredesk/internal/auth/models"
	"github.com/abhinavxd/libredesk/internal/envelope"
	notifier "github.com/abhinavxd/libredesk/internal/notification"
	nmodels "github.com/abhinavxd/libredesk/internal/notification/models"
	"github.com/abhinavxd/libredesk/internal/stringutil"
	"github.com/abhinavxd/libredesk/internal/user/models"
	"github.com/valyala/fasthttp"
	"github.com/volatiletech/null/v9"
	"github.com/zerodha/fastglue"
)

type createContactNoteReq struct {
	Note          string `json:"note"`
	NotifyUserIDs []int  `json:"notify_user_ids,omitempty"`
}

type blockContactReq struct {
	Enabled bool `json:"enabled"`
}

type quickCreateContactReq struct {
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

// handleGetContacts returns a list of contacts from the database.
func handleGetContacts(r *fastglue.Request) error {
	var (
		app     = r.Context.(*App)
		order   = string(r.RequestCtx.QueryArgs().Peek("order"))
		orderBy = string(r.RequestCtx.QueryArgs().Peek("order_by"))
		filters = string(r.RequestCtx.QueryArgs().Peek("filters"))
		total   = 0
	)
	page, pageSize := getPagination(r)
	contacts, err := app.user.GetContacts(page, pageSize, order, orderBy, filters)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	if len(contacts) > 0 {
		total = contacts[0].Total
	}
	return r.SendEnvelope(envelope.PageResults{
		Results:    contacts,
		Total:      total,
		PerPage:    pageSize,
		TotalPages: (total + pageSize - 1) / pageSize,
		Page:       page,
	})
}

// handleGetTags returns a contact from the database.
func handleGetContact(r *fastglue.Request) error {
	var (
		app   = r.Context.(*App)
		id, _ = strconv.Atoi(r.RequestCtx.UserValue("id").(string))
	)
	if id <= 0 {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("globals.messages.somethingWentWrong"), nil, envelope.InputError)
	}
	c, err := app.user.GetContactOrVisitor(id, "")
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(c)
}

// handleUpdateContact updates a contact in the database.
func handleUpdateContact(r *fastglue.Request) error {
	var (
		app   = r.Context.(*App)
		id, _ = strconv.Atoi(r.RequestCtx.UserValue("id").(string))
	)
	if id <= 0 {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("globals.messages.somethingWentWrong"), nil, envelope.InputError)
	}

	contact, err := app.user.GetContactOrVisitor(id, "")
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	form, err := r.RequestCtx.MultipartForm()
	if err != nil {
		app.lo.Error("error parsing form data", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, app.i18n.T("errors.parsingRequest"), nil, envelope.GeneralError)
	}

	// Parse form data
	firstName := ""
	if v, ok := form.Value["first_name"]; ok && len(v) > 0 {
		firstName = string(v[0])
	}
	lastName := ""
	if v, ok := form.Value["last_name"]; ok && len(v) > 0 {
		lastName = string(v[0])
	}
	email := ""
	if v, ok := form.Value["email"]; ok && len(v) > 0 {
		email = strings.TrimSpace(string(v[0]))
	}
	phoneNumber := ""
	if v, ok := form.Value["phone_number"]; ok && len(v) > 0 {
		phoneNumber = string(v[0])
	}
	phoneNumberCountryCode := ""
	if v, ok := form.Value["phone_number_country_code"]; ok && len(v) > 0 {
		phoneNumberCountryCode = string(v[0])
	}
	country := ""
	if v, ok := form.Value["country"]; ok && len(v) > 0 {
		country = string(v[0])
	}
	avatarURL := ""
	if v, ok := form.Value["avatar_url"]; ok && len(v) > 0 {
		avatarURL = string(v[0])
	}

	// Set nulls to empty strings.
	if avatarURL == "null" {
		avatarURL = ""
	}
	if phoneNumberCountryCode == "null" {
		phoneNumberCountryCode = ""
	}
	if phoneNumber == "null" {
		phoneNumber = ""
	}
	if country == "null" {
		country = ""
	}

	// Validate mandatory fields.
	if email == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.Ts("globals.messages.empty", "name", "email"), nil, envelope.InputError)
	}
	if !stringutil.ValidEmail(email) {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("validation.invalidEmail"), nil, envelope.InputError)
	}
	if firstName == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.Ts("globals.messages.empty", "name", "first_name"), nil, envelope.InputError)
	}

	contactToUpdate := models.User{
		FirstName:              firstName,
		LastName:               lastName,
		Email:                  null.StringFrom(email),
		AvatarURL:              null.NewString(avatarURL, avatarURL != ""),
		PhoneNumber:            null.NewString(phoneNumber, phoneNumber != ""),
		PhoneNumberCountryCode: null.NewString(phoneNumberCountryCode, phoneNumberCountryCode != ""),
		Country:                null.NewString(country, country != ""),
	}

	if err := app.user.UpdateContact(id, contactToUpdate); err != nil {
		return sendErrorEnvelope(r, err)
	}

	// Delete avatar?
	if avatarURL == "" && contact.AvatarURL.Valid {
		fileName := filepath.Base(contact.AvatarURL.String)
		app.media.Delete(fileName)
		contact.AvatarURL.Valid = false
		contact.AvatarURL.String = ""
	}

	// Upload avatar?
	files, ok := form.File["files"]
	if ok && len(files) > 0 {
		if err := uploadUserAvatar(r, contact, files); err != nil {
			return sendErrorEnvelope(r, err)
		}
	}

	// Refetch contact and return it
	contact, err = app.user.GetContactOrVisitor(id, "")
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(contact)
}

// handleGetContactNotes returns all notes for a contact.
func handleGetContactNotes(r *fastglue.Request) error {
	var (
		app          = r.Context.(*App)
		contactID, _ = strconv.Atoi(r.RequestCtx.UserValue("id").(string))
	)
	if contactID <= 0 {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("globals.messages.somethingWentWrong"), nil, envelope.InputError)
	}
	notes, err := app.user.GetNotes(contactID)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(notes)
}

// handleCreateContactNote creates a note for a contact. If the request also
// carries notify_user_ids the agent gets a Freshdesk-style "@-mention without
// the @" — the listed agents receive an in-app + email notification with the
// note body excerpted as plain text. Useful for a CS lead leaving a note on
// a VIP contact and pinging the account manager without having to switch to
// a conversation just to drop a mention.
func handleCreateContactNote(r *fastglue.Request) error {
	var (
		app          = r.Context.(*App)
		contactID, _ = strconv.Atoi(r.RequestCtx.UserValue("id").(string))
		auser        = r.RequestCtx.UserValue("user").(amodels.User)
		req          = createContactNoteReq{}
	)
	if err := r.Decode(&req, "json"); err != nil {
		return sendErrorEnvelope(r, envelope.NewError(envelope.InputError, app.i18n.T("errors.parsingRequest"), nil))
	}
	if len(req.Note) == 0 {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.Ts("globals.messages.empty", "name", "note"), nil, envelope.InputError)
	}
	// Sanitize HTML to prevent stored XSS/HTML injection.
	req.Note = stringutil.SanitizeHTML(req.Note)
	n, err := app.user.CreateNote(contactID, auser.ID, req.Note)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	n, err = app.user.GetNote(n.ID)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	// Dispatch the recipient notifications off the request goroutine so a
	// slow SMTP server can't block the API response. Failures are logged in
	// the dispatcher but never surfaced to the agent — the note is already
	// saved at this point and a missed notification is recoverable.
	if len(req.NotifyUserIDs) > 0 {
		go notifyContactNote(app, contactID, auser.ID, req.Note, req.NotifyUserIDs)
	}

	return r.SendEnvelope(n)
}

// notifyContactNote fans out an in-app + email notification to each agent the
// note author selected from the recipients picker. Self-notifications are
// dropped (no one needs an email about their own action). Email body uses
// the SanitizeHTML-cleaned note as-is for the in-template blockquote and a
// plain-text excerpt (HTML2Text + 500-char truncation) for the in-app body.
func notifyContactNote(app *App, contactID, actorID int, noteHTML string, recipientIDs []int) {
	if app.notifDispatcher == nil {
		return
	}

	// De-dupe and skip the actor.
	uniq := make(map[int]struct{}, len(recipientIDs))
	for _, id := range recipientIDs {
		if id <= 0 || id == actorID {
			continue
		}
		uniq[id] = struct{}{}
	}
	if len(uniq) == 0 {
		return
	}

	contact, err := app.user.GetContactOrVisitor(contactID, "")
	if err != nil {
		app.lo.Warn("contact note notify: failed to fetch contact", "contact_id", contactID, "error", err)
		return
	}
	author, err := app.user.GetAgent(actorID, "")
	if err != nil {
		app.lo.Warn("contact note notify: failed to fetch author", "actor_id", actorID, "error", err)
		return
	}

	contactName := strings.TrimSpace(contact.FirstName + " " + contact.LastName)
	if contactName == "" {
		contactName = contact.Email.String
	}
	authorName := strings.TrimSpace(author.FirstName + " " + author.LastName)

	// Plain-text excerpt for the in-app body (the in-app surface doesn't
	// render HTML — long sanitised note bodies otherwise show as raw tags).
	excerpt := stringutil.HTML2Text(noteHTML)
	if len(excerpt) > 500 {
		excerpt = excerpt[:500] + "..."
	}

	// Resolve email addresses in the same loop so we can keep the per-
	// recipient email slice aligned with the recipient ID slice for
	// SendWithEmails. Recipients without an email get an empty
	// EmailNotification slot — the dispatcher then sends the in-app
	// notification only.
	var ids []int
	var emails []notifier.EmailNotification
	subject := app.i18n.Ts("notification.contactNote.subject", "contact", contactName)
	body := fmt.Sprintf("<p><strong>%s</strong> added a note on contact <strong>%s</strong>:</p><blockquote>%s</blockquote>",
		authorName, contactName, noteHTML)
	for id := range uniq {
		ids = append(ids, id)

		agent, err := app.user.GetAgent(id, "")
		if err != nil || !agent.Email.Valid || agent.Email.String == "" {
			emails = append(emails, notifier.EmailNotification{})
			continue
		}
		emails = append(emails, notifier.EmailNotification{
			Recipients: []string{agent.Email.String},
			Subject:    subject,
			Content:    body,
		})
	}

	app.notifDispatcher.SendWithEmails(notifier.Notification{
		Type:           nmodels.NotificationTypeContactNote,
		RecipientIDs:   ids,
		Title:          app.i18n.Ts("notification.contactNote.title", "author", authorName, "contact", contactName),
		Body:           null.StringFrom(excerpt),
		ActorID:        null.IntFrom(actorID),
		ActorFirstName: author.FirstName,
		ActorLastName:  author.LastName,
	}, emails)
}

// handleDeleteContactNote deletes a note for a contact.
func handleDeleteContactNote(r *fastglue.Request) error {
	var (
		app          = r.Context.(*App)
		contactID, _ = strconv.Atoi(r.RequestCtx.UserValue("id").(string))
		noteID, _    = strconv.Atoi(r.RequestCtx.UserValue("note_id").(string))
		auser        = r.RequestCtx.UserValue("user").(amodels.User)
	)
	if contactID <= 0 {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("globals.messages.somethingWentWrong"), nil, envelope.InputError)
	}
	if noteID <= 0 {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("globals.messages.somethingWentWrong"), nil, envelope.InputError)
	}

	agent, err := app.user.GetAgent(auser.ID, "")
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	// Allow deletion of only own notes and not those created by others, but also allow `Admin` to delete any note.
	if !agent.HasAdminRole() {
		note, err := app.user.GetNote(noteID)
		if err != nil {
			return sendErrorEnvelope(r, err)
		}
		if note.UserID != auser.ID {
			return r.SendErrorEnvelope(fasthttp.StatusForbidden, app.i18n.T("errors.canOnlyDeleteOwnNote"), nil, envelope.InputError)
		}
	}

	app.lo.Info("deleting contact note", "note_id", noteID, "contact_id", contactID, "actor_id", auser.ID)

	if err := app.user.DeleteNote(noteID, contactID); err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(true)
}

// handleBlockContact blocks a contact.
func handleBlockContact(r *fastglue.Request) error {
	var (
		app          = r.Context.(*App)
		contactID, _ = strconv.Atoi(r.RequestCtx.UserValue("id").(string))
		auser        = r.RequestCtx.UserValue("user").(amodels.User)
		req          = blockContactReq{}
	)

	if contactID <= 0 {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("globals.messages.somethingWentWrong"), nil, envelope.InputError)
	}

	if err := r.Decode(&req, "json"); err != nil {
		return sendErrorEnvelope(r, envelope.NewError(envelope.InputError, app.i18n.T("errors.parsingRequest"), nil))
	}

	app.lo.Info("setting contact block status", "contact_id", contactID, "enabled", req.Enabled, "actor_id", auser.ID)

	contact, err := app.user.GetContactOrVisitor(contactID, "")
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	if err := app.user.ToggleEnabled(contactID, contact.Type, req.Enabled); err != nil {
		return sendErrorEnvelope(r, err)
	}

	contact, err = app.user.GetContactOrVisitor(contactID, "")
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(contact)
}

// handleQuickCreateContact creates a contact from email/first/last name only
// (no phone, no avatar, no custom attributes). Used by the conversation
// sidebar "change contact" affordance when the agent searches for a contact
// that doesn't exist yet — a one-step inline create+assign instead of
// kicking the agent over to the full contacts admin form. Reuses the
// existing CreateContact upsert path, so submitting with an email that
// already belongs to a contact returns that contact's id rather than
// erroring.
func handleQuickCreateContact(r *fastglue.Request) error {
	var (
		app = r.Context.(*App)
		req = quickCreateContactReq{}
	)
	if err := r.Decode(&req, "json"); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("errors.parsingRequest"), nil, envelope.InputError)
	}

	email := strings.TrimSpace(req.Email)
	if email == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.Ts("globals.messages.empty", "name", "email"), nil, envelope.InputError)
	}
	if !stringutil.ValidEmail(email) {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("validation.invalidEmail"), nil, envelope.InputError)
	}

	contact := &models.User{
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Email:     null.StringFrom(email),
	}
	if err := app.user.CreateContact(contact); err != nil {
		app.lo.Error("error creating contact via quick-create", "error", err)
		return sendErrorEnvelope(r, envelope.NewError(envelope.GeneralError, app.i18n.T("globals.messages.somethingWentWrong"), nil))
	}

	created, err := app.user.GetContactOrVisitor(contact.ID, "")
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(created)
}

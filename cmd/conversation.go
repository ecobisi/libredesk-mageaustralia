package main

import (
	"encoding/json"
	"slices"
	"strconv"
	"strings"
	"time"

	amodels "github.com/abhinavxd/libredesk/internal/auth/models"
	authzModels "github.com/abhinavxd/libredesk/internal/authz/models"
	"github.com/abhinavxd/libredesk/internal/automation/models"
	cmodels "github.com/abhinavxd/libredesk/internal/conversation/models"
	"github.com/abhinavxd/libredesk/internal/envelope"
	"github.com/abhinavxd/libredesk/internal/stringutil"
	umodels "github.com/abhinavxd/libredesk/internal/user/models"
	vmodels "github.com/abhinavxd/libredesk/internal/view/models"
	wmodels "github.com/abhinavxd/libredesk/internal/webhook/models"
	"github.com/valyala/fasthttp"
	"github.com/volatiletech/null/v9"
	"github.com/zerodha/fastglue"
)

type assigneeChangeReq struct {
	AssigneeID int `json:"assignee_id"`
}

type teamAssigneeChangeReq struct {
	AssigneeID int `json:"assignee_id"`
}

type priorityUpdateReq struct {
	Priority string `json:"priority"`
}

type statusUpdateReq struct {
	Status       string `json:"status"`
	SnoozedUntil string `json:"snoozed_until,omitempty"`
}

type subjectUpdateReq struct {
	Subject string `json:"subject"`
}

type tagsUpdateReq struct {
	Tags []string `json:"tags"`
}

type createConversationRequest struct {
	InboxID          int            `json:"inbox_id"`
	AssignedAgentID  int            `json:"agent_id"`
	AssignedTeamID   int            `json:"team_id"`
	Email            string         `json:"contact_email"`
	FirstName        string         `json:"first_name"`
	LastName         string         `json:"last_name"`
	ExternalUserID   string         `json:"external_user_id"`
	Subject          string         `json:"subject"`
	Content          string         `json:"content"`
	// EC7: Gmail-style new conversation. Email is comma-separated TO list (the
	// first address is the primary contact used to look up / create the User
	// row); CC and BCC are optional comma-separated extras carried straight
	// through to QueueReply.
	CC               string         `json:"cc"`
	BCC              string         `json:"bcc"`
	Attachments      []int          `json:"attachments"`
	Initiator        string         `json:"initiator"` // "contact" | "agent"
	CustomAttributes map[string]any `json:"custom_attributes"`
	// EC20: Submit & Set Status. Optional follow-up status name to apply
	// after the conversation is created. Mirrors the reply-box "Send & Set
	// Status" picker. Empty = leave as Open (default for new conversations).
	SetStatus string `json:"set_status,omitempty"`
}

// handleGetAllConversations retrieves all conversations.
func handleGetAllConversations(r *fastglue.Request) error {
	var (
		app     = r.Context.(*App)
		user    = r.RequestCtx.UserValue("user").(amodels.User)
		order   = string(r.RequestCtx.QueryArgs().Peek("order"))
		orderBy = string(r.RequestCtx.QueryArgs().Peek("order_by"))
		filters = string(r.RequestCtx.QueryArgs().Peek("filters"))
		total   = 0
	)
	page, pageSize := getPagination(r)

	conversations, err := app.conversation.GetAllConversationsList(user.ID, order, orderBy, filters, page, pageSize)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	if len(conversations) > 0 {
		total = conversations[0].Total
	}

	return r.SendEnvelope(envelope.PageResults{
		Results:    conversations,
		Total:      total,
		PerPage:    pageSize,
		TotalPages: (total + pageSize - 1) / pageSize,
		Page:       page,
	})
}

// handleGetAssignedConversations retrieves conversations assigned to the current user.
func handleGetAssignedConversations(r *fastglue.Request) error {
	var (
		app     = r.Context.(*App)
		user    = r.RequestCtx.UserValue("user").(amodels.User)
		order   = string(r.RequestCtx.QueryArgs().Peek("order"))
		orderBy = string(r.RequestCtx.QueryArgs().Peek("order_by"))
		filters = string(r.RequestCtx.QueryArgs().Peek("filters"))
		total   = 0
	)
	page, pageSize := getPagination(r)
	conversations, err := app.conversation.GetAssignedConversationsList(user.ID, user.ID, order, orderBy, filters, page, pageSize)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	if len(conversations) > 0 {
		total = conversations[0].Total
	}

	return r.SendEnvelope(envelope.PageResults{
		Results:    conversations,
		Total:      total,
		PerPage:    pageSize,
		TotalPages: (total + pageSize - 1) / pageSize,
		Page:       page,
	})
}

// handleGetUnassignedConversations retrieves unassigned conversations.
func handleGetUnassignedConversations(r *fastglue.Request) error {
	var (
		app     = r.Context.(*App)
		user    = r.RequestCtx.UserValue("user").(amodels.User)
		order   = string(r.RequestCtx.QueryArgs().Peek("order"))
		orderBy = string(r.RequestCtx.QueryArgs().Peek("order_by"))
		filters = string(r.RequestCtx.QueryArgs().Peek("filters"))
		total   = 0
	)
	page, pageSize := getPagination(r)

	conversations, err := app.conversation.GetUnassignedConversationsList(user.ID, order, orderBy, filters, page, pageSize)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	if len(conversations) > 0 {
		total = conversations[0].Total
	}

	return r.SendEnvelope(envelope.PageResults{
		Results:    conversations,
		Total:      total,
		PerPage:    pageSize,
		TotalPages: (total + pageSize - 1) / pageSize,
		Page:       page,
	})
}

// handleGetMentionedConversations retrieves conversations where the current user is mentioned.
func handleGetMentionedConversations(r *fastglue.Request) error {
	var (
		app     = r.Context.(*App)
		user    = r.RequestCtx.UserValue("user").(amodels.User)
		order   = string(r.RequestCtx.QueryArgs().Peek("order"))
		orderBy = string(r.RequestCtx.QueryArgs().Peek("order_by"))
		filters = string(r.RequestCtx.QueryArgs().Peek("filters"))
		total   = 0
	)
	page, pageSize := getPagination(r)

	conversations, err := app.conversation.GetMentionedConversationsList(user.ID, order, orderBy, filters, page, pageSize)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	if len(conversations) > 0 {
		total = conversations[0].Total
	}

	return r.SendEnvelope(envelope.PageResults{
		Results:    conversations,
		Total:      total,
		PerPage:    pageSize,
		TotalPages: (total + pageSize - 1) / pageSize,
		Page:       page,
	})
}

// handleGetViewConversations retrieves conversations for a view.
func handleGetViewConversations(r *fastglue.Request) error {
	var (
		app       = r.Context.(*App)
		auser     = r.RequestCtx.UserValue("user").(amodels.User)
		viewID, _ = strconv.Atoi(r.RequestCtx.UserValue("id").(string))
		order     = string(r.RequestCtx.QueryArgs().Peek("order"))
		orderBy   = string(r.RequestCtx.QueryArgs().Peek("order_by"))
		total     = 0
	)
	page, pageSize := getPagination(r)
	if viewID < 1 {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("globals.messages.somethingWentWrong"), nil, envelope.InputError)
	}

	// Check if user has access to the view.
	view, err := app.view.Get(viewID)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	user, err := app.user.GetAgent(auser.ID, "")
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	hasAccess := false
	switch view.Visibility {
	case vmodels.VisibilityUser:
		hasAccess = view.UserID != nil && *view.UserID == auser.ID
	case vmodels.VisibilityAll:
		hasAccess = true
	case vmodels.VisibilityTeam:
		if view.TeamID != nil {
			hasAccess = slices.Contains(user.Teams.IDs(), *view.TeamID)
		}
	}

	if !hasAccess {
		return r.SendErrorEnvelope(fasthttp.StatusForbidden, app.i18n.T("conversation.viewPermissionDenied"), nil, envelope.PermissionError)
	}

	// Prepare lists user has access to based on user permissions, internally this prepares the SQL query.
	lists := []string{}
	hasTeamAll := slices.Contains(user.Permissions, authzModels.PermConversationsReadTeamAll)
	for _, perm := range user.Permissions {
		if perm == authzModels.PermConversationsReadAll {
			// No further lists required as user has access to all conversations.
			lists = []string{cmodels.AllConversations}
			break
		}
		if perm == authzModels.PermConversationsReadUnassigned {
			lists = append(lists, cmodels.UnassignedConversations)
		}
		if perm == authzModels.PermConversationsReadAssigned {
			lists = append(lists, cmodels.AssignedConversations)
		}
		// Skip TeamUnassignedConversations if user has TeamAllConversations (superset).
		if perm == authzModels.PermConversationsReadTeamInbox && !hasTeamAll {
			lists = append(lists, cmodels.TeamUnassignedConversations)
		}
		if perm == authzModels.PermConversationsReadTeamAll {
			lists = append(lists, cmodels.TeamAllConversations)
		}
	}

	// No lists found, user doesn't have access to any conversations.
	if len(lists) == 0 {
		return r.SendErrorEnvelope(fasthttp.StatusForbidden, app.i18n.T("status.deniedPermission"), nil, envelope.PermissionError)
	}

	// Substitute the $current_user placeholder so shared views can filter by the logged-in agent.
	viewFilters := strings.ReplaceAll(string(view.Filters), "$current_user", strconv.Itoa(user.ID))
	conversations, err := app.conversation.GetViewConversationsList(user.ID, user.ID, user.Teams.IDs(), lists, order, orderBy, viewFilters, page, pageSize)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	if len(conversations) > 0 {
		total = conversations[0].Total
	}

	return r.SendEnvelope(envelope.PageResults{
		Results:    conversations,
		Total:      total,
		PerPage:    pageSize,
		TotalPages: (total + pageSize - 1) / pageSize,
		Page:       page,
	})
}

// handleGetTeamUnassignedConversations returns conversations assigned to a team but not to any user.
func handleGetTeamUnassignedConversations(r *fastglue.Request) error {
	var (
		app       = r.Context.(*App)
		auser     = r.RequestCtx.UserValue("user").(amodels.User)
		teamIDStr = r.RequestCtx.UserValue("id").(string)
		order     = string(r.RequestCtx.QueryArgs().Peek("order"))
		orderBy   = string(r.RequestCtx.QueryArgs().Peek("order_by"))
		filters   = string(r.RequestCtx.QueryArgs().Peek("filters"))
		total     = 0
	)
	page, pageSize := getPagination(r)
	teamID, _ := strconv.Atoi(teamIDStr)
	if teamID < 1 {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("globals.messages.somethingWentWrong"), nil, envelope.InputError)
	}

	// Check if user belongs to the team.
	exists, err := app.team.UserBelongsToTeam(teamID, auser.ID)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	if !exists {
		return sendErrorEnvelope(r, envelope.NewError(envelope.PermissionError, app.i18n.T("conversation.notMemberOfTeam"), nil))
	}

	conversations, err := app.conversation.GetTeamUnassignedConversationsList(auser.ID, teamID, order, orderBy, filters, page, pageSize)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	if len(conversations) > 0 {
		total = conversations[0].Total
	}

	return r.SendEnvelope(envelope.PageResults{
		Results:    conversations,
		Total:      total,
		PerPage:    pageSize,
		TotalPages: (total + pageSize - 1) / pageSize,
		Page:       page,
	})
}

// handleGetConversation retrieves a single conversation by it's UUID.
func handleGetConversation(r *fastglue.Request) error {
	var (
		app   = r.Context.(*App)
		uuid  = r.RequestCtx.UserValue("uuid").(string)
		auser = r.RequestCtx.UserValue("user").(amodels.User)
	)

	user, err := app.user.GetAgent(auser.ID, "")
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	conv, err := enforceConversationAccess(app, uuid, user)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	prev, _ := app.conversation.GetContactPreviousConversations(conv.ContactID, 10)
	conv.PreviousConversations = filterCurrentPreviousConv(prev, conv.UUID)
	return r.SendEnvelope(conv)
}

// handleGetContactPageVisits returns the recent page visits for the contact of a conversation.
func handleGetContactPageVisits(r *fastglue.Request) error {
	var (
		app   = r.Context.(*App)
		uuid  = r.RequestCtx.UserValue("uuid").(string)
		auser = r.RequestCtx.UserValue("user").(amodels.User)
	)

	user, err := app.user.GetAgent(auser.ID, "")
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	conv, err := enforceConversationAccess(app, uuid, user)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	pages := getPageVisitsFromRedis(app, conv.ContactID)
	return r.SendEnvelope(pages)
}

// handleUpdateConversationAssigneeLastSeen updates the current user's last seen timestamp for a conversation.
func handleUpdateConversationAssigneeLastSeen(r *fastglue.Request) error {
	var (
		app   = r.Context.(*App)
		uuid  = r.RequestCtx.UserValue("uuid").(string)
		auser = r.RequestCtx.UserValue("user").(amodels.User)
	)
	user, err := app.user.GetAgent(auser.ID, "")
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	_, err = enforceConversationAccess(app, uuid, user)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	if err = app.conversation.UpdateUserLastSeen(uuid, auser.ID); err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(true)
}

// handleMarkConversationAsUnread marks a conversation as unread for the current user.
func handleMarkConversationAsUnread(r *fastglue.Request) error {
	var (
		app   = r.Context.(*App)
		uuid  = r.RequestCtx.UserValue("uuid").(string)
		auser = r.RequestCtx.UserValue("user").(amodels.User)
	)
	user, err := app.user.GetAgent(auser.ID, "")
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	_, err = enforceConversationAccess(app, uuid, user)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	if err = app.conversation.MarkAsUnread(uuid, auser.ID); err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(true)
}

// handleGetConversationParticipants retrieves participants of a conversation.
func handleGetConversationParticipants(r *fastglue.Request) error {
	var (
		app   = r.Context.(*App)
		uuid  = r.RequestCtx.UserValue("uuid").(string)
		auser = r.RequestCtx.UserValue("user").(amodels.User)
	)
	user, err := app.user.GetAgent(auser.ID, "")
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	_, err = enforceConversationAccess(app, uuid, user)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	p, err := app.conversation.GetConversationParticipants(uuid)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(p)
}

// handleUpdateUserAssignee updates the user assigned to a conversation.
func handleUpdateUserAssignee(r *fastglue.Request) error {
	var (
		app   = r.Context.(*App)
		uuid  = r.RequestCtx.UserValue("uuid").(string)
		auser = r.RequestCtx.UserValue("user").(amodels.User)
		req   = assigneeChangeReq{}
	)

	if err := r.Decode(&req, "json"); err != nil {
		app.lo.Error("error decoding assignee change request", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("errors.parsingRequest"), nil, envelope.InputError)
	}

	user, err := app.user.GetAgent(auser.ID, "")
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	conversation, err := enforceConversationAccess(app, uuid, user)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	// Already assigned?
	if conversation.AssignedUserID.Int == req.AssigneeID {
		return r.SendEnvelope(true)
	}

	if err := app.conversation.UpdateConversationUserAssignee(uuid, req.AssigneeID, user); err != nil {
		return sendErrorEnvelope(r, err)
	}

	return r.SendEnvelope(true)
}

// handleUpdateTeamAssignee updates the team assigned to a conversation.
func handleUpdateTeamAssignee(r *fastglue.Request) error {
	var (
		app   = r.Context.(*App)
		uuid  = r.RequestCtx.UserValue("uuid").(string)
		auser = r.RequestCtx.UserValue("user").(amodels.User)
		req   = teamAssigneeChangeReq{}
	)

	if err := r.Decode(&req, "json"); err != nil {
		app.lo.Error("error decoding team assignee change request", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("errors.parsingRequest"), nil, envelope.InputError)
	}

	assigneeID := req.AssigneeID

	user, err := app.user.GetAgent(auser.ID, "")
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	_, err = app.team.Get(assigneeID)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	conversation, err := enforceConversationAccess(app, uuid, user)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	// Already assigned?
	if conversation.AssignedTeamID.Int == assigneeID {
		return r.SendEnvelope(true)
	}
	if err := app.conversation.UpdateConversationTeamAssignee(uuid, assigneeID, user); err != nil {
		return sendErrorEnvelope(r, err)
	}

	return r.SendEnvelope(true)
}

// handleUpdateConversationPriority updates the priority of a conversation.
func handleUpdateConversationPriority(r *fastglue.Request) error {
	var (
		app   = r.Context.(*App)
		uuid  = r.RequestCtx.UserValue("uuid").(string)
		auser = r.RequestCtx.UserValue("user").(amodels.User)
		req   = priorityUpdateReq{}
	)

	if err := r.Decode(&req, "json"); err != nil {
		app.lo.Error("error decoding priority update request", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("errors.parsingRequest"), nil, envelope.InputError)
	}

	priority := req.Priority
	if priority == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.Ts("globals.messages.empty", "name", "`priority`"), nil, envelope.InputError)
	}

	user, err := app.user.GetAgent(auser.ID, "")
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	_, err = enforceConversationAccess(app, uuid, user)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	if err := app.conversation.UpdateConversationPriority(uuid, 0 /**priority_id**/, priority, user); err != nil {
		return sendErrorEnvelope(r, err)
	}

	return r.SendEnvelope(true)
}

// handleUpdateConversationSubject updates the subject of a conversation. The
// sticky-header inline-edit affordance posts here. Wrapped in
// enforceConversationAccess so an agent without access to a conversation
// can't rewrite its subject by guessing the UUID. Length cap and trim happen
// in UpdateConversationSubject so future callers (CLI, automation) inherit
// the same guards.
func handleUpdateConversationSubject(r *fastglue.Request) error {
	var (
		app   = r.Context.(*App)
		uuid  = r.RequestCtx.UserValue("uuid").(string)
		auser = r.RequestCtx.UserValue("user").(amodels.User)
		req   = subjectUpdateReq{}
	)

	if err := r.Decode(&req, "json"); err != nil {
		app.lo.Error("error decoding subject update request", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("errors.parsingRequest"), nil, envelope.InputError)
	}

	user, err := app.user.GetAgent(auser.ID, "")
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	if _, err := enforceConversationAccess(app, uuid, user); err != nil {
		return sendErrorEnvelope(r, err)
	}

	if err := app.conversation.UpdateConversationSubject(uuid, req.Subject, user); err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(true)
}

// handleUpdateConversationStatus updates the status of a conversation.
func handleUpdateConversationStatus(r *fastglue.Request) error {
	var (
		app   = r.Context.(*App)
		uuid  = r.RequestCtx.UserValue("uuid").(string)
		auser = r.RequestCtx.UserValue("user").(amodels.User)
		req   = statusUpdateReq{}
	)

	if err := r.Decode(&req, "json"); err != nil {
		app.lo.Error("error decoding status update request", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("errors.parsingRequest"), nil, envelope.InputError)
	}

	status := req.Status
	snoozedUntil := req.SnoozedUntil

	// Validate inputs
	if status == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.Ts("globals.messages.empty", "name", "`status`"), nil, envelope.InputError)
	}
	if snoozedUntil == "" && status == cmodels.StatusSnoozed {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.Ts("globals.messages.empty", "name", "`snoozed_until`"), nil, envelope.InputError)
	}
	if status == cmodels.StatusSnoozed {
		_, err := time.ParseDuration(snoozedUntil)
		if err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("globals.messages.badRequest"), nil, envelope.InputError)
		}
	}

	// Enforce conversation access.
	user, err := app.user.GetAgent(auser.ID, "")
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	conversation, err := enforceConversationAccess(app, uuid, user)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	// Update conversation status.
	if err := app.conversation.UpdateConversationStatus(uuid, 0 /**status_id**/, status, snoozedUntil, user); err != nil {
		return sendErrorEnvelope(r, err)
	}

	// If status is `Resolved`, send CSAT survey if enabled on inbox.
	if status == cmodels.StatusResolved {
		// Check if CSAT is enabled on the inbox and send CSAT survey message.
		inbox, err := app.inbox.GetDBRecord(conversation.InboxID)
		if err != nil {
			return sendErrorEnvelope(r, err)
		}
		if inbox.CSATEnabled {
			if err := app.conversation.SendCSATReply(user.ID, *conversation); err != nil {
				return sendErrorEnvelope(r, err)
			}
		}
	}
	return r.SendEnvelope(true)
}

// handleUpdateConversationtags updates conversation tags.
func handleUpdateConversationtags(r *fastglue.Request) error {
	var (
		app   = r.Context.(*App)
		auser = r.RequestCtx.UserValue("user").(amodels.User)
		uuid  = r.RequestCtx.UserValue("uuid").(string)
		req   = tagsUpdateReq{}
	)

	if err := r.Decode(&req, "json"); err != nil {
		app.lo.Error("error decoding tags update request", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("errors.parsingRequest"), nil, envelope.InputError)
	}

	tagNames := req.Tags

	user, err := app.user.GetAgent(auser.ID, "")
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	_, err = enforceConversationAccess(app, uuid, user)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	if err := app.conversation.SetConversationTags(uuid, models.ActionSetTags, tagNames, user); err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(true)
}

// handleUpdateConversationCustomAttributes updates custom attributes of a conversation.
func handleUpdateConversationCustomAttributes(r *fastglue.Request) error {
	var (
		app        = r.Context.(*App)
		attributes = map[string]any{}
		auser      = r.RequestCtx.UserValue("user").(amodels.User)
		uuid       = r.RequestCtx.UserValue("uuid").(string)
	)
	if err := r.Decode(&attributes, ""); err != nil {
		app.lo.Error("error unmarshalling custom attributes JSON", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, app.i18n.T("errors.parsingRequest"), nil, envelope.InputError)
	}

	// Enforce conversation access.
	user, err := app.user.GetAgent(auser.ID, "")
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	_, err = enforceConversationAccess(app, uuid, user)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	// Update custom attributes.
	if err := app.conversation.UpdateConversationCustomAttributes(uuid, attributes); err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(true)
}

// handleUpdateContactCustomAttributes updates custom attributes of a contact.
func handleUpdateContactCustomAttributes(r *fastglue.Request) error {
	var (
		app        = r.Context.(*App)
		attributes = map[string]any{}
		auser      = r.RequestCtx.UserValue("user").(amodels.User)
		uuid       = r.RequestCtx.UserValue("uuid").(string)
	)
	if err := r.Decode(&attributes, ""); err != nil {
		app.lo.Error("error unmarshalling custom attributes JSON", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, app.i18n.T("errors.parsingRequest"), nil, envelope.InputError)
	}

	// Enforce conversation access.
	user, err := app.user.GetAgent(auser.ID, "")
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	conversation, err := enforceConversationAccess(app, uuid, user)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	if err := app.user.SaveCustomAttributes(conversation.ContactID, attributes, false); err != nil {
		return sendErrorEnvelope(r, err)
	}
	// Broadcast update.
	app.conversation.BroadcastContactUpdate(conversation.ContactID, map[string]any{"custom_attributes": attributes})
	return r.SendEnvelope(true)
}

// enforceConversationAccess fetches the conversation and checks if the user has access to it.
func enforceConversationAccess(app *App, uuid string, user umodels.User) (*cmodels.Conversation, error) {
	conversation, err := app.conversation.GetConversation(0, uuid, "")
	if err != nil {
		return nil, err
	}
	allowed, err := app.authz.EnforceConversationAccess(user, conversation)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, envelope.NewError(envelope.PermissionError, "Permission denied", nil)
	}
	return &conversation, nil
}

// handleRemoveUserAssignee removes the user assigned to a conversation.
func handleRemoveUserAssignee(r *fastglue.Request) error {
	var (
		app   = r.Context.(*App)
		uuid  = r.RequestCtx.UserValue("uuid").(string)
		auser = r.RequestCtx.UserValue("user").(amodels.User)
	)
	user, err := app.user.GetAgent(auser.ID, "")
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	_, err = enforceConversationAccess(app, uuid, user)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	if err = app.conversation.RemoveConversationAssignee(uuid, "user", user); err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(true)
}

// handleRemoveTeamAssignee removes the team assigned to a conversation.
func handleRemoveTeamAssignee(r *fastglue.Request) error {
	var (
		app   = r.Context.(*App)
		uuid  = r.RequestCtx.UserValue("uuid").(string)
		auser = r.RequestCtx.UserValue("user").(amodels.User)
	)
	user, err := app.user.GetAgent(auser.ID, "")
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	_, err = enforceConversationAccess(app, uuid, user)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	if err = app.conversation.RemoveConversationAssignee(uuid, "team", user); err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(true)
}

// filterCurrentPreviousConv removes the current conversation from the list of previous conversations.
func filterCurrentPreviousConv(convs []cmodels.PreviousConversation, uuid string) []cmodels.PreviousConversation {
	for i, c := range convs {
		if c.UUID == uuid {
			return append(convs[:i], convs[i+1:]...)
		}
	}
	return []cmodels.PreviousConversation{}
}

// handleCreateConversation creates a new conversation and sends a message to it.
func handleCreateConversation(r *fastglue.Request) error {
	var (
		app   = r.Context.(*App)
		auser = r.RequestCtx.UserValue("user").(amodels.User)
		req   = createConversationRequest{}
	)

	if err := r.Decode(&req, "json"); err != nil {
		app.lo.Error("error decoding create conversation request", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("errors.parsingRequest"), nil, envelope.InputError)
	}

	// Validate the request
	if err := validateCreateConversationRequest(req, app); err != nil {
		return sendErrorEnvelope(r, err)
	}

	// EC7: Build TO list. Primary contact's email is always TO[0]; any additional
	// recipients the agent typed in the chip-input (CreateConversation forwards
	// them concatenated to req.Email post-EC7 frontend changes — but for safety
	// we re-derive here, since the backend is the source of truth) are appended,
	// de-duplicated. SplitEmailList mirrors the frontend chip parser so `,`,
	// `;`, and whitespace all work as delimiters.
	to := stringutil.SplitEmailList(req.Email)
	if len(to) == 0 {
		to = []string{req.Email}
	}
	user, err := app.user.GetAgent(auser.ID, "")
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	// Find or create contact.
	contact := umodels.User{
		Email:            null.StringFrom(req.Email),
		FirstName:        req.FirstName,
		LastName:         req.LastName,
		ExternalUserID:   null.NewString(req.ExternalUserID, req.ExternalUserID != ""),
		CustomAttributes: json.RawMessage(`{}`),
	}
	if err := app.user.CreateContact(&contact); err != nil {
		return sendErrorEnvelope(r, envelope.NewError(envelope.GeneralError, app.i18n.T("globals.messages.somethingWentWrong"), nil))
	}

	// Create conversation first.
	conversationID, conversationUUID, err := app.conversation.CreateConversation(
		contact.ID,
		req.InboxID,
		"",         /** last_message **/
		time.Now(), /** last_message_at **/
		req.Subject,
		true, /** append reference number to subject? **/
		nil,
		req.CustomAttributes,
		0, 0,
	)
	if err != nil {
		app.lo.Error("error creating conversation", "error", err)
		return sendErrorEnvelope(r, envelope.NewError(envelope.GeneralError, app.i18n.T("globals.messages.somethingWentWrong"), nil))
	}

	// Get media for the attachment ids, skip any already associated with a model.
	media, err := getUnassociatedMedia(app, req.Attachments)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, app.i18n.T("globals.messages.somethingWentWrong"), nil, envelope.GeneralError)
	}

	// EC7: Parse CC/BCC strings into slices. Both are optional, accept `,`,
	// `;`, or whitespace as delimiters (mirrors the chip-input parser) and
	// drop empties so "a@b.com,  ,c@d.com" doesn't surface a blank recipient.
	ccList := stringutil.SplitEmailList(req.CC)
	bccList := stringutil.SplitEmailList(req.BCC)

	// Send initial message based on the initiator of conversation.
	switch req.Initiator {
	case umodels.UserTypeAgent:
		// Queue reply.
		if _, err := app.conversation.QueueReply(media, req.InboxID, auser.ID /**sender_id**/, contact.ID, conversationUUID, req.Content, to, ccList, bccList, map[string]any{} /**meta**/); err != nil {
			// Delete the conversation if msg queue fails.
			if err := app.conversation.DeleteConversation(conversationUUID); err != nil {
				app.lo.Error("error deleting conversation", "error", err)
			}
			return sendErrorEnvelope(r, envelope.NewError(envelope.GeneralError, app.i18n.T("globals.messages.errorSendingMessage"), nil))
		}
		// Trigger webhook for agent-initiated conversation, for contact intitiated the incoming message hooks handle it.
		if c, err := app.conversation.GetConversation(0, conversationUUID, ""); err == nil {
			app.webhook.TriggerEvent(wmodels.EventConversationCreated, c)
		}
	case umodels.UserTypeContact:
		// Create contact message.
		if _, err := app.conversation.CreateContactMessage(media, contact.ID, conversationUUID, req.Content, cmodels.ContentTypeHTML, true); err != nil {
			// Delete the conversation if message creation fails.
			if err := app.conversation.DeleteConversation(conversationUUID); err != nil {
				app.lo.Error("error deleting conversation", "error", err)
			}
			return sendErrorEnvelope(r, envelope.NewError(envelope.GeneralError, app.i18n.T("globals.messages.errorSendingMessage"), nil))
		}
	default:
		// Guard anyway.
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("globals.messages.somethingWentWrong"), nil, envelope.InputError)
	}

	// Assign the conversation to team/agent if provided, always assign team first as it clears assigned agent.
	if req.AssignedTeamID > 0 {
		app.conversation.UpdateConversationTeamAssignee(conversationUUID, req.AssignedTeamID, user)
	}
	if req.AssignedAgentID > 0 {
		app.conversation.UpdateConversationUserAssignee(conversationUUID, req.AssignedAgentID, user)
	}

	// EC20: Apply requested follow-up status. UpdateConversationStatus
	// internally validates the status name via the status store, so any
	// invalid name (or one the agent isn't allowed to set) is rejected
	// there. We reject Open (it's the default and a no-op) and Snoozed
	// (it requires a duration we don't collect on the create dialog) up
	// front. Failure to apply the status is logged but doesn't fail the
	// create — the conversation already exists and the agent can adjust
	// the status from the header afterward.
	if req.SetStatus != "" && req.SetStatus != cmodels.StatusOpen && req.SetStatus != cmodels.StatusSnoozed {
		if err := app.conversation.UpdateConversationStatus(conversationUUID, 0 /**status_id**/, req.SetStatus, "" /**snoozed_until**/, user); err != nil {
			app.lo.Warn("could not apply set_status on new conversation", "uuid", conversationUUID, "status", req.SetStatus, "error", err)
		}
	}

	conversation, _ := app.conversation.GetConversation(conversationID, "", "")
	return r.SendEnvelope(conversation)
}

// validateCreateConversationRequest validates the create conversation request fields.
func validateCreateConversationRequest(req createConversationRequest, app *App) error {
	if req.InboxID <= 0 {
		return envelope.NewError(envelope.InputError, app.i18n.Ts("globals.messages.required", "name", "`inbox_id`"), nil)
	}
	if req.Content == "" {
		return envelope.NewError(envelope.InputError, app.i18n.Ts("globals.messages.required", "name", "`content`"), nil)
	}
	if req.Email == "" {
		return envelope.NewError(envelope.InputError, app.i18n.Ts("globals.messages.required", "name", "`contact_email`"), nil)
	}
	if req.FirstName == "" {
		return envelope.NewError(envelope.InputError, app.i18n.Ts("globals.messages.required", "name", "`first_name`"), nil)
	}
	if !stringutil.ValidEmail(req.Email) {
		return envelope.NewError(envelope.InputError, app.i18n.T("validation.invalidEmail"), nil)
	}
	if req.Initiator != umodels.UserTypeContact && req.Initiator != umodels.UserTypeAgent {
		return envelope.NewError(envelope.InputError, app.i18n.T("globals.messages.somethingWentWrong"), nil)
	}

	// Check if inbox exists and is enabled.
	inbox, err := app.inbox.GetDBRecord(req.InboxID)
	if err != nil {
		return err
	}
	if !inbox.Enabled {
		return envelope.NewError(envelope.InputError, app.i18n.T("globals.messages.disabled"), nil)
	}
	if inbox.Channel != "email" {
		return envelope.NewError(envelope.InputError, app.i18n.T("globals.messages.somethingWentWrong"), nil)
	}

	// Validate custom attribute keys. Skip unknown keys.
	if len(req.CustomAttributes) > 0 {
		attrs, err := app.customAttribute.GetAll("conversation")
		if err != nil {
			return err
		}
		validKeys := make(map[string]struct{}, len(attrs))
		for _, a := range attrs {
			validKeys[a.Key] = struct{}{}
		}
		for key := range req.CustomAttributes {
			if _, ok := validKeys[key]; !ok {
				delete(req.CustomAttributes, key)
			}
		}
	}

	return nil
}

// handleMoveToTrash moves a conversation to trash.
func handleMoveToTrash(r *fastglue.Request) error {
	var (
		app   = r.Context.(*App)
		uuid  = r.RequestCtx.UserValue("uuid").(string)
		auser = r.RequestCtx.UserValue("user").(amodels.User)
	)
	user, err := app.user.GetAgent(auser.ID, "")
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	if _, err := enforceConversationAccess(app, uuid, user); err != nil {
		return sendErrorEnvelope(r, err)
	}
	if err := app.conversation.MoveToTrash(uuid, user); err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(true)
}

// handleRestoreFromTrash restores a conversation from trash to open.
func handleRestoreFromTrash(r *fastglue.Request) error {
	var (
		app   = r.Context.(*App)
		uuid  = r.RequestCtx.UserValue("uuid").(string)
		auser = r.RequestCtx.UserValue("user").(amodels.User)
	)
	user, err := app.user.GetAgent(auser.ID, "")
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	if _, err := enforceConversationAccess(app, uuid, user); err != nil {
		return sendErrorEnvelope(r, err)
	}
	if err := app.conversation.RestoreFromTrash(uuid, user); err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(true)
}

// handlePermanentDeleteConversation permanently deletes a trashed conversation
// (FS13). Gated to Trashed conversations to ensure the only legitimate caller
// is the bulk-delete affordance on the trash list. The frontend already hides
// the button outside the trash view, but a defensive status check here prevents
// an Open conversation from being erased by a hand-rolled DELETE request.
func handlePermanentDeleteConversation(r *fastglue.Request) error {
	var (
		app   = r.Context.(*App)
		uuid  = r.RequestCtx.UserValue("uuid").(string)
		auser = r.RequestCtx.UserValue("user").(amodels.User)
	)
	user, err := app.user.GetAgent(auser.ID, "")
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	conversation, err := enforceConversationAccess(app, uuid, user)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	if conversation.Status.String != cmodels.StatusTrashed {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest,
			app.i18n.T("conversation.permanentDelete.requiresTrash"), nil, envelope.InputError)
	}
	if err := app.conversation.DeleteConversation(uuid); err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(true)
}

// handleMarkAsSpam marks a conversation as spam.
func handleMarkAsSpam(r *fastglue.Request) error {
	var (
		app   = r.Context.(*App)
		uuid  = r.RequestCtx.UserValue("uuid").(string)
		auser = r.RequestCtx.UserValue("user").(amodels.User)
	)
	user, err := app.user.GetAgent(auser.ID, "")
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	if _, err := enforceConversationAccess(app, uuid, user); err != nil {
		return sendErrorEnvelope(r, err)
	}
	if err := app.conversation.MarkAsSpam(uuid, user); err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(true)
}

// handleMarkAsNotSpam moves a spam conversation back to open.
func handleMarkAsNotSpam(r *fastglue.Request) error {
	var (
		app   = r.Context.(*App)
		uuid  = r.RequestCtx.UserValue("uuid").(string)
		auser = r.RequestCtx.UserValue("user").(amodels.User)
	)
	user, err := app.user.GetAgent(auser.ID, "")
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	if _, err := enforceConversationAccess(app, uuid, user); err != nil {
		return sendErrorEnvelope(r, err)
	}
	if err := app.conversation.MarkAsNotSpam(uuid, user); err != nil {
		return sendErrorEnvelope(r, err)
	}

	// Best-effort: tell the IMAP server (e.g. Gmail) "this isn't spam" by
	// moving the most recent incoming message of this conversation back to
	// INBOX. Async so a slow/dead IMAP server doesn't block the response,
	// and a failure here doesn't undo the not-spam mark — the agent's UI
	// is already updated regardless.
	sourceID, inboxID, err := app.conversation.GetLatestIncomingMessageRef(uuid)
	if err != nil {
		app.lo.Warn("could not look up source message for IMAP unspam", "uuid", uuid, "error", err)
	} else if sourceID != "" {
		go func() {
			if err := app.inbox.UnspamIMAPMessage(inboxID, sourceID); err != nil {
				app.lo.Error("failed to move message out of IMAP spam folder", "error", err, "message_id", sourceID, "inbox_id", inboxID)
			}
		}()
	}

	return r.SendEnvelope(true)
}

// handleGetSpamConversations retrieves spam conversations.
func handleGetSpamConversations(r *fastglue.Request) error {
	var (
		app     = r.Context.(*App)
		user    = r.RequestCtx.UserValue("user").(amodels.User)
		order   = string(r.RequestCtx.QueryArgs().Peek("order"))
		orderBy = string(r.RequestCtx.QueryArgs().Peek("order_by"))
		filters = string(r.RequestCtx.QueryArgs().Peek("filters"))
		total   = 0
	)
	page, pageSize := getPagination(r)
	conversations, err := app.conversation.GetSpamConversationsList(user.ID, order, orderBy, filters, page, pageSize)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	if len(conversations) > 0 {
		total = conversations[0].Total
	}
	return r.SendEnvelope(envelope.PageResults{
		Results:    conversations,
		Total:      total,
		PerPage:    pageSize,
		TotalPages: (total + pageSize - 1) / pageSize,
		Page:       page,
	})
}

// handleGetTrashedConversations retrieves trashed conversations.
func handleGetTrashedConversations(r *fastglue.Request) error {
	var (
		app     = r.Context.(*App)
		user    = r.RequestCtx.UserValue("user").(amodels.User)
		order   = string(r.RequestCtx.QueryArgs().Peek("order"))
		orderBy = string(r.RequestCtx.QueryArgs().Peek("order_by"))
		filters = string(r.RequestCtx.QueryArgs().Peek("filters"))
		total   = 0
	)
	page, pageSize := getPagination(r)
	conversations, err := app.conversation.GetTrashedConversationsList(user.ID, order, orderBy, filters, page, pageSize)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	if len(conversations) > 0 {
		total = conversations[0].Total
	}
	return r.SendEnvelope(envelope.PageResults{
		Results:    conversations,
		Total:      total,
		PerPage:    pageSize,
		TotalPages: (total + pageSize - 1) / pageSize,
		Page:       page,
	})
}

// mergeConversationsReq is the JSON body shape for handleMergeConversations.
type mergeConversationsReq struct {
	PrimaryUUID    string   `json:"primary_uuid"`
	SecondaryUUIDs []string `json:"secondary_uuids"`
}

// handleMergeConversations merges one or more secondary conversations into a primary.
// Permission: conversations:update_status (matches the trash/spam status mutators).
func handleMergeConversations(r *fastglue.Request) error {
	var (
		app   = r.Context.(*App)
		auser = r.RequestCtx.UserValue("user").(amodels.User)
		req   mergeConversationsReq
	)

	if err := r.Decode(&req, "json"); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("globals.messages.badRequest"), nil, envelope.InputError)
	}

	if req.PrimaryUUID == "" || len(req.SecondaryUUIDs) == 0 {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "primary_uuid and at least one secondary_uuid are required", nil, envelope.InputError)
	}

	if len(req.SecondaryUUIDs) > 50 {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.Ts("conversation.merge.tooManyTickets", "max", "50"), nil, envelope.InputError)
	}

	user, err := app.user.GetAgent(auser.ID, "")
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	// Permission gate applies to the primary AND each secondary — agents
	// can only merge conversations they are allowed to access.
	if _, err := enforceConversationAccess(app, req.PrimaryUUID, user); err != nil {
		return sendErrorEnvelope(r, err)
	}
	for _, secUUID := range req.SecondaryUUIDs {
		if _, err := enforceConversationAccess(app, secUUID, user); err != nil {
			return sendErrorEnvelope(r, err)
		}
	}

	result, err := app.conversation.MergeConversations(req.PrimaryUUID, req.SecondaryUUIDs, user)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	if len(result.Warnings) > 0 {
		return r.SendEnvelope(map[string]any{
			"success":  true,
			"warnings": result.Warnings,
		})
	}

	return r.SendEnvelope(true)
}

// handleGetConversationByRef retrieves a single conversation by its reference number.
// This dedicated endpoint avoids the 3-character minimum enforced by the search endpoint,
// allowing lookup of tickets with reference numbers #1-#99.
func handleGetConversationByRef(r *fastglue.Request) error {
	var (
		app   = r.Context.(*App)
		ref   = r.RequestCtx.UserValue("ref").(string)
		auser = r.RequestCtx.UserValue("user").(amodels.User)
	)

	user, err := app.user.GetAgent(auser.ID, "")
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	conv, err := app.conversation.GetConversation(0, "", ref)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	allowed, err := app.authz.EnforceConversationAccess(user, conv)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	if !allowed {
		return r.SendErrorEnvelope(fasthttp.StatusForbidden, app.i18n.T("status.deniedPermission"), nil, envelope.PermissionError)
	}

	return r.SendEnvelope(conv)
}

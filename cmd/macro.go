package main

import (
	"encoding/json"
	"slices"
	"strconv"

	amodels "github.com/abhinavxd/libredesk/internal/auth/models"
	autoModels "github.com/abhinavxd/libredesk/internal/automation/models"
	"github.com/abhinavxd/libredesk/internal/envelope"
	"github.com/abhinavxd/libredesk/internal/macro/models"
	mmodels "github.com/abhinavxd/libredesk/internal/media/models"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

// macroRequest wraps a macro with an optional attachments list (media IDs).
// Attachments are uploaded separately via the /media endpoint with linked_model
// set to 'macros'; the resulting media IDs are then submitted here so the
// macro handler can Attach() them after the macro itself is created/updated.
type macroRequest struct {
	models.Macro
	AttachmentIDs []int `json:"attachment_ids"`
}

// handleGetMacros returns all macros, sorted by the calling agent's
// most-recently-used (MP6). Each agent gets their own picker order; the
// global usage_count column is unaffected.
func handleGetMacros(r *fastglue.Request) error {
	var (
		app   = r.Context.(*App)
		auser = r.RequestCtx.UserValue("user").(amodels.User)
	)
	macros, err := app.macro.GetAllForUser(auser.ID)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	for i, m := range macros {
		var actions []autoModels.RuleAction
		if err := json.Unmarshal(m.Actions, &actions); err != nil {
			app.lo.Error("error unmarshalling macro actions", "macro_id", m.ID, "error", err)
			return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, app.i18n.T("globals.messages.somethingWentWrong"), nil, envelope.GeneralError)
		}
		// Set display values for actions as the value field can contain DB IDs
		if err := setDisplayValues(app, actions); err != nil {
			app.lo.Warn("error setting display values", "error", err)
		}
		if macros[i].Actions, err = json.Marshal(actions); err != nil {
			app.lo.Error("error marshalling macro actions", "macro_id", m.ID, "error", err)
			return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, app.i18n.T("globals.messages.somethingWentWrong"), nil, envelope.GeneralError)
		}
		// Attachments are stored polymorphically in the media table; populate
		// inline so the picker can show paperclip indicators without a second
		// round trip per macro.
		macros[i].Attachments = populateMacroAttachments(app, macros[i].ID)
	}
	return r.SendEnvelope(macros)
}

// handleGetMacro returns a macro.
func handleGetMacro(r *fastglue.Request) error {
	var (
		app     = r.Context.(*App)
		id, err = strconv.Atoi(r.RequestCtx.UserValue("id").(string))
	)
	if err != nil || id == 0 {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("globals.messages.somethingWentWrong"), nil, envelope.InputError)
	}

	macro, err := app.macro.Get(id)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	var actions []autoModels.RuleAction
	if err := json.Unmarshal(macro.Actions, &actions); err != nil {
		app.lo.Error("error unmarshalling macro actions", "macro_id", id, "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, app.i18n.T("globals.messages.somethingWentWrong"), nil, envelope.GeneralError)
	}
	// Set display values for actions as the value field can contain DB IDs
	if err := setDisplayValues(app, actions); err != nil {
		app.lo.Warn("error setting display values", "error", err)
	}
	if macro.Actions, err = json.Marshal(actions); err != nil {
		app.lo.Error("error marshalling macro actions", "macro_id", id, "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, app.i18n.T("globals.messages.somethingWentWrong"), nil, envelope.GeneralError)
	}

	macro.Attachments = populateMacroAttachments(app, macro.ID)

	return r.SendEnvelope(macro)
}

// handleCreateMacro creates new macro.
func handleCreateMacro(r *fastglue.Request) error {
	var (
		app = r.Context.(*App)
		req = macroRequest{}
	)

	if err := r.Decode(&req, "json"); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("errors.parsingRequest"), err.Error(), envelope.InputError)
	}

	if err := validateMacro(app, req.Macro); err != nil {
		return sendErrorEnvelope(r, err)
	}

	createdMacro, err := app.macro.Create(req.Name, req.MessageContent, req.UserID, req.TeamID, req.Visibility, req.VisibleWhen, req.Actions)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	// Link any media files the agent uploaded against this macro. Attach
	// failures are logged but non-fatal — the macro itself was created
	// successfully and the orphaned media will be reaped by the unlinked-
	// media sweeper.
	for _, mediaID := range req.AttachmentIDs {
		if err := app.media.Attach(mediaID, mmodels.ModelMacros, createdMacro.ID); err != nil {
			app.lo.Warn("error attaching media to macro", "media_id", mediaID, "macro_id", createdMacro.ID, "error", err)
		}
	}

	createdMacro.Attachments = populateMacroAttachments(app, createdMacro.ID)
	return r.SendEnvelope(createdMacro)
}

// handleUpdateMacro updates a macro.
func handleUpdateMacro(r *fastglue.Request) error {
	var (
		app = r.Context.(*App)
		req = macroRequest{}
	)

	id, err := strconv.Atoi(r.RequestCtx.UserValue("id").(string))
	if err != nil || id == 0 {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest,
			"Invalid macro `id`.", nil, envelope.InputError)
	}

	if err := r.Decode(&req, "json"); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "decode failed", err.Error(), envelope.InputError)
	}

	if err := validateMacro(app, req.Macro); err != nil {
		return sendErrorEnvelope(r, err)
	}

	updatedMacro, err := app.macro.Update(id, req.Name, req.MessageContent, req.UserID, req.TeamID, req.Visibility, req.VisibleWhen, req.Actions)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	// Re-sync attachments: detach the existing set, then attach whatever the
	// client sent. The frontend posts the full intended attachment list (kept
	// + newly-uploaded), so any media that was on the macro but is missing
	// from req.AttachmentIDs has been removed by the agent. We don't delete
	// the storage blobs here — orphaned media gets swept up by the unlinked-
	// message cleaner since it's now model_id=NULL with model_type also NULL.
	if err := app.media.DetachModelMedia(mmodels.ModelMacros, id); err != nil {
		app.lo.Warn("error detaching macro media", "macro_id", id, "error", err)
	}
	for _, mediaID := range req.AttachmentIDs {
		if err := app.media.Attach(mediaID, mmodels.ModelMacros, id); err != nil {
			app.lo.Warn("error attaching media to macro", "media_id", mediaID, "macro_id", id, "error", err)
		}
	}

	updatedMacro.Attachments = populateMacroAttachments(app, id)
	return r.SendEnvelope(updatedMacro)
}

// handleDeleteMacro deletes macro.
func handleDeleteMacro(r *fastglue.Request) error {
	var app = r.Context.(*App)
	id, err := strconv.Atoi(r.RequestCtx.UserValue("id").(string))
	if err != nil || id == 0 {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("globals.messages.somethingWentWrong"), nil, envelope.InputError)
	}

	// Delete the macro's attachments before the macro itself. Best-effort:
	// orphaned media here is non-fatal but worth logging.
	if err := app.media.DeleteModelMedia(mmodels.ModelMacros, id); err != nil {
		app.lo.Warn("error deleting macro media", "macro_id", id, "error", err)
	}

	if err := app.macro.Delete(id); err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(true)
}

// handleCloneMacroAttachments duplicates a macro's attachments so they can be
// included in a reply or new conversation. Each clone gets its own UUID and
// DB row (model_type='messages', model_id=NULL, ready to be attached when the
// message is sent), so deleting either the macro or the attachment afterwards
// won't invalidate the already-sent copy.
func handleCloneMacroAttachments(r *fastglue.Request) error {
	var app = r.Context.(*App)
	id, err := strconv.Atoi(r.RequestCtx.UserValue("id").(string))
	if err != nil || id == 0 {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("globals.messages.somethingWentWrong"), nil, envelope.InputError)
	}

	// Verify the macro exists and is readable. This also gives us the standard
	// not-found envelope rather than silently returning an empty list when the
	// id is bogus.
	if _, err := app.macro.Get(id); err != nil {
		return sendErrorEnvelope(r, err)
	}

	newMedia, err := app.media.DuplicateForModel(mmodels.ModelMacros, id)
	if err != nil {
		app.lo.Error("error duplicating macro attachments", "macro_id", id, "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, app.i18n.T("globals.messages.somethingWentWrong"), nil, envelope.GeneralError)
	}

	return r.SendEnvelope(newMedia)
}

// handleApplyMacro applies macro actions to a conversation.
func handleApplyMacro(r *fastglue.Request) error {
	var (
		app              = r.Context.(*App)
		auser            = r.RequestCtx.UserValue("user").(amodels.User)
		conversationUUID = r.RequestCtx.UserValue("uuid").(string)
		id, _            = strconv.Atoi(r.RequestCtx.UserValue("id").(string))
		incomingActions  = []autoModels.RuleAction{}
	)
	user, err := app.user.GetAgent(auser.ID, "")
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	// Enforce conversation access.
	conversation, err := app.conversation.GetConversation(0, conversationUUID, "")
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	if allowed, err := app.authz.EnforceConversationAccess(user, conversation); err != nil || !allowed {
		return sendErrorEnvelope(r, envelope.NewError(envelope.PermissionError, app.i18n.T("status.deniedPermission"), nil))
	}

	macro, err := app.macro.Get(id)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	// Decode incoming actions.
	if err := r.Decode(&incomingActions, "json"); err != nil {
		app.lo.Error("error unmashalling incoming actions", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("globals.messages.somethingWentWrong"), err.Error(), envelope.InputError)
	}

	// Make sure no duplicate action types are present.
	actionTypes := make(map[string]bool, len(incomingActions))
	for _, act := range incomingActions {
		if actionTypes[act.Type] {
			app.lo.Warn("duplicate action types found in macro apply apply request", "action", act.Type, "user_id", user.ID)
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("macro.duplicateActionsNotAllowed"), nil, envelope.InputError)
		}
		actionTypes[act.Type] = true
	}

	// Validate action permissions.
	for _, act := range incomingActions {
		if !isMacroActionAllowed(act.Type) {
			app.lo.Warn("action not allowed in macro", "action", act.Type, "user_id", user.ID)
			return r.SendErrorEnvelope(fasthttp.StatusForbidden, app.i18n.Ts("macro.actionNotAllowed", "name", act.Type), nil, envelope.PermissionError)
		}
		if !hasActionPermission(act.Type, user.Permissions) {
			app.lo.Warn("no permission to execute macro action", "action", act.Type, "user_id", user.ID)
			return r.SendErrorEnvelope(fasthttp.StatusForbidden, app.i18n.T("macro.permissionDenied"), nil, envelope.PermissionError)
		}
	}

	// Apply actions.
	successCount := 0
	for _, act := range incomingActions {
		if err := app.conversation.ApplyAction(act, conversation, user); err == nil {
			successCount++
		}
	}

	if successCount == 0 {
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, app.i18n.T("macro.couldNotApply"), nil, envelope.GeneralError)
	}

	// Increment global usage count and per-user MRU tracking (MP6).
	// MarkUsed failure is non-fatal: the macro applied successfully, the
	// per-user reorder will just be missed for this apply.
	app.macro.IncrementUsageCount(macro.ID)
	if err := app.macro.MarkUsed(user.ID, macro.ID); err != nil {
		app.lo.Warn("could not record per-user macro usage", "user_id", user.ID, "macro_id", macro.ID, "error", err)
	}

	if successCount < len(incomingActions) {
		return r.SendJSON(fasthttp.StatusMultiStatus, map[string]interface{}{
			"message": app.i18n.T("macro.partiallyApplied"),
		})
	}

	return r.SendJSON(fasthttp.StatusOK, map[string]interface{}{
		"message": app.i18n.T("macro.applied"),
	})
}

// populateMacroAttachments returns the media files attached to a macro with
// signed/served URLs filled in. GetByModel itself doesn't populate URL (only
// the single-row Get does), so we set it here. Errors are logged and swallowed
// — a missing attachment list shouldn't block the macro response.
func populateMacroAttachments(app *App, macroID int) []mmodels.Media {
	attachments, err := app.media.GetByModel(macroID, mmodels.ModelMacros)
	if err != nil {
		app.lo.Warn("error fetching macro attachments", "macro_id", macroID, "error", err)
		return []mmodels.Media{}
	}
	for i := range attachments {
		attachments[i].URL = app.media.GetURL(attachments[i].UUID, attachments[i].ContentType, attachments[i].Filename)
	}
	return attachments
}

// hasActionPermission checks user permission for given action
func hasActionPermission(action string, userPerms []string) bool {
	requiredPerm, exists := autoModels.ActionPermissions[action]
	if !exists {
		return false
	}
	return slices.Contains(userPerms, requiredPerm)
}

// setDisplayValues sets display values for actions.
func setDisplayValues(app *App, actions []autoModels.RuleAction) error {
	getters := map[string]func(int) (string, error){
		autoModels.ActionAssignTeam: func(id int) (string, error) {
			t, err := app.team.Get(id)
			if err != nil {
				app.lo.Warn("team not found for macro action", "team_id", id)
				return "", err
			}
			return t.Name, nil
		},
		autoModels.ActionAssignUser: func(id int) (string, error) {
			u, err := app.user.GetAgent(id, "")
			if err != nil {
				app.lo.Warn("user not found for macro action", "user_id", id)
				return "", err
			}
			return u.FullName(), nil
		},
		autoModels.ActionSetPriority: func(id int) (string, error) {
			p, err := app.priority.Get(id)
			if err != nil {
				app.lo.Warn("priority not found for macro action", "priority_id", id)
				return "", err
			}
			return p.Name, nil
		},
		autoModels.ActionSetStatus: func(id int) (string, error) {
			s, err := app.status.Get(id)
			if err != nil {
				app.lo.Warn("status not found for macro action", "status_id", id)
				return "", err
			}
			return s.Name, nil
		},
	}
	for i := range actions {
		actions[i].DisplayValue = []string{}
		if getter, ok := getters[actions[i].Type]; ok {
			id, _ := strconv.Atoi(actions[i].Value[0])
			if name, err := getter(id); err == nil {
				actions[i].DisplayValue = append(actions[i].DisplayValue, name)
			}
		}
	}
	return nil
}

// validateMacro validates an incoming macro.
func validateMacro(app *App, macro models.Macro) error {
	if macro.Name == "" {
		return envelope.NewError(envelope.InputError, app.i18n.Ts("globals.messages.empty", "name", "`name`"), nil)
	}

	if len(macro.VisibleWhen) == 0 {
		return envelope.NewError(envelope.InputError, app.i18n.Ts("globals.messages.empty", "name", "`visible_when`"), nil)
	}

	var act []autoModels.RuleAction
	if err := json.Unmarshal(macro.Actions, &act); err != nil {
		app.lo.Error("error unmarshalling macro actions", "error", err)
		return envelope.NewError(envelope.InputError, app.i18n.T("globals.messages.somethingWentWrong"), nil)
	}
	for _, a := range act {
		if len(a.Value) == 0 {
			return envelope.NewError(envelope.InputError, app.i18n.Ts("globals.messages.empty", "name", a.Type), nil)
		}
	}
	return nil
}

// isMacroActionAllowed returns true if the action is allowed in a macro.
func isMacroActionAllowed(action string) bool {
	switch action {
	case autoModels.ActionSendPrivateNote, autoModels.ActionReply:
		return false
	case autoModels.ActionAssignTeam, autoModels.ActionAssignUser, autoModels.ActionSetStatus, autoModels.ActionSetPriority, autoModels.ActionAddTags, autoModels.ActionSetTags, autoModels.ActionRemoveTags:
		return true
	default:
		return false
	}
}

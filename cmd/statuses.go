package main

import (
	"strconv"

	cmodels "github.com/abhinavxd/libredesk/internal/conversation/models"
	"github.com/abhinavxd/libredesk/internal/envelope"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

func handleGetStatuses(r *fastglue.Request) error {
	var (
		app = r.Context.(*App)
	)
	out, err := app.status.GetAll()
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(out)
}

func handleCreateStatus(r *fastglue.Request) error {
	var (
		app    = r.Context.(*App)
		status = cmodels.Status{}
	)
	if err := r.Decode(&status, "json"); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("errors.parsingRequest"), err.Error(), envelope.InputError)
	}

	if status.Name == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.Ts("globals.messages.empty", "name", "`name`"), nil, envelope.InputError)
	}

	createdStatus, err := app.status.Create(status.Name, status.Category, status.Color)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	return r.SendEnvelope(createdStatus)
}

func handleDeleteStatus(r *fastglue.Request) error {
	var (
		app = r.Context.(*App)
	)
	id, err := strconv.Atoi(r.RequestCtx.UserValue("id").(string))
	if err != nil || id <= 0 {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("globals.messages.somethingWentWrong"), nil, envelope.InputError)
	}
	err = app.status.Delete(id)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(true)
}

func handleUpdateStatus(r *fastglue.Request) error {
	var (
		app    = r.Context.(*App)
		status = cmodels.Status{}
	)
	id, err := strconv.Atoi(r.RequestCtx.UserValue("id").(string))
	if err != nil || id == 0 {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("globals.messages.somethingWentWrong"), nil, envelope.InputError)
	}

	if err := r.Decode(&status, "json"); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("errors.parsingRequest"), err.Error(), envelope.InputError)
	}

	if status.Name == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.Ts("globals.messages.empty", "name", "`name`"), nil, envelope.InputError)
	}

	updatedStatus, err := app.status.Update(id, status.Name, status.Category, status.Color)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	return r.SendEnvelope(updatedStatus)
}

// handleUpdateStatusColor updates only the colour of a status. Separate route
// from the full update so the inline colour picker in the admin status list
// can re-colour a default status (Open/Snoozed/Resolved/Closed) without hitting
// the "cannot update default status" guard on the full UpdateStatus path.
func handleUpdateStatusColor(r *fastglue.Request) error {
	var (
		app = r.Context.(*App)
		req struct {
			Color string `json:"color"`
		}
	)
	id, err := strconv.Atoi(r.RequestCtx.UserValue("id").(string))
	if err != nil || id <= 0 {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("globals.messages.somethingWentWrong"), nil, envelope.InputError)
	}
	if err := r.Decode(&req, "json"); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("errors.parsingRequest"), err.Error(), envelope.InputError)
	}
	if err := app.status.UpdateColor(id, req.Color); err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(true)
}

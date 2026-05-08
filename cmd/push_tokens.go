package main

import (
	amodels "github.com/abhinavxd/libredesk/internal/auth/models"
	"github.com/abhinavxd/libredesk/internal/envelope"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

// pushTokenReq is the body of POST /api/v1/agents/me/push-token.
// Platform is required + restricted to "android"/"ios" (the DB CHECK
// constraint enforces the same set).
type pushTokenReq struct {
	Token    string `json:"token"`
	Platform string `json:"platform"`
}

// handleRegisterPushToken stores an FCM device token for the current agent.
// The mobile app calls this after sign-in so the FCM dispatcher (T3ad) can
// later look up tokens to push to.
func handleRegisterPushToken(r *fastglue.Request) error {
	var (
		app   = r.Context.(*App)
		auser = r.RequestCtx.UserValue("user").(amodels.User)
	)

	var req pushTokenReq
	if err := r.Decode(&req, "json"); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("errors.parsingRequest"), nil, envelope.InputError)
	}
	if req.Token == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.Ts("globals.messages.empty", "name", "`token`"), nil, envelope.InputError)
	}
	if req.Platform != "android" && req.Platform != "ios" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("user.invalidPushPlatform"), nil, envelope.InputError)
	}

	if err := app.user.RegisterPushToken(auser.ID, req.Token, req.Platform); err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(true)
}

// handleUnregisterPushToken removes a push token on logout / app uninstall.
// Idempotent — a missing row returns success (the caller's intent is that
// the token is gone).
func handleUnregisterPushToken(r *fastglue.Request) error {
	var (
		app   = r.Context.(*App)
		auser = r.RequestCtx.UserValue("user").(amodels.User)
	)

	var req struct {
		Token string `json:"token"`
	}
	if err := r.Decode(&req, "json"); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("errors.parsingRequest"), nil, envelope.InputError)
	}
	if req.Token == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.Ts("globals.messages.empty", "name", "`token`"), nil, envelope.InputError)
	}

	if err := app.user.UnregisterPushToken(auser.ID, req.Token); err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(true)
}

package main

import (
	"strings"

	"github.com/abhinavxd/libredesk/internal/envelope"
	"github.com/abhinavxd/libredesk/internal/user/models"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

// googleMobileAuthReq is the body of POST /api/v1/auth/google-mobile.
// IDToken is the JWT minted by the Flutter app's native Google Sign-In SDK.
type googleMobileAuthReq struct {
	IDToken string `json:"id_token"`
}

// handleGoogleMobileAuth exchanges a Google ID token for a libredesk API key.
//
// The endpoint is unauthenticated by design — the Google ID token IS the
// auth proof. The flow:
//
//  1. Verify the token's signature against Google's published JWKS
//     (audience check is intentionally skipped — the mobile app uses
//     platform-specific Google client IDs and we don't want every fork to
//     enumerate them in config; signature alone proves the token came from
//     Google).
//  2. Require email_verified=true to prove the user controls the address.
//  3. Look up an enabled agent account by that email — no JIT user
//     creation; admins still control who has access.
//  4. Mint a long-lived API key/secret pair the mobile app uses for all
//     subsequent calls via `Authorization: Basic <key>:<secret>`. This
//     replaces the cookie/CSRF flow used by the web SPA — `auth_mobile`
//     adds nothing to the existing API-key middleware path which the
//     desktop "Generate API Key" feature already exercised.
//
// Source: ported from cmd/auth_mobile.go in v1.0.3 commit d4f953b1.
// v2 deltas: uses coreos/go-oidc via app.auth (already vendored) instead
// of google.golang.org/api/idtoken (would have been a new dep with its
// own un-guarded http.Client); routed through the SS2 SSRF-guarded client;
// agent lookup uses v2's generic user.Get(0, email, [agent]) instead of
// v1.0.3's bespoke GetAgent; error strings i18n'd.
func handleGoogleMobileAuth(r *fastglue.Request) error {
	app := r.Context.(*App)

	var req googleMobileAuthReq
	if err := r.Decode(&req, "json"); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("errors.parsingRequest"), nil, envelope.InputError)
	}

	if strings.TrimSpace(req.IDToken) == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.Ts("globals.messages.empty", "name", "`id_token`"), nil, envelope.InputError)
	}

	claims, err := app.auth.VerifyGoogleIDToken(r.RequestCtx, req.IDToken)
	if err != nil {
		app.lo.Error("error validating Google ID token", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, app.i18n.T("auth.googleMobileTokenInvalid"), nil, envelope.GeneralError)
	}

	email := strings.TrimSpace(strings.ToLower(claims.Email))
	if email == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.Ts("globals.messages.empty", "name", "`email`"), nil, envelope.InputError)
	}
	if !claims.EmailVerified {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("auth.emailNotVerified"), nil, envelope.InputError)
	}

	// Look up the agent by email. Returns NotFoundError if no row matches —
	// surface as 401 rather than 404 to avoid leaking which emails are
	// registered as agents.
	user, err := app.user.Get(0, email, []string{models.UserTypeAgent})
	if err != nil {
		app.lo.Warn("google mobile auth: agent not found", "email", email)
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, app.i18n.T("auth.googleMobileNoAgent"), nil, envelope.GeneralError)
	}

	if !user.Enabled {
		return r.SendErrorEnvelope(fasthttp.StatusForbidden, app.i18n.T("user.accountDisabled"), nil, envelope.PermissionError)
	}

	// Generate a fresh API key/secret pair. The plaintext secret is only
	// returned here; the DB stores a bcrypt hash. Re-running this endpoint
	// for the same agent rotates the key (any previously-issued mobile
	// session is invalidated).
	apiKey, apiSecret, err := app.user.GenerateAPIKey(user.ID)
	if err != nil {
		app.lo.Error("error generating API key for mobile auth", "error", err, "user_id", user.ID)
		return sendErrorEnvelope(r, err)
	}

	return r.SendEnvelope(map[string]any{
		"api_key":    apiKey,
		"api_secret": apiSecret,
		"user": map[string]any{
			"id":         user.ID,
			"first_name": user.FirstName,
			"last_name":  user.LastName,
			"email":      user.Email.String,
		},
	})
}

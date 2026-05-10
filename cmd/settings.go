package main

import (
	"encoding/json"
	"net/mail"
	"strings"

	"github.com/abhinavxd/libredesk/internal/envelope"
	"github.com/abhinavxd/libredesk/internal/setting/models"
	"github.com/abhinavxd/libredesk/internal/stringutil"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

// handleGetGeneralSettings fetches general settings, this endpoint is not behind auth as it has no sensitive data and is required for the app to function.
func handleGetGeneralSettings(r *fastglue.Request) error {
	var (
		app = r.Context.(*App)
	)
	out, err := app.setting.GetByPrefix("app")
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	// Unmarshal to set the app.update to the settings, so the frontend can show that an update is available.
	var settings map[string]interface{}
	if err := json.Unmarshal(out, &settings); err != nil {
		app.lo.Error("error unmarshalling settings", "err", err)
		return sendErrorEnvelope(r, envelope.NewError(envelope.GeneralError, app.i18n.T("globals.messages.somethingWentWrong"), nil))
	}
	// Set the app.update to the settings, adding `app` prefix to the key to match the settings structure in db.
	settings["app.update"] = app.update
	// Set app version.
	settings["app.version"] = versionString
	// Set restart required flag.
	settings["app.restart_required"] = app.restartRequired
	return r.SendEnvelope(settings)
}

// handleUpdateGeneralSettings updates general settings.
func handleUpdateGeneralSettings(r *fastglue.Request) error {
	var (
		app = r.Context.(*App)
		req = models.General{}
	)

	if err := r.Decode(&req, "json"); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("globals.messages.badRequest"), nil, envelope.InputError)
	}

	// Trim whitespace from string fields.
	req.SiteName = strings.TrimSpace(req.SiteName)
	req.FaviconURL = strings.TrimSpace(req.FaviconURL)
	req.LogoURL = strings.TrimSpace(req.LogoURL)
	req.Timezone = strings.TrimSpace(req.Timezone)
	// Trim whitespace and trailing slash from root URL.
	req.RootURL = strings.TrimRight(strings.TrimSpace(req.RootURL), "/")

	// Get current language before update.
	app.Lock()
	oldLang := ko.String("app.lang")
	app.Unlock()

	if err := app.setting.Update(req); err != nil {
		return sendErrorEnvelope(r, err)
	}
	// Reload the settings and templates.
	if err := reloadSettings(app); err != nil {
		app.lo.Error("error reloading settings", "error", err)
		return envelope.NewError(envelope.GeneralError, app.i18n.T("globals.messages.somethingWentWrong"), nil)
	}

	// Check if language changed and reload i18n if needed.
	app.Lock()
	newLang := ko.String("app.lang")
	if oldLang != newLang {
		app.lo.Info("language changed, reloading i18n", "old_lang", oldLang, "new_lang", newLang)
		app.i18n = initI18n(app.fs)
		app.lo.Info("reloaded i18n", "old_lang", oldLang, "new_lang", newLang)
	}
	app.Unlock()

	if err := reloadTemplates(app); err != nil {
		app.lo.Error("error reloading templates", "error", err)
		return envelope.NewError(envelope.GeneralError, app.i18n.T("globals.messages.somethingWentWrong"), nil)
	}
	return r.SendEnvelope(true)
}

// handleGetEmailNotificationSettings fetches email notification settings.
func handleGetEmailNotificationSettings(r *fastglue.Request) error {
	var (
		app   = r.Context.(*App)
		notif = models.EmailNotification{}
	)

	out, err := app.setting.GetByPrefix("notification.email")
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	// Unmarshal and filter out password.
	if err := json.Unmarshal(out, &notif); err != nil {
		return sendErrorEnvelope(r, envelope.NewError(envelope.GeneralError, app.i18n.T("globals.messages.somethingWentWrong"), nil))
	}
	if notif.Password != "" {
		notif.Password = strings.Repeat(stringutil.PasswordDummy, 10)
	}
	return r.SendEnvelope(notif)
}

// handleUpdateEmailNotificationSettings updates email notification settings.
func handleUpdateEmailNotificationSettings(r *fastglue.Request) error {
	var (
		app = r.Context.(*App)
		req = models.EmailNotification{}
		cur = models.EmailNotification{}
	)

	if err := r.Decode(&req, "json"); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("globals.messages.badRequest"), nil, envelope.InputError)
	}

	// Trim whitespace from string fields (Password intentionally NOT trimmed).
	req.Host = strings.TrimSpace(req.Host)
	req.Username = strings.TrimSpace(req.Username)
	req.EmailAddress = strings.TrimSpace(req.EmailAddress)
	req.HelloHostname = strings.TrimSpace(req.HelloHostname)
	req.IdleTimeout = strings.TrimSpace(req.IdleTimeout)
	req.WaitTimeout = strings.TrimSpace(req.WaitTimeout)

	out, err := app.setting.GetByPrefix("notification.email")
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	if err := json.Unmarshal(out, &cur); err != nil {
		return sendErrorEnvelope(r, envelope.NewError(envelope.GeneralError, app.i18n.T("globals.messages.somethingWentWrong"), nil))
	}

	// Make sure it's a valid from email address.
	if _, err := mail.ParseAddress(req.EmailAddress); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("validation.invalidFromAddress"), nil, envelope.InputError)
	}

	// Retain current password if not changed.
	if req.Password == "" {
		req.Password = cur.Password
	}

	if err := app.setting.Update(req); err != nil {
		return sendErrorEnvelope(r, err)
	}

	// Email notification settings require app restart to take effect.
	app.Lock()
	app.restartRequired = true
	app.Unlock()

	return r.SendEnvelope(true)
}

// handleGetTrashSettings fetches trash/spam cleanup settings.
func handleGetTrashSettings(r *fastglue.Request) error {
	app := r.Context.(*App)
	out, err := app.setting.GetByPrefix("trash.")
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(json.RawMessage(out))
}

// handleUpdateTrashSettings updates trash/spam cleanup settings.
//
// No reloadSettings(app) call is needed — the trash worker reads via
// setting.GetByPrefix on every cycle (see makeTrashSettingsFunc in main.go) so
// the next hourly tick picks up the change without a restart. If a future trash
// setting needs koanf-backed access, add a reloadSettings call here.
func handleUpdateTrashSettings(r *fastglue.Request) error {
	var (
		app = r.Context.(*App)
		req = models.TrashSettings{}
	)
	if err := r.Decode(&req, "json"); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("globals.messages.badRequest"), nil, envelope.InputError)
	}
	if req.AutoTrashResolvedDays < 0 || req.AutoTrashSpamDays < 0 || req.AutoDeleteDays < 0 || req.ActivityPurgeDays < 0 {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.Ts("globals.messages.invalid", "name", "{globals.terms.day}"), nil, envelope.InputError)
	}
	if err := app.setting.Update(req); err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(true)
}

// handleGetPCISettings fetches PCI redaction notification settings (T3y).
// Returns the raw "pci."-prefixed envelope so the frontend can drive a
// generic key/value form mirroring the trash + AI settings pages.
func handleGetPCISettings(r *fastglue.Request) error {
	app := r.Context.(*App)
	out, err := app.setting.GetByPrefix("pci.")
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(json.RawMessage(out))
}

// handleGetAISettings returns the "ai."-prefixed settings envelope (T3v
// voicemail-transcription toggles for now). Same shape as the trash/PCI
// endpoints — frontend drives a generic key/value form.
func handleGetAISettings(r *fastglue.Request) error {
	app := r.Context.(*App)
	out, err := app.setting.GetByPrefix("ai.")
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(json.RawMessage(out))
}

// handleUpdateAISettings updates the "ai."-prefixed settings (T3v
// voicemail-transcription toggles + T3c RAG system prompt + tuning).
// The transcription pipeline reads via settings.GetAISettings on each
// incoming message, and the RAG handler does the same on each
// generate-response call, so changes take effect immediately without a
// restart.
//
// Partial-save merge: AISettings has no `omitempty` on JSON tags
// (intentional — admins must be able to set fields back to their zero
// value, e.g. clear a custom system prompt to revert to the default,
// or disable transcription). Without merging, a partial save from one
// of the cards on the AISettings.vue page would clobber the others
// with zero defaults. We pre-fetch current settings into the request
// struct, then Decode overlays only the fields actually present in
// the JSON body. Mirrors the pattern v1.0.3 used for password-field
// preservation, generalised here so any subset save round-trips cleanly.
func handleUpdateAISettings(r *fastglue.Request) error {
	app := r.Context.(*App)
	cur, err := app.setting.GetAISettings()
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	req := cur
	if err := r.Decode(&req, "json"); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("globals.messages.badRequest"), nil, envelope.InputError)
	}
	if req.TranscriptionProvider != "" && req.TranscriptionProvider != "openai" && req.TranscriptionProvider != "local" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.Ts("globals.messages.invalidValueFor", "name", "transcription_provider", "options", "openai, local"), nil, envelope.InputError)
	}
	// Clamp the tuning numerics to reject malformed input outright (the
	// rag.go handler also has runtime fallbacks for stale rows, but
	// silent acceptance of nonsense at save-time would let admins
	// believe a value is in effect when it's actually being ignored).
	if req.MaxContextChunks < 0 || req.MaxContextChunks > 50 {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("admin.ai.rag.maxContextChunksInvalid"), nil, envelope.InputError)
	}
	if req.SimilarityThreshold < 0 || req.SimilarityThreshold > 1 {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("admin.ai.rag.similarityThresholdInvalid"), nil, envelope.InputError)
	}
	// T3d external-search bounds. Negative = nonsense; >10 results per
	// endpoint is well past what the LLM can usefully reason about and
	// would balloon prompt-token cost. JSON-string fields (endpoints,
	// headers) are validated at use-time in performExternalSearch
	// because parse failure there is non-fatal — admins can save
	// half-finished JSON during edit and the runtime degrades to "no
	// external search".
	if req.ExternalSearchMaxResults < 0 || req.ExternalSearchMaxResults > 10 {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("admin.ai.externalSearch.maxResultsInvalid"), nil, envelope.InputError)
	}
	if err := app.setting.Update(req); err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(true)
}

// handleUpdatePCISettings updates PCI redaction notification settings.
//
// notify_method is one of "in_app" / "email" / "both" (or empty, treated as
// "both" by the dispatcher). notify_agent_id == 0 disables alerts entirely.
// As with trash, no reloadSettings call is needed — pci_redact.go reads
// settings on every alert via GetPCISettings, so changes take effect on
// the next IMAP-delete failure.
func handleUpdatePCISettings(r *fastglue.Request) error {
	app := r.Context.(*App)
	var req models.PCISettings
	if err := r.Decode(&req, "json"); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("globals.messages.badRequest"), nil, envelope.InputError)
	}
	if req.NotifyMethod != "" && req.NotifyMethod != "in_app" && req.NotifyMethod != "email" && req.NotifyMethod != "both" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.Ts("globals.messages.invalidValueFor", "name", "notify_method", "options", "in_app, email, both"), nil, envelope.InputError)
	}
	if err := app.setting.Update(req); err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(true)
}

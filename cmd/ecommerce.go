package main

import (
	"encoding/json"
	"strings"

	"github.com/abhinavxd/libredesk/internal/ecommerce"
	"github.com/abhinavxd/libredesk/internal/ecommerce/magento1"
	"github.com/abhinavxd/libredesk/internal/envelope"
	"github.com/abhinavxd/libredesk/internal/stringutil"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
	"github.com/zerodha/logf"
)

const (
	ecommerceSettingsKey = "ecommerce"
)

// ecommerceConfigReq is the request structure for ecommerce settings.
type ecommerceConfigReq struct {
	Type         string            `json:"type"`
	BaseURL      string            `json:"base_url"`
	ClientID     string            `json:"client_id"`
	ClientSecret string            `json:"client_secret"`
	ExtraConfig  map[string]string `json:"extra_config,omitempty"`
}

// handleGetEcommerceSettings returns the current ecommerce configuration.
// The client_secret is masked with PasswordDummy if present so the real value
// never leaves the server.
func handleGetEcommerceSettings(r *fastglue.Request) error {
	app := r.Context.(*App)

	out, err := app.setting.GetByPrefix(ecommerceSettingsKey)
	if err != nil {
		// Return empty config if not set.
		return r.SendEnvelope(ecommerceConfigReq{})
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(out, &settings); err != nil {
		return r.SendEnvelope(ecommerceConfigReq{})
	}

	// Build response from settings.
	config := ecommerceConfigReq{
		Type:     getStringFromSettings(settings, "ecommerce.type"),
		BaseURL:  getStringFromSettings(settings, "ecommerce.base_url"),
		ClientID: getStringFromSettings(settings, "ecommerce.client_id"),
	}

	// Mask the client secret if present.
	if secret := getStringFromSettings(settings, "ecommerce.client_secret"); secret != "" {
		config.ClientSecret = strings.Repeat(stringutil.PasswordDummy, 10)
	}

	// Parse extra config.
	if extra := getStringFromSettings(settings, "ecommerce.extra_config"); extra != "" {
		var extraConfig map[string]string
		if json.Unmarshal([]byte(extra), &extraConfig) == nil {
			config.ExtraConfig = extraConfig
		}
	}

	return r.SendEnvelope(config)
}

// handleUpdateEcommerceSettings saves ecommerce configuration. The setting
// manager auto-encrypts ecommerce.client_secret because it is registered in
// encryptedFields. If the client secret is empty or the masked PasswordDummy,
// the existing stored value is preserved.
func handleUpdateEcommerceSettings(r *fastglue.Request) error {
	app := r.Context.(*App)

	var req ecommerceConfigReq
	if err := r.Decode(&req, "json"); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("globals.messages.badRequest"), nil, envelope.InputError)
	}

	// Get current settings to preserve secret if not provided.
	curJSON, _ := app.setting.GetByPrefix(ecommerceSettingsKey)
	var curSettings map[string]interface{}
	if curJSON != nil {
		json.Unmarshal(curJSON, &curSettings)
	}

	// If secret is empty or dummy, retain the existing one.
	if req.ClientSecret == "" || strings.HasPrefix(req.ClientSecret, stringutil.PasswordDummy) {
		if curSettings != nil {
			req.ClientSecret = getStringFromSettings(curSettings, "ecommerce.client_secret")
		}
	}

	// Build the settings map in the flat format used by the settings package.
	// The setting manager will auto-encrypt ecommerce.client_secret since it's
	// in encryptedFields.
	extraJSON := ""
	if req.ExtraConfig != nil {
		b, _ := json.Marshal(req.ExtraConfig)
		extraJSON = string(b)
	}

	settings := map[string]interface{}{
		"ecommerce.type":          req.Type,
		"ecommerce.base_url":      req.BaseURL,
		"ecommerce.client_id":     req.ClientID,
		"ecommerce.client_secret": req.ClientSecret,
		"ecommerce.extra_config":  extraJSON,
	}

	if err := app.setting.Update(settings); err != nil {
		return sendErrorEnvelope(r, err)
	}

	// Reinitialize the ecommerce manager with new settings.
	if err := initEcommerceManager(app); err != nil {
		app.lo.Warn("failed to initialize ecommerce manager after update", "error", err)
	}

	return r.SendEnvelope(true)
}

// handleTestEcommerceConnection tests the ecommerce provider connection.
//
// SS3: the underlying error is logged server-side; the client receives a
// generic "Connection failed. Check your settings and try again." message
// to avoid leaking internal details (auth backends, hostnames, stack traces).
func handleTestEcommerceConnection(r *fastglue.Request) error {
	app := r.Context.(*App)

	var req ecommerceConfigReq
	if err := r.Decode(&req, "json"); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("globals.messages.badRequest"), nil, envelope.InputError)
	}

	// If secret is empty or dummy, get from stored config.
	if req.ClientSecret == "" || strings.HasPrefix(req.ClientSecret, stringutil.PasswordDummy) {
		curJSON, _ := app.setting.GetByPrefix(ecommerceSettingsKey)
		if curJSON != nil {
			var curSettings map[string]interface{}
			if json.Unmarshal(curJSON, &curSettings) == nil {
				// GetByPrefix returns decrypted values.
				req.ClientSecret = getStringFromSettings(curSettings, "ecommerce.client_secret")
			}
		}
	}

	// Validate required fields.
	if req.Type == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.Ts("globals.messages.required", "name", "{globals.terms.provider}"), nil, envelope.InputError)
	}
	if req.BaseURL == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.Ts("globals.messages.required", "name", "{globals.terms.url}"), nil, envelope.InputError)
	}
	if req.ClientSecret == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.Ts("globals.messages.required", "name", "{globals.terms.clientSecret}"), nil, envelope.InputError)
	}

	// Create provider for testing.
	config := ecommerce.ProviderConfig{
		Type:         req.Type,
		BaseURL:      req.BaseURL,
		ClientID:     req.ClientID,
		ClientSecret: req.ClientSecret,
		ExtraConfig:  req.ExtraConfig,
	}

	provider, err := createEcommerceProvider(config, app.lo)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, err.Error(), nil, envelope.InputError)
	}
	if provider == nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("validation.invalidProvider"), nil, envelope.InputError)
	}

	// Test the connection. SS3: log full error server-side, return generic
	// message to client.
	if err := provider.TestConnection(r.RequestCtx); err != nil {
		app.lo.Error("ecommerce connection test failed", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("admin.ecommerce.connectionFailed"), nil, envelope.InputError)
	}

	return r.SendEnvelope(map[string]string{"status": "ok", "message": "Connection successful"})
}

// handleGetEcommerceStatus returns whether ecommerce is configured (for UI
// visibility / conditional rendering of "+ Orders" button).
func handleGetEcommerceStatus(r *fastglue.Request) error {
	app := r.Context.(*App)

	configured := app.ecommerce != nil && app.ecommerce.IsConfigured()
	return r.SendEnvelope(map[string]bool{"configured": configured})
}

// handleTestEcommerceCustomerLookup tests looking up a customer + recent
// orders + per-message order matching by email. The full multi-stage
// EcommerceContext (including Warnings populated by the manager) is returned
// so the admin Test Customer panel can surface auth/lookup failures (T3ae(d)).
func handleTestEcommerceCustomerLookup(r *fastglue.Request) error {
	app := r.Context.(*App)

	email := string(r.RequestCtx.QueryArgs().Peek("email"))
	if email == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.Ts("globals.messages.required", "name", "{globals.terms.email}"), nil, envelope.InputError)
	}

	// T3ae diagnostic logging.
	app.lo.Info("ecommerce test lookup request", "kind", "customer", "email", email)

	if app.ecommerce == nil || !app.ecommerce.IsConfigured() {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("admin.ecommerce.notConfigured"), nil, envelope.InputError)
	}

	ctx, err := app.ecommerce.GatherFullContext(r.RequestCtx, email, nil, 10)
	if err != nil {
		app.lo.Error("ecommerce customer lookup failed", "error", err, "email", email)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, app.i18n.T("globals.messages.somethingWentWrong"), nil, envelope.GeneralError)
	}

	return r.SendEnvelope(ctx)
}

// handleTestEcommerceOrderLookup tests looking up an order by its display
// number. Returns 404 with the underlying error string when the order isn't
// found (so admins see the diagnostic), success envelope otherwise.
func handleTestEcommerceOrderLookup(r *fastglue.Request) error {
	app := r.Context.(*App)

	orderNumber := string(r.RequestCtx.QueryArgs().Peek("order_number"))
	if orderNumber == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.Ts("globals.messages.required", "name", "{globals.terms.orderNumber}"), nil, envelope.InputError)
	}

	// T3ae diagnostic logging.
	app.lo.Info("ecommerce test lookup request", "kind", "order", "order_number", orderNumber)

	if app.ecommerce == nil || !app.ecommerce.IsConfigured() {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("admin.ecommerce.notConfigured"), nil, envelope.InputError)
	}

	order, err := app.ecommerce.GetOrderByNumber(r.RequestCtx, orderNumber)
	if err != nil {
		app.lo.Warn("ecommerce order lookup failed", "error", err, "order_number", orderNumber)
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, app.i18n.T("admin.ecommerce.orderNotFound"), nil, envelope.NotFoundError)
	}

	return r.SendEnvelope(order)
}

// createEcommerceProvider creates a provider instance from config. Returns
// (nil, nil) for unknown provider types so callers can distinguish "type not
// supported" from "construction failed".
func createEcommerceProvider(config ecommerce.ProviderConfig, lo *logf.Logger) (ecommerce.Provider, error) {
	switch config.Type {
	case "magento1":
		return magento1.New(config, lo)
	// Future providers:
	// case "magento2":
	//     return magento2.New(config, lo)
	// case "shopify":
	//     return shopify.New(config, lo)
	default:
		return nil, nil
	}
}

// initEcommerceManager initializes the ecommerce manager from stored
// settings. Missing/invalid config is not an error — app.ecommerce is set to
// nil and downstream callers (rag.go, status handler) gracefully no-op.
func initEcommerceManager(app *App) error {
	settingsJSON, err := app.setting.GetByPrefix(ecommerceSettingsKey)
	if err != nil {
		app.ecommerce = nil
		return nil // Not an error, just not configured.
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(settingsJSON, &settings); err != nil {
		app.ecommerce = nil
		return nil
	}

	providerType := getStringFromSettings(settings, "ecommerce.type")
	if providerType == "" {
		app.ecommerce = nil
		return nil
	}

	// Client secret is already decrypted by GetByPrefix.
	clientSecret := getStringFromSettings(settings, "ecommerce.client_secret")

	// Parse extra config.
	var extraConfig map[string]string
	if extra := getStringFromSettings(settings, "ecommerce.extra_config"); extra != "" {
		json.Unmarshal([]byte(extra), &extraConfig)
	}

	config := ecommerce.ProviderConfig{
		Type:         providerType,
		BaseURL:      getStringFromSettings(settings, "ecommerce.base_url"),
		ClientID:     getStringFromSettings(settings, "ecommerce.client_id"),
		ClientSecret: clientSecret,
		ExtraConfig:  extraConfig,
	}

	provider, err := createEcommerceProvider(config, app.lo)
	if err != nil {
		app.lo.Error("failed to create ecommerce provider", "error", err)
		app.ecommerce = nil
		return err
	}

	if provider == nil {
		app.lo.Warn("unknown ecommerce provider type", "type", providerType)
		app.ecommerce = nil
		return nil
	}

	app.ecommerce = ecommerce.NewManager(provider, *app.lo)
	app.lo.Info("ecommerce provider initialized", "type", providerType)
	return nil
}

// getStringFromSettings safely extracts a string value from a settings map.
func getStringFromSettings(settings map[string]interface{}, key string) string {
	if val, ok := settings[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

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
)

const (
	ecommerceSettingsKey = "ecommerce"
)

// ecommerceConfigReq is the request structure for ecommerce settings
type ecommerceConfigReq struct {
	Type         string            `json:"type"`
	BaseURL      string            `json:"base_url"`
	ClientID     string            `json:"client_id"`
	ClientSecret string            `json:"client_secret"`
	ExtraConfig  map[string]string `json:"extra_config,omitempty"`
}

// handleGetEcommerceSettings returns the current ecommerce configuration
func handleGetEcommerceSettings(r *fastglue.Request) error {
	app := r.Context.(*App)

	out, err := app.setting.GetByPrefix(ecommerceSettingsKey)
	if err != nil {
		// Return empty config if not set
		return r.SendEnvelope(ecommerceConfigReq{})
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(out, &settings); err != nil {
		return r.SendEnvelope(ecommerceConfigReq{})
	}

	// Build response from settings
	config := ecommerceConfigReq{
		Type:     getStringFromSettings(settings, "ecommerce.type"),
		BaseURL:  getStringFromSettings(settings, "ecommerce.base_url"),
		ClientID: getStringFromSettings(settings, "ecommerce.client_id"),
	}

	// Mask the client secret if present
	if secret := getStringFromSettings(settings, "ecommerce.client_secret"); secret != "" {
		config.ClientSecret = strings.Repeat(stringutil.PasswordDummy, 10)
	}

	// Parse extra config
	if extra := getStringFromSettings(settings, "ecommerce.extra_config"); extra != "" {
		var extraConfig map[string]string
		if json.Unmarshal([]byte(extra), &extraConfig) == nil {
			config.ExtraConfig = extraConfig
		}
	}

	return r.SendEnvelope(config)
}

// handleUpdateEcommerceSettings saves ecommerce configuration
func handleUpdateEcommerceSettings(r *fastglue.Request) error {
	app := r.Context.(*App)

	var req ecommerceConfigReq
	if err := r.Decode(&req, "json"); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("globals.messages.badRequest"), nil, envelope.InputError)
	}

	// Get current settings to preserve secret if not provided
	curJSON, _ := app.setting.GetByPrefix(ecommerceSettingsKey)
	var curSettings map[string]interface{}
	if curJSON != nil {
		json.Unmarshal(curJSON, &curSettings)
	}

	// If secret is empty or dummy, retain the existing one
	if req.ClientSecret == "" || strings.HasPrefix(req.ClientSecret, stringutil.PasswordDummy) {
		if curSettings != nil {
			req.ClientSecret = getStringFromSettings(curSettings, "ecommerce.client_secret")
		}
	}

	// Build the settings map in the flat format used by the settings package
	// The setting manager will auto-encrypt ecommerce.client_secret since it's in encryptedFields
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

	// Reinitialize the ecommerce manager with new settings
	if err := initEcommerceManager(app); err != nil {
		app.lo.Warn("failed to initialize ecommerce manager after update", "error", err)
	}

	return r.SendEnvelope(true)
}

// handleTestEcommerceConnection tests the ecommerce provider connection
func handleTestEcommerceConnection(r *fastglue.Request) error {
	app := r.Context.(*App)

	var req ecommerceConfigReq
	if err := r.Decode(&req, "json"); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("globals.messages.badRequest"), nil, envelope.InputError)
	}

	// If secret is empty or dummy, get from stored config
	if req.ClientSecret == "" || strings.HasPrefix(req.ClientSecret, stringutil.PasswordDummy) {
		curJSON, _ := app.setting.GetByPrefix(ecommerceSettingsKey)
		if curJSON != nil {
			var curSettings map[string]interface{}
			if json.Unmarshal(curJSON, &curSettings) == nil {
				// GetByPrefix returns decrypted values
				req.ClientSecret = getStringFromSettings(curSettings, "ecommerce.client_secret")
			}
		}
	}

	// Validate required fields
	if req.Type == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Provider type is required", nil, envelope.InputError)
	}
	if req.BaseURL == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Base URL is required", nil, envelope.InputError)
	}
	if req.ClientSecret == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Client secret is required", nil, envelope.InputError)
	}

	// Create provider for testing
	config := ecommerce.ProviderConfig{
		Type:         req.Type,
		BaseURL:      req.BaseURL,
		ClientID:     req.ClientID,
		ClientSecret: req.ClientSecret,
		ExtraConfig:  req.ExtraConfig,
	}

	provider, err := createEcommerceProvider(config)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, err.Error(), nil, envelope.InputError)
	}
	if provider == nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Unknown provider type: "+req.Type, nil, envelope.InputError)
	}

	// Test the connection
	if err := provider.TestConnection(r.RequestCtx); err != nil {
		app.lo.Error("ecommerce connection test failed", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Connection failed. Check your settings and try again.", nil, envelope.InputError)
	}

	return r.SendEnvelope(map[string]string{"status": "ok", "message": "Connection successful"})
}

// handleGetEcommerceStatus returns whether ecommerce is configured (for UI visibility)
func handleGetEcommerceStatus(r *fastglue.Request) error {
	app := r.Context.(*App)

	configured := app.ecommerce != nil && app.ecommerce.IsConfigured()
	return r.SendEnvelope(map[string]bool{"configured": configured})
}

// createEcommerceProvider creates a provider instance from config
func createEcommerceProvider(config ecommerce.ProviderConfig) (ecommerce.Provider, error) {
	switch config.Type {
	case "magento1":
		return magento1.New(config)
	// Future providers:
	// case "magento2":
	//     return magento2.New(config)
	// case "shopify":
	//     return shopify.New(config)
	default:
		return nil, nil
	}
}

// initEcommerceManager initializes the ecommerce manager from stored settings
func initEcommerceManager(app *App) error {
	settingsJSON, err := app.setting.GetByPrefix(ecommerceSettingsKey)
	if err != nil {
		app.ecommerce = nil
		return nil // Not an error, just not configured
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

	// Client secret is already decrypted by GetByPrefix
	clientSecret := getStringFromSettings(settings, "ecommerce.client_secret")

	// Parse extra config
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

	provider, err := createEcommerceProvider(config)
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

// getStringFromSettings safely extracts a string value from settings map
func getStringFromSettings(settings map[string]interface{}, key string) string {
	if val, ok := settings[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// handleTestEcommerceCustomerLookup tests looking up a customer by email
func handleTestEcommerceCustomerLookup(r *fastglue.Request) error {
	app := r.Context.(*App)

	email := string(r.RequestCtx.QueryArgs().Peek("email"))
	if email == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Email is required", nil, envelope.InputError)
	}

	app.lo.Info("ecommerce test lookup request", "kind", "customer", "email", email)

	if app.ecommerce == nil || !app.ecommerce.IsConfigured() {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Ecommerce not configured", nil, envelope.InputError)
	}

	ctx, err := app.ecommerce.GatherFullContext(r.RequestCtx, email, nil, 10)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Lookup failed: "+err.Error(), nil, envelope.GeneralError)
	}

	return r.SendEnvelope(ctx)
}

// handleTestEcommerceOrderLookup tests looking up an order by number
func handleTestEcommerceOrderLookup(r *fastglue.Request) error {
	app := r.Context.(*App)

	orderNumber := string(r.RequestCtx.QueryArgs().Peek("order_number"))
	if orderNumber == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Order number is required", nil, envelope.InputError)
	}

	app.lo.Info("ecommerce test lookup request", "kind", "order", "order_number", orderNumber)

	if app.ecommerce == nil || !app.ecommerce.IsConfigured() {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Ecommerce not configured", nil, envelope.InputError)
	}

	order, err := app.ecommerce.GetOrderByNumber(r.RequestCtx, orderNumber)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Order not found: "+err.Error(), nil, envelope.NotFoundError)
	}

	return r.SendEnvelope(order)
}

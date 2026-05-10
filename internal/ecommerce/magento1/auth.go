package magento1

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/zerodha/logf"
)

type tokenResponse struct {
	Token     string `json:"token"`
	TokenType string `json:"token_type"`
	ExpiresIn int    `json:"expires_in"`
}

type authClient struct {
	baseURL      string
	clientID     string
	clientSecret string
	userAgent    string

	httpClient *http.Client

	mu          sync.RWMutex
	token       string
	tokenExpiry time.Time

	lo *logf.Logger
}

// debugLogOnce gates a one-time raw response body log so we can confirm the
// wire format from Maho without spamming the logs every refresh.
var debugLogOnce sync.Once

// jwtPattern matches a JWT-shaped string (three base64-url chunks separated by dots).
var jwtPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+$`)

func newAuthClient(baseURL, clientID, clientSecret, userAgent string, lo *logf.Logger) *authClient {
	return &authClient{
		baseURL:      baseURL,
		clientID:     clientID,
		clientSecret: clientSecret,
		userAgent:    userAgent,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		lo:           lo,
	}
}

func (a *authClient) getToken() (string, error) {
	a.mu.RLock()
	if a.token != "" && time.Now().Before(a.tokenExpiry) {
		token := a.token
		a.mu.RUnlock()
		return token, nil
	}
	a.mu.RUnlock()
	return a.refreshToken()
}

func (a *authClient) refreshToken() (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Double-check after lock
	if a.token != "" && time.Now().Before(a.tokenExpiry) {
		return a.token, nil
	}

	// Maho API Platform v2 supports OAuth2 client_credentials grant for
	// service integrations (the human-user customer grant uses email/password).
	// LibreDesk is an integration, so we use client_credentials.
	payload := map[string]string{
		"grant_type":    "client_credentials",
		"client_id":     a.clientID,
		"client_secret": a.clientSecret,
	}
	body, _ := json.Marshal(payload)

	tokenURL := a.baseURL + "/api/rest/v2/auth/token"
	a.lo.Debug("requesting token", "url", tokenURL)

	req, err := http.NewRequest(http.MethodPost, tokenURL, bytes.NewBuffer(body))
	if err != nil {
		return "", fmt.Errorf("build POST %s failed: %w", tokenURL, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", a.userAgent)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("POST %s failed: %w", tokenURL, err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		bodyStr := string(respBody)
		if len(bodyStr) > 200 {
			bodyStr = bodyStr[:200] + "..."
		}
		a.lo.Warn("token request failed", "status", resp.StatusCode)
		return "", fmt.Errorf("POST %s returned %d: %s", tokenURL, resp.StatusCode, bodyStr)
	}

	// One-time debug log of raw response body so we can confirm the
	// wire format from Maho. Truncated to keep logs readable.
	debugLogOnce.Do(func() {
		preview := string(respBody)
		if len(preview) > 500 {
			preview = preview[:500] + "...(truncated)"
		}
		a.lo.Debug("raw token response (one-time)", "status", resp.StatusCode, "body", preview)
	})

	var tokenResp tokenResponse
	if err := json.Unmarshal(respBody, &tokenResp); err != nil {
		return "", fmt.Errorf("failed to decode token response: %w", err)
	}

	tokenStr := tokenResp.Token

	// Fallback: if the documented field name changed, scan the JSON for any
	// string value that looks like a JWT (three base64-url chunks split by dots).
	if tokenStr == "" {
		var raw map[string]interface{}
		if err := json.Unmarshal(respBody, &raw); err == nil {
			for k, v := range raw {
				if s, ok := v.(string); ok && jwtPattern.MatchString(s) {
					a.lo.Warn("token field 'token' empty; using JWT-shaped value from alternate field", "field", k)
					tokenStr = s
					break
				}
			}
		}
	}

	if tokenStr == "" {
		preview := string(respBody)
		if len(preview) > 200 {
			preview = preview[:200] + "..."
		}
		return "", fmt.Errorf("token response did not contain a JWT-shaped token; body=%s", strings.TrimSpace(preview))
	}

	a.lo.Info("token obtained", "expires_in_seconds", tokenResp.ExpiresIn)
	a.token = tokenStr

	// Default to a 1-hour TTL if expires_in isn't provided; refresh 5 minutes
	// before expiry. (Many JWT auth endpoints omit expires_in.)
	expSec := tokenResp.ExpiresIn
	if expSec <= 0 {
		expSec = 3600
	}
	a.tokenExpiry = time.Now().Add(time.Duration(expSec-300) * time.Second)
	return a.token, nil
}

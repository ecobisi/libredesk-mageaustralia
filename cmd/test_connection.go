package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/mail"
	smtplib "net/smtp"
	"strings"
	"time"

	"github.com/abhinavxd/libredesk/internal/envelope"
	emailchan "github.com/abhinavxd/libredesk/internal/inbox/channel/email"
	emailoauth "github.com/abhinavxd/libredesk/internal/inbox/channel/email/oauth"
	imodels "github.com/abhinavxd/libredesk/internal/inbox/models"
	"github.com/abhinavxd/libredesk/internal/setting/models"
	"github.com/abhinavxd/libredesk/internal/stringutil"
	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/knadh/smtppool"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

// testInboxRequest is the body for POST /api/v1/inboxes/test-connection.
type testInboxRequest struct {
	InboxID   int                 `json:"inbox_id"`
	IMAP      *imodels.IMAPConfig `json:"imap"`
	SMTP      *imodels.SMTPConfig `json:"smtp"`
	AuthType  string              `json:"auth_type"`
	TestEmail string              `json:"test_email"`
}

// testInboxResponse is returned from POST /api/v1/inboxes/test-connection.
type testInboxResponse struct {
	Success  bool     `json:"success"`
	IMAPLogs []string `json:"imap_logs"`
	SMTPLogs []string `json:"smtp_logs"`
}

// testEmailRequest is the body for POST /api/v1/settings/notifications/email/test.
type testEmailRequest struct {
	models.EmailNotification
	TestEmail string `json:"test_email"`
}

// testEmailResponse is returned from POST /api/v1/settings/notifications/email/test.
type testEmailResponse struct {
	Success bool     `json:"success"`
	Logs    []string `json:"logs"`
}

// handleTestInboxConnection tests IMAP and/or SMTP connection with the provided config.
func handleTestInboxConnection(r *fastglue.Request) error {
	var (
		app = r.Context.(*App)
		req = testInboxRequest{}
	)

	if err := r.Decode(&req, "json"); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("globals.messages.badRequest"), nil, envelope.InputError)
	}

	// When an existing inbox ID is provided, fetch the stored config so we can
	// substitute dummy placeholders the frontend can't know the real values of
	// (masked passwords, OAuth tokens).
	var storedCfg *imodels.Config
	if req.InboxID != 0 {
		dbInbox, err := app.inbox.GetDBRecord(req.InboxID)
		if err == nil {
			var cfg imodels.Config
			if jsonErr := json.Unmarshal(dbInbox.Config, &cfg); jsonErr == nil {
				storedCfg = &cfg
			}
		}
	}

	// Substitute dummy passwords / OAuth tokens from stored config when present.
	if storedCfg != nil {
		if req.IMAP != nil && len(storedCfg.IMAP) > 0 {
			if req.IMAP.Password == "" || strings.Contains(req.IMAP.Password, stringutil.PasswordDummy) {
				req.IMAP.Password = storedCfg.IMAP[0].Password
			}
		}
		if req.SMTP != nil && len(storedCfg.SMTP) > 0 {
			if req.SMTP.Password == "" || strings.Contains(req.SMTP.Password, stringutil.PasswordDummy) {
				req.SMTP.Password = storedCfg.SMTP[0].Password
			}
		}
		// Use the stored auth_type when caller didn't set one.
		if req.AuthType == "" {
			req.AuthType = storedCfg.AuthType
		}
	}

	resp := testInboxResponse{Success: true}

	// Test IMAP if config provided.
	if req.IMAP != nil && req.IMAP.Host != "" {
		imapLogs, imapOK := testIMAPConnection(req.IMAP, req.AuthType, storedCfg)
		resp.IMAPLogs = imapLogs
		if !imapOK {
			resp.Success = false
		}
	}

	// Test SMTP if config provided.
	if req.SMTP != nil && req.SMTP.Host != "" {
		smtpLogs, smtpOK := testSMTPConn(req.SMTP, req.TestEmail, storedCfg)
		resp.SMTPLogs = smtpLogs
		if !smtpOK {
			resp.Success = false
		}
	}

	return r.SendEnvelope(resp)
}

// handleTestEmailNotificationSettings tests the notification email SMTP settings.
func handleTestEmailNotificationSettings(r *fastglue.Request) error {
	var (
		app  = r.Context.(*App)
		req  = testEmailRequest{}
		cur  = models.EmailNotification{}
		logs = []string{}
	)

	addLog := func(msg string, args ...any) {
		logs = append(logs, fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), fmt.Sprintf(msg, args...)))
	}

	if err := r.Decode(&req, "json"); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("globals.messages.badRequest"), nil, envelope.InputError)
	}

	// Validate test email.
	if req.TestEmail == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.Ts("globals.messages.required", "name", "{globals.terms.email}"), nil, envelope.InputError)
	}
	if _, err := mail.ParseAddress(req.TestEmail); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("validation.invalidEmail"), nil, envelope.InputError)
	}

	addLog("Starting SMTP test to %s", req.TestEmail)

	// Fetch current stored settings to retrieve password when dummy is passed.
	out, err := app.setting.GetByPrefix("notification.email")
	if err != nil {
		addLog("Error fetching current settings: %v", err)
		return r.SendEnvelope(testEmailResponse{Success: false, Logs: logs})
	}
	if err := json.Unmarshal(out, &cur); err != nil {
		addLog("Error parsing current settings: %v", err)
		return r.SendEnvelope(testEmailResponse{Success: false, Logs: logs})
	}

	// Use the stored password when the request carries a dummy placeholder.
	password := req.Password
	if password == "" || strings.Contains(password, stringutil.PasswordDummy) {
		password = cur.Password
	}

	ok := runSMTPSession(
		req.Host, req.Port,
		req.Username, password,
		req.AuthProtocol, req.TLSType,
		req.EmailAddress, req.TestEmail,
		req.HelloHostname,
		req.TLSSkipVerify,
		addLog,
	)
	return r.SendEnvelope(testEmailResponse{Success: ok, Logs: logs})
}

// runSMTPSession dials an SMTP server, authenticates, optionally sends a test
// message, and returns whether the session succeeded. All diagnostic output is
// written via addLog. Both handleTestEmailNotificationSettings and testSMTPConn
// use this helper so TLS-type validation, timeout handling, and auth are
// applied uniformly.
func runSMTPSession(
	host string, port int,
	username, password string,
	authProtocol, tlsType string,
	fromAddress, testEmail string,
	helloHostname string,
	tlsSkipVerify bool,
	addLog func(string, ...any),
) bool {
	serverAddr := fmt.Sprintf("%s:%d", host, port)
	addLog("Connecting to SMTP server: %s", serverAddr)

	tlsConfig := &tls.Config{
		ServerName:         host,
		InsecureSkipVerify: tlsSkipVerify,
	}

	// Dial with a 30-second client-side timeout so slow servers fail fast
	// rather than blowing past the frontend axios timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var client *smtplib.Client

	switch tlsType {
	case "tls":
		addLog("Using SSL/TLS connection")
		tlsDialer := tls.Dialer{NetDialer: &net.Dialer{}, Config: tlsConfig}
		conn, err := tlsDialer.DialContext(ctx, "tcp", serverAddr)
		if err != nil {
			addLog("TLS connection failed: %v", err)
			return false
		}
		defer conn.Close()
		var clientErr error
		client, clientErr = smtplib.NewClient(conn, host)
		if clientErr != nil {
			addLog("Failed to create SMTP client: %v", clientErr)
			return false
		}
	case "none", "starttls":
		d := net.Dialer{}
		conn, err := d.DialContext(ctx, "tcp", serverAddr)
		if err != nil {
			addLog("Connection failed: %v", err)
			return false
		}
		var clientErr error
		client, clientErr = smtplib.NewClient(conn, host)
		if clientErr != nil {
			addLog("Failed to create SMTP client: %v", clientErr)
			conn.Close()
			return false
		}
	default:
		addLog("Unknown TLS type: %s — must be one of: none, starttls, tls", tlsType)
		return false
	}
	defer client.Close()
	addLog("Connected successfully")

	// EHLO.
	hostname := helloHostname
	if hostname == "" {
		hostname = "localhost"
	}
	addLog("Sending EHLO %s", hostname)
	if err := client.Hello(hostname); err != nil {
		addLog("EHLO failed: %v", err)
		return false
	}

	// STARTTLS if required.
	if tlsType == "starttls" {
		addLog("Starting TLS (STARTTLS)")
		if err := client.StartTLS(tlsConfig); err != nil {
			addLog("STARTTLS failed: %v", err)
			return false
		}
		addLog("TLS connection established")
	}

	// Authenticate if credentials provided.
	if username != "" && password != "" {
		addLog("Authenticating as %s using %s", username, authProtocol)
		var auth smtplib.Auth
		switch authProtocol {
		case "plain":
			auth = smtplib.PlainAuth("", username, password, host)
		case "login":
			auth = &smtppool.LoginAuth{Username: username, Password: password}
		case "cram":
			auth = smtplib.CRAMMD5Auth(username, password)
		case "none", "":
			addLog("No authentication required")
		default:
			auth = smtplib.PlainAuth("", username, password, host)
		}
		if auth != nil {
			if err := client.Auth(auth); err != nil {
				addLog("Authentication failed: %v", err)
				return false
			}
			addLog("Authentication successful")
		}
	}

	// Send test email if address provided.
	if testEmail != "" {
		fromAddr := fromAddress
		if fromAddr == "" {
			fromAddr = username
		}
		addLog("Setting sender: %s", fromAddr)
		if err := client.Mail(fromAddr); err != nil {
			addLog("MAIL FROM failed: %v", err)
			return false
		}
		addLog("Setting recipient: %s", testEmail)
		if err := client.Rcpt(testEmail); err != nil {
			addLog("RCPT TO failed: %v", err)
			return false
		}
		addLog("Sending test message")
		w, err := client.Data()
		if err != nil {
			addLog("DATA command failed: %v", err)
			return false
		}
		msg := fmt.Sprintf(
			"From: %s\r\nTo: %s\r\nSubject: LibreDesk SMTP Test\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n"+
				"This is a test email from LibreDesk to verify your SMTP settings are working correctly.\r\n\r\nSent at: %s",
			fromAddr, testEmail, time.Now().Format(time.RFC1123),
		)
		if _, err := w.Write([]byte(msg)); err != nil {
			addLog("Failed to write message: %v", err)
			return false
		}
		if err := w.Close(); err != nil {
			addLog("Failed to close message: %v", err)
			return false
		}
		addLog("Test email sent successfully!")
	}

	addLog("SMTP test completed successfully!")
	client.Quit()
	return true
}

// testIMAPConnection dials the IMAP server and tries to authenticate.
// When authType is "oauth2" and storedCfg contains a valid OAuth config,
// XOAUTH2 is used instead of basic Login. Credentials are never echoed in logs.
func testIMAPConnection(cfg *imodels.IMAPConfig, authType string, storedCfg *imodels.Config) ([]string, bool) {
	logs := []string{}
	addLog := func(msg string, args ...any) {
		logs = append(logs, fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), fmt.Sprintf(msg, args...)))
	}

	address := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	addLog("Connecting to IMAP server: %s", address)

	imapOptions := &imapclient.Options{
		TLSConfig: &tls.Config{
			InsecureSkipVerify: cfg.TLSSkipVerify,
		},
	}

	var (
		client *imapclient.Client
		err    error
	)

	switch cfg.TLSType {
	case "none":
		addLog("Using plain connection (no TLS)")
		client, err = imapclient.DialInsecure(address, imapOptions)
	case "starttls":
		addLog("Using STARTTLS connection")
		client, err = imapclient.DialStartTLS(address, imapOptions)
	case "tls":
		addLog("Using SSL/TLS connection")
		client, err = imapclient.DialTLS(address, imapOptions)
	default:
		addLog("Unknown TLS type: %s — must be one of: none, starttls, tls", cfg.TLSType)
		return logs, false
	}

	if err != nil {
		addLog("Connection failed: %v", err)
		return logs, false
	}
	defer client.Logout()
	addLog("Connected successfully")

	// Authenticate: use XOAUTH2 for OAuth inboxes, basic Login otherwise.
	if authType == imodels.AuthTypeOAuth2 && storedCfg != nil && storedCfg.OAuth != nil {
		addLog("Authenticating as %s using XOAUTH2", cfg.Username)

		// Refresh the token if needed before authenticating.
		oauthCfg := storedCfg.OAuth
		if emailoauth.IsTokenExpired(oauthCfg.ExpiresAt) {
			addLog("OAuth token expired — attempting refresh")
			refreshed, err := emailchan.RefreshOAuthConfig(oauthCfg)
			if err != nil {
				addLog("OAuth token refresh failed: %v", err)
				return logs, false
			}
			oauthCfg = refreshed
			addLog("OAuth token refreshed successfully")
		}

		saslClient := &xoauth2IMAPSASLClient{
			username: cfg.Username,
			token:    oauthCfg.AccessToken,
		}
		if err := client.Authenticate(saslClient); err != nil {
			addLog("XOAUTH2 authentication failed: %v", err)
			return logs, false
		}
	} else {
		addLog("Authenticating as %s using basic login", cfg.Username)
		if err := client.Login(cfg.Username, cfg.Password).Wait(); err != nil {
			addLog("Authentication failed: %v", err)
			return logs, false
		}
	}
	addLog("Authentication successful")

	// Select mailbox.
	addLog("Selecting mailbox: %s", cfg.Mailbox)
	mbox, err := client.Select(cfg.Mailbox, &imap.SelectOptions{ReadOnly: true}).Wait()
	if err != nil {
		addLog("Failed to select mailbox: %v", err)
		return logs, false
	}
	addLog("Mailbox selected — %d messages", mbox.NumMessages)
	addLog("IMAP test completed successfully!")
	return logs, true
}

// testSMTPConn dials the SMTP server and tries to authenticate.
// A test email is sent only when testEmail is non-empty.
// Credentials are never echoed in logs.
func testSMTPConn(cfg *imodels.SMTPConfig, testEmail string, storedCfg *imodels.Config) ([]string, bool) {
	logs := []string{}
	addLog := func(msg string, args ...any) {
		logs = append(logs, fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), fmt.Sprintf(msg, args...)))
	}

	ok := runSMTPSession(
		cfg.Host, cfg.Port,
		cfg.Username, cfg.Password,
		cfg.AuthProtocol, cfg.TLSType,
		"", testEmail,
		cfg.HelloHostname,
		cfg.TLSSkipVerify,
		addLog,
	)
	return logs, ok
}

// xoauth2IMAPSASLClient is a local SASL client for IMAP XOAUTH2 testing.
// It mirrors the unexported xoauth2IMAPClient in internal/inbox/channel/email/xoauth2.go
// but lives here so cmd/ can use it without importing the email package's internal type.
type xoauth2IMAPSASLClient struct {
	username string
	token    string
}

func (c *xoauth2IMAPSASLClient) Start() (string, []byte, error) {
	authString := fmt.Sprintf("user=%s\x01auth=Bearer %s\x01\x01", c.username, c.token)
	return "XOAUTH2", []byte(authString), nil
}

func (c *xoauth2IMAPSASLClient) Next(challenge []byte) ([]byte, error) {
	return nil, nil
}

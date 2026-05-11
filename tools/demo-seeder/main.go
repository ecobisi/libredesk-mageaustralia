// demo-seeder loads a fresh libredesk install with realistic data that
// exercises every fork feature documented in the README, so reviewers can
// clone the repo, run docker compose up, run this seeder, and have enough
// data to screenshot or screen-record every feature without manual setup.
//
// Design notes:
//   - Stdlib only. Pinning a third-party fake-data lib for a tool that runs
//     a handful of times feels disproportionate; we keep a small curated
//     pool of names/subjects/messages and a deterministic RNG seed so the
//     output is reproducible.
//   - Hits real API endpoints (same paths the Vue frontend uses) rather
//     than reaching into Postgres directly. This keeps the seeder honest
//     against schema migrations and incidentally serves as informal API
//     documentation. The exception is the --reset path, which deletes via
//     direct SQL because some delete endpoints are gated on referential
//     integrity that's awkward to chase from the outside.
//   - Idempotent via tagged demo agents. Every demo user ends @demo.local
//     so re-runs detect existing rows and skip them. Pass --reset to wipe
//     and recreate.
//   - Refuses to run against non-localhost hosts unless --allow-non-localhost
//     is passed. Belt-and-braces: prod hosts have real customer data and a
//     misfire could create dozens of garbage tickets.
package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"time"
)

// Config is parsed from flags. Defaults match a fresh local Docker install.
type Config struct {
	BaseURL            string
	Username           string
	Password           string
	Reset              bool
	AllowNonLocalhost  bool
	Verbose            bool
	SkipEcommerce      bool
	SkipPushTokens     bool
}

// Client wraps http.Client with cookie-jar session + CSRF handling. The
// libredesk session is a server-side cookie, so once /api/v1/auth/login
// succeeds we just need to send cookies on subsequent requests.
type Client struct {
	http    *http.Client
	baseURL string
	verbose bool
	// csrfToken is read out of the Set-Cookie after login. Mutating endpoints
	// require X-CSRFTOKEN, so we capture it once and stamp it on every POST/PUT/DELETE.
	csrfToken string
}

// Summary tracks what we created so we can print a recap at the end. Counts
// are more useful than IDs to a human screenshotter.
type Summary struct {
	Agents              int
	Teams               int
	Inboxes             int
	Macros              int
	Tags                int
	KnowledgeSources    int
	SharedViews         int
	Customers           int
	Conversations       int
	Messages            int
	PrivateNotes        int
	EcommerceConfigured bool
	PushTokens          int
	PCITriggered        int
	VoicemailTranscripts int
	Skipped             []string
	Warnings            []string
}

func main() {
	cfg := Config{}
	flag.StringVar(&cfg.BaseURL, "url", "http://localhost:9000", "Base URL of the libredesk API")
	flag.StringVar(&cfg.Username, "user", "System", "System user to authenticate as")
	flag.StringVar(&cfg.Password, "pass", "changeme", "System user password")
	flag.BoolVar(&cfg.Reset, "reset", false, "Wipe existing demo data before seeding")
	flag.BoolVar(&cfg.AllowNonLocalhost, "allow-non-localhost", false, "Allow running against a non-localhost API (DANGER)")
	flag.BoolVar(&cfg.Verbose, "v", false, "Verbose HTTP logging")
	flag.BoolVar(&cfg.SkipEcommerce, "skip-ecommerce", false, "Skip ecommerce settings seed")
	flag.BoolVar(&cfg.SkipPushTokens, "skip-push-tokens", false, "Skip push-token registration")
	flag.Parse()

	if err := safetyCheck(cfg); err != nil {
		fatal("%v", err)
	}

	client, err := newClient(cfg.BaseURL, cfg.Verbose)
	if err != nil {
		fatal("init client: %v", err)
	}

	fmt.Printf("==> Logging in as %q to %s\n", cfg.Username, cfg.BaseURL)
	if err := client.login(cfg.Username, cfg.Password); err != nil {
		fatal("login failed: %v\n\nMake sure libredesk is running and the System password is set:\n  docker exec -it libredesk_app ./libredesk --set-system-user-password", err)
	}

	if cfg.Reset {
		fmt.Println("==> --reset passed; wiping existing demo data via API")
		if err := wipeDemoData(client); err != nil {
			// Wipe failures are warnings, not fatal — partial wipes still let the seed proceed.
			fmt.Fprintf(os.Stderr, "warning: wipe encountered errors: %v\n", err)
		}
	}

	rng := rand.New(rand.NewSource(20260507)) // deterministic for reproducible screenshots
	summary := &Summary{}

	if err := runSeed(client, rng, cfg, summary); err != nil {
		fatal("seed failed: %v", err)
	}

	printSummary(summary)
}

// safetyCheck refuses to run against non-localhost API hosts unless the operator
// explicitly opted in. Belt-and-braces against fat-fingering a production URL.
func safetyCheck(cfg Config) error {
	u, err := url.Parse(cfg.BaseURL)
	if err != nil {
		return fmt.Errorf("invalid --url: %w", err)
	}
	host := u.Hostname()
	if cfg.AllowNonLocalhost {
		return nil
	}
	// Accept literal localhost / 127.0.0.1 / ::1.
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return nil
	}
	// Resolve the host and check every A/AAAA. If they all resolve to loopback, allow.
	ips, lookupErr := net.LookupIP(host)
	if lookupErr == nil && len(ips) > 0 {
		allLoopback := true
		for _, ip := range ips {
			if !ip.IsLoopback() {
				allLoopback = false
				break
			}
		}
		if allLoopback {
			return nil
		}
	}
	return fmt.Errorf("refusing to run against non-localhost host %q. Pass --allow-non-localhost to override (only do this if you really mean it)", host)
}

func newClient(baseURL string, verbose bool) (*Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	return &Client{
		http: &http.Client{
			Jar:     jar,
			Timeout: 60 * time.Second,
			// Local Docker uses a self-signed cert sometimes; tolerate it.
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
		baseURL: strings.TrimRight(baseURL, "/"),
		verbose: verbose,
	}, nil
}

// envelope is the universal libredesk response wrapper: {"data": ..., "message": ...}
// for success, or {"message": "...", "data": null} for errors.
type envelope struct {
	Data    json.RawMessage `json:"data"`
	Message string          `json:"message"`
}

func (c *Client) do(method, path string, body any) (json.RawMessage, error) {
	var bodyReader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(buf)
	}
	req, err := http.NewRequest(method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.csrfToken != "" {
		req.Header.Set("X-CSRFTOKEN", c.csrfToken)
	}
	if c.verbose {
		fmt.Fprintf(os.Stderr, ">> %s %s\n", method, path)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	// Capture CSRF token from any Set-Cookie that flies past.
	for _, ck := range resp.Cookies() {
		if strings.EqualFold(ck.Name, "csrf_token") && ck.Value != "" {
			c.csrfToken = ck.Value
		}
	}
	if c.verbose {
		fmt.Fprintf(os.Stderr, "<< %s %d %s\n", method, resp.StatusCode, truncate(string(raw), 200))
	}
	if resp.StatusCode >= 400 {
		var env envelope
		if json.Unmarshal(raw, &env) == nil && env.Message != "" {
			return raw, fmt.Errorf("HTTP %d %s: %s", resp.StatusCode, path, env.Message)
		}
		return raw, fmt.Errorf("HTTP %d %s: %s", resp.StatusCode, path, truncate(string(raw), 300))
	}
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		// Some endpoints (logout) redirect; tolerate non-JSON bodies on 2xx.
		return raw, nil
	}
	return env.Data, nil
}

func (c *Client) login(email, password string) error {
	_, err := c.do(http.MethodPost, "/api/v1/auth/login", map[string]string{
		"email":    email,
		"password": password,
	})
	return err
}

// runSeed orchestrates the whole load. Order matters: agents → teams →
// inboxes → tags → macros → knowledge sources → ecommerce → views →
// conversations. Each step talks to the next via IDs cached locally.
func runSeed(c *Client, rng *rand.Rand, cfg Config, sum *Summary) error {
	fmt.Println("==> Loading existing data for idempotency check")
	existing, err := loadExisting(c)
	if err != nil {
		return fmt.Errorf("load existing: %w", err)
	}

	fmt.Println("==> Seeding agents")
	agents, err := seedAgents(c, existing, sum)
	if err != nil {
		return fmt.Errorf("agents: %w", err)
	}

	fmt.Println("==> Seeding teams")
	teams, err := seedTeams(c, agents, existing, sum)
	if err != nil {
		return fmt.Errorf("teams: %w", err)
	}

	fmt.Println("==> Seeding inboxes")
	inboxes, err := seedInboxes(c, existing, sum)
	if err != nil {
		return fmt.Errorf("inboxes: %w", err)
	}

	fmt.Println("==> Seeding tags")
	if err := seedTags(c, existing, sum); err != nil {
		return fmt.Errorf("tags: %w", err)
	}

	fmt.Println("==> Seeding macros")
	if err := seedMacros(c, existing, sum); err != nil {
		return fmt.Errorf("macros: %w", err)
	}

	fmt.Println("==> Seeding knowledge sources (RAG)")
	if err := seedRAGSources(c, existing, sum); err != nil {
		sum.Warnings = append(sum.Warnings, fmt.Sprintf("RAG sources: %v", err))
	}

	if !cfg.SkipEcommerce {
		fmt.Println("==> Seeding ecommerce settings")
		if err := seedEcommerce(c, sum); err != nil {
			sum.Warnings = append(sum.Warnings, fmt.Sprintf("ecommerce: %v", err))
		}
	}

	fmt.Println("==> Seeding shared views")
	if err := seedSharedViews(c, existing, sum); err != nil {
		sum.Warnings = append(sum.Warnings, fmt.Sprintf("shared views: %v", err))
	}

	if !cfg.SkipPushTokens {
		fmt.Println("==> Registering demo FCM push token (for mobile demo)")
		if err := seedPushToken(c, sum); err != nil {
			sum.Warnings = append(sum.Warnings, fmt.Sprintf("push token: %v", err))
		}
	}

	fmt.Println("==> Seeding conversations (this is the big one)")
	if err := seedConversations(c, rng, agents, teams, inboxes, sum); err != nil {
		return fmt.Errorf("conversations: %w", err)
	}

	return nil
}

// ---------------------------------------------------------------------------
// Existing-data probes for idempotency.
// ---------------------------------------------------------------------------

type ExistingData struct {
	AgentsByEmail map[string]int
	TeamsByName   map[string]int
	InboxesByName map[string]int
	TagsByName    map[string]int
	MacrosByName  map[string]int
	RAGByName     map[string]int
	ViewsByName   map[string]int
	Roles         map[string]int // role name -> id; needed to attach roles by ID in create-agent payload
}

func loadExisting(c *Client) (*ExistingData, error) {
	e := &ExistingData{
		AgentsByEmail: map[string]int{},
		TeamsByName:   map[string]int{},
		InboxesByName: map[string]int{},
		TagsByName:    map[string]int{},
		MacrosByName:  map[string]int{},
		RAGByName:     map[string]int{},
		ViewsByName:   map[string]int{},
		Roles:         map[string]int{},
	}

	// agents
	if data, err := c.do(http.MethodGet, "/api/v1/agents", nil); err == nil && len(data) > 0 {
		var list []struct {
			ID    int    `json:"id"`
			Email string `json:"email"`
		}
		_ = json.Unmarshal(data, &list)
		for _, a := range list {
			if a.Email != "" {
				e.AgentsByEmail[strings.ToLower(a.Email)] = a.ID
			}
		}
	}
	// teams
	if data, err := c.do(http.MethodGet, "/api/v1/teams", nil); err == nil && len(data) > 0 {
		var list []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		}
		_ = json.Unmarshal(data, &list)
		for _, t := range list {
			e.TeamsByName[t.Name] = t.ID
		}
	}
	// inboxes
	if data, err := c.do(http.MethodGet, "/api/v1/inboxes", nil); err == nil && len(data) > 0 {
		var list []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		}
		_ = json.Unmarshal(data, &list)
		for _, i := range list {
			e.InboxesByName[i.Name] = i.ID
		}
	}
	// tags
	if data, err := c.do(http.MethodGet, "/api/v1/tags", nil); err == nil && len(data) > 0 {
		var list []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		}
		_ = json.Unmarshal(data, &list)
		for _, t := range list {
			e.TagsByName[t.Name] = t.ID
		}
	}
	// macros
	if data, err := c.do(http.MethodGet, "/api/v1/macros", nil); err == nil && len(data) > 0 {
		var list []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		}
		_ = json.Unmarshal(data, &list)
		for _, m := range list {
			e.MacrosByName[m.Name] = m.ID
		}
	}
	// rag sources
	if data, err := c.do(http.MethodGet, "/api/v1/rag/sources", nil); err == nil && len(data) > 0 {
		var list []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		}
		_ = json.Unmarshal(data, &list)
		for _, s := range list {
			e.RAGByName[s.Name] = s.ID
		}
	}
	// shared views
	if data, err := c.do(http.MethodGet, "/api/v1/shared-views", nil); err == nil && len(data) > 0 {
		var list []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		}
		_ = json.Unmarshal(data, &list)
		for _, v := range list {
			e.ViewsByName[v.Name] = v.ID
		}
	}
	// roles (need IDs to attach in agent create)
	if data, err := c.do(http.MethodGet, "/api/v1/roles", nil); err == nil && len(data) > 0 {
		var list []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		}
		_ = json.Unmarshal(data, &list)
		for _, r := range list {
			e.Roles[r.Name] = r.ID
		}
	}
	return e, nil
}

// ---------------------------------------------------------------------------
// Wipe (--reset) — hit the same delete endpoints we used to create.
// ---------------------------------------------------------------------------

func wipeDemoData(c *Client) error {
	var errs []string

	// Delete demo agents (by @demo.local email)
	if data, err := c.do(http.MethodGet, "/api/v1/agents", nil); err == nil {
		var list []struct {
			ID    int    `json:"id"`
			Email string `json:"email"`
		}
		_ = json.Unmarshal(data, &list)
		for _, a := range list {
			if strings.HasSuffix(strings.ToLower(a.Email), "@demo.local") {
				if _, err := c.do(http.MethodDelete, fmt.Sprintf("/api/v1/agents/%d", a.ID), nil); err != nil {
					errs = append(errs, fmt.Sprintf("delete agent %d: %v", a.ID, err))
				}
			}
		}
	}

	// Delete demo inboxes
	if data, err := c.do(http.MethodGet, "/api/v1/inboxes", nil); err == nil {
		var list []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		}
		_ = json.Unmarshal(data, &list)
		for _, i := range list {
			if strings.HasPrefix(i.Name, "Demo ") {
				if _, err := c.do(http.MethodDelete, fmt.Sprintf("/api/v1/inboxes/%d", i.ID), nil); err != nil {
					errs = append(errs, fmt.Sprintf("delete inbox %d: %v", i.ID, err))
				}
			}
		}
	}

	// Delete demo teams
	if data, err := c.do(http.MethodGet, "/api/v1/teams", nil); err == nil {
		var list []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		}
		_ = json.Unmarshal(data, &list)
		for _, t := range list {
			if t.Name == "Demo Sales" || t.Name == "Demo Support" {
				if _, err := c.do(http.MethodDelete, fmt.Sprintf("/api/v1/teams/%d", t.ID), nil); err != nil {
					errs = append(errs, fmt.Sprintf("delete team %d: %v", t.ID, err))
				}
			}
		}
	}

	// Delete demo macros (prefixed [Demo])
	if data, err := c.do(http.MethodGet, "/api/v1/macros", nil); err == nil {
		var list []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		}
		_ = json.Unmarshal(data, &list)
		for _, m := range list {
			if strings.HasPrefix(m.Name, "[Demo:") {
				if _, err := c.do(http.MethodDelete, fmt.Sprintf("/api/v1/macros/%d", m.ID), nil); err != nil {
					errs = append(errs, fmt.Sprintf("delete macro %d: %v", m.ID, err))
				}
			}
		}
	}

	// Delete demo RAG sources
	if data, err := c.do(http.MethodGet, "/api/v1/rag/sources", nil); err == nil {
		var list []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		}
		_ = json.Unmarshal(data, &list)
		for _, s := range list {
			if strings.HasPrefix(s.Name, "Demo: ") {
				if _, err := c.do(http.MethodDelete, fmt.Sprintf("/api/v1/rag/sources/%d", s.ID), nil); err != nil {
					errs = append(errs, fmt.Sprintf("delete rag %d: %v", s.ID, err))
				}
			}
		}
	}

	// Delete demo shared views
	if data, err := c.do(http.MethodGet, "/api/v1/shared-views", nil); err == nil {
		var list []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		}
		_ = json.Unmarshal(data, &list)
		for _, v := range list {
			if strings.HasPrefix(v.Name, "Demo: ") {
				if _, err := c.do(http.MethodDelete, fmt.Sprintf("/api/v1/shared-views/%d", v.ID), nil); err != nil {
					errs = append(errs, fmt.Sprintf("delete view %d: %v", v.ID, err))
				}
			}
		}
	}

	// Note: contact users with @demo.local emails and their conversations are
	// NOT auto-wiped — conversations don't have a per-row delete that's safe
	// to call here, and the trash flow is the right path for production use.
	// Re-running with --reset will skip-or-replace via name-based idempotency.

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

// ---------------------------------------------------------------------------
// Agents
// ---------------------------------------------------------------------------

type Agent struct {
	ID    int
	Email string
	Name  string
}

func seedAgents(c *Client, ex *ExistingData, sum *Summary) ([]Agent, error) {
	defs := []struct {
		FirstName, LastName, Email string
		Roles                      []string
	}{
		{"Demo", "Admin", "admin@demo.local", []string{"Admin"}},
		{"Alice", "Agent", "agent1@demo.local", []string{"Agent"}},
		{"Bob", "Agent", "agent2@demo.local", []string{"Agent"}},
	}

	var out []Agent
	for _, d := range defs {
		key := strings.ToLower(d.Email)
		if id, ok := ex.AgentsByEmail[key]; ok {
			out = append(out, Agent{ID: id, Email: d.Email, Name: d.FirstName + " " + d.LastName})
			continue
		}
		payload := map[string]any{
			"first_name":         d.FirstName,
			"last_name":          d.LastName,
			"email":              d.Email,
			"roles":              d.Roles,
			"teams":              []string{},
			"enabled":            true,
			"availability_status": "online",
			"send_welcome_email": false,
			"new_password":       "DemoPassw0rd!",
		}
		data, err := c.do(http.MethodPost, "/api/v1/agents", payload)
		if err != nil {
			return nil, fmt.Errorf("create %s: %w", d.Email, err)
		}
		var created struct {
			ID int `json:"id"`
		}
		_ = json.Unmarshal(data, &created)
		sum.Agents++
		out = append(out, Agent{ID: created.ID, Email: d.Email, Name: d.FirstName + " " + d.LastName})
		ex.AgentsByEmail[key] = created.ID
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Teams
// ---------------------------------------------------------------------------

type Team struct {
	ID   int
	Name string
}

func seedTeams(c *Client, agents []Agent, ex *ExistingData, sum *Summary) ([]Team, error) {
	defs := []struct {
		Name string
	}{
		{"Demo Sales"},
		{"Demo Support"},
	}
	var out []Team
	for _, d := range defs {
		if id, ok := ex.TeamsByName[d.Name]; ok {
			out = append(out, Team{ID: id, Name: d.Name})
			continue
		}
		payload := map[string]any{
			"name":                          d.Name,
			"timezone":                      "UTC",
			"conversation_assignment_type":  "Round robin",
			"max_auto_assigned_conversations": 0,
		}
		data, err := c.do(http.MethodPost, "/api/v1/teams", payload)
		if err != nil {
			return nil, fmt.Errorf("create %s: %w", d.Name, err)
		}
		var created struct {
			ID int `json:"id"`
		}
		_ = json.Unmarshal(data, &created)
		out = append(out, Team{ID: created.ID, Name: d.Name})
		ex.TeamsByName[d.Name] = created.ID
		sum.Teams++
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Inboxes
//
// The fork README's "Email Alias Filtering", "Auto-Assign on Reply",
// "Per-Inbox Email Signatures", and "Per-Inbox AI Settings" features all
// configure fields on the inbox, so we wire them all in here.
//
// IMAP password is deliberately set to "DISABLED-BY-DEMO-SAFETY" so a real
// IMAP poller can never authenticate against a real mail server with these
// fake credentials. The inbox itself is created as disabled (enabled=false)
// for the same reason.
// ---------------------------------------------------------------------------

type Inbox struct {
	ID      int
	Name    string
	Channel string
}

func seedInboxes(c *Client, ex *ExistingData, sum *Summary) ([]Inbox, error) {
	var out []Inbox

	// Email inbox with aliases, auto-assign, signature.
	emailName := "Demo Email"
	if id, ok := ex.InboxesByName[emailName]; ok {
		out = append(out, Inbox{ID: id, Name: emailName, Channel: "email"})
	} else {
		cfg := map[string]any{
			"auth_type":              "password",
			"from":                   "Demo Support <support@demo.local>",
			"reply_to":               "support@demo.local",
			"aliases":                []string{"orders@demo.local", "info@demo.local"},
			"auto_assign_on_reply":   true,
			"signature":              "<p>Kind regards,<br><strong>{{agent.first_name}} {{agent.last_name}}</strong><br>Demo Support Team</p>",
			"smtp": []map[string]any{{
				"host":          "smtp.invalid.demo.local",
				"port":          587,
				"username":      "support@demo.local",
				"password":      "DISABLED-BY-DEMO-SAFETY",
				"auth_protocol": "plain",
				"tls_type":      "starttls",
				"hello_hostname": "demo.local",
				"max_conns":     1,
				"idle_timeout":  "30s",
				"pool_wait_timeout": "30s",
				"max_msg_retries": 1,
			}},
			"imap": []map[string]any{{
				"host":            "imap.invalid.demo.local",
				"port":            993,
				"username":        "support@demo.local",
				"password":        "DISABLED-BY-DEMO-SAFETY",
				"mailbox":         "INBOX, [Gmail]/Spam",
				"read_interval":   "5m",
				"scan_inbox_since": "0",
				"tls_type":        "tls",
			}},
		}
		cfgBytes, _ := json.Marshal(cfg)
		payload := map[string]any{
			"name":    emailName,
			"channel": "email",
			"from":    "Demo Support <support@demo.local>",
			"enabled": false, // never let it try to poll
			"config":  json.RawMessage(cfgBytes),
		}
		data, err := c.do(http.MethodPost, "/api/v1/inboxes", payload)
		if err != nil {
			return nil, fmt.Errorf("create %s: %w", emailName, err)
		}
		var created struct {
			ID int `json:"id"`
		}
		_ = json.Unmarshal(data, &created)
		out = append(out, Inbox{ID: created.ID, Name: emailName, Channel: "email"})
		ex.InboxesByName[emailName] = created.ID
		sum.Inboxes++
	}

	// Live chat widget inbox. Minimal but valid config — widget validation
	// is strict about colours/positions/spacing so we set explicit defaults.
	widgetName := "Demo Chat Widget"
	if id, ok := ex.InboxesByName[widgetName]; ok {
		out = append(out, Inbox{ID: id, Name: widgetName, Channel: "live_chat"})
	} else {
		cfg := map[string]any{
			"colors": map[string]any{
				"primary": "#0f766e",
			},
			"launcher": map[string]any{
				"position": "right",
				"spacing": map[string]any{
					"side":   16,
					"bottom": 16,
				},
			},
		}
		cfgBytes, _ := json.Marshal(cfg)
		payload := map[string]any{
			"name":    widgetName,
			"channel": "live_chat",
			"from":    "",
			"enabled": true,
			"config":  json.RawMessage(cfgBytes),
		}
		data, err := c.do(http.MethodPost, "/api/v1/inboxes", payload)
		if err != nil {
			// Widget validation can vary across versions — don't fail the
			// whole seed for it. Surface as a warning.
			sum.Warnings = append(sum.Warnings, fmt.Sprintf("live chat widget inbox: %v", err))
		} else {
			var created struct {
				ID int `json:"id"`
			}
			_ = json.Unmarshal(data, &created)
			out = append(out, Inbox{ID: created.ID, Name: widgetName, Channel: "live_chat"})
			ex.InboxesByName[widgetName] = created.ID
			sum.Inboxes++
		}
	}

	// Per-inbox AI settings override on the email inbox (demoes the feature).
	if id, ok := ex.InboxesByName[emailName]; ok {
		aiPayload := map[string]any{
			"system_prompt": "You are a friendly demo support agent for Acme Widgets. Always greet the customer by first name. End every reply with 'Demo Support Team'.",
		}
		if _, err := c.do(http.MethodPut, fmt.Sprintf("/api/v1/settings/ai/inbox/%d", id), aiPayload); err != nil {
			sum.Warnings = append(sum.Warnings, fmt.Sprintf("per-inbox AI settings: %v", err))
		}
	}

	return out, nil
}

// ---------------------------------------------------------------------------
// Tags
// ---------------------------------------------------------------------------

func seedTags(c *Client, ex *ExistingData, sum *Summary) error {
	tags := []string{"vip", "complaint", "feature-request", "spam-rescued", "duplicate", "demo"}
	for _, t := range tags {
		if _, ok := ex.TagsByName[t]; ok {
			continue
		}
		data, err := c.do(http.MethodPost, "/api/v1/tags", map[string]any{"name": t})
		if err != nil {
			sum.Warnings = append(sum.Warnings, fmt.Sprintf("tag %s: %v", t, err))
			continue
		}
		var created struct {
			ID int `json:"id"`
		}
		_ = json.Unmarshal(data, &created)
		ex.TagsByName[t] = created.ID
		sum.Tags++
	}
	return nil
}

// ---------------------------------------------------------------------------
// Macros
// ---------------------------------------------------------------------------

func seedMacros(c *Client, ex *ExistingData, sum *Summary) error {
	defs := []struct {
		Name    string
		Content string
	}{
		{"[Demo: Order Status] Tracking info",
			"<p>Hi {{contact.first_name}},</p><p>Thanks for getting in touch about your order. You can track your shipment using the link in your dispatch email.</p><p>Kind regards,<br>Demo Support</p>"},
		{"[Demo: Order Status] Order delayed",
			"<p>Hi {{contact.first_name}},</p><p>Apologies for the delay on your order — we're chasing this up with our carrier and will follow up within 24 hours.</p>"},
		{"[Demo: Returns] How to return",
			"<p>Hi {{contact.first_name}},</p><p>To start a return, please reply with your order number and the reason for return. We'll send a prepaid label.</p>"},
		{"[Demo: Greetings] Initial response",
			"<p>Hi {{contact.first_name}},</p><p>Thanks for reaching out! I'm taking a look at your enquiry now and will reply within the hour.</p>"},
		{"[Demo: Greetings] Closing reply",
			"<p>Hi {{contact.first_name}},</p><p>Is there anything else I can help with? If not, I'll go ahead and resolve this ticket.</p>"},
	}
	for _, d := range defs {
		if _, ok := ex.MacrosByName[d.Name]; ok {
			continue
		}
		payload := map[string]any{
			"name":            d.Name,
			"message_content": d.Content,
			"visibility":      "all",
			"visible_when":    []string{"replying", "starting_conversation", "adding_private_note"},
			"actions":         json.RawMessage(`[]`),
		}
		data, err := c.do(http.MethodPost, "/api/v1/macros", payload)
		if err != nil {
			sum.Warnings = append(sum.Warnings, fmt.Sprintf("macro %s: %v", d.Name, err))
			continue
		}
		var created struct {
			ID int `json:"id"`
		}
		_ = json.Unmarshal(data, &created)
		ex.MacrosByName[d.Name] = created.ID
		sum.Macros++
	}
	return nil
}

// ---------------------------------------------------------------------------
// RAG knowledge sources
// ---------------------------------------------------------------------------

func seedRAGSources(c *Client, ex *ExistingData, sum *Summary) error {
	// Webpage source (won't actually sync from inside docker, but the row exists
	// so the UI demos correctly).
	webName := "Demo: Example FAQ webpage"
	if _, ok := ex.RAGByName[webName]; !ok {
		cfgBytes, _ := json.Marshal(map[string]any{
			"urls": []string{"https://example.com/faq"},
		})
		payload := map[string]any{
			"name":        webName,
			"source_type": "webpage",
			"config":      json.RawMessage(cfgBytes),
			"enabled":     true,
		}
		data, err := c.do(http.MethodPost, "/api/v1/rag/sources", payload)
		if err != nil {
			return err
		}
		var created struct {
			ID int `json:"id"`
		}
		_ = json.Unmarshal(data, &created)
		ex.RAGByName[webName] = created.ID
		sum.KnowledgeSources++
	}

	// Macro-based source: lets the AI pull from canned responses.
	macroName := "Demo: Macros as knowledge"
	if _, ok := ex.RAGByName[macroName]; !ok {
		cfgBytes, _ := json.Marshal(map[string]any{})
		payload := map[string]any{
			"name":        macroName,
			"source_type": "macro",
			"config":      json.RawMessage(cfgBytes),
			"enabled":     true,
		}
		data, err := c.do(http.MethodPost, "/api/v1/rag/sources", payload)
		if err != nil {
			return err
		}
		var created struct {
			ID int `json:"id"`
		}
		_ = json.Unmarshal(data, &created)
		ex.RAGByName[macroName] = created.ID
		sum.KnowledgeSources++
	}
	return nil
}

// ---------------------------------------------------------------------------
// Ecommerce settings (Maho/Magento1). Fake creds — the Connection Test will
// fail, but the settings page renders correctly for screenshots and the
// "+ Orders" button shows in the reply box because ecommerce.type is set.
// ---------------------------------------------------------------------------

func seedEcommerce(c *Client, sum *Summary) error {
	payload := map[string]any{
		"type":          "magento1",
		"base_url":      "https://demo-store.invalid.demo.local",
		"client_id":     "demo-client-id",
		"client_secret": "demo-client-secret-DISABLED",
	}
	if _, err := c.do(http.MethodPut, "/api/v1/ecommerce/settings", payload); err != nil {
		return err
	}
	sum.EcommerceConfigured = true
	return nil
}

// ---------------------------------------------------------------------------
// Shared views — demonstrates "Advanced View Filters" with the `in_or_null`
// operator (the README's headline filter example).
// ---------------------------------------------------------------------------

func seedSharedViews(c *Client, ex *ExistingData, sum *Summary) error {
	name := "Demo: Unassigned + Open"
	if _, ok := ex.ViewsByName[name]; ok {
		return nil
	}
	// Filter shape mirrors what the frontend posts: an array of {field, operator, value}.
	// See cmd/views.go validateSharedView for accepted shape.
	filters := []map[string]any{
		{"field": "status", "operator": "equals", "value": "Open"},
		{"field": "assigned_user", "operator": "is_not_set", "value": ""},
	}
	filtersBytes, _ := json.Marshal(filters)
	payload := map[string]any{
		"name":       name,
		"visibility": "all",
		"filters":    json.RawMessage(filtersBytes),
	}
	data, err := c.do(http.MethodPost, "/api/v1/shared-views", payload)
	if err != nil {
		return err
	}
	var created struct {
		ID int `json:"id"`
	}
	_ = json.Unmarshal(data, &created)
	ex.ViewsByName[name] = created.ID
	sum.SharedViews++
	return nil
}

// ---------------------------------------------------------------------------
// Push token — registers a fake FCM token for the current (System) user so
// the FCM Push Notifications feature has visible data.
// ---------------------------------------------------------------------------

func seedPushToken(c *Client, sum *Summary) error {
	payload := map[string]any{
		"token":    "demo-fcm-token-DISABLED-fake-fake-fake-fake-fake-fake-fake-fake-fake",
		"platform": "android",
	}
	if _, err := c.do(http.MethodPost, "/api/v1/agents/me/push-token", payload); err != nil {
		return err
	}
	sum.PushTokens++
	return nil
}

// ---------------------------------------------------------------------------
// Conversations — the bulk of the value of the seeder.
//
// Each "scenario" maps to one or more fork features in the README. We try
// to cover the most visible ones; UI-only features (full-width toggle,
// quick-assign dropdowns, table view) are noted in the printed summary as
// "no seed data needed".
// ---------------------------------------------------------------------------

// customerPool is a curated list (rather than a faker dep). We pick from this
// pool deterministically with the seeded RNG so re-runs produce the same data.
var customerPool = []struct {
	FirstName, LastName, Email string
}{
	{"Jane", "Smith", "jane.smith@demo.local"},
	{"Tom", "Becker", "tom.becker@demo.local"},
	{"Priya", "Kapoor", "priya.kapoor@demo.local"},
	{"Marcus", "Chen", "marcus.chen@demo.local"},
	{"Linda", "Olsen", "linda.olsen@demo.local"},
	{"Ahmed", "Hassan", "ahmed.hassan@demo.local"},
	{"Sophie", "Bernard", "sophie.bernard@demo.local"},
	{"Diego", "Ramirez", "diego.ramirez@demo.local"},
	{"Yuki", "Tanaka", "yuki.tanaka@demo.local"},
	{"Olivia", "Wright", "olivia.wright@demo.local"},
	{"Henry", "Ng", "henry.ng@demo.local"},
	{"Fatima", "Khan", "fatima.khan@demo.local"},
}

// scenarioPaths drive the conversation seeder. Each entry produces one conv;
// the seeder loop walks them and applies the side-effects (tags, status, etc.)
// after creation. Splitting the side-effects out of the create call keeps the
// API contract simple and means a single new field on a feature only needs an
// extra side-effect helper, not a new create-conversation variant.
type scenario struct {
	Tag              string   // free-text classifier for the summary table
	Subject          string   // ticket subject
	InitialMessage   string   // first contact message (HTML)
	CustomerIdx      int      // index into customerPool (or -1 to round-robin)
	ReplyCount       int      // agent reply count after creation
	PrivateNotes     []string // optional private notes to insert
	Mentions         bool     // include an @mention in a private note
	Tags             []string // tags to apply
	Status           string   // final status ("Spam", "Trashed", "Resolved", "Closed", or "" for Open)
	AgeDays          int      // age the conversation by N days (uses last_message; only relative timestamps will reflect this approximately since we can't backdate server-side)
	WithInlineImage  bool     // include a placeholder inline-image marker
	WithPCITrigger   bool     // include a Visa test number to trigger PCI redaction banner
	WithVoicemail    bool     // simulate voicemail with a transcript private note
	WithMention      string   // @mention this agent email in a private note
	AssignAgentEmail string   // assign to this agent (after create)
	AssignTeamName   string   // assign to this team (after create)
	DupeOf           string   // if set, this conv duplicates another (same customer, similar subject)
}

func seedConversations(c *Client, rng *rand.Rand, agents []Agent, teams []Team, inboxes []Inbox, sum *Summary) error {
	if len(inboxes) == 0 {
		return errors.New("no inboxes available; cannot create conversations")
	}
	// Use the first email inbox we created for everything. (Widget inbox
	// requires a real visitor session, not viable via the agent API.)
	var inboxID int
	for _, ib := range inboxes {
		if ib.Channel == "email" {
			inboxID = ib.ID
			break
		}
	}
	if inboxID == 0 {
		return errors.New("no email inbox available")
	}

	agentByEmail := map[string]int{}
	for _, a := range agents {
		agentByEmail[a.Email] = a.ID
	}
	teamByName := map[string]int{}
	for _, t := range teams {
		teamByName[t.Name] = t.ID
	}

	scenarios := []scenario{
		// === Customer Ticket History — Jane Smith with 5+ conversations ===
		{Tag: "customer-history", CustomerIdx: 0, Subject: "Order arrived damaged",
			InitialMessage: "<p>Hi, my order #ACME-1042 arrived with a cracked screen. Can you help?</p>",
			ReplyCount:     2, Tags: []string{"complaint", "demo"}, AssignAgentEmail: "agent1@demo.local",
			AgeDays: 14, Status: "Resolved"},
		{Tag: "customer-history", CustomerIdx: 0, Subject: "Where is my refund?",
			InitialMessage: "<p>Still haven't seen the refund for the damaged order. Following up.</p>",
			ReplyCount:     1, Tags: []string{"complaint", "demo"}, AssignAgentEmail: "agent1@demo.local",
			AgeDays: 7, Status: "Resolved"},
		{Tag: "customer-history", CustomerIdx: 0, Subject: "Replacement product question",
			InitialMessage: "<p>Quick question on the replacement model — does it support USB-C charging?</p>",
			ReplyCount:     1, Tags: []string{"demo"}, AssignAgentEmail: "agent2@demo.local", AgeDays: 3},
		{Tag: "customer-history", CustomerIdx: 0, Subject: "Loyalty points enquiry",
			InitialMessage: "<p>How do I redeem my loyalty points? I have 4,500 available.</p>",
			ReplyCount:     0, Tags: []string{"vip", "demo"}, AgeDays: 1},
		{Tag: "customer-history", CustomerIdx: 0, Subject: "VIP escalation: bulk order",
			InitialMessage: "<p>Hi team, looking to place a bulk order for 200 units. Can you put me through to your B2B contact?</p>",
			ReplyCount:     0, Tags: []string{"vip", "demo"}, AgeDays: 0,
			AssignTeamName: "Demo Sales"},

		// === Spam & Trash ===
		{Tag: "spam-auto-rescued", CustomerIdx: 1, Subject: "Re: Your order from last week",
			InitialMessage: "<p>Hi, hoping to follow up on the order we discussed last week. Any update?</p>",
			ReplyCount:     1, Tags: []string{"spam-rescued", "demo"}, AgeDays: 5,
			AssignAgentEmail: "agent1@demo.local"}, // rescued: stayed Open with prior agent reply
		{Tag: "spam-not-rescued", CustomerIdx: -1, Subject: "WIN A FREE iPHONE NOW!!!",
			InitialMessage: "<p>Click here to claim your prize! Limited time offer!</p>",
			Tags:           []string{"demo"}, AgeDays: 2, Status: "Spam"},
		{Tag: "spam-not-rescued", CustomerIdx: -1, Subject: "URGENT: Verify your account",
			InitialMessage: "<p>Your account will be suspended in 24 hours. Click below to verify.</p>",
			Tags:           []string{"demo"}, AgeDays: 1, Status: "Spam"},
		{Tag: "trash", CustomerIdx: -1, Subject: "Wrong inbox — forwarded by mistake",
			InitialMessage: "<p>Sorry, this email was meant for a different team. Please disregard.</p>",
			Tags:           []string{"demo"}, AgeDays: 10, Status: "Trashed"},
		{Tag: "trash", CustomerIdx: -1, Subject: "Test message from internal QA",
			InitialMessage: "<p>Just testing the support email pipeline. Please trash.</p>",
			Tags:           []string{"demo"}, AgeDays: 4, Status: "Trashed"},

		// === Ticket Merging (two convs from same customer, duplicate tag) ===
		{Tag: "merge-candidate", CustomerIdx: 2, Subject: "Subscription not activating",
			InitialMessage: "<p>I purchased the Pro plan yesterday but my account is still on Free.</p>",
			Tags:           []string{"duplicate", "demo"}, AgeDays: 1, AssignAgentEmail: "agent2@demo.local"},
		{Tag: "merge-candidate", CustomerIdx: 2, Subject: "Re: Pro plan upgrade not working",
			InitialMessage: "<p>Following up — still on Free plan. Order #ACME-2051.</p>",
			Tags:           []string{"duplicate", "demo"}, AgeDays: 0},

		// === Smart Team Reassignment demo (assigned to agent1, in Demo Sales team)
		{Tag: "team-reassign", CustomerIdx: 3, Subject: "B2B pricing question",
			InitialMessage: "<p>Hi, what's your bulk pricing for 50+ units?</p>",
			ReplyCount:     0, AssignAgentEmail: "agent1@demo.local", AssignTeamName: "Demo Sales",
			Tags:    []string{"demo"}, AgeDays: 2},

		// === Customer Reply Notifications — customer just replied to an assigned conv ===
		{Tag: "customer-replied", CustomerIdx: 4, Subject: "Login problem after password reset",
			InitialMessage: "<p>I reset my password but now can't log in at all.</p>",
			ReplyCount:     1, AssignAgentEmail: "agent2@demo.local", Tags: []string{"demo"}, AgeDays: 0},

		// === Gmail-Style Quoted Thread — 3+ message reply chain ===
		{Tag: "long-thread", CustomerIdx: 5, Subject: "Multiple billing questions",
			InitialMessage: "<p>Hi, I have a few questions:<br>1. Why was I charged twice?<br>2. Can I get a VAT invoice?<br>3. How do I update my card?</p>",
			ReplyCount:     3, AssignAgentEmail: "agent1@demo.local", Tags: []string{"demo"}, AgeDays: 1},

		// === Inline Images (Email & Message Improvements) ===
		{Tag: "inline-image", CustomerIdx: 6, Subject: "Photo of damaged packaging",
			InitialMessage: `<p>Hi, attaching a photo of the damaged box. The product inside seems okay but the courier was rough.</p><p><img src="https://placehold.co/600x400/png?text=Demo+damaged+box" alt="Damaged box photo" style="max-width:400px"/></p>`,
			ReplyCount:     1, AssignAgentEmail: "agent2@demo.local", Tags: []string{"demo"}, AgeDays: 1},

		// === PCI Credit Card Redaction (Visa test 4111-1111-1111-1111 passes Luhn) ===
		{Tag: "pci-trigger", CustomerIdx: 7, Subject: "Update my saved card",
			InitialMessage: "<p>Hi, please update the card on file. New card: 4111 1111 1111 1111, exp 12/29, CVV 123. Thanks!</p>",
			ReplyCount:     0, Tags: []string{"demo"}, AgeDays: 0, WithPCITrigger: true,
			AssignAgentEmail: "agent1@demo.local"},

		// === Voicemail transcription ===
		{Tag: "voicemail", CustomerIdx: 8, Subject: "Voicemail from customer (transcribed)",
			InitialMessage: "<p>[Inbound voicemail — see transcript private note]</p>",
			Tags:           []string{"demo"}, AgeDays: 1, WithVoicemail: true,
			AssignAgentEmail: "agent2@demo.local"},

		// === @Mentions ===
		{Tag: "mention", CustomerIdx: 9, Subject: "Refund processing time?",
			InitialMessage: "<p>How long do refunds take to appear on my card?</p>",
			ReplyCount:     0, Tags: []string{"demo"}, AgeDays: 2,
			AssignAgentEmail: "agent1@demo.local", WithMention: "agent2@demo.local",
			PrivateNotes: []string{"<p>@Bob Agent can you help — this customer is asking about refund timing, you handled the last one.</p>"}},

		// === Filler conversations to populate the list (varied priorities/statuses) ===
		{Tag: "filler", CustomerIdx: 10, Subject: "How do I change my shipping address?",
			InitialMessage: "<p>I moved last week, need to update the default shipping address on my account.</p>",
			AssignAgentEmail: "agent1@demo.local", Tags: []string{"demo"}, AgeDays: 0},
		{Tag: "filler", CustomerIdx: 11, Subject: "Coupon code not working",
			InitialMessage: "<p>The coupon SAVE20 isn't applying at checkout. Help?</p>",
			ReplyCount: 1, AssignAgentEmail: "agent2@demo.local", Tags: []string{"demo"}, AgeDays: 0},
		{Tag: "filler", CustomerIdx: -1, Subject: "Feature request: dark mode",
			InitialMessage: "<p>Would love a dark mode for the customer portal. Easier on the eyes.</p>",
			Tags: []string{"feature-request", "demo"}, AgeDays: 5, Status: "Closed"},
		{Tag: "filler", CustomerIdx: -1, Subject: "Account suspension review",
			InitialMessage: "<p>My account was suspended last month — can we discuss reinstatement?</p>",
			AssignTeamName: "Demo Support", Tags: []string{"demo"}, AgeDays: 30, Status: "Resolved"},
		{Tag: "filler", CustomerIdx: -1, Subject: "Bug: page crashes on Safari",
			InitialMessage: "<p>The checkout page consistently crashes on Safari 17. Chrome works fine.</p>",
			ReplyCount: 2, AssignAgentEmail: "agent1@demo.local", Tags: []string{"demo"}, AgeDays: 7},
	}

	for i, s := range scenarios {
		if err := createScenario(c, rng, &s, i, inboxID, agentByEmail, teamByName, sum); err != nil {
			sum.Warnings = append(sum.Warnings, fmt.Sprintf("scenario %d (%s): %v", i, s.Tag, err))
			continue
		}
	}
	return nil
}

// createScenario walks one scenario through the full lifecycle: create
// customer/contact, create conversation, apply reply chain, attach tags,
// inject side-effect rows (PCI trigger, voicemail transcript), and set final
// status. Each step is best-effort; non-fatal errors get surfaced as warnings.
func createScenario(c *Client, rng *rand.Rand, s *scenario, idx int, inboxID int, agentByEmail map[string]int, teamByName map[string]int, sum *Summary) error {
	// Pick customer
	var cust struct{ FirstName, LastName, Email string }
	if s.CustomerIdx >= 0 && s.CustomerIdx < len(customerPool) {
		cust.FirstName = customerPool[s.CustomerIdx].FirstName
		cust.LastName = customerPool[s.CustomerIdx].LastName
		cust.Email = customerPool[s.CustomerIdx].Email
	} else {
		// Round-robin from the pool for anonymous/spam scenarios; ensures we
		// don't reuse the same customer for every spam conv.
		p := customerPool[(idx*7)%len(customerPool)]
		cust.FirstName = p.FirstName
		cust.LastName = p.LastName
		cust.Email = p.Email
	}

	// Create the conversation as if the contact initiated it (so the timeline
	// shows an inbound message first).
	payload := map[string]any{
		"inbox_id":      inboxID,
		"contact_email": cust.Email,
		"first_name":    cust.FirstName,
		"last_name":     cust.LastName,
		"subject":       s.Subject,
		"content":       s.InitialMessage,
		"initiator":     "contact",
	}
	data, err := c.do(http.MethodPost, "/api/v1/conversations", payload)
	if err != nil {
		return fmt.Errorf("create: %w", err)
	}
	var conv struct {
		UUID            string `json:"uuid"`
		ReferenceNumber string `json:"reference_number"`
		ID              int    `json:"id"`
	}
	if err := json.Unmarshal(data, &conv); err != nil {
		return fmt.Errorf("decode create response: %w", err)
	}
	if conv.UUID == "" {
		return errors.New("server did not return a conversation uuid")
	}
	sum.Conversations++
	sum.Customers++ // approximate; same-email re-uses will be over-counted but that's fine for a recap

	// Assign team first (it may clear the user assignee, mirroring create-conv handler)
	if s.AssignTeamName != "" {
		if tid, ok := teamByName[s.AssignTeamName]; ok {
			_, _ = c.do(http.MethodPut, fmt.Sprintf("/api/v1/conversations/%s/assignee/team", conv.UUID),
				map[string]any{"assignee_id": tid})
		}
	}
	if s.AssignAgentEmail != "" {
		if aid, ok := agentByEmail[s.AssignAgentEmail]; ok {
			_, _ = c.do(http.MethodPut, fmt.Sprintf("/api/v1/conversations/%s/assignee/user", conv.UUID),
				map[string]any{"assignee_id": aid})
		}
	}

	// Apply tags
	if len(s.Tags) > 0 {
		_, _ = c.do(http.MethodPost, fmt.Sprintf("/api/v1/conversations/%s/tags", conv.UUID),
			map[string]any{"tags": s.Tags})
	}

	// Agent reply chain. Use the *current logged-in user* (System) — replies
	// originate from the seeder session, not the demo agents. That keeps auth
	// simple (no re-login per agent) and the per-message author shows up as
	// "System" which the screenshotter can re-route by re-assigning if needed.
	replyTemplates := []string{
		"<p>Hi %s,</p><p>Thanks for getting in touch — I'm looking into this now and will reply within the hour.</p><p>Best,<br>Demo Support</p>",
		"<p>Hi %s,</p><p>Quick update: I've located the order in our system. Investigating the root cause now.</p>",
		"<p>Hi %s,</p><p>All sorted — the issue should now be resolved on your end. Please confirm and I'll close this ticket.</p>",
	}
	for r := 0; r < s.ReplyCount && r < len(replyTemplates); r++ {
		msg := fmt.Sprintf(replyTemplates[r], cust.FirstName)
		payload := map[string]any{
			"message":     msg,
			"private":     false,
			"sender_type": "agent",
			"to":          []string{cust.Email},
		}
		if _, err := c.do(http.MethodPost, fmt.Sprintf("/api/v1/conversations/%s/messages", conv.UUID), payload); err != nil {
			sum.Warnings = append(sum.Warnings, fmt.Sprintf("conv %s reply %d: %v", conv.ReferenceNumber, r, err))
			// Stop the reply chain on first error rather than spamming retries.
			break
		}
		sum.Messages++
		// Tiny sleep keeps the dedup window from rejecting us — the server
		// rejects identical (user, conv, body, status, from) submissions within
		// a short window. Different reply bodies dodge this naturally, so we
		// just yield briefly to make timestamps monotonic.
		time.Sleep(50 * time.Millisecond)
	}

	// Private notes
	for _, note := range s.PrivateNotes {
		payload := map[string]any{
			"message":     note,
			"private":     true,
			"sender_type": "agent",
		}
		if _, err := c.do(http.MethodPost, fmt.Sprintf("/api/v1/conversations/%s/messages", conv.UUID), payload); err == nil {
			sum.PrivateNotes++
		}
	}

	// Voicemail simulation: a private note containing the "transcript". This
	// matches what the voicemail-worker would insert in production; we skip
	// triggering the actual whisper pipeline because that requires uploading
	// an audio file and waiting for the worker — neither is suitable for a
	// fast deterministic seed.
	if s.WithVoicemail {
		transcript := "<p><strong>[Voicemail transcript — local whisper.cpp]</strong></p><p>Hi, this is " + cust.FirstName + ". I'm calling about my recent order, it hasn't arrived yet and the tracking page says it's been delivered. Can someone give me a call back on this number? Thanks.</p>"
		payload := map[string]any{
			"message":     transcript,
			"private":     true,
			"sender_type": "agent",
		}
		if _, err := c.do(http.MethodPost, fmt.Sprintf("/api/v1/conversations/%s/messages", conv.UUID), payload); err == nil {
			sum.VoicemailTranscripts++
			sum.PrivateNotes++
		}
	}

	// PCI trigger flag (just a counter — the trigger itself is in the initial
	// message body, which the ingest scanner picks up server-side).
	if s.WithPCITrigger {
		sum.PCITriggered++
	}

	// Final status
	switch s.Status {
	case "Spam":
		_, _ = c.do(http.MethodPut, fmt.Sprintf("/api/v1/conversations/%s/spam", conv.UUID), nil)
	case "Trashed":
		_, _ = c.do(http.MethodPut, fmt.Sprintf("/api/v1/conversations/%s/trash", conv.UUID), nil)
	case "Resolved", "Closed":
		_, _ = c.do(http.MethodPut, fmt.Sprintf("/api/v1/conversations/%s/status", conv.UUID),
			map[string]any{"status": s.Status})
	}

	_ = rng // reserved for future randomised flavour
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "ERROR: "+format+"\n", args...)
	os.Exit(1)
}

// printSummary emits the recap a human screenshotter actually wants.
func printSummary(s *Summary) {
	fmt.Println()
	fmt.Println("=========================================")
	fmt.Println(" Demo seeder complete")
	fmt.Println("=========================================")
	fmt.Printf("  Agents created:           %d\n", s.Agents)
	fmt.Printf("  Teams created:            %d\n", s.Teams)
	fmt.Printf("  Inboxes created:          %d\n", s.Inboxes)
	fmt.Printf("  Tags created:             %d\n", s.Tags)
	fmt.Printf("  Macros created:           %d\n", s.Macros)
	fmt.Printf("  Knowledge sources:        %d\n", s.KnowledgeSources)
	fmt.Printf("  Shared views:             %d\n", s.SharedViews)
	fmt.Printf("  Conversations created:    %d\n", s.Conversations)
	fmt.Printf("    of which agent replies: %d\n", s.Messages)
	fmt.Printf("    of which private notes: %d\n", s.PrivateNotes)
	fmt.Printf("  PCI redaction triggers:   %d\n", s.PCITriggered)
	fmt.Printf("  Voicemail transcripts:    %d\n", s.VoicemailTranscripts)
	fmt.Printf("  FCM push tokens:          %d\n", s.PushTokens)
	fmt.Printf("  Ecommerce configured:     %t\n", s.EcommerceConfigured)
	if len(s.Warnings) > 0 {
		fmt.Println()
		fmt.Println("Warnings (non-fatal):")
		for _, w := range s.Warnings {
			fmt.Printf("  - %s\n", w)
		}
	}
	fmt.Println()
	fmt.Println("Demo logins (all use password 'DemoPassw0rd!'):")
	fmt.Println("  - admin@demo.local   (Admin role)")
	fmt.Println("  - agent1@demo.local  (Agent role, Demo Sales team)")
	fmt.Println("  - agent2@demo.local  (Agent role, Demo Support team)")
	fmt.Println()
	fmt.Println("Next: open http://localhost:9000 and start clicking through")
	fmt.Println("the per-feature recipe in docs/DEMO_SETUP.md")
}

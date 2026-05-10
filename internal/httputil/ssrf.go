package httputil

import (
	"net"
	"net/http"
	"net/netip"
	"time"

	"github.com/abhinavxd/ssrfguard"
	"github.com/zerodha/logf"
)

// SSRFGuardedClientOpts configures NewSSRFGuardedClient. Zero values for
// any timeout fall back to the constants below; callers should set the
// fields that matter for their workload (e.g. webhook delivery uses
// tighter handshake budgets than OIDC discovery).
type SSRFGuardedClientOpts struct {
	// AllowedHosts are CIDR prefixes permitted to bypass the SSRF deny
	// list (loopback / RFC1918 / link-local / IPv6-reserved). Invalid
	// entries are logged via Lo and skipped.
	AllowedHosts []string

	// Lo logs invalid CIDR entries. Required.
	Lo *logf.Logger

	// HostsConfigName is the koanf-style name of the allowlist setting
	// surfaced in the warning when an entry fails to parse (e.g.
	// "webhook" → "ignoring invalid webhook `allowed_hosts` entry").
	// Empty falls back to "ssrf".
	HostsConfigName string

	// Timeout is the overall http.Client.Timeout. Defaults to 15s.
	Timeout time.Duration
	// DialTimeout is the per-dial budget. Defaults to 5s.
	DialTimeout time.Duration
	// TLSHandshakeTimeout. Defaults to 5s.
	TLSHandshakeTimeout time.Duration
	// ResponseHeaderTimeout. Defaults to 10s.
	ResponseHeaderTimeout time.Duration
	// KeepAlive on the dialer. Defaults to 30s.
	KeepAlive time.Duration
}

const (
	defaultSSRFTimeout               = 15 * time.Second
	defaultSSRFDialTimeout           = 5 * time.Second
	defaultSSRFTLSHandshakeTimeout   = 5 * time.Second
	defaultSSRFResponseHeaderTimeout = 10 * time.Second
	defaultSSRFKeepAlive             = 30 * time.Second
)

// NewSSRFGuardedClient builds an *http.Client whose dialer rejects
// connections to private/reserved IP ranges. The check fires after DNS
// resolution but before the TCP handshake, so it also defends against
// DNS-rebinding attacks.
//
// Single source of truth for the SSRF-guarded client pattern used by
// auth/oidc, webhooks and external search; previously this dialer +
// guard wiring was duplicated three times with subtly different timeout
// budgets — those budgets are now per-callsite via opts.
func NewSSRFGuardedClient(opts SSRFGuardedClientOpts) *http.Client {
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = defaultSSRFTimeout
	}
	dialTimeout := opts.DialTimeout
	if dialTimeout <= 0 {
		dialTimeout = defaultSSRFDialTimeout
	}
	tlsTimeout := opts.TLSHandshakeTimeout
	if tlsTimeout <= 0 {
		tlsTimeout = defaultSSRFTLSHandshakeTimeout
	}
	respHeaderTimeout := opts.ResponseHeaderTimeout
	if respHeaderTimeout <= 0 {
		respHeaderTimeout = defaultSSRFResponseHeaderTimeout
	}
	keepAlive := opts.KeepAlive
	if keepAlive <= 0 {
		keepAlive = defaultSSRFKeepAlive
	}

	prefixes := ParseAllowedCIDRs(opts.AllowedHosts, opts.HostsConfigName, opts.Lo)
	guard := ssrfguard.New(prefixes...)

	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   dialTimeout,
				KeepAlive: keepAlive,
				Control:   guard.Control,
			}).DialContext,
			TLSHandshakeTimeout:   tlsTimeout,
			ResponseHeaderTimeout: respHeaderTimeout,
		},
	}
}

// ParseAllowedCIDRs parses CIDR strings into netip.Prefix slices,
// logging and skipping any malformed entries. configName is interpolated
// into the warning ("ignoring invalid <name> `allowed_hosts` entry") so
// the operator can identify which setting holds the bad value.
func ParseAllowedCIDRs(hosts []string, configName string, lo *logf.Logger) []netip.Prefix {
	if configName == "" {
		configName = "ssrf"
	}
	var prefixes []netip.Prefix
	for _, h := range hosts {
		prefix, err := netip.ParsePrefix(h)
		if err != nil {
			if lo != nil {
				lo.Warn("ignoring invalid `allowed_hosts` entry", "config", configName, "entry", h, "error", err)
			}
			continue
		}
		prefixes = append(prefixes, prefix)
	}
	return prefixes
}

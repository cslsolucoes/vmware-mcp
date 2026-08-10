// Package vmware wraps github.com/vmware/govmomi with the connection,
// keepalive, and inventory-lookup conventions this MCP server's tools rely
// on. See referencia/govmomi/ (this repo) for the vendored source of that
// dependency — read-only reference, resolved for builds via go.mod/the Go
// module cache, never imported from the local clone directly.
package vmware

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/session/keepalive"
	"github.com/vmware/govmomi/vapi/rest"
)

// Config holds the connection parameters for a vCenter/ESXi endpoint.
type Config struct {
	// URL is a bare host ("vcenter.local") or a full URL
	// ("https://vcenter.local/sdk"), without embedded credentials.
	URL      string
	Username string
	Password string
	Insecure bool // skip TLS certificate verification
}

// keepAliveIdle is how long the SOAP session may sit idle before the
// keepalive handler pings the server to keep it from expiring. vCenter/ESXi
// default session timeout is 30 minutes; 10 minutes leaves ample margin.
const keepAliveIdle = 10 * time.Minute

// Client wraps *govmomi.Client with a keepalive-protected SOAP session and a
// ready-to-use inventory Finder. A REST/VAPI client (needed for VAMI
// appliance administration) is created lazily on first use via REST().
type Client struct {
	*govmomi.Client
	Finder *find.Finder

	cfg  Config       // retained for lazy REST login (see REST())
	rest *rest.Client // nil until the first appliance tool call
	mu   sync.Mutex
}

// NewClient connects to a vCenter/ESXi endpoint described by cfg, attaches a
// keepalive round-tripper BEFORE authenticating, then logs in and resolves
// the default datacenter for the Finder (if any is visible to the account
// used).
//
// Ordering matters: govmomi.NewClient only auto-logs-in when the SDK URL
// carries userinfo (see referencia/govmomi/client.go — "Only login if the
// URL contains user information"), and the keepalive handler only starts its
// ticker when it observes a login round-trip pass through it (see
// referencia/govmomi/session/keepalive/handler.go). So the URL passed to
// govmomi.NewClient here deliberately has NO userinfo — login happens
// afterward, once the keepalive round-tripper is already wrapping the
// client's transport.
func NewClient(ctx context.Context, cfg Config) (*Client, error) {
	return newClient(ctx, cfg, keepAliveIdle)
}

// newClient is NewClient with the keepalive idle interval as a parameter,
// so tests can compress it (production code always goes through NewClient,
// which fixes idle at keepAliveIdle).
func newClient(ctx context.Context, cfg Config, idle time.Duration) (*Client, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("url cannot be empty")
	}
	if cfg.Username == "" || cfg.Password == "" {
		return nil, fmt.Errorf("username and password are required")
	}

	u, err := sdkURL(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid vcenter url: %w", err)
	}

	gc, err := govmomi.NewClient(ctx, u, cfg.Insecure)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", u.Redacted(), err)
	}

	gc.RoundTripper = keepalive.NewHandlerSOAP(gc.RoundTripper, idle, nil)

	if err := gc.Login(ctx, url.UserPassword(cfg.Username, cfg.Password)); err != nil {
		return nil, fmt.Errorf("failed to authenticate to %s: %w", u.Redacted(), err)
	}

	finder := find.NewFinder(gc.Client, true)
	if dc, dcErr := finder.DefaultDatacenter(ctx); dcErr == nil {
		finder.SetDatacenter(dc)
	}
	// A missing/ambiguous default datacenter is not fatal here: tools that
	// need one call finder.SetDatacenter explicitly, and read-only,
	// datacenter-scoped tools will surface a clear error at call time.

	return &Client{Client: gc, Finder: finder, cfg: cfg}, nil
}

// REST returns a VAPI/REST client for VAMI (appliance administration) and
// other REST-only endpoints (tags, content library), logging in lazily on
// first call and reusing the session afterward. Connecting to an ESXi
// standalone host (no VAMI) fails here with a message naming that as the
// likely cause, rather than at server startup — a plain SOAP-only session
// (the common case, e.g. this project's ESXi test target) must not be
// penalized for a capability it was never going to use.
func (c *Client) REST(ctx context.Context) (*rest.Client, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.rest != nil {
		return c.rest, nil
	}

	rc := rest.NewClient(c.Client.Client)
	rc.Transport = keepalive.NewHandlerREST(rc, keepAliveIdle, nil)
	if err := rc.Login(ctx, url.UserPassword(c.cfg.Username, c.cfg.Password)); err != nil {
		return nil, fmt.Errorf("REST/VAPI login failed (is the target a vCenter Server Appliance? an ESXi standalone host has no VAMI): %w", err)
	}
	c.rest = rc
	return c.rest, nil
}

// Close logs out of the SOAP session (which also stops the keepalive
// ticker) and, if a REST session was ever established, logs that out too.
func (c *Client) Close(ctx context.Context) error {
	c.mu.Lock()
	rc := c.rest
	c.mu.Unlock()

	if rc != nil {
		if err := rc.Logout(ctx); err != nil {
			return fmt.Errorf("REST logout failed: %w", err)
		}
	}
	return c.Logout(ctx)
}

// sdkURL builds the govmomi SDK endpoint URL from a bare host or a full URL,
// defaulting the scheme to https and the path to the standard "/sdk". No
// userinfo is ever attached here — see NewClient's doc comment for why.
func sdkURL(raw string) (*url.URL, error) {
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	if u.Path == "" || u.Path == "/" {
		u.Path = "/sdk"
	}
	return u, nil
}

// Package cloudaws is a small HTTP client for VMware Cloud on AWS (VMC) —
// a third product distinct from vSphere/ESXi and VMware Workstation Pro,
// with no dependency on govmomi. Spec vendorized at .workspace/VMware
// Cloud on AWS APIs.postman_collection.json (99 operations). Auth is CSP
// (VMware Cloud Services Platform) token exchange — neither SOAP userpass
// nor plain Basic Auth: a long-lived refresh_token (obtained manually from
// the VMC console web UI, outside this MCP server's reach — there is no
// API to mint one) is exchanged for a short-lived access_token, sent as
// "Authorization: Bearer <access_token>" on every VMC API call.
package cloudaws

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// defaultAuthURL/defaultBaseURL are the real CSP/VMC endpoints. Config.AuthURL/
// BaseURL exist purely so tests can point at an httptest fixture instead —
// no VMC account/organization is available to this project (see the plan's
// Fase 10 "não-verificável-ponta-a-ponta" note), so every real call this
// client makes outside a test is against the actual VMware-operated service.
const (
	defaultAuthURL = "https://console.cloud.vmware.com"
	defaultBaseURL = "https://vmc.vmware.com"
)

// tokenExpiryBuffer is subtracted from the token's reported expires_in so a
// refresh is triggered comfortably before the real expiry, not exactly at
// it (avoids a request racing the token's last valid instant).
const tokenExpiryBuffer = 60 * time.Second

// Config holds the connection parameters for VMC on AWS.
type Config struct {
	// RefreshToken is the long-lived CSP API token generated manually in
	// the VMC console (console.cloud.vmware.com → My Account → API
	// Tokens) — this client never creates or renews the refresh token
	// itself, only the short-lived access token it's exchanged for.
	RefreshToken string
	// AuthURL overrides the CSP token endpoint's base ("https://console.
	// cloud.vmware.com" by default) — only ever set in tests, to point at
	// an httptest fixture instead of the real CSP service.
	AuthURL string
	// BaseURL overrides the VMC API base ("https://vmc.vmware.com" by
	// default) — only ever set in tests, same reasoning as AuthURL.
	BaseURL  string
	Insecure bool // skip TLS certificate verification (tests against an httptest fixture only — never needed for the real CSP/VMC hosts)
}

// Client is a thin wrapper over net/http.Client with automatic CSP token
// refresh — unlike vmware.Client (SOAP session + keepalive) and
// workstation.Client (Basic Auth, no session at all), VMC's session is a
// short-lived bearer token this client renews transparently on expiry.
type Client struct {
	authURL      string
	baseURL      string
	refreshToken string
	http         *http.Client

	mu          sync.Mutex
	accessToken string
	expiresAt   time.Time
}

// tokenResponse mirrors CSP's token-exchange response — the standard CSP
// OAuth2-style token shape documented publicly by VMware (console.cloud.
// vmware.com's own API docs), NOT verified against a live call in this
// project (no VMC account available — see the plan's Fase 10 note). Decode
// is deliberately tolerant (unknown fields ignored) so a real-world field
// this project never got to confirm doesn't break the exchange; only
// access_token/expires_in are actually used.
type tokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

// errorModel is a best-effort shape for VMC's error responses — like
// tokenResponse, based on VMware's public API docs, not verified live.
// Do falls back to the raw response body when this doesn't decode.
type errorModel struct {
	ErrorCode    string `json:"error_code"`
	ErrorMessage string `json:"error_message"`
	Message      string `json:"message"`
}

// NewClient builds a Client from cfg. No round-trip happens here (the
// first token exchange is deferred to the first real Do call, same lazy
// pattern as vmware.Client.REST) — fails fast only on a missing refresh
// token or a malformed override URL.
func NewClient(cfg Config) (*Client, error) {
	if strings.TrimSpace(cfg.RefreshToken) == "" {
		return nil, fmt.Errorf("VMC on AWS requires a CSP refresh token (generated manually in the VMC console — My Account → API Tokens); none was provided")
	}

	authURL := defaultAuthURL
	if cfg.AuthURL != "" {
		authURL = cfg.AuthURL
	}
	baseURL := defaultBaseURL
	if cfg.BaseURL != "" {
		baseURL = cfg.BaseURL
	}
	for name, u := range map[string]string{"AuthURL": authURL, "BaseURL": baseURL} {
		parsed, err := url.Parse(u)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return nil, fmt.Errorf("invalid %s %q: must be an absolute http(s) URL", name, u)
		}
	}

	transport := http.DefaultTransport
	if cfg.Insecure {
		transport = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}} //nolint:gosec // explicit opt-in via Config.Insecure, tests-only in practice (see Config.Insecure's doc comment)
	}

	return &Client{
		authURL:      strings.TrimRight(authURL, "/"),
		baseURL:      strings.TrimRight(baseURL, "/"),
		refreshToken: cfg.RefreshToken,
		http:         &http.Client{Transport: transport, Timeout: 30 * time.Second},
	}, nil
}

// ensureToken exchanges the refresh token for a fresh access token if none
// is cached or the cached one is within tokenExpiryBuffer of expiring.
func (c *Client) ensureToken(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.accessToken != "" && time.Now().Before(c.expiresAt) {
		return nil
	}

	authReqURL := c.authURL + "/csp/gateway/am/api/auth/api-tokens/authorize?refresh_token=" + url.QueryEscape(c.refreshToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, authReqURL, nil)
	if err != nil {
		return fmt.Errorf("failed to build CSP token exchange request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("CSP token exchange failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("CSP token exchange: failed to read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("CSP token exchange failed: %d %s: %s", resp.StatusCode, resp.Status, strings.TrimSpace(string(body)))
	}

	var tr tokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return fmt.Errorf("CSP token exchange: failed to decode response: %w", err)
	}
	if tr.AccessToken == "" {
		return fmt.Errorf("CSP token exchange: response had no access_token")
	}

	c.accessToken = tr.AccessToken
	ttl := time.Duration(tr.ExpiresIn) * time.Second
	if ttl <= tokenExpiryBuffer {
		ttl = tokenExpiryBuffer // never cache a token as "expiring in the past"
	}
	c.expiresAt = time.Now().Add(ttl - tokenExpiryBuffer)
	return nil
}

// Do issues method against path (relative to the configured VMC API base,
// e.g. "/vmc/api/orgs" or "/vmc/autoscaler/api/orgs/"+org+"/sddcs/"+sddc+
// "/edrs-policy"), refreshing the CSP access token first if needed,
// encoding body as JSON when non-nil, and decoding a JSON response into
// out when non-nil. On a 401 (token rejected/revoked server-side despite
// looking unexpired locally), forces exactly one re-exchange and retries
// once — never loops.
func (c *Client) Do(ctx context.Context, method, path string, body, out interface{}) error {
	if err := c.ensureToken(ctx); err != nil {
		return err
	}
	resp, retryable, err := c.doOnce(ctx, method, path, body, out)
	if retryable {
		c.mu.Lock()
		c.accessToken = "" // force a real re-exchange, not just an early-expiry check
		c.mu.Unlock()
		if err2 := c.ensureToken(ctx); err2 != nil {
			return err2
		}
		resp, _, err = c.doOnce(ctx, method, path, body, out)
	}
	_ = resp
	return err
}

// doOnce is one HTTP attempt. retryable is true only for a 401 — the one
// condition Do retries after forcing a fresh token exchange.
func (c *Client) doOnce(ctx context.Context, method, path string, body, out interface{}) (statusCode int, retryable bool, err error) {
	u := c.baseURL + path

	var reqBody io.Reader
	if body != nil {
		b, jsonErr := json.Marshal(body)
		if jsonErr != nil {
			return 0, false, fmt.Errorf("failed to encode request body: %w", jsonErr)
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, u, reqBody)
	if err != nil {
		return 0, false, fmt.Errorf("failed to build request: %w", err)
	}
	c.mu.Lock()
	token := c.accessToken
	c.mu.Unlock()
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, false, fmt.Errorf("%s %s: %w", method, u, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, false, fmt.Errorf("%s %s: failed to read response body: %w", method, u, err)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return resp.StatusCode, true, fmt.Errorf("%s %s: 401 Unauthorized (access token rejected)", method, u)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var em errorModel
		if jsonErr := json.Unmarshal(respBody, &em); jsonErr == nil && (em.ErrorMessage != "" || em.Message != "") {
			msg := em.ErrorMessage
			if msg == "" {
				msg = em.Message
			}
			return resp.StatusCode, false, fmt.Errorf("%s %s: %d %s (%s)", method, u, resp.StatusCode, msg, em.ErrorCode)
		}
		return resp.StatusCode, false, fmt.Errorf("%s %s: %d %s: %s", method, u, resp.StatusCode, resp.Status, strings.TrimSpace(string(respBody)))
	}

	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return resp.StatusCode, false, fmt.Errorf("%s %s: failed to decode response: %w", method, u, err)
		}
	}
	return resp.StatusCode, false, nil
}

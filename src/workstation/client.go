// Package workstation is a small HTTP client for VMware Workstation Pro's
// vmrest REST service — a product distinct from vSphere/ESXi, with no
// dependency on github.com/vmware/govmomi. Spec vendorized at
// .workspace/VMware Workstation Pro API.postman_collection.json (28
// operations, generated from the official swagger_WS.json — see
// .wolf/cerebrum.md "vmrest local testado ao vivo" for the live-test
// notes this client's conventions are based on).
package workstation

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
	"time"
)

// vmrestContentType is the media type every vmrest request/response uses —
// confirmed against the vendored swagger spec and a live vmrest 1.3.1
// instance (10/08/2026 session), not guessed.
const vmrestContentType = "application/vnd.vmware.vmw.rest-v1+json"

// Config holds the connection parameters for a local vmrest service.
// Mirrors vmware.Config's field names for consistency across the two
// client packages.
type Config struct {
	// URL is the vmrest base URL, e.g. "http://127.0.0.1:8697/api" (the
	// default vmrest listens on) or "https://127.0.0.1:8697/api" if the
	// service was started with -c/-k (HTTPS). Trailing slash optional.
	URL      string
	Username string
	Password string
	Insecure bool // skip TLS certificate verification (only relevant for https:// URLs)
}

// Client is a thin wrapper over net/http.Client — vmrest has no
// session/token concept (unlike vim25/vapi), Basic Auth is sent on every
// request, confirmed live against vmrest 1.3.1.
type Client struct {
	baseURL  *url.URL
	username string
	password string
	http     *http.Client
}

// errorModel mirrors vmrest's error response body. Field names are
// capitalized ("Code"/"Message") in the real service's JSON — confirmed
// live 10/08/2026, NOT the lowercase "code"/"message" the swagger spec's
// schema names would suggest — see .wolf/cerebrum.md.
type errorModel struct {
	Code    int    `json:"Code"`
	Message string `json:"Message"`
}

// NewClient builds a Client from cfg. No round-trip happens here — vmrest
// has no login/session step to fail on, so "connected" only really means
// "the first real request succeeded." Fails fast only on a malformed URL.
func NewClient(cfg Config) (*Client, error) {
	raw := strings.TrimRight(cfg.URL, "/")
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid vmrest URL %q: %w", cfg.URL, err)
	}
	if u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("invalid vmrest URL %q: must be an absolute http(s) URL, e.g. http://127.0.0.1:8697/api", cfg.URL)
	}

	transport := http.DefaultTransport
	if u.Scheme == "https" && cfg.Insecure {
		transport = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}} //nolint:gosec // explicit opt-in via Config.Insecure, same posture as vmware.Config.Insecure
	}

	return &Client{
		baseURL:  u,
		username: cfg.Username,
		password: cfg.Password,
		http:     &http.Client{Transport: transport, Timeout: 30 * time.Second},
	}, nil
}

// Do issues method against path (relative to the configured base URL, e.g.
// "/vms" or "/vms/"+id+"/power"), encoding body as JSON when non-nil and
// decoding a JSON response into out when non-nil. A non-2xx response is
// decoded as vmrest's errorModel and returned as a descriptive error — the
// same convention tools/appliance.go and the generated_vami_*.go handlers
// use for VAMI.
func (c *Client) Do(ctx context.Context, method, path string, body, out interface{}) error {
	var reqBody []byte
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to encode request body: %w", err)
		}
		reqBody = b
	}
	return c.doRaw(ctx, method, path, reqBody, out)
}

// DoRawBody is Do's counterpart for the one vmrest operation whose request
// body is a bare string, not JSON — PUT /vms/{id}/power, whose
// VMPowerOperation body is literally "on"/"off"/"shutdown"/"suspend"/
// "pause"/"unpause" (unquoted), confirmed reading the vendored Postman
// collection's request definition. vmrest still expects the same
// Content-Type header as every JSON body — this is NOT a generic
// text/plain escape hatch, it exists solely for that one route's quirk.
func (c *Client) DoRawBody(ctx context.Context, method, path string, rawBody []byte, out interface{}) error {
	return c.doRaw(ctx, method, path, rawBody, out)
}

// doRaw is the shared request/response plumbing behind Do and DoRawBody —
// send reqBody (already-encoded bytes, nil for no body) and decode the
// response.
func (c *Client) doRaw(ctx context.Context, method, path string, reqBody []byte, out interface{}) error {
	u := *c.baseURL
	u.Path = u.Path + path

	var bodyReader io.Reader
	if reqBody != nil {
		bodyReader = bytes.NewReader(reqBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), bodyReader)
	if err != nil {
		return fmt.Errorf("failed to build request: %w", err)
	}
	req.SetBasicAuth(c.username, c.password)
	req.Header.Set("Accept", vmrestContentType)
	if reqBody != nil {
		req.Header.Set("Content-Type", vmrestContentType)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, u.String(), err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("%s %s: failed to read response body: %w", method, u.String(), err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var em errorModel
		if jsonErr := json.Unmarshal(respBody, &em); jsonErr == nil && em.Message != "" {
			return fmt.Errorf("%s %s: %d %s (code %d)", method, u.String(), resp.StatusCode, em.Message, em.Code)
		}
		return fmt.Errorf("%s %s: %d %s: %s", method, u.String(), resp.StatusCode, resp.Status, strings.TrimSpace(string(respBody)))
	}

	if out == nil || len(respBody) == 0 {
		return nil
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("%s %s: failed to decode response: %w", method, u.String(), err)
	}
	return nil
}

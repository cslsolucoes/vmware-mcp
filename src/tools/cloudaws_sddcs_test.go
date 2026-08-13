package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/cslsoftwares/mcpvmware/cloudaws"
)

// cloudAWSSDDCsCapture records the most recent request the fixture server in
// this file received — method/path/query/decoded-JSON-body — so a test can
// assert not just "the tool returned no error" but "the right HTTP call,
// with the right query/body, actually went out." Same reasoning as
// generated_vami_network_system_test.go's vamiCapture. Guarded by a mutex
// even though this package's tests run sequentially (not t.Parallel) —
// cheap insurance, matches that precedent too.
type cloudAWSSDDCsCapture struct {
	mu       sync.Mutex
	method   string
	path     string
	rawQuery string
	body     map[string]interface{}
}

func (c *cloudAWSSDDCsCapture) record(r *http.Request) map[string]interface{} {
	var body map[string]interface{}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&body) // best-effort; GET/DELETE/no-body requests decode to nil, which is fine
	}
	c.mu.Lock()
	c.method = r.Method
	c.path = r.URL.Path
	c.rawQuery = r.URL.RawQuery
	c.body = body
	c.mu.Unlock()
	return body
}

func (c *cloudAWSSDDCsCapture) snapshot() (method, path, rawQuery string, body map[string]interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.method, c.path, c.rawQuery, c.body
}

func writeCloudAWSJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// newCloudAWSSDDCsFixture starts a fake CSP + VMC server covering every one
// of this file's 23 routes with a canned (stateless) response, plus a CSP
// token-exchange endpoint so cloudaws.Client's login path is exercised too
// (not bypassed) — same posture as cloudaws/client_test.go's fixture. No
// VMC simulator exists (see cloudaws/client.go's top doc comment), so this
// is a fixture unit test, not an integration test against a real service.
func newCloudAWSSDDCsFixture(t *testing.T) (*Registry, *cloudAWSSDDCsCapture) {
	t.Helper()
	capture := &cloudAWSSDDCsCapture{}
	mux := http.NewServeMux()

	mux.HandleFunc("POST /csp/gateway/am/api/auth/api-tokens/authorize", func(w http.ResponseWriter, r *http.Request) {
		writeCloudAWSJSON(w, map[string]interface{}{"access_token": "fixture-access-token", "expires_in": 3600})
	})

	// --- SDDCs (base) -----------------------------------------------------
	mux.HandleFunc("GET /vmc/api/orgs/{org}/sddcs", func(w http.ResponseWriter, r *http.Request) {
		capture.record(r)
		writeCloudAWSJSON(w, []map[string]interface{}{{"id": "sddc-1", "name": "demo-sddc"}})
	})
	mux.HandleFunc("GET /vmc/api/orgs/{org}/sddcs/{sddc}", func(w http.ResponseWriter, r *http.Request) {
		capture.record(r)
		writeCloudAWSJSON(w, map[string]interface{}{"id": r.PathValue("sddc"), "name": "demo-sddc", "num_hosts": float64(4)})
	})
	mux.HandleFunc("POST /vmc/api/orgs/{org}/sddcs", func(w http.ResponseWriter, r *http.Request) {
		body := capture.record(r)
		resp := map[string]interface{}{"id": "sddc-new", "task_id": "task-create-1"}
		for k, v := range body {
			resp[k] = v
		}
		writeCloudAWSJSON(w, resp)
	})
	mux.HandleFunc("DELETE /vmc/api/orgs/{org}/sddcs/{sddc}", func(w http.ResponseWriter, r *http.Request) {
		capture.record(r)
		writeCloudAWSJSON(w, map[string]interface{}{"task_id": "task-delete-1", "resource_id": r.PathValue("sddc")})
	})
	mux.HandleFunc("PATCH /vmc/api/orgs/{org}/sddcs/{sddc}", func(w http.ResponseWriter, r *http.Request) {
		body := capture.record(r)
		resp := map[string]interface{}{"id": r.PathValue("sddc"), "task_id": "task-update-1"}
		for k, v := range body {
			resp[k] = v
		}
		writeCloudAWSJSON(w, resp)
	})
	mux.HandleFunc("POST /vmc/api/orgs/{org}/sddcs/{sddc}/convert", func(w http.ResponseWriter, r *http.Request) {
		capture.record(r)
		writeCloudAWSJSON(w, map[string]interface{}{"task_id": "task-convert-1"})
	})

	// --- Cluster ------------------------------------------------------------
	mux.HandleFunc("POST /vmc/api/orgs/{org}/sddcs/{sddc}/clusters", func(w http.ResponseWriter, r *http.Request) {
		body := capture.record(r)
		resp := map[string]interface{}{"id": "cluster-new", "task_id": "task-cluster-create-1"}
		for k, v := range body {
			resp[k] = v
		}
		writeCloudAWSJSON(w, resp)
	})
	mux.HandleFunc("DELETE /vmc/api/orgs/{org}/sddcs/{sddc}/clusters/{cluster}", func(w http.ResponseWriter, r *http.Request) {
		capture.record(r)
		writeCloudAWSJSON(w, map[string]interface{}{"task_id": "task-cluster-delete-1", "cluster": r.PathValue("cluster")})
	})

	// --- Hosts --------------------------------------------------------------
	mux.HandleFunc("POST /vmc/api/orgs/{org}/sddcs/{sddc}/esxs", func(w http.ResponseWriter, r *http.Request) {
		capture.record(r)
		writeCloudAWSJSON(w, map[string]interface{}{"task_id": "task-esxs-1", "action": r.URL.Query().Get("action")})
	})

	// --- Cluster/EDRS (note: /vmc/autoscaler/api/, not /vmc/api/) -----------
	mux.HandleFunc("GET /vmc/autoscaler/api/orgs/{org}/sddcs/{sddc}/clusters/{cluster}/edrs-policy", func(w http.ResponseWriter, r *http.Request) {
		capture.record(r)
		writeCloudAWSJSON(w, map[string]interface{}{"enable_edrs": true, "policy_type": "performance", "min_hosts": float64(4), "max_hosts": float64(16)})
	})
	mux.HandleFunc("GET /vmc/autoscaler/api/orgs/{org}/sddcs/{sddc}/edrs-policy", func(w http.ResponseWriter, r *http.Request) {
		capture.record(r)
		writeCloudAWSJSON(w, []map[string]interface{}{{"cluster_id": "cluster-1", "enable_edrs": true}})
	})
	mux.HandleFunc("POST /vmc/autoscaler/api/orgs/{org}/sddcs/{sddc}/clusters/{cluster}/edrs-policy", func(w http.ResponseWriter, r *http.Request) {
		body := capture.record(r)
		writeCloudAWSJSON(w, body)
	})

	// --- DNS ------------------------------------------------------------
	mux.HandleFunc("PUT /vmc/api/orgs/{org}/sddcs/{sddc}/dns/public", func(w http.ResponseWriter, r *http.Request) {
		capture.record(r)
		writeCloudAWSJSON(w, map[string]interface{}{"updated": true, "type": "public"})
	})
	mux.HandleFunc("PUT /vmc/api/orgs/{org}/sddcs/{sddc}/dns/private", func(w http.ResponseWriter, r *http.Request) {
		capture.record(r)
		writeCloudAWSJSON(w, map[string]interface{}{"updated": true, "type": "private"})
	})

	// --- Public IPs -----------------------------------------------------------
	mux.HandleFunc("GET /vmc/api/orgs/{org}/sddcs/{sddc}/publicips", func(w http.ResponseWriter, r *http.Request) {
		capture.record(r)
		writeCloudAWSJSON(w, []map[string]interface{}{{"id": "ip-1", "public_ip": "203.0.113.1"}})
	})
	mux.HandleFunc("GET /vmc/api/orgs/{org}/sddcs/{sddc}/publicips/{id}", func(w http.ResponseWriter, r *http.Request) {
		capture.record(r)
		writeCloudAWSJSON(w, map[string]interface{}{"id": r.PathValue("id"), "public_ip": "203.0.113.1"})
	})
	mux.HandleFunc("POST /vmc/api/orgs/{org}/sddcs/{sddc}/publicips", func(w http.ResponseWriter, r *http.Request) {
		capture.record(r)
		writeCloudAWSJSON(w, map[string]interface{}{"id": "ip-new", "public_ip": "203.0.113.9"})
	})
	mux.HandleFunc("PATCH /vmc/api/orgs/{org}/sddcs/{sddc}/publicips/{id}", func(w http.ResponseWriter, r *http.Request) {
		capture.record(r)
		writeCloudAWSJSON(w, map[string]interface{}{"id": r.PathValue("id"), "action": r.URL.Query().Get("action")})
	})
	mux.HandleFunc("DELETE /vmc/api/orgs/{org}/sddcs/{sddc}/publicips/{id}", func(w http.ResponseWriter, r *http.Request) {
		capture.record(r)
		writeCloudAWSJSON(w, map[string]interface{}{"deleted": true, "id": r.PathValue("id")})
	})

	// --- Addon credentials --------------------------------------------------
	mux.HandleFunc("GET /vmc/api/orgs/{org}/sddcs/{sddcId}/addons/{addonType}/credentials", func(w http.ResponseWriter, r *http.Request) {
		capture.record(r)
		writeCloudAWSJSON(w, []map[string]interface{}{{"name": "cred-1"}})
	})
	mux.HandleFunc("GET /vmc/api/orgs/{org}/sddcs/{sddcId}/addons/{addonType}/credentials/{name}", func(w http.ResponseWriter, r *http.Request) {
		capture.record(r)
		writeCloudAWSJSON(w, map[string]interface{}{"name": r.PathValue("name")})
	})
	mux.HandleFunc("POST /vmc/api/orgs/{org}/sddcs/{sddcId}/addons/{addonType}/credentials", func(w http.ResponseWriter, r *http.Request) {
		body := capture.record(r)
		resp := map[string]interface{}{"name": "cred-new"}
		for k, v := range body {
			resp[k] = v
		}
		writeCloudAWSJSON(w, resp)
	})
	mux.HandleFunc("PUT /vmc/api/orgs/{org}/sddcs/{sddcId}/addons/{addonType}/credentials/{name}", func(w http.ResponseWriter, r *http.Request) {
		body := capture.record(r)
		resp := map[string]interface{}{"name": r.PathValue("name")}
		for k, v := range body {
			resp[k] = v
		}
		writeCloudAWSJSON(w, resp)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client, err := cloudaws.NewClient(cloudaws.Config{RefreshToken: "fixture-refresh-token", AuthURL: srv.URL, BaseURL: srv.URL, Insecure: true})
	if err != nil {
		t.Fatalf("cloudaws.NewClient: %v", err)
	}

	r := NewRegistry(context.Background(), nil, RegistryOptions{CloudAWSClient: client, ConnectionMode: ConnectionModeCloudAWS})
	return r, capture
}

// --- happy path: read-only tools -----------------------------------------

func TestCloudAWSSDDCs_ReadOnlyHappyPath(t *testing.T) {
	r, capture := newCloudAWSSDDCsFixture(t)

	cases := []struct {
		tool string
		args map[string]interface{}
	}{
		{"vmware_cloudaws_sddc_list", map[string]interface{}{"org": "org-1"}},
		{"vmware_cloudaws_sddc_get", map[string]interface{}{"org": "org-1", "sddc": "sddc-1"}},
		{"vmware_cloudaws_sddc_edrs_cluster_list", map[string]interface{}{"org": "org-1", "sddc": "sddc-1", "cluster": "cluster-1"}},
		{"vmware_cloudaws_sddc_edrs_list", map[string]interface{}{"org": "org-1", "sddc": "sddc-1"}},
		{"vmware_cloudaws_sddc_publicip_list", map[string]interface{}{"org": "org-1", "sddc": "sddc-1"}},
		{"vmware_cloudaws_sddc_publicip_get", map[string]interface{}{"org": "org-1", "sddc": "sddc-1", "id": "ip-1"}},
		{"vmware_cloudaws_sddc_addon_credential_list", map[string]interface{}{"org": "org-1", "sddc": "sddc-1", "addon_type": "HCX"}},
		{"vmware_cloudaws_sddc_addon_credential_get", map[string]interface{}{"org": "org-1", "sddc": "sddc-1", "addon_type": "HCX", "name": "cred-1"}},
	}

	for _, tc := range cases {
		raw, err := r.CallTool(tc.tool, tc.args)
		if err != nil {
			t.Fatalf("%s failed: %v", tc.tool, err)
		}
		if raw == "" {
			t.Fatalf("%s returned empty result", tc.tool)
		}
	}

	// GET /sddcs/{sddc} confirms path substitution actually reached the
	// fixture with the right sddc ID, not just "some 200 came back".
	raw, err := r.CallTool("vmware_cloudaws_sddc_get", map[string]interface{}{"org": "org-1", "sddc": "sddc-42"})
	if err != nil {
		t.Fatalf("vmware_cloudaws_sddc_get failed: %v", err)
	}
	m := decodeResult(t, raw)
	if m["id"] != "sddc-42" {
		t.Fatalf("expected echoed sddc id sddc-42, got %v (%s)", m["id"], raw)
	}
	_, path, _, _ := capture.snapshot()
	if path != "/vmc/api/orgs/org-1/sddcs/sddc-42" {
		t.Fatalf("unexpected captured request path: %s", path)
	}
}

// --- Tier 1 gate: sddc_create / sddc_delete --------------------------------
// The brief calls this out explicitly given the real financial cost of an
// accidental SDDC create/delete — verify the 3-layer gate (server flag +
// confirm:true) blocks both BEFORE any VMC round trip.

func TestCloudAWSSDDCs_CreateDeleteBlockedWithoutGate(t *testing.T) {
	r, capture := newCloudAWSSDDCsFixture(t) // AllowDestructive defaults to false

	if _, err := r.CallTool("vmware_cloudaws_sddc_create", map[string]interface{}{
		"org":     "org-1",
		"spec":    map[string]interface{}{"name": "should-not-happen", "num_hosts": float64(1), "provider": "AWS", "region": "US_WEST_2"},
		"confirm": true,
	}); err == nil {
		t.Fatal("expected sddc_create to be denied with the destructive gate closed")
	}
	if _, err := r.CallTool("vmware_cloudaws_sddc_delete", map[string]interface{}{
		"org": "org-1", "sddc": "sddc-1", "confirm": true,
	}); err == nil {
		t.Fatal("expected sddc_delete to be denied with the destructive gate closed")
	}

	method, _, _, _ := capture.snapshot()
	if method != "" {
		t.Fatalf("expected no VMC round trip at all, but a %s request reached the fixture", method)
	}
}

func TestCloudAWSSDDCs_CreateDeleteBlockedWithoutConfirm(t *testing.T) {
	// Gate open, to isolate the confirm:true check specifically (a
	// gate-closed test already proved the gate check alone in
	// TestCloudAWSSDDCs_CreateDeleteBlockedWithoutGate).
	fr, capture := newCloudAWSSDDCsFixtureWithGate(t, true)

	cases := []map[string]interface{}{
		{"org": "org-1", "spec": map[string]interface{}{"name": "x", "num_hosts": float64(1), "provider": "AWS", "region": "US_WEST_2"}}, // missing confirm
		{"org": "org-1", "spec": map[string]interface{}{"name": "x"}, "confirm": false},                                                  // explicit false
		{"org": "org-1", "spec": map[string]interface{}{"name": "x"}, "confirm": "true"},                                                 // truthy string, not bool
	}
	for _, args := range cases {
		if _, err := fr.CallTool("vmware_cloudaws_sddc_create", args); err == nil {
			t.Fatalf("expected sddc_create to be denied for args %#v (gate open, but confirm not exactly true)", args)
		}
	}

	method, _, _, _ := capture.snapshot()
	if method != "" {
		t.Fatalf("expected no VMC round trip without a strict confirm:true, but a %s request reached the fixture", method)
	}
}

func TestCloudAWSSDDCs_CreateSucceedsWithGateAndConfirm(t *testing.T) {
	r, capture := newCloudAWSSDDCsFixtureWithGate(t, true)

	raw, err := r.CallTool("vmware_cloudaws_sddc_create", map[string]interface{}{
		"org": "org-1",
		"spec": map[string]interface{}{
			"name": "my-new-sddc", "num_hosts": float64(1), "provider": "AWS", "region": "US_WEST_2",
		},
		"confirm": true,
	})
	if err != nil {
		t.Fatalf("expected sddc_create to succeed with gate open + confirm:true, got: %v", err)
	}
	m := decodeResult(t, raw)
	if m["name"] != "my-new-sddc" || m["id"] != "sddc-new" {
		t.Fatalf("unexpected create response: %s", raw)
	}

	method, path, _, body := capture.snapshot()
	if method != http.MethodPost || path != "/vmc/api/orgs/org-1/sddcs" {
		t.Fatalf("unexpected captured request: %s %s", method, path)
	}
	if body["region"] != "US_WEST_2" {
		t.Fatalf("expected the spec body to be forwarded as-is, got: %+v", body)
	}
}

func TestCloudAWSSDDCs_DeleteSucceedsWithGateAndConfirm(t *testing.T) {
	r, capture := newCloudAWSSDDCsFixtureWithGate(t, true)

	raw, err := r.CallTool("vmware_cloudaws_sddc_delete", map[string]interface{}{
		"org": "org-1", "sddc": "sddc-1", "confirm": true,
	})
	if err != nil {
		t.Fatalf("expected sddc_delete to succeed with gate open + confirm:true, got: %v", err)
	}
	m := decodeResult(t, raw)
	if m["resource_id"] != "sddc-1" {
		t.Fatalf("unexpected delete response: %s", raw)
	}

	method, path, _, _ := capture.snapshot()
	if method != http.MethodDelete || path != "/vmc/api/orgs/org-1/sddcs/sddc-1" {
		t.Fatalf("unexpected captured request: %s %s", method, path)
	}
}

// TestCloudAWSSDDCs_DeleteFallsBackToConfirmationWhenBodyEmpty proves
// cloudAWSSDDCWithFallback kicks in when VMC answers a write with no
// decodable body (e.g. a bare 204/200 with an empty body) — real VMC
// behavior for delete/convert/dns-update is unverifiable (no account
// available, see this file's top doc comment), so the tool must not surface
// an empty result either way.
func TestCloudAWSSDDCs_DeleteFallsBackToConfirmationWhenBodyEmpty(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /csp/gateway/am/api/auth/api-tokens/authorize", func(w http.ResponseWriter, r *http.Request) {
		writeCloudAWSJSON(w, map[string]interface{}{"access_token": "fixture-access-token", "expires_in": 3600})
	})
	mux.HandleFunc("DELETE /vmc/api/orgs/{org}/sddcs/{sddc}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent) // no body at all
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client, err := cloudaws.NewClient(cloudaws.Config{RefreshToken: "fixture-refresh-token", AuthURL: srv.URL, BaseURL: srv.URL, Insecure: true})
	if err != nil {
		t.Fatalf("cloudaws.NewClient: %v", err)
	}
	r := NewRegistry(context.Background(), nil, RegistryOptions{CloudAWSClient: client, ConnectionMode: ConnectionModeCloudAWS, AllowDestructive: true})

	raw, err := r.CallTool("vmware_cloudaws_sddc_delete", map[string]interface{}{"org": "org-1", "sddc": "sddc-1", "confirm": true})
	if err != nil {
		t.Fatalf("expected success despite an empty response body, got: %v", err)
	}
	m := decodeResult(t, raw)
	if m["deleted"] != true || m["sddc"] != "sddc-1" || m["org"] != "org-1" {
		t.Fatalf("expected a fallback confirmation object, got: %s", raw)
	}
}

// --- Tier 1: cluster/hosts/publicip write operations -----------------------

func TestCloudAWSSDDCs_ClusterAndHostsWritesRequireGate(t *testing.T) {
	r, _ := newCloudAWSSDDCsFixture(t) // gate closed by default

	writes := []struct {
		tool string
		args map[string]interface{}
	}{
		{"vmware_cloudaws_sddc_update", map[string]interface{}{"org": "org-1", "sddc": "sddc-1", "spec": map[string]interface{}{"name": "renamed"}, "confirm": true}},
		{"vmware_cloudaws_sddc_convert", map[string]interface{}{"org": "org-1", "sddc": "sddc-1", "confirm": true}},
		{"vmware_cloudaws_sddc_cluster_create", map[string]interface{}{"org": "org-1", "sddc": "sddc-1", "spec": map[string]interface{}{"num_hosts": float64(3)}, "confirm": true}},
		{"vmware_cloudaws_sddc_cluster_delete", map[string]interface{}{"org": "org-1", "sddc": "sddc-1", "cluster": "cluster-1", "confirm": true}},
		{"vmware_cloudaws_sddc_hosts_update", map[string]interface{}{"org": "org-1", "sddc": "sddc-1", "action": "add", "spec": map[string]interface{}{"num_hosts": float64(1)}, "confirm": true}},
		{"vmware_cloudaws_sddc_publicip_create", map[string]interface{}{"org": "org-1", "sddc": "sddc-1", "confirm": true}},
		{"vmware_cloudaws_sddc_publicip_update", map[string]interface{}{"org": "org-1", "sddc": "sddc-1", "id": "ip-1", "action": "attach", "confirm": true}},
		{"vmware_cloudaws_sddc_publicip_delete", map[string]interface{}{"org": "org-1", "sddc": "sddc-1", "id": "ip-1", "confirm": true}},
	}
	for _, w := range writes {
		if _, err := r.CallTool(w.tool, w.args); err == nil {
			t.Fatalf("expected %s to be denied with the destructive gate closed", w.tool)
		}
	}
}

func TestCloudAWSSDDCs_ClusterAndHostsWritesSucceedWithGate(t *testing.T) {
	r, capture := newCloudAWSSDDCsFixtureWithGate(t, true)

	raw, err := r.CallTool("vmware_cloudaws_sddc_cluster_create", map[string]interface{}{
		"org": "org-1", "sddc": "sddc-1", "spec": map[string]interface{}{"num_hosts": float64(3)}, "confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_cloudaws_sddc_cluster_create failed: %v", err)
	}
	m := decodeResult(t, raw)
	if m["id"] != "cluster-new" {
		t.Fatalf("unexpected cluster_create response: %s", raw)
	}

	if _, err := r.CallTool("vmware_cloudaws_sddc_cluster_delete", map[string]interface{}{
		"org": "org-1", "sddc": "sddc-1", "cluster": "cluster-1", "confirm": true,
	}); err != nil {
		t.Fatalf("vmware_cloudaws_sddc_cluster_delete failed: %v", err)
	}
	method, path, _, _ := capture.snapshot()
	if method != http.MethodDelete || path != "/vmc/api/orgs/org-1/sddcs/sddc-1/clusters/cluster-1" {
		t.Fatalf("unexpected captured request: %s %s", method, path)
	}

	raw, err = r.CallTool("vmware_cloudaws_sddc_hosts_update", map[string]interface{}{
		"org": "org-1", "sddc": "sddc-1", "action": "add", "spec": map[string]interface{}{"num_hosts": float64(2)}, "confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_cloudaws_sddc_hosts_update failed: %v", err)
	}
	m = decodeResult(t, raw)
	if m["action"] != "add" {
		t.Fatalf("expected the action query param to round-trip, got: %s", raw)
	}
	_, _, rawQuery, _ := capture.snapshot()
	if rawQuery != "action=add" {
		t.Fatalf("expected raw query action=add, got %q", rawQuery)
	}

	raw, err = r.CallTool("vmware_cloudaws_sddc_publicip_update", map[string]interface{}{
		"org": "org-1", "sddc": "sddc-1", "id": "ip-1", "action": "detach", "confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_cloudaws_sddc_publicip_update failed: %v", err)
	}
	m = decodeResult(t, raw)
	if m["action"] != "detach" {
		t.Fatalf("expected the action query param to round-trip, got: %s", raw)
	}

	if _, err := r.CallTool("vmware_cloudaws_sddc_publicip_delete", map[string]interface{}{
		"org": "org-1", "sddc": "sddc-1", "id": "ip-1", "confirm": true,
	}); err != nil {
		t.Fatalf("vmware_cloudaws_sddc_publicip_delete failed: %v", err)
	}
}

// --- Tier 2: EDRS set / DNS update / addon credentials ---------------------
// Same gate/confirm mechanics as Tier 1 (registerDestructiveCloudAWS applies
// identically regardless of tier — only the audit log's "tier" field
// differs), verified separately here because these 3 routes are this file's
// deliberate departure from "every SDDCs write is tier1" (see the file's top
// doc comment) — worth pinning down on their own.

func TestCloudAWSSDDCs_Tier2WritesRequireGate(t *testing.T) {
	r, _ := newCloudAWSSDDCsFixture(t)

	writes := []struct {
		tool string
		args map[string]interface{}
	}{
		{"vmware_cloudaws_sddc_edrs_cluster_set", map[string]interface{}{"org": "org-1", "sddc": "sddc-1", "cluster": "cluster-1", "spec": map[string]interface{}{"enable_edrs": true}, "confirm": true}},
		{"vmware_cloudaws_sddc_dns_update_public", map[string]interface{}{"org": "org-1", "sddc": "sddc-1", "confirm": true}},
		{"vmware_cloudaws_sddc_dns_update_private", map[string]interface{}{"org": "org-1", "sddc": "sddc-1", "confirm": true}},
		{"vmware_cloudaws_sddc_addon_credential_create", map[string]interface{}{"org": "org-1", "sddc": "sddc-1", "addon_type": "HCX", "spec": map[string]interface{}{"username": "svc"}, "confirm": true}},
		{"vmware_cloudaws_sddc_addon_credential_update", map[string]interface{}{"org": "org-1", "sddc": "sddc-1", "addon_type": "HCX", "name": "cred-1", "spec": map[string]interface{}{"username": "svc"}, "confirm": true}},
	}
	for _, w := range writes {
		if _, err := r.CallTool(w.tool, w.args); err == nil {
			t.Fatalf("expected %s (tier2) to be denied with the destructive gate closed", w.tool)
		}
	}
}

func TestCloudAWSSDDCs_Tier2WritesSucceedWithGate(t *testing.T) {
	r, capture := newCloudAWSSDDCsFixtureWithGate(t, true)

	raw, err := r.CallTool("vmware_cloudaws_sddc_edrs_cluster_set", map[string]interface{}{
		"org": "org-1", "sddc": "sddc-1", "cluster": "cluster-1",
		"spec":    map[string]interface{}{"enable_edrs": true, "policy_type": "performance", "min_hosts": float64(4), "max_hosts": float64(16)},
		"confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_cloudaws_sddc_edrs_cluster_set failed: %v", err)
	}
	m := decodeResult(t, raw)
	if m["policy_type"] != "performance" {
		t.Fatalf("expected the EDRS spec body to be forwarded as-is, got: %s", raw)
	}
	method, path, _, _ := capture.snapshot()
	if method != http.MethodPost || path != "/vmc/autoscaler/api/orgs/org-1/sddcs/sddc-1/clusters/cluster-1/edrs-policy" {
		t.Fatalf("unexpected captured request (wrong prefix?): %s %s", method, path)
	}

	if _, err := r.CallTool("vmware_cloudaws_sddc_dns_update_public", map[string]interface{}{"org": "org-1", "sddc": "sddc-1", "confirm": true}); err != nil {
		t.Fatalf("vmware_cloudaws_sddc_dns_update_public failed: %v", err)
	}
	if _, err := r.CallTool("vmware_cloudaws_sddc_dns_update_private", map[string]interface{}{"org": "org-1", "sddc": "sddc-1", "confirm": true}); err != nil {
		t.Fatalf("vmware_cloudaws_sddc_dns_update_private failed: %v", err)
	}

	raw, err = r.CallTool("vmware_cloudaws_sddc_addon_credential_create", map[string]interface{}{
		"org": "org-1", "sddc": "sddc-1", "addon_type": "HCX", "spec": map[string]interface{}{"username": "svc-account"}, "confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_cloudaws_sddc_addon_credential_create failed: %v", err)
	}
	m = decodeResult(t, raw)
	if m["username"] != "svc-account" {
		t.Fatalf("expected the credential spec body to be forwarded as-is, got: %s", raw)
	}

	if _, err := r.CallTool("vmware_cloudaws_sddc_addon_credential_update", map[string]interface{}{
		"org": "org-1", "sddc": "sddc-1", "addon_type": "HCX", "name": "cred-1", "spec": map[string]interface{}{"username": "svc-account-2"}, "confirm": true,
	}); err != nil {
		t.Fatalf("vmware_cloudaws_sddc_addon_credential_update failed: %v", err)
	}
}

// --- required-argument validation ------------------------------------------

func TestCloudAWSSDDCs_RequiredArgsValidation(t *testing.T) {
	r, capture := newCloudAWSSDDCsFixtureWithGate(t, true)

	cases := []struct {
		name string
		tool string
		args map[string]interface{}
	}{
		{"list missing org", "vmware_cloudaws_sddc_list", map[string]interface{}{}},
		{"get missing sddc", "vmware_cloudaws_sddc_get", map[string]interface{}{"org": "org-1"}},
		{"create missing spec", "vmware_cloudaws_sddc_create", map[string]interface{}{"org": "org-1", "confirm": true}},
		{"create empty spec", "vmware_cloudaws_sddc_create", map[string]interface{}{"org": "org-1", "spec": map[string]interface{}{}, "confirm": true}},
		{"cluster_create missing spec", "vmware_cloudaws_sddc_cluster_create", map[string]interface{}{"org": "org-1", "sddc": "sddc-1", "confirm": true}},
		{"hosts_update missing action", "vmware_cloudaws_sddc_hosts_update", map[string]interface{}{"org": "org-1", "sddc": "sddc-1", "spec": map[string]interface{}{"num_hosts": float64(1)}, "confirm": true}},
		{"edrs_cluster_set missing cluster", "vmware_cloudaws_sddc_edrs_cluster_set", map[string]interface{}{"org": "org-1", "sddc": "sddc-1", "spec": map[string]interface{}{"enable_edrs": true}, "confirm": true}},
		{"publicip_get missing id", "vmware_cloudaws_sddc_publicip_get", map[string]interface{}{"org": "org-1", "sddc": "sddc-1"}},
		{"publicip_update missing action", "vmware_cloudaws_sddc_publicip_update", map[string]interface{}{"org": "org-1", "sddc": "sddc-1", "id": "ip-1", "confirm": true}},
		{"addon_credential_create missing addon_type", "vmware_cloudaws_sddc_addon_credential_create", map[string]interface{}{"org": "org-1", "sddc": "sddc-1", "spec": map[string]interface{}{"username": "x"}, "confirm": true}},
		{"addon_credential_update missing spec", "vmware_cloudaws_sddc_addon_credential_update", map[string]interface{}{"org": "org-1", "sddc": "sddc-1", "addon_type": "HCX", "name": "cred-1", "confirm": true}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := r.CallTool(tc.tool, tc.args); err == nil {
				t.Fatalf("expected %s to fail validation for args %#v", tc.tool, tc.args)
			}
		})
	}

	// None of the above should have reached the fixture server at all —
	// argument validation must happen before any round trip.
	method, _, _, _ := capture.snapshot()
	if method != "" {
		t.Fatalf("expected no VMC round trip for any invalid-args case, but a %s request reached the fixture", method)
	}
}

// newCloudAWSSDDCsFixtureWithGate is newCloudAWSSDDCsFixture with
// AllowDestructive explicitly set — split out because most tests need the
// gate closed (the default) and only a handful need it open.
func newCloudAWSSDDCsFixtureWithGate(t *testing.T, allowDestructive bool) (*Registry, *cloudAWSSDDCsCapture) {
	t.Helper()
	r, capture := newCloudAWSSDDCsFixture(t)
	if !allowDestructive {
		return r, capture
	}
	// Rebuild with the gate open — newCloudAWSSDDCsFixture always starts
	// closed (the project's own default), so re-derive a Registry against
	// the same already-built client instead of duplicating the whole mux.
	return NewRegistry(context.Background(), nil, RegistryOptions{CloudAWSClient: r.cloudClient, ConnectionMode: ConnectionModeCloudAWS, AllowDestructive: true}), capture
}

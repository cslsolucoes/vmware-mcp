package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// vamiAccessServicesCapture records the decoded JSON body of every request the fixture
// server receives, keyed by "METHOD /path" — lets a test assert the
// handler actually built and sent the body it claims to (e.g. that
// vmware_appliance_techpreview_snmp_set really put the caller's config
// object under a "config" wrapper), the same "no simulator, no live
// target" verification gap tools/appliance_test.go's fixture doesn't need
// to cover (that domain is GET-only).
type vamiAccessServicesCapture struct {
	bodies map[string]map[string]interface{}
}

func newVAMIAccessServicesCapture() *vamiAccessServicesCapture {
	return &vamiAccessServicesCapture{bodies: make(map[string]map[string]interface{})}
}

// handle returns an http.HandlerFunc that records the request body (if
// any) under "METHOD /path" and replies with value wrapped in the
// {"value": ...} envelope every /rest/... endpoint uses (see
// vapi/rest/client.go's Do).
func (c *vamiAccessServicesCapture) handle(value interface{}) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&body) // empty body -> EOF -> body stays nil, fine
		}
		c.bodies[r.Method+" "+r.URL.Path] = body
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"value": value})
	}
}

// vamiServicesAccountsVmonFixture starts a fake VAMI server covering every
// route generated_vami_services_accounts_vmon.go's 27 tools hit — same
// "session endpoint + canned /appliance/... responses" shape as
// tools/appliance_test.go's applianceFixture (Fase 4), extended to
// POST/PUT/PATCH/DELETE since this group's routes are not GET-only. No
// vcsim coverage exists for any of these routes — see this file's sibling
// production file's top doc comment.
func vamiServicesAccountsVmonFixture(t *testing.T) (*httptest.Server, *vamiAccessServicesCapture) {
	t.Helper()
	mux := http.NewServeMux()
	capture := newVAMIAccessServicesCapture()

	mux.HandleFunc("POST /rest/com/vmware/cis/session", capture.handle("fixture-session-id"))

	// SNMP
	mux.HandleFunc("GET /rest/appliance/techpreview/monitoring/snmp", capture.handle(map[string]interface{}{"port": float64(161), "authentication": "none"}))
	mux.HandleFunc("POST /rest/appliance/techpreview/monitoring/snmp/enable", capture.handle(nil))
	mux.HandleFunc("POST /rest/appliance/techpreview/monitoring/snmp/disable", capture.handle(nil))
	mux.HandleFunc("GET /rest/appliance/techpreview/monitoring/snmp/stats", capture.handle(map[string]interface{}{"packets_in": float64(42)}))
	mux.HandleFunc("POST /rest/appliance/techpreview/monitoring/snmp/test", capture.handle(nil))
	mux.HandleFunc("POST /rest/appliance/techpreview/monitoring/snmp/hash", capture.handle(map[string]interface{}{"auth_hash": "hashed-auth", "priv_hash": "hashed-priv"}))
	mux.HandleFunc("POST /rest/appliance/techpreview/monitoring/snmp", capture.handle(nil))
	mux.HandleFunc("POST /rest/appliance/techpreview/monitoring/snmp/reset", capture.handle(nil))
	mux.HandleFunc("GET /rest/appliance/techpreview/monitoring/snmp/limits", capture.handle(map[string]interface{}{"max_communities": float64(5)}))

	// Services
	mux.HandleFunc("GET /rest/appliance/techpreview/services", capture.handle([]interface{}{map[string]interface{}{"name": "vmware-vpxd"}}))
	mux.HandleFunc("POST /rest/appliance/techpreview/services/status/get", capture.handle(map[string]interface{}{"state": "STARTED"}))
	mux.HandleFunc("POST /rest/appliance/techpreview/services/control", capture.handle(nil))
	mux.HandleFunc("POST /rest/appliance/techpreview/services/restart", capture.handle(nil))
	mux.HandleFunc("POST /rest/appliance/techpreview/services/stop", capture.handle(nil))

	// System update
	mux.HandleFunc("GET /rest/appliance/techpreview/system/update", capture.handle(map[string]interface{}{"state": "UP_TO_DATE"}))
	mux.HandleFunc("POST /rest/appliance/techpreview/system/update", capture.handle(nil))

	// Local accounts
	mux.HandleFunc("GET /rest/appliance/techpreview/local-accounts/user", capture.handle([]interface{}{map[string]interface{}{"username": "root"}}))
	mux.HandleFunc("GET /rest/appliance/techpreview/local-accounts/user/{username}", capture.handle(map[string]interface{}{"username": "root", "role": "admin"}))
	mux.HandleFunc("POST /rest/appliance/techpreview/local-accounts/user", capture.handle(nil))
	mux.HandleFunc("DELETE /rest/appliance/techpreview/local-accounts/user/{username}", capture.handle(nil))
	mux.HandleFunc("PUT /rest/appliance/techpreview/local-accounts/user", capture.handle(nil))

	// Vmon
	mux.HandleFunc("GET /rest/appliance/vmon/service", capture.handle([]interface{}{"content-library"}))
	mux.HandleFunc("GET /rest/appliance/vmon/service/{id}", capture.handle(map[string]interface{}{"state": "RUNNING"}))
	mux.HandleFunc("POST /rest/appliance/vmon/service/{id}/restart", capture.handle(nil))
	mux.HandleFunc("POST /rest/appliance/vmon/service/{id}/start", capture.handle(nil))
	mux.HandleFunc("POST /rest/appliance/vmon/service/{id}/stop", capture.handle(nil))
	mux.HandleFunc("PATCH /rest/appliance/vmon/service/{id}", capture.handle(nil))

	return httptest.NewServer(mux), capture
}

// newVAMIServicesAccountsVmonRegistry builds a Registry the normal way and
// then manually layers registerVAMIServicesAccountsVmonTools on top via
// withClass — same pattern as newVAMINetworkSystemRegistry
// (generated_vami_network_system_test.go, a sibling Fase 8b group). Required
// because this group's brief explicitly says not to edit registry.go, so
// NewRegistry's own registerTools() sweep does not call this file's
// register function on its own.
func newVAMIServicesAccountsVmonRegistry(ctx context.Context, c *vmware.Client, opts RegistryOptions) *Registry {
	r := NewRegistry(ctx, c, opts)
	r.withClass(modeVCenterOnly, registerVAMIServicesAccountsVmonTools)
	return r
}

// --- Happy path: plain (non-destructive) reads ------------------------------

func TestVAMIServicesAccountsVmon_PlainReads(t *testing.T) {
	srv, _ := vamiServicesAccountsVmonFixture(t)
	defer srv.Close()
	c := newApplianceFixtureClient(t, srv)
	r := newVAMIServicesAccountsVmonRegistry(context.Background(), c, RegistryOptions{})

	cases := []struct {
		tool string
		args map[string]interface{}
	}{
		{"vmware_appliance_techpreview_snmp_get", map[string]interface{}{}},
		{"vmware_appliance_techpreview_snmp_stats", map[string]interface{}{}},
		{"vmware_appliance_techpreview_snmp_test", map[string]interface{}{}},
		{"vmware_appliance_techpreview_snmp_generate_hash", map[string]interface{}{"auth_hash": "secret", "raw_secret": true}},
		{"vmware_appliance_techpreview_snmp_limits", map[string]interface{}{}},
		{"vmware_appliance_techpreview_services_list", map[string]interface{}{}},
		{"vmware_appliance_techpreview_services_get", map[string]interface{}{"name": "vmware-vpxd"}},
		{"vmware_appliance_techpreview_system_update_get", map[string]interface{}{}},
		{"vmware_appliance_techpreview_local_accounts_list", map[string]interface{}{}},
		{"vmware_appliance_techpreview_local_accounts_get", map[string]interface{}{"username": "root"}},
		{"vmware_appliance_vmon_service_list", map[string]interface{}{}},
		{"vmware_appliance_vmon_service_get", map[string]interface{}{"service_id": "content-library"}},
	}

	for _, tc := range cases {
		t.Run(tc.tool, func(t *testing.T) {
			raw, err := r.CallTool(tc.tool, tc.args)
			if err != nil {
				t.Fatalf("%s failed: %v", tc.tool, err)
			}
			if raw == "" {
				t.Fatalf("%s returned an empty result", tc.tool)
			}
		})
	}
}

// --- Happy path + body verification: destructive (tier1/tier2) tools -------

func TestVAMIServicesAccountsVmon_DestructiveToolsSendExpectedBodyWhenAllowed(t *testing.T) {
	srv, capture := vamiServicesAccountsVmonFixture(t)
	defer srv.Close()
	c := newApplianceFixtureClient(t, srv)
	r := newVAMIServicesAccountsVmonRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	t.Run("snmp_enable", func(t *testing.T) {
		if _, err := r.CallTool("vmware_appliance_techpreview_snmp_enable", map[string]interface{}{"confirm": true}); err != nil {
			t.Fatalf("failed: %v", err)
		}
	})

	t.Run("snmp_disable", func(t *testing.T) {
		if _, err := r.CallTool("vmware_appliance_techpreview_snmp_disable", map[string]interface{}{"confirm": true}); err != nil {
			t.Fatalf("failed: %v", err)
		}
	})

	t.Run("snmp_set sends config under a config wrapper", func(t *testing.T) {
		cfg := map[string]interface{}{"port": float64(1161), "authentication": "SHA"}
		if _, err := r.CallTool("vmware_appliance_techpreview_snmp_set", map[string]interface{}{"config": cfg, "confirm": true}); err != nil {
			t.Fatalf("failed: %v", err)
		}
		body := capture.bodies["POST /rest/appliance/techpreview/monitoring/snmp"]
		got, ok := body["config"].(map[string]interface{})
		if !ok {
			t.Fatalf("expected a config object in the request body, got %#v", body)
		}
		if got["port"] != float64(1161) || got["authentication"] != "SHA" {
			t.Fatalf("config was not sent through unmodified: %#v", got)
		}
	})

	t.Run("snmp_reset is tier1", func(t *testing.T) {
		if _, err := r.CallTool("vmware_appliance_techpreview_snmp_reset", map[string]interface{}{"confirm": true}); err != nil {
			t.Fatalf("failed: %v", err)
		}
	})

	t.Run("services_control sends name/args/timeout", func(t *testing.T) {
		args := map[string]interface{}{"name": "vmware-vpxd", "args": []interface{}{"--foo"}, "timeout": 30, "confirm": true}
		if _, err := r.CallTool("vmware_appliance_techpreview_services_control", args); err != nil {
			t.Fatalf("failed: %v", err)
		}
		body := capture.bodies["POST /rest/appliance/techpreview/services/control"]
		if body["name"] != "vmware-vpxd" {
			t.Fatalf("expected name in body, got %#v", body)
		}
		argList, ok := body["args"].([]interface{})
		if !ok || len(argList) != 1 || argList[0] != "--foo" {
			t.Fatalf("expected args [\"--foo\"] in body, got %#v", body["args"])
		}
	})

	t.Run("services_restart", func(t *testing.T) {
		if _, err := r.CallTool("vmware_appliance_techpreview_services_restart", map[string]interface{}{"name": "vmware-vpxd", "confirm": true}); err != nil {
			t.Fatalf("failed: %v", err)
		}
	})

	t.Run("services_stop", func(t *testing.T) {
		if _, err := r.CallTool("vmware_appliance_techpreview_services_stop", map[string]interface{}{"name": "vmware-vpxd", "confirm": true}); err != nil {
			t.Fatalf("failed: %v", err)
		}
	})

	t.Run("system_update_set maps current_url to current_URL", func(t *testing.T) {
		args := map[string]interface{}{"current_url": "https://repo.example.com", "username": "svc", "confirm": true}
		if _, err := r.CallTool("vmware_appliance_techpreview_system_update_set", args); err != nil {
			t.Fatalf("failed: %v", err)
		}
		body := capture.bodies["POST /rest/appliance/techpreview/system/update"]
		got, ok := body["config"].(map[string]interface{})
		if !ok {
			t.Fatalf("expected a config object, got %#v", body)
		}
		if got["current_URL"] != "https://repo.example.com" {
			t.Fatalf("expected current_url to be sent as current_URL, got %#v", got)
		}
	})

	t.Run("local_accounts_create", func(t *testing.T) {
		args := map[string]interface{}{"username": "newuser", "password": "s3cret", "role": "admin", "confirm": true}
		if _, err := r.CallTool("vmware_appliance_techpreview_local_accounts_create", args); err != nil {
			t.Fatalf("failed: %v", err)
		}
		body := capture.bodies["POST /rest/appliance/techpreview/local-accounts/user"]
		got, ok := body["config"].(map[string]interface{})
		if !ok || got["username"] != "newuser" || got["password"] != "s3cret" {
			t.Fatalf("expected create config with username/password, got %#v", body)
		}
	})

	t.Run("local_accounts_delete is tier1", func(t *testing.T) {
		if _, err := r.CallTool("vmware_appliance_techpreview_local_accounts_delete", map[string]interface{}{"username": "olduser", "confirm": true}); err != nil {
			t.Fatalf("failed: %v", err)
		}
	})

	t.Run("local_accounts_update identifies the account via config.username", func(t *testing.T) {
		args := map[string]interface{}{"username": "existing", "status": "disabled", "confirm": true}
		if _, err := r.CallTool("vmware_appliance_techpreview_local_accounts_update", args); err != nil {
			t.Fatalf("failed: %v", err)
		}
		body := capture.bodies["PUT /rest/appliance/techpreview/local-accounts/user"]
		got, ok := body["config"].(map[string]interface{})
		if !ok || got["username"] != "existing" || got["status"] != "disabled" {
			t.Fatalf("expected update config with username/status, got %#v", body)
		}
	})

	t.Run("vmon_service_restart", func(t *testing.T) {
		if _, err := r.CallTool("vmware_appliance_vmon_service_restart", map[string]interface{}{"service_id": "content-library", "confirm": true}); err != nil {
			t.Fatalf("failed: %v", err)
		}
	})

	t.Run("vmon_service_start", func(t *testing.T) {
		if _, err := r.CallTool("vmware_appliance_vmon_service_start", map[string]interface{}{"service_id": "content-library", "confirm": true}); err != nil {
			t.Fatalf("failed: %v", err)
		}
	})

	t.Run("vmon_service_stop", func(t *testing.T) {
		if _, err := r.CallTool("vmware_appliance_vmon_service_stop", map[string]interface{}{"service_id": "content-library", "confirm": true}); err != nil {
			t.Fatalf("failed: %v", err)
		}
	})

	t.Run("vmon_service_update sends spec.startup_type", func(t *testing.T) {
		args := map[string]interface{}{"service_id": "content-library", "startup_type": "AUTOMATIC", "confirm": true}
		if _, err := r.CallTool("vmware_appliance_vmon_service_update", args); err != nil {
			t.Fatalf("failed: %v", err)
		}
		body := capture.bodies["PATCH /rest/appliance/vmon/service/content-library"]
		spec, ok := body["spec"].(map[string]interface{})
		if !ok || spec["startup_type"] != "AUTOMATIC" {
			t.Fatalf("expected spec.startup_type=AUTOMATIC, got %#v", body)
		}
	})
}

// --- Tier gating (registerDestructive wiring) -------------------------------

func TestVAMIServicesAccountsVmon_GateClosedDeniesTier1AndTier2(t *testing.T) {
	srv, _ := vamiServicesAccountsVmonFixture(t)
	defer srv.Close()
	c := newApplianceFixtureClient(t, srv)
	r := newVAMIServicesAccountsVmonRegistry(context.Background(), c, RegistryOptions{AllowDestructive: false})

	cases := []struct {
		tool string
		args map[string]interface{}
	}{
		{"vmware_appliance_techpreview_snmp_reset", map[string]interface{}{"confirm": true}},                                // tier1
		{"vmware_appliance_techpreview_local_accounts_delete", map[string]interface{}{"username": "root", "confirm": true}}, // tier1
		{"vmware_appliance_techpreview_snmp_enable", map[string]interface{}{"confirm": true}},                               // tier2
		{"vmware_appliance_techpreview_services_restart", map[string]interface{}{"name": "vmware-vpxd", "confirm": true}},   // tier2
		{"vmware_appliance_vmon_service_restart", map[string]interface{}{"service_id": "content-library", "confirm": true}}, // tier2
	}
	for _, tc := range cases {
		t.Run(tc.tool, func(t *testing.T) {
			if _, err := r.CallTool(tc.tool, tc.args); err == nil {
				t.Fatalf("%s: expected an error with the gate closed, got none", tc.tool)
			}
		})
	}
}

func TestVAMIServicesAccountsVmon_ConfirmRequiredEvenWithGateOpen(t *testing.T) {
	srv, _ := vamiServicesAccountsVmonFixture(t)
	defer srv.Close()
	c := newApplianceFixtureClient(t, srv)
	r := newVAMIServicesAccountsVmonRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	if _, err := r.CallTool("vmware_appliance_techpreview_snmp_enable", map[string]interface{}{}); err == nil {
		t.Fatal("expected an error without confirm:true, got none")
	}
	if _, err := r.CallTool("vmware_appliance_techpreview_local_accounts_delete", map[string]interface{}{"username": "root"}); err == nil {
		t.Fatal("expected an error without confirm:true, got none")
	}
}

// --- Argument validation -----------------------------------------------------

func TestVAMIServicesAccountsVmon_ArgumentValidation(t *testing.T) {
	srv, _ := vamiServicesAccountsVmonFixture(t)
	defer srv.Close()
	c := newApplianceFixtureClient(t, srv)
	r := newVAMIServicesAccountsVmonRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	cases := []struct {
		name string
		tool string
		args map[string]interface{}
	}{
		{"services_get missing name", "vmware_appliance_techpreview_services_get", map[string]interface{}{}},
		{"local_accounts_get missing username", "vmware_appliance_techpreview_local_accounts_get", map[string]interface{}{}},
		{"local_accounts_create missing password", "vmware_appliance_techpreview_local_accounts_create", map[string]interface{}{"username": "x", "confirm": true}},
		{"local_accounts_delete missing username", "vmware_appliance_techpreview_local_accounts_delete", map[string]interface{}{"confirm": true}},
		{"vmon_service_get missing service_id", "vmware_appliance_vmon_service_get", map[string]interface{}{}},
		{"vmon_service_update missing startup_type", "vmware_appliance_vmon_service_update", map[string]interface{}{"service_id": "content-library", "confirm": true}},
		{"snmp_set missing config", "vmware_appliance_techpreview_snmp_set", map[string]interface{}{"confirm": true}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := r.CallTool(tc.tool, tc.args); err == nil {
				t.Fatalf("%s: expected a validation error, got none", tc.tool)
			}
		})
	}
}

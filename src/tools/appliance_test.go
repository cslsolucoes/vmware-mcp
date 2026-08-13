package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/soap"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// applianceFixture starts a fake VAMI server: a session endpoint (so
// vmware.Client.REST()'s Login succeeds) plus canned /appliance/... GET
// responses, all wrapped in the {"value": ...} envelope govmomi's
// rest.Client expects from /rest endpoints (see vapi/rest/client.go). No
// simulator covers /appliance/system/version or /appliance/health/* (only
// access/logging/networking/shutdown — see the plan's Fase 4 note), so this
// is a fixture unit test, not a vcsim integration test like every other
// domain's tests in this package.
func applianceFixture(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	writeValue := func(w http.ResponseWriter, v interface{}) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"value": v})
	}

	mux.HandleFunc("/rest/com/vmware/cis/session", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeValue(w, "fixture-session-id")
	})
	mux.HandleFunc("/rest/appliance/system/version", func(w http.ResponseWriter, r *http.Request) {
		writeValue(w, map[string]interface{}{"version": "8.0.3", "build": "99999", "product": "VMware vCenter Server"})
	})
	mux.HandleFunc("/rest/appliance/system/uptime", func(w http.ResponseWriter, r *http.Request) {
		writeValue(w, float64(123456))
	})
	for _, sub := range applianceHealthSubsystems {
		sub := sub
		mux.HandleFunc("/rest/appliance/health/"+sub, func(w http.ResponseWriter, r *http.Request) {
			writeValue(w, "green")
		})
	}

	return httptest.NewServer(mux)
}

// newApplianceFixtureClient builds a *vmware.Client whose REST() calls hit
// srv — no SOAP login or simulator involved, since VAMI's REST session
// (POST /rest/com/vmware/cis/session) is independent of the SOAP session
// (vmware/client.go's REST() only needs a *vim25.Client to build a REST
// client from; it never touches ServiceContent).
func newApplianceFixtureClient(t *testing.T, srv *httptest.Server) *vmware.Client {
	t.Helper()
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("failed to parse fixture URL: %v", err)
	}
	vc := &vim25.Client{Client: soap.NewClient(u, true)}
	return &vmware.Client{Client: &govmomi.Client{Client: vc}}
}

func TestApplianceTools_Version(t *testing.T) {
	srv := applianceFixture(t)
	defer srv.Close()
	c := newApplianceFixtureClient(t, srv)
	r := NewRegistry(context.Background(), c, RegistryOptions{})

	raw, err := r.CallTool("vmware_appliance_version", map[string]interface{}{})
	if err != nil {
		t.Fatalf("vmware_appliance_version failed: %v", err)
	}
	m := decodeResult(t, raw)
	if m["version"] != "8.0.3" || m["build"] != "99999" {
		t.Fatalf("unexpected version result: %s", raw)
	}
}

func TestApplianceTools_Uptime(t *testing.T) {
	srv := applianceFixture(t)
	defer srv.Close()
	c := newApplianceFixtureClient(t, srv)
	r := NewRegistry(context.Background(), c, RegistryOptions{})

	raw, err := r.CallTool("vmware_appliance_uptime", map[string]interface{}{})
	if err != nil {
		t.Fatalf("vmware_appliance_uptime failed: %v", err)
	}
	if raw != "123456" {
		t.Fatalf("expected raw uptime 123456, got %s", raw)
	}
}

func TestApplianceTools_Health(t *testing.T) {
	srv := applianceFixture(t)
	defer srv.Close()
	c := newApplianceFixtureClient(t, srv)
	r := NewRegistry(context.Background(), c, RegistryOptions{})

	raw, err := r.CallTool("vmware_appliance_health", map[string]interface{}{})
	if err != nil {
		t.Fatalf("vmware_appliance_health failed: %v", err)
	}
	if raw != `"green"` {
		t.Fatalf("expected raw health \"green\", got %s", raw)
	}
}

func TestApplianceTools_HealthDetail(t *testing.T) {
	srv := applianceFixture(t)
	defer srv.Close()
	c := newApplianceFixtureClient(t, srv)
	r := NewRegistry(context.Background(), c, RegistryOptions{})

	raw, err := r.CallTool("vmware_appliance_health_detail", map[string]interface{}{})
	if err != nil {
		t.Fatalf("vmware_appliance_health_detail failed: %v", err)
	}
	m := decodeResult(t, raw)
	if len(m) != len(applianceHealthSubsystems) {
		t.Fatalf("expected %d subsystems, got %d: %s", len(applianceHealthSubsystems), len(m), raw)
	}
	for _, sub := range applianceHealthSubsystems {
		if m[sub] != "green" {
			t.Fatalf("expected subsystem %q = \"green\", got %v (%s)", sub, m[sub], raw)
		}
	}
}

// TestApplianceTools_MissingSubsystemDoesNotFailWholeCall proves
// handleApplianceHealthDetail reports a per-subsystem error inline instead
// of failing the whole aggregate — a real vCenter Appliance may not expose
// every subsystem depending on version/deployment size.
func TestApplianceTools_MissingSubsystemDoesNotFailWholeCall(t *testing.T) {
	srv := applianceFixture(t)
	defer srv.Close()

	// Overwrite one subsystem's route to 404, simulating an endpoint this
	// vCenter Appliance doesn't expose.
	mux := http.NewServeMux()
	mux.Handle("/", srv.Config.Handler)
	mux.HandleFunc("/rest/appliance/health/swap", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	srv.Config.Handler = mux

	c := newApplianceFixtureClient(t, srv)
	r := NewRegistry(context.Background(), c, RegistryOptions{})

	raw, err := r.CallTool("vmware_appliance_health_detail", map[string]interface{}{})
	if err != nil {
		t.Fatalf("expected the aggregate call to succeed despite one subsystem failing, got: %v", err)
	}
	m := decodeResult(t, raw)
	swapEntry, ok := m["swap"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected swap to be an inline error object, got %v (%s)", m["swap"], raw)
	}
	if _, ok := swapEntry["error"]; !ok {
		t.Fatalf("expected swap error entry to have an \"error\" field: %v", swapEntry)
	}
	if m["system"] != "green" {
		t.Fatalf("expected the other subsystems to still succeed, system=%v", m["system"])
	}
}

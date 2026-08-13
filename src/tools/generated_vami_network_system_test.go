package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// vamiCapture records the last request body/query string handleFuncs in
// vamiNetworkSystemFixture observed, so tests can assert on exactly what a
// PUT/POST tool sent to the server — the handlers under test summarize their
// own success in the returned JSON (e.g. {"domains": [...], "result": "set"})
// rather than echoing the server's raw response, so the fixture itself is
// the only place that can prove the wire body/query were built correctly.
// Guarded by a mutex only for `go test -race` hygiene; every test in this
// file calls r.CallTool synchronously and inspects the capture only after
// it returns, so there is no real concurrent access.
type vamiCapture struct {
	mu    sync.Mutex
	body  map[string]interface{}
	query url.Values
}

func (c *vamiCapture) setBody(b map[string]interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.body = b
}

func (c *vamiCapture) Body() map[string]interface{} {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.body
}

func (c *vamiCapture) setQuery(q url.Values) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.query = q
}

func (c *vamiCapture) Query() url.Values {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.query
}

// vamiNetworkSystemFixture starts a fake VAMI server covering this group's
// 19 routes, same shape as appliance.go's applianceFixture (session login +
// canned /appliance/... responses wrapped in the {"value": ...} envelope
// govmomi's rest.Client expects from /rest endpoints) — no simulator covers
// any route in this group either (health/lastcheck, monitoring,
// networking/dns, networking/interfaces, system/storage, system/time all
// absent from govmomi's vapi/appliance and vapi/simulator packages), so this
// is a fixture unit test, not a vcsim integration test.
func vamiNetworkSystemFixture(t *testing.T) (*httptest.Server, *vamiCapture) {
	t.Helper()
	mux := http.NewServeMux()
	capture := &vamiCapture{}

	writeValue := func(w http.ResponseWriter, v interface{}) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"value": v})
	}
	decodeBody := func(r *http.Request) map[string]interface{} {
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		return body
	}

	mux.HandleFunc("/rest/com/vmware/cis/session", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeValue(w, "fixture-session-id")
	})

	// --- Health ---
	mux.HandleFunc("/rest/appliance/health/system/lastcheck", func(w http.ResponseWriter, r *http.Request) {
		writeValue(w, "2024-01-01T00:00:00.000Z")
	})

	// --- Monitoring ---
	mux.HandleFunc("/rest/appliance/monitoring", func(w http.ResponseWriter, r *http.Request) {
		writeValue(w, []interface{}{
			map[string]interface{}{"id": "net.rx.activity.eth0", "units": "KBps"},
		})
	})
	mux.HandleFunc("/rest/appliance/monitoring/net.rx.activity.eth0", func(w http.ResponseWriter, r *http.Request) {
		writeValue(w, map[string]interface{}{"id": "net.rx.activity.eth0", "units": "KBps", "description": "Network RX activity for eth0"})
	})
	mux.HandleFunc("/rest/appliance/monitoring/query", func(w http.ResponseWriter, r *http.Request) {
		capture.setQuery(r.URL.Query())
		writeValue(w, []interface{}{
			map[string]interface{}{"name": "net.rx.activity.eth0", "data": []interface{}{}},
		})
	})

	// --- Networking: DNS domains ---
	mux.HandleFunc("/rest/appliance/networking/dns/domains", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeValue(w, []interface{}{"vmware.com"})
		case http.MethodPut, http.MethodPost:
			capture.setBody(decodeBody(r))
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	// --- Networking: DNS hostname ---
	mux.HandleFunc("/rest/appliance/networking/dns/hostname", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeValue(w, "vcsa.example.com")
		case http.MethodPut:
			capture.setBody(decodeBody(r))
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/rest/appliance/networking/dns/hostname/test", func(w http.ResponseWriter, r *http.Request) {
		capture.setBody(decodeBody(r))
		writeValue(w, []interface{}{})
	})

	// --- Networking: DNS servers ---
	mux.HandleFunc("/rest/appliance/networking/dns/servers", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeValue(w, map[string]interface{}{"mode": "is_static", "servers": []interface{}{"8.8.8.8"}})
		case http.MethodPost, http.MethodPut:
			capture.setBody(decodeBody(r))
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/rest/appliance/networking/dns/servers/test", func(w http.ResponseWriter, r *http.Request) {
		capture.setBody(decodeBody(r))
		writeValue(w, []interface{}{})
	})

	// --- Networking: interfaces ---
	mux.HandleFunc("/rest/appliance/networking/interfaces", func(w http.ResponseWriter, r *http.Request) {
		writeValue(w, []interface{}{map[string]interface{}{"name": "nic0", "status": "up"}})
	})
	mux.HandleFunc("/rest/appliance/networking/interfaces/nic0", func(w http.ResponseWriter, r *http.Request) {
		writeValue(w, map[string]interface{}{"name": "nic0", "status": "up", "mac": "00:11:22:33:44:55"})
	})

	// --- System ---
	mux.HandleFunc("/rest/appliance/system/storage", func(w http.ResponseWriter, r *http.Request) {
		writeValue(w, []interface{}{map[string]interface{}{"disk": "/", "partition": "root"}})
	})
	mux.HandleFunc("/rest/appliance/system/storage/resize", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/rest/appliance/system/time", func(w http.ResponseWriter, r *http.Request) {
		writeValue(w, map[string]interface{}{"timezone": "UTC", "date": "2024-01-01"})
	})

	return httptest.NewServer(mux), capture
}

// newVAMINetworkSystemRegistry builds a Registry the normal way and then
// manually layers registerVAMINetworkSystemTools on top via withClass — same
// pattern as newApplianceSmallRegistry (generated_appliance_small_test.go).
// This file must not edit registry.go itself (per the group brief).
func newVAMINetworkSystemRegistry(ctx context.Context, c *vmware.Client, opts RegistryOptions) *Registry {
	r := NewRegistry(ctx, c, opts)
	r.withClass(modeVCenterOnly, registerVAMINetworkSystemTools)
	return r
}

// TestVAMINetworkSystem_Registration proves all 19 tools are registered and
// reachable via ListTools.
func TestVAMINetworkSystem_Registration(t *testing.T) {
	srv, _ := vamiNetworkSystemFixture(t)
	defer srv.Close()
	c := newApplianceFixtureClient(t, srv)
	r := newVAMINetworkSystemRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	want := []string{
		"vmware_appliance_health_lastcheck",
		"vmware_appliance_monitoring_list",
		"vmware_appliance_monitoring_item",
		"vmware_appliance_monitoring_query",
		"vmware_appliance_network_dns_domains_list",
		"vmware_appliance_network_dns_domains_set",
		"vmware_appliance_network_dns_domains_add",
		"vmware_appliance_network_dns_hostname",
		"vmware_appliance_network_dns_hostname_test",
		"vmware_appliance_network_dns_hostname_set",
		"vmware_appliance_network_dns_servers_list",
		"vmware_appliance_network_dns_servers_add",
		"vmware_appliance_network_dns_servers_set",
		"vmware_appliance_network_dns_servers_test",
		"vmware_appliance_network_interfaces_list",
		"vmware_appliance_network_interface_details",
		"vmware_appliance_system_storage",
		"vmware_appliance_system_storage_resize",
		"vmware_appliance_system_time",
	}
	if len(want) != 19 {
		t.Fatalf("test bug: want list has %d entries, expected 19", len(want))
	}

	got := map[string]bool{}
	for _, tl := range r.ListTools() {
		got[tl.Name] = true
	}
	for _, name := range want {
		if !got[name] {
			t.Errorf("tool %s not registered", name)
		}
	}
}

// --- Health ------------------------------------------------------------------

func TestVAMINetworkSystem_HealthLastcheck(t *testing.T) {
	srv, _ := vamiNetworkSystemFixture(t)
	defer srv.Close()
	c := newApplianceFixtureClient(t, srv)
	r := newVAMINetworkSystemRegistry(context.Background(), c, RegistryOptions{})

	raw, err := r.CallTool("vmware_appliance_health_lastcheck", map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if raw != `"2024-01-01T00:00:00.000Z"` {
		t.Fatalf("unexpected result: %s", raw)
	}
}

// --- Monitoring ---------------------------------------------------------------

func TestVAMINetworkSystem_MonitoringList(t *testing.T) {
	srv, _ := vamiNetworkSystemFixture(t)
	defer srv.Close()
	c := newApplianceFixtureClient(t, srv)
	r := newVAMINetworkSystemRegistry(context.Background(), c, RegistryOptions{})

	raw, err := r.CallTool("vmware_appliance_monitoring_list", map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var list []interface{}
	if err := json.Unmarshal([]byte(raw), &list); err != nil {
		t.Fatalf("failed to decode list: %v (%s)", err, raw)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 monitored item, got %d: %s", len(list), raw)
	}
}

func TestVAMINetworkSystem_MonitoringItem(t *testing.T) {
	srv, _ := vamiNetworkSystemFixture(t)
	defer srv.Close()
	c := newApplianceFixtureClient(t, srv)
	r := newVAMINetworkSystemRegistry(context.Background(), c, RegistryOptions{})

	raw, err := r.CallTool("vmware_appliance_monitoring_item", map[string]interface{}{"item_id": "net.rx.activity.eth0"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := decodeResult(t, raw)
	if m["id"] != "net.rx.activity.eth0" {
		t.Fatalf("unexpected result: %s", raw)
	}

	if _, err := r.CallTool("vmware_appliance_monitoring_item", map[string]interface{}{}); err == nil {
		t.Fatal("expected an error when item_id is missing")
	}
}

func TestVAMINetworkSystem_MonitoringQuery(t *testing.T) {
	srv, capture := vamiNetworkSystemFixture(t)
	defer srv.Close()
	c := newApplianceFixtureClient(t, srv)
	r := newVAMINetworkSystemRegistry(context.Background(), c, RegistryOptions{})

	raw, err := r.CallTool("vmware_appliance_monitoring_query", map[string]interface{}{
		"names":      []interface{}{"net.rx.activity.eth0", "net.tx.activity.eth0"},
		"interval":   "HOURS2",
		"start_time": "2017-02-06T22:13:05.651Z",
		"end_time":   "2017-02-10T22:13:05.651Z",
		"function":   "COUNT",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var list []interface{}
	if err := json.Unmarshal([]byte(raw), &list); err != nil {
		t.Fatalf("failed to decode result: %v (%s)", err, raw)
	}

	q := capture.Query()
	if q.Get("item.names.1") != "net.rx.activity.eth0" || q.Get("item.names.2") != "net.tx.activity.eth0" {
		t.Fatalf("expected indexed item.names.N query params, got %v", q)
	}
	if q.Get("item.interval") != "HOURS2" || q.Get("item.function") != "COUNT" {
		t.Fatalf("expected item.interval/item.function query params, got %v", q)
	}
	if q.Get("item.start_time") != "2017-02-06T22:13:05.651Z" || q.Get("item.end_time") != "2017-02-10T22:13:05.651Z" {
		t.Fatalf("expected item.start_time/item.end_time query params, got %v", q)
	}

	if _, err := r.CallTool("vmware_appliance_monitoring_query", map[string]interface{}{}); err == nil {
		t.Fatal("expected an error when names is missing")
	}
	if _, err := r.CallTool("vmware_appliance_monitoring_query", map[string]interface{}{"names": []interface{}{}}); err == nil {
		t.Fatal("expected an error when names is empty")
	}
}

// --- Networking: DNS domains ---------------------------------------------------

func TestVAMINetworkSystem_DNSDomainsList(t *testing.T) {
	srv, _ := vamiNetworkSystemFixture(t)
	defer srv.Close()
	c := newApplianceFixtureClient(t, srv)
	r := newVAMINetworkSystemRegistry(context.Background(), c, RegistryOptions{})

	raw, err := r.CallTool("vmware_appliance_network_dns_domains_list", map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var list []interface{}
	if err := json.Unmarshal([]byte(raw), &list); err != nil || len(list) != 1 || list[0] != "vmware.com" {
		t.Fatalf("unexpected result: %s (err=%v)", raw, err)
	}
}

func TestVAMINetworkSystem_DNSDomainsSet(t *testing.T) {
	srv, capture := vamiNetworkSystemFixture(t)
	defer srv.Close()
	c := newApplianceFixtureClient(t, srv)

	closedGate := newVAMINetworkSystemRegistry(context.Background(), c, RegistryOptions{AllowDestructive: false})
	if _, err := closedGate.CallTool("vmware_appliance_network_dns_domains_set", map[string]interface{}{
		"domains": []interface{}{"vmware.com"}, "confirm": true,
	}); err == nil {
		t.Fatal("expected the gate-closed call to be denied")
	}

	openGate := newVAMINetworkSystemRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	if _, err := openGate.CallTool("vmware_appliance_network_dns_domains_set", map[string]interface{}{
		"domains": []interface{}{"vmware.com"},
	}); err == nil {
		t.Fatal("expected failure without confirm:true")
	}
	if _, err := openGate.CallTool("vmware_appliance_network_dns_domains_set", map[string]interface{}{"confirm": true}); err == nil {
		t.Fatal("expected an error when domains is missing")
	}

	raw, err := openGate.CallTool("vmware_appliance_network_dns_domains_set", map[string]interface{}{
		"domains": []interface{}{"vmware.com", "myvmware.com"}, "confirm": true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := decodeResult(t, raw)
	if m["result"] != "set" {
		t.Fatalf("unexpected result: %s", raw)
	}

	body := capture.Body()
	domains, _ := body["domains"].([]interface{})
	if len(domains) != 2 || domains[0] != "vmware.com" || domains[1] != "myvmware.com" {
		t.Fatalf("expected the request body to carry the domains list, got %v", body)
	}
}

func TestVAMINetworkSystem_DNSDomainsAdd(t *testing.T) {
	srv, capture := vamiNetworkSystemFixture(t)
	defer srv.Close()
	c := newApplianceFixtureClient(t, srv)

	closedGate := newVAMINetworkSystemRegistry(context.Background(), c, RegistryOptions{AllowDestructive: false})
	if _, err := closedGate.CallTool("vmware_appliance_network_dns_domains_add", map[string]interface{}{
		"domain": "myvmware.com", "confirm": true,
	}); err == nil {
		t.Fatal("expected the gate-closed call to be denied")
	}

	openGate := newVAMINetworkSystemRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	if _, err := openGate.CallTool("vmware_appliance_network_dns_domains_add", map[string]interface{}{"domain": "myvmware.com"}); err == nil {
		t.Fatal("expected failure without confirm:true")
	}
	if _, err := openGate.CallTool("vmware_appliance_network_dns_domains_add", map[string]interface{}{"confirm": true}); err == nil {
		t.Fatal("expected an error when domain is missing")
	}

	raw, err := openGate.CallTool("vmware_appliance_network_dns_domains_add", map[string]interface{}{
		"domain": "myvmware.com", "confirm": true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := decodeResult(t, raw)
	if m["result"] != "added" || m["domain"] != "myvmware.com" {
		t.Fatalf("unexpected result: %s", raw)
	}
	if capture.Body()["domain"] != "myvmware.com" {
		t.Fatalf("expected the request body to carry the domain, got %v", capture.Body())
	}
}

// --- Networking: DNS hostname ---------------------------------------------------

func TestVAMINetworkSystem_DNSHostname(t *testing.T) {
	srv, _ := vamiNetworkSystemFixture(t)
	defer srv.Close()
	c := newApplianceFixtureClient(t, srv)
	r := newVAMINetworkSystemRegistry(context.Background(), c, RegistryOptions{})

	raw, err := r.CallTool("vmware_appliance_network_dns_hostname", map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if raw != `"vcsa.example.com"` {
		t.Fatalf("unexpected result: %s", raw)
	}
}

func TestVAMINetworkSystem_DNSHostnameTest(t *testing.T) {
	srv, capture := vamiNetworkSystemFixture(t)
	defer srv.Close()
	c := newApplianceFixtureClient(t, srv)
	r := newVAMINetworkSystemRegistry(context.Background(), c, RegistryOptions{})

	// Read-only: must NOT be gated even with AllowDestructive:false.
	raw, err := r.CallTool("vmware_appliance_network_dns_hostname_test", map[string]interface{}{"name": "vcsa.example.com"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var list []interface{}
	if err := json.Unmarshal([]byte(raw), &list); err != nil {
		t.Fatalf("failed to decode result: %v (%s)", err, raw)
	}
	if capture.Body()["name"] != "vcsa.example.com" {
		t.Fatalf("expected the request body to carry the candidate name, got %v", capture.Body())
	}

	if _, err := r.CallTool("vmware_appliance_network_dns_hostname_test", map[string]interface{}{}); err == nil {
		t.Fatal("expected an error when name is missing")
	}
}

func TestVAMINetworkSystem_DNSHostnameSet(t *testing.T) {
	srv, capture := vamiNetworkSystemFixture(t)
	defer srv.Close()
	c := newApplianceFixtureClient(t, srv)

	closedGate := newVAMINetworkSystemRegistry(context.Background(), c, RegistryOptions{AllowDestructive: false})
	if _, err := closedGate.CallTool("vmware_appliance_network_dns_hostname_set", map[string]interface{}{
		"name": "vcsa2.example.com", "confirm": true,
	}); err == nil {
		t.Fatal("expected the gate-closed call to be denied")
	}

	openGate := newVAMINetworkSystemRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	if _, err := openGate.CallTool("vmware_appliance_network_dns_hostname_set", map[string]interface{}{"name": "vcsa2.example.com"}); err == nil {
		t.Fatal("expected failure without confirm:true")
	}
	if _, err := openGate.CallTool("vmware_appliance_network_dns_hostname_set", map[string]interface{}{"confirm": true}); err == nil {
		t.Fatal("expected an error when name is missing")
	}

	raw, err := openGate.CallTool("vmware_appliance_network_dns_hostname_set", map[string]interface{}{
		"name": "vcsa2.example.com", "confirm": true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := decodeResult(t, raw)
	if m["result"] != "set" || m["name"] != "vcsa2.example.com" {
		t.Fatalf("unexpected result: %s", raw)
	}
	if capture.Body()["name"] != "vcsa2.example.com" {
		t.Fatalf("expected the request body to carry the new name, got %v", capture.Body())
	}
}

// --- Networking: DNS servers ------------------------------------------------

func TestVAMINetworkSystem_DNSServersList(t *testing.T) {
	srv, _ := vamiNetworkSystemFixture(t)
	defer srv.Close()
	c := newApplianceFixtureClient(t, srv)
	r := newVAMINetworkSystemRegistry(context.Background(), c, RegistryOptions{})

	raw, err := r.CallTool("vmware_appliance_network_dns_servers_list", map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := decodeResult(t, raw)
	if m["mode"] != "is_static" {
		t.Fatalf("unexpected result: %s", raw)
	}
}

func TestVAMINetworkSystem_DNSServersAdd(t *testing.T) {
	srv, capture := vamiNetworkSystemFixture(t)
	defer srv.Close()
	c := newApplianceFixtureClient(t, srv)

	closedGate := newVAMINetworkSystemRegistry(context.Background(), c, RegistryOptions{AllowDestructive: false})
	if _, err := closedGate.CallTool("vmware_appliance_network_dns_servers_add", map[string]interface{}{
		"server": "9.9.9.9", "confirm": true,
	}); err == nil {
		t.Fatal("expected the gate-closed call to be denied")
	}

	openGate := newVAMINetworkSystemRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	if _, err := openGate.CallTool("vmware_appliance_network_dns_servers_add", map[string]interface{}{"server": "9.9.9.9"}); err == nil {
		t.Fatal("expected failure without confirm:true")
	}
	if _, err := openGate.CallTool("vmware_appliance_network_dns_servers_add", map[string]interface{}{"confirm": true}); err == nil {
		t.Fatal("expected an error when server is missing")
	}

	raw, err := openGate.CallTool("vmware_appliance_network_dns_servers_add", map[string]interface{}{
		"server": "9.9.9.9", "confirm": true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := decodeResult(t, raw)
	if m["result"] != "added" || m["server"] != "9.9.9.9" {
		t.Fatalf("unexpected result: %s", raw)
	}
	if capture.Body()["server"] != "9.9.9.9" {
		t.Fatalf("expected the request body to carry the server, got %v", capture.Body())
	}
}

func TestVAMINetworkSystem_DNSServersSet(t *testing.T) {
	srv, capture := vamiNetworkSystemFixture(t)
	defer srv.Close()
	c := newApplianceFixtureClient(t, srv)

	closedGate := newVAMINetworkSystemRegistry(context.Background(), c, RegistryOptions{AllowDestructive: false})
	if _, err := closedGate.CallTool("vmware_appliance_network_dns_servers_set", map[string]interface{}{
		"mode": "is_static", "servers": []interface{}{"1.1.1.1"}, "confirm": true,
	}); err == nil {
		t.Fatal("expected the gate-closed call to be denied")
	}

	openGate := newVAMINetworkSystemRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	if _, err := openGate.CallTool("vmware_appliance_network_dns_servers_set", map[string]interface{}{
		"mode": "is_static", "servers": []interface{}{"1.1.1.1"},
	}); err == nil {
		t.Fatal("expected failure without confirm:true")
	}
	if _, err := openGate.CallTool("vmware_appliance_network_dns_servers_set", map[string]interface{}{"confirm": true}); err == nil {
		t.Fatal("expected an error when mode is missing")
	}

	raw, err := openGate.CallTool("vmware_appliance_network_dns_servers_set", map[string]interface{}{
		"mode": "is_static", "servers": []interface{}{"1.1.1.1", "8.8.8.8"}, "confirm": true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := decodeResult(t, raw)
	if m["result"] != "set" || m["mode"] != "is_static" {
		t.Fatalf("unexpected result: %s", raw)
	}

	body := capture.Body()
	cfg, ok := body["config"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected the request body to carry a nested config object, got %v", body)
	}
	if cfg["mode"] != "is_static" {
		t.Fatalf("expected config.mode = is_static, got %v", cfg["mode"])
	}
	servers, _ := cfg["servers"].([]interface{})
	if len(servers) != 2 || servers[0] != "1.1.1.1" || servers[1] != "8.8.8.8" {
		t.Fatalf("expected config.servers to carry both servers, got %v", cfg["servers"])
	}

	// mode-only (dhcp), no servers supplied — must default to an empty list,
	// not omit the field or fail.
	raw, err = openGate.CallTool("vmware_appliance_network_dns_servers_set", map[string]interface{}{
		"mode": "dhcp", "confirm": true,
	})
	if err != nil {
		t.Fatalf("unexpected error for mode-only dhcp call: %v", err)
	}
	body = capture.Body()
	cfg, _ = body["config"].(map[string]interface{})
	if cfg["mode"] != "dhcp" {
		t.Fatalf("expected config.mode = dhcp, got %v", cfg["mode"])
	}
	if servers, ok := cfg["servers"].([]interface{}); !ok || len(servers) != 0 {
		t.Fatalf("expected config.servers to be an empty array for dhcp, got %v (%s)", cfg["servers"], raw)
	}
}

func TestVAMINetworkSystem_DNSServersTest(t *testing.T) {
	srv, capture := vamiNetworkSystemFixture(t)
	defer srv.Close()
	c := newApplianceFixtureClient(t, srv)
	r := newVAMINetworkSystemRegistry(context.Background(), c, RegistryOptions{})

	// Read-only: must NOT be gated even with AllowDestructive:false.
	raw, err := r.CallTool("vmware_appliance_network_dns_servers_test", map[string]interface{}{
		"servers": []interface{}{"9.9.9.9"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var list []interface{}
	if err := json.Unmarshal([]byte(raw), &list); err != nil {
		t.Fatalf("failed to decode result: %v (%s)", err, raw)
	}
	servers, _ := capture.Body()["servers"].([]interface{})
	if len(servers) != 1 || servers[0] != "9.9.9.9" {
		t.Fatalf("expected the request body to carry the candidate servers, got %v", capture.Body())
	}

	if _, err := r.CallTool("vmware_appliance_network_dns_servers_test", map[string]interface{}{}); err == nil {
		t.Fatal("expected an error when servers is missing")
	}
}

// --- Networking: interfaces ------------------------------------------------

func TestVAMINetworkSystem_InterfacesList(t *testing.T) {
	srv, _ := vamiNetworkSystemFixture(t)
	defer srv.Close()
	c := newApplianceFixtureClient(t, srv)
	r := newVAMINetworkSystemRegistry(context.Background(), c, RegistryOptions{})

	raw, err := r.CallTool("vmware_appliance_network_interfaces_list", map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var list []interface{}
	if err := json.Unmarshal([]byte(raw), &list); err != nil || len(list) != 1 {
		t.Fatalf("unexpected result: %s (err=%v)", raw, err)
	}
}

func TestVAMINetworkSystem_InterfaceDetails(t *testing.T) {
	srv, _ := vamiNetworkSystemFixture(t)
	defer srv.Close()
	c := newApplianceFixtureClient(t, srv)
	r := newVAMINetworkSystemRegistry(context.Background(), c, RegistryOptions{})

	raw, err := r.CallTool("vmware_appliance_network_interface_details", map[string]interface{}{"interface_id": "nic0"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := decodeResult(t, raw)
	if m["name"] != "nic0" {
		t.Fatalf("unexpected result: %s", raw)
	}

	if _, err := r.CallTool("vmware_appliance_network_interface_details", map[string]interface{}{}); err == nil {
		t.Fatal("expected an error when interface_id is missing")
	}
}

// --- System ------------------------------------------------------------------

func TestVAMINetworkSystem_SystemStorage(t *testing.T) {
	srv, _ := vamiNetworkSystemFixture(t)
	defer srv.Close()
	c := newApplianceFixtureClient(t, srv)
	r := newVAMINetworkSystemRegistry(context.Background(), c, RegistryOptions{})

	raw, err := r.CallTool("vmware_appliance_system_storage", map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var list []interface{}
	if err := json.Unmarshal([]byte(raw), &list); err != nil || len(list) != 1 {
		t.Fatalf("unexpected result: %s (err=%v)", raw, err)
	}
}

func TestVAMINetworkSystem_SystemStorageResize(t *testing.T) {
	srv, _ := vamiNetworkSystemFixture(t)
	defer srv.Close()
	c := newApplianceFixtureClient(t, srv)

	closedGate := newVAMINetworkSystemRegistry(context.Background(), c, RegistryOptions{AllowDestructive: false})
	if _, err := closedGate.CallTool("vmware_appliance_system_storage_resize", map[string]interface{}{"confirm": true}); err == nil {
		t.Fatal("expected the gate-closed call to be denied")
	}

	openGate := newVAMINetworkSystemRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	if _, err := openGate.CallTool("vmware_appliance_system_storage_resize", map[string]interface{}{}); err == nil {
		t.Fatal("expected failure without confirm:true")
	}

	raw, err := openGate.CallTool("vmware_appliance_system_storage_resize", map[string]interface{}{"confirm": true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := decodeResult(t, raw)
	if m["result"] != "resize_triggered" {
		t.Fatalf("unexpected result: %s", raw)
	}
}

func TestVAMINetworkSystem_SystemTime(t *testing.T) {
	srv, _ := vamiNetworkSystemFixture(t)
	defer srv.Close()
	c := newApplianceFixtureClient(t, srv)
	r := newVAMINetworkSystemRegistry(context.Background(), c, RegistryOptions{})

	raw, err := r.CallTool("vmware_appliance_system_time", map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := decodeResult(t, raw)
	if m["timezone"] != "UTC" {
		t.Fatalf("unexpected result: %s", raw)
	}
}

package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// techpreviewFixture starts a fake VAMI server for the rest/appliance/
// techpreview/... routes registered by generated_vami_techpreview_network.go
// — same rationale as tools/appliance_test.go's applianceFixture (see that
// file's doc comment and generated_vami_techpreview_network.go's top doc
// comment "Testing: httptest fixture, not vcsim"): vmware.Client.REST(ctx)
// never touches SOAP/ServiceContent, so these VAMI REST tools are fully
// testable against a bare httptest.Server, independent of vcsim.
//
// Go 1.22+ method-prefixed ServeMux patterns ("GET /path", "POST /path", ...)
// are used throughout because several techpreview routes share the same URL
// path across different HTTP methods (e.g. GET vs POST "/networking/ipv4"
// for get vs set) — this project's go.mod pins go 1.25.0, well past the
// method-routing cutover.
//
// Every mutating (POST/PUT) route is wired to echoBody instead of a fixed
// canned response: it decodes the incoming JSON body generically and
// reflects it back as {"received": <body>}, wrapped in the same
// {"value": ...} envelope every /rest response uses. This lets
// TestVAMITechpreviewNetworkTools_MutationsForwardExpectedBody assert the
// exact body shape each handler constructed (e.g. {"rule": {...}} vs
// {"config": {...}}) — not just that some 200 came back — which matters
// here specifically because every handler in the source file hand-builds
// its request body from named top-level args (no typed struct exists to
// lean on for this API).
func techpreviewFixture(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	writeValue := func(w http.ResponseWriter, v interface{}) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"value": v})
	}

	echoBody := func(w http.ResponseWriter, r *http.Request) {
		var body interface{}
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&body)
		}
		writeValue(w, map[string]interface{}{"received": body})
	}

	mux.HandleFunc("POST /rest/com/vmware/cis/session", func(w http.ResponseWriter, r *http.Request) {
		writeValue(w, "fixture-session-id")
	})

	// --- Firewall ---------------------------------------------------------
	mux.HandleFunc("GET /rest/appliance/techpreview/networking/firewall/addr/inbound", func(w http.ResponseWriter, r *http.Request) {
		writeValue(w, []interface{}{
			map[string]interface{}{"address": "10.0.0.0", "prefix": float64(8), "interface_name": "*", "policy": "allow"},
		})
	})
	mux.HandleFunc("POST /rest/appliance/techpreview/networking/firewall/addr/inbound", echoBody)
	mux.HandleFunc("POST /rest/appliance/techpreview/networking/firewall/addr/inbound/delete", echoBody)
	mux.HandleFunc("PUT /rest/appliance/techpreview/networking/firewall/addr/inbound", echoBody)

	// --- IPv4 ---------------------------------------------------------------
	mux.HandleFunc("GET /rest/appliance/techpreview/networking/ipv4", func(w http.ResponseWriter, r *http.Request) {
		writeValue(w, []interface{}{map[string]interface{}{"interface_name": "nic0", "mode": "dhcp"}})
	})
	mux.HandleFunc("POST /rest/appliance/techpreview/networking/ipv4/get", echoBody)
	mux.HandleFunc("POST /rest/appliance/techpreview/networking/ipv4/renew", echoBody)
	mux.HandleFunc("POST /rest/appliance/techpreview/networking/ipv4", echoBody)

	// --- IPv6 ---------------------------------------------------------------
	mux.HandleFunc("GET /rest/appliance/techpreview/networking/ipv6", func(w http.ResponseWriter, r *http.Request) {
		writeValue(w, []interface{}{map[string]interface{}{"interface_name": "nic0", "dhcp": true}})
	})
	mux.HandleFunc("POST /rest/appliance/techpreview/networking/ipv6/get", echoBody)
	mux.HandleFunc("POST /rest/appliance/techpreview/networking/ipv6", echoBody)

	// --- NTP ------------------------------------------------------------------
	mux.HandleFunc("GET /rest/appliance/techpreview/ntp", func(w http.ResponseWriter, r *http.Request) {
		writeValue(w, map[string]interface{}{"servers": []interface{}{"time.example.com"}})
	})
	mux.HandleFunc("POST /rest/appliance/techpreview/ntp/test", echoBody)
	mux.HandleFunc("POST /rest/appliance/techpreview/ntp/server", echoBody)
	mux.HandleFunc("POST /rest/appliance/techpreview/ntp/server/delete", echoBody)
	mux.HandleFunc("PUT /rest/appliance/techpreview/ntp/server", echoBody)

	// --- Proxy ----------------------------------------------------------------
	mux.HandleFunc("GET /rest/appliance/techpreview/networking/proxy", func(w http.ResponseWriter, r *http.Request) {
		writeValue(w, map[string]interface{}{"configlist": []interface{}{}, "status": "disabled"})
	})
	mux.HandleFunc("PUT /rest/appliance/techpreview/networking/proxy", echoBody)
	mux.HandleFunc("POST /rest/appliance/techpreview/networking/proxy/test", echoBody)
	mux.HandleFunc("POST /rest/appliance/techpreview/networking/proxy/delete", echoBody)

	// --- Routes ---------------------------------------------------------------
	mux.HandleFunc("GET /rest/appliance/techpreview/networking/routes", func(w http.ResponseWriter, r *http.Request) {
		writeValue(w, []interface{}{
			map[string]interface{}{"destination": "0.0.0.0", "prefix": float64(0), "gateway": "10.0.0.1", "interface_name": "nic0"},
		})
	})
	mux.HandleFunc("PUT /rest/appliance/techpreview/networking/routes", echoBody)
	mux.HandleFunc("POST /rest/appliance/techpreview/networking/routes", echoBody)
	mux.HandleFunc("POST /rest/appliance/techpreview/networking/routes/test", echoBody)
	mux.HandleFunc("POST /rest/appliance/techpreview/networking/routes/delete", echoBody)

	// --- Timesync -------------------------------------------------------------
	mux.HandleFunc("GET /rest/appliance/techpreview/timesync", func(w http.ResponseWriter, r *http.Request) {
		writeValue(w, map[string]interface{}{"mode": "host"})
	})
	mux.HandleFunc("PUT /rest/appliance/techpreview/timesync", echoBody)

	return httptest.NewServer(mux)
}

// newTechpreviewNetworkRegistry builds a Registry the normal way (NewRegistry,
// which wires every other domain via registerTools) and then manually layers
// registerVAMITechpreviewNetworkTools on top via withClass — same pattern as
// newApplianceSmallRegistry (generated_appliance_small_test.go). Required
// here because this group's brief explicitly forbids editing registry.go, so
// registerVAMITechpreviewNetworkTools is not called from registerTools.
func newTechpreviewNetworkRegistry(ctx context.Context, c *vmware.Client, opts RegistryOptions) *Registry {
	r := NewRegistry(ctx, c, opts)
	r.withClass(modeVCenterOnly, registerVAMITechpreviewNetworkTools)
	return r
}

// techpreviewReceivedBody unwraps the {"received": ...} envelope echoBody
// (techpreviewFixture) wraps every mutating request's decoded body in.
func techpreviewReceivedBody(t *testing.T, raw string) interface{} {
	t.Helper()
	var wrapper struct {
		Received interface{} `json:"received"`
	}
	if err := json.Unmarshal([]byte(raw), &wrapper); err != nil {
		t.Fatalf("failed to decode echoed body %q: %v", raw, err)
	}
	return wrapper.Received
}

// TestVAMITechpreviewNetworkTools_Registration proves all 27 tools are
// registered and reachable via ListTools.
func TestVAMITechpreviewNetworkTools_Registration(t *testing.T) {
	srv := techpreviewFixture(t)
	defer srv.Close()
	c := newApplianceFixtureClient(t, srv)
	r := newTechpreviewNetworkRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	want := []string{
		"vmware_appliance_techpreview_firewall_list",
		"vmware_appliance_techpreview_firewall_create",
		"vmware_appliance_techpreview_firewall_delete",
		"vmware_appliance_techpreview_firewall_replace",
		"vmware_appliance_techpreview_ipv4_get",
		"vmware_appliance_techpreview_ipv4_details",
		"vmware_appliance_techpreview_ipv4_renew",
		"vmware_appliance_techpreview_ipv4_set",
		"vmware_appliance_techpreview_ipv6_get",
		"vmware_appliance_techpreview_ipv6_details",
		"vmware_appliance_techpreview_ipv6_set",
		"vmware_appliance_techpreview_ntp_get",
		"vmware_appliance_techpreview_ntp_test",
		"vmware_appliance_techpreview_ntp_server_add",
		"vmware_appliance_techpreview_ntp_server_delete",
		"vmware_appliance_techpreview_ntp_server_set",
		"vmware_appliance_techpreview_proxy_get",
		"vmware_appliance_techpreview_proxy_set",
		"vmware_appliance_techpreview_proxy_test",
		"vmware_appliance_techpreview_proxy_delete",
		"vmware_appliance_techpreview_routes_list",
		"vmware_appliance_techpreview_routes_set",
		"vmware_appliance_techpreview_routes_add",
		"vmware_appliance_techpreview_routes_test",
		"vmware_appliance_techpreview_routes_delete",
		"vmware_appliance_techpreview_timesync_get",
		"vmware_appliance_techpreview_timesync_set",
	}
	if len(want) != 27 {
		t.Fatalf("test bug: want list has %d entries, expected 27", len(want))
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

// TestVAMITechpreviewNetworkTools_Getters proves every read-only GET tool
// decodes the fixture's canned response correctly.
func TestVAMITechpreviewNetworkTools_Getters(t *testing.T) {
	srv := techpreviewFixture(t)
	defer srv.Close()
	c := newApplianceFixtureClient(t, srv)
	r := newTechpreviewNetworkRegistry(context.Background(), c, RegistryOptions{})

	cases := []struct {
		tool  string
		check func(t *testing.T, raw string)
	}{
		{"vmware_appliance_techpreview_firewall_list", func(t *testing.T, raw string) {
			var rules []map[string]interface{}
			if err := json.Unmarshal([]byte(raw), &rules); err != nil {
				t.Fatalf("decode failed: %v (%s)", err, raw)
			}
			if len(rules) != 1 || rules[0]["policy"] != "allow" {
				t.Fatalf("unexpected firewall rules: %s", raw)
			}
		}},
		{"vmware_appliance_techpreview_ipv4_get", func(t *testing.T, raw string) {
			var cfg []map[string]interface{}
			if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
				t.Fatalf("decode failed: %v (%s)", err, raw)
			}
			if len(cfg) != 1 || cfg[0]["mode"] != "dhcp" {
				t.Fatalf("unexpected ipv4 config: %s", raw)
			}
		}},
		{"vmware_appliance_techpreview_ipv6_get", func(t *testing.T, raw string) {
			var cfg []map[string]interface{}
			if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
				t.Fatalf("decode failed: %v (%s)", err, raw)
			}
			if len(cfg) != 1 || cfg[0]["dhcp"] != true {
				t.Fatalf("unexpected ipv6 config: %s", raw)
			}
		}},
		{"vmware_appliance_techpreview_ntp_get", func(t *testing.T, raw string) {
			m := decodeResult(t, raw)
			servers, ok := m["servers"].([]interface{})
			if !ok || len(servers) != 1 || servers[0] != "time.example.com" {
				t.Fatalf("unexpected ntp config: %s", raw)
			}
		}},
		{"vmware_appliance_techpreview_proxy_get", func(t *testing.T, raw string) {
			m := decodeResult(t, raw)
			if m["status"] != "disabled" {
				t.Fatalf("unexpected proxy config: %s", raw)
			}
		}},
		{"vmware_appliance_techpreview_routes_list", func(t *testing.T, raw string) {
			var routes []map[string]interface{}
			if err := json.Unmarshal([]byte(raw), &routes); err != nil {
				t.Fatalf("decode failed: %v (%s)", err, raw)
			}
			if len(routes) != 1 || routes[0]["gateway"] != "10.0.0.1" {
				t.Fatalf("unexpected routes: %s", raw)
			}
		}},
		{"vmware_appliance_techpreview_timesync_get", func(t *testing.T, raw string) {
			m := decodeResult(t, raw)
			if m["mode"] != "host" {
				t.Fatalf("unexpected timesync config: %s", raw)
			}
		}},
	}

	for _, tc := range cases {
		t.Run(tc.tool, func(t *testing.T) {
			raw, err := r.CallTool(tc.tool, map[string]interface{}{})
			if err != nil {
				t.Fatalf("%s failed: %v", tc.tool, err)
			}
			tc.check(t, raw)
		})
	}
}

// TestVAMITechpreviewNetworkTools_MutationsForwardExpectedBody proves every
// POST/PUT tool builds the exact VAMI request body shape confirmed from the
// Postman collection (generated_vami_techpreview_network.go's top doc
// comment) — not a repacked/renamed shape — by round-tripping through
// techpreviewFixture's echoBody and comparing the decoded body against the
// expected shape with reflect.DeepEqual. Covers all 20 mutating/testing
// tools; the other 7 (pure GET) are covered by
// TestVAMITechpreviewNetworkTools_Getters.
func TestVAMITechpreviewNetworkTools_MutationsForwardExpectedBody(t *testing.T) {
	srv := techpreviewFixture(t)
	defer srv.Close()
	c := newApplianceFixtureClient(t, srv)
	r := newTechpreviewNetworkRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	cases := []struct {
		tool string
		args map[string]interface{}
		want map[string]interface{}
	}{
		{
			tool: "vmware_appliance_techpreview_firewall_create",
			args: map[string]interface{}{
				"pos":     float64(1),
				"rule":    map[string]interface{}{"address": "10.0.0.0", "prefix": float64(8), "interface_name": "*", "policy": "allow"},
				"confirm": true,
			},
			want: map[string]interface{}{
				"pos":  float64(1),
				"rule": map[string]interface{}{"address": "10.0.0.0", "prefix": float64(8), "interface_name": "*", "policy": "allow"},
			},
		},
		{
			tool: "vmware_appliance_techpreview_firewall_delete",
			args: map[string]interface{}{
				"config":  map[string]interface{}{"all": true},
				"confirm": true,
			},
			want: map[string]interface{}{"config": map[string]interface{}{"all": true}},
		},
		{
			tool: "vmware_appliance_techpreview_firewall_replace",
			args: map[string]interface{}{
				"rules": []interface{}{
					map[string]interface{}{"address": "0.0.0.0", "prefix": float64(0), "interface_name": "*", "policy": "deny"},
				},
				"confirm": true,
			},
			want: map[string]interface{}{
				"rules": []interface{}{
					map[string]interface{}{"address": "0.0.0.0", "prefix": float64(0), "interface_name": "*", "policy": "deny"},
				},
			},
		},
		{
			tool: "vmware_appliance_techpreview_ipv4_details",
			args: map[string]interface{}{"interfaces": []interface{}{"nic0"}},
			want: map[string]interface{}{"interfaces": []interface{}{"nic0"}},
		},
		{
			tool: "vmware_appliance_techpreview_ipv4_renew",
			args: map[string]interface{}{"interfaces": []interface{}{"nic0"}, "confirm": true},
			want: map[string]interface{}{"interfaces": []interface{}{"nic0"}},
		},
		{
			tool: "vmware_appliance_techpreview_ipv4_set",
			args: map[string]interface{}{
				"config": []interface{}{
					map[string]interface{}{"interface_name": "nic0", "mode": "static", "address": "10.0.0.5", "prefix": float64(24), "default_gateway": "10.0.0.1"},
				},
				"confirm": true,
			},
			want: map[string]interface{}{
				"config": []interface{}{
					map[string]interface{}{"interface_name": "nic0", "mode": "static", "address": "10.0.0.5", "prefix": float64(24), "default_gateway": "10.0.0.1"},
				},
			},
		},
		{
			tool: "vmware_appliance_techpreview_ipv6_details",
			args: map[string]interface{}{"interfaces": []interface{}{"nic0"}},
			want: map[string]interface{}{"interfaces": []interface{}{"nic0"}},
		},
		{
			tool: "vmware_appliance_techpreview_ipv6_set",
			args: map[string]interface{}{
				"config": []interface{}{
					map[string]interface{}{"interface_name": "nic0", "dhcp": true},
				},
				"confirm": true,
			},
			want: map[string]interface{}{
				"config": []interface{}{
					map[string]interface{}{"interface_name": "nic0", "dhcp": true},
				},
			},
		},
		{
			tool: "vmware_appliance_techpreview_ntp_test",
			args: map[string]interface{}{"servers": []interface{}{"time.example.com"}},
			want: map[string]interface{}{"servers": []interface{}{"time.example.com"}},
		},
		{
			tool: "vmware_appliance_techpreview_ntp_server_add",
			args: map[string]interface{}{"servers": []interface{}{"time.example.com"}, "confirm": true},
			want: map[string]interface{}{"servers": []interface{}{"time.example.com"}},
		},
		{
			tool: "vmware_appliance_techpreview_ntp_server_delete",
			args: map[string]interface{}{"servers": []interface{}{"old.example.com"}, "confirm": true},
			want: map[string]interface{}{"servers": []interface{}{"old.example.com"}},
		},
		{
			tool: "vmware_appliance_techpreview_ntp_server_set",
			args: map[string]interface{}{"servers": []interface{}{"a.example.com", "b.example.com"}, "confirm": true},
			want: map[string]interface{}{"servers": []interface{}{"a.example.com", "b.example.com"}},
		},
		{
			tool: "vmware_appliance_techpreview_proxy_set",
			args: map[string]interface{}{
				"config": map[string]interface{}{
					"configlist": []interface{}{
						map[string]interface{}{"protocol": "ftp", "server": "proxy.example.com", "port": float64(3128)},
					},
					"status": "disabled",
				},
				"confirm": true,
			},
			want: map[string]interface{}{
				"config": map[string]interface{}{
					"configlist": []interface{}{
						map[string]interface{}{"protocol": "ftp", "server": "proxy.example.com", "port": float64(3128)},
					},
					"status": "disabled",
				},
			},
		},
		{
			tool: "vmware_appliance_techpreview_proxy_test",
			args: map[string]interface{}{
				"config": map[string]interface{}{"protocol": "ftp", "server": "proxy.example.com", "port": float64(3128), "testhost": "example.com"},
			},
			want: map[string]interface{}{
				"config": map[string]interface{}{"protocol": "ftp", "server": "proxy.example.com", "port": float64(3128), "testhost": "example.com"},
			},
		},
		{
			tool: "vmware_appliance_techpreview_proxy_delete",
			args: map[string]interface{}{"protocol": "ftp", "confirm": true},
			want: map[string]interface{}{"protocol": "ftp"},
		},
		{
			tool: "vmware_appliance_techpreview_routes_set",
			args: map[string]interface{}{
				"routes": []interface{}{
					map[string]interface{}{"destination": "0.0.0.0", "prefix": float64(0), "gateway": "10.0.0.1", "interface_name": "nic0"},
				},
				"confirm": true,
			},
			want: map[string]interface{}{
				"routes": []interface{}{
					map[string]interface{}{"destination": "0.0.0.0", "prefix": float64(0), "gateway": "10.0.0.1", "interface_name": "nic0"},
				},
			},
		},
		{
			tool: "vmware_appliance_techpreview_routes_add",
			args: map[string]interface{}{
				"route":   map[string]interface{}{"destination": "192.168.1.0", "prefix": float64(24), "gateway": "10.0.0.1", "interface_name": "nic0"},
				"confirm": true,
			},
			want: map[string]interface{}{
				"route": map[string]interface{}{"destination": "192.168.1.0", "prefix": float64(24), "gateway": "10.0.0.1", "interface_name": "nic0"},
			},
		},
		{
			tool: "vmware_appliance_techpreview_routes_test",
			args: map[string]interface{}{"gateways": []interface{}{"10.0.0.1"}},
			want: map[string]interface{}{"gateways": []interface{}{"10.0.0.1"}},
		},
		{
			tool: "vmware_appliance_techpreview_routes_delete",
			args: map[string]interface{}{
				"route":   map[string]interface{}{"destination": "192.168.1.0", "prefix": float64(24), "gateway": "10.0.0.1", "interface_name": "nic0"},
				"confirm": true,
			},
			want: map[string]interface{}{
				"route": map[string]interface{}{"destination": "192.168.1.0", "prefix": float64(24), "gateway": "10.0.0.1", "interface_name": "nic0"},
			},
		},
		{
			tool: "vmware_appliance_techpreview_timesync_set",
			args: map[string]interface{}{
				"config":  map[string]interface{}{"mode": "host"},
				"confirm": true,
			},
			want: map[string]interface{}{"config": map[string]interface{}{"mode": "host"}},
		},
	}
	if len(cases) != 20 {
		t.Fatalf("test bug: cases has %d entries, expected 20 (27 tools - 7 pure GETs)", len(cases))
	}

	for _, tc := range cases {
		t.Run(tc.tool, func(t *testing.T) {
			raw, err := r.CallTool(tc.tool, tc.args)
			if err != nil {
				t.Fatalf("%s failed: %v", tc.tool, err)
			}
			got := techpreviewReceivedBody(t, raw)
			if !reflect.DeepEqual(got, interface{}(tc.want)) {
				t.Fatalf("%s: request body mismatch\n got:  %#v\n want: %#v", tc.tool, got, tc.want)
			}
		})
	}
}

// TestVAMITechpreviewNetworkTools_ArgValidation proves each argument-shape
// helper (requireObjectArg, requireArrayArg, requiredStringArg) rejects a
// missing required argument with a clean error, before any network call —
// one representative tool per helper/shape.
func TestVAMITechpreviewNetworkTools_ArgValidation(t *testing.T) {
	srv := techpreviewFixture(t)
	defer srv.Close()
	c := newApplianceFixtureClient(t, srv)
	r := newTechpreviewNetworkRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	cases := []struct {
		tool string
		args map[string]interface{}
	}{
		{"vmware_appliance_techpreview_firewall_create", map[string]interface{}{"confirm": true}},                          // missing object arg "rule"
		{"vmware_appliance_techpreview_firewall_replace", map[string]interface{}{"confirm": true}},                         // missing array arg "rules"
		{"vmware_appliance_techpreview_ipv4_details", map[string]interface{}{}},                                            // missing array arg "interfaces"
		{"vmware_appliance_techpreview_proxy_delete", map[string]interface{}{"confirm": true}},                             // missing string arg "protocol"
		{"vmware_appliance_techpreview_routes_add", map[string]interface{}{"confirm": true}},                               // missing object arg "route"
		{"vmware_appliance_techpreview_firewall_create", map[string]interface{}{"rule": "not-an-object", "confirm": true}}, // wrong type
	}

	for _, tc := range cases {
		t.Run(tc.tool, func(t *testing.T) {
			if _, err := r.CallTool(tc.tool, tc.args); err == nil {
				t.Errorf("%s: expected an error with args %v, got success", tc.tool, tc.args)
			}
		})
	}
}

// TestVAMITechpreviewNetworkTools_GateAndConfirm proves the Tier 1 and
// Tier 2 tools in this file are wired through registerDestructive — same
// 3-layer protection check pattern as generated_appliance_small_test.go's
// TestApplianceSmallTools_GateAndConfirm. Uses one Tier 1 (routes_delete)
// and one Tier 2 (ntp_server_add) tool as representatives.
func TestVAMITechpreviewNetworkTools_GateAndConfirm(t *testing.T) {
	srv := techpreviewFixture(t)
	defer srv.Close()
	c := newApplianceFixtureClient(t, srv)

	validRoute := map[string]interface{}{"destination": "192.168.1.0", "prefix": float64(24), "gateway": "10.0.0.1", "interface_name": "nic0"}

	closedGate := newTechpreviewNetworkRegistry(context.Background(), c, RegistryOptions{AllowDestructive: false})
	if _, err := closedGate.CallTool("vmware_appliance_techpreview_routes_delete", map[string]interface{}{"route": validRoute, "confirm": true}); err == nil {
		t.Fatal("expected vmware_appliance_techpreview_routes_delete to be denied with the gate closed")
	}
	if _, err := closedGate.CallTool("vmware_appliance_techpreview_ntp_server_add", map[string]interface{}{"servers": []interface{}{"time.example.com"}, "confirm": true}); err == nil {
		t.Fatal("expected vmware_appliance_techpreview_ntp_server_add to be denied with the gate closed")
	}

	openGate := newTechpreviewNetworkRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	if _, err := openGate.CallTool("vmware_appliance_techpreview_routes_delete", map[string]interface{}{"route": validRoute}); err == nil {
		t.Fatal("expected vmware_appliance_techpreview_routes_delete to fail without confirm:true")
	}
	if _, err := openGate.CallTool("vmware_appliance_techpreview_ntp_server_add", map[string]interface{}{"servers": []interface{}{"time.example.com"}}); err == nil {
		t.Fatal("expected vmware_appliance_techpreview_ntp_server_add to fail without confirm:true")
	}

	// Sanity: with the gate open AND confirm:true, both succeed against the fixture.
	if _, err := openGate.CallTool("vmware_appliance_techpreview_routes_delete", map[string]interface{}{"route": validRoute, "confirm": true}); err != nil {
		t.Fatalf("expected vmware_appliance_techpreview_routes_delete to succeed with gate open + confirm: %v", err)
	}
	if _, err := openGate.CallTool("vmware_appliance_techpreview_ntp_server_add", map[string]interface{}{"servers": []interface{}{"time.example.com"}, "confirm": true}); err != nil {
		t.Fatalf("expected vmware_appliance_techpreview_ntp_server_add to succeed with gate open + confirm: %v", err)
	}
}

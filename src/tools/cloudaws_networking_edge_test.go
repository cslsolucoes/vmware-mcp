package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cslsoftwares/mcpvmware/cloudaws"
)

// Fixture identifiers used across every test in this file — arbitrary but
// consistent, so a handler's path-building can be asserted against a known
// value.
const (
	cloudAWSEdgeTestOrg  = "org1"
	cloudAWSEdgeTestSDDC = "sddc1"
	cloudAWSEdgeTestEdge = "edge1"
)

// cloudAWSNetworkingEdgeFixture starts a fake VMC/CSP server: the CSP
// token-exchange endpoint (so cloudaws.Client.ensureToken succeeds) plus
// canned responses for one representative route per sub-domain covered by
// cloudaws_networking_edge.go. No VMC organization/SDDC is reachable from
// this project (see cloudaws/client.go's package doc comment) and there is
// no simulator for VMC's REST surface (unlike vSphere's vcsim), so this is
// a fixture unit test — same approach already established for
// tools/appliance_test.go.
func cloudAWSNetworkingEdgeFixture(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	writeJSON := func(w http.ResponseWriter, v interface{}) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(v)
	}

	mux.HandleFunc("/csp/gateway/am/api/auth/api-tokens/authorize", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, map[string]interface{}{"access_token": "fixture-access-token", "expires_in": 3600})
	})

	edgeBase := "/vmc/api/orgs/" + cloudAWSEdgeTestOrg + "/sddcs/" + cloudAWSEdgeTestSDDC + "/networks/4.0/edges/" + cloudAWSEdgeTestEdge

	// --- IPSec ---
	mux.HandleFunc(edgeBase+"/ipsec/config", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, map[string]interface{}{"enabled": true, "showSensitiveData": r.URL.Query().Get("showSensitiveData")})
		case http.MethodPut:
			var body interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			writeJSON(w, map[string]interface{}{"updated": true, "received": body})
		case http.MethodDelete:
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc(edgeBase+"/ipsec/statistics", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{"tunnels": []interface{}{}})
	})

	// --- L2VPN --- (note: config PUT/DELETE use .../sddc/cgws/{edgeId}/l2vpn/config,
	// NOT the .../edges/{edgeId}/... prefix — confirmed by reading the vendored
	// Postman collection directly; statistics uses the .../edges/{edgeId}/... prefix.)
	l2vpnConfigPath := "/vmc/api/orgs/" + cloudAWSEdgeTestOrg + "/sddcs/" + cloudAWSEdgeTestSDDC + "/networks/4.0/sddc/cgws/" + cloudAWSEdgeTestEdge + "/l2vpn/config"
	mux.HandleFunc(l2vpnConfigPath, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			var body interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			writeJSON(w, map[string]interface{}{"updated": true, "showSensitiveData": r.URL.Query().Get("showSensitiveData"), "received": body})
		case http.MethodDelete:
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc(edgeBase+"/l2vpn/config/statistics", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{"peers": []interface{}{}})
	})

	// --- Edge DNS ---
	mux.HandleFunc(edgeBase+"/dns/config", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, map[string]interface{}{"enabled": true})
		case http.MethodPut:
			var body interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			writeJSON(w, map[string]interface{}{"updated": true, "received": body})
		case http.MethodPost:
			writeJSON(w, map[string]interface{}{"enable": r.URL.Query().Get("enable")})
		case http.MethodDelete:
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc(edgeBase+"/dns/statistics", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{"queries": 0})
	})

	// --- Edge Devices ---
	edgesListPath := "/vmc/api/orgs/" + cloudAWSEdgeTestOrg + "/sddcs/" + cloudAWSEdgeTestSDDC + "/networks/4.0/edges"
	mux.HandleFunc(edgesListPath, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{"edgePage": map[string]interface{}{"data": []interface{}{cloudAWSEdgeTestEdge}}})
	})
	mux.HandleFunc(edgeBase+"/status", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{
			"getlatest": r.URL.Query().Get("getlatest"),
			"detailed":  r.URL.Query().Get("detailed"),
		})
	})
	mux.HandleFunc(edgeBase+"/vnics", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{"vnics": []interface{}{}})
	})
	mux.HandleFunc(edgeBase+"/peerconfig", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		writeJSON(w, map[string]interface{}{
			"objecttype": q.Get("objecttype"),
			"objectid":   q.Get("objectid"),
			"templateid": q.Get("templateid"),
		})
	})

	// --- DHCP ---
	mux.HandleFunc(edgeBase+"/dhcp/leaseInfo", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{"leases": []interface{}{}})
	})

	// --- Statistics ---
	for _, section := range []string{"interface", "firewall", "ipsec"} {
		section := section
		mux.HandleFunc(edgeBase+"/statistics/dashboard/"+section, func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, map[string]interface{}{"section": section, "interval": r.URL.Query().Get("interval")})
		})
	}
	mux.HandleFunc(edgeBase+"/statistics/interfaces", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{"startTime": r.URL.Query().Get("startTime"), "endTime": r.URL.Query().Get("endTime")})
	})
	mux.HandleFunc(edgeBase+"/statistics/interfaces/uplink", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{"startTime": r.URL.Query().Get("startTime"), "endTime": r.URL.Query().Get("endTime")})
	})

	// --- Connectivity ---
	connPath := "/vmc/api/orgs/" + cloudAWSEdgeTestOrg + "/sddcs/" + cloudAWSEdgeTestSDDC + "/networking/connectivity-tests"
	mux.HandleFunc(connPath, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, map[string]interface{}{"tests": []interface{}{}})
		case http.MethodPost:
			var body interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			writeJSON(w, map[string]interface{}{"action": r.URL.Query().Get("action"), "received": body})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	return httptest.NewServer(mux)
}

// newCloudAWSNetworkingEdgeFixtureClient builds a *cloudaws.Client whose
// CSP auth and VMC API calls both hit srv.
func newCloudAWSNetworkingEdgeFixtureClient(t *testing.T, srv *httptest.Server) *cloudaws.Client {
	t.Helper()
	c, err := cloudaws.NewClient(cloudaws.Config{
		RefreshToken: "fixture-refresh-token",
		AuthURL:      srv.URL,
		BaseURL:      srv.URL,
	})
	if err != nil {
		t.Fatalf("failed to build fixture cloudaws.Client: %v", err)
	}
	return c
}

// newCloudAWSNetworkingEdgeRegistry builds a Registry restricted to
// ConnectionModeCloudAWS, backed by client, with the destructive gate
// opened only when allowDestructive is true — mirrors how main.go wires
// --cloud-aws-url plus --allow-destructive.
func newCloudAWSNetworkingEdgeRegistry(client *cloudaws.Client, allowDestructive bool) *Registry {
	return NewRegistry(context.Background(), nil, RegistryOptions{
		CloudAWSClient:   client,
		ConnectionMode:   ConnectionModeCloudAWS,
		AllowDestructive: allowDestructive,
	})
}

func baseEdgeArgs() map[string]interface{} {
	return map[string]interface{}{
		"org":     cloudAWSEdgeTestOrg,
		"sddc":    cloudAWSEdgeTestSDDC,
		"edge_id": cloudAWSEdgeTestEdge,
	}
}

// --- IPSec ---------------------------------------------------------------

func TestCloudAWSNetworkingEdge_IPSecConfigGet(t *testing.T) {
	srv := cloudAWSNetworkingEdgeFixture(t)
	defer srv.Close()
	r := newCloudAWSNetworkingEdgeRegistry(newCloudAWSNetworkingEdgeFixtureClient(t, srv), false)

	args := baseEdgeArgs()
	args["show_sensitive_data"] = true
	raw, err := r.CallTool("vmware_cloudaws_network_ipsec_config_get", args)
	if err != nil {
		t.Fatalf("vmware_cloudaws_network_ipsec_config_get failed: %v", err)
	}
	m := decodeResult(t, raw)
	if m["showSensitiveData"] != "true" {
		t.Fatalf("expected showSensitiveData=true to reach the server, got: %s", raw)
	}
}

func TestCloudAWSNetworkingEdge_IPSecConfigUpdate(t *testing.T) {
	srv := cloudAWSNetworkingEdgeFixture(t)
	defer srv.Close()
	r := newCloudAWSNetworkingEdgeRegistry(newCloudAWSNetworkingEdgeFixtureClient(t, srv), true)

	args := baseEdgeArgs()
	args["config"] = map[string]interface{}{"enabled": true}
	args["confirm"] = true
	raw, err := r.CallTool("vmware_cloudaws_network_ipsec_config_update", args)
	if err != nil {
		t.Fatalf("vmware_cloudaws_network_ipsec_config_update failed: %v", err)
	}
	m := decodeResult(t, raw)
	if m["updated"] != true {
		t.Fatalf("expected updated:true, got: %s", raw)
	}
}

func TestCloudAWSNetworkingEdge_IPSecConfigDelete(t *testing.T) {
	srv := cloudAWSNetworkingEdgeFixture(t)
	defer srv.Close()
	r := newCloudAWSNetworkingEdgeRegistry(newCloudAWSNetworkingEdgeFixtureClient(t, srv), true)

	args := baseEdgeArgs()
	args["confirm"] = true
	raw, err := r.CallTool("vmware_cloudaws_network_ipsec_config_delete", args)
	if err != nil {
		t.Fatalf("vmware_cloudaws_network_ipsec_config_delete failed: %v", err)
	}
	m := decodeResult(t, raw)
	if m["result"] != "deleted" || m["edge_id"] != cloudAWSEdgeTestEdge {
		t.Fatalf("expected a deletion confirmation, got: %s", raw)
	}
}

func TestCloudAWSNetworkingEdge_IPSecStatisticsGet(t *testing.T) {
	srv := cloudAWSNetworkingEdgeFixture(t)
	defer srv.Close()
	r := newCloudAWSNetworkingEdgeRegistry(newCloudAWSNetworkingEdgeFixtureClient(t, srv), false)

	raw, err := r.CallTool("vmware_cloudaws_network_ipsec_statistics_get", baseEdgeArgs())
	if err != nil {
		t.Fatalf("vmware_cloudaws_network_ipsec_statistics_get failed: %v", err)
	}
	if _, ok := decodeResult(t, raw)["tunnels"]; !ok {
		t.Fatalf("expected a tunnels field, got: %s", raw)
	}
}

// --- L2VPN -----------------------------------------------------------------

func TestCloudAWSNetworkingEdge_L2VPNConfigUpdate(t *testing.T) {
	srv := cloudAWSNetworkingEdgeFixture(t)
	defer srv.Close()
	r := newCloudAWSNetworkingEdgeRegistry(newCloudAWSNetworkingEdgeFixtureClient(t, srv), true)

	args := baseEdgeArgs()
	args["config"] = map[string]interface{}{"peerCode": "abc"}
	args["show_sensitive_data"] = false
	args["confirm"] = true
	raw, err := r.CallTool("vmware_cloudaws_network_l2vpn_config_update", args)
	if err != nil {
		t.Fatalf("vmware_cloudaws_network_l2vpn_config_update failed: %v", err)
	}
	m := decodeResult(t, raw)
	// show_sensitive_data:false must still reach the request as "false", not
	// be silently dropped as if it were "not supplied" — proves
	// cloudAWSEdgeOptBoolParam distinguishes present-false from absent.
	if m["showSensitiveData"] != "false" {
		t.Fatalf("expected showSensitiveData=false to reach the server (present-but-false must not be dropped), got: %s", raw)
	}
}

func TestCloudAWSNetworkingEdge_L2VPNConfigDelete(t *testing.T) {
	srv := cloudAWSNetworkingEdgeFixture(t)
	defer srv.Close()
	r := newCloudAWSNetworkingEdgeRegistry(newCloudAWSNetworkingEdgeFixtureClient(t, srv), true)

	args := baseEdgeArgs()
	args["confirm"] = true
	raw, err := r.CallTool("vmware_cloudaws_network_l2vpn_config_delete", args)
	if err != nil {
		t.Fatalf("vmware_cloudaws_network_l2vpn_config_delete failed: %v", err)
	}
	if decodeResult(t, raw)["result"] != "deleted" {
		t.Fatalf("expected a deletion confirmation, got: %s", raw)
	}
}

func TestCloudAWSNetworkingEdge_L2VPNStatisticsGet(t *testing.T) {
	srv := cloudAWSNetworkingEdgeFixture(t)
	defer srv.Close()
	r := newCloudAWSNetworkingEdgeRegistry(newCloudAWSNetworkingEdgeFixtureClient(t, srv), false)

	raw, err := r.CallTool("vmware_cloudaws_network_l2vpn_statistics_get", baseEdgeArgs())
	if err != nil {
		t.Fatalf("vmware_cloudaws_network_l2vpn_statistics_get failed: %v", err)
	}
	if _, ok := decodeResult(t, raw)["peers"]; !ok {
		t.Fatalf("expected a peers field, got: %s", raw)
	}
}

// --- Edge DNS ----------------------------------------------------------------

func TestCloudAWSNetworkingEdge_EdgeDNSConfigGet(t *testing.T) {
	srv := cloudAWSNetworkingEdgeFixture(t)
	defer srv.Close()
	r := newCloudAWSNetworkingEdgeRegistry(newCloudAWSNetworkingEdgeFixtureClient(t, srv), false)

	raw, err := r.CallTool("vmware_cloudaws_network_edge_dns_config_get", baseEdgeArgs())
	if err != nil {
		t.Fatalf("vmware_cloudaws_network_edge_dns_config_get failed: %v", err)
	}
	if decodeResult(t, raw)["enabled"] != true {
		t.Fatalf("unexpected result: %s", raw)
	}
}

func TestCloudAWSNetworkingEdge_EdgeDNSConfigUpdate(t *testing.T) {
	srv := cloudAWSNetworkingEdgeFixture(t)
	defer srv.Close()
	r := newCloudAWSNetworkingEdgeRegistry(newCloudAWSNetworkingEdgeFixtureClient(t, srv), true)

	args := baseEdgeArgs()
	args["config"] = map[string]interface{}{"dnsViewId": "default"}
	args["confirm"] = true
	raw, err := r.CallTool("vmware_cloudaws_network_edge_dns_config_update", args)
	if err != nil {
		t.Fatalf("vmware_cloudaws_network_edge_dns_config_update failed: %v", err)
	}
	if decodeResult(t, raw)["updated"] != true {
		t.Fatalf("unexpected result: %s", raw)
	}
}

func TestCloudAWSNetworkingEdge_EdgeDNSStatusSet(t *testing.T) {
	srv := cloudAWSNetworkingEdgeFixture(t)
	defer srv.Close()
	r := newCloudAWSNetworkingEdgeRegistry(newCloudAWSNetworkingEdgeFixtureClient(t, srv), true)

	args := baseEdgeArgs()
	args["enable"] = true
	args["confirm"] = true
	raw, err := r.CallTool("vmware_cloudaws_network_edge_dns_status_set", args)
	if err != nil {
		t.Fatalf("vmware_cloudaws_network_edge_dns_status_set failed: %v", err)
	}
	if decodeResult(t, raw)["enable"] != "true" {
		t.Fatalf("expected enable=true to reach the server, got: %s", raw)
	}
}

func TestCloudAWSNetworkingEdge_EdgeDNSStatusSet_RequiresEnable(t *testing.T) {
	srv := cloudAWSNetworkingEdgeFixture(t)
	defer srv.Close()
	r := newCloudAWSNetworkingEdgeRegistry(newCloudAWSNetworkingEdgeFixtureClient(t, srv), true)

	args := baseEdgeArgs()
	args["confirm"] = true // enable deliberately omitted
	if _, err := r.CallTool("vmware_cloudaws_network_edge_dns_status_set", args); err == nil {
		t.Fatal("expected an error: enable is required")
	}
}

func TestCloudAWSNetworkingEdge_EdgeDNSConfigDelete(t *testing.T) {
	srv := cloudAWSNetworkingEdgeFixture(t)
	defer srv.Close()
	r := newCloudAWSNetworkingEdgeRegistry(newCloudAWSNetworkingEdgeFixtureClient(t, srv), true)

	args := baseEdgeArgs()
	args["confirm"] = true
	raw, err := r.CallTool("vmware_cloudaws_network_edge_dns_config_delete", args)
	if err != nil {
		t.Fatalf("vmware_cloudaws_network_edge_dns_config_delete failed: %v", err)
	}
	if decodeResult(t, raw)["result"] != "deleted" {
		t.Fatalf("expected a deletion confirmation, got: %s", raw)
	}
}

func TestCloudAWSNetworkingEdge_EdgeDNSStatisticsGet(t *testing.T) {
	srv := cloudAWSNetworkingEdgeFixture(t)
	defer srv.Close()
	r := newCloudAWSNetworkingEdgeRegistry(newCloudAWSNetworkingEdgeFixtureClient(t, srv), false)

	raw, err := r.CallTool("vmware_cloudaws_network_edge_dns_statistics_get", baseEdgeArgs())
	if err != nil {
		t.Fatalf("vmware_cloudaws_network_edge_dns_statistics_get failed: %v", err)
	}
	if _, ok := decodeResult(t, raw)["queries"]; !ok {
		t.Fatalf("unexpected result: %s", raw)
	}
}

// --- Edge Devices --------------------------------------------------------------

func TestCloudAWSNetworkingEdge_EdgeList(t *testing.T) {
	srv := cloudAWSNetworkingEdgeFixture(t)
	defer srv.Close()
	r := newCloudAWSNetworkingEdgeRegistry(newCloudAWSNetworkingEdgeFixtureClient(t, srv), false)

	raw, err := r.CallTool("vmware_cloudaws_network_edge_list", map[string]interface{}{
		"org": cloudAWSEdgeTestOrg, "sddc": cloudAWSEdgeTestSDDC,
	})
	if err != nil {
		t.Fatalf("vmware_cloudaws_network_edge_list failed: %v", err)
	}
	if _, ok := decodeResult(t, raw)["edgePage"]; !ok {
		t.Fatalf("unexpected result: %s", raw)
	}
}

func TestCloudAWSNetworkingEdge_EdgeStatusGet(t *testing.T) {
	srv := cloudAWSNetworkingEdgeFixture(t)
	defer srv.Close()
	r := newCloudAWSNetworkingEdgeRegistry(newCloudAWSNetworkingEdgeFixtureClient(t, srv), false)

	args := baseEdgeArgs()
	args["get_latest"] = true
	args["detailed"] = false // present-but-false must still reach the request
	raw, err := r.CallTool("vmware_cloudaws_network_edge_status_get", args)
	if err != nil {
		t.Fatalf("vmware_cloudaws_network_edge_status_get failed: %v", err)
	}
	m := decodeResult(t, raw)
	if m["getlatest"] != "true" || m["detailed"] != "false" {
		t.Fatalf("expected getlatest=true&detailed=false to reach the server, got: %s", raw)
	}
}

func TestCloudAWSNetworkingEdge_EdgeVNICsList(t *testing.T) {
	srv := cloudAWSNetworkingEdgeFixture(t)
	defer srv.Close()
	r := newCloudAWSNetworkingEdgeRegistry(newCloudAWSNetworkingEdgeFixtureClient(t, srv), false)

	raw, err := r.CallTool("vmware_cloudaws_network_edge_vnics_list", baseEdgeArgs())
	if err != nil {
		t.Fatalf("vmware_cloudaws_network_edge_vnics_list failed: %v", err)
	}
	if _, ok := decodeResult(t, raw)["vnics"]; !ok {
		t.Fatalf("unexpected result: %s", raw)
	}
}

func TestCloudAWSNetworkingEdge_EdgePeerConfigList(t *testing.T) {
	srv := cloudAWSNetworkingEdgeFixture(t)
	defer srv.Close()
	r := newCloudAWSNetworkingEdgeRegistry(newCloudAWSNetworkingEdgeFixtureClient(t, srv), false)

	args := baseEdgeArgs()
	args["object_type"] = "ipsecVpnSite"
	args["object_id"] = "site-1"
	args["template_id"] = "tmpl-1"
	raw, err := r.CallTool("vmware_cloudaws_network_edge_peerconfig_list", args)
	if err != nil {
		t.Fatalf("vmware_cloudaws_network_edge_peerconfig_list failed: %v", err)
	}
	m := decodeResult(t, raw)
	if m["objecttype"] != "ipsecVpnSite" || m["objectid"] != "site-1" || m["templateid"] != "tmpl-1" {
		t.Fatalf("expected the 3 optional query params to reach the server, got: %s", raw)
	}
}

// --- DHCP ------------------------------------------------------------------------

func TestCloudAWSNetworkingEdge_EdgeDHCPLeasesList(t *testing.T) {
	srv := cloudAWSNetworkingEdgeFixture(t)
	defer srv.Close()
	r := newCloudAWSNetworkingEdgeRegistry(newCloudAWSNetworkingEdgeFixtureClient(t, srv), false)

	raw, err := r.CallTool("vmware_cloudaws_network_edge_dhcp_leases_list", baseEdgeArgs())
	if err != nil {
		t.Fatalf("vmware_cloudaws_network_edge_dhcp_leases_list failed: %v", err)
	}
	if _, ok := decodeResult(t, raw)["leases"]; !ok {
		t.Fatalf("unexpected result: %s", raw)
	}
}

// --- Statistics --------------------------------------------------------------------

func TestCloudAWSNetworkingEdge_StatsDashboard(t *testing.T) {
	srv := cloudAWSNetworkingEdgeFixture(t)
	defer srv.Close()
	r := newCloudAWSNetworkingEdgeRegistry(newCloudAWSNetworkingEdgeFixtureClient(t, srv), false)

	cases := []struct {
		tool    string
		section string
	}{
		{"vmware_cloudaws_network_edge_stats_dashboard_interface", "interface"},
		{"vmware_cloudaws_network_edge_stats_dashboard_firewall", "firewall"},
		{"vmware_cloudaws_network_edge_stats_dashboard_ipsec", "ipsec"},
	}
	for _, c := range cases {
		args := baseEdgeArgs()
		args["interval"] = "day"
		raw, err := r.CallTool(c.tool, args)
		if err != nil {
			t.Fatalf("%s failed: %v", c.tool, err)
		}
		m := decodeResult(t, raw)
		if m["section"] != c.section || m["interval"] != "day" {
			t.Fatalf("%s: unexpected result: %s", c.tool, raw)
		}
	}
}

func TestCloudAWSNetworkingEdge_StatsInterfaces(t *testing.T) {
	srv := cloudAWSNetworkingEdgeFixture(t)
	defer srv.Close()
	r := newCloudAWSNetworkingEdgeRegistry(newCloudAWSNetworkingEdgeFixtureClient(t, srv), false)

	for _, tool := range []string{"vmware_cloudaws_network_edge_stats_interfaces", "vmware_cloudaws_network_edge_stats_interfaces_uplink"} {
		args := baseEdgeArgs()
		args["start_time"] = "1000"
		args["end_time"] = "2000"
		raw, err := r.CallTool(tool, args)
		if err != nil {
			t.Fatalf("%s failed: %v", tool, err)
		}
		m := decodeResult(t, raw)
		if m["startTime"] != "1000" || m["endTime"] != "2000" {
			t.Fatalf("%s: expected startTime/endTime to reach the server, got: %s", tool, raw)
		}
	}
}

// --- Connectivity -------------------------------------------------------------------

func TestCloudAWSNetworkingEdge_ConnectivityTestList(t *testing.T) {
	srv := cloudAWSNetworkingEdgeFixture(t)
	defer srv.Close()
	r := newCloudAWSNetworkingEdgeRegistry(newCloudAWSNetworkingEdgeFixtureClient(t, srv), false)

	raw, err := r.CallTool("vmware_cloudaws_network_connectivity_test_list", map[string]interface{}{
		"org": cloudAWSEdgeTestOrg, "sddc": cloudAWSEdgeTestSDDC,
	})
	if err != nil {
		t.Fatalf("vmware_cloudaws_network_connectivity_test_list failed: %v", err)
	}
	if _, ok := decodeResult(t, raw)["tests"]; !ok {
		t.Fatalf("unexpected result: %s", raw)
	}
}

func TestCloudAWSNetworkingEdge_ConnectivityTestRun(t *testing.T) {
	srv := cloudAWSNetworkingEdgeFixture(t)
	defer srv.Close()
	// sem-tier: no gate, no confirm needed even though it's a POST — proves
	// this tool was NOT registered via registerDestructiveCloudAWS.
	r := newCloudAWSNetworkingEdgeRegistry(newCloudAWSNetworkingEdgeFixtureClient(t, srv), false)

	raw, err := r.CallTool("vmware_cloudaws_network_connectivity_test_run", map[string]interface{}{
		"org":     cloudAWSEdgeTestOrg,
		"sddc":    cloudAWSEdgeTestSDDC,
		"action":  "layer3",
		"request": map[string]interface{}{"destination": "10.0.0.1"},
	})
	if err != nil {
		t.Fatalf("vmware_cloudaws_network_connectivity_test_run failed: %v", err)
	}
	if decodeResult(t, raw)["action"] != "layer3" {
		t.Fatalf("expected action=layer3 to reach the server, got: %s", raw)
	}
}

func TestCloudAWSNetworkingEdge_ConnectivityTestRun_RequiresAction(t *testing.T) {
	srv := cloudAWSNetworkingEdgeFixture(t)
	defer srv.Close()
	r := newCloudAWSNetworkingEdgeRegistry(newCloudAWSNetworkingEdgeFixtureClient(t, srv), false)

	_, err := r.CallTool("vmware_cloudaws_network_connectivity_test_run", map[string]interface{}{
		"org": cloudAWSEdgeTestOrg, "sddc": cloudAWSEdgeTestSDDC,
	})
	if err == nil {
		t.Fatal("expected an error: action is required")
	}
}

// --- Cross-cutting: destructive gate / confirm / required args --------------------

// TestCloudAWSNetworkingEdge_DestructiveGateClosed proves a tier1/tier2 tool
// fails before any round trip to the fixture server when the server was
// started without --allow-destructive — even with confirm:true supplied.
func TestCloudAWSNetworkingEdge_DestructiveGateClosed(t *testing.T) {
	srv := cloudAWSNetworkingEdgeFixture(t)
	defer srv.Close()
	r := newCloudAWSNetworkingEdgeRegistry(newCloudAWSNetworkingEdgeFixtureClient(t, srv), false)

	args := baseEdgeArgs()
	args["confirm"] = true
	if _, err := r.CallTool("vmware_cloudaws_network_ipsec_config_delete", args); err == nil {
		t.Fatal("expected an error: destructive gate is closed (AllowDestructive not set)")
	}
}

// TestCloudAWSNetworkingEdge_DestructiveConfirmMissing proves a tier1/tier2
// tool fails when the gate is open but confirm:true was not passed.
func TestCloudAWSNetworkingEdge_DestructiveConfirmMissing(t *testing.T) {
	srv := cloudAWSNetworkingEdgeFixture(t)
	defer srv.Close()
	r := newCloudAWSNetworkingEdgeRegistry(newCloudAWSNetworkingEdgeFixtureClient(t, srv), true)

	if _, err := r.CallTool("vmware_cloudaws_network_ipsec_config_delete", baseEdgeArgs()); err == nil {
		t.Fatal("expected an error: confirm:true was not passed")
	}
}

// TestCloudAWSNetworkingEdge_RequiredArgsMissing proves every edge-scoped
// tool rejects a call missing org/sddc/edge_id before touching the fixture
// server.
func TestCloudAWSNetworkingEdge_RequiredArgsMissing(t *testing.T) {
	srv := cloudAWSNetworkingEdgeFixture(t)
	defer srv.Close()
	r := newCloudAWSNetworkingEdgeRegistry(newCloudAWSNetworkingEdgeFixtureClient(t, srv), false)

	cases := []struct {
		name string
		args map[string]interface{}
	}{
		{"missing org", map[string]interface{}{"sddc": cloudAWSEdgeTestSDDC, "edge_id": cloudAWSEdgeTestEdge}},
		{"missing sddc", map[string]interface{}{"org": cloudAWSEdgeTestOrg, "edge_id": cloudAWSEdgeTestEdge}},
		{"missing edge_id", map[string]interface{}{"org": cloudAWSEdgeTestOrg, "sddc": cloudAWSEdgeTestSDDC}},
	}
	for _, c := range cases {
		if _, err := r.CallTool("vmware_cloudaws_network_ipsec_config_get", c.args); err == nil {
			t.Fatalf("%s: expected an error", c.name)
		}
	}

	if _, err := r.CallTool("vmware_cloudaws_network_edge_list", map[string]interface{}{"sddc": cloudAWSEdgeTestSDDC}); err == nil {
		t.Fatal("missing org (edge_list): expected an error")
	}
}

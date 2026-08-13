package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cslsoftwares/mcpvmware/cloudaws"
)

// cloudAWSNetCoreFixture starts a CSP auth server (token exchange, always
// succeeds) and a VMC API server backed by mux, and returns a *cloudaws.
// Client wired to both — cloudaws/client_test.go already covers the token-
// exchange/retry mechanics in isolation, so this fixture only needs a
// client that authenticates once and then hits mux, mirroring
// applianceFixture/newApplianceFixtureClient's split in appliance_test.go.
// No simulator and no live VMC on AWS org/SDDC is available to this
// project (see cloudaws/client.go's package doc comment), so every test in
// this file is a fixture unit test, not an integration test.
func cloudAWSNetCoreFixture(t *testing.T, mux *http.ServeMux) (*cloudaws.Client, func()) {
	t.Helper()
	authSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"access_token": "access-1", "expires_in": 1800})
	}))
	apiSrv := httptest.NewServer(mux)

	c, err := cloudaws.NewClient(cloudaws.Config{RefreshToken: "refresh-abc", AuthURL: authSrv.URL, BaseURL: apiSrv.URL})
	if err != nil {
		t.Fatalf("cloudaws.NewClient: %v", err)
	}

	cleanup := func() {
		authSrv.Close()
		apiSrv.Close()
	}
	return c, cleanup
}

// cloudAWSNetCoreRegistry builds a Registry over a fixture client. gateOpen
// controls RegistryOptions.AllowDestructive — tests exercising tier1/tier2
// happy paths pass true; tests exercising the closed-gate denial pass
// false.
func cloudAWSNetCoreRegistry(client *cloudaws.Client, gateOpen bool) *Registry {
	return NewRegistry(context.Background(), nil, RegistryOptions{
		CloudAWSClient:   client,
		AllowDestructive: gateOpen,
	})
}

// cloudAWSNetCoreWriteJSON is named with this file's prefix (not a bare
// writeJSON) deliberately — 3 other Fase 10 "Grupo" test files are being
// written in parallel against the same package and a generic helper name
// would risk a duplicate-declaration collision at integration time.
func cloudAWSNetCoreWriteJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// --- Networks -----------------------------------------------------------

func TestCloudAWSNetworkingCore_NetworkList_HappyPath(t *testing.T) {
	var gotPath, gotMethod string
	mux := http.NewServeMux()
	mux.HandleFunc("/vmc/api/orgs/org-1/sddcs/sddc-1/networks/4.0/sddc/networks/", func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		cloudAWSNetCoreWriteJSON(w, []map[string]string{{"id": "net-1", "name": "web-tier"}})
	})
	client, cleanup := cloudAWSNetCoreFixture(t, mux)
	defer cleanup()
	r := cloudAWSNetCoreRegistry(client, false)

	raw, err := r.CallTool("vmware_cloudaws_network_list", map[string]interface{}{"org": "org-1", "sddc": "sddc-1"})
	if err != nil {
		t.Fatalf("vmware_cloudaws_network_list failed: %v", err)
	}
	if gotMethod != http.MethodGet {
		t.Fatalf("expected GET, got %s", gotMethod)
	}
	if gotPath != "/vmc/api/orgs/org-1/sddcs/sddc-1/networks/4.0/sddc/networks/" {
		t.Fatalf("unexpected path: %s", gotPath)
	}
	var out []map[string]string
	if err := json.Unmarshal([]byte(raw), &out); err != nil || len(out) != 1 || out[0]["id"] != "net-1" {
		t.Fatalf("unexpected result: %s (err=%v)", raw, err)
	}
}

func TestCloudAWSNetworkingCore_NetworkList_MissingOrgFailsBeforeAnyCall(t *testing.T) {
	called := false
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { called = true })
	client, cleanup := cloudAWSNetCoreFixture(t, mux)
	defer cleanup()
	r := cloudAWSNetCoreRegistry(client, false)

	if _, err := r.CallTool("vmware_cloudaws_network_list", map[string]interface{}{"sddc": "sddc-1"}); err == nil {
		t.Fatal("expected error for missing org, got nil")
	}
	if called {
		t.Fatal("handler must not reach the VMC API when a required argument is missing")
	}
}

func TestCloudAWSNetworkingCore_NetworkGet_HappyPath(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/vmc/api/orgs/org-1/sddcs/sddc-1/networks/4.0/sddc/networks/net-1", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		cloudAWSNetCoreWriteJSON(w, map[string]string{"id": "net-1", "name": "web-tier"})
	})
	client, cleanup := cloudAWSNetCoreFixture(t, mux)
	defer cleanup()
	r := cloudAWSNetCoreRegistry(client, false)

	raw, err := r.CallTool("vmware_cloudaws_network_get", map[string]interface{}{"org": "org-1", "sddc": "sddc-1", "network_id": "net-1"})
	if err != nil {
		t.Fatalf("vmware_cloudaws_network_get failed: %v", err)
	}
	m := decodeResult(t, raw)
	if m["id"] != "net-1" {
		t.Fatalf("unexpected result: %s", raw)
	}
}

func TestCloudAWSNetworkingCore_NetworkCreate_GateClosedDenied(t *testing.T) {
	called := false
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { called = true })
	client, cleanup := cloudAWSNetCoreFixture(t, mux)
	defer cleanup()
	r := cloudAWSNetCoreRegistry(client, false) // gate closed

	_, err := r.CallTool("vmware_cloudaws_network_create", map[string]interface{}{
		"org": "org-1", "sddc": "sddc-1",
		"body":    map[string]interface{}{"name": "new-net"},
		"confirm": true,
	})
	if err == nil {
		t.Fatal("expected error with the destructive gate closed, got nil")
	}
	if called {
		t.Fatal("handler must not reach the VMC API when the destructive gate is closed")
	}
}

func TestCloudAWSNetworkingCore_NetworkCreate_ConfirmRequiredEvenWithGateOpen(t *testing.T) {
	called := false
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { called = true })
	client, cleanup := cloudAWSNetCoreFixture(t, mux)
	defer cleanup()
	r := cloudAWSNetCoreRegistry(client, true) // gate open

	_, err := r.CallTool("vmware_cloudaws_network_create", map[string]interface{}{
		"org": "org-1", "sddc": "sddc-1",
		"body": map[string]interface{}{"name": "new-net"},
		// confirm omitted
	})
	if err == nil {
		t.Fatal("expected error without confirm:true, got nil")
	}
	if called {
		t.Fatal("handler must not reach the VMC API without confirm:true")
	}
}

func TestCloudAWSNetworkingCore_NetworkCreate_HappyPath(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody map[string]interface{}
	mux := http.NewServeMux()
	mux.HandleFunc("/vmc/api/orgs/org-1/sddcs/sddc-1/networks/4.0/sddc/networks", func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		cloudAWSNetCoreWriteJSON(w, map[string]string{"id": "net-2", "name": "new-net"})
	})
	client, cleanup := cloudAWSNetCoreFixture(t, mux)
	defer cleanup()
	r := cloudAWSNetCoreRegistry(client, true)

	raw, err := r.CallTool("vmware_cloudaws_network_create", map[string]interface{}{
		"org": "org-1", "sddc": "sddc-1",
		"body":    map[string]interface{}{"name": "new-net"},
		"confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_cloudaws_network_create failed: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("expected POST, got %s", gotMethod)
	}
	if gotPath != "/vmc/api/orgs/org-1/sddcs/sddc-1/networks/4.0/sddc/networks" {
		t.Fatalf("unexpected path: %s", gotPath)
	}
	if gotBody["name"] != "new-net" {
		t.Fatalf("request body not forwarded verbatim: %+v", gotBody)
	}
	m := decodeResult(t, raw)
	if m["id"] != "net-2" {
		t.Fatalf("unexpected result: %s", raw)
	}
}

func TestCloudAWSNetworkingCore_NetworkCreate_BodyMustBeObject(t *testing.T) {
	called := false
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { called = true })
	client, cleanup := cloudAWSNetCoreFixture(t, mux)
	defer cleanup()
	r := cloudAWSNetCoreRegistry(client, true)

	_, err := r.CallTool("vmware_cloudaws_network_create", map[string]interface{}{
		"org": "org-1", "sddc": "sddc-1",
		"body":    "not-an-object",
		"confirm": true,
	})
	if err == nil {
		t.Fatal("expected error for a non-object body, got nil")
	}
	if called {
		t.Fatal("handler must not reach the VMC API with an invalid body")
	}
}

func TestCloudAWSNetworkingCore_NetworkDelete_Tier1RequiresConfirm(t *testing.T) {
	client, cleanup := cloudAWSNetCoreFixture(t, http.NewServeMux())
	defer cleanup()
	r := cloudAWSNetCoreRegistry(client, true)

	_, err := r.CallTool("vmware_cloudaws_network_delete", map[string]interface{}{
		"org": "org-1", "sddc": "sddc-1", "network_id": "net-1",
	})
	if err == nil {
		t.Fatal("expected error without confirm:true, got nil")
	}
}

func TestCloudAWSNetworkingCore_NetworkDelete_HappyPath(t *testing.T) {
	var gotMethod string
	mux := http.NewServeMux()
	mux.HandleFunc("/vmc/api/orgs/org-1/sddcs/sddc-1/networks/4.0/sddc/networks/net-1", func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.WriteHeader(http.StatusNoContent)
	})
	client, cleanup := cloudAWSNetCoreFixture(t, mux)
	defer cleanup()
	r := cloudAWSNetCoreRegistry(client, true)

	raw, err := r.CallTool("vmware_cloudaws_network_delete", map[string]interface{}{
		"org": "org-1", "sddc": "sddc-1", "network_id": "net-1", "confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_cloudaws_network_delete failed: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Fatalf("expected DELETE, got %s", gotMethod)
	}
	m := decodeResult(t, raw)
	if m["result"] != "ok" {
		t.Fatalf("expected the no-body-response fallback {\"result\":\"ok\"}, got %s", raw)
	}
}

// --- Firewall -------------------------------------------------------------

func TestCloudAWSNetworkingCore_FirewallList_HappyPath(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/vmc/api/orgs/org-1/sddcs/sddc-1/networks/4.0/edges/edge-1/firewall/config", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		cloudAWSNetCoreWriteJSON(w, map[string]interface{}{"firewallRules": []interface{}{}})
	})
	client, cleanup := cloudAWSNetCoreFixture(t, mux)
	defer cleanup()
	r := cloudAWSNetCoreRegistry(client, false)

	raw, err := r.CallTool("vmware_cloudaws_network_firewall_list", map[string]interface{}{"org": "org-1", "sddc": "sddc-1", "edge_id": "edge-1"})
	if err != nil {
		t.Fatalf("vmware_cloudaws_network_firewall_list failed: %v", err)
	}
	m := decodeResult(t, raw)
	if _, ok := m["firewallRules"]; !ok {
		t.Fatalf("unexpected result: %s", raw)
	}
}

func TestCloudAWSNetworkingCore_FirewallUpdate_Tier1HappyPath(t *testing.T) {
	var gotMethod string
	mux := http.NewServeMux()
	mux.HandleFunc("/vmc/api/orgs/org-1/sddcs/sddc-1/networks/4.0/edges/edge-1/firewall/config", func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		cloudAWSNetCoreWriteJSON(w, map[string]interface{}{"firewallRules": []interface{}{"rule-1"}})
	})
	client, cleanup := cloudAWSNetCoreFixture(t, mux)
	defer cleanup()
	r := cloudAWSNetCoreRegistry(client, true)

	_, err := r.CallTool("vmware_cloudaws_network_firewall_update", map[string]interface{}{
		"org": "org-1", "sddc": "sddc-1", "edge_id": "edge-1",
		"body":    map[string]interface{}{"firewallRules": []interface{}{}},
		"confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_cloudaws_network_firewall_update failed: %v", err)
	}
	if gotMethod != http.MethodPut {
		t.Fatalf("expected PUT, got %s", gotMethod)
	}
}

func TestCloudAWSNetworkingCore_FirewallUpdate_GateClosedDenied(t *testing.T) {
	called := false
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { called = true })
	client, cleanup := cloudAWSNetCoreFixture(t, mux)
	defer cleanup()
	r := cloudAWSNetCoreRegistry(client, false)

	_, err := r.CallTool("vmware_cloudaws_network_firewall_update", map[string]interface{}{
		"org": "org-1", "sddc": "sddc-1", "edge_id": "edge-1",
		"body":    map[string]interface{}{"firewallRules": []interface{}{}},
		"confirm": true,
	})
	if err == nil {
		t.Fatal("expected error with the destructive gate closed, got nil")
	}
	if called {
		t.Fatal("handler must not reach the VMC API when the destructive gate is closed")
	}
}

func TestCloudAWSNetworkingCore_FirewallRuleGet_HappyPath(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/vmc/api/orgs/org-1/sddcs/sddc-1/networks/4.0/edges/edge-1/firewall/config/rules/rule-1", func(w http.ResponseWriter, r *http.Request) {
		cloudAWSNetCoreWriteJSON(w, map[string]interface{}{"ruleId": "rule-1", "action": "accept"})
	})
	client, cleanup := cloudAWSNetCoreFixture(t, mux)
	defer cleanup()
	r := cloudAWSNetCoreRegistry(client, false)

	raw, err := r.CallTool("vmware_cloudaws_network_firewall_rule_get", map[string]interface{}{
		"org": "org-1", "sddc": "sddc-1", "edge_id": "edge-1", "rule_id": "rule-1",
	})
	if err != nil {
		t.Fatalf("vmware_cloudaws_network_firewall_rule_get failed: %v", err)
	}
	m := decodeResult(t, raw)
	if m["ruleId"] != "rule-1" {
		t.Fatalf("unexpected result: %s", raw)
	}
}

func TestCloudAWSNetworkingCore_FirewallRuleCreate_Tier2HappyPath(t *testing.T) {
	var gotMethod, gotPath string
	mux := http.NewServeMux()
	mux.HandleFunc("/vmc/api/orgs/org-1/sddcs/sddc-1/networks/4.0/edges/edge-1/firewall/config/rules", func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		cloudAWSNetCoreWriteJSON(w, map[string]interface{}{"ruleId": "rule-2"})
	})
	client, cleanup := cloudAWSNetCoreFixture(t, mux)
	defer cleanup()
	r := cloudAWSNetCoreRegistry(client, true)

	raw, err := r.CallTool("vmware_cloudaws_network_firewall_rule_create", map[string]interface{}{
		"org": "org-1", "sddc": "sddc-1", "edge_id": "edge-1",
		"body":    map[string]interface{}{"action": "accept"},
		"confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_cloudaws_network_firewall_rule_create failed: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("expected POST, got %s", gotMethod)
	}
	if gotPath != "/vmc/api/orgs/org-1/sddcs/sddc-1/networks/4.0/edges/edge-1/firewall/config/rules" {
		t.Fatalf("unexpected path: %s", gotPath)
	}
	m := decodeResult(t, raw)
	if m["ruleId"] != "rule-2" {
		t.Fatalf("unexpected result: %s", raw)
	}
}

func TestCloudAWSNetworkingCore_FirewallRuleDelete_Tier1RequiresConfirm(t *testing.T) {
	client, cleanup := cloudAWSNetCoreFixture(t, http.NewServeMux())
	defer cleanup()
	r := cloudAWSNetCoreRegistry(client, true)

	_, err := r.CallTool("vmware_cloudaws_network_firewall_rule_delete", map[string]interface{}{
		"org": "org-1", "sddc": "sddc-1", "edge_id": "edge-1", "rule_id": "rule-1",
	})
	if err == nil {
		t.Fatal("expected error without confirm:true, got nil")
	}
}

func TestCloudAWSNetworkingCore_FirewallRuleStats_HappyPath(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/vmc/api/orgs/org-1/sddcs/sddc-1/networks/4.0/edges/edge-1/firewall/statistics/rule-1", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		cloudAWSNetCoreWriteJSON(w, map[string]interface{}{"ruleId": "rule-1", "packetCount": 42})
	})
	client, cleanup := cloudAWSNetCoreFixture(t, mux)
	defer cleanup()
	r := cloudAWSNetCoreRegistry(client, false)

	raw, err := r.CallTool("vmware_cloudaws_network_firewall_rule_stats", map[string]interface{}{
		"org": "org-1", "sddc": "sddc-1", "edge_id": "edge-1", "rule_id": "rule-1",
	})
	if err != nil {
		t.Fatalf("vmware_cloudaws_network_firewall_rule_stats failed: %v", err)
	}
	m := decodeResult(t, raw)
	if m["packetCount"] != float64(42) {
		t.Fatalf("unexpected result: %s", raw)
	}
}

// --- NAT --------------------------------------------------------------------

func TestCloudAWSNetworkingCore_NatList_HappyPath(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/vmc/api/orgs/org-1/sddcs/sddc-1/networks/4.0/edges/edge-1/nat/config", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		cloudAWSNetCoreWriteJSON(w, map[string]interface{}{"natRules": []interface{}{}})
	})
	client, cleanup := cloudAWSNetCoreFixture(t, mux)
	defer cleanup()
	r := cloudAWSNetCoreRegistry(client, false)

	raw, err := r.CallTool("vmware_cloudaws_network_nat_list", map[string]interface{}{"org": "org-1", "sddc": "sddc-1", "edge_id": "edge-1"})
	if err != nil {
		t.Fatalf("vmware_cloudaws_network_nat_list failed: %v", err)
	}
	m := decodeResult(t, raw)
	if _, ok := m["natRules"]; !ok {
		t.Fatalf("unexpected result: %s", raw)
	}
}

func TestCloudAWSNetworkingCore_NatUpdate_Tier1RequiresConfirm(t *testing.T) {
	client, cleanup := cloudAWSNetCoreFixture(t, http.NewServeMux())
	defer cleanup()
	r := cloudAWSNetCoreRegistry(client, true)

	_, err := r.CallTool("vmware_cloudaws_network_nat_update", map[string]interface{}{
		"org": "org-1", "sddc": "sddc-1", "edge_id": "edge-1",
		"body": map[string]interface{}{"natRules": []interface{}{}},
	})
	if err == nil {
		t.Fatal("expected error without confirm:true, got nil")
	}
}

func TestCloudAWSNetworkingCore_NatDelete_HappyPath(t *testing.T) {
	var gotMethod string
	mux := http.NewServeMux()
	mux.HandleFunc("/vmc/api/orgs/org-1/sddcs/sddc-1/networks/4.0/edges/edge-1/nat/config", func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.WriteHeader(http.StatusNoContent)
	})
	client, cleanup := cloudAWSNetCoreFixture(t, mux)
	defer cleanup()
	r := cloudAWSNetCoreRegistry(client, true)

	raw, err := r.CallTool("vmware_cloudaws_network_nat_delete", map[string]interface{}{
		"org": "org-1", "sddc": "sddc-1", "edge_id": "edge-1", "confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_cloudaws_network_nat_delete failed: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Fatalf("expected DELETE, got %s", gotMethod)
	}
	m := decodeResult(t, raw)
	if m["result"] != "ok" {
		t.Fatalf("expected the no-body-response fallback {\"result\":\"ok\"}, got %s", raw)
	}
}

// TestCloudAWSNetworkingCore_NatRuleCreate_DedupedRouteHappyPath exercises
// vmware_cloudaws_network_nat_rule_create — the tool that collapses the
// vendored Postman collection's two identical "NAT/Create" and "NAT/Rules/
// Create" items (same POST .../nat/config/rules) into one.
func TestCloudAWSNetworkingCore_NatRuleCreate_DedupedRouteHappyPath(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody map[string]interface{}
	mux := http.NewServeMux()
	mux.HandleFunc("/vmc/api/orgs/org-1/sddcs/sddc-1/networks/4.0/edges/edge-1/nat/config/rules", func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		cloudAWSNetCoreWriteJSON(w, map[string]interface{}{"ruleId": "nat-rule-1"})
	})
	client, cleanup := cloudAWSNetCoreFixture(t, mux)
	defer cleanup()
	r := cloudAWSNetCoreRegistry(client, true)

	raw, err := r.CallTool("vmware_cloudaws_network_nat_rule_create", map[string]interface{}{
		"org": "org-1", "sddc": "sddc-1", "edge_id": "edge-1",
		"body":    map[string]interface{}{"action": "dnat", "protocol": "tcp"},
		"confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_cloudaws_network_nat_rule_create failed: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("expected POST, got %s", gotMethod)
	}
	if gotPath != "/vmc/api/orgs/org-1/sddcs/sddc-1/networks/4.0/edges/edge-1/nat/config/rules" {
		t.Fatalf("unexpected path: %s", gotPath)
	}
	if gotBody["action"] != "dnat" || gotBody["protocol"] != "tcp" {
		t.Fatalf("request body not forwarded verbatim: %+v", gotBody)
	}
	m := decodeResult(t, raw)
	if m["ruleId"] != "nat-rule-1" {
		t.Fatalf("unexpected result: %s", raw)
	}
}

func TestCloudAWSNetworkingCore_NatRuleUpdate_HappyPath(t *testing.T) {
	var gotMethod, gotPath string
	mux := http.NewServeMux()
	mux.HandleFunc("/vmc/api/orgs/org-1/sddcs/sddc-1/networks/4.0/edges/edge-1/nat/config/rules/nat-rule-1", func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		cloudAWSNetCoreWriteJSON(w, map[string]interface{}{"ruleId": "nat-rule-1", "protocol": "udp"})
	})
	client, cleanup := cloudAWSNetCoreFixture(t, mux)
	defer cleanup()
	r := cloudAWSNetCoreRegistry(client, true)

	_, err := r.CallTool("vmware_cloudaws_network_nat_rule_update", map[string]interface{}{
		"org": "org-1", "sddc": "sddc-1", "edge_id": "edge-1", "rule_id": "nat-rule-1",
		"body":    map[string]interface{}{"protocol": "udp"},
		"confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_cloudaws_network_nat_rule_update failed: %v", err)
	}
	if gotMethod != http.MethodPut {
		t.Fatalf("expected PUT, got %s", gotMethod)
	}
	if gotPath != "/vmc/api/orgs/org-1/sddcs/sddc-1/networks/4.0/edges/edge-1/nat/config/rules/nat-rule-1" {
		t.Fatalf("unexpected path: %s", gotPath)
	}
}

func TestCloudAWSNetworkingCore_NatRuleDelete_Tier1RequiresConfirm(t *testing.T) {
	client, cleanup := cloudAWSNetCoreFixture(t, http.NewServeMux())
	defer cleanup()
	r := cloudAWSNetCoreRegistry(client, true)

	_, err := r.CallTool("vmware_cloudaws_network_nat_rule_delete", map[string]interface{}{
		"org": "org-1", "sddc": "sddc-1", "edge_id": "edge-1", "rule_id": "nat-rule-1",
	})
	if err == nil {
		t.Fatal("expected error without confirm:true, got nil")
	}
}

func TestCloudAWSNetworkingCore_MissingEdgeIDFailsBeforeAnyCall(t *testing.T) {
	called := false
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { called = true })
	client, cleanup := cloudAWSNetCoreFixture(t, mux)
	defer cleanup()
	r := cloudAWSNetCoreRegistry(client, false)

	if _, err := r.CallTool("vmware_cloudaws_network_nat_list", map[string]interface{}{"org": "org-1", "sddc": "sddc-1"}); err == nil {
		t.Fatal("expected error for missing edge_id, got nil")
	}
	if called {
		t.Fatal("handler must not reach the VMC API when a required argument is missing")
	}
}

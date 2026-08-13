package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cslsoftwares/mcpvmware/cloudaws"
)

// newCloudAWSOrgsFixture starts a fake VMC API server covering every route
// this file's tools call — there is no simulator for VMC on AWS (no account
// available to this project, see cloudaws_orgs.go's top doc comment), so
// this is a fixture unit test like appliance_test.go and
// workstation_vm_test.go, not a vcsim integration test. Handlers echo path/
// query parameters (and, for writes, the decoded body) back in the response
// wherever a test needs to assert what actually reached the wire.
func newCloudAWSOrgsFixture(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	writeJSON := func(w http.ResponseWriter, v interface{}) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(v)
	}
	decodeBody := func(r *http.Request) map[string]interface{} {
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		return body
	}

	mux.HandleFunc("GET /vmc/api/orgs", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]string{{"id": "org-1", "display_name": "Test Org"}})
	})
	mux.HandleFunc("GET /vmc/api/orgs/{org}", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]string{"id": r.PathValue("org"), "display_name": "Test Org"})
	})
	mux.HandleFunc("GET /vmc/api/orgs/{org}/reservations", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]string{{"id": "res-1"}})
	})
	mux.HandleFunc("GET /vmc/api/orgs/{org}/reservations/{reservation}/mw", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]string{"reservation": r.PathValue("reservation"), "day_of_week": "Sunday"})
	})
	mux.HandleFunc("PUT /vmc/api/orgs/{org}/reservations/{reservation}/mw", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{"reservation": r.PathValue("reservation"), "updated": true, "received": decodeBody(r)})
	})
	mux.HandleFunc("GET /vmc/api/orgs/{org}/tasks", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{"filter": r.URL.Query().Get("$filter"), "tasks": []string{"task-1"}})
	})
	mux.HandleFunc("GET /vmc/api/orgs/{org}/tasks/{task}", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]string{"id": r.PathValue("task"), "status": "FINISHED"})
	})
	mux.HandleFunc("POST /vmc/api/orgs/{org}/tasks/{task}", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]string{"id": r.PathValue("task"), "action": r.URL.Query().Get("action"), "status": "CANCELING"})
	})
	mux.HandleFunc("GET /vmc/api/orgs/{org}/providers", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []string{"AWS"})
	})
	mux.HandleFunc("GET /vmc/api/orgs/{org}/sddc-templates/{templateId}", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]string{"id": r.PathValue("templateId")})
	})
	mux.HandleFunc("DELETE /vmc/api/orgs/{org}/sddc-templates/{templateId}", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]string{"id": r.PathValue("templateId"), "deleted": "true"})
	})
	mux.HandleFunc("GET /vmc/api/orgs/{org}/sddc-templates", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]string{{"id": "tmpl-1"}})
	})
	mux.HandleFunc("GET /vmc/api/orgs/{org}/sddcs/{sddc}/sddc-template", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]string{"sddc": r.PathValue("sddc")})
	})
	mux.HandleFunc("GET /vmc/api/orgs/{org}/storage/cluster-constraints", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]string{"provider": r.URL.Query().Get("provider"), "num_hosts": r.URL.Query().Get("num_hosts")})
	})
	mux.HandleFunc("GET /vmc/api/orgs/{org}/account-link/sddc-connections", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]string{{"sddc": r.URL.Query().Get("sddc")}})
	})
	mux.HandleFunc("GET /vmc/api/orgs/{org}/account-link/compatible-subnets-async", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]string{
			"linkedAccountId": r.URL.Query().Get("linkedAccountId"),
			"region":          r.URL.Query().Get("region"),
			"sddc":            r.URL.Query().Get("sddc"),
			"task_id":         "task-async-1",
		})
	})
	mux.HandleFunc("POST /vmc/api/orgs/{org}/account-link/compatible-subnets-async", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{"task_id": "task-async-2", "received": decodeBody(r)})
	})
	mux.HandleFunc("POST /vmc/api/orgs/{org}/account-link/compatible-subnets", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]string{"result": "linked"})
	})
	mux.HandleFunc("GET /vmc/api/orgs/{org}/account-link", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]string{"url": "https://console.aws.amazon.com/link-account"})
	})
	mux.HandleFunc("DELETE /vmc/api/orgs/{org}/account-link/connected-accounts/{linkedAccountPathId}", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]string{"id": r.PathValue("linkedAccountPathId"), "force": r.URL.Query().Get("forceEvenWhenSddcPresent")})
	})
	mux.HandleFunc("POST /vmc/api/orgs/{org}/account-link/map-customer-zones", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{"task_id": "task-remap-1", "received": decodeBody(r)})
	})
	mux.HandleFunc("GET /vmc/api/orgs/{org}/account-link/connected-accounts", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]string{{"provider": r.URL.Query().Get("provider")}})
	})
	mux.HandleFunc("GET /vmc/api/orgs/{org}/subscriptions/offer-instances", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]string{
			"region":       r.URL.Query().Get("region"),
			"product_type": r.URL.Query().Get("product_type"),
		})
	})
	mux.HandleFunc("POST /vmc/api/orgs/{org}/subscriptions", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{"id": "sub-1", "received": decodeBody(r)})
	})
	mux.HandleFunc("GET /vmc/api/orgs/{org}/subscriptions/products", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []string{"product-a"})
	})
	mux.HandleFunc("GET /vmc/api/orgs/{org}/subscriptions/{subscription}", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]string{"id": r.PathValue("subscription")})
	})
	mux.HandleFunc("PUT /vmc/api/orgs/{org}/tbrs/support-window/{id}", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{"id": r.PathValue("id"), "moved": true, "received": decodeBody(r)})
	})
	mux.HandleFunc("GET /vmc/api/orgs/{org}/tbrs/support-window", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]string{{"minimum_seats_available": r.URL.Query().Get("minimumSeatsAvailable"), "created_by": r.URL.Query().Get("createdBy")}})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// newCloudAWSRegistry builds a Registry the way tools.NewRegistry is meant
// to be used for a VMC-on-AWS-only connection: client (vSphere)/
// WorkstationClient are both nil, CloudAWSClient is set, ConnectionMode
// restricts to modeCloudAWS — matching how main.go wires --cloud-aws-url.
// The CSP token-exchange endpoint is its own tiny fixture server, mirroring
// cloudaws/client_test.go's own fixture helper.
func newCloudAWSRegistry(t *testing.T, apiSrv *httptest.Server, allowDestructive bool) *Registry {
	t.Helper()
	authSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"access_token": "test-access-token", "expires_in": 1800})
	}))
	t.Cleanup(authSrv.Close)

	cloudClient, err := cloudaws.NewClient(cloudaws.Config{
		RefreshToken: "refresh-token-x",
		AuthURL:      authSrv.URL,
		BaseURL:      apiSrv.URL,
	})
	if err != nil {
		t.Fatalf("cloudaws.NewClient: %v", err)
	}
	return NewRegistry(context.Background(), nil, RegistryOptions{
		CloudAWSClient:   cloudClient,
		ConnectionMode:   ConnectionModeCloudAWS,
		AllowDestructive: allowDestructive,
	})
}

// TestCloudAWSOrgsTools_Registration proves all 29 Orgs-domain tools are
// registered under ConnectionModeCloudAWS. Not an exact-count check: the
// active ConnectionMode also registers every sibling modeCloudAWS tool from
// this fase's other 3 parallel groups (SDDCs, Networking Core, Networking
// Edge) once they exist — same posture as workstation_vm_test.go's
// registration test.
func TestCloudAWSOrgsTools_Registration(t *testing.T) {
	srv := newCloudAWSOrgsFixture(t)
	r := newCloudAWSRegistry(t, srv, true)

	want := []string{
		"vmware_cloudaws_org_list",
		"vmware_cloudaws_org_details",
		"vmware_cloudaws_reservation_list",
		"vmware_cloudaws_reservation_maintenance_window_get",
		"vmware_cloudaws_reservation_maintenance_window_update",
		"vmware_cloudaws_task_list",
		"vmware_cloudaws_task_list_filtered",
		"vmware_cloudaws_task_details",
		"vmware_cloudaws_task_action",
		"vmware_cloudaws_provider_list",
		"vmware_cloudaws_sddc_template_details",
		"vmware_cloudaws_sddc_template_delete",
		"vmware_cloudaws_sddc_template_list",
		"vmware_cloudaws_sddc_template_for_sddc",
		"vmware_cloudaws_storage_cluster_constraints_list",
		"vmware_cloudaws_account_link_sddc_connections_list",
		"vmware_cloudaws_account_link_compatible_subnets_async_get",
		"vmware_cloudaws_account_link_compatible_subnets_async_create",
		"vmware_cloudaws_account_link_compatible_subnets_calculate",
		"vmware_cloudaws_account_link_url_create",
		"vmware_cloudaws_account_link_delete",
		"vmware_cloudaws_account_link_map_customer_zones",
		"vmware_cloudaws_account_link_connected_accounts_list",
		"vmware_cloudaws_subscription_offers_list",
		"vmware_cloudaws_subscription_create",
		"vmware_cloudaws_subscription_details",
		"vmware_cloudaws_subscription_products_list",
		"vmware_cloudaws_support_window_update",
		"vmware_cloudaws_support_window_list",
	}
	if len(want) != 29 {
		t.Fatalf("test bug: want list has %d entries, expected 29", len(want))
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
	if len(got) < len(want) {
		t.Errorf("expected at least the %d Orgs tools to be registered, got %d total: %v", len(want), len(got), got)
	}
}

// TestCloudAWSOrgsTools_AllToolsSmoke drives every one of the 29 tools once
// against the fixture with a minimal set of valid arguments (confirm:true +
// AllowDestructive:true for the tier1/tier2 ones), asserting each call
// succeeds and returns non-empty JSON — a broad "ciclo feliz representativo"
// across the whole file, not just a handful of hand-picked routes.
func TestCloudAWSOrgsTools_AllToolsSmoke(t *testing.T) {
	srv := newCloudAWSOrgsFixture(t)
	r := newCloudAWSRegistry(t, srv, true)

	cases := []struct {
		name string
		args map[string]interface{}
	}{
		{"vmware_cloudaws_org_list", map[string]interface{}{}},
		{"vmware_cloudaws_org_details", map[string]interface{}{"org": "org-1"}},
		{"vmware_cloudaws_reservation_list", map[string]interface{}{"org": "org-1"}},
		{"vmware_cloudaws_reservation_maintenance_window_get", map[string]interface{}{"org": "org-1", "reservation": "res-1"}},
		{"vmware_cloudaws_reservation_maintenance_window_update", map[string]interface{}{
			"org": "org-1", "reservation": "res-1",
			"body":    map[string]interface{}{"day_of_week": "Monday"},
			"confirm": true,
		}},
		{"vmware_cloudaws_task_list", map[string]interface{}{"org": "org-1"}},
		{"vmware_cloudaws_task_list_filtered", map[string]interface{}{"org": "org-1", "filter": "status eq 'FINISHED'"}},
		{"vmware_cloudaws_task_details", map[string]interface{}{"org": "org-1", "task": "task-1"}},
		{"vmware_cloudaws_task_action", map[string]interface{}{"org": "org-1", "task": "task-1", "action": "cancel", "confirm": true}},
		{"vmware_cloudaws_provider_list", map[string]interface{}{"org": "org-1"}},
		{"vmware_cloudaws_sddc_template_details", map[string]interface{}{"org": "org-1", "template_id": "tmpl-1"}},
		{"vmware_cloudaws_sddc_template_delete", map[string]interface{}{"org": "org-1", "template_id": "tmpl-1", "confirm": true}},
		{"vmware_cloudaws_sddc_template_list", map[string]interface{}{"org": "org-1"}},
		{"vmware_cloudaws_sddc_template_for_sddc", map[string]interface{}{"org": "org-1", "sddc": "sddc-1"}},
		{"vmware_cloudaws_storage_cluster_constraints_list", map[string]interface{}{"org": "org-1", "provider": "AWS", "num_hosts": 4}},
		{"vmware_cloudaws_account_link_sddc_connections_list", map[string]interface{}{"org": "org-1"}},
		{"vmware_cloudaws_account_link_compatible_subnets_async_get", map[string]interface{}{
			"org": "org-1", "linked_account_id": "acct-1", "region": "us-west-2", "sddc": "sddc-1",
		}},
		{"vmware_cloudaws_account_link_compatible_subnets_async_create", map[string]interface{}{
			"org": "org-1", "body": map[string]interface{}{"subnetId": "subnet-1"}, "confirm": true,
		}},
		{"vmware_cloudaws_account_link_compatible_subnets_calculate", map[string]interface{}{"org": "org-1"}},
		{"vmware_cloudaws_account_link_url_create", map[string]interface{}{"org": "org-1"}},
		{"vmware_cloudaws_account_link_delete", map[string]interface{}{"org": "org-1", "linked_account_path_id": "link-1", "confirm": true}},
		{"vmware_cloudaws_account_link_map_customer_zones", map[string]interface{}{
			"org": "org-1", "body": map[string]interface{}{"zones": []interface{}{"z1"}}, "confirm": true,
		}},
		{"vmware_cloudaws_account_link_connected_accounts_list", map[string]interface{}{"org": "org-1"}},
		{"vmware_cloudaws_subscription_offers_list", map[string]interface{}{"org": "org-1", "region": "us-west-2", "product_type": "sddc"}},
		{"vmware_cloudaws_subscription_create", map[string]interface{}{
			"org": "org-1", "body": map[string]interface{}{"term": "12"}, "confirm": true,
		}},
		{"vmware_cloudaws_subscription_details", map[string]interface{}{"org": "org-1", "subscription": "sub-1"}},
		{"vmware_cloudaws_subscription_products_list", map[string]interface{}{"org": "org-1"}},
		{"vmware_cloudaws_support_window_update", map[string]interface{}{
			"org": "org-1", "support_window_id": "sw-1", "body": map[string]interface{}{"sddc": "sddc-1"}, "confirm": true,
		}},
		{"vmware_cloudaws_support_window_list", map[string]interface{}{"org": "org-1"}},
	}
	if len(cases) != 29 {
		t.Fatalf("test bug: cases has %d entries, expected 29", len(cases))
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := r.CallTool(tc.name, tc.args)
			if err != nil {
				t.Fatalf("%s failed: %v", tc.name, err)
			}
			if raw == "" {
				t.Fatalf("%s returned empty result", tc.name)
			}
			var v interface{}
			if err := json.Unmarshal([]byte(raw), &v); err != nil {
				t.Fatalf("%s returned invalid JSON %q: %v", tc.name, raw, err)
			}
		})
	}
}

// TestCloudAWSOrgsTools_RequiredArgsValidation spot-checks a handful of
// required-argument validations across different shapes (bare ID, required
// query filter, required-pair query args) — proving the handler fails
// before any HTTP round trip, not that the server rejected something.
func TestCloudAWSOrgsTools_RequiredArgsValidation(t *testing.T) {
	srv := newCloudAWSOrgsFixture(t)
	r := newCloudAWSRegistry(t, srv, true)

	cases := []struct {
		name string
		args map[string]interface{}
	}{
		{"vmware_cloudaws_org_details", map[string]interface{}{}},                                                                    // missing org
		{"vmware_cloudaws_task_list_filtered", map[string]interface{}{"org": "org-1"}},                                               // missing filter
		{"vmware_cloudaws_storage_cluster_constraints_list", map[string]interface{}{"org": "org-1", "provider": "AWS"}},              // missing num_hosts
		{"vmware_cloudaws_account_link_compatible_subnets_async_get", map[string]interface{}{"org": "org-1", "region": "us-west-2"}}, // missing linked_account_id/sddc
		{"vmware_cloudaws_subscription_offers_list", map[string]interface{}{"org": "org-1", "region": "us-west-2"}},                  // missing product_type
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := r.CallTool(tc.name, tc.args); err == nil {
				t.Fatalf("%s: expected an error for missing required argument, got nil", tc.name)
			}
		})
	}
}

// TestCloudAWSOrgsTools_SubscriptionCreate_TierGate proves the 3-layer
// destructive protection (server gate, strict confirm:true, audit — see
// destructive.go) on vmware_cloudaws_subscription_create, this file's
// highest-stakes tool: a real billing commitment, tier1 specifically because
// of that financial consequence (not because it's irreversible via the API
// alone).
func TestCloudAWSOrgsTools_SubscriptionCreate_TierGate(t *testing.T) {
	srv := newCloudAWSOrgsFixture(t)
	body := map[string]interface{}{"term": "12"}

	t.Run("gate closed denies even with confirm:true", func(t *testing.T) {
		r := newCloudAWSRegistry(t, srv, false)
		_, err := r.CallTool("vmware_cloudaws_subscription_create", map[string]interface{}{"org": "org-1", "body": body, "confirm": true})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("gate open but missing confirm denies", func(t *testing.T) {
		r := newCloudAWSRegistry(t, srv, true)
		_, err := r.CallTool("vmware_cloudaws_subscription_create", map[string]interface{}{"org": "org-1", "body": body})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("gate open, confirmed, missing body denies before any HTTP call", func(t *testing.T) {
		r := newCloudAWSRegistry(t, srv, true)
		_, err := r.CallTool("vmware_cloudaws_subscription_create", map[string]interface{}{"org": "org-1", "confirm": true})
		if err == nil {
			t.Fatal("expected error for missing body, got nil")
		}
	})

	t.Run("gate open and confirmed succeeds, body reaches VMC", func(t *testing.T) {
		r := newCloudAWSRegistry(t, srv, true)
		raw, err := r.CallTool("vmware_cloudaws_subscription_create", map[string]interface{}{"org": "org-1", "body": body, "confirm": true})
		if err != nil {
			t.Fatalf("expected success, got: %v", err)
		}
		m := decodeResult(t, raw)
		if m["id"] != "sub-1" {
			t.Fatalf("unexpected result: %s", raw)
		}
		received, ok := m["received"].(map[string]interface{})
		if !ok || received["term"] != "12" {
			t.Fatalf("expected request body to reach VMC, got: %s", raw)
		}
	})
}

// TestCloudAWSOrgsTools_AccountLinkDelete_TierGate proves the same 3-layer
// protection on vmware_cloudaws_account_link_delete, this file's other
// explicitly-flagged tier1 tool — an irreversible AWS-account unlink,
// optionally forced even while SDDCs remain on that account.
func TestCloudAWSOrgsTools_AccountLinkDelete_TierGate(t *testing.T) {
	srv := newCloudAWSOrgsFixture(t)

	t.Run("gate closed denies even with confirm:true", func(t *testing.T) {
		r := newCloudAWSRegistry(t, srv, false)
		_, err := r.CallTool("vmware_cloudaws_account_link_delete", map[string]interface{}{"org": "org-1", "linked_account_path_id": "link-1", "confirm": true})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("gate open but missing confirm denies", func(t *testing.T) {
		r := newCloudAWSRegistry(t, srv, true)
		_, err := r.CallTool("vmware_cloudaws_account_link_delete", map[string]interface{}{"org": "org-1", "linked_account_path_id": "link-1"})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("gate open and confirmed succeeds, force flag reaches VMC", func(t *testing.T) {
		r := newCloudAWSRegistry(t, srv, true)
		raw, err := r.CallTool("vmware_cloudaws_account_link_delete", map[string]interface{}{
			"org": "org-1", "linked_account_path_id": "link-1", "force_even_when_sddc_present": true, "confirm": true,
		})
		if err != nil {
			t.Fatalf("expected success, got: %v", err)
		}
		m := decodeResult(t, raw)
		if m["id"] != "link-1" || m["force"] != "true" {
			t.Fatalf("unexpected result (force flag should have reached VMC as 'true'): %s", raw)
		}
	})
}

// TestCloudAWSOrgsTools_Task_Tier2Gate covers one representative tier2 tool
// (vmware_cloudaws_task_action) end to end — same gate/confirm mechanics as
// the tier1 tests above, disruptive-but-reversible tier instead.
func TestCloudAWSOrgsTools_Task_Tier2Gate(t *testing.T) {
	srv := newCloudAWSOrgsFixture(t)

	t.Run("gate closed denies", func(t *testing.T) {
		r := newCloudAWSRegistry(t, srv, false)
		_, err := r.CallTool("vmware_cloudaws_task_action", map[string]interface{}{"org": "org-1", "task": "task-1", "action": "cancel", "confirm": true})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("gate open and confirmed succeeds, action reaches VMC", func(t *testing.T) {
		r := newCloudAWSRegistry(t, srv, true)
		raw, err := r.CallTool("vmware_cloudaws_task_action", map[string]interface{}{"org": "org-1", "task": "task-1", "action": "cancel", "confirm": true})
		if err != nil {
			t.Fatalf("expected success, got: %v", err)
		}
		m := decodeResult(t, raw)
		if m["id"] != "task-1" || m["action"] != "cancel" {
			t.Fatalf("unexpected result: %s", raw)
		}
	})
}

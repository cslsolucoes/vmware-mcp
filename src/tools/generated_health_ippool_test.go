package tools

import (
	"context"
	"testing"

	"github.com/vmware/govmomi/simulator"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// healthToolNames is the exact set registered for HealthUpdateManager by
// registerHealthIpPoolTools — kept here so TestHealthIpPoolTools_Registration
// can't silently drift from generated_health_ippool.go. Once the coordinator
// wires registerHealthIpPoolTools into registry.go's registerTools (this
// file/generated_health_ippool.go were built under this task's "do not edit
// registry.go" constraint), this list is also what mode_test.go's
// vcenterOnlyTools-style inventory should match.
var healthToolNames = []string{
	"vmware_health_query_update_infos",
	"vmware_health_add_monitored_entities",
	"vmware_health_remove_monitored_entities",
	"vmware_health_query_monitored_entities",
	"vmware_health_add_filter",
	"vmware_health_remove_filter",
	"vmware_health_query_filter_list",
	"vmware_health_query_filter_info_ids",
	"vmware_health_query_filter_entities",
	"vmware_health_post_updates",
}

// ippoolToolNames is the same kind of drift guard for the IpPoolManager
// tools registered by registerHealthIpPoolTools.
var ippoolToolNames = []string{
	"vmware_ippool_query",
	"vmware_ippool_create",
	"vmware_ippool_update",
	"vmware_ippool_destroy",
	"vmware_ippool_allocate_ipv4",
	"vmware_ippool_allocate_ipv6",
	"vmware_ippool_release_allocation",
	"vmware_ippool_query_allocations",
}

// newHealthIpPoolRegistry builds a Registry the normal way (NewRegistry,
// which already wires in every OTHER tool file via registry.go's
// registerTools) and additionally registers this file's tools via
// withClass — registry.go itself is intentionally left untouched by this
// change (task constraint: do not edit registry.go or mode_test.go).
// Mirrors generated_alarm_test.go's newAlarmRegistry for the same situation.
func newHealthIpPoolRegistry(ctx context.Context, c *vmware.Client, opts RegistryOptions) *Registry {
	r := NewRegistry(ctx, c, opts)
	r.withClass(modeVCenterOnly, registerHealthIpPoolTools)
	return r
}

// firstDatacenterPath returns the inventory path of the first datacenter
// visible to r — local to this file per the task's "all test helpers in the
// file itself" instruction (every OTHER helper here — newSimClient,
// decodeResult, assertReachesServer, marshalJSON, decodeJSONArg,
// toStringSlice — already exists package-wide and is reused as-is, same
// convention generated_alarm_test.go follows for firstVMPath).
func firstDatacenterPath(t *testing.T, r *Registry) string {
	t.Helper()
	raw, err := r.CallTool("vmware_list_datacenters", map[string]interface{}{})
	if err != nil {
		t.Fatalf("vmware_list_datacenters failed: %v", err)
	}
	list, _ := decodeResult(t, raw)["datacenters"].([]interface{})
	if len(list) == 0 {
		t.Fatal("simulator model has no datacenters")
	}
	return list[0].(string)
}

// TestHealthIpPoolTools_Registration proves all 18 HealthUpdateManager +
// IpPoolManager tools are reachable via ListTools once
// registerHealthIpPoolTools runs.
func TestHealthIpPoolTools_Registration(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newHealthIpPoolRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	if len(healthToolNames) != 10 {
		t.Fatalf("test bug: healthToolNames has %d entries, expected 10", len(healthToolNames))
	}
	if len(ippoolToolNames) != 8 {
		t.Fatalf("test bug: ippoolToolNames has %d entries, expected 8", len(ippoolToolNames))
	}

	got := map[string]bool{}
	for _, tl := range r.ListTools() {
		got[tl.Name] = true
	}
	for _, name := range append(append([]string{}, healthToolNames...), ippoolToolNames...) {
		if !got[name] {
			t.Errorf("tool %s not registered", name)
		}
	}
}

// TestHealthIpPoolTools_Validation proves each handler rejects missing/empty
// required arguments BEFORE any network round trip (so these fail even with
// the gate open and confirm:true), same convention as
// generated_alarm_test.go's TestAlarmTools_Validation.
func TestHealthIpPoolTools_Validation(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newHealthIpPoolRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	dc := firstDatacenterPath(t, r)
	entity := map[string]interface{}{"type": "HostSystem", "value": "host-1"}

	cases := []struct {
		name string
		args map[string]interface{}
		why  string
	}{
		// HealthUpdateManager.
		{"vmware_health_query_update_infos", map[string]interface{}{}, "missing provider_id"},
		{"vmware_health_add_monitored_entities", map[string]interface{}{"entities": []interface{}{entity}, "confirm": true}, "missing provider_id"},
		{"vmware_health_add_monitored_entities", map[string]interface{}{"provider_id": "p1", "confirm": true}, "missing entities"},
		{"vmware_health_add_monitored_entities", map[string]interface{}{"provider_id": "p1", "entities": []interface{}{}, "confirm": true}, "empty entities"},
		{"vmware_health_remove_monitored_entities", map[string]interface{}{"provider_id": "p1", "confirm": true}, "missing entities"},
		{"vmware_health_query_monitored_entities", map[string]interface{}{}, "missing provider_id"},
		{"vmware_health_add_filter", map[string]interface{}{"provider_id": "p1", "confirm": true}, "missing filter_name"},
		{"vmware_health_add_filter", map[string]interface{}{"filter_name": "f1", "confirm": true}, "missing provider_id"},
		{"vmware_health_remove_filter", map[string]interface{}{"confirm": true}, "missing filter_id"},
		{"vmware_health_query_filter_list", map[string]interface{}{}, "missing provider_id"},
		{"vmware_health_query_filter_info_ids", map[string]interface{}{}, "missing filter_id"},
		{"vmware_health_query_filter_entities", map[string]interface{}{}, "missing filter_id"},
		{"vmware_health_post_updates", map[string]interface{}{"provider_id": "p1", "confirm": true}, "missing updates"},
		{"vmware_health_post_updates", map[string]interface{}{"provider_id": "p1", "updates": []interface{}{}, "confirm": true}, "empty updates"},

		// IpPoolManager.
		{"vmware_ippool_query", map[string]interface{}{}, "missing dc"},
		{"vmware_ippool_create", map[string]interface{}{"dc": dc, "confirm": true}, "missing pool"},
		{"vmware_ippool_create", map[string]interface{}{"pool": map[string]interface{}{"name": "p"}, "confirm": true}, "missing dc"},
		{"vmware_ippool_update", map[string]interface{}{"dc": dc, "pool": map[string]interface{}{"name": "p"}, "confirm": true}, "missing pool.id"},
		{"vmware_ippool_destroy", map[string]interface{}{"dc": dc, "confirm": true}, "missing pool_id"},
		{"vmware_ippool_allocate_ipv4", map[string]interface{}{"dc": dc, "pool_id": float64(1), "confirm": true}, "missing allocation_id"},
		{"vmware_ippool_allocate_ipv6", map[string]interface{}{"dc": dc, "pool_id": float64(1), "confirm": true}, "missing allocation_id"},
		{"vmware_ippool_release_allocation", map[string]interface{}{"dc": dc, "pool_id": float64(1), "confirm": true}, "missing allocation_id"},
		{"vmware_ippool_query_allocations", map[string]interface{}{"dc": dc, "pool_id": float64(1)}, "missing extension_key"},
	}

	for _, tc := range cases {
		t.Run(tc.name+"/"+tc.why, func(t *testing.T) {
			if _, err := r.CallTool(tc.name, tc.args); err == nil {
				t.Fatalf("expected an error (%s) before any round trip", tc.why)
			}
		})
	}
}

// TestHealthIpPoolTools_GateAndConfirm proves the tier1/tier2 destructive
// protection is wired on every mutating tool in both families: a closed
// --allow-destructive gate denies the call, and an open gate still requires
// confirm:true. Same shape as generated_alarm_test.go's
// TestAlarmTools_GateAndConfirm.
func TestHealthIpPoolTools_GateAndConfirm(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	closed := newHealthIpPoolRegistry(context.Background(), c, RegistryOptions{})
	open := newHealthIpPoolRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	dc := firstDatacenterPath(t, open)
	entity := map[string]interface{}{"type": "HostSystem", "value": "host-1"}
	poolSpec := map[string]interface{}{"name": "gate-test-pool"}
	update := map[string]interface{}{
		"entity":             entity,
		"healthUpdateInfoId": "info-1",
		"id":                 "update-1",
		"status":             "red",
	}

	cases := []struct {
		name string
		args map[string]interface{}
	}{
		// HealthUpdateManager mutations.
		{"vmware_health_add_monitored_entities", map[string]interface{}{"provider_id": "p1", "entities": []interface{}{entity}, "confirm": true}},
		{"vmware_health_remove_monitored_entities", map[string]interface{}{"provider_id": "p1", "entities": []interface{}{entity}, "confirm": true}},
		{"vmware_health_add_filter", map[string]interface{}{"provider_id": "p1", "filter_name": "f1", "confirm": true}},
		{"vmware_health_remove_filter", map[string]interface{}{"filter_id": "filter-1", "confirm": true}},
		{"vmware_health_post_updates", map[string]interface{}{"provider_id": "p1", "updates": []interface{}{update}, "confirm": true}},

		// IpPoolManager mutations.
		{"vmware_ippool_create", map[string]interface{}{"dc": dc, "pool": poolSpec, "confirm": true}},
		{"vmware_ippool_update", map[string]interface{}{"dc": dc, "pool": map[string]interface{}{"id": float64(1), "name": "x"}, "confirm": true}},
		{"vmware_ippool_destroy", map[string]interface{}{"dc": dc, "pool_id": float64(1), "confirm": true}},
		{"vmware_ippool_allocate_ipv4", map[string]interface{}{"dc": dc, "pool_id": float64(1), "allocation_id": "a1", "confirm": true}},
		{"vmware_ippool_allocate_ipv6", map[string]interface{}{"dc": dc, "pool_id": float64(1), "allocation_id": "a1", "confirm": true}},
		{"vmware_ippool_release_allocation", map[string]interface{}{"dc": dc, "pool_id": float64(1), "allocation_id": "a1", "confirm": true}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := closed.CallTool(tc.name, tc.args); err == nil {
				t.Fatalf("%s: expected the closed destructive gate to deny the call", tc.name)
			}

			noConfirm := map[string]interface{}{}
			for k, v := range tc.args {
				if k != "confirm" {
					noConfirm[k] = v
				}
			}
			if _, err := open.CallTool(tc.name, noConfirm); err == nil {
				t.Fatalf("%s: expected an error without confirm:true", tc.name)
			}
		})
	}
}

// TestIpPoolTools_RealSuccess drives all 8 vmware_ippool_* tools through one
// real create -> query -> allocate(v4) -> allocate(v6) -> query_allocations
// -> release -> update -> destroy lifecycle against simulator.VPX(), whose
// referencia/govmomi/simulator/ip_pool_manager.go implements every one of
// these 8 methods for real (see generated_health_ippool.go's top doc
// comment) — asserting actual returned state, not just "no error".
func TestIpPoolTools_RealSuccess(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newHealthIpPoolRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	dc := firstDatacenterPath(t, r)

	// Create.
	poolSpec := map[string]interface{}{
		"name": "MCPVMWare RealSuccess Test Pool",
		"ipv4Config": map[string]interface{}{
			"subnetAddress": "192.168.77.0",
			"netmask":       "255.255.255.0",
			"gateway":       "192.168.77.1",
			"range":         "192.168.77.10#5",
		},
		"ipv6Config": map[string]interface{}{
			"subnetAddress": "2001:db8:77::",
			"netmask":       "ffff:ffff:ffff::",
			"range":         "2001:db8:77::10#5",
		},
	}
	rawCreate, err := r.CallTool("vmware_ippool_create", map[string]interface{}{"dc": dc, "pool": poolSpec, "confirm": true})
	if err != nil {
		t.Fatalf("vmware_ippool_create failed: %v", err)
	}
	created := decodeResult(t, rawCreate)
	poolIDf, ok := created["pool_id"].(float64)
	if !ok {
		t.Fatalf("vmware_ippool_create did not return a numeric pool_id: %v", created)
	}
	poolID := poolIDf

	// Query — the new pool must be present. Matched by NAME, not id: vcsim's
	// CreateIpPool (referencia/govmomi/simulator/ip_pool_manager.go) stores
	// the pool config exactly as submitted in the request
	// (m.pools[id] = NewIpPool(&req.Pool)) without backfilling req.Pool.Id to
	// the server-assigned id — only CreateIpPoolResponse.Returnval (captured
	// above as poolID) carries the real assigned id. Every subsequent call
	// (allocate/release/update/destroy) still resolves correctly by that
	// poolID because vcsim looks callers up by the m.pools map key, not by
	// the stored config's own Id field — confirmed by reading
	// ip_pool_manager.go's Allocate*/ReleaseIpAllocation/UpdateIpPool/
	// DestroyIpPool, which all key off req.PoolId/req.Pool.Id/req.Id, never
	// off pool.config.Id.
	rawQuery, err := r.CallTool("vmware_ippool_query", map[string]interface{}{"dc": dc})
	if err != nil {
		t.Fatalf("vmware_ippool_query failed: %v", err)
	}
	queried := decodeResult(t, rawQuery)
	pools, _ := queried["pools"].([]interface{})
	found := false
	for _, p := range pools {
		m, _ := p.(map[string]interface{})
		if m["name"] == "MCPVMWare RealSuccess Test Pool" {
			found = true
		}
	}
	if !found {
		t.Fatalf("vmware_ippool_query did not return the newly created pool (name match) among %d pools: %v", len(pools), pools)
	}

	// Allocate IPv4.
	rawV4, err := r.CallTool("vmware_ippool_allocate_ipv4", map[string]interface{}{"dc": dc, "pool_id": poolID, "allocation_id": "alloc-v4-1", "confirm": true})
	if err != nil {
		t.Fatalf("vmware_ippool_allocate_ipv4 failed: %v", err)
	}
	v4 := decodeResult(t, rawV4)
	ipv4, _ := v4["ip_address"].(string)
	if ipv4 == "" {
		t.Fatalf("vmware_ippool_allocate_ipv4 did not return ip_address: %v", v4)
	}

	// Allocate IPv6.
	rawV6, err := r.CallTool("vmware_ippool_allocate_ipv6", map[string]interface{}{"dc": dc, "pool_id": poolID, "allocation_id": "alloc-v6-1", "confirm": true})
	if err != nil {
		t.Fatalf("vmware_ippool_allocate_ipv6 failed: %v", err)
	}
	v6 := decodeResult(t, rawV6)
	ipv6, _ := v6["ip_address"].(string)
	if ipv6 == "" {
		t.Fatalf("vmware_ippool_allocate_ipv6 did not return ip_address: %v", v6)
	}

	// Query allocations — must find the IPv4 address allocated above under
	// the same key (vcsim's simulator keys allocations by ExtensionKey,
	// which this tool's "extension_key" argument maps to — see
	// generated_health_ippool.go's top doc comment).
	rawAlloc, err := r.CallTool("vmware_ippool_query_allocations", map[string]interface{}{"dc": dc, "pool_id": poolID, "extension_key": "alloc-v4-1"})
	if err != nil {
		t.Fatalf("vmware_ippool_query_allocations failed: %v", err)
	}
	allocResult := decodeResult(t, rawAlloc)
	allocations, _ := allocResult["allocations"].([]interface{})
	if len(allocations) == 0 {
		t.Fatalf("vmware_ippool_query_allocations returned no allocations for alloc-v4-1: %v", allocResult)
	}
	firstAlloc, _ := allocations[0].(map[string]interface{})
	if firstAlloc["ipAddress"] != ipv4 {
		t.Errorf("vmware_ippool_query_allocations: ipAddress = %v, want %v", firstAlloc["ipAddress"], ipv4)
	}

	// Release both allocations so the pool has none active (vcsim's
	// UpdateIpPool rejects updating a pool with active allocations — see
	// generated_health_ippool.go's top doc comment).
	if _, err := r.CallTool("vmware_ippool_release_allocation", map[string]interface{}{"dc": dc, "pool_id": poolID, "allocation_id": "alloc-v4-1", "confirm": true}); err != nil {
		t.Fatalf("vmware_ippool_release_allocation (v4) failed: %v", err)
	}
	if _, err := r.CallTool("vmware_ippool_release_allocation", map[string]interface{}{"dc": dc, "pool_id": poolID, "allocation_id": "alloc-v6-1", "confirm": true}); err != nil {
		t.Fatalf("vmware_ippool_release_allocation (v6) failed: %v", err)
	}

	// Update.
	updatedSpec := map[string]interface{}{
		"id":   poolID,
		"name": "MCPVMWare RealSuccess Test Pool (updated)",
		"ipv4Config": map[string]interface{}{
			"subnetAddress": "192.168.77.0",
			"netmask":       "255.255.255.0",
			"gateway":       "192.168.77.1",
			"range":         "192.168.77.10#5",
		},
	}
	if _, err := r.CallTool("vmware_ippool_update", map[string]interface{}{"dc": dc, "pool": updatedSpec, "confirm": true}); err != nil {
		t.Fatalf("vmware_ippool_update failed: %v", err)
	}
	rawQuery2, err := r.CallTool("vmware_ippool_query", map[string]interface{}{"dc": dc})
	if err != nil {
		t.Fatalf("vmware_ippool_query (post-update) failed: %v", err)
	}
	queried2 := decodeResult(t, rawQuery2)
	pools2, _ := queried2["pools"].([]interface{})
	updatedFound := false
	for _, p := range pools2 {
		m, _ := p.(map[string]interface{})
		if id, _ := m["id"].(float64); id == poolID {
			updatedFound = true
			if m["name"] != "MCPVMWare RealSuccess Test Pool (updated)" {
				t.Errorf("vmware_ippool_update did not take effect: name = %v", m["name"])
			}
		}
	}
	if !updatedFound {
		t.Fatalf("updated pool %v not found in vmware_ippool_query", poolID)
	}

	// Destroy.
	if _, err := r.CallTool("vmware_ippool_destroy", map[string]interface{}{"dc": dc, "pool_id": poolID, "confirm": true}); err != nil {
		t.Fatalf("vmware_ippool_destroy failed: %v", err)
	}
	rawQuery3, err := r.CallTool("vmware_ippool_query", map[string]interface{}{"dc": dc})
	if err != nil {
		t.Fatalf("vmware_ippool_query (post-destroy) failed: %v", err)
	}
	queried3 := decodeResult(t, rawQuery3)
	pools3, _ := queried3["pools"].([]interface{})
	for _, p := range pools3 {
		m, _ := p.(map[string]interface{})
		if id, _ := m["id"].(float64); id == poolID {
			t.Fatalf("vmware_ippool_destroy did not remove pool %v — still present in vmware_ippool_query", poolID)
		}
	}
}

// TestHealthTools_ReachesServer drives all 10 vmware_health_* tools, none of
// which have a server-side handler on vcsim (no simulator/health_update_
// manager.go exists at all — see generated_health_ippool.go's top doc
// comment): each call is expected to reach vcsim's dispatcher and come back
// with a clean server-side fault, proving the wiring — schema, gate,
// HealthUpdateManager MoRef, raw method dispatch — reaches vcsim and returns
// a clean error, not an unknown-tool wiring bug or a recovered panic. Same
// helper/rationale as generated_alarm_test.go's TestAlarmTools_ReachesServer.
func TestHealthTools_ReachesServer(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newHealthIpPoolRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	entity := map[string]interface{}{"type": "HostSystem", "value": "host-1"}
	update := map[string]interface{}{
		"entity":             entity,
		"healthUpdateInfoId": "info-1",
		"id":                 "update-1",
		"status":             "red",
	}

	cases := []struct {
		name string
		args map[string]interface{}
	}{
		{"vmware_health_query_update_infos", map[string]interface{}{"provider_id": "p1"}},
		{"vmware_health_add_monitored_entities", map[string]interface{}{"provider_id": "p1", "entities": []interface{}{entity}, "confirm": true}},
		{"vmware_health_remove_monitored_entities", map[string]interface{}{"provider_id": "p1", "entities": []interface{}{entity}, "confirm": true}},
		{"vmware_health_query_monitored_entities", map[string]interface{}{"provider_id": "p1"}},
		{"vmware_health_add_filter", map[string]interface{}{"provider_id": "p1", "filter_name": "f1", "confirm": true}},
		{"vmware_health_remove_filter", map[string]interface{}{"filter_id": "filter-1", "confirm": true}},
		{"vmware_health_query_filter_list", map[string]interface{}{"provider_id": "p1"}},
		{"vmware_health_query_filter_info_ids", map[string]interface{}{"filter_id": "filter-1"}},
		{"vmware_health_query_filter_entities", map[string]interface{}{"filter_id": "filter-1"}},
		{"vmware_health_post_updates", map[string]interface{}{"provider_id": "p1", "updates": []interface{}{update}, "confirm": true}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := r.CallTool(tc.name, tc.args)
			assertReachesServer(t, err, tc.name)
		})
	}
}

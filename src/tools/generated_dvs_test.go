package tools

import (
	"context"
	"testing"

	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25/mo"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// newDvsRegistry builds a Registry the normal way (NewRegistry, which wires
// vm.go/host.go/etc — including firstVMPath's vmware_list_vms and
// firstHostPath's vmware_list_hosts, plus generated_network.go's existing 6
// DVS/DVPG tools — via registerTools) and then manually layers this group's
// 28 DVS/DistributedVirtualSwitchManager tools on top via withClass, exactly
// as registry.go's real wiring for registerDvsTools will do once another
// change adds it there — this file must not edit registry.go itself (see
// generated_dvs.go's top doc comment and this task's constraints), matching
// generated_vm_ft_test.go's newFtRegistry precedent for the exact same
// constraint.
func newDvsRegistry(ctx context.Context, c *vmware.Client, opts RegistryOptions) *Registry {
	r := NewRegistry(ctx, c, opts)
	r.withClass(modeVCenterOnly, registerDvsTools)
	return r
}

// dvsToolNames is the exact set registered by registerDvsTools — kept here
// so TestDvsTools/Registration can't silently drift from the real
// registration list.
var dvsToolNames = []string{
	// DistributedVirtualSwitch (15)
	"vmware_dvs_perform_product_spec_operation",
	"vmware_dvs_merge",
	"vmware_dvs_add_network_resource_pool",
	"vmware_dvs_update_network_resource_pool",
	"vmware_dvs_remove_network_resource_pool",
	"vmware_dvs_enable_network_resource_management",
	"vmware_dvs_update_capability",
	"vmware_dvs_move_dvport",
	"vmware_dvs_rectify_host",
	"vmware_dvs_rollback",
	"vmware_dvs_update_health_check_config",
	"vmware_dvs_refresh_dvport_state",
	"vmware_dvs_fetch_dvport_keys",
	"vmware_dvs_lookup_dvportgroup",
	"vmware_dvs_reconfigure_vmvnic_network_resource_pool",
	// DistributedVirtualSwitchManager (13)
	"vmware_dvsmgr_query_available_dvs_spec",
	"vmware_dvsmgr_query_dvs_compatible_host_spec",
	"vmware_dvsmgr_query_compatible_host_for_new_dvs",
	"vmware_dvsmgr_query_compatible_host_for_existing_dvs",
	"vmware_dvsmgr_query_dvs_check_compatibility",
	"vmware_dvsmgr_query_compatible_vmnics_from_hosts",
	"vmware_dvsmgr_query_dvs_by_uuid",
	"vmware_dvsmgr_query_dvs_config_target",
	"vmware_dvsmgr_query_dvs_feature_capability",
	"vmware_dvsmgr_lookup_dvportgroup",
	"vmware_dvsmgr_export_entity",
	"vmware_dvsmgr_import_entity",
	"vmware_dvsmgr_rectify_dvs_on_host",
}

// dvsCase pairs a tool name with a minimal-but-fully-valid argument set
// (confirm:true already included for destructive tools) — reused by both
// TestDvsTools/GateAndConfirm (which strips "confirm" back out to prove the
// open-gate-without-confirm path) and TestDvsTools/ReachesServer.
type dvsCase struct {
	name        string
	args        map[string]interface{}
	destructive bool
}

// dvsCases covers every tool registered by registerDvsTools EXCEPT
// vmware_dvsmgr_lookup_dvportgroup, which gets its own dedicated real
// success test (TestDvsTools/ManagerLookupSuccess) instead of
// assertReachesServer — see generated_dvs.go's top doc comment: it is the
// only one of the 28 tools with genuine vcsim server-side support
// (simulator.DistributedVirtualSwitchManager.DVSManagerLookupDvPortGroup).
// "DVS0" is simulator.VPX()'s default model's pre-existing DVS (see
// generated_network_test.go's top comment); "/DC0" is its default
// datacenter (see generated_authorization_test.go's
// "/DC0/vm/does-not-exist" fixture path).
func dvsCases(host string) []dvsCase {
	return []dvsCase{
		// --- DistributedVirtualSwitch (destructive) ---
		{"vmware_dvs_perform_product_spec_operation", map[string]interface{}{"network": "DVS0", "operation": "upgrade", "confirm": true}, true},
		{"vmware_dvs_merge", map[string]interface{}{"network": "DVS0", "source_network": "DVS0", "confirm": true}, true},
		{"vmware_dvs_add_network_resource_pool", map[string]interface{}{"network": "DVS0", "config_spec": []interface{}{map[string]interface{}{"name": "rp1"}}, "confirm": true}, true},
		{"vmware_dvs_update_network_resource_pool", map[string]interface{}{"network": "DVS0", "config_spec": []interface{}{map[string]interface{}{"key": "rp1", "name": "rp1"}}, "confirm": true}, true},
		{"vmware_dvs_remove_network_resource_pool", map[string]interface{}{"network": "DVS0", "keys": []interface{}{"rp1"}, "confirm": true}, true},
		{"vmware_dvs_enable_network_resource_management", map[string]interface{}{"network": "DVS0", "enable": true, "confirm": true}, true},
		{"vmware_dvs_update_capability", map[string]interface{}{"network": "DVS0", "capability": map[string]interface{}{"dvsOperationSupported": true}, "confirm": true}, true},
		{"vmware_dvs_move_dvport", map[string]interface{}{"network": "DVS0", "port_keys": []interface{}{"0"}, "confirm": true}, true},
		{"vmware_dvs_rectify_host", map[string]interface{}{"network": "DVS0", "confirm": true}, true},
		{"vmware_dvs_rollback", map[string]interface{}{"network": "DVS0", "confirm": true}, true},
		{"vmware_dvs_update_health_check_config", map[string]interface{}{"network": "DVS0", "health_check_config": []interface{}{map[string]interface{}{"type": "vlanMtu", "enable": true, "interval": 1}}, "confirm": true}, true},
		{"vmware_dvs_refresh_dvport_state", map[string]interface{}{"network": "DVS0", "confirm": true}, true},
		{"vmware_dvs_reconfigure_vmvnic_network_resource_pool", map[string]interface{}{"network": "DVS0", "config_spec": []interface{}{map[string]interface{}{"operation": "add", "name": "vnicrp1"}}, "confirm": true}, true},
		// --- DistributedVirtualSwitch (read-only) ---
		{"vmware_dvs_fetch_dvport_keys", map[string]interface{}{"network": "DVS0"}, false},
		{"vmware_dvs_lookup_dvportgroup", map[string]interface{}{"network": "DVS0", "portgroup_key": "0"}, false},
		// --- DistributedVirtualSwitchManager (read-only) ---
		{"vmware_dvsmgr_query_available_dvs_spec", map[string]interface{}{}, false},
		{"vmware_dvsmgr_query_dvs_compatible_host_spec", map[string]interface{}{}, false},
		{"vmware_dvsmgr_query_compatible_host_for_new_dvs", map[string]interface{}{"container_path": "/DC0"}, false},
		{"vmware_dvsmgr_query_compatible_host_for_existing_dvs", map[string]interface{}{"container_path": "/DC0", "network": "DVS0"}, false},
		{"vmware_dvsmgr_query_dvs_check_compatibility", map[string]interface{}{"container_path": "/DC0"}, false},
		{"vmware_dvsmgr_query_compatible_vmnics_from_hosts", map[string]interface{}{"network": "DVS0", "host_names": []interface{}{host}}, false},
		{"vmware_dvsmgr_query_dvs_by_uuid", map[string]interface{}{"uuid": "bogus-uuid"}, false},
		{"vmware_dvsmgr_query_dvs_config_target", map[string]interface{}{}, false},
		{"vmware_dvsmgr_query_dvs_feature_capability", map[string]interface{}{}, false},
		{"vmware_dvsmgr_export_entity", map[string]interface{}{"selections": []interface{}{map[string]interface{}{"kind": "dvs", "dvs_uuid": "bogus-uuid"}}}, false},
		// --- DistributedVirtualSwitchManager (destructive) ---
		{"vmware_dvsmgr_import_entity", map[string]interface{}{"entity_backup": []interface{}{map[string]interface{}{"entityType": "distributedVirtualSwitch", "configBlob": "aGVsbG8="}}, "import_type": "createEntityWithNewIdentifier", "confirm": true}, true},
		{"vmware_dvsmgr_rectify_dvs_on_host", map[string]interface{}{"host_names": []interface{}{host}, "confirm": true}, true},
	}
}

// TestDvsTools drives every tool registered by registerDvsTools against a
// SINGLE vcsim server (simulator.VPX(), started once via newSimClient and
// reused by every t.Run below) — creating more than one in-process vcsim
// server in this file would exhaust Windows's ephemeral port range and hang
// the test run, per this task's explicit constraint.
func TestDvsTools(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	t.Run("Registration", func(t *testing.T) {
		r := newDvsRegistry(context.Background(), c, RegistryOptions{})

		if len(dvsToolNames) != 28 {
			t.Fatalf("test bug: dvsToolNames has %d entries, expected 28", len(dvsToolNames))
		}

		got := map[string]bool{}
		for _, tl := range r.ListTools() {
			got[tl.Name] = true
		}
		for _, name := range dvsToolNames {
			if !got[name] {
				t.Errorf("tool %s not registered", name)
			}
		}
	})

	t.Run("Validation", func(t *testing.T) {
		r := newDvsRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

		cases := []struct {
			name string
			args map[string]interface{}
			why  string
		}{
			{"vmware_dvs_perform_product_spec_operation", map[string]interface{}{"network": "DVS0", "confirm": true}, "missing operation"},
			{"vmware_dvs_merge", map[string]interface{}{"network": "DVS0", "confirm": true}, "missing source_network"},
			{"vmware_dvs_add_network_resource_pool", map[string]interface{}{"network": "DVS0", "confirm": true}, "missing config_spec"},
			{"vmware_dvs_update_network_resource_pool", map[string]interface{}{"network": "DVS0", "confirm": true}, "missing config_spec"},
			{"vmware_dvs_remove_network_resource_pool", map[string]interface{}{"network": "DVS0", "confirm": true}, "missing keys"},
			{"vmware_dvs_enable_network_resource_management", map[string]interface{}{"network": "DVS0", "confirm": true}, "missing enable"},
			{"vmware_dvs_update_capability", map[string]interface{}{"network": "DVS0", "confirm": true}, "missing capability"},
			{"vmware_dvs_move_dvport", map[string]interface{}{"network": "DVS0", "confirm": true}, "missing port_keys"},
			{"vmware_dvs_rectify_host", map[string]interface{}{"confirm": true}, "missing network"},
			{"vmware_dvs_rollback", map[string]interface{}{"confirm": true}, "missing network"},
			{"vmware_dvs_update_health_check_config", map[string]interface{}{"network": "DVS0", "confirm": true}, "missing health_check_config"},
			{"vmware_dvs_refresh_dvport_state", map[string]interface{}{"confirm": true}, "missing network"},
			{"vmware_dvs_reconfigure_vmvnic_network_resource_pool", map[string]interface{}{"network": "DVS0", "confirm": true}, "missing config_spec"},
			{"vmware_dvs_fetch_dvport_keys", map[string]interface{}{}, "missing network"},
			{"vmware_dvs_lookup_dvportgroup", map[string]interface{}{"network": "DVS0"}, "missing portgroup_key"},
			{"vmware_dvsmgr_query_compatible_host_for_new_dvs", map[string]interface{}{}, "missing container_path"},
			{"vmware_dvsmgr_query_compatible_host_for_existing_dvs", map[string]interface{}{"container_path": "/DC0"}, "missing network"},
			{"vmware_dvsmgr_query_dvs_check_compatibility", map[string]interface{}{}, "missing container_path"},
			{"vmware_dvsmgr_query_compatible_vmnics_from_hosts", map[string]interface{}{"network": "DVS0"}, "missing host_names"},
			{"vmware_dvsmgr_query_dvs_by_uuid", map[string]interface{}{}, "missing uuid"},
			{"vmware_dvsmgr_lookup_dvportgroup", map[string]interface{}{"switch_uuid": "x"}, "missing portgroup_key"},
			{"vmware_dvsmgr_export_entity", map[string]interface{}{}, "missing selections"},
			{"vmware_dvsmgr_import_entity", map[string]interface{}{"confirm": true}, "missing entity_backup"},
			{"vmware_dvsmgr_rectify_dvs_on_host", map[string]interface{}{"confirm": true}, "missing host_names"},
		}

		for _, tc := range cases {
			t.Run(tc.name+"/"+tc.why, func(t *testing.T) {
				if _, err := r.CallTool(tc.name, tc.args); err == nil {
					t.Fatalf("expected an error (%s) before any round trip", tc.why)
				}
			})
		}
	})

	t.Run("GateAndConfirm", func(t *testing.T) {
		host := firstHostPath(t, newDvsRegistry(context.Background(), c, RegistryOptions{}))

		closed := newDvsRegistry(context.Background(), c, RegistryOptions{})
		open := newDvsRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

		for _, tc := range dvsCases(host) {
			if !tc.destructive {
				continue
			}
			t.Run(tc.name, func(t *testing.T) {
				if _, err := closed.CallTool(tc.name, tc.args); err == nil {
					t.Fatalf("%s: expected the closed destructive gate to deny the call", tc.name)
				}

				withoutConfirm := map[string]interface{}{}
				for k, v := range tc.args {
					if k != "confirm" {
						withoutConfirm[k] = v
					}
				}
				if _, err := open.CallTool(tc.name, withoutConfirm); err == nil {
					t.Fatalf("%s: expected an error without confirm:true", tc.name)
				}
			})
		}
	})

	t.Run("ReachesServer", func(t *testing.T) {
		r := newDvsRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
		host := firstHostPath(t, r)

		for _, tc := range dvsCases(host) {
			t.Run(tc.name, func(t *testing.T) {
				_, err := r.CallTool(tc.name, tc.args)
				assertReachesServer(t, err, tc.name)
			})
		}
	})

	// TestDvsTools/ManagerLookupSuccess proves vmware_dvsmgr_lookup_dvportgroup
	// actually works end-to-end against vcsim: simulator.DistributedVirtualSwitchManager
	// is the ONLY one of this file's 28 tools with a real server-side handler
	// (DVSManagerLookupDvPortGroup — see generated_dvs.go's top doc comment
	// and simulator/dvs_manager.go). It creates a real portgroup via the
	// EXISTING vmware_dvs_add_portgroup tool (generated_network.go, already
	// wired into NewRegistry via registry.go), then reads that portgroup's
	// real Key and its parent DVS's real Uuid straight off vcsim via a
	// Properties() read — the exact same "key" + "config.distributedVirtualSwitch"
	// / "uuid" property paths object.DistributedVirtualPortgroup.EthernetCardBackingInfo
	// uses for this identical switchUuid+portgroupKey pairing (confirmed by
	// reading referencia/govmomi/object/distributed_virtual_portgroup.go) —
	// then calls the tool with that real pair and expects a genuine,
	// non-nil "portgroup" result, not just "an error came back clean".
	t.Run("ManagerLookupSuccess", func(t *testing.T) {
		r := newDvsRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
		ctx := context.Background()

		const pgName = "pg-dvsmgr-lookup-fixture"
		if _, err := r.CallTool("vmware_dvs_add_portgroup", map[string]interface{}{
			"network": "DVS0",
			"spec":    []interface{}{map[string]interface{}{"name": pgName, "numPorts": 4}},
			"confirm": true,
		}); err != nil {
			t.Fatalf("fixture vmware_dvs_add_portgroup failed: %v", err)
		}

		dvpg, err := resolveDVPG(ctx, c, map[string]interface{}{"network": pgName})
		if err != nil {
			t.Fatalf("failed to resolve fixture portgroup %q: %v", pgName, err)
		}
		var dvpgProps mo.DistributedVirtualPortgroup
		if err := dvpg.Properties(ctx, dvpg.Reference(), []string{"key", "config.distributedVirtualSwitch"}, &dvpgProps); err != nil {
			t.Fatalf("failed to read fixture portgroup properties: %v", err)
		}
		if dvpgProps.Config.DistributedVirtualSwitch == nil {
			t.Fatal("fixture portgroup has no parent DistributedVirtualSwitch reference")
		}
		var dvsProps mo.DistributedVirtualSwitch
		if err := dvpg.Properties(ctx, *dvpgProps.Config.DistributedVirtualSwitch, []string{"uuid"}, &dvsProps); err != nil {
			t.Fatalf("failed to read parent DVS uuid: %v", err)
		}

		raw, err := r.CallTool("vmware_dvsmgr_lookup_dvportgroup", map[string]interface{}{
			"switch_uuid":   dvsProps.Uuid,
			"portgroup_key": dvpgProps.Key,
		})
		if err != nil {
			t.Fatalf("vmware_dvsmgr_lookup_dvportgroup failed against a real switch_uuid/portgroup_key pair: %v (%s)", err, raw)
		}
		m := decodeResult(t, raw)
		if found, _ := m["found"].(bool); !found {
			t.Fatalf("expected found:true for a real portgroup, got: %s", raw)
		}
		if m["portgroup"] == nil {
			t.Fatalf("expected a non-nil portgroup reference, got: %s", raw)
		}
	})

	// TestDvsTools/WrongKindResolution proves dvsResolve fails cleanly (not a
	// panic) when "network" names something that resolves but isn't a
	// DistributedVirtualSwitch — same check generated_network_test.go's
	// TestNetworkTools_WrongKindResolution already does for the DVS tools in
	// that file, exercised here for this file's dvsResolve wrapper.
	t.Run("WrongKindResolution", func(t *testing.T) {
		r := newDvsRegistry(context.Background(), c, RegistryOptions{})

		if _, err := r.CallTool("vmware_dvs_fetch_dvport_keys", map[string]interface{}{
			"network": "VM Network",
		}); err == nil {
			t.Fatal("expected vmware_dvs_fetch_dvport_keys against a plain Network to fail with a clear type error")
		}
	})
}

package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/vmware/govmomi/simulator"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// newVsanRegistry builds a Registry the normal way (NewRegistry, which wires
// vm.go/host.go/generated_host_misc.go/etc via registerTools) and then
// manually layers this group's tools on top via withClass, exactly as
// registry.go's real wiring for registerVsanTools will do once another
// change adds it there — this file must not edit registry.go itself (see
// generated_vsan.go's top doc comment / this task's constraints).
func newVsanRegistry(ctx context.Context, c *vmware.Client, opts RegistryOptions) *Registry {
	r := NewRegistry(ctx, c, opts)
	r.withClass(modeVSphereGeneral, registerVsanTools)
	return r
}

// vsanToolNames is the exact set registered by registerVsanTools — kept here
// so TestVsanTools_Registration can't silently drift from the real
// registration list, and so it can't collide with generated_host_misc.go's
// 4 pre-existing vmware_host_vsan* tools (vmware_host_vsan_update,
// vmware_host_vsan_internal_query_object_uuids,
// vmware_host_vsan_internal_get_obj_ext_attrs,
// vmware_host_vsan_internal_delete_objects), which this file intentionally
// does not reimplement.
var vsanToolNames = []string{
	// HostVsanSystem
	"vmware_host_vsan_query_host_status",
	"vmware_host_vsan_query_disks_for_vsan",
	"vmware_host_vsan_add_disks",
	"vmware_host_vsan_initialize_disks",
	"vmware_host_vsan_remove_disk",
	"vmware_host_vsan_remove_disk_mapping",
	"vmware_host_vsan_unmount_disk_mapping",
	"vmware_host_vsan_evacuate_node",
	"vmware_host_vsan_recommission_node",
	// HostVsanInternalSystem
	"vmware_host_vsan_internal_query_objects",
	"vmware_host_vsan_internal_query_objects_on_physical_disk",
	"vmware_host_vsan_internal_query_statistics",
	"vmware_host_vsan_internal_query_cmmds",
	"vmware_host_vsan_internal_query_syncing_objects",
	"vmware_host_vsan_internal_query_physical_disks",
	"vmware_host_vsan_internal_reconfigure_dom_object",
	"vmware_host_vsan_internal_abdicate_dom_ownership",
	"vmware_host_vsan_internal_run_disk_diagnostics",
	"vmware_host_vsan_internal_upgrade_objects",
}

// assertVsanReachesServer proves a vsan tool call reached real vcsim/govmomi
// plumbing (arg parsing, host resolution, ConfigManager().VsanSystem(ctx)/
// VsanInternalSystem(ctx), and the raw SOAP dispatch) instead of failing on
// something wired wrong in this file. err is expected to be non-nil — see
// generated_vsan.go's top doc comment for why vcsim cannot succeed against
// any of these 19 methods (HostVsanSystem/HostVsanInternalSystem's
// ConfigManager MoRef is a template placeholder never registered as a real
// vcsim object, so every real SOAP call faults "managed object not found").
// Defined locally in this file (not reused from
// generated_host_misc_test.go's assertCleanFailure /
// generated_vm_lifecycle_test.go's assertReachesServer) per this task's
// "every helper the test needs lives in the test file itself" requirement.
func assertVsanReachesServer(t *testing.T, err error, tool string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s: expected an error (vcsim has no real HostVsanSystem/HostVsanInternalSystem object behind the ConfigManager placeholder MoRef — see generated_vsan.go's top doc comment) — got success instead", tool)
	}
	msg := err.Error()
	if strings.Contains(msg, "unknown tool") {
		t.Fatalf("%s: tool is not registered: %v", tool, err)
	}
	if strings.Contains(msg, "panicked") {
		t.Fatalf("%s: handler panicked instead of returning a clean error: %v", tool, err)
	}
	t.Logf("%s: real vcsim call returned (expected): %v", tool, err)
}

// TestVsan runs every subtest against a single shared vcsim client
// (simulator.ESX(), matching generated_host_misc.go's classification of
// HostVsanSystem/HostVsanInternalSystem as host-level, not vCenter-only) —
// one newSimClient/defer cleanup for the whole test, per this task's
// constraint against standing up multiple simulators in one test run.
func TestVsan(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	openGate := newVsanRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	closedGate := newVsanRegistry(context.Background(), c, RegistryOptions{AllowDestructive: false})
	host := firstHostPath(t, openGate)

	t.Run("Registration", func(t *testing.T) {
		if len(vsanToolNames) != 19 {
			t.Fatalf("test bug: vsanToolNames has %d entries, expected 19", len(vsanToolNames))
		}
		got := map[string]bool{}
		for _, tl := range openGate.ListTools() {
			got[tl.Name] = true
		}
		for _, name := range vsanToolNames {
			if !got[name] {
				t.Errorf("tool %s not registered", name)
			}
		}
		// Must not collide with generated_host_misc.go's pre-existing vsan
		// tools — proves this file didn't accidentally reimplement them.
		preexisting := []string{
			"vmware_host_vsan_update",
			"vmware_host_vsan_internal_query_object_uuids",
			"vmware_host_vsan_internal_get_obj_ext_attrs",
			"vmware_host_vsan_internal_delete_objects",
		}
		for _, name := range preexisting {
			if !got[name] {
				t.Errorf("expected pre-existing tool %s to still be registered by NewRegistry", name)
			}
			for _, own := range vsanToolNames {
				if own == name {
					t.Errorf("vsanToolNames must not duplicate pre-existing tool %s", name)
				}
			}
		}
	})

	t.Run("Validation", func(t *testing.T) {
		cases := []struct {
			name string
			args map[string]interface{}
			why  string
		}{
			{"vmware_host_vsan_query_host_status", map[string]interface{}{}, "missing host"},
			{"vmware_host_vsan_add_disks", map[string]interface{}{"host": host, "confirm": true}, "missing disks"},
			{"vmware_host_vsan_add_disks", map[string]interface{}{"host": host, "disks": []interface{}{}, "confirm": true}, "empty disks"},
			{"vmware_host_vsan_add_disks", map[string]interface{}{"host": host, "disks": "not-an-array", "confirm": true}, "disks not an array"},
			{"vmware_host_vsan_initialize_disks", map[string]interface{}{"host": host, "confirm": true}, "missing mappings"},
			{"vmware_host_vsan_initialize_disks", map[string]interface{}{"host": host, "mappings": []interface{}{}, "confirm": true}, "empty mappings"},
			{"vmware_host_vsan_remove_disk", map[string]interface{}{"host": host, "confirm": true}, "missing disks"},
			{"vmware_host_vsan_remove_disk_mapping", map[string]interface{}{"host": host, "confirm": true}, "missing mappings"},
			{"vmware_host_vsan_unmount_disk_mapping", map[string]interface{}{"host": host, "confirm": true}, "missing mappings"},
			{"vmware_host_vsan_internal_query_objects_on_physical_disk", map[string]interface{}{"host": host}, "missing disks"},
			{"vmware_host_vsan_internal_query_objects_on_physical_disk", map[string]interface{}{"host": host, "disks": []interface{}{}}, "empty disks"},
			{"vmware_host_vsan_internal_query_statistics", map[string]interface{}{"host": host}, "missing labels"},
			{"vmware_host_vsan_internal_query_cmmds", map[string]interface{}{"host": host}, "missing queries"},
			{"vmware_host_vsan_internal_query_cmmds", map[string]interface{}{"host": host, "queries": []interface{}{}}, "empty queries"},
			{"vmware_host_vsan_internal_reconfigure_dom_object", map[string]interface{}{"host": host, "policy": "p", "confirm": true}, "missing uuid"},
			{"vmware_host_vsan_internal_reconfigure_dom_object", map[string]interface{}{"host": host, "uuid": "u", "confirm": true}, "missing policy"},
			{"vmware_host_vsan_internal_abdicate_dom_ownership", map[string]interface{}{"host": host, "confirm": true}, "missing uuids"},
			{"vmware_host_vsan_internal_upgrade_objects", map[string]interface{}{"host": host, "confirm": true}, "missing uuids"},
			{"vmware_host_vsan_internal_upgrade_objects", map[string]interface{}{"host": host, "uuids": []interface{}{"u1"}, "confirm": true}, "missing new_version"},
		}
		for _, tc := range cases {
			t.Run(tc.name+"/"+tc.why, func(t *testing.T) {
				if _, err := openGate.CallTool(tc.name, tc.args); err == nil {
					t.Fatalf("expected an error (%s) before any round trip", tc.why)
				}
			})
		}
	})

	t.Run("GateAndConfirm", func(t *testing.T) {
		validDisk := map[string]interface{}{"deviceName": "/vmfs/devices/disks/naa.001", "canonicalName": "naa.001"}
		validMapping := map[string]interface{}{"ssd": validDisk, "nonSsd": []interface{}{validDisk}}

		cases := []struct {
			name string
			args map[string]interface{}
		}{
			{"vmware_host_vsan_add_disks", map[string]interface{}{"host": host, "disks": []interface{}{validDisk}}},
			{"vmware_host_vsan_initialize_disks", map[string]interface{}{"host": host, "mappings": []interface{}{validMapping}}},
			{"vmware_host_vsan_remove_disk", map[string]interface{}{"host": host, "disks": []interface{}{validDisk}}},
			{"vmware_host_vsan_remove_disk_mapping", map[string]interface{}{"host": host, "mappings": []interface{}{validMapping}}},
			{"vmware_host_vsan_unmount_disk_mapping", map[string]interface{}{"host": host, "mappings": []interface{}{validMapping}}},
			{"vmware_host_vsan_evacuate_node", map[string]interface{}{"host": host}},
			{"vmware_host_vsan_recommission_node", map[string]interface{}{"host": host}},
			{"vmware_host_vsan_internal_reconfigure_dom_object", map[string]interface{}{"host": host, "uuid": "u1", "policy": "p"}},
			{"vmware_host_vsan_internal_abdicate_dom_ownership", map[string]interface{}{"host": host, "uuids": []interface{}{"u1"}}},
			{"vmware_host_vsan_internal_run_disk_diagnostics", map[string]interface{}{"host": host}},
			{"vmware_host_vsan_internal_upgrade_objects", map[string]interface{}{"host": host, "uuids": []interface{}{"u1"}, "new_version": float64(3)}},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				withConfirm := map[string]interface{}{}
				for k, v := range tc.args {
					withConfirm[k] = v
				}
				withConfirm["confirm"] = true

				if _, err := closedGate.CallTool(tc.name, withConfirm); err == nil {
					t.Fatalf("%s: expected the closed destructive gate to deny the call", tc.name)
				}
				if _, err := openGate.CallTool(tc.name, tc.args); err == nil {
					t.Fatalf("%s: expected an error without confirm:true", tc.name)
				}
			})
		}
	})

	t.Run("ReachesServer", func(t *testing.T) {
		validDisk := map[string]interface{}{"deviceName": "/vmfs/devices/disks/naa.001", "canonicalName": "naa.001"}
		validMapping := map[string]interface{}{"ssd": validDisk, "nonSsd": []interface{}{validDisk}}

		cases := []struct {
			name string
			args map[string]interface{}
		}{
			// HostVsanSystem: reads
			{"vmware_host_vsan_query_host_status", map[string]interface{}{"host": host}},
			{"vmware_host_vsan_query_disks_for_vsan", map[string]interface{}{"host": host}},
			{"vmware_host_vsan_query_disks_for_vsan", map[string]interface{}{"host": host, "canonical_names": []interface{}{"naa.001"}}},
			// HostVsanSystem: tier2
			{"vmware_host_vsan_add_disks", map[string]interface{}{"host": host, "disks": []interface{}{validDisk}, "confirm": true}},
			{"vmware_host_vsan_initialize_disks", map[string]interface{}{"host": host, "mappings": []interface{}{validMapping}, "confirm": true}},
			{"vmware_host_vsan_remove_disk", map[string]interface{}{"host": host, "disks": []interface{}{validDisk}, "confirm": true}},
			{"vmware_host_vsan_remove_disk", map[string]interface{}{"host": host, "disks": []interface{}{validDisk}, "maintenance_spec": map[string]interface{}{"purpose": "vsanUpgrade"}, "timeout": float64(60), "confirm": true}},
			{"vmware_host_vsan_remove_disk_mapping", map[string]interface{}{"host": host, "mappings": []interface{}{validMapping}, "confirm": true}},
			{"vmware_host_vsan_unmount_disk_mapping", map[string]interface{}{"host": host, "mappings": []interface{}{validMapping}, "confirm": true}},
			{"vmware_host_vsan_evacuate_node", map[string]interface{}{"host": host, "confirm": true}},
			{"vmware_host_vsan_evacuate_node", map[string]interface{}{"host": host, "maintenance_spec": map[string]interface{}{"purpose": "vsanUpgrade"}, "timeout": float64(300), "confirm": true}},
			{"vmware_host_vsan_recommission_node", map[string]interface{}{"host": host, "confirm": true}},
			// HostVsanInternalSystem: reads
			{"vmware_host_vsan_internal_query_objects", map[string]interface{}{"host": host}},
			{"vmware_host_vsan_internal_query_objects", map[string]interface{}{"host": host, "uuids": []interface{}{"11111111-1111-1111-1111-111111111111"}}},
			{"vmware_host_vsan_internal_query_objects_on_physical_disk", map[string]interface{}{"host": host, "disks": []interface{}{"22222222-2222-2222-2222-222222222222"}}},
			{"vmware_host_vsan_internal_query_statistics", map[string]interface{}{"host": host, "labels": []interface{}{"dom"}}},
			{"vmware_host_vsan_internal_query_cmmds", map[string]interface{}{"host": host, "queries": []interface{}{map[string]interface{}{"type": "DOM_OBJECT"}}}},
			{"vmware_host_vsan_internal_query_syncing_objects", map[string]interface{}{"host": host}},
			{"vmware_host_vsan_internal_query_syncing_objects", map[string]interface{}{"host": host, "uuids": []interface{}{"11111111-1111-1111-1111-111111111111"}}},
			{"vmware_host_vsan_internal_query_physical_disks", map[string]interface{}{"host": host}},
			{"vmware_host_vsan_internal_query_physical_disks", map[string]interface{}{"host": host, "props": []interface{}{"capacity"}}},
			// HostVsanInternalSystem: tier2
			{"vmware_host_vsan_internal_reconfigure_dom_object", map[string]interface{}{"host": host, "uuid": "11111111-1111-1111-1111-111111111111", "policy": "(\"hostFailuresToTolerate\" i1)", "confirm": true}},
			{"vmware_host_vsan_internal_abdicate_dom_ownership", map[string]interface{}{"host": host, "uuids": []interface{}{"11111111-1111-1111-1111-111111111111"}, "confirm": true}},
			{"vmware_host_vsan_internal_run_disk_diagnostics", map[string]interface{}{"host": host, "confirm": true}},
			{"vmware_host_vsan_internal_run_disk_diagnostics", map[string]interface{}{"host": host, "disks": []interface{}{"22222222-2222-2222-2222-222222222222"}, "confirm": true}},
			{"vmware_host_vsan_internal_upgrade_objects", map[string]interface{}{"host": host, "uuids": []interface{}{"11111111-1111-1111-1111-111111111111"}, "new_version": float64(3), "confirm": true}},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				_, err := openGate.CallTool(tc.name, tc.args)
				assertVsanReachesServer(t, err, tc.name)
			})
		}
	})
}

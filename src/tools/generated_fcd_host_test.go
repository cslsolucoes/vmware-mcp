package tools

import (
	"context"
	"testing"

	"github.com/vmware/govmomi/simulator"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// newFcdHostRegistry builds a Registry the normal way (NewRegistry, which
// wires vm.go/host.go/datastore.go/etc via registerTools) and then manually
// layers this group's tools on top via withClass — same pattern
// generated_vm_ft_test.go's newFtRegistry and generated_datastore_browser_test.go's
// newDatastoreBrowserRegistry use, and for the same reason: registry.go
// itself must not be edited by this file (see generated_fcd_host.go's top
// doc comment / this task's constraints).
func newFcdHostRegistry(ctx context.Context, c *vmware.Client, opts RegistryOptions) *Registry {
	r := NewRegistry(ctx, c, opts)
	r.withClass(modeVSphereGeneral, registerFcdHostTools)
	return r
}

// fcdHostToolNames is the exact set registered by registerFcdHostTools —
// kept here so TestFcdHostTools_Registration can't silently drift from the
// real registration list.
var fcdHostToolNames = []string{
	"vmware_fcdhost_create_disk",
	"vmware_fcdhost_register_disk",
	"vmware_fcdhost_extend_disk",
	"vmware_fcdhost_inflate_disk",
	"vmware_fcdhost_rename",
	"vmware_fcdhost_delete",
	"vmware_fcdhost_get",
	"vmware_fcdhost_get_state",
	"vmware_fcdhost_list",
	"vmware_fcdhost_clone",
	"vmware_fcdhost_relocate",
	"vmware_fcdhost_set_control_flags",
	"vmware_fcdhost_clear_control_flags",
	"vmware_fcdhost_reconcile_datastore_inventory",
	"vmware_fcdhost_schedule_reconcile_datastore_inventory",
	"vmware_fcdhost_create_snapshot",
	"vmware_fcdhost_delete_snapshot",
	"vmware_fcdhost_get_snapshot_info",
}

// fcdHostDestructiveToolNames is the tier1/tier2 subset of fcdHostToolNames
// (14 of 18 — the other 4, get/get_state/list/get_snapshot_info, are plain
// r.register reads) — used by TestFcdHostTools_GateAndConfirm.
var fcdHostDestructiveToolNames = []string{
	"vmware_fcdhost_create_disk",
	"vmware_fcdhost_register_disk",
	"vmware_fcdhost_extend_disk",
	"vmware_fcdhost_inflate_disk",
	"vmware_fcdhost_rename",
	"vmware_fcdhost_delete",
	"vmware_fcdhost_clone",
	"vmware_fcdhost_relocate",
	"vmware_fcdhost_set_control_flags",
	"vmware_fcdhost_clear_control_flags",
	"vmware_fcdhost_reconcile_datastore_inventory",
	"vmware_fcdhost_schedule_reconcile_datastore_inventory",
	"vmware_fcdhost_create_snapshot",
	"vmware_fcdhost_delete_snapshot",
}

// TestFcdHostTools_Registration proves all 18 host-level FCD tools are
// wired through newFcdHostRegistry's withClass call and reachable via
// ListTools.
func TestFcdHostTools_Registration(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newFcdHostRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	if len(fcdHostToolNames) != 18 {
		t.Fatalf("test bug: fcdHostToolNames has %d entries, expected 18", len(fcdHostToolNames))
	}
	if len(fcdHostDestructiveToolNames) != 14 {
		t.Fatalf("test bug: fcdHostDestructiveToolNames has %d entries, expected 14", len(fcdHostDestructiveToolNames))
	}

	got := map[string]bool{}
	for _, tl := range r.ListTools() {
		got[tl.Name] = true
	}
	for _, name := range fcdHostToolNames {
		if !got[name] {
			t.Errorf("tool %s not registered", name)
		}
	}
}

// TestFcdHostTools_Validation proves each handler rejects missing/empty
// required arguments BEFORE any network round trip (so these fail even with
// the gate open and confirm:true).
func TestFcdHostTools_Validation(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newFcdHostRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	host := firstHostPath(t, r)
	ds := firstDatastorePath(t, r)
	const id = "52e14b1e-0000-0000-0000-000000000001"

	cases := []struct {
		name string
		args map[string]interface{}
		why  string
	}{
		{"vmware_fcdhost_create_disk", map[string]interface{}{"host": host, "datastore": ds, "capacity_mb": 1024, "confirm": true}, "missing name"},
		{"vmware_fcdhost_create_disk", map[string]interface{}{"host": host, "datastore": ds, "name": "d1", "confirm": true}, "missing capacity_mb"},
		{"vmware_fcdhost_create_disk", map[string]interface{}{"host": host, "name": "d1", "capacity_mb": 1024, "confirm": true}, "missing datastore"},
		{"vmware_fcdhost_register_disk", map[string]interface{}{"host": host, "confirm": true}, "missing path"},
		{"vmware_fcdhost_extend_disk", map[string]interface{}{"host": host, "datastore": ds, "new_capacity_mb": 2048, "confirm": true}, "missing id"},
		{"vmware_fcdhost_extend_disk", map[string]interface{}{"host": host, "id": id, "datastore": ds, "confirm": true}, "missing new_capacity_mb"},
		{"vmware_fcdhost_inflate_disk", map[string]interface{}{"host": host, "datastore": ds, "confirm": true}, "missing id"},
		{"vmware_fcdhost_rename", map[string]interface{}{"host": host, "id": id, "datastore": ds, "confirm": true}, "missing name"},
		{"vmware_fcdhost_delete", map[string]interface{}{"host": host, "datastore": ds, "confirm": true}, "missing id"},
		{"vmware_fcdhost_get", map[string]interface{}{"host": host, "datastore": ds}, "missing id"},
		{"vmware_fcdhost_get_state", map[string]interface{}{"host": host, "id": id}, "missing datastore"},
		{"vmware_fcdhost_list", map[string]interface{}{"host": host}, "missing datastore"},
		{"vmware_fcdhost_clone", map[string]interface{}{"host": host, "id": id, "datastore": ds, "target_datastore": ds, "confirm": true}, "missing name"},
		{"vmware_fcdhost_clone", map[string]interface{}{"host": host, "id": id, "datastore": ds, "name": "clone1", "confirm": true}, "missing target_datastore"},
		{"vmware_fcdhost_relocate", map[string]interface{}{"host": host, "id": id, "datastore": ds, "confirm": true}, "missing target_datastore"},
		{"vmware_fcdhost_set_control_flags", map[string]interface{}{"host": host, "id": id, "datastore": ds, "confirm": true}, "missing control_flags"},
		{"vmware_fcdhost_set_control_flags", map[string]interface{}{"host": host, "id": id, "datastore": ds, "control_flags": []interface{}{}, "confirm": true}, "empty control_flags"},
		{"vmware_fcdhost_clear_control_flags", map[string]interface{}{"host": host, "id": id, "datastore": ds, "confirm": true}, "missing control_flags"},
		{"vmware_fcdhost_reconcile_datastore_inventory", map[string]interface{}{"host": host, "confirm": true}, "missing datastore"},
		{"vmware_fcdhost_schedule_reconcile_datastore_inventory", map[string]interface{}{"host": host, "confirm": true}, "missing datastore"},
		{"vmware_fcdhost_create_snapshot", map[string]interface{}{"host": host, "id": id, "datastore": ds, "confirm": true}, "missing description"},
		{"vmware_fcdhost_delete_snapshot", map[string]interface{}{"host": host, "id": id, "datastore": ds, "confirm": true}, "missing snapshot_id"},
		{"vmware_fcdhost_get_snapshot_info", map[string]interface{}{"host": host, "datastore": ds}, "missing id"},
	}

	for _, tc := range cases {
		t.Run(tc.name+"/"+tc.why, func(t *testing.T) {
			if _, err := r.CallTool(tc.name, tc.args); err == nil {
				t.Fatalf("expected an error (%s) before any round trip", tc.why)
			}
		})
	}
}

// fcdHostBaseArgs returns a plausible, fully-populated (minus confirm)
// argument set for every one of the 18 tools, given a real host/datastore
// resolved through r — shared by TestFcdHostTools_GateAndConfirm (adds
// confirm:true itself) and TestFcdHostTools_ReachesServer.
func fcdHostBaseArgs(host, ds string) map[string]map[string]interface{} {
	const id = "52e14b1e-0000-0000-0000-000000000001"
	const snapshotID = "52e14b1e-0000-0000-0000-000000000002"

	return map[string]map[string]interface{}{
		"vmware_fcdhost_create_disk":                            {"host": host, "datastore": ds, "name": "fcdhost-disk-1", "capacity_mb": 1024},
		"vmware_fcdhost_register_disk":                          {"host": host, "path": "[" + ds + "] fcd/existing.vmdk"},
		"vmware_fcdhost_extend_disk":                            {"host": host, "id": id, "datastore": ds, "new_capacity_mb": 2048},
		"vmware_fcdhost_inflate_disk":                           {"host": host, "id": id, "datastore": ds},
		"vmware_fcdhost_rename":                                 {"host": host, "id": id, "datastore": ds, "name": "renamed-disk"},
		"vmware_fcdhost_delete":                                 {"host": host, "id": id, "datastore": ds},
		"vmware_fcdhost_clone":                                  {"host": host, "id": id, "datastore": ds, "name": "clone-1", "target_datastore": ds},
		"vmware_fcdhost_relocate":                               {"host": host, "id": id, "datastore": ds, "target_datastore": ds},
		"vmware_fcdhost_set_control_flags":                      {"host": host, "id": id, "datastore": ds, "control_flags": []interface{}{"keepAfterDeleteVm"}},
		"vmware_fcdhost_clear_control_flags":                    {"host": host, "id": id, "datastore": ds, "control_flags": []interface{}{"keepAfterDeleteVm"}},
		"vmware_fcdhost_reconcile_datastore_inventory":          {"host": host, "datastore": ds},
		"vmware_fcdhost_schedule_reconcile_datastore_inventory": {"host": host, "datastore": ds},
		"vmware_fcdhost_create_snapshot":                        {"host": host, "id": id, "datastore": ds, "description": "test snapshot"},
		"vmware_fcdhost_delete_snapshot":                        {"host": host, "id": id, "datastore": ds, "snapshot_id": snapshotID},
		// Read-only tools (no confirm needed, but included so
		// TestFcdHostTools_ReachesServer can drive all 18 from one map).
		"vmware_fcdhost_get":               {"host": host, "id": id, "datastore": ds},
		"vmware_fcdhost_get_state":         {"host": host, "id": id, "datastore": ds},
		"vmware_fcdhost_list":              {"host": host, "datastore": ds},
		"vmware_fcdhost_get_snapshot_info": {"host": host, "id": id, "datastore": ds},
	}
}

// TestFcdHostTools_GateAndConfirm proves the tier1/tier2 destructive
// protection is actually wired on every one of the 14 mutating tools: a
// closed --allow-destructive gate denies the call, and an open gate still
// requires confirm:true.
func TestFcdHostTools_GateAndConfirm(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	seed := newFcdHostRegistry(context.Background(), c, RegistryOptions{})
	host := firstHostPath(t, seed)
	ds := firstDatastorePath(t, seed)
	base := fcdHostBaseArgs(host, ds)

	closed := newFcdHostRegistry(context.Background(), c, RegistryOptions{})
	open := newFcdHostRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	for _, name := range fcdHostDestructiveToolNames {
		args, ok := base[name]
		if !ok {
			t.Fatalf("test bug: no base args for %s", name)
		}

		withConfirm := map[string]interface{}{}
		for k, v := range args {
			withConfirm[k] = v
		}
		withConfirm["confirm"] = true

		t.Run(name, func(t *testing.T) {
			if _, err := closed.CallTool(name, withConfirm); err == nil {
				t.Fatalf("%s: expected the closed destructive gate to deny the call", name)
			}
			if _, err := open.CallTool(name, args); err == nil {
				t.Fatalf("%s: expected an error without confirm:true", name)
			}
		})
	}
}

// TestFcdHostTools_ReachesServer drives every one of the 18 tools with
// valid-looking input, gate open, and confirm:true where applicable. As
// this file's top doc comment documents at length, HostVStorageObjectManager
// is a whole-object gap in vcsim on simulator.ESX() (its ServiceContent.
// VStorageObjectManager MoRef is genuinely of type "HostVStorageObjectManager"
// there, but that type has no entry in simulator/model.go's `kinds` map and
// no receiver for any Host* method anywhere in the simulator package), so
// every raw SOAP call here reaches vcsim's dispatcher and comes back with
// vcsim's "does not implement: HostXxx" fault (empirically observed running
// this very test, not just inferred — see generated_fcd_host.go's top doc
// comment) — assertReachesServer (generated_vm_lifecycle_test.go), the same
// helper generated_host_iscsi_portbinding_test.go and generated_vm_ft_test.go
// use for their own unsimulated/whole-object-gap methods. The point is
// proving the wiring (schema, tier gate, fcdhostManager MoRef resolution,
// raw SOAP dispatch) reaches vcsim, not simulating real FCD behavior — real
// functional validation is expected against a real standalone ESXi host.
func TestFcdHostTools_ReachesServer(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newFcdHostRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	host := firstHostPath(t, r)
	ds := firstDatastorePath(t, r)
	base := fcdHostBaseArgs(host, ds)

	for _, name := range fcdHostToolNames {
		args, ok := base[name]
		if !ok {
			t.Fatalf("test bug: no base args for %s", name)
		}
		callArgs := map[string]interface{}{}
		for k, v := range args {
			callArgs[k] = v
		}
		for _, destructive := range fcdHostDestructiveToolNames {
			if destructive == name {
				callArgs["confirm"] = true
				break
			}
		}

		t.Run(name, func(t *testing.T) {
			_, err := r.CallTool(name, callArgs)
			assertReachesServer(t, err, name)
		})
	}
}

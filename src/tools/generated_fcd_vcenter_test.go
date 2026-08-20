package tools

import (
	"context"
	"testing"

	"github.com/vmware/govmomi/simulator"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// fcdToolNames is the exact set registered by registerFcdVcenterTools — kept
// here so TestFcdVcenterTools_Registration can't silently drift from
// generated_fcd_vcenter.go's real registration list.
var fcdToolNames = []string{
	"vmware_fcd_create_disk",
	"vmware_fcd_register_disk",
	"vmware_fcd_extend_disk",
	"vmware_fcd_inflate_disk",
	"vmware_fcd_rename",
	"vmware_fcd_delete",
	"vmware_fcd_get",
	"vmware_fcd_get_state",
	"vmware_fcd_list",
	"vmware_fcd_clone",
	"vmware_fcd_relocate",
	"vmware_fcd_set_control_flags",
	"vmware_fcd_clear_control_flags",
	"vmware_fcd_attach_tag",
	"vmware_fcd_detach_tag",
	"vmware_fcd_list_attached_to_tag",
	"vmware_fcd_reconcile_datastore_inventory",
	"vmware_fcd_schedule_reconcile_datastore_inventory",
	"vmware_fcd_create_snapshot",
	"vmware_fcd_delete_snapshot",
	"vmware_fcd_get_snapshot_info",
	"vmware_fcd_query_changed_disk_areas",
}

// newFcdRegistry builds a Registry the normal way (NewRegistry, which
// already wires in every OTHER tool file via registry.go's registerTools —
// critically registerTagsTools and registerVirtualDiskTools, both reused by
// TestFcdVcenterTools_RealSuccess below) and additionally registers this
// file's tools via withClass — registry.go itself is intentionally left
// untouched by this change (task constraint: do not edit registry.go or
// mode_test.go). Mirrors generated_health_ippool_test.go's
// newHealthIpPoolRegistry / generated_vm_ft_test.go's newFtRegistry for the
// same situation.
func newFcdRegistry(ctx context.Context, c *vmware.Client, opts RegistryOptions) *Registry {
	r := NewRegistry(ctx, c, opts)
	r.withClass(modeVCenterOnly, registerFcdVcenterTools)
	return r
}

// fcdNestedString/fcdNestedMap/fcdNestedFloat navigate the nested
// map[string]interface{} shape decodeResult produces for this file's
// responses — e.g. vstorage_object.config.id.id — without a type assertion
// chain at every call site.
func fcdNestedMap(t *testing.T, m map[string]interface{}, key string) map[string]interface{} {
	t.Helper()
	v, ok := m[key].(map[string]interface{})
	if !ok {
		t.Fatalf("expected %q to be an object in %v", key, m)
	}
	return v
}

func fcdNestedString(t *testing.T, m map[string]interface{}, key string) string {
	t.Helper()
	v, ok := m[key].(string)
	if !ok {
		t.Fatalf("expected %q to be a string in %v", key, m)
	}
	return v
}

// TestFcdVcenterTools_Registration proves all 22 vmware_fcd_* tools are
// reachable via ListTools once registerFcdVcenterTools runs.
func TestFcdVcenterTools_Registration(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newFcdRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	if len(fcdToolNames) != 22 {
		t.Fatalf("test bug: fcdToolNames has %d entries, expected 22", len(fcdToolNames))
	}

	got := map[string]bool{}
	for _, tl := range r.ListTools() {
		got[tl.Name] = true
	}
	for _, name := range fcdToolNames {
		if !got[name] {
			t.Errorf("tool %s not registered", name)
		}
	}
}

// TestFcdVcenterTools_Validation proves each handler rejects missing/empty
// required arguments BEFORE any network round trip (so these fail even with
// the gate open and confirm:true), same convention as
// generated_health_ippool_test.go's TestHealthIpPoolTools_Validation.
func TestFcdVcenterTools_Validation(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newFcdRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	ds := firstDatastorePath(t, r)

	cases := []struct {
		name string
		args map[string]interface{}
		why  string
	}{
		{"vmware_fcd_create_disk", map[string]interface{}{"name": "n", "capacity_mb": float64(1), "confirm": true}, "missing datastore"},
		{"vmware_fcd_create_disk", map[string]interface{}{"datastore": ds, "capacity_mb": float64(1), "confirm": true}, "missing name"},
		{"vmware_fcd_create_disk", map[string]interface{}{"datastore": ds, "name": "n", "confirm": true}, "missing capacity_mb"},
		{"vmware_fcd_register_disk", map[string]interface{}{"confirm": true}, "missing path"},
		{"vmware_fcd_extend_disk", map[string]interface{}{"datastore": ds, "new_capacity_mb": float64(1), "confirm": true}, "missing id"},
		{"vmware_fcd_extend_disk", map[string]interface{}{"id": "x", "datastore": ds, "confirm": true}, "missing new_capacity_mb"},
		{"vmware_fcd_inflate_disk", map[string]interface{}{"datastore": ds, "confirm": true}, "missing id"},
		{"vmware_fcd_rename", map[string]interface{}{"id": "x", "datastore": ds, "confirm": true}, "missing name"},
		{"vmware_fcd_delete", map[string]interface{}{"datastore": ds, "confirm": true}, "missing id"},
		{"vmware_fcd_get", map[string]interface{}{"datastore": ds}, "missing id"},
		{"vmware_fcd_get", map[string]interface{}{"id": "x"}, "missing datastore"},
		{"vmware_fcd_get_state", map[string]interface{}{"datastore": ds}, "missing id"},
		{"vmware_fcd_list", map[string]interface{}{}, "missing datastore"},
		{"vmware_fcd_clone", map[string]interface{}{"id": "x", "datastore": ds, "dest_datastore": ds, "confirm": true}, "missing name"},
		{"vmware_fcd_clone", map[string]interface{}{"id": "x", "datastore": ds, "name": "n", "confirm": true}, "missing dest_datastore"},
		{"vmware_fcd_relocate", map[string]interface{}{"id": "x", "datastore": ds, "confirm": true}, "missing dest_datastore"},
		{"vmware_fcd_set_control_flags", map[string]interface{}{"id": "x", "datastore": ds, "confirm": true}, "missing control_flags"},
		{"vmware_fcd_set_control_flags", map[string]interface{}{"id": "x", "datastore": ds, "control_flags": []interface{}{}, "confirm": true}, "empty control_flags"},
		{"vmware_fcd_clear_control_flags", map[string]interface{}{"id": "x", "datastore": ds, "confirm": true}, "missing control_flags"},
		{"vmware_fcd_attach_tag", map[string]interface{}{"id": "x", "tag": "t", "confirm": true}, "missing category"},
		{"vmware_fcd_attach_tag", map[string]interface{}{"id": "x", "category": "c", "confirm": true}, "missing tag"},
		{"vmware_fcd_detach_tag", map[string]interface{}{"id": "x", "tag": "t", "confirm": true}, "missing category"},
		{"vmware_fcd_list_attached_to_tag", map[string]interface{}{"tag": "t"}, "missing category"},
		{"vmware_fcd_list_attached_to_tag", map[string]interface{}{"category": "c"}, "missing tag"},
		{"vmware_fcd_reconcile_datastore_inventory", map[string]interface{}{"confirm": true}, "missing datastore"},
		{"vmware_fcd_schedule_reconcile_datastore_inventory", map[string]interface{}{"confirm": true}, "missing datastore"},
		{"vmware_fcd_create_snapshot", map[string]interface{}{"id": "x", "datastore": ds, "confirm": true}, "missing description"},
		{"vmware_fcd_delete_snapshot", map[string]interface{}{"id": "x", "datastore": ds, "confirm": true}, "missing snapshot_id"},
		{"vmware_fcd_get_snapshot_info", map[string]interface{}{"datastore": ds}, "missing id"},
		{"vmware_fcd_query_changed_disk_areas", map[string]interface{}{"id": "x", "datastore": ds, "start_offset": float64(0), "change_id": "c"}, "missing snapshot_id"},
		{"vmware_fcd_query_changed_disk_areas", map[string]interface{}{"id": "x", "datastore": ds, "snapshot_id": "s", "change_id": "c"}, "missing start_offset"},
		{"vmware_fcd_query_changed_disk_areas", map[string]interface{}{"id": "x", "datastore": ds, "snapshot_id": "s", "start_offset": float64(0)}, "missing change_id"},
	}

	for _, tc := range cases {
		t.Run(tc.name+"/"+tc.why, func(t *testing.T) {
			if _, err := r.CallTool(tc.name, tc.args); err == nil {
				t.Fatalf("expected an error (%s) before any round trip", tc.why)
			}
		})
	}
}

// TestFcdVcenterTools_GateAndConfirm proves the tier1/tier2 destructive
// protection is wired on every one of the 16 mutating vmware_fcd_* tools: a
// closed --allow-destructive gate denies the call, and an open gate still
// requires confirm:true. Gate/confirm are checked BEFORE the handler runs
// (destructive.go's wrapDestructive), so the args below don't need to
// resolve to anything real — same posture as generated_health_ippool_test.go's
// TestHealthIpPoolTools_GateAndConfirm.
func TestFcdVcenterTools_GateAndConfirm(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	closed := newFcdRegistry(context.Background(), c, RegistryOptions{})
	open := newFcdRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	ds := firstDatastorePath(t, open)

	cases := []struct {
		name string
		args map[string]interface{}
	}{
		{"vmware_fcd_create_disk", map[string]interface{}{"datastore": ds, "name": "n", "capacity_mb": float64(16)}},
		{"vmware_fcd_register_disk", map[string]interface{}{"path": "https://x/folder/x.vmdk?dcPath=dc&dsName=ds"}},
		{"vmware_fcd_extend_disk", map[string]interface{}{"id": "x", "datastore": ds, "new_capacity_mb": float64(32)}},
		{"vmware_fcd_inflate_disk", map[string]interface{}{"id": "x", "datastore": ds}},
		{"vmware_fcd_rename", map[string]interface{}{"id": "x", "datastore": ds, "name": "y"}},
		{"vmware_fcd_delete", map[string]interface{}{"id": "x", "datastore": ds}},
		{"vmware_fcd_clone", map[string]interface{}{"id": "x", "datastore": ds, "name": "n", "dest_datastore": ds}},
		{"vmware_fcd_relocate", map[string]interface{}{"id": "x", "datastore": ds, "dest_datastore": ds}},
		{"vmware_fcd_set_control_flags", map[string]interface{}{"id": "x", "datastore": ds, "control_flags": []interface{}{"disableRelocation"}}},
		{"vmware_fcd_clear_control_flags", map[string]interface{}{"id": "x", "datastore": ds, "control_flags": []interface{}{"disableRelocation"}}},
		{"vmware_fcd_attach_tag", map[string]interface{}{"id": "x", "category": "c", "tag": "t"}},
		{"vmware_fcd_detach_tag", map[string]interface{}{"id": "x", "category": "c", "tag": "t"}},
		{"vmware_fcd_reconcile_datastore_inventory", map[string]interface{}{"datastore": ds}},
		{"vmware_fcd_schedule_reconcile_datastore_inventory", map[string]interface{}{"datastore": ds}},
		{"vmware_fcd_create_snapshot", map[string]interface{}{"id": "x", "datastore": ds, "description": "d"}},
		{"vmware_fcd_delete_snapshot", map[string]interface{}{"id": "x", "datastore": ds, "snapshot_id": "s"}},
	}

	if len(cases) != 16 {
		t.Fatalf("test bug: %d gate cases, expected 16 mutating tools", len(cases))
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withConfirm := map[string]interface{}{}
			for k, v := range tc.args {
				withConfirm[k] = v
			}
			withConfirm["confirm"] = true

			if _, err := closed.CallTool(tc.name, withConfirm); err == nil {
				t.Fatalf("%s: expected the closed destructive gate to deny the call", tc.name)
			}
			if _, err := open.CallTool(tc.name, tc.args); err == nil {
				t.Fatalf("%s: expected an error without confirm:true", tc.name)
			}
		})
	}
}

// TestFcdVcenterTools_RealSuccess drives the 13 vmware_fcd_* methods that
// DO have a real server-side handler on vcsim (referencia/govmomi/simulator/
// vstorage_object_manager.go, module cache copy — see generated_fcd_vcenter.go's
// top doc comment) through real create -> list -> get -> extend -> snapshot
// create/retrieve/delete -> tag attach/list/detach -> reconcile -> delete
// lifecycles, plus a separate register-disk sub-test, asserting actual
// returned/observed state — not just "no error" — same posture as
// generated_health_ippool_test.go's TestIpPoolTools_RealSuccess.
func TestFcdVcenterTools_RealSuccess(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newFcdRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	ds := firstDatastorePath(t, r)

	t.Run("create_list_get_extend_snapshot_tag_reconcile_delete", func(t *testing.T) {
		// --- Create -------------------------------------------------------
		rawCreate, err := r.CallTool("vmware_fcd_create_disk", map[string]interface{}{
			"datastore": ds, "name": "MCPVMWare RealSuccess FCD", "capacity_mb": float64(256), "confirm": true,
		})
		if err != nil {
			t.Fatalf("vmware_fcd_create_disk failed: %v", err)
		}
		created := decodeResult(t, rawCreate)
		if created["result"] != "disk_created" {
			t.Fatalf("vmware_fcd_create_disk: unexpected result: %v", created)
		}
		vso := fcdNestedMap(t, created, "vstorage_object")
		cfg := fcdNestedMap(t, vso, "config")
		idObj := fcdNestedMap(t, cfg, "id")
		id := fcdNestedString(t, idObj, "id")
		if id == "" {
			t.Fatalf("vmware_fcd_create_disk did not return a non-empty id: %v", created)
		}
		if name := cfg["name"]; name != "MCPVMWare RealSuccess FCD" {
			t.Errorf("vmware_fcd_create_disk: config.name = %v, want %q", name, "MCPVMWare RealSuccess FCD")
		}
		if capMB := cfg["capacityInMB"]; capMB != float64(256) {
			t.Errorf("vmware_fcd_create_disk: config.capacityInMB = %v, want 256", capMB)
		}

		// --- List — the new FCD must be present ----------------------------
		rawList, err := r.CallTool("vmware_fcd_list", map[string]interface{}{"datastore": ds})
		if err != nil {
			t.Fatalf("vmware_fcd_list failed: %v", err)
		}
		listed := decodeResult(t, rawList)
		ids, _ := listed["ids"].([]interface{})
		found := false
		for _, v := range ids {
			m, _ := v.(map[string]interface{})
			if m != nil && m["id"] == id {
				found = true
			}
		}
		if !found {
			t.Fatalf("vmware_fcd_list did not return the newly created FCD %s among %v", id, ids)
		}

		// --- Get ------------------------------------------------------------
		rawGet, err := r.CallTool("vmware_fcd_get", map[string]interface{}{"id": id, "datastore": ds})
		if err != nil {
			t.Fatalf("vmware_fcd_get failed: %v", err)
		}
		gotCfg := fcdNestedMap(t, fcdNestedMap(t, decodeResult(t, rawGet), "vstorage_object"), "config")
		if gotCfg["name"] != "MCPVMWare RealSuccess FCD" {
			t.Errorf("vmware_fcd_get: config.name = %v, want %q", gotCfg["name"], "MCPVMWare RealSuccess FCD")
		}

		// --- Extend -----------------------------------------------------
		rawExtend, err := r.CallTool("vmware_fcd_extend_disk", map[string]interface{}{"id": id, "datastore": ds, "new_capacity_mb": float64(512), "confirm": true})
		if err != nil {
			t.Fatalf("vmware_fcd_extend_disk failed: %v", err)
		}
		if decodeResult(t, rawExtend)["result"] != "disk_extended" {
			t.Fatalf("vmware_fcd_extend_disk: unexpected result: %s", rawExtend)
		}
		rawGet2, err := r.CallTool("vmware_fcd_get", map[string]interface{}{"id": id, "datastore": ds})
		if err != nil {
			t.Fatalf("vmware_fcd_get (post-extend) failed: %v", err)
		}
		gotCfg2 := fcdNestedMap(t, fcdNestedMap(t, decodeResult(t, rawGet2), "vstorage_object"), "config")
		if gotCfg2["capacityInMB"] != float64(512) {
			t.Errorf("vmware_fcd_extend_disk did not take effect: config.capacityInMB = %v, want 512", gotCfg2["capacityInMB"])
		}

		// --- Snapshot: create -> get_snapshot_info -> delete ----------------
		rawSnap, err := r.CallTool("vmware_fcd_create_snapshot", map[string]interface{}{"id": id, "datastore": ds, "description": "RealSuccess snapshot", "confirm": true})
		if err != nil {
			t.Fatalf("vmware_fcd_create_snapshot failed: %v", err)
		}
		snapCreated := decodeResult(t, rawSnap)
		if snapCreated["result"] != "snapshot_created" {
			t.Fatalf("vmware_fcd_create_snapshot: unexpected result: %v", snapCreated)
		}
		snapIDObj := fcdNestedMap(t, snapCreated, "snapshot_id")
		snapID := fcdNestedString(t, snapIDObj, "id")
		if snapID == "" {
			t.Fatalf("vmware_fcd_create_snapshot did not return a non-empty snapshot id: %v", snapCreated)
		}

		rawSnapInfo, err := r.CallTool("vmware_fcd_get_snapshot_info", map[string]interface{}{"id": id, "datastore": ds})
		if err != nil {
			t.Fatalf("vmware_fcd_get_snapshot_info failed: %v", err)
		}
		snapInfo := decodeResult(t, rawSnapInfo)
		snaps, _ := snapInfo["snapshots"].([]interface{})
		snapFound := false
		for _, v := range snaps {
			m, _ := v.(map[string]interface{})
			if m == nil {
				continue
			}
			sid, _ := m["id"].(map[string]interface{})
			if sid != nil && sid["id"] == snapID {
				snapFound = true
			}
		}
		if !snapFound {
			t.Fatalf("vmware_fcd_get_snapshot_info did not list the newly created snapshot %s among %v", snapID, snaps)
		}

		if _, err := r.CallTool("vmware_fcd_delete_snapshot", map[string]interface{}{"id": id, "datastore": ds, "snapshot_id": snapID, "confirm": true}); err != nil {
			t.Fatalf("vmware_fcd_delete_snapshot failed: %v", err)
		}
		rawSnapInfo2, err := r.CallTool("vmware_fcd_get_snapshot_info", map[string]interface{}{"id": id, "datastore": ds})
		if err != nil {
			t.Fatalf("vmware_fcd_get_snapshot_info (post-delete) failed: %v", err)
		}
		snaps2, _ := decodeResult(t, rawSnapInfo2)["snapshots"].([]interface{})
		for _, v := range snaps2 {
			m, _ := v.(map[string]interface{})
			if m == nil {
				continue
			}
			sid, _ := m["id"].(map[string]interface{})
			if sid != nil && sid["id"] == snapID {
				t.Fatalf("vmware_fcd_delete_snapshot did not remove snapshot %s — still present: %v", snapID, snaps2)
			}
		}

		// --- Tags: attach -> list_attached_to_tag -> detach -----------------
		catRaw, err := r.CallTool("vmware_tags_create_category", map[string]interface{}{
			"name": "MCPVMWare FCD RealSuccess Category", "cardinality": "MULTIPLE", "confirm": true,
		})
		if err != nil {
			t.Fatalf("vmware_tags_create_category failed: %v", err)
		}
		catID, _ := decodeResult(t, catRaw)["category_id"].(string)
		if catID == "" {
			t.Fatalf("vmware_tags_create_category did not return category_id: %s", catRaw)
		}
		const category = "MCPVMWare FCD RealSuccess Category"
		const tagName = "MCPVMWare FCD RealSuccess Tag"
		if _, err := r.CallTool("vmware_tags_create_tag", map[string]interface{}{
			"name": tagName, "description": "FCD RealSuccess", "category_id": catID, "confirm": true,
		}); err != nil {
			t.Fatalf("vmware_tags_create_tag failed: %v", err)
		}

		if _, err := r.CallTool("vmware_fcd_attach_tag", map[string]interface{}{"id": id, "category": category, "tag": tagName, "confirm": true}); err != nil {
			t.Fatalf("vmware_fcd_attach_tag failed: %v", err)
		}

		rawAttached, err := r.CallTool("vmware_fcd_list_attached_to_tag", map[string]interface{}{"category": category, "tag": tagName})
		if err != nil {
			t.Fatalf("vmware_fcd_list_attached_to_tag failed: %v", err)
		}
		attachedIDs, _ := decodeResult(t, rawAttached)["ids"].([]interface{})
		attachedFound := false
		for _, v := range attachedIDs {
			m, _ := v.(map[string]interface{})
			if m != nil && m["id"] == id {
				attachedFound = true
			}
		}
		if !attachedFound {
			t.Fatalf("vmware_fcd_list_attached_to_tag did not return FCD %s among %v", id, attachedIDs)
		}

		if _, err := r.CallTool("vmware_fcd_detach_tag", map[string]interface{}{"id": id, "category": category, "tag": tagName, "confirm": true}); err != nil {
			t.Fatalf("vmware_fcd_detach_tag failed: %v", err)
		}
		rawAttached2, err := r.CallTool("vmware_fcd_list_attached_to_tag", map[string]interface{}{"category": category, "tag": tagName})
		if err != nil {
			t.Fatalf("vmware_fcd_list_attached_to_tag (post-detach) failed: %v", err)
		}
		attachedIDs2, _ := decodeResult(t, rawAttached2)["ids"].([]interface{})
		for _, v := range attachedIDs2 {
			m, _ := v.(map[string]interface{})
			if m != nil && m["id"] == id {
				t.Fatalf("vmware_fcd_detach_tag did not remove the attachment — FCD %s still listed: %v", id, attachedIDs2)
			}
		}

		// --- Reconcile datastore inventory (no-op here — file is intact,
		// just proves the call reaches vcsim's real handler and succeeds) ---
		if _, err := r.CallTool("vmware_fcd_reconcile_datastore_inventory", map[string]interface{}{"datastore": ds, "confirm": true}); err != nil {
			t.Fatalf("vmware_fcd_reconcile_datastore_inventory failed: %v", err)
		}

		// --- Delete -----------------------------------------------------
		rawDelete, err := r.CallTool("vmware_fcd_delete", map[string]interface{}{"id": id, "datastore": ds, "confirm": true})
		if err != nil {
			t.Fatalf("vmware_fcd_delete failed: %v", err)
		}
		if decodeResult(t, rawDelete)["result"] != "disk_deleted" {
			t.Fatalf("vmware_fcd_delete: unexpected result: %s", rawDelete)
		}
		rawListAfter, err := r.CallTool("vmware_fcd_list", map[string]interface{}{"datastore": ds})
		if err != nil {
			t.Fatalf("vmware_fcd_list (post-delete) failed: %v", err)
		}
		idsAfter, _ := decodeResult(t, rawListAfter)["ids"].([]interface{})
		for _, v := range idsAfter {
			m, _ := v.(map[string]interface{})
			if m != nil && m["id"] == id {
				t.Fatalf("vmware_fcd_delete did not remove FCD %s — still present in vmware_fcd_list: %v", id, idsAfter)
			}
		}
	})

	// register_disk needs a pre-existing (but not yet FCD-tracked) disk file
	// on the datastore. vcsim's RegisterDisk (vstorage_object_manager.go)
	// parses req.Path as a URL and locates the datastore via its dcPath/
	// dsName query params (the same shape object.Datastore.NewURL produces),
	// then os.Stat()s the resolved local path — so the file must genuinely
	// exist first. vmware_virtual_disk_create (generated_virtual_disk.go,
	// VirtualDiskManager — a different, already-registered-in-NewRegistry
	// tool family) creates a real backing file the same way
	// generated_virtual_disk_test.go's own lifecycle test does, at a path
	// this sub-test controls, which register_disk then points at.
	t.Run("register_disk", func(t *testing.T) {
		dc := firstDatacenterPath(t, r)
		const relPath = "fcd-register-test/register-me.vmdk"

		if _, err := r.CallTool("vmware_virtual_disk_create", map[string]interface{}{
			"datastore":  ds,
			"path":       relPath,
			"datacenter": dc,
			"spec": map[string]interface{}{
				"diskType":    "thin",
				"adapterType": "lsiLogic",
				"capacityKb":  float64(1024),
			},
			"confirm": true,
		}); err != nil {
			t.Fatalf("fixture vmware_virtual_disk_create failed: %v", err)
		}

		dsObj, err := resolveDatastore(context.Background(), c, ds)
		if err != nil {
			t.Fatalf("resolveDatastore(%q) failed: %v", ds, err)
		}
		registerURL := dsObj.NewURL(relPath).String()

		rawReg, err := r.CallTool("vmware_fcd_register_disk", map[string]interface{}{
			"path": registerURL, "name": "MCPVMWare RealSuccess Registered FCD", "confirm": true,
		})
		if err != nil {
			t.Fatalf("vmware_fcd_register_disk failed: %v (url: %s)", err, registerURL)
		}
		registered := decodeResult(t, rawReg)
		if registered["result"] != "disk_registered" {
			t.Fatalf("vmware_fcd_register_disk: unexpected result: %v", registered)
		}
		vso := fcdNestedMap(t, registered, "vstorage_object")
		cfg := fcdNestedMap(t, vso, "config")
		idObj := fcdNestedMap(t, cfg, "id")
		id := fcdNestedString(t, idObj, "id")
		if id == "" {
			t.Fatalf("vmware_fcd_register_disk did not return a non-empty id: %v", registered)
		}

		// Cleanup: delete the registered FCD (also exercises vmware_fcd_delete
		// a second time, against an object created via a different path than
		// the primary lifecycle sub-test above).
		if _, err := r.CallTool("vmware_fcd_delete", map[string]interface{}{"id": id, "datastore": ds, "confirm": true}); err != nil {
			t.Fatalf("vmware_fcd_delete (registered disk cleanup) failed: %v", err)
		}
	})
}

// TestFcdVcenterTools_ReachesServer drives the 9 vmware_fcd_* tools whose
// underlying method has NO server-side handler on
// simulator.VcenterVStorageObjectManager (confirmed by reading the entire
// module cache file referencia/govmomi/simulator/vstorage_object_manager.go
// — see generated_fcd_vcenter.go's top doc comment): InflateDisk_Task,
// RenameVStorageObject, RetrieveVStorageObjectState, CloneVStorageObject_Task,
// RelocateVStorageObject_Task, SetVStorageObjectControlFlags,
// ClearVStorageObjectControlFlags, ScheduleReconcileDatastoreInventory,
// VstorageObjectVCenterQueryChangedDiskAreas. Each call is expected to reach
// vcsim's dispatcher and come back with a clean server-side fault, proving
// the wiring — schema, tier gate, VStorageObjectManager MoRef, raw SOAP
// dispatch — reaches vcsim and returns a clean error, not an unknown-tool
// wiring bug or a recovered panic. Same helper/rationale as
// generated_health_ippool_test.go's TestHealthTools_ReachesServer and
// generated_vm_ft_test.go's TestVMFaultToleranceTools_ReachesServer.
//
// A real FCD is created first (CreateDisk_Task IS simulated) purely to hand
// these 9 unsimulated calls a syntactically valid "id" — same posture as
// generated_vm_ft_test.go reusing a real resolved VM path for its own
// unsimulated FT calls: the point is proving the wiring reaches vcsim, not
// simulating real clone/relocate/rename semantics (impossible here — vcsim
// has no handler for any of the 9 regardless of how valid the arguments are).
func TestFcdVcenterTools_ReachesServer(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newFcdRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	ds := firstDatastorePath(t, r)

	rawCreate, err := r.CallTool("vmware_fcd_create_disk", map[string]interface{}{
		"datastore": ds, "name": "MCPVMWare ReachesServer FCD", "capacity_mb": float64(64), "confirm": true,
	})
	if err != nil {
		t.Fatalf("fixture vmware_fcd_create_disk failed: %v", err)
	}
	created := decodeResult(t, rawCreate)
	cfg := fcdNestedMap(t, fcdNestedMap(t, created, "vstorage_object"), "config")
	id := fcdNestedString(t, fcdNestedMap(t, cfg, "id"), "id")
	if id == "" {
		t.Fatalf("fixture vmware_fcd_create_disk did not return a usable id: %v", created)
	}

	cases := []struct {
		name string
		args map[string]interface{}
	}{
		{"vmware_fcd_inflate_disk", map[string]interface{}{"id": id, "datastore": ds, "confirm": true}},
		{"vmware_fcd_rename", map[string]interface{}{"id": id, "datastore": ds, "name": "renamed", "confirm": true}},
		{"vmware_fcd_get_state", map[string]interface{}{"id": id, "datastore": ds}},
		{"vmware_fcd_clone", map[string]interface{}{"id": id, "datastore": ds, "name": "clone", "dest_datastore": ds, "confirm": true}},
		{"vmware_fcd_relocate", map[string]interface{}{"id": id, "datastore": ds, "dest_datastore": ds, "confirm": true}},
		{"vmware_fcd_set_control_flags", map[string]interface{}{"id": id, "datastore": ds, "control_flags": []interface{}{"disableRelocation"}, "confirm": true}},
		{"vmware_fcd_clear_control_flags", map[string]interface{}{"id": id, "datastore": ds, "control_flags": []interface{}{"disableRelocation"}, "confirm": true}},
		{"vmware_fcd_schedule_reconcile_datastore_inventory", map[string]interface{}{"datastore": ds, "confirm": true}},
		{"vmware_fcd_query_changed_disk_areas", map[string]interface{}{"id": id, "datastore": ds, "snapshot_id": "s", "start_offset": float64(0), "change_id": "c"}},
	}

	if len(cases) != 9 {
		t.Fatalf("test bug: %d ReachesServer cases, expected 9 unsimulated methods", len(cases))
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := r.CallTool(tc.name, tc.args)
			assertReachesServer(t, err, tc.name)
		})
	}
}

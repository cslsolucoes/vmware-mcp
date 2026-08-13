package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/vmware/govmomi/simulator"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// newVirtualDiskRegistry builds a Registry the normal way (NewRegistry,
// which wires vm.go/host.go/datastore.go/etc via registerTools) and then
// manually layers this group's tools on top via withClass — this file must
// not edit registry.go itself (see generated_virtual_disk.go's top doc
// comment / this project's Fase 4 convention, generated_vm_lifecycle_test.go's
// newLifecycleRegistry).
func newVirtualDiskRegistry(ctx context.Context, c *vmware.Client, opts RegistryOptions) *Registry {
	r := NewRegistry(ctx, c, opts)
	r.withClass(modeVSphereGeneral, registerVirtualDiskTools)
	return r
}

// TestVirtualDiskTools_EndToEndLifecycle proves the highest-value path in
// this whole group: create -> query_info -> extend -> query_uuid ->
// set_uuid -> move -> copy -> delete, all against a real vcsim ESX() model,
// exercising the 8 methods that DO have a real simulator handler (see
// generated_virtual_disk.go's top doc comment, deviation 6).
func TestVirtualDiskTools_EndToEndLifecycle(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newVirtualDiskRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	base := map[string]interface{}{
		"datastore":  "LocalDS_0",
		"path":       "disks/disk1.vmdk",
		"datacenter": "ha-datacenter",
	}

	// --- create ------------------------------------------------------
	createArgs := map[string]interface{}{
		"datastore":  base["datastore"],
		"path":       base["path"],
		"datacenter": base["datacenter"],
		"spec": map[string]interface{}{
			"diskType":    "thin",
			"adapterType": "lsiLogic",
			"capacityKb":  1024 * 1024,
		},
		"confirm": true,
	}
	raw, err := r.CallTool("vmware_virtual_disk_create", createArgs)
	if err != nil {
		t.Fatalf("vmware_virtual_disk_create failed: %v", err)
	}
	res := decodeResult(t, raw)
	if res["result"] != "created" {
		t.Fatalf("vmware_virtual_disk_create: unexpected result: %s", raw)
	}
	diskName, _ := res["disk"].(string)
	if diskName == "" || !strings.Contains(diskName, "disk1.vmdk") {
		t.Fatalf("vmware_virtual_disk_create: unexpected disk name %q (raw: %s)", diskName, raw)
	}
	t.Logf("created disk: %s", diskName)

	// A 2nd create at the same path must fail (already exists) — proves the
	// call really reached vcsim's file-exists check, not a no-op stub.
	if _, err := r.CallTool("vmware_virtual_disk_create", createArgs); err == nil {
		t.Fatal("expected a 2nd vmware_virtual_disk_create at the same path to fail (disk already exists)")
	}

	// --- query_info ----------------------------------------------------
	raw, err = r.CallTool("vmware_virtual_disk_query_info", map[string]interface{}{
		"datastore": base["datastore"], "path": base["path"], "datacenter": base["datacenter"],
	})
	if err != nil {
		t.Fatalf("vmware_virtual_disk_query_info failed: %v", err)
	}
	res = decodeResult(t, raw)
	if c, ok := res["count"].(float64); !ok || c < 1 {
		t.Fatalf("vmware_virtual_disk_query_info: expected count >= 1, got %s", raw)
	}

	// --- extend ----------------------------------------------------------
	raw, err = r.CallTool("vmware_virtual_disk_extend", map[string]interface{}{
		"datastore": base["datastore"], "path": base["path"], "datacenter": base["datacenter"],
		"capacity_kb": 2 * 1024 * 1024,
		"confirm":     true,
	})
	if err != nil {
		t.Fatalf("vmware_virtual_disk_extend failed: %v", err)
	}
	res = decodeResult(t, raw)
	if res["result"] != "extended" {
		t.Fatalf("vmware_virtual_disk_extend: unexpected result: %s", raw)
	}

	// Shrinking via extend (smaller capacity) must fail — vcsim's
	// vdmExtendVirtualDisk explicitly rejects "cannot shrink disk".
	if _, err := r.CallTool("vmware_virtual_disk_extend", map[string]interface{}{
		"datastore": base["datastore"], "path": base["path"], "datacenter": base["datacenter"],
		"capacity_kb": 1024,
		"confirm":     true,
	}); err == nil {
		t.Fatal("expected vmware_virtual_disk_extend to fail when shrinking capacity")
	}

	// --- query_uuid --------------------------------------------------
	raw, err = r.CallTool("vmware_virtual_disk_query_uuid", map[string]interface{}{
		"datastore": base["datastore"], "path": base["path"], "datacenter": base["datacenter"],
	})
	if err != nil {
		t.Fatalf("vmware_virtual_disk_query_uuid failed: %v", err)
	}
	res = decodeResult(t, raw)
	originalUUID, _ := res["uuid"].(string)
	if originalUUID == "" {
		t.Fatalf("vmware_virtual_disk_query_uuid: empty uuid (raw: %s)", raw)
	}

	// --- set_uuid (Deviation 7: vcsim stub does not persist) -------------
	raw, err = r.CallTool("vmware_virtual_disk_set_uuid", map[string]interface{}{
		"datastore": base["datastore"], "path": base["path"], "datacenter": base["datacenter"],
		"uuid":    "11111111-2222-3333-4444-555555555555",
		"confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_virtual_disk_set_uuid failed: %v", err)
	}
	res = decodeResult(t, raw)
	if res["result"] != "uuid_set" {
		t.Fatalf("vmware_virtual_disk_set_uuid: unexpected result: %s", raw)
	}

	raw, err = r.CallTool("vmware_virtual_disk_query_uuid", map[string]interface{}{
		"datastore": base["datastore"], "path": base["path"], "datacenter": base["datacenter"],
	})
	if err != nil {
		t.Fatalf("vmware_virtual_disk_query_uuid (post set_uuid) failed: %v", err)
	}
	res = decodeResult(t, raw)
	afterSetUUID, _ := res["uuid"].(string)
	if afterSetUUID != originalUUID {
		t.Fatalf("unexpected: vcsim's query_uuid changed after set_uuid (was %q, now %q) — if this project's simulator dependency started persisting SetVirtualDiskUuid, update generated_virtual_disk.go's top doc comment (deviation 7), this assertion, and consider whether the tool description's caution note is now stale", originalUUID, afterSetUUID)
	}
	if afterSetUUID == "11111111-2222-3333-4444-555555555555" {
		t.Fatal("unexpected: vcsim's query_uuid returned exactly the uuid we set — same reason as above, deviation 7 would need updating")
	}
	t.Logf("confirmed deviation 7: set_uuid succeeded but query_uuid still returns the hash-derived uuid %q, not what was set", afterSetUUID)

	// --- move --------------------------------------------------------
	raw, err = r.CallTool("vmware_virtual_disk_move", map[string]interface{}{
		"source_datastore": base["datastore"], "source_path": base["path"], "source_datacenter": base["datacenter"],
		"dest_datastore": base["datastore"], "dest_path": "disks/disk2.vmdk", "dest_datacenter": base["datacenter"],
		"confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_virtual_disk_move failed: %v", err)
	}
	res = decodeResult(t, raw)
	if res["result"] != "moved" {
		t.Fatalf("vmware_virtual_disk_move: unexpected result: %s", raw)
	}

	// The old path must be gone now — querying it should fail.
	if _, err := r.CallTool("vmware_virtual_disk_query_uuid", map[string]interface{}{
		"datastore": base["datastore"], "path": base["path"], "datacenter": base["datacenter"],
	}); err == nil {
		t.Fatal("expected vmware_virtual_disk_query_uuid on the old (pre-move) path to fail — the disk was moved away")
	}

	// --- copy ----------------------------------------------------------
	raw, err = r.CallTool("vmware_virtual_disk_copy", map[string]interface{}{
		"source_datastore": base["datastore"], "source_path": "disks/disk2.vmdk", "source_datacenter": base["datacenter"],
		"dest_datastore": base["datastore"], "dest_path": base["path"], "dest_datacenter": base["datacenter"],
		"dest_spec": map[string]interface{}{"diskType": "thin", "adapterType": "lsiLogic"},
		"confirm":   true,
	})
	if err != nil {
		t.Fatalf("vmware_virtual_disk_copy failed: %v", err)
	}
	res = decodeResult(t, raw)
	if res["result"] != "copied" {
		t.Fatalf("vmware_virtual_disk_copy: unexpected result: %s", raw)
	}

	// Both disk1.vmdk (copied back) and disk2.vmdk (moved original) must
	// now exist — query_uuid on both should succeed.
	for _, p := range []string{base["path"].(string), "disks/disk2.vmdk"} {
		if _, err := r.CallTool("vmware_virtual_disk_query_uuid", map[string]interface{}{
			"datastore": base["datastore"], "path": p, "datacenter": base["datacenter"],
		}); err != nil {
			t.Fatalf("vmware_virtual_disk_query_uuid on %s failed after copy: %v", p, err)
		}
	}

	// --- delete_virtual_disk ---------------------------------------------
	for _, p := range []string{base["path"].(string), "disks/disk2.vmdk"} {
		raw, err = r.CallTool("vmware_virtual_disk_delete_virtual_disk", map[string]interface{}{
			"datastore": base["datastore"], "path": p, "datacenter": base["datacenter"],
			"confirm": true,
		})
		if err != nil {
			t.Fatalf("vmware_virtual_disk_delete_virtual_disk(%s) failed: %v", p, err)
		}
		res = decodeResult(t, raw)
		if res["result"] != "deleted" {
			t.Fatalf("vmware_virtual_disk_delete_virtual_disk(%s): unexpected result: %s", p, raw)
		}
	}

	// A 2nd delete of the same (now-gone) disk must fail — irreversible,
	// proves this isn't a silent no-op.
	if _, err := r.CallTool("vmware_virtual_disk_delete_virtual_disk", map[string]interface{}{
		"datastore": base["datastore"], "path": base["path"], "datacenter": base["datacenter"],
		"confirm": true,
	}); err == nil {
		t.Fatal("expected a 2nd delete of the same disk to fail")
	}
}

// TestVirtualDiskTools_GateAndConfirm proves the Tier 1/2 destructive-action
// protection (server gate + confirm:true) is wired for a representative
// Tier 2 tool (create) and the sole Tier 1 tool (delete) — same proof
// pattern as generated_vm_lifecycle_test.go's
// TestVMLifecycleTools_ShutdownGuestGateAndConfirm /
// host_test.go's TestHostTools_MaintenanceEnterGateAndConfirm.
func TestVirtualDiskTools_GateAndConfirm(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	closedGate := newVirtualDiskRegistry(context.Background(), c, RegistryOptions{})
	openGate := newVirtualDiskRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	createArgs := map[string]interface{}{
		"datastore":  "LocalDS_0",
		"path":       "gate-test/disk.vmdk",
		"datacenter": "ha-datacenter",
		"spec":       map[string]interface{}{"diskType": "thin", "adapterType": "lsiLogic", "capacityKb": 1024},
	}

	// Closed gate: fails even with confirm:true.
	withConfirm := map[string]interface{}{}
	for k, v := range createArgs {
		withConfirm[k] = v
	}
	withConfirm["confirm"] = true
	if _, err := closedGate.CallTool("vmware_virtual_disk_create", withConfirm); err == nil {
		t.Fatal("expected vmware_virtual_disk_create to fail with the destructive gate closed, even with confirm:true")
	}

	// Open gate, no confirm: still fails.
	withoutConfirm := map[string]interface{}{}
	for k, v := range createArgs {
		withoutConfirm[k] = v
	}
	if _, err := openGate.CallTool("vmware_virtual_disk_create", withoutConfirm); err == nil {
		t.Fatal("expected vmware_virtual_disk_create to fail without confirm:true, even with the gate open")
	}

	// Open gate + confirm:true: succeeds.
	if _, err := openGate.CallTool("vmware_virtual_disk_create", withConfirm); err != nil {
		t.Fatalf("vmware_virtual_disk_create failed with gate open and confirm:true: %v", err)
	}

	deleteArgs := map[string]interface{}{
		"datastore": "LocalDS_0", "path": "gate-test/disk.vmdk", "datacenter": "ha-datacenter",
	}

	// Tier 1 delete: closed gate fails even with confirm:true.
	deleteWithConfirm := map[string]interface{}{}
	for k, v := range deleteArgs {
		deleteWithConfirm[k] = v
	}
	deleteWithConfirm["confirm"] = true
	if _, err := closedGate.CallTool("vmware_virtual_disk_delete_virtual_disk", deleteWithConfirm); err == nil {
		t.Fatal("expected vmware_virtual_disk_delete_virtual_disk to fail with the destructive gate closed")
	}

	deleteWithoutConfirm := map[string]interface{}{}
	for k, v := range deleteArgs {
		deleteWithoutConfirm[k] = v
	}
	if _, err := openGate.CallTool("vmware_virtual_disk_delete_virtual_disk", deleteWithoutConfirm); err == nil {
		t.Fatal("expected vmware_virtual_disk_delete_virtual_disk to fail without confirm:true")
	}

	if _, err := openGate.CallTool("vmware_virtual_disk_delete_virtual_disk", deleteWithConfirm); err != nil {
		t.Fatalf("vmware_virtual_disk_delete_virtual_disk failed with gate open and confirm:true: %v", err)
	}
}

// TestVirtualDiskTools_NoSimulatorSupport proves the 3 methods vcsim has no
// server-side implementation for (InflateVirtualDisk, ShrinkVirtualDisk,
// CreateChildDisk — see generated_virtual_disk.go's top doc comment,
// deviation 6) reach the real server and get back a clean
// types.MethodNotFound-based error via assertReachesServer, reused from
// generated_vm_lifecycle_test.go — proving the plumbing (schema, tier
// gating, resolveDiskLocation) works even though vcsim itself can't
// simulate the actual disk operation.
func TestVirtualDiskTools_NoSimulatorSupport(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newVirtualDiskRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	// Fixture: a real disk to target (Inflate/Shrink need an existing file
	// to even get past client-side/early server-side checks before hitting
	// the missing method dispatch).
	createArgs := map[string]interface{}{
		"datastore":  "LocalDS_0",
		"path":       "nosim/disk.vmdk",
		"datacenter": "ha-datacenter",
		"spec":       map[string]interface{}{"diskType": "thin", "adapterType": "lsiLogic", "capacityKb": 1024},
		"confirm":    true,
	}
	if _, err := r.CallTool("vmware_virtual_disk_create", createArgs); err != nil {
		t.Fatalf("fixture vmware_virtual_disk_create failed: %v", err)
	}

	t.Run("inflate", func(t *testing.T) {
		_, err := r.CallTool("vmware_virtual_disk_inflate", map[string]interface{}{
			"datastore": "LocalDS_0", "path": "nosim/disk.vmdk", "datacenter": "ha-datacenter",
			"confirm": true,
		})
		assertReachesServer(t, err, "vmware_virtual_disk_inflate")
	})

	t.Run("shrink", func(t *testing.T) {
		_, err := r.CallTool("vmware_virtual_disk_shrink", map[string]interface{}{
			"datastore": "LocalDS_0", "path": "nosim/disk.vmdk", "datacenter": "ha-datacenter",
			"confirm": true,
		})
		assertReachesServer(t, err, "vmware_virtual_disk_shrink")
	})

	t.Run("create_child", func(t *testing.T) {
		_, err := r.CallTool("vmware_virtual_disk_create_child", map[string]interface{}{
			"parent_datastore": "LocalDS_0", "parent_path": "nosim/disk.vmdk", "parent_datacenter": "ha-datacenter",
			"datastore": "LocalDS_0", "path": "nosim/child.vmdk", "datacenter": "ha-datacenter",
			"confirm": true,
		})
		assertReachesServer(t, err, "vmware_virtual_disk_create_child")
	})
}

// TestVirtualDiskTools_ArgumentValidation proves the required-argument
// guard clauses fail cleanly (not a panic, not a nil-pointer dereference)
// when a required disk-location argument is missing — cheap coverage for
// resolveDiskLocation's 3 early-return branches, exercised through the
// unprefixed single-disk shape used by 8 of these 11 tools.
func TestVirtualDiskTools_ArgumentValidation(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newVirtualDiskRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	cases := []struct {
		name string
		args map[string]interface{}
	}{
		{"missing datastore", map[string]interface{}{"path": "x.vmdk", "datacenter": "ha-datacenter"}},
		{"missing path", map[string]interface{}{"datastore": "LocalDS_0", "datacenter": "ha-datacenter"}},
		{"missing datacenter", map[string]interface{}{"datastore": "LocalDS_0", "path": "x.vmdk"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := r.CallTool("vmware_virtual_disk_query_uuid", tc.args)
			if err == nil {
				t.Fatalf("expected vmware_virtual_disk_query_uuid to fail for case %q", tc.name)
			}
			if strings.Contains(err.Error(), "panicked") {
				t.Fatalf("case %q: handler panicked instead of returning a clean error: %v", tc.name, err)
			}
		})
	}
}

// TestVirtualDiskTools_Registration proves all 11 tools are registered
// under modeVSphereGeneral with the expected tier gating shape: the 9
// destructive tools (Tier 1 delete + 8 Tier 2 mutators) all reject a call
// without confirm:true even with the gate open, and the 2 read-only query
// tools need no confirm at all — a request against a nonexistent disk still
// reaches the "not found"-style error, not a "confirm required" one.
func TestVirtualDiskTools_Registration(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newVirtualDiskRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	destructiveTools := []string{
		"vmware_virtual_disk_copy",
		"vmware_virtual_disk_create_child",
		"vmware_virtual_disk_create",
		"vmware_virtual_disk_extend",
		"vmware_virtual_disk_inflate",
		"vmware_virtual_disk_move",
		"vmware_virtual_disk_set_uuid",
		"vmware_virtual_disk_shrink",
		"vmware_virtual_disk_delete_virtual_disk",
	}
	for _, tool := range destructiveTools {
		t.Run(tool+"_requires_confirm", func(t *testing.T) {
			_, err := r.CallTool(tool, map[string]interface{}{})
			if err == nil {
				t.Fatalf("%s: expected a failure when called with no arguments at all (at minimum confirm:true is missing)", tool)
			}
			if !strings.Contains(err.Error(), "confirm") {
				t.Fatalf("%s: expected the confirm:true requirement to be the (or an early) error with no args given, got: %v", tool, err)
			}
		})
	}

	// A nonexistent DATASTORE (not just a nonexistent disk path) is used
	// here deliberately: vcsim's QueryVirtualDiskInfoTask handler
	// (referencia/govmomi/simulator/virtual_disk_manager.go) only
	// syntactically resolves the datastore path via FileManager.resolve —
	// it never os.Stat()s the vmdk file — so a nonexistent disk PATH on a
	// real datastore does not error (confirmed empirically; the disk simply
	// doesn't need to exist for this call to "succeed" against vcsim).
	// QueryVirtualDiskUuid's handler does os.Stat and would fail on a
	// missing path, but an unresolvable datastore name fails both cleanly
	// and uniformly through resolveDiskLocation's own resolveDatastore
	// call, before either tool's real govmomi call even runs.
	readOnlyTools := []string{"vmware_virtual_disk_query_info", "vmware_virtual_disk_query_uuid"}
	for _, tool := range readOnlyTools {
		t.Run(tool+"_no_confirm_needed", func(t *testing.T) {
			_, err := r.CallTool(tool, map[string]interface{}{
				"datastore": "does-not-exist-ds", "path": "does-not-exist.vmdk", "datacenter": "ha-datacenter",
			})
			if err == nil {
				t.Fatalf("%s: expected a failure for a nonexistent datastore", tool)
			}
			if strings.Contains(err.Error(), "confirm") {
				t.Fatalf("%s: read-only tool should never mention confirm:true, got: %v", tool, err)
			}
		})
	}
}

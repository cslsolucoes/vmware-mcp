package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/simulator"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// newDatastoreFileManagerRegistry builds a Registry the normal way
// (NewRegistry, which wires vm.go/host.go/etc via registerTools) and then
// manually layers this group's tools on top via withClass — same pattern as
// generated_host_storage_test.go's newHostStorageRegistry, and for the same
// reason: registry.go itself must not be edited by this file (see
// generated_datastore_filemanagers.go's top doc comment) — a human wires
// registerDatastoreFileManagerTools into registry.go later, alongside the 3
// sibling Fase 4 "file managers" slices.
func newDatastoreFileManagerRegistry(ctx context.Context, c *vmware.Client, opts RegistryOptions) *Registry {
	r := NewRegistry(ctx, c, opts)
	r.withClass(modeVSphereGeneral, registerDatastoreFileManagerTools)
	return r
}

// dfmFirstDatacenterName returns the first datacenter's inventory path
// (e.g. "/ha-datacenter") from the simulator model, for the 4
// FileManager-scoped tools below (which take an explicit
// "datacenter"/"source_datacenter"/"destination_datacenter" argument — see
// this group's top doc comment). vmware_list_datacenters
// (tools/inventory.go's handleListDatacenters) returns a plain []string of
// InventoryPaths, NOT a list of {"name": ...} objects like
// vmware_list_datastores/firstDatastoreName — confirmed empirically after an
// initial version of this helper assumed the wrong shape and failed with
// "first datacenter entry has no name field". client.Finder.Datacenter
// accepts either a bare name or a full inventory path, so returning the path
// as-is works directly as the "datacenter" argument value. Named with a dfm
// prefix to avoid colliding with an equivalent helper another Fase 4 sibling
// slice (datastore browser, storage DRS, virtual disk manager) may add to
// this same package independently.
func dfmFirstDatacenterName(t *testing.T, r *Registry) string {
	t.Helper()
	raw, err := r.CallTool("vmware_list_datacenters", map[string]interface{}{})
	if err != nil {
		t.Fatalf("vmware_list_datacenters failed: %v", err)
	}
	list, _ := decodeResult(t, raw)["datacenters"].([]interface{})
	if len(list) == 0 {
		t.Fatal("simulator model has no datacenters")
	}
	path, _ := list[0].(string)
	if path == "" {
		t.Fatal("first datacenter entry is not a non-empty string")
	}
	return path
}

// dfmUpload writes content to a local temp file and uploads it to remotePath
// on ds — the real, SDK-level way (same object.Datastore.UploadFile
// tools/datastore.go's vmware_datastore_upload_file uses) to seed files this
// group's tools then copy/move/delete, so every test below exercises real
// vcsim file I/O, never a mock.
func dfmUpload(t *testing.T, ctx context.Context, ds *object.Datastore, remotePath, content string) {
	t.Helper()
	dir := t.TempDir()
	local := filepath.Join(dir, filepath.Base(remotePath))
	if err := os.WriteFile(local, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write local temp file for %s: %v", remotePath, err)
	}
	if err := ds.UploadFile(ctx, local, remotePath, nil); err != nil {
		t.Fatalf("failed to upload test file %s: %v", remotePath, err)
	}
}

// dfmExists reports whether remotePath exists on ds, via the same
// Stat+datastoreFileMissing pattern datastore.go's own upload handler uses.
func dfmExists(t *testing.T, ctx context.Context, ds *object.Datastore, remotePath string) bool {
	t.Helper()
	_, err := ds.Stat(ctx, remotePath)
	if err == nil {
		return true
	}
	if datastoreFileMissing(err) {
		return false
	}
	t.Fatalf("unexpected error checking %s on datastore %s: %v", remotePath, ds.InventoryPath, err)
	return false
}

// TestDatastoreFileManagerTools_Registration proves all 11 tools are
// registered and reachable via ListTools — a basic wiring smoke test before
// the more specific behavioral tests below.
func TestDatastoreFileManagerTools_Registration(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newDatastoreFileManagerRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	want := []string{
		"vmware_datastore_file_copy",
		"vmware_datastore_file_copy_file",
		"vmware_datastore_file_move",
		"vmware_datastore_file_move_file",
		"vmware_datastore_file_delete",
		"vmware_datastore_file_delete_file",
		"vmware_datastore_file_delete_virtual_disk",
		"vmware_file_copy_datastore_file",
		"vmware_file_move_datastore_file",
		"vmware_file_make_directory",
		"vmware_file_delete_datastore_file",
	}
	if len(want) != 11 {
		t.Fatalf("test bug: want list has %d entries, expected 11", len(want))
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

// TestDatastoreFileManagerTools_PlainFileCopyMoveDelete exercises the
// non-.vmdk path of all 3 copy/move/delete tool families against a real
// uploaded file on vcsim — copy, force-overwrite behavior, move, then
// delete, asserting real existence via Datastore.Stat at every step (not
// just "no error").
func TestDatastoreFileManagerTools_PlainFileCopyMoveDelete(t *testing.T) {
	ctx := context.Background()
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newDatastoreFileManagerRegistry(ctx, c, RegistryOptions{AllowDestructive: true})
	dsName := firstDatastoreName(t, r)
	ds, err := resolveDatastore(ctx, r.client, dsName)
	if err != nil {
		t.Fatalf("resolveDatastore(%s) failed: %v", dsName, err)
	}

	dfmUpload(t, ctx, ds, "plain-src.txt", "hello from TestDatastoreFileManagerTools_PlainFileCopyMoveDelete")

	t.Run("copy_file", func(t *testing.T) {
		raw, err := r.CallTool("vmware_datastore_file_copy_file", map[string]interface{}{
			"datastore": dsName, "source": "plain-src.txt", "destination": "plain-copy.txt", "confirm": true,
		})
		if err != nil {
			t.Fatalf("vmware_datastore_file_copy_file failed: %v", err)
		}
		if !dfmExists(t, ctx, ds, "plain-copy.txt") {
			t.Fatalf("plain-copy.txt does not exist after copy_file: %s", raw)
		}
		if !dfmExists(t, ctx, ds, "plain-src.txt") {
			t.Fatal("plain-src.txt (the source) should still exist after a copy")
		}
	})

	t.Run("copy_file_refuses_overwrite_without_force", func(t *testing.T) {
		if _, err := r.CallTool("vmware_datastore_file_copy_file", map[string]interface{}{
			"datastore": dsName, "source": "plain-src.txt", "destination": "plain-copy.txt", "confirm": true,
		}); err == nil {
			t.Fatal("expected an error copying onto an existing destination without force:true")
		}
	})

	t.Run("copy_file_overwrites_with_force", func(t *testing.T) {
		if _, err := r.CallTool("vmware_datastore_file_copy_file", map[string]interface{}{
			"datastore": dsName, "source": "plain-src.txt", "destination": "plain-copy.txt", "force": true, "confirm": true,
		}); err != nil {
			t.Fatalf("vmware_datastore_file_copy_file with force:true failed: %v", err)
		}
	})

	t.Run("move_file", func(t *testing.T) {
		raw, err := r.CallTool("vmware_datastore_file_move_file", map[string]interface{}{
			"datastore": dsName, "source": "plain-copy.txt", "destination": "plain-moved.txt", "confirm": true,
		})
		if err != nil {
			t.Fatalf("vmware_datastore_file_move_file failed: %v", err)
		}
		if dfmExists(t, ctx, ds, "plain-copy.txt") {
			t.Fatal("plain-copy.txt (the source) should be gone after a move")
		}
		if !dfmExists(t, ctx, ds, "plain-moved.txt") {
			t.Fatalf("plain-moved.txt does not exist after move_file: %s", raw)
		}
	})

	t.Run("delete_file", func(t *testing.T) {
		raw, err := r.CallTool("vmware_datastore_file_delete_file", map[string]interface{}{
			"datastore": dsName, "name": "plain-moved.txt", "confirm": true,
		})
		if err != nil {
			t.Fatalf("vmware_datastore_file_delete_file failed: %v", err)
		}
		if dfmExists(t, ctx, ds, "plain-moved.txt") {
			t.Fatalf("plain-moved.txt still exists after delete_file: %s", raw)
		}
	})

	t.Run("delete_dispatches_to_delete_file_for_non_vmdk", func(t *testing.T) {
		dfmUpload(t, ctx, ds, "plain-for-smart-delete.txt", "x")
		if _, err := r.CallTool("vmware_datastore_file_delete", map[string]interface{}{
			"datastore": dsName, "name": "plain-for-smart-delete.txt", "confirm": true,
		}); err != nil {
			t.Fatalf("vmware_datastore_file_delete failed: %v", err)
		}
		if dfmExists(t, ctx, ds, "plain-for-smart-delete.txt") {
			t.Fatal("plain-for-smart-delete.txt still exists after vmware_datastore_file_delete")
		}
	})
}

// TestDatastoreFileManagerTools_VirtualDiskDispatch proves — against real
// vcsim file I/O, not by reading the source alone — the documented real
// distinctions between Copy/CopyFile, Move/MoveFile, and
// Delete/DeleteFile/DeleteVirtualDisk for .vmdk paths: the "smart" tools
// (Copy/Move/Delete) carry the "-flat.vmdk" backing extent along with the
// descriptor; the "always plain" tools (CopyFile/MoveFile/DeleteFile) do
// not, which is exactly the trap this group's tool descriptions warn about.
func TestDatastoreFileManagerTools_VirtualDiskDispatch(t *testing.T) {
	ctx := context.Background()
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newDatastoreFileManagerRegistry(ctx, c, RegistryOptions{AllowDestructive: true})
	dsName := firstDatastoreName(t, r)
	ds, err := resolveDatastore(ctx, r.client, dsName)
	if err != nil {
		t.Fatalf("resolveDatastore(%s) failed: %v", dsName, err)
	}

	t.Run("copy_file_plain_leaves_flat_extent_behind", func(t *testing.T) {
		dfmUpload(t, ctx, ds, "cf-disk.vmdk", "descriptor")
		dfmUpload(t, ctx, ds, "cf-disk-flat.vmdk", "extent")

		if _, err := r.CallTool("vmware_datastore_file_copy_file", map[string]interface{}{
			"datastore": dsName, "source": "cf-disk.vmdk", "destination": "cf-disk-plaincopy.vmdk", "confirm": true,
		}); err != nil {
			t.Fatalf("vmware_datastore_file_copy_file failed: %v", err)
		}
		if !dfmExists(t, ctx, ds, "cf-disk-plaincopy.vmdk") {
			t.Fatal("cf-disk-plaincopy.vmdk (descriptor) missing after copy_file")
		}
		if dfmExists(t, ctx, ds, "cf-disk-plaincopy-flat.vmdk") {
			t.Fatal("cf-disk-plaincopy-flat.vmdk should NOT exist — copy_file is documented as a plain byte copy that does not carry the backing extent")
		}
	})

	t.Run("copy_dispatches_to_virtual_disk_and_carries_flat_extent", func(t *testing.T) {
		dfmUpload(t, ctx, ds, "c-disk.vmdk", "descriptor")
		dfmUpload(t, ctx, ds, "c-disk-flat.vmdk", "extent")

		if _, err := r.CallTool("vmware_datastore_file_copy", map[string]interface{}{
			"datastore": dsName, "source": "c-disk.vmdk", "destination": "c-disk-copy.vmdk", "confirm": true,
		}); err != nil {
			t.Fatalf("vmware_datastore_file_copy failed: %v", err)
		}
		if !dfmExists(t, ctx, ds, "c-disk-copy.vmdk") {
			t.Fatal("c-disk-copy.vmdk (descriptor) missing after copy")
		}
		if !dfmExists(t, ctx, ds, "c-disk-copy-flat.vmdk") {
			t.Fatal("c-disk-copy-flat.vmdk (backing extent) missing after copy — vmware_datastore_file_copy must dispatch to the virtual-disk-aware copy for a .vmdk source")
		}

		t.Run("then_move_carries_flat_extent_too", func(t *testing.T) {
			if _, err := r.CallTool("vmware_datastore_file_move", map[string]interface{}{
				"datastore": dsName, "source": "c-disk-copy.vmdk", "destination": "c-disk-moved.vmdk", "confirm": true,
			}); err != nil {
				t.Fatalf("vmware_datastore_file_move failed: %v", err)
			}
			if dfmExists(t, ctx, ds, "c-disk-copy.vmdk") || dfmExists(t, ctx, ds, "c-disk-copy-flat.vmdk") {
				t.Fatal("source descriptor/extent should both be gone after move")
			}
			if !dfmExists(t, ctx, ds, "c-disk-moved.vmdk") || !dfmExists(t, ctx, ds, "c-disk-moved-flat.vmdk") {
				t.Fatal("both destination descriptor and backing extent should exist after vmware_datastore_file_move of a .vmdk")
			}

			t.Run("then_delete_virtual_disk_removes_both", func(t *testing.T) {
				if _, err := r.CallTool("vmware_datastore_file_delete_virtual_disk", map[string]interface{}{
					"datastore": dsName, "name": "c-disk-moved.vmdk", "confirm": true,
				}); err != nil {
					t.Fatalf("vmware_datastore_file_delete_virtual_disk failed: %v", err)
				}
				if dfmExists(t, ctx, ds, "c-disk-moved.vmdk") || dfmExists(t, ctx, ds, "c-disk-moved-flat.vmdk") {
					t.Fatal("both descriptor and backing extent should be gone after delete_virtual_disk")
				}
			})
		})
	})

	t.Run("delete_file_orphans_the_flat_extent", func(t *testing.T) {
		dfmUpload(t, ctx, ds, "df-disk.vmdk", "descriptor")
		dfmUpload(t, ctx, ds, "df-disk-flat.vmdk", "extent")

		if _, err := r.CallTool("vmware_datastore_file_delete_file", map[string]interface{}{
			"datastore": dsName, "name": "df-disk.vmdk", "confirm": true,
		}); err != nil {
			t.Fatalf("vmware_datastore_file_delete_file failed: %v", err)
		}
		if dfmExists(t, ctx, ds, "df-disk.vmdk") {
			t.Fatal("df-disk.vmdk descriptor should be gone after delete_file")
		}
		if !dfmExists(t, ctx, ds, "df-disk-flat.vmdk") {
			t.Fatal("df-disk-flat.vmdk should still exist — delete_file is documented as leaving the backing extent orphaned when pointed at a .vmdk descriptor")
		}

		// Clean up the orphan via the virtual-disk-aware delete, proving it
		// can also remove a lone leftover extent's sibling once both names
		// are addressable — not required by the brief, just tidiness.
		dfmUpload(t, ctx, ds, "df-disk.vmdk", "descriptor-again")
		if _, err := r.CallTool("vmware_datastore_file_delete_virtual_disk", map[string]interface{}{
			"datastore": dsName, "name": "df-disk.vmdk", "confirm": true,
		}); err != nil {
			t.Fatalf("cleanup vmware_datastore_file_delete_virtual_disk failed: %v", err)
		}
	})

	t.Run("delete_dispatches_to_delete_virtual_disk_for_vmdk", func(t *testing.T) {
		dfmUpload(t, ctx, ds, "d-disk.vmdk", "descriptor")
		dfmUpload(t, ctx, ds, "d-disk-flat.vmdk", "extent")

		if _, err := r.CallTool("vmware_datastore_file_delete", map[string]interface{}{
			"datastore": dsName, "name": "d-disk.vmdk", "confirm": true,
		}); err != nil {
			t.Fatalf("vmware_datastore_file_delete failed: %v", err)
		}
		if dfmExists(t, ctx, ds, "d-disk.vmdk") || dfmExists(t, ctx, ds, "d-disk-flat.vmdk") {
			t.Fatal("both descriptor and backing extent should be gone — vmware_datastore_file_delete must dispatch to the virtual-disk-aware delete for a .vmdk name")
		}
	})
}

// TestDatastoreFileManagerTools_ForceDeletableFlag exercises
// DeleteVirtualDisk's force:true path (markDiskAsDeletable in the real
// source: download the descriptor, strip a "ddb.deletable" line if present,
// re-upload, THEN delete) against a descriptor that actually contains a
// ddb.deletable line — proving the real download/patch/re-upload/delete
// round trip completes against vcsim without erroring. vcsim's simulator
// does not itself enforce ddb.deletable (DeleteVirtualDiskTask deletes
// unconditionally — confirmed by reading
// referencia/govmomi/simulator/virtual_disk_manager.go), so this cannot
// prove force:true changes the OUTCOME under vcsim — only that the extra
// code path it triggers runs cleanly end-to-end.
func TestDatastoreFileManagerTools_ForceDeletableFlag(t *testing.T) {
	ctx := context.Background()
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newDatastoreFileManagerRegistry(ctx, c, RegistryOptions{AllowDestructive: true})
	dsName := firstDatastoreName(t, r)
	ds, err := resolveDatastore(ctx, r.client, dsName)
	if err != nil {
		t.Fatalf("resolveDatastore(%s) failed: %v", dsName, err)
	}

	dfmUpload(t, ctx, ds, "protected.vmdk", "version=1\nddb.deletable = \"false\"\n")
	dfmUpload(t, ctx, ds, "protected-flat.vmdk", "extent")

	if _, err := r.CallTool("vmware_datastore_file_delete_virtual_disk", map[string]interface{}{
		"datastore": dsName, "name": "protected.vmdk", "force": true, "confirm": true,
	}); err != nil {
		t.Fatalf("vmware_datastore_file_delete_virtual_disk with force:true failed: %v", err)
	}
	if dfmExists(t, ctx, ds, "protected.vmdk") || dfmExists(t, ctx, ds, "protected-flat.vmdk") {
		t.Fatal("both files should be gone after a force:true delete_virtual_disk")
	}
}

// TestDatastoreFileManagerTools_MissingRequiredArgs proves every required
// argument is validated client-side (a clean error, no panic, no network
// call) before any of the 7 DatastoreFileManager-scoped tools would touch
// vcsim.
func TestDatastoreFileManagerTools_MissingRequiredArgs(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()
	ctx := context.Background()

	r := newDatastoreFileManagerRegistry(ctx, c, RegistryOptions{AllowDestructive: true})
	dsName := firstDatastoreName(t, r)

	cases := []struct {
		tool string
		args map[string]interface{}
	}{
		{"vmware_datastore_file_copy", map[string]interface{}{"source": "a", "destination": "b", "confirm": true}},
		{"vmware_datastore_file_copy", map[string]interface{}{"datastore": dsName, "destination": "b", "confirm": true}},
		{"vmware_datastore_file_copy", map[string]interface{}{"datastore": dsName, "source": "a", "confirm": true}},
		{"vmware_datastore_file_delete", map[string]interface{}{"confirm": true}},
		{"vmware_datastore_file_delete", map[string]interface{}{"datastore": dsName, "confirm": true}},
	}

	for i, tc := range cases {
		if _, err := r.CallTool(tc.tool, tc.args); err == nil {
			t.Errorf("case %d (%s): expected an error for incomplete args %v", i, tc.tool, tc.args)
		}
	}
}

// TestDatastoreFileManagerTools_DestructiveGateAndConfirm proves the Fase 1a
// protection layers (server gate + strict confirm:true) apply to both a
// Tier 2 (copy_file) and a Tier 1 (delete_file) tool in this group, and that
// a denied call leaves the datastore state unchanged.
func TestDatastoreFileManagerTools_DestructiveGateAndConfirm(t *testing.T) {
	ctx := context.Background()
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newDatastoreFileManagerRegistry(ctx, c, RegistryOptions{AllowDestructive: true})
	dsName := firstDatastoreName(t, r)
	ds, err := resolveDatastore(ctx, r.client, dsName)
	if err != nil {
		t.Fatalf("resolveDatastore(%s) failed: %v", dsName, err)
	}
	dfmUpload(t, ctx, ds, "gate-src.txt", "x")

	closedGate := newDatastoreFileManagerRegistry(ctx, c, RegistryOptions{AllowDestructive: false})

	t.Run("copy_file_tier2", func(t *testing.T) {
		if _, err := closedGate.CallTool("vmware_datastore_file_copy_file", map[string]interface{}{
			"datastore": dsName, "source": "gate-src.txt", "destination": "gate-dst.txt", "confirm": true,
		}); err == nil {
			t.Fatal("expected vmware_datastore_file_copy_file to be denied with the gate closed")
		}
		if _, err := r.CallTool("vmware_datastore_file_copy_file", map[string]interface{}{
			"datastore": dsName, "source": "gate-src.txt", "destination": "gate-dst.txt",
		}); err == nil {
			t.Fatal("expected vmware_datastore_file_copy_file to fail without confirm:true")
		}
		if dfmExists(t, ctx, ds, "gate-dst.txt") {
			t.Fatal("gate-dst.txt should not exist — both denied calls must be no-ops")
		}
	})

	t.Run("delete_file_tier1", func(t *testing.T) {
		if _, err := closedGate.CallTool("vmware_datastore_file_delete_file", map[string]interface{}{
			"datastore": dsName, "name": "gate-src.txt", "confirm": true,
		}); err == nil {
			t.Fatal("expected vmware_datastore_file_delete_file to be denied with the gate closed")
		}
		if _, err := r.CallTool("vmware_datastore_file_delete_file", map[string]interface{}{
			"datastore": dsName, "name": "gate-src.txt",
		}); err == nil {
			t.Fatal("expected vmware_datastore_file_delete_file to fail without confirm:true")
		}
		if !dfmExists(t, ctx, ds, "gate-src.txt") {
			t.Fatal("gate-src.txt should still exist — both denied delete calls must be no-ops")
		}
	})
}

// TestFileManagerScopedTools_RoundTrip exercises the 4 FileManager-scoped
// tools (explicit "datacenter" argument, full bracketed "[datastore] path"
// strings) end to end: make a directory, copy a file into it, move that
// copy, then delete it — against real vcsim file I/O.
func TestFileManagerScopedTools_RoundTrip(t *testing.T) {
	ctx := context.Background()
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newDatastoreFileManagerRegistry(ctx, c, RegistryOptions{AllowDestructive: true})
	dsName := firstDatastoreName(t, r)
	dcName := dfmFirstDatacenterName(t, r)
	ds, err := resolveDatastore(ctx, r.client, dsName)
	if err != nil {
		t.Fatalf("resolveDatastore(%s) failed: %v", dsName, err)
	}

	bracket := func(p string) string { return "[" + dsName + "] " + p }

	dfmUpload(t, ctx, ds, "fm-src.txt", "hello from TestFileManagerScopedTools_RoundTrip")

	t.Run("make_directory", func(t *testing.T) {
		if _, err := r.CallTool("vmware_file_make_directory", map[string]interface{}{
			"path": bracket("fm-dir"), "datacenter": dcName, "confirm": true,
		}); err != nil {
			t.Fatalf("vmware_file_make_directory failed: %v", err)
		}
		if !dfmExists(t, ctx, ds, "fm-dir") {
			t.Fatal("fm-dir does not exist after vmware_file_make_directory")
		}
	})

	t.Run("make_directory_with_missing_parents", func(t *testing.T) {
		if _, err := r.CallTool("vmware_file_make_directory", map[string]interface{}{
			"path": bracket("fm-parent/fm-child"), "datacenter": dcName, "confirm": true,
		}); err == nil {
			t.Fatal("expected an error creating fm-parent/fm-child without create_parent_directories:true and no existing fm-parent")
		}
		if _, err := r.CallTool("vmware_file_make_directory", map[string]interface{}{
			"path": bracket("fm-parent/fm-child"), "datacenter": dcName, "create_parent_directories": true, "confirm": true,
		}); err != nil {
			t.Fatalf("vmware_file_make_directory with create_parent_directories:true failed: %v", err)
		}
		if !dfmExists(t, ctx, ds, "fm-parent/fm-child") {
			t.Fatal("fm-parent/fm-child does not exist after create_parent_directories:true")
		}
	})

	t.Run("copy_datastore_file", func(t *testing.T) {
		raw, err := r.CallTool("vmware_file_copy_datastore_file", map[string]interface{}{
			"source_path": bracket("fm-src.txt"), "source_datacenter": dcName,
			"destination_path": bracket("fm-dir/fm-copy.txt"), "destination_datacenter": dcName,
			"confirm": true,
		})
		if err != nil {
			t.Fatalf("vmware_file_copy_datastore_file failed: %v", err)
		}
		if !dfmExists(t, ctx, ds, "fm-dir/fm-copy.txt") {
			t.Fatalf("fm-dir/fm-copy.txt missing after vmware_file_copy_datastore_file: %s", raw)
		}
	})

	t.Run("move_datastore_file", func(t *testing.T) {
		raw, err := r.CallTool("vmware_file_move_datastore_file", map[string]interface{}{
			"source_path": bracket("fm-dir/fm-copy.txt"), "source_datacenter": dcName,
			"destination_path": bracket("fm-dir/fm-moved.txt"), "destination_datacenter": dcName,
			"confirm": true,
		})
		if err != nil {
			t.Fatalf("vmware_file_move_datastore_file failed: %v", err)
		}
		if dfmExists(t, ctx, ds, "fm-dir/fm-copy.txt") {
			t.Fatal("fm-dir/fm-copy.txt (the source) should be gone after a move")
		}
		if !dfmExists(t, ctx, ds, "fm-dir/fm-moved.txt") {
			t.Fatalf("fm-dir/fm-moved.txt missing after vmware_file_move_datastore_file: %s", raw)
		}
	})

	t.Run("delete_datastore_file", func(t *testing.T) {
		raw, err := r.CallTool("vmware_file_delete_datastore_file", map[string]interface{}{
			"path": bracket("fm-dir/fm-moved.txt"), "datacenter": dcName, "confirm": true,
		})
		if err != nil {
			t.Fatalf("vmware_file_delete_datastore_file failed: %v", err)
		}
		if dfmExists(t, ctx, ds, "fm-dir/fm-moved.txt") {
			t.Fatalf("fm-dir/fm-moved.txt still exists after vmware_file_delete_datastore_file: %s", raw)
		}
	})

	t.Run("missing_required_args", func(t *testing.T) {
		if _, err := r.CallTool("vmware_file_make_directory", map[string]interface{}{"datacenter": dcName, "confirm": true}); err == nil {
			t.Fatal("expected an error with path missing")
		}
		if _, err := r.CallTool("vmware_file_make_directory", map[string]interface{}{"path": bracket("x"), "confirm": true}); err == nil {
			t.Fatal("expected an error with datacenter missing")
		}
		if _, err := r.CallTool("vmware_file_delete_datastore_file", map[string]interface{}{"confirm": true}); err == nil {
			t.Fatal("expected an error with both path and datacenter missing")
		}
	})
}

// TestDatastoreFileManagerTools_MultiDatacenterVCenter proves
// datacenterForDatastore (generated_datastore_filemanagers.go) resolves the
// CORRECT owning Datacenter for the 7 DatastoreFileManager-scoped tools even
// against a vCenter with more than one datacenter — each default-model
// datacenter has its own identically-named datastore ("LocalDS_0"), so a
// wrong-datacenter pick would make the underlying vcsim server fault
// types.InvalidDatastore (or silently operate on the WRONG datacenter's
// backing files, which would then make the Stat-based assertions below fail
// since the file was actually uploaded through the correctly-scoped
// Finder-resolved Datastore for each DC) rather than the operation
// succeeding correctly for both DC0 and DC1 independently.
func TestDatastoreFileManagerTools_MultiDatacenterVCenter(t *testing.T) {
	ctx := context.Background()
	model := simulator.VPX()
	model.Datacenter = 2

	c, cleanup := newSimClient(t, model)
	defer cleanup()

	r := newDatastoreFileManagerRegistry(ctx, c, RegistryOptions{AllowDestructive: true})

	for _, dcPath := range []string{"/DC0/datastore/LocalDS_0", "/DC1/datastore/LocalDS_0"} {
		dcPath := dcPath
		t.Run(dcPath, func(t *testing.T) {
			ds, err := resolveDatastore(ctx, r.client, dcPath)
			if err != nil {
				t.Fatalf("resolveDatastore(%s) failed: %v", dcPath, err)
			}

			dfmUpload(t, ctx, ds, "mdc-src.txt", "hello from "+dcPath)

			if _, err := r.CallTool("vmware_datastore_file_copy_file", map[string]interface{}{
				"datastore": dcPath, "source": "mdc-src.txt", "destination": "mdc-copy.txt", "confirm": true,
			}); err != nil {
				t.Fatalf("vmware_datastore_file_copy_file failed for %s: %v", dcPath, err)
			}
			if !dfmExists(t, ctx, ds, "mdc-copy.txt") {
				t.Fatalf("mdc-copy.txt missing after copy on %s — datacenterForDatastore likely resolved the wrong Datacenter", dcPath)
			}

			if _, err := r.CallTool("vmware_datastore_file_delete_file", map[string]interface{}{
				"datastore": dcPath, "name": "mdc-copy.txt", "confirm": true,
			}); err != nil {
				t.Fatalf("vmware_datastore_file_delete_file failed for %s: %v", dcPath, err)
			}
		})
	}
}

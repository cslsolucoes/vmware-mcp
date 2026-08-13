package tools

import (
	"context"
	"testing"

	"github.com/vmware/govmomi/simulator"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// newHostStorageRegistry builds a Registry the normal way (NewRegistry,
// which wires vm.go/host.go/etc via registerTools) and then manually layers
// this group's tools on top via withClass — same pattern as
// generated_vm_lifecycle_test.go's newLifecycleRegistry, and for the same
// reason: registry.go itself must not be edited by this file (see
// generated_host_storage.go's top doc comment).
func newHostStorageRegistry(ctx context.Context, c *vmware.Client, opts RegistryOptions) *Registry {
	r := NewRegistry(ctx, c, opts)
	r.withClass(modeVSphereGeneral, registerHostStorageTools)
	return r
}

// TestHostStorageTools_Registration proves all 20 tools are registered and
// reachable via ListTools — a basic wiring smoke test before the more
// specific behavioral tests below.
func TestHostStorageTools_Registration(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newHostStorageRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	want := []string{
		"vmware_host_storage_attach_scsi_lun",
		"vmware_host_storage_compute_disk_partition_info",
		"vmware_host_storage_mark_as_local",
		"vmware_host_storage_mark_as_non_local",
		"vmware_host_storage_mark_as_non_ssd",
		"vmware_host_storage_mark_as_ssd",
		"vmware_host_storage_refresh",
		"vmware_host_storage_rescan_all_hba",
		"vmware_host_storage_rescan_vmfs",
		"vmware_host_storage_retrieve_disk_partition_info",
		"vmware_host_storage_query_unresolved_vmfs_volumes",
		"vmware_host_storage_unmount_vmfs_volume",
		"vmware_host_storage_update_disk_partition_info",
		"vmware_host_datastore_create_local",
		"vmware_host_datastore_create_nas",
		"vmware_host_datastore_create_vmfs",
		"vmware_host_datastore_query_available_disks_for_vmfs",
		"vmware_host_datastore_query_vmfs_datastore_create_options",
		"vmware_host_datastore_remove",
		"vmware_host_datastore_resignature_unresolved_vmfs_volumes",
	}
	if len(want) != 20 {
		t.Fatalf("test bug: want list has %d entries, expected 20", len(want))
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

// TestHostStorageTools_RealSuccessPaths exercises the 5 methods vcsim DOES
// implement server-side (see generated_host_storage.go's top doc comment):
// HostStorageSystem.RescanAllHba/RescanVmfs/Refresh and
// HostDatastoreSystem.CreateLocalDatastore/CreateNasDatastore. These are
// full, real success paths — not just registration/gate checks.
func TestHostStorageTools_RealSuccessPaths(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newHostStorageRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	host := firstHostPath(t, r)

	t.Run("rescan_all_hba", func(t *testing.T) {
		raw, err := r.CallTool("vmware_host_storage_rescan_all_hba", map[string]interface{}{"host": host, "confirm": true})
		if err != nil {
			t.Fatalf("vmware_host_storage_rescan_all_hba failed: %v", err)
		}
		m := decodeResult(t, raw)
		if m["result"] != "hbas_rescanned" {
			t.Fatalf("unexpected result: %s", raw)
		}
	})

	t.Run("rescan_vmfs", func(t *testing.T) {
		raw, err := r.CallTool("vmware_host_storage_rescan_vmfs", map[string]interface{}{"host": host, "confirm": true})
		if err != nil {
			t.Fatalf("vmware_host_storage_rescan_vmfs failed: %v", err)
		}
		m := decodeResult(t, raw)
		if m["result"] != "vmfs_rescanned" {
			t.Fatalf("unexpected result: %s", raw)
		}
	})

	t.Run("refresh", func(t *testing.T) {
		raw, err := r.CallTool("vmware_host_storage_refresh", map[string]interface{}{"host": host, "confirm": true})
		if err != nil {
			t.Fatalf("vmware_host_storage_refresh failed: %v", err)
		}
		m := decodeResult(t, raw)
		if m["result"] != "refreshed" {
			t.Fatalf("unexpected result: %s", raw)
		}
	})

	t.Run("create_local_datastore", func(t *testing.T) {
		dir := t.TempDir()
		raw, err := r.CallTool("vmware_host_datastore_create_local", map[string]interface{}{
			"host": host, "name": "local-ds-test", "path": dir, "confirm": true,
		})
		if err != nil {
			t.Fatalf("vmware_host_datastore_create_local failed: %v", err)
		}
		m := decodeResult(t, raw)
		dsPath, _ := m["datastore"].(string)
		if dsPath == "" {
			t.Fatalf("expected a non-empty datastore inventory path (this is exactly the InventoryPath gotcha documented in this file's doc comment) — got: %s", raw)
		}

		// Confirm the datastore is actually visible via the normal
		// vmware_list_datastores tool — proves it was really registered in
		// the inventory, not just that CreateLocalDatastore returned
		// something.
		listRaw, err := r.CallTool("vmware_list_datastores", map[string]interface{}{})
		if err != nil {
			t.Fatalf("vmware_list_datastores failed: %v", err)
		}
		list, _ := decodeResult(t, listRaw)["datastores"].([]interface{})
		found := false
		for _, item := range list {
			entry, ok := item.(map[string]interface{})
			if ok && entry["name"] == "local-ds-test" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("local-ds-test not found in vmware_list_datastores after creation: %s", listRaw)
		}
	})

	t.Run("create_nas_datastore", func(t *testing.T) {
		dir := t.TempDir()
		raw, err := r.CallTool("vmware_host_datastore_create_nas", map[string]interface{}{
			"host": host,
			"spec": map[string]interface{}{
				"remoteHost": "nas.example.com",
				"remotePath": "/export/vol1",
				"localPath":  dir,
				"accessMode": "readWrite",
				"type":       "NFS",
			},
			"confirm": true,
		})
		if err != nil {
			t.Fatalf("vmware_host_datastore_create_nas failed: %v", err)
		}
		m := decodeResult(t, raw)
		dsPath, _ := m["datastore"].(string)
		if dsPath == "" {
			t.Fatalf("expected a non-empty datastore inventory path — got: %s", raw)
		}
	})

	t.Run("create_nas_datastore_missing_remote_host", func(t *testing.T) {
		dir := t.TempDir()
		_, err := r.CallTool("vmware_host_datastore_create_nas", map[string]interface{}{
			"host": host,
			"spec": map[string]interface{}{
				"remotePath": "/export/vol1",
				"localPath":  dir,
			},
			"confirm": true,
		})
		if err == nil {
			t.Fatal("expected an error when spec.remoteHost is missing — vcsim validates this server-side (see simulator/host_datastore_system.go's CreateNasDatastore)")
		}
	})
}

// TestHostStorageTools_UnsimulatedMethods covers the 15 methods vcsim's
// simulator has no server-side implementation for (see
// generated_host_storage.go's top doc comment): each is proven registered,
// rejects bad/missing input before making any network call, and — given
// valid input, gate open, and confirm:true where applicable — reaches the
// real vcsim server and gets back a clean types.MethodNotFound-based error
// via assertReachesServer (generated_vm_lifecycle_test.go, same package),
// not a wiring failure (unknown tool) or a recovered panic.
func TestHostStorageTools_UnsimulatedMethods(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newHostStorageRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	host := firstHostPath(t, r)

	t.Run("query_unresolved_vmfs_volumes", func(t *testing.T) {
		if _, err := r.CallTool("vmware_host_storage_query_unresolved_vmfs_volumes", map[string]interface{}{"host": "does-not-exist"}); err == nil {
			t.Fatal("expected an error for an unresolvable host")
		}
		_, err := r.CallTool("vmware_host_storage_query_unresolved_vmfs_volumes", map[string]interface{}{"host": host})
		assertReachesServer(t, err, "vmware_host_storage_query_unresolved_vmfs_volumes")
	})

	t.Run("retrieve_disk_partition_info", func(t *testing.T) {
		if _, err := r.CallTool("vmware_host_storage_retrieve_disk_partition_info", map[string]interface{}{"host": host}); err == nil {
			t.Fatal("expected an error when device_path is missing")
		}
		_, err := r.CallTool("vmware_host_storage_retrieve_disk_partition_info", map[string]interface{}{"host": host, "device_path": "/vmfs/devices/disks/naa.fake"})
		assertReachesServer(t, err, "vmware_host_storage_retrieve_disk_partition_info")
	})

	t.Run("compute_disk_partition_info", func(t *testing.T) {
		if _, err := r.CallTool("vmware_host_storage_compute_disk_partition_info", map[string]interface{}{"host": host, "device_path": "/vmfs/devices/disks/naa.fake"}); err == nil {
			t.Fatal("expected an error when layout is missing")
		}
		_, err := r.CallTool("vmware_host_storage_compute_disk_partition_info", map[string]interface{}{
			"host": host, "device_path": "/vmfs/devices/disks/naa.fake",
			"layout": map[string]interface{}{"partition": []interface{}{}},
		})
		assertReachesServer(t, err, "vmware_host_storage_compute_disk_partition_info")
	})

	t.Run("attach_scsi_lun", func(t *testing.T) {
		closedGate := newHostStorageRegistry(context.Background(), c, RegistryOptions{})
		if _, err := closedGate.CallTool("vmware_host_storage_attach_scsi_lun", map[string]interface{}{"host": host, "uuid": "fake-uuid", "confirm": true}); err == nil {
			t.Fatal("expected the closed destructive gate to deny attach_scsi_lun")
		}
		if _, err := r.CallTool("vmware_host_storage_attach_scsi_lun", map[string]interface{}{"host": host, "uuid": "fake-uuid"}); err == nil {
			t.Fatal("expected an error without confirm:true")
		}
		_, err := r.CallTool("vmware_host_storage_attach_scsi_lun", map[string]interface{}{"host": host, "uuid": "fake-uuid", "confirm": true})
		assertReachesServer(t, err, "vmware_host_storage_attach_scsi_lun")
	})

	t.Run("mark_as_local", func(t *testing.T) {
		_, err := r.CallTool("vmware_host_storage_mark_as_local", map[string]interface{}{"host": host, "uuid": "fake-uuid", "confirm": true})
		assertReachesServer(t, err, "vmware_host_storage_mark_as_local")
	})

	t.Run("mark_as_non_local", func(t *testing.T) {
		_, err := r.CallTool("vmware_host_storage_mark_as_non_local", map[string]interface{}{"host": host, "uuid": "fake-uuid", "confirm": true})
		assertReachesServer(t, err, "vmware_host_storage_mark_as_non_local")
	})

	t.Run("mark_as_ssd", func(t *testing.T) {
		_, err := r.CallTool("vmware_host_storage_mark_as_ssd", map[string]interface{}{"host": host, "uuid": "fake-uuid", "confirm": true})
		assertReachesServer(t, err, "vmware_host_storage_mark_as_ssd")
	})

	t.Run("mark_as_non_ssd", func(t *testing.T) {
		_, err := r.CallTool("vmware_host_storage_mark_as_non_ssd", map[string]interface{}{"host": host, "uuid": "fake-uuid", "confirm": true})
		assertReachesServer(t, err, "vmware_host_storage_mark_as_non_ssd")
	})

	t.Run("update_disk_partition_info", func(t *testing.T) {
		if _, err := r.CallTool("vmware_host_storage_update_disk_partition_info", map[string]interface{}{"host": host, "device_path": "/vmfs/devices/disks/naa.fake", "confirm": true}); err == nil {
			t.Fatal("expected an error when spec is missing")
		}
		_, err := r.CallTool("vmware_host_storage_update_disk_partition_info", map[string]interface{}{
			"host": host, "device_path": "/vmfs/devices/disks/naa.fake",
			"spec":    map[string]interface{}{"partitionFormat": "gpt"},
			"confirm": true,
		})
		assertReachesServer(t, err, "vmware_host_storage_update_disk_partition_info")
	})

	t.Run("unmount_vmfs_volume", func(t *testing.T) {
		closedGate := newHostStorageRegistry(context.Background(), c, RegistryOptions{})
		if _, err := closedGate.CallTool("vmware_host_storage_unmount_vmfs_volume", map[string]interface{}{"host": host, "vmfs_uuid": "fake-uuid", "confirm": true}); err == nil {
			t.Fatal("expected the closed destructive gate to deny unmount_vmfs_volume (tier 1)")
		}
		_, err := r.CallTool("vmware_host_storage_unmount_vmfs_volume", map[string]interface{}{"host": host, "vmfs_uuid": "fake-uuid", "confirm": true})
		assertReachesServer(t, err, "vmware_host_storage_unmount_vmfs_volume")
	})

	t.Run("query_available_disks_for_vmfs", func(t *testing.T) {
		_, err := r.CallTool("vmware_host_datastore_query_available_disks_for_vmfs", map[string]interface{}{"host": host})
		assertReachesServer(t, err, "vmware_host_datastore_query_available_disks_for_vmfs")
	})

	t.Run("query_vmfs_datastore_create_options", func(t *testing.T) {
		if _, err := r.CallTool("vmware_host_datastore_query_vmfs_datastore_create_options", map[string]interface{}{"host": host}); err == nil {
			t.Fatal("expected an error when device_path is missing")
		}
		_, err := r.CallTool("vmware_host_datastore_query_vmfs_datastore_create_options", map[string]interface{}{"host": host, "device_path": "/vmfs/devices/disks/naa.fake"})
		assertReachesServer(t, err, "vmware_host_datastore_query_vmfs_datastore_create_options")
	})

	t.Run("create_vmfs_datastore", func(t *testing.T) {
		if _, err := r.CallTool("vmware_host_datastore_create_vmfs", map[string]interface{}{"host": host, "confirm": true}); err == nil {
			t.Fatal("expected an error when spec is missing")
		}
		_, err := r.CallTool("vmware_host_datastore_create_vmfs", map[string]interface{}{
			"host": host,
			"spec": map[string]interface{}{
				"partition": map[string]interface{}{"partitionFormat": "gpt"},
				"vmfs":      map[string]interface{}{"volumeName": "vmfs-test"},
			},
			"confirm": true,
		})
		assertReachesServer(t, err, "vmware_host_datastore_create_vmfs")
	})

	t.Run("resignature_unresolved_vmfs_volumes", func(t *testing.T) {
		if _, err := r.CallTool("vmware_host_datastore_resignature_unresolved_vmfs_volumes", map[string]interface{}{"host": host, "confirm": true}); err == nil {
			t.Fatal("expected an error when device_paths is missing")
		}
		if _, err := r.CallTool("vmware_host_datastore_resignature_unresolved_vmfs_volumes", map[string]interface{}{"host": host, "device_paths": []interface{}{}, "confirm": true}); err == nil {
			t.Fatal("expected an error when device_paths is empty")
		}
		_, err := r.CallTool("vmware_host_datastore_resignature_unresolved_vmfs_volumes", map[string]interface{}{
			"host": host, "device_paths": []interface{}{"/vmfs/devices/disks/naa.fake"}, "confirm": true,
		})
		assertReachesServer(t, err, "vmware_host_datastore_resignature_unresolved_vmfs_volumes")
	})

	t.Run("remove_datastore", func(t *testing.T) {
		// Use a real datastore (vcsim's default local datastore) so the
		// failure genuinely comes from RemoveDatastore being unsimulated,
		// not from resolveDatastore failing to find anything first.
		listRaw, err := r.CallTool("vmware_list_datastores", map[string]interface{}{})
		if err != nil {
			t.Fatalf("vmware_list_datastores failed: %v", err)
		}
		list, _ := decodeResult(t, listRaw)["datastores"].([]interface{})
		if len(list) == 0 {
			t.Fatal("simulator model has no datastores")
		}
		first, _ := list[0].(map[string]interface{})
		dsName, _ := first["name"].(string)
		if dsName == "" {
			t.Fatalf("could not read a datastore name from %s", listRaw)
		}

		closedGate := newHostStorageRegistry(context.Background(), c, RegistryOptions{})
		if _, err := closedGate.CallTool("vmware_host_datastore_remove", map[string]interface{}{"host": host, "datastore": dsName, "confirm": true}); err == nil {
			t.Fatal("expected the closed destructive gate to deny remove (tier 1)")
		}

		_, err = r.CallTool("vmware_host_datastore_remove", map[string]interface{}{"host": host, "datastore": dsName, "confirm": true})
		assertReachesServer(t, err, "vmware_host_datastore_remove")
	})
}

// TestHostStorageTools_VPXSanity spot-checks this group against
// simulator.VPX() (a vCenter-fronted topology), not just simulator.ESX()
// (standalone host) used by every test above — per this task's brief to try
// both and note which works. HostStorageSystem/HostDatastoreSystem are
// per-host managed objects backed by the exact same simulator types
// (simulator/host_storage_system.go, host_datastore_system.go) regardless of
// whether a vCenter fronts the host, so a single real-success-path spot
// check plus a resolveHost/gate check is enough evidence here — this is not
// a second full copy of every subtest above.
func TestHostStorageTools_VPXSanity(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newHostStorageRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	host := firstHostPath(t, r)

	raw, err := r.CallTool("vmware_host_storage_rescan_all_hba", map[string]interface{}{"host": host, "confirm": true})
	if err != nil {
		t.Fatalf("vmware_host_storage_rescan_all_hba failed against simulator.VPX(): %v", err)
	}
	if decodeResult(t, raw)["result"] != "hbas_rescanned" {
		t.Fatalf("unexpected result against simulator.VPX(): %s", raw)
	}

	closedGate := newHostStorageRegistry(context.Background(), c, RegistryOptions{})
	if _, err := closedGate.CallTool("vmware_host_storage_unmount_vmfs_volume", map[string]interface{}{"host": host, "vmfs_uuid": "fake", "confirm": true}); err == nil {
		t.Fatal("expected the closed destructive gate to deny unmount_vmfs_volume against simulator.VPX()")
	}
}

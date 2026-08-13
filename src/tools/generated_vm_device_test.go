package tools

import (
	"context"
	"testing"

	"github.com/vmware/govmomi/simulator"
)

// newVMDeviceTestRegistry builds a Registry against simulator.ESX() and
// layers registerVMDeviceTools onto it via r.withClass — registry.go's
// registerTools doesn't call registerVMDeviceTools yet (this file's tools
// aren't wired into the real server until a human reviews and merges this
// slice alongside the sibling lifecycle/provisioning slices), so every test
// in this file builds its own registry this way instead of using
// NewRegistry alone.
func newVMDeviceTestRegistry(t *testing.T, opts RegistryOptions) (*Registry, func()) {
	t.Helper()
	c, cleanup := newSimClient(t, simulator.ESX())
	r := NewRegistry(context.Background(), c, opts)
	r.withClass(modeVSphereGeneral, registerVMDeviceTools)
	return r, cleanup
}

// vmDeviceList returns the raw "device" list vmware_vm_device would expose,
// read directly via CallTool's underlying vm.Device(ctx) equivalent — since
// vmware_vm_device (the sibling read-only tool from the lifecycle slice)
// isn't registered in this test registry, tests assert on
// vmware_vm_add_device/vmware_vm_remove_device's own results instead of a
// separate list call. deviceKeysByType is a small local helper used only to
// locate a device this file's own tools just added, via the govmomi client
// directly (not through an MCP tool), which is legitimate for test setup
// even though production tool code never reaches past the Registry.
func deviceKeysByType(t *testing.T, r *Registry, vmPath string, want string) []int32 {
	t.Helper()
	vm, err := resolveVM(context.Background(), r.client, map[string]interface{}{"vm": vmPath})
	if err != nil {
		t.Fatalf("resolveVM(%s) failed: %v", vmPath, err)
	}
	devices, err := vm.Device(context.Background())
	if err != nil {
		t.Fatalf("vm.Device failed: %v", err)
	}
	var keys []int32
	for _, d := range devices {
		if devices.Type(d) == want {
			keys = append(keys, d.GetVirtualDevice().Key)
		}
	}
	return keys
}

// firstDatastoreName returns the name (not path) of the first datastore in
// the simulator model — vmware_vm_add_device's "datastore" arg and
// vmware_vm_attach_disk's "datastore" arg both take a name/pattern, not an
// inventory path.
func firstDatastoreName(t *testing.T, r *Registry) string {
	t.Helper()
	raw, err := r.CallTool("vmware_list_datastores", map[string]interface{}{})
	if err != nil {
		t.Fatalf("vmware_list_datastores failed: %v", err)
	}
	list, _ := decodeResult(t, raw)["datastores"].([]interface{})
	if len(list) == 0 {
		t.Fatal("simulator model has no datastores")
	}
	first, _ := list[0].(map[string]interface{})
	name, _ := first["name"].(string)
	if name == "" {
		t.Fatal("first datastore entry has no name field")
	}
	return name
}

// TestVMDevice_AddCdromWithIsoThenRemove proves the full add -> verify ->
// remove -> verify cycle for a CD-ROM device against vcsim: an ISO-backed
// CD-ROM actually appears in vm.Device() after vmware_vm_add_device, and is
// genuinely gone after vmware_vm_remove_device — not just that both calls
// returned no error.
func TestVMDevice_AddCdromWithIsoThenRemove(t *testing.T) {
	r, cleanup := newVMDeviceTestRegistry(t, RegistryOptions{AllowDestructive: true})
	defer cleanup()
	vm := firstVMPath(t, r)

	before := deviceKeysByType(t, r, vm, "cdrom")

	raw, err := r.CallTool("vmware_vm_add_device", map[string]interface{}{
		"vm":          vm,
		"device_type": "cdrom",
		"iso_path":    "[LocalDS_0] ISOs/test.iso",
		"confirm":     true,
	})
	if err != nil {
		t.Fatalf("vmware_vm_add_device(cdrom) failed: %v", err)
	}
	if m := decodeResult(t, raw); m["result"] != "added" {
		t.Fatalf("expected result=added, got %v (%s)", m["result"], raw)
	}

	after := deviceKeysByType(t, r, vm, "cdrom")
	if len(after) != len(before)+1 {
		t.Fatalf("expected exactly 1 new cdrom device, before=%v after=%v", before, after)
	}

	// The new key is whichever one wasn't present before.
	var newKey int32 = -1
	seen := make(map[int32]bool, len(before))
	for _, k := range before {
		seen[k] = true
	}
	for _, k := range after {
		if !seen[k] {
			newKey = k
			break
		}
	}
	if newKey == -1 {
		t.Fatalf("could not identify the newly added cdrom device key: before=%v after=%v", before, after)
	}

	if _, err := r.CallTool("vmware_vm_remove_device", map[string]interface{}{
		"vm":         vm,
		"device_key": float64(newKey),
		"confirm":    true,
	}); err != nil {
		t.Fatalf("vmware_vm_remove_device failed: %v", err)
	}

	final := deviceKeysByType(t, r, vm, "cdrom")
	if len(final) != len(before) {
		t.Fatalf("expected the added cdrom to be gone after remove, before=%v final=%v", before, final)
	}
}

// TestVMDevice_AddDisk proves vmware_vm_add_device with device_type "disk"
// actually adds a new virtual disk of the requested size against vcsim.
func TestVMDevice_AddDisk(t *testing.T) {
	r, cleanup := newVMDeviceTestRegistry(t, RegistryOptions{AllowDestructive: true})
	defer cleanup()
	vm := firstVMPath(t, r)
	ds := firstDatastoreName(t, r)

	before := deviceKeysByType(t, r, vm, "disk")

	raw, err := r.CallTool("vmware_vm_add_device", map[string]interface{}{
		"vm":          vm,
		"device_type": "disk",
		"datastore":   ds,
		"capacity_kb": float64(2 * 1024 * 1024), // 2GB
		"file_name":   "extra-disk.vmdk",
		"confirm":     true,
	})
	if err != nil {
		t.Fatalf("vmware_vm_add_device(disk) failed: %v", err)
	}
	if m := decodeResult(t, raw); m["result"] != "added" {
		t.Fatalf("expected result=added, got %v (%s)", m["result"], raw)
	}

	after := deviceKeysByType(t, r, vm, "disk")
	if len(after) != len(before)+1 {
		t.Fatalf("expected exactly 1 new disk device, before=%v after=%v", before, after)
	}
}

// TestVMDevice_EditDeviceCdromIsoSwapAndEject proves EditDevice's MVP scope
// (CD-ROM backing only): inserting an ISO into the VM's existing (empty)
// CD-ROM, then ejecting it back.
func TestVMDevice_EditDeviceCdromIsoSwapAndEject(t *testing.T) {
	r, cleanup := newVMDeviceTestRegistry(t, RegistryOptions{AllowDestructive: true})
	defer cleanup()
	vm := firstVMPath(t, r)

	cdromKeys := deviceKeysByType(t, r, vm, "cdrom")
	if len(cdromKeys) == 0 {
		t.Fatal("expected the default simulator VM to already have a cdrom device")
	}
	key := cdromKeys[0]

	if _, err := r.CallTool("vmware_vm_edit_device", map[string]interface{}{
		"vm":         vm,
		"device_key": float64(key),
		"iso_path":   "[LocalDS_0] ISOs/swap.iso",
		"confirm":    true,
	}); err != nil {
		t.Fatalf("vmware_vm_edit_device(iso_path) failed: %v", err)
	}

	if _, err := r.CallTool("vmware_vm_edit_device", map[string]interface{}{
		"vm":         vm,
		"device_key": float64(key),
		"eject":      true,
		"confirm":    true,
	}); err != nil {
		t.Fatalf("vmware_vm_edit_device(eject) failed: %v", err)
	}

	// Both iso_path and eject together must be rejected.
	if _, err := r.CallTool("vmware_vm_edit_device", map[string]interface{}{
		"vm":         vm,
		"device_key": float64(key),
		"iso_path":   "[LocalDS_0] ISOs/a.iso",
		"eject":      true,
		"confirm":    true,
	}); err == nil {
		t.Fatal("expected iso_path + eject together to be rejected")
	}

	// Neither iso_path nor eject must be rejected too.
	if _, err := r.CallTool("vmware_vm_edit_device", map[string]interface{}{
		"vm":         vm,
		"device_key": float64(key),
		"confirm":    true,
	}); err == nil {
		t.Fatal("expected neither iso_path nor eject to be rejected")
	}
}

// TestVMDevice_EditDeviceRejectsNonCdrom proves the MVP-scope guard: editing
// a device that resolves but isn't a CD-ROM (e.g. the VM's disk) is a clear
// error, not a panic or a silent no-op.
func TestVMDevice_EditDeviceRejectsNonCdrom(t *testing.T) {
	r, cleanup := newVMDeviceTestRegistry(t, RegistryOptions{AllowDestructive: true})
	defer cleanup()
	vm := firstVMPath(t, r)

	diskKeys := deviceKeysByType(t, r, vm, "disk")
	if len(diskKeys) == 0 {
		t.Fatal("expected the default simulator VM to already have a disk device")
	}

	if _, err := r.CallTool("vmware_vm_edit_device", map[string]interface{}{
		"vm":         vm,
		"device_key": float64(diskKeys[0]),
		"eject":      true,
		"confirm":    true,
	}); err == nil {
		t.Fatal("expected editing a non-cdrom device to be rejected")
	}
}

// TestVMDevice_AddDeviceRejectsMalformedDeviceType proves an unknown
// device_type is a clean error, not a panic — CallTool's own panic recovery
// (registry.go) would also catch a real crash, but this proves the handler
// itself validates instead of relying on that safety net.
func TestVMDevice_AddDeviceRejectsMalformedDeviceType(t *testing.T) {
	r, cleanup := newVMDeviceTestRegistry(t, RegistryOptions{AllowDestructive: true})
	defer cleanup()
	vm := firstVMPath(t, r)

	_, err := r.CallTool("vmware_vm_add_device", map[string]interface{}{
		"vm":          vm,
		"device_type": "nic",
		"confirm":     true,
	})
	if err == nil {
		t.Fatal("expected an error for an unsupported device_type")
	}
}

// TestVMDevice_AddDeviceDiskRequiresDatastoreAndCapacity proves the
// disk-specific required fields are validated with a clear error.
func TestVMDevice_AddDeviceDiskRequiresDatastoreAndCapacity(t *testing.T) {
	r, cleanup := newVMDeviceTestRegistry(t, RegistryOptions{AllowDestructive: true})
	defer cleanup()
	vm := firstVMPath(t, r)

	if _, err := r.CallTool("vmware_vm_add_device", map[string]interface{}{
		"vm":          vm,
		"device_type": "disk",
		"confirm":     true,
	}); err == nil {
		t.Fatal("expected an error when datastore/capacity_kb are missing for device_type=disk")
	}
}

// TestVMDevice_RemoveDeviceRejectsUnknownKey proves a device_key that
// doesn't resolve to any device is a clean error.
func TestVMDevice_RemoveDeviceRejectsUnknownKey(t *testing.T) {
	r, cleanup := newVMDeviceTestRegistry(t, RegistryOptions{AllowDestructive: true})
	defer cleanup()
	vm := firstVMPath(t, r)

	if _, err := r.CallTool("vmware_vm_remove_device", map[string]interface{}{
		"vm":         vm,
		"device_key": float64(999999),
		"confirm":    true,
	}); err == nil {
		t.Fatal("expected an error for a device_key that doesn't exist")
	}
}

// TestVMDevice_GateAndConfirmDenyAllFiveTools proves every one of the 5
// tools in this file is actually wired through registerDestructive — a
// closed gate denies each before it touches the simulator, and each also
// requires confirm:true with the gate open. Matches host_test.go's
// TestHostTools_MaintenanceEnterGateAndConfirm pattern, applied to all 5
// tools in one test since they share the same vm/args-shape setup.
func TestVMDevice_GateAndConfirmDenyAllFiveTools(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	probe := NewRegistry(context.Background(), c, RegistryOptions{})
	probe.withClass(modeVSphereGeneral, registerVMDeviceTools)
	vm := firstVMPath(t, probe)
	ds := firstDatastoreName(t, probe)
	cdromKey := deviceKeysByType(t, probe, vm, "cdrom")[0]
	diskKey := deviceKeysByType(t, probe, vm, "disk")[0]

	cases := []struct {
		tool string
		args map[string]interface{}
	}{
		{"vmware_vm_add_device", map[string]interface{}{"vm": vm, "device_type": "cdrom"}},
		{"vmware_vm_edit_device", map[string]interface{}{"vm": vm, "device_key": float64(cdromKey), "eject": true}},
		{"vmware_vm_attach_disk", map[string]interface{}{"vm": vm, "id": "some/disk.vmdk", "datastore": ds}},
		{"vmware_vm_remove_device", map[string]interface{}{"vm": vm, "device_key": float64(diskKey)}},
		{"vmware_vm_detach_disk", map[string]interface{}{"vm": vm, "id": "some/disk.vmdk"}},
	}

	closedGate, cleanupClosed := newVMDeviceTestRegistry(t, RegistryOptions{AllowDestructive: false})
	defer cleanupClosed()
	openGateNoConfirm, cleanupOpen := newVMDeviceTestRegistry(t, RegistryOptions{AllowDestructive: true})
	defer cleanupOpen()

	for _, tc := range cases {
		t.Run(tc.tool+"/gate_closed", func(t *testing.T) {
			withConfirm := map[string]interface{}{"confirm": true}
			for k, v := range tc.args {
				withConfirm[k] = v
			}
			if _, err := closedGate.CallTool(tc.tool, withConfirm); err == nil {
				t.Fatalf("expected %s to be denied with the gate closed", tc.tool)
			}
		})
		t.Run(tc.tool+"/missing_confirm", func(t *testing.T) {
			if _, err := openGateNoConfirm.CallTool(tc.tool, tc.args); err == nil {
				t.Fatalf("expected %s to fail without confirm:true", tc.tool)
			}
		})
	}
}

// TestVMDevice_AttachDiskVcsimLimitation documents a real vcsim constraint
// found empirically (not assumed) while writing this suite:
// VirtualMachine.AttachDisk resolves the target disk via
// vm.fcd(ctx, datastore, diskId) in the simulator (see
// referencia/govmomi/simulator/virtual_machine.go's AttachDiskTask), which
// only recognizes First Class Disks (VStorageObjects registered through the
// VStorageObjectManager) — a plain vmdk path that was never registered as
// an FCD does not resolve.
//
// Critically, simulator.Registry.VStorageObjectManager() (see
// simulator/registry.go) unconditionally type-asserts the singleton to
// *simulator.VcenterVStorageObjectManager. Against simulator.ESX()
// (standalone-host mode, used by every other test in this package) that
// object is actually a *simulator.HostVStorageObjectManager, so the
// assertion PANICS — inside a goroutine vcsim's own Task.Run spawns with no
// recover, which is fatal to the whole test process, not something
// CallTool's panic recovery (registry.go) can catch (that recover only
// covers the handler's own goroutine, not a task goroutine vcsim spawns
// internally). This was confirmed by triggering it once during development;
// it is deliberately NOT exercised against simulator.ESX() here.
//
// Against simulator.VPX() (vCenter mode) the real VcenterVStorageObjectManager
// is registered and AttachDisk instead returns a clean *types.InvalidArgument
// fault for an unregistered disk id — proven below. This means
// vmware_vm_attach_disk's real (non-gate-denied) behavior is only verifiable
// against a vCenter-backed vcsim in this project's test suite; a standalone
// ESXi target is untested beyond the gate/confirm denial path (see
// TestVMDevice_GateAndConfirmDenyAllFiveTools) — flagging this for a human
// reviewer, since every other tool in this file is verified against
// simulator.ESX() like the rest of the package.
func TestVMDevice_AttachDiskVcsimLimitation(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := NewRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	r.withClass(modeVSphereGeneral, registerVMDeviceTools)
	vm := firstVMPath(t, r)
	ds := firstDatastoreName(t, r)

	_, err := r.CallTool("vmware_vm_attach_disk", map[string]interface{}{
		"vm":        vm,
		"id":        "not-a-registered-fcd.vmdk",
		"datastore": ds,
		"confirm":   true,
	})
	if err == nil {
		t.Fatal("expected vcsim (VPX) to reject attaching a disk id that was never registered as a First Class Disk")
	}
}

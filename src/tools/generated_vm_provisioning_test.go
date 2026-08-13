package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/vmware/govmomi/simulator"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// newVMProvisioningRegistry layers registerVMProvisioningTools
// (modeVSphereGeneral) and registerVMProvisioningVCenterOnlyTools
// (modeVCenterOnly) onto a fresh Registry without touching registry.go —
// exactly as the task brief for this file directs, mirroring how
// registry.go's registerTools wires every other domain's register function.
func newVMProvisioningRegistry(c *vmware.Client, opts RegistryOptions) *Registry {
	r := NewRegistry(context.Background(), c, opts)
	r.withClass(modeVSphereGeneral, registerVMProvisioningTools)
	r.withClass(modeVCenterOnly, registerVMProvisioningVCenterOnlyTools)
	return r
}

// TestVMProvisioning_Registered proves all 11 tools are present under an
// unrestricted ConnectionMode, and that the 3 checker tools (only) drop out
// under ConnectionModeVMware (standalone ESXi has no vCenter-only surface) —
// the exact filtering the nil-ServiceContent guard in this file's doc
// comment depends on being wired correctly.
func TestVMProvisioning_Registered(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	generalTools := []string{
		"vmware_vm_clone",
		"vmware_vm_instant_clone",
		"vmware_vm_relocate",
		"vmware_vm_migrate",
		"vmware_vm_customize",
		"vmware_vm_export",
		"vmware_vm_export_snapshot",
		"vmware_vm_promote_disks",
	}
	vcenterOnlyTools := []string{
		"vmware_vm_provisioning_check_relocate",
		"vmware_vm_compatibility_check_config",
		"vmware_vm_compatibility_check",
	}

	t.Run("unrestricted", func(t *testing.T) {
		r := newVMProvisioningRegistry(c, RegistryOptions{})
		names := make(map[string]bool)
		for _, tool := range r.ListTools() {
			names[tool.Name] = true
		}
		for _, name := range append(append([]string{}, generalTools...), vcenterOnlyTools...) {
			if !names[name] {
				t.Errorf("expected %s to be registered under the unrestricted ConnectionMode", name)
			}
		}
	})

	t.Run("vmware_mode_excludes_vcenter_only", func(t *testing.T) {
		r := newVMProvisioningRegistry(c, RegistryOptions{ConnectionMode: ConnectionModeVMware})
		names := make(map[string]bool)
		for _, tool := range r.ListTools() {
			names[tool.Name] = true
		}
		for _, name := range generalTools {
			if !names[name] {
				t.Errorf("expected %s to be registered under ConnectionModeVMware", name)
			}
		}
		for _, name := range vcenterOnlyTools {
			if names[name] {
				t.Errorf("expected %s to be EXCLUDED under ConnectionModeVMware (standalone ESXi has no vCenter-only surface)", name)
			}
			if _, err := r.CallTool(name, map[string]interface{}{}); err == nil || !strings.Contains(err.Error(), "unknown tool") {
				t.Errorf("expected CallTool(%s) to report an unknown tool under ConnectionModeVMware, got: %v", name, err)
			}
		}
	})

	t.Run("vcenter_mode_includes_everything", func(t *testing.T) {
		r := newVMProvisioningRegistry(c, RegistryOptions{ConnectionMode: ConnectionModeVCenter})
		names := make(map[string]bool)
		for _, tool := range r.ListTools() {
			names[tool.Name] = true
		}
		for _, name := range append(append([]string{}, generalTools...), vcenterOnlyTools...) {
			if !names[name] {
				t.Errorf("expected %s to be registered under ConnectionModeVCenter", name)
			}
		}
	})
}

// TestVMProvisioning_CheckerNilServiceContentGuard is the single most
// important test in this file: it proves the exact bug this task exists to
// prevent. object.NewVmProvisioningChecker/NewVmCompatibilityChecker
// dereference client.ServiceContent.VmProvisioningChecker/
// VmCompatibilityChecker, both nil on standalone ESXi (confirmed by reading
// referencia/govmomi/simulator/esx/service_content.go) — calling either
// constructor unguarded PANICS. This test proves each of the 3 checker
// tools instead returns a clean, informative error (via
// requireProvisioningChecker/requireCompatibilityChecker in
// generated_vm_provisioning.go) — NOT a generic "tool X panicked: ..."
// message, which would mean this handler-level guard isn't actually wired
// and registry.go's CallTool recover() is doing all the work instead.
func TestVMProvisioning_CheckerNilServiceContentGuard(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newVMProvisioningRegistry(c, RegistryOptions{})
	vm := firstVMPath(t, r)

	cases := []struct {
		tool string
		args map[string]interface{}
	}{
		{"vmware_vm_provisioning_check_relocate", map[string]interface{}{"vm": vm, "spec": map[string]interface{}{}}},
		{"vmware_vm_compatibility_check_config", map[string]interface{}{"vm": vm, "spec": map[string]interface{}{}}},
		{"vmware_vm_compatibility_check", map[string]interface{}{"vm": vm}},
	}

	for _, tc := range cases {
		t.Run(tc.tool, func(t *testing.T) {
			_, err := r.CallTool(tc.tool, tc.args)
			if err == nil {
				t.Fatalf("expected %s to fail against standalone ESXi (no VmProvisioningChecker/VmCompatibilityChecker), got success", tc.tool)
			}
			if strings.Contains(err.Error(), "panicked") {
				t.Fatalf("%s: expected a clean guard error, got a recovered panic instead: %v", tc.tool, err)
			}
			if !strings.Contains(err.Error(), "not available on this connection") {
				t.Fatalf("%s: expected the explicit nil-ServiceContent guard error, got: %v", tc.tool, err)
			}
			if !strings.Contains(err.Error(), "vCenter") {
				t.Fatalf("%s: expected the guard error to explain vCenter is required, got: %v", tc.tool, err)
			}
		})
	}
}

// TestVMProvisioning_CheckersAgainstVPX proves CheckRelocate/CheckVmConfig/
// CheckCompatibility actually work end-to-end against a vCenter-shaped
// simulator (VmProvisioningChecker/VmCompatibilityChecker ARE non-nil on
// simulator.VPX() — see referencia/govmomi/simulator/vpx/service_content.go)
// — the positive counterpart to the nil-guard test above.
func TestVMProvisioning_CheckersAgainstVPX(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newVMProvisioningRegistry(c, RegistryOptions{})
	vm := firstVMPath(t, r)

	t.Run("CheckRelocate", func(t *testing.T) {
		raw, err := r.CallTool("vmware_vm_provisioning_check_relocate", map[string]interface{}{
			"vm":   vm,
			"spec": map[string]interface{}{},
		})
		if err != nil {
			t.Fatalf("vmware_vm_provisioning_check_relocate failed against VPX: %v", err)
		}
		if countOf(t, raw) == 0 {
			t.Fatalf("expected at least 1 check result, got 0: %s", raw)
		}
	})

	t.Run("CheckCompatibility", func(t *testing.T) {
		raw, err := r.CallTool("vmware_vm_compatibility_check", map[string]interface{}{"vm": vm})
		if err != nil {
			t.Fatalf("vmware_vm_compatibility_check failed against VPX: %v", err)
		}
		if countOf(t, raw) == 0 {
			t.Fatalf("expected at least 1 check result, got 0: %s", raw)
		}
	})

	t.Run("CheckVmConfig", func(t *testing.T) {
		raw, err := r.CallTool("vmware_vm_compatibility_check_config", map[string]interface{}{
			"vm":   vm,
			"spec": map[string]interface{}{"numCPUs": float64(2)},
		})
		if err != nil {
			t.Fatalf("vmware_vm_compatibility_check_config failed against VPX: %v", err)
		}
		if countOf(t, raw) == 0 {
			t.Fatalf("expected at least 1 check result, got 0: %s", raw)
		}
	})

	t.Run("CheckVmConfig_requires_vm_host_or_pool", func(t *testing.T) {
		if _, err := r.CallTool("vmware_vm_compatibility_check_config", map[string]interface{}{
			"spec": map[string]interface{}{"numCPUs": float64(2)},
		}); err == nil {
			t.Fatal("expected vmware_vm_compatibility_check_config to fail when none of vm/host/pool is given")
		}
	})
}

// TestVMProvisioning_Clone proves the most commonly used tool end-to-end:
// clone a VM (an empty spec.location is enough on standalone ESXi, where
// simulator.CloneVMTask defaults the pool to the source VM's own resource
// pool — see referencia/govmomi/simulator/virtual_machine.go), then confirm
// the new VM resolves via vmware_vm_info.
func TestVMProvisioning_Clone(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newVMProvisioningRegistry(c, RegistryOptions{AllowDestructive: true})
	vm := firstVMPath(t, r)

	raw, err := r.CallTool("vmware_vm_clone", map[string]interface{}{
		"vm":      vm,
		"folder":  "/ha-datacenter/vm",
		"name":    "cloned-vm-01",
		"spec":    map[string]interface{}{"location": map[string]interface{}{}},
		"confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_vm_clone failed: %v", err)
	}
	m := decodeResult(t, raw)
	if m["result"] != "cloned" {
		t.Fatalf("expected result=cloned, got %v (%s)", m["result"], raw)
	}

	info := vmInfo(t, r, "cloned-vm-01")
	if info["power_state"] == nil {
		t.Fatalf("expected the clone to resolve via vmware_vm_info, got %+v", info)
	}
}

// TestVMProvisioning_CloneRequiresArgs proves folder/name/spec are enforced
// client-side before any round trip, and that the gate/confirm wiring
// actually protects this real tool (not just the dummy one in
// destructive_test.go).
func TestVMProvisioning_CloneRequiresArgs(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newVMProvisioningRegistry(c, RegistryOptions{AllowDestructive: true})
	vm := firstVMPath(t, r)

	base := map[string]interface{}{
		"vm":      vm,
		"folder":  "/ha-datacenter/vm",
		"name":    "should-not-exist",
		"spec":    map[string]interface{}{"location": map[string]interface{}{}},
		"confirm": true,
	}

	for _, missing := range []string{"folder", "name", "spec"} {
		args := map[string]interface{}{}
		for k, v := range base {
			if k != missing {
				args[k] = v
			}
		}
		if _, err := r.CallTool("vmware_vm_clone", args); err == nil {
			t.Errorf("expected vmware_vm_clone to fail with %s missing", missing)
		}
	}

	closedGate := newVMProvisioningRegistry(c, RegistryOptions{AllowDestructive: false})
	if _, err := closedGate.CallTool("vmware_vm_clone", base); err == nil {
		t.Fatal("expected vmware_vm_clone to be denied with the gate closed")
	}

	noConfirm := map[string]interface{}{}
	for k, v := range base {
		if k != "confirm" {
			noConfirm[k] = v
		}
	}
	if _, err := r.CallTool("vmware_vm_clone", noConfirm); err == nil {
		t.Fatal("expected vmware_vm_clone to fail without confirm:true")
	}

	// Prove none of the rejected calls actually created a VM.
	raw, err := r.CallTool("vmware_list_vms", map[string]interface{}{})
	if err != nil {
		t.Fatalf("vmware_list_vms failed: %v", err)
	}
	if strings.Contains(raw, "should-not-exist") {
		t.Fatalf("a rejected vmware_vm_clone call still created a VM: %s", raw)
	}
}

// TestVMProvisioning_Relocate proves the plumbing end-to-end: an empty spec
// is a valid (no-op) types.VirtualMachineRelocateSpec, and
// simulator.RelocateVMTask accepts it (see virtual_machine.go — every field
// of req.Spec is nil-checked before use).
func TestVMProvisioning_Relocate(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newVMProvisioningRegistry(c, RegistryOptions{AllowDestructive: true})
	vm := firstVMPath(t, r)

	raw, err := r.CallTool("vmware_vm_relocate", map[string]interface{}{
		"vm":      vm,
		"spec":    map[string]interface{}{},
		"confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_vm_relocate failed: %v", err)
	}
	if decodeResult(t, raw)["result"] != "relocated" {
		t.Fatalf("expected result=relocated, got %s", raw)
	}

	// Gate/confirm still apply to this tool.
	if _, err := r.CallTool("vmware_vm_relocate", map[string]interface{}{"vm": vm, "spec": map[string]interface{}{}}); err == nil {
		t.Fatal("expected vmware_vm_relocate to fail without confirm:true")
	}
}

// TestVMProvisioning_Customize proves the plumbing end-to-end: the VM must
// be powered off (simulator.CustomizeVMTask rejects poweredOn — see
// virtual_machine.go), and spec.NicSettingMap's length must match
// vm.Guest.Net's length (see setPendingCustomization's NicSettingMismatch
// check) — the default simulator.ESX() model VM gets exactly 1 Guest.Net
// entry as soon as its 1 default NIC device is added (confirmed by reading
// virtual_machine.go's AddDevice path — this happens at VM-create time, not
// only after power-on/tools-running), so 1 (empty, "ip" omitted so the
// polymorphic CustomizationIPSettings.Ip interface field is never touched by
// decodeInto — see this file's top doc comment, deviation 4)
// CustomizationAdapterMapping entry is required for setPendingCustomization
// to accept the spec.
func TestVMProvisioning_Customize(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newVMProvisioningRegistry(c, RegistryOptions{AllowDestructive: true})
	vm := firstVMPath(t, r)

	if vmInfo(t, r, vm)["power_state"] == "poweredOn" {
		if _, err := r.CallTool("vmware_vm_power_off", map[string]interface{}{"vm": vm, "confirm": true}); err != nil {
			t.Fatalf("vmware_vm_power_off (pre-customize) failed: %v", err)
		}
	}

	raw, err := r.CallTool("vmware_vm_customize", map[string]interface{}{
		"vm": vm,
		"spec": map[string]interface{}{
			"nicSettingMap": []interface{}{map[string]interface{}{}},
		},
		"confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_vm_customize failed: %v", err)
	}
	if decodeResult(t, raw)["result"] != "customized" {
		t.Fatalf("expected result=customized, got %s", raw)
	}
}

// TestVMProvisioning_PromoteDisks proves the plumbing end-to-end: omitting
// "disks" promotes "every eligible disk" per the real API's documented
// behavior, which on a VM with no delta-backed disks is a well-defined,
// successful no-op (see simulator.PromoteDisksTask).
func TestVMProvisioning_PromoteDisks(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newVMProvisioningRegistry(c, RegistryOptions{AllowDestructive: true})
	vm := firstVMPath(t, r)

	raw, err := r.CallTool("vmware_vm_promote_disks", map[string]interface{}{
		"vm":      vm,
		"confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_vm_promote_disks failed: %v", err)
	}
	m := decodeResult(t, raw)
	if m["result"] != "disks_promoted" {
		t.Fatalf("expected result=disks_promoted, got %v (%s)", m["result"], raw)
	}
	if m["disk_count"] != float64(0) {
		t.Fatalf("expected disk_count=0 (omitted disks), got %v", m["disk_count"])
	}

	if _, err := r.CallTool("vmware_vm_promote_disks", map[string]interface{}{"vm": vm}); err == nil {
		t.Fatal("expected vmware_vm_promote_disks to fail without confirm:true")
	}
}

// TestVMProvisioning_ExportSnapshot proves the plumbing end-to-end:
// simulator.snapshot.go's VirtualMachineSnapshot.ExportSnapshot IS
// implemented (unlike plain VM-level ExportVm — see
// TestVMProvisioning_Export_NotSimulated) and reaches the Ready lease state
// synchronously, so Lease.Wait returns real device URLs.
func TestVMProvisioning_ExportSnapshot(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newVMProvisioningRegistry(c, RegistryOptions{AllowDestructive: true})
	vm := firstVMPath(t, r)

	if _, err := r.CallTool("vmware_vm_snapshot_create", map[string]interface{}{"vm": vm, "name": "export-me"}); err != nil {
		t.Fatalf("vmware_vm_snapshot_create failed: %v", err)
	}

	raw, err := r.CallTool("vmware_vm_snapshot_find", map[string]interface{}{"vm": vm, "name": "export-me"})
	if err != nil {
		t.Fatalf("vmware_vm_snapshot_find failed: %v", err)
	}
	found := decodeResult(t, raw)
	if found["found"] != true {
		t.Fatalf("expected the snapshot to be found: %s", raw)
	}
	snapshotRef, ok := found["snapshot"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected \"snapshot\" to be an object with a moref, got: %s", raw)
	}
	value, _ := snapshotRef["value"].(string)
	if value == "" {
		t.Fatalf("expected a non-empty moref Value in the found snapshot: %s", raw)
	}

	raw, err = r.CallTool("vmware_vm_export_snapshot", map[string]interface{}{
		"snapshot": value,
		"confirm":  true,
	})
	if err != nil {
		t.Fatalf("vmware_vm_export_snapshot failed: %v", err)
	}
	m := decodeResult(t, raw)
	if m["result"] != "export_lease_ready" {
		t.Fatalf("expected result=export_lease_ready, got %v (%s)", m["result"], raw)
	}

	// Gate/confirm still apply.
	if _, err := r.CallTool("vmware_vm_export_snapshot", map[string]interface{}{"snapshot": value}); err == nil {
		t.Fatal("expected vmware_vm_export_snapshot to fail without confirm:true")
	}
	if _, err := r.CallTool("vmware_vm_export_snapshot", map[string]interface{}{"confirm": true}); err == nil {
		t.Fatal("expected vmware_vm_export_snapshot to fail without a snapshot value")
	}
}

// TestVMProvisioning_Export_NotSimulated documents and proves a real vcsim
// gap (see this file's top doc comment, deviation 7): referencia/govmomi/
// simulator has no ExportVm handler for VirtualMachine, so this call cannot
// be functionally exercised end-to-end here. What IS proven: the tool is
// wired (reaches the real govmomi call, not a "no such tool" error), the
// gate/confirm layers still apply, and a real API-level failure surfaces as
// a normal wrapped error — not a panic.
func TestVMProvisioning_Export_NotSimulated(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newVMProvisioningRegistry(c, RegistryOptions{AllowDestructive: true})
	vm := firstVMPath(t, r)

	closedGate := newVMProvisioningRegistry(c, RegistryOptions{AllowDestructive: false})
	if _, err := closedGate.CallTool("vmware_vm_export", map[string]interface{}{"vm": vm, "confirm": true}); err == nil {
		t.Fatal("expected vmware_vm_export to be denied with the gate closed")
	}

	if _, err := r.CallTool("vmware_vm_export", map[string]interface{}{"vm": vm}); err == nil {
		t.Fatal("expected vmware_vm_export to fail without confirm:true")
	}

	_, err := r.CallTool("vmware_vm_export", map[string]interface{}{"vm": vm, "confirm": true})
	if err == nil {
		t.Fatal("expected vmware_vm_export to fail against vcsim (no ExportVm handler in the simulator) — if this now succeeds, vcsim gained support and this test (and the doc comment) should be updated")
	}
	if strings.Contains(err.Error(), "panicked") {
		t.Fatalf("expected a normal wrapped API error, got a recovered panic: %v", err)
	}
}

// TestVMProvisioning_InstantClone_NotSimulated mirrors
// TestVMProvisioning_Export_NotSimulated for InstantCloneTask — see this
// file's top doc comment, deviation 7.
func TestVMProvisioning_InstantClone_NotSimulated(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newVMProvisioningRegistry(c, RegistryOptions{AllowDestructive: true})
	vm := firstVMPath(t, r)

	if _, err := r.CallTool("vmware_vm_instant_clone", map[string]interface{}{"vm": vm, "confirm": true}); err == nil {
		t.Fatal("expected vmware_vm_instant_clone to fail without a spec")
	}

	closedGate := newVMProvisioningRegistry(c, RegistryOptions{AllowDestructive: false})
	if _, err := closedGate.CallTool("vmware_vm_instant_clone", map[string]interface{}{
		"vm":      vm,
		"spec":    map[string]interface{}{"name": "instant-01"},
		"confirm": true,
	}); err == nil {
		t.Fatal("expected vmware_vm_instant_clone to be denied with the gate closed")
	}

	_, err := r.CallTool("vmware_vm_instant_clone", map[string]interface{}{
		"vm":      vm,
		"spec":    map[string]interface{}{"name": "instant-01"},
		"confirm": true,
	})
	if err == nil {
		t.Fatal("expected vmware_vm_instant_clone to fail against vcsim (no InstantCloneTask handler in the simulator) — if this now succeeds, vcsim gained support and this test (and the doc comment) should be updated")
	}
	if strings.Contains(err.Error(), "panicked") {
		t.Fatalf("expected a normal wrapped API error, got a recovered panic: %v", err)
	}
}

// TestVMProvisioning_Migrate_NotSimulated mirrors
// TestVMProvisioning_Export_NotSimulated for MigrateVMTask — see this file's
// top doc comment, deviation 7. Also proves the "at least one of
// resource_pool or host" client-side validation, which does not depend on
// vcsim support.
func TestVMProvisioning_Migrate_NotSimulated(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newVMProvisioningRegistry(c, RegistryOptions{AllowDestructive: true})
	vm := firstVMPath(t, r)
	host := firstHostPath(t, r)

	if _, err := r.CallTool("vmware_vm_migrate", map[string]interface{}{"vm": vm, "confirm": true}); err == nil {
		t.Fatal("expected vmware_vm_migrate to fail when neither resource_pool nor host is given")
	}

	closedGate := newVMProvisioningRegistry(c, RegistryOptions{AllowDestructive: false})
	if _, err := closedGate.CallTool("vmware_vm_migrate", map[string]interface{}{"vm": vm, "host": host, "confirm": true}); err == nil {
		t.Fatal("expected vmware_vm_migrate to be denied with the gate closed")
	}

	_, err := r.CallTool("vmware_vm_migrate", map[string]interface{}{"vm": vm, "host": host, "confirm": true})
	if err == nil {
		t.Fatal("expected vmware_vm_migrate to fail against vcsim (no MigrateVMTask handler in the simulator) — if this now succeeds, vcsim gained support and this test (and the doc comment) should be updated")
	}
	if strings.Contains(err.Error(), "panicked") {
		t.Fatalf("expected a normal wrapped API error, got a recovered panic: %v", err)
	}
}

package tools

import (
	"context"
	"testing"

	"github.com/vmware/govmomi/simulator"
)

func firstVMPath(t *testing.T, r *Registry) string {
	t.Helper()
	raw, err := r.CallTool("vmware_list_vms", map[string]interface{}{})
	if err != nil {
		t.Fatalf("vmware_list_vms failed: %v", err)
	}
	list, _ := decodeResult(t, raw)["vms"].([]interface{})
	if len(list) == 0 {
		t.Fatal("simulator model has no VMs")
	}
	return list[0].(string)
}

func vmInfo(t *testing.T, r *Registry, vm string) map[string]interface{} {
	t.Helper()
	raw, err := r.CallTool("vmware_vm_info", map[string]interface{}{"vm": vm})
	if err != nil {
		t.Fatalf("vmware_vm_info(%s) failed: %v", vm, err)
	}
	return decodeResult(t, raw)
}

// TestVMTools_GateClosedDeniesDestructiveOps proves the real Tier 1/2 tools
// in this file are actually wired through registerDestructive, not just the
// dummy tool in destructive_test.go — a closed gate must deny
// vmware_vm_destroy before it touches the simulator, leaving the VM intact.
func TestVMTools_GateClosedDeniesDestructiveOps(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := NewRegistry(context.Background(), c, RegistryOptions{AllowDestructive: false})
	vm := firstVMPath(t, r)

	if _, err := r.CallTool("vmware_vm_destroy", map[string]interface{}{"vm": vm, "confirm": true}); err == nil {
		t.Fatal("expected vmware_vm_destroy to be denied with the gate closed")
	}

	// Prove the VM is genuinely untouched, not just that an error came back.
	raw, err := r.CallTool("vmware_list_vms", map[string]interface{}{})
	if err != nil {
		t.Fatalf("vmware_list_vms failed: %v", err)
	}
	if countOf(t, raw) == 0 {
		t.Fatal("VM was destroyed despite the gate being closed")
	}
}

// TestVMTools_PowerCycle proves power_on/reset/suspend/power_off transition
// a VM through the states vmware_vm_info reports, in the sequence a real
// caller would use — not just that each call returns no error.
func TestVMTools_PowerCycle(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := NewRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	vm := firstVMPath(t, r)

	// Simulator VMs may start powered off or on depending on Model.Autostart
	// — get to a known "poweredOn" state first regardless of which.
	if vmInfo(t, r, vm)["power_state"] != "poweredOn" {
		if _, err := r.CallTool("vmware_vm_power_on", map[string]interface{}{"vm": vm}); err != nil {
			t.Fatalf("vmware_vm_power_on failed: %v", err)
		}
	}
	if got := vmInfo(t, r, vm)["power_state"]; got != "poweredOn" {
		t.Fatalf("expected poweredOn after power_on, got %v", got)
	}

	if _, err := r.CallTool("vmware_vm_reset", map[string]interface{}{"vm": vm, "confirm": true}); err != nil {
		t.Fatalf("vmware_vm_reset failed: %v", err)
	}
	if got := vmInfo(t, r, vm)["power_state"]; got != "poweredOn" {
		t.Fatalf("expected poweredOn after reset, got %v", got)
	}

	if _, err := r.CallTool("vmware_vm_suspend", map[string]interface{}{"vm": vm, "confirm": true}); err != nil {
		t.Fatalf("vmware_vm_suspend failed: %v", err)
	}
	if got := vmInfo(t, r, vm)["power_state"]; got != "suspended" {
		t.Fatalf("expected suspended after suspend, got %v", got)
	}

	if _, err := r.CallTool("vmware_vm_power_on", map[string]interface{}{"vm": vm}); err != nil {
		t.Fatalf("vmware_vm_power_on (resume from suspend) failed: %v", err)
	}
	if got := vmInfo(t, r, vm)["power_state"]; got != "poweredOn" {
		t.Fatalf("expected poweredOn after resuming from suspend, got %v", got)
	}

	if _, err := r.CallTool("vmware_vm_power_off", map[string]interface{}{"vm": vm, "confirm": true}); err != nil {
		t.Fatalf("vmware_vm_power_off failed: %v", err)
	}
	if got := vmInfo(t, r, vm)["power_state"]; got != "poweredOff" {
		t.Fatalf("expected poweredOff after power_off, got %v", got)
	}
}

// TestVMTools_PowerOffRequiresConfirm proves the confirm:true requirement
// (Tier 2) actually blocks a real tool, not just the dummy one, and that a
// missing/false confirm leaves the VM's power state unchanged.
func TestVMTools_PowerOffRequiresConfirm(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := NewRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	vm := firstVMPath(t, r)

	if vmInfo(t, r, vm)["power_state"] != "poweredOn" {
		if _, err := r.CallTool("vmware_vm_power_on", map[string]interface{}{"vm": vm}); err != nil {
			t.Fatalf("vmware_vm_power_on failed: %v", err)
		}
	}

	if _, err := r.CallTool("vmware_vm_power_off", map[string]interface{}{"vm": vm}); err == nil {
		t.Fatal("expected vmware_vm_power_off to fail without confirm:true")
	}
	if got := vmInfo(t, r, vm)["power_state"]; got != "poweredOn" {
		t.Fatalf("VM power state changed despite missing confirm: got %v", got)
	}
}

// TestVMTools_Reconfigure proves num_cpus/memory_mb actually change what
// vmware_vm_info reports, and that omitting one field leaves it untouched
// (VirtualMachineConfigSpec.NumCPUs/MemoryMB are both `omitempty` — a zero
// Go value must not be sent as "set to 0").
func TestVMTools_Reconfigure(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := NewRegistry(context.Background(), c, RegistryOptions{})
	vm := firstVMPath(t, r)

	before := vmInfo(t, r, vm)
	beforeMemory := before["memory_mb"]

	if _, err := r.CallTool("vmware_vm_reconfigure", map[string]interface{}{"vm": vm, "num_cpus": float64(4)}); err != nil {
		t.Fatalf("vmware_vm_reconfigure(num_cpus) failed: %v", err)
	}
	after := vmInfo(t, r, vm)
	if after["num_cpu"] != float64(4) {
		t.Fatalf("expected num_cpu=4 after reconfigure, got %v", after["num_cpu"])
	}
	if after["memory_mb"] != beforeMemory {
		t.Fatalf("memory_mb changed (%v -> %v) despite not being in the reconfigure args — omitempty regression", beforeMemory, after["memory_mb"])
	}

	if _, err := r.CallTool("vmware_vm_reconfigure", map[string]interface{}{"vm": vm}); err == nil {
		t.Fatal("expected an error when neither num_cpus nor memory_mb is given")
	}
}

// TestVMTools_SnapshotCycle proves create -> list -> revert -> remove ->
// list against vcsim, and that revert/remove are gated Tier 1 tools.
func TestVMTools_SnapshotCycle(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := NewRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	vm := firstVMPath(t, r)

	if _, err := r.CallTool("vmware_vm_snapshot_create", map[string]interface{}{"vm": vm, "name": "snap1", "description": "first"}); err != nil {
		t.Fatalf("vmware_vm_snapshot_create failed: %v", err)
	}

	raw, err := r.CallTool("vmware_vm_snapshot_list", map[string]interface{}{"vm": vm})
	if err != nil {
		t.Fatalf("vmware_vm_snapshot_list failed: %v", err)
	}
	m := decodeResult(t, raw)
	if countOf(t, raw) != 1 {
		t.Fatalf("expected exactly 1 snapshot after create, got %v (%s)", m["count"], raw)
	}
	list, _ := m["snapshots"].([]interface{})
	first, _ := list[0].(map[string]interface{})
	if first["name"] != "snap1" {
		t.Fatalf("expected snapshot named snap1, got %v", first["name"])
	}

	// revert without confirm must be denied (Tier 1)
	if _, err := r.CallTool("vmware_vm_snapshot_revert", map[string]interface{}{"vm": vm, "snapshot_name": "snap1"}); err == nil {
		t.Fatal("expected vmware_vm_snapshot_revert to fail without confirm:true")
	}

	if _, err := r.CallTool("vmware_vm_snapshot_revert", map[string]interface{}{"vm": vm, "snapshot_name": "snap1", "confirm": true}); err != nil {
		t.Fatalf("vmware_vm_snapshot_revert failed: %v", err)
	}

	if _, err := r.CallTool("vmware_vm_snapshot_remove", map[string]interface{}{"vm": vm, "snapshot_name": "snap1", "confirm": true}); err != nil {
		t.Fatalf("vmware_vm_snapshot_remove failed: %v", err)
	}

	raw, err = r.CallTool("vmware_vm_snapshot_list", map[string]interface{}{"vm": vm})
	if err != nil {
		t.Fatalf("vmware_vm_snapshot_list (after remove) failed: %v", err)
	}
	if countOf(t, raw) != 0 {
		t.Fatalf("expected 0 snapshots after remove, got %v (%s)", decodeResult(t, raw)["count"], raw)
	}
}

// TestVMTools_Destroy proves the full destroy path: gate open + confirm
// actually removes the VM from inventory, permanently.
func TestVMTools_Destroy(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := NewRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	vm := firstVMPath(t, r)

	// vSphere refuses Destroy_Task on a powered-on VM (InvalidPowerState) —
	// confirmed by this test failing against the real fault before this
	// power-off was added. vmware_vm_destroy deliberately does NOT
	// auto-power-off as a side effect (a Tier 1 tool silently doing extra
	// destructive work would be worse, not safer).
	if vmInfo(t, r, vm)["power_state"] == "poweredOn" {
		if _, err := r.CallTool("vmware_vm_power_off", map[string]interface{}{"vm": vm, "confirm": true}); err != nil {
			t.Fatalf("vmware_vm_power_off (pre-destroy) failed: %v", err)
		}
	}

	if _, err := r.CallTool("vmware_vm_destroy", map[string]interface{}{"vm": vm, "confirm": true}); err != nil {
		t.Fatalf("vmware_vm_destroy failed: %v", err)
	}

	if _, err := r.CallTool("vmware_vm_info", map[string]interface{}{"vm": vm}); err == nil {
		t.Fatalf("expected %s to no longer resolve after destroy", vm)
	}
}

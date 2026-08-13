package tools

import (
	"context"
	"testing"

	"github.com/vmware/govmomi/simulator"
)

func TestVMSnapshotTools_FindCreateExRevertCurrentRemoveAll(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := NewRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	vm := firstVMPath(t, r)

	// Before any snapshot exists, find must report not-found rather than error.
	raw, err := r.CallTool("vmware_vm_snapshot_find", map[string]interface{}{"vm": vm, "name": "pilot-snap"})
	if err != nil {
		t.Fatalf("vmware_vm_snapshot_find (pre) failed: %v", err)
	}
	if decodeResult(t, raw)["found"] != false {
		t.Fatalf("expected found=false before any snapshot exists, got %v", decodeResult(t, raw))
	}

	if _, err := r.CallTool("vmware_vm_snapshot_create_ex", map[string]interface{}{
		"vm":   vm,
		"name": "pilot-snap",
	}); err != nil {
		t.Fatalf("vmware_vm_snapshot_create_ex failed: %v", err)
	}

	raw, err = r.CallTool("vmware_vm_snapshot_find", map[string]interface{}{"vm": vm, "name": "pilot-snap"})
	if err != nil {
		t.Fatalf("vmware_vm_snapshot_find (post) failed: %v", err)
	}
	if decodeResult(t, raw)["found"] != true {
		t.Fatalf("expected found=true after creating pilot-snap, got %v", decodeResult(t, raw))
	}

	if _, err := r.CallTool("vmware_vm_snapshot_revert_current", map[string]interface{}{"vm": vm, "confirm": true}); err != nil {
		t.Fatalf("vmware_vm_snapshot_revert_current failed: %v", err)
	}

	if _, err := r.CallTool("vmware_vm_snapshot_remove_all", map[string]interface{}{"vm": vm, "confirm": true}); err != nil {
		t.Fatalf("vmware_vm_snapshot_remove_all failed: %v", err)
	}

	raw, err = r.CallTool("vmware_vm_snapshot_find", map[string]interface{}{"vm": vm, "name": "pilot-snap"})
	if err != nil {
		t.Fatalf("vmware_vm_snapshot_find (after remove_all) failed: %v", err)
	}
	if decodeResult(t, raw)["found"] != false {
		t.Fatalf("expected found=false after snapshot_remove_all, got %v", decodeResult(t, raw))
	}
}

func TestVMSnapshotTools_DestructiveOpsRequireGateAndConfirm(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	vm := firstVMPath(t, NewRegistry(context.Background(), c, RegistryOptions{}))

	closedGate := NewRegistry(context.Background(), c, RegistryOptions{AllowDestructive: false})
	if _, err := closedGate.CallTool("vmware_vm_snapshot_revert_current", map[string]interface{}{"vm": vm, "confirm": true}); err == nil {
		t.Fatal("expected vmware_vm_snapshot_revert_current to be denied with the gate closed")
	}
	if _, err := closedGate.CallTool("vmware_vm_snapshot_remove_all", map[string]interface{}{"vm": vm, "confirm": true}); err == nil {
		t.Fatal("expected vmware_vm_snapshot_remove_all to be denied with the gate closed")
	}

	openGate := NewRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	if _, err := openGate.CallTool("vmware_vm_snapshot_revert_current", map[string]interface{}{"vm": vm}); err == nil {
		t.Fatal("expected vmware_vm_snapshot_revert_current to fail without confirm:true")
	}
	if _, err := openGate.CallTool("vmware_vm_snapshot_remove_all", map[string]interface{}{"vm": vm}); err == nil {
		t.Fatal("expected vmware_vm_snapshot_remove_all to fail without confirm:true")
	}
}

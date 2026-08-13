package tools

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// registerVMTools registers VM lifecycle (power, reconfigure, destroy) and
// snapshot tools. Every Tier 1/2 tool (see destructive.go) is registered
// via r.registerDestructive, not r.register directly — vmware_vm_power_on,
// vmware_vm_reconfigure, vmware_vm_snapshot_create/list, and vmware_vm_info
// are not tiered: powering on, growing CPU/RAM, creating a snapshot, and
// reading state are not destructive by this project's definition (see the
// plan's Fase 1a severity table).
func registerVMTools(r *Registry) {
	vmArg := map[string]interface{}{
		"type":        "string",
		"description": `VM identifier: a name/pattern (e.g. "cac-WN02") or a full inventory path (e.g. "/ha-datacenter/vm/cac-WN02") as returned by vmware_list_vms. Must resolve to exactly one VM.`,
	}
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}

	r.register("vmware_vm_power_on",
		"Power on a virtual machine.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"vm": vmArg},
			"required":   []interface{}{"vm"},
		},
		Tool{Handler: handleVMPowerOn},
	)

	r.registerDestructive("vmware_vm_power_off",
		"Power off (hard) a virtual machine. Ungraceful — any unsaved guest OS state is lost, same as pulling the plug.",
		tier2,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"vm": vmArg, "confirm": confirmArg},
			"required":   []interface{}{"vm", "confirm"},
		},
		Tool{Handler: handleVMPowerOff},
	)

	r.registerDestructive("vmware_vm_reset",
		"Hard-reset a virtual machine (equivalent to pressing the physical reset button). Ungraceful — any unsaved guest OS state is lost.",
		tier2,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"vm": vmArg, "confirm": confirmArg},
			"required":   []interface{}{"vm", "confirm"},
		},
		Tool{Handler: handleVMReset},
	)

	r.registerDestructive("vmware_vm_suspend",
		"Suspend a virtual machine (saves state to disk and stops it — reversible via power-on, but the VM is briefly unavailable during the transition).",
		tier2,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"vm": vmArg, "confirm": confirmArg},
			"required":   []interface{}{"vm", "confirm"},
		},
		Tool{Handler: handleVMSuspend},
	)

	r.register("vmware_vm_reconfigure",
		"Change a virtual machine's CPU count and/or memory size. Requires the VM to support hot-add for the change to apply while powered on, otherwise it takes effect on next boot; this tool only submits the reconfiguration, it does not power-cycle the VM.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm":        vmArg,
				"num_cpus":  map[string]interface{}{"type": "integer", "description": "New vCPU count. Omit to leave unchanged."},
				"memory_mb": map[string]interface{}{"type": "integer", "description": "New memory size in MB. Omit to leave unchanged."},
			},
			"required": []interface{}{"vm"},
		},
		Tool{Handler: handleVMReconfigure},
	)

	r.registerDestructive("vmware_vm_destroy",
		"Permanently delete a virtual machine and its virtual disks. Irreversible. The VM must already be powered off (vmware_vm_power_off) — vSphere rejects destroying a powered-on VM, and this tool does not power it off for you.",
		tier1,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"vm": vmArg, "confirm": confirmArg},
			"required":   []interface{}{"vm", "confirm"},
		},
		Tool{Handler: handleVMDestroy},
	)

	snapshotNameArg := map[string]interface{}{
		"type":        "string",
		"description": "Snapshot name, matched via VirtualMachine.FindSnapshot (exact name, or a unique tree/path match).",
	}

	r.register("vmware_vm_snapshot_create",
		"Create a new snapshot of a virtual machine's current state.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm":          vmArg,
				"name":        map[string]interface{}{"type": "string", "description": "Snapshot name."},
				"description": map[string]interface{}{"type": "string", "description": "Optional snapshot description."},
				"memory":      map[string]interface{}{"type": "boolean", "description": "Include the VM's memory state in the snapshot (allows reverting to a running state). Default false."},
				"quiesce":     map[string]interface{}{"type": "boolean", "description": "Quiesce the guest file system before snapshotting (requires VMware Tools). Default false."},
			},
			"required": []interface{}{"vm", "name"},
		},
		Tool{Handler: handleSnapshotCreate},
	)

	r.registerDestructive("vmware_vm_snapshot_revert",
		"Revert a virtual machine to a named snapshot, discarding any state changed since that snapshot was taken.",
		tier1,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm":                vmArg,
				"snapshot_name":     snapshotNameArg,
				"suppress_power_on": map[string]interface{}{"type": "boolean", "description": "Do not power on the VM after reverting even if the snapshot was taken while powered on. Default false."},
				"confirm":           confirmArg,
			},
			"required": []interface{}{"vm", "snapshot_name", "confirm"},
		},
		Tool{Handler: handleSnapshotRevert},
	)

	r.registerDestructive("vmware_vm_snapshot_remove",
		"Delete a named snapshot, merging its delta disks into its parent. Irreversible.",
		tier1,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm":              vmArg,
				"snapshot_name":   snapshotNameArg,
				"remove_children": map[string]interface{}{"type": "boolean", "description": "Also remove every descendant snapshot in the tree. Default false."},
				"confirm":         confirmArg,
			},
			"required": []interface{}{"vm", "snapshot_name", "confirm"},
		},
		Tool{Handler: handleSnapshotRemove},
	)

	r.register("vmware_vm_snapshot_list",
		"List the snapshot tree of a virtual machine.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"vm": vmArg},
			"required":   []interface{}{"vm"},
		},
		Tool{Handler: handleSnapshotList},
	)

	r.register("vmware_vm_info",
		"Get power state and summary configuration (CPU, memory, guest OS, UUID, connection state) for a virtual machine.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"vm": vmArg},
			"required":   []interface{}{"vm"},
		},
		Tool{Handler: handleVMInfo},
	)
}

// resolveVM resolves the required "vm" argument to exactly one VM, scoping
// a non-absolute name/pattern with dcScopedPath so this also works against
// a vCenter with more than one datacenter — see finder.go.
func resolveVM(ctx context.Context, client *vmware.Client, args map[string]interface{}) (*object.VirtualMachine, error) {
	name, _ := args["vm"].(string)
	if name == "" {
		return nil, fmt.Errorf("vm is required")
	}

	vm, err := client.Finder.VirtualMachine(ctx, dcScopedPath("vm", name))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve vm %q: %w", name, err)
	}
	return vm, nil
}

// waitForTask blocks until t completes, returning its error if it failed —
// synchronous from the calling MCP tool's point of view, matching every
// other tool in this project (no async/polling tools exist yet).
func waitForTask(ctx context.Context, t *object.Task) error {
	_, err := t.WaitForResult(ctx)
	return err
}

// toInt32/toInt64 accept the numeric types a tool argument can arrive as:
// float64 from real JSON-RPC (encoding/json decodes all JSON numbers into
// interface{} as float64), or a Go int/int32/int64 literal from tests that
// build args by hand.
func toInt32(v interface{}) (int32, error) {
	switch n := v.(type) {
	case float64:
		return int32(n), nil
	case int:
		return int32(n), nil
	case int32:
		return n, nil
	case int64:
		return int32(n), nil
	default:
		return 0, fmt.Errorf("expected a number, got %T", v)
	}
}

func toInt64(v interface{}) (int64, error) {
	switch n := v.(type) {
	case float64:
		return int64(n), nil
	case int:
		return int64(n), nil
	case int32:
		return int64(n), nil
	case int64:
		return n, nil
	default:
		return 0, fmt.Errorf("expected a number, got %T", v)
	}
}

func handleVMPowerOn(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, err := resolveVM(ctx, client, args)
	if err != nil {
		return "", err
	}

	task, err := vm.PowerOn(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to power on %s: %w", vm.InventoryPath, err)
	}
	if err := waitForTask(ctx, task); err != nil {
		return "", fmt.Errorf("power-on task failed for %s: %w", vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"vm": vm.InventoryPath, "result": "powered_on"})
}

func handleVMPowerOff(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, err := resolveVM(ctx, client, args)
	if err != nil {
		return "", err
	}

	task, err := vm.PowerOff(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to power off %s: %w", vm.InventoryPath, err)
	}
	if err := waitForTask(ctx, task); err != nil {
		return "", fmt.Errorf("power-off task failed for %s: %w", vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"vm": vm.InventoryPath, "result": "powered_off"})
}

func handleVMReset(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, err := resolveVM(ctx, client, args)
	if err != nil {
		return "", err
	}

	task, err := vm.Reset(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to reset %s: %w", vm.InventoryPath, err)
	}
	if err := waitForTask(ctx, task); err != nil {
		return "", fmt.Errorf("reset task failed for %s: %w", vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"vm": vm.InventoryPath, "result": "reset"})
}

func handleVMSuspend(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, err := resolveVM(ctx, client, args)
	if err != nil {
		return "", err
	}

	task, err := vm.Suspend(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to suspend %s: %w", vm.InventoryPath, err)
	}
	if err := waitForTask(ctx, task); err != nil {
		return "", fmt.Errorf("suspend task failed for %s: %w", vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"vm": vm.InventoryPath, "result": "suspended"})
}

func handleVMReconfigure(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, err := resolveVM(ctx, client, args)
	if err != nil {
		return "", err
	}

	spec := types.VirtualMachineConfigSpec{}
	changed := false

	if v, ok := args["num_cpus"]; ok {
		n, err := toInt32(v)
		if err != nil {
			return "", fmt.Errorf("invalid num_cpus: %w", err)
		}
		spec.NumCPUs = n
		changed = true
	}
	if v, ok := args["memory_mb"]; ok {
		n, err := toInt64(v)
		if err != nil {
			return "", fmt.Errorf("invalid memory_mb: %w", err)
		}
		spec.MemoryMB = n
		changed = true
	}
	if !changed {
		return "", fmt.Errorf("at least one of num_cpus or memory_mb is required")
	}

	task, err := vm.Reconfigure(ctx, spec)
	if err != nil {
		return "", fmt.Errorf("failed to reconfigure %s: %w", vm.InventoryPath, err)
	}
	if err := waitForTask(ctx, task); err != nil {
		return "", fmt.Errorf("reconfigure task failed for %s: %w", vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"vm": vm.InventoryPath, "result": "reconfigured", "num_cpus": spec.NumCPUs, "memory_mb": spec.MemoryMB})
}

func handleVMDestroy(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, err := resolveVM(ctx, client, args)
	if err != nil {
		return "", err
	}
	path := vm.InventoryPath

	task, err := vm.Destroy(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to destroy %s: %w", path, err)
	}
	if err := waitForTask(ctx, task); err != nil {
		return "", fmt.Errorf("destroy task failed for %s: %w", path, err)
	}

	return marshalJSON(map[string]interface{}{"vm": path, "result": "destroyed"})
}

func handleSnapshotCreate(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, err := resolveVM(ctx, client, args)
	if err != nil {
		return "", err
	}
	name, _ := args["name"].(string)
	if name == "" {
		return "", fmt.Errorf("name is required")
	}
	description, _ := args["description"].(string)
	memory, _ := args["memory"].(bool)
	quiesce, _ := args["quiesce"].(bool)

	task, err := vm.CreateSnapshot(ctx, name, description, memory, quiesce)
	if err != nil {
		return "", fmt.Errorf("failed to create snapshot %q on %s: %w", name, vm.InventoryPath, err)
	}
	if err := waitForTask(ctx, task); err != nil {
		return "", fmt.Errorf("snapshot-create task failed for %s: %w", vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"vm": vm.InventoryPath, "snapshot": name, "result": "created"})
}

func handleSnapshotRevert(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, err := resolveVM(ctx, client, args)
	if err != nil {
		return "", err
	}
	name, _ := args["snapshot_name"].(string)
	if name == "" {
		return "", fmt.Errorf("snapshot_name is required")
	}
	suppressPowerOn, _ := args["suppress_power_on"].(bool)

	task, err := vm.RevertToSnapshot(ctx, name, suppressPowerOn)
	if err != nil {
		return "", fmt.Errorf("failed to revert %s to snapshot %q: %w", vm.InventoryPath, name, err)
	}
	if err := waitForTask(ctx, task); err != nil {
		return "", fmt.Errorf("snapshot-revert task failed for %s: %w", vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"vm": vm.InventoryPath, "snapshot": name, "result": "reverted"})
}

func handleSnapshotRemove(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, err := resolveVM(ctx, client, args)
	if err != nil {
		return "", err
	}
	name, _ := args["snapshot_name"].(string)
	if name == "" {
		return "", fmt.Errorf("snapshot_name is required")
	}
	removeChildren, _ := args["remove_children"].(bool)

	task, err := vm.RemoveSnapshot(ctx, name, removeChildren, nil)
	if err != nil {
		return "", fmt.Errorf("failed to remove snapshot %q on %s: %w", name, vm.InventoryPath, err)
	}
	if err := waitForTask(ctx, task); err != nil {
		return "", fmt.Errorf("snapshot-remove task failed for %s: %w", vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"vm": vm.InventoryPath, "snapshot": name, "result": "removed"})
}

func handleSnapshotList(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, err := resolveVM(ctx, client, args)
	if err != nil {
		return "", err
	}

	var moVM mo.VirtualMachine
	if err := vm.Properties(ctx, vm.Reference(), []string{"snapshot"}, &moVM); err != nil {
		return "", fmt.Errorf("failed to read snapshot info for %s: %w", vm.InventoryPath, err)
	}

	var snapshots []map[string]interface{}
	if moVM.Snapshot != nil {
		snapshots = flattenSnapshots(moVM.Snapshot.RootSnapshotList)
	}

	return marshalJSON(map[string]interface{}{
		"vm":        vm.InventoryPath,
		"count":     len(snapshots),
		"snapshots": snapshots,
	})
}

// flattenSnapshots walks the snapshot tree recursively — VMware snapshots
// form a tree (a snapshot can have multiple children), not a flat list.
func flattenSnapshots(nodes []types.VirtualMachineSnapshotTree) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, map[string]interface{}{
			"name":        n.Name,
			"description": n.Description,
			"id":          n.Id,
			"create_time": n.CreateTime,
			"state":       string(n.State),
			"quiesced":    n.Quiesced,
			"children":    flattenSnapshots(n.ChildSnapshotList),
		})
	}
	return out
}

func handleVMInfo(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, err := resolveVM(ctx, client, args)
	if err != nil {
		return "", err
	}

	var moVM mo.VirtualMachine
	if err := vm.Properties(ctx, vm.Reference(), []string{"summary"}, &moVM); err != nil {
		return "", fmt.Errorf("failed to read info for %s: %w", vm.InventoryPath, err)
	}
	s := moVM.Summary

	return marshalJSON(map[string]interface{}{
		"vm":               vm.InventoryPath,
		"power_state":      string(s.Runtime.PowerState),
		"connection_state": string(s.Runtime.ConnectionState),
		"guest_full_name":  s.Config.GuestFullName,
		"num_cpu":          s.Config.NumCpu,
		"memory_mb":        s.Config.MemorySizeMB,
		"uuid":             s.Config.Uuid,
	})
}

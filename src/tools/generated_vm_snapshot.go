package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/vmware/govmomi/vim25/types"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// registerVMSnapshotTools is part of Fase 2 of the codegen plan
// (".workspace/plans/MCPVMWare2026-08-10-175300-plano-cobertura-completa-api-codegen.plan.md")
// — VirtualMachine's remaining snapshot-related methods, hand-transcribed
// from src/gen/classification.json following the vm.go/generated_option.go
// conventions.
//
// Curation deviations from the raw classification (human review required
// for this domain per the plan — VM has the largest destructive surface):
//   - CreateSnapshot and RevertToSnapshot are EXCLUDED: vm.go's existing
//     vmware_vm_snapshot_create/vmware_vm_snapshot_revert already wrap the
//     exact same govmomi calls with the exact same parameters — the Fase 0
//     generator's exact-name dedup didn't catch these because the proposed
//     names use a different word order (vmware_vm_create_snapshot vs the
//     existing vmware_vm_snapshot_create), not because the functionality
//     differs.
//   - RemoveSnapshot is EXCLUDED for the same reason: vmware_vm_snapshot_remove
//     already wraps object.VirtualMachine.RemoveSnapshot(ctx, name,
//     removeChildren, nil) — the only difference is the generated candidate
//     would additionally expose the consolidate *bool parameter the
//     hand-written tool hardcodes to nil. Extending the existing tool with
//     that parameter (if ever needed) is a smaller, clearer change than
//     shipping a second near-identical tool.
//   - The 4 tools kept here are renamed from the generator's raw
//     vmware_vm_<verb>_snapshot / vmware_vm_snapshot_<verb> mix to a
//     consistent vmware_vm_snapshot_<verb> — matching the naming pattern of
//     vm.go's 4 existing snapshot tools exactly, so an LLM caller sees one
//     coherent vmware_vm_snapshot_* family instead of two different word
//     orders for the same domain.
func registerVMSnapshotTools(r *Registry) {
	vmArg := map[string]interface{}{
		"type":        "string",
		"description": `VM identifier: a name/pattern (e.g. "cac-WN02") or a full inventory path (e.g. "/ha-datacenter/vm/cac-WN02") as returned by vmware_list_vms. Must resolve to exactly one VM.`,
	}
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}

	r.register("vmware_vm_snapshot_find",
		"Find a snapshot by name (exact name, or a unique tree/path match) and return its managed object reference. Useful to confirm a name resolves before calling a destructive snapshot tool.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm":   vmArg,
				"name": map[string]interface{}{"type": "string", "description": "Snapshot name to look up."},
			},
			"required": []interface{}{"vm", "name"},
		},
		Tool{Handler: handleVMSnapshotFind},
	)

	r.register("vmware_vm_snapshot_create_ex",
		"Create a new snapshot of a virtual machine's current state, with a full guest-quiesce spec instead of just a boolean — use vmware_vm_snapshot_create for the common case; this exists for guest-specific quiesce options (e.g. VSS types on Windows).",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm":           vmArg,
				"name":         map[string]interface{}{"type": "string", "description": "Snapshot name."},
				"description":  map[string]interface{}{"type": "string", "description": "Optional snapshot description."},
				"memory":       map[string]interface{}{"type": "boolean", "description": "Include the VM's memory state in the snapshot. Default false."},
				"quiesce_spec": map[string]interface{}{"type": "object", "description": "A types.BaseVirtualMachineGuestQuiesceSpec (e.g. VirtualMachineWindowsQuiesceSpec) as a JSON object matching its Go struct fields. Omit for no quiesce spec (same as memory-only snapshot)."},
			},
			"required": []interface{}{"vm", "name"},
		},
		Tool{Handler: handleVMSnapshotCreateEx},
	)

	r.registerDestructive("vmware_vm_snapshot_revert_current",
		"Revert a virtual machine to its \"current\" snapshot (the one the snapshot tree pointer is on), without needing to name it. Discards any state changed since that snapshot was taken.",
		tier1,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm":                vmArg,
				"suppress_power_on": map[string]interface{}{"type": "boolean", "description": "Do not power on the VM after reverting even if the snapshot was taken while powered on. Default false."},
				"confirm":           confirmArg,
			},
			"required": []interface{}{"vm", "confirm"},
		},
		Tool{Handler: handleVMSnapshotRevertCurrent},
	)

	r.registerDestructive("vmware_vm_snapshot_remove_all",
		"Delete every snapshot of a virtual machine, consolidating all delta disks into the base disk. Irreversible — unlike vmware_vm_snapshot_remove, there is no snapshot_name to target: this removes the entire tree in one call.",
		tier1,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm":          vmArg,
				"consolidate": map[string]interface{}{"type": "boolean", "description": "Consolidate disks after removal. Omit to use vSphere's default (true)."},
				"confirm":     confirmArg,
			},
			"required": []interface{}{"vm", "confirm"},
		},
		Tool{Handler: handleVMSnapshotRemoveAll},
	)
}

// decodeQuiesceSpec accepts the JSON object under "quiesce_spec" and decodes
// it into types.VirtualMachineGuestQuiesceSpec — the base/common
// implementer of the polymorphic types.BaseVirtualMachineGuestQuiesceSpec
// interface (only field: an optional Timeout in minutes). Guest-specific
// variants (e.g. VirtualMachineWindowsQuiesceSpec's VSS options) aren't
// supported by this MVP scope — a caller needing those should omit
// quiesce_spec and use memory:true instead, or this tool needs a real
// "type" discriminator added later, matching the plan's Fase 0 schema
// strategy item 2(b): generic JSON in, no recursive typed schema.
func decodeQuiesceSpec(raw interface{}) (types.BaseVirtualMachineGuestQuiesceSpec, error) {
	if raw == nil {
		return nil, nil
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var spec types.VirtualMachineGuestQuiesceSpec
	if err := json.Unmarshal(b, &spec); err != nil {
		return nil, err
	}
	return &spec, nil
}

func handleVMSnapshotFind(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, err := resolveVM(ctx, client, args)
	if err != nil {
		return "", err
	}
	name, _ := args["name"].(string)
	if name == "" {
		return "", fmt.Errorf("name is required")
	}

	ref, err := vm.FindSnapshot(ctx, name)
	if err != nil {
		// object.VirtualMachine.FindSnapshot (confirmed by reading the
		// source, not assumed) returns a plain, non-sentinel error for
		// every "no match" case — no snapshots at all, name not found, or
		// an ambiguous match — as well as for a genuine property-read
		// failure; there is nothing to type-switch on to tell them apart.
		// Treat it as "not found" with the message as a hint, matching
		// this project's list-tool convention (0 results is success, not
		// a tool error — see finder.go's emptyOnNotFound / bug-002 /
		// generated_option.go's isInvalidNameFault) instead of forcing
		// every caller to string-match the error to tell "not found" from
		// "something broke."
		return marshalJSON(map[string]interface{}{
			"vm":     vm.InventoryPath,
			"name":   name,
			"found":  false,
			"detail": err.Error(),
		})
	}

	return marshalJSON(map[string]interface{}{
		"vm":       vm.InventoryPath,
		"name":     name,
		"found":    true,
		"snapshot": ref,
	})
}

func handleVMSnapshotCreateEx(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
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

	quiesceSpec, err := decodeQuiesceSpec(args["quiesce_spec"])
	if err != nil {
		return "", fmt.Errorf("invalid quiesce_spec: %w", err)
	}

	task, err := vm.CreateSnapshotEx(ctx, name, description, memory, quiesceSpec)
	if err != nil {
		return "", fmt.Errorf("failed to create snapshot %q on %s: %w", name, vm.InventoryPath, err)
	}
	if err := waitForTask(ctx, task); err != nil {
		return "", fmt.Errorf("snapshot-create-ex task failed for %s: %w", vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"vm": vm.InventoryPath, "snapshot": name, "result": "created"})
}

func handleVMSnapshotRevertCurrent(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, err := resolveVM(ctx, client, args)
	if err != nil {
		return "", err
	}
	suppressPowerOn, _ := args["suppress_power_on"].(bool)

	task, err := vm.RevertToCurrentSnapshot(ctx, suppressPowerOn)
	if err != nil {
		return "", fmt.Errorf("failed to revert %s to its current snapshot: %w", vm.InventoryPath, err)
	}
	if err := waitForTask(ctx, task); err != nil {
		return "", fmt.Errorf("snapshot-revert-current task failed for %s: %w", vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"vm": vm.InventoryPath, "result": "reverted"})
}

func handleVMSnapshotRemoveAll(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, err := resolveVM(ctx, client, args)
	if err != nil {
		return "", err
	}

	var consolidate *bool
	if v, ok := args["consolidate"].(bool); ok {
		consolidate = &v
	}

	task, err := vm.RemoveAllSnapshot(ctx, consolidate)
	if err != nil {
		return "", fmt.Errorf("failed to remove all snapshots on %s: %w", vm.InventoryPath, err)
	}
	if err := waitForTask(ctx, task); err != nil {
		return "", fmt.Errorf("snapshot-remove-all task failed for %s: %w", vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"vm": vm.InventoryPath, "result": "all_snapshots_removed"})
}

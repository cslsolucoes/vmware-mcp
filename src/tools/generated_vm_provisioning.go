package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/vmware/govmomi/nfc"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// registerVMProvisioningTools and registerVMProvisioningVCenterOnlyTools are
// part of Fase 2 of the codegen plan
// (".workspace/plans/MCPVMWare2026-08-10-175300-plano-cobertura-completa-api-codegen.plan.md")
// — the "provisioning" slice: VirtualMachine's clone/migrate/relocate/
// customize/export methods plus 2 pre-flight-check helper types
// (object.VmProvisioningChecker, object.VmCompatibilityChecker),
// hand-transcribed from src/gen/classification.json following the
// vm.go/host.go/generated_option.go/generated_vm_snapshot.go conventions.
//
// The brief for this slice asked for "12 methods" but enumerated only 11
// distinct tool names (Clone, InstantClone, Relocate, Migrate, Customize,
// Export, ExportSnapshot, PromoteDisks, CheckRelocate, CheckVmConfig,
// CheckCompatibility) — flagged here rather than inventing an unspecified
// 12th tool. All 11 are implemented below.
//
// Curation deviations from the raw classification/brief (human review
// required for this domain — provisioning has the largest destructive
// surface after VM lifecycle itself):
//
//  1. Split into two register functions, two toolMode classes. CheckRelocate/
//     CheckVmConfig/CheckCompatibility are built from object.VmProvisioningChecker/
//     object.VmCompatibilityChecker, whose constructors
//     (object.NewVmProvisioningChecker/NewVmCompatibilityChecker) dereference
//     client.ServiceContent.VmProvisioningChecker/VmCompatibilityChecker — both
//     *types.ManagedObjectReference fields that are nil on a standalone ESXi
//     host (confirmed in referencia/govmomi/simulator/esx/service_content.go)
//     but non-nil on vCenter (referencia/govmomi/simulator/vpx/service_content.go).
//     Calling either constructor against standalone ESXi PANICS. registry.go's
//     CallTool already carries a recover() wrapper as defense-in-depth (added
//     during this same Fase 2 pass), but the 3 checker handlers here ALSO
//     guard explicitly before constructing the checker, returning a clean,
//     informative error instead of relying on the panic recovery — see
//     requireProvisioningChecker/requireCompatibilityChecker below. Because of
//     that risk, all 3 checker tools are registered under modeVCenterOnly in
//     registerVMProvisioningVCenterOnlyTools (a SEPARATE function), not
//     modeVSphereGeneral — a correction to the Fase 0 AST classifier's raw
//     guess of vsphere-general (the source file names don't obviously read as
//     vCenter-only). The other 8 tools stay in registerVMProvisioningTools
//     under modeVSphereGeneral. Whoever wires this into registry.go calls
//     both: r.withClass(modeVSphereGeneral, registerVMProvisioningTools) and
//     r.withClass(modeVCenterOnly, registerVMProvisioningVCenterOnlyTools).
//
//  2. CheckRelocate/CheckVmConfig/CheckCompatibility are dry-run checks — none
//     of the 3 mutate VM/host/pool state, they only report feasibility/
//     warnings (confirmed by reading object/vm_provisioning_checker.go and
//     object/vm_compatability_checker.go — note the misspelling in that real
//     file name — and their simulator counterparts). Registered via
//     r.register, NOT r.registerDestructive, correcting the raw
//     classification's tier2 guess (this is effectively a read-only endpoint,
//     like vmware_vm_snapshot_find). The other 8 tools in this file ARE
//     tier2/registerDestructive, per the brief.
//
//  3. CheckVmConfig/CheckCompatibility's vm/host/pool arguments are resolved
//     by NAME (via resolveVM/resolveHost/resolveResourcePool — the same
//     dcScopedPath-based convention every other tool in this project uses),
//     not accepted as a raw moref "type:value" string as one reading of the
//     brief suggested. Deviation reasoning: no other tool in this project
//     hands a caller a raw moref to feed back in (vmware_list_hosts/
//     vmware_list_resource_pools return InventoryPath strings, not morefs),
//     so requiring a raw moref here would be a UX dead end for a caller that
//     doesn't already have vSphere API/PowerCLI access on the side. Only
//     vmware_vm_export_snapshot's "snapshot" argument stays a raw moref VALUE
//     (Type is fixed to "VirtualMachineSnapshot") — because
//     vmware_vm_snapshot_find already hands back that exact
//     {type, value} shape as its own tool output, so a caller has a real,
//     in-project path to obtain it.
//
//  4. Complex/polymorphic specs (VirtualMachineCloneSpec,
//     VirtualMachineInstantCloneSpec, VirtualMachineRelocateSpec,
//     CustomizationSpec, VirtualDisk[]) are accepted as a generic JSON object
//     and decoded via decodeInto (json.Marshal the already-decoded
//     interface{} tree back to bytes, then json.Unmarshal into the real
//     govmomi struct) — the same "accept the concrete struct via generic
//     JSON, not a hand-built recursive schema" approach as
//     generated_option.go's Update/generated_vm_snapshot.go's
//     decodeQuiesceSpec. CustomizationSpec's Identity field is itself
//     polymorphic (types.BaseCustomizationIdentitySettings) — this MVP only
//     makes the generic decode work; a caller needs to know the underlying Go
//     struct field names (e.g. "identity":{"hostName":{...}}) to exercise a
//     concrete identity variant, there is no schema guidance for which
//     variant to use.
//
//  5. Export/ExportSnapshot start an OVF export lease, Wait() for it to reach
//     the Ready state, and return the list of downloadable device URLs (each
//     a full URL, host included) — see leaseResult. Neither tool downloads
//     the files itself (multi-file OVF download/orchestration is out of
//     scope for this MVP); a caller fetches the returned URLs on their own.
//
//  6. PromoteDisks: omitting/emptying "disks" is the primary supported path —
//     per the real API's documented behavior (see
//     referencia/govmomi/vim25/types/types.go's PromoteDisksRequestType doc
//     comment: "If this value is unset or the array is empty, all disks which
//     have delta disk backings are promoted"), NOT a client-side default this
//     project invented. Specifying individual disks is also supported (each
//     array element generic-JSON-decoded into types.VirtualDisk) but requires
//     knowing the full VirtualDisk struct shape (including its Key, which
//     must match an existing device) — documented as the advanced path.
//
//  7. vcsim simulation gaps found while testing (not assumed — confirmed by
//     grepping referencia/govmomi/simulator for a handler): InstantCloneTask,
//     MigrateVMTask, and ExportVm have NO simulator implementation (only
//     CloneVMTask, RelocateVMTask, CustomizeVMTask, PromoteDisksTask, the
//     VirtualMachineSnapshot-scoped ExportSnapshot, and the 2 checker types
//     are simulated). vmware_vm_instant_clone/vmware_vm_migrate/
//     vmware_vm_export are therefore NOT functionally exercised end-to-end in
//     this file's tests against vcsim — those 3 tests instead prove
//     registration, tier2 gate/confirm enforcement, and (where applicable)
//     argument validation, and assert the call fails cleanly (a normal
//     wrapped error) rather than silently skipping. The remaining 8 tools
//     (Clone, Relocate, Customize, PromoteDisks, ExportSnapshot,
//     CheckRelocate, CheckVmConfig, CheckCompatibility) ARE functionally
//     simulated and tested end-to-end.
func registerVMProvisioningTools(r *Registry) {
	vmArg := map[string]interface{}{
		"type":        "string",
		"description": `VM identifier: a name/pattern (e.g. "cac-WN02") or a full inventory path (e.g. "/ha-datacenter/vm/cac-WN02") as returned by vmware_list_vms. Must resolve to exactly one VM.`,
	}
	hostArg := map[string]interface{}{
		"type":        "string",
		"description": `Host identifier: a name/pattern (e.g. "esxi-01.local") or a full inventory path as returned by vmware_list_hosts. Must resolve to exactly one host.`,
	}
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}
	priorityArg := map[string]interface{}{
		"type":        "string",
		"enum":        []interface{}{"lowPriority", "highPriority", "defaultPriority"},
		"description": `Task scheduling priority (types.VirtualMachineMovePriority). Default "defaultPriority" if omitted.`,
	}

	r.registerDestructive("vmware_vm_clone",
		"Clone a virtual machine into a new VM. Starts a long-running task; this call blocks until it completes (or fails).",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm": vmArg,
				"folder": map[string]interface{}{
					"type":        "string",
					"description": `Destination inventory folder for the clone: a full inventory path (e.g. "/ha-datacenter/vm") or a subfolder name/pattern under the datacenter's "vm" folder, resolved the same way vmArg/hostArg are (see dcScopedPath in finder.go).`,
				},
				"name": map[string]interface{}{"type": "string", "description": "Name for the new (cloned) VM."},
				"spec": map[string]interface{}{
					"type":        "object",
					"description": `types.VirtualMachineCloneSpec as a JSON object matching its Go struct field names, e.g. {"location":{"pool":"<resource pool moref value>","datastore":"<datastore moref value>"},"powerOn":false,"template":false}. "location" is always required by vSphere; on a topology with only one implicit resource pool (e.g. standalone ESXi) an empty {} location is enough. See referencia/govmomi/vim25/types/types.go's VirtualMachineCloneSpec/VirtualMachineRelocateSpec for the full field list — accepted via generic JSON decode, not a hand-built recursive schema.`,
				},
				"confirm": confirmArg,
			},
			"required": []interface{}{"vm", "folder", "name", "spec", "confirm"},
		},
		Tool{Handler: handleVMClone},
	)

	r.registerDestructive("vmware_vm_instant_clone",
		"Instantly clone a running virtual machine (VMware Instant Clone — near-zero downtime, shares memory pages with the parent until they diverge). NOTE: not simulated by this project's test harness (vcsim has no InstantCloneTask handler) — exercise carefully against a real vCenter first.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm": vmArg,
				"spec": map[string]interface{}{
					"type":        "object",
					"description": `types.VirtualMachineInstantCloneSpec as a JSON object matching its Go struct field names, e.g. {"name":"clone-01","location":{"pool":"<resource pool moref value>"}}. See referencia/govmomi/vim25/types/types.go — accepted via generic JSON decode.`,
				},
				"confirm": confirmArg,
			},
			"required": []interface{}{"vm", "spec", "confirm"},
		},
		Tool{Handler: handleVMInstantClone},
	)

	r.registerDestructive("vmware_vm_relocate",
		"Relocate a virtual machine's storage/compute placement (datastore, host, resource pool) without changing its identity, unlike clone.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm": vmArg,
				"spec": map[string]interface{}{
					"type":        "object",
					"description": `types.VirtualMachineRelocateSpec as a JSON object matching its Go struct field names, e.g. {"pool":"<resource pool moref value>","host":"<host moref value>","datastore":"<datastore moref value>"}. Every field is optional — an empty {} is a valid (no-op) spec. See referencia/govmomi/vim25/types/types.go — accepted via generic JSON decode.`,
				},
				"priority": priorityArg,
				"confirm":  confirmArg,
			},
			"required": []interface{}{"vm", "spec", "confirm"},
		},
		Tool{Handler: handleVMRelocate},
	)

	r.registerDestructive("vmware_vm_migrate",
		"Migrate (vMotion/Storage vMotion) a virtual machine to a different resource pool and/or host, optionally changing its power state. At least one of resource_pool or host is required. NOTE: not simulated by this project's test harness (vcsim has no MigrateVMTask handler) — exercise carefully against a real vCenter first.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm": vmArg,
				"resource_pool": map[string]interface{}{
					"type":        "string",
					"description": `Destination resource pool identifier: a name/pattern as returned (as an inventory path) by vmware_list_resource_pools. Omit to leave the resource pool unchanged (host must then be given).`,
				},
				"host":     hostArg,
				"priority": priorityArg,
				"state": map[string]interface{}{
					"type":        "string",
					"enum":        []interface{}{"poweredOff", "poweredOn", "suspended"},
					"description": "Desired power state after the migration (types.VirtualMachinePowerState). Omit to leave the power state unchanged.",
				},
				"confirm": confirmArg,
			},
			"required": []interface{}{"vm", "confirm"},
		},
		Tool{Handler: handleVMMigrate},
	)

	r.registerDestructive("vmware_vm_customize",
		"Apply a guest OS customization spec (network settings, hostname, sysprep/cloud-init identity, etc.) to a virtual machine. The VM must be powered off. Takes effect on next power-on.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm": vmArg,
				"spec": map[string]interface{}{
					"type":        "object",
					"description": `types.CustomizationSpec as a JSON object matching its Go struct field names, e.g. {"globalIPSettings":{},"nicSettingMap":[],"identity":{...}}. "identity" is polymorphic (types.BaseCustomizationIdentitySettings, e.g. a Sysprep/Linux/CloudinitPrep variant) — this tool accepts it via generic JSON decode into the field names of the concrete Go struct you intend, there is no schema-guided discriminator for which variant to use. See referencia/govmomi/vim25/types/types.go's CustomizationSpec for the full field list.`,
				},
				"confirm": confirmArg,
			},
			"required": []interface{}{"vm", "spec", "confirm"},
		},
		Tool{Handler: handleVMCustomize},
	)

	r.registerDestructive("vmware_vm_export",
		"Start an OVF export lease for a virtual machine and return the downloadable device (VMDK/OVF) URLs once the lease is ready. Does not download the files itself — a caller fetches the returned URLs separately. NOTE: not simulated by this project's test harness (vcsim has no ExportVm handler) — exercise carefully against a real vCenter/ESXi first.",
		tier2,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"vm": vmArg, "confirm": confirmArg},
			"required":   []interface{}{"vm", "confirm"},
		},
		Tool{Handler: handleVMExport},
	)

	r.registerDestructive("vmware_vm_export_snapshot",
		"Start an OVF export lease for all VMDK files up to (but not including) a given snapshot — useful for exporting a running VM's on-disk state as of that snapshot. Returns the downloadable device URLs once ready, same as vmware_vm_export.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"snapshot": map[string]interface{}{
					"type":        "string",
					"description": `The snapshot's moref VALUE (not a name) — e.g. the "value" field of the "snapshot" object returned by vmware_vm_snapshot_find (its Type is always "VirtualMachineSnapshot", filled in automatically).`,
				},
				"confirm": confirmArg,
			},
			"required": []interface{}{"snapshot", "confirm"},
		},
		Tool{Handler: handleVMExportSnapshot},
	)

	r.registerDestructive("vmware_vm_promote_disks",
		"Promote a virtual machine's delta-disk-backed disks by consolidating their delta chain into the base disk. Omit/empty \"disks\" to promote every eligible disk (vSphere's documented default when the list is unset or empty) — the common case.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm":     vmArg,
				"unlink": map[string]interface{}{"type": "boolean", "description": "Unlink the promoted disks from their delta chain as part of promotion. Default false."},
				"disks": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "object"},
					"description": `Advanced: specific disks to promote instead of every eligible disk. Each element is a types.VirtualDisk JSON object matching its Go struct field names (must include a "key" matching an existing virtual disk device on the VM). Omit for the common case (promote everything eligible).`,
				},
				"confirm": confirmArg,
			},
			"required": []interface{}{"vm", "confirm"},
		},
		Tool{Handler: handleVMPromoteDisks},
	)
}

// registerVMProvisioningVCenterOnlyTools registers the 3 pre-flight-check
// tools built on object.VmProvisioningChecker/object.VmCompatibilityChecker —
// see registerVMProvisioningTools's doc comment, deviation 1, for why these
// are split into their own toolMode/function instead of living in
// registerVMProvisioningTools.
func registerVMProvisioningVCenterOnlyTools(r *Registry) {
	vmArg := map[string]interface{}{
		"type":        "string",
		"description": `VM identifier: a name/pattern (e.g. "cac-WN02") or a full inventory path as returned by vmware_list_vms. Must resolve to exactly one VM.`,
	}
	hostArg := map[string]interface{}{
		"type":        "string",
		"description": `Host identifier: a name/pattern (e.g. "esxi-01.local") or a full inventory path as returned by vmware_list_hosts. Must resolve to exactly one host.`,
	}
	poolArg := map[string]interface{}{
		"type":        "string",
		"description": `Resource pool identifier: a name/pattern as returned (as an inventory path) by vmware_list_resource_pools.`,
	}
	testTypesArg := map[string]interface{}{
		"type":        "array",
		"items":       map[string]interface{}{"type": "string"},
		"description": "Optional list of types.CheckTestType values to limit which checks run. Omit to run every applicable check.",
	}

	r.register("vmware_vm_provisioning_check_relocate",
		"Dry-run check of whether relocating a virtual machine with the given spec is feasible, without actually relocating it. Read-only — requires vCenter (object.VmProvisioningChecker is not available on standalone ESXi).",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm": vmArg,
				"spec": map[string]interface{}{
					"type":        "object",
					"description": `types.VirtualMachineRelocateSpec as a JSON object matching its Go struct field names — same shape as vmware_vm_relocate's "spec". Accepted via generic JSON decode.`,
				},
				"test_types": testTypesArg,
			},
			"required": []interface{}{"vm", "spec"},
		},
		Tool{Handler: handleVMProvisioningCheckRelocate},
	)

	r.register("vmware_vm_compatibility_check_config",
		"Dry-run check of whether a virtual hardware/config spec is compatible with a given VM's current placement and/or a specific host/resource pool, without applying it. At least one of vm, host, or pool is required (used to resolve the host(s) being checked against). Read-only — requires vCenter (object.VmCompatibilityChecker is not available on standalone ESXi).",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"spec": map[string]interface{}{
					"type":        "object",
					"description": `types.VirtualMachineConfigSpec as a JSON object matching its Go struct field names, e.g. {"numCPUs":4,"memoryMB":8192,"guestId":"otherGuest64"}. Accepted via generic JSON decode.`,
				},
				"vm":         vmArg,
				"host":       hostArg,
				"pool":       poolArg,
				"test_types": testTypesArg,
			},
			"required": []interface{}{"spec"},
		},
		Tool{Handler: handleVMCompatibilityCheckConfig},
	)

	r.register("vmware_vm_compatibility_check",
		"Dry-run check of whether an existing virtual machine is compatible with a specific host and/or resource pool (e.g. before a manual migration), without moving it. Read-only — requires vCenter (object.VmCompatibilityChecker is not available on standalone ESXi).",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm":         vmArg,
				"host":       hostArg,
				"pool":       poolArg,
				"test_types": testTypesArg,
			},
			"required": []interface{}{"vm"},
		},
		Tool{Handler: handleVMCompatibilityCheck},
	)
}

// decodeInto JSON-round-trips raw — the already-decoded interface{} tree a
// JSON-RPC tool argument arrives as (map[string]interface{}/[]interface{}/
// scalars) — into out, a pointer to a concrete govmomi struct. This is the
// "accept the concrete struct via generic JSON, not a hand-built recursive
// schema" approach this file's complex/polymorphic specs need — see
// generated_option.go's Update handler and generated_vm_snapshot.go's
// decodeQuiesceSpec for the precedent. A nil raw is a no-op (out keeps its
// zero value) so callers can pass an optional field through unconditionally.
func decodeInto(raw interface{}, out interface{}) error {
	if raw == nil {
		return nil
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}

// resolveResourcePool resolves name to exactly one resource pool, scoped
// with dcScopedPath("host", name) — resource pools live under the
// per-datacenter "host" inventory folder, same as HostSystem/
// ClusterComputeResource (see inventory.go's handleListResourcePools).
func resolveResourcePool(ctx context.Context, client *vmware.Client, name string) (*object.ResourcePool, error) {
	if name == "" {
		return nil, fmt.Errorf("resource pool name is required")
	}
	pool, err := client.Finder.ResourcePool(ctx, dcScopedPath("host", name))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve resource pool %q: %w", name, err)
	}
	return pool, nil
}

// movePriorityArg reads the optional "priority" argument, defaulting to
// types.VirtualMachineMovePriorityDefaultPriority — matching vSphere's own
// default for Relocate/Migrate when a caller doesn't care.
func movePriorityArg(args map[string]interface{}) types.VirtualMachineMovePriority {
	if s, _ := args["priority"].(string); s != "" {
		return types.VirtualMachineMovePriority(s)
	}
	return types.VirtualMachineMovePriorityDefaultPriority
}

// powerStateArg reads the optional "state" argument. An empty
// types.VirtualMachinePowerState("") is vSphere's own "leave the power state
// unchanged" signal for MigrateVM_Task (confirmed by reading
// object/virtual_machine.go's Migrate and its ManagedObjectTypes doc — State
// is not a pointer, and simulator.MigrateVMTask/the real API treat "" as a
// no-op), so omitting "state" naturally passes that through.
func powerStateArg(args map[string]interface{}) types.VirtualMachinePowerState {
	s, _ := args["state"].(string)
	return types.VirtualMachinePowerState(s)
}

// checkTestTypesArg reads the optional "test_types" string array shared by
// all 3 vcenter-only check tools.
func checkTestTypesArg(args map[string]interface{}) ([]types.CheckTestType, error) {
	raw, ok := args["test_types"].([]interface{})
	if !ok || len(raw) == 0 {
		return nil, nil
	}
	out := make([]types.CheckTestType, 0, len(raw))
	for i, v := range raw {
		s, ok := v.(string)
		if !ok || s == "" {
			return nil, fmt.Errorf("test_types[%d] must be a non-empty string", i)
		}
		out = append(out, types.CheckTestType(s))
	}
	return out, nil
}

// leaseResult waits for lease to leave the Initializing state (Lease.Wait
// with a nil items list — per nfc/lease.go's newLeaseInfo, len(items)==0 is
// the "this is an export" branch) and formats its downloadable device URLs.
// Shared by handleVMExport and handleVMExportSnapshot — neither tool
// downloads the files itself, see this file's top doc comment, deviation 5.
func leaseResult(ctx context.Context, subject string, lease *nfc.Lease) (string, error) {
	info, err := lease.Wait(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("export lease for %s did not become ready: %w", subject, err)
	}

	devices := make([]map[string]interface{}, 0, len(info.Items))
	for _, item := range info.Items {
		device := map[string]interface{}{
			"device_id": item.DeviceId,
			"path":      item.Path,
			"size":      item.Size,
		}
		if item.URL != nil {
			device["url"] = item.URL.String()
		}
		devices = append(devices, device)
	}

	return marshalJSON(map[string]interface{}{
		"subject": subject,
		"result":  "export_lease_ready",
		"count":   len(devices),
		"devices": devices,
	})
}

func handleVMClone(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, err := resolveVM(ctx, client, args)
	if err != nil {
		return "", err
	}

	folderName, _ := args["folder"].(string)
	if folderName == "" {
		return "", fmt.Errorf("folder is required")
	}
	folder, err := client.Finder.Folder(ctx, dcScopedPath("vm", folderName))
	if err != nil {
		return "", fmt.Errorf("failed to resolve folder %q: %w", folderName, err)
	}

	name, _ := args["name"].(string)
	if name == "" {
		return "", fmt.Errorf("name is required")
	}

	if args["spec"] == nil {
		return "", fmt.Errorf("spec is required")
	}
	var spec types.VirtualMachineCloneSpec
	if err := decodeInto(args["spec"], &spec); err != nil {
		return "", fmt.Errorf("invalid spec: %w", err)
	}

	task, err := vm.Clone(ctx, folder, name, spec)
	if err != nil {
		return "", fmt.Errorf("failed to clone %s to %q: %w", vm.InventoryPath, name, err)
	}
	if err := waitForTask(ctx, task); err != nil {
		return "", fmt.Errorf("clone task failed for %s: %w", vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{
		"vm":       vm.InventoryPath,
		"folder":   folder.InventoryPath,
		"new_name": name,
		"result":   "cloned",
	})
}

func handleVMInstantClone(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, err := resolveVM(ctx, client, args)
	if err != nil {
		return "", err
	}

	if args["spec"] == nil {
		return "", fmt.Errorf("spec is required")
	}
	var spec types.VirtualMachineInstantCloneSpec
	if err := decodeInto(args["spec"], &spec); err != nil {
		return "", fmt.Errorf("invalid spec: %w", err)
	}

	task, err := vm.InstantClone(ctx, spec)
	if err != nil {
		return "", fmt.Errorf("failed to instant-clone %s: %w", vm.InventoryPath, err)
	}
	if err := waitForTask(ctx, task); err != nil {
		return "", fmt.Errorf("instant-clone task failed for %s: %w", vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"vm": vm.InventoryPath, "result": "instant_cloned"})
}

func handleVMRelocate(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, err := resolveVM(ctx, client, args)
	if err != nil {
		return "", err
	}

	if args["spec"] == nil {
		return "", fmt.Errorf("spec is required")
	}
	var spec types.VirtualMachineRelocateSpec
	if err := decodeInto(args["spec"], &spec); err != nil {
		return "", fmt.Errorf("invalid spec: %w", err)
	}

	priority := movePriorityArg(args)

	task, err := vm.Relocate(ctx, spec, priority)
	if err != nil {
		return "", fmt.Errorf("failed to relocate %s: %w", vm.InventoryPath, err)
	}
	if err := waitForTask(ctx, task); err != nil {
		return "", fmt.Errorf("relocate task failed for %s: %w", vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"vm": vm.InventoryPath, "result": "relocated"})
}

func handleVMMigrate(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, err := resolveVM(ctx, client, args)
	if err != nil {
		return "", err
	}

	var pool *object.ResourcePool
	if name, _ := args["resource_pool"].(string); name != "" {
		pool, err = resolveResourcePool(ctx, client, name)
		if err != nil {
			return "", err
		}
	}

	var host *object.HostSystem
	if name, _ := args["host"].(string); name != "" {
		host, err = resolveHost(ctx, client, args)
		if err != nil {
			return "", err
		}
	}

	if pool == nil && host == nil {
		return "", fmt.Errorf("at least one of resource_pool or host is required")
	}

	priority := movePriorityArg(args)
	state := powerStateArg(args)

	task, err := vm.Migrate(ctx, pool, host, priority, state)
	if err != nil {
		return "", fmt.Errorf("failed to migrate %s: %w", vm.InventoryPath, err)
	}
	if err := waitForTask(ctx, task); err != nil {
		return "", fmt.Errorf("migrate task failed for %s: %w", vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"vm": vm.InventoryPath, "result": "migrated"})
}

func handleVMCustomize(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, err := resolveVM(ctx, client, args)
	if err != nil {
		return "", err
	}

	if args["spec"] == nil {
		return "", fmt.Errorf("spec is required")
	}
	var spec types.CustomizationSpec
	if err := decodeInto(args["spec"], &spec); err != nil {
		return "", fmt.Errorf("invalid spec: %w", err)
	}

	task, err := vm.Customize(ctx, spec)
	if err != nil {
		return "", fmt.Errorf("failed to customize %s: %w", vm.InventoryPath, err)
	}
	if err := waitForTask(ctx, task); err != nil {
		return "", fmt.Errorf("customize task failed for %s: %w", vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"vm": vm.InventoryPath, "result": "customized"})
}

func handleVMExport(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, err := resolveVM(ctx, client, args)
	if err != nil {
		return "", err
	}

	lease, err := vm.Export(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to start export lease for %s: %w", vm.InventoryPath, err)
	}

	return leaseResult(ctx, vm.InventoryPath, lease)
}

func handleVMExportSnapshot(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	value, _ := args["snapshot"].(string)
	if value == "" {
		return "", fmt.Errorf("snapshot is required")
	}
	ref := types.ManagedObjectReference{Type: "VirtualMachineSnapshot", Value: value}

	// object.VirtualMachine.ExportSnapshot's SOAP request targets the
	// snapshot's own moref (This: *snapshot — see
	// referencia/govmomi/object/virtual_machine.go); the *object.VirtualMachine
	// receiver only supplies the underlying vim25 client
	// (object.Common.Client(), independent of its own moref — see
	// referencia/govmomi/object/common.go), so a handle with an empty moref
	// bound to this connection's client is sufficient. There is no need (and
	// no governomi-exposed way) to resolve a specific VM by name for this
	// call — the snapshot's moref alone identifies what gets exported.
	vm := object.NewVirtualMachine(client.Client.Client, types.ManagedObjectReference{})

	lease, err := vm.ExportSnapshot(ctx, &ref)
	if err != nil {
		return "", fmt.Errorf("failed to start export-snapshot lease for snapshot %s: %w", value, err)
	}

	return leaseResult(ctx, fmt.Sprintf("snapshot:%s", value), lease)
}

func handleVMPromoteDisks(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, err := resolveVM(ctx, client, args)
	if err != nil {
		return "", err
	}

	unlink, _ := args["unlink"].(bool)

	var disks []types.VirtualDisk
	if raw, ok := args["disks"].([]interface{}); ok && len(raw) > 0 {
		if err := decodeInto(raw, &disks); err != nil {
			return "", fmt.Errorf("invalid disks: %w", err)
		}
	}

	task, err := vm.PromoteDisks(ctx, unlink, disks)
	if err != nil {
		return "", fmt.Errorf("failed to promote disks on %s: %w", vm.InventoryPath, err)
	}
	if err := waitForTask(ctx, task); err != nil {
		return "", fmt.Errorf("promote-disks task failed for %s: %w", vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{
		"vm":         vm.InventoryPath,
		"result":     "disks_promoted",
		"disk_count": len(disks),
	})
}

// requireProvisioningChecker is the explicit, friendly nil-guard for
// object.NewVmProvisioningChecker — see this file's top doc comment,
// deviation 1, for the panic this prevents and why registry.go's recover()
// alone isn't enough. client.Client.Client is this connection's *vim25.Client
// (same field path datastore.go's object.NewFileManager(client.Client.Client)
// uses — vmware.Client embeds *govmomi.Client [field name "Client"], which
// itself embeds *vim25.Client [also field name "Client"]).
func requireProvisioningChecker(client *vmware.Client) (*object.VmProvisioningChecker, error) {
	vc := client.Client.Client
	if vc.ServiceContent.VmProvisioningChecker == nil {
		return nil, fmt.Errorf("vmware_vm_provisioning_check_relocate is not available on this connection (VmProvisioningChecker requires vCenter — it is not present on a standalone ESXi host's ServiceContent)")
	}
	return object.NewVmProvisioningChecker(vc), nil
}

// requireCompatibilityChecker is requireProvisioningChecker's counterpart
// for object.NewVmCompatibilityChecker — same nil-guard reasoning, shared by
// both vmware_vm_compatibility_check_config and vmware_vm_compatibility_check.
func requireCompatibilityChecker(client *vmware.Client) (*object.VmCompatibilityChecker, error) {
	vc := client.Client.Client
	if vc.ServiceContent.VmCompatibilityChecker == nil {
		return nil, fmt.Errorf("this compatibility-check tool is not available on this connection (VmCompatibilityChecker requires vCenter — it is not present on a standalone ESXi host's ServiceContent)")
	}
	return object.NewVmCompatibilityChecker(vc), nil
}

func handleVMProvisioningCheckRelocate(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, err := resolveVM(ctx, client, args)
	if err != nil {
		return "", err
	}

	if args["spec"] == nil {
		return "", fmt.Errorf("spec is required")
	}
	var spec types.VirtualMachineRelocateSpec
	if err := decodeInto(args["spec"], &spec); err != nil {
		return "", fmt.Errorf("invalid spec: %w", err)
	}

	testTypes, err := checkTestTypesArg(args)
	if err != nil {
		return "", err
	}

	checker, err := requireProvisioningChecker(client)
	if err != nil {
		return "", err
	}

	results, err := checker.CheckRelocate(ctx, vm.Reference(), spec, testTypes...)
	if err != nil {
		return "", fmt.Errorf("check-relocate failed for %s: %w", vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{
		"vm":      vm.InventoryPath,
		"count":   len(results),
		"results": results,
	})
}

func handleVMCompatibilityCheckConfig(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	if args["spec"] == nil {
		return "", fmt.Errorf("spec is required")
	}
	var spec types.VirtualMachineConfigSpec
	if err := decodeInto(args["spec"], &spec); err != nil {
		return "", fmt.Errorf("invalid spec: %w", err)
	}

	var vmRef, hostRef, poolRef *types.ManagedObjectReference

	if name, _ := args["vm"].(string); name != "" {
		vm, err := resolveVM(ctx, client, args)
		if err != nil {
			return "", err
		}
		ref := vm.Reference()
		vmRef = &ref
	}
	if name, _ := args["host"].(string); name != "" {
		host, err := resolveHost(ctx, client, args)
		if err != nil {
			return "", err
		}
		ref := host.Reference()
		hostRef = &ref
	}
	if name, _ := args["pool"].(string); name != "" {
		pool, err := resolveResourcePool(ctx, client, name)
		if err != nil {
			return "", err
		}
		ref := pool.Reference()
		poolRef = &ref
	}
	if vmRef == nil && hostRef == nil && poolRef == nil {
		return "", fmt.Errorf("at least one of vm, host, or pool is required")
	}

	testTypes, err := checkTestTypesArg(args)
	if err != nil {
		return "", err
	}

	checker, err := requireCompatibilityChecker(client)
	if err != nil {
		return "", err
	}

	results, err := checker.CheckVmConfig(ctx, spec, vmRef, hostRef, poolRef, testTypes...)
	if err != nil {
		return "", fmt.Errorf("check-vm-config failed: %w", err)
	}

	return marshalJSON(map[string]interface{}{
		"count":   len(results),
		"results": results,
	})
}

func handleVMCompatibilityCheck(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, err := resolveVM(ctx, client, args)
	if err != nil {
		return "", err
	}

	var hostRef, poolRef *types.ManagedObjectReference

	if name, _ := args["host"].(string); name != "" {
		host, err := resolveHost(ctx, client, args)
		if err != nil {
			return "", err
		}
		ref := host.Reference()
		hostRef = &ref
	}
	if name, _ := args["pool"].(string); name != "" {
		pool, err := resolveResourcePool(ctx, client, name)
		if err != nil {
			return "", err
		}
		ref := pool.Reference()
		poolRef = &ref
	}

	testTypes, err := checkTestTypesArg(args)
	if err != nil {
		return "", err
	}

	checker, err := requireCompatibilityChecker(client)
	if err != nil {
		return "", err
	}

	results, err := checker.CheckCompatibility(ctx, vm.Reference(), hostRef, poolRef, testTypes...)
	if err != nil {
		return "", fmt.Errorf("check-compatibility failed for %s: %w", vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{
		"vm":      vm.InventoryPath,
		"count":   len(results),
		"results": results,
	})
}

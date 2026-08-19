package tools

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// registerVMFaultToleranceTools adds the 7 vSphere Fault Tolerance (FT)
// methods of the VirtualMachine managed object
// (referencia/govmomi/vim25/types/types.go: CreateSecondaryVM_Task,
// CreateSecondaryVMEx_Task, DisableSecondaryVM_Task, EnableSecondaryVM_Task,
// MakePrimaryVM_Task, TerminateFaultTolerantVM_Task,
// TurnOffFaultToleranceForVM_Task) that have no tool anywhere else in this
// project — confirmed by grepping every generated_vm_*.go/vm.go file for
// each method name before writing this one.
//
// No object.VirtualMachine wrapper exists for any of the 7 (confirmed by
// grepping referencia/govmomi/object/virtual_machine.go for "SecondaryVM",
// "FaultTolerant", "FaultToleranceForVM", "MakePrimaryVM" — zero matches, and
// a search of the entire referencia/govmomi/object tree for the same terms
// also comes back empty). Every handler below therefore dials the raw vim25
// SOAP method directly — methods.Xxx_Task(ctx, client.Client.Client,
// &types.Xxx_Task{This: ref, ...}) — the same "no object.* wrapper, go
// straight to methods+types" pattern generated_host_iscsi_portbinding.go
// documents at length for IscsiManager. Each of the 7 request structs was
// read directly from types.go (not assumed from a brief/generator report)
// to confirm field names/pointer-ness:
//
//   - CreateSecondaryVMRequestType / CreateSecondaryVMExRequestType: This
//     ManagedObjectReference, Host *ManagedObjectReference (optional — "If no
//     host is specified, a compatible host will be selected by the system").
//     CreateSecondaryVMExRequestType additionally has Spec
//     *FaultToleranceConfigSpec (optional).
//   - DisableSecondaryVMRequestType / EnableSecondaryVMRequestType /
//     MakePrimaryVMRequestType: This ManagedObjectReference, Vm
//     ManagedObjectReference (required — the secondary VM in the FT group;
//     for MakePrimaryVM, the secondary that is promoted to primary).
//     EnableSecondaryVMRequestType additionally has Host
//     *ManagedObjectReference (optional, same "system picks a compatible
//     host" semantics as Create).
//   - TerminateFaultTolerantVMRequestType: This ManagedObjectReference, Vm
//     *ManagedObjectReference (OPTIONAL, unlike Disable/Enable/MakePrimary's
//     required Vm — per its doc comment, "If no virtual machine is
//     specified, all secondary virtual machines will be terminated").
//   - TurnOffFaultToleranceForVMRequestType: This ManagedObjectReference
//     only, no other fields.
//
// Every one of the 7 *_TaskResponse types has a plain "Returnval
// ManagedObjectReference" (the task moref, not a pointer) — this file's
// ftWaitTask helper wraps that moref in an *object.Task via
// object.NewTask(client.Client.Client, ref), the exact same client-side-only
// construction (no round trip until the first real call against it)
// generated_task.go's resolveTaskArg already uses for a caller-supplied task
// moref, and then calls this package's existing waitForTask (vm.go) —
// blocking synchronously until the task completes, matching every other
// tier2 tool in this project (no async/polling tool exists anywhere here).
//
// Class: modeVCenterOnly. The SOAP methods themselves are defined on
// VirtualMachine (present on a standalone ESXi connection too, unlike e.g.
// ClusterComputeResource-scoped DRS/HA methods, which genuinely don't exist
// there) — so this is a judgment call, not a nil-moref-panic certainty like
// generated_vm_provisioning.go's VmProvisioningChecker classification.
// Reasoning, in order of how verifiable each point is:
//  1. Verified structurally in this project: every resolveHost-based tool
//     against simulator.ESX() (see host_test.go's firstHostPath, used by
//     dozens of tests in this package) resolves exactly ONE host — a
//     standalone ESXi connection's "host" Finder folder only ever contains
//     the host you're connected to. FT's entire purpose is running the
//     secondary VM instance on a DIFFERENT host than the primary for
//     redundancy (CreateSecondaryVM's own doc comment: "a compatible host
//     will be selected by the system" — implying a choice among several).
//     A connection whose inventory structurally contains only one host can
//     never supply that second host, regardless of whether the SOAP method
//     is nominally callable.
//  2. Not independently re-verified in this pass but widely documented
//     VMware product behavior (vSphere Availability Guide): vSphere Fault
//     Tolerance's primary/secondary VM pairing, host compatibility checks,
//     and EVC requirements are managed through vCenter Server — the same
//     "vCenter-managed cluster feature" bucket this project already puts
//     DRS/HA in (registerInventoryComputeVCenterOnlyTools, registerNetworkTools).
//
// Given both, all 7 tools are registered under modeVCenterOnly.
//
// Tier: registerDestructive/tier2 (disruptive but reversible) for all 7 —
// per this group's brief: DisableSecondaryVM is reversed by
// EnableSecondaryVM, TurnOffFaultToleranceForVM's effect can be reversed by
// re-enabling FT (CreateSecondaryVM again), and Create/Enable/MakePrimary/
// Terminate are all normal, expected FT lifecycle operations rather than
// irreversible data-destroying actions (nothing here deletes a VM's disks or
// the primary VM itself).
//
// vcsim coverage: NONE of the 7 have a simulator-side handler — confirmed by
// grepping referencia/govmomi/simulator/virtual_machine.go for a matching
// "func (vm *VirtualMachine) Xxx" receiver method for every one of the 7
// names and finding zero matches (the same "grep the whole simulator tree,
// find no receiver" verification generated_virtual_disk.go's top doc comment
// used for InflateVirtualDisk/ShrinkVirtualDisk/CreateChildDisk). A call
// against vcsim always faults types.MethodNotFound (simulator/simulator.go's
// method-dispatch fallback). generated_vm_ft_test.go drives every tool with
// assertReachesServer (generated_vm_lifecycle_test.go) for exactly this
// reason: it proves the wiring (schema, tier gate, resolveVM/resolveHost,
// raw SOAP dispatch) reaches vcsim and gets back a clean server-side fault,
// not an unknown-tool wiring bug or a recovered panic. Behavioral validation
// of a real FT pairing is expected against a real vCenter-managed cluster
// with 2+ compatible hosts.
func registerVMFaultToleranceTools(r *Registry) {
	vmArg := map[string]interface{}{
		"type":        "string",
		"description": `Primary virtual machine identifier: a name/pattern (e.g. "cac-WN02") or a full inventory path (e.g. "/ha-datacenter/vm/cac-WN02") as returned by vmware_list_vms. Must resolve to exactly one VM. For vmware_vm_disable_secondary/enable_secondary/make_primary/terminate_fault_tolerant/turn_off_fault_tolerance, this must be the PRIMARY VM of the fault tolerant group — these operations "can only be invoked from the primary virtual machine in the group" per the underlying API.`,
	}
	secondaryVMArg := map[string]interface{}{
		"type":        "string",
		"description": `Secondary virtual machine identifier (name/pattern or full inventory path, same resolution rules as "vm"). Must already be part of the fault tolerant group associated with "vm".`,
	}
	hostArg := map[string]interface{}{
		"type":        "string",
		"description": `Host identifier (name/pattern or full inventory path, see vmware_list_hosts) where the secondary VM should be placed/powered on. Optional — omit to let vSphere pick a compatible host automatically. If the secondary VM is not compatible with an explicitly given host, the operation fails.`,
	}
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}

	r.registerDestructive("vmware_vm_create_secondary",
		"Turn on vSphere Fault Tolerance for a virtual machine by creating and powering on its secondary VM. The primary VM becomes fault tolerant once the secondary is created. Reversible via vmware_vm_turn_off_fault_tolerance.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm":      vmArg,
				"host":    hostArg,
				"confirm": confirmArg,
			},
			"required": []interface{}{"vm", "confirm"},
		},
		Tool{Handler: handleVMCreateSecondaryVM},
	)

	r.registerDestructive("vmware_vm_create_secondary_ex",
		"Turn on vSphere Fault Tolerance for a virtual machine, like vmware_vm_create_secondary, but additionally accepting an explicit FaultToleranceConfigSpec (storage placement for the tie-breaker file, secondary config file, and secondary disks). Reversible via vmware_vm_turn_off_fault_tolerance.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm":      vmArg,
				"host":    hostArg,
				"spec":    map[string]interface{}{"type": "object", "description": "A types.FaultToleranceConfigSpec JSON object matching its Go struct fields (e.g. {\"secondaryVmSpec\": {...}, \"metaDataPath\": {...}}). Optional — omit for system defaults."},
				"confirm": confirmArg,
			},
			"required": []interface{}{"vm", "confirm"},
		},
		Tool{Handler: handleVMCreateSecondaryVMEx},
	)

	r.registerDestructive("vmware_vm_disable_secondary",
		"Disable (but do not remove) a secondary VM in a fault tolerant group, invoked from the primary VM. The secondary stops shadowing the primary but remains part of the group. Reversible via vmware_vm_enable_secondary.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm":           vmArg,
				"secondary_vm": secondaryVMArg,
				"confirm":      confirmArg,
			},
			"required": []interface{}{"vm", "secondary_vm", "confirm"},
		},
		Tool{Handler: handleVMDisableSecondaryVM},
	)

	r.registerDestructive("vmware_vm_enable_secondary",
		"Re-enable a previously disabled secondary VM in a fault tolerant group, invoked from the primary VM, optionally onto a specific host. Reverses vmware_vm_disable_secondary.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm":           vmArg,
				"secondary_vm": secondaryVMArg,
				"host":         hostArg,
				"confirm":      confirmArg,
			},
			"required": []interface{}{"vm", "secondary_vm", "confirm"},
		},
		Tool{Handler: handleVMEnableSecondaryVM},
	)

	r.registerDestructive("vmware_vm_make_primary",
		"Promote a secondary VM to become the new primary VM of its fault tolerant group, invoked from the current primary VM. The former primary becomes a secondary.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm":           vmArg,
				"secondary_vm": secondaryVMArg,
				"confirm":      confirmArg,
			},
			"required": []interface{}{"vm", "secondary_vm", "confirm"},
		},
		Tool{Handler: handleVMMakePrimaryVM},
	)

	r.registerDestructive("vmware_vm_terminate_fault_tolerant",
		"Terminate one secondary VM (or, if secondary_vm is omitted, ALL secondary VMs) in a fault tolerant group, invoked from the primary VM, allowing fault tolerance protection to lapse for that secondary. A terminated secondary may be respawned later (potentially on a different host) via vmware_vm_create_secondary/enable_secondary.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm":           vmArg,
				"secondary_vm": map[string]interface{}{"type": "string", "description": `Secondary VM to terminate (name/pattern or full inventory path). Optional — omit to terminate ALL secondary VMs in the group.`},
				"confirm":      confirmArg,
			},
			"required": []interface{}{"vm", "confirm"},
		},
		Tool{Handler: handleVMTerminateFaultTolerantVM},
	)

	r.registerDestructive("vmware_vm_turn_off_fault_tolerance",
		"Turn off vSphere Fault Tolerance for a virtual machine entirely, removing its secondary VM(s) and the FT relationship. Reversible by turning FT back on via vmware_vm_create_secondary/create_secondary_ex.",
		tier2,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"vm": vmArg, "confirm": confirmArg},
			"required":   []interface{}{"vm", "confirm"},
		},
		Tool{Handler: handleVMTurnOffFaultToleranceForVM},
	)
}

// ftSecondaryVM resolves the required "secondary_vm" argument to exactly one
// VM by building a one-key args submap under "vm" and delegating to this
// package's existing resolveVM (vm.go) — the same "reuse resolveVM/
// resolveHost through a one-key submap" pattern generated_vm_lifecycle.go's
// handleVMMarkAsVirtualMachine already uses for its optional "host"
// argument, applied here to a required argument under a different name.
func ftSecondaryVM(ctx context.Context, client *vmware.Client, args map[string]interface{}) (*object.VirtualMachine, error) {
	name, _ := args["secondary_vm"].(string)
	if name == "" {
		return nil, fmt.Errorf("secondary_vm is required")
	}
	return resolveVM(ctx, client, map[string]interface{}{"vm": name})
}

// ftOptionalHost resolves the optional "host" argument to a *object.HostSystem
// via resolveHost (host.go), or returns (nil, nil) when omitted/empty — same
// one-key-submap reuse pattern as ftSecondaryVM above.
func ftOptionalHost(ctx context.Context, client *vmware.Client, args map[string]interface{}) (*object.HostSystem, error) {
	name, _ := args["host"].(string)
	if name == "" {
		return nil, nil
	}
	return resolveHost(ctx, client, map[string]interface{}{"host": name})
}

// ftWaitTask blocks until the task moref returned by one of the 7 raw
// methods.Xxx_Task calls in this file completes, wrapping the bare
// types.ManagedObjectReference in a client-side-only *object.Task (no round
// trip until the first real call against it — the same construction
// generated_task.go's resolveTaskArg uses for a caller-supplied task moref)
// and delegating to this package's existing waitForTask (vm.go). Needed
// because none of the 7 FT methods have an object.VirtualMachine wrapper
// (see this file's top doc comment) to hand back an *object.Task directly,
// unlike e.g. vm.PowerOn(ctx) elsewhere in this project.
func ftWaitTask(ctx context.Context, client *vmware.Client, ref types.ManagedObjectReference) error {
	return waitForTask(ctx, object.NewTask(client.Client.Client, ref))
}

func handleVMCreateSecondaryVM(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, err := resolveVM(ctx, client, args)
	if err != nil {
		return "", err
	}
	host, err := ftOptionalHost(ctx, client, args)
	if err != nil {
		return "", err
	}

	req := &types.CreateSecondaryVM_Task{This: vm.Reference()}
	if host != nil {
		ref := host.Reference()
		req.Host = &ref
	}

	resp, err := methods.CreateSecondaryVM_Task(ctx, client.Client.Client, req)
	if err != nil {
		return "", fmt.Errorf("failed to create secondary VM for %s: %w", vm.InventoryPath, err)
	}
	if err := ftWaitTask(ctx, client, resp.Returnval); err != nil {
		return "", fmt.Errorf("create-secondary-vm task failed for %s: %w", vm.InventoryPath, err)
	}

	result := map[string]interface{}{"vm": vm.InventoryPath, "result": "secondary_vm_created"}
	if host != nil {
		result["host"] = host.InventoryPath
	}
	return marshalJSON(result)
}

func handleVMCreateSecondaryVMEx(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, err := resolveVM(ctx, client, args)
	if err != nil {
		return "", err
	}
	host, err := ftOptionalHost(ctx, client, args)
	if err != nil {
		return "", err
	}

	req := &types.CreateSecondaryVMEx_Task{This: vm.Reference()}
	if host != nil {
		ref := host.Reference()
		req.Host = &ref
	}
	if raw, ok := args["spec"]; ok {
		var spec types.FaultToleranceConfigSpec
		if err := decodeJSONArg(raw, &spec); err != nil {
			return "", fmt.Errorf("invalid spec: %w", err)
		}
		req.Spec = &spec
	}

	resp, err := methods.CreateSecondaryVMEx_Task(ctx, client.Client.Client, req)
	if err != nil {
		return "", fmt.Errorf("failed to create secondary VM (ex) for %s: %w", vm.InventoryPath, err)
	}
	if err := ftWaitTask(ctx, client, resp.Returnval); err != nil {
		return "", fmt.Errorf("create-secondary-vm-ex task failed for %s: %w", vm.InventoryPath, err)
	}

	result := map[string]interface{}{"vm": vm.InventoryPath, "result": "secondary_vm_created"}
	if host != nil {
		result["host"] = host.InventoryPath
	}
	return marshalJSON(result)
}

func handleVMDisableSecondaryVM(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, err := resolveVM(ctx, client, args)
	if err != nil {
		return "", err
	}
	secondary, err := ftSecondaryVM(ctx, client, args)
	if err != nil {
		return "", err
	}

	resp, err := methods.DisableSecondaryVM_Task(ctx, client.Client.Client, &types.DisableSecondaryVM_Task{
		This: vm.Reference(),
		Vm:   secondary.Reference(),
	})
	if err != nil {
		return "", fmt.Errorf("failed to disable secondary VM %s (primary %s): %w", secondary.InventoryPath, vm.InventoryPath, err)
	}
	if err := ftWaitTask(ctx, client, resp.Returnval); err != nil {
		return "", fmt.Errorf("disable-secondary-vm task failed for %s: %w", secondary.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"vm": vm.InventoryPath, "secondary_vm": secondary.InventoryPath, "result": "secondary_disabled"})
}

func handleVMEnableSecondaryVM(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, err := resolveVM(ctx, client, args)
	if err != nil {
		return "", err
	}
	secondary, err := ftSecondaryVM(ctx, client, args)
	if err != nil {
		return "", err
	}
	host, err := ftOptionalHost(ctx, client, args)
	if err != nil {
		return "", err
	}

	req := &types.EnableSecondaryVM_Task{
		This: vm.Reference(),
		Vm:   secondary.Reference(),
	}
	if host != nil {
		ref := host.Reference()
		req.Host = &ref
	}

	resp, err := methods.EnableSecondaryVM_Task(ctx, client.Client.Client, req)
	if err != nil {
		return "", fmt.Errorf("failed to enable secondary VM %s (primary %s): %w", secondary.InventoryPath, vm.InventoryPath, err)
	}
	if err := ftWaitTask(ctx, client, resp.Returnval); err != nil {
		return "", fmt.Errorf("enable-secondary-vm task failed for %s: %w", secondary.InventoryPath, err)
	}

	result := map[string]interface{}{"vm": vm.InventoryPath, "secondary_vm": secondary.InventoryPath, "result": "secondary_enabled"}
	if host != nil {
		result["host"] = host.InventoryPath
	}
	return marshalJSON(result)
}

func handleVMMakePrimaryVM(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, err := resolveVM(ctx, client, args)
	if err != nil {
		return "", err
	}
	secondary, err := ftSecondaryVM(ctx, client, args)
	if err != nil {
		return "", err
	}

	resp, err := methods.MakePrimaryVM_Task(ctx, client.Client.Client, &types.MakePrimaryVM_Task{
		This: vm.Reference(),
		Vm:   secondary.Reference(),
	})
	if err != nil {
		return "", fmt.Errorf("failed to make %s the primary VM (was secondary of %s): %w", secondary.InventoryPath, vm.InventoryPath, err)
	}
	if err := ftWaitTask(ctx, client, resp.Returnval); err != nil {
		return "", fmt.Errorf("make-primary-vm task failed for %s: %w", secondary.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"vm": vm.InventoryPath, "secondary_vm": secondary.InventoryPath, "result": "made_primary"})
}

func handleVMTerminateFaultTolerantVM(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, err := resolveVM(ctx, client, args)
	if err != nil {
		return "", err
	}

	req := &types.TerminateFaultTolerantVM_Task{This: vm.Reference()}
	var secondaryPath string
	if name, ok := args["secondary_vm"].(string); ok && name != "" {
		secondary, err := resolveVM(ctx, client, map[string]interface{}{"vm": name})
		if err != nil {
			return "", err
		}
		ref := secondary.Reference()
		req.Vm = &ref
		secondaryPath = secondary.InventoryPath
	}

	resp, err := methods.TerminateFaultTolerantVM_Task(ctx, client.Client.Client, req)
	if err != nil {
		return "", fmt.Errorf("failed to terminate fault tolerant VM(s) for %s: %w", vm.InventoryPath, err)
	}
	if err := ftWaitTask(ctx, client, resp.Returnval); err != nil {
		return "", fmt.Errorf("terminate-fault-tolerant-vm task failed for %s: %w", vm.InventoryPath, err)
	}

	result := map[string]interface{}{"vm": vm.InventoryPath, "result": "fault_tolerant_vm_terminated"}
	if secondaryPath != "" {
		result["secondary_vm"] = secondaryPath
	} else {
		result["secondary_vm"] = "all"
	}
	return marshalJSON(result)
}

func handleVMTurnOffFaultToleranceForVM(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, err := resolveVM(ctx, client, args)
	if err != nil {
		return "", err
	}

	resp, err := methods.TurnOffFaultToleranceForVM_Task(ctx, client.Client.Client, &types.TurnOffFaultToleranceForVM_Task{This: vm.Reference()})
	if err != nil {
		return "", fmt.Errorf("failed to turn off fault tolerance for %s: %w", vm.InventoryPath, err)
	}
	if err := ftWaitTask(ctx, client, resp.Returnval); err != nil {
		return "", fmt.Errorf("turn-off-fault-tolerance task failed for %s: %w", vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"vm": vm.InventoryPath, "result": "fault_tolerance_turned_off"})
}

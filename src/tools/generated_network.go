package tools

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// registerNetworkTools is Fase 5 of the codegen plan
// (".workspace/plans/MCPVMWare2026-08-10-175300-plano-cobertura-completa-api-codegen.plan.md")
// — DistributedVirtualSwitch, DistributedVirtualPortgroup, and OpaqueNetwork,
// hand-transcribed from src/gen/classification.json following the
// established generated_*.go conventions. HostNetworkSystem's 21 methods
// were already covered in Fase 3's "host network" group (host-scoped ESXi
// networking, a different receiver) — not duplicated here. Written directly
// (not delegated to a subagent) because the domain is small enough after
// curation (7 tools) that the coordination overhead of parallel subagents
// wasn't worth it, unlike the 40-70-tool domains in Fases 2-4.
//
// Curation deviations from the raw classification (human review before
// generating, same rigor as every prior fase):
//
//   - The 4 zero-arg EthernetCardBackingInfo() accessors (on Network,
//     OpaqueNetwork, DistributedVirtualSwitch, DistributedVirtualPortgroup)
//     are SKIPPED — each returns a types.BaseVirtualDeviceBackingInfo blob
//     meant to be embedded in a VirtualEthernetCard.Backing field when
//     adding a NIC device to a VM, but no tool in this project creates
//     network-adapter devices yet (vmware_vm_add_device from Fase 2 is
//     scoped to VirtualDisk/VirtualCdrom only) — same "no further tool to
//     chain to" reasoning already used to skip VirtualMachine.EnvironmentBrowser
//     (Fase 2) and Datastore.Browser (Fase 4). Revisit if a NIC-add tool is
//     ever added.
//
//   - OpaqueNetwork.Summary() is registered read-only (no tier), not the
//     AST classifier's tier2 fail-safe default — a pure zero-arg property
//     read (Properties(ctx, ref, []string{"summary"}, ...)), same pattern
//     already corrected repeatedly since the Fase 0 review (VirtualMachine.
//     BootOptions/Device/HostSystem/ResourcePool in Fase 2).
//
//   - All 6 DistributedVirtualSwitch/DistributedVirtualPortgroup methods
//     are mode=vcenter-only (confirmed already correct from Fase 0 — a DVS
//     is created/managed only through vCenter, gen/main.go's
//     vcenterOnlyFiles already lists distributed_virtual_switch.go /
//     distributed_virtual_portgroup.go). OpaqueNetwork stays
//     vsphere-general (an NSX-T opaque network reference can exist on a
//     standalone ESXi host too) — since withClass only applies one toolMode
//     per registerXTools call, this file has TWO register functions:
//     registerNetworkTools (the 6 vcenter-only tools, below) and
//     registerNetworkVSphereGeneralTools (OpaqueNetwork's 1 tool, at the
//     bottom of this file) — same split already used by Fase 2's
//     registerVMProvisioningTools / registerVMProvisioningVCenterOnlyTools.
//
//   - Resolution: both a DVS and a DVPG are resolved through the SAME
//     Finder method — find.Finder.Network(ctx, path) returns an
//     object.NetworkReference interface that could concretely be a
//     *Network, *OpaqueNetwork, *DistributedVirtualPortgroup, or
//     *DistributedVirtualSwitch (confirmed by reading find/finder.go's
//     NetworkList, which type-switches on the resolved MOR type) — so
//     resolveDVS/resolveDVPG/resolveOpaqueNetworkRef below share one
//     resolveNetworkRef helper and just type-assert with a clear error if
//     the name resolves to the wrong kind (e.g. calling a DVS-reconfigure
//     tool against a plain portgroup name).
//
//   - vcsim support confirmed by the real SOAP method name (not receiver
//     name), same discipline as every prior fase: ReconfigureDvs_Task,
//     AddDVPortgroup_Task, FetchDVPorts, ReconfigureDVPortgroup_Task all
//     have real simulator handlers (simulator/dvs.go, simulator/portgroup.go).
//     ReconfigureDVPort_Task and UpdateDVSLacpGroupConfig_Task (backing
//     ReconfigureDVPort/ReconfigureLACP) have none — registered per real
//     vSphere support, tested only for registration/arg-validation/tier
//     gating, same "vcsim gap, not a bug" pattern as prior fases.
func registerNetworkTools(r *Registry) {
	networkArg := map[string]interface{}{
		"type":        "string",
		"description": `Network identifier: a name/pattern (e.g. "DVS0") or a full inventory path, as returned by vmware_list_networks. Must resolve to exactly one network.`,
	}
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}

	r.registerDestructive("vmware_dvpg_reconfigure",
		"Reconfigure a Distributed Virtual Portgroup (VLAN, teaming, security policy, port count, etc.).",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"network": networkArg,
				"spec":    map[string]interface{}{"type": "object", "description": "A types.DVPortgroupConfigSpec as a JSON object matching its Go struct fields."},
				"confirm": confirmArg,
			},
			"required": []interface{}{"network", "spec", "confirm"},
		},
		Tool{Handler: handleDVPGReconfigure},
	)

	r.registerDestructive("vmware_dvs_add_portgroup",
		"Add one or more portgroups to a Distributed Virtual Switch.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"network": networkArg,
				"spec":    map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "object"}, "description": "Array of types.DVPortgroupConfigSpec JSON objects."},
				"confirm": confirmArg,
			},
			"required": []interface{}{"network", "spec", "confirm"},
		},
		Tool{Handler: handleDVSAddPortgroup},
	)

	r.register("vmware_dvs_fetch_dvports",
		"List the ports of a Distributed Virtual Switch, optionally filtered by criteria.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"network":  networkArg,
				"criteria": map[string]interface{}{"type": "object", "description": "Optional types.DistributedVirtualSwitchPortCriteria JSON object to filter results. Omit to fetch every port."},
			},
			"required": []interface{}{"network"},
		},
		Tool{Handler: handleDVSFetchDVPorts},
	)

	r.registerDestructive("vmware_dvs_reconfigure",
		"Reconfigure a Distributed Virtual Switch (uplinks, LACP groups, network I/O control, etc.). Accepts a types.VMwareDVSConfigSpec — the standard config spec shape for a VMware DVS (not the generic vendor-neutral types.DVSConfigSpec base).",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"network": networkArg,
				"spec":    map[string]interface{}{"type": "object", "description": "A types.VMwareDVSConfigSpec as a JSON object matching its Go struct fields."},
				"confirm": confirmArg,
			},
			"required": []interface{}{"network", "spec", "confirm"},
		},
		Tool{Handler: handleDVSReconfigure},
	)

	r.registerDestructive("vmware_dvs_reconfigure_dvport",
		"Reconfigure one or more individual ports on a Distributed Virtual Switch. Not implemented by vcsim (registration/validation only against the simulator) — real vSphere supports it.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"network": networkArg,
				"spec":    map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "object"}, "description": "Array of types.DVPortConfigSpec JSON objects."},
				"confirm": confirmArg,
			},
			"required": []interface{}{"network", "spec", "confirm"},
		},
		Tool{Handler: handleDVSReconfigureDVPort},
	)

	r.registerDestructive("vmware_dvs_reconfigure_lacp",
		"Reconfigure LACP (link aggregation) groups on a Distributed Virtual Switch. Not implemented by vcsim (registration/validation only against the simulator) — real vSphere supports it.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"network": networkArg,
				"spec":    map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "object"}, "description": "Array of types.VMwareDvsLacpGroupSpec JSON objects."},
				"confirm": confirmArg,
			},
			"required": []interface{}{"network", "spec", "confirm"},
		},
		Tool{Handler: handleDVSReconfigureLACP},
	)
}

// registerNetworkVSphereGeneralTools registers OpaqueNetwork's tool, which —
// unlike the 6 DVS/DVPG tools above — is NOT vCenter-only: an NSX-T opaque
// network reference can exist on a standalone ESXi host too. withClass only
// applies one toolMode per registerXTools call, so this genuinely needs to
// be a separate function from registerNetworkTools (same split already used
// by Fase 2's registerVMProvisioningTools / registerVMProvisioningVCenterOnlyTools
// for the same reason: one file, two modes).
func registerNetworkVSphereGeneralTools(r *Registry) {
	networkArg := map[string]interface{}{
		"type":        "string",
		"description": `Network identifier: a name/pattern (e.g. "DVS0") or a full inventory path, as returned by vmware_list_networks. Must resolve to exactly one network.`,
	}

	r.register("vmware_opaque_network_summary",
		"Get summary info (e.g. NSX-T logical switch details) for an opaque network.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"network": networkArg},
			"required":   []interface{}{"network"},
		},
		Tool{Handler: handleOpaqueNetworkSummary},
	)
}

// resolveNetworkRef resolves the required "network" argument via the
// Finder's single network-resolution entry point, scoped with dcScopedPath
// like every other resolve* helper in this package.
func resolveNetworkRef(ctx context.Context, client *vmware.Client, args map[string]interface{}) (object.NetworkReference, error) {
	name, _ := args["network"].(string)
	if name == "" {
		return nil, fmt.Errorf("network is required")
	}
	ref, err := client.Finder.Network(ctx, dcScopedPath("network", name))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve network %q: %w", name, err)
	}
	return ref, nil
}

func resolveDVS(ctx context.Context, client *vmware.Client, args map[string]interface{}) (*object.DistributedVirtualSwitch, error) {
	ref, err := resolveNetworkRef(ctx, client, args)
	if err != nil {
		return nil, err
	}
	dvs, ok := ref.(*object.DistributedVirtualSwitch)
	if !ok {
		return nil, fmt.Errorf("network %q is a %T, not a DistributedVirtualSwitch", args["network"], ref)
	}
	return dvs, nil
}

func resolveDVPG(ctx context.Context, client *vmware.Client, args map[string]interface{}) (*object.DistributedVirtualPortgroup, error) {
	ref, err := resolveNetworkRef(ctx, client, args)
	if err != nil {
		return nil, err
	}
	dvpg, ok := ref.(*object.DistributedVirtualPortgroup)
	if !ok {
		return nil, fmt.Errorf("network %q is a %T, not a DistributedVirtualPortgroup", args["network"], ref)
	}
	return dvpg, nil
}

func resolveOpaqueNetworkRef(ctx context.Context, client *vmware.Client, args map[string]interface{}) (*object.OpaqueNetwork, error) {
	ref, err := resolveNetworkRef(ctx, client, args)
	if err != nil {
		return nil, err
	}
	on, ok := ref.(*object.OpaqueNetwork)
	if !ok {
		return nil, fmt.Errorf("network %q is a %T, not an OpaqueNetwork", args["network"], ref)
	}
	return on, nil
}

func handleDVPGReconfigure(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	dvpg, err := resolveDVPG(ctx, client, args)
	if err != nil {
		return "", err
	}
	if args["spec"] == nil {
		return "", fmt.Errorf("spec is required")
	}
	var spec types.DVPortgroupConfigSpec
	if err := decodeJSONArg(args["spec"], &spec); err != nil {
		return "", fmt.Errorf("invalid spec: %w", err)
	}

	task, err := dvpg.Reconfigure(ctx, spec)
	if err != nil {
		return "", fmt.Errorf("failed to reconfigure %s: %w", dvpg.InventoryPath, err)
	}
	if err := waitForTask(ctx, task); err != nil {
		return "", fmt.Errorf("reconfigure task failed for %s: %w", dvpg.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"network": dvpg.InventoryPath, "result": "reconfigured"})
}

func handleDVSAddPortgroup(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	dvs, err := resolveDVS(ctx, client, args)
	if err != nil {
		return "", err
	}
	raw, ok := args["spec"].([]interface{})
	if !ok || len(raw) == 0 {
		return "", fmt.Errorf("spec is required and must be a non-empty array")
	}
	specs := make([]types.DVPortgroupConfigSpec, 0, len(raw))
	for i, item := range raw {
		var spec types.DVPortgroupConfigSpec
		if err := decodeJSONArg(item, &spec); err != nil {
			return "", fmt.Errorf("invalid spec[%d]: %w", i, err)
		}
		specs = append(specs, spec)
	}

	task, err := dvs.AddPortgroup(ctx, specs)
	if err != nil {
		return "", fmt.Errorf("failed to add portgroup(s) to %s: %w", dvs.InventoryPath, err)
	}
	if err := waitForTask(ctx, task); err != nil {
		return "", fmt.Errorf("add-portgroup task failed for %s: %w", dvs.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"network": dvs.InventoryPath, "result": "portgroups_added", "count": len(specs)})
}

func handleDVSFetchDVPorts(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	dvs, err := resolveDVS(ctx, client, args)
	if err != nil {
		return "", err
	}
	var criteria *types.DistributedVirtualSwitchPortCriteria
	if args["criteria"] != nil {
		criteria = &types.DistributedVirtualSwitchPortCriteria{}
		if err := decodeJSONArg(args["criteria"], criteria); err != nil {
			return "", fmt.Errorf("invalid criteria: %w", err)
		}
	}

	ports, err := dvs.FetchDVPorts(ctx, criteria)
	if err != nil {
		return "", fmt.Errorf("failed to fetch ports for %s: %w", dvs.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"network": dvs.InventoryPath, "count": len(ports), "ports": ports})
}

func handleDVSReconfigure(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	dvs, err := resolveDVS(ctx, client, args)
	if err != nil {
		return "", err
	}
	if args["spec"] == nil {
		return "", fmt.Errorf("spec is required")
	}
	var spec types.VMwareDVSConfigSpec
	if err := decodeJSONArg(args["spec"], &spec); err != nil {
		return "", fmt.Errorf("invalid spec: %w", err)
	}

	task, err := dvs.Reconfigure(ctx, &spec)
	if err != nil {
		return "", fmt.Errorf("failed to reconfigure %s: %w", dvs.InventoryPath, err)
	}
	if err := waitForTask(ctx, task); err != nil {
		return "", fmt.Errorf("reconfigure task failed for %s: %w", dvs.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"network": dvs.InventoryPath, "result": "reconfigured"})
}

func handleDVSReconfigureDVPort(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	dvs, err := resolveDVS(ctx, client, args)
	if err != nil {
		return "", err
	}
	raw, ok := args["spec"].([]interface{})
	if !ok || len(raw) == 0 {
		return "", fmt.Errorf("spec is required and must be a non-empty array")
	}
	specs := make([]types.DVPortConfigSpec, 0, len(raw))
	for i, item := range raw {
		var spec types.DVPortConfigSpec
		if err := decodeJSONArg(item, &spec); err != nil {
			return "", fmt.Errorf("invalid spec[%d]: %w", i, err)
		}
		specs = append(specs, spec)
	}

	task, err := dvs.ReconfigureDVPort(ctx, specs)
	if err != nil {
		return "", fmt.Errorf("failed to reconfigure ports on %s: %w", dvs.InventoryPath, err)
	}
	if err := waitForTask(ctx, task); err != nil {
		return "", fmt.Errorf("reconfigure-dvport task failed for %s: %w", dvs.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"network": dvs.InventoryPath, "result": "ports_reconfigured", "count": len(specs)})
}

func handleDVSReconfigureLACP(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	dvs, err := resolveDVS(ctx, client, args)
	if err != nil {
		return "", err
	}
	raw, ok := args["spec"].([]interface{})
	if !ok || len(raw) == 0 {
		return "", fmt.Errorf("spec is required and must be a non-empty array")
	}
	specs := make([]types.VMwareDvsLacpGroupSpec, 0, len(raw))
	for i, item := range raw {
		var spec types.VMwareDvsLacpGroupSpec
		if err := decodeJSONArg(item, &spec); err != nil {
			return "", fmt.Errorf("invalid spec[%d]: %w", i, err)
		}
		specs = append(specs, spec)
	}

	task, err := dvs.ReconfigureLACP(ctx, specs)
	if err != nil {
		return "", fmt.Errorf("failed to reconfigure LACP on %s: %w", dvs.InventoryPath, err)
	}
	if err := waitForTask(ctx, task); err != nil {
		return "", fmt.Errorf("reconfigure-lacp task failed for %s: %w", dvs.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"network": dvs.InventoryPath, "result": "lacp_reconfigured", "count": len(specs)})
}

func handleOpaqueNetworkSummary(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	on, err := resolveOpaqueNetworkRef(ctx, client, args)
	if err != nil {
		return "", err
	}

	summary, err := on.Summary(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to read summary for %s: %w", on.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"network": on.InventoryPath, "summary": summary})
}

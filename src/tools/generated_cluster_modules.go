// Package tools — generated_cluster_modules.go is Fase 8a (Wave 2, group
// CS-CRYPTO) of the codegen plan
// (".workspace/plans/MCPVMWare2026-08-10-175300-plano-cobertura-completa-api-codegen.plan.md"),
// covering govmomi's vapi/cluster package (referencia/govmomi/vapi/cluster/cluster.go)
// — vSphere DRS "cluster modules", used to group VMs for advanced
// affinity/anti-affinity rules. 6 tools total.
//
// Architecture: same vapi/*-over-*rest.Client family as generated_tags.go —
// see generated_esx_settings_cluster_vms.go's top doc comment for the full
// rationale (native json tags, decodeJSONArg reuse policy, etc.), not
// repeated here.
//
// mode=vcenter-only: the entire vapi/* domain requires a vCenter Server
// Appliance (VAMI/VAPI session) — see client.REST's doc comment.
//
// vcsim gap, not a bug: confirmed directly that
// referencia/govmomi/vapi/simulator/simulator.go does not import this
// package's simulator sibling (grep -cE
// "vapi/(esx/settings|cluster\"|crypto|cis/tasks)" against that file returns
// 0), even though a standalone vapi/cluster/simulator package DOES exist
// upstream (used by govmomi's own cluster_test.go via a blank import this
// project's testhelpers_test.go does not perform) — every call from this
// project's vcsim-backed tests reaches a real, unhandled REST endpoint and
// gets back a genuine HTTP-level error, not a wiring bug. Tests use
// assertReachesServer (generated_vm_lifecycle_test.go) for exactly this
// reason.
//
// Entity resolution:
//
//   - CreateModule's "ref mo.Reference" parameter (the cluster to create the
//     module in) is exposed here as "cluster_path", resolved via
//     resolveEntityRef (generated_authorization.go, Fase 7) — its returned
//     types.ManagedObjectReference value already satisfies the mo.Reference
//     interface (confirmed in generated_tags.go's own doc comment,
//     vim25/types/helpers.go), so no extra adapter is needed for this single-
//     ref case.
//
//   - AddModuleMembers/RemoveModuleMembers's variadic "vms ...mo.Reference"
//     parameter is exposed here as "vm_paths" ([]string inventory paths),
//     resolved via resolveEntityRefs then adapted with toMoRefs
//     (generated_tags.go, Fase 8a) — reused as-is, not reimplemented, same
//     "written together in this batch, define once" discipline documented in
//     generated_authorization.go.
//
//   - ListModuleMembers's []types.ManagedObjectReference result is enriched
//     with a best-effort inventory_path per entry via tagRefInfoList
//     (generated_tags.go) — same degrade-to-null-on-failure behavior
//     documented there (e.g. a member VM deleted since being added to the
//     module).
//
// Curation:
//
//   - RemoveModuleMembers is classified tier1 here (per the codegen brief,
//     unchanged) even though AddModuleMembers — its exact structural
//     opposite — is tier2. This mirrors a known, previously-reviewed
//     inconsistency already called out in generated_tags.go's own top doc
//     comment for DetachTag/DetachMultipleTagsFromObject vs. AttachTag*
//     (tier1 vs tier2 for reversible opposite actions) — left as-is per
//     explicit instruction from the orchestrator's brief, not independently
//     "fixed" here.
//
//   - AddModuleMembers/RemoveModuleMembers return a single bool ("all
//     members were added/removed") from the *whole* call, not a per-VM
//     breakdown — that is the real API shape (confirmed by reading
//     cluster.go's moduleMembers helper and its ModuleMembers/Status
//     internal wire types), not a simplification made here.
package tools

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/vapi/cluster"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

func registerClusterModulesTools(r *Registry) {
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}
	moduleIDArg := map[string]interface{}{
		"type":        "string",
		"description": "Cluster module ID, as returned by vmware_cluster_create_module / vmware_cluster_list_modules.",
	}
	vmPathsArg := map[string]interface{}{
		"type":        "array",
		"items":       map[string]interface{}{"type": "string"},
		"description": "One or more full inventory paths of VirtualMachine objects (e.g. \"/DC0/vm/my-vm\"), each resolved via SearchIndex.FindByInventoryPath. Must be non-empty. All VMs must belong to the same vCenter cluster as the module.",
	}

	r.registerDestructive("vmware_cluster_add_module_members",
		"Add one or more virtual machines to a cluster module (used for DRS VM-VM anti-affinity). All VMs must already be in the module's own vCenter cluster. Returns true if every VM was newly added, false if any VM was already a member or not within the module's cluster (not an error in that case).",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"module_id": moduleIDArg,
				"vm_paths":  vmPathsArg,
				"confirm":   confirmArg,
			},
			"required": []interface{}{"module_id", "vm_paths", "confirm"},
		},
		Tool{Handler: handleClusterAddModuleMembers},
	)

	r.registerDestructive("vmware_cluster_create_module",
		"Create a new (empty) cluster module for the given vCenter cluster.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"cluster_path": map[string]interface{}{"type": "string", "description": `Full inventory path of the target ClusterComputeResource (e.g. "/DC0/host/cluster1"), resolved via SearchIndex.FindByInventoryPath.`},
				"confirm":      confirmArg,
			},
			"required": []interface{}{"cluster_path", "confirm"},
		},
		Tool{Handler: handleClusterCreateModule},
	)

	r.registerDestructive("vmware_cluster_delete_module",
		"Delete a cluster module. Irreversible — its VM-VM anti-affinity relation is lost; re-create the module and re-add members if needed.",
		tier1,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"module_id": moduleIDArg, "confirm": confirmArg},
			"required":   []interface{}{"module_id", "confirm"},
		},
		Tool{Handler: handleClusterDeleteModule},
	)

	r.register("vmware_cluster_list_module_members",
		"List the virtual machines that are members of a cluster module, each enriched with a best-effort inventory path.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"module_id": moduleIDArg},
			"required":   []interface{}{"module_id"},
		},
		Tool{Handler: handleClusterListModuleMembers},
	)

	r.register("vmware_cluster_list_modules",
		"List every cluster module available on this vCenter server, with the vCenter cluster each belongs to.",
		map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
		Tool{Handler: handleClusterListModules},
	)

	r.registerDestructive("vmware_cluster_remove_module_members",
		"Remove one or more virtual machines from a cluster module. Returns true if every VM was removed, false if any VM was not a member (not an error in that case). Classified tier1 (not tier2, unlike its structural opposite vmware_cluster_add_module_members) per this project's existing tier classification — see this file's top doc comment.",
		tier1,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"module_id": moduleIDArg,
				"vm_paths":  vmPathsArg,
				"confirm":   confirmArg,
			},
			"required": []interface{}{"module_id", "vm_paths", "confirm"},
		},
		Tool{Handler: handleClusterRemoveModuleMembers},
	)
}

// clusterModulesManager returns a cluster.Manager bound to client's VAPI/
// REST session, logging in lazily via client.REST — same pattern as
// generated_tags.go's tagsManager.
func clusterModulesManager(ctx context.Context, client *vmware.Client) (*cluster.Manager, error) {
	rc, err := client.REST(ctx)
	if err != nil {
		return nil, err
	}
	return cluster.NewManager(rc), nil
}

func handleClusterAddModuleMembers(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := clusterModulesManager(ctx, client)
	if err != nil {
		return "", err
	}
	moduleID, _ := args["module_id"].(string)
	if moduleID == "" {
		return "", fmt.Errorf("module_id is required")
	}
	paths, err := toStringSlice(args["vm_paths"])
	if err != nil {
		return "", fmt.Errorf("invalid vm_paths: %w", err)
	}
	refs, err := resolveEntityRefs(ctx, client, paths)
	if err != nil {
		return "", err
	}

	allAdded, err := m.AddModuleMembers(ctx, moduleID, toMoRefs(refs)...)
	if err != nil {
		return "", fmt.Errorf("failed to add members to module %q: %w", moduleID, err)
	}
	return marshalJSON(map[string]interface{}{"result": "members_add_requested", "module_id": moduleID, "vm_paths": paths, "all_added": allAdded})
}

func handleClusterCreateModule(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := clusterModulesManager(ctx, client)
	if err != nil {
		return "", err
	}
	clusterPath, _ := args["cluster_path"].(string)
	ref, err := resolveEntityRef(ctx, client, clusterPath)
	if err != nil {
		return "", err
	}

	id, err := m.CreateModule(ctx, ref)
	if err != nil {
		return "", fmt.Errorf("failed to create module on %s: %w", clusterPath, err)
	}
	return marshalJSON(map[string]interface{}{"result": "module_created", "cluster_path": clusterPath, "module_id": id})
}

func handleClusterDeleteModule(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := clusterModulesManager(ctx, client)
	if err != nil {
		return "", err
	}
	moduleID, _ := args["module_id"].(string)
	if moduleID == "" {
		return "", fmt.Errorf("module_id is required")
	}

	if err := m.DeleteModule(ctx, moduleID); err != nil {
		return "", fmt.Errorf("failed to delete module %q: %w", moduleID, err)
	}
	return marshalJSON(map[string]interface{}{"result": "module_deleted", "module_id": moduleID})
}

func handleClusterListModuleMembers(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := clusterModulesManager(ctx, client)
	if err != nil {
		return "", err
	}
	moduleID, _ := args["module_id"].(string)
	if moduleID == "" {
		return "", fmt.Errorf("module_id is required")
	}

	refs, err := m.ListModuleMembers(ctx, moduleID)
	if err != nil {
		return "", fmt.Errorf("failed to list members of module %q: %w", moduleID, err)
	}
	return marshalJSON(map[string]interface{}{"module_id": moduleID, "count": len(refs), "members": tagRefInfoList(ctx, client, toMoRefs(refs))})
}

func handleClusterListModules(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := clusterModulesManager(ctx, client)
	if err != nil {
		return "", err
	}

	modules, err := m.ListModules(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to list modules: %w", err)
	}
	return marshalJSON(map[string]interface{}{"count": len(modules), "modules": modules})
}

func handleClusterRemoveModuleMembers(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := clusterModulesManager(ctx, client)
	if err != nil {
		return "", err
	}
	moduleID, _ := args["module_id"].(string)
	if moduleID == "" {
		return "", fmt.Errorf("module_id is required")
	}
	paths, err := toStringSlice(args["vm_paths"])
	if err != nil {
		return "", fmt.Errorf("invalid vm_paths: %w", err)
	}
	refs, err := resolveEntityRefs(ctx, client, paths)
	if err != nil {
		return "", err
	}

	allRemoved, err := m.RemoveModuleMembers(ctx, moduleID, toMoRefs(refs)...)
	if err != nil {
		return "", fmt.Errorf("failed to remove members from module %q: %w", moduleID, err)
	}
	return marshalJSON(map[string]interface{}{"result": "members_remove_requested", "module_id": moduleID, "vm_paths": paths, "all_removed": allRemoved})
}

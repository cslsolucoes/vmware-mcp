package tools

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// registerClusterTools adds the remaining ClusterComputeResource managed-
// object methods (DRS/HA recommendations, HCI quickstart workflow, and two
// host/cluster-membership methods) that have no tool anywhere else in this
// project — confirmed by grepping every generated_*.go/*.go file in this
// package for `"vmware_cluster` before writing this one (already covered
// elsewhere: vmware_cluster_add_host, vmware_cluster_configuration,
// vmware_cluster_move_into, vmware_cluster_place_vm in
// generated_inventory_compute.go, plus the vmware_cluster_*_module* group in
// generated_cluster_modules.go — none of those are touched or duplicated
// here).
//
// Candidate-method vetting (every name re-verified against
// $(go env GOMODCACHE)/github.com/vmware/govmomi@v0.55.1/vim25/{types,methods}
// directly, not trusted from a brief — two of the brief's candidates were
// REJECTED after this check):
//
//   - ReconfigureComputeResource_Task: REJECTED — its own request type's doc
//     comment ("The parameters of `ComputeResource.ReconfigureComputeResource_Task`.")
//     and object/compute_resource.go's ComputeResource.Reconfigure both
//     confirm this is already fully covered by
//     vmware_compute_resource_reconfigure in generated_inventory_compute.go
//     (cr.Reconfigure(ctx, spec, modify) calls this exact SOAP method).
//     ClusterComputeResource embeds ComputeResource by value, so the method
//     is reachable on a cluster too, but through the SAME wrapper/tool —
//     re-implementing it here as a second cluster-scoped tool would be a
//     pure duplicate.
//   - PlaceVmsXCluster: REJECTED — this is a Folder method (see
//     vmware_folder_place_vms_xcluster, already registered elsewhere in this
//     project), not a ClusterComputeResource method; grepping vim25/methods
//     for "PlaceVmsXCluster" finds no such function at all (govmomi's real
//     name for the folder-scoped method is PlaceVmsXCluster on Folder,
//     already implemented — there is no second, cluster-scoped method of
//     this name to add).
//   - MoveInto_Task: NOT re-implemented — this is the existing
//     vmware_cluster_move_into (multiple hosts, no resource-pool-preservation
//     argument: MoveIntoRequestType only has This+Host[]). MoveHostInto_Task
//     (below) is a DIFFERENT, genuinely uncovered method: it moves exactly
//     ONE host and additionally accepts an optional target ResourcePool to
//     graft the host's existing resource pool hierarchy into (ownDoc comment
//     on MoveHostIntoRequestType: "The parameters of
//     `ClusterComputeResource.MoveHostInto_Task`.") — confirmed a distinct
//     request type/doc comment from MoveIntoRequestType, not a duplicate.
//
// Confirmed ClusterComputeResource-scoped by an explicit "The parameters of
// `ClusterComputeResource.X`." doc comment directly on the request type in
// vim25/types/types.go (the same discipline generated_inventory_compute.go's
// resolveClusterComputeResource-scoped tools already used): MoveHostInto_Task,
// RecommendHostsForVm, ApplyRecommendation, CancelRecommendation,
// ClusterEnterMaintenanceMode, ConfigureHCI_Task, ExtendHCI_Task.
//
// The remaining 6 (RefreshRecommendation, RetrieveDasAdvancedRuntimeInfo,
// StampAllRulesWithUuid_Task, AbandonHciWorkflow, GetResourceUsage,
// GetSystemVMsRestrictedDatastores) lack that literal doc-comment sentence in
// this vendored copy (verified by reading every line around each request
// struct's declaration — genuinely absent, not missed), so scope was
// confirmed by the next-strongest evidence instead, checked for every one of
// the 6 individually rather than assumed as a group:
//   - RetrieveDasAdvancedRuntimeInfo's response wraps
//     BaseClusterDasAdvancedRuntimeInfo, whose concrete
//     ClusterDasAdvancedRuntimeInfo doc comment reads "Base class for
//     advanced runtime information related to the high availability service
//     for a CLUSTER" — DAS is vSphere HA, a cluster-level feature exactly
//     like the already-cluster-scoped DRS recommendation methods alongside
//     it.
//   - GetResourceUsage's response wraps ClusterResourceUsageSummary, whose
//     doc comment reads "cpu, memory and storage usage information at
//     CLUSTER level."
//   - RefreshRecommendation is grouped with (alphabetically adjacent to, same
//     "Recommendation" family as) the 3 doc-comment-confirmed
//     ClusterComputeResource methods RecommendHostsForVm/
//     ApplyRecommendation/CancelRecommendation — refreshing what those 3
//     methods report on.
//   - StampAllRulesWithUuid_Task: "rules" here are VM/host affinity rules,
//     which live in ClusterConfigInfoEx.Rule (a per-cluster config list, see
//     vmware_cluster_configuration/vmware_compute_resource_reconfigure) —
//     no other managed object in vim25 owns a "Rule" collection to stamp.
//   - AbandonHciWorkflow: grouped with ConfigureHCI_Task/ExtendHCI_Task (both
//     doc-comment-confirmed ClusterComputeResource methods) as the 3-method
//     HCI "quickstart" workflow family; "abandon" cancels an
//     in-progress Configure/Extend HCI workflow on the same cluster.
//   - GetSystemVMsRestrictedDatastores: "system VMs" is vSphere's vCLS
//     (vSphere Cluster Services) agent VM mechanism, a per-cluster feature —
//     no host/datacenter/folder-scoped equivalent exists in this API.
//
// A grep of the ENTIRE vendored govmomi tree (methods.go, types.go, every
// object/*.go, every simulator/*.go, every govc/cli/*.go) for all 13 final
// method names outside methods.go/types.go itself comes back empty: no
// object.* wrapper exists for any of them (unlike AddHost/MoveInto/PlaceVm,
// which DO have ClusterComputeResource wrapper methods reused by
// generated_inventory_compute.go), so — same "no wrapper, go straight to
// methods+types" pattern generated_vm_ft.go documents at length for VM Fault
// Tolerance — every handler below dials the raw vim25 SOAP method directly.
//
// Sync vs. Task, verified per-method by reading each RequestType/
// ResponseType pair directly (not assumed from the "_Task" suffix
// convention): MoveHostInto_Task, StampAllRulesWithUuid_Task,
// ConfigureHCI_Task, and ExtendHCI_Task all return a bare
// ManagedObjectReference (the task moref) — clusterWaitRawTask (below) wraps
// it in a client-side-only *object.Task and calls this package's existing
// waitForTask (vm.go), the same construction generated_vm_ft.go's
// ftWaitTask/generated_task.go's resolveTaskArg use for a bare task moref.
// RefreshRecommendation, RecommendHostsForVm, ApplyRecommendation,
// CancelRecommendation, RetrieveDasAdvancedRuntimeInfo,
// ClusterEnterMaintenanceMode, AbandonHciWorkflow, GetResourceUsage, and
// GetSystemVMsRestrictedDatastores are all synchronous — their Response
// structs return a concrete result (or nothing) directly, never a Task moref,
// so no waitForTask call is made for any of them.
//
// vcsim coverage: NONE of these 13 methods have a simulator-side handler —
// confirmed by grepping simulator/cluster_compute_resource.go's full receiver
// list for *ClusterComputeResource (RenameTask, AddHostTask,
// ReconfigureComputeResourceTask, MoveIntoTask, PlaceVm, plus unexported
// update*/vsanIsEnabled/addStorageHost helpers — none of the 13 names below
// appear) and the rest of the simulator tree for any of the 13 method names
// standalone. A call against vcsim always faults types.MethodNotFound
// (simulator/simulator.go's method-dispatch fallback) — same situation as
// generated_vm_ft.go's 7 FT methods. generated_cluster_test.go therefore
// drives every tool with assertReachesServer for exactly that reason: it
// proves the wiring (schema, tier gate, resolveClusterComputeResource/
// resolveHost/resolveResourcePool/resolveVM, raw SOAP dispatch) reaches
// vcsim and gets back a clean server-side fault, not an unknown-tool wiring
// bug or a recovered panic. Behavioral validation of a real DRS
// recommendation, HA runtime state, or HCI workflow is expected against a
// real vCenter-managed cluster.
//
// SDDCBase opaque passthrough (ConfigureHCI's cluster_spec.vSanConfigSpec and
// ExtendHCI's vsan_config_spec): types.SDDCBase's own doc comment reads "An
// empty data object which can be used as the base class for data objects
// outside VIM namespace which have to be proxied through vCenter opaquely.
// For example, vSan configuration spec will extend from this" — this
// vendored govmomi copy carries no concrete vSAN ReconfigSpec type (grepping
// the whole tree for "VsanReconfigSpec"/"vsan.ReconfigSpec" finds nothing;
// that type lives in a separate vSAN-specific SDK/package this project does
// not vendor). decodeJSONArg into types.SDDCBase (embeds DynamicData only)
// therefore only carries whatever "dynamicType"/"dynamicProperty" shape the
// caller supplies — this is documented on both affected tool schemas rather
// than silently accepting arbitrary vSAN JSON that would be dropped on
// re-marshal.
//
// Class: modeVCenterOnly — ClusterComputeResource is a vCenter-only
// inventory concept (gen/main.go's vcenterOnlyFiles already lists
// cluster_compute_resource.go; registerInventoryComputeVCenterOnlyTools and
// registerClusterModulesTools already register their cluster-scoped tools
// the same way). A standalone ESXi connection has no ClusterComputeResource
// at all — resolveClusterComputeResource (generated_inventory_compute.go)
// would simply fail to resolve "cluster" there, same as every existing
// cluster-scoped tool.
//
// Tier: read-only/no-tier for the 5 pure query/dry-run methods
// (RecommendHostsForVm, RetrieveDasAdvancedRuntimeInfo, GetResourceUsage,
// GetSystemVMsRestrictedDatastores, ClusterEnterMaintenanceMode — the last is
// a dry-run despite its name: ClusterEnterMaintenanceResult's own doc comment
// says "Application of the recommendations is not supported currently. The
// client will have to put the hosts into maintenance mode by calling the
// separate method `HostSystem.EnterMaintenanceMode_Task`" — i.e. the
// already-existing, separately-gated vmware_host_maintenance_enter tier2
// tool; this method itself changes nothing). Tier 2 (disruptive but
// reversible) for the remaining 8, all of which mutate cluster/host state or
// trigger real VM/host actions: vmware_cluster_move_host_into (host
// membership change, reversible via vmware_cluster_move_into/another
// move-host-into), vmware_cluster_refresh_recommendation (no config change,
// but classified alongside the cluster's other DRS-recommendation mutating
// actions per the same convention vmware_storage_refresh_drs_recommendation
// already established for Storage DRS in generated_storage_drs.go),
// vmware_cluster_apply_recommendation (triggers the underlying vMotion/host
// action; reversible the same way any migration is), vmware_cluster_cancel_
// recommendation (discards a recommendation — same tier as Storage DRS's own
// cancel counterpart), vmware_cluster_stamp_all_rules_with_uuid (Task-
// returning cluster-rule mutation), vmware_cluster_abandon_hci_workflow/
// vmware_cluster_configure_hci/vmware_cluster_extend_hci (the 3-method HCI
// quickstart workflow — all mutate cluster/host configuration).
func registerClusterTools(r *Registry) {
	clusterArg := map[string]interface{}{
		"type":        "string",
		"description": `Cluster identifier: a name/pattern (e.g. "cluster-01") or a full inventory path, as returned by vmware_list_clusters. Must resolve to exactly one cluster.`,
	}
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}

	// --- Read-only / dry-run --------------------------------------------

	r.register("vmware_cluster_recommend_hosts_for_vm",
		"Ask DRS to recommend hosts within a cluster suitable for placing/migrating a virtual machine — a read-only query, distinct from vmware_cluster_place_vm's fuller create/reconfigure/relocate/clone placement spec.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"cluster": clusterArg,
				"vm":      vmArgForCluster,
				"pool":    map[string]interface{}{"type": "string", "description": "Optional target ResourcePool name/pattern the VM would be migrated into (see vmware_list_resource_pools). Required by the underlying API when the VM is powered on; this ResourcePool cannot be in the same cluster as the VM."},
			},
			"required": []interface{}{"cluster", "vm"},
		},
		Tool{Handler: handleClusterRecommendHostsForVm},
	)

	r.register("vmware_cluster_retrieve_das_advanced_runtime_info",
		"Get advanced vSphere HA (DAS) runtime information for a cluster — internal/troubleshooting details beyond what vmware_cluster_configuration's dasConfig exposes.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"cluster": clusterArg},
			"required":   []interface{}{"cluster"},
		},
		Tool{Handler: handleClusterRetrieveDasAdvancedRuntimeInfo},
	)

	r.register("vmware_cluster_get_resource_usage",
		"Get aggregate CPU, memory, and storage usage/capacity for a cluster (types.ClusterResourceUsageSummary).",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"cluster": clusterArg},
			"required":   []interface{}{"cluster"},
		},
		Tool{Handler: handleClusterGetResourceUsage},
	)

	r.register("vmware_cluster_get_system_vms_restricted_datastores",
		"List the datastores restricted from hosting a cluster's system VMs (vSphere Cluster Services / vCLS agent VMs).",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"cluster": clusterArg},
			"required":   []interface{}{"cluster"},
		},
		Tool{Handler: handleClusterGetSystemVMsRestrictedDatastores},
	)

	r.register("vmware_cluster_enter_maintenance_mode",
		`Ask the cluster for evacuation recommendations (host maintenance actions plus any vMotions needed) for putting the given hosts into maintenance mode — a DRY-RUN query, it does NOT actually enter maintenance mode. Per the underlying API's own result-type doc comment, applying the recommendations is not supported by this call; use the already-existing vmware_host_maintenance_enter (HostSystem.EnterMaintenanceMode_Task) per host to actually do so.`,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"cluster": clusterArg,
				"hosts":   map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Host identifiers (name/pattern or full inventory path, as returned by vmware_list_hosts) to evaluate for maintenance mode."},
				"option":  map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "object"}, "description": "Optional array of types.OptionValue JSON objects ({\"key\":..., \"value\":...}) overriding advanced DRS options for this evaluation."},
				"info":    map[string]interface{}{"type": "object", "description": "Optional types.ClusterComputeResourceMaintenanceInfo JSON object (e.g. {\"partialMMId\": \"...\"}) requesting partial maintenance mode instead of full. Requires a sufficiently recent vCenter (8.0.3.0+)."},
			},
			"required": []interface{}{"cluster", "hosts"},
		},
		Tool{Handler: handleClusterEnterMaintenanceMode},
	)

	// --- Tier 2 (disruptive but reversible) ------------------------------

	r.registerDestructive("vmware_cluster_move_host_into",
		`Move exactly ONE already-registered ESXi host into a cluster, optionally grafting its existing resource pool hierarchy into a target ResourcePool (matching a stand-alone host's root resource pool structure). Distinct from vmware_cluster_move_into, which moves one or more hosts but has no resource-pool-preservation argument.`,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"cluster":       clusterArg,
				"host":          hostArgForCluster,
				"resource_pool": map[string]interface{}{"type": "string", "description": "Optional ResourcePool name/pattern (see vmware_list_resource_pools) to graft the host's existing resource pool hierarchy into. Has no effect if the host is already part of a cluster."},
				"confirm":       confirmArg,
			},
			"required": []interface{}{"cluster", "host", "confirm"},
		},
		Tool{Handler: handleClusterMoveHostInto},
	)

	r.registerDestructive("vmware_cluster_refresh_recommendation",
		"Force DRS to recompute its host/VM recommendations for a cluster right now, instead of waiting for its next scheduled interval. No configuration change, but classified alongside the cluster's other DRS-recommendation mutating actions here (same convention as vmware_storage_refresh_drs_recommendation for Storage DRS).",
		tier2,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"cluster": clusterArg, "confirm": confirmArg},
			"required":   []interface{}{"cluster", "confirm"},
		},
		Tool{Handler: handleClusterRefreshRecommendation},
	)

	r.registerDestructive("vmware_cluster_apply_recommendation",
		"Apply a single previously computed DRS recommendation by key (see vmware_cluster_recommend_hosts_for_vm or a cluster's reported recommendations), triggering the underlying host/VM action (e.g. vMotion).",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"cluster": clusterArg,
				"key":     map[string]interface{}{"type": "string", "description": "The key field of the DrsRecommendation/Recommendation to apply."},
				"confirm": confirmArg,
			},
			"required": []interface{}{"cluster", "key", "confirm"},
		},
		Tool{Handler: handleClusterApplyRecommendation},
	)

	r.registerDestructive("vmware_cluster_cancel_recommendation",
		"Discard a single pending DRS recommendation by key without applying it — it will not be re-offered and must be recomputed via vmware_cluster_refresh_recommendation if still needed.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"cluster": clusterArg,
				"key":     map[string]interface{}{"type": "string", "description": "The key field of the Recommendation to discard."},
				"confirm": confirmArg,
			},
			"required": []interface{}{"cluster", "key", "confirm"},
		},
		Tool{Handler: handleClusterCancelRecommendation},
	)

	r.registerDestructive("vmware_cluster_stamp_all_rules_with_uuid",
		"Stamp a UUID onto every VM/host affinity/anti-affinity rule in a cluster's configuration that does not already have one — an internal bookkeeping/migration operation, not a behavioral rule change.",
		tier2,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"cluster": clusterArg, "confirm": confirmArg},
			"required":   []interface{}{"cluster", "confirm"},
		},
		Tool{Handler: handleClusterStampAllRulesWithUuid},
	)

	r.registerDestructive("vmware_cluster_abandon_hci_workflow",
		"Abandon an in-progress HCI (hyper-converged infrastructure) quickstart configuration workflow on a cluster started via vmware_cluster_configure_hci/vmware_cluster_extend_hci.",
		tier2,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"cluster": clusterArg, "confirm": confirmArg},
			"required":   []interface{}{"cluster", "confirm"},
		},
		Tool{Handler: handleClusterAbandonHciWorkflow},
	)

	r.registerDestructive("vmware_cluster_configure_hci",
		"Run the HCI (hyper-converged infrastructure) quickstart workflow on a cluster: network (DVS/portgroup), host service, and vCenter/EVC configuration in one step. Hosts not already in maintenance mode are skipped unless allowed via cluster_spec/host_inputs; skipped hosts are reported in the result's failedHosts. See this file's top doc comment for cluster_spec.vSanConfigSpec's opaque-passthrough limitation (only dynamicType/dynamicProperty are transported; no concrete vSAN reconfig spec type is vendored here).",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"cluster":      clusterArg,
				"cluster_spec": map[string]interface{}{"type": "object", "description": "A types.ClusterComputeResourceHCIConfigSpec JSON object matching its Go struct fields: dvsProf (array), hostConfigProfile, vSanConfigSpec (opaque — see this tool's description), vcProf."},
				"host_inputs":  map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "object"}, "description": "Optional array of types.ClusterComputeResourceHostConfigurationInput JSON objects ({\"host\": {\"type\":\"HostSystem\",\"value\":\"...\"}, \"hostVmkNics\": [...], \"allowedInNonMaintenanceMode\": false}). Omit to operate on every host in the cluster."},
				"confirm":      confirmArg,
			},
			"required": []interface{}{"cluster", "cluster_spec", "confirm"},
		},
		Tool{Handler: handleClusterConfigureHCI},
	)

	r.registerDestructive("vmware_cluster_extend_hci",
		"Extend an already-HCI-configured cluster's network/host-service configuration to an additional set of hosts (e.g. newly added hosts), and optionally reconfigure vSAN on them. See this file's top doc comment for vsan_config_spec's opaque-passthrough limitation.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"cluster":          clusterArg,
				"host_inputs":      map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "object"}, "description": "Required non-empty array of types.ClusterComputeResourceHostConfigurationInput JSON objects — see vmware_cluster_configure_hci's host_inputs for field shape. Hosts must be in maintenance mode unless allowedInNonMaintenanceMode is set on their entry."},
				"vsan_config_spec": map[string]interface{}{"type": "object", "description": "Optional opaque vSAN reconfig spec (types.SDDCBase) — see this tool's description for the opaque-passthrough limitation."},
				"confirm":          confirmArg,
			},
			"required": []interface{}{"cluster", "host_inputs", "confirm"},
		},
		Tool{Handler: handleClusterExtendHCI},
	)
}

// vmArgForCluster/hostArgForCluster are this file's own copies of the
// "vm"/"host" argument schema description used across the package (e.g.
// vm.go's/host.go's own tool registrations) — kept local rather than
// importing a shared package-level var so this file has no compile-time
// dependency on another group's exact wording, matching every other
// generated_*.go file's convention of declaring its own arg schema literals.
var vmArgForCluster = map[string]interface{}{
	"type":        "string",
	"description": `Virtual machine identifier: a name/pattern (e.g. "web-01") or a full inventory path (e.g. "/dc1/vm/web-01") as returned by vmware_list_vms. Must resolve to exactly one VM.`,
}

var hostArgForCluster = map[string]interface{}{
	"type":        "string",
	"description": `ESXi host identifier: a name/pattern or a full inventory path, as returned by vmware_list_hosts. Must resolve to exactly one host.`,
}

// clusterWaitRawTask wraps the task moref returned by one of this file's raw
// methods.Xxx_Task calls (MoveHostInto_Task, StampAllRulesWithUuid_Task,
// ConfigureHCI_Task, ExtendHCI_Task — none of which have an
// object.ClusterComputeResource wrapper method, see this file's top doc
// comment) in a client-side-only *object.Task — object.NewTask(client.
// Client.Client, ref), no round trip until the first real call against it,
// the same construction generated_vm_ft.go's ftWaitTask and
// generated_task.go's resolveTaskArg use for a bare task moref — and blocks
// on it via this package's existing waitForTask (vm.go).
func clusterWaitRawTask(ctx context.Context, client *vmware.Client, ref types.ManagedObjectReference) error {
	return waitForTask(ctx, object.NewTask(client.Client.Client, ref))
}

func handleClusterRecommendHostsForVm(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	cluster, err := resolveClusterComputeResource(ctx, client, args)
	if err != nil {
		return "", err
	}
	vm, err := resolveVM(ctx, client, args)
	if err != nil {
		return "", err
	}

	req := &types.RecommendHostsForVm{
		This: cluster.Reference(),
		Vm:   vm.Reference(),
	}
	if name, ok := args["pool"].(string); ok && name != "" {
		pool, err := resolveResourcePool(ctx, client, name)
		if err != nil {
			return "", err
		}
		ref := pool.Reference()
		req.Pool = &ref
	}

	resp, err := methods.RecommendHostsForVm(ctx, client.Client.Client, req)
	if err != nil {
		return "", fmt.Errorf("failed to recommend hosts for vm %s in cluster %s: %w", vm.InventoryPath, cluster.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{
		"cluster":         cluster.InventoryPath,
		"vm":              vm.InventoryPath,
		"count":           len(resp.Returnval),
		"recommendations": resp.Returnval,
	})
}

func handleClusterRetrieveDasAdvancedRuntimeInfo(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	cluster, err := resolveClusterComputeResource(ctx, client, args)
	if err != nil {
		return "", err
	}

	resp, err := methods.RetrieveDasAdvancedRuntimeInfo(ctx, client.Client.Client, &types.RetrieveDasAdvancedRuntimeInfo{This: cluster.Reference()})
	if err != nil {
		return "", fmt.Errorf("failed to retrieve DAS advanced runtime info for %s: %w", cluster.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"cluster": cluster.InventoryPath, "result": resp.Returnval})
}

func handleClusterGetResourceUsage(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	cluster, err := resolveClusterComputeResource(ctx, client, args)
	if err != nil {
		return "", err
	}

	resp, err := methods.GetResourceUsage(ctx, client.Client.Client, &types.GetResourceUsage{This: cluster.Reference()})
	if err != nil {
		return "", fmt.Errorf("failed to get resource usage for %s: %w", cluster.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"cluster": cluster.InventoryPath, "usage": resp.Returnval})
}

func handleClusterGetSystemVMsRestrictedDatastores(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	cluster, err := resolveClusterComputeResource(ctx, client, args)
	if err != nil {
		return "", err
	}

	resp, err := methods.GetSystemVMsRestrictedDatastores(ctx, client.Client.Client, &types.GetSystemVMsRestrictedDatastores{This: cluster.Reference()})
	if err != nil {
		return "", fmt.Errorf("failed to get system-VM-restricted datastores for %s: %w", cluster.InventoryPath, err)
	}

	// resp.Returnval came back as bare morefs, never through a Finder — same
	// datastoreInventoryPath reuse as generated_inventory_compute.go's
	// handleComputeResourceDatastores.
	paths := make([]string, 0, len(resp.Returnval))
	for _, ref := range resp.Returnval {
		ds := object.NewDatastore(client.Client.Client, ref)
		p, err := datastoreInventoryPath(ctx, client, ds)
		if err != nil {
			return "", err
		}
		paths = append(paths, p)
	}

	return marshalJSON(map[string]interface{}{
		"cluster":    cluster.InventoryPath,
		"count":      len(paths),
		"datastores": paths,
	})
}

func handleClusterEnterMaintenanceMode(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	cluster, err := resolveClusterComputeResource(ctx, client, args)
	if err != nil {
		return "", err
	}

	rawHosts, ok := args["hosts"].([]interface{})
	if !ok || len(rawHosts) == 0 {
		return "", fmt.Errorf("hosts is required and must be a non-empty array")
	}
	hostRefs := make([]types.ManagedObjectReference, 0, len(rawHosts))
	hostPaths := make([]string, 0, len(rawHosts))
	for i, item := range rawHosts {
		name, ok := item.(string)
		if !ok || name == "" {
			return "", fmt.Errorf("hosts[%d] must be a non-empty string", i)
		}
		h, err := resolveHost(ctx, client, map[string]interface{}{"host": name})
		if err != nil {
			return "", err
		}
		hostRefs = append(hostRefs, h.Reference())
		hostPaths = append(hostPaths, h.InventoryPath)
	}

	req := &types.ClusterEnterMaintenanceMode{
		This: cluster.Reference(),
		Host: hostRefs,
	}

	// Same "[]interface{} of raw option objects -> []types.BaseOptionValue"
	// pattern generated_inventory_folder.go's handleDatacenterPowerOnVM
	// already uses for Datacenter.PowerOnVM's own optional "option" argument.
	if raw, ok := args["option"]; ok && raw != nil {
		arr, ok := raw.([]interface{})
		if !ok {
			return "", fmt.Errorf("option must be an array")
		}
		opts := make([]types.BaseOptionValue, 0, len(arr))
		for i, item := range arr {
			var ov types.OptionValue
			if err := decodeJSONArg(item, &ov); err != nil {
				return "", fmt.Errorf("invalid option[%d]: %w", i, err)
			}
			opts = append(opts, &ov)
		}
		req.Option = opts
	}

	if raw, ok := args["info"]; ok && raw != nil {
		var info types.ClusterComputeResourceMaintenanceInfo
		if err := decodeJSONArg(raw, &info); err != nil {
			return "", fmt.Errorf("invalid info: %w", err)
		}
		req.Info = &info
	}

	resp, err := methods.ClusterEnterMaintenanceMode(ctx, client.Client.Client, req)
	if err != nil {
		return "", fmt.Errorf("failed to compute maintenance-mode recommendations for %s: %w", cluster.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{
		"cluster": cluster.InventoryPath,
		"hosts":   hostPaths,
		"result":  resp.Returnval,
	})
}

func handleClusterMoveHostInto(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	cluster, err := resolveClusterComputeResource(ctx, client, args)
	if err != nil {
		return "", err
	}
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}

	req := &types.MoveHostInto_Task{
		This: cluster.Reference(),
		Host: host.Reference(),
	}
	if name, ok := args["resource_pool"].(string); ok && name != "" {
		pool, err := resolveResourcePool(ctx, client, name)
		if err != nil {
			return "", err
		}
		ref := pool.Reference()
		req.ResourcePool = &ref
	}

	resp, err := methods.MoveHostInto_Task(ctx, client.Client.Client, req)
	if err != nil {
		return "", fmt.Errorf("failed to move host %s into cluster %s: %w", host.InventoryPath, cluster.InventoryPath, err)
	}
	if err := clusterWaitRawTask(ctx, client, resp.Returnval); err != nil {
		return "", fmt.Errorf("move-host-into task failed for %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"cluster": cluster.InventoryPath, "host": host.InventoryPath, "result": "host_moved"})
}

func handleClusterRefreshRecommendation(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	cluster, err := resolveClusterComputeResource(ctx, client, args)
	if err != nil {
		return "", err
	}

	if _, err := methods.RefreshRecommendation(ctx, client.Client.Client, &types.RefreshRecommendation{This: cluster.Reference()}); err != nil {
		return "", fmt.Errorf("failed to refresh recommendations for %s: %w", cluster.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"cluster": cluster.InventoryPath, "result": "recommendations_refreshed"})
}

func handleClusterApplyRecommendation(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	cluster, err := resolveClusterComputeResource(ctx, client, args)
	if err != nil {
		return "", err
	}
	key, _ := args["key"].(string)
	if key == "" {
		return "", fmt.Errorf("key is required")
	}

	if _, err := methods.ApplyRecommendation(ctx, client.Client.Client, &types.ApplyRecommendation{This: cluster.Reference(), Key: key}); err != nil {
		return "", fmt.Errorf("failed to apply recommendation %s for cluster %s: %w", key, cluster.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"cluster": cluster.InventoryPath, "key": key, "result": "recommendation_applied"})
}

func handleClusterCancelRecommendation(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	cluster, err := resolveClusterComputeResource(ctx, client, args)
	if err != nil {
		return "", err
	}
	key, _ := args["key"].(string)
	if key == "" {
		return "", fmt.Errorf("key is required")
	}

	if _, err := methods.CancelRecommendation(ctx, client.Client.Client, &types.CancelRecommendation{This: cluster.Reference(), Key: key}); err != nil {
		return "", fmt.Errorf("failed to cancel recommendation %s for cluster %s: %w", key, cluster.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"cluster": cluster.InventoryPath, "key": key, "result": "recommendation_cancelled"})
}

func handleClusterStampAllRulesWithUuid(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	cluster, err := resolveClusterComputeResource(ctx, client, args)
	if err != nil {
		return "", err
	}

	resp, err := methods.StampAllRulesWithUuid_Task(ctx, client.Client.Client, &types.StampAllRulesWithUuid_Task{This: cluster.Reference()})
	if err != nil {
		return "", fmt.Errorf("failed to stamp rules with uuid for %s: %w", cluster.InventoryPath, err)
	}
	if err := clusterWaitRawTask(ctx, client, resp.Returnval); err != nil {
		return "", fmt.Errorf("stamp-all-rules-with-uuid task failed for %s: %w", cluster.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"cluster": cluster.InventoryPath, "result": "rules_stamped"})
}

func handleClusterAbandonHciWorkflow(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	cluster, err := resolveClusterComputeResource(ctx, client, args)
	if err != nil {
		return "", err
	}

	if _, err := methods.AbandonHciWorkflow(ctx, client.Client.Client, &types.AbandonHciWorkflow{This: cluster.Reference()}); err != nil {
		return "", fmt.Errorf("failed to abandon HCI workflow for %s: %w", cluster.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"cluster": cluster.InventoryPath, "result": "hci_workflow_abandoned"})
}

func handleClusterConfigureHCI(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	cluster, err := resolveClusterComputeResource(ctx, client, args)
	if err != nil {
		return "", err
	}
	if args["cluster_spec"] == nil {
		return "", fmt.Errorf("cluster_spec is required")
	}
	var spec types.ClusterComputeResourceHCIConfigSpec
	if err := decodeJSONArg(args["cluster_spec"], &spec); err != nil {
		return "", fmt.Errorf("invalid cluster_spec: %w", err)
	}

	req := &types.ConfigureHCI_Task{
		This:        cluster.Reference(),
		ClusterSpec: spec,
	}
	if raw, ok := args["host_inputs"]; ok && raw != nil {
		var inputs []types.ClusterComputeResourceHostConfigurationInput
		if err := decodeJSONArg(raw, &inputs); err != nil {
			return "", fmt.Errorf("invalid host_inputs: %w", err)
		}
		req.HostInputs = inputs
	}

	resp, err := methods.ConfigureHCI_Task(ctx, client.Client.Client, req)
	if err != nil {
		return "", fmt.Errorf("failed to configure HCI for %s: %w", cluster.InventoryPath, err)
	}
	if err := clusterWaitRawTask(ctx, client, resp.Returnval); err != nil {
		return "", fmt.Errorf("configure-hci task failed for %s: %w", cluster.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"cluster": cluster.InventoryPath, "result": "hci_configured"})
}

func handleClusterExtendHCI(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	cluster, err := resolveClusterComputeResource(ctx, client, args)
	if err != nil {
		return "", err
	}
	rawInputs, ok := args["host_inputs"].([]interface{})
	if !ok || len(rawInputs) == 0 {
		return "", fmt.Errorf("host_inputs is required and must be a non-empty array")
	}
	var inputs []types.ClusterComputeResourceHostConfigurationInput
	if err := decodeJSONArg(rawInputs, &inputs); err != nil {
		return "", fmt.Errorf("invalid host_inputs: %w", err)
	}

	req := &types.ExtendHCI_Task{
		This:       cluster.Reference(),
		HostInputs: inputs,
	}
	if raw, ok := args["vsan_config_spec"]; ok && raw != nil {
		var spec types.SDDCBase
		if err := decodeJSONArg(raw, &spec); err != nil {
			return "", fmt.Errorf("invalid vsan_config_spec: %w", err)
		}
		req.VSanConfigSpec = &spec
	}

	resp, err := methods.ExtendHCI_Task(ctx, client.Client.Client, req)
	if err != nil {
		return "", fmt.Errorf("failed to extend HCI for %s: %w", cluster.InventoryPath, err)
	}
	if err := clusterWaitRawTask(ctx, client, resp.Returnval); err != nil {
		return "", fmt.Errorf("extend-hci task failed for %s: %w", cluster.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"cluster": cluster.InventoryPath, "result": "hci_extended"})
}

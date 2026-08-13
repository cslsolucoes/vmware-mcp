package tools

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// registerStorageDrsTools is the "storage DRS" slice of Fase 4 of the
// codegen plan
// (".workspace/plans/MCPVMWare2026-08-10-175300-plano-cobertura-completa-api-codegen.plan.md")
// — all 9 methods of object.StorageResourceManager, hand-transcribed from
// referencia/govmomi/object/storage_resource_manager.go following the
// host.go/datastore.go/generated_host_storage.go/generated_vm_lifecycle.go
// conventions.
//
// Mode correction (already applied, not re-litigated here): an earlier pass
// of this project's generator classified StorageResourceManager as
// vcenter-only, reasoning that "Storage DRS only exists on vCenter" implies
// the manager object itself doesn't exist on standalone ESXi. That premise
// was wrong — object.NewStorageResourceManager dereferences
// c.ServiceContent.StorageResourceManager unconditionally, and that field is
// non-nil in BOTH referencia/govmomi/simulator/esx/service_content.go and
// vpx/service_content.go, confirmed by reading both files directly. All 9
// tools below are registered modeVSphereGeneral. The Storage DRS FEATURE
// (real recommendations) only ever does anything useful when a real
// datastore cluster/StoragePod managed by vCenter exists — that is a
// runtime/topology fact, not a connection-mode fact, and surfaces as a
// clean fault or an empty/no-op result against standalone ESXi, not a
// wrong-mode error.
//
// Curation deviations from this group's brief (human review required):
//
//   - vmware_storage_recommend_datastores is registered READ-ONLY
//     (r.register, no tier), NOT Tier 2 as the brief listed it. Confirmed
//     by reading the real method: RecommendDatastores makes zero server-side
//     mutations — it returns a types.StoragePlacementResult whose own doc
//     comment says its Recommendations "list... that the client needs to
//     approve manually" (referencia/govmomi/vim25/types/types.go, the
//     comment directly above StoragePlacementResult). Nothing is actually
//     moved/reconfigured until a caller separately calls
//     vmware_storage_apply_drs_recommendation(_to_pod) with one of the
//     returned recommendation keys. This mirrors this project's existing
//     precedent in generated_host_storage.go, where
//     vmware_host_storage_compute_disk_partition_info was downgraded from
//     the brief's suggested Tier 2 to read-only for the identical reason
//     (propose vs. apply — the "apply" counterpart is what's actually
//     destructive, and its tool already carries the gate). Treating a pure
//     computation as destructive would be a false positive that trains
//     callers to expect confirm:true gating on a query.
//
//   - ConfigureDatastoreIORM's "key" parameter is a dead argument in
//     govmomi's own Go wrapper: reading the real method body
//     (object.StorageResourceManager.ConfigureDatastoreIORM) shows key is
//     accepted but never copied into types.ConfigureDatastoreIORM_Task's
//     request fields (types.ConfigureDatastoreIORMRequestType has only
//     Datastore and Spec — no Key field at all, confirmed directly in
//     vim25/types/types.go). vmware_storage_configure_datastore_iorm still
//     accepts an optional "key" argument for Go-signature fidelity with the
//     rest of this project's convention, but its schema description says
//     plainly that it is accepted and silently ignored — better to be
//     honest about a real upstream quirk than hide a parameter a caller
//     might reasonably expect to matter.
//
//   - vcsim has NO server-side implementation for 7 of these 9 methods —
//     confirmed by grepping every method name across the entire
//     referencia/govmomi/simulator tree (not assumed from the brief):
//     simulator/storage_resource_manager.go implements only
//     ConfigureStorageDrsForPodTask (backs ConfigureStorageDrsForPod) and
//     RecommendDatastores for real. ApplyStorageDrsRecommendation(ToPod),
//     CancelStorageDrsRecommendation, ConfigureDatastoreIORM,
//     QueryDatastorePerformanceSummary, QueryIORMConfigOption, and
//     RefreshStorageDrsRecommendation have no matching receiver method
//     anywhere in the simulator package (the 3 hits for those name
//     fragments outside storage_resource_manager.go are task-description
//     string keys in simulator/vpx/task_manager.go and
//     simulator/esx/task_manager.go, not method implementations — verified
//     by reading those hits directly). A call against vcsim for those 7
//     always faults types.MethodNotFound (simulator/simulator.go's
//     method-dispatch fallback — same mechanism documented in
//     generated_vm_lifecycle.go's and generated_host_storage.go's top
//     comments). Every tool below is still registered exactly as real
//     vSphere supports it — a vcsim simulation gap is not a reason to omit
//     a tool real deployments can use.
//     generated_storage_drs_test.go uses the existing assertReachesServer
//     helper (generated_vm_lifecycle_test.go, same package) for those 7: it
//     proves the plumbing (schema, tier gating, resolveHost/resolveDatastore/
//     resolveStoragePod, decodeJSONArg) reaches vcsim's real method
//     dispatch and gets back a clean MethodNotFound-based error, not a
//     wiring bug (unknown tool) or a recovered panic.
//
//   - ConfigureStorageDrsForPod and RecommendDatastores — the 2 methods
//     vcsim DOES implement server-side — get real, driven-to-success
//     functional tests, built on a real simulator.VPX() StoragePod fixture
//     (folders.DatastoreFolder.CreateStoragePod, then pod.MoveInto to add a
//     real datastore as a child — both real object-layer calls, backed by
//     real simulator/folder.go and simulator/storage_pod.go handlers, not
//     mocked). Getting RecommendDatastores to a real non-empty
//     recommendation list turned out NOT to require
//     VirtualMachineConfigSpec.DeviceChange (a polymorphic
//     []BaseVirtualDeviceConfigSpec field) at all: reading
//     simulator.StorageResourceManager.RecommendDatastores's real body shows
//     the per-placement disk/device-key cross-check
//     (devices.FindByKey(disk.DiskId)) only runs when
//     VmPodConfigForPlacement.Disk is non-empty — an empty Disk list still
//     produces one recommendation per datastore in the pod, with zero
//     polymorphic fields anywhere in the request. That sidesteps a real
//     limitation confirmed by grep: vim25/types has no custom
//     UnmarshalJSON anywhere, so a genuinely polymorphic field (Base* device
//     types) submitted as a plain JSON object via this tool's generic
//     decodeJSONArg would fail to decode (encoding/json cannot populate a
//     named, non-empty interface field without one) — this is the same MVP
//     limitation the brief flagged for storageSpec in general (callers need
//     to know the underlying Go field names), just sharper for any spec
//     that also needs a DeviceChange. Documented here, not worked around,
//     consistent with this project's accepted posture for
//     generated_vm_provisioning.go's CustomizationSpec.
//
//   - resolveStoragePod (below) scopes "pod" name/pattern arguments with
//     dcScopedPath("datastore", name) before calling
//     client.Finder.DatastoreCluster — StoragePod objects live in the same
//     per-datacenter "datastore" Finder folder as Datastore objects
//     (confirmed by reading find.Finder.DatastoreClusterList's Relative:
//     f.datastoreFolder), so this reuses the exact same multi-datacenter
//     scoping reasoning as resolveDatastore (datastore.go) and resolveHost
//     (host.go) rather than inventing a new one.
//
//   - Tier assignments otherwise follow the brief exactly: Tier 2
//     (disruptive but reversible in the sense that Storage DRS config/task
//     state can be changed again, not that data is destroyed) for
//     ApplyStorageDrsRecommendation, ApplyStorageDrsRecommendationToPod,
//     CancelStorageDrsRecommendation, ConfigureDatastoreIORM,
//     ConfigureStorageDrsForPod, and RefreshStorageDrsRecommendation;
//     read-only for QueryDatastorePerformanceSummary and
//     QueryIORMConfigOption (plus RecommendDatastores per the deviation
//     above).
func registerStorageDrsTools(r *Registry) {
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}
	hostArg := map[string]interface{}{
		"type":        "string",
		"description": `Host identifier: a name/pattern (e.g. "esxi-01.local") or a full inventory path as returned by vmware_list_hosts. Must resolve to exactly one host.`,
	}
	datastoreArg := map[string]interface{}{
		"type":        "string",
		"description": `Datastore name/pattern (e.g. "datastore1") as returned by vmware_list_datastores. Must resolve to exactly one datastore.`,
	}
	podArg := map[string]interface{}{
		"type":        "string",
		"description": `Datastore cluster (StoragePod) name/pattern (e.g. "pod1") or a full inventory path. Must resolve to exactly one datastore cluster.`,
	}
	keyArrayArg := map[string]interface{}{
		"type":        "array",
		"items":       map[string]interface{}{"type": "string"},
		"description": "Recommendation key(s) (ClusterRecommendation.key) to act on — typically obtained from a prior vmware_storage_recommend_datastores call's result.",
	}

	// --- Read-only ---------------------------------------------------

	r.register("vmware_storage_query_datastore_performance_summary",
		"Query recent I/O performance statistics (latency, IOPS, throughput) Storage I/O Control has recorded for a datastore.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"datastore": datastoreArg},
			"required":   []interface{}{"datastore"},
		},
		Tool{Handler: handleStorageQueryDatastorePerformanceSummary},
	)

	r.register("vmware_storage_query_iorm_config_option",
		"Query the valid Storage I/O Resource Management (IORM) configuration options/limits for a host — the input constraints for vmware_storage_configure_datastore_iorm's spec.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"host": hostArg},
			"required":   []interface{}{"host"},
		},
		Tool{Handler: handleStorageQueryIORMConfigOption},
	)

	r.register("vmware_storage_recommend_datastores",
		"Compute (but do not apply) Storage DRS initial-placement/migration recommendations for a given placement spec. Read-only: it returns a list of recommendations for a caller to review and separately approve via vmware_storage_apply_drs_recommendation(_to_pod) — nothing is moved or reconfigured by this call itself (see this file's top doc comment for why this deviates from Tier 2). storage_spec must be a types.StoragePlacementSpec JSON object matching its Go struct field names exactly (\"type\": \"create\"|\"clone\"|\"relocate\"|\"reconfigure\", \"podSelectionSpec\": {\"initialVmConfig\": [{\"storagePod\": {\"type\":\"StoragePod\",\"value\":\"...\"}, \"disk\": [...]}]}, etc.) — this is a real, fairly complex/polymorphic struct; a caller needs to know the underlying govmomi field names (same MVP limitation already accepted elsewhere in this project for complex specs, e.g. generated_vm_provisioning.go's CustomizationSpec). Fields requiring a genuinely polymorphic Go type (e.g. configSpec.deviceChange) cannot be supplied through this tool at all — vim25/types has no custom JSON unmarshaling for those, so submitting one fails to decode; omit configSpec.deviceChange and leave any per-disk placement.disk lists empty to still get a valid per-datastore recommendation list.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"storage_spec": map[string]interface{}{"type": "object", "description": "A types.StoragePlacementSpec JSON object — see this tool's description for field-name and polymorphic-field caveats."},
			},
			"required": []interface{}{"storage_spec"},
		},
		Tool{Handler: handleStorageRecommendDatastores},
	)

	// --- Tier 2 (disruptive but reversible) ---------------------------

	r.registerDestructive("vmware_storage_apply_drs_recommendation",
		"Apply one or more previously computed Storage DRS recommendations (see vmware_storage_recommend_datastores) by key, triggering the underlying disk/VM relocation(s).",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"key":     keyArrayArg,
				"confirm": confirmArg,
			},
			"required": []interface{}{"key", "confirm"},
		},
		Tool{Handler: handleStorageApplyDrsRecommendation},
	)

	r.registerDestructive("vmware_storage_apply_drs_recommendation_to_pod",
		"Apply a single previously computed Storage DRS recommendation by key, scoped to a specific datastore cluster (StoragePod).",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"pod":     podArg,
				"key":     map[string]interface{}{"type": "string", "description": "Recommendation key (ClusterRecommendation.key) to apply."},
				"confirm": confirmArg,
			},
			"required": []interface{}{"pod", "key", "confirm"},
		},
		Tool{Handler: handleStorageApplyDrsRecommendationToPod},
	)

	r.registerDestructive("vmware_storage_cancel_drs_recommendation",
		"Discard one or more pending Storage DRS recommendations by key without applying them — they will not be re-offered and must be recomputed via vmware_storage_recommend_datastores/vmware_storage_refresh_drs_recommendation if still needed.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"key":     keyArrayArg,
				"confirm": confirmArg,
			},
			"required": []interface{}{"key", "confirm"},
		},
		Tool{Handler: handleStorageCancelDrsRecommendation},
	)

	r.registerDestructive("vmware_storage_configure_datastore_iorm",
		"Configure Storage I/O Resource Management (IORM/Storage I/O Control) settings for a datastore. The \"key\" argument is accepted for Go-signature fidelity but is NOT sent to the server by govmomi's own client wrapper — confirmed by reading the real method body; it has no effect and exists only if a future govmomi version starts using it.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"datastore": datastoreArg,
				"spec":      map[string]interface{}{"type": "object", "description": "A types.StorageIORMConfigSpec JSON object matching its Go struct fields (e.g. {\"enabled\": true, \"congestionThreshold\": 30})."},
				"key":       map[string]interface{}{"type": "string", "description": "Accepted but currently ignored — see this tool's description."},
				"confirm":   confirmArg,
			},
			"required": []interface{}{"datastore", "spec", "confirm"},
		},
		Tool{Handler: handleStorageConfigureDatastoreIORM},
	)

	r.registerDestructive("vmware_storage_configure_drs_for_pod",
		"Configure Storage DRS settings (enable/disable, default VM behavior, load-balance interval, per-VM overrides) for a datastore cluster (StoragePod).",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"pod":     podArg,
				"spec":    map[string]interface{}{"type": "object", "description": "A types.StorageDrsConfigSpec JSON object matching its Go struct fields, e.g. {\"podConfigSpec\": {\"enabled\": true, \"defaultVmBehavior\": \"automated\"}}."},
				"modify":  map[string]interface{}{"type": "boolean", "description": "If true (default false/zero value), spec is applied incrementally on top of the existing configuration; if false, the pod's configuration is replaced to match spec exactly."},
				"confirm": confirmArg,
			},
			"required": []interface{}{"pod", "spec", "confirm"},
		},
		Tool{Handler: handleStorageConfigureDrsForPod},
	)

	r.registerDestructive("vmware_storage_refresh_drs_recommendation",
		"Force Storage DRS to recompute its recommendations for a datastore cluster (StoragePod) right now, instead of waiting for its next scheduled interval. No configuration change, but classified alongside the other Storage DRS mutating actions here per the plan's Fase 1a severity table (see vmware_host_storage_refresh/vmware_vm_refresh_storage_info for the same convention).",
		tier2,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"pod": podArg, "confirm": confirmArg},
			"required":   []interface{}{"pod", "confirm"},
		},
		Tool{Handler: handleStorageRefreshDrsRecommendation},
	)
}

// storageResourceManager builds the client's StorageResourceManager —
// object.NewStorageResourceManager(client.Client.Client) is the same
// "*vim25.Client" access pattern already used for other bare managers in
// this codebase.
func storageResourceManager(client *vmware.Client) *object.StorageResourceManager {
	return object.NewStorageResourceManager(client.Client.Client)
}

// resolveStoragePod resolves the given name/pattern to exactly one
// datastore cluster (StoragePod) — see this file's top doc comment for why
// it is scoped with dcScopedPath("datastore", ...), the same
// per-datacenter Finder folder as resolveDatastore.
func resolveStoragePod(ctx context.Context, client *vmware.Client, name string) (*object.StoragePod, error) {
	if name == "" {
		return nil, fmt.Errorf("pod is required")
	}
	pod, err := client.Finder.DatastoreCluster(ctx, dcScopedPath("datastore", name))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve datastore cluster (pod) %q: %w", name, err)
	}
	return pod, nil
}

func handleStorageQueryDatastorePerformanceSummary(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	dsName, _ := args["datastore"].(string)
	ds, err := resolveDatastore(ctx, client, dsName)
	if err != nil {
		return "", err
	}

	summaries, err := storageResourceManager(client).QueryDatastorePerformanceSummary(ctx, ds)
	if err != nil {
		return "", fmt.Errorf("failed to query performance summary for datastore %s: %w", ds.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{
		"datastore": ds.InventoryPath,
		"count":     len(summaries),
		"summaries": summaries,
	})
}

func handleStorageQueryIORMConfigOption(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}

	opt, err := storageResourceManager(client).QueryIORMConfigOption(ctx, host)
	if err != nil {
		return "", fmt.Errorf("failed to query IORM config option for %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "result": opt})
}

func handleStorageRecommendDatastores(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	raw, ok := args["storage_spec"]
	if !ok {
		return "", fmt.Errorf("storage_spec is required")
	}
	var spec types.StoragePlacementSpec
	if err := decodeJSONArg(raw, &spec); err != nil {
		return "", fmt.Errorf("invalid storage_spec: %w", err)
	}

	result, err := storageResourceManager(client).RecommendDatastores(ctx, spec)
	if err != nil {
		return "", fmt.Errorf("failed to recommend datastores: %w", err)
	}

	return marshalJSON(map[string]interface{}{
		"recommendation_count": len(result.Recommendations),
		"result":               result,
	})
}

func handleStorageApplyDrsRecommendation(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	raw, ok := args["key"]
	if !ok {
		return "", fmt.Errorf("key is required")
	}
	key, err := toStringSlice(raw)
	if err != nil {
		return "", fmt.Errorf("invalid key: %w", err)
	}
	if len(key) == 0 {
		return "", fmt.Errorf("key must be a non-empty array")
	}

	task, err := storageResourceManager(client).ApplyStorageDrsRecommendation(ctx, key)
	if err != nil {
		return "", fmt.Errorf("failed to apply storage DRS recommendation(s) %v: %w", key, err)
	}
	if err := waitForTask(ctx, task); err != nil {
		return "", fmt.Errorf("apply-storage-drs-recommendation task failed for %v: %w", key, err)
	}

	return marshalJSON(map[string]interface{}{"key": key, "result": "applied"})
}

func handleStorageApplyDrsRecommendationToPod(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	podName, _ := args["pod"].(string)
	pod, err := resolveStoragePod(ctx, client, podName)
	if err != nil {
		return "", err
	}
	key, _ := args["key"].(string)
	if key == "" {
		return "", fmt.Errorf("key is required")
	}

	task, err := storageResourceManager(client).ApplyStorageDrsRecommendationToPod(ctx, pod, key)
	if err != nil {
		return "", fmt.Errorf("failed to apply storage DRS recommendation %q to pod %s: %w", key, pod.InventoryPath, err)
	}
	if err := waitForTask(ctx, task); err != nil {
		return "", fmt.Errorf("apply-storage-drs-recommendation-to-pod task failed for %s: %w", pod.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"pod": pod.InventoryPath, "key": key, "result": "applied"})
}

func handleStorageCancelDrsRecommendation(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	raw, ok := args["key"]
	if !ok {
		return "", fmt.Errorf("key is required")
	}
	key, err := toStringSlice(raw)
	if err != nil {
		return "", fmt.Errorf("invalid key: %w", err)
	}
	if len(key) == 0 {
		return "", fmt.Errorf("key must be a non-empty array")
	}

	if err := storageResourceManager(client).CancelStorageDrsRecommendation(ctx, key); err != nil {
		return "", fmt.Errorf("failed to cancel storage DRS recommendation(s) %v: %w", key, err)
	}

	return marshalJSON(map[string]interface{}{"key": key, "result": "cancelled"})
}

func handleStorageConfigureDatastoreIORM(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	dsName, _ := args["datastore"].(string)
	ds, err := resolveDatastore(ctx, client, dsName)
	if err != nil {
		return "", err
	}
	rawSpec, ok := args["spec"]
	if !ok {
		return "", fmt.Errorf("spec is required")
	}
	var spec types.StorageIORMConfigSpec
	if err := decodeJSONArg(rawSpec, &spec); err != nil {
		return "", fmt.Errorf("invalid spec: %w", err)
	}
	key, _ := args["key"].(string)

	task, err := storageResourceManager(client).ConfigureDatastoreIORM(ctx, ds, spec, key)
	if err != nil {
		return "", fmt.Errorf("failed to configure IORM for datastore %s: %w", ds.InventoryPath, err)
	}
	if err := waitForTask(ctx, task); err != nil {
		return "", fmt.Errorf("configure-datastore-iorm task failed for %s: %w", ds.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"datastore": ds.InventoryPath, "result": "configured"})
}

func handleStorageConfigureDrsForPod(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	podName, _ := args["pod"].(string)
	pod, err := resolveStoragePod(ctx, client, podName)
	if err != nil {
		return "", err
	}
	rawSpec, ok := args["spec"]
	if !ok {
		return "", fmt.Errorf("spec is required")
	}
	var spec types.StorageDrsConfigSpec
	if err := decodeJSONArg(rawSpec, &spec); err != nil {
		return "", fmt.Errorf("invalid spec: %w", err)
	}
	modify, _ := args["modify"].(bool)

	task, err := storageResourceManager(client).ConfigureStorageDrsForPod(ctx, pod, spec, modify)
	if err != nil {
		return "", fmt.Errorf("failed to configure storage DRS for pod %s: %w", pod.InventoryPath, err)
	}
	if err := waitForTask(ctx, task); err != nil {
		return "", fmt.Errorf("configure-storage-drs-for-pod task failed for %s: %w", pod.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"pod": pod.InventoryPath, "modify": modify, "result": "configured"})
}

func handleStorageRefreshDrsRecommendation(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	podName, _ := args["pod"].(string)
	pod, err := resolveStoragePod(ctx, client, podName)
	if err != nil {
		return "", err
	}

	if err := storageResourceManager(client).RefreshStorageDrsRecommendation(ctx, pod); err != nil {
		return "", fmt.Errorf("failed to refresh storage DRS recommendation for pod %s: %w", pod.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"pod": pod.InventoryPath, "result": "refreshed"})
}

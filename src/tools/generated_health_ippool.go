package tools

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// registerHealthIpPoolTools adds two unrelated global vCenter manager
// families that happen to share this file because both are small, both are
// vCenter-only singletons reached off ServiceContent, and neither has an
// object.* wrapper in referencia/govmomi/object (confirmed by directory
// listing) — every handler below dials the raw vim25 SOAP method directly:
// methods.Xxx(ctx, client.Client.Client, &types.Xxx{This: ref, ...}), same
// pattern as generated_alarm.go (AlarmManager) and generated_host_iscsi_
// portbinding.go (IscsiManager).
//
// --- HealthUpdateManager (vmware_health_*) ------------------------------
//
// MoRef: client.Client.ServiceContent.HealthUpdateManager
// (*types.ManagedObjectReference, referencia/govmomi/vim25/types/types.go).
// HealthUpdateManager tracks "health update providers" (extensions that
// monitor hardware/component health — e.g. a hardware health plugin) and the
// entities/filters/updates each provider registers.
//
// Method inventory (confirmed against referencia/govmomi/vim25/methods/
// methods.go + types.go — no invented names): QueryHealthUpdateInfos,
// AddMonitoredEntities, RemoveMonitoredEntities, QueryMonitoredEntities,
// AddFilter, RemoveFilter, QueryFilterList, QueryFilterInfoIds,
// QueryFilterEntities, PostHealthUpdates. The task brief also named
// "GetHealthUpdateInfos" and "ReadNextHealthUpdates" — grepping methods.go
// and types.go for both found NEITHER; they do not exist in vim25 v0.55.1 (no
// such request/response type, no such SOAP method). Not implemented — adding
// a tool that dials a nonexistent method would fail every call with a Go
// compile error, not a runtime one. QueryHealthUpdateInfos is this API's real
// "list the health updates a provider has reported" accessor.
//
// Tier: AddMonitoredEntities, RemoveMonitoredEntities, AddFilter,
// RemoveFilter, and PostHealthUpdates are tier2 (registerDestructive) — all
// five mutate HealthUpdateManager's registration/filter/update state but are
// reversible (add/remove are each other's inverse; a filter can be re-added;
// posted updates are superseded by the next post). QueryHealthUpdateInfos,
// QueryMonitoredEntities, QueryFilterList, QueryFilterInfoIds, and
// QueryFilterEntities are plain r.register (read-only).
//
// --- IpPoolManager (vmware_ippool_*) ------------------------------------
//
// MoRef: client.Client.ServiceContent.IpPoolManager
// (*types.ManagedObjectReference). IP pools are scoped to a Datacenter — every
// IpPoolManager method takes a "dc" ManagedObjectReference alongside its own
// This; every tool below resolves that via this project's existing
// resolveDatacenter(ctx, client, name) helper (generated_datastore_
// browser.go), the same Finder-based name/path resolution vmware_datastore_
// namespace_create_directory's tests already exercise for "dc".
//
// Method inventory (confirmed): QueryIpPools, CreateIpPool, UpdateIpPool,
// DestroyIpPool, AllocateIpv4Address, AllocateIpv6Address,
// ReleaseIpAllocation, QueryIPAllocations.
//
// Field-naming note: DestroyIpPoolRequestType names its pool identifier
// field "Id" (int32) while Allocate*/ReleaseIpAllocation/QueryIPAllocations
// all name theirs "PoolId" (int32) — same integer, two different vim25 field
// names on different request structs. Every tool here accepts one
// consistently-named "pool_id" JSON argument and maps it to whichever wire
// field its request type actually has, rather than exposing the
// inconsistency to callers.
//
// QueryIPAllocationsRequestType's own identifying field is "ExtensionKey"
// (not "AllocationId") — vcsim's simulator/ip_pool_manager.go looks it up in
// the very same map Allocate*/Release key by allocation ID
// (pool.ipv4Allocation[req.ExtensionKey]), confirming ExtensionKey and
// AllocationId are the same conceptual value in this API. vmware_ippool_
// query_allocations' argument is still named "extension_key" (matching the
// real wire field) rather than silently renamed to "allocation_id", so the
// tool's JSON schema stays honest about what vim25 actually calls it.
//
// Tier: CreateIpPool, UpdateIpPool, AllocateIpv4Address, AllocateIpv6Address,
// and ReleaseIpAllocation are tier2 (disruptive to IP allocation but
// reversible — created pools can be destroyed, updates can be reapplied,
// allocations can be released and re-allocated). DestroyIpPool is tier1
// (irreversible — deletes the pool definition; "force" lets it proceed even
// with active allocations, per DestroyIpPoolRequestType's own Force field).
// QueryIpPools and QueryIPAllocations are plain r.register (read-only).
//
// --- Class: modeVCenterOnly (both managers, evidence-based) -------------
//
// referencia/govmomi/simulator/esx/service_content.go sets both
// HealthUpdateManager and IpPoolManager to nil ((*types.ManagedObjectReference)
// (nil)); referencia/govmomi/simulator/vpx/service_content.go populates both
// with real MoRefs. A standalone-ESXi connection therefore has neither
// manager to call — same evidence pattern generated_alarm.go used for
// AlarmManager (also nil on ESX(), populated on VPX()).
//
// --- vcsim coverage ------------------------------------------------------
//
// IpPoolManager: referencia/govmomi/simulator/ip_pool_manager.go implements
// a real (if simplified — "all pools are shared across different
// datacenters", per its own doc comment) handler for ALL 8 methods:
// CreateIpPool, DestroyIpPool, QueryIpPools, UpdateIpPool,
// AllocateIpv4Address, AllocateIpv6Address, ReleaseIpAllocation,
// QueryIPAllocations. generated_health_ippool_test.go therefore drives every
// vmware_ippool_* tool through one real create -> query -> allocate ->
// query_allocations -> release -> update -> destroy lifecycle against
// simulator.VPX(), asserting actual returned state (assertRealSuccess-style,
// same posture as generated_alarm.go's TestAlarmTools_RealSuccess).
//
// HealthUpdateManager: there is NO simulator/health_update_manager.go (or
// any file implementing HealthUpdateManager) anywhere under referencia/
// govmomi/simulator — confirmed by directory listing (only ip_pool_manager.go
// exists for this file's two families). Every vmware_health_* call therefore
// reaches vcsim's generic dispatch fallback and comes back with a clean
// server-side fault; generated_health_ippool_test.go drives all 10 with
// assertReachesServer (same helper/rationale as generated_alarm.go's
// TestAlarmTools_ReachesServer for AlarmManager's 4 unsimulated methods, and
// generated_host_iscsi_portbinding_test.go's 15 unsimulated HostStorageSystem
// methods): the tests prove the wiring — schema, gate, HealthUpdateManager
// MoRef, raw method dispatch — reaches vcsim and returns a clean server-side
// error, not an unknown-tool wiring bug or a recovered panic. Behavioral
// validation of HealthUpdateManager itself is expected against a real
// vCenter with a hardware health provider registered.
func registerHealthIpPoolTools(r *Registry) {
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}
	moRefSchema := map[string]interface{}{
		"type":        "object",
		"description": `A ManagedObjectReference, e.g. {"type": "HostSystem", "value": "host-42"}.`,
		"properties": map[string]interface{}{
			"type":  map[string]interface{}{"type": "string", "description": "The managed object's vSphere type, e.g. \"HostSystem\", \"VirtualMachine\"."},
			"value": map[string]interface{}{"type": "string", "description": "The managed object's MoRef value."},
		},
		"required": []interface{}{"type", "value"},
	}

	// --- HealthUpdateManager argument schemas -----------------------------

	hupdProviderIDArg := map[string]interface{}{
		"type":        "string",
		"description": `The HealthUpdateProvider identifier (assigned by the extension/plugin that registers health monitoring for a component — e.g. a hardware health provider), as returned in the "providerId" of the entities/filters/updates it has registered.`,
	}
	hupdFilterIDArg := map[string]interface{}{
		"type":        "string",
		"description": `The filter's ID, as returned by vmware_health_add_filter's "filter" field.`,
	}
	hupdEntitiesArg := map[string]interface{}{
		"type":        "array",
		"items":       moRefSchema,
		"description": `The entities (ManagedObjectReference, e.g. HostSystem) a health update provider is newly monitoring, or no longer monitoring.`,
	}

	r.register("vmware_health_query_update_infos",
		"List the HealthUpdateInfo definitions (id/componentType/description) a health update provider has reported to HealthUpdateManager.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"provider_id": hupdProviderIDArg},
			"required":   []interface{}{"provider_id"},
		},
		Tool{Handler: handleHealthQueryUpdateInfos},
	)

	r.registerDestructive("vmware_health_add_monitored_entities",
		"Register entities (e.g. hosts) as newly monitored by a health update provider. Reversible via vmware_health_remove_monitored_entities.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"provider_id": hupdProviderIDArg,
				"entities":    hupdEntitiesArg,
				"confirm":     confirmArg,
			},
			"required": []interface{}{"provider_id", "entities", "confirm"},
		},
		Tool{Handler: handleHealthAddMonitoredEntities},
	)

	r.registerDestructive("vmware_health_remove_monitored_entities",
		"Unregister entities from a health update provider's monitored set. Reversible via vmware_health_add_monitored_entities.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"provider_id": hupdProviderIDArg,
				"entities":    hupdEntitiesArg,
				"confirm":     confirmArg,
			},
			"required": []interface{}{"provider_id", "entities", "confirm"},
		},
		Tool{Handler: handleHealthRemoveMonitoredEntities},
	)

	r.register("vmware_health_query_monitored_entities",
		"List the entities currently monitored by a health update provider.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"provider_id": hupdProviderIDArg},
			"required":   []interface{}{"provider_id"},
		},
		Tool{Handler: handleHealthQueryMonitoredEntities},
	)

	r.registerDestructive("vmware_health_add_filter",
		"Create a HealthUpdateManager filter scoping which HealthUpdateInfo IDs from a provider are of interest. Reversible via vmware_health_remove_filter.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"provider_id": hupdProviderIDArg,
				"filter_name": map[string]interface{}{"type": "string", "description": "Name for the new filter."},
				"info_ids": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "HealthUpdateInfo IDs to filter on. Omit/empty for no ID restriction.",
				},
				"confirm": confirmArg,
			},
			"required": []interface{}{"provider_id", "filter_name", "confirm"},
		},
		Tool{Handler: handleHealthAddFilter},
	)

	r.registerDestructive("vmware_health_remove_filter",
		"Delete a HealthUpdateManager filter. Reversible via vmware_health_add_filter with the same provider/name/infoIds.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"filter_id": hupdFilterIDArg,
				"confirm":   confirmArg,
			},
			"required": []interface{}{"filter_id", "confirm"},
		},
		Tool{Handler: handleHealthRemoveFilter},
	)

	r.register("vmware_health_query_filter_list",
		"List the filter IDs a health update provider has registered on HealthUpdateManager.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"provider_id": hupdProviderIDArg},
			"required":   []interface{}{"provider_id"},
		},
		Tool{Handler: handleHealthQueryFilterList},
	)

	r.register("vmware_health_query_filter_info_ids",
		"List the HealthUpdateInfo IDs a filter is scoped to.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"filter_id": hupdFilterIDArg},
			"required":   []interface{}{"filter_id"},
		},
		Tool{Handler: handleHealthQueryFilterInfoIds},
	)

	r.register("vmware_health_query_filter_entities",
		"List the entities a filter is scoped to.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"filter_id": hupdFilterIDArg},
			"required":   []interface{}{"filter_id"},
		},
		Tool{Handler: handleHealthQueryFilterEntities},
	)

	r.registerDestructive("vmware_health_post_updates",
		"Report one or more health state changes (HealthUpdate) from a provider to HealthUpdateManager. Reversible only in the sense that a subsequent post supersedes prior state — HealthUpdateManager keeps no post history to roll back to.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"provider_id": hupdProviderIDArg,
				"updates": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type":        "object",
						"description": "One HealthUpdate.",
						"properties": map[string]interface{}{
							"entity":             moRefSchema,
							"healthUpdateInfoId": map[string]interface{}{"type": "string", "description": "The ID of the corresponding HealthUpdateInfo."},
							"id":                 map[string]interface{}{"type": "string", "description": "This HealthUpdate instance's own ID, for cross-reference with provider logs."},
							"status":             map[string]interface{}{"type": "string", "description": `Current health status: "gray"|"green"|"yellow"|"red".`},
							"remediation":        map[string]interface{}{"type": "string", "description": `Description of the physical remediation required, e.g. "Replace Fan #3".`},
						},
						"required": []interface{}{"entity", "healthUpdateInfoId", "id", "status"},
					},
					"description": "One or more health state changes to report.",
				},
				"confirm": confirmArg,
			},
			"required": []interface{}{"provider_id", "updates", "confirm"},
		},
		Tool{Handler: handleHealthPostUpdates},
	)

	// --- IpPoolManager argument schemas ------------------------------------

	ippoolDcArg := map[string]interface{}{
		"type":        "string",
		"description": `Datacenter identifier: a name/pattern or inventory path, as returned by vmware_list_datacenters. IP pools are scoped to a datacenter.`,
	}
	ippoolPoolIDArg := map[string]interface{}{
		"type":        "integer",
		"description": `The pool's unique numeric ID, as returned by vmware_ippool_create's "pool_id" field or vmware_ippool_query's per-entry "id" field.`,
	}
	ippoolAllocationIDArg := map[string]interface{}{
		"type":        "string",
		"description": "Caller-chosen unique identifier for this allocation (e.g. a VM's instance UUID or MoRef value). Allocating twice with the same allocation_id against the same pool returns the same IP rather than allocating a second one.",
	}
	ippoolConfigSchema := map[string]interface{}{
		"type":        "object",
		"description": "IpPoolIpPoolConfigInfo — one address family's configuration for a pool.",
		"properties": map[string]interface{}{
			"subnetAddress":       map[string]interface{}{"type": "string", "description": "Subnet address, e.g. \"192.168.5.0\" (IPv4) or \"2001:0db8:85a3::\" (IPv6)."},
			"netmask":             map[string]interface{}{"type": "string", "description": "Netmask, e.g. \"255.255.255.0\" (IPv4) or \"ffff:ffff:ffff::\" (IPv6)."},
			"gateway":             map[string]interface{}{"type": "string", "description": "Gateway address. Empty string if none configured."},
			"range":               map[string]interface{}{"type": "string", "description": "Comma-separated ranges as \"<start-address>#<length>\", e.g. \"192.0.2.235#20\"."},
			"dns":                 map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "DNS server addresses."},
			"dhcpServerAvailable": map[string]interface{}{"type": "boolean"},
			"ipPoolEnabled":       map[string]interface{}{"type": "boolean", "description": "IP addresses can only be allocated from this range while true."},
		},
	}
	ippoolSpecArg := map[string]interface{}{
		"type":        "object",
		"description": "IpPool spec. \"name\" required on create. \"id\" is required on update (identifies the pool to update) and ignored on create (the server assigns it). At least one of ipv4Config/ipv6Config should be set for the pool to be usable.",
		"properties": map[string]interface{}{
			"id":            map[string]interface{}{"type": "integer", "description": "Required on update; ignored on create."},
			"name":          map[string]interface{}{"type": "string", "description": "Pool name, unique within the datacenter."},
			"ipv4Config":    ippoolConfigSchema,
			"ipv6Config":    ippoolConfigSchema,
			"dnsDomain":     map[string]interface{}{"type": "string"},
			"dnsSearchPath": map[string]interface{}{"type": "string"},
			"hostPrefix":    map[string]interface{}{"type": "string"},
			"httpProxy":     map[string]interface{}{"type": "string", "description": "HTTP proxy for this network, e.g. \"proxy.example.com:3128\"."},
		},
	}

	r.register("vmware_ippool_query",
		"List the IP pools defined on a datacenter.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"dc": ippoolDcArg},
			"required":   []interface{}{"dc"},
		},
		Tool{Handler: handleIppoolQuery},
	)

	r.registerDestructive("vmware_ippool_create",
		"Create a new IP pool on a datacenter. Reversible via vmware_ippool_destroy.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"dc":      ippoolDcArg,
				"pool":    ippoolSpecArg,
				"confirm": confirmArg,
			},
			"required": []interface{}{"dc", "pool", "confirm"},
		},
		Tool{Handler: handleIppoolCreate},
	)

	r.registerDestructive("vmware_ippool_update",
		"Replace an existing IP pool's configuration (pool.id selects which pool). Fails if the pool has active allocations, on servers that enforce that (vcsim does). Reversible by updating again with the previous configuration.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"dc":      ippoolDcArg,
				"pool":    ippoolSpecArg,
				"confirm": confirmArg,
			},
			"required": []interface{}{"dc", "pool", "confirm"},
		},
		Tool{Handler: handleIppoolUpdate},
	)

	r.registerDestructive("vmware_ippool_destroy",
		"Permanently delete an IP pool. Irreversible — the pool definition is gone; re-creating it requires vmware_ippool_create with the same spec.",
		tier1,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"dc":      ippoolDcArg,
				"pool_id": ippoolPoolIDArg,
				"force":   map[string]interface{}{"type": "boolean", "description": "Destroy even if the pool has active allocations. Defaults to false."},
				"confirm": confirmArg,
			},
			"required": []interface{}{"dc", "pool_id", "confirm"},
		},
		Tool{Handler: handleIppoolDestroy},
	)

	r.registerDestructive("vmware_ippool_allocate_ipv4",
		"Allocate an IPv4 address from a pool for allocation_id (idempotent — the same allocation_id returns the same address). Reversible via vmware_ippool_release_allocation.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"dc":            ippoolDcArg,
				"pool_id":       ippoolPoolIDArg,
				"allocation_id": ippoolAllocationIDArg,
				"confirm":       confirmArg,
			},
			"required": []interface{}{"dc", "pool_id", "allocation_id", "confirm"},
		},
		Tool{Handler: handleIppoolAllocateIpv4},
	)

	r.registerDestructive("vmware_ippool_allocate_ipv6",
		"Allocate an IPv6 address from a pool for allocation_id (idempotent — the same allocation_id returns the same address). Reversible via vmware_ippool_release_allocation.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"dc":            ippoolDcArg,
				"pool_id":       ippoolPoolIDArg,
				"allocation_id": ippoolAllocationIDArg,
				"confirm":       confirmArg,
			},
			"required": []interface{}{"dc", "pool_id", "allocation_id", "confirm"},
		},
		Tool{Handler: handleIppoolAllocateIpv6},
	)

	r.registerDestructive("vmware_ippool_release_allocation",
		"Release a previously allocated IPv4 or IPv6 address back to the pool. Reversible via vmware_ippool_allocate_ipv4/vmware_ippool_allocate_ipv6 with the same allocation_id (may return a different address).",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"dc":            ippoolDcArg,
				"pool_id":       ippoolPoolIDArg,
				"allocation_id": ippoolAllocationIDArg,
				"confirm":       confirmArg,
			},
			"required": []interface{}{"dc", "pool_id", "allocation_id", "confirm"},
		},
		Tool{Handler: handleIppoolReleaseAllocation},
	)

	r.register("vmware_ippool_query_allocations",
		"Look up the IP address (IPv4 and/or IPv6) currently allocated under a given extension_key on a pool.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"dc":            ippoolDcArg,
				"pool_id":       ippoolPoolIDArg,
				"extension_key": map[string]interface{}{"type": "string", "description": "The extension/allocation key to look up — the same value passed as allocation_id to vmware_ippool_allocate_ipv4/vmware_ippool_allocate_ipv6."},
			},
			"required": []interface{}{"dc", "pool_id", "extension_key"},
		},
		Tool{Handler: handleIppoolQueryAllocations},
	)
}

// --- HealthUpdateManager: ref/arg helpers --------------------------------

// hupdManagerRef returns the connected endpoint's HealthUpdateManager MoRef.
// Nil on a standalone ESXi host (this file's top doc comment) — every tool
// here registers modeVCenterOnly, so this should never actually observe a
// nil ref through a connection mode that lets the class register at all; the
// check is defense in depth, same posture as generated_alarm.go's
// alarmManagerRef.
func hupdManagerRef(client *vmware.Client) (types.ManagedObjectReference, error) {
	ref := client.Client.ServiceContent.HealthUpdateManager
	if ref == nil {
		return types.ManagedObjectReference{}, fmt.Errorf("this vCenter/ESXi endpoint does not expose a HealthUpdateManager")
	}
	return *ref, nil
}

// hupdProviderID reads and validates the required provider_id argument.
func hupdProviderID(args map[string]interface{}) (string, error) {
	id, _ := args["provider_id"].(string)
	if id == "" {
		return "", fmt.Errorf("provider_id is required")
	}
	return id, nil
}

// hupdFilterID reads and validates the required filter_id argument.
func hupdFilterID(args map[string]interface{}) (string, error) {
	id, _ := args["filter_id"].(string)
	if id == "" {
		return "", fmt.Errorf("filter_id is required")
	}
	return id, nil
}

// hupdEntities decodes and validates the required, non-empty entities
// argument into []types.ManagedObjectReference.
func hupdEntities(args map[string]interface{}) ([]types.ManagedObjectReference, error) {
	raw, ok := args["entities"]
	if !ok {
		return nil, fmt.Errorf("entities is required")
	}
	var entities []types.ManagedObjectReference
	if err := decodeJSONArg(raw, &entities); err != nil {
		return nil, fmt.Errorf("invalid entities: %w", err)
	}
	if len(entities) == 0 {
		return nil, fmt.Errorf("entities must be a non-empty array")
	}
	return entities, nil
}

// --- HealthUpdateManager: handlers ----------------------------------------

func handleHealthQueryUpdateInfos(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgr, err := hupdManagerRef(client)
	if err != nil {
		return "", err
	}
	providerID, err := hupdProviderID(args)
	if err != nil {
		return "", err
	}

	resp, err := methods.QueryHealthUpdateInfos(ctx, client.Client.Client, &types.QueryHealthUpdateInfos{
		This:       mgr,
		ProviderId: providerID,
	})
	if err != nil {
		return "", fmt.Errorf("failed to query health update infos for provider %s: %w", providerID, err)
	}

	return marshalJSON(map[string]interface{}{
		"provider_id": providerID,
		"count":       len(resp.Returnval),
		"infos":       resp.Returnval,
	})
}

func handleHealthAddMonitoredEntities(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgr, err := hupdManagerRef(client)
	if err != nil {
		return "", err
	}
	providerID, err := hupdProviderID(args)
	if err != nil {
		return "", err
	}
	entities, err := hupdEntities(args)
	if err != nil {
		return "", err
	}

	if _, err := methods.AddMonitoredEntities(ctx, client.Client.Client, &types.AddMonitoredEntities{
		This:       mgr,
		ProviderId: providerID,
		Entities:   entities,
	}); err != nil {
		return "", fmt.Errorf("failed to add monitored entities for provider %s: %w", providerID, err)
	}

	return marshalJSON(map[string]interface{}{"provider_id": providerID, "count": len(entities), "result": "entities_added"})
}

func handleHealthRemoveMonitoredEntities(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgr, err := hupdManagerRef(client)
	if err != nil {
		return "", err
	}
	providerID, err := hupdProviderID(args)
	if err != nil {
		return "", err
	}
	entities, err := hupdEntities(args)
	if err != nil {
		return "", err
	}

	if _, err := methods.RemoveMonitoredEntities(ctx, client.Client.Client, &types.RemoveMonitoredEntities{
		This:       mgr,
		ProviderId: providerID,
		Entities:   entities,
	}); err != nil {
		return "", fmt.Errorf("failed to remove monitored entities for provider %s: %w", providerID, err)
	}

	return marshalJSON(map[string]interface{}{"provider_id": providerID, "count": len(entities), "result": "entities_removed"})
}

func handleHealthQueryMonitoredEntities(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgr, err := hupdManagerRef(client)
	if err != nil {
		return "", err
	}
	providerID, err := hupdProviderID(args)
	if err != nil {
		return "", err
	}

	resp, err := methods.QueryMonitoredEntities(ctx, client.Client.Client, &types.QueryMonitoredEntities{
		This:       mgr,
		ProviderId: providerID,
	})
	if err != nil {
		return "", fmt.Errorf("failed to query monitored entities for provider %s: %w", providerID, err)
	}

	return marshalJSON(map[string]interface{}{
		"provider_id": providerID,
		"count":       len(resp.Returnval),
		"entities":    resp.Returnval,
	})
}

func handleHealthAddFilter(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgr, err := hupdManagerRef(client)
	if err != nil {
		return "", err
	}
	providerID, err := hupdProviderID(args)
	if err != nil {
		return "", err
	}
	filterName, _ := args["filter_name"].(string)
	if filterName == "" {
		return "", fmt.Errorf("filter_name is required")
	}
	var infoIDs []string
	if raw, ok := args["info_ids"]; ok && raw != nil {
		infoIDs, err = toStringSlice(raw)
		if err != nil {
			return "", fmt.Errorf("invalid info_ids: %w", err)
		}
	}

	resp, err := methods.AddFilter(ctx, client.Client.Client, &types.AddFilter{
		This:       mgr,
		ProviderId: providerID,
		FilterName: filterName,
		InfoIds:    infoIDs,
	})
	if err != nil {
		return "", fmt.Errorf("failed to add filter %q for provider %s: %w", filterName, providerID, err)
	}

	return marshalJSON(map[string]interface{}{"provider_id": providerID, "filter_name": filterName, "filter": resp.Returnval, "result": "filter_added"})
}

func handleHealthRemoveFilter(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgr, err := hupdManagerRef(client)
	if err != nil {
		return "", err
	}
	filterID, err := hupdFilterID(args)
	if err != nil {
		return "", err
	}

	if _, err := methods.RemoveFilter(ctx, client.Client.Client, &types.RemoveFilter{
		This:     mgr,
		FilterId: filterID,
	}); err != nil {
		return "", fmt.Errorf("failed to remove filter %s: %w", filterID, err)
	}

	return marshalJSON(map[string]interface{}{"filter_id": filterID, "result": "filter_removed"})
}

func handleHealthQueryFilterList(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgr, err := hupdManagerRef(client)
	if err != nil {
		return "", err
	}
	providerID, err := hupdProviderID(args)
	if err != nil {
		return "", err
	}

	resp, err := methods.QueryFilterList(ctx, client.Client.Client, &types.QueryFilterList{
		This:       mgr,
		ProviderId: providerID,
	})
	if err != nil {
		return "", fmt.Errorf("failed to query filter list for provider %s: %w", providerID, err)
	}

	return marshalJSON(map[string]interface{}{
		"provider_id": providerID,
		"count":       len(resp.Returnval),
		"filters":     resp.Returnval,
	})
}

func handleHealthQueryFilterInfoIds(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgr, err := hupdManagerRef(client)
	if err != nil {
		return "", err
	}
	filterID, err := hupdFilterID(args)
	if err != nil {
		return "", err
	}

	resp, err := methods.QueryFilterInfoIds(ctx, client.Client.Client, &types.QueryFilterInfoIds{
		This:     mgr,
		FilterId: filterID,
	})
	if err != nil {
		return "", fmt.Errorf("failed to query filter info ids for filter %s: %w", filterID, err)
	}

	return marshalJSON(map[string]interface{}{
		"filter_id": filterID,
		"count":     len(resp.Returnval),
		"info_ids":  resp.Returnval,
	})
}

func handleHealthQueryFilterEntities(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgr, err := hupdManagerRef(client)
	if err != nil {
		return "", err
	}
	filterID, err := hupdFilterID(args)
	if err != nil {
		return "", err
	}

	resp, err := methods.QueryFilterEntities(ctx, client.Client.Client, &types.QueryFilterEntities{
		This:     mgr,
		FilterId: filterID,
	})
	if err != nil {
		return "", fmt.Errorf("failed to query filter entities for filter %s: %w", filterID, err)
	}

	return marshalJSON(map[string]interface{}{
		"filter_id": filterID,
		"count":     len(resp.Returnval),
		"entities":  resp.Returnval,
	})
}

func handleHealthPostUpdates(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgr, err := hupdManagerRef(client)
	if err != nil {
		return "", err
	}
	providerID, err := hupdProviderID(args)
	if err != nil {
		return "", err
	}
	raw, ok := args["updates"]
	if !ok {
		return "", fmt.Errorf("updates is required")
	}
	var updates []types.HealthUpdate
	if err := decodeJSONArg(raw, &updates); err != nil {
		return "", fmt.Errorf("invalid updates: %w", err)
	}
	if len(updates) == 0 {
		return "", fmt.Errorf("updates must be a non-empty array")
	}

	if _, err := methods.PostHealthUpdates(ctx, client.Client.Client, &types.PostHealthUpdates{
		This:       mgr,
		ProviderId: providerID,
		Updates:    updates,
	}); err != nil {
		return "", fmt.Errorf("failed to post health updates for provider %s: %w", providerID, err)
	}

	return marshalJSON(map[string]interface{}{"provider_id": providerID, "count": len(updates), "result": "updates_posted"})
}

// --- IpPoolManager: ref/arg helpers ----------------------------------------

// ippoolManagerRef returns the connected endpoint's IpPoolManager MoRef. Nil
// on a standalone ESXi host (this file's top doc comment) — defense in
// depth, same posture as hupdManagerRef/alarmManagerRef.
func ippoolManagerRef(client *vmware.Client) (types.ManagedObjectReference, error) {
	ref := client.Client.ServiceContent.IpPoolManager
	if ref == nil {
		return types.ManagedObjectReference{}, fmt.Errorf("this vCenter/ESXi endpoint does not expose an IpPoolManager")
	}
	return *ref, nil
}

// ippoolResolveDC resolves the required "dc" argument via this project's
// existing resolveDatacenter helper (generated_datastore_browser.go),
// returning both the MoRef IpPoolManager's methods need as their own "dc"
// field and a display string (inventory path) for JSON responses/errors.
func ippoolResolveDC(ctx context.Context, client *vmware.Client, args map[string]interface{}) (types.ManagedObjectReference, string, error) {
	name, _ := args["dc"].(string)
	if name == "" {
		return types.ManagedObjectReference{}, "", fmt.Errorf("dc is required")
	}
	dc, err := resolveDatacenter(ctx, client, name)
	if err != nil {
		return types.ManagedObjectReference{}, "", err
	}
	return dc.Reference(), dc.InventoryPath, nil
}

// ippoolPoolID reads and validates the required pool_id argument. JSON
// numbers decode as float64 through args' generic map[string]interface{},
// same convention this project uses elsewhere for integer arguments.
func ippoolPoolID(args map[string]interface{}) (int32, error) {
	raw, ok := args["pool_id"]
	if !ok {
		return 0, fmt.Errorf("pool_id is required")
	}
	f, ok := raw.(float64)
	if !ok {
		return 0, fmt.Errorf("pool_id must be a number, got %T", raw)
	}
	return int32(f), nil
}

// ippoolAllocationID reads and validates the required allocation_id
// argument.
func ippoolAllocationID(args map[string]interface{}) (string, error) {
	id, _ := args["allocation_id"].(string)
	if id == "" {
		return "", fmt.Errorf("allocation_id is required")
	}
	return id, nil
}

// ippoolBuildSpec decodes the required "pool" argument into a types.IpPool —
// a plain (non-polymorphic) struct, so a direct decodeJSONArg round trip is
// enough (unlike generated_alarm.go's AlarmSpec, which needs the _vimType
// router because Expression/Action are interface-typed fields).
func ippoolBuildSpec(args map[string]interface{}) (types.IpPool, error) {
	raw, ok := args["pool"]
	if !ok || raw == nil {
		return types.IpPool{}, fmt.Errorf("pool is required")
	}
	var spec types.IpPool
	if err := decodeJSONArg(raw, &spec); err != nil {
		return types.IpPool{}, fmt.Errorf("invalid pool: %w", err)
	}
	return spec, nil
}

// --- IpPoolManager: handlers ------------------------------------------------

func handleIppoolQuery(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgr, err := ippoolManagerRef(client)
	if err != nil {
		return "", err
	}
	dcRef, dcDisp, err := ippoolResolveDC(ctx, client, args)
	if err != nil {
		return "", err
	}

	resp, err := methods.QueryIpPools(ctx, client.Client.Client, &types.QueryIpPools{
		This: mgr,
		Dc:   dcRef,
	})
	if err != nil {
		return "", fmt.Errorf("failed to query IP pools on datacenter %s: %w", dcDisp, err)
	}

	return marshalJSON(map[string]interface{}{
		"dc":    dcDisp,
		"count": len(resp.Returnval),
		"pools": resp.Returnval,
	})
}

func handleIppoolCreate(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgr, err := ippoolManagerRef(client)
	if err != nil {
		return "", err
	}
	dcRef, dcDisp, err := ippoolResolveDC(ctx, client, args)
	if err != nil {
		return "", err
	}
	spec, err := ippoolBuildSpec(args)
	if err != nil {
		return "", err
	}

	resp, err := methods.CreateIpPool(ctx, client.Client.Client, &types.CreateIpPool{
		This: mgr,
		Dc:   dcRef,
		Pool: spec,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create IP pool %q on datacenter %s: %w", spec.Name, dcDisp, err)
	}

	return marshalJSON(map[string]interface{}{"dc": dcDisp, "name": spec.Name, "pool_id": resp.Returnval, "result": "pool_created"})
}

func handleIppoolUpdate(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgr, err := ippoolManagerRef(client)
	if err != nil {
		return "", err
	}
	dcRef, dcDisp, err := ippoolResolveDC(ctx, client, args)
	if err != nil {
		return "", err
	}
	spec, err := ippoolBuildSpec(args)
	if err != nil {
		return "", err
	}
	if spec.Id == 0 {
		return "", fmt.Errorf("pool.id is required")
	}

	if _, err := methods.UpdateIpPool(ctx, client.Client.Client, &types.UpdateIpPool{
		This: mgr,
		Dc:   dcRef,
		Pool: spec,
	}); err != nil {
		return "", fmt.Errorf("failed to update IP pool %d on datacenter %s: %w", spec.Id, dcDisp, err)
	}

	return marshalJSON(map[string]interface{}{"dc": dcDisp, "pool_id": spec.Id, "result": "pool_updated"})
}

func handleIppoolDestroy(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgr, err := ippoolManagerRef(client)
	if err != nil {
		return "", err
	}
	dcRef, dcDisp, err := ippoolResolveDC(ctx, client, args)
	if err != nil {
		return "", err
	}
	poolID, err := ippoolPoolID(args)
	if err != nil {
		return "", err
	}
	force, _ := args["force"].(bool)

	if _, err := methods.DestroyIpPool(ctx, client.Client.Client, &types.DestroyIpPool{
		This:  mgr,
		Dc:    dcRef,
		Id:    poolID,
		Force: force,
	}); err != nil {
		return "", fmt.Errorf("failed to destroy IP pool %d on datacenter %s: %w", poolID, dcDisp, err)
	}

	return marshalJSON(map[string]interface{}{"dc": dcDisp, "pool_id": poolID, "force": force, "result": "pool_destroyed"})
}

func handleIppoolAllocateIpv4(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgr, err := ippoolManagerRef(client)
	if err != nil {
		return "", err
	}
	dcRef, dcDisp, err := ippoolResolveDC(ctx, client, args)
	if err != nil {
		return "", err
	}
	poolID, err := ippoolPoolID(args)
	if err != nil {
		return "", err
	}
	allocationID, err := ippoolAllocationID(args)
	if err != nil {
		return "", err
	}

	resp, err := methods.AllocateIpv4Address(ctx, client.Client.Client, &types.AllocateIpv4Address{
		This:         mgr,
		Dc:           dcRef,
		PoolId:       poolID,
		AllocationId: allocationID,
	})
	if err != nil {
		return "", fmt.Errorf("failed to allocate IPv4 address from pool %d on datacenter %s: %w", poolID, dcDisp, err)
	}

	return marshalJSON(map[string]interface{}{"dc": dcDisp, "pool_id": poolID, "allocation_id": allocationID, "ip_address": resp.Returnval})
}

func handleIppoolAllocateIpv6(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgr, err := ippoolManagerRef(client)
	if err != nil {
		return "", err
	}
	dcRef, dcDisp, err := ippoolResolveDC(ctx, client, args)
	if err != nil {
		return "", err
	}
	poolID, err := ippoolPoolID(args)
	if err != nil {
		return "", err
	}
	allocationID, err := ippoolAllocationID(args)
	if err != nil {
		return "", err
	}

	resp, err := methods.AllocateIpv6Address(ctx, client.Client.Client, &types.AllocateIpv6Address{
		This:         mgr,
		Dc:           dcRef,
		PoolId:       poolID,
		AllocationId: allocationID,
	})
	if err != nil {
		return "", fmt.Errorf("failed to allocate IPv6 address from pool %d on datacenter %s: %w", poolID, dcDisp, err)
	}

	return marshalJSON(map[string]interface{}{"dc": dcDisp, "pool_id": poolID, "allocation_id": allocationID, "ip_address": resp.Returnval})
}

func handleIppoolReleaseAllocation(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgr, err := ippoolManagerRef(client)
	if err != nil {
		return "", err
	}
	dcRef, dcDisp, err := ippoolResolveDC(ctx, client, args)
	if err != nil {
		return "", err
	}
	poolID, err := ippoolPoolID(args)
	if err != nil {
		return "", err
	}
	allocationID, err := ippoolAllocationID(args)
	if err != nil {
		return "", err
	}

	if _, err := methods.ReleaseIpAllocation(ctx, client.Client.Client, &types.ReleaseIpAllocation{
		This:         mgr,
		Dc:           dcRef,
		PoolId:       poolID,
		AllocationId: allocationID,
	}); err != nil {
		return "", fmt.Errorf("failed to release allocation %s from pool %d on datacenter %s: %w", allocationID, poolID, dcDisp, err)
	}

	return marshalJSON(map[string]interface{}{"dc": dcDisp, "pool_id": poolID, "allocation_id": allocationID, "result": "allocation_released"})
}

func handleIppoolQueryAllocations(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgr, err := ippoolManagerRef(client)
	if err != nil {
		return "", err
	}
	dcRef, dcDisp, err := ippoolResolveDC(ctx, client, args)
	if err != nil {
		return "", err
	}
	poolID, err := ippoolPoolID(args)
	if err != nil {
		return "", err
	}
	extensionKey, _ := args["extension_key"].(string)
	if extensionKey == "" {
		return "", fmt.Errorf("extension_key is required")
	}

	resp, err := methods.QueryIPAllocations(ctx, client.Client.Client, &types.QueryIPAllocations{
		This:         mgr,
		Dc:           dcRef,
		PoolId:       poolID,
		ExtensionKey: extensionKey,
	})
	if err != nil {
		return "", fmt.Errorf("failed to query IP allocations for extension key %s on pool %d, datacenter %s: %w", extensionKey, poolID, dcDisp, err)
	}

	return marshalJSON(map[string]interface{}{
		"dc":            dcDisp,
		"pool_id":       poolID,
		"extension_key": extensionKey,
		"count":         len(resp.Returnval),
		"allocations":   resp.Returnval,
	})
}

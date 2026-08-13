package tools

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// registerTenantTools is the "TenantManager" slice of Fase 7's Grupo C
// (".workspace/plans/MCPVMWare2026-08-10-175300-plano-cobertura-completa-api-codegen.plan.md")
// — object.TenantManager (all 3 methods: MarkServiceProviderEntities,
// RetrieveServiceProviderEntities, UnmarkServiceProviderEntities),
// hand-transcribed from the real referencia/govmomi/object/tenant_manager.go
// source. This is the multi-tenant entity-marking API used by integrations
// like VMware Cloud Director to flag which managed entities in this
// vCenter's inventory belong to the "service provider" (vs. tenant-owned)
// side of a shared deployment — modeVCenterOnly per gen/classification.json
// (already correct since Fase 0, per the brief) and confirmed independently
// here: types.ServiceContent.TenantManager (vim25/types/types.go) is a
// `*ManagedObjectReference` populated only in
// referencia/govmomi/simulator/vpx/service_content.go, never set (so nil) in
// esx/service_content.go — same nil-pointer-panic risk class as
// ExtensionManager/CustomizationSpecManager the brief already flagged.
//
// Nil-safety: object.NewTenantManager dereferences *c.ServiceContent.
// TenantManager directly with no guard of its own (confirmed by reading
// object/tenant_manager.go — same situation as CustomizationSpecManager, not
// ExtensionManager's ready-made GetExtensionManager). requireTenantManager
// below adds the explicit nil-check this project already established for
// this exact bug class (generated_vm_provisioning.go's
// requireProvisioningChecker precedent) — registry.go's CallTool recover()
// must not be the only safety net.
//
// Entity identification: all 3 methods operate on arbitrary managed entities
// (types.ManagedObjectReference — could be a VirtualMachine, ResourcePool,
// Folder, Datacenter, ... whatever a service provider wants to flag), the
// exact same "any entity" shape generated_authorization.go's
// RetrieveEntityPermissions/SetEntityPermissions/generated_custom_fields.go's
// Set already need. Rather than duplicate a resolver, this file reuses
// resolveEntityRefs/resolveEntityRef (defined in generated_authorization.go,
// wrapping object.SearchIndex.FindByInventoryPath — confirmed genuinely
// implemented server-side by vcsim per that file's own top doc comment) for
// the "entity_paths" input argument, and referenceInventoryPath (defined in
// generated_resourcepool_vapp.go, wrapping find.InventoryPath — already
// merged/stable from Fase 6, unlike the other two helpers this file also
// leans on which land in this same parallel batch) for
// vmware_tenant_retrieve_service_provider_entities' output, so every tool in
// this project that surfaces or accepts a managed entity does so via the
// same inventory-path convention, never a raw {type,value} moref blob —
// CROSS-GROUP COMPILE DEPENDENCY the orchestrator should be aware of when
// integrating: this file will not build unless generated_authorization.go
// (or an equivalent resolveEntityRefs/resolveEntityRef) is present in the
// same package. If that file is dropped for any reason, this file needs its
// own fallback resolver added before it can compile.
//
// Real vcsim server-side support, confirmed by reading
// referencia/govmomi/simulator/tenant_manager.go directly (not assumed from
// the object-package method name): ALL 3 methods have genuine
// simulator.TenantManager receiver methods (MarkServiceProviderEntities/
// UnmarkServiceProviderEntities maintain an in-memory set,
// RetrieveServiceProviderEntities reads it back) — every tool below gets a
// real functional test exercising mark -> retrieve -> unmark -> retrieve
// end to end, no assertReachesServer fallback needed anywhere in this file.
func registerTenantTools(r *Registry) {
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}
	entityPathsArg := map[string]interface{}{
		"type":        "array",
		"description": `Inventory paths of the managed entities to mark/unmark as service-provider-owned, e.g. ["/DC0/vm/my-vm", "/DC0/host/esx1"]. Any managed entity type is accepted (VirtualMachine, ResourcePool, Folder, Datacenter, HostSystem, ...) — resolved the same "any entity, by inventory path" way as vmware_authorization_set_entity_permissions (see this file's top doc comment).`,
		"items":       map[string]interface{}{"type": "string"},
	}

	// --- read-only -----------------------------------------------------

	r.register("vmware_tenant_retrieve_service_provider_entities",
		"List every managed entity currently marked as service-provider-owned on this connection.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		Tool{Handler: handleTenantRetrieveServiceProviderEntities},
	)

	// --- Tier 2 (disruptive but reversible) -----------------------------

	r.registerDestructive("vmware_tenant_mark_service_provider_entities",
		"Mark one or more managed entities as service-provider-owned (vs. tenant-owned), for multi-tenant integrations like VMware Cloud Director. Reversible via vmware_tenant_unmark_service_provider_entities.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"entity_paths": entityPathsArg,
				"confirm":      confirmArg,
			},
			"required": []interface{}{"entity_paths", "confirm"},
		},
		Tool{Handler: handleTenantMarkServiceProviderEntities},
	)

	r.registerDestructive("vmware_tenant_unmark_service_provider_entities",
		"Remove the service-provider-owned mark from one or more managed entities. Reversible via vmware_tenant_mark_service_provider_entities.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"entity_paths": entityPathsArg,
				"confirm":      confirmArg,
			},
			"required": []interface{}{"entity_paths", "confirm"},
		},
		Tool{Handler: handleTenantUnmarkServiceProviderEntities},
	)
}

// requireTenantManager is the explicit nil-guard for object.NewTenantManager
// — see this file's top doc comment for why this file needs its own (unlike
// generated_extension.go, which reuses object.GetExtensionManager's
// ready-made guard).
func requireTenantManager(client *vmware.Client) (*object.TenantManager, error) {
	vc := client.Client.Client
	if vc.ServiceContent.TenantManager == nil {
		return nil, fmt.Errorf("tenant-manager tools are not available on this connection (TenantManager requires vCenter — it is not present on a standalone ESXi host's ServiceContent)")
	}
	return object.NewTenantManager(vc), nil
}

// tenantEntityResults resolves each ref in refs to its inventory path via
// referenceInventoryPath (generated_resourcepool_vapp.go), degrading
// gracefully per-entry instead of failing the whole call — a marked entity
// may have been deleted since it was marked (TenantManager's in-memory
// mark/unmark set has no lifecycle tie to the entity's own existence, real
// vSphere included), which would otherwise turn a successful "list what's
// marked" read into a hard error over one stale reference.
func tenantEntityResults(ctx context.Context, client *vmware.Client, refs []types.ManagedObjectReference) []map[string]interface{} {
	results := make([]map[string]interface{}, 0, len(refs))
	for _, ref := range refs {
		entry := map[string]interface{}{"type": ref.Type, "value": ref.Value}
		if path, err := referenceInventoryPath(ctx, client, ref); err == nil {
			entry["inventory_path"] = path
		} else {
			entry["inventory_path"] = ""
			entry["resolve_error"] = err.Error()
		}
		results = append(results, entry)
	}
	return results
}

func handleTenantRetrieveServiceProviderEntities(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	tm, err := requireTenantManager(client)
	if err != nil {
		return "", err
	}

	refs, err := tm.RetrieveServiceProviderEntities(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve service provider entities: %w", err)
	}

	entities := tenantEntityResults(ctx, client, refs)
	return marshalJSON(map[string]interface{}{"count": len(entities), "entities": entities})
}

func handleTenantMarkServiceProviderEntities(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	tm, err := requireTenantManager(client)
	if err != nil {
		return "", err
	}
	paths, err := toStringSlice(args["entity_paths"])
	if err != nil {
		return "", fmt.Errorf("invalid entity_paths: %w", err)
	}
	refs, err := resolveEntityRefs(ctx, client, paths)
	if err != nil {
		return "", err
	}

	if err := tm.MarkServiceProviderEntities(ctx, refs); err != nil {
		return "", fmt.Errorf("failed to mark service provider entities: %w", err)
	}

	return marshalJSON(map[string]interface{}{
		"entity_paths": paths,
		"count":        len(refs),
		"result":       "marked",
	})
}

func handleTenantUnmarkServiceProviderEntities(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	tm, err := requireTenantManager(client)
	if err != nil {
		return "", err
	}
	paths, err := toStringSlice(args["entity_paths"])
	if err != nil {
		return "", fmt.Errorf("invalid entity_paths: %w", err)
	}
	refs, err := resolveEntityRefs(ctx, client, paths)
	if err != nil {
		return "", err
	}

	if err := tm.UnmarkServiceProviderEntities(ctx, refs); err != nil {
		return "", fmt.Errorf("failed to unmark service provider entities: %w", err)
	}

	return marshalJSON(map[string]interface{}{
		"entity_paths": paths,
		"count":        len(refs),
		"result":       "unmarked",
	})
}

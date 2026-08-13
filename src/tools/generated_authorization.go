package tools

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// registerAuthorizationTools is Fase 7 (Grupo B) of the codegen plan
// (".workspace/plans/MCPVMWare2026-08-10-175300-plano-cobertura-completa-api-codegen.plan.md")
// — object.AuthorizationManager, hand-transcribed from
// referencia/govmomi/object/authorization_manager.go and
// authorization_manager_internal.go following the generated_network.go/
// generated_vm_provisioning.go conventions. 14 tools total.
//
// mode=vsphere-general (not vcenter-only): confirmed by reading
// referencia/govmomi/simulator/esx/service_content.go and
// referencia/govmomi/simulator/vpx/service_content.go directly —
// ServiceContent.AuthorizationManager is a non-nil *types.ManagedObjectReference
// in BOTH templates ("ha-authmgr" on ESX, "AuthorizationManager" on VPX), so
// object.NewAuthorizationManager (which dereferences that field) never
// panics on a standalone ESXi host. This is the opposite finding from the
// same risk class already found for CustomFieldsManager below (nil on ESX)
// and, in Fase 2, for VmProvisioningChecker/VmCompatibilityChecker — no
// nil-guard is needed here, unlike those.
//
// Curation deviations from the raw src/gen/classification.json (human
// review before generating, same rigor as every prior fase):
//
//   - vmware_authorization_role_list (RoleList): the AST classifier tagged
//     this tier2 (fail-safe default — no name pattern matched "list"),
//     corrected to read-only here. Confirmed by reading the real source:
//     RoleList only calls m.Properties(ctx, m.Reference(), []string{"roleList"},
//     &am) — a pure property-collector read, the identical pattern already
//     corrected repeatedly since the Fase 0 review (OpaqueNetwork.Summary in
//     Fase 5, VirtualMachine.BootOptions/Device/... in Fase 2).
//
//   - vmware_authorization_disable_methods / vmware_authorization_enable_methods
//     (DisableMethods/EnableMethods, from authorization_manager_internal.go):
//     these target the "urn:internalvim25" SOAP namespace, not the public
//     "urn:vim25" one — confirmed by reading the raw XML body types in the
//     source (disableMethodsBody/enableMethodsBody's `xml:"urn:internalvim25 ..."`
//     tags). This is an INTERNAL, UNDOCUMENTED vCenter API — used internally
//     by vCenter itself (e.g. to lock out concurrent operations on an entity
//     during Storage vMotion), not published as part of the supported
//     vSphere API surface, and may change or disappear without notice
//     between vSphere versions. Registered anyway per this project's 100%
//     coverage goal — both tool descriptions below carry an explicit warning
//     — but treat these two as higher-risk than the other 12. vcsim has NO
//     handler for either SOAP operation (confirmed: neither name appears
//     anywhere under referencia/govmomi/simulator, unlike every other method
//     in this file) — registered per real vSphere support, tested only for
//     registration/arg-validation/tier-gating/"reaches the server cleanly"
//     against vcsim, same "vcsim gap, not a bug" pattern as prior fases
//     (e.g. generated_network.go's ReconfigureDVPort/UpdateDVSLacpGroupConfig).
//
// Entity resolution: RetrieveEntityPermissions/RemoveEntityPermission/
// SetEntityPermissions/DisableMethods/EnableMethods all operate on an
// arbitrary managed entity (Folder, Datacenter, ComputeResource,
// ResourcePool, VirtualMachine, HostSystem, ...) identified by a
// types.ManagedObjectReference — unlike every resolveX helper elsewhere in
// this package (resolveVM/resolveHost/resolveDatastore/resolveNetworkRef),
// which are each scoped to exactly one entity kind via find.Finder's
// per-kind methods, there is no single Finder method that resolves "any
// entity". object.SearchIndex.FindByInventoryPath is the one govmomi API
// that does — resolveEntityRef/resolveEntityRefs below wrap it, taking a
// full inventory path (e.g. "/DC0", "/DC0/vm/my-vm", "/DC0/host/esx1") the
// same way every other tool in this project identifies inventory objects.
// Confirmed genuinely implemented server-side by vcsim (not a stub): see
// referencia/govmomi/simulator/search_index.go's FindByInventoryPath.
// generated_custom_fields.go's CustomFieldsManager.Set also needs this exact
// same "any entity" resolution — since both files are written together in
// this same batch (no registry.go/mode_test.go dependency between them),
// the helpers are defined once here and reused from that file, rather than
// duplicated (same "reuse, don't duplicate" discipline as dcScopedPath in
// finder.go being shared by every resolveX helper in the package).
//
// vcsim support confirmed by the real SOAP method name (not receiver name),
// same discipline as every prior fase: every method except DisableMethods/
// EnableMethods (see above) has a genuine simulator.AuthorizationManager
// handler in referencia/govmomi/simulator/authorization_manager.go.
func registerAuthorizationTools(r *Registry) {
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}
	entityPathArg := map[string]interface{}{
		"type":        "string",
		"description": `Full inventory path of the managed entity (e.g. "/DC0", "/DC0/vm/my-vm", "/DC0/host/esx1.example.com"), resolved via SearchIndex.FindByInventoryPath — works for any entity kind (Folder, Datacenter, ComputeResource, ResourcePool, VirtualMachine, HostSystem, etc.), not just VMs/hosts.`,
	}
	entityPathsArg := map[string]interface{}{
		"type":        "array",
		"items":       map[string]interface{}{"type": "string"},
		"description": "One or more full inventory paths, each resolved the same way as a single entity_path argument (see its description). Must be non-empty.",
	}
	privilegeIDsArg := map[string]interface{}{
		"type":        "array",
		"items":       map[string]interface{}{"type": "string"},
		"description": `Privilege IDs, e.g. "System.View", "VirtualMachine.Interact.PowerOn".`,
	}

	r.registerDestructive("vmware_authorization_add_role",
		"Create a new authorization role with the given set of privilege IDs.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name":          map[string]interface{}{"type": "string", "description": "Name of the new role."},
				"privilege_ids": privilegeIDsArg,
				"confirm":       confirmArg,
			},
			"required": []interface{}{"name", "confirm"},
		},
		Tool{Handler: handleAuthorizationAddRole},
	)

	r.registerDestructive("vmware_authorization_disable_methods",
		"Disable a set of methods on one or more entities, marking them locked with a reason. INTERNAL/UNDOCUMENTED vCenter API (urn:internalvim25 namespace) — used internally by vCenter itself (e.g. to lock out concurrent operations on an entity during Storage vMotion), not part of the public/supported vSphere API surface, and subject to change or removal without notice between vSphere versions. Use with caution. Not implemented by vcsim (registration/validation only against the simulator).",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"entity_paths": entityPathsArg,
				"methods": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"method": map[string]interface{}{"type": "string", "description": `Name of the method to disable, e.g. "PowerOnVM_Task".`},
							"reason": map[string]interface{}{"type": "string", "description": "Reason ID recorded for the lock."},
						},
						"required": []interface{}{"method"},
					},
					"description": "Methods to disable, each as {method, reason}. Must be non-empty.",
				},
				"source":  map[string]interface{}{"type": "string", "description": "Identifier of the caller/source requesting the lock."},
				"confirm": confirmArg,
			},
			"required": []interface{}{"entity_paths", "methods", "source", "confirm"},
		},
		Tool{Handler: handleAuthorizationDisableMethods},
	)

	r.registerDestructive("vmware_authorization_enable_methods",
		"Re-enable a set of previously disabled methods on one or more entities — counterpart to vmware_authorization_disable_methods. INTERNAL/UNDOCUMENTED vCenter API (urn:internalvim25 namespace), not part of the public/supported vSphere API surface, and subject to change or removal without notice between vSphere versions. Use with caution. Not implemented by vcsim (registration/validation only against the simulator).",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"entity_paths": entityPathsArg,
				"methods": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": `Names of the methods to re-enable, e.g. ["PowerOnVM_Task"]. Must be non-empty.`,
				},
				"source":  map[string]interface{}{"type": "string", "description": "Identifier of the caller/source that requested the original lock."},
				"confirm": confirmArg,
			},
			"required": []interface{}{"entity_paths", "methods", "source", "confirm"},
		},
		Tool{Handler: handleAuthorizationEnableMethods},
	)

	r.register("vmware_authorization_fetch_user_privilege_on_entities",
		"Fetch the effective privileges a user has on one or more entities.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"entity_paths": entityPathsArg,
				"user_name":    map[string]interface{}{"type": "string", "description": `User or group name, e.g. "VSPHERE.LOCAL\\Administrator".`},
			},
			"required": []interface{}{"entity_paths", "user_name"},
		},
		Tool{Handler: handleAuthorizationFetchUserPrivilegeOnEntities},
	)

	r.register("vmware_authorization_has_privilege_on_entity",
		"Check whether a set of privileges is granted on a single entity for a given session.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"entity_path":   entityPathArg,
				"session_id":    map[string]interface{}{"type": "string", "description": "Session ID to check against. Omit to check the connection's own session."},
				"privilege_ids": privilegeIDsArg,
			},
			"required": []interface{}{"entity_path", "privilege_ids"},
		},
		Tool{Handler: handleAuthorizationHasPrivilegeOnEntity},
	)

	r.register("vmware_authorization_has_user_privilege_on_entities",
		"Check whether a user has a set of privileges on one or more entities.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"entity_paths":  entityPathsArg,
				"user_name":     map[string]interface{}{"type": "string", "description": "User or group name to check."},
				"privilege_ids": privilegeIDsArg,
			},
			"required": []interface{}{"entity_paths", "user_name", "privilege_ids"},
		},
		Tool{Handler: handleAuthorizationHasUserPrivilegeOnEntities},
	)

	r.registerDestructive("vmware_authorization_remove_entity_permission",
		"Remove a permission (role assignment) for a user or group on an entity. Irreversible — the removed permission must be re-added via vmware_authorization_set_entity_permissions if needed.",
		tier1,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"entity_path": entityPathArg,
				"user":        map[string]interface{}{"type": "string", "description": "User or group name whose permission is removed."},
				"is_group":    map[string]interface{}{"type": "boolean", "description": `True if "user" identifies a group, false if it identifies an individual user.`},
				"confirm":     confirmArg,
			},
			"required": []interface{}{"entity_path", "user", "is_group", "confirm"},
		},
		Tool{Handler: handleAuthorizationRemoveEntityPermission},
	)

	r.registerDestructive("vmware_authorization_remove_role",
		"Remove an authorization role by ID. Irreversible.",
		tier1,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"role_id":      map[string]interface{}{"type": "integer", "description": "ID of the role to remove."},
				"fail_if_used": map[string]interface{}{"type": "boolean", "description": "If true, the call fails when the role is still assigned to any permission. If false, the role is force-removed even if in use (also removing those permission assignments) — required explicitly (no default) so a caller cannot land on the more dangerous behavior by omission."},
				"confirm":      confirmArg,
			},
			"required": []interface{}{"role_id", "fail_if_used", "confirm"},
		},
		Tool{Handler: handleAuthorizationRemoveRole},
	)

	r.register("vmware_authorization_retrieve_all_permissions",
		"Retrieve every permission (role assignment) defined anywhere in the inventory.",
		map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
		Tool{Handler: handleAuthorizationRetrieveAllPermissions},
	)

	r.register("vmware_authorization_retrieve_entity_permissions",
		"Retrieve the permissions (role assignments) defined directly on an entity, optionally including permissions inherited from its ancestors.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"entity_path": entityPathArg,
				"inherited":   map[string]interface{}{"type": "boolean", "description": "If true, also include permissions inherited from ancestor entities. Default false."},
			},
			"required": []interface{}{"entity_path"},
		},
		Tool{Handler: handleAuthorizationRetrieveEntityPermissions},
	)

	r.register("vmware_authorization_retrieve_role_permissions",
		"Retrieve every permission (entity assignment) that uses a given role.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"role_id": map[string]interface{}{"type": "integer", "description": "ID of the role."}},
			"required":   []interface{}{"role_id"},
		},
		Tool{Handler: handleAuthorizationRetrieveRolePermissions},
	)

	r.register("vmware_authorization_role_list",
		"List every authorization role defined on this connection (both built-in system roles and custom roles).",
		map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
		Tool{Handler: handleAuthorizationRoleList},
	)

	r.registerDestructive("vmware_authorization_set_entity_permissions",
		"Set (replace) the permissions defined directly on an entity.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"entity_path": entityPathArg,
				"permissions": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "object"},
					"description": `Array of types.Permission JSON objects (e.g. {"principal": "user1", "group": false, "roleId": -1, "propagate": true}). Any "entity" field present in an item is ignored — the entity is always the one identified by entity_path.`,
				},
				"confirm": confirmArg,
			},
			"required": []interface{}{"entity_path", "permissions", "confirm"},
		},
		Tool{Handler: handleAuthorizationSetEntityPermissions},
	)

	r.registerDestructive("vmware_authorization_update_role",
		"Rename a role and/or replace its set of privilege IDs.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"role_id": map[string]interface{}{"type": "integer", "description": "ID of the role to update."},
				"name":    map[string]interface{}{"type": "string", "description": "New name for the role."},
				"privilege_ids": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "New set of privilege IDs. Omit or pass an empty array to leave the role's privileges unchanged (confirmed against vcsim's simulator.AuthorizationManager.UpdateAuthorizationRole; verify this against a real vCenter before relying on it there too).",
				},
				"confirm": confirmArg,
			},
			"required": []interface{}{"role_id", "name", "confirm"},
		},
		Tool{Handler: handleAuthorizationUpdateRole},
	)
}

// authorizationManager returns an object.AuthorizationManager bound to
// client's underlying vim25 client. No nil-guard needed — see this file's
// top doc comment for why AuthorizationManager is safe on both ESXi and
// vCenter, unlike CustomFieldsManager below.
func authorizationManager(client *vmware.Client) *object.AuthorizationManager {
	return object.NewAuthorizationManager(client.Client.Client)
}

// resolveEntityRef resolves path to the types.ManagedObjectReference of the
// managed entity at that inventory path — see this file's top doc comment
// ("Entity resolution") for why object.SearchIndex.FindByInventoryPath is
// used instead of one of find.Finder's per-kind methods. Shared by this file
// and generated_custom_fields.go.
func resolveEntityRef(ctx context.Context, client *vmware.Client, path string) (types.ManagedObjectReference, error) {
	if path == "" {
		return types.ManagedObjectReference{}, fmt.Errorf("entity_path is required")
	}
	si := object.NewSearchIndex(client.Client.Client)
	ref, err := si.FindByInventoryPath(ctx, path)
	if err != nil {
		return types.ManagedObjectReference{}, fmt.Errorf("failed to resolve entity %q: %w", path, err)
	}
	if ref == nil {
		return types.ManagedObjectReference{}, fmt.Errorf("no managed entity found at inventory path %q", path)
	}
	return ref.Reference(), nil
}

// resolveEntityRefs resolves each of paths via resolveEntityRef, in order.
func resolveEntityRefs(ctx context.Context, client *vmware.Client, paths []string) ([]types.ManagedObjectReference, error) {
	if len(paths) == 0 {
		return nil, fmt.Errorf("entity_paths is required and must be non-empty")
	}
	refs := make([]types.ManagedObjectReference, 0, len(paths))
	for i, p := range paths {
		ref, err := resolveEntityRef(ctx, client, p)
		if err != nil {
			return nil, fmt.Errorf("entity_paths[%d]: %w", i, err)
		}
		refs = append(refs, ref)
	}
	return refs, nil
}

func handleAuthorizationAddRole(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	name, _ := args["name"].(string)
	if name == "" {
		return "", fmt.Errorf("name is required")
	}
	var ids []string
	if raw, ok := args["privilege_ids"]; ok && raw != nil {
		var err error
		ids, err = toStringSlice(raw)
		if err != nil {
			return "", fmt.Errorf("invalid privilege_ids: %w", err)
		}
	}

	roleID, err := authorizationManager(client).AddRole(ctx, name, ids)
	if err != nil {
		return "", fmt.Errorf("failed to add role %q: %w", name, err)
	}

	return marshalJSON(map[string]interface{}{"result": "role_added", "name": name, "role_id": roleID})
}

func handleAuthorizationDisableMethods(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	paths, err := toStringSlice(args["entity_paths"])
	if err != nil {
		return "", fmt.Errorf("invalid entity_paths: %w", err)
	}
	entities, err := resolveEntityRefs(ctx, client, paths)
	if err != nil {
		return "", err
	}

	rawMethods, ok := args["methods"].([]interface{})
	if !ok || len(rawMethods) == 0 {
		return "", fmt.Errorf("methods is required and must be a non-empty array")
	}
	methodReqs := make([]object.DisabledMethodRequest, 0, len(rawMethods))
	for i, item := range rawMethods {
		var m object.DisabledMethodRequest
		if err := decodeJSONArg(item, &m); err != nil {
			return "", fmt.Errorf("invalid methods[%d]: %w", i, err)
		}
		if m.Method == "" {
			return "", fmt.Errorf("methods[%d].method is required", i)
		}
		methodReqs = append(methodReqs, m)
	}

	source, _ := args["source"].(string)
	if source == "" {
		return "", fmt.Errorf("source is required")
	}

	if err := authorizationManager(client).DisableMethods(ctx, entities, methodReqs, source); err != nil {
		return "", fmt.Errorf("failed to disable methods: %w", err)
	}

	return marshalJSON(map[string]interface{}{"result": "methods_disabled", "entity_count": len(entities), "method_count": len(methodReqs)})
}

func handleAuthorizationEnableMethods(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	paths, err := toStringSlice(args["entity_paths"])
	if err != nil {
		return "", fmt.Errorf("invalid entity_paths: %w", err)
	}
	entities, err := resolveEntityRefs(ctx, client, paths)
	if err != nil {
		return "", err
	}

	methodNames, err := toStringSlice(args["methods"])
	if err != nil {
		return "", fmt.Errorf("invalid methods: %w", err)
	}
	if len(methodNames) == 0 {
		return "", fmt.Errorf("methods is required and must be a non-empty array")
	}

	source, _ := args["source"].(string)
	if source == "" {
		return "", fmt.Errorf("source is required")
	}

	if err := authorizationManager(client).EnableMethods(ctx, entities, methodNames, source); err != nil {
		return "", fmt.Errorf("failed to enable methods: %w", err)
	}

	return marshalJSON(map[string]interface{}{"result": "methods_enabled", "entity_count": len(entities), "method_count": len(methodNames)})
}

func handleAuthorizationFetchUserPrivilegeOnEntities(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	paths, err := toStringSlice(args["entity_paths"])
	if err != nil {
		return "", fmt.Errorf("invalid entity_paths: %w", err)
	}
	entities, err := resolveEntityRefs(ctx, client, paths)
	if err != nil {
		return "", err
	}
	userName, _ := args["user_name"].(string)
	if userName == "" {
		return "", fmt.Errorf("user_name is required")
	}

	results, err := authorizationManager(client).FetchUserPrivilegeOnEntities(ctx, entities, userName)
	if err != nil {
		return "", fmt.Errorf("failed to fetch user privileges for %q: %w", userName, err)
	}

	return marshalJSON(map[string]interface{}{"user_name": userName, "count": len(results), "results": results})
}

func handleAuthorizationHasPrivilegeOnEntity(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	path, _ := args["entity_path"].(string)
	entity, err := resolveEntityRef(ctx, client, path)
	if err != nil {
		return "", err
	}
	sessionID, _ := args["session_id"].(string)
	privIDs, err := toStringSlice(args["privilege_ids"])
	if err != nil {
		return "", fmt.Errorf("invalid privilege_ids: %w", err)
	}
	if len(privIDs) == 0 {
		return "", fmt.Errorf("privilege_ids is required and must be non-empty")
	}

	granted, err := authorizationManager(client).HasPrivilegeOnEntity(ctx, entity, sessionID, privIDs)
	if err != nil {
		return "", fmt.Errorf("failed to check privileges on %s: %w", path, err)
	}

	return marshalJSON(map[string]interface{}{"entity_path": path, "privilege_ids": privIDs, "granted": granted})
}

func handleAuthorizationHasUserPrivilegeOnEntities(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	paths, err := toStringSlice(args["entity_paths"])
	if err != nil {
		return "", fmt.Errorf("invalid entity_paths: %w", err)
	}
	entities, err := resolveEntityRefs(ctx, client, paths)
	if err != nil {
		return "", err
	}
	userName, _ := args["user_name"].(string)
	if userName == "" {
		return "", fmt.Errorf("user_name is required")
	}
	privIDs, err := toStringSlice(args["privilege_ids"])
	if err != nil {
		return "", fmt.Errorf("invalid privilege_ids: %w", err)
	}
	if len(privIDs) == 0 {
		return "", fmt.Errorf("privilege_ids is required and must be non-empty")
	}

	results, err := authorizationManager(client).HasUserPrivilegeOnEntities(ctx, entities, userName, privIDs)
	if err != nil {
		return "", fmt.Errorf("failed to check user privileges for %q: %w", userName, err)
	}

	return marshalJSON(map[string]interface{}{"user_name": userName, "count": len(results), "results": results})
}

func handleAuthorizationRemoveEntityPermission(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	path, _ := args["entity_path"].(string)
	entity, err := resolveEntityRef(ctx, client, path)
	if err != nil {
		return "", err
	}
	user, _ := args["user"].(string)
	if user == "" {
		return "", fmt.Errorf("user is required")
	}
	isGroup, _ := args["is_group"].(bool)

	if err := authorizationManager(client).RemoveEntityPermission(ctx, entity, user, isGroup); err != nil {
		return "", fmt.Errorf("failed to remove permission for %q on %s: %w", user, path, err)
	}

	return marshalJSON(map[string]interface{}{"result": "permission_removed", "entity_path": path, "user": user, "is_group": isGroup})
}

func handleAuthorizationRemoveRole(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	roleIDRaw, ok := args["role_id"]
	if !ok {
		return "", fmt.Errorf("role_id is required")
	}
	roleID, err := toInt32(roleIDRaw)
	if err != nil {
		return "", fmt.Errorf("invalid role_id: %w", err)
	}
	failIfUsed, ok := args["fail_if_used"].(bool)
	if !ok {
		return "", fmt.Errorf("fail_if_used is required")
	}

	if err := authorizationManager(client).RemoveRole(ctx, roleID, failIfUsed); err != nil {
		return "", fmt.Errorf("failed to remove role %d: %w", roleID, err)
	}

	return marshalJSON(map[string]interface{}{"result": "role_removed", "role_id": roleID})
}

func handleAuthorizationRetrieveAllPermissions(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	perms, err := authorizationManager(client).RetrieveAllPermissions(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve all permissions: %w", err)
	}
	return marshalJSON(map[string]interface{}{"count": len(perms), "permissions": perms})
}

func handleAuthorizationRetrieveEntityPermissions(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	path, _ := args["entity_path"].(string)
	entity, err := resolveEntityRef(ctx, client, path)
	if err != nil {
		return "", err
	}
	inherited, _ := args["inherited"].(bool)

	perms, err := authorizationManager(client).RetrieveEntityPermissions(ctx, entity, inherited)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve permissions for %s: %w", path, err)
	}

	return marshalJSON(map[string]interface{}{"entity_path": path, "count": len(perms), "permissions": perms})
}

func handleAuthorizationRetrieveRolePermissions(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	roleIDRaw, ok := args["role_id"]
	if !ok {
		return "", fmt.Errorf("role_id is required")
	}
	roleID, err := toInt32(roleIDRaw)
	if err != nil {
		return "", fmt.Errorf("invalid role_id: %w", err)
	}

	perms, err := authorizationManager(client).RetrieveRolePermissions(ctx, roleID)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve permissions for role %d: %w", roleID, err)
	}

	return marshalJSON(map[string]interface{}{"role_id": roleID, "count": len(perms), "permissions": perms})
}

func handleAuthorizationRoleList(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	roles, err := authorizationManager(client).RoleList(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to list roles: %w", err)
	}
	return marshalJSON(map[string]interface{}{"count": len(roles), "roles": roles})
}

func handleAuthorizationSetEntityPermissions(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	path, _ := args["entity_path"].(string)
	entity, err := resolveEntityRef(ctx, client, path)
	if err != nil {
		return "", err
	}

	rawPerms, ok := args["permissions"].([]interface{})
	if !ok || len(rawPerms) == 0 {
		return "", fmt.Errorf("permissions is required and must be a non-empty array")
	}
	perms := make([]types.Permission, 0, len(rawPerms))
	for i, item := range rawPerms {
		var p types.Permission
		if err := decodeJSONArg(item, &p); err != nil {
			return "", fmt.Errorf("invalid permissions[%d]: %w", i, err)
		}
		perms = append(perms, p)
	}

	if err := authorizationManager(client).SetEntityPermissions(ctx, entity, perms); err != nil {
		return "", fmt.Errorf("failed to set permissions on %s: %w", path, err)
	}

	return marshalJSON(map[string]interface{}{"result": "permissions_set", "entity_path": path, "count": len(perms)})
}

func handleAuthorizationUpdateRole(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	roleIDRaw, ok := args["role_id"]
	if !ok {
		return "", fmt.Errorf("role_id is required")
	}
	roleID, err := toInt32(roleIDRaw)
	if err != nil {
		return "", fmt.Errorf("invalid role_id: %w", err)
	}
	name, _ := args["name"].(string)
	if name == "" {
		return "", fmt.Errorf("name is required")
	}
	var ids []string
	if raw, ok := args["privilege_ids"]; ok && raw != nil {
		ids, err = toStringSlice(raw)
		if err != nil {
			return "", fmt.Errorf("invalid privilege_ids: %w", err)
		}
	}

	if err := authorizationManager(client).UpdateRole(ctx, roleID, name, ids); err != nil {
		return "", fmt.Errorf("failed to update role %d: %w", roleID, err)
	}

	return marshalJSON(map[string]interface{}{"result": "role_updated", "role_id": roleID, "name": name})
}

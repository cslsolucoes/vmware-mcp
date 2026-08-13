package tools

import (
	"context"
	"errors"
	"fmt"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// registerCustomFieldsTools is Fase 7 (Grupo B) of the codegen plan
// (".workspace/plans/MCPVMWare2026-08-10-175300-plano-cobertura-completa-api-codegen.plan.md")
// — object.CustomFieldsManager, hand-transcribed from
// referencia/govmomi/object/custom_fields_manager.go following the
// generated_authorization.go/generated_vm_provisioning.go conventions.
// 6 tools total.
//
// mode=vcenter-only: confirmed (not re-derived here — see this group's
// brief) by reading referencia/govmomi/simulator/esx/service_content.go
// (ServiceContent.CustomFieldsManager is a nil *types.ManagedObjectReference)
// vs. referencia/govmomi/simulator/vpx/service_content.go (non-nil) —
// object.NewCustomFieldsManager dereferences that field directly and would
// panic against a standalone ESXi host. The correction already applied
// upstream in src/gen/classification.json before this group started (was
// wrongly classified vsphere-general by the raw AST pass) — the same
// nil-pointer risk class as VmProvisioningChecker/VmCompatibilityChecker
// (Fase 2).
//
// Nil-safety: every handler below resolves the manager via
// requireCustomFieldsManager, which calls object.GetCustomFieldsManager
// (not object.NewCustomFieldsManager directly) — that helper already
// contains the nil check and returns object.ErrNotSupported instead of
// panicking, confirmed by reading the source. requireCustomFieldsManager
// wraps it with a clearer, tool-specific error message, same nil-guard
// discipline as generated_vm_provisioning.go's requireProvisioningChecker/
// requireCompatibilityChecker — not relying solely on registry.go's
// CallTool recover() as defense-in-depth. Proven by
// TestCustomFieldsTools_NilGuardAgainstESX below (a real call against
// simulator.ESX(), not just a code-reading argument).
//
// Curation deviations from the raw src/gen/classification.json (human
// review before generating; the Add/Remove/Rename/Set/FindKey
// classifications were already correct and are kept as-is):
//
//   - vmware_custom_field_field (Field): the AST classifier tagged this
//     tier2 (fail-safe default), corrected to read-only here. Confirmed by
//     reading the real source: Field only calls m.Properties(ctx,
//     m.Reference(), []string{"field"}, &fm) — a pure property-collector
//     read, the same pattern already corrected for
//     vmware_authorization_role_list in generated_authorization.go (and
//     repeatedly since the Fase 0 review).
//
// Entity resolution: CustomFieldsManager.Set applies a custom field value to
// an arbitrary managed entity (any type a custom field can be defined
// against), so vmware_custom_field_set reuses resolveEntityRef from
// generated_authorization.go (object.SearchIndex.FindByInventoryPath-backed
// — see that file's top doc comment, "Entity resolution", for why) instead
// of duplicating a second copy of the same helper.
//
// vcsim support confirmed by the real SOAP method name (not receiver name):
// referencia/govmomi/simulator/custom_fields_manager.go implements
// AddCustomFieldDef/RemoveCustomFieldDef/RenameCustomFieldDef/SetField in
// full — every one of this group's 6 tools has genuine server-side vcsim
// support (Field/FindKey are pure client-side property reads/lookups, not
// SOAP methods of their own, so they ride on the same property-collector
// support every other read-only accessor in this project already has).
func registerCustomFieldsTools(r *Registry) {
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}
	keyArg := map[string]interface{}{
		"type":        "integer",
		"description": "The custom field definition's key, as returned by vmware_custom_field_add or vmware_custom_field_field.",
	}

	r.registerDestructive("vmware_custom_field_add",
		"Define a new custom field, optionally scoped to one managed-object type.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{"type": "string", "description": "Name of the new custom field."},
				"managed_object_type": map[string]interface{}{
					"type":        "string",
					"description": `Managed-object type this field applies to, e.g. "VirtualMachine", "HostSystem". Omit or pass "" for a field that applies to every entity type.`,
				},
				"field_def_policy": map[string]interface{}{
					"type":        "object",
					"description": `Optional types.PrivilegePolicyDef JSON object ({"createPrivilege", "readPrivilege", "updatePrivilege", "deletePrivilege"}) controlling who can manage the field definition itself.`,
				},
				"field_policy": map[string]interface{}{
					"type":        "object",
					"description": "Optional types.PrivilegePolicyDef JSON object controlling who can read/write this field's value on entities.",
				},
				"confirm": confirmArg,
			},
			"required": []interface{}{"name", "confirm"},
		},
		Tool{Handler: handleCustomFieldAdd},
	)

	r.register("vmware_custom_field_field",
		"List every custom field definition visible on this connection.",
		map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
		Tool{Handler: handleCustomFieldField},
	)

	r.register("vmware_custom_field_find_key",
		"Resolve a custom field's key by name (falls back to parsing name itself as an integer key if no definition matches).",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"name": map[string]interface{}{"type": "string", "description": "Custom field name, or a literal key as a numeric string."}},
			"required":   []interface{}{"name"},
		},
		Tool{Handler: handleCustomFieldFindKey},
	)

	r.registerDestructive("vmware_custom_field_remove",
		"Remove a custom field definition, and its value from every entity that carries it. Irreversible.",
		tier1,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"key":     keyArg,
				"confirm": confirmArg,
			},
			"required": []interface{}{"key", "confirm"},
		},
		Tool{Handler: handleCustomFieldRemove},
	)

	r.registerDestructive("vmware_custom_field_rename",
		"Rename an existing custom field definition.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"key":     keyArg,
				"name":    map[string]interface{}{"type": "string", "description": "New name for the field."},
				"confirm": confirmArg,
			},
			"required": []interface{}{"key", "name", "confirm"},
		},
		Tool{Handler: handleCustomFieldRename},
	)

	r.registerDestructive("vmware_custom_field_set",
		"Set a custom field's value on a specific managed entity.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"entity_path": map[string]interface{}{
					"type":        "string",
					"description": `Full inventory path of the entity to set the value on (e.g. "/DC0/vm/my-vm"), resolved via SearchIndex.FindByInventoryPath — works for any entity kind.`,
				},
				"key":     keyArg,
				"value":   map[string]interface{}{"type": "string", "description": "New value for the field on this entity."},
				"confirm": confirmArg,
			},
			"required": []interface{}{"entity_path", "key", "value", "confirm"},
		},
		Tool{Handler: handleCustomFieldSet},
	)
}

// requireCustomFieldsManager resolves the CustomFieldsManager via
// object.GetCustomFieldsManager, which already nil-guards
// ServiceContent.CustomFieldsManager internally (returns
// object.ErrNotSupported instead of panicking) — see this file's top doc
// comment for why that guard exists (nil on standalone ESXi) and why it is
// used here explicitly instead of relying only on registry.go's CallTool
// recover(). client.Client.Client is this connection's *vim25.Client, same
// field path as generated_vm_provisioning.go's requireProvisioningChecker.
func requireCustomFieldsManager(client *vmware.Client) (*object.CustomFieldsManager, error) {
	m, err := object.GetCustomFieldsManager(client.Client.Client)
	if err != nil {
		if errors.Is(err, object.ErrNotSupported) {
			return nil, fmt.Errorf("custom fields are not available on this connection (CustomFieldsManager requires vCenter — ServiceContent.CustomFieldsManager is nil on a standalone ESXi host): %w", err)
		}
		return nil, err
	}
	return m, nil
}

func handleCustomFieldAdd(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := requireCustomFieldsManager(client)
	if err != nil {
		return "", err
	}
	name, _ := args["name"].(string)
	if name == "" {
		return "", fmt.Errorf("name is required")
	}
	moType, _ := args["managed_object_type"].(string)

	var fieldDefPolicy *types.PrivilegePolicyDef
	if args["field_def_policy"] != nil {
		fieldDefPolicy = &types.PrivilegePolicyDef{}
		if err := decodeJSONArg(args["field_def_policy"], fieldDefPolicy); err != nil {
			return "", fmt.Errorf("invalid field_def_policy: %w", err)
		}
	}
	var fieldPolicy *types.PrivilegePolicyDef
	if args["field_policy"] != nil {
		fieldPolicy = &types.PrivilegePolicyDef{}
		if err := decodeJSONArg(args["field_policy"], fieldPolicy); err != nil {
			return "", fmt.Errorf("invalid field_policy: %w", err)
		}
	}

	def, err := m.Add(ctx, name, moType, fieldDefPolicy, fieldPolicy)
	if err != nil {
		return "", fmt.Errorf("failed to add custom field %q: %w", name, err)
	}

	return marshalJSON(map[string]interface{}{"result": "custom_field_added", "field": def})
}

func handleCustomFieldField(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := requireCustomFieldsManager(client)
	if err != nil {
		return "", err
	}

	fields, err := m.Field(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to list custom fields: %w", err)
	}

	return marshalJSON(map[string]interface{}{"count": len(fields), "fields": fields})
}

func handleCustomFieldFindKey(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := requireCustomFieldsManager(client)
	if err != nil {
		return "", err
	}
	name, _ := args["name"].(string)
	if name == "" {
		return "", fmt.Errorf("name is required")
	}

	key, err := m.FindKey(ctx, name)
	if err != nil {
		return "", fmt.Errorf("failed to find key for %q: %w", name, err)
	}

	return marshalJSON(map[string]interface{}{"name": name, "key": key})
}

func handleCustomFieldRemove(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := requireCustomFieldsManager(client)
	if err != nil {
		return "", err
	}
	keyRaw, ok := args["key"]
	if !ok {
		return "", fmt.Errorf("key is required")
	}
	key, err := toInt32(keyRaw)
	if err != nil {
		return "", fmt.Errorf("invalid key: %w", err)
	}

	if err := m.Remove(ctx, key); err != nil {
		return "", fmt.Errorf("failed to remove custom field %d: %w", key, err)
	}

	return marshalJSON(map[string]interface{}{"result": "custom_field_removed", "key": key})
}

func handleCustomFieldRename(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := requireCustomFieldsManager(client)
	if err != nil {
		return "", err
	}
	keyRaw, ok := args["key"]
	if !ok {
		return "", fmt.Errorf("key is required")
	}
	key, err := toInt32(keyRaw)
	if err != nil {
		return "", fmt.Errorf("invalid key: %w", err)
	}
	name, _ := args["name"].(string)
	if name == "" {
		return "", fmt.Errorf("name is required")
	}

	if err := m.Rename(ctx, key, name); err != nil {
		return "", fmt.Errorf("failed to rename custom field %d: %w", key, err)
	}

	return marshalJSON(map[string]interface{}{"result": "custom_field_renamed", "key": key, "name": name})
}

func handleCustomFieldSet(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := requireCustomFieldsManager(client)
	if err != nil {
		return "", err
	}
	path, _ := args["entity_path"].(string)
	entity, err := resolveEntityRef(ctx, client, path)
	if err != nil {
		return "", err
	}
	keyRaw, ok := args["key"]
	if !ok {
		return "", fmt.Errorf("key is required")
	}
	key, err := toInt32(keyRaw)
	if err != nil {
		return "", fmt.Errorf("invalid key: %w", err)
	}
	value, _ := args["value"].(string)

	if err := m.Set(ctx, entity, key, value); err != nil {
		return "", fmt.Errorf("failed to set custom field %d on %s: %w", key, path, err)
	}

	return marshalJSON(map[string]interface{}{"result": "custom_field_set", "entity_path": path, "key": key, "value": value})
}

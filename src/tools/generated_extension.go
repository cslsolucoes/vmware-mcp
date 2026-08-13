package tools

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// registerExtensionTools is the "ExtensionManager" slice of Fase 7's Grupo C
// (".workspace/plans/MCPVMWare2026-08-10-175300-plano-cobertura-completa-api-codegen.plan.md")
// — object.ExtensionManager (all 6 methods: Find, List, Register,
// SetCertificate, Unregister, Update), hand-transcribed from the real
// referencia/govmomi/object/extension_manager.go source, modeVCenterOnly per
// the brief (ExtensionManager is nil on standalone ESXi's ServiceContent —
// see gen/classification.json's "mode":"vcenter-only" for this file and the
// esx/service_content.go vs. vpx/service_content.go comparison the brief
// already did).
//
// Nil-safety (per the brief's explicit ask): object/extension_manager.go
// already ships a guard, GetExtensionManager(c *vim25.Client) (*ExtensionManager,
// error), which returns object.ErrNotSupported instead of dereferencing
// *c.ServiceContent.ExtensionManager when nil — the same pattern
// object/custom_fields_manager.go's GetCustomFieldsManager uses. This file
// reuses it (requireExtensionManager below) rather than re-deriving its own
// nil-check the way generated_vm_provisioning.go's requireProvisioningChecker
// had to (object.NewVmProvisioningChecker has no such guard) — but still
// wraps the resulting error with this project's own message convention
// (naming the affected tool class, explaining the vCenter-only requirement)
// instead of surfacing the generic "product/version specific feature not
// supported by target" text ErrNotSupported carries on its own, matching
// requireProvisioningChecker's/requireCompatibilityChecker's precedent that
// registry.go's CallTool recover() must not be the only safety net AND that
// a caller should get a clear, actionable message either way.
//
// Real vcsim server-side support, confirmed by reading
// referencia/govmomi/simulator/extension_manager.go directly (not assumed
// from the object-package method name): ALL 6 methods have genuine coverage
// — FindExtension, RegisterExtension, UnregisterExtension, UpdateExtension,
// SetExtensionCertificate all have real simulator.ExtensionManager receiver
// methods, and List needs no SOAP method of its own (it is a pure
// property-collector read of mo.ExtensionManager.extensionList, always
// served by vcsim's generic property collector — same reasoning as
// generated_datastore_browser.go's AttachedHosts/Type notes). Every tool
// below gets a genuine functional test against simulator.VPX() — no
// assertReachesServer fallback needed anywhere in this file.
//
// Curation deviation beyond what the brief provided (human review needed):
//
//   - types.Extension.Description is declared as the polymorphic interface
//     types.BaseDescription (`xml:"description,typeattr" json:"description"`)
//     — this project's usual decodeJSONArg (json.Marshal the generic
//     interface{} tree, then json.Unmarshal into the real govmomi struct)
//     CANNOT populate an interface field: confirmed empirically (not
//     assumed) that decodeJSONArg(raw, &types.Extension{}) errors with
//     `json: cannot unmarshal object into Go struct field Extension.description
//     of type types.BaseDescription` whenever the caller's JSON includes a
//     "description" key. types.Description is the ONLY type in
//     referencia/govmomi/vim25/types implementing BaseDescription (grepped
//     if.go, not assumed), so decodeExtensionArg below special-cases exactly
//     that one field: pull "description" out of the raw map before the
//     normal decodeJSONArg pass over the rest of Extension's fields, then
//     decode it separately into a concrete types.Description and assign the
//     pointer — every other Extension field decodes normally since none of
//     the rest are polymorphic. vmware_extension_register/
//     vmware_extension_update both go through this helper; vmware_extension_find/
//     vmware_extension_list only ever produce an Extension (json.Marshal has
//     no trouble serializing an interface field that already holds a
//     concrete value, unlike Unmarshal), so they need no such workaround.
func registerExtensionTools(r *Registry) {
	keyArg := map[string]interface{}{
		"type":        "string",
		"description": `Extension key (Java-package-style unique identifier, e.g. "com.example.myplugin"), as returned by vmware_extension_list or vmware_extension_find.`,
	}
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}
	extensionArg := map[string]interface{}{
		"type":        "object",
		"description": `A types.Extension JSON object matching its Go struct field names. "key" (string, Java-package-style unique identifier, e.g. "com.example.myplugin") and "version" (dot-separated string, e.g. "1.0.0") are effectively required by the real API/vCenter UI even though vcsim itself only rejects a duplicate "key". Common optional fields: "company", "type", "subjectName". "description", if supplied, must be {"label":"...","summary":"..."} — the one concrete shape this tool's decode supports for that field (see this file's top doc comment). See referencia/govmomi/vim25/types/types.go's Extension struct for the full field list (server/client/task/event/fault/privilege/resource lists are accepted as-is if supplied, but are rarely needed for a basic registration).`,
	}

	// --- read-only ---------------------------------------------------------

	r.register("vmware_extension_find",
		`Look up a single registered vCenter extension (a plugin/solution registration, e.g. from the Solutions/Client Plug-Ins inventory) by its key. Reports {"found": false} instead of an error if no extension with that key is registered.`,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"key": keyArg},
			"required":   []interface{}{"key"},
		},
		Tool{Handler: handleExtensionFind},
	)

	r.register("vmware_extension_list",
		"List every vCenter extension (plugin/solution) registered on this connection.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		Tool{Handler: handleExtensionList},
	)

	// --- Tier 2 (disruptive but reversible) --------------------------------

	r.registerDestructive("vmware_extension_register",
		"Register a new vCenter extension (plugin/solution). Fails if an extension with the same key is already registered — use vmware_extension_update to modify an existing one.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"extension": extensionArg,
				"confirm":   confirmArg,
			},
			"required": []interface{}{"extension", "confirm"},
		},
		Tool{Handler: handleExtensionRegister},
	)

	r.registerDestructive("vmware_extension_set_certificate",
		"Set (replace) the client certificate (PEM-encoded) associated with an already-registered extension, used to authenticate that extension via LoginExtensionByCertificate.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"key":             keyArg,
				"certificate_pem": map[string]interface{}{"type": "string", "description": "PEM-encoded X.509 certificate to associate with the extension."},
				"confirm":         confirmArg,
			},
			"required": []interface{}{"key", "certificate_pem", "confirm"},
		},
		Tool{Handler: handleExtensionSetCertificate},
	)

	r.registerDestructive("vmware_extension_update",
		"Update (overwrite) the registration of an already-registered vCenter extension. Fails if no extension with the given key is registered — use vmware_extension_register to create a new one.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"extension": extensionArg,
				"confirm":   confirmArg,
			},
			"required": []interface{}{"extension", "confirm"},
		},
		Tool{Handler: handleExtensionUpdate},
	)

	// --- Tier 1 (irreversible) ----------------------------------------------

	r.registerDestructive("vmware_extension_unregister",
		"Permanently remove a vCenter extension (plugin/solution) registration. Irreversible (the registration itself, and any client-side certificate association, is gone — re-registering starts fresh).",
		tier1,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"key": keyArg, "confirm": confirmArg},
			"required":   []interface{}{"key", "confirm"},
		},
		Tool{Handler: handleExtensionUnregister},
	)
}

// requireExtensionManager wraps object.GetExtensionManager (see this file's
// top doc comment) with this project's own clear-error-message convention —
// registry.go's CallTool recover() must not be the only safety net, and a
// caller should get an explanation, not ErrNotSupported's bare generic text.
func requireExtensionManager(client *vmware.Client) (*object.ExtensionManager, error) {
	em, err := object.GetExtensionManager(client.Client.Client)
	if err != nil {
		return nil, fmt.Errorf("extension-manager tools are not available on this connection (ExtensionManager requires vCenter — it is not present on a standalone ESXi host's ServiceContent): %w", err)
	}
	return em, nil
}

// decodeExtensionArg decodes the "extension" argument into a types.Extension
// — see this file's top doc comment for why the polymorphic "description"
// field needs special handling that the rest of the struct doesn't.
func decodeExtensionArg(raw interface{}) (types.Extension, error) {
	var ext types.Extension
	m, ok := raw.(map[string]interface{})
	if !ok {
		return ext, fmt.Errorf("extension must be a JSON object")
	}

	descRaw, hasDesc := m["description"]
	if hasDesc {
		rest := make(map[string]interface{}, len(m))
		for k, v := range m {
			if k != "description" {
				rest[k] = v
			}
		}
		m = rest
	}
	if err := decodeJSONArg(m, &ext); err != nil {
		return ext, err
	}
	if hasDesc {
		var desc types.Description
		if err := decodeJSONArg(descRaw, &desc); err != nil {
			return ext, fmt.Errorf("invalid description: %w", err)
		}
		ext.Description = &desc
	}
	return ext, nil
}

func handleExtensionFind(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	em, err := requireExtensionManager(client)
	if err != nil {
		return "", err
	}
	key, _ := args["key"].(string)
	if key == "" {
		return "", fmt.Errorf("key is required")
	}

	ext, err := em.Find(ctx, key)
	if err != nil {
		return "", fmt.Errorf("failed to find extension %q: %w", key, err)
	}
	if ext == nil {
		return marshalJSON(map[string]interface{}{"key": key, "found": false})
	}

	return marshalJSON(map[string]interface{}{"key": key, "found": true, "extension": ext})
}

func handleExtensionList(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	em, err := requireExtensionManager(client)
	if err != nil {
		return "", err
	}

	list, err := em.List(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to list extensions: %w", err)
	}

	return marshalJSON(map[string]interface{}{"count": len(list), "extensions": list})
}

func handleExtensionRegister(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	em, err := requireExtensionManager(client)
	if err != nil {
		return "", err
	}
	if args["extension"] == nil {
		return "", fmt.Errorf("extension is required")
	}
	ext, err := decodeExtensionArg(args["extension"])
	if err != nil {
		return "", fmt.Errorf("invalid extension: %w", err)
	}

	if err := em.Register(ctx, ext); err != nil {
		return "", fmt.Errorf("failed to register extension %q: %w", ext.Key, err)
	}

	return marshalJSON(map[string]interface{}{"key": ext.Key, "result": "registered"})
}

func handleExtensionSetCertificate(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	em, err := requireExtensionManager(client)
	if err != nil {
		return "", err
	}
	key, _ := args["key"].(string)
	if key == "" {
		return "", fmt.Errorf("key is required")
	}
	certPem, _ := args["certificate_pem"].(string)
	if certPem == "" {
		return "", fmt.Errorf("certificate_pem is required")
	}

	if err := em.SetCertificate(ctx, key, certPem); err != nil {
		return "", fmt.Errorf("failed to set certificate for extension %q: %w", key, err)
	}

	return marshalJSON(map[string]interface{}{"key": key, "result": "certificate_set"})
}

func handleExtensionUnregister(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	em, err := requireExtensionManager(client)
	if err != nil {
		return "", err
	}
	key, _ := args["key"].(string)
	if key == "" {
		return "", fmt.Errorf("key is required")
	}

	if err := em.Unregister(ctx, key); err != nil {
		return "", fmt.Errorf("failed to unregister extension %q: %w", key, err)
	}

	return marshalJSON(map[string]interface{}{"key": key, "result": "unregistered"})
}

func handleExtensionUpdate(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	em, err := requireExtensionManager(client)
	if err != nil {
		return "", err
	}
	if args["extension"] == nil {
		return "", fmt.Errorf("extension is required")
	}
	ext, err := decodeExtensionArg(args["extension"])
	if err != nil {
		return "", fmt.Errorf("invalid extension: %w", err)
	}

	if err := em.Update(ctx, ext); err != nil {
		return "", fmt.Errorf("failed to update extension %q: %w", ext.Key, err)
	}

	return marshalJSON(map[string]interface{}{"key": ext.Key, "result": "updated"})
}

package tools

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// registerCustomizationSpecTools is the "CustomizationSpecManager" slice of
// Fase 7's Grupo C
// (".workspace/plans/MCPVMWare2026-08-10-175300-plano-cobertura-completa-api-codegen.plan.md")
// — object.CustomizationSpecManager (all 10 methods), hand-transcribed from
// the real referencia/govmomi/object/customization_spec_manager.go source,
// modeVCenterOnly (gen/classification.json already has "mode":"vcenter-only"
// for this file — CustomizationSpecManager is nil on standalone ESXi's
// ServiceContent, same esx/service_content.go-vs-vpx/service_content.go
// pattern the brief already confirmed for extension_manager.go).
//
// Nil-safety: unlike object.NewExtensionManager (which has a ready-made
// GetExtensionManager guard — see generated_extension.go), object.
// NewCustomizationSpecManager dereferences *c.ServiceContent.
// CustomizationSpecManager directly with no guard of its own (confirmed by
// reading object/customization_spec_manager.go). requireCustomizationSpecManager
// below adds the same explicit nil-check generated_vm_provisioning.go's
// requireProvisioningChecker/requireCompatibilityChecker already established
// for this exact class of bug (registry.go's CallTool recover() must not be
// the only safety net against a standalone-ESXi caller hitting a nil-pointer
// panic) — not explicitly spelled out in the brief for this file, but the
// same real risk the brief DID call out for extension_manager.go, so applied
// here too as a curation decision beyond what was given.
//
// Tier corrections beyond the 3 the brief pre-authorized (Info,
// CustomizationSpecItemToXml, XmlToCustomizationSpecItem — all corrected
// tier2->no-tier per the brief, since none of them mutate server state):
// gen/classification.json also has DoesCustomizationSpecExist classified
// tier2 ("fail-safe default (no pattern matched)" — the classifier's naming
// heuristic doesn't recognize a "Does...Exist" prefix as read-only the way
// it recognizes "Get"/"List"/"Query"), even though the brief's own text
// claims "DoesCustomizationSpecExist/GetCustomizationSpec já sem-tier,
// mantenha." That claim does not match the actual classification.json
// content (double-checked by grepping it directly, not assumed) — ONLY
// GetCustomizationSpec is already "" (no tier); DoesCustomizationSpecExist
// is tier2. Reading the real source (object/customization_spec_manager.go)
// confirms DoesCustomizationSpecExist is a pure boolean existence check with
// zero server-side mutation (calls the DoesCustomizationSpecExist SOAP
// method, returns a bool) — same class of pure-read as the 3 the brief did
// flag, so it is corrected to no-tier here too, for the same reason.
//
// Curation deviation the brief flagged in advance, confirmed and applied
// as-is: item types.CustomizationSpecItem (used by CreateCustomizationSpec/
// OverwriteCustomizationSpec/CustomizationSpecItemToXml) decodes via the
// project's normal decodeJSONArg, no special-casing needed at the
// CustomizationSpecItem level itself. BUT (found while implementing, not in
// the brief): CustomizationSpecItem.Spec (types.CustomizationSpec) has 2
// polymorphic interface fields of its own — Options
// (types.BaseCustomizationOptions) and Identity (types.
// BaseCustomizationIdentitySettings) — confirmed empirically (not assumed)
// that plain json.Unmarshal into types.CustomizationSpec errors with `json:
// cannot unmarshal object into Go struct field CustomizationSpec.identity of
// type types.BaseCustomizationIdentitySettings` whenever a caller's JSON
// supplies "spec.identity" (same failure class as generated_extension.go's
// Description field, and the SAME pre-existing limitation already present
// and documented in generated_vm_provisioning.go's handleVMCustomize for
// this exact types.CustomizationSpec struct — see that file's top doc
// comment, point 4). Given (a) the brief's explicit ask to decode
// CustomizationSpecItem plainly via json.Unmarshal, (b) this project's
// established precedent of documenting rather than solving this class of
// deep, multi-variant (6 Identity variants x 2 Options variants x further
// polymorphic NicSettingMap[].Adapter.Ip) polymorphism as an MVP limitation,
// and (c) no existing helper anywhere in this project actually resolves it
// (inventing one is out of scope here — it would need to be shared with
// vm_provisioning.go, not duplicated), the SAME documented-limitation stance
// is kept: the "item"/"spec" schema descriptions below spell out plainly
// that "spec.identity"/"spec.options" must be omitted or the call fails with
// a clear decode error (not a panic, not a silently-wrong request) — proven
// empirically against real vcsim (not asserted): CreateCustomizationSpec/
// OverwriteCustomizationSpec/GetCustomizationSpec/Info all round-trip a
// bare-Options/bare-Identity CustomizationSpecItem successfully end to end,
// which is what generated_customization_spec_test.go exercises as the real
// functional success path for the 5 vcsim-backed tools.
//
// Real vcsim server-side support, confirmed empirically by running each
// method against a live simulator.VPX() (not assumed from the object-package
// method name, and not trusted from a source-level grep alone this time —
// actually executed): CreateCustomizationSpec, DoesCustomizationSpecExist,
// GetCustomizationSpec, Info, and OverwriteCustomizationSpec all genuinely
// work end to end. CustomizationSpecItemToXml, DeleteCustomizationSpec,
// DuplicateCustomizationSpec, RenameCustomizationSpec, and
// XmlToCustomizationSpecItem all fault cleanly with "CustomizationSpecManager
// does not implement: <MethodName>" (a real MethodNotFound-shaped fault from
// vcsim's generic dispatch reaching the server, not a client-side crash) —
// matching this file's source-level grep across referencia/govmomi/simulator
// finding zero receiver methods for any of those 5 names. All 5 are still
// registered exactly as real vSphere supports them (a vcsim gap is not a
// reason to omit a tool real deployments can use) and get an
// assertReachesServer-style test instead of a full success path — same
// posture as generated_resourcepool_vapp.go's 4 VirtualApp methods vcsim
// doesn't implement.
func registerCustomizationSpecTools(r *Registry) {
	nameArg := map[string]interface{}{
		"type":        "string",
		"description": `Unique name of a customization spec, e.g. "vcsim-linux" (vcsim's own built-in default) or any name returned by vmware_customization_spec_info.`,
	}
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}
	itemArg := map[string]interface{}{
		"type":        "object",
		"description": `A types.CustomizationSpecItem JSON object matching its Go struct field names: "info" (types.CustomizationSpecInfo: "name" required unique string, plus "description"/"type" strings) and "spec" (types.CustomizationSpec). IMPORTANT LIMITATION (see this file's top doc comment): "spec.identity" and "spec.options" are polymorphic fields (6 possible Identity variants, 2 possible Options variants) that this tool's generic JSON decode cannot populate — omit both (leave "spec" with only e.g. "globalIPSettings":{} and "nicSettingMap":[]) or the call fails with a clear decode error naming the offending field. "spec.globalIPSettings" and "spec.nicSettingMap" are ordinary concrete structs and decode normally. See referencia/govmomi/vim25/types/types.go's CustomizationSpecItem/CustomizationSpec for the full field list.`,
	}

	// --- read-only -----------------------------------------------------

	r.register("vmware_customization_spec_does_exist",
		"Check whether a customization spec with the given name exists. Corrected to no-tier vs. the raw generator output — see this file's top doc comment.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"name": nameArg},
			"required":   []interface{}{"name"},
		},
		Tool{Handler: handleCustomizationSpecDoesExist},
	)

	r.register("vmware_customization_spec_get",
		"Get a customization spec (guest OS identity/network settings template) by name.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"name": nameArg},
			"required":   []interface{}{"name"},
		},
		Tool{Handler: handleCustomizationSpecGet},
	)

	r.register("vmware_customization_spec_info",
		"List summary info (name, description, guest OS type, change version, last update time) for every customization spec available on this connection. Corrected to no-tier vs. the raw generator output — see this file's top doc comment.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		Tool{Handler: handleCustomizationSpecInfo},
	)

	r.register("vmware_customization_spec_item_to_xml",
		"Convert a customization spec item to its XML representation. Pure format conversion with no server-side mutation — corrected to no-tier vs. the raw generator output (see this file's top doc comment). NOTE: not simulated by this project's test harness (vcsim has no CustomizationSpecItemToXml handler) — exercise carefully against a real vCenter first.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"item": itemArg},
			"required":   []interface{}{"item"},
		},
		Tool{Handler: handleCustomizationSpecItemToXml},
	)

	r.register("vmware_customization_spec_xml_to_item",
		"Convert an XML customization spec representation back to a types.CustomizationSpecItem. Pure format conversion with no server-side mutation — corrected to no-tier vs. the raw generator output (see this file's top doc comment). NOTE: not simulated by this project's test harness (vcsim has no XmlToCustomizationSpecItem handler) — exercise carefully against a real vCenter first.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"xml": map[string]interface{}{"type": "string", "description": "XML customization spec representation, e.g. as returned by vmware_customization_spec_item_to_xml."},
			},
			"required": []interface{}{"xml"},
		},
		Tool{Handler: handleCustomizationSpecXmlToItem},
	)

	// --- Tier 2 (disruptive but reversible) -----------------------------

	r.registerDestructive("vmware_customization_spec_create",
		"Create a new customization spec. Fails if a spec with the same name already exists — use vmware_customization_spec_overwrite to replace an existing one.",
		tier2,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"item": itemArg, "confirm": confirmArg},
			"required":   []interface{}{"item", "confirm"},
		},
		Tool{Handler: handleCustomizationSpecCreate},
	)

	r.registerDestructive("vmware_customization_spec_duplicate",
		"Duplicate an existing customization spec under a new name. NOTE: not simulated by this project's test harness (vcsim has no DuplicateCustomizationSpec handler) — exercise carefully against a real vCenter first.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name":     nameArg,
				"new_name": map[string]interface{}{"type": "string", "description": "Name for the duplicated spec."},
				"confirm":  confirmArg,
			},
			"required": []interface{}{"name", "new_name", "confirm"},
		},
		Tool{Handler: handleCustomizationSpecDuplicate},
	)

	r.registerDestructive("vmware_customization_spec_overwrite",
		"Overwrite an existing customization spec (matched by item.info.name) with new content. Fails if no spec with that name exists — use vmware_customization_spec_create for a new one.",
		tier2,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"item": itemArg, "confirm": confirmArg},
			"required":   []interface{}{"item", "confirm"},
		},
		Tool{Handler: handleCustomizationSpecOverwrite},
	)

	r.registerDestructive("vmware_customization_spec_rename",
		"Rename an existing customization spec. NOTE: not simulated by this project's test harness (vcsim has no RenameCustomizationSpec handler) — exercise carefully against a real vCenter first.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name":     nameArg,
				"new_name": map[string]interface{}{"type": "string", "description": "New name for the spec."},
				"confirm":  confirmArg,
			},
			"required": []interface{}{"name", "new_name", "confirm"},
		},
		Tool{Handler: handleCustomizationSpecRename},
	)

	// --- Tier 1 (irreversible) -------------------------------------------

	r.registerDestructive("vmware_customization_spec_delete",
		"Permanently delete a customization spec. Irreversible. NOTE: not simulated by this project's test harness (vcsim has no DeleteCustomizationSpec handler) — exercise carefully against a real vCenter first.",
		tier1,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"name": nameArg, "confirm": confirmArg},
			"required":   []interface{}{"name", "confirm"},
		},
		Tool{Handler: handleCustomizationSpecDelete},
	)
}

// requireCustomizationSpecManager is the explicit nil-guard for
// object.NewCustomizationSpecManager — see this file's top doc comment for
// why this file needs its own (unlike generated_extension.go, which reuses
// object.GetExtensionManager's ready-made guard).
func requireCustomizationSpecManager(client *vmware.Client) (*object.CustomizationSpecManager, error) {
	vc := client.Client.Client
	if vc.ServiceContent.CustomizationSpecManager == nil {
		return nil, fmt.Errorf("customization-spec tools are not available on this connection (CustomizationSpecManager requires vCenter — it is not present on a standalone ESXi host's ServiceContent)")
	}
	return object.NewCustomizationSpecManager(vc), nil
}

// decodeCustomizationSpecItemArg decodes the "item" argument into a
// types.CustomizationSpecItem via the project's normal decodeJSONArg — see
// this file's top doc comment for the documented (not solved)
// spec.identity/spec.options polymorphism limitation this carries.
func decodeCustomizationSpecItemArg(raw interface{}) (types.CustomizationSpecItem, error) {
	var item types.CustomizationSpecItem
	if raw == nil {
		return item, fmt.Errorf("item is required")
	}
	if err := decodeJSONArg(raw, &item); err != nil {
		return item, fmt.Errorf("invalid item (see this tool's schema description for the spec.identity/spec.options limitation): %w", err)
	}
	return item, nil
}

func handleCustomizationSpecDoesExist(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	cs, err := requireCustomizationSpecManager(client)
	if err != nil {
		return "", err
	}
	name, _ := args["name"].(string)
	if name == "" {
		return "", fmt.Errorf("name is required")
	}

	exists, err := cs.DoesCustomizationSpecExist(ctx, name)
	if err != nil {
		return "", fmt.Errorf("failed to check existence of customization spec %q: %w", name, err)
	}

	return marshalJSON(map[string]interface{}{"name": name, "exists": exists})
}

func handleCustomizationSpecGet(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	cs, err := requireCustomizationSpecManager(client)
	if err != nil {
		return "", err
	}
	name, _ := args["name"].(string)
	if name == "" {
		return "", fmt.Errorf("name is required")
	}

	item, err := cs.GetCustomizationSpec(ctx, name)
	if err != nil {
		return "", fmt.Errorf("failed to get customization spec %q: %w", name, err)
	}

	return marshalJSON(map[string]interface{}{"name": name, "item": item})
}

func handleCustomizationSpecInfo(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	cs, err := requireCustomizationSpecManager(client)
	if err != nil {
		return "", err
	}

	info, err := cs.Info(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to list customization spec info: %w", err)
	}

	return marshalJSON(map[string]interface{}{"count": len(info), "specs": info})
}

func handleCustomizationSpecItemToXml(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	cs, err := requireCustomizationSpecManager(client)
	if err != nil {
		return "", err
	}
	item, err := decodeCustomizationSpecItemArg(args["item"])
	if err != nil {
		return "", err
	}

	xml, err := cs.CustomizationSpecItemToXml(ctx, item)
	if err != nil {
		return "", fmt.Errorf("failed to convert customization spec item to XML: %w", err)
	}

	return marshalJSON(map[string]interface{}{"xml": xml})
}

func handleCustomizationSpecXmlToItem(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	cs, err := requireCustomizationSpecManager(client)
	if err != nil {
		return "", err
	}
	xmlArg, _ := args["xml"].(string)
	if xmlArg == "" {
		return "", fmt.Errorf("xml is required")
	}

	item, err := cs.XmlToCustomizationSpecItem(ctx, xmlArg)
	if err != nil {
		return "", fmt.Errorf("failed to convert XML to customization spec item: %w", err)
	}

	return marshalJSON(map[string]interface{}{"item": item})
}

func handleCustomizationSpecCreate(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	cs, err := requireCustomizationSpecManager(client)
	if err != nil {
		return "", err
	}
	item, err := decodeCustomizationSpecItemArg(args["item"])
	if err != nil {
		return "", err
	}

	if err := cs.CreateCustomizationSpec(ctx, item); err != nil {
		return "", fmt.Errorf("failed to create customization spec %q: %w", item.Info.Name, err)
	}

	return marshalJSON(map[string]interface{}{"name": item.Info.Name, "result": "created"})
}

func handleCustomizationSpecDuplicate(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	cs, err := requireCustomizationSpecManager(client)
	if err != nil {
		return "", err
	}
	name, _ := args["name"].(string)
	if name == "" {
		return "", fmt.Errorf("name is required")
	}
	newName, _ := args["new_name"].(string)
	if newName == "" {
		return "", fmt.Errorf("new_name is required")
	}

	if err := cs.DuplicateCustomizationSpec(ctx, name, newName); err != nil {
		return "", fmt.Errorf("failed to duplicate customization spec %q to %q: %w", name, newName, err)
	}

	return marshalJSON(map[string]interface{}{"name": name, "new_name": newName, "result": "duplicated"})
}

func handleCustomizationSpecOverwrite(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	cs, err := requireCustomizationSpecManager(client)
	if err != nil {
		return "", err
	}
	item, err := decodeCustomizationSpecItemArg(args["item"])
	if err != nil {
		return "", err
	}

	if err := cs.OverwriteCustomizationSpec(ctx, item); err != nil {
		return "", fmt.Errorf("failed to overwrite customization spec %q: %w", item.Info.Name, err)
	}

	return marshalJSON(map[string]interface{}{"name": item.Info.Name, "result": "overwritten"})
}

func handleCustomizationSpecRename(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	cs, err := requireCustomizationSpecManager(client)
	if err != nil {
		return "", err
	}
	name, _ := args["name"].(string)
	if name == "" {
		return "", fmt.Errorf("name is required")
	}
	newName, _ := args["new_name"].(string)
	if newName == "" {
		return "", fmt.Errorf("new_name is required")
	}

	if err := cs.RenameCustomizationSpec(ctx, name, newName); err != nil {
		return "", fmt.Errorf("failed to rename customization spec %q to %q: %w", name, newName, err)
	}

	return marshalJSON(map[string]interface{}{"name": name, "new_name": newName, "result": "renamed"})
}

func handleCustomizationSpecDelete(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	cs, err := requireCustomizationSpecManager(client)
	if err != nil {
		return "", err
	}
	name, _ := args["name"].(string)
	if name == "" {
		return "", fmt.Errorf("name is required")
	}

	if err := cs.DeleteCustomizationSpec(ctx, name); err != nil {
		return "", fmt.Errorf("failed to delete customization spec %q: %w", name, err)
	}

	return marshalJSON(map[string]interface{}{"name": name, "result": "deleted"})
}

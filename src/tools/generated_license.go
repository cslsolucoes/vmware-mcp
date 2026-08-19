package tools

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// registerLicenseTools adds vSphere licensing tools for the 2 managed
// objects the API exposes for it — LicenseManager and
// LicenseAssignmentManager — neither of which had any tool before this file,
// confirmed by grepping every existing tools/*.go for "License" prior to
// writing this (only unrelated hits, e.g. generated_extension.go's
// unrelated "licensed" wording did not match). Both are, like IscsiManager
// (see generated_host_iscsi_portbinding.go's top doc comment, the model this
// file follows), managed objects with NO object.* wrapper in
// referencia/govmomi/object/*.go (confirmed by grep — LicenseManager only
// appears in vim25/mo, vim25/types, vim25/methods and the simulator, never
// under object/), so every handler below dials the raw vim25 SOAP method
// directly: methods.Xxx(ctx, client.Client.Client, &types.Xxx{This: ref, ...}).
//
// MoRef resolution:
//
//   - LicenseManager's MoRef comes straight off
//     client.Client.ServiceContent.LicenseManager (*types.ManagedObjectReference,
//     confirmed in referencia/govmomi/vim25/types/types.go's ServiceContent
//     struct) — no property-collector round trip needed. Present on BOTH a
//     standalone ESXi host and vCenter (confirmed in
//     referencia/govmomi/simulator/esx/service_content.go and
//     vpx/service_content.go — both set it), so this file's class is
//     modeVSphereGeneral, not modeVCenterOnly.
//
//   - LicenseAssignmentManager's MoRef is NOT a ServiceContent field — it is
//     a property of LicenseManager itself (mo.LicenseManager.
//     LicenseAssignmentManager *types.ManagedObjectReference), read via one
//     object.NewCommon(...).Properties call (the same Properties-read
//     pattern generated_host_iscsi_portbinding.go's hostIscsiManager and
//     host.go's handleHostInfo use for a manager reachable only through a
//     property, not a direct accessor). Confirmed empirically by reading
//     referencia/govmomi/simulator/license_manager.go's LicenseManager.init():
//     it only ever sets LicenseAssignmentManager (and only ever registers a
//     simulator.LicenseAssignmentManager handler for it) `if r.IsVPX()` — a
//     standalone ESXi host's LicenseManager never populates this property.
//     licenseAssignmentManagerRef below reports that as a clear error
//     ("requires vCenter"), the same nil-guard convention
//     generated_extension.go's requireExtensionManager established, rather
//     than letting a nil MoRef reach the SOAP call or registry.go's CallTool
//     recover() catch a panic.
//
// Tier, per the brief: read-only (Decode/Query/Check) → r.register;
// mutations (Add/Remove/Update/Configure/Set) → registerDestructive tier2
// (disruptive — a wrong license/assignment change can affect what features
// an entity has access to — but reversible: every mutation here has a
// counterpart or can simply be called again with different arguments, unlike
// e.g. vmware_vm_destroy's tier1 irreversibility).
//
// vcsim coverage, confirmed by reading
// referencia/govmomi/simulator/license_manager.go directly (not assumed from
// the method name): LicenseManager's AddLicense, RemoveLicense,
// DecodeLicense, UpdateLicenseLabel and LicenseAssignmentManager's
// QueryAssignedLicenses, UpdateAssignedLicense all have real receiver
// methods — a genuine functional test reaches a real success path for those
// 6. UpdateLicense, QueryLicenseSourceAvailability, QuerySupportedFeatures,
// CheckLicenseFeature, ConfigureLicenseSource, SetLicenseEdition, and
// RemoveAssignedLicense have NO matching receiver method anywhere in that
// file — a call against vcsim reaches the simulator's generic method-dispatch
// fallback and comes back with a clean MethodNotFound-style fault (same
// reasoning/pattern as generated_host_iscsi_portbinding.go's 5 Query* methods
// and generated_vm_lifecycle.go's 6 unsimulated methods);
// generated_license_test.go drives those 7 with assertReachesServer instead
// of a full success-path assertion.
//
// Naming/method-list deviation from the brief (human review; grounded in the
// real referencia/govmomi/vim25/types/types.go & methods.go source, not
// invented): the brief's suggested tool names omitted UpdateLicenseLabel,
// ConfigureLicenseSource, and SetLicenseEdition even though its own method
// list included all 3 — this file adds vmware_license_update_label,
// vmware_license_configure_source, and vmware_license_set_edition to cover
// them, keeping the same vmware_license_<verb> naming convention as every
// other tool here. RemoveLicenseLabel (a real, separate WSDL method
// alongside UpdateLicenseLabel) is deliberately NOT added — it was in
// neither the brief's method list nor its tool-name list, so adding it would
// be scope creep beyond what was asked.
func registerLicenseTools(r *Registry) {
	licenseKeyArg := map[string]interface{}{
		"type":        "string",
		"description": `A license key, e.g. a serial number ("XXXXX-XXXXX-XXXXX-XXXXX-XXXXX"), as returned by vmware_license_decode/vmware_license_add or listed under "licenses" in this connection's LicenseManager.`,
	}
	labelsArg := map[string]interface{}{
		"type": "array",
		"items": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"key":   map[string]interface{}{"type": "string"},
				"value": map[string]interface{}{"type": "string"},
			},
			"required": []interface{}{"key", "value"},
		},
		"description": `Optional key-value labels to attach to the license (types.KeyValue[]). Per the API's own doc comment, ignored by ESX Server — meaningful against vCenter.`,
	}
	hostArg := map[string]interface{}{
		"type":        "string",
		"description": `Optional host identifier (name/pattern, e.g. "esxi-01.local", or a full inventory path) as returned by vmware_list_hosts — "host to act on if LicenseManager is not on a host" per the real API's own doc comment. Meaningful against a vCenter's LicenseManager (which is not bound to any single host); omit when connected directly to a standalone ESXi host, whose LicenseManager already applies to that host.`,
	}
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}
	entityIDArg := map[string]interface{}{
		"type":        "string",
		"description": `ID of the entity a license is assigned to (e.g. a HostSystem). Read this from a prior vmware_license_query_assigned call's own "entity_id" output rather than constructing it — the exact ID scheme (host hardware UUID vs. something else) is server-dependent.`,
	}

	// --- LicenseManager: read-only ----------------------------------------

	r.register("vmware_license_decode",
		"Decode a license key (e.g. validate it and report its edition/features) WITHOUT adding it to this connection's list of available licenses. Use vmware_license_add to actually register it.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"license_key": licenseKeyArg},
			"required":   []interface{}{"license_key"},
		},
		Tool{Handler: handleLicenseDecode},
	)

	r.register("vmware_license_query_source_availability",
		"List the license types available from this connection's (or, if host is given, a specific host's) current license source, with total/available counts per feature.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"host": hostArg},
		},
		Tool{Handler: handleLicenseQuerySourceAvailability},
	)

	r.register("vmware_license_query_supported_features",
		"List the license features supported by this connection's (or, if host is given, a specific host's) current license edition/source.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"host": hostArg},
		},
		Tool{Handler: handleLicenseQuerySupportedFeatures},
	)

	r.register("vmware_license_check_feature",
		"Check whether a specific license feature is currently enabled/available on this connection (or, if host is given, a specific host).",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":        hostArg,
				"feature_key": map[string]interface{}{"type": "string", "description": `Name of the feature to check, e.g. "dvs" or "vmotion" — see vmware_license_query_supported_features for the list of keys this connection recognizes.`},
			},
			"required": []interface{}{"feature_key"},
		},
		Tool{Handler: handleLicenseCheckFeature},
	)

	// --- LicenseManager: destructive (tier2) -------------------------------

	r.registerDestructive("vmware_license_add",
		"Add a license key to this connection's list of available licenses. If the key is already present, returns its existing info without duplicating it.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"license_key": licenseKeyArg,
				"labels":      labelsArg,
				"confirm":     confirmArg,
			},
			"required": []interface{}{"license_key", "confirm"},
		},
		Tool{Handler: handleLicenseAdd},
	)

	r.registerDestructive("vmware_license_remove",
		"Remove a license key from this connection's list of available licenses. Reversible via vmware_license_add. A no-op (not an error) if the key isn't present.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"license_key": licenseKeyArg,
				"confirm":     confirmArg,
			},
			"required": []interface{}{"license_key", "confirm"},
		},
		Tool{Handler: handleLicenseRemove},
	)

	r.registerDestructive("vmware_license_update",
		"Update an already-added license key's labels (replaces any existing labels for that key). Fails if the key was never added via vmware_license_add.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"license_key": licenseKeyArg,
				"labels":      labelsArg,
				"confirm":     confirmArg,
			},
			"required": []interface{}{"license_key", "confirm"},
		},
		Tool{Handler: handleLicenseUpdate},
	)

	r.registerDestructive("vmware_license_update_label",
		`Set (or, if label_value is an empty string, remove) a single key-value label on an already-added license key. Ignored by ESX Server per the API's own doc comment — meaningful against vCenter.`,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"license_key": licenseKeyArg,
				"label_key":   map[string]interface{}{"type": "string", "description": "The label's key."},
				"label_value": map[string]interface{}{"type": "string", "description": "The label's new value. An empty string removes the label if it already exists (or creates an empty-value label if it doesn't)."},
				"confirm":     confirmArg,
			},
			"required": []interface{}{"license_key", "label_key", "label_value", "confirm"},
		},
		Tool{Handler: handleLicenseUpdateLabel},
	)

	r.registerDestructive("vmware_license_configure_source",
		"Reconfigure this connection's (or, if host is given, a specific host's) license source — switch between a local license key, a license server, or evaluation mode. Reversible by calling again with a different source.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host": hostArg,
				"license_source": map[string]interface{}{
					"type":        "object",
					"description": `The license source to configure, one of 3 shapes selected by "type": {"type":"local","license_keys":"..."} (types.LocalLicenseSource — a locally-installed license key string), {"type":"served","license_server":"host:port"} (types.LicenseServerSource — a license server to connect to), or {"type":"evaluation"} (types.EvaluationLicenseSource — revert to evaluation mode).`,
					"properties": map[string]interface{}{
						"type":           map[string]interface{}{"type": "string", "enum": []interface{}{"local", "served", "evaluation"}},
						"license_keys":   map[string]interface{}{"type": "string", "description": `Required when type is "local" — the local license key string (types.LocalLicenseSource.LicenseKeys).`},
						"license_server": map[string]interface{}{"type": "string", "description": `Required when type is "served" — "host:port" of the license server (types.LicenseServerSource.LicenseServer).`},
					},
					"required": []interface{}{"type"},
				},
				"confirm": confirmArg,
			},
			"required": []interface{}{"license_source", "confirm"},
		},
		Tool{Handler: handleLicenseConfigureSource},
	)

	r.registerDestructive("vmware_license_set_edition",
		`Select this connection's (or, if host is given, a specific host's) license edition by feature key. Passing an empty feature_key makes the product unlicensed (per the API's own doc comment) — reversible by calling again with a valid feature_key.`,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":        hostArg,
				"feature_key": map[string]interface{}{"type": "string", "description": `Name of the edition feature to select, e.g. "vc.standard". Empty/omitted makes the product unlicensed.`},
				"confirm":     confirmArg,
			},
			"required": []interface{}{"confirm"},
		},
		Tool{Handler: handleLicenseSetEdition},
	)

	// --- LicenseAssignmentManager: read-only -------------------------------

	r.register("vmware_license_query_assigned",
		"List license assignments (vCenter only — see this file's top doc comment). Pass entity_id to filter to one entity's assignment, or omit it to list every current assignment.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"entity_id": map[string]interface{}{"type": "string", "description": `Optional entity ID to filter to (e.g. a HostSystem). Omit to list every current license assignment.`},
			},
		},
		Tool{Handler: handleLicenseQueryAssigned},
	)

	// --- LicenseAssignmentManager: destructive (tier2) ---------------------

	r.registerDestructive("vmware_license_update_assigned",
		"Assign a license key to an entity (vCenter only — see this file's top doc comment), e.g. a host. Reversible via vmware_license_update_assigned (re-assign a different key) or vmware_license_remove_assigned.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"entity_id":           entityIDArg,
				"license_key":         licenseKeyArg,
				"entity_display_name": map[string]interface{}{"type": "string", "description": "Optional display name for the entity, shown in the assignment listing."},
				"confirm":             confirmArg,
			},
			"required": []interface{}{"entity_id", "license_key", "confirm"},
		},
		Tool{Handler: handleLicenseUpdateAssigned},
	)

	r.registerDestructive("vmware_license_remove_assigned",
		"Remove an entity's license assignment (vCenter only — see this file's top doc comment). Reversible via vmware_license_update_assigned.",
		tier2,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"entity_id": entityIDArg, "confirm": confirmArg},
			"required":   []interface{}{"entity_id", "confirm"},
		},
		Tool{Handler: handleLicenseRemoveAssigned},
	)
}

// requireLicenseManager resolves the LicenseManager MoRef straight off
// ServiceContent — present on both a standalone ESXi host and vCenter (see
// this file's top doc comment), unlike LicenseAssignmentManager below. The
// nil check is defense in depth (matches generated_extension.go's
// requireExtensionManager / generated_vm_provisioning.go's
// requireProvisioningChecker precedent) — registry.go's CallTool recover()
// must not be the only safety net.
func requireLicenseManager(client *vmware.Client) (types.ManagedObjectReference, error) {
	ref := client.Client.ServiceContent.LicenseManager
	if ref == nil {
		return types.ManagedObjectReference{}, fmt.Errorf("license-manager tools are not available on this connection (ServiceContent.LicenseManager is nil)")
	}
	return *ref, nil
}

// licenseAssignmentManagerRef resolves the LicenseAssignmentManager MoRef —
// a property of LicenseManager, not a direct ServiceContent field — via one
// object.NewCommon(...).Properties read. See this file's top doc comment for
// why a standalone ESXi host never populates this property (vCenter-only in
// practice).
func licenseAssignmentManagerRef(ctx context.Context, client *vmware.Client) (types.ManagedObjectReference, error) {
	lmRef, err := requireLicenseManager(client)
	if err != nil {
		return types.ManagedObjectReference{}, err
	}

	common := object.NewCommon(client.Client.Client, lmRef)
	var lm mo.LicenseManager
	if err := common.Properties(ctx, lmRef, []string{"licenseAssignmentManager"}, &lm); err != nil {
		return types.ManagedObjectReference{}, fmt.Errorf("failed to read license assignment manager: %w", err)
	}
	if lm.LicenseAssignmentManager == nil {
		return types.ManagedObjectReference{}, fmt.Errorf("license-assignment tools require vCenter (LicenseAssignmentManager is not available on a standalone ESXi host's LicenseManager)")
	}
	return *lm.LicenseAssignmentManager, nil
}

// licenseOptionalHostRef resolves the optional "host" argument (name/pattern,
// same resolution as resolveHost elsewhere in this package) to a
// *types.ManagedObjectReference, or nil if the argument was omitted — see
// this file's hostArg schema doc comment for which methods accept it and why.
func licenseOptionalHostRef(ctx context.Context, client *vmware.Client, args map[string]interface{}) (*types.ManagedObjectReference, error) {
	name, _ := args["host"].(string)
	if name == "" {
		return nil, nil
	}
	host, err := client.Finder.HostSystem(ctx, dcScopedPath("host", name))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve host %q: %w", name, err)
	}
	ref := host.Reference()
	return &ref, nil
}

// licenseKeyFromArgs reads and validates the required license_key argument.
func licenseKeyFromArgs(args map[string]interface{}) (string, error) {
	key, _ := args["license_key"].(string)
	if key == "" {
		return "", fmt.Errorf("license_key is required")
	}
	return key, nil
}

// decodeLicenseLabels decodes the optional "labels" argument into
// []types.KeyValue via decodeJSONArg — nil (no labels) if the argument was
// omitted.
func decodeLicenseLabels(raw interface{}) ([]types.KeyValue, error) {
	if raw == nil {
		return nil, nil
	}
	var labels []types.KeyValue
	if err := decodeJSONArg(raw, &labels); err != nil {
		return nil, fmt.Errorf("invalid labels: %w", err)
	}
	return labels, nil
}

// decodeLicenseSource decodes the "license_source" argument into one of the
// 3 concrete types.BaseLicenseSource implementations (types.LocalLicenseSource,
// types.LicenseServerSource, types.EvaluationLicenseSource — confirmed as
// the only 3 by grepping referencia/govmomi/vim25/types/types.go for
// "LicenseSource\n" embedded fields), selected by the caller's "type"
// discriminator. types.ConfigureLicenseSourceRequestType.LicenseSource is
// itself the polymorphic field (`xml:"licenseSource,typeattr"`), the same
// kind of field generated_extension.go's decodeExtensionArg works around for
// types.Extension.Description — but here this handler builds the concrete
// struct directly instead of a generic decodeJSONArg pass, since the request
// as a whole is small enough to construct by hand.
func decodeLicenseSource(raw interface{}) (types.BaseLicenseSource, error) {
	m, ok := raw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("license_source must be a JSON object")
	}
	kind, _ := m["type"].(string)
	switch kind {
	case "local":
		keys, _ := m["license_keys"].(string)
		if keys == "" {
			return nil, fmt.Errorf(`license_source.license_keys is required when license_source.type is "local"`)
		}
		return &types.LocalLicenseSource{LicenseKeys: keys}, nil
	case "served":
		server, _ := m["license_server"].(string)
		if server == "" {
			return nil, fmt.Errorf(`license_source.license_server is required when license_source.type is "served"`)
		}
		return &types.LicenseServerSource{LicenseServer: server}, nil
	case "evaluation":
		return &types.EvaluationLicenseSource{}, nil
	default:
		return nil, fmt.Errorf(`license_source.type must be one of "local", "served", "evaluation" (got %q)`, kind)
	}
}

// --- LicenseManager: read-only handlers -----------------------------------

func handleLicenseDecode(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	ref, err := requireLicenseManager(client)
	if err != nil {
		return "", err
	}
	key, err := licenseKeyFromArgs(args)
	if err != nil {
		return "", err
	}

	resp, err := methods.DecodeLicense(ctx, client.Client.Client, &types.DecodeLicense{
		This:       ref,
		LicenseKey: key,
	})
	if err != nil {
		return "", fmt.Errorf("failed to decode license key: %w", err)
	}

	return marshalJSON(map[string]interface{}{"license": resp.Returnval})
}

func handleLicenseQuerySourceAvailability(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	ref, err := requireLicenseManager(client)
	if err != nil {
		return "", err
	}
	hostRef, err := licenseOptionalHostRef(ctx, client, args)
	if err != nil {
		return "", err
	}

	resp, err := methods.QueryLicenseSourceAvailability(ctx, client.Client.Client, &types.QueryLicenseSourceAvailability{
		This: ref,
		Host: hostRef,
	})
	if err != nil {
		return "", fmt.Errorf("failed to query license source availability: %w", err)
	}

	return marshalJSON(map[string]interface{}{"count": len(resp.Returnval), "availability": resp.Returnval})
}

func handleLicenseQuerySupportedFeatures(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	ref, err := requireLicenseManager(client)
	if err != nil {
		return "", err
	}
	hostRef, err := licenseOptionalHostRef(ctx, client, args)
	if err != nil {
		return "", err
	}

	resp, err := methods.QuerySupportedFeatures(ctx, client.Client.Client, &types.QuerySupportedFeatures{
		This: ref,
		Host: hostRef,
	})
	if err != nil {
		return "", fmt.Errorf("failed to query supported license features: %w", err)
	}

	return marshalJSON(map[string]interface{}{"count": len(resp.Returnval), "features": resp.Returnval})
}

func handleLicenseCheckFeature(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	ref, err := requireLicenseManager(client)
	if err != nil {
		return "", err
	}
	hostRef, err := licenseOptionalHostRef(ctx, client, args)
	if err != nil {
		return "", err
	}
	featureKey, _ := args["feature_key"].(string)
	if featureKey == "" {
		return "", fmt.Errorf("feature_key is required")
	}

	resp, err := methods.CheckLicenseFeature(ctx, client.Client.Client, &types.CheckLicenseFeature{
		This:       ref,
		Host:       hostRef,
		FeatureKey: featureKey,
	})
	if err != nil {
		return "", fmt.Errorf("failed to check license feature %q: %w", featureKey, err)
	}

	return marshalJSON(map[string]interface{}{"feature_key": featureKey, "enabled": resp.Returnval})
}

// --- LicenseManager: destructive handlers ---------------------------------

func handleLicenseAdd(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	ref, err := requireLicenseManager(client)
	if err != nil {
		return "", err
	}
	key, err := licenseKeyFromArgs(args)
	if err != nil {
		return "", err
	}
	labels, err := decodeLicenseLabels(args["labels"])
	if err != nil {
		return "", err
	}

	resp, err := methods.AddLicense(ctx, client.Client.Client, &types.AddLicense{
		This:       ref,
		LicenseKey: key,
		Labels:     labels,
	})
	if err != nil {
		return "", fmt.Errorf("failed to add license key: %w", err)
	}

	return marshalJSON(map[string]interface{}{"result": "added", "license": resp.Returnval})
}

func handleLicenseRemove(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	ref, err := requireLicenseManager(client)
	if err != nil {
		return "", err
	}
	key, err := licenseKeyFromArgs(args)
	if err != nil {
		return "", err
	}

	if _, err := methods.RemoveLicense(ctx, client.Client.Client, &types.RemoveLicense{
		This:       ref,
		LicenseKey: key,
	}); err != nil {
		return "", fmt.Errorf("failed to remove license key: %w", err)
	}

	return marshalJSON(map[string]interface{}{"license_key": key, "result": "removed"})
}

func handleLicenseUpdate(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	ref, err := requireLicenseManager(client)
	if err != nil {
		return "", err
	}
	key, err := licenseKeyFromArgs(args)
	if err != nil {
		return "", err
	}
	labels, err := decodeLicenseLabels(args["labels"])
	if err != nil {
		return "", err
	}

	resp, err := methods.UpdateLicense(ctx, client.Client.Client, &types.UpdateLicense{
		This:       ref,
		LicenseKey: key,
		Labels:     labels,
	})
	if err != nil {
		return "", fmt.Errorf("failed to update license key: %w", err)
	}

	return marshalJSON(map[string]interface{}{"result": "updated", "license": resp.Returnval})
}

func handleLicenseUpdateLabel(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	ref, err := requireLicenseManager(client)
	if err != nil {
		return "", err
	}
	key, err := licenseKeyFromArgs(args)
	if err != nil {
		return "", err
	}
	labelKey, _ := args["label_key"].(string)
	if labelKey == "" {
		return "", fmt.Errorf("label_key is required")
	}
	labelValue, ok := args["label_value"].(string)
	if !ok {
		return "", fmt.Errorf("label_value is required (pass an empty string to remove the label)")
	}

	if _, err := methods.UpdateLicenseLabel(ctx, client.Client.Client, &types.UpdateLicenseLabel{
		This:       ref,
		LicenseKey: key,
		LabelKey:   labelKey,
		LabelValue: labelValue,
	}); err != nil {
		return "", fmt.Errorf("failed to update label %q on license key: %w", labelKey, err)
	}

	return marshalJSON(map[string]interface{}{"license_key": key, "label_key": labelKey, "label_value": labelValue, "result": "label_updated"})
}

func handleLicenseConfigureSource(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	ref, err := requireLicenseManager(client)
	if err != nil {
		return "", err
	}
	hostRef, err := licenseOptionalHostRef(ctx, client, args)
	if err != nil {
		return "", err
	}
	if args["license_source"] == nil {
		return "", fmt.Errorf("license_source is required")
	}
	source, err := decodeLicenseSource(args["license_source"])
	if err != nil {
		return "", err
	}

	if _, err := methods.ConfigureLicenseSource(ctx, client.Client.Client, &types.ConfigureLicenseSource{
		This:          ref,
		Host:          hostRef,
		LicenseSource: source,
	}); err != nil {
		return "", fmt.Errorf("failed to configure license source: %w", err)
	}

	return marshalJSON(map[string]interface{}{"result": "license_source_configured"})
}

func handleLicenseSetEdition(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	ref, err := requireLicenseManager(client)
	if err != nil {
		return "", err
	}
	hostRef, err := licenseOptionalHostRef(ctx, client, args)
	if err != nil {
		return "", err
	}
	featureKey, _ := args["feature_key"].(string)

	if _, err := methods.SetLicenseEdition(ctx, client.Client.Client, &types.SetLicenseEdition{
		This:       ref,
		Host:       hostRef,
		FeatureKey: featureKey,
	}); err != nil {
		return "", fmt.Errorf("failed to set license edition: %w", err)
	}

	return marshalJSON(map[string]interface{}{"feature_key": featureKey, "result": "edition_set"})
}

// --- LicenseAssignmentManager: read-only handler --------------------------

func handleLicenseQueryAssigned(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	ref, err := licenseAssignmentManagerRef(ctx, client)
	if err != nil {
		return "", err
	}
	entityID, _ := args["entity_id"].(string)

	resp, err := methods.QueryAssignedLicenses(ctx, client.Client.Client, &types.QueryAssignedLicenses{
		This:     ref,
		EntityId: entityID,
	})
	if err != nil {
		return "", fmt.Errorf("failed to query assigned licenses: %w", err)
	}

	return marshalJSON(map[string]interface{}{"count": len(resp.Returnval), "assignments": resp.Returnval})
}

// --- LicenseAssignmentManager: destructive handlers -----------------------

func handleLicenseUpdateAssigned(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	ref, err := licenseAssignmentManagerRef(ctx, client)
	if err != nil {
		return "", err
	}
	entityID, _ := args["entity_id"].(string)
	if entityID == "" {
		return "", fmt.Errorf("entity_id is required")
	}
	key, err := licenseKeyFromArgs(args)
	if err != nil {
		return "", err
	}
	displayName, _ := args["entity_display_name"].(string)

	resp, err := methods.UpdateAssignedLicense(ctx, client.Client.Client, &types.UpdateAssignedLicense{
		This:              ref,
		Entity:            entityID,
		LicenseKey:        key,
		EntityDisplayName: displayName,
	})
	if err != nil {
		return "", fmt.Errorf("failed to update assigned license for entity %q: %w", entityID, err)
	}

	return marshalJSON(map[string]interface{}{"entity_id": entityID, "result": "assigned", "license": resp.Returnval})
}

func handleLicenseRemoveAssigned(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	ref, err := licenseAssignmentManagerRef(ctx, client)
	if err != nil {
		return "", err
	}
	entityID, _ := args["entity_id"].(string)
	if entityID == "" {
		return "", fmt.Errorf("entity_id is required")
	}

	if _, err := methods.RemoveAssignedLicense(ctx, client.Client.Client, &types.RemoveAssignedLicense{
		This:     ref,
		EntityId: entityID,
	}); err != nil {
		return "", fmt.Errorf("failed to remove assigned license for entity %q: %w", entityID, err)
	}

	return marshalJSON(map[string]interface{}{"entity_id": entityID, "result": "assignment_removed"})
}

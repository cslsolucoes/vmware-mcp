package tools

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// registerHostIscsiTools adds the iSCSI initiator configuration + multipath
// tools that the Fase 3 storage group (generated_host_storage.go) left out:
// object.HostStorageSystem — the high-level govmomi wrapper the codegen
// worked from — exposes only 13 methods, NONE of them iSCSI, so the whole
// InternetScsi* family (and EnableMultipathPath/DisableMultipathPath/
// SetMultipathLunPolicy) had no wrapper to generate a tool from. They DO
// exist on the raw vim25 SOAP HostStorageSystem managed object (72
// InternetScsi references in referencia/govmomi/vim25/methods/methods.go),
// so every handler here calls methods.XxxInternetScsiYyy(ctx,
// client.Client.Client, &types.XxxInternetScsiYyy{This: ss.Reference(), ...})
// directly against the host's StorageSystem MoRef — the same raw-methods
// pattern generated_host_security.go's account handlers use, not an
// object.* convenience wrapper (there is none).
//
// Class: modeVSphereGeneral — iSCSI is host-level configuration that a bare
// ESXi host supports directly, so these must be reachable under --vmware-url
// (standalone ESXi), not just vCenter. This raises the vsphere-general tool
// count from 251 to 266; mode_test.go's vsphereGeneralTools list is updated
// to match.
//
// Tier: every tool is registerDestructive/tier2 — each mutates the host's
// storage adapter configuration (targets, CHAP, IQN, IP, discovery,
// multipath path state/policy). All are reversible (call the counterpart or
// re-set the property), so tier2 (disruptive-but-reversible), not tier1 —
// consistent with the rest of the host-storage config family.
//
// vcsim coverage: the simulator does not implement any of these methods
// server-side, so generated_host_iscsi_test.go drives them with
// assertReachesServer (same helper/rationale as the 15 unsimulated methods in
// generated_host_storage_test.go): the tests prove the wiring — schema, gate,
// resolveHost, StorageSystem MoRef, raw method dispatch — reaches vcsim and
// returns a clean server-side error, not an unknown-tool wiring bug or a
// recovered panic. Behavioral validation is expected against a real ESXi
// host.
//
// Struct-valued arguments (targets, CHAP/digest/discovery/IP properties,
// advanced options, target set) are accepted as JSON objects/arrays decoded
// via decodeJSONArg into the concrete types.* struct — the same "accept the
// concrete struct, not a hand-built schema" approach as
// generated_host_storage.go's create_nas/create_vmfs specs. JSON field names
// are the govmomi types' json tags (camelCase, e.g. "chapAuthEnabled",
// "address", "port").
func registerHostIscsiTools(r *Registry) {
	hostArg := map[string]interface{}{
		"type":        "string",
		"description": `Host identifier: a name/pattern (e.g. "esxi-01.local") or a full inventory path (e.g. "/ha-datacenter/host/esxi-01.local/esxi-01.local") as returned by vmware_list_hosts. Must resolve to exactly one host.`,
	}
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}
	deviceArg := map[string]interface{}{
		"type":        "string",
		"description": `iSCSI HBA device name, e.g. "vmhba64" (the software iSCSI adapter) — see the host's storage adapters (HostHostBusAdapter.device).`,
	}
	targetSetArg := map[string]interface{}{
		"type":        "object",
		"description": `Optional types.HostInternetScsiHbaTargetSet JSON object scoping the change to specific targets: "sendTargets" (array of {address, port}) and/or "staticTargets" (array of {address, port, iScsiName}). Omit to apply the change at the adapter (initiator) level.`,
	}

	// --- Software adapter + identity -------------------------------------

	r.registerDestructive("vmware_host_iscsi_software_set_enabled",
		"Enable or disable the software iSCSI initiator (adapter) on a host. Reversible by calling again with the opposite value.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":    hostArg,
				"enabled": map[string]interface{}{"type": "boolean", "description": "true to enable the software iSCSI adapter, false to disable it."},
				"confirm": confirmArg,
			},
			"required": []interface{}{"host", "enabled", "confirm"},
		},
		Tool{Handler: handleHostIscsiSoftwareSetEnabled},
	)

	r.registerDestructive("vmware_host_iscsi_update_name",
		"Set the iSCSI name (IQN) of an iSCSI HBA on a host. Reversible by setting it back to the previous IQN.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":             hostArg,
				"iscsi_hba_device": deviceArg,
				"iscsi_name":       map[string]interface{}{"type": "string", "description": `The new iSCSI qualified name (IQN), e.g. "iqn.1998-01.com.vmware:esxi-01-1a2b3c4d".`},
				"confirm":          confirmArg,
			},
			"required": []interface{}{"host", "iscsi_hba_device", "iscsi_name", "confirm"},
		},
		Tool{Handler: handleHostIscsiUpdateName},
	)

	r.registerDestructive("vmware_host_iscsi_update_alias",
		"Set the human-readable alias of an iSCSI HBA on a host. Reversible by setting it back.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":             hostArg,
				"iscsi_hba_device": deviceArg,
				"iscsi_alias":      map[string]interface{}{"type": "string", "description": "The new adapter alias."},
				"confirm":          confirmArg,
			},
			"required": []interface{}{"host", "iscsi_hba_device", "iscsi_alias", "confirm"},
		},
		Tool{Handler: handleHostIscsiUpdateAlias},
	)

	// --- Send (dynamic discovery) targets --------------------------------

	r.registerDestructive("vmware_host_iscsi_add_send_targets",
		"Add SendTargets (dynamic discovery) addresses to an iSCSI HBA on a host. Reversible via vmware_host_iscsi_remove_send_targets.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":             hostArg,
				"iscsi_hba_device": deviceArg,
				"targets": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "object"},
					"description": `Array of types.HostInternetScsiHbaSendTarget JSON objects. Each needs at least "address"; "port" defaults to 3260. Optional per-target "authenticationProperties"/"digestProperties".`,
				},
				"confirm": confirmArg,
			},
			"required": []interface{}{"host", "iscsi_hba_device", "targets", "confirm"},
		},
		Tool{Handler: handleHostIscsiAddSendTargets},
	)

	r.registerDestructive("vmware_host_iscsi_remove_send_targets",
		"Remove SendTargets (dynamic discovery) addresses from an iSCSI HBA on a host. Reversible via vmware_host_iscsi_add_send_targets.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":             hostArg,
				"iscsi_hba_device": deviceArg,
				"targets": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "object"},
					"description": `Array of types.HostInternetScsiHbaSendTarget JSON objects to remove (match by "address"/"port").`,
				},
				"confirm": confirmArg,
			},
			"required": []interface{}{"host", "iscsi_hba_device", "targets", "confirm"},
		},
		Tool{Handler: handleHostIscsiRemoveSendTargets},
	)

	// --- Static targets --------------------------------------------------

	r.registerDestructive("vmware_host_iscsi_add_static_targets",
		"Add static targets to an iSCSI HBA on a host. Reversible via vmware_host_iscsi_remove_static_targets.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":             hostArg,
				"iscsi_hba_device": deviceArg,
				"targets": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "object"},
					"description": `Array of types.HostInternetScsiHbaStaticTarget JSON objects. Each needs "address" and "iScsiName" (the target IQN); "port" defaults to 3260.`,
				},
				"confirm": confirmArg,
			},
			"required": []interface{}{"host", "iscsi_hba_device", "targets", "confirm"},
		},
		Tool{Handler: handleHostIscsiAddStaticTargets},
	)

	r.registerDestructive("vmware_host_iscsi_remove_static_targets",
		"Remove static targets from an iSCSI HBA on a host. Reversible via vmware_host_iscsi_add_static_targets.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":             hostArg,
				"iscsi_hba_device": deviceArg,
				"targets": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "object"},
					"description": `Array of types.HostInternetScsiHbaStaticTarget JSON objects to remove (match by "address"/"port"/"iScsiName").`,
				},
				"confirm": confirmArg,
			},
			"required": []interface{}{"host", "iscsi_hba_device", "targets", "confirm"},
		},
		Tool{Handler: handleHostIscsiRemoveStaticTargets},
	)

	// --- Authentication (CHAP) / digest / discovery / IP / advanced ------

	r.registerDestructive("vmware_host_iscsi_update_authentication_properties",
		"Set CHAP authentication properties on an iSCSI HBA (or a specific target set) on a host — one-way and/or mutual CHAP. Reversible by re-setting the properties.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":             hostArg,
				"iscsi_hba_device": deviceArg,
				"authentication_properties": map[string]interface{}{
					"type":        "object",
					"description": `types.HostInternetScsiHbaAuthenticationProperties JSON object: "chapAuthEnabled" (bool), "chapName", "chapSecret", "chapAuthenticationType" ("chapProhibited"|"chapDiscouraged"|"chapPreferred"|"chapRequired"), and the "mutualChap*" equivalents for mutual CHAP.`,
				},
				"target_set": targetSetArg,
				"confirm":    confirmArg,
			},
			"required": []interface{}{"host", "iscsi_hba_device", "authentication_properties", "confirm"},
		},
		Tool{Handler: handleHostIscsiUpdateAuthenticationProperties},
	)

	r.registerDestructive("vmware_host_iscsi_update_digest_properties",
		"Set header/data digest properties on an iSCSI HBA (or a specific target set) on a host. Reversible by re-setting the properties.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":             hostArg,
				"iscsi_hba_device": deviceArg,
				"digest_properties": map[string]interface{}{
					"type":        "object",
					"description": `types.HostInternetScsiHbaDigestProperties JSON object: "headerDigestType"/"dataDigestType" ("digestProhibited"|"digestDiscouraged"|"digestPreferred"|"digestRequired").`,
				},
				"target_set": targetSetArg,
				"confirm":    confirmArg,
			},
			"required": []interface{}{"host", "iscsi_hba_device", "digest_properties", "confirm"},
		},
		Tool{Handler: handleHostIscsiUpdateDigestProperties},
	)

	r.registerDestructive("vmware_host_iscsi_update_discovery_properties",
		"Set discovery properties (iSNS/SLP/static/send-targets discovery enablement) on an iSCSI HBA on a host. Reversible by re-setting the properties.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":             hostArg,
				"iscsi_hba_device": deviceArg,
				"discovery_properties": map[string]interface{}{
					"type":        "object",
					"description": `types.HostInternetScsiHbaDiscoveryProperties JSON object: "iSnsDiscoveryEnabled", "slpDiscoveryEnabled", "staticTargetDiscoveryEnabled", "sendTargetsDiscoveryEnabled" (bools) plus the matching method/host fields.`,
				},
				"confirm": confirmArg,
			},
			"required": []interface{}{"host", "iscsi_hba_device", "discovery_properties", "confirm"},
		},
		Tool{Handler: handleHostIscsiUpdateDiscoveryProperties},
	)

	r.registerDestructive("vmware_host_iscsi_update_ip_properties",
		"Set IP properties (address, subnet, gateway, DHCP, MTU, jumbo frames, DNS) on an independent/dependent hardware iSCSI HBA on a host. Reversible by re-setting the properties.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":             hostArg,
				"iscsi_hba_device": deviceArg,
				"ip_properties": map[string]interface{}{
					"type":        "object",
					"description": `types.HostInternetScsiHbaIPProperties JSON object: "dhcpConfigurationEnabled" (bool), "address", "subnetMask", "defaultGateway", "primaryDnsServerAddress", "mtu", "jumboFramesEnabled", etc.`,
				},
				"confirm": confirmArg,
			},
			"required": []interface{}{"host", "iscsi_hba_device", "ip_properties", "confirm"},
		},
		Tool{Handler: handleHostIscsiUpdateIPProperties},
	)

	r.registerDestructive("vmware_host_iscsi_update_advanced_options",
		"Set advanced options (key/value parameters) on an iSCSI HBA (or a specific target set) on a host. Reversible by re-setting the options.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":             hostArg,
				"iscsi_hba_device": deviceArg,
				"options": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "object"},
					"description": `Array of types.HostInternetScsiHbaParamValue JSON objects, each an OptionValue: "key" (string) and "value".`,
				},
				"target_set": targetSetArg,
				"confirm":    confirmArg,
			},
			"required": []interface{}{"host", "iscsi_hba_device", "options", "confirm"},
		},
		Tool{Handler: handleHostIscsiUpdateAdvancedOptions},
	)

	// --- Multipath (HostStorageSystem) -----------------------------------

	r.registerDestructive("vmware_host_storage_enable_multipath_path",
		"Enable a specific multipath path on a host's storage. Reversible via vmware_host_storage_disable_multipath_path.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":      hostArg,
				"path_name": map[string]interface{}{"type": "string", "description": `Multipath path name (HostMultipathInfoPath.name), e.g. "vmhba64:C0:T0:L0".`},
				"confirm":   confirmArg,
			},
			"required": []interface{}{"host", "path_name", "confirm"},
		},
		Tool{Handler: handleHostStorageEnableMultipathPath},
	)

	r.registerDestructive("vmware_host_storage_disable_multipath_path",
		"Disable a specific multipath path on a host's storage. Reversible via vmware_host_storage_enable_multipath_path.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":      hostArg,
				"path_name": map[string]interface{}{"type": "string", "description": `Multipath path name (HostMultipathInfoPath.name), e.g. "vmhba64:C0:T0:L0".`},
				"confirm":   confirmArg,
			},
			"required": []interface{}{"host", "path_name", "confirm"},
		},
		Tool{Handler: handleHostStorageDisableMultipathPath},
	)

	r.registerDestructive("vmware_host_storage_set_multipath_lun_policy",
		"Set the path selection policy of a multipath LUN on a host. Reversible by setting another policy.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":    hostArg,
				"lun_id":  map[string]interface{}{"type": "string", "description": "Multipath LUN id (HostMultipathInfoLogicalUnit.id)."},
				"policy":  map[string]interface{}{"type": "string", "description": `Path selection policy name (HostMultipathInfoLogicalUnitPolicy.policy), e.g. "VMW_PSP_FIXED", "VMW_PSP_RR", "VMW_PSP_MRU".`},
				"confirm": confirmArg,
			},
			"required": []interface{}{"host", "lun_id", "policy", "confirm"},
		},
		Tool{Handler: handleHostStorageSetMultipathLunPolicy},
	)
}

// hostAndStorageSystem resolves the host and its HostStorageSystem sub-manager
// in one step — every handler in this file needs both (the host for its
// InventoryPath in results, the storage system for its MoRef as the SOAP
// _this). Reuses resolveHost (finder.go) and hostStorageSystem
// (generated_host_storage.go).
func hostAndStorageSystem(ctx context.Context, client *vmware.Client, args map[string]interface{}) (*object.HostSystem, *object.HostStorageSystem, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return nil, nil, err
	}
	ss, err := hostStorageSystem(ctx, host)
	if err != nil {
		return nil, nil, err
	}
	return host, ss, nil
}

// iscsiDevice reads and validates the required iscsi_hba_device argument.
func iscsiDevice(args map[string]interface{}) (string, error) {
	device, _ := args["iscsi_hba_device"].(string)
	if device == "" {
		return "", fmt.Errorf("iscsi_hba_device is required")
	}
	return device, nil
}

// optionalIscsiTargetSet decodes the optional target_set argument into a
// *HostInternetScsiHbaTargetSet, or returns nil when the argument is absent
// (the API treats a nil target set as "apply at the adapter level").
func optionalIscsiTargetSet(args map[string]interface{}) (*types.HostInternetScsiHbaTargetSet, error) {
	raw, ok := args["target_set"]
	if !ok || raw == nil {
		return nil, nil
	}
	ts := &types.HostInternetScsiHbaTargetSet{}
	if err := decodeJSONArg(raw, ts); err != nil {
		return nil, fmt.Errorf("invalid target_set: %w", err)
	}
	return ts, nil
}

func handleHostIscsiSoftwareSetEnabled(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, ss, err := hostAndStorageSystem(ctx, client, args)
	if err != nil {
		return "", err
	}
	rawEnabled, ok := args["enabled"]
	if !ok {
		return "", fmt.Errorf("enabled is required (boolean)")
	}
	enabled, ok := rawEnabled.(bool)
	if !ok {
		return "", fmt.Errorf("enabled must be a boolean")
	}

	if _, err := methods.UpdateSoftwareInternetScsiEnabled(ctx, client.Client.Client, &types.UpdateSoftwareInternetScsiEnabled{
		This:    ss.Reference(),
		Enabled: enabled,
	}); err != nil {
		return "", fmt.Errorf("failed to set software iSCSI enabled=%v on %s: %w", enabled, host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "enabled": enabled, "result": "software_iscsi_updated"})
}

func handleHostIscsiUpdateName(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, ss, err := hostAndStorageSystem(ctx, client, args)
	if err != nil {
		return "", err
	}
	device, err := iscsiDevice(args)
	if err != nil {
		return "", err
	}
	name, _ := args["iscsi_name"].(string)
	if name == "" {
		return "", fmt.Errorf("iscsi_name is required")
	}

	if _, err := methods.UpdateInternetScsiName(ctx, client.Client.Client, &types.UpdateInternetScsiName{
		This:           ss.Reference(),
		IScsiHbaDevice: device,
		IScsiName:      name,
	}); err != nil {
		return "", fmt.Errorf("failed to update iSCSI name on %s (%s): %w", device, host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "iscsi_hba_device": device, "iscsi_name": name, "result": "iscsi_name_updated"})
}

func handleHostIscsiUpdateAlias(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, ss, err := hostAndStorageSystem(ctx, client, args)
	if err != nil {
		return "", err
	}
	device, err := iscsiDevice(args)
	if err != nil {
		return "", err
	}
	alias, _ := args["iscsi_alias"].(string)
	if alias == "" {
		return "", fmt.Errorf("iscsi_alias is required")
	}

	if _, err := methods.UpdateInternetScsiAlias(ctx, client.Client.Client, &types.UpdateInternetScsiAlias{
		This:           ss.Reference(),
		IScsiHbaDevice: device,
		IScsiAlias:     alias,
	}); err != nil {
		return "", fmt.Errorf("failed to update iSCSI alias on %s (%s): %w", device, host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "iscsi_hba_device": device, "iscsi_alias": alias, "result": "iscsi_alias_updated"})
}

func handleHostIscsiAddSendTargets(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, ss, err := hostAndStorageSystem(ctx, client, args)
	if err != nil {
		return "", err
	}
	device, err := iscsiDevice(args)
	if err != nil {
		return "", err
	}
	targets, err := decodeIscsiSendTargets(args)
	if err != nil {
		return "", err
	}

	if _, err := methods.AddInternetScsiSendTargets(ctx, client.Client.Client, &types.AddInternetScsiSendTargets{
		This:           ss.Reference(),
		IScsiHbaDevice: device,
		Targets:        targets,
	}); err != nil {
		return "", fmt.Errorf("failed to add iSCSI send targets on %s (%s): %w", device, host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "iscsi_hba_device": device, "count": len(targets), "result": "send_targets_added"})
}

func handleHostIscsiRemoveSendTargets(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, ss, err := hostAndStorageSystem(ctx, client, args)
	if err != nil {
		return "", err
	}
	device, err := iscsiDevice(args)
	if err != nil {
		return "", err
	}
	targets, err := decodeIscsiSendTargets(args)
	if err != nil {
		return "", err
	}

	if _, err := methods.RemoveInternetScsiSendTargets(ctx, client.Client.Client, &types.RemoveInternetScsiSendTargets{
		This:           ss.Reference(),
		IScsiHbaDevice: device,
		Targets:        targets,
	}); err != nil {
		return "", fmt.Errorf("failed to remove iSCSI send targets on %s (%s): %w", device, host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "iscsi_hba_device": device, "count": len(targets), "result": "send_targets_removed"})
}

func handleHostIscsiAddStaticTargets(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, ss, err := hostAndStorageSystem(ctx, client, args)
	if err != nil {
		return "", err
	}
	device, err := iscsiDevice(args)
	if err != nil {
		return "", err
	}
	targets, err := decodeIscsiStaticTargets(args)
	if err != nil {
		return "", err
	}

	if _, err := methods.AddInternetScsiStaticTargets(ctx, client.Client.Client, &types.AddInternetScsiStaticTargets{
		This:           ss.Reference(),
		IScsiHbaDevice: device,
		Targets:        targets,
	}); err != nil {
		return "", fmt.Errorf("failed to add iSCSI static targets on %s (%s): %w", device, host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "iscsi_hba_device": device, "count": len(targets), "result": "static_targets_added"})
}

func handleHostIscsiRemoveStaticTargets(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, ss, err := hostAndStorageSystem(ctx, client, args)
	if err != nil {
		return "", err
	}
	device, err := iscsiDevice(args)
	if err != nil {
		return "", err
	}
	targets, err := decodeIscsiStaticTargets(args)
	if err != nil {
		return "", err
	}

	if _, err := methods.RemoveInternetScsiStaticTargets(ctx, client.Client.Client, &types.RemoveInternetScsiStaticTargets{
		This:           ss.Reference(),
		IScsiHbaDevice: device,
		Targets:        targets,
	}); err != nil {
		return "", fmt.Errorf("failed to remove iSCSI static targets on %s (%s): %w", device, host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "iscsi_hba_device": device, "count": len(targets), "result": "static_targets_removed"})
}

func handleHostIscsiUpdateAuthenticationProperties(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, ss, err := hostAndStorageSystem(ctx, client, args)
	if err != nil {
		return "", err
	}
	device, err := iscsiDevice(args)
	if err != nil {
		return "", err
	}
	raw, ok := args["authentication_properties"]
	if !ok {
		return "", fmt.Errorf("authentication_properties is required")
	}
	var props types.HostInternetScsiHbaAuthenticationProperties
	if err := decodeJSONArg(raw, &props); err != nil {
		return "", fmt.Errorf("invalid authentication_properties: %w", err)
	}
	targetSet, err := optionalIscsiTargetSet(args)
	if err != nil {
		return "", err
	}

	if _, err := methods.UpdateInternetScsiAuthenticationProperties(ctx, client.Client.Client, &types.UpdateInternetScsiAuthenticationProperties{
		This:                     ss.Reference(),
		IScsiHbaDevice:           device,
		AuthenticationProperties: props,
		TargetSet:                targetSet,
	}); err != nil {
		return "", fmt.Errorf("failed to update iSCSI authentication properties on %s (%s): %w", device, host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "iscsi_hba_device": device, "result": "authentication_properties_updated"})
}

func handleHostIscsiUpdateDigestProperties(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, ss, err := hostAndStorageSystem(ctx, client, args)
	if err != nil {
		return "", err
	}
	device, err := iscsiDevice(args)
	if err != nil {
		return "", err
	}
	raw, ok := args["digest_properties"]
	if !ok {
		return "", fmt.Errorf("digest_properties is required")
	}
	var props types.HostInternetScsiHbaDigestProperties
	if err := decodeJSONArg(raw, &props); err != nil {
		return "", fmt.Errorf("invalid digest_properties: %w", err)
	}
	targetSet, err := optionalIscsiTargetSet(args)
	if err != nil {
		return "", err
	}

	if _, err := methods.UpdateInternetScsiDigestProperties(ctx, client.Client.Client, &types.UpdateInternetScsiDigestProperties{
		This:             ss.Reference(),
		IScsiHbaDevice:   device,
		TargetSet:        targetSet,
		DigestProperties: props,
	}); err != nil {
		return "", fmt.Errorf("failed to update iSCSI digest properties on %s (%s): %w", device, host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "iscsi_hba_device": device, "result": "digest_properties_updated"})
}

func handleHostIscsiUpdateDiscoveryProperties(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, ss, err := hostAndStorageSystem(ctx, client, args)
	if err != nil {
		return "", err
	}
	device, err := iscsiDevice(args)
	if err != nil {
		return "", err
	}
	raw, ok := args["discovery_properties"]
	if !ok {
		return "", fmt.Errorf("discovery_properties is required")
	}
	var props types.HostInternetScsiHbaDiscoveryProperties
	if err := decodeJSONArg(raw, &props); err != nil {
		return "", fmt.Errorf("invalid discovery_properties: %w", err)
	}

	if _, err := methods.UpdateInternetScsiDiscoveryProperties(ctx, client.Client.Client, &types.UpdateInternetScsiDiscoveryProperties{
		This:                ss.Reference(),
		IScsiHbaDevice:      device,
		DiscoveryProperties: props,
	}); err != nil {
		return "", fmt.Errorf("failed to update iSCSI discovery properties on %s (%s): %w", device, host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "iscsi_hba_device": device, "result": "discovery_properties_updated"})
}

func handleHostIscsiUpdateIPProperties(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, ss, err := hostAndStorageSystem(ctx, client, args)
	if err != nil {
		return "", err
	}
	device, err := iscsiDevice(args)
	if err != nil {
		return "", err
	}
	raw, ok := args["ip_properties"]
	if !ok {
		return "", fmt.Errorf("ip_properties is required")
	}
	var props types.HostInternetScsiHbaIPProperties
	if err := decodeJSONArg(raw, &props); err != nil {
		return "", fmt.Errorf("invalid ip_properties: %w", err)
	}

	if _, err := methods.UpdateInternetScsiIPProperties(ctx, client.Client.Client, &types.UpdateInternetScsiIPProperties{
		This:           ss.Reference(),
		IScsiHbaDevice: device,
		IpProperties:   props,
	}); err != nil {
		return "", fmt.Errorf("failed to update iSCSI IP properties on %s (%s): %w", device, host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "iscsi_hba_device": device, "result": "ip_properties_updated"})
}

func handleHostIscsiUpdateAdvancedOptions(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, ss, err := hostAndStorageSystem(ctx, client, args)
	if err != nil {
		return "", err
	}
	device, err := iscsiDevice(args)
	if err != nil {
		return "", err
	}
	raw, ok := args["options"]
	if !ok {
		return "", fmt.Errorf("options is required")
	}
	var opts []types.HostInternetScsiHbaParamValue
	if err := decodeJSONArg(raw, &opts); err != nil {
		return "", fmt.Errorf("invalid options: %w", err)
	}
	targetSet, err := optionalIscsiTargetSet(args)
	if err != nil {
		return "", err
	}

	if _, err := methods.UpdateInternetScsiAdvancedOptions(ctx, client.Client.Client, &types.UpdateInternetScsiAdvancedOptions{
		This:           ss.Reference(),
		IScsiHbaDevice: device,
		TargetSet:      targetSet,
		Options:        opts,
	}); err != nil {
		return "", fmt.Errorf("failed to update iSCSI advanced options on %s (%s): %w", device, host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "iscsi_hba_device": device, "count": len(opts), "result": "advanced_options_updated"})
}

func handleHostStorageEnableMultipathPath(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, ss, err := hostAndStorageSystem(ctx, client, args)
	if err != nil {
		return "", err
	}
	pathName, _ := args["path_name"].(string)
	if pathName == "" {
		return "", fmt.Errorf("path_name is required")
	}

	if _, err := methods.EnableMultipathPath(ctx, client.Client.Client, &types.EnableMultipathPath{
		This:     ss.Reference(),
		PathName: pathName,
	}); err != nil {
		return "", fmt.Errorf("failed to enable multipath path %s on %s: %w", pathName, host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "path_name": pathName, "result": "path_enabled"})
}

func handleHostStorageDisableMultipathPath(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, ss, err := hostAndStorageSystem(ctx, client, args)
	if err != nil {
		return "", err
	}
	pathName, _ := args["path_name"].(string)
	if pathName == "" {
		return "", fmt.Errorf("path_name is required")
	}

	if _, err := methods.DisableMultipathPath(ctx, client.Client.Client, &types.DisableMultipathPath{
		This:     ss.Reference(),
		PathName: pathName,
	}); err != nil {
		return "", fmt.Errorf("failed to disable multipath path %s on %s: %w", pathName, host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "path_name": pathName, "result": "path_disabled"})
}

func handleHostStorageSetMultipathLunPolicy(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, ss, err := hostAndStorageSystem(ctx, client, args)
	if err != nil {
		return "", err
	}
	lunID, _ := args["lun_id"].(string)
	if lunID == "" {
		return "", fmt.Errorf("lun_id is required")
	}
	policy, _ := args["policy"].(string)
	if policy == "" {
		return "", fmt.Errorf("policy is required")
	}

	if _, err := methods.SetMultipathLunPolicy(ctx, client.Client.Client, &types.SetMultipathLunPolicy{
		This:   ss.Reference(),
		LunId:  lunID,
		Policy: &types.HostMultipathInfoLogicalUnitPolicy{Policy: policy},
	}); err != nil {
		return "", fmt.Errorf("failed to set multipath LUN policy %q for %s on %s: %w", policy, lunID, host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "lun_id": lunID, "policy": policy, "result": "lun_policy_updated"})
}

// decodeIscsiSendTargets decodes the required "targets" argument into a slice
// of HostInternetScsiHbaSendTarget (dynamic discovery). Shared by the add and
// remove send-target handlers.
func decodeIscsiSendTargets(args map[string]interface{}) ([]types.HostInternetScsiHbaSendTarget, error) {
	raw, ok := args["targets"]
	if !ok {
		return nil, fmt.Errorf("targets is required")
	}
	var targets []types.HostInternetScsiHbaSendTarget
	if err := decodeJSONArg(raw, &targets); err != nil {
		return nil, fmt.Errorf("invalid targets: %w", err)
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("targets must be a non-empty array")
	}
	return targets, nil
}

// decodeIscsiStaticTargets decodes the required "targets" argument into a
// slice of HostInternetScsiHbaStaticTarget. Shared by the add and remove
// static-target handlers.
func decodeIscsiStaticTargets(args map[string]interface{}) ([]types.HostInternetScsiHbaStaticTarget, error) {
	raw, ok := args["targets"]
	if !ok {
		return nil, fmt.Errorf("targets is required")
	}
	var targets []types.HostInternetScsiHbaStaticTarget
	if err := decodeJSONArg(raw, &targets); err != nil {
		return nil, fmt.Errorf("invalid targets: %w", err)
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("targets must be a non-empty array")
	}
	return targets, nil
}

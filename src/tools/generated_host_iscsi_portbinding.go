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

// registerHostIscsiPortBindingTools adds the iSCSI port binding (VMkernel
// NIC ↔ software iSCSI HBA) tools that neither generated_host_iscsi.go nor
// generated_host_storage.go cover: those two files were generated from
// object.HostStorageSystem (72 InternetScsi* raw SOAP methods, no
// object.* wrapper) — but BindVnic/UnbindVnic/QueryBoundVnics/
// QueryCandidateNics/QueryMigrationDependencies/QueryVnicStatus/
// QueryPnicStatus are NOT HostStorageSystem methods at all. They belong to a
// separate managed object, IscsiManager (referencia/govmomi/vim25/mo/mo.go's
// mo.IscsiManager), reachable from a host only via
// HostConfigManager.IscsiManager (types.go) — object.HostConfigManager
// (referencia/govmomi/object/host_config_manager.go) has accessors for
// StorageSystem/DatastoreSystem/NetworkSystem/etc. but none for IscsiManager,
// so there is no object.* wrapper to call and, like the HostStorageSystem
// iSCSI family, every handler here dials the raw vim25 SOAP method directly:
// methods.Xxx(ctx, client.Client.Client, &types.Xxx{This: ref, ...}) against
// the IscsiManager MoRef resolved by this file's hostIscsiManager helper
// (host.Properties(ctx, host.Reference(), []string{"configManager.iscsiManager"},
// &mo.HostSystem{}) — the same Properties-read pattern host.go's handleHostInfo
// already uses for "summary").
//
// Class: modeVSphereGeneral — iSCSI port binding is host-level configuration
// a bare ESXi host supports directly (not vCenter-only), consistent with
// generated_host_iscsi.go's classification. Adds 7 tools to that class;
// mode_test.go's vsphereGeneralTools list is updated to match.
//
// Tier: BindVnic/UnbindVnic mutate the host's iSCSI port binding
// (registerDestructive/tier2 — disruptive to iSCSI I/O over that VMkernel
// NIC but reversible by calling the counterpart). The five Query* methods
// are read-only (r.register, no confirm/tier).
//
// vcsim coverage: the simulator's ESX() model does populate
// configManager.iscsiManager with a MoRef (referencia/govmomi/simulator/esx/
// host_system.go: {Type: "IscsiManager", Value: "iscsiManager"}), so
// hostIscsiManager resolves successfully — but vcsim registers no
// simulator.IscsiManager object to back that MoRef (no BindVnic/QueryXxx
// handler exists anywhere under referencia/govmomi/simulator/), so the raw
// SOAP call itself reaches vcsim's dispatcher and comes back with a clean
// ManagedObjectNotFound-style fault. generated_host_iscsi_portbinding_test.go
// drives that path with assertReachesServer (same helper/rationale as
// generated_host_iscsi_test.go's 15 unsimulated HostStorageSystem methods):
// the tests prove the wiring — schema, gate, resolveHost, IscsiManager MoRef
// resolution, raw method dispatch — reaches vcsim and returns a clean
// server-side error, not an unknown-tool wiring bug or a recovered panic.
// Behavioral validation is expected against a real ESXi host with a software
// iSCSI adapter configured.
func registerHostIscsiPortBindingTools(r *Registry) {
	hostArg := map[string]interface{}{
		"type":        "string",
		"description": `Host identifier: a name/pattern (e.g. "esxi-01.local") or a full inventory path (e.g. "/ha-datacenter/host/esxi-01.local/esxi-01.local") as returned by vmware_list_hosts. Must resolve to exactly one host.`,
	}
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}
	iscsiHbaNameArg := map[string]interface{}{
		"type":        "string",
		"description": `iSCSI HBA (adapter) name, e.g. "vmhba64" (the software iSCSI adapter) — see the host's storage adapters (HostHostBusAdapter.device).`,
	}
	vnicDeviceArg := map[string]interface{}{
		"type":        "string",
		"description": `VMkernel NIC device name, e.g. "vmk1" — see the host's virtual NICs (HostVirtualNic.device).`,
	}
	pnicDeviceArg := map[string]interface{}{
		"type":        "string",
		"description": `Physical NIC device name, e.g. "vmnic1" — see the host's physical NICs (PhysicalNic.device).`,
	}

	// --- Bind / unbind (destructive, tier2) ------------------------------

	r.registerDestructive("vmware_host_iscsi_bind_vnic",
		"Bind a VMkernel NIC to a software iSCSI HBA on a host (port binding for multipathing). Reversible via vmware_host_iscsi_unbind_vnic.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":           hostArg,
				"iscsi_hba_name": iscsiHbaNameArg,
				"vnic_device":    vnicDeviceArg,
				"confirm":        confirmArg,
			},
			"required": []interface{}{"host", "iscsi_hba_name", "vnic_device", "confirm"},
		},
		Tool{Handler: handleHostIscsiBindVnic},
	)

	r.registerDestructive("vmware_host_iscsi_unbind_vnic",
		"Unbind a VMkernel NIC from a software iSCSI HBA on a host. Reversible via vmware_host_iscsi_bind_vnic.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":           hostArg,
				"iscsi_hba_name": iscsiHbaNameArg,
				"vnic_device":    vnicDeviceArg,
				"force":          map[string]interface{}{"type": "boolean", "description": "Force the unbind even if it would disrupt active iSCSI sessions/paths. Defaults to false."},
				"confirm":        confirmArg,
			},
			"required": []interface{}{"host", "iscsi_hba_name", "vnic_device", "confirm"},
		},
		Tool{Handler: handleHostIscsiUnbindVnic},
	)

	// --- Query (read-only) ------------------------------------------------

	r.register("vmware_host_iscsi_query_bound_vnics",
		"List the VMkernel NICs currently bound to a software iSCSI HBA on a host, with their port binding compliance status.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":           hostArg,
				"iscsi_hba_name": iscsiHbaNameArg,
			},
			"required": []interface{}{"host", "iscsi_hba_name"},
		},
		Tool{Handler: handleHostIscsiQueryBoundVnics},
	)

	r.register("vmware_host_iscsi_query_candidate_nics",
		"List the VMkernel NICs on a host that are eligible to be bound to a software iSCSI HBA (candidates for vmware_host_iscsi_bind_vnic), with their compliance status.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":           hostArg,
				"iscsi_hba_name": iscsiHbaNameArg,
			},
			"required": []interface{}{"host", "iscsi_hba_name"},
		},
		Tool{Handler: handleHostIscsiQueryCandidateNics},
	)

	r.register("vmware_host_iscsi_query_migration_dependencies",
		"Check whether migrating one or more physical NICs on a host would affect any VMkernel NIC currently bound to a software iSCSI HBA (port binding dependency check).",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host": hostArg,
				"pnic_devices": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": `Physical NIC device names to check, e.g. ["vmnic1", "vmnic2"].`,
				},
			},
			"required": []interface{}{"host", "pnic_devices"},
		},
		Tool{Handler: handleHostIscsiQueryMigrationDependencies},
	)

	r.register("vmware_host_iscsi_query_vnic_status",
		"Get the iSCSI port binding compliance status of a single VMkernel NIC on a host.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":        hostArg,
				"vnic_device": vnicDeviceArg,
			},
			"required": []interface{}{"host", "vnic_device"},
		},
		Tool{Handler: handleHostIscsiQueryVnicStatus},
	)

	r.register("vmware_host_iscsi_query_pnic_status",
		"Get the iSCSI port binding compliance status of a single physical NIC on a host.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":        hostArg,
				"pnic_device": pnicDeviceArg,
			},
			"required": []interface{}{"host", "pnic_device"},
		},
		Tool{Handler: handleHostIscsiQueryPnicStatus},
	)
}

// hostIscsiManager resolves the host and its IscsiManager MoRef in one step.
// object.HostConfigManager has no IscsiManager accessor (see this file's top
// doc comment), so this reads configManager.iscsiManager directly via
// mo.HostSystem — the same Properties-read pattern host.go's handleHostInfo
// uses for "summary". A host with no iSCSI HBA configured has a nil
// IscsiManager reference; that is reported as an error rather than a zero
// MoRef reaching the SOAP call.
func hostIscsiManager(ctx context.Context, client *vmware.Client, args map[string]interface{}) (*object.HostSystem, types.ManagedObjectReference, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return nil, types.ManagedObjectReference{}, err
	}
	var h mo.HostSystem
	if err := host.Properties(ctx, host.Reference(), []string{"configManager.iscsiManager"}, &h); err != nil {
		return nil, types.ManagedObjectReference{}, fmt.Errorf("failed to read iSCSI manager for %s: %w", host.InventoryPath, err)
	}
	if h.ConfigManager.IscsiManager == nil {
		return nil, types.ManagedObjectReference{}, fmt.Errorf("host %s does not expose an iSCSI manager (no iSCSI HBA?)", host.InventoryPath)
	}
	return host, *h.ConfigManager.IscsiManager, nil
}

// iscsiHbaName reads and validates the required iscsi_hba_name argument.
func iscsiHbaName(args map[string]interface{}) (string, error) {
	name, _ := args["iscsi_hba_name"].(string)
	if name == "" {
		return "", fmt.Errorf("iscsi_hba_name is required")
	}
	return name, nil
}

// iscsiVnicDevice reads and validates the required vnic_device argument.
func iscsiVnicDevice(args map[string]interface{}) (string, error) {
	device, _ := args["vnic_device"].(string)
	if device == "" {
		return "", fmt.Errorf("vnic_device is required")
	}
	return device, nil
}

func handleHostIscsiBindVnic(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, ref, err := hostIscsiManager(ctx, client, args)
	if err != nil {
		return "", err
	}
	hbaName, err := iscsiHbaName(args)
	if err != nil {
		return "", err
	}
	vnicDevice, err := iscsiVnicDevice(args)
	if err != nil {
		return "", err
	}

	if _, err := methods.BindVnic(ctx, client.Client.Client, &types.BindVnic{
		This:         ref,
		IScsiHbaName: hbaName,
		VnicDevice:   vnicDevice,
	}); err != nil {
		return "", fmt.Errorf("failed to bind vnic %s to iSCSI HBA %s on %s: %w", vnicDevice, hbaName, host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "iscsi_hba_name": hbaName, "vnic_device": vnicDevice, "result": "vnic_bound"})
}

func handleHostIscsiUnbindVnic(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, ref, err := hostIscsiManager(ctx, client, args)
	if err != nil {
		return "", err
	}
	hbaName, err := iscsiHbaName(args)
	if err != nil {
		return "", err
	}
	vnicDevice, err := iscsiVnicDevice(args)
	if err != nil {
		return "", err
	}
	force, _ := args["force"].(bool)

	if _, err := methods.UnbindVnic(ctx, client.Client.Client, &types.UnbindVnic{
		This:         ref,
		IScsiHbaName: hbaName,
		VnicDevice:   vnicDevice,
		Force:        force,
	}); err != nil {
		return "", fmt.Errorf("failed to unbind vnic %s from iSCSI HBA %s on %s: %w", vnicDevice, hbaName, host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "iscsi_hba_name": hbaName, "vnic_device": vnicDevice, "force": force, "result": "vnic_unbound"})
}

func handleHostIscsiQueryBoundVnics(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, ref, err := hostIscsiManager(ctx, client, args)
	if err != nil {
		return "", err
	}
	hbaName, err := iscsiHbaName(args)
	if err != nil {
		return "", err
	}

	resp, err := methods.QueryBoundVnics(ctx, client.Client.Client, &types.QueryBoundVnics{
		This:         ref,
		IScsiHbaName: hbaName,
	})
	if err != nil {
		return "", fmt.Errorf("failed to query bound vnics for iSCSI HBA %s on %s: %w", hbaName, host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{
		"host":           host.InventoryPath,
		"iscsi_hba_name": hbaName,
		"count":          len(resp.Returnval),
		"vnics":          resp.Returnval,
	})
}

func handleHostIscsiQueryCandidateNics(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, ref, err := hostIscsiManager(ctx, client, args)
	if err != nil {
		return "", err
	}
	hbaName, err := iscsiHbaName(args)
	if err != nil {
		return "", err
	}

	resp, err := methods.QueryCandidateNics(ctx, client.Client.Client, &types.QueryCandidateNics{
		This:         ref,
		IScsiHbaName: hbaName,
	})
	if err != nil {
		return "", fmt.Errorf("failed to query candidate nics for iSCSI HBA %s on %s: %w", hbaName, host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{
		"host":           host.InventoryPath,
		"iscsi_hba_name": hbaName,
		"count":          len(resp.Returnval),
		"vnics":          resp.Returnval,
	})
}

func handleHostIscsiQueryMigrationDependencies(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, ref, err := hostIscsiManager(ctx, client, args)
	if err != nil {
		return "", err
	}
	raw, ok := args["pnic_devices"]
	if !ok {
		return "", fmt.Errorf("pnic_devices is required")
	}
	pnicDevices, err := toStringSlice(raw)
	if err != nil {
		return "", fmt.Errorf("invalid pnic_devices: %w", err)
	}
	if len(pnicDevices) == 0 {
		return "", fmt.Errorf("pnic_devices must be a non-empty array")
	}

	resp, err := methods.QueryMigrationDependencies(ctx, client.Client.Client, &types.QueryMigrationDependencies{
		This:       ref,
		PnicDevice: pnicDevices,
	})
	if err != nil {
		return "", fmt.Errorf("failed to query iSCSI migration dependencies for %v on %s: %w", pnicDevices, host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{
		"host":         host.InventoryPath,
		"pnic_devices": pnicDevices,
		"dependency":   resp.Returnval,
	})
}

func handleHostIscsiQueryVnicStatus(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, ref, err := hostIscsiManager(ctx, client, args)
	if err != nil {
		return "", err
	}
	vnicDevice, err := iscsiVnicDevice(args)
	if err != nil {
		return "", err
	}

	resp, err := methods.QueryVnicStatus(ctx, client.Client.Client, &types.QueryVnicStatus{
		This:       ref,
		VnicDevice: vnicDevice,
	})
	if err != nil {
		return "", fmt.Errorf("failed to query vnic status for %s on %s: %w", vnicDevice, host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{
		"host":        host.InventoryPath,
		"vnic_device": vnicDevice,
		"status":      resp.Returnval,
	})
}

func handleHostIscsiQueryPnicStatus(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, ref, err := hostIscsiManager(ctx, client, args)
	if err != nil {
		return "", err
	}
	pnicDevice, _ := args["pnic_device"].(string)
	if pnicDevice == "" {
		return "", fmt.Errorf("pnic_device is required")
	}

	resp, err := methods.QueryPnicStatus(ctx, client.Client.Client, &types.QueryPnicStatus{
		This:       ref,
		PnicDevice: pnicDevice,
	})
	if err != nil {
		return "", fmt.Errorf("failed to query pnic status for %s on %s: %w", pnicDevice, host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{
		"host":        host.InventoryPath,
		"pnic_device": pnicDevice,
		"status":      resp.Returnval,
	})
}

package tools

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// registerVsanTools adds the HostVsanSystem and HostVsanInternalSystem
// methods that generated_host_misc.go's vsan section does NOT already cover
// — confirmed by grepping every generated_*.go file in this package for
// "vmware_host_vsan" before writing this one (see this task's own grep):
// vmware_host_vsan_update (HostVsanSystem.Update/UpdateVsan_Task),
// vmware_host_vsan_internal_query_object_uuids
// (QueryVsanObjectUuidsByFilter), vmware_host_vsan_internal_get_obj_ext_attrs
// (GetVsanObjExtAttrs) and vmware_host_vsan_internal_delete_objects
// (DeleteVsanObjects) already exist there and are NOT reimplemented here.
//
// Every method added here was confirmed to exist for real in
// github.com/vmware/govmomi@v0.55.1's vim25/types/types.go and
// vim25/methods/methods.go (module cache, not referencia/ — this project
// vendors via go.mod, referencia/ is documentation only) before being
// written — 19 methods total:
//
//   - HostVsanSystem (9): QueryHostStatus, QueryDisksForVsan,
//     AddDisks_Task, InitializeDisks_Task, RemoveDisk_Task,
//     RemoveDiskMapping_Task, UnmountDiskMapping_Task, EvacuateVsanNode_Task,
//     RecommissionVsanNode_Task.
//   - HostVsanInternalSystem (10): QueryVsanObjects, ReconfigureDomObject,
//     QueryObjectsOnPhysicalVsanDisk, AbdicateDomOwnership,
//     QueryVsanStatistics, RunVsanPhysicalDiskDiagnostics, QueryCmmds,
//     QuerySyncingVsanObjects, UpgradeVsanObjects, QueryPhysicalVsanDisks.
//
// No object.* wrapper exists for any of these 19 (object/host_vsan_system.go
// only wraps Update/updateVnic; object/host_vsan_internal_system.go only
// wraps QueryVsanObjectUuidsByFilter/GetVsanObjExtAttrs/DeleteVsanObjects —
// confirmed by grepping "^func" in both files) — every handler below
// therefore dials the raw vim25 SOAP method directly:
// methods.Xxx(ctx, client.Client.Client, &types.Xxx{This: ref, ...}), the
// same "no object.* wrapper, go straight to methods+types" pattern
// generated_host_iscsi_portbinding.go and generated_vm_ft.go document at
// length. The MoRef itself IS reached via the object layer though —
// host.ConfigManager().VsanSystem(ctx)/.VsanInternalSystem(ctx)
// (object/host_config_manager.go has accessors for both, unlike
// IscsiManager) — the same access path generated_host_misc.go's existing
// vsan handlers (handleHostVsanUpdate,
// handleHostVsanInternalQueryObjectUUIDs/GetObjExtAttrs/DeleteObjects)
// already use, reused here via this file's vsanSystem/vsanInternalSystem
// helpers rather than re-deriving the MoRef by hand via a property-collector
// read of configManager.vsanSystem/vsanInternalSystem (both are equivalent —
// HostConfigManager.VsanSystem/VsanInternalSystem's own implementation is
// exactly that property read, see object/host_config_manager.go's
// reference() helper — but reusing the accessor already established in this
// package keeps this file consistent with the 4 vsan tools next to it
// instead of introducing a second way to reach the same MoRef).
//
// Every request struct's field names/pointer-ness/required-ness was read
// directly from types.go (not assumed) — HostScsiDisk (AddDisks_Task/
// RemoveDisk_Task's "disk" field) and VsanHostDiskMapping
// (InitializeDisks_Task/RemoveDiskMapping_Task/UnmountDiskMapping_Task's
// "mapping" field) are both large nested vim25 structs with no compact
// scalar shape, so — like generated_vm_ft.go's CreateSecondaryVMEx "spec"
// argument and generated_host_misc.go's vsan_update "config" argument —
// they are accepted as raw JSON arrays/objects and decoded via
// decodeJSONArg (generated_vm_lifecycle.go) into the real govmomi struct,
// not hand-built as a nested MCP schema.
//
// Class: modeVSphereGeneral, matching generated_host_misc.go's existing
// vsan tools — HostVsanSystem/HostVsanInternalSystem are host-scoped managed
// objects reachable via ConfigManager on a standalone ESXi connection too,
// not a vCenter-only cluster feature.
//
// Tier judgment calls (this project's tier1=irreversible / tier2=disruptive-
// but-reversible split — see destructive.go):
//   - QueryHostStatus, QueryDisksForVsan, QueryVsanObjects,
//     QueryObjectsOnPhysicalVsanDisk, QueryVsanStatistics, QueryCmmds,
//     QuerySyncingVsanObjects, QueryPhysicalVsanDisks are plain reads —
//     r.register, no tier.
//   - AddDisks_Task/InitializeDisks_Task/RemoveDisk_Task/
//     RemoveDiskMapping_Task/UnmountDiskMapping_Task/EvacuateVsanNode_Task/
//     RecommissionVsanNode_Task are Tier 2: they change which physical disks
//     back a host's vSAN storage or its cluster membership, but none of them
//     is a data-destroying delete in the vmware_vm_destroy/
//     vsan_internal_delete_objects sense — a disk can be re-added/
//     re-initialized, a node re-evacuated/re-commissioned. This mirrors
//     generated_host_storage.go's own precedent: even
//     vmware_host_datastore_create_vmfs (which reformats a disk, destroying
//     any prior VMFS on it) is classified Tier 2 in this project, not Tier 1
//     — Tier 1 is reserved for outright object/VM destruction.
//   - ReconfigureDomObject/AbdicateDomOwnership/UpgradeVsanObjects (the 3
//     HostVsanInternalSystem methods that mutate rather than query) are also
//     Tier 2 for the same reason: a DOM object's storage policy or ownership
//     can be reconfigured again, and format-upgrading a vSAN object does not
//     destroy any data. RunVsanPhysicalDiskDiagnostics is Tier 2 too even
//     though it does not persist any config change — it actively drives I/O
//     against physical vSAN disks (per its own doc comment, "This API can be
//     slow"), the same "active/disruptive action on live storage, not a
//     passive read" reasoning generated_host_storage.go applies to
//     vmware_host_storage_rescan_all_hba/rescan_vmfs.
//
// vcsim coverage: NONE of the 19 reach a working simulator object — every
// one goes through the exact same host.ConfigManager().VsanSystem(ctx)/
// .VsanInternalSystem(ctx) MoRef resolution generated_host_misc.go's
// existing 4 vsan tools use, and that file's top doc comment already
// documents (independently re-verified, not just trusted) that vcsim's
// static ESX() host template gives VsanSystem/VsanInternalSystem a
// well-formed placeholder ManagedObjectReference
// ("HostVsanSystem:vsanSystem" / "HostVsanInternalSystem:ha-vsan-internal-
// system") that referencia/govmomi/simulator/host_system.go's
// NewHostSystem never overwrites with a real registered object (unlike
// DatastoreSystem/NetworkSystem/VirtualNicManager/etc., which it does wire
// up for real). So host.ConfigManager().VsanSystem(ctx)/
// VsanInternalSystem(ctx) itself always succeeds against vcsim (it is just a
// property read of that placeholder ref), but every one of this file's 19
// raw SOAP calls against that ref reaches vcsim's generic dispatcher and
// faults server-side with "ServerFaultCode: managed object not found:
// HostVsanSystem:vsanSystem" / "...HostVsanInternalSystem:ha-vsan-internal-
// system" — proving the wiring (schema, arg decode/validation, tier
// gate, MoRef resolution, raw SOAP dispatch) reaches vcsim and gets back a
// real server-side fault, not an unknown-tool wiring bug or a recovered
// panic. generated_vsan_test.go drives every tool with a local
// assertVsanReachesServer helper for exactly this reason (same proof shape
// as generated_host_misc_test.go's assertCleanFailure, redefined locally
// here per this task's "every helper in the test file itself"
// requirement). Behavioral validation of a real vSAN cluster (disk
// groups, DOM object policies, node evacuation) is expected against a real
// vSAN-enabled ESXi host.
func registerVsanTools(r *Registry) {
	hostArg := map[string]interface{}{
		"type":        "string",
		"description": `Host identifier: a name/pattern (e.g. "esxi-01.local") or a full inventory path (e.g. "/ha-datacenter/host/esxi-01.local/esxi-01.local") as returned by vmware_list_hosts. Must resolve to exactly one host.`,
	}
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}
	disksArg := map[string]interface{}{
		"type":        "array",
		"items":       map[string]interface{}{"type": "object"},
		"description": `Array of types.HostScsiDisk JSON objects (the raw disk device — see the ScsiLun/HostScsiDisk fields, e.g. as returned by vmware_host_storage_query_available_disks_for_vsan / vmware_host_storage_query_available_disks_for_vmfs), matching the Go struct fields exactly.`,
	}
	mappingsArg := map[string]interface{}{
		"type":        "array",
		"items":       map[string]interface{}{"type": "object"},
		"description": `Array of types.VsanHostDiskMapping JSON objects ({"ssd": <HostScsiDisk>, "nonSsd": [<HostScsiDisk>, ...]}) matching the Go struct fields exactly — one entry per disk group.`,
	}
	maintenanceSpecArg := map[string]interface{}{
		"type":        "object",
		"description": `Optional types.HostMaintenanceSpec JSON object ({"vsanMode": {"objectAction": "..."}, "purpose": "..."}) controlling how vSAN moves data out of the disk(s)/node before the operation. Omit for the server default (ensureObjectAccessibility).`,
	}
	timeoutArg := map[string]interface{}{
		"type":        "integer",
		"description": "Time to wait for the task to complete, in seconds. Omit or 0 for no timeout.",
	}
	uuidsOptionalArg := map[string]interface{}{
		"type":        "array",
		"items":       map[string]interface{}{"type": "string"},
		"description": "vSAN DOM object UUIDs to restrict the query to. Omit/empty for no filter (server-defined semantics — typically all objects).",
	}

	// --- HostVsanSystem: reads ----------------------------------------------

	r.register("vmware_host_vsan_query_host_status",
		"Get an ESXi host's vSAN cluster membership/health status (node UUID, state, membership list) from HostVsanSystem.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"host": hostArg},
			"required":   []interface{}{"host"},
		},
		Tool{Handler: handleHostVsanQueryHostStatus},
	)

	r.register("vmware_host_vsan_query_disks_for_vsan",
		"List the disks on an ESXi host and how each one is currently used (or eligible to be used) by vSAN, optionally restricted to specific disk canonical names.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host": hostArg,
				"canonical_names": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": `Restrict the query to these HostScsiDisk canonical names (e.g. "naa.xxxx"). Omit for every disk on the host.`,
				},
			},
			"required": []interface{}{"host"},
		},
		Tool{Handler: handleHostVsanQueryDisksForVsan},
	)

	// --- HostVsanSystem: Tier 2 (disruptive, reversible) --------------------

	r.registerDestructive("vmware_host_vsan_add_disks",
		"Add one or more disks to an ESXi host's vSAN service (single-tier / non-disk-group usage). Reversible via vmware_host_vsan_remove_disk.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":    hostArg,
				"disks":   disksArg,
				"confirm": confirmArg,
			},
			"required": []interface{}{"host", "disks", "confirm"},
		},
		Tool{Handler: handleHostVsanAddDisks},
	)

	r.registerDestructive("vmware_host_vsan_initialize_disks",
		"Initialize one or more disk mappings (SSD + non-SSD backing disks) on an ESXi host for vSAN disk-group usage — WARNING: formats the target disks, destroying any prior data on them (same class of operation as vmware_host_datastore_create_vmfs). Reversible in the sense that the disk group can be removed via vmware_host_vsan_remove_disk_mapping and re-initialized.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":     hostArg,
				"mappings": mappingsArg,
				"confirm":  confirmArg,
			},
			"required": []interface{}{"host", "mappings", "confirm"},
		},
		Tool{Handler: handleHostVsanInitializeDisks},
	)

	r.registerDestructive("vmware_host_vsan_remove_disk",
		"Remove one or more disks from use by an ESXi host's vSAN service. Reversible via vmware_host_vsan_add_disks.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":             hostArg,
				"disks":            disksArg,
				"maintenance_spec": maintenanceSpecArg,
				"timeout":          timeoutArg,
				"confirm":          confirmArg,
			},
			"required": []interface{}{"host", "disks", "confirm"},
		},
		Tool{Handler: handleHostVsanRemoveDisk},
	)

	r.registerDestructive("vmware_host_vsan_remove_disk_mapping",
		"Remove one or more disk mappings (disk groups) from an ESXi host's vSAN service. Reversible via vmware_host_vsan_initialize_disks.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":             hostArg,
				"mappings":         mappingsArg,
				"maintenance_spec": maintenanceSpecArg,
				"timeout":          timeoutArg,
				"confirm":          confirmArg,
			},
			"required": []interface{}{"host", "mappings", "confirm"},
		},
		Tool{Handler: handleHostVsanRemoveDiskMapping},
	)

	r.registerDestructive("vmware_host_vsan_unmount_disk_mapping",
		"Unmount (without removing) one or more disk mappings on an ESXi host's vSAN service. Reversible by re-mounting/re-initializing.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":     hostArg,
				"mappings": mappingsArg,
				"confirm":  confirmArg,
			},
			"required": []interface{}{"host", "mappings", "confirm"},
		},
		Tool{Handler: handleHostVsanUnmountDiskMapping},
	)

	r.registerDestructive("vmware_host_vsan_evacuate_node",
		"Evacuate vSAN data from an ESXi host in preparation for maintenance/decommission, moving data off the host per the given maintenance spec. Reversible via vmware_host_vsan_recommission_node.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":             hostArg,
				"maintenance_spec": maintenanceSpecArg,
				"timeout":          timeoutArg,
				"confirm":          confirmArg,
			},
			"required": []interface{}{"host", "confirm"},
		},
		Tool{Handler: handleHostVsanEvacuateNode},
	)

	r.registerDestructive("vmware_host_vsan_recommission_node",
		"Recommission a previously evacuated ESXi host back into the vSAN cluster. Reverses vmware_host_vsan_evacuate_node.",
		tier2,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"host": hostArg, "confirm": confirmArg},
			"required":   []interface{}{"host", "confirm"},
		},
		Tool{Handler: handleHostVsanRecommissionNode},
	)

	// --- HostVsanInternalSystem: reads (internal/troubleshooting API) ------

	r.register("vmware_host_vsan_internal_query_objects",
		"Get vSAN DOM object layout/configuration (as a JSON-formatted string) for the given object UUIDs on an ESXi host. Internal/troubleshooting API.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":  hostArg,
				"uuids": uuidsOptionalArg,
			},
			"required": []interface{}{"host"},
		},
		Tool{Handler: handleHostVsanInternalQueryObjects},
	)

	r.register("vmware_host_vsan_internal_query_objects_on_physical_disk",
		"List vSAN DOM objects present on the given physical vSAN disk UUIDs on an ESXi host (as a JSON-formatted string). Internal/troubleshooting API.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host": hostArg,
				"disks": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "vSAN physical disk UUIDs to inspect.",
				},
			},
			"required": []interface{}{"host", "disks"},
		},
		Tool{Handler: handleHostVsanInternalQueryObjectsOnPhysicalDisk},
	)

	r.register("vmware_host_vsan_internal_query_statistics",
		"Get vSAN performance/health statistics (as a JSON-formatted string) for the given counter labels on an ESXi host. Internal/troubleshooting API.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host": hostArg,
				"labels": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": `Labels of the counters to retrieve (server-defined names, e.g. "dom", "lsom", "cmmds").`,
				},
			},
			"required": []interface{}{"host", "labels"},
		},
		Tool{Handler: handleHostVsanInternalQueryStatistics},
	)

	r.register("vmware_host_vsan_internal_query_cmmds",
		"Query the vSAN Cluster Monitoring, Membership and Directory Service (CMMDS) on an ESXi host (as a JSON-formatted string), by type/uuid/owner filters. Internal/troubleshooting API.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host": hostArg,
				"queries": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "object"},
					"description": `Array of CMMDS query specs, each a types.HostVsanInternalSystemCmmdsQuery JSON object ({"type": "DOM_OBJECT", "uuid": "...", "owner": "..."}) — every field is optional, but at least one must be set per entry to be a valid query.`,
				},
			},
			"required": []interface{}{"host", "queries"},
		},
		Tool{Handler: handleHostVsanInternalQueryCmmds},
	)

	r.register("vmware_host_vsan_internal_query_syncing_objects",
		"List vSAN DOM objects currently resyncing on an ESXi host (as a JSON-formatted string), optionally restricted to specific object UUIDs. Internal/troubleshooting API.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":  hostArg,
				"uuids": uuidsOptionalArg,
			},
			"required": []interface{}{"host"},
		},
		Tool{Handler: handleHostVsanInternalQuerySyncingObjects},
	)

	r.register("vmware_host_vsan_internal_query_physical_disks",
		"List physical vSAN disks on an ESXi host and their properties (as a JSON-formatted string), optionally restricted to specific property names. Internal/troubleshooting API.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host": hostArg,
				"props": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Property names to fetch. Omit for every property.",
				},
			},
			"required": []interface{}{"host"},
		},
		Tool{Handler: handleHostVsanInternalQueryPhysicalDisks},
	)

	// --- HostVsanInternalSystem: Tier 2 (disruptive, reversible) -----------

	r.registerDestructive("vmware_host_vsan_internal_reconfigure_dom_object",
		"Reconfigure the storage policy of a single vSAN DOM object on an ESXi host, by UUID. Internal/troubleshooting API — reversible by reconfiguring the policy again.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":    hostArg,
				"uuid":    map[string]interface{}{"type": "string", "description": "vSAN DOM object UUID."},
				"policy":  map[string]interface{}{"type": "string", "description": "vSAN expression-formatted policy string to apply."},
				"confirm": confirmArg,
			},
			"required": []interface{}{"host", "uuid", "policy", "confirm"},
		},
		Tool{Handler: handleHostVsanInternalReconfigureDomObject},
	)

	r.registerDestructive("vmware_host_vsan_internal_abdicate_dom_ownership",
		"Force a vSAN DOM object (by UUID) to give up its current owner node, so a new owner is elected. Internal/troubleshooting API — disruptive but self-healing (vSAN elects a new owner automatically).",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":    hostArg,
				"uuids":   map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "vSAN DOM object UUIDs to abdicate ownership of."},
				"confirm": confirmArg,
			},
			"required": []interface{}{"host", "uuids", "confirm"},
		},
		Tool{Handler: handleHostVsanInternalAbdicateDomOwnership},
	)

	r.registerDestructive("vmware_host_vsan_internal_run_disk_diagnostics",
		"Run vSAN physical disk diagnostics on an ESXi host, optionally restricted to specific vSAN disk UUIDs. Internal/troubleshooting API — WARNING: actively drives I/O against the disks and can be slow.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host": hostArg,
				"disks": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "vSAN physical disk UUIDs to restrict diagnostics to. Omit to run against every vSAN disk on the host.",
				},
				"confirm": confirmArg,
			},
			"required": []interface{}{"host", "confirm"},
		},
		Tool{Handler: handleHostVsanInternalRunDiskDiagnostics},
	)

	r.registerDestructive("vmware_host_vsan_internal_upgrade_objects",
		"Upgrade the on-disk format version of one or more vSAN DOM objects on an ESXi host, by UUID. Internal/troubleshooting API — does not destroy data, but object format upgrades are a one-way progression (no downgrade path).",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host": hostArg,
				"uuids": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "vSAN DOM object UUIDs to upgrade.",
				},
				"new_version": map[string]interface{}{"type": "integer", "description": "Target object format version to upgrade to."},
				"confirm":     confirmArg,
			},
			"required": []interface{}{"host", "uuids", "new_version", "confirm"},
		},
		Tool{Handler: handleHostVsanInternalUpgradeObjects},
	)
}

// --- vsan* helpers (resolve MoRefs + decode complex arguments) -------------

// vsanSystem resolves the host and its HostVsanSystem via
// host.ConfigManager().VsanSystem(ctx) — the same accessor
// generated_host_misc.go's handleHostVsanUpdate already uses. See this
// file's top doc comment for why this is preferred over re-deriving the
// MoRef by hand through a property-collector read.
func vsanSystem(ctx context.Context, client *vmware.Client, args map[string]interface{}) (*object.HostSystem, *object.HostVsanSystem, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return nil, nil, err
	}
	vs, err := host.ConfigManager().VsanSystem(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get vsan system for %s: %w", host.InventoryPath, err)
	}
	return host, vs, nil
}

// vsanInternalSystem resolves the host and its HostVsanInternalSystem via
// host.ConfigManager().VsanInternalSystem(ctx) — same accessor
// generated_host_misc.go's 3 existing vsan-internal handlers already use.
func vsanInternalSystem(ctx context.Context, client *vmware.Client, args map[string]interface{}) (*object.HostSystem, *object.HostVsanInternalSystem, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return nil, nil, err
	}
	vis, err := host.ConfigManager().VsanInternalSystem(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get vsan internal system for %s: %w", host.InventoryPath, err)
	}
	return host, vis, nil
}

// vsanRequiredDisks decodes the required "disks" argument (an array of
// types.HostScsiDisk JSON objects) into []types.HostScsiDisk, erroring if
// absent or empty — same "raw JSON array, decodeJSONArg into the real
// govmomi struct" approach as generated_vm_ft.go's "spec" argument.
func vsanRequiredDisks(args map[string]interface{}) ([]types.HostScsiDisk, error) {
	raw, ok := args["disks"]
	if !ok {
		return nil, fmt.Errorf("disks is required")
	}
	var disks []types.HostScsiDisk
	if err := decodeJSONArg(raw, &disks); err != nil {
		return nil, fmt.Errorf("invalid disks: %w", err)
	}
	if len(disks) == 0 {
		return nil, fmt.Errorf("disks must be a non-empty array")
	}
	return disks, nil
}

// vsanRequiredMappings decodes the required "mappings" argument (an array of
// types.VsanHostDiskMapping JSON objects) into []types.VsanHostDiskMapping,
// erroring if absent or empty.
func vsanRequiredMappings(args map[string]interface{}) ([]types.VsanHostDiskMapping, error) {
	raw, ok := args["mappings"]
	if !ok {
		return nil, fmt.Errorf("mappings is required")
	}
	var mappings []types.VsanHostDiskMapping
	if err := decodeJSONArg(raw, &mappings); err != nil {
		return nil, fmt.Errorf("invalid mappings: %w", err)
	}
	if len(mappings) == 0 {
		return nil, fmt.Errorf("mappings must be a non-empty array")
	}
	return mappings, nil
}

// vsanMaintenanceSpec decodes the optional "maintenance_spec" argument into
// a *types.HostMaintenanceSpec, or returns (nil, nil) when omitted — same
// nil-when-absent shape as generated_vm_ft.go's ftOptionalHost.
func vsanMaintenanceSpec(args map[string]interface{}) (*types.HostMaintenanceSpec, error) {
	raw, ok := args["maintenance_spec"]
	if !ok {
		return nil, nil
	}
	var spec types.HostMaintenanceSpec
	if err := decodeJSONArg(raw, &spec); err != nil {
		return nil, fmt.Errorf("invalid maintenance_spec: %w", err)
	}
	return &spec, nil
}

// vsanOptionalStrings reads an optional string-array argument under key,
// returning nil (not an error) when absent — used for every "uuids"/"disks"/
// "props"/"labels"-style optional filter in this file.
func vsanOptionalStrings(args map[string]interface{}, key string) ([]string, error) {
	raw, ok := args[key]
	if !ok {
		return nil, nil
	}
	vals, err := toStringSlice(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid %s: %w", key, err)
	}
	return vals, nil
}

// vsanRequiredStrings reads a required, non-empty string-array argument
// under key.
func vsanRequiredStrings(args map[string]interface{}, key string) ([]string, error) {
	raw, ok := args[key]
	if !ok {
		return nil, fmt.Errorf("%s is required", key)
	}
	vals, err := toStringSlice(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid %s: %w", key, err)
	}
	if len(vals) == 0 {
		return nil, fmt.Errorf("%s must be a non-empty array", key)
	}
	return vals, nil
}

// --- HostVsanSystem handlers -------------------------------------------

func handleHostVsanQueryHostStatus(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, vs, err := vsanSystem(ctx, client, args)
	if err != nil {
		return "", err
	}

	resp, err := methods.QueryHostStatus(ctx, client.Client.Client, &types.QueryHostStatus{This: vs.Reference()})
	if err != nil {
		return "", fmt.Errorf("failed to query vsan host status for %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "status": resp.Returnval})
}

func handleHostVsanQueryDisksForVsan(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, vs, err := vsanSystem(ctx, client, args)
	if err != nil {
		return "", err
	}
	canonicalNames, err := vsanOptionalStrings(args, "canonical_names")
	if err != nil {
		return "", err
	}

	resp, err := methods.QueryDisksForVsan(ctx, client.Client.Client, &types.QueryDisksForVsan{
		This:          vs.Reference(),
		CanonicalName: canonicalNames,
	})
	if err != nil {
		return "", fmt.Errorf("failed to query disks for vsan on %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "count": len(resp.Returnval), "disks": resp.Returnval})
}

func handleHostVsanAddDisks(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, vs, err := vsanSystem(ctx, client, args)
	if err != nil {
		return "", err
	}
	disks, err := vsanRequiredDisks(args)
	if err != nil {
		return "", err
	}

	resp, err := methods.AddDisks_Task(ctx, client.Client.Client, &types.AddDisks_Task{
		This: vs.Reference(),
		Disk: disks,
	})
	if err != nil {
		return "", fmt.Errorf("failed to add disks to vsan on %s: %w", host.InventoryPath, err)
	}
	if err := ftWaitTask(ctx, client, resp.Returnval); err != nil {
		return "", fmt.Errorf("add-vsan-disks task failed for %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "count": len(disks), "result": "disks_added"})
}

func handleHostVsanInitializeDisks(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, vs, err := vsanSystem(ctx, client, args)
	if err != nil {
		return "", err
	}
	mappings, err := vsanRequiredMappings(args)
	if err != nil {
		return "", err
	}

	resp, err := methods.InitializeDisks_Task(ctx, client.Client.Client, &types.InitializeDisks_Task{
		This:    vs.Reference(),
		Mapping: mappings,
	})
	if err != nil {
		return "", fmt.Errorf("failed to initialize vsan disks on %s: %w", host.InventoryPath, err)
	}
	if err := ftWaitTask(ctx, client, resp.Returnval); err != nil {
		return "", fmt.Errorf("initialize-vsan-disks task failed for %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "count": len(mappings), "result": "disks_initialized"})
}

func handleHostVsanRemoveDisk(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, vs, err := vsanSystem(ctx, client, args)
	if err != nil {
		return "", err
	}
	disks, err := vsanRequiredDisks(args)
	if err != nil {
		return "", err
	}
	spec, err := vsanMaintenanceSpec(args)
	if err != nil {
		return "", err
	}
	var timeout int32
	if v, ok := args["timeout"]; ok {
		timeout, err = toInt32(v)
		if err != nil {
			return "", fmt.Errorf("invalid timeout: %w", err)
		}
	}

	resp, err := methods.RemoveDisk_Task(ctx, client.Client.Client, &types.RemoveDisk_Task{
		This:            vs.Reference(),
		Disk:            disks,
		MaintenanceSpec: spec,
		Timeout:         timeout,
	})
	if err != nil {
		return "", fmt.Errorf("failed to remove vsan disks on %s: %w", host.InventoryPath, err)
	}
	if err := ftWaitTask(ctx, client, resp.Returnval); err != nil {
		return "", fmt.Errorf("remove-vsan-disk task failed for %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "count": len(disks), "result": "disks_removed"})
}

func handleHostVsanRemoveDiskMapping(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, vs, err := vsanSystem(ctx, client, args)
	if err != nil {
		return "", err
	}
	mappings, err := vsanRequiredMappings(args)
	if err != nil {
		return "", err
	}
	spec, err := vsanMaintenanceSpec(args)
	if err != nil {
		return "", err
	}
	var timeout int32
	if v, ok := args["timeout"]; ok {
		timeout, err = toInt32(v)
		if err != nil {
			return "", fmt.Errorf("invalid timeout: %w", err)
		}
	}

	resp, err := methods.RemoveDiskMapping_Task(ctx, client.Client.Client, &types.RemoveDiskMapping_Task{
		This:            vs.Reference(),
		Mapping:         mappings,
		MaintenanceSpec: spec,
		Timeout:         timeout,
	})
	if err != nil {
		return "", fmt.Errorf("failed to remove vsan disk mappings on %s: %w", host.InventoryPath, err)
	}
	if err := ftWaitTask(ctx, client, resp.Returnval); err != nil {
		return "", fmt.Errorf("remove-vsan-disk-mapping task failed for %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "count": len(mappings), "result": "disk_mappings_removed"})
}

func handleHostVsanUnmountDiskMapping(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, vs, err := vsanSystem(ctx, client, args)
	if err != nil {
		return "", err
	}
	mappings, err := vsanRequiredMappings(args)
	if err != nil {
		return "", err
	}

	resp, err := methods.UnmountDiskMapping_Task(ctx, client.Client.Client, &types.UnmountDiskMapping_Task{
		This:    vs.Reference(),
		Mapping: mappings,
	})
	if err != nil {
		return "", fmt.Errorf("failed to unmount vsan disk mappings on %s: %w", host.InventoryPath, err)
	}
	if err := ftWaitTask(ctx, client, resp.Returnval); err != nil {
		return "", fmt.Errorf("unmount-vsan-disk-mapping task failed for %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "count": len(mappings), "result": "disk_mappings_unmounted"})
}

func handleHostVsanEvacuateNode(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, vs, err := vsanSystem(ctx, client, args)
	if err != nil {
		return "", err
	}
	spec, err := vsanMaintenanceSpec(args)
	if err != nil {
		return "", err
	}
	var timeout int32
	if v, ok := args["timeout"]; ok {
		timeout, err = toInt32(v)
		if err != nil {
			return "", fmt.Errorf("invalid timeout: %w", err)
		}
	}

	req := &types.EvacuateVsanNode_Task{This: vs.Reference(), Timeout: timeout}
	if spec != nil {
		req.MaintenanceSpec = *spec
	}

	resp, err := methods.EvacuateVsanNode_Task(ctx, client.Client.Client, req)
	if err != nil {
		return "", fmt.Errorf("failed to evacuate vsan node %s: %w", host.InventoryPath, err)
	}
	if err := ftWaitTask(ctx, client, resp.Returnval); err != nil {
		return "", fmt.Errorf("evacuate-vsan-node task failed for %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "result": "node_evacuated"})
}

func handleHostVsanRecommissionNode(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, vs, err := vsanSystem(ctx, client, args)
	if err != nil {
		return "", err
	}

	resp, err := methods.RecommissionVsanNode_Task(ctx, client.Client.Client, &types.RecommissionVsanNode_Task{This: vs.Reference()})
	if err != nil {
		return "", fmt.Errorf("failed to recommission vsan node %s: %w", host.InventoryPath, err)
	}
	if err := ftWaitTask(ctx, client, resp.Returnval); err != nil {
		return "", fmt.Errorf("recommission-vsan-node task failed for %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "result": "node_recommissioned"})
}

// --- HostVsanInternalSystem handlers -------------------------------------

func handleHostVsanInternalQueryObjects(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, vis, err := vsanInternalSystem(ctx, client, args)
	if err != nil {
		return "", err
	}
	uuids, err := vsanOptionalStrings(args, "uuids")
	if err != nil {
		return "", err
	}

	resp, err := methods.QueryVsanObjects(ctx, client.Client.Client, &types.QueryVsanObjects{
		This:  vis.Reference(),
		Uuids: uuids,
	})
	if err != nil {
		return "", fmt.Errorf("failed to query vsan objects on %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "result": resp.Returnval})
}

func handleHostVsanInternalQueryObjectsOnPhysicalDisk(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, vis, err := vsanInternalSystem(ctx, client, args)
	if err != nil {
		return "", err
	}
	disks, err := vsanRequiredStrings(args, "disks")
	if err != nil {
		return "", err
	}

	resp, err := methods.QueryObjectsOnPhysicalVsanDisk(ctx, client.Client.Client, &types.QueryObjectsOnPhysicalVsanDisk{
		This:  vis.Reference(),
		Disks: disks,
	})
	if err != nil {
		return "", fmt.Errorf("failed to query objects on physical vsan disk(s) on %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "result": resp.Returnval})
}

func handleHostVsanInternalQueryStatistics(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, vis, err := vsanInternalSystem(ctx, client, args)
	if err != nil {
		return "", err
	}
	labels, err := vsanRequiredStrings(args, "labels")
	if err != nil {
		return "", err
	}

	resp, err := methods.QueryVsanStatistics(ctx, client.Client.Client, &types.QueryVsanStatistics{
		This:   vis.Reference(),
		Labels: labels,
	})
	if err != nil {
		return "", fmt.Errorf("failed to query vsan statistics on %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "result": resp.Returnval})
}

func handleHostVsanInternalQueryCmmds(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, vis, err := vsanInternalSystem(ctx, client, args)
	if err != nil {
		return "", err
	}
	raw, ok := args["queries"]
	if !ok {
		return "", fmt.Errorf("queries is required")
	}
	var queries []types.HostVsanInternalSystemCmmdsQuery
	if err := decodeJSONArg(raw, &queries); err != nil {
		return "", fmt.Errorf("invalid queries: %w", err)
	}
	if len(queries) == 0 {
		return "", fmt.Errorf("queries must be a non-empty array")
	}

	resp, err := methods.QueryCmmds(ctx, client.Client.Client, &types.QueryCmmds{
		This:    vis.Reference(),
		Queries: queries,
	})
	if err != nil {
		return "", fmt.Errorf("failed to query cmmds on %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "result": resp.Returnval})
}

func handleHostVsanInternalQuerySyncingObjects(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, vis, err := vsanInternalSystem(ctx, client, args)
	if err != nil {
		return "", err
	}
	uuids, err := vsanOptionalStrings(args, "uuids")
	if err != nil {
		return "", err
	}

	resp, err := methods.QuerySyncingVsanObjects(ctx, client.Client.Client, &types.QuerySyncingVsanObjects{
		This:  vis.Reference(),
		Uuids: uuids,
	})
	if err != nil {
		return "", fmt.Errorf("failed to query syncing vsan objects on %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "result": resp.Returnval})
}

func handleHostVsanInternalQueryPhysicalDisks(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, vis, err := vsanInternalSystem(ctx, client, args)
	if err != nil {
		return "", err
	}
	props, err := vsanOptionalStrings(args, "props")
	if err != nil {
		return "", err
	}

	resp, err := methods.QueryPhysicalVsanDisks(ctx, client.Client.Client, &types.QueryPhysicalVsanDisks{
		This:  vis.Reference(),
		Props: props,
	})
	if err != nil {
		return "", fmt.Errorf("failed to query physical vsan disks on %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "result": resp.Returnval})
}

func handleHostVsanInternalReconfigureDomObject(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, vis, err := vsanInternalSystem(ctx, client, args)
	if err != nil {
		return "", err
	}
	uuid, _ := args["uuid"].(string)
	if uuid == "" {
		return "", fmt.Errorf("uuid is required")
	}
	policy, _ := args["policy"].(string)
	if policy == "" {
		return "", fmt.Errorf("policy is required")
	}

	if _, err := methods.ReconfigureDomObject(ctx, client.Client.Client, &types.ReconfigureDomObject{
		This:   vis.Reference(),
		Uuid:   uuid,
		Policy: policy,
	}); err != nil {
		return "", fmt.Errorf("failed to reconfigure vsan dom object %s on %s: %w", uuid, host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "uuid": uuid, "policy": policy, "result": "dom_object_reconfigured"})
}

func handleHostVsanInternalAbdicateDomOwnership(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, vis, err := vsanInternalSystem(ctx, client, args)
	if err != nil {
		return "", err
	}
	uuids, err := vsanRequiredStrings(args, "uuids")
	if err != nil {
		return "", err
	}

	resp, err := methods.AbdicateDomOwnership(ctx, client.Client.Client, &types.AbdicateDomOwnership{
		This:  vis.Reference(),
		Uuids: uuids,
	})
	if err != nil {
		return "", fmt.Errorf("failed to abdicate vsan dom ownership on %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "count": len(resp.Returnval), "abdicated": resp.Returnval})
}

func handleHostVsanInternalRunDiskDiagnostics(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, vis, err := vsanInternalSystem(ctx, client, args)
	if err != nil {
		return "", err
	}
	disks, err := vsanOptionalStrings(args, "disks")
	if err != nil {
		return "", err
	}

	resp, err := methods.RunVsanPhysicalDiskDiagnostics(ctx, client.Client.Client, &types.RunVsanPhysicalDiskDiagnostics{
		This:  vis.Reference(),
		Disks: disks,
	})
	if err != nil {
		return "", fmt.Errorf("failed to run vsan physical disk diagnostics on %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "count": len(resp.Returnval), "results": resp.Returnval})
}

func handleHostVsanInternalUpgradeObjects(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, vis, err := vsanInternalSystem(ctx, client, args)
	if err != nil {
		return "", err
	}
	uuids, err := vsanRequiredStrings(args, "uuids")
	if err != nil {
		return "", err
	}
	v, ok := args["new_version"]
	if !ok {
		return "", fmt.Errorf("new_version is required")
	}
	newVersion, err := toInt32(v)
	if err != nil {
		return "", fmt.Errorf("invalid new_version: %w", err)
	}

	resp, err := methods.UpgradeVsanObjects(ctx, client.Client.Client, &types.UpgradeVsanObjects{
		This:       vis.Reference(),
		Uuids:      uuids,
		NewVersion: newVersion,
	})
	if err != nil {
		return "", fmt.Errorf("failed to upgrade vsan objects on %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "count": len(resp.Returnval), "results": resp.Returnval})
}

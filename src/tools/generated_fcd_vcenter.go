package tools

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// registerFcdVcenterTools adds tools for VcenterVStorageObjectManager — the
// vCenter-level managed object behind First Class Disks (FCD, a.k.a.
// Improved Virtual Disks / vStorageObjects): standalone virtual disks that
// exist independently of any VM, identified by a types.ID (a UUID string)
// scoped to the datastore that backs them. This is a DIFFERENT managed
// object from VirtualDiskManager (generated_virtual_disk.go's
// vmware_virtual_disk_* tools, which operate on plain .vmdk files by
// datastore path/datacenter) — confirmed no existing tool anywhere in this
// project touches VStorageObject/VcenterVStorageObjectManager before this
// file (grepped every tools/*.go for both strings first).
//
// MoRef: client.Client.ServiceContent.VStorageObjectManager
// (*types.ManagedObjectReference, vim25/types/types.go's ServiceContent
// struct). Evidence this is genuinely vCenter-only, not just
// vCenter-flavored: referencia/govmomi/simulator/{esx,vpx}/service_content.go
// (module cache copy, github.com/vmware/govmomi@v0.55.1) populate this SAME
// field with two DIFFERENT managed-object TYPES —
//
//	esx/service_content.go: {Type: "HostVStorageObjectManager", ...}
//	vpx/service_content.go: {Type: "VcenterVStorageObjectManager", ...}
//
// — i.e. a standalone ESXi host exposes the host-level FCD manager
// (HostVStorageObjectManager, a distinct set of "Host*"-prefixed SOAP
// methods this file does NOT implement) at that same ServiceContent field,
// never VcenterVStorageObjectManager. Every method this file calls
// (CreateDisk_Task, RegisterDisk, ExtendDisk_Task, ...) is documented in
// vim25/types/types.go as `VcenterVStorageObjectManager.Xxx` specifically —
// dialing one of them against an ESX-shaped ref would target the wrong
// managed object type server-side. Class: modeVCenterOnly.
//
// No object.* wrapper exists for VcenterVStorageObjectManager (confirmed:
// no object/vstorage_object*.go anywhere under referencia/govmomi/object),
// so — same as generated_host_iscsi_portbinding.go's IscsiManager and
// generated_health_ippool.go's HealthUpdateManager/IpPoolManager — every
// handler below dials the raw vim25 SOAP method directly:
// methods.Xxx(ctx, client.Client.Client, &types.Xxx{This: fcdMgrRef, ...}).
//
// Method inventory (22 tools; every name confirmed against the module cache
// github.com/vmware/govmomi@v0.55.1's vim25/methods/methods.go +
// vim25/types/types.go — no invented names). The task brief additionally
// named "ListVStorageObjectForSpec", "AttachDisk_Task", and a bare
// "QueryChangedDiskAreas" as candidates: grepping both files for
// "ListVStorageObjectForSpec" finds NOTHING (does not exist in v0.55.1);
// "AttachDisk_Task" exists but is a `VirtualMachine` method (already the
// vmware_vm_attach_disk tool, generated_vm_lifecycle.go) — not a
// VcenterVStorageObjectManager method, so out of scope here; the real name
// for the changed-disk-areas query on VcenterVStorageObjectManager is
// `VstorageObjectVCenterQueryChangedDiskAreas` (types.go line ~100795),
// implemented below as vmware_fcd_query_changed_disk_areas. None of the 3
// were implemented under an invented name.
//
//	Task-returning (waited via fcdWaitTask, this file's helper wrapping
//	object.NewTask(...).WaitForResult — same client-side-only *object.Task
//	construction generated_vm_ft.go's ftWaitTask uses):
//	  CreateDisk_Task, ExtendDisk_Task, InflateDisk_Task,
//	  DeleteVStorageObject_Task, CloneVStorageObject_Task,
//	  RelocateVStorageObject_Task, ReconcileDatastoreInventory_Task,
//	  VStorageObjectCreateSnapshot_Task, DeleteSnapshot_Task.
//	Direct (no task; RegisterDisk returns a value synchronously, the rest
//	return an empty ack struct — success is "no error"):
//	  RegisterDisk, RenameVStorageObject, RetrieveVStorageObject,
//	  RetrieveVStorageObjectState, ListVStorageObject,
//	  SetVStorageObjectControlFlags, ClearVStorageObjectControlFlags,
//	  AttachTagToVStorageObject, DetachTagFromVStorageObject,
//	  ListVStorageObjectsAttachedToTag, ScheduleReconcileDatastoreInventory,
//	  RetrieveSnapshotInfo, VstorageObjectVCenterQueryChangedDiskAreas.
//
// Polymorphic BackingSpec, deliberately NOT exposed as a raw JSON blob:
// VslmCreateSpec.BackingSpec (used by CreateDisk_Task) and
// VslmMigrateSpec.BackingSpec (embedded into VslmCloneSpec/VslmRelocateSpec,
// used by CloneVStorageObject_Task/RelocateVStorageObject_Task) are both
// typed `BaseVslmCreateSpecBackingSpec` — a Go INTERFACE, not a concrete
// struct. This project's decodeJSONArg (generated_vm_lifecycle.go) is a
// plain json.Marshal/json.Unmarshal round trip; encoding/json cannot
// populate a non-empty interface field from a JSON object (there's no
// registered UnmarshalJSON anywhere in vim25/types — confirmed by grepping
// the whole package — unlike the xml wire encoding, which resolves
// xsi:type via the `,typeattr` xml tag). Threading a "_vimType" discriminator
// through here the way generated_alarm.go's AlarmSpec does would only be
// buying support for the OTHER BackingSpec variant,
// VslmCreateSpecRawDiskMappingBackingSpec (RDM) — and vcsim's own
// VcenterVStorageObjectManager.createObject (referencia/govmomi/simulator/
// vstorage_object_manager.go) unconditionally type-asserts
// `req.Spec.BackingSpec.(*types.VslmCreateSpecDiskFileBackingSpec)` with no
// ", ok" — passing any other concrete type would panic the simulator, not
// fault cleanly. So every tool that needs a BackingSpec here (create/clone/
// relocate) instead exposes flattened arguments (dest_datastore, dest_path,
// provisioning_type) and this file's fcdDiskFileBackingSpec helper builds
// exactly the one concrete type (*types.VslmCreateSpecDiskFileBackingSpec)
// vcsim itself supports — this also matches the overwhelmingly common real
// use case (FCDs backed by a datastore file, not a raw LUN mapping).
//
// Tier: DeleteVStorageObject_Task and DeleteSnapshot_Task are tier1
// (irreversible — the vStorageObject/snapshot data is gone; there is no
// undelete in this API). Every other mutation (create/register/extend/
// inflate/rename/clone/relocate/set|clear control flags/attach|detach tag/
// reconcile|schedule-reconcile inventory/create snapshot) is tier2
// (registerDestructive) — disruptive but each has a natural undo path
// (delete what was created, rename back, detach what was attached,
// re-extend is a no-op since ExtendDisk can only grow — same "cannot shrink
// this way" posture generated_virtual_disk.go's vmware_virtual_disk_extend
// already documents for the unrelated VirtualDiskManager API) or is a
// read-repair maintenance op with no destructive side effect of its own.
// RetrieveVStorageObject/RetrieveVStorageObjectState/ListVStorageObject/
// ListVStorageObjectsAttachedToTag/RetrieveSnapshotInfo/
// VstorageObjectVCenterQueryChangedDiskAreas are plain r.register
// (read-only).
//
// vcsim coverage (referencia/govmomi/simulator/vstorage_object_manager.go,
// module cache copy — read in full before writing this file): implements
// 13 of the 22 methods for real — ListVStorageObject, RetrieveVStorageObject,
// ReconcileDatastoreInventoryTask, RegisterDisk, CreateDiskTask,
// DeleteVStorageObjectTask, RetrieveSnapshotInfo,
// VStorageObjectCreateSnapshotTask, ExtendDiskTask, DeleteSnapshotTask,
// AttachTagToVStorageObject, DetachTagFromVStorageObject,
// ListVStorageObjectsAttachedToTag — generated_fcd_vcenter_test.go drives
// those 13 with real assertions (create -> list -> retrieve -> extend ->
// snapshot -> retrieve-snapshot-info -> delete-snapshot -> attach/detach/
// list-tag -> reconcile -> delete lifecycle) against simulator.VPX(), the
// same "RealSuccess" posture generated_health_ippool_test.go uses for
// IpPoolManager. The remaining 9 — InflateDisk_Task, RenameVStorageObject,
// RetrieveVStorageObjectState, CloneVStorageObject_Task,
// RelocateVStorageObject_Task, SetVStorageObjectControlFlags,
// ClearVStorageObjectControlFlags, ScheduleReconcileDatastoreInventory,
// VstorageObjectVCenterQueryChangedDiskAreas — have NO receiver method on
// simulator.VcenterVStorageObjectManager (confirmed by reading the entire
// file); those are driven with assertReachesServer (generated_vm_lifecycle_
// test.go), the same helper generated_vm_ft_test.go and
// generated_host_iscsi_portbinding_test.go use for their own unsimulated
// methods — proving the wiring (schema, tier gate, MoRef resolution, raw
// SOAP dispatch) reaches vcsim and returns a clean server-side fault, not an
// unknown-tool wiring bug or a recovered panic. Behavioral validation of
// those 9 is expected against a real vCenter.
func registerFcdVcenterTools(r *Registry) {
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}
	datastoreArg := map[string]interface{}{
		"type":        "string",
		"description": `Datastore name/pattern (e.g. "datastore1") as returned by vmware_list_datastores, where the virtual storage object is located. Must resolve to exactly one datastore.`,
	}
	idArg := map[string]interface{}{
		"type":        "string",
		"description": `The virtual storage object's ID (a UUID string), as returned by vmware_fcd_create_disk/vmware_fcd_list's "id" field.`,
	}

	// --- Create / register ---------------------------------------------

	r.registerDestructive("vmware_fcd_create_disk",
		"Create a new First Class Disk (FCD / vStorageObject / Improved Virtual Disk) — a standalone virtual disk not attached to any VM, backed by a file on a datastore. Reversible via vmware_fcd_delete.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"datastore":            datastoreArg,
				"name":                 map[string]interface{}{"type": "string", "description": "Descriptive name for the new virtual storage object."},
				"capacity_mb":          map[string]interface{}{"type": "integer", "description": "Size of the new disk, in MB."},
				"path":                 map[string]interface{}{"type": "string", "description": `Relative path within the datastore where the disk file should be created, e.g. "fcd/my-disk.vmdk". Optional — omit to let the server pick the default FCD location on that datastore.`},
				"provisioning_type":    map[string]interface{}{"type": "string", "description": `Disk provisioning type, e.g. "thin" (default if unset), "eagerZeroedThick", "lazyZeroedThick". See BaseConfigInfoDiskFileBackingInfoProvisioningType_enum.`},
				"keep_after_delete_vm": map[string]interface{}{"type": "boolean", "description": "Whether the disk should survive deletion of the VM(s) consuming it. Defaults to true (server default) if omitted."},
				"confirm":              confirmArg,
			},
			"required": []interface{}{"datastore", "name", "capacity_mb", "confirm"},
		},
		Tool{Handler: handleFcdCreateDisk},
	)

	r.registerDestructive("vmware_fcd_register_disk",
		"Register an existing virtual disk file already present on a datastore as a First Class Disk (vStorageObject), bringing it under FCD management without copying data. Reversible via vmware_fcd_delete (removes the FCD registration; the underlying disk file is deleted along with it, per DeleteVStorageObject semantics).",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path":    map[string]interface{}{"type": "string", "description": `URL or datastore path to the existing virtual disk file, e.g. "[datastore1] fcd/existing-disk.vmdk" or a full https URL as returned by other tools.`},
				"name":    map[string]interface{}{"type": "string", "description": "Descriptive name for the registered disk object. Optional — if unset, the name is derived from the path."},
				"confirm": confirmArg,
			},
			"required": []interface{}{"path", "confirm"},
		},
		Tool{Handler: handleFcdRegisterDisk},
	)

	// --- Resize -----------------------------------------------------------

	r.registerDestructive("vmware_fcd_extend_disk",
		"Extend (grow) an existing First Class Disk's capacity. Cannot shrink — the new capacity must be greater than or equal to the current one.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"id":              idArg,
				"datastore":       datastoreArg,
				"new_capacity_mb": map[string]interface{}{"type": "integer", "description": "New total capacity of the disk, in MB. Must be >= the disk's current capacity."},
				"confirm":         confirmArg,
			},
			"required": []interface{}{"id", "datastore", "new_capacity_mb", "confirm"},
		},
		Tool{Handler: handleFcdExtendDisk},
	)

	r.registerDestructive("vmware_fcd_inflate_disk",
		"Inflate a thin-provisioned First Class Disk into an eager-zeroed-thick disk (allocates and zeroes all backing storage up front). Not simulated by this project's test harness (vcsim has no InflateDisk_Task handler) — exercise carefully against a real vCenter first.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"id":        idArg,
				"datastore": datastoreArg,
				"confirm":   confirmArg,
			},
			"required": []interface{}{"id", "datastore", "confirm"},
		},
		Tool{Handler: handleFcdInflateDisk},
	)

	// --- Rename / delete ----------------------------------------------------

	r.registerDestructive("vmware_fcd_rename",
		"Rename a First Class Disk. Not simulated by this project's test harness (vcsim has no RenameVStorageObject handler) — exercise carefully against a real vCenter first.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"id":        idArg,
				"datastore": datastoreArg,
				"name":      map[string]interface{}{"type": "string", "description": "New name for the virtual storage object."},
				"confirm":   confirmArg,
			},
			"required": []interface{}{"id", "datastore", "name", "confirm"},
		},
		Tool{Handler: handleFcdRename},
	)

	r.registerDestructive("vmware_fcd_delete",
		"Permanently delete a First Class Disk and its backing file(s). Irreversible — fails if the disk still has active consumers (e.g. attached to a VM); detach it first.",
		tier1,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"id":        idArg,
				"datastore": datastoreArg,
				"confirm":   confirmArg,
			},
			"required": []interface{}{"id", "datastore", "confirm"},
		},
		Tool{Handler: handleFcdDelete},
	)

	// --- Read accessors -----------------------------------------------------

	r.register("vmware_fcd_get",
		"Retrieve the configuration (VStorageObject) of a First Class Disk by ID.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"id":        idArg,
				"datastore": datastoreArg,
				"disk_info_flags": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": `Which pieces of FCD info to retrieve (see vslmDiskInfoFlag_enum, e.g. "capacity", "path", "controlFlags"). Optional — omit to retrieve everything.`,
				},
			},
			"required": []interface{}{"id", "datastore"},
		},
		Tool{Handler: handleFcdGet},
	)

	r.register("vmware_fcd_get_state",
		"Retrieve the dynamic state (VStorageObjectStateInfo — e.g. whether it is 'tentative', mid-provisioning) of a First Class Disk. Not simulated by this project's test harness (vcsim has no RetrieveVStorageObjectState handler).",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"id": idArg, "datastore": datastoreArg},
			"required":   []interface{}{"id", "datastore"},
		},
		Tool{Handler: handleFcdGetState},
	)

	r.register("vmware_fcd_list",
		"List the IDs of all First Class Disks (vStorageObjects) present on a datastore.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"datastore": datastoreArg},
			"required":   []interface{}{"datastore"},
		},
		Tool{Handler: handleFcdList},
	)

	// --- Clone / relocate -----------------------------------------------

	r.registerDestructive("vmware_fcd_clone",
		"Clone a First Class Disk into a new one on a (possibly different) datastore. Not simulated by this project's test harness (vcsim has no CloneVStorageObject_Task handler) — exercise carefully against a real vCenter first. Reversible via vmware_fcd_delete on the clone.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"id":                   idArg,
				"datastore":            datastoreArg,
				"name":                 map[string]interface{}{"type": "string", "description": "Descriptive name for the cloned virtual storage object."},
				"dest_datastore":       map[string]interface{}{"type": "string", "description": "Destination datastore name/pattern for the clone's backing file. May be the same as datastore."},
				"dest_path":            map[string]interface{}{"type": "string", "description": "Relative path within dest_datastore for the clone's disk file. Optional — omit for the server default FCD location."},
				"provisioning_type":    map[string]interface{}{"type": "string", "description": `Provisioning type for the clone's backing, e.g. "thin", "eagerZeroedThick", "lazyZeroedThick". Optional.`},
				"keep_after_delete_vm": map[string]interface{}{"type": "boolean", "description": "Whether the clone should survive deletion of the VM(s) consuming it. Optional."},
				"confirm":              confirmArg,
			},
			"required": []interface{}{"id", "datastore", "name", "dest_datastore", "confirm"},
		},
		Tool{Handler: handleFcdClone},
	)

	r.registerDestructive("vmware_fcd_relocate",
		"Relocate a First Class Disk's backing to a (possibly different) datastore, in place — same ID, new location. Not simulated by this project's test harness (vcsim has no RelocateVStorageObject_Task handler) — exercise carefully against a real vCenter first.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"id":                idArg,
				"datastore":         datastoreArg,
				"dest_datastore":    map[string]interface{}{"type": "string", "description": "Destination datastore name/pattern to relocate the backing file to."},
				"dest_path":         map[string]interface{}{"type": "string", "description": "Relative path within dest_datastore for the relocated disk file. Optional — omit for the server default FCD location."},
				"provisioning_type": map[string]interface{}{"type": "string", "description": `Provisioning type for the relocated backing. Optional.`},
				"confirm":           confirmArg,
			},
			"required": []interface{}{"id", "datastore", "dest_datastore", "confirm"},
		},
		Tool{Handler: handleFcdRelocate},
	)

	// --- Control flags ------------------------------------------------------

	controlFlagsArg := map[string]interface{}{
		"type":        "array",
		"items":       map[string]interface{}{"type": "string"},
		"description": `One or more control flags (vslmVStorageObjectControlFlag_enum): "keepAfterDeleteVm", "disableRelocation", "enableChangedBlockTracking". Flags not listed are left untouched.`,
	}

	r.registerDestructive("vmware_fcd_set_control_flags",
		"Set one or more control flags on a First Class Disk. Not simulated by this project's test harness (vcsim has no SetVStorageObjectControlFlags handler) — exercise carefully against a real vCenter first. Reversible via vmware_fcd_clear_control_flags.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"id":            idArg,
				"datastore":     datastoreArg,
				"control_flags": controlFlagsArg,
				"confirm":       confirmArg,
			},
			"required": []interface{}{"id", "datastore", "control_flags", "confirm"},
		},
		Tool{Handler: handleFcdSetControlFlags},
	)

	r.registerDestructive("vmware_fcd_clear_control_flags",
		"Clear one or more control flags on a First Class Disk. Not simulated by this project's test harness (vcsim has no ClearVStorageObjectControlFlags handler) — exercise carefully against a real vCenter first. Reverses vmware_fcd_set_control_flags.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"id":            idArg,
				"datastore":     datastoreArg,
				"control_flags": controlFlagsArg,
				"confirm":       confirmArg,
			},
			"required": []interface{}{"id", "datastore", "control_flags", "confirm"},
		},
		Tool{Handler: handleFcdClearControlFlags},
	)

	// --- Tags -----------------------------------------------------------

	categoryArg := map[string]interface{}{"type": "string", "description": "The tag category name."}
	tagArg := map[string]interface{}{"type": "string", "description": "The tag name within category."}

	r.registerDestructive("vmware_fcd_attach_tag",
		"Attach a (category, tag) pair to a First Class Disk. Reversible via vmware_fcd_detach_tag.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"id":       idArg,
				"category": categoryArg,
				"tag":      tagArg,
				"confirm":  confirmArg,
			},
			"required": []interface{}{"id", "category", "tag", "confirm"},
		},
		Tool{Handler: handleFcdAttachTag},
	)

	r.registerDestructive("vmware_fcd_detach_tag",
		"Detach a (category, tag) pair from a First Class Disk. Reverses vmware_fcd_attach_tag.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"id":       idArg,
				"category": categoryArg,
				"tag":      tagArg,
				"confirm":  confirmArg,
			},
			"required": []interface{}{"id", "category", "tag", "confirm"},
		},
		Tool{Handler: handleFcdDetachTag},
	)

	r.register("vmware_fcd_list_attached_to_tag",
		"List the IDs of every First Class Disk that has the given (category, tag) pair attached.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"category": categoryArg, "tag": tagArg},
			"required":   []interface{}{"category", "tag"},
		},
		Tool{Handler: handleFcdListAttachedToTag},
	)

	// --- Datastore inventory reconciliation ----------------------------

	deepCleansingArg := map[string]interface{}{"type": "boolean", "description": "If true, also verify each object's extent files and descriptor content (slow). Defaults to false."}

	r.registerDestructive("vmware_fcd_reconcile_datastore_inventory",
		"Reconcile the First Class Disk catalog for a datastore against its actual on-disk contents, synchronously, removing entries whose backing files no longer exist.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"datastore":      datastoreArg,
				"deep_cleansing": deepCleansingArg,
				"confirm":        confirmArg,
			},
			"required": []interface{}{"datastore", "confirm"},
		},
		Tool{Handler: handleFcdReconcileDatastoreInventory},
	)

	r.registerDestructive("vmware_fcd_schedule_reconcile_datastore_inventory",
		"Schedule (asynchronously, fire-and-forget) a reconciliation of the First Class Disk catalog for a datastore. Not simulated by this project's test harness (vcsim has no ScheduleReconcileDatastoreInventory handler) — exercise carefully against a real vCenter first.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"datastore":      datastoreArg,
				"deep_cleansing": deepCleansingArg,
				"confirm":        confirmArg,
			},
			"required": []interface{}{"datastore", "confirm"},
		},
		Tool{Handler: handleFcdScheduleReconcileDatastoreInventory},
	)

	// --- Snapshots ------------------------------------------------------

	snapshotIDArg := map[string]interface{}{
		"type":        "string",
		"description": `The snapshot's ID, as returned by vmware_fcd_create_snapshot's "snapshot_id" field or vmware_fcd_get_snapshot_info's per-entry "id".`,
	}

	r.registerDestructive("vmware_fcd_create_snapshot",
		"Create a snapshot of a First Class Disk. Reversible via vmware_fcd_delete_snapshot.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"id":          idArg,
				"datastore":   datastoreArg,
				"description": map[string]interface{}{"type": "string", "description": "Short description to associate with the snapshot."},
				"confirm":     confirmArg,
			},
			"required": []interface{}{"id", "datastore", "description", "confirm"},
		},
		Tool{Handler: handleFcdCreateSnapshot},
	)

	r.registerDestructive("vmware_fcd_delete_snapshot",
		"Permanently delete a First Class Disk snapshot. Irreversible.",
		tier1,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"id":          idArg,
				"datastore":   datastoreArg,
				"snapshot_id": snapshotIDArg,
				"confirm":     confirmArg,
			},
			"required": []interface{}{"id", "datastore", "snapshot_id", "confirm"},
		},
		Tool{Handler: handleFcdDeleteSnapshot},
	)

	r.register("vmware_fcd_get_snapshot_info",
		"List the snapshots of a First Class Disk.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"id": idArg, "datastore": datastoreArg},
			"required":   []interface{}{"id", "datastore"},
		},
		Tool{Handler: handleFcdGetSnapshotInfo},
	)

	r.register("vmware_fcd_query_changed_disk_areas",
		"Query which areas of a First Class Disk changed since a given change ID, as of a given snapshot (Changed Block Tracking for FCDs). Not simulated by this project's test harness (vcsim has no VstorageObjectVCenterQueryChangedDiskAreas handler) — exercise carefully against a real vCenter first.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"id":           idArg,
				"datastore":    datastoreArg,
				"snapshot_id":  snapshotIDArg,
				"start_offset": map[string]interface{}{"type": "integer", "description": "Byte offset at which to start computing changes. Start with 0, then re-call with the offset past the end of the previously returned range until the whole disk has been covered."},
				"change_id":    map[string]interface{}{"type": "string", "description": "Identifier for a prior point in time (e.g. from a previous backup) to compute changes since."},
			},
			"required": []interface{}{"id", "datastore", "snapshot_id", "start_offset", "change_id"},
		},
		Tool{Handler: handleFcdQueryChangedDiskAreas},
	)
}

// --- Argument / MoRef helpers -----------------------------------------------

// fcdMgrRef returns the connected endpoint's VcenterVStorageObjectManager
// MoRef. Every tool here registers modeVCenterOnly, so a nil ref should
// never actually be observed through a connection mode that lets this class
// register at all — this check is defense in depth, same posture as
// generated_health_ippool.go's ippoolManagerRef/hupdManagerRef.
func fcdMgrRef(client *vmware.Client) (types.ManagedObjectReference, error) {
	ref := client.Client.ServiceContent.VStorageObjectManager
	if ref == nil {
		return types.ManagedObjectReference{}, fmt.Errorf("this vCenter/ESXi endpoint does not expose a VStorageObjectManager")
	}
	return *ref, nil
}

// fcdID reads and validates the required "id" argument.
func fcdID(args map[string]interface{}) (types.ID, error) {
	id, _ := args["id"].(string)
	if id == "" {
		return types.ID{}, fmt.Errorf("id is required")
	}
	return types.ID{Id: id}, nil
}

// fcdResolveDatastore reads the required argument named argName and resolves
// it via this project's existing resolveDatastore (datastore.go).
func fcdResolveDatastore(ctx context.Context, client *vmware.Client, args map[string]interface{}, argName string) (*object.Datastore, error) {
	name, _ := args[argName].(string)
	if name == "" {
		return nil, fmt.Errorf("%s is required", argName)
	}
	return resolveDatastore(ctx, client, name)
}

// fcdDiskFileBackingSpec builds the one concrete BackingSpec type this
// project supports (see this file's top doc comment for why: it is the
// overwhelmingly common real case AND the only one vcsim's simulator
// doesn't panic on).
func fcdDiskFileBackingSpec(ds types.ManagedObjectReference, path, provisioningType string) *types.VslmCreateSpecDiskFileBackingSpec {
	return &types.VslmCreateSpecDiskFileBackingSpec{
		VslmCreateSpecBackingSpec: types.VslmCreateSpecBackingSpec{
			Datastore: ds,
			Path:      path,
		},
		ProvisioningType: provisioningType,
	}
}

// fcdControlFlags reads and validates the required, non-empty control_flags
// argument.
func fcdControlFlags(args map[string]interface{}) ([]string, error) {
	raw, ok := args["control_flags"]
	if !ok {
		return nil, fmt.Errorf("control_flags is required")
	}
	flags, err := toStringSlice(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid control_flags: %w", err)
	}
	if len(flags) == 0 {
		return nil, fmt.Errorf("control_flags must be a non-empty array")
	}
	return flags, nil
}

// fcdCategoryAndTag reads and validates the required category/tag arguments.
func fcdCategoryAndTag(args map[string]interface{}) (string, string, error) {
	category, _ := args["category"].(string)
	if category == "" {
		return "", "", fmt.Errorf("category is required")
	}
	tag, _ := args["tag"].(string)
	if tag == "" {
		return "", "", fmt.Errorf("tag is required")
	}
	return category, tag, nil
}

// fcdWaitTask blocks until the task moref returned by one of this file's
// *_Task calls completes, wrapping the bare types.ManagedObjectReference in
// a client-side-only *object.Task (no round trip until the first real call
// against it — the same construction generated_vm_ft.go's ftWaitTask and
// generated_task.go's resolveTaskArg use for a caller-supplied task moref)
// and returning the task's final Result (types.AnyType — e.g. the created
// VStorageObject for CreateDisk_Task, or nil for tasks with no result value)
// instead of discarding it like this package's plain waitForTask (vm.go),
// since several FCD *_Task methods carry their real payload in the task
// result rather than the immediate response.
func fcdWaitTask(ctx context.Context, client *vmware.Client, ref types.ManagedObjectReference) (types.AnyType, error) {
	info, err := object.NewTask(client.Client.Client, ref).WaitForResult(ctx)
	if err != nil {
		return nil, err
	}
	return info.Result, nil
}

// --- Handlers: create / register --------------------------------------------

func handleFcdCreateDisk(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgr, err := fcdMgrRef(client)
	if err != nil {
		return "", err
	}
	ds, err := fcdResolveDatastore(ctx, client, args, "datastore")
	if err != nil {
		return "", err
	}
	name, _ := args["name"].(string)
	if name == "" {
		return "", fmt.Errorf("name is required")
	}
	rawCap, ok := args["capacity_mb"]
	if !ok {
		return "", fmt.Errorf("capacity_mb is required")
	}
	capacityMB, err := toInt64(rawCap)
	if err != nil {
		return "", fmt.Errorf("invalid capacity_mb: %w", err)
	}
	path, _ := args["path"].(string)
	provisioningType, _ := args["provisioning_type"].(string)

	spec := types.VslmCreateSpec{
		Name:         name,
		CapacityInMB: capacityMB,
		BackingSpec:  fcdDiskFileBackingSpec(ds.Reference(), path, provisioningType),
	}
	if keep, ok := args["keep_after_delete_vm"].(bool); ok {
		spec.KeepAfterDeleteVm = types.NewBool(keep)
	}

	resp, err := methods.CreateDisk_Task(ctx, client.Client.Client, &types.CreateDisk_Task{This: mgr, Spec: spec})
	if err != nil {
		return "", fmt.Errorf("failed to create FCD %q on datastore %s: %w", name, ds.InventoryPath, err)
	}
	result, err := fcdWaitTask(ctx, client, resp.Returnval)
	if err != nil {
		return "", fmt.Errorf("create-disk task failed for %q on datastore %s: %w", name, ds.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{
		"datastore":       ds.InventoryPath,
		"name":            name,
		"capacity_mb":     capacityMB,
		"vstorage_object": result,
		"result":          "disk_created",
	})
}

func handleFcdRegisterDisk(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgr, err := fcdMgrRef(client)
	if err != nil {
		return "", err
	}
	path, _ := args["path"].(string)
	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	name, _ := args["name"].(string)

	resp, err := methods.RegisterDisk(ctx, client.Client.Client, &types.RegisterDisk{This: mgr, Path: path, Name: name})
	if err != nil {
		return "", fmt.Errorf("failed to register disk %q as an FCD: %w", path, err)
	}

	return marshalJSON(map[string]interface{}{
		"path":            path,
		"name":            name,
		"vstorage_object": resp.Returnval,
		"result":          "disk_registered",
	})
}

// --- Handlers: resize -----------------------------------------------------

func handleFcdExtendDisk(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgr, err := fcdMgrRef(client)
	if err != nil {
		return "", err
	}
	id, err := fcdID(args)
	if err != nil {
		return "", err
	}
	ds, err := fcdResolveDatastore(ctx, client, args, "datastore")
	if err != nil {
		return "", err
	}
	rawCap, ok := args["new_capacity_mb"]
	if !ok {
		return "", fmt.Errorf("new_capacity_mb is required")
	}
	newCapacityMB, err := toInt64(rawCap)
	if err != nil {
		return "", fmt.Errorf("invalid new_capacity_mb: %w", err)
	}

	resp, err := methods.ExtendDisk_Task(ctx, client.Client.Client, &types.ExtendDisk_Task{
		This:            mgr,
		Id:              id,
		Datastore:       ds.Reference(),
		NewCapacityInMB: newCapacityMB,
	})
	if err != nil {
		return "", fmt.Errorf("failed to extend FCD %s on datastore %s: %w", id.Id, ds.InventoryPath, err)
	}
	if _, err := fcdWaitTask(ctx, client, resp.Returnval); err != nil {
		return "", fmt.Errorf("extend-disk task failed for FCD %s on datastore %s: %w", id.Id, ds.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"id": id.Id, "datastore": ds.InventoryPath, "new_capacity_mb": newCapacityMB, "result": "disk_extended"})
}

func handleFcdInflateDisk(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgr, err := fcdMgrRef(client)
	if err != nil {
		return "", err
	}
	id, err := fcdID(args)
	if err != nil {
		return "", err
	}
	ds, err := fcdResolveDatastore(ctx, client, args, "datastore")
	if err != nil {
		return "", err
	}

	resp, err := methods.InflateDisk_Task(ctx, client.Client.Client, &types.InflateDisk_Task{This: mgr, Id: id, Datastore: ds.Reference()})
	if err != nil {
		return "", fmt.Errorf("failed to inflate FCD %s on datastore %s: %w", id.Id, ds.InventoryPath, err)
	}
	if _, err := fcdWaitTask(ctx, client, resp.Returnval); err != nil {
		return "", fmt.Errorf("inflate-disk task failed for FCD %s on datastore %s: %w", id.Id, ds.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"id": id.Id, "datastore": ds.InventoryPath, "result": "disk_inflated"})
}

// --- Handlers: rename / delete ----------------------------------------------

func handleFcdRename(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgr, err := fcdMgrRef(client)
	if err != nil {
		return "", err
	}
	id, err := fcdID(args)
	if err != nil {
		return "", err
	}
	ds, err := fcdResolveDatastore(ctx, client, args, "datastore")
	if err != nil {
		return "", err
	}
	name, _ := args["name"].(string)
	if name == "" {
		return "", fmt.Errorf("name is required")
	}

	if _, err := methods.RenameVStorageObject(ctx, client.Client.Client, &types.RenameVStorageObject{
		This:      mgr,
		Id:        id,
		Datastore: ds.Reference(),
		Name:      name,
	}); err != nil {
		return "", fmt.Errorf("failed to rename FCD %s on datastore %s to %q: %w", id.Id, ds.InventoryPath, name, err)
	}

	return marshalJSON(map[string]interface{}{"id": id.Id, "datastore": ds.InventoryPath, "name": name, "result": "disk_renamed"})
}

func handleFcdDelete(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgr, err := fcdMgrRef(client)
	if err != nil {
		return "", err
	}
	id, err := fcdID(args)
	if err != nil {
		return "", err
	}
	ds, err := fcdResolveDatastore(ctx, client, args, "datastore")
	if err != nil {
		return "", err
	}

	resp, err := methods.DeleteVStorageObject_Task(ctx, client.Client.Client, &types.DeleteVStorageObject_Task{This: mgr, Id: id, Datastore: ds.Reference()})
	if err != nil {
		return "", fmt.Errorf("failed to delete FCD %s on datastore %s: %w", id.Id, ds.InventoryPath, err)
	}
	if _, err := fcdWaitTask(ctx, client, resp.Returnval); err != nil {
		return "", fmt.Errorf("delete-disk task failed for FCD %s on datastore %s: %w", id.Id, ds.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"id": id.Id, "datastore": ds.InventoryPath, "result": "disk_deleted"})
}

// --- Handlers: read accessors ------------------------------------------

func handleFcdGet(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgr, err := fcdMgrRef(client)
	if err != nil {
		return "", err
	}
	id, err := fcdID(args)
	if err != nil {
		return "", err
	}
	ds, err := fcdResolveDatastore(ctx, client, args, "datastore")
	if err != nil {
		return "", err
	}
	var flags []string
	if raw, ok := args["disk_info_flags"]; ok && raw != nil {
		flags, err = toStringSlice(raw)
		if err != nil {
			return "", fmt.Errorf("invalid disk_info_flags: %w", err)
		}
	}

	resp, err := methods.RetrieveVStorageObject(ctx, client.Client.Client, &types.RetrieveVStorageObject{
		This:          mgr,
		Id:            id,
		Datastore:     ds.Reference(),
		DiskInfoFlags: flags,
	})
	if err != nil {
		return "", fmt.Errorf("failed to retrieve FCD %s on datastore %s: %w", id.Id, ds.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"id": id.Id, "datastore": ds.InventoryPath, "vstorage_object": resp.Returnval})
}

func handleFcdGetState(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgr, err := fcdMgrRef(client)
	if err != nil {
		return "", err
	}
	id, err := fcdID(args)
	if err != nil {
		return "", err
	}
	ds, err := fcdResolveDatastore(ctx, client, args, "datastore")
	if err != nil {
		return "", err
	}

	resp, err := methods.RetrieveVStorageObjectState(ctx, client.Client.Client, &types.RetrieveVStorageObjectState{This: mgr, Id: id, Datastore: ds.Reference()})
	if err != nil {
		return "", fmt.Errorf("failed to retrieve state of FCD %s on datastore %s: %w", id.Id, ds.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"id": id.Id, "datastore": ds.InventoryPath, "state": resp.Returnval})
}

func handleFcdList(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgr, err := fcdMgrRef(client)
	if err != nil {
		return "", err
	}
	ds, err := fcdResolveDatastore(ctx, client, args, "datastore")
	if err != nil {
		return "", err
	}

	resp, err := methods.ListVStorageObject(ctx, client.Client.Client, &types.ListVStorageObject{This: mgr, Datastore: ds.Reference()})
	if err != nil {
		return "", fmt.Errorf("failed to list FCDs on datastore %s: %w", ds.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{
		"datastore": ds.InventoryPath,
		"count":     len(resp.Returnval),
		"ids":       resp.Returnval,
	})
}

// --- Handlers: clone / relocate ---------------------------------------------

func handleFcdClone(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgr, err := fcdMgrRef(client)
	if err != nil {
		return "", err
	}
	id, err := fcdID(args)
	if err != nil {
		return "", err
	}
	ds, err := fcdResolveDatastore(ctx, client, args, "datastore")
	if err != nil {
		return "", err
	}
	name, _ := args["name"].(string)
	if name == "" {
		return "", fmt.Errorf("name is required")
	}
	destDS, err := fcdResolveDatastore(ctx, client, args, "dest_datastore")
	if err != nil {
		return "", err
	}
	destPath, _ := args["dest_path"].(string)
	provisioningType, _ := args["provisioning_type"].(string)

	spec := types.VslmCloneSpec{
		VslmMigrateSpec: types.VslmMigrateSpec{
			BackingSpec: fcdDiskFileBackingSpec(destDS.Reference(), destPath, provisioningType),
		},
		Name: name,
	}
	if keep, ok := args["keep_after_delete_vm"].(bool); ok {
		spec.KeepAfterDeleteVm = types.NewBool(keep)
	}

	resp, err := methods.CloneVStorageObject_Task(ctx, client.Client.Client, &types.CloneVStorageObject_Task{
		This:      mgr,
		Id:        id,
		Datastore: ds.Reference(),
		Spec:      spec,
	})
	if err != nil {
		return "", fmt.Errorf("failed to clone FCD %s (datastore %s) to %q on %s: %w", id.Id, ds.InventoryPath, name, destDS.InventoryPath, err)
	}
	result, err := fcdWaitTask(ctx, client, resp.Returnval)
	if err != nil {
		return "", fmt.Errorf("clone-vstorage-object task failed for FCD %s: %w", id.Id, err)
	}

	return marshalJSON(map[string]interface{}{
		"id":              id.Id,
		"datastore":       ds.InventoryPath,
		"name":            name,
		"dest_datastore":  destDS.InventoryPath,
		"vstorage_object": result,
		"result":          "disk_cloned",
	})
}

func handleFcdRelocate(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgr, err := fcdMgrRef(client)
	if err != nil {
		return "", err
	}
	id, err := fcdID(args)
	if err != nil {
		return "", err
	}
	ds, err := fcdResolveDatastore(ctx, client, args, "datastore")
	if err != nil {
		return "", err
	}
	destDS, err := fcdResolveDatastore(ctx, client, args, "dest_datastore")
	if err != nil {
		return "", err
	}
	destPath, _ := args["dest_path"].(string)
	provisioningType, _ := args["provisioning_type"].(string)

	spec := types.VslmRelocateSpec{
		VslmMigrateSpec: types.VslmMigrateSpec{
			BackingSpec: fcdDiskFileBackingSpec(destDS.Reference(), destPath, provisioningType),
		},
	}

	resp, err := methods.RelocateVStorageObject_Task(ctx, client.Client.Client, &types.RelocateVStorageObject_Task{
		This:      mgr,
		Id:        id,
		Datastore: ds.Reference(),
		Spec:      spec,
	})
	if err != nil {
		return "", fmt.Errorf("failed to relocate FCD %s (datastore %s) to %s: %w", id.Id, ds.InventoryPath, destDS.InventoryPath, err)
	}
	if _, err := fcdWaitTask(ctx, client, resp.Returnval); err != nil {
		return "", fmt.Errorf("relocate-vstorage-object task failed for FCD %s: %w", id.Id, err)
	}

	return marshalJSON(map[string]interface{}{"id": id.Id, "datastore": ds.InventoryPath, "dest_datastore": destDS.InventoryPath, "result": "disk_relocated"})
}

// --- Handlers: control flags -----------------------------------------------

func handleFcdSetControlFlags(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgr, err := fcdMgrRef(client)
	if err != nil {
		return "", err
	}
	id, err := fcdID(args)
	if err != nil {
		return "", err
	}
	ds, err := fcdResolveDatastore(ctx, client, args, "datastore")
	if err != nil {
		return "", err
	}
	flags, err := fcdControlFlags(args)
	if err != nil {
		return "", err
	}

	if _, err := methods.SetVStorageObjectControlFlags(ctx, client.Client.Client, &types.SetVStorageObjectControlFlags{
		This:         mgr,
		Id:           id,
		Datastore:    ds.Reference(),
		ControlFlags: flags,
	}); err != nil {
		return "", fmt.Errorf("failed to set control flags on FCD %s: %w", id.Id, err)
	}

	return marshalJSON(map[string]interface{}{"id": id.Id, "datastore": ds.InventoryPath, "control_flags": flags, "result": "control_flags_set"})
}

func handleFcdClearControlFlags(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgr, err := fcdMgrRef(client)
	if err != nil {
		return "", err
	}
	id, err := fcdID(args)
	if err != nil {
		return "", err
	}
	ds, err := fcdResolveDatastore(ctx, client, args, "datastore")
	if err != nil {
		return "", err
	}
	flags, err := fcdControlFlags(args)
	if err != nil {
		return "", err
	}

	if _, err := methods.ClearVStorageObjectControlFlags(ctx, client.Client.Client, &types.ClearVStorageObjectControlFlags{
		This:         mgr,
		Id:           id,
		Datastore:    ds.Reference(),
		ControlFlags: flags,
	}); err != nil {
		return "", fmt.Errorf("failed to clear control flags on FCD %s: %w", id.Id, err)
	}

	return marshalJSON(map[string]interface{}{"id": id.Id, "datastore": ds.InventoryPath, "control_flags": flags, "result": "control_flags_cleared"})
}

// --- Handlers: tags ----------------------------------------------------

func handleFcdAttachTag(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgr, err := fcdMgrRef(client)
	if err != nil {
		return "", err
	}
	id, err := fcdID(args)
	if err != nil {
		return "", err
	}
	category, tag, err := fcdCategoryAndTag(args)
	if err != nil {
		return "", err
	}

	if _, err := methods.AttachTagToVStorageObject(ctx, client.Client.Client, &types.AttachTagToVStorageObject{
		This:     mgr,
		Id:       id,
		Category: category,
		Tag:      tag,
	}); err != nil {
		return "", fmt.Errorf("failed to attach tag %s/%s to FCD %s: %w", category, tag, id.Id, err)
	}

	return marshalJSON(map[string]interface{}{"id": id.Id, "category": category, "tag": tag, "result": "tag_attached"})
}

func handleFcdDetachTag(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgr, err := fcdMgrRef(client)
	if err != nil {
		return "", err
	}
	id, err := fcdID(args)
	if err != nil {
		return "", err
	}
	category, tag, err := fcdCategoryAndTag(args)
	if err != nil {
		return "", err
	}

	if _, err := methods.DetachTagFromVStorageObject(ctx, client.Client.Client, &types.DetachTagFromVStorageObject{
		This:     mgr,
		Id:       id,
		Category: category,
		Tag:      tag,
	}); err != nil {
		return "", fmt.Errorf("failed to detach tag %s/%s from FCD %s: %w", category, tag, id.Id, err)
	}

	return marshalJSON(map[string]interface{}{"id": id.Id, "category": category, "tag": tag, "result": "tag_detached"})
}

func handleFcdListAttachedToTag(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgr, err := fcdMgrRef(client)
	if err != nil {
		return "", err
	}
	category, tag, err := fcdCategoryAndTag(args)
	if err != nil {
		return "", err
	}

	resp, err := methods.ListVStorageObjectsAttachedToTag(ctx, client.Client.Client, &types.ListVStorageObjectsAttachedToTag{This: mgr, Category: category, Tag: tag})
	if err != nil {
		return "", fmt.Errorf("failed to list FCDs attached to tag %s/%s: %w", category, tag, err)
	}

	return marshalJSON(map[string]interface{}{
		"category": category,
		"tag":      tag,
		"count":    len(resp.Returnval),
		"ids":      resp.Returnval,
	})
}

// --- Handlers: datastore inventory reconciliation ------------------------

func handleFcdReconcileDatastoreInventory(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgr, err := fcdMgrRef(client)
	if err != nil {
		return "", err
	}
	ds, err := fcdResolveDatastore(ctx, client, args, "datastore")
	if err != nil {
		return "", err
	}
	req := &types.ReconcileDatastoreInventory_Task{This: mgr, Datastore: ds.Reference()}
	if deep, ok := args["deep_cleansing"].(bool); ok {
		req.DeepCleansing = types.NewBool(deep)
	}

	resp, err := methods.ReconcileDatastoreInventory_Task(ctx, client.Client.Client, req)
	if err != nil {
		return "", fmt.Errorf("failed to reconcile FCD inventory on datastore %s: %w", ds.InventoryPath, err)
	}
	if _, err := fcdWaitTask(ctx, client, resp.Returnval); err != nil {
		return "", fmt.Errorf("reconcile-datastore-inventory task failed for datastore %s: %w", ds.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"datastore": ds.InventoryPath, "result": "inventory_reconciled"})
}

func handleFcdScheduleReconcileDatastoreInventory(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgr, err := fcdMgrRef(client)
	if err != nil {
		return "", err
	}
	ds, err := fcdResolveDatastore(ctx, client, args, "datastore")
	if err != nil {
		return "", err
	}
	req := &types.ScheduleReconcileDatastoreInventory{This: mgr, Datastore: ds.Reference()}
	if deep, ok := args["deep_cleansing"].(bool); ok {
		req.DeepCleansing = types.NewBool(deep)
	}

	if _, err := methods.ScheduleReconcileDatastoreInventory(ctx, client.Client.Client, req); err != nil {
		return "", fmt.Errorf("failed to schedule FCD inventory reconciliation on datastore %s: %w", ds.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"datastore": ds.InventoryPath, "result": "inventory_reconcile_scheduled"})
}

// --- Handlers: snapshots -------------------------------------------------

func handleFcdCreateSnapshot(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgr, err := fcdMgrRef(client)
	if err != nil {
		return "", err
	}
	id, err := fcdID(args)
	if err != nil {
		return "", err
	}
	ds, err := fcdResolveDatastore(ctx, client, args, "datastore")
	if err != nil {
		return "", err
	}
	description, _ := args["description"].(string)
	if description == "" {
		return "", fmt.Errorf("description is required")
	}

	resp, err := methods.VStorageObjectCreateSnapshot_Task(ctx, client.Client.Client, &types.VStorageObjectCreateSnapshot_Task{
		This:        mgr,
		Id:          id,
		Datastore:   ds.Reference(),
		Description: description,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create snapshot of FCD %s: %w", id.Id, err)
	}
	result, err := fcdWaitTask(ctx, client, resp.Returnval)
	if err != nil {
		return "", fmt.Errorf("create-snapshot task failed for FCD %s: %w", id.Id, err)
	}

	return marshalJSON(map[string]interface{}{
		"id":          id.Id,
		"datastore":   ds.InventoryPath,
		"description": description,
		"snapshot_id": result,
		"result":      "snapshot_created",
	})
}

func handleFcdDeleteSnapshot(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgr, err := fcdMgrRef(client)
	if err != nil {
		return "", err
	}
	id, err := fcdID(args)
	if err != nil {
		return "", err
	}
	ds, err := fcdResolveDatastore(ctx, client, args, "datastore")
	if err != nil {
		return "", err
	}
	snapID, _ := args["snapshot_id"].(string)
	if snapID == "" {
		return "", fmt.Errorf("snapshot_id is required")
	}

	resp, err := methods.DeleteSnapshot_Task(ctx, client.Client.Client, &types.DeleteSnapshot_Task{
		This:       mgr,
		Id:         id,
		Datastore:  ds.Reference(),
		SnapshotId: types.ID{Id: snapID},
	})
	if err != nil {
		return "", fmt.Errorf("failed to delete snapshot %s of FCD %s: %w", snapID, id.Id, err)
	}
	if _, err := fcdWaitTask(ctx, client, resp.Returnval); err != nil {
		return "", fmt.Errorf("delete-snapshot task failed for FCD %s snapshot %s: %w", id.Id, snapID, err)
	}

	return marshalJSON(map[string]interface{}{"id": id.Id, "datastore": ds.InventoryPath, "snapshot_id": snapID, "result": "snapshot_deleted"})
}

func handleFcdGetSnapshotInfo(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgr, err := fcdMgrRef(client)
	if err != nil {
		return "", err
	}
	id, err := fcdID(args)
	if err != nil {
		return "", err
	}
	ds, err := fcdResolveDatastore(ctx, client, args, "datastore")
	if err != nil {
		return "", err
	}

	resp, err := methods.RetrieveSnapshotInfo(ctx, client.Client.Client, &types.RetrieveSnapshotInfo{This: mgr, Id: id, Datastore: ds.Reference()})
	if err != nil {
		return "", fmt.Errorf("failed to retrieve snapshot info for FCD %s: %w", id.Id, err)
	}

	return marshalJSON(map[string]interface{}{
		"id":        id.Id,
		"datastore": ds.InventoryPath,
		"count":     len(resp.Returnval.Snapshots),
		"snapshots": resp.Returnval.Snapshots,
	})
}

func handleFcdQueryChangedDiskAreas(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgr, err := fcdMgrRef(client)
	if err != nil {
		return "", err
	}
	id, err := fcdID(args)
	if err != nil {
		return "", err
	}
	ds, err := fcdResolveDatastore(ctx, client, args, "datastore")
	if err != nil {
		return "", err
	}
	snapID, _ := args["snapshot_id"].(string)
	if snapID == "" {
		return "", fmt.Errorf("snapshot_id is required")
	}
	rawOffset, ok := args["start_offset"]
	if !ok {
		return "", fmt.Errorf("start_offset is required")
	}
	startOffset, err := toInt64(rawOffset)
	if err != nil {
		return "", fmt.Errorf("invalid start_offset: %w", err)
	}
	changeID, _ := args["change_id"].(string)
	if changeID == "" {
		return "", fmt.Errorf("change_id is required")
	}

	resp, err := methods.VstorageObjectVCenterQueryChangedDiskAreas(ctx, client.Client.Client, &types.VstorageObjectVCenterQueryChangedDiskAreas{
		This:        mgr,
		Id:          id,
		Datastore:   ds.Reference(),
		SnapshotId:  types.ID{Id: snapID},
		StartOffset: startOffset,
		ChangeId:    changeID,
	})
	if err != nil {
		return "", fmt.Errorf("failed to query changed disk areas for FCD %s snapshot %s: %w", id.Id, snapID, err)
	}

	return marshalJSON(map[string]interface{}{
		"id":          id.Id,
		"datastore":   ds.InventoryPath,
		"snapshot_id": snapID,
		"change_info": resp.Returnval,
	})
}

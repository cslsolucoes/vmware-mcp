// Package tools — generated_fcd_host.go covers the ESXi/host-level First
// Class Disk (FCD, a.k.a. Improved Virtual Disk / vStorageObject) API:
// HostVStorageObjectManager. This is the host-scoped counterpart to
// generated_fcd_vcenter.go's VcenterVStorageObjectManager (vmware_fcd_*
// tools) — a DIFFERENT managed object with its own "Host*"-prefixed SOAP
// method names, confirmed no existing tool anywhere in this project touches
// it before this file (grepped every tools/*.go for "vmware_fcdhost"/
// "HostVStorageObjectManager" first, zero matches).
//
// MoRef — corrected from the task brief, not as given: the brief assumed
// HostVStorageObjectManager is reachable via a host's own
// HostConfigManager.vStorageObjectManager property (mirroring
// generated_host_iscsi_portbinding.go's IscsiManager pattern). That is
// FALSE for this managed object — confirmed by reading
// vim25/types/types.go's HostConfigManager struct in full: it has fields for
// StorageSystem/DatastoreSystem/IscsiManager/CryptoManager/... but NO
// VStorageObjectManager field at all. The real location, confirmed in the
// same file, is client.Client.ServiceContent.VStorageObjectManager
// (*types.ManagedObjectReference — a single value per connection, not a
// per-host property), documented right there as: "If connected to a vCenter
// Server, this is the VcenterVStorageObjectManager; If connected to an ESXi
// host, this is the HostVStorageObjectManager." Independently
// cross-confirmed by generated_fcd_vcenter.go's own top doc comment, which
// reads the exact same field for its (different-typed) sibling manager.
// Every handler below therefore resolves the manager the same way
// generated_alarm.go/generated_crypto_kmip.go/generated_fcd_vcenter.go do
// for their own ServiceContent-rooted managers: fcdhostMgrRef(client)
// (nil-guarded), not a Properties() collector call against the host. The
// "host" argument every tool still takes is for caller-facing symmetry with
// every other host-scoped tool in this project (vmware_host_iscsi_*,
// vmware_host_storage_*, ...) and to validate/echo which endpoint the call
// targets — it does NOT change which MoRef gets called, since
// ServiceContent.VStorageObjectManager is one value for the whole
// connection, and a standalone ESXi connection's host Finder folder only
// ever contains the one host you're connected to anyway (the same
// single-host structural fact generated_vm_ft.go's top doc comment already
// relies on for its own host-related reasoning).
//
// No object.* wrapper exists for HostVStorageObjectManager (confirmed: no
// object/vstorage_object*.go anywhere under referencia/govmomi/object — same
// gap generated_host_iscsi_portbinding.go and generated_fcd_vcenter.go
// document for their own managers), so every handler dials the raw vim25
// SOAP method directly: methods.Xxx(ctx, client.Client.Client,
// &types.Xxx{This: mgrRef, ...}).
//
// Method inventory (18 tools; every name confirmed against the module cache
// github.com/vmware/govmomi@v0.55.1's vim25/methods/methods.go +
// vim25/types/types.go — not invented, not assumed from the brief). The
// brief additionally named "HostListVStorageObjectForSpec" as a 19th
// candidate: grepping both files for that exact identifier finds NOTHING —
// it does not exist anywhere in this govmomi version (there is no
// list-by-spec variant on the Host-scoped manager at all, unlike some other
// vSphere APIs) — excluded rather than invented.
//
//	Task-returning (waited via fcdhostWaitTask, mirroring
//	generated_fcd_vcenter.go's fcdWaitTask — object.NewTask(...).
//	WaitForResult, returning TaskInfo.Result since HostCreateDisk_Task/
//	HostCloneVStorageObject_Task/HostVStorageObjectCreateSnapshot_Task carry
//	their real payload (the created VStorageObject / snapshot ID) there, not
//	in the immediate task-moref response):
//	  HostCreateDisk_Task, HostExtendDisk_Task, HostInflateDisk_Task,
//	  HostReconcileDatastoreInventory_Task, HostCloneVStorageObject_Task,
//	  HostRelocateVStorageObject_Task, HostDeleteVStorageObject_Task,
//	  HostVStorageObjectCreateSnapshot_Task, HostVStorageObjectDeleteSnapshot_Task.
//	Direct (no task; HostRegisterDisk/HostListVStorageObject/
//	HostRetrieveVStorageObject/HostRetrieveVStorageObjectState/
//	HostVStorageObjectRetrieveSnapshotInfo return a value synchronously, the
//	rest return an empty ack struct — success is "no error"):
//	  HostRegisterDisk, HostListVStorageObject, HostRetrieveVStorageObject,
//	  HostRetrieveVStorageObjectState, HostRenameVStorageObject,
//	  HostSetVStorageObjectControlFlags, HostClearVStorageObjectControlFlags,
//	  HostScheduleReconcileDatastoreInventory,
//	  HostVStorageObjectRetrieveSnapshotInfo.
//
// Polymorphic BackingSpec — same deliberate scope cut as
// generated_fcd_vcenter.go, for the same two reasons documented there: (1)
// VslmCreateSpec.BackingSpec / VslmMigrateSpec.BackingSpec are typed
// `BaseVslmCreateSpecBackingSpec`, a Go interface — decodeJSONArg's plain
// json.Marshal/Unmarshal round trip cannot populate a non-empty interface
// field from a JSON object (no UnmarshalJSON registered anywhere in
// vim25/types for it); (2) building a full "_vimType" discriminator router
// the way generated_alarm.go's AlarmSpec does would only buy support for the
// raw-disk-mapping (RDM) variant, an edge case neither this project's sibling
// FCD file nor real-world FCD usage needs for the common case. This file
// therefore exposes flattened scalar arguments (datastore/path/
// provisioning_type, or target_datastore/target_path/provisioning_type for
// clone/relocate) and reuses generated_fcd_vcenter.go's exact
// *types.VslmCreateSpecDiskFileBackingSpec shape (re-declared here as
// fcdhostDiskFileBackingSpec, not imported — that file's helper is
// unexported and this file must not edit it to add an export).
//
// Class: modeVSphereGeneral. Evidence, not assumption — this MIRRORS
// generated_fcd_vcenter.go's own classification evidence exactly, read from
// the same two files: referencia/govmomi/simulator/{esx,vpx}/
// service_content.go (module cache copy) populate
// ServiceContent.VStorageObjectManager with two DIFFERENT managed-object
// TYPES at that one field —
//
//	esx/service_content.go: {Type: "HostVStorageObjectManager", Value: "ha-vstorage-object-manager"}
//	vpx/service_content.go: {Type: "VcenterVStorageObjectManager", Value: "VStorageObjectManager"}
//
// — i.e. a standalone ESXi host genuinely exposes HostVStorageObjectManager
// (this file's managed object) at that field; a vCenter connection exposes
// the different VcenterVStorageObjectManager type there instead (that
// file's managed object). Every method this file calls
// (HostCreateDisk_Task, HostRegisterDisk, ...) is documented in
// vim25/types/types.go as `HostVStorageObjectManager.HostXxx` specifically —
// dialing one of them against a VPX-shaped ref would target the wrong
// managed object type server-side, so this file's tools are genuinely
// ESXi-only in the sense that matters, even though modeVSphereGeneral
// (rather than a narrower "esxi-only" class, which doesn't exist in this
// project's toolMode enum) is what registers them for both --vmware-url and
// --vcenter-url connections.
//
// Tier: HostDeleteVStorageObject_Task and
// HostVStorageObjectDeleteSnapshot_Task are tier1 (irreversible — per this
// task's own brief, "Delete = tier1" — and, for the snapshot case, matching
// this project's existing vmware_vm_snapshot_remove precedent in vm.go,
// also tier1 for the same "merges/loses that point in time, no undelete"
// reasoning). Every other mutation (create/register/extend/inflate/rename/
// clone/relocate/set|clear control flags/reconcile|schedule-reconcile
// inventory/create snapshot) is tier2 (registerDestructive) per the brief's
// "resto tier2" — each has a natural undo path (delete what was created,
// rename back, clear what was set) or is a read-repair maintenance op with
// no destructive side effect of its own. HostListVStorageObject/
// HostRetrieveVStorageObject/HostRetrieveVStorageObjectState/
// HostVStorageObjectRetrieveSnapshotInfo are plain r.register (read-only).
//
// vcsim coverage: NONE of the 18 have any server-side simulation — a whole-
// object gap, not a per-method one. Confirmed two ways: (1) simulator/
// model.go's `kinds` map (which vcsim's PropertyCollector/object-loading
// path consults) registers "VcenterVStorageObjectManager" but has NO entry
// for "HostVStorageObjectManager" at all; (2) grepping the entire
// referencia/govmomi/simulator tree for "HostVStorageObjectManager" finds
// only the one struct-literal assignment in esx/service_content.go quoted
// above — no simulator.HostVStorageObjectManager{} Go receiver for any
// Host* method is ever defined anywhere. Empirically confirmed by actually
// running generated_fcd_host_test.go's TestFcdHostTools_ReachesServer
// against simulator.ESX() (not just inferred from the grep): every one of
// the 18 calls reaches vcsim's dispatcher and comes back with the exact
// fault text `HostVStorageObjectManager:ha-vstorage-object-manager does not
// implement: HostXxx` for its own method name — vcsim's generic
// "object exists but has no matching Go method" fault, the live-observed
// evidence superseding this comment's earlier (pre-test-run) guess of a bare
// ManagedObjectNotFound — not a wiring bug, not a recovered panic.
// generated_fcd_host_test.go drives every one of the 18 tools with
// assertReachesServer (generated_vm_lifecycle_test.go, reused) for exactly
// this reason — the same posture generated_host_iscsi_portbinding_test.go
// already uses for IscsiManager's own whole-object gap on ESX(). Real
// functional (create → list → retrieve → ... → delete) validation is
// expected against a real standalone ESXi host, matching the
// "exercise carefully against a real vCenter/ESXi first" caveat this
// project already states for every other unsimulated method (e.g.
// generated_virtual_disk.go's InflateVirtualDisk).
package tools

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

func registerFcdHostTools(r *Registry) {
	hostArg := map[string]interface{}{
		"type":        "string",
		"description": `Host identifier: a name/pattern (e.g. "esxi-01.local") or a full inventory path (e.g. "/ha-datacenter/host/esxi-01.local/esxi-01.local") as returned by vmware_list_hosts. Must resolve to exactly one host. Included for symmetry with every other host-scoped tool in this project and to validate/echo which endpoint the call targets; the underlying HostVStorageObjectManager is a single manager per connection (ServiceContent.vStorageObjectManager), not a per-host property — see this file's top doc comment.`,
	}
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
		"description": `The First Class Disk's ID (a UUID string), as returned by vmware_fcdhost_create_disk/vmware_fcdhost_list's "id" field.`,
	}

	// --- Create / register --------------------------------------------------

	r.registerDestructive("vmware_fcdhost_create_disk",
		"Create a new First Class Disk (FCD / vStorageObject / Improved Virtual Disk) directly on an ESXi host — a standalone virtual disk not attached to any VM, backed by a file on a datastore. Reversible via vmware_fcdhost_delete.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":                 hostArg,
				"datastore":            datastoreArg,
				"name":                 map[string]interface{}{"type": "string", "description": "Descriptive name for the new virtual storage object."},
				"capacity_mb":          map[string]interface{}{"type": "integer", "description": "Size of the new disk, in MB."},
				"path":                 map[string]interface{}{"type": "string", "description": `Relative path within the datastore where the disk file should be created, e.g. "fcd/my-disk.vmdk". Optional — omit to let the server pick the default FCD location on that datastore.`},
				"provisioning_type":    map[string]interface{}{"type": "string", "description": `Disk provisioning type, e.g. "thin" (default if unset), "eagerZeroedThick", "lazyZeroedThick". See BaseConfigInfoDiskFileBackingInfoProvisioningType_enum.`},
				"keep_after_delete_vm": map[string]interface{}{"type": "boolean", "description": "Whether the disk should survive deletion of the VM(s) consuming it. Defaults to true (server default) if omitted."},
				"confirm":              confirmArg,
			},
			"required": []interface{}{"host", "datastore", "name", "capacity_mb", "confirm"},
		},
		Tool{Handler: handleFcdHostCreateDisk},
	)

	r.registerDestructive("vmware_fcdhost_register_disk",
		"Register an existing virtual disk file already present on a datastore as a First Class Disk (vStorageObject) on an ESXi host, bringing it under FCD management without copying data. Reversible via vmware_fcdhost_delete.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":                 hostArg,
				"path":                 map[string]interface{}{"type": "string", "description": `URL or datastore path to the existing virtual disk file, e.g. "[datastore1] fcd/existing-disk.vmdk".`},
				"name":                 map[string]interface{}{"type": "string", "description": "Descriptive name for the registered disk object. Optional — if unset, the name is derived from the path."},
				"modify_control_flags": map[string]interface{}{"type": "boolean", "description": "Whether the control flags should be reset to their default values as part of registration. Optional."},
				"confirm":              confirmArg,
			},
			"required": []interface{}{"host", "path", "confirm"},
		},
		Tool{Handler: handleFcdHostRegisterDisk},
	)

	// --- Resize --------------------------------------------------------------

	r.registerDestructive("vmware_fcdhost_extend_disk",
		"Extend (grow) an existing First Class Disk's capacity, directly on an ESXi host. Cannot shrink — the new capacity must be greater than or equal to the current one.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":            hostArg,
				"id":              idArg,
				"datastore":       datastoreArg,
				"new_capacity_mb": map[string]interface{}{"type": "integer", "description": "New total capacity of the disk, in MB. Must be >= the disk's current capacity."},
				"confirm":         confirmArg,
			},
			"required": []interface{}{"host", "id", "datastore", "new_capacity_mb", "confirm"},
		},
		Tool{Handler: handleFcdHostExtendDisk},
	)

	r.registerDestructive("vmware_fcdhost_inflate_disk",
		"Inflate a thin-provisioned First Class Disk into an eager-zeroed-thick disk on an ESXi host (allocates and zeroes all backing storage up front). Not simulated by this project's test harness — exercise carefully against a real ESXi host first.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":      hostArg,
				"id":        idArg,
				"datastore": datastoreArg,
				"confirm":   confirmArg,
			},
			"required": []interface{}{"host", "id", "datastore", "confirm"},
		},
		Tool{Handler: handleFcdHostInflateDisk},
	)

	// --- Rename / delete -------------------------------------------------------

	r.registerDestructive("vmware_fcdhost_rename",
		"Rename a First Class Disk on an ESXi host. Not simulated by this project's test harness — exercise carefully against a real ESXi host first.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":      hostArg,
				"id":        idArg,
				"datastore": datastoreArg,
				"name":      map[string]interface{}{"type": "string", "description": "New name for the virtual storage object."},
				"confirm":   confirmArg,
			},
			"required": []interface{}{"host", "id", "datastore", "name", "confirm"},
		},
		Tool{Handler: handleFcdHostRename},
	)

	r.registerDestructive("vmware_fcdhost_delete",
		"Permanently delete a First Class Disk and its backing file(s) on an ESXi host. Irreversible — fails if the disk still has active consumers (e.g. attached to a VM); detach it first.",
		tier1,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":      hostArg,
				"id":        idArg,
				"datastore": datastoreArg,
				"confirm":   confirmArg,
			},
			"required": []interface{}{"host", "id", "datastore", "confirm"},
		},
		Tool{Handler: handleFcdHostDelete},
	)

	// --- Read accessors --------------------------------------------------------

	r.register("vmware_fcdhost_get",
		"Retrieve the configuration (VStorageObject) of a First Class Disk on an ESXi host, by ID.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":      hostArg,
				"id":        idArg,
				"datastore": datastoreArg,
				"disk_info_flags": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": `Which pieces of FCD info to retrieve (see vslmDiskInfoFlag_enum, e.g. "capacity", "path", "controlFlags"). Optional — omit to retrieve everything.`,
				},
			},
			"required": []interface{}{"host", "id", "datastore"},
		},
		Tool{Handler: handleFcdHostGet},
	)

	r.register("vmware_fcdhost_get_state",
		"Retrieve the dynamic state (VStorageObjectStateInfo — e.g. whether it is 'tentative', mid-provisioning) of a First Class Disk on an ESXi host. Not simulated by this project's test harness.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"host": hostArg, "id": idArg, "datastore": datastoreArg},
			"required":   []interface{}{"host", "id", "datastore"},
		},
		Tool{Handler: handleFcdHostGetState},
	)

	r.register("vmware_fcdhost_list",
		"List the IDs of all First Class Disks (vStorageObjects) present on a datastore, as seen directly by an ESXi host.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"host": hostArg, "datastore": datastoreArg},
			"required":   []interface{}{"host", "datastore"},
		},
		Tool{Handler: handleFcdHostList},
	)

	// --- Clone / relocate --------------------------------------------------

	r.registerDestructive("vmware_fcdhost_clone",
		"Clone a First Class Disk into a new one on a (possibly different) datastore, directly on an ESXi host. Not simulated by this project's test harness — exercise carefully against a real ESXi host first. Reversible via vmware_fcdhost_delete on the clone.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":                 hostArg,
				"id":                   idArg,
				"datastore":            datastoreArg,
				"name":                 map[string]interface{}{"type": "string", "description": "Descriptive name for the cloned virtual storage object."},
				"target_datastore":     map[string]interface{}{"type": "string", "description": "Destination datastore name/pattern for the clone's backing file. May be the same as datastore."},
				"target_path":          map[string]interface{}{"type": "string", "description": "Relative path within target_datastore for the clone's disk file. Optional — omit for the server default FCD location."},
				"provisioning_type":    map[string]interface{}{"type": "string", "description": `Provisioning type for the clone's backing, e.g. "thin", "eagerZeroedThick", "lazyZeroedThick". Optional.`},
				"keep_after_delete_vm": map[string]interface{}{"type": "boolean", "description": "Whether the clone should survive deletion of the VM(s) consuming it. Optional."},
				"confirm":              confirmArg,
			},
			"required": []interface{}{"host", "id", "datastore", "name", "target_datastore", "confirm"},
		},
		Tool{Handler: handleFcdHostClone},
	)

	r.registerDestructive("vmware_fcdhost_relocate",
		"Relocate a First Class Disk's backing to a (possibly different) datastore, in place — same ID, new location, directly on an ESXi host. Not simulated by this project's test harness — exercise carefully against a real ESXi host first.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":              hostArg,
				"id":                idArg,
				"datastore":         datastoreArg,
				"target_datastore":  map[string]interface{}{"type": "string", "description": "Destination datastore name/pattern to relocate the backing file to."},
				"target_path":       map[string]interface{}{"type": "string", "description": "Relative path within target_datastore for the relocated disk file. Optional — omit for the server default FCD location."},
				"provisioning_type": map[string]interface{}{"type": "string", "description": `Provisioning type for the relocated backing. Optional.`},
				"confirm":           confirmArg,
			},
			"required": []interface{}{"host", "id", "datastore", "target_datastore", "confirm"},
		},
		Tool{Handler: handleFcdHostRelocate},
	)

	// --- Control flags -----------------------------------------------------

	controlFlagsArg := map[string]interface{}{
		"type":        "array",
		"items":       map[string]interface{}{"type": "string"},
		"description": `One or more control flags (vslmVStorageObjectControlFlag_enum): "keepAfterDeleteVm", "disableRelocation", "enableChangedBlockTracking". Flags not listed are left untouched.`,
	}

	r.registerDestructive("vmware_fcdhost_set_control_flags",
		"Set one or more control flags on a First Class Disk, directly on an ESXi host. Not simulated by this project's test harness — exercise carefully against a real ESXi host first. Reversible via vmware_fcdhost_clear_control_flags.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":          hostArg,
				"id":            idArg,
				"datastore":     datastoreArg,
				"control_flags": controlFlagsArg,
				"confirm":       confirmArg,
			},
			"required": []interface{}{"host", "id", "datastore", "control_flags", "confirm"},
		},
		Tool{Handler: handleFcdHostSetControlFlags},
	)

	r.registerDestructive("vmware_fcdhost_clear_control_flags",
		"Clear one or more control flags on a First Class Disk, directly on an ESXi host. Not simulated by this project's test harness — exercise carefully against a real ESXi host first. Reverses vmware_fcdhost_set_control_flags.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":          hostArg,
				"id":            idArg,
				"datastore":     datastoreArg,
				"control_flags": controlFlagsArg,
				"confirm":       confirmArg,
			},
			"required": []interface{}{"host", "id", "datastore", "control_flags", "confirm"},
		},
		Tool{Handler: handleFcdHostClearControlFlags},
	)

	// --- Datastore inventory reconciliation ---------------------------------

	deepCleansingArg := map[string]interface{}{"type": "boolean", "description": "If true, also verify each object's extent files and descriptor content (slow). Defaults to false."}

	r.registerDestructive("vmware_fcdhost_reconcile_datastore_inventory",
		"Reconcile the First Class Disk catalog for a datastore against its actual on-disk contents, synchronously, directly on an ESXi host. Not simulated by this project's test harness — exercise carefully against a real ESXi host first.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":           hostArg,
				"datastore":      datastoreArg,
				"deep_cleansing": deepCleansingArg,
				"confirm":        confirmArg,
			},
			"required": []interface{}{"host", "datastore", "confirm"},
		},
		Tool{Handler: handleFcdHostReconcileDatastoreInventory},
	)

	r.registerDestructive("vmware_fcdhost_schedule_reconcile_datastore_inventory",
		"Schedule (asynchronously, fire-and-forget) a reconciliation of the First Class Disk catalog for a datastore, directly on an ESXi host. Not simulated by this project's test harness — exercise carefully against a real ESXi host first.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":           hostArg,
				"datastore":      datastoreArg,
				"deep_cleansing": deepCleansingArg,
				"confirm":        confirmArg,
			},
			"required": []interface{}{"host", "datastore", "confirm"},
		},
		Tool{Handler: handleFcdHostScheduleReconcileDatastoreInventory},
	)

	// --- Snapshots -----------------------------------------------------------

	snapshotIDArg := map[string]interface{}{
		"type":        "string",
		"description": `The snapshot's ID, as returned by vmware_fcdhost_create_snapshot's "snapshot_id" field or vmware_fcdhost_get_snapshot_info's per-entry "id".`,
	}

	r.registerDestructive("vmware_fcdhost_create_snapshot",
		"Create a snapshot of a First Class Disk, directly on an ESXi host. Not simulated by this project's test harness — exercise carefully against a real ESXi host first. Reversible via vmware_fcdhost_delete_snapshot.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":        hostArg,
				"id":          idArg,
				"datastore":   datastoreArg,
				"description": map[string]interface{}{"type": "string", "description": "Short description to associate with the snapshot."},
				"confirm":     confirmArg,
			},
			"required": []interface{}{"host", "id", "datastore", "description", "confirm"},
		},
		Tool{Handler: handleFcdHostCreateSnapshot},
	)

	r.registerDestructive("vmware_fcdhost_delete_snapshot",
		"Permanently delete a First Class Disk snapshot, directly on an ESXi host. Irreversible. Not simulated by this project's test harness — exercise carefully against a real ESXi host first.",
		tier1,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":        hostArg,
				"id":          idArg,
				"datastore":   datastoreArg,
				"snapshot_id": snapshotIDArg,
				"confirm":     confirmArg,
			},
			"required": []interface{}{"host", "id", "datastore", "snapshot_id", "confirm"},
		},
		Tool{Handler: handleFcdHostDeleteSnapshot},
	)

	r.register("vmware_fcdhost_get_snapshot_info",
		"List the snapshots of a First Class Disk, directly on an ESXi host. Not simulated by this project's test harness.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"host": hostArg, "id": idArg, "datastore": datastoreArg},
			"required":   []interface{}{"host", "id", "datastore"},
		},
		Tool{Handler: handleFcdHostGetSnapshotInfo},
	)
}

// --- Argument / MoRef helpers -------------------------------------------------

// fcdhostManager resolves the "host" argument (for caller-facing symmetry —
// see this file's top doc comment) AND the connected endpoint's
// HostVStorageObjectManager MoRef (client.Client.ServiceContent.
// VStorageObjectManager — NOT a property of the resolved host; see the top
// doc comment's MoRef section for why the original host.ConfigManager-based
// approach in the task brief does not exist on this managed object). The nil
// check is defense in depth: every tool here registers modeVSphereGeneral,
// so a nil ref should never actually be observed in practice against a real
// vSphere endpoint (ESXi always populates it; only a mock/incomplete
// ServiceContent would not), same posture as generated_fcd_vcenter.go's
// fcdMgrRef.
func fcdhostManager(ctx context.Context, client *vmware.Client, args map[string]interface{}) (*object.HostSystem, types.ManagedObjectReference, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return nil, types.ManagedObjectReference{}, err
	}
	ref := client.Client.ServiceContent.VStorageObjectManager
	if ref == nil {
		return nil, types.ManagedObjectReference{}, fmt.Errorf("this connection (resolved host %s) does not expose a (Host)VStorageObjectManager", host.InventoryPath)
	}
	return host, *ref, nil
}

// fcdhostID reads and validates the required "id" argument.
func fcdhostID(args map[string]interface{}) (types.ID, error) {
	id, _ := args["id"].(string)
	if id == "" {
		return types.ID{}, fmt.Errorf("id is required")
	}
	return types.ID{Id: id}, nil
}

// fcdhostResolveDatastore reads the required argument named argName and
// resolves it via this project's existing resolveDatastore (datastore.go) —
// same helper generated_fcd_vcenter.go's fcdResolveDatastore reuses for its
// own sibling manager.
func fcdhostResolveDatastore(ctx context.Context, client *vmware.Client, args map[string]interface{}, argName string) (*object.Datastore, error) {
	name, _ := args[argName].(string)
	if name == "" {
		return nil, fmt.Errorf("%s is required", argName)
	}
	return resolveDatastore(ctx, client, name)
}

// fcdhostDiskFileBackingSpec builds the one concrete BackingSpec type this
// file supports — same scope cut as generated_fcd_vcenter.go's
// fcdDiskFileBackingSpec (see this file's top doc comment's "Polymorphic
// BackingSpec" section for why), re-declared here rather than imported since
// that helper is unexported and this file must not edit
// generated_fcd_vcenter.go to add an export.
func fcdhostDiskFileBackingSpec(ds types.ManagedObjectReference, path, provisioningType string) *types.VslmCreateSpecDiskFileBackingSpec {
	return &types.VslmCreateSpecDiskFileBackingSpec{
		VslmCreateSpecBackingSpec: types.VslmCreateSpecBackingSpec{
			Datastore: ds,
			Path:      path,
		},
		ProvisioningType: provisioningType,
	}
}

// fcdhostControlFlags reads and validates the required, non-empty
// control_flags argument.
func fcdhostControlFlags(args map[string]interface{}) ([]string, error) {
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

// fcdhostWaitTask blocks until the task moref returned by one of this file's
// *_Task calls completes, wrapping the bare types.ManagedObjectReference in
// a client-side-only *object.Task (no round trip until the first real call
// against it — the same construction generated_vm_ft.go's ftWaitTask and
// generated_fcd_vcenter.go's fcdWaitTask use) and returning the task's final
// Result (types.AnyType) instead of discarding it like this package's plain
// waitForTask (vm.go) — several Host*_Task methods here carry their real
// payload (the created/cloned VStorageObject, the new snapshot ID) in the
// task result rather than the immediate response.
func fcdhostWaitTask(ctx context.Context, client *vmware.Client, ref types.ManagedObjectReference) (types.AnyType, error) {
	info, err := object.NewTask(client.Client.Client, ref).WaitForResult(ctx)
	if err != nil {
		return nil, err
	}
	return info.Result, nil
}

// --- Handlers: create / register ----------------------------------------------

func handleFcdHostCreateDisk(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	_, mgr, err := fcdhostManager(ctx, client, args)
	if err != nil {
		return "", err
	}
	ds, err := fcdhostResolveDatastore(ctx, client, args, "datastore")
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
		BackingSpec:  fcdhostDiskFileBackingSpec(ds.Reference(), path, provisioningType),
	}
	if keep, ok := args["keep_after_delete_vm"].(bool); ok {
		spec.KeepAfterDeleteVm = types.NewBool(keep)
	}

	resp, err := methods.HostCreateDisk_Task(ctx, client.Client.Client, &types.HostCreateDisk_Task{This: mgr, Spec: spec})
	if err != nil {
		return "", fmt.Errorf("failed to create FCD %q on datastore %s: %w", name, ds.InventoryPath, err)
	}
	result, err := fcdhostWaitTask(ctx, client, resp.Returnval)
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

func handleFcdHostRegisterDisk(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, mgr, err := fcdhostManager(ctx, client, args)
	if err != nil {
		return "", err
	}
	path, _ := args["path"].(string)
	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	name, _ := args["name"].(string)

	req := &types.HostRegisterDisk{This: mgr, Path: path, Name: name}
	if modify, ok := args["modify_control_flags"].(bool); ok {
		req.ModifyControlFlags = types.NewBool(modify)
	}

	resp, err := methods.HostRegisterDisk(ctx, client.Client.Client, req)
	if err != nil {
		return "", fmt.Errorf("failed to register disk %q as an FCD on %s: %w", path, host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{
		"host":            host.InventoryPath,
		"path":            path,
		"name":            name,
		"vstorage_object": resp.Returnval,
		"result":          "disk_registered",
	})
}

// --- Handlers: resize --------------------------------------------------------

func handleFcdHostExtendDisk(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	_, mgr, err := fcdhostManager(ctx, client, args)
	if err != nil {
		return "", err
	}
	id, err := fcdhostID(args)
	if err != nil {
		return "", err
	}
	ds, err := fcdhostResolveDatastore(ctx, client, args, "datastore")
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

	resp, err := methods.HostExtendDisk_Task(ctx, client.Client.Client, &types.HostExtendDisk_Task{
		This:            mgr,
		Id:              id,
		Datastore:       ds.Reference(),
		NewCapacityInMB: newCapacityMB,
	})
	if err != nil {
		return "", fmt.Errorf("failed to extend FCD %s on datastore %s: %w", id.Id, ds.InventoryPath, err)
	}
	if _, err := fcdhostWaitTask(ctx, client, resp.Returnval); err != nil {
		return "", fmt.Errorf("extend-disk task failed for FCD %s on datastore %s: %w", id.Id, ds.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"id": id.Id, "datastore": ds.InventoryPath, "new_capacity_mb": newCapacityMB, "result": "disk_extended"})
}

func handleFcdHostInflateDisk(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	_, mgr, err := fcdhostManager(ctx, client, args)
	if err != nil {
		return "", err
	}
	id, err := fcdhostID(args)
	if err != nil {
		return "", err
	}
	ds, err := fcdhostResolveDatastore(ctx, client, args, "datastore")
	if err != nil {
		return "", err
	}

	resp, err := methods.HostInflateDisk_Task(ctx, client.Client.Client, &types.HostInflateDisk_Task{This: mgr, Id: id, Datastore: ds.Reference()})
	if err != nil {
		return "", fmt.Errorf("failed to inflate FCD %s on datastore %s: %w", id.Id, ds.InventoryPath, err)
	}
	if _, err := fcdhostWaitTask(ctx, client, resp.Returnval); err != nil {
		return "", fmt.Errorf("inflate-disk task failed for FCD %s on datastore %s: %w", id.Id, ds.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"id": id.Id, "datastore": ds.InventoryPath, "result": "disk_inflated"})
}

// --- Handlers: rename / delete -------------------------------------------------

func handleFcdHostRename(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	_, mgr, err := fcdhostManager(ctx, client, args)
	if err != nil {
		return "", err
	}
	id, err := fcdhostID(args)
	if err != nil {
		return "", err
	}
	ds, err := fcdhostResolveDatastore(ctx, client, args, "datastore")
	if err != nil {
		return "", err
	}
	name, _ := args["name"].(string)
	if name == "" {
		return "", fmt.Errorf("name is required")
	}

	if _, err := methods.HostRenameVStorageObject(ctx, client.Client.Client, &types.HostRenameVStorageObject{
		This:      mgr,
		Id:        id,
		Datastore: ds.Reference(),
		Name:      name,
	}); err != nil {
		return "", fmt.Errorf("failed to rename FCD %s on datastore %s to %q: %w", id.Id, ds.InventoryPath, name, err)
	}

	return marshalJSON(map[string]interface{}{"id": id.Id, "datastore": ds.InventoryPath, "name": name, "result": "disk_renamed"})
}

func handleFcdHostDelete(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	_, mgr, err := fcdhostManager(ctx, client, args)
	if err != nil {
		return "", err
	}
	id, err := fcdhostID(args)
	if err != nil {
		return "", err
	}
	ds, err := fcdhostResolveDatastore(ctx, client, args, "datastore")
	if err != nil {
		return "", err
	}

	resp, err := methods.HostDeleteVStorageObject_Task(ctx, client.Client.Client, &types.HostDeleteVStorageObject_Task{This: mgr, Id: id, Datastore: ds.Reference()})
	if err != nil {
		return "", fmt.Errorf("failed to delete FCD %s on datastore %s: %w", id.Id, ds.InventoryPath, err)
	}
	if _, err := fcdhostWaitTask(ctx, client, resp.Returnval); err != nil {
		return "", fmt.Errorf("delete-disk task failed for FCD %s on datastore %s: %w", id.Id, ds.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"id": id.Id, "datastore": ds.InventoryPath, "result": "disk_deleted"})
}

// --- Handlers: read accessors -------------------------------------------------

func handleFcdHostGet(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	_, mgr, err := fcdhostManager(ctx, client, args)
	if err != nil {
		return "", err
	}
	id, err := fcdhostID(args)
	if err != nil {
		return "", err
	}
	ds, err := fcdhostResolveDatastore(ctx, client, args, "datastore")
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

	resp, err := methods.HostRetrieveVStorageObject(ctx, client.Client.Client, &types.HostRetrieveVStorageObject{
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

func handleFcdHostGetState(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	_, mgr, err := fcdhostManager(ctx, client, args)
	if err != nil {
		return "", err
	}
	id, err := fcdhostID(args)
	if err != nil {
		return "", err
	}
	ds, err := fcdhostResolveDatastore(ctx, client, args, "datastore")
	if err != nil {
		return "", err
	}

	resp, err := methods.HostRetrieveVStorageObjectState(ctx, client.Client.Client, &types.HostRetrieveVStorageObjectState{This: mgr, Id: id, Datastore: ds.Reference()})
	if err != nil {
		return "", fmt.Errorf("failed to retrieve state of FCD %s on datastore %s: %w", id.Id, ds.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"id": id.Id, "datastore": ds.InventoryPath, "state": resp.Returnval})
}

func handleFcdHostList(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	_, mgr, err := fcdhostManager(ctx, client, args)
	if err != nil {
		return "", err
	}
	ds, err := fcdhostResolveDatastore(ctx, client, args, "datastore")
	if err != nil {
		return "", err
	}

	resp, err := methods.HostListVStorageObject(ctx, client.Client.Client, &types.HostListVStorageObject{This: mgr, Datastore: ds.Reference()})
	if err != nil {
		return "", fmt.Errorf("failed to list FCDs on datastore %s: %w", ds.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{
		"datastore": ds.InventoryPath,
		"count":     len(resp.Returnval),
		"ids":       resp.Returnval,
	})
}

// --- Handlers: clone / relocate ------------------------------------------------

func handleFcdHostClone(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	_, mgr, err := fcdhostManager(ctx, client, args)
	if err != nil {
		return "", err
	}
	id, err := fcdhostID(args)
	if err != nil {
		return "", err
	}
	ds, err := fcdhostResolveDatastore(ctx, client, args, "datastore")
	if err != nil {
		return "", err
	}
	name, _ := args["name"].(string)
	if name == "" {
		return "", fmt.Errorf("name is required")
	}
	targetDS, err := fcdhostResolveDatastore(ctx, client, args, "target_datastore")
	if err != nil {
		return "", err
	}
	targetPath, _ := args["target_path"].(string)
	provisioningType, _ := args["provisioning_type"].(string)

	spec := types.VslmCloneSpec{
		VslmMigrateSpec: types.VslmMigrateSpec{
			BackingSpec: fcdhostDiskFileBackingSpec(targetDS.Reference(), targetPath, provisioningType),
		},
		Name: name,
	}
	if keep, ok := args["keep_after_delete_vm"].(bool); ok {
		spec.KeepAfterDeleteVm = types.NewBool(keep)
	}

	resp, err := methods.HostCloneVStorageObject_Task(ctx, client.Client.Client, &types.HostCloneVStorageObject_Task{
		This:      mgr,
		Id:        id,
		Datastore: ds.Reference(),
		Spec:      spec,
	})
	if err != nil {
		return "", fmt.Errorf("failed to clone FCD %s (datastore %s) to %q on %s: %w", id.Id, ds.InventoryPath, name, targetDS.InventoryPath, err)
	}
	result, err := fcdhostWaitTask(ctx, client, resp.Returnval)
	if err != nil {
		return "", fmt.Errorf("clone-vstorage-object task failed for FCD %s: %w", id.Id, err)
	}

	return marshalJSON(map[string]interface{}{
		"id":               id.Id,
		"datastore":        ds.InventoryPath,
		"name":             name,
		"target_datastore": targetDS.InventoryPath,
		"vstorage_object":  result,
		"result":           "disk_cloned",
	})
}

func handleFcdHostRelocate(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	_, mgr, err := fcdhostManager(ctx, client, args)
	if err != nil {
		return "", err
	}
	id, err := fcdhostID(args)
	if err != nil {
		return "", err
	}
	ds, err := fcdhostResolveDatastore(ctx, client, args, "datastore")
	if err != nil {
		return "", err
	}
	targetDS, err := fcdhostResolveDatastore(ctx, client, args, "target_datastore")
	if err != nil {
		return "", err
	}
	targetPath, _ := args["target_path"].(string)
	provisioningType, _ := args["provisioning_type"].(string)

	spec := types.VslmRelocateSpec{
		VslmMigrateSpec: types.VslmMigrateSpec{
			BackingSpec: fcdhostDiskFileBackingSpec(targetDS.Reference(), targetPath, provisioningType),
		},
	}

	resp, err := methods.HostRelocateVStorageObject_Task(ctx, client.Client.Client, &types.HostRelocateVStorageObject_Task{
		This:      mgr,
		Id:        id,
		Datastore: ds.Reference(),
		Spec:      spec,
	})
	if err != nil {
		return "", fmt.Errorf("failed to relocate FCD %s (datastore %s) to %s: %w", id.Id, ds.InventoryPath, targetDS.InventoryPath, err)
	}
	if _, err := fcdhostWaitTask(ctx, client, resp.Returnval); err != nil {
		return "", fmt.Errorf("relocate-vstorage-object task failed for FCD %s: %w", id.Id, err)
	}

	return marshalJSON(map[string]interface{}{"id": id.Id, "datastore": ds.InventoryPath, "target_datastore": targetDS.InventoryPath, "result": "disk_relocated"})
}

// --- Handlers: control flags --------------------------------------------------

func handleFcdHostSetControlFlags(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	_, mgr, err := fcdhostManager(ctx, client, args)
	if err != nil {
		return "", err
	}
	id, err := fcdhostID(args)
	if err != nil {
		return "", err
	}
	ds, err := fcdhostResolveDatastore(ctx, client, args, "datastore")
	if err != nil {
		return "", err
	}
	flags, err := fcdhostControlFlags(args)
	if err != nil {
		return "", err
	}

	if _, err := methods.HostSetVStorageObjectControlFlags(ctx, client.Client.Client, &types.HostSetVStorageObjectControlFlags{
		This:         mgr,
		Id:           id,
		Datastore:    ds.Reference(),
		ControlFlags: flags,
	}); err != nil {
		return "", fmt.Errorf("failed to set control flags on FCD %s: %w", id.Id, err)
	}

	return marshalJSON(map[string]interface{}{"id": id.Id, "datastore": ds.InventoryPath, "control_flags": flags, "result": "control_flags_set"})
}

func handleFcdHostClearControlFlags(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	_, mgr, err := fcdhostManager(ctx, client, args)
	if err != nil {
		return "", err
	}
	id, err := fcdhostID(args)
	if err != nil {
		return "", err
	}
	ds, err := fcdhostResolveDatastore(ctx, client, args, "datastore")
	if err != nil {
		return "", err
	}
	flags, err := fcdhostControlFlags(args)
	if err != nil {
		return "", err
	}

	if _, err := methods.HostClearVStorageObjectControlFlags(ctx, client.Client.Client, &types.HostClearVStorageObjectControlFlags{
		This:         mgr,
		Id:           id,
		Datastore:    ds.Reference(),
		ControlFlags: flags,
	}); err != nil {
		return "", fmt.Errorf("failed to clear control flags on FCD %s: %w", id.Id, err)
	}

	return marshalJSON(map[string]interface{}{"id": id.Id, "datastore": ds.InventoryPath, "control_flags": flags, "result": "control_flags_cleared"})
}

// --- Handlers: datastore inventory reconciliation ------------------------------

func handleFcdHostReconcileDatastoreInventory(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	_, mgr, err := fcdhostManager(ctx, client, args)
	if err != nil {
		return "", err
	}
	ds, err := fcdhostResolveDatastore(ctx, client, args, "datastore")
	if err != nil {
		return "", err
	}
	req := &types.HostReconcileDatastoreInventory_Task{This: mgr, Datastore: ds.Reference()}
	if deep, ok := args["deep_cleansing"].(bool); ok {
		req.DeepCleansing = types.NewBool(deep)
	}

	resp, err := methods.HostReconcileDatastoreInventory_Task(ctx, client.Client.Client, req)
	if err != nil {
		return "", fmt.Errorf("failed to reconcile FCD inventory on datastore %s: %w", ds.InventoryPath, err)
	}
	if _, err := fcdhostWaitTask(ctx, client, resp.Returnval); err != nil {
		return "", fmt.Errorf("reconcile-datastore-inventory task failed for datastore %s: %w", ds.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"datastore": ds.InventoryPath, "result": "inventory_reconciled"})
}

func handleFcdHostScheduleReconcileDatastoreInventory(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	_, mgr, err := fcdhostManager(ctx, client, args)
	if err != nil {
		return "", err
	}
	ds, err := fcdhostResolveDatastore(ctx, client, args, "datastore")
	if err != nil {
		return "", err
	}
	req := &types.HostScheduleReconcileDatastoreInventory{This: mgr, Datastore: ds.Reference()}
	if deep, ok := args["deep_cleansing"].(bool); ok {
		req.DeepCleansing = types.NewBool(deep)
	}

	if _, err := methods.HostScheduleReconcileDatastoreInventory(ctx, client.Client.Client, req); err != nil {
		return "", fmt.Errorf("failed to schedule FCD inventory reconciliation on datastore %s: %w", ds.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"datastore": ds.InventoryPath, "result": "inventory_reconcile_scheduled"})
}

// --- Handlers: snapshots -------------------------------------------------------

func handleFcdHostCreateSnapshot(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	_, mgr, err := fcdhostManager(ctx, client, args)
	if err != nil {
		return "", err
	}
	id, err := fcdhostID(args)
	if err != nil {
		return "", err
	}
	ds, err := fcdhostResolveDatastore(ctx, client, args, "datastore")
	if err != nil {
		return "", err
	}
	description, _ := args["description"].(string)
	if description == "" {
		return "", fmt.Errorf("description is required")
	}

	resp, err := methods.HostVStorageObjectCreateSnapshot_Task(ctx, client.Client.Client, &types.HostVStorageObjectCreateSnapshot_Task{
		This:        mgr,
		Id:          id,
		Datastore:   ds.Reference(),
		Description: description,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create snapshot of FCD %s: %w", id.Id, err)
	}
	result, err := fcdhostWaitTask(ctx, client, resp.Returnval)
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

func handleFcdHostDeleteSnapshot(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	_, mgr, err := fcdhostManager(ctx, client, args)
	if err != nil {
		return "", err
	}
	id, err := fcdhostID(args)
	if err != nil {
		return "", err
	}
	ds, err := fcdhostResolveDatastore(ctx, client, args, "datastore")
	if err != nil {
		return "", err
	}
	snapID, _ := args["snapshot_id"].(string)
	if snapID == "" {
		return "", fmt.Errorf("snapshot_id is required")
	}

	resp, err := methods.HostVStorageObjectDeleteSnapshot_Task(ctx, client.Client.Client, &types.HostVStorageObjectDeleteSnapshot_Task{
		This:       mgr,
		Id:         id,
		Datastore:  ds.Reference(),
		SnapshotId: types.ID{Id: snapID},
	})
	if err != nil {
		return "", fmt.Errorf("failed to delete snapshot %s of FCD %s: %w", snapID, id.Id, err)
	}
	if _, err := fcdhostWaitTask(ctx, client, resp.Returnval); err != nil {
		return "", fmt.Errorf("delete-snapshot task failed for FCD %s snapshot %s: %w", id.Id, snapID, err)
	}

	return marshalJSON(map[string]interface{}{"id": id.Id, "datastore": ds.InventoryPath, "snapshot_id": snapID, "result": "snapshot_deleted"})
}

func handleFcdHostGetSnapshotInfo(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	_, mgr, err := fcdhostManager(ctx, client, args)
	if err != nil {
		return "", err
	}
	id, err := fcdhostID(args)
	if err != nil {
		return "", err
	}
	ds, err := fcdhostResolveDatastore(ctx, client, args, "datastore")
	if err != nil {
		return "", err
	}

	resp, err := methods.HostVStorageObjectRetrieveSnapshotInfo(ctx, client.Client.Client, &types.HostVStorageObjectRetrieveSnapshotInfo{This: mgr, Id: id, Datastore: ds.Reference()})
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

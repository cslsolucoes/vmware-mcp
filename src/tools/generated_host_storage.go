package tools

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// registerHostStorageTools is the "storage" slice of Fase 3 of the codegen
// plan (".workspace/plans/MCPVMWare2026-08-10-175300-plano-cobertura-completa-api-codegen.plan.md")
// — object.HostStorageSystem (13 methods) + object.HostDatastoreSystem (7
// methods), hand-transcribed from the real referencia/govmomi/object source
// following the host.go/generated_option.go/generated_vm_lifecycle.go
// conventions. Every method's signature was re-verified directly against
// referencia/govmomi/object/host_storage_system.go and
// host_datastore_system.go before transcribing (not trusted from the
// generator report) — all 20 matched the brief exactly, no signature
// corrections were needed.
//
// Curation deviations from the brief (human review required):
//
//   - vmware_host_storage_compute_disk_partition_info is registered
//     read-only (r.register, no tier), not Tier 2 as the brief suggested.
//     Confirmed by reading the real method (ComputeDiskPartitionInfo just
//     invokes the SOAP method of the same name and returns its result) and
//     the official API semantics: "Compute the partition table specification
//     for the given devices" — it PROPOSES a layout, it does not apply one.
//     UpdateDiskPartitionInfo (kept at Tier 2 below) is the method that
//     actually writes a partition table to disk. Treating a pure computation
//     as destructive would be a false positive that trains callers to expect
//     confirm:true gating on a query.
//
//   - vcsim coverage is much sparser here than the VM domain: of these 20
//     methods, only 5 have ANY server-side simulator implementation —
//     confirmed by grepping every method/request-type name (RescanAllHba,
//     RescanVmfs, RefreshStorageSystem/Refresh, CreateLocalDatastore,
//     CreateNasDatastore, plus the other 15) across the entire
//     referencia/govmomi/simulator tree, not assumed from the object-layer
//     source alone. simulator.HostStorageSystem
//     (referencia/govmomi/simulator/host_storage_system.go) only defines
//     RescanAllHba, RescanVmfs, and RefreshStorageSystem; every other
//     HostStorageSystem method (AttachScsiLun, ComputeDiskPartitionInfo,
//     MarkAsLocal, MarkAsNonLocal, MarkAsNonSsd, MarkAsSsd,
//     QueryUnresolvedVmfsVolumes, RetrieveDiskPartitionInfo,
//     UnmountVmfsVolume, UpdateDiskPartitionInfo) has no matching receiver
//     method anywhere in the simulator package, so a call against vcsim
//     always faults types.MethodNotFound (simulator/simulator.go's
//     method-dispatch fallback — same mechanism documented in
//     generated_vm_lifecycle.go's top comment for 6 of its VM methods).
//     simulator.HostDatastoreSystem
//     (referencia/govmomi/simulator/host_datastore_system.go) implements
//     CreateLocalDatastore and CreateNasDatastore only; CreateVmfsDatastore,
//     Remove, QueryAvailableDisksForVmfs, QueryVmfsDatastoreCreateOptions,
//     and ResignatureUnresolvedVmfsVolumes all fault MethodNotFound the same
//     way. Every tool below is still registered exactly as real vSphere
//     supports it — a vcsim simulation gap is not a reason to omit a tool
//     real deployments can use. generated_host_storage_test.go uses the
//     existing assertReachesServer helper (generated_vm_lifecycle_test.go,
//     same package) for the 15 gap methods: it proves the plumbing (schema,
//     tier gating, resolveHost, the storage/datastore sub-manager lookup)
//     reaches vcsim's real method dispatch and gets back a clean
//     MethodNotFound-based error, not a wiring bug (unknown tool) or a
//     recovered panic.
//
//   - Every tool that returns *object.Datastore (CreateLocalDatastore,
//     CreateNasDatastore, CreateVmfsDatastore) applies the same InventoryPath
//     fix documented in generated_vm_lifecycle.go's top comment: the
//     Datastore value comes back from HostDatastoreSystem's raw
//     NewDatastore(c, ref) constructor, never through a Finder, so its
//     .InventoryPath field is always "". find.InventoryPath(ctx,
//     client.Client.Client, ds.Reference()) is used instead to resolve the
//     real path by walking ManagedEntity.Parent. This was verified by
//     reading object.NewDatastore's call sites in host_datastore_system.go
//     (not assumed by analogy alone), and confirmed empirically by
//     TestHostStorageTools_CreateDatastores in the test file — an earlier
//     draft that returned ds.InventoryPath directly produced an empty
//     "datastore" field in the tool result.
//
//   - vmware_host_datastore_remove takes "datastore" as a plain name/pattern
//     argument and resolves it via this package's existing resolveDatastore
//     helper (datastore.go) instead of duplicating that Finder/dcScopedPath
//     logic — reusing the same resolution semantics
//     (vmware_datastore_upload_file already depends on it) rather than a
//     second, subtly different implementation.
//
//   - Tier assignments otherwise follow the brief as given: Tier 2
//     (disruptive but reversible) for AttachScsiLun, MarkAsLocal/
//     MarkAsNonLocal/MarkAsSsd/MarkAsNonSsd, Refresh, RescanAllHba,
//     RescanVmfs, UpdateDiskPartitionInfo, CreateLocalDatastore/
//     CreateNasDatastore/CreateVmfsDatastore, and
//     ResignatureUnresolvedVmfsVolumes; Tier 1 (irreversible) for
//     UnmountVmfsVolume and vmware_host_datastore_remove. Refresh/
//     RescanAllHba/RescanVmfs cause no configuration change but are kept at
//     Tier 2 anyway, consistent with vmware_vm_refresh_storage_info in
//     generated_vm_lifecycle.go — same severity-table reasoning, same
//     project convention of classifying the "refresh cached info from the
//     device" family of calls alongside genuinely disruptive ones rather
//     than carving out a no-tier exception.
//
//   - HostNasVolumeSpec, VmfsDatastoreCreateSpec, and HostDiskPartitionSpec/
//     HostDiskPartitionLayout are all accepted as generic JSON objects
//     decoded via decodeJSONArg (generated_vm_lifecycle.go, same package)
//     instead of hand-built recursive schemas — same "accept the concrete
//     struct, not a schema for the interface" approach as
//     generated_option.go's Update and generated_vm_lifecycle.go's
//     SetBootOptions/PutUsbScanCodes.
func registerHostStorageTools(r *Registry) {
	hostArg := map[string]interface{}{
		"type":        "string",
		"description": `Host identifier: a name/pattern (e.g. "esxi-01.local") or a full inventory path (e.g. "/ha-datacenter/host/esxi-01.local/esxi-01.local") as returned by vmware_list_hosts. Must resolve to exactly one host.`,
	}
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}
	devicePathArg := map[string]interface{}{
		"type":        "string",
		"description": `Device path of a SCSI disk (see HostScsiDisk.devicePath, e.g. from vmware_host_datastore_query_available_disks_for_vmfs), e.g. "/vmfs/devices/disks/naa.xxxx".`,
	}
	uuidArg := map[string]interface{}{
		"type":        "string",
		"description": "UUID of the SCSI disk/LUN (HostScsiDisk.uuid).",
	}

	// --- HostStorageSystem: read-only -----------------------------------

	r.register("vmware_host_storage_query_unresolved_vmfs_volumes",
		"List VMFS volumes on a host's visible SCSI disks whose extents cannot be automatically resolved (e.g. copies of a volume created by array-level snapshot/replication).",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"host": hostArg},
			"required":   []interface{}{"host"},
		},
		Tool{Handler: handleHostStorageQueryUnresolvedVmfsVolumes},
	)

	r.register("vmware_host_storage_retrieve_disk_partition_info",
		"Read the current partition table of a disk device on a host.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":        hostArg,
				"device_path": devicePathArg,
			},
			"required": []interface{}{"host", "device_path"},
		},
		Tool{Handler: handleHostStorageRetrieveDiskPartitionInfo},
	)

	r.register("vmware_host_storage_compute_disk_partition_info",
		"Compute (but do not apply) a proposed partition table layout for a disk device, given a target layout. Read-only: it returns the computed HostDiskPartitionInfo without writing anything to disk — use vmware_host_storage_update_disk_partition_info to actually apply a partition table.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":        hostArg,
				"device_path": devicePathArg,
				"layout":      map[string]interface{}{"type": "object", "description": "A types.HostDiskPartitionLayout JSON object matching its Go struct fields (\"total\" dimensions, \"partition\" block ranges)."},
			},
			"required": []interface{}{"host", "device_path", "layout"},
		},
		Tool{Handler: handleHostStorageComputeDiskPartitionInfo},
	)

	// --- HostStorageSystem: Tier 2 (disruptive but reversible) ----------

	r.registerDestructive("vmware_host_storage_attach_scsi_lun",
		"Attach a previously detached SCSI LUN on a host, making it visible again.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":    hostArg,
				"uuid":    uuidArg,
				"confirm": confirmArg,
			},
			"required": []interface{}{"host", "uuid", "confirm"},
		},
		Tool{Handler: handleHostStorageAttachScsiLun},
	)

	r.registerDestructive("vmware_host_storage_mark_as_local",
		"Mark a SCSI disk on a host as local storage (vs. shared/remote). Reversible via vmware_host_storage_mark_as_non_local.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":    hostArg,
				"uuid":    uuidArg,
				"confirm": confirmArg,
			},
			"required": []interface{}{"host", "uuid", "confirm"},
		},
		Tool{Handler: handleHostStorageMarkAsLocal},
	)

	r.registerDestructive("vmware_host_storage_mark_as_non_local",
		"Mark a SCSI disk on a host as NOT local storage (shared/remote). Reversible via vmware_host_storage_mark_as_local.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":    hostArg,
				"uuid":    uuidArg,
				"confirm": confirmArg,
			},
			"required": []interface{}{"host", "uuid", "confirm"},
		},
		Tool{Handler: handleHostStorageMarkAsNonLocal},
	)

	r.registerDestructive("vmware_host_storage_mark_as_ssd",
		"Mark a SCSI disk on a host as SSD-backed. Reversible via vmware_host_storage_mark_as_non_ssd.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":    hostArg,
				"uuid":    uuidArg,
				"confirm": confirmArg,
			},
			"required": []interface{}{"host", "uuid", "confirm"},
		},
		Tool{Handler: handleHostStorageMarkAsSsd},
	)

	r.registerDestructive("vmware_host_storage_mark_as_non_ssd",
		"Mark a SCSI disk on a host as NOT SSD-backed. Reversible via vmware_host_storage_mark_as_ssd.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":    hostArg,
				"uuid":    uuidArg,
				"confirm": confirmArg,
			},
			"required": []interface{}{"host", "uuid", "confirm"},
		},
		Tool{Handler: handleHostStorageMarkAsNonSsd},
	)

	r.registerDestructive("vmware_host_storage_refresh",
		"Refresh a host's cached storage system information (disks, HBAs, filesystems) from the device. No configuration change, but classified alongside the other storage-refresh actions here per the plan's Fase 1a severity table (see vmware_vm_refresh_storage_info for the same convention).",
		tier2,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"host": hostArg, "confirm": confirmArg},
			"required":   []interface{}{"host", "confirm"},
		},
		Tool{Handler: handleHostStorageRefresh},
	)

	r.registerDestructive("vmware_host_storage_rescan_all_hba",
		"Rescan all of a host's storage host bus adapters (HBAs) for new SCSI devices/LUNs. No configuration change, but classified alongside vmware_host_storage_refresh per the same convention.",
		tier2,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"host": hostArg, "confirm": confirmArg},
			"required":   []interface{}{"host", "confirm"},
		},
		Tool{Handler: handleHostStorageRescanAllHba},
	)

	r.registerDestructive("vmware_host_storage_rescan_vmfs",
		"Rescan a host's already-visible SCSI disks for VMFS volumes. No configuration change, but classified alongside vmware_host_storage_refresh per the same convention.",
		tier2,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"host": hostArg, "confirm": confirmArg},
			"required":   []interface{}{"host", "confirm"},
		},
		Tool{Handler: handleHostStorageRescanVmfs},
	)

	r.registerDestructive("vmware_host_storage_update_disk_partition_info",
		"Write a new partition table to a disk device on a host — the mutating counterpart of vmware_host_storage_compute_disk_partition_info (typically called with that tool's computed result). Reversible only in the sense that the disk can be repartitioned again; any data on the old partitions is not preserved.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":        hostArg,
				"device_path": devicePathArg,
				"spec":        map[string]interface{}{"type": "object", "description": "A types.HostDiskPartitionSpec JSON object matching its Go struct fields (\"partitionFormat\", \"partition\" list, etc.) — typically the HostDiskPartitionInfo.spec from vmware_host_storage_compute_disk_partition_info."},
				"confirm":     confirmArg,
			},
			"required": []interface{}{"host", "device_path", "spec", "confirm"},
		},
		Tool{Handler: handleHostStorageUpdateDiskPartitionInfo},
	)

	// --- HostStorageSystem: Tier 1 (irreversible) ------------------------

	r.registerDestructive("vmware_host_storage_unmount_vmfs_volume",
		"Unmount a VMFS volume from a host. Irreversible from this server's point of view (no tool here remounts a VMFS volume by UUID) — any VM registered on the volume becomes inaccessible from this host until it is remounted through other means.",
		tier1,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":      hostArg,
				"vmfs_uuid": map[string]interface{}{"type": "string", "description": "UUID of the VMFS volume to unmount (see HostVmfsVolume.uuid)."},
				"confirm":   confirmArg,
			},
			"required": []interface{}{"host", "vmfs_uuid", "confirm"},
		},
		Tool{Handler: handleHostStorageUnmountVmfsVolume},
	)

	// --- HostDatastoreSystem: read-only ----------------------------------

	r.register("vmware_host_datastore_query_available_disks_for_vmfs",
		"List SCSI disks on a host that are available (unpartitioned/unused) as candidates for creating a new VMFS datastore.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"host": hostArg},
			"required":   []interface{}{"host"},
		},
		Tool{Handler: handleHostDatastoreQueryAvailableDisksForVmfs},
	)

	r.register("vmware_host_datastore_query_vmfs_datastore_create_options",
		"List the possible VMFS-datastore-creation options (partition layouts) for a given disk device on a host — the input to vmware_host_datastore_create_vmfs's spec.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":        hostArg,
				"device_path": devicePathArg,
			},
			"required": []interface{}{"host", "device_path"},
		},
		Tool{Handler: handleHostDatastoreQueryVmfsDatastoreCreateOptions},
	)

	// --- HostDatastoreSystem: Tier 2 (disruptive but reversible) --------

	r.registerDestructive("vmware_host_datastore_create_local",
		"Create a local (VMFS-less, plain directory) datastore on a host from an existing directory path on that host's filesystem.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":    hostArg,
				"name":    map[string]interface{}{"type": "string", "description": "Name for the new datastore."},
				"path":    map[string]interface{}{"type": "string", "description": "Absolute directory path on the host's filesystem to back the datastore. Must already exist."},
				"confirm": confirmArg,
			},
			"required": []interface{}{"host", "name", "path", "confirm"},
		},
		Tool{Handler: handleHostDatastoreCreateLocal},
	)

	r.registerDestructive("vmware_host_datastore_create_nas",
		`Mount an NFS/CIFS share on a host as a NAS datastore.`,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":    hostArg,
				"spec":    map[string]interface{}{"type": "object", "description": `A types.HostNasVolumeSpec JSON object matching its Go struct fields — remoteHost, remotePath, localPath (the datastore name), accessMode ("readWrite"/"readOnly"), and type ("NFS"/"NFS41"/"CIFS"). remoteHost and remotePath are required by the server.`},
				"confirm": confirmArg,
			},
			"required": []interface{}{"host", "spec", "confirm"},
		},
		Tool{Handler: handleHostDatastoreCreateNas},
	)

	r.registerDestructive("vmware_host_datastore_create_vmfs",
		"Create a new VMFS datastore on a host from a disk device — the mutating counterpart of vmware_host_datastore_query_vmfs_datastore_create_options (typically called with one of that tool's returned VmfsDatastoreOption.spec values).",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":    hostArg,
				"spec":    map[string]interface{}{"type": "object", "description": "A types.VmfsDatastoreCreateSpec JSON object matching its Go struct fields (\"partition\", \"vmfs\", \"extent\") — typically a VmfsDatastoreOption.spec value from vmware_host_datastore_query_vmfs_datastore_create_options."},
				"confirm": confirmArg,
			},
			"required": []interface{}{"host", "spec", "confirm"},
		},
		Tool{Handler: handleHostDatastoreCreateVmfs},
	)

	r.registerDestructive("vmware_host_datastore_resignature_unresolved_vmfs_volumes",
		"Resignature (assign a new UUID/label to) unresolved VMFS volume copies detected on a host's disk devices, making them mountable as new datastores instead of conflicting with the original volume — see vmware_host_storage_query_unresolved_vmfs_volumes to find candidates.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":         hostArg,
				"device_paths": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Device paths of the unresolved VMFS extent(s) to resignature (see HostUnresolvedVmfsExtent.devicePath)."},
				"confirm":      confirmArg,
			},
			"required": []interface{}{"host", "device_paths", "confirm"},
		},
		Tool{Handler: handleHostDatastoreResignatureUnresolvedVmfsVolumes},
	)

	// --- HostDatastoreSystem: Tier 1 (irreversible) -----------------------

	r.registerDestructive("vmware_host_datastore_remove",
		"Remove (unmount and delete the registration of) a datastore from a host. Irreversible from this server's point of view — the underlying files are not deleted by this call, but no tool here re-adds an already-created datastore back to a host's inventory by path.",
		tier1,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":      hostArg,
				"datastore": map[string]interface{}{"type": "string", "description": `Datastore name/pattern (e.g. "datastore1") as returned by vmware_list_datastores. Must resolve to exactly one datastore.`},
				"confirm":   confirmArg,
			},
			"required": []interface{}{"host", "datastore", "confirm"},
		},
		Tool{Handler: handleHostDatastoreRemove},
	)
}

// hostStorageSystem resolves host's HostStorageSystem sub-manager, the same
// "get the ConfigManager sub-manager, wrap the error" pattern as
// generated_option.go's inline OptionManager lookup — extracted to a helper
// here since 13 handlers in this file need it (vs. 2 for OptionManager).
func hostStorageSystem(ctx context.Context, host *object.HostSystem) (*object.HostStorageSystem, error) {
	ss, err := host.ConfigManager().StorageSystem(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get storage system for %s: %w", host.InventoryPath, err)
	}
	return ss, nil
}

// hostDatastoreSystem resolves host's HostDatastoreSystem sub-manager — same
// reasoning as hostStorageSystem above, for the 7 datastore-system handlers.
func hostDatastoreSystem(ctx context.Context, host *object.HostSystem) (*object.HostDatastoreSystem, error) {
	ds, err := host.ConfigManager().DatastoreSystem(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get datastore system for %s: %w", host.InventoryPath, err)
	}
	return ds, nil
}

// datastoreInventoryPath resolves ds's real inventory path via find.InventoryPath
// instead of ds.InventoryPath — see this file's top doc comment for why:
// HostDatastoreSystem's Create* methods build the returned *object.Datastore
// via a bare NewDatastore(c, ref) constructor, never through a Finder, so
// ds.InventoryPath is always "".
func datastoreInventoryPath(ctx context.Context, client *vmware.Client, ds *object.Datastore) (string, error) {
	p, err := find.InventoryPath(ctx, client.Client.Client, ds.Reference())
	if err != nil {
		return "", fmt.Errorf("failed to resolve inventory path for datastore %s: %w", ds.Reference().Value, err)
	}
	return p, nil
}

func handleHostStorageQueryUnresolvedVmfsVolumes(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}
	ss, err := hostStorageSystem(ctx, host)
	if err != nil {
		return "", err
	}

	vols, err := ss.QueryUnresolvedVmfsVolumes(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to query unresolved VMFS volumes on %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{
		"host":    host.InventoryPath,
		"count":   len(vols),
		"volumes": vols,
	})
}

func handleHostStorageRetrieveDiskPartitionInfo(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}
	devicePath, _ := args["device_path"].(string)
	if devicePath == "" {
		return "", fmt.Errorf("device_path is required")
	}
	ss, err := hostStorageSystem(ctx, host)
	if err != nil {
		return "", err
	}

	info, err := ss.RetrieveDiskPartitionInfo(ctx, devicePath)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve disk partition info for %s on %s: %w", devicePath, host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{
		"host":        host.InventoryPath,
		"device_path": devicePath,
		"result":      info,
	})
}

func handleHostStorageComputeDiskPartitionInfo(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}
	devicePath, _ := args["device_path"].(string)
	if devicePath == "" {
		return "", fmt.Errorf("device_path is required")
	}
	rawLayout, ok := args["layout"]
	if !ok {
		return "", fmt.Errorf("layout is required")
	}
	var layout types.HostDiskPartitionLayout
	if err := decodeJSONArg(rawLayout, &layout); err != nil {
		return "", fmt.Errorf("invalid layout: %w", err)
	}

	ss, err := hostStorageSystem(ctx, host)
	if err != nil {
		return "", err
	}

	info, err := ss.ComputeDiskPartitionInfo(ctx, devicePath, layout)
	if err != nil {
		return "", fmt.Errorf("failed to compute disk partition info for %s on %s: %w", devicePath, host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{
		"host":        host.InventoryPath,
		"device_path": devicePath,
		"result":      info,
	})
}

func handleHostStorageAttachScsiLun(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}
	uuid, _ := args["uuid"].(string)
	if uuid == "" {
		return "", fmt.Errorf("uuid is required")
	}
	ss, err := hostStorageSystem(ctx, host)
	if err != nil {
		return "", err
	}

	if err := ss.AttachScsiLun(ctx, uuid); err != nil {
		return "", fmt.Errorf("failed to attach SCSI LUN %s on %s: %w", uuid, host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "uuid": uuid, "result": "attached"})
}

func handleHostStorageMarkAsLocal(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}
	uuid, _ := args["uuid"].(string)
	if uuid == "" {
		return "", fmt.Errorf("uuid is required")
	}
	ss, err := hostStorageSystem(ctx, host)
	if err != nil {
		return "", err
	}

	task, err := ss.MarkAsLocal(ctx, uuid)
	if err != nil {
		return "", fmt.Errorf("failed to mark disk %s as local on %s: %w", uuid, host.InventoryPath, err)
	}
	if err := waitForTask(ctx, task); err != nil {
		return "", fmt.Errorf("mark-as-local task failed for disk %s on %s: %w", uuid, host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "uuid": uuid, "result": "marked_as_local"})
}

func handleHostStorageMarkAsNonLocal(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}
	uuid, _ := args["uuid"].(string)
	if uuid == "" {
		return "", fmt.Errorf("uuid is required")
	}
	ss, err := hostStorageSystem(ctx, host)
	if err != nil {
		return "", err
	}

	task, err := ss.MarkAsNonLocal(ctx, uuid)
	if err != nil {
		return "", fmt.Errorf("failed to mark disk %s as non-local on %s: %w", uuid, host.InventoryPath, err)
	}
	if err := waitForTask(ctx, task); err != nil {
		return "", fmt.Errorf("mark-as-non-local task failed for disk %s on %s: %w", uuid, host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "uuid": uuid, "result": "marked_as_non_local"})
}

func handleHostStorageMarkAsSsd(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}
	uuid, _ := args["uuid"].(string)
	if uuid == "" {
		return "", fmt.Errorf("uuid is required")
	}
	ss, err := hostStorageSystem(ctx, host)
	if err != nil {
		return "", err
	}

	task, err := ss.MarkAsSsd(ctx, uuid)
	if err != nil {
		return "", fmt.Errorf("failed to mark disk %s as SSD on %s: %w", uuid, host.InventoryPath, err)
	}
	if err := waitForTask(ctx, task); err != nil {
		return "", fmt.Errorf("mark-as-ssd task failed for disk %s on %s: %w", uuid, host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "uuid": uuid, "result": "marked_as_ssd"})
}

func handleHostStorageMarkAsNonSsd(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}
	uuid, _ := args["uuid"].(string)
	if uuid == "" {
		return "", fmt.Errorf("uuid is required")
	}
	ss, err := hostStorageSystem(ctx, host)
	if err != nil {
		return "", err
	}

	task, err := ss.MarkAsNonSsd(ctx, uuid)
	if err != nil {
		return "", fmt.Errorf("failed to mark disk %s as non-SSD on %s: %w", uuid, host.InventoryPath, err)
	}
	if err := waitForTask(ctx, task); err != nil {
		return "", fmt.Errorf("mark-as-non-ssd task failed for disk %s on %s: %w", uuid, host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "uuid": uuid, "result": "marked_as_non_ssd"})
}

func handleHostStorageRefresh(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}
	ss, err := hostStorageSystem(ctx, host)
	if err != nil {
		return "", err
	}

	if err := ss.Refresh(ctx); err != nil {
		return "", fmt.Errorf("failed to refresh storage system on %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "result": "refreshed"})
}

func handleHostStorageRescanAllHba(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}
	ss, err := hostStorageSystem(ctx, host)
	if err != nil {
		return "", err
	}

	if err := ss.RescanAllHba(ctx); err != nil {
		return "", fmt.Errorf("failed to rescan HBAs on %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "result": "hbas_rescanned"})
}

func handleHostStorageRescanVmfs(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}
	ss, err := hostStorageSystem(ctx, host)
	if err != nil {
		return "", err
	}

	if err := ss.RescanVmfs(ctx); err != nil {
		return "", fmt.Errorf("failed to rescan VMFS on %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "result": "vmfs_rescanned"})
}

func handleHostStorageUpdateDiskPartitionInfo(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}
	devicePath, _ := args["device_path"].(string)
	if devicePath == "" {
		return "", fmt.Errorf("device_path is required")
	}
	rawSpec, ok := args["spec"]
	if !ok {
		return "", fmt.Errorf("spec is required")
	}
	var spec types.HostDiskPartitionSpec
	if err := decodeJSONArg(rawSpec, &spec); err != nil {
		return "", fmt.Errorf("invalid spec: %w", err)
	}

	ss, err := hostStorageSystem(ctx, host)
	if err != nil {
		return "", err
	}

	if err := ss.UpdateDiskPartitionInfo(ctx, devicePath, spec); err != nil {
		return "", fmt.Errorf("failed to update disk partition info for %s on %s: %w", devicePath, host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "device_path": devicePath, "result": "partition_info_updated"})
}

func handleHostStorageUnmountVmfsVolume(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}
	vmfsUuid, _ := args["vmfs_uuid"].(string)
	if vmfsUuid == "" {
		return "", fmt.Errorf("vmfs_uuid is required")
	}
	ss, err := hostStorageSystem(ctx, host)
	if err != nil {
		return "", err
	}

	if err := ss.UnmountVmfsVolume(ctx, vmfsUuid); err != nil {
		return "", fmt.Errorf("failed to unmount VMFS volume %s on %s: %w", vmfsUuid, host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "vmfs_uuid": vmfsUuid, "result": "unmounted"})
}

func handleHostDatastoreQueryAvailableDisksForVmfs(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}
	dss, err := hostDatastoreSystem(ctx, host)
	if err != nil {
		return "", err
	}

	disks, err := dss.QueryAvailableDisksForVmfs(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to query available disks for VMFS on %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{
		"host":  host.InventoryPath,
		"count": len(disks),
		"disks": disks,
	})
}

func handleHostDatastoreQueryVmfsDatastoreCreateOptions(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}
	devicePath, _ := args["device_path"].(string)
	if devicePath == "" {
		return "", fmt.Errorf("device_path is required")
	}
	dss, err := hostDatastoreSystem(ctx, host)
	if err != nil {
		return "", err
	}

	opts, err := dss.QueryVmfsDatastoreCreateOptions(ctx, devicePath)
	if err != nil {
		return "", fmt.Errorf("failed to query VMFS datastore create options for %s on %s: %w", devicePath, host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{
		"host":        host.InventoryPath,
		"device_path": devicePath,
		"count":       len(opts),
		"options":     opts,
	})
}

func handleHostDatastoreCreateLocal(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}
	name, _ := args["name"].(string)
	if name == "" {
		return "", fmt.Errorf("name is required")
	}
	dsPath, _ := args["path"].(string)
	if dsPath == "" {
		return "", fmt.Errorf("path is required")
	}
	dss, err := hostDatastoreSystem(ctx, host)
	if err != nil {
		return "", err
	}

	ds, err := dss.CreateLocalDatastore(ctx, name, dsPath)
	if err != nil {
		return "", fmt.Errorf("failed to create local datastore %q on %s: %w", name, host.InventoryPath, err)
	}
	dsInvPath, err := datastoreInventoryPath(ctx, client, ds)
	if err != nil {
		return "", err
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "datastore": dsInvPath, "result": "created"})
}

func handleHostDatastoreCreateNas(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}
	rawSpec, ok := args["spec"]
	if !ok {
		return "", fmt.Errorf("spec is required")
	}
	var spec types.HostNasVolumeSpec
	if err := decodeJSONArg(rawSpec, &spec); err != nil {
		return "", fmt.Errorf("invalid spec: %w", err)
	}

	dss, err := hostDatastoreSystem(ctx, host)
	if err != nil {
		return "", err
	}

	ds, err := dss.CreateNasDatastore(ctx, spec)
	if err != nil {
		return "", fmt.Errorf("failed to create NAS datastore on %s: %w", host.InventoryPath, err)
	}
	dsInvPath, err := datastoreInventoryPath(ctx, client, ds)
	if err != nil {
		return "", err
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "datastore": dsInvPath, "result": "created"})
}

func handleHostDatastoreCreateVmfs(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}
	rawSpec, ok := args["spec"]
	if !ok {
		return "", fmt.Errorf("spec is required")
	}
	var spec types.VmfsDatastoreCreateSpec
	if err := decodeJSONArg(rawSpec, &spec); err != nil {
		return "", fmt.Errorf("invalid spec: %w", err)
	}

	dss, err := hostDatastoreSystem(ctx, host)
	if err != nil {
		return "", err
	}

	ds, err := dss.CreateVmfsDatastore(ctx, spec)
	if err != nil {
		return "", fmt.Errorf("failed to create VMFS datastore on %s: %w", host.InventoryPath, err)
	}
	dsInvPath, err := datastoreInventoryPath(ctx, client, ds)
	if err != nil {
		return "", err
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "datastore": dsInvPath, "result": "created"})
}

func handleHostDatastoreResignatureUnresolvedVmfsVolumes(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}
	raw, ok := args["device_paths"]
	if !ok {
		return "", fmt.Errorf("device_paths is required")
	}
	devicePaths, err := toStringSlice(raw)
	if err != nil {
		return "", fmt.Errorf("invalid device_paths: %w", err)
	}
	if len(devicePaths) == 0 {
		return "", fmt.Errorf("device_paths must be a non-empty array")
	}

	dss, err := hostDatastoreSystem(ctx, host)
	if err != nil {
		return "", err
	}

	task, err := dss.ResignatureUnresolvedVmfsVolumes(ctx, devicePaths)
	if err != nil {
		return "", fmt.Errorf("failed to resignature unresolved VMFS volumes on %s: %w", host.InventoryPath, err)
	}
	if err := waitForTask(ctx, task); err != nil {
		return "", fmt.Errorf("resignature-unresolved-vmfs-volumes task failed for %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "device_paths": devicePaths, "result": "resignatured"})
}

func handleHostDatastoreRemove(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}
	dsName, _ := args["datastore"].(string)
	ds, err := resolveDatastore(ctx, client, dsName)
	if err != nil {
		return "", err
	}
	dsPath := ds.InventoryPath

	dss, err := hostDatastoreSystem(ctx, host)
	if err != nil {
		return "", err
	}

	if err := dss.Remove(ctx, ds); err != nil {
		return "", fmt.Errorf("failed to remove datastore %s from %s: %w", dsPath, host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "datastore": dsPath, "result": "removed"})
}

package tools

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// registerInventoryFolderTools is the "folder/datacenter" slice of Fase 6 of
// the codegen plan
// (".workspace/plans/MCPVMWare2026-08-10-175300-plano-cobertura-completa-api-codegen.plan.md")
// — object.Folder (11 methods) + object.Datacenter (3 methods), the
// Inventory/Organization domain, hand-transcribed from the real
// referencia/govmomi/object/folder.go and datacenter.go source following the
// inventory.go/generated_network.go/generated_host_storage.go/
// generated_vm_lifecycle.go conventions. Every signature was re-verified
// directly against the real source before transcribing (not trusted from the
// brief) — 2 signature-adjacent corrections were needed, noted below.
//
// Curation deviations from the brief (human review required):
//
//   - resolveDatacenter is NOT defined in this file: generated_datastore_browser.go
//     (Fase 4) already defines exactly this helper
//     (`resolveDatacenter(ctx, client, name string) (*object.Datacenter,
//     error)`) — this file's handlers extract "datacenter" from args and
//     call that existing SSOT helper directly, rather than duplicating it
//     with a slightly different (args map) signature, per this project's
//     "reuse SSOT, never duplicate" convention. A first draft did duplicate
//     it and failed to compile ("resolveDatacenter redeclared in this
//     block") — fixed by deleting the duplicate.
//
//   - resolveFolder's default per-datacenter sub-folder scope is NOT a
//     single "vm" default for every tool, despite the brief's suggestion —
//     verified against each simulator handler's folderHasChildType check in
//     referencia/govmomi/simulator/folder.go, not assumed:
//     AddStandaloneHostTask and CreateClusterEx both require the folder to
//     have "ComputeResource"+"Folder" in ChildType — that is the
//     datacenter's HOST folder, not "vm" — so vmware_folder_add_standalone_host
//     and vmware_folder_create_cluster default-scope "folder" under "host".
//     CreateStoragePod requires "StoragePod" ChildType — the DATASTORE
//     folder — so vmware_folder_create_storage_pod defaults to "datastore".
//     CreateDVSTask itself performs no ChildType check in vcsim, but real
//     vSphere only allows creating a DVS under a datacenter's NETWORK
//     folder, so vmware_folder_create_dvs defaults to "network" anyway
//     (matches real-world usage even though vcsim wouldn't catch a mistake
//     here). vmware_folder_create_vm, vmware_folder_register_vm,
//     vmware_folder_create_folder, and vmware_folder_children all default to
//     "vm" per the brief (the common case — CreateVM/RegisterVM only ever
//     target the VM folder tree in practice, and CreateFolder/Children work
//     under any folder type). vmware_folder_create_datacenter and
//     vmware_folder_place_vms_xcluster use NO default scope at all
//     (dcScopedPath is not applied — the caller's "folder" value is passed
//     to the Finder exactly as given): CreateDatacenter requires
//     "Datacenter"+"Folder" ChildType, which only the inventory ROOT folder
//     (or a plain folder created directly under it) ever has — none of the
//     4 well-known per-datacenter sub-folders qualify, so there is no
//     sensible per-datacenter default to apply. PlaceVmsXCluster is even
//     stricter: referencia/govmomi/simulator/folder.go's PlaceVmsXCluster
//     handler explicitly rejects any request whose target is not the exact
//     root folder (`req.This != ctx.Map.content().RootFolder` →
//     types.InvalidRequest), confirmed by reading the simulator source, not
//     assumed — so that tool's "folder" argument must resolve to "/"
//     (the root) and no other value will ever succeed against real vSphere
//     either. Every one of these defaults is bypassed by passing an
//     absolute inventory path (starting with "/") — dcScopedPath already
//     passes those through unchanged (see finder.go).
//
//   - vmware_folder_place_vms_xcluster is registered READ-ONLY (r.register,
//     no tier), per the brief: Folder.PlaceVmsXCluster's real method body
//     (confirmed by reading the source) calls the plain SOAP method
//     `PlaceVmsXCluster`, not a `_Task` variant — it is a synchronous DRS
//     cross-cluster placement RECOMMENDATION, not an action, same
//     "propose, don't apply" semantics as vmware_storage_recommend_datastores
//     (Fase 4) and vmware_host_storage_compute_disk_partition_info (Fase 3).
//
//   - Task-vs-not-a-Task, verified against the real source for every one of
//     the 14 methods (not assumed from the brief): CreateDatacenter,
//     CreateCluster, CreateFolder, CreateStoragePod (all on Folder),
//     Children (Folder), Folders (Datacenter), and PlaceVmsXCluster (Folder)
//     are ALL plain synchronous calls — no *object.Task, no waitForTask.
//     AddStandaloneHost, CreateVM, RegisterVM, CreateDVS, MoveInto (all on
//     Folder) and Destroy, PowerOnVM (both on Datacenter) DO return
//     *object.Task and are waited on.
//
//   - Folder.CreateVM's pool argument is dereferenced UNCONDITIONALLY inside
//     govmomi (`Pool: pool.Reference()`, no nil-check) — since
//     Common.Reference() has a value receiver, calling it through a nil
//     *ResourcePool panics (Go must dereference the nil pointer to obtain
//     the Common value to pass as receiver). resource_pool is therefore a
//     REQUIRED argument for vmware_folder_create_vm — not optional, despite
//     the Go signature's "pool *ResourcePool" reading as if it might be.
//     host stays optional: govmomi DOES nil-check host before dereferencing
//     it. This mirrors generated_vm_lifecycle.go's base_snapshot
//     required-despite-being-a-pointer precedent (QueryChangedDiskAreas).
//
//   - Folder.RegisterVM's pool CAN be nil (govmomi nil-checks it before use)
//     — but the simulator's business logic (registerVM.Run in
//     simulator/folder.go) requires either (host set AND pool unset, for
//     asTemplate:true) or (pool set, otherwise), and real vSphere enforces
//     the same rule. Both resource_pool and host are OPTIONAL arguments on
//     vmware_folder_register_vm; the server validates the combination and
//     returns a clear InvalidArgument fault if violated — this project's
//     established "don't duplicate business-rule validation client-side"
//     convention (see generated_host_storage.go, generated_vm_lifecycle.go).
//
//   - Folder.AddStandaloneHost's compResSpec parameter is
//     *types.BaseComputeResourceConfigSpec — a POINTER TO AN INTERFACE, not
//     a pointer to a struct. It is decoded into the base concrete
//     implementer types.ComputeResourceConfigSpec (confirmed via
//     vim25/types/if.go's `func (b *ComputeResourceConfigSpec)
//     GetComputeResourceConfigSpec() *ComputeResourceConfigSpec`), then
//     wrapped as `&iface` — same "decode into the base concrete struct that
//     implements the polymorphic interface" approach as
//     generated_vm_snapshot.go's decodeQuiesceSpec.
//
//   - Folder.CreateDVS's spec.ConfigSpec field is a polymorphic
//     types.BaseDVSConfigSpec — encoding/json cannot decode into a bare
//     interface field (none of these vim25/types structs define a JSON
//     UnmarshalJSON; typeattr-based polymorphism is an XML/SOAP-only
//     mechanism), so it is split into a separate top-level config_spec
//     argument decoded into the concrete types.VMwareDVSConfigSpec (the same
//     type generated_network.go's vmware_dvs_reconfigure already uses for
//     the standard VMware DVS shape) plus an optional product_info argument
//     — same "split the polymorphic field out of its parent object" pattern
//     as generated_host_network.go's dns_config/ip_route_config split
//     (confirmed necessary the same way that file's top comment documents,
//     re-verified here rather than assumed).
//
//   - InventoryPath gotcha (this project's single most common bug across
//     every prior fase, re-confirmed here rather than assumed): every
//     object CreateDatacenter/CreateCluster/CreateFolder/CreateStoragePod
//     hand back, and every moref a create-Task's Result carries
//     (AddStandaloneHost/CreateVM/RegisterVM/CreateDVS — confirmed by
//     reading each simulator handler's CreateTask closure, e.g.
//     folder.go's CreateDVSTask ultimately returning dvs.Reference()), is
//     built from a bare moref via a raw NewXxx(c, ref) constructor or is a
//     raw types.ManagedObjectReference — never through a Finder call — so
//     .InventoryPath is always "". find.InventoryPath(ctx,
//     client.Client.Client, ref) is used everywhere instead, same fix as
//     generated_vm_lifecycle.go's handleVMHostSystem/handleVMResourcePool.
//     Folder.Children()'s returned []object.Reference values are ALSO built
//     via the bare object.NewReference(c, ref) constructor (object/types.go)
//     — same fix applied per child. Datacenter.Folders() is the ONE
//     exception: reading object/datacenter.go's real source shows it builds
//     each sub-folder's InventoryPath CLIENT-SIDE by string-joining the
//     resolved Datacenter's own InventoryPath (`path.Join(dcPath, "vm")`
//     etc.) — no extra round trip needed there. resolvedFolderPath below
//     still falls back to find.InventoryPath defensively (cheap insurance)
//     in the unexpected case that field ever comes back empty, verified
//     empirically non-empty by TestInventoryFolderTools_DatacenterFolders.
//
//   - Folder.CreateDatacenter and Folder.CreateCluster both legitimately
//     return (nil, nil) — no error — when the connected endpoint is a
//     standalone ESXi host, not a vCenter (per the real source's own
//     comment: "Response will be nil if this is an ESX host that does not
//     belong to a vCenter"). Both handlers report this case as
//     {"result": "not_supported_on_standalone_esxi"} rather than as an
//     error — same "0/no result is not a tool error" convention as vm.go's
//     isNoResourcePoolErr — since the call itself did not fail; standalone
//     ESXi simply has no concept of multiple datacenters/clusters via this
//     API. This domain being "vsphere-general" (must work against both
//     ESXi and vCenter) makes this a real, not hypothetical, case.
//
//   - Datacenter.PowerOnVM likewise returns (nil, nil) on success against a
//     standalone ESXi host: govmomi's own implementation
//     (object/datacenter.go) loops PowerOn serially per VM client-side and
//     never creates a Task in that branch (`if d.Client().IsVC() {
//     ...Task...} else { for _, ref := range vm { ...obj.PowerOn... } }`).
//     handleDatacenterPowerOnVM checks for a nil task and skips
//     WaitForResult, reporting success directly — a "the task can
//     legitimately be nil on success" case new to this domain (no Fase 2-5
//     tool needed it).
//
//   - vcsim support confirmed by the real Go method/request-type name (not
//     receiver name), same discipline as every prior fase: all 14 tools have
//     a real simulator handler — Folder.{AddStandaloneHostTask, CreateFolder,
//     CreateStoragePod, CreateDatacenter, CreateClusterEx, CreateVMTask,
//     RegisterVMTask, MoveIntoFolderTask, CreateDVSTask, PlaceVmsXCluster}
//     and Datacenter.{DestroyTask, PowerOnMultiVMTask} all have matching
//     receiver methods in referencia/govmomi/simulator/folder.go and
//     datacenter.go (grepped directly). Children/Folders need no simulator
//     handler at all — both are pure PropertyCollector reads against the
//     standard mo.Folder/mo.Datacenter property set vcsim already serves for
//     every object, so they get real functional tests too.
//
//   - simulator.VPX()'s default Model already creates 1 DVS named "DVS0" per
//     datacenter (Model.Portgroup:1 side effect, confirmed — a sibling Fase
//     5 test hit exactly this and wasted time on a DuplicateName collision);
//     TestInventoryFolderTools_CreateDVS uses a distinct name to avoid it.
//
//   - Datacenter.DestroyTask's simulator implementation faults
//     types.ResourceInUse if the datacenter's vm/host folders are non-empty
//     (confirmed by reading simulator/datacenter.go's DestroyTask). The
//     success-path test creates a fresh, empty datacenter via
//     vmware_folder_create_datacenter first, rather than trying to destroy
//     the Model's pre-populated default datacenter.
func registerInventoryFolderTools(r *Registry) {
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}
	datacenterArg := map[string]interface{}{
		"type":        "string",
		"description": `Datacenter identifier: a name/pattern (e.g. "DC0") or a full inventory path, as returned by vmware_list_datacenters. Must resolve to exactly one datacenter.`,
	}
	hostArg := map[string]interface{}{
		"type":        "string",
		"description": `Optional host identifier (see vmware_list_hosts) to place the VM/registration on. Omit to let vSphere pick.`,
	}
	resourcePoolArg := map[string]interface{}{
		"type":        "string",
		"description": `Resource pool name/pattern or full inventory path (e.g. "Resources") as returned by vmware_list_resource_pools.`,
	}

	// --- Read-only --------------------------------------------------------

	r.register("vmware_folder_children",
		"List the immediate children (VMs, sub-folders, hosts, datastores, networks, etc. depending on folder type) of an inventory folder.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"folder": folderArgSchema("vm")},
			"required":   []interface{}{"folder"},
		},
		Tool{Handler: handleFolderChildren},
	)

	r.register("vmware_folder_place_vms_xcluster",
		`Compute (but do not apply) a cross-cluster DRS initial-placement RECOMMENDATION for one or more VMs across a set of resource pools/clusters. Read-only — it returns a types.PlaceVmsXClusterResult with recommendations/faults, it does not create, move, or power on anything. The "folder" argument must resolve to the inventory ROOT folder ("/") — real vSphere/vcsim reject this call against any other folder.`,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"folder": folderArgSchema(""),
				"spec":   map[string]interface{}{"type": "object", "description": `A types.PlaceVmsXClusterSpec JSON object matching its Go struct fields — "resourcePools" (array of {"type":"ResourcePool","value":"..."} moref objects, required, non-empty) is the main input; "placementType" defaults to "createAndPowerOn" if omitted.`},
			},
			"required": []interface{}{"folder", "spec"},
		},
		Tool{Handler: handleFolderPlaceVmsXCluster},
	)

	r.register("vmware_datacenter_folders",
		`Get the 4 well-known inventory sub-folders of a datacenter (vm/host/datastore/network) with their resolved inventory paths.`,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"datacenter": datacenterArg},
			"required":   []interface{}{"datacenter"},
		},
		Tool{Handler: handleDatacenterFolders},
	)

	// --- Folder: Tier 2 (disruptive but reversible) ------------------------

	r.registerDestructive("vmware_folder_add_standalone_host",
		"Add a standalone ESXi host to vCenter's inventory under a folder (must be a datacenter's \"host\" folder — see this file's top doc comment). No-op/error if called against a standalone ESXi endpoint (adding a host to itself makes no sense).",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"folder":        folderArgSchema("host"),
				"spec":          map[string]interface{}{"type": "object", "description": `A types.HostConnectSpec JSON object matching its Go struct fields — "hostName" is required, "userName"/"password" are required for a real connection attempt to succeed.`},
				"add_connected": map[string]interface{}{"type": "boolean", "description": "Mark the host as connected immediately after adding it. Default false."},
				"license":       map[string]interface{}{"type": "string", "description": "Optional license key to assign to the host."},
				"comp_res_spec": map[string]interface{}{"type": "object", "description": "Optional types.ComputeResourceConfigSpec JSON object (the base/common compute-resource config — e.g. vmSwapPlacement) matching its Go struct fields."},
				"confirm":       confirmArg,
			},
			"required": []interface{}{"folder", "spec", "confirm"},
		},
		Tool{Handler: handleFolderAddStandaloneHost},
	)

	r.registerDestructive("vmware_folder_create_cluster",
		"Create a new cluster (ClusterComputeResource) under a folder (must be a datacenter's \"host\" folder — see this file's top doc comment). Reports {\"result\":\"not_supported_on_standalone_esxi\"} (not an error) when connected directly to an ESXi host.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"folder":  folderArgSchema("host"),
				"name":    map[string]interface{}{"type": "string", "description": "Name for the new cluster."},
				"spec":    map[string]interface{}{"type": "object", "description": "Optional types.ClusterConfigSpecEx JSON object matching its Go struct fields (DRS/HA config, swap placement, etc.). Omit for a plain cluster with no DRS/HA config."},
				"confirm": confirmArg,
			},
			"required": []interface{}{"folder", "name", "confirm"},
		},
		Tool{Handler: handleFolderCreateCluster},
	)

	r.registerDestructive("vmware_folder_create_datacenter",
		`Create a new datacenter under a folder. The folder must have "Datacenter" among its allowed child types — in practice this is the inventory ROOT folder ("/") or a plain folder created directly under it, NOT one of the 4 well-known per-datacenter sub-folders. Reports {"result":"not_supported_on_standalone_esxi"} (not an error) when connected directly to an ESXi host.`,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"folder":  folderArgSchema(""),
				"name":    map[string]interface{}{"type": "string", "description": "Name for the new datacenter."},
				"confirm": confirmArg,
			},
			"required": []interface{}{"folder", "name", "confirm"},
		},
		Tool{Handler: handleFolderCreateDatacenter},
	)

	r.registerDestructive("vmware_folder_create_dvs",
		"Create a new Distributed Virtual Switch under a folder (must be a datacenter's \"network\" folder — see this file's top doc comment).",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"folder":       folderArgSchema("network"),
				"config_spec":  map[string]interface{}{"type": "object", "description": `A types.VMwareDVSConfigSpec JSON object matching its Go struct fields — "name" is required (also used for the DVS's inventory name). Corresponds to DVSCreateSpec.configSpec, split out here because the real field is a polymorphic interface that cannot be nested inside a generic JSON object — see this file's top doc comment.`},
				"product_info": map[string]interface{}{"type": "object", "description": "Optional types.DistributedVirtualSwitchProductSpec JSON object. Omit to let the server pick the latest supported version."},
				"confirm":      confirmArg,
			},
			"required": []interface{}{"folder", "config_spec", "confirm"},
		},
		Tool{Handler: handleFolderCreateDVS},
	)

	r.registerDestructive("vmware_folder_create_folder",
		"Create a new sub-folder under a folder.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"folder":  folderArgSchema("vm"),
				"name":    map[string]interface{}{"type": "string", "description": "Name for the new sub-folder."},
				"confirm": confirmArg,
			},
			"required": []interface{}{"folder", "name", "confirm"},
		},
		Tool{Handler: handleFolderCreateFolder},
	)

	r.registerDestructive("vmware_folder_create_storage_pod",
		"Create a new Storage DRS datastore cluster (StoragePod) under a folder (must be a datacenter's \"datastore\" folder — see this file's top doc comment).",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"folder":  folderArgSchema("datastore"),
				"name":    map[string]interface{}{"type": "string", "description": "Name for the new storage pod."},
				"confirm": confirmArg,
			},
			"required": []interface{}{"folder", "name", "confirm"},
		},
		Tool{Handler: handleFolderCreateStoragePod},
	)

	r.registerDestructive("vmware_folder_create_vm",
		"Create a new virtual machine under a folder from a full config spec. Unlike vmware_vm_reconfigure's narrow num_cpus/memory_mb subset, this accepts the WHOLE types.VirtualMachineConfigSpec as generic JSON — a caller building one from scratch needs to know the real struct field names (e.g. \"files\":{\"vmPathName\":\"[datastore1]\"}).",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"folder":        folderArgSchema("vm"),
				"config":        map[string]interface{}{"type": "object", "description": `A types.VirtualMachineConfigSpec JSON object matching its Go struct fields. "name" and "files.vmPathName" (e.g. "[datastore1]") are required by vSphere. Polymorphic sub-fields (e.g. deviceChange) will NOT decode from JSON — omit them and reconfigure devices afterward with a device-management tool.`},
				"resource_pool": resourcePoolArg,
				"host":          hostArg,
				"confirm":       confirmArg,
			},
			"required": []interface{}{"folder", "config", "resource_pool", "confirm"},
		},
		Tool{Handler: handleFolderCreateVM},
	)

	r.registerDestructive("vmware_folder_move_into",
		"Move one or more existing inventory objects (VMs, sub-folders, etc.) into a folder.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"folder":  folderArgSchema("vm"),
				"refs":    map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "object", "properties": map[string]interface{}{"type": map[string]interface{}{"type": "string"}, "value": map[string]interface{}{"type": "string"}}}, "description": `Array of managed object references to move, e.g. [{"type":"VirtualMachine","value":"vm-123"}]. Non-empty, required.`},
				"confirm": confirmArg,
			},
			"required": []interface{}{"folder", "refs", "confirm"},
		},
		Tool{Handler: handleFolderMoveInto},
	)

	r.registerDestructive("vmware_folder_register_vm",
		"Register an existing .vmx file on a datastore as a virtual machine (or template) under a folder.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"folder":        folderArgSchema("vm"),
				"path":          map[string]interface{}{"type": "string", "description": `Datastore path to the existing .vmx file, e.g. "[datastore1] myvm/myvm.vmx". Required.`},
				"name":          map[string]interface{}{"type": "string", "description": "Name to register the VM under. Omit to derive it from the .vmx path."},
				"as_template":   map[string]interface{}{"type": "boolean", "description": `Register as a template instead of a regular VM. Requires "host" to be set and "resource_pool" to be omitted. Default false.`},
				"resource_pool": map[string]interface{}{"type": "string", "description": resourcePoolArg["description"].(string) + ` Required unless as_template is true.`},
				"host":          hostArg,
				"confirm":       confirmArg,
			},
			"required": []interface{}{"folder", "path", "confirm"},
		},
		Tool{Handler: handleFolderRegisterVM},
	)

	// --- Datacenter: Tier 1 (irreversible) ---------------------------------

	r.registerDestructive("vmware_datacenter_destroy",
		"Permanently delete a datacenter and everything in it. Irreversible. Fails with ResourceInUse if the datacenter's vm/host folders still contain anything — empty them first.",
		tier1,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"datacenter": datacenterArg,
				"confirm":    confirmArg,
			},
			"required": []interface{}{"datacenter", "confirm"},
		},
		Tool{Handler: handleDatacenterDestroy},
	)

	// --- Datacenter: Tier 2 (disruptive but reversible) --------------------

	r.registerDestructive("vmware_datacenter_power_on_vm",
		"Power on multiple virtual machines in a datacenter with a single call (more efficient than calling vmware_vm_power_on once per VM against vCenter). Against a standalone ESXi host, VMs are powered on serially instead (no batching available) — reported the same way.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"datacenter": datacenterArg,
				"vms":        map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": `VM identifiers by name/pattern (see vmware_list_vms), each resolved to exactly one VM. Non-empty, required.`},
				"option":     map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "object"}, "description": `Optional array of types.OptionValue JSON objects ({"key":"...","value":"..."}) — low-level power-on options. Omit in the common case.`},
				"confirm":    confirmArg,
			},
			"required": []interface{}{"datacenter", "vms", "confirm"},
		},
		Tool{Handler: handleDatacenterPowerOnVM},
	)
}

// folderArgSchema builds the "folder" argument's JSON schema, documenting
// which per-datacenter sub-folder a bare (non-absolute) name/pattern is
// scoped under by default — see this file's top doc comment for why the
// default varies per tool instead of being a single "vm" default.
// defaultScope == "" means no scoping is applied at all (the value is passed
// to the Finder exactly as given — used for vmware_folder_create_datacenter
// and vmware_folder_place_vms_xcluster, whose targets don't live inside any
// datacenter's 4 well-known sub-folders).
func folderArgSchema(defaultScope string) map[string]interface{} {
	var note string
	if defaultScope == "" {
		note = `resolved EXACTLY as given, with no per-datacenter folder prefix applied — pass a full inventory path (e.g. "/" for the inventory root)`
	} else {
		note = fmt.Sprintf(`resolved under each datacenter's %q folder by default (the usual real-vSphere target for this operation)`, defaultScope)
	}
	return map[string]interface{}{
		"type": "string",
		"description": fmt.Sprintf(
			`Folder identifier: a name/pattern %s, or an absolute inventory path (starting with "/") to reach any folder directly — see vmware_folder_children. Must resolve to exactly one folder.`,
			note),
	}
}

// resolveFolder resolves the required "folder" argument to exactly one
// Folder. A bare (non-absolute) name/pattern is scoped under defaultScope
// via dcScopedPath (finder.go); an absolute path (starting with "/") is
// passed through unchanged either way. Pass defaultScope == "" to apply no
// scoping at all — see folderArgSchema's doc comment for why some of this
// file's tools need that.
func resolveFolder(ctx context.Context, client *vmware.Client, args map[string]interface{}, defaultScope string) (*object.Folder, error) {
	name, _ := args["folder"].(string)
	if name == "" {
		return nil, fmt.Errorf("folder is required")
	}
	path := name
	if defaultScope != "" {
		path = dcScopedPath(defaultScope, name)
	}
	folder, err := client.Finder.Folder(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve folder %q: %w", name, err)
	}
	return folder, nil
}

// taskResultRefPath waits for t and asserts its result is a
// types.ManagedObjectReference, then resolves the real inventory path via
// find.InventoryPath. AddStandaloneHost/CreateVM/RegisterVM/CreateDVS all
// hand back the object they created this way — a bare moref in the task's
// Result, wrapped by neither the object layer nor a Finder call (confirmed
// by reading each simulator handler's CreateTask closure — e.g.
// folder.go's CreateDVSTask ultimately returning dvs.Reference()) — so a
// freshly-built wrapper's .InventoryPath would always be empty even if we
// constructed one; find.InventoryPath resolves the real path by walking
// ManagedEntity.Parent instead. See this file's top doc comment.
func taskResultRefPath(ctx context.Context, client *vmware.Client, t *object.Task) (string, error) {
	info, err := t.WaitForResult(ctx)
	if err != nil {
		return "", err
	}
	ref, ok := info.Result.(types.ManagedObjectReference)
	if !ok {
		return "", fmt.Errorf("unexpected task result type %T (expected a managed object reference)", info.Result)
	}
	p, err := find.InventoryPath(ctx, client.Client.Client, ref)
	if err != nil {
		return "", fmt.Errorf("failed to resolve inventory path for task result %s: %w", ref.Value, err)
	}
	return p, nil
}

// resolvedFolderPath returns f.InventoryPath if already populated, falling
// back to find.InventoryPath otherwise. Datacenter.Folders() already
// populates each returned sub-folder's InventoryPath client-side (see this
// file's top doc comment), so the fallback is defensive insurance, not the
// expected path in normal use.
func resolvedFolderPath(ctx context.Context, client *vmware.Client, f *object.Folder) (string, error) {
	if f.InventoryPath != "" {
		return f.InventoryPath, nil
	}
	return find.InventoryPath(ctx, client.Client.Client, f.Reference())
}

func handleFolderChildren(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	folder, err := resolveFolder(ctx, client, args, "vm")
	if err != nil {
		return "", err
	}

	children, err := folder.Children(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to list children of %s: %w", folder.InventoryPath, err)
	}

	result := make([]map[string]interface{}, 0, len(children))
	for _, c := range children {
		ref := c.Reference()
		path, err := find.InventoryPath(ctx, client.Client.Client, ref)
		if err != nil {
			return "", fmt.Errorf("failed to resolve inventory path for child %s (%s): %w", ref.Value, ref.Type, err)
		}
		result = append(result, map[string]interface{}{"type": ref.Type, "path": path})
	}

	return marshalJSON(map[string]interface{}{
		"folder":   folder.InventoryPath,
		"count":    len(result),
		"children": result,
	})
}

func handleFolderPlaceVmsXCluster(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	folder, err := resolveFolder(ctx, client, args, "")
	if err != nil {
		return "", err
	}
	rawSpec, ok := args["spec"]
	if !ok {
		return "", fmt.Errorf("spec is required")
	}
	var spec types.PlaceVmsXClusterSpec
	if err := decodeJSONArg(rawSpec, &spec); err != nil {
		return "", fmt.Errorf("invalid spec: %w", err)
	}

	result, err := folder.PlaceVmsXCluster(ctx, spec)
	if err != nil {
		return "", fmt.Errorf("failed to compute cross-cluster placement in %s: %w", folder.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"folder": folder.InventoryPath, "result": result})
}

func handleDatacenterFolders(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	dcName, _ := args["datacenter"].(string)
	dc, err := resolveDatacenter(ctx, client, dcName)
	if err != nil {
		return "", err
	}

	folders, err := dc.Folders(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to read folders for %s: %w", dc.InventoryPath, err)
	}

	vmPath, err := resolvedFolderPath(ctx, client, folders.VmFolder)
	if err != nil {
		return "", fmt.Errorf("failed to resolve vm folder path for %s: %w", dc.InventoryPath, err)
	}
	hostPath, err := resolvedFolderPath(ctx, client, folders.HostFolder)
	if err != nil {
		return "", fmt.Errorf("failed to resolve host folder path for %s: %w", dc.InventoryPath, err)
	}
	datastorePath, err := resolvedFolderPath(ctx, client, folders.DatastoreFolder)
	if err != nil {
		return "", fmt.Errorf("failed to resolve datastore folder path for %s: %w", dc.InventoryPath, err)
	}
	networkPath, err := resolvedFolderPath(ctx, client, folders.NetworkFolder)
	if err != nil {
		return "", fmt.Errorf("failed to resolve network folder path for %s: %w", dc.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{
		"datacenter":       dc.InventoryPath,
		"vm_folder":        vmPath,
		"host_folder":      hostPath,
		"datastore_folder": datastorePath,
		"network_folder":   networkPath,
	})
}

func handleFolderAddStandaloneHost(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	folder, err := resolveFolder(ctx, client, args, "host")
	if err != nil {
		return "", err
	}
	rawSpec, ok := args["spec"]
	if !ok {
		return "", fmt.Errorf("spec is required")
	}
	var spec types.HostConnectSpec
	if err := decodeJSONArg(rawSpec, &spec); err != nil {
		return "", fmt.Errorf("invalid spec: %w", err)
	}
	addConnected, _ := args["add_connected"].(bool)

	var licensePtr *string
	if v, ok := args["license"].(string); ok && v != "" {
		licensePtr = &v
	}

	var compResSpecPtr *types.BaseComputeResourceConfigSpec
	if raw, ok := args["comp_res_spec"]; ok && raw != nil {
		var crs types.ComputeResourceConfigSpec
		if err := decodeJSONArg(raw, &crs); err != nil {
			return "", fmt.Errorf("invalid comp_res_spec: %w", err)
		}
		var iface types.BaseComputeResourceConfigSpec = &crs
		compResSpecPtr = &iface
	}

	task, err := folder.AddStandaloneHost(ctx, spec, addConnected, licensePtr, compResSpecPtr)
	if err != nil {
		return "", fmt.Errorf("failed to add standalone host to %s: %w", folder.InventoryPath, err)
	}
	hostPath, err := taskResultRefPath(ctx, client, task)
	if err != nil {
		return "", fmt.Errorf("add-standalone-host task failed for %s: %w", folder.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"folder": folder.InventoryPath, "host": hostPath, "result": "added"})
}

func handleFolderCreateCluster(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	folder, err := resolveFolder(ctx, client, args, "host")
	if err != nil {
		return "", err
	}
	name, _ := args["name"].(string)
	if name == "" {
		return "", fmt.Errorf("name is required")
	}
	var spec types.ClusterConfigSpecEx
	if raw, ok := args["spec"]; ok && raw != nil {
		if err := decodeJSONArg(raw, &spec); err != nil {
			return "", fmt.Errorf("invalid spec: %w", err)
		}
	}

	cluster, err := folder.CreateCluster(ctx, name, spec)
	if err != nil {
		return "", fmt.Errorf("failed to create cluster %q in %s: %w", name, folder.InventoryPath, err)
	}
	if cluster == nil {
		// Folder.CreateCluster returns (nil, nil) when the connected
		// endpoint is a standalone ESXi host, not a vCenter — see this
		// file's top doc comment.
		return marshalJSON(map[string]interface{}{"folder": folder.InventoryPath, "cluster": nil, "result": "not_supported_on_standalone_esxi"})
	}
	clusterPath, err := find.InventoryPath(ctx, client.Client.Client, cluster.Reference())
	if err != nil {
		return "", fmt.Errorf("failed to resolve inventory path for new cluster %q: %w", name, err)
	}

	return marshalJSON(map[string]interface{}{"folder": folder.InventoryPath, "cluster": clusterPath, "result": "created"})
}

func handleFolderCreateDatacenter(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	folder, err := resolveFolder(ctx, client, args, "")
	if err != nil {
		return "", err
	}
	name, _ := args["name"].(string)
	if name == "" {
		return "", fmt.Errorf("name is required")
	}

	dc, err := folder.CreateDatacenter(ctx, name)
	if err != nil {
		return "", fmt.Errorf("failed to create datacenter %q in %s: %w", name, folder.InventoryPath, err)
	}
	if dc == nil {
		// Folder.CreateDatacenter returns (nil, nil) when the connected
		// endpoint is a standalone ESXi host, not a vCenter — see this
		// file's top doc comment.
		return marshalJSON(map[string]interface{}{"folder": folder.InventoryPath, "datacenter": nil, "result": "not_supported_on_standalone_esxi"})
	}
	dcPath, err := find.InventoryPath(ctx, client.Client.Client, dc.Reference())
	if err != nil {
		return "", fmt.Errorf("failed to resolve inventory path for new datacenter %q: %w", name, err)
	}

	return marshalJSON(map[string]interface{}{"folder": folder.InventoryPath, "datacenter": dcPath, "result": "created"})
}

func handleFolderCreateDVS(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	folder, err := resolveFolder(ctx, client, args, "network")
	if err != nil {
		return "", err
	}
	rawConfig, ok := args["config_spec"]
	if !ok {
		return "", fmt.Errorf("config_spec is required")
	}
	var configSpec types.VMwareDVSConfigSpec
	if err := decodeJSONArg(rawConfig, &configSpec); err != nil {
		return "", fmt.Errorf("invalid config_spec: %w", err)
	}
	if configSpec.Name == "" {
		return "", fmt.Errorf("config_spec.name is required")
	}

	spec := types.DVSCreateSpec{ConfigSpec: &configSpec}
	if raw, ok := args["product_info"]; ok && raw != nil {
		var pi types.DistributedVirtualSwitchProductSpec
		if err := decodeJSONArg(raw, &pi); err != nil {
			return "", fmt.Errorf("invalid product_info: %w", err)
		}
		spec.ProductInfo = &pi
	}

	task, err := folder.CreateDVS(ctx, spec)
	if err != nil {
		return "", fmt.Errorf("failed to create DVS %q in %s: %w", configSpec.Name, folder.InventoryPath, err)
	}
	dvsPath, err := taskResultRefPath(ctx, client, task)
	if err != nil {
		return "", fmt.Errorf("create-dvs task failed for %q in %s: %w", configSpec.Name, folder.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"folder": folder.InventoryPath, "network": dvsPath, "result": "created"})
}

func handleFolderCreateFolder(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	folder, err := resolveFolder(ctx, client, args, "vm")
	if err != nil {
		return "", err
	}
	name, _ := args["name"].(string)
	if name == "" {
		return "", fmt.Errorf("name is required")
	}

	newFolder, err := folder.CreateFolder(ctx, name)
	if err != nil {
		return "", fmt.Errorf("failed to create folder %q in %s: %w", name, folder.InventoryPath, err)
	}
	newPath, err := find.InventoryPath(ctx, client.Client.Client, newFolder.Reference())
	if err != nil {
		return "", fmt.Errorf("failed to resolve inventory path for new folder %q: %w", name, err)
	}

	return marshalJSON(map[string]interface{}{"folder": folder.InventoryPath, "new_folder": newPath, "result": "created"})
}

func handleFolderCreateStoragePod(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	folder, err := resolveFolder(ctx, client, args, "datastore")
	if err != nil {
		return "", err
	}
	name, _ := args["name"].(string)
	if name == "" {
		return "", fmt.Errorf("name is required")
	}

	pod, err := folder.CreateStoragePod(ctx, name)
	if err != nil {
		return "", fmt.Errorf("failed to create storage pod %q in %s: %w", name, folder.InventoryPath, err)
	}
	podPath, err := find.InventoryPath(ctx, client.Client.Client, pod.Reference())
	if err != nil {
		return "", fmt.Errorf("failed to resolve inventory path for new storage pod %q: %w", name, err)
	}

	return marshalJSON(map[string]interface{}{"folder": folder.InventoryPath, "storage_pod": podPath, "result": "created"})
}

func handleFolderCreateVM(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	folder, err := resolveFolder(ctx, client, args, "vm")
	if err != nil {
		return "", err
	}
	rawConfig, ok := args["config"]
	if !ok {
		return "", fmt.Errorf("config is required")
	}
	var config types.VirtualMachineConfigSpec
	if err := decodeJSONArg(rawConfig, &config); err != nil {
		return "", fmt.Errorf("invalid config: %w", err)
	}

	// resource_pool is required, not optional: govmomi's Folder.CreateVM
	// dereferences pool unconditionally (`Pool: pool.Reference()`, no
	// nil-check) — passing a nil pool panics, see this file's top doc
	// comment.
	poolName, _ := args["resource_pool"].(string)
	if poolName == "" {
		return "", fmt.Errorf("resource_pool is required")
	}
	pool, err := client.Finder.ResourcePool(ctx, dcScopedPath("host", poolName))
	if err != nil {
		return "", fmt.Errorf("failed to resolve resource_pool %q: %w", poolName, err)
	}

	var host *object.HostSystem
	if hostName, ok := args["host"].(string); ok && hostName != "" {
		host, err = resolveHost(ctx, client, map[string]interface{}{"host": hostName})
		if err != nil {
			return "", err
		}
	}

	task, err := folder.CreateVM(ctx, config, pool, host)
	if err != nil {
		return "", fmt.Errorf("failed to create VM %q in %s: %w", config.Name, folder.InventoryPath, err)
	}
	vmPath, err := taskResultRefPath(ctx, client, task)
	if err != nil {
		return "", fmt.Errorf("create-vm task failed for %q in %s: %w", config.Name, folder.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"folder": folder.InventoryPath, "resource_pool": pool.InventoryPath, "vm": vmPath, "result": "created"})
}

func handleFolderMoveInto(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	folder, err := resolveFolder(ctx, client, args, "vm")
	if err != nil {
		return "", err
	}
	raw, ok := args["refs"].([]interface{})
	if !ok || len(raw) == 0 {
		return "", fmt.Errorf("refs is required and must be a non-empty array")
	}
	list := make([]types.ManagedObjectReference, 0, len(raw))
	for i, item := range raw {
		var ref types.ManagedObjectReference
		if err := decodeJSONArg(item, &ref); err != nil {
			return "", fmt.Errorf("invalid refs[%d]: %w", i, err)
		}
		if ref.Type == "" || ref.Value == "" {
			return "", fmt.Errorf(`refs[%d] must have non-empty "type" and "value"`, i)
		}
		list = append(list, ref)
	}

	task, err := folder.MoveInto(ctx, list)
	if err != nil {
		return "", fmt.Errorf("failed to move %d object(s) into %s: %w", len(list), folder.InventoryPath, err)
	}
	if err := waitForTask(ctx, task); err != nil {
		return "", fmt.Errorf("move-into-folder task failed for %s: %w", folder.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"folder": folder.InventoryPath, "count": len(list), "result": "moved"})
}

func handleFolderRegisterVM(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	folder, err := resolveFolder(ctx, client, args, "vm")
	if err != nil {
		return "", err
	}
	vmxPath, _ := args["path"].(string)
	if vmxPath == "" {
		return "", fmt.Errorf("path is required")
	}
	name, _ := args["name"].(string)
	asTemplate, _ := args["as_template"].(bool)

	var pool *object.ResourcePool
	if poolName, ok := args["resource_pool"].(string); ok && poolName != "" {
		pool, err = client.Finder.ResourcePool(ctx, dcScopedPath("host", poolName))
		if err != nil {
			return "", fmt.Errorf("failed to resolve resource_pool %q: %w", poolName, err)
		}
	}

	var host *object.HostSystem
	if hostName, ok := args["host"].(string); ok && hostName != "" {
		host, err = resolveHost(ctx, client, map[string]interface{}{"host": hostName})
		if err != nil {
			return "", err
		}
	}

	task, err := folder.RegisterVM(ctx, vmxPath, name, asTemplate, pool, host)
	if err != nil {
		return "", fmt.Errorf("failed to register VM from %q in %s: %w", vmxPath, folder.InventoryPath, err)
	}
	vmPath, err := taskResultRefPath(ctx, client, task)
	if err != nil {
		return "", fmt.Errorf("register-vm task failed for %q in %s: %w", vmxPath, folder.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"folder": folder.InventoryPath, "path": vmxPath, "vm": vmPath, "result": "registered"})
}

func handleDatacenterDestroy(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	dcName, _ := args["datacenter"].(string)
	dc, err := resolveDatacenter(ctx, client, dcName)
	if err != nil {
		return "", err
	}
	path := dc.InventoryPath

	task, err := dc.Destroy(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to destroy %s: %w", path, err)
	}
	if err := waitForTask(ctx, task); err != nil {
		return "", fmt.Errorf("destroy task failed for %s: %w", path, err)
	}

	return marshalJSON(map[string]interface{}{"datacenter": path, "result": "destroyed"})
}

func handleDatacenterPowerOnVM(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	dcName, _ := args["datacenter"].(string)
	dc, err := resolveDatacenter(ctx, client, dcName)
	if err != nil {
		return "", err
	}
	rawVMs, ok := args["vms"].([]interface{})
	if !ok || len(rawVMs) == 0 {
		return "", fmt.Errorf("vms is required and must be a non-empty array")
	}
	refs := make([]types.ManagedObjectReference, 0, len(rawVMs))
	for i, item := range rawVMs {
		name, ok := item.(string)
		if !ok || name == "" {
			return "", fmt.Errorf("vms[%d] must be a non-empty string", i)
		}
		vm, err := resolveVM(ctx, client, map[string]interface{}{"vm": name})
		if err != nil {
			return "", err
		}
		refs = append(refs, vm.Reference())
	}

	var opts []types.BaseOptionValue
	if raw, ok := args["option"]; ok && raw != nil {
		arr, ok := raw.([]interface{})
		if !ok {
			return "", fmt.Errorf("option must be an array")
		}
		for i, item := range arr {
			var ov types.OptionValue
			if err := decodeJSONArg(item, &ov); err != nil {
				return "", fmt.Errorf("invalid option[%d]: %w", i, err)
			}
			opts = append(opts, &ov)
		}
	}

	task, err := dc.PowerOnVM(ctx, refs, opts...)
	if err != nil {
		return "", fmt.Errorf("failed to power on VMs in %s: %w", dc.InventoryPath, err)
	}

	result := map[string]interface{}{"datacenter": dc.InventoryPath, "vm_count": len(refs)}
	if task == nil {
		// Datacenter.PowerOnVM powers on each VM serially client-side and
		// returns (nil, nil) on success when the connected endpoint is a
		// standalone ESXi host, not a vCenter — see this file's top doc
		// comment.
		result["result"] = "powered_on"
	} else {
		info, err := task.WaitForResult(ctx)
		if err != nil {
			return "", fmt.Errorf("power-on-vm task failed in %s: %w", dc.InventoryPath, err)
		}
		result["result"] = "powered_on"
		result["task_result"] = info.Result
	}

	return marshalJSON(result)
}

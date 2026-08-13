package tools

import (
	"context"
	"fmt"
	"path"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// registerVirtualDiskTools is the "virtual disk manager" slice of Fase 4 of
// the codegen plan
// (".workspace/plans/MCPVMWare2026-08-10-175300-plano-cobertura-completa-api-codegen.plan.md")
// — all 11 methods of object.VirtualDiskManager (10 in
// referencia/govmomi/object/virtual_disk_manager.go, 1 —
// QueryVirtualDiskInfo/CreateChildDisk are actually split: QueryVirtualDiskInfo
// and CreateChildDisk both live in virtual_disk_manager_internal.go — see
// deviation 5 below), hand-transcribed following the
// datastore.go/generated_host_storage.go/generated_vm_provisioning.go
// conventions. Every signature below was re-verified directly against the
// real referencia/govmomi/object source (not trusted from the generator
// report) — 2 corrections were needed, see deviations 3 and 5.
//
// Naming note (cross-reference for reviewers, corrected 2026-08-11 — the
// sibling "file managers" group verified this by reading both call chains,
// an earlier draft of this comment was wrong): vmware_virtual_disk_delete_virtual_disk
// (this file, Tier 1) and vmware_datastore_file_delete_virtual_disk (a
// sibling Fase 4 slice, DatastoreFileManager.DeleteVirtualDisk) are NOT
// different SOAP methods — DatastoreFileManager.DeleteVirtualDisk internally
// calls the exact same VirtualDiskManager.DeleteVirtualDisk this file wraps
// (same wire method, DeleteVirtualDisk_Task). Both tools are still kept
// deliberately: they differ in argument shape/convenience, not in
// underlying operation — this one takes an explicit Datacenter argument
// (the classic API), the DatastoreFileManager variant auto-derives it from
// the datastore and additionally supports force/markDiskAsDeletable. Use
// whichever matches the API surface you already have a handle for.
//
// Curation deviations from the brief (human review required):
//
//  1. Every method's "name" (a datastore-path string vSphere expects in
//     "[datastore] path/to/disk.vmdk" bracket notation — confirmed by
//     reading referencia/govmomi/simulator/virtual_disk_manager_test.go's
//     own fixtures, e.g. `name := "[LocalDS_0] disks/disk1.vmdk"`) is
//     decomposed here into 2 tool arguments — "<prefix>datastore" (resolved
//     via this package's existing resolveDatastore helper, datastore.go) and
//     "<prefix>path" (relative to the datastore root) — combined via
//     object.Datastore.Path(path), exactly the construction
//     referencia/govmomi/cli/datastore/disk/create.go's own govc command
//     uses (`ds.Path(f.Arg(0))`). This matches this project's existing
//     "datastore"+"path"/"remote_path" decomposition convention
//     (datastore.go's vmware_datastore_upload_file,
//     generated_host_storage.go's vmware_host_datastore_create_local)
//     instead of requiring a caller to hand-format VMware's bracket
//     notation themselves. See resolveDiskLocation below.
//
//  2. "datacenter" is a separate required argument on every tool (own
//     "<prefix>datacenter", resolved via client.Finder.Datacenter — a
//     top-level Finder lookup, no dcScopedPath needed since datacenters
//     aren't nested under another datacenter's folder), per this slice's
//     brief — even though it overlaps semantically with the datastore
//     argument (a datastore already lives under exactly one datacenter),
//     VirtualDiskManager's real SOAP methods take Datacenter as an explicit,
//     separate parameter and resolveDatastore alone does not return one.
//
//  3. CreateVirtualDisk/CopyVirtualDisk's spec/dest_spec arguments decode
//     into types.FileBackedVirtualDiskSpec, NOT the bare types.VirtualDiskSpec
//     the raw brief suggested. Confirmed necessary by reading
//     referencia/govmomi/simulator/virtual_disk_manager.go's
//     vdmCreateVirtualDisk: it type-asserts req.Spec to
//     types.BaseFileBackedVirtualDiskSpec to read CapacityKb — a bare
//     *types.VirtualDiskSpec (which has no CapacityKb field at all) fails
//     that assertion and the create/copy always faults FileFault. This
//     mirrors referencia/govmomi/cli/datastore/disk/create.go's own govc
//     command, which builds a types.FileBackedVirtualDiskSpec, never a bare
//     VirtualDiskSpec. *types.FileBackedVirtualDiskSpec still implements
//     types.BaseVirtualDiskSpec (via its embedded VirtualDiskSpec's promoted
//     GetVirtualDiskSpec method — see vim25/types/if.go), so it satisfies
//     both CreateVirtualDisk's and CopyVirtualDisk's destSpec parameter
//     type. Polymorphic sub-fields of FileBackedVirtualDiskSpec (Profile,
//     Crypto) are reachable via generic JSON decode same as elsewhere in
//     this project (decodeJSONArg), but there is no schema-guided
//     discriminator for which Crypto/Profile variant to use — same MVP
//     scope note as generated_vm_provisioning.go's CustomizationSpec.Identity.
//
//  4. vmware_virtual_disk_create and vmware_virtual_disk_create_child
//     best-effort create the disk's parent datastore folder before calling
//     CreateVirtualDisk/CreateChildDisk (see ensureDiskDir below) — the same
//     "create the destination folder if missing, swallow the error, let the
//     real call surface anything genuinely wrong" pattern datastore.go's
//     handleDatastoreUploadFile already uses. This was not just copied by
//     analogy: it is REQUIRED for CreateVirtualDisk to succeed against
//     vcsim at all — confirmed by reading
//     referencia/govmomi/simulator/virtual_disk_manager.go's
//     vdmCreateVirtualDisk (a plain os.Create(file), no parent-dir creation)
//     and referencia/govmomi/simulator/virtual_disk_manager_test.go's own
//     TestVirtualDiskManager, whose first CreateVirtualDisk call is
//     EXPECTED to fail before it explicitly calls fm.MakeDirectory once.
//     Verified empirically here too, not just by reading the reference test.
//
//  5. QueryVirtualDiskInfo and CreateChildDisk are NOT declared in
//     virtual_disk_manager.go — the brief's file pointer was imprecise, both
//     actually live in virtual_disk_manager_internal.go (confirmed by
//     reading the real source, not assumed). More notably, CreateChildDisk
//     is built on a hand-rolled *internal-namespace* SOAP request
//     (urn:internalvim25 CreateChildDisk_Task, via a private
//     createChildDiskTaskRequest/Body — not the vim25-namespaced
//     methods.CreateChildDisk_Task other tools in this file use), and
//     QueryVirtualDiskInfo similarly goes through internal.QueryVirtualDiskInfoTask,
//     not a plain methods.* call. Both are still real, supported vSphere
//     operations (CreateChildDisk backs govc's own two-argument
//     `datastore.disk.create parent.vmdk child.vmdk` form — see
//     referencia/govmomi/cli/datastore/disk/create.go) — the internal
//     namespace is an implementation detail of how govmomi's object layer
//     reaches them, not a reason to omit either tool.
//
//  6. vcsim coverage (confirmed by reading
//     referencia/govmomi/simulator/virtual_disk_manager.go method-by-method,
//     not assumed): 8 of these 11 methods have a real, functional simulator
//     handler — CreateVirtualDisk, ExtendVirtualDisk, DeleteVirtualDisk,
//     MoveVirtualDisk, CopyVirtualDisk, QueryVirtualDiskUuid,
//     QueryVirtualDiskInfo, and SetVirtualDiskUuid (see deviation 7 for a
//     sharp caveat on that last one). 3 have NO simulator-side
//     implementation at all — InflateVirtualDisk, ShrinkVirtualDisk, and
//     CreateChildDisk — confirmed by grepping the entire
//     referencia/govmomi/simulator tree for a matching receiver method and
//     finding none; calls against vcsim for those 3 always fault
//     types.MethodNotFound (simulator/simulator.go's method-dispatch
//     fallback — same mechanism documented in generated_vm_lifecycle.go's
//     and generated_host_storage.go's top comments).
//     generated_virtual_disk_test.go uses the existing assertReachesServer
//     helper (generated_vm_lifecycle_test.go, same package) for these 3: it
//     proves the plumbing (schema, tier gating, arg resolution) reaches
//     vcsim's real method dispatch and gets back a clean MethodNotFound-
//     based error, not a wiring bug (unknown tool) or a recovered panic.
//     All 3 are still registered exactly as real vSphere supports them — a
//     vcsim gap is not a reason to omit a tool real deployments can use.
//
//  7. vmware_virtual_disk_set_uuid IS reachable against vcsim (unlike
//     Inflate/Shrink/CreateChildDisk) but its simulator handler
//     (VirtualDiskManager.SetVirtualDiskUuid in
//     referencia/govmomi/simulator/virtual_disk_manager.go) is a literal
//     stub: `// TODO: validate uuid format and persist` — it unconditionally
//     returns success without storing the uuid anywhere.
//     vmware_virtual_disk_query_uuid's own simulator handler independently
//     computes a deterministic UUID from a hash of the datacenter+file
//     path (virtualDiskUUID) and NEVER reads back whatever
//     vmware_virtual_disk_set_uuid "set". A caller who calls set_uuid then
//     query_uuid against vcsim expecting the queried value to match what
//     they just set will be surprised — this is a real vcsim limitation,
//     not a bug in this file, and generated_virtual_disk_test.go asserts
//     this exact (non-)round-trip explicitly so it stays documented and
//     doesn't silently regress into an assumed-working feature. Against
//     real vCenter/ESXi this almost certainly round-trips correctly; only
//     the simulator is stubbed here.
//
//  8. Tier assignments: Tier 1 (irreversible) for DeleteVirtualDisk only —
//     matches this project's severity table (a deleted vmdk has no built-in
//     undo, same reasoning as vmware_host_datastore_remove). Tier 2
//     (disruptive but reversible or at least non-catastrophic) for the
//     other 8 mutating methods (Copy, CreateChildDisk, Create, Extend,
//     Inflate, Move, SetUuid, Shrink) per the brief. QueryVirtualDiskInfo
//     and QueryVirtualDiskUuid are read-only (r.register, no tier).
//
//  9. vmware_virtual_disk_query_info does NOT error on a disk path that
//     doesn't actually exist on disk, confirmed empirically (see
//     TestVirtualDiskTools_Registration) and by reading
//     QueryVirtualDiskInfoTask's simulator handler: it only calls
//     FileManager.resolve (syntactic datastore-path resolution), never
//     os.Stat on the vmdk file itself — unlike QueryVirtualDiskUuid, whose
//     handler does os.Stat and fails cleanly on a missing file. This is a
//     real vcsim behavior, not a bug in this file's handler; a caller
//     relying on vmware_virtual_disk_query_info to detect "does this disk
//     exist" against vcsim should use vmware_virtual_disk_query_uuid
//     instead (or test against real vCenter/ESXi, where this may differ).
func registerVirtualDiskTools(r *Registry) {
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}

	// --- CopyVirtualDisk (Tier 2) ----------------------------------------

	copyProps := map[string]interface{}{}
	addDiskLocationArgs(copyProps, "source_", "Source")
	addDiskLocationArgs(copyProps, "dest_", "Destination")
	copyProps["dest_spec"] = map[string]interface{}{
		"type":        "object",
		"description": `A types.FileBackedVirtualDiskSpec JSON object matching its Go struct field names, e.g. {"diskType":"thin","adapterType":"lsiLogic","capacityKb":1048576}. Governs how the destination disk is written (thin/thick, adapter type); the destination's actual size always comes from the source disk being copied, capacityKb here is only consulted for certain conversions. Accepted via generic JSON decode — see this file's top doc comment, deviation 3, for why FileBackedVirtualDiskSpec (not the bare VirtualDiskSpec) is required.`,
	}
	copyProps["force"] = map[string]interface{}{"type": "boolean", "description": "Overwrite the destination disk if it already exists. Default false."}
	copyProps["confirm"] = confirmArg
	r.registerDestructive("vmware_virtual_disk_copy",
		"Copy a virtual disk (vmdk), optionally converting its format (thin/thick/adapter type) per dest_spec. Classic VirtualDiskManager-scoped variant — requires an explicit source/destination datacenter, unlike the newer per-datastore file-manager copy tool.",
		tier2,
		map[string]interface{}{"type": "object", "properties": copyProps, "required": []interface{}{"source_datastore", "source_path", "source_datacenter", "dest_datastore", "dest_path", "dest_datacenter", "dest_spec", "confirm"}},
		Tool{Handler: handleVirtualDiskCopy},
	)

	// --- CreateChildDisk (Tier 2) -----------------------------------------

	childProps := map[string]interface{}{}
	addDiskLocationArgs(childProps, "parent_", "Parent")
	addDiskLocationArgs(childProps, "", "Child (new)")
	childProps["linked"] = map[string]interface{}{"type": "boolean", "description": "Create a linked clone (delta disk referencing the parent) instead of a full copy. Default false."}
	childProps["confirm"] = confirmArg
	r.registerDestructive("vmware_virtual_disk_create_child",
		"Create a child virtual disk (snapshot delta or linked clone) from an existing parent disk. Built on an internal-namespace SOAP method (see this file's top doc comment, deviation 5) but a real, supported vSphere operation — same one govc's `datastore.disk.create parent.vmdk child.vmdk` two-argument form uses. NOT simulated by this project's test harness (vcsim has no CreateChildDisk handler) — exercise carefully against a real vCenter/ESXi first.",
		tier2,
		map[string]interface{}{"type": "object", "properties": childProps, "required": []interface{}{"parent_datastore", "parent_path", "parent_datacenter", "datastore", "path", "datacenter", "confirm"}},
		Tool{Handler: handleVirtualDiskCreateChild},
	)

	// --- CreateVirtualDisk (Tier 2) ---------------------------------------

	createProps := map[string]interface{}{}
	addDiskLocationArgs(createProps, "", "")
	createProps["spec"] = map[string]interface{}{
		"type":        "object",
		"description": `A types.FileBackedVirtualDiskSpec JSON object matching its Go struct field names, e.g. {"diskType":"thin","adapterType":"lsiLogic","capacityKb":1048576}. capacityKb is required to get a disk of the intended size. See this file's top doc comment, deviation 3, for why FileBackedVirtualDiskSpec (not the bare VirtualDiskSpec) is required. Accepted via generic JSON decode.`,
	}
	createProps["confirm"] = confirmArg
	r.registerDestructive("vmware_virtual_disk_create",
		"Create a new virtual disk (vmdk) file on a datastore. The parent folder of path is created automatically if missing (same best-effort behavior as vmware_datastore_upload_file).",
		tier2,
		map[string]interface{}{"type": "object", "properties": createProps, "required": []interface{}{"datastore", "path", "datacenter", "spec", "confirm"}},
		Tool{Handler: handleVirtualDiskCreate},
	)

	// --- ExtendVirtualDisk (Tier 2) ---------------------------------------

	extendProps := map[string]interface{}{}
	addDiskLocationArgs(extendProps, "", "")
	extendProps["capacity_kb"] = map[string]interface{}{"type": "integer", "description": "New total capacity of the disk, in KB. Must be greater than or equal to the disk's current capacity — VirtualDiskManager cannot shrink a disk this way (see vmware_virtual_disk_shrink for that, which is a different operation entirely — it removes unused blocks, it does not change the logical capacity)."}
	extendProps["eager_zero"] = map[string]interface{}{"type": "boolean", "description": "Optional: zero out the newly added blocks eagerly. Omit to leave the format's default behavior unchanged."}
	extendProps["confirm"] = confirmArg
	r.registerDestructive("vmware_virtual_disk_extend",
		"Extend (grow) an existing virtual disk's capacity.",
		tier2,
		map[string]interface{}{"type": "object", "properties": extendProps, "required": []interface{}{"datastore", "path", "datacenter", "capacity_kb", "confirm"}},
		Tool{Handler: handleVirtualDiskExtend},
	)

	// --- InflateVirtualDisk (Tier 2) --------------------------------------

	inflateProps := map[string]interface{}{}
	addDiskLocationArgs(inflateProps, "", "")
	inflateProps["confirm"] = confirmArg
	r.registerDestructive("vmware_virtual_disk_inflate",
		"Inflate a thin-provisioned virtual disk into a full (thick, eager-zeroed) allocation, filling it to its declared capacity. NOTE: not simulated by this project's test harness (vcsim has no InflateVirtualDisk handler) — exercise carefully against a real vCenter/ESXi first.",
		tier2,
		map[string]interface{}{"type": "object", "properties": inflateProps, "required": []interface{}{"datastore", "path", "datacenter", "confirm"}},
		Tool{Handler: handleVirtualDiskInflate},
	)

	// --- MoveVirtualDisk (Tier 2) ------------------------------------------

	moveProps := map[string]interface{}{}
	addDiskLocationArgs(moveProps, "source_", "Source")
	addDiskLocationArgs(moveProps, "dest_", "Destination")
	moveProps["force"] = map[string]interface{}{"type": "boolean", "description": "Overwrite the destination disk if it already exists. Default false."}
	moveProps["confirm"] = confirmArg
	r.registerDestructive("vmware_virtual_disk_move",
		"Move (rename) a virtual disk from one datastore path to another. Classic VirtualDiskManager-scoped variant — requires an explicit source/destination datacenter.",
		tier2,
		map[string]interface{}{"type": "object", "properties": moveProps, "required": []interface{}{"source_datastore", "source_path", "source_datacenter", "dest_datastore", "dest_path", "dest_datacenter", "confirm"}},
		Tool{Handler: handleVirtualDiskMove},
	)

	// --- SetVirtualDiskUuid (Tier 2) --------------------------------------

	setUuidProps := map[string]interface{}{}
	addDiskLocationArgs(setUuidProps, "", "")
	setUuidProps["uuid"] = map[string]interface{}{"type": "string", "description": "UUID to assign to the virtual disk, e.g. \"12345678-1234-1234-1234-123456789abc\"."}
	setUuidProps["confirm"] = confirmArg
	r.registerDestructive("vmware_virtual_disk_set_uuid",
		"Set a virtual disk's UUID. CAUTION when testing against vcsim: this project's simulator backend accepts and reports success unconditionally without actually persisting the uuid — vmware_virtual_disk_query_uuid afterward will NOT reflect it (see this file's top doc comment, deviation 7). Expected to round-trip correctly against a real vCenter/ESXi.",
		tier2,
		map[string]interface{}{"type": "object", "properties": setUuidProps, "required": []interface{}{"datastore", "path", "datacenter", "uuid", "confirm"}},
		Tool{Handler: handleVirtualDiskSetUuid},
	)

	// --- ShrinkVirtualDisk (Tier 2) ----------------------------------------

	shrinkProps := map[string]interface{}{}
	addDiskLocationArgs(shrinkProps, "", "")
	shrinkProps["copy"] = map[string]interface{}{"type": "boolean", "description": "Optional: perform a copy-shrink (rewrite into a fresh file) instead of an in-place shrink. Omit to use vSphere's default (in-place)."}
	shrinkProps["confirm"] = confirmArg
	r.registerDestructive("vmware_virtual_disk_shrink",
		"Shrink a virtual disk by reclaiming unused blocks (does not change its logical capacity — see vmware_virtual_disk_extend for that). NOTE: not simulated by this project's test harness (vcsim has no ShrinkVirtualDisk handler) — exercise carefully against a real vCenter/ESXi first.",
		tier2,
		map[string]interface{}{"type": "object", "properties": shrinkProps, "required": []interface{}{"datastore", "path", "datacenter", "confirm"}},
		Tool{Handler: handleVirtualDiskShrink},
	)

	// --- DeleteVirtualDisk (Tier 1) ----------------------------------------

	deleteProps := map[string]interface{}{}
	addDiskLocationArgs(deleteProps, "", "")
	deleteProps["confirm"] = confirmArg
	r.registerDestructive("vmware_virtual_disk_delete_virtual_disk",
		"Delete a virtual disk (vmdk) file from a datastore. Irreversible. Classic VirtualDiskManager-scoped variant — requires an explicit datacenter argument. vmware_datastore_file_delete_virtual_disk wraps the exact same underlying operation with an auto-derived datacenter plus optional force/markDiskAsDeletable — see this file's top doc comment for the full relationship. Use whichever matches the API surface you already have a handle for.",
		tier1,
		map[string]interface{}{"type": "object", "properties": deleteProps, "required": []interface{}{"datastore", "path", "datacenter", "confirm"}},
		Tool{Handler: handleVirtualDiskDeleteVirtualDisk},
	)

	// --- QueryVirtualDiskInfo (read-only) -----------------------------------

	queryInfoProps := map[string]interface{}{}
	addDiskLocationArgs(queryInfoProps, "", "")
	queryInfoProps["include_parents"] = map[string]interface{}{"type": "boolean", "description": "Also include info for the disk's parent chain (snapshot deltas), if any. Default false."}
	r.register("vmware_virtual_disk_query_info",
		"Get information (disk type, capacity, parent chain) about a virtual disk.",
		map[string]interface{}{"type": "object", "properties": queryInfoProps, "required": []interface{}{"datastore", "path", "datacenter"}},
		Tool{Handler: handleVirtualDiskQueryInfo},
	)

	// --- QueryVirtualDiskUuid (read-only) -----------------------------------

	queryUuidProps := map[string]interface{}{}
	addDiskLocationArgs(queryUuidProps, "", "")
	r.register("vmware_virtual_disk_query_uuid",
		"Get a virtual disk's UUID.",
		map[string]interface{}{"type": "object", "properties": queryUuidProps, "required": []interface{}{"datastore", "path", "datacenter"}},
		Tool{Handler: handleVirtualDiskQueryUuid},
	)
}

// addDiskLocationArgs writes the "<prefix>datastore"/"<prefix>path"/
// "<prefix>datacenter" trio of schema properties into props — the disk
// location argument shape every tool in this file needs (once for a single
// disk, twice for the source/dest pair of Copy/Move, twice differently
// named for the parent/child pair of CreateChildDisk). what is a short
// human label used in the generated description ("Source", "Destination",
// "Parent", "Child (new)", or "" for the unprefixed single-disk case).
func addDiskLocationArgs(props map[string]interface{}, prefix, what string) {
	label := what
	if label != "" {
		label += " "
	}
	props[prefix+"datastore"] = map[string]interface{}{
		"type":        "string",
		"description": fmt.Sprintf(`%sdatastore name/pattern (e.g. "LocalDS_0") as returned by vmware_list_datastores. Must resolve to exactly one datastore.`, label),
	}
	props[prefix+"path"] = map[string]interface{}{
		"type":        "string",
		"description": fmt.Sprintf(`Path to the %sdisk's .vmdk file relative to the datastore root, e.g. "disks/disk1.vmdk". Combined with %sdatastore to build the vSphere-internal "[datastore] path" datastore-path string VirtualDiskManager expects — no need to hand-format that bracket notation yourself.`, label, prefix+"datastore"),
	}
	props[prefix+"datacenter"] = map[string]interface{}{
		"type":        "string",
		"description": fmt.Sprintf(`%sdatacenter name/pattern (e.g. "ha-datacenter") as returned by vmware_list_datacenters. Must resolve to exactly one datacenter — required by VirtualDiskManager's SOAP API even though %sdatastore already narrows the search (see this file's top doc comment, deviation 2).`, label, prefix+"datastore"),
	}
}

// virtualDiskManager builds a fresh object.VirtualDiskManager bound to this
// connection's *vim25.Client — same client.Client.Client field path
// datastore.go's object.NewFileManager(client.Client.Client) and
// generated_vm_provisioning.go's requireProvisioningChecker use (see
// generated_vm_provisioning.go's doc comment for the full field-path
// explanation).
func virtualDiskManager(client *vmware.Client) *object.VirtualDiskManager {
	return object.NewVirtualDiskManager(client.Client.Client)
}

// resolveDiskLocation resolves the "<prefix>datastore"/"<prefix>path"/
// "<prefix>datacenter" argument trio (written by addDiskLocationArgs) into
// the datastore-path "name" string and *object.Datacenter the real
// VirtualDiskManager API wants, plus the resolved *object.Datastore itself
// (needed by callers that also want to best-effort-create the disk's parent
// folder — see ensureDiskDir). See this file's top doc comment, deviation
// 1, for why this 3-argument decomposition exists instead of a single raw
// "[datastore] path" string argument.
func resolveDiskLocation(ctx context.Context, client *vmware.Client, args map[string]interface{}, prefix string) (string, *object.Datastore, *object.Datacenter, error) {
	dsName, _ := args[prefix+"datastore"].(string)
	diskPath, _ := args[prefix+"path"].(string)
	dcName, _ := args[prefix+"datacenter"].(string)

	if dsName == "" {
		return "", nil, nil, fmt.Errorf("%sdatastore is required", prefix)
	}
	if diskPath == "" {
		return "", nil, nil, fmt.Errorf("%spath is required", prefix)
	}
	if dcName == "" {
		return "", nil, nil, fmt.Errorf("%sdatacenter is required", prefix)
	}

	ds, err := resolveDatastore(ctx, client, dsName)
	if err != nil {
		return "", nil, nil, err
	}
	dc, err := client.Finder.Datacenter(ctx, dcName)
	if err != nil {
		return "", nil, nil, fmt.Errorf("failed to resolve datacenter %q: %w", dcName, err)
	}

	return ds.Path(diskPath), ds, dc, nil
}

// ensureDiskDir best-effort creates diskPath's parent folder on ds (and any
// missing parents) before a Create-style call — the same "create the
// destination folder if missing, swallow the error, let the real call
// surface anything genuinely wrong" pattern datastore.go's
// handleDatastoreUploadFile uses, and (per this file's top doc comment,
// deviation 4) actually REQUIRED for vmware_virtual_disk_create to succeed
// against vcsim at all.
func ensureDiskDir(ctx context.Context, client *vmware.Client, ds *object.Datastore, dc *object.Datacenter, diskPath string) {
	dir := path.Dir(diskPath)
	if dir == "." || dir == "/" {
		return
	}
	fm := object.NewFileManager(client.Client.Client)
	_ = fm.MakeDirectory(ctx, ds.Path(dir), dc, true)
}

func handleVirtualDiskCopy(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	sourceName, _, sourceDC, err := resolveDiskLocation(ctx, client, args, "source_")
	if err != nil {
		return "", err
	}
	destName, destDs, destDC, err := resolveDiskLocation(ctx, client, args, "dest_")
	if err != nil {
		return "", err
	}

	if args["dest_spec"] == nil {
		return "", fmt.Errorf("dest_spec is required")
	}
	var spec types.FileBackedVirtualDiskSpec
	if err := decodeJSONArg(args["dest_spec"], &spec); err != nil {
		return "", fmt.Errorf("invalid dest_spec: %w", err)
	}
	force, _ := args["force"].(bool)

	destPath, _ := args["dest_path"].(string)
	ensureDiskDir(ctx, client, destDs, destDC, destPath)

	m := virtualDiskManager(client)
	task, err := m.CopyVirtualDisk(ctx, sourceName, sourceDC, destName, destDC, &spec, force)
	if err != nil {
		return "", fmt.Errorf("failed to copy virtual disk %s to %s: %w", sourceName, destName, err)
	}
	if err := waitForTask(ctx, task); err != nil {
		return "", fmt.Errorf("copy-virtual-disk task failed for %s -> %s: %w", sourceName, destName, err)
	}

	return marshalJSON(map[string]interface{}{"source": sourceName, "dest": destName, "result": "copied"})
}

func handleVirtualDiskCreateChild(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	parentName, _, parentDC, err := resolveDiskLocation(ctx, client, args, "parent_")
	if err != nil {
		return "", err
	}
	childName, childDs, childDC, err := resolveDiskLocation(ctx, client, args, "")
	if err != nil {
		return "", err
	}

	linked, _ := args["linked"].(bool)

	childPath, _ := args["path"].(string)
	ensureDiskDir(ctx, client, childDs, childDC, childPath)

	m := virtualDiskManager(client)
	task, err := m.CreateChildDisk(ctx, parentName, parentDC, childName, childDC, linked)
	if err != nil {
		return "", fmt.Errorf("failed to create child disk %s from parent %s: %w", childName, parentName, err)
	}
	if err := waitForTask(ctx, task); err != nil {
		return "", fmt.Errorf("create-child-disk task failed for %s: %w", childName, err)
	}

	return marshalJSON(map[string]interface{}{"parent": parentName, "disk": childName, "linked": linked, "result": "created"})
}

func handleVirtualDiskCreate(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	name, ds, dc, err := resolveDiskLocation(ctx, client, args, "")
	if err != nil {
		return "", err
	}

	if args["spec"] == nil {
		return "", fmt.Errorf("spec is required")
	}
	var spec types.FileBackedVirtualDiskSpec
	if err := decodeJSONArg(args["spec"], &spec); err != nil {
		return "", fmt.Errorf("invalid spec: %w", err)
	}

	diskPath, _ := args["path"].(string)
	ensureDiskDir(ctx, client, ds, dc, diskPath)

	m := virtualDiskManager(client)
	task, err := m.CreateVirtualDisk(ctx, name, dc, &spec)
	if err != nil {
		return "", fmt.Errorf("failed to create virtual disk %s: %w", name, err)
	}
	if err := waitForTask(ctx, task); err != nil {
		return "", fmt.Errorf("create-virtual-disk task failed for %s: %w", name, err)
	}

	return marshalJSON(map[string]interface{}{"disk": name, "result": "created"})
}

func handleVirtualDiskExtend(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	name, _, dc, err := resolveDiskLocation(ctx, client, args, "")
	if err != nil {
		return "", err
	}

	capRaw, ok := args["capacity_kb"]
	if !ok {
		return "", fmt.Errorf("capacity_kb is required")
	}
	capacityKb, err := toInt64(capRaw)
	if err != nil {
		return "", fmt.Errorf("invalid capacity_kb: %w", err)
	}

	var eagerZero *bool
	if v, ok := args["eager_zero"]; ok {
		b, ok := v.(bool)
		if !ok {
			return "", fmt.Errorf("eager_zero must be a boolean")
		}
		eagerZero = &b
	}

	m := virtualDiskManager(client)
	task, err := m.ExtendVirtualDisk(ctx, name, dc, capacityKb, eagerZero)
	if err != nil {
		return "", fmt.Errorf("failed to extend virtual disk %s: %w", name, err)
	}
	if err := waitForTask(ctx, task); err != nil {
		return "", fmt.Errorf("extend-virtual-disk task failed for %s: %w", name, err)
	}

	return marshalJSON(map[string]interface{}{"disk": name, "capacity_kb": capacityKb, "result": "extended"})
}

func handleVirtualDiskInflate(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	name, _, dc, err := resolveDiskLocation(ctx, client, args, "")
	if err != nil {
		return "", err
	}

	m := virtualDiskManager(client)
	task, err := m.InflateVirtualDisk(ctx, name, dc)
	if err != nil {
		return "", fmt.Errorf("failed to inflate virtual disk %s: %w", name, err)
	}
	if err := waitForTask(ctx, task); err != nil {
		return "", fmt.Errorf("inflate-virtual-disk task failed for %s: %w", name, err)
	}

	return marshalJSON(map[string]interface{}{"disk": name, "result": "inflated"})
}

func handleVirtualDiskMove(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	sourceName, _, sourceDC, err := resolveDiskLocation(ctx, client, args, "source_")
	if err != nil {
		return "", err
	}
	destName, _, destDC, err := resolveDiskLocation(ctx, client, args, "dest_")
	if err != nil {
		return "", err
	}
	force, _ := args["force"].(bool)

	m := virtualDiskManager(client)
	task, err := m.MoveVirtualDisk(ctx, sourceName, sourceDC, destName, destDC, force)
	if err != nil {
		return "", fmt.Errorf("failed to move virtual disk %s to %s: %w", sourceName, destName, err)
	}
	if err := waitForTask(ctx, task); err != nil {
		return "", fmt.Errorf("move-virtual-disk task failed for %s -> %s: %w", sourceName, destName, err)
	}

	return marshalJSON(map[string]interface{}{"source": sourceName, "dest": destName, "result": "moved"})
}

func handleVirtualDiskSetUuid(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	name, _, dc, err := resolveDiskLocation(ctx, client, args, "")
	if err != nil {
		return "", err
	}
	uuid, _ := args["uuid"].(string)
	if uuid == "" {
		return "", fmt.Errorf("uuid is required")
	}

	m := virtualDiskManager(client)
	if err := m.SetVirtualDiskUuid(ctx, name, dc, uuid); err != nil {
		return "", fmt.Errorf("failed to set uuid for virtual disk %s: %w", name, err)
	}

	return marshalJSON(map[string]interface{}{"disk": name, "uuid": uuid, "result": "uuid_set"})
}

func handleVirtualDiskShrink(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	name, _, dc, err := resolveDiskLocation(ctx, client, args, "")
	if err != nil {
		return "", err
	}

	var copyPtr *bool
	if v, ok := args["copy"]; ok {
		b, ok := v.(bool)
		if !ok {
			return "", fmt.Errorf("copy must be a boolean")
		}
		copyPtr = &b
	}

	m := virtualDiskManager(client)
	task, err := m.ShrinkVirtualDisk(ctx, name, dc, copyPtr)
	if err != nil {
		return "", fmt.Errorf("failed to shrink virtual disk %s: %w", name, err)
	}
	if err := waitForTask(ctx, task); err != nil {
		return "", fmt.Errorf("shrink-virtual-disk task failed for %s: %w", name, err)
	}

	return marshalJSON(map[string]interface{}{"disk": name, "result": "shrunk"})
}

func handleVirtualDiskDeleteVirtualDisk(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	name, _, dc, err := resolveDiskLocation(ctx, client, args, "")
	if err != nil {
		return "", err
	}

	m := virtualDiskManager(client)
	task, err := m.DeleteVirtualDisk(ctx, name, dc)
	if err != nil {
		return "", fmt.Errorf("failed to delete virtual disk %s: %w", name, err)
	}
	if err := waitForTask(ctx, task); err != nil {
		return "", fmt.Errorf("delete-virtual-disk task failed for %s: %w", name, err)
	}

	return marshalJSON(map[string]interface{}{"disk": name, "result": "deleted"})
}

func handleVirtualDiskQueryInfo(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	name, _, dc, err := resolveDiskLocation(ctx, client, args, "")
	if err != nil {
		return "", err
	}
	includeParents, _ := args["include_parents"].(bool)

	m := virtualDiskManager(client)
	info, err := m.QueryVirtualDiskInfo(ctx, name, dc, includeParents)
	if err != nil {
		return "", fmt.Errorf("failed to query virtual disk info for %s: %w", name, err)
	}

	return marshalJSON(map[string]interface{}{"disk": name, "count": len(info), "info": info})
}

func handleVirtualDiskQueryUuid(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	name, _, dc, err := resolveDiskLocation(ctx, client, args, "")
	if err != nil {
		return "", err
	}

	m := virtualDiskManager(client)
	uuid, err := m.QueryVirtualDiskUuid(ctx, name, dc)
	if err != nil {
		return "", fmt.Errorf("failed to query uuid for virtual disk %s: %w", name, err)
	}

	return marshalJSON(map[string]interface{}{"disk": name, "uuid": uuid})
}

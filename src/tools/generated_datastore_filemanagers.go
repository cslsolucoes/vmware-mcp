package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/vmware/govmomi/object"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// registerDatastoreFileManagerTools is the "file managers" slice of Fase 4 of
// the codegen plan
// (".workspace/plans/MCPVMWare2026-08-10-175300-plano-cobertura-completa-api-codegen.plan.md")
// — object.DatastoreFileManager (7 methods) + object.FileManager (4 methods),
// hand-transcribed from the real referencia/govmomi/object source following
// the datastore.go/generated_host_storage.go/generated_vm_lifecycle.go
// conventions.
//
// Curation deviations / findings from reading the real source (human review
// required — none of this was assumed from the brief):
//
//   - The brief's assumed constructor "object.NewDatastoreFileManager(?)"
//     does not exist. The real constructor
//     (referencia/govmomi/object/datastore_file_manager.go) is a method on
//     Datastore: `func (d Datastore) NewFileManager(dc *Datacenter, force
//     bool) *DatastoreFileManager` — called as `ds.NewFileManager(dc,
//     force)`, not a package-level function.
//
//   - The brief assumed the 7 DatastoreFileManager tools need "no explicit
//     Datacenter arg", but the real constructor requires a non-nil
//     *Datacenter (vCenter faults types.InvalidArgument on a nil one — see
//     simulator/file_manager.go's resolve()). Resolved without adding a
//     "datacenter" argument to these 7 tools by reading the leading path
//     segment off the Finder-resolved *object.Datastore's own
//     InventoryPath (e.g. "/DC0/datastore/ds1" -> "DC0") and looking that
//     name up via client.Finder.Datacenter — see datacenterForDatastore
//     below. This is correct even against a multi-datacenter vCenter
//     (verified by TestDatastoreFileManagerTools_MultiDatacenterVCenter,
//     which deliberately picks a same-named datastore from 2 different
//     datacenters and proves each operation lands in the right one — a
//     wrong-datacenter pick would fault types.InvalidDatastore server-side,
//     not silently succeed) because it is derived from the SPECIFIC
//     datastore the caller resolved, not a blind "default datacenter" guess.
//
//   - The 7 DatastoreFileManager-level methods (Copy, CopyFile, Move,
//     MoveFile, Delete, DeleteFile, DeleteVirtualDisk) all return a plain
//     `error`, NOT `(*object.Task, error)` — every one of them already calls
//     its internal m.wait(ctx, task) before returning (confirmed by reading
//     every method body). Their tool handlers below therefore do NOT call
//     this package's waitForTask a second time — there is no *object.Task
//     handle to wait on. Only the 3 Task-returning FileManager-level methods
//     (CopyDatastoreFile, MoveDatastoreFile, DeleteDatastoreFile) need the
//     explicit waitForTask call; MakeDirectory returns a plain `error` too
//     (no Task at all — verified from its real signature, not assumed from
//     its 3 siblings' shape).
//
//   - Real Copy vs CopyFile distinction (read from the source, not guessed):
//     CopyFile ALWAYS calls FileManager.CopyDatastoreFile — a generic
//     byte-for-byte file copy regardless of extension. Copy is a dispatcher:
//     it inspects the SOURCE path's extension and, only if it is ".vmdk",
//     calls VirtualDiskManager.CopyVirtualDisk instead (with a nil
//     conversion spec) — which also copies the matching "-flat.vmdk" backing
//     extent (see vdmNames in simulator/virtual_disk_manager.go), so the
//     resulting virtual disk stays usable. For any other extension Copy
//     behaves identically to CopyFile. Practical guidance baked into the
//     tool descriptions below: use vmware_datastore_file_copy for anything
//     that might be a virtual disk; vmware_datastore_file_copy_file is a
//     plain byte copy that will silently produce a broken/incomplete virtual
//     disk if pointed at a .vmdk descriptor (its backing extent is left
//     behind).
//
//   - Real Move vs MoveFile: the identical relationship — MoveFile always
//     FileManager.MoveDatastoreFile (plain rename); Move dispatches to
//     VirtualDiskManager.MoveVirtualDisk (renames descriptor + "-flat.vmdk"
//     together) only when the source ends in ".vmdk".
//
//   - Real Delete vs DeleteFile vs DeleteVirtualDisk: Delete inspects name's
//     extension — ".vmdk" dispatches to DeleteVirtualDisk, everything else
//     to DeleteFile. DeleteFile always calls FileManager.DeleteDatastoreFile
//     — a generic single-path delete with no virtual-disk awareness at all;
//     calling it directly on a .vmdk descriptor removes ONLY the descriptor
//     and leaves the "-flat.vmdk" extent behind as an orphan blob (confirmed
//     against simulator.VirtualDiskManager.DeleteVirtualDiskTask, which
//     loops over vdmNames(name) = [name-flat.vmdk, name] and deletes both —
//     DeleteFile/DeleteDatastoreFile has no equivalent loop). DeleteVirtualDisk
//     always calls VirtualDiskManager.DeleteVirtualDisk, which removes both
//     files, plus (when force:true) first best-effort clears the disk's
//     ddb.deletable=false flag by downloading, patching, and re-uploading the
//     descriptor (markDiskAsDeletable in the real source) before issuing the
//     delete task.
//
//   - Cross-reference with the sibling agent's classic-API tool
//     vmware_virtual_disk_delete_virtual_disk (VirtualDiskManager.DeleteVirtualDisk
//     exposed directly, with an explicit "datacenter" argument): reading
//     DatastoreFileManager.DeleteVirtualDisk's body confirms it is NOT a
//     different SOAP operation — it is a thin, datastore-scoped convenience
//     wrapper around that exact same VirtualDiskManager.DeleteVirtualDisk
//     call (methods.DeleteVirtualDisk_Task under the hood). The only real
//     differences: (1) vmware_datastore_file_delete_virtual_disk (this file)
//     resolves its Datacenter automatically from the "datastore" argument
//     instead of requiring a separate one, and (2) it adds the optional
//     force/markDiskAsDeletable pre-step the raw VirtualDiskManager-level
//     tool does not offer. Both tools are kept — see the task's cross-agent
//     note — and each one's description below says so explicitly so a
//     caller/reviewer can tell them apart at a glance.
//
//   - source/destination/name argument shape: passed straight through,
//     unparsed, to DatastoreFileManager.Path (which itself calls
//     DatastorePath.FromString). That means each accepts EITHER a path
//     relative to the tool's own "datastore" argument (e.g. "ISOs/old.iso")
//     OR a full bracketed datastore path (e.g. "[datastore2] ISOs/old.iso")
//     to reach a DIFFERENT datastore than "datastore" supplied. Important
//     caveat found by reading the constructor: only the DATASTORE segment
//     can differ this way. The DATACENTER used for both source and
//     destination is always the single Datacenter auto-resolved from the
//     tool's own "datastore" argument — DatastoreFileManager's
//     DatacenterTarget field is hard-set equal to Datacenter in
//     Datastore.NewFileManager and nothing in this constructor lets it be
//     overridden. So this bracketed-path trick only reaches another
//     datastore in the SAME datacenter; a genuinely cross-datacenter
//     copy/move needs the FileManager-level tools below instead, which take
//     independent source/destination datacenter arguments.
//
//   - FileManager (4 tools): constructor object.NewFileManager(client.Client.Client)
//     — the exact same call tools/datastore.go already makes internally, but
//     for a different purpose: there, only to best-effort auto-create a
//     missing parent folder before an ISO upload (errors swallowed). Here,
//     vmware_file_make_directory exposes MakeDirectory as its own standalone
//     operation (e.g. pre-create an empty folder without uploading anything)
//     and surfaces real errors instead of swallowing them — new value, not a
//     duplicate of datastore.go's internal usage. All 4 FileManager methods
//     require an explicit "datacenter" argument (resolved via
//     client.Finder.Datacenter(ctx, name)) because a bare "[datastore] path"
//     string alone — unlike a Finder-resolved *object.Datastore — carries no
//     InventoryPath to auto-derive one from.
//
//   - vcsim coverage: ALL 7 underlying SOAP methods used across both types
//     (CopyDatastoreFile_Task, MoveDatastoreFile_Task, DeleteDatastoreFile_Task,
//     MakeDirectory, CopyVirtualDisk_Task, MoveVirtualDisk_Task,
//     DeleteVirtualDisk_Task) are FULLY implemented server-side —
//     simulator.FileManager (referencia/govmomi/simulator/file_manager.go)
//     and simulator.VirtualDiskManager
//     (referencia/govmomi/simulator/virtual_disk_manager.go) both perform
//     real file I/O against the simulator's on-disk datastore backing —
//     confirmed by grepping the exact SOAP method/request-type names across
//     referencia/govmomi/simulator, not the receiver type names (the known
//     pitfall from a sibling group in this same plan). This domain has much
//     better vcsim coverage than Host Storage (Fase 3): every one of the 11
//     tools below is exercised in the test file against real simulated file
//     operations (upload a real file, copy/move/delete it, assert via
//     Datastore.Stat), not just a "reaches the server" smoke assertion.
//
//   - Tier assignments follow the brief exactly, no deviation: Tier 2
//     (disruptive but reversible — the source file still exists after a
//     copy, and a move/rename can be undone by moving back) for the 4
//     copy/move DatastoreFileManager tools plus the 2 FileManager copy/move
//     tools and vmware_file_make_directory; Tier 1 (irreversible) for the 3
//     delete DatastoreFileManager tools plus vmware_file_delete_datastore_file.
func registerDatastoreFileManagerTools(r *Registry) {
	registerDatastoreFileManagerScopedTools(r)
	registerFileManagerScopedTools(r)
}

// datacenterForDatastore resolves the Datacenter that owns ds by reading the
// leading path segment of its InventoryPath (e.g. "/DC0/datastore/ds1" ->
// "DC0") and looking it up via the Finder. ds must come from a Finder-based
// resolution (resolveDatastore/dcScopedPath) so InventoryPath is always
// populated — unlike the raw object.NewDatastore(c, ref) construction path
// documented in generated_host_storage.go's top comment (which needed
// find.InventoryPath as a workaround for an EMPTY InventoryPath), a
// Finder-resolved Datastore's InventoryPath is populated by the Finder
// itself. See this file's top doc comment for why this avoids adding a
// redundant "datacenter" argument to the 7 DatastoreFileManager-scoped tools.
func datacenterForDatastore(ctx context.Context, client *vmware.Client, ds *object.Datastore) (*object.Datacenter, error) {
	segs := strings.SplitN(strings.TrimPrefix(ds.InventoryPath, "/"), "/", 2)
	if len(segs) == 0 || segs[0] == "" {
		return nil, fmt.Errorf("could not determine the owning datacenter from datastore inventory path %q", ds.InventoryPath)
	}

	dc, err := client.Finder.Datacenter(ctx, segs[0])
	if err != nil {
		return nil, fmt.Errorf("failed to resolve datacenter %q owning datastore %s: %w", segs[0], ds.InventoryPath, err)
	}
	return dc, nil
}

// resolveDatastoreFileManager resolves the "datastore" and optional "force"
// arguments shared by all 7 DatastoreFileManager-scoped tools into a ready
// *object.DatastoreFileManager, plus the underlying *object.Datastore (for
// building result payloads/error messages with its InventoryPath).
func resolveDatastoreFileManager(ctx context.Context, client *vmware.Client, args map[string]interface{}) (*object.DatastoreFileManager, *object.Datastore, error) {
	dsName, _ := args["datastore"].(string)
	ds, err := resolveDatastore(ctx, client, dsName)
	if err != nil {
		return nil, nil, err
	}

	dc, err := datacenterForDatastore(ctx, client, ds)
	if err != nil {
		return nil, nil, err
	}

	force, _ := args["force"].(bool)
	return ds.NewFileManager(dc, force), ds, nil
}

func registerDatastoreFileManagerScopedTools(r *Registry) {
	datastoreArg := map[string]interface{}{
		"type":        "string",
		"description": `Datastore name/pattern (e.g. "datastore1") as returned by vmware_list_datastores. Must resolve to exactly one datastore — its owning Datacenter is derived automatically from it, no separate datacenter argument needed.`,
	}
	forceArg := map[string]interface{}{
		"type":        "boolean",
		"description": `Copy/move tools: overwrite an existing destination file instead of failing. vmware_datastore_file_delete_virtual_disk only: best-effort clear the virtual disk's ddb.deletable=false flag before deleting it. Default false in both cases.`,
	}
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}
	pathArgDesc := func(role string) map[string]interface{} {
		return map[string]interface{}{
			"type":        "string",
			"description": fmt.Sprintf(`Path to the %s file, relative to "datastore" (e.g. "ISOs/old.iso"), or a full bracketed datastore path (e.g. "[datastore2] ISOs/old.iso") to reach a DIFFERENT datastore than "datastore" — same datacenter only (see this file's top doc comment).`, role),
		}
	}
	sourceArg := pathArgDesc("source")
	destArg := pathArgDesc("destination")
	nameArg := pathArgDesc("target")

	copyMoveSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"datastore":   datastoreArg,
			"source":      sourceArg,
			"destination": destArg,
			"force":       forceArg,
			"confirm":     confirmArg,
		},
		"required": []interface{}{"datastore", "source", "destination", "confirm"},
	}
	deleteSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"datastore": datastoreArg,
			"name":      nameArg,
			"force":     forceArg,
			"confirm":   confirmArg,
		},
		"required": []interface{}{"datastore", "name", "confirm"},
	}

	r.registerDestructive("vmware_datastore_file_copy",
		`Copy a file on a datastore. If the source path ends in ".vmdk" this dispatches to the virtual-disk-aware copy (also copies the matching "-flat.vmdk" backing extent) instead of a plain byte copy — see vmware_datastore_file_copy_file for the always-plain-byte-copy variant, which would leave a .vmdk copy broken. Fails if destination already exists unless force:true.`,
		tier2, copyMoveSchema, Tool{Handler: handleDatastoreFileCopy})

	r.registerDestructive("vmware_datastore_file_copy_file",
		`Copy a file on a datastore as a plain byte-for-byte copy, even if the source is a ".vmdk" descriptor — in that case the "-flat.vmdk" backing extent is NOT copied, producing an unusable/incomplete virtual disk at the destination. Use vmware_datastore_file_copy instead for anything that might be a virtual disk. Fails if destination already exists unless force:true.`,
		tier2, copyMoveSchema, Tool{Handler: handleDatastoreFileCopyFile})

	r.registerDestructive("vmware_datastore_file_move",
		`Move/rename a file on a datastore. If the source path ends in ".vmdk" this dispatches to the virtual-disk-aware move (also moves the matching "-flat.vmdk" backing extent) instead of a plain rename — see vmware_datastore_file_move_file for the always-plain-rename variant, which would leave a .vmdk move broken. Fails if destination already exists unless force:true.`,
		tier2, copyMoveSchema, Tool{Handler: handleDatastoreFileMove})

	r.registerDestructive("vmware_datastore_file_move_file",
		`Move/rename a file on a datastore as a plain rename, even if the source is a ".vmdk" descriptor — in that case the "-flat.vmdk" backing extent is NOT moved, producing an unusable/incomplete virtual disk at the destination. Use vmware_datastore_file_move instead for anything that might be a virtual disk. Fails if destination already exists unless force:true.`,
		tier2, copyMoveSchema, Tool{Handler: handleDatastoreFileMoveFile})

	r.registerDestructive("vmware_datastore_file_delete",
		`Delete a file or folder on a datastore. If name ends in ".vmdk" this dispatches to the virtual-disk-aware delete (also deletes the matching "-flat.vmdk" backing extent) instead of a plain single-file delete — see vmware_datastore_file_delete_file for the always-single-file variant, which would orphan a .vmdk's backing extent. Irreversible.`,
		tier1, deleteSchema, Tool{Handler: handleDatastoreFileDelete})

	r.registerDestructive("vmware_datastore_file_delete_file",
		`Delete exactly one file or folder on a datastore, even if it is a ".vmdk" descriptor — in that case its "-flat.vmdk" backing extent is left behind as an orphan. Use vmware_datastore_file_delete_virtual_disk instead to remove both files of a virtual disk together. Irreversible.`,
		tier1, deleteSchema, Tool{Handler: handleDatastoreFileDeleteFile})

	r.registerDestructive("vmware_datastore_file_delete_virtual_disk",
		`Delete a virtual disk (".vmdk" descriptor + its "-flat.vmdk" backing extent) on a datastore, with the Datacenter derived automatically from "datastore" — no separate datacenter argument needed. NOTE: this wraps the SAME underlying VirtualDiskManager.DeleteVirtualDisk SOAP call as vmware_virtual_disk_delete_virtual_disk (the classic/older API, which instead requires an explicit "datacenter" argument and a full datastore-scoped name) — both delete the same kind of object; pick whichever argument shape is more convenient. force:true also best-effort clears the disk's ddb.deletable=false flag before deleting. Irreversible.`,
		tier1, deleteSchema, Tool{Handler: handleDatastoreFileDeleteVirtualDisk})
}

func handleDatastoreFileCopy(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	return runDatastoreFileCopyMove(ctx, client, args, "copy", "copied", func(dfm *object.DatastoreFileManager, src, dst string) error {
		return dfm.Copy(ctx, src, dst)
	})
}

func handleDatastoreFileCopyFile(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	return runDatastoreFileCopyMove(ctx, client, args, "copy", "copied", func(dfm *object.DatastoreFileManager, src, dst string) error {
		return dfm.CopyFile(ctx, src, dst)
	})
}

func handleDatastoreFileMove(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	return runDatastoreFileCopyMove(ctx, client, args, "move", "moved", func(dfm *object.DatastoreFileManager, src, dst string) error {
		return dfm.Move(ctx, src, dst)
	})
}

func handleDatastoreFileMoveFile(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	return runDatastoreFileCopyMove(ctx, client, args, "move", "moved", func(dfm *object.DatastoreFileManager, src, dst string) error {
		return dfm.MoveFile(ctx, src, dst)
	})
}

// runDatastoreFileCopyMove is shared by the 4 copy/move DatastoreFileManager
// tool handlers above — they differ only in which DatastoreFileManager
// method they call and the present/past-tense verb used in messages.
func runDatastoreFileCopyMove(ctx context.Context, client *vmware.Client, args map[string]interface{}, presentVerb, pastVerb string, op func(dfm *object.DatastoreFileManager, src, dst string) error) (string, error) {
	dfm, ds, err := resolveDatastoreFileManager(ctx, client, args)
	if err != nil {
		return "", err
	}

	src, _ := args["source"].(string)
	if src == "" {
		return "", fmt.Errorf("source is required")
	}
	dst, _ := args["destination"].(string)
	if dst == "" {
		return "", fmt.Errorf("destination is required")
	}

	if err := op(dfm, src, dst); err != nil {
		return "", fmt.Errorf("failed to %s %s to %s on datastore %s: %w", presentVerb, src, dst, ds.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{
		"datastore":   ds.InventoryPath,
		"source":      src,
		"destination": dst,
		"result":      pastVerb,
	})
}

func handleDatastoreFileDelete(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	return runDatastoreFileDelete(ctx, client, args, func(dfm *object.DatastoreFileManager, name string) error {
		return dfm.Delete(ctx, name)
	})
}

func handleDatastoreFileDeleteFile(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	return runDatastoreFileDelete(ctx, client, args, func(dfm *object.DatastoreFileManager, name string) error {
		return dfm.DeleteFile(ctx, name)
	})
}

func handleDatastoreFileDeleteVirtualDisk(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	return runDatastoreFileDelete(ctx, client, args, func(dfm *object.DatastoreFileManager, name string) error {
		return dfm.DeleteVirtualDisk(ctx, name)
	})
}

// runDatastoreFileDelete is shared by the 3 delete DatastoreFileManager tool
// handlers above — they differ only in which DatastoreFileManager method
// they call.
func runDatastoreFileDelete(ctx context.Context, client *vmware.Client, args map[string]interface{}, op func(dfm *object.DatastoreFileManager, name string) error) (string, error) {
	dfm, ds, err := resolveDatastoreFileManager(ctx, client, args)
	if err != nil {
		return "", err
	}

	name, _ := args["name"].(string)
	if name == "" {
		return "", fmt.Errorf("name is required")
	}

	if err := op(dfm, name); err != nil {
		return "", fmt.Errorf("failed to delete %s on datastore %s: %w", name, ds.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{
		"datastore": ds.InventoryPath,
		"name":      name,
		"result":    "deleted",
	})
}

func registerFileManagerScopedTools(r *Registry) {
	dcArg := map[string]interface{}{
		"type":        "string",
		"description": `Datacenter name/pattern (e.g. "DC0") as returned by vmware_list_datacenters. Must resolve to exactly one datacenter.`,
	}
	pathArg := map[string]interface{}{
		"type":        "string",
		"description": `Full bracketed datastore path, e.g. "[datastore1] ISOs/old.iso" — required here (unlike the vmware_datastore_file_* tools above) because this tool has no "datastore" argument of its own to auto-derive a Datacenter from.`,
	}
	forceArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Overwrite an existing destination file instead of failing. Default false.",
	}
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}

	copyMoveSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"source_path":            pathArg,
			"source_datacenter":      dcArg,
			"destination_path":       pathArg,
			"destination_datacenter": dcArg,
			"force":                  forceArg,
			"confirm":                confirmArg,
		},
		"required": []interface{}{"source_path", "source_datacenter", "destination_path", "destination_datacenter", "confirm"},
	}

	r.registerDestructive("vmware_file_copy_datastore_file",
		`Copy a file between datastore paths, each independently scoped to its own Datacenter — unlike vmware_datastore_file_copy_file (which derives a single Datacenter from one "datastore" argument), this supports a genuinely cross-datacenter copy. Path arguments must be full bracketed datastore paths, e.g. "[datastore1] ISOs/old.iso". Fails if destination already exists unless force:true.`,
		tier2, copyMoveSchema, Tool{Handler: handleFileCopyDatastoreFile})

	r.registerDestructive("vmware_file_move_datastore_file",
		`Move a file between datastore paths, each independently scoped to its own Datacenter — unlike vmware_datastore_file_move_file, this supports a genuinely cross-datacenter move. Path arguments must be full bracketed datastore paths, e.g. "[datastore1] ISOs/old.iso". Fails if destination already exists unless force:true.`,
		tier2, copyMoveSchema, Tool{Handler: handleFileMoveDatastoreFile})

	r.registerDestructive("vmware_file_make_directory",
		`Create a folder at a datastore path. This is the SAME underlying FileManager.MakeDirectory call tools/datastore.go already uses internally inside vmware_datastore_upload_file (there, best-effort and with errors swallowed, only to auto-create a missing parent folder before an upload) — this tool exposes it directly as its own operation (e.g. pre-create an empty folder without uploading anything) and surfaces real errors instead of swallowing them.`,
		tier2, map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path":                      pathArg,
				"datacenter":                dcArg,
				"create_parent_directories": map[string]interface{}{"type": "boolean", "description": "Create any missing parent folders too. Default false — fails if the immediate parent folder doesn't already exist."},
				"confirm":                   confirmArg,
			},
			"required": []interface{}{"path", "datacenter", "confirm"},
		}, Tool{Handler: handleFileMakeDirectory})

	r.registerDestructive("vmware_file_delete_datastore_file",
		`Delete exactly one file or folder at a datastore path, scoped to an explicit Datacenter argument — unlike vmware_datastore_file_delete_file, which derives its Datacenter from a "datastore" argument. Path argument must be a full bracketed datastore path, e.g. "[datastore1] ISOs/old.iso". Irreversible.`,
		tier1, map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path":       pathArg,
				"datacenter": dcArg,
				"confirm":    confirmArg,
			},
			"required": []interface{}{"path", "datacenter", "confirm"},
		}, Tool{Handler: handleFileDeleteDatastoreFile})
}

func handleFileCopyDatastoreFile(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	return runFileCopyMoveDatastoreFile(ctx, client, args, "copy", "copied", func(fm *object.FileManager, src string, srcDC *object.Datacenter, dst string, dstDC *object.Datacenter, force bool) (*object.Task, error) {
		return fm.CopyDatastoreFile(ctx, src, srcDC, dst, dstDC, force)
	})
}

func handleFileMoveDatastoreFile(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	return runFileCopyMoveDatastoreFile(ctx, client, args, "move", "moved", func(fm *object.FileManager, src string, srcDC *object.Datacenter, dst string, dstDC *object.Datacenter, force bool) (*object.Task, error) {
		return fm.MoveDatastoreFile(ctx, src, srcDC, dst, dstDC, force)
	})
}

// runFileCopyMoveDatastoreFile is shared by the 2 copy/move FileManager tool
// handlers above — they differ only in which FileManager method they call
// and the present/past-tense verb used in messages.
func runFileCopyMoveDatastoreFile(ctx context.Context, client *vmware.Client, args map[string]interface{}, presentVerb, pastVerb string, op func(fm *object.FileManager, src string, srcDC *object.Datacenter, dst string, dstDC *object.Datacenter, force bool) (*object.Task, error)) (string, error) {
	srcPath, _ := args["source_path"].(string)
	if srcPath == "" {
		return "", fmt.Errorf("source_path is required")
	}
	dstPath, _ := args["destination_path"].(string)
	if dstPath == "" {
		return "", fmt.Errorf("destination_path is required")
	}

	srcDCName, _ := args["source_datacenter"].(string)
	if srcDCName == "" {
		return "", fmt.Errorf("source_datacenter is required")
	}
	srcDC, err := client.Finder.Datacenter(ctx, srcDCName)
	if err != nil {
		return "", fmt.Errorf("failed to resolve source_datacenter %q: %w", srcDCName, err)
	}

	dstDCName, _ := args["destination_datacenter"].(string)
	if dstDCName == "" {
		return "", fmt.Errorf("destination_datacenter is required")
	}
	dstDC, err := client.Finder.Datacenter(ctx, dstDCName)
	if err != nil {
		return "", fmt.Errorf("failed to resolve destination_datacenter %q: %w", dstDCName, err)
	}

	force, _ := args["force"].(bool)

	fm := object.NewFileManager(client.Client.Client)
	task, err := op(fm, srcPath, srcDC, dstPath, dstDC, force)
	if err != nil {
		return "", fmt.Errorf("failed to start %s of %s to %s: %w", presentVerb, srcPath, dstPath, err)
	}
	if err := waitForTask(ctx, task); err != nil {
		return "", fmt.Errorf("%s-datastore-file task failed for %s -> %s: %w", presentVerb, srcPath, dstPath, err)
	}

	return marshalJSON(map[string]interface{}{
		"source_path":      srcPath,
		"destination_path": dstPath,
		"result":           pastVerb,
	})
}

func handleFileMakeDirectory(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	p, _ := args["path"].(string)
	if p == "" {
		return "", fmt.Errorf("path is required")
	}

	dcName, _ := args["datacenter"].(string)
	if dcName == "" {
		return "", fmt.Errorf("datacenter is required")
	}
	dc, err := client.Finder.Datacenter(ctx, dcName)
	if err != nil {
		return "", fmt.Errorf("failed to resolve datacenter %q: %w", dcName, err)
	}

	createParents, _ := args["create_parent_directories"].(bool)

	fm := object.NewFileManager(client.Client.Client)
	if err := fm.MakeDirectory(ctx, p, dc, createParents); err != nil {
		return "", fmt.Errorf("failed to create directory %s: %w", p, err)
	}

	return marshalJSON(map[string]interface{}{
		"path":       p,
		"datacenter": dcName,
		"result":     "created",
	})
}

func handleFileDeleteDatastoreFile(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	p, _ := args["path"].(string)
	if p == "" {
		return "", fmt.Errorf("path is required")
	}

	dcName, _ := args["datacenter"].(string)
	if dcName == "" {
		return "", fmt.Errorf("datacenter is required")
	}
	dc, err := client.Finder.Datacenter(ctx, dcName)
	if err != nil {
		return "", fmt.Errorf("failed to resolve datacenter %q: %w", dcName, err)
	}

	fm := object.NewFileManager(client.Client.Client)
	task, err := fm.DeleteDatastoreFile(ctx, p, dc)
	if err != nil {
		return "", fmt.Errorf("failed to start delete of %s: %w", p, err)
	}
	if err := waitForTask(ctx, task); err != nil {
		return "", fmt.Errorf("delete-datastore-file task failed for %s: %w", p, err)
	}

	return marshalJSON(map[string]interface{}{
		"path":       p,
		"datacenter": dcName,
		"result":     "deleted",
	})
}

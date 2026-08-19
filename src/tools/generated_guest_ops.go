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

// registerGuestOpsTools adds Guest Operations tools — executing programs and
// managing files INSIDE a VM's guest OS via VMware Tools — closing the gap
// this project's earlier fases left open ("sem GuestOperations" in the guest
// OS itself, as opposed to configuring the VM's virtual hardware/config from
// outside it). Every tool here requires VMware Tools already running in the
// guest and a valid guest OS account (guest_username/guest_password) — this
// server has no way to verify either before the round trip; a missing/dead
// Tools installation surfaces as a GuestOperationsUnavailable fault, bad
// credentials as InvalidGuestLogin, both from the real vCenter/ESXi/vcsim
// handler, not from argument validation in this file.
//
// Managed objects / MoRefs involved (confirmed by reading the real source,
// not assumed):
//
//   - GuestOperationsManager is a direct ServiceContent field
//     (client.Client.ServiceContent.GuestOperationsManager,
//     *types.ManagedObjectReference) — present on both a standalone ESXi
//     host and vCenter (confirmed against
//     referencia/govmomi/simulator/esx/service_content.go and
//     vpx/service_content.go, both of which populate it).
//
//   - Its two children — GuestFileManager and GuestProcessManager — are its
//     "fileManager"/"processManager" properties (types.go's
//     GuestOperationsManager struct, mo.GuestOperationsManager). There is no
//     object.* wrapper for either (confirmed: no
//     referencia/govmomi/object/guest*.go file exists at all, unlike
//     HostConfigManager's StorageSystem/DatastoreSystem/NetworkSystem
//     accessors) — every handler in this file therefore dials the raw vim25
//     SOAP method directly, exactly like generated_host_iscsi_portbinding.go's
//     IscsiManager methods: methods.Xxx(ctx, client.Client.Client,
//     &types.Xxx{This: ref, ...}). The sub-manager MoRefs are read the same
//     way generated_license.go's licenseAssignmentManagerRef reads
//     LicenseAssignmentManager off LicenseManager: one
//     object.NewCommon(...).Properties call against mo.GuestOperationsManager
//     — see guestOperationsManagerRefs below.
//
// Tier classification: List*/Read*/InitiateFileTransferFromGuest/
// CreateTemporary* are read-only (r.register, no gate/confirm) — none of
// them mutate anything (a temp file/dir is trivial scratch space, not a
// meaningful side effect worth gating). Every mutation
// (Make/Delete/Move/Change/Start/Terminate/InitiateFileTransferToGuest) goes
// through registerDestructive: Delete* is tier1 (irreversible — a deleted
// guest file/directory cannot be recovered by this server), everything else
// is tier2 (disruptive but reversible/re-doable — a moved/renamed file can be
// moved back, a running program can be killed, a directory can be removed
// again).
//
// ChangeFileAttributesInGuest/InitiateFileTransferToGuest only build
// *types.GuestPosixFileAttributes (owner_id/group_id/permissions), not
// *types.GuestWindowsFileAttributes (hidden/read_only/create_time) — matches
// what referencia/govmomi/simulator/guest_operations_manager.go's
// ChangeFileAttributesInGuest actually type-asserts against
// (`attr, ok := req.FileAttributes.(*types.GuestPosixFileAttributes)`), and
// POSIX ownership/permission semantics are the common case; Windows-guest
// hidden/read-only attributes are out of scope for this batch.
//
// vcsim coverage (confirmed by reading
// referencia/govmomi/simulator/guest_operations_manager.go and
// referencia/govmomi/simulator/container_virtual_machine.go, not assumed —
// see generated_guest_ops_test.go's assertGuestOpsWiringReachesServer for the
// full breakdown): vcsim implements every one of these 15 methods except
// ReadEnvironmentVariableInGuest (no handler exists for it at all — a plain
// vcsim VM faults MethodNotFound). Its GuestFileManager/GuestProcessManager
// implementation is built around an OPTIONAL Docker-container backing per VM
// (simVM, wired only when the VM's ExtraConfig carries the "RUN.container"
// key) — every simulator.ESX()/simulator.VPX() default-model VM in this
// project's tests has no such backing, so vm.svm is nil. Against that nil
// backing: every GuestFileManager mutation's vm.svm.exec(...) short-circuits
// on the nil receiver to ("", nil) — no fault — so
// Make/Delete/Move/ChangeFileAttributesInGuest and
// CreateTemporary{File,Directory}InGuest/ListFilesInGuest all genuinely
// SUCCEED as silent no-ops against vcsim, not just "reach" it.
// ListProcessesInGuest never touches svm at all (its process.Manager is a
// separate, always-initialized singleton) and also succeeds trivially
// (empty list). InitiateFileTransferFromGuest/ToGuest DO check
// vm.svm.prepareGuestOperation and return a clean GuestOperationsUnavailable
// fault when nil. TerminateProcessInGuest cleanly faults
// GuestProcessNotFound (nothing was ever really started). StartProgramInGuest
// is the one genuine vcsim bug found here: after prepareGuestOperation
// returns a non-nil fault for a nil svm, the handler does not return early —
// it falls through and unconditionally dereferences vm.svm.c.id, a nil
// pointer field access (not a nil-checked method call), which panics
// server-side; Go's net/http server recovers that per-request panic and
// closes the connection, so the client sees a network-level error, still a
// clean non-nil error from this server's point of view, just not a SOAP
// fault. None of this is behaviorally validated end-to-end (a real guest OS
// executing a real command) by this test suite — only real vSphere/ESXi with
// VMware Tools running in an actual guest can do that.
func registerGuestOpsTools(r *Registry) {
	vmArg := map[string]interface{}{
		"type":        "string",
		"description": `VM identifier: a name/pattern (e.g. "cac-WN02") or a full inventory path (e.g. "/ha-datacenter/vm/cac-WN02") as returned by vmware_list_vms. Must resolve to exactly one VM. VMware Tools must be running in its guest OS for any guest-operations tool to succeed.`,
	}
	guestUsernameArg := map[string]interface{}{
		"type":        "string",
		"description": "Username of a valid account inside the guest OS to authenticate the guest operation as. Not this server's vCenter/ESXi credentials — a separate, guest-OS-local (or domain, if the guest is joined) account.",
	}
	guestPasswordArg := map[string]interface{}{
		"type":        "string",
		"description": "Password for guest_username.",
	}
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}
	ownerIDArg := map[string]interface{}{
		"type":        "integer",
		"description": "POSIX owner (user) ID to set on the guest file. Omit to leave unchanged. POSIX guests only.",
	}
	groupIDArg := map[string]interface{}{
		"type":        "integer",
		"description": "POSIX group ID to set on the guest file. Omit to leave unchanged. POSIX guests only.",
	}
	permissionsArg := map[string]interface{}{
		"type":        "integer",
		"description": "POSIX file permissions in chmod(2) numeric form (e.g. 420 decimal = 0644 octal). Omit to leave unchanged. POSIX guests only.",
	}

	// --- Read-only -------------------------------------------------------

	r.register("vmware_guest_list_files",
		"List files/directories inside a VM's guest OS at a given path, with pagination and an optional perl-compatible regex filter. Requires VMware Tools running in the guest.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm":             vmArg,
				"guest_username": guestUsernameArg,
				"guest_password": guestPasswordArg,
				"file_path":      map[string]interface{}{"type": "string", "description": "Complete path to the directory or file to query, inside the guest."},
				"index":          map[string]interface{}{"type": "integer", "description": "Which result to start the list with. Default 0."},
				"max_results":    map[string]interface{}{"type": "integer", "description": "Maximum number of results to return. Default 50."},
				"match_pattern":  map[string]interface{}{"type": "string", "description": `Perl-compatible regex filter on file names. Default '.*' (everything) if omitted.`},
			},
			"required": []interface{}{"vm", "guest_username", "guest_password", "file_path"},
		},
		Tool{Handler: handleGuestListFiles},
	)

	r.register("vmware_guest_create_temp_file",
		"Create an empty temporary file inside a VM's guest OS and return its complete path. Requires VMware Tools running in the guest.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm":             vmArg,
				"guest_username": guestUsernameArg,
				"guest_password": guestPasswordArg,
				"prefix":         map[string]interface{}{"type": "string", "description": "Prefix for the generated file name. Optional."},
				"suffix":         map[string]interface{}{"type": "string", "description": "Suffix for the generated file name. Optional."},
				"directory_path": map[string]interface{}{"type": "string", "description": "Complete path to the directory in which to create the file, inside the guest. Omit for a guest-specific default location."},
			},
			"required": []interface{}{"vm", "guest_username", "guest_password"},
		},
		Tool{Handler: handleGuestCreateTempFile},
	)

	r.register("vmware_guest_create_temp_directory",
		"Create an empty temporary directory inside a VM's guest OS and return its complete path. Requires VMware Tools running in the guest.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm":             vmArg,
				"guest_username": guestUsernameArg,
				"guest_password": guestPasswordArg,
				"prefix":         map[string]interface{}{"type": "string", "description": "Prefix for the generated directory name. Optional."},
				"suffix":         map[string]interface{}{"type": "string", "description": "Suffix for the generated directory name. Optional."},
				"directory_path": map[string]interface{}{"type": "string", "description": "Complete path to the directory in which to create the new directory, inside the guest. Omit for a guest-specific default location."},
			},
			"required": []interface{}{"vm", "guest_username", "guest_password"},
		},
		Tool{Handler: handleGuestCreateTempDirectory},
	)

	r.register("vmware_guest_file_transfer_from",
		"Initiate downloading a file FROM a VM's guest OS: returns a one-time HTTP GET URL (valid ~10 minutes) and the file's size/attributes. This tool only initiates the transfer — performing the actual HTTP GET against the returned URL is the caller's responsibility, outside this MCP server. Requires VMware Tools running in the guest.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm":              vmArg,
				"guest_username":  guestUsernameArg,
				"guest_password":  guestPasswordArg,
				"guest_file_path": map[string]interface{}{"type": "string", "description": "Complete path, inside the guest, to the file to transfer out. Cannot be a directory or symbolic link."},
			},
			"required": []interface{}{"vm", "guest_username", "guest_password", "guest_file_path"},
		},
		Tool{Handler: handleGuestFileTransferFrom},
	)

	r.register("vmware_guest_list_processes",
		"List processes currently running (or recently completed, if started via vmware_guest_start_program and queried within 5 minutes) inside a VM's guest OS. Requires VMware Tools running in the guest.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm":             vmArg,
				"guest_username": guestUsernameArg,
				"guest_password": guestPasswordArg,
				"pids":           map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "integer"}, "description": "Only return information about these process IDs. Omit to return every process."},
			},
			"required": []interface{}{"vm", "guest_username", "guest_password"},
		},
		Tool{Handler: handleGuestListProcesses},
	)

	r.register("vmware_guest_read_environment_variable",
		"Read environment variables from inside a VM's guest OS, in the context of guest_username's session. Requires VMware Tools running in the guest.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm":             vmArg,
				"guest_username": guestUsernameArg,
				"guest_password": guestPasswordArg,
				"names":          map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": `Names of the variables to read (e.g. ["PATH", "HOME"]). Omit to return every environment variable.`},
			},
			"required": []interface{}{"vm", "guest_username", "guest_password"},
		},
		Tool{Handler: handleGuestReadEnvironmentVariable},
	)

	// --- Tier 2: disruptive but reversible --------------------------------

	r.registerDestructive("vmware_guest_make_directory",
		"Create a directory inside a VM's guest OS. Reversible via vmware_guest_delete_directory. Requires VMware Tools running in the guest.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm":                        vmArg,
				"guest_username":            guestUsernameArg,
				"guest_password":            guestPasswordArg,
				"directory_path":            map[string]interface{}{"type": "string", "description": "Complete path to the directory to create, inside the guest."},
				"create_parent_directories": map[string]interface{}{"type": "boolean", "description": "Create any missing parent directories too. Default false."},
				"confirm":                   confirmArg,
			},
			"required": []interface{}{"vm", "guest_username", "guest_password", "directory_path", "confirm"},
		},
		Tool{Handler: handleGuestMakeDirectory},
	)

	r.registerDestructive("vmware_guest_move_directory",
		"Move/rename a directory inside a VM's guest OS. Reversible by moving it back. Requires VMware Tools running in the guest.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm":                 vmArg,
				"guest_username":     guestUsernameArg,
				"guest_password":     guestPasswordArg,
				"src_directory_path": map[string]interface{}{"type": "string", "description": "Complete path, inside the guest, to the directory to move."},
				"dst_directory_path": map[string]interface{}{"type": "string", "description": "Complete path, inside the guest, to move it to (its new location/name). Cannot be an existing directory or file."},
				"confirm":            confirmArg,
			},
			"required": []interface{}{"vm", "guest_username", "guest_password", "src_directory_path", "dst_directory_path", "confirm"},
		},
		Tool{Handler: handleGuestMoveDirectory},
	)

	r.registerDestructive("vmware_guest_move_file",
		"Move/rename a file (or symbolic link) inside a VM's guest OS. Reversible by moving it back. Requires VMware Tools running in the guest.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm":             vmArg,
				"guest_username": guestUsernameArg,
				"guest_password": guestPasswordArg,
				"src_file_path":  map[string]interface{}{"type": "string", "description": "Complete path, inside the guest, to the original file/symbolic link to move."},
				"dst_file_path":  map[string]interface{}{"type": "string", "description": "Complete path, inside the guest, to rename it to. Cannot be an existing directory."},
				"overwrite":      map[string]interface{}{"type": "boolean", "description": "Clobber the destination file if it already exists. Default false."},
				"confirm":        confirmArg,
			},
			"required": []interface{}{"vm", "guest_username", "guest_password", "src_file_path", "dst_file_path", "confirm"},
		},
		Tool{Handler: handleGuestMoveFile},
	)

	r.registerDestructive("vmware_guest_change_file_attributes",
		"Change POSIX ownership/permission attributes of a file inside a VM's guest OS (owner_id/group_id/permissions — any not specified are left unchanged). POSIX guests only. Reversible by setting the previous values back. Requires VMware Tools running in the guest.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm":              vmArg,
				"guest_username":  guestUsernameArg,
				"guest_password":  guestPasswordArg,
				"guest_file_path": map[string]interface{}{"type": "string", "description": "Complete path, inside the guest, to the file to change attributes of. If it is a symbolic link, the attributes of the target file are changed."},
				"owner_id":        ownerIDArg,
				"group_id":        groupIDArg,
				"permissions":     permissionsArg,
				"confirm":         confirmArg,
			},
			"required": []interface{}{"vm", "guest_username", "guest_password", "guest_file_path", "confirm"},
		},
		Tool{Handler: handleGuestChangeFileAttributes},
	)

	r.registerDestructive("vmware_guest_file_transfer_to",
		"Initiate uploading a file TO a VM's guest OS: returns a one-time HTTP PUT URL. This tool only initiates the transfer — performing the actual HTTP PUT of the file content against the returned URL is the caller's responsibility, outside this MCP server. Requires VMware Tools running in the guest.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm":              vmArg,
				"guest_username":  guestUsernameArg,
				"guest_password":  guestPasswordArg,
				"guest_file_path": map[string]interface{}{"type": "string", "description": "Complete destination path, inside the guest, to create the file at. Cannot be a directory or symbolic link."},
				"file_size":       map[string]interface{}{"type": "integer", "description": "Size in bytes of the file that will be uploaded."},
				"overwrite":       map[string]interface{}{"type": "boolean", "description": "Clobber the destination file if it already exists. Default false."},
				"owner_id":        ownerIDArg,
				"group_id":        groupIDArg,
				"permissions":     permissionsArg,
				"confirm":         confirmArg,
			},
			"required": []interface{}{"vm", "guest_username", "guest_password", "guest_file_path", "file_size", "confirm"},
		},
		Tool{Handler: handleGuestFileTransferTo},
	)

	r.registerDestructive("vmware_guest_start_program",
		"Start a program inside a VM's guest OS and return its guest process ID. On Linux/Solaris guests the program is run via a shell (stdio redirection possible); on Windows guests, prefix with \"cmd /c\" for redirection. Reversible via vmware_guest_terminate_process. Requires VMware Tools running in the guest.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm":                vmArg,
				"guest_username":    guestUsernameArg,
				"guest_password":    guestPasswordArg,
				"program_path":      map[string]interface{}{"type": "string", "description": "Absolute path, inside the guest, to the program to start."},
				"arguments":         map[string]interface{}{"type": "string", "description": "Arguments to the program. Optional."},
				"working_directory": map[string]interface{}{"type": "string", "description": "Absolute path, inside the guest, of the working directory for the program. VMware recommends always setting this explicitly. Optional."},
				"env_variables":     map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": `Complete set of environment variables for the program, guest-OS notation (e.g. ["PATH=/usr/bin", "FOO=bar"]) — these REPLACE the default environment, they are not additions. Omit to use guest-dependent defaults.`},
				"confirm":           confirmArg,
			},
			"required": []interface{}{"vm", "guest_username", "guest_password", "program_path", "confirm"},
		},
		Tool{Handler: handleGuestStartProgram},
	)

	r.registerDestructive("vmware_guest_terminate_process",
		"Terminate a running process inside a VM's guest OS by its guest process ID (see vmware_guest_list_processes / the pid returned by vmware_guest_start_program). Requires VMware Tools running in the guest.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm":             vmArg,
				"guest_username": guestUsernameArg,
				"guest_password": guestPasswordArg,
				"pid":            map[string]interface{}{"type": "integer", "description": "Guest process ID to terminate."},
				"confirm":        confirmArg,
			},
			"required": []interface{}{"vm", "guest_username", "guest_password", "pid", "confirm"},
		},
		Tool{Handler: handleGuestTerminateProcess},
	)

	// --- Tier 1: irreversible ----------------------------------------------

	r.registerDestructive("vmware_guest_delete_file",
		"Delete a file (or symbolic link) inside a VM's guest OS. Irreversible — this server has no way to recover a deleted guest file. Requires VMware Tools running in the guest.",
		tier1,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm":             vmArg,
				"guest_username": guestUsernameArg,
				"guest_password": guestPasswordArg,
				"file_path":      map[string]interface{}{"type": "string", "description": "Complete path, inside the guest, to the file or symbolic link to delete."},
				"confirm":        confirmArg,
			},
			"required": []interface{}{"vm", "guest_username", "guest_password", "file_path", "confirm"},
		},
		Tool{Handler: handleGuestDeleteFile},
	)

	r.registerDestructive("vmware_guest_delete_directory",
		"Delete a directory inside a VM's guest OS (optionally recursive). Irreversible — this server has no way to recover a deleted guest directory. Requires VMware Tools running in the guest.",
		tier1,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm":             vmArg,
				"guest_username": guestUsernameArg,
				"guest_password": guestPasswordArg,
				"directory_path": map[string]interface{}{"type": "string", "description": "Complete path, inside the guest, to the directory to delete."},
				"recursive":      map[string]interface{}{"type": "boolean", "description": "Delete all subdirectories/files too. If false, the directory must already be empty. Default false."},
				"confirm":        confirmArg,
			},
			"required": []interface{}{"vm", "guest_username", "guest_password", "directory_path", "confirm"},
		},
		Tool{Handler: handleGuestDeleteDirectory},
	)
}

// guestOperationsManagerRefs resolves the GuestFileManager/GuestProcessManager
// MoRefs off ServiceContent.GuestOperationsManager — see this file's top doc
// comment for why this reads properties directly instead of going through an
// object.* wrapper (none exists). Nil-guards every step even though vcsim's
// GuestOperationsManager.init always populates both children — a real
// vCenter/ESXi is not guaranteed to.
func guestOperationsManagerRefs(ctx context.Context, client *vmware.Client) (fileMgr, procMgr types.ManagedObjectReference, err error) {
	goRef := client.Client.ServiceContent.GuestOperationsManager
	if goRef == nil {
		return types.ManagedObjectReference{}, types.ManagedObjectReference{}, fmt.Errorf("guest operations are not available on this connection (ServiceContent.GuestOperationsManager is nil)")
	}

	common := object.NewCommon(client.Client.Client, *goRef)
	var gom mo.GuestOperationsManager
	if err := common.Properties(ctx, *goRef, []string{"fileManager", "processManager"}, &gom); err != nil {
		return types.ManagedObjectReference{}, types.ManagedObjectReference{}, fmt.Errorf("failed to read guest operations manager: %w", err)
	}
	if gom.FileManager == nil {
		return types.ManagedObjectReference{}, types.ManagedObjectReference{}, fmt.Errorf("this connection's GuestOperationsManager does not expose a fileManager")
	}
	if gom.ProcessManager == nil {
		return types.ManagedObjectReference{}, types.ManagedObjectReference{}, fmt.Errorf("this connection's GuestOperationsManager does not expose a processManager")
	}
	return *gom.FileManager, *gom.ProcessManager, nil
}

// guestFileManager resolves both the target VM and the GuestFileManager MoRef
// in one step — the (vm, ref, error) shape mirrors
// generated_host_iscsi_portbinding.go's hostIscsiManager helper.
func guestFileManager(ctx context.Context, client *vmware.Client, args map[string]interface{}) (*object.VirtualMachine, types.ManagedObjectReference, error) {
	vm, err := resolveVM(ctx, client, args)
	if err != nil {
		return nil, types.ManagedObjectReference{}, err
	}
	fileMgr, _, err := guestOperationsManagerRefs(ctx, client)
	if err != nil {
		return nil, types.ManagedObjectReference{}, err
	}
	return vm, fileMgr, nil
}

// guestProcessManager is guestFileManager's counterpart for
// GuestProcessManager.
func guestProcessManager(ctx context.Context, client *vmware.Client, args map[string]interface{}) (*object.VirtualMachine, types.ManagedObjectReference, error) {
	vm, err := resolveVM(ctx, client, args)
	if err != nil {
		return nil, types.ManagedObjectReference{}, err
	}
	_, procMgr, err := guestOperationsManagerRefs(ctx, client)
	if err != nil {
		return nil, types.ManagedObjectReference{}, err
	}
	return vm, procMgr, nil
}

// guestAuth builds the NamePasswordAuthentication every guest operation
// requires (types.BaseGuestAuthentication) from the required guest_username/
// guest_password arguments.
func guestAuth(args map[string]interface{}) (types.BaseGuestAuthentication, error) {
	username, _ := args["guest_username"].(string)
	if username == "" {
		return nil, fmt.Errorf("guest_username is required")
	}
	password, _ := args["guest_password"].(string)
	if password == "" {
		return nil, fmt.Errorf("guest_password is required")
	}
	return &types.NamePasswordAuthentication{
		GuestAuthentication: types.GuestAuthentication{InteractiveSession: false},
		Username:            username,
		Password:            password,
	}, nil
}

// guestRequiredArg reads and validates a required string argument, with a
// consistent "<key> is required" error shared by every handler in this file.
func guestRequiredArg(args map[string]interface{}, key string) (string, error) {
	v, _ := args[key].(string)
	if v == "" {
		return "", fmt.Errorf("%s is required", key)
	}
	return v, nil
}

// guestPosixAttributesFromArgs builds a *types.GuestPosixFileAttributes from
// the optional owner_id/group_id/permissions arguments shared by
// vmware_guest_change_file_attributes and vmware_guest_file_transfer_to.
// Always returns a non-nil object (even with every field left unset/nil) —
// both methods' docs describe unset properties as "left unchanged"/"default
// value used," implying the FileAttributes object itself must still be
// present, just with unset fields. See this file's top doc comment for why
// only the POSIX variant is supported.
func guestPosixAttributesFromArgs(args map[string]interface{}) (*types.GuestPosixFileAttributes, error) {
	attr := &types.GuestPosixFileAttributes{}
	if v, ok := args["owner_id"]; ok {
		n, err := toInt32(v)
		if err != nil {
			return nil, fmt.Errorf("invalid owner_id: %w", err)
		}
		attr.OwnerId = &n
	}
	if v, ok := args["group_id"]; ok {
		n, err := toInt32(v)
		if err != nil {
			return nil, fmt.Errorf("invalid group_id: %w", err)
		}
		attr.GroupId = &n
	}
	if v, ok := args["permissions"]; ok {
		n, err := toInt64(v)
		if err != nil {
			return nil, fmt.Errorf("invalid permissions: %w", err)
		}
		attr.Permissions = n
	}
	return attr, nil
}

// guestPidsFromArgs reads the optional "pids" array argument (guest process
// IDs) into a []int64, or (nil, nil) if omitted — used by
// vmware_guest_list_processes.
func guestPidsFromArgs(args map[string]interface{}) ([]int64, error) {
	raw, ok := args["pids"]
	if !ok {
		return nil, nil
	}
	arr, ok := raw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("expected an array of numbers, got %T", raw)
	}
	out := make([]int64, 0, len(arr))
	for i, item := range arr {
		n, err := toInt64(item)
		if err != nil {
			return nil, fmt.Errorf("[%d]: %w", i, err)
		}
		out = append(out, n)
	}
	return out, nil
}

func handleGuestListFiles(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, ref, err := guestFileManager(ctx, client, args)
	if err != nil {
		return "", err
	}
	auth, err := guestAuth(args)
	if err != nil {
		return "", err
	}
	filePath, err := guestRequiredArg(args, "file_path")
	if err != nil {
		return "", err
	}

	req := &types.ListFilesInGuest{
		This:     ref,
		Vm:       vm.Reference(),
		Auth:     auth,
		FilePath: filePath,
	}
	if v, ok := args["index"]; ok {
		n, err := toInt32(v)
		if err != nil {
			return "", fmt.Errorf("invalid index: %w", err)
		}
		req.Index = n
	}
	if v, ok := args["max_results"]; ok {
		n, err := toInt32(v)
		if err != nil {
			return "", fmt.Errorf("invalid max_results: %w", err)
		}
		req.MaxResults = n
	}
	if v, ok := args["match_pattern"].(string); ok {
		req.MatchPattern = v
	}

	resp, err := methods.ListFilesInGuest(ctx, client.Client.Client, req)
	if err != nil {
		return "", fmt.Errorf("failed to list files at %s on %s: %w", filePath, vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{
		"vm":        vm.InventoryPath,
		"file_path": filePath,
		"remaining": resp.Returnval.Remaining,
		"count":     len(resp.Returnval.Files),
		"files":     resp.Returnval.Files,
	})
}

func handleGuestCreateTempFile(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, ref, err := guestFileManager(ctx, client, args)
	if err != nil {
		return "", err
	}
	auth, err := guestAuth(args)
	if err != nil {
		return "", err
	}
	prefix, _ := args["prefix"].(string)
	suffix, _ := args["suffix"].(string)
	directoryPath, _ := args["directory_path"].(string)

	resp, err := methods.CreateTemporaryFileInGuest(ctx, client.Client.Client, &types.CreateTemporaryFileInGuest{
		This:          ref,
		Vm:            vm.Reference(),
		Auth:          auth,
		Prefix:        prefix,
		Suffix:        suffix,
		DirectoryPath: directoryPath,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create a temporary file on %s: %w", vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"vm": vm.InventoryPath, "path": resp.Returnval})
}

func handleGuestCreateTempDirectory(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, ref, err := guestFileManager(ctx, client, args)
	if err != nil {
		return "", err
	}
	auth, err := guestAuth(args)
	if err != nil {
		return "", err
	}
	prefix, _ := args["prefix"].(string)
	suffix, _ := args["suffix"].(string)
	directoryPath, _ := args["directory_path"].(string)

	resp, err := methods.CreateTemporaryDirectoryInGuest(ctx, client.Client.Client, &types.CreateTemporaryDirectoryInGuest{
		This:          ref,
		Vm:            vm.Reference(),
		Auth:          auth,
		Prefix:        prefix,
		Suffix:        suffix,
		DirectoryPath: directoryPath,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create a temporary directory on %s: %w", vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"vm": vm.InventoryPath, "path": resp.Returnval})
}

func handleGuestFileTransferFrom(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, ref, err := guestFileManager(ctx, client, args)
	if err != nil {
		return "", err
	}
	auth, err := guestAuth(args)
	if err != nil {
		return "", err
	}
	guestFilePath, err := guestRequiredArg(args, "guest_file_path")
	if err != nil {
		return "", err
	}

	resp, err := methods.InitiateFileTransferFromGuest(ctx, client.Client.Client, &types.InitiateFileTransferFromGuest{
		This:          ref,
		Vm:            vm.Reference(),
		Auth:          auth,
		GuestFilePath: guestFilePath,
	})
	if err != nil {
		return "", fmt.Errorf("failed to initiate file transfer from guest for %s on %s: %w", guestFilePath, vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{
		"vm":              vm.InventoryPath,
		"guest_file_path": guestFilePath,
		"size":            resp.Returnval.Size,
		"url":             resp.Returnval.Url,
	})
}

func handleGuestListProcesses(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, ref, err := guestProcessManager(ctx, client, args)
	if err != nil {
		return "", err
	}
	auth, err := guestAuth(args)
	if err != nil {
		return "", err
	}
	pids, err := guestPidsFromArgs(args)
	if err != nil {
		return "", fmt.Errorf("invalid pids: %w", err)
	}

	resp, err := methods.ListProcessesInGuest(ctx, client.Client.Client, &types.ListProcessesInGuest{
		This: ref,
		Vm:   vm.Reference(),
		Auth: auth,
		Pids: pids,
	})
	if err != nil {
		return "", fmt.Errorf("failed to list processes on %s: %w", vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{
		"vm":        vm.InventoryPath,
		"count":     len(resp.Returnval),
		"processes": resp.Returnval,
	})
}

func handleGuestReadEnvironmentVariable(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, ref, err := guestProcessManager(ctx, client, args)
	if err != nil {
		return "", err
	}
	auth, err := guestAuth(args)
	if err != nil {
		return "", err
	}

	var names []string
	if raw, ok := args["names"]; ok {
		names, err = toStringSlice(raw)
		if err != nil {
			return "", fmt.Errorf("invalid names: %w", err)
		}
	}

	resp, err := methods.ReadEnvironmentVariableInGuest(ctx, client.Client.Client, &types.ReadEnvironmentVariableInGuest{
		This:  ref,
		Vm:    vm.Reference(),
		Auth:  auth,
		Names: names,
	})
	if err != nil {
		return "", fmt.Errorf("failed to read environment variables on %s: %w", vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"vm": vm.InventoryPath, "variables": resp.Returnval})
}

func handleGuestMakeDirectory(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, ref, err := guestFileManager(ctx, client, args)
	if err != nil {
		return "", err
	}
	auth, err := guestAuth(args)
	if err != nil {
		return "", err
	}
	directoryPath, err := guestRequiredArg(args, "directory_path")
	if err != nil {
		return "", err
	}
	createParents, _ := args["create_parent_directories"].(bool)

	if _, err := methods.MakeDirectoryInGuest(ctx, client.Client.Client, &types.MakeDirectoryInGuest{
		This:                    ref,
		Vm:                      vm.Reference(),
		Auth:                    auth,
		DirectoryPath:           directoryPath,
		CreateParentDirectories: createParents,
	}); err != nil {
		return "", fmt.Errorf("failed to create directory %s on %s: %w", directoryPath, vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"vm": vm.InventoryPath, "directory_path": directoryPath, "result": "directory_created"})
}

func handleGuestMoveDirectory(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, ref, err := guestFileManager(ctx, client, args)
	if err != nil {
		return "", err
	}
	auth, err := guestAuth(args)
	if err != nil {
		return "", err
	}
	src, err := guestRequiredArg(args, "src_directory_path")
	if err != nil {
		return "", err
	}
	dst, err := guestRequiredArg(args, "dst_directory_path")
	if err != nil {
		return "", err
	}

	if _, err := methods.MoveDirectoryInGuest(ctx, client.Client.Client, &types.MoveDirectoryInGuest{
		This:             ref,
		Vm:               vm.Reference(),
		Auth:             auth,
		SrcDirectoryPath: src,
		DstDirectoryPath: dst,
	}); err != nil {
		return "", fmt.Errorf("failed to move directory %s to %s on %s: %w", src, dst, vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"vm": vm.InventoryPath, "src_directory_path": src, "dst_directory_path": dst, "result": "directory_moved"})
}

func handleGuestMoveFile(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, ref, err := guestFileManager(ctx, client, args)
	if err != nil {
		return "", err
	}
	auth, err := guestAuth(args)
	if err != nil {
		return "", err
	}
	src, err := guestRequiredArg(args, "src_file_path")
	if err != nil {
		return "", err
	}
	dst, err := guestRequiredArg(args, "dst_file_path")
	if err != nil {
		return "", err
	}
	overwrite, _ := args["overwrite"].(bool)

	if _, err := methods.MoveFileInGuest(ctx, client.Client.Client, &types.MoveFileInGuest{
		This:        ref,
		Vm:          vm.Reference(),
		Auth:        auth,
		SrcFilePath: src,
		DstFilePath: dst,
		Overwrite:   overwrite,
	}); err != nil {
		return "", fmt.Errorf("failed to move file %s to %s on %s: %w", src, dst, vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"vm": vm.InventoryPath, "src_file_path": src, "dst_file_path": dst, "overwrite": overwrite, "result": "file_moved"})
}

func handleGuestChangeFileAttributes(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, ref, err := guestFileManager(ctx, client, args)
	if err != nil {
		return "", err
	}
	auth, err := guestAuth(args)
	if err != nil {
		return "", err
	}
	guestFilePath, err := guestRequiredArg(args, "guest_file_path")
	if err != nil {
		return "", err
	}
	attr, err := guestPosixAttributesFromArgs(args)
	if err != nil {
		return "", err
	}

	if _, err := methods.ChangeFileAttributesInGuest(ctx, client.Client.Client, &types.ChangeFileAttributesInGuest{
		This:           ref,
		Vm:             vm.Reference(),
		Auth:           auth,
		GuestFilePath:  guestFilePath,
		FileAttributes: attr,
	}); err != nil {
		return "", fmt.Errorf("failed to change file attributes for %s on %s: %w", guestFilePath, vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"vm": vm.InventoryPath, "guest_file_path": guestFilePath, "result": "attributes_changed"})
}

func handleGuestFileTransferTo(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, ref, err := guestFileManager(ctx, client, args)
	if err != nil {
		return "", err
	}
	auth, err := guestAuth(args)
	if err != nil {
		return "", err
	}
	guestFilePath, err := guestRequiredArg(args, "guest_file_path")
	if err != nil {
		return "", err
	}
	fileSizeRaw, ok := args["file_size"]
	if !ok {
		return "", fmt.Errorf("file_size is required")
	}
	fileSize, err := toInt64(fileSizeRaw)
	if err != nil {
		return "", fmt.Errorf("invalid file_size: %w", err)
	}
	overwrite, _ := args["overwrite"].(bool)
	attr, err := guestPosixAttributesFromArgs(args)
	if err != nil {
		return "", err
	}

	resp, err := methods.InitiateFileTransferToGuest(ctx, client.Client.Client, &types.InitiateFileTransferToGuest{
		This:           ref,
		Vm:             vm.Reference(),
		Auth:           auth,
		GuestFilePath:  guestFilePath,
		FileAttributes: attr,
		FileSize:       fileSize,
		Overwrite:      overwrite,
	})
	if err != nil {
		return "", fmt.Errorf("failed to initiate file transfer to guest for %s on %s: %w", guestFilePath, vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{
		"vm":              vm.InventoryPath,
		"guest_file_path": guestFilePath,
		"file_size":       fileSize,
		"upload_url":      resp.Returnval,
	})
}

func handleGuestStartProgram(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, ref, err := guestProcessManager(ctx, client, args)
	if err != nil {
		return "", err
	}
	auth, err := guestAuth(args)
	if err != nil {
		return "", err
	}
	programPath, err := guestRequiredArg(args, "program_path")
	if err != nil {
		return "", err
	}
	arguments, _ := args["arguments"].(string)
	workingDirectory, _ := args["working_directory"].(string)

	var envVars []string
	if raw, ok := args["env_variables"]; ok {
		envVars, err = toStringSlice(raw)
		if err != nil {
			return "", fmt.Errorf("invalid env_variables: %w", err)
		}
	}

	resp, err := methods.StartProgramInGuest(ctx, client.Client.Client, &types.StartProgramInGuest{
		This: ref,
		Vm:   vm.Reference(),
		Auth: auth,
		Spec: &types.GuestProgramSpec{
			ProgramPath:      programPath,
			Arguments:        arguments,
			WorkingDirectory: workingDirectory,
			EnvVariables:     envVars,
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to start program %q on %s: %w", programPath, vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"vm": vm.InventoryPath, "program_path": programPath, "pid": resp.Returnval})
}

func handleGuestTerminateProcess(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, ref, err := guestProcessManager(ctx, client, args)
	if err != nil {
		return "", err
	}
	auth, err := guestAuth(args)
	if err != nil {
		return "", err
	}
	pidRaw, ok := args["pid"]
	if !ok {
		return "", fmt.Errorf("pid is required")
	}
	pid, err := toInt64(pidRaw)
	if err != nil {
		return "", fmt.Errorf("invalid pid: %w", err)
	}

	if _, err := methods.TerminateProcessInGuest(ctx, client.Client.Client, &types.TerminateProcessInGuest{
		This: ref,
		Vm:   vm.Reference(),
		Auth: auth,
		Pid:  pid,
	}); err != nil {
		return "", fmt.Errorf("failed to terminate process %d on %s: %w", pid, vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"vm": vm.InventoryPath, "pid": pid, "result": "terminated"})
}

func handleGuestDeleteFile(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, ref, err := guestFileManager(ctx, client, args)
	if err != nil {
		return "", err
	}
	auth, err := guestAuth(args)
	if err != nil {
		return "", err
	}
	filePath, err := guestRequiredArg(args, "file_path")
	if err != nil {
		return "", err
	}

	if _, err := methods.DeleteFileInGuest(ctx, client.Client.Client, &types.DeleteFileInGuest{
		This:     ref,
		Vm:       vm.Reference(),
		Auth:     auth,
		FilePath: filePath,
	}); err != nil {
		return "", fmt.Errorf("failed to delete file %s on %s: %w", filePath, vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"vm": vm.InventoryPath, "file_path": filePath, "result": "file_deleted"})
}

func handleGuestDeleteDirectory(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, ref, err := guestFileManager(ctx, client, args)
	if err != nil {
		return "", err
	}
	auth, err := guestAuth(args)
	if err != nil {
		return "", err
	}
	directoryPath, err := guestRequiredArg(args, "directory_path")
	if err != nil {
		return "", err
	}
	recursive, _ := args["recursive"].(bool)

	if _, err := methods.DeleteDirectoryInGuest(ctx, client.Client.Client, &types.DeleteDirectoryInGuest{
		This:          ref,
		Vm:            vm.Reference(),
		Auth:          auth,
		DirectoryPath: directoryPath,
		Recursive:     recursive,
	}); err != nil {
		return "", fmt.Errorf("failed to delete directory %s on %s: %w", directoryPath, vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"vm": vm.InventoryPath, "directory_path": directoryPath, "recursive": recursive, "result": "directory_deleted"})
}

package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// defaultWaitTimeout bounds the 3 vmware_vm_wait_for_* tools when the caller
// doesn't pass timeout_seconds. Added after finding a real hang, not a
// hypothetical one: an early version of this file passed ctx straight
// through to WaitForIP/WaitForNetIP/WaitForPowerState with a code comment
// claiming it "relies on the caller's MCP request timeout" — but nothing in
// mcpvmware-mcp/main.go's stdio dispatch loop or mcp/types.go ever sets a
// deadline (confirmed by grep, not assumed), so that assumption was false.
// TestVMLifecycleTools_WaitForIPAndNetIP proved it directly: calling
// vmware_vm_wait_for_net_ip against a VM that never reports an IP blocked
// the property.Collector's long-poll for 10 minutes until Go's own test
// timeout force-killed the process — in production that would hang the
// whole stdio connection (this server handles one request at a time)
// indefinitely, not just this one tool call.
const defaultWaitTimeout = 5 * time.Minute

// waitTimeoutFrom reads an optional "timeout_seconds" arg and returns a
// derived context bounded by it (or defaultWaitTimeout if omitted/zero) plus
// its cancel func — callers must defer the cancel func.
func waitTimeoutFrom(ctx context.Context, args map[string]interface{}) (context.Context, context.CancelFunc, error) {
	d := defaultWaitTimeout
	if v, ok := args["timeout_seconds"]; ok {
		n, err := toInt64(v)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid timeout_seconds: %w", err)
		}
		if n <= 0 {
			return nil, nil, fmt.Errorf("timeout_seconds must be positive, got %d", n)
		}
		d = time.Duration(n) * time.Second
	}
	c, cancel := context.WithTimeout(ctx, d)
	return c, cancel, nil
}

var timeoutSecondsArg = map[string]interface{}{
	"type":        "integer",
	"description": "Give up after this many seconds and return an error instead of blocking forever. Default 300 (5 minutes) if omitted.",
}

// registerVMLifecycleTools is part of Fase 2 of the codegen plan
// (".workspace/plans/MCPVMWare2026-08-10-175300-plano-cobertura-completa-api-codegen.plan.md")
// — the "lifecycle" slice of VirtualMachine: pure read-only accessors plus
// simple guest/tools/misc actions, hand-transcribed from
// src/gen/classification.json following the vm.go/host.go/generated_option.go/
// generated_vm_snapshot.go conventions. Device specs, clone/relocate/migrate
// specs, and the VmProvisioningChecker/VmCompatibilityChecker types are out
// of scope here — a separate group handles those in parallel.
//
// Curation deviations from the raw classification / this group's brief (human
// review required — read the real referencia/govmomi/object/virtual_machine.go
// source for every method below before transcribing, not just the brief):
//
//   - EnvironmentBrowser is SKIPPED: it returns a manager handle with no
//     further tools in this batch to chain to, so it has no practical use yet.
//
//   - QueryChangedDiskAreas: the brief suggested building a *types.VirtualDisk
//     with only Key set, on the theory that the API "only needs disk.Key". The
//     real method body (confirmed by reading the source, not assumed) also
//     bounds-checks offset against disk.CapacityInBytes *client-side*, before
//     any server round trip — a zero CapacityInBytes would make every non-zero
//     offset fail with "offset is greater than the disk size". Fetching the
//     real device via vm.Device(ctx) + VirtualDeviceList.FindByKey(disk_key)
//     instead gives an accurate CapacityInBytes (and Backing) for free, so
//     that's what this does — simpler and more correct than asking the caller
//     to also pass a capacity_in_bytes they likely don't know offhand. Also:
//     baseSnapshot is unconditionally dereferenced (baseSnapshot.Reference())
//     with no nil check in the real method — passing a nil baseSnapshot
//     panics inside the govmomi client call, which CallTool's recover() would
//     turn into an ugly "tool ... panicked" error. base_snapshot is therefore
//     a REQUIRED argument here (not optional as the brief suggested), with an
//     explicit empty-string guard producing a clean error instead of relying
//     on the panic recovery as a first line of defense.
//
//   - ResourcePool(): the brief's note to check "can return (nil, nil) for
//     some VM types" doesn't match the real source — it returns a plain
//     error ("VM doesn't have a resourcePool"), never (nil, nil). Matched
//     this project's "0/no result is not a tool error" convention (see
//     finder.go's emptyOnNotFound, generated_option.go's isInvalidNameFault,
//     generated_vm_snapshot.go's FindSnapshot handling) instead: that
//     specific error is caught and reported as "resource_pool": null.
//
//   - UnmountToolsInstaller is registered at Tier 2, not the AST classifier's
//     Tier 1 default — per this group's brief, the classifier's name-keyword
//     match on "Unmount" (aimed at the genuinely dangerous
//     HostStorageSystem.UnmountVmfsVolume) is a false positive here; ejecting
//     the Tools-installer virtual CD is trivial and matches its counterpart
//     MountToolsInstaller's tier.
//
//   - MarkAsVirtualMachine's resource_pool argument is resolved via
//     client.Finder.ResourcePool(ctx, dcScopedPath("host", name)) — resource
//     pools live under the per-datacenter "host" Finder folder, not a
//     "resourcepool" one (confirmed against inventory.go's
//     handleListResourcePools, which uses the same dcScopedPath("host", ...)
//     scoping). Its optional host argument reuses resolveHost from host.go
//     (same package) by building a one-key args submap, rather than
//     duplicating host.go's resolution logic.
//
//   - 6 of these 25 methods have NO handler at all on simulator.VirtualMachine
//     (confirmed by reading referencia/govmomi/simulator/virtual_machine.go —
//     no matching receiver method — and, for AcquireTicket specifically, by
//     an explicit comment in referencia/govmomi/simulator/example_extend_test.go:
//     "Add AcquireTicket method, not implemented by simulator.VirtualMachine"):
//     MountToolsInstaller, UnmountToolsInstaller, UpgradeTools, Answer,
//     AcquireTicket, PutUsbScanCodes. A call against vcsim always faults
//     types.MethodNotFound (simulator/simulator.go's method-dispatch
//     fallback). They are still registered exactly as real vSphere supports
//     them — this is a vcsim simulation gap, not a reason to omit a tool real
//     deployments can use. See generated_vm_lifecycle_test.go for how each is
//     tested short of a full success path.
//
//   - RebootGuest and UpgradeVM ARE implemented by the simulator, but their
//     success path needs state that no public client call can arrange in
//     vcsim: RebootGuest requires Guest.ToolsRunningStatus ==
//     "guestToolsRunning" (nothing in the public API sets that — it is only
//     ever written by the simulator's own internal container/guest-tools
//     helpers), and UpgradeVM's default simulator VM already sits at the
//     latest supported hardware version, so any upgrade attempt legitimately
//     faults ("is latest version"/AlreadyUpgraded) rather than succeeding.
//     Both are still exercised for real against vcsim — the test proves the
//     plumbing reaches vSphere/vcsim's real business-rule validation, just
//     not the unreachable full-success state.
//
//   - All 3 wait_for_* tools gained a required-by-default timeout
//     (timeout_seconds, default 300s via defaultWaitTimeout below) after
//     TestVMLifecycleTools_WaitForIPAndNetIP hung for 10 real minutes in an
//     earlier version of this file — nothing anywhere in this server's stdio
//     dispatch path bounds a tool call, so an unbounded property.Wait blocks
//     the whole connection forever if its condition never becomes true (e.g.
//     a VM with no VMware Tools / no guest IP ever reported). See
//     defaultWaitTimeout's doc comment for the full story.
//
//   - vmware_vm_wait_for_net_ip cannot be driven to a real success path
//     against vcsim: object.VirtualMachine.WaitForNetIP waits on
//     GuestNicInfo.IpConfig.IpAddress (confirmed by reading the real
//     source), but vcsim's only guest-IP test fixture
//     ("SET.guest.ipAddress" via ExtraConfig, see
//     simulator/virtual_machine.go's applyExtraConfig) only ever writes the
//     older, different GuestNicInfo.IpAddress field — there is no vcsim
//     fixture for IpConfig. Confirmed genuinely unreachable, not a bug in
//     this tool; see generated_vm_lifecycle_test.go's
//     wait_for_net_ip_bounded_by_timeout subtest for what actually gets
//     verified instead (registration, arg validation, and — the important
//     part — that the timeout bound above actually works).
func registerVMLifecycleTools(r *Registry) {
	vmArg := map[string]interface{}{
		"type":        "string",
		"description": `VM identifier: a name/pattern (e.g. "cac-WN02") or a full inventory path (e.g. "/ha-datacenter/vm/cac-WN02") as returned by vmware_list_vms. Must resolve to exactly one VM.`,
	}
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}

	// --- Read-only accessors -------------------------------------------

	r.register("vmware_vm_wait_for_power_state",
		`Block until a virtual machine reaches the given power state. Returns immediately if it is already in that state. Bounded by timeout_seconds (default 300) — returns an error instead of blocking forever if the VM never reaches the requested state.`,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm":              vmArg,
				"state":           map[string]interface{}{"type": "string", "enum": []interface{}{"poweredOff", "poweredOn", "suspended"}, "description": "Power state to wait for."},
				"timeout_seconds": timeoutSecondsArg,
			},
			"required": []interface{}{"vm", "state"},
		},
		Tool{Handler: handleVMWaitForPowerState},
	)

	r.register("vmware_vm_wait_for_net_ip",
		`Block until every (or a specific) NIC on a virtual machine reports a guest IP address, keyed by MAC address. Requires VMware Tools running in the guest. Bounded by timeout_seconds (default 300) — returns an error instead of blocking forever if no IP is ever reported.`,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm":              vmArg,
				"v4":              map[string]interface{}{"type": "boolean", "description": "Only consider IPv4 addresses. Default true."},
				"device":          map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": `Wait only for these NICs, identified by MAC address or device name (e.g. "ethernet-0"). Omit to wait for every NIC.`},
				"timeout_seconds": timeoutSecondsArg,
			},
			"required": []interface{}{"vm"},
		},
		Tool{Handler: handleVMWaitForNetIP},
	)

	r.register("vmware_vm_wait_for_ip",
		`Block until a virtual machine's guest.ipAddress property reports an IP address. Requires VMware Tools running in the guest. Bounded by timeout_seconds (default 300) — returns an error instead of blocking forever if no IP is ever reported.`,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm":              vmArg,
				"v4":              map[string]interface{}{"type": "boolean", "description": "Wait specifically for an IPv4 address (an IPv6 address may be reported first otherwise). Default false (accept the first address reported, v4 or v6)."},
				"timeout_seconds": timeoutSecondsArg,
			},
			"required": []interface{}{"vm"},
		},
		Tool{Handler: handleVMWaitForIP},
	)

	r.register("vmware_vm_is_tools_running",
		"Report whether VMware Tools is currently running in a virtual machine's guest OS.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"vm": vmArg},
			"required":   []interface{}{"vm"},
		},
		Tool{Handler: handleVMIsToolsRunning},
	)

	r.register("vmware_vm_is_template",
		"Report whether a virtual machine is marked as a template.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"vm": vmArg},
			"required":   []interface{}{"vm"},
		},
		Tool{Handler: handleVMIsTemplate},
	)

	r.register("vmware_vm_query_changed_disk_areas",
		`Query which byte ranges of a virtual disk changed since a given point, using Changed Block Tracking (CBT). CBT must already be enabled and a changeId recorded for the disk (typically via a prior snapshot taken with CBT on) — otherwise this fails with "CBT is not enabled on disk". Passing offset equal to the disk's real capacity is a valid trivial call that returns a zero-length result without needing CBT at all (useful to confirm the tool/disk resolve correctly).`,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm":            vmArg,
				"disk_key":      map[string]interface{}{"type": "integer", "description": "Key of the virtual disk device on this VM (see vmware_vm_device)."},
				"base_snapshot": map[string]interface{}{"type": "string", "description": "Managed object reference value (e.g. \"snapshot-123\") of the snapshot to diff from. Required — the underlying API dereferences this unconditionally."},
				"cur_snapshot":  map[string]interface{}{"type": "string", "description": `Managed object reference value of the snapshot representing "now". Omit to use the VM's current state (only valid while the VM is powered off).`},
				"offset":        map[string]interface{}{"type": "integer", "description": "Byte offset to start computing changes from."},
			},
			"required": []interface{}{"vm", "disk_key", "base_snapshot", "offset"},
		},
		Tool{Handler: handleVMQueryChangedDiskAreas},
	)

	r.register("vmware_vm_boot_options",
		"Get a virtual machine's firmware boot options (boot delay, BIOS setup on next boot, boot order, etc.).",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"vm": vmArg},
			"required":   []interface{}{"vm"},
		},
		Tool{Handler: handleVMBootOptions},
	)

	r.register("vmware_vm_device",
		"List a virtual machine's virtual hardware devices (key, type, label). Returns a simplified summary, not the full polymorphic device spec — use a device-management tool to reconfigure devices.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"vm": vmArg},
			"required":   []interface{}{"vm"},
		},
		Tool{Handler: handleVMDevice},
	)

	r.register("vmware_vm_host_system",
		"Get the ESXi host a virtual machine currently runs on.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"vm": vmArg},
			"required":   []interface{}{"vm"},
		},
		Tool{Handler: handleVMHostSystem},
	)

	r.register("vmware_vm_resource_pool",
		"Get the resource pool a virtual machine belongs to. Some VMs (e.g. templates) have none — that is reported as a null resource_pool, not an error.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"vm": vmArg},
			"required":   []interface{}{"vm"},
		},
		Tool{Handler: handleVMResourcePool},
	)

	// --- Tier 2: disruptive but reversible ------------------------------

	r.registerDestructive("vmware_vm_standby_guest",
		"Put a virtual machine's guest OS into standby (suspend from within the guest). Requires the VM to be powered on.",
		tier2,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"vm": vmArg, "confirm": confirmArg},
			"required":   []interface{}{"vm", "confirm"},
		},
		Tool{Handler: handleVMStandbyGuest},
	)

	r.registerDestructive("vmware_vm_shutdown_guest",
		"Gracefully shut down a virtual machine's guest OS (requires VMware Tools). Reversible via vmware_vm_power_on, but unlike vmware_vm_power_off this asks the guest OS to shut itself down cleanly.",
		tier2,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"vm": vmArg, "confirm": confirmArg},
			"required":   []interface{}{"vm", "confirm"},
		},
		Tool{Handler: handleVMShutdownGuest},
	)

	r.registerDestructive("vmware_vm_reboot_guest",
		"Gracefully reboot a virtual machine's guest OS (requires VMware Tools running in the guest — fails with ToolsUnavailable otherwise).",
		tier2,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"vm": vmArg, "confirm": confirmArg},
			"required":   []interface{}{"vm", "confirm"},
		},
		Tool{Handler: handleVMRebootGuest},
	)

	r.registerDestructive("vmware_vm_mark_as_template",
		"Convert a powered-off virtual machine into a template. Templates cannot be powered on directly — use vmware_vm_mark_as_virtual_machine to convert back first.",
		tier2,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"vm": vmArg, "confirm": confirmArg},
			"required":   []interface{}{"vm", "confirm"},
		},
		Tool{Handler: handleVMMarkAsTemplate},
	)

	r.registerDestructive("vmware_vm_mark_as_virtual_machine",
		"Convert a template back into a regular (powered-off) virtual machine, assigning it a resource pool and optionally a host.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm":            vmArg,
				"resource_pool": map[string]interface{}{"type": "string", "description": `Resource pool name/pattern or full inventory path (e.g. "Resources") as returned by vmware_list_resource_pools. Required.`},
				"host":          map[string]interface{}{"type": "string", "description": "Host identifier (see vmware_list_hosts). Optional — omit to let vSphere pick."},
				"confirm":       confirmArg,
			},
			"required": []interface{}{"vm", "resource_pool", "confirm"},
		},
		Tool{Handler: handleVMMarkAsVirtualMachine},
	)

	r.registerDestructive("vmware_vm_set_boot_options",
		"Set a virtual machine's firmware boot options (boot delay, BIOS setup on next boot, boot order, etc.). Reversible by setting the options back.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm":      vmArg,
				"options": map[string]interface{}{"type": "object", "description": "A types.VirtualMachineBootOptions JSON object matching its Go struct fields (e.g. {\"bootDelay\": 5000})."},
				"confirm": confirmArg,
			},
			"required": []interface{}{"vm", "options", "confirm"},
		},
		Tool{Handler: handleVMSetBootOptions},
	)

	r.registerDestructive("vmware_vm_refresh_storage_info",
		"Refresh a virtual machine's cached storage/disk-usage information from disk. No configuration change, but classified alongside the other lifecycle actions here per the plan's Fase 1a severity table.",
		tier2,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"vm": vmArg, "confirm": confirmArg},
			"required":   []interface{}{"vm", "confirm"},
		},
		Tool{Handler: handleVMRefreshStorageInfo},
	)

	r.registerDestructive("vmware_vm_upgrade_vm",
		"Upgrade a virtual machine's hardware version (e.g. to \"vmx-19\"). Requires the VM to be powered off. Irreversible in the sense that a VM's hardware version cannot be downgraded, but does not destroy any data.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm":      vmArg,
				"version": map[string]interface{}{"type": "string", "description": `Target hardware version, e.g. "vmx-19". Required.`},
				"confirm": confirmArg,
			},
			"required": []interface{}{"vm", "version", "confirm"},
		},
		Tool{Handler: handleVMUpgradeVM},
	)

	r.registerDestructive("vmware_vm_upgrade_tools",
		"Upgrade VMware Tools in a virtual machine's guest OS. Requires VMware Tools already installed and the VM powered on with the Tools installer mountable.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm":      vmArg,
				"options": map[string]interface{}{"type": "string", "description": "Installer command-line options passed to the guest Tools installer. Optional."},
				"confirm": confirmArg,
			},
			"required": []interface{}{"vm", "confirm"},
		},
		Tool{Handler: handleVMUpgradeTools},
	)

	r.registerDestructive("vmware_vm_mount_tools_installer",
		"Mount the VMware Tools installer as a virtual CD-ROM in a virtual machine's guest OS.",
		tier2,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"vm": vmArg, "confirm": confirmArg},
			"required":   []interface{}{"vm", "confirm"},
		},
		Tool{Handler: handleVMMountToolsInstaller},
	)

	r.registerDestructive("vmware_vm_unmount_tools_installer",
		"Eject the VMware Tools installer virtual CD-ROM from a virtual machine's guest OS. Trivial and reversible (vmware_vm_mount_tools_installer re-mounts it) — kept at Tier 2, not Tier 1, despite the classifier's name-keyword match (see this file's top doc comment).",
		tier2,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"vm": vmArg, "confirm": confirmArg},
			"required":   []interface{}{"vm", "confirm"},
		},
		Tool{Handler: handleVMUnmountToolsInstaller},
	)

	r.registerDestructive("vmware_vm_answer",
		"Answer a virtual machine's pending question (e.g. a device-locked or CD-ROM-swap prompt blocking guest execution).",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm":      vmArg,
				"id":      map[string]interface{}{"type": "string", "description": "The pending question's ID (see the VM's runtime.question property)."},
				"answer":  map[string]interface{}{"type": "string", "description": "The chosen answer's choice ID (one of the question's possible answers)."},
				"confirm": confirmArg,
			},
			"required": []interface{}{"vm", "id", "answer", "confirm"},
		},
		Tool{Handler: handleVMAnswer},
	)

	r.registerDestructive("vmware_vm_acquire_ticket",
		"Acquire a single-use ticket for out-of-band access to a virtual machine (e.g. a WebMKS console session or guest control).",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm":      vmArg,
				"kind":    map[string]interface{}{"type": "string", "description": `Ticket type, e.g. "webmks" or "guestControl".`},
				"confirm": confirmArg,
			},
			"required": []interface{}{"vm", "kind", "confirm"},
		},
		Tool{Handler: handleVMAcquireTicket},
	)

	r.registerDestructive("vmware_vm_put_usb_scan_codes",
		"Inject USB HID keyboard/mouse scan codes into a virtual machine's guest OS (simulated key presses).",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm":      vmArg,
				"spec":    map[string]interface{}{"type": "object", "description": "A types.UsbScanCodeSpec JSON object matching its Go struct fields (a \"keyEvents\" array)."},
				"confirm": confirmArg,
			},
			"required": []interface{}{"vm", "spec", "confirm"},
		},
		Tool{Handler: handleVMPutUsbScanCodes},
	)

	// --- Tier 1: irreversible --------------------------------------------

	r.registerDestructive("vmware_vm_unregister",
		"Remove a virtual machine from inventory WITHOUT deleting its files from the datastore (unlike vmware_vm_destroy, which is irreversible and deletes everything). The files remain and could theoretically be re-registered via their .vmx path, but this tool does not track or expose that path, so treat this as effectively irreversible from this server's point of view. Kept at Tier 1 (the AST classifier's fail-safe default) rather than downgraded, since re-registration isn't something this tool can do for the caller. The VM must already be powered off (vmware_vm_power_off) — vSphere rejects unregistering a powered-on VM, and this tool does not power it off for you (same precondition as vmware_vm_destroy).",
		tier1,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"vm": vmArg, "confirm": confirmArg},
			"required":   []interface{}{"vm", "confirm"},
		},
		Tool{Handler: handleVMUnregister},
	)
}

// decodeJSONArg re-marshals a generic JSON value (as decoded into
// interface{} args) and unmarshals it into dst — the same "accept the
// concrete struct via generic JSON, not a hand-built recursive schema"
// approach as generated_option.go's Update handler and
// generated_vm_snapshot.go's decodeQuiesceSpec, reused here for
// vmware_vm_set_boot_options and vmware_vm_put_usb_scan_codes.
func decodeJSONArg(raw interface{}, dst interface{}) error {
	b, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dst)
}

// toStringSlice converts a JSON array argument (decoded as []interface{})
// into a []string, for the variadic "device" argument of
// vmware_vm_wait_for_net_ip.
func toStringSlice(v interface{}) ([]string, error) {
	arr, ok := v.([]interface{})
	if !ok {
		return nil, fmt.Errorf("expected an array of strings, got %T", v)
	}
	out := make([]string, 0, len(arr))
	for i, item := range arr {
		s, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("[%d] must be a string, got %T", i, item)
		}
		out = append(out, s)
	}
	return out, nil
}

// isNoResourcePoolErr reports whether err is the plain error
// object.VirtualMachine.ResourcePool returns when a VM has no resource pool
// (confirmed by reading the source, not the (nil, nil) this group's brief
// suggested checking for — see this file's top doc comment).
func isNoResourcePoolErr(err error) bool {
	return err != nil && err.Error() == "VM doesn't have a resourcePool"
}

func handleVMWaitForPowerState(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, err := resolveVM(ctx, client, args)
	if err != nil {
		return "", err
	}

	stateStr, _ := args["state"].(string)
	switch types.VirtualMachinePowerState(stateStr) {
	case types.VirtualMachinePowerStatePoweredOff, types.VirtualMachinePowerStatePoweredOn, types.VirtualMachinePowerStateSuspended:
	default:
		return "", fmt.Errorf(`state must be one of "poweredOff", "poweredOn", "suspended", got %q`, stateStr)
	}

	waitCtx, cancel, err := waitTimeoutFrom(ctx, args)
	if err != nil {
		return "", err
	}
	defer cancel()

	if err := vm.WaitForPowerState(waitCtx, types.VirtualMachinePowerState(stateStr)); err != nil {
		return "", fmt.Errorf("failed waiting for %s to reach power state %q: %w", vm.InventoryPath, stateStr, err)
	}

	return marshalJSON(map[string]interface{}{"vm": vm.InventoryPath, "state": stateStr, "result": "reached"})
}

func handleVMWaitForNetIP(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, err := resolveVM(ctx, client, args)
	if err != nil {
		return "", err
	}

	v4 := true
	if v, ok := args["v4"].(bool); ok {
		v4 = v
	}

	var devices []string
	if raw, ok := args["device"]; ok {
		devices, err = toStringSlice(raw)
		if err != nil {
			return "", fmt.Errorf("invalid device: %w", err)
		}
	}

	waitCtx, cancel, err := waitTimeoutFrom(ctx, args)
	if err != nil {
		return "", err
	}
	defer cancel()

	macs, err := vm.WaitForNetIP(waitCtx, v4, devices...)
	if err != nil {
		return "", fmt.Errorf("failed waiting for net IPs on %s: %w", vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"vm": vm.InventoryPath, "v4_only": v4, "net_ip": macs})
}

func handleVMWaitForIP(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, err := resolveVM(ctx, client, args)
	if err != nil {
		return "", err
	}

	waitCtx, cancel, err := waitTimeoutFrom(ctx, args)
	if err != nil {
		return "", err
	}
	defer cancel()

	var ip string
	if v4, ok := args["v4"].(bool); ok {
		ip, err = vm.WaitForIP(waitCtx, v4)
	} else {
		ip, err = vm.WaitForIP(waitCtx)
	}
	if err != nil {
		return "", fmt.Errorf("failed waiting for an IP on %s: %w", vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"vm": vm.InventoryPath, "ip": ip})
}

func handleVMIsToolsRunning(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, err := resolveVM(ctx, client, args)
	if err != nil {
		return "", err
	}

	running, err := vm.IsToolsRunning(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to read tools running status for %s: %w", vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"vm": vm.InventoryPath, "tools_running": running})
}

func handleVMIsTemplate(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, err := resolveVM(ctx, client, args)
	if err != nil {
		return "", err
	}

	isTemplate, err := vm.IsTemplate(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to read template status for %s: %w", vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"vm": vm.InventoryPath, "is_template": isTemplate})
}

func handleVMQueryChangedDiskAreas(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, err := resolveVM(ctx, client, args)
	if err != nil {
		return "", err
	}

	diskKeyRaw, ok := args["disk_key"]
	if !ok {
		return "", fmt.Errorf("disk_key is required")
	}
	diskKey, err := toInt32(diskKeyRaw)
	if err != nil {
		return "", fmt.Errorf("invalid disk_key: %w", err)
	}

	baseSnapshotStr, _ := args["base_snapshot"].(string)
	if baseSnapshotStr == "" {
		return "", fmt.Errorf("base_snapshot is required")
	}
	baseRef := types.ManagedObjectReference{Type: "VirtualMachineSnapshot", Value: baseSnapshotStr}

	var curRef *types.ManagedObjectReference
	if s, ok := args["cur_snapshot"].(string); ok && s != "" {
		r := types.ManagedObjectReference{Type: "VirtualMachineSnapshot", Value: s}
		curRef = &r
	}

	offsetRaw, ok := args["offset"]
	if !ok {
		return "", fmt.Errorf("offset is required")
	}
	offset, err := toInt64(offsetRaw)
	if err != nil {
		return "", fmt.Errorf("invalid offset: %w", err)
	}

	devices, err := vm.Device(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to read devices for %s: %w", vm.InventoryPath, err)
	}
	dev := devices.FindByKey(diskKey)
	disk, ok := dev.(*types.VirtualDisk)
	if !ok {
		return "", fmt.Errorf("disk_key %d does not identify a virtual disk on %s", diskKey, vm.InventoryPath)
	}

	info, err := vm.QueryChangedDiskAreas(ctx, &baseRef, curRef, disk, offset)
	if err != nil {
		return "", fmt.Errorf("failed to query changed disk areas on %s: %w", vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{
		"vm":       vm.InventoryPath,
		"disk_key": diskKey,
		"result":   info,
	})
}

func handleVMBootOptions(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, err := resolveVM(ctx, client, args)
	if err != nil {
		return "", err
	}

	opts, err := vm.BootOptions(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to read boot options for %s: %w", vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"vm": vm.InventoryPath, "boot_options": opts})
}

func handleVMDevice(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, err := resolveVM(ctx, client, args)
	if err != nil {
		return "", err
	}

	devices, err := vm.Device(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to read devices for %s: %w", vm.InventoryPath, err)
	}

	result := make([]map[string]interface{}, 0, len(devices))
	for _, d := range devices {
		bd := d.GetVirtualDevice()
		entry := map[string]interface{}{
			"key":  bd.Key,
			"type": fmt.Sprintf("%T", d),
		}
		if bd.DeviceInfo != nil {
			desc := bd.DeviceInfo.GetDescription()
			entry["label"] = desc.Label
			entry["summary"] = desc.Summary
		}
		result = append(result, entry)
	}

	return marshalJSON(map[string]interface{}{
		"vm":      vm.InventoryPath,
		"count":   len(result),
		"devices": result,
	})
}

func handleVMHostSystem(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, err := resolveVM(ctx, client, args)
	if err != nil {
		return "", err
	}

	host, err := vm.HostSystem(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to read host system for %s: %w", vm.InventoryPath, err)
	}

	// host was built by VirtualMachine.HostSystem via object.NewHostSystem(c,
	// ref) directly from a property value, not via the Finder — its
	// InventoryPath field is therefore always "" (confirmed by running this
	// handler against vcsim, not assumed: TestVMLifecycleTools_ReadOnlyAccessors's
	// host_system subtest caught this for real before this fix). find.InventoryPath
	// walks ManagedEntity.Parent to compose the real path from the bare moref.
	hostPath, err := find.InventoryPath(ctx, client.Client.Client, host.Reference())
	if err != nil {
		return "", fmt.Errorf("failed to resolve inventory path for host of %s: %w", vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"vm": vm.InventoryPath, "host": hostPath})
}

func handleVMResourcePool(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, err := resolveVM(ctx, client, args)
	if err != nil {
		return "", err
	}

	rp, err := vm.ResourcePool(ctx)
	if err != nil {
		if isNoResourcePoolErr(err) {
			return marshalJSON(map[string]interface{}{"vm": vm.InventoryPath, "resource_pool": nil})
		}
		return "", fmt.Errorf("failed to read resource pool for %s: %w", vm.InventoryPath, err)
	}

	// Same InventoryPath gotcha as handleVMHostSystem above: rp came from a
	// bare object.NewResourcePool(c, ref), never a Finder, so
	// rp.InventoryPath is always "" — use find.InventoryPath instead.
	rpPath, err := find.InventoryPath(ctx, client.Client.Client, rp.Reference())
	if err != nil {
		return "", fmt.Errorf("failed to resolve inventory path for resource pool of %s: %w", vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"vm": vm.InventoryPath, "resource_pool": rpPath})
}

func handleVMStandbyGuest(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, err := resolveVM(ctx, client, args)
	if err != nil {
		return "", err
	}

	if err := vm.StandbyGuest(ctx); err != nil {
		return "", fmt.Errorf("failed to standby guest on %s: %w", vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"vm": vm.InventoryPath, "result": "guest_standby"})
}

func handleVMShutdownGuest(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, err := resolveVM(ctx, client, args)
	if err != nil {
		return "", err
	}

	if err := vm.ShutdownGuest(ctx); err != nil {
		return "", fmt.Errorf("failed to shut down guest on %s: %w", vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"vm": vm.InventoryPath, "result": "guest_shutdown"})
}

func handleVMRebootGuest(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, err := resolveVM(ctx, client, args)
	if err != nil {
		return "", err
	}

	if err := vm.RebootGuest(ctx); err != nil {
		return "", fmt.Errorf("failed to reboot guest on %s: %w", vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"vm": vm.InventoryPath, "result": "guest_rebooted"})
}

func handleVMMarkAsTemplate(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, err := resolveVM(ctx, client, args)
	if err != nil {
		return "", err
	}

	if err := vm.MarkAsTemplate(ctx); err != nil {
		return "", fmt.Errorf("failed to mark %s as template: %w", vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"vm": vm.InventoryPath, "result": "marked_as_template"})
}

func handleVMMarkAsVirtualMachine(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, err := resolveVM(ctx, client, args)
	if err != nil {
		return "", err
	}

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

	if err := vm.MarkAsVirtualMachine(ctx, *pool, host); err != nil {
		return "", fmt.Errorf("failed to mark %s as a virtual machine: %w", vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"vm": vm.InventoryPath, "resource_pool": pool.InventoryPath, "result": "marked_as_virtual_machine"})
}

func handleVMSetBootOptions(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, err := resolveVM(ctx, client, args)
	if err != nil {
		return "", err
	}

	raw, ok := args["options"]
	if !ok {
		return "", fmt.Errorf("options is required")
	}
	var opts types.VirtualMachineBootOptions
	if err := decodeJSONArg(raw, &opts); err != nil {
		return "", fmt.Errorf("invalid options: %w", err)
	}

	if err := vm.SetBootOptions(ctx, &opts); err != nil {
		return "", fmt.Errorf("failed to set boot options on %s: %w", vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"vm": vm.InventoryPath, "result": "boot_options_set"})
}

func handleVMRefreshStorageInfo(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, err := resolveVM(ctx, client, args)
	if err != nil {
		return "", err
	}

	if err := vm.RefreshStorageInfo(ctx); err != nil {
		return "", fmt.Errorf("failed to refresh storage info on %s: %w", vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"vm": vm.InventoryPath, "result": "storage_info_refreshed"})
}

func handleVMUpgradeVM(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, err := resolveVM(ctx, client, args)
	if err != nil {
		return "", err
	}
	version, _ := args["version"].(string)
	if version == "" {
		return "", fmt.Errorf("version is required")
	}

	task, err := vm.UpgradeVM(ctx, version)
	if err != nil {
		return "", fmt.Errorf("failed to upgrade %s to %q: %w", vm.InventoryPath, version, err)
	}
	if err := waitForTask(ctx, task); err != nil {
		return "", fmt.Errorf("upgrade-vm task failed for %s: %w", vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"vm": vm.InventoryPath, "version": version, "result": "upgraded"})
}

func handleVMUpgradeTools(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, err := resolveVM(ctx, client, args)
	if err != nil {
		return "", err
	}
	options, _ := args["options"].(string)

	task, err := vm.UpgradeTools(ctx, options)
	if err != nil {
		return "", fmt.Errorf("failed to upgrade tools on %s: %w", vm.InventoryPath, err)
	}
	if err := waitForTask(ctx, task); err != nil {
		return "", fmt.Errorf("upgrade-tools task failed for %s: %w", vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"vm": vm.InventoryPath, "result": "tools_upgraded"})
}

func handleVMMountToolsInstaller(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, err := resolveVM(ctx, client, args)
	if err != nil {
		return "", err
	}

	if err := vm.MountToolsInstaller(ctx); err != nil {
		return "", fmt.Errorf("failed to mount tools installer on %s: %w", vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"vm": vm.InventoryPath, "result": "tools_installer_mounted"})
}

func handleVMUnmountToolsInstaller(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, err := resolveVM(ctx, client, args)
	if err != nil {
		return "", err
	}

	if err := vm.UnmountToolsInstaller(ctx); err != nil {
		return "", fmt.Errorf("failed to unmount tools installer on %s: %w", vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"vm": vm.InventoryPath, "result": "tools_installer_unmounted"})
}

func handleVMAnswer(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, err := resolveVM(ctx, client, args)
	if err != nil {
		return "", err
	}
	id, _ := args["id"].(string)
	if id == "" {
		return "", fmt.Errorf("id is required")
	}
	answer, _ := args["answer"].(string)
	if answer == "" {
		return "", fmt.Errorf("answer is required")
	}

	if err := vm.Answer(ctx, id, answer); err != nil {
		return "", fmt.Errorf("failed to answer question %q on %s: %w", id, vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"vm": vm.InventoryPath, "id": id, "answer": answer, "result": "answered"})
}

func handleVMAcquireTicket(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, err := resolveVM(ctx, client, args)
	if err != nil {
		return "", err
	}
	kind, _ := args["kind"].(string)
	if kind == "" {
		return "", fmt.Errorf("kind is required")
	}

	ticket, err := vm.AcquireTicket(ctx, kind)
	if err != nil {
		return "", fmt.Errorf("failed to acquire %q ticket on %s: %w", kind, vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"vm": vm.InventoryPath, "kind": kind, "ticket": ticket})
}

func handleVMPutUsbScanCodes(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, err := resolveVM(ctx, client, args)
	if err != nil {
		return "", err
	}

	raw, ok := args["spec"]
	if !ok {
		return "", fmt.Errorf("spec is required")
	}
	var spec types.UsbScanCodeSpec
	if err := decodeJSONArg(raw, &spec); err != nil {
		return "", fmt.Errorf("invalid spec: %w", err)
	}

	n, err := vm.PutUsbScanCodes(ctx, spec)
	if err != nil {
		return "", fmt.Errorf("failed to put USB scan codes on %s: %w", vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"vm": vm.InventoryPath, "codes_processed": n})
}

func handleVMUnregister(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, err := resolveVM(ctx, client, args)
	if err != nil {
		return "", err
	}
	path := vm.InventoryPath

	if err := vm.Unregister(ctx); err != nil {
		return "", fmt.Errorf("failed to unregister %s: %w", path, err)
	}

	return marshalJSON(map[string]interface{}{"vm": path, "result": "unregistered"})
}

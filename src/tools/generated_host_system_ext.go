package tools

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// registerHostSystemExtTools adds the HostSystem managed-object methods that
// had no tool anywhere else in this project as of 2026-08-19 — confirmed by
// grepping every existing "vmware_host_*" tool name across src/tools/*.go
// before writing this file (maintenance/reconnect/info live in host.go;
// service/datetime/nic/vsan in generated_host_misc.go; option/certificate/
// account/storage/network/datastore/iscsi in their own generated_host_*.go
// files — none of the 13 methods below were among them).
//
// Every one of the 13 methods was confirmed directly against the govmomi
// v0.55.1 module cache (go.mod pins this version) rather than assumed:
//   - vim25/methods/methods.go has a matching Xxx(ctx, r, req) function for
//     each of the 13 names below.
//   - vim25/types/types.go's "XxxRequestType" struct for each carries a doc
//     comment "The parameters of `HostSystem.Xxx`." confirming the managed
//     object (RebootHost_Task, ShutdownHost_Task, PowerDownHostToStandBy_Task,
//     PowerUpHostFromStandBy_Task, UpdateFlags, UpdateSystemResources,
//     UpdateSystemSwapConfiguration, UpdateIpmi all have that comment
//     verbatim); EnterLockdownMode/ExitLockdownMode/AcquireCimServicesTicket/
//     RetrieveHardwareUptime/QueryHostConnectionInfo have no such comment in
//     the generated source (their RequestType has only a "This" field and no
//     other params, so the generator apparently skipped the doc comment for
//     them) but are documented HostSystem methods in the vSphere API
//     Reference and are NOT present on any other manager type in this
//     module (no HostAccessManager/HostCertificateManager/etc. wrapper
//     exists for them either).
//   - object/host_system.go (the object.HostSystem wrapper) was grepped for
//     every one of the 13 method names — zero matches. The only Xxx(ctx)
//     methods object.HostSystem already wraps are ConfigManager,
//     ResourcePool, ManagementIPs, Disconnect, Reconnect,
//     EnterMaintenanceMode, ExitMaintenanceMode, UpdatePodVMProperty — none
//     of which overlap this file's 13. Every handler below therefore dials
//     the raw vim25 SOAP method directly (methods.Xxx(ctx,
//     client.Client.Client, &types.Xxx{This: host.Reference(), ...})), the
//     same "no object.* wrapper, go straight to methods+types" pattern
//     generated_vm_ft.go documents at length for VirtualMachine's Fault
//     Tolerance methods.
//
// Request struct field shapes (read directly from types.go, not assumed):
//   - RebootHostRequestType / ShutdownHostRequestType: This
//     ManagedObjectReference, Force bool (plain, not pointer — "reboot/shut
//     down even if VMs are running or the host is not in maintenance mode").
//   - PowerDownHostToStandByRequestType: This, TimeoutSec int32 (plain),
//     EvacuatePoweredOffVms *bool (optional — VirtualCenter-only semantics
//     per its doc comment; harmless to omit against ESXi directly).
//   - PowerUpHostFromStandByRequestType: This, TimeoutSec int32 (plain).
//   - EnterLockdownModeRequestType / ExitLockdownModeRequestType /
//     AcquireCimServicesTicketRequestType / RetrieveHardwareUptimeRequestType
//     / QueryHostConnectionInfoRequestType: This only, no other fields.
//   - UpdateFlagsRequestType: This, FlagInfo types.HostFlagInfo (required,
//     not a pointer — just one bool field, BackgroundSnapshotsEnabled
//     *bool).
//   - UpdateSystemResourcesRequestType: This, ResourceInfo
//     types.HostSystemResourceInfo (required — Key string, Config
//     *ResourceConfigSpec, Child []HostSystemResourceInfo).
//   - UpdateSystemSwapConfigurationRequestType: This, SysSwapConfig
//     types.HostSystemSwapConfiguration (required — Option
//     []BaseHostSystemSwapConfigurationSystemSwapOption).
//   - UpdateIpmiRequestType: This, IpmiInfo types.HostIpmiInfo (required —
//     BmcIpAddress/BmcMacAddress/Login/Password, all plain strings).
//
// Response shapes: the 4 "_Task" methods (RebootHost, ShutdownHost,
// PowerDownHostToStandBy, PowerUpHostFromStandBy) each return a plain
// "Returnval ManagedObjectReference" (the task moref) — waited on via
// ftWaitTask (generated_vm_ft.go), the same client-side-only *object.Task
// construction + waitForTask (vm.go) blocking pattern every tier2 tool in
// this project already uses (reused here rather than duplicated, per this
// project's "reuse SSOT" convention — no new hostxWaitTask was written).
// The other 9 methods are synchronous (no task): EnterLockdownMode/
// ExitLockdownMode/UpdateFlags/UpdateSystemResources/
// UpdateSystemSwapConfiguration/UpdateIpmi return an empty response struct;
// AcquireCimServicesTicket returns Returnval types.HostServiceTicket;
// RetrieveHardwareUptime returns Returnval int64 (seconds);
// QueryHostConnectionInfo returns Returnval types.HostConnectInfo.
//
// Class: modeVSphereGeneral. HostSystem is present on a standalone ESXi
// connection (unlike vCenter-only managers such as ClusterComputeResource's
// DRS/HA) and every one of these 13 methods is a plain host-scoped
// operation with no cluster/vCenter-only concept baked into its request
// (contrast generated_vm_ft.go's FT tools, which need a SECOND host for the
// secondary VM — nothing here does). Matches host.go/generated_host_misc.go/
// generated_host_iscsi.go's own modeVSphereGeneral classification for the
// same reason.
//
// Tier judgment calls, per this task's explicit instructions: Reboot/
// Shutdown are tier1 (host-wide impact — every VM on it goes down with no
// graceful guest shutdown guarantee, matching this project's other tier1
// irreversible-impact operations even though the host itself can come back
// up). Every other mutating method (PowerDownToStandBy, PowerUpFromStandBy,
// EnterLockdownMode, ExitLockdownMode, UpdateFlags, UpdateSystemResources,
// UpdateSystemSwapConfiguration, UpdateIpmi) is tier2 — disruptive but
// reversible (standby is reversed by powering back up, lockdown mode is
// reversed by exiting it, every Update* method's prior config can simply be
// read back and reapplied). AcquireCimServicesTicket, RetrieveHardwareUptime,
// and QueryHostConnectionInfo are plain reads — registered via r.register,
// no tier.
//
// vcsim coverage: NONE of the 13 methods have a simulator-side handler —
// confirmed by grepping the entire referencia/govmomi/simulator tree for a
// matching "func (h *HostSystem) Xxx" receiver (or any other receiver) for
// every one of the 13 exact method names and finding zero matches, the same
// "grep the whole simulator tree, find no receiver" verification
// generated_vm_ft.go's and generated_host_misc.go's top doc comments used
// for their own unsimulated methods. A call against vcsim always faults
// types.MethodNotFound. generated_host_system_ext_test.go therefore drives
// every one of the 13 tools with assertReachesServer (reused from
// generated_vm_lifecycle_test.go, not redefined) rather than a positive
// success assertion — proving the wiring (schema, tier gate,
// resolveHost/hostx helpers, raw SOAP dispatch) reaches vcsim and gets back
// a clean server-side fault, not an unknown-tool wiring bug or a recovered
// panic.
func registerHostSystemExtTools(r *Registry) {
	hostArg := map[string]interface{}{
		"type":        "string",
		"description": `Host identifier: a name/pattern (e.g. "esxi-01.local") or a full inventory path (e.g. "/ha-datacenter/host/esxi-01.local/esxi-01.local") as returned by vmware_list_hosts. Must resolve to exactly one host.`,
	}
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}
	forceArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Force the operation even if the host is not in maintenance mode or has VMs running/other operations in progress. Optional, default false.",
	}
	timeoutSecArg := map[string]interface{}{
		"type":        "integer",
		"description": "Seconds the task waits for the host to report the expected heartbeat state before declaring it timed out. Optional, default 0 (server-defined behavior — typically no/minimal wait).",
	}

	// --- power state ---------------------------------------------------

	r.registerDestructive("vmware_host_reboot",
		"Reboot an ESXi host, restarting its hypervisor. Every VM running on it goes down abruptly (no graceful guest shutdown) unless the guest OS reacts fast enough on its own — any unsaved state is lost. Tier 1: treated as high-risk/irreversible impact on running workloads, even though the host itself comes back up on its own.",
		tier1,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":    hostArg,
				"force":   forceArg,
				"confirm": confirmArg,
			},
			"required": []interface{}{"host", "confirm"},
		},
		Tool{Handler: handleHostReboot},
	)

	r.registerDestructive("vmware_host_shutdown",
		"Shut down an ESXi host, powering off its hypervisor entirely. Every VM running on it goes down abruptly. The host stays off until it is powered back on out-of-band (physical power button, IPMI/iLO/DRAC, Wake-on-LAN) — this server has no tool that can power it back on. Tier 1: same high-risk-impact reasoning as vmware_host_reboot, compounded by needing an out-of-band step to undo.",
		tier1,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":    hostArg,
				"force":   forceArg,
				"confirm": confirmArg,
			},
			"required": []interface{}{"host", "confirm"},
		},
		Tool{Handler: handleHostShutdown},
	)

	r.registerDestructive("vmware_host_power_down_to_standby",
		"Power down an ESXi host into standby mode (Distributed Power Management-style), stopping its heartbeat while preserving enough state to be woken later. Reversible via vmware_host_power_up_from_standby.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":                     hostArg,
				"timeout_sec":              timeoutSecArg,
				"evacuate_powered_off_vms": map[string]interface{}{"type": "boolean", "description": "VirtualCenter-only: whether powered-off VMs must first be manually/automatically reregistered elsewhere before the task succeeds. Optional, omit against a standalone ESXi connection."},
				"confirm":                  confirmArg,
			},
			"required": []interface{}{"host", "confirm"},
		},
		Tool{Handler: handleHostPowerDownToStandBy},
	)

	r.registerDestructive("vmware_host_power_up_from_standby",
		"Power an ESXi host back up from standby mode. Reverses vmware_host_power_down_to_standby.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":        hostArg,
				"timeout_sec": timeoutSecArg,
				"confirm":     confirmArg,
			},
			"required": []interface{}{"host", "confirm"},
		},
		Tool{Handler: handleHostPowerUpFromStandBy},
	)

	// --- lockdown mode ---------------------------------------------------

	r.registerDestructive("vmware_host_lockdown_enter",
		"Put an ESXi host into lockdown mode, restricting local console (DCUI) and direct API/SSH access to it in favor of vCenter-only management. Reversible via vmware_host_lockdown_exit.",
		tier2,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"host": hostArg, "confirm": confirmArg},
			"required":   []interface{}{"host", "confirm"},
		},
		Tool{Handler: handleHostLockdownEnter},
	)

	r.registerDestructive("vmware_host_lockdown_exit",
		"Take an ESXi host back out of lockdown mode, restoring local console/direct access. Reverses vmware_host_lockdown_enter.",
		tier2,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"host": hostArg, "confirm": confirmArg},
			"required":   []interface{}{"host", "confirm"},
		},
		Tool{Handler: handleHostLockdownExit},
	)

	// --- CIM ---------------------------------------------------------------

	r.register("vmware_host_acquire_cim_services_ticket",
		"Acquire a one-time CIM (Common Information Model/WBEM) services ticket for an ESXi host, used to authenticate a separate CIM client connection to it.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"host": hostArg},
			"required":   []interface{}{"host"},
		},
		Tool{Handler: handleHostAcquireCimServicesTicket},
	)

	// --- flags / system resources / swap / ipmi -----------------------------

	r.registerDestructive("vmware_host_update_flags",
		`Update an ESXi host's HostFlagInfo settings. Reversible by updating the flags back.`,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":      hostArg,
				"flag_info": map[string]interface{}{"type": "object", "description": `A types.HostFlagInfo JSON object matching its Go struct fields (e.g. {"backgroundSnapshotsEnabled": true}).`},
				"confirm":   confirmArg,
			},
			"required": []interface{}{"host", "flag_info", "confirm"},
		},
		Tool{Handler: handleHostUpdateFlags},
	)

	r.registerDestructive("vmware_host_update_system_resources",
		"Update an ESXi host's system resource group configuration (CPU/memory shares, reservations and limits reserved for host agent/system services). Reversible by updating it back.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":          hostArg,
				"resource_info": map[string]interface{}{"type": "object", "description": `A types.HostSystemResourceInfo JSON object matching its Go struct fields (e.g. {"key": "host", "config": {"cpuAllocation": {"shares": {"level": "normal"}}}}).`},
				"confirm":       confirmArg,
			},
			"required": []interface{}{"host", "resource_info", "confirm"},
		},
		Tool{Handler: handleHostUpdateSystemResources},
	)

	r.registerDestructive("vmware_host_update_system_swap_configuration",
		"Update an ESXi host's system swap configuration (which system swap options — datastore, host cache, hypervisor swap, or disabled — are enabled). Reversible by updating it back.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":            hostArg,
				"sys_swap_config": map[string]interface{}{"type": "object", "description": `A types.HostSystemSwapConfiguration JSON object matching its Go struct fields (e.g. {"option": [{"enabled": true}]}).`},
				"confirm":         confirmArg,
			},
			"required": []interface{}{"host", "sys_swap_config", "confirm"},
		},
		Tool{Handler: handleHostUpdateSystemSwapConfiguration},
	)

	r.registerDestructive("vmware_host_update_ipmi",
		"Update an ESXi host's IPMI/BMC out-of-band management configuration (BMC IP/MAC address, login, password). Reversible by updating it back.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":      hostArg,
				"ipmi_info": map[string]interface{}{"type": "object", "description": `A types.HostIpmiInfo JSON object matching its Go struct fields (e.g. {"bmcIpAddress": "10.0.0.5", "bmcMacAddress": "00:11:22:33:44:55", "login": "admin", "password": "..."}).`},
				"confirm":   confirmArg,
			},
			"required": []interface{}{"host", "ipmi_info", "confirm"},
		},
		Tool{Handler: handleHostUpdateIpmi},
	)

	// --- reads ---------------------------------------------------------------

	r.register("vmware_host_retrieve_hardware_uptime",
		"Get an ESXi host's hardware uptime in seconds — time since the physical hardware was powered on, distinct from hypervisor/software uptime.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"host": hostArg},
			"required":   []interface{}{"host"},
		},
		Tool{Handler: handleHostRetrieveHardwareUptime},
	)

	r.register("vmware_host_query_connection_info",
		"Query connection info for an ESXi host as if freshly discovered: the vCenter Server IP already managing it (if any), HA cluster membership, datastores, networks, and license info. Primarily useful before adding/re-adding a host to a vCenter.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"host": hostArg},
			"required":   []interface{}{"host"},
		},
		Tool{Handler: handleHostQueryConnectionInfo},
	)
}

// hostxOptionalBoolPtr reads an optional boolean argument as a *bool (nil
// when absent/wrong type) — needed for
// PowerDownHostToStandByRequestType.EvacuatePoweredOffVms, the only *bool
// field among this file's request structs (every other bool field here,
// e.g. RebootHostRequestType.Force, is a plain non-pointer bool read
// directly with args[key].(bool)). Same pattern
// generated_host_misc.go's handleHostVsanInternalDeleteObjects already uses
// inline for its own "force" *bool field; pulled out to a named hostx
// helper here since this file is a fresh addition, not an edit to that one.
func hostxOptionalBoolPtr(args map[string]interface{}, key string) *bool {
	if v, ok := args[key].(bool); ok {
		return &v
	}
	return nil
}

// --- power state handlers ---------------------------------------------------

func handleHostReboot(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}
	force, _ := args["force"].(bool)

	resp, err := methods.RebootHost_Task(ctx, client.Client.Client, &types.RebootHost_Task{
		This:  host.Reference(),
		Force: force,
	})
	if err != nil {
		return "", fmt.Errorf("failed to reboot %s: %w", host.InventoryPath, err)
	}
	if err := ftWaitTask(ctx, client, resp.Returnval); err != nil {
		return "", fmt.Errorf("reboot-host task failed for %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "force": force, "result": "rebooted"})
}

func handleHostShutdown(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}
	force, _ := args["force"].(bool)

	resp, err := methods.ShutdownHost_Task(ctx, client.Client.Client, &types.ShutdownHost_Task{
		This:  host.Reference(),
		Force: force,
	})
	if err != nil {
		return "", fmt.Errorf("failed to shut down %s: %w", host.InventoryPath, err)
	}
	if err := ftWaitTask(ctx, client, resp.Returnval); err != nil {
		return "", fmt.Errorf("shutdown-host task failed for %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "force": force, "result": "shut_down"})
}

func handleHostPowerDownToStandBy(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}
	var timeoutSec int32
	if v, ok := args["timeout_sec"]; ok {
		timeoutSec, err = toInt32(v)
		if err != nil {
			return "", fmt.Errorf("invalid timeout_sec: %w", err)
		}
	}
	evacuate := hostxOptionalBoolPtr(args, "evacuate_powered_off_vms")

	resp, err := methods.PowerDownHostToStandBy_Task(ctx, client.Client.Client, &types.PowerDownHostToStandBy_Task{
		This:                  host.Reference(),
		TimeoutSec:            timeoutSec,
		EvacuatePoweredOffVms: evacuate,
	})
	if err != nil {
		return "", fmt.Errorf("failed to power down %s to standby: %w", host.InventoryPath, err)
	}
	if err := ftWaitTask(ctx, client, resp.Returnval); err != nil {
		return "", fmt.Errorf("power-down-to-standby task failed for %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "result": "powered_down_to_standby"})
}

func handleHostPowerUpFromStandBy(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}
	var timeoutSec int32
	if v, ok := args["timeout_sec"]; ok {
		timeoutSec, err = toInt32(v)
		if err != nil {
			return "", fmt.Errorf("invalid timeout_sec: %w", err)
		}
	}

	resp, err := methods.PowerUpHostFromStandBy_Task(ctx, client.Client.Client, &types.PowerUpHostFromStandBy_Task{
		This:       host.Reference(),
		TimeoutSec: timeoutSec,
	})
	if err != nil {
		return "", fmt.Errorf("failed to power up %s from standby: %w", host.InventoryPath, err)
	}
	if err := ftWaitTask(ctx, client, resp.Returnval); err != nil {
		return "", fmt.Errorf("power-up-from-standby task failed for %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "result": "powered_up_from_standby"})
}

// --- lockdown mode handlers --------------------------------------------------

func handleHostLockdownEnter(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}

	_, err = methods.EnterLockdownMode(ctx, client.Client.Client, &types.EnterLockdownMode{This: host.Reference()})
	if err != nil {
		return "", fmt.Errorf("failed to enter lockdown mode on %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "result": "entered_lockdown_mode"})
}

func handleHostLockdownExit(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}

	_, err = methods.ExitLockdownMode(ctx, client.Client.Client, &types.ExitLockdownMode{This: host.Reference()})
	if err != nil {
		return "", fmt.Errorf("failed to exit lockdown mode on %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "result": "exited_lockdown_mode"})
}

// --- CIM handler ---------------------------------------------------------

func handleHostAcquireCimServicesTicket(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}

	resp, err := methods.AcquireCimServicesTicket(ctx, client.Client.Client, &types.AcquireCimServicesTicket{This: host.Reference()})
	if err != nil {
		return "", fmt.Errorf("failed to acquire CIM services ticket for %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "ticket": resp.Returnval})
}

// --- flags / system resources / swap / ipmi handlers ------------------------

func handleHostUpdateFlags(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}
	raw, ok := args["flag_info"]
	if !ok {
		return "", fmt.Errorf("flag_info is required")
	}
	var flagInfo types.HostFlagInfo
	if err := decodeJSONArg(raw, &flagInfo); err != nil {
		return "", fmt.Errorf("invalid flag_info: %w", err)
	}

	_, err = methods.UpdateFlags(ctx, client.Client.Client, &types.UpdateFlags{
		This:     host.Reference(),
		FlagInfo: flagInfo,
	})
	if err != nil {
		return "", fmt.Errorf("failed to update flags on %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "result": "flags_updated"})
}

func handleHostUpdateSystemResources(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}
	raw, ok := args["resource_info"]
	if !ok {
		return "", fmt.Errorf("resource_info is required")
	}
	var resourceInfo types.HostSystemResourceInfo
	if err := decodeJSONArg(raw, &resourceInfo); err != nil {
		return "", fmt.Errorf("invalid resource_info: %w", err)
	}

	_, err = methods.UpdateSystemResources(ctx, client.Client.Client, &types.UpdateSystemResources{
		This:         host.Reference(),
		ResourceInfo: resourceInfo,
	})
	if err != nil {
		return "", fmt.Errorf("failed to update system resources on %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "result": "system_resources_updated"})
}

func handleHostUpdateSystemSwapConfiguration(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}
	raw, ok := args["sys_swap_config"]
	if !ok {
		return "", fmt.Errorf("sys_swap_config is required")
	}
	var sysSwapConfig types.HostSystemSwapConfiguration
	if err := decodeJSONArg(raw, &sysSwapConfig); err != nil {
		return "", fmt.Errorf("invalid sys_swap_config: %w", err)
	}

	_, err = methods.UpdateSystemSwapConfiguration(ctx, client.Client.Client, &types.UpdateSystemSwapConfiguration{
		This:          host.Reference(),
		SysSwapConfig: sysSwapConfig,
	})
	if err != nil {
		return "", fmt.Errorf("failed to update system swap configuration on %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "result": "system_swap_configuration_updated"})
}

func handleHostUpdateIpmi(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}
	raw, ok := args["ipmi_info"]
	if !ok {
		return "", fmt.Errorf("ipmi_info is required")
	}
	var ipmiInfo types.HostIpmiInfo
	if err := decodeJSONArg(raw, &ipmiInfo); err != nil {
		return "", fmt.Errorf("invalid ipmi_info: %w", err)
	}

	_, err = methods.UpdateIpmi(ctx, client.Client.Client, &types.UpdateIpmi{
		This:     host.Reference(),
		IpmiInfo: ipmiInfo,
	})
	if err != nil {
		return "", fmt.Errorf("failed to update IPMI configuration on %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "result": "ipmi_updated"})
}

// --- read handlers ---------------------------------------------------------

func handleHostRetrieveHardwareUptime(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}

	resp, err := methods.RetrieveHardwareUptime(ctx, client.Client.Client, &types.RetrieveHardwareUptime{This: host.Reference()})
	if err != nil {
		return "", fmt.Errorf("failed to retrieve hardware uptime for %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "uptime_seconds": resp.Returnval})
}

func handleHostQueryConnectionInfo(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}

	resp, err := methods.QueryHostConnectionInfo(ctx, client.Client.Client, &types.QueryHostConnectionInfo{This: host.Reference()})
	if err != nil {
		return "", fmt.Errorf("failed to query connection info for %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "connection_info": resp.Returnval})
}

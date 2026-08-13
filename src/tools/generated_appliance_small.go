package tools

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/vapi/appliance/access/consolecli"
	"github.com/vmware/govmomi/vapi/appliance/access/dcui"
	"github.com/vmware/govmomi/vapi/appliance/access/shell"
	"github.com/vmware/govmomi/vapi/appliance/access/ssh"
	"github.com/vmware/govmomi/vapi/appliance/logging"
	"github.com/vmware/govmomi/vapi/appliance/networking"
	"github.com/vmware/govmomi/vapi/appliance/shutdown"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// registerApplianceSmallTools is part of the "MISC-APPLIANCE" group of Fase
// 8a Wave 2 of the codegen plan
// (".workspace/plans/MCPVMWare2026-08-10-175300-plano-cobertura-completa-api-codegen.plan.md")
// — 6 small vapi/appliance/... sub-packages (shutdown, access/{consolecli,
// dcui,shell,ssh}, networking (proxy), logging (forwarding)), each with a
// tiny 1-4 method Manager, all hand-transcribed from the real
// referencia/govmomi sources (confirmed identical, byte-for-byte modulo line
// endings, to the pinned dependency github.com/vmware/govmomi v0.55.1
// actually resolved by src/go.mod). 15 tools total (4 shutdown + 2×4 access
// + 2 networking + 1 logging).
//
// This is the same VAMI (Virtual Appliance Management Interface, vCenter
// Server Appliance administration) family already started in Fase 4's
// tools/appliance.go (vmware_appliance_version/uptime/health/health_detail),
// but architecturally different: Fase 4 had no typed govmomi wrapper for the
// endpoints it needed, so it hand-rolled a generic interface{} decode
// (appliance.go's applianceGet). Every sub-package here DOES have a typed
// govmomi Manager with native json-tagged structs, so — same as every other
// Fase 8a vapi/*.go file (generated_library_core.go, generated_vm_dataset.go)
// — this file decodes straight into those concrete structs instead. Separate
// file, separate registerXTools function; tools/appliance.go is not touched.
//
// Curation deviations from src/gen/classification.json (decided by this
// group's brief, reconfirmed here by reading the real source, not just
// trusted):
//
//   - vmware_appliance_networking_no_proxy / vmware_appliance_networking_proxy_list
//     (NoProxy/ProxyList in networking/proxy.go): the AST classifier's
//     fail-safe default tagged both tier2 (no keyword pattern matched their
//     names). Both are, in reality, pure GET calls with zero request body and
//     zero mutation — confirmed by reading the source: NoProxy is exactly
//     `m.Do(ctx, r.Request(http.MethodGet), &res)` against
//     /appliance/networking/noproxy, and ProxyList is the identical GET
//     pattern against /appliance/networking/proxy (with a client-side
//     []struct{Key,Value} -> ProxyList reshape after the fact, still no
//     mutation). Corrected to read-only (registered via r.register, not
//     r.registerDestructive) — the same "AST keyword match missed a getter
//     that isn't literally named Get*/List*" correction pattern already
//     applied repeatedly in prior fases (e.g. generated_authorization.go's
//     RoleList, this project's OpaqueNetwork.Summary in Fase 5).
//
//   - vmware_appliance_logging_forwarding (Forwarding in logging/forwarding.go):
//     same fail-safe-default tier2 mistag, same correction, same reasoning —
//     confirmed by reading the source: `m.Do(ctx, r.Request(http.MethodGet), &res)`
//     against /appliance/logging/forwarding, no body, no mutation. Corrected
//     to read-only.
//
// vcsim gap, not a bug, for the ENTIRE domain — confirmed directly:
// `grep -rn "shutdown\|consolecli\|dcui\|access/shell\|access/ssh\|networking/proxy\|noproxy\|logging/forwarding" referencia/govmomi/vapi/simulator/simulator.go`
// returns 0 matches (vapi/simulator only imports vapi/library, vapi/tags,
// vapi/vcenter, vapi/rest). Every handler below is therefore tested only for
// "reaches the server cleanly" (assertReachesServer, defined in
// generated_vm_lifecycle_test.go, reused verbatim), same as every other
// vcsim-gap domain in this project.
func registerApplianceSmallTools(r *Registry) {
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}
	emptySchema := map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
	requiresVCSA := " Requires a vCenter Server Appliance — fails against a standalone ESXi host, which has no VAMI (Virtual Appliance Management Interface)."
	enabledArg := map[string]interface{}{
		"type":        "boolean",
		"description": "true to enable, false to disable.",
	}

	// --- Shutdown ------------------------------------------------------------

	r.register("vmware_appliance_shutdown_get",
		"Get details about the vCenter Server Appliance's pending shutdown action (if any): action, reason, scheduled shutdown time."+requiresVCSA,
		emptySchema,
		Tool{Handler: handleApplianceShutdownGet},
	)

	r.registerDestructive("vmware_appliance_shutdown_cancel",
		"Cancel a pending vCenter Server Appliance shutdown/reboot/poweroff action scheduled via vmware_appliance_shutdown_power_off or vmware_appliance_shutdown_reboot."+requiresVCSA,
		tier2,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"confirm": confirmArg},
			"required":   []interface{}{"confirm"},
		},
		Tool{Handler: handleApplianceShutdownCancel},
	)

	r.registerDestructive("vmware_appliance_shutdown_power_off",
		"Schedule a power-off of the vCenter Server Appliance itself (the VCSA VM, not any managed ESXi host/VM). Reversible before it takes effect via vmware_appliance_shutdown_cancel."+requiresVCSA,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"reason":  map[string]interface{}{"type": "string", "description": "Reason recorded for the shutdown. Optional."},
				"delay":   map[string]interface{}{"type": "integer", "description": "Delay in minutes before the power-off takes effect. Default 0 (as soon as possible)."},
				"confirm": confirmArg,
			},
			"required": []interface{}{"confirm"},
		},
		Tool{Handler: handleApplianceShutdownPowerOff},
	)

	r.registerDestructive("vmware_appliance_shutdown_reboot",
		"Schedule a reboot of the vCenter Server Appliance itself (the VCSA VM, not any managed ESXi host/VM). Reversible before it takes effect via vmware_appliance_shutdown_cancel."+requiresVCSA,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"reason":  map[string]interface{}{"type": "string", "description": "Reason recorded for the reboot. Optional."},
				"delay":   map[string]interface{}{"type": "integer", "description": "Delay in minutes before the reboot takes effect. Default 0 (as soon as possible)."},
				"confirm": confirmArg,
			},
			"required": []interface{}{"confirm"},
		},
		Tool{Handler: handleApplianceShutdownReboot},
	)

	// --- Access: consolecli (TTY1) --------------------------------------------

	r.register("vmware_appliance_access_consolecli_get",
		"Get whether the console-based controlled CLI (TTY1) is enabled on the vCenter Server Appliance."+requiresVCSA,
		emptySchema,
		Tool{Handler: handleApplianceAccessConsoleCliGet},
	)

	r.registerDestructive("vmware_appliance_access_consolecli_set",
		"Enable or disable the console-based controlled CLI (TTY1) on the vCenter Server Appliance."+requiresVCSA,
		tier2,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"enabled": enabledArg, "confirm": confirmArg},
			"required":   []interface{}{"enabled", "confirm"},
		},
		Tool{Handler: handleApplianceAccessConsoleCliSet},
	)

	// --- Access: dcui (TTY2) ---------------------------------------------------

	r.register("vmware_appliance_access_dcui_get",
		"Get whether the Direct Console User Interface (DCUI, TTY2) is enabled on the vCenter Server Appliance."+requiresVCSA,
		emptySchema,
		Tool{Handler: handleApplianceAccessDcuiGet},
	)

	r.registerDestructive("vmware_appliance_access_dcui_set",
		"Enable or disable the Direct Console User Interface (DCUI, TTY2) on the vCenter Server Appliance."+requiresVCSA,
		tier2,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"enabled": enabledArg, "confirm": confirmArg},
			"required":   []interface{}{"enabled", "confirm"},
		},
		Tool{Handler: handleApplianceAccessDcuiSet},
	)

	// --- Access: shell (BASH) ----------------------------------------------------

	r.register("vmware_appliance_access_shell_get",
		"Get whether BASH access from within the controlled CLI is enabled on the vCenter Server Appliance, and its configured timeout."+requiresVCSA,
		emptySchema,
		Tool{Handler: handleApplianceAccessShellGet},
	)

	r.registerDestructive("vmware_appliance_access_shell_set",
		"Enable or disable BASH access from within the controlled CLI on the vCenter Server Appliance, and set its timeout."+requiresVCSA,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"enabled": enabledArg,
				"timeout": map[string]interface{}{"type": "integer", "description": "Shell access timeout in seconds. Default 0."},
				"confirm": confirmArg,
			},
			"required": []interface{}{"enabled", "confirm"},
		},
		Tool{Handler: handleApplianceAccessShellSet},
	)

	// --- Access: ssh ---------------------------------------------------------

	r.register("vmware_appliance_access_ssh_get",
		"Get whether the SSH-based controlled CLI is enabled on the vCenter Server Appliance."+requiresVCSA,
		emptySchema,
		Tool{Handler: handleApplianceAccessSshGet},
	)

	r.registerDestructive("vmware_appliance_access_ssh_set",
		"Enable or disable the SSH-based controlled CLI on the vCenter Server Appliance."+requiresVCSA,
		tier2,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"enabled": enabledArg, "confirm": confirmArg},
			"required":   []interface{}{"enabled", "confirm"},
		},
		Tool{Handler: handleApplianceAccessSshSet},
	)

	// --- Networking: proxy (read-only — see this file's top doc comment) -----

	r.register("vmware_appliance_networking_no_proxy",
		"List servers excluded from proxying on the vCenter Server Appliance (the noproxy list)."+requiresVCSA,
		emptySchema,
		Tool{Handler: handleApplianceNetworkingNoProxy},
	)

	r.register("vmware_appliance_networking_proxy_list",
		"Get the vCenter Server Appliance's FTP/HTTP/HTTPS proxy configuration."+requiresVCSA,
		emptySchema,
		Tool{Handler: handleApplianceNetworkingProxyList},
	)

	// --- Logging: forwarding (read-only — see this file's top doc comment) ---

	r.register("vmware_appliance_logging_forwarding",
		"Get the vCenter Server Appliance's log message forwarding configuration (remote syslog destinations)."+requiresVCSA,
		emptySchema,
		Tool{Handler: handleApplianceLoggingForwarding},
	)
}

// --- Manager accessors -------------------------------------------------------
//
// Each mirrors libraryCoreManager/vmDatasetManager: client.REST(ctx) (added
// in Fase 4 for VAMI) already names the likely cause of failure ("is the
// target a vCenter Server Appliance?") if called against a standalone ESXi
// host.

func applianceShutdownManager(ctx context.Context, client *vmware.Client) (*shutdown.Manager, error) {
	rc, err := client.REST(ctx)
	if err != nil {
		return nil, err
	}
	return shutdown.NewManager(rc), nil
}

func applianceConsoleCliManager(ctx context.Context, client *vmware.Client) (*consolecli.Manager, error) {
	rc, err := client.REST(ctx)
	if err != nil {
		return nil, err
	}
	return consolecli.NewManager(rc), nil
}

func applianceDcuiManager(ctx context.Context, client *vmware.Client) (*dcui.Manager, error) {
	rc, err := client.REST(ctx)
	if err != nil {
		return nil, err
	}
	return dcui.NewManager(rc), nil
}

func applianceShellManager(ctx context.Context, client *vmware.Client) (*shell.Manager, error) {
	rc, err := client.REST(ctx)
	if err != nil {
		return nil, err
	}
	return shell.NewManager(rc), nil
}

func applianceSshManager(ctx context.Context, client *vmware.Client) (*ssh.Manager, error) {
	rc, err := client.REST(ctx)
	if err != nil {
		return nil, err
	}
	return ssh.NewManager(rc), nil
}

func applianceNetworkingManager(ctx context.Context, client *vmware.Client) (*networking.Manager, error) {
	rc, err := client.REST(ctx)
	if err != nil {
		return nil, err
	}
	return networking.NewManager(rc), nil
}

func applianceLoggingManager(ctx context.Context, client *vmware.Client) (*logging.Manager, error) {
	rc, err := client.REST(ctx)
	if err != nil {
		return nil, err
	}
	return logging.NewManager(rc), nil
}

// --- Shutdown handlers ---------------------------------------------------------

func handleApplianceShutdownGet(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := applianceShutdownManager(ctx, client)
	if err != nil {
		return "", err
	}
	cfg, err := m.Get(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get pending shutdown action: %w", err)
	}
	return marshalJSON(map[string]interface{}{"config": cfg})
}

func handleApplianceShutdownCancel(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := applianceShutdownManager(ctx, client)
	if err != nil {
		return "", err
	}
	if err := m.Cancel(ctx); err != nil {
		return "", fmt.Errorf("failed to cancel pending shutdown action: %w", err)
	}
	return marshalJSON(map[string]interface{}{"result": "cancelled"})
}

func handleApplianceShutdownPowerOff(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := applianceShutdownManager(ctx, client)
	if err != nil {
		return "", err
	}
	reason, _ := args["reason"].(string)
	delay := 0
	if v, ok := args["delay"]; ok {
		n, err := toInt32(v)
		if err != nil {
			return "", fmt.Errorf("invalid delay: %w", err)
		}
		delay = int(n)
	}

	if err := m.PowerOff(ctx, reason, delay); err != nil {
		return "", fmt.Errorf("failed to schedule appliance power-off: %w", err)
	}
	return marshalJSON(map[string]interface{}{"reason": reason, "delay": delay, "result": "power_off_scheduled"})
}

func handleApplianceShutdownReboot(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := applianceShutdownManager(ctx, client)
	if err != nil {
		return "", err
	}
	reason, _ := args["reason"].(string)
	delay := 0
	if v, ok := args["delay"]; ok {
		n, err := toInt32(v)
		if err != nil {
			return "", fmt.Errorf("invalid delay: %w", err)
		}
		delay = int(n)
	}

	if err := m.Reboot(ctx, reason, delay); err != nil {
		return "", fmt.Errorf("failed to schedule appliance reboot: %w", err)
	}
	return marshalJSON(map[string]interface{}{"reason": reason, "delay": delay, "result": "reboot_scheduled"})
}

// --- Access: consolecli handlers -------------------------------------------

func handleApplianceAccessConsoleCliGet(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := applianceConsoleCliManager(ctx, client)
	if err != nil {
		return "", err
	}
	enabled, err := m.Get(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get consolecli access state: %w", err)
	}
	return marshalJSON(map[string]interface{}{"enabled": enabled})
}

func handleApplianceAccessConsoleCliSet(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := applianceConsoleCliManager(ctx, client)
	if err != nil {
		return "", err
	}
	enabled, ok := args["enabled"].(bool)
	if !ok {
		return "", fmt.Errorf("enabled is required")
	}

	if err := m.Set(ctx, consolecli.Access{Enabled: enabled}); err != nil {
		return "", fmt.Errorf("failed to set consolecli access state: %w", err)
	}
	return marshalJSON(map[string]interface{}{"enabled": enabled, "result": "set"})
}

// --- Access: dcui handlers ---------------------------------------------------

func handleApplianceAccessDcuiGet(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := applianceDcuiManager(ctx, client)
	if err != nil {
		return "", err
	}
	enabled, err := m.Get(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get dcui access state: %w", err)
	}
	return marshalJSON(map[string]interface{}{"enabled": enabled})
}

func handleApplianceAccessDcuiSet(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := applianceDcuiManager(ctx, client)
	if err != nil {
		return "", err
	}
	enabled, ok := args["enabled"].(bool)
	if !ok {
		return "", fmt.Errorf("enabled is required")
	}

	if err := m.Set(ctx, dcui.Access{Enabled: enabled}); err != nil {
		return "", fmt.Errorf("failed to set dcui access state: %w", err)
	}
	return marshalJSON(map[string]interface{}{"enabled": enabled, "result": "set"})
}

// --- Access: shell handlers ---------------------------------------------------

func handleApplianceAccessShellGet(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := applianceShellManager(ctx, client)
	if err != nil {
		return "", err
	}
	access, err := m.Get(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get shell access state: %w", err)
	}
	return marshalJSON(map[string]interface{}{"enabled": access.Enabled, "timeout": access.Timeout})
}

func handleApplianceAccessShellSet(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := applianceShellManager(ctx, client)
	if err != nil {
		return "", err
	}
	enabled, ok := args["enabled"].(bool)
	if !ok {
		return "", fmt.Errorf("enabled is required")
	}
	timeout := 0
	if v, ok := args["timeout"]; ok {
		n, err := toInt32(v)
		if err != nil {
			return "", fmt.Errorf("invalid timeout: %w", err)
		}
		timeout = int(n)
	}

	if err := m.Set(ctx, shell.Access{Enabled: enabled, Timeout: timeout}); err != nil {
		return "", fmt.Errorf("failed to set shell access state: %w", err)
	}
	return marshalJSON(map[string]interface{}{"enabled": enabled, "timeout": timeout, "result": "set"})
}

// --- Access: ssh handlers -----------------------------------------------------

func handleApplianceAccessSshGet(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := applianceSshManager(ctx, client)
	if err != nil {
		return "", err
	}
	enabled, err := m.Get(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get ssh access state: %w", err)
	}
	return marshalJSON(map[string]interface{}{"enabled": enabled})
}

func handleApplianceAccessSshSet(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := applianceSshManager(ctx, client)
	if err != nil {
		return "", err
	}
	enabled, ok := args["enabled"].(bool)
	if !ok {
		return "", fmt.Errorf("enabled is required")
	}

	if err := m.Set(ctx, ssh.Access{Enabled: enabled}); err != nil {
		return "", fmt.Errorf("failed to set ssh access state: %w", err)
	}
	return marshalJSON(map[string]interface{}{"enabled": enabled, "result": "set"})
}

// --- Networking handlers -------------------------------------------------------

func handleApplianceNetworkingNoProxy(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := applianceNetworkingManager(ctx, client)
	if err != nil {
		return "", err
	}
	list, err := m.NoProxy(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get no-proxy list: %w", err)
	}
	return marshalJSON(map[string]interface{}{"no_proxy": list, "count": len(list)})
}

func handleApplianceNetworkingProxyList(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := applianceNetworkingManager(ctx, client)
	if err != nil {
		return "", err
	}
	pl, err := m.ProxyList(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get proxy configuration: %w", err)
	}
	return marshalJSON(map[string]interface{}{"proxies": pl})
}

// --- Logging handlers ----------------------------------------------------------

func handleApplianceLoggingForwarding(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := applianceLoggingManager(ctx, client)
	if err != nil {
		return "", err
	}
	list, err := m.Forwarding(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get logging forwarding configuration: %w", err)
	}
	return marshalJSON(map[string]interface{}{"forwarding": list, "count": len(list)})
}

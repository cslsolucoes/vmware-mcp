// Package tools — generated_vami_access_shutdown.go is Fase 8b (Grupo G4,
// part 2 of 2) of the codegen plan
// (".workspace/plans/MCPVMWare2026-08-10-175300-plano-cobertura-completa-api-codegen.plan.md"),
// covering the LEGACY VAMI access-control routes under /rest/appliance/access/...
// (console CLI, DCUI, shell, SSH) plus the tech-preview shutdown/reboot/
// poweroff routes under /rest/appliance/techpreview/shutdown/.... 12 tools
// total (8 access + 4 shutdown). Same architecture, same source-of-truth
// (the vendored Postman collection, no Go SDK, no simulator coverage) as
// generated_vami_services_accounts_vmon.go — see that file's top doc comment
// for the full rationale; not repeated here. This file reuses that file's
// applianceMutate helper (same package, same 39-route group) and
// appliance.go's applianceGet (Fase 4).
//
// Curation decisions:
//
//   - "LEGACY" naming for Access: Fase 8a (generated_appliance_small.go)
//     already registered vmware_appliance_access_{consolecli,dcui,shell,ssh}_get/_set,
//     built from govmomi's typed vapi/appliance/access/{consolecli,dcui,shell,ssh}
//     Go wrappers, which target the MODERN "/api/appliance/access/..." path
//     (isAPI(path) in vapi/rest/client.go — unwrapped JSON, no {"value":...}
//     envelope). This file's collection instead documents the OLDER
//     "/rest/appliance/access/..." path (wrapped {"value":...} envelope,
//     like every other /rest/... route) — 2 generations of the same logical
//     capability, not a duplicate. Every tool name here therefore carries an
//     explicit "_legacy_" segment (e.g. vmware_appliance_access_legacy_consolecli_get)
//     so it can never be confused with the Fase 8a modern tool of the same
//     resource.
//
//   - Per-resource body shape differs and is NOT uniform, confirmed by
//     reading both the "enable" and "disable" example request bodies for
//     each of the 4 resources in the vendored collection (not assumed):
//     consolecli/dcui/ssh all use a flat {"enabled": bool} body, but shell
//     uses a nested {"config": {"enabled": bool, "timeout": int}} body — one
//     extra field (timeout) the other 3 don't have. This matches the shape
//     already independently confirmed for the MODERN shell endpoint in
//     generated_appliance_small.go (shell.Access{Enabled, Timeout} vs.
//     consolecli/dcui/ssh's bare bool) — the legacy and modern APIs agree on
//     shell's shape even though their JSON envelopes differ. The 8 Postman
//     "Access" folder entries (4 GET + 4×2 enable/disable PUT examples) are
//     collapsed into 4 GET + 4 PUT tools (not 8 PUT tools) — each PUT takes
//     an `enabled` argument rather than being hardcoded to one of the two
//     example bodies, per the plan's explicit instruction.
//
//   - Shutdown tool naming uses "_techpreview_" (not "_legacy_") to match
//     its actual Postman folder ("Techpreview - Power operations") and path
//     prefix (/rest/appliance/techpreview/shutdown/...) — this one has no
//     naming collision risk: Fase 8a's modern shutdown tools
//     (vmware_appliance_shutdown_get/cancel/power_off/reboot, from
//     vapi/appliance/shutdown, generated_appliance_small.go) use different
//     action-name suffixes (power_off/reboot vs. this file's poweroff/restart),
//     so no _legacy_/_techpreview_ disambiguation was strictly required to
//     avoid a collision, but the prefix is kept anyway for the same
//     "instability warning up front" reason as every other techpreview/*
//     tool in this group (see generated_vami_services_accounts_vmon.go).
//     vmware_appliance_techpreview_shutdown_poweroff/restart are tier2
//     (disruptive but reversible before it takes effect, same as this
//     project's existing vmware_vm_power_off/vmware_host_maintenance_enter
//     precedent, per this group's brief) — NOT tier1, unlike this project's
//     genuinely irreversible tier1 tools (vmware_vm_destroy, local account
//     delete).
//
// No vcsim coverage for either sub-domain (confirmed: neither
// appliance/access/{consolecli,dcui,shell,ssh} nor
// appliance/techpreview/shutdown appear anywhere under referencia/govmomi's
// simulator packages — same "Postman collection documents a REST surface
// govmomi's simulator never modeled" situation as this group's other file).
// Tests use a hand-rolled httptest fixture (tools/appliance_test.go's
// pattern), not vcsim.
package tools

import (
	"context"
	"fmt"
	"net/http"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// registerVAMIAccessShutdownTools registers this file's 12 tools: 4 legacy
// access resources (GET + PUT each) and 4 tech-preview shutdown operations.
func registerVAMIAccessShutdownTools(r *Registry) {
	requiresVCSA := " Requires a vCenter Server Appliance — fails against a standalone ESXi host, which has no VAMI (Virtual Appliance Management Interface)."
	techPreviewWarning := " TECH PREVIEW API (VMware's own designation, under the techpreview/ path prefix) — undocumented/unstable, may change or disappear without notice between vSphere versions. No simulator or live vCenter Server Appliance is available to this project to verify field names/behavior against; registered per this project's 100% coverage goal."
	legacyNote := " LEGACY REST path (/rest/appliance/access/...) — distinct from this project's vmware_appliance_access_* tools (Fase 8a), which target the newer /api/appliance/access/... path via a typed govmomi wrapper. Both control the same underlying appliance setting; use whichever path your target vCenter Server Appliance version actually serves."
	emptySchema := map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}
	enabledArg := map[string]interface{}{
		"type":        "boolean",
		"description": "true to enable, false to disable.",
	}

	// --- Access: consolecli (legacy) --------------------------------------------

	r.register("vmware_appliance_access_legacy_consolecli_get",
		"Get whether the console-based controlled CLI (TTY1) is enabled on the vCenter Server Appliance (VAMI legacy, GET /appliance/access/consolecli)."+requiresVCSA+legacyNote,
		emptySchema,
		Tool{Handler: handleVAMIAccessLegacyConsoleCliGet},
	)

	r.registerDestructive("vmware_appliance_access_legacy_consolecli_set",
		"Enable or disable the console-based controlled CLI (TTY1) on the vCenter Server Appliance (VAMI legacy, PUT /appliance/access/consolecli)."+requiresVCSA+legacyNote,
		tier2,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"enabled": enabledArg, "confirm": confirmArg},
			"required":   []interface{}{"enabled", "confirm"},
		},
		Tool{Handler: handleVAMIAccessLegacyConsoleCliSet},
	)

	// --- Access: dcui (legacy) ---------------------------------------------------

	r.register("vmware_appliance_access_legacy_dcui_get",
		"Get whether the Direct Console User Interface (DCUI, TTY2) is enabled on the vCenter Server Appliance (VAMI legacy, GET /appliance/access/dcui)."+requiresVCSA+legacyNote,
		emptySchema,
		Tool{Handler: handleVAMIAccessLegacyDcuiGet},
	)

	r.registerDestructive("vmware_appliance_access_legacy_dcui_set",
		"Enable or disable the Direct Console User Interface (DCUI, TTY2) on the vCenter Server Appliance (VAMI legacy, PUT /appliance/access/dcui)."+requiresVCSA+legacyNote,
		tier2,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"enabled": enabledArg, "confirm": confirmArg},
			"required":   []interface{}{"enabled", "confirm"},
		},
		Tool{Handler: handleVAMIAccessLegacyDcuiSet},
	)

	// --- Access: shell (legacy) — note: nested config.timeout, unlike the other 3 -

	r.register("vmware_appliance_access_legacy_shell_get",
		"Get whether BASH access from within the controlled CLI is enabled on the vCenter Server Appliance, and its configured timeout (VAMI legacy, GET /appliance/access/shell)."+requiresVCSA+legacyNote,
		emptySchema,
		Tool{Handler: handleVAMIAccessLegacyShellGet},
	)

	r.registerDestructive("vmware_appliance_access_legacy_shell_set",
		"Enable or disable BASH access from within the controlled CLI on the vCenter Server Appliance, and set its timeout (VAMI legacy, PUT /appliance/access/shell)."+requiresVCSA+legacyNote,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"enabled": enabledArg,
				"timeout": map[string]interface{}{"type": "integer", "description": "Shell session timeout in seconds. The collection's own examples use 3600 when enabling and 1 when disabling; pass an explicit value to override either default. Optional — 0 sent if omitted."},
				"confirm": confirmArg,
			},
			"required": []interface{}{"enabled", "confirm"},
		},
		Tool{Handler: handleVAMIAccessLegacyShellSet},
	)

	// --- Access: ssh (legacy) -----------------------------------------------------

	r.register("vmware_appliance_access_legacy_ssh_get",
		"Get whether the SSH-based controlled CLI is enabled on the vCenter Server Appliance (VAMI legacy, GET /appliance/access/ssh)."+requiresVCSA+legacyNote,
		emptySchema,
		Tool{Handler: handleVAMIAccessLegacySshGet},
	)

	r.registerDestructive("vmware_appliance_access_legacy_ssh_set",
		"Enable or disable the SSH-based controlled CLI on the vCenter Server Appliance (VAMI legacy, PUT /appliance/access/ssh)."+requiresVCSA+legacyNote,
		tier2,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"enabled": enabledArg, "confirm": confirmArg},
			"required":   []interface{}{"enabled", "confirm"},
		},
		Tool{Handler: handleVAMIAccessLegacySshSet},
	)

	// --- Shutdown (tech preview) --------------------------------------------------

	r.register("vmware_appliance_techpreview_shutdown_get",
		"Get details about the vCenter Server Appliance's pending shutdown/reboot/poweroff action, if any (VAMI, GET /appliance/techpreview/shutdown)."+requiresVCSA+techPreviewWarning,
		emptySchema,
		Tool{Handler: handleVAMIShutdownGet},
	)

	r.registerDestructive("vmware_appliance_techpreview_shutdown_poweroff",
		"Schedule a power-off of the vCenter Server Appliance itself (the VCSA VM, not any managed ESXi host/VM) (VAMI, POST /appliance/techpreview/shutdown/poweroff). Reversible before it takes effect via vmware_appliance_techpreview_shutdown_cancel."+requiresVCSA+techPreviewWarning,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"delay":   map[string]interface{}{"type": "integer", "description": "Delay in minutes before the power-off takes effect. Optional."},
				"reason":  map[string]interface{}{"type": "string", "description": "Reason recorded for the power-off. Optional."},
				"confirm": confirmArg,
			},
			"required": []interface{}{"confirm"},
		},
		Tool{Handler: handleVAMIShutdownPoweroff},
	)

	r.registerDestructive("vmware_appliance_techpreview_shutdown_cancel",
		"Cancel a pending vCenter Server Appliance shutdown/reboot/poweroff action scheduled via vmware_appliance_techpreview_shutdown_poweroff or vmware_appliance_techpreview_shutdown_restart (VAMI, POST /appliance/techpreview/shutdown/cancel)."+requiresVCSA+techPreviewWarning,
		tier2,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"confirm": confirmArg},
			"required":   []interface{}{"confirm"},
		},
		Tool{Handler: handleVAMIShutdownCancel},
	)

	r.registerDestructive("vmware_appliance_techpreview_shutdown_restart",
		"Schedule a reboot of the vCenter Server Appliance itself (the VCSA VM, not any managed ESXi host/VM) (VAMI, POST /appliance/techpreview/shutdown/restart). Reversible before it takes effect via vmware_appliance_techpreview_shutdown_cancel."+requiresVCSA+techPreviewWarning,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"delay":   map[string]interface{}{"type": "integer", "description": "Delay in minutes before the reboot takes effect. Optional."},
				"reason":  map[string]interface{}{"type": "string", "description": "Reason recorded for the reboot. Optional."},
				"confirm": confirmArg,
			},
			"required": []interface{}{"confirm"},
		},
		Tool{Handler: handleVAMIShutdownRestart},
	)
}

// --- Access: consolecli handlers (legacy) -------------------------------------

const vamiAccessConsoleCliPath = "/appliance/access/consolecli"

func handleVAMIAccessLegacyConsoleCliGet(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	v, err := applianceGet(ctx, client, vamiAccessConsoleCliPath)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

func handleVAMIAccessLegacyConsoleCliSet(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	enabled, ok := args["enabled"].(bool)
	if !ok {
		return "", fmt.Errorf("enabled is required")
	}
	v, err := applianceMutate(ctx, client, http.MethodPut, vamiAccessConsoleCliPath, map[string]interface{}{"enabled": enabled})
	if err != nil {
		return "", fmt.Errorf("failed to set legacy consolecli access state: %w", err)
	}
	return marshalJSON(map[string]interface{}{"enabled": enabled, "result": "set", "response": v})
}

// --- Access: dcui handlers (legacy) ---------------------------------------------

const vamiAccessDcuiPath = "/appliance/access/dcui"

func handleVAMIAccessLegacyDcuiGet(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	v, err := applianceGet(ctx, client, vamiAccessDcuiPath)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

func handleVAMIAccessLegacyDcuiSet(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	enabled, ok := args["enabled"].(bool)
	if !ok {
		return "", fmt.Errorf("enabled is required")
	}
	v, err := applianceMutate(ctx, client, http.MethodPut, vamiAccessDcuiPath, map[string]interface{}{"enabled": enabled})
	if err != nil {
		return "", fmt.Errorf("failed to set legacy dcui access state: %w", err)
	}
	return marshalJSON(map[string]interface{}{"enabled": enabled, "result": "set", "response": v})
}

// --- Access: shell handlers (legacy) ---------------------------------------------

const vamiAccessShellPath = "/appliance/access/shell"

func handleVAMIAccessLegacyShellGet(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	v, err := applianceGet(ctx, client, vamiAccessShellPath)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

func handleVAMIAccessLegacyShellSet(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	enabled, ok := args["enabled"].(bool)
	if !ok {
		return "", fmt.Errorf("enabled is required")
	}
	timeout := int32(0)
	if v, ok := args["timeout"]; ok {
		n, err := toInt32(v)
		if err != nil {
			return "", fmt.Errorf("invalid timeout: %w", err)
		}
		timeout = n
	}

	body := map[string]interface{}{"config": map[string]interface{}{"enabled": enabled, "timeout": timeout}}
	v, err := applianceMutate(ctx, client, http.MethodPut, vamiAccessShellPath, body)
	if err != nil {
		return "", fmt.Errorf("failed to set legacy shell access state: %w", err)
	}
	return marshalJSON(map[string]interface{}{"enabled": enabled, "timeout": timeout, "result": "set", "response": v})
}

// --- Access: ssh handlers (legacy) -----------------------------------------------

const vamiAccessSshPath = "/appliance/access/ssh"

func handleVAMIAccessLegacySshGet(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	v, err := applianceGet(ctx, client, vamiAccessSshPath)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

func handleVAMIAccessLegacySshSet(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	enabled, ok := args["enabled"].(bool)
	if !ok {
		return "", fmt.Errorf("enabled is required")
	}
	v, err := applianceMutate(ctx, client, http.MethodPut, vamiAccessSshPath, map[string]interface{}{"enabled": enabled})
	if err != nil {
		return "", fmt.Errorf("failed to set legacy ssh access state: %w", err)
	}
	return marshalJSON(map[string]interface{}{"enabled": enabled, "result": "set", "response": v})
}

// --- Shutdown handlers (tech preview) ---------------------------------------------

const vamiShutdownPath = "/appliance/techpreview/shutdown"

func handleVAMIShutdownGet(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	v, err := applianceGet(ctx, client, vamiShutdownPath)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

// vamiShutdownDelayReasonBody builds the {"config": {"delay":..., "reason":...}}
// body shared by shutdown poweroff/restart.
func vamiShutdownDelayReasonBody(args map[string]interface{}) (map[string]interface{}, error) {
	config := map[string]interface{}{}
	if v, ok := args["delay"]; ok {
		n, err := toInt32(v)
		if err != nil {
			return nil, fmt.Errorf("invalid delay: %w", err)
		}
		config["delay"] = n
	}
	if v, ok := args["reason"]; ok {
		config["reason"] = v
	}
	return map[string]interface{}{"config": config}, nil
}

func handleVAMIShutdownPoweroff(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	body, err := vamiShutdownDelayReasonBody(args)
	if err != nil {
		return "", err
	}
	v, err := applianceMutate(ctx, client, http.MethodPost, vamiShutdownPath+"/poweroff", body)
	if err != nil {
		return "", fmt.Errorf("failed to schedule appliance power-off: %w", err)
	}
	return marshalJSON(map[string]interface{}{"result": "power_off_scheduled", "response": v})
}

func handleVAMIShutdownCancel(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	v, err := applianceMutate(ctx, client, http.MethodPost, vamiShutdownPath+"/cancel", nil)
	if err != nil {
		return "", fmt.Errorf("failed to cancel pending shutdown action: %w", err)
	}
	return marshalJSON(map[string]interface{}{"result": "cancelled", "response": v})
}

func handleVAMIShutdownRestart(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	body, err := vamiShutdownDelayReasonBody(args)
	if err != nil {
		return "", err
	}
	v, err := applianceMutate(ctx, client, http.MethodPost, vamiShutdownPath+"/restart", body)
	if err != nil {
		return "", fmt.Errorf("failed to schedule appliance reboot: %w", err)
	}
	return marshalJSON(map[string]interface{}{"result": "reboot_scheduled", "response": v})
}

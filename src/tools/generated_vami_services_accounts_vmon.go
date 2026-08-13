// Package tools — generated_vami_services_accounts_vmon.go is Fase 8b
// (Grupo G4, part 1 of 2) of the codegen plan
// (".workspace/plans/MCPVMWare2026-08-10-175300-plano-cobertura-completa-api-codegen.plan.md"),
// covering the VAMI (Virtual Appliance Management Interface) SNMP/services/
// system-update/local-accounts routes under /rest/appliance/techpreview/...
// plus the (non-techpreview) /rest/appliance/vmon/service/... routes. 27
// tools total.
//
// Architecturally NEW technique vs. every prior fase: these routes have NO
// Go SDK at all — not a typed vapi/*.go wrapper (Fase 8a's technique) and
// not even a hand-transcribed one (generated_appliance_small.go, also Fase
// 8a). They come solely from a vendored Postman collection
// (".workspace/vSphere Automation REST Resources for appliance.postman_collection.json"),
// which is this project's only source of truth for method/path/body shape —
// there is no simulator or live vCenter Server Appliance available to verify
// against either (same "tensão conhecida" already logged for tools/appliance.go
// in Fase 4: the only real test target, 10.100.2.54, is a standalone ESXi
// host with no VAMI at all). Every handler below therefore goes through
// applianceMutate (this file) for anything beyond a bare GET, and reuses
// appliance.go's applianceGet (Fase 4) for bare GETs — same "generic
// interface{} decode is more honest than a guessed struct" philosophy,
// applied here to the REQUEST body too, not just the response.
//
// techpreview/* is VMware's own name for this API surface — explicitly
// unstable/subject-to-change between vSphere versions, the same caveat this
// project already carries for AuthorizationManager.DisableMethods/
// EnableMethods (Fase 7, generated_authorization.go): registered anyway per
// this project's 100% coverage goal, with an explicit "tech preview" warning
// in every techpreview tool's description. vmon/service (this file's last 6
// tools) is NOT under techpreview/ in the collection — it is vCenter's
// service-monitor daemon (vmon) API, comparatively more stable, but still
// only documented via this same Postman collection (no Go SDK either).
//
// Curation decisions (human review of the raw Postman collection, same
// rigor as every prior fase's classification.json review):
//
//   - Request body shape: for small, simple, well-documented bodies (all
//     string/int/bool fields, no arrays-of-objects) this file exposes named,
//     typed MCP tool arguments and assembles the VAMI JSON body server-side
//     — better caller UX/validation than a raw passthrough. The ONE
//     exception is vmware_appliance_techpreview_snmp_set: its Postman
//     example body has 12 fields, several of them arrays whose element
//     shape is ambiguous from a single generic example ("string"/"bstring"
//     placeholders — e.g. "communities": ["string","bstring"] could mean
//     "any number of community strings" or something more structured; there
//     is no schema doc, simulator, or live target to disambiguate). Per this
//     file's top-level "generic decode is more honest than a guessed
//     struct" philosophy, that one tool accepts a single freeform `config`
//     object argument passed through as-is, with the known field names
//     listed in the tool description as a caller reference rather than
//     enforced as individual typed properties.
//
//   - vmware_appliance_techpreview_snmp_test / vmware_appliance_techpreview_snmp_hash:
//     both are HTTP POST in the collection but neither mutates persisted
//     appliance state (test exercises the already-configured SNMP
//     target(s); hash is a pure client-independent computation — hashing an
//     auth/priv secret for later use in a users/remoteusers config entry).
//     Registered as plain reads (r.register), matching this project's
//     established "POST that only computes/tests, not a real state change,
//     is not destructive" correction pattern (e.g. generated_authorization.go's
//     RoleList, generated_appliance_small.go's NoProxy/ProxyList).
//
//   - vmware_appliance_techpreview_snmp_reset is tier1 (irreversible): it
//     resets SNMP configuration to factory defaults, discarding whatever was
//     configured — same reasoning as this project's other tier1 "discards
//     configuration with no undo" tools (e.g. vmware_authorization_remove_role).
//
//   - vmware_appliance_techpreview_local_accounts_delete is tier1
//     (irreversible account removal); every other local-accounts/services/
//     snmp-enable-disable/system-update/vmon mutation is tier2 (disruptive
//     but reversible — re-enable, re-create, re-configure, restart again).
//
//   - vmware_appliance_techpreview_services_control is the generic
//     start/stop/... action endpoint (collection body includes an "args"
//     array passed to the underlying service command) — distinct from the
//     dedicated restart/stop endpoints below it, which take no args. Kept as
//     3 separate tools (control/restart/stop), matching the 3 separate
//     Postman requests, rather than fusing them into one — restart/stop
//     don't accept "args" at all in the collection, so fusing would force a
//     dead argument onto 2/3 of the calls.
//
//   - vmware_appliance_vmon_service_update (PATCH .../vmon/service/{id}) only
//     exposes startup_type — the sole field in the Postman "spec" example —
//     as a typed argument (small/simple, one string field, no ambiguity).
//
// No vcsim coverage for ANY of these routes (confirmed: none of
// techpreview/monitoring/snmp, techpreview/services, techpreview/system/
// update, techpreview/local-accounts, vmon/service appear anywhere under
// referencia/govmomi — this Postman collection documents a REST surface
// govmomi's simulator never modeled). Tests use a hand-rolled httptest
// fixture, following tools/appliance_test.go's established pattern for this
// exact "no simulator, no live target" situation — not vcsim.
package tools

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// registerVAMIServicesAccountsVmonTools registers this file's 27 tools:
// SNMP (9), services (5), system update (2), local accounts (5), vmon (6).
func registerVAMIServicesAccountsVmonTools(r *Registry) {
	emptySchema := map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
	requiresVCSA := " Requires a vCenter Server Appliance — fails against a standalone ESXi host, which has no VAMI (Virtual Appliance Management Interface)."
	techPreviewWarning := " TECH PREVIEW API (VMware's own designation, under the techpreview/ path prefix) — undocumented/unstable, may change or disappear without notice between vSphere versions. No simulator or live vCenter Server Appliance is available to this project to verify field names/behavior against; registered per this project's 100% coverage goal."
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}

	// --- SNMP (9 tools) ------------------------------------------------------

	r.register("vmware_appliance_techpreview_snmp_get",
		"Get the vCenter Server Appliance's SNMP agent configuration (VAMI, GET /appliance/techpreview/monitoring/snmp)."+requiresVCSA+techPreviewWarning,
		emptySchema,
		Tool{Handler: handleVAMISnmpGet},
	)

	r.registerDestructive("vmware_appliance_techpreview_snmp_enable",
		"Enable the vCenter Server Appliance's SNMP agent (VAMI, POST /appliance/techpreview/monitoring/snmp/enable)."+requiresVCSA+techPreviewWarning,
		tier2,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"confirm": confirmArg},
			"required":   []interface{}{"confirm"},
		},
		Tool{Handler: handleVAMISnmpEnable},
	)

	r.registerDestructive("vmware_appliance_techpreview_snmp_disable",
		"Disable the vCenter Server Appliance's SNMP agent (VAMI, POST /appliance/techpreview/monitoring/snmp/disable)."+requiresVCSA+techPreviewWarning,
		tier2,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"confirm": confirmArg},
			"required":   []interface{}{"confirm"},
		},
		Tool{Handler: handleVAMISnmpDisable},
	)

	r.register("vmware_appliance_techpreview_snmp_stats",
		"Get the vCenter Server Appliance's SNMP agent statistics (VAMI, GET /appliance/techpreview/monitoring/snmp/stats)."+requiresVCSA+techPreviewWarning,
		emptySchema,
		Tool{Handler: handleVAMISnmpStats},
	)

	r.register("vmware_appliance_techpreview_snmp_test",
		"Send a test SNMP notification using the appliance's currently configured SNMP target(s) (VAMI, POST /appliance/techpreview/monitoring/snmp/test). Does not modify the SNMP configuration — read-only from the appliance's persisted-state point of view."+requiresVCSA+techPreviewWarning,
		emptySchema,
		Tool{Handler: handleVAMISnmpTest},
	)

	r.register("vmware_appliance_techpreview_snmp_generate_hash",
		"Compute an SNMPv3 authentication/privacy secret hash for later use in an SNMP users/remoteusers config entry (VAMI, POST /appliance/techpreview/monitoring/snmp/hash). Pure computation — does not read or modify the appliance's persisted SNMP configuration."+requiresVCSA+techPreviewWarning,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"auth_hash":  map[string]interface{}{"type": "string", "description": "Authentication secret to hash."},
				"priv_hash":  map[string]interface{}{"type": "string", "description": "Privacy secret to hash."},
				"raw_secret": map[string]interface{}{"type": "boolean", "description": "Whether auth_hash/priv_hash are raw (unhashed) secrets. Optional."},
			},
		},
		Tool{Handler: handleVAMISnmpGenerateHash},
	)

	r.registerDestructive("vmware_appliance_techpreview_snmp_set",
		"Replace the vCenter Server Appliance's SNMP agent configuration (VAMI, POST /appliance/techpreview/monitoring/snmp). The config object's exact shape is not verifiable against any simulator or live target — see this file's top doc comment; pass the fields documented in the vendored Postman collection example."+requiresVCSA+techPreviewWarning,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"config": map[string]interface{}{
					"type":        "object",
					"description": `Freeform SNMP config object, passed through as-is (no typed struct exists for this route — see this file's top doc comment). Known field names from the vendored Postman collection example: authentication (string, e.g. "none"), communities (array of strings), engineid (string), loglevel (string), notraps (array of strings), port (integer), privacy (string, e.g. "AES128"), remoteusers (array of strings), syscontact (string), syslocation (string), targets (array of strings), users (array of strings), v3targets (array of strings).`,
				},
				"confirm": confirmArg,
			},
			"required": []interface{}{"config", "confirm"},
		},
		Tool{Handler: handleVAMISnmpSet},
	)

	r.registerDestructive("vmware_appliance_techpreview_snmp_reset",
		"Reset the vCenter Server Appliance's SNMP agent configuration to factory defaults (VAMI, POST /appliance/techpreview/monitoring/snmp/reset). Irreversible — any custom SNMP configuration (targets, communities, users, ...) is discarded with no undo."+requiresVCSA+techPreviewWarning,
		tier1,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"confirm": confirmArg},
			"required":   []interface{}{"confirm"},
		},
		Tool{Handler: handleVAMISnmpReset},
	)

	r.register("vmware_appliance_techpreview_snmp_limits",
		"Get the vCenter Server Appliance's SNMP agent configuration limits (e.g. max targets/communities/users) (VAMI, GET /appliance/techpreview/monitoring/snmp/limits)."+requiresVCSA+techPreviewWarning,
		emptySchema,
		Tool{Handler: handleVAMISnmpLimits},
	)

	// --- Services (5 tools) ---------------------------------------------------

	r.register("vmware_appliance_techpreview_services_list",
		"List the vCenter Server Appliance's manageable services (VAMI, GET /appliance/techpreview/services)."+requiresVCSA+techPreviewWarning,
		emptySchema,
		Tool{Handler: handleVAMIServicesList},
	)

	r.register("vmware_appliance_techpreview_services_get",
		"Get details (state, health, ...) of a single vCenter Server Appliance service by name (VAMI, POST /appliance/techpreview/services/status/get)."+requiresVCSA+techPreviewWarning,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name":    map[string]interface{}{"type": "string", "description": "Service name, as returned by vmware_appliance_techpreview_services_list."},
				"timeout": map[string]interface{}{"type": "integer", "description": "Timeout in seconds for the underlying status query. Optional."},
			},
			"required": []interface{}{"name"},
		},
		Tool{Handler: handleVAMIServicesGet},
	)

	r.registerDestructive("vmware_appliance_techpreview_services_control",
		"Send a generic control command to a vCenter Server Appliance service (VAMI, POST /appliance/techpreview/services/control) — args are passed through to the underlying service command (e.g. start/stop/status depending on the appliance's service manager)."+requiresVCSA+techPreviewWarning,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name":    map[string]interface{}{"type": "string", "description": "Service name, as returned by vmware_appliance_techpreview_services_list."},
				"args":    map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Control command arguments passed through to the underlying service command. Optional."},
				"timeout": map[string]interface{}{"type": "integer", "description": "Timeout in seconds for the control operation. Optional."},
				"confirm": confirmArg,
			},
			"required": []interface{}{"name", "confirm"},
		},
		Tool{Handler: handleVAMIServicesControl},
	)

	r.registerDestructive("vmware_appliance_techpreview_services_restart",
		"Restart a vCenter Server Appliance service by name (VAMI, POST /appliance/techpreview/services/restart)."+requiresVCSA+techPreviewWarning,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name":    map[string]interface{}{"type": "string", "description": "Service name, as returned by vmware_appliance_techpreview_services_list."},
				"timeout": map[string]interface{}{"type": "integer", "description": "Timeout in seconds for the restart operation. Optional."},
				"confirm": confirmArg,
			},
			"required": []interface{}{"name", "confirm"},
		},
		Tool{Handler: handleVAMIServicesRestart},
	)

	r.registerDestructive("vmware_appliance_techpreview_services_stop",
		"Stop a vCenter Server Appliance service by name (VAMI, POST /appliance/techpreview/services/stop)."+requiresVCSA+techPreviewWarning,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name":    map[string]interface{}{"type": "string", "description": "Service name, as returned by vmware_appliance_techpreview_services_list."},
				"timeout": map[string]interface{}{"type": "integer", "description": "Timeout in seconds for the stop operation. Optional."},
				"confirm": confirmArg,
			},
			"required": []interface{}{"name", "confirm"},
		},
		Tool{Handler: handleVAMIServicesStop},
	)

	// --- System update (2 tools) -----------------------------------------------

	r.register("vmware_appliance_techpreview_system_update_get",
		"Get the vCenter Server Appliance's software update repository configuration and status (VAMI, GET /appliance/techpreview/system/update)."+requiresVCSA+techPreviewWarning,
		emptySchema,
		Tool{Handler: handleVAMISystemUpdateGet},
	)

	r.registerDestructive("vmware_appliance_techpreview_system_update_set",
		"Set the vCenter Server Appliance's software update repository configuration (VAMI, POST /appliance/techpreview/system/update) — repository URL/credentials and automatic-check schedule."+requiresVCSA+techPreviewWarning,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"check_updates": map[string]interface{}{"type": "string", "description": `Automatic update-check mode, e.g. "disabled". Optional.`},
				"current_url":   map[string]interface{}{"type": "string", "description": "Update repository URL. Optional."},
				"day":           map[string]interface{}{"type": "string", "description": `Day of week for the automatic check, e.g. "Monday". Optional.`},
				"password":      map[string]interface{}{"type": "string", "description": "Password for the update repository, if authenticated. Optional."},
				"time":          map[string]interface{}{"type": "string", "description": "Time of day for the automatic check. Optional."},
				"username":      map[string]interface{}{"type": "string", "description": "Username for the update repository, if authenticated. Optional."},
				"confirm":       confirmArg,
			},
			"required": []interface{}{"confirm"},
		},
		Tool{Handler: handleVAMISystemUpdateSet},
	)

	// --- Local accounts (5 tools) -----------------------------------------------

	r.register("vmware_appliance_techpreview_local_accounts_list",
		"List the vCenter Server Appliance's local OS-level user accounts (VAMI, GET /appliance/techpreview/local-accounts/user)."+requiresVCSA+techPreviewWarning,
		emptySchema,
		Tool{Handler: handleVAMILocalAccountsList},
	)

	r.register("vmware_appliance_techpreview_local_accounts_get",
		"Get details of a single vCenter Server Appliance local OS-level user account by username (VAMI, GET /appliance/techpreview/local-accounts/user/{username})."+requiresVCSA+techPreviewWarning,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"username": map[string]interface{}{"type": "string", "description": "Local account username."}},
			"required":   []interface{}{"username"},
		},
		Tool{Handler: handleVAMILocalAccountsGet},
	)

	r.registerDestructive("vmware_appliance_techpreview_local_accounts_create",
		"Create a new vCenter Server Appliance local OS-level user account (VAMI, POST /appliance/techpreview/local-accounts/user)."+requiresVCSA+techPreviewWarning,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"username": map[string]interface{}{"type": "string", "description": "New account's username."},
				"password": map[string]interface{}{"type": "string", "description": "New account's password."},
				"email":    map[string]interface{}{"type": "string", "description": "New account's email address. Optional."},
				"fullname": map[string]interface{}{"type": "string", "description": "New account's full (display) name. Optional."},
				"role":     map[string]interface{}{"type": "string", "description": `New account's role, e.g. "admin". Optional.`},
				"confirm":  confirmArg,
			},
			"required": []interface{}{"username", "password", "confirm"},
		},
		Tool{Handler: handleVAMILocalAccountsCreate},
	)

	r.registerDestructive("vmware_appliance_techpreview_local_accounts_delete",
		"Remove a vCenter Server Appliance local OS-level user account by username (VAMI, DELETE /appliance/techpreview/local-accounts/user/{username}). Irreversible."+requiresVCSA+techPreviewWarning,
		tier1,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"username": map[string]interface{}{"type": "string", "description": "Local account username to remove."},
				"confirm":  confirmArg,
			},
			"required": []interface{}{"username", "confirm"},
		},
		Tool{Handler: handleVAMILocalAccountsDelete},
	)

	r.registerDestructive("vmware_appliance_techpreview_local_accounts_update",
		"Update a vCenter Server Appliance local OS-level user account (VAMI, PUT /appliance/techpreview/local-accounts/user). The target account is identified by the username field itself (this route has no username path parameter, unlike get/delete)."+requiresVCSA+techPreviewWarning,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"username": map[string]interface{}{"type": "string", "description": "Username of the account to update."},
				"email":    map[string]interface{}{"type": "string", "description": "New email address. Optional."},
				"fullname": map[string]interface{}{"type": "string", "description": "New full (display) name. Optional."},
				"role":     map[string]interface{}{"type": "string", "description": `New role, e.g. "admin". Optional.`},
				"status":   map[string]interface{}{"type": "string", "description": `New account status, e.g. "enabled"/"disabled". Optional.`},
				"confirm":  confirmArg,
			},
			"required": []interface{}{"username", "confirm"},
		},
		Tool{Handler: handleVAMILocalAccountsUpdate},
	)

	// --- Vmon (6 tools, NOT under techpreview/) ---------------------------------

	r.register("vmware_appliance_vmon_service_list",
		"List vCenter Server Appliance services managed by vmon (the service-monitor daemon) (VAMI, GET /appliance/vmon/service)."+requiresVCSA,
		emptySchema,
		Tool{Handler: handleVAMIVmonServiceList},
	)

	r.register("vmware_appliance_vmon_service_get",
		"Get details of a single vmon-managed service by service ID (VAMI, GET /appliance/vmon/service/{service-id})."+requiresVCSA,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"service_id": map[string]interface{}{"type": "string", "description": `vmon service ID, e.g. "content-library".`}},
			"required":   []interface{}{"service_id"},
		},
		Tool{Handler: handleVAMIVmonServiceGet},
	)

	r.registerDestructive("vmware_appliance_vmon_service_restart",
		"Restart a vmon-managed vCenter Server Appliance service (VAMI, POST /appliance/vmon/service/{service-id}/restart)."+requiresVCSA,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"service_id": map[string]interface{}{"type": "string", "description": `vmon service ID, e.g. "content-library".`},
				"confirm":    confirmArg,
			},
			"required": []interface{}{"service_id", "confirm"},
		},
		Tool{Handler: handleVAMIVmonServiceRestart},
	)

	r.registerDestructive("vmware_appliance_vmon_service_start",
		"Start a vmon-managed vCenter Server Appliance service (VAMI, POST /appliance/vmon/service/{service-id}/start)."+requiresVCSA,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"service_id": map[string]interface{}{"type": "string", "description": `vmon service ID, e.g. "content-library".`},
				"confirm":    confirmArg,
			},
			"required": []interface{}{"service_id", "confirm"},
		},
		Tool{Handler: handleVAMIVmonServiceStart},
	)

	r.registerDestructive("vmware_appliance_vmon_service_stop",
		"Stop a vmon-managed vCenter Server Appliance service (VAMI, POST /appliance/vmon/service/{service-id}/stop)."+requiresVCSA,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"service_id": map[string]interface{}{"type": "string", "description": `vmon service ID, e.g. "content-library".`},
				"confirm":    confirmArg,
			},
			"required": []interface{}{"service_id", "confirm"},
		},
		Tool{Handler: handleVAMIVmonServiceStop},
	)

	r.registerDestructive("vmware_appliance_vmon_service_update",
		"Update a vmon-managed service's configuration, e.g. its startup type (VAMI, PATCH /appliance/vmon/service/{service-id})."+requiresVCSA,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"service_id":   map[string]interface{}{"type": "string", "description": `vmon service ID, e.g. "content-library".`},
				"startup_type": map[string]interface{}{"type": "string", "description": `New startup type, e.g. "AUTOMATIC", "MANUAL", "DISABLED".`},
				"confirm":      confirmArg,
			},
			"required": []interface{}{"service_id", "startup_type", "confirm"},
		},
		Tool{Handler: handleVAMIVmonServiceUpdate},
	)
}

// applianceMutate issues an HTTP request with method against a VAMI
// /appliance/... path, optionally JSON-encoding body (pass nil for a
// request with no body — matching the collection's own empty-body examples
// for action endpoints like snmp/enable, snmp/test, shutdown/cancel exactly,
// rather than sending a literal "null" body), and decodes the response
// generically. Complements appliance.go's applianceGet (Fase 4, GET-only) —
// reused here for GET calls, this helper covers everything else
// (POST/PUT/PATCH/DELETE) that this group's ~39 routes need. See this file's
// top doc comment for why no typed request/response structs exist for any
// of it.
func applianceMutate(ctx context.Context, client *vmware.Client, method, path string, body interface{}) (interface{}, error) {
	rc, err := client.REST(ctx)
	if err != nil {
		return nil, err // REST() already names the likely cause (no VAMI on standalone ESXi)
	}

	var req *http.Request
	if body == nil {
		req = rc.Resource(path).Request(method)
	} else {
		req = rc.Resource(path).Request(method, body)
	}

	var result interface{}
	if err := rc.Do(ctx, req, &result); err != nil {
		return nil, fmt.Errorf("VAMI request %s %s failed: %w", method, path, err)
	}
	return result, nil
}

// --- SNMP handlers -----------------------------------------------------------

const vamiSnmpPath = "/appliance/techpreview/monitoring/snmp"

func handleVAMISnmpGet(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	v, err := applianceGet(ctx, client, vamiSnmpPath)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

func handleVAMISnmpEnable(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	v, err := applianceMutate(ctx, client, http.MethodPost, vamiSnmpPath+"/enable", nil)
	if err != nil {
		return "", fmt.Errorf("failed to enable SNMP: %w", err)
	}
	return marshalJSON(map[string]interface{}{"result": "snmp_enabled", "response": v})
}

func handleVAMISnmpDisable(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	v, err := applianceMutate(ctx, client, http.MethodPost, vamiSnmpPath+"/disable", nil)
	if err != nil {
		return "", fmt.Errorf("failed to disable SNMP: %w", err)
	}
	return marshalJSON(map[string]interface{}{"result": "snmp_disabled", "response": v})
}

func handleVAMISnmpStats(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	v, err := applianceGet(ctx, client, vamiSnmpPath+"/stats")
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

func handleVAMISnmpTest(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	v, err := applianceMutate(ctx, client, http.MethodPost, vamiSnmpPath+"/test", nil)
	if err != nil {
		return "", fmt.Errorf("failed to send test SNMP notification: %w", err)
	}
	return marshalJSON(map[string]interface{}{"result": "snmp_test_sent", "response": v})
}

func handleVAMISnmpGenerateHash(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	config := map[string]interface{}{}
	if v, ok := args["auth_hash"]; ok {
		config["auth_hash"] = v
	}
	if v, ok := args["priv_hash"]; ok {
		config["priv_hash"] = v
	}
	if v, ok := args["raw_secret"]; ok {
		config["raw_secret"] = v
	}

	v, err := applianceMutate(ctx, client, http.MethodPost, vamiSnmpPath+"/hash", map[string]interface{}{"config": config})
	if err != nil {
		return "", fmt.Errorf("failed to generate SNMP hash: %w", err)
	}
	return marshalJSON(v)
}

func handleVAMISnmpSet(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	config, ok := args["config"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("config is required and must be an object")
	}

	v, err := applianceMutate(ctx, client, http.MethodPost, vamiSnmpPath, map[string]interface{}{"config": config})
	if err != nil {
		return "", fmt.Errorf("failed to set SNMP configuration: %w", err)
	}
	return marshalJSON(map[string]interface{}{"result": "snmp_config_set", "response": v})
}

func handleVAMISnmpReset(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	v, err := applianceMutate(ctx, client, http.MethodPost, vamiSnmpPath+"/reset", nil)
	if err != nil {
		return "", fmt.Errorf("failed to reset SNMP configuration to factory defaults: %w", err)
	}
	return marshalJSON(map[string]interface{}{"result": "snmp_reset", "response": v})
}

func handleVAMISnmpLimits(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	v, err := applianceGet(ctx, client, vamiSnmpPath+"/limits")
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

// --- Services handlers ---------------------------------------------------------

const vamiServicesPath = "/appliance/techpreview/services"

func handleVAMIServicesList(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	v, err := applianceGet(ctx, client, vamiServicesPath)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

// vamiServiceNameTimeoutBody builds the {"name":..., "timeout":...} body
// shared by services_get/services_restart/services_stop.
func vamiServiceNameTimeoutBody(args map[string]interface{}) (map[string]interface{}, error) {
	name, _ := args["name"].(string)
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	body := map[string]interface{}{"name": name}
	if v, ok := args["timeout"]; ok {
		n, err := toInt32(v)
		if err != nil {
			return nil, fmt.Errorf("invalid timeout: %w", err)
		}
		body["timeout"] = n
	}
	return body, nil
}

func handleVAMIServicesGet(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	body, err := vamiServiceNameTimeoutBody(args)
	if err != nil {
		return "", err
	}
	v, err := applianceMutate(ctx, client, http.MethodPost, vamiServicesPath+"/status/get", body)
	if err != nil {
		return "", fmt.Errorf("failed to get service details for %q: %w", body["name"], err)
	}
	return marshalJSON(v)
}

func handleVAMIServicesControl(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	name, _ := args["name"].(string)
	if name == "" {
		return "", fmt.Errorf("name is required")
	}
	body := map[string]interface{}{"name": name}
	if raw, ok := args["args"]; ok && raw != nil {
		argList, err := toStringSlice(raw)
		if err != nil {
			return "", fmt.Errorf("invalid args: %w", err)
		}
		body["args"] = argList
	}
	if v, ok := args["timeout"]; ok {
		n, err := toInt32(v)
		if err != nil {
			return "", fmt.Errorf("invalid timeout: %w", err)
		}
		body["timeout"] = n
	}

	v, err := applianceMutate(ctx, client, http.MethodPost, vamiServicesPath+"/control", body)
	if err != nil {
		return "", fmt.Errorf("failed to control service %q: %w", name, err)
	}
	return marshalJSON(map[string]interface{}{"result": "service_controlled", "name": name, "response": v})
}

func handleVAMIServicesRestart(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	body, err := vamiServiceNameTimeoutBody(args)
	if err != nil {
		return "", err
	}
	v, err := applianceMutate(ctx, client, http.MethodPost, vamiServicesPath+"/restart", body)
	if err != nil {
		return "", fmt.Errorf("failed to restart service %q: %w", body["name"], err)
	}
	return marshalJSON(map[string]interface{}{"result": "service_restarted", "name": body["name"], "response": v})
}

func handleVAMIServicesStop(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	body, err := vamiServiceNameTimeoutBody(args)
	if err != nil {
		return "", err
	}
	v, err := applianceMutate(ctx, client, http.MethodPost, vamiServicesPath+"/stop", body)
	if err != nil {
		return "", fmt.Errorf("failed to stop service %q: %w", body["name"], err)
	}
	return marshalJSON(map[string]interface{}{"result": "service_stopped", "name": body["name"], "response": v})
}

// --- System update handlers -----------------------------------------------------

const vamiSystemUpdatePath = "/appliance/techpreview/system/update"

func handleVAMISystemUpdateGet(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	v, err := applianceGet(ctx, client, vamiSystemUpdatePath)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

func handleVAMISystemUpdateSet(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	config := map[string]interface{}{}
	if v, ok := args["check_updates"]; ok {
		config["check_updates"] = v
	}
	if v, ok := args["current_url"]; ok {
		config["current_URL"] = v
	}
	if v, ok := args["day"]; ok {
		config["day"] = v
	}
	if v, ok := args["password"]; ok {
		config["password"] = v
	}
	if v, ok := args["time"]; ok {
		config["time"] = v
	}
	if v, ok := args["username"]; ok {
		config["username"] = v
	}

	v, err := applianceMutate(ctx, client, http.MethodPost, vamiSystemUpdatePath, map[string]interface{}{"config": config})
	if err != nil {
		return "", fmt.Errorf("failed to set system update repository configuration: %w", err)
	}
	return marshalJSON(map[string]interface{}{"result": "system_update_config_set", "response": v})
}

// --- Local accounts handlers -----------------------------------------------------

const vamiLocalAccountsPath = "/appliance/techpreview/local-accounts/user"

func handleVAMILocalAccountsList(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	v, err := applianceGet(ctx, client, vamiLocalAccountsPath)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

func handleVAMILocalAccountsGet(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	username, _ := args["username"].(string)
	if username == "" {
		return "", fmt.Errorf("username is required")
	}
	v, err := applianceGet(ctx, client, vamiLocalAccountsPath+"/"+url.PathEscape(username))
	if err != nil {
		return "", fmt.Errorf("failed to get local account %q: %w", username, err)
	}
	return marshalJSON(v)
}

func handleVAMILocalAccountsCreate(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	username, _ := args["username"].(string)
	if username == "" {
		return "", fmt.Errorf("username is required")
	}
	password, _ := args["password"].(string)
	if password == "" {
		return "", fmt.Errorf("password is required")
	}
	config := map[string]interface{}{"username": username, "password": password}
	if v, ok := args["email"]; ok {
		config["email"] = v
	}
	if v, ok := args["fullname"]; ok {
		config["fullname"] = v
	}
	if v, ok := args["role"]; ok {
		config["role"] = v
	}

	v, err := applianceMutate(ctx, client, http.MethodPost, vamiLocalAccountsPath, map[string]interface{}{"config": config})
	if err != nil {
		return "", fmt.Errorf("failed to create local account %q: %w", username, err)
	}
	return marshalJSON(map[string]interface{}{"result": "local_account_created", "username": username, "response": v})
}

func handleVAMILocalAccountsDelete(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	username, _ := args["username"].(string)
	if username == "" {
		return "", fmt.Errorf("username is required")
	}

	v, err := applianceMutate(ctx, client, http.MethodDelete, vamiLocalAccountsPath+"/"+url.PathEscape(username), nil)
	if err != nil {
		return "", fmt.Errorf("failed to delete local account %q: %w", username, err)
	}
	return marshalJSON(map[string]interface{}{"result": "local_account_deleted", "username": username, "response": v})
}

func handleVAMILocalAccountsUpdate(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	username, _ := args["username"].(string)
	if username == "" {
		return "", fmt.Errorf("username is required")
	}
	config := map[string]interface{}{"username": username}
	if v, ok := args["email"]; ok {
		config["email"] = v
	}
	if v, ok := args["fullname"]; ok {
		config["fullname"] = v
	}
	if v, ok := args["role"]; ok {
		config["role"] = v
	}
	if v, ok := args["status"]; ok {
		config["status"] = v
	}

	v, err := applianceMutate(ctx, client, http.MethodPut, vamiLocalAccountsPath, map[string]interface{}{"config": config})
	if err != nil {
		return "", fmt.Errorf("failed to update local account %q: %w", username, err)
	}
	return marshalJSON(map[string]interface{}{"result": "local_account_updated", "username": username, "response": v})
}

// --- Vmon handlers ---------------------------------------------------------------

const vamiVmonServicePath = "/appliance/vmon/service"

func handleVAMIVmonServiceList(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	v, err := applianceGet(ctx, client, vamiVmonServicePath)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

func handleVAMIVmonServiceGet(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	serviceID, _ := args["service_id"].(string)
	if serviceID == "" {
		return "", fmt.Errorf("service_id is required")
	}
	v, err := applianceGet(ctx, client, vamiVmonServicePath+"/"+url.PathEscape(serviceID))
	if err != nil {
		return "", fmt.Errorf("failed to get vmon service %q: %w", serviceID, err)
	}
	return marshalJSON(v)
}

func handleVAMIVmonServiceRestart(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	serviceID, _ := args["service_id"].(string)
	if serviceID == "" {
		return "", fmt.Errorf("service_id is required")
	}
	v, err := applianceMutate(ctx, client, http.MethodPost, vamiVmonServicePath+"/"+url.PathEscape(serviceID)+"/restart", nil)
	if err != nil {
		return "", fmt.Errorf("failed to restart vmon service %q: %w", serviceID, err)
	}
	return marshalJSON(map[string]interface{}{"result": "vmon_service_restarted", "service_id": serviceID, "response": v})
}

func handleVAMIVmonServiceStart(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	serviceID, _ := args["service_id"].(string)
	if serviceID == "" {
		return "", fmt.Errorf("service_id is required")
	}
	v, err := applianceMutate(ctx, client, http.MethodPost, vamiVmonServicePath+"/"+url.PathEscape(serviceID)+"/start", nil)
	if err != nil {
		return "", fmt.Errorf("failed to start vmon service %q: %w", serviceID, err)
	}
	return marshalJSON(map[string]interface{}{"result": "vmon_service_started", "service_id": serviceID, "response": v})
}

func handleVAMIVmonServiceStop(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	serviceID, _ := args["service_id"].(string)
	if serviceID == "" {
		return "", fmt.Errorf("service_id is required")
	}
	v, err := applianceMutate(ctx, client, http.MethodPost, vamiVmonServicePath+"/"+url.PathEscape(serviceID)+"/stop", nil)
	if err != nil {
		return "", fmt.Errorf("failed to stop vmon service %q: %w", serviceID, err)
	}
	return marshalJSON(map[string]interface{}{"result": "vmon_service_stopped", "service_id": serviceID, "response": v})
}

func handleVAMIVmonServiceUpdate(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	serviceID, _ := args["service_id"].(string)
	if serviceID == "" {
		return "", fmt.Errorf("service_id is required")
	}
	startupType, _ := args["startup_type"].(string)
	if startupType == "" {
		return "", fmt.Errorf("startup_type is required")
	}

	body := map[string]interface{}{"spec": map[string]interface{}{"startup_type": startupType}}
	v, err := applianceMutate(ctx, client, http.MethodPatch, vamiVmonServicePath+"/"+url.PathEscape(serviceID), body)
	if err != nil {
		return "", fmt.Errorf("failed to update vmon service %q: %w", serviceID, err)
	}
	return marshalJSON(map[string]interface{}{"result": "vmon_service_updated", "service_id": serviceID, "startup_type": startupType, "response": v})
}

package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// registerHostMiscTools is the "misc" slice of Fase 3 of the codegen plan
// (".workspace/plans/MCPVMWare2026-08-10-175300-plano-cobertura-completa-api-codegen.plan.md")
// — the 5 smallest Host-domain managers bundled together, hand-transcribed
// from referencia/govmomi/object following the host.go/generated_option.go/
// generated_vm_lifecycle.go conventions: HostServiceSystem (5 methods),
// HostDateTimeSystem (3), HostVirtualNicManager (3), HostVsanSystem (1),
// HostVsanInternalSystem (3) = 15 tools. Every receiver is reached the same
// way as generated_option.go's OptionManager: host.ConfigManager().XSystem(ctx).
//
// vcsim coverage (verified by grepping the exact SOAP method names each Go
// method wraps — methods.XxxYyy — in referencia/govmomi/simulator/*.go, not
// by receiver-type-name alone; a type-name-only search previously gave a
// WRONG answer for a sibling group's HostAccountManager in this same
// project, so this was independently re-verified here rather than trusted
// from the group brief):
//
//   - StartService, StopService, RestartService, UpdateServicePolicy
//     (HostServiceSystem): zero hits anywhere in the simulator package.
//   - QueryDateTime, UpdateDateTime, UpdateDateTimeConfig
//     (HostDateTimeSystem): zero hits.
//   - UpdateVsan_Task (HostVsanSystem.Update): zero hits.
//   - QueryVsanObjectUuidsByFilter, GetVsanObjExtAttrs, DeleteVsanObjects
//     (HostVsanInternalSystem): zero hits.
//   - SelectVnicForNicType, DeselectVnicForNicType (HostVirtualNicManager):
//     zero hits.
//
// A SECOND, deeper vcsim gap not in the original brief, found empirically by
// actually running generated_host_misc_test.go against vcsim (the first
// version of this comment guessed "the ConfigManager reference is nil /
// ErrNotSupported" from reading host_system.go alone — that guess was WRONG,
// caught by reading the real go test output rather than trusting the static
// read, per this project's "run it and read the result" rule): every
// ConfigManager field, including ServiceSystem/DateTimeSystem/VsanSystem/
// VsanInternalSystem, already carries a well-formed placeholder
// ManagedObjectReference from the static ESX host template
// (referencia/govmomi/simulator/esx/host_system.go's package-level
// HostSystem var, e.g. ServiceSystem -> "HostServiceSystem:serviceSystem",
// DateTimeSystem -> "HostDateTimeSystem:dateTimeSystem", VsanSystem ->
// "HostVsanSystem:vsanSystem", VsanInternalSystem ->
// "HostVsanInternalSystem:ha-vsan-internal-system") — so
// host.ConfigManager().XSystem(ctx) (object/host_config_manager.go's
// reference() helper) SUCCEEDS, returning that placeholder reference with no
// error. referencia/govmomi/simulator/host_system.go's NewHostSystem then
// only actually instantiates and ctx.Map.Put()s real simulator objects for a
// SUBSET of managers — DatastoreSystem, NetworkSystem, VirtualNicManager,
// AdvancedOption, FirewallSystem, StorageSystem, CertificateManager (lines
// ~89-106) — overwriting their placeholder refs with real, working ones.
// ServiceSystem/DateTimeSystem/VsanSystem/VsanInternalSystem are left
// pointing at their template placeholder, which was never registered as a
// real object. The result: the FIRST real SOAP call against that manager —
// whether a plain property read (HostServiceSystem.Service) or the
// dedicated action method (UpdateVsan_Task, QueryVsanObjectUuidsByFilter,
// ...) — reaches vcsim's generic method dispatcher
// (referencia/govmomi/simulator/simulator.go) and faults server-side with
// "ServerFaultCode: managed object not found: <Type>:<value>" (confirmed
// verbatim in generated_host_misc_test.go's t.Logf output for every one of
// these tools). This is actually STRONGER proof of correct plumbing than the
// originally-guessed client-side short-circuit would have been: every one of
// these 12 tools (5 service + 3 datetime + 1 vsan-update + 3 vsan-internal)
// genuinely reaches the wire and gets a real fault back from vcsim, not a
// local pre-check.
//
// HostVirtualNicManager is the one mixed case: its ConfigManager entry IS
// overwritten with a real, ctx.Map.Put()'d object (host_system.go line ~95:
// "{&hs.ConfigManager.VirtualNicManager, NewHostVirtualNicManager(&hs.HostSystem)}"),
// and referencia/govmomi/simulator/host_vnic_manager.go's
// NewHostVirtualNicManager populates a real Info.NetConfig from
// esx.VirtualNicManagerNetConfig. So vmware_host_nic_info — a plain
// property read via Properties(ctx, ref, []string{"info"}, &vnm), not a
// dedicated SOAP method — gets a genuine functional test against real
// simulator data. vmware_host_nic_select_vnic/deselect_vnic still cannot
// succeed, but for a DIFFERENT reason than the gap above (this manager IS a
// real, registered object): its own SelectVnicForNicType/
// DeselectVnicForNicType handlers were simply never implemented on
// simulator.HostVirtualNicManager (confirmed by the actual fault text: "...
// does not implement: SelectVnicForNicType" / "...DeselectVnicForNicType" —
// vcsim's generic dispatcher finding the object fine, then reporting no
// matching method on it). Separately, for nicType=="vsan" specifically,
// govmomi's own client code (object/host_virtual_nic_manager.go)
// special-cases it and routes through HostVsanSystem.updateVnic instead
// (via ConfigManager().VsanSystem(ctx), which hits the first gap above); the
// test uses a non-"vsan" nicType ("vmotion") to exercise the tools' primary
// real-world path and its own specific "not implemented" fault directly,
// rather than the vsan special case's different gap.
//
// Signature correction (this IS what Fase 3 curation is for): the
// classification report rendered HostVsanInternalSystem.GetVsanObjExtAttrs's
// return type as "*ast.MapType" — a known renderer bug for map/func types in
// this project's codegen pipeline (same class of bug already documented in
// generated_vm_lifecycle.go). The real signature, confirmed by reading
// referencia/govmomi/object/host_vsan_internal_system.go directly, is
// func (m HostVsanInternalSystem) GetVsanObjExtAttrs(ctx context.Context,
// uuids []string) (map[string]VsanObjExtAttrs, error) — VsanObjExtAttrs is a
// small struct defined in that same object.go file (Type/Class/Size/Path/Name
// describing one vSAN DOM object), keyed by object UUID. "*ast.MapType" was
// never a real Go type; it is the AST parser's own node-type name leaking
// through the report generator.
//
// Renames (dropping awkward word repetition the generator's raw
// method-name-derived tool names had):
//   - vmware_host_service_service -> vmware_host_service_list
//   - vmware_host_vsan_internal_query_vsan_object_uuids_by_filter ->
//     vmware_host_vsan_internal_query_object_uuids
//   - vmware_host_vsan_internal_delete_vsan_objects ->
//     vmware_host_vsan_internal_delete_objects
//
// Tier judgment calls:
//   - Service restart/start/stop/update_policy, datetime update/update_config,
//     nic select/deselect_vnic, and vsan_update are all Tier 2 (disruptive but
//     reversible on real vSphere — a stopped service can be started again, a
//     changed clock can be set back, a deselected vnic can be reselected, a
//     vsan host config can be updated back).
//   - vmware_host_vsan_internal_delete_objects is Tier 1: per
//     HostVsanInternalSystem.DeleteVsanObjects's own doc comment
//     ("internal and intended for troubleshooting/debugging only... WARNING:
//     This API can be slow"), it deletes vSAN DOM objects outright — the same
//     irreversibility class as vmware_vm_destroy/snapshot remove, not a
//     reversible config toggle.
//   - vmware_host_service_list, vmware_host_datetime_query,
//     vmware_host_nic_info, and the 2 vsan-internal query/read tools
//     (QueryVsanObjectUuidsByFilter, GetVsanObjExtAttrs) are plain reads —
//     registered via r.register, no tier.
//
// Testing approach, given the vcsim gaps above leave only 1 of 15 tools able
// to reach a real success path: every tool gets (a) a registration/schema
// presence check via ListTools, (b) required-argument validation proven to
// happen before any vCenter round trip (this works regardless of the vcsim
// gaps — it is pure Go arg parsing in the handler), (c) for every
// Tier 1/2 tool, gate-closed and missing-confirm denial proven the same way
// (wrapDestructive runs before the handler), and (d) a well-formed call
// against a real vcsim ESX() host proven to fail with a clean, non-panic
// error reaching real vSphere/vcsim method dispatch — not a success, but
// proof the plumbing (arg decoding, manager resolution, SOAP call shape) is
// correct up to the point vcsim's own coverage gap takes over.
// vmware_host_nic_info additionally gets a real positive-data assertion.
func registerHostMiscTools(r *Registry) {
	hostArg := map[string]interface{}{
		"type":        "string",
		"description": `Host identifier: a name/pattern (e.g. "esxi-01.local") or a full inventory path (e.g. "/ha-datacenter/host/esxi-01.local/esxi-01.local") as returned by vmware_list_hosts. Must resolve to exactly one host.`,
	}
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}
	serviceIDArg := map[string]interface{}{
		"type":        "string",
		"description": `Service key/id, e.g. "ntpd", "sshd", "vpxa" (see vmware_host_service_list).`,
	}
	uuidsArg := map[string]interface{}{
		"type":        "array",
		"items":       map[string]interface{}{"type": "string"},
		"description": "vSAN DOM object UUIDs.",
	}

	// --- HostServiceSystem ----------------------------------------------

	r.register("vmware_host_service_list",
		"List an ESXi host's system services (key, label, running state, activation policy).",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"host": hostArg},
			"required":   []interface{}{"host"},
		},
		Tool{Handler: handleHostServiceList},
	)

	r.registerDestructive("vmware_host_service_restart",
		"Restart a system service on an ESXi host. Reversible (the service can be stopped/started again).",
		tier2,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"host": hostArg, "id": serviceIDArg, "confirm": confirmArg},
			"required":   []interface{}{"host", "id", "confirm"},
		},
		Tool{Handler: handleHostServiceRestart},
	)

	r.registerDestructive("vmware_host_service_start",
		"Start a system service on an ESXi host. Reversible via vmware_host_service_stop.",
		tier2,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"host": hostArg, "id": serviceIDArg, "confirm": confirmArg},
			"required":   []interface{}{"host", "id", "confirm"},
		},
		Tool{Handler: handleHostServiceStart},
	)

	r.registerDestructive("vmware_host_service_stop",
		"Stop a system service on an ESXi host. Reversible via vmware_host_service_start.",
		tier2,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"host": hostArg, "id": serviceIDArg, "confirm": confirmArg},
			"required":   []interface{}{"host", "id", "confirm"},
		},
		Tool{Handler: handleHostServiceStop},
	)

	r.registerDestructive("vmware_host_service_update_policy",
		`Set a system service's activation policy on an ESXi host ("on", "off", or "automatic"). Reversible by setting the policy back.`,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":    hostArg,
				"id":      serviceIDArg,
				"policy":  map[string]interface{}{"type": "string", "enum": []interface{}{"on", "off", "automatic"}, "description": "New activation policy."},
				"confirm": confirmArg,
			},
			"required": []interface{}{"host", "id", "policy", "confirm"},
		},
		Tool{Handler: handleHostServiceUpdatePolicy},
	)

	// --- HostDateTimeSystem ----------------------------------------------

	r.register("vmware_host_datetime_query",
		"Get an ESXi host's current date/time.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"host": hostArg},
			"required":   []interface{}{"host"},
		},
		Tool{Handler: handleHostDateTimeQuery},
	)

	r.registerDestructive("vmware_host_datetime_update",
		"Set an ESXi host's date/time directly. Reversible by setting it back. Prefer vmware_host_datetime_update_config with NTP configured for hosts that should stay in sync automatically.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":    hostArg,
				"date":    map[string]interface{}{"type": "string", "description": `Date/time to set, RFC3339 (e.g. "2026-08-11T12:00:00Z").`},
				"confirm": confirmArg,
			},
			"required": []interface{}{"host", "date", "confirm"},
		},
		Tool{Handler: handleHostDateTimeUpdate},
	)

	r.registerDestructive("vmware_host_datetime_update_config",
		"Set an ESXi host's date/time configuration (time zone, NTP servers, protocol). Reversible by setting the config back.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":    hostArg,
				"config":  map[string]interface{}{"type": "object", "description": "A types.HostDateTimeConfig JSON object matching its Go struct fields (e.g. {\"timeZone\": \"UTC\", \"ntpConfig\": {\"server\": [\"pool.ntp.org\"]}})."},
				"confirm": confirmArg,
			},
			"required": []interface{}{"host", "config", "confirm"},
		},
		Tool{Handler: handleHostDateTimeUpdateConfig},
	)

	// --- HostVirtualNicManager --------------------------------------------

	r.register("vmware_host_nic_info",
		"Get an ESXi host's VirtualNicManager info: which VMkernel NICs are selected for each traffic type (management, vMotion, vSAN, etc.).",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"host": hostArg},
			"required":   []interface{}{"host"},
		},
		Tool{Handler: handleHostNicInfo},
	)

	r.registerDestructive("vmware_host_nic_select_vnic",
		`Select a VMkernel NIC (device) for a given traffic type (nic_type) on an ESXi host, e.g. "vmotion", "management", "vsan". Reversible via vmware_host_nic_deselect_vnic.`,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":     hostArg,
				"nic_type": map[string]interface{}{"type": "string", "description": `Traffic type, e.g. "vmotion", "management", "vsan", "faultToleranceLogging".`},
				"device":   map[string]interface{}{"type": "string", "description": `VMkernel NIC device name, e.g. "vmk0".`},
				"confirm":  confirmArg,
			},
			"required": []interface{}{"host", "nic_type", "device", "confirm"},
		},
		Tool{Handler: handleHostNicSelectVnic},
	)

	r.registerDestructive("vmware_host_nic_deselect_vnic",
		"Deselect a VMkernel NIC (device) from a given traffic type (nic_type) on an ESXi host. Reversible via vmware_host_nic_select_vnic.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":     hostArg,
				"nic_type": map[string]interface{}{"type": "string", "description": `Traffic type, e.g. "vmotion", "management", "vsan", "faultToleranceLogging".`},
				"device":   map[string]interface{}{"type": "string", "description": `VMkernel NIC device name, e.g. "vmk0".`},
				"confirm":  confirmArg,
			},
			"required": []interface{}{"host", "nic_type", "device", "confirm"},
		},
		Tool{Handler: handleHostNicDeselectVnic},
	)

	// --- HostVsanSystem ----------------------------------------------------

	r.registerDestructive("vmware_host_vsan_update",
		"Update an ESXi host's vSAN configuration (enable/disable, cluster/network/storage info). Reversible by updating it back.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":    hostArg,
				"config":  map[string]interface{}{"type": "object", "description": "A types.VsanHostConfigInfo JSON object matching its Go struct fields (e.g. {\"enabled\": true})."},
				"confirm": confirmArg,
			},
			"required": []interface{}{"host", "config", "confirm"},
		},
		Tool{Handler: handleHostVsanUpdate},
	)

	// --- HostVsanInternalSystem ---------------------------------------------

	r.register("vmware_host_vsan_internal_query_object_uuids",
		"Query vSAN DOM object UUIDs on an ESXi host, optionally filtered by a list of UUIDs. Internal/troubleshooting API.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":    hostArg,
				"uuids":   map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "UUIDs to filter by. Omit/empty for no filter (server-defined semantics)."},
				"limit":   map[string]interface{}{"type": "integer", "description": "Maximum number of results. Default 0."},
				"version": map[string]interface{}{"type": "integer", "description": "Query filter format version. Default 0."},
			},
			"required": []interface{}{"host"},
		},
		Tool{Handler: handleHostVsanInternalQueryObjectUUIDs},
	)

	r.register("vmware_host_vsan_internal_get_obj_ext_attrs",
		"Get extended attributes (type, class, size, path, friendly name) for vSAN DOM objects on an ESXi host, by UUID. Internal/troubleshooting API — WARNING: can be slow, reads every object.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":  hostArg,
				"uuids": uuidsArg,
			},
			"required": []interface{}{"host", "uuids"},
		},
		Tool{Handler: handleHostVsanInternalGetObjExtAttrs},
	)

	r.registerDestructive("vmware_host_vsan_internal_delete_objects",
		"Delete vSAN DOM objects on an ESXi host by UUID. Internal/troubleshooting API only — WARNING: irreversible, deletes the objects outright (not a reversible config change like the other tools in this file).",
		tier1,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":    hostArg,
				"uuids":   uuidsArg,
				"force":   map[string]interface{}{"type": "boolean", "description": "Force-delete objects that have lost quorum (normally inaccessible). Optional, default false."},
				"confirm": confirmArg,
			},
			"required": []interface{}{"host", "uuids", "confirm"},
		},
		Tool{Handler: handleHostVsanInternalDeleteObjects},
	)
}

// --- HostServiceSystem handlers --------------------------------------------

func handleHostServiceList(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}

	ss, err := host.ConfigManager().ServiceSystem(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get service system for %s: %w", host.InventoryPath, err)
	}

	services, err := ss.Service(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to list services on %s: %w", host.InventoryPath, err)
	}

	result := make([]map[string]interface{}, 0, len(services))
	for _, s := range services {
		result = append(result, map[string]interface{}{
			"key":      s.Key,
			"label":    s.Label,
			"required": s.Required,
			"running":  s.Running,
			"policy":   s.Policy,
		})
	}

	return marshalJSON(map[string]interface{}{
		"host":     host.InventoryPath,
		"count":    len(result),
		"services": result,
	})
}

func handleHostServiceRestart(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, id, err := resolveHostAndServiceID(ctx, client, args)
	if err != nil {
		return "", err
	}

	ss, err := host.ConfigManager().ServiceSystem(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get service system for %s: %w", host.InventoryPath, err)
	}
	if err := ss.Restart(ctx, id); err != nil {
		return "", fmt.Errorf("failed to restart service %q on %s: %w", id, host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "id": id, "result": "restarted"})
}

func handleHostServiceStart(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, id, err := resolveHostAndServiceID(ctx, client, args)
	if err != nil {
		return "", err
	}

	ss, err := host.ConfigManager().ServiceSystem(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get service system for %s: %w", host.InventoryPath, err)
	}
	if err := ss.Start(ctx, id); err != nil {
		return "", fmt.Errorf("failed to start service %q on %s: %w", id, host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "id": id, "result": "started"})
}

func handleHostServiceStop(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, id, err := resolveHostAndServiceID(ctx, client, args)
	if err != nil {
		return "", err
	}

	ss, err := host.ConfigManager().ServiceSystem(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get service system for %s: %w", host.InventoryPath, err)
	}
	if err := ss.Stop(ctx, id); err != nil {
		return "", fmt.Errorf("failed to stop service %q on %s: %w", id, host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "id": id, "result": "stopped"})
}

func handleHostServiceUpdatePolicy(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, id, err := resolveHostAndServiceID(ctx, client, args)
	if err != nil {
		return "", err
	}
	policy, _ := args["policy"].(string)
	if policy == "" {
		return "", fmt.Errorf("policy is required")
	}

	ss, err := host.ConfigManager().ServiceSystem(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get service system for %s: %w", host.InventoryPath, err)
	}
	if err := ss.UpdatePolicy(ctx, id, policy); err != nil {
		return "", fmt.Errorf("failed to update policy for service %q on %s: %w", id, host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "id": id, "policy": policy, "result": "policy_updated"})
}

// resolveHostAndServiceID resolves the "host" and "id" arguments shared by
// every HostServiceSystem action tool (Restart/Start/Stop/UpdatePolicy).
func resolveHostAndServiceID(ctx context.Context, client *vmware.Client, args map[string]interface{}) (host *object.HostSystem, id string, err error) {
	host, err = resolveHost(ctx, client, args)
	if err != nil {
		return nil, "", err
	}
	id, _ = args["id"].(string)
	if id == "" {
		return nil, "", fmt.Errorf("id is required")
	}
	return host, id, nil
}

// --- HostDateTimeSystem handlers --------------------------------------------

func handleHostDateTimeQuery(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}

	dts, err := host.ConfigManager().DateTimeSystem(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get date/time system for %s: %w", host.InventoryPath, err)
	}

	t, err := dts.Query(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to query date/time on %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "date_time": t.Format(time.RFC3339)})
}

func handleHostDateTimeUpdate(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}
	dateStr, _ := args["date"].(string)
	if dateStr == "" {
		return "", fmt.Errorf("date is required")
	}
	date, err := time.Parse(time.RFC3339, dateStr)
	if err != nil {
		return "", fmt.Errorf("invalid date (must be RFC3339): %w", err)
	}

	dts, err := host.ConfigManager().DateTimeSystem(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get date/time system for %s: %w", host.InventoryPath, err)
	}
	if err := dts.Update(ctx, date); err != nil {
		return "", fmt.Errorf("failed to update date/time on %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "date": dateStr, "result": "updated"})
}

func handleHostDateTimeUpdateConfig(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}
	raw, ok := args["config"]
	if !ok {
		return "", fmt.Errorf("config is required")
	}
	var cfg types.HostDateTimeConfig
	if err := decodeJSONArg(raw, &cfg); err != nil {
		return "", fmt.Errorf("invalid config: %w", err)
	}

	dts, err := host.ConfigManager().DateTimeSystem(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get date/time system for %s: %w", host.InventoryPath, err)
	}
	if err := dts.UpdateConfig(ctx, cfg); err != nil {
		return "", fmt.Errorf("failed to update date/time config on %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "result": "config_updated"})
}

// --- HostVirtualNicManager handlers ------------------------------------------

func handleHostNicInfo(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}

	vnm, err := host.ConfigManager().VirtualNicManager(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get virtual NIC manager for %s: %w", host.InventoryPath, err)
	}

	info, err := vnm.Info(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to read virtual NIC manager info for %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "info": info})
}

func handleHostNicSelectVnic(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, nicType, device, err := resolveHostNicTypeAndDevice(ctx, client, args)
	if err != nil {
		return "", err
	}

	vnm, err := host.ConfigManager().VirtualNicManager(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get virtual NIC manager for %s: %w", host.InventoryPath, err)
	}
	if err := vnm.SelectVnic(ctx, nicType, device); err != nil {
		return "", fmt.Errorf("failed to select vnic %q for nic type %q on %s: %w", device, nicType, host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "nic_type": nicType, "device": device, "result": "vnic_selected"})
}

func handleHostNicDeselectVnic(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, nicType, device, err := resolveHostNicTypeAndDevice(ctx, client, args)
	if err != nil {
		return "", err
	}

	vnm, err := host.ConfigManager().VirtualNicManager(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get virtual NIC manager for %s: %w", host.InventoryPath, err)
	}
	if err := vnm.DeselectVnic(ctx, nicType, device); err != nil {
		return "", fmt.Errorf("failed to deselect vnic %q for nic type %q on %s: %w", device, nicType, host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "nic_type": nicType, "device": device, "result": "vnic_deselected"})
}

// resolveHostNicTypeAndDevice resolves the "host", "nic_type" and "device"
// arguments shared by vmware_host_nic_select_vnic/deselect_vnic.
func resolveHostNicTypeAndDevice(ctx context.Context, client *vmware.Client, args map[string]interface{}) (host *object.HostSystem, nicType, device string, err error) {
	host, err = resolveHost(ctx, client, args)
	if err != nil {
		return nil, "", "", err
	}
	nicType, _ = args["nic_type"].(string)
	if nicType == "" {
		return nil, "", "", fmt.Errorf("nic_type is required")
	}
	device, _ = args["device"].(string)
	if device == "" {
		return nil, "", "", fmt.Errorf("device is required")
	}
	return host, nicType, device, nil
}

// --- HostVsanSystem handlers -------------------------------------------------

func handleHostVsanUpdate(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}
	raw, ok := args["config"]
	if !ok {
		return "", fmt.Errorf("config is required")
	}
	var cfg types.VsanHostConfigInfo
	if err := decodeJSONArg(raw, &cfg); err != nil {
		return "", fmt.Errorf("invalid config: %w", err)
	}

	vs, err := host.ConfigManager().VsanSystem(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get vsan system for %s: %w", host.InventoryPath, err)
	}

	task, err := vs.Update(ctx, cfg)
	if err != nil {
		return "", fmt.Errorf("failed to update vsan config on %s: %w", host.InventoryPath, err)
	}
	if err := waitForTask(ctx, task); err != nil {
		return "", fmt.Errorf("update-vsan task failed for %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "result": "vsan_updated"})
}

// --- HostVsanInternalSystem handlers ------------------------------------------

func handleHostVsanInternalQueryObjectUUIDs(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}

	var uuids []string
	if raw, ok := args["uuids"]; ok {
		uuids, err = toStringSlice(raw)
		if err != nil {
			return "", fmt.Errorf("invalid uuids: %w", err)
		}
	}
	var limit, version int32
	if v, ok := args["limit"]; ok {
		limit, err = toInt32(v)
		if err != nil {
			return "", fmt.Errorf("invalid limit: %w", err)
		}
	}
	if v, ok := args["version"]; ok {
		version, err = toInt32(v)
		if err != nil {
			return "", fmt.Errorf("invalid version: %w", err)
		}
	}

	vis, err := host.ConfigManager().VsanInternalSystem(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get vsan internal system for %s: %w", host.InventoryPath, err)
	}

	result, err := vis.QueryVsanObjectUuidsByFilter(ctx, uuids, limit, version)
	if err != nil {
		return "", fmt.Errorf("failed to query vsan object uuids on %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "count": len(result), "uuids": result})
}

func handleHostVsanInternalGetObjExtAttrs(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}
	raw, ok := args["uuids"]
	if !ok {
		return "", fmt.Errorf("uuids is required")
	}
	uuids, err := toStringSlice(raw)
	if err != nil {
		return "", fmt.Errorf("invalid uuids: %w", err)
	}

	vis, err := host.ConfigManager().VsanInternalSystem(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get vsan internal system for %s: %w", host.InventoryPath, err)
	}

	attrs, err := vis.GetVsanObjExtAttrs(ctx, uuids)
	if err != nil {
		return "", fmt.Errorf("failed to get vsan object ext attrs on %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "count": len(attrs), "attrs": attrs})
}

func handleHostVsanInternalDeleteObjects(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}
	raw, ok := args["uuids"]
	if !ok {
		return "", fmt.Errorf("uuids is required")
	}
	uuids, err := toStringSlice(raw)
	if err != nil {
		return "", fmt.Errorf("invalid uuids: %w", err)
	}
	if len(uuids) == 0 {
		return "", fmt.Errorf("uuids must be a non-empty array")
	}
	var forcePtr *bool
	if v, ok := args["force"].(bool); ok {
		forcePtr = &v
	}

	vis, err := host.ConfigManager().VsanInternalSystem(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get vsan internal system for %s: %w", host.InventoryPath, err)
	}

	results, err := vis.DeleteVsanObjects(ctx, uuids, forcePtr)
	if err != nil {
		return "", fmt.Errorf("failed to delete vsan objects on %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "count": len(results), "results": results})
}

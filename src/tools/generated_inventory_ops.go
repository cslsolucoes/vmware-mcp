package tools

import (
	"context"
	"fmt"
	"reflect"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// registerInventoryOpsTools / registerInventoryOpsVCenterOnlyTools close out
// 3 small gaps left after generated_inventory_folder.go and
// generated_alarm.go: the 4 Datacenter methods that don't have an
// object.Datacenter wrapper (QueryConnectionInfo, QueryConnectionInfoViaSpec,
// ReconfigureDatacenter_Task, QueryDatacenterConfigOptionDescriptor — dialed
// raw via vim25/methods, same as generated_alarm.go's AlarmManager family),
// plus 2 whole managed-object families that had NO tools at all yet:
// ScheduledTaskManager+ScheduledTask (7 methods) and EventManager (5
// methods). Every method name below was confirmed against the real
// D:\Users\claiton.linhares\go\pkg\mod\github.com\vmware\govmomi@v0.55.1
// module cache's vim25/methods/methods.go + vim25/types/types.go — not
// invented, not trusted from a brief. object.Datacenter itself
// (referencia-equivalent govmomi@v0.55.1/object/datacenter.go) has ONLY
// Folders/Destroy/PowerOnVM — all 3 already wired by
// generated_inventory_folder.go (vmware_datacenter_folders/_destroy/
// _power_on_vm) — so PowerOnMultiVM_Task is intentionally NOT repeated here.
//
// # "This" targets (3 different managed objects, confirmed per method)
//
//   - QueryConnectionInfo/QueryConnectionInfoViaSpec/ReconfigureDatacenter_Task/
//     QueryDatacenterConfigOptionDescriptor: This is the Datacenter itself
//     (QueryConnectionInfoViaSpecRequestType's own doc comment says "See
//     Datacenter.QueryConnectionInfo" — confirms the sibling method's
//     receiver type too). Resolved via resolveDatacenter
//     (generated_datastore_browser.go's existing helper — reused, not
//     duplicated, same as generated_inventory_folder.go already does).
//
//   - CreateScheduledTask/CreateObjectScheduledTask/RetrieveObjectScheduledTask/
//     RetrieveEntityScheduledTask: This is the ScheduledTaskManager
//     singleton MoRef (client.Client.ServiceContent.ScheduledTaskManager) —
//     schedManagerRef below, same "raw MoRef off ServiceContent" shape as
//     generated_alarm.go's alarmManagerRef. ReconfigureScheduledTask/
//     RemoveScheduledTask/RunScheduledTask: This is the individual
//     ScheduledTask object's own MoRef (Type "ScheduledTask"), built from a
//     bare string value by schedTaskRefArg — exactly
//     generated_alarm.go's alarmRefArg convention for Alarm/Type "Alarm".
//
//   - QueryEvents/CreateCollectorForEvents/LogUserEvent/PostEvent/
//     RetrieveArgumentDescription: This is the EventManager singleton MoRef
//     (client.Client.ServiceContent.EventManager) — eventManagerRef below.
//
// # Class, decided by evidence (not assumption), same discipline as
// generated_alarm.go's own class note:
//
//   - Datacenter's 4 extra methods: modeVSphereGeneral, matching every
//     sibling Datacenter tool already in generated_inventory_folder.go — the
//     Datacenter object itself exists under both connection types (a
//     standalone ESXi host has its own default "ha-datacenter").
//
//   - ScheduledTaskManager: modeVCenterOnly.
//     referencia-equivalent govmomi@v0.55.1/simulator/esx/service_content.go
//     sets ScheduledTaskManager to a nil *ManagedObjectReference; only
//     simulator/vpx/service_content.go populates a real one
//     ({"ScheduledTaskManager","ScheduledTaskManager"}) — grepped directly,
//     not assumed. A standalone-ESXi connection has no ScheduledTaskManager
//     to call at all.
//
//   - EventManager: modeVSphereGeneral — checked exactly because the task
//     brief flagged it as worth checking rather than assuming vcenter-only
//     like ScheduledTaskManager. Both simulator/esx/service_content.go AND
//     simulator/vpx/service_content.go populate a real EventManager MoRef
//     ("ha-eventmgr" / "EventManager" respectively) — confirmed by grep, not
//     assumed. Real standalone ESXi hosts do expose a (locally-scoped)
//     EventManager too. Split into its own vsphere-general register
//     function (registerInventoryOpsTools) alongside the Datacenter tools,
//     separate from registerInventoryOpsVCenterOnlyTools's
//     ScheduledTaskManager family — same "2 register functions, 1 per class,
//     in one domain file" shape as registry.go's existing
//     registerInventoryComputeTools/registerInventoryComputeVCenterOnlyTools
//     pair and registerVMProvisioningTools/
//     registerVMProvisioningVCenterOnlyTools pair.
//
// # Reused polymorphic-field decoders (SSOT — not duplicated)
//
// ScheduledTaskSpec.Action is types.BaseAction — the exact same interface
// AlarmTriggeringAction.Action already decodes via generated_alarm.go's
// alarmBuildLeafAction/alarmLeafActionTypes (MethodAction/SendEmailAction/
// SendSNMPAction/RunScriptAction/CreateTaskAction) — reused here unchanged
// rather than re-implementing the same "_vimType" leaf-action router.
// alarmResolveEntity/alarmResolveOptionalEntity (generated_alarm.go) are
// likewise reused as-is for every "target a ManagedEntity/ManagedObject via
// entity{type,value} JSON, or the vm/host convenience shortcuts" argument
// below (LogUserEvent's required entity, CreateScheduledTask's required
// entity, CreateObjectScheduledTask's required — but broader — obj,
// RetrieveEntityScheduledTask/RetrieveObjectScheduledTask's optional
// entity/obj) — CreateObjectScheduledTask's Obj can technically be ANY
// managed object, a strictly wider target set than alarmResolveEntity's
// ManagedEntity-flavored resolution, but the {entity}/{vm}/{host} argument
// shape and resolution logic are identical, so reusing it (with an accurate
// "any managed object" doc string on the schema, not the resolver) avoids
// duplicating a whole entity-resolution helper for a difference that only
// matters for the (rare, advanced) explicit "entity" MoRef escape hatch —
// the caller is free to pass any {"type":...,"value":...} there. The
// alarmTypeRegistry map type (also generated_alarm.go) is reused verbatim
// for this file's own schedSchedulerTypes registry below (ScheduledTaskSpec.
// Scheduler is types.BaseTaskScheduler — a DIFFERENT polymorphic interface
// with no existing decoder anywhere in this package, so schedBuildScheduler
// is new, but the registry container type itself is not re-invented).
//
// # Polymorphic field new to this file: ScheduledTaskSpec.Scheduler
// (types.BaseTaskScheduler)
//
// Scoped to the 7 concrete leaf types vSphere's own API guide documents as
// the real choices (confirmed by reading each struct's embedding chain in
// vim25/types/types.go, not assumed): OnceTaskScheduler,
// AfterStartupTaskScheduler, HourlyTaskScheduler, DailyTaskScheduler,
// WeeklyTaskScheduler, MonthlyByDayTaskScheduler,
// MonthlyByWeekdayTaskScheduler. RecurrentTaskScheduler and
// MonthlyTaskScheduler are both abstract-in-practice intermediate embeds (no
// discriminating field of their own beyond what their leaf subtypes add) —
// same "finite set of concrete implementers, not the whole embedding tree"
// curation generated_alarm.go's alarmExpressionTypes/alarmActionTypes/
// alarmLeafActionTypes already applied.
//
// # PostEvent's EventToPost (types.BaseEvent) scope
//
// types.BaseEvent has dozens of concrete implementers (every predefined
// vSphere event type) — this tool intentionally supports only
// types.EventEx, the one implementer vSphere's own API guide documents as
// the generic/custom vehicle for posting a caller-defined event (it carries
// its own eventTypeId/message/severity/arguments fields instead of being
// tied to one hardcoded event class) — same "don't build a giant polymorphic
// registry for a field whose real usage is one well-known generic subtype"
// judgment call as this file's Datacenter/EventManager scope decisions.
// PostEventRequestType.TaskInfo (*types.TaskInfo, optional) is NOT exposed —
// TaskInfo.Reason is itself a different polymorphic interface
// (types.BaseTaskReason) with no caller-facing value for a synthetic
// "caller-posted" event; omitted, not a decode target.
//
// # vcsim coverage (confirmed by grepping
// govmomi@v0.55.1/simulator/*.go — not assumed)
//
// Datacenter's 4 extra methods: NO simulator handler at all for any of them
// (grepped simulator/datacenter.go and the whole simulator/ tree) — all 4
// reach vcsim's generic dispatch fallback and fault cleanly
// (assertReachesServer, reusing generated_vm_lifecycle_test.go's helper).
//
// ScheduledTaskManager: simulator/ has NO scheduled_task*.go file and no
// ScheduledTaskManager/ScheduledTask receiver anywhere (grepped) — all 7
// tools are assertReachesServer against simulator.VPX() (the class that
// actually exposes the MoRef to try).
//
// EventManager: simulator/event_manager.go implements a real (if partial)
// EventManager — QueryEvents, CreateCollectorForEvents, and PostEvent all
// have working handlers (confirmed by reading the file), so those 3 get
// assertRealSuccess-style behavioral tests. LogUserEvent and
// RetrieveArgumentDescription have NO handler on
// referencia-equivalent simulator.EventManager (grepped) — both are
// assertReachesServer. Note QueryEvents' own simulator handler explicitly
// faults types.NotImplemented when ctx.Map.IsESX() — its RealSuccess test
// therefore runs against simulator.VPX(), not ESX(), even though the tool
// itself is modeVSphereGeneral and registers under both.
//
// # Tiers
//
// Read-only (r.register, no confirm): vmware_datacenter_query_connection_info,
// vmware_datacenter_query_connection_info_via_spec,
// vmware_datacenter_query_config_option_descriptor,
// vmware_scheduledtask_retrieve_for_object,
// vmware_scheduledtask_retrieve_for_entity, vmware_event_query,
// vmware_event_retrieve_argument_description. Tier2 (disruptive, reversible):
// vmware_datacenter_reconfigure (reconfigure again to undo),
// vmware_scheduledtask_create, vmware_scheduledtask_create_object (both
// undone by _remove), vmware_scheduledtask_reconfigure (reconfigure again),
// vmware_scheduledtask_run (re-running a task is not itself destructive of
// the task DEFINITION — its ACTION might be, same "the tool wraps the
// dispatch, not the arbitrary downstream effect" posture as every other
// _run/_create tool in this project), vmware_event_create_collector,
// vmware_event_log_user_event, vmware_event_post. Tier1 (irreversible):
// vmware_scheduledtask_remove — deletes the scheduled task definition
// itself, same "Remove = tier1" rule generated_alarm.go's
// vmware_alarm_remove already follows.
func registerInventoryOpsTools(r *Registry) {
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}
	datacenterArg := map[string]interface{}{
		"type":        "string",
		"description": `Datacenter identifier: a name/pattern (e.g. "DC0") or a full inventory path, as returned by vmware_list_datacenters. Must resolve to exactly one datacenter.`,
	}
	entityObjArg := map[string]interface{}{
		"type":        "object",
		"description": `Explicit MoRef of the target managed object/entity, e.g. {"type": "ClusterComputeResource", "value": "domain-c7"} — use this to target a kind vmware_list_vms/vmware_list_hosts can't resolve directly (cluster, datacenter, resource pool, datastore, network, folder, vApp...). Alternative to "vm"/"host".`,
		"properties": map[string]interface{}{
			"type":  map[string]interface{}{"type": "string", "description": `Managed object type, e.g. "VirtualMachine", "HostSystem", "ClusterComputeResource", "Datacenter", "ResourcePool", "Folder".`},
			"value": map[string]interface{}{"type": "string", "description": "The object's MoRef value."},
		},
	}
	vmEntityArg := map[string]interface{}{
		"type":        "string",
		"description": `VM identifier: a name/pattern or inventory path, as returned by vmware_list_vms. Alternative to "entity"/"host".`,
	}
	hostEntityArg := map[string]interface{}{
		"type":        "string",
		"description": `Host identifier: a name/pattern or inventory path, as returned by vmware_list_hosts. Alternative to "entity"/"vm".`,
	}
	eventFilterArg := map[string]interface{}{
		"type": "object",
		"description": `Optional types.EventFilterSpec JSON object matching its Go struct fields — all fields optional, an empty/omitted filter matches every visible event. "entity": {"entity":{"type":"...","value":"..."},"recursion":"self"|"children"|"all"}. "time": {"beginTime":"RFC3339","endTime":"RFC3339"}. "userName": {"userList":["..."],"systemUser":bool}. "eventChainId": int32. "alarm"/"scheduledTask": {"type":"Alarm"|"ScheduledTask","value":"..."}. "category": array of severity strings ("info"|"warning"|"error"|"user"). "eventTypeId": array of event type-id strings (e.g. "VmPoweredOnEvent", or a custom EventEx eventTypeId). "maxCount": int32 cap on returned events.`,
	}

	// --- Datacenter: read-only -------------------------------------------

	r.register("vmware_datacenter_query_connection_info",
		"Probe whether a host (by hostname/port/credentials) can be added to this datacenter, without adding it — returns the host's summary, VM list, and whether it's already managed by another vCenter. Performs a real network connection to the target host to answer the question; does not modify inventory.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"datacenter":      datacenterArg,
				"hostname":        map[string]interface{}{"type": "string", "description": "Target host's hostname or IP address. Required."},
				"port":            map[string]interface{}{"type": "integer", "description": "Target port. ESX 3.x+/VMware Server hosts use the https port (443 by default); pass -1 to let the server try the default. Required."},
				"username":        map[string]interface{}{"type": "string", "description": "Username to authenticate against the target host. Required."},
				"password":        map[string]interface{}{"type": "string", "description": "Password to authenticate against the target host. Required."},
				"ssl_thumbprint":  map[string]interface{}{"type": "string", "description": "Optional expected SSL thumbprint of the host's certificate."},
				"ssl_certificate": map[string]interface{}{"type": "string", "description": "Optional expected SSL certificate of the host in PEM format — a fallback when ssl_thumbprint can't be verified via a trusted CA. Mutually exclusive with ssl_thumbprint."},
			},
			"required": []interface{}{"datacenter", "hostname", "port", "username", "password"},
		},
		Tool{Handler: handleDatacenterQueryConnectionInfo},
	)

	r.register("vmware_datacenter_query_connection_info_via_spec",
		"Same probe as vmware_datacenter_query_connection_info_via_spec's sibling vmware_datacenter_query_connection_info, but driven by a full HostConnectSpec instead of separate hostname/port/username/password arguments.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"datacenter": datacenterArg,
				"spec":       map[string]interface{}{"type": "object", "description": `A types.HostConnectSpec JSON object matching its Go struct fields — "hostName"/"userName"/"password" are required for a real connection attempt to succeed. Required.`},
			},
			"required": []interface{}{"datacenter", "spec"},
		},
		Tool{Handler: handleDatacenterQueryConnectionInfoViaSpec},
	)

	r.register("vmware_datacenter_query_config_option_descriptor",
		"List the virtual hardware version options (VirtualMachineConfigOptionDescriptor) selectable as this datacenter's default/maximum hardware version.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"datacenter": datacenterArg},
			"required":   []interface{}{"datacenter"},
		},
		Tool{Handler: handleDatacenterQueryConfigOptionDescriptor},
	)

	// --- Datacenter: Tier 2 (disruptive but reversible) -------------------

	r.registerDestructive("vmware_datacenter_reconfigure",
		"Change a datacenter's default/maximum virtual hardware version (DatacenterConfigSpec). Reversible by reconfiguring again with the previous keys.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"datacenter": datacenterArg,
				"spec":       map[string]interface{}{"type": "object", "description": `A types.DatacenterConfigSpec JSON object matching its Go struct fields — "defaultHardwareVersionKey"/"maximumHardwareVersionKey" (both optional strings, e.g. "vmx-19"; see vmware_datacenter_query_config_option_descriptor's "key" field for valid values). Required.`},
				"modify":     map[string]interface{}{"type": "boolean", "description": "If true (default false), spec is applied incrementally (unset fields keep their current value); if false, the datacenter's config matches spec exactly (unset fields reset to default)."},
				"confirm":    confirmArg,
			},
			"required": []interface{}{"datacenter", "spec", "confirm"},
		},
		Tool{Handler: handleDatacenterReconfigure},
	)

	// --- EventManager: read-only -------------------------------------------

	r.register("vmware_event_query",
		"Query the event history for events matching a filter (by entity/time/user/category/type). One-shot — returns the latest page of matches directly, no separate collector object to manage (use vmware_event_create_collector for paged/live collection instead).",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"filter": eventFilterArg},
		},
		Tool{Handler: handleEventQuery},
	)

	r.register("vmware_event_retrieve_argument_description",
		"Get the argument descriptions (name/type per positional argument) for a given event type id — useful to interpret an EventEx/ExtendedEvent's raw Arguments array.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"event_type_id": map[string]interface{}{"type": "string", "description": `The event type id, e.g. "com.vmware.applmgmt.MyEvent" or a predefined event class name like "VmPoweredOnEvent". Required.`},
			},
			"required": []interface{}{"event_type_id"},
		},
		Tool{Handler: handleEventRetrieveArgumentDescription},
	)

	// --- EventManager: Tier 2 (disruptive but reversible) -------------------

	r.registerDestructive("vmware_event_create_collector",
		"Create a server-side EventHistoryCollector scoped to a filter, for paged/incremental event retrieval. Reversible in the sense that the collector is a disposable server-side resource (there is a per-vCenter cap on how many can exist at once — MaxCollector); this project has no separate tool yet to read pages from or destroy the returned collector — the moref is returned for advanced/future use.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"filter":  map[string]interface{}{"type": "object", "description": eventFilterArg["description"].(string) + ` Required (may be an empty object {} to match every event).`},
				"confirm": confirmArg,
			},
			"required": []interface{}{"filter", "confirm"},
		},
		Tool{Handler: handleEventCreateCollector},
	)

	r.registerDestructive("vmware_event_log_user_event",
		"Log a simple user-defined event (just a message string) against an entity. For a fully custom event with its own type id/severity/arguments, use vmware_event_post instead.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"entity":  entityObjArg,
				"vm":      vmEntityArg,
				"host":    hostEntityArg,
				"msg":     map[string]interface{}{"type": "string", "description": "The message to log. Required."},
				"confirm": confirmArg,
			},
			"required": []interface{}{"msg", "confirm"},
		},
		Tool{Handler: handleEventLogUserEvent},
	)

	r.registerDestructive("vmware_event_post",
		"Post a fully custom event (types.EventEx) into vCenter/ESXi's event history — visible to vmware_event_query, alarms, and any other event consumer.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"event": map[string]interface{}{
					"type":        "object",
					"description": `A types.EventEx JSON object matching its Go struct fields — "eventTypeId" (string, required) identifies the custom event type; "message" (string), "severity" ("info"|"warning"|"error"|"user", default info), "arguments" (array of {"key":"...","value":"..."}), "objectId"/"objectType"/"objectName" (the affected object, e.g. objectType:"VirtualMachine"). Server-managed fields (key/chainId/createdTime/userName) are ignored if set — the server assigns real values on post. The full range of predefined vSphere event types (VmPoweredOnEvent, etc.) is NOT supported here — EventEx is the one generic/custom vehicle this tool exposes.`,
				},
				"confirm": confirmArg,
			},
			"required": []interface{}{"event", "confirm"},
		},
		Tool{Handler: handleEventPost},
	)
}

// registerInventoryOpsVCenterOnlyTools is this file's ScheduledTaskManager/
// ScheduledTask family — see this file's top doc comment for why it's split
// from registerInventoryOpsTools into its own vcenter-only register
// function.
func registerInventoryOpsVCenterOnlyTools(r *Registry) {
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}
	entityObjArg := map[string]interface{}{
		"type":        "object",
		"description": `Explicit MoRef of the target managed object/entity, e.g. {"type": "ClusterComputeResource", "value": "domain-c7"} — use this to target a kind vmware_list_vms/vmware_list_hosts can't resolve directly. Alternative to "vm"/"host".`,
		"properties": map[string]interface{}{
			"type":  map[string]interface{}{"type": "string", "description": `Managed object type, e.g. "VirtualMachine", "HostSystem", "ClusterComputeResource", "Datacenter", "ResourcePool", "Folder".`},
			"value": map[string]interface{}{"type": "string", "description": "The object's MoRef value."},
		},
	}
	vmEntityArg := map[string]interface{}{
		"type":        "string",
		"description": `VM identifier: a name/pattern or inventory path, as returned by vmware_list_vms. Alternative to "entity"/"host".`,
	}
	hostEntityArg := map[string]interface{}{
		"type":        "string",
		"description": `Host identifier: a name/pattern or inventory path, as returned by vmware_list_hosts. Alternative to "entity"/"vm".`,
	}
	scheduledTaskIDArg := map[string]interface{}{
		"type":        "string",
		"description": `The scheduled task's MoRef value (Type is always "ScheduledTask"), as returned by vmware_scheduledtask_create/_create_object's "scheduled_task" field or vmware_scheduledtask_retrieve_for_entity/_for_object's entries.`,
	}
	schedSpecArg := map[string]interface{}{
		"type": "object",
		"description": `A types.ScheduledTaskSpec JSON object matching its Go struct fields — "name" (string, required), "enabled" (bool, required), "scheduler" (object, required, see below), "action" (object, required, see below), "description" (string), "notification" (string, email address for completion notification, empty = none).

"scheduler" is vSphere's polymorphic (xsi:type) TaskScheduler field — encode the concrete type as a JSON object carrying a "_vimType" discriminator naming the exact govmomi struct. All variants also accept optional "activeTime"/"expireTime" (RFC3339 timestamps; activeTime defaults to creation time, expireTime defaults to never).
  scheduler._vimType one of:
    "OnceTaskScheduler"             {"runAt": "RFC3339 timestamp, optional — omit to run immediately once activated"}
    "AfterStartupTaskScheduler"     {"minute": int32 — delay in minutes after vCenter Server restarts}
    "HourlyTaskScheduler"           {"interval": int32 (1-999, default 1), "minute": int32 (0-59, UTC)}
    "DailyTaskScheduler"            {"interval": int32, "minute": int32, "hour": int32 (0-23, UTC)}
    "WeeklyTaskScheduler"           {"interval": int32, "minute": int32, "hour": int32, "sunday".."saturday": bool (at least one must be true)}
    "MonthlyByDayTaskScheduler"     {"interval": int32, "minute": int32, "hour": int32, "day": int32 (1-31)}
    "MonthlyByWeekdayTaskScheduler" {"interval": int32, "minute": int32, "hour": int32, "offset": "first"|"second"|"third"|"fourth"|"last", "weekday": "sunday".."saturday"}

"action" is the same BaseAction leaf shape vmware_alarm_create's spec.action.action uses (_vimType one of "SendEmailAction"|"SendSNMPAction"|"RunScriptAction"|"CreateTaskAction"|"MethodAction") — see vmware_alarm_create's description for each leaf's fields.`,
	}

	// --- ScheduledTaskManager: read-only -----------------------------------

	r.register("vmware_scheduledtask_retrieve_for_object",
		"List every scheduled task defined on a given managed object (or on every visible object if none given).",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"entity": entityObjArg,
				"vm":     vmEntityArg,
				"host":   hostEntityArg,
			},
		},
		Tool{Handler: handleScheduledTaskRetrieveForObject},
	)

	r.register("vmware_scheduledtask_retrieve_for_entity",
		"List every scheduled task defined on a given managed entity, including ones inherited from its ancestor folder/datacenter/compute-resource/resource-pool (or on every visible entity if none given).",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"entity": entityObjArg,
				"vm":     vmEntityArg,
				"host":   hostEntityArg,
			},
		},
		Tool{Handler: handleScheduledTaskRetrieveForEntity},
	)

	// --- ScheduledTaskManager: Tier 2 (disruptive but reversible) ----------

	r.registerDestructive("vmware_scheduledtask_create",
		"Create a new scheduled task on a managed entity (VM, host, cluster, datacenter, folder, or resource pool — the action applies to every VM/host descendant if the entity isn't a leaf). Reversible via vmware_scheduledtask_remove.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"entity":  entityObjArg,
				"vm":      vmEntityArg,
				"host":    hostEntityArg,
				"spec":    schedSpecArg,
				"confirm": confirmArg,
			},
			"required": []interface{}{"spec", "confirm"},
		},
		Tool{Handler: handleScheduledTaskCreate},
	)

	r.registerDestructive("vmware_scheduledtask_create_object",
		"Create a new scheduled task on any managed object (broader than vmware_scheduledtask_create's ManagedEntity restriction — e.g. a HostVStorageObjectManager or other non-entity managed object may also be a valid target for some actions). Reversible via vmware_scheduledtask_remove.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"entity":  map[string]interface{}{"type": "object", "description": entityObjArg["description"].(string) + ` Despite the argument name, this may be ANY managed object, not only an entity.`, "properties": entityObjArg["properties"]},
				"vm":      vmEntityArg,
				"host":    hostEntityArg,
				"spec":    schedSpecArg,
				"confirm": confirmArg,
			},
			"required": []interface{}{"spec", "confirm"},
		},
		Tool{Handler: handleScheduledTaskCreateObject},
	)

	r.registerDestructive("vmware_scheduledtask_reconfigure",
		"Replace an existing scheduled task's specification (name/scheduler/action/etc). Reversible by reconfiguring again with the previous spec.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"scheduled_task": scheduledTaskIDArg,
				"spec":           schedSpecArg,
				"confirm":        confirmArg,
			},
			"required": []interface{}{"scheduled_task", "spec", "confirm"},
		},
		Tool{Handler: handleScheduledTaskReconfigure},
	)

	r.registerDestructive("vmware_scheduledtask_run",
		"Run a scheduled task's action immediately, outside its normal schedule. The task definition itself is untouched — only re-running it (or waiting for its next scheduled fire) can trigger the action again.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"scheduled_task": scheduledTaskIDArg,
				"confirm":        confirmArg,
			},
			"required": []interface{}{"scheduled_task", "confirm"},
		},
		Tool{Handler: handleScheduledTaskRun},
	)

	// --- ScheduledTaskManager: Tier 1 (irreversible) -----------------------

	r.registerDestructive("vmware_scheduledtask_remove",
		"Permanently delete a scheduled task definition. Irreversible — the task and its configuration are gone; re-creating it requires vmware_scheduledtask_create/_create_object with the same spec.",
		tier1,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"scheduled_task": scheduledTaskIDArg,
				"confirm":        confirmArg,
			},
			"required": []interface{}{"scheduled_task", "confirm"},
		},
		Tool{Handler: handleScheduledTaskRemove},
	)
}

// --- Datacenter: MoRef/raw-method plumbing ------------------------------

func handleDatacenterQueryConnectionInfo(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	dcName, _ := args["datacenter"].(string)
	dc, err := resolveDatacenter(ctx, client, dcName)
	if err != nil {
		return "", err
	}
	hostname, _ := args["hostname"].(string)
	if hostname == "" {
		return "", fmt.Errorf("hostname is required")
	}
	port, err := toInt32(args["port"])
	if err != nil {
		return "", fmt.Errorf("invalid port: %w", err)
	}
	username, _ := args["username"].(string)
	if username == "" {
		return "", fmt.Errorf("username is required")
	}
	password, _ := args["password"].(string)
	if password == "" {
		return "", fmt.Errorf("password is required")
	}
	sslThumbprint, _ := args["ssl_thumbprint"].(string)
	sslCertificate, _ := args["ssl_certificate"].(string)

	resp, err := methods.QueryConnectionInfo(ctx, client.Client.Client, &types.QueryConnectionInfo{
		This:           dc.Reference(),
		Hostname:       hostname,
		Port:           port,
		Username:       username,
		Password:       password,
		SslThumbprint:  sslThumbprint,
		SslCertificate: sslCertificate,
	})
	if err != nil {
		return "", fmt.Errorf("failed to query connection info for %s from %s: %w", hostname, dc.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"datacenter": dc.InventoryPath, "hostname": hostname, "result": resp.Returnval})
}

func handleDatacenterQueryConnectionInfoViaSpec(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	dcName, _ := args["datacenter"].(string)
	dc, err := resolveDatacenter(ctx, client, dcName)
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

	resp, err := methods.QueryConnectionInfoViaSpec(ctx, client.Client.Client, &types.QueryConnectionInfoViaSpec{
		This: dc.Reference(),
		Spec: spec,
	})
	if err != nil {
		return "", fmt.Errorf("failed to query connection info via spec from %s: %w", dc.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"datacenter": dc.InventoryPath, "hostname": spec.HostName, "result": resp.Returnval})
}

func handleDatacenterQueryConfigOptionDescriptor(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	dcName, _ := args["datacenter"].(string)
	dc, err := resolveDatacenter(ctx, client, dcName)
	if err != nil {
		return "", err
	}

	resp, err := methods.QueryDatacenterConfigOptionDescriptor(ctx, client.Client.Client, &types.QueryDatacenterConfigOptionDescriptor{
		This: dc.Reference(),
	})
	if err != nil {
		return "", fmt.Errorf("failed to query config option descriptors for %s: %w", dc.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"datacenter": dc.InventoryPath, "count": len(resp.Returnval), "options": resp.Returnval})
}

func handleDatacenterReconfigure(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	dcName, _ := args["datacenter"].(string)
	dc, err := resolveDatacenter(ctx, client, dcName)
	if err != nil {
		return "", err
	}
	rawSpec, ok := args["spec"]
	if !ok {
		return "", fmt.Errorf("spec is required")
	}
	var spec types.DatacenterConfigSpec
	if err := decodeJSONArg(rawSpec, &spec); err != nil {
		return "", fmt.Errorf("invalid spec: %w", err)
	}
	modify, _ := args["modify"].(bool)

	resp, err := methods.ReconfigureDatacenter_Task(ctx, client.Client.Client, &types.ReconfigureDatacenter_Task{
		This:   dc.Reference(),
		Spec:   spec,
		Modify: modify,
	})
	if err != nil {
		return "", fmt.Errorf("failed to reconfigure %s: %w", dc.InventoryPath, err)
	}
	if err := waitForTask(ctx, object.NewTask(client.Client.Client, resp.Returnval)); err != nil {
		return "", fmt.Errorf("reconfigure-datacenter task failed for %s: %w", dc.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"datacenter": dc.InventoryPath, "result": "reconfigured"})
}

// --- EventManager: MoRef/raw-method plumbing ----------------------------

// eventManagerRef returns the connected endpoint's EventManager MoRef —
// populated on both ESXi and vCenter (see this file's top doc comment), so
// unlike alarmManagerRef/schedManagerRef this should never actually observe
// a nil ref through any connection mode that lets these tools register; the
// check is still defense in depth, same posture as those two.
func eventManagerRef(client *vmware.Client) (types.ManagedObjectReference, error) {
	ref := client.Client.ServiceContent.EventManager
	if ref == nil {
		return types.ManagedObjectReference{}, fmt.Errorf("this vCenter/ESXi endpoint does not expose an EventManager")
	}
	return *ref, nil
}

func handleEventQuery(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgr, err := eventManagerRef(client)
	if err != nil {
		return "", err
	}
	var filter types.EventFilterSpec
	if raw, ok := args["filter"]; ok && raw != nil {
		if err := decodeJSONArg(raw, &filter); err != nil {
			return "", fmt.Errorf("invalid filter: %w", err)
		}
	}

	resp, err := methods.QueryEvents(ctx, client.Client.Client, &types.QueryEvents{This: mgr, Filter: filter})
	if err != nil {
		return "", fmt.Errorf("failed to query events: %w", err)
	}

	return marshalJSON(map[string]interface{}{"count": len(resp.Returnval), "events": resp.Returnval})
}

func handleEventCreateCollector(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgr, err := eventManagerRef(client)
	if err != nil {
		return "", err
	}
	rawFilter, ok := args["filter"]
	if !ok {
		return "", fmt.Errorf("filter is required")
	}
	var filter types.EventFilterSpec
	if err := decodeJSONArg(rawFilter, &filter); err != nil {
		return "", fmt.Errorf("invalid filter: %w", err)
	}

	resp, err := methods.CreateCollectorForEvents(ctx, client.Client.Client, &types.CreateCollectorForEvents{This: mgr, Filter: filter})
	if err != nil {
		return "", fmt.Errorf("failed to create event collector: %w", err)
	}

	return marshalJSON(map[string]interface{}{"collector": resp.Returnval.Value, "result": "collector_created"})
}

func handleEventLogUserEvent(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgr, err := eventManagerRef(client)
	if err != nil {
		return "", err
	}
	entity, entityDisp, err := alarmResolveEntity(ctx, client, args)
	if err != nil {
		return "", err
	}
	msg, _ := args["msg"].(string)
	if msg == "" {
		return "", fmt.Errorf("msg is required")
	}

	if _, err := methods.LogUserEvent(ctx, client.Client.Client, &types.LogUserEvent{This: mgr, Entity: entity, Msg: msg}); err != nil {
		return "", fmt.Errorf("failed to log user event on %s: %w", entityDisp, err)
	}

	return marshalJSON(map[string]interface{}{"entity": entityDisp, "msg": msg, "result": "event_logged"})
}

func handleEventPost(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgr, err := eventManagerRef(client)
	if err != nil {
		return "", err
	}
	rawEvent, ok := args["event"]
	if !ok {
		return "", fmt.Errorf("event is required")
	}
	var ev types.EventEx
	if err := decodeJSONArg(rawEvent, &ev); err != nil {
		return "", fmt.Errorf("invalid event: %w", err)
	}
	if ev.EventTypeId == "" {
		return "", fmt.Errorf("event.eventTypeId is required")
	}

	if _, err := methods.PostEvent(ctx, client.Client.Client, &types.PostEvent{This: mgr, EventToPost: &ev}); err != nil {
		return "", fmt.Errorf("failed to post event %q: %w", ev.EventTypeId, err)
	}

	return marshalJSON(map[string]interface{}{"event_type_id": ev.EventTypeId, "result": "event_posted"})
}

func handleEventRetrieveArgumentDescription(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgr, err := eventManagerRef(client)
	if err != nil {
		return "", err
	}
	eventTypeID, _ := args["event_type_id"].(string)
	if eventTypeID == "" {
		return "", fmt.Errorf("event_type_id is required")
	}

	resp, err := methods.RetrieveArgumentDescription(ctx, client.Client.Client, &types.RetrieveArgumentDescription{This: mgr, EventTypeId: eventTypeID})
	if err != nil {
		return "", fmt.Errorf("failed to retrieve argument description for %q: %w", eventTypeID, err)
	}

	return marshalJSON(map[string]interface{}{"event_type_id": eventTypeID, "count": len(resp.Returnval), "arguments": resp.Returnval})
}

// --- ScheduledTaskManager/ScheduledTask: MoRef/raw-method plumbing -------

// schedManagerRef returns the connected endpoint's ScheduledTaskManager
// MoRef. Nil on a standalone ESXi host (this file's top doc comment) —
// every tool in registerInventoryOpsVCenterOnlyTools registers
// modeVCenterOnly, so a nil ServiceContent.ScheduledTaskManager should never
// actually be observed through a connection mode that lets the class
// register at all; the check is defense in depth, same posture as
// generated_alarm.go's alarmManagerRef.
func schedManagerRef(client *vmware.Client) (types.ManagedObjectReference, error) {
	ref := client.Client.ServiceContent.ScheduledTaskManager
	if ref == nil {
		return types.ManagedObjectReference{}, fmt.Errorf("this vCenter/ESXi endpoint does not expose a ScheduledTaskManager")
	}
	return *ref, nil
}

// schedTaskRefArg reads the required "scheduled_task" argument — a bare
// ScheduledTask MoRef value (Type is always "ScheduledTask"), same ".Value
// only" convention as generated_alarm.go's alarmRefArg for Alarm morefs.
func schedTaskRefArg(args map[string]interface{}) (types.ManagedObjectReference, error) {
	val, _ := args["scheduled_task"].(string)
	if val == "" {
		return types.ManagedObjectReference{}, fmt.Errorf("scheduled_task is required")
	}
	return types.ManagedObjectReference{Type: "ScheduledTask", Value: val}, nil
}

// --- ScheduledTaskSpec.Scheduler (types.BaseTaskScheduler) decoding -----
//
// New to this file — reuses generated_alarm.go's alarmTypeRegistry
// container type (see this file's top doc comment) but is its own registry
// / decode function, since BaseTaskScheduler is a different interface than
// anything generated_alarm.go already decodes.

var schedSchedulerTypes = alarmTypeRegistry{
	"OnceTaskScheduler":             reflect.TypeOf(types.OnceTaskScheduler{}),
	"AfterStartupTaskScheduler":     reflect.TypeOf(types.AfterStartupTaskScheduler{}),
	"HourlyTaskScheduler":           reflect.TypeOf(types.HourlyTaskScheduler{}),
	"DailyTaskScheduler":            reflect.TypeOf(types.DailyTaskScheduler{}),
	"WeeklyTaskScheduler":           reflect.TypeOf(types.WeeklyTaskScheduler{}),
	"MonthlyByDayTaskScheduler":     reflect.TypeOf(types.MonthlyByDayTaskScheduler{}),
	"MonthlyByWeekdayTaskScheduler": reflect.TypeOf(types.MonthlyByWeekdayTaskScheduler{}),
}

// schedBuildScheduler decodes one JSON-encoded BaseTaskScheduler — same
// "_vimType discriminator -> reflect.New the concrete struct -> decodeJSONArg
// -> type-assert to the interface" approach as generated_alarm.go's
// alarmBuildExpression default branch, scoped to the 7 leaf types this
// file's top doc comment documents.
func schedBuildScheduler(raw interface{}) (types.BaseTaskScheduler, error) {
	m, typeName, err := alarmVimType(raw, schedSchedulerTypes, "spec.scheduler")
	if err != nil {
		return nil, err
	}
	rt := schedSchedulerTypes[typeName]
	ptr := reflect.New(rt)
	if err := decodeJSONArg(m, ptr.Interface()); err != nil {
		return nil, fmt.Errorf("invalid spec.scheduler (_vimType %s): %w", typeName, err)
	}
	scheduler, ok := ptr.Interface().(types.BaseTaskScheduler)
	if !ok {
		return nil, fmt.Errorf("internal error: %s does not implement BaseTaskScheduler", typeName)
	}
	return scheduler, nil
}

// schedSpecFields mirrors ScheduledTaskSpec's own non-polymorphic fields,
// decoded directly via decodeJSONArg; Scheduler/Action are built separately
// (schedBuildScheduler / generated_alarm.go's alarmBuildLeafAction — see
// this file's top doc comment).
type schedSpecFields struct {
	Name         string `json:"name"`
	Description  string `json:"description,omitempty"`
	Enabled      bool   `json:"enabled"`
	Notification string `json:"notification,omitempty"`
}

// schedBuildSpec decodes the required "spec" argument into a
// *types.ScheduledTaskSpec, used by vmware_scheduledtask_create/
// _create_object/_reconfigure.
func schedBuildSpec(args map[string]interface{}) (*types.ScheduledTaskSpec, error) {
	raw, ok := args["spec"]
	if !ok || raw == nil {
		return nil, fmt.Errorf("spec is required")
	}
	m, ok := raw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("spec must be a JSON object")
	}

	var fields schedSpecFields
	if err := decodeJSONArg(m, &fields); err != nil {
		return nil, fmt.Errorf("invalid spec: %w", err)
	}
	if fields.Name == "" {
		return nil, fmt.Errorf("spec.name is required")
	}

	rawScheduler, ok := m["scheduler"]
	if !ok || rawScheduler == nil {
		return nil, fmt.Errorf("spec.scheduler is required")
	}
	scheduler, err := schedBuildScheduler(rawScheduler)
	if err != nil {
		return nil, err
	}

	rawAction, ok := m["action"]
	if !ok || rawAction == nil {
		return nil, fmt.Errorf("spec.action is required")
	}
	action, err := alarmBuildLeafAction(rawAction)
	if err != nil {
		return nil, err
	}

	return &types.ScheduledTaskSpec{
		Name:         fields.Name,
		Description:  fields.Description,
		Enabled:      fields.Enabled,
		Scheduler:    scheduler,
		Action:       action,
		Notification: fields.Notification,
	}, nil
}

func handleScheduledTaskRetrieveForObject(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgr, err := schedManagerRef(client)
	if err != nil {
		return "", err
	}
	obj, objDisp, err := alarmResolveOptionalEntity(ctx, client, args)
	if err != nil {
		return "", err
	}

	resp, err := methods.RetrieveObjectScheduledTask(ctx, client.Client.Client, &types.RetrieveObjectScheduledTask{This: mgr, Obj: obj})
	if err != nil {
		return "", fmt.Errorf("failed to retrieve scheduled tasks for %s: %w", objDisp, err)
	}

	return marshalJSON(map[string]interface{}{"object": objDisp, "count": len(resp.Returnval), "scheduled_tasks": resp.Returnval})
}

func handleScheduledTaskRetrieveForEntity(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgr, err := schedManagerRef(client)
	if err != nil {
		return "", err
	}
	entity, entityDisp, err := alarmResolveOptionalEntity(ctx, client, args)
	if err != nil {
		return "", err
	}

	resp, err := methods.RetrieveEntityScheduledTask(ctx, client.Client.Client, &types.RetrieveEntityScheduledTask{This: mgr, Entity: entity})
	if err != nil {
		return "", fmt.Errorf("failed to retrieve scheduled tasks for %s: %w", entityDisp, err)
	}

	return marshalJSON(map[string]interface{}{"entity": entityDisp, "count": len(resp.Returnval), "scheduled_tasks": resp.Returnval})
}

func handleScheduledTaskCreate(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgr, err := schedManagerRef(client)
	if err != nil {
		return "", err
	}
	entity, entityDisp, err := alarmResolveEntity(ctx, client, args)
	if err != nil {
		return "", err
	}
	spec, err := schedBuildSpec(args)
	if err != nil {
		return "", err
	}

	resp, err := methods.CreateScheduledTask(ctx, client.Client.Client, &types.CreateScheduledTask{This: mgr, Entity: entity, Spec: spec})
	if err != nil {
		return "", fmt.Errorf("failed to create scheduled task %q on %s: %w", spec.Name, entityDisp, err)
	}

	return marshalJSON(map[string]interface{}{"entity": entityDisp, "name": spec.Name, "scheduled_task": resp.Returnval.Value, "result": "scheduled_task_created"})
}

func handleScheduledTaskCreateObject(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgr, err := schedManagerRef(client)
	if err != nil {
		return "", err
	}
	obj, objDisp, err := alarmResolveEntity(ctx, client, args)
	if err != nil {
		return "", err
	}
	spec, err := schedBuildSpec(args)
	if err != nil {
		return "", err
	}

	resp, err := methods.CreateObjectScheduledTask(ctx, client.Client.Client, &types.CreateObjectScheduledTask{This: mgr, Obj: obj, Spec: spec})
	if err != nil {
		return "", fmt.Errorf("failed to create object scheduled task %q on %s: %w", spec.Name, objDisp, err)
	}

	return marshalJSON(map[string]interface{}{"object": objDisp, "name": spec.Name, "scheduled_task": resp.Returnval.Value, "result": "scheduled_task_created"})
}

func handleScheduledTaskReconfigure(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	ref, err := schedTaskRefArg(args)
	if err != nil {
		return "", err
	}
	spec, err := schedBuildSpec(args)
	if err != nil {
		return "", err
	}

	if _, err := methods.ReconfigureScheduledTask(ctx, client.Client.Client, &types.ReconfigureScheduledTask{This: ref, Spec: spec}); err != nil {
		return "", fmt.Errorf("failed to reconfigure scheduled task %s: %w", ref.Value, err)
	}

	return marshalJSON(map[string]interface{}{"scheduled_task": ref.Value, "name": spec.Name, "result": "scheduled_task_reconfigured"})
}

func handleScheduledTaskRun(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	ref, err := schedTaskRefArg(args)
	if err != nil {
		return "", err
	}

	if _, err := methods.RunScheduledTask(ctx, client.Client.Client, &types.RunScheduledTask{This: ref}); err != nil {
		return "", fmt.Errorf("failed to run scheduled task %s: %w", ref.Value, err)
	}

	return marshalJSON(map[string]interface{}{"scheduled_task": ref.Value, "result": "scheduled_task_run_triggered"})
}

func handleScheduledTaskRemove(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	ref, err := schedTaskRefArg(args)
	if err != nil {
		return "", err
	}

	if _, err := methods.RemoveScheduledTask(ctx, client.Client.Client, &types.RemoveScheduledTask{This: ref}); err != nil {
		return "", fmt.Errorf("failed to remove scheduled task %s: %w", ref.Value, err)
	}

	return marshalJSON(map[string]interface{}{"scheduled_task": ref.Value, "result": "scheduled_task_removed"})
}

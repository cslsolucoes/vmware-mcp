package tools

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// registerAlarmTools adds the AlarmManager family (vSphere alarms): create/
// reconfigure/remove an alarm definition, list alarms and their triggered
// state for an entity, acknowledge a triggered alarm, clear triggered
// alarms, and enable/disable/query alarm actions for an entity.
//
// Managed object / MoRef: client.Client.ServiceContent.AlarmManager
// (*types.ManagedObjectReference — referencia/govmomi/vim25/types/types.go
// line ~74324) — no object.* wrapper exists for AlarmManager in
// referencia/govmomi/object (confirmed by directory listing), so every
// handler here dials the raw vim25 SOAP method directly:
// methods.Xxx(ctx, client.Client.Client, &types.Xxx{This: ref, ...}), same
// as generated_host_iscsi_portbinding.go's IscsiManager family.
//
// Two different "This": most AlarmManager operations (CreateAlarm, GetAlarm,
// GetAlarmState, AcknowledgeAlarm, ClearTriggeredAlarms, EnableAlarmActions,
// AreAlarmActionsEnabled) take the AlarmManager MoRef as This. But
// ReconfigureAlarm and RemoveAlarm are, per their own doc comments in
// types.go ("The parameters of `Alarm.ReconfigureAlarm`."), methods on the
// individual Alarm managed object (referencia/govmomi/vim25/mo/mo.go's
// mo.Alarm) — This there is the alarm's own MoRef (Type "Alarm"), not
// AlarmManager's. vmware_alarm_reconfigure/vmware_alarm_remove build that
// MoRef from a bare "alarm" string value (Type is always "Alarm" — the same
// convention generated_task.go's resolveTaskArg uses for Task morefs).
//
// Method inventory (confirmed against referencia/govmomi/vim25/methods/
// methods.go + types.go — no invented names): CreateAlarm, ReconfigureAlarm,
// RemoveAlarm, GetAlarm, GetAlarmState, AcknowledgeAlarm,
// ClearTriggeredAlarms, EnableAlarmActions, AreAlarmActionsEnabled. There is
// NO separate "DisableAlarmActions" method in vim25 — enable and disable are
// the same EnableAlarmActions method with its "enabled" bool flipped (see
// EnableAlarmActionsRequestType). vmware_alarm_enable_actions and
// vmware_alarm_disable_actions below are both thin wrappers around that one
// method, calling it with Enabled:true and Enabled:false respectively — kept
// as two tools (rather than one with an "enabled" argument) to match this
// project's naming convention for other on/off pairs and because it reads
// unambiguously in a tool list.
//
// Class: modeVCenterOnly — decided by evidence, not assumption. vcsim's ESX()
// model (referencia/govmomi/simulator/esx/service_content.go) sets
// AlarmManager to a nil *ManagedObjectReference; only the VPX() (vCenter)
// model (referencia/govmomi/simulator/vpx/service_content.go) populates it
// with a real MoRef. A standalone-ESXi connection therefore has no
// AlarmManager to call at all.
//
// vcsim coverage: referencia/govmomi/simulator/alarm_manager.go implements a
// real (if partial) AlarmManager — GetAlarm, CreateAlarm, AcknowledgeAlarm,
// and Alarm.ReconfigureAlarm/RemoveAlarm all have working handlers, so those
// 5 tools are tested with assertRealSuccess-style behavioral assertions
// against simulator.VPX(). GetAlarmState, ClearTriggeredAlarms,
// EnableAlarmActions, and AreAlarmActionsEnabled have NO handler registered
// on the simulator's AlarmManager type — those 4 reach vcsim's dispatcher and
// come back with a clean server-side fault (assertReachesServer, same
// rationale/helper as the iSCSI port binding family).
//
// Tier: vmware_alarm_remove is tier1 (irreversible — deletes an alarm
// definition, not just its triggered state). vmware_alarm_create,
// vmware_alarm_reconfigure, vmware_alarm_acknowledge,
// vmware_alarm_clear_triggered, vmware_alarm_enable_actions, and
// vmware_alarm_disable_actions are tier2 (disruptive to alarm behavior but
// reversible — create can be undone with remove, reconfigure/enable/disable
// can be flipped back, acknowledge/clear only touch transient triggered
// state). vmware_alarm_get, vmware_alarm_get_state, and
// vmware_alarm_actions_enabled are plain r.register (read-only).
func registerAlarmTools(r *Registry) {
	entityObjArg := map[string]interface{}{
		"type":        "object",
		"description": `Explicit MoRef of the target ManagedEntity, e.g. {"type": "ClusterComputeResource", "value": "domain-c7"} — use this to target an entity kind vmware_list_vms/vmware_list_hosts can't resolve directly (cluster, datacenter, resource pool, datastore, network, folder, vApp...). Alternative to "vm"/"host".`,
		"properties": map[string]interface{}{
			"type":  map[string]interface{}{"type": "string", "description": `vSphere managed entity type, e.g. "VirtualMachine", "HostSystem", "ClusterComputeResource", "Datacenter", "ResourcePool", "Datastore", "Network", "Folder".`},
			"value": map[string]interface{}{"type": "string", "description": "The entity's MoRef value."},
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
	alarmIDArg := map[string]interface{}{
		"type":        "string",
		"description": `The alarm's MoRef value (Type is always "Alarm"), as returned by vmware_alarm_create's "alarm" field or vmware_alarm_get's "alarm" field per entry.`,
	}
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}
	specArg := map[string]interface{}{
		"type": "object",
		"description": `AlarmSpec (referencia/govmomi/vim25/types/types.go's AlarmSpec). Required: "name" (string), "enabled" (bool), "expression" (object, see below). Optional: "description" (string), "systemName" (string, predefined alarms only), "actionFrequency" (int32 seconds between repeat actions), "setting" ({"toleranceRange": int32, "reportingFrequency": int32}), "action" (object, see below).

"expression" and "action" are vSphere's polymorphic (xsi:type) fields — encode the concrete type as a JSON object carrying a "_vimType" discriminator naming the exact govmomi struct:
  expression._vimType one of:
    "EventAlarmExpression"  {"eventTypeId": "...", "objectType": "...", "status": "gray|green|yellow|red", "comparisons": [{"attributeName":"...","operator":"...","value":"..."}]}
    "MetricAlarmExpression" {"operator": "isAbove|isBelow", "type": "<ManagedEntity type>", "metric": {"counterId": int32, "instance": "..."}, "yellow": int32, "yellowInterval": int32, "red": int32, "redInterval": int32}
    "StateAlarmExpression"  {"operator": "isEqual|isUnequal", "type": "<ManagedEntity type>", "statePath": "runtime.powerState", "yellow": "...", "red": "..."}
    "AndAlarmExpression" / "OrAlarmExpression"  {"expression": [<nested expression objects, same _vimType convention, recursive>]}
  action._vimType one of:
    "AlarmTriggeringAction" {"action": <nested BaseAction — see leaf types below>, "transitionSpecs": [{"startState":"green|yellow|red","finalState":"green|yellow|red","repeats":bool}], "green2yellow": bool, "yellow2red": bool, "red2yellow": bool, "yellow2green": bool}
    "GroupAlarmAction"      {"action": [<nested AlarmTriggeringAction objects>]}
  action.action (leaf BaseAction) _vimType one of:
    "SendEmailAction" {"toList":"a@b.com","ccList":"","subject":"...","body":"..."} | "SendSNMPAction" {} | "RunScriptAction" {"script":"/path/to/script"} | "CreateTaskAction" {"taskTypeId":"...","cancelable":bool} | "MethodAction" {"name":"...","argument":[{"value":...}]}

See referencia/govmomi/vim25/types/types.go for the exact field set of every nested object (EventAlarmExpressionComparison, PerfMetricId, AlarmTriggeringActionTransitionSpec, MethodActionArgument, ...).`,
	}
	filterArg := map[string]interface{}{
		"type":        "object",
		"description": `AlarmFilterSpec. All fields optional — an empty/omitted filter clears every triggered alarm. "status": array of "gray"|"green"|"yellow"|"red" (filters by triggered severity). "typeEntity": "entityTypeAll"|"entityTypeHost"|"entityTypeVm". "typeTrigger": "triggerTypeAll"|"triggerTypeEvent"|"triggerTypeMetric".`,
		"properties": map[string]interface{}{
			"status":      map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
			"typeEntity":  map[string]interface{}{"type": "string"},
			"typeTrigger": map[string]interface{}{"type": "string"},
		},
	}

	// --- Alarm definitions (mutating) ---------------------------------

	r.registerDestructive("vmware_alarm_create",
		"Create a new alarm definition on a managed entity (VM, host, cluster, datacenter, or any other ManagedEntity). Reversible via vmware_alarm_remove.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"entity":  entityObjArg,
				"vm":      vmEntityArg,
				"host":    hostEntityArg,
				"spec":    specArg,
				"confirm": confirmArg,
			},
			"required": []interface{}{"spec", "confirm"},
		},
		Tool{Handler: handleAlarmCreate},
	)

	r.registerDestructive("vmware_alarm_reconfigure",
		"Replace an existing alarm's specification (name/expression/action/etc). Reversible by reconfiguring again with the previous spec.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"alarm":   alarmIDArg,
				"spec":    specArg,
				"confirm": confirmArg,
			},
			"required": []interface{}{"alarm", "spec", "confirm"},
		},
		Tool{Handler: handleAlarmReconfigure},
	)

	r.registerDestructive("vmware_alarm_remove",
		"Permanently delete an alarm definition. Irreversible — the alarm and its configuration are gone; re-creating it requires vmware_alarm_create with the same spec.",
		tier1,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"alarm":   alarmIDArg,
				"confirm": confirmArg,
			},
			"required": []interface{}{"alarm", "confirm"},
		},
		Tool{Handler: handleAlarmRemove},
	)

	// --- Alarm queries (read-only) -------------------------------------

	r.register("vmware_alarm_get",
		"List alarm definitions, with their info (name/description/enabled/expression owner entity), for one entity or for every visible entity if none is given.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"entity": entityObjArg,
				"vm":     vmEntityArg,
				"host":   hostEntityArg,
			},
		},
		Tool{Handler: handleAlarmGet},
	)

	r.register("vmware_alarm_get_state",
		"Get the triggered alarm state (severity, time, acknowledged) for every alarm currently triggered on an entity.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"entity": entityObjArg,
				"vm":     vmEntityArg,
				"host":   hostEntityArg,
			},
		},
		Tool{Handler: handleAlarmGetState},
	)

	r.register("vmware_alarm_actions_enabled",
		"Check whether alarm actions are enabled for an entity (and, by inheritance, its descendants).",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"entity": entityObjArg,
				"vm":     vmEntityArg,
				"host":   hostEntityArg,
			},
		},
		Tool{Handler: handleAlarmActionsEnabled},
	)

	// --- Triggered-state mutations (mutating) ---------------------------

	r.registerDestructive("vmware_alarm_acknowledge",
		"Acknowledge a triggered alarm on an entity, suppressing further repeat actions for it until the alarm re-triggers from a different state. Reversible only by the alarm re-triggering — vSphere has no un-acknowledge operation.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"alarm":   alarmIDArg,
				"entity":  entityObjArg,
				"vm":      vmEntityArg,
				"host":    hostEntityArg,
				"confirm": confirmArg,
			},
			"required": []interface{}{"alarm", "confirm"},
		},
		Tool{Handler: handleAlarmAcknowledge},
	)

	r.registerDestructive("vmware_alarm_clear_triggered",
		"Clear triggered alarm state matching a filter (by severity/entity-type/trigger-type) across the whole inventory. Reversible only by the alarm re-triggering.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"filter":  filterArg,
				"confirm": confirmArg,
			},
			"required": []interface{}{"confirm"},
		},
		Tool{Handler: handleAlarmClearTriggered},
	)

	r.registerDestructive("vmware_alarm_enable_actions",
		"Enable alarm actions on an entity (and its descendants) — the counterpart is vmware_alarm_disable_actions; both call the same underlying vim25 EnableAlarmActions method with its enabled flag flipped.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"entity":  entityObjArg,
				"vm":      vmEntityArg,
				"host":    hostEntityArg,
				"confirm": confirmArg,
			},
			"required": []interface{}{"confirm"},
		},
		Tool{Handler: handleAlarmEnableActions},
	)

	r.registerDestructive("vmware_alarm_disable_actions",
		"Disable alarm actions on an entity (and its descendants) — the counterpart is vmware_alarm_enable_actions; both call the same underlying vim25 EnableAlarmActions method with its enabled flag flipped.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"entity":  entityObjArg,
				"vm":      vmEntityArg,
				"host":    hostEntityArg,
				"confirm": confirmArg,
			},
			"required": []interface{}{"confirm"},
		},
		Tool{Handler: handleAlarmDisableActions},
	)
}

// --- Entity / ref resolution -------------------------------------------

// alarmManagerRef returns the connected endpoint's AlarmManager MoRef. Nil
// on a standalone ESXi host (this file's top doc comment) — every tool here
// registers modeVCenterOnly, so a nil ServiceContent.AlarmManager should
// never actually be observed through a connection mode that lets the class
// register at all; the check is defense in depth, same posture as
// generated_host_iscsi_portbinding.go's hostIscsiManager.
func alarmManagerRef(client *vmware.Client) (types.ManagedObjectReference, error) {
	ref := client.Client.ServiceContent.AlarmManager
	if ref == nil {
		return types.ManagedObjectReference{}, fmt.Errorf("this vCenter/ESXi endpoint does not expose an AlarmManager")
	}
	return *ref, nil
}

// alarmRefArg reads the required "alarm" argument — a bare Alarm MoRef
// value (Type is always "Alarm", the same "just the .Value" convention
// generated_task.go's resolveTaskArg uses for Task morefs).
func alarmRefArg(args map[string]interface{}) (types.ManagedObjectReference, error) {
	val, _ := args["alarm"].(string)
	if val == "" {
		return types.ManagedObjectReference{}, fmt.Errorf("alarm is required")
	}
	return types.ManagedObjectReference{Type: "Alarm", Value: val}, nil
}

// alarmResolveEntity resolves the target ManagedEntity from, in priority
// order: an explicit "entity" {type,value} MoRef, a "vm" name/path
// (resolveVM), or a "host" name/path (resolveHost) — an alarm can be
// registered on any ManagedEntity, not just VMs/hosts, so the generic
// escape hatch sits alongside this project's two existing convenience
// resolvers. Also returns a display string for JSON responses/error
// messages.
func alarmResolveEntity(ctx context.Context, client *vmware.Client, args map[string]interface{}) (types.ManagedObjectReference, string, error) {
	if raw, ok := args["entity"]; ok && raw != nil {
		var ref types.ManagedObjectReference
		if err := decodeJSONArg(raw, &ref); err != nil {
			return types.ManagedObjectReference{}, "", fmt.Errorf("invalid entity: %w", err)
		}
		if ref.Type == "" || ref.Value == "" {
			return types.ManagedObjectReference{}, "", fmt.Errorf(`entity must have non-empty "type" and "value"`)
		}
		return ref, fmt.Sprintf("%s:%s", ref.Type, ref.Value), nil
	}
	if name, _ := args["vm"].(string); name != "" {
		vm, err := resolveVM(ctx, client, args)
		if err != nil {
			return types.ManagedObjectReference{}, "", err
		}
		return vm.Reference(), vm.InventoryPath, nil
	}
	if name, _ := args["host"].(string); name != "" {
		host, err := resolveHost(ctx, client, args)
		if err != nil {
			return types.ManagedObjectReference{}, "", err
		}
		return host.Reference(), host.InventoryPath, nil
	}
	return types.ManagedObjectReference{}, "", fmt.Errorf(`one of "entity" ({"type","value"} MoRef), "vm", or "host" is required to identify the target managed entity`)
}

// alarmResolveOptionalEntity is alarmResolveEntity's counterpart for
// vmware_alarm_get, whose GetAlarmRequestType.Entity is a *pointer* — nil
// (no "entity"/"vm"/"host" argument at all) means "every visible entity",
// per GetAlarmRequestType's own doc comment.
func alarmResolveOptionalEntity(ctx context.Context, client *vmware.Client, args map[string]interface{}) (*types.ManagedObjectReference, string, error) {
	_, hasEntity := args["entity"]
	vmName, _ := args["vm"].(string)
	hostName, _ := args["host"].(string)
	if !hasEntity && vmName == "" && hostName == "" {
		return nil, "(all entities)", nil
	}
	ref, disp, err := alarmResolveEntity(ctx, client, args)
	if err != nil {
		return nil, "", err
	}
	return &ref, disp, nil
}

// --- Polymorphic AlarmSpec construction ---------------------------------
//
// AlarmSpec.Expression (types.BaseAlarmExpression) and AlarmSpec.Action
// (types.BaseAlarmAction) are vim25 "typeattr" polymorphic fields: the SOAP
// wire format picks the concrete XML element via xsi:type, and govmomi's Go
// struct mirrors that with a named interface field. encoding/json cannot
// unmarshal a JSON object into a field typed as a *named* interface (it can
// only do that for the empty interface{}), so decodeJSONArg alone — which
// every other tool file in this project uses for non-polymorphic
// specs/configs — does not work for these two AlarmSpec fields. The
// functions below are this file's own small, explicit type router: every
// JSON object standing in for one of these fields must carry a "_vimType"
// string discriminator (e.g. "EventAlarmExpression"); alarmBuildExpression/
// alarmBuildAction/alarmBuildLeafAction use it to allocate the matching
// concrete govmomi struct via reflection, then decodeJSONArg fills in that
// struct's own (non-polymorphic) fields. Scoped to the finite set of
// concrete types AlarmSpec can actually reference — 5 BaseAlarmExpression
// implementers, 2 BaseAlarmAction implementers, 5 leaf BaseAction
// implementers used inside AlarmTriggeringAction — confirmed by grepping
// referencia/govmomi/vim25/types/types.go for every struct embedding
// AlarmExpression/AlarmAction/Action.

type alarmTypeRegistry map[string]reflect.Type

func (reg alarmTypeRegistry) names() string {
	names := make([]string, 0, len(reg))
	for name := range reg {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

var alarmExpressionTypes = alarmTypeRegistry{
	"AndAlarmExpression":    reflect.TypeOf(types.AndAlarmExpression{}),
	"OrAlarmExpression":     reflect.TypeOf(types.OrAlarmExpression{}),
	"EventAlarmExpression":  reflect.TypeOf(types.EventAlarmExpression{}),
	"MetricAlarmExpression": reflect.TypeOf(types.MetricAlarmExpression{}),
	"StateAlarmExpression":  reflect.TypeOf(types.StateAlarmExpression{}),
}

var alarmActionTypes = alarmTypeRegistry{
	"AlarmTriggeringAction": reflect.TypeOf(types.AlarmTriggeringAction{}),
	"GroupAlarmAction":      reflect.TypeOf(types.GroupAlarmAction{}),
}

var alarmLeafActionTypes = alarmTypeRegistry{
	"MethodAction":     reflect.TypeOf(types.MethodAction{}),
	"SendEmailAction":  reflect.TypeOf(types.SendEmailAction{}),
	"SendSNMPAction":   reflect.TypeOf(types.SendSNMPAction{}),
	"RunScriptAction":  reflect.TypeOf(types.RunScriptAction{}),
	"CreateTaskAction": reflect.TypeOf(types.CreateTaskAction{}),
}

// alarmVimType extracts and validates a raw JSON argument's "_vimType"
// discriminator against reg, returning the decoded object map for further
// decoding by the caller.
func alarmVimType(raw interface{}, reg alarmTypeRegistry, field string) (map[string]interface{}, string, error) {
	m, ok := raw.(map[string]interface{})
	if !ok {
		return nil, "", fmt.Errorf("%s must be a JSON object", field)
	}
	typeName, _ := m["_vimType"].(string)
	if typeName == "" {
		return nil, "", fmt.Errorf("%s._vimType is required (one of: %s)", field, reg.names())
	}
	if _, ok := reg[typeName]; !ok {
		return nil, "", fmt.Errorf("%s._vimType %q is not supported (one of: %s)", field, typeName, reg.names())
	}
	return m, typeName, nil
}

// alarmBuildExpression decodes one JSON-encoded BaseAlarmExpression,
// recursing into AndAlarmExpression/OrAlarmExpression's own nested
// "expression" array (also BaseAlarmExpression, also polymorphic) — the
// other 3 leaf types (Event/Metric/State) have no further polymorphic
// fields, so decodeJSONArg handles the rest of their fields directly.
func alarmBuildExpression(raw interface{}) (types.BaseAlarmExpression, error) {
	m, typeName, err := alarmVimType(raw, alarmExpressionTypes, "expression")
	if err != nil {
		return nil, err
	}

	switch typeName {
	case "AndAlarmExpression", "OrAlarmExpression":
		rawChildren, _ := m["expression"].([]interface{})
		if len(rawChildren) == 0 {
			return nil, fmt.Errorf(`expression._vimType %s requires a non-empty "expression" array of nested expressions`, typeName)
		}
		children := make([]types.BaseAlarmExpression, 0, len(rawChildren))
		for i, rawChild := range rawChildren {
			child, err := alarmBuildExpression(rawChild)
			if err != nil {
				return nil, fmt.Errorf("expression.expression[%d]: %w", i, err)
			}
			children = append(children, child)
		}
		if typeName == "AndAlarmExpression" {
			return &types.AndAlarmExpression{Expression: children}, nil
		}
		return &types.OrAlarmExpression{Expression: children}, nil
	default:
		rt := alarmExpressionTypes[typeName]
		ptr := reflect.New(rt)
		if err := decodeJSONArg(m, ptr.Interface()); err != nil {
			return nil, fmt.Errorf("invalid expression (_vimType %s): %w", typeName, err)
		}
		expr, ok := ptr.Interface().(types.BaseAlarmExpression)
		if !ok {
			return nil, fmt.Errorf("internal error: %s does not implement BaseAlarmExpression", typeName)
		}
		return expr, nil
	}
}

// alarmBuildLeafAction decodes one JSON-encoded BaseAction — the concrete
// action an AlarmTriggeringAction fires (email/SNMP/script/task/extension
// method).
func alarmBuildLeafAction(raw interface{}) (types.BaseAction, error) {
	m, typeName, err := alarmVimType(raw, alarmLeafActionTypes, "action.action")
	if err != nil {
		return nil, err
	}
	rt := alarmLeafActionTypes[typeName]
	ptr := reflect.New(rt)
	if err := decodeJSONArg(m, ptr.Interface()); err != nil {
		return nil, fmt.Errorf("invalid action.action (_vimType %s): %w", typeName, err)
	}
	action, ok := ptr.Interface().(types.BaseAction)
	if !ok {
		return nil, fmt.Errorf("internal error: %s does not implement BaseAction", typeName)
	}
	return action, nil
}

// alarmTriggeringActionFields mirrors AlarmTriggeringAction's own non-
// polymorphic fields, decoded directly via decodeJSONArg; its "action"
// field (types.BaseAction) is decoded separately via alarmBuildLeafAction.
type alarmTriggeringActionFields struct {
	TransitionSpecs []types.AlarmTriggeringActionTransitionSpec `json:"transitionSpecs,omitempty"`
	Green2yellow    bool                                        `json:"green2yellow,omitempty"`
	Yellow2red      bool                                        `json:"yellow2red,omitempty"`
	Red2yellow      bool                                        `json:"red2yellow,omitempty"`
	Yellow2green    bool                                        `json:"yellow2green,omitempty"`
}

// alarmBuildAction decodes one JSON-encoded BaseAlarmAction (AlarmSpec.
// Action) — AlarmTriggeringAction (the common case: fire one BaseAction on
// a status transition) or GroupAlarmAction (fire several
// AlarmTriggeringActions together).
func alarmBuildAction(raw interface{}) (types.BaseAlarmAction, error) {
	m, typeName, err := alarmVimType(raw, alarmActionTypes, "action")
	if err != nil {
		return nil, err
	}

	switch typeName {
	case "AlarmTriggeringAction":
		var fields alarmTriggeringActionFields
		if err := decodeJSONArg(m, &fields); err != nil {
			return nil, fmt.Errorf("invalid action (_vimType AlarmTriggeringAction): %w", err)
		}
		rawLeaf, ok := m["action"]
		if !ok || rawLeaf == nil {
			return nil, fmt.Errorf(`action._vimType AlarmTriggeringAction requires a nested "action" (one of: %s)`, alarmLeafActionTypes.names())
		}
		leaf, err := alarmBuildLeafAction(rawLeaf)
		if err != nil {
			return nil, err
		}
		return &types.AlarmTriggeringAction{
			Action:          leaf,
			TransitionSpecs: fields.TransitionSpecs,
			Green2yellow:    fields.Green2yellow,
			Yellow2red:      fields.Yellow2red,
			Red2yellow:      fields.Red2yellow,
			Yellow2green:    fields.Yellow2green,
		}, nil
	case "GroupAlarmAction":
		rawChildren, _ := m["action"].([]interface{})
		if len(rawChildren) == 0 {
			return nil, fmt.Errorf(`action._vimType GroupAlarmAction requires a non-empty "action" array of nested actions`)
		}
		children := make([]types.BaseAlarmAction, 0, len(rawChildren))
		for i, rawChild := range rawChildren {
			child, err := alarmBuildAction(rawChild)
			if err != nil {
				return nil, fmt.Errorf("action.action[%d]: %w", i, err)
			}
			children = append(children, child)
		}
		return &types.GroupAlarmAction{Action: children}, nil
	default:
		return nil, fmt.Errorf("internal error: unhandled action _vimType %s", typeName)
	}
}

// alarmSpecFields mirrors AlarmSpec's own non-polymorphic fields —
// including Setting, a concrete (non-interface) struct decodeJSONArg
// handles directly. Expression/Action are built separately (see above).
type alarmSpecFields struct {
	Name            string              `json:"name"`
	SystemName      string              `json:"systemName,omitempty"`
	Description     string              `json:"description"`
	Enabled         bool                `json:"enabled"`
	ActionFrequency int32               `json:"actionFrequency,omitempty"`
	Setting         *types.AlarmSetting `json:"setting,omitempty"`
}

// alarmBuildSpec decodes the required "spec" argument into a
// *types.AlarmSpec, used by vmware_alarm_create/vmware_alarm_reconfigure.
func alarmBuildSpec(args map[string]interface{}) (*types.AlarmSpec, error) {
	raw, ok := args["spec"]
	if !ok || raw == nil {
		return nil, fmt.Errorf("spec is required")
	}
	m, ok := raw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("spec must be a JSON object")
	}

	var fields alarmSpecFields
	if err := decodeJSONArg(m, &fields); err != nil {
		return nil, fmt.Errorf("invalid spec: %w", err)
	}
	if fields.Name == "" {
		return nil, fmt.Errorf("spec.name is required")
	}

	rawExpr, ok := m["expression"]
	if !ok || rawExpr == nil {
		return nil, fmt.Errorf("spec.expression is required")
	}
	expr, err := alarmBuildExpression(rawExpr)
	if err != nil {
		return nil, err
	}

	spec := &types.AlarmSpec{
		Name:            fields.Name,
		SystemName:      fields.SystemName,
		Description:     fields.Description,
		Enabled:         fields.Enabled,
		Expression:      expr,
		ActionFrequency: fields.ActionFrequency,
		Setting:         fields.Setting,
	}

	if rawAction, ok := m["action"]; ok && rawAction != nil {
		action, err := alarmBuildAction(rawAction)
		if err != nil {
			return nil, err
		}
		spec.Action = action
	}

	return spec, nil
}

// --- Handlers: alarm definitions (mutating) -----------------------------

func handleAlarmCreate(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgr, err := alarmManagerRef(client)
	if err != nil {
		return "", err
	}
	entity, entityDisp, err := alarmResolveEntity(ctx, client, args)
	if err != nil {
		return "", err
	}
	spec, err := alarmBuildSpec(args)
	if err != nil {
		return "", err
	}

	resp, err := methods.CreateAlarm(ctx, client.Client.Client, &types.CreateAlarm{
		This:   mgr,
		Entity: entity,
		Spec:   spec,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create alarm %q on %s: %w", spec.Name, entityDisp, err)
	}

	return marshalJSON(map[string]interface{}{
		"entity": entityDisp,
		"name":   spec.Name,
		"alarm":  resp.Returnval.Value,
		"result": "alarm_created",
	})
}

func handleAlarmReconfigure(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	ref, err := alarmRefArg(args)
	if err != nil {
		return "", err
	}
	spec, err := alarmBuildSpec(args)
	if err != nil {
		return "", err
	}

	if _, err := methods.ReconfigureAlarm(ctx, client.Client.Client, &types.ReconfigureAlarm{
		This: ref,
		Spec: spec,
	}); err != nil {
		return "", fmt.Errorf("failed to reconfigure alarm %s: %w", ref.Value, err)
	}

	return marshalJSON(map[string]interface{}{"alarm": ref.Value, "name": spec.Name, "result": "alarm_reconfigured"})
}

func handleAlarmRemove(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	ref, err := alarmRefArg(args)
	if err != nil {
		return "", err
	}

	if _, err := methods.RemoveAlarm(ctx, client.Client.Client, &types.RemoveAlarm{This: ref}); err != nil {
		return "", fmt.Errorf("failed to remove alarm %s: %w", ref.Value, err)
	}

	return marshalJSON(map[string]interface{}{"alarm": ref.Value, "result": "alarm_removed"})
}

// --- Handlers: alarm queries (read-only) --------------------------------

// handleAlarmGet calls AlarmManager.GetAlarm (which returns only bare
// MoRefs) and then, when at least one alarm was found, enriches the result
// with each alarm's Info via one batched PropertyCollector round trip
// (property.DefaultCollector(...).Retrieve into []mo.Alarm) — the same
// batch-fetch pattern inventory.go's datastoreSummaries uses, rather than
// one RetrieveOne call per alarm.
func handleAlarmGet(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgr, err := alarmManagerRef(client)
	if err != nil {
		return "", err
	}
	entity, entityDisp, err := alarmResolveOptionalEntity(ctx, client, args)
	if err != nil {
		return "", err
	}

	resp, err := methods.GetAlarm(ctx, client.Client.Client, &types.GetAlarm{This: mgr, Entity: entity})
	if err != nil {
		return "", fmt.Errorf("failed to get alarms for %s: %w", entityDisp, err)
	}

	if len(resp.Returnval) == 0 {
		return marshalJSON(map[string]interface{}{
			"entity": entityDisp,
			"count":  0,
			"alarms": []interface{}{},
		})
	}

	var mods []mo.Alarm
	if err := property.DefaultCollector(client.Client.Client).Retrieve(ctx, resp.Returnval, []string{"info"}, &mods); err != nil {
		return "", fmt.Errorf("failed to read alarm info for %s: %w", entityDisp, err)
	}

	alarms := make([]map[string]interface{}, 0, len(mods))
	for _, a := range mods {
		alarms = append(alarms, map[string]interface{}{
			"alarm":       a.Self.Value,
			"key":         a.Info.Key,
			"name":        a.Info.Name,
			"description": a.Info.Description,
			"enabled":     a.Info.Enabled,
			"entity":      a.Info.Entity,
		})
	}

	return marshalJSON(map[string]interface{}{
		"entity": entityDisp,
		"count":  len(alarms),
		"alarms": alarms,
	})
}

func handleAlarmGetState(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgr, err := alarmManagerRef(client)
	if err != nil {
		return "", err
	}
	entity, entityDisp, err := alarmResolveEntity(ctx, client, args)
	if err != nil {
		return "", err
	}

	resp, err := methods.GetAlarmState(ctx, client.Client.Client, &types.GetAlarmState{This: mgr, Entity: entity})
	if err != nil {
		return "", fmt.Errorf("failed to get alarm state for %s: %w", entityDisp, err)
	}

	return marshalJSON(map[string]interface{}{
		"entity": entityDisp,
		"count":  len(resp.Returnval),
		"states": resp.Returnval,
	})
}

func handleAlarmActionsEnabled(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgr, err := alarmManagerRef(client)
	if err != nil {
		return "", err
	}
	entity, entityDisp, err := alarmResolveEntity(ctx, client, args)
	if err != nil {
		return "", err
	}

	resp, err := methods.AreAlarmActionsEnabled(ctx, client.Client.Client, &types.AreAlarmActionsEnabled{This: mgr, Entity: entity})
	if err != nil {
		return "", fmt.Errorf("failed to check alarm actions enabled for %s: %w", entityDisp, err)
	}

	return marshalJSON(map[string]interface{}{"entity": entityDisp, "actions_enabled": resp.Returnval})
}

// --- Handlers: triggered-state mutations --------------------------------

func handleAlarmAcknowledge(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgr, err := alarmManagerRef(client)
	if err != nil {
		return "", err
	}
	alarm, err := alarmRefArg(args)
	if err != nil {
		return "", err
	}
	entity, entityDisp, err := alarmResolveEntity(ctx, client, args)
	if err != nil {
		return "", err
	}

	if _, err := methods.AcknowledgeAlarm(ctx, client.Client.Client, &types.AcknowledgeAlarm{
		This:   mgr,
		Alarm:  alarm,
		Entity: entity,
	}); err != nil {
		return "", fmt.Errorf("failed to acknowledge alarm %s on %s: %w", alarm.Value, entityDisp, err)
	}

	return marshalJSON(map[string]interface{}{"alarm": alarm.Value, "entity": entityDisp, "result": "alarm_acknowledged"})
}

func handleAlarmClearTriggered(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgr, err := alarmManagerRef(client)
	if err != nil {
		return "", err
	}

	var filter types.AlarmFilterSpec
	if raw, ok := args["filter"]; ok && raw != nil {
		if err := decodeJSONArg(raw, &filter); err != nil {
			return "", fmt.Errorf("invalid filter: %w", err)
		}
	}

	if _, err := methods.ClearTriggeredAlarms(ctx, client.Client.Client, &types.ClearTriggeredAlarms{
		This:   mgr,
		Filter: filter,
	}); err != nil {
		return "", fmt.Errorf("failed to clear triggered alarms: %w", err)
	}

	return marshalJSON(map[string]interface{}{"filter": filter, "result": "triggered_alarms_cleared"})
}

// alarmSetActionsEnabled is the shared implementation behind
// vmware_alarm_enable_actions and vmware_alarm_disable_actions — vim25 has
// one method (EnableAlarmActions) for both, selected by its "enabled" bool
// (see this file's top doc comment).
func alarmSetActionsEnabled(ctx context.Context, client *vmware.Client, args map[string]interface{}, enabled bool) (string, error) {
	mgr, err := alarmManagerRef(client)
	if err != nil {
		return "", err
	}
	entity, entityDisp, err := alarmResolveEntity(ctx, client, args)
	if err != nil {
		return "", err
	}

	if _, err := methods.EnableAlarmActions(ctx, client.Client.Client, &types.EnableAlarmActions{
		This:    mgr,
		Entity:  entity,
		Enabled: enabled,
	}); err != nil {
		return "", fmt.Errorf("failed to set alarm actions enabled=%t for %s: %w", enabled, entityDisp, err)
	}

	result := "alarm_actions_enabled"
	if !enabled {
		result = "alarm_actions_disabled"
	}
	return marshalJSON(map[string]interface{}{"entity": entityDisp, "enabled": enabled, "result": result})
}

func handleAlarmEnableActions(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	return alarmSetActionsEnabled(ctx, client, args, true)
}

func handleAlarmDisableActions(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	return alarmSetActionsEnabled(ctx, client, args, false)
}

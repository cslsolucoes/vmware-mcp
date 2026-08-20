package tools

import (
	"context"
	"testing"

	"github.com/vmware/govmomi/simulator"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// inventoryOpsToolNames is the exact set registered by
// registerInventoryOpsTools (vsphere-general: Datacenter + EventManager)
// plus registerInventoryOpsVCenterOnlyTools (vcenter-only:
// ScheduledTaskManager) — kept here so TestInventoryOpsTools/Registration
// can't silently drift from generated_inventory_ops.go. Once the
// coordinator wires both functions into registry.go's registerTools (this
// file/generated_inventory_ops.go were built without touching registry.go
// or mode_test.go, per this task's constraints), this list is also what
// mode_test.go's tool-class inventories should match.
var inventoryOpsToolNames = []string{
	// Datacenter (vsphere-general).
	"vmware_datacenter_query_connection_info",
	"vmware_datacenter_query_connection_info_via_spec",
	"vmware_datacenter_query_config_option_descriptor",
	"vmware_datacenter_reconfigure",
	// EventManager (vsphere-general).
	"vmware_event_query",
	"vmware_event_retrieve_argument_description",
	"vmware_event_create_collector",
	"vmware_event_log_user_event",
	"vmware_event_post",
	// ScheduledTaskManager/ScheduledTask (vcenter-only).
	"vmware_scheduledtask_retrieve_for_object",
	"vmware_scheduledtask_retrieve_for_entity",
	"vmware_scheduledtask_create",
	"vmware_scheduledtask_create_object",
	"vmware_scheduledtask_reconfigure",
	"vmware_scheduledtask_run",
	"vmware_scheduledtask_remove",
}

// newInventoryOpsRegistry builds a Registry the normal way (NewRegistry,
// which already wires in every OTHER tool file via registry.go's
// registerTools) and additionally registers this file's 2 register
// functions via withClass — registry.go itself is intentionally left
// untouched by this change (parallel-work constraint; the coordinator
// integrates both functions into registerTools separately). Same 2-class
// pattern/rationale as generated_inventory_compute_test.go's
// newInventoryComputeRegistry.
func newInventoryOpsRegistry(ctx context.Context, c *vmware.Client, opts RegistryOptions) *Registry {
	r := NewRegistry(ctx, c, opts)
	r.withClass(modeVSphereGeneral, registerInventoryOpsTools)
	r.withClass(modeVCenterOnly, registerInventoryOpsVCenterOnlyTools)
	return r
}

// schedMinimalSpec is a bare-minimum, valid ScheduledTaskSpec argument (JSON
// form) reused across the subtests that need *a* spec but don't care about
// its content — a OnceTaskScheduler (no runAt = run immediately once
// activated) triggering a MethodAction, the same minimalism level as
// generated_alarm_test.go's alarmMinimalSpec.
func schedMinimalSpec(name string) map[string]interface{} {
	return map[string]interface{}{
		"name":    name,
		"enabled": true,
		"scheduler": map[string]interface{}{
			"_vimType": "OnceTaskScheduler",
		},
		"action": map[string]interface{}{
			"_vimType": "MethodAction",
			"name":     "PowerOnVM_Task",
		},
	}
}

// TestInventoryOpsTools drives every subtest against ONE shared vcsim
// server/connection (simulator.VPX(), started once via newSimClient and
// closed via a single deferred cleanup) — 2 Registry values are built on top
// of that SAME *vmware.Client (open/closed destructive gate), never a second
// vcsim instance. VPX (not ESX) is required because: (1) EventManager.
// QueryEvents' own simulator handler explicitly faults NotImplemented when
// ctx.Map.IsESX() (see generated_inventory_ops.go's top doc comment), and
// (2) ScheduledTaskManager is nil on ESX entirely — both are exercised for
// real here.
func TestInventoryOpsTools(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	open := newInventoryOpsRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	closed := newInventoryOpsRegistry(context.Background(), c, RegistryOptions{})
	dc := firstDatacenterPath(t, open)
	vm := firstVMPath(t, open)

	t.Run("Registration", func(t *testing.T) {
		if len(inventoryOpsToolNames) != 16 {
			t.Fatalf("test bug: inventoryOpsToolNames has %d entries, expected 16", len(inventoryOpsToolNames))
		}
		got := map[string]bool{}
		for _, tl := range open.ListTools() {
			got[tl.Name] = true
		}
		for _, name := range inventoryOpsToolNames {
			if !got[name] {
				t.Errorf("tool %s not registered", name)
			}
		}
	})

	// Validation proves each handler rejects missing/empty required
	// arguments BEFORE any network round trip — run with the gate open and
	// confirm:true on every destructive case, same convention as
	// generated_alarm_test.go's TestAlarmTools_Validation, so the gate/
	// confirm check itself can never be what's actually failing the call.
	t.Run("Validation", func(t *testing.T) {
		schedSpec := schedMinimalSpec("Validation Test Scheduled Task")

		cases := []struct {
			name string
			args map[string]interface{}
			why  string
		}{
			{"vmware_datacenter_query_connection_info", map[string]interface{}{"datacenter": dc, "port": 443, "username": "root", "password": "pass"}, "missing hostname"},
			{"vmware_datacenter_query_connection_info", map[string]interface{}{"datacenter": dc, "hostname": "10.0.0.99", "username": "root", "password": "pass"}, "missing port"},
			{"vmware_datacenter_query_connection_info", map[string]interface{}{"datacenter": dc, "hostname": "10.0.0.99", "port": 443, "password": "pass"}, "missing username"},
			{"vmware_datacenter_query_connection_info", map[string]interface{}{"datacenter": dc, "hostname": "10.0.0.99", "port": 443, "username": "root"}, "missing password"},
			{"vmware_datacenter_query_connection_info_via_spec", map[string]interface{}{"datacenter": dc}, "missing spec"},
			{"vmware_datacenter_reconfigure", map[string]interface{}{"datacenter": dc, "confirm": true}, "missing spec"},
			{"vmware_scheduledtask_create", map[string]interface{}{"vm": vm, "confirm": true}, "missing spec"},
			{"vmware_scheduledtask_create", map[string]interface{}{"spec": schedSpec, "confirm": true}, "missing entity/vm/host"},
			{"vmware_scheduledtask_create_object", map[string]interface{}{"vm": vm, "confirm": true}, "missing spec"},
			{"vmware_scheduledtask_reconfigure", map[string]interface{}{"spec": schedSpec, "confirm": true}, "missing scheduled_task"},
			{"vmware_scheduledtask_reconfigure", map[string]interface{}{"scheduled_task": "task-1", "confirm": true}, "missing spec"},
			{"vmware_scheduledtask_run", map[string]interface{}{"confirm": true}, "missing scheduled_task"},
			{"vmware_scheduledtask_remove", map[string]interface{}{"confirm": true}, "missing scheduled_task"},
			{"vmware_event_create_collector", map[string]interface{}{"confirm": true}, "missing filter"},
			{"vmware_event_log_user_event", map[string]interface{}{"vm": vm, "confirm": true}, "missing msg"},
			{"vmware_event_log_user_event", map[string]interface{}{"msg": "hi", "confirm": true}, "missing entity/vm/host"},
			{"vmware_event_post", map[string]interface{}{"confirm": true}, "missing event"},
			{"vmware_event_post", map[string]interface{}{"event": map[string]interface{}{"message": "no type id"}, "confirm": true}, "missing event.eventTypeId"},
			{"vmware_event_retrieve_argument_description", map[string]interface{}{}, "missing event_type_id"},
		}

		for _, tc := range cases {
			t.Run(tc.name+"/"+tc.why, func(t *testing.T) {
				if _, err := open.CallTool(tc.name, tc.args); err == nil {
					t.Fatalf("expected an error (%s) before any round trip", tc.why)
				}
			})
		}
	})

	// GateAndConfirm proves the tier1/tier2 destructive protection is wired
	// on all 9 mutating tools: a closed --allow-destructive gate denies the
	// call, and an open gate still requires confirm:true. Same shape as
	// generated_alarm_test.go's TestAlarmTools_GateAndConfirm.
	t.Run("GateAndConfirm", func(t *testing.T) {
		schedSpec := schedMinimalSpec("Gate Test Scheduled Task")

		cases := []struct {
			name string
			args map[string]interface{}
		}{
			{"vmware_datacenter_reconfigure", map[string]interface{}{"datacenter": dc, "spec": map[string]interface{}{"defaultHardwareVersionKey": "vmx-19"}, "confirm": true}},
			{"vmware_scheduledtask_create", map[string]interface{}{"vm": vm, "spec": schedSpec, "confirm": true}},
			{"vmware_scheduledtask_create_object", map[string]interface{}{"vm": vm, "spec": schedSpec, "confirm": true}},
			{"vmware_scheduledtask_reconfigure", map[string]interface{}{"scheduled_task": "task-1", "spec": schedSpec, "confirm": true}},
			{"vmware_scheduledtask_run", map[string]interface{}{"scheduled_task": "task-1", "confirm": true}},
			{"vmware_scheduledtask_remove", map[string]interface{}{"scheduled_task": "task-1", "confirm": true}},
			{"vmware_event_create_collector", map[string]interface{}{"filter": map[string]interface{}{}, "confirm": true}},
			{"vmware_event_log_user_event", map[string]interface{}{"vm": vm, "msg": "gate test", "confirm": true}},
			{"vmware_event_post", map[string]interface{}{"event": map[string]interface{}{"eventTypeId": "com.mcpvmware.test.GateEvent"}, "confirm": true}},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				if _, err := closed.CallTool(tc.name, tc.args); err == nil {
					t.Fatalf("%s: expected the closed destructive gate to deny the call", tc.name)
				}

				noConfirm := map[string]interface{}{}
				for k, v := range tc.args {
					if k != "confirm" {
						noConfirm[k] = v
					}
				}
				if _, err := open.CallTool(tc.name, noConfirm); err == nil {
					t.Fatalf("%s: expected an error without confirm:true", tc.name)
				}
			})
		}
	})

	// EventRealSuccess drives the 3 EventManager tools backed by a working
	// referencia-equivalent simulator/event_manager.go handler (PostEvent,
	// QueryEvents, CreateCollectorForEvents — see generated_inventory_ops.go's
	// top doc comment) through one real post -> query -> create-collector
	// flow against simulator.VPX(), asserting actual state, not just "no
	// error". The posted event's objectType/objectId MUST point at a real
	// inventory object — confirmed empirically (not assumed): vcsim's own
	// EventManager.PostEvent handler unconditionally forwards every posted
	// *types.EventEx to AlarmManager.postEvent, which does
	// `ctx.Map.Get(types.ManagedObjectReference{Type: event.ObjectType,
	// Value: event.ObjectId}).(mo.Entity)` with NO nil-check
	// (simulator/alarm_manager.go) — an EventEx with objectType/objectId
	// left empty (a perfectly valid, real-vSphere-legal "untargeted" custom
	// event) makes Map.Get return nil and that type assertion panic. Not a
	// bug in this project's tool — the tool sends exactly the JSON object
	// the caller provided; this is vcsim's own simulator being stricter
	// than real vCenter. Worked around here (not "fixed" — nothing in this
	// project's own code is wrong) by scoping the test event to the real VM
	// vm's own moref.
	t.Run("EventRealSuccess", func(t *testing.T) {
		const eventTypeID = "com.mcpvmware.test.RealSuccessEvent"
		const message = "MCPVMWare RealSuccess Test Event"

		vmObj, err := c.Finder.VirtualMachine(context.Background(), vm)
		if err != nil {
			t.Fatalf("failed to resolve %s for the event's objectId fixture: %v", vm, err)
		}
		vmRef := vmObj.Reference()

		rawPost, err := open.CallTool("vmware_event_post", map[string]interface{}{
			"event": map[string]interface{}{
				"eventTypeId": eventTypeID,
				"message":     message,
				"severity":    "info",
				"objectType":  vmRef.Type,
				"objectId":    vmRef.Value,
			},
			"confirm": true,
		})
		if err != nil {
			t.Fatalf("vmware_event_post failed: %v", err)
		}
		posted := decodeResult(t, rawPost)
		if posted["event_type_id"] != eventTypeID {
			t.Errorf("vmware_event_post: event_type_id = %v, want %q", posted["event_type_id"], eventTypeID)
		}

		rawQuery, err := open.CallTool("vmware_event_query", map[string]interface{}{
			"filter": map[string]interface{}{
				"eventTypeId": []interface{}{eventTypeID},
			},
		})
		if err != nil {
			t.Fatalf("vmware_event_query failed: %v", err)
		}
		queried := decodeResult(t, rawQuery)
		events, _ := queried["events"].([]interface{})
		found := false
		for _, e := range events {
			m, _ := e.(map[string]interface{})
			if m["eventTypeId"] == eventTypeID {
				found = true
				if m["message"] != message {
					t.Errorf("vmware_event_query: event %s has message %v, want %q", eventTypeID, m["message"], message)
				}
			}
		}
		if !found {
			t.Fatalf("vmware_event_query did not return the just-posted event %s among %d events", eventTypeID, len(events))
		}

		rawCollector, err := open.CallTool("vmware_event_create_collector", map[string]interface{}{
			"filter": map[string]interface{}{
				"eventTypeId": []interface{}{eventTypeID},
			},
			"confirm": true,
		})
		if err != nil {
			t.Fatalf("vmware_event_create_collector failed: %v", err)
		}
		collector := decodeResult(t, rawCollector)
		if collector["collector"] == "" || collector["collector"] == nil {
			t.Fatalf("vmware_event_create_collector did not return a collector moref: %v", collector)
		}
	})

	// ReachesServer drives every tool whose underlying vim25 method has NO
	// simulator handler at all (Datacenter's 4 extra methods,
	// ScheduledTaskManager's 7 methods, EventManager.LogUserEvent, and
	// EventManager.RetrieveArgumentDescription — see
	// generated_inventory_ops.go's top doc comment): each call is expected
	// to reach vcsim's dispatcher (or fault on an unregistered managed
	// object reference server-side) and come back with a clean error,
	// proving the wiring — schema, gate, entity/datacenter resolution,
	// manager MoRef, raw method dispatch — reaches vcsim and returns a
	// clean server-side error, not an unknown-tool wiring bug or a
	// recovered panic. Same helper/rationale as generated_alarm_test.go's
	// TestAlarmTools_ReachesServer.
	t.Run("ReachesServer", func(t *testing.T) {
		schedSpec := schedMinimalSpec("ReachesServer Test Scheduled Task")

		cases := []struct {
			name string
			args map[string]interface{}
		}{
			{"vmware_datacenter_query_connection_info", map[string]interface{}{"datacenter": dc, "hostname": "10.0.0.99", "port": 443, "username": "root", "password": "pass"}},
			{"vmware_datacenter_query_connection_info_via_spec", map[string]interface{}{"datacenter": dc, "spec": map[string]interface{}{"hostName": "10.0.0.99", "userName": "root", "password": "pass"}}},
			{"vmware_datacenter_query_config_option_descriptor", map[string]interface{}{"datacenter": dc}},
			{"vmware_datacenter_reconfigure", map[string]interface{}{"datacenter": dc, "spec": map[string]interface{}{"defaultHardwareVersionKey": "vmx-19"}, "confirm": true}},
			{"vmware_scheduledtask_retrieve_for_object", map[string]interface{}{"vm": vm}},
			{"vmware_scheduledtask_retrieve_for_entity", map[string]interface{}{"vm": vm}},
			{"vmware_scheduledtask_create", map[string]interface{}{"vm": vm, "spec": schedSpec, "confirm": true}},
			{"vmware_scheduledtask_create_object", map[string]interface{}{"vm": vm, "spec": schedSpec, "confirm": true}},
			{"vmware_scheduledtask_reconfigure", map[string]interface{}{"scheduled_task": "task-1", "spec": schedSpec, "confirm": true}},
			{"vmware_scheduledtask_run", map[string]interface{}{"scheduled_task": "task-1", "confirm": true}},
			{"vmware_scheduledtask_remove", map[string]interface{}{"scheduled_task": "task-1", "confirm": true}},
			{"vmware_event_log_user_event", map[string]interface{}{"vm": vm, "msg": "reaches server test", "confirm": true}},
			{"vmware_event_retrieve_argument_description", map[string]interface{}{"event_type_id": "VmPoweredOnEvent"}},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				_, err := open.CallTool(tc.name, tc.args)
				assertReachesServer(t, err, tc.name)
			})
		}
	})
}

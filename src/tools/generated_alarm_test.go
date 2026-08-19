package tools

import (
	"context"
	"testing"

	"github.com/vmware/govmomi/simulator"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// alarmToolNames is the exact set registered by registerAlarmTools — kept
// here so TestAlarmTools_Registration can't silently drift from
// generated_alarm.go. Once the coordinator wires registerAlarmTools into
// registry.go's registerTools (this file/generated_alarm.go were built in
// parallel with that integration, per this task's constraints), this list
// is also what mode_test.go's vcenterOnlyTools-style inventory should match.
var alarmToolNames = []string{
	"vmware_alarm_create",
	"vmware_alarm_reconfigure",
	"vmware_alarm_remove",
	"vmware_alarm_get",
	"vmware_alarm_get_state",
	"vmware_alarm_actions_enabled",
	"vmware_alarm_acknowledge",
	"vmware_alarm_clear_triggered",
	"vmware_alarm_enable_actions",
	"vmware_alarm_disable_actions",
}

// newAlarmRegistry builds a Registry the normal way (NewRegistry, which
// already wires in every OTHER tool file via registry.go's registerTools)
// and additionally registers this file's tools via withClass — registry.go
// itself is intentionally left untouched by this change (parallel-work
// constraint; the coordinator integrates registerAlarmTools into
// registerTools separately). Mirrors generated_host_iscsi_portbinding_
// test.go's local-helper note for the same situation.
func newAlarmRegistry(ctx context.Context, c *vmware.Client, opts RegistryOptions) *Registry {
	r := NewRegistry(ctx, c, opts)
	r.withClass(modeVCenterOnly, registerAlarmTools)
	return r
}

// alarmMinimalSpec is a bare-minimum, valid AlarmSpec argument (JSON form)
// reused across tests that need *a* spec but don't care about its content.
func alarmMinimalSpec(name string) map[string]interface{} {
	return map[string]interface{}{
		"name":    name,
		"enabled": true,
		"expression": map[string]interface{}{
			"_vimType":    "EventAlarmExpression",
			"eventTypeId": "com.vmware.test.Event",
		},
	}
}

// TestAlarmTools_Registration proves all 10 AlarmManager tools are
// reachable via ListTools once registerAlarmTools runs.
func TestAlarmTools_Registration(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newAlarmRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	if len(alarmToolNames) != 10 {
		t.Fatalf("test bug: alarmToolNames has %d entries, expected 10", len(alarmToolNames))
	}

	got := map[string]bool{}
	for _, tl := range r.ListTools() {
		got[tl.Name] = true
	}
	for _, name := range alarmToolNames {
		if !got[name] {
			t.Errorf("tool %s not registered", name)
		}
	}
}

// TestAlarmTools_Validation proves each handler rejects missing/empty
// required arguments BEFORE any network round trip (so these fail even
// with the gate open and confirm:true, same convention as
// generated_host_iscsi_portbinding_test.go).
func TestAlarmTools_Validation(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newAlarmRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	vm := firstVMPath(t, r)
	spec := alarmMinimalSpec("Validation Test Alarm")

	cases := []struct {
		name string
		args map[string]interface{}
		why  string
	}{
		{"vmware_alarm_create", map[string]interface{}{"vm": vm, "confirm": true}, "missing spec"},
		{"vmware_alarm_create", map[string]interface{}{"spec": spec, "confirm": true}, "missing entity/vm/host"},
		{"vmware_alarm_reconfigure", map[string]interface{}{"spec": spec, "confirm": true}, "missing alarm"},
		{"vmware_alarm_reconfigure", map[string]interface{}{"alarm": "alarm-1", "confirm": true}, "missing spec"},
		{"vmware_alarm_remove", map[string]interface{}{"confirm": true}, "missing alarm"},
		{"vmware_alarm_get_state", map[string]interface{}{}, "missing entity/vm/host"},
		{"vmware_alarm_actions_enabled", map[string]interface{}{}, "missing entity/vm/host"},
		{"vmware_alarm_acknowledge", map[string]interface{}{"vm": vm, "confirm": true}, "missing alarm"},
		{"vmware_alarm_acknowledge", map[string]interface{}{"alarm": "alarm-1", "confirm": true}, "missing entity/vm/host"},
		{"vmware_alarm_enable_actions", map[string]interface{}{"confirm": true}, "missing entity/vm/host"},
		{"vmware_alarm_disable_actions", map[string]interface{}{"confirm": true}, "missing entity/vm/host"},
	}

	for _, tc := range cases {
		t.Run(tc.name+"/"+tc.why, func(t *testing.T) {
			if _, err := r.CallTool(tc.name, tc.args); err == nil {
				t.Fatalf("expected an error (%s) before any round trip", tc.why)
			}
		})
	}
}

// TestAlarmTools_GateAndConfirm proves the tier1/tier2 destructive
// protection is wired on all 7 mutating alarm tools: a closed
// --allow-destructive gate denies the call, and an open gate still requires
// confirm:true. Same shape as generated_host_iscsi_portbinding_test.go's
// GateAndConfirm test.
func TestAlarmTools_GateAndConfirm(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	closed := newAlarmRegistry(context.Background(), c, RegistryOptions{})
	open := newAlarmRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	vm := firstVMPath(t, open)
	spec := alarmMinimalSpec("Gate Test Alarm")

	cases := []struct {
		name string
		args map[string]interface{}
	}{
		{"vmware_alarm_create", map[string]interface{}{"vm": vm, "spec": spec, "confirm": true}},
		{"vmware_alarm_reconfigure", map[string]interface{}{"alarm": "alarm-1", "spec": spec, "confirm": true}},
		{"vmware_alarm_remove", map[string]interface{}{"alarm": "alarm-1", "confirm": true}},
		{"vmware_alarm_acknowledge", map[string]interface{}{"alarm": "alarm-1", "vm": vm, "confirm": true}},
		{"vmware_alarm_clear_triggered", map[string]interface{}{"confirm": true}},
		{"vmware_alarm_enable_actions", map[string]interface{}{"vm": vm, "confirm": true}},
		{"vmware_alarm_disable_actions", map[string]interface{}{"vm": vm, "confirm": true}},
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
}

// TestAlarmTools_RealSuccess drives the 5 tools backed by a working
// referencia/govmomi/simulator/alarm_manager.go handler (GetAlarm,
// CreateAlarm, AcknowledgeAlarm, Alarm.ReconfigureAlarm, Alarm.RemoveAlarm —
// see generated_alarm.go's top doc comment) through one real create ->
// list -> reconfigure -> acknowledge -> remove -> list lifecycle against
// simulator.VPX(), asserting actual state, not just "no error".
func TestAlarmTools_RealSuccess(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newAlarmRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	vm := firstVMPath(t, r)

	const alarmName = "MCPVMWare RealSuccess Test Alarm"
	spec := map[string]interface{}{
		"name":        alarmName,
		"description": "created by TestAlarmTools_RealSuccess",
		"enabled":     true,
		"expression": map[string]interface{}{
			"_vimType":    "EventAlarmExpression",
			"eventTypeId": "com.vmware.test.RealSuccessEvent",
			"objectType":  "VirtualMachine",
			"status":      "red",
		},
	}

	// Create.
	rawCreate, err := r.CallTool("vmware_alarm_create", map[string]interface{}{"vm": vm, "spec": spec, "confirm": true})
	if err != nil {
		t.Fatalf("vmware_alarm_create failed: %v", err)
	}
	created := decodeResult(t, rawCreate)
	alarmID, _ := created["alarm"].(string)
	if alarmID == "" {
		t.Fatalf("vmware_alarm_create did not return an alarm id: %v", created)
	}

	// Get — the new alarm must be present in the full (no entity filter)
	// listing, with the name we created it with.
	rawGet, err := r.CallTool("vmware_alarm_get", map[string]interface{}{})
	if err != nil {
		t.Fatalf("vmware_alarm_get failed: %v", err)
	}
	got := decodeResult(t, rawGet)
	alarms, _ := got["alarms"].([]interface{})
	found := false
	for _, a := range alarms {
		m, _ := a.(map[string]interface{})
		if m["alarm"] == alarmID {
			found = true
			if m["name"] != alarmName {
				t.Errorf("vmware_alarm_get: alarm %s has name %v, want %q", alarmID, m["name"], alarmName)
			}
		}
	}
	if !found {
		t.Fatalf("vmware_alarm_get did not return the newly created alarm %s among %d alarms", alarmID, len(alarms))
	}

	// Reconfigure.
	newSpec := map[string]interface{}{
		"name":        alarmName,
		"description": "reconfigured by TestAlarmTools_RealSuccess",
		"enabled":     false,
		"expression": map[string]interface{}{
			"_vimType":    "EventAlarmExpression",
			"eventTypeId": "com.vmware.test.RealSuccessEvent",
			"objectType":  "VirtualMachine",
			"status":      "yellow",
		},
	}
	if _, err := r.CallTool("vmware_alarm_reconfigure", map[string]interface{}{"alarm": alarmID, "spec": newSpec, "confirm": true}); err != nil {
		t.Fatalf("vmware_alarm_reconfigure failed: %v", err)
	}

	// Acknowledge — no triggered state exists for this alarm (nothing
	// posted the matching event), so the simulator's AcknowledgeAlarm is a
	// documented no-op (see generated_alarm.go's handleAlarmAcknowledge
	// doc comment) — success, not an error.
	if _, err := r.CallTool("vmware_alarm_acknowledge", map[string]interface{}{"alarm": alarmID, "vm": vm, "confirm": true}); err != nil {
		t.Fatalf("vmware_alarm_acknowledge failed: %v", err)
	}

	// Remove.
	if _, err := r.CallTool("vmware_alarm_remove", map[string]interface{}{"alarm": alarmID, "confirm": true}); err != nil {
		t.Fatalf("vmware_alarm_remove failed: %v", err)
	}

	// Get again — the removed alarm must be gone.
	rawGet2, err := r.CallTool("vmware_alarm_get", map[string]interface{}{})
	if err != nil {
		t.Fatalf("vmware_alarm_get (post-remove) failed: %v", err)
	}
	got2 := decodeResult(t, rawGet2)
	alarms2, _ := got2["alarms"].([]interface{})
	for _, a := range alarms2 {
		m, _ := a.(map[string]interface{})
		if m["alarm"] == alarmID {
			t.Fatalf("vmware_alarm_remove did not remove alarm %s — still present in vmware_alarm_get", alarmID)
		}
	}
}

// TestAlarmTools_ReachesServer drives the 5 tools whose vim25 method has NO
// handler on referencia/govmomi/simulator/alarm_manager.go's *AlarmManager
// (GetAlarmState, ClearTriggeredAlarms, EnableAlarmActions — used by both
// enable_actions and disable_actions — and AreAlarmActionsEnabled): each
// call is expected to reach vcsim's dispatcher and come back with a clean
// MethodNotFound-style server fault (referencia/govmomi/simulator/
// simulator.go's generic dispatch fallback), proving the wiring — schema,
// gate, entity resolution, AlarmManager MoRef, raw method dispatch —
// reaches vcsim and returns a clean server-side error, not an unknown-tool
// wiring bug or a recovered panic. Same helper/rationale as
// generated_host_iscsi_portbinding_test.go's ReachesServer test.
func TestAlarmTools_ReachesServer(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newAlarmRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	vm := firstVMPath(t, r)

	cases := []struct {
		name string
		args map[string]interface{}
	}{
		{"vmware_alarm_get_state", map[string]interface{}{"vm": vm}},
		{"vmware_alarm_actions_enabled", map[string]interface{}{"vm": vm}},
		{"vmware_alarm_clear_triggered", map[string]interface{}{"confirm": true}},
		{"vmware_alarm_enable_actions", map[string]interface{}{"vm": vm, "confirm": true}},
		{"vmware_alarm_disable_actions", map[string]interface{}{"vm": vm, "confirm": true}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := r.CallTool(tc.name, tc.args)
			assertReachesServer(t, err, tc.name)
		})
	}
}

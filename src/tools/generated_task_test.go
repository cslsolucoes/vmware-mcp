package tools

import (
	"context"
	"testing"
	"time"

	"github.com/vmware/govmomi/simulator"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// newTaskRegistry builds a Registry the normal way and layers this group's
// task tools on top via withClass, same pattern as
// generated_vm_lifecycle_test.go's newLifecycleRegistry — this file must not
// edit registry.go itself (the orchestrator wires that in after integrating
// all 3 parallel groups of this phase).
func newTaskRegistry(ctx context.Context, c *vmware.Client, opts RegistryOptions) *Registry {
	r := NewRegistry(ctx, c, opts)
	r.withClass(modeVSphereGeneral, registerTaskTools)
	return r
}

// startDelayedPowerOffTask triggers a real PowerOffVM_Task against vmPath
// and returns its moref Value while it is guaranteed to still be in the
// "running" state for at least delayMS — see generated_task.go's top doc
// comment. This works via simulator.TaskDelay.MethodDelay["PowerOff"]:
// confirmed against referencia/govmomi/simulator/virtual_machine.go's
// PowerOffVMTask (calls CreateTask(runner.Reference(), "powerOff", ...)) and
// referencia/govmomi/simulator/task.go's CreateTask (ucFirst("powerOff") ==
// "PowerOff" becomes task.Info.Name — NOT "PowerOff_Task"; task.go has an
// explicit comment on this exact API-name-vs-task-name difference) and
// Task.Run (calls TaskDelay.delay(t.Info.Name) — i.e. delay("PowerOff") —
// from a goroutine, AFTER already flipping info.state to "running"
// synchronously). Restores the shared simulator.TaskDelay package var via
// t.Cleanup since it is global state shared by every test in this binary.
func startDelayedPowerOffTask(t *testing.T, ctx context.Context, c *vmware.Client, vmPath string, delayMS int) string {
	t.Helper()
	orig := simulator.TaskDelay
	simulator.TaskDelay = simulator.DelayConfig{MethodDelay: map[string]int{"PowerOff": delayMS}}
	t.Cleanup(func() { simulator.TaskDelay = orig })

	vm, err := c.Finder.VirtualMachine(ctx, vmPath)
	if err != nil {
		t.Fatalf("failed to resolve %s: %v", vmPath, err)
	}
	task, err := vm.PowerOff(ctx)
	if err != nil {
		t.Fatalf("PowerOff(%s) failed to start: %v", vmPath, err)
	}
	return task.Reference().Value
}

// TestTaskTools_MutateRunningTask proves vmware_task_update_progress,
// vmware_task_set_description, vmware_task_set_state, and
// vmware_task_cancel all reach vcsim's real Task SOAP methods and succeed
// against a task that is genuinely still "running" — not just a
// registration smoke test. Order matters: vmware_task_cancel is called
// last, because CancelTask immediately marks the task done (state->error)
// server-side (referencia/govmomi/simulator/task.go's CancelTask), which
// would make every later SetTaskState/SetTaskDescription/UpdateProgress
// call fault InvalidState — a real business rule (see
// TestTaskTools_GateConfirmAndCompletedTaskBusinessRule below), not a bug,
// but it would break this test's "expect success" assumptions if called out
// of order.
func TestTaskTools_MutateRunningTask(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newTaskRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	vm := firstVMPath(t, r)

	if vmInfo(t, r, vm)["power_state"] != "poweredOn" {
		if _, err := r.CallTool("vmware_vm_power_on", map[string]interface{}{"vm": vm}); err != nil {
			t.Fatalf("vmware_vm_power_on failed: %v", err)
		}
	}

	taskVal := startDelayedPowerOffTask(t, context.Background(), c, vm, 3000)

	if _, err := r.CallTool("vmware_task_update_progress", map[string]interface{}{"task": taskVal, "percent_done": float64(42), "confirm": true}); err != nil {
		t.Fatalf("vmware_task_update_progress on a running task failed: %v", err)
	}

	if _, err := r.CallTool("vmware_task_set_description", map[string]interface{}{
		"task":        taskVal,
		"description": map[string]interface{}{"key": "com.mcpvmware.test", "message": "test phase"},
		"confirm":     true,
	}); err != nil {
		t.Fatalf("vmware_task_set_description on a running task failed: %v", err)
	}

	if _, err := r.CallTool("vmware_task_set_state", map[string]interface{}{"task": taskVal, "state": "running", "confirm": true}); err != nil {
		t.Fatalf("vmware_task_set_state(running) on a running task failed: %v", err)
	}

	raw, err := r.CallTool("vmware_task_cancel", map[string]interface{}{"task": taskVal, "confirm": true})
	if err != nil {
		t.Fatalf("vmware_task_cancel on a running task failed: %v", err)
	}
	if m := decodeResult(t, raw); m["result"] != "cancel_requested" {
		t.Fatalf("expected result=cancel_requested, got %v (%s)", m["result"], raw)
	}
}

// TestTaskTools_WaitForRealCompletion proves vmware_task_wait actually
// blocks until a real (undelayed) task finishes and reports its final
// state — the primary success path for the 4-variants-collapsed-into-1 tool
// described in generated_task.go's top doc comment. Verifies the VM's power
// state is actually observably poweredOff through vmware_vm_info afterward,
// not just that the tool call itself returned no error.
func TestTaskTools_WaitForRealCompletion(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newTaskRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	vm := firstVMPath(t, r)

	if vmInfo(t, r, vm)["power_state"] != "poweredOn" {
		if _, err := r.CallTool("vmware_vm_power_on", map[string]interface{}{"vm": vm}); err != nil {
			t.Fatalf("vmware_vm_power_on failed: %v", err)
		}
	}

	vmObj, err := c.Finder.VirtualMachine(context.Background(), vm)
	if err != nil {
		t.Fatalf("failed to resolve %s: %v", vm, err)
	}
	task, err := vmObj.PowerOff(context.Background())
	if err != nil {
		t.Fatalf("PowerOff(%s) failed to start: %v", vm, err)
	}

	raw, err := r.CallTool("vmware_task_wait", map[string]interface{}{"task": task.Reference().Value, "timeout_seconds": float64(30)})
	if err != nil {
		t.Fatalf("vmware_task_wait failed: %v", err)
	}
	m := decodeResult(t, raw)
	info, ok := m["info"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected an \"info\" object: %s", raw)
	}
	if info["state"] != "success" {
		t.Fatalf("expected info.state=success, got %v (%s)", info["state"], raw)
	}

	if got := vmInfo(t, r, vm)["power_state"]; got != "poweredOff" {
		t.Fatalf("expected the VM to actually be powered off after vmware_task_wait returned, got %v", got)
	}
}

// TestTaskTools_WaitInvalidTask proves vmware_task_wait fails cleanly and
// quickly for a task moref that does not exist, instead of hanging — the
// same "must not block the whole stdio connection forever" concern
// documented in generated_vm_lifecycle.go's defaultWaitTimeout.
func TestTaskTools_WaitInvalidTask(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newTaskRegistry(context.Background(), c, RegistryOptions{})

	done := make(chan error, 1)
	go func() {
		_, err := r.CallTool("vmware_task_wait", map[string]interface{}{"task": "task-does-not-exist", "timeout_seconds": float64(5)})
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected an error for a nonexistent task moref")
		}
	case <-time.After(15 * time.Second):
		t.Fatal("vmware_task_wait on a nonexistent task did not return within 15s")
	}
}

// TestTaskTools_GateConfirmAndCompletedTaskBusinessRule proves the 4 Tier 2
// tools are genuinely gated (closed gate and missing confirm both deny them
// before any network call), and that acting on an already-completed task
// reaches vcsim's real business-rule validation (an InvalidState-based
// fault) instead of failing on something wired wrong in this file — the
// same "reaches the real business validation, even though a trivial full
// success from this exact state isn't reachable" pattern as
// generated_vm_lifecycle.go's RebootGuest/UpgradeVM tests.
func TestTaskTools_GateConfirmAndCompletedTaskBusinessRule(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	closedGate := newTaskRegistry(context.Background(), c, RegistryOptions{AllowDestructive: false})
	openGate := newTaskRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	vm := firstVMPath(t, openGate)

	for _, tool := range []string{"vmware_task_cancel", "vmware_task_set_description", "vmware_task_set_state", "vmware_task_update_progress"} {
		args := map[string]interface{}{"task": "task-1", "confirm": true}
		switch tool {
		case "vmware_task_set_description":
			args["description"] = map[string]interface{}{"key": "k", "message": "m"}
		case "vmware_task_set_state":
			args["state"] = "running"
		case "vmware_task_update_progress":
			args["percent_done"] = float64(1)
		}
		if _, err := closedGate.CallTool(tool, args); err == nil {
			t.Fatalf("%s: expected denial with the gate closed", tool)
		}
		noConfirm := map[string]interface{}{}
		for k, v := range args {
			if k != "confirm" {
				noConfirm[k] = v
			}
		}
		if _, err := openGate.CallTool(tool, noConfirm); err == nil {
			t.Fatalf("%s: expected denial without confirm:true", tool)
		}
	}

	if vmInfo(t, openGate, vm)["power_state"] != "poweredOn" {
		if _, err := openGate.CallTool("vmware_vm_power_on", map[string]interface{}{"vm": vm}); err != nil {
			t.Fatalf("vmware_vm_power_on failed: %v", err)
		}
	}
	vmObj, err := c.Finder.VirtualMachine(context.Background(), vm)
	if err != nil {
		t.Fatalf("failed to resolve %s: %v", vm, err)
	}
	task, err := vmObj.PowerOff(context.Background())
	if err != nil {
		t.Fatalf("PowerOff(%s) failed to start: %v", vm, err)
	}
	if err := task.Wait(context.Background()); err != nil {
		t.Fatalf("PowerOff(%s) task failed: %v", vm, err)
	}

	_, err = openGate.CallTool("vmware_task_cancel", map[string]interface{}{"task": task.Reference().Value, "confirm": true})
	assertReachesServer(t, err, "vmware_task_cancel (already completed)")
}

// TestTaskTools_SetStateValidation proves vmware_task_set_state validates
// its "state" enum client-side before making any network call.
func TestTaskTools_SetStateValidation(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newTaskRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	if _, err := r.CallTool("vmware_task_set_state", map[string]interface{}{"task": "task-1", "state": "bogus", "confirm": true}); err == nil {
		t.Fatal("expected an error for an invalid state value")
	}
}

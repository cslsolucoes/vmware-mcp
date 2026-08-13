package tools

import (
	"context"
	"testing"
	"time"

	"github.com/vmware/govmomi/simulator"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// vcsim gap, not a bug — see generated_cis_tasks.go's top doc comment. Same
// test discipline as generated_esx_settings_cluster_vms_test.go's top doc
// comment, not repeated verbatim here. None of these 4 tools are
// destructive (no tier gate to prove), so this file focuses on
// registration, required timeout_seconds enforcement, and
// assertReachesServer.

// newCisTasksRegistry builds a Registry the normal way and layers
// registerCisTasksTools on top via withClass — same pattern as this batch's
// other test files; must not edit registry.go.
func newCisTasksRegistry(ctx context.Context, c *vmware.Client, opts RegistryOptions) *Registry {
	r := NewRegistry(ctx, c, opts)
	r.withClass(modeVCenterOnly, registerCisTasksTools)
	return r
}

func TestCisTasksTools_Registration(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newCisTasksRegistry(context.Background(), c, RegistryOptions{})

	want := []string{
		"vmware_cis_tasks_get",
		"vmware_cis_tasks_wait_for_completion",
		"vmware_cis_tasks_wait_for_running_or_error",
		"vmware_cis_tasks_wait_for_running_or_terminal_state",
	}
	if len(want) != 4 {
		t.Fatalf("test bug: want list has %d entries, expected 4", len(want))
	}

	got := map[string]bool{}
	for _, tl := range r.ListTools() {
		got[tl.Name] = true
	}
	for _, name := range want {
		if !got[name] {
			t.Errorf("tool %s not registered", name)
		}
	}
}

// TestCisTasksTools_RequiredArgsValidation proves task_id and timeout_seconds
// are both required, client-side, for all 4 tools — timeout_seconds being
// OBLIGATORY (no default, unlike the optional/defaulted timeoutSecondsArg
// used elsewhere in this project) is the notable, brief-mandated deviation
// this file exists to prove.
func TestCisTasksTools_RequiredArgsValidation(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newCisTasksRegistry(context.Background(), c, RegistryOptions{})

	names := []string{
		"vmware_cis_tasks_get",
		"vmware_cis_tasks_wait_for_completion",
		"vmware_cis_tasks_wait_for_running_or_error",
		"vmware_cis_tasks_wait_for_running_or_terminal_state",
	}

	for _, name := range names {
		t.Run(name+"_missing_task_id", func(t *testing.T) {
			if _, err := r.CallTool(name, map[string]interface{}{"timeout_seconds": float64(5)}); err == nil {
				t.Fatalf("%s: expected an error when task_id is missing", name)
			}
		})

		t.Run(name+"_missing_timeout_seconds", func(t *testing.T) {
			_, err := r.CallTool(name, map[string]interface{}{"task_id": "task-1"})
			if err == nil {
				t.Fatalf("%s: expected an error when timeout_seconds is missing (required, no default)", name)
			}
		})

		t.Run(name+"_zero_timeout_seconds_rejected", func(t *testing.T) {
			// waitTimeoutFrom (reused by requiredWaitTimeout) rejects
			// non-positive durations — confirmed here rather than assumed.
			_, err := r.CallTool(name, map[string]interface{}{"task_id": "task-1", "timeout_seconds": float64(0)})
			if err == nil {
				t.Fatalf("%s: expected an error when timeout_seconds is 0", name)
			}
		})
	}
}

// TestCisTasksTools_ReachesServer proves every one of the 4 tools, given
// fully valid input, reaches the real vcsim server and gets back a genuine
// error — not a panic, not "unknown tool" — bounded by a short
// timeout_seconds so a genuine wiring regression (e.g. the timeout not
// actually being applied) fails fast instead of hanging the test suite,
// same discipline as generated_vm_lifecycle_test.go's
// TestVMLifecycleTools_WaitTimeoutIsBounded.
func TestCisTasksTools_ReachesServer(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newCisTasksRegistry(context.Background(), c, RegistryOptions{})

	names := []string{
		"vmware_cis_tasks_get",
		"vmware_cis_tasks_wait_for_completion",
		"vmware_cis_tasks_wait_for_running_or_error",
		"vmware_cis_tasks_wait_for_running_or_terminal_state",
	}

	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			done := make(chan error, 1)
			go func() {
				_, err := r.CallTool(name, map[string]interface{}{"task_id": "task-1", "timeout_seconds": float64(5)})
				done <- err
			}()
			select {
			case err := <-done:
				assertReachesServer(t, err, name)
			case <-time.After(10 * time.Second):
				t.Fatalf("%s did not return within 10s — timeout_seconds bound is not working", name)
			}
		})
	}
}

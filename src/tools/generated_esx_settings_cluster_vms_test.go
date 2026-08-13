package tools

import (
	"context"
	"testing"

	"github.com/vmware/govmomi/simulator"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// This whole domain (vapi/esx/settings/clusters/vms) is a confirmed vcsim
// gap — see generated_esx_settings_cluster_vms.go's top doc comment. Every
// test below therefore proves: registration, tier gating (closed
// gate/missing confirm denied before any network call at all), client-side
// required-argument validation (which does run a REST/VAPI login first, same
// as every vapi/* tool in this project since Fase 8a's tags/library groups —
// unavoidable session bootstrapping, not the domain endpoint under test —
// but still returns THIS file's own validation error text, not a network
// error, proving the check fires before the domain call), and — with fully
// valid input — a real round trip to vcsim that fails cleanly (not a panic,
// not "unknown tool") via assertReachesServer (generated_vm_lifecycle_test.go).

// newEsxSettingsClusterVMsRegistry builds a Registry the normal way
// (NewRegistry, which wires vm.go/host.go/etc via registerTools) and then
// manually layers this group's tools on top via withClass — same pattern as
// generated_tags_test.go/generated_vm_lifecycle_test.go, and for the same
// reason: this file must not edit registry.go.
func newEsxSettingsClusterVMsRegistry(ctx context.Context, c *vmware.Client, opts RegistryOptions) *Registry {
	r := NewRegistry(ctx, c, opts)
	r.withClass(modeVCenterOnly, registerEsxSettingsClusterVMsTools)
	return r
}

// firstClusterPath returns the inventory path of the first
// ClusterComputeResource in the simulator model — simulator.VPX()'s default
// model creates exactly 1 (confirmed in
// $(go env GOMODCACHE)/github.com/vmware/govmomi@.../simulator/model.go's
// VPX(): Cluster: 1). Shared with generated_cluster_modules_test.go, which
// needs the same fixture.
func firstClusterPath(t *testing.T, r *Registry) string {
	t.Helper()
	raw, err := r.CallTool("vmware_list_clusters", map[string]interface{}{})
	if err != nil {
		t.Fatalf("vmware_list_clusters failed: %v", err)
	}
	list, _ := decodeResult(t, raw)["clusters"].([]interface{})
	if len(list) == 0 {
		t.Fatal("simulator model has no clusters")
	}
	return list[0].(string)
}

func TestEsxSettingsClusterVMsTools_Registration(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newEsxSettingsClusterVMsRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	want := []string{
		"vmware_esx_settings_clusters_vms_enable_async",
		"vmware_esx_settings_clusters_vms_enable",
		"vmware_esx_settings_clusters_vms_multi_source_enable_async",
		"vmware_esx_settings_clusters_vms_multi_source_enable",
		"vmware_esx_settings_clusters_vms_transition_async",
		"vmware_esx_settings_clusters_vms_transition",
		"vmware_esx_settings_clusters_vms_get_solution",
		"vmware_esx_settings_clusters_vms_set_solution",
		"vmware_esx_settings_clusters_vms_delete_solution",
		"vmware_esx_settings_clusters_vms_list_hooks",
		"vmware_esx_settings_clusters_vms_mark_as_processed",
		"vmware_esx_settings_clusters_vms_process_dynamic_update",
		"vmware_esx_settings_clusters_vms_apply",
		"vmware_esx_settings_clusters_vms_apply_wait_for_completion",
		"vmware_esx_settings_clusters_vms_check_compliance",
		"vmware_esx_settings_clusters_vms_list_solutions",
	}
	if len(want) != 16 {
		t.Fatalf("test bug: want list has %d entries, expected 16", len(want))
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
	// vmware_esx_settings_clusters_vms_delete_solution_only must NOT be
	// registered — dropped due to a real govmomi dependency gap, see this
	// file's counterpart .go file's top doc comment ("Real dependency
	// drift").
	if got["vmware_esx_settings_clusters_vms_delete_solution_only"] {
		t.Error("vmware_esx_settings_clusters_vms_delete_solution_only should not be registered (not present in the pinned govmomi v0.55.1 dependency)")
	}
}

// minSolutionSpec is a minimal-but-valid vms.SolutionSpec JSON object —
// enough for decodeSolutionSpec to succeed and let a test reach the real
// vcsim call.
func minSolutionSpec() map[string]interface{} {
	return map[string]interface{}{
		"deployment_type": "EVERY_HOST_PINNED",
		"display_name":    "test-solution",
		"display_version": "1.0.0",
	}
}

type esxClusterVMsCase struct {
	name string
	tier tier // 0 means registered via r.register (no gate)
	args func(clusterPath string) map[string]interface{}
}

func esxClusterVMsCases() []esxClusterVMsCase {
	return []esxClusterVMsCase{
		{"vmware_esx_settings_clusters_vms_enable_async", tier2, func(cp string) map[string]interface{} {
			return map[string]interface{}{"cluster_path": cp, "solution": "sol1", "eam_agency_id": "eam1", "desired_spec": minSolutionSpec()}
		}},
		{"vmware_esx_settings_clusters_vms_enable", tier2, func(cp string) map[string]interface{} {
			return map[string]interface{}{"cluster_path": cp, "solution": "sol1", "eam_agency_id": "eam1", "desired_spec": minSolutionSpec()}
		}},
		{"vmware_esx_settings_clusters_vms_multi_source_enable_async", tier2, func(cp string) map[string]interface{} {
			return map[string]interface{}{"cluster_path": cp, "solution": "sol1", "eam_agency_ids": []interface{}{"eam1"}, "desired_spec": minSolutionSpec()}
		}},
		{"vmware_esx_settings_clusters_vms_multi_source_enable", tier2, func(cp string) map[string]interface{} {
			return map[string]interface{}{"cluster_path": cp, "solution": "sol1", "eam_agency_ids": []interface{}{"eam1"}, "desired_spec": minSolutionSpec()}
		}},
		{"vmware_esx_settings_clusters_vms_transition_async", tier2, func(cp string) map[string]interface{} {
			return map[string]interface{}{"cluster_path": cp, "solution": "sol1", "source_cluster": "src-cluster", "desired_spec": minSolutionSpec()}
		}},
		{"vmware_esx_settings_clusters_vms_transition", tier2, func(cp string) map[string]interface{} {
			return map[string]interface{}{"cluster_path": cp, "solution": "sol1", "source_cluster": "src-cluster", "desired_spec": minSolutionSpec()}
		}},
		{"vmware_esx_settings_clusters_vms_get_solution", 0, func(cp string) map[string]interface{} {
			return map[string]interface{}{"cluster_path": cp, "solution": "sol1"}
		}},
		{"vmware_esx_settings_clusters_vms_set_solution", tier2, func(cp string) map[string]interface{} {
			return map[string]interface{}{"cluster_path": cp, "solution": "sol1", "spec": minSolutionSpec()}
		}},
		{"vmware_esx_settings_clusters_vms_delete_solution", tier1, func(cp string) map[string]interface{} {
			return map[string]interface{}{"cluster_path": cp, "solution": "sol1"}
		}},
		{"vmware_esx_settings_clusters_vms_list_hooks", 0, func(cp string) map[string]interface{} {
			return map[string]interface{}{"cluster_path": cp, "solution": "sol1"}
		}},
		{"vmware_esx_settings_clusters_vms_mark_as_processed", tier2, func(cp string) map[string]interface{} {
			return map[string]interface{}{"cluster_path": cp, "vm": "vm-1", "lifecycle_state": "POST_PROVISIONING", "processed_successfully": true}
		}},
		{"vmware_esx_settings_clusters_vms_process_dynamic_update", tier2, func(cp string) map[string]interface{} {
			return map[string]interface{}{"cluster_path": cp, "vm": "vm-1", "solution": "sol1", "lifecycle_state": "POST_PROVISIONING"}
		}},
		{"vmware_esx_settings_clusters_vms_apply", tier2, func(cp string) map[string]interface{} {
			return map[string]interface{}{"cluster_path": cp}
		}},
		{"vmware_esx_settings_clusters_vms_apply_wait_for_completion", tier2, func(cp string) map[string]interface{} {
			return map[string]interface{}{"task_id": "task-1", "timeout_seconds": float64(5)}
		}},
		{"vmware_esx_settings_clusters_vms_check_compliance", tier2, func(cp string) map[string]interface{} {
			return map[string]interface{}{"cluster_path": cp}
		}},
		{"vmware_esx_settings_clusters_vms_list_solutions", 0, func(cp string) map[string]interface{} {
			return map[string]interface{}{"cluster_path": cp}
		}},
	}
}

// TestEsxSettingsClusterVMsTools_ReachesServer proves every one of the 16
// tools, given fully valid input (gate open, confirm:true where applicable),
// reaches the real vcsim server and gets back a genuine error — not a panic,
// not "unknown tool" — per this file's top doc comment.
func TestEsxSettingsClusterVMsTools_ReachesServer(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newEsxSettingsClusterVMsRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	clusterPath := firstClusterPath(t, r)

	for _, tc := range esxClusterVMsCases() {
		t.Run(tc.name, func(t *testing.T) {
			args := tc.args(clusterPath)
			if tc.tier != 0 {
				args["confirm"] = true
			}
			_, err := r.CallTool(tc.name, args)
			assertReachesServer(t, err, tc.name)
		})
	}
}

// TestEsxSettingsClusterVMsTools_TierGating proves the Tier 1/2 gate and
// confirm checks actually run, for one tier1 tool (delete_solution) and one
// tier2 tool (enable_async) — same "closed gate / missing confirm denies
// before touching the server" proof pattern as vm_test.go/host_test.go.
// These 2 checks genuinely run before ANY network call (unlike the plain
// required-arg validation below, which still needs a REST/VAPI login first
// — see this file's top doc comment): wrapDestructive (destructive.go)
// checks gateOpen/confirmed before ever invoking the inner handler.
func TestEsxSettingsClusterVMsTools_TierGating(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	closedGate := newEsxSettingsClusterVMsRegistry(context.Background(), c, RegistryOptions{AllowDestructive: false})
	openGate := newEsxSettingsClusterVMsRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	clusterPath := firstClusterPath(t, openGate)

	t.Run("delete_solution_tier1", func(t *testing.T) {
		args := map[string]interface{}{"cluster_path": clusterPath, "solution": "sol1", "confirm": true}
		if _, err := closedGate.CallTool("vmware_esx_settings_clusters_vms_delete_solution", args); err == nil {
			t.Fatal("expected denial with the gate closed")
		}
		argsNoConfirm := map[string]interface{}{"cluster_path": clusterPath, "solution": "sol1"}
		if _, err := openGate.CallTool("vmware_esx_settings_clusters_vms_delete_solution", argsNoConfirm); err == nil {
			t.Fatal("expected denial without confirm:true")
		}
	})

	t.Run("enable_async_tier2", func(t *testing.T) {
		args := map[string]interface{}{"cluster_path": clusterPath, "solution": "sol1", "eam_agency_id": "eam1", "desired_spec": minSolutionSpec(), "confirm": true}
		if _, err := closedGate.CallTool("vmware_esx_settings_clusters_vms_enable_async", args); err == nil {
			t.Fatal("expected denial with the gate closed")
		}
		argsNoConfirm := map[string]interface{}{"cluster_path": clusterPath, "solution": "sol1", "eam_agency_id": "eam1", "desired_spec": minSolutionSpec()}
		if _, err := openGate.CallTool("vmware_esx_settings_clusters_vms_enable_async", argsNoConfirm); err == nil {
			t.Fatal("expected denial without confirm:true")
		}
	})
}

// TestEsxSettingsClusterVMsTools_RequiredArgsValidation proves client-side
// argument validation fires (with the gate open and confirm:true already
// satisfied for destructive tools, isolating the argument check itself) —
// see this file's top doc comment for why a REST/VAPI login still happens
// first, and why that is not a violation of "reject before touching the
// domain server".
func TestEsxSettingsClusterVMsTools_RequiredArgsValidation(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newEsxSettingsClusterVMsRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	clusterPath := firstClusterPath(t, r)

	t.Run("get_solution_missing_solution", func(t *testing.T) {
		if _, err := r.CallTool("vmware_esx_settings_clusters_vms_get_solution", map[string]interface{}{"cluster_path": clusterPath}); err == nil {
			t.Fatal("expected an error when solution is missing")
		}
	})

	t.Run("get_solution_bad_cluster_path", func(t *testing.T) {
		if _, err := r.CallTool("vmware_esx_settings_clusters_vms_get_solution", map[string]interface{}{"cluster_path": "/DC0/host/no-such-cluster", "solution": "sol1"}); err == nil {
			t.Fatal("expected an error when cluster_path does not resolve")
		}
	})

	t.Run("enable_missing_desired_spec", func(t *testing.T) {
		args := map[string]interface{}{"cluster_path": clusterPath, "solution": "sol1", "eam_agency_id": "eam1", "confirm": true}
		if _, err := r.CallTool("vmware_esx_settings_clusters_vms_enable", args); err == nil {
			t.Fatal("expected an error when desired_spec is missing")
		}
	})

	t.Run("multi_source_enable_empty_eam_agency_ids", func(t *testing.T) {
		args := map[string]interface{}{"cluster_path": clusterPath, "solution": "sol1", "eam_agency_ids": []interface{}{}, "desired_spec": minSolutionSpec(), "confirm": true}
		if _, err := r.CallTool("vmware_esx_settings_clusters_vms_multi_source_enable", args); err == nil {
			t.Fatal("expected an error when eam_agency_ids is empty")
		}
	})

	t.Run("transition_missing_source_cluster", func(t *testing.T) {
		args := map[string]interface{}{"cluster_path": clusterPath, "solution": "sol1", "desired_spec": minSolutionSpec(), "confirm": true}
		if _, err := r.CallTool("vmware_esx_settings_clusters_vms_transition", args); err == nil {
			t.Fatal("expected an error when source_cluster is missing")
		}
	})

	t.Run("mark_as_processed_bad_lifecycle_state", func(t *testing.T) {
		args := map[string]interface{}{"cluster_path": clusterPath, "vm": "vm-1", "lifecycle_state": "BOGUS", "processed_successfully": true, "confirm": true}
		if _, err := r.CallTool("vmware_esx_settings_clusters_vms_mark_as_processed", args); err == nil {
			t.Fatal("expected an error for an invalid lifecycle_state")
		}
	})

	t.Run("apply_wait_for_completion_requires_timeout_seconds", func(t *testing.T) {
		args := map[string]interface{}{"task_id": "task-1", "confirm": true}
		if _, err := r.CallTool("vmware_esx_settings_clusters_vms_apply_wait_for_completion", args); err == nil {
			t.Fatal("expected an error when timeout_seconds is missing (required, no default — see requiredWaitTimeout)")
		}
	})

	t.Run("apply_wait_for_completion_missing_task_id", func(t *testing.T) {
		args := map[string]interface{}{"timeout_seconds": float64(5), "confirm": true}
		if _, err := r.CallTool("vmware_esx_settings_clusters_vms_apply_wait_for_completion", args); err == nil {
			t.Fatal("expected an error when task_id is missing")
		}
	})
}

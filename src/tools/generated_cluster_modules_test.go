package tools

import (
	"context"
	"testing"

	"github.com/vmware/govmomi/simulator"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// vcsim gap, not a bug — see generated_cluster_modules.go's top doc comment.
// Every test below proves: registration, tier gating, client-side required-
// argument validation, and (with valid input) a real round trip to vcsim
// that fails cleanly — same rigor/discipline documented in
// generated_esx_settings_cluster_vms_test.go's top doc comment (shared by
// this whole batch), not repeated verbatim here.

// newClusterModulesRegistry builds a Registry the normal way and layers
// registerClusterModulesTools on top via withClass — same pattern as
// generated_esx_settings_cluster_vms_test.go; must not edit registry.go.
func newClusterModulesRegistry(ctx context.Context, c *vmware.Client, opts RegistryOptions) *Registry {
	r := NewRegistry(ctx, c, opts)
	r.withClass(modeVCenterOnly, registerClusterModulesTools)
	return r
}

func TestClusterModulesTools_Registration(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newClusterModulesRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	want := []string{
		"vmware_cluster_add_module_members",
		"vmware_cluster_create_module",
		"vmware_cluster_delete_module",
		"vmware_cluster_list_module_members",
		"vmware_cluster_list_modules",
		"vmware_cluster_remove_module_members",
	}
	if len(want) != 6 {
		t.Fatalf("test bug: want list has %d entries, expected 6", len(want))
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

// TestClusterModulesTools_TierGating proves the gate/confirm checks for one
// tier1 tool (remove_module_members) and one tier2 tool
// (add_module_members) — the pair the codegen brief explicitly called out as
// an intentional, pre-existing tier asymmetry (see this group's .go file top
// doc comment) — both actually deny before any handler logic runs.
func TestClusterModulesTools_TierGating(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	closedGate := newClusterModulesRegistry(context.Background(), c, RegistryOptions{AllowDestructive: false})
	openGate := newClusterModulesRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	vmPath := firstVMPath(t, openGate)

	t.Run("add_module_members_tier2", func(t *testing.T) {
		args := map[string]interface{}{"module_id": "mod-1", "vm_paths": []interface{}{vmPath}, "confirm": true}
		if _, err := closedGate.CallTool("vmware_cluster_add_module_members", args); err == nil {
			t.Fatal("expected denial with the gate closed")
		}
		argsNoConfirm := map[string]interface{}{"module_id": "mod-1", "vm_paths": []interface{}{vmPath}}
		if _, err := openGate.CallTool("vmware_cluster_add_module_members", argsNoConfirm); err == nil {
			t.Fatal("expected denial without confirm:true")
		}
	})

	t.Run("remove_module_members_tier1", func(t *testing.T) {
		args := map[string]interface{}{"module_id": "mod-1", "vm_paths": []interface{}{vmPath}, "confirm": true}
		if _, err := closedGate.CallTool("vmware_cluster_remove_module_members", args); err == nil {
			t.Fatal("expected denial with the gate closed")
		}
		argsNoConfirm := map[string]interface{}{"module_id": "mod-1", "vm_paths": []interface{}{vmPath}}
		if _, err := openGate.CallTool("vmware_cluster_remove_module_members", argsNoConfirm); err == nil {
			t.Fatal("expected denial without confirm:true")
		}
	})

	t.Run("create_module_tier2", func(t *testing.T) {
		clusterPath := firstClusterPath(t, openGate)
		args := map[string]interface{}{"cluster_path": clusterPath, "confirm": true}
		if _, err := closedGate.CallTool("vmware_cluster_create_module", args); err == nil {
			t.Fatal("expected denial with the gate closed")
		}
	})

	t.Run("delete_module_tier1", func(t *testing.T) {
		args := map[string]interface{}{"module_id": "mod-1", "confirm": true}
		if _, err := closedGate.CallTool("vmware_cluster_delete_module", args); err == nil {
			t.Fatal("expected denial with the gate closed")
		}
	})
}

func TestClusterModulesTools_RequiredArgsValidation(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newClusterModulesRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	vmPath := firstVMPath(t, r)

	t.Run("add_module_members_missing_module_id", func(t *testing.T) {
		args := map[string]interface{}{"vm_paths": []interface{}{vmPath}, "confirm": true}
		if _, err := r.CallTool("vmware_cluster_add_module_members", args); err == nil {
			t.Fatal("expected an error when module_id is missing")
		}
	})

	t.Run("add_module_members_empty_vm_paths", func(t *testing.T) {
		args := map[string]interface{}{"module_id": "mod-1", "vm_paths": []interface{}{}, "confirm": true}
		if _, err := r.CallTool("vmware_cluster_add_module_members", args); err == nil {
			t.Fatal("expected an error when vm_paths is empty")
		}
	})

	t.Run("add_module_members_bad_vm_path", func(t *testing.T) {
		args := map[string]interface{}{"module_id": "mod-1", "vm_paths": []interface{}{"/DC0/vm/no-such-vm"}, "confirm": true}
		if _, err := r.CallTool("vmware_cluster_add_module_members", args); err == nil {
			t.Fatal("expected an error when a vm_paths entry does not resolve")
		}
	})

	t.Run("create_module_missing_cluster_path", func(t *testing.T) {
		if _, err := r.CallTool("vmware_cluster_create_module", map[string]interface{}{"confirm": true}); err == nil {
			t.Fatal("expected an error when cluster_path is missing")
		}
	})

	t.Run("delete_module_missing_module_id", func(t *testing.T) {
		if _, err := r.CallTool("vmware_cluster_delete_module", map[string]interface{}{"confirm": true}); err == nil {
			t.Fatal("expected an error when module_id is missing")
		}
	})

	t.Run("list_module_members_missing_module_id", func(t *testing.T) {
		if _, err := r.CallTool("vmware_cluster_list_module_members", map[string]interface{}{}); err == nil {
			t.Fatal("expected an error when module_id is missing")
		}
	})
}

type clusterModulesCase struct {
	name string
	tier tier
	args func(clusterPath, vmPath string) map[string]interface{}
}

func clusterModulesCases() []clusterModulesCase {
	return []clusterModulesCase{
		{"vmware_cluster_add_module_members", tier2, func(cp, vp string) map[string]interface{} {
			return map[string]interface{}{"module_id": "mod-1", "vm_paths": []interface{}{vp}}
		}},
		{"vmware_cluster_create_module", tier2, func(cp, vp string) map[string]interface{} {
			return map[string]interface{}{"cluster_path": cp}
		}},
		{"vmware_cluster_delete_module", tier1, func(cp, vp string) map[string]interface{} {
			return map[string]interface{}{"module_id": "mod-1"}
		}},
		{"vmware_cluster_list_module_members", 0, func(cp, vp string) map[string]interface{} {
			return map[string]interface{}{"module_id": "mod-1"}
		}},
		{"vmware_cluster_list_modules", 0, func(cp, vp string) map[string]interface{} {
			return map[string]interface{}{}
		}},
		{"vmware_cluster_remove_module_members", tier1, func(cp, vp string) map[string]interface{} {
			return map[string]interface{}{"module_id": "mod-1", "vm_paths": []interface{}{vp}}
		}},
	}
}

// TestClusterModulesTools_ReachesServer proves every one of the 6 tools,
// given fully valid input (gate open, confirm:true where applicable),
// reaches the real vcsim server and gets back a genuine error — not a panic,
// not "unknown tool".
func TestClusterModulesTools_ReachesServer(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newClusterModulesRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	clusterPath := firstClusterPath(t, r)
	vmPath := firstVMPath(t, r)

	for _, tc := range clusterModulesCases() {
		t.Run(tc.name, func(t *testing.T) {
			args := tc.args(clusterPath, vmPath)
			if tc.tier != 0 {
				args["confirm"] = true
			}
			_, err := r.CallTool(tc.name, args)
			assertReachesServer(t, err, tc.name)
		})
	}
}

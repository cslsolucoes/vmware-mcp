package tools

import (
	"context"
	"testing"

	"github.com/vmware/govmomi/simulator"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// This file drives registerClusterTools (generated_cluster.go) against a
// SINGLE vcsim instance shared by every subtest below via one TestClusterTools
// function with t.Run — not a separate simulator.VPX() per Test function like
// some earlier generated_*_test.go files in this package (e.g.
// generated_vm_ft_test.go, generated_cluster_modules_test.go) — a
// *vmware.Client is a thin wrapper over an already-established SOAP session;
// building a fresh *Registry against the SAME client via newClusterRegistry
// for each phase (Registration/Validation/GateAndConfirm/ReachesServer) is
// cheap (pure Go struct wiring, no reconnect) and avoids paying vcsim
// startup cost 4 times over for one tool group.
//
// None of the 13 methods under test have a vcsim-side handler (see
// generated_cluster.go's top doc comment "vcsim coverage") — every
// ReachesServer case is therefore expected to fault types.MethodNotFound via
// assertReachesServer (generated_vm_lifecycle_test.go), proving the wiring
// (schema, tier gate, resolveClusterComputeResource/resolveHost/
// resolveResourcePool/resolveVM, raw SOAP dispatch) reaches vcsim, not that
// vcsim actually simulates DRS/HA/HCI behavior.

// newClusterRegistry builds a Registry the normal way (NewRegistry, which
// wires vm.go/host.go/etc — including firstVMPath's vmware_list_vms,
// firstHostPath's vmware_list_hosts, firstClusterPath's vmware_list_clusters,
// and firstResourcePoolPath's vmware_list_resource_pools — via
// registerTools) and then manually layers this group's cluster tools on top
// via withClass, exactly as registry.go's real wiring for
// registerClusterTools will do once another change adds it there — this file
// must not edit registry.go itself (see generated_cluster.go's top doc
// comment and this task's constraints).
func newClusterRegistry(ctx context.Context, c *vmware.Client, opts RegistryOptions) *Registry {
	r := NewRegistry(ctx, c, opts)
	r.withClass(modeVCenterOnly, registerClusterTools)
	return r
}

// clusterToolNames is the exact set registered by registerClusterTools —
// kept here so TestClusterTools/Registration can't silently drift from the
// real registration list.
var clusterToolNames = []string{
	"vmware_cluster_recommend_hosts_for_vm",
	"vmware_cluster_retrieve_das_advanced_runtime_info",
	"vmware_cluster_get_resource_usage",
	"vmware_cluster_get_system_vms_restricted_datastores",
	"vmware_cluster_enter_maintenance_mode",
	"vmware_cluster_move_host_into",
	"vmware_cluster_refresh_recommendation",
	"vmware_cluster_apply_recommendation",
	"vmware_cluster_cancel_recommendation",
	"vmware_cluster_stamp_all_rules_with_uuid",
	"vmware_cluster_abandon_hci_workflow",
	"vmware_cluster_configure_hci",
	"vmware_cluster_extend_hci",
}

// clusterTier2ToolArgs supplies a minimal-but-valid (sans "confirm") argument
// set per Tier 2 tool registered by registerClusterTools — the 8 mutating
// tools out of the 13 (the other 5 are plain register/no-tier: recommend_
// hosts_for_vm, retrieve_das_advanced_runtime_info, get_resource_usage,
// get_system_vms_restricted_datastores, enter_maintenance_mode — all pure
// queries/dry-runs, see generated_cluster.go's top doc comment "Tier").
// "host-1" below is vcsim's well-known first-host moref value used
// elsewhere in this package's fixtures — irrelevant for GateAndConfirm since
// the gate/confirm check happens before the handler ever touches this value.
var clusterTier2ToolArgs = map[string]func(clusterPath, hostPath, vmPath string) map[string]interface{}{
	"vmware_cluster_move_host_into": func(cp, hp, vp string) map[string]interface{} {
		return map[string]interface{}{"cluster": cp, "host": hp}
	},
	"vmware_cluster_refresh_recommendation": func(cp, hp, vp string) map[string]interface{} {
		return map[string]interface{}{"cluster": cp}
	},
	"vmware_cluster_apply_recommendation": func(cp, hp, vp string) map[string]interface{} {
		return map[string]interface{}{"cluster": cp, "key": "rec-1"}
	},
	"vmware_cluster_cancel_recommendation": func(cp, hp, vp string) map[string]interface{} {
		return map[string]interface{}{"cluster": cp, "key": "rec-1"}
	},
	"vmware_cluster_stamp_all_rules_with_uuid": func(cp, hp, vp string) map[string]interface{} {
		return map[string]interface{}{"cluster": cp}
	},
	"vmware_cluster_abandon_hci_workflow": func(cp, hp, vp string) map[string]interface{} {
		return map[string]interface{}{"cluster": cp}
	},
	"vmware_cluster_configure_hci": func(cp, hp, vp string) map[string]interface{} {
		return map[string]interface{}{"cluster": cp, "cluster_spec": map[string]interface{}{}}
	},
	"vmware_cluster_extend_hci": func(cp, hp, vp string) map[string]interface{} {
		return map[string]interface{}{
			"cluster":     cp,
			"host_inputs": []interface{}{map[string]interface{}{"host": map[string]interface{}{"type": "HostSystem", "value": "host-1"}}},
		}
	},
}

func TestClusterTools(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	ctx := context.Background()
	seed := newClusterRegistry(ctx, c, RegistryOptions{AllowDestructive: true})
	clusterPath := firstClusterPath(t, seed)
	hostPath := firstHostPath(t, seed)
	vmPath := firstVMPath(t, seed)
	poolPath := firstResourcePoolPath(t, seed)

	t.Run("Registration", func(t *testing.T) {
		if len(clusterToolNames) != 13 {
			t.Fatalf("test bug: clusterToolNames has %d entries, expected 13", len(clusterToolNames))
		}
		got := map[string]bool{}
		for _, tl := range seed.ListTools() {
			got[tl.Name] = true
		}
		for _, name := range clusterToolNames {
			if !got[name] {
				t.Errorf("tool %s not registered", name)
			}
		}
	})

	// Validation proves each handler rejects a missing/empty required
	// argument. Cases that omit "cluster" entirely fail via
	// resolveClusterComputeResource's empty-name guard BEFORE any vcsim
	// round trip (same as generated_vm_ft_test.go's Validation cases for
	// "vm"). Cases that supply a valid "cluster" but omit a second required
	// argument (key/host/hosts/cluster_spec/host_inputs) let the cluster
	// resolve for real against vcsim first and then fail on the local
	// handler-side check — still proves the handler validates before ever
	// attempting the actual mutating/query SOAP call.
	t.Run("Validation", func(t *testing.T) {
		r := newClusterRegistry(ctx, c, RegistryOptions{AllowDestructive: true})

		cases := []struct {
			name string
			args map[string]interface{}
			why  string
		}{
			{"vmware_cluster_recommend_hosts_for_vm", map[string]interface{}{"cluster": clusterPath}, "missing vm"},
			{"vmware_cluster_recommend_hosts_for_vm", map[string]interface{}{"vm": vmPath}, "missing cluster"},
			{"vmware_cluster_retrieve_das_advanced_runtime_info", map[string]interface{}{}, "missing cluster"},
			{"vmware_cluster_get_resource_usage", map[string]interface{}{}, "missing cluster"},
			{"vmware_cluster_get_system_vms_restricted_datastores", map[string]interface{}{}, "missing cluster"},
			{"vmware_cluster_enter_maintenance_mode", map[string]interface{}{}, "missing cluster"},
			{"vmware_cluster_enter_maintenance_mode", map[string]interface{}{"cluster": clusterPath}, "missing hosts"},
			{"vmware_cluster_enter_maintenance_mode", map[string]interface{}{"cluster": clusterPath, "hosts": []interface{}{}}, "empty hosts"},
			{"vmware_cluster_enter_maintenance_mode", map[string]interface{}{"cluster": clusterPath, "hosts": []interface{}{"/DC0/host/no-such-host"}}, "unresolvable host"},
			{"vmware_cluster_move_host_into", map[string]interface{}{"cluster": clusterPath, "confirm": true}, "missing host"},
			{"vmware_cluster_move_host_into", map[string]interface{}{"host": hostPath, "confirm": true}, "missing cluster"},
			{"vmware_cluster_refresh_recommendation", map[string]interface{}{"confirm": true}, "missing cluster"},
			{"vmware_cluster_apply_recommendation", map[string]interface{}{"confirm": true}, "missing cluster"},
			{"vmware_cluster_apply_recommendation", map[string]interface{}{"cluster": clusterPath, "confirm": true}, "missing key"},
			{"vmware_cluster_cancel_recommendation", map[string]interface{}{"confirm": true}, "missing cluster"},
			{"vmware_cluster_cancel_recommendation", map[string]interface{}{"cluster": clusterPath, "confirm": true}, "missing key"},
			{"vmware_cluster_stamp_all_rules_with_uuid", map[string]interface{}{"confirm": true}, "missing cluster"},
			{"vmware_cluster_abandon_hci_workflow", map[string]interface{}{"confirm": true}, "missing cluster"},
			{"vmware_cluster_configure_hci", map[string]interface{}{"confirm": true}, "missing cluster"},
			{"vmware_cluster_configure_hci", map[string]interface{}{"cluster": clusterPath, "confirm": true}, "missing cluster_spec"},
			{"vmware_cluster_extend_hci", map[string]interface{}{"confirm": true}, "missing cluster"},
			{"vmware_cluster_extend_hci", map[string]interface{}{"cluster": clusterPath, "confirm": true}, "missing host_inputs"},
			{"vmware_cluster_extend_hci", map[string]interface{}{"cluster": clusterPath, "host_inputs": []interface{}{}, "confirm": true}, "empty host_inputs"},
		}

		for _, tc := range cases {
			t.Run(tc.name+"/"+tc.why, func(t *testing.T) {
				if _, err := r.CallTool(tc.name, tc.args); err == nil {
					t.Fatalf("expected an error (%s)", tc.why)
				}
			})
		}
	})

	// GateAndConfirm proves the Tier 2 destructive protection is wired on
	// every one of the 8 mutating tools: a closed --allow-destructive gate
	// denies the call, and an open gate still requires confirm:true — same
	// pair of checks generated_vm_ft_test.go's GateAndConfirm and
	// generated_cluster_modules_test.go's TierGating already exercise for
	// their own tier2/tier1 tools.
	t.Run("GateAndConfirm", func(t *testing.T) {
		closed := newClusterRegistry(ctx, c, RegistryOptions{})
		open := newClusterRegistry(ctx, c, RegistryOptions{AllowDestructive: true})

		for name, argsFn := range clusterTier2ToolArgs {
			name, argsFn := name, argsFn
			t.Run(name, func(t *testing.T) {
				base := argsFn(clusterPath, hostPath, vmPath)

				withConfirm := map[string]interface{}{}
				for k, v := range base {
					withConfirm[k] = v
				}
				withConfirm["confirm"] = true

				if _, err := closed.CallTool(name, withConfirm); err == nil {
					t.Fatalf("%s: expected the closed destructive gate to deny the call", name)
				}
				if _, err := open.CallTool(name, base); err == nil {
					t.Fatalf("%s: expected an error without confirm:true", name)
				}
			})
		}
	})

	// ReachesServer drives every tool with valid input (gate open, confirm:
	// true where applicable) and proves it reaches vcsim's real dispatch —
	// see this file's top doc comment for why every case is expected to
	// fault MethodNotFound rather than succeed.
	t.Run("ReachesServer", func(t *testing.T) {
		r := newClusterRegistry(ctx, c, RegistryOptions{AllowDestructive: true})

		cases := []struct {
			name string
			args map[string]interface{}
		}{
			{"vmware_cluster_recommend_hosts_for_vm", map[string]interface{}{"cluster": clusterPath, "vm": vmPath}},
			{"vmware_cluster_recommend_hosts_for_vm", map[string]interface{}{"cluster": clusterPath, "vm": vmPath, "pool": poolPath}},
			{"vmware_cluster_retrieve_das_advanced_runtime_info", map[string]interface{}{"cluster": clusterPath}},
			{"vmware_cluster_get_resource_usage", map[string]interface{}{"cluster": clusterPath}},
			{"vmware_cluster_get_system_vms_restricted_datastores", map[string]interface{}{"cluster": clusterPath}},
			{"vmware_cluster_enter_maintenance_mode", map[string]interface{}{"cluster": clusterPath, "hosts": []interface{}{hostPath}}},
			{"vmware_cluster_enter_maintenance_mode", map[string]interface{}{
				"cluster": clusterPath,
				"hosts":   []interface{}{hostPath},
				"option":  []interface{}{map[string]interface{}{"key": "opt", "value": "1"}},
				"info":    map[string]interface{}{"partialMMId": "noAction"},
			}},
			{"vmware_cluster_move_host_into", map[string]interface{}{"cluster": clusterPath, "host": hostPath, "confirm": true}},
			{"vmware_cluster_move_host_into", map[string]interface{}{"cluster": clusterPath, "host": hostPath, "resource_pool": poolPath, "confirm": true}},
			{"vmware_cluster_refresh_recommendation", map[string]interface{}{"cluster": clusterPath, "confirm": true}},
			{"vmware_cluster_apply_recommendation", map[string]interface{}{"cluster": clusterPath, "key": "rec-1", "confirm": true}},
			{"vmware_cluster_cancel_recommendation", map[string]interface{}{"cluster": clusterPath, "key": "rec-1", "confirm": true}},
			{"vmware_cluster_stamp_all_rules_with_uuid", map[string]interface{}{"cluster": clusterPath, "confirm": true}},
			{"vmware_cluster_abandon_hci_workflow", map[string]interface{}{"cluster": clusterPath, "confirm": true}},
			{"vmware_cluster_configure_hci", map[string]interface{}{"cluster": clusterPath, "cluster_spec": map[string]interface{}{}, "confirm": true}},
			{"vmware_cluster_configure_hci", map[string]interface{}{
				"cluster": clusterPath,
				"cluster_spec": map[string]interface{}{
					"vSanConfigSpec": map[string]interface{}{"dynamicType": "vim.vsan.ReconfigSpec"},
				},
				"host_inputs": []interface{}{map[string]interface{}{"host": map[string]interface{}{"type": "HostSystem", "value": "host-1"}}},
				"confirm":     true,
			}},
			{"vmware_cluster_extend_hci", map[string]interface{}{
				"cluster":     clusterPath,
				"host_inputs": []interface{}{map[string]interface{}{"host": map[string]interface{}{"type": "HostSystem", "value": "host-1"}}},
				"confirm":     true,
			}},
			{"vmware_cluster_extend_hci", map[string]interface{}{
				"cluster":          clusterPath,
				"host_inputs":      []interface{}{map[string]interface{}{"host": map[string]interface{}{"type": "HostSystem", "value": "host-1"}}},
				"vsan_config_spec": map[string]interface{}{"dynamicType": "vim.vsan.ReconfigSpec"},
				"confirm":          true,
			}},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				_, err := r.CallTool(tc.name, tc.args)
				assertReachesServer(t, err, tc.name)
			})
		}
	})
}

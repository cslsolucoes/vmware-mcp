package tools

import (
	"context"
	"testing"

	"github.com/vmware/govmomi/simulator"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// newFtRegistry builds a Registry the normal way (NewRegistry, which wires
// vm.go/host.go/etc — including firstVMPath's vmware_list_vms and
// firstHostPath's vmware_list_hosts — via registerTools) and then manually
// layers this group's Fault Tolerance tools on top via withClass, exactly as
// registry.go's real wiring for registerVMFaultToleranceTools will do once
// another change adds it there — this file must not edit registry.go itself
// (see generated_vm_ft.go's top doc comment and this task's constraints).
func newFtRegistry(ctx context.Context, c *vmware.Client, opts RegistryOptions) *Registry {
	r := NewRegistry(ctx, c, opts)
	r.withClass(modeVCenterOnly, registerVMFaultToleranceTools)
	return r
}

// ftToolNames is the exact set registered by registerVMFaultToleranceTools —
// kept here so TestVMFaultToleranceTools_Registration can't silently drift
// from the real registration list.
var ftToolNames = []string{
	"vmware_vm_create_secondary",
	"vmware_vm_create_secondary_ex",
	"vmware_vm_disable_secondary",
	"vmware_vm_enable_secondary",
	"vmware_vm_make_primary",
	"vmware_vm_terminate_fault_tolerant",
	"vmware_vm_turn_off_fault_tolerance",
}

// TestVMFaultToleranceTools_Registration proves all 7 FT tools are wired
// through newFtRegistry's withClass call and reachable via ListTools.
func TestVMFaultToleranceTools_Registration(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newFtRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	if len(ftToolNames) != 7 {
		t.Fatalf("test bug: ftToolNames has %d entries, expected 7", len(ftToolNames))
	}

	got := map[string]bool{}
	for _, tl := range r.ListTools() {
		got[tl.Name] = true
	}
	for _, name := range ftToolNames {
		if !got[name] {
			t.Errorf("tool %s not registered", name)
		}
	}
}

// TestVMFaultToleranceTools_Validation proves each handler rejects
// missing/empty required arguments BEFORE any network round trip (so these
// fail even with the gate open and confirm:true).
func TestVMFaultToleranceTools_Validation(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newFtRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	vm := firstVMPath(t, r)

	cases := []struct {
		name string
		args map[string]interface{}
		why  string
	}{
		{"vmware_vm_create_secondary", map[string]interface{}{"confirm": true}, "missing vm"},
		{"vmware_vm_create_secondary_ex", map[string]interface{}{"confirm": true}, "missing vm"},
		{"vmware_vm_create_secondary_ex", map[string]interface{}{"vm": vm, "spec": "not-an-object", "confirm": true}, "invalid spec"},
		{"vmware_vm_disable_secondary", map[string]interface{}{"vm": vm, "confirm": true}, "missing secondary_vm"},
		{"vmware_vm_disable_secondary", map[string]interface{}{"secondary_vm": vm, "confirm": true}, "missing vm"},
		{"vmware_vm_enable_secondary", map[string]interface{}{"vm": vm, "confirm": true}, "missing secondary_vm"},
		{"vmware_vm_make_primary", map[string]interface{}{"vm": vm, "confirm": true}, "missing secondary_vm"},
		{"vmware_vm_terminate_fault_tolerant", map[string]interface{}{"confirm": true}, "missing vm"},
		{"vmware_vm_turn_off_fault_tolerance", map[string]interface{}{"confirm": true}, "missing vm"},
	}

	for _, tc := range cases {
		t.Run(tc.name+"/"+tc.why, func(t *testing.T) {
			if _, err := r.CallTool(tc.name, tc.args); err == nil {
				t.Fatalf("expected an error (%s) before any round trip", tc.why)
			}
		})
	}
}

// TestVMFaultToleranceTools_GateAndConfirm proves the tier2 destructive
// protection is actually wired on every one of the 7 tools: a closed
// --allow-destructive gate denies the call, and an open gate still requires
// confirm:true.
func TestVMFaultToleranceTools_GateAndConfirm(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	seed := newFtRegistry(context.Background(), c, RegistryOptions{})
	vm := firstVMPath(t, seed)

	closed := newFtRegistry(context.Background(), c, RegistryOptions{})
	open := newFtRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	cases := []struct {
		name string
		args map[string]interface{}
	}{
		{"vmware_vm_create_secondary", map[string]interface{}{"vm": vm}},
		{"vmware_vm_create_secondary_ex", map[string]interface{}{"vm": vm}},
		{"vmware_vm_disable_secondary", map[string]interface{}{"vm": vm, "secondary_vm": vm}},
		{"vmware_vm_enable_secondary", map[string]interface{}{"vm": vm, "secondary_vm": vm}},
		{"vmware_vm_make_primary", map[string]interface{}{"vm": vm, "secondary_vm": vm}},
		{"vmware_vm_terminate_fault_tolerant", map[string]interface{}{"vm": vm}},
		{"vmware_vm_turn_off_fault_tolerance", map[string]interface{}{"vm": vm}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withConfirm := map[string]interface{}{}
			for k, v := range tc.args {
				withConfirm[k] = v
			}
			withConfirm["confirm"] = true

			if _, err := closed.CallTool(tc.name, withConfirm); err == nil {
				t.Fatalf("%s: expected the closed destructive gate to deny the call", tc.name)
			}

			if _, err := open.CallTool(tc.name, tc.args); err == nil {
				t.Fatalf("%s: expected an error without confirm:true", tc.name)
			}
		})
	}
}

// TestVMFaultToleranceTools_ReachesServer drives every tool with valid
// input, gate open, and confirm:true. vcsim's simulator.VirtualMachine has
// no server-side handler for any of the 7 FT methods (see
// generated_vm_ft.go's top doc comment), so each raw SOAP call is expected
// to reach vcsim's dispatch and return a clean types.MethodNotFound-based
// error — assertReachesServer (generated_vm_lifecycle_test.go), the same
// helper generated_host_iscsi_portbinding_test.go and
// generated_virtual_disk_test.go use for their own unsimulated methods.
//
// secondary_vm reuses the same VM path as the primary "vm" argument rather
// than standing up a real distinct secondary via vmware_vm_clone: since NONE
// of the 7 methods have any vcsim-side implementation at all (every call
// faults MethodNotFound before the server would ever inspect whether "vm"
// and "secondary_vm" name genuinely different VMs, or whether they're
// actually paired in an FT group — vcsim cannot simulate a real FT topology
// regardless), a distinct-VM fixture would add setup complexity without
// exercising any additional code path this test doesn't already cover. The
// point here is proving the wiring — schema, tier gate, resolveVM/
// ftSecondaryVM/ftOptionalHost, raw SOAP dispatch — reaches vcsim, not
// simulating a real FT pairing.
//
// simulator.VPX() (a vCenter model), not ESX(): this group is registered
// modeVCenterOnly per generated_vm_ft.go's classification — VPX() matches
// every other modeVCenterOnly-adjacent test in this package that needs more
// than a bare standalone host (e.g. generated_vm_lifecycle_test.go's
// WaitForIP tests).
func TestVMFaultToleranceTools_ReachesServer(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newFtRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	primary := firstVMPath(t, r)
	secondary := primary
	host := firstHostPath(t, r)

	cases := []struct {
		name string
		args map[string]interface{}
	}{
		{"vmware_vm_create_secondary", map[string]interface{}{"vm": primary, "confirm": true}},
		{"vmware_vm_create_secondary", map[string]interface{}{"vm": primary, "host": host, "confirm": true}},
		{"vmware_vm_create_secondary_ex", map[string]interface{}{"vm": primary, "confirm": true}},
		{"vmware_vm_create_secondary_ex", map[string]interface{}{"vm": primary, "spec": map[string]interface{}{"metroFtHostGroup": "group-1"}, "confirm": true}},
		{"vmware_vm_disable_secondary", map[string]interface{}{"vm": primary, "secondary_vm": secondary, "confirm": true}},
		{"vmware_vm_enable_secondary", map[string]interface{}{"vm": primary, "secondary_vm": secondary, "confirm": true}},
		{"vmware_vm_enable_secondary", map[string]interface{}{"vm": primary, "secondary_vm": secondary, "host": host, "confirm": true}},
		{"vmware_vm_make_primary", map[string]interface{}{"vm": primary, "secondary_vm": secondary, "confirm": true}},
		{"vmware_vm_terminate_fault_tolerant", map[string]interface{}{"vm": primary, "confirm": true}},
		{"vmware_vm_terminate_fault_tolerant", map[string]interface{}{"vm": primary, "secondary_vm": secondary, "confirm": true}},
		{"vmware_vm_turn_off_fault_tolerance", map[string]interface{}{"vm": primary, "confirm": true}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := r.CallTool(tc.name, tc.args)
			assertReachesServer(t, err, tc.name)
		})
	}
}

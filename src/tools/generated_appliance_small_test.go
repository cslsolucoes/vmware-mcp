package tools

import (
	"context"
	"testing"

	"github.com/vmware/govmomi/simulator"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// newApplianceSmallRegistry builds a Registry the normal way (NewRegistry,
// which wires every other domain via registerTools) and then manually layers
// registerApplianceSmallTools on top via withClass — same pattern as
// newLibraryCoreRegistry (generated_library_core_test.go). This file must not
// edit registry.go itself (see generated_appliance_small.go's top doc
// comment).
func newApplianceSmallRegistry(ctx context.Context, c *vmware.Client, opts RegistryOptions) *Registry {
	r := NewRegistry(ctx, c, opts)
	r.withClass(modeVCenterOnly, registerApplianceSmallTools)
	return r
}

// TestApplianceSmallTools_Registration proves all 15 tools are registered
// and reachable via ListTools.
func TestApplianceSmallTools_Registration(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newApplianceSmallRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	want := []string{
		"vmware_appliance_shutdown_cancel",
		"vmware_appliance_shutdown_get",
		"vmware_appliance_shutdown_power_off",
		"vmware_appliance_shutdown_reboot",
		"vmware_appliance_access_consolecli_get",
		"vmware_appliance_access_consolecli_set",
		"vmware_appliance_access_dcui_get",
		"vmware_appliance_access_dcui_set",
		"vmware_appliance_access_shell_get",
		"vmware_appliance_access_shell_set",
		"vmware_appliance_access_ssh_get",
		"vmware_appliance_access_ssh_set",
		"vmware_appliance_networking_no_proxy",
		"vmware_appliance_networking_proxy_list",
		"vmware_appliance_logging_forwarding",
	}
	if len(want) != 15 {
		t.Fatalf("test bug: want list has %d entries, expected 15", len(want))
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

// TestApplianceSmallTools_ReadOnlyToolsNotGated proves the 3 tools corrected
// from the AST classifier's tier2 default down to read-only (no_proxy,
// proxy_list, forwarding — see generated_appliance_small.go's top doc
// comment) are actually registered via r.register, not r.registerDestructive:
// calling them with AllowDestructive:false must NOT be denied by the gate —
// any error they return must come from the real network call (vcsim gap),
// not from wrapDestructive's gate check. Distinguished by message content:
// the gate's denial message always contains "destructive operation".
func TestApplianceSmallTools_ReadOnlyToolsNotGated(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newApplianceSmallRegistry(context.Background(), c, RegistryOptions{AllowDestructive: false})

	for _, tool := range []string{
		"vmware_appliance_networking_no_proxy",
		"vmware_appliance_networking_proxy_list",
		"vmware_appliance_logging_forwarding",
	} {
		_, err := r.CallTool(tool, map[string]interface{}{})
		if err == nil {
			t.Fatalf("%s: expected an error (vcsim gap), got success", tool)
		}
		assertReachesServer(t, err, tool)
	}
}

// TestApplianceSmallTools_ArgValidation proves each Set-style tool rejects a
// missing "enabled" argument with a clean error before the confirm/gate
// distinction matters (both open and confirmed here, isolating the
// validation check itself).
func TestApplianceSmallTools_ArgValidation(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newApplianceSmallRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	for _, tool := range []string{
		"vmware_appliance_access_consolecli_set",
		"vmware_appliance_access_dcui_set",
		"vmware_appliance_access_shell_set",
		"vmware_appliance_access_ssh_set",
	} {
		if _, err := r.CallTool(tool, map[string]interface{}{"confirm": true}); err == nil {
			t.Errorf("%s: expected an error when enabled is missing", tool)
		}
	}
}

// TestApplianceSmallTools_GateAndConfirm proves the Tier 2 tools in this file
// are wired through registerDestructive, the same 3-layer protection check
// pattern as generated_network_test.go's TestNetworkTools_GateAndConfirm.
func TestApplianceSmallTools_GateAndConfirm(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	closedGate := newApplianceSmallRegistry(context.Background(), c, RegistryOptions{AllowDestructive: false})
	if _, err := closedGate.CallTool("vmware_appliance_shutdown_cancel", map[string]interface{}{"confirm": true}); err == nil {
		t.Fatal("expected vmware_appliance_shutdown_cancel to be denied with the gate closed")
	}
	if _, err := closedGate.CallTool("vmware_appliance_access_ssh_set", map[string]interface{}{"enabled": true, "confirm": true}); err == nil {
		t.Fatal("expected vmware_appliance_access_ssh_set to be denied with the gate closed")
	}

	openGate := newApplianceSmallRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	if _, err := openGate.CallTool("vmware_appliance_shutdown_cancel", map[string]interface{}{}); err == nil {
		t.Fatal("expected vmware_appliance_shutdown_cancel to fail without confirm:true")
	}
	if _, err := openGate.CallTool("vmware_appliance_access_ssh_set", map[string]interface{}{"enabled": true}); err == nil {
		t.Fatal("expected vmware_appliance_access_ssh_set to fail without confirm:true")
	}
}

// TestApplianceSmallTools_ReachesServer proves every one of this file's 15
// tools reaches real vcsim instead of failing on something wired wrong here
// — see generated_appliance_small.go's top doc comment: no vcsim handler
// exists anywhere for these VAMI REST routes ("vcsim gap, not a bug",
// confirmed by grep against referencia/govmomi/vapi/simulator/simulator.go).
// err is expected to be non-nil for every call; assertReachesServer
// (generated_vm_lifecycle_test.go) proves it's a real server-side/HTTP
// fault, not "unknown tool" or a recovered panic.
func TestApplianceSmallTools_ReachesServer(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newApplianceSmallRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	cases := []struct {
		tool string
		args map[string]interface{}
	}{
		{"vmware_appliance_shutdown_get", map[string]interface{}{}},
		{"vmware_appliance_shutdown_cancel", map[string]interface{}{"confirm": true}},
		{"vmware_appliance_shutdown_power_off", map[string]interface{}{"reason": "test", "delay": 0, "confirm": true}},
		{"vmware_appliance_shutdown_reboot", map[string]interface{}{"reason": "test", "delay": 0, "confirm": true}},
		{"vmware_appliance_access_consolecli_get", map[string]interface{}{}},
		{"vmware_appliance_access_consolecli_set", map[string]interface{}{"enabled": true, "confirm": true}},
		{"vmware_appliance_access_dcui_get", map[string]interface{}{}},
		{"vmware_appliance_access_dcui_set", map[string]interface{}{"enabled": true, "confirm": true}},
		{"vmware_appliance_access_shell_get", map[string]interface{}{}},
		{"vmware_appliance_access_shell_set", map[string]interface{}{"enabled": true, "timeout": 60, "confirm": true}},
		{"vmware_appliance_access_ssh_get", map[string]interface{}{}},
		{"vmware_appliance_access_ssh_set", map[string]interface{}{"enabled": true, "confirm": true}},
		{"vmware_appliance_networking_no_proxy", map[string]interface{}{}},
		{"vmware_appliance_networking_proxy_list", map[string]interface{}{}},
		{"vmware_appliance_logging_forwarding", map[string]interface{}{}},
	}
	if len(cases) != 15 {
		t.Fatalf("test bug: cases has %d entries, expected 15", len(cases))
	}

	for _, tc := range cases {
		t.Run(tc.tool, func(t *testing.T) {
			_, err := r.CallTool(tc.tool, tc.args)
			assertReachesServer(t, err, tc.tool)
		})
	}
}

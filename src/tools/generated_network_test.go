package tools

import (
	"context"
	"testing"

	"github.com/vmware/govmomi/simulator"
)

// simulator.VPX()'s default model already creates 1 DistributedVirtualSwitch
// named "DVS0" per datacenter as a side effect of Model.Portgroup: 1 (a
// DVPortgroup needs a parent DVS to exist) — confirmed empirically (an
// earlier version of this file tried to create its own "DVS0" fixture via
// Folder.CreateDVS and got InvalidArgument{InvalidProperty:"name"}, a real
// name collision, not a flake) by reading simulator/model.go's Portgroup
// field doc comment and its model-generation code (m.fmtName("DVS", 0)).
// No fixture setup needed — every test below uses the pre-existing "DVS0".

func TestNetworkTools_Registration(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	// An isolated registry with ONLY this group's tools — NewRegistry alone
	// would already include every other domain's tools via registerTools(),
	// which would make an exact-count assertion meaningless. withClass on a
	// registry built with RegistryOptions{} still runs registerTools()
	// first; there is no "empty" NewRegistry constructor in this package,
	// so assert presence of the 7 names, not an exact total count.
	r := NewRegistry(context.Background(), c, RegistryOptions{})
	r.withClass(modeVCenterOnly, registerNetworkTools)
	r.withClass(modeVSphereGeneral, registerNetworkVSphereGeneralTools)

	names := map[string]bool{}
	for _, tool := range r.ListTools() {
		names[tool.Name] = true
	}
	want := []string{
		"vmware_dvpg_reconfigure",
		"vmware_dvs_add_portgroup",
		"vmware_dvs_fetch_dvports",
		"vmware_dvs_reconfigure",
		"vmware_dvs_reconfigure_dvport",
		"vmware_dvs_reconfigure_lacp",
		"vmware_opaque_network_summary",
	}
	for _, w := range want {
		if !names[w] {
			t.Errorf("expected tool %q to be registered", w)
		}
	}
}

func TestNetworkTools_DVSLifecycle(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := NewRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	r.withClass(modeVCenterOnly, registerNetworkTools)

	raw, err := r.CallTool("vmware_dvs_add_portgroup", map[string]interface{}{
		"network": "DVS0",
		"spec":    []interface{}{map[string]interface{}{"name": "pg-fixture", "numPorts": 8}},
		"confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_dvs_add_portgroup failed: %v (%s)", err, raw)
	}

	raw, err = r.CallTool("vmware_dvs_fetch_dvports", map[string]interface{}{"network": "DVS0"})
	if err != nil {
		t.Fatalf("vmware_dvs_fetch_dvports failed: %v", err)
	}
	m := decodeResult(t, raw)
	if count, _ := m["count"].(float64); count <= 0 {
		t.Fatalf("expected at least 1 port on the default DVS0, got %v: %s", m["count"], raw)
	}

	raw, err = r.CallTool("vmware_dvpg_reconfigure", map[string]interface{}{
		"network": "pg-fixture",
		"spec":    map[string]interface{}{"numPorts": 16},
		"confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_dvpg_reconfigure failed: %v (%s)", err, raw)
	}

	raw, err = r.CallTool("vmware_dvs_reconfigure", map[string]interface{}{
		"network": "DVS0",
		"spec":    map[string]interface{}{"configVersion": "", "description": "reconfigured by test"},
		"confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_dvs_reconfigure failed: %v (%s)", err, raw)
	}
}

func TestNetworkTools_GateAndConfirm(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	closedGate := NewRegistry(context.Background(), c, RegistryOptions{AllowDestructive: false})
	closedGate.withClass(modeVCenterOnly, registerNetworkTools)
	if _, err := closedGate.CallTool("vmware_dvs_reconfigure", map[string]interface{}{
		"network": "DVS0", "spec": map[string]interface{}{}, "confirm": true,
	}); err == nil {
		t.Fatal("expected vmware_dvs_reconfigure to be denied with the gate closed")
	}

	openGate := NewRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	openGate.withClass(modeVCenterOnly, registerNetworkTools)
	if _, err := openGate.CallTool("vmware_dvs_reconfigure", map[string]interface{}{
		"network": "DVS0", "spec": map[string]interface{}{},
	}); err == nil {
		t.Fatal("expected vmware_dvs_reconfigure to fail without confirm:true")
	}
}

func TestNetworkTools_WrongKindResolution(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := NewRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	r.withClass(modeVCenterOnly, registerNetworkTools)

	// "VM Network" is a plain Network in the default VPX model, not a DVS —
	// calling a DVS-only tool against it must fail cleanly, not panic.
	if _, err := r.CallTool("vmware_dvs_reconfigure", map[string]interface{}{
		"network": "VM Network",
		"spec":    map[string]interface{}{},
		"confirm": true,
	}); err == nil {
		t.Fatal("expected vmware_dvs_reconfigure against a plain Network to fail with a clear type error")
	}
}

func TestNetworkTools_UnsimulatedMethods(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := NewRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	r.withClass(modeVCenterOnly, registerNetworkTools)

	// ReconfigureDVPort and ReconfigureLACP have no vcsim handler — prove
	// the call reaches the server cleanly (not a wiring bug/panic) rather
	// than silently skipping them.
	if _, err := r.CallTool("vmware_dvs_reconfigure_dvport", map[string]interface{}{
		"network": "DVS0",
		"spec":    []interface{}{map[string]interface{}{"key": "0"}},
		"confirm": true,
	}); err == nil {
		t.Error("expected vmware_dvs_reconfigure_dvport to fail against vcsim (no simulator handler) — got success")
	}
	if _, err := r.CallTool("vmware_dvs_reconfigure_lacp", map[string]interface{}{
		"network": "DVS0",
		"spec":    []interface{}{map[string]interface{}{"name": "lag0"}},
		"confirm": true,
	}); err == nil {
		t.Error("expected vmware_dvs_reconfigure_lacp to fail against vcsim (no simulator handler) — got success")
	}
}

func TestNetworkTools_OpaqueNetworkSummaryRequiredArgs(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := NewRegistry(context.Background(), c, RegistryOptions{})
	r.withClass(modeVSphereGeneral, registerNetworkVSphereGeneralTools)

	if _, err := r.CallTool("vmware_opaque_network_summary", map[string]interface{}{}); err == nil {
		t.Fatal("expected vmware_opaque_network_summary to require the network argument")
	}
}

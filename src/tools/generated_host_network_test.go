package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/vmware/govmomi/simulator"
)

// TestHostNetworkTools_QueryNetworkHint proves vmware_host_network_query_network_hint
// resolves the host and reaches the real vcsim QueryNetworkHint handler (one
// of the 6 methods vcsim genuinely implements — see
// generated_host_network.go's top doc comment). A fresh vcsim host always
// reports an empty hint list: NewHostNetworkSystem never seeds
// types.QueryNetworkHintResponse and no public client call can (confirmed by
// reading simulator/host_network_system.go) — that empty-list shape is what
// gets verified here, both with and without an explicit device filter.
func TestHostNetworkTools_QueryNetworkHint(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := NewRegistry(context.Background(), c, RegistryOptions{})
	r.withClass(modeVSphereGeneral, registerHostNetworkTools)
	host := firstHostPath(t, r)

	t.Run("no_device_filter", func(t *testing.T) {
		raw, err := r.CallTool("vmware_host_network_query_network_hint", map[string]interface{}{"host": host})
		if err != nil {
			t.Fatalf("vmware_host_network_query_network_hint failed: %v", err)
		}
		m := decodeResult(t, raw)
		if got := m["count"]; got != float64(0) {
			t.Fatalf("expected count=0 on a fresh vcsim host, got %v (%s)", got, raw)
		}
		if _, ok := m["hints"].([]interface{}); !ok {
			t.Fatalf("expected a \"hints\" array in the result: %s", raw)
		}
	})

	t.Run("with_device_filter", func(t *testing.T) {
		raw, err := r.CallTool("vmware_host_network_query_network_hint", map[string]interface{}{"host": host, "device": []interface{}{"vmnic0"}})
		if err != nil {
			t.Fatalf("vmware_host_network_query_network_hint (with device filter) failed: %v", err)
		}
		if got := countOf(t, raw); got != 0 {
			t.Fatalf("expected count=0, got %v (%s)", got, raw)
		}
	})

	t.Run("invalid_device_type", func(t *testing.T) {
		if _, err := r.CallTool("vmware_host_network_query_network_hint", map[string]interface{}{"host": host, "device": "not-an-array"}); err == nil {
			t.Fatal("expected an error when device is not an array")
		}
	})
}

// TestHostNetworkTools_VirtualSwitchLifecycle round-trips
// vmware_host_network_add_virtual_switch / _remove_virtual_switch against
// real vcsim state (AddVirtualSwitch/RemoveVirtualSwitch are 2 of the 6
// genuinely simulated methods) — proves the add/remove calls actually
// mutate server-side state, not just that they return no error: a second
// add of the same name faults AlreadyExists, and removing a name that was
// never added (or already removed) faults NotFound. Also proves the Tier
// 2/Tier 1 gate is wired (mirrors host_test.go's
// TestHostTools_MaintenanceEnterGateAndConfirm pattern) for these two tools
// specifically — the generic gate/confirm MECHANISM itself is already
// covered exhaustively in destructive_test.go, so this is just proving THIS
// pair of tools was actually wrapped with it.
func TestHostNetworkTools_VirtualSwitchLifecycle(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := NewRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	r.withClass(modeVSphereGeneral, registerHostNetworkTools)
	host := firstHostPath(t, r)

	const swName = "TestSwitch1"

	if _, err := r.CallTool("vmware_host_network_add_virtual_switch", map[string]interface{}{"host": host, "vswitch_name": swName, "confirm": true}); err != nil {
		t.Fatalf("vmware_host_network_add_virtual_switch failed: %v", err)
	}

	t.Run("duplicate_add_faults", func(t *testing.T) {
		if _, err := r.CallTool("vmware_host_network_add_virtual_switch", map[string]interface{}{"host": host, "vswitch_name": swName, "confirm": true}); err == nil {
			t.Fatal("expected an AlreadyExists-style error adding the same vswitch name twice")
		}
	})

	t.Run("remove_nonexistent_faults", func(t *testing.T) {
		if _, err := r.CallTool("vmware_host_network_remove_virtual_switch", map[string]interface{}{"host": host, "vswitch_name": "NoSuchSwitch", "confirm": true}); err == nil {
			t.Fatal("expected a NotFound-style error removing a vswitch that was never added")
		}
	})

	t.Run("gate_and_confirm", func(t *testing.T) {
		closedGate := NewRegistry(context.Background(), c, RegistryOptions{AllowDestructive: false})
		closedGate.withClass(modeVSphereGeneral, registerHostNetworkTools)
		if _, err := closedGate.CallTool("vmware_host_network_add_virtual_switch", map[string]interface{}{"host": host, "vswitch_name": "GateTestSwitch", "confirm": true}); err == nil {
			t.Fatal("expected vmware_host_network_add_virtual_switch to be denied with the gate closed")
		}
		if _, err := closedGate.CallTool("vmware_host_network_remove_virtual_switch", map[string]interface{}{"host": host, "vswitch_name": swName, "confirm": true}); err == nil {
			t.Fatal("expected vmware_host_network_remove_virtual_switch to be denied with the gate closed")
		}
		// Prove the denied remove really did not run: swName must still be
		// removable (would fault NotFound if the denied call had somehow
		// gone through).
		if _, err := r.CallTool("vmware_host_network_add_virtual_switch", map[string]interface{}{"host": host, "vswitch_name": swName, "confirm": true}); err == nil {
			t.Fatal("re-adding swName should still fault AlreadyExists — the closed-gate remove call must not have run")
		}

		if _, err := r.CallTool("vmware_host_network_remove_virtual_switch", map[string]interface{}{"host": host, "vswitch_name": swName}); err == nil {
			t.Fatal("expected vmware_host_network_remove_virtual_switch to fail without confirm:true")
		}
	})

	if _, err := r.CallTool("vmware_host_network_remove_virtual_switch", map[string]interface{}{"host": host, "vswitch_name": swName, "confirm": true}); err != nil {
		t.Fatalf("vmware_host_network_remove_virtual_switch failed: %v", err)
	}

	t.Run("remove_again_faults", func(t *testing.T) {
		if _, err := r.CallTool("vmware_host_network_remove_virtual_switch", map[string]interface{}{"host": host, "vswitch_name": swName, "confirm": true}); err == nil {
			t.Fatal("expected a NotFound-style error removing an already-removed vswitch — proves the earlier remove genuinely mutated state")
		}
	})
}

// TestHostNetworkTools_PortGroupLifecycle round-trips
// vmware_host_network_add_port_group / _update_port_group /
// _remove_port_group against real vcsim state, on the default "vSwitch0"
// vcsim seeds for every host (confirmed in simulator/host_network_system.go's
// NewHostNetworkSystem). AddPortGroup/RemovePortGroup are 2 of the 6
// genuinely simulated methods; UpdatePortGroup is NOT (see this file's
// TestHostNetworkTools_UnsimulatedMethods) — it is exercised here only for
// its client-side required-arg validation, not a state change.
func TestHostNetworkTools_PortGroupLifecycle(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := NewRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	r.withClass(modeVSphereGeneral, registerHostNetworkTools)
	host := firstHostPath(t, r)

	const pgName = "TestPG1"
	portgrp := map[string]interface{}{"name": pgName, "vlanId": float64(0), "vswitchName": "vSwitch0"}

	if _, err := r.CallTool("vmware_host_network_add_port_group", map[string]interface{}{"host": host, "portgrp": portgrp, "confirm": true}); err != nil {
		t.Fatalf("vmware_host_network_add_port_group failed: %v", err)
	}

	t.Run("duplicate_name_faults", func(t *testing.T) {
		if _, err := r.CallTool("vmware_host_network_add_port_group", map[string]interface{}{"host": host, "portgrp": portgrp, "confirm": true}); err == nil {
			t.Fatal("expected a DuplicateName-style error adding the same port group name twice")
		}
	})

	t.Run("unknown_vswitch_faults", func(t *testing.T) {
		bad := map[string]interface{}{"name": "AnotherPG", "vlanId": float64(0), "vswitchName": "NoSuchVSwitch"}
		if _, err := r.CallTool("vmware_host_network_add_port_group", map[string]interface{}{"host": host, "portgrp": bad, "confirm": true}); err == nil {
			t.Fatal("expected a NotFound-style error adding a port group on a vswitch that does not exist")
		}
	})

	t.Run("empty_name_faults", func(t *testing.T) {
		bad := map[string]interface{}{"name": "", "vlanId": float64(0), "vswitchName": "vSwitch0"}
		if _, err := r.CallTool("vmware_host_network_add_port_group", map[string]interface{}{"host": host, "portgrp": bad, "confirm": true}); err == nil {
			t.Fatal("expected an InvalidArgument-style error adding a port group with an empty name")
		}
	})

	t.Run("update_requires_args", func(t *testing.T) {
		if _, err := r.CallTool("vmware_host_network_update_port_group", map[string]interface{}{"host": host, "portgrp": portgrp, "confirm": true}); err == nil {
			t.Fatal("expected an error when pg_name is missing")
		}
		if _, err := r.CallTool("vmware_host_network_update_port_group", map[string]interface{}{"host": host, "pg_name": pgName, "confirm": true}); err == nil {
			t.Fatal("expected an error when portgrp is missing")
		}
	})

	if _, err := r.CallTool("vmware_host_network_remove_port_group", map[string]interface{}{"host": host, "pg_name": pgName, "confirm": true}); err != nil {
		t.Fatalf("vmware_host_network_remove_port_group failed: %v", err)
	}

	t.Run("remove_again_faults", func(t *testing.T) {
		if _, err := r.CallTool("vmware_host_network_remove_port_group", map[string]interface{}{"host": host, "pg_name": pgName, "confirm": true}); err == nil {
			t.Fatal("expected a NotFound-style error removing an already-removed port group")
		}
	})

	t.Run("re_add_after_remove_succeeds", func(t *testing.T) {
		// Proves the earlier remove genuinely cleared pgName from the
		// inventory folder (AddPortGroup's DuplicateName check is
		// folder-based, not NetworkInfo.Portgroup-slice-based — confirmed by
		// reading simulator/host_network_system.go) — a re-add would fault
		// DuplicateName otherwise.
		if _, err := r.CallTool("vmware_host_network_add_port_group", map[string]interface{}{"host": host, "portgrp": portgrp, "confirm": true}); err != nil {
			t.Fatalf("re-adding %q after removal failed: %v", pgName, err)
		}
	})

	t.Run("gate_and_confirm", func(t *testing.T) {
		closedGate := NewRegistry(context.Background(), c, RegistryOptions{AllowDestructive: false})
		closedGate.withClass(modeVSphereGeneral, registerHostNetworkTools)
		other := map[string]interface{}{"name": "GatedPG", "vlanId": float64(0), "vswitchName": "vSwitch0"}
		if _, err := closedGate.CallTool("vmware_host_network_add_port_group", map[string]interface{}{"host": host, "portgrp": other, "confirm": true}); err == nil {
			t.Fatal("expected vmware_host_network_add_port_group to be denied with the gate closed")
		}
		if _, err := r.CallTool("vmware_host_network_remove_port_group", map[string]interface{}{"host": host, "pg_name": "GatedPG", "confirm": true}); err == nil {
			t.Fatal("removing GatedPG should fault NotFound — the closed-gate add call must not have run")
		}
	})
}

// TestHostNetworkTools_UpdateNetworkConfig exercises
// vmware_host_network_update_network_config end to end against vcsim's
// genuinely simulated UpdateNetworkConfig handler (stores the config
// verbatim and returns an empty types.HostNetworkConfigResult{} — confirmed
// by reading simulator/host_network_system.go), including the
// dns_config/ip_route_config/console_ip_route_config split-out described in
// generated_host_network.go's top doc comment, and proves the one documented
// limitation (nesting dnsConfig/etc directly inside "config") fails loudly
// instead of silently dropping the field.
func TestHostNetworkTools_UpdateNetworkConfig(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := NewRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	r.withClass(modeVSphereGeneral, registerHostNetworkTools)
	host := firstHostPath(t, r)

	t.Run("invalid_change_mode_rejected_client_side", func(t *testing.T) {
		if _, err := r.CallTool("vmware_host_network_update_network_config", map[string]interface{}{"host": host, "change_mode": "bogus", "confirm": true}); err == nil {
			t.Fatal("expected an error for an invalid change_mode")
		}
		if _, err := r.CallTool("vmware_host_network_update_network_config", map[string]interface{}{"host": host, "confirm": true}); err == nil {
			t.Fatal("expected an error when change_mode is missing")
		}
	})

	t.Run("nested_polymorphic_field_fails_loudly", func(t *testing.T) {
		config := map[string]interface{}{"dnsConfig": map[string]interface{}{"dhcp": true, "hostName": "esx01"}}
		_, err := r.CallTool("vmware_host_network_update_network_config", map[string]interface{}{
			"host": host, "config": config, "change_mode": "modify", "confirm": true,
		})
		if err == nil {
			t.Fatal("expected nesting dnsConfig inside config to fail (polymorphic interface field — see generated_host_network.go's top doc comment), got success")
		}
		if !strings.Contains(err.Error(), "dns_config") {
			t.Fatalf("expected the error to point the caller at the dns_config argument, got: %v", err)
		}
	})

	t.Run("success_with_split_out_polymorphic_args", func(t *testing.T) {
		raw, err := r.CallTool("vmware_host_network_update_network_config", map[string]interface{}{
			"host":                    host,
			"config":                  map[string]interface{}{"ipV6Enabled": false},
			"dns_config":              map[string]interface{}{"dhcp": false, "hostName": "esx01"},
			"ip_route_config":         map[string]interface{}{"defaultGateway": "192.168.1.1"},
			"console_ip_route_config": map[string]interface{}{"defaultGateway": "192.168.1.1"},
			"change_mode":             "modify",
			"confirm":                 true,
		})
		if err != nil {
			t.Fatalf("vmware_host_network_update_network_config failed: %v", err)
		}
		m := decodeResult(t, raw)
		if m["change_mode"] != "modify" {
			t.Fatalf("expected change_mode=modify echoed back, got %v (%s)", m["change_mode"], raw)
		}
		if _, ok := m["result"]; !ok {
			t.Fatalf("expected a \"result\" field: %s", raw)
		}
	})

	t.Run("gate_and_confirm", func(t *testing.T) {
		closedGate := NewRegistry(context.Background(), c, RegistryOptions{AllowDestructive: false})
		closedGate.withClass(modeVSphereGeneral, registerHostNetworkTools)
		if _, err := closedGate.CallTool("vmware_host_network_update_network_config", map[string]interface{}{"host": host, "change_mode": "modify", "confirm": true}); err == nil {
			t.Fatal("expected vmware_host_network_update_network_config to be denied with the gate closed")
		}
	})
}

// TestHostNetworkTools_UnsimulatedMethods proves the 15 methods with no
// server-side implementation in simulator.HostNetworkSystem (confirmed by
// reading referencia/govmomi/simulator/host_network_system.go — see
// generated_host_network.go's top doc comment) are correctly registered,
// resolve the host, validate their own required args client-side, and
// genuinely reach vcsim's method dispatch (getting back a clean
// types.MethodNotFound-based error via assertReachesServer, reused from
// generated_vm_lifecycle_test.go — same package), not a wiring failure or a
// panic.
func TestHostNetworkTools_UnsimulatedMethods(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := NewRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	r.withClass(modeVSphereGeneral, registerHostNetworkTools)
	host := firstHostPath(t, r)

	nicSpec := map[string]interface{}{"ip": map[string]interface{}{"dhcp": true}, "portgroup": "VM Network"}
	pgSpec := map[string]interface{}{"name": "SomePG", "vlanId": float64(0), "vswitchName": "vSwitch0"}
	switchSpec := map[string]interface{}{"numPorts": float64(128)}

	cases := []struct {
		name string
		tool string
		args map[string]interface{}
	}{
		{"add_service_console_virtual_nic", "vmware_host_network_add_service_console_virtual_nic", map[string]interface{}{"portgroup": "Service Console", "nic": nicSpec}},
		{"add_virtual_nic", "vmware_host_network_add_virtual_nic", map[string]interface{}{"portgroup": "VM Network", "nic": nicSpec}},
		{"refresh", "vmware_host_network_refresh", map[string]interface{}{}},
		{"restart_service_console_virtual_nic", "vmware_host_network_restart_service_console_virtual_nic", map[string]interface{}{"device": "vswif0"}},
		{"update_console_ip_route_config", "vmware_host_network_update_console_ip_route_config", map[string]interface{}{"config": map[string]interface{}{"defaultGateway": "192.168.1.1"}}},
		{"update_dns_config", "vmware_host_network_update_dns_config", map[string]interface{}{"config": map[string]interface{}{"dhcp": false, "hostName": "esx01"}}},
		{"update_ip_route_config", "vmware_host_network_update_ip_route_config", map[string]interface{}{"config": map[string]interface{}{"defaultGateway": "192.168.1.1"}}},
		{"update_ip_route_table_config", "vmware_host_network_update_ip_route_table_config", map[string]interface{}{"config": map[string]interface{}{}}},
		{"update_physical_nic_link_speed", "vmware_host_network_update_physical_nic_link_speed", map[string]interface{}{"device": "vmnic0", "link_speed": map[string]interface{}{"speedMb": float64(1000), "duplex": true}}},
		{"update_port_group", "vmware_host_network_update_port_group", map[string]interface{}{"pg_name": "VM Network", "portgrp": pgSpec}},
		{"update_service_console_virtual_nic", "vmware_host_network_update_service_console_virtual_nic", map[string]interface{}{"device": "vswif0", "nic": nicSpec}},
		{"update_virtual_nic", "vmware_host_network_update_virtual_nic", map[string]interface{}{"device": "vmk0", "nic": nicSpec}},
		{"update_virtual_switch", "vmware_host_network_update_virtual_switch", map[string]interface{}{"vswitch_name": "vSwitch0", "spec": switchSpec}},
		{"remove_service_console_virtual_nic", "vmware_host_network_remove_service_console_virtual_nic", map[string]interface{}{"device": "vswif0"}},
		{"remove_virtual_nic", "vmware_host_network_remove_virtual_nic", map[string]interface{}{"device": "vmk0"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			args := map[string]interface{}{"host": host, "confirm": true}
			for k, v := range tc.args {
				args[k] = v
			}
			_, err := r.CallTool(tc.tool, args)
			assertReachesServer(t, err, tc.tool)
		})
	}

	// Spot-check required-arg validation happens BEFORE any vcsim round
	// trip (a handful of representative cases, not all 15 — the
	// per-argument pattern is identical across every handler in this file).
	t.Run("required_arg_validation", func(t *testing.T) {
		if _, err := r.CallTool("vmware_host_network_add_virtual_nic", map[string]interface{}{"host": host, "nic": nicSpec, "confirm": true}); err == nil {
			t.Fatal("expected an error when portgroup is missing")
		}
		if _, err := r.CallTool("vmware_host_network_add_virtual_nic", map[string]interface{}{"host": host, "portgroup": "VM Network", "confirm": true}); err == nil {
			t.Fatal("expected an error when nic is missing")
		}
		if _, err := r.CallTool("vmware_host_network_restart_service_console_virtual_nic", map[string]interface{}{"host": host, "confirm": true}); err == nil {
			t.Fatal("expected an error when device is missing")
		}
		if _, err := r.CallTool("vmware_host_network_update_dns_config", map[string]interface{}{"host": host, "confirm": true}); err == nil {
			t.Fatal("expected an error when config is missing")
		}
		if _, err := r.CallTool("vmware_host_network_update_virtual_switch", map[string]interface{}{"host": host, "vswitch_name": "vSwitch0", "confirm": true}); err == nil {
			t.Fatal("expected an error when spec is missing")
		}
		if _, err := r.CallTool("vmware_host_network_remove_virtual_nic", map[string]interface{}{"host": host, "confirm": true}); err == nil {
			t.Fatal("expected an error when device is missing")
		}
	})

	t.Run("gate_and_confirm_spot_check", func(t *testing.T) {
		closedGate := NewRegistry(context.Background(), c, RegistryOptions{AllowDestructive: false})
		closedGate.withClass(modeVSphereGeneral, registerHostNetworkTools)
		// Tier 2 example.
		if _, err := closedGate.CallTool("vmware_host_network_refresh", map[string]interface{}{"host": host, "confirm": true}); err == nil {
			t.Fatal("expected vmware_host_network_refresh to be denied with the gate closed")
		}
		// Tier 1 example.
		if _, err := closedGate.CallTool("vmware_host_network_remove_virtual_nic", map[string]interface{}{"host": host, "device": "vmk0", "confirm": true}); err == nil {
			t.Fatal("expected vmware_host_network_remove_virtual_nic to be denied with the gate closed")
		}
		// Confirm still required with an open gate.
		if _, err := r.CallTool("vmware_host_network_refresh", map[string]interface{}{"host": host}); err == nil {
			t.Fatal("expected vmware_host_network_refresh to fail without confirm:true")
		}
	})
}

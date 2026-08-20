package tools

import (
	"context"
	"testing"

	"github.com/vmware/govmomi/simulator"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// newHostSystemExtRegistry builds a Registry the normal way (NewRegistry,
// which wires host.go/generated_host_misc.go/etc via registerTools) and
// then manually layers this group's tools on top via withClass, exactly as
// registry.go's real wiring for registerHostSystemExtTools will do once
// another change adds it there — this file must not edit registry.go itself
// (see generated_host_system_ext.go's top doc comment and this task's
// constraints).
func newHostSystemExtRegistry(ctx context.Context, c *vmware.Client, opts RegistryOptions) *Registry {
	r := NewRegistry(ctx, c, opts)
	r.withClass(modeVSphereGeneral, registerHostSystemExtTools)
	return r
}

// hostSystemExtToolNames is the exact set registered by
// registerHostSystemExtTools — kept here so TestHostSystemExt's Registration
// subtest can't silently drift from the real registration list.
var hostSystemExtToolNames = []string{
	"vmware_host_reboot",
	"vmware_host_shutdown",
	"vmware_host_power_down_to_standby",
	"vmware_host_power_up_from_standby",
	"vmware_host_lockdown_enter",
	"vmware_host_lockdown_exit",
	"vmware_host_acquire_cim_services_ticket",
	"vmware_host_update_flags",
	"vmware_host_update_system_resources",
	"vmware_host_update_system_swap_configuration",
	"vmware_host_update_ipmi",
	"vmware_host_retrieve_hardware_uptime",
	"vmware_host_query_connection_info",
}

// hostSystemExtDestructiveToolNames is the subset of hostSystemExtToolNames
// registered via registerDestructive (tier1/tier2) — every tool except the
// 3 plain reads (acquire_cim_services_ticket, retrieve_hardware_uptime,
// query_connection_info).
var hostSystemExtDestructiveToolNames = []string{
	"vmware_host_reboot",
	"vmware_host_shutdown",
	"vmware_host_power_down_to_standby",
	"vmware_host_power_up_from_standby",
	"vmware_host_lockdown_enter",
	"vmware_host_lockdown_exit",
	"vmware_host_update_flags",
	"vmware_host_update_system_resources",
	"vmware_host_update_system_swap_configuration",
	"vmware_host_update_ipmi",
}

// TestHostSystemExt drives every check for this group (registration,
// argument validation, the destructive gate/confirm layer, and reaching a
// real vcsim server) against a SINGLE simulator.ESX() instance, spun up
// once at the top of this function and reused by every t.Run subtest below
// — per this task's constraint to avoid multiple newSimClient calls in one
// test file. simulator.ESX() (a standalone host, not VPX()) matches this
// group's modeVSphereGeneral classification (see
// generated_host_system_ext.go's top doc comment) and every other
// modeVSphereGeneral host-domain test file in this package (host_test.go,
// generated_host_misc_test.go).
func TestHostSystemExt(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	t.Run("Registration", func(t *testing.T) {
		r := newHostSystemExtRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

		if len(hostSystemExtToolNames) != 13 {
			t.Fatalf("test bug: hostSystemExtToolNames has %d entries, expected 13", len(hostSystemExtToolNames))
		}

		got := map[string]bool{}
		for _, tl := range r.ListTools() {
			got[tl.Name] = true
		}
		for _, name := range hostSystemExtToolNames {
			if !got[name] {
				t.Errorf("tool %s not registered", name)
			}
		}
	})

	t.Run("Validation", func(t *testing.T) {
		r := newHostSystemExtRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
		host := firstHostPath(t, r)

		cases := []struct {
			name string
			args map[string]interface{}
			why  string
		}{
			{"vmware_host_reboot", map[string]interface{}{"confirm": true}, "missing host"},
			{"vmware_host_shutdown", map[string]interface{}{"confirm": true}, "missing host"},
			{"vmware_host_power_down_to_standby", map[string]interface{}{"confirm": true}, "missing host"},
			{"vmware_host_power_up_from_standby", map[string]interface{}{"confirm": true}, "missing host"},
			{"vmware_host_lockdown_enter", map[string]interface{}{"confirm": true}, "missing host"},
			{"vmware_host_lockdown_exit", map[string]interface{}{"confirm": true}, "missing host"},
			{"vmware_host_acquire_cim_services_ticket", map[string]interface{}{}, "missing host"},
			{"vmware_host_update_flags", map[string]interface{}{"confirm": true}, "missing host"},
			{"vmware_host_update_flags", map[string]interface{}{"host": host, "confirm": true}, "missing flag_info"},
			{"vmware_host_update_flags", map[string]interface{}{"host": host, "flag_info": "not-an-object", "confirm": true}, "invalid flag_info"},
			{"vmware_host_update_system_resources", map[string]interface{}{"confirm": true}, "missing host"},
			{"vmware_host_update_system_resources", map[string]interface{}{"host": host, "confirm": true}, "missing resource_info"},
			{"vmware_host_update_system_resources", map[string]interface{}{"host": host, "resource_info": "not-an-object", "confirm": true}, "invalid resource_info"},
			{"vmware_host_update_system_swap_configuration", map[string]interface{}{"confirm": true}, "missing host"},
			{"vmware_host_update_system_swap_configuration", map[string]interface{}{"host": host, "confirm": true}, "missing sys_swap_config"},
			{"vmware_host_update_system_swap_configuration", map[string]interface{}{"host": host, "sys_swap_config": "not-an-object", "confirm": true}, "invalid sys_swap_config"},
			{"vmware_host_update_ipmi", map[string]interface{}{"confirm": true}, "missing host"},
			{"vmware_host_update_ipmi", map[string]interface{}{"host": host, "confirm": true}, "missing ipmi_info"},
			{"vmware_host_update_ipmi", map[string]interface{}{"host": host, "ipmi_info": "not-an-object", "confirm": true}, "invalid ipmi_info"},
			{"vmware_host_retrieve_hardware_uptime", map[string]interface{}{}, "missing host"},
			{"vmware_host_query_connection_info", map[string]interface{}{}, "missing host"},
		}

		for _, tc := range cases {
			t.Run(tc.name+"/"+tc.why, func(t *testing.T) {
				if _, err := r.CallTool(tc.name, tc.args); err == nil {
					t.Fatalf("expected an error (%s) before any round trip", tc.why)
				}
			})
		}
	})

	t.Run("GateAndConfirm", func(t *testing.T) {
		seed := newHostSystemExtRegistry(context.Background(), c, RegistryOptions{})
		host := firstHostPath(t, seed)

		closed := newHostSystemExtRegistry(context.Background(), c, RegistryOptions{})
		open := newHostSystemExtRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

		cases := []struct {
			name string
			args map[string]interface{}
		}{
			{"vmware_host_reboot", map[string]interface{}{"host": host}},
			{"vmware_host_shutdown", map[string]interface{}{"host": host}},
			{"vmware_host_power_down_to_standby", map[string]interface{}{"host": host}},
			{"vmware_host_power_up_from_standby", map[string]interface{}{"host": host}},
			{"vmware_host_lockdown_enter", map[string]interface{}{"host": host}},
			{"vmware_host_lockdown_exit", map[string]interface{}{"host": host}},
			{"vmware_host_update_flags", map[string]interface{}{"host": host, "flag_info": map[string]interface{}{"backgroundSnapshotsEnabled": true}}},
			{"vmware_host_update_system_resources", map[string]interface{}{"host": host, "resource_info": map[string]interface{}{"key": "host"}}},
			{"vmware_host_update_system_swap_configuration", map[string]interface{}{"host": host, "sys_swap_config": map[string]interface{}{}}},
			{"vmware_host_update_ipmi", map[string]interface{}{"host": host, "ipmi_info": map[string]interface{}{"bmcIpAddress": "10.0.0.5"}}},
		}

		if len(cases) != len(hostSystemExtDestructiveToolNames) {
			t.Fatalf("test bug: %d gate cases but %d destructive tool names", len(cases), len(hostSystemExtDestructiveToolNames))
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
	})

	t.Run("ReachesServer", func(t *testing.T) {
		// simulator.HostSystem has no server-side handler for any of the 13
		// methods this group wraps (see generated_host_system_ext.go's top
		// doc comment — the whole referencia/govmomi/simulator tree was
		// grepped for every one of the 13 exact method names with zero
		// matches), so every call here is expected to reach vcsim's generic
		// dispatcher and fault, proving the wiring (schema, tier gate,
		// resolveHost, raw SOAP dispatch) rather than simulating real
		// behavior — assertReachesServer (generated_vm_lifecycle_test.go),
		// reused here exactly as generated_vm_ft_test.go and
		// generated_host_misc_test.go already do for their own unsimulated
		// methods.
		r := newHostSystemExtRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
		host := firstHostPath(t, r)

		cases := []struct {
			name string
			args map[string]interface{}
		}{
			{"vmware_host_reboot", map[string]interface{}{"host": host, "confirm": true}},
			{"vmware_host_reboot", map[string]interface{}{"host": host, "force": true, "confirm": true}},
			{"vmware_host_shutdown", map[string]interface{}{"host": host, "confirm": true}},
			{"vmware_host_power_down_to_standby", map[string]interface{}{"host": host, "confirm": true}},
			{"vmware_host_power_down_to_standby", map[string]interface{}{"host": host, "timeout_sec": 30, "evacuate_powered_off_vms": true, "confirm": true}},
			{"vmware_host_power_up_from_standby", map[string]interface{}{"host": host, "confirm": true}},
			{"vmware_host_power_up_from_standby", map[string]interface{}{"host": host, "timeout_sec": 30, "confirm": true}},
			{"vmware_host_lockdown_enter", map[string]interface{}{"host": host, "confirm": true}},
			{"vmware_host_lockdown_exit", map[string]interface{}{"host": host, "confirm": true}},
			{"vmware_host_acquire_cim_services_ticket", map[string]interface{}{"host": host}},
			{"vmware_host_update_flags", map[string]interface{}{"host": host, "flag_info": map[string]interface{}{"backgroundSnapshotsEnabled": true}, "confirm": true}},
			{"vmware_host_update_system_resources", map[string]interface{}{"host": host, "resource_info": map[string]interface{}{"key": "host"}, "confirm": true}},
			{"vmware_host_update_system_swap_configuration", map[string]interface{}{"host": host, "sys_swap_config": map[string]interface{}{}, "confirm": true}},
			{"vmware_host_update_ipmi", map[string]interface{}{"host": host, "ipmi_info": map[string]interface{}{"bmcIpAddress": "10.0.0.5", "login": "admin"}, "confirm": true}},
			{"vmware_host_retrieve_hardware_uptime", map[string]interface{}{"host": host}},
			{"vmware_host_query_connection_info", map[string]interface{}{"host": host}},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				_, err := r.CallTool(tc.name, tc.args)
				assertReachesServer(t, err, tc.name)
			})
		}
	})
}

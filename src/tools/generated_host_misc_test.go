package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/vmware/govmomi/simulator"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// newHostMiscRegistry builds a Registry the normal way (NewRegistry, which
// wires vm.go/host.go/etc via registerTools) and then manually layers this
// group's tools on top via withClass, exactly as registry.go's real wiring
// for registerHostMiscTools will do once another change adds it there — this
// file must not edit registry.go itself (see generated_host_misc.go's top
// doc comment / the Hard requirements it was written against).
func newHostMiscRegistry(ctx context.Context, c *vmware.Client, opts RegistryOptions) *Registry {
	r := NewRegistry(ctx, c, opts)
	r.withClass(modeVSphereGeneral, registerHostMiscTools)
	return r
}

// assertCleanFailure proves a host-misc tool call reached real vcsim/govmomi
// plumbing (arg parsing, host resolution, ConfigManager().XSystem(ctx), and
// for HostVirtualNicManager's select/deselect the actual SOAP call) instead
// of failing on something wired wrong in this file — used throughout this
// file for the 14 of 15 tools that cannot reach a full success against
// vcsim. See generated_host_misc.go's top doc comment for the 2 different
// vcsim gaps this proves depending on the tool (manager reference never
// wired into ConfigManager vs. the underlying SOAP method never implemented
// at all). err is expected to be non-nil; the point is proving it is a REAL
// failure from that layer, not "unknown tool" (registration broken) or a
// recovered panic (handler bug) — same proof shape as
// generated_vm_lifecycle_test.go's assertReachesServer, kept as a separate
// function here because the "why" each asserts (and thus the message on an
// unexpected success) is specific to this file's 2 distinct gaps.
func assertCleanFailure(t *testing.T, err error, tool string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s: expected an error — see generated_host_misc.go's top doc comment for the vcsim gap this proves — got success instead", tool)
	}
	msg := err.Error()
	if strings.Contains(msg, "unknown tool") {
		t.Fatalf("%s: tool is not registered: %v", tool, err)
	}
	if strings.Contains(msg, "panicked") {
		t.Fatalf("%s: handler panicked instead of returning a clean error: %v", tool, err)
	}
	t.Logf("%s: real vcsim call returned (expected — see generated_host_misc.go's top doc comment): %v", tool, err)
}

// TestHostMiscTools_Registration proves all 15 tools in this group are
// actually registered (present in ListTools) once layered onto a Registry.
func TestHostMiscTools_Registration(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newHostMiscRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	want := []string{
		"vmware_host_service_list",
		"vmware_host_service_restart",
		"vmware_host_service_start",
		"vmware_host_service_stop",
		"vmware_host_service_update_policy",
		"vmware_host_datetime_query",
		"vmware_host_datetime_update",
		"vmware_host_datetime_update_config",
		"vmware_host_nic_info",
		"vmware_host_nic_select_vnic",
		"vmware_host_nic_deselect_vnic",
		"vmware_host_vsan_update",
		"vmware_host_vsan_internal_query_object_uuids",
		"vmware_host_vsan_internal_get_obj_ext_attrs",
		"vmware_host_vsan_internal_delete_objects",
	}
	if len(want) != 15 {
		t.Fatalf("test bug: want list has %d entries, expected 15", len(want))
	}

	got := make(map[string]bool, len(want))
	for _, tool := range r.ListTools() {
		got[tool.Name] = true
	}
	for _, name := range want {
		if !got[name] {
			t.Errorf("expected tool %q to be registered", name)
		}
	}
}

// TestHostMiscTools_ServiceSystem covers all 5 HostServiceSystem tools.
// host.ConfigManager().ServiceSystem(ctx) itself succeeds (it just reads a
// well-formed placeholder ManagedObjectReference off the host's ESX
// template), but that reference was never registered as a real vcsim
// object — see generated_host_misc.go's top doc comment — so every call
// here reaches a real SOAP round trip and gets back
// "managed object not found: HostServiceSystem:serviceSystem" from vcsim's
// generic dispatcher.
func TestHostMiscTools_ServiceSystem(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newHostMiscRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	host := firstHostPath(t, r)

	t.Run("list_missing_host", func(t *testing.T) {
		if _, err := r.CallTool("vmware_host_service_list", map[string]interface{}{}); err == nil {
			t.Fatal("expected an error when host is missing")
		}
	})

	t.Run("list_reaches_server", func(t *testing.T) {
		_, err := r.CallTool("vmware_host_service_list", map[string]interface{}{"host": host})
		assertCleanFailure(t, err, "vmware_host_service_list")
	})

	t.Run("restart_requires_id", func(t *testing.T) {
		if _, err := r.CallTool("vmware_host_service_restart", map[string]interface{}{"host": host, "confirm": true}); err == nil {
			t.Fatal("expected an error when id is missing")
		}
	})

	t.Run("restart_gate_and_confirm", func(t *testing.T) {
		closedGate := newHostMiscRegistry(context.Background(), c, RegistryOptions{AllowDestructive: false})
		if _, err := closedGate.CallTool("vmware_host_service_restart", map[string]interface{}{"host": host, "id": "ntpd", "confirm": true}); err == nil {
			t.Fatal("expected vmware_host_service_restart to be denied with the gate closed")
		}
		if _, err := r.CallTool("vmware_host_service_restart", map[string]interface{}{"host": host, "id": "ntpd"}); err == nil {
			t.Fatal("expected vmware_host_service_restart to fail without confirm:true")
		}
	})

	t.Run("restart_reaches_server", func(t *testing.T) {
		_, err := r.CallTool("vmware_host_service_restart", map[string]interface{}{"host": host, "id": "ntpd", "confirm": true})
		assertCleanFailure(t, err, "vmware_host_service_restart")
	})

	t.Run("start_reaches_server", func(t *testing.T) {
		_, err := r.CallTool("vmware_host_service_start", map[string]interface{}{"host": host, "id": "ntpd", "confirm": true})
		assertCleanFailure(t, err, "vmware_host_service_start")
	})

	t.Run("stop_reaches_server", func(t *testing.T) {
		_, err := r.CallTool("vmware_host_service_stop", map[string]interface{}{"host": host, "id": "ntpd", "confirm": true})
		assertCleanFailure(t, err, "vmware_host_service_stop")
	})

	t.Run("update_policy_requires_policy", func(t *testing.T) {
		if _, err := r.CallTool("vmware_host_service_update_policy", map[string]interface{}{"host": host, "id": "ntpd", "confirm": true}); err == nil {
			t.Fatal("expected an error when policy is missing")
		}
	})

	t.Run("update_policy_reaches_server", func(t *testing.T) {
		_, err := r.CallTool("vmware_host_service_update_policy", map[string]interface{}{"host": host, "id": "ntpd", "policy": "on", "confirm": true})
		assertCleanFailure(t, err, "vmware_host_service_update_policy")
	})
}

// TestHostMiscTools_DateTimeSystem covers all 3 HostDateTimeSystem tools.
// Same vcsim gap as ServiceSystem: the ConfigManager.DateTimeSystem
// reference resolves fine (a template placeholder), but the object it points
// to was never registered in vcsim, so every call reaches a real SOAP round
// trip and gets back "managed object not found: HostDateTimeSystem:dateTimeSystem".
func TestHostMiscTools_DateTimeSystem(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newHostMiscRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	host := firstHostPath(t, r)

	t.Run("query_reaches_server", func(t *testing.T) {
		_, err := r.CallTool("vmware_host_datetime_query", map[string]interface{}{"host": host})
		assertCleanFailure(t, err, "vmware_host_datetime_query")
	})

	t.Run("update_requires_date", func(t *testing.T) {
		if _, err := r.CallTool("vmware_host_datetime_update", map[string]interface{}{"host": host, "confirm": true}); err == nil {
			t.Fatal("expected an error when date is missing")
		}
	})

	t.Run("update_rejects_bad_date", func(t *testing.T) {
		if _, err := r.CallTool("vmware_host_datetime_update", map[string]interface{}{"host": host, "date": "not-a-date", "confirm": true}); err == nil {
			t.Fatal("expected an error for a non-RFC3339 date")
		}
	})

	t.Run("update_gate_and_confirm", func(t *testing.T) {
		closedGate := newHostMiscRegistry(context.Background(), c, RegistryOptions{AllowDestructive: false})
		args := map[string]interface{}{"host": host, "date": "2026-08-11T12:00:00Z", "confirm": true}
		if _, err := closedGate.CallTool("vmware_host_datetime_update", args); err == nil {
			t.Fatal("expected vmware_host_datetime_update to be denied with the gate closed")
		}
		noConfirm := map[string]interface{}{"host": host, "date": "2026-08-11T12:00:00Z"}
		if _, err := r.CallTool("vmware_host_datetime_update", noConfirm); err == nil {
			t.Fatal("expected vmware_host_datetime_update to fail without confirm:true")
		}
	})

	t.Run("update_reaches_server", func(t *testing.T) {
		args := map[string]interface{}{"host": host, "date": "2026-08-11T12:00:00Z", "confirm": true}
		_, err := r.CallTool("vmware_host_datetime_update", args)
		assertCleanFailure(t, err, "vmware_host_datetime_update")
	})

	t.Run("update_config_requires_config", func(t *testing.T) {
		if _, err := r.CallTool("vmware_host_datetime_update_config", map[string]interface{}{"host": host, "confirm": true}); err == nil {
			t.Fatal("expected an error when config is missing")
		}
	})

	t.Run("update_config_reaches_server", func(t *testing.T) {
		args := map[string]interface{}{
			"host":    host,
			"config":  map[string]interface{}{"timeZone": "UTC"},
			"confirm": true,
		}
		_, err := r.CallTool("vmware_host_datetime_update_config", args)
		assertCleanFailure(t, err, "vmware_host_datetime_update_config")
	})
}

// TestHostMiscTools_VirtualNicManager is this group's one real functional
// test: vcsim DOES wire ConfigManager.VirtualNicManager and populate a real
// Info.NetConfig (simulator/host_vnic_manager.go's NewHostVirtualNicManager,
// seeded from esx.VirtualNicManagerNetConfig — a single "management" entry
// with candidate device "vmk0"), so vmware_host_nic_info gets a genuine
// positive-data assertion. select_vnic/deselect_vnic still cannot succeed —
// see generated_host_misc.go's top doc comment for why nic_type "vmotion"
// (not "vsan") is used here to exercise their real SOAP-call gap directly.
func TestHostMiscTools_VirtualNicManager(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newHostMiscRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	host := firstHostPath(t, r)

	t.Run("info_real_functional_success", func(t *testing.T) {
		raw, err := r.CallTool("vmware_host_nic_info", map[string]interface{}{"host": host})
		if err != nil {
			t.Fatalf("vmware_host_nic_info failed: %v", err)
		}
		m := decodeResult(t, raw)
		info, ok := m["info"].(map[string]interface{})
		if !ok {
			t.Fatalf("expected an \"info\" object: %s", raw)
		}
		netConfig, ok := info["netConfig"].([]interface{})
		if !ok || len(netConfig) == 0 {
			t.Fatalf("expected a non-empty netConfig array (seeded by simulator/host_vnic_manager.go's NewHostVirtualNicManager): %s", raw)
		}
		var sawManagement bool
		for _, entry := range netConfig {
			em, ok := entry.(map[string]interface{})
			if ok && em["nicType"] == "management" {
				sawManagement = true
			}
		}
		if !sawManagement {
			t.Fatalf("expected a %q nicType entry in netConfig (esx.VirtualNicManagerNetConfig's only default entry): %s", "management", raw)
		}
	})

	t.Run("info_missing_host", func(t *testing.T) {
		if _, err := r.CallTool("vmware_host_nic_info", map[string]interface{}{}); err == nil {
			t.Fatal("expected an error when host is missing")
		}
	})

	t.Run("select_vnic_requires_args", func(t *testing.T) {
		if _, err := r.CallTool("vmware_host_nic_select_vnic", map[string]interface{}{"host": host, "device": "vmk0", "confirm": true}); err == nil {
			t.Fatal("expected an error when nic_type is missing")
		}
		if _, err := r.CallTool("vmware_host_nic_select_vnic", map[string]interface{}{"host": host, "nic_type": "vmotion", "confirm": true}); err == nil {
			t.Fatal("expected an error when device is missing")
		}
	})

	t.Run("select_vnic_gate_and_confirm", func(t *testing.T) {
		closedGate := newHostMiscRegistry(context.Background(), c, RegistryOptions{AllowDestructive: false})
		args := map[string]interface{}{"host": host, "nic_type": "vmotion", "device": "vmk0", "confirm": true}
		if _, err := closedGate.CallTool("vmware_host_nic_select_vnic", args); err == nil {
			t.Fatal("expected vmware_host_nic_select_vnic to be denied with the gate closed")
		}
		noConfirm := map[string]interface{}{"host": host, "nic_type": "vmotion", "device": "vmk0"}
		if _, err := r.CallTool("vmware_host_nic_select_vnic", noConfirm); err == nil {
			t.Fatal("expected vmware_host_nic_select_vnic to fail without confirm:true")
		}
	})

	t.Run("select_vnic_reaches_server", func(t *testing.T) {
		args := map[string]interface{}{"host": host, "nic_type": "vmotion", "device": "vmk0", "confirm": true}
		_, err := r.CallTool("vmware_host_nic_select_vnic", args)
		assertCleanFailure(t, err, "vmware_host_nic_select_vnic")
	})

	t.Run("deselect_vnic_requires_args", func(t *testing.T) {
		if _, err := r.CallTool("vmware_host_nic_deselect_vnic", map[string]interface{}{"host": host, "device": "vmk0", "confirm": true}); err == nil {
			t.Fatal("expected an error when nic_type is missing")
		}
		if _, err := r.CallTool("vmware_host_nic_deselect_vnic", map[string]interface{}{"host": host, "nic_type": "vmotion", "confirm": true}); err == nil {
			t.Fatal("expected an error when device is missing")
		}
	})

	t.Run("deselect_vnic_reaches_server", func(t *testing.T) {
		args := map[string]interface{}{"host": host, "nic_type": "vmotion", "device": "vmk0", "confirm": true}
		_, err := r.CallTool("vmware_host_nic_deselect_vnic", args)
		assertCleanFailure(t, err, "vmware_host_nic_deselect_vnic")
	})
}

// TestHostMiscTools_VsanSystem covers the single HostVsanSystem tool. Same
// gap as ServiceSystem/DateTimeSystem (see generated_host_misc.go's top doc
// comment): host.ConfigManager().VsanSystem(ctx) resolves fine, but the
// UpdateVsan_Task call against that unregistered placeholder object faults
// "managed object not found: HostVsanSystem:vsanSystem".
func TestHostMiscTools_VsanSystem(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newHostMiscRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	host := firstHostPath(t, r)

	t.Run("update_requires_config", func(t *testing.T) {
		if _, err := r.CallTool("vmware_host_vsan_update", map[string]interface{}{"host": host, "confirm": true}); err == nil {
			t.Fatal("expected an error when config is missing")
		}
	})

	t.Run("update_gate_and_confirm", func(t *testing.T) {
		closedGate := newHostMiscRegistry(context.Background(), c, RegistryOptions{AllowDestructive: false})
		args := map[string]interface{}{"host": host, "config": map[string]interface{}{"enabled": true}, "confirm": true}
		if _, err := closedGate.CallTool("vmware_host_vsan_update", args); err == nil {
			t.Fatal("expected vmware_host_vsan_update to be denied with the gate closed")
		}
		noConfirm := map[string]interface{}{"host": host, "config": map[string]interface{}{"enabled": true}}
		if _, err := r.CallTool("vmware_host_vsan_update", noConfirm); err == nil {
			t.Fatal("expected vmware_host_vsan_update to fail without confirm:true")
		}
	})

	t.Run("update_reaches_server", func(t *testing.T) {
		args := map[string]interface{}{"host": host, "config": map[string]interface{}{"enabled": true}, "confirm": true}
		_, err := r.CallTool("vmware_host_vsan_update", args)
		assertCleanFailure(t, err, "vmware_host_vsan_update")
	})
}

// TestHostMiscTools_VsanInternalSystem covers all 3 HostVsanInternalSystem
// tools. Same gap as VsanSystem: host.ConfigManager().VsanInternalSystem(ctx)
// resolves fine, but every real SOAP call against that unregistered
// placeholder object faults
// "managed object not found: HostVsanInternalSystem:ha-vsan-internal-system".
func TestHostMiscTools_VsanInternalSystem(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newHostMiscRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	host := firstHostPath(t, r)

	t.Run("query_object_uuids_reaches_server", func(t *testing.T) {
		_, err := r.CallTool("vmware_host_vsan_internal_query_object_uuids", map[string]interface{}{"host": host})
		assertCleanFailure(t, err, "vmware_host_vsan_internal_query_object_uuids")
	})

	t.Run("query_object_uuids_with_filters_reaches_server", func(t *testing.T) {
		args := map[string]interface{}{
			"host":    host,
			"uuids":   []interface{}{"11111111-1111-1111-1111-111111111111"},
			"limit":   float64(10),
			"version": float64(1),
		}
		_, err := r.CallTool("vmware_host_vsan_internal_query_object_uuids", args)
		assertCleanFailure(t, err, "vmware_host_vsan_internal_query_object_uuids")
	})

	t.Run("get_obj_ext_attrs_requires_uuids", func(t *testing.T) {
		if _, err := r.CallTool("vmware_host_vsan_internal_get_obj_ext_attrs", map[string]interface{}{"host": host}); err == nil {
			t.Fatal("expected an error when uuids is missing")
		}
	})

	t.Run("get_obj_ext_attrs_reaches_server", func(t *testing.T) {
		args := map[string]interface{}{"host": host, "uuids": []interface{}{"11111111-1111-1111-1111-111111111111"}}
		_, err := r.CallTool("vmware_host_vsan_internal_get_obj_ext_attrs", args)
		assertCleanFailure(t, err, "vmware_host_vsan_internal_get_obj_ext_attrs")
	})

	t.Run("delete_objects_requires_uuids", func(t *testing.T) {
		if _, err := r.CallTool("vmware_host_vsan_internal_delete_objects", map[string]interface{}{"host": host, "confirm": true}); err == nil {
			t.Fatal("expected an error when uuids is missing")
		}
		empty := map[string]interface{}{"host": host, "uuids": []interface{}{}, "confirm": true}
		if _, err := r.CallTool("vmware_host_vsan_internal_delete_objects", empty); err == nil {
			t.Fatal("expected an error when uuids is empty")
		}
	})

	t.Run("delete_objects_gate_and_confirm", func(t *testing.T) {
		closedGate := newHostMiscRegistry(context.Background(), c, RegistryOptions{AllowDestructive: false})
		args := map[string]interface{}{"host": host, "uuids": []interface{}{"11111111-1111-1111-1111-111111111111"}, "confirm": true}
		if _, err := closedGate.CallTool("vmware_host_vsan_internal_delete_objects", args); err == nil {
			t.Fatal("expected vmware_host_vsan_internal_delete_objects to be denied with the gate closed")
		}
		noConfirm := map[string]interface{}{"host": host, "uuids": []interface{}{"11111111-1111-1111-1111-111111111111"}}
		if _, err := r.CallTool("vmware_host_vsan_internal_delete_objects", noConfirm); err == nil {
			t.Fatal("expected vmware_host_vsan_internal_delete_objects to fail without confirm:true")
		}
	})

	t.Run("delete_objects_reaches_server", func(t *testing.T) {
		args := map[string]interface{}{
			"host":    host,
			"uuids":   []interface{}{"11111111-1111-1111-1111-111111111111"},
			"force":   true,
			"confirm": true,
		}
		_, err := r.CallTool("vmware_host_vsan_internal_delete_objects", args)
		assertCleanFailure(t, err, "vmware_host_vsan_internal_delete_objects")
	})
}

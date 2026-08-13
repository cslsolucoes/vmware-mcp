package tools

import (
	"context"
	"testing"

	"github.com/vmware/govmomi/simulator"
)

func firstHostPath(t *testing.T, r *Registry) string {
	t.Helper()
	raw, err := r.CallTool("vmware_list_hosts", map[string]interface{}{})
	if err != nil {
		t.Fatalf("vmware_list_hosts failed: %v", err)
	}
	list, _ := decodeResult(t, raw)["hosts"].([]interface{})
	if len(list) == 0 {
		t.Fatal("simulator model has no hosts")
	}
	return list[0].(string)
}

func hostInfo(t *testing.T, r *Registry, host string) map[string]interface{} {
	t.Helper()
	raw, err := r.CallTool("vmware_host_info", map[string]interface{}{"host": host})
	if err != nil {
		t.Fatalf("vmware_host_info(%s) failed: %v", host, err)
	}
	return decodeResult(t, raw)
}

func TestHostTools_MaintenanceCycle(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := NewRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	host := firstHostPath(t, r)

	if got := hostInfo(t, r, host)["in_maintenance_mode"]; got != false {
		t.Fatalf("expected host to start out of maintenance mode, got %v", got)
	}

	if _, err := r.CallTool("vmware_host_maintenance_enter", map[string]interface{}{"host": host, "confirm": true}); err != nil {
		t.Fatalf("vmware_host_maintenance_enter failed: %v", err)
	}
	if got := hostInfo(t, r, host)["in_maintenance_mode"]; got != true {
		t.Fatalf("expected in_maintenance_mode=true after entering, got %v", got)
	}

	if _, err := r.CallTool("vmware_host_maintenance_exit", map[string]interface{}{"host": host}); err != nil {
		t.Fatalf("vmware_host_maintenance_exit failed: %v", err)
	}
	if got := hostInfo(t, r, host)["in_maintenance_mode"]; got != false {
		t.Fatalf("expected in_maintenance_mode=false after exiting, got %v", got)
	}
}

func TestHostTools_MaintenanceEnterGateAndConfirm(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	host := firstHostPath(t, NewRegistry(context.Background(), c, RegistryOptions{}))

	closedGate := NewRegistry(context.Background(), c, RegistryOptions{AllowDestructive: false})
	if _, err := closedGate.CallTool("vmware_host_maintenance_enter", map[string]interface{}{"host": host, "confirm": true}); err == nil {
		t.Fatal("expected vmware_host_maintenance_enter to be denied with the gate closed")
	}
	if got := hostInfo(t, closedGate, host)["in_maintenance_mode"]; got != false {
		t.Fatalf("host entered maintenance mode despite the gate being closed: %v", got)
	}

	openGate := NewRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	if _, err := openGate.CallTool("vmware_host_maintenance_enter", map[string]interface{}{"host": host}); err == nil {
		t.Fatal("expected vmware_host_maintenance_enter to fail without confirm:true")
	}
	if got := hostInfo(t, openGate, host)["in_maintenance_mode"]; got != false {
		t.Fatalf("host entered maintenance mode despite missing confirm: %v", got)
	}
}

func TestHostTools_ManagementIPsAndReconnect(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := NewRegistry(context.Background(), c, RegistryOptions{})
	host := firstHostPath(t, r)

	raw, err := r.CallTool("vmware_host_management_ips", map[string]interface{}{"host": host})
	if err != nil {
		t.Fatalf("vmware_host_management_ips failed: %v", err)
	}
	m := decodeResult(t, raw)
	if _, ok := m["ips"].([]interface{}); !ok {
		t.Fatalf("expected an \"ips\" array in the result: %s", raw)
	}

	if _, err := r.CallTool("vmware_host_reconnect", map[string]interface{}{"host": host}); err != nil {
		t.Fatalf("vmware_host_reconnect failed: %v", err)
	}
}

func TestHostTools_Info(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := NewRegistry(context.Background(), c, RegistryOptions{})
	host := firstHostPath(t, r)

	info := hostInfo(t, r, host)
	for _, field := range []string{"name", "connection_state", "power_state", "in_maintenance_mode", "model", "num_cpu_cores", "memory_bytes"} {
		if _, ok := info[field]; !ok {
			t.Fatalf("vmware_host_info result missing %q field: %+v", field, info)
		}
	}
	if info["power_state"] != "poweredOn" {
		t.Fatalf("expected the ESXi host itself to report poweredOn, got %v", info["power_state"])
	}
}

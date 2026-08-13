package tools

import (
	"context"
	"testing"

	"github.com/vmware/govmomi/simulator"
)

func TestOptionTools_QueryAndUpdateCycle(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := NewRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	host := firstHostPath(t, r)

	raw, err := r.CallTool("vmware_host_option_query", map[string]interface{}{"host": host})
	if err != nil {
		t.Fatalf("vmware_host_option_query failed: %v", err)
	}
	before := decodeResult(t, raw)
	baseline, _ := before["count"].(float64)
	if baseline <= 0 {
		t.Fatalf("expected the simulator's host to already report advanced options, got count=%v", before["count"])
	}

	if _, err := r.CallTool("vmware_host_option_update", map[string]interface{}{
		"host": host,
		"options": []interface{}{
			map[string]interface{}{"key": "MCPVMWareTest.Pilot", "value": "hello"},
		},
		"confirm": true,
	}); err != nil {
		t.Fatalf("vmware_host_option_update failed: %v", err)
	}

	raw, err = r.CallTool("vmware_host_option_query", map[string]interface{}{"host": host, "name": "MCPVMWareTest.Pilot"})
	if err != nil {
		t.Fatalf("vmware_host_option_query (filtered) failed: %v", err)
	}
	after := decodeResult(t, raw)
	opts, _ := after["options"].([]interface{})
	if len(opts) != 1 {
		t.Fatalf("expected exactly 1 option named MCPVMWareTest.Pilot after update, got %d: %v", len(opts), opts)
	}
	got := opts[0].(map[string]interface{})
	if got["key"] != "MCPVMWareTest.Pilot" || got["value"] != "hello" {
		t.Fatalf("unexpected option after update: %+v", got)
	}
}

func TestOptionTools_UpdateGateAndConfirm(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	host := firstHostPath(t, NewRegistry(context.Background(), c, RegistryOptions{}))
	updateArgs := map[string]interface{}{
		"host":    host,
		"options": []interface{}{map[string]interface{}{"key": "MCPVMWareTest.Denied", "value": "x"}},
	}

	closedGate := NewRegistry(context.Background(), c, RegistryOptions{AllowDestructive: false})
	withConfirm := map[string]interface{}{"host": host, "options": updateArgs["options"], "confirm": true}
	if _, err := closedGate.CallTool("vmware_host_option_update", withConfirm); err == nil {
		t.Fatal("expected vmware_host_option_update to be denied with the gate closed")
	}

	openGate := NewRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	if _, err := openGate.CallTool("vmware_host_option_update", updateArgs); err == nil {
		t.Fatal("expected vmware_host_option_update to fail without confirm:true")
	}

	// Prove the denied calls above never reached the simulator: the option
	// must not exist under either name.
	raw, err := openGate.CallTool("vmware_host_option_query", map[string]interface{}{"host": host, "name": "MCPVMWareTest.Denied"})
	if err != nil {
		t.Fatalf("vmware_host_option_query failed: %v", err)
	}
	if count, _ := decodeResult(t, raw)["count"].(float64); count != 0 {
		t.Fatalf("expected 0 options named MCPVMWareTest.Denied (update should have been denied), got %v", count)
	}
}

func TestOptionTools_UpdateRejectsMalformedOptions(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := NewRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	host := firstHostPath(t, r)

	cases := []map[string]interface{}{
		{"host": host, "confirm": true},                                                                 // missing options entirely
		{"host": host, "confirm": true, "options": []interface{}{}},                                     // empty array
		{"host": host, "confirm": true, "options": []interface{}{"not-an-object"}},                      // wrong item type
		{"host": host, "confirm": true, "options": []interface{}{map[string]interface{}{"value": "x"}}}, // missing key
	}
	for i, args := range cases {
		if _, err := r.CallTool("vmware_host_option_update", args); err == nil {
			t.Errorf("case %d: expected vmware_host_option_update to reject malformed options, args=%v", i, args)
		}
	}
}

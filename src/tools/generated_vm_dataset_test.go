package tools

import (
	"context"
	"testing"

	"github.com/vmware/govmomi/simulator"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// newVMDatasetRegistry builds a Registry the normal way (NewRegistry, which
// wires every other domain via registerTools) and then manually layers
// registerVMDatasetTools on top via withClass — same pattern as
// newLibraryCoreRegistry (generated_library_core_test.go). This file must not
// edit registry.go itself (see generated_vm_dataset.go's top doc comment).
func newVMDatasetRegistry(ctx context.Context, c *vmware.Client, opts RegistryOptions) *Registry {
	r := NewRegistry(ctx, c, opts)
	r.withClass(modeVCenterOnly, registerVMDatasetTools)
	return r
}

// TestVMDatasetTools_Registration proves all 9 tools are registered and
// reachable via ListTools.
func TestVMDatasetTools_Registration(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newVMDatasetRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	want := []string{
		"vmware_vm_dataset_create_data_set",
		"vmware_vm_dataset_delete_data_set",
		"vmware_vm_dataset_delete_entry",
		"vmware_vm_dataset_get_data_set",
		"vmware_vm_dataset_get_entry",
		"vmware_vm_dataset_list_data_sets",
		"vmware_vm_dataset_list_entries",
		"vmware_vm_dataset_set_entry",
		"vmware_vm_dataset_update_data_set",
	}
	if len(want) != 9 {
		t.Fatalf("test bug: want list has %d entries, expected 9", len(want))
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

// TestVMDatasetTools_ArgValidation proves each tool rejects missing required
// arguments with a clean error (not a panic), matching this group's brief:
// "validação de argumentos obrigatórios falha ANTES de tocar o servidor" —
// vm/data_set/key/spec presence is checked in Go before any dataset.Manager
// method call reaches vcsim's HTTP layer (only client.REST(ctx)'s login,
// shared/cached across every handler in this package, touches the network
// first).
func TestVMDatasetTools_ArgValidation(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newVMDatasetRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	cases := []struct {
		tool string
		args map[string]interface{}
	}{
		{"vmware_vm_dataset_get_data_set", map[string]interface{}{"data_set": "com.example.test"}},                                                                                     // missing vm
		{"vmware_vm_dataset_get_data_set", map[string]interface{}{"vm": "vm-1"}},                                                                                                       // missing data_set
		{"vmware_vm_dataset_get_entry", map[string]interface{}{"vm": "vm-1", "data_set": "com.example.test"}},                                                                          // missing key
		{"vmware_vm_dataset_list_data_sets", map[string]interface{}{}},                                                                                                                 // missing vm
		{"vmware_vm_dataset_list_entries", map[string]interface{}{"vm": "vm-1"}},                                                                                                       // missing data_set
		{"vmware_vm_dataset_create_data_set", map[string]interface{}{"vm": "vm-1", "confirm": true}},                                                                                   // missing spec
		{"vmware_vm_dataset_create_data_set", map[string]interface{}{"vm": "vm-1", "confirm": true, "spec": map[string]interface{}{"host": "READ_ONLY", "guest": "READ_ONLY"}}},        // missing spec.name
		{"vmware_vm_dataset_create_data_set", map[string]interface{}{"vm": "vm-1", "confirm": true, "spec": map[string]interface{}{"name": "com.example.test", "guest": "READ_ONLY"}}}, // missing spec.host
		{"vmware_vm_dataset_create_data_set", map[string]interface{}{"vm": "vm-1", "confirm": true, "spec": map[string]interface{}{"name": "com.example.test", "host": "READ_ONLY"}}},  // missing spec.guest
		{"vmware_vm_dataset_set_entry", map[string]interface{}{"vm": "vm-1", "data_set": "com.example.test", "confirm": true}},                                                         // missing key
		{"vmware_vm_dataset_update_data_set", map[string]interface{}{"vm": "vm-1", "confirm": true}},                                                                                   // missing data_set
		{"vmware_vm_dataset_update_data_set", map[string]interface{}{"vm": "vm-1", "data_set": "com.example.test", "confirm": true}},                                                   // missing spec
		{"vmware_vm_dataset_delete_data_set", map[string]interface{}{"vm": "vm-1", "confirm": true}},                                                                                   // missing data_set
		{"vmware_vm_dataset_delete_entry", map[string]interface{}{"vm": "vm-1", "data_set": "com.example.test", "confirm": true}},                                                      // missing key
	}

	for _, tc := range cases {
		if _, err := r.CallTool(tc.tool, tc.args); err == nil {
			t.Errorf("%s(%v): expected an error for missing required argument", tc.tool, tc.args)
		}
	}
}

// TestVMDatasetTools_GateAndConfirm proves the Tier 1/2 tools in this file
// are wired through registerDestructive, the same 3-layer protection check
// pattern as generated_network_test.go's TestNetworkTools_GateAndConfirm.
func TestVMDatasetTools_GateAndConfirm(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	validSpec := map[string]interface{}{
		"vm": "vm-1",
		"spec": map[string]interface{}{
			"name":  "com.example.test",
			"host":  "READ_ONLY",
			"guest": "READ_ONLY",
		},
	}

	closedGate := newVMDatasetRegistry(context.Background(), c, RegistryOptions{AllowDestructive: false})
	args := map[string]interface{}{"vm": validSpec["vm"], "spec": validSpec["spec"], "confirm": true}
	if _, err := closedGate.CallTool("vmware_vm_dataset_create_data_set", args); err == nil {
		t.Fatal("expected vmware_vm_dataset_create_data_set to be denied with the gate closed")
	}
	if _, err := closedGate.CallTool("vmware_vm_dataset_delete_data_set", map[string]interface{}{"vm": "vm-1", "data_set": "x", "confirm": true}); err == nil {
		t.Fatal("expected vmware_vm_dataset_delete_data_set to be denied with the gate closed")
	}

	openGate := newVMDatasetRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	noConfirmArgs := map[string]interface{}{"vm": validSpec["vm"], "spec": validSpec["spec"]}
	if _, err := openGate.CallTool("vmware_vm_dataset_create_data_set", noConfirmArgs); err == nil {
		t.Fatal("expected vmware_vm_dataset_create_data_set to fail without confirm:true")
	}
	if _, err := openGate.CallTool("vmware_vm_dataset_delete_entry", map[string]interface{}{"vm": "vm-1", "data_set": "x", "key": "k"}); err == nil {
		t.Fatal("expected vmware_vm_dataset_delete_entry to fail without confirm:true")
	}
}

// TestVMDatasetTools_ReachesServer proves every one of this file's 9 tools
// reaches real vcsim (or, for the read-only tools, real business logic
// inside the govmomi REST client) instead of failing on something wired
// wrong here — see generated_vm_dataset.go's top doc comment: no vcsim
// handler exists anywhere for the vm/dataset REST routes ("vcsim gap, not a
// bug", confirmed by grep against referencia/govmomi/vapi/simulator/simulator.go).
// err is expected to be non-nil for every call; assertReachesServer
// (generated_vm_lifecycle_test.go) proves it's a real server-side/HTTP fault,
// not "unknown tool" or a recovered panic.
func TestVMDatasetTools_ReachesServer(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newVMDatasetRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	const vm = "vm-1"
	const dataSet = "com.example.test"
	const key = "k1"

	t.Run("get_data_set", func(t *testing.T) {
		_, err := r.CallTool("vmware_vm_dataset_get_data_set", map[string]interface{}{"vm": vm, "data_set": dataSet})
		assertReachesServer(t, err, "vmware_vm_dataset_get_data_set")
	})

	t.Run("get_entry", func(t *testing.T) {
		_, err := r.CallTool("vmware_vm_dataset_get_entry", map[string]interface{}{"vm": vm, "data_set": dataSet, "key": key})
		assertReachesServer(t, err, "vmware_vm_dataset_get_entry")
	})

	t.Run("list_data_sets", func(t *testing.T) {
		_, err := r.CallTool("vmware_vm_dataset_list_data_sets", map[string]interface{}{"vm": vm})
		assertReachesServer(t, err, "vmware_vm_dataset_list_data_sets")
	})

	t.Run("list_entries", func(t *testing.T) {
		_, err := r.CallTool("vmware_vm_dataset_list_entries", map[string]interface{}{"vm": vm, "data_set": dataSet})
		assertReachesServer(t, err, "vmware_vm_dataset_list_entries")
	})

	t.Run("create_data_set", func(t *testing.T) {
		_, err := r.CallTool("vmware_vm_dataset_create_data_set", map[string]interface{}{
			"vm": vm,
			"spec": map[string]interface{}{
				"name":  dataSet,
				"host":  "READ_ONLY",
				"guest": "READ_ONLY",
			},
			"confirm": true,
		})
		assertReachesServer(t, err, "vmware_vm_dataset_create_data_set")
	})

	t.Run("set_entry", func(t *testing.T) {
		_, err := r.CallTool("vmware_vm_dataset_set_entry", map[string]interface{}{
			"vm": vm, "data_set": dataSet, "key": key, "value": "v1", "confirm": true,
		})
		assertReachesServer(t, err, "vmware_vm_dataset_set_entry")
	})

	t.Run("update_data_set", func(t *testing.T) {
		_, err := r.CallTool("vmware_vm_dataset_update_data_set", map[string]interface{}{
			"vm": vm, "data_set": dataSet,
			"spec":    map[string]interface{}{"description": "updated"},
			"confirm": true,
		})
		assertReachesServer(t, err, "vmware_vm_dataset_update_data_set")
	})

	t.Run("delete_data_set", func(t *testing.T) {
		_, err := r.CallTool("vmware_vm_dataset_delete_data_set", map[string]interface{}{
			"vm": vm, "data_set": dataSet, "confirm": true,
		})
		assertReachesServer(t, err, "vmware_vm_dataset_delete_data_set")
	})

	t.Run("delete_entry", func(t *testing.T) {
		_, err := r.CallTool("vmware_vm_dataset_delete_entry", map[string]interface{}{
			"vm": vm, "data_set": dataSet, "key": key, "confirm": true,
		})
		assertReachesServer(t, err, "vmware_vm_dataset_delete_entry")
	})
}

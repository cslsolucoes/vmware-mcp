package tools

import (
	"context"
	"testing"

	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25/mo"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// newSearchIndexRegistry builds a Registry the normal way and layers this
// group's search-index tools on top via withClass, same pattern as
// generated_vm_lifecycle_test.go's newLifecycleRegistry.
func newSearchIndexRegistry(ctx context.Context, c *vmware.Client, opts RegistryOptions) *Registry {
	r := NewRegistry(ctx, c, opts)
	r.withClass(modeVSphereGeneral, registerSearchIndexTools)
	return r
}

// vmPathName reads a VM's real config.files.vmPathName (its .vmx datastore
// path) directly via the client's property collector — needed as a test
// fixture for vmware_search_index_find_by_datastore_path, which no tool in
// this project exposes directly (same "direct client access for fixture
// setup, not a registered tool" pattern as generated_vm_lifecycle_test.go's
// firstDiskKeyAndCapacity).
func vmPathName(t *testing.T, ctx context.Context, c *vmware.Client, vmPath string) string {
	t.Helper()
	vm, err := c.Finder.VirtualMachine(ctx, vmPath)
	if err != nil {
		t.Fatalf("failed to resolve %s: %v", vmPath, err)
	}
	var mvm mo.VirtualMachine
	if err := vm.Properties(ctx, vm.Reference(), []string{"config.files.vmPathName"}, &mvm); err != nil {
		t.Fatalf("failed to read config.files.vmPathName for %s: %v", vmPath, err)
	}
	return mvm.Config.Files.VmPathName
}

// TestSearchIndexTools_FindByInventoryPath proves the simplest, always-valid
// success path (every result type resolves via find.InventoryPath) and the
// "not found" (not an error) path.
func TestSearchIndexTools_FindByInventoryPath(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newSearchIndexRegistry(context.Background(), c, RegistryOptions{})
	vm := firstVMPath(t, r)

	t.Run("found", func(t *testing.T) {
		raw, err := r.CallTool("vmware_search_index_find_by_inventory_path", map[string]interface{}{"path": vm})
		if err != nil {
			t.Fatalf("vmware_search_index_find_by_inventory_path(%s) failed: %v", vm, err)
		}
		m := decodeResult(t, raw)
		if m["found"] != true {
			t.Fatalf("expected found=true for %s, got %v (%s)", vm, m["found"], raw)
		}
		if m["type"] != "VirtualMachine" {
			t.Fatalf("expected type=VirtualMachine, got %v (%s)", m["type"], raw)
		}
		if m["inventory_path"] != vm {
			t.Fatalf("expected inventory_path=%q, got %v (%s)", vm, m["inventory_path"], raw)
		}
	})

	t.Run("not_found_is_not_an_error", func(t *testing.T) {
		raw, err := r.CallTool("vmware_search_index_find_by_inventory_path", map[string]interface{}{"path": "/ha-datacenter/vm/does-not-exist"})
		if err != nil {
			t.Fatalf("expected a clean {found:false} result, not an error: %v", err)
		}
		if m := decodeResult(t, raw); m["found"] != false {
			t.Fatalf("expected found=false, got %v (%s)", m["found"], raw)
		}
	})

	t.Run("path_required", func(t *testing.T) {
		if _, err := r.CallTool("vmware_search_index_find_by_inventory_path", map[string]interface{}{}); err == nil {
			t.Fatal("expected an error when path is missing")
		}
	})
}

// TestSearchIndexTools_FindByUuid proves a real UUID lookup round-trips
// through vmware_vm_info's own "uuid" field.
func TestSearchIndexTools_FindByUuid(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newSearchIndexRegistry(context.Background(), c, RegistryOptions{})
	vm := firstVMPath(t, r)

	uuid, _ := vmInfo(t, r, vm)["uuid"].(string)
	if uuid == "" {
		t.Fatal("expected a non-empty uuid from vmware_vm_info")
	}

	t.Run("find_by_uuid", func(t *testing.T) {
		raw, err := r.CallTool("vmware_search_index_find_by_uuid", map[string]interface{}{"uuid": uuid})
		if err != nil {
			t.Fatalf("vmware_search_index_find_by_uuid(%s) failed: %v", uuid, err)
		}
		m := decodeResult(t, raw)
		if m["found"] != true || m["inventory_path"] != vm {
			t.Fatalf("expected found=true, inventory_path=%q, got %s", vm, raw)
		}
	})

	t.Run("find_all_by_uuid", func(t *testing.T) {
		raw, err := r.CallTool("vmware_search_index_find_all_by_uuid", map[string]interface{}{"uuid": uuid})
		if err != nil {
			t.Fatalf("vmware_search_index_find_all_by_uuid(%s) failed: %v", uuid, err)
		}
		m := decodeResult(t, raw)
		if m["count"] != float64(1) {
			t.Fatalf("expected count=1, got %v (%s)", m["count"], raw)
		}
	})

	t.Run("uuid_required", func(t *testing.T) {
		if _, err := r.CallTool("vmware_search_index_find_by_uuid", map[string]interface{}{}); err == nil {
			t.Fatal("expected an error when uuid is missing")
		}
	})
}

// TestSearchIndexTools_FindByIp proves a real IP lookup, using the same
// "SET.guest.ipAddress" ExtraConfig fixture trick as
// generated_vm_lifecycle_test.go's setGuestIP (reused here, not duplicated) —
// confirmed against referencia/govmomi/simulator/search_index.go's
// FindAllByIp, which reads the same vm.Guest.IpAddress field that trick
// sets.
func TestSearchIndexTools_FindByIp(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newSearchIndexRegistry(context.Background(), c, RegistryOptions{})
	vm := firstVMPath(t, r)

	setGuestIP(t, context.Background(), c, vm, "10.20.30.40")

	t.Run("find_by_ip", func(t *testing.T) {
		raw, err := r.CallTool("vmware_search_index_find_by_ip", map[string]interface{}{"ip": "10.20.30.40"})
		if err != nil {
			t.Fatalf("vmware_search_index_find_by_ip failed: %v", err)
		}
		m := decodeResult(t, raw)
		if m["found"] != true || m["inventory_path"] != vm {
			t.Fatalf("expected found=true, inventory_path=%q, got %s", vm, raw)
		}
	})

	t.Run("find_all_by_ip", func(t *testing.T) {
		raw, err := r.CallTool("vmware_search_index_find_all_by_ip", map[string]interface{}{"ip": "10.20.30.40"})
		if err != nil {
			t.Fatalf("vmware_search_index_find_all_by_ip failed: %v", err)
		}
		if m := decodeResult(t, raw); m["count"] != float64(1) {
			t.Fatalf("expected count=1, got %v (%s)", m["count"], raw)
		}
	})

	t.Run("not_found", func(t *testing.T) {
		raw, err := r.CallTool("vmware_search_index_find_by_ip", map[string]interface{}{"ip": "192.0.2.1"})
		if err != nil {
			t.Fatalf("expected a clean {found:false} result, not an error: %v", err)
		}
		if m := decodeResult(t, raw); m["found"] != false {
			t.Fatalf("expected found=false, got %v (%s)", m["found"], raw)
		}
	})
}

// TestSearchIndexTools_FindByDnsName proves the not-found path (no vcsim
// fixture exists for setting Guest.HostName without a real guest OS) and
// that datacenter-scoping is at least accepted.
func TestSearchIndexTools_FindByDnsName(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newSearchIndexRegistry(context.Background(), c, RegistryOptions{})

	raw, err := r.CallTool("vmware_search_index_find_by_dns_name", map[string]interface{}{"dns_name": "no-such-host.example.com"})
	if err != nil {
		t.Fatalf("vmware_search_index_find_by_dns_name failed: %v", err)
	}
	if m := decodeResult(t, raw); m["found"] != false {
		t.Fatalf("expected found=false, got %v (%s)", m["found"], raw)
	}

	if _, err := r.CallTool("vmware_search_index_find_by_dns_name", map[string]interface{}{}); err == nil {
		t.Fatal("expected an error when dns_name is missing")
	}
}

// TestSearchIndexTools_FindByDatastorePath proves the real .vmx path lookup
// succeeds, and that datacenter is genuinely required for this one tool
// (unlike every other tool in this file) — see generated_search_index.go's
// top doc comment for why (unconditional dereference in the real source).
func TestSearchIndexTools_FindByDatastorePath(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newSearchIndexRegistry(context.Background(), c, RegistryOptions{})
	vm := firstVMPath(t, r)
	vmxPath := vmPathName(t, context.Background(), c, vm)
	if vmxPath == "" {
		t.Fatal("expected a non-empty config.files.vmPathName")
	}

	t.Run("datacenter_required", func(t *testing.T) {
		if _, err := r.CallTool("vmware_search_index_find_by_datastore_path", map[string]interface{}{"path": vmxPath}); err == nil {
			t.Fatal("expected an error when datacenter is missing")
		}
	})

	t.Run("found", func(t *testing.T) {
		raw, err := r.CallTool("vmware_search_index_find_by_datastore_path", map[string]interface{}{"datacenter": "ha-datacenter", "path": vmxPath})
		if err != nil {
			t.Fatalf("vmware_search_index_find_by_datastore_path(%s) failed: %v", vmxPath, err)
		}
		m := decodeResult(t, raw)
		if m["found"] != true || m["inventory_path"] != vm {
			t.Fatalf("expected found=true, inventory_path=%q, got %s", vm, raw)
		}
	})
}

// TestSearchIndexTools_FindChild proves a real folder-child lookup: the
// datacenter's own vmFolder is found by inventory path first (giving a real
// moref to pass as "entity"), then FindChild locates the VM by name inside
// it.
func TestSearchIndexTools_FindChild(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newSearchIndexRegistry(context.Background(), c, RegistryOptions{})
	vm := firstVMPath(t, r)

	raw, err := r.CallTool("vmware_search_index_find_by_inventory_path", map[string]interface{}{"path": "/ha-datacenter/vm"})
	if err != nil {
		t.Fatalf("vmware_search_index_find_by_inventory_path(/ha-datacenter/vm) failed: %v", err)
	}
	folder := decodeResult(t, raw)
	if folder["found"] != true {
		t.Fatalf("expected the vm folder to be found: %s", raw)
	}
	entity := map[string]interface{}{"type": folder["type"], "value": folder["value"]}

	// vm is a full inventory path like "/ha-datacenter/vm/<name>" — the
	// child's bare name is the last path segment.
	name := vm[len("/ha-datacenter/vm/"):]

	t.Run("found", func(t *testing.T) {
		raw, err := r.CallTool("vmware_search_index_find_child", map[string]interface{}{"entity": entity, "name": name})
		if err != nil {
			t.Fatalf("vmware_search_index_find_child failed: %v", err)
		}
		m := decodeResult(t, raw)
		if m["found"] != true || m["inventory_path"] != vm {
			t.Fatalf("expected found=true, inventory_path=%q, got %s", vm, raw)
		}
	})

	t.Run("entity_required_fields", func(t *testing.T) {
		if _, err := r.CallTool("vmware_search_index_find_child", map[string]interface{}{"entity": map[string]interface{}{}, "name": name}); err == nil {
			t.Fatal("expected an error for an entity with empty type/value")
		}
	})
}

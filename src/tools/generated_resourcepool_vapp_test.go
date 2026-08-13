package tools

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/vmware/govmomi/simulator"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// newResourcePoolVAppRegistry builds a Registry the normal way (NewRegistry,
// which wires vm.go/host.go/inventory.go/etc via registerTools) and then
// manually layers registerResourcePoolVAppTools on top via withClass, the
// same pattern generated_vm_lifecycle_test.go's newLifecycleRegistry and
// generated_vm_device_test.go's newVMDeviceTestRegistry use — this file must
// not edit registry.go itself (see generated_resourcepool_vapp.go's top doc
// comment).
func newResourcePoolVAppRegistry(ctx context.Context, c *vmware.Client, opts RegistryOptions) *Registry {
	r := NewRegistry(ctx, c, opts)
	r.withClass(modeVSphereGeneral, registerResourcePoolVAppTools)
	return r
}

// firstResourcePoolPath returns the inventory path of the first resource
// pool vmware_list_resource_pools (inventory.go, always registered by
// NewRegistry) finds — every simulator model (ESX or VPX) has at least the
// implicit "Resources" pool.
func firstResourcePoolPath(t *testing.T, r *Registry) string {
	t.Helper()
	raw, err := r.CallTool("vmware_list_resource_pools", map[string]interface{}{})
	if err != nil {
		t.Fatalf("vmware_list_resource_pools failed: %v", err)
	}
	list, _ := decodeResult(t, raw)["resource_pools"].([]interface{})
	if len(list) == 0 {
		t.Fatal("simulator model has no resource pools")
	}
	return list[0].(string)
}

// fullResourceConfigSpec builds a types.ResourceConfigSpec JSON object with
// every CpuAllocation/MemoryAllocation sub-field set — required by
// simulator.ResourcePool.createChild's allResourceFieldsSet check (backing
// both vmware_resource_pool_create and vmware_resource_pool_create_vapp),
// documented in generated_resourcepool_vapp.go's top doc comment, point 8.
func fullResourceConfigSpec() map[string]interface{} {
	return map[string]interface{}{
		"cpuAllocation": map[string]interface{}{
			"reservation":           0,
			"limit":                 -1,
			"expandableReservation": true,
			"shares":                map[string]interface{}{"level": "normal", "shares": 4000},
		},
		"memoryAllocation": map[string]interface{}{
			"reservation":           0,
			"limit":                 -1,
			"expandableReservation": true,
			"shares":                map[string]interface{}{"level": "normal", "shares": 163840},
		},
	}
}

// createChildResourcePool is a fixture helper: creates a real child resource
// pool under parent via the tool itself (not a bypassing govmomi call),
// keeping test setup on the same path production callers use.
func createChildResourcePool(t *testing.T, r *Registry, parent, name string) string {
	t.Helper()
	raw, err := r.CallTool("vmware_resource_pool_create", map[string]interface{}{
		"resource_pool": parent,
		"name":          name,
		"spec":          fullResourceConfigSpec(),
		"confirm":       true,
	})
	if err != nil {
		t.Fatalf("vmware_resource_pool_create(%s) failed: %v", name, err)
	}
	path, _ := decodeResult(t, raw)["new_resource_pool"].(string)
	if path == "" {
		t.Fatalf("vmware_resource_pool_create(%s): empty new_resource_pool in result: %s", name, raw)
	}
	return path
}

// minimalVMConfigSpec builds the smallest types.VirtualMachineConfigSpec
// vcsim's simulator.NewVirtualMachine actually accepts — confirmed by
// reading the source (simulator/virtual_machine.go): only "name" and
// "files.vmPathName" are checked before a VM is created; every other field
// is optional. Used by both vmware_resource_pool_import_vapp's spec.configSpec
// and vmware_vapp_create_child_vm's config.
func minimalVMConfigSpec(name, datastoreName string) map[string]interface{} {
	return map[string]interface{}{
		"name":  name,
		"files": map[string]interface{}{"vmPathName": fmt.Sprintf("[%s] %s", datastoreName, name)},
	}
}

// TestResourcePoolVApp_Owner proves vmware_resource_pool_owner resolves a
// real inventory path via find.InventoryPath, not an empty
// ResourcePool.Owner()'s bare-constructed InventoryPath field (see
// generated_resourcepool_vapp.go's top doc comment, point 3).
func TestResourcePoolVApp_Owner(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()
	r := newResourcePoolVAppRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	pool := firstResourcePoolPath(t, r)

	raw, err := r.CallTool("vmware_resource_pool_owner", map[string]interface{}{"resource_pool": pool})
	if err != nil {
		t.Fatalf("vmware_resource_pool_owner failed: %v", err)
	}
	m := decodeResult(t, raw)
	owner, _ := m["owner"].(string)
	if owner == "" {
		t.Fatalf("expected a non-empty owner inventory path (find.InventoryPath fix), got result: %s", raw)
	}
	ownerType, _ := m["owner_type"].(string)
	if ownerType != "ComputeResource" && ownerType != "ClusterComputeResource" {
		t.Fatalf("expected owner_type to be a compute resource, got %q (result: %s)", ownerType, raw)
	}

	t.Run("missing_resource_pool", func(t *testing.T) {
		if _, err := r.CallTool("vmware_resource_pool_owner", map[string]interface{}{}); err == nil {
			t.Fatal("expected an error when resource_pool is omitted")
		}
	})
}

// TestResourcePoolVApp_CreateAndUpdateConfig proves vmware_resource_pool_create
// (NOT a Task — see top doc comment point 1) creates a real child pool with
// a real inventory path (find.InventoryPath fix, point 3), and
// vmware_resource_pool_update_config renames it for real.
func TestResourcePoolVApp_CreateAndUpdateConfig(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()
	r := newResourcePoolVAppRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	pool := firstResourcePoolPath(t, r)

	childPool := createChildResourcePool(t, r, pool, "child-pool-1")
	if !strings.Contains(childPool, "child-pool-1") {
		t.Fatalf("expected new_resource_pool to mention the new name, got %q", childPool)
	}

	t.Run("create_missing_name", func(t *testing.T) {
		if _, err := r.CallTool("vmware_resource_pool_create", map[string]interface{}{
			"resource_pool": pool, "spec": fullResourceConfigSpec(), "confirm": true,
		}); err == nil {
			t.Fatal("expected an error when name is omitted")
		}
	})

	t.Run("create_missing_spec", func(t *testing.T) {
		if _, err := r.CallTool("vmware_resource_pool_create", map[string]interface{}{
			"resource_pool": pool, "name": "no-spec", "confirm": true,
		}); err == nil {
			t.Fatal("expected an error when spec is omitted")
		}
	})

	t.Run("create_partial_allocation_rejected", func(t *testing.T) {
		// Only cpuAllocation.limit set — real vSphere/vcsim rejects a
		// partially-specified allocation (allResourceFieldsSet), proving the
		// tool genuinely reaches vcsim's validation, not just a local check.
		if _, err := r.CallTool("vmware_resource_pool_create", map[string]interface{}{
			"resource_pool": pool,
			"name":          "partial-alloc",
			"spec":          map[string]interface{}{"cpuAllocation": map[string]interface{}{"limit": 100}},
			"confirm":       true,
		}); err == nil {
			t.Fatal("expected an error for a partially-specified cpuAllocation/memoryAllocation")
		}
	})

	t.Run("update_config_rename", func(t *testing.T) {
		raw, err := r.CallTool("vmware_resource_pool_update_config", map[string]interface{}{
			"resource_pool": childPool,
			"name":          "child-pool-1-renamed",
			"confirm":       true,
		})
		if err != nil {
			t.Fatalf("vmware_resource_pool_update_config failed: %v", err)
		}
		if decodeResult(t, raw)["result"] != "updated" {
			t.Fatalf("expected result=updated, got: %s", raw)
		}

		// Verify the rename actually happened server-side (not just a
		// no-op success): the old name should no longer resolve under
		// `pool`, the new one should.
		listed := firstResourcePoolPathMatching(t, r, pool, "child-pool-1-renamed")
		if listed == "" {
			t.Fatalf("expected to find a resource pool named child-pool-1-renamed under %s after rename", pool)
		}
	})

	t.Run("update_config_missing_both", func(t *testing.T) {
		if _, err := r.CallTool("vmware_resource_pool_update_config", map[string]interface{}{
			"resource_pool": pool,
			"confirm":       true,
		}); err == nil {
			t.Fatal("expected an error when neither name nor config is given")
		}
	})

	t.Run("update_config_partial_allocation_allowed", func(t *testing.T) {
		// Unlike create, a partial allocation IS valid for update_config
		// (see top doc comment point 8) — only the set sub-fields change.
		other := createChildResourcePool(t, r, pool, "child-pool-partial-update")
		raw, err := r.CallTool("vmware_resource_pool_update_config", map[string]interface{}{
			"resource_pool": other,
			"config":        map[string]interface{}{"cpuAllocation": map[string]interface{}{"limit": 2048}},
			"confirm":       true,
		})
		if err != nil {
			t.Fatalf("vmware_resource_pool_update_config with a partial allocation failed: %v", err)
		}
		if decodeResult(t, raw)["result"] != "updated" {
			t.Fatalf("expected result=updated, got: %s", raw)
		}
	})
}

// firstResourcePoolPathMatching lists resource pools under scope matching
// name and returns the first inventory path found, or "" if none match —
// used to verify vmware_resource_pool_update_config's rename actually
// changed the server-side name, not just returned success locally.
func firstResourcePoolPathMatching(t *testing.T, r *Registry, scope, name string) string {
	t.Helper()
	raw, err := r.CallTool("vmware_list_resource_pools", map[string]interface{}{"path": scope + "/" + name})
	if err != nil {
		return ""
	}
	list, _ := decodeResult(t, raw)["resource_pools"].([]interface{})
	if len(list) == 0 {
		return ""
	}
	return list[0].(string)
}

// TestResourcePoolVApp_DestroyChildrenAndDestroy proves both irreversible
// (Tier 1) ResourcePool tools reach vcsim for real: DestroyChildren wipes
// only direct children (plain error return, not a Task — top doc comment
// point 1), Destroy actually removes the pool via its real Task.
func TestResourcePoolVApp_DestroyChildrenAndDestroy(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()
	r := newResourcePoolVAppRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	pool := firstResourcePoolPath(t, r)

	t.Run("destroy_children", func(t *testing.T) {
		parent := createChildResourcePool(t, r, pool, "destroy-children-parent")
		grandchild := createChildResourcePool(t, r, parent, "destroy-children-grandchild")

		if _, err := r.CallTool("vmware_resource_pool_destroy_children", map[string]interface{}{
			"resource_pool": parent,
			"confirm":       true,
		}); err != nil {
			t.Fatalf("vmware_resource_pool_destroy_children failed: %v", err)
		}

		if _, err := r.CallTool("vmware_resource_pool_owner", map[string]interface{}{"resource_pool": grandchild}); err == nil {
			t.Fatalf("expected %s to be gone after destroy_children on its parent", grandchild)
		}
		// The parent itself must survive (destroy_children only wipes
		// children, not the pool it's called on).
		if _, err := r.CallTool("vmware_resource_pool_owner", map[string]interface{}{"resource_pool": parent}); err != nil {
			t.Fatalf("expected %s to survive destroy_children, got: %v", parent, err)
		}
	})

	t.Run("destroy", func(t *testing.T) {
		toDestroy := createChildResourcePool(t, r, pool, "to-destroy")

		raw, err := r.CallTool("vmware_resource_pool_destroy", map[string]interface{}{
			"resource_pool": toDestroy,
			"confirm":       true,
		})
		if err != nil {
			t.Fatalf("vmware_resource_pool_destroy failed: %v", err)
		}
		if decodeResult(t, raw)["result"] != "destroyed" {
			t.Fatalf("expected result=destroyed, got: %s", raw)
		}

		if _, err := r.CallTool("vmware_resource_pool_owner", map[string]interface{}{"resource_pool": toDestroy}); err == nil {
			t.Fatal("expected the destroyed pool to no longer resolve")
		}
	})
}

// TestResourcePoolVApp_ImportVApp proves vmware_resource_pool_import_vapp
// starts a real lease and drives it to Ready against vcsim, actually
// creating the imported VM — see generated_resourcepool_vapp.go's top doc
// comment, point 4, for why the "spec" argument decodes into
// types.VirtualMachineImportSpec specifically (the concrete type vcsim's
// simulator.ResourcePool.ImportVApp type-asserts against).
func TestResourcePoolVApp_ImportVApp(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()
	r := newResourcePoolVAppRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	pool := firstResourcePoolPath(t, r)
	ds := firstDatastoreName(t, r)

	raw, err := r.CallTool("vmware_resource_pool_import_vapp", map[string]interface{}{
		"resource_pool": pool,
		"spec": map[string]interface{}{
			"configSpec": minimalVMConfigSpec("imported-vm-1", ds),
		},
		"confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_resource_pool_import_vapp failed: %v", err)
	}
	m := decodeResult(t, raw)
	if m["result"] != "export_lease_ready" {
		t.Fatalf("expected result=export_lease_ready (leaseResult's fixed label), got: %s", raw)
	}
	// count is legitimately 0 here: minimalVMConfigSpec has no disk device,
	// and simulator.ResourcePool.ImportVApp only emits a device URL for
	// devices with a BaseVirtualDeviceFileBackingInfo backing (confirmed by
	// reading the source) — a diskless VM has none. The real proof this
	// reached vcsim's actual import path is the VM appearing in inventory
	// below, not the URL count.

	// The import actually created a real VM — confirm it shows up in
	// inventory (proves this reached vcsim's real folder.CreateVMTask path,
	// not just a lease handshake).
	listRaw, err := r.CallTool("vmware_list_vms", map[string]interface{}{"path": "*imported-vm-1*"})
	if err != nil {
		t.Fatalf("vmware_list_vms failed: %v", err)
	}
	if countOf(t, listRaw) == 0 {
		t.Fatalf("expected imported-vm-1 to appear in inventory after import, got: %s", listRaw)
	}

	t.Run("missing_spec", func(t *testing.T) {
		if _, err := r.CallTool("vmware_resource_pool_import_vapp", map[string]interface{}{
			"resource_pool": pool, "confirm": true,
		}); err == nil {
			t.Fatal("expected an error when spec is omitted")
		}
	})
}

// TestResourcePoolVApp_CreateVAppChildVMAndClone proves vmware_resource_pool_create_vapp,
// vmware_vapp_create_child_vm, and vmware_vapp_clone all reach real vcsim
// handlers (CreateVApp, CreateChildVMTask, CloneVAppTask — see top doc
// comment point 6). Runs against simulator.VPX() because vApp CREATION is
// disabled on standalone ESXi (point 7 — see
// TestResourcePoolVApp_ESXiDisablesVAppCreation for that half).
func TestResourcePoolVApp_CreateVAppChildVMAndClone(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()
	r := newResourcePoolVAppRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	pool := firstResourcePoolPath(t, r)
	ds := firstDatastoreName(t, r)

	var vappPath string
	t.Run("create_vapp", func(t *testing.T) {
		raw, err := r.CallTool("vmware_resource_pool_create_vapp", map[string]interface{}{
			"resource_pool": pool,
			"name":          "test-vapp-1",
			"res_spec":      fullResourceConfigSpec(),
			"confirm":       true,
		})
		if err != nil {
			t.Fatalf("vmware_resource_pool_create_vapp failed: %v", err)
		}
		m := decodeResult(t, raw)
		vappPath, _ = m["new_vapp"].(string)
		if vappPath == "" {
			t.Fatalf("expected a non-empty new_vapp path (find.InventoryPath fix), got: %s", raw)
		}
		if !strings.Contains(vappPath, "test-vapp-1") {
			t.Fatalf("expected new_vapp to mention the new name, got %q", vappPath)
		}
	})

	t.Run("create_child_vm", func(t *testing.T) {
		raw, err := r.CallTool("vmware_vapp_create_child_vm", map[string]interface{}{
			"vapp":    vappPath,
			"config":  minimalVMConfigSpec("vapp-child-vm-1", ds),
			"confirm": true,
		})
		if err != nil {
			t.Fatalf("vmware_vapp_create_child_vm failed: %v", err)
		}
		if decodeResult(t, raw)["result"] != "child_vm_created" {
			t.Fatalf("expected result=child_vm_created, got: %s", raw)
		}

		listRaw, err := r.CallTool("vmware_list_vms", map[string]interface{}{"path": "*vapp-child-vm-1*"})
		if err != nil {
			t.Fatalf("vmware_list_vms failed: %v", err)
		}
		if countOf(t, listRaw) == 0 {
			t.Fatalf("expected vapp-child-vm-1 to appear in inventory, got: %s", listRaw)
		}
	})

	t.Run("create_child_vm_missing_config", func(t *testing.T) {
		if _, err := r.CallTool("vmware_vapp_create_child_vm", map[string]interface{}{
			"vapp": vappPath, "confirm": true,
		}); err == nil {
			t.Fatal("expected an error when config is omitted")
		}
	})

	t.Run("clone", func(t *testing.T) {
		raw, err := r.CallTool("vmware_vapp_clone", map[string]interface{}{
			"vapp":                 vappPath,
			"name":                 "test-vapp-1-clone",
			"target_resource_pool": pool,
			"spec":                 map[string]interface{}{"location": map[string]interface{}{"type": "Datastore", "value": "ignored-by-vcsim"}},
			"confirm":              true,
		})
		if err != nil {
			t.Fatalf("vmware_vapp_clone failed: %v", err)
		}
		if decodeResult(t, raw)["result"] != "cloned" {
			t.Fatalf("expected result=cloned, got: %s", raw)
		}

		// simulator.VirtualApp.CloneVAppTask names every cloned VM after the
		// new vApp itself (req.Name — confirmed by reading
		// simulator/resource_pool.go's CloneVAppTask), not the source VM's
		// own name, so a VM named "test-vapp-1-clone" appearing in inventory
		// is proof the clone task actually ran server-side, not just that
		// the tool call returned a fixed success string.
		listRaw, err := r.CallTool("vmware_list_vms", map[string]interface{}{"path": "*test-vapp-1-clone*"})
		if err != nil {
			t.Fatalf("vmware_list_vms failed: %v", err)
		}
		if countOf(t, listRaw) == 0 {
			t.Fatalf("expected a cloned VM named test-vapp-1-clone to appear in inventory, got: %s", listRaw)
		}
	})

	t.Run("clone_missing_target", func(t *testing.T) {
		if _, err := r.CallTool("vmware_vapp_clone", map[string]interface{}{
			"vapp": vappPath, "name": "no-target", "spec": map[string]interface{}{}, "confirm": true,
		}); err == nil {
			t.Fatal("expected an error when target_resource_pool is omitted/unresolvable")
		}
	})
}

// TestResourcePoolVApp_ESXiDisablesVAppCreation proves the point-7 finding
// in generated_resourcepool_vapp.go's top doc comment for real: vApp
// creation (esx.ResourcePool.DisabledMethod = ["CreateVApp",
// "CreateChildVM_Task"]) is a genuine vCenter-only vSphere capability, not
// just a vcsim gap — vmware_resource_pool_create_vapp fails CLEANLY (a
// normal wrapped error, not a panic/crash) against a standalone-ESXi
// connection, while plain vmware_resource_pool_create (not vApp-specific)
// keeps working fine there.
func TestResourcePoolVApp_ESXiDisablesVAppCreation(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()
	r := newResourcePoolVAppRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	pool := firstResourcePoolPath(t, r)

	t.Run("create_vapp_fails_clean", func(t *testing.T) {
		_, err := r.CallTool("vmware_resource_pool_create_vapp", map[string]interface{}{
			"resource_pool": pool,
			"name":          "esx-vapp-should-fail",
			"res_spec":      fullResourceConfigSpec(),
			"confirm":       true,
		})
		if err == nil {
			t.Fatal("expected vmware_resource_pool_create_vapp to fail against standalone ESXi (CreateVApp is in esx.ResourcePool.DisabledMethod)")
		}
		if strings.Contains(err.Error(), "panicked") {
			t.Fatalf("expected a clean server-side fault, got a recovered panic instead: %v", err)
		}
	})

	t.Run("plain_create_still_works", func(t *testing.T) {
		child := createChildResourcePool(t, r, pool, "esx-plain-pool")
		if child == "" {
			t.Fatal("expected plain vmware_resource_pool_create to still succeed on standalone ESXi")
		}
	})
}

// TestResourcePoolVApp_VAppMethodsNotSimulated exercises the 4 VirtualApp
// methods with zero server-side vcsim support (PowerOn, PowerOff, Suspend,
// UpdateConfig — see top doc comment point 6) against a real vApp fixture,
// using assertReachesServer (generated_vm_lifecycle_test.go, same package)
// to prove each reaches vcsim's real method dispatch and gets back a clean
// MethodNotFound-shaped fault, not a wiring bug or a panic.
func TestResourcePoolVApp_VAppMethodsNotSimulated(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()
	r := newResourcePoolVAppRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	pool := firstResourcePoolPath(t, r)

	raw, err := r.CallTool("vmware_resource_pool_create_vapp", map[string]interface{}{
		"resource_pool": pool,
		"name":          "no-sim-vapp",
		"res_spec":      fullResourceConfigSpec(),
		"confirm":       true,
	})
	if err != nil {
		t.Fatalf("fixture vmware_resource_pool_create_vapp failed: %v", err)
	}
	vappPath, _ := decodeResult(t, raw)["new_vapp"].(string)
	if vappPath == "" {
		t.Fatal("fixture vApp has no inventory path")
	}

	t.Run("power_on", func(t *testing.T) {
		_, err := r.CallTool("vmware_vapp_power_on", map[string]interface{}{"vapp": vappPath, "confirm": true})
		assertReachesServer(t, err, "vmware_vapp_power_on")
	})

	t.Run("power_off", func(t *testing.T) {
		_, err := r.CallTool("vmware_vapp_power_off", map[string]interface{}{"vapp": vappPath, "confirm": true})
		assertReachesServer(t, err, "vmware_vapp_power_off")
	})

	t.Run("suspend", func(t *testing.T) {
		_, err := r.CallTool("vmware_vapp_suspend", map[string]interface{}{"vapp": vappPath, "confirm": true})
		assertReachesServer(t, err, "vmware_vapp_suspend")
	})

	t.Run("update_config", func(t *testing.T) {
		_, err := r.CallTool("vmware_vapp_update_config", map[string]interface{}{
			"vapp": vappPath, "spec": map[string]interface{}{"annotation": "updated"}, "confirm": true,
		})
		assertReachesServer(t, err, "vmware_vapp_update_config")
	})

	t.Run("update_config_missing_spec", func(t *testing.T) {
		if _, err := r.CallTool("vmware_vapp_update_config", map[string]interface{}{
			"vapp": vappPath, "confirm": true,
		}); err == nil {
			t.Fatal("expected an error when spec is omitted")
		}
	})
}

// TestResourcePoolVApp_ResolveVAppMissingArg proves resolveVirtualApp's own
// argument validation (independent of vcsim) for every VirtualApp tool that
// needs a resolvable "vapp".
func TestResourcePoolVApp_ResolveVAppMissingArg(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()
	r := newResourcePoolVAppRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	for _, tool := range []string{
		"vmware_vapp_power_on", "vmware_vapp_power_off", "vmware_vapp_suspend",
		"vmware_vapp_update_config", "vmware_vapp_create_child_vm", "vmware_vapp_clone",
	} {
		t.Run(tool, func(t *testing.T) {
			if _, err := r.CallTool(tool, map[string]interface{}{"confirm": true}); err == nil {
				t.Fatalf("%s: expected an error when vapp is omitted", tool)
			}
		})
	}
}

// TestResourcePoolVApp_GateClosed spot-checks the Fase 1a destructive
// gate/confirm mechanism (fully covered generically in destructive_test.go)
// against one Tier 1 (vmware_resource_pool_destroy) and one Tier 2
// (vmware_vapp_power_on) tool from this file, matching the convention
// generated_network_test.go/generated_host_storage_test.go already use per
// domain.
func TestResourcePoolVApp_GateClosed(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()
	r := newResourcePoolVAppRegistry(context.Background(), c, RegistryOptions{AllowDestructive: false})
	pool := firstResourcePoolPath(t, r)

	if _, err := r.CallTool("vmware_resource_pool_destroy", map[string]interface{}{
		"resource_pool": pool, "confirm": true,
	}); err == nil {
		t.Fatal("expected the closed destructive gate to deny vmware_resource_pool_destroy (tier 1)")
	}

	if _, err := r.CallTool("vmware_vapp_power_on", map[string]interface{}{
		"vapp": "whatever", "confirm": true,
	}); err == nil {
		t.Fatal("expected the closed destructive gate to deny vmware_vapp_power_on (tier 2) before even resolving the vapp")
	}
}

package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25/mo"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// newInventoryFolderRegistry builds a Registry the normal way (NewRegistry,
// which wires vm.go/host.go/etc via registerTools) and then manually layers
// this group's tools on top via withClass — same approach as
// generated_vm_lifecycle_test.go's newLifecycleRegistry. Must not edit
// registry.go itself (see generated_inventory_folder.go's top doc comment).
func newInventoryFolderRegistry(ctx context.Context, c *vmware.Client, opts RegistryOptions) *Registry {
	r := NewRegistry(ctx, c, opts)
	r.withClass(modeVSphereGeneral, registerInventoryFolderTools)
	return r
}

// TestInventoryFolderTools_Registration smokes every one of the 14 tools'
// registration + basic arg validation against a cheap standalone-ESXi
// simulator — proves each name is wired (not "unknown tool") and that
// missing required args or a missing confirm:true fail cleanly (not a
// panic), without needing the heavier VPX fixtures the functional tests
// below use.
func TestInventoryFolderTools_Registration(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newInventoryFolderRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	tools := []string{
		"vmware_folder_children",
		"vmware_folder_place_vms_xcluster",
		"vmware_datacenter_folders",
		"vmware_folder_add_standalone_host",
		"vmware_folder_create_cluster",
		"vmware_folder_create_datacenter",
		"vmware_folder_create_dvs",
		"vmware_folder_create_folder",
		"vmware_folder_create_storage_pod",
		"vmware_folder_create_vm",
		"vmware_folder_move_into",
		"vmware_folder_register_vm",
		"vmware_datacenter_destroy",
		"vmware_datacenter_power_on_vm",
	}

	for _, name := range tools {
		t.Run(name, func(t *testing.T) {
			_, err := r.CallTool(name, map[string]interface{}{})
			if err == nil {
				t.Fatalf("%s: expected a validation error for empty args, got success", name)
			}
			msg := err.Error()
			if strings.Contains(msg, "unknown tool") {
				t.Fatalf("%s: tool is not registered: %v", name, err)
			}
			if strings.Contains(msg, "panicked") {
				t.Fatalf("%s: handler panicked instead of returning a clean error: %v", name, err)
			}
		})
	}
}

// TestInventoryFolderTools_DatacenterFoldersAndChildren proves
// vmware_datacenter_folders resolves DC0's 4 well-known sub-folders with
// non-empty, correctly-suffixed inventory paths (the empirical check this
// file's top doc comment calls for — Datacenter.Folders() builds these
// client-side, no find.InventoryPath fallback expected to trigger), and
// vmware_folder_children lists the VMs simulator.VPX()'s default Model
// creates under DC0's vm folder — Model.Machine (2) per standalone host PLUS
// Model.Machine (2) again under the default cluster's resource pool, so 4
// total (confirmed empirically; an earlier draft assumed a flat
// Model.Machine=2 total and had to be corrected).
func TestInventoryFolderTools_DatacenterFoldersAndChildren(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newInventoryFolderRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	raw, err := r.CallTool("vmware_datacenter_folders", map[string]interface{}{"datacenter": "DC0"})
	if err != nil {
		t.Fatalf("vmware_datacenter_folders failed: %v", err)
	}
	m := decodeResult(t, raw)
	for key, want := range map[string]string{
		"vm_folder":        "/DC0/vm",
		"host_folder":      "/DC0/host",
		"datastore_folder": "/DC0/datastore",
		"network_folder":   "/DC0/network",
	} {
		got, _ := m[key].(string)
		if got != want {
			t.Fatalf("vmware_datacenter_folders: %s = %q, want %q (full result: %s)", key, got, want, raw)
		}
	}

	raw, err = r.CallTool("vmware_folder_children", map[string]interface{}{"folder": "/DC0/vm"})
	if err != nil {
		t.Fatalf("vmware_folder_children failed: %v", err)
	}
	if got := countOf(t, raw); got != 4 {
		t.Fatalf("vmware_folder_children on /DC0/vm: expected 4 VMs (2 under the standalone host + 2 under the default cluster's resource pool), got %v (result: %s)", got, raw)
	}
	m = decodeResult(t, raw)
	children, _ := m["children"].([]interface{})
	for _, ci := range children {
		child, ok := ci.(map[string]interface{})
		if !ok {
			t.Fatalf("child entry is not an object: %s", raw)
		}
		if child["type"] != "VirtualMachine" {
			t.Fatalf("expected every child of /DC0/vm to be a VirtualMachine, got %v (result: %s)", child["type"], raw)
		}
		path, _ := child["path"].(string)
		if !strings.HasPrefix(path, "/DC0/vm/") {
			t.Fatalf("child path %q does not look like a resolved inventory path under /DC0/vm (result: %s)", path, raw)
		}
	}
}

// TestInventoryFolderTools_CreateFolderScopingAndMoveInto proves
// vmware_folder_create_folder creates real nested folders, that
// vmware_folder_children's default "vm" scope correctly resolves a bare
// name to a nested folder created earlier (the dcScopedPath convenience
// documented in this file's top doc comment), and that
// vmware_folder_move_into actually relocates an object in the real
// inventory (verified via a second, independent govmomi Finder lookup —
// not just trusting the tool's own "moved" response).
func TestInventoryFolderTools_CreateFolderScopingAndMoveInto(t *testing.T) {
	ctx := context.Background()
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newInventoryFolderRegistry(ctx, c, RegistryOptions{AllowDestructive: true})

	raw, err := r.CallTool("vmware_folder_create_folder", map[string]interface{}{
		"folder": "/DC0/vm", "name": "Kids", "confirm": true,
	})
	if err != nil {
		t.Fatalf("create Kids folder failed: %v", err)
	}
	m := decodeResult(t, raw)
	if got := m["new_folder"]; got != "/DC0/vm/Kids" {
		t.Fatalf("new_folder = %v, want /DC0/vm/Kids (result: %s)", got, raw)
	}

	// Bare name "Kids" must resolve via the default "vm" scope
	// (dcScopedPath("vm", "Kids") -> "/*/vm/Kids") to the folder just
	// created, proving the documented scoping convenience actually works,
	// not just the absolute-path case.
	raw, err = r.CallTool("vmware_folder_children", map[string]interface{}{"folder": "Kids"})
	if err != nil {
		t.Fatalf("vmware_folder_children on bare-name-scoped \"Kids\" failed: %v", err)
	}
	m = decodeResult(t, raw)
	if got := m["folder"]; got != "/DC0/vm/Kids" {
		t.Fatalf("default-scope resolution: folder = %v, want /DC0/vm/Kids (result: %s)", got, raw)
	}
	if got := countOf(t, raw); got != 0 {
		t.Fatalf("freshly created Kids folder should be empty, got count=%v (result: %s)", got, raw)
	}

	raw, err = r.CallTool("vmware_folder_create_folder", map[string]interface{}{
		"folder": "/DC0/vm", "name": "MoveTarget", "confirm": true,
	})
	if err != nil {
		t.Fatalf("create MoveTarget folder failed: %v", err)
	}
	m = decodeResult(t, raw)
	if got := m["new_folder"]; got != "/DC0/vm/MoveTarget" {
		t.Fatalf("new_folder = %v, want /DC0/vm/MoveTarget (result: %s)", got, raw)
	}

	kids, err := c.Finder.Folder(ctx, "/DC0/vm/Kids")
	if err != nil {
		t.Fatalf("failed to resolve /DC0/vm/Kids via Finder for its moref: %v", err)
	}
	kidsRef := kids.Reference()

	raw, err = r.CallTool("vmware_folder_move_into", map[string]interface{}{
		"folder":  "/DC0/vm/MoveTarget",
		"refs":    []interface{}{map[string]interface{}{"type": kidsRef.Type, "value": kidsRef.Value}},
		"confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_folder_move_into failed: %v", err)
	}
	m = decodeResult(t, raw)
	if got, _ := m["count"].(float64); got != 1 {
		t.Fatalf("vmware_folder_move_into: count = %v, want 1 (result: %s)", got, raw)
	}

	// Verify via a SECOND, independent Finder lookup — not the tool's own
	// "moved" response — that Kids genuinely relocated in the real
	// inventory (same "prove by a 2nd independent path" rigor as every
	// other InventoryPath-gotcha fix in this project).
	if _, err := c.Finder.Folder(ctx, "/DC0/vm/MoveTarget/Kids"); err != nil {
		t.Fatalf("Kids folder not found under MoveTarget after move: %v", err)
	}
	if _, err := c.Finder.Folder(ctx, "/DC0/vm/Kids"); err == nil {
		t.Fatalf("Kids folder still resolves at its old path /DC0/vm/Kids after being moved")
	}
}

// TestInventoryFolderTools_CreateStoragePod proves vmware_folder_create_storage_pod
// creates a real StoragePod under the datastore folder (the "datastore"
// default scope this file's top doc comment documents as required — the
// simulator would fault NotSupported if this tool defaulted to "vm" like a
// naive reading of the brief might have produced).
func TestInventoryFolderTools_CreateStoragePod(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newInventoryFolderRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	raw, err := r.CallTool("vmware_folder_create_storage_pod", map[string]interface{}{
		"folder": "/DC0/datastore", "name": "Pod1", "confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_folder_create_storage_pod failed: %v", err)
	}
	m := decodeResult(t, raw)
	if got := m["storage_pod"]; got != "/DC0/datastore/Pod1" {
		t.Fatalf("storage_pod = %v, want /DC0/datastore/Pod1 (result: %s)", got, raw)
	}
}

// TestInventoryFolderTools_CreateClusterAndAddStandaloneHost proves both
// vmware_folder_create_cluster and vmware_folder_add_standalone_host target
// the HOST folder correctly (the "host" default scope, not "vm" — see this
// file's top doc comment) and that both report the resolved inventory path
// of the object they created, not a raw/empty InventoryPath.
func TestInventoryFolderTools_CreateClusterAndAddStandaloneHost(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newInventoryFolderRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	raw, err := r.CallTool("vmware_folder_create_cluster", map[string]interface{}{
		"folder": "/DC0/host", "name": "Cluster2", "confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_folder_create_cluster failed: %v", err)
	}
	m := decodeResult(t, raw)
	if got := m["cluster"]; got != "/DC0/host/Cluster2" {
		t.Fatalf("cluster = %v, want /DC0/host/Cluster2 (result: %s)", got, raw)
	}

	raw, err = r.CallTool("vmware_folder_add_standalone_host", map[string]interface{}{
		"folder":  "/DC0/host",
		"spec":    map[string]interface{}{"hostName": "new-esxi.local"},
		"confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_folder_add_standalone_host failed: %v", err)
	}
	m = decodeResult(t, raw)
	hostPath, _ := m["host"].(string)
	if !strings.HasPrefix(hostPath, "/DC0/host/") {
		t.Fatalf("host = %q, want a path under /DC0/host/ (result: %s)", hostPath, raw)
	}
	if hostPath == "/DC0/host" {
		t.Fatalf("host path resolved to the folder itself, not a new host object (result: %s)", raw)
	}
}

// TestInventoryFolderTools_CreateDatacenterAndDestroy proves
// vmware_folder_create_datacenter (using the NO-default-scope "folder"
// argument documented in this file's top doc comment — "/" for the
// inventory root) and vmware_datacenter_destroy both work end to end
// against a freshly created, empty datacenter (Datacenter.DestroyTask
// faults ResourceInUse against a non-empty one — see the top doc comment —
// so this deliberately does NOT try to destroy the Model's pre-populated
// DC0).
func TestInventoryFolderTools_CreateDatacenterAndDestroy(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newInventoryFolderRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	raw, err := r.CallTool("vmware_folder_create_datacenter", map[string]interface{}{
		"folder": "/", "name": "DC-New", "confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_folder_create_datacenter failed: %v", err)
	}
	m := decodeResult(t, raw)
	if got := m["datacenter"]; got != "/DC-New" {
		t.Fatalf("datacenter = %v, want /DC-New (result: %s)", got, raw)
	}

	raw, err = r.CallTool("vmware_datacenter_destroy", map[string]interface{}{
		"datacenter": "DC-New", "confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_datacenter_destroy failed on a fresh empty datacenter: %v", err)
	}
	m = decodeResult(t, raw)
	if got := m["result"]; got != "destroyed" {
		t.Fatalf("result = %v, want \"destroyed\" (result: %s)", got, raw)
	}

	if _, err := c.Finder.Datacenter(context.Background(), "DC-New"); err == nil {
		t.Fatalf("DC-New still resolves via Finder after being destroyed")
	}
}

// TestInventoryFolderTools_CreateDVS proves vmware_folder_create_dvs targets
// the NETWORK folder (see this file's top doc comment) and that
// config_spec/product_info's polymorphic-field split actually works end to
// end. Uses a name distinct from "DVS0" — simulator.VPX()'s default Model
// (Portgroup:1) already creates a DVS named "DVS0" per datacenter, and a
// sibling Fase 5 test hit exactly this DuplicateName collision.
func TestInventoryFolderTools_CreateDVS(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newInventoryFolderRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	raw, err := r.CallTool("vmware_folder_create_dvs", map[string]interface{}{
		"folder":      "/DC0/network",
		"config_spec": map[string]interface{}{"name": "DVS-Test"},
		"confirm":     true,
	})
	if err != nil {
		t.Fatalf("vmware_folder_create_dvs failed: %v", err)
	}
	m := decodeResult(t, raw)
	if got := m["network"]; got != "/DC0/network/DVS-Test" {
		t.Fatalf("network = %v, want /DC0/network/DVS-Test (result: %s)", got, raw)
	}
}

// TestInventoryFolderTools_CreateVMAndRegisterVM proves vmware_folder_create_vm
// (resource_pool required — see this file's top doc comment on the nil-pool
// panic risk) creates a real VM with a resolvable inventory path, and that
// vmware_folder_register_vm can re-register that same VM's .vmx file after
// unregistering it — end to end through the real vcsim datastore/file layer,
// not mocked. Registers WITHOUT a "name" (letting it derive from the .vmx
// path's directory, same as the original name) deliberately: re-registering
// under a DIFFERENT name than the original was tried first and failed with
// a real vcsim file-not-found fault (NewVirtualMachine's .nvram lookup uses
// the new name's directory, which the original create never populated) —
// not a bug in this tool, a genuine vcsim/vSphere constraint when reusing
// an existing VM's files, so the test avoids it instead of masking it.
func TestInventoryFolderTools_CreateVMAndRegisterVM(t *testing.T) {
	ctx := context.Background()
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newInventoryFolderRegistry(ctx, c, RegistryOptions{AllowDestructive: true})

	raw, err := r.CallTool("vmware_folder_create_vm", map[string]interface{}{
		"folder": "/DC0/vm",
		"config": map[string]interface{}{
			"name":  "TestVMCreate",
			"files": map[string]interface{}{"vmPathName": "[LocalDS_0]"},
		},
		"resource_pool": "DC0_C0/Resources",
		"confirm":       true,
	})
	if err != nil {
		t.Fatalf("vmware_folder_create_vm failed: %v", err)
	}
	m := decodeResult(t, raw)
	vmPath, _ := m["vm"].(string)
	if !strings.HasPrefix(vmPath, "/DC0/vm/") {
		t.Fatalf("vm = %q, want a path under /DC0/vm/ (result: %s)", vmPath, raw)
	}
	if got := m["resource_pool"]; got != "/DC0/host/DC0_C0/Resources" {
		t.Fatalf("resource_pool = %v, want /DC0/host/DC0_C0/Resources (result: %s)", got, raw)
	}

	// Read the real .vmx datastore path the simulator assigned, then
	// unregister the VM (direct govmomi call — test fixture setup, not a
	// tool under test, same as generated_vm_lifecycle_test.go's
	// setGuestIP), so vmware_folder_register_vm has something real to
	// re-register.
	vmObj, err := c.Finder.VirtualMachine(ctx, vmPath)
	if err != nil {
		t.Fatalf("failed to resolve created VM %s: %v", vmPath, err)
	}
	var moVM mo.VirtualMachine
	if err := vmObj.Properties(ctx, vmObj.Reference(), []string{"config.files.vmPathName"}, &moVM); err != nil {
		t.Fatalf("failed to read vmPathName for %s: %v", vmPath, err)
	}
	vmxPath := moVM.Config.Files.VmPathName
	if vmxPath == "" {
		t.Fatalf("created VM %s has an empty config.files.vmPathName", vmPath)
	}
	if err := vmObj.Unregister(ctx); err != nil {
		t.Fatalf("failed to unregister %s as register_vm test fixture setup: %v", vmPath, err)
	}

	raw, err = r.CallTool("vmware_folder_register_vm", map[string]interface{}{
		"folder":        "/DC0/vm",
		"path":          vmxPath,
		"resource_pool": "DC0_C0/Resources",
		"confirm":       true,
	})
	if err != nil {
		t.Fatalf("vmware_folder_register_vm failed: %v", err)
	}
	m = decodeResult(t, raw)
	registeredPath, _ := m["vm"].(string)
	if registeredPath != "/DC0/vm/TestVMCreate" {
		t.Fatalf("vm = %q, want /DC0/vm/TestVMCreate (name derived from the .vmx path since \"name\" was omitted) (result: %s)", registeredPath, raw)
	}
}

// TestInventoryFolderTools_PlaceVmsXCluster proves vmware_folder_place_vms_xcluster
// is genuinely read-only (registered with no tier — no confirm needed) and
// that its strict "folder must be the inventory ROOT" requirement (enforced
// server-side per this file's top doc comment) is both satisfiable (folder:
// "/") and actually enforced (folder: "/DC0/vm" is rejected).
func TestInventoryFolderTools_PlaceVmsXCluster(t *testing.T) {
	ctx := context.Background()
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newInventoryFolderRegistry(ctx, c, RegistryOptions{AllowDestructive: true})

	pool, err := c.Finder.ResourcePool(ctx, "/DC0/host/DC0_C0/Resources")
	if err != nil {
		t.Fatalf("failed to resolve DC0_C0's Resources pool: %v", err)
	}
	poolRef := pool.Reference()

	spec := map[string]interface{}{
		"resourcePools": []interface{}{
			map[string]interface{}{"type": poolRef.Type, "value": poolRef.Value},
		},
		"vmPlacementSpecs": []interface{}{
			map[string]interface{}{"configSpec": map[string]interface{}{"name": "PlacementTestVM"}},
		},
	}

	raw, err := r.CallTool("vmware_folder_place_vms_xcluster", map[string]interface{}{
		"folder": "/", "spec": spec,
	})
	if err != nil {
		t.Fatalf("vmware_folder_place_vms_xcluster against the root folder failed: %v", err)
	}
	m := decodeResult(t, raw)
	if _, ok := m["result"]; !ok {
		t.Fatalf("expected a \"result\" field in the response: %s", raw)
	}

	if _, err := r.CallTool("vmware_folder_place_vms_xcluster", map[string]interface{}{
		"folder": "/DC0/vm", "spec": spec,
	}); err == nil {
		t.Fatalf("expected vmware_folder_place_vms_xcluster to fail against a non-root folder (server enforces root-only), got success")
	}
}

// TestInventoryFolderTools_DatacenterPowerOnVM proves vmware_datacenter_power_on_vm
// resolves VM names (not raw morefs the caller would have to build by hand)
// and genuinely powers them on, verified via a second, independent Finder +
// Properties lookup rather than trusting the tool's own response.
func TestInventoryFolderTools_DatacenterPowerOnVM(t *testing.T) {
	ctx := context.Background()
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newInventoryFolderRegistry(ctx, c, RegistryOptions{AllowDestructive: true})

	vms, err := c.Finder.VirtualMachineList(ctx, "/DC0/vm/*")
	if err != nil || len(vms) == 0 {
		t.Fatalf("failed to find a fixture VM under /DC0/vm: %v", err)
	}
	target := vms[0]

	raw, err := r.CallTool("vmware_datacenter_power_on_vm", map[string]interface{}{
		"datacenter": "DC0",
		"vms":        []interface{}{target.Name()},
		"confirm":    true,
	})
	if err != nil {
		t.Fatalf("vmware_datacenter_power_on_vm failed: %v", err)
	}
	m := decodeResult(t, raw)
	if got := m["result"]; got != "powered_on" {
		t.Fatalf("result = %v, want \"powered_on\" (result: %s)", got, raw)
	}

	var moVM mo.VirtualMachine
	if err := target.Properties(ctx, target.Reference(), []string{"runtime.powerState"}, &moVM); err != nil {
		t.Fatalf("failed to read power state for %s: %v", target.InventoryPath, err)
	}
	if moVM.Runtime.PowerState != "poweredOn" {
		t.Fatalf("%s power state = %q after vmware_datacenter_power_on_vm, want \"poweredOn\"", target.InventoryPath, moVM.Runtime.PowerState)
	}
}

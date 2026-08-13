package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vapi/library"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// newVCenterTemplateRegistry builds a Registry the normal way (NewRegistry)
// and then manually layers registerVCenterTemplateTools on top via
// withClass — same pattern as newTagsRegistry/newAuthorizationRegistry; this
// file must not edit registry.go (see generated_vcenter_template.go's top
// doc comment).
func newVCenterTemplateRegistry(ctx context.Context, c *vmware.Client, opts RegistryOptions) *Registry {
	r := NewRegistry(ctx, c, opts)
	r.withClass(modeVCenterOnly, registerVCenterTemplateTools)
	return r
}

// vmFolderPath returns the parent folder of an inventory path, e.g.
// "/DC0/vm" for "/DC0/vm/DC0_H0_VM0" — vcsim's libraryItemCreateTemplate/
// libraryItemTemplateID dereference Placement.Folder unconditionally (see
// generated_vcenter_template.go's top doc comment), so every test below that
// exercises CreateTemplate/DeployTemplateLibraryItem/CheckOut must supply
// one.
func vmFolderPath(p string) string {
	i := strings.LastIndex(p, "/")
	if i <= 0 {
		return "/"
	}
	return p[:i]
}

// vmDatacenterPath returns the datacenter inventory path a VM path lives
// under, e.g. "/DC0" for "/DC0/vm/DC0_H0_VM0" (two levels up: strip the VM
// folder segment too, not just the VM name).
func vmDatacenterPath(vmPath string) string {
	return vmFolderPath(vmFolderPath(vmPath))
}

// firstDatastoreInventoryPath returns a full inventory path (e.g.
// "/DC0/datastore/LocalDS_0") for the first datastore vmware_list_datastores
// reports — firstDatastorePath (generated_datastore_browser_test.go) only
// returns the bare datastore name, which resolveEntityRef's
// SearchIndex.FindByInventoryPath cannot resolve on its own (it needs an
// absolute path, unlike find.Finder's glob-friendly methods).
func firstDatastoreInventoryPath(t *testing.T, r *Registry, vmPath string) string {
	t.Helper()
	return vmDatacenterPath(vmPath) + "/datastore/" + firstDatastorePath(t, r)
}

// newTestLibrary creates a local Content Library backed by the given
// datastore path, using library.NewManager directly (bypassing this
// project's own tools layer — content library CRUD is a different Fase
// 8a group's scope, not this one's, per the orchestrator's brief) — and
// returns its ID plus a cleanup func.
func newTestLibrary(t *testing.T, ctx context.Context, c *vmware.Client, name, datastorePath string) (string, func()) {
	t.Helper()
	rc, err := c.REST(ctx)
	if err != nil {
		t.Fatalf("REST login failed: %v", err)
	}
	libM := library.NewManager(rc)

	dsRef, err := resolveEntityRef(ctx, c, datastorePath)
	if err != nil {
		t.Fatalf("resolve datastore %s failed: %v", datastorePath, err)
	}

	id, err := libM.CreateLibrary(ctx, library.Library{
		Name: name,
		Type: "LOCAL",
		Storage: []library.StorageBacking{{
			DatastoreID: dsRef.Value,
			Type:        "DATASTORE",
		}},
	})
	if err != nil {
		t.Fatalf("CreateLibrary(%s) failed: %v", name, err)
	}

	cleanup := func() {
		_ = libM.DeleteLibrary(ctx, &library.Library{ID: id, Type: "LOCAL"})
	}
	return id, cleanup
}

// TestVCenterTemplateTools_Registration proves all 10 tools are registered
// and reachable via ListTools.
func TestVCenterTemplateTools_Registration(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newVCenterTemplateRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	want := []string{
		"vmware_vcenter_check_in",
		"vmware_vcenter_check_out",
		"vmware_vcenter_create_template",
		"vmware_vcenter_deploy_template_library_item",
		"vmware_vcenter_get_library_template_info",
		"vmware_vcenter_sync_template_library",
		"vmware_vcenter_sync_template_library_item",
		"vmware_vcenter_create_ovf",
		"vmware_vcenter_deploy_library_item",
		"vmware_vcenter_filter_library_item",
	}
	if len(want) != 10 {
		t.Fatalf("test bug: want list has %d entries, expected 10", len(want))
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

// TestVCenterTemplateTools_TemplateLifecycle exercises CreateTemplate ->
// GetLibraryTemplateInfo -> DeployTemplateLibraryItem -> CheckOut -> CheckIn
// end to end against a real vcsim vcenter.Manager, and confirms the
// Placement.Folder requirement documented in this file's top doc comment
// (folder derived from the source VM's own inventory path).
func TestVCenterTemplateTools_TemplateLifecycle(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()
	ctx := context.Background()

	r := newVCenterTemplateRegistry(ctx, c, RegistryOptions{AllowDestructive: true})
	vm := firstVMPath(t, r)
	folder := vmFolderPath(vm)
	rp := firstResourcePoolPath(t, r)
	ds := firstDatastoreInventoryPath(t, r, vm)

	libID, libCleanup := newTestLibrary(t, ctx, c, "TemplateLifecycleLib", ds)
	defer libCleanup()

	raw, err := r.CallTool("vmware_vcenter_create_template", map[string]interface{}{
		"name":       "TestTemplate",
		"library_id": libID,
		"source_vm":  vm,
		"placement":  map[string]interface{}{"folder": folder, "resource_pool": rp},
		"confirm":    true,
	})
	if err != nil {
		t.Fatalf("vmware_vcenter_create_template failed: %v (%s)", err, raw)
	}
	itemID, _ := decodeResult(t, raw)["library_item_id"].(string)
	if itemID == "" {
		t.Fatalf("expected a non-empty library_item_id in %s", raw)
	}

	raw, err = r.CallTool("vmware_vcenter_get_library_template_info", map[string]interface{}{"library_item_id": itemID})
	if err != nil {
		t.Fatalf("vmware_vcenter_get_library_template_info failed: %v (%s)", err, raw)
	}

	raw, err = r.CallTool("vmware_vcenter_deploy_template_library_item", map[string]interface{}{
		"library_item_id": itemID,
		"name":            "DeployedFromTemplate",
		"placement":       map[string]interface{}{"folder": folder, "resource_pool": rp},
		"confirm":         true,
	})
	if err != nil {
		t.Fatalf("vmware_vcenter_deploy_template_library_item failed: %v (%s)", err, raw)
	}
	deployedVM, _ := decodeResult(t, raw)["vm"].(string)
	if deployedVM == "" || deployedVM == vm {
		t.Fatalf("expected a new, non-empty deployed VM path, got %q: %s", deployedVM, raw)
	}

	raw, err = r.CallTool("vmware_vcenter_check_out", map[string]interface{}{
		"library_item_id": itemID,
		"name":            "CheckedOutVM",
		"placement":       map[string]interface{}{"folder": folder, "resource_pool": rp},
		"confirm":         true,
	})
	if err != nil {
		t.Fatalf("vmware_vcenter_check_out failed: %v (%s)", err, raw)
	}
	checkedOutVM, _ := decodeResult(t, raw)["vm"].(string)
	if checkedOutVM == "" {
		t.Fatalf("expected a non-empty checked-out VM path in %s", raw)
	}

	raw, err = r.CallTool("vmware_vcenter_check_in", map[string]interface{}{
		"library_item_id": itemID,
		"vm":              checkedOutVM,
		"message":         "checked in by TestVCenterTemplateTools_TemplateLifecycle",
		"confirm":         true,
	})
	if err != nil {
		t.Fatalf("vmware_vcenter_check_in failed: %v (%s)", err, raw)
	}
}

// TestVCenterTemplateTools_OVFFlow exercises CreateOVF -> FilterLibraryItem
// against a real vcsim vcenter.Manager, and proves DeployLibraryItem reaches
// the server cleanly for an OVF item with no uploaded content — a real vcsim
// gap (os.ReadFile on a .ovf file that was never uploaded), documented in
// generated_vcenter_template.go's top doc comment, not a bug in this
// project's tool wiring.
func TestVCenterTemplateTools_OVFFlow(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()
	ctx := context.Background()

	r := newVCenterTemplateRegistry(ctx, c, RegistryOptions{AllowDestructive: true})
	vm := firstVMPath(t, r)
	rp := firstResourcePoolPath(t, r)
	ds := firstDatastoreInventoryPath(t, r, vm)

	libID, libCleanup := newTestLibrary(t, ctx, c, "OVFFlowLib", ds)
	defer libCleanup()

	raw, err := r.CallTool("vmware_vcenter_create_ovf", map[string]interface{}{
		"name":       "TestOVF",
		"source_vm":  vm,
		"library_id": libID,
		"confirm":    true,
	})
	if err != nil {
		t.Fatalf("vmware_vcenter_create_ovf failed: %v (%s)", err, raw)
	}
	itemID, _ := decodeResult(t, raw)["library_item_id"].(string)
	if itemID == "" {
		t.Fatalf("expected a non-empty library_item_id in %s", raw)
	}

	raw, err = r.CallTool("vmware_vcenter_filter_library_item", map[string]interface{}{
		"library_item_id": itemID,
		"target":          map[string]interface{}{"resource_pool": rp},
		"confirm":         true,
	})
	if err != nil {
		t.Fatalf("vmware_vcenter_filter_library_item failed: %v (%s)", err, raw)
	}
	if name, _ := decodeResult(t, raw)["name"].(string); name != "TestOVF" {
		t.Fatalf("expected filter response name=TestOVF, got %s", raw)
	}

	_, err = r.CallTool("vmware_vcenter_deploy_library_item", map[string]interface{}{
		"library_item_id": itemID,
		"deployment_spec": map[string]interface{}{"name": "ovf-deploy", "default_datastore": ds},
		"target":          map[string]interface{}{"resource_pool": rp},
		"confirm":         true,
	})
	assertReachesServer(t, err, "vmware_vcenter_deploy_library_item")
}

// TestVCenterTemplateTools_SyncTemplateLibrary exercises
// vmware_vcenter_sync_template_library against a source library with zero
// OVF items — a genuine, fully-successful vcsim round trip (SyncTemplateLibrary
// loops over the source library's items and does nothing when there are
// none), unlike the OVF-deploy path exercised in TestVCenterTemplateTools_OVFFlow.
func TestVCenterTemplateTools_SyncTemplateLibrary(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()
	ctx := context.Background()

	r := newVCenterTemplateRegistry(ctx, c, RegistryOptions{AllowDestructive: true})
	vm := firstVMPath(t, r)
	ds := firstDatastoreInventoryPath(t, r, vm)

	srcID, srcCleanup := newTestLibrary(t, ctx, c, "SyncSrcEmpty", ds)
	defer srcCleanup()
	dstID, dstCleanup := newTestLibrary(t, ctx, c, "SyncDstEmpty", ds)
	defer dstCleanup()

	raw, err := r.CallTool("vmware_vcenter_sync_template_library", map[string]interface{}{
		"source_library_id":      srcID,
		"destination_library_id": dstID,
		"confirm":                true,
	})
	if err != nil {
		t.Fatalf("vmware_vcenter_sync_template_library failed: %v (%s)", err, raw)
	}
	if count, _ := decodeResult(t, raw)["item_count"].(float64); count != 0 {
		t.Fatalf("expected item_count=0 for an empty source library, got %v: %s", count, raw)
	}
}

// TestVCenterTemplateTools_SyncTemplateLibraryItem exercises
// vmware_vcenter_sync_template_library_item with template.source_vm supplied
// directly — govmomi's SyncTemplateLibraryItem skips its own internal
// DeployLibraryItem call entirely when Template.SourceVM is already set (see
// vcenter_vmtx.go's SyncTemplateLibraryItem: "if spec.SourceVM == \"\" {
// deploy from OVF }"), so this exercises a real, fully-successful path that
// sidesteps the content-upload gap documented for
// TestVCenterTemplateTools_OVFFlow's DeployLibraryItem case.
func TestVCenterTemplateTools_SyncTemplateLibraryItem(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()
	ctx := context.Background()

	r := newVCenterTemplateRegistry(ctx, c, RegistryOptions{AllowDestructive: true})
	vm := firstVMPath(t, r)
	folder := vmFolderPath(vm)
	rp := firstResourcePoolPath(t, r)
	ds := firstDatastoreInventoryPath(t, r, vm)

	srcID, srcCleanup := newTestLibrary(t, ctx, c, "SyncItemSrc", ds)
	defer srcCleanup()
	dstID, dstCleanup := newTestLibrary(t, ctx, c, "SyncItemDst", ds)
	defer dstCleanup()

	// A source OVF item is still required as the library_item_id argument
	// (this tool's handler fetches it via library.Manager.GetLibraryItem
	// before calling SyncTemplateLibraryItem), even though it is never
	// actually read by govmomi's SyncTemplateLibraryItem once
	// Template.SourceVM is non-empty — see this test's top doc comment.
	raw, err := r.CallTool("vmware_vcenter_create_ovf", map[string]interface{}{
		"name":       "SyncItemSourceOVF",
		"source_vm":  vm,
		"library_id": srcID,
		"confirm":    true,
	})
	if err != nil {
		t.Fatalf("vmware_vcenter_create_ovf failed: %v (%s)", err, raw)
	}
	ovfItemID, _ := decodeResult(t, raw)["library_item_id"].(string)

	raw, err = r.CallTool("vmware_vcenter_sync_template_library_item", map[string]interface{}{
		"library_item_id": ovfItemID,
		"template": map[string]interface{}{
			"name":       "SyncItemTemplate",
			"library_id": dstID,
			"source_vm":  vm,
			"placement":  map[string]interface{}{"folder": folder, "resource_pool": rp},
		},
		"confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_vcenter_sync_template_library_item failed: %v (%s)", err, raw)
	}
	m := decodeResult(t, raw)
	if m["template_name"] != "SyncItemTemplate" || m["template_library_id"] != dstID {
		t.Fatalf("unexpected result: %s", raw)
	}
}

// TestVCenterTemplateTools_GateAndConfirm proves this file's tools are wired
// through registerDestructive (all tier2) — same 3-layer protection check
// pattern as generated_authorization_test.go's TestAuthorizationTools_GateAndConfirm.
func TestVCenterTemplateTools_GateAndConfirm(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	closedGate := newVCenterTemplateRegistry(context.Background(), c, RegistryOptions{AllowDestructive: false})
	if _, err := closedGate.CallTool("vmware_vcenter_create_ovf", map[string]interface{}{
		"name": "ShouldNotExist", "source_vm": "/DC0/vm/does-not-matter", "library_id": "lib-1", "confirm": true,
	}); err == nil {
		t.Fatal("expected vmware_vcenter_create_ovf to be denied with the gate closed")
	}

	openGate := newVCenterTemplateRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	if _, err := openGate.CallTool("vmware_vcenter_create_ovf", map[string]interface{}{
		"name": "ShouldNotExist", "source_vm": "/DC0/vm/does-not-matter", "library_id": "lib-1",
	}); err == nil {
		t.Fatal("expected vmware_vcenter_create_ovf to fail without confirm:true")
	}
}

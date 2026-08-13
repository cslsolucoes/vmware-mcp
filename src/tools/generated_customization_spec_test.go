package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/vmware/govmomi/simulator"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// newCustomizationSpecRegistry builds a Registry the normal way (NewRegistry,
// which wires vm.go/host.go/inventory.go/etc via registerTools) and then
// manually layers registerCustomizationSpecTools on top via withClass, the
// same pattern every other Fase 2+ test file in this package uses — this
// file must not edit registry.go itself (see generated_customization_spec.go's
// top doc comment).
func newCustomizationSpecRegistry(ctx context.Context, c *vmware.Client, opts RegistryOptions) *Registry {
	r := NewRegistry(ctx, c, opts)
	r.withClass(modeVCenterOnly, registerCustomizationSpecTools)
	return r
}

// bareCustomizationSpecItem builds the smallest types.CustomizationSpecItem
// this project's decode can populate — omitting "spec.identity"/
// "spec.options" entirely, per this file's top doc comment's documented
// polymorphism limitation. Empirically confirmed (see the top doc comment)
// that vcsim's CreateCustomizationSpec/OverwriteCustomizationSpec accept a
// bare spec like this with no validation error.
func bareCustomizationSpecItem(name string) map[string]interface{} {
	return map[string]interface{}{
		"info": map[string]interface{}{"name": name, "type": "Linux"},
		"spec": map[string]interface{}{
			"globalIPSettings": map[string]interface{}{},
		},
	}
}

// TestCustomizationSpecTools_NilGuardOnStandaloneESXi proves
// requireCustomizationSpecManager turns a nil
// ServiceContent.CustomizationSpecManager into a clean, clearly worded error
// instead of a panic caught only by registry.go's CallTool recover() — see
// this file's top doc comment.
func TestCustomizationSpecTools_NilGuardOnStandaloneESXi(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()
	r := newCustomizationSpecRegistry(context.Background(), c, RegistryOptions{})

	_, err := r.CallTool("vmware_customization_spec_info", map[string]interface{}{})
	if err == nil {
		t.Fatal("expected an error on standalone ESXi (CustomizationSpecManager is nil), got success")
	}
	if !strings.Contains(err.Error(), "requires vCenter") {
		t.Fatalf("expected a clear vCenter-only nil-guard message, got: %v", err)
	}
	if strings.Contains(err.Error(), "panicked") {
		t.Fatalf("nil-guard should return a clean error, not let CallTool's recover() catch a panic: %v", err)
	}
}

// TestCustomizationSpecTools_ReadOnlyAgainstVcsimDefaults exercises
// vmware_customization_spec_does_exist/_get/_info against vcsim's own 4
// built-in default specs (DefaultCustomizationSpec in
// referencia/govmomi/simulator/customization_spec_manager.go: vcsim-linux,
// vcsim-linux-static, vcsim-windows-static, vcsim-windows-domain), so this
// test needs no prior write to reach a real success path — and, since these
// defaults DO have "identity"/"options" populated, this also proves the
// read/output direction (json.Marshal of an interface field that already
// holds a concrete value) has no trouble with the same polymorphism that
// blocks the write/input direction (see this file's top doc comment).
func TestCustomizationSpecTools_ReadOnlyAgainstVcsimDefaults(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()
	r := newCustomizationSpecRegistry(context.Background(), c, RegistryOptions{})

	raw, err := r.CallTool("vmware_customization_spec_does_exist", map[string]interface{}{"name": "vcsim-linux"})
	if err != nil {
		t.Fatalf("vmware_customization_spec_does_exist(vcsim-linux) failed: %v", err)
	}
	if m := decodeResult(t, raw); m["exists"] != true {
		t.Fatalf("expected vcsim-linux to exist, got: %s", raw)
	}

	raw, err = r.CallTool("vmware_customization_spec_does_exist", map[string]interface{}{"name": "no-such-spec"})
	if err != nil {
		t.Fatalf("vmware_customization_spec_does_exist(no-such-spec) failed: %v", err)
	}
	if m := decodeResult(t, raw); m["exists"] != false {
		t.Fatalf("expected no-such-spec to not exist, got: %s", raw)
	}

	raw, err = r.CallTool("vmware_customization_spec_get", map[string]interface{}{"name": "vcsim-windows-static"})
	if err != nil {
		t.Fatalf("vmware_customization_spec_get(vcsim-windows-static) failed: %v", err)
	}
	m := decodeResult(t, raw)
	item, _ := m["item"].(map[string]interface{})
	if item == nil {
		t.Fatalf("expected a non-nil \"item\" in the result, got: %s", raw)
	}
	spec, _ := item["spec"].(map[string]interface{})
	if spec == nil || spec["identity"] == nil {
		t.Fatalf("expected the real vcsim-windows-static default's spec.identity to round-trip out (Marshal side, no polymorphism issue), got: %s", raw)
	}

	raw, err = r.CallTool("vmware_customization_spec_info", map[string]interface{}{})
	if err != nil {
		t.Fatalf("vmware_customization_spec_info failed: %v", err)
	}
	if countOf(t, raw) < 4 {
		t.Fatalf("expected at least vcsim's 4 built-in default specs, got: %s", raw)
	}
}

// TestCustomizationSpecTools_CreateAndOverwrite proves the full
// Create -> DoesExist -> Overwrite round trip against real vcsim using a
// bare spec (see bareCustomizationSpecItem), plus vcsim's own duplicate-name/
// NotFound business rules.
func TestCustomizationSpecTools_CreateAndOverwrite(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()
	r := newCustomizationSpecRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	item := bareCustomizationSpecItem("mcpvmware-test-spec")
	raw, err := r.CallTool("vmware_customization_spec_create", map[string]interface{}{"item": item, "confirm": true})
	if err != nil {
		t.Fatalf("vmware_customization_spec_create failed: %v", err)
	}
	if m := decodeResult(t, raw); m["result"] != "created" {
		t.Fatalf("unexpected create result: %s", raw)
	}

	// vcsim rejects a duplicate name (see simulator/customization_spec_manager.go's
	// CreateCustomizationSpec) — proves this reaches the real server.
	if _, err := r.CallTool("vmware_customization_spec_create", map[string]interface{}{"item": item, "confirm": true}); err == nil {
		t.Fatal("expected creating a duplicate-named spec to fail")
	}

	raw, err = r.CallTool("vmware_customization_spec_does_exist", map[string]interface{}{"name": "mcpvmware-test-spec"})
	if err != nil {
		t.Fatalf("vmware_customization_spec_does_exist after create failed: %v", err)
	}
	if m := decodeResult(t, raw); m["exists"] != true {
		t.Fatalf("expected the newly created spec to exist, got: %s", raw)
	}

	item2 := bareCustomizationSpecItem("mcpvmware-test-spec")
	item2["info"].(map[string]interface{})["description"] = "updated by generated_customization_spec_test.go"
	raw, err = r.CallTool("vmware_customization_spec_overwrite", map[string]interface{}{"item": item2, "confirm": true})
	if err != nil {
		t.Fatalf("vmware_customization_spec_overwrite failed: %v", err)
	}
	if m := decodeResult(t, raw); m["result"] != "overwritten" {
		t.Fatalf("unexpected overwrite result: %s", raw)
	}

	// vcsim faults NotFound overwriting a name that was never created.
	if _, err := r.CallTool("vmware_customization_spec_overwrite", map[string]interface{}{
		"item": bareCustomizationSpecItem("does-not-exist"), "confirm": true,
	}); err == nil {
		t.Fatal("expected overwriting a non-existent spec to fail")
	}
}

// TestCustomizationSpecTools_PolymorphicFieldLimitation proves the
// documented spec.identity/spec.options limitation (this file's top doc
// comment) fails with a clean decode error naming the offending field — not
// a panic, not a silently-wrong request sent to the server.
func TestCustomizationSpecTools_PolymorphicFieldLimitation(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()
	r := newCustomizationSpecRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	item := map[string]interface{}{
		"info": map[string]interface{}{"name": "poly-test", "type": "Linux"},
		"spec": map[string]interface{}{
			"identity": map[string]interface{}{"domain": "example.com"},
		},
	}
	_, err := r.CallTool("vmware_customization_spec_create", map[string]interface{}{"item": item, "confirm": true})
	if err == nil {
		t.Fatal("expected supplying spec.identity to fail with a decode error")
	}
	if strings.Contains(err.Error(), "panicked") {
		t.Fatalf("expected a clean decode error, not a panic: %v", err)
	}
	if !strings.Contains(err.Error(), "identity") {
		t.Fatalf("expected the decode error to name the offending field, got: %v", err)
	}
}

// TestCustomizationSpecTools_NotSimulatedByVcsim proves the 5 methods with no
// server-side vcsim implementation (this file's top doc comment) each reach
// the real server and fault cleanly ("does not implement: <Method>"), not a
// registration bug or a client-side panic — same assertReachesServer
// discipline generated_vm_lifecycle_test.go/generated_resourcepool_vapp_test.go
// established.
func TestCustomizationSpecTools_NotSimulatedByVcsim(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()
	r := newCustomizationSpecRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	item := bareCustomizationSpecItem("reaches-server-item")

	_, err := r.CallTool("vmware_customization_spec_item_to_xml", map[string]interface{}{"item": item})
	assertReachesServer(t, err, "vmware_customization_spec_item_to_xml")

	_, err = r.CallTool("vmware_customization_spec_xml_to_item", map[string]interface{}{"xml": "<CustomizationSpecItem/>"})
	assertReachesServer(t, err, "vmware_customization_spec_xml_to_item")

	_, err = r.CallTool("vmware_customization_spec_delete", map[string]interface{}{"name": "vcsim-linux", "confirm": true})
	assertReachesServer(t, err, "vmware_customization_spec_delete")

	_, err = r.CallTool("vmware_customization_spec_duplicate", map[string]interface{}{
		"name": "vcsim-linux", "new_name": "vcsim-linux-copy", "confirm": true,
	})
	assertReachesServer(t, err, "vmware_customization_spec_duplicate")

	_, err = r.CallTool("vmware_customization_spec_rename", map[string]interface{}{
		"name": "vcsim-linux", "new_name": "vcsim-linux-renamed", "confirm": true,
	})
	assertReachesServer(t, err, "vmware_customization_spec_rename")
}

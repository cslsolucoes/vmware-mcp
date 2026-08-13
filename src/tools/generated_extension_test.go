package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/vmware/govmomi/simulator"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// newExtensionRegistry builds a Registry the normal way (NewRegistry, which
// wires vm.go/host.go/inventory.go/etc via registerTools) and then manually
// layers registerExtensionTools on top via withClass, the same pattern every
// other Fase 2+ test file in this package uses — this file must not edit
// registry.go itself (see generated_extension.go's top doc comment).
func newExtensionRegistry(ctx context.Context, c *vmware.Client, opts RegistryOptions) *Registry {
	r := NewRegistry(ctx, c, opts)
	r.withClass(modeVCenterOnly, registerExtensionTools)
	return r
}

// TestExtensionTools_NilGuardOnStandaloneESXi proves requireExtensionManager
// (which reuses object.GetExtensionManager) turns a nil
// ServiceContent.ExtensionManager into a clean, clearly worded error instead
// of a panic caught only by registry.go's CallTool recover() — see this
// file's top doc comment.
func TestExtensionTools_NilGuardOnStandaloneESXi(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()
	r := newExtensionRegistry(context.Background(), c, RegistryOptions{})

	_, err := r.CallTool("vmware_extension_list", map[string]interface{}{})
	if err == nil {
		t.Fatal("expected an error on standalone ESXi (ExtensionManager is nil), got success")
	}
	if !strings.Contains(err.Error(), "requires vCenter") {
		t.Fatalf("expected a clear vCenter-only nil-guard message, got: %v", err)
	}
	if strings.Contains(err.Error(), "panicked") {
		t.Fatalf("nil-guard should return a clean error, not let CallTool's recover() catch a panic: %v", err)
	}
}

// TestExtensionTools_FindAndList exercises the 2 read-only tools against
// vcsim's own seeded extension ("com.vmware.govmomi.simulator" — only added
// when r.IsVPX(), see referencia/govmomi/simulator/extension_manager.go's
// init()), so this test needs no prior registration to reach a real success
// path.
func TestExtensionTools_FindAndList(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()
	r := newExtensionRegistry(context.Background(), c, RegistryOptions{})

	raw, err := r.CallTool("vmware_extension_find", map[string]interface{}{"key": "com.vmware.govmomi.simulator"})
	if err != nil {
		t.Fatalf("vmware_extension_find failed: %v", err)
	}
	m := decodeResult(t, raw)
	if found, _ := m["found"].(bool); !found {
		t.Fatalf("expected the vcsim-seeded extension to be found, got: %s", raw)
	}
	if _, ok := m["extension"]; !ok {
		t.Fatalf("expected an \"extension\" field when found, got: %s", raw)
	}

	raw, err = r.CallTool("vmware_extension_find", map[string]interface{}{"key": "no.such.extension"})
	if err != nil {
		t.Fatalf("vmware_extension_find(missing) failed: %v", err)
	}
	m = decodeResult(t, raw)
	if found, _ := m["found"].(bool); found {
		t.Fatalf("expected found:false for a missing extension, got: %s", raw)
	}

	raw, err = r.CallTool("vmware_extension_list", map[string]interface{}{})
	if err != nil {
		t.Fatalf("vmware_extension_list failed: %v", err)
	}
	if countOf(t, raw) < 1 {
		t.Fatalf("expected at least the vcsim-seeded extension in the list, got: %s", raw)
	}
}

// TestExtensionTools_Lifecycle proves the full Register -> Find -> SetCertificate
// -> Update -> Unregister -> Find (gone) round trip against real vcsim,
// including the decodeExtensionArg workaround for the polymorphic
// "description" field (see this file's top doc comment) and vcsim's own
// duplicate-key/NotFound business rules.
func TestExtensionTools_Lifecycle(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()
	r := newExtensionRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	ext := map[string]interface{}{
		"key":     "com.mcpvmware.test",
		"version": "1.0.0",
		"company": "MCPVMWare Tests",
		"description": map[string]interface{}{
			"label":   "Test Extension",
			"summary": "Created by generated_extension_test.go",
		},
	}

	raw, err := r.CallTool("vmware_extension_register", map[string]interface{}{"extension": ext, "confirm": true})
	if err != nil {
		t.Fatalf("vmware_extension_register failed: %v", err)
	}
	if m := decodeResult(t, raw); m["result"] != "registered" {
		t.Fatalf("unexpected register result: %s", raw)
	}

	// vcsim rejects a duplicate key (see simulator/extension_manager.go's
	// RegisterExtension) — proves this reaches the real server, not a stub.
	if _, err := r.CallTool("vmware_extension_register", map[string]interface{}{"extension": ext, "confirm": true}); err == nil {
		t.Fatal("expected registering a duplicate key to fail")
	}

	raw, err = r.CallTool("vmware_extension_find", map[string]interface{}{"key": "com.mcpvmware.test"})
	if err != nil {
		t.Fatalf("vmware_extension_find after register failed: %v", err)
	}
	if m := decodeResult(t, raw); m["found"] != true {
		t.Fatalf("expected the newly registered extension to be found, got: %s", raw)
	}

	pem := "-----BEGIN CERTIFICATE-----\nMIIB...fake-test-cert...\n-----END CERTIFICATE-----"
	if _, err := r.CallTool("vmware_extension_set_certificate", map[string]interface{}{
		"key": "com.mcpvmware.test", "certificate_pem": pem, "confirm": true,
	}); err != nil {
		t.Fatalf("vmware_extension_set_certificate failed: %v", err)
	}
	// SetCertificate on a key that isn't registered faults NotFound —
	// confirms the call really reaches simulator.ExtensionManager.SetExtensionCertificate.
	if _, err := r.CallTool("vmware_extension_set_certificate", map[string]interface{}{
		"key": "no.such.extension", "certificate_pem": pem, "confirm": true,
	}); err == nil {
		t.Fatal("expected SetCertificate on an unregistered key to fail")
	}

	ext["company"] = "MCPVMWare Tests Updated"
	raw, err = r.CallTool("vmware_extension_update", map[string]interface{}{"extension": ext, "confirm": true})
	if err != nil {
		t.Fatalf("vmware_extension_update failed: %v", err)
	}
	if m := decodeResult(t, raw); m["result"] != "updated" {
		t.Fatalf("unexpected update result: %s", raw)
	}

	raw, err = r.CallTool("vmware_extension_unregister", map[string]interface{}{"key": "com.mcpvmware.test", "confirm": true})
	if err != nil {
		t.Fatalf("vmware_extension_unregister failed: %v", err)
	}
	if m := decodeResult(t, raw); m["result"] != "unregistered" {
		t.Fatalf("unexpected unregister result: %s", raw)
	}

	raw, err = r.CallTool("vmware_extension_find", map[string]interface{}{"key": "com.mcpvmware.test"})
	if err != nil {
		t.Fatalf("vmware_extension_find after unregister failed: %v", err)
	}
	if m := decodeResult(t, raw); m["found"] != false {
		t.Fatalf("expected the unregistered extension to be gone, got: %s", raw)
	}

	if _, err := r.CallTool("vmware_extension_unregister", map[string]interface{}{"key": "com.mcpvmware.test", "confirm": true}); err == nil {
		t.Fatal("expected unregistering an already-gone key to fail")
	}
}

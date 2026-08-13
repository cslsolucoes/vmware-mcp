package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/vmware/govmomi/simulator"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// newCustomFieldsRegistry builds a Registry the normal way (NewRegistry,
// which wires vm.go/host.go/etc via registerTools) and then manually layers
// this group's tools on top via withClass — same pattern as
// generated_authorization_test.go/generated_network_test.go, and for the
// same reason: registry.go itself must not be edited by this file (see
// generated_custom_fields.go's top doc comment). Registration itself is
// unaffected by RegistryOptions.ConnectionMode being left at its zero value
// here (unrestricted) — every test in this file needs vmware_custom_field_*
// registered regardless of whether the underlying vcsim model is VPX or ESX,
// so the nil-guard test below can prove the FAILURE happens inside the
// handler (ServiceContent.CustomFieldsManager is nil), not because the tool
// was filtered out of the registry.
func newCustomFieldsRegistry(ctx context.Context, c *vmware.Client, opts RegistryOptions) *Registry {
	r := NewRegistry(ctx, c, opts)
	r.withClass(modeVCenterOnly, registerCustomFieldsTools)
	return r
}

func TestCustomFieldsTools_Registration(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newCustomFieldsRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	want := []string{
		"vmware_custom_field_add",
		"vmware_custom_field_field",
		"vmware_custom_field_find_key",
		"vmware_custom_field_remove",
		"vmware_custom_field_rename",
		"vmware_custom_field_set",
	}
	if len(want) != 6 {
		t.Fatalf("test bug: want list has %d entries, expected 6", len(want))
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

// TestCustomFieldsTools_Lifecycle exercises Add -> FindKey -> Field ->
// Rename -> Set -> Remove end to end against a real vcsim
// CustomFieldsManager (simulator.VPX() — CustomFieldsManager is non-nil
// there, see this file's/generated_custom_fields.go's top doc comment).
func TestCustomFieldsTools_Lifecycle(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newCustomFieldsRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	raw, err := r.CallTool("vmware_custom_field_add", map[string]interface{}{
		"name":                "TestField",
		"managed_object_type": "VirtualMachine",
		"confirm":             true,
	})
	if err != nil {
		t.Fatalf("vmware_custom_field_add failed: %v (%s)", err, raw)
	}
	m := decodeResult(t, raw)
	field, ok := m["field"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected a field object in %s", raw)
	}
	key, ok := field["key"].(float64)
	if !ok {
		t.Fatalf("expected numeric field.key in %s", raw)
	}

	raw, err = r.CallTool("vmware_custom_field_find_key", map[string]interface{}{"name": "TestField"})
	if err != nil {
		t.Fatalf("vmware_custom_field_find_key failed: %v", err)
	}
	if foundKey, _ := decodeResult(t, raw)["key"].(float64); foundKey != key {
		t.Fatalf("expected key %v from find_key, got %v: %s", key, foundKey, raw)
	}

	raw, err = r.CallTool("vmware_custom_field_field", map[string]interface{}{})
	if err != nil {
		t.Fatalf("vmware_custom_field_field failed: %v", err)
	}
	if !customFieldListHasName(t, raw, "TestField") {
		t.Fatalf("expected TestField in field list: %s", raw)
	}

	raw, err = r.CallTool("vmware_custom_field_rename", map[string]interface{}{
		"key":     key,
		"name":    "TestFieldRenamed",
		"confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_custom_field_rename failed: %v (%s)", err, raw)
	}

	raw, err = r.CallTool("vmware_custom_field_field", map[string]interface{}{})
	if err != nil {
		t.Fatalf("vmware_custom_field_field (after rename) failed: %v", err)
	}
	if !customFieldListHasName(t, raw, "TestFieldRenamed") {
		t.Fatalf("expected TestFieldRenamed after rename: %s", raw)
	}

	vm := firstVMPath(t, r)
	raw, err = r.CallTool("vmware_custom_field_set", map[string]interface{}{
		"entity_path": vm,
		"key":         key,
		"value":       "hello",
		"confirm":     true,
	})
	if err != nil {
		t.Fatalf("vmware_custom_field_set failed: %v (%s)", err, raw)
	}

	raw, err = r.CallTool("vmware_custom_field_remove", map[string]interface{}{
		"key":     key,
		"confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_custom_field_remove failed: %v (%s)", err, raw)
	}

	raw, err = r.CallTool("vmware_custom_field_field", map[string]interface{}{})
	if err != nil {
		t.Fatalf("vmware_custom_field_field (after remove) failed: %v", err)
	}
	if customFieldListHasName(t, raw, "TestFieldRenamed") {
		t.Fatalf("expected TestFieldRenamed to be removed, still present: %s", raw)
	}
}

func customFieldListHasName(t *testing.T, raw string, name string) bool {
	t.Helper()
	fields, _ := decodeResult(t, raw)["fields"].([]interface{})
	for _, item := range fields {
		fm, _ := item.(map[string]interface{})
		if fm["name"] == name {
			return true
		}
	}
	return false
}

func TestCustomFieldsTools_GateAndConfirm(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	closedGate := newCustomFieldsRegistry(context.Background(), c, RegistryOptions{AllowDestructive: false})
	if _, err := closedGate.CallTool("vmware_custom_field_add", map[string]interface{}{
		"name": "ShouldNotExist", "confirm": true,
	}); err == nil {
		t.Fatal("expected vmware_custom_field_add to be denied with the gate closed")
	}

	openGate := newCustomFieldsRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	if _, err := openGate.CallTool("vmware_custom_field_add", map[string]interface{}{
		"name": "ShouldNotExist",
	}); err == nil {
		t.Fatal("expected vmware_custom_field_add to fail without confirm:true")
	}
}

// TestCustomFieldsTools_NilGuardAgainstESX proves requireCustomFieldsManager's
// nil-guard genuinely works against a real connection where
// ServiceContent.CustomFieldsManager is nil (standalone ESXi — confirmed by
// reading referencia/govmomi/simulator/esx/service_content.go), not just in
// theory — every one of this group's 6 tools must fail with a clean,
// explicit error (never a recovered panic) when called against
// simulator.ESX(). Same discipline/message-shape check as
// generated_vm_provisioning_test.go's
// TestVMProvisioning_CheckerNilServiceContentGuard, which proved the
// equivalent guard for VmProvisioningChecker/VmCompatibilityChecker.
func TestCustomFieldsTools_NilGuardAgainstESX(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newCustomFieldsRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	cases := []struct {
		tool string
		args map[string]interface{}
	}{
		{"vmware_custom_field_add", map[string]interface{}{"name": "X", "confirm": true}},
		{"vmware_custom_field_field", map[string]interface{}{}},
		{"vmware_custom_field_find_key", map[string]interface{}{"name": "X"}},
		{"vmware_custom_field_remove", map[string]interface{}{"key": 1, "confirm": true}},
		{"vmware_custom_field_rename", map[string]interface{}{"key": 1, "name": "X", "confirm": true}},
		{"vmware_custom_field_set", map[string]interface{}{"entity_path": "/", "key": 1, "value": "x", "confirm": true}},
	}

	for _, tc := range cases {
		t.Run(tc.tool, func(t *testing.T) {
			_, err := r.CallTool(tc.tool, tc.args)
			if err == nil {
				t.Fatalf("expected %s to fail against standalone ESXi (CustomFieldsManager is nil there), got success", tc.tool)
			}
			if strings.Contains(err.Error(), "panicked") {
				t.Fatalf("%s: expected a clean guard error, got a recovered panic instead: %v", tc.tool, err)
			}
			if !strings.Contains(err.Error(), "not available on this connection") {
				t.Fatalf("%s: expected the explicit nil-ServiceContent guard error, got: %v", tc.tool, err)
			}
			if !strings.Contains(err.Error(), "vCenter") {
				t.Fatalf("%s: expected the guard error to explain vCenter is required, got: %v", tc.tool, err)
			}
		})
	}
}

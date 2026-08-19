package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/vmware/govmomi/simulator"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// newLicenseRegistry builds a Registry the normal way (NewRegistry, which
// wires vm.go/host.go/inventory.go/etc via registerTools) and then manually
// layers registerLicenseTools on top via withClass — the same pattern every
// other Fase 2+ test file in this package uses (see generated_extension_test.go's
// newExtensionRegistry) — this file must not edit registry.go itself (see
// generated_license.go's top doc comment; a later coordination pass wires
// registerLicenseTools into Registry.registerTools and mode_test.go's
// vsphereGeneralTools list).
func newLicenseRegistry(ctx context.Context, c *vmware.Client, opts RegistryOptions) *Registry {
	r := NewRegistry(ctx, c, opts)
	r.withClass(modeVSphereGeneral, registerLicenseTools)
	return r
}

// licenseToolNames is the exact set registered by registerLicenseTools —
// kept here so TestLicenseTools_Registration can't silently drift from the
// real list in generated_license.go.
var licenseToolNames = []string{
	"vmware_license_decode",
	"vmware_license_query_source_availability",
	"vmware_license_query_supported_features",
	"vmware_license_check_feature",
	"vmware_license_add",
	"vmware_license_remove",
	"vmware_license_update",
	"vmware_license_update_label",
	"vmware_license_configure_source",
	"vmware_license_set_edition",
	"vmware_license_query_assigned",
	"vmware_license_update_assigned",
	"vmware_license_remove_assigned",
}

// TestLicenseTools_Registration proves all 13 license tools are registered
// and reachable via ListTools.
func TestLicenseTools_Registration(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()
	r := newLicenseRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	if len(licenseToolNames) != 13 {
		t.Fatalf("test bug: licenseToolNames has %d entries, expected 13", len(licenseToolNames))
	}
	got := toolNameSet(t, r)
	for _, name := range licenseToolNames {
		if !got[name] {
			t.Errorf("tool %s not registered", name)
		}
	}
}

// TestLicenseTools_Validation proves each LicenseManager-level handler
// (everything except the 3 LicenseAssignmentManager tools, validated
// separately in TestLicenseTools_AssignmentValidation since they need a
// vCenter LicenseAssignmentManager to resolve before argument validation
// even runs) rejects missing/invalid required arguments BEFORE any network
// round trip, even with the gate open and confirm:true.
func TestLicenseTools_Validation(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()
	r := newLicenseRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	cases := []struct {
		name string
		args map[string]interface{}
		why  string
	}{
		{"vmware_license_decode", map[string]interface{}{}, "missing license_key"},
		{"vmware_license_check_feature", map[string]interface{}{}, "missing feature_key"},
		{"vmware_license_add", map[string]interface{}{"confirm": true}, "missing license_key"},
		{"vmware_license_remove", map[string]interface{}{"confirm": true}, "missing license_key"},
		{"vmware_license_update", map[string]interface{}{"confirm": true}, "missing license_key"},
		{"vmware_license_update_label", map[string]interface{}{"license_key": "x", "confirm": true}, "missing label_key"},
		{"vmware_license_update_label", map[string]interface{}{"license_key": "x", "label_key": "k", "confirm": true}, "missing label_value"},
		{"vmware_license_configure_source", map[string]interface{}{"confirm": true}, "missing license_source"},
		{"vmware_license_configure_source", map[string]interface{}{"license_source": map[string]interface{}{"type": "bogus"}, "confirm": true}, "invalid license_source.type"},
		{"vmware_license_configure_source", map[string]interface{}{"license_source": map[string]interface{}{"type": "local"}, "confirm": true}, "missing license_source.license_keys"},
		{"vmware_license_configure_source", map[string]interface{}{"license_source": map[string]interface{}{"type": "served"}, "confirm": true}, "missing license_source.license_server"},
	}

	for _, tc := range cases {
		t.Run(tc.name+"/"+tc.why, func(t *testing.T) {
			if _, err := r.CallTool(tc.name, tc.args); err == nil {
				t.Fatalf("expected an error (%s) before any round trip", tc.why)
			}
		})
	}
}

// TestLicenseTools_NilGuardOnStandaloneESXi_AssignmentManager proves
// licenseAssignmentManagerRef (see generated_license.go's top doc comment on
// why a standalone ESXi host never populates LicenseManager.
// licenseAssignmentManager) turns that into a clean, clearly worded error for
// all 3 LicenseAssignmentManager tools instead of a panic caught only by
// registry.go's CallTool recover().
func TestLicenseTools_NilGuardOnStandaloneESXi_AssignmentManager(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()
	r := newLicenseRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	cases := []struct {
		name string
		args map[string]interface{}
	}{
		{"vmware_license_query_assigned", map[string]interface{}{}},
		{"vmware_license_update_assigned", map[string]interface{}{"entity_id": "e1", "license_key": "k1", "confirm": true}},
		{"vmware_license_remove_assigned", map[string]interface{}{"entity_id": "e1", "confirm": true}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := r.CallTool(tc.name, tc.args)
			if err == nil {
				t.Fatalf("%s: expected an error on standalone ESXi (LicenseAssignmentManager is nil), got success", tc.name)
			}
			if !strings.Contains(err.Error(), "require vCenter") {
				t.Fatalf("%s: expected a clear vCenter-only nil-guard message, got: %v", tc.name, err)
			}
			if strings.Contains(err.Error(), "panicked") {
				t.Fatalf("%s: nil-guard should return a clean error, not let CallTool's recover() catch a panic: %v", tc.name, err)
			}
		})
	}
}

// TestLicenseTools_AssignmentValidation proves vmware_license_update_assigned
// / vmware_license_remove_assigned reject missing required arguments against
// a real vCenter (where licenseAssignmentManagerRef resolves successfully,
// unlike ESX above) — isolating argument validation from the nil-guard path.
func TestLicenseTools_AssignmentValidation(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()
	r := newLicenseRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	cases := []struct {
		name string
		args map[string]interface{}
		why  string
	}{
		{"vmware_license_update_assigned", map[string]interface{}{"license_key": "k1", "confirm": true}, "missing entity_id"},
		{"vmware_license_update_assigned", map[string]interface{}{"entity_id": "e1", "confirm": true}, "missing license_key"},
		{"vmware_license_remove_assigned", map[string]interface{}{"confirm": true}, "missing entity_id"},
	}

	for _, tc := range cases {
		t.Run(tc.name+"/"+tc.why, func(t *testing.T) {
			if _, err := r.CallTool(tc.name, tc.args); err == nil {
				t.Fatalf("expected an error (%s) before any round trip", tc.why)
			}
		})
	}
}

// TestLicenseTools_GateAndConfirm proves the tier2 destructive protection is
// wired: a closed --allow-destructive gate denies vmware_license_add, and an
// open gate still requires confirm:true.
func TestLicenseTools_GateAndConfirm(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	closed := newLicenseRegistry(context.Background(), c, RegistryOptions{})
	open := newLicenseRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	validArgs := map[string]interface{}{"license_key": "GATE1-GATE1-GATE1-GATE1-GATE1", "confirm": true}
	if _, err := closed.CallTool("vmware_license_add", validArgs); err == nil {
		t.Fatal("expected the closed destructive gate to deny vmware_license_add")
	}

	noConfirm := map[string]interface{}{"license_key": "GATE1-GATE1-GATE1-GATE1-GATE1"}
	if _, err := open.CallTool("vmware_license_add", noConfirm); err == nil {
		t.Fatal("expected an error without confirm:true")
	}
}

// TestLicenseTools_LicenseManagerLifecycle_RealSuccess drives the
// Add -> Decode -> UpdateLicenseLabel -> Remove -> Decode(gone) round trip
// against real vcsim, for the 4 LicenseManager methods
// referencia/govmomi/simulator/license_manager.go actually implements
// (AddLicense, RemoveLicense, DecodeLicense, UpdateLicenseLabel — confirmed
// by reading that file directly, see generated_license.go's top doc
// comment).
func TestLicenseTools_LicenseManagerLifecycle_RealSuccess(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()
	r := newLicenseRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	const key = "TEST1-TEST1-TEST1-TEST1-TEST1"

	raw, err := r.CallTool("vmware_license_add", map[string]interface{}{
		"license_key": key,
		"labels": []interface{}{
			map[string]interface{}{"key": "purpose", "value": "test"},
		},
		"confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_license_add failed: %v", err)
	}
	m := decodeResult(t, raw)
	if m["result"] != "added" {
		t.Fatalf("unexpected add result: %s", raw)
	}
	lic, _ := m["license"].(map[string]interface{})
	if lic["licenseKey"] != key {
		t.Fatalf("expected licenseKey %q in add response, got: %s", key, raw)
	}

	raw, err = r.CallTool("vmware_license_decode", map[string]interface{}{"license_key": key})
	if err != nil {
		t.Fatalf("vmware_license_decode failed: %v", err)
	}
	m = decodeResult(t, raw)
	lic, _ = m["license"].(map[string]interface{})
	if lic["licenseKey"] != key {
		t.Fatalf("expected the just-added key to be decodable, got: %s", raw)
	}

	if _, err := r.CallTool("vmware_license_update_label", map[string]interface{}{
		"license_key": key, "label_key": "purpose", "label_value": "updated", "confirm": true,
	}); err != nil {
		t.Fatalf("vmware_license_update_label failed: %v", err)
	}

	// UpdateLicenseLabel on an unregistered key faults InvalidArgument (see
	// simulator.LicenseManager.UpdateLicenseLabel) — proves this reaches the
	// real simulator business rule, not a stub.
	if _, err := r.CallTool("vmware_license_update_label", map[string]interface{}{
		"license_key": "NO-SUCH-KEY", "label_key": "x", "label_value": "y", "confirm": true,
	}); err == nil {
		t.Fatal("expected UpdateLicenseLabel on an unregistered key to fail")
	}

	raw, err = r.CallTool("vmware_license_remove", map[string]interface{}{"license_key": key, "confirm": true})
	if err != nil {
		t.Fatalf("vmware_license_remove failed: %v", err)
	}
	if m := decodeResult(t, raw); m["result"] != "removed" {
		t.Fatalf("unexpected remove result: %s", raw)
	}

	// DecodeLicense doesn't require the key to already be registered — it
	// always succeeds — but after removal no license in the manager's own
	// list matches, so simulator.LicenseManager.DecodeLicense (confirmed by
	// reading its source) leaves Returnval as its zero value instead of
	// echoing the key back.
	raw, err = r.CallTool("vmware_license_decode", map[string]interface{}{"license_key": key})
	if err != nil {
		t.Fatalf("vmware_license_decode after remove failed: %v", err)
	}
	m = decodeResult(t, raw)
	lic, _ = m["license"].(map[string]interface{})
	if lic["licenseKey"] == key {
		t.Fatalf("expected the removed key to no longer decode to itself, got: %s", raw)
	}
}

// TestLicenseTools_AssignmentManagerLifecycle_RealSuccess drives
// vmware_license_query_assigned and vmware_license_update_assigned against
// real vcsim. vcsim seeds one assignment per host/cluster automatically
// (simulator.LicenseAssignmentManager.PutObject, confirmed by reading its
// source), so simulator.VPX()'s default hosts/clusters give this a real
// entity_id to work with without this test creating one itself.
func TestLicenseTools_AssignmentManagerLifecycle_RealSuccess(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()
	r := newLicenseRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	raw, err := r.CallTool("vmware_license_query_assigned", map[string]interface{}{})
	if err != nil {
		t.Fatalf("vmware_license_query_assigned failed: %v", err)
	}
	m := decodeResult(t, raw)
	assignments, _ := m["assignments"].([]interface{})
	if len(assignments) == 0 {
		t.Fatalf("expected at least one seeded license assignment (vcsim assigns EvalLicense to each host/cluster on creation), got: %s", raw)
	}
	first, _ := assignments[0].(map[string]interface{})
	entityID, _ := first["entityId"].(string)
	if entityID == "" {
		t.Fatalf("expected a non-empty entityId in the first seeded assignment, got: %s", raw)
	}

	raw, err = r.CallTool("vmware_license_query_assigned", map[string]interface{}{"entity_id": entityID})
	if err != nil {
		t.Fatalf("vmware_license_query_assigned(entity_id) failed: %v", err)
	}
	if countOf(t, raw) != 1 {
		t.Fatalf("expected exactly 1 assignment filtered by entity_id %q, got: %s", entityID, raw)
	}

	const newKey = "ASSIGN-TEST-TEST1-TEST1-TEST1"
	if _, err := r.CallTool("vmware_license_add", map[string]interface{}{"license_key": newKey, "confirm": true}); err != nil {
		t.Fatalf("vmware_license_add (setup for assignment test) failed: %v", err)
	}

	raw, err = r.CallTool("vmware_license_update_assigned", map[string]interface{}{
		"entity_id": entityID, "license_key": newKey, "entity_display_name": "mcpvmware-test", "confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_license_update_assigned failed: %v", err)
	}
	m = decodeResult(t, raw)
	if m["result"] != "assigned" {
		t.Fatalf("unexpected update_assigned result: %s", raw)
	}
	lic, _ := m["license"].(map[string]interface{})
	if lic["licenseKey"] != newKey {
		t.Fatalf("expected the entity's assigned license to now be %q, got: %s", newKey, raw)
	}

	// UpdateAssignedLicense with an unregistered license_key faults
	// InvalidArgument (see simulator.LicenseAssignmentManager.
	// UpdateAssignedLicense) — proves this reaches the real simulator
	// business rule, not a stub.
	if _, err := r.CallTool("vmware_license_update_assigned", map[string]interface{}{
		"entity_id": entityID, "license_key": "NO-SUCH-KEY", "confirm": true,
	}); err == nil {
		t.Fatal("expected UpdateAssignedLicense with an unregistered license_key to fail")
	}
}

// TestLicenseTools_UnsimulatedMethods_ReachesServer drives the 7 methods
// with NO matching receiver on referencia/govmomi/simulator/license_manager.go
// (confirmed by reading that file directly, not assumed from the method
// name — see generated_license.go's top doc comment): UpdateLicense,
// QueryLicenseSourceAvailability, QuerySupportedFeatures, CheckLicenseFeature,
// ConfigureLicenseSource, SetLicenseEdition, RemoveAssignedLicense. Each is
// expected to reach vcsim's dispatcher and come back with a clean
// MethodNotFound-style fault — assertReachesServer proves that, not "unknown
// tool" (registration broken) or a recovered panic (handler bug), the same
// pattern generated_vm_lifecycle_test.go and
// generated_host_iscsi_portbinding_test.go use for their own unsimulated
// methods.
func TestLicenseTools_UnsimulatedMethods_ReachesServer(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()
	r := newLicenseRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	// A real assignment entity_id, needed for RemoveAssignedLicense — vcsim
	// assigns EvalLicense to every host/cluster on creation (see
	// TestLicenseTools_AssignmentManagerLifecycle_RealSuccess's doc comment).
	raw, err := r.CallTool("vmware_license_query_assigned", map[string]interface{}{})
	if err != nil {
		t.Fatalf("vmware_license_query_assigned failed: %v", err)
	}
	assignments, _ := decodeResult(t, raw)["assignments"].([]interface{})
	if len(assignments) == 0 {
		t.Fatal("expected at least one seeded assignment to test vmware_license_remove_assigned against")
	}
	first, _ := assignments[0].(map[string]interface{})
	entityID, _ := first["entityId"].(string)

	cases := []struct {
		name string
		args map[string]interface{}
	}{
		{"vmware_license_update", map[string]interface{}{"license_key": "00000-00000-00000-00000-00000", "confirm": true}},
		{"vmware_license_query_source_availability", map[string]interface{}{}},
		{"vmware_license_query_supported_features", map[string]interface{}{}},
		{"vmware_license_check_feature", map[string]interface{}{"feature_key": "dvs"}},
		{"vmware_license_configure_source", map[string]interface{}{"license_source": map[string]interface{}{"type": "evaluation"}, "confirm": true}},
		{"vmware_license_set_edition", map[string]interface{}{"feature_key": "vc.standard", "confirm": true}},
		{"vmware_license_remove_assigned", map[string]interface{}{"entity_id": entityID, "confirm": true}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := r.CallTool(tc.name, tc.args)
			assertReachesServer(t, err, tc.name)
		})
	}
}

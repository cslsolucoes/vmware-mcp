package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/vmware/govmomi/simulator"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// newTenantRegistry builds a Registry the normal way (NewRegistry, which
// wires vm.go/host.go/inventory.go/etc via registerTools) and then manually
// layers registerTenantTools on top via withClass, the same pattern every
// other Fase 2+ test file in this package uses — this file must not edit
// registry.go itself (see generated_tenant.go's top doc comment). Reuses
// firstVMPath (vm_test.go) and firstHostPath (host_test.go), both already
// registered as part of NewRegistry's normal wiring, to get real entity
// paths to mark/unmark instead of hardcoding vcsim's internal default-model
// naming.
func newTenantRegistry(ctx context.Context, c *vmware.Client, opts RegistryOptions) *Registry {
	r := NewRegistry(ctx, c, opts)
	r.withClass(modeVCenterOnly, registerTenantTools)
	return r
}

// TestTenantTools_NilGuardOnStandaloneESXi proves requireTenantManager turns
// a nil ServiceContent.TenantManager into a clean, clearly worded error
// instead of a panic caught only by registry.go's CallTool recover() — see
// this file's top doc comment.
func TestTenantTools_NilGuardOnStandaloneESXi(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()
	r := newTenantRegistry(context.Background(), c, RegistryOptions{})

	_, err := r.CallTool("vmware_tenant_retrieve_service_provider_entities", map[string]interface{}{})
	if err == nil {
		t.Fatal("expected an error on standalone ESXi (TenantManager is nil), got success")
	}
	if !strings.Contains(err.Error(), "requires vCenter") {
		t.Fatalf("expected a clear vCenter-only nil-guard message, got: %v", err)
	}
	if strings.Contains(err.Error(), "panicked") {
		t.Fatalf("nil-guard should return a clean error, not let CallTool's recover() catch a panic: %v", err)
	}
}

// TestTenantTools_MarkRetrieveUnmark proves the full
// Mark -> Retrieve -> Unmark -> Retrieve round trip against real vcsim
// (simulator.TenantManager's in-memory spEntities set — see this file's top
// doc comment), including that vmware_tenant_retrieve_service_provider_entities
// resolves each returned moref back to a real inventory path via
// referenceInventoryPath (tenantEntityResults), not a bare {type,value} blob.
func TestTenantTools_MarkRetrieveUnmark(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()
	r := newTenantRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	vmPath := firstVMPath(t, r)
	hostPath := firstHostPath(t, r)

	raw, err := r.CallTool("vmware_tenant_retrieve_service_provider_entities", map[string]interface{}{})
	if err != nil {
		t.Fatalf("vmware_tenant_retrieve_service_provider_entities (before mark) failed: %v", err)
	}
	if countOf(t, raw) != 0 {
		t.Fatalf("expected 0 marked entities before any mark, got: %s", raw)
	}

	raw, err = r.CallTool("vmware_tenant_mark_service_provider_entities", map[string]interface{}{
		"entity_paths": []interface{}{vmPath, hostPath},
		"confirm":      true,
	})
	if err != nil {
		t.Fatalf("vmware_tenant_mark_service_provider_entities failed: %v", err)
	}
	if m := decodeResult(t, raw); m["result"] != "marked" {
		t.Fatalf("unexpected mark result: %s", raw)
	}
	if countOf(t, raw) != 2 {
		t.Fatalf("expected count 2 in mark result, got: %s", raw)
	}

	raw, err = r.CallTool("vmware_tenant_retrieve_service_provider_entities", map[string]interface{}{})
	if err != nil {
		t.Fatalf("vmware_tenant_retrieve_service_provider_entities (after mark) failed: %v", err)
	}
	if countOf(t, raw) != 2 {
		t.Fatalf("expected 2 marked entities after marking, got: %s", raw)
	}
	m := decodeResult(t, raw)
	entities, _ := m["entities"].([]interface{})
	gotPaths := map[string]bool{}
	for _, e := range entities {
		em, ok := e.(map[string]interface{})
		if !ok {
			t.Fatalf("expected each entity to be an object, got: %#v", e)
		}
		p, _ := em["inventory_path"].(string)
		if p == "" {
			t.Fatalf("expected a resolved inventory_path for a live entity, got empty: %+v", em)
		}
		gotPaths[p] = true
	}
	if !gotPaths[vmPath] {
		t.Fatalf("expected %s to be in the retrieved set, got: %s", vmPath, raw)
	}
	if !gotPaths[hostPath] {
		t.Fatalf("expected %s to be in the retrieved set, got: %s", hostPath, raw)
	}

	raw, err = r.CallTool("vmware_tenant_unmark_service_provider_entities", map[string]interface{}{
		"entity_paths": []interface{}{vmPath},
		"confirm":      true,
	})
	if err != nil {
		t.Fatalf("vmware_tenant_unmark_service_provider_entities failed: %v", err)
	}
	if m := decodeResult(t, raw); m["result"] != "unmarked" {
		t.Fatalf("unexpected unmark result: %s", raw)
	}

	raw, err = r.CallTool("vmware_tenant_retrieve_service_provider_entities", map[string]interface{}{})
	if err != nil {
		t.Fatalf("vmware_tenant_retrieve_service_provider_entities (after unmark) failed: %v", err)
	}
	if countOf(t, raw) != 1 {
		t.Fatalf("expected 1 marked entity remaining after unmarking one, got: %s", raw)
	}
	m = decodeResult(t, raw)
	entities, _ = m["entities"].([]interface{})
	remaining := entities[0].(map[string]interface{})
	if remaining["inventory_path"] != hostPath {
		t.Fatalf("expected the remaining marked entity to be %s, got: %s", hostPath, raw)
	}
}

// TestTenantTools_MarkRejectsUnresolvableEntityPath proves an entity_paths
// entry that doesn't resolve to any real inventory object fails cleanly
// (via resolveEntityRefs/object.SearchIndex.FindByInventoryPath, generated_authorization.go)
// instead of silently marking nothing or crashing.
func TestTenantTools_MarkRejectsUnresolvableEntityPath(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()
	r := newTenantRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	_, err := r.CallTool("vmware_tenant_mark_service_provider_entities", map[string]interface{}{
		"entity_paths": []interface{}{"/DC0/vm/does-not-exist"},
		"confirm":      true,
	})
	if err == nil {
		t.Fatal("expected an error for an unresolvable entity path")
	}
	if strings.Contains(err.Error(), "panicked") {
		t.Fatalf("expected a clean resolution error, not a panic: %v", err)
	}
}

package tools

import (
	"context"
	"testing"

	"github.com/vmware/govmomi/simulator"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// newAuthorizationRegistry builds a Registry the normal way (NewRegistry,
// which wires vm.go/host.go/etc via registerTools) and then manually layers
// this group's tools on top via withClass — same pattern as
// generated_network_test.go/generated_vm_lifecycle_test.go, and for the same
// reason: registry.go itself must not be edited by this file (see
// generated_authorization.go's top doc comment).
func newAuthorizationRegistry(ctx context.Context, c *vmware.Client, opts RegistryOptions) *Registry {
	r := NewRegistry(ctx, c, opts)
	r.withClass(modeVSphereGeneral, registerAuthorizationTools)
	return r
}

// TestAuthorizationTools_Registration proves all 14 tools are registered and
// reachable via ListTools — a basic wiring smoke test before the more
// specific behavioral tests below. Run against simulator.VPX() (not ESX())
// even though AuthorizationManager is mode=vsphere-general/safe on both —
// VPX's richer default inventory (multiple hosts/a datacenter) is more
// useful for the entity-scoped tests further down in this file.
func TestAuthorizationTools_Registration(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newAuthorizationRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	want := []string{
		"vmware_authorization_add_role",
		"vmware_authorization_disable_methods",
		"vmware_authorization_enable_methods",
		"vmware_authorization_fetch_user_privilege_on_entities",
		"vmware_authorization_has_privilege_on_entity",
		"vmware_authorization_has_user_privilege_on_entities",
		"vmware_authorization_remove_entity_permission",
		"vmware_authorization_remove_role",
		"vmware_authorization_retrieve_all_permissions",
		"vmware_authorization_retrieve_entity_permissions",
		"vmware_authorization_retrieve_role_permissions",
		"vmware_authorization_role_list",
		"vmware_authorization_set_entity_permissions",
		"vmware_authorization_update_role",
	}
	if len(want) != 14 {
		t.Fatalf("test bug: want list has %d entries, expected 14", len(want))
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

// TestAuthorizationTools_RoleLifecycle exercises AddRole -> RoleList ->
// UpdateRole -> RetrieveRolePermissions -> RemoveRole end to end against a
// real vcsim AuthorizationManager.
func TestAuthorizationTools_RoleLifecycle(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newAuthorizationRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	raw, err := r.CallTool("vmware_authorization_add_role", map[string]interface{}{
		"name":          "TestRole",
		"privilege_ids": []interface{}{"System.View"},
		"confirm":       true,
	})
	if err != nil {
		t.Fatalf("vmware_authorization_add_role failed: %v (%s)", err, raw)
	}
	m := decodeResult(t, raw)
	roleID, ok := m["role_id"].(float64)
	if !ok {
		t.Fatalf("expected numeric role_id in %s", raw)
	}

	raw, err = r.CallTool("vmware_authorization_role_list", map[string]interface{}{})
	if err != nil {
		t.Fatalf("vmware_authorization_role_list failed: %v", err)
	}
	if !roleListHasName(t, raw, "TestRole") {
		t.Fatalf("expected TestRole in role list: %s", raw)
	}

	raw, err = r.CallTool("vmware_authorization_update_role", map[string]interface{}{
		"role_id": roleID,
		"name":    "TestRoleRenamed",
		"confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_authorization_update_role failed: %v (%s)", err, raw)
	}

	raw, err = r.CallTool("vmware_authorization_role_list", map[string]interface{}{})
	if err != nil {
		t.Fatalf("vmware_authorization_role_list (after rename) failed: %v", err)
	}
	if !roleListHasName(t, raw, "TestRoleRenamed") {
		t.Fatalf("expected TestRoleRenamed after rename: %s", raw)
	}

	raw, err = r.CallTool("vmware_authorization_retrieve_role_permissions", map[string]interface{}{"role_id": roleID})
	if err != nil {
		t.Fatalf("vmware_authorization_retrieve_role_permissions failed: %v", err)
	}
	if count, _ := decodeResult(t, raw)["count"].(float64); count != 0 {
		t.Fatalf("expected 0 permissions for a role nobody was assigned, got %v: %s", count, raw)
	}

	// fail_if_used=true — note: vcsim's simulator.AuthorizationManager.
	// RemoveAuthorizationRole does not actually enforce FailIfUsed
	// (confirmed by reading referencia/govmomi/simulator/authorization_manager.go
	// — it removes unconditionally regardless of the request field), so this
	// only proves the tool round-trips and the role is gone afterward, not
	// that fail_if_used is enforced server-side by vcsim.
	raw, err = r.CallTool("vmware_authorization_remove_role", map[string]interface{}{
		"role_id":      roleID,
		"fail_if_used": true,
		"confirm":      true,
	})
	if err != nil {
		t.Fatalf("vmware_authorization_remove_role failed: %v (%s)", err, raw)
	}

	raw, err = r.CallTool("vmware_authorization_role_list", map[string]interface{}{})
	if err != nil {
		t.Fatalf("vmware_authorization_role_list (after remove) failed: %v", err)
	}
	if roleListHasName(t, raw, "TestRoleRenamed") {
		t.Fatalf("expected TestRoleRenamed to be removed, still present: %s", raw)
	}
}

func roleListHasName(t *testing.T, raw string, name string) bool {
	t.Helper()
	roles, _ := decodeResult(t, raw)["roles"].([]interface{})
	for _, item := range roles {
		roleMap, _ := item.(map[string]interface{})
		if roleMap["name"] == name {
			return true
		}
	}
	return false
}

// TestAuthorizationTools_EntityPermissions exercises RetrieveEntityPermissions/
// RetrieveAllPermissions/SetEntityPermissions/RemoveEntityPermission against
// real inventory entities (the root folder and the first VM).
func TestAuthorizationTools_EntityPermissions(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newAuthorizationRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	// The root folder ("/") carries a default Admin permission for vcsim's
	// DefaultUserGroup principal — confirmed by reading
	// referencia/govmomi/simulator/authorization_manager.go's init(), which
	// assigns m.permissions[root] unconditionally on startup.
	raw, err := r.CallTool("vmware_authorization_retrieve_entity_permissions", map[string]interface{}{"entity_path": "/"})
	if err != nil {
		t.Fatalf(`vmware_authorization_retrieve_entity_permissions("/") failed: %v`, err)
	}
	if count, _ := decodeResult(t, raw)["count"].(float64); count <= 0 {
		t.Fatalf("expected at least 1 default permission on root, got %v: %s", count, raw)
	}

	raw, err = r.CallTool("vmware_authorization_retrieve_all_permissions", map[string]interface{}{})
	if err != nil {
		t.Fatalf("vmware_authorization_retrieve_all_permissions failed: %v", err)
	}
	if count, _ := decodeResult(t, raw)["count"].(float64); count <= 0 {
		t.Fatalf("expected at least 1 permission across the inventory, got %v: %s", count, raw)
	}

	vm := firstVMPath(t, r)
	raw, err = r.CallTool("vmware_authorization_set_entity_permissions", map[string]interface{}{
		"entity_path": vm,
		"permissions": []interface{}{
			map[string]interface{}{"principal": "test-user", "group": false, "roleId": -1, "propagate": true},
		},
		"confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_authorization_set_entity_permissions failed: %v (%s)", err, raw)
	}

	raw, err = r.CallTool("vmware_authorization_retrieve_entity_permissions", map[string]interface{}{"entity_path": vm})
	if err != nil {
		t.Fatalf("vmware_authorization_retrieve_entity_permissions(vm) failed: %v", err)
	}
	if !permissionsHavePrincipal(t, raw, "test-user") {
		t.Fatalf("expected test-user permission on %s, got %s", vm, raw)
	}

	raw, err = r.CallTool("vmware_authorization_remove_entity_permission", map[string]interface{}{
		"entity_path": vm,
		"user":        "test-user",
		"is_group":    false,
		"confirm":     true,
	})
	if err != nil {
		t.Fatalf("vmware_authorization_remove_entity_permission failed: %v (%s)", err, raw)
	}

	raw, err = r.CallTool("vmware_authorization_retrieve_entity_permissions", map[string]interface{}{"entity_path": vm})
	if err != nil {
		t.Fatalf("vmware_authorization_retrieve_entity_permissions(vm, after remove) failed: %v", err)
	}
	if permissionsHavePrincipal(t, raw, "test-user") {
		t.Fatalf("expected test-user permission to be removed, still present: %s", raw)
	}
}

func permissionsHavePrincipal(t *testing.T, raw string, principal string) bool {
	t.Helper()
	perms, _ := decodeResult(t, raw)["permissions"].([]interface{})
	for _, item := range perms {
		p, _ := item.(map[string]interface{})
		if p["principal"] == principal {
			return true
		}
	}
	return false
}

// TestAuthorizationTools_PrivilegeChecks exercises the 3 read-only
// "does X have privilege Y" tools. vcsim's simulator.AuthorizationManager
// always answers these with a fixed "granted" shape (see
// HasPrivilegeOnEntity/HasUserPrivilegeOnEntities/FetchUserPrivilegeOnEntities
// in referencia/govmomi/simulator/authorization_manager.go — none of them
// actually check the caller's real privileges), so this only proves the
// tools round-trip real requests/responses shaped correctly, not that
// privilege enforcement itself works.
func TestAuthorizationTools_PrivilegeChecks(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newAuthorizationRegistry(context.Background(), c, RegistryOptions{})
	vm := firstVMPath(t, r)

	raw, err := r.CallTool("vmware_authorization_has_privilege_on_entity", map[string]interface{}{
		"entity_path":   vm,
		"privilege_ids": []interface{}{"System.View"},
	})
	if err != nil {
		t.Fatalf("vmware_authorization_has_privilege_on_entity failed: %v", err)
	}
	granted, _ := decodeResult(t, raw)["granted"].([]interface{})
	if len(granted) != 1 {
		t.Fatalf("expected 1 granted entry, got %s", raw)
	}

	raw, err = r.CallTool("vmware_authorization_has_user_privilege_on_entities", map[string]interface{}{
		"entity_paths":  []interface{}{vm},
		"user_name":     "test-user",
		"privilege_ids": []interface{}{"System.View"},
	})
	if err != nil {
		t.Fatalf("vmware_authorization_has_user_privilege_on_entities failed: %v", err)
	}
	if count, _ := decodeResult(t, raw)["count"].(float64); count != 1 {
		t.Fatalf("expected count 1, got %v: %s", count, raw)
	}

	raw, err = r.CallTool("vmware_authorization_fetch_user_privilege_on_entities", map[string]interface{}{
		"entity_paths": []interface{}{vm},
		"user_name":    "test-user",
	})
	if err != nil {
		t.Fatalf("vmware_authorization_fetch_user_privilege_on_entities failed: %v", err)
	}
	if count, _ := decodeResult(t, raw)["count"].(float64); count != 1 {
		t.Fatalf("expected count 1, got %v: %s", count, raw)
	}
}

// TestAuthorizationTools_EntityResolutionFailure proves resolveEntityRef
// fails cleanly (not a panic) for an inventory path that does not resolve
// to any managed entity.
func TestAuthorizationTools_EntityResolutionFailure(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newAuthorizationRegistry(context.Background(), c, RegistryOptions{})

	if _, err := r.CallTool("vmware_authorization_retrieve_entity_permissions", map[string]interface{}{
		"entity_path": "/DC0/vm/does-not-exist",
	}); err == nil {
		t.Fatal("expected vmware_authorization_retrieve_entity_permissions to fail for a nonexistent inventory path")
	}
}

// TestAuthorizationTools_GateAndConfirm proves the real Tier 1/2 tools in
// this file are wired through registerDestructive, the same 3-layer
// protection check pattern as generated_network_test.go's
// TestNetworkTools_GateAndConfirm.
func TestAuthorizationTools_GateAndConfirm(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	closedGate := newAuthorizationRegistry(context.Background(), c, RegistryOptions{AllowDestructive: false})
	if _, err := closedGate.CallTool("vmware_authorization_add_role", map[string]interface{}{
		"name": "ShouldNotExist", "confirm": true,
	}); err == nil {
		t.Fatal("expected vmware_authorization_add_role to be denied with the gate closed")
	}

	openGate := newAuthorizationRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	if _, err := openGate.CallTool("vmware_authorization_add_role", map[string]interface{}{
		"name": "ShouldNotExist",
	}); err == nil {
		t.Fatal("expected vmware_authorization_add_role to fail without confirm:true")
	}
}

// TestAuthorizationTools_InternalMethodsReachServer proves
// vmware_authorization_disable_methods/vmware_authorization_enable_methods
// reach vcsim cleanly instead of failing on something wired wrong in this
// file — see generated_authorization.go's top doc comment: neither
// DisableMethods nor EnableMethods (the "urn:internalvim25" internal API)
// has a vcsim handler, so a real call is expected to fail, but it must fail
// with the server's own response, not "unknown tool" (registration broken)
// or a recovered panic (handler bug). Uses assertReachesServer from
// generated_vm_lifecycle_test.go, same helper/discipline as
// generated_network_test.go's TestNetworkTools_UnsimulatedMethods.
func TestAuthorizationTools_InternalMethodsReachServer(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newAuthorizationRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	_, err := r.CallTool("vmware_authorization_disable_methods", map[string]interface{}{
		"entity_paths": []interface{}{"/"},
		"methods":      []interface{}{map[string]interface{}{"method": "PowerOnVM_Task", "reason": "test"}},
		"source":       "test",
		"confirm":      true,
	})
	assertReachesServer(t, err, "vmware_authorization_disable_methods")

	_, err = r.CallTool("vmware_authorization_enable_methods", map[string]interface{}{
		"entity_paths": []interface{}{"/"},
		"methods":      []interface{}{"PowerOnVM_Task"},
		"source":       "test",
		"confirm":      true,
	})
	assertReachesServer(t, err, "vmware_authorization_enable_methods")
}

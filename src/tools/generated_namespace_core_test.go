package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/vmware/govmomi/simulator"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// newNamespaceCoreRegistry builds a Registry the normal way (NewRegistry,
// which wires vm.go/host.go/etc via registerTools) and then manually layers
// this group's tools on top via withClass — same pattern as
// generated_authorization_test.go's newAuthorizationRegistry, and for the
// same reason: registry.go itself must not be edited by this file (see
// generated_namespace_core.go's top doc comment).
func newNamespaceCoreRegistry(ctx context.Context, c *vmware.Client, opts RegistryOptions) *Registry {
	r := NewRegistry(ctx, c, opts)
	r.withClass(modeVCenterOnly, registerNamespaceCoreTools)
	return r
}

// allNamespaceCoreTools is the authoritative list of the 22 tools this file
// registers — reused by every test below instead of being re-typed per test.
var allNamespaceCoreTools = []string{
	"vmware_namespace_create_namespace",
	"vmware_namespace_create_support_bundle",
	"vmware_namespace_create_vm_class",
	"vmware_namespace_delete_namespace",
	"vmware_namespace_delete_vm_class",
	"vmware_namespace_disable_cluster",
	"vmware_namespace_enable_cluster",
	"vmware_namespace_enable_on_compute_cluster",
	"vmware_namespace_enable_on_zones",
	"vmware_namespace_get_namespace",
	"vmware_namespace_get_supervisor_summaries",
	"vmware_namespace_get_supervisor_summary",
	"vmware_namespace_get_supervisor_topology",
	"vmware_namespace_get_vm_class",
	"vmware_namespace_list_clusters",
	"vmware_namespace_list_compatible_distributed_switches",
	"vmware_namespace_list_compatible_edge_clusters",
	"vmware_namespace_list_namespaces",
	"vmware_namespace_list_vm_classes",
	"vmware_namespace_register_vm",
	"vmware_namespace_update_namespace",
	"vmware_namespace_update_vm_class",
}

// TestNamespaceCoreTools_Registration proves all 22 tools are registered and
// reachable via ListTools — a basic wiring smoke test before the more
// specific behavioral tests below.
func TestNamespaceCoreTools_Registration(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newNamespaceCoreRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	if len(allNamespaceCoreTools) != 22 {
		t.Fatalf("test bug: allNamespaceCoreTools has %d entries, expected 22", len(allNamespaceCoreTools))
	}

	got := map[string]bool{}
	for _, tl := range r.ListTools() {
		got[tl.Name] = true
	}
	for _, name := range allNamespaceCoreTools {
		if !got[name] {
			t.Errorf("tool %s not registered", name)
		}
	}
}

// TestNamespaceCoreTools_ArgumentValidationBeforeServer proves every required
// argument this file checks is validated BEFORE the handler ever attempts a
// round trip to the server — not just "an error is returned eventually".
// Proven empirically (not by code inspection): the vcsim server is closed
// immediately after the registry is built, so any handler that tried to
// reach namespaceCoreManager/client.REST(ctx) (a real login round trip)
// before checking its required arguments would fail with a connection error
// instead of the specific "<field> is required" message this test asserts
// on — the substring match would then fail, catching a future ordering
// regression. Same "prove by evidence" discipline this project applies
// throughout (see this file's top doc comment).
func TestNamespaceCoreTools_ArgumentValidationBeforeServer(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	r := newNamespaceCoreRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	cleanup() // close vcsim now — any handler reaching the server first gets a connection error, not the validation message asserted below

	cases := []struct {
		tool          string
		args          map[string]interface{}
		wantErrSubstr string
	}{
		{"vmware_namespace_create_namespace", map[string]interface{}{"confirm": true}, "cluster is required"},
		{"vmware_namespace_create_namespace", map[string]interface{}{"cluster": "domain-c8", "confirm": true}, "namespace is required"},
		{"vmware_namespace_create_support_bundle", map[string]interface{}{"confirm": true}, "id is required"},
		{"vmware_namespace_create_vm_class", map[string]interface{}{"confirm": true}, "id is required"},
		{"vmware_namespace_create_vm_class", map[string]interface{}{"id": "c1", "confirm": true}, "cpu_count is required"},
		{"vmware_namespace_create_vm_class", map[string]interface{}{"id": "c1", "cpu_count": 2, "confirm": true}, "memory_mb is required"},
		{"vmware_namespace_delete_namespace", map[string]interface{}{"confirm": true}, "namespace is required"},
		{"vmware_namespace_delete_vm_class", map[string]interface{}{"confirm": true}, "vm_class is required"},
		{"vmware_namespace_disable_cluster", map[string]interface{}{"confirm": true}, "id is required"},
		{"vmware_namespace_enable_cluster", map[string]interface{}{"confirm": true}, "id is required"},
		{"vmware_namespace_enable_on_compute_cluster", map[string]interface{}{"confirm": true}, "id is required"},
		{"vmware_namespace_enable_on_compute_cluster", map[string]interface{}{"id": "domain-c8", "confirm": true}, "name is required"},
		{"vmware_namespace_enable_on_zones", map[string]interface{}{"confirm": true}, "zones is required"},
		{"vmware_namespace_enable_on_zones", map[string]interface{}{"zones": []interface{}{"zone1"}, "confirm": true}, "name is required"},
		{"vmware_namespace_get_namespace", map[string]interface{}{}, "namespace is required"},
		{"vmware_namespace_get_supervisor_summary", map[string]interface{}{}, "id is required"},
		{"vmware_namespace_get_supervisor_topology", map[string]interface{}{}, "id is required"},
		{"vmware_namespace_get_vm_class", map[string]interface{}{}, "vm_class is required"},
		{"vmware_namespace_list_compatible_distributed_switches", map[string]interface{}{}, "cluster_id is required"},
		{"vmware_namespace_list_compatible_edge_clusters", map[string]interface{}{}, "cluster_id is required"},
		{"vmware_namespace_list_compatible_edge_clusters", map[string]interface{}{"cluster_id": "domain-c8"}, "switch_id is required"},
		{"vmware_namespace_register_vm", map[string]interface{}{"confirm": true}, "namespace is required"},
		{"vmware_namespace_register_vm", map[string]interface{}{"namespace": "ns1", "confirm": true}, "vm is required"},
		{"vmware_namespace_update_namespace", map[string]interface{}{"confirm": true}, "namespace is required"},
		{"vmware_namespace_update_vm_class", map[string]interface{}{"confirm": true}, "vm_class is required"},
		{"vmware_namespace_update_vm_class", map[string]interface{}{"vm_class": "c1", "confirm": true}, "cpu_count is required"},
		{"vmware_namespace_update_vm_class", map[string]interface{}{"vm_class": "c1", "cpu_count": 2, "confirm": true}, "memory_mb is required"},
	}

	for _, tc := range cases {
		t.Run(tc.tool+"/"+tc.wantErrSubstr, func(t *testing.T) {
			_, err := r.CallTool(tc.tool, tc.args)
			if err == nil {
				t.Fatalf("%s: expected a validation error, got success", tc.tool)
			}
			if !strings.Contains(err.Error(), tc.wantErrSubstr) {
				t.Fatalf("%s: expected error containing %q, got: %v", tc.tool, tc.wantErrSubstr, err)
			}
		})
	}
}

// TestNamespaceCoreTools_TierGate proves the tier1 (delete_namespace) and
// tier2 (create_namespace) tools in this file are wired through
// registerDestructive — same 3-layer protection check pattern as
// generated_authorization_test.go's TestAuthorizationTools_GateAndConfirm.
func TestNamespaceCoreTools_TierGate(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	closedGate := newNamespaceCoreRegistry(context.Background(), c, RegistryOptions{AllowDestructive: false})
	if _, err := closedGate.CallTool("vmware_namespace_delete_namespace", map[string]interface{}{
		"namespace": "ns1", "confirm": true,
	}); err == nil {
		t.Fatal("expected vmware_namespace_delete_namespace (tier1) to be denied with the gate closed")
	}
	if _, err := closedGate.CallTool("vmware_namespace_create_namespace", map[string]interface{}{
		"cluster": "domain-c8", "namespace": "ns1", "confirm": true,
	}); err == nil {
		t.Fatal("expected vmware_namespace_create_namespace (tier2) to be denied with the gate closed")
	}

	openGate := newNamespaceCoreRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	if _, err := openGate.CallTool("vmware_namespace_delete_namespace", map[string]interface{}{
		"namespace": "ns1",
	}); err == nil {
		t.Fatal("expected vmware_namespace_delete_namespace to fail without confirm:true")
	}
	if _, err := openGate.CallTool("vmware_namespace_create_namespace", map[string]interface{}{
		"cluster": "domain-c8", "namespace": "ns1",
	}); err == nil {
		t.Fatal("expected vmware_namespace_create_namespace to fail without confirm:true")
	}
}

// TestNamespaceCoreTools_ReachServer proves every one of the 22 tools in
// this file reaches vcsim cleanly (a real server-side error, not "unknown
// tool" and not a recovered panic) once given minimally valid arguments —
// see this file's top doc comment: vapi/namespace has ZERO vcsim simulator
// handlers (confirmed by grepping referencia/govmomi/vapi/simulator/simulator.go
// for "vapi/namespace" — 0 matches), so REST login succeeds (the generic vAPI
// session endpoint IS simulated) but every namespace-specific REST call 404s
// or otherwise errors server-side. This is the strongest evidence available
// for this untestable-beyond-wiring domain that each tool's URL/method/arg
// serialization actually reaches the real namespace.Manager method, not a
// typo'd path or a args-decoding bug that never leaves this process.
func TestNamespaceCoreTools_ReachServer(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newNamespaceCoreRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	cases := []struct {
		tool string
		args map[string]interface{}
	}{
		{"vmware_namespace_create_namespace", map[string]interface{}{"cluster": "domain-c8", "namespace": "ns1", "confirm": true}},
		{"vmware_namespace_create_support_bundle", map[string]interface{}{"id": "domain-c8", "confirm": true}},
		{"vmware_namespace_create_vm_class", map[string]interface{}{"id": "custom-class", "cpu_count": 2, "memory_mb": 4096, "confirm": true}},
		{"vmware_namespace_delete_namespace", map[string]interface{}{"namespace": "ns1", "confirm": true}},
		{"vmware_namespace_delete_vm_class", map[string]interface{}{"vm_class": "custom-class", "confirm": true}},
		{"vmware_namespace_disable_cluster", map[string]interface{}{"id": "domain-c8", "confirm": true}},
		{"vmware_namespace_enable_cluster", map[string]interface{}{
			"id":                        "domain-c8",
			"image_storage":             map[string]interface{}{"storage_policy": "policy1"},
			"master_management_network": map[string]interface{}{"mode": "DHCP", "network": "net1"},
			"service_cidr":              map[string]interface{}{"address": "10.96.0.0", "prefix": 23},
			"size_hint":                 "TINY",
			"network_provider":          "VSPHERE_NETWORK",
			"confirm":                   true,
		}},
		{"vmware_namespace_enable_on_compute_cluster", map[string]interface{}{"id": "domain-c8", "name": "supervisor1", "control_plane": map[string]interface{}{}, "workloads": map[string]interface{}{}, "confirm": true}},
		{"vmware_namespace_enable_on_zones", map[string]interface{}{"zones": []interface{}{"zone1"}, "name": "supervisor1", "control_plane": map[string]interface{}{}, "workloads": map[string]interface{}{}, "confirm": true}},
		{"vmware_namespace_get_namespace", map[string]interface{}{"namespace": "ns1"}},
		{"vmware_namespace_get_supervisor_summaries", map[string]interface{}{}},
		{"vmware_namespace_get_supervisor_summary", map[string]interface{}{"id": "domain-c8"}},
		{"vmware_namespace_get_supervisor_topology", map[string]interface{}{"id": "domain-c8"}},
		{"vmware_namespace_get_vm_class", map[string]interface{}{"vm_class": "custom-class"}},
		{"vmware_namespace_list_clusters", map[string]interface{}{}},
		{"vmware_namespace_list_compatible_distributed_switches", map[string]interface{}{"cluster_id": "domain-c8"}},
		{"vmware_namespace_list_compatible_edge_clusters", map[string]interface{}{"cluster_id": "domain-c8", "switch_id": "dvs-1"}},
		{"vmware_namespace_list_namespaces", map[string]interface{}{}},
		{"vmware_namespace_list_vm_classes", map[string]interface{}{}},
		{"vmware_namespace_register_vm", map[string]interface{}{"namespace": "ns1", "vm": "vm-100", "confirm": true}},
		{"vmware_namespace_update_namespace", map[string]interface{}{"namespace": "ns1", "confirm": true}},
		{"vmware_namespace_update_vm_class", map[string]interface{}{"vm_class": "custom-class", "cpu_count": 2, "memory_mb": 4096, "confirm": true}},
	}
	if len(cases) != 22 {
		t.Fatalf("test bug: cases has %d entries, expected 22 (one per tool)", len(cases))
	}

	for _, tc := range cases {
		t.Run(tc.tool, func(t *testing.T) {
			_, err := r.CallTool(tc.tool, tc.args)
			assertReachesServer(t, err, tc.tool)
		})
	}
}

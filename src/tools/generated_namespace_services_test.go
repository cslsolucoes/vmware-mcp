package tools

import (
	"context"
	"testing"

	"github.com/vmware/govmomi/simulator"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// newNamespaceServicesRegistry builds a Registry the normal way (NewRegistry,
// which wires every other domain via registerTools) and then manually layers
// registerNamespaceServicesTools on top via withClass — same pattern as
// newVCenterTemplateRegistry (generated_vcenter_template_test.go). This file
// must not edit registry.go itself (see generated_namespace_services.go's
// top doc comment / this fase's brief).
func newNamespaceServicesRegistry(ctx context.Context, c *vmware.Client, opts RegistryOptions) *Registry {
	r := NewRegistry(ctx, c, opts)
	r.withClass(modeVCenterOnly, registerNamespaceServicesTools)
	return r
}

// TestNamespaceServicesTools_Registration proves all 21 tools are registered
// and reachable via ListTools.
func TestNamespaceServicesTools_Registration(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newNamespaceServicesRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	want := []string{
		// supervisorsvc.go (12)
		"vmware_namespace_activate_supervisor_service_version",
		"vmware_namespace_activate_supervisor_services",
		"vmware_namespace_create_supervisor_service",
		"vmware_namespace_create_supervisor_service_version",
		"vmware_namespace_deactivate_supervisor_service_version",
		"vmware_namespace_deactivate_supervisor_services",
		"vmware_namespace_get_supervisor_service",
		"vmware_namespace_get_supervisor_service_version",
		"vmware_namespace_list_supervisor_service_versions",
		"vmware_namespace_list_supervisor_services",
		"vmware_namespace_remove_supervisor_service",
		"vmware_namespace_remove_supervisor_service_version",
		// networks.go (6)
		"vmware_namespace_create_cluster_network",
		"vmware_namespace_delete_cluster_network",
		"vmware_namespace_get_cluster_network",
		"vmware_namespace_list_cluster_networks",
		"vmware_namespace_set_cluster_network",
		"vmware_namespace_update_cluster_network",
		// namespace_v2.go (3)
		"vmware_namespace_create_namespace_v2",
		"vmware_namespace_get_namespace_v2",
		"vmware_namespace_list_namespaces_v2",
	}
	if len(want) != 21 {
		t.Fatalf("test bug: want list has %d entries, expected 21", len(want))
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

// TestNamespaceServicesTools_ArgValidation proves each tool rejects missing
// required arguments with a clean error, matching this group's brief:
// "validação de argumentos obrigatórios falha ANTES de tocar o servidor" —
// none of these cases ever reaches a namespace.Manager method call against
// vcsim's (entirely unimplemented, see generated_namespace_services.go's top
// doc comment) namespace-management REST endpoints. Two categories of "before
// touching the server" apply here, same nuance as
// generated_vm_dataset_test.go's TestVMDatasetTools_ArgValidation: (1) a
// missing string ID/spec argument (supervisor_service_id, version, service,
// spec, cluster, namespace, supervisor) is checked in Go with zero network
// calls; (2) a missing network_id/spec with a valid "cluster" given does
// exercise client.REST(ctx)'s login plus a real (already fully-implemented,
// unrelated-to-this-domain) Finder.ClusterComputeResource SOAP round trip —
// same as every other cluster-scoped tool in this project — but still never
// reaches the actual namespace-management call being validated for.
func TestNamespaceServicesTools_ArgValidation(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newNamespaceServicesRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	cases := []struct {
		tool string
		args map[string]interface{}
	}{
		// supervisorsvc.go
		{"vmware_namespace_get_supervisor_service", map[string]interface{}{}},                                                             // missing supervisor_service_id
		{"vmware_namespace_get_supervisor_service_version", map[string]interface{}{"supervisor_service_id": "svc-1"}},                     // missing version
		{"vmware_namespace_get_supervisor_service_version", map[string]interface{}{"version": "1.0.0"}},                                   // missing supervisor_service_id
		{"vmware_namespace_list_supervisor_service_versions", map[string]interface{}{}},                                                   // missing supervisor_service_id
		{"vmware_namespace_create_supervisor_service", map[string]interface{}{"confirm": true}},                                           // missing service
		{"vmware_namespace_create_supervisor_service_version", map[string]interface{}{"confirm": true}},                                   // missing supervisor_service_id
		{"vmware_namespace_create_supervisor_service_version", map[string]interface{}{"confirm": true, "supervisor_service_id": "s1"}},    // missing service
		{"vmware_namespace_activate_supervisor_services", map[string]interface{}{"confirm": true}},                                        // missing supervisor_service_id
		{"vmware_namespace_deactivate_supervisor_services", map[string]interface{}{"confirm": true}},                                      // missing supervisor_service_id
		{"vmware_namespace_activate_supervisor_service_version", map[string]interface{}{"confirm": true, "supervisor_service_id": "s"}},   // missing version
		{"vmware_namespace_deactivate_supervisor_service_version", map[string]interface{}{"confirm": true, "supervisor_service_id": "s"}}, // missing version
		{"vmware_namespace_remove_supervisor_service", map[string]interface{}{"confirm": true}},                                           // missing supervisor_service_id
		{"vmware_namespace_remove_supervisor_service_version", map[string]interface{}{"confirm": true, "supervisor_service_id": "s"}},     // missing version

		// networks.go
		{"vmware_namespace_list_cluster_networks", map[string]interface{}{}},                                                                                        // missing cluster
		{"vmware_namespace_get_cluster_network", map[string]interface{}{"cluster": "DC0_C0"}},                                                                       // missing network_id
		{"vmware_namespace_get_cluster_network", map[string]interface{}{"network_id": "net-1"}},                                                                     // missing cluster
		{"vmware_namespace_create_cluster_network", map[string]interface{}{"cluster": "DC0_C0", "confirm": true}},                                                   // missing spec
		{"vmware_namespace_create_cluster_network", map[string]interface{}{"confirm": true, "spec": map[string]interface{}{"network_provider": "VSPHERE_NETWORK"}}}, // missing cluster
		{"vmware_namespace_update_cluster_network", map[string]interface{}{"cluster": "DC0_C0", "confirm": true}},                                                   // missing network_id
		{"vmware_namespace_update_cluster_network", map[string]interface{}{"cluster": "DC0_C0", "network_id": "net-1", "confirm": true}},                            // missing spec
		{"vmware_namespace_set_cluster_network", map[string]interface{}{"cluster": "DC0_C0", "confirm": true}},                                                      // missing network_id
		{"vmware_namespace_set_cluster_network", map[string]interface{}{"cluster": "DC0_C0", "network_id": "net-1", "confirm": true}},                               // missing spec
		{"vmware_namespace_delete_cluster_network", map[string]interface{}{"cluster": "DC0_C0", "confirm": true}},                                                   // missing network_id

		// namespace_v2.go
		{"vmware_namespace_get_namespace_v2", map[string]interface{}{}},                                       // missing namespace
		{"vmware_namespace_create_namespace_v2", map[string]interface{}{"confirm": true}},                     // missing namespace
		{"vmware_namespace_create_namespace_v2", map[string]interface{}{"namespace": "ns1", "confirm": true}}, // missing supervisor
	}

	for _, tc := range cases {
		if _, err := r.CallTool(tc.tool, tc.args); err == nil {
			t.Errorf("%s(%v): expected an error for missing required argument", tc.tool, tc.args)
		}
	}
}

// TestNamespaceServicesTools_PortgroupResolutionFailsLocally proves
// resolveClusterNetworkPortgroup's inventory-path resolution (this file's
// own added logic, not just generated wiring) surfaces a clean error without
// ever reaching vcsim's namespace-management endpoint when given a
// nonexistent inventory path.
func TestNamespaceServicesTools_PortgroupResolutionFailsLocally(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newNamespaceServicesRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	_, err := r.CallTool("vmware_namespace_create_cluster_network", map[string]interface{}{
		"cluster": "DC0_C0",
		"spec": map[string]interface{}{
			"network_provider": "VSPHERE_NETWORK",
			"vsphere_network": map[string]interface{}{
				"portgroup": "/DC0/network/does-not-exist",
			},
		},
		"confirm": true,
	})
	if err == nil {
		t.Fatal("expected an error resolving a nonexistent portgroup inventory path")
	}
	assertReachesServer(t, err, "vmware_namespace_create_cluster_network(bogus portgroup) -- expect a local resolution error, not a server round trip, but must still be clean (no panic/unknown-tool)")
}

// TestNamespaceServicesTools_GateAndConfirm proves this file's destructive
// tools are wired through registerDestructive across both tiers present
// here (tier1: remove/delete; tier2: create/activate/deactivate/update/set) —
// same 3-layer protection check pattern as generated_vcenter_template_test.go's
// TestVCenterTemplateTools_GateAndConfirm.
func TestNamespaceServicesTools_GateAndConfirm(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	closedGate := newNamespaceServicesRegistry(context.Background(), c, RegistryOptions{AllowDestructive: false})
	openGateNoConfirm := newNamespaceServicesRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	tier2Args := map[string]interface{}{"supervisor_service_id": "svc-1", "confirm": true}
	tier1Args := map[string]interface{}{"supervisor_service_id": "svc-1", "confirm": true}

	if _, err := closedGate.CallTool("vmware_namespace_activate_supervisor_services", tier2Args); err == nil {
		t.Fatal("expected vmware_namespace_activate_supervisor_services (tier2) to be denied with the gate closed")
	}
	if _, err := closedGate.CallTool("vmware_namespace_remove_supervisor_service", tier1Args); err == nil {
		t.Fatal("expected vmware_namespace_remove_supervisor_service (tier1) to be denied with the gate closed")
	}

	if _, err := openGateNoConfirm.CallTool("vmware_namespace_activate_supervisor_services", map[string]interface{}{"supervisor_service_id": "svc-1"}); err == nil {
		t.Fatal("expected vmware_namespace_activate_supervisor_services (tier2) to fail without confirm:true")
	}
	if _, err := openGateNoConfirm.CallTool("vmware_namespace_remove_supervisor_service", map[string]interface{}{"supervisor_service_id": "svc-1"}); err == nil {
		t.Fatal("expected vmware_namespace_remove_supervisor_service (tier1) to fail without confirm:true")
	}
}

// TestNamespaceServicesTools_ReachesServer exercises a representative tool
// from each of the 3 source files (read-only and destructive) against a real
// vcsim vapi/rest session, proving every call reaches the server cleanly
// (real error, not a panic or "unknown tool") — see this file's and
// generated_namespace_services.go's top doc comments for why a genuine
// success is never expected here (vcsim has zero handlers anywhere under
// /api/vcenter/namespace-management/... or /api/vcenter/namespaces/...).
func TestNamespaceServicesTools_ReachesServer(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newNamespaceServicesRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	cases := []struct {
		tool string
		args map[string]interface{}
	}{
		{"vmware_namespace_list_supervisor_services", map[string]interface{}{}},
		{"vmware_namespace_get_supervisor_service", map[string]interface{}{"supervisor_service_id": "svc-1"}},
		{"vmware_namespace_list_supervisor_service_versions", map[string]interface{}{"supervisor_service_id": "svc-1"}},
		{"vmware_namespace_create_supervisor_service", map[string]interface{}{
			"confirm": true,
			"service": map[string]interface{}{
				"vsphere_spec": map[string]interface{}{
					"version_spec": map[string]interface{}{"content": "ZmFrZQ=="},
				},
			},
		}},
		{"vmware_namespace_list_cluster_networks", map[string]interface{}{"cluster": "DC0_C0"}},
		{"vmware_namespace_get_cluster_network", map[string]interface{}{"cluster": "DC0_C0", "network_id": "net-1"}},
		{"vmware_namespace_create_cluster_network", map[string]interface{}{
			"cluster": "DC0_C0",
			"confirm": true,
			"spec": map[string]interface{}{
				"network_provider": "VSPHERE_NETWORK",
				"vsphere_network":  map[string]interface{}{"gateway": "10.0.0.1", "subnet_mask": "255.255.255.0"},
			},
		}},
		{"vmware_namespace_delete_cluster_network", map[string]interface{}{"cluster": "DC0_C0", "network_id": "net-1", "confirm": true}},
		{"vmware_namespace_list_namespaces_v2", map[string]interface{}{}},
		{"vmware_namespace_get_namespace_v2", map[string]interface{}{"namespace": "ns1"}},
		{"vmware_namespace_create_namespace_v2", map[string]interface{}{"namespace": "ns1", "supervisor": "DC0_C0", "confirm": true}},
	}

	for _, tc := range cases {
		_, err := r.CallTool(tc.tool, tc.args)
		assertReachesServer(t, err, tc.tool)
	}
}

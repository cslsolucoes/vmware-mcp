package tools

import (
	"context"
	"testing"

	"github.com/vmware/govmomi/simulator"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// newInventoryComputeRegistry builds a Registry the normal way (NewRegistry,
// which wires every existing domain via registerTools) and then layers this
// group's 2 register functions on top via withClass — same pattern as
// generated_vm_lifecycle_test.go's newLifecycleRegistry. Must not edit
// registry.go itself (see generated_inventory_compute.go's top doc comment).
func newInventoryComputeRegistry(ctx context.Context, c *vmware.Client, opts RegistryOptions) *Registry {
	r := NewRegistry(ctx, c, opts)
	r.withClass(modeVSphereGeneral, registerInventoryComputeTools)
	r.withClass(modeVCenterOnly, registerInventoryComputeVCenterOnlyTools)
	return r
}

// TestInventoryComputeTools_StandaloneESXi proves the ComputeResource +
// EnvironmentBrowser tools work against a standalone ESXi host's implicit
// compute resource (simulator.ESX()) — "*" resolves unambiguously since
// there is exactly one compute resource, same dcScopedPath convention as
// every other read-only list tool in this project (inventory_test.go).
func TestInventoryComputeTools_StandaloneESXi(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newInventoryComputeRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	args := map[string]interface{}{"compute_resource": "*"}

	t.Run("hosts", func(t *testing.T) {
		raw, err := r.CallTool("vmware_compute_resource_hosts", args)
		if err != nil {
			t.Fatalf("vmware_compute_resource_hosts failed: %v", err)
		}
		m := decodeResult(t, raw)
		if got := countOf(t, raw); got != 1 {
			t.Fatalf("expected exactly 1 host on standalone ESXi, got %v (result: %s)", got, raw)
		}
		hosts, _ := m["hosts"].([]interface{})
		hostPath, _ := hosts[0].(string)
		if hostPath == "" {
			t.Fatalf("host path is empty — ComputeResource.Hosts's own InventoryPath population is broken: %s", raw)
		}
		t.Logf("standalone host path: %s", hostPath)
	})

	t.Run("datastores", func(t *testing.T) {
		raw, err := r.CallTool("vmware_compute_resource_datastores", args)
		if err != nil {
			t.Fatalf("vmware_compute_resource_datastores failed: %v", err)
		}
		m := decodeResult(t, raw)
		if got := countOf(t, raw); got < 1 {
			t.Fatalf("expected at least 1 datastore, got %v (result: %s)", got, raw)
		}
		dss, _ := m["datastores"].([]interface{})
		dsPath, _ := dss[0].(string)
		if dsPath == "" {
			t.Fatalf("datastore path is empty — find.InventoryPath fix is broken: %s", raw)
		}
	})

	t.Run("resource_pool", func(t *testing.T) {
		raw, err := r.CallTool("vmware_compute_resource_resource_pool", args)
		if err != nil {
			t.Fatalf("vmware_compute_resource_resource_pool failed: %v", err)
		}
		m := decodeResult(t, raw)
		rp, _ := m["resource_pool"].(string)
		if rp == "" {
			t.Fatalf("resource_pool path is empty — find.InventoryPath fix is broken: %s", raw)
		}
		t.Logf("resource pool path: %s", rp)
	})

	t.Run("environment_browser", func(t *testing.T) {
		raw, err := r.CallTool("vmware_compute_resource_environment_browser", args)
		if err != nil {
			t.Fatalf("vmware_compute_resource_environment_browser failed: %v", err)
		}
		m := decodeResult(t, raw)
		eb, _ := m["environment_browser"].(string)
		if eb == "" {
			t.Fatalf("environment_browser moref is empty: %s", raw)
		}
	})

	t.Run("environment_query_config_option", func(t *testing.T) {
		raw, err := r.CallTool("vmware_environment_query_config_option", args)
		if err != nil {
			t.Fatalf("vmware_environment_query_config_option failed: %v", err)
		}
		m := decodeResult(t, raw)
		if _, ok := m["result"]; !ok {
			t.Fatalf("expected a \"result\" field: %s", raw)
		}
	})

	t.Run("environment_query_config_option_descriptor", func(t *testing.T) {
		raw, err := r.CallTool("vmware_environment_query_config_option_descriptor", args)
		if err != nil {
			t.Fatalf("vmware_environment_query_config_option_descriptor failed: %v", err)
		}
		decodeResult(t, raw)
	})

	t.Run("environment_query_config_target", func(t *testing.T) {
		raw, err := r.CallTool("vmware_environment_query_config_target", args)
		if err != nil {
			t.Fatalf("vmware_environment_query_config_target failed: %v", err)
		}
		decodeResult(t, raw)
	})

	t.Run("environment_query_target_capabilities", func(t *testing.T) {
		raw, err := r.CallTool("vmware_environment_query_target_capabilities", args)
		if err != nil {
			t.Fatalf("vmware_environment_query_target_capabilities failed: %v", err)
		}
		decodeResult(t, raw)
	})

	// vmware_compute_resource_reconfigure: real vSphere/govc always target
	// this at a CLUSTER in practice — vcsim has NO server-side handler at
	// all for a plain (non-cluster) ComputeResource (no simulator.ComputeResource
	// Go type exists — see this file's sibling generated_inventory_compute.go
	// top doc comment). assertReachesServer proves this reaches vcsim's real
	// method dispatch (MethodNotFound), not a wiring bug.
	t.Run("reconfigure_no_simulator_support_on_plain_compute_resource", func(t *testing.T) {
		reconfigureArgs := map[string]interface{}{
			"compute_resource": "*",
			"spec":             map[string]interface{}{},
			"confirm":          true,
		}
		_, err := r.CallTool("vmware_compute_resource_reconfigure", reconfigureArgs)
		assertReachesServer(t, err, "vmware_compute_resource_reconfigure")
	})
}

// TestInventoryComputeTools_ClusterFunctional proves full functional
// coverage against simulator.VPX()'s default topology (1 datacenter, 1
// standalone host "DC0_H0", 1 cluster "DC0_C0" with 3 hosts) — vcsim has a
// real simulator.ClusterComputeResource handler for every mutating/query
// method in this group (see generated_inventory_compute.go's top doc
// comment), so this test exercises real success paths, not just
// registration/error probes.
func TestInventoryComputeTools_ClusterFunctional(t *testing.T) {
	model := simulator.VPX()
	c, cleanup := newSimClient(t, model)
	defer cleanup()

	r := newInventoryComputeRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	t.Run("compute_resource_hosts_standalone_host", func(t *testing.T) {
		raw, err := r.CallTool("vmware_compute_resource_hosts", map[string]interface{}{"compute_resource": "DC0_H0"})
		if err != nil {
			t.Fatalf("vmware_compute_resource_hosts(DC0_H0) failed: %v", err)
		}
		if got := countOf(t, raw); got != 1 {
			t.Fatalf("expected exactly 1 host under DC0_H0's implicit compute resource, got %v (result: %s)", got, raw)
		}
	})

	t.Run("compute_resource_hosts_cluster", func(t *testing.T) {
		raw, err := r.CallTool("vmware_compute_resource_hosts", map[string]interface{}{"compute_resource": "DC0_C0"})
		if err != nil {
			t.Fatalf("vmware_compute_resource_hosts(DC0_C0) failed: %v", err)
		}
		if got := countOf(t, raw); got != 3 {
			t.Fatalf("expected exactly 3 hosts in cluster DC0_C0 (ClusterHost default), got %v (result: %s)", got, raw)
		}
	})

	t.Run("cluster_configuration_read", func(t *testing.T) {
		raw, err := r.CallTool("vmware_cluster_configuration", map[string]interface{}{"cluster": "DC0_C0"})
		if err != nil {
			t.Fatalf("vmware_cluster_configuration failed: %v", err)
		}
		decodeResult(t, raw)
	})

	// vmware_compute_resource_reconfigure against the CLUSTER's compute
	// resource — the only vcsim-testable success path for this method (see
	// this file's top-of-domain reconfigure subtest above and
	// generated_inventory_compute.go's top doc comment). Mirrors
	// referencia/govmomi/simulator/cluster_compute_resource_test.go's
	// TestClusterVC.
	t.Run("reconfigure_cluster_enables_drs_and_ha", func(t *testing.T) {
		reconfigureArgs := map[string]interface{}{
			"compute_resource": "DC0_C0",
			"spec": map[string]interface{}{
				"drsConfig": map[string]interface{}{"enabled": true, "defaultVmBehavior": "fullyAutomated"},
				"dasConfig": map[string]interface{}{"enabled": true},
			},
			"confirm": true,
		}
		_, err := r.CallTool("vmware_compute_resource_reconfigure", reconfigureArgs)
		if err != nil {
			t.Fatalf("vmware_compute_resource_reconfigure(DC0_C0) failed: %v", err)
		}

		raw, err := r.CallTool("vmware_cluster_configuration", map[string]interface{}{"cluster": "DC0_C0"})
		if err != nil {
			t.Fatalf("vmware_cluster_configuration after reconfigure failed: %v", err)
		}
		m := decodeResult(t, raw)
		cfg, ok := m["configuration"].(map[string]interface{})
		if !ok {
			t.Fatalf("configuration field is not an object: %s", raw)
		}
		drs, _ := cfg["drsConfig"].(map[string]interface{})
		das, _ := cfg["dasConfig"].(map[string]interface{})
		if drs["enabled"] != true {
			t.Fatalf("expected drsConfig.enabled == true after reconfigure, got %v (result: %s)", drs["enabled"], raw)
		}
		if das["enabled"] != true {
			t.Fatalf("expected dasConfig.enabled == true after reconfigure, got %v (result: %s)", das["enabled"], raw)
		}
	})

	t.Run("add_host_empty_hostname_fails", func(t *testing.T) {
		addArgs := map[string]interface{}{
			"cluster": "DC0_C0",
			"spec":    map[string]interface{}{},
			"confirm": true,
		}
		_, err := r.CallTool("vmware_cluster_add_host", addArgs)
		assertReachesServer(t, err, "vmware_cluster_add_host (empty hostName)")
	})

	t.Run("add_host_succeeds", func(t *testing.T) {
		addArgs := map[string]interface{}{
			"cluster": "DC0_C0",
			"spec":    map[string]interface{}{"hostName": "added-host-01.local"},
			"confirm": true,
		}
		raw, err := r.CallTool("vmware_cluster_add_host", addArgs)
		if err != nil {
			t.Fatalf("vmware_cluster_add_host failed: %v", err)
		}
		decodeResult(t, raw)

		raw, err = r.CallTool("vmware_compute_resource_hosts", map[string]interface{}{"compute_resource": "DC0_C0"})
		if err != nil {
			t.Fatalf("vmware_compute_resource_hosts(DC0_C0) after add_host failed: %v", err)
		}
		if got := countOf(t, raw); got != 4 {
			t.Fatalf("expected 4 hosts in cluster DC0_C0 after adding one, got %v (result: %s)", got, raw)
		}
	})

	t.Run("move_into_succeeds", func(t *testing.T) {
		// DC0_H0's parent is a plain (non-cluster) ComputeResource, so
		// vcsim's MoveIntoTask does not require it to be in maintenance mode
		// first (that check only applies when the host's current parent is
		// itself a ClusterComputeResource) — confirmed by reading
		// simulator/cluster_compute_resource.go's MoveIntoTask switch.
		moveArgs := map[string]interface{}{
			"cluster": "DC0_C0",
			"hosts":   []interface{}{"DC0_H0"},
			"confirm": true,
		}
		raw, err := r.CallTool("vmware_cluster_move_into", moveArgs)
		if err != nil {
			t.Fatalf("vmware_cluster_move_into failed: %v", err)
		}
		decodeResult(t, raw)

		raw, err = r.CallTool("vmware_compute_resource_hosts", map[string]interface{}{"compute_resource": "DC0_C0"})
		if err != nil {
			t.Fatalf("vmware_compute_resource_hosts(DC0_C0) after move_into failed: %v", err)
		}
		if got := countOf(t, raw); got != 5 {
			t.Fatalf("expected 5 hosts in cluster DC0_C0 after moving DC0_H0 in, got %v (result: %s)", got, raw)
		}
	})

	t.Run("place_vm_create", func(t *testing.T) {
		placeArgs := map[string]interface{}{
			"cluster": "DC0_C0",
			"spec":    map[string]interface{}{"placementType": "create"},
		}
		raw, err := r.CallTool("vmware_cluster_place_vm", placeArgs)
		if err != nil {
			t.Fatalf("vmware_cluster_place_vm(create) failed: %v", err)
		}
		decodeResult(t, raw)
	})

	t.Run("place_vm_unsupported_type_reaches_server", func(t *testing.T) {
		placeArgs := map[string]interface{}{
			"cluster": "DC0_C0",
			"spec":    map[string]interface{}{"placementType": "unsupported"},
		}
		_, err := r.CallTool("vmware_cluster_place_vm", placeArgs)
		assertReachesServer(t, err, "vmware_cluster_place_vm (unsupported placementType)")
	})

	t.Run("environment_query_config_option_descriptor_has_entries", func(t *testing.T) {
		raw, err := r.CallTool("vmware_environment_query_config_option_descriptor", map[string]interface{}{"compute_resource": "DC0_C0"})
		if err != nil {
			t.Fatalf("vmware_environment_query_config_option_descriptor(DC0_C0) failed: %v", err)
		}
		if got := countOf(t, raw); got < 1 {
			t.Fatalf("expected at least 1 config option descriptor for a cluster with real hosts, got %v (result: %s)", got, raw)
		}
	})

	t.Run("environment_query_config_target_aggregates_hosts", func(t *testing.T) {
		raw, err := r.CallTool("vmware_environment_query_config_target", map[string]interface{}{"compute_resource": "DC0_C0"})
		if err != nil {
			t.Fatalf("vmware_environment_query_config_target(DC0_C0) failed: %v", err)
		}
		m := decodeResult(t, raw)
		target, ok := m["result"].(map[string]interface{})
		if !ok {
			t.Fatalf("result field is not an object: %s", raw)
		}
		numCpus, _ := target["numCpus"].(float64)
		if numCpus <= 0 {
			t.Fatalf("expected numCpus > 0 aggregated across cluster hosts, got %v (result: %s)", target["numCpus"], raw)
		}
	})

	t.Run("environment_query_target_capabilities_scoped_to_one_host", func(t *testing.T) {
		hostsRaw, err := r.CallTool("vmware_list_hosts", map[string]interface{}{"path": "/DC0/host/DC0_C0/*"})
		if err != nil {
			t.Fatalf("vmware_list_hosts failed: %v", err)
		}
		hm := decodeResult(t, hostsRaw)
		hostList, _ := hm["hosts"].([]interface{})
		if len(hostList) == 0 {
			t.Fatalf("expected at least 1 host under DC0_C0: %s", hostsRaw)
		}
		hostPath, _ := hostList[0].(string)

		raw, err := r.CallTool("vmware_environment_query_target_capabilities", map[string]interface{}{
			"compute_resource": "DC0_C0",
			"host":             hostPath,
		})
		if err != nil {
			t.Fatalf("vmware_environment_query_target_capabilities(host=%s) failed: %v", hostPath, err)
		}
		m := decodeResult(t, raw)
		caps, ok := m["result"].(map[string]interface{})
		if !ok {
			t.Fatalf("result field is not an object: %s", raw)
		}
		if caps["vmotionSupported"] != true {
			t.Fatalf("expected vmotionSupported == true, got %v (result: %s)", caps["vmotionSupported"], raw)
		}
	})
}

// TestInventoryComputeTools_MultiDatacenterVCenter proves the resolve
// helpers (resolveComputeResource/resolveClusterComputeResource) work
// against a vCenter with MORE THAN ONE datacenter — same dcScopedPath
// requirement as inventory_test.go's TestInventoryTools_MultiDatacenterVCenter,
// re-verified here since this group adds its own resolve helpers rather than
// reusing resolveHost/resolveVM.
func TestInventoryComputeTools_MultiDatacenterVCenter(t *testing.T) {
	model := simulator.VPX()
	model.Datacenter = 2

	c, cleanup := newSimClient(t, model)
	defer cleanup()

	r := newInventoryComputeRegistry(context.Background(), c, RegistryOptions{})

	t.Run("compute_resource_absolute_path", func(t *testing.T) {
		raw, err := r.CallTool("vmware_compute_resource_hosts", map[string]interface{}{"compute_resource": "/DC1/host/DC1_C0"})
		if err != nil {
			t.Fatalf("vmware_compute_resource_hosts against a 2-datacenter vCenter (absolute path) failed: %v", err)
		}
		if got := countOf(t, raw); got <= 0 {
			t.Fatalf("expected count > 0, got %v (result: %s)", got, raw)
		}
	})

	t.Run("cluster_absolute_path", func(t *testing.T) {
		raw, err := r.CallTool("vmware_cluster_configuration", map[string]interface{}{"cluster": "/DC1/host/DC1_C0"})
		if err != nil {
			t.Fatalf("vmware_cluster_configuration against a 2-datacenter vCenter (absolute path) failed: %v", err)
		}
		decodeResult(t, raw)
	})
}

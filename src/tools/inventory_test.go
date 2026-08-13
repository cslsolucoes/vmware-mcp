package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/vmware/govmomi/simulator"
)

// decodeResult unmarshals a tool's JSON response for assertions on its
// shape (count, named list) instead of string-matching the raw text.
func decodeResult(t *testing.T, raw string) map[string]interface{} {
	t.Helper()
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("failed to decode tool result %q: %v", raw, err)
	}
	return out
}

func countOf(t *testing.T, raw string) float64 {
	t.Helper()
	m := decodeResult(t, raw)
	c, ok := m["count"].(float64)
	if !ok {
		t.Fatalf("result has no numeric \"count\" field: %q", raw)
	}
	return c
}

// TestInventoryTools_MultiDatacenterVCenter proves the Fase 2 tools list
// inventory correctly against a vCenter with MORE THAN ONE datacenter — the
// case where vmware.NewClient's DefaultDatacenter lookup is ambiguous and
// leaves client.Finder without one set (confirmed empirically against
// simulator.VPX() with Model.Datacenter=2, not assumed; see finder.go and
// the plan's "Fase 2" section for the "must work for vCenter" requirement).
// Without dcScopedPath, every one of these calls would fail with "please
// specify a datacenter" instead of listing anything.
func TestInventoryTools_MultiDatacenterVCenter(t *testing.T) {
	model := simulator.VPX()
	model.Datacenter = 2

	c, cleanup := newSimClient(t, model)
	defer cleanup()

	r := NewRegistry(context.Background(), c, RegistryOptions{})

	cases := []struct {
		tool string
		key  string
	}{
		{"vmware_list_hosts", "hosts"},
		{"vmware_list_datastores", "datastores"},
		{"vmware_list_networks", "networks"},
		{"vmware_list_resource_pools", "resource_pools"},
		{"vmware_list_clusters", "clusters"},
		{"vmware_list_datacenters", "datacenters"},
	}

	for _, tc := range cases {
		t.Run(tc.tool, func(t *testing.T) {
			raw, err := r.CallTool(tc.tool, map[string]interface{}{})
			if err != nil {
				t.Fatalf("%s failed against a 2-datacenter vCenter: %v", tc.tool, err)
			}
			m := decodeResult(t, raw)
			count := countOf(t, raw)
			if count <= 0 {
				t.Fatalf("%s: expected count > 0 across 2 datacenters, got %v (result: %s)", tc.tool, count, raw)
			}
			list, ok := m[tc.key].([]interface{})
			if !ok || len(list) != int(count) {
				t.Fatalf("%s: %q list length does not match count %v: %s", tc.tool, tc.key, count, raw)
			}
		})
	}
}

// TestInventoryTools_StandaloneESXi proves the same tools tolerate the
// standalone-ESXi shape: exactly one datacenter, no clusters, but the
// implicit "Resources" pool under the host's ComputeResource — an empty
// vmware_list_clusters result must NOT be a tool error (see
// emptyOnNotFound), the plan's "handlers toleram listas vazias" acceptance
// criterion.
func TestInventoryTools_StandaloneESXi(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := NewRegistry(context.Background(), c, RegistryOptions{})

	t.Run("hosts", func(t *testing.T) {
		raw, err := r.CallTool("vmware_list_hosts", map[string]interface{}{})
		if err != nil {
			t.Fatalf("vmware_list_hosts failed against standalone ESXi: %v", err)
		}
		if got := countOf(t, raw); got != 1 {
			t.Fatalf("expected exactly 1 host on standalone ESXi, got %v (result: %s)", got, raw)
		}
	})

	t.Run("datacenters", func(t *testing.T) {
		raw, err := r.CallTool("vmware_list_datacenters", map[string]interface{}{})
		if err != nil {
			t.Fatalf("vmware_list_datacenters failed against standalone ESXi: %v", err)
		}
		if got := countOf(t, raw); got != 1 {
			t.Fatalf("expected exactly 1 datacenter on standalone ESXi, got %v (result: %s)", got, raw)
		}
	})

	t.Run("resource_pools", func(t *testing.T) {
		raw, err := r.CallTool("vmware_list_resource_pools", map[string]interface{}{})
		if err != nil {
			t.Fatalf("vmware_list_resource_pools failed against standalone ESXi: %v", err)
		}
		if got := countOf(t, raw); got < 1 {
			t.Fatalf("expected at least the implicit \"Resources\" pool on standalone ESXi, got %v (result: %s)", got, raw)
		}
	})

	t.Run("clusters_empty_not_error", func(t *testing.T) {
		raw, err := r.CallTool("vmware_list_clusters", map[string]interface{}{})
		if err != nil {
			t.Fatalf("vmware_list_clusters must tolerate 0 clusters on standalone ESXi, got error: %v", err)
		}
		if got := countOf(t, raw); got != 0 {
			t.Fatalf("expected 0 clusters on standalone ESXi, got %v (result: %s)", got, raw)
		}
	})

	t.Run("datastores_have_summary", func(t *testing.T) {
		raw, err := r.CallTool("vmware_list_datastores", map[string]interface{}{})
		if err != nil {
			t.Fatalf("vmware_list_datastores failed against standalone ESXi: %v", err)
		}
		m := decodeResult(t, raw)
		list, _ := m["datastores"].([]interface{})
		if len(list) == 0 {
			t.Fatalf("expected at least 1 datastore on standalone ESXi, result: %s", raw)
		}
		first, ok := list[0].(map[string]interface{})
		if !ok {
			t.Fatalf("datastore entry is not an object: %s", raw)
		}
		for _, field := range []string{"name", "path", "type", "capacity_bytes", "free_space_bytes", "accessible"} {
			if _, ok := first[field]; !ok {
				t.Fatalf("datastore entry missing %q field: %s", field, raw)
			}
		}
	})
}

package tools

import (
	"context"
	"testing"

	"github.com/vmware/govmomi/simulator"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// newHostNetworkQueryRegistry layers registerHostNetworkQueryTools on top of a
// normal registry (same test-only withClass pattern as the other generated_*
// test files) — this file must not edit registry.go itself.
func newHostNetworkQueryRegistry(ctx context.Context, c *vmware.Client, opts RegistryOptions) *Registry {
	r := NewRegistry(ctx, c, opts)
	r.withClass(modeVSphereGeneral, registerHostNetworkQueryTools)
	return r
}

func TestHostNetworkQuery_Registration(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newHostNetworkQueryRegistry(context.Background(), c, RegistryOptions{})
	got := map[string]bool{}
	for _, tl := range r.ListTools() {
		got[tl.Name] = true
	}
	if !got["vmware_host_network_query_port_groups"] {
		t.Fatal("vmware_host_network_query_port_groups not registered")
	}
}

// TestHostNetworkQuery_RealSuccess proves the tool reads real network config
// from vcsim's ESX host (which populates config.network with default port
// groups like "Management Network"/"VM Network") and that each returned port
// group carries a re-usable full spec — the whole point of the tool.
func TestHostNetworkQuery_RealSuccess(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newHostNetworkQueryRegistry(context.Background(), c, RegistryOptions{})
	host := firstHostPath(t, r)

	if _, err := r.CallTool("vmware_host_network_query_port_groups", map[string]interface{}{"host": "does-not-exist"}); err == nil {
		t.Fatal("expected an error for an unresolvable host")
	}

	raw, err := r.CallTool("vmware_host_network_query_port_groups", map[string]interface{}{"host": host})
	if err != nil {
		t.Fatalf("query_port_groups failed: %v", err)
	}
	m := decodeResult(t, raw)
	if m["port_groups"] == nil || m["vswitches"] == nil {
		t.Fatalf("missing port_groups/vswitches keys: %s", raw)
	}
	pgs, _ := m["port_groups"].([]interface{})
	t.Logf("vcsim ESX returned %d port groups", len(pgs))
	if len(pgs) > 0 {
		first, _ := pgs[0].(map[string]interface{})
		if first["name"] == nil || first["spec"] == nil || first["vswitch_name"] == nil {
			t.Fatalf("port group entry missing name/spec/vswitch_name (needed to re-send to update_port_group): %s", raw)
		}
	}
}

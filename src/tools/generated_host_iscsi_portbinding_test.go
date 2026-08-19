package tools

import (
	"context"
	"testing"

	"github.com/vmware/govmomi/simulator"
)

// iscsiPortBindingToolNames is the exact set registered by
// registerHostIscsiPortBindingTools — kept here so
// TestHostIscsiPortBindingTools_Registration and the vsphereGeneralTools
// list in mode_test.go can't silently drift from each other.
var iscsiPortBindingToolNames = []string{
	"vmware_host_iscsi_bind_vnic",
	"vmware_host_iscsi_unbind_vnic",
	"vmware_host_iscsi_query_bound_vnics",
	"vmware_host_iscsi_query_candidate_nics",
	"vmware_host_iscsi_query_migration_dependencies",
	"vmware_host_iscsi_query_vnic_status",
	"vmware_host_iscsi_query_pnic_status",
}

// TestHostIscsiPortBindingTools_Registration proves all 7 iSCSI port
// binding tools are wired through registry.go's registerTools (NewRegistry,
// not a test-only withClass) and reachable via ListTools.
func TestHostIscsiPortBindingTools_Registration(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := NewRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	if len(iscsiPortBindingToolNames) != 7 {
		t.Fatalf("test bug: iscsiPortBindingToolNames has %d entries, expected 7", len(iscsiPortBindingToolNames))
	}

	got := map[string]bool{}
	for _, tl := range r.ListTools() {
		got[tl.Name] = true
	}
	for _, name := range iscsiPortBindingToolNames {
		if !got[name] {
			t.Errorf("tool %s not registered", name)
		}
	}
}

// TestHostIscsiPortBindingTools_Validation proves each handler rejects
// missing/empty required arguments BEFORE any network round trip (so these
// fail even with the gate open and confirm:true).
func TestHostIscsiPortBindingTools_Validation(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := NewRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	host := firstHostPath(t, r)

	cases := []struct {
		name string
		args map[string]interface{}
		why  string
	}{
		{"vmware_host_iscsi_bind_vnic", map[string]interface{}{"host": host, "vnic_device": "vmk1", "confirm": true}, "missing iscsi_hba_name"},
		{"vmware_host_iscsi_bind_vnic", map[string]interface{}{"host": host, "iscsi_hba_name": "vmhba64", "confirm": true}, "missing vnic_device"},
		{"vmware_host_iscsi_unbind_vnic", map[string]interface{}{"host": host, "vnic_device": "vmk1", "confirm": true}, "missing iscsi_hba_name"},
		{"vmware_host_iscsi_unbind_vnic", map[string]interface{}{"host": host, "iscsi_hba_name": "vmhba64", "confirm": true}, "missing vnic_device"},
		{"vmware_host_iscsi_query_bound_vnics", map[string]interface{}{"host": host}, "missing iscsi_hba_name"},
		{"vmware_host_iscsi_query_candidate_nics", map[string]interface{}{"host": host}, "missing iscsi_hba_name"},
		{"vmware_host_iscsi_query_migration_dependencies", map[string]interface{}{"host": host}, "missing pnic_devices"},
		{"vmware_host_iscsi_query_migration_dependencies", map[string]interface{}{"host": host, "pnic_devices": []interface{}{}}, "empty pnic_devices"},
		{"vmware_host_iscsi_query_vnic_status", map[string]interface{}{"host": host}, "missing vnic_device"},
		{"vmware_host_iscsi_query_pnic_status", map[string]interface{}{"host": host}, "missing pnic_device"},
	}

	for _, tc := range cases {
		t.Run(tc.name+"/"+tc.why, func(t *testing.T) {
			if _, err := r.CallTool(tc.name, tc.args); err == nil {
				t.Fatalf("expected an error (%s) before any round trip", tc.why)
			}
		})
	}
}

// TestHostIscsiPortBindingTools_GateAndConfirm proves the tier2 destructive
// protection is wired on the two mutating tools (Bind/Unbind): a closed
// --allow-destructive gate denies the call, and an open gate still requires
// confirm:true.
func TestHostIscsiPortBindingTools_GateAndConfirm(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	host := firstHostPath(t, NewRegistry(context.Background(), c, RegistryOptions{}))

	closed := NewRegistry(context.Background(), c, RegistryOptions{})
	open := NewRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	for _, name := range []string{"vmware_host_iscsi_bind_vnic", "vmware_host_iscsi_unbind_vnic"} {
		validEnough := map[string]interface{}{"host": host, "iscsi_hba_name": "vmhba64", "vnic_device": "vmk1", "confirm": true}

		if _, err := closed.CallTool(name, validEnough); err == nil {
			t.Fatalf("%s: expected the closed destructive gate to deny the call", name)
		}

		noConfirm := map[string]interface{}{"host": host, "iscsi_hba_name": "vmhba64", "vnic_device": "vmk1"}
		if _, err := open.CallTool(name, noConfirm); err == nil {
			t.Fatalf("%s: expected an error without confirm:true", name)
		}
	}
}

// TestHostIscsiPortBindingTools_ReachesServer drives every tool with valid
// input, gate open, and confirm:true. vcsim's ESX() model populates
// configManager.iscsiManager with a MoRef (so hostIscsiManager resolves
// successfully) but registers no simulator.IscsiManager object to back it
// (see generated_host_iscsi_portbinding.go's top doc comment), so each raw
// SOAP call is expected to reach vcsim's dispatch and return a clean
// server-side error — assertReachesServer, the same helper
// generated_host_iscsi_test.go uses for its 15 unsimulated
// HostStorageSystem iSCSI/multipath methods.
func TestHostIscsiPortBindingTools_ReachesServer(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := NewRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	host := firstHostPath(t, r)
	const hba = "vmhba64"
	const vnic = "vmk1"
	const pnic = "vmnic1"

	cases := []struct {
		name string
		args map[string]interface{}
	}{
		{"vmware_host_iscsi_bind_vnic", map[string]interface{}{"host": host, "iscsi_hba_name": hba, "vnic_device": vnic, "confirm": true}},
		{"vmware_host_iscsi_unbind_vnic", map[string]interface{}{"host": host, "iscsi_hba_name": hba, "vnic_device": vnic, "confirm": true}},
		{"vmware_host_iscsi_query_bound_vnics", map[string]interface{}{"host": host, "iscsi_hba_name": hba}},
		{"vmware_host_iscsi_query_candidate_nics", map[string]interface{}{"host": host, "iscsi_hba_name": hba}},
		{"vmware_host_iscsi_query_migration_dependencies", map[string]interface{}{"host": host, "pnic_devices": []interface{}{pnic}}},
		{"vmware_host_iscsi_query_vnic_status", map[string]interface{}{"host": host, "vnic_device": vnic}},
		{"vmware_host_iscsi_query_pnic_status", map[string]interface{}{"host": host, "pnic_device": pnic}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := r.CallTool(tc.name, tc.args)
			assertReachesServer(t, err, tc.name)
		})
	}
}

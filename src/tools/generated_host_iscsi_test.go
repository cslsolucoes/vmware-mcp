package tools

import (
	"context"
	"testing"

	"github.com/vmware/govmomi/simulator"
)

// iscsiToolNames is the exact set registered by registerHostIscsiTools —
// kept here so TestHostIscsiTools_Registration and the vsphereGeneralTools
// list in mode_test.go can't silently drift from each other.
var iscsiToolNames = []string{
	"vmware_host_iscsi_software_set_enabled",
	"vmware_host_iscsi_update_name",
	"vmware_host_iscsi_update_alias",
	"vmware_host_iscsi_add_send_targets",
	"vmware_host_iscsi_remove_send_targets",
	"vmware_host_iscsi_add_static_targets",
	"vmware_host_iscsi_remove_static_targets",
	"vmware_host_iscsi_update_authentication_properties",
	"vmware_host_iscsi_update_digest_properties",
	"vmware_host_iscsi_update_discovery_properties",
	"vmware_host_iscsi_update_ip_properties",
	"vmware_host_iscsi_update_advanced_options",
	"vmware_host_storage_enable_multipath_path",
	"vmware_host_storage_disable_multipath_path",
	"vmware_host_storage_set_multipath_lun_policy",
}

// TestHostIscsiTools_Registration proves all 15 iSCSI/multipath tools are
// wired through registry.go's registerTools (NewRegistry, not a test-only
// withClass) and reachable via ListTools.
func TestHostIscsiTools_Registration(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := NewRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	if len(iscsiToolNames) != 15 {
		t.Fatalf("test bug: iscsiToolNames has %d entries, expected 15", len(iscsiToolNames))
	}

	got := map[string]bool{}
	for _, tl := range r.ListTools() {
		got[tl.Name] = true
	}
	for _, name := range iscsiToolNames {
		if !got[name] {
			t.Errorf("tool %s not registered", name)
		}
	}
}

// TestHostIscsiTools_Validation proves each handler rejects missing/empty
// required arguments BEFORE any network round trip (so these fail even with
// the gate open and confirm:true).
func TestHostIscsiTools_Validation(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := NewRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	host := firstHostPath(t, r)

	cases := []struct {
		name string
		args map[string]interface{}
		why  string
	}{
		{"vmware_host_iscsi_software_set_enabled", map[string]interface{}{"host": host, "confirm": true}, "missing enabled"},
		{"vmware_host_iscsi_update_name", map[string]interface{}{"host": host, "iscsi_name": "iqn.x", "confirm": true}, "missing iscsi_hba_device"},
		{"vmware_host_iscsi_update_name", map[string]interface{}{"host": host, "iscsi_hba_device": "vmhba64", "confirm": true}, "missing iscsi_name"},
		{"vmware_host_iscsi_update_alias", map[string]interface{}{"host": host, "iscsi_hba_device": "vmhba64", "confirm": true}, "missing iscsi_alias"},
		{"vmware_host_iscsi_add_send_targets", map[string]interface{}{"host": host, "iscsi_hba_device": "vmhba64", "confirm": true}, "missing targets"},
		{"vmware_host_iscsi_add_send_targets", map[string]interface{}{"host": host, "iscsi_hba_device": "vmhba64", "targets": []interface{}{}, "confirm": true}, "empty targets"},
		{"vmware_host_iscsi_add_static_targets", map[string]interface{}{"host": host, "iscsi_hba_device": "vmhba64", "confirm": true}, "missing targets"},
		{"vmware_host_iscsi_update_authentication_properties", map[string]interface{}{"host": host, "iscsi_hba_device": "vmhba64", "confirm": true}, "missing authentication_properties"},
		{"vmware_host_iscsi_update_digest_properties", map[string]interface{}{"host": host, "iscsi_hba_device": "vmhba64", "confirm": true}, "missing digest_properties"},
		{"vmware_host_iscsi_update_discovery_properties", map[string]interface{}{"host": host, "iscsi_hba_device": "vmhba64", "confirm": true}, "missing discovery_properties"},
		{"vmware_host_iscsi_update_ip_properties", map[string]interface{}{"host": host, "iscsi_hba_device": "vmhba64", "confirm": true}, "missing ip_properties"},
		{"vmware_host_iscsi_update_advanced_options", map[string]interface{}{"host": host, "iscsi_hba_device": "vmhba64", "confirm": true}, "missing options"},
		{"vmware_host_storage_enable_multipath_path", map[string]interface{}{"host": host, "confirm": true}, "missing path_name"},
		{"vmware_host_storage_set_multipath_lun_policy", map[string]interface{}{"host": host, "lun_id": "id", "confirm": true}, "missing policy"},
	}

	for _, tc := range cases {
		t.Run(tc.name+"/"+tc.why, func(t *testing.T) {
			if _, err := r.CallTool(tc.name, tc.args); err == nil {
				t.Fatalf("expected an error (%s) before any round trip", tc.why)
			}
		})
	}
}

// TestHostIscsiTools_GateAndConfirm proves the tier2 destructive protection is
// wired: a closed --allow-destructive gate denies the call, and an open gate
// still requires confirm:true. Spot-checked on one iSCSI and one multipath
// tool (every tool in this file shares the same registerDestructive wrapper).
func TestHostIscsiTools_GateAndConfirm(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	host := firstHostPath(t, NewRegistry(context.Background(), c, RegistryOptions{}))

	closed := NewRegistry(context.Background(), c, RegistryOptions{})
	open := NewRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	for _, name := range []string{"vmware_host_iscsi_software_set_enabled", "vmware_host_storage_enable_multipath_path"} {
		validEnough := map[string]interface{}{"host": host, "enabled": true, "path_name": "vmhba64:C0:T0:L0", "confirm": true}

		if _, err := closed.CallTool(name, validEnough); err == nil {
			t.Fatalf("%s: expected the closed destructive gate to deny the call", name)
		}

		noConfirm := map[string]interface{}{"host": host, "enabled": true, "path_name": "vmhba64:C0:T0:L0"}
		if _, err := open.CallTool(name, noConfirm); err == nil {
			t.Fatalf("%s: expected an error without confirm:true", name)
		}
	}
}

// TestHostIscsiTools_ReachesServer drives every tool with valid input, gate
// open, and confirm:true. vcsim has no server-side implementation for any of
// these HostStorageSystem iSCSI/multipath methods, so each is expected to
// reach the real vcsim dispatch and return a clean server-side error (not an
// unknown-tool wiring bug or a recovered panic) — assertReachesServer, the
// same helper generated_host_storage_test.go uses for its 15 unsimulated
// methods. Behavioral success is validated against a real ESXi host.
func TestHostIscsiTools_ReachesServer(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := NewRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	host := firstHostPath(t, r)
	const dev = "vmhba64"

	cases := []struct {
		name string
		args map[string]interface{}
	}{
		{"vmware_host_iscsi_software_set_enabled", map[string]interface{}{"host": host, "enabled": true, "confirm": true}},
		{"vmware_host_iscsi_update_name", map[string]interface{}{"host": host, "iscsi_hba_device": dev, "iscsi_name": "iqn.1998-01.com.vmware:esxi-test", "confirm": true}},
		{"vmware_host_iscsi_update_alias", map[string]interface{}{"host": host, "iscsi_hba_device": dev, "iscsi_alias": "test-alias", "confirm": true}},
		{"vmware_host_iscsi_add_send_targets", map[string]interface{}{"host": host, "iscsi_hba_device": dev, "targets": []interface{}{map[string]interface{}{"address": "10.0.0.1", "port": 3260}}, "confirm": true}},
		{"vmware_host_iscsi_remove_send_targets", map[string]interface{}{"host": host, "iscsi_hba_device": dev, "targets": []interface{}{map[string]interface{}{"address": "10.0.0.1", "port": 3260}}, "confirm": true}},
		{"vmware_host_iscsi_add_static_targets", map[string]interface{}{"host": host, "iscsi_hba_device": dev, "targets": []interface{}{map[string]interface{}{"address": "10.0.0.1", "port": 3260, "iScsiName": "iqn.2000-01.com.example:target0"}}, "confirm": true}},
		{"vmware_host_iscsi_remove_static_targets", map[string]interface{}{"host": host, "iscsi_hba_device": dev, "targets": []interface{}{map[string]interface{}{"address": "10.0.0.1", "port": 3260, "iScsiName": "iqn.2000-01.com.example:target0"}}, "confirm": true}},
		{"vmware_host_iscsi_update_authentication_properties", map[string]interface{}{"host": host, "iscsi_hba_device": dev, "authentication_properties": map[string]interface{}{"chapAuthEnabled": true, "chapName": "user", "chapSecret": "secret", "chapAuthenticationType": "chapRequired"}, "confirm": true}},
		{"vmware_host_iscsi_update_digest_properties", map[string]interface{}{"host": host, "iscsi_hba_device": dev, "digest_properties": map[string]interface{}{"headerDigestType": "digestProhibited", "dataDigestType": "digestProhibited"}, "confirm": true}},
		{"vmware_host_iscsi_update_discovery_properties", map[string]interface{}{"host": host, "iscsi_hba_device": dev, "discovery_properties": map[string]interface{}{"staticTargetDiscoveryEnabled": true, "sendTargetsDiscoveryEnabled": true, "iSnsDiscoveryEnabled": false, "slpDiscoveryEnabled": false}, "confirm": true}},
		{"vmware_host_iscsi_update_ip_properties", map[string]interface{}{"host": host, "iscsi_hba_device": dev, "ip_properties": map[string]interface{}{"dhcpConfigurationEnabled": true}, "confirm": true}},
		{"vmware_host_iscsi_update_advanced_options", map[string]interface{}{"host": host, "iscsi_hba_device": dev, "options": []interface{}{map[string]interface{}{"key": "LoginTimeout", "value": "5"}}, "confirm": true}},
		{"vmware_host_storage_enable_multipath_path", map[string]interface{}{"host": host, "path_name": "vmhba64:C0:T0:L0", "confirm": true}},
		{"vmware_host_storage_disable_multipath_path", map[string]interface{}{"host": host, "path_name": "vmhba64:C0:T0:L0", "confirm": true}},
		{"vmware_host_storage_set_multipath_lun_policy", map[string]interface{}{"host": host, "lun_id": "0000", "policy": "VMW_PSP_FIXED", "confirm": true}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := r.CallTool(tc.name, tc.args)
			assertReachesServer(t, err, tc.name)
		})
	}
}

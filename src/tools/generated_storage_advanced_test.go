package tools

import (
	"context"
	"testing"

	"github.com/vmware/govmomi/simulator"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// newStorageAdvancedRegistry builds a Registry the normal way (NewRegistry,
// which wires vm.go/host.go/etc via registerTools) and then manually layers
// this group's two classes on top via withClass — same pattern
// generated_vm_ft_test.go's newFtRegistry and
// generated_esx_settings_cluster_vms_test.go's
// newEsxSettingsClusterVMsRegistry use, for the same reason: this file must
// not edit registry.go. Two withClass calls because HostVFlashManager
// (modeVSphereGeneral) and IoFilterManager (modeVCenterOnly) are
// evidence-classified differently — see generated_storage_advanced.go's top
// doc comment.
func newStorageAdvancedRegistry(ctx context.Context, c *vmware.Client, opts RegistryOptions) *Registry {
	r := NewRegistry(ctx, c, opts)
	r.withClass(modeVSphereGeneral, registerHostVFlashTools)
	r.withClass(modeVCenterOnly, registerIoFilterTools)
	return r
}

// storageAdvancedToolNames is the exact set registered by
// registerHostVFlashTools + registerIoFilterTools — kept here so
// TestStorageAdvancedTools_Registration can't silently drift from the real
// registration lists.
var storageAdvancedToolNames = []string{
	"vmware_host_vflash_configure_resource_ex",
	"vmware_host_vflash_configure_resource",
	"vmware_host_vflash_configure_cache",
	"vmware_host_vflash_get_module_default_config",
	"vmware_host_vflash_remove_resource",
	"vmware_iofilter_install",
	"vmware_iofilter_uninstall",
	"vmware_iofilter_upgrade",
	"vmware_iofilter_query_info",
	"vmware_iofilter_query_issues",
	"vmware_iofilter_query_disks_using_filter",
	"vmware_iofilter_resolve_installation_errors_on_cluster",
	"vmware_iofilter_resolve_installation_errors_on_host",
}

// TestStorageAdvancedTools_Registration proves all 13 tools are wired
// through newStorageAdvancedRegistry's two withClass calls and reachable via
// ListTools.
func TestStorageAdvancedTools_Registration(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newStorageAdvancedRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	if len(storageAdvancedToolNames) != 13 {
		t.Fatalf("test bug: storageAdvancedToolNames has %d entries, expected 13", len(storageAdvancedToolNames))
	}

	got := map[string]bool{}
	for _, tl := range r.ListTools() {
		got[tl.Name] = true
	}
	for _, name := range storageAdvancedToolNames {
		if !got[name] {
			t.Errorf("tool %s not registered", name)
		}
	}
}

// TestStorageAdvancedTools_Validation proves each handler rejects
// missing/empty required arguments BEFORE any network round trip (so these
// fail even with the gate open and confirm:true).
func TestStorageAdvancedTools_Validation(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newStorageAdvancedRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	host := firstHostPath(t, r)
	cluster := firstClusterPath(t, r)

	cases := []struct {
		name string
		args map[string]interface{}
		why  string
	}{
		{"vmware_host_vflash_configure_resource_ex", map[string]interface{}{"confirm": true}, "missing host"},
		{"vmware_host_vflash_configure_resource_ex", map[string]interface{}{"host": host, "confirm": true}, "missing device_paths"},
		{"vmware_host_vflash_configure_resource_ex", map[string]interface{}{"host": host, "device_paths": []interface{}{}, "confirm": true}, "empty device_paths"},
		{"vmware_host_vflash_configure_resource", map[string]interface{}{"confirm": true}, "missing host"},
		{"vmware_host_vflash_configure_resource", map[string]interface{}{"host": host, "confirm": true}, "missing vffs_uuid"},
		{"vmware_host_vflash_configure_cache", map[string]interface{}{"confirm": true}, "missing host"},
		{"vmware_host_vflash_configure_cache", map[string]interface{}{"host": host, "confirm": true}, "missing default_vflash_module"},
		{"vmware_host_vflash_get_module_default_config", map[string]interface{}{}, "missing host"},
		{"vmware_host_vflash_get_module_default_config", map[string]interface{}{"host": host}, "missing vflash_module"},
		{"vmware_host_vflash_remove_resource", map[string]interface{}{"confirm": true}, "missing host"},
		{"vmware_iofilter_install", map[string]interface{}{"confirm": true}, "missing cluster"},
		{"vmware_iofilter_install", map[string]interface{}{"cluster": cluster, "confirm": true}, "missing vib_url"},
		{"vmware_iofilter_uninstall", map[string]interface{}{"confirm": true}, "missing cluster"},
		{"vmware_iofilter_uninstall", map[string]interface{}{"cluster": cluster, "confirm": true}, "missing filter_id"},
		{"vmware_iofilter_upgrade", map[string]interface{}{"confirm": true}, "missing cluster"},
		{"vmware_iofilter_upgrade", map[string]interface{}{"cluster": cluster, "confirm": true}, "missing filter_id"},
		{"vmware_iofilter_upgrade", map[string]interface{}{"cluster": cluster, "filter_id": "filter-1", "confirm": true}, "missing vib_url"},
		{"vmware_iofilter_query_info", map[string]interface{}{}, "missing cluster"},
		{"vmware_iofilter_query_issues", map[string]interface{}{}, "missing cluster"},
		{"vmware_iofilter_query_issues", map[string]interface{}{"cluster": cluster}, "missing filter_id"},
		{"vmware_iofilter_query_disks_using_filter", map[string]interface{}{}, "missing cluster"},
		{"vmware_iofilter_query_disks_using_filter", map[string]interface{}{"cluster": cluster}, "missing filter_id"},
		{"vmware_iofilter_resolve_installation_errors_on_cluster", map[string]interface{}{"confirm": true}, "missing cluster"},
		{"vmware_iofilter_resolve_installation_errors_on_cluster", map[string]interface{}{"cluster": cluster, "confirm": true}, "missing filter_id"},
		{"vmware_iofilter_resolve_installation_errors_on_host", map[string]interface{}{"confirm": true}, "missing host"},
		{"vmware_iofilter_resolve_installation_errors_on_host", map[string]interface{}{"host": host, "confirm": true}, "missing filter_id"},
	}

	for _, tc := range cases {
		t.Run(tc.name+"/"+tc.why, func(t *testing.T) {
			if _, err := r.CallTool(tc.name, tc.args); err == nil {
				t.Fatalf("expected an error (%s) before any round trip", tc.why)
			}
		})
	}
}

// TestStorageAdvancedTools_GateAndConfirm proves the tier2 destructive
// protection is actually wired on every mutating tool: a closed
// --allow-destructive gate denies the call, and an open gate still requires
// confirm:true. The 4 read-only Query/Get tools are deliberately excluded —
// they take no confirm argument and are not gated (see this file's
// generated_storage_advanced.go top doc comment, "Tier").
func TestStorageAdvancedTools_GateAndConfirm(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	seed := newStorageAdvancedRegistry(context.Background(), c, RegistryOptions{})
	host := firstHostPath(t, seed)
	cluster := firstClusterPath(t, seed)

	closed := newStorageAdvancedRegistry(context.Background(), c, RegistryOptions{})
	open := newStorageAdvancedRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	cases := []struct {
		name string
		args map[string]interface{}
	}{
		{"vmware_host_vflash_configure_resource_ex", map[string]interface{}{"host": host, "device_paths": []interface{}{"/vmfs/devices/disks/naa.fake"}}},
		{"vmware_host_vflash_configure_resource", map[string]interface{}{"host": host, "vffs_uuid": "fake-uuid"}},
		{"vmware_host_vflash_configure_cache", map[string]interface{}{"host": host, "default_vflash_module": "vfc"}},
		{"vmware_host_vflash_remove_resource", map[string]interface{}{"host": host}},
		{"vmware_iofilter_install", map[string]interface{}{"cluster": cluster, "vib_url": "http://example.com/filter.vib"}},
		{"vmware_iofilter_uninstall", map[string]interface{}{"cluster": cluster, "filter_id": "filter-1"}},
		{"vmware_iofilter_upgrade", map[string]interface{}{"cluster": cluster, "filter_id": "filter-1", "vib_url": "http://example.com/filter.vib"}},
		{"vmware_iofilter_resolve_installation_errors_on_cluster", map[string]interface{}{"cluster": cluster, "filter_id": "filter-1"}},
		{"vmware_iofilter_resolve_installation_errors_on_host", map[string]interface{}{"host": host, "filter_id": "filter-1"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withConfirm := map[string]interface{}{}
			for k, v := range tc.args {
				withConfirm[k] = v
			}
			withConfirm["confirm"] = true

			if _, err := closed.CallTool(tc.name, withConfirm); err == nil {
				t.Fatalf("%s: expected the closed destructive gate to deny the call", tc.name)
			}

			if _, err := open.CallTool(tc.name, tc.args); err == nil {
				t.Fatalf("%s: expected an error without confirm:true", tc.name)
			}
		})
	}
}

// TestStorageAdvancedTools_ReachesServer drives every tool with valid input,
// gate open, and confirm:true where applicable. Neither HostVFlashManager
// nor IoFilterManager has a server-side handler anywhere in vcsim (see
// generated_storage_advanced.go's top doc comment, "vcsim coverage"), so
// each raw SOAP call is expected to reach vcsim's dispatch and return a
// clean server-side fault — assertReachesServer (generated_vm_lifecycle_test.go),
// the same helper generated_vm_ft_test.go and
// generated_host_iscsi_portbinding_test.go use for their own unsimulated
// methods.
//
// simulator.VPX() (a vCenter model), not ESX(): IoFilterManager is
// modeVCenterOnly and needs ServiceContent.IoFilterManager to be non-nil
// (only true under VPX(), see this file's top doc comment) plus a real
// cluster fixture (firstClusterPath); VPX() also has a host
// (ConfigManager.VFlashManager populated the same way under both ESX() and
// VPX(), see generated_storage_advanced.go's classification evidence), so
// one model covers every tool in this file.
func TestStorageAdvancedTools_ReachesServer(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newStorageAdvancedRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	host := firstHostPath(t, r)
	cluster := firstClusterPath(t, r)
	const filterID = "filter-1"
	const vibURL = "http://example.com/filter.vib"

	cases := []struct {
		name string
		args map[string]interface{}
	}{
		{"vmware_host_vflash_configure_resource_ex", map[string]interface{}{"host": host, "device_paths": []interface{}{"/vmfs/devices/disks/naa.fake"}, "confirm": true}},
		{"vmware_host_vflash_configure_resource", map[string]interface{}{"host": host, "vffs_uuid": "fake-uuid", "confirm": true}},
		{"vmware_host_vflash_configure_cache", map[string]interface{}{"host": host, "default_vflash_module": "vfc", "swap_cache_reservation_gb": 4, "confirm": true}},
		{"vmware_host_vflash_get_module_default_config", map[string]interface{}{"host": host, "vflash_module": "vfc"}},
		{"vmware_host_vflash_remove_resource", map[string]interface{}{"host": host, "confirm": true}},
		{"vmware_iofilter_install", map[string]interface{}{"cluster": cluster, "vib_url": vibURL, "confirm": true}},
		{"vmware_iofilter_uninstall", map[string]interface{}{"cluster": cluster, "filter_id": filterID, "confirm": true}},
		{"vmware_iofilter_upgrade", map[string]interface{}{"cluster": cluster, "filter_id": filterID, "vib_url": vibURL, "confirm": true}},
		{"vmware_iofilter_query_info", map[string]interface{}{"cluster": cluster}},
		{"vmware_iofilter_query_issues", map[string]interface{}{"cluster": cluster, "filter_id": filterID}},
		{"vmware_iofilter_query_disks_using_filter", map[string]interface{}{"cluster": cluster, "filter_id": filterID}},
		{"vmware_iofilter_resolve_installation_errors_on_cluster", map[string]interface{}{"cluster": cluster, "filter_id": filterID, "confirm": true}},
		{"vmware_iofilter_resolve_installation_errors_on_host", map[string]interface{}{"host": host, "filter_id": filterID, "confirm": true}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := r.CallTool(tc.name, tc.args)
			assertReachesServer(t, err, tc.name)
		})
	}
}

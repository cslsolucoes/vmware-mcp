package tools

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// This file covers two advanced storage managed objects that neither have an
// object.* wrapper in govmomi nor any existing tool coverage in this
// project — confirmed empty by `grep -rhoE '"vmware_(vflash|iofilter)[a-z_]*"'
// across every tools/*.go file before writing this one:
//
//   - HostVFlashManager (vFlash Read Cache / vFlash resource pools —
//     govmomi@v0.55.1/vim25/mo/mo.go's mo.HostVFlashManager), reachable per
//     host via HostSystem.ConfigManager.VFlashManager (types.go's
//     HostConfigManager.VFlashManager *ManagedObjectReference,
//     xml:"vFlashManager"). object.HostConfigManager
//     (govmomi/object/host_config_manager.go) has no VFlashManager
//     accessor — same "no object.* wrapper, no accessor either" gap
//     generated_host_iscsi_portbinding.go already documents for
//     IscsiManager — so every handler here dials the raw vim25 SOAP method
//     directly (methods.Xxx(ctx, client.Client.Client, &types.Xxx{This: ref,
//     ...})) against the MoRef this file's vflashManager helper resolves via
//     Properties(ctx, ..., []string{"configManager.vFlashManager"}, &mo.HostSystem{})
//     — the exact same Properties-read pattern hostIscsiManager already uses.
//     5 real methods confirmed one-by-one by reading vim25/methods/methods.go
//     + the "The parameters of `HostVFlashManager.Xxx`." doc comment directly
//     above each request struct in vim25/types/types.go (not assumed from a
//     brief): ConfigureVFlashResourceEx_Task, HostConfigureVFlashResource,
//     HostConfigVFlashCache, HostGetVFlashModuleDefaultConfig,
//     HostRemoveVFlashResource. "HostConfigVFlashCache"/"HostGetVFlash
//     ModuleDefaultConfig" as named in the brief ARE the real method names
//     (not "ConfigureHostVFlashCache"/"HostGetVFlashModuleDefaultConfig" —
//     the brief's guess for the cache-config method happened to match
//     exactly, confirmed against methods.go, not assumed).
//
//   - IoFilterManager (I/O Filter — cache/replication/encryption filters —
//     mo.go's mo.IoFilterManager), reachable via
//     ServiceContent.IoFilterManager (types.go's ServiceContent.IoFilterManager
//     *ManagedObjectReference, xml:"ioFilterManager") — already in memory
//     from login, no round trip needed, the same "read straight off
//     ServiceContent, no Properties call" pattern system.go's handleAbout
//     already uses for ServiceContent.About. No object.* wrapper exists for
//     IoFilterManager either, so every handler here also dials raw vim25
//     SOAP methods directly. 8 real methods confirmed the same way (methods.go
//     + "The parameters of `IoFilterManager.Xxx`." doc comments in types.go):
//     InstallIoFilter_Task, UninstallIoFilter_Task, UpgradeIoFilter_Task,
//     QueryIoFilterInfo, QueryIoFilterIssues, QueryDisksUsingFilter,
//     ResolveInstallationErrorsOnCluster_Task,
//     ResolveInstallationErrorsOnHost_Task. Every one of these except
//     ResolveInstallationErrorsOnHost_Task takes a "compRes"/"cluster"
//     ManagedObjectReference that "must be a cluster" per its own doc
//     comment — resolved here via resolveClusterComputeResource
//     (generated_inventory_compute.go), already reading args["cluster"] the
//     same way every other cluster-scoped tool in this project does; no new
//     resolver needed. Install/Upgrade's optional VibSslTrust field
//     (BaseIoFilterManagerSslTrust, a polymorphic xsi:type union gated behind
//     vim:"8.0.3.0") is deliberately NOT exposed as a tool argument — it has
//     no simple JSON shape decodeJSONArg could target without a concrete
//     subtype, and every field using it is optional with a documented
//     zero-value fallback ("if unset, the server certificate is validated
//     against the trusted root certificates").
//
// Class, evidence-based (per method group, not per file — see toolMode's
// doc comment for why one file may register two classes via two separate
// registerXTools functions, one withClass call each):
//
//   - HostVFlashManager → modeVSphereGeneral. Confirmed structurally: vcsim's
//     ESX() model (a standalone-host connection) populates
//     HostConfigManager.VFlashManager with a real, non-nil MoRef
//     (govmomi@.../simulator/esx/host_system.go:
//     "VFlashManager: &types.ManagedObjectReference{Type: "HostVFlashManager",
//     Value: "ha-vflash-manager"}") — vFlash is a per-host storage feature a
//     bare ESXi host genuinely exposes on its own, not something that only
//     exists once a host joins a vCenter-managed cluster (unlike DRS/HA).
//     simulator/vpx/task_manager.go also carries the identical
//     "host.VFlashManager.*" task-description keys, confirming the same
//     per-host object is reachable unchanged once under vCenter too.
//   - IoFilterManager → modeVCenterOnly. Confirmed structurally, the same
//     nil-vs-populated evidence generated_vm_ft.go's top doc comment uses for
//     its own judgment calls, but here it is not even a judgment call:
//     simulator/esx/service_content.go sets
//     "IoFilterManager: (*types.ManagedObjectReference)(nil)" — a standalone
//     ESXi connection's ServiceContent.IoFilterManager is a genuine nil
//     pointer, not merely unpopulated by vcsim's model — while
//     simulator/vpx/service_content.go sets a real MoRef
//     ("IoFilterManager: &types.ManagedObjectReference{Type: "IoFilterManager",
//     Value: "IoFilterManager"}"). I/O Filters are also documented VMware
//     product behavior as a vCenter-managed, cluster-scoped feature (every
//     non-host-scoped method here takes a "compRes" that "must be a
//     cluster") — consistent with the nil-on-ESX() evidence, not contradicting
//     it. iofilterManagerRef below returns a clean error instead of dialing
//     a zero-value MoRef when this field is nil.
//
// Tier: read-only Query/Get methods (HostGetVFlashModuleDefaultConfig,
// QueryIoFilterInfo, QueryIoFilterIssues, QueryDisksUsingFilter) are
// r.register, no confirm/tier. Every mutation (Configure*/Remove* on
// HostVFlashManager; Install/Uninstall/Upgrade/ResolveInstallationErrorsOn*
// on IoFilterManager) is registerDestructive/tier2 (disruptive — reconfigures
// storage caching/filtering behavior or installs/removes host-level VIB
// packages — but reversible: reconfigure/reinstall undoes each one; nothing
// here destroys VM data or is permanently unrecoverable the way
// vmware_vm_destroy/snapshot_remove are).
//
// vcsim coverage: NONE of the 13 methods across both managers have a
// simulator-side handler — confirmed by grepping the entire
// govmomi@v0.55.1/simulator tree for "HostVFlashManager"/"IoFilterManager"
// outside the esx/ and vpx/ description-only subpackages (task-key labels
// and default-model MoRef/nil wiring only, zero `func (m *HostVFlashManager)`
// or `func (m *IoFilterManager)` receivers anywhere) — and directly by
// simulator/model.go's own loadObject comment: "No vcsim wrapper for this
// type, e.g. IoFilterManager". Every mutating/query call therefore reaches
// vcsim's dispatcher and comes back with a clean server-side fault (Method
// NotFound-style, or ManagedObjectNotFound for the fabricated MoRefs vcsim's
// ESX()/VPX() models set but never back with a live object), never a
// recovered panic or an unknown-tool wiring bug — proven by
// TestStorageAdvancedTools_ReachesServer below via assertReachesServer, the
// same helper generated_vm_ft_test.go and
// generated_host_iscsi_portbinding_test.go use for their own unsimulated
// methods. Behavioral validation is expected against a real vCenter-managed
// cluster (IoFilterManager) / a real ESXi host with vFlash-capable SSDs
// (HostVFlashManager).
func registerHostVFlashTools(r *Registry) {
	hostArg := map[string]interface{}{
		"type":        "string",
		"description": `Host identifier: a name/pattern (e.g. "esxi-01.local") or a full inventory path (e.g. "/ha-datacenter/host/esxi-01.local/esxi-01.local") as returned by vmware_list_hosts. Must resolve to exactly one host.`,
	}
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}

	r.registerDestructive("vmware_host_vflash_configure_resource_ex",
		"Create/expand a host's vFlash resource pool from one or more local SSD disks, given their device paths. Reversible via vmware_host_vflash_remove_resource.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host": hostArg,
				"device_paths": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": `SCSI disk device path names that identify the disks to add to the vFlash resource pool, e.g. ["/vmfs/devices/disks/naa.xxx"] — see the host's ScsiDisk devices.`,
				},
				"confirm": confirmArg,
			},
			"required": []interface{}{"host", "device_paths", "confirm"},
		},
		Tool{Handler: handleHostVFlashConfigureResourceEx},
	)

	r.registerDestructive("vmware_host_vflash_configure_resource",
		"Configure a host's vFlash resource pool from an already-created VFFS (Virtual Flash File System) volume, given its UUID. Reversible via vmware_host_vflash_remove_resource.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":      hostArg,
				"vffs_uuid": map[string]interface{}{"type": "string", "description": "UUID of the VFFS volume backing the vFlash resource pool."},
				"confirm":   confirmArg,
			},
			"required": []interface{}{"host", "vffs_uuid", "confirm"},
		},
		Tool{Handler: handleHostVFlashConfigureResource},
	)

	r.registerDestructive("vmware_host_vflash_configure_cache",
		"Configure a host's vFlash swap cache: the default vFlash module for per-VM read/write caches and the amount of vFlash resource reserved for host swap cache. Setting the reservation to 0 disables the host swap cache. Reversible by calling again with different values.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":                   hostArg,
				"default_vflash_module":  map[string]interface{}{"type": "string", "description": "Name of the default vFlash module used for read-write caches associated with this host's VMs (can be overridden per-VMDK)."},
				"swap_cache_reservation_gb": map[string]interface{}{"type": "integer", "description": "Amount of vFlash resource (in GB) reserved for the host swap cache. 0 (the default if omitted) disables the host swap cache."},
				"confirm":                confirmArg,
			},
			"required": []interface{}{"host", "default_vflash_module", "confirm"},
		},
		Tool{Handler: handleHostVFlashConfigureCache},
	)

	r.register("vmware_host_vflash_get_module_default_config",
		"Get the default vFlash Read Cache configuration (block size, cache reservation) for a named vFlash module on a host.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":          hostArg,
				"vflash_module": map[string]interface{}{"type": "string", "description": `Name of the vFlash module, e.g. "vfc" (the default VMware vFlash Read Cache module).`},
			},
			"required": []interface{}{"host", "vflash_module"},
		},
		Tool{Handler: handleHostVFlashGetModuleDefaultConfig},
	)

	r.registerDestructive("vmware_host_vflash_remove_resource",
		"Remove a host's vFlash resource pool entirely, freeing the underlying disks/VFFS volume. Reversible via vmware_host_vflash_configure_resource_ex/vmware_host_vflash_configure_resource.",
		tier2,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"host": hostArg, "confirm": confirmArg},
			"required":   []interface{}{"host", "confirm"},
		},
		Tool{Handler: handleHostVFlashRemoveResource},
	)
}

// registerIoFilterTools adds the 8 IoFilterManager tools — see this file's
// top doc comment for the method list, evidence for the modeVCenterOnly
// classification, and vcsim coverage.
func registerIoFilterTools(r *Registry) {
	clusterArg := map[string]interface{}{
		"type":        "string",
		"description": `Cluster identifier: a name/pattern (e.g. "cluster-01") or a full inventory path, as returned by vmware_list_clusters. Must resolve to exactly one cluster — every IoFilterManager method scoped by "compRes" requires a ClusterComputeResource, not a standalone host or other compute resource.`,
	}
	hostArg := map[string]interface{}{
		"type":        "string",
		"description": `Host identifier: a name/pattern (e.g. "esxi-01.local") or a full inventory path, as returned by vmware_list_hosts. Must resolve to exactly one host.`,
	}
	filterIDArg := map[string]interface{}{
		"type":        "string",
		"description": "ID of the I/O filter (as returned by vmware_iofilter_query_info's filter list).",
	}
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}

	r.registerDestructive("vmware_iofilter_install",
		"Install an I/O filter VIB package on every host in a cluster. Reversible via vmware_iofilter_uninstall.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"cluster": clusterArg,
				"vib_url": map[string]interface{}{"type": "string", "description": "URL pointing to the I/O filter VIB package to install."},
				"confirm": confirmArg,
			},
			"required": []interface{}{"cluster", "vib_url", "confirm"},
		},
		Tool{Handler: handleIoFilterInstall},
	)

	r.registerDestructive("vmware_iofilter_uninstall",
		"Uninstall an I/O filter from every host in a cluster. Reversible via vmware_iofilter_install.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"cluster":   clusterArg,
				"filter_id": filterIDArg,
				"confirm":   confirmArg,
			},
			"required": []interface{}{"cluster", "filter_id", "confirm"},
		},
		Tool{Handler: handleIoFilterUninstall},
	)

	r.registerDestructive("vmware_iofilter_upgrade",
		"Upgrade an already-installed I/O filter on every host in a cluster to a new VIB package.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"cluster":   clusterArg,
				"filter_id": filterIDArg,
				"vib_url":   map[string]interface{}{"type": "string", "description": "URL pointing to the new I/O filter VIB package."},
				"confirm":   confirmArg,
			},
			"required": []interface{}{"cluster", "filter_id", "vib_url", "confirm"},
		},
		Tool{Handler: handleIoFilterUpgrade},
	)

	r.register("vmware_iofilter_query_info",
		"List the I/O filters installed on a cluster, with per-host installation status.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"cluster": clusterArg},
			"required":   []interface{}{"cluster"},
		},
		Tool{Handler: handleIoFilterQueryInfo},
	)

	r.register("vmware_iofilter_query_issues",
		"Get outstanding installation/compliance issues for a specific I/O filter on a cluster.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"cluster":   clusterArg,
				"filter_id": filterIDArg,
			},
			"required": []interface{}{"cluster", "filter_id"},
		},
		Tool{Handler: handleIoFilterQueryIssues},
	)

	r.register("vmware_iofilter_query_disks_using_filter",
		"List the virtual disks currently using a specific I/O filter on a cluster.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"cluster":   clusterArg,
				"filter_id": filterIDArg,
			},
			"required": []interface{}{"cluster", "filter_id"},
		},
		Tool{Handler: handleIoFilterQueryDisksUsingFilter},
	)

	r.registerDestructive("vmware_iofilter_resolve_installation_errors_on_cluster",
		"Retry/resolve I/O filter installation errors for every host in a cluster.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"cluster":   clusterArg,
				"filter_id": filterIDArg,
				"confirm":   confirmArg,
			},
			"required": []interface{}{"cluster", "filter_id", "confirm"},
		},
		Tool{Handler: handleIoFilterResolveInstallationErrorsOnCluster},
	)

	r.registerDestructive("vmware_iofilter_resolve_installation_errors_on_host",
		"Retry/resolve I/O filter installation errors for a single host.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":      hostArg,
				"filter_id": filterIDArg,
				"confirm":   confirmArg,
			},
			"required": []interface{}{"host", "filter_id", "confirm"},
		},
		Tool{Handler: handleIoFilterResolveInstallationErrorsOnHost},
	)
}

// vflashManager resolves the host and its HostVFlashManager MoRef in one
// step — object.HostConfigManager has no VFlashManager accessor (see this
// file's top doc comment), so this reads configManager.vFlashManager
// directly via mo.HostSystem, the same Properties-read pattern
// generated_host_iscsi_portbinding.go's hostIscsiManager uses for
// IscsiManager.
func vflashManager(ctx context.Context, client *vmware.Client, args map[string]interface{}) (*object.HostSystem, types.ManagedObjectReference, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return nil, types.ManagedObjectReference{}, err
	}
	var h mo.HostSystem
	if err := host.Properties(ctx, host.Reference(), []string{"configManager.vFlashManager"}, &h); err != nil {
		return nil, types.ManagedObjectReference{}, fmt.Errorf("failed to read vFlash manager for %s: %w", host.InventoryPath, err)
	}
	if h.ConfigManager.VFlashManager == nil {
		return nil, types.ManagedObjectReference{}, fmt.Errorf("host %s does not expose a vFlash manager (no vFlash-capable SSDs?)", host.InventoryPath)
	}
	return host, *h.ConfigManager.VFlashManager, nil
}

// iofilterManagerRef reads IoFilterManager's MoRef straight off
// ServiceContent — already populated at login, no round trip needed, same
// pattern system.go's handleAbout uses for ServiceContent.About. A
// standalone ESXi connection has ServiceContent.IoFilterManager == nil (see
// this file's top doc comment); that is reported as a clean error here
// rather than a zero-value MoRef reaching a SOAP call.
func iofilterManagerRef(client *vmware.Client) (types.ManagedObjectReference, error) {
	ref := client.Client.ServiceContent.IoFilterManager
	if ref == nil {
		return types.ManagedObjectReference{}, fmt.Errorf("this connection has no IoFilterManager (a vCenter-only feature — connect to a vCenter Server, not a standalone ESXi host)")
	}
	return *ref, nil
}

// iofilterFilterID reads and validates the required filter_id argument.
func iofilterFilterID(args map[string]interface{}) (string, error) {
	id, _ := args["filter_id"].(string)
	if id == "" {
		return "", fmt.Errorf("filter_id is required")
	}
	return id, nil
}

// storageAdvancedWaitTask blocks until the task moref returned by one of
// this file's raw methods.Xxx_Task calls completes, wrapping the bare
// types.ManagedObjectReference in a client-side-only *object.Task (no round
// trip until the first real call against it) and delegating to this
// package's existing waitForTask (vm.go) — the same construction
// generated_vm_ft.go's ftWaitTask uses for the same reason (no object.*
// wrapper exists for either HostVFlashManager's or IoFilterManager's task
// methods to hand back an *object.Task directly).
func storageAdvancedWaitTask(ctx context.Context, client *vmware.Client, ref types.ManagedObjectReference) error {
	return waitForTask(ctx, object.NewTask(client.Client.Client, ref))
}

// --- HostVFlashManager handlers ---------------------------------------------

func handleHostVFlashConfigureResourceEx(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, ref, err := vflashManager(ctx, client, args)
	if err != nil {
		return "", err
	}
	raw, ok := args["device_paths"]
	if !ok {
		return "", fmt.Errorf("device_paths is required")
	}
	devicePaths, err := toStringSlice(raw)
	if err != nil {
		return "", fmt.Errorf("invalid device_paths: %w", err)
	}
	if len(devicePaths) == 0 {
		return "", fmt.Errorf("device_paths must be a non-empty array")
	}

	resp, err := methods.ConfigureVFlashResourceEx_Task(ctx, client.Client.Client, &types.ConfigureVFlashResourceEx_Task{
		This:       ref,
		DevicePath: devicePaths,
	})
	if err != nil {
		return "", fmt.Errorf("failed to configure vFlash resource (ex) on %s: %w", host.InventoryPath, err)
	}
	if err := storageAdvancedWaitTask(ctx, client, resp.Returnval); err != nil {
		return "", fmt.Errorf("configure-vflash-resource-ex task failed for %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "device_paths": devicePaths, "result": "vflash_resource_configured"})
}

func handleHostVFlashConfigureResource(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, ref, err := vflashManager(ctx, client, args)
	if err != nil {
		return "", err
	}
	vffsUUID, _ := args["vffs_uuid"].(string)
	if vffsUUID == "" {
		return "", fmt.Errorf("vffs_uuid is required")
	}

	if _, err := methods.HostConfigureVFlashResource(ctx, client.Client.Client, &types.HostConfigureVFlashResource{
		This: ref,
		Spec: types.HostVFlashManagerVFlashResourceConfigSpec{VffsUuid: vffsUUID},
	}); err != nil {
		return "", fmt.Errorf("failed to configure vFlash resource on %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "vffs_uuid": vffsUUID, "result": "vflash_resource_configured"})
}

func handleHostVFlashConfigureCache(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, ref, err := vflashManager(ctx, client, args)
	if err != nil {
		return "", err
	}
	defaultModule, _ := args["default_vflash_module"].(string)
	if defaultModule == "" {
		return "", fmt.Errorf("default_vflash_module is required")
	}
	var swapGB int64
	if v, ok := args["swap_cache_reservation_gb"]; ok {
		n, err := toInt64(v)
		if err != nil {
			return "", fmt.Errorf("invalid swap_cache_reservation_gb: %w", err)
		}
		swapGB = n
	}

	if _, err := methods.HostConfigVFlashCache(ctx, client.Client.Client, &types.HostConfigVFlashCache{
		This: ref,
		Spec: types.HostVFlashManagerVFlashCacheConfigSpec{
			DefaultVFlashModule:      defaultModule,
			SwapCacheReservationInGB: swapGB,
		},
	}); err != nil {
		return "", fmt.Errorf("failed to configure vFlash cache on %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{
		"host":                      host.InventoryPath,
		"default_vflash_module":     defaultModule,
		"swap_cache_reservation_gb": swapGB,
		"result":                    "vflash_cache_configured",
	})
}

func handleHostVFlashGetModuleDefaultConfig(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, ref, err := vflashManager(ctx, client, args)
	if err != nil {
		return "", err
	}
	module, _ := args["vflash_module"].(string)
	if module == "" {
		return "", fmt.Errorf("vflash_module is required")
	}

	resp, err := methods.HostGetVFlashModuleDefaultConfig(ctx, client.Client.Client, &types.HostGetVFlashModuleDefaultConfig{
		This:         ref,
		VFlashModule: module,
	})
	if err != nil {
		return "", fmt.Errorf("failed to get default config for vFlash module %s on %s: %w", module, host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{
		"host":          host.InventoryPath,
		"vflash_module": module,
		"config":        resp.Returnval,
	})
}

func handleHostVFlashRemoveResource(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, ref, err := vflashManager(ctx, client, args)
	if err != nil {
		return "", err
	}

	if _, err := methods.HostRemoveVFlashResource(ctx, client.Client.Client, &types.HostRemoveVFlashResource{This: ref}); err != nil {
		return "", fmt.Errorf("failed to remove vFlash resource on %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "result": "vflash_resource_removed"})
}

// --- IoFilterManager handlers ------------------------------------------------

func handleIoFilterInstall(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	ref, err := iofilterManagerRef(client)
	if err != nil {
		return "", err
	}
	cluster, err := resolveClusterComputeResource(ctx, client, args)
	if err != nil {
		return "", err
	}
	vibURL, _ := args["vib_url"].(string)
	if vibURL == "" {
		return "", fmt.Errorf("vib_url is required")
	}

	resp, err := methods.InstallIoFilter_Task(ctx, client.Client.Client, &types.InstallIoFilter_Task{
		This:    ref,
		VibUrl:  vibURL,
		CompRes: cluster.Reference(),
	})
	if err != nil {
		return "", fmt.Errorf("failed to install I/O filter (vib %s) on cluster %s: %w", vibURL, cluster.InventoryPath, err)
	}
	if err := storageAdvancedWaitTask(ctx, client, resp.Returnval); err != nil {
		return "", fmt.Errorf("install-iofilter task failed for cluster %s: %w", cluster.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"cluster": cluster.InventoryPath, "vib_url": vibURL, "result": "iofilter_installed"})
}

func handleIoFilterUninstall(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	ref, err := iofilterManagerRef(client)
	if err != nil {
		return "", err
	}
	cluster, err := resolveClusterComputeResource(ctx, client, args)
	if err != nil {
		return "", err
	}
	filterID, err := iofilterFilterID(args)
	if err != nil {
		return "", err
	}

	resp, err := methods.UninstallIoFilter_Task(ctx, client.Client.Client, &types.UninstallIoFilter_Task{
		This:     ref,
		FilterId: filterID,
		CompRes:  cluster.Reference(),
	})
	if err != nil {
		return "", fmt.Errorf("failed to uninstall I/O filter %s from cluster %s: %w", filterID, cluster.InventoryPath, err)
	}
	if err := storageAdvancedWaitTask(ctx, client, resp.Returnval); err != nil {
		return "", fmt.Errorf("uninstall-iofilter task failed for %s on cluster %s: %w", filterID, cluster.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"cluster": cluster.InventoryPath, "filter_id": filterID, "result": "iofilter_uninstalled"})
}

func handleIoFilterUpgrade(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	ref, err := iofilterManagerRef(client)
	if err != nil {
		return "", err
	}
	cluster, err := resolveClusterComputeResource(ctx, client, args)
	if err != nil {
		return "", err
	}
	filterID, err := iofilterFilterID(args)
	if err != nil {
		return "", err
	}
	vibURL, _ := args["vib_url"].(string)
	if vibURL == "" {
		return "", fmt.Errorf("vib_url is required")
	}

	resp, err := methods.UpgradeIoFilter_Task(ctx, client.Client.Client, &types.UpgradeIoFilter_Task{
		This:     ref,
		FilterId: filterID,
		CompRes:  cluster.Reference(),
		VibUrl:   vibURL,
	})
	if err != nil {
		return "", fmt.Errorf("failed to upgrade I/O filter %s on cluster %s: %w", filterID, cluster.InventoryPath, err)
	}
	if err := storageAdvancedWaitTask(ctx, client, resp.Returnval); err != nil {
		return "", fmt.Errorf("upgrade-iofilter task failed for %s on cluster %s: %w", filterID, cluster.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"cluster": cluster.InventoryPath, "filter_id": filterID, "vib_url": vibURL, "result": "iofilter_upgraded"})
}

func handleIoFilterQueryInfo(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	ref, err := iofilterManagerRef(client)
	if err != nil {
		return "", err
	}
	cluster, err := resolveClusterComputeResource(ctx, client, args)
	if err != nil {
		return "", err
	}

	resp, err := methods.QueryIoFilterInfo(ctx, client.Client.Client, &types.QueryIoFilterInfo{
		This:    ref,
		CompRes: cluster.Reference(),
	})
	if err != nil {
		return "", fmt.Errorf("failed to query I/O filter info for cluster %s: %w", cluster.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{
		"cluster": cluster.InventoryPath,
		"count":   len(resp.Returnval),
		"filters": resp.Returnval,
	})
}

func handleIoFilterQueryIssues(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	ref, err := iofilterManagerRef(client)
	if err != nil {
		return "", err
	}
	cluster, err := resolveClusterComputeResource(ctx, client, args)
	if err != nil {
		return "", err
	}
	filterID, err := iofilterFilterID(args)
	if err != nil {
		return "", err
	}

	resp, err := methods.QueryIoFilterIssues(ctx, client.Client.Client, &types.QueryIoFilterIssues{
		This:     ref,
		FilterId: filterID,
		CompRes:  cluster.Reference(),
	})
	if err != nil {
		return "", fmt.Errorf("failed to query I/O filter issues for %s on cluster %s: %w", filterID, cluster.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{
		"cluster":   cluster.InventoryPath,
		"filter_id": filterID,
		"issues":    resp.Returnval,
	})
}

func handleIoFilterQueryDisksUsingFilter(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	ref, err := iofilterManagerRef(client)
	if err != nil {
		return "", err
	}
	cluster, err := resolveClusterComputeResource(ctx, client, args)
	if err != nil {
		return "", err
	}
	filterID, err := iofilterFilterID(args)
	if err != nil {
		return "", err
	}

	resp, err := methods.QueryDisksUsingFilter(ctx, client.Client.Client, &types.QueryDisksUsingFilter{
		This:     ref,
		FilterId: filterID,
		CompRes:  cluster.Reference(),
	})
	if err != nil {
		return "", fmt.Errorf("failed to query disks using I/O filter %s on cluster %s: %w", filterID, cluster.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{
		"cluster":   cluster.InventoryPath,
		"filter_id": filterID,
		"count":     len(resp.Returnval),
		"disks":     resp.Returnval,
	})
}

func handleIoFilterResolveInstallationErrorsOnCluster(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	ref, err := iofilterManagerRef(client)
	if err != nil {
		return "", err
	}
	cluster, err := resolveClusterComputeResource(ctx, client, args)
	if err != nil {
		return "", err
	}
	filterID, err := iofilterFilterID(args)
	if err != nil {
		return "", err
	}

	resp, err := methods.ResolveInstallationErrorsOnCluster_Task(ctx, client.Client.Client, &types.ResolveInstallationErrorsOnCluster_Task{
		This:     ref,
		FilterId: filterID,
		Cluster:  cluster.Reference(),
	})
	if err != nil {
		return "", fmt.Errorf("failed to resolve installation errors for I/O filter %s on cluster %s: %w", filterID, cluster.InventoryPath, err)
	}
	if err := storageAdvancedWaitTask(ctx, client, resp.Returnval); err != nil {
		return "", fmt.Errorf("resolve-installation-errors-on-cluster task failed for %s on cluster %s: %w", filterID, cluster.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"cluster": cluster.InventoryPath, "filter_id": filterID, "result": "installation_errors_resolved"})
}

func handleIoFilterResolveInstallationErrorsOnHost(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	ref, err := iofilterManagerRef(client)
	if err != nil {
		return "", err
	}
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}
	filterID, err := iofilterFilterID(args)
	if err != nil {
		return "", err
	}

	resp, err := methods.ResolveInstallationErrorsOnHost_Task(ctx, client.Client.Client, &types.ResolveInstallationErrorsOnHost_Task{
		This:     ref,
		FilterId: filterID,
		Host:     host.Reference(),
	})
	if err != nil {
		return "", fmt.Errorf("failed to resolve installation errors for I/O filter %s on host %s: %w", filterID, host.InventoryPath, err)
	}
	if err := storageAdvancedWaitTask(ctx, client, resp.Returnval); err != nil {
		return "", fmt.Errorf("resolve-installation-errors-on-host task failed for %s on host %s: %w", filterID, host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "filter_id": filterID, "result": "installation_errors_resolved"})
}

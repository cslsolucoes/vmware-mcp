package tools

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// registerDvsTools adds the DistributedVirtualSwitch and
// DistributedVirtualSwitchManager methods that generated_network.go's Fase 5
// pass did NOT cover — that file only transcribed 6 curated
// DistributedVirtualSwitch/DistributedVirtualPortgroup tools
// (vmware_dvs_add_portgroup, vmware_dvs_fetch_dvports, vmware_dvs_reconfigure,
// vmware_dvs_reconfigure_dvport, vmware_dvs_reconfigure_lacp,
// vmware_dvpg_reconfigure) plus vmware_dvpg_reconfigure/
// vmware_opaque_network_summary — confirmed by grepping every generated_*.go
// file in this package for "vmware_dvs" / "vmware_dvpg" before writing this
// one. This file adds the remaining 15 DistributedVirtualSwitch methods
// (PerformDvsProductSpecOperation_Task, MergeDvs_Task,
// AddNetworkResourcePool, UpdateNetworkResourcePool,
// RemoveNetworkResourcePool, EnableNetworkResourceManagement,
// UpdateDvsCapability, MoveDVPort_Task, RectifyDvsHost_Task,
// DVSRollback_Task, UpdateDVSHealthCheckConfig_Task, RefreshDVPortState,
// FetchDVPortKeys, LookupDvPortGroup,
// DvsReconfigureVmVnicNetworkResourcePool_Task) plus the entire
// DistributedVirtualSwitchManager singleton (13 methods:
// QueryAvailableDvsSpec, QueryDvsCompatibleHostSpec,
// QueryCompatibleHostForNewDvs, QueryCompatibleHostForExistingDvs,
// QueryDvsCheckCompatibility, QueryCompatibleVmnicsFromHosts,
// QueryDvsByUuid, QueryDvsConfigTarget, QueryDvsFeatureCapability,
// DVSManagerLookupDvPortGroup, DVSManagerExportEntity_Task,
// DVSManagerImportEntity_Task, RectifyDvsOnHost_Task) — 28 tools total.
//
// No object.* wrapper exists for DistributedVirtualSwitchManager at all
// (confirmed by `find referencia.../govmomi/object -iname "*dvs*"` /
// "*distributed*" turning up only distributed_virtual_switch.go and
// distributed_virtual_portgroup.go, no manager file), and
// object.DistributedVirtualSwitch itself only wraps 5 of its 20 real SOAP
// methods (Reconfigure/AddPortgroup/FetchDVPorts/ReconfigureDVPort/
// ReconfigureLACP — confirmed by reading
// referencia/govmomi/object/distributed_virtual_switch.go directly). Every
// handler in this file therefore dials the raw vim25 SOAP method directly —
// methods.Xxx(ctx, client.Client.Client, &types.Xxx{This: ref, ...}) — the
// same "no object.* wrapper, go straight to methods+types" pattern
// generated_host_iscsi_portbinding.go documents at length for IscsiManager
// and generated_vm_ft.go documents for VirtualMachine's 7 Fault Tolerance
// methods. Every request struct below was read directly from
// vim25/types/types.go (module cache, go.mod pins govmomi v0.55.1), not
// assumed from a brief.
//
// DistributedVirtualSwitchManager MoRef resolution: unlike IscsiManager
// (reached per-host via HostConfigManager.IscsiManager),
// DistributedVirtualSwitchManager is a connection-level singleton reached via
// ServiceContent.DvSwitchManager (`*types.ManagedObjectReference`,
// vim25/types/types.go) — dvsManagerRef below reads that field directly off
// client.Client.Client.ServiceContent (the same *vim25.Client field path
// generated_custom_fields.go/generated_extension.go already use for their
// own ServiceContent-singleton managers), with a nil guard producing a clear
// error instead of building a zero-value MoRef that would silently fault
// ManagedObjectNotFound at the server.
//
// Class: modeVCenterOnly, matching generated_network.go's classification of
// every other DVS/DVPG tool in this project (gen/main.go's vcenterOnlyFiles
// already lists distributed_virtual_switch.go/distributed_virtual_portgroup.go
// — a DVS is created/managed only through vCenter). Confirmed for
// DistributedVirtualSwitchManager specifically too: referencia/govmomi/
// simulator/esx/service_content.go DOES populate DvSwitchManager with a
// MoRef (unlike CustomFieldsManager/ExtensionManager, which are nil there) —
// so the MoRef itself resolves against a standalone ESXi connection — but
// the underlying feature (managing a distributed switch spanning multiple
// hosts) is meaningless without vCenter's multi-host inventory, the same
// "vCenter-managed cluster feature" reasoning generated_vm_ft.go's top doc
// comment already applies to Fault Tolerance. Kept under one toolMode
// (modeVCenterOnly) for every tool in this file — no vsphere-general split
// needed here, unlike generated_network.go's OpaqueNetwork carve-out.
//
// Resolution helpers (all prefixed "dvs" per this group's brief):
//   - dvsResolve resolves the "network" argument to a
//     *object.DistributedVirtualSwitch, thinly wrapping this package's
//     existing resolveDVS (generated_network.go) rather than duplicating its
//     Finder.Network + type-assertion logic.
//   - dvsResolveHosts resolves a "host_names" array to a
//     []types.ManagedObjectReference, one resolveHost (host.go) call per
//     name via a one-key submap — same reuse pattern generated_vm_ft.go's
//     ftSecondaryVM/ftOptionalHost already use for a different argument name.
//   - dvsManagerRef resolves the DistributedVirtualSwitchManager singleton's
//     MoRef, see above.
//   - dvsWaitTask/dvsWaitTaskResult wrap the task moref returned by a
//     methods.Xxx_Task call in a client-side-only *object.Task
//     (object.NewTask — no round trip until the first real call against it,
//     the exact construction generated_task.go's resolveTaskArg and
//     generated_vm_ft.go's ftWaitTask already use for a caller-supplied/
//     handler-returned task moref) and block via this package's existing
//     waitForTask (vm.go) — dvsWaitTaskResult additionally returns
//     TaskInfo.Result for the 2 methods whose real payload comes back only
//     that way (DVSManagerExportEntity_Task's exported backup blobs,
//     DVSManagerImportEntity_Task's outcome), the same "task_result" pattern
//     generated_inventory_folder.go's bulk-Destroy handler and
//     generated_datastore_browser.go's search handlers already use, since
//     plain dvsWaitTask/waitForfTask discard TaskInfo.Result entirely.
//
// Polymorphic Base* input fields — curated to exactly ONE concrete shape
// each (documented at each decode helper below), the same
// "decodeJSONArg (json.Marshal/Unmarshal round trip) cannot populate an
// interface field — confirmed empirically, not assumed" limitation
// generated_extension.go's decodeExtensionArg already documents for
// types.Extension.Description, applied here to 3 fields:
//   - UpdateDVSHealthCheckConfig_Task.HealthCheckConfig
//     ([]BaseDVSHealthCheckConfig) — dvsDecodeHealthCheckConfig, an explicit
//     "type": "vlanMtu"|"teaming" discriminator per array item (the only 2
//     concrete leaf types reachable by embedding, confirmed by grepping
//     types.go for every struct embedding DVSHealthCheckConfig/
//     VMwareDVSHealthCheckConfig).
//   - QueryDvsCheckCompatibility.HostFilterSpec
//     ([]BaseDistributedVirtualSwitchManagerHostDvsFilterSpec) — curated down
//     to ONE concrete shape, types.DistributedVirtualSwitchManagerHostArrayFilter
//     (an explicit "host_names" list + "inclusive" flag), skipping the other
//     2 real concrete filter shapes (HostContainerFilter,
//     HostDvsMembershipFilter) — the array-of-specific-hosts shape is the
//     most directly actionable via this project's name-based host
//     resolution; the other 2 are not exposed by this tool (documented
//     deviation, same spirit as decodeExtensionArg picking exactly 1
//     concrete Description shape).
//   - DVSManagerExportEntity_Task.SelectionSet ([]BaseSelectionSet) —
//     dvsDecodeSelectionSet, an explicit "kind": "dvs"|"dvportgroup"
//     discriminator (the only 2 concrete implementers of BaseSelectionSet in
//     the whole types package, confirmed by grepping types.go for every
//     struct embedding SelectionSet — DVSSelection, DVPortgroupSelection).
//   - UpdateDvsCapability's DVSCapability.FeaturesSupported
//     (BaseDVSFeatureCapability) is a 4th polymorphic field, but is NOT
//     curated: its own doc comment marks it "read-only, with [a narrow
//     third-party-switch] exception" — dvsDecodeCapability rejects it
//     outright with a clear error naming the field, rather than silently
//     dropping caller input or attempting a decode that would just fail with
//     json's generic "cannot unmarshal object into Go struct field" message.
//
// Every other spec-shaped argument in this file (DVSNetworkResourcePoolConfigSpec,
// DistributedVirtualSwitchProductSpec, DistributedVirtualSwitchManagerDvsProductSpec,
// EntityBackupConfig, DvsVmVnicResourcePoolConfigSpec, ...) is a concrete,
// non-polymorphic struct — accepted as a raw JSON object/array matching its
// Go struct field names verbatim via decodeJSONArg, the same "spec object
// matching its Go struct fields" convention generated_network.go's
// vmware_dvs_reconfigure/vmware_dvpg_reconfigure and
// generated_extension.go's vmware_extension_register already use, including
// any nested types.ManagedObjectReference sub-fields (e.g.
// EntityBackupConfig.Container) — those are accepted raw rather than
// resolved by name, since they identify whichever entity a caller's own
// externally-obtained backup blob names, not a fresh lookup this tool can
// offer a clean name for.
//
// vcsim coverage, confirmed by reading referencia/govmomi/simulator/dvs.go
// and simulator/dvs_manager.go directly (not assumed from the method name):
// simulator.DistributedVirtualSwitch implements only 4 receiver methods
// (AddDVPortgroupTask, ReconfigureDvsTask, FetchDVPorts, DestroyTask) — NONE
// of this file's 15 DistributedVirtualSwitch tools has a vcsim handler.
// simulator.DistributedVirtualSwitchManager implements exactly 1
// (DVSManagerLookupDvPortGroup) — the other 12
// DistributedVirtualSwitchManager tools have none.
// generated_dvs_test.go therefore drives every tool except
// vmware_dvsmgr_lookup_dvportgroup with assertReachesServer
// (generated_vm_lifecycle_test.go) — proving the wiring (schema, tier gate,
// dvsResolve/dvsResolveHosts/dvsManagerRef, raw SOAP dispatch) reaches vcsim
// and gets back a clean server-side fault, not an unknown-tool wiring bug or
// a recovered panic — and drives vmware_dvsmgr_lookup_dvportgroup with one
// genuine success path: create a real portgroup via the existing
// vmware_dvs_add_portgroup tool (generated_network.go), read its real
// Key/parent-DVS Uuid off vcsim via a Properties read (the same
// "key"+"config.distributedVirtualSwitch" / "uuid" property paths
// object.DistributedVirtualPortgroup.EthernetCardBackingInfo already reads
// for the exact same switchUuid+portgroupKey pairing, confirmed by reading
// referencia/govmomi/object/distributed_virtual_portgroup.go directly), and
// call the tool with that real pair. Behavioral validation of the other 27
// tools is expected against a real vCenter-managed DVS.
func registerDvsTools(r *Registry) {
	networkArg := map[string]interface{}{
		"type":        "string",
		"description": `Distributed Virtual Switch identifier: a name/pattern (e.g. "DVS0") or a full inventory path, as returned by vmware_list_networks. Must resolve to exactly one DistributedVirtualSwitch (not a plain Network or DistributedVirtualPortgroup).`,
	}
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}
	hostNamesArg := map[string]interface{}{
		"type":        "array",
		"items":       map[string]interface{}{"type": "string"},
		"description": `Host identifiers (name/pattern or full inventory path, see vmware_list_hosts). Each must resolve to exactly one host.`,
	}

	// --- DistributedVirtualSwitch: destructive (tier1/tier2) ---------------

	r.registerDestructive("vmware_dvs_perform_product_spec_operation",
		"Perform a product-spec operation (e.g. \"upgrade\") on a Distributed Virtual Switch.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"network":      networkArg,
				"operation":    map[string]interface{}{"type": "string", "description": `The operation — see DistributedVirtualSwitchProductSpecOperationType_enum. For a VMware DVS, only "upgrade" is valid.`},
				"product_spec": map[string]interface{}{"type": "object", "description": "Optional types.DistributedVirtualSwitchProductSpec JSON object (name/vendor/version/build/...) describing the target product info."},
				"confirm":      confirmArg,
			},
			"required": []interface{}{"network", "operation", "confirm"},
		},
		Tool{Handler: handleDVSPerformProductSpecOperation},
	)

	r.registerDestructive("vmware_dvs_merge",
		"Merge a source Distributed Virtual Switch into this one. The source switch is deleted as part of the merge. Irreversible.",
		tier1,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"network":        networkArg,
				"source_network": map[string]interface{}{"type": "string", "description": "The source DVS to merge into \"network\" and delete. Same resolution rules as \"network\"."},
				"confirm":        confirmArg,
			},
			"required": []interface{}{"network", "source_network", "confirm"},
		},
		Tool{Handler: handleDVSMerge},
	)

	r.registerDestructive("vmware_dvs_add_network_resource_pool",
		"Add one or more network resource pools to a Distributed Virtual Switch (for network I/O control).",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"network":     networkArg,
				"config_spec": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "object"}, "description": "Array of types.DVSNetworkResourcePoolConfigSpec JSON objects. \"name\" is required per spec; \"key\" is ignored for adds."},
				"confirm":     confirmArg,
			},
			"required": []interface{}{"network", "config_spec", "confirm"},
		},
		Tool{Handler: handleDVSAddNetworkResourcePool},
	)

	r.registerDestructive("vmware_dvs_update_network_resource_pool",
		"Update one or more existing network resource pools on a Distributed Virtual Switch.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"network":     networkArg,
				"config_spec": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "object"}, "description": "Array of types.DVSNetworkResourcePoolConfigSpec JSON objects. \"key\" identifies the pool to update."},
				"confirm":     confirmArg,
			},
			"required": []interface{}{"network", "config_spec", "confirm"},
		},
		Tool{Handler: handleDVSUpdateNetworkResourcePool},
	)

	r.registerDestructive("vmware_dvs_remove_network_resource_pool",
		"Remove one or more network resource pools from a Distributed Virtual Switch. Reversible via vmware_dvs_add_network_resource_pool.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"network": networkArg,
				"keys":    map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Network resource pool keys to remove."},
				"confirm": confirmArg,
			},
			"required": []interface{}{"network", "keys", "confirm"},
		},
		Tool{Handler: handleDVSRemoveNetworkResourcePool},
	)

	r.registerDestructive("vmware_dvs_enable_network_resource_management",
		"Enable or disable network I/O control (network resource management) on a Distributed Virtual Switch.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"network": networkArg,
				"enable":  map[string]interface{}{"type": "boolean", "description": "true to enable network I/O control, false to disable it."},
				"confirm": confirmArg,
			},
			"required": []interface{}{"network", "enable", "confirm"},
		},
		Tool{Handler: handleDVSEnableNetworkResourceManagement},
	)

	r.registerDestructive("vmware_dvs_update_capability",
		"Update the switch-level capability flags of a Distributed Virtual Switch (which config levels vCenter users may modify).",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"network": networkArg,
				"capability": map[string]interface{}{
					"type":        "object",
					"description": `A types.DVSCapability JSON object matching its Go struct field names ("dvsOperationSupported", "dvPortGroupOperationSupported", "dvPortOperationSupported", "compatibleHostComponentProductInfo"). "featuresSupported" is NOT supported by this tool (a polymorphic, largely read-only field — see generated_dvs.go's top doc comment) and is rejected with a clear error if supplied.`,
				},
				"confirm": confirmArg,
			},
			"required": []interface{}{"network", "capability", "confirm"},
		},
		Tool{Handler: handleDVSUpdateCapability},
	)

	r.registerDestructive("vmware_dvs_move_dvport",
		"Move one or more ports of a Distributed Virtual Switch into a different portgroup, or to the switch itself.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"network":                   networkArg,
				"port_keys":                 map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Keys of the ports to move."},
				"destination_portgroup_key": map[string]interface{}{"type": "string", "description": "Key of the destination portgroup. Omit to move the ports directly under the switch."},
				"confirm":                   confirmArg,
			},
			"required": []interface{}{"network", "port_keys", "confirm"},
		},
		Tool{Handler: handleDVSMoveDVPort},
	)

	r.registerDestructive("vmware_dvs_rectify_host",
		"Rectify (reconcile) the configuration of one or more hosts that are members of a Distributed Virtual Switch.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"network":    networkArg,
				"host_names": hostNamesArg,
				"confirm":    confirmArg,
			},
			"required": []interface{}{"network", "confirm"},
		},
		Tool{Handler: handleDVSRectifyHost},
	)

	r.registerDestructive("vmware_dvs_rollback",
		"Roll back a Distributed Virtual Switch's configuration, optionally to a previously exported backup.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"network": networkArg,
				"entity_backup": map[string]interface{}{
					"type":        "object",
					"description": `Optional types.EntityBackupConfig JSON object ("entityType", "configBlob" base64-encoded, "key", "name", "container", "configVersion") as previously returned by vmware_dvsmgr_export_entity. Omit to roll back to the switch's last known-good configuration.`,
				},
				"confirm": confirmArg,
			},
			"required": []interface{}{"network", "confirm"},
		},
		Tool{Handler: handleDVSRollback},
	)

	r.registerDestructive("vmware_dvs_update_health_check_config",
		"Update the VLAN/MTU and teaming health check configuration of a Distributed Virtual Switch.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"network": networkArg,
				"health_check_config": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "object"},
					"description": `Array of {"type": "vlanMtu"|"teaming", "enable": bool, "interval": int} objects — one entry per health check kind (curated shape; see generated_dvs.go's top doc comment for why this project picks an explicit "type" discriminator instead of accepting the raw polymorphic types.BaseDVSHealthCheckConfig shape).`,
				},
				"confirm": confirmArg,
			},
			"required": []interface{}{"network", "health_check_config", "confirm"},
		},
		Tool{Handler: handleDVSUpdateHealthCheckConfig},
	)

	r.registerDestructive("vmware_dvs_refresh_dvport_state",
		"Force a refresh of the runtime state of one or more (or, if omitted, all) ports on a Distributed Virtual Switch.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"network":   networkArg,
				"port_keys": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Keys of the ports to refresh. Omit to refresh every port."},
				"confirm":   confirmArg,
			},
			"required": []interface{}{"network", "confirm"},
		},
		Tool{Handler: handleDVSRefreshDVPortState},
	)

	r.registerDestructive("vmware_dvs_reconfigure_vmvnic_network_resource_pool",
		"Add, update, or remove virtual NIC network resource pools on a Distributed Virtual Switch.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"network":     networkArg,
				"config_spec": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "object"}, "description": `Array of types.DvsVmVnicResourcePoolConfigSpec JSON objects — "operation" ("add"/"edit"/"remove", see ConfigSpecOperation_enum) plus "key"/"name"/"description"/"allocationInfo" as applicable.`},
				"confirm":     confirmArg,
			},
			"required": []interface{}{"network", "config_spec", "confirm"},
		},
		Tool{Handler: handleDVSReconfigureVmVnicNetworkResourcePool},
	)

	// --- DistributedVirtualSwitch: read-only --------------------------------

	r.register("vmware_dvs_fetch_dvport_keys",
		"List the port keys of a Distributed Virtual Switch, optionally filtered by criteria.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"network":  networkArg,
				"criteria": map[string]interface{}{"type": "object", "description": "Optional types.DistributedVirtualSwitchPortCriteria JSON object to filter results. Omit to fetch every port's key."},
			},
			"required": []interface{}{"network"},
		},
		Tool{Handler: handleDVSFetchDVPortKeys},
	)

	r.register("vmware_dvs_lookup_dvportgroup",
		"Look up a Distributed Virtual Portgroup on this switch by its port group key.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"network":       networkArg,
				"portgroup_key": map[string]interface{}{"type": "string", "description": "The key that identifies a portgroup of this switch."},
			},
			"required": []interface{}{"network", "portgroup_key"},
		},
		Tool{Handler: handleDVSLookupDVPortGroup},
	)

	// --- DistributedVirtualSwitchManager: read-only -------------------------

	r.register("vmware_dvsmgr_query_available_dvs_spec",
		"List the Distributed Virtual Switch product specs (versions) this vCenter supports creating.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"recommended": map[string]interface{}{"type": "boolean", "description": "true for only recommended versions, false for only non-recommended versions. Omit for all supported versions."},
			},
		},
		Tool{Handler: handleDVSMgrQueryAvailableDvsSpec},
	)

	r.register("vmware_dvsmgr_query_dvs_compatible_host_spec",
		"List the host product specs compatible with a given (or the default) Distributed Virtual Switch product spec.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"switch_product_spec": map[string]interface{}{"type": "object", "description": "Optional types.DistributedVirtualSwitchProductSpec JSON object. Omit to use the default used for DVS creation."},
			},
		},
		Tool{Handler: handleDVSMgrQueryDvsCompatibleHostSpec},
	)

	r.register("vmware_dvsmgr_query_compatible_host_for_new_dvs",
		"List the hosts under a container (Datacenter/ComputeResource/Folder) compatible with creating a new Distributed Virtual Switch.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"container_path":      map[string]interface{}{"type": "string", "description": "Full inventory path of a Datacenter, ComputeResource, or Folder to search under."},
				"recursive":           map[string]interface{}{"type": "boolean", "description": "Search subfolders too. For a Datacenter container, applies to its host folder."},
				"switch_product_spec": map[string]interface{}{"type": "object", "description": "Optional types.DistributedVirtualSwitchProductSpec JSON object. Omit to use the default used for DVS creation."},
			},
			"required": []interface{}{"container_path"},
		},
		Tool{Handler: handleDVSMgrQueryCompatibleHostForNewDvs},
	)

	r.register("vmware_dvsmgr_query_compatible_host_for_existing_dvs",
		"List the hosts under a container compatible with joining an existing Distributed Virtual Switch.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"container_path": map[string]interface{}{"type": "string", "description": "Full inventory path of a Datacenter, ComputeResource, or Folder to search under."},
				"recursive":      map[string]interface{}{"type": "boolean", "description": "Search subfolders too."},
				"network":        networkArg,
			},
			"required": []interface{}{"container_path", "network"},
		},
		Tool{Handler: handleDVSMgrQueryCompatibleHostForExistingDvs},
	)

	r.register("vmware_dvsmgr_query_dvs_check_compatibility",
		"Check host compatibility with a Distributed Virtual Switch product spec, optionally restricted to a specific list of hosts.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"container_path":   map[string]interface{}{"type": "string", "description": "Full inventory path of a Datacenter, ComputeResource, or Folder whose hosts are checked."},
				"recursive":        map[string]interface{}{"type": "boolean", "description": "Include hosts of all levels in the hierarchy under container_path."},
				"dvs_product_spec": map[string]interface{}{"type": "object", "description": "Optional types.DistributedVirtualSwitchManagerDvsProductSpec JSON object. Omit to use the default used for DVS creation."},
				"host_names":       map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": `Optional: restrict the check to exactly these hosts (curated shape — builds one types.DistributedVirtualSwitchManagerHostArrayFilter; see generated_dvs.go's top doc comment). Omit to check every host under container_path.`},
				"inclusive":        map[string]interface{}{"type": "boolean", "description": `Only used with host_names: true (default) to check exactly those hosts, false to check every host EXCEPT those. Ignored if host_names is omitted.`},
			},
			"required": []interface{}{"container_path"},
		},
		Tool{Handler: handleDVSMgrQueryDvsCheckCompatibility},
	)

	r.register("vmware_dvsmgr_query_compatible_vmnics_from_hosts",
		"List the physical NICs on the given hosts that are compatible with a Distributed Virtual Switch. Requires vCenter 8.0.0.1 or later.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"network":    networkArg,
				"host_names": hostNamesArg,
			},
			"required": []interface{}{"network", "host_names"},
		},
		Tool{Handler: handleDVSMgrQueryCompatibleVmnicsFromHosts},
	)

	r.register("vmware_dvsmgr_query_dvs_by_uuid",
		"Resolve a Distributed Virtual Switch by its UUID.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"uuid": map[string]interface{}{"type": "string", "description": "The DVS UUID."}},
			"required":   []interface{}{"uuid"},
		},
		Tool{Handler: handleDVSMgrQueryDvsByUuid},
	)

	r.register("vmware_dvsmgr_query_dvs_config_target",
		"Get the DVS configuration target (available physical NICs, VLANs, etc.) for a host and/or a specific Distributed Virtual Switch.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":    map[string]interface{}{"type": "string", "description": "Optional host identifier (name/pattern or full inventory path). Required if calling this against a connection where the host cannot otherwise be inferred."},
				"network": map[string]interface{}{"type": "string", "description": `Optional DVS identifier. Omit to encompass every DVS available on the host.`},
			},
		},
		Tool{Handler: handleDVSMgrQueryDvsConfigTarget},
	)

	r.register("vmware_dvsmgr_query_dvs_feature_capability",
		"Get the version-specific feature capabilities available for a Distributed Virtual Switch product spec.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"switch_product_spec": map[string]interface{}{"type": "object", "description": "Optional types.DistributedVirtualSwitchProductSpec JSON object. Omit to use the default used for DVS creation."},
			},
		},
		Tool{Handler: handleDVSMgrQueryDvsFeatureCapability},
	)

	r.register("vmware_dvsmgr_lookup_dvportgroup",
		"Look up a Distributed Virtual Portgroup by its parent switch UUID and its portgroup key.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"switch_uuid":   map[string]interface{}{"type": "string", "description": "UUID of the DistributedVirtualSwitch."},
				"portgroup_key": map[string]interface{}{"type": "string", "description": "The key that identifies a DistributedVirtualPortgroup."},
			},
			"required": []interface{}{"switch_uuid", "portgroup_key"},
		},
		Tool{Handler: handleDVSMgrLookupDvPortGroup},
	)

	r.register("vmware_dvsmgr_export_entity",
		"Export the configuration of one or more Distributed Virtual Switches/Portgroups as an opaque backup blob, for later restore via vmware_dvs_rollback or vmware_dvsmgr_import_entity. Read-only (does not modify any switch) despite being a *_Task method — see generated_dvs.go's top doc comment.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"selections": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "object"},
					"description": `Array of {"kind": "dvs", "dvs_uuid": "..."} or {"kind": "dvportgroup", "dvs_uuid": "...", "portgroup_keys": ["..."]} objects — one entry per entity to export (curated shape; see generated_dvs.go's top doc comment for why this project picks an explicit "kind" discriminator instead of accepting the raw polymorphic types.BaseSelectionSet shape).`,
				},
			},
			"required": []interface{}{"selections"},
		},
		Tool{Handler: handleDVSMgrExportEntity},
	)

	// --- DistributedVirtualSwitchManager: destructive (tier2) ---------------

	r.registerDestructive("vmware_dvsmgr_import_entity",
		"Restore (create or overwrite) one or more Distributed Virtual Switches/Portgroups from backup blobs previously produced by vmware_dvsmgr_export_entity.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"entity_backup": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "object"},
					"description": `Array of types.EntityBackupConfig JSON objects ("entityType", "configBlob" base64-encoded, "key", "name", "container", "configVersion"), as returned by vmware_dvsmgr_export_entity.`,
				},
				"import_type": map[string]interface{}{"type": "string", "description": "How to apply the backup — see EntityImportType_enum (e.g. \"createEntityWithNewIdentifier\", \"createEntityWithOriginalIdentifier\", \"applyToEntitySpecified\")."},
				"confirm":     confirmArg,
			},
			"required": []interface{}{"entity_backup", "import_type", "confirm"},
		},
		Tool{Handler: handleDVSMgrImportEntity},
	)

	r.registerDestructive("vmware_dvsmgr_rectify_dvs_on_host",
		"Rectify (reconcile) the Distributed Virtual Switch configuration on one or more hosts.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host_names": hostNamesArg,
				"confirm":    confirmArg,
			},
			"required": []interface{}{"host_names", "confirm"},
		},
		Tool{Handler: handleDVSMgrRectifyDvsOnHost},
	)
}

// dvsResolve resolves the required "network" argument to a
// *object.DistributedVirtualSwitch, thinly wrapping this package's existing
// resolveDVS (generated_network.go) rather than duplicating its
// Finder.Network + type-assertion logic — see this file's top doc comment.
func dvsResolve(ctx context.Context, client *vmware.Client, args map[string]interface{}) (*object.DistributedVirtualSwitch, error) {
	return resolveDVS(ctx, client, args)
}

// dvsResolveHosts resolves each of names to a HostSystem's
// ManagedObjectReference via this package's existing resolveHost (host.go),
// one one-key-submap lookup per name — same reuse pattern generated_vm_ft.go's
// ftSecondaryVM/ftOptionalHost already use for a different argument name.
func dvsResolveHosts(ctx context.Context, client *vmware.Client, names []string) ([]types.ManagedObjectReference, error) {
	refs := make([]types.ManagedObjectReference, 0, len(names))
	for i, name := range names {
		h, err := resolveHost(ctx, client, map[string]interface{}{"host": name})
		if err != nil {
			return nil, fmt.Errorf("host_names[%d]: %w", i, err)
		}
		refs = append(refs, h.Reference())
	}
	return refs, nil
}

// dvsManagerRef resolves the DistributedVirtualSwitchManager singleton's
// MoRef from this connection's ServiceContent.DvSwitchManager — see this
// file's top doc comment for why (no object.* wrapper exists to guard this
// the way object.GetCustomFieldsManager/GetExtensionManager do for their own
// singletons).
func dvsManagerRef(client *vmware.Client) (types.ManagedObjectReference, error) {
	sc := client.Client.Client.ServiceContent
	if sc.DvSwitchManager == nil {
		return types.ManagedObjectReference{}, fmt.Errorf("DistributedVirtualSwitchManager is not available on this connection")
	}
	return *sc.DvSwitchManager, nil
}

// dvsWaitTask mirrors generated_vm_ft.go's ftWaitTask: wraps the task moref
// returned by one of this file's raw methods.Xxx_Task calls in a
// client-side-only *object.Task (object.NewTask — no round trip until the
// first real call against it) and blocks via this package's existing
// waitForTask (vm.go), discarding TaskInfo.Result — used by every _Task
// handler below except the 2 whose real payload comes back only through
// Result (see dvsWaitTaskResult).
func dvsWaitTask(ctx context.Context, client *vmware.Client, ref types.ManagedObjectReference) error {
	return waitForTask(ctx, object.NewTask(client.Client.Client, ref))
}

// dvsWaitTaskResult is dvsWaitTask's counterpart for
// DVSManagerExportEntity_Task/DVSManagerImportEntity_Task, whose real
// payload comes back only through TaskInfo.Result — same "task_result"
// pattern generated_inventory_folder.go's bulk-Destroy handler and
// generated_datastore_browser.go's search handlers already use, since plain
// dvsWaitTask/waitForTask discard it entirely.
func dvsWaitTaskResult(ctx context.Context, client *vmware.Client, ref types.ManagedObjectReference) (types.AnyType, error) {
	info, err := object.NewTask(client.Client.Client, ref).WaitForResult(ctx)
	if err != nil {
		return nil, err
	}
	return info.Result, nil
}

// dvsDecodeHealthCheckConfig curates ONE concrete shape per array item for
// the polymorphic HealthCheckConfig ([]types.BaseDVSHealthCheckConfig) field
// of UpdateDVSHealthCheckConfig_Task — see this file's top doc comment.
func dvsDecodeHealthCheckConfig(raw interface{}) ([]types.BaseDVSHealthCheckConfig, error) {
	arr, ok := raw.([]interface{})
	if !ok || len(arr) == 0 {
		return nil, fmt.Errorf("health_check_config is required and must be a non-empty array")
	}
	out := make([]types.BaseDVSHealthCheckConfig, 0, len(arr))
	for i, item := range arr {
		m, ok := item.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("health_check_config[%d] must be a JSON object", i)
		}
		var base types.DVSHealthCheckConfig
		if v, has := m["enable"]; has {
			b, ok := v.(bool)
			if !ok {
				return nil, fmt.Errorf("health_check_config[%d].enable must be a boolean", i)
			}
			base.Enable = &b
		}
		if v, has := m["interval"]; has {
			iv, err := toInt32(v)
			if err != nil {
				return nil, fmt.Errorf("health_check_config[%d].interval: %w", i, err)
			}
			base.Interval = iv
		}
		kind, _ := m["type"].(string)
		switch kind {
		case "vlanMtu":
			out = append(out, &types.VMwareDVSVlanMtuHealthCheckConfig{VMwareDVSHealthCheckConfig: types.VMwareDVSHealthCheckConfig{DVSHealthCheckConfig: base}})
		case "teaming":
			out = append(out, &types.VMwareDVSTeamingHealthCheckConfig{VMwareDVSHealthCheckConfig: types.VMwareDVSHealthCheckConfig{DVSHealthCheckConfig: base}})
		default:
			return nil, fmt.Errorf(`health_check_config[%d].type must be "vlanMtu" or "teaming", got %q`, i, kind)
		}
	}
	return out, nil
}

// dvsHostArrayFilter curates the polymorphic HostFilterSpec
// ([]types.BaseDistributedVirtualSwitchManagerHostDvsFilterSpec) field of
// QueryDvsCheckCompatibility down to exactly one concrete shape,
// types.DistributedVirtualSwitchManagerHostArrayFilter — see this file's top
// doc comment. Returns (nil, nil) when hostNames is empty (the field is
// optional — omitting it checks every host under the container instead).
func dvsHostArrayFilter(ctx context.Context, client *vmware.Client, hostNames []string, inclusive bool) ([]types.BaseDistributedVirtualSwitchManagerHostDvsFilterSpec, error) {
	if len(hostNames) == 0 {
		return nil, nil
	}
	refs, err := dvsResolveHosts(ctx, client, hostNames)
	if err != nil {
		return nil, err
	}
	filter := &types.DistributedVirtualSwitchManagerHostArrayFilter{
		DistributedVirtualSwitchManagerHostDvsFilterSpec: types.DistributedVirtualSwitchManagerHostDvsFilterSpec{Inclusive: inclusive},
		Host: refs,
	}
	return []types.BaseDistributedVirtualSwitchManagerHostDvsFilterSpec{filter}, nil
}

// dvsDecodeSelectionSet curates ONE concrete shape per array item for the
// polymorphic SelectionSet ([]types.BaseSelectionSet) field of
// DVSManagerExportEntity_Task — see this file's top doc comment.
func dvsDecodeSelectionSet(raw interface{}) ([]types.BaseSelectionSet, error) {
	arr, ok := raw.([]interface{})
	if !ok || len(arr) == 0 {
		return nil, fmt.Errorf("selections is required and must be a non-empty array")
	}
	out := make([]types.BaseSelectionSet, 0, len(arr))
	for i, item := range arr {
		m, ok := item.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("selections[%d] must be a JSON object", i)
		}
		uuid, _ := m["dvs_uuid"].(string)
		if uuid == "" {
			return nil, fmt.Errorf("selections[%d].dvs_uuid is required", i)
		}
		kind, _ := m["kind"].(string)
		switch kind {
		case "dvs":
			out = append(out, &types.DVSSelection{DvsUuid: uuid})
		case "dvportgroup":
			keys, err := toStringSlice(m["portgroup_keys"])
			if err != nil {
				return nil, fmt.Errorf("selections[%d].portgroup_keys: %w", i, err)
			}
			out = append(out, &types.DVPortgroupSelection{DvsUuid: uuid, PortgroupKey: keys})
		default:
			return nil, fmt.Errorf(`selections[%d].kind must be "dvs" or "dvportgroup", got %q`, i, kind)
		}
	}
	return out, nil
}

// dvsDecodeCapability decodes the "capability" argument into a
// types.DVSCapability, rejecting the one polymorphic sub-field
// (FeaturesSupported) this tool does not curate — see this file's top doc
// comment.
func dvsDecodeCapability(raw interface{}) (types.DVSCapability, error) {
	var capability types.DVSCapability
	m, ok := raw.(map[string]interface{})
	if !ok {
		return capability, fmt.Errorf("capability must be a JSON object")
	}
	if _, has := m["featuresSupported"]; has {
		return capability, fmt.Errorf(`capability.featuresSupported is not supported by this tool (types.DVSCapability.FeaturesSupported is a polymorphic, largely read-only field — see generated_dvs.go's top doc comment); omit it`)
	}
	if err := decodeJSONArg(m, &capability); err != nil {
		return capability, err
	}
	return capability, nil
}

// === DistributedVirtualSwitch handlers ====================================

func handleDVSPerformProductSpecOperation(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	dvs, err := dvsResolve(ctx, client, args)
	if err != nil {
		return "", err
	}
	operation, _ := args["operation"].(string)
	if operation == "" {
		return "", fmt.Errorf("operation is required")
	}
	var productSpec *types.DistributedVirtualSwitchProductSpec
	if args["product_spec"] != nil {
		productSpec = &types.DistributedVirtualSwitchProductSpec{}
		if err := decodeJSONArg(args["product_spec"], productSpec); err != nil {
			return "", fmt.Errorf("invalid product_spec: %w", err)
		}
	}

	resp, err := methods.PerformDvsProductSpecOperation_Task(ctx, client.Client.Client, &types.PerformDvsProductSpecOperation_Task{
		This:        dvs.Reference(),
		Operation:   operation,
		ProductSpec: productSpec,
	})
	if err != nil {
		return "", fmt.Errorf("failed to perform product spec operation %q on %s: %w", operation, dvs.InventoryPath, err)
	}
	if err := dvsWaitTask(ctx, client, resp.Returnval); err != nil {
		return "", fmt.Errorf("perform-product-spec-operation task failed for %s: %w", dvs.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"network": dvs.InventoryPath, "operation": operation, "result": "product_spec_operation_performed"})
}

func handleDVSMerge(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	target, err := dvsResolve(ctx, client, args)
	if err != nil {
		return "", err
	}
	sourceName, _ := args["source_network"].(string)
	if sourceName == "" {
		return "", fmt.Errorf("source_network is required")
	}
	source, err := dvsResolve(ctx, client, map[string]interface{}{"network": sourceName})
	if err != nil {
		return "", err
	}

	resp, err := methods.MergeDvs_Task(ctx, client.Client.Client, &types.MergeDvs_Task{
		This: target.Reference(),
		Dvs:  source.Reference(),
	})
	if err != nil {
		return "", fmt.Errorf("failed to merge %s into %s: %w", source.InventoryPath, target.InventoryPath, err)
	}
	if err := dvsWaitTask(ctx, client, resp.Returnval); err != nil {
		return "", fmt.Errorf("merge task failed merging %s into %s: %w", source.InventoryPath, target.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"network": target.InventoryPath, "source_network": source.InventoryPath, "result": "merged"})
}

func handleDVSAddNetworkResourcePool(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	dvs, err := dvsResolve(ctx, client, args)
	if err != nil {
		return "", err
	}
	specs, err := decodeDVSNetworkResourcePoolConfigSpecs(args)
	if err != nil {
		return "", err
	}

	if _, err := methods.AddNetworkResourcePool(ctx, client.Client.Client, &types.AddNetworkResourcePool{
		This:       dvs.Reference(),
		ConfigSpec: specs,
	}); err != nil {
		return "", fmt.Errorf("failed to add network resource pool(s) to %s: %w", dvs.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"network": dvs.InventoryPath, "result": "network_resource_pool_added", "count": len(specs)})
}

func handleDVSUpdateNetworkResourcePool(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	dvs, err := dvsResolve(ctx, client, args)
	if err != nil {
		return "", err
	}
	specs, err := decodeDVSNetworkResourcePoolConfigSpecs(args)
	if err != nil {
		return "", err
	}

	if _, err := methods.UpdateNetworkResourcePool(ctx, client.Client.Client, &types.UpdateNetworkResourcePool{
		This:       dvs.Reference(),
		ConfigSpec: specs,
	}); err != nil {
		return "", fmt.Errorf("failed to update network resource pool(s) on %s: %w", dvs.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"network": dvs.InventoryPath, "result": "network_resource_pool_updated", "count": len(specs)})
}

// decodeDVSNetworkResourcePoolConfigSpecs decodes the "config_spec" argument
// shared by vmware_dvs_add_network_resource_pool/
// vmware_dvs_update_network_resource_pool into
// []types.DVSNetworkResourcePoolConfigSpec — a concrete, non-polymorphic
// struct, accepted as raw JSON matching its Go field names (see this file's
// top doc comment's "every other spec-shaped argument" note).
func decodeDVSNetworkResourcePoolConfigSpecs(args map[string]interface{}) ([]types.DVSNetworkResourcePoolConfigSpec, error) {
	raw, ok := args["config_spec"].([]interface{})
	if !ok || len(raw) == 0 {
		return nil, fmt.Errorf("config_spec is required and must be a non-empty array")
	}
	specs := make([]types.DVSNetworkResourcePoolConfigSpec, 0, len(raw))
	for i, item := range raw {
		var spec types.DVSNetworkResourcePoolConfigSpec
		if err := decodeJSONArg(item, &spec); err != nil {
			return nil, fmt.Errorf("invalid config_spec[%d]: %w", i, err)
		}
		specs = append(specs, spec)
	}
	return specs, nil
}

func handleDVSRemoveNetworkResourcePool(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	dvs, err := dvsResolve(ctx, client, args)
	if err != nil {
		return "", err
	}
	keys, err := toStringSlice(args["keys"])
	if err != nil || len(keys) == 0 {
		return "", fmt.Errorf("keys is required and must be a non-empty array of strings")
	}

	if _, err := methods.RemoveNetworkResourcePool(ctx, client.Client.Client, &types.RemoveNetworkResourcePool{
		This: dvs.Reference(),
		Key:  keys,
	}); err != nil {
		return "", fmt.Errorf("failed to remove network resource pool(s) from %s: %w", dvs.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"network": dvs.InventoryPath, "result": "network_resource_pool_removed", "keys": keys})
}

func handleDVSEnableNetworkResourceManagement(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	dvs, err := dvsResolve(ctx, client, args)
	if err != nil {
		return "", err
	}
	enable, ok := args["enable"].(bool)
	if !ok {
		return "", fmt.Errorf("enable is required")
	}

	if _, err := methods.EnableNetworkResourceManagement(ctx, client.Client.Client, &types.EnableNetworkResourceManagement{
		This:   dvs.Reference(),
		Enable: enable,
	}); err != nil {
		return "", fmt.Errorf("failed to set network resource management (enable=%v) on %s: %w", enable, dvs.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"network": dvs.InventoryPath, "enable": enable, "result": "network_resource_management_set"})
}

func handleDVSUpdateCapability(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	dvs, err := dvsResolve(ctx, client, args)
	if err != nil {
		return "", err
	}
	if args["capability"] == nil {
		return "", fmt.Errorf("capability is required")
	}
	capability, err := dvsDecodeCapability(args["capability"])
	if err != nil {
		return "", fmt.Errorf("invalid capability: %w", err)
	}

	if _, err := methods.UpdateDvsCapability(ctx, client.Client.Client, &types.UpdateDvsCapability{
		This:       dvs.Reference(),
		Capability: capability,
	}); err != nil {
		return "", fmt.Errorf("failed to update capability on %s: %w", dvs.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"network": dvs.InventoryPath, "result": "capability_updated"})
}

func handleDVSMoveDVPort(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	dvs, err := dvsResolve(ctx, client, args)
	if err != nil {
		return "", err
	}
	portKeys, err := toStringSlice(args["port_keys"])
	if err != nil || len(portKeys) == 0 {
		return "", fmt.Errorf("port_keys is required and must be a non-empty array of strings")
	}
	destKey, _ := args["destination_portgroup_key"].(string)

	resp, err := methods.MoveDVPort_Task(ctx, client.Client.Client, &types.MoveDVPort_Task{
		This:                    dvs.Reference(),
		PortKey:                 portKeys,
		DestinationPortgroupKey: destKey,
	})
	if err != nil {
		return "", fmt.Errorf("failed to move port(s) on %s: %w", dvs.InventoryPath, err)
	}
	if err := dvsWaitTask(ctx, client, resp.Returnval); err != nil {
		return "", fmt.Errorf("move-dvport task failed for %s: %w", dvs.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"network": dvs.InventoryPath, "port_keys": portKeys, "destination_portgroup_key": destKey, "result": "ports_moved"})
}

func handleDVSRectifyHost(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	dvs, err := dvsResolve(ctx, client, args)
	if err != nil {
		return "", err
	}
	var hostRefs []types.ManagedObjectReference
	if raw, ok := args["host_names"]; ok && raw != nil {
		names, err := toStringSlice(raw)
		if err != nil {
			return "", fmt.Errorf("invalid host_names: %w", err)
		}
		hostRefs, err = dvsResolveHosts(ctx, client, names)
		if err != nil {
			return "", err
		}
	}

	resp, err := methods.RectifyDvsHost_Task(ctx, client.Client.Client, &types.RectifyDvsHost_Task{
		This:  dvs.Reference(),
		Hosts: hostRefs,
	})
	if err != nil {
		return "", fmt.Errorf("failed to rectify host(s) on %s: %w", dvs.InventoryPath, err)
	}
	if err := dvsWaitTask(ctx, client, resp.Returnval); err != nil {
		return "", fmt.Errorf("rectify-dvs-host task failed for %s: %w", dvs.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"network": dvs.InventoryPath, "host_count": len(hostRefs), "result": "hosts_rectified"})
}

func handleDVSRollback(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	dvs, err := dvsResolve(ctx, client, args)
	if err != nil {
		return "", err
	}
	var backup *types.EntityBackupConfig
	if args["entity_backup"] != nil {
		backup = &types.EntityBackupConfig{}
		if err := decodeJSONArg(args["entity_backup"], backup); err != nil {
			return "", fmt.Errorf("invalid entity_backup: %w", err)
		}
	}

	resp, err := methods.DVSRollback_Task(ctx, client.Client.Client, &types.DVSRollback_Task{
		This:         dvs.Reference(),
		EntityBackup: backup,
	})
	if err != nil {
		return "", fmt.Errorf("failed to roll back %s: %w", dvs.InventoryPath, err)
	}
	if err := dvsWaitTask(ctx, client, resp.Returnval); err != nil {
		return "", fmt.Errorf("rollback task failed for %s: %w", dvs.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"network": dvs.InventoryPath, "result": "rolled_back"})
}

func handleDVSUpdateHealthCheckConfig(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	dvs, err := dvsResolve(ctx, client, args)
	if err != nil {
		return "", err
	}
	configs, err := dvsDecodeHealthCheckConfig(args["health_check_config"])
	if err != nil {
		return "", err
	}

	resp, err := methods.UpdateDVSHealthCheckConfig_Task(ctx, client.Client.Client, &types.UpdateDVSHealthCheckConfig_Task{
		This:              dvs.Reference(),
		HealthCheckConfig: configs,
	})
	if err != nil {
		return "", fmt.Errorf("failed to update health check config on %s: %w", dvs.InventoryPath, err)
	}
	if err := dvsWaitTask(ctx, client, resp.Returnval); err != nil {
		return "", fmt.Errorf("update-health-check-config task failed for %s: %w", dvs.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"network": dvs.InventoryPath, "count": len(configs), "result": "health_check_config_updated"})
}

func handleDVSRefreshDVPortState(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	dvs, err := dvsResolve(ctx, client, args)
	if err != nil {
		return "", err
	}
	var portKeys []string
	if raw, ok := args["port_keys"]; ok && raw != nil {
		portKeys, err = toStringSlice(raw)
		if err != nil {
			return "", fmt.Errorf("invalid port_keys: %w", err)
		}
	}

	if _, err := methods.RefreshDVPortState(ctx, client.Client.Client, &types.RefreshDVPortState{
		This:     dvs.Reference(),
		PortKeys: portKeys,
	}); err != nil {
		return "", fmt.Errorf("failed to refresh port state on %s: %w", dvs.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"network": dvs.InventoryPath, "port_keys": portKeys, "result": "dvport_state_refreshed"})
}

func handleDVSReconfigureVmVnicNetworkResourcePool(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	dvs, err := dvsResolve(ctx, client, args)
	if err != nil {
		return "", err
	}
	raw, ok := args["config_spec"].([]interface{})
	if !ok || len(raw) == 0 {
		return "", fmt.Errorf("config_spec is required and must be a non-empty array")
	}
	specs := make([]types.DvsVmVnicResourcePoolConfigSpec, 0, len(raw))
	for i, item := range raw {
		var spec types.DvsVmVnicResourcePoolConfigSpec
		if err := decodeJSONArg(item, &spec); err != nil {
			return "", fmt.Errorf("invalid config_spec[%d]: %w", i, err)
		}
		specs = append(specs, spec)
	}

	resp, err := methods.DvsReconfigureVmVnicNetworkResourcePool_Task(ctx, client.Client.Client, &types.DvsReconfigureVmVnicNetworkResourcePool_Task{
		This:       dvs.Reference(),
		ConfigSpec: specs,
	})
	if err != nil {
		return "", fmt.Errorf("failed to reconfigure vNIC network resource pool(s) on %s: %w", dvs.InventoryPath, err)
	}
	if err := dvsWaitTask(ctx, client, resp.Returnval); err != nil {
		return "", fmt.Errorf("reconfigure-vmvnic-network-resource-pool task failed for %s: %w", dvs.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"network": dvs.InventoryPath, "count": len(specs), "result": "vmvnic_network_resource_pool_reconfigured"})
}

func handleDVSFetchDVPortKeys(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	dvs, err := dvsResolve(ctx, client, args)
	if err != nil {
		return "", err
	}
	var criteria *types.DistributedVirtualSwitchPortCriteria
	if args["criteria"] != nil {
		criteria = &types.DistributedVirtualSwitchPortCriteria{}
		if err := decodeJSONArg(args["criteria"], criteria); err != nil {
			return "", fmt.Errorf("invalid criteria: %w", err)
		}
	}

	resp, err := methods.FetchDVPortKeys(ctx, client.Client.Client, &types.FetchDVPortKeys{
		This:     dvs.Reference(),
		Criteria: criteria,
	})
	if err != nil {
		return "", fmt.Errorf("failed to fetch port keys for %s: %w", dvs.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"network": dvs.InventoryPath, "count": len(resp.Returnval), "port_keys": resp.Returnval})
}

func handleDVSLookupDVPortGroup(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	dvs, err := dvsResolve(ctx, client, args)
	if err != nil {
		return "", err
	}
	pgKey, _ := args["portgroup_key"].(string)
	if pgKey == "" {
		return "", fmt.Errorf("portgroup_key is required")
	}

	resp, err := methods.LookupDvPortGroup(ctx, client.Client.Client, &types.LookupDvPortGroup{
		This:         dvs.Reference(),
		PortgroupKey: pgKey,
	})
	if err != nil {
		return "", fmt.Errorf("failed to look up portgroup %q on %s: %w", pgKey, dvs.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"network": dvs.InventoryPath, "portgroup_key": pgKey, "found": resp.Returnval != nil, "portgroup": resp.Returnval})
}

// === DistributedVirtualSwitchManager handlers ==============================

func handleDVSMgrQueryAvailableDvsSpec(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	ref, err := dvsManagerRef(client)
	if err != nil {
		return "", err
	}
	var recommended *bool
	if v, ok := args["recommended"].(bool); ok {
		recommended = &v
	}

	resp, err := methods.QueryAvailableDvsSpec(ctx, client.Client.Client, &types.QueryAvailableDvsSpec{
		This:        ref,
		Recommended: recommended,
	})
	if err != nil {
		return "", fmt.Errorf("failed to query available DVS specs: %w", err)
	}

	return marshalJSON(map[string]interface{}{"count": len(resp.Returnval), "specs": resp.Returnval})
}

func handleDVSMgrQueryDvsCompatibleHostSpec(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	ref, err := dvsManagerRef(client)
	if err != nil {
		return "", err
	}
	spec, err := decodeOptionalSwitchProductSpec(args)
	if err != nil {
		return "", err
	}

	resp, err := methods.QueryDvsCompatibleHostSpec(ctx, client.Client.Client, &types.QueryDvsCompatibleHostSpec{
		This:              ref,
		SwitchProductSpec: spec,
	})
	if err != nil {
		return "", fmt.Errorf("failed to query DVS-compatible host specs: %w", err)
	}

	return marshalJSON(map[string]interface{}{"count": len(resp.Returnval), "specs": resp.Returnval})
}

// decodeOptionalSwitchProductSpec decodes the "switch_product_spec" argument
// shared by several DistributedVirtualSwitchManager read-only tools into a
// *types.DistributedVirtualSwitchProductSpec — a concrete, non-polymorphic
// struct accepted as raw JSON (see this file's top doc comment).
func decodeOptionalSwitchProductSpec(args map[string]interface{}) (*types.DistributedVirtualSwitchProductSpec, error) {
	if args["switch_product_spec"] == nil {
		return nil, nil
	}
	spec := &types.DistributedVirtualSwitchProductSpec{}
	if err := decodeJSONArg(args["switch_product_spec"], spec); err != nil {
		return nil, fmt.Errorf("invalid switch_product_spec: %w", err)
	}
	return spec, nil
}

func handleDVSMgrQueryCompatibleHostForNewDvs(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	ref, err := dvsManagerRef(client)
	if err != nil {
		return "", err
	}
	containerPath, _ := args["container_path"].(string)
	if containerPath == "" {
		return "", fmt.Errorf("container_path is required")
	}
	container, err := resolveEntityRef(ctx, client, containerPath)
	if err != nil {
		return "", err
	}
	recursive, _ := args["recursive"].(bool)
	spec, err := decodeOptionalSwitchProductSpec(args)
	if err != nil {
		return "", err
	}

	resp, err := methods.QueryCompatibleHostForNewDvs(ctx, client.Client.Client, &types.QueryCompatibleHostForNewDvs{
		This:              ref,
		Container:         container,
		Recursive:         recursive,
		SwitchProductSpec: spec,
	})
	if err != nil {
		return "", fmt.Errorf("failed to query compatible hosts for a new DVS under %q: %w", containerPath, err)
	}

	return marshalJSON(map[string]interface{}{"container_path": containerPath, "count": len(resp.Returnval), "hosts": resp.Returnval})
}

func handleDVSMgrQueryCompatibleHostForExistingDvs(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	ref, err := dvsManagerRef(client)
	if err != nil {
		return "", err
	}
	containerPath, _ := args["container_path"].(string)
	if containerPath == "" {
		return "", fmt.Errorf("container_path is required")
	}
	container, err := resolveEntityRef(ctx, client, containerPath)
	if err != nil {
		return "", err
	}
	recursive, _ := args["recursive"].(bool)
	dvs, err := dvsResolve(ctx, client, args)
	if err != nil {
		return "", err
	}

	resp, err := methods.QueryCompatibleHostForExistingDvs(ctx, client.Client.Client, &types.QueryCompatibleHostForExistingDvs{
		This:      ref,
		Container: container,
		Recursive: recursive,
		Dvs:       dvs.Reference(),
	})
	if err != nil {
		return "", fmt.Errorf("failed to query compatible hosts for %s under %q: %w", dvs.InventoryPath, containerPath, err)
	}

	return marshalJSON(map[string]interface{}{"container_path": containerPath, "network": dvs.InventoryPath, "count": len(resp.Returnval), "hosts": resp.Returnval})
}

func handleDVSMgrQueryDvsCheckCompatibility(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	ref, err := dvsManagerRef(client)
	if err != nil {
		return "", err
	}
	containerPath, _ := args["container_path"].(string)
	if containerPath == "" {
		return "", fmt.Errorf("container_path is required")
	}
	container, err := resolveEntityRef(ctx, client, containerPath)
	if err != nil {
		return "", err
	}
	recursive, _ := args["recursive"].(bool)

	var dvsProductSpec *types.DistributedVirtualSwitchManagerDvsProductSpec
	if args["dvs_product_spec"] != nil {
		dvsProductSpec = &types.DistributedVirtualSwitchManagerDvsProductSpec{}
		if err := decodeJSONArg(args["dvs_product_spec"], dvsProductSpec); err != nil {
			return "", fmt.Errorf("invalid dvs_product_spec: %w", err)
		}
	}

	var hostNames []string
	if raw, ok := args["host_names"]; ok && raw != nil {
		hostNames, err = toStringSlice(raw)
		if err != nil {
			return "", fmt.Errorf("invalid host_names: %w", err)
		}
	}
	inclusive := true
	if v, ok := args["inclusive"].(bool); ok {
		inclusive = v
	}
	filterSpec, err := dvsHostArrayFilter(ctx, client, hostNames, inclusive)
	if err != nil {
		return "", err
	}

	resp, err := methods.QueryDvsCheckCompatibility(ctx, client.Client.Client, &types.QueryDvsCheckCompatibility{
		This: ref,
		HostContainer: types.DistributedVirtualSwitchManagerHostContainer{
			Container: container,
			Recursive: recursive,
		},
		DvsProductSpec: dvsProductSpec,
		HostFilterSpec: filterSpec,
	})
	if err != nil {
		return "", fmt.Errorf("failed to check DVS compatibility under %q: %w", containerPath, err)
	}

	return marshalJSON(map[string]interface{}{"container_path": containerPath, "count": len(resp.Returnval), "results": resp.Returnval})
}

func handleDVSMgrQueryCompatibleVmnicsFromHosts(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	ref, err := dvsManagerRef(client)
	if err != nil {
		return "", err
	}
	dvs, err := dvsResolve(ctx, client, args)
	if err != nil {
		return "", err
	}
	names, err := toStringSlice(args["host_names"])
	if err != nil || len(names) == 0 {
		return "", fmt.Errorf("host_names is required and must be a non-empty array of strings")
	}
	hostRefs, err := dvsResolveHosts(ctx, client, names)
	if err != nil {
		return "", err
	}

	resp, err := methods.QueryCompatibleVmnicsFromHosts(ctx, client.Client.Client, &types.QueryCompatibleVmnicsFromHosts{
		This:  ref,
		Hosts: hostRefs,
		Dvs:   dvs.Reference(),
	})
	if err != nil {
		return "", fmt.Errorf("failed to query compatible vmnics from hosts for %s: %w", dvs.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"network": dvs.InventoryPath, "count": len(resp.Returnval), "results": resp.Returnval})
}

func handleDVSMgrQueryDvsByUuid(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	ref, err := dvsManagerRef(client)
	if err != nil {
		return "", err
	}
	uuid, _ := args["uuid"].(string)
	if uuid == "" {
		return "", fmt.Errorf("uuid is required")
	}

	resp, err := methods.QueryDvsByUuid(ctx, client.Client.Client, &types.QueryDvsByUuid{
		This: ref,
		Uuid: uuid,
	})
	if err != nil {
		return "", fmt.Errorf("failed to query DVS by uuid %q: %w", uuid, err)
	}

	return marshalJSON(map[string]interface{}{"uuid": uuid, "found": resp.Returnval != nil, "network": resp.Returnval})
}

func handleDVSMgrQueryDvsConfigTarget(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	ref, err := dvsManagerRef(client)
	if err != nil {
		return "", err
	}
	var hostRef *types.ManagedObjectReference
	var hostPath string
	if name, ok := args["host"].(string); ok && name != "" {
		h, err := resolveHost(ctx, client, map[string]interface{}{"host": name})
		if err != nil {
			return "", err
		}
		r := h.Reference()
		hostRef = &r
		hostPath = h.InventoryPath
	}
	var dvsRef *types.ManagedObjectReference
	var networkPath string
	if name, ok := args["network"].(string); ok && name != "" {
		dvs, err := dvsResolve(ctx, client, map[string]interface{}{"network": name})
		if err != nil {
			return "", err
		}
		r := dvs.Reference()
		dvsRef = &r
		networkPath = dvs.InventoryPath
	}

	resp, err := methods.QueryDvsConfigTarget(ctx, client.Client.Client, &types.QueryDvsConfigTarget{
		This: ref,
		Host: hostRef,
		Dvs:  dvsRef,
	})
	if err != nil {
		return "", fmt.Errorf("failed to query DVS config target: %w", err)
	}

	result := map[string]interface{}{"config_target": resp.Returnval}
	if hostPath != "" {
		result["host"] = hostPath
	}
	if networkPath != "" {
		result["network"] = networkPath
	}
	return marshalJSON(result)
}

func handleDVSMgrQueryDvsFeatureCapability(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	ref, err := dvsManagerRef(client)
	if err != nil {
		return "", err
	}
	spec, err := decodeOptionalSwitchProductSpec(args)
	if err != nil {
		return "", err
	}

	resp, err := methods.QueryDvsFeatureCapability(ctx, client.Client.Client, &types.QueryDvsFeatureCapability{
		This:              ref,
		SwitchProductSpec: spec,
	})
	if err != nil {
		return "", fmt.Errorf("failed to query DVS feature capability: %w", err)
	}

	return marshalJSON(map[string]interface{}{"capability": resp.Returnval})
}

func handleDVSMgrLookupDvPortGroup(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	ref, err := dvsManagerRef(client)
	if err != nil {
		return "", err
	}
	switchUuid, _ := args["switch_uuid"].(string)
	if switchUuid == "" {
		return "", fmt.Errorf("switch_uuid is required")
	}
	pgKey, _ := args["portgroup_key"].(string)
	if pgKey == "" {
		return "", fmt.Errorf("portgroup_key is required")
	}

	resp, err := methods.DVSManagerLookupDvPortGroup(ctx, client.Client.Client, &types.DVSManagerLookupDvPortGroup{
		This:         ref,
		SwitchUuid:   switchUuid,
		PortgroupKey: pgKey,
	})
	if err != nil {
		return "", fmt.Errorf("failed to look up portgroup %q on switch %q: %w", pgKey, switchUuid, err)
	}

	return marshalJSON(map[string]interface{}{"switch_uuid": switchUuid, "portgroup_key": pgKey, "found": resp.Returnval != nil, "portgroup": resp.Returnval})
}

func handleDVSMgrExportEntity(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	ref, err := dvsManagerRef(client)
	if err != nil {
		return "", err
	}
	selections, err := dvsDecodeSelectionSet(args["selections"])
	if err != nil {
		return "", err
	}

	resp, err := methods.DVSManagerExportEntity_Task(ctx, client.Client.Client, &types.DVSManagerExportEntity_Task{
		This:         ref,
		SelectionSet: selections,
	})
	if err != nil {
		return "", fmt.Errorf("failed to export entity/entities: %w", err)
	}
	result, err := dvsWaitTaskResult(ctx, client, resp.Returnval)
	if err != nil {
		return "", fmt.Errorf("export-entity task failed: %w", err)
	}

	return marshalJSON(map[string]interface{}{"count": len(selections), "result": "exported", "entity_backup": result})
}

func handleDVSMgrImportEntity(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	ref, err := dvsManagerRef(client)
	if err != nil {
		return "", err
	}
	raw, ok := args["entity_backup"].([]interface{})
	if !ok || len(raw) == 0 {
		return "", fmt.Errorf("entity_backup is required and must be a non-empty array")
	}
	backups := make([]types.EntityBackupConfig, 0, len(raw))
	for i, item := range raw {
		var backup types.EntityBackupConfig
		if err := decodeJSONArg(item, &backup); err != nil {
			return "", fmt.Errorf("invalid entity_backup[%d]: %w", i, err)
		}
		backups = append(backups, backup)
	}
	importType, _ := args["import_type"].(string)
	if importType == "" {
		return "", fmt.Errorf("import_type is required")
	}

	resp, err := methods.DVSManagerImportEntity_Task(ctx, client.Client.Client, &types.DVSManagerImportEntity_Task{
		This:         ref,
		EntityBackup: backups,
		ImportType:   importType,
	})
	if err != nil {
		return "", fmt.Errorf("failed to import entity/entities: %w", err)
	}
	result, err := dvsWaitTaskResult(ctx, client, resp.Returnval)
	if err != nil {
		return "", fmt.Errorf("import-entity task failed: %w", err)
	}

	out := map[string]interface{}{"count": len(backups), "import_type": importType, "result": "imported"}
	if result != nil {
		out["task_result"] = result
	}
	return marshalJSON(out)
}

func handleDVSMgrRectifyDvsOnHost(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	ref, err := dvsManagerRef(client)
	if err != nil {
		return "", err
	}
	names, err := toStringSlice(args["host_names"])
	if err != nil || len(names) == 0 {
		return "", fmt.Errorf("host_names is required and must be a non-empty array of strings")
	}
	hostRefs, err := dvsResolveHosts(ctx, client, names)
	if err != nil {
		return "", err
	}

	resp, err := methods.RectifyDvsOnHost_Task(ctx, client.Client.Client, &types.RectifyDvsOnHost_Task{
		This:  ref,
		Hosts: hostRefs,
	})
	if err != nil {
		return "", fmt.Errorf("failed to rectify DVS on host(s): %w", err)
	}
	if err := dvsWaitTask(ctx, client, resp.Returnval); err != nil {
		return "", fmt.Errorf("rectify-dvs-on-host task failed: %w", err)
	}

	return marshalJSON(map[string]interface{}{"host_count": len(hostRefs), "result": "rectified"})
}

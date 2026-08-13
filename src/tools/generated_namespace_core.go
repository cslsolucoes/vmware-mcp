// Package tools — generated_namespace_core.go is Fase 8a (Grupo NS-A) of the
// codegen plan (".workspace/plans/MCPVMWare2026-08-10-175300-plano-cobertura-completa-api-codegen.plan.md"),
// covering referencia/govmomi/vapi/namespace/namespace.go — 22 tools, one
// registration function (registerNamespaceCoreTools). Grupo NS-A is one of 4
// parallel groups splitting govmomi's vapi/namespace package this wave;
// networks.go/supervisorsvc.go/namespace_v2.go belong to sibling groups
// (NS-B/C/D) and are NOT covered here — handler/helper names in this file are
// prefixed "NamespaceCore" specifically to avoid collisions with those
// sibling files in the same package.
//
// Same REST/JSON architecture as generated_tags.go/generated_vcenter_template.go
// (Fase 8a Wave 1): namespace.Manager wraps a *rest.Client, and every struct
// here (NamespacesInstanceCreateSpec, EnableClusterSpec, VirtualMachineClassCreateSpec,
// ...) already carries real `json:"..."` tags — tool arguments are decoded
// straight into those concrete structs via decodeJSONArg (json.Marshal then
// json.Unmarshal — see generated_vm_lifecycle.go's decodeJSONArg doc
// comment), reused as-is here, not reimplemented. For several handlers
// (CreateNamespace, UpdateNamespace, RegisterVM, EnableCluster,
// EnableOnComputeCluster, EnableOnZones) the ENTIRE tool call's args map is
// decoded wholesale straight into the target govmomi spec struct — safe
// because encoding/json silently ignores map keys with no matching struct
// field (e.g. "confirm", or a sibling path-parameter key), and because every
// property name exposed in this file's JSON schemas is deliberately spelled
// to match the target struct's own json tag exactly (see the two carve-outs
// below) — so there is no separate hand-rolled mapping layer to keep in sync
// or get wrong.
//
// mode=vcenter-only — the entire vapi/* domain requires a vCenter Server
// Appliance (VAMI/VAPI session), never valid against a standalone ESXi host.
//
// Entity resolution: NONE of the 22 methods in this file take a
// types.ManagedObjectReference parameter (confirmed by reading every method
// signature in namespace.go) — every identifier (cluster id, namespace name,
// vm_class id, distributed-switch id, RegisterVMSpec.VM) is a raw string in
// the vSphere Namespaces / Tanzu Supervisor REST wire format, not a vim25
// moref and not an inventory path. Per the orchestrator's explicit brief,
// these are decoded as plain strings — no resolveEntityRef/find.InventoryPath
// anywhere in this file, unlike generated_tags.go/generated_authorization.go.
//
// Curation decisions (beyond the ones already made by the orchestrator before
// this file was written — see this project's classification.json + the
// task brief for CreateNamespace/CreateSupportBundle/.../UpdateVmClass tiers,
// and for SupportBundleRequest's exclusion, confirmed independently here by
// re-reading namespace.go line ~1037: it only builds an unsent *http.Request,
// no round trip, and an *http.Request does not serialize usefully to JSON —
// CreateSupportBundle above it already returns the URL+token a caller needs):
//
//   - VirtualMachineClassCreateSpec/VirtualMachineClassUpdateSpec are the ONE
//     pair of structs in this file NOT decoded wholesale from args — their
//     MemoryMb field carries the wire tag `json:"memory_MB"` (inconsistent
//     capitalization vs. every sibling field's lower_snake_case, e.g.
//     "cpu_count"). Exposing that exact spelling as a public tool argument
//     name would be an unnecessary footgun with no compensating benefit (it
//     is not read back from any response, only sent), so
//     vmware_namespace_create_vm_class/vmware_namespace_update_vm_class
//     expose a clean "memory_mb" argument instead and build the struct
//     field-by-field (see vmClassSpecFromArgs below). Every other field of
//     that struct (id, cpu_count, cpu_reservation, memory_reservation,
//     devices, config_spec) already has a clean tag and is copied straight
//     across.
//
//   - EnableClusterSpec (used by vmware_namespace_enable_cluster) carries a
//     second, more surprising wire-tag inconsistency in the same vein:
//     MasterDNSNames is tagged `json:"Master_DNS_names"` — capital M, unlike
//     every sibling *_DNS_* field (master_DNS_search_domains, master_NTP_servers,
//     worker_DNS, master_DNS all start lowercase). Given EnableClusterSpec has
//     20 fields total (several deeply nested — NSX-T, Avi/HA-Proxy load
//     balancer config, workload network specs — see the struct's own doc
//     comments in namespace.go for the full shape) and this whole domain has
//     zero vcsim coverage to validate a hand-rolled remapping against (see
//     below), this one field is deliberately left as the literal wire
//     spelling "Master_DNS_names" in vmware_namespace_enable_cluster's schema
//     (called out explicitly in that tool's own argument description) rather
//     than silently renaming it and risking a mapping bug nothing here can
//     catch. Every other EnableClusterSpec/EnableOnZonesSpec/
//     EnableOnComputeClusterSpec field is exposed under its own exact wire
//     tag for the same reason — these 3 spec types are accepted essentially
//     as documented pass-through objects (schema gives per-field
//     descriptions and required/enum markers, but does not expand
//     ControlPlane/Workloads/LoadBalancerConfigSpec's own nested shape), the
//     same "pass the deeply-nested spec through, document don't
//     re-model" precedent already established by generated_vcenter_template.go's
//     deploymentSpecSchema (vm_config_spec, network_mappings, ...).
//
// vcsim support: NONE. Confirmed directly (not assumed) by grepping
// referencia/govmomi/vapi/simulator/simulator.go for "vapi/namespace" — 0
// matches; that file only imports vapi/library, vapi/rest, vapi/tags,
// vapi/vcenter. vSphere with Tanzu / the Kubernetes Supervisor cluster has no
// simulator implementation anywhere in this codebase, unlike Content
// Library/Tags/vCenter template (Fase 8a Wave 1), which do have real REST
// simulator handlers. Every tool in this file is therefore tested only up to
// "registers correctly, validates required arguments before touching the
// server, gates tier1/tier2 correctly, and reaches vcsim with a clean
// server-side error (not a panic, not 'unknown tool')" via assertReachesServer
// (reused from generated_vm_lifecycle_test.go) — same "vcsim gap, not a bug"
// discipline already applied to generated_authorization.go's DisableMethods/
// EnableMethods and generated_network.go's ReconfigureDVPort/
// UpdateDVSLacpGroupConfig.
package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/vmware/govmomi/vapi/namespace"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

func registerNamespaceCoreTools(r *Registry) {
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}
	clusterIDArg := map[string]interface{}{
		"type":        "string",
		"description": `Supervisor cluster ID — the raw ComputeResource ID string (e.g. "domain-c8"), NOT a full inventory path and NOT resolved via SearchIndex.FindByInventoryPath. This is the vSphere Namespaces / Tanzu Supervisor wire identifier used throughout this API family.`,
	}
	supervisorIDArg := map[string]interface{}{
		"type":        "string",
		"description": `Supervisor ID. For a cluster-based Supervisor this equals the cluster's raw ComputeResource ID string (see vmware_namespace_list_clusters). Not an inventory path.`,
	}
	namespaceNameArg := map[string]interface{}{
		"type":        "string",
		"description": `vSphere Namespace name (the Kubernetes/Supervisor namespace identifier, e.g. "my-namespace").`,
	}
	vmClassIDArg := map[string]interface{}{
		"type":        "string",
		"description": `Virtual Machine Class ID (e.g. "best-effort-2xlarge", or a custom class ID created via vmware_namespace_create_vm_class).`,
	}
	vmServiceSpecArg := map[string]interface{}{
		"type":        "object",
		"description": `Optional VM Service config for the namespace: {"content_libraries": [...library IDs...], "vm_classes": [...VM Class IDs...]}.`,
		"properties": map[string]interface{}{
			"content_libraries": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
			"vm_classes":        map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
		},
	}
	storageSpecsArg := map[string]interface{}{
		"type":        "array",
		"description": `Optional per-storage-policy quota for the namespace.`,
		"items": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"policy": map[string]interface{}{"type": "string", "description": "Storage policy ID."},
				"limit":  map[string]interface{}{"type": "integer", "description": "Storage quota limit in MB. 0/omitted means unlimited."},
			},
			"required": []interface{}{"policy"},
		},
	}
	controlPlaneSchema := map[string]interface{}{
		"type":        "object",
		"description": `Supervisor control plane VM sizing/network config. Full nested shape: {"network": {...ControlPlaneNetwork...}, "login_banner": "...", "size": "TINY|SMALL|MEDIUM|LARGE", "storage_policy": "...", "count": N}. See namespace.ControlPlane/ControlPlaneNetwork in referencia/govmomi/vapi/namespace/namespace.go for the authoritative shape — passed straight through to the server (this domain has no vcsim to validate a hand-built nested schema against), not expanded further here.`,
	}
	workloadsSchema := map[string]interface{}{
		"type":        "object",
		"description": `Workload network/edge/image/storage config for the Supervisor. Full nested shape: {"network": {...}, "edge": {...}, "kube_api_server_options": {...}, "images": {...}, "storage": {...}}. See namespace.Workloads in referencia/govmomi/vapi/namespace/namespace.go for the authoritative shape — passed straight through to the server, not expanded further here.`,
	}

	r.registerDestructive("vmware_namespace_create_namespace",
		"Create a new vSphere Namespace (Tanzu Supervisor namespace) on a Supervisor-enabled cluster.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"cluster":         clusterIDArg,
				"namespace":       namespaceNameArg,
				"vm_service_spec": vmServiceSpecArg,
				"storage_specs":   storageSpecsArg,
				"confirm":         confirmArg,
			},
			"required": []interface{}{"cluster", "namespace", "confirm"},
		},
		Tool{Handler: handleNamespaceCoreCreateNamespace},
	)

	r.registerDestructive("vmware_namespace_create_support_bundle",
		"Generate a Namespaces-related support bundle for a Supervisor cluster and return its download URL and access token. Actually downloading the bundle (issuing the HTTP GET against that URL with the token) is out of scope for this tool — see this file's top doc comment for the corresponding namespace.Manager.SupportBundleRequest exclusion (it only builds an unsent *http.Request, no round trip).",
		tier2,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"id": clusterIDArg, "confirm": confirmArg},
			"required":   []interface{}{"id", "confirm"},
		},
		Tool{Handler: handleNamespaceCoreCreateSupportBundle},
	)

	r.registerDestructive("vmware_namespace_create_vm_class",
		"Create a custom Virtual Machine Class (a reusable CPU/memory/device profile) for use by vSphere Namespaces workloads.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"id":                 map[string]interface{}{"type": "string", "description": "New VM Class ID/name."},
				"cpu_count":          map[string]interface{}{"type": "integer", "description": "Number of virtual CPUs."},
				"memory_mb":          map[string]interface{}{"type": "integer", "description": "Memory in MB."},
				"cpu_reservation":    map[string]interface{}{"type": "integer", "description": "CPU reservation percentage (0-100). Optional."},
				"memory_reservation": map[string]interface{}{"type": "integer", "description": "Memory reservation percentage (0-100). Optional."},
				"devices": map[string]interface{}{
					"type":        "object",
					"description": `Optional passthrough devices: {"direct_path_io_devices": [{"custom_label","device_id","vendor_id"}, ...], "vgpu_devices": [{"profile_name"}, ...]}.`,
				},
				"config_spec": map[string]interface{}{"type": "object", "description": "Optional raw/version-specific extended VM Class config spec, passed through verbatim."},
				"confirm":     confirmArg,
			},
			"required": []interface{}{"id", "cpu_count", "memory_mb", "confirm"},
		},
		Tool{Handler: handleNamespaceCoreCreateVmClass},
	)

	r.registerDestructive("vmware_namespace_delete_namespace",
		"Delete a vSphere Namespace. Irreversible — also removes its workloads.",
		tier1,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"namespace": namespaceNameArg, "confirm": confirmArg},
			"required":   []interface{}{"namespace", "confirm"},
		},
		Tool{Handler: handleNamespaceCoreDeleteNamespace},
	)

	r.registerDestructive("vmware_namespace_delete_vm_class",
		"Delete a Virtual Machine Class. Irreversible.",
		tier1,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"vm_class": vmClassIDArg, "confirm": confirmArg},
			"required":   []interface{}{"vm_class", "confirm"},
		},
		Tool{Handler: handleNamespaceCoreDeleteVmClass},
	)

	r.registerDestructive("vmware_namespace_disable_cluster",
		"Disable vSphere Namespaces (Tanzu Supervisor) on a cluster, removing its Supervisor control plane.",
		tier2,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"id": clusterIDArg, "confirm": confirmArg},
			"required":   []interface{}{"id", "confirm"},
		},
		Tool{Handler: handleNamespaceCoreDisableCluster},
	)

	r.registerDestructive("vmware_namespace_enable_cluster",
		`Enable vSphere Namespaces (Tanzu Supervisor) on the given cluster, using the legacy (pre-8.0.0.1) EnableClusterSpec. See vmware_namespace_enable_on_compute_cluster/vmware_namespace_enable_on_zones for the newer 8.0.0.1+ zonal Supervisor-enablement APIs. Every property below (other than "id"/"confirm") is passed straight through to the server using the exact wire field name shown — including "Master_DNS_names", which the underlying vSphere REST API spells with an inconsistent leading capital letter unlike every sibling *_DNS_* field (documented, not a bug in this tool).`,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"id":                        clusterIDArg,
				"image_storage":             map[string]interface{}{"type": "object", "description": `Required. {"storage_policy": "<storage policy ID>"}.`},
				"master_management_network": map[string]interface{}{"type": "object", "description": `Required. {"mode": "DHCP|STATICRANGE", "network": "...", "floating_IP": "...", "address_range": {...}}.`},
				"service_cidr":              map[string]interface{}{"type": "object", "description": `Required. {"address": "...", "prefix": N} — Kubernetes service CIDR.`},
				"size_hint":                 map[string]interface{}{"type": "string", "enum": []interface{}{"TINY", "SMALL", "MEDIUM", "LARGE"}, "description": "Required. Supervisor control plane sizing."},
				"network_provider":          map[string]interface{}{"type": "string", "enum": []interface{}{"NSXT_CONTAINER_PLUGIN", "VSPHERE_NETWORK"}, "description": "Required."},
				"master_DNS_search_domains": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
				"ncp_cluster_network_spec":  map[string]interface{}{"type": "object", "description": "NSX-T workload network spec — 7.0.0 to 7.0u1 only."},
				"workload_networks_spec":    map[string]interface{}{"type": "object", "description": `7.0u1+, supersedes ncp_cluster_network_spec: {"supervisor_primary_workload_network": {...NetworksCreateSpec...}}.`},
				"Master_DNS_names":          map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": `NOTE the exact key spelling (capital "M") — see this tool's own top-level description.`},
				"master_NTP_servers":        map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
				"ephemeral_storage_policy":  map[string]interface{}{"type": "string"},
				"default_image_repository":  map[string]interface{}{"type": "string"},
				"login_banner":              map[string]interface{}{"type": "string"},
				"worker_DNS":                map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
				"default_image_registry":    map[string]interface{}{"type": "object", "description": `{"hostname": "...", "port": N}.`},
				"master_DNS":                map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
				"master_storage_policy":     map[string]interface{}{"type": "string"},
				"default_kubernetes_service_content_library": map[string]interface{}{"type": "string"},
				"workload_ntp_servers":                       map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
				"load_balancer_config_spec":                  map[string]interface{}{"type": "object", "description": `{"id": "...", "provider": "HA_PROXY|AVI", "address_ranges": [...], "ha_proxy_config_create_spec": {...}, "avi_config_create_spec": {...}}.`},
				"confirm":                                    confirmArg,
			},
			"required": []interface{}{"id", "image_storage", "master_management_network", "service_cidr", "size_hint", "network_provider", "confirm"},
		},
		Tool{Handler: handleNamespaceCoreEnableCluster},
	)

	r.registerDestructive("vmware_namespace_enable_on_compute_cluster",
		"Enable a Supervisor on a single vSphere cluster (8.0.0.1+ Supervisor API). Returns a server response string (implementation-defined; typically empty or an activation identifier).",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"id":            clusterIDArg,
				"zone":          map[string]interface{}{"type": "string", "description": "Optional vSphere Zone ID."},
				"name":          map[string]interface{}{"type": "string", "description": "Name of the new Supervisor."},
				"control_plane": controlPlaneSchema,
				"workloads":     workloadsSchema,
				"confirm":       confirmArg,
			},
			"required": []interface{}{"id", "name", "control_plane", "workloads", "confirm"},
		},
		Tool{Handler: handleNamespaceCoreEnableOnComputeCluster},
	)

	r.registerDestructive("vmware_namespace_enable_on_zones",
		"Enable a Supervisor across a set of vSphere Zones (8.0.0.1+ zonal Supervisor API). Returns a server response string (implementation-defined; typically empty or an activation identifier).",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"zones":         map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "vSphere Zone IDs to enable the Supervisor across. Must be non-empty."},
				"name":          map[string]interface{}{"type": "string", "description": "Name of the new Supervisor."},
				"control_plane": controlPlaneSchema,
				"workloads":     workloadsSchema,
				"confirm":       confirmArg,
			},
			"required": []interface{}{"zones", "name", "control_plane", "workloads", "confirm"},
		},
		Tool{Handler: handleNamespaceCoreEnableOnZones},
	)

	r.register("vmware_namespace_get_namespace",
		"Fetch detailed information (config status, description, stats, VM Service spec, storage specs) for a vSphere Namespace.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"namespace": namespaceNameArg},
			"required":   []interface{}{"namespace"},
		},
		Tool{Handler: handleNamespaceCoreGetNamespace},
	)

	r.register("vmware_namespace_get_supervisor_summaries",
		"Fetch the summary of every Supervisor in the inventory.",
		map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
		Tool{Handler: handleNamespaceCoreGetSupervisorSummaries},
	)

	r.register("vmware_namespace_get_supervisor_summary",
		"Fetch the summary (name, stats, config/kubernetes status) of a specific Supervisor.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"id": supervisorIDArg},
			"required":   []interface{}{"id"},
		},
		Tool{Handler: handleNamespaceCoreGetSupervisorSummary},
	)

	r.register("vmware_namespace_get_supervisor_topology",
		"Fetch the zone/cluster topology of a specific Supervisor.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"id": supervisorIDArg},
			"required":   []interface{}{"id"},
		},
		Tool{Handler: handleNamespaceCoreGetSupervisorTopology},
	)

	r.register("vmware_namespace_get_vm_class",
		"Fetch detailed information for a Virtual Machine Class.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"vm_class": vmClassIDArg},
			"required":   []interface{}{"vm_class"},
		},
		Tool{Handler: handleNamespaceCoreGetVmClass},
	)

	r.register("vmware_namespace_list_clusters",
		"List every cluster with vSphere Namespaces enabled, with a summary (kubernetes/config status) for each.",
		map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
		Tool{Handler: handleNamespaceCoreListClusters},
	)

	r.register("vmware_namespace_list_compatible_distributed_switches",
		"List the vSphere Distributed Switches compatible for enabling vSphere Namespaces on a given cluster.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"cluster_id": clusterIDArg},
			"required":   []interface{}{"cluster_id"},
		},
		Tool{Handler: handleNamespaceCoreListCompatibleDistributedSwitches},
	)

	r.register("vmware_namespace_list_compatible_edge_clusters",
		"List the NSX-T Edge Clusters compatible for enabling vSphere Namespaces on a given cluster/distributed-switch pair.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"cluster_id": clusterIDArg,
				"switch_id":  map[string]interface{}{"type": "string", "description": "Distributed Switch ID (raw wire identifier, as returned by vmware_namespace_list_compatible_distributed_switches — not an inventory path)."},
			},
			"required": []interface{}{"cluster_id", "switch_id"},
		},
		Tool{Handler: handleNamespaceCoreListCompatibleEdgeClusters},
	)

	r.register("vmware_namespace_list_namespaces",
		"List every vSphere Namespace, with a summary (cluster, config status, stats) for each.",
		map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
		Tool{Handler: handleNamespaceCoreListNamespaces},
	)

	r.register("vmware_namespace_list_vm_classes",
		"List every Virtual Machine Class, with full detail for each.",
		map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
		Tool{Handler: handleNamespaceCoreListVmClasses},
	)

	r.registerDestructive("vmware_namespace_register_vm",
		"Register an existing (unmanaged) VM into a vSphere Namespace, so it becomes visible/manageable as a namespace workload. Returns a server response string (implementation-defined; typically a task/activation identifier).",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"namespace": namespaceNameArg,
				"vm":        map[string]interface{}{"type": "string", "description": "Raw VM identifier in the vSphere Namespaces wire format (RegisterVMSpec.vm) — NOT an inventory path."},
				"confirm":   confirmArg,
			},
			"required": []interface{}{"namespace", "vm", "confirm"},
		},
		Tool{Handler: handleNamespaceCoreRegisterVM},
	)

	r.registerDestructive("vmware_namespace_update_namespace",
		"Update a vSphere Namespace's VM Service spec, storage specs, and/or network spec.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"namespace":       namespaceNameArg,
				"vm_service_spec": vmServiceSpecArg,
				"storage_specs":   storageSpecsArg,
				"network_spec": map[string]interface{}{
					"type":        "object",
					"description": `Optional. {"network_provider": "...", "vpc_config": {"default_subnet_size": N, "private_cidrs": [{"address","prefix"}, ...]}} — 9.0.0.0+.`,
				},
				"confirm": confirmArg,
			},
			"required": []interface{}{"namespace", "confirm"},
		},
		Tool{Handler: handleNamespaceCoreUpdateNamespace},
	)

	r.registerDestructive("vmware_namespace_update_vm_class",
		"Update (replace) a Virtual Machine Class's CPU/memory/device profile. Behaves as a full replace, not a partial patch — every field is re-sent; fields omitted here fall back to their zero value except id, which defaults to vm_class if omitted.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm_class":           vmClassIDArg,
				"id":                 map[string]interface{}{"type": "string", "description": "VM Class ID for the updated spec's body. Defaults to vm_class if omitted — only override this if you specifically need to test/observe a mismatch."},
				"cpu_count":          map[string]interface{}{"type": "integer", "description": "Number of virtual CPUs."},
				"memory_mb":          map[string]interface{}{"type": "integer", "description": "Memory in MB."},
				"cpu_reservation":    map[string]interface{}{"type": "integer", "description": "CPU reservation percentage (0-100). Optional."},
				"memory_reservation": map[string]interface{}{"type": "integer", "description": "Memory reservation percentage (0-100). Optional."},
				"devices": map[string]interface{}{
					"type":        "object",
					"description": `Optional passthrough devices: {"direct_path_io_devices": [...], "vgpu_devices": [...]}.`,
				},
				"config_spec": map[string]interface{}{"type": "object", "description": "Optional raw/version-specific extended VM Class config spec, passed through verbatim."},
				"confirm":     confirmArg,
			},
			"required": []interface{}{"vm_class", "cpu_count", "memory_mb", "confirm"},
		},
		Tool{Handler: handleNamespaceCoreUpdateVmClass},
	)
}

// namespaceCoreManager returns a namespace.Manager bound to client's VAPI/REST
// session, logging in lazily via client.REST — same pattern as
// generated_tags.go's tagsManager/generated_vcenter_template.go's
// vcenterTemplateManager. Named with the "Core" suffix (not just
// "namespaceManager") to avoid a same-package name collision with sibling
// Fase 8a NS-B/C/D files that also wrap vapi/namespace.Manager — see this
// file's top doc comment.
func namespaceCoreManager(ctx context.Context, client *vmware.Client) (*namespace.Manager, error) {
	rc, err := client.REST(ctx)
	if err != nil {
		return nil, err
	}
	return namespace.NewManager(rc), nil
}

// vmClassSpecFromArgs builds a namespace.VirtualMachineClassCreateSpec from
// args field-by-field (not a wholesale decodeJSONArg(args, &spec)) — see this
// file's top doc comment ("Curation decisions") for why MemoryMb's
// inconsistent `json:"memory_MB"` wire tag makes an explicit mapping the
// better choice here. fallbackID is used for spec.Id when args["id"] is
// empty/absent — vmware_namespace_create_vm_class calls this with "" (id is
// mandatory there), vmware_namespace_update_vm_class calls it with the
// vm_class path argument (id in the body may be omitted there, defaulting to
// the class being updated).
func vmClassSpecFromArgs(args map[string]interface{}, fallbackID string) (namespace.VirtualMachineClassCreateSpec, error) {
	var spec namespace.VirtualMachineClassCreateSpec

	id, _ := args["id"].(string)
	if id == "" {
		id = fallbackID
	}
	if id == "" {
		return spec, fmt.Errorf("id is required")
	}
	spec.Id = id

	cpuCountRaw, ok := args["cpu_count"]
	if !ok {
		return spec, fmt.Errorf("cpu_count is required")
	}
	cpuCount, err := toInt64(cpuCountRaw)
	if err != nil {
		return spec, fmt.Errorf("invalid cpu_count: %w", err)
	}
	spec.CpuCount = cpuCount

	memoryMbRaw, ok := args["memory_mb"]
	if !ok {
		return spec, fmt.Errorf("memory_mb is required")
	}
	memoryMb, err := toInt64(memoryMbRaw)
	if err != nil {
		return spec, fmt.Errorf("invalid memory_mb: %w", err)
	}
	spec.MemoryMb = memoryMb

	if raw, ok := args["cpu_reservation"]; ok && raw != nil {
		v, err := toInt64(raw)
		if err != nil {
			return spec, fmt.Errorf("invalid cpu_reservation: %w", err)
		}
		spec.CpuReservation = v
	}
	if raw, ok := args["memory_reservation"]; ok && raw != nil {
		v, err := toInt64(raw)
		if err != nil {
			return spec, fmt.Errorf("invalid memory_reservation: %w", err)
		}
		spec.MemoryReservation = v
	}
	if raw, ok := args["devices"]; ok && raw != nil {
		var devices namespace.VirtualDevices
		if err := decodeJSONArg(raw, &devices); err != nil {
			return spec, fmt.Errorf("invalid devices: %w", err)
		}
		spec.Devices = devices
	}
	if raw, ok := args["config_spec"]; ok && raw != nil {
		b, err := json.Marshal(raw)
		if err != nil {
			return spec, fmt.Errorf("invalid config_spec: %w", err)
		}
		spec.ConfigSpec = json.RawMessage(b)
	}

	return spec, nil
}

func handleNamespaceCoreCreateNamespace(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	var spec namespace.NamespacesInstanceCreateSpec
	if err := decodeJSONArg(args, &spec); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if spec.Cluster == "" {
		return "", fmt.Errorf("cluster is required")
	}
	if spec.Namespace == "" {
		return "", fmt.Errorf("namespace is required")
	}

	m, err := namespaceCoreManager(ctx, client)
	if err != nil {
		return "", err
	}
	if err := m.CreateNamespace(ctx, spec); err != nil {
		return "", fmt.Errorf("failed to create namespace %q on cluster %q: %w", spec.Namespace, spec.Cluster, err)
	}
	return marshalJSON(map[string]interface{}{"result": "namespace_created", "cluster": spec.Cluster, "namespace": spec.Namespace})
}

func handleNamespaceCoreCreateSupportBundle(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	id, _ := args["id"].(string)
	if id == "" {
		return "", fmt.Errorf("id is required")
	}

	m, err := namespaceCoreManager(ctx, client)
	if err != nil {
		return "", err
	}
	loc, err := m.CreateSupportBundle(ctx, id)
	if err != nil {
		return "", fmt.Errorf("failed to create support bundle for cluster %q: %w", id, err)
	}
	return marshalJSON(map[string]interface{}{
		"result":       "support_bundle_created",
		"cluster_id":   id,
		"url":          loc.URL,
		"token":        loc.Token.Token,
		"token_expiry": loc.Token.Expiry,
	})
}

func handleNamespaceCoreCreateVmClass(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	spec, err := vmClassSpecFromArgs(args, "")
	if err != nil {
		return "", err
	}

	m, err := namespaceCoreManager(ctx, client)
	if err != nil {
		return "", err
	}
	if err := m.CreateVmClass(ctx, spec); err != nil {
		return "", fmt.Errorf("failed to create VM class %q: %w", spec.Id, err)
	}
	return marshalJSON(map[string]interface{}{"result": "vm_class_created", "vm_class_id": spec.Id})
}

func handleNamespaceCoreDeleteNamespace(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	ns, _ := args["namespace"].(string)
	if ns == "" {
		return "", fmt.Errorf("namespace is required")
	}

	m, err := namespaceCoreManager(ctx, client)
	if err != nil {
		return "", err
	}
	if err := m.DeleteNamespace(ctx, ns); err != nil {
		return "", fmt.Errorf("failed to delete namespace %q: %w", ns, err)
	}
	return marshalJSON(map[string]interface{}{"result": "namespace_deleted", "namespace": ns})
}

func handleNamespaceCoreDeleteVmClass(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vmClass, _ := args["vm_class"].(string)
	if vmClass == "" {
		return "", fmt.Errorf("vm_class is required")
	}

	m, err := namespaceCoreManager(ctx, client)
	if err != nil {
		return "", err
	}
	if err := m.DeleteVmClass(ctx, vmClass); err != nil {
		return "", fmt.Errorf("failed to delete VM class %q: %w", vmClass, err)
	}
	return marshalJSON(map[string]interface{}{"result": "vm_class_deleted", "vm_class_id": vmClass})
}

func handleNamespaceCoreDisableCluster(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	id, _ := args["id"].(string)
	if id == "" {
		return "", fmt.Errorf("id is required")
	}

	m, err := namespaceCoreManager(ctx, client)
	if err != nil {
		return "", err
	}
	if err := m.DisableCluster(ctx, id); err != nil {
		return "", fmt.Errorf("failed to disable namespaces on cluster %q: %w", id, err)
	}
	return marshalJSON(map[string]interface{}{"result": "cluster_disabled", "cluster_id": id})
}

func handleNamespaceCoreEnableCluster(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	id, _ := args["id"].(string)
	if id == "" {
		return "", fmt.Errorf("id is required")
	}
	var spec namespace.EnableClusterSpec
	if err := decodeJSONArg(args, &spec); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	m, err := namespaceCoreManager(ctx, client)
	if err != nil {
		return "", err
	}
	if err := m.EnableCluster(ctx, id, &spec); err != nil {
		return "", fmt.Errorf("failed to enable namespaces on cluster %q: %w", id, err)
	}
	return marshalJSON(map[string]interface{}{"result": "cluster_enabled", "cluster_id": id})
}

func handleNamespaceCoreEnableOnComputeCluster(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	id, _ := args["id"].(string)
	if id == "" {
		return "", fmt.Errorf("id is required")
	}
	var spec namespace.EnableOnComputeClusterSpec
	if err := decodeJSONArg(args, &spec); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if spec.Name == "" {
		return "", fmt.Errorf("name is required")
	}

	m, err := namespaceCoreManager(ctx, client)
	if err != nil {
		return "", err
	}
	resp, err := m.EnableOnComputeCluster(ctx, id, &spec)
	if err != nil {
		return "", fmt.Errorf("failed to enable supervisor on cluster %q: %w", id, err)
	}
	return marshalJSON(map[string]interface{}{"result": "supervisor_enable_requested", "cluster_id": id, "response": resp})
}

func handleNamespaceCoreEnableOnZones(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	var spec namespace.EnableOnZonesSpec
	if err := decodeJSONArg(args, &spec); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if len(spec.Zones) == 0 {
		return "", fmt.Errorf("zones is required and must be non-empty")
	}
	if spec.Name == "" {
		return "", fmt.Errorf("name is required")
	}

	m, err := namespaceCoreManager(ctx, client)
	if err != nil {
		return "", err
	}
	resp, err := m.EnableOnZones(ctx, &spec)
	if err != nil {
		return "", fmt.Errorf("failed to enable supervisor on zones %v: %w", spec.Zones, err)
	}
	return marshalJSON(map[string]interface{}{"result": "supervisor_enable_requested", "zones": spec.Zones, "response": resp})
}

func handleNamespaceCoreGetNamespace(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	ns, _ := args["namespace"].(string)
	if ns == "" {
		return "", fmt.Errorf("namespace is required")
	}

	m, err := namespaceCoreManager(ctx, client)
	if err != nil {
		return "", err
	}
	info, err := m.GetNamespace(ctx, ns)
	if err != nil {
		return "", fmt.Errorf("failed to get namespace %q: %w", ns, err)
	}
	return marshalJSON(info)
}

func handleNamespaceCoreGetSupervisorSummaries(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := namespaceCoreManager(ctx, client)
	if err != nil {
		return "", err
	}

	result, err := m.GetSupervisorSummaries(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get supervisor summaries: %w", err)
	}
	return marshalJSON(map[string]interface{}{"count": len(result.Items), "items": result.Items})
}

func handleNamespaceCoreGetSupervisorSummary(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	id, _ := args["id"].(string)
	if id == "" {
		return "", fmt.Errorf("id is required")
	}

	m, err := namespaceCoreManager(ctx, client)
	if err != nil {
		return "", err
	}
	info, err := m.GetSupervisorSummary(ctx, id)
	if err != nil {
		return "", fmt.Errorf("failed to get supervisor summary for %q: %w", id, err)
	}
	return marshalJSON(info)
}

func handleNamespaceCoreGetSupervisorTopology(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	id, _ := args["id"].(string)
	if id == "" {
		return "", fmt.Errorf("id is required")
	}

	m, err := namespaceCoreManager(ctx, client)
	if err != nil {
		return "", err
	}
	list, err := m.GetSupervisorTopology(ctx, id)
	if err != nil {
		return "", fmt.Errorf("failed to get supervisor topology for %q: %w", id, err)
	}
	return marshalJSON(map[string]interface{}{"supervisor_id": id, "count": len(list), "topology": list})
}

func handleNamespaceCoreGetVmClass(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vmClass, _ := args["vm_class"].(string)
	if vmClass == "" {
		return "", fmt.Errorf("vm_class is required")
	}

	m, err := namespaceCoreManager(ctx, client)
	if err != nil {
		return "", err
	}
	info, err := m.GetVmClass(ctx, vmClass)
	if err != nil {
		return "", fmt.Errorf("failed to get VM class %q: %w", vmClass, err)
	}
	return marshalJSON(info)
}

func handleNamespaceCoreListClusters(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := namespaceCoreManager(ctx, client)
	if err != nil {
		return "", err
	}

	list, err := m.ListClusters(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to list namespace-enabled clusters: %w", err)
	}
	return marshalJSON(map[string]interface{}{"count": len(list), "clusters": list})
}

func handleNamespaceCoreListCompatibleDistributedSwitches(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	clusterID, _ := args["cluster_id"].(string)
	if clusterID == "" {
		return "", fmt.Errorf("cluster_id is required")
	}

	m, err := namespaceCoreManager(ctx, client)
	if err != nil {
		return "", err
	}
	list, err := m.ListCompatibleDistributedSwitches(ctx, clusterID)
	if err != nil {
		return "", fmt.Errorf("failed to list compatible distributed switches for cluster %q: %w", clusterID, err)
	}
	return marshalJSON(map[string]interface{}{"cluster_id": clusterID, "count": len(list), "distributed_switches": list})
}

func handleNamespaceCoreListCompatibleEdgeClusters(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	clusterID, _ := args["cluster_id"].(string)
	if clusterID == "" {
		return "", fmt.Errorf("cluster_id is required")
	}
	switchID, _ := args["switch_id"].(string)
	if switchID == "" {
		return "", fmt.Errorf("switch_id is required")
	}

	m, err := namespaceCoreManager(ctx, client)
	if err != nil {
		return "", err
	}
	list, err := m.ListCompatibleEdgeClusters(ctx, clusterID, switchID)
	if err != nil {
		return "", fmt.Errorf("failed to list compatible edge clusters for cluster %q / switch %q: %w", clusterID, switchID, err)
	}
	return marshalJSON(map[string]interface{}{"cluster_id": clusterID, "distributed_switch_id": switchID, "count": len(list), "edge_clusters": list})
}

func handleNamespaceCoreListNamespaces(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := namespaceCoreManager(ctx, client)
	if err != nil {
		return "", err
	}

	list, err := m.ListNamespaces(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to list namespaces: %w", err)
	}
	return marshalJSON(map[string]interface{}{"count": len(list), "namespaces": list})
}

func handleNamespaceCoreListVmClasses(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := namespaceCoreManager(ctx, client)
	if err != nil {
		return "", err
	}

	list, err := m.ListVmClasses(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to list VM classes: %w", err)
	}
	return marshalJSON(map[string]interface{}{"count": len(list), "vm_classes": list})
}

func handleNamespaceCoreRegisterVM(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	ns, _ := args["namespace"].(string)
	if ns == "" {
		return "", fmt.Errorf("namespace is required")
	}
	var spec namespace.RegisterVMSpec
	if err := decodeJSONArg(args, &spec); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if spec.VM == "" {
		return "", fmt.Errorf("vm is required")
	}

	m, err := namespaceCoreManager(ctx, client)
	if err != nil {
		return "", err
	}
	resp, err := m.RegisterVM(ctx, ns, spec)
	if err != nil {
		return "", fmt.Errorf("failed to register VM %q into namespace %q: %w", spec.VM, ns, err)
	}
	return marshalJSON(map[string]interface{}{"result": "vm_register_requested", "namespace": ns, "vm": spec.VM, "response": resp})
}

func handleNamespaceCoreUpdateNamespace(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	ns, _ := args["namespace"].(string)
	if ns == "" {
		return "", fmt.Errorf("namespace is required")
	}
	var spec namespace.NamespacesInstanceUpdateSpec
	if err := decodeJSONArg(args, &spec); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	m, err := namespaceCoreManager(ctx, client)
	if err != nil {
		return "", err
	}
	if err := m.UpdateNamespace(ctx, ns, spec); err != nil {
		return "", fmt.Errorf("failed to update namespace %q: %w", ns, err)
	}
	return marshalJSON(map[string]interface{}{"result": "namespace_updated", "namespace": ns})
}

func handleNamespaceCoreUpdateVmClass(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vmClass, _ := args["vm_class"].(string)
	if vmClass == "" {
		return "", fmt.Errorf("vm_class is required")
	}
	spec, err := vmClassSpecFromArgs(args, vmClass)
	if err != nil {
		return "", err
	}

	m, err := namespaceCoreManager(ctx, client)
	if err != nil {
		return "", err
	}
	if err := m.UpdateVmClass(ctx, vmClass, namespace.VirtualMachineClassUpdateSpec(spec)); err != nil {
		return "", fmt.Errorf("failed to update VM class %q: %w", vmClass, err)
	}
	return marshalJSON(map[string]interface{}{"result": "vm_class_updated", "vm_class_id": spec.Id})
}

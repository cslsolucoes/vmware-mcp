// Package tools — generated_namespace_services.go is Fase 8a (Wave 2, Grupo
// NS-B "Namespace/Supervisor") of the codegen plan
// (".workspace/plans/MCPVMWare2026-08-10-175300-plano-cobertura-completa-api-codegen.plan.md"),
// covering referencia/govmomi/vapi/namespace/{supervisorsvc,networks,namespace_v2}.go
// — 12 + 6 + 3 = 21 tools, one registration function
// (registerNamespaceServicesTools).
//
// Same REST/JSON architecture note as generated_tags.go/generated_vcenter_template.go:
// namespace.Manager wraps a *rest.Client, and every struct in these three
// source files (SupervisorService*, NetworkCreateSpec/UpdateSpec/SetSpec,
// NamespaceInstance*V2, ...) already carries real `json:"..."` tags — tool
// arguments are decoded straight into (or built field-by-field into) those
// concrete structs via decodeJSONArg (generated_vm_lifecycle.go), no generic
// object/*.go-style decode helper needed.
//
// mode=vcenter-only — the whole vapi/* domain requires a vCenter Server
// Appliance (VAMI/VAPI session), same as every other vapi/*.go file in this
// project.
//
// vcsim coverage — CONFIRMED GAP, not a bug, not a hypothesis:
//   - `grep -c "vapi/namespace" referencia/govmomi/vapi/simulator/simulator.go`
//     returns 0 — that file (the one actually wired into vcsim's HTTP mux,
//     and the one this project's newSimClient/testhelpers_test.go blank-imports
//     for RegisterEndpoints=true) never imports vapi/namespace at all, so
//     none of ClusterNetworkProvider/SupervisorService/NamespaceInstanceV2
//     have any server-side handler.
//   - A second, easy-to-miss detail also checked directly (not assumed):
//     referencia/govmomi/vapi/namespace/simulator/simulator.go DOES exist
//     (492 lines) as a *separate, standalone* subpackage, but it is not
//     imported by vapi/simulator/simulator.go, by anything else under
//     referencia/govmomi/simulator/, or by this project's testhelpers_test.go
//     — so it never actually runs against the in-process vcsim server this
//     project's tests spin up. Confirmed via
//     `grep -rn "vapi/namespace" referencia/govmomi/vapi/simulator/ referencia/govmomi/simulator/`
//     (0 matches). Every tool below is therefore tested only up to "reaches
//     the server and gets a real (non-panic, non-"unknown tool") response
//     back" via assertReachesServer (generated_vm_lifecycle_test.go), exactly
//     like generated_network.go's ReconfigureDVPort/UpdateDVSLacpGroupConfig
//     or generated_authorization.go's DisableMethods/EnableMethods before it.
//
// Entity resolution (curation decisions made in this file, since none of
// this fase's brief specified them beyond tool names/tiers/counts):
//
//   - "cluster" (networks.go's ListClusterNetworks/CreateClusterNetwork/
//     GetClusterNetwork/UpdateClusterNetwork/SetClusterNetwork/
//     DeleteClusterNetwork — literally the {cluster} path segment of
//     /vcenter/namespace-management/clusters/{cluster}/networks...) is
//     exposed here as a cluster name/inventory path, resolved via
//     resolveClusterComputeResource (generated_inventory_compute.go, same
//     package — reused as-is, not duplicated, per this project's "reuse
//     SSOT" discipline) into the ClusterComputeResource's raw moref value,
//     which is exactly what the wire format's {cluster} path segment
//     expects (a WCP-enabled ClusterComputeResource's ID, e.g. "domain-c8").
//
//   - "supervisor" (namespace_v2.go's CreateNamespaceV2 top-level Supervisor
//     field) is treated identically to "cluster" above — resolved via the
//     same resolveClusterComputeResource helper (called with a synthetic
//     {"cluster": <value>} args map, so the shared helper's hardcoded
//     "cluster" key still applies without editing that file). Reasoning: in
//     every vSphere Namespaces topology documented for 7.0-8.x, a Supervisor
//     is 1:1 with the ClusterComputeResource it was enabled on, and its
//     "supervisor" identifier on the wire IS that cluster's moref value —
//     so this keeps the same "always accept a real inventory path, never an
//     opaque ID" ergonomic promise this project makes everywhere else (see
//     generated_vcenter_template.go's "Entity resolution design" note) and
//     reuses the one helper that already does this instead of inventing a
//     second one. Flagged explicitly as a judgment call, not a spec-verified
//     fact — vSphere 9's VPC-Supervisor topology may eventually decouple this
//     1:1 relationship, and there is zero vcsim coverage in this domain to
//     verify either way; if a future vSphere version's Supervisor ID stops
//     being the cluster moref, this is the one spot to revisit.
//
//   - "portgroup" (networks.go's VsphereNetworkCreateSpec/
//     VsphereDVPGNetworkSetSpec.Portgroup, nested under spec.vsphere_network
//     in CreateClusterNetwork/UpdateClusterNetwork/SetClusterNetwork) is a
//     genuine DistributedVirtualPortgroup — resolved via resolveEntityRef
//     (generated_authorization.go, generic any-entity resolution, reused as-is)
//     into its raw moref value. This is different from the network's own
//     top-level "network"/network_id identifier (see next bullet) and from
//     every NSX-T field (see the bullet after): only the vSphere-DVPG path
//     addresses a real vCenter inventory object.
//
//   - The Namespaces network object's own identifier — NetworkCreateSpec.Network
//     ("network" in the create spec, a caller-chosen DNS_LABEL string) and
//     the {network} path segment used by Get/Update/Set/Delete/List (exposed
//     here as "network_id") — is NOT a vSphere inventory object at all (it is
//     an opaque, namespace-management-scoped label the vSphere Namespaces
//     API itself assigns meaning to), so it is passed through verbatim as a
//     plain string, never resolved.
//
//   - Every NSX-T field (NsxNetworkCreateSpec/UpdateSpec/SetSpec's
//     NsxTier0Gateway, *Cidrs, LoadBalancerSize, ...) and every CIDR-only
//     field (namespace_v2.go's NamespaceNetworkCreateSpec/NetworkConfigCreateSpec/
//     VpcNetworkCreateSpec) references NSX Manager-side or pure-address-block
//     concepts with no vSphere inventory-path equivalent — decoded verbatim
//     via decodeJSONArg, same "no inventory-path equivalent, so passed
//     through raw" treatment as generated_vcenter_template.go's
//     DeploymentSpec.network_mappings/storage_mappings.
//
//   - Every SupervisorService*/SupervisorServiceVersion* field (base64
//     "content", "trusted_provider", "accept_EULA") is raw package/EULA data
//     with no inventory-path equivalent either — decoded verbatim.
//
// namespace_v2 vs namespace (v1, this fase's sibling Grupo NS-A file,
// namespace.go — NOT read or depended on here per the orchestrator's brief):
// CreateNamespaceV2/GetNamespaceV2/ListNamespacesV2 are a newer, parallel API
// version of NS-A's CreateNamespace/GetNamespace/ListNamespaces (different
// REST paths: /api/vcenter/namespaces/instances/v2 vs
// /api/vcenter/namespaces/instances — see referencia/govmomi/vapi/namespace/internal/internal.go's
// NamespacesPathV2 vs NamespacesPath constants). Both are kept as genuinely
// distinct tools/endpoints here, per explicit instruction — this is NOT the
// Fase 7 Task.Wait*-style "same operation reachable two ways, register once"
// pattern, and no v1/v2 fallback logic is implemented in either direction.
//
// Tiers (as given, not re-derived): CreateSupervisorService/CreateSupervisorServiceVersion/
// ActivateSupervisorServiceVersion/ActivateSupervisorServices/
// DeactivateSupervisorServiceVersion/DeactivateSupervisorServices/
// CreateClusterNetwork/UpdateClusterNetwork/SetClusterNetwork/CreateNamespaceV2
// = tier2; RemoveSupervisorService/RemoveSupervisorServiceVersion/
// DeleteClusterNetwork = tier1; every Get*/List* = read-only, no tier.
package tools

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/vapi/namespace"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

func registerNamespaceServicesTools(r *Registry) {
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}
	clusterArg := map[string]interface{}{
		"type":        "string",
		"description": `Cluster name or full inventory path of the WCP/Supervisor-enabled ClusterComputeResource (e.g. "Cluster1" or "/DC0/host/Cluster1") — resolved the same way as this project's other cluster-scoped tools (e.g. vmware_cluster_* tools), not a raw vCenter moref ID.`,
	}
	supervisorServiceIDArg := map[string]interface{}{
		"type":        "string",
		"description": "Supervisor Service ID, as returned by vmware_namespace_list_supervisor_services or embedded in the content used to create the service.",
	}
	versionArg := map[string]interface{}{
		"type":        "string",
		"description": "Supervisor Service version string, as returned by vmware_namespace_list_supervisor_service_versions.",
	}
	networkIDArg := map[string]interface{}{
		"type":        "string",
		"description": `Namespaces network object identifier — an opaque, namespace-management-scoped label (NOT a vSphere Network/Portgroup inventory object; see this file's top doc comment), as returned by vmware_namespace_create_cluster_network or vmware_namespace_list_cluster_networks.`,
	}
	vsphereVersionCreateSpecSchema := map[string]interface{}{
		"type":        "object",
		"description": "vSphere-format Supervisor Service version content.",
		"properties": map[string]interface{}{
			"content":          map[string]interface{}{"type": "string", "description": "Base64-encoded service definition content."},
			"trusted_provider": map[string]interface{}{"type": "boolean"},
			"accept_EULA":      map[string]interface{}{"type": "boolean"},
		},
		"required": []interface{}{"content"},
	}
	carvelVersionCreateSpecSchema := map[string]interface{}{
		"type":        "object",
		"description": "Carvel-format (Package/PackageMetadata) Supervisor Service version content.",
		"properties": map[string]interface{}{
			"content": map[string]interface{}{"type": "string", "description": "Base64-encoded Carvel Package/PackageMetadata content."},
		},
		"required": []interface{}{"content"},
	}
	createServiceSchema := map[string]interface{}{
		"type":        "object",
		"description": "New Supervisor Service specification — give exactly one of vsphere_spec/carvel_spec.",
		"properties": map[string]interface{}{
			"vsphere_spec": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"version_spec": vsphereVersionCreateSpecSchema,
				},
				"required": []interface{}{"version_spec"},
			},
			"carvel_spec": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"version_spec": carvelVersionCreateSpecSchema,
				},
				"required": []interface{}{"version_spec"},
			},
		},
	}
	createServiceVersionSchema := map[string]interface{}{
		"type":        "object",
		"description": "New Supervisor Service version specification — give exactly one of vsphere_spec/carvel_spec. NOTE: unlike the service-level spec above, these are the version content objects directly (no extra version_spec wrapper).",
		"properties": map[string]interface{}{
			"vsphere_spec": vsphereVersionCreateSpecSchema,
			"carvel_spec":  carvelVersionCreateSpecSchema,
		},
	}
	nsxNetworkSchema := map[string]interface{}{
		"type":        "object",
		"description": "NSX-T-backed network fields — passed through verbatim (NSX Manager-side concepts, no vSphere inventory-path equivalent).",
		"properties": map[string]interface{}{
			"egress_cidrs":            map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "object", "properties": map[string]interface{}{"address": map[string]interface{}{"type": "string"}, "prefix": map[string]interface{}{"type": "integer"}}}},
			"ingress_cidrs":           map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "object", "properties": map[string]interface{}{"address": map[string]interface{}{"type": "string"}, "prefix": map[string]interface{}{"type": "integer"}}}},
			"namespace_network_cidrs": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "object", "properties": map[string]interface{}{"address": map[string]interface{}{"type": "string"}, "prefix": map[string]interface{}{"type": "integer"}}}},
			"load_balancer_size":      map[string]interface{}{"type": "string", "description": `"SMALL", "MEDIUM", or "LARGE".`},
			"nsx_tier0_gateway":       map[string]interface{}{"type": "string"},
			"routed_mode":             map[string]interface{}{"type": "boolean"},
			"subnet_prefix_length":    map[string]interface{}{"type": "integer"},
		},
	}
	vsphereNetworkCreateSchema := map[string]interface{}{
		"type":        "object",
		"description": "DVPG-backed network fields.",
		"properties": map[string]interface{}{
			"address_ranges":     map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "object", "properties": map[string]interface{}{"address": map[string]interface{}{"type": "string"}, "count": map[string]interface{}{"type": "integer"}}}},
			"gateway":            map[string]interface{}{"type": "string"},
			"ip_assignment_mode": map[string]interface{}{"type": "string", "description": `"DHCP" or "STATICRANGE".`},
			"portgroup":          map[string]interface{}{"type": "string", "description": "Inventory path of the DistributedVirtualPortgroup backing this network — resolved the same way as this project's other inventory-path arguments (unlike every other field in this object)."},
			"subnet_mask":        map[string]interface{}{"type": "string"},
		},
	}
	vsphereNetworkSetSchema := map[string]interface{}{
		"type":        "object",
		"description": "DVPG-backed network fields (update/replace shape — no ip_assignment_mode field here, unlike the create shape).",
		"properties": map[string]interface{}{
			"address_ranges": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "object", "properties": map[string]interface{}{"address": map[string]interface{}{"type": "string"}, "count": map[string]interface{}{"type": "integer"}}}},
			"gateway":        map[string]interface{}{"type": "string"},
			"portgroup":      map[string]interface{}{"type": "string", "description": "Inventory path of the DistributedVirtualPortgroup backing this network — resolved the same way as this project's other inventory-path arguments (unlike every other field in this object)."},
			"subnet_mask":    map[string]interface{}{"type": "string"},
		},
	}
	createNetworkSpecSchema := map[string]interface{}{
		"type":        "object",
		"description": "Namespaces network create spec. Give exactly one of nsx_network/vsphere_network, matching network_provider.",
		"properties": map[string]interface{}{
			"network":          map[string]interface{}{"type": "string", "description": "Desired identifier for the new network object (DNS_LABEL: max 63 chars, alphanumeric + '-'). Optional — server-assigned if omitted."},
			"network_provider": map[string]interface{}{"type": "string", "description": `"NSXT_CONTAINER_PLUGIN" or "VSPHERE_NETWORK".`},
			"nsx_network":      nsxNetworkSchema,
			"vsphere_network":  vsphereNetworkCreateSchema,
		},
		"required": []interface{}{"network_provider"},
	}
	nsxNetworkUpdateSchema := map[string]interface{}{
		"type":        "object",
		"description": "NSX-T-backed network partial-update fields — passed through verbatim.",
		"properties": map[string]interface{}{
			"egress_cidrs":            map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "object", "properties": map[string]interface{}{"address": map[string]interface{}{"type": "string"}, "prefix": map[string]interface{}{"type": "integer"}}}},
			"ingress_cidrs":           map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "object", "properties": map[string]interface{}{"address": map[string]interface{}{"type": "string"}, "prefix": map[string]interface{}{"type": "integer"}}}},
			"namespace_network_cidrs": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "object", "properties": map[string]interface{}{"address": map[string]interface{}{"type": "string"}, "prefix": map[string]interface{}{"type": "integer"}}}},
		},
	}
	updateOrSetNetworkSpecSchema := func(desc string) map[string]interface{} {
		return map[string]interface{}{
			"type":        "object",
			"description": desc,
			"properties": map[string]interface{}{
				"network_provider": map[string]interface{}{"type": "string", "description": `"NSXT_CONTAINER_PLUGIN" or "VSPHERE_NETWORK".`},
				"nsx_network":      nsxNetworkUpdateSchema,
				"vsphere_network":  vsphereNetworkSetSchema,
			},
			"required": []interface{}{"network_provider"},
		}
	}

	// -------- supervisorsvc.go (12) --------

	r.register("vmware_namespace_get_supervisor_service",
		"Get the information of a specific Supervisor Service.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"supervisor_service_id": supervisorServiceIDArg},
			"required":   []interface{}{"supervisor_service_id"},
		},
		Tool{Handler: handleNamespaceGetSupervisorService},
	)

	r.register("vmware_namespace_get_supervisor_service_version",
		"Get the information of a specific Supervisor Service version.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"supervisor_service_id": supervisorServiceIDArg,
				"version":               versionArg,
			},
			"required": []interface{}{"supervisor_service_id", "version"},
		},
		Tool{Handler: handleNamespaceGetSupervisorServiceVersion},
	)

	r.register("vmware_namespace_list_supervisor_service_versions",
		"List all versions of the given Supervisor Service.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"supervisor_service_id": supervisorServiceIDArg},
			"required":   []interface{}{"supervisor_service_id"},
		},
		Tool{Handler: handleNamespaceListSupervisorServiceVersions},
	)

	r.register("vmware_namespace_list_supervisor_services",
		"List a summary of every registered Supervisor Service.",
		map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
		Tool{Handler: handleNamespaceListSupervisorServices},
	)

	r.registerDestructive("vmware_namespace_create_supervisor_service",
		"Create a new Supervisor Service on vCenter, from inline vSphere-format or Carvel-format content.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"service": createServiceSchema,
				"confirm": confirmArg,
			},
			"required": []interface{}{"service", "confirm"},
		},
		Tool{Handler: handleNamespaceCreateSupervisorService},
	)

	r.registerDestructive("vmware_namespace_create_supervisor_service_version",
		"Create a new version for an existing Supervisor Service.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"supervisor_service_id": supervisorServiceIDArg,
				"service":               createServiceVersionSchema,
				"confirm":               confirmArg,
			},
			"required": []interface{}{"supervisor_service_id", "service", "confirm"},
		},
		Tool{Handler: handleNamespaceCreateSupervisorServiceVersion},
	)

	r.registerDestructive("vmware_namespace_activate_supervisor_services",
		"Activate a previously registered Supervisor Service.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"supervisor_service_id": supervisorServiceIDArg,
				"confirm":               confirmArg,
			},
			"required": []interface{}{"supervisor_service_id", "confirm"},
		},
		Tool{Handler: handleNamespaceActivateSupervisorServices},
	)

	r.registerDestructive("vmware_namespace_deactivate_supervisor_services",
		"Deactivate a previously registered Supervisor Service.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"supervisor_service_id": supervisorServiceIDArg,
				"confirm":               confirmArg,
			},
			"required": []interface{}{"supervisor_service_id", "confirm"},
		},
		Tool{Handler: handleNamespaceDeactivateSupervisorServices},
	)

	r.registerDestructive("vmware_namespace_activate_supervisor_service_version",
		"Activate a specific version of an existing Supervisor Service.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"supervisor_service_id": supervisorServiceIDArg,
				"version":               versionArg,
				"confirm":               confirmArg,
			},
			"required": []interface{}{"supervisor_service_id", "version", "confirm"},
		},
		Tool{Handler: handleNamespaceActivateSupervisorServiceVersion},
	)

	r.registerDestructive("vmware_namespace_deactivate_supervisor_service_version",
		"Deactivate a specific version of an existing Supervisor Service.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"supervisor_service_id": supervisorServiceIDArg,
				"version":               versionArg,
				"confirm":               confirmArg,
			},
			"required": []interface{}{"supervisor_service_id", "version", "confirm"},
		},
		Tool{Handler: handleNamespaceDeactivateSupervisorServiceVersion},
	)

	r.registerDestructive("vmware_namespace_remove_supervisor_service",
		"Remove a previously deactivated Supervisor Service. Irreversible.",
		tier1,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"supervisor_service_id": supervisorServiceIDArg,
				"confirm":               confirmArg,
			},
			"required": []interface{}{"supervisor_service_id", "confirm"},
		},
		Tool{Handler: handleNamespaceRemoveSupervisorService},
	)

	r.registerDestructive("vmware_namespace_remove_supervisor_service_version",
		"Remove a previously deactivated Supervisor Service version. Irreversible.",
		tier1,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"supervisor_service_id": supervisorServiceIDArg,
				"version":               versionArg,
				"confirm":               confirmArg,
			},
			"required": []interface{}{"supervisor_service_id", "version", "confirm"},
		},
		Tool{Handler: handleNamespaceRemoveSupervisorServiceVersion},
	)

	// -------- networks.go (6) --------

	r.register("vmware_namespace_list_cluster_networks",
		"List all vSphere Namespaces networks for the given Supervisor-enabled cluster.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"cluster": clusterArg},
			"required":   []interface{}{"cluster"},
		},
		Tool{Handler: handleNamespaceListClusterNetworks},
	)

	r.register("vmware_namespace_get_cluster_network",
		"Get detailed information about a specific vSphere Namespaces network.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"cluster":    clusterArg,
				"network_id": networkIDArg,
			},
			"required": []interface{}{"cluster", "network_id"},
		},
		Tool{Handler: handleNamespaceGetClusterNetwork},
	)

	r.registerDestructive("vmware_namespace_create_cluster_network",
		"Create a new vSphere Namespaces network object for the given Supervisor-enabled cluster. NOTE (per govmomi's own doc comment on CreateClusterNetwork): NSX-backed create specs are not supported via this endpoint (server returns an unsupported error) — use VSPHERE_NETWORK or a pre-provisioned NSX network instead.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"cluster": clusterArg,
				"spec":    createNetworkSpecSchema,
				"confirm": confirmArg,
			},
			"required": []interface{}{"cluster", "spec", "confirm"},
		},
		Tool{Handler: handleNamespaceCreateClusterNetwork},
	)

	r.registerDestructive("vmware_namespace_update_cluster_network",
		"Partially update an existing vSphere Namespaces network (PATCH semantics — only fields present in spec are applied).",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"cluster":    clusterArg,
				"network_id": networkIDArg,
				"spec":       updateOrSetNetworkSpecSchema("Partial-update fields — omitted fields are left unchanged."),
				"confirm":    confirmArg,
			},
			"required": []interface{}{"cluster", "network_id", "spec", "confirm"},
		},
		Tool{Handler: handleNamespaceUpdateClusterNetwork},
	)

	r.registerDestructive("vmware_namespace_set_cluster_network",
		"Fully replace the configuration of an existing vSphere Namespaces network (PUT semantics — omitted mutable fields are reset to defaults).",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"cluster":    clusterArg,
				"network_id": networkIDArg,
				"spec":       updateOrSetNetworkSpecSchema("Full-replacement fields — omitted mutable fields are reset to defaults."),
				"confirm":    confirmArg,
			},
			"required": []interface{}{"cluster", "network_id", "spec", "confirm"},
		},
		Tool{Handler: handleNamespaceSetClusterNetwork},
	)

	r.registerDestructive("vmware_namespace_delete_cluster_network",
		"Remove a vSphere Namespaces network object from the given cluster. The network must not be referenced by any namespace. Irreversible.",
		tier1,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"cluster":    clusterArg,
				"network_id": networkIDArg,
				"confirm":    confirmArg,
			},
			"required": []interface{}{"cluster", "network_id", "confirm"},
		},
		Tool{Handler: handleNamespaceDeleteClusterNetwork},
	)

	// -------- namespace_v2.go (3) --------

	r.register("vmware_namespace_list_namespaces_v2",
		"List a summary of every vSphere Namespace instance (v2 API — see this file's top doc comment for the v1/v2 relationship).",
		map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
		Tool{Handler: handleNamespaceListNamespacesV2},
	)

	r.register("vmware_namespace_get_namespace_v2",
		"Get detailed information about a specific vSphere Namespace instance (v2 API).",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"namespace": map[string]interface{}{"type": "string", "description": "Name of the vSphere Namespace instance."},
			},
			"required": []interface{}{"namespace"},
		},
		Tool{Handler: handleNamespaceGetNamespaceV2},
	)

	r.registerDestructive("vmware_namespace_create_namespace_v2",
		"Create a new vSphere Namespace instance on a Supervisor (v2 API — see this file's top doc comment for the v1/v2 relationship and how 'supervisor' is resolved).",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"namespace":   map[string]interface{}{"type": "string", "description": "Name of the new vSphere Namespace instance."},
				"supervisor":  clusterArg,
				"description": map[string]interface{}{"type": "string"},
				"storage_specs": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"policy": map[string]interface{}{"type": "string", "description": "Storage policy ID."},
							"limit":  map[string]interface{}{"type": "integer", "description": "Storage limit in MB."},
						},
						"required": []interface{}{"policy"},
					},
				},
				"vm_service_spec": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"content_libraries": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
						"vm_classes":        map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
					},
				},
				"content_libraries": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"content_library":          map[string]interface{}{"type": "string"},
							"writable":                 map[string]interface{}{"type": "boolean"},
							"allow_import":             map[string]interface{}{"type": "boolean"},
							"resource_naming_strategy": map[string]interface{}{"type": "string"},
						},
						"required": []interface{}{"content_library"},
					},
				},
				"self_service_namespace": map[string]interface{}{"type": "boolean"},
				"network_spec": map[string]interface{}{
					"type":        "object",
					"description": "NetworkConfigCreateSpec — passed through verbatim (network_provider, vpc_network{vpc_config,vpc,default_subnet_size} — no inventory-addressable sub-fields).",
				},
				"namespace_network": map[string]interface{}{
					"type":        "object",
					"description": "NamespaceNetworkCreateSpec — passed through verbatim (network_provider, network{namespace_network_cidrs,ingress_cidrs,egress_cidrs,nsx_tier0_gateway,subnet_prefix_length,routed_mode,load_balancer_size} — CIDR-based, no inventory-addressable sub-fields).",
				},
				"confirm": confirmArg,
			},
			"required": []interface{}{"namespace", "supervisor", "confirm"},
		},
		Tool{Handler: handleNamespaceCreateNamespaceV2},
	)
}

// namespaceServicesManager returns a namespace.Manager bound to client's
// VAPI/REST session, logging in lazily via client.REST — same pattern as
// generated_tags.go's tagsManager / generated_vcenter_template.go's
// vcenterTemplateManager.
func namespaceServicesManager(ctx context.Context, client *vmware.Client) (*namespace.Manager, error) {
	rc, err := client.REST(ctx)
	if err != nil {
		return nil, err
	}
	return namespace.NewManager(rc), nil
}

// resolveClusterNetworkPortgroup extracts specRaw's nested
// vsphere_network.portgroup (an inventory path, per this file's top doc
// comment) and resolves it to the underlying DistributedVirtualPortgroup's
// raw moref value via resolveEntityRef (generated_authorization.go). Returns
// "" if specRaw has no such field — the caller only overwrites the decoded
// spec's Portgroup field when this is non-empty, leaving every other decode
// path (NSX-backed specs, or a vsphere_network with no portgroup) untouched.
func resolveClusterNetworkPortgroup(ctx context.Context, client *vmware.Client, specRaw interface{}) (string, error) {
	m, ok := specRaw.(map[string]interface{})
	if !ok {
		return "", nil
	}
	vn, ok := m["vsphere_network"].(map[string]interface{})
	if !ok {
		return "", nil
	}
	pg, _ := vn["portgroup"].(string)
	if pg == "" {
		return "", nil
	}
	ref, err := resolveEntityRef(ctx, client, pg)
	if err != nil {
		return "", fmt.Errorf("spec.vsphere_network.portgroup: %w", err)
	}
	return ref.Value, nil
}

// -------- supervisorsvc.go handlers --------

func handleNamespaceGetSupervisorService(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := namespaceServicesManager(ctx, client)
	if err != nil {
		return "", err
	}
	id, _ := args["supervisor_service_id"].(string)
	if id == "" {
		return "", fmt.Errorf("supervisor_service_id is required")
	}
	info, err := m.GetSupervisorService(ctx, id)
	if err != nil {
		return "", fmt.Errorf("failed to get supervisor service %q: %w", id, err)
	}
	return marshalJSON(info)
}

func handleNamespaceGetSupervisorServiceVersion(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := namespaceServicesManager(ctx, client)
	if err != nil {
		return "", err
	}
	id, _ := args["supervisor_service_id"].(string)
	if id == "" {
		return "", fmt.Errorf("supervisor_service_id is required")
	}
	version, _ := args["version"].(string)
	if version == "" {
		return "", fmt.Errorf("version is required")
	}
	info, err := m.GetSupervisorServiceVersion(ctx, id, version)
	if err != nil {
		return "", fmt.Errorf("failed to get supervisor service %q version %q: %w", id, version, err)
	}
	return marshalJSON(info)
}

func handleNamespaceListSupervisorServiceVersions(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := namespaceServicesManager(ctx, client)
	if err != nil {
		return "", err
	}
	id, _ := args["supervisor_service_id"].(string)
	if id == "" {
		return "", fmt.Errorf("supervisor_service_id is required")
	}
	list, err := m.ListSupervisorServiceVersions(ctx, id)
	if err != nil {
		return "", fmt.Errorf("failed to list versions for supervisor service %q: %w", id, err)
	}
	return marshalJSON(map[string]interface{}{"supervisor_service_id": id, "count": len(list), "versions": list})
}

func handleNamespaceListSupervisorServices(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := namespaceServicesManager(ctx, client)
	if err != nil {
		return "", err
	}
	list, err := m.ListSupervisorServices(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to list supervisor services: %w", err)
	}
	return marshalJSON(map[string]interface{}{"count": len(list), "supervisor_services": list})
}

func handleNamespaceCreateSupervisorService(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := namespaceServicesManager(ctx, client)
	if err != nil {
		return "", err
	}
	raw, ok := args["service"]
	if !ok || raw == nil {
		return "", fmt.Errorf("service is required")
	}
	var spec namespace.SupervisorService
	if err := decodeJSONArg(raw, &spec); err != nil {
		return "", fmt.Errorf("service: %w", err)
	}
	if spec.VsphereService == nil && spec.CarvelService == nil {
		return "", fmt.Errorf("service must set exactly one of vsphere_spec/carvel_spec")
	}
	if err := m.CreateSupervisorService(ctx, &spec); err != nil {
		return "", fmt.Errorf("failed to create supervisor service: %w", err)
	}
	return marshalJSON(map[string]interface{}{"result": "supervisor_service_created"})
}

func handleNamespaceCreateSupervisorServiceVersion(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := namespaceServicesManager(ctx, client)
	if err != nil {
		return "", err
	}
	id, _ := args["supervisor_service_id"].(string)
	if id == "" {
		return "", fmt.Errorf("supervisor_service_id is required")
	}
	raw, ok := args["service"]
	if !ok || raw == nil {
		return "", fmt.Errorf("service is required")
	}
	var spec namespace.SupervisorServiceVersion
	if err := decodeJSONArg(raw, &spec); err != nil {
		return "", fmt.Errorf("service: %w", err)
	}
	if spec.VsphereService == nil && spec.CarvelService == nil {
		return "", fmt.Errorf("service must set exactly one of vsphere_spec/carvel_spec")
	}
	if err := m.CreateSupervisorServiceVersion(ctx, id, &spec); err != nil {
		return "", fmt.Errorf("failed to create version for supervisor service %q: %w", id, err)
	}
	return marshalJSON(map[string]interface{}{"result": "supervisor_service_version_created", "supervisor_service_id": id})
}

func handleNamespaceActivateSupervisorServices(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := namespaceServicesManager(ctx, client)
	if err != nil {
		return "", err
	}
	id, _ := args["supervisor_service_id"].(string)
	if id == "" {
		return "", fmt.Errorf("supervisor_service_id is required")
	}
	if err := m.ActivateSupervisorServices(ctx, id); err != nil {
		return "", fmt.Errorf("failed to activate supervisor service %q: %w", id, err)
	}
	return marshalJSON(map[string]interface{}{"result": "supervisor_service_activated", "supervisor_service_id": id})
}

func handleNamespaceDeactivateSupervisorServices(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := namespaceServicesManager(ctx, client)
	if err != nil {
		return "", err
	}
	id, _ := args["supervisor_service_id"].(string)
	if id == "" {
		return "", fmt.Errorf("supervisor_service_id is required")
	}
	if err := m.DeactivateSupervisorServices(ctx, id); err != nil {
		return "", fmt.Errorf("failed to deactivate supervisor service %q: %w", id, err)
	}
	return marshalJSON(map[string]interface{}{"result": "supervisor_service_deactivated", "supervisor_service_id": id})
}

func handleNamespaceActivateSupervisorServiceVersion(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := namespaceServicesManager(ctx, client)
	if err != nil {
		return "", err
	}
	id, _ := args["supervisor_service_id"].(string)
	if id == "" {
		return "", fmt.Errorf("supervisor_service_id is required")
	}
	version, _ := args["version"].(string)
	if version == "" {
		return "", fmt.Errorf("version is required")
	}
	if err := m.ActivateSupervisorServiceVersion(ctx, id, version); err != nil {
		return "", fmt.Errorf("failed to activate supervisor service %q version %q: %w", id, version, err)
	}
	return marshalJSON(map[string]interface{}{"result": "supervisor_service_version_activated", "supervisor_service_id": id, "version": version})
}

func handleNamespaceDeactivateSupervisorServiceVersion(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := namespaceServicesManager(ctx, client)
	if err != nil {
		return "", err
	}
	id, _ := args["supervisor_service_id"].(string)
	if id == "" {
		return "", fmt.Errorf("supervisor_service_id is required")
	}
	version, _ := args["version"].(string)
	if version == "" {
		return "", fmt.Errorf("version is required")
	}
	if err := m.DeactivateSupervisorServiceVersion(ctx, id, version); err != nil {
		return "", fmt.Errorf("failed to deactivate supervisor service %q version %q: %w", id, version, err)
	}
	return marshalJSON(map[string]interface{}{"result": "supervisor_service_version_deactivated", "supervisor_service_id": id, "version": version})
}

func handleNamespaceRemoveSupervisorService(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := namespaceServicesManager(ctx, client)
	if err != nil {
		return "", err
	}
	id, _ := args["supervisor_service_id"].(string)
	if id == "" {
		return "", fmt.Errorf("supervisor_service_id is required")
	}
	if err := m.RemoveSupervisorService(ctx, id); err != nil {
		return "", fmt.Errorf("failed to remove supervisor service %q: %w", id, err)
	}
	return marshalJSON(map[string]interface{}{"result": "supervisor_service_removed", "supervisor_service_id": id})
}

func handleNamespaceRemoveSupervisorServiceVersion(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := namespaceServicesManager(ctx, client)
	if err != nil {
		return "", err
	}
	id, _ := args["supervisor_service_id"].(string)
	if id == "" {
		return "", fmt.Errorf("supervisor_service_id is required")
	}
	version, _ := args["version"].(string)
	if version == "" {
		return "", fmt.Errorf("version is required")
	}
	if err := m.RemoveSupervisorServiceVersion(ctx, id, version); err != nil {
		return "", fmt.Errorf("failed to remove supervisor service %q version %q: %w", id, version, err)
	}
	return marshalJSON(map[string]interface{}{"result": "supervisor_service_version_removed", "supervisor_service_id": id, "version": version})
}

// -------- networks.go handlers --------

func handleNamespaceListClusterNetworks(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := namespaceServicesManager(ctx, client)
	if err != nil {
		return "", err
	}
	cluster, err := resolveClusterComputeResource(ctx, client, args)
	if err != nil {
		return "", err
	}
	clusterID := cluster.Reference().Value
	list, err := m.ListClusterNetworks(ctx, clusterID)
	if err != nil {
		return "", fmt.Errorf("failed to list cluster networks for cluster %q: %w", clusterID, err)
	}
	return marshalJSON(map[string]interface{}{"cluster": clusterID, "count": len(list), "networks": list})
}

func handleNamespaceGetClusterNetwork(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := namespaceServicesManager(ctx, client)
	if err != nil {
		return "", err
	}
	cluster, err := resolveClusterComputeResource(ctx, client, args)
	if err != nil {
		return "", err
	}
	networkID, _ := args["network_id"].(string)
	if networkID == "" {
		return "", fmt.Errorf("network_id is required")
	}
	info, err := m.GetClusterNetwork(ctx, cluster.Reference().Value, networkID)
	if err != nil {
		return "", fmt.Errorf("failed to get cluster network %q: %w", networkID, err)
	}
	return marshalJSON(info)
}

func handleNamespaceCreateClusterNetwork(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := namespaceServicesManager(ctx, client)
	if err != nil {
		return "", err
	}
	cluster, err := resolveClusterComputeResource(ctx, client, args)
	if err != nil {
		return "", err
	}
	clusterID := cluster.Reference().Value

	specRaw, ok := args["spec"]
	if !ok || specRaw == nil {
		return "", fmt.Errorf("spec is required")
	}
	var spec namespace.NetworkCreateSpec
	if err := decodeJSONArg(specRaw, &spec); err != nil {
		return "", fmt.Errorf("spec: %w", err)
	}
	pgValue, err := resolveClusterNetworkPortgroup(ctx, client, specRaw)
	if err != nil {
		return "", err
	}
	if pgValue != "" && spec.VsphereNetwork != nil {
		spec.VsphereNetwork.Portgroup = pgValue
	}

	if err := m.CreateClusterNetwork(ctx, clusterID, &spec); err != nil {
		return "", fmt.Errorf("failed to create cluster network on cluster %q: %w", clusterID, err)
	}
	return marshalJSON(map[string]interface{}{"result": "cluster_network_created", "cluster": clusterID, "network": spec.Network})
}

func handleNamespaceUpdateClusterNetwork(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := namespaceServicesManager(ctx, client)
	if err != nil {
		return "", err
	}
	cluster, err := resolveClusterComputeResource(ctx, client, args)
	if err != nil {
		return "", err
	}
	clusterID := cluster.Reference().Value
	networkID, _ := args["network_id"].(string)
	if networkID == "" {
		return "", fmt.Errorf("network_id is required")
	}

	specRaw, ok := args["spec"]
	if !ok || specRaw == nil {
		return "", fmt.Errorf("spec is required")
	}
	var spec namespace.NetworkUpdateSpec
	if err := decodeJSONArg(specRaw, &spec); err != nil {
		return "", fmt.Errorf("spec: %w", err)
	}
	pgValue, err := resolveClusterNetworkPortgroup(ctx, client, specRaw)
	if err != nil {
		return "", err
	}
	if pgValue != "" && spec.VsphereNetwork != nil {
		spec.VsphereNetwork.Portgroup = pgValue
	}

	if err := m.UpdateClusterNetwork(ctx, clusterID, networkID, &spec); err != nil {
		return "", fmt.Errorf("failed to update cluster network %q: %w", networkID, err)
	}
	return marshalJSON(map[string]interface{}{"result": "cluster_network_updated", "cluster": clusterID, "network_id": networkID})
}

func handleNamespaceSetClusterNetwork(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := namespaceServicesManager(ctx, client)
	if err != nil {
		return "", err
	}
	cluster, err := resolveClusterComputeResource(ctx, client, args)
	if err != nil {
		return "", err
	}
	clusterID := cluster.Reference().Value
	networkID, _ := args["network_id"].(string)
	if networkID == "" {
		return "", fmt.Errorf("network_id is required")
	}

	specRaw, ok := args["spec"]
	if !ok || specRaw == nil {
		return "", fmt.Errorf("spec is required")
	}
	var spec namespace.NetworkSetSpec
	if err := decodeJSONArg(specRaw, &spec); err != nil {
		return "", fmt.Errorf("spec: %w", err)
	}
	pgValue, err := resolveClusterNetworkPortgroup(ctx, client, specRaw)
	if err != nil {
		return "", err
	}
	if pgValue != "" && spec.VsphereNetwork != nil {
		spec.VsphereNetwork.Portgroup = pgValue
	}

	if err := m.SetClusterNetwork(ctx, clusterID, networkID, &spec); err != nil {
		return "", fmt.Errorf("failed to set cluster network %q: %w", networkID, err)
	}
	return marshalJSON(map[string]interface{}{"result": "cluster_network_set", "cluster": clusterID, "network_id": networkID})
}

func handleNamespaceDeleteClusterNetwork(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := namespaceServicesManager(ctx, client)
	if err != nil {
		return "", err
	}
	cluster, err := resolveClusterComputeResource(ctx, client, args)
	if err != nil {
		return "", err
	}
	clusterID := cluster.Reference().Value
	networkID, _ := args["network_id"].(string)
	if networkID == "" {
		return "", fmt.Errorf("network_id is required")
	}
	if err := m.DeleteClusterNetwork(ctx, clusterID, networkID); err != nil {
		return "", fmt.Errorf("failed to delete cluster network %q: %w", networkID, err)
	}
	return marshalJSON(map[string]interface{}{"result": "cluster_network_deleted", "cluster": clusterID, "network_id": networkID})
}

// -------- namespace_v2.go handlers --------

func handleNamespaceListNamespacesV2(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := namespaceServicesManager(ctx, client)
	if err != nil {
		return "", err
	}
	list, err := m.ListNamespacesV2(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to list namespaces (v2): %w", err)
	}
	return marshalJSON(map[string]interface{}{"count": len(list), "namespaces": list})
}

func handleNamespaceGetNamespaceV2(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := namespaceServicesManager(ctx, client)
	if err != nil {
		return "", err
	}
	ns, _ := args["namespace"].(string)
	if ns == "" {
		return "", fmt.Errorf("namespace is required")
	}
	info, err := m.GetNamespaceV2(ctx, ns)
	if err != nil {
		return "", fmt.Errorf("failed to get namespace %q (v2): %w", ns, err)
	}
	return marshalJSON(info)
}

func handleNamespaceCreateNamespaceV2(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := namespaceServicesManager(ctx, client)
	if err != nil {
		return "", err
	}

	nsName, _ := args["namespace"].(string)
	if nsName == "" {
		return "", fmt.Errorf("namespace is required")
	}
	supervisorPath, _ := args["supervisor"].(string)
	if supervisorPath == "" {
		return "", fmt.Errorf("supervisor is required")
	}
	cluster, err := resolveClusterComputeResource(ctx, client, map[string]interface{}{"cluster": supervisorPath})
	if err != nil {
		return "", fmt.Errorf("supervisor: %w", err)
	}

	var spec namespace.NamespaceInstanceCreateSpecV2
	if err := decodeJSONArg(args, &spec); err != nil {
		return "", fmt.Errorf("failed to decode create spec: %w", err)
	}
	spec.Namespace = nsName
	spec.Supervisor = cluster.Reference().Value

	if err := m.CreateNamespaceV2(ctx, spec); err != nil {
		return "", fmt.Errorf("failed to create namespace %q (v2): %w", nsName, err)
	}
	return marshalJSON(map[string]interface{}{"result": "namespace_created", "namespace": nsName, "supervisor": spec.Supervisor})
}

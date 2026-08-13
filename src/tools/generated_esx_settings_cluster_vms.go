// Package tools — generated_esx_settings_cluster_vms.go is Fase 8a (Wave 2,
// group CS-CRYPTO) of the codegen plan
// (".workspace/plans/MCPVMWare2026-08-10-175300-plano-cobertura-completa-api-codegen.plan.md"),
// covering govmomi's vapi/esx/settings/clusters/vms package:
// referencia/govmomi/vapi/esx/settings/clusters/vms/
// {transition,solutions,lifecycle_hook,apply,compliance,list}.go. 16 tools
// total (17 in the brief — see "Real dependency drift" below for why one was
// dropped) — vLCM (vSphere Lifecycle Manager) "System VM Solutions"
// management for a cluster (e.g. NSX/EAM-managed appliances transitioning
// from EAM to vLCM ownership).
//
// Real dependency drift (found by actually building, not assumed):
// transition.DeleteSolutionOnly — one of the 17 methods named in the
// brief — does NOT exist in this project's actually-compiled govmomi
// dependency. `go.mod` pins github.com/vmware/govmomi v0.55.1 (no replace
// directive), which is OLDER than the referencia/govmomi/ checkout this
// batch was briefed to read from — `diff --strip-trailing-cr` between
// $(go env GOMODCACHE)/github.com/vmware/govmomi@v0.55.1/vapi/esx/settings/clusters/vms/transition.go
// and referencia/govmomi/vapi/esx/settings/clusters/vms/transition.go shows
// DeleteSolutionOnly is the ONLY difference between the two files — every
// other method in every other file this group touches (solutions.go,
// lifecycle_hook.go, apply.go, compliance.go, list.go, ovf_resource.go, and
// vapi/cluster/cluster.go, vapi/crypto/crypto.go, vapi/cis/tasks/tasks.go in
// full) diffed byte-identical against v0.55.1. First attempt at this file
// included a vmware_esx_settings_clusters_vms_delete_solution_only tool
// wrapping it; `go build ./...` failed with "m.DeleteSolutionOnly undefined
// (type *vms.Manager has no field or method DeleteSolutionOnly)", confirming
// this is a real API-surface gap in the pinned dependency, not a typo. That
// tool (and its handler) were removed rather than bumping go.mod, which
// would be an unreviewed, out-of-scope dependency change touching every
// other Fase's generated files — flagging this for the orchestrator instead.
//
// Architecture (same as generated_tags.go, the reference file for this
// domain family): vapi/* wraps a *rest.Client (VAPI/JSON, not SOAP) — every
// struct here already carries real `json:"..."` tags. This file follows
// generated_tags.go's own precedent exactly: SMALL/flat structs
// (ProcessedHookSpec's 3 fields, DynamicUpdateSpec's 3 scalar fields) are
// built field-by-field from named top-level arguments (matches tags.go's
// CreateTag/CreateCategory), while LARGE/deeply-nested structs (EnableSpec,
// MultiSourceEnableSpec, TransitionSpec, SolutionSpec, ApplySpec,
// CheckComplianceFilterSpec — several of which recursively embed
// *SolutionSpec, itself ~15 fields with nested objects) are accepted as one
// generic JSON object argument and decoded via decodeJSONArg
// (generated_vm_lifecycle.go) — the same generic marshal/unmarshal bridge
// already used for types.LocalizableMessage in generated_task.go and
// types.Permission in generated_authorization.go. decodeJSONArg is reused
// as-is here, not reimplemented: it is architecture-agnostic (it only
// re-marshals an already-decoded interface{} value to JSON bytes and
// unmarshals into a concrete Go struct via its `json:"..."` tags) — for
// vapi/* structs, whose json tags are native snake_case (unlike vim25/types'
// SOAP structs), this gives exactly the ergonomic "JSON in, JSON out"
// decoding the orchestrator's brief described, without inventing a second,
// duplicate helper that would do the identical marshal/unmarshal dance.
//
// mode=vcenter-only: the entire vapi/* domain requires a vCenter Server
// Appliance (VAMI/VAPI session) — see client.REST's doc comment.
//
// vcsim gap, not a bug: confirmed directly (not assumed) that
// referencia/govmomi/vapi/simulator/simulator.go does NOT import this
// package's simulator sibling (grep -cE
// "vapi/(esx/settings|cluster\"|crypto|cis/tasks)" against that file returns
// 0) — even though a standalone
// vapi/esx/settings/clusters/vms/simulator package DOES exist upstream (used
// by govmomi's own transition_test.go via a blank import this project's
// testhelpers_test.go does NOT perform), so every call from this project's
// vcsim-backed tests reaches a real, unhandled REST endpoint and gets back a
// genuine HTTP-level error (typically 404), not a registration/wiring bug.
// Tests use assertReachesServer (generated_vm_lifecycle_test.go) for exactly
// this reason, same discipline as every other vcsim-gap group in this fase.
//
// Entity resolution: every "cluster" parameter (types.ManagedObjectReference
// identifying the target ClusterComputeResource) is exposed here as a
// "cluster_path" full inventory path argument, resolved via resolveEntityRef
// (generated_authorization.go, Fase 7) — reused as-is per this project's
// "reuse, don't duplicate" discipline, exactly like generated_tags.go's
// inventory_path arguments.
//
// Curation:
//
//   - transition.TransitionSpec.SourceCluster is typed as a plain JSON
//     string in the real API (not a types.ManagedObjectReference) — the
//     source (transition.go) never documents its expected format beyond
//     "the cluster to transition from". Exposed here as a raw
//     "source_cluster" string argument, passed through verbatim rather than
//     resolved via resolveEntityRef — resolving it would require guessing an
//     unproven format (inventory path vs. moref Value vs. some other vLCM-
//     internal cluster identifier), which this file does not do.
//
//   - solutions.go's Get/Set/Delete are named vmware_esx_settings_clusters_vms_
//     {get,set,delete}_solution here (not a bare "get/set/delete") — kept
//     this way even after transition.DeleteSolutionOnly itself had to be
//     dropped (see "Real dependency drift" above), since solutions.Delete
//     (removes the solution's desired spec outright) and
//     DeleteSolutionOnly (marks System VMs for abandonment on the next
//     apply, spec generally still visible until then) really are two
//     different operations in the upstream API, and a future govmomi bump
//     restoring DeleteSolutionOnly should not have to rename this one.
//
//   - Timeout handling: the brief only explicitly called out
//     ApplyWaitForCompletion as needing an OBLIGATORY timeout_seconds (no
//     default — see requiredWaitTimeout, defined in generated_cis_tasks.go
//     and reused here per the same "written together in this batch, define
//     once" discipline as resolveEntityRef/toMoRefs above). But reading the
//     real source shows every one of Enable/MultiSourceEnable/Transition
//     (transition.go) and solutions.Set/solutions.Delete (solutions.go) and
//     compliance.CheckCompliance (compliance.go) ALSO internally call
//     tasks.NewManager(m.Client).WaitForCompletion(ctx, taskId) — the exact
//     same "polling/wait with no server-side timeout of its own" shape this
//     project's established convention (§"Padrão já estabelecido" in this
//     group's brief; see also generated_vm_lifecycle.go's defaultWaitTimeout
//     postmortem, a REAL 10-minute test hang this project hit once already)
//     exists specifically to guard against. Applied waitTimeoutFrom
//     (optional, defaults to 5 min — generated_vm_lifecycle.go) uniformly to
//     all 6 of those tools too, not just the one the brief named — a
//     defense-in-depth extension of an explicit instruction, not a
//     deviation from it. The *Async variants (EnableAsync/
//     MultiSourceEnableAsync/TransitionAsync), apply.Apply, lifecycle_hook.*,
//     and list.List do NOT block on a task wait (confirmed by reading each:
//     no `.WithParam("vmw-task", "true")` + task-wait call chain) — no
//     timeout argument was added to those.
//
//   - lifecycle_hook.MarkAsProcessed's real signature returns
//     (*HookListResult, error), but reading the source shows it ALWAYS
//     returns (nil, nil) on success — the response body is decoded into a
//     throwaway rest.Error value that is only inspected on failure. This is
//     upstream govmomi's own behavior (not a bug introduced here); the
//     handler below reports plain success, not a fabricated HookListResult.
package tools

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/vapi/esx/settings/clusters/vms"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

func registerEsxSettingsClusterVMsTools(r *Registry) {
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}
	clusterPathArg := map[string]interface{}{
		"type":        "string",
		"description": `Full inventory path of the target ClusterComputeResource (e.g. "/DC0/host/cluster1"), resolved via SearchIndex.FindByInventoryPath.`,
	}
	solutionArg := map[string]interface{}{
		"type":        "string",
		"description": `Identifier of the vLCM System VM Solution within the cluster (e.g. an EAM agency ID, or a caller-defined identifier used consistently across Set/Get/Delete/Enable/Transition calls for it).`,
	}
	solutionSpecArg := map[string]interface{}{
		"type":        "object",
		"description": `A vms.SolutionSpec JSON object (see referencia/govmomi/vapi/esx/settings/clusters/vms/solutions.go) describing the desired vLCM solution configuration. Key fields: deployment_type ("EVERY_HOST_PINNED"|"CLUSTER_VM_SET"), display_name, display_version, vm_name_template ({prefix, suffix: "UUID"|"COUNTER"}), cluster_solution_spec (only for CLUSTER_VM_SET: vm_count, vm_placement_policies, vm_networks, vm_datastores, devices, remediation_policy, alternative_vm_specs), hook_configurations (map of "POST_PROVISIONING"/"POST_POWER_ON" -> {timeout}), ovf_resource ({location_type, url, ssl_certificate_validation, certificate, authentication_scheme}), ovf_descriptor_properties, vm_clone_config, vm_storage_policy, vm_storage_profiles, vm_disk_type, vm_resource_pool, vm_folder, vm_resource_spec, redeployment_policy.`,
	}
	optionalTimeoutArg := map[string]interface{}{
		"type":        "integer",
		"description": "Give up after this many seconds and return an error instead of blocking forever — this call polls a REST task with no server-side timeout of its own (see this file's top doc comment). Default 300 (5 minutes) if omitted.",
	}

	// --- transition.go (6 — DeleteSolutionOnly dropped, see this file's top
	// doc comment "Real dependency drift": not present in the pinned
	// govmomi v0.55.1) ---

	r.registerDestructive("vmware_esx_settings_clusters_vms_enable_async",
		"Enable an EAM-managed solution in vLCM asynchronously and return a task ID (feed it into vmware_cis_tasks_wait_for_completion for polling). Only transfers ownership from EAM to vLCM and sets the desired state — the solution's System VMs are untouched until a following apply.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"cluster_path":  clusterPathArg,
				"solution":      solutionArg,
				"eam_agency_id": map[string]interface{}{"type": "string", "description": "Identifier of the solution in EAM (EAM agency) being transitioned to vLCM."},
				"desired_spec":  solutionSpecArg,
				"confirm":       confirmArg,
			},
			"required": []interface{}{"cluster_path", "solution", "eam_agency_id", "desired_spec", "confirm"},
		},
		Tool{Handler: handleEsxSettingsClusterVMsEnableAsync},
	)

	r.registerDestructive("vmware_esx_settings_clusters_vms_enable",
		"Synchronous counterpart of vmware_esx_settings_clusters_vms_enable_async — enables an EAM-managed solution in vLCM and blocks until the underlying task completes (bounded by timeout_seconds, default 300s — see this file's top doc comment).",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"cluster_path":    clusterPathArg,
				"solution":        solutionArg,
				"eam_agency_id":   map[string]interface{}{"type": "string", "description": "Identifier of the solution in EAM (EAM agency) being transitioned to vLCM."},
				"desired_spec":    solutionSpecArg,
				"timeout_seconds": optionalTimeoutArg,
				"confirm":         confirmArg,
			},
			"required": []interface{}{"cluster_path", "solution", "eam_agency_id", "desired_spec", "confirm"},
		},
		Tool{Handler: handleEsxSettingsClusterVMsEnable},
	)

	r.registerDestructive("vmware_esx_settings_clusters_vms_multi_source_enable_async",
		"Enable multiple EAM-managed solutions in vLCM as a single combined solution, asynchronously, and return a task ID. Supported only for solutions with deployment type CLUSTER_VM_SET.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"cluster_path":   clusterPathArg,
				"solution":       solutionArg,
				"eam_agency_ids": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "EAM agency identifiers of the solutions being merged. Must be non-empty."},
				"desired_spec":   solutionSpecArg,
				"cluster_module": map[string]interface{}{"type": "string", "description": "Optional: existing cluster module (see vmware_cluster_create_module) to reuse for VM-VM anti-affinity between the transitioned System VMs. If omitted, vLCM creates a new module if needed."},
				"vm_selections":  map[string]interface{}{"type": "object", "description": `Optional map from VM identifier to a VmSelectionSpec JSON object ({"selection_type": "VM_EXTRA_CONFIG", "extra_config_value": "..."}), matching MultiSourceEnableSpec.SourceVmSelectionSpecs. Provided VM IDs must be part of the solution being transitioned and present in the desired_spec's cluster_solution_spec.alternative_vm_specs.`},
				"confirm":        confirmArg,
			},
			"required": []interface{}{"cluster_path", "solution", "eam_agency_ids", "desired_spec", "confirm"},
		},
		Tool{Handler: handleEsxSettingsClusterVMsMultiSourceEnableAsync},
	)

	r.registerDestructive("vmware_esx_settings_clusters_vms_multi_source_enable",
		"Synchronous counterpart of vmware_esx_settings_clusters_vms_multi_source_enable_async — blocks until the underlying task completes (bounded by timeout_seconds, default 300s).",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"cluster_path":    clusterPathArg,
				"solution":        solutionArg,
				"eam_agency_ids":  map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "EAM agency identifiers of the solutions being merged. Must be non-empty."},
				"desired_spec":    solutionSpecArg,
				"cluster_module":  map[string]interface{}{"type": "string", "description": "Optional: existing cluster module to reuse for VM-VM anti-affinity between the transitioned System VMs."},
				"vm_selections":   map[string]interface{}{"type": "object", "description": `Optional map from VM identifier to a VmSelectionSpec JSON object, matching MultiSourceEnableSpec.SourceVmSelectionSpecs.`},
				"timeout_seconds": optionalTimeoutArg,
				"confirm":         confirmArg,
			},
			"required": []interface{}{"cluster_path", "solution", "eam_agency_ids", "desired_spec", "confirm"},
		},
		Tool{Handler: handleEsxSettingsClusterVMsMultiSourceEnable},
	)

	r.registerDestructive("vmware_esx_settings_clusters_vms_transition_async",
		"Transition a System VM Solution's desired state to a target cluster, asynchronously, and return a task ID. Only initiates the transition — the target desired state is NOT applied and the solution's System VMs remain untouched until a following vmware_esx_settings_clusters_vms_apply.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"cluster_path":   clusterPathArg,
				"solution":       solutionArg,
				"source_cluster": map[string]interface{}{"type": "string", "description": "Opaque identifier of the cluster to transition from — see this file's top doc comment ('Curation') for why this is passed through verbatim rather than resolved as an inventory path."},
				"desired_spec":   solutionSpecArg,
				"confirm":        confirmArg,
			},
			"required": []interface{}{"cluster_path", "solution", "source_cluster", "desired_spec", "confirm"},
		},
		Tool{Handler: handleEsxSettingsClusterVMsTransitionAsync},
	)

	r.registerDestructive("vmware_esx_settings_clusters_vms_transition",
		"Synchronous counterpart of vmware_esx_settings_clusters_vms_transition_async — blocks until the underlying task completes (bounded by timeout_seconds, default 300s).",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"cluster_path":    clusterPathArg,
				"solution":        solutionArg,
				"source_cluster":  map[string]interface{}{"type": "string", "description": "Opaque identifier of the cluster to transition from — see this file's top doc comment ('Curation')."},
				"desired_spec":    solutionSpecArg,
				"timeout_seconds": optionalTimeoutArg,
				"confirm":         confirmArg,
			},
			"required": []interface{}{"cluster_path", "solution", "source_cluster", "desired_spec", "confirm"},
		},
		Tool{Handler: handleEsxSettingsClusterVMsTransition},
	)

	// --- solutions.go (3) ---

	r.register("vmware_esx_settings_clusters_vms_get_solution",
		"Fetch the current desired specification for a solution in a cluster.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"cluster_path": clusterPathArg, "solution": solutionArg},
			"required":   []interface{}{"cluster_path", "solution"},
		},
		Tool{Handler: handleEsxSettingsClusterVMsGetSolution},
	)

	r.registerDestructive("vmware_esx_settings_clusters_vms_set_solution",
		"Set (create or overwrite) the desired specification for a solution in a cluster. The specification is validated before being accepted. Blocks until the underlying task completes (bounded by timeout_seconds, default 300s).",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"cluster_path":    clusterPathArg,
				"solution":        solutionArg,
				"spec":            solutionSpecArg,
				"timeout_seconds": optionalTimeoutArg,
				"confirm":         confirmArg,
			},
			"required": []interface{}{"cluster_path", "solution", "spec", "confirm"},
		},
		Tool{Handler: handleEsxSettingsClusterVMsSetSolution},
	)

	r.registerDestructive("vmware_esx_settings_clusters_vms_delete_solution",
		"Delete a solution's desired specification for a cluster outright. Irreversible — distinct from vmware_esx_settings_clusters_vms_delete_solution_only, which only marks System VMs for abandonment on the next apply (see this file's top doc comment). Blocks until the underlying task completes (bounded by timeout_seconds, default 300s).",
		tier1,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"cluster_path":    clusterPathArg,
				"solution":        solutionArg,
				"timeout_seconds": optionalTimeoutArg,
				"confirm":         confirmArg,
			},
			"required": []interface{}{"cluster_path", "solution", "confirm"},
		},
		Tool{Handler: handleEsxSettingsClusterVMsDeleteSolution},
	)

	// --- lifecycle_hook.go (3) ---

	r.register("vmware_esx_settings_clusters_vms_list_hooks",
		"List the VM lifecycle hooks currently activated for a solution's System VMs in a cluster (e.g. waiting to be processed after provisioning or power-on).",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"cluster_path": clusterPathArg, "solution": solutionArg},
			"required":   []interface{}{"cluster_path", "solution"},
		},
		Tool{Handler: handleEsxSettingsClusterVMsListHooks},
	)

	r.registerDestructive("vmware_esx_settings_clusters_vms_mark_as_processed",
		"Mark an activated VM lifecycle hook as processed by the solution, unblocking vLCM to continue toward the desired specification for that VM. The real API always reports plain success (no body) — see this file's top doc comment.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"cluster_path":           clusterPathArg,
				"vm":                     map[string]interface{}{"type": "string", "description": "Identifier of the VM for which the hook is activated (ProcessedHookSpec.Vm)."},
				"lifecycle_state":        map[string]interface{}{"type": "string", "enum": []interface{}{"POST_PROVISIONING", "POST_POWER_ON"}, "description": "Which lifecycle hook is being acknowledged."},
				"processed_successfully": map[string]interface{}{"type": "boolean", "description": "Whether the solution processed the hook successfully."},
				"confirm":                confirmArg,
			},
			"required": []interface{}{"cluster_path", "vm", "lifecycle_state", "processed_successfully", "confirm"},
		},
		Tool{Handler: handleEsxSettingsClusterVMsMarkAsProcessed},
	)

	r.registerDestructive("vmware_esx_settings_clusters_vms_process_dynamic_update",
		"Apply a dynamic update (an alternative VM spec) to an already-activated VM lifecycle hook, without waiting for the next full apply cycle.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"cluster_path":        clusterPathArg,
				"vm":                  map[string]interface{}{"type": "string", "description": "Identifier of the VM for which the hook is activated (DynamicUpdateSpec.Vm)."},
				"solution":            solutionArg,
				"lifecycle_state":     map[string]interface{}{"type": "string", "enum": []interface{}{"POST_PROVISIONING", "POST_POWER_ON"}, "description": "Which lifecycle hook the dynamic update targets."},
				"alternative_vm_spec": map[string]interface{}{"type": "object", "description": `Optional AlternativeVmSpec JSON object ({"selection_criteria": {...VmSelectionSpec...}, "devices": [...]}). Omit to clear it.`},
				"confirm":             confirmArg,
			},
			"required": []interface{}{"cluster_path", "vm", "solution", "lifecycle_state", "confirm"},
		},
		Tool{Handler: handleEsxSettingsClusterVMsProcessDynamicUpdate},
	)

	// --- apply.go (2) ---

	r.registerDestructive("vmware_esx_settings_clusters_vms_apply",
		"Apply the current desired solution specification(s) to a cluster and return a task ID (feed it into vmware_esx_settings_clusters_vms_apply_wait_for_completion, or vmware_cis_tasks_wait_for_completion, to poll it). Does not itself block on the task.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"cluster_path": clusterPathArg,
				"apply_spec": map[string]interface{}{
					"type":        "object",
					"description": `Optional ApplySpec JSON object narrowing which solutions/hosts to apply: {"host_solutions": {"solutions": [...], "hosts": [...]}, "cluster_solutions": {"solutions": [...], "hosts": [...]}} (both filters optional; omitted/empty on both means "apply everything on the cluster").`,
				},
				"confirm": confirmArg,
			},
			"required": []interface{}{"cluster_path", "confirm"},
		},
		Tool{Handler: handleEsxSettingsClusterVMsApply},
	)

	r.registerDestructive("vmware_esx_settings_clusters_vms_apply_wait_for_completion",
		"Block until an apply task (started by vmware_esx_settings_clusters_vms_apply) completes, and return its ApplyResult. timeout_seconds is REQUIRED (no default) — this polls a REST task with no server-side timeout of its own.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"task_id":         map[string]interface{}{"type": "string", "description": "Task ID returned by vmware_esx_settings_clusters_vms_apply."},
				"timeout_seconds": requiredTimeoutSecondsArg,
				"confirm":         confirmArg,
			},
			"required": []interface{}{"task_id", "timeout_seconds", "confirm"},
		},
		Tool{Handler: handleEsxSettingsClusterVMsApplyWaitForCompletion},
	)

	// --- compliance.go (1) ---

	r.registerDestructive("vmware_esx_settings_clusters_vms_check_compliance",
		"Check compliance of a cluster's System VM deployments against their desired solution specifications. Blocks until the underlying task completes (bounded by timeout_seconds, default 300s) — despite being a read/check operation, this is registered as tier2 exactly as classified by the codegen brief for this group (kept as specified, not independently 'corrected' — see this file's top doc comment).",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"cluster_path": clusterPathArg,
				"filter_spec": map[string]interface{}{
					"type":        "object",
					"description": `Optional CheckComplianceFilterSpec JSON object narrowing the check: {"solutions": [...], "hosts": [...], "deployment_units": [...]} (all optional; omitted/empty means "check everything in the cluster").`,
				},
				"timeout_seconds": optionalTimeoutArg,
				"confirm":         confirmArg,
			},
			"required": []interface{}{"cluster_path", "confirm"},
		},
		Tool{Handler: handleEsxSettingsClusterVMsCheckCompliance},
	)

	// --- list.go (1) ---

	r.register("vmware_esx_settings_clusters_vms_list_solutions",
		"List every solution's desired specification currently set on a cluster.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"cluster_path": clusterPathArg},
			"required":   []interface{}{"cluster_path"},
		},
		Tool{Handler: handleEsxSettingsClusterVMsListSolutions},
	)
}

// esxSettingsClusterVMsManager returns a vms.Manager bound to client's VAPI/
// REST session, logging in lazily via client.REST — same pattern as
// generated_tags.go's tagsManager. vms.Manager has no exported NewManager
// constructor upstream (confirmed by reading the package; only
// &vms.Manager{Client: rc} is used, e.g. in transition_test.go), so it is
// built via a struct literal here, same as upstream's own tests do.
func esxSettingsClusterVMsManager(ctx context.Context, client *vmware.Client) (*vms.Manager, error) {
	rc, err := client.REST(ctx)
	if err != nil {
		return nil, err
	}
	return &vms.Manager{Client: rc}, nil
}

// decodeSolutionSpec decodes the required "key" argument (a generic JSON
// object) into a *vms.SolutionSpec.
func decodeSolutionSpec(args map[string]interface{}, key string) (*vms.SolutionSpec, error) {
	raw, ok := args[key]
	if !ok || raw == nil {
		return nil, fmt.Errorf("%s is required", key)
	}
	var spec vms.SolutionSpec
	if err := decodeJSONArg(raw, &spec); err != nil {
		return nil, fmt.Errorf("invalid %s: %w", key, err)
	}
	return &spec, nil
}

func handleEsxSettingsClusterVMsEnableAsync(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := esxSettingsClusterVMsManager(ctx, client)
	if err != nil {
		return "", err
	}
	clusterPath, _ := args["cluster_path"].(string)
	ref, err := resolveEntityRef(ctx, client, clusterPath)
	if err != nil {
		return "", err
	}
	solution, _ := args["solution"].(string)
	if solution == "" {
		return "", fmt.Errorf("solution is required")
	}
	eamAgencyID, _ := args["eam_agency_id"].(string)
	if eamAgencyID == "" {
		return "", fmt.Errorf("eam_agency_id is required")
	}
	desired, err := decodeSolutionSpec(args, "desired_spec")
	if err != nil {
		return "", err
	}

	taskID, err := m.EnableAsync(ctx, ref, solution, &vms.EnableSpec{EamAgencyID: eamAgencyID, Solution: desired})
	if err != nil {
		return "", fmt.Errorf("failed to enable-async solution %q on %s: %w", solution, clusterPath, err)
	}
	return marshalJSON(map[string]interface{}{"result": "enable_started", "cluster_path": clusterPath, "solution": solution, "task_id": taskID})
}

func handleEsxSettingsClusterVMsEnable(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := esxSettingsClusterVMsManager(ctx, client)
	if err != nil {
		return "", err
	}
	clusterPath, _ := args["cluster_path"].(string)
	ref, err := resolveEntityRef(ctx, client, clusterPath)
	if err != nil {
		return "", err
	}
	solution, _ := args["solution"].(string)
	if solution == "" {
		return "", fmt.Errorf("solution is required")
	}
	eamAgencyID, _ := args["eam_agency_id"].(string)
	if eamAgencyID == "" {
		return "", fmt.Errorf("eam_agency_id is required")
	}
	desired, err := decodeSolutionSpec(args, "desired_spec")
	if err != nil {
		return "", err
	}
	waitCtx, cancel, err := waitTimeoutFrom(ctx, args)
	if err != nil {
		return "", err
	}
	defer cancel()

	if err := m.Enable(waitCtx, ref, solution, &vms.EnableSpec{EamAgencyID: eamAgencyID, Solution: desired}); err != nil {
		return "", fmt.Errorf("failed to enable solution %q on %s: %w", solution, clusterPath, err)
	}
	return marshalJSON(map[string]interface{}{"result": "enabled", "cluster_path": clusterPath, "solution": solution})
}

func handleEsxSettingsClusterVMsMultiSourceEnableAsync(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := esxSettingsClusterVMsManager(ctx, client)
	if err != nil {
		return "", err
	}
	spec, ref, clusterPath, solution, err := buildMultiSourceEnableSpec(ctx, client, args)
	if err != nil {
		return "", err
	}

	taskID, err := m.MultiSourceEnableAsync(ctx, ref, solution, spec)
	if err != nil {
		return "", fmt.Errorf("failed to multi-source-enable-async solution %q on %s: %w", solution, clusterPath, err)
	}
	return marshalJSON(map[string]interface{}{"result": "multi_source_enable_started", "cluster_path": clusterPath, "solution": solution, "task_id": taskID})
}

func handleEsxSettingsClusterVMsMultiSourceEnable(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := esxSettingsClusterVMsManager(ctx, client)
	if err != nil {
		return "", err
	}
	spec, ref, clusterPath, solution, err := buildMultiSourceEnableSpec(ctx, client, args)
	if err != nil {
		return "", err
	}
	waitCtx, cancel, err := waitTimeoutFrom(ctx, args)
	if err != nil {
		return "", err
	}
	defer cancel()

	if err := m.MultiSourceEnable(waitCtx, ref, solution, spec); err != nil {
		return "", fmt.Errorf("failed to multi-source-enable solution %q on %s: %w", solution, clusterPath, err)
	}
	return marshalJSON(map[string]interface{}{"result": "multi_source_enabled", "cluster_path": clusterPath, "solution": solution})
}

// buildMultiSourceEnableSpec factors the argument parsing shared by the
// async/sync MultiSourceEnable handlers.
func buildMultiSourceEnableSpec(ctx context.Context, client *vmware.Client, args map[string]interface{}) (*vms.MultiSourceEnableSpec, types.ManagedObjectReference, string, string, error) {
	clusterPath, _ := args["cluster_path"].(string)
	ref, err := resolveEntityRef(ctx, client, clusterPath)
	if err != nil {
		return nil, types.ManagedObjectReference{}, "", "", err
	}
	solution, _ := args["solution"].(string)
	if solution == "" {
		return nil, types.ManagedObjectReference{}, "", "", fmt.Errorf("solution is required")
	}
	eamAgencyIDs, err := toStringSlice(args["eam_agency_ids"])
	if err != nil {
		return nil, types.ManagedObjectReference{}, "", "", fmt.Errorf("invalid eam_agency_ids: %w", err)
	}
	if len(eamAgencyIDs) == 0 {
		return nil, types.ManagedObjectReference{}, "", "", fmt.Errorf("eam_agency_ids is required and must be non-empty")
	}
	desired, err := decodeSolutionSpec(args, "desired_spec")
	if err != nil {
		return nil, types.ManagedObjectReference{}, "", "", err
	}
	clusterModule, _ := args["cluster_module"].(string)

	spec := &vms.MultiSourceEnableSpec{EamAgencyIDs: eamAgencyIDs, Solution: desired, ClusterModule: clusterModule}
	if raw, ok := args["vm_selections"]; ok && raw != nil {
		var m map[string]vms.VmSelectionSpec
		if err := decodeJSONArg(raw, &m); err != nil {
			return nil, types.ManagedObjectReference{}, "", "", fmt.Errorf("invalid vm_selections: %w", err)
		}
		spec.SourceVmSelectionSpecs = m
	}

	return spec, ref, clusterPath, solution, nil
}

func handleEsxSettingsClusterVMsTransitionAsync(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := esxSettingsClusterVMsManager(ctx, client)
	if err != nil {
		return "", err
	}
	spec, ref, clusterPath, solution, err := buildTransitionSpec(ctx, client, args)
	if err != nil {
		return "", err
	}

	taskID, err := m.TransitionAsync(ctx, ref, solution, spec)
	if err != nil {
		return "", fmt.Errorf("failed to transition-async solution %q on %s: %w", solution, clusterPath, err)
	}
	return marshalJSON(map[string]interface{}{"result": "transition_started", "cluster_path": clusterPath, "solution": solution, "task_id": taskID})
}

func handleEsxSettingsClusterVMsTransition(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := esxSettingsClusterVMsManager(ctx, client)
	if err != nil {
		return "", err
	}
	spec, ref, clusterPath, solution, err := buildTransitionSpec(ctx, client, args)
	if err != nil {
		return "", err
	}
	waitCtx, cancel, err := waitTimeoutFrom(ctx, args)
	if err != nil {
		return "", err
	}
	defer cancel()

	if err := m.Transition(waitCtx, ref, solution, spec); err != nil {
		return "", fmt.Errorf("failed to transition solution %q on %s: %w", solution, clusterPath, err)
	}
	return marshalJSON(map[string]interface{}{"result": "transitioned", "cluster_path": clusterPath, "solution": solution})
}

// buildTransitionSpec factors the argument parsing shared by the async/sync
// Transition handlers.
func buildTransitionSpec(ctx context.Context, client *vmware.Client, args map[string]interface{}) (*vms.TransitionSpec, types.ManagedObjectReference, string, string, error) {
	clusterPath, _ := args["cluster_path"].(string)
	ref, err := resolveEntityRef(ctx, client, clusterPath)
	if err != nil {
		return nil, types.ManagedObjectReference{}, "", "", err
	}
	solution, _ := args["solution"].(string)
	if solution == "" {
		return nil, types.ManagedObjectReference{}, "", "", fmt.Errorf("solution is required")
	}
	sourceCluster, _ := args["source_cluster"].(string)
	if sourceCluster == "" {
		return nil, types.ManagedObjectReference{}, "", "", fmt.Errorf("source_cluster is required")
	}
	desired, err := decodeSolutionSpec(args, "desired_spec")
	if err != nil {
		return nil, types.ManagedObjectReference{}, "", "", err
	}

	return &vms.TransitionSpec{SourceCluster: sourceCluster, Solution: desired}, ref, clusterPath, solution, nil
}

func handleEsxSettingsClusterVMsGetSolution(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := esxSettingsClusterVMsManager(ctx, client)
	if err != nil {
		return "", err
	}
	clusterPath, _ := args["cluster_path"].(string)
	ref, err := resolveEntityRef(ctx, client, clusterPath)
	if err != nil {
		return "", err
	}
	solution, _ := args["solution"].(string)
	if solution == "" {
		return "", fmt.Errorf("solution is required")
	}

	info, err := m.Get(ctx, ref, solution)
	if err != nil {
		return "", fmt.Errorf("failed to get solution %q on %s: %w", solution, clusterPath, err)
	}
	return marshalJSON(map[string]interface{}{"cluster_path": clusterPath, "solution": solution, "info": info})
}

func handleEsxSettingsClusterVMsSetSolution(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := esxSettingsClusterVMsManager(ctx, client)
	if err != nil {
		return "", err
	}
	clusterPath, _ := args["cluster_path"].(string)
	ref, err := resolveEntityRef(ctx, client, clusterPath)
	if err != nil {
		return "", err
	}
	solution, _ := args["solution"].(string)
	if solution == "" {
		return "", fmt.Errorf("solution is required")
	}
	spec, err := decodeSolutionSpec(args, "spec")
	if err != nil {
		return "", err
	}
	waitCtx, cancel, err := waitTimeoutFrom(ctx, args)
	if err != nil {
		return "", err
	}
	defer cancel()

	if err := m.Set(waitCtx, ref, solution, spec); err != nil {
		return "", fmt.Errorf("failed to set solution %q on %s: %w", solution, clusterPath, err)
	}
	return marshalJSON(map[string]interface{}{"result": "solution_set", "cluster_path": clusterPath, "solution": solution})
}

func handleEsxSettingsClusterVMsDeleteSolution(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := esxSettingsClusterVMsManager(ctx, client)
	if err != nil {
		return "", err
	}
	clusterPath, _ := args["cluster_path"].(string)
	ref, err := resolveEntityRef(ctx, client, clusterPath)
	if err != nil {
		return "", err
	}
	solution, _ := args["solution"].(string)
	if solution == "" {
		return "", fmt.Errorf("solution is required")
	}
	waitCtx, cancel, err := waitTimeoutFrom(ctx, args)
	if err != nil {
		return "", err
	}
	defer cancel()

	if err := m.Delete(waitCtx, ref, solution); err != nil {
		return "", fmt.Errorf("failed to delete solution %q on %s: %w", solution, clusterPath, err)
	}
	return marshalJSON(map[string]interface{}{"result": "solution_deleted", "cluster_path": clusterPath, "solution": solution})
}

func handleEsxSettingsClusterVMsListHooks(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := esxSettingsClusterVMsManager(ctx, client)
	if err != nil {
		return "", err
	}
	clusterPath, _ := args["cluster_path"].(string)
	ref, err := resolveEntityRef(ctx, client, clusterPath)
	if err != nil {
		return "", err
	}
	solution, _ := args["solution"].(string)
	if solution == "" {
		return "", fmt.Errorf("solution is required")
	}

	res, err := m.ListHooks(ctx, ref, solution)
	if err != nil {
		return "", fmt.Errorf("failed to list hooks for solution %q on %s: %w", solution, clusterPath, err)
	}
	hooks := []vms.LifecycleHookInfo{}
	if res != nil {
		hooks = res.Hooks
	}
	return marshalJSON(map[string]interface{}{"cluster_path": clusterPath, "solution": solution, "count": len(hooks), "hooks": hooks})
}

func handleEsxSettingsClusterVMsMarkAsProcessed(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := esxSettingsClusterVMsManager(ctx, client)
	if err != nil {
		return "", err
	}
	clusterPath, _ := args["cluster_path"].(string)
	ref, err := resolveEntityRef(ctx, client, clusterPath)
	if err != nil {
		return "", err
	}
	vm, _ := args["vm"].(string)
	if vm == "" {
		return "", fmt.Errorf("vm is required")
	}
	lifecycleState, _ := args["lifecycle_state"].(string)
	switch vms.LifecycleState(lifecycleState) {
	case vms.PostProvisioning, vms.PostPowerOn:
	default:
		return "", fmt.Errorf(`lifecycle_state must be "POST_PROVISIONING" or "POST_POWER_ON", got %q`, lifecycleState)
	}
	processedOK, ok := args["processed_successfully"].(bool)
	if !ok {
		return "", fmt.Errorf("processed_successfully is required")
	}

	if _, err := m.MarkAsProcessed(ctx, ref, &vms.ProcessedHookSpec{
		Vm:                    vm,
		LifecycleState:        vms.LifecycleState(lifecycleState),
		ProcessedSuccessfully: processedOK,
	}); err != nil {
		return "", fmt.Errorf("failed to mark hook as processed for vm %q on %s: %w", vm, clusterPath, err)
	}
	return marshalJSON(map[string]interface{}{"result": "hook_marked_as_processed", "cluster_path": clusterPath, "vm": vm, "lifecycle_state": lifecycleState})
}

func handleEsxSettingsClusterVMsProcessDynamicUpdate(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := esxSettingsClusterVMsManager(ctx, client)
	if err != nil {
		return "", err
	}
	clusterPath, _ := args["cluster_path"].(string)
	ref, err := resolveEntityRef(ctx, client, clusterPath)
	if err != nil {
		return "", err
	}
	vm, _ := args["vm"].(string)
	if vm == "" {
		return "", fmt.Errorf("vm is required")
	}
	solution, _ := args["solution"].(string)
	if solution == "" {
		return "", fmt.Errorf("solution is required")
	}
	lifecycleState, _ := args["lifecycle_state"].(string)
	switch vms.LifecycleState(lifecycleState) {
	case vms.PostProvisioning, vms.PostPowerOn:
	default:
		return "", fmt.Errorf(`lifecycle_state must be "POST_PROVISIONING" or "POST_POWER_ON", got %q`, lifecycleState)
	}

	spec := &vms.DynamicUpdateSpec{Vm: vm, Solution: solution, LifecycleState: vms.LifecycleState(lifecycleState)}
	if raw, ok := args["alternative_vm_spec"]; ok && raw != nil {
		var avs vms.AlternativeVmSpec
		if err := decodeJSONArg(raw, &avs); err != nil {
			return "", fmt.Errorf("invalid alternative_vm_spec: %w", err)
		}
		spec.AlternativeVmSpec = &avs
	}

	if err := m.ProcessDynamicUpdate(ctx, ref, spec); err != nil {
		return "", fmt.Errorf("failed to process dynamic update for vm %q on %s: %w", vm, clusterPath, err)
	}
	return marshalJSON(map[string]interface{}{"result": "dynamic_update_processed", "cluster_path": clusterPath, "vm": vm, "solution": solution})
}

func handleEsxSettingsClusterVMsApply(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := esxSettingsClusterVMsManager(ctx, client)
	if err != nil {
		return "", err
	}
	clusterPath, _ := args["cluster_path"].(string)
	ref, err := resolveEntityRef(ctx, client, clusterPath)
	if err != nil {
		return "", err
	}
	var spec *vms.ApplySpec
	if raw, ok := args["apply_spec"]; ok && raw != nil {
		var s vms.ApplySpec
		if err := decodeJSONArg(raw, &s); err != nil {
			return "", fmt.Errorf("invalid apply_spec: %w", err)
		}
		spec = &s
	}

	taskID, err := m.Apply(ctx, ref, spec)
	if err != nil {
		return "", fmt.Errorf("failed to apply on %s: %w", clusterPath, err)
	}
	return marshalJSON(map[string]interface{}{"result": "apply_started", "cluster_path": clusterPath, "task_id": taskID})
}

func handleEsxSettingsClusterVMsApplyWaitForCompletion(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := esxSettingsClusterVMsManager(ctx, client)
	if err != nil {
		return "", err
	}
	taskID, _ := args["task_id"].(string)
	if taskID == "" {
		return "", fmt.Errorf("task_id is required")
	}
	waitCtx, cancel, err := requiredWaitTimeout(ctx, args)
	if err != nil {
		return "", err
	}
	defer cancel()

	result, err := m.ApplyWaitForCompletion(waitCtx, taskID)
	if err != nil {
		return "", fmt.Errorf("failed waiting for apply task %s: %w", taskID, err)
	}
	return marshalJSON(map[string]interface{}{"task_id": taskID, "result": result})
}

func handleEsxSettingsClusterVMsCheckCompliance(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := esxSettingsClusterVMsManager(ctx, client)
	if err != nil {
		return "", err
	}
	clusterPath, _ := args["cluster_path"].(string)
	ref, err := resolveEntityRef(ctx, client, clusterPath)
	if err != nil {
		return "", err
	}
	var filterSpec *vms.CheckComplianceFilterSpec
	if raw, ok := args["filter_spec"]; ok && raw != nil {
		var s vms.CheckComplianceFilterSpec
		if err := decodeJSONArg(raw, &s); err != nil {
			return "", fmt.Errorf("invalid filter_spec: %w", err)
		}
		filterSpec = &s
	}
	waitCtx, cancel, err := waitTimeoutFrom(ctx, args)
	if err != nil {
		return "", err
	}
	defer cancel()

	compliance, err := m.CheckCompliance(waitCtx, ref, filterSpec)
	if err != nil {
		return "", fmt.Errorf("failed to check compliance on %s: %w", clusterPath, err)
	}
	return marshalJSON(map[string]interface{}{"cluster_path": clusterPath, "compliance": compliance})
}

func handleEsxSettingsClusterVMsListSolutions(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := esxSettingsClusterVMsManager(ctx, client)
	if err != nil {
		return "", err
	}
	clusterPath, _ := args["cluster_path"].(string)
	ref, err := resolveEntityRef(ctx, client, clusterPath)
	if err != nil {
		return "", err
	}

	res, err := m.List(ctx, ref)
	if err != nil {
		return "", fmt.Errorf("failed to list solutions on %s: %w", clusterPath, err)
	}
	solutions := map[string]vms.SolutionInfo{}
	if res != nil {
		solutions = res.Solutions
	}
	return marshalJSON(map[string]interface{}{"cluster_path": clusterPath, "count": len(solutions), "solutions": solutions})
}

// Package tools — generated_cis_tasks.go is Fase 8a (Wave 2, group
// CS-CRYPTO) of the codegen plan
// (".workspace/plans/MCPVMWare2026-08-10-175300-plano-cobertura-completa-api-codegen.plan.md"),
// covering govmomi's vapi/cis/tasks package (referencia/govmomi/vapi/cis/tasks/tasks.go)
// — the generic vAPI-level task manager (Get/wait-for-state), used by every
// other vapi/* manager in this project (tags, library, and — in this same
// batch — vms/cluster/crypto) whenever a call returns a "vmw-task" ID
// instead of blocking internally. DISTINCT from object.Task (vim25 SOAP
// Task, generated_task.go, Fase 7) — a different task system at a different
// API layer; not merged with it, per the orchestrator's brief.
//
// Architecture: same vapi/*-over-*rest.Client family as generated_tags.go —
// see generated_esx_settings_cluster_vms.go's top doc comment for the full
// rationale, not repeated here.
//
// mode=vcenter-only: the entire vapi/* domain requires a vCenter Server
// Appliance (VAMI/VAPI session) — see client.REST's doc comment.
//
// vcsim gap, not a bug: confirmed directly that
// referencia/govmomi/vapi/simulator/simulator.go does not import this
// package's simulator sibling (grep -cE
// "vapi/(esx/settings|cluster\"|crypto|cis/tasks)" against that file returns
// 0), even though a standalone vapi/cis/tasks/simulator package DOES exist
// upstream — this project's testhelpers_test.go does not blank-import it, so
// every call from this project's vcsim-backed tests reaches a real,
// unhandled REST endpoint and gets back a genuine HTTP-level error, not a
// wiring bug. Tests use assertReachesServer (generated_vm_lifecycle_test.go)
// for exactly this reason.
//
// Curation: all 4 methods are registered as plain (non-destructive) reads —
// none of them mutate any state; they only observe/poll an already-running
// task. This matches this project's existing vmware_task_wait
// (generated_task.go, the object.Task/SOAP equivalent), which is likewise
// ungated. All 4 are genuinely distinct, not near-duplicates fused into one
// tool the way generated_task.go fused Wait/WaitEx/WaitForResult/
// WaitForResultEx: confirmed by reading waitForState's check functions —
// WaitForCompletion stops at any terminal state (IsDone: not
// Pending/Running), WaitForRunningOrError stops specifically at
// Running-or-Failed (so a task that goes straight to Succeeded without ever
// observing Running blocks until timeout — a real, documented upstream
// limitation, not a bug in this file), and WaitForRunningOrTerminalState
// stops at anything other than Pending (Running, Succeeded, Failed, or
// Blocked) — 3 different termination conditions serving different polling
// needs.
//
// Per the brief, timeout_seconds is REQUIRED (not optional/defaulted) on all
// 4 tools here, including Get (a single non-looping read) — for consistency
// across the file and because even a single HTTP round trip benefits from an
// explicit client-side bound; see requiredWaitTimeout below.
package tools

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/vapi/cis/tasks"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// requiredTimeoutSecondsArg is the JSON schema fragment for an OBLIGATORY
// timeout_seconds argument (no default — the caller must always pass one),
// as opposed to timeoutSecondsArg (generated_vm_lifecycle.go), which
// defaults to defaultWaitTimeout when omitted. Defined here (this file
// concentrates every "wait on a vAPI task" tool in this batch) and reused by
// generated_esx_settings_cluster_vms.go's ApplyWaitForCompletion handler —
// both were called out by the orchestrator's brief as needing a mandatory
// bound; written together in this same batch, so defined once here rather
// than duplicated, same discipline as resolveEntityRef/toMoRefs being shared
// across sibling files elsewhere in this package.
var requiredTimeoutSecondsArg = map[string]interface{}{
	"type":        "integer",
	"description": "Give up after this many seconds and return an error instead of blocking forever. REQUIRED — no default — this call polls a REST endpoint with no server-side timeout of its own.",
}

// requiredWaitTimeout is like waitTimeoutFrom (generated_vm_lifecycle.go)
// but requires timeout_seconds to be present rather than silently falling
// back to defaultWaitTimeout — see requiredTimeoutSecondsArg's doc comment
// for which tools in this batch use this vs. the optional/defaulted
// waitTimeoutFrom.
func requiredWaitTimeout(ctx context.Context, args map[string]interface{}) (context.Context, context.CancelFunc, error) {
	if _, ok := args["timeout_seconds"]; !ok {
		return nil, nil, fmt.Errorf("timeout_seconds is required")
	}
	return waitTimeoutFrom(ctx, args)
}

func registerCisTasksTools(r *Registry) {
	taskIDArg := map[string]interface{}{
		"type":        "string",
		"description": `ID of the vAPI/CIS task (e.g. as returned in a "task_id" field by another vapi/* tool in this server, such as vmware_esx_settings_clusters_vms_apply). NOT a vmware_task_* (object.Task/SOAP) moref — these are two distinct task systems, see this file's top doc comment.`,
	}

	r.register("vmware_cis_tasks_get",
		"Fetch the current status/progress/result of a vAPI/CIS task, without waiting for it to reach any particular state. A single non-looping read.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"task_id":         taskIDArg,
				"timeout_seconds": requiredTimeoutSecondsArg,
			},
			"required": []interface{}{"task_id", "timeout_seconds"},
		},
		Tool{Handler: handleCisTasksGet},
	)

	r.register("vmware_cis_tasks_wait_for_completion",
		"Block until a vAPI/CIS task reaches any terminal state (anything other than PENDING/RUNNING — i.e. SUCCEEDED, FAILED, or BLOCKED) and return its final info. Returns an error if the task's final status is FAILED.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"task_id":         taskIDArg,
				"timeout_seconds": requiredTimeoutSecondsArg,
			},
			"required": []interface{}{"task_id", "timeout_seconds"},
		},
		Tool{Handler: handleCisTasksWaitForCompletion},
	)

	r.register("vmware_cis_tasks_wait_for_running_or_error",
		"Block until a vAPI/CIS task's status becomes RUNNING or FAILED specifically (not SUCCEEDED/BLOCKED/PENDING). Known upstream limitation (not a bug in this tool): a task that transitions straight to SUCCEEDED without ever being observed as RUNNING will block until timeout_seconds expires, since SUCCEEDED satisfies neither condition — prefer vmware_cis_tasks_wait_for_completion unless you specifically need to detect the RUNNING transition.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"task_id":         taskIDArg,
				"timeout_seconds": requiredTimeoutSecondsArg,
			},
			"required": []interface{}{"task_id", "timeout_seconds"},
		},
		Tool{Handler: handleCisTasksWaitForRunningOrError},
	)

	r.register("vmware_cis_tasks_wait_for_running_or_terminal_state",
		"Block until a vAPI/CIS task leaves the PENDING state — i.e. its status becomes RUNNING, SUCCEEDED, FAILED, or BLOCKED. Returns an error if the task's final observed status is FAILED.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"task_id":         taskIDArg,
				"timeout_seconds": requiredTimeoutSecondsArg,
			},
			"required": []interface{}{"task_id", "timeout_seconds"},
		},
		Tool{Handler: handleCisTasksWaitForRunningOrTerminalState},
	)
}

// cisTasksManager returns a tasks.Manager bound to client's VAPI/REST
// session, logging in lazily via client.REST — same pattern as
// generated_tags.go's tagsManager.
func cisTasksManager(ctx context.Context, client *vmware.Client) (*tasks.Manager, error) {
	rc, err := client.REST(ctx)
	if err != nil {
		return nil, err
	}
	return tasks.NewManager(rc), nil
}

func handleCisTasksGet(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := cisTasksManager(ctx, client)
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

	info, err := m.Get(waitCtx, taskID)
	if err != nil {
		return "", fmt.Errorf("failed to get task %s: %w", taskID, err)
	}
	return marshalJSON(map[string]interface{}{"task_id": taskID, "info": info})
}

func handleCisTasksWaitForCompletion(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := cisTasksManager(ctx, client)
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

	info, err := m.WaitForCompletion(waitCtx, taskID)
	if err != nil {
		return "", fmt.Errorf("failed waiting for task %s to complete: %w", taskID, err)
	}
	return marshalJSON(map[string]interface{}{"task_id": taskID, "info": info})
}

func handleCisTasksWaitForRunningOrError(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := cisTasksManager(ctx, client)
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

	info, err := m.WaitForRunningOrError(waitCtx, taskID)
	if err != nil {
		return "", fmt.Errorf("failed waiting for task %s to be running/error: %w", taskID, err)
	}
	return marshalJSON(map[string]interface{}{"task_id": taskID, "info": info})
}

func handleCisTasksWaitForRunningOrTerminalState(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := cisTasksManager(ctx, client)
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

	info, err := m.WaitForRunningOrTerminalState(waitCtx, taskID)
	if err != nil {
		return "", fmt.Errorf("failed waiting for task %s to leave pending: %w", taskID, err)
	}
	return marshalJSON(map[string]interface{}{"task_id": taskID, "info": info})
}

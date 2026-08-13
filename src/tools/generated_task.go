package tools

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// registerTaskTools is the "task" slice of Fase 7 (Grupo A) of the codegen
// plan (".workspace/plans/MCPVMWare2026-08-10-175300-plano-cobertura-completa-api-codegen.plan.md")
// — object.Task (referencia/govmomi/object/task.go), hand-transcribed
// following the vm.go/generated_vm_lifecycle.go conventions.
//
// Honest scoping note (read before wiring a caller against these tools):
// today NOTHING ELSE in this server hands a caller a "task" moref to feed
// into these 5 tools — every other tool in this project that triggers a
// *object.Task (PowerOffVM_Task, Destroy_Task, UpgradeVM_Task, ...) already
// waits for it internally via vm.go's waitForTask before ever returning to
// the caller (this project's deliberate synchronous design — see
// waitForTask's doc comment; no async/polling tool exists anywhere in this
// server). These 5 tools exist for API-surface completeness (real vSphere
// supports all 5 regardless of what this server's other tools do) and
// become useful the moment a caller discovers a task moref some other way —
// e.g. a future "recent tasks" tool over TaskManager.RecentTask (out of
// scope for this group; TaskManager.CreateTask/RecentTask are not part of
// object/task.go, which only wraps an already-existing task) or a task
// moref surfaced by vCenter's own UI/API outside this server entirely.
// generated_task_test.go proves all 5 for real against vcsim anyway, using
// simulator.TaskDelay to keep a real PowerOffVM_Task in the "running" state
// long enough to Cancel/SetState/SetDescription/UpdateProgress it before it
// completes on its own — not a registration-only smoke test.
//
// Curation:
//
//   - Wait/WaitEx/WaitForResult/WaitForResultEx are 4 variants of the exact
//     same observable behavior over MCP's JSON-RPC boundary (block until the
//     task finishes, return its final types.TaskInfo) — they differ only in
//     an internal implementation detail (WaitForResult/Wait create a
//     thread-safe-but-more-expensive PropertyCollector per call; WaitEx/
//     WaitForResultEx reuse a shared one) and in accepting an optional
//     progress.Sinker (a Go channel — no JSON-RPC representation). Collapsed
//     into one tool, vmware_task_wait, which calls WaitForResult(ctx)
//     internally — the same one vm.go's existing waitForTask already calls,
//     for consistency within this codebase (an arbitrary pick between the
//     two functionally-equivalent pairs).
//
//   - "task" is represented as a bare moref Value string (e.g. "task-123"),
//     Type always fixed to "Task" server-side — the same "accept the raw
//     moref Value, Type is implied by context" pattern already used by
//     vmware_vm_query_changed_disk_areas's base_snapshot/cur_snapshot
//     arguments in generated_vm_lifecycle.go. Tasks have no inventory path
//     (they are not part of the datacenter folder hierarchy the way a VM/
//     host/datastore is), so there is no name/pattern form to resolve via a
//     Finder here, unlike vm/host/datastore arguments elsewhere in this
//     project.
//
//   - vmware_task_set_state's optional "fault" is deliberately narrowed vs.
//     the real API: types.LocalizedMethodFault.Fault is typed
//     BaseMethodFault, a polymorphic interface with dozens of concrete
//     fault subtypes (InvalidState, RequestCanceled, ...) — there is no
//     generic "decode arbitrary JSON into the right concrete fault subtype"
//     mapping available (unlike decodeJSONArg's use elsewhere in this
//     project, which always targets one already-known concrete struct).
//     This tool only accepts a plain "fault_message" string, wrapped as
//     &types.LocalizedMethodFault{Fault: &types.MethodFault{}, LocalizedMessage: msg}
//     — types.MethodFault is the base struct every fault subtype embeds,
//     and it alone satisfies BaseMethodFault via its own GetMethodFault()
//     method (referencia/govmomi/vim25/types/if.go's
//     "func (b *MethodFault) GetMethodFault() *MethodFault { return b }"),
//     so this is a real, valid (if generic/subtype-less) fault value, not a
//     fabricated one. SetTaskState itself is a rarely-needed, low-level API
//     (documented for extension/solution developers managing their own
//     custom tasks, not something a caller would typically use against a
//     built-in vCenter/ESXi task) — narrowing this one argument was judged
//     not worth a hand-built ~25-subtype fault schema.
func registerTaskTools(r *Registry) {
	taskArg := map[string]interface{}{
		"type":        "string",
		"description": `Managed object reference VALUE of the task (e.g. "task-123" — NOT "Task:task-123"; the type is always "Task"). This server has no tool today that hands you one directly (see this file's top doc comment) — obtain it from another source (e.g. vCenter's own API/UI) that surfaces a live task moref.`,
	}
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}

	r.register("vmware_task_wait",
		"Block until a task finishes (successfully or with an error) and return its final info (state, result, error, progress). Fuses object.Task's Wait/WaitEx/WaitForResult/WaitForResultEx into one tool — see this file's top doc comment for why. Bounded by timeout_seconds (default 300).",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"task":            taskArg,
				"timeout_seconds": timeoutSecondsArg,
			},
			"required": []interface{}{"task"},
		},
		Tool{Handler: handleTaskWait},
	)

	r.registerDestructive("vmware_task_cancel",
		"Request cancellation of a running task. Best-effort: vSphere/vcsim accepts the cancel request and marks the task cancelled, but does not necessarily interrupt work already in flight (a task that is nearly done may still complete normally). Fails if the task has already completed.",
		tier2,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"task": taskArg, "confirm": confirmArg},
			"required":   []interface{}{"task", "confirm"},
		},
		Tool{Handler: handleTaskCancel},
	)

	r.registerDestructive("vmware_task_set_description",
		"Update a task's description (the localized message describing its current phase). Fails if the task has already completed.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"task":        taskArg,
				"description": map[string]interface{}{"type": "object", "description": `A types.LocalizableMessage JSON object matching its Go struct fields (e.g. {"key": "com.example.phase1", "message": "Phase 1"}).`},
				"confirm":     confirmArg,
			},
			"required": []interface{}{"task", "description", "confirm"},
		},
		Tool{Handler: handleTaskSetDescription},
	)

	r.registerDestructive("vmware_task_set_state",
		`Directly set a task's state (queued/running/success/error), optionally with a result value and/or a fault message. Fails if the task has already completed — see this file's top doc comment for "fault_message"'s narrowed representation vs. the real API's polymorphic fault type.`,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"task":          taskArg,
				"state":         map[string]interface{}{"type": "string", "enum": []interface{}{"queued", "running", "success", "error"}, "description": "New task state."},
				"result":        map[string]interface{}{"description": "Arbitrary JSON value to record as the task's result. Optional."},
				"fault_message": map[string]interface{}{"type": "string", "description": `Localized message for a generic fault. Only meaningful when state is "error". Optional.`},
				"confirm":       confirmArg,
			},
			"required": []interface{}{"task", "state", "confirm"},
		},
		Tool{Handler: handleTaskSetState},
	)

	r.registerDestructive("vmware_task_update_progress",
		"Set a task's percentage-done value (0-100; out-of-range values are clamped server-side). Fails unless the task is currently running.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"task":         taskArg,
				"percent_done": map[string]interface{}{"type": "integer", "description": "0-100."},
				"confirm":      confirmArg,
			},
			"required": []interface{}{"task", "percent_done", "confirm"},
		},
		Tool{Handler: handleTaskUpdateProgress},
	)
}

// resolveTaskArg builds a client-side *object.Task wrapper around the
// "task" argument's bare moref Value — a pure local construction with no
// network round trip (the same pattern as e.g.
// object.NewDatastoreNamespaceManager elsewhere in this project); the round
// trip happens on the first real method call made against it. Also returns
// the raw string value for use in JSON responses/error messages.
func resolveTaskArg(client *vmware.Client, args map[string]interface{}) (*object.Task, string, error) {
	val, _ := args["task"].(string)
	if val == "" {
		return nil, "", fmt.Errorf("task is required")
	}
	ref := types.ManagedObjectReference{Type: "Task", Value: val}
	return object.NewTask(client.Client.Client, ref), val, nil
}

func handleTaskWait(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	t, val, err := resolveTaskArg(client, args)
	if err != nil {
		return "", err
	}

	waitCtx, cancel, err := waitTimeoutFrom(ctx, args)
	if err != nil {
		return "", err
	}
	defer cancel()

	info, err := t.WaitForResult(waitCtx)
	if err != nil {
		return "", fmt.Errorf("failed waiting for task %s: %w", val, err)
	}

	return marshalJSON(map[string]interface{}{"task": val, "info": info})
}

func handleTaskCancel(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	t, val, err := resolveTaskArg(client, args)
	if err != nil {
		return "", err
	}

	if err := t.Cancel(ctx); err != nil {
		return "", fmt.Errorf("failed to cancel task %s: %w", val, err)
	}

	return marshalJSON(map[string]interface{}{"task": val, "result": "cancel_requested"})
}

func handleTaskSetDescription(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	t, val, err := resolveTaskArg(client, args)
	if err != nil {
		return "", err
	}

	raw, ok := args["description"]
	if !ok {
		return "", fmt.Errorf("description is required")
	}
	var desc types.LocalizableMessage
	if err := decodeJSONArg(raw, &desc); err != nil {
		return "", fmt.Errorf("invalid description: %w", err)
	}

	if err := t.SetDescription(ctx, desc); err != nil {
		return "", fmt.Errorf("failed to set description on task %s: %w", val, err)
	}

	return marshalJSON(map[string]interface{}{"task": val, "result": "description_set"})
}

func handleTaskSetState(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	t, val, err := resolveTaskArg(client, args)
	if err != nil {
		return "", err
	}

	stateStr, _ := args["state"].(string)
	switch types.TaskInfoState(stateStr) {
	case types.TaskInfoStateQueued, types.TaskInfoStateRunning, types.TaskInfoStateSuccess, types.TaskInfoStateError:
	default:
		return "", fmt.Errorf(`state must be one of "queued", "running", "success", "error", got %q`, stateStr)
	}

	var result types.AnyType
	if v, ok := args["result"]; ok {
		result = v
	}

	var fault *types.LocalizedMethodFault
	if msg, ok := args["fault_message"].(string); ok && msg != "" {
		fault = &types.LocalizedMethodFault{Fault: &types.MethodFault{}, LocalizedMessage: msg}
	}

	if err := t.SetState(ctx, types.TaskInfoState(stateStr), result, fault); err != nil {
		return "", fmt.Errorf("failed to set state on task %s: %w", val, err)
	}

	return marshalJSON(map[string]interface{}{"task": val, "state": stateStr, "result": "state_set"})
}

func handleTaskUpdateProgress(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	t, val, err := resolveTaskArg(client, args)
	if err != nil {
		return "", err
	}

	raw, ok := args["percent_done"]
	if !ok {
		return "", fmt.Errorf("percent_done is required")
	}
	n, err := toInt32(raw)
	if err != nil {
		return "", fmt.Errorf("invalid percent_done: %w", err)
	}

	if err := t.UpdateProgress(ctx, int(n)); err != nil {
		return "", fmt.Errorf("failed to update progress on task %s: %w", val, err)
	}

	return marshalJSON(map[string]interface{}{"task": val, "percent_done": n, "result": "progress_updated"})
}

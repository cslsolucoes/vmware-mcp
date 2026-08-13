package tools

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/cslsoftwares/mcpvmware/workstation"
)

// registerWorkstationVMTools/registerWorkstationPowerTools are Fase 9's
// "Grupo A" slice: the "VM Management" (9 ops) + "VM Power Management" (2
// ops) folders of `.workspace/VMware Workstation Pro API.postman_collection.json`
// — VMware Workstation Pro's vmrest REST service, a product distinct from
// vSphere/ESXi with no govmomi dependency (see workstation/client.go).
//
// Prerequisite for every tool in this file: vmrest only listens locally —
// Workstation Pro has no remote/vCenter-style management plane. If this MCP
// server runs on a different machine than the one with Workstation Pro
// installed and vmrest started, none of these tools have a reachable target.
// Repeated in each tool's description via requiresVmrestLocal below.
//
// Response decoding: every handler below decodes into a generic interface{}
// and re-marshals it, instead of a hand-typed struct — same posture as
// tools/appliance.go's applianceGet for VAMI. Reason it matters here
// specifically: this project already caught vmrest's live implementation
// disagreeing with its own spec's casing (workstation/client.go's errorModel
// comment — real responses use "Code"/"Message", the spec's schema says
// lowercase "code"/"message"), confirmed against a live vmrest 1.3.1
// instance, not guessed. With no simulator to verify success-response field
// names against either, a typed struct risks silently dropping/misnaming
// fields the same way; a generic decode is more honest.
//
// Curation deviations from the raw spec (each is a case where the spec
// marks a body field "not required" but leaving it out makes the tool call
// a pointless no-op — tightened here, confirmed by re-reading the vendored
// collection's own request/response tables, not guessed):
//   - vmware_workstation_vm_config_param_set: spec lists both `name` and
//     `value` of ConfigVMParamsParameter as optional; this tool requires
//     both, since a config-param-set call with neither is meaningless.
//   - vmware_workstation_vm_register: spec lists both `name` and `path` of
//     VMRegisterParameter as optional; this tool requires `path` (vmrest
//     needs the .vmx path to know what to register at all) and leaves
//     `name` optional.
//   - vmware_workstation_vm_update: spec lists both `processors` and
//     `memory` of VMParameter as optional; this tool requires at least one
//     of the two to be present (an empty settings update is a no-op).
//
// Argument naming: the VM identifier argument is named "id" (not "vm" as
// vSphere-domain tools in this project use) because it matches vmrest's own
// path parameter name and is a different kind of identifier entirely (a
// vmrest-assigned hex ID / .vmx path, not a vim25 MoRef) — using "vm" here
// would incorrectly imply the two are interchangeable. The optional query
// parameter vmrest calls "vmPassword" is exposed as the snake_case
// "vm_password" per this project's schema-naming convention (see
// generated_vm_lifecycle.go's timeout_seconds/disk_key), translated back to
// "vmPassword" only when building the actual request URL.
const requiresVmrestLocal = " Requires VMware Workstation Pro's vmrest service running locally on the same machine as this MCP server — Workstation Pro has no remote/vCenter-style management plane. If this server runs on a different machine than the one with Workstation Pro installed and vmrest started, this tool has no reachable target."

// wsIDArg/wsVMPasswordArg are the two schema fragments shared by nearly
// every tool in this file.
var (
	wsIDArg = map[string]interface{}{
		"type":        "string",
		"description": `VM ID as returned by vmware_workstation_vm_list or vmware_workstation_vm_register (a vmrest-assigned identifier, e.g. a 32-character hex string) — not a vSphere/vim25 managed object reference.`,
	}
	wsVMPasswordArg = map[string]interface{}{
		"type":        "string",
		"description": "Password for an encrypted VM. Optional — omit for a non-encrypted VM; for power operations, also unnecessary once the VM is already powered on.",
	}
)

// wsAppendVMPassword appends the optional vmPassword query parameter vmrest
// accepts on every route in this file except POST /vms/registration (no
// such parameter in the spec for that one route) — confirmed reading the
// vendored Postman collection's per-request parameter tables.
func wsAppendVMPassword(path string, args map[string]interface{}) string {
	pw, _ := args["vm_password"].(string)
	if pw == "" {
		return path
	}
	v := url.Values{}
	v.Set("vmPassword", pw)
	return path + "?" + v.Encode()
}

// registerWorkstationVMTools registers the 9 "VM Management" tools.
func registerWorkstationVMTools(r *Registry) {
	emptySchema := map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}

	r.registerWorkstation("vmware_workstation_vm_list",
		"List every VM known to this VMware Workstation Pro instance: vmrest-assigned ID and .vmx path (GET /vms)."+requiresVmrestLocal,
		emptySchema,
		Tool{WSHandler: handleWorkstationVMList},
	)

	r.registerDestructiveWorkstation("vmware_workstation_vm_clone",
		"Create a full clone of an existing VM (POST /vms)."+requiresVmrestLocal,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name":        map[string]interface{}{"type": "string", "description": "Name for the new (cloned) VM."},
				"parent_id":   map[string]interface{}{"type": "string", "description": "ID of the existing VM to clone from (see vmware_workstation_vm_list)."},
				"vm_password": wsVMPasswordArg,
				"confirm":     confirmArg,
			},
			"required": []interface{}{"name", "parent_id", "confirm"},
		},
		Tool{WSHandler: handleWorkstationVMClone},
	)

	r.registerWorkstation("vmware_workstation_vm_get",
		"Get a VM's setting information — ID, processor count, memory (GET /vms/{id})."+requiresVmrestLocal,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"id":          wsIDArg,
				"vm_password": wsVMPasswordArg,
			},
			"required": []interface{}{"id"},
		},
		Tool{WSHandler: handleWorkstationVMGet},
	)

	r.registerDestructiveWorkstation("vmware_workstation_vm_update",
		"Update a VM's processor count and/or memory size (PUT /vms/{id}). At least one of processors/memory must be provided."+requiresVmrestLocal,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"id":          wsIDArg,
				"processors":  map[string]interface{}{"type": "integer", "description": "New virtual processor count. Optional."},
				"memory":      map[string]interface{}{"type": "integer", "description": "New memory size in MB. Optional."},
				"vm_password": wsVMPasswordArg,
				"confirm":     confirmArg,
			},
			"required": []interface{}{"id", "confirm"},
		},
		Tool{WSHandler: handleWorkstationVMUpdate},
	)

	r.registerDestructiveWorkstation("vmware_workstation_vm_delete",
		"Delete a VM — removes it from disk. Irreversible (DELETE /vms/{id})."+requiresVmrestLocal,
		tier1,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"id":          wsIDArg,
				"vm_password": wsVMPasswordArg,
				"confirm":     confirmArg,
			},
			"required": []interface{}{"id", "confirm"},
		},
		Tool{WSHandler: handleWorkstationVMDelete},
	)

	r.registerDestructiveWorkstation("vmware_workstation_vm_config_param_set",
		"Set one .vmx configuration parameter on a VM by name/value (PUT /vms/{id}/configparams)."+requiresVmrestLocal,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"id":          wsIDArg,
				"name":        map[string]interface{}{"type": "string", "description": "Config parameter name."},
				"value":       map[string]interface{}{"type": "string", "description": "Config parameter value."},
				"vm_password": wsVMPasswordArg,
				"confirm":     confirmArg,
			},
			"required": []interface{}{"id", "name", "value", "confirm"},
		},
		Tool{WSHandler: handleWorkstationVMConfigParamSet},
	)

	r.registerWorkstation("vmware_workstation_vm_config_param_get",
		"Get one .vmx configuration parameter's value by name (GET /vms/{id}/params/{name})."+requiresVmrestLocal,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"id":          wsIDArg,
				"name":        map[string]interface{}{"type": "string", "description": "Config parameter name to read."},
				"vm_password": wsVMPasswordArg,
			},
			"required": []interface{}{"id", "name"},
		},
		Tool{WSHandler: handleWorkstationVMConfigParamGet},
	)

	r.registerWorkstation("vmware_workstation_vm_restrictions",
		"Get a VM's restrictions information — devices (CD/DVD, floppy, NIC, USB, parallel/serial ports), guest isolation flags, remote VNC settings, and more (GET /vms/{id}/restrictions)."+requiresVmrestLocal,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"id":          wsIDArg,
				"vm_password": wsVMPasswordArg,
			},
			"required": []interface{}{"id"},
		},
		Tool{WSHandler: handleWorkstationVMRestrictions},
	)

	r.registerDestructiveWorkstation("vmware_workstation_vm_register",
		"Register an existing VM (a .vmx file already on disk) into Workstation Pro's VM library (POST /vms/registration)."+requiresVmrestLocal,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name":    map[string]interface{}{"type": "string", "description": "Display name to register the VM under. Optional — vmrest defaults it from the .vmx if omitted."},
				"path":    map[string]interface{}{"type": "string", "description": `Path to the VM's .vmx file on the local filesystem, e.g. "C:\\VMs\\MyVM\\MyVM.vmx".`},
				"confirm": confirmArg,
			},
			"required": []interface{}{"path", "confirm"},
		},
		Tool{WSHandler: handleWorkstationVMRegister},
	)
}

// registerWorkstationPowerTools registers the 2 "VM Power Management"
// tools.
func registerWorkstationPowerTools(r *Registry) {
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}

	r.registerWorkstation("vmware_workstation_vm_power_get",
		"Get a VM's current power state (GET /vms/{id}/power)."+requiresVmrestLocal,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"id":          wsIDArg,
				"vm_password": wsVMPasswordArg,
			},
			"required": []interface{}{"id"},
		},
		Tool{WSHandler: handleWorkstationVMPowerGet},
	)

	r.registerDestructiveWorkstation("vmware_workstation_vm_power_set",
		`Change a VM's power state (PUT /vms/{id}/power). The off/suspend/pause operations are disruptive to the target VM — confirm the target before sending.`+requiresVmrestLocal,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"id":          wsIDArg,
				"operation":   map[string]interface{}{"type": "string", "enum": []interface{}{"on", "off", "shutdown", "suspend", "pause", "unpause"}, "description": "Power operation to apply."},
				"vm_password": wsVMPasswordArg,
				"confirm":     confirmArg,
			},
			"required": []interface{}{"id", "operation", "confirm"},
		},
		Tool{WSHandler: handleWorkstationVMPowerSet},
	)
}

// --- VM Management handlers -------------------------------------------------

func handleWorkstationVMList(ctx context.Context, client *workstation.Client, args map[string]interface{}) (string, error) {
	var out interface{}
	if err := client.Do(ctx, http.MethodGet, "/vms", nil, &out); err != nil {
		return "", fmt.Errorf("failed to list VMs: %w", err)
	}
	return marshalJSON(out)
}

func handleWorkstationVMClone(ctx context.Context, client *workstation.Client, args map[string]interface{}) (string, error) {
	name, _ := args["name"].(string)
	if name == "" {
		return "", fmt.Errorf("name is required")
	}
	parentID, _ := args["parent_id"].(string)
	if parentID == "" {
		return "", fmt.Errorf("parent_id is required")
	}

	path := wsAppendVMPassword("/vms", args)
	body := map[string]interface{}{"name": name, "parentId": parentID}
	var out interface{}
	if err := client.Do(ctx, http.MethodPost, path, body, &out); err != nil {
		return "", fmt.Errorf("failed to clone VM %q from parent %q: %w", name, parentID, err)
	}
	return marshalJSON(out)
}

func handleWorkstationVMGet(ctx context.Context, client *workstation.Client, args map[string]interface{}) (string, error) {
	id, _ := args["id"].(string)
	if id == "" {
		return "", fmt.Errorf("id is required")
	}

	path := wsAppendVMPassword("/vms/"+id, args)
	var out interface{}
	if err := client.Do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return "", fmt.Errorf("failed to get VM %q: %w", id, err)
	}
	return marshalJSON(out)
}

func handleWorkstationVMUpdate(ctx context.Context, client *workstation.Client, args map[string]interface{}) (string, error) {
	id, _ := args["id"].(string)
	if id == "" {
		return "", fmt.Errorf("id is required")
	}

	body := map[string]interface{}{}
	if v, ok := args["processors"]; ok && v != nil {
		n, err := toInt32(v)
		if err != nil {
			return "", fmt.Errorf("invalid processors: %w", err)
		}
		body["processors"] = n
	}
	if v, ok := args["memory"]; ok && v != nil {
		n, err := toInt32(v)
		if err != nil {
			return "", fmt.Errorf("invalid memory: %w", err)
		}
		body["memory"] = n
	}
	if len(body) == 0 {
		return "", fmt.Errorf("at least one of processors/memory is required")
	}

	path := wsAppendVMPassword("/vms/"+id, args)
	var out interface{}
	if err := client.Do(ctx, http.MethodPut, path, body, &out); err != nil {
		return "", fmt.Errorf("failed to update VM %q: %w", id, err)
	}
	return marshalJSON(out)
}

func handleWorkstationVMDelete(ctx context.Context, client *workstation.Client, args map[string]interface{}) (string, error) {
	id, _ := args["id"].(string)
	if id == "" {
		return "", fmt.Errorf("id is required")
	}

	path := wsAppendVMPassword("/vms/"+id, args)
	if err := client.Do(ctx, http.MethodDelete, path, nil, nil); err != nil {
		return "", fmt.Errorf("failed to delete VM %q: %w", id, err)
	}
	// 204 No Content on success — vmrest returns no body, so report our own
	// confirmation rather than marshaling nothing.
	return marshalJSON(map[string]interface{}{"id": id, "deleted": true})
}

func handleWorkstationVMConfigParamSet(ctx context.Context, client *workstation.Client, args map[string]interface{}) (string, error) {
	id, _ := args["id"].(string)
	if id == "" {
		return "", fmt.Errorf("id is required")
	}
	name, _ := args["name"].(string)
	if name == "" {
		return "", fmt.Errorf("name is required")
	}
	value, _ := args["value"].(string)
	if value == "" {
		return "", fmt.Errorf("value is required")
	}

	path := wsAppendVMPassword("/vms/"+id+"/configparams", args)
	body := map[string]interface{}{"name": name, "value": value}
	var out interface{}
	if err := client.Do(ctx, http.MethodPut, path, body, &out); err != nil {
		return "", fmt.Errorf("failed to set config param %q on VM %q: %w", name, id, err)
	}
	// The vendored spec's 200 response schema for this route is a copy-paste
	// of ErrorModel (a spec authoring bug, not a real error shape) and vmrest
	// returns no useful body on success — fall back to our own confirmation
	// when the decode came back empty.
	if out == nil {
		return marshalJSON(map[string]interface{}{"id": id, "name": name, "value": value, "updated": true})
	}
	return marshalJSON(out)
}

func handleWorkstationVMConfigParamGet(ctx context.Context, client *workstation.Client, args map[string]interface{}) (string, error) {
	id, _ := args["id"].(string)
	if id == "" {
		return "", fmt.Errorf("id is required")
	}
	name, _ := args["name"].(string)
	if name == "" {
		return "", fmt.Errorf("name is required")
	}

	path := wsAppendVMPassword("/vms/"+id+"/params/"+url.PathEscape(name), args)
	var out interface{}
	if err := client.Do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return "", fmt.Errorf("failed to get config param %q on VM %q: %w", name, id, err)
	}
	return marshalJSON(out)
}

func handleWorkstationVMRestrictions(ctx context.Context, client *workstation.Client, args map[string]interface{}) (string, error) {
	id, _ := args["id"].(string)
	if id == "" {
		return "", fmt.Errorf("id is required")
	}

	path := wsAppendVMPassword("/vms/"+id+"/restrictions", args)
	var out interface{}
	if err := client.Do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return "", fmt.Errorf("failed to get restrictions for VM %q: %w", id, err)
	}
	return marshalJSON(out)
}

func handleWorkstationVMRegister(ctx context.Context, client *workstation.Client, args map[string]interface{}) (string, error) {
	path, _ := args["path"].(string)
	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	name, _ := args["name"].(string)

	body := map[string]interface{}{"path": path}
	if name != "" {
		body["name"] = name
	}
	var out interface{}
	if err := client.Do(ctx, http.MethodPost, "/vms/registration", body, &out); err != nil {
		return "", fmt.Errorf("failed to register VM at %q: %w", path, err)
	}
	return marshalJSON(out)
}

// --- VM Power Management handlers -------------------------------------------

func handleWorkstationVMPowerGet(ctx context.Context, client *workstation.Client, args map[string]interface{}) (string, error) {
	id, _ := args["id"].(string)
	if id == "" {
		return "", fmt.Errorf("id is required")
	}

	path := wsAppendVMPassword("/vms/"+id+"/power", args)
	var out interface{}
	if err := client.Do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return "", fmt.Errorf("failed to get power state for VM %q: %w", id, err)
	}
	return marshalJSON(out)
}

// wsPowerOperations is the exact, confirmed set of values vmrest's
// VMPowerOperation body accepts — read from the vendored Postman
// collection's request description, not guessed.
var wsPowerOperations = map[string]bool{
	"on": true, "off": true, "shutdown": true, "suspend": true, "pause": true, "unpause": true,
}

func handleWorkstationVMPowerSet(ctx context.Context, client *workstation.Client, args map[string]interface{}) (string, error) {
	id, _ := args["id"].(string)
	if id == "" {
		return "", fmt.Errorf("id is required")
	}
	operation, _ := args["operation"].(string)
	if operation == "" {
		return "", fmt.Errorf("operation is required")
	}
	if !wsPowerOperations[operation] {
		return "", fmt.Errorf("invalid operation %q: must be one of on, off, shutdown, suspend, pause, unpause", operation)
	}

	path := wsAppendVMPassword("/vms/"+id+"/power", args)
	var out interface{}
	if err := client.DoRawBody(ctx, http.MethodPut, path, []byte(operation), &out); err != nil {
		return "", fmt.Errorf("failed to set power state %q on VM %q: %w", operation, id, err)
	}
	return marshalJSON(out)
}

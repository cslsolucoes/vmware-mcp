// Fase 9 "Grupo B" — VM Shared Folders + VM Network Adapters + Host Networks
// Management for VMware Workstation Pro's vmrest service. Routes confirmed
// by parsing `.workspace/VMware Workstation Pro API.postman_collection.json`
// directly (Python json.load walk over the 3 folders below) rather than
// trusting the plan's headline counts: "VM Shared Folders Management" (4),
// "VM Network Adapters Management" (6), "Host Networks Management" (7) — the
// plan's prose said "8" for the last folder, but the collection itself only
// has 7 request items under it (list vmnet, list/update mactoip, list/
// update/delete portforward, create vmnets) — 17 tools total, not 18.
package tools

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/cslsoftwares/mcpvmware/workstation"
)

// wsRunningNote is appended to every tool description in this file — vmrest
// is a local-only service (no remote/cloud form, confirmed in
// workstation/client.go's package doc comment), so these tools are useless
// unless vmrest is actually running on the same machine as mcpvmware-mcp.exe.
const wsRunningNote = " Requires VMware Workstation Pro's vmrest service to be running locally on the machine where mcpvmware-mcp.exe executes — if the MCP server runs on a different machine than the one with Workstation Pro installed, this tool has no target to reach."

// wsGet issues a GET against path and marshals whatever vmrest returned
// (array or object — no simulator/live-verified typed struct exists for
// these responses, so a generic decode is honest rather than a guessed
// struct that could silently drop fields, same posture as
// appliance.go's applianceGet).
func wsGet(ctx context.Context, client *workstation.Client, path string) (string, error) {
	var out interface{}
	if err := client.Do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return "", err
	}
	return marshalJSON(out)
}

// wsMutate issues method (POST/PUT/DELETE) against path with body (nil for
// DELETE) and marshals the response. Several vmrest mutation endpoints
// return no body at all (204/empty) — wsMutate reports a plain {"result":
// "ok"} in that case instead of an empty string, so every tool in this file
// always returns valid, non-empty JSON.
func wsMutate(ctx context.Context, client *workstation.Client, method, path string, body interface{}) (string, error) {
	var out interface{}
	if err := client.Do(ctx, method, path, body, &out); err != nil {
		return "", err
	}
	if out == nil {
		return marshalJSON(map[string]interface{}{"result": "ok"})
	}
	return marshalJSON(out)
}

// wsRequiredInt reads args[key] as a required integer (float64 from real
// JSON-RPC, or a Go int literal from tests — see vm.go's toInt64 for why
// both forms are accepted). Error wording mirrors requiredStringArg's "%s is
// required" (defined in generated_library_core.go, reused as-is by every
// handler below that takes a bare string ID).
func wsRequiredInt(args map[string]interface{}, key string) (int, error) {
	v, ok := args[key]
	if !ok {
		return 0, fmt.Errorf("%s is required", key)
	}
	n, err := toInt64(v)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", key, err)
	}
	return int(n), nil
}

// --- VM Shared Folders Management (4 tools) --------------------------------

func registerWorkstationSharedFolderTools(r *Registry) {
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}
	idArg := map[string]interface{}{"type": "string", "description": "VM identifier, as returned by vmware_workstation_vm_list."}
	folderIDArg := map[string]interface{}{"type": "string", "description": "Shared folder identifier (folder_id), as returned by vmware_workstation_shared_folder_list."}

	r.registerWorkstation("vmware_workstation_shared_folder_list",
		"List shared folders mounted in a VMware Workstation Pro VM (GET /vms/{id}/sharedfolders)."+wsRunningNote,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"id": idArg},
			"required":   []interface{}{"id"},
		},
		Tool{WSHandler: handleWSSharedFolderList},
	)

	r.registerDestructiveWorkstation("vmware_workstation_shared_folder_create",
		"Mount a new shared folder in a VMware Workstation Pro VM (POST /vms/{id}/sharedfolders). Disruptive but reversible — the mounted folder can be removed again with vmware_workstation_shared_folder_delete."+wsRunningNote,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"id":        idArg,
				"folder_id": map[string]interface{}{"type": "string", "description": "New shared folder's identifier — this is the name it will appear under inside the guest, not a path."},
				"host_path": map[string]interface{}{"type": "string", "description": `Absolute path on the HOST filesystem to share, e.g. "C:\\SharedFolders\\demo".`},
				"flags":     map[string]interface{}{"type": "integer", "description": "Bitmask of vmrest shared-folder flags (e.g. 4 = read/write enabled, 0 = read-only). See VMware Workstation Pro's vmrest API documentation for the full bit meanings."},
				"confirm":   confirmArg,
			},
			"required": []interface{}{"id", "folder_id", "host_path", "flags", "confirm"},
		},
		Tool{WSHandler: handleWSSharedFolderCreate},
	)

	r.registerDestructiveWorkstation("vmware_workstation_shared_folder_update",
		"Update a shared folder already mounted in a VMware Workstation Pro VM — host path and/or flags (PUT /vms/{id}/sharedfolders/{folderId})."+wsRunningNote,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"id":        idArg,
				"folder_id": folderIDArg,
				"host_path": map[string]interface{}{"type": "string", "description": `New absolute HOST path to share, e.g. "C:\\SharedFolders\\demo".`},
				"flags":     map[string]interface{}{"type": "integer", "description": "New bitmask of vmrest shared-folder flags."},
				"confirm":   confirmArg,
			},
			"required": []interface{}{"id", "folder_id", "host_path", "flags", "confirm"},
		},
		Tool{WSHandler: handleWSSharedFolderUpdate},
	)

	r.registerDestructiveWorkstation("vmware_workstation_shared_folder_delete",
		"Unmount/remove a shared folder from a VMware Workstation Pro VM (DELETE /vms/{id}/sharedfolders/{folderId}). Reversible — the same folder can be remounted with vmware_workstation_shared_folder_create."+wsRunningNote,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"id":        idArg,
				"folder_id": folderIDArg,
				"confirm":   confirmArg,
			},
			"required": []interface{}{"id", "folder_id", "confirm"},
		},
		Tool{WSHandler: handleWSSharedFolderDelete},
	)
}

func handleWSSharedFolderList(ctx context.Context, client *workstation.Client, args map[string]interface{}) (string, error) {
	id, err := requiredStringArg(args, "id")
	if err != nil {
		return "", err
	}
	return wsGet(ctx, client, "/vms/"+id+"/sharedfolders")
}

type wsSharedFolderCreateBody struct {
	FolderID string `json:"folder_id"`
	HostPath string `json:"host_path"`
	Flags    int    `json:"flags"`
}

func handleWSSharedFolderCreate(ctx context.Context, client *workstation.Client, args map[string]interface{}) (string, error) {
	id, err := requiredStringArg(args, "id")
	if err != nil {
		return "", err
	}
	folderID, err := requiredStringArg(args, "folder_id")
	if err != nil {
		return "", err
	}
	hostPath, err := requiredStringArg(args, "host_path")
	if err != nil {
		return "", err
	}
	flags, err := wsRequiredInt(args, "flags")
	if err != nil {
		return "", err
	}
	body := wsSharedFolderCreateBody{FolderID: folderID, HostPath: hostPath, Flags: flags}
	return wsMutate(ctx, client, http.MethodPost, "/vms/"+id+"/sharedfolders", body)
}

type wsSharedFolderUpdateBody struct {
	HostPath string `json:"host_path"`
	Flags    int    `json:"flags"`
}

func handleWSSharedFolderUpdate(ctx context.Context, client *workstation.Client, args map[string]interface{}) (string, error) {
	id, err := requiredStringArg(args, "id")
	if err != nil {
		return "", err
	}
	folderID, err := requiredStringArg(args, "folder_id")
	if err != nil {
		return "", err
	}
	hostPath, err := requiredStringArg(args, "host_path")
	if err != nil {
		return "", err
	}
	flags, err := wsRequiredInt(args, "flags")
	if err != nil {
		return "", err
	}
	body := wsSharedFolderUpdateBody{HostPath: hostPath, Flags: flags}
	return wsMutate(ctx, client, http.MethodPut, "/vms/"+id+"/sharedfolders/"+folderID, body)
}

func handleWSSharedFolderDelete(ctx context.Context, client *workstation.Client, args map[string]interface{}) (string, error) {
	id, err := requiredStringArg(args, "id")
	if err != nil {
		return "", err
	}
	folderID, err := requiredStringArg(args, "folder_id")
	if err != nil {
		return "", err
	}
	return wsMutate(ctx, client, http.MethodDelete, "/vms/"+id+"/sharedfolders/"+folderID, nil)
}

// --- VM Network Adapters Management (6 tools) -------------------------------

func registerWorkstationNetworkAdapterTools(r *Registry) {
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}
	idArg := map[string]interface{}{"type": "string", "description": "VM identifier, as returned by vmware_workstation_vm_list."}
	indexArg := map[string]interface{}{"type": "integer", "description": "Zero-based network adapter index within the VM, as returned by vmware_workstation_nic_list."}
	nicTypeArg := map[string]interface{}{
		"type":        "string",
		"enum":        []interface{}{"bridged", "hostonly", "nat", "custom"},
		"description": `Network adapter connection type. "custom" requires "vmnet" to select the specific virtual network (e.g. "vmnet8"); "bridged"/"hostonly"/"nat" ignore "vmnet" and use the host's default network of that kind.`,
	}
	vmnetArg := map[string]interface{}{"type": "string", "description": `Virtual network name (e.g. "vmnet8"), as returned by vmware_workstation_vmnet_list — required only when type is "custom".`}

	r.registerWorkstation("vmware_workstation_nic_ip",
		"Get the primary IP address of a VMware Workstation Pro VM (GET /vms/{id}/ip). Requires VMware Tools to be running in the guest."+wsRunningNote,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"id": idArg},
			"required":   []interface{}{"id"},
		},
		Tool{WSHandler: handleWSNicIP},
	)

	r.registerWorkstation("vmware_workstation_nic_list",
		"List all network adapters (NICs) of a VMware Workstation Pro VM (GET /vms/{id}/nic)."+wsRunningNote,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"id": idArg},
			"required":   []interface{}{"id"},
		},
		Tool{WSHandler: handleWSNicList},
	)

	r.registerDestructiveWorkstation("vmware_workstation_nic_create",
		"Create a new network adapter in a VMware Workstation Pro VM (POST /vms/{id}/nic)."+wsRunningNote,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"id":      idArg,
				"type":    nicTypeArg,
				"vmnet":   vmnetArg,
				"confirm": confirmArg,
			},
			"required": []interface{}{"id", "type", "confirm"},
		},
		Tool{WSHandler: handleWSNicCreate},
	)

	r.registerDestructiveWorkstation("vmware_workstation_nic_update",
		"Update an existing network adapter's connection type/virtual network in a VMware Workstation Pro VM (PUT /vms/{id}/nic/{index})."+wsRunningNote,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"id":      idArg,
				"index":   indexArg,
				"type":    nicTypeArg,
				"vmnet":   vmnetArg,
				"confirm": confirmArg,
			},
			"required": []interface{}{"id", "index", "type", "confirm"},
		},
		Tool{WSHandler: handleWSNicUpdate},
	)

	r.registerDestructiveWorkstation("vmware_workstation_nic_delete",
		"Remove a network adapter from a VMware Workstation Pro VM (DELETE /vms/{id}/nic/{index})."+wsRunningNote,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"id":      idArg,
				"index":   indexArg,
				"confirm": confirmArg,
			},
			"required": []interface{}{"id", "index", "confirm"},
		},
		Tool{WSHandler: handleWSNicDelete},
	)

	r.registerWorkstation("vmware_workstation_vm_nic_ips",
		"Get the IP stack configuration (all addresses/families reported by VMware Tools) of every network adapter in a VMware Workstation Pro VM (GET /vms/{id}/nicips). Distinct from vmware_workstation_nic_ip, which returns only the single primary IP."+wsRunningNote,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"id": idArg},
			"required":   []interface{}{"id"},
		},
		Tool{WSHandler: handleWSNicIPs},
	)
}

func handleWSNicIP(ctx context.Context, client *workstation.Client, args map[string]interface{}) (string, error) {
	id, err := requiredStringArg(args, "id")
	if err != nil {
		return "", err
	}
	return wsGet(ctx, client, "/vms/"+id+"/ip")
}

func handleWSNicList(ctx context.Context, client *workstation.Client, args map[string]interface{}) (string, error) {
	id, err := requiredStringArg(args, "id")
	if err != nil {
		return "", err
	}
	return wsGet(ctx, client, "/vms/"+id+"/nic")
}

type wsNicBody struct {
	Type  string `json:"type"`
	Vmnet string `json:"vmnet,omitempty"`
}

func handleWSNicCreate(ctx context.Context, client *workstation.Client, args map[string]interface{}) (string, error) {
	id, err := requiredStringArg(args, "id")
	if err != nil {
		return "", err
	}
	typ, err := requiredStringArg(args, "type")
	if err != nil {
		return "", err
	}
	vmnet, _ := args["vmnet"].(string)
	body := wsNicBody{Type: typ, Vmnet: vmnet}
	return wsMutate(ctx, client, http.MethodPost, "/vms/"+id+"/nic", body)
}

func handleWSNicUpdate(ctx context.Context, client *workstation.Client, args map[string]interface{}) (string, error) {
	id, err := requiredStringArg(args, "id")
	if err != nil {
		return "", err
	}
	index, err := wsRequiredInt(args, "index")
	if err != nil {
		return "", err
	}
	typ, err := requiredStringArg(args, "type")
	if err != nil {
		return "", err
	}
	vmnet, _ := args["vmnet"].(string)
	body := wsNicBody{Type: typ, Vmnet: vmnet}
	return wsMutate(ctx, client, http.MethodPut, "/vms/"+id+"/nic/"+strconv.Itoa(index), body)
}

func handleWSNicDelete(ctx context.Context, client *workstation.Client, args map[string]interface{}) (string, error) {
	id, err := requiredStringArg(args, "id")
	if err != nil {
		return "", err
	}
	index, err := wsRequiredInt(args, "index")
	if err != nil {
		return "", err
	}
	return wsMutate(ctx, client, http.MethodDelete, "/vms/"+id+"/nic/"+strconv.Itoa(index), nil)
}

func handleWSNicIPs(ctx context.Context, client *workstation.Client, args map[string]interface{}) (string, error) {
	id, err := requiredStringArg(args, "id")
	if err != nil {
		return "", err
	}
	return wsGet(ctx, client, "/vms/"+id+"/nicips")
}

// --- Host Networks Management (7 tools) -------------------------------------
//
// The plan's headline said 8 routes in this folder; the vendored Postman
// collection only has 7 request items under "Host Networks Management" (list
// vmnet, list/update mactoip, list/update/delete portforward, create
// vmnets) — confirmed by parsing the collection JSON directly, not assumed.

func registerWorkstationHostNetworkTools(r *Registry) {
	emptySchema := map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}
	vmnetArg := map[string]interface{}{"type": "string", "description": `Virtual network name, e.g. "vmnet0", "vmnet1", "vmnet8", as returned by vmware_workstation_vmnet_list.`}
	protocolArg := map[string]interface{}{"type": "string", "enum": []interface{}{"tcp", "udp"}, "description": "Transport protocol for the port forwarding rule."}
	portArg := map[string]interface{}{"type": "integer", "description": "Host-side port number for the port forwarding rule (1-65535)."}

	r.registerWorkstation("vmware_workstation_vmnet_list",
		"List all virtual networks known to the VMware Workstation Pro host (GET /vmnet)."+wsRunningNote,
		emptySchema,
		Tool{WSHandler: handleWSVmnetList},
	)

	r.registerDestructiveWorkstation("vmware_workstation_vmnet_create",
		"Create a new virtual network on the VMware Workstation Pro host (POST /vmnets)."+wsRunningNote,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name":    map[string]interface{}{"type": "string", "description": `New virtual network's name, e.g. "vmnet5".`},
				"type":    map[string]interface{}{"type": "string", "enum": []interface{}{"bridged", "hostonly", "nat"}, "description": "Virtual network kind."},
				"confirm": confirmArg,
			},
			"required": []interface{}{"name", "type", "confirm"},
		},
		Tool{WSHandler: handleWSVmnetCreate},
	)

	r.registerWorkstation("vmware_workstation_vmnet_mactoip_list",
		"List all MAC-address-to-IP bindings configured for a virtual network's DHCP service (GET /vmnet/{vmnet}/mactoip)."+wsRunningNote,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"vmnet": vmnetArg},
			"required":   []interface{}{"vmnet"},
		},
		Tool{WSHandler: handleWSVmnetMactoipList},
	)

	r.registerDestructiveWorkstation("vmware_workstation_vmnet_mactoip_set",
		"Create or update a MAC-address-to-IP binding for a virtual network's DHCP service (PUT /vmnet/{vmnet}/mactoip/{mac})."+wsRunningNote,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vmnet":   vmnetArg,
				"mac":     map[string]interface{}{"type": "string", "description": `MAC address to bind, e.g. "00:0c:29:12:34:56".`},
				"ip":      map[string]interface{}{"type": "string", "description": `IP address to assign to that MAC via DHCP, e.g. "192.168.130.10".`},
				"confirm": confirmArg,
			},
			"required": []interface{}{"vmnet", "mac", "ip", "confirm"},
		},
		Tool{WSHandler: handleWSVmnetMactoipSet},
	)

	r.registerWorkstation("vmware_workstation_vmnet_portforward_list",
		"List all port forwarding rules configured on a virtual network (GET /vmnet/{vmnet}/portforward)."+wsRunningNote,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"vmnet": vmnetArg},
			"required":   []interface{}{"vmnet"},
		},
		Tool{WSHandler: handleWSVmnetPortforwardList},
	)

	r.registerDestructiveWorkstation("vmware_workstation_vmnet_portforward_set",
		"Create or update a port forwarding rule on a virtual network — host port to guest IP:port (PUT /vmnet/{vmnet}/portforward/{protocol}/{port})."+wsRunningNote,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vmnet":      vmnetArg,
				"protocol":   protocolArg,
				"port":       portArg,
				"guest_ip":   map[string]interface{}{"type": "string", "description": `Guest VM IP address to forward to, e.g. "192.168.130.10".`},
				"guest_port": map[string]interface{}{"type": "integer", "description": "Guest-side port number."},
				"desc":       map[string]interface{}{"type": "string", "description": "Optional free-text description for the rule."},
				"confirm":    confirmArg,
			},
			"required": []interface{}{"vmnet", "protocol", "port", "guest_ip", "guest_port", "confirm"},
		},
		Tool{WSHandler: handleWSVmnetPortforwardSet},
	)

	r.registerDestructiveWorkstation("vmware_workstation_vmnet_portforward_delete",
		"Remove a port forwarding rule from a virtual network (DELETE /vmnet/{vmnet}/portforward/{protocol}/{port})."+wsRunningNote,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vmnet":    vmnetArg,
				"protocol": protocolArg,
				"port":     portArg,
				"confirm":  confirmArg,
			},
			"required": []interface{}{"vmnet", "protocol", "port", "confirm"},
		},
		Tool{WSHandler: handleWSVmnetPortforwardDelete},
	)
}

func handleWSVmnetList(ctx context.Context, client *workstation.Client, args map[string]interface{}) (string, error) {
	return wsGet(ctx, client, "/vmnet")
}

type wsVmnetCreateBody struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

func handleWSVmnetCreate(ctx context.Context, client *workstation.Client, args map[string]interface{}) (string, error) {
	name, err := requiredStringArg(args, "name")
	if err != nil {
		return "", err
	}
	typ, err := requiredStringArg(args, "type")
	if err != nil {
		return "", err
	}
	body := wsVmnetCreateBody{Name: name, Type: typ}
	return wsMutate(ctx, client, http.MethodPost, "/vmnets", body)
}

func handleWSVmnetMactoipList(ctx context.Context, client *workstation.Client, args map[string]interface{}) (string, error) {
	vmnet, err := requiredStringArg(args, "vmnet")
	if err != nil {
		return "", err
	}
	return wsGet(ctx, client, "/vmnet/"+vmnet+"/mactoip")
}

type wsMacToIPBody struct {
	IP string `json:"IP"`
}

func handleWSVmnetMactoipSet(ctx context.Context, client *workstation.Client, args map[string]interface{}) (string, error) {
	vmnet, err := requiredStringArg(args, "vmnet")
	if err != nil {
		return "", err
	}
	mac, err := requiredStringArg(args, "mac")
	if err != nil {
		return "", err
	}
	ip, err := requiredStringArg(args, "ip")
	if err != nil {
		return "", err
	}
	body := wsMacToIPBody{IP: ip}
	return wsMutate(ctx, client, http.MethodPut, "/vmnet/"+vmnet+"/mactoip/"+mac, body)
}

func handleWSVmnetPortforwardList(ctx context.Context, client *workstation.Client, args map[string]interface{}) (string, error) {
	vmnet, err := requiredStringArg(args, "vmnet")
	if err != nil {
		return "", err
	}
	return wsGet(ctx, client, "/vmnet/"+vmnet+"/portforward")
}

type wsPortForwardBody struct {
	GuestIP   string `json:"guestIp"`
	GuestPort int    `json:"guestPort"`
	Desc      string `json:"desc,omitempty"`
}

func handleWSVmnetPortforwardSet(ctx context.Context, client *workstation.Client, args map[string]interface{}) (string, error) {
	vmnet, err := requiredStringArg(args, "vmnet")
	if err != nil {
		return "", err
	}
	protocol, err := requiredStringArg(args, "protocol")
	if err != nil {
		return "", err
	}
	port, err := wsRequiredInt(args, "port")
	if err != nil {
		return "", err
	}
	guestIP, err := requiredStringArg(args, "guest_ip")
	if err != nil {
		return "", err
	}
	guestPort, err := wsRequiredInt(args, "guest_port")
	if err != nil {
		return "", err
	}
	desc, _ := args["desc"].(string)
	body := wsPortForwardBody{GuestIP: guestIP, GuestPort: guestPort, Desc: desc}
	return wsMutate(ctx, client, http.MethodPut, "/vmnet/"+vmnet+"/portforward/"+protocol+"/"+strconv.Itoa(port), body)
}

func handleWSVmnetPortforwardDelete(ctx context.Context, client *workstation.Client, args map[string]interface{}) (string, error) {
	vmnet, err := requiredStringArg(args, "vmnet")
	if err != nil {
		return "", err
	}
	protocol, err := requiredStringArg(args, "protocol")
	if err != nil {
		return "", err
	}
	port, err := wsRequiredInt(args, "port")
	if err != nil {
		return "", err
	}
	return wsMutate(ctx, client, http.MethodDelete, "/vmnet/"+vmnet+"/portforward/"+protocol+"/"+strconv.Itoa(port), nil)
}

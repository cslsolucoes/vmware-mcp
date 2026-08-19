package tools

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/vim25/mo"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// registerHostNetworkQueryTools adds the read-only port-group/vSwitch
// inventory tool that generated_host_network.go's write-only Add/Update/Remove
// set was missing. HostNetworkSystem exposes UpdatePortGroup — which REPLACES
// the whole HostPortGroupSpec rather than patching it — but no way to READ the
// current spec first, so a caller had no safe source for the name/vswitchName/
// policy fields that must be preserved across an update (e.g. a VLAN change).
// This reads host.config.network and returns each port group's full current
// Spec (ready to re-send to vmware_host_network_update_port_group with only
// the field(s) you want changed, e.g. vlanId), plus the vSwitches for context.
//
// Class: modeVSphereGeneral — host-level network config, works on a standalone
// ESXi host. Added 2026-08-19 to make VLAN changes on iSCSI/storage port
// groups safe (see the update_port_group "replaces the whole spec" gotcha).
func registerHostNetworkQueryTools(r *Registry) {
	r.register("vmware_host_network_query_port_groups",
		"List the standard-switch port groups on an ESXi host with their current config (name, VLAN id, virtual switch, and the full HostPortGroupSpec incl. NIC teaming/security/traffic-shaping policy), plus the virtual switches. Read-only. Use this BEFORE vmware_host_network_update_port_group: that call REPLACES the entire port group spec, so take the returned \"spec\", change only the field(s) you want (e.g. vlanId), and send that complete spec back — a partial spec zeroes name/vswitchName/policy and breaks the port group.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host": map[string]interface{}{
					"type":        "string",
					"description": `Host identifier: a name/pattern (e.g. "esxi-01.local") or a full inventory path as returned by vmware_list_hosts. Must resolve to exactly one host.`,
				},
			},
			"required": []interface{}{"host"},
		},
		Tool{Handler: handleHostNetworkQueryPortGroups},
	)
}

func handleHostNetworkQueryPortGroups(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}

	var moHost mo.HostSystem
	if err := host.Properties(ctx, host.Reference(), []string{"config.network"}, &moHost); err != nil {
		return "", fmt.Errorf("failed to read network config for %s: %w", host.InventoryPath, err)
	}
	if moHost.Config == nil || moHost.Config.Network == nil {
		return marshalJSON(map[string]interface{}{
			"host": host.InventoryPath, "count": 0,
			"port_groups": []interface{}{}, "vswitches": []interface{}{},
		})
	}
	net := moHost.Config.Network

	pgs := make([]map[string]interface{}, 0, len(net.Portgroup))
	for i := range net.Portgroup {
		pg := net.Portgroup[i]
		pgs = append(pgs, map[string]interface{}{
			"name":         pg.Spec.Name,
			"vlan_id":      pg.Spec.VlanId,
			"vswitch_name": pg.Spec.VswitchName,
			// Full current spec — re-send this verbatim to
			// vmware_host_network_update_port_group with only the field(s) you
			// want changed. UpdatePortGroup replaces the whole spec, so keeping
			// name/vswitchName/policy here is what prevents breaking the group.
			"spec": pg.Spec,
		})
	}

	vsws := make([]map[string]interface{}, 0, len(net.Vswitch))
	for i := range net.Vswitch {
		vs := net.Vswitch[i]
		vsws = append(vsws, map[string]interface{}{
			"name":       vs.Name,
			"mtu":        vs.Mtu,
			"pnics":      vs.Pnic,
			"portgroups": vs.Portgroup,
		})
	}

	return marshalJSON(map[string]interface{}{
		"host":        host.InventoryPath,
		"count":       len(pgs),
		"port_groups": pgs,
		"vswitches":   vsws,
	})
}

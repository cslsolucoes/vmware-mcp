package tools

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// registerHostTools registers ESXi host operations: maintenance mode,
// reconnect, management IPs, and summary info. vmware_host_maintenance_enter
// is the only Tier 2 tool here — on a standalone ESXi host (no cluster to
// evacuate to) it can power off every VM on that host, as disruptive as any
// Tier 1 operation in that topology (see the plan's Fase 1a severity table).
func registerHostTools(r *Registry) {
	hostArg := map[string]interface{}{
		"type":        "string",
		"description": `Host identifier: a name/pattern (e.g. "esxi-01.local") or a full inventory path (e.g. "/ha-datacenter/host/esxi-01.local/esxi-01.local") as returned by vmware_list_hosts. Must resolve to exactly one host.`,
	}
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}

	r.registerDestructive("vmware_host_maintenance_enter",
		"Put an ESXi host into maintenance mode. On a standalone host (no cluster/DRS to evacuate powered-on VMs to) this can power off every VM running on it — treat it as destructive on that topology, not just a mode toggle.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":     hostArg,
				"timeout":  map[string]interface{}{"type": "integer", "description": "Seconds to wait for the operation before giving up server-side. 0 (default) waits indefinitely."},
				"evacuate": map[string]interface{}{"type": "boolean", "description": "Evacuate powered-off VMs' storage to other hosts (only meaningful with DRS/a cluster — a no-op on standalone ESXi). Default false."},
				"confirm":  confirmArg,
			},
			"required": []interface{}{"host", "confirm"},
		},
		Tool{Handler: handleHostMaintenanceEnter},
	)

	r.register("vmware_host_maintenance_exit",
		"Take an ESXi host back out of maintenance mode.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":    hostArg,
				"timeout": map[string]interface{}{"type": "integer", "description": "Seconds to wait for the operation before giving up server-side. 0 (default) waits indefinitely."},
			},
			"required": []interface{}{"host"},
		},
		Tool{Handler: handleHostMaintenanceExit},
	)

	r.register("vmware_host_reconnect",
		"Reconnect a disconnected ESXi host to vCenter (no-op/error against a standalone ESXi host, which has no vCenter managing it).",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"host": hostArg},
			"required":   []interface{}{"host"},
		},
		Tool{Handler: handleHostReconnect},
	)

	r.register("vmware_host_management_ips",
		"List the management network IP addresses of an ESXi host.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"host": hostArg},
			"required":   []interface{}{"host"},
		},
		Tool{Handler: handleHostManagementIPs},
	)

	r.register("vmware_host_info",
		"Get connection/power/maintenance state and hardware summary (model, CPU, memory) for an ESXi host.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"host": hostArg},
			"required":   []interface{}{"host"},
		},
		Tool{Handler: handleHostInfo},
	)
}

// resolveHost resolves the required "host" argument to exactly one
// HostSystem, scoping a non-absolute name/pattern with dcScopedPath — same
// reasoning as resolveVM in vm.go (works against a vCenter with more than
// one datacenter, not just standalone ESXi/single-datacenter vCenter).
func resolveHost(ctx context.Context, client *vmware.Client, args map[string]interface{}) (*object.HostSystem, error) {
	name, _ := args["host"].(string)
	if name == "" {
		return nil, fmt.Errorf("host is required")
	}

	host, err := client.Finder.HostSystem(ctx, dcScopedPath("host", name))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve host %q: %w", name, err)
	}
	return host, nil
}

func handleHostMaintenanceEnter(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}

	var timeout int32
	if v, ok := args["timeout"]; ok {
		n, err := toInt32(v)
		if err != nil {
			return "", fmt.Errorf("invalid timeout: %w", err)
		}
		timeout = n
	}
	evacuate, _ := args["evacuate"].(bool)

	task, err := host.EnterMaintenanceMode(ctx, timeout, evacuate, nil)
	if err != nil {
		return "", fmt.Errorf("failed to enter maintenance mode on %s: %w", host.InventoryPath, err)
	}
	if err := waitForTask(ctx, task); err != nil {
		return "", fmt.Errorf("enter-maintenance-mode task failed for %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "result": "entered_maintenance_mode"})
}

func handleHostMaintenanceExit(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}

	var timeout int32
	if v, ok := args["timeout"]; ok {
		n, err := toInt32(v)
		if err != nil {
			return "", fmt.Errorf("invalid timeout: %w", err)
		}
		timeout = n
	}

	task, err := host.ExitMaintenanceMode(ctx, timeout)
	if err != nil {
		return "", fmt.Errorf("failed to exit maintenance mode on %s: %w", host.InventoryPath, err)
	}
	if err := waitForTask(ctx, task); err != nil {
		return "", fmt.Errorf("exit-maintenance-mode task failed for %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "result": "exited_maintenance_mode"})
}

func handleHostReconnect(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}

	task, err := host.Reconnect(ctx, nil, nil)
	if err != nil {
		return "", fmt.Errorf("failed to reconnect %s: %w", host.InventoryPath, err)
	}
	if err := waitForTask(ctx, task); err != nil {
		return "", fmt.Errorf("reconnect task failed for %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "result": "reconnected"})
}

func handleHostManagementIPs(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}

	ips, err := host.ManagementIPs(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to read management IPs for %s: %w", host.InventoryPath, err)
	}

	addrs := make([]string, 0, len(ips))
	for _, ip := range ips {
		addrs = append(addrs, ip.String())
	}

	return marshalJSON(map[string]interface{}{
		"host":  host.InventoryPath,
		"count": len(addrs),
		"ips":   addrs,
	})
}

func handleHostInfo(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}

	var moHost mo.HostSystem
	if err := host.Properties(ctx, host.Reference(), []string{"summary"}, &moHost); err != nil {
		return "", fmt.Errorf("failed to read info for %s: %w", host.InventoryPath, err)
	}
	s := moHost.Summary

	result := map[string]interface{}{
		"host": host.InventoryPath,
		"name": s.Config.Name,
	}
	if s.Runtime != nil {
		result["connection_state"] = string(s.Runtime.ConnectionState)
		result["power_state"] = string(s.Runtime.PowerState)
		result["in_maintenance_mode"] = s.Runtime.InMaintenanceMode
	}
	if s.Hardware != nil {
		result["model"] = s.Hardware.Model
		result["cpu_model"] = s.Hardware.CpuModel
		result["num_cpu_cores"] = s.Hardware.NumCpuCores
		result["memory_bytes"] = s.Hardware.MemorySize
	}

	return marshalJSON(result)
}

package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// registerSystemTools registers the seed read-only tools that prove the
// connect -> registry -> stdio-handler pipeline end to end. Real VMware
// domain tools (VM lifecycle, datastores, hosts, clusters, networks, etc.)
// get their own tools/<namespace>.go file added incrementally, following
// this file's shape.
func registerSystemTools(r *Registry) {
	r.register("vmware_about",
		"Get information about the connected vCenter/ESXi endpoint (product, version, build, API type, instance UUID).",
		map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
		Tool{Handler: handleAbout},
	)

	r.register("vmware_list_vms",
		"List virtual machines visible to the connected session under an inventory path pattern (govmomi find syntax).",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": `Inventory path pattern. Defaults to "*" (every VM under the default datacenter's VM folder).`,
					"default":     "*",
				},
			},
		},
		Tool{Handler: handleListVMs},
	)
}

// marshalJSON is the shared JSON response formatter for tool handlers in
// this package.
func marshalJSON(v interface{}) (string, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal response: %w", err)
	}
	return string(b), nil
}

// handleAbout returns the ServiceContent.About block already fetched during
// login (govmomi.NewClient populates it as part of connecting) — no extra
// round trip to the endpoint is needed.
func handleAbout(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	about := client.Client.ServiceContent.About
	return marshalJSON(map[string]interface{}{
		"name":          about.Name,
		"full_name":     about.FullName,
		"vendor":        about.Vendor,
		"version":       about.Version,
		"build":         about.Build,
		"api_type":      about.ApiType,
		"api_version":   about.ApiVersion,
		"instance_uuid": about.InstanceUuid,
	})
}

// handleListVMs resolves path via the client's Finder (see vmware.NewClient
// — the Finder's default datacenter is set at connect time when resolvable).
func handleListVMs(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	path := "*"
	if v, ok := args["path"].(string); ok && v != "" {
		path = v
	}

	vms, err := client.Finder.VirtualMachineList(ctx, path)
	if err != nil {
		return "", fmt.Errorf("failed to list virtual machines matching %q: %w", path, err)
	}

	names := make([]string, 0, len(vms))
	for _, vm := range vms {
		names = append(names, vm.InventoryPath)
	}
	return marshalJSON(map[string]interface{}{
		"path":  path,
		"count": len(names),
		"vms":   names,
	})
}

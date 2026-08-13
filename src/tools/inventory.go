package tools

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// registerInventoryTools registers the read-only listing tools for the
// inventory objects a datacenter groups: hosts, datastores, networks,
// resource pools, clusters, and datacenters themselves. vmware_list_vms
// lives in system.go (seed tool) and is not repeated here.
//
// Every path defaults to "*" and, unless already absolute, is scoped with
// dcScopedPath — see finder.go for why: a bare "*" fails against a vCenter
// with more than one datacenter (client.Finder has no default datacenter set
// in that case), while the scoped form works there and against standalone
// ESXi/single-datacenter vCenter alike. Handlers tolerate empty results
// (via emptyOnNotFound) instead of erroring — a standalone ESXi host has no
// clusters and often only one datacenter, that is expected, not a failure.
func registerInventoryTools(r *Registry) {
	pathSchema := func(folder, example string) map[string]interface{} {
		return map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type": "string",
					"description": fmt.Sprintf(
						`Name pattern (govmomi find syntax) matched under every datacenter's %s folder, e.g. "*" for all or %q to filter by name. Pass a full inventory path starting with "/" to scope to one datacenter instead. Defaults to "*".`,
						folder, example),
					"default": "*",
				},
			},
		}
	}

	r.register("vmware_list_hosts",
		"List ESXi hosts visible to the connected session under an inventory path pattern.",
		pathSchema("host", "esxi-*"),
		Tool{Handler: handleListHosts},
	)

	r.register("vmware_list_datastores",
		"List datastores visible to the connected session, with capacity/free space/accessibility summary.",
		pathSchema("datastore", "datastore1"),
		Tool{Handler: handleListDatastores},
	)

	r.register("vmware_list_networks",
		"List networks (standard port groups, opaque networks, and distributed port groups) visible to the connected session.",
		pathSchema("network", "VM Network"),
		Tool{Handler: handleListNetworks},
	)

	r.register("vmware_list_resource_pools",
		`List resource pools visible to the connected session (every ComputeResource/ClusterComputeResource has at least an implicit "Resources" pool).`,
		pathSchema("host", "Resources"),
		Tool{Handler: handleListResourcePools},
	)

	r.register("vmware_list_clusters",
		"List clusters (ClusterComputeResource) visible to the connected session. Empty on a standalone ESXi host or a vCenter with no clusters configured — that is expected, not an error.",
		pathSchema("host", "cluster-*"),
		Tool{Handler: handleListClusters},
	)

	r.register("vmware_list_datacenters",
		`List datacenters visible to the connected session. A standalone ESXi host always reports exactly one ("ha-datacenter").`,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": `Name pattern (govmomi find syntax), e.g. "*" for all. Defaults to "*".`,
					"default":     "*",
				},
			},
		},
		Tool{Handler: handleListDatacenters},
	)
}

// pathArg reads the optional "path" tool argument, defaulting to "*".
func pathArg(args map[string]interface{}) string {
	if v, ok := args["path"].(string); ok && v != "" {
		return v
	}
	return "*"
}

func handleListHosts(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	path := pathArg(args)

	hosts, err := client.Finder.HostSystemList(ctx, dcScopedPath("host", path))
	if err = emptyOnNotFound(err); err != nil {
		return "", fmt.Errorf("failed to list hosts matching %q: %w", path, err)
	}

	names := make([]string, 0, len(hosts))
	for _, h := range hosts {
		names = append(names, h.InventoryPath)
	}
	return marshalJSON(map[string]interface{}{
		"path":  path,
		"count": len(names),
		"hosts": names,
	})
}

func handleListDatastores(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	path := pathArg(args)

	datastores, err := client.Finder.DatastoreList(ctx, dcScopedPath("datastore", path))
	if err = emptyOnNotFound(err); err != nil {
		return "", fmt.Errorf("failed to list datastores matching %q: %w", path, err)
	}

	summaries, err := datastoreSummaries(ctx, client, datastores)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve summary for datastores matching %q: %w", path, err)
	}

	return marshalJSON(map[string]interface{}{
		"path":       path,
		"count":      len(summaries),
		"datastores": summaries,
	})
}

// datastoreSummaries batch-fetches the "summary" property for every
// datastore in one PropertyCollector round trip (property.Collector.Retrieve)
// instead of one round trip per datastore — the pattern already confirmed as
// correct/expected for this project when the 3 reference MCP servers were
// analyzed (see .wolf/STATUS.md, "Done" 2026-08-09 22:43: "PropertyCollector
// em lote em vez de N+1 chamadas REST").
func datastoreSummaries(ctx context.Context, client *vmware.Client, datastores []*object.Datastore) ([]map[string]interface{}, error) {
	if len(datastores) == 0 {
		return []map[string]interface{}{}, nil
	}

	refs := make([]types.ManagedObjectReference, len(datastores))
	for i, ds := range datastores {
		refs[i] = ds.Reference()
	}

	var mods []mo.Datastore
	pc := property.DefaultCollector(client.Client.Client)
	if err := pc.Retrieve(ctx, refs, []string{"summary"}, &mods); err != nil {
		return nil, err
	}

	byRef := make(map[types.ManagedObjectReference]mo.Datastore, len(mods))
	for _, m := range mods {
		byRef[m.Self] = m
	}

	out := make([]map[string]interface{}, 0, len(datastores))
	for _, ds := range datastores {
		s := byRef[ds.Reference()].Summary
		out = append(out, map[string]interface{}{
			"name":             s.Name,
			"path":             ds.InventoryPath,
			"type":             s.Type,
			"capacity_bytes":   s.Capacity,
			"free_space_bytes": s.FreeSpace,
			"accessible":       s.Accessible,
		})
	}
	return out, nil
}

func handleListNetworks(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	path := pathArg(args)

	networks, err := client.Finder.NetworkList(ctx, dcScopedPath("network", path))
	if err = emptyOnNotFound(err); err != nil {
		return "", fmt.Errorf("failed to list networks matching %q: %w", path, err)
	}

	names := make([]string, 0, len(networks))
	for _, n := range networks {
		names = append(names, n.GetInventoryPath())
	}
	return marshalJSON(map[string]interface{}{
		"path":     path,
		"count":    len(names),
		"networks": names,
	})
}

func handleListResourcePools(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	path := pathArg(args)

	pools, err := client.Finder.ResourcePoolList(ctx, dcScopedPath("host", path))
	if err = emptyOnNotFound(err); err != nil {
		return "", fmt.Errorf("failed to list resource pools matching %q: %w", path, err)
	}

	names := make([]string, 0, len(pools))
	for _, p := range pools {
		names = append(names, p.InventoryPath)
	}
	return marshalJSON(map[string]interface{}{
		"path":           path,
		"count":          len(names),
		"resource_pools": names,
	})
}

func handleListClusters(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	path := pathArg(args)

	clusters, err := client.Finder.ClusterComputeResourceList(ctx, dcScopedPath("host", path))
	if err = emptyOnNotFound(err); err != nil {
		return "", fmt.Errorf("failed to list clusters matching %q: %w", path, err)
	}

	names := make([]string, 0, len(clusters))
	for _, c := range clusters {
		names = append(names, c.InventoryPath)
	}
	return marshalJSON(map[string]interface{}{
		"path":     path,
		"count":    len(names),
		"clusters": names,
	})
}

func handleListDatacenters(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	path := pathArg(args)

	datacenters, err := client.Finder.DatacenterList(ctx, path)
	if err = emptyOnNotFound(err); err != nil {
		return "", fmt.Errorf("failed to list datacenters matching %q: %w", path, err)
	}

	names := make([]string, 0, len(datacenters))
	for _, dc := range datacenters {
		names = append(names, dc.InventoryPath)
	}
	return marshalJSON(map[string]interface{}{
		"path":        path,
		"count":       len(names),
		"datacenters": names,
	})
}

package tools

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// registerSearchIndexTools is the "search index" slice of Fase 7 (Grupo A)
// of the codegen plan (".workspace/plans/MCPVMWare2026-08-10-175300-plano-cobertura-completa-api-codegen.plan.md")
// — object.SearchIndex (referencia/govmomi/object/search_index.go), all 9
// methods, hand-transcribed following the vm.go/generated_vm_lifecycle.go
// conventions. All 9 are legitimate, already-classified no-tier (pure
// lookups, no mutation) — no exclusions in this file, unlike most other
// generated_*.go files in this project.
//
// Every one of the 9 methods has real, working server-side support in
// referencia/govmomi/simulator/search_index.go — confirmed by reading it
// directly, not assumed from the file existing (FindByDnsName/FindByIp
// simply delegate to their FindAllBy* counterpart and take the first
// result, both are implemented for real). generated_search_index_test.go
// therefore exercises genuine success paths (not just
// registration/assertReachesServer), including one full round trip through
// vmware_search_index_find_by_ip using the same "SET.guest.ipAddress"
// ExtraConfig fixture trick generated_vm_lifecycle_test.go's setGuestIP
// already uses (reused here, not duplicated) — confirmed to be the same
// field simulator.SearchIndex.FindAllByIp reads (vm.Guest.IpAddress).
//
// Curation:
//
//   - Every method result is an object.Reference (a moref-backed interface,
//     concretely *object.VirtualMachine/*object.HostSystem/etc., or nil for
//     "not found") — never resolved via a Finder, so bare .InventoryPath is
//     always "" on it, the same InventoryPath gotcha documented throughout
//     this project (e.g. generated_vm_lifecycle.go's handleVMHostSystem).
//     searchIndexReferenceJSON below resolves the real path via
//     find.InventoryPath for every result uniformly (including
//     FindByInventoryPath's result, which the real object/search_index.go
//     source does populate .InventoryPath on directly via a
//     SetInventoryPath side effect — re-resolving it the same uniform way
//     here anyway is simpler than special-casing one of the 9 methods, and
//     costs one extra property-collector round trip at most).
//
//   - A nil Reference (SearchIndex found nothing) is reported as
//     {"found": false}, not a tool error — matching this project's "0/no
//     result is not a tool error" convention (see finder.go's
//     emptyOnNotFound, generated_vm_lifecycle.go's ResourcePool handling).
//
//   - vmware_search_index_find_by_datastore_path's "datacenter" argument is
//     REQUIRED, unlike every other tool in this file where it is optional
//     (omit to search across every datacenter). Read directly from the real
//     source (referencia/govmomi/object/search_index.go's
//     FindByDatastorePath): it unconditionally dereferences
//     "Datacenter: dc.Reference()" with no nil check, unlike every other
//     method here, which all guard with "if dc != nil { ... }" first — a
//     nil dc would panic inside the govmomi client call (CallTool's
//     recover() would turn that into an ugly "tool ... panicked" error
//     instead of a normal one). Same class of finding as
//     generated_vm_lifecycle.go's vmware_vm_query_changed_disk_areas
//     base_snapshot argument.
//
//   - vmware_search_index_find_child's "entity" argument accepts a raw
//     {"type": "...", "value": "..."} managed object reference (decoded
//     into a plain types.ManagedObjectReference, which satisfies
//     object.Reference directly via its own self-referential
//     "func (r ManagedObjectReference) Reference() ManagedObjectReference"
//     method in vim25/types/helpers.go — no need to wrap it through
//     object.NewReference first). FindChild walks a parent entity's direct
//     children (datacenter/folder/compute-resource/resource-pool/vApp) by
//     name; a caller typically already has a moref to pass here from
//     another vmware_search_index_* tool's result (this file's uniform
//     "type"/"value" result shape, via searchIndexReferenceJSON) or from
//     elsewhere. There is no existing by-name Finder-backed alternative for
//     this specific "list a folder's children" operation in this project.
func registerSearchIndexTools(r *Registry) {
	datacenterArg := map[string]interface{}{
		"type":        "string",
		"description": `Datacenter name/pattern (e.g. "ha-datacenter") as returned by vmware_list_datacenters. Optional — omit to search across every datacenter.`,
	}
	vmSearchArg := map[string]interface{}{
		"type":        "boolean",
		"description": "true to search virtual machines, false to search hosts instead. Default true.",
	}
	instanceUUIDArg := map[string]interface{}{
		"type":        "boolean",
		"description": "true to match a VM's instance UUID instead of its BIOS UUID. Only meaningful when vm_search is true (or omitted). Omit to use the server default (BIOS UUID).",
	}

	r.register("vmware_search_index_find_by_datastore_path",
		`Find a single virtual machine by its .vmx path on a datastore (e.g. "[datastore1] myvm/myvm.vmx"). Unlike this file's other tools, datacenter is REQUIRED here — the underlying API dereferences it unconditionally (see this file's top doc comment).`,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"datacenter": map[string]interface{}{"type": "string", "description": `Datacenter name/pattern (e.g. "ha-datacenter") as returned by vmware_list_datacenters. Required — see this tool's description.`},
				"path":       map[string]interface{}{"type": "string", "description": `Datastore path, e.g. "[datastore1] myvm/myvm.vmx".`},
			},
			"required": []interface{}{"datacenter", "path"},
		},
		Tool{Handler: handleSearchIndexFindByDatastorePath},
	)

	r.register("vmware_search_index_find_by_dns_name",
		"Find a single virtual machine or host by guest/management DNS name. Not an error if nothing matches — reports {\"found\": false}.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"datacenter": datacenterArg,
				"dns_name":   map[string]interface{}{"type": "string", "description": "DNS name to search for."},
				"vm_search":  vmSearchArg,
			},
			"required": []interface{}{"dns_name"},
		},
		Tool{Handler: handleSearchIndexFindByDnsName},
	)

	r.register("vmware_search_index_find_all_by_dns_name",
		"Find every virtual machine or host matching a guest/management DNS name.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"datacenter": datacenterArg,
				"dns_name":   map[string]interface{}{"type": "string", "description": "DNS name to search for."},
				"vm_search":  vmSearchArg,
			},
			"required": []interface{}{"dns_name"},
		},
		Tool{Handler: handleSearchIndexFindAllByDnsName},
	)

	r.register("vmware_search_index_find_by_ip",
		"Find a single virtual machine or host by IP address. Not an error if nothing matches — reports {\"found\": false}.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"datacenter": datacenterArg,
				"ip":         map[string]interface{}{"type": "string", "description": "IP address to search for."},
				"vm_search":  vmSearchArg,
			},
			"required": []interface{}{"ip"},
		},
		Tool{Handler: handleSearchIndexFindByIP},
	)

	r.register("vmware_search_index_find_all_by_ip",
		"Find every virtual machine or host matching an IP address.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"datacenter": datacenterArg,
				"ip":         map[string]interface{}{"type": "string", "description": "IP address to search for."},
				"vm_search":  vmSearchArg,
			},
			"required": []interface{}{"ip"},
		},
		Tool{Handler: handleSearchIndexFindAllByIP},
	)

	r.register("vmware_search_index_find_by_uuid",
		"Find a single virtual machine or host by UUID (BIOS UUID by default, or instance UUID with instance_uuid:true). Not an error if nothing matches — reports {\"found\": false}.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"datacenter":    datacenterArg,
				"uuid":          map[string]interface{}{"type": "string", "description": "UUID to search for."},
				"vm_search":     vmSearchArg,
				"instance_uuid": instanceUUIDArg,
			},
			"required": []interface{}{"uuid"},
		},
		Tool{Handler: handleSearchIndexFindByUUID},
	)

	r.register("vmware_search_index_find_all_by_uuid",
		"Find every virtual machine or host matching a UUID (BIOS UUID by default, or instance UUID with instance_uuid:true).",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"datacenter":    datacenterArg,
				"uuid":          map[string]interface{}{"type": "string", "description": "UUID to search for."},
				"vm_search":     vmSearchArg,
				"instance_uuid": instanceUUIDArg,
			},
			"required": []interface{}{"uuid"},
		},
		Tool{Handler: handleSearchIndexFindAllByUUID},
	)

	r.register("vmware_search_index_find_by_inventory_path",
		"Find any managed entity (VM, host, folder, datacenter, ...) by its full inventory path. Not an error if nothing matches — reports {\"found\": false}.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{"type": "string", "description": `Full inventory path, e.g. "/ha-datacenter/vm/myvm".`},
			},
			"required": []interface{}{"path"},
		},
		Tool{Handler: handleSearchIndexFindByInventoryPath},
	)

	r.register("vmware_search_index_find_child",
		"Find a direct child of a managed entity (datacenter, folder, compute resource, resource pool, or vApp) by name. Not an error if nothing matches — reports {\"found\": false}.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"entity": map[string]interface{}{"type": "object", "description": `A managed object reference {"type": "...", "value": "..."} identifying the parent entity to search under — e.g. the "type"/"value" fields from another vmware_search_index_* tool's result.`},
				"name":   map[string]interface{}{"type": "string", "description": "Name of the direct child to find."},
			},
			"required": []interface{}{"entity", "name"},
		},
		Tool{Handler: handleSearchIndexFindChild},
	)
}

// searchIndexReferenceJSON converts a SearchIndex result (nil on "not
// found") into this file's uniform JSON shape — see this file's top doc
// comment for the InventoryPath resolution rationale.
func searchIndexReferenceJSON(ctx context.Context, client *vmware.Client, ref object.Reference) map[string]interface{} {
	if ref == nil {
		return map[string]interface{}{"found": false}
	}
	moref := ref.Reference()
	result := map[string]interface{}{
		"found": true,
		"type":  moref.Type,
		"value": moref.Value,
	}
	// Some result types/stale morefs may not have a resolvable inventory
	// path — degrade to null rather than failing the whole call; the moref
	// itself (type+value) is still meaningful on its own.
	if path, err := find.InventoryPath(ctx, client.Client.Client, moref); err == nil {
		result["inventory_path"] = path
	} else {
		result["inventory_path"] = nil
	}
	return result
}

// searchIndexReferenceListJSON is searchIndexReferenceJSON applied to every
// element of a FindAllBy* result.
func searchIndexReferenceListJSON(ctx context.Context, client *vmware.Client, refs []object.Reference) map[string]interface{} {
	results := make([]map[string]interface{}, 0, len(refs))
	for _, ref := range refs {
		results = append(results, searchIndexReferenceJSON(ctx, client, ref))
	}
	return map[string]interface{}{"count": len(results), "results": results}
}

// vmSearchArgValue reads the optional "vm_search" argument, defaulting to
// true (search VMs) when omitted, matching every SearchIndex method's own
// vmSearch bool parameter.
func vmSearchArgValue(args map[string]interface{}) bool {
	if v, ok := args["vm_search"].(bool); ok {
		return v
	}
	return true
}

// instanceUUIDArgValue reads the optional "instance_uuid" argument as a
// *bool (nil when omitted, matching FindByUuid/FindAllByUuid's own
// *bool instanceUuid parameter — a nil pointer there means "server default
// (BIOS UUID)", not "false").
func instanceUUIDArgValue(args map[string]interface{}) *bool {
	if v, ok := args["instance_uuid"].(bool); ok {
		return &v
	}
	return nil
}

func handleSearchIndexFindByDatastorePath(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	dcName, _ := args["datacenter"].(string)
	dc, err := resolveDatacenter(ctx, client, dcName)
	if err != nil {
		return "", err
	}
	path, _ := args["path"].(string)
	if path == "" {
		return "", fmt.Errorf("path is required")
	}

	si := object.NewSearchIndex(client.Client.Client)
	ref, err := si.FindByDatastorePath(ctx, dc, path)
	if err != nil {
		return "", fmt.Errorf("failed to find by datastore path %q: %w", path, err)
	}

	return marshalJSON(searchIndexReferenceJSON(ctx, client, ref))
}

func handleSearchIndexFindByDnsName(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	dc, err := resolveOptionalDatacenter(ctx, client, args)
	if err != nil {
		return "", err
	}
	dnsName, _ := args["dns_name"].(string)
	if dnsName == "" {
		return "", fmt.Errorf("dns_name is required")
	}

	si := object.NewSearchIndex(client.Client.Client)
	ref, err := si.FindByDnsName(ctx, dc, dnsName, vmSearchArgValue(args))
	if err != nil {
		return "", fmt.Errorf("failed to find by DNS name %q: %w", dnsName, err)
	}

	return marshalJSON(searchIndexReferenceJSON(ctx, client, ref))
}

func handleSearchIndexFindAllByDnsName(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	dc, err := resolveOptionalDatacenter(ctx, client, args)
	if err != nil {
		return "", err
	}
	dnsName, _ := args["dns_name"].(string)
	if dnsName == "" {
		return "", fmt.Errorf("dns_name is required")
	}

	si := object.NewSearchIndex(client.Client.Client)
	refs, err := si.FindAllByDnsName(ctx, dc, dnsName, vmSearchArgValue(args))
	if err != nil {
		return "", fmt.Errorf("failed to find all by DNS name %q: %w", dnsName, err)
	}

	return marshalJSON(searchIndexReferenceListJSON(ctx, client, refs))
}

func handleSearchIndexFindByIP(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	dc, err := resolveOptionalDatacenter(ctx, client, args)
	if err != nil {
		return "", err
	}
	ip, _ := args["ip"].(string)
	if ip == "" {
		return "", fmt.Errorf("ip is required")
	}

	si := object.NewSearchIndex(client.Client.Client)
	ref, err := si.FindByIp(ctx, dc, ip, vmSearchArgValue(args))
	if err != nil {
		return "", fmt.Errorf("failed to find by IP %q: %w", ip, err)
	}

	return marshalJSON(searchIndexReferenceJSON(ctx, client, ref))
}

func handleSearchIndexFindAllByIP(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	dc, err := resolveOptionalDatacenter(ctx, client, args)
	if err != nil {
		return "", err
	}
	ip, _ := args["ip"].(string)
	if ip == "" {
		return "", fmt.Errorf("ip is required")
	}

	si := object.NewSearchIndex(client.Client.Client)
	refs, err := si.FindAllByIp(ctx, dc, ip, vmSearchArgValue(args))
	if err != nil {
		return "", fmt.Errorf("failed to find all by IP %q: %w", ip, err)
	}

	return marshalJSON(searchIndexReferenceListJSON(ctx, client, refs))
}

func handleSearchIndexFindByUUID(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	dc, err := resolveOptionalDatacenter(ctx, client, args)
	if err != nil {
		return "", err
	}
	uuid, _ := args["uuid"].(string)
	if uuid == "" {
		return "", fmt.Errorf("uuid is required")
	}

	si := object.NewSearchIndex(client.Client.Client)
	ref, err := si.FindByUuid(ctx, dc, uuid, vmSearchArgValue(args), instanceUUIDArgValue(args))
	if err != nil {
		return "", fmt.Errorf("failed to find by UUID %q: %w", uuid, err)
	}

	return marshalJSON(searchIndexReferenceJSON(ctx, client, ref))
}

func handleSearchIndexFindAllByUUID(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	dc, err := resolveOptionalDatacenter(ctx, client, args)
	if err != nil {
		return "", err
	}
	uuid, _ := args["uuid"].(string)
	if uuid == "" {
		return "", fmt.Errorf("uuid is required")
	}

	si := object.NewSearchIndex(client.Client.Client)
	refs, err := si.FindAllByUuid(ctx, dc, uuid, vmSearchArgValue(args), instanceUUIDArgValue(args))
	if err != nil {
		return "", fmt.Errorf("failed to find all by UUID %q: %w", uuid, err)
	}

	return marshalJSON(searchIndexReferenceListJSON(ctx, client, refs))
}

func handleSearchIndexFindByInventoryPath(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	path, _ := args["path"].(string)
	if path == "" {
		return "", fmt.Errorf("path is required")
	}

	si := object.NewSearchIndex(client.Client.Client)
	ref, err := si.FindByInventoryPath(ctx, path)
	if err != nil {
		return "", fmt.Errorf("failed to find by inventory path %q: %w", path, err)
	}

	return marshalJSON(searchIndexReferenceJSON(ctx, client, ref))
}

func handleSearchIndexFindChild(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	raw, ok := args["entity"]
	if !ok {
		return "", fmt.Errorf("entity is required")
	}
	var entityRef types.ManagedObjectReference
	if err := decodeJSONArg(raw, &entityRef); err != nil {
		return "", fmt.Errorf("invalid entity: %w", err)
	}
	if entityRef.Type == "" || entityRef.Value == "" {
		return "", fmt.Errorf(`entity must have non-empty "type" and "value" fields`)
	}
	name, _ := args["name"].(string)
	if name == "" {
		return "", fmt.Errorf("name is required")
	}

	si := object.NewSearchIndex(client.Client.Client)
	ref, err := si.FindChild(ctx, entityRef, name)
	if err != nil {
		return "", fmt.Errorf("failed to find child %q: %w", name, err)
	}

	return marshalJSON(searchIndexReferenceJSON(ctx, client, ref))
}

// resolveOptionalDatacenter resolves the optional "datacenter" argument, or
// (nil, nil) when omitted — every SearchIndex method except
// FindByDatastorePath accepts a nil *Datacenter to mean "search every
// datacenter" (see this file's top doc comment for the one exception).
// Reuses generated_datastore_browser.go's resolveDatacenter, which already
// requires a non-empty name.
func resolveOptionalDatacenter(ctx context.Context, client *vmware.Client, args map[string]interface{}) (*object.Datacenter, error) {
	name, _ := args["datacenter"].(string)
	if name == "" {
		return nil, nil
	}
	return resolveDatacenter(ctx, client, name)
}

// Package tools — generated_vcenter_template.go is Fase 8a (Grupo TAG-VC) of
// the codegen plan (".workspace/plans/MCPVMWare2026-08-10-175300-plano-implementacao-tools-mcp.plan.md"
// / "...cobertura-completa-api-codegen.plan.md"), covering govmomi's whole
// vapi/vcenter package used by this fase: referencia/govmomi/vapi/vcenter/
// {vcenter_vmtx,vcenter_ovf}.go — 7 + 3 = 10 tools, one registration function
// (both source files are small enough, and share entity-resolution helpers
// closely enough, to live in a single generated file/register call per the
// orchestrator's brief).
//
// Same REST/JSON architecture note as generated_tags.go: vcenter.Manager
// wraps a *rest.Client, every struct here (Template, DeployTemplate,
// CheckOut, CheckIn, OVF, Deploy, FilterRequest, ...) already carries real
// `json:"..."` tags, so tool arguments are decoded straight into (or built
// field-by-field into) those concrete structs — no generic object/*.go-style
// decode helper needed.
//
// mode=vcenter-only — see generated_tags.go's top doc comment; the whole
// vapi/* domain requires a vCenter Server Appliance (VAMI/VAPI session).
//
// Entity resolution design (a deliberate ergonomic choice beyond the literal
// govmomi signatures, consistent with how every other tool in this project
// addresses inventory objects — see resolveEntityRef's own doc comment in
// generated_authorization.go): Template.SourceVM, CheckIn's vm, and every
// Placement/Target sub-field (folder/resource_pool/host/cluster/network/
// datastore) are exposed here as full inventory paths, resolved via
// resolveEntityRef/resolveEntityRefs, and converted to the raw moref ID
// string the vapi/vcenter wire format actually expects — rather than asking
// a caller to already know an opaque "group-v3"/"resgroup-24"-style ID. The
// one thing NOT given this treatment is the OVF-descriptor-internal fields
// of DeploymentSpec (network_mappings values, storage_mappings values,
// storage_profile_id, vm_config_spec, additional_parameters) — those
// reference OVF-package-internal keys or storage-policy IDs with no
// inventory-path equivalent, so they are decoded verbatim via decodeJSONArg
// (raw IDs, as the wire format itself uses).
//
// Deployed/checked-out/checked-in VM output: DeployTemplateLibraryItem,
// DeployLibraryItem, and CheckOut all return a raw *types.ManagedObjectReference
// from govmomi — vcRefToPath below converts that to a real inventory path via
// find.InventoryPath (same "resolve for a friendlier result, matching this
// project's addressing convention" pattern used throughout Fases 2-7), so the
// tool result names the created VM the same way every other VM-returning tool
// in this project does, not a bare moref.
//
// vcsim support confirmed by directly reading referencia/govmomi/vapi/simulator/
// simulator.go's libraryItemOVF/libraryItemOVFID/libraryItemCreateTemplate/
// libraryItemTemplateID/libraryItemCheckOuts handlers (registered against
// internal.VCenterOVFLibraryItem/VCenterVMTXLibraryItem) — all 10 methods
// reach a genuine simulator implementation, not a 404 stub. Two real gaps
// found while building this file's tests (documented in detail in
// generated_vcenter_template_test.go, not repeated here):
//   - simulator.libraryItemCreateTemplate/libraryItemTemplateID's "deploy"
//     path call handler.cloneVM, which dereferences Placement.Folder
//     unconditionally (nil pointer panic if empty) — every test/tool call
//     that reaches CreateTemplate/DeployTemplateLibraryItem/CheckOut MUST
//     supply placement.folder, not just placement.resource_pool, even though
//     the real vSphere REST API documents Placement fields as all-optional.
//   - simulator.libraryDeploy (used by DeployLibraryItem, and internally by
//     SyncTemplateLibraryItem when Template.SourceVM is empty) reads the
//     item's .ovf file straight off disk via os.ReadFile — an OVF library
//     item created via CreateOVF (from a live VM, not an uploaded package)
//     has no such file, so a real deploy of it always fails server-side with
//     a file-not-found error. This is a vcsim content-upload gap, not a bug
//     in this file — DeployLibraryItem/SyncTemplateLibraryItem are tested
//     only up to "reaches the server cleanly" for that path.
package tools

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/vapi/library"
	"github.com/vmware/govmomi/vapi/vcenter"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

func registerVCenterTemplateTools(r *Registry) {
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}
	libraryItemIDArg := map[string]interface{}{
		"type":        "string",
		"description": "Content Library item ID (e.g. as returned by vmware_vcenter_create_template/vmware_vcenter_create_ovf, or listed by another content-library tool).",
	}
	placementSchema := map[string]interface{}{
		"type":        "object",
		"description": `Placement for the deployed/cloned VM. Each field, if given, is a full inventory path (resolved the same way as this project's other inventory-path arguments), not a raw vCenter object ID. IMPORTANT (vcsim-confirmed requirement, see this file's top doc comment): "folder" must be supplied whenever this argument is used against vcsim, even though the real vSphere REST API documents every Placement field as optional — an empty folder crashes the simulator's clone step.`,
		"properties": map[string]interface{}{
			"folder":        map[string]interface{}{"type": "string", "description": "Inventory path of the destination VM folder."},
			"resource_pool": map[string]interface{}{"type": "string", "description": "Inventory path of the destination resource pool."},
			"host":          map[string]interface{}{"type": "string", "description": "Inventory path of the destination host."},
			"cluster":       map[string]interface{}{"type": "string", "description": "Inventory path of the destination cluster."},
			"network":       map[string]interface{}{"type": "string", "description": "Inventory path of the destination network."},
		},
	}
	diskStorageSchema := map[string]interface{}{
		"type":        "object",
		"description": "Storage location for VM disk(s).",
		"properties": map[string]interface{}{
			"datastore": map[string]interface{}{"type": "string", "description": "Inventory path of the destination datastore."},
			"storage_policy": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"policy": map[string]interface{}{"type": "string", "description": "Storage policy ID."},
					"type":   map[string]interface{}{"type": "string", "description": `Storage policy type, e.g. "USE_SPECIFIED_POLICY" or "USE_SOURCE_POLICY".`},
				},
			},
		},
	}
	diskStorageOverridesSchema := map[string]interface{}{
		"type":        "array",
		"description": "Per-disk storage overrides.",
		"items": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"key":   map[string]interface{}{"type": "string", "description": "Disk key (as reported by the source VM's virtual hardware)."},
				"value": diskStorageSchema,
			},
			"required": []interface{}{"key", "value"},
		},
	}
	targetSchema := map[string]interface{}{
		"type":        "object",
		"description": "Deployment target. Each field, if given, is a full inventory path, not a raw vCenter object ID.",
		"properties": map[string]interface{}{
			"resource_pool": map[string]interface{}{"type": "string", "description": "Inventory path of the destination resource pool."},
			"host":          map[string]interface{}{"type": "string", "description": "Inventory path of the destination host."},
			"folder":        map[string]interface{}{"type": "string", "description": "Inventory path of the destination VM folder."},
		},
	}
	deploymentSpecSchema := map[string]interface{}{
		"type":        "object",
		"description": `OVF deployment spec. "default_datastore" (an inventory path, resolved automatically) is this project's friendlier alternative to the raw "default_datastore_id" field — give at most one. Every other field (network_mappings, storage_mappings, additional_parameters, storage_provisioning, storage_profile_id, locale, flags, vm_config_spec) is passed through verbatim to vcenter.DeploymentSpec — these reference OVF-package-internal keys or storage-policy IDs, not inventory paths.`,
		"properties": map[string]interface{}{
			"name":                  map[string]interface{}{"type": "string"},
			"annotation":            map[string]interface{}{"type": "string"},
			"accept_all_EULA":       map[string]interface{}{"type": "boolean"},
			"default_datastore":     map[string]interface{}{"type": "string", "description": "Inventory path of the default datastore — resolved automatically."},
			"default_datastore_id":  map[string]interface{}{"type": "string", "description": "Raw datastore moref ID, used only if default_datastore (the inventory path) is not given."},
			"storage_provisioning":  map[string]interface{}{"type": "string"},
			"storage_profile_id":    map[string]interface{}{"type": "string"},
			"locale":                map[string]interface{}{"type": "string"},
			"flags":                 map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
			"network_mappings":      map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "object"}, "description": `Array of {"key","value"} — key is the OVF network name, value is the raw target Network moref ID.`},
			"storage_mappings":      map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "object"}},
			"additional_parameters": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "object"}},
			"vm_config_spec":        map[string]interface{}{"type": "object"},
		},
	}

	r.registerDestructive("vmware_vcenter_create_template",
		"Create a VM template (VMTX) library item in a Content Library from an existing powered-off VM.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name":                   map[string]interface{}{"type": "string", "description": "Name of the new template item."},
				"description":            map[string]interface{}{"type": "string"},
				"library_id":             map[string]interface{}{"type": "string", "description": "Destination Content Library ID."},
				"source_vm":              map[string]interface{}{"type": "string", "description": "Inventory path of the source VM to template."},
				"placement":              placementSchema,
				"disk_storage":           diskStorageSchema,
				"disk_storage_overrides": diskStorageOverridesSchema,
				"vm_home_storage":        diskStorageSchema,
				"confirm":                confirmArg,
			},
			"required": []interface{}{"name", "library_id", "source_vm", "confirm"},
		},
		Tool{Handler: handleVCenterTemplateCreateTemplate},
	)

	r.register("vmware_vcenter_get_library_template_info",
		"Fetch VM template info (CPU, memory, disks, NICs) for a VMTX library item.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"library_item_id": libraryItemIDArg},
			"required":   []interface{}{"library_item_id"},
		},
		Tool{Handler: handleVCenterTemplateGetLibraryTemplateInfo},
	)

	r.registerDestructive("vmware_vcenter_deploy_template_library_item",
		"Deploy a new VM as a copy of the source VM template contained in a VMTX library item. Returns the inventory path of the deployed VM.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"library_item_id":        libraryItemIDArg,
				"name":                   map[string]interface{}{"type": "string", "description": "Name of the deployed VM."},
				"description":            map[string]interface{}{"type": "string"},
				"placement":              placementSchema,
				"disk_storage":           diskStorageSchema,
				"disk_storage_overrides": diskStorageOverridesSchema,
				"vm_home_storage":        diskStorageSchema,
				"guest_customization": map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{"name": map[string]interface{}{"type": "string", "description": "Name of a pre-existing guest customization spec to apply."}},
				},
				"powered_on": map[string]interface{}{"type": "boolean", "description": "Power on the deployed VM immediately. Default false."},
				"confirm":    confirmArg,
			},
			"required": []interface{}{"library_item_id", "placement", "confirm"},
		},
		Tool{Handler: handleVCenterTemplateDeployTemplateLibraryItem},
	)

	r.registerDestructive("vmware_vcenter_check_out",
		"Check out a VM template library item, cloning it into a new, editable VM. Returns the inventory path of the checked-out VM.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"library_item_id": libraryItemIDArg,
				"name":            map[string]interface{}{"type": "string", "description": "Name of the checked-out VM."},
				"placement":       placementSchema,
				"powered_on":      map[string]interface{}{"type": "boolean", "description": "Power on the checked-out VM immediately. Default false."},
				"confirm":         confirmArg,
			},
			"required": []interface{}{"library_item_id", "placement", "confirm"},
		},
		Tool{Handler: handleVCenterTemplateCheckOut},
	)

	r.registerDestructive("vmware_vcenter_check_in",
		"Check a previously checked-out VM back into its VM template library item, updating the template's content.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"library_item_id": libraryItemIDArg,
				"vm":              map[string]interface{}{"type": "string", "description": "Inventory path of the checked-out VM to check back in."},
				"message":         map[string]interface{}{"type": "string", "description": "Check-in message recorded on the new content version."},
				"confirm":         confirmArg,
			},
			"required": []interface{}{"library_item_id", "vm", "message", "confirm"},
		},
		Tool{Handler: handleVCenterTemplateCheckIn},
	)

	r.registerDestructive("vmware_vcenter_sync_template_library",
		"Convert every (or a given subset of) OVF library item(s) in a source Content Library into VM Template items in a destination Content Library — deploys each OVF, converts the result to a VMTX template item, then deletes the intermediate VM. See this file's top doc comment for a vcsim content-upload gap affecting the underlying deploy step.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"source_library_id":      map[string]interface{}{"type": "string", "description": "Source Content Library ID (holding OVF items)."},
				"destination_library_id": map[string]interface{}{"type": "string", "description": "Destination Content Library ID (VMTX items are created here)."},
				"placement":              targetSchema,
				"item_ids": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Specific source library item IDs to sync. Omit to sync every OVF item in the source library that doesn't already have a same-named item in the destination.",
				},
				"confirm": confirmArg,
			},
			"required": []interface{}{"source_library_id", "destination_library_id", "confirm"},
		},
		Tool{Handler: handleVCenterTemplateSyncTemplateLibrary},
	)

	r.registerDestructive("vmware_vcenter_sync_template_library_item",
		"Deploy a single OVF library item and convert the result into a VM Template (VMTX) library item, deleting the intermediate VM afterward. Lower-level building block behind vmware_vcenter_sync_template_library. See this file's top doc comment for a vcsim content-upload gap affecting the underlying deploy step.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"library_item_id": map[string]interface{}{"type": "string", "description": "Source OVF library item ID to deploy."},
				"deployment_spec": deploymentSpecSchema,
				"target":          targetSchema,
				"template": map[string]interface{}{
					"type":        "object",
					"description": "Same shape as vmware_vcenter_create_template's top-level arguments (name, library_id, placement, disk_storage, disk_storage_overrides, vm_home_storage). source_vm is optional here — if omitted, the OVF is deployed first and the resulting VM used as the source, then destroyed after the template is created.",
					"properties": map[string]interface{}{
						"name":                   map[string]interface{}{"type": "string"},
						"description":            map[string]interface{}{"type": "string"},
						"library_id":             map[string]interface{}{"type": "string"},
						"source_vm":              map[string]interface{}{"type": "string", "description": "Optional inventory path of an already-deployed source VM. Normally omitted (see this argument's parent description)."},
						"placement":              placementSchema,
						"disk_storage":           diskStorageSchema,
						"disk_storage_overrides": diskStorageOverridesSchema,
						"vm_home_storage":        diskStorageSchema,
					},
					"required": []interface{}{"name", "library_id"},
				},
				"confirm": confirmArg,
			},
			"required": []interface{}{"library_item_id", "template", "confirm"},
		},
		Tool{Handler: handleVCenterTemplateSyncTemplateLibraryItem},
	)

	r.registerDestructive("vmware_vcenter_create_ovf",
		"Create an OVF library item in a Content Library from an existing VM.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name":            map[string]interface{}{"type": "string", "description": "Name of the new OVF item."},
				"description":     map[string]interface{}{"type": "string"},
				"flags":           map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
				"source_vm":       map[string]interface{}{"type": "string", "description": "Inventory path of the source VM."},
				"library_id":      map[string]interface{}{"type": "string", "description": "Destination Content Library ID. Give exactly one of library_id/library_item_id."},
				"library_item_id": map[string]interface{}{"type": "string", "description": "Destination Content Library item ID (to overwrite an existing item's content). Give exactly one of library_id/library_item_id — NOTE: not implemented by vcsim (confirmed by reading vapi/simulator/simulator.go's libraryItemOVF handler, which silently no-ops this branch); use library_id for testing against vcsim."},
				"confirm":         confirmArg,
			},
			"required": []interface{}{"name", "source_vm", "confirm"},
		},
		Tool{Handler: handleVCenterTemplateCreateOVF},
	)

	r.registerDestructive("vmware_vcenter_deploy_library_item",
		"Deploy a VM from an OVF library item. Returns the inventory path of the deployed VM. See this file's top doc comment for a vcsim content-upload gap that makes a full deploy fail server-side for OVF items created via vmware_vcenter_create_ovf (no uploaded OVF content) — this tool is expected to reach the server cleanly in that case, not necessarily succeed.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"library_item_id": libraryItemIDArg,
				"deployment_spec": deploymentSpecSchema,
				"target":          targetSchema,
				"confirm":         confirmArg,
			},
			"required": []interface{}{"library_item_id", "confirm"},
		},
		Tool{Handler: handleVCenterTemplateDeployLibraryItem},
	)

	r.registerDestructive("vmware_vcenter_filter_library_item",
		"Preview how an OVF library item would deploy against a given target (EULAs, network/storage mappings, additional parameters) without actually deploying it. Classified tier2 per this fase's curation despite being a read-only preview call — kept as decided, not re-classified here.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"library_item_id": libraryItemIDArg,
				"target":          targetSchema,
				"confirm":         confirmArg,
			},
			"required": []interface{}{"library_item_id", "confirm"},
		},
		Tool{Handler: handleVCenterTemplateFilterLibraryItem},
	)
}

// vcenterTemplateManager returns a vcenter.Manager bound to client's
// VAPI/REST session, logging in lazily via client.REST — same pattern as
// generated_tags.go's tagsManager.
func vcenterTemplateManager(ctx context.Context, client *vmware.Client) (*vcenter.Manager, error) {
	rc, err := client.REST(ctx)
	if err != nil {
		return nil, err
	}
	return vcenter.NewManager(rc), nil
}

// resolvePlacement decodes raw (a JSON object with folder/resource_pool/
// host/cluster/network string keys, each a full inventory path) into a
// *library.Placement — vcenter.Placement is a type alias for library.Placement
// (see vcenter_vmtx.go), so the same struct serves Template/DeployTemplate/
// CheckOut. nil in, nil out (every field optional at this layer; individual
// tools document which combinations vcsim actually requires — see this
// file's top doc comment).
func resolvePlacement(ctx context.Context, client *vmware.Client, raw interface{}) (*library.Placement, error) {
	if raw == nil {
		return nil, nil
	}
	m, ok := raw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("placement must be a JSON object")
	}
	p := &library.Placement{}
	fields := map[string]*string{
		"folder":        &p.Folder,
		"resource_pool": &p.ResourcePool,
		"host":          &p.Host,
		"cluster":       &p.Cluster,
		"network":       &p.Network,
	}
	for key, dst := range fields {
		v, _ := m[key].(string)
		if v == "" {
			continue
		}
		ref, err := resolveEntityRef(ctx, client, v)
		if err != nil {
			return nil, fmt.Errorf("placement.%s: %w", key, err)
		}
		*dst = ref.Value
	}
	return p, nil
}

// resolveDiskStorage decodes raw (a JSON object with a "datastore" inventory
// path and an optional raw "storage_policy") into a *vcenter.DiskStorage.
// nil in, nil out.
func resolveDiskStorage(ctx context.Context, client *vmware.Client, raw interface{}) (*vcenter.DiskStorage, error) {
	if raw == nil {
		return nil, nil
	}
	m, ok := raw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("disk storage must be a JSON object")
	}
	ds := &vcenter.DiskStorage{}
	if v, _ := m["datastore"].(string); v != "" {
		ref, err := resolveEntityRef(ctx, client, v)
		if err != nil {
			return nil, fmt.Errorf("disk storage datastore: %w", err)
		}
		ds.Datastore = ref.Value
	}
	if sp, ok := m["storage_policy"]; ok && sp != nil {
		var pol vcenter.StoragePolicy
		if err := decodeJSONArg(sp, &pol); err != nil {
			return nil, fmt.Errorf("disk storage storage_policy: %w", err)
		}
		ds.StoragePolicy = &pol
	}
	return ds, nil
}

// resolveDiskStorageOverrides decodes raw (a JSON array of {"key","value"}
// objects, value shaped like resolveDiskStorage's argument) into
// []vcenter.DiskStorageOverride.
func resolveDiskStorageOverrides(ctx context.Context, client *vmware.Client, raw interface{}) ([]vcenter.DiskStorageOverride, error) {
	if raw == nil {
		return nil, nil
	}
	arr, ok := raw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("disk_storage_overrides must be a JSON array")
	}
	out := make([]vcenter.DiskStorageOverride, 0, len(arr))
	for i, item := range arr {
		m, ok := item.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("disk_storage_overrides[%d] must be a JSON object", i)
		}
		key, _ := m["key"].(string)
		if key == "" {
			return nil, fmt.Errorf("disk_storage_overrides[%d].key is required", i)
		}
		ds, err := resolveDiskStorage(ctx, client, m["value"])
		if err != nil {
			return nil, fmt.Errorf("disk_storage_overrides[%d]: %w", i, err)
		}
		if ds == nil {
			ds = &vcenter.DiskStorage{}
		}
		out = append(out, vcenter.DiskStorageOverride{Key: key, Value: *ds})
	}
	return out, nil
}

// resolveTarget decodes raw (a JSON object with resource_pool/host/folder
// inventory paths) into a vcenter.Target — used by CreateOVF/DeployLibraryItem/
// FilterLibraryItem/SyncTemplateLibrary(Item), all of which address their
// deployment target via the (unaliased) vcenter.Target type rather than
// library.Placement.
func resolveTarget(ctx context.Context, client *vmware.Client, raw interface{}) (vcenter.Target, error) {
	var t vcenter.Target
	if raw == nil {
		return t, nil
	}
	m, ok := raw.(map[string]interface{})
	if !ok {
		return t, fmt.Errorf("target must be a JSON object")
	}
	fields := map[string]*string{
		"resource_pool": &t.ResourcePoolID,
		"host":          &t.HostID,
		"folder":        &t.FolderID,
	}
	for key, dst := range fields {
		v, _ := m[key].(string)
		if v == "" {
			continue
		}
		ref, err := resolveEntityRef(ctx, client, v)
		if err != nil {
			return t, fmt.Errorf("target.%s: %w", key, err)
		}
		*dst = ref.Value
	}
	return t, nil
}

// deploymentSpecFromArg decodes raw into a vcenter.DeploymentSpec — every
// field is passed through verbatim via decodeJSONArg except
// default_datastore, this project's friendlier inventory-path alternative to
// the raw default_datastore_id field (see this file's top doc comment).
func deploymentSpecFromArg(ctx context.Context, client *vmware.Client, raw interface{}) (vcenter.DeploymentSpec, error) {
	var spec vcenter.DeploymentSpec
	if raw == nil {
		return spec, nil
	}
	if err := decodeJSONArg(raw, &spec); err != nil {
		return spec, fmt.Errorf("deployment_spec: %w", err)
	}
	if m, ok := raw.(map[string]interface{}); ok {
		if v, _ := m["default_datastore"].(string); v != "" {
			ref, err := resolveEntityRef(ctx, client, v)
			if err != nil {
				return spec, fmt.Errorf("deployment_spec.default_datastore: %w", err)
			}
			spec.DefaultDatastoreID = ref.Value
		}
	}
	return spec, nil
}

// buildDeploy assembles a vcenter.Deploy (DeploymentSpec + Target) from the
// deployment_spec/target tool arguments — shared by
// vmware_vcenter_deploy_library_item and vmware_vcenter_sync_template_library_item.
func buildDeploy(ctx context.Context, client *vmware.Client, deploymentSpecRaw, targetRaw interface{}) (vcenter.Deploy, error) {
	spec, err := deploymentSpecFromArg(ctx, client, deploymentSpecRaw)
	if err != nil {
		return vcenter.Deploy{}, err
	}
	target, err := resolveTarget(ctx, client, targetRaw)
	if err != nil {
		return vcenter.Deploy{}, err
	}
	return vcenter.Deploy{DeploymentSpec: spec, Target: target}, nil
}

// templateSpecFromMap builds a vcenter.Template from m — shared by
// vmware_vcenter_create_template (m = the tool's top-level args) and
// vmware_vcenter_sync_template_library_item (m = its nested "template" arg).
// requireSourceVM is true for the former (source_vm is mandatory there) and
// false for the latter (SyncTemplateLibraryItem deploys its own source VM
// from the OVF item when Template.SourceVM is empty — see vcenter_vmtx.go's
// SyncTemplateLibraryItem).
func templateSpecFromMap(ctx context.Context, client *vmware.Client, m map[string]interface{}, requireSourceVM bool) (vcenter.Template, error) {
	var t vcenter.Template

	name, _ := m["name"].(string)
	if name == "" {
		return t, fmt.Errorf("name is required")
	}
	libraryID, _ := m["library_id"].(string)
	if libraryID == "" {
		return t, fmt.Errorf("library_id is required")
	}
	description, _ := m["description"].(string)

	var sourceVMValue string
	sourceVMPath, _ := m["source_vm"].(string)
	if sourceVMPath != "" {
		ref, err := resolveEntityRef(ctx, client, sourceVMPath)
		if err != nil {
			return t, fmt.Errorf("source_vm: %w", err)
		}
		sourceVMValue = ref.Value
	} else if requireSourceVM {
		return t, fmt.Errorf("source_vm is required")
	}

	placement, err := resolvePlacement(ctx, client, m["placement"])
	if err != nil {
		return t, err
	}
	diskStorage, err := resolveDiskStorage(ctx, client, m["disk_storage"])
	if err != nil {
		return t, err
	}
	overrides, err := resolveDiskStorageOverrides(ctx, client, m["disk_storage_overrides"])
	if err != nil {
		return t, err
	}
	vmHomeStorage, err := resolveDiskStorage(ctx, client, m["vm_home_storage"])
	if err != nil {
		return t, err
	}

	return vcenter.Template{
		Description:          description,
		DiskStorage:          diskStorage,
		DiskStorageOverrides: overrides,
		Library:              libraryID,
		Name:                 name,
		Placement:            placement,
		SourceVM:             sourceVMValue,
		VMHomeStorage:        vmHomeStorage,
	}, nil
}

// deployTemplateSpecFromArgs builds a vcenter.DeployTemplate from the tool
// call's top-level args — used by vmware_vcenter_deploy_template_library_item.
func deployTemplateSpecFromArgs(ctx context.Context, client *vmware.Client, args map[string]interface{}) (vcenter.DeployTemplate, error) {
	var d vcenter.DeployTemplate

	name, _ := args["name"].(string)
	description, _ := args["description"].(string)
	poweredOn, _ := args["powered_on"].(bool)

	placement, err := resolvePlacement(ctx, client, args["placement"])
	if err != nil {
		return d, err
	}
	diskStorage, err := resolveDiskStorage(ctx, client, args["disk_storage"])
	if err != nil {
		return d, err
	}
	overrides, err := resolveDiskStorageOverrides(ctx, client, args["disk_storage_overrides"])
	if err != nil {
		return d, err
	}
	vmHomeStorage, err := resolveDiskStorage(ctx, client, args["vm_home_storage"])
	if err != nil {
		return d, err
	}

	var guestCustom *vcenter.GuestCustomization
	if raw, ok := args["guest_customization"]; ok && raw != nil {
		var gc vcenter.GuestCustomization
		if err := decodeJSONArg(raw, &gc); err != nil {
			return d, fmt.Errorf("guest_customization: %w", err)
		}
		guestCustom = &gc
	}

	return vcenter.DeployTemplate{
		Description:          description,
		DiskStorage:          diskStorage,
		DiskStorageOverrides: overrides,
		GuestCustomization:   guestCustom,
		Name:                 name,
		Placement:            placement,
		PoweredOn:            poweredOn,
		VMHomeStorage:        vmHomeStorage,
	}, nil
}

// vcRefToPath converts a deployed/checked-out VM's raw moref (as returned by
// DeployTemplateLibraryItem/DeployLibraryItem/CheckOut) into a real inventory
// path — see this file's top doc comment.
func vcRefToPath(ctx context.Context, client *vmware.Client, ref *types.ManagedObjectReference) (string, error) {
	if ref == nil {
		return "", fmt.Errorf("server returned no VM reference")
	}
	path, err := find.InventoryPath(ctx, client.Client.Client, *ref)
	if err != nil {
		return "", fmt.Errorf("resolve deployed VM %s inventory path: %w", ref.Value, err)
	}
	return path, nil
}

func handleVCenterTemplateCreateTemplate(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := vcenterTemplateManager(ctx, client)
	if err != nil {
		return "", err
	}
	spec, err := templateSpecFromMap(ctx, client, args, true)
	if err != nil {
		return "", err
	}

	id, err := m.CreateTemplate(ctx, spec)
	if err != nil {
		return "", fmt.Errorf("failed to create template %q: %w", spec.Name, err)
	}
	return marshalJSON(map[string]interface{}{"result": "template_created", "library_item_id": id, "name": spec.Name, "library_id": spec.Library})
}

func handleVCenterTemplateGetLibraryTemplateInfo(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := vcenterTemplateManager(ctx, client)
	if err != nil {
		return "", err
	}
	itemID, _ := args["library_item_id"].(string)
	if itemID == "" {
		return "", fmt.Errorf("library_item_id is required")
	}

	info, err := m.GetLibraryTemplateInfo(ctx, itemID)
	if err != nil {
		return "", fmt.Errorf("failed to get template info for %q: %w", itemID, err)
	}
	return marshalJSON(info)
}

func handleVCenterTemplateDeployTemplateLibraryItem(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := vcenterTemplateManager(ctx, client)
	if err != nil {
		return "", err
	}
	itemID, _ := args["library_item_id"].(string)
	if itemID == "" {
		return "", fmt.Errorf("library_item_id is required")
	}
	spec, err := deployTemplateSpecFromArgs(ctx, client, args)
	if err != nil {
		return "", err
	}

	ref, err := m.DeployTemplateLibraryItem(ctx, itemID, spec)
	if err != nil {
		return "", fmt.Errorf("failed to deploy template library item %q: %w", itemID, err)
	}
	path, err := vcRefToPath(ctx, client, ref)
	if err != nil {
		return "", err
	}
	return marshalJSON(map[string]interface{}{"result": "vm_deployed", "library_item_id": itemID, "vm": path})
}

func handleVCenterTemplateCheckOut(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := vcenterTemplateManager(ctx, client)
	if err != nil {
		return "", err
	}
	itemID, _ := args["library_item_id"].(string)
	if itemID == "" {
		return "", fmt.Errorf("library_item_id is required")
	}
	name, _ := args["name"].(string)
	poweredOn, _ := args["powered_on"].(bool)
	placement, err := resolvePlacement(ctx, client, args["placement"])
	if err != nil {
		return "", err
	}

	ref, err := m.CheckOut(ctx, itemID, &vcenter.CheckOut{Name: name, Placement: placement, PoweredOn: poweredOn})
	if err != nil {
		return "", fmt.Errorf("failed to check out library item %q: %w", itemID, err)
	}
	path, err := vcRefToPath(ctx, client, ref)
	if err != nil {
		return "", err
	}
	return marshalJSON(map[string]interface{}{"result": "checked_out", "library_item_id": itemID, "vm": path})
}

func handleVCenterTemplateCheckIn(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := vcenterTemplateManager(ctx, client)
	if err != nil {
		return "", err
	}
	itemID, _ := args["library_item_id"].(string)
	if itemID == "" {
		return "", fmt.Errorf("library_item_id is required")
	}
	vmPath, _ := args["vm"].(string)
	ref, err := resolveEntityRef(ctx, client, vmPath)
	if err != nil {
		return "", err
	}
	message, _ := args["message"].(string)
	if message == "" {
		return "", fmt.Errorf("message is required")
	}

	contentVersion, err := m.CheckIn(ctx, itemID, ref, &vcenter.CheckIn{Message: message})
	if err != nil {
		return "", fmt.Errorf("failed to check in %s to library item %q: %w", vmPath, itemID, err)
	}
	return marshalJSON(map[string]interface{}{"result": "checked_in", "library_item_id": itemID, "vm": vmPath, "content_version": contentVersion})
}

func handleVCenterTemplateSyncTemplateLibrary(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := vcenterTemplateManager(ctx, client)
	if err != nil {
		return "", err
	}
	libM := library.NewManager(m.Client)

	srcID, _ := args["source_library_id"].(string)
	if srcID == "" {
		return "", fmt.Errorf("source_library_id is required")
	}
	dstID, _ := args["destination_library_id"].(string)
	if dstID == "" {
		return "", fmt.Errorf("destination_library_id is required")
	}
	srcLib, err := libM.GetLibraryByID(ctx, srcID)
	if err != nil {
		return "", fmt.Errorf("failed to get source library %q: %w", srcID, err)
	}
	dstLib, err := libM.GetLibraryByID(ctx, dstID)
	if err != nil {
		return "", fmt.Errorf("failed to get destination library %q: %w", dstID, err)
	}
	target, err := resolveTarget(ctx, client, args["placement"])
	if err != nil {
		return "", err
	}

	var items []library.Item
	if raw, ok := args["item_ids"]; ok && raw != nil {
		ids, err := toStringSlice(raw)
		if err != nil {
			return "", fmt.Errorf("invalid item_ids: %w", err)
		}
		for _, id := range ids {
			it, err := libM.GetLibraryItem(ctx, id)
			if err != nil {
				return "", fmt.Errorf("failed to get library item %q: %w", id, err)
			}
			items = append(items, *it)
		}
	}

	tl := vcenter.TemplateLibrary{Source: *srcLib, Destination: *dstLib, Placement: target}
	if err := m.SyncTemplateLibrary(ctx, tl, items...); err != nil {
		return "", fmt.Errorf("failed to sync template library %q -> %q: %w", srcID, dstID, err)
	}
	return marshalJSON(map[string]interface{}{"result": "template_library_synced", "source_library_id": srcID, "destination_library_id": dstID, "item_count": len(items)})
}

func handleVCenterTemplateSyncTemplateLibraryItem(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := vcenterTemplateManager(ctx, client)
	if err != nil {
		return "", err
	}
	libM := library.NewManager(m.Client)

	itemID, _ := args["library_item_id"].(string)
	if itemID == "" {
		return "", fmt.Errorf("library_item_id is required")
	}
	item, err := libM.GetLibraryItem(ctx, itemID)
	if err != nil {
		return "", fmt.Errorf("failed to get library item %q: %w", itemID, err)
	}

	deploy, err := buildDeploy(ctx, client, args["deployment_spec"], args["target"])
	if err != nil {
		return "", err
	}

	tmplRaw, ok := args["template"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("template is required and must be a JSON object")
	}
	tmplSpec, err := templateSpecFromMap(ctx, client, tmplRaw, false)
	if err != nil {
		return "", fmt.Errorf("template: %w", err)
	}

	if err := m.SyncTemplateLibraryItem(ctx, *item, &deploy, &tmplSpec); err != nil {
		return "", fmt.Errorf("failed to sync template library item %q: %w", itemID, err)
	}
	return marshalJSON(map[string]interface{}{"result": "template_library_item_synced", "library_item_id": itemID, "template_name": tmplSpec.Name, "template_library_id": tmplSpec.Library})
}

func handleVCenterTemplateCreateOVF(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := vcenterTemplateManager(ctx, client)
	if err != nil {
		return "", err
	}
	name, _ := args["name"].(string)
	if name == "" {
		return "", fmt.Errorf("name is required")
	}
	sourceVMPath, _ := args["source_vm"].(string)
	if sourceVMPath == "" {
		return "", fmt.Errorf("source_vm is required")
	}
	ref, err := resolveEntityRef(ctx, client, sourceVMPath)
	if err != nil {
		return "", err
	}
	description, _ := args["description"].(string)
	libraryID, _ := args["library_id"].(string)
	libraryItemID, _ := args["library_item_id"].(string)
	if libraryID == "" && libraryItemID == "" {
		return "", fmt.Errorf("one of library_id/library_item_id is required")
	}
	var flags []string
	if raw, ok := args["flags"]; ok && raw != nil {
		flags, err = toStringSlice(raw)
		if err != nil {
			return "", fmt.Errorf("invalid flags: %w", err)
		}
	}

	id, err := m.CreateOVF(ctx, vcenter.OVF{
		Spec:   vcenter.CreateSpec{Name: name, Description: description, Flags: flags},
		Source: vcenter.ResourceID{Type: ref.Type, Value: ref.Value},
		Target: vcenter.LibraryTarget{LibraryID: libraryID, LibraryItemID: libraryItemID},
	})
	if err != nil {
		return "", fmt.Errorf("failed to create OVF %q: %w", name, err)
	}
	return marshalJSON(map[string]interface{}{"result": "ovf_created", "library_item_id": id, "name": name, "source_vm": sourceVMPath})
}

func handleVCenterTemplateDeployLibraryItem(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := vcenterTemplateManager(ctx, client)
	if err != nil {
		return "", err
	}
	itemID, _ := args["library_item_id"].(string)
	if itemID == "" {
		return "", fmt.Errorf("library_item_id is required")
	}
	deploy, err := buildDeploy(ctx, client, args["deployment_spec"], args["target"])
	if err != nil {
		return "", err
	}

	ref, err := m.DeployLibraryItem(ctx, itemID, deploy)
	if err != nil {
		return "", fmt.Errorf("failed to deploy library item %q: %w", itemID, err)
	}
	path, err := vcRefToPath(ctx, client, ref)
	if err != nil {
		return "", err
	}
	return marshalJSON(map[string]interface{}{"result": "vm_deployed", "library_item_id": itemID, "vm": path})
}

func handleVCenterTemplateFilterLibraryItem(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := vcenterTemplateManager(ctx, client)
	if err != nil {
		return "", err
	}
	itemID, _ := args["library_item_id"].(string)
	if itemID == "" {
		return "", fmt.Errorf("library_item_id is required")
	}
	target, err := resolveTarget(ctx, client, args["target"])
	if err != nil {
		return "", err
	}

	resp, err := m.FilterLibraryItem(ctx, itemID, vcenter.FilterRequest{Target: target})
	if err != nil {
		return "", fmt.Errorf("failed to filter library item %q: %w", itemID, err)
	}
	return marshalJSON(resp)
}

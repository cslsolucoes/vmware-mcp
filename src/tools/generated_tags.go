// Package tools — generated_tags.go is Fase 8a (Grupo TAG-VC) of the codegen
// plan (".workspace/plans/MCPVMWare2026-08-10-175300-plano-cobertura-completa-api-codegen.plan.md"),
// covering govmomi's vapi/tags package: referencia/govmomi/vapi/tags/
// {tags,categories,tag_association}.go. 27 tools total.
//
// Architecturally different from every prior fase: Fases 0-7 wrapped
// object/*.go — SOAP/XML types from vim25/types with no native json tags,
// requiring a generic decode helper. vapi/tags wraps a *rest.Client
// (VAPI/JSON, not SOAP) and every struct (Tag, Category, AttachedObjects,
// AttachedTags) already carries real `json:"..."` tags — this file decodes
// tool arguments straight into those concrete structs (or builds them field
// by field from args) and marshals the structs straight back out, no
// generic-decode indirection needed. tagsManager below lazily logs into the
// VAPI/REST session via client.REST(ctx) (added to vmware/client.go
// specifically for this — vAPI is vCenter-only, confirmed unreachable on a
// standalone ESXi host per that method's own doc comment), mirroring
// authorizationManager/library.NewManager-style accessors elsewhere in this
// package.
//
// mode=vcenter-only: the entire vapi/* domain requires a vCenter Server
// Appliance (VAMI/VAPI session) — never valid against a standalone ESXi
// host. See client.REST's doc comment.
//
// Entity resolution: every ref mo.Reference/refs []mo.Reference parameter in
// tag_association.go accepts a full inventory path (or paths) here, resolved
// via resolveEntityRef/resolveEntityRefs from generated_authorization.go
// (Fase 7) — object.SearchIndex.FindByInventoryPath works uniformly for any
// entity kind (VM, Folder, Datastore, Network, ...), so it is reused as-is
// rather than reimplemented (per this project's "reuse, don't duplicate"
// discipline). toMoRefs below adapts the []types.ManagedObjectReference that
// resolveEntityRefs returns into the []mo.Reference shape tag_association.go's
// multi-object methods expect (types.ManagedObjectReference already
// implements mo.Reference — see vim25/types/helpers.go — so this is a plain
// element-wise slice conversion, not resolution work of its own).
//
// tagRefInfo enriches every mo.Reference this package returns (attached
// object lists) with a best-effort inventory_path — same
// "resolve via find.InventoryPath, degrade to null on failure rather than
// failing the whole call" pattern as generated_search_index.go's
// searchIndexReferenceJSON, reused here rather than reinvented.
//
// Tag/category ID-or-name convenience: CreateTag/AttachTag/GetTag/... resolve
// a bare name to an ID internally in govmomi (tags.Manager's private
// isName/tagID helpers), but DeleteTag/UpdateTag/DeleteCategory/UpdateCategory
// do NOT — they use tag.ID/category.ID verbatim in the URL path. Since those
// private helpers aren't exported, resolveTagID/resolveCategoryID below
// replicate the same tiny "starts with urn: ? use as-is : GetTag/GetCategory
// first" check locally, so every vmware_tags_* tool taking a tag_id/
// category_id argument accepts either a name or an ID uniformly — a
// consistency improvement over the raw SDK, not a divergence from it (the
// resolved ID is always what actually goes on the wire).
//
// Curation note carried over from the Fase 0 classifier (per the
// orchestrator's brief — not re-discovered here, not "fixed" here):
// AttachTagToMultipleObjects is tier2 but DetachTag/DetachMultipleTagsFromObject
// are tier1, even though detaching a tag is trivially reversible (re-attach
// the same tagID to the same object). This is a known, previously-reviewed
// inconsistency in the project's own tier classification — left as-is per
// explicit instruction, not a bug introduced here.
package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/vapi/tags"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

func registerTagsTools(r *Registry) {
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}
	tagIDArg := map[string]interface{}{
		"type":        "string",
		"description": `Tag ID (URN, e.g. "urn:vmomi:InventoryServiceTag:...:GLOBAL") or Tag Name — a bare name is resolved to its ID automatically.`,
	}
	tagIDsArg := map[string]interface{}{
		"type":        "array",
		"items":       map[string]interface{}{"type": "string"},
		"description": "One or more tag IDs. Must be non-empty.",
	}
	tagURNsArg := map[string]interface{}{
		"type":        "array",
		"items":       map[string]interface{}{"type": "string"},
		"description": `One or more tag IDs in URN notation (e.g. "urn:vmomi:InventoryServiceTag:...:GLOBAL") — display names are rejected by this batch API. Must be non-empty.`,
	}
	categoryIDArg := map[string]interface{}{
		"type":        "string",
		"description": "Category ID or Category Name — a bare name is resolved to its ID automatically.",
	}
	inventoryPathArg := map[string]interface{}{
		"type":        "string",
		"description": `Full inventory path of the managed entity to tag (e.g. "/DC0/vm/my-vm", "/DC0/host/esx1.example.com", "/DC0"), resolved via SearchIndex.FindByInventoryPath — works for any entity kind, not just VMs.`,
	}
	inventoryPathsArg := map[string]interface{}{
		"type":        "array",
		"items":       map[string]interface{}{"type": "string"},
		"description": "One or more full inventory paths, each resolved the same way as a single inventory_path argument. Must be non-empty.",
	}

	r.registerDestructive("vmware_tags_create_tag",
		"Create a new tag under a category.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name":        map[string]interface{}{"type": "string", "description": "Name of the new tag."},
				"description": map[string]interface{}{"type": "string", "description": "Optional description."},
				"category_id": categoryIDArg,
				"tag_id":      map[string]interface{}{"type": "string", "description": "Optional pre-assigned tag ID (URN). Leave empty to let the server generate one — the normal case."},
				"confirm":     confirmArg,
			},
			"required": []interface{}{"name", "category_id", "confirm"},
		},
		Tool{Handler: handleTagsCreateTag},
	)

	r.registerDestructive("vmware_tags_delete_tag",
		"Delete a tag. Irreversible — detaches it from every object it was attached to.",
		tier1,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"tag_id": tagIDArg, "confirm": confirmArg},
			"required":   []interface{}{"tag_id", "confirm"},
		},
		Tool{Handler: handleTagsDeleteTag},
	)

	r.register("vmware_tags_get_tag",
		"Fetch tag information for a given tag ID or name.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"id": tagIDArg},
			"required":   []interface{}{"id"},
		},
		Tool{Handler: handleTagsGetTag},
	)

	r.register("vmware_tags_get_tag_for_category",
		"Fetch tag information for a given tag ID or name, scoped to a specific category (disambiguates when the same tag name exists in multiple categories).",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"id":       tagIDArg,
				"category": map[string]interface{}{"type": "string", "description": "Category ID or Name to scope the lookup to. If omitted, behaves like vmware_tags_get_tag."},
			},
			"required": []interface{}{"id"},
		},
		Tool{Handler: handleTagsGetTagForCategory},
	)

	r.register("vmware_tags_get_tags",
		"Fetch every tag defined in the system, with full tag information.",
		map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
		Tool{Handler: handleTagsGetTags},
	)

	r.register("vmware_tags_get_tags_for_category",
		"Fetch every tag defined under a given category, with full tag information.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"category_id": categoryIDArg},
			"required":   []interface{}{"category_id"},
		},
		Tool{Handler: handleTagsGetTagsForCategory},
	)

	r.register("vmware_tags_list_tags",
		"List every tag ID defined in the system.",
		map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
		Tool{Handler: handleTagsListTags},
	)

	r.register("vmware_tags_list_tags_for_category",
		"List every tag ID defined under a given category.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"category_id": categoryIDArg},
			"required":   []interface{}{"category_id"},
		},
		Tool{Handler: handleTagsListTagsForCategory},
	)

	r.registerDestructive("vmware_tags_update_tag",
		"Update a tag's name and/or description. Fields left empty are unchanged.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"tag_id":      tagIDArg,
				"name":        map[string]interface{}{"type": "string", "description": "New name. Leave empty to keep the current name."},
				"description": map[string]interface{}{"type": "string", "description": "New description. Leave empty to keep the current description."},
				"confirm":     confirmArg,
			},
			"required": []interface{}{"tag_id", "confirm"},
		},
		Tool{Handler: handleTagsUpdateTag},
	)

	r.registerDestructive("vmware_tags_create_category",
		"Create a new tag category.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name":        map[string]interface{}{"type": "string", "description": "Name of the new category."},
				"description": map[string]interface{}{"type": "string", "description": "Optional description."},
				"cardinality": map[string]interface{}{"type": "string", "enum": []interface{}{"SINGLE", "MULTIPLE"}, "description": `"SINGLE": an object can have at most one tag from this category. "MULTIPLE": an object can have several.`},
				"associable_types": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": `Object types this category's tags may be attached to (e.g. ["VirtualMachine", "Datastore"]). Empty/omitted means any type.`,
				},
				"category_id": map[string]interface{}{"type": "string", "description": "Optional pre-assigned category ID (URN). Leave empty to let the server generate one — the normal case."},
				"confirm":     confirmArg,
			},
			"required": []interface{}{"name", "cardinality", "confirm"},
		},
		Tool{Handler: handleTagsCreateCategory},
	)

	r.registerDestructive("vmware_tags_delete_category",
		"Delete a tag category. Irreversible — also deletes every tag under it.",
		tier1,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"category_id": categoryIDArg, "confirm": confirmArg},
			"required":   []interface{}{"category_id", "confirm"},
		},
		Tool{Handler: handleTagsDeleteCategory},
	)

	r.register("vmware_tags_get_category",
		"Fetch category information for a given category ID or name.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"id": categoryIDArg},
			"required":   []interface{}{"id"},
		},
		Tool{Handler: handleTagsGetCategory},
	)

	r.register("vmware_tags_get_categories",
		"Fetch every category defined in the system, with full category information.",
		map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
		Tool{Handler: handleTagsGetCategories},
	)

	r.register("vmware_tags_list_categories",
		"List every category ID defined in the system.",
		map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
		Tool{Handler: handleTagsListCategories},
	)

	r.registerDestructive("vmware_tags_update_category",
		"Update a category's name, description, cardinality, and/or associable types. Fields left empty/omitted are unchanged. associable_types can only grow (existing entries must be included, in their original order) — the server rejects a request that would remove a type already allowed.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"category_id": categoryIDArg,
				"name":        map[string]interface{}{"type": "string", "description": "New name. Leave empty to keep the current name."},
				"description": map[string]interface{}{"type": "string", "description": "New description. Leave empty to keep the current description."},
				"cardinality": map[string]interface{}{"type": "string", "enum": []interface{}{"SINGLE", "MULTIPLE"}, "description": "New cardinality. Leave empty to keep the current value."},
				"associable_types": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "New associable-types list (must be a superset of the current list, preserving order). Omit to leave unchanged.",
				},
				"confirm": confirmArg,
			},
			"required": []interface{}{"category_id", "confirm"},
		},
		Tool{Handler: handleTagsUpdateCategory},
	)

	r.registerDestructive("vmware_tags_attach_multiple_tags_to_object",
		"Attach multiple tags to a single managed object in one call. Idempotent (already-attached tags are a no-op) but not atomic — a partial failure can leave the object partially tagged.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"tag_ids":        tagURNsArg,
				"inventory_path": inventoryPathArg,
				"confirm":        confirmArg,
			},
			"required": []interface{}{"tag_ids", "inventory_path", "confirm"},
		},
		Tool{Handler: handleTagsAttachMultipleTagsToObject},
	)

	r.registerDestructive("vmware_tags_attach_tag",
		"Attach a single tag to a managed object.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"tag_id":         tagIDArg,
				"inventory_path": inventoryPathArg,
				"confirm":        confirmArg,
			},
			"required": []interface{}{"tag_id", "inventory_path", "confirm"},
		},
		Tool{Handler: handleTagsAttachTag},
	)

	r.registerDestructive("vmware_tags_attach_tag_to_multiple_objects",
		"Attach a single tag to multiple managed objects in one call. Idempotent (already-attached objects are a no-op).",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"tag_id":          tagIDArg,
				"inventory_paths": inventoryPathsArg,
				"confirm":         confirmArg,
			},
			"required": []interface{}{"tag_id", "inventory_paths", "confirm"},
		},
		Tool{Handler: handleTagsAttachTagToMultipleObjects},
	)

	r.registerDestructive("vmware_tags_detach_multiple_tags_from_object",
		"Detach multiple tags from a single managed object in one call. Idempotent (already-detached tags are a no-op) but not atomic. Trivially reversible (re-attach the same tag IDs) — classified tier1 per this project's existing (known, unchanged) tier classification, see this file's top doc comment.",
		tier1,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"tag_ids":        tagURNsArg,
				"inventory_path": inventoryPathArg,
				"confirm":        confirmArg,
			},
			"required": []interface{}{"tag_ids", "inventory_path", "confirm"},
		},
		Tool{Handler: handleTagsDetachMultipleTagsFromObject},
	)

	r.registerDestructive("vmware_tags_detach_tag",
		"Detach a single tag from a managed object. If already detached, this is a no-op (no error). Trivially reversible (re-attach the same tag ID) — classified tier1 per this project's existing (known, unchanged) tier classification, see this file's top doc comment.",
		tier1,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"tag_id":         tagIDArg,
				"inventory_path": inventoryPathArg,
				"confirm":        confirmArg,
			},
			"required": []interface{}{"tag_id", "inventory_path", "confirm"},
		},
		Tool{Handler: handleTagsDetachTag},
	)

	r.register("vmware_tags_get_attached_objects_on_tags",
		"Fetch the objects attached to each of the given tags, plus full tag information for each tag.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"tag_ids": tagIDsArg},
			"required":   []interface{}{"tag_ids"},
		},
		Tool{Handler: handleTagsGetAttachedObjectsOnTags},
	)

	r.register("vmware_tags_get_attached_tags",
		"Fetch the tags attached to a managed object, with full tag information.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"inventory_path": inventoryPathArg},
			"required":   []interface{}{"inventory_path"},
		},
		Tool{Handler: handleTagsGetAttachedTags},
	)

	r.register("vmware_tags_get_attached_tags_on_objects",
		"Fetch the tags attached to each of the given managed objects, with full tag information.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"inventory_paths": inventoryPathsArg},
			"required":   []interface{}{"inventory_paths"},
		},
		Tool{Handler: handleTagsGetAttachedTagsOnObjects},
	)

	r.register("vmware_tags_list_attached_objects",
		"List the objects a given tag is attached to.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"tag_id": tagIDArg},
			"required":   []interface{}{"tag_id"},
		},
		Tool{Handler: handleTagsListAttachedObjects},
	)

	r.register("vmware_tags_list_attached_objects_on_tags",
		"List the objects attached to each of the given tags (IDs only, no tag details — see vmware_tags_get_attached_objects_on_tags for that).",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"tag_ids": tagIDsArg},
			"required":   []interface{}{"tag_ids"},
		},
		Tool{Handler: handleTagsListAttachedObjectsOnTags},
	)

	r.register("vmware_tags_list_attached_tags",
		"List the tag IDs attached to a managed object.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"inventory_path": inventoryPathArg},
			"required":   []interface{}{"inventory_path"},
		},
		Tool{Handler: handleTagsListAttachedTags},
	)

	r.register("vmware_tags_list_attached_tags_on_objects",
		"List the tag IDs attached to each of the given managed objects (IDs only, no tag details — see vmware_tags_get_attached_tags_on_objects for that).",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"inventory_paths": inventoryPathsArg},
			"required":   []interface{}{"inventory_paths"},
		},
		Tool{Handler: handleTagsListAttachedTagsOnObjects},
	)
}

// tagsManager returns a tags.Manager bound to client's VAPI/REST session,
// logging in lazily (and reusing the session afterward) via client.REST —
// see this file's top doc comment.
func tagsManager(ctx context.Context, client *vmware.Client) (*tags.Manager, error) {
	rc, err := client.REST(ctx)
	if err != nil {
		return nil, err
	}
	return tags.NewManager(rc), nil
}

// isTagURN reports whether id looks like a vAPI URN rather than a display
// name — mirrors tags.isName's (unexported) check.
func isTagURN(id string) bool {
	return strings.HasPrefix(id, "urn:")
}

// resolveTagID resolves id (a tag ID or a tag Name) to its real tag ID — see
// this file's top doc comment ("Tag/category ID-or-name convenience").
func resolveTagID(ctx context.Context, m *tags.Manager, id string) (string, error) {
	if isTagURN(id) {
		return id, nil
	}
	t, err := m.GetTag(ctx, id)
	if err != nil {
		return "", fmt.Errorf("resolve tag %q: %w", id, err)
	}
	return t.ID, nil
}

// resolveCategoryID resolves id (a category ID or a category Name) to its
// real category ID — see this file's top doc comment.
func resolveCategoryID(ctx context.Context, m *tags.Manager, id string) (string, error) {
	if isTagURN(id) {
		return id, nil
	}
	c, err := m.GetCategory(ctx, id)
	if err != nil {
		return "", fmt.Errorf("resolve category %q: %w", id, err)
	}
	return c.ID, nil
}

// toMoRefs adapts a []types.ManagedObjectReference (what resolveEntityRefs
// returns) into the []mo.Reference shape tag_association.go's multi-object
// methods expect. types.ManagedObjectReference already implements
// mo.Reference (vim25/types/helpers.go), so this is a plain element-wise
// slice conversion, not resolution work of its own.
func toMoRefs(refs []types.ManagedObjectReference) []mo.Reference {
	out := make([]mo.Reference, len(refs))
	for i := range refs {
		out[i] = refs[i]
	}
	return out
}

// tagRefInfo enriches ref with a best-effort inventory_path — see this
// file's top doc comment. Degrades to a null inventory_path (not a failed
// call) when the object no longer resolves, e.g. deleted since being
// tagged — same discipline as generated_search_index.go's
// searchIndexReferenceJSON.
func tagRefInfo(ctx context.Context, client *vmware.Client, ref mo.Reference) map[string]interface{} {
	moref := ref.Reference()
	result := map[string]interface{}{"type": moref.Type, "value": moref.Value}
	if path, err := find.InventoryPath(ctx, client.Client.Client, moref); err == nil {
		result["inventory_path"] = path
	} else {
		result["inventory_path"] = nil
	}
	return result
}

// tagRefInfoList applies tagRefInfo to every element of refs.
func tagRefInfoList(ctx context.Context, client *vmware.Client, refs []mo.Reference) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(refs))
	for _, ref := range refs {
		out = append(out, tagRefInfo(ctx, client, ref))
	}
	return out
}

func handleTagsCreateTag(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := tagsManager(ctx, client)
	if err != nil {
		return "", err
	}
	name, _ := args["name"].(string)
	if name == "" {
		return "", fmt.Errorf("name is required")
	}
	categoryID, _ := args["category_id"].(string)
	if categoryID == "" {
		return "", fmt.Errorf("category_id is required")
	}
	description, _ := args["description"].(string)
	presetID, _ := args["tag_id"].(string)

	id, err := m.CreateTag(ctx, &tags.Tag{Name: name, Description: description, CategoryID: categoryID, TagID: presetID})
	if err != nil {
		return "", fmt.Errorf("failed to create tag %q: %w", name, err)
	}
	return marshalJSON(map[string]interface{}{"result": "tag_created", "tag_id": id, "name": name, "category_id": categoryID})
}

func handleTagsDeleteTag(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := tagsManager(ctx, client)
	if err != nil {
		return "", err
	}
	rawID, _ := args["tag_id"].(string)
	if rawID == "" {
		return "", fmt.Errorf("tag_id is required")
	}
	id, err := resolveTagID(ctx, m, rawID)
	if err != nil {
		return "", err
	}

	if err := m.DeleteTag(ctx, &tags.Tag{ID: id}); err != nil {
		return "", fmt.Errorf("failed to delete tag %q: %w", id, err)
	}
	return marshalJSON(map[string]interface{}{"result": "tag_deleted", "tag_id": id})
}

func handleTagsGetTag(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := tagsManager(ctx, client)
	if err != nil {
		return "", err
	}
	id, _ := args["id"].(string)
	if id == "" {
		return "", fmt.Errorf("id is required")
	}

	t, err := m.GetTag(ctx, id)
	if err != nil {
		return "", fmt.Errorf("failed to get tag %q: %w", id, err)
	}
	return marshalJSON(t)
}

func handleTagsGetTagForCategory(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := tagsManager(ctx, client)
	if err != nil {
		return "", err
	}
	id, _ := args["id"].(string)
	if id == "" {
		return "", fmt.Errorf("id is required")
	}
	category, _ := args["category"].(string)

	t, err := m.GetTagForCategory(ctx, id, category)
	if err != nil {
		return "", fmt.Errorf("failed to get tag %q for category %q: %w", id, category, err)
	}
	return marshalJSON(t)
}

func handleTagsGetTags(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := tagsManager(ctx, client)
	if err != nil {
		return "", err
	}
	list, err := m.GetTags(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get tags: %w", err)
	}
	return marshalJSON(map[string]interface{}{"count": len(list), "tags": list})
}

func handleTagsGetTagsForCategory(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := tagsManager(ctx, client)
	if err != nil {
		return "", err
	}
	categoryID, _ := args["category_id"].(string)
	if categoryID == "" {
		return "", fmt.Errorf("category_id is required")
	}
	list, err := m.GetTagsForCategory(ctx, categoryID)
	if err != nil {
		return "", fmt.Errorf("failed to get tags for category %q: %w", categoryID, err)
	}
	return marshalJSON(map[string]interface{}{"count": len(list), "tags": list})
}

func handleTagsListTags(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := tagsManager(ctx, client)
	if err != nil {
		return "", err
	}
	ids, err := m.ListTags(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to list tags: %w", err)
	}
	return marshalJSON(map[string]interface{}{"count": len(ids), "tag_ids": ids})
}

func handleTagsListTagsForCategory(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := tagsManager(ctx, client)
	if err != nil {
		return "", err
	}
	categoryID, _ := args["category_id"].(string)
	if categoryID == "" {
		return "", fmt.Errorf("category_id is required")
	}
	ids, err := m.ListTagsForCategory(ctx, categoryID)
	if err != nil {
		return "", fmt.Errorf("failed to list tags for category %q: %w", categoryID, err)
	}
	return marshalJSON(map[string]interface{}{"count": len(ids), "tag_ids": ids})
}

func handleTagsUpdateTag(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := tagsManager(ctx, client)
	if err != nil {
		return "", err
	}
	rawID, _ := args["tag_id"].(string)
	if rawID == "" {
		return "", fmt.Errorf("tag_id is required")
	}
	id, err := resolveTagID(ctx, m, rawID)
	if err != nil {
		return "", err
	}
	name, _ := args["name"].(string)
	description, _ := args["description"].(string)

	if err := m.UpdateTag(ctx, &tags.Tag{ID: id, Name: name, Description: description}); err != nil {
		return "", fmt.Errorf("failed to update tag %q: %w", id, err)
	}
	return marshalJSON(map[string]interface{}{"result": "tag_updated", "tag_id": id})
}

func handleTagsCreateCategory(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := tagsManager(ctx, client)
	if err != nil {
		return "", err
	}
	name, _ := args["name"].(string)
	if name == "" {
		return "", fmt.Errorf("name is required")
	}
	cardinality, _ := args["cardinality"].(string)
	if cardinality == "" {
		return "", fmt.Errorf("cardinality is required")
	}
	description, _ := args["description"].(string)
	presetID, _ := args["category_id"].(string)
	var associableTypes []string
	if raw, ok := args["associable_types"]; ok && raw != nil {
		associableTypes, err = toStringSlice(raw)
		if err != nil {
			return "", fmt.Errorf("invalid associable_types: %w", err)
		}
	}

	id, err := m.CreateCategory(ctx, &tags.Category{
		Name:            name,
		Description:     description,
		Cardinality:     cardinality,
		AssociableTypes: associableTypes,
		CategoryID:      presetID,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create category %q: %w", name, err)
	}
	return marshalJSON(map[string]interface{}{"result": "category_created", "category_id": id, "name": name})
}

func handleTagsDeleteCategory(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := tagsManager(ctx, client)
	if err != nil {
		return "", err
	}
	rawID, _ := args["category_id"].(string)
	if rawID == "" {
		return "", fmt.Errorf("category_id is required")
	}
	id, err := resolveCategoryID(ctx, m, rawID)
	if err != nil {
		return "", err
	}

	if err := m.DeleteCategory(ctx, &tags.Category{ID: id}); err != nil {
		return "", fmt.Errorf("failed to delete category %q: %w", id, err)
	}
	return marshalJSON(map[string]interface{}{"result": "category_deleted", "category_id": id})
}

func handleTagsGetCategory(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := tagsManager(ctx, client)
	if err != nil {
		return "", err
	}
	id, _ := args["id"].(string)
	if id == "" {
		return "", fmt.Errorf("id is required")
	}

	c, err := m.GetCategory(ctx, id)
	if err != nil {
		return "", fmt.Errorf("failed to get category %q: %w", id, err)
	}
	return marshalJSON(c)
}

func handleTagsGetCategories(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := tagsManager(ctx, client)
	if err != nil {
		return "", err
	}
	list, err := m.GetCategories(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get categories: %w", err)
	}
	return marshalJSON(map[string]interface{}{"count": len(list), "categories": list})
}

func handleTagsListCategories(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := tagsManager(ctx, client)
	if err != nil {
		return "", err
	}
	ids, err := m.ListCategories(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to list categories: %w", err)
	}
	return marshalJSON(map[string]interface{}{"count": len(ids), "category_ids": ids})
}

func handleTagsUpdateCategory(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := tagsManager(ctx, client)
	if err != nil {
		return "", err
	}
	rawID, _ := args["category_id"].(string)
	if rawID == "" {
		return "", fmt.Errorf("category_id is required")
	}
	id, err := resolveCategoryID(ctx, m, rawID)
	if err != nil {
		return "", err
	}
	name, _ := args["name"].(string)
	description, _ := args["description"].(string)
	cardinality, _ := args["cardinality"].(string)
	var associableTypes []string
	if raw, ok := args["associable_types"]; ok && raw != nil {
		associableTypes, err = toStringSlice(raw)
		if err != nil {
			return "", fmt.Errorf("invalid associable_types: %w", err)
		}
	}

	if err := m.UpdateCategory(ctx, &tags.Category{
		ID:              id,
		Name:            name,
		Description:     description,
		Cardinality:     cardinality,
		AssociableTypes: associableTypes,
	}); err != nil {
		return "", fmt.Errorf("failed to update category %q: %w", id, err)
	}
	return marshalJSON(map[string]interface{}{"result": "category_updated", "category_id": id})
}

func handleTagsAttachMultipleTagsToObject(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := tagsManager(ctx, client)
	if err != nil {
		return "", err
	}
	tagIDs, err := toStringSlice(args["tag_ids"])
	if err != nil {
		return "", fmt.Errorf("invalid tag_ids: %w", err)
	}
	if len(tagIDs) == 0 {
		return "", fmt.Errorf("tag_ids is required and must be non-empty")
	}
	path, _ := args["inventory_path"].(string)
	ref, err := resolveEntityRef(ctx, client, path)
	if err != nil {
		return "", err
	}

	if err := m.AttachMultipleTagsToObject(ctx, tagIDs, ref); err != nil {
		return "", fmt.Errorf("failed to attach tags to %s: %w", path, err)
	}
	return marshalJSON(map[string]interface{}{"result": "tags_attached", "tag_ids": tagIDs, "inventory_path": path})
}

func handleTagsAttachTag(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := tagsManager(ctx, client)
	if err != nil {
		return "", err
	}
	tagID, _ := args["tag_id"].(string)
	if tagID == "" {
		return "", fmt.Errorf("tag_id is required")
	}
	path, _ := args["inventory_path"].(string)
	ref, err := resolveEntityRef(ctx, client, path)
	if err != nil {
		return "", err
	}

	if err := m.AttachTag(ctx, tagID, ref); err != nil {
		return "", fmt.Errorf("failed to attach tag %q to %s: %w", tagID, path, err)
	}
	return marshalJSON(map[string]interface{}{"result": "tag_attached", "tag_id": tagID, "inventory_path": path})
}

func handleTagsAttachTagToMultipleObjects(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := tagsManager(ctx, client)
	if err != nil {
		return "", err
	}
	tagID, _ := args["tag_id"].(string)
	if tagID == "" {
		return "", fmt.Errorf("tag_id is required")
	}
	paths, err := toStringSlice(args["inventory_paths"])
	if err != nil {
		return "", fmt.Errorf("invalid inventory_paths: %w", err)
	}
	refs, err := resolveEntityRefs(ctx, client, paths)
	if err != nil {
		return "", err
	}

	if err := m.AttachTagToMultipleObjects(ctx, tagID, toMoRefs(refs)); err != nil {
		return "", fmt.Errorf("failed to attach tag %q to multiple objects: %w", tagID, err)
	}
	return marshalJSON(map[string]interface{}{"result": "tag_attached", "tag_id": tagID, "inventory_paths": paths})
}

func handleTagsDetachMultipleTagsFromObject(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := tagsManager(ctx, client)
	if err != nil {
		return "", err
	}
	tagIDs, err := toStringSlice(args["tag_ids"])
	if err != nil {
		return "", fmt.Errorf("invalid tag_ids: %w", err)
	}
	if len(tagIDs) == 0 {
		return "", fmt.Errorf("tag_ids is required and must be non-empty")
	}
	path, _ := args["inventory_path"].(string)
	ref, err := resolveEntityRef(ctx, client, path)
	if err != nil {
		return "", err
	}

	if err := m.DetachMultipleTagsFromObject(ctx, tagIDs, ref); err != nil {
		return "", fmt.Errorf("failed to detach tags from %s: %w", path, err)
	}
	return marshalJSON(map[string]interface{}{"result": "tags_detached", "tag_ids": tagIDs, "inventory_path": path})
}

func handleTagsDetachTag(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := tagsManager(ctx, client)
	if err != nil {
		return "", err
	}
	tagID, _ := args["tag_id"].(string)
	if tagID == "" {
		return "", fmt.Errorf("tag_id is required")
	}
	path, _ := args["inventory_path"].(string)
	ref, err := resolveEntityRef(ctx, client, path)
	if err != nil {
		return "", err
	}

	if err := m.DetachTag(ctx, tagID, ref); err != nil {
		return "", fmt.Errorf("failed to detach tag %q from %s: %w", tagID, path, err)
	}
	return marshalJSON(map[string]interface{}{"result": "tag_detached", "tag_id": tagID, "inventory_path": path})
}

func handleTagsGetAttachedObjectsOnTags(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := tagsManager(ctx, client)
	if err != nil {
		return "", err
	}
	tagIDs, err := toStringSlice(args["tag_ids"])
	if err != nil {
		return "", fmt.Errorf("invalid tag_ids: %w", err)
	}
	if len(tagIDs) == 0 {
		return "", fmt.Errorf("tag_ids is required and must be non-empty")
	}

	list, err := m.GetAttachedObjectsOnTags(ctx, tagIDs)
	if err != nil {
		return "", fmt.Errorf("failed to get attached objects on tags: %w", err)
	}

	results := make([]map[string]interface{}, 0, len(list))
	for _, entry := range list {
		objs := make([]mo.Reference, len(entry.ObjectIDs))
		copy(objs, entry.ObjectIDs)
		results = append(results, map[string]interface{}{
			"tag_id":  entry.TagID,
			"tag":     entry.Tag,
			"objects": tagRefInfoList(ctx, client, objs),
		})
	}
	return marshalJSON(map[string]interface{}{"count": len(results), "results": results})
}

func handleTagsGetAttachedTags(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := tagsManager(ctx, client)
	if err != nil {
		return "", err
	}
	path, _ := args["inventory_path"].(string)
	ref, err := resolveEntityRef(ctx, client, path)
	if err != nil {
		return "", err
	}

	list, err := m.GetAttachedTags(ctx, ref)
	if err != nil {
		return "", fmt.Errorf("failed to get attached tags for %s: %w", path, err)
	}
	return marshalJSON(map[string]interface{}{"inventory_path": path, "count": len(list), "tags": list})
}

func handleTagsGetAttachedTagsOnObjects(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := tagsManager(ctx, client)
	if err != nil {
		return "", err
	}
	paths, err := toStringSlice(args["inventory_paths"])
	if err != nil {
		return "", fmt.Errorf("invalid inventory_paths: %w", err)
	}
	refs, err := resolveEntityRefs(ctx, client, paths)
	if err != nil {
		return "", err
	}

	list, err := m.GetAttachedTagsOnObjects(ctx, toMoRefs(refs))
	if err != nil {
		return "", fmt.Errorf("failed to get attached tags on objects: %w", err)
	}

	results := make([]map[string]interface{}, 0, len(list))
	for _, entry := range list {
		results = append(results, map[string]interface{}{
			"object":  tagRefInfo(ctx, client, entry.ObjectID),
			"tag_ids": entry.TagIDs,
			"tags":    entry.Tags,
		})
	}
	return marshalJSON(map[string]interface{}{"count": len(results), "results": results})
}

func handleTagsListAttachedObjects(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := tagsManager(ctx, client)
	if err != nil {
		return "", err
	}
	tagID, _ := args["tag_id"].(string)
	if tagID == "" {
		return "", fmt.Errorf("tag_id is required")
	}

	refs, err := m.ListAttachedObjects(ctx, tagID)
	if err != nil {
		return "", fmt.Errorf("failed to list attached objects for tag %q: %w", tagID, err)
	}
	return marshalJSON(map[string]interface{}{"tag_id": tagID, "count": len(refs), "objects": tagRefInfoList(ctx, client, refs)})
}

func handleTagsListAttachedObjectsOnTags(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := tagsManager(ctx, client)
	if err != nil {
		return "", err
	}
	tagIDs, err := toStringSlice(args["tag_ids"])
	if err != nil {
		return "", fmt.Errorf("invalid tag_ids: %w", err)
	}
	if len(tagIDs) == 0 {
		return "", fmt.Errorf("tag_ids is required and must be non-empty")
	}

	list, err := m.ListAttachedObjectsOnTags(ctx, tagIDs)
	if err != nil {
		return "", fmt.Errorf("failed to list attached objects on tags: %w", err)
	}

	results := make([]map[string]interface{}, 0, len(list))
	for _, entry := range list {
		objs := make([]mo.Reference, len(entry.ObjectIDs))
		copy(objs, entry.ObjectIDs)
		results = append(results, map[string]interface{}{
			"tag_id":  entry.TagID,
			"objects": tagRefInfoList(ctx, client, objs),
		})
	}
	return marshalJSON(map[string]interface{}{"count": len(results), "results": results})
}

func handleTagsListAttachedTags(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := tagsManager(ctx, client)
	if err != nil {
		return "", err
	}
	path, _ := args["inventory_path"].(string)
	ref, err := resolveEntityRef(ctx, client, path)
	if err != nil {
		return "", err
	}

	ids, err := m.ListAttachedTags(ctx, ref)
	if err != nil {
		return "", fmt.Errorf("failed to list attached tags for %s: %w", path, err)
	}
	return marshalJSON(map[string]interface{}{"inventory_path": path, "count": len(ids), "tag_ids": ids})
}

func handleTagsListAttachedTagsOnObjects(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := tagsManager(ctx, client)
	if err != nil {
		return "", err
	}
	paths, err := toStringSlice(args["inventory_paths"])
	if err != nil {
		return "", fmt.Errorf("invalid inventory_paths: %w", err)
	}
	refs, err := resolveEntityRefs(ctx, client, paths)
	if err != nil {
		return "", err
	}

	list, err := m.ListAttachedTagsOnObjects(ctx, toMoRefs(refs))
	if err != nil {
		return "", fmt.Errorf("failed to list attached tags on objects: %w", err)
	}

	results := make([]map[string]interface{}, 0, len(list))
	for _, entry := range list {
		results = append(results, map[string]interface{}{
			"object":  tagRefInfo(ctx, client, entry.ObjectID),
			"tag_ids": entry.TagIDs,
		})
	}
	return marshalJSON(map[string]interface{}{"count": len(results), "results": results})
}

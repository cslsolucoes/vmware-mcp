package tools

import (
	"context"
	"testing"

	"github.com/vmware/govmomi/simulator"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// newTagsRegistry builds a Registry the normal way (NewRegistry, which wires
// vm.go/host.go/etc via registerTools) and then manually layers
// registerTagsTools on top via withClass — same pattern as
// generated_authorization_test.go/generated_resourcepool_vapp_test.go, and
// for the same reason: this file must not edit registry.go (see
// generated_tags.go's top doc comment).
func newTagsRegistry(ctx context.Context, c *vmware.Client, opts RegistryOptions) *Registry {
	r := NewRegistry(ctx, c, opts)
	r.withClass(modeVCenterOnly, registerTagsTools)
	return r
}

// TestTagsTools_Registration proves all 27 tools are registered and
// reachable via ListTools.
func TestTagsTools_Registration(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newTagsRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	want := []string{
		"vmware_tags_create_tag",
		"vmware_tags_delete_tag",
		"vmware_tags_get_tag",
		"vmware_tags_get_tag_for_category",
		"vmware_tags_get_tags",
		"vmware_tags_get_tags_for_category",
		"vmware_tags_list_tags",
		"vmware_tags_list_tags_for_category",
		"vmware_tags_update_tag",
		"vmware_tags_create_category",
		"vmware_tags_delete_category",
		"vmware_tags_get_category",
		"vmware_tags_get_categories",
		"vmware_tags_list_categories",
		"vmware_tags_update_category",
		"vmware_tags_attach_multiple_tags_to_object",
		"vmware_tags_attach_tag",
		"vmware_tags_attach_tag_to_multiple_objects",
		"vmware_tags_detach_multiple_tags_from_object",
		"vmware_tags_detach_tag",
		"vmware_tags_get_attached_objects_on_tags",
		"vmware_tags_get_attached_tags",
		"vmware_tags_get_attached_tags_on_objects",
		"vmware_tags_list_attached_objects",
		"vmware_tags_list_attached_objects_on_tags",
		"vmware_tags_list_attached_tags",
		"vmware_tags_list_attached_tags_on_objects",
	}
	if len(want) != 27 {
		t.Fatalf("test bug: want list has %d entries, expected 27", len(want))
	}

	got := map[string]bool{}
	for _, tl := range r.ListTools() {
		got[tl.Name] = true
	}
	for _, name := range want {
		if !got[name] {
			t.Errorf("tool %s not registered", name)
		}
	}
}

// tagsListVMPaths returns every VM inventory path the simulator model has —
// used by the multi-object association tests below, which need 2 distinct
// VMs. simulator.VPX()'s default model (Host:1, Cluster:1, ClusterHost:3,
// Machine:2) has 8 VMs, well above the 2 these tests need.
func tagsListVMPaths(t *testing.T, r *Registry) []string {
	t.Helper()
	raw, err := r.CallTool("vmware_list_vms", map[string]interface{}{})
	if err != nil {
		t.Fatalf("vmware_list_vms failed: %v", err)
	}
	list, _ := decodeResult(t, raw)["vms"].([]interface{})
	out := make([]string, 0, len(list))
	for _, v := range list {
		out = append(out, v.(string))
	}
	return out
}

// TestTagsTools_CategoryLifecycle exercises CreateCategory -> GetCategory ->
// ListCategories -> UpdateCategory -> DeleteCategory end to end against a
// real vcsim tags.Manager.
func TestTagsTools_CategoryLifecycle(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newTagsRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	raw, err := r.CallTool("vmware_tags_create_category", map[string]interface{}{
		"name":        "TestCategory",
		"description": "created by TestTagsTools_CategoryLifecycle",
		"cardinality": "MULTIPLE",
		"confirm":     true,
	})
	if err != nil {
		t.Fatalf("vmware_tags_create_category failed: %v (%s)", err, raw)
	}
	catID, _ := decodeResult(t, raw)["category_id"].(string)
	if catID == "" {
		t.Fatalf("expected a non-empty category_id in %s", raw)
	}

	raw, err = r.CallTool("vmware_tags_get_category", map[string]interface{}{"id": catID})
	if err != nil {
		t.Fatalf("vmware_tags_get_category failed: %v (%s)", err, raw)
	}
	if name, _ := decodeResult(t, raw)["name"].(string); name != "TestCategory" {
		t.Fatalf("expected name=TestCategory, got %s", raw)
	}

	raw, err = r.CallTool("vmware_tags_list_categories", map[string]interface{}{})
	if err != nil {
		t.Fatalf("vmware_tags_list_categories failed: %v", err)
	}
	if !stringListHas(t, raw, "category_ids", catID) {
		t.Fatalf("expected %s in category_ids: %s", catID, raw)
	}

	raw, err = r.CallTool("vmware_tags_update_category", map[string]interface{}{
		"category_id": catID,
		"name":        "TestCategoryRenamed",
		"confirm":     true,
	})
	if err != nil {
		t.Fatalf("vmware_tags_update_category failed: %v (%s)", err, raw)
	}

	raw, err = r.CallTool("vmware_tags_get_category", map[string]interface{}{"id": catID})
	if err != nil {
		t.Fatalf("vmware_tags_get_category (after rename) failed: %v", err)
	}
	if name, _ := decodeResult(t, raw)["name"].(string); name != "TestCategoryRenamed" {
		t.Fatalf("expected name=TestCategoryRenamed after rename, got %s", raw)
	}

	raw, err = r.CallTool("vmware_tags_delete_category", map[string]interface{}{"category_id": catID, "confirm": true})
	if err != nil {
		t.Fatalf("vmware_tags_delete_category failed: %v (%s)", err, raw)
	}

	raw, err = r.CallTool("vmware_tags_list_categories", map[string]interface{}{})
	if err != nil {
		t.Fatalf("vmware_tags_list_categories (after delete) failed: %v", err)
	}
	if stringListHas(t, raw, "category_ids", catID) {
		t.Fatalf("expected %s to be removed from category_ids, still present: %s", catID, raw)
	}
}

// stringListHas reports whether raw's field key (a JSON array of strings)
// contains want.
func stringListHas(t *testing.T, raw string, field, want string) bool {
	t.Helper()
	list, _ := decodeResult(t, raw)[field].([]interface{})
	for _, v := range list {
		if s, ok := v.(string); ok && s == want {
			return true
		}
	}
	return false
}

// TestTagsTools_TagLifecycle exercises CreateTag -> GetTag -> ListTags ->
// GetTagsForCategory -> ListTagsForCategory -> UpdateTag -> DeleteTag end to
// end, cleaning up the backing category afterward.
func TestTagsTools_TagLifecycle(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newTagsRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	raw, err := r.CallTool("vmware_tags_create_category", map[string]interface{}{
		"name": "TagLifecycleCategory", "cardinality": "MULTIPLE", "confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_tags_create_category failed: %v", err)
	}
	catID, _ := decodeResult(t, raw)["category_id"].(string)

	raw, err = r.CallTool("vmware_tags_create_tag", map[string]interface{}{
		"name": "TestTag", "description": "created by TestTagsTools_TagLifecycle", "category_id": catID, "confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_tags_create_tag failed: %v (%s)", err, raw)
	}
	tagID, _ := decodeResult(t, raw)["tag_id"].(string)
	if tagID == "" {
		t.Fatalf("expected a non-empty tag_id in %s", raw)
	}

	raw, err = r.CallTool("vmware_tags_get_tag", map[string]interface{}{"id": tagID})
	if err != nil {
		t.Fatalf("vmware_tags_get_tag failed: %v (%s)", err, raw)
	}
	if name, _ := decodeResult(t, raw)["name"].(string); name != "TestTag" {
		t.Fatalf("expected name=TestTag, got %s", raw)
	}

	raw, err = r.CallTool("vmware_tags_list_tags", map[string]interface{}{})
	if err != nil {
		t.Fatalf("vmware_tags_list_tags failed: %v", err)
	}
	if !stringListHas(t, raw, "tag_ids", tagID) {
		t.Fatalf("expected %s in tag_ids: %s", tagID, raw)
	}

	raw, err = r.CallTool("vmware_tags_list_tags_for_category", map[string]interface{}{"category_id": catID})
	if err != nil {
		t.Fatalf("vmware_tags_list_tags_for_category failed: %v", err)
	}
	if !stringListHas(t, raw, "tag_ids", tagID) {
		t.Fatalf("expected %s in tag_ids for category: %s", tagID, raw)
	}

	raw, err = r.CallTool("vmware_tags_get_tags_for_category", map[string]interface{}{"category_id": catID})
	if err != nil {
		t.Fatalf("vmware_tags_get_tags_for_category failed: %v", err)
	}
	if count, _ := decodeResult(t, raw)["count"].(float64); count != 1 {
		t.Fatalf("expected count=1, got %v: %s", count, raw)
	}

	raw, err = r.CallTool("vmware_tags_get_tag_for_category", map[string]interface{}{"id": "TestTag", "category": catID})
	if err != nil {
		t.Fatalf("vmware_tags_get_tag_for_category failed: %v (%s)", err, raw)
	}
	if id, _ := decodeResult(t, raw)["id"].(string); id != tagID {
		t.Fatalf("expected id=%s, got %s", tagID, raw)
	}

	raw, err = r.CallTool("vmware_tags_update_tag", map[string]interface{}{
		"tag_id": tagID, "description": "renamed by TestTagsTools_TagLifecycle", "confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_tags_update_tag failed: %v (%s)", err, raw)
	}

	raw, err = r.CallTool("vmware_tags_get_tag", map[string]interface{}{"id": tagID})
	if err != nil {
		t.Fatalf("vmware_tags_get_tag (after update) failed: %v", err)
	}
	if desc, _ := decodeResult(t, raw)["description"].(string); desc != "renamed by TestTagsTools_TagLifecycle" {
		t.Fatalf("expected updated description, got %s", raw)
	}

	raw, err = r.CallTool("vmware_tags_delete_tag", map[string]interface{}{"tag_id": tagID, "confirm": true})
	if err != nil {
		t.Fatalf("vmware_tags_delete_tag failed: %v (%s)", err, raw)
	}

	raw, err = r.CallTool("vmware_tags_list_tags", map[string]interface{}{})
	if err != nil {
		t.Fatalf("vmware_tags_list_tags (after delete) failed: %v", err)
	}
	if stringListHas(t, raw, "tag_ids", tagID) {
		t.Fatalf("expected %s to be removed from tag_ids, still present: %s", tagID, raw)
	}

	if _, err := r.CallTool("vmware_tags_delete_category", map[string]interface{}{"category_id": catID, "confirm": true}); err != nil {
		t.Fatalf("cleanup vmware_tags_delete_category failed: %v", err)
	}
}

// TestTagsTools_AttachDetachFlow is the exact lifecycle the orchestrator's
// brief specified: CreateCategory -> CreateTag -> AttachTag (to a real VM
// from the simulator fixture, resolved via resolveEntityRef) ->
// GetAttachedTags -> DetachTag -> DeleteTag -> DeleteCategory. Also exercises
// ListAttachedTags/ListAttachedObjects along the way.
func TestTagsTools_AttachDetachFlow(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newTagsRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	vm := firstVMPath(t, r)

	raw, err := r.CallTool("vmware_tags_create_category", map[string]interface{}{
		"name": "AttachFlowCategory", "cardinality": "MULTIPLE", "confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_tags_create_category failed: %v", err)
	}
	catID, _ := decodeResult(t, raw)["category_id"].(string)

	raw, err = r.CallTool("vmware_tags_create_tag", map[string]interface{}{
		"name": "AttachFlowTag", "category_id": catID, "confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_tags_create_tag failed: %v", err)
	}
	tagID, _ := decodeResult(t, raw)["tag_id"].(string)

	raw, err = r.CallTool("vmware_tags_attach_tag", map[string]interface{}{
		"tag_id": tagID, "inventory_path": vm, "confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_tags_attach_tag failed: %v (%s)", err, raw)
	}

	raw, err = r.CallTool("vmware_tags_get_attached_tags", map[string]interface{}{"inventory_path": vm})
	if err != nil {
		t.Fatalf("vmware_tags_get_attached_tags failed: %v", err)
	}
	if count, _ := decodeResult(t, raw)["count"].(float64); count != 1 {
		t.Fatalf("expected 1 attached tag, got %v: %s", count, raw)
	}

	raw, err = r.CallTool("vmware_tags_list_attached_tags", map[string]interface{}{"inventory_path": vm})
	if err != nil {
		t.Fatalf("vmware_tags_list_attached_tags failed: %v", err)
	}
	if !stringListHas(t, raw, "tag_ids", tagID) {
		t.Fatalf("expected %s in attached tag_ids: %s", tagID, raw)
	}

	raw, err = r.CallTool("vmware_tags_list_attached_objects", map[string]interface{}{"tag_id": tagID})
	if err != nil {
		t.Fatalf("vmware_tags_list_attached_objects failed: %v", err)
	}
	if count, _ := decodeResult(t, raw)["count"].(float64); count != 1 {
		t.Fatalf("expected 1 attached object, got %v: %s", count, raw)
	}
	objs, _ := decodeResult(t, raw)["objects"].([]interface{})
	if len(objs) != 1 {
		t.Fatalf("expected 1 object entry, got %d: %s", len(objs), raw)
	}
	first, _ := objs[0].(map[string]interface{})
	if first["inventory_path"] != vm {
		t.Fatalf("expected object inventory_path=%s, got %v", vm, first["inventory_path"])
	}

	raw, err = r.CallTool("vmware_tags_detach_tag", map[string]interface{}{
		"tag_id": tagID, "inventory_path": vm, "confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_tags_detach_tag failed: %v (%s)", err, raw)
	}

	raw, err = r.CallTool("vmware_tags_get_attached_tags", map[string]interface{}{"inventory_path": vm})
	if err != nil {
		t.Fatalf("vmware_tags_get_attached_tags (after detach) failed: %v", err)
	}
	if count, _ := decodeResult(t, raw)["count"].(float64); count != 0 {
		t.Fatalf("expected 0 attached tags after detach, got %v: %s", count, raw)
	}

	if _, err := r.CallTool("vmware_tags_delete_tag", map[string]interface{}{"tag_id": tagID, "confirm": true}); err != nil {
		t.Fatalf("cleanup vmware_tags_delete_tag failed: %v", err)
	}
	if _, err := r.CallTool("vmware_tags_delete_category", map[string]interface{}{"category_id": catID, "confirm": true}); err != nil {
		t.Fatalf("cleanup vmware_tags_delete_category failed: %v", err)
	}
}

// TestTagsTools_MultiObjectAssociations covers the batch tag_association
// tools not already exercised by TestTagsTools_AttachDetachFlow:
// AttachMultipleTagsToObject, AttachTagToMultipleObjects,
// DetachMultipleTagsFromObject, GetAttachedObjectsOnTags,
// ListAttachedObjectsOnTags, GetAttachedTagsOnObjects,
// ListAttachedTagsOnObjects.
func TestTagsTools_MultiObjectAssociations(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newTagsRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	vms := tagsListVMPaths(t, r)
	if len(vms) < 2 {
		t.Fatalf("test needs at least 2 VMs in the simulator fixture, got %d", len(vms))
	}
	vm1, vm2 := vms[0], vms[1]

	raw, err := r.CallTool("vmware_tags_create_category", map[string]interface{}{
		"name": "MultiObjCategory", "cardinality": "MULTIPLE", "confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_tags_create_category failed: %v", err)
	}
	catID, _ := decodeResult(t, raw)["category_id"].(string)

	createTag := func(name string) string {
		raw, err := r.CallTool("vmware_tags_create_tag", map[string]interface{}{
			"name": name, "category_id": catID, "confirm": true,
		})
		if err != nil {
			t.Fatalf("vmware_tags_create_tag(%s) failed: %v", name, err)
		}
		id, _ := decodeResult(t, raw)["tag_id"].(string)
		if id == "" {
			t.Fatalf("expected a non-empty tag_id for %s: %s", name, raw)
		}
		return id
	}
	tag1 := createTag("MultiObjTag1")
	tag2 := createTag("MultiObjTag2")
	tag3 := createTag("MultiObjTag3")

	// vm1 gets tag1+tag2 via AttachMultipleTagsToObject.
	raw, err = r.CallTool("vmware_tags_attach_multiple_tags_to_object", map[string]interface{}{
		"tag_ids": []interface{}{tag1, tag2}, "inventory_path": vm1, "confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_tags_attach_multiple_tags_to_object failed: %v (%s)", err, raw)
	}

	// tag3 goes on both vm1 and vm2 via AttachTagToMultipleObjects.
	raw, err = r.CallTool("vmware_tags_attach_tag_to_multiple_objects", map[string]interface{}{
		"tag_id": tag3, "inventory_paths": []interface{}{vm1, vm2}, "confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_tags_attach_tag_to_multiple_objects failed: %v (%s)", err, raw)
	}

	// vm1 should now carry 3 tags, vm2 should carry 1.
	raw, err = r.CallTool("vmware_tags_get_attached_tags", map[string]interface{}{"inventory_path": vm1})
	if err != nil {
		t.Fatalf("vmware_tags_get_attached_tags(vm1) failed: %v", err)
	}
	if count, _ := decodeResult(t, raw)["count"].(float64); count != 3 {
		t.Fatalf("expected vm1 to carry 3 tags, got %v: %s", count, raw)
	}

	// Detach tag1+tag2 from vm1, leaving only tag3.
	raw, err = r.CallTool("vmware_tags_detach_multiple_tags_from_object", map[string]interface{}{
		"tag_ids": []interface{}{tag1, tag2}, "inventory_path": vm1, "confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_tags_detach_multiple_tags_from_object failed: %v (%s)", err, raw)
	}
	raw, err = r.CallTool("vmware_tags_list_attached_tags", map[string]interface{}{"inventory_path": vm1})
	if err != nil {
		t.Fatalf("vmware_tags_list_attached_tags(vm1) failed: %v", err)
	}
	if count, _ := decodeResult(t, raw)["count"].(float64); count != 1 {
		t.Fatalf("expected vm1 to carry 1 tag after detach, got %v: %s", count, raw)
	}
	if !stringListHas(t, raw, "tag_ids", tag3) {
		t.Fatalf("expected tag3 (%s) to remain on vm1: %s", tag3, raw)
	}

	// GetAttachedObjectsOnTags/ListAttachedObjectsOnTags([tag3]) should report
	// both vm1 and vm2.
	raw, err = r.CallTool("vmware_tags_list_attached_objects_on_tags", map[string]interface{}{"tag_ids": []interface{}{tag3}})
	if err != nil {
		t.Fatalf("vmware_tags_list_attached_objects_on_tags failed: %v", err)
	}
	results, _ := decodeResult(t, raw)["results"].([]interface{})
	if len(results) != 1 {
		t.Fatalf("expected 1 result entry (1 tag queried), got %d: %s", len(results), raw)
	}
	entry, _ := results[0].(map[string]interface{})
	if entry["tag_id"] != tag3 {
		t.Fatalf("expected tag_id=%s, got %v", tag3, entry["tag_id"])
	}
	objs, _ := entry["objects"].([]interface{})
	if len(objs) != 2 {
		t.Fatalf("expected tag3 to be attached to 2 objects, got %d: %s", len(objs), raw)
	}

	raw, err = r.CallTool("vmware_tags_get_attached_objects_on_tags", map[string]interface{}{"tag_ids": []interface{}{tag3}})
	if err != nil {
		t.Fatalf("vmware_tags_get_attached_objects_on_tags failed: %v", err)
	}
	results, _ = decodeResult(t, raw)["results"].([]interface{})
	if len(results) != 1 {
		t.Fatalf("expected 1 result entry, got %d: %s", len(results), raw)
	}
	entry, _ = results[0].(map[string]interface{})
	tagObj, _ := entry["tag"].(map[string]interface{})
	if tagObj["name"] != "MultiObjTag3" {
		t.Fatalf("expected populated tag info with name=MultiObjTag3, got %v: %s", tagObj, raw)
	}

	// GetAttachedTagsOnObjects/ListAttachedTagsOnObjects([vm1,vm2]) should
	// each report tag3.
	raw, err = r.CallTool("vmware_tags_list_attached_tags_on_objects", map[string]interface{}{"inventory_paths": []interface{}{vm1, vm2}})
	if err != nil {
		t.Fatalf("vmware_tags_list_attached_tags_on_objects failed: %v", err)
	}
	objResults, _ := decodeResult(t, raw)["results"].([]interface{})
	if len(objResults) != 2 {
		t.Fatalf("expected 2 result entries (2 objects queried), got %d: %s", len(objResults), raw)
	}
	for _, r0 := range objResults {
		e, _ := r0.(map[string]interface{})
		tagIDs, _ := e["tag_ids"].([]interface{})
		found := false
		for _, id := range tagIDs {
			if id == tag3 {
				found = true
			}
		}
		if !found {
			t.Fatalf("expected tag3 (%s) in every object's tag_ids: %s", tag3, raw)
		}
	}

	raw, err = r.CallTool("vmware_tags_get_attached_tags_on_objects", map[string]interface{}{"inventory_paths": []interface{}{vm1, vm2}})
	if err != nil {
		t.Fatalf("vmware_tags_get_attached_tags_on_objects failed: %v", err)
	}
	objResults, _ = decodeResult(t, raw)["results"].([]interface{})
	if len(objResults) != 2 {
		t.Fatalf("expected 2 result entries, got %d: %s", len(objResults), raw)
	}
	e0, _ := objResults[0].(map[string]interface{})
	tagsList, _ := e0["tags"].([]interface{})
	if len(tagsList) == 0 {
		t.Fatalf("expected populated tag details, got %s", raw)
	}

	// Cleanup.
	for _, id := range []string{tag1, tag2, tag3} {
		if _, err := r.CallTool("vmware_tags_delete_tag", map[string]interface{}{"tag_id": id, "confirm": true}); err != nil {
			t.Fatalf("cleanup vmware_tags_delete_tag(%s) failed: %v", id, err)
		}
	}
	if _, err := r.CallTool("vmware_tags_delete_category", map[string]interface{}{"category_id": catID, "confirm": true}); err != nil {
		t.Fatalf("cleanup vmware_tags_delete_category failed: %v", err)
	}
}

// TestTagsTools_NameResolution proves resolveTagID/resolveCategoryID's
// convenience addition — DeleteTag/UpdateTag/DeleteCategory/UpdateCategory
// accept a bare Name (not just a URN ID), unlike the raw govmomi SDK methods
// they wrap (see generated_tags.go's top doc comment).
func TestTagsTools_NameResolution(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newTagsRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	if _, err := r.CallTool("vmware_tags_create_category", map[string]interface{}{
		"name": "NameResCat", "cardinality": "MULTIPLE", "confirm": true,
	}); err != nil {
		t.Fatalf("vmware_tags_create_category failed: %v", err)
	}

	if _, err := r.CallTool("vmware_tags_create_tag", map[string]interface{}{
		"name": "NameResTag", "category_id": "NameResCat", "confirm": true,
	}); err != nil {
		t.Fatalf("vmware_tags_create_tag (category by name) failed: %v", err)
	}

	// UpdateTag by tag NAME (not URN).
	if _, err := r.CallTool("vmware_tags_update_tag", map[string]interface{}{
		"tag_id": "NameResTag", "name": "NameResTagRenamed", "confirm": true,
	}); err != nil {
		t.Fatalf("vmware_tags_update_tag by name failed: %v", err)
	}
	raw, err := r.CallTool("vmware_tags_get_tag", map[string]interface{}{"id": "NameResTagRenamed"})
	if err != nil {
		t.Fatalf("vmware_tags_get_tag(NameResTagRenamed) failed: %v", err)
	}
	if name, _ := decodeResult(t, raw)["name"].(string); name != "NameResTagRenamed" {
		t.Fatalf("expected renamed tag, got %s", raw)
	}

	// DeleteTag by tag NAME.
	if _, err := r.CallTool("vmware_tags_delete_tag", map[string]interface{}{"tag_id": "NameResTagRenamed", "confirm": true}); err != nil {
		t.Fatalf("vmware_tags_delete_tag by name failed: %v", err)
	}

	// UpdateCategory by category NAME.
	if _, err := r.CallTool("vmware_tags_update_category", map[string]interface{}{
		"category_id": "NameResCat", "name": "NameResCatRenamed", "confirm": true,
	}); err != nil {
		t.Fatalf("vmware_tags_update_category by name failed: %v", err)
	}

	// DeleteCategory by category NAME (the new, renamed one).
	if _, err := r.CallTool("vmware_tags_delete_category", map[string]interface{}{"category_id": "NameResCatRenamed", "confirm": true}); err != nil {
		t.Fatalf("vmware_tags_delete_category by name failed: %v", err)
	}
}

// TestTagsTools_GateAndConfirm proves the Tier 1/2 tools in this file are
// wired through registerDestructive — same 3-layer protection check pattern
// as generated_authorization_test.go's TestAuthorizationTools_GateAndConfirm.
func TestTagsTools_GateAndConfirm(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	closedGate := newTagsRegistry(context.Background(), c, RegistryOptions{AllowDestructive: false})
	if _, err := closedGate.CallTool("vmware_tags_create_category", map[string]interface{}{
		"name": "ShouldNotExist", "cardinality": "MULTIPLE", "confirm": true,
	}); err == nil {
		t.Fatal("expected vmware_tags_create_category to be denied with the gate closed")
	}

	openGate := newTagsRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	if _, err := openGate.CallTool("vmware_tags_create_category", map[string]interface{}{
		"name": "ShouldNotExist", "cardinality": "MULTIPLE",
	}); err == nil {
		t.Fatal("expected vmware_tags_create_category to fail without confirm:true")
	}
}

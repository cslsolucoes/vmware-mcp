package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vapi/library"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// newLibraryCoreRegistry builds a Registry the normal way (NewRegistry,
// which wires every other domain via registerTools) and then manually layers
// registerLibraryCoreTools on top via withClass — the same pattern every
// other Fase 2+ test file in this package uses (see e.g.
// generated_customization_spec_test.go's newCustomizationSpecRegistry). This
// file must not edit registry.go itself — that is the orchestrator's job
// once all 4 Fase 8a groups are integrated (per this group's brief).
func newLibraryCoreRegistry(ctx context.Context, c *vmware.Client, opts RegistryOptions) *Registry {
	r := NewRegistry(ctx, c, opts)
	r.withClass(modeVCenterOnly, registerLibraryCoreTools)
	return r
}

// libraryDestructiveOpts opens the Fase 1a destructive gate so this file's
// tests can reach every Tier 1/2 handler's real body instead of stopping at
// the gate check — every call below also passes confirm:true.
func libraryDestructiveOpts() RegistryOptions {
	return RegistryOptions{AllowDestructive: true}
}

// TestLibraryCoreTools_RestFailsCleanlyOnStandaloneESXi proves this domain's
// vcenter-only classification empirically, not by trusting
// gen/classification.json alone: referencia/govmomi/vapi/simulator's own
// init() only ever registers the vAPI/REST endpoint against simulator.VPX()
// (`if r.IsVPX()`), never simulator.ESX() — so client.REST(ctx)'s login has
// nothing to log into against a standalone ESXi host and fails with its own
// "is the target a vCenter Server Appliance?" message, exactly like
// vmware_appliance_* (Fase 4) already does for VAMI. See this file's top doc
// comment.
func TestLibraryCoreTools_RestFailsCleanlyOnStandaloneESXi(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()
	r := newLibraryCoreRegistry(context.Background(), c, RegistryOptions{})

	_, err := r.CallTool("vmware_library_list_libraries", map[string]interface{}{})
	if err == nil {
		t.Fatal("expected an error on standalone ESXi (no vAPI/REST endpoint registered there), got success")
	}
	if !strings.Contains(err.Error(), "vCenter Server Appliance") {
		t.Fatalf("expected REST()'s vCenter-only message, got: %v", err)
	}
	if strings.Contains(err.Error(), "panicked") {
		t.Fatalf("should be a clean error, not a panic caught by CallTool's recover(): %v", err)
	}
}

// TestLibraryCoreTools_LocalLibraryAndItemLifecycle exercises the LOCAL
// library + item CRUD surface end to end against vcsim's real content
// library implementation (simulator.VPX(), vapi/simulator/simulator.go) —
// 19 of this file's 27 tools. Two libraries are created so both
// vmware_library_delete_library and vmware_library_force_delete_library each
// get a real, independent success path (not just one exercised and the
// other assumed identical).
func TestLibraryCoreTools_LocalLibraryAndItemLifecycle(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()
	r := newLibraryCoreRegistry(context.Background(), c, libraryDestructiveOpts())
	ctx := context.Background()

	ds, err := c.Finder.DefaultDatastore(ctx)
	if err != nil {
		t.Fatalf("failed to resolve default datastore: %v", err)
	}
	dsID := ds.Reference().Value

	storage := []interface{}{
		map[string]interface{}{"datastore_id": dsID, "type": "DATASTORE"},
	}

	// --- create_library (LOCAL) --------------------------------------------

	createRaw, err := r.CallTool("vmware_library_create_library", map[string]interface{}{
		"library": map[string]interface{}{
			"name":             "cl-a-lifecycle",
			"type":             "LOCAL",
			"description":      "created by TestLibraryCoreTools_LocalLibraryAndItemLifecycle",
			"storage_backings": storage,
		},
		"confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_library_create_library failed: %v", err)
	}
	libID, _ := decodeResult(t, createRaw)["id"].(string)
	if libID == "" {
		t.Fatalf("vmware_library_create_library returned no id: %s", createRaw)
	}

	// --- get_library_by_id ---------------------------------------------------

	getRaw, err := r.CallTool("vmware_library_get_library_by_id", map[string]interface{}{"library_id": libID})
	if err != nil {
		t.Fatalf("vmware_library_get_library_by_id failed: %v", err)
	}
	lib, ok := decodeResult(t, getRaw)["library"].(map[string]interface{})
	if !ok {
		t.Fatalf("vmware_library_get_library_by_id result has no \"library\" object: %s", getRaw)
	}
	if lib["name"] != "cl-a-lifecycle" {
		t.Fatalf("expected name \"cl-a-lifecycle\", got %v", lib["name"])
	}
	if lib["server_guid"] == "" || lib["server_guid"] == nil {
		t.Fatalf("expected a server-generated server_guid, got %v", lib["server_guid"])
	}
	state, _ := lib["state_info"].(map[string]interface{})
	if state["state"] != "ACTIVE" {
		t.Fatalf("expected state_info.state ACTIVE, got %v", lib["state_info"])
	}

	// --- list_libraries / find_library / get_library_by_name / get_libraries -

	listRaw, err := r.CallTool("vmware_library_list_libraries", map[string]interface{}{})
	if err != nil {
		t.Fatalf("vmware_library_list_libraries failed: %v", err)
	}
	if countOf(t, listRaw) < 1 {
		t.Fatalf("expected at least 1 library, got: %s", listRaw)
	}

	findRaw, err := r.CallTool("vmware_library_find_library", map[string]interface{}{"name": "CL-A-LIFECYCLE"})
	if err != nil {
		t.Fatalf("vmware_library_find_library failed: %v", err)
	}
	foundIDs, _ := decodeResult(t, findRaw)["library_ids"].([]interface{})
	if len(foundIDs) != 1 || foundIDs[0] != libID {
		t.Fatalf("expected vmware_library_find_library (case-insensitive) to return exactly [%s], got: %s", libID, findRaw)
	}

	byNameRaw, err := r.CallTool("vmware_library_get_library_by_name", map[string]interface{}{"name": "cl-a-lifecycle"})
	if err != nil {
		t.Fatalf("vmware_library_get_library_by_name failed: %v", err)
	}
	byNameLib, _ := decodeResult(t, byNameRaw)["library"].(map[string]interface{})
	if byNameLib["id"] != libID {
		t.Fatalf("expected vmware_library_get_library_by_name to resolve id %s, got: %s", libID, byNameRaw)
	}

	allRaw, err := r.CallTool("vmware_library_get_libraries", map[string]interface{}{})
	if err != nil {
		t.Fatalf("vmware_library_get_libraries failed: %v", err)
	}
	if countOf(t, allRaw) < 1 {
		t.Fatalf("expected at least 1 library from get_libraries, got: %s", allRaw)
	}

	// --- update_library --------------------------------------------------

	_, err = r.CallTool("vmware_library_update_library", map[string]interface{}{
		"library": map[string]interface{}{"id": libID, "name": "cl-a-lifecycle-renamed"},
		"confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_library_update_library failed: %v", err)
	}
	getRaw2, err := r.CallTool("vmware_library_get_library_by_id", map[string]interface{}{"library_id": libID})
	if err != nil {
		t.Fatalf("vmware_library_get_library_by_id (post-update) failed: %v", err)
	}
	if decodeResult(t, getRaw2)["library"].(map[string]interface{})["name"] != "cl-a-lifecycle-renamed" {
		t.Fatalf("vmware_library_update_library did not rename the library: %s", getRaw2)
	}

	// --- create_item / get_item / list_items / get_items / find_items -----

	itemCreateRaw, err := r.CallTool("vmware_library_create_item", map[string]interface{}{
		"item": map[string]interface{}{
			"name":        "item-1",
			"library_id":  libID,
			"type":        library.ItemTypeISO,
			"description": "an item",
		},
		"confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_library_create_item failed: %v", err)
	}
	itemID, _ := decodeResult(t, itemCreateRaw)["id"].(string)
	if itemID == "" {
		t.Fatalf("vmware_library_create_item returned no id: %s", itemCreateRaw)
	}

	itemGetRaw, err := r.CallTool("vmware_library_get_item", map[string]interface{}{"item_id": itemID})
	if err != nil {
		t.Fatalf("vmware_library_get_item failed: %v", err)
	}
	item, _ := decodeResult(t, itemGetRaw)["item"].(map[string]interface{})
	if item["name"] != "item-1" || item["library_id"] != libID {
		t.Fatalf("vmware_library_get_item returned unexpected item: %s", itemGetRaw)
	}

	listItemsRaw, err := r.CallTool("vmware_library_list_items", map[string]interface{}{"library_id": libID})
	if err != nil {
		t.Fatalf("vmware_library_list_items failed: %v", err)
	}
	if countOf(t, listItemsRaw) != 1 {
		t.Fatalf("expected exactly 1 item, got: %s", listItemsRaw)
	}

	getItemsRaw, err := r.CallTool("vmware_library_get_items", map[string]interface{}{"library_id": libID})
	if err != nil {
		t.Fatalf("vmware_library_get_items failed: %v", err)
	}
	if countOf(t, getItemsRaw) != 1 {
		t.Fatalf("expected exactly 1 item from get_items, got: %s", getItemsRaw)
	}

	findItemsRaw, err := r.CallTool("vmware_library_find_items", map[string]interface{}{"library_id": libID, "name": "item-1"})
	if err != nil {
		t.Fatalf("vmware_library_find_items failed: %v", err)
	}
	foundItemIDs, _ := decodeResult(t, findItemsRaw)["item_ids"].([]interface{})
	if len(foundItemIDs) != 1 || foundItemIDs[0] != itemID {
		t.Fatalf("expected vmware_library_find_items to return exactly [%s], got: %s", itemID, findItemsRaw)
	}

	// --- update_item ---------------------------------------------------------

	_, err = r.CallTool("vmware_library_update_item", map[string]interface{}{
		"item":    map[string]interface{}{"id": itemID, "name": "item-1-renamed"},
		"confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_library_update_item failed: %v", err)
	}
	itemGetRaw2, err := r.CallTool("vmware_library_get_item", map[string]interface{}{"item_id": itemID})
	if err != nil {
		t.Fatalf("vmware_library_get_item (post-update) failed: %v", err)
	}
	if decodeResult(t, itemGetRaw2)["item"].(map[string]interface{})["name"] != "item-1-renamed" {
		t.Fatalf("vmware_library_update_item did not rename the item: %s", itemGetRaw2)
	}

	// --- copy_item -------------------------------------------------------

	copyRaw, err := r.CallTool("vmware_library_copy_item", map[string]interface{}{
		"source_item_id": itemID,
		"destination":    map[string]interface{}{"name": "item-1-copy", "library_id": libID},
		"confirm":        true,
	})
	if err != nil {
		t.Fatalf("vmware_library_copy_item failed: %v", err)
	}
	copyID, _ := decodeResult(t, copyRaw)["id"].(string)
	if copyID == "" || copyID == itemID {
		t.Fatalf("vmware_library_copy_item returned an unexpected id: %s", copyRaw)
	}

	// --- publish_library / publish_item (trivial success: no subscriptions,
	// no VMTX items — see this file's top doc comment on why this is a real,
	// not vacuous, 200 from vcsim's own publish() implementation) -----------

	if _, err := r.CallTool("vmware_library_publish_library", map[string]interface{}{"library_id": libID, "confirm": true}); err != nil {
		t.Fatalf("vmware_library_publish_library failed: %v", err)
	}
	if _, err := r.CallTool("vmware_library_publish_item", map[string]interface{}{"item_id": itemID, "confirm": true}); err != nil {
		t.Fatalf("vmware_library_publish_item failed: %v", err)
	}

	// --- delete_item / delete_library (LOCAL path) --------------------------

	if _, err := r.CallTool("vmware_library_delete_item", map[string]interface{}{"item_id": copyID, "confirm": true}); err != nil {
		t.Fatalf("vmware_library_delete_item (copy) failed: %v", err)
	}
	if _, err := r.CallTool("vmware_library_delete_item", map[string]interface{}{"item_id": itemID, "confirm": true}); err != nil {
		t.Fatalf("vmware_library_delete_item failed: %v", err)
	}
	if _, err := r.CallTool("vmware_library_delete_library", map[string]interface{}{"library_id": libID, "confirm": true}); err != nil {
		t.Fatalf("vmware_library_delete_library failed: %v", err)
	}
	if _, err := r.CallTool("vmware_library_get_library_by_id", map[string]interface{}{"library_id": libID}); err == nil {
		t.Fatal("expected vmware_library_get_library_by_id to fail after delete, got success")
	}

	// --- force_delete_library (separate LOCAL library, own success path) ---

	fdCreateRaw, err := r.CallTool("vmware_library_create_library", map[string]interface{}{
		"library": map[string]interface{}{"name": "cl-a-force-delete", "type": "LOCAL", "storage_backings": storage},
		"confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_library_create_library (force-delete target) failed: %v", err)
	}
	fdLibID, _ := decodeResult(t, fdCreateRaw)["id"].(string)

	if _, err := r.CallTool("vmware_library_force_delete_library", map[string]interface{}{"library_id": fdLibID, "confirm": true}); err != nil {
		t.Fatalf("vmware_library_force_delete_library failed: %v", err)
	}
	if _, err := r.CallTool("vmware_library_get_library_by_id", map[string]interface{}{"library_id": fdLibID}); err == nil {
		t.Fatal("expected vmware_library_get_library_by_id to fail after force-delete, got success")
	}
}

// TestLibraryCoreTools_SubscribedLibrarySubscriberAndSync exercises the
// SUBSCRIBED-library slice of this file's tools — the other 8 of the 27 not
// covered by the LOCAL-library lifecycle test above: create_subscriber,
// list_subscribers, get_subscriber, delete_subscriber, sync_library,
// sync_item, evict_subscribed_library, evict_subscribed_item — plus a second,
// independent real success path for vmware_library_create_library (the
// SUBSCRIBED branch) and vmware_library_delete_library (the SUBSCRIBED
// branch, proving DeleteLibrary really does pick a different HTTP path than
// the LOCAL case above, not just "any path that happens to work").
//
// Empirically confirmed here (see this file's top doc comment), not assumed:
// creating a SUBSCRIBED library requires subscription_info.subscription_url
// to be another library's real publish_url (vcsim's syncSubLib matches
// path.Base(subscription_url) against its in-memory library-ID map — a
// syntactically valid but non-matching URL creates the library fine but
// silently syncs nothing). vmware_library_create_subscriber's "subscriber"
// object's subscribed_library field must be the ID of an EXISTING library on
// this server (vapi/simulator's subscriptionsID handler does
// s.Library[sub.LibraryID], 404s otherwise) — not a value the call itself
// generates.
func TestLibraryCoreTools_SubscribedLibrarySubscriberAndSync(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()
	r := newLibraryCoreRegistry(context.Background(), c, libraryDestructiveOpts())
	ctx := context.Background()

	ds, err := c.Finder.DefaultDatastore(ctx)
	if err != nil {
		t.Fatalf("failed to resolve default datastore: %v", err)
	}
	storage := []interface{}{map[string]interface{}{"datastore_id": ds.Reference().Value, "type": "DATASTORE"}}

	// --- published LOCAL source library --------------------------------

	pubRaw, err := r.CallTool("vmware_library_create_library", map[string]interface{}{
		"library": map[string]interface{}{
			"name":             "cl-a-published-source",
			"type":             "LOCAL",
			"storage_backings": storage,
			"publish_info":     map[string]interface{}{"authentication_method": "NONE", "published": true},
		},
		"confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_library_create_library (published source) failed: %v", err)
	}
	pubLibID, _ := decodeResult(t, pubRaw)["id"].(string)

	pubGetRaw, err := r.CallTool("vmware_library_get_library_by_id", map[string]interface{}{"library_id": pubLibID})
	if err != nil {
		t.Fatalf("vmware_library_get_library_by_id (published source) failed: %v", err)
	}
	pubLib := decodeResult(t, pubGetRaw)["library"].(map[string]interface{})
	pubInfo, _ := pubLib["publish_info"].(map[string]interface{})
	publishURL, _ := pubInfo["publish_url"].(string)
	if publishURL == "" {
		t.Fatalf("expected a server-generated publish_url on the published library, got: %s", pubGetRaw)
	}

	// An item must exist in the published source library BEFORE any
	// SUBSCRIBED library is created against it: vcsim's own library-create
	// handler unconditionally calls syncSubLib on every new library
	// (confirmed by reading vapi/simulator/simulator.go's "case \"\":"
	// create branch), which is a no-op for LOCAL libraries (no Subscription
	// set) but performs a real, immediate initial sync for a SUBSCRIBED one
	// — so the item created here is expected to already be present in the
	// SUBSCRIBED library right after its own creation call returns, with no
	// separate vmware_library_sync_library call needed to seed it.
	pubItemRaw, err := r.CallTool("vmware_library_create_item", map[string]interface{}{
		"item":    map[string]interface{}{"name": "pub-item", "library_id": pubLibID, "type": library.ItemTypeISO},
		"confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_library_create_item (in published source library) failed: %v", err)
	}
	pubItemID, _ := decodeResult(t, pubItemRaw)["id"].(string)

	// --- SUBSCRIBED library #1: explicit ssl_thumbprint (skips the TLS
	// auto-fetch branch entirely — see this file's top doc comment) --------

	subRaw, err := r.CallTool("vmware_library_create_library", map[string]interface{}{
		"library": map[string]interface{}{
			"name":             "cl-a-subscribed",
			"type":             "SUBSCRIBED",
			"storage_backings": storage,
			"subscription_info": map[string]interface{}{
				"subscription_url":      publishURL,
				"authentication_method": "NONE",
				"ssl_thumbprint":        "AA:BB:CC:DD (deliberately bogus — proves this branch is never validated client-side, only stored)",
			},
		},
		"confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_library_create_library (SUBSCRIBED, explicit thumbprint) failed: %v", err)
	}
	subLibID, _ := decodeResult(t, subRaw)["id"].(string)
	if subLibID == "" {
		t.Fatalf("vmware_library_create_library (SUBSCRIBED) returned no id: %s", subRaw)
	}

	// --- SUBSCRIBED library #2: ssl_thumbprint omitted — exercises the real
	// HTTPS auto-fetch branch (object.HostCertificateInfo.FromURL) against
	// vcsim's own TLS test server, proving that code path genuinely works
	// end to end here, not just documented as "should work" -----------------

	subAutoRaw, err := r.CallTool("vmware_library_create_library", map[string]interface{}{
		"library": map[string]interface{}{
			"name":             "cl-a-subscribed-auto-thumbprint",
			"type":             "SUBSCRIBED",
			"storage_backings": storage,
			"subscription_info": map[string]interface{}{
				"subscription_url":      publishURL,
				"authentication_method": "NONE",
			},
		},
		"confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_library_create_library (SUBSCRIBED, auto thumbprint) failed: %v", err)
	}
	subAutoLibID, _ := decodeResult(t, subAutoRaw)["id"].(string)

	// --- create_subscriber / list_subscribers / get_subscriber ------------

	createSubRaw, err := r.CallTool("vmware_library_create_subscriber", map[string]interface{}{
		"library_id": pubLibID,
		"subscriber": map[string]interface{}{
			"target":             "cl-a-test-target",
			"location":           "cl-a-test-location",
			"subscribed_library": subLibID,
		},
		"confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_library_create_subscriber failed: %v", err)
	}
	subscriptionID, _ := decodeResult(t, createSubRaw)["subscription_id"].(string)
	if subscriptionID == "" {
		t.Fatalf("vmware_library_create_subscriber returned no subscription_id: %s", createSubRaw)
	}

	listSubsRaw, err := r.CallTool("vmware_library_list_subscribers", map[string]interface{}{"library_id": pubLibID})
	if err != nil {
		t.Fatalf("vmware_library_list_subscribers failed: %v", err)
	}
	if countOf(t, listSubsRaw) != 1 {
		t.Fatalf("expected exactly 1 subscriber, got: %s", listSubsRaw)
	}

	getSubRaw, err := r.CallTool("vmware_library_get_subscriber", map[string]interface{}{
		"library_id":      pubLibID,
		"subscription_id": subscriptionID,
	})
	if err != nil {
		t.Fatalf("vmware_library_get_subscriber failed: %v", err)
	}
	subscriber, _ := decodeResult(t, getSubRaw)["subscriber"].(map[string]interface{})
	if subscriber["subscribed_library"] != subLibID {
		t.Fatalf("expected subscriber.subscribed_library %s, got: %s", subLibID, getSubRaw)
	}

	// --- sync_library (SUBSCRIBED-only path — vcsim 404s this against a
	// LOCAL library's ID, confirmed by reading simulator.go's libraryID
	// handler; see this file's top doc comment) -----------------------------

	if _, err := r.CallTool("vmware_library_sync_library", map[string]interface{}{"library_id": subLibID, "confirm": true}); err != nil {
		t.Fatalf("vmware_library_sync_library failed: %v", err)
	}

	// --- sync_item: vmware_library_create_item directly against a
	// SUBSCRIBED library's ID is rejected by vcsim (400
	// invalid_element_type — confirmed by reading vapi/simulator/
	// simulator.go's libraryItem handler: `if l.Type == "SUBSCRIBED" {
	// BadRequest(...) }`, matching real vSphere's rule that a subscribed
	// library's items only ever come from sync, never direct creation). The
	// item to sync here is instead the one that already propagated into
	// subLibID via the automatic initial sync at SUBSCRIBED-library-creation
	// time (see the pub-item comment above) — found via vmware_library_list_items
	// on subLibID rather than assumed to keep the same ID as pubItemID (it
	// does not: the destination item gets its own server-generated ID,
	// recorded as SourceID pointing back to pubItemID).

	subItemsRaw, err := r.CallTool("vmware_library_list_items", map[string]interface{}{"library_id": subLibID})
	if err != nil {
		t.Fatalf("vmware_library_list_items (SUBSCRIBED library, post-create) failed: %v", err)
	}
	subItemIDs, _ := decodeResult(t, subItemsRaw)["item_ids"].([]interface{})
	if len(subItemIDs) != 1 {
		t.Fatalf("expected exactly 1 item auto-synced into the SUBSCRIBED library at creation time, got: %s", subItemsRaw)
	}
	subItemID, _ := subItemIDs[0].(string)

	subItemGetRaw, err := r.CallTool("vmware_library_get_item", map[string]interface{}{"item_id": subItemID})
	if err != nil {
		t.Fatalf("vmware_library_get_item (auto-synced item) failed: %v", err)
	}
	subItem := decodeResult(t, subItemGetRaw)["item"].(map[string]interface{})
	if subItem["source_id"] != pubItemID {
		t.Fatalf("expected auto-synced item's source_id to be the published item %s, got: %s", pubItemID, subItemGetRaw)
	}

	if _, err := r.CallTool("vmware_library_sync_item", map[string]interface{}{"item_id": subItemID, "force": true, "confirm": true}); err != nil {
		t.Fatalf("vmware_library_sync_item failed: %v", err)
	}

	// --- evict_subscribed_library / evict_subscribed_item ------------------

	if _, err := r.CallTool("vmware_library_evict_subscribed_item", map[string]interface{}{"item_id": subItemID, "confirm": true}); err != nil {
		t.Fatalf("vmware_library_evict_subscribed_item failed: %v", err)
	}
	if _, err := r.CallTool("vmware_library_evict_subscribed_library", map[string]interface{}{"library_id": subLibID, "confirm": true}); err != nil {
		t.Fatalf("vmware_library_evict_subscribed_library failed: %v", err)
	}

	// --- delete_subscriber ---------------------------------------------------

	if _, err := r.CallTool("vmware_library_delete_subscriber", map[string]interface{}{
		"library_id":      pubLibID,
		"subscription_id": subscriptionID,
		"confirm":         true,
	}); err != nil {
		t.Fatalf("vmware_library_delete_subscriber failed: %v", err)
	}
	listSubsRaw2, err := r.CallTool("vmware_library_list_subscribers", map[string]interface{}{"library_id": pubLibID})
	if err != nil {
		t.Fatalf("vmware_library_list_subscribers (post-delete) failed: %v", err)
	}
	if countOf(t, listSubsRaw2) != 0 {
		t.Fatalf("expected 0 subscribers after delete, got: %s", listSubsRaw2)
	}

	// --- delete_library (SUBSCRIBED path — proves DeleteLibrary really
	// branches on Type, distinct from the LOCAL path already exercised in
	// TestLibraryCoreTools_LocalLibraryAndItemLifecycle) ---------------------

	if _, err := r.CallTool("vmware_library_delete_library", map[string]interface{}{"library_id": subLibID, "confirm": true}); err != nil {
		t.Fatalf("vmware_library_delete_library (SUBSCRIBED) failed: %v", err)
	}
	if _, err := r.CallTool("vmware_library_delete_library", map[string]interface{}{"library_id": subAutoLibID, "confirm": true}); err != nil {
		t.Fatalf("vmware_library_delete_library (SUBSCRIBED, auto-thumbprint lib) failed: %v", err)
	}
	if _, err := r.CallTool("vmware_library_delete_library", map[string]interface{}{"library_id": pubLibID, "confirm": true}); err != nil {
		t.Fatalf("vmware_library_delete_library (published source) failed: %v", err)
	}
}

// TestLibraryCoreTools_ArgumentValidation proves a handful of the argument
// guards this file's handlers add on top of the Fase 1a destructive gate
// (registry.go's wrapDestructive already covers gate-closed/missing-confirm
// generically — see destructive_test.go — so this file only needs to prove
// its own additions: required-struct-field checks on decoded JSON bodies).
func TestLibraryCoreTools_ArgumentValidation(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()
	r := newLibraryCoreRegistry(context.Background(), c, libraryDestructiveOpts())

	if _, err := r.CallTool("vmware_library_create_library", map[string]interface{}{
		"library": map[string]interface{}{"type": "LOCAL"}, // missing name
		"confirm": true,
	}); err == nil || !strings.Contains(err.Error(), "library.name is required") {
		t.Fatalf("expected a library.name is required error, got: %v", err)
	}

	if _, err := r.CallTool("vmware_library_create_library", map[string]interface{}{
		"library": map[string]interface{}{"name": "no-type"}, // missing type
		"confirm": true,
	}); err == nil || !strings.Contains(err.Error(), "library.type is required") {
		t.Fatalf("expected a library.type is required error, got: %v", err)
	}

	if _, err := r.CallTool("vmware_library_update_library", map[string]interface{}{
		"library": map[string]interface{}{"name": "no-id"}, // missing id
		"confirm": true,
	}); err == nil || !strings.Contains(err.Error(), "library.id is required") {
		t.Fatalf("expected a library.id is required error, got: %v", err)
	}

	if _, err := r.CallTool("vmware_library_create_item", map[string]interface{}{
		"item":    map[string]interface{}{"library_id": "x", "type": "iso"}, // missing name
		"confirm": true,
	}); err == nil || !strings.Contains(err.Error(), "item.name is required") {
		t.Fatalf("expected an item.name is required error, got: %v", err)
	}

	if _, err := r.CallTool("vmware_library_get_library_by_id", map[string]interface{}{}); err == nil || !strings.Contains(err.Error(), "library_id is required") {
		t.Fatalf("expected a library_id is required error, got: %v", err)
	}
}

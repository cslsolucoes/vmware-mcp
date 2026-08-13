package tools

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/vapi/library"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// registerLibraryCoreTools is group "CL-A" (Content Library core: libraries +
// items) of Fase 8a of the codegen plan
// (".workspace/plans/MCPVMWare2026-08-10-175300-plano-cobertura-completa-api-codegen.plan.md")
// — vapi/library.Manager, hand-transcribed from the real
// referencia/govmomi/vapi/library/library.go (16 methods) and
// library_item.go (11 methods) sources. All 27 candidate methods from the
// brief are implemented, no exclusions.
//
// Architecturally different from every Fase 0-7 file: this is the first
// group to wrap vapi/*.go (REST/JSON over *rest.Client, structs with native
// json tags) instead of object/*.go (SOAP/XML over vim25/types, no json
// tags). No decodeJSONArg-into-a-hand-built-schema workaround is needed for
// polymorphism (there is none in this domain) — arguments that map onto a
// library.Library/library.Item/library.SubscriberLibrary are decoded
// directly into the concrete govmomi struct via the existing decodeJSONArg
// helper (generated_vm_lifecycle.go), same mechanism, different reason for
// using it (convenience/consistency, not polymorphism).
//
// IDs are opaque server-generated strings here (library.Library.ID,
// library.Item.ID), never types.ManagedObjectReference — there is no
// resolveXxx-by-name-pattern helper for this domain the way resolveVM/
// resolveDatastore work for object/*.go. GetLibraryByName and FindLibrary/
// FindLibraryItems are this domain's own name->ID lookup tools; a caller
// wanting an ID from a name calls one of those first, same 2-step shape as
// this project's other ID-returning create tools.
//
// libraryCoreManager(ctx, client) is this file's equivalent of resolveDatastore/
// resolveVM: it calls client.REST(ctx) (the accessor added in Fase 4 for
// VAMI, reused here unmodified) to get a *rest.Client, then wraps it in
// library.NewManager. Confirmed by reading
// referencia/govmomi/vapi/simulator/simulator.go's init()
// (simulator.RegisterEndpoint(... if r.IsVPX() ...)) that the whole vAPI/
// REST endpoint — not just content library — only ever registers against
// simulator.VPX(), never simulator.ESX(): client.REST(ctx)'s rc.Login
// against simulator.ESX() has nothing to log into and fails cleanly with
// REST()'s own "is the target a vCenter Server Appliance?" message.
// TestLibraryCoreTools_RestFailsCleanlyOnStandaloneESXi in this file's test
// proves that real behavior end to end (not a nil-guard assumption) — the
// same vcenter-only conclusion the brief states, reached independently here
// by hitting the actual REST login path instead of trusting the
// classification file alone.
//
// Curation decisions (no method excluded, but several needed a judgment call
// reading the real vapi/library sources plus vapi/simulator's server-side
// handling — TestManagerCreateLibrary/TestManagerLibraryUsage in
// referencia/govmomi/vapi/library/library_test.go were read first as the
// closest thing to a spec for expected behavior, then re-verified against
// this project's own tests, not assumed to still hold unchanged):
//
//   - vmware_library_delete_library / vmware_library_force_delete_library:
//     Library.DeleteLibrary/ForceDeleteLibrary both branch on
//     library.Type ("SUBSCRIBED" vs anything else) to pick the SOAP-era-
//     named-but-actually-REST path (local-library vs subscribed-library),
//     confirmed by reading library.go's source directly. Rather than make
//     the caller pass a redundant "type" argument alongside library_id
//     (which they may not know/remember, and which GetLibraryByID already
//     returns), both handlers call m.GetLibraryByID first and pass the
//     resulting *Library straight into Delete/ForceDeleteLibrary — one
//     extra round trip against a real vCenter, worth it for a
//     can't-get-it-wrong argument shape. EvictSubscribedLibrary does NOT
//     get this treatment: its govmomi implementation always uses
//     SubscribedLibraryPath regardless of the Library.Type field passed in
//     (confirmed by reading the source — it never inspects .Type at all),
//     so a bare &library.Library{ID: id} is correct and sufficient; no
//     extra lookup buys anything.
//
//   - vmware_library_update_library / vmware_library_update_item: both
//     govmomi UpdateLibrary/UpdateLibraryItem implementations silently
//     forward only a subset of the decoded struct's fields (Name,
//     Description, Configuration for library; Name, Description for item)
//     and ignore everything else even if the caller's JSON sets it —
//     confirmed by reading both function bodies, not assumed from the
//     struct's field list. Documented in both tools' schema descriptions so
//     a caller doesn't waste a round trip expecting e.g. Storage or
//     Publication to change via update_library.
//
//   - vmware_library_create_subscriber: library.SubscriberLibrary's
//     LibraryID field (json "subscribed_library") is documented in
//     library.go only as "specification for a subscribed library to be
//     associated with a subscription" with no further detail on what value
//     is expected. Reading vapi/simulator/simulator.go's subscriptionsID
//     handler (case "create") shows it does `s.Library[sub.LibraryID]` —
//     i.e. this field must be the ID of an ALREADY-EXISTING library on this
//     same server (typically one created as type=SUBSCRIBED, pointed at the
//     published library's publish_url), not a value the server generates or
//     a free-form label. generated_library_core_test.go's subscriber test
//     creates that second library first and documents this real,
//     empirically-confirmed requirement in the tool's schema description —
//     not guessed from the struct comment alone.
//
//   - vmware_library_create_library (type=SUBSCRIBED path): library.go's
//     CreateLibrary auto-fetches an SSL thumbprint via
//     object.HostCertificateInfo.FromURL when subscription_info.ssl_thumbprint
//     is omitted and the subscription_url is https — this project's vcsim
//     test server is always TLS (testhelpers_test.go's
//     model.Service.TLS = new(tls.Config)), so that fetch path is real and
//     exercised once in generated_library_core_test.go, but every other
//     SUBSCRIBED-library test in this file supplies ssl_thumbprint
//     explicitly to skip it — both are legitimate real paths, not a
//     workaround; documented in the schema description so a caller knows
//     omitting ssl_thumbprint is not an error, just a slower path.
//
//   - Real bug found by testing, not by reading: a first attempt at
//     TestLibraryCoreTools_SubscribedLibrarySubscriberAndSync created a
//     SUBSCRIBED library with only subscription_info set (no
//     storage_backings) — reasonable per library.go's own doc comments,
//     which describe storage_backings only in the context of LOCAL
//     libraries. vcsim's handler panicked instead
//     (vapi/simulator/simulator.go's libraryPath unconditionally indexes
//     Storage[0] to compute the new library's on-disk directory, for every
//     library type, during create — confirmed by reading the panic's own
//     stack trace back to that line, then the source at that line). The
//     panic happens inside vcsim's own HTTP handler goroutine, not this
//     project's tool handler, so registry.go's CallTool recover() never
//     sees it — the Go HTTP client here just observes the connection drop
//     and returns a plain "EOF" error, no crash of the test binary or a
//     "tool ... panicked" message. Fixed by adding storage_backings to
//     every SUBSCRIBED library this file's test creates, and by
//     documenting the real (server-side, not client-side) requirement in
//     librarySpecArg's schema description above — a caller of
//     vmware_library_create_library against a real vCenter Server Appliance
//     may find storage_backings genuinely optional for type=SUBSCRIBED (not
//     verified against real hardware, no vCenter Appliance available to
//     this project — see appliance.go's Fase 4 note), but it is mandatory
//     against this project's own vcsim-backed test suite and is documented
//     as required unconditionally, the safer default.
//
// Tier assignments follow the brief exactly (see the brief's per-method
// tier annotations) — no reclassification.
func registerLibraryCoreTools(r *Registry) {
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}
	libraryIDArg := map[string]interface{}{
		"type":        "string",
		"description": `Content library ID (a server-generated opaque string, e.g. "d0055587-9d01-4b31-9e35-1a2b3c4d5e6f") as returned by vmware_library_create_library, vmware_library_list_libraries, vmware_library_find_library, or vmware_library_get_library_by_name. Not a library name.`,
	}
	itemIDArg := map[string]interface{}{
		"type":        "string",
		"description": `Library item ID (a server-generated opaque string) as returned by vmware_library_create_item, vmware_library_list_items, or vmware_library_find_items. Not an item name.`,
	}
	subscriptionsArg := map[string]interface{}{
		"type":        "array",
		"items":       map[string]interface{}{"type": "string"},
		"description": "Subscription IDs (as returned by vmware_library_create_subscriber / vmware_library_list_subscribers) to target. Omit or leave empty to target every subscription of the library/item.",
	}
	forceArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Force a full re-sync of content even if it already appears up to date. Default false.",
	}
	librarySpecArg := map[string]interface{}{
		"type":        "object",
		"description": `A library.Library JSON object (matching its Go struct's json tags — referencia/govmomi/vapi/library/library.go). Required: "name", "type" ("LOCAL" or "SUBSCRIBED"), and "storage_backings" (an array of {"datastore_id": "<datastore moref value, e.g. from vmware_list_datastores>", "type": "DATASTORE"}) — storage_backings is required for BOTH types against this server, not just LOCAL as the Go source's doc comment alone would suggest (empirically confirmed: vcsim's own library-creation handler indexes storage_backings[0] unconditionally to compute the new library's on-disk path and panics on an empty array — see this file's top doc comment). For type="SUBSCRIBED", additionally required: "subscription_info", an object {"subscription_url": "<https URL to another library's publish_url>", "authentication_method": "NONE"|"BASIC", "user_name", "password", "ssl_thumbprint" (optional — auto-fetched via TLS if subscription_url is https and this is omitted), "automatic_sync_enabled", "on_demand"}. Optional either way: "description", "publish_info" (object: "authentication_method", "published" bool, "user_name", "password"), "security_policy_id", "configuration_info" (object: {"apply_library_usage_to_items": bool}). Server-generated fields ("id", "creation_time", "last_modified_time", "last_sync_time", "server_guid", "state_info") are ignored if supplied.`,
	}
	libraryUpdateSpecArg := map[string]interface{}{
		"type":        "object",
		"description": `A library.Library JSON object. Required: "id" (the library to update). Only "name", "description", and "configuration_info" ({"apply_library_usage_to_items": bool}) are honored by the server — every other field is silently ignored even if supplied (see this file's top doc comment).`,
	}
	itemSpecArg := map[string]interface{}{
		"type":        "object",
		"description": `A library.Item JSON object. Required: "name", "library_id" (the destination library — see vmware_library_list_libraries), "type" (e.g. "iso", "ovf", "vm-template"). Optional: "description". Every other field is ignored even if supplied — the server only forwards these 4.`,
	}
	itemUpdateSpecArg := map[string]interface{}{
		"type":        "object",
		"description": `A library.Item JSON object. Required: "id" (the item to update). Only "name" and "description" are honored by the server — every other field is silently ignored even if supplied (see this file's top doc comment).`,
	}
	itemCopyDestArg := map[string]interface{}{
		"type":        "object",
		"description": `A library.Item JSON object describing the destination item, e.g. {"name": "...", "library_id": "...", "description": "..."}. Passed through as the copy's destination_create_spec.`,
	}
	subscriberSpecArg := map[string]interface{}{
		"type":        "object",
		"description": `A library.SubscriberLibrary JSON object. Required: "target" (a free-form location/hostname label recorded on the subscription), "location" (free-form), "subscribed_library" (the ID of an ALREADY-EXISTING library on this server that represents the subscriber side — typically one created earlier with vmware_library_create_library type="SUBSCRIBED" — see this file's top doc comment for why this is empirically required, not just documented). Optional: "vcenter" (object: "hostname", "https_port", "server_guid"), "placement" (object: "resource_pool", "host", "folder", "cluster", "network").`,
	}

	// --- Library: read-only -----------------------------------------------

	r.register("vmware_library_find_library",
		"Find content library IDs matching a name (case-insensitive) and/or type. Omit both to list every library, same as vmware_library_list_libraries.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{"type": "string", "description": "Case-insensitive library name to match. Omit to match every name."},
				"type": map[string]interface{}{"type": "string", "description": `Library type to match: "LOCAL" or "SUBSCRIBED". Omit to match either.`},
			},
		},
		Tool{Handler: handleLibraryFindLibrary},
	)

	r.register("vmware_library_list_libraries",
		"List the IDs of every content library in the system.",
		map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
		Tool{Handler: handleLibraryListLibraries},
	)

	r.register("vmware_library_get_library_by_id",
		"Get full details of one content library by ID.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"library_id": libraryIDArg},
			"required":   []interface{}{"library_id"},
		},
		Tool{Handler: handleLibraryGetLibraryByID},
	)

	r.register("vmware_library_get_library_by_name",
		"Get full details of one content library by exact name. Fails if no library has that exact name.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{"type": "string", "description": "Exact library name."},
			},
			"required": []interface{}{"name"},
		},
		Tool{Handler: handleLibraryGetLibraryByName},
	)

	r.register("vmware_library_get_libraries",
		"Get full details of every content library in the system (equivalent to calling vmware_library_get_library_by_id for every ID from vmware_library_list_libraries, aggregated).",
		map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
		Tool{Handler: handleLibraryGetLibraries},
	)

	r.register("vmware_library_list_subscribers",
		"List the subscriptions of a published library.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"library_id": libraryIDArg},
			"required":   []interface{}{"library_id"},
		},
		Tool{Handler: handleLibraryListSubscribers},
	)

	r.register("vmware_library_get_subscriber",
		"Get details of one subscription of a published library.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"library_id":      libraryIDArg,
				"subscription_id": map[string]interface{}{"type": "string", "description": "Subscription ID as returned by vmware_library_create_subscriber / vmware_library_list_subscribers."},
			},
			"required": []interface{}{"library_id", "subscription_id"},
		},
		Tool{Handler: handleLibraryGetSubscriber},
	)

	// --- Library: Tier 2 (disruptive, reversible) --------------------------

	r.registerDestructive("vmware_library_create_library",
		"Create a new content library (LOCAL, backed by a datastore, or SUBSCRIBED, pulling from another library's publish URL). Returns the new library's ID.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"library": librarySpecArg,
				"confirm": confirmArg,
			},
			"required": []interface{}{"library", "confirm"},
		},
		Tool{Handler: handleLibraryCreateLibrary},
	)

	r.registerDestructive("vmware_library_sync_library",
		"Sync a SUBSCRIBED library against its source library now, instead of waiting for its normal sync schedule.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"library_id": libraryIDArg,
				"confirm":    confirmArg,
			},
			"required": []interface{}{"library_id", "confirm"},
		},
		Tool{Handler: handleLibrarySyncLibrary},
	)

	r.registerDestructive("vmware_library_publish_library",
		"Publish a library's content to its subscriptions (or all of them if subscription_ids is omitted).",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"library_id":    libraryIDArg,
				"subscriptions": subscriptionsArg,
				"confirm":       confirmArg,
			},
			"required": []interface{}{"library_id", "confirm"},
		},
		Tool{Handler: handleLibraryPublishLibrary},
	)

	r.registerDestructive("vmware_library_update_library",
		"Update a content library's name, description, and/or configuration. See the library_spec argument description for exactly which fields the server honors.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"library": libraryUpdateSpecArg,
				"confirm": confirmArg,
			},
			"required": []interface{}{"library", "confirm"},
		},
		Tool{Handler: handleLibraryUpdateLibrary},
	)

	r.registerDestructive("vmware_library_evict_subscribed_library",
		"Evict (remove) the cached content of an on-demand SUBSCRIBED library, freeing storage. The library and its item metadata remain; only cached file content is removed.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"library_id": libraryIDArg,
				"confirm":    confirmArg,
			},
			"required": []interface{}{"library_id", "confirm"},
		},
		Tool{Handler: handleLibraryEvictSubscribedLibrary},
	)

	r.registerDestructive("vmware_library_create_subscriber",
		"Create a subscription of a published library, associating an already-existing SUBSCRIBED library on this server as the subscriber. Returns the new subscription's ID.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"library_id": libraryIDArg,
				"subscriber": subscriberSpecArg,
				"confirm":    confirmArg,
			},
			"required": []interface{}{"library_id", "subscriber", "confirm"},
		},
		Tool{Handler: handleLibraryCreateSubscriber},
	)

	// --- Library: Tier 1 (irreversible) ------------------------------------

	r.registerDestructive("vmware_library_delete_library",
		"Delete a content library. Irreversible. Fails if the library still has usage/items depending on it — use vmware_library_force_delete_library to skip that check.",
		tier1,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"library_id": libraryIDArg,
				"confirm":    confirmArg,
			},
			"required": []interface{}{"library_id", "confirm"},
		},
		Tool{Handler: handleLibraryDeleteLibrary},
	)

	r.registerDestructive("vmware_library_force_delete_library",
		"Delete a content library, skipping the usage check vmware_library_delete_library enforces. Irreversible.",
		tier1,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"library_id": libraryIDArg,
				"confirm":    confirmArg,
			},
			"required": []interface{}{"library_id", "confirm"},
		},
		Tool{Handler: handleLibraryForceDeleteLibrary},
	)

	r.registerDestructive("vmware_library_delete_subscriber",
		"Delete a subscription of a published library. Irreversible — the subscription record is removed (the subscriber library itself is not deleted).",
		tier1,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"library_id":      libraryIDArg,
				"subscription_id": map[string]interface{}{"type": "string", "description": "Subscription ID as returned by vmware_library_create_subscriber / vmware_library_list_subscribers."},
				"confirm":         confirmArg,
			},
			"required": []interface{}{"library_id", "subscription_id", "confirm"},
		},
		Tool{Handler: handleLibraryDeleteSubscriber},
	)

	// --- LibraryItem: read-only ---------------------------------------------

	r.register("vmware_library_list_items",
		"List the IDs of every item in a content library.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"library_id": libraryIDArg},
			"required":   []interface{}{"library_id"},
		},
		Tool{Handler: handleLibraryListItems},
	)

	r.register("vmware_library_get_item",
		"Get full details of one library item by ID.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"item_id": itemIDArg},
			"required":   []interface{}{"item_id"},
		},
		Tool{Handler: handleLibraryGetItem},
	)

	r.register("vmware_library_get_items",
		"Get full details of every item in a content library (equivalent to calling vmware_library_get_item for every ID from vmware_library_list_items, aggregated).",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"library_id": libraryIDArg},
			"required":   []interface{}{"library_id"},
		},
		Tool{Handler: handleLibraryGetItems},
	)

	r.register("vmware_library_find_items",
		"Find library item IDs matching any combination of library, name, type, source item ID, and cached state. Omit every field to list every item in the system, same as calling vmware_library_get_libraries + vmware_library_list_items per library.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"library_id": map[string]interface{}{"type": "string", "description": "Restrict the search to this library ID. Omit to search every library."},
				"name":       map[string]interface{}{"type": "string", "description": "Item name to match. Omit to match every name."},
				"type":       map[string]interface{}{"type": "string", "description": `Item type to match, e.g. "iso", "ovf", "vm-template". Omit to match every type.`},
				"source_id":  map[string]interface{}{"type": "string", "description": "For items in a SUBSCRIBED library, the source item ID on the published library. Omit to match any."},
				"cached":     map[string]interface{}{"type": "boolean", "description": "Match only items whose cached content matches this value. Omit to match either."},
			},
		},
		Tool{Handler: handleLibraryFindItems},
	)

	// --- LibraryItem: Tier 2 (disruptive, reversible) -----------------------

	r.registerDestructive("vmware_library_create_item",
		"Create a new (empty) item in a content library. Returns the new item's ID. Content is added separately (out of this group's scope — no upload-session tools are registered here).",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"item":    itemSpecArg,
				"confirm": confirmArg,
			},
			"required": []interface{}{"item", "confirm"},
		},
		Tool{Handler: handleLibraryCreateItem},
	)

	r.registerDestructive("vmware_library_copy_item",
		"Copy a library item to a new item (same or different library). Returns the new item's ID.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"source_item_id": itemIDArg,
				"destination":    itemCopyDestArg,
				"confirm":        confirmArg,
			},
			"required": []interface{}{"source_item_id", "destination", "confirm"},
		},
		Tool{Handler: handleLibraryCopyItem},
	)

	r.registerDestructive("vmware_library_sync_item",
		"Sync one item of a SUBSCRIBED library against its source item now, instead of waiting for the library's normal sync schedule.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"item_id": itemIDArg,
				"force":   forceArg,
				"confirm": confirmArg,
			},
			"required": []interface{}{"item_id", "confirm"},
		},
		Tool{Handler: handleLibrarySyncItem},
	)

	r.registerDestructive("vmware_library_publish_item",
		"Publish one library item to its library's subscriptions (or all of them if subscriptions is omitted).",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"item_id":       itemIDArg,
				"force":         forceArg,
				"subscriptions": subscriptionsArg,
				"confirm":       confirmArg,
			},
			"required": []interface{}{"item_id", "confirm"},
		},
		Tool{Handler: handleLibraryPublishItem},
	)

	r.registerDestructive("vmware_library_update_item",
		"Update a library item's name and/or description. See the item_spec argument description for exactly which fields the server honors.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"item":    itemUpdateSpecArg,
				"confirm": confirmArg,
			},
			"required": []interface{}{"item", "confirm"},
		},
		Tool{Handler: handleLibraryUpdateItem},
	)

	r.registerDestructive("vmware_library_evict_subscribed_item",
		"Evict (remove) the cached content of one item in an on-demand SUBSCRIBED library, freeing storage. The item metadata remains; only cached file content is removed.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"item_id": itemIDArg,
				"confirm": confirmArg,
			},
			"required": []interface{}{"item_id", "confirm"},
		},
		Tool{Handler: handleLibraryEvictSubscribedItem},
	)

	// --- LibraryItem: Tier 1 (irreversible) ---------------------------------

	r.registerDestructive("vmware_library_delete_item",
		"Delete a library item. Irreversible.",
		tier1,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"item_id": itemIDArg,
				"confirm": confirmArg,
			},
			"required": []interface{}{"item_id", "confirm"},
		},
		Tool{Handler: handleLibraryDeleteItem},
	)
}

// libraryCoreManager returns a *library.Manager for client — this file's
// equivalent of resolveDatastore/resolveVM: every handler below calls this
// first. client.REST(ctx) (added in Fase 4 for VAMI) already names the
// likely cause of failure ("is the target a vCenter Server Appliance?") —
// see this file's top doc comment for why that failure is real and expected
// against a standalone ESXi host, not just a generic error passthrough.
//
// Named libraryCoreManager, not libraryManager, because
// generated_library_sessions.go (a sibling Fase 8a group's file, already
// present in this package when this file was written) independently
// declares its own identical-signature, identical-body libraryManager — a
// genuine duplicate, flagged here for the orchestrator's integration pass
// rather than silently deleted, since this group's brief says not to touch
// other groups' files.
func libraryCoreManager(ctx context.Context, client *vmware.Client) (*library.Manager, error) {
	rc, err := client.REST(ctx)
	if err != nil {
		return nil, err
	}
	return library.NewManager(rc), nil
}

// requiredStringArg reads args[key] as a non-empty string or returns a clear
// error — shared by every handler below that takes a bare ID argument
// instead of a full struct (decodeJSONArg handles those).
func requiredStringArg(args map[string]interface{}, key string) (string, error) {
	v, _ := args[key].(string)
	if v == "" {
		return "", fmt.Errorf("%s is required", key)
	}
	return v, nil
}

// --- Library handlers -------------------------------------------------------

func handleLibraryFindLibrary(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryCoreManager(ctx, client)
	if err != nil {
		return "", err
	}
	name, _ := args["name"].(string)
	typ, _ := args["type"].(string)

	ids, err := m.FindLibrary(ctx, library.Find{Name: name, Type: typ})
	if err != nil {
		return "", fmt.Errorf("failed to find libraries: %w", err)
	}
	return marshalJSON(map[string]interface{}{"library_ids": ids, "count": len(ids)})
}

func handleLibraryListLibraries(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryCoreManager(ctx, client)
	if err != nil {
		return "", err
	}

	ids, err := m.ListLibraries(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to list libraries: %w", err)
	}
	return marshalJSON(map[string]interface{}{"library_ids": ids, "count": len(ids)})
}

func handleLibraryGetLibraryByID(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryCoreManager(ctx, client)
	if err != nil {
		return "", err
	}
	id, err := requiredStringArg(args, "library_id")
	if err != nil {
		return "", err
	}

	lib, err := m.GetLibraryByID(ctx, id)
	if err != nil {
		return "", fmt.Errorf("failed to get library %s: %w", id, err)
	}
	return marshalJSON(map[string]interface{}{"library": lib})
}

func handleLibraryGetLibraryByName(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryCoreManager(ctx, client)
	if err != nil {
		return "", err
	}
	name, err := requiredStringArg(args, "name")
	if err != nil {
		return "", err
	}

	lib, err := m.GetLibraryByName(ctx, name)
	if err != nil {
		return "", fmt.Errorf("failed to get library %q: %w", name, err)
	}
	return marshalJSON(map[string]interface{}{"library": lib})
}

func handleLibraryGetLibraries(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryCoreManager(ctx, client)
	if err != nil {
		return "", err
	}

	libs, err := m.GetLibraries(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get libraries: %w", err)
	}
	return marshalJSON(map[string]interface{}{"libraries": libs, "count": len(libs)})
}

func handleLibraryListSubscribers(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryCoreManager(ctx, client)
	if err != nil {
		return "", err
	}
	id, err := requiredStringArg(args, "library_id")
	if err != nil {
		return "", err
	}

	subs, err := m.ListSubscribers(ctx, &library.Library{ID: id})
	if err != nil {
		return "", fmt.Errorf("failed to list subscribers of library %s: %w", id, err)
	}
	return marshalJSON(map[string]interface{}{"library_id": id, "subscribers": subs, "count": len(subs)})
}

func handleLibraryGetSubscriber(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryCoreManager(ctx, client)
	if err != nil {
		return "", err
	}
	libID, err := requiredStringArg(args, "library_id")
	if err != nil {
		return "", err
	}
	subID, err := requiredStringArg(args, "subscription_id")
	if err != nil {
		return "", err
	}

	sub, err := m.GetSubscriber(ctx, &library.Library{ID: libID}, subID)
	if err != nil {
		return "", fmt.Errorf("failed to get subscriber %s of library %s: %w", subID, libID, err)
	}
	return marshalJSON(map[string]interface{}{"library_id": libID, "subscription_id": subID, "subscriber": sub})
}

func handleLibraryCreateLibrary(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryCoreManager(ctx, client)
	if err != nil {
		return "", err
	}
	raw, ok := args["library"]
	if !ok {
		return "", fmt.Errorf("library is required")
	}
	var lib library.Library
	if err := decodeJSONArg(raw, &lib); err != nil {
		return "", fmt.Errorf("invalid library: %w", err)
	}
	if lib.Name == "" {
		return "", fmt.Errorf("library.name is required")
	}
	if lib.Type == "" {
		return "", fmt.Errorf(`library.type is required ("LOCAL" or "SUBSCRIBED")`)
	}

	id, err := m.CreateLibrary(ctx, lib)
	if err != nil {
		return "", fmt.Errorf("failed to create library %q: %w", lib.Name, err)
	}
	return marshalJSON(map[string]interface{}{"id": id, "name": lib.Name, "type": lib.Type, "result": "created"})
}

func handleLibrarySyncLibrary(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryCoreManager(ctx, client)
	if err != nil {
		return "", err
	}
	id, err := requiredStringArg(args, "library_id")
	if err != nil {
		return "", err
	}

	if err := m.SyncLibrary(ctx, &library.Library{ID: id}); err != nil {
		return "", fmt.Errorf("failed to sync library %s: %w", id, err)
	}
	return marshalJSON(map[string]interface{}{"library_id": id, "result": "synced"})
}

func handleLibraryPublishLibrary(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryCoreManager(ctx, client)
	if err != nil {
		return "", err
	}
	id, err := requiredStringArg(args, "library_id")
	if err != nil {
		return "", err
	}
	var subs []string
	if raw, ok := args["subscriptions"]; ok {
		subs, err = toStringSlice(raw)
		if err != nil {
			return "", fmt.Errorf("invalid subscriptions: %w", err)
		}
	}

	if err := m.PublishLibrary(ctx, &library.Library{ID: id}, subs); err != nil {
		return "", fmt.Errorf("failed to publish library %s: %w", id, err)
	}
	return marshalJSON(map[string]interface{}{"library_id": id, "subscriptions": subs, "result": "published"})
}

func handleLibraryUpdateLibrary(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryCoreManager(ctx, client)
	if err != nil {
		return "", err
	}
	raw, ok := args["library"]
	if !ok {
		return "", fmt.Errorf("library is required")
	}
	var lib library.Library
	if err := decodeJSONArg(raw, &lib); err != nil {
		return "", fmt.Errorf("invalid library: %w", err)
	}
	if lib.ID == "" {
		return "", fmt.Errorf("library.id is required")
	}

	if err := m.UpdateLibrary(ctx, &lib); err != nil {
		return "", fmt.Errorf("failed to update library %s: %w", lib.ID, err)
	}
	return marshalJSON(map[string]interface{}{"library_id": lib.ID, "result": "updated"})
}

func handleLibraryEvictSubscribedLibrary(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryCoreManager(ctx, client)
	if err != nil {
		return "", err
	}
	id, err := requiredStringArg(args, "library_id")
	if err != nil {
		return "", err
	}

	if err := m.EvictSubscribedLibrary(ctx, &library.Library{ID: id}); err != nil {
		return "", fmt.Errorf("failed to evict subscribed library %s: %w", id, err)
	}
	return marshalJSON(map[string]interface{}{"library_id": id, "result": "evicted"})
}

func handleLibraryCreateSubscriber(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryCoreManager(ctx, client)
	if err != nil {
		return "", err
	}
	libID, err := requiredStringArg(args, "library_id")
	if err != nil {
		return "", err
	}
	raw, ok := args["subscriber"]
	if !ok {
		return "", fmt.Errorf("subscriber is required")
	}
	var sub library.SubscriberLibrary
	if err := decodeJSONArg(raw, &sub); err != nil {
		return "", fmt.Errorf("invalid subscriber: %w", err)
	}

	id, err := m.CreateSubscriber(ctx, &library.Library{ID: libID}, sub)
	if err != nil {
		return "", fmt.Errorf("failed to create subscriber of library %s: %w", libID, err)
	}
	return marshalJSON(map[string]interface{}{"library_id": libID, "subscription_id": id, "result": "created"})
}

func handleLibraryDeleteLibrary(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryCoreManager(ctx, client)
	if err != nil {
		return "", err
	}
	id, err := requiredStringArg(args, "library_id")
	if err != nil {
		return "", err
	}

	// DeleteLibrary branches on Library.Type to pick its path — resolve the
	// full library first so the caller doesn't have to know/pass type. See
	// this file's top doc comment.
	lib, err := m.GetLibraryByID(ctx, id)
	if err != nil {
		return "", fmt.Errorf("failed to resolve library %s before delete: %w", id, err)
	}
	if err := m.DeleteLibrary(ctx, lib); err != nil {
		return "", fmt.Errorf("failed to delete library %s: %w", id, err)
	}
	return marshalJSON(map[string]interface{}{"library_id": id, "result": "deleted"})
}

func handleLibraryForceDeleteLibrary(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryCoreManager(ctx, client)
	if err != nil {
		return "", err
	}
	id, err := requiredStringArg(args, "library_id")
	if err != nil {
		return "", err
	}

	lib, err := m.GetLibraryByID(ctx, id)
	if err != nil {
		return "", fmt.Errorf("failed to resolve library %s before force-delete: %w", id, err)
	}
	if err := m.ForceDeleteLibrary(ctx, lib); err != nil {
		return "", fmt.Errorf("failed to force-delete library %s: %w", id, err)
	}
	return marshalJSON(map[string]interface{}{"library_id": id, "result": "deleted"})
}

func handleLibraryDeleteSubscriber(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryCoreManager(ctx, client)
	if err != nil {
		return "", err
	}
	libID, err := requiredStringArg(args, "library_id")
	if err != nil {
		return "", err
	}
	subID, err := requiredStringArg(args, "subscription_id")
	if err != nil {
		return "", err
	}

	if err := m.DeleteSubscriber(ctx, &library.Library{ID: libID}, subID); err != nil {
		return "", fmt.Errorf("failed to delete subscriber %s of library %s: %w", subID, libID, err)
	}
	return marshalJSON(map[string]interface{}{"library_id": libID, "subscription_id": subID, "result": "deleted"})
}

// --- LibraryItem handlers ----------------------------------------------------

func handleLibraryListItems(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryCoreManager(ctx, client)
	if err != nil {
		return "", err
	}
	libID, err := requiredStringArg(args, "library_id")
	if err != nil {
		return "", err
	}

	ids, err := m.ListLibraryItems(ctx, libID)
	if err != nil {
		return "", fmt.Errorf("failed to list items of library %s: %w", libID, err)
	}
	return marshalJSON(map[string]interface{}{"library_id": libID, "item_ids": ids, "count": len(ids)})
}

func handleLibraryGetItem(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryCoreManager(ctx, client)
	if err != nil {
		return "", err
	}
	id, err := requiredStringArg(args, "item_id")
	if err != nil {
		return "", err
	}

	item, err := m.GetLibraryItem(ctx, id)
	if err != nil {
		return "", fmt.Errorf("failed to get item %s: %w", id, err)
	}
	return marshalJSON(map[string]interface{}{"item": item})
}

func handleLibraryGetItems(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryCoreManager(ctx, client)
	if err != nil {
		return "", err
	}
	libID, err := requiredStringArg(args, "library_id")
	if err != nil {
		return "", err
	}

	items, err := m.GetLibraryItems(ctx, libID)
	if err != nil {
		return "", fmt.Errorf("failed to get items of library %s: %w", libID, err)
	}
	return marshalJSON(map[string]interface{}{"library_id": libID, "items": items, "count": len(items)})
}

func handleLibraryFindItems(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryCoreManager(ctx, client)
	if err != nil {
		return "", err
	}
	libID, _ := args["library_id"].(string)
	name, _ := args["name"].(string)
	typ, _ := args["type"].(string)
	sourceID, _ := args["source_id"].(string)

	search := library.FindItem{LibraryID: libID, Name: name, Type: typ, SourceID: sourceID}
	if v, ok := args["cached"]; ok {
		if b, ok := v.(bool); ok {
			search.Cached = &b
		}
	}

	ids, err := m.FindLibraryItems(ctx, search)
	if err != nil {
		return "", fmt.Errorf("failed to find library items: %w", err)
	}
	return marshalJSON(map[string]interface{}{"item_ids": ids, "count": len(ids)})
}

func handleLibraryCreateItem(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryCoreManager(ctx, client)
	if err != nil {
		return "", err
	}
	raw, ok := args["item"]
	if !ok {
		return "", fmt.Errorf("item is required")
	}
	var item library.Item
	if err := decodeJSONArg(raw, &item); err != nil {
		return "", fmt.Errorf("invalid item: %w", err)
	}
	if item.Name == "" {
		return "", fmt.Errorf("item.name is required")
	}
	if item.LibraryID == "" {
		return "", fmt.Errorf("item.library_id is required")
	}
	if item.Type == "" {
		return "", fmt.Errorf("item.type is required")
	}

	id, err := m.CreateLibraryItem(ctx, item)
	if err != nil {
		return "", fmt.Errorf("failed to create item %q in library %s: %w", item.Name, item.LibraryID, err)
	}
	return marshalJSON(map[string]interface{}{"id": id, "library_id": item.LibraryID, "result": "created"})
}

func handleLibraryCopyItem(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryCoreManager(ctx, client)
	if err != nil {
		return "", err
	}
	srcID, err := requiredStringArg(args, "source_item_id")
	if err != nil {
		return "", err
	}
	raw, ok := args["destination"]
	if !ok {
		return "", fmt.Errorf("destination is required")
	}
	var dst library.Item
	if err := decodeJSONArg(raw, &dst); err != nil {
		return "", fmt.Errorf("invalid destination: %w", err)
	}

	id, err := m.CopyLibraryItem(ctx, &library.Item{ID: srcID}, dst)
	if err != nil {
		return "", fmt.Errorf("failed to copy item %s: %w", srcID, err)
	}
	return marshalJSON(map[string]interface{}{"source_item_id": srcID, "id": id, "result": "copied"})
}

func handleLibrarySyncItem(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryCoreManager(ctx, client)
	if err != nil {
		return "", err
	}
	id, err := requiredStringArg(args, "item_id")
	if err != nil {
		return "", err
	}
	force, _ := args["force"].(bool)

	if err := m.SyncLibraryItem(ctx, &library.Item{ID: id}, force); err != nil {
		return "", fmt.Errorf("failed to sync item %s: %w", id, err)
	}
	return marshalJSON(map[string]interface{}{"item_id": id, "force": force, "result": "synced"})
}

func handleLibraryPublishItem(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryCoreManager(ctx, client)
	if err != nil {
		return "", err
	}
	id, err := requiredStringArg(args, "item_id")
	if err != nil {
		return "", err
	}
	force, _ := args["force"].(bool)
	var subs []string
	if raw, ok := args["subscriptions"]; ok {
		subs, err = toStringSlice(raw)
		if err != nil {
			return "", fmt.Errorf("invalid subscriptions: %w", err)
		}
	}

	if err := m.PublishLibraryItem(ctx, &library.Item{ID: id}, force, subs); err != nil {
		return "", fmt.Errorf("failed to publish item %s: %w", id, err)
	}
	return marshalJSON(map[string]interface{}{"item_id": id, "force": force, "subscriptions": subs, "result": "published"})
}

func handleLibraryUpdateItem(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryCoreManager(ctx, client)
	if err != nil {
		return "", err
	}
	raw, ok := args["item"]
	if !ok {
		return "", fmt.Errorf("item is required")
	}
	var item library.Item
	if err := decodeJSONArg(raw, &item); err != nil {
		return "", fmt.Errorf("invalid item: %w", err)
	}
	if item.ID == "" {
		return "", fmt.Errorf("item.id is required")
	}

	if err := m.UpdateLibraryItem(ctx, &item); err != nil {
		return "", fmt.Errorf("failed to update item %s: %w", item.ID, err)
	}
	return marshalJSON(map[string]interface{}{"item_id": item.ID, "result": "updated"})
}

func handleLibraryDeleteItem(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryCoreManager(ctx, client)
	if err != nil {
		return "", err
	}
	id, err := requiredStringArg(args, "item_id")
	if err != nil {
		return "", err
	}

	if err := m.DeleteLibraryItem(ctx, &library.Item{ID: id}); err != nil {
		return "", fmt.Errorf("failed to delete item %s: %w", id, err)
	}
	return marshalJSON(map[string]interface{}{"item_id": id, "result": "deleted"})
}

func handleLibraryEvictSubscribedItem(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryCoreManager(ctx, client)
	if err != nil {
		return "", err
	}
	id, err := requiredStringArg(args, "item_id")
	if err != nil {
		return "", err
	}

	if err := m.EvictSubscribedLibraryItem(ctx, &library.Item{ID: id}); err != nil {
		return "", fmt.Errorf("failed to evict subscribed item %s: %w", id, err)
	}
	return marshalJSON(map[string]interface{}{"item_id": id, "result": "evicted"})
}

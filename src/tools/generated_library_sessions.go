package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/vmware/govmomi/vapi/library"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// registerLibrarySessionsTools is the "CL-B" (Content Library — upload/
// download sessions) slice of Fase 8a (Grupo CL-B) of the codegen plan
// (".workspace/plans/MCPVMWare2026-08-10-175300-plano-cobertura-completa-api-codegen.plan.md")
// — referencia/govmomi/vapi/library/library_item_updatesession.go,
// library_item_updatesession_file.go and library_item_downloadsession_file.go,
// hand-transcribed following appliance.go's REST/VAPI conventions (this
// project's first REST-based tool file was Fase 4's appliance.go; every
// vapi/library struct already carries native `json` tags, so — unlike every
// object/*.go-backed group before Fase 8a — arguments are decoded straight
// into the concrete govmomi structs via decodeJSONArg, no generic-decode
// scaffolding needed).
//
// Scope (26 tools total, matching the 3 source files' method counts):
//
//   - Update session lifecycle (library_item_updatesession.go, update-session
//     half): create/get/list/cancel/complete/delete/fail/keep-alive, plus
//     WaitOn (fused into 1 tool — see below). REST route
//     "/com/vmware/content/library/item/update-session".
//   - Download session lifecycle (library_item_updatesession.go,
//     download-session half): create/get/list/cancel/delete/fail/keep-alive
//     (no "complete" — the real API has none for downloads either). REST
//     route "/com/vmware/content/library/item/download-session".
//   - Update session files (library_item_updatesession_file.go): add,
//     add-from-uri, get, list, probe, remove, validate. REST route
//     "/com/vmware/content/library/item/updatesession/file".
//   - Download session files (library_item_downloadsession_file.go): get,
//     list, prepare. REST route
//     "/com/vmware/content/library/item/downloadsession/file".
//
// Curation:
//
//   - WaitOnLibraryItemUpdateSession's Go signature takes an
//     "intervalCallback func()" — a client-side progress callback with no
//     JSON-RPC representation. Fused into vmware_library_wait_on_library_item_update_session,
//     which always passes nil for that callback and always bounds the call
//     with a context.WithTimeout (waitTimeoutFrom — the same
//     defaultWaitTimeout/timeout_seconds pattern as vmware_task_wait in
//     generated_task.go), because the underlying loop
//     (for { Get; if not ACTIVE return; time.Sleep(interval) }) is otherwise
//     unbounded — an orphaned/stuck session would hang this tool call, and
//     with it this project's single-request-at-a-time stdio server, forever.
//     Note time.Sleep between polls is not itself ctx-aware, but the next
//     iteration's Get call is bound to the same ctx and returns a context
//     error the instant the deadline passes, so the real worst-case overrun
//     is one poll_interval_seconds, not "forever" — confirmed by reading
//     WaitOnLibraryItemUpdateSession's loop body, not assumed. On success
//     (session left ACTIVE), this tool does one extra
//     GetLibraryItemUpdateSession call (WaitOn itself only returns error,
//     not the final Session) so the caller gets the terminal state back
//     instead of an empty ack.
//
//   - AddLibraryItemFileFromURI's variadic "checksum ...Checksum" (valid
//     only as 0 or 1 elements — see its own doc comment/error path) is
//     exposed as an optional JSON array argument ("checksum"), decoded into
//     []library.Checksum and passed through as-is; a caller passing 2+
//     entries gets AddLibraryItemFileFromURI's own
//     "expected 0 or 1 checksum, got N" error rather than a second,
//     redundant pre-check in this file.
//
//   - RemoveLibraryItemUpdateSessionFile's real API contract (per its Go doc
//     comment) is "requests a file to be removed; only effectively removed
//     when the update session is completed" — a soft per-file mark, not an
//     immediate delete. vcsim's handler for this action is simplified (it
//     deletes the whole update session rather than tracking a per-file
//     removal mark) — see generated_library_sessions_test.go's test for this
//     tool for the concrete, observed vcsim behavior and how the test scopes
//     around it. This tool's description below documents the real API's
//     contract, not vcsim's simplification.
func registerLibrarySessionsTools(r *Registry) {
	emptySchema := map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}
	updateSessionIDArg := map[string]interface{}{
		"type":        "string",
		"description": "Update session ID, as returned by vmware_library_create_library_item_update_session.",
	}
	downloadSessionIDArg := map[string]interface{}{
		"type":        "string",
		"description": "Download session ID, as returned by vmware_library_create_library_item_download_session.",
	}
	fileNameArg := map[string]interface{}{
		"type":        "string",
		"description": "File name as recorded in the session (the file_spec.name used when it was added, or a name seen via the session's file-list tool).",
	}
	libraryItemIDArg := map[string]interface{}{
		"type":        "string",
		"description": "Library item ID (a content library item's \"id\" field, e.g. from a content-library item-listing tool) to associate the session with.",
	}

	// --- Update session lifecycle ---

	r.registerDestructive("vmware_library_create_library_item_update_session",
		"Create a new content library item update session (POST .../item/update-session), used to upload/replace a library item's files. Returns the new session's ID.",
		tier2,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"library_item_id": libraryItemIDArg, "confirm": confirmArg},
			"required":   []interface{}{"library_item_id", "confirm"},
		},
		Tool{Handler: handleLibraryCreateLibraryItemUpdateSession},
	)

	r.register("vmware_library_get_library_item_update_session",
		"Get an update session's information and status (id, library_item_id, state, client_progress, expiration_time, error_message).",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"session_id": updateSessionIDArg},
			"required":   []interface{}{"session_id"},
		},
		Tool{Handler: handleLibraryGetLibraryItemUpdateSession},
	)

	r.register("vmware_library_list_library_item_update_session",
		"List every update session's ID currently known to the server.",
		emptySchema,
		Tool{Handler: handleLibraryListLibraryItemUpdateSession},
	)

	r.registerDestructive("vmware_library_cancel_library_item_update_session",
		"Cancel an update session.",
		tier2,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"session_id": updateSessionIDArg, "confirm": confirmArg},
			"required":   []interface{}{"session_id", "confirm"},
		},
		Tool{Handler: handleLibraryCancelLibraryItemUpdateSession},
	)

	r.registerDestructive("vmware_library_complete_library_item_update_session",
		"Complete an update session, committing every file added to it into the library item. Real vCenter/vcsim process this asynchronously — poll with vmware_library_get_library_item_update_session or block with vmware_library_wait_on_library_item_update_session to observe the final state (state leaves \"ACTIVE\").",
		tier2,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"session_id": updateSessionIDArg, "confirm": confirmArg},
			"required":   []interface{}{"session_id", "confirm"},
		},
		Tool{Handler: handleLibraryCompleteLibraryItemUpdateSession},
	)

	r.registerDestructive("vmware_library_delete_library_item_update_session",
		"Delete an update session (DELETE .../item/update-session/{id}), discarding it and any files added but not yet committed via complete.",
		tier1,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"session_id": updateSessionIDArg, "confirm": confirmArg},
			"required":   []interface{}{"session_id", "confirm"},
		},
		Tool{Handler: handleLibraryDeleteLibraryItemUpdateSession},
	)

	r.registerDestructive("vmware_library_fail_library_item_update_session",
		"Mark an update session as failed.",
		tier2,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"session_id": updateSessionIDArg, "confirm": confirmArg},
			"required":   []interface{}{"session_id", "confirm"},
		},
		Tool{Handler: handleLibraryFailLibraryItemUpdateSession},
	)

	r.registerDestructive("vmware_library_keep_alive_library_item_update_session",
		"Keep an otherwise-idle update session alive past its expiration_time (sessions expire and are cleaned up automatically otherwise).",
		tier2,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"session_id": updateSessionIDArg, "confirm": confirmArg},
			"required":   []interface{}{"session_id", "confirm"},
		},
		Tool{Handler: handleLibraryKeepAliveLibraryItemUpdateSession},
	)

	r.register("vmware_library_wait_on_library_item_update_session",
		"Block until an update session leaves the ACTIVE state (e.g. after vmware_library_complete_library_item_update_session), then return its final state. Fuses object-model WaitOnLibraryItemUpdateSession's client-side progress callback out (no JSON-RPC representation — see this file's top doc comment) and always bounds the wait with timeout_seconds (default 300s) so a stuck/orphaned session cannot block the server indefinitely. Returns an error if the session ends in the ERROR state.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"session_id":            updateSessionIDArg,
				"poll_interval_seconds": map[string]interface{}{"type": "integer", "description": "Delay between polls, in seconds. Default 2."},
				"timeout_seconds":       timeoutSecondsArg,
			},
			"required": []interface{}{"session_id"},
		},
		Tool{Handler: handleLibraryWaitOnLibraryItemUpdateSession},
	)

	// --- Download session lifecycle ---

	r.registerDestructive("vmware_library_create_library_item_download_session",
		"Create a new content library item download session (POST .../item/download-session), used to download a library item's files. Returns the new session's ID.",
		tier2,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"library_item_id": libraryItemIDArg, "confirm": confirmArg},
			"required":   []interface{}{"library_item_id", "confirm"},
		},
		Tool{Handler: handleLibraryCreateLibraryItemDownloadSession},
	)

	r.register("vmware_library_get_library_item_download_session",
		"Get a download session's information and status (id, library_item_id, state, client_progress, expiration_time, error_message).",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"session_id": downloadSessionIDArg},
			"required":   []interface{}{"session_id"},
		},
		Tool{Handler: handleLibraryGetLibraryItemDownloadSession},
	)

	r.register("vmware_library_list_library_item_download_session",
		"List every download session's ID currently known to the server.",
		emptySchema,
		Tool{Handler: handleLibraryListLibraryItemDownloadSession},
	)

	r.registerDestructive("vmware_library_cancel_library_item_download_session",
		"Cancel a download session.",
		tier2,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"session_id": downloadSessionIDArg, "confirm": confirmArg},
			"required":   []interface{}{"session_id", "confirm"},
		},
		Tool{Handler: handleLibraryCancelLibraryItemDownloadSession},
	)

	r.registerDestructive("vmware_library_delete_library_item_download_session",
		"Delete a download session (DELETE .../item/download-session/{id}).",
		tier1,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"session_id": downloadSessionIDArg, "confirm": confirmArg},
			"required":   []interface{}{"session_id", "confirm"},
		},
		Tool{Handler: handleLibraryDeleteLibraryItemDownloadSession},
	)

	r.registerDestructive("vmware_library_fail_library_item_download_session",
		"Mark a download session as failed.",
		tier2,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"session_id": downloadSessionIDArg, "confirm": confirmArg},
			"required":   []interface{}{"session_id", "confirm"},
		},
		Tool{Handler: handleLibraryFailLibraryItemDownloadSession},
	)

	r.registerDestructive("vmware_library_keep_alive_library_item_download_session",
		"Keep an otherwise-idle download session alive past its expiration_time (sessions expire and are cleaned up automatically otherwise).",
		tier2,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"session_id": downloadSessionIDArg, "confirm": confirmArg},
			"required":   []interface{}{"session_id", "confirm"},
		},
		Tool{Handler: handleLibraryKeepAliveLibraryItemDownloadSession},
	)

	// --- Update session files ---

	r.registerDestructive("vmware_library_add_library_item_file",
		`Add a file to an update session (POST .../updatesession/file/{session_id}?~action=add). file_spec matches library.UpdateFile's JSON shape: {"name": "...", "source_type": "PUSH"|"PULL", "source_endpoint": {"uri": "...", "ssl_certificate": "...", "ssl_certificate_thumbprint": "..."} (required when source_type is PULL), "checksum_info": {"algorithm": "...", "checksum": "..."}}. For PUSH, the response's upload_endpoint.uri is where the caller must separately PUT the file bytes — this server has no byte-transfer tool. For a simpler PULL-from-URL flow, prefer vmware_library_add_library_item_file_from_uri instead.`,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"session_id": updateSessionIDArg,
				"file_spec":  map[string]interface{}{"type": "object", "description": "A library.UpdateFile JSON object — see this tool's description for the shape."},
				"confirm":    confirmArg,
			},
			"required": []interface{}{"session_id", "file_spec", "confirm"},
		},
		Tool{Handler: handleLibraryAddLibraryItemFile},
	)

	r.registerDestructive("vmware_library_add_library_item_file_from_uri",
		`Add a file to an update session by pulling it from a remote http(s) URI (AddLibraryItemFileFromURI — probes the URI first via HEAD/ProbeTransferEndpoint, then adds it as a PULL-source file). "checksum" is optional: 0 or 1 objects of {"algorithm": "...", "checksum": "..."} — passing 2+ fails with the real API's own "expected 0 or 1 checksum" error.`,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"session_id": updateSessionIDArg,
				"name":       map[string]interface{}{"type": "string", "description": "Destination file name within the library item."},
				"uri":        map[string]interface{}{"type": "string", "description": "Source http(s) URL to pull the file from."},
				"checksum": map[string]interface{}{
					"type":        "array",
					"description": "0 or 1 checksum objects: [{\"algorithm\": \"SHA1\"|\"SHA256\"|..., \"checksum\": \"hex digest\"}].",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"algorithm": map[string]interface{}{"type": "string"},
							"checksum":  map[string]interface{}{"type": "string"},
						},
					},
				},
				"confirm": confirmArg,
			},
			"required": []interface{}{"session_id", "name", "uri", "confirm"},
		},
		Tool{Handler: handleLibraryAddLibraryItemFileFromURI},
	)

	r.register("vmware_library_get_library_item_update_session_file",
		"Get information about one specific file in an update session (POST .../updatesession/file/{session_id}?~action=get).",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"session_id": updateSessionIDArg, "file_name": fileNameArg},
			"required":   []interface{}{"session_id", "file_name"},
		},
		Tool{Handler: handleLibraryGetLibraryItemUpdateSessionFile},
	)

	r.register("vmware_library_list_library_item_update_session_file",
		"List every file added to an update session so far.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"session_id": updateSessionIDArg},
			"required":   []interface{}{"session_id"},
		},
		Tool{Handler: handleLibraryListLibraryItemUpdateSessionFile},
	)

	r.registerDestructive("vmware_library_probe_transfer_endpoint",
		"Probe a remote http(s) URI (POST .../updatesession/file?~action=probe) to check reachability and retrieve its TLS certificate/thumbprint before adding it as a PULL source — the same probe AddLibraryItemFileFromURI runs internally when a plain HEAD request fails. Tier2: this reaches out to an arbitrary caller-supplied URI from the server.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"uri":                        map[string]interface{}{"type": "string", "description": "http(s) URL to probe."},
				"ssl_certificate":            map[string]interface{}{"type": "string", "description": "Optional PEM certificate to pin, matching library.TransferEndpoint.SSLCertificate."},
				"ssl_certificate_thumbprint": map[string]interface{}{"type": "string", "description": "Optional SHA-1 thumbprint to pin, matching library.TransferEndpoint.SSLCertificateThumbprint."},
				"confirm":                    confirmArg,
			},
			"required": []interface{}{"uri", "confirm"},
		},
		Tool{Handler: handleLibraryProbeTransferEndpoint},
	)

	r.registerDestructive("vmware_library_remove_library_item_update_session_file",
		`Request a file be removed from an update session (POST .../updatesession/file/{session_id}?~action=remove). Per the real API's contract, this is a soft mark — the file is only effectively removed once the session is completed via vmware_library_complete_library_item_update_session, not immediately.`,
		tier1,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"session_id": updateSessionIDArg, "file_name": fileNameArg, "confirm": confirmArg},
			"required":   []interface{}{"session_id", "file_name", "confirm"},
		},
		Tool{Handler: handleLibraryRemoveLibraryItemUpdateSessionFile},
	)

	r.registerDestructive("vmware_library_validate_library_item_update_session_file",
		"Validate every file added to an update session so far (POST .../updatesession/file/{session_id}?~action=validate) — checks for missing/invalid files (e.g. an incomplete OVF file set) before completing the session. Fails if the session is not ACTIVE.",
		tier2,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"session_id": updateSessionIDArg, "confirm": confirmArg},
			"required":   []interface{}{"session_id", "confirm"},
		},
		Tool{Handler: handleLibraryValidateLibraryItemUpdateSessionFile},
	)

	// --- Download session files ---

	r.register("vmware_library_get_library_item_download_session_file",
		"Get information about one specific file in a download session (POST .../downloadsession/file/{session_id}?~action=get).",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"session_id": downloadSessionIDArg, "file_name": fileNameArg},
			"required":   []interface{}{"session_id", "file_name"},
		},
		Tool{Handler: handleLibraryGetLibraryItemDownloadSessionFile},
	)

	r.register("vmware_library_list_library_item_download_session_file",
		"List every file available in a download session (pre-populated from the library item's files when the session was created).",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"session_id": downloadSessionIDArg},
			"required":   []interface{}{"session_id"},
		},
		Tool{Handler: handleLibraryListLibraryItemDownloadSessionFile},
	)

	r.registerDestructive("vmware_library_prepare_library_item_download_session_file",
		"Prepare a file for download (POST .../downloadsession/file/{session_id}?~action=prepare), returning a download_endpoint.uri the caller must separately GET to fetch the bytes — this server has no byte-transfer tool.",
		tier2,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"session_id": downloadSessionIDArg, "file_name": fileNameArg, "confirm": confirmArg},
			"required":   []interface{}{"session_id", "file_name", "confirm"},
		},
		Tool{Handler: handleLibraryPrepareLibraryItemDownloadSessionFile},
	)
}

// libraryManager returns a *library.Manager bound to client's lazily-created
// REST/VAPI session (vmware.Client.REST) — every handler in this file uses
// it as its entry point, following appliance.go's applianceGet pattern (this
// project's first REST-based tool file, Fase 4).
func libraryManager(ctx context.Context, client *vmware.Client) (*library.Manager, error) {
	rc, err := client.REST(ctx)
	if err != nil {
		return nil, err // REST() already names the likely cause
	}
	return library.NewManager(rc), nil
}

// requiredArgString reads a required string argument, erroring with its key
// name if missing/empty — the same inline pattern resolveTaskArg uses in
// generated_task.go, factored out here since this file has many more
// single-string-argument handlers than that one did.
func requiredArgString(args map[string]interface{}, key string) (string, error) {
	v, _ := args[key].(string)
	if v == "" {
		return "", fmt.Errorf("%s is required", key)
	}
	return v, nil
}

// --- Update session lifecycle handlers ---

func handleLibraryCreateLibraryItemUpdateSession(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryManager(ctx, client)
	if err != nil {
		return "", err
	}
	itemID, err := requiredArgString(args, "library_item_id")
	if err != nil {
		return "", err
	}

	id, err := m.CreateLibraryItemUpdateSession(ctx, library.Session{LibraryItemID: itemID})
	if err != nil {
		return "", fmt.Errorf("failed to create update session for library item %s: %w", itemID, err)
	}
	return marshalJSON(map[string]interface{}{"session_id": id, "library_item_id": itemID})
}

func handleLibraryGetLibraryItemUpdateSession(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryManager(ctx, client)
	if err != nil {
		return "", err
	}
	id, err := requiredArgString(args, "session_id")
	if err != nil {
		return "", err
	}

	session, err := m.GetLibraryItemUpdateSession(ctx, id)
	if err != nil {
		return "", fmt.Errorf("failed to get update session %s: %w", id, err)
	}
	return marshalJSON(session)
}

func handleLibraryListLibraryItemUpdateSession(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryManager(ctx, client)
	if err != nil {
		return "", err
	}

	ids, err := m.ListLibraryItemUpdateSession(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to list update sessions: %w", err)
	}
	return marshalJSON(map[string]interface{}{"session_ids": ids})
}

func handleLibraryCancelLibraryItemUpdateSession(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryManager(ctx, client)
	if err != nil {
		return "", err
	}
	id, err := requiredArgString(args, "session_id")
	if err != nil {
		return "", err
	}

	if err := m.CancelLibraryItemUpdateSession(ctx, id); err != nil {
		return "", fmt.Errorf("failed to cancel update session %s: %w", id, err)
	}
	return marshalJSON(map[string]interface{}{"session_id": id, "result": "cancel_requested"})
}

func handleLibraryCompleteLibraryItemUpdateSession(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryManager(ctx, client)
	if err != nil {
		return "", err
	}
	id, err := requiredArgString(args, "session_id")
	if err != nil {
		return "", err
	}

	if err := m.CompleteLibraryItemUpdateSession(ctx, id); err != nil {
		return "", fmt.Errorf("failed to complete update session %s: %w", id, err)
	}
	return marshalJSON(map[string]interface{}{"session_id": id, "result": "complete_requested"})
}

func handleLibraryDeleteLibraryItemUpdateSession(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryManager(ctx, client)
	if err != nil {
		return "", err
	}
	id, err := requiredArgString(args, "session_id")
	if err != nil {
		return "", err
	}

	if err := m.DeleteLibraryItemUpdateSession(ctx, id); err != nil {
		return "", fmt.Errorf("failed to delete update session %s: %w", id, err)
	}
	return marshalJSON(map[string]interface{}{"session_id": id, "result": "deleted"})
}

func handleLibraryFailLibraryItemUpdateSession(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryManager(ctx, client)
	if err != nil {
		return "", err
	}
	id, err := requiredArgString(args, "session_id")
	if err != nil {
		return "", err
	}

	if err := m.FailLibraryItemUpdateSession(ctx, id); err != nil {
		return "", fmt.Errorf("failed to fail update session %s: %w", id, err)
	}
	return marshalJSON(map[string]interface{}{"session_id": id, "result": "fail_requested"})
}

func handleLibraryKeepAliveLibraryItemUpdateSession(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryManager(ctx, client)
	if err != nil {
		return "", err
	}
	id, err := requiredArgString(args, "session_id")
	if err != nil {
		return "", err
	}

	if err := m.KeepAliveLibraryItemUpdateSession(ctx, id); err != nil {
		return "", fmt.Errorf("failed to keep-alive update session %s: %w", id, err)
	}
	return marshalJSON(map[string]interface{}{"session_id": id, "result": "keep_alive_sent"})
}

func handleLibraryWaitOnLibraryItemUpdateSession(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryManager(ctx, client)
	if err != nil {
		return "", err
	}
	id, err := requiredArgString(args, "session_id")
	if err != nil {
		return "", err
	}

	waitCtx, cancel, err := waitTimeoutFrom(ctx, args)
	if err != nil {
		return "", err
	}
	defer cancel()

	interval := 2 * time.Second
	if v, ok := args["poll_interval_seconds"]; ok {
		n, err := toInt64(v)
		if err != nil {
			return "", fmt.Errorf("invalid poll_interval_seconds: %w", err)
		}
		if n <= 0 {
			return "", fmt.Errorf("poll_interval_seconds must be positive, got %d", n)
		}
		interval = time.Duration(n) * time.Second
	}

	if err := m.WaitOnLibraryItemUpdateSession(waitCtx, id, interval, nil); err != nil {
		return "", fmt.Errorf("update session %s did not finish successfully: %w", id, err)
	}

	// WaitOnLibraryItemUpdateSession only returns an error, not the final
	// Session — fetch it (via the outer, un-timed-out ctx) so the caller
	// gets the terminal state back instead of a bare ack.
	final, err := m.GetLibraryItemUpdateSession(ctx, id)
	if err != nil {
		return "", fmt.Errorf("update session %s finished but a follow-up get failed: %w", id, err)
	}
	return marshalJSON(final)
}

// --- Download session lifecycle handlers ---

func handleLibraryCreateLibraryItemDownloadSession(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryManager(ctx, client)
	if err != nil {
		return "", err
	}
	itemID, err := requiredArgString(args, "library_item_id")
	if err != nil {
		return "", err
	}

	id, err := m.CreateLibraryItemDownloadSession(ctx, library.Session{LibraryItemID: itemID})
	if err != nil {
		return "", fmt.Errorf("failed to create download session for library item %s: %w", itemID, err)
	}
	return marshalJSON(map[string]interface{}{"session_id": id, "library_item_id": itemID})
}

func handleLibraryGetLibraryItemDownloadSession(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryManager(ctx, client)
	if err != nil {
		return "", err
	}
	id, err := requiredArgString(args, "session_id")
	if err != nil {
		return "", err
	}

	session, err := m.GetLibraryItemDownloadSession(ctx, id)
	if err != nil {
		return "", fmt.Errorf("failed to get download session %s: %w", id, err)
	}
	return marshalJSON(session)
}

func handleLibraryListLibraryItemDownloadSession(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryManager(ctx, client)
	if err != nil {
		return "", err
	}

	ids, err := m.ListLibraryItemDownloadSession(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to list download sessions: %w", err)
	}
	return marshalJSON(map[string]interface{}{"session_ids": ids})
}

func handleLibraryCancelLibraryItemDownloadSession(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryManager(ctx, client)
	if err != nil {
		return "", err
	}
	id, err := requiredArgString(args, "session_id")
	if err != nil {
		return "", err
	}

	if err := m.CancelLibraryItemDownloadSession(ctx, id); err != nil {
		return "", fmt.Errorf("failed to cancel download session %s: %w", id, err)
	}
	return marshalJSON(map[string]interface{}{"session_id": id, "result": "cancel_requested"})
}

func handleLibraryDeleteLibraryItemDownloadSession(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryManager(ctx, client)
	if err != nil {
		return "", err
	}
	id, err := requiredArgString(args, "session_id")
	if err != nil {
		return "", err
	}

	if err := m.DeleteLibraryItemDownloadSession(ctx, id); err != nil {
		return "", fmt.Errorf("failed to delete download session %s: %w", id, err)
	}
	return marshalJSON(map[string]interface{}{"session_id": id, "result": "deleted"})
}

func handleLibraryFailLibraryItemDownloadSession(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryManager(ctx, client)
	if err != nil {
		return "", err
	}
	id, err := requiredArgString(args, "session_id")
	if err != nil {
		return "", err
	}

	if err := m.FailLibraryItemDownloadSession(ctx, id); err != nil {
		return "", fmt.Errorf("failed to fail download session %s: %w", id, err)
	}
	return marshalJSON(map[string]interface{}{"session_id": id, "result": "fail_requested"})
}

func handleLibraryKeepAliveLibraryItemDownloadSession(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryManager(ctx, client)
	if err != nil {
		return "", err
	}
	id, err := requiredArgString(args, "session_id")
	if err != nil {
		return "", err
	}

	if err := m.KeepAliveLibraryItemDownloadSession(ctx, id); err != nil {
		return "", fmt.Errorf("failed to keep-alive download session %s: %w", id, err)
	}
	return marshalJSON(map[string]interface{}{"session_id": id, "result": "keep_alive_sent"})
}

// --- Update session file handlers ---

func handleLibraryAddLibraryItemFile(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryManager(ctx, client)
	if err != nil {
		return "", err
	}
	id, err := requiredArgString(args, "session_id")
	if err != nil {
		return "", err
	}

	raw, ok := args["file_spec"]
	if !ok {
		return "", fmt.Errorf("file_spec is required")
	}
	var file library.UpdateFile
	if err := decodeJSONArg(raw, &file); err != nil {
		return "", fmt.Errorf("invalid file_spec: %w", err)
	}

	res, err := m.AddLibraryItemFile(ctx, id, file)
	if err != nil {
		return "", fmt.Errorf("failed to add file to update session %s: %w", id, err)
	}
	return marshalJSON(res)
}

func handleLibraryAddLibraryItemFileFromURI(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryManager(ctx, client)
	if err != nil {
		return "", err
	}
	id, err := requiredArgString(args, "session_id")
	if err != nil {
		return "", err
	}
	name, err := requiredArgString(args, "name")
	if err != nil {
		return "", err
	}
	uri, err := requiredArgString(args, "uri")
	if err != nil {
		return "", err
	}

	var checksums []library.Checksum
	if raw, ok := args["checksum"]; ok && raw != nil {
		if err := decodeJSONArg(raw, &checksums); err != nil {
			return "", fmt.Errorf("invalid checksum: %w", err)
		}
	}

	res, err := m.AddLibraryItemFileFromURI(ctx, id, name, uri, checksums...)
	if err != nil {
		return "", fmt.Errorf("failed to add file %q from %s to update session %s: %w", name, uri, id, err)
	}
	return marshalJSON(res)
}

func handleLibraryGetLibraryItemUpdateSessionFile(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryManager(ctx, client)
	if err != nil {
		return "", err
	}
	id, err := requiredArgString(args, "session_id")
	if err != nil {
		return "", err
	}
	name, err := requiredArgString(args, "file_name")
	if err != nil {
		return "", err
	}

	res, err := m.GetLibraryItemUpdateSessionFile(ctx, id, name)
	if err != nil {
		return "", fmt.Errorf("failed to get file %q in update session %s: %w", name, id, err)
	}
	return marshalJSON(res)
}

func handleLibraryListLibraryItemUpdateSessionFile(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryManager(ctx, client)
	if err != nil {
		return "", err
	}
	id, err := requiredArgString(args, "session_id")
	if err != nil {
		return "", err
	}

	res, err := m.ListLibraryItemUpdateSessionFile(ctx, id)
	if err != nil {
		return "", fmt.Errorf("failed to list files in update session %s: %w", id, err)
	}
	return marshalJSON(map[string]interface{}{"session_id": id, "files": res})
}

func handleLibraryProbeTransferEndpoint(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryManager(ctx, client)
	if err != nil {
		return "", err
	}
	uri, err := requiredArgString(args, "uri")
	if err != nil {
		return "", err
	}

	endpoint := library.TransferEndpoint{URI: uri}
	if v, ok := args["ssl_certificate"].(string); ok {
		endpoint.SSLCertificate = v
	}
	if v, ok := args["ssl_certificate_thumbprint"].(string); ok {
		endpoint.SSLCertificateThumbprint = v
	}

	res, err := m.ProbeTransferEndpoint(ctx, endpoint)
	if err != nil {
		return "", fmt.Errorf("failed to probe transfer endpoint %s: %w", uri, err)
	}
	return marshalJSON(res)
}

func handleLibraryRemoveLibraryItemUpdateSessionFile(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryManager(ctx, client)
	if err != nil {
		return "", err
	}
	id, err := requiredArgString(args, "session_id")
	if err != nil {
		return "", err
	}
	name, err := requiredArgString(args, "file_name")
	if err != nil {
		return "", err
	}

	if err := m.RemoveLibraryItemUpdateSessionFile(ctx, id, name); err != nil {
		return "", fmt.Errorf("failed to remove file %q from update session %s: %w", name, id, err)
	}
	return marshalJSON(map[string]interface{}{"session_id": id, "file_name": name, "result": "remove_requested"})
}

func handleLibraryValidateLibraryItemUpdateSessionFile(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryManager(ctx, client)
	if err != nil {
		return "", err
	}
	id, err := requiredArgString(args, "session_id")
	if err != nil {
		return "", err
	}

	res, err := m.ValidateLibraryItemUpdateSessionFile(ctx, id)
	if err != nil {
		return "", fmt.Errorf("failed to validate files in update session %s: %w", id, err)
	}
	return marshalJSON(res)
}

// --- Download session file handlers ---

func handleLibraryGetLibraryItemDownloadSessionFile(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryManager(ctx, client)
	if err != nil {
		return "", err
	}
	id, err := requiredArgString(args, "session_id")
	if err != nil {
		return "", err
	}
	name, err := requiredArgString(args, "file_name")
	if err != nil {
		return "", err
	}

	res, err := m.GetLibraryItemDownloadSessionFile(ctx, id, name)
	if err != nil {
		return "", fmt.Errorf("failed to get file %q in download session %s: %w", name, id, err)
	}
	return marshalJSON(res)
}

func handleLibraryListLibraryItemDownloadSessionFile(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryManager(ctx, client)
	if err != nil {
		return "", err
	}
	id, err := requiredArgString(args, "session_id")
	if err != nil {
		return "", err
	}

	res, err := m.ListLibraryItemDownloadSessionFile(ctx, id)
	if err != nil {
		return "", fmt.Errorf("failed to list files in download session %s: %w", id, err)
	}
	return marshalJSON(map[string]interface{}{"session_id": id, "files": res})
}

func handleLibraryPrepareLibraryItemDownloadSessionFile(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryManager(ctx, client)
	if err != nil {
		return "", err
	}
	id, err := requiredArgString(args, "session_id")
	if err != nil {
		return "", err
	}
	name, err := requiredArgString(args, "file_name")
	if err != nil {
		return "", err
	}

	res, err := m.PrepareLibraryItemDownloadSessionFile(ctx, id, name)
	if err != nil {
		return "", fmt.Errorf("failed to prepare file %q in download session %s: %w", name, id, err)
	}
	return marshalJSON(res)
}

package tools

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vapi/library"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// newLibrarySessionsRegistry builds a Registry the normal way and layers
// this group's (CL-B) tools on top via withClass — same pattern as
// generated_task_test.go's newTaskRegistry. This file must not edit
// registry.go itself (the orchestrator wires that in after integrating all
// parallel groups of Fase 8a's first wave).
func newLibrarySessionsRegistry(ctx context.Context, c *vmware.Client, opts RegistryOptions) *Registry {
	r := NewRegistry(ctx, c, opts)
	r.withClass(modeVCenterOnly, registerLibrarySessionsTools)
	return r
}

// createTestLibraryItem builds a fresh LOCAL content library (backed by the
// simulator's default datastore) and one empty item inside it, directly via
// library.NewManager against the vcsim REST session — this group's tools
// only cover update/download sessions, not library/item CRUD (a separate
// parallel CL-A group's scope), so the fixture is built by hand here per the
// orchestrator's brief rather than through a tool this file doesn't own.
func createTestLibraryItem(t *testing.T, ctx context.Context, c *vmware.Client) (libraryID, itemID string) {
	t.Helper()

	rc, err := c.REST(ctx)
	if err != nil {
		t.Fatalf("REST() failed: %v", err)
	}
	m := library.NewManager(rc)

	ds, err := c.Finder.DefaultDatastore(ctx)
	if err != nil {
		t.Fatalf("DefaultDatastore failed: %v", err)
	}

	libID, err := m.CreateLibrary(ctx, library.Library{
		Name: fmt.Sprintf("cl-b-test-%d", time.Now().UnixNano()),
		Type: "LOCAL",
		Storage: []library.StorageBacking{{
			DatastoreID: ds.Reference().Value,
			Type:        "DATASTORE",
		}},
	})
	if err != nil {
		t.Fatalf("CreateLibrary fixture setup failed: %v", err)
	}

	desc := "CL-B fixture item"
	itmID, err := m.CreateLibraryItem(ctx, library.Item{
		Name:        "item1",
		Description: &desc,
		LibraryID:   libID,
		Type:        library.ItemTypeISO,
	})
	if err != nil {
		t.Fatalf("CreateLibraryItem fixture setup failed: %v", err)
	}

	return libID, itmID
}

// newFileServer serves fixed content over plain HTTP for
// vmware_library_add_library_item_file_from_uri / vmware_library_probe_transfer_endpoint
// to pull/probe against — vcsim's pullSource does a real http.Client.Get
// against this URI, and AddLibraryItemFileFromURI itself first tries an
// http.Head against it.
func newFileServer(content string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(content)))
		w.WriteHeader(http.StatusOK)
		if r.Method != http.MethodHead {
			_, _ = w.Write([]byte(content))
		}
	}))
}

// TestLibrarySessionsTools_UpdateSessionFullCycle drives the plausible
// end-to-end flow the orchestrator specified: create an update session for a
// real library item, add a file to it by pulling from a real (httptest)
// URI, list/get/validate it, complete the session, block on
// vmware_library_wait_on_library_item_update_session until it leaves ACTIVE,
// then exercise the resulting download session (which vcsim pre-populates
// from the item's now-committed file) through get/list/prepare, ending with
// cancel. Covers 15 of this file's 26 tools with real vcsim round trips, not
// registration-only smoke tests.
func TestLibrarySessionsTools_UpdateSessionFullCycle(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()
	ctx := context.Background()

	_, itemID := createTestLibraryItem(t, ctx, c)

	r := newLibrarySessionsRegistry(ctx, c, RegistryOptions{AllowDestructive: true})

	// create_library_item_update_session
	raw, err := r.CallTool("vmware_library_create_library_item_update_session", map[string]interface{}{
		"library_item_id": itemID, "confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_library_create_library_item_update_session failed: %v", err)
	}
	sessionID, _ := decodeResult(t, raw)["session_id"].(string)
	if sessionID == "" {
		t.Fatalf("expected a non-empty session_id, got %q", raw)
	}

	// get_library_item_update_session — proves it is really ACTIVE and tied
	// to the item we just created it for.
	raw, err = r.CallTool("vmware_library_get_library_item_update_session", map[string]interface{}{"session_id": sessionID})
	if err != nil {
		t.Fatalf("vmware_library_get_library_item_update_session failed: %v", err)
	}
	got := decodeResult(t, raw)
	if got["state"] != "ACTIVE" {
		t.Fatalf("expected state ACTIVE right after creation, got %v (full: %s)", got["state"], raw)
	}
	if got["library_item_id"] != itemID {
		t.Fatalf("expected library_item_id %s, got %v", itemID, got["library_item_id"])
	}

	// list_library_item_update_session — proves our session is really
	// tracked server-side, not just echoed back.
	raw, err = r.CallTool("vmware_library_list_library_item_update_session", map[string]interface{}{})
	if err != nil {
		t.Fatalf("vmware_library_list_library_item_update_session failed: %v", err)
	}
	ids, _ := decodeResult(t, raw)["session_ids"].([]interface{})
	if !containsString(ids, sessionID) {
		t.Fatalf("expected session_ids to contain %s, got %v", sessionID, ids)
	}

	// keep_alive_library_item_update_session
	if _, err := r.CallTool("vmware_library_keep_alive_library_item_update_session", map[string]interface{}{
		"session_id": sessionID, "confirm": true,
	}); err != nil {
		t.Fatalf("vmware_library_keep_alive_library_item_update_session failed: %v", err)
	}

	srv := newFileServer("cl-b test file content\n")
	defer srv.Close()

	// add_library_item_file_from_uri — the real PULL path: vcsim actually
	// fetches srv.URL/file1.txt in a background goroutine.
	raw, err = r.CallTool("vmware_library_add_library_item_file_from_uri", map[string]interface{}{
		"session_id": sessionID,
		"name":       "file1.txt",
		"uri":        srv.URL + "/file1.txt",
		"confirm":    true,
	})
	if err != nil {
		t.Fatalf("vmware_library_add_library_item_file_from_uri failed: %v", err)
	}
	addedFile := decodeResult(t, raw)
	if addedFile["name"] != "file1.txt" {
		t.Fatalf("expected added file name file1.txt, got %v (full: %s)", addedFile["name"], raw)
	}
	if addedFile["source_type"] != "PULL" {
		t.Fatalf("expected source_type PULL, got %v", addedFile["source_type"])
	}

	// list_library_item_update_session_file
	raw, err = r.CallTool("vmware_library_list_library_item_update_session_file", map[string]interface{}{"session_id": sessionID})
	if err != nil {
		t.Fatalf("vmware_library_list_library_item_update_session_file failed: %v", err)
	}
	files, _ := decodeResult(t, raw)["files"].([]interface{})
	if len(files) != 1 {
		t.Fatalf("expected exactly 1 file in the session, got %d (%s)", len(files), raw)
	}

	// get_library_item_update_session_file
	raw, err = r.CallTool("vmware_library_get_library_item_update_session_file", map[string]interface{}{
		"session_id": sessionID, "file_name": "file1.txt",
	})
	if err != nil {
		t.Fatalf("vmware_library_get_library_item_update_session_file failed: %v", err)
	}
	if decodeResult(t, raw)["name"] != "file1.txt" {
		t.Fatalf("expected to get back file1.txt, got %s", raw)
	}

	// validate_library_item_update_session_file — vcsim always reports
	// has_errors:false for an ACTIVE session (confirmed by reading its
	// handler: it never populates MissingFiles/InvalidFiles), but the call
	// itself proves the tool reaches the real endpoint and only succeeds
	// while ACTIVE (see the not_allowed_in_current_state branch it would hit
	// otherwise).
	raw, err = r.CallTool("vmware_library_validate_library_item_update_session_file", map[string]interface{}{
		"session_id": sessionID, "confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_library_validate_library_item_update_session_file failed: %v", err)
	}
	if decodeResult(t, raw)["has_errors"] != false {
		t.Fatalf("expected has_errors false, got %s", raw)
	}

	// complete_library_item_update_session — vcsim processes this
	// asynchronously (a goroutine that waits on the session's WaitGroup
	// before flipping state to DONE), so the state is not yet guaranteed to
	// have left ACTIVE by the time this call returns.
	if _, err := r.CallTool("vmware_library_complete_library_item_update_session", map[string]interface{}{
		"session_id": sessionID, "confirm": true,
	}); err != nil {
		t.Fatalf("vmware_library_complete_library_item_update_session failed: %v", err)
	}

	// wait_on_library_item_update_session — blocks until the async complete
	// above actually lands, then returns the terminal Session.
	raw, err = r.CallTool("vmware_library_wait_on_library_item_update_session", map[string]interface{}{
		"session_id": sessionID, "poll_interval_seconds": float64(1), "timeout_seconds": float64(30),
	})
	if err != nil {
		t.Fatalf("vmware_library_wait_on_library_item_update_session failed: %v", err)
	}
	final := decodeResult(t, raw)
	if final["state"] != "DONE" {
		t.Fatalf("expected final state DONE, got %v (full: %s)", final["state"], raw)
	}

	// --- Download session: the file committed above should now be visible
	// through a freshly created download session (vcsim pre-populates
	// download session files from the item's committed File list). ---

	raw, err = r.CallTool("vmware_library_create_library_item_download_session", map[string]interface{}{
		"library_item_id": itemID, "confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_library_create_library_item_download_session failed: %v", err)
	}
	dlSessionID, _ := decodeResult(t, raw)["session_id"].(string)
	if dlSessionID == "" {
		t.Fatalf("expected a non-empty download session_id, got %s", raw)
	}

	raw, err = r.CallTool("vmware_library_get_library_item_download_session", map[string]interface{}{"session_id": dlSessionID})
	if err != nil {
		t.Fatalf("vmware_library_get_library_item_download_session failed: %v", err)
	}
	if decodeResult(t, raw)["state"] != "ACTIVE" {
		t.Fatalf("expected download session state ACTIVE, got %s", raw)
	}

	raw, err = r.CallTool("vmware_library_list_library_item_download_session", map[string]interface{}{})
	if err != nil {
		t.Fatalf("vmware_library_list_library_item_download_session failed: %v", err)
	}
	dlIDs, _ := decodeResult(t, raw)["session_ids"].([]interface{})
	if !containsString(dlIDs, dlSessionID) {
		t.Fatalf("expected download session_ids to contain %s, got %v", dlSessionID, dlIDs)
	}

	raw, err = r.CallTool("vmware_library_list_library_item_download_session_file", map[string]interface{}{"session_id": dlSessionID})
	if err != nil {
		t.Fatalf("vmware_library_list_library_item_download_session_file failed: %v", err)
	}
	dlFiles, _ := decodeResult(t, raw)["files"].([]interface{})
	if len(dlFiles) != 1 {
		t.Fatalf("expected exactly 1 file pre-populated in the download session (the file1.txt committed above), got %d (%s)", len(dlFiles), raw)
	}

	raw, err = r.CallTool("vmware_library_get_library_item_download_session_file", map[string]interface{}{
		"session_id": dlSessionID, "file_name": "file1.txt",
	})
	if err != nil {
		t.Fatalf("vmware_library_get_library_item_download_session_file failed: %v", err)
	}
	if decodeResult(t, raw)["status"] != "UNPREPARED" {
		t.Fatalf("expected freshly listed download file status UNPREPARED, got %s", raw)
	}

	raw, err = r.CallTool("vmware_library_prepare_library_item_download_session_file", map[string]interface{}{
		"session_id": dlSessionID, "file_name": "file1.txt", "confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_library_prepare_library_item_download_session_file failed: %v", err)
	}
	prepared := decodeResult(t, raw)
	if prepared["status"] != "PREPARED" {
		t.Fatalf("expected status PREPARED after prepare, got %v (full: %s)", prepared["status"], raw)
	}
	endpoint, _ := prepared["download_endpoint"].(map[string]interface{})
	if endpoint == nil || endpoint["uri"] == "" {
		t.Fatalf("expected a non-empty download_endpoint.uri, got %s", raw)
	}

	if _, err := r.CallTool("vmware_library_keep_alive_library_item_download_session", map[string]interface{}{
		"session_id": dlSessionID, "confirm": true,
	}); err != nil {
		t.Fatalf("vmware_library_keep_alive_library_item_download_session failed: %v", err)
	}

	// cancel_library_item_download_session — vcsim deletes the session
	// outright on cancel (a documented simplification: "TODO: fully mock
	// VC's behavior" in its own source). Confirm it really took effect by
	// proving a follow-up Get now fails, not just that the call itself
	// returned no error.
	if _, err := r.CallTool("vmware_library_cancel_library_item_download_session", map[string]interface{}{
		"session_id": dlSessionID, "confirm": true,
	}); err != nil {
		t.Fatalf("vmware_library_cancel_library_item_download_session failed: %v", err)
	}
	if _, err := r.CallTool("vmware_library_get_library_item_download_session", map[string]interface{}{"session_id": dlSessionID}); err == nil {
		t.Fatalf("expected vmware_library_get_library_item_download_session to fail after cancel, it succeeded")
	}
}

// TestLibrarySessionsTools_AddLibraryItemFilePush proves
// vmware_library_add_library_item_file's PUSH path (the raw file_spec
// argument, as opposed to the URI convenience tool exercised above) reaches
// vcsim and gets back a real upload_endpoint — this server has no
// byte-transfer tool, so PUT-ing to that endpoint is out of scope, but the
// session-file bookkeeping itself is real and worth proving.
func TestLibrarySessionsTools_AddLibraryItemFilePush(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()
	ctx := context.Background()

	_, itemID := createTestLibraryItem(t, ctx, c)
	r := newLibrarySessionsRegistry(ctx, c, RegistryOptions{AllowDestructive: true})

	raw, err := r.CallTool("vmware_library_create_library_item_update_session", map[string]interface{}{
		"library_item_id": itemID, "confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_library_create_library_item_update_session failed: %v", err)
	}
	sessionID, _ := decodeResult(t, raw)["session_id"].(string)

	raw, err = r.CallTool("vmware_library_add_library_item_file", map[string]interface{}{
		"session_id": sessionID,
		"file_spec": map[string]interface{}{
			"name":        "pushed.iso",
			"source_type": "PUSH",
		},
		"confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_library_add_library_item_file failed: %v", err)
	}
	res := decodeResult(t, raw)
	if res["name"] != "pushed.iso" {
		t.Fatalf("expected name pushed.iso, got %v (full: %s)", res["name"], raw)
	}
	upload, _ := res["upload_endpoint"].(map[string]interface{})
	if upload == nil || upload["uri"] == "" {
		t.Fatalf("expected a non-empty upload_endpoint.uri for a PUSH file, got %s", raw)
	}

	// Missing file_spec must be a clean argument error, not a panic/nil deref.
	if _, err := r.CallTool("vmware_library_add_library_item_file", map[string]interface{}{
		"session_id": sessionID, "confirm": true,
	}); err == nil {
		t.Fatalf("expected an error when file_spec is omitted")
	}
}

// TestLibrarySessionsTools_ProbeTransferEndpoint proves
// vmware_library_probe_transfer_endpoint reaches vcsim's real probe handler
// (an HTTP HEAD against the caller-supplied URI, done server-side) rather
// than just echoing the input back.
func TestLibrarySessionsTools_ProbeTransferEndpoint(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()
	ctx := context.Background()

	r := newLibrarySessionsRegistry(ctx, c, RegistryOptions{AllowDestructive: true})

	srv := newFileServer("probe me\n")
	defer srv.Close()

	raw, err := r.CallTool("vmware_library_probe_transfer_endpoint", map[string]interface{}{
		"uri": srv.URL + "/probe.txt", "confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_library_probe_transfer_endpoint failed: %v", err)
	}
	res := decodeResult(t, raw)
	if res["status"] != "SUCCESS" {
		t.Fatalf("expected probe status SUCCESS against a real reachable server, got %v (full: %s)", res["status"], raw)
	}

	// An unsupported scheme must produce a real (non-panicking) INVALID_URL
	// result — this exercises the tool's error-shape path, not just the
	// happy path above.
	raw, err = r.CallTool("vmware_library_probe_transfer_endpoint", map[string]interface{}{
		"uri": "ftp://example.invalid/file", "confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_library_probe_transfer_endpoint on an unsupported scheme should return a result, not a Go error: %v", err)
	}
	if decodeResult(t, raw)["status"] != "INVALID_URL" {
		t.Fatalf("expected status INVALID_URL for an ftp:// uri, got %s", raw)
	}
}

// TestLibrarySessionsTools_RemoveFileVcsimGap documents a real, observed
// vcsim simplification (not a bug in this file's tool code): the real
// vSphere API's RemoveLibraryItemUpdateSessionFile only *marks* a file for
// removal (effective at complete-time — see its Go doc comment), but vcsim's
// handler for the "remove" action unconditionally deletes the *entire*
// update session instead of tracking a per-file removal mark (confirmed by
// reading vapi/simulator/simulator.go's libraryItemUpdateSessionFileID
// "remove" case: `delete(s.Update, id)` where id is the *session* ID from
// the URL, with the file_name from the request body never even decoded).
// This test proves vmware_library_remove_library_item_update_session_file
// itself behaves correctly against that gap (a clean call, no panic) and
// documents the resulting session-gone side effect so nobody rediscovers it
// as a mystery "session not found" later.
func TestLibrarySessionsTools_RemoveFileVcsimGap(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()
	ctx := context.Background()

	_, itemID := createTestLibraryItem(t, ctx, c)
	r := newLibrarySessionsRegistry(ctx, c, RegistryOptions{AllowDestructive: true})

	raw, err := r.CallTool("vmware_library_create_library_item_update_session", map[string]interface{}{
		"library_item_id": itemID, "confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_library_create_library_item_update_session failed: %v", err)
	}
	sessionID, _ := decodeResult(t, raw)["session_id"].(string)

	if _, err := r.CallTool("vmware_library_add_library_item_file", map[string]interface{}{
		"session_id": sessionID,
		"file_spec":  map[string]interface{}{"name": "to-remove.txt", "source_type": "PUSH"},
		"confirm":    true,
	}); err != nil {
		t.Fatalf("vmware_library_add_library_item_file failed: %v", err)
	}

	if _, err := r.CallTool("vmware_library_remove_library_item_update_session_file", map[string]interface{}{
		"session_id": sessionID, "file_name": "to-remove.txt", "confirm": true,
	}); err != nil {
		t.Fatalf("vmware_library_remove_library_item_update_session_file itself should not error against vcsim: %v", err)
	}

	// Documented vcsim gap: the whole session is now gone, not just the
	// file. If this assertion ever starts failing (i.e. Get succeeds), a
	// future govmomi/vcsim update likely implemented real per-file removal
	// — update this test's expectations (and this file's top doc comment)
	// accordingly rather than deleting the check.
	if _, err := r.CallTool("vmware_library_get_library_item_update_session", map[string]interface{}{"session_id": sessionID}); err == nil {
		t.Fatalf("vcsim gap assumption changed: expected the session to be gone after remove, but Get still succeeded")
	}
}

// TestLibrarySessionsTools_FailThenWait proves
// vmware_library_wait_on_library_item_update_session returns a real error
// (never hangs, never silently succeeds) once a session has been failed via
// vmware_library_fail_library_item_update_session. Also documents a real
// finding from running this for real, corrected after an initial wrong
// theory (see git history of this comment if curious — first assumed this
// would panic, then actually ran it and read the output, per this project's
// "never affirm behavior without running and reading it" rule): vcsim's
// "fail" action only flips Session.State to "ERROR" and never populates
// ErrorMessage, so govmomi's own WaitOnLibraryItemUpdateSession — which does
// `return session.ErrorMessage` on an ERROR state — returns a typed-nil
// *rest.LocalizableMessage wrapped in a non-nil error interface (the classic
// Go "nil concrete value in a non-nil interface" gotcha: err != nil is true
// here). This tool's own `fmt.Errorf("...: %w", err)` then formats that
// error; %w/%v call the wrapped value's Error() method, which panics on nil
// receiver `m` accessing `m.DefaultMessage` — but Go's fmt package has a
// built-in guard for exactly this (fmt/print.go's catchPanic: on a
// recovered panic where the arg is a nil pointer, it prints the literal
// string "<nil>" instead of re-panicking or crashing). Confirmed by the
// actual test run: the returned error text is
// `...did not finish successfully: <nil>` — no panic, registry.go's
// CallTool-level panic recovery is never even invoked on this path. This
// test only asserts "CallTool returns a non-nil error" (true either way)
// and logs the exact text for anyone who rediscovers this.
func TestLibrarySessionsTools_FailThenWait(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()
	ctx := context.Background()

	_, itemID := createTestLibraryItem(t, ctx, c)
	r := newLibrarySessionsRegistry(ctx, c, RegistryOptions{AllowDestructive: true})

	raw, err := r.CallTool("vmware_library_create_library_item_update_session", map[string]interface{}{
		"library_item_id": itemID, "confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_library_create_library_item_update_session failed: %v", err)
	}
	sessionID, _ := decodeResult(t, raw)["session_id"].(string)

	if _, err := r.CallTool("vmware_library_fail_library_item_update_session", map[string]interface{}{
		"session_id": sessionID, "confirm": true,
	}); err != nil {
		t.Fatalf("vmware_library_fail_library_item_update_session failed: %v", err)
	}

	_, err = r.CallTool("vmware_library_wait_on_library_item_update_session", map[string]interface{}{
		"session_id": sessionID, "poll_interval_seconds": float64(1), "timeout_seconds": float64(10),
	})
	if err == nil {
		t.Fatalf("expected vmware_library_wait_on_library_item_update_session to report an error after the session was failed")
	}
	t.Logf("wait_on after fail returned (see this test's doc comment for why this shape is expected either way): %v", err)
}

// TestLibrarySessionsTools_CancelDeleteAndListSessions rounds out coverage
// of the remaining lifecycle tools not already exercised by the full-cycle
// test above: cancel/delete on an update session, delete/fail on a download
// session, and confirms both list tools reflect real removals (not just
// additions).
func TestLibrarySessionsTools_CancelDeleteAndListSessions(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()
	ctx := context.Background()

	_, itemID := createTestLibraryItem(t, ctx, c)
	r := newLibrarySessionsRegistry(ctx, c, RegistryOptions{AllowDestructive: true})

	// Update session: create -> cancel -> delete -> confirm gone from list.
	raw, err := r.CallTool("vmware_library_create_library_item_update_session", map[string]interface{}{
		"library_item_id": itemID, "confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_library_create_library_item_update_session failed: %v", err)
	}
	upID, _ := decodeResult(t, raw)["session_id"].(string)

	if _, err := r.CallTool("vmware_library_cancel_library_item_update_session", map[string]interface{}{
		"session_id": upID, "confirm": true,
	}); err != nil {
		t.Fatalf("vmware_library_cancel_library_item_update_session failed: %v", err)
	}
	raw, err = r.CallTool("vmware_library_get_library_item_update_session", map[string]interface{}{"session_id": upID})
	if err != nil {
		t.Fatalf("vmware_library_get_library_item_update_session after cancel failed: %v", err)
	}
	if decodeResult(t, raw)["state"] != "CANCELED" {
		t.Fatalf("expected state CANCELED after cancel, got %s", raw)
	}
	if _, err := r.CallTool("vmware_library_delete_library_item_update_session", map[string]interface{}{
		"session_id": upID, "confirm": true,
	}); err != nil {
		t.Fatalf("vmware_library_delete_library_item_update_session failed: %v", err)
	}
	raw, err = r.CallTool("vmware_library_list_library_item_update_session", map[string]interface{}{})
	if err != nil {
		t.Fatalf("vmware_library_list_library_item_update_session failed: %v", err)
	}
	if ids, _ := decodeResult(t, raw)["session_ids"].([]interface{}); containsString(ids, upID) {
		t.Fatalf("expected %s to be gone from the update session list after delete, got %v", upID, ids)
	}

	// Download session: create -> fail -> delete -> confirm gone from list.
	raw, err = r.CallTool("vmware_library_create_library_item_download_session", map[string]interface{}{
		"library_item_id": itemID, "confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_library_create_library_item_download_session failed: %v", err)
	}
	dlID, _ := decodeResult(t, raw)["session_id"].(string)

	if _, err := r.CallTool("vmware_library_fail_library_item_download_session", map[string]interface{}{
		"session_id": dlID, "confirm": true,
	}); err != nil {
		t.Fatalf("vmware_library_fail_library_item_download_session failed: %v", err)
	}
	// vcsim's download-session "fail" (like "cancel"/"complete") deletes the
	// session outright rather than leaving it queryable in a failed state
	// (see libraryItemDownloadSessionID's handler) — so delete here is
	// exercised against an already-gone session, proving the tool still
	// returns a clean error rather than panicking.
	if _, err := r.CallTool("vmware_library_delete_library_item_download_session", map[string]interface{}{
		"session_id": dlID, "confirm": true,
	}); err == nil {
		t.Fatalf("expected delete of an already-failed(=deleted) download session to error, it succeeded")
	}
	raw, err = r.CallTool("vmware_library_list_library_item_download_session", map[string]interface{}{})
	if err != nil {
		t.Fatalf("vmware_library_list_library_item_download_session failed: %v", err)
	}
	if ids, _ := decodeResult(t, raw)["session_ids"].([]interface{}); containsString(ids, dlID) {
		t.Fatalf("expected %s to be gone from the download session list after fail, got %v", dlID, ids)
	}
}

// TestLibrarySessionsTools_DestructiveGate proves this file's tier1/tier2
// tools are really wired through registerDestructive — closed gate and
// missing confirm both deny before any vcsim round trip — using
// vmware_library_delete_library_item_update_session (tier1) and
// vmware_library_cancel_library_item_update_session (tier2) as
// representatives, the same style as vm_test.go's
// TestVMTools_GateClosedDeniesDestructiveOps.
func TestLibrarySessionsTools_DestructiveGate(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()
	ctx := context.Background()

	_, itemID := createTestLibraryItem(t, ctx, c)

	// Gate closed (AllowDestructive: false, the zero value).
	closedR := newLibrarySessionsRegistry(ctx, c, RegistryOptions{})
	openR := newLibrarySessionsRegistry(ctx, c, RegistryOptions{AllowDestructive: true})

	raw, err := openR.CallTool("vmware_library_create_library_item_update_session", map[string]interface{}{
		"library_item_id": itemID, "confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_library_create_library_item_update_session failed: %v", err)
	}
	sessionID, _ := decodeResult(t, raw)["session_id"].(string)

	if _, err := closedR.CallTool("vmware_library_delete_library_item_update_session", map[string]interface{}{
		"session_id": sessionID, "confirm": true,
	}); err == nil {
		t.Fatalf("expected vmware_library_delete_library_item_update_session to be denied with the gate closed")
	}

	if _, err := openR.CallTool("vmware_library_cancel_library_item_update_session", map[string]interface{}{
		"session_id": sessionID,
	}); err == nil {
		t.Fatalf("expected vmware_library_cancel_library_item_update_session to be denied without confirm:true")
	}

	// Prove the session is genuinely untouched by either denied call.
	raw, err = openR.CallTool("vmware_library_get_library_item_update_session", map[string]interface{}{"session_id": sessionID})
	if err != nil {
		t.Fatalf("vmware_library_get_library_item_update_session failed: %v", err)
	}
	if decodeResult(t, raw)["state"] != "ACTIVE" {
		t.Fatalf("expected the session to still be ACTIVE after 2 denied destructive calls, got %s", raw)
	}
}

// containsString reports whether list (as decoded from a JSON array into
// []interface{}) contains s.
func containsString(list []interface{}, s string) bool {
	for _, v := range list {
		if str, ok := v.(string); ok && str == s {
			return true
		}
	}
	return false
}

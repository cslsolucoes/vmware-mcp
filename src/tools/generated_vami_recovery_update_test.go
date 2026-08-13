package tools

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// newVAMIRecoveryUpdateRegistry builds a Registry the normal way (NewRegistry,
// which wires every other domain via registerTools) and then manually layers
// registerVAMIRecoveryUpdateTools on top via withClass — same pattern as
// newApplianceSmallRegistry (generated_appliance_small_test.go). This file
// must not edit registry.go itself (see generated_vami_recovery_update.go's
// top doc comment).
func newVAMIRecoveryUpdateRegistry(ctx context.Context, c *vmware.Client, opts RegistryOptions) *Registry {
	r := NewRegistry(ctx, c, opts)
	r.withClass(modeVCenterOnly, registerVAMIRecoveryUpdateTools)
	return r
}

// vamiFixtureState records what each fixture route actually received —
// decoded JSON body and raw query string, keyed by a caller-chosen string
// (not method+path, since several tools share one route distinguished only
// by a query parameter: GET .../update/pending's source_type, POST
// .../update/pending/{version}'s action). Lets tests prove (not assume) that
// a handler built the right request: the right path parameter substitution,
// the right query string, and the right JSON body shape.
type vamiFixtureState struct {
	mu      sync.Mutex
	bodies  map[string]map[string]interface{}
	queries map[string]string
	seen    map[string]bool
}

func newVAMIFixtureState() *vamiFixtureState {
	return &vamiFixtureState{
		bodies:  make(map[string]map[string]interface{}),
		queries: make(map[string]string),
		seen:    make(map[string]bool),
	}
}

func (s *vamiFixtureState) record(key string, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seen[key] = true
	s.queries[key] = r.URL.RawQuery
	if r.Body != nil {
		data, _ := io.ReadAll(r.Body)
		if len(data) > 0 {
			var body map[string]interface{}
			if json.Unmarshal(data, &body) == nil {
				s.bodies[key] = body
			}
		}
	}
}

func (s *vamiFixtureState) body(key string) map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.bodies[key]
}

func (s *vamiFixtureState) query(key string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.queries[key]
}

func (s *vamiFixtureState) hit(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.seen[key]
}

// vamiRecoveryUpdateFixture starts a fake VAMI server covering all 31 routes
// this group registers — a session endpoint (so vmware.Client.REST()'s Login
// succeeds, reusing newApplianceFixtureClient from appliance_test.go) plus
// canned responses for every /appliance/recovery/... and /appliance/update/...
// route, all wrapped in the {"value": ...} envelope govmomi's rest.Client
// expects from /rest endpoints — same technique as appliance_test.go's
// applianceFixture, required here because there is no vcsim coverage for any
// of this (see generated_vami_recovery_update.go's top doc comment).
//
// Uses Go 1.22+ http.ServeMux method+wildcard patterns (this module targets
// go 1.25 — see go.mod). Where one real route serves several tools
// distinguished only by a query parameter (source_type, action), a single
// handler branches on that query value instead of registering duplicate
// patterns. The one genuinely ambiguous route — POST
// .../backup/schedules/{id} (create, no trailing slash) vs POST
// .../backup/schedules/{id}/?action=run (run, WITH a trailing slash — see
// generated_vami_recovery_update.go's top doc comment for why the trailing
// slash is preserved) — is handled with a single "{tail...}" remainder
// wildcard pattern that matches both shapes, branching on the "action" query
// parameter.
func vamiRecoveryUpdateFixture(t *testing.T) (*httptest.Server, *vamiFixtureState) {
	t.Helper()
	state := newVAMIFixtureState()
	mux := http.NewServeMux()

	writeValue := func(w http.ResponseWriter, v interface{}) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"value": v})
	}

	mux.HandleFunc("/rest/com/vmware/cis/session", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeValue(w, "fixture-session-id")
	})

	// --- Recovery: Backup job -----------------------------------------------

	mux.HandleFunc("GET /rest/appliance/recovery/backup/job", func(w http.ResponseWriter, r *http.Request) {
		state.record("backup_job_list", r)
		writeValue(w, []string{"job-1", "job-2"})
	})
	mux.HandleFunc("POST /rest/appliance/recovery/backup/validate", func(w http.ResponseWriter, r *http.Request) {
		state.record("backup_validate", r)
		writeValue(w, map[string]interface{}{"notifications": map[string]interface{}{}})
	})
	mux.HandleFunc("POST /rest/appliance/recovery/backup/job", func(w http.ResponseWriter, r *http.Request) {
		state.record("backup_job_create", r)
		writeValue(w, "job-99")
	})
	mux.HandleFunc("POST /rest/appliance/recovery/backup/job/{id}/cancel", func(w http.ResponseWriter, r *http.Request) {
		state.record("backup_job_cancel", r)
		writeValue(w, "cancelled")
	})
	mux.HandleFunc("GET /rest/appliance/recovery/backup/job/details", func(w http.ResponseWriter, r *http.Request) {
		state.record("backup_job_details", r)
		writeValue(w, map[string]interface{}{"parts": []string{"seat"}})
	})
	mux.HandleFunc("GET /rest/appliance/recovery/backup/job/{id}", func(w http.ResponseWriter, r *http.Request) {
		state.record("backup_job_status", r)
		writeValue(w, map[string]interface{}{"state": "INPROGRESS", "id": r.PathValue("id")})
	})
	mux.HandleFunc("GET /rest/appliance/recovery/backup/parts", func(w http.ResponseWriter, r *http.Request) {
		state.record("backup_parts", r)
		writeValue(w, []string{"seat", "common"})
	})
	mux.HandleFunc("GET /rest/appliance/recovery/backup/parts/{id}", func(w http.ResponseWriter, r *http.Request) {
		state.record("backup_part_size", r)
		writeValue(w, float64(1024))
	})

	// --- Recovery: Backup schedule -------------------------------------------

	mux.HandleFunc("GET /rest/appliance/recovery/backup/schedules", func(w http.ResponseWriter, r *http.Request) {
		state.record("backup_schedule_list", r)
		writeValue(w, []string{"default"})
	})
	mux.HandleFunc("GET /rest/appliance/recovery/backup/schedules/{id}", func(w http.ResponseWriter, r *http.Request) {
		state.record("backup_schedule_get", r)
		writeValue(w, map[string]interface{}{"enable": true, "id": r.PathValue("id")})
	})
	mux.HandleFunc("DELETE /rest/appliance/recovery/backup/schedules/{id}", func(w http.ResponseWriter, r *http.Request) {
		state.record("backup_schedule_delete", r)
		writeValue(w, nil)
	})
	mux.HandleFunc("PUT /rest/appliance/recovery/backup/schedules/{id}", func(w http.ResponseWriter, r *http.Request) {
		state.record("backup_schedule_update", r)
		writeValue(w, nil)
	})
	mux.HandleFunc("POST /rest/appliance/recovery/backup/schedules/{tail...}", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("action") == "run" {
			state.record("backup_schedule_run", r)
			writeValue(w, "run-started")
			return
		}
		state.record("backup_schedule_create", r)
		writeValue(w, "sched-created")
	})

	// --- Recovery: Restore job ------------------------------------------------

	mux.HandleFunc("GET /rest/appliance/recovery/restore/job", func(w http.ResponseWriter, r *http.Request) {
		state.record("restore_job_status", r)
		writeValue(w, map[string]interface{}{"state": "NONE"})
	})
	mux.HandleFunc("POST /rest/appliance/recovery/restore/job/cancel", func(w http.ResponseWriter, r *http.Request) {
		state.record("restore_job_cancel", r)
		writeValue(w, nil)
	})
	mux.HandleFunc("POST /rest/appliance/recovery/restore/job", func(w http.ResponseWriter, r *http.Request) {
		state.record("restore_job_create", r)
		writeValue(w, "restore-1")
	})

	// --- Update: Check for updates ---------------------------------------------

	mux.HandleFunc("GET /rest/appliance/update/pending", func(w http.ResponseWriter, r *http.Request) {
		st := r.URL.Query().Get("source_type")
		state.record("update_check_"+st, r)
		writeValue(w, map[string]interface{}{"source_type_seen": st, "version": "8.0.3.00100"})
	})
	mux.HandleFunc("GET /rest/appliance/update/pending/{version}", func(w http.ResponseWriter, r *http.Request) {
		state.record("update_pending_details", r)
		writeValue(w, map[string]interface{}{"version": r.PathValue("version"), "description": "test update"})
	})

	// --- Update: Policy ------------------------------------------------------

	mux.HandleFunc("GET /rest/appliance/update/policy", func(w http.ResponseWriter, r *http.Request) {
		state.record("update_policy_get", r)
		writeValue(w, map[string]interface{}{"auto_stage": false})
	})
	mux.HandleFunc("PUT /rest/appliance/update/policy", func(w http.ResponseWriter, r *http.Request) {
		state.record("update_policy_set", r)
		writeValue(w, nil)
	})

	// --- Update: Stage / Install (action query) -------------------------------

	mux.HandleFunc("POST /rest/appliance/update/pending/{version}", func(w http.ResponseWriter, r *http.Request) {
		action := r.URL.Query().Get("action")
		state.record("update_action_"+action, r)
		switch action {
		case "precheck", "validate":
			writeValue(w, map[string]interface{}{"issues": []string{}})
		default:
			writeValue(w, nil)
		}
	})
	mux.HandleFunc("GET /rest/appliance/update/staged", func(w http.ResponseWriter, r *http.Request) {
		state.record("update_staged_get", r)
		writeValue(w, map[string]interface{}{"version": "8.0.3.00100"})
	})
	mux.HandleFunc("DELETE /rest/appliance/update/staged", func(w http.ResponseWriter, r *http.Request) {
		state.record("update_staged_delete", r)
		writeValue(w, nil)
	})

	// --- Update: Status --------------------------------------------------------

	mux.HandleFunc("GET /rest/appliance/update", func(w http.ResponseWriter, r *http.Request) {
		state.record("update_status", r)
		writeValue(w, map[string]interface{}{"state": "UP_TO_DATE"})
	})

	return httptest.NewServer(mux), state
}

// TestVAMIRecoveryUpdateTools_Registration proves all 31 tools are
// registered and reachable via ListTools.
func TestVAMIRecoveryUpdateTools_Registration(t *testing.T) {
	srv, _ := vamiRecoveryUpdateFixture(t)
	defer srv.Close()
	c := newApplianceFixtureClient(t, srv)
	r := newVAMIRecoveryUpdateRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	want := []string{
		"vmware_appliance_recovery_backup_job_list",
		"vmware_appliance_recovery_backup_validate",
		"vmware_appliance_recovery_backup_job_create",
		"vmware_appliance_recovery_backup_job_cancel",
		"vmware_appliance_recovery_backup_job_status",
		"vmware_appliance_recovery_backup_job_details",
		"vmware_appliance_recovery_backup_parts",
		"vmware_appliance_recovery_backup_part_size",
		"vmware_appliance_recovery_backup_schedule_list",
		"vmware_appliance_recovery_backup_schedule_create",
		"vmware_appliance_recovery_backup_schedule_get",
		"vmware_appliance_recovery_backup_schedule_delete",
		"vmware_appliance_recovery_backup_schedule_run",
		"vmware_appliance_recovery_backup_schedule_update",
		"vmware_appliance_recovery_restore_job_status",
		"vmware_appliance_recovery_restore_job_cancel",
		"vmware_appliance_recovery_restore_job_create",
		"vmware_appliance_update_check_url_cdrom",
		"vmware_appliance_update_check_cdrom",
		"vmware_appliance_update_check_last",
		"vmware_appliance_update_pending_details",
		"vmware_appliance_update_policy_get",
		"vmware_appliance_update_policy_set",
		"vmware_appliance_update_stage",
		"vmware_appliance_update_staged_get",
		"vmware_appliance_update_staged_delete",
		"vmware_appliance_update_precheck",
		"vmware_appliance_update_install",
		"vmware_appliance_update_stage_and_install",
		"vmware_appliance_update_validate",
		"vmware_appliance_update_status",
	}
	if len(want) != 31 {
		t.Fatalf("test bug: want list has %d entries, expected 31", len(want))
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

// TestVAMIRecoveryUpdateTools_GateAndConfirm spot-checks the 3-layer
// destructive protection (server gate + strict confirm:true + audit) across
// a representative tier1 and tier2 tool from each of the 3 sub-domains —
// same check pattern as generated_appliance_small_test.go's
// TestApplianceSmallTools_GateAndConfirm.
func TestVAMIRecoveryUpdateTools_GateAndConfirm(t *testing.T) {
	srv, _ := vamiRecoveryUpdateFixture(t)
	defer srv.Close()
	c := newApplianceFixtureClient(t, srv)

	cases := []struct {
		tool string
		args map[string]interface{}
	}{
		{"vmware_appliance_recovery_backup_job_create", map[string]interface{}{"backup_password": "p", "location": "https://x", "location_type": "HTTPS"}},
		{"vmware_appliance_recovery_backup_schedule_delete", map[string]interface{}{"schedule_id": "default"}},
		{"vmware_appliance_recovery_restore_job_create", map[string]interface{}{"backup_password": "p", "location": "https://x", "location_type": "HTTPS"}},
		{"vmware_appliance_update_policy_set", map[string]interface{}{"policy": map[string]interface{}{"auto_stage": true}}},
		{"vmware_appliance_update_install", map[string]interface{}{"version": "8.0.3.00100"}},
	}

	closedGate := newVAMIRecoveryUpdateRegistry(context.Background(), c, RegistryOptions{AllowDestructive: false})
	for _, tc := range cases {
		args := map[string]interface{}{"confirm": true}
		for k, v := range tc.args {
			args[k] = v
		}
		if _, err := closedGate.CallTool(tc.tool, args); err == nil {
			t.Errorf("%s: expected denial with the gate closed, got success", tc.tool)
		}
	}

	openGate := newVAMIRecoveryUpdateRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	for _, tc := range cases {
		if _, err := openGate.CallTool(tc.tool, tc.args); err == nil {
			t.Errorf("%s: expected denial without confirm:true, got success", tc.tool)
		}
	}
}

// TestVAMIRecoveryUpdateTools_ArgValidation proves required arguments are
// enforced with a clean error before any request is sent — gate open in
// every case so the destructive-gate/confirm distinction doesn't interfere
// with isolating the validation check itself (same isolation technique as
// generated_appliance_small_test.go's TestApplianceSmallTools_ArgValidation).
func TestVAMIRecoveryUpdateTools_ArgValidation(t *testing.T) {
	srv, _ := vamiRecoveryUpdateFixture(t)
	defer srv.Close()
	c := newApplianceFixtureClient(t, srv)
	r := newVAMIRecoveryUpdateRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	cases := []struct {
		tool string
		args map[string]interface{}
	}{
		{"vmware_appliance_recovery_backup_validate", map[string]interface{}{}},                                                                                                    // missing backup_password/location/location_type
		{"vmware_appliance_recovery_backup_validate", map[string]interface{}{"backup_password": "p", "location": "l"}},                                                             // missing location_type
		{"vmware_appliance_recovery_backup_job_cancel", map[string]interface{}{"confirm": true}},                                                                                   // missing backup_job_id
		{"vmware_appliance_recovery_backup_job_status", map[string]interface{}{}},                                                                                                  // missing backup_job_id
		{"vmware_appliance_recovery_backup_part_size", map[string]interface{}{}},                                                                                                   // missing part_id
		{"vmware_appliance_recovery_backup_schedule_get", map[string]interface{}{}},                                                                                                // missing schedule_id
		{"vmware_appliance_recovery_backup_schedule_create", map[string]interface{}{"confirm": true}},                                                                              // missing everything
		{"vmware_appliance_recovery_backup_schedule_create", map[string]interface{}{"schedule_id": "d", "backup_password": "p", "location": "l", "enable": true, "confirm": true}}, // missing recurrence_info/retention_info
		{"vmware_appliance_recovery_restore_job_create", map[string]interface{}{"confirm": true}},                                                                                  // missing piece fields
		{"vmware_appliance_update_pending_details", map[string]interface{}{}},                                                                                                      // missing version
		{"vmware_appliance_update_policy_set", map[string]interface{}{"confirm": true}},                                                                                            // missing policy
		{"vmware_appliance_update_stage", map[string]interface{}{"confirm": true}},                                                                                                 // missing version
		{"vmware_appliance_update_precheck", map[string]interface{}{}},                                                                                                             // missing version
		{"vmware_appliance_update_install", map[string]interface{}{"confirm": true}},                                                                                               // missing version
	}

	for _, tc := range cases {
		if _, err := r.CallTool(tc.tool, tc.args); err == nil {
			t.Errorf("%s: expected a validation error for args %#v, got success", tc.tool, tc.args)
		}
	}
}

// TestVAMIRecoveryUpdateTools_ReachesFixtureHappyPath calls every one of the
// 31 tools with well-formed arguments against the httptest fixture and
// proves each one succeeds — i.e. the request actually reached the right
// method+path(+query), was decoded correctly by vamiCall, and the JSON
// result round-tripped. Deeper body/query assertions for the more complex
// handlers (nested piece/spec objects, query-discriminated shared routes)
// follow in their own dedicated checks below.
func TestVAMIRecoveryUpdateTools_ReachesFixtureHappyPath(t *testing.T) {
	srv, state := vamiRecoveryUpdateFixture(t)
	defer srv.Close()
	c := newApplianceFixtureClient(t, srv)
	r := newVAMIRecoveryUpdateRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	recurrenceInfo := map[string]interface{}{"hour": float64(0), "minute": float64(0), "days": []interface{}{"MONDAY"}}
	retentionInfo := map[string]interface{}{"max_count": float64(5)}
	userData := []interface{}{map[string]interface{}{"key": "vmdir.password", "value": "secret"}}

	cases := []struct {
		tool string
		args map[string]interface{}
	}{
		{"vmware_appliance_recovery_backup_job_list", map[string]interface{}{}},
		{"vmware_appliance_recovery_backup_validate", map[string]interface{}{"backup_password": "p", "location": "https://x", "location_type": "HTTPS"}},
		{"vmware_appliance_recovery_backup_job_create", map[string]interface{}{"backup_password": "p", "location": "https://x", "location_type": "HTTPS", "confirm": true}},
		{"vmware_appliance_recovery_backup_job_cancel", map[string]interface{}{"backup_job_id": "job-1", "confirm": true}},
		{"vmware_appliance_recovery_backup_job_status", map[string]interface{}{"backup_job_id": "job-1"}},
		{"vmware_appliance_recovery_backup_job_details", map[string]interface{}{}},
		{"vmware_appliance_recovery_backup_parts", map[string]interface{}{}},
		{"vmware_appliance_recovery_backup_part_size", map[string]interface{}{"part_id": "seat"}},
		{"vmware_appliance_recovery_backup_schedule_list", map[string]interface{}{}},
		{"vmware_appliance_recovery_backup_schedule_create", map[string]interface{}{
			"schedule_id": "default", "backup_password": "p", "location": "https://x", "enable": true,
			"recurrence_info": recurrenceInfo, "retention_info": retentionInfo, "confirm": true,
		}},
		{"vmware_appliance_recovery_backup_schedule_get", map[string]interface{}{"schedule_id": "default"}},
		{"vmware_appliance_recovery_backup_schedule_run", map[string]interface{}{"schedule_id": "default", "comment": "test run", "confirm": true}},
		{"vmware_appliance_recovery_backup_schedule_update", map[string]interface{}{
			"schedule_id": "default", "backup_password": "p", "location": "https://x", "enable": true,
			"recurrence_info": recurrenceInfo, "retention_info": retentionInfo, "confirm": true,
		}},
		{"vmware_appliance_recovery_backup_schedule_delete", map[string]interface{}{"schedule_id": "default", "confirm": true}},
		{"vmware_appliance_recovery_restore_job_status", map[string]interface{}{}},
		{"vmware_appliance_recovery_restore_job_cancel", map[string]interface{}{"confirm": true}},
		{"vmware_appliance_recovery_restore_job_create", map[string]interface{}{"backup_password": "p", "location": "https://x", "location_type": "HTTPS", "confirm": true}},
		{"vmware_appliance_update_check_url_cdrom", map[string]interface{}{}},
		{"vmware_appliance_update_check_cdrom", map[string]interface{}{}},
		{"vmware_appliance_update_check_last", map[string]interface{}{}},
		{"vmware_appliance_update_pending_details", map[string]interface{}{"version": "8.0.3.00100"}},
		{"vmware_appliance_update_policy_get", map[string]interface{}{}},
		{"vmware_appliance_update_policy_set", map[string]interface{}{"policy": map[string]interface{}{"auto_stage": true}, "confirm": true}},
		{"vmware_appliance_update_stage", map[string]interface{}{"version": "8.0.3.00100", "confirm": true}},
		{"vmware_appliance_update_staged_get", map[string]interface{}{}},
		{"vmware_appliance_update_staged_delete", map[string]interface{}{"confirm": true}},
		{"vmware_appliance_update_precheck", map[string]interface{}{"version": "8.0.3.00100"}},
		{"vmware_appliance_update_install", map[string]interface{}{"version": "8.0.3.00100", "user_data": userData, "confirm": true}},
		{"vmware_appliance_update_stage_and_install", map[string]interface{}{"version": "8.0.3.00100", "user_data": userData, "confirm": true}},
		{"vmware_appliance_update_validate", map[string]interface{}{"version": "8.0.3.00100", "user_data": userData}},
		{"vmware_appliance_update_status", map[string]interface{}{}},
	}
	if len(cases) != 31 {
		t.Fatalf("test bug: cases has %d entries, expected 31", len(cases))
	}

	for _, tc := range cases {
		t.Run(tc.tool, func(t *testing.T) {
			raw, err := r.CallTool(tc.tool, tc.args)
			if err != nil {
				t.Fatalf("%s: expected success against the fixture, got: %v", tc.tool, err)
			}
			if raw == "" {
				t.Fatalf("%s: expected a non-empty result", tc.tool)
			}
		})
	}

	// --- Deeper checks: path parameter substitution round-trips -------------

	if m := decodeResult(t, mustCall(t, r, "vmware_appliance_recovery_backup_job_status", map[string]interface{}{"backup_job_id": "job-1"})); m["id"] != "job-1" {
		t.Errorf("backup_job_status: expected id echoed back as job-1, got %v", m["id"])
	}
	if m := decodeResult(t, mustCall(t, r, "vmware_appliance_recovery_backup_schedule_get", map[string]interface{}{"schedule_id": "sched-xyz"})); m["id"] != "sched-xyz" {
		t.Errorf("backup_schedule_get: expected id echoed back as sched-xyz, got %v", m["id"])
	}
	if m := decodeResult(t, mustCall(t, r, "vmware_appliance_update_pending_details", map[string]interface{}{"version": "8.0.3.00200"})); m["version"] != "8.0.3.00200" {
		t.Errorf("update_pending_details: expected version echoed back as 8.0.3.00200, got %v", m["version"])
	}

	// --- Deeper checks: query-discriminated shared routes --------------------

	for _, key := range []string{"update_check_LOCAL_AND_ONLINE", "update_check_LOCAL", "update_check_LAST_CHECK"} {
		if !state.hit(key) {
			t.Errorf("expected the fixture to have recorded a hit for %s (source_type routing)", key)
		}
	}
	for _, key := range []string{"update_action_stage", "update_action_precheck", "update_action_install", "update_action_stage-and-install", "update_action_validate"} {
		if !state.hit(key) {
			t.Errorf("expected the fixture to have recorded a hit for %s (action routing)", key)
		}
	}
	if !state.hit("backup_schedule_create") {
		t.Error("expected the fixture to have recorded a hit for backup_schedule_create (POST, no trailing slash, no action)")
	}
	if !state.hit("backup_schedule_run") {
		t.Error("expected the fixture to have recorded a hit for backup_schedule_run (POST, trailing slash, action=run)")
	}

	// --- Deeper checks: request body shape -----------------------------------

	if body := state.body("backup_validate"); body != nil {
		piece, ok := body["piece"].(map[string]interface{})
		if !ok {
			t.Fatalf("backup_validate: expected a \"piece\" object in the request body, got %#v", body)
		}
		if piece["location_type"] != "HTTPS" || piece["location"] != "https://x" {
			t.Errorf("backup_validate: unexpected piece contents: %#v", piece)
		}
	} else {
		t.Error("backup_validate: fixture recorded no request body")
	}

	if body := state.body("backup_schedule_create"); body != nil {
		spec, ok := body["spec"].(map[string]interface{})
		if !ok {
			t.Fatalf("backup_schedule_create: expected a \"spec\" object in the request body, got %#v", body)
		}
		if spec["enable"] != true {
			t.Errorf("backup_schedule_create: expected spec.enable=true, got %v", spec["enable"])
		}
		ri, ok := spec["retention_info"].(map[string]interface{})
		if !ok || ri["max_count"] != float64(5) {
			t.Errorf("backup_schedule_create: expected spec.retention_info.max_count=5 forwarded verbatim, got %#v", spec["retention_info"])
		}
		rc, ok := spec["recurrence_info"].(map[string]interface{})
		if !ok || rc["hour"] != float64(0) {
			t.Errorf("backup_schedule_create: expected spec.recurrence_info.hour=0 forwarded verbatim, got %#v", spec["recurrence_info"])
		}
	} else {
		t.Error("backup_schedule_create: fixture recorded no request body")
	}

	if body := state.body("backup_schedule_run"); body != nil {
		if body["comment"] != "test run" {
			t.Errorf("backup_schedule_run: expected comment=\"test run\", got %v", body["comment"])
		}
	} else {
		t.Error("backup_schedule_run: fixture recorded no request body")
	}
	if q := state.query("backup_schedule_run"); q != "action=run" {
		t.Errorf("backup_schedule_run: expected query \"action=run\", got %q", q)
	}

	if body := state.body("update_policy_set"); body != nil {
		policy, ok := body["policy"].(map[string]interface{})
		if !ok || policy["auto_stage"] != true {
			t.Errorf("update_policy_set: expected policy.auto_stage=true forwarded verbatim, got %#v", body["policy"])
		}
	} else {
		t.Error("update_policy_set: fixture recorded no request body")
	}

	if body := state.body("update_action_install"); body != nil {
		ud, ok := body["user_data"].([]interface{})
		if !ok || len(ud) != 1 {
			t.Fatalf("update_action_install: expected a 1-element user_data array, got %#v", body["user_data"])
		}
		entry, ok := ud[0].(map[string]interface{})
		if !ok || entry["key"] != "vmdir.password" || entry["value"] != "secret" {
			t.Errorf("update_action_install: expected user_data[0]={key:vmdir.password,value:secret} forwarded verbatim, got %#v", ud[0])
		}
	} else {
		t.Error("update_action_install: fixture recorded no request body")
	}

	// update_precheck/vmware_appliance_update_stage send NO body — proves
	// this tool does not manufacture one where Postman's own sample has none
	// (see generated_vami_recovery_update.go's handleApplianceUpdatePrecheck/
	// handleApplianceUpdateStage).
	if body := state.body("update_action_precheck"); body != nil {
		t.Errorf("update_action_precheck: expected no request body (Postman's own sample has none), got %#v", body)
	}
	if body := state.body("update_action_stage"); body != nil {
		t.Errorf("update_action_stage: expected no request body (Postman's own sample has none), got %#v", body)
	}
}

// mustCall is a small helper for the "deeper checks" section above: call a
// tool and fail the test immediately on error, returning the raw JSON result.
func mustCall(t *testing.T, r *Registry, tool string, args map[string]interface{}) string {
	t.Helper()
	raw, err := r.CallTool(tool, args)
	if err != nil {
		t.Fatalf("%s: unexpected error: %v", tool, err)
	}
	return raw
}

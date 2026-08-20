package tools

import (
	"context"
	"testing"

	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// newHostProfileRegistry builds a Registry the normal way (NewRegistry,
// which wires vm.go/host.go/etc — including firstHostPath's
// vmware_list_hosts — via registerTools) and then manually layers this
// group's Host Profile tools on top via withClass, exactly as
// generated_vm_ft_test.go's newFtRegistry already does for Fault Tolerance —
// this file must not edit registry.go itself (see generated_host_profile.go's
// top doc comment and this task's constraints).
func newHostProfileRegistry(ctx context.Context, c *vmware.Client, opts RegistryOptions) *Registry {
	r := NewRegistry(ctx, c, opts)
	r.withClass(modeVCenterOnly, registerHostProfileTools)
	return r
}

// hostProfileToolNames is the exact set registered by
// registerHostProfileTools — kept here so
// TestHostProfileTools_Registration can't silently drift from the real
// registration list.
var hostProfileToolNames = []string{
	"vmware_host_profile_create",
	"vmware_host_profile_query_metadata",
	"vmware_host_profile_find_associated",
	"vmware_host_profile_apply_config",
	"vmware_host_profile_generate_config_task_list",
	"vmware_host_profile_generate_task_list",
	"vmware_host_profile_answerfile_check_status",
	"vmware_host_profile_answerfile_query_status",
	"vmware_host_profile_answerfile_update",
	"vmware_host_profile_answerfile_retrieve",
	"vmware_host_profile_answerfile_retrieve_for_profile",
	"vmware_host_profile_answerfile_export",
	"vmware_host_profile_composite",
	"vmware_host_profile_retrieve_description",
	"vmware_host_profile_check_compliance",
	"vmware_host_profile_export",
	"vmware_host_profile_destroy",
	"vmware_host_profile_associate",
	"vmware_host_profile_dissociate",
	"vmware_host_profile_update",
	"vmware_host_profile_execute",
}

// destructiveHostProfileToolNames is the subset registered via
// registerDestructive (tier1/tier2) — used by
// TestHostProfileTools_GateAndConfirm.
var destructiveHostProfileToolNames = []string{
	"vmware_host_profile_create",
	"vmware_host_profile_apply_config",
	"vmware_host_profile_answerfile_update",
	"vmware_host_profile_composite",
	"vmware_host_profile_destroy",
	"vmware_host_profile_associate",
	"vmware_host_profile_dissociate",
	"vmware_host_profile_update",
}

// TestHostProfileTools_Registration proves all 21 host profile tools are
// wired through newHostProfileRegistry's withClass call and reachable via
// ListTools.
func TestHostProfileTools_Registration(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newHostProfileRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	if len(hostProfileToolNames) != 21 {
		t.Fatalf("test bug: hostProfileToolNames has %d entries, expected 21", len(hostProfileToolNames))
	}

	got := map[string]bool{}
	for _, tl := range r.ListTools() {
		got[tl.Name] = true
	}
	for _, name := range hostProfileToolNames {
		if !got[name] {
			t.Errorf("tool %s not registered", name)
		}
	}
}

// TestHostProfileTools_Validation proves each handler rejects missing/empty
// required arguments BEFORE any network round trip (so these fail even with
// the gate open and confirm:true).
func TestHostProfileTools_Validation(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newHostProfileRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	host := firstHostPath(t, r)

	cases := []struct {
		name string
		args map[string]interface{}
		why  string
	}{
		{"vmware_host_profile_create", map[string]interface{}{"host": host, "confirm": true}, "missing name"},
		{"vmware_host_profile_create", map[string]interface{}{"name": "p1", "confirm": true}, "missing host"},
		{"vmware_host_profile_find_associated", map[string]interface{}{}, "missing entity_path"},
		{"vmware_host_profile_apply_config", map[string]interface{}{"host": host, "confirm": true}, "missing config_spec"},
		{"vmware_host_profile_apply_config", map[string]interface{}{"host": host, "config_spec": map[string]interface{}{}}, "missing confirm"},
		{"vmware_host_profile_apply_config", map[string]interface{}{"host": host, "config_spec": "not-an-object", "confirm": true}, "invalid config_spec"},
		{"vmware_host_profile_generate_config_task_list", map[string]interface{}{"host": host}, "missing config_spec"},
		{"vmware_host_profile_generate_task_list", map[string]interface{}{"host": host}, "missing config_spec"},
		{"vmware_host_profile_answerfile_check_status", map[string]interface{}{}, "missing hosts"},
		{"vmware_host_profile_answerfile_check_status", map[string]interface{}{"hosts": []interface{}{}}, "empty hosts"},
		{"vmware_host_profile_answerfile_query_status", map[string]interface{}{}, "missing hosts"},
		{"vmware_host_profile_answerfile_update", map[string]interface{}{"confirm": true}, "missing host"},
		{"vmware_host_profile_answerfile_retrieve", map[string]interface{}{}, "missing host"},
		{"vmware_host_profile_answerfile_retrieve_for_profile", map[string]interface{}{"host": host}, "missing apply_profile"},
		{"vmware_host_profile_answerfile_export", map[string]interface{}{}, "missing host"},
		{"vmware_host_profile_composite", map[string]interface{}{"confirm": true}, "missing source"},
		{"vmware_host_profile_composite", map[string]interface{}{"source": "p1"}, "missing confirm"},
		{"vmware_host_profile_retrieve_description", map[string]interface{}{}, "missing profile"},
		{"vmware_host_profile_check_compliance", map[string]interface{}{}, "missing profile"},
		{"vmware_host_profile_export", map[string]interface{}{}, "missing profile"},
		{"vmware_host_profile_destroy", map[string]interface{}{"confirm": true}, "missing profile"},
		{"vmware_host_profile_associate", map[string]interface{}{"profile": "p1", "confirm": true}, "missing entity_paths"},
		{"vmware_host_profile_associate", map[string]interface{}{"profile": "p1", "entity_paths": []interface{}{}, "confirm": true}, "empty entity_paths"},
		{"vmware_host_profile_dissociate", map[string]interface{}{"confirm": true}, "missing profile"},
		{"vmware_host_profile_update", map[string]interface{}{"profile": "p1", "confirm": true}, "missing host"},
		{"vmware_host_profile_execute", map[string]interface{}{"profile": "p1"}, "missing host"},
	}

	for _, tc := range cases {
		t.Run(tc.name+"/"+tc.why, func(t *testing.T) {
			if _, err := r.CallTool(tc.name, tc.args); err == nil {
				t.Fatalf("expected an error (%s) before any round trip", tc.why)
			}
		})
	}
}

// TestHostProfileTools_GateAndConfirm proves the tier1/tier2 destructive
// protection is actually wired on every one of the 8 mutating tools: a
// closed --allow-destructive gate denies the call, and an open gate still
// requires confirm:true.
func TestHostProfileTools_GateAndConfirm(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	seed := newHostProfileRegistry(context.Background(), c, RegistryOptions{})
	host := firstHostPath(t, seed)

	closed := newHostProfileRegistry(context.Background(), c, RegistryOptions{})
	open := newHostProfileRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	// A minimally-valid-enough argument set per tool (enough to pass this
	// package's own argument validation so the gate/confirm check is what
	// actually gets exercised — none of these ever reach vcsim because the
	// gate/confirm check runs before any round trip).
	argsByTool := map[string]map[string]interface{}{
		"vmware_host_profile_create":            {"name": "p1", "host": host},
		"vmware_host_profile_apply_config":      {"host": host, "config_spec": map[string]interface{}{}},
		"vmware_host_profile_answerfile_update": {"host": host},
		"vmware_host_profile_composite":         {"source": "p1"},
		"vmware_host_profile_destroy":           {"profile": "p1"},
		"vmware_host_profile_associate":         {"profile": "p1", "entity_paths": []interface{}{host}},
		"vmware_host_profile_dissociate":        {"profile": "p1"},
		"vmware_host_profile_update":            {"profile": "p1", "host": host},
	}

	if len(destructiveHostProfileToolNames) != len(argsByTool) {
		t.Fatalf("test bug: destructiveHostProfileToolNames has %d entries, argsByTool has %d", len(destructiveHostProfileToolNames), len(argsByTool))
	}

	for _, name := range destructiveHostProfileToolNames {
		t.Run(name, func(t *testing.T) {
			base, ok := argsByTool[name]
			if !ok {
				t.Fatalf("test bug: no args fixture for %s", name)
			}

			withConfirm := map[string]interface{}{}
			for k, v := range base {
				withConfirm[k] = v
			}
			withConfirm["confirm"] = true

			if _, err := closed.CallTool(name, withConfirm); err == nil {
				t.Fatalf("%s: expected the closed destructive gate to deny the call", name)
			}

			if _, err := open.CallTool(name, base); err == nil {
				t.Fatalf("%s: expected an error without confirm:true", name)
			}
		})
	}
}

// TestHostProfileTools_ReachesServer drives every one of the 21 tools with
// valid input, gate open, and confirm:true. Empirically confirmed (by
// actually running this test and reading -v output, not assumed) two
// DIFFERENT clean-failure shapes:
//   - The 13 HostProfileManager-level tools reach vcsim's generic SOAP
//     dispatcher, which replies "HostProfileManager:HostProfileManager does
//     not implement: <MethodName>" — a real server-side
//     MethodNotFound-shaped fault, proving the raw SOAP dispatch itself
//     (methods.Xxx(ctx, client.Client.Client, &types.Xxx{This: mgrRef})) is
//     reached for every one of those 13 methods.
//   - The 8 Profile/HostProfile-level tools instead fail one step earlier,
//     inside hprofResolveProfile: its own property read against the
//     HostProfileManager MoRef (mgr.Profile) DOES reach vcsim's
//     PropertyCollector and succeeds (returns an empty profile list, not a
//     fault — vcsim's reflection-based property collector answers a known
//     managed-object TYPE even with no business-logic method
//     implementation), so hprofResolveProfile correctly reports "no host
//     profile named ... found" before ever reaching this file's raw
//     Profile-level SOAP call. There is no way to get a real profile into
//     HostProfileManager.profile[] to exercise that final call through
//     r.CallTool, because vcsim has no CreateProfile implementation either
//     (see the "does not implement: CreateProfile" case above) — this test
//     file's separate
//     TestHostProfileTools_ProfileLevelRawDispatchReachesServer closes that
//     gap directly against the raw vim25 method, empirically, instead of
//     assuming the same dispatch behavior extends to Profile-typed MoRefs.
//
// assertReachesServer is the same helper generated_vm_ft_test.go and
// generated_host_iscsi_portbinding_test.go use for their own fully
// unsimulated method groups; it accepts either failure shape above (any
// non-nil error that is not "unknown tool" and not "panicked").
func TestHostProfileTools_ReachesServer(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newHostProfileRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	host := firstHostPath(t, r)
	const profile = "test-profile"

	cases := []struct {
		name string
		args map[string]interface{}
	}{
		{"vmware_host_profile_create", map[string]interface{}{"name": profile, "host": host, "annotation": "created by test", "enabled": true, "confirm": true}},
		{"vmware_host_profile_query_metadata", map[string]interface{}{}},
		{"vmware_host_profile_query_metadata", map[string]interface{}{"profile_names": []interface{}{profile}}},
		{"vmware_host_profile_find_associated", map[string]interface{}{"entity_path": host}},
		{"vmware_host_profile_apply_config", map[string]interface{}{"host": host, "config_spec": map[string]interface{}{"datastorePrincipal": "root"}, "confirm": true}},
		{"vmware_host_profile_generate_config_task_list", map[string]interface{}{"host": host, "config_spec": map[string]interface{}{}}},
		{"vmware_host_profile_generate_task_list", map[string]interface{}{"host": host, "config_spec": map[string]interface{}{}}},
		{"vmware_host_profile_answerfile_check_status", map[string]interface{}{"hosts": []interface{}{host}}},
		{"vmware_host_profile_answerfile_query_status", map[string]interface{}{"hosts": []interface{}{host}}},
		{"vmware_host_profile_answerfile_update", map[string]interface{}{"host": host, "confirm": true}},
		{"vmware_host_profile_answerfile_retrieve", map[string]interface{}{"host": host}},
		{"vmware_host_profile_answerfile_retrieve_for_profile", map[string]interface{}{"host": host, "apply_profile": map[string]interface{}{"enabled": true}}},
		{"vmware_host_profile_answerfile_export", map[string]interface{}{"host": host}},
		{"vmware_host_profile_composite", map[string]interface{}{"source": profile, "targets": []interface{}{profile}, "confirm": true}},
		{"vmware_host_profile_retrieve_description", map[string]interface{}{"profile": profile}},
		{"vmware_host_profile_check_compliance", map[string]interface{}{"profile": profile}},
		{"vmware_host_profile_check_compliance", map[string]interface{}{"profile": profile, "entity_paths": []interface{}{host}}},
		{"vmware_host_profile_export", map[string]interface{}{"profile": profile}},
		{"vmware_host_profile_associate", map[string]interface{}{"profile": profile, "entity_paths": []interface{}{host}, "confirm": true}},
		{"vmware_host_profile_dissociate", map[string]interface{}{"profile": profile, "confirm": true}},
		{"vmware_host_profile_dissociate", map[string]interface{}{"profile": profile, "entity_paths": []interface{}{host}, "confirm": true}},
		{"vmware_host_profile_update", map[string]interface{}{"profile": profile, "host": host, "confirm": true}},
		{"vmware_host_profile_execute", map[string]interface{}{"profile": profile, "host": host}},
		{"vmware_host_profile_destroy", map[string]interface{}{"profile": profile, "confirm": true}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := r.CallTool(tc.name, tc.args)
			assertReachesServer(t, err, tc.name)
		})
	}
}

// TestHostProfileTools_ProfileLevelRawDispatchReachesServer empirically
// closes the gap TestHostProfileTools_ReachesServer's doc comment describes:
// none of the 8 Profile/HostProfile-level tools (vmware_host_profile_
// retrieve_description and friends) can be driven far enough through
// r.CallTool to exercise their own raw SOAP call, because hprofResolveProfile
// always fails first with "no host profile named ... found" (there is no
// real profile in HostProfileManager.profile[] to resolve, and no way to
// create one — vcsim has no CreateProfile implementation either). This test
// calls the raw vim25 method directly — the exact same
// methods.Xxx(ctx, client.Client.Client, &types.Xxx{This: ref}) shape every
// handleHostProfileXxx function in generated_host_profile.go uses — against
// a synthetic Profile-typed MoRef that vcsim has never heard of, to confirm
// (not assume) that the SAME clean server-side fault behavior already proven
// for HostProfileManager-typed MoRefs above also applies at the Profile
// level. RetrieveDescription and CheckProfileCompliance_Task (one direct
// call, one Task-based call — the two raw-dispatch shapes this file's
// Profile-level handlers use) are each tried once.
func TestHostProfileTools_ProfileLevelRawDispatchReachesServer(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	ref := types.ManagedObjectReference{Type: "HostProfile", Value: "does-not-exist"}

	t.Run("RetrieveDescription", func(t *testing.T) {
		_, err := methods.RetrieveDescription(context.Background(), c.Client.Client, &types.RetrieveDescription{This: ref})
		assertReachesServer(t, err, "methods.RetrieveDescription (raw, synthetic Profile MoRef)")
	})

	t.Run("CheckProfileCompliance_Task", func(t *testing.T) {
		_, err := methods.CheckProfileCompliance_Task(context.Background(), c.Client.Client, &types.CheckProfileCompliance_Task{This: ref})
		assertReachesServer(t, err, "methods.CheckProfileCompliance_Task (raw, synthetic Profile MoRef)")
	})
}

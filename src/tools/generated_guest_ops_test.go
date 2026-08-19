package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/vmware/govmomi/simulator"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// newGuestOpsRegistry builds a Registry the normal way (NewRegistry, which
// wires every other file via registerTools) and then manually layers this
// group's tools on top via withClass — this file must not edit registry.go
// itself (generated_guest_ops.go was written in parallel with other codegen
// groups; the coordinator integrates the real registry.go wiring later).
func newGuestOpsRegistry(ctx context.Context, c *vmware.Client, opts RegistryOptions) *Registry {
	r := NewRegistry(ctx, c, opts)
	r.withClass(modeVSphereGeneral, registerGuestOpsTools)
	return r
}

// guestOpsToolNames is the exact set registered by registerGuestOpsTools —
// kept here so the registration subtest can't silently drift from the real
// registration list.
var guestOpsToolNames = []string{
	"vmware_guest_list_files",
	"vmware_guest_create_temp_file",
	"vmware_guest_create_temp_directory",
	"vmware_guest_file_transfer_from",
	"vmware_guest_list_processes",
	"vmware_guest_read_environment_variable",
	"vmware_guest_make_directory",
	"vmware_guest_move_directory",
	"vmware_guest_move_file",
	"vmware_guest_change_file_attributes",
	"vmware_guest_file_transfer_to",
	"vmware_guest_start_program",
	"vmware_guest_terminate_process",
	"vmware_guest_delete_file",
	"vmware_guest_delete_directory",
}

// guestOpsVMPaths lists every VM in the model — the reaches_server subtest
// needs two distinct VMs (see its comment: one throwaway VM to absorb the
// single call that poisons vcsim's per-VM object lock). simulator.ESX() ships
// Machine:2, so len>=2 holds.
func guestOpsVMPaths(t *testing.T, r *Registry) []string {
	t.Helper()
	raw, err := r.CallTool("vmware_list_vms", map[string]interface{}{})
	if err != nil {
		t.Fatalf("vmware_list_vms failed: %v", err)
	}
	list, _ := decodeResult(t, raw)["vms"].([]interface{})
	if len(list) == 0 {
		t.Fatal("simulator model has no VMs")
	}
	out := make([]string, 0, len(list))
	for _, v := range list {
		out = append(out, v.(string))
	}
	return out
}

// guestDestructiveToolArgs builds the minimal valid (non-confirm) argument
// set for each of the 9 destructive guest-ops tools, keyed by tool name —
// shared by the gate_and_confirm and reaches_server subtests.
func guestDestructiveToolArgs(vm string) map[string]map[string]interface{} {
	base := map[string]interface{}{"vm": vm, "guest_username": "testuser", "guest_password": "testpass"}
	merge := func(extra map[string]interface{}) map[string]interface{} {
		m := map[string]interface{}{}
		for k, v := range base {
			m[k] = v
		}
		for k, v := range extra {
			m[k] = v
		}
		return m
	}
	return map[string]map[string]interface{}{
		"vmware_guest_make_directory":         merge(map[string]interface{}{"directory_path": "/tmp/mcpvmware-test"}),
		"vmware_guest_move_directory":         merge(map[string]interface{}{"src_directory_path": "/tmp/a", "dst_directory_path": "/tmp/b"}),
		"vmware_guest_move_file":              merge(map[string]interface{}{"src_file_path": "/tmp/a.txt", "dst_file_path": "/tmp/b.txt"}),
		"vmware_guest_change_file_attributes": merge(map[string]interface{}{"guest_file_path": "/tmp/a.txt", "permissions": 420}),
		"vmware_guest_file_transfer_to":       merge(map[string]interface{}{"guest_file_path": "/tmp/upload.txt", "file_size": 1024}),
		"vmware_guest_start_program":          merge(map[string]interface{}{"program_path": "/bin/true"}),
		"vmware_guest_terminate_process":      merge(map[string]interface{}{"pid": 1}),
		"vmware_guest_delete_file":            merge(map[string]interface{}{"file_path": "/tmp/a.txt"}),
		"vmware_guest_delete_directory":       merge(map[string]interface{}{"directory_path": "/tmp/a"}),
	}
}

// assertGuestOpsWiringReachesServer proves a guest-ops tool call actually
// reached this server's real handler and vcsim's real GuestFileManager/
// GuestProcessManager dispatch (schema parse, destructive gate/confirm,
// resolveVM, guestFileManager/guestProcessManager sub-manager resolution,
// raw SOAP call all wired correctly) rather than failing on "unknown tool"
// (registration broken) or a recovered panic in THIS server's own handler
// (registry.go's CallTool recover(), a bug in generated_guest_ops.go).
//
// It does NOT require err != nil: as documented in generated_guest_ops.go's
// top doc comment (confirmed by reading
// referencia/govmomi/simulator/guest_operations_manager.go and
// container_virtual_machine.go), vcsim's default (non-container-backed) VM
// makes most GuestFileManager mutations/queries genuinely SUCCEED as silent
// no-ops — a nil error there is even stronger proof of correct wiring (the
// full round trip completed) than a clean fault. Only "unknown tool" and
// "panicked" (our own handler panicking) are treated as this project's bugs;
// every other outcome (nil or non-nil) is logged, not asserted on, because
// vcsim's per-method behavior without a container backing varies (see that
// doc comment) and only a real vCenter/ESXi with VMware Tools running in an
// actual guest can validate the true end-to-end behavior.
func assertGuestOpsWiringReachesServer(t *testing.T, err error, tool string) {
	t.Helper()
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "unknown tool") {
			t.Fatalf("%s: tool is not registered: %v", tool, err)
		}
		if strings.Contains(msg, "panicked") {
			t.Fatalf("%s: our handler panicked instead of returning a clean error: %v", tool, err)
		}
	}
	t.Logf("%s: reached the real handler/vcsim — err=%v", tool, err)
}

// TestGuestOps exercises the 15 Guest Operations tools against a SINGLE shared
// vcsim server (one newSimClient for the whole file). The four concerns —
// registration, argument validation, the destructive gate/confirm layer, and
// reaching the real server — run as sequential subtests over that one client,
// deliberately NOT one vcsim per concern.
//
// Two independent reasons this file shares one vcsim + isolates one call:
//
//  1. Sharing one vcsim: each newSimClient spins up (and Closes) an httptest
//     TLS server; on Windows, cycling several per file adds latency/socket
//     churn for no benefit here. One server, two registries (open/closed) over
//     the same *vmware.Client is enough — NewRegistry never touches the wire.
//
//  2. Isolating vmware_guest_start_program onto its own throwaway VM: this is
//     the load-bearing fix for a real deadlock (not a hypothetical). vcsim's
//     StartProgramInGuest, against a default (non-container-backed) VM,
//     computes a fault from the nil svm but then falls through WITHOUT
//     returning and dereferences vm.svm.c.id — a nil-pointer panic
//     (referencia/govmomi/simulator/guest_operations_manager.go's
//     StartProgramInGuest). vcsim dispatches every method under
//     Registry.WithLock, which has NO defer on its unlock
//     (referencia/govmomi/simulator/registry.go: `f(); unlock()`), so that
//     server-side panic — recovered harmlessly by net/http at the connection
//     level — leaves the VM's ObjectLock mutex PERMANENTLY held. Any later
//     guest-op on that same VM then blocks forever in ObjectLock.wait ->
//     l.lock.Lock(). Driving StartProgramInGuest on a dedicated VM that
//     nothing else touches keeps the shared VM's lock clean; the call itself
//     still returns a server-originated error (its connection is reset by the
//     recovered panic), which is exactly the "wiring reached vcsim" signal we
//     want. Confirmed empirically: mixing StartProgramInGuest and any other
//     guest-op on one VM in random map order hung the package at Go's 10-minute
//     test timeout, with the goroutine dump pinned on
//     TerminateProcessInGuest -> ObjectLock.wait.
func TestGuestOps(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	ctx := context.Background()
	open := newGuestOpsRegistry(ctx, c, RegistryOptions{AllowDestructive: true})
	closed := newGuestOpsRegistry(ctx, c, RegistryOptions{})

	vmPaths := guestOpsVMPaths(t, open)
	vm := vmPaths[0]
	// Dedicated VM to absorb the single lock-poisoning call — distinct from vm
	// whenever the model has >=2 VMs (ESX() has 2). See this function's doc
	// comment for why.
	startProgVM := vmPaths[len(vmPaths)-1]

	// --- registration ----------------------------------------------------
	t.Run("registration", func(t *testing.T) {
		if len(guestOpsToolNames) != 15 {
			t.Fatalf("test bug: guestOpsToolNames has %d entries, expected 15", len(guestOpsToolNames))
		}
		got := map[string]bool{}
		for _, tl := range open.ListTools() {
			got[tl.Name] = true
		}
		for _, name := range guestOpsToolNames {
			if !got[name] {
				t.Errorf("tool %s not registered", name)
			}
		}
	})

	// --- validation ------------------------------------------------------
	// Each handler must reject missing/empty required arguments with a clean
	// error before any round trip. Destructive cases carry confirm:true and an
	// open gate so the failure being tested is argument validation, not the
	// gate.
	t.Run("validation", func(t *testing.T) {
		creds := map[string]interface{}{"guest_username": "testuser", "guest_password": "testpass"}
		withVM := func(extra map[string]interface{}) map[string]interface{} {
			m := map[string]interface{}{"vm": vm}
			for k, v := range creds {
				m[k] = v
			}
			for k, v := range extra {
				m[k] = v
			}
			return m
		}

		cases := []struct {
			name string
			args map[string]interface{}
			why  string
		}{
			{"vmware_guest_list_files", map[string]interface{}{"guest_username": "u", "guest_password": "p", "file_path": "/tmp"}, "missing vm"},
			{"vmware_guest_list_files", withVM(map[string]interface{}{}), "missing file_path"},
			{"vmware_guest_list_files", map[string]interface{}{"vm": vm, "guest_password": "p", "file_path": "/tmp"}, "missing guest_username"},
			{"vmware_guest_list_files", map[string]interface{}{"vm": vm, "guest_username": "u", "file_path": "/tmp"}, "missing guest_password"},

			{"vmware_guest_create_temp_file", map[string]interface{}{"vm": vm, "guest_password": "p"}, "missing guest_username"},
			{"vmware_guest_create_temp_directory", map[string]interface{}{"vm": vm, "guest_username": "u"}, "missing guest_password"},
			{"vmware_guest_file_transfer_from", withVM(map[string]interface{}{}), "missing guest_file_path"},
			{"vmware_guest_list_processes", map[string]interface{}{"vm": vm}, "missing guest_username and guest_password"},
			{"vmware_guest_read_environment_variable", map[string]interface{}{"guest_username": "u", "guest_password": "p"}, "missing vm"},

			{"vmware_guest_make_directory", withVM(map[string]interface{}{"confirm": true}), "missing directory_path"},
			{"vmware_guest_delete_file", withVM(map[string]interface{}{"confirm": true}), "missing file_path"},
			{"vmware_guest_delete_directory", withVM(map[string]interface{}{"confirm": true}), "missing directory_path"},
			{"vmware_guest_move_directory", withVM(map[string]interface{}{"dst_directory_path": "/tmp/b", "confirm": true}), "missing src_directory_path"},
			{"vmware_guest_move_directory", withVM(map[string]interface{}{"src_directory_path": "/tmp/a", "confirm": true}), "missing dst_directory_path"},
			{"vmware_guest_move_file", withVM(map[string]interface{}{"dst_file_path": "/tmp/b.txt", "confirm": true}), "missing src_file_path"},
			{"vmware_guest_move_file", withVM(map[string]interface{}{"src_file_path": "/tmp/a.txt", "confirm": true}), "missing dst_file_path"},
			{"vmware_guest_change_file_attributes", withVM(map[string]interface{}{"confirm": true}), "missing guest_file_path"},
			{"vmware_guest_file_transfer_to", withVM(map[string]interface{}{"file_size": 10, "confirm": true}), "missing guest_file_path"},
			{"vmware_guest_file_transfer_to", withVM(map[string]interface{}{"guest_file_path": "/tmp/x", "confirm": true}), "missing file_size"},
			{"vmware_guest_start_program", withVM(map[string]interface{}{"confirm": true}), "missing program_path"},
			{"vmware_guest_terminate_process", withVM(map[string]interface{}{"confirm": true}), "missing pid"},
		}

		for _, tc := range cases {
			t.Run(tc.name+"/"+tc.why, func(t *testing.T) {
				if _, err := open.CallTool(tc.name, tc.args); err == nil {
					t.Fatalf("expected an error (%s)", tc.why)
				}
			})
		}
	})

	// --- gate_and_confirm ------------------------------------------------
	// The tier1/tier2 destructive protection must be wired on every one of the
	// 9 mutating tools: a closed --allow-destructive gate denies the call
	// before any round trip, and an open gate still requires confirm:true.
	// Neither path reaches vcsim (both are denied before the handler runs), so
	// this safely reuses vm.
	t.Run("gate_and_confirm", func(t *testing.T) {
		dargs := guestDestructiveToolArgs(vm)
		if len(dargs) != 9 {
			t.Fatalf("test bug: guestDestructiveToolArgs has %d entries, expected 9", len(dargs))
		}
		for name, args := range dargs {
			t.Run(name, func(t *testing.T) {
				withConfirm := map[string]interface{}{}
				for k, v := range args {
					withConfirm[k] = v
				}
				withConfirm["confirm"] = true

				if _, err := closed.CallTool(name, withConfirm); err == nil {
					t.Fatalf("%s: expected the closed destructive gate to deny the call", name)
				}
				if _, err := open.CallTool(name, args); err == nil {
					t.Fatalf("%s: expected an error without confirm:true", name)
				}
			})
		}
	})

	// --- reaches_server --------------------------------------------------
	// Drive every tool with valid-shaped input (gate open, confirm:true for the
	// 9 destructive ones) and prove the wiring reaches vcsim's real
	// GuestOperationsManager dispatch — see assertGuestOpsWiringReachesServer.
	// vmware_guest_start_program is driven LAST and on startProgVM (not vm), so
	// its server-side lock poisoning (see TestGuestOps' doc comment) can't
	// deadlock any other call.
	t.Run("reaches_server", func(t *testing.T) {
		creds := map[string]interface{}{"vm": vm, "guest_username": "testuser", "guest_password": "testpass"}
		withCreds := func(extra map[string]interface{}) map[string]interface{} {
			m := map[string]interface{}{}
			for k, v := range creds {
				m[k] = v
			}
			for k, v := range extra {
				m[k] = v
			}
			return m
		}

		readOnly := map[string]map[string]interface{}{
			"vmware_guest_list_files":                withCreds(map[string]interface{}{"file_path": "/tmp"}),
			"vmware_guest_create_temp_file":          withCreds(map[string]interface{}{"prefix": "mcpvmware", "suffix": ".tmp"}),
			"vmware_guest_create_temp_directory":     withCreds(map[string]interface{}{"prefix": "mcpvmware", "suffix": ".tmp"}),
			"vmware_guest_file_transfer_from":        withCreds(map[string]interface{}{"guest_file_path": "/etc/hostname"}),
			"vmware_guest_list_processes":            withCreds(map[string]interface{}{}),
			"vmware_guest_read_environment_variable": withCreds(map[string]interface{}{"names": []interface{}{"PATH"}}),
		}
		for name, args := range readOnly {
			t.Run(name, func(t *testing.T) {
				_, err := open.CallTool(name, args)
				assertGuestOpsWiringReachesServer(t, err, name)
			})
		}

		// Every destructive tool EXCEPT start_program, on the shared vm — none
		// of these poison vcsim's object lock (they either succeed as no-ops or
		// return a clean fault; confirmed against the vcsim source).
		for name, args := range guestDestructiveToolArgs(vm) {
			if name == "vmware_guest_start_program" {
				continue
			}
			name, args := name, args
			t.Run(name, func(t *testing.T) {
				withConfirm := map[string]interface{}{}
				for k, v := range args {
					withConfirm[k] = v
				}
				withConfirm["confirm"] = true
				_, err := open.CallTool(name, withConfirm)
				assertGuestOpsWiringReachesServer(t, err, name)
			})
		}

		// start_program last, on the throwaway VM — see TestGuestOps' doc
		// comment for the vcsim StartProgramInGuest panic + WithLock-no-defer
		// lock-poison this deliberately isolates.
		t.Run("vmware_guest_start_program", func(t *testing.T) {
			spArgs := guestDestructiveToolArgs(startProgVM)["vmware_guest_start_program"]
			spArgs["confirm"] = true
			_, err := open.CallTool("vmware_guest_start_program", spArgs)
			assertGuestOpsWiringReachesServer(t, err, "vmware_guest_start_program")
		})
	})
}

package tools

import (
	"context"
	"encoding/base64"
	"reflect"
	"strings"
	"testing"

	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// newGuestRegistryAliasRegistry builds a Registry the normal way
// (NewRegistry, which wires every other file via registerTools, including
// registerSystemTools's vmware_list_vms this file's tests rely on for a VM
// path) and then manually layers this batch's 2 register functions on top
// via withClass — this file must not edit registry.go itself, same
// constraint/pattern as generated_guest_ops_test.go's newGuestOpsRegistry
// (a later coordination pass wires registerGuestWindowsRegistryTools/
// registerGuestAliasTools into Registry.registerTools and mode_test.go's
// vsphereGeneralTools list).
func newGuestRegistryAliasRegistry(ctx context.Context, c *vmware.Client, opts RegistryOptions) *Registry {
	r := NewRegistry(ctx, c, opts)
	r.withClass(modeVSphereGeneral, registerGuestWindowsRegistryTools)
	r.withClass(modeVSphereGeneral, registerGuestAliasTools)
	return r
}

// guestRegistryToolNames/guestAliasToolNames are the exact sets registered
// by registerGuestWindowsRegistryTools/registerGuestAliasTools — kept here
// so the registration subtest can't silently drift from the real
// registration lists in generated_guest_registry.go.
var guestRegistryToolNames = []string{
	"vmware_guest_registry_list_keys",
	"vmware_guest_registry_list_values",
	"vmware_guest_registry_create_key",
	"vmware_guest_registry_set_value",
	"vmware_guest_registry_delete_key",
	"vmware_guest_registry_delete_value",
}

var guestAliasToolNames = []string{
	"vmware_guest_alias_list",
	"vmware_guest_alias_list_mapped",
	"vmware_guest_alias_add",
	"vmware_guest_alias_remove",
	"vmware_guest_alias_remove_by_cert",
}

// guestRegistryDestructiveArgs builds the minimal valid (non-confirm)
// argument set for each of the 4 destructive GuestWindowsRegistryManager
// tools, keyed by tool name — shared by the gate_and_confirm and
// reaches_server subtests, mirroring generated_guest_ops_test.go's
// guestDestructiveToolArgs.
func guestRegistryDestructiveArgs(vm string) map[string]map[string]interface{} {
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
	const path = `HKEY_LOCAL_MACHINE\SOFTWARE\mcpvmware-test`
	return map[string]map[string]interface{}{
		"vmware_guest_registry_create_key":   merge(map[string]interface{}{"registry_path": path}),
		"vmware_guest_registry_set_value":    merge(map[string]interface{}{"registry_path": path, "value_type": "string", "value_data": "hello"}),
		"vmware_guest_registry_delete_key":   merge(map[string]interface{}{"registry_path": path}),
		"vmware_guest_registry_delete_value": merge(map[string]interface{}{"registry_path": path, "value_name": "MyValue"}),
	}
}

// guestAliasDestructiveArgs is guestRegistryDestructiveArgs' counterpart for
// GuestAliasManager's 3 destructive tools.
func guestAliasDestructiveArgs(vm string) map[string]map[string]interface{} {
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
		"vmware_guest_alias_add":            merge(map[string]interface{}{"username": "guestuser", "base64_cert": "TVRFU1RDRVJU"}),
		"vmware_guest_alias_remove":         merge(map[string]interface{}{"username": "guestuser", "base64_cert": "TVRFU1RDRVJU"}),
		"vmware_guest_alias_remove_by_cert": merge(map[string]interface{}{"username": "guestuser", "base64_cert": "TVRFU1RDRVJU"}),
	}
}

// assertGuestRegistryOrAliasNilGuard proves a guest-registry/guest-alias
// tool call reached this server's real handler AND a real vcsim
// property-collector round trip (schema parse, destructive gate/confirm,
// resolveVM, gwinManager/galiasManager's Properties() call against
// ServiceContent.GuestOperationsManager all wired correctly) — landing on
// THIS file's documented nil-guard (see generated_guest_registry.go's top
// doc comment: vcsim's GuestOperationsManager.init() never populates
// GuestWindowsRegistryManager/AliasManager, confirmed by reading
// referencia/govmomi/simulator/guest_operations_manager.go directly) rather
// than "unknown tool" (registration broken) or a recovered panic (a bug in
// this server's own handler). Stronger than the shared assertReachesServer
// alone (generated_vm_lifecycle_test.go) — it also asserts the error text
// is THIS specific nil-guard ("does not expose a
// guestWindowsRegistryManager"/"does not expose an aliasManager", both
// matched by the shared substring below), not some unrelated failure that
// happens to also return non-nil.
func assertGuestRegistryOrAliasNilGuard(t *testing.T, err error, tool string) {
	t.Helper()
	assertReachesServer(t, err, tool)
	if !strings.Contains(err.Error(), "does not expose a") {
		t.Fatalf("%s: expected the documented GuestWindowsRegistryManager/AliasManager nil-guard error, got: %v", tool, err)
	}
}

// TestGuestRegistryAliasTools exercises the 11 GuestWindowsRegistryManager +
// GuestAliasManager tools against a SINGLE shared vcsim server (one
// newSimClient for the whole file, per generated_guest_ops_test.go's
// established pattern for this package — vcsim never simulates either
// manager, so there is no per-VM object-lock deadlock risk here to isolate
// against, unlike TestGuestOps' vmware_guest_start_program case).
func TestGuestRegistryAliasTools(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	ctx := context.Background()
	open := newGuestRegistryAliasRegistry(ctx, c, RegistryOptions{AllowDestructive: true})
	closed := newGuestRegistryAliasRegistry(ctx, c, RegistryOptions{})

	vm := guestOpsVMPaths(t, open)[0] // reuses generated_guest_ops_test.go's helper — same package, same simulator.ESX() model shape

	allToolNames := func() []string {
		out := make([]string, 0, len(guestRegistryToolNames)+len(guestAliasToolNames))
		out = append(out, guestRegistryToolNames...)
		out = append(out, guestAliasToolNames...)
		return out
	}()

	// --- registration ------------------------------------------------------
	t.Run("registration", func(t *testing.T) {
		if len(guestRegistryToolNames) != 6 {
			t.Fatalf("test bug: guestRegistryToolNames has %d entries, expected 6", len(guestRegistryToolNames))
		}
		if len(guestAliasToolNames) != 5 {
			t.Fatalf("test bug: guestAliasToolNames has %d entries, expected 5", len(guestAliasToolNames))
		}
		got := toolNameSet(t, open) // shared helper, defined in mode_test.go
		for _, name := range allToolNames {
			if !got[name] {
				t.Errorf("tool %s not registered", name)
			}
		}
	})

	// --- missing_vm ----------------------------------------------------------
	// resolveVM's own "vm is required" check runs BEFORE gwinManager/
	// galiasManager ever contact vcsim (see generated_guest_registry.go's
	// gwinManager/galiasManager: resolveVM is called first) — the one
	// argument-validation case provable with NO round trip at all, for every
	// one of the 11 tools. Every OTHER required argument (registry_path,
	// guest_username, username, ...) also produces a non-nil error when
	// omitted, but only because gwinManager/galiasManager's manager-ref
	// resolution always fails first against vcsim (see reaches_server below)
	// — that path can't distinguish "bad argument" from "manager
	// unavailable," so this project does not claim it does.
	t.Run("missing_vm", func(t *testing.T) {
		args := map[string]interface{}{"guest_username": "u", "guest_password": "p", "confirm": true}
		for _, name := range allToolNames {
			t.Run(name, func(t *testing.T) {
				if _, err := open.CallTool(name, args); err == nil {
					t.Fatalf("%s: expected an error for missing vm", name)
				}
			})
		}
	})

	// --- gate_and_confirm ------------------------------------------------
	// The tier1/tier2 destructive protection must be wired on every one of
	// the 7 mutating tools: a closed --allow-destructive gate denies the
	// call before any round trip, and an open gate still requires
	// confirm:true. Neither path reaches vcsim (both are denied before the
	// handler runs), so this safely reuses vm.
	t.Run("gate_and_confirm", func(t *testing.T) {
		dargs := map[string]map[string]interface{}{}
		for name, args := range guestRegistryDestructiveArgs(vm) {
			dargs[name] = args
		}
		for name, args := range guestAliasDestructiveArgs(vm) {
			dargs[name] = args
		}
		if len(dargs) != 7 {
			t.Fatalf("test bug: destructive tool args has %d entries, expected 7", len(dargs))
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
	// Drive every tool with valid-shaped input (gate open, confirm:true for
	// the 7 destructive ones) and prove the wiring reaches vcsim's real
	// property collector, landing on this file's documented nil-guard — see
	// assertGuestRegistryOrAliasNilGuard.
	t.Run("reaches_server", func(t *testing.T) {
		readOnly := map[string]map[string]interface{}{
			"vmware_guest_registry_list_keys":   {"vm": vm, "guest_username": "u", "guest_password": "p", "registry_path": `HKEY_LOCAL_MACHINE\SOFTWARE`},
			"vmware_guest_registry_list_values": {"vm": vm, "guest_username": "u", "guest_password": "p", "registry_path": `HKEY_LOCAL_MACHINE\SOFTWARE`},
			"vmware_guest_alias_list":           {"vm": vm, "guest_username": "u", "guest_password": "p", "username": "guestuser"},
			"vmware_guest_alias_list_mapped":    {"vm": vm, "guest_username": "u", "guest_password": "p"},
		}
		for name, args := range readOnly {
			t.Run(name, func(t *testing.T) {
				_, err := open.CallTool(name, args)
				assertGuestRegistryOrAliasNilGuard(t, err, name)
			})
		}

		destructive := map[string]map[string]interface{}{}
		for name, args := range guestRegistryDestructiveArgs(vm) {
			destructive[name] = args
		}
		for name, args := range guestAliasDestructiveArgs(vm) {
			destructive[name] = args
		}
		for name, args := range destructive {
			t.Run(name, func(t *testing.T) {
				withConfirm := map[string]interface{}{}
				for k, v := range args {
					withConfirm[k] = v
				}
				withConfirm["confirm"] = true
				_, err := open.CallTool(name, withConfirm)
				assertGuestRegistryOrAliasNilGuard(t, err, name)
			})
		}
	})
}

// The subtests below are pure unit tests over this batch's argument-parsing/
// response-shaping helpers, calling them directly rather than through
// CallTool — necessary because, as TestGuestRegistryAliasTools' reaches_server
// subtest proves, vcsim's manager-ref nil-guard fires INSIDE
// gwinManager/galiasManager before a handler ever reaches
// gwinRegValueDataFromArgs/galiasSubjectFromArgs, so no vcsim-backed test can
// exercise this logic end-to-end. No newSimClient/network round trip
// involved in any of these.

// TestGuestRegistryValueDataFromArgs covers every value_type branch of
// gwinRegValueDataFromArgs (the discriminated-union builder for
// vmware_guest_registry_set_value) plus its error paths.
func TestGuestRegistryValueDataFromArgs(t *testing.T) {
	cases := []struct {
		name    string
		args    map[string]interface{}
		wantErr bool
		check   func(t *testing.T, data types.BaseGuestRegValueDataSpec)
	}{
		{
			name: "string",
			args: map[string]interface{}{"value_type": "string", "value_data": "hello"},
			check: func(t *testing.T, data types.BaseGuestRegValueDataSpec) {
				v, ok := data.(*types.GuestRegValueStringSpec)
				if !ok || v.Value != "hello" {
					t.Fatalf("unexpected data: %#v", data)
				}
			},
		},
		{
			name: "expand_string",
			args: map[string]interface{}{"value_type": "expand_string", "value_data": "%PATH%"},
			check: func(t *testing.T, data types.BaseGuestRegValueDataSpec) {
				v, ok := data.(*types.GuestRegValueExpandStringSpec)
				if !ok || v.Value != "%PATH%" {
					t.Fatalf("unexpected data: %#v", data)
				}
			},
		},
		{
			name: "multi_string",
			args: map[string]interface{}{"value_type": "multi_string", "value_data": []interface{}{"a", "b"}},
			check: func(t *testing.T, data types.BaseGuestRegValueDataSpec) {
				v, ok := data.(*types.GuestRegValueMultiStringSpec)
				if !ok || len(v.Value) != 2 || v.Value[0] != "a" || v.Value[1] != "b" {
					t.Fatalf("unexpected data: %#v", data)
				}
			},
		},
		{
			name: "multi_string_omitted_data",
			args: map[string]interface{}{"value_type": "multi_string"},
			check: func(t *testing.T, data types.BaseGuestRegValueDataSpec) {
				v, ok := data.(*types.GuestRegValueMultiStringSpec)
				if !ok || len(v.Value) != 0 {
					t.Fatalf("expected an empty multi-string spec, got: %#v", data)
				}
			},
		},
		{
			name: "dword",
			args: map[string]interface{}{"value_type": "dword", "value_data": float64(42)},
			check: func(t *testing.T, data types.BaseGuestRegValueDataSpec) {
				v, ok := data.(*types.GuestRegValueDwordSpec)
				if !ok || v.Value != 42 {
					t.Fatalf("unexpected data: %#v", data)
				}
			},
		},
		{
			name: "qword",
			args: map[string]interface{}{"value_type": "qword", "value_data": float64(9999999999)},
			check: func(t *testing.T, data types.BaseGuestRegValueDataSpec) {
				v, ok := data.(*types.GuestRegValueQwordSpec)
				if !ok || v.Value != 9999999999 {
					t.Fatalf("unexpected data: %#v", data)
				}
			},
		},
		{
			name: "binary",
			args: map[string]interface{}{"value_type": "binary", "value_data": base64.StdEncoding.EncodeToString([]byte{1, 2, 3})},
			check: func(t *testing.T, data types.BaseGuestRegValueDataSpec) {
				v, ok := data.(*types.GuestRegValueBinarySpec)
				if !ok || len(v.Value) != 3 || v.Value[0] != 1 || v.Value[1] != 2 || v.Value[2] != 3 {
					t.Fatalf("unexpected data: %#v", data)
				}
			},
		},
		{name: "missing_value_type", args: map[string]interface{}{}, wantErr: true},
		{name: "invalid_value_type", args: map[string]interface{}{"value_type": "bogus"}, wantErr: true},
		{name: "dword_missing_data", args: map[string]interface{}{"value_type": "dword"}, wantErr: true},
		{name: "qword_missing_data", args: map[string]interface{}{"value_type": "qword"}, wantErr: true},
		{name: "binary_invalid_base64", args: map[string]interface{}{"value_type": "binary", "value_data": "not valid base64!!"}, wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := gwinRegValueDataFromArgs(tc.args)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tc.check(t, data)
		})
	}
}

// TestGuestRegistryValueDataToJSON is gwinRegValueDataFromArgs' inverse —
// every concrete BaseGuestRegValueDataSpec variant round-trips to the
// documented {"type":..., "value":...} shape.
func TestGuestRegistryValueDataToJSON(t *testing.T) {
	cases := []struct {
		name      string
		data      types.BaseGuestRegValueDataSpec
		wantType  string
		wantValue interface{}
	}{
		{"dword", &types.GuestRegValueDwordSpec{Value: 7}, "dword", int32(7)},
		{"qword", &types.GuestRegValueQwordSpec{Value: 8}, "qword", int64(8)},
		{"string", &types.GuestRegValueStringSpec{Value: "x"}, "string", "x"},
		{"expand_string", &types.GuestRegValueExpandStringSpec{Value: "y"}, "expand_string", "y"},
		{"multi_string", &types.GuestRegValueMultiStringSpec{Value: []string{"a", "b"}}, "multi_string", []string{"a", "b"}},
		{"binary", &types.GuestRegValueBinarySpec{Value: []byte{1, 2}}, "binary", base64.StdEncoding.EncodeToString([]byte{1, 2})},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := gwinValueDataToJSON(tc.data).(map[string]interface{})
			if !ok {
				t.Fatalf("expected a map, got %#v", gwinValueDataToJSON(tc.data))
			}
			if got["type"] != tc.wantType {
				t.Fatalf("type: got %v, want %v", got["type"], tc.wantType)
			}
			if !reflect.DeepEqual(got["value"], tc.wantValue) {
				t.Fatalf("value: got %#v, want %#v", got["value"], tc.wantValue)
			}
		})
	}

	if gwinValueDataToJSON(nil) != nil {
		t.Fatalf("expected nil for a nil BaseGuestRegValueDataSpec")
	}
}

// TestGuestRegistryKeyNameSpec covers gwinKeyNameSpec's default wow_bitness
// and required registry_path.
func TestGuestRegistryKeyNameSpec(t *testing.T) {
	t.Run("default_wow_bitness", func(t *testing.T) {
		spec, err := gwinKeyNameSpec(map[string]interface{}{"registry_path": `HKEY_LOCAL_MACHINE\SOFTWARE`})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if spec.WowBitness != string(types.GuestRegKeyWowSpecWOWNative) {
			t.Fatalf("expected default WowBitness WOWNative, got %q", spec.WowBitness)
		}
	})
	t.Run("explicit_wow_bitness", func(t *testing.T) {
		spec, err := gwinKeyNameSpec(map[string]interface{}{"registry_path": `HKEY_LOCAL_MACHINE\SOFTWARE`, "wow_bitness": "WOW64"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if spec.WowBitness != "WOW64" {
			t.Fatalf("expected WOW64, got %q", spec.WowBitness)
		}
	})
	t.Run("missing_registry_path", func(t *testing.T) {
		if _, err := gwinKeyNameSpec(map[string]interface{}{}); err == nil {
			t.Fatal("expected an error for missing registry_path")
		}
	})
}

// TestGuestRegistryValueNameSpec covers gwinValueNameSpec's "empty
// value_name means the key's unnamed/default value" contract.
func TestGuestRegistryValueNameSpec(t *testing.T) {
	t.Run("empty_value_name_is_default_value", func(t *testing.T) {
		spec, err := gwinValueNameSpec(map[string]interface{}{"registry_path": `HKEY_LOCAL_MACHINE\SOFTWARE`})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if spec.Name != "" {
			t.Fatalf("expected an empty Name for the key's default value, got %q", spec.Name)
		}
	})
	t.Run("named_value", func(t *testing.T) {
		spec, err := gwinValueNameSpec(map[string]interface{}{"registry_path": `HKEY_LOCAL_MACHINE\SOFTWARE`, "value_name": "MyValue"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if spec.Name != "MyValue" {
			t.Fatalf("expected Name MyValue, got %q", spec.Name)
		}
	})
	t.Run("missing_registry_path", func(t *testing.T) {
		if _, err := gwinValueNameSpec(map[string]interface{}{"value_name": "x"}); err == nil {
			t.Fatal("expected an error for missing registry_path")
		}
	})
}

// TestGuestAliasSubjectFromArgs covers galiasSubjectFromArgs' default
// ("any"), explicit "any"/"named" branches, and error paths.
func TestGuestAliasSubjectFromArgs(t *testing.T) {
	t.Run("default_any", func(t *testing.T) {
		s, err := galiasSubjectFromArgs(map[string]interface{}{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, ok := s.(*types.GuestAuthAnySubject); !ok {
			t.Fatalf("expected *types.GuestAuthAnySubject, got %#v", s)
		}
	})
	t.Run("explicit_any", func(t *testing.T) {
		s, err := galiasSubjectFromArgs(map[string]interface{}{"subject_type": "any"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, ok := s.(*types.GuestAuthAnySubject); !ok {
			t.Fatalf("expected *types.GuestAuthAnySubject, got %#v", s)
		}
	})
	t.Run("named", func(t *testing.T) {
		s, err := galiasSubjectFromArgs(map[string]interface{}{"subject_type": "named", "subject_name": "alice"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		named, ok := s.(*types.GuestAuthNamedSubject)
		if !ok || named.Name != "alice" {
			t.Fatalf("unexpected subject: %#v", s)
		}
	})
	t.Run("named_missing_subject_name", func(t *testing.T) {
		if _, err := galiasSubjectFromArgs(map[string]interface{}{"subject_type": "named"}); err == nil {
			t.Fatal("expected an error for missing subject_name")
		}
	})
	t.Run("invalid_subject_type", func(t *testing.T) {
		if _, err := galiasSubjectFromArgs(map[string]interface{}{"subject_type": "bogus"}); err == nil {
			t.Fatal("expected an error for invalid subject_type")
		}
	})
}

// TestGuestAliasSubjectToJSON is galiasSubjectFromArgs' inverse.
func TestGuestAliasSubjectToJSON(t *testing.T) {
	t.Run("any", func(t *testing.T) {
		got, ok := galiasSubjectToJSON(&types.GuestAuthAnySubject{}).(map[string]interface{})
		if !ok || got["type"] != "any" {
			t.Fatalf("unexpected any-subject JSON: %#v", got)
		}
	})
	t.Run("named", func(t *testing.T) {
		got, ok := galiasSubjectToJSON(&types.GuestAuthNamedSubject{Name: "bob"}).(map[string]interface{})
		if !ok || got["type"] != "named" || got["name"] != "bob" {
			t.Fatalf("unexpected named-subject JSON: %#v", got)
		}
	})
	t.Run("nil", func(t *testing.T) {
		if galiasSubjectToJSON(nil) != nil {
			t.Fatal("expected nil for a nil BaseGuestAuthSubject")
		}
	})
}

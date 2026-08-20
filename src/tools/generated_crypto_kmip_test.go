package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/vmware/govmomi/simulator"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// newCryptoKmipRegistry builds a Registry the normal way and layers
// registerCryptoKmipTools on top via withClass — same pattern as
// generated_crypto_test.go's newCryptoRegistry; must not edit registry.go
// (this batch runs in parallel with another agent's work there).
func newCryptoKmipRegistry(ctx context.Context, c *vmware.Client, opts RegistryOptions) *Registry {
	r := NewRegistry(ctx, c, opts)
	r.withClass(modeVCenterOnly, registerCryptoKmipTools)
	return r
}

// cryptoKmipToolNames is every tool this file registers, grouped exactly as
// generated_crypto_kmip.go registers them — used by both the registration
// test and (implicitly, by construction) as the master list this whole test
// file must account for.
func cryptoKmipToolNames() []string {
	return []string{
		// topology
		"vmware_crypto_list_kmip_servers",
		"vmware_crypto_register_kmip_server",
		"vmware_crypto_update_kmip_server",
		"vmware_crypto_remove_kmip_server",
		"vmware_crypto_register_kms_cluster",
		"vmware_crypto_unregister_kms_cluster",
		"vmware_crypto_get_default_kms_cluster",
		"vmware_crypto_set_default_kms_cluster",
		"vmware_crypto_mark_default_kms_cluster",
		"vmware_crypto_retrieve_kmip_servers_status",
		"vmware_crypto_is_kms_cluster_active",
		// keys
		"vmware_crypto_add_key",
		"vmware_crypto_add_keys",
		"vmware_crypto_remove_key",
		"vmware_crypto_remove_keys",
		"vmware_crypto_list_keys",
		"vmware_crypto_query_key_status",
		"vmware_crypto_generate_key",
		// certificates
		"vmware_crypto_generate_client_csr",
		"vmware_crypto_generate_self_signed_client_cert",
		"vmware_crypto_retrieve_client_csr",
		"vmware_crypto_retrieve_client_cert",
		"vmware_crypto_retrieve_kmip_server_cert",
		"vmware_crypto_retrieve_self_signed_client_cert",
		"vmware_crypto_upload_kmip_server_cert",
		"vmware_crypto_upload_client_cert",
		"vmware_crypto_update_kms_signed_csr_client_cert",
	}
}

func TestCryptoKmipTools_Registration(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newCryptoKmipRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	want := cryptoKmipToolNames()
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

	// Must not collide with the pre-existing vapi/crypto tool names
	// (generated_crypto.go) — a literal string-set check, not a registry
	// presence check: NewRegistry's "normal way" (registry.go's
	// registerTools) already registers generated_crypto.go's 3 tools
	// unconditionally as part of the full ~700-tool set, so all 3 ARE
	// expected to be present in r.ListTools() regardless of this file —
	// what actually matters is that none of this file's 27 names collide
	// with theirs, confirmed here at the string level.
	preExisting := map[string]bool{
		"vmware_crypto_kms_provider_create": true,
		"vmware_crypto_kms_provider_delete": true,
		"vmware_crypto_kms_provider_export": true,
	}
	for _, name := range want {
		if preExisting[name] {
			t.Errorf("test bug: %s collides with a generated_crypto.go tool name", name)
		}
	}
}

// TestCryptoKmipTools_TierGating spot-checks the gate/confirm checks for a
// tier1 tool (remove_key) and a tier2 tool (add_key) actually deny before
// any handler logic runs — same discipline as generated_crypto_test.go's
// TestCryptoTools_TierGating.
func TestCryptoKmipTools_TierGating(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	closedGate := newCryptoKmipRegistry(context.Background(), c, RegistryOptions{AllowDestructive: false})
	openGate := newCryptoKmipRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	t.Run("add_key_tier2", func(t *testing.T) {
		args := map[string]interface{}{"key_id": "k1", "algorithm": "AES-256", "key_data": "deadbeef", "confirm": true}
		if _, err := closedGate.CallTool("vmware_crypto_add_key", args); err == nil {
			t.Fatal("expected denial with the gate closed")
		}
		argsNoConfirm := map[string]interface{}{"key_id": "k1", "algorithm": "AES-256", "key_data": "deadbeef"}
		if _, err := openGate.CallTool("vmware_crypto_add_key", argsNoConfirm); err == nil {
			t.Fatal("expected denial without confirm:true")
		}
	})

	t.Run("remove_key_tier1", func(t *testing.T) {
		args := map[string]interface{}{"key_id": "k1", "confirm": true}
		if _, err := closedGate.CallTool("vmware_crypto_remove_key", args); err == nil {
			t.Fatal("expected denial with the gate closed")
		}
		argsNoConfirm := map[string]interface{}{"key_id": "k1"}
		if _, err := openGate.CallTool("vmware_crypto_remove_key", argsNoConfirm); err == nil {
			t.Fatal("expected denial without confirm:true")
		}
	})

	t.Run("unregister_kms_cluster_tier1", func(t *testing.T) {
		args := map[string]interface{}{"cluster_id": "c1", "confirm": true}
		if _, err := closedGate.CallTool("vmware_crypto_unregister_kms_cluster", args); err == nil {
			t.Fatal("expected denial with the gate closed")
		}
	})
}

// TestCryptoKmipTools_RequiredArgsValidation spot-checks required-argument
// validation across the 3 argument shapes this file uses (flattened scalar,
// nested object via decodeJSONArg, array via decodeJSONArg) without
// round-tripping to vcsim for any of them — every one of these must fail
// before cryptoManagerRef/methods.Xxx is ever reached.
func TestCryptoKmipTools_RequiredArgsValidation(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newCryptoKmipRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	cases := []struct {
		name string
		args map[string]interface{}
	}{
		{"vmware_crypto_register_kmip_server", map[string]interface{}{"confirm": true}},                                                              // missing cluster_id + server_info
		{"vmware_crypto_register_kmip_server", map[string]interface{}{"cluster_id": "c1", "confirm": true}},                                          // missing server_info
		{"vmware_crypto_register_kmip_server", map[string]interface{}{"cluster_id": "c1", "server_info": map[string]interface{}{}, "confirm": true}}, // server_info.name missing
		{"vmware_crypto_remove_kmip_server", map[string]interface{}{"cluster_id": "c1", "confirm": true}},                                            // missing server_name
		{"vmware_crypto_add_key", map[string]interface{}{"confirm": true}},                                                                           // missing key_id/algorithm/key_data
		{"vmware_crypto_add_keys", map[string]interface{}{"confirm": true}},                                                                          // missing keys
		{"vmware_crypto_add_keys", map[string]interface{}{"keys": []interface{}{}, "confirm": true}},                                                 // empty keys
		{"vmware_crypto_remove_key", map[string]interface{}{"confirm": true}},                                                                        // missing key_id
		{"vmware_crypto_remove_keys", map[string]interface{}{"keys": []interface{}{}, "confirm": true}},                                              // empty keys
		{"vmware_crypto_query_key_status", map[string]interface{}{}},                                                                                 // missing key_ids
		{"vmware_crypto_query_key_status", map[string]interface{}{"key_ids": []interface{}{}}},                                                       // empty key_ids
		{"vmware_crypto_generate_client_csr", map[string]interface{}{}},                                                                              // missing cluster_id
		{"vmware_crypto_retrieve_kmip_server_cert", map[string]interface{}{"cluster_id": "c1"}},                                                      // missing server_info
		{"vmware_crypto_upload_client_cert", map[string]interface{}{"cluster_id": "c1", "certificate": "PEM", "confirm": true}},                      // missing private_key
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := r.CallTool(tc.name, tc.args); err == nil {
				t.Fatalf("%s: expected a validation error for args %v", tc.name, tc.args)
			}
		})
	}
}

// TestCryptoKmipTools_UnsimulatedMethods_ReachServer proves the 14 tools
// backed by methods with no Go receiver on simulator.CryptoManagerKmip (this
// file's top doc comment's "vcsim coverage" section) reach the real vcsim
// server and get back a genuine types.MethodNotFound-class fault — not a
// panic, not "unknown tool" — using assertReachesServer
// (generated_vm_lifecycle_test.go, reused).
func TestCryptoKmipTools_UnsimulatedMethods_ReachServer(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newCryptoKmipRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	cases := []struct {
		name string
		args map[string]interface{}
	}{
		{"vmware_crypto_add_key", map[string]interface{}{"key_id": "k1", "algorithm": "AES-256", "key_data": "deadbeef"}},
		{"vmware_crypto_add_keys", map[string]interface{}{"keys": []interface{}{map[string]interface{}{"keyId": map[string]interface{}{"keyId": "k1"}, "algorithm": "AES-256", "keyData": "deadbeef"}}}},
		{"vmware_crypto_remove_key", map[string]interface{}{"key_id": "k1"}},
		{"vmware_crypto_remove_keys", map[string]interface{}{"keys": []interface{}{map[string]interface{}{"keyId": "k1"}}}},
		{"vmware_crypto_is_kms_cluster_active", map[string]interface{}{"cluster_id": "c1"}},
		{"vmware_crypto_generate_client_csr", map[string]interface{}{"cluster_id": "c1"}},
		{"vmware_crypto_generate_self_signed_client_cert", map[string]interface{}{"cluster_id": "c1"}},
		{"vmware_crypto_retrieve_client_csr", map[string]interface{}{"cluster_id": "c1"}},
		{"vmware_crypto_retrieve_client_cert", map[string]interface{}{"cluster_id": "c1"}},
		{"vmware_crypto_retrieve_kmip_server_cert", map[string]interface{}{"cluster_id": "c1", "server_info": map[string]interface{}{"name": "s1", "address": "1.2.3.4", "port": 5696}}},
		{"vmware_crypto_retrieve_self_signed_client_cert", map[string]interface{}{"cluster_id": "c1"}},
		{"vmware_crypto_upload_kmip_server_cert", map[string]interface{}{"cluster_id": "c1", "certificate": "PEM"}},
		{"vmware_crypto_upload_client_cert", map[string]interface{}{"cluster_id": "c1", "certificate": "PEM", "private_key": "KEY"}},
		{"vmware_crypto_update_kms_signed_csr_client_cert", map[string]interface{}{"cluster_id": "c1", "certificate": "PEM"}},
	}
	if len(cases) != 14 {
		t.Fatalf("test bug: cases has %d entries, expected 14 (this file's documented per-method vcsim gap count)", len(cases))
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			args := map[string]interface{}{}
			for k, v := range tc.args {
				args[k] = v
			}
			args["confirm"] = true
			_, err := r.CallTool(tc.name, args)
			assertReachesServer(t, err, tc.name)
		})
	}
}

// TestCryptoKmipTools_ImplementedMethods drives the 13 tools backed by a
// real simulator.CryptoManagerKmip Go method (this file's top doc comment's
// "vcsim coverage" section) through genuine functional round trips against
// vcsim, not just "reaches server" — register topology, read it back,
// generate/list/query/remove a key, retrieve status, set/get/mark defaults,
// unregister. Each subtest uses its own cluster ID(s) to stay independent of
// subtest execution order (Go runs top-level t.Run subtests sequentially,
// but this avoids relying on that).
//
// management_type is always passed explicitly as "unknown" — vcsim's
// RegisterKmsCluster (referencia/govmomi/simulator/crypto_manager_kmip.go)
// only accepts {unknown, trustAuthority, nativeProvider} and does not apply
// the real API's "defaults to trustAuthority when omitted" behavior itself;
// confirmed by reading its validClusterTypes check before relying on it
// here.
func TestCryptoKmipTools_ImplementedMethods(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newCryptoKmipRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	t.Run("register_kms_cluster_and_kmip_server_then_list", func(t *testing.T) {
		mustCall(t, r, "vmware_crypto_register_kms_cluster", map[string]interface{}{
			"cluster_id": "cluster-A", "management_type": "unknown", "confirm": true,
		})
		mustCall(t, r, "vmware_crypto_register_kmip_server", map[string]interface{}{
			"cluster_id":  "cluster-A",
			"server_info": map[string]interface{}{"name": "kmip1", "address": "10.0.0.5", "port": 5696},
			"confirm":     true,
		})

		out := mustCall(t, r, "vmware_crypto_list_kmip_servers", map[string]interface{}{})
		if !strings.Contains(out, "cluster-A") || !strings.Contains(out, "kmip1") {
			t.Errorf("expected cluster-A/kmip1 in list_kmip_servers output, got %s", out)
		}
	})

	t.Run("update_and_remove_kmip_server", func(t *testing.T) {
		mustCall(t, r, "vmware_crypto_register_kms_cluster", map[string]interface{}{
			"cluster_id": "cluster-B", "management_type": "unknown", "confirm": true,
		})
		mustCall(t, r, "vmware_crypto_register_kmip_server", map[string]interface{}{
			"cluster_id":  "cluster-B",
			"server_info": map[string]interface{}{"name": "kmipB", "address": "10.0.0.6", "port": 5696},
			"confirm":     true,
		})
		mustCall(t, r, "vmware_crypto_update_kmip_server", map[string]interface{}{
			"cluster_id":  "cluster-B",
			"server_info": map[string]interface{}{"name": "kmipB", "address": "10.0.0.7", "port": 5697},
			"confirm":     true,
		})
		mustCall(t, r, "vmware_crypto_remove_kmip_server", map[string]interface{}{
			"cluster_id": "cluster-B", "server_name": "kmipB", "confirm": true,
		})

		// removing the same server again must fail (invalid server name, per
		// simulator.CryptoManagerKmip.RemoveKmipServer)
		if _, err := r.CallTool("vmware_crypto_remove_kmip_server", map[string]interface{}{
			"cluster_id": "cluster-B", "server_name": "kmipB", "confirm": true,
		}); err == nil {
			t.Error("expected an error removing an already-removed KMIP server")
		}
	})

	t.Run("default_cluster_get_set_mark", func(t *testing.T) {
		mustCall(t, r, "vmware_crypto_register_kms_cluster", map[string]interface{}{
			"cluster_id": "cluster-C", "management_type": "unknown", "confirm": true,
		})

		// simulator.CryptoManagerKmip.GetDefaultKmsCluster faults ("No
		// default provider") rather than returning a nil Returnval when no
		// cluster has been marked default yet — confirmed by reading its
		// source before asserting on it here, not assumed from the real
		// API's doc comment (which only says Returnval is optional).
		if _, err := r.CallTool("vmware_crypto_get_default_kms_cluster", map[string]interface{}{}); err == nil {
			t.Error("expected an error getting the default KMS cluster before any cluster is marked default")
		}

		mustCall(t, r, "vmware_crypto_mark_default_kms_cluster", map[string]interface{}{"cluster_id": "cluster-C", "confirm": true})

		out := mustCall(t, r, "vmware_crypto_get_default_kms_cluster", map[string]interface{}{})
		if !strings.Contains(out, "cluster-C") {
			t.Errorf("expected cluster-C as the default after mark_default, got %s", out)
		}

		mustCall(t, r, "vmware_crypto_set_default_kms_cluster", map[string]interface{}{"confirm": true}) // cluster_id omitted -> clears

		// same "faults instead of null" behavior as above, now that the
		// default was just cleared.
		if _, err := r.CallTool("vmware_crypto_get_default_kms_cluster", map[string]interface{}{}); err == nil {
			t.Error("expected an error getting the default KMS cluster after it was cleared")
		}
	})

	t.Run("retrieve_kmip_servers_status", func(t *testing.T) {
		mustCall(t, r, "vmware_crypto_register_kms_cluster", map[string]interface{}{
			"cluster_id": "cluster-D", "management_type": "unknown", "confirm": true,
		})
		mustCall(t, r, "vmware_crypto_register_kmip_server", map[string]interface{}{
			"cluster_id":  "cluster-D",
			"server_info": map[string]interface{}{"name": "kmipD", "address": "10.0.0.8", "port": 5696},
			"confirm":     true,
		})

		out := mustCall(t, r, "vmware_crypto_retrieve_kmip_servers_status", map[string]interface{}{
			"cluster_ids": []interface{}{"cluster-D"},
		})
		if !strings.Contains(out, "cluster-D") {
			t.Errorf("expected cluster-D in retrieve_kmip_servers_status output, got %s", out)
		}
		if !strings.Contains(out, `"count": 1`) { // marshalJSON pretty-prints (json.MarshalIndent) — space after ':'
			t.Errorf("expected exactly 1 cluster status, got %s", out)
		}
	})

	t.Run("generate_list_query_key", func(t *testing.T) {
		mustCall(t, r, "vmware_crypto_register_kms_cluster", map[string]interface{}{
			"cluster_id": "cluster-E", "management_type": "unknown", "confirm": true,
		})

		genOut := mustCall(t, r, "vmware_crypto_generate_key", map[string]interface{}{"cluster_id": "cluster-E", "confirm": true})
		var gen struct {
			Result struct {
				KeyId struct {
					KeyId string `json:"keyId"`
				} `json:"keyId"`
				Success bool `json:"success"`
			} `json:"result"`
		}
		if err := json.Unmarshal([]byte(genOut), &gen); err != nil {
			t.Fatalf("failed to parse generate_key output %q: %v", genOut, err)
		}
		if !gen.Result.Success || gen.Result.KeyId.KeyId == "" {
			t.Fatalf("expected a successful key generation with a non-empty key ID, got %s", genOut)
		}
		keyID := gen.Result.KeyId.KeyId

		listOut := mustCall(t, r, "vmware_crypto_list_keys", map[string]interface{}{})
		if !strings.Contains(listOut, keyID) {
			t.Errorf("expected generated key %q in list_keys output, got %s", keyID, listOut)
		}

		queryOut := mustCall(t, r, "vmware_crypto_query_key_status", map[string]interface{}{
			"key_ids": []interface{}{map[string]interface{}{"keyId": keyID}},
		})
		if !strings.Contains(queryOut, keyID) {
			t.Errorf("expected key %q in query_key_status output, got %s", keyID, queryOut)
		}

		// Not calling vmware_crypto_remove_key here: RemoveKey (unlike
		// RemoveKmipServer/UnregisterKmsCluster) has no Go receiver on
		// simulator.CryptoManagerKmip at all (this file's top doc comment's
		// "vcsim coverage" list) — it is exercised instead by
		// TestCryptoKmipTools_UnsimulatedMethods_ReachServer.
	})

	t.Run("unregister_kms_cluster", func(t *testing.T) {
		mustCall(t, r, "vmware_crypto_register_kms_cluster", map[string]interface{}{
			"cluster_id": "cluster-F", "management_type": "unknown", "confirm": true,
		})
		mustCall(t, r, "vmware_crypto_unregister_kms_cluster", map[string]interface{}{"cluster_id": "cluster-F", "confirm": true})

		// unregistering again must fail (invalid cluster ID, per
		// simulator.CryptoManagerKmip.UnregisterKmsCluster)
		if _, err := r.CallTool("vmware_crypto_unregister_kms_cluster", map[string]interface{}{
			"cluster_id": "cluster-F", "confirm": true,
		}); err == nil {
			t.Error("expected an error unregistering an already-unregistered KMS cluster")
		}
	})
}

package tools

import (
	"context"
	"testing"

	"github.com/vmware/govmomi/simulator"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// vcsim gap, not a bug — see generated_crypto.go's top doc comment (the
// strictest form in this batch: no vapi/crypto/simulator package exists
// upstream at all). Same test discipline as
// generated_esx_settings_cluster_vms_test.go's top doc comment, not repeated
// verbatim here.

// newCryptoRegistry builds a Registry the normal way and layers
// registerCryptoTools on top via withClass — same pattern as this batch's
// other test files; must not edit registry.go.
func newCryptoRegistry(ctx context.Context, c *vmware.Client, opts RegistryOptions) *Registry {
	r := NewRegistry(ctx, c, opts)
	r.withClass(modeVCenterOnly, registerCryptoTools)
	return r
}

func TestCryptoTools_Registration(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newCryptoRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	want := []string{
		"vmware_crypto_kms_provider_create",
		"vmware_crypto_kms_provider_delete",
		"vmware_crypto_kms_provider_export",
	}
	if len(want) != 3 {
		t.Fatalf("test bug: want list has %d entries, expected 3", len(want))
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

	// KmsProviderExportRequest deliberately has no tool — see this group's
	// .go file top doc comment (Curation).
	if got["vmware_crypto_kms_provider_export_request"] {
		t.Error("vmware_crypto_kms_provider_export_request should not be registered — see curation note in generated_crypto.go")
	}
}

// TestCryptoTools_TierGating proves the gate/confirm checks for the tier1
// tool (kms_provider_delete) and a tier2 tool (kms_provider_create) actually
// deny before any handler logic runs.
func TestCryptoTools_TierGating(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	closedGate := newCryptoRegistry(context.Background(), c, RegistryOptions{AllowDestructive: false})
	openGate := newCryptoRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	t.Run("kms_provider_create_tier2", func(t *testing.T) {
		args := map[string]interface{}{"provider": "kms1", "confirm": true}
		if _, err := closedGate.CallTool("vmware_crypto_kms_provider_create", args); err == nil {
			t.Fatal("expected denial with the gate closed")
		}
		argsNoConfirm := map[string]interface{}{"provider": "kms1"}
		if _, err := openGate.CallTool("vmware_crypto_kms_provider_create", argsNoConfirm); err == nil {
			t.Fatal("expected denial without confirm:true")
		}
	})

	t.Run("kms_provider_delete_tier1", func(t *testing.T) {
		args := map[string]interface{}{"provider": "kms1", "confirm": true}
		if _, err := closedGate.CallTool("vmware_crypto_kms_provider_delete", args); err == nil {
			t.Fatal("expected denial with the gate closed")
		}
		argsNoConfirm := map[string]interface{}{"provider": "kms1"}
		if _, err := openGate.CallTool("vmware_crypto_kms_provider_delete", argsNoConfirm); err == nil {
			t.Fatal("expected denial without confirm:true")
		}
	})

	t.Run("kms_provider_export_tier2", func(t *testing.T) {
		args := map[string]interface{}{"provider": "kms1", "confirm": true}
		if _, err := closedGate.CallTool("vmware_crypto_kms_provider_export", args); err == nil {
			t.Fatal("expected denial with the gate closed")
		}
	})
}

func TestCryptoTools_RequiredArgsValidation(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newCryptoRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	t.Run("create_missing_provider", func(t *testing.T) {
		if _, err := r.CallTool("vmware_crypto_kms_provider_create", map[string]interface{}{"confirm": true}); err == nil {
			t.Fatal("expected an error when provider is missing")
		}
	})

	t.Run("delete_missing_provider", func(t *testing.T) {
		if _, err := r.CallTool("vmware_crypto_kms_provider_delete", map[string]interface{}{"confirm": true}); err == nil {
			t.Fatal("expected an error when provider is missing")
		}
	})

	t.Run("export_missing_provider", func(t *testing.T) {
		if _, err := r.CallTool("vmware_crypto_kms_provider_export", map[string]interface{}{"confirm": true}); err == nil {
			t.Fatal("expected an error when provider is missing")
		}
	})
}

type cryptoCase struct {
	name string
	tier tier
	args map[string]interface{}
}

func cryptoCases() []cryptoCase {
	return []cryptoCase{
		{"vmware_crypto_kms_provider_create", tier2, map[string]interface{}{"provider": "kms1", "tpm_required": true}},
		{"vmware_crypto_kms_provider_delete", tier1, map[string]interface{}{"provider": "kms1"}},
		{"vmware_crypto_kms_provider_export", tier2, map[string]interface{}{"provider": "kms1", "password": "s3cret"}},
	}
}

// TestCryptoTools_ReachesServer proves every one of the 3 tools, given fully
// valid input (gate open, confirm:true), reaches the real vcsim server and
// gets back a genuine error — not a panic, not "unknown tool". No
// vapi/crypto/simulator package exists upstream at all (this file's top doc
// comment), so this also incidentally proves the REST/VAPI login itself
// (client.REST) succeeds against vcsim even though the crypto endpoints
// specifically do not.
func TestCryptoTools_ReachesServer(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newCryptoRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	for _, tc := range cryptoCases() {
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

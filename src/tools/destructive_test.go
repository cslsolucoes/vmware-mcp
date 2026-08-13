package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/vmware/govmomi/simulator"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// newDestructiveTestRegistry builds a Registry against a standalone-ESXi
// vcsim and registers a single dummy Tier 1 tool through registerDestructive
// — no real Tier 1/2 tool exists yet (Fase 1a only builds the mechanism;
// Fases 1/3 wrap their real handlers with it). called is set true if and
// only if the wrapped handler actually ran, proving the gate/confirm checks
// happen BEFORE any vCenter/ESXi round trip, not just before returning a
// result.
func newDestructiveTestRegistry(t *testing.T, opts RegistryOptions) (r *Registry, called *bool) {
	t.Helper()
	c, cleanup := newSimClient(t, simulator.ESX())
	t.Cleanup(cleanup)

	r = NewRegistry(context.Background(), c, opts)
	called = new(bool)
	r.registerDestructive("test_destructive_tool", "test-only tool for Fase 1a coverage", tier1,
		map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
		Tool{Handler: func(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
			*called = true
			return marshalJSON(map[string]interface{}{"ok": true})
		}},
	)
	return r, called
}

func TestDestructive_GateClosedDeniesBeforeHandlerRuns(t *testing.T) {
	r, called := newDestructiveTestRegistry(t, RegistryOptions{AllowDestructive: false})

	_, err := r.CallTool("test_destructive_tool", map[string]interface{}{"confirm": true})
	if err == nil {
		t.Fatal("expected an error with the gate closed, got none")
	}
	if *called {
		t.Fatal("handler ran despite the gate being closed — confirm:true must never be enough on its own")
	}
}

func TestDestructive_ConfirmRequiredEvenWithGateOpen(t *testing.T) {
	r, called := newDestructiveTestRegistry(t, RegistryOptions{AllowDestructive: true})

	cases := []map[string]interface{}{
		{},                  // missing confirm
		{"confirm": false},  // explicit false
		{"confirm": "true"}, // truthy string, not a bool — must NOT count
		{"confirm": 1},      // truthy int, not a bool — must NOT count
	}
	for _, args := range cases {
		_, err := r.CallTool("test_destructive_tool", args)
		if err == nil {
			t.Fatalf("expected an error for args %#v, got none", args)
		}
	}
	if *called {
		t.Fatal("handler ran without a strict confirm:true — the check must be exact-bool, not truthy")
	}
}

func TestDestructive_AllowedRunsHandlerAndAudits(t *testing.T) {
	auditPath := filepath.Join(t.TempDir(), "audit.jsonl")
	r, called := newDestructiveTestRegistry(t, RegistryOptions{AllowDestructive: true, AuditLogPath: auditPath})

	raw, err := r.CallTool("test_destructive_tool", map[string]interface{}{"confirm": true})
	if err != nil {
		t.Fatalf("expected success with gate open + confirm:true, got: %v", err)
	}
	if !*called {
		t.Fatal("handler did not run despite gate open + confirm:true")
	}
	if raw == "" {
		t.Fatal("expected a non-empty result from the wrapped handler")
	}

	data, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("expected an audit log at %s: %v", auditPath, err)
	}
	var entry auditEntry
	if err := json.Unmarshal(data[:len(data)-1], &entry); err != nil { // trim trailing "\n"
		t.Fatalf("audit log line is not valid JSON: %v (%s)", err, data)
	}
	if entry.Tool != "test_destructive_tool" || entry.Tier != tier1 || !entry.Allowed || !entry.GateOpen || !entry.Confirmed {
		t.Fatalf("audit entry has unexpected shape: %+v", entry)
	}
}

func TestDestructive_DeniedCallsAreAuditedToo(t *testing.T) {
	auditPath := filepath.Join(t.TempDir(), "audit.jsonl")
	r, _ := newDestructiveTestRegistry(t, RegistryOptions{AllowDestructive: false, AuditLogPath: auditPath})

	if _, err := r.CallTool("test_destructive_tool", map[string]interface{}{"confirm": true}); err == nil {
		t.Fatal("expected an error with the gate closed")
	}

	data, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("expected a denied call to still be audited: %v", err)
	}
	var entry auditEntry
	if err := json.Unmarshal(data[:len(data)-1], &entry); err != nil {
		t.Fatalf("audit log line is not valid JSON: %v (%s)", err, data)
	}
	if entry.Allowed || entry.GateOpen || entry.Error == "" {
		t.Fatalf("denied audit entry has unexpected shape: %+v", entry)
	}
}

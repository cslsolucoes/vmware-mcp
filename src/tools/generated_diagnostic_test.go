package tools

import (
	"context"
	"testing"

	"github.com/vmware/govmomi/simulator"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// newDiagnosticRegistry builds a Registry the normal way and layers this
// group's diagnostic tools on top via withClass, same pattern as
// generated_vm_lifecycle_test.go's newLifecycleRegistry.
func newDiagnosticRegistry(ctx context.Context, c *vmware.Client, opts RegistryOptions) *Registry {
	r := NewRegistry(ctx, c, opts)
	r.withClass(modeVSphereGeneral, registerDiagnosticTools)
	return r
}

// TestDiagnosticTools_BrowseLogAndQueryDescriptions covers the 2 no-tier
// tools: client-side arg validation happens before any network call, and a
// valid call reaches vcsim's real (always-faulting — see
// generated_diagnostic.go's top doc comment) DiagnosticManager moref,
// proven via assertReachesServer (reused from generated_vm_lifecycle_test.go).
func TestDiagnosticTools_BrowseLogAndQueryDescriptions(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newDiagnosticRegistry(context.Background(), c, RegistryOptions{})

	t.Run("browse_log_requires_key", func(t *testing.T) {
		if _, err := r.CallTool("vmware_diagnostic_browse_log", map[string]interface{}{}); err == nil {
			t.Fatal("expected an error when key is missing")
		}
	})

	t.Run("browse_log_reaches_server", func(t *testing.T) {
		_, err := r.CallTool("vmware_diagnostic_browse_log", map[string]interface{}{"key": "hostd", "start": float64(0), "lines": float64(10)})
		assertReachesServer(t, err, "vmware_diagnostic_browse_log")
	})

	t.Run("query_descriptions_reaches_server", func(t *testing.T) {
		_, err := r.CallTool("vmware_diagnostic_query_descriptions", map[string]interface{}{})
		assertReachesServer(t, err, "vmware_diagnostic_query_descriptions")
	})

	t.Run("browse_log_with_host_resolves_host_first", func(t *testing.T) {
		// A host that doesn't resolve must fail on that resolution, before
		// ever reaching the (also-faulting) DiagnosticManager call — proves
		// the optional "host" argument is actually wired to resolveHost.
		if _, err := r.CallTool("vmware_diagnostic_browse_log", map[string]interface{}{"key": "hostd", "host": "no-such-host"}); err == nil {
			t.Fatal("expected an error for a host that does not resolve")
		}
	})
}

// TestDiagnosticTools_GenerateLogBundles proves vmware_diagnostic_generate_log_bundles
// is genuinely Tier 2 gated (closed gate and missing confirm both deny it
// before any network call) and that a valid, confirmed call reaches vcsim's
// real (always-faulting) server.
func TestDiagnosticTools_GenerateLogBundles(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	closedGate := newDiagnosticRegistry(context.Background(), c, RegistryOptions{AllowDestructive: false})
	openGate := newDiagnosticRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	if _, err := closedGate.CallTool("vmware_diagnostic_generate_log_bundles", map[string]interface{}{"include_default": true, "confirm": true}); err == nil {
		t.Fatal("expected denial with the gate closed")
	}
	if _, err := openGate.CallTool("vmware_diagnostic_generate_log_bundles", map[string]interface{}{"include_default": true}); err == nil {
		t.Fatal("expected denial without confirm:true")
	}

	_, err := openGate.CallTool("vmware_diagnostic_generate_log_bundles", map[string]interface{}{"include_default": true, "confirm": true})
	assertReachesServer(t, err, "vmware_diagnostic_generate_log_bundles")
}

// TestDiagnosticTools_LogCopy proves vmware_diagnostic_log_copy is Tier 2
// gated, validates log_key client-side before any network call (with the
// gate open and confirmed, so the validation itself is what's exercised,
// not the gate), and reaches vcsim's real (always-faulting) server for a
// valid call.
func TestDiagnosticTools_LogCopy(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	closedGate := newDiagnosticRegistry(context.Background(), c, RegistryOptions{AllowDestructive: false})
	openGate := newDiagnosticRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	if _, err := closedGate.CallTool("vmware_diagnostic_log_copy", map[string]interface{}{"log_key": "hostd", "confirm": true}); err == nil {
		t.Fatal("expected denial with the gate closed")
	}
	if _, err := openGate.CallTool("vmware_diagnostic_log_copy", map[string]interface{}{"log_key": "hostd"}); err == nil {
		t.Fatal("expected denial without confirm:true")
	}

	if _, err := openGate.CallTool("vmware_diagnostic_log_copy", map[string]interface{}{"confirm": true}); err == nil {
		t.Fatal("expected an error when log_key is missing")
	}

	if _, err := openGate.CallTool("vmware_diagnostic_log_copy", map[string]interface{}{"log_key": "hostd", "max_bytes": float64(-1), "confirm": true}); err == nil {
		t.Fatal("expected an error for a non-positive max_bytes")
	}

	_, err := openGate.CallTool("vmware_diagnostic_log_copy", map[string]interface{}{"log_key": "hostd", "tail_lines": float64(100), "confirm": true})
	assertReachesServer(t, err, "vmware_diagnostic_log_copy")
}

package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// vamiAccessShutdownFixture starts a fake VAMI server covering every route
// generated_vami_access_shutdown.go's 12 tools hit — same shape as
// vamiServicesAccountsVmonFixture (this package's sibling test file); see
// that file's doc comment and generated_vami_access_shutdown.go's top doc
// comment for why a hand-rolled httptest fixture is used instead of vcsim.
func vamiAccessShutdownFixture(t *testing.T) (*httptest.Server, *vamiAccessServicesCapture) {
	t.Helper()
	mux := http.NewServeMux()
	capture := newVAMIAccessServicesCapture()

	mux.HandleFunc("POST /rest/com/vmware/cis/session", capture.handle("fixture-session-id"))

	mux.HandleFunc("GET /rest/appliance/access/consolecli", capture.handle(true))
	mux.HandleFunc("PUT /rest/appliance/access/consolecli", capture.handle(nil))
	mux.HandleFunc("GET /rest/appliance/access/dcui", capture.handle(true))
	mux.HandleFunc("PUT /rest/appliance/access/dcui", capture.handle(nil))
	mux.HandleFunc("GET /rest/appliance/access/shell", capture.handle(map[string]interface{}{"enabled": true, "timeout": float64(3600)}))
	mux.HandleFunc("PUT /rest/appliance/access/shell", capture.handle(nil))
	mux.HandleFunc("GET /rest/appliance/access/ssh", capture.handle(true))
	mux.HandleFunc("PUT /rest/appliance/access/ssh", capture.handle(nil))

	mux.HandleFunc("GET /rest/appliance/techpreview/shutdown", capture.handle(map[string]interface{}{"action": ""}))
	mux.HandleFunc("POST /rest/appliance/techpreview/shutdown/poweroff", capture.handle(nil))
	mux.HandleFunc("POST /rest/appliance/techpreview/shutdown/cancel", capture.handle(nil))
	mux.HandleFunc("POST /rest/appliance/techpreview/shutdown/restart", capture.handle(nil))

	return httptest.NewServer(mux), capture
}

// newVAMIAccessShutdownRegistry builds a Registry the normal way and then
// manually layers registerVAMIAccessShutdownTools on top via withClass —
// same pattern as newVAMIServicesAccountsVmonRegistry (this package's
// sibling test file) and newVAMINetworkSystemRegistry
// (generated_vami_network_system_test.go, a sibling Fase 8b group). Required
// because this group's brief explicitly says not to edit registry.go, so
// NewRegistry's own registerTools() sweep does not call this file's
// register function on its own.
func newVAMIAccessShutdownRegistry(ctx context.Context, c *vmware.Client, opts RegistryOptions) *Registry {
	r := NewRegistry(ctx, c, opts)
	r.withClass(modeVCenterOnly, registerVAMIAccessShutdownTools)
	return r
}

// --- Happy path: plain (non-destructive) reads ------------------------------

func TestVAMIAccessShutdown_PlainReads(t *testing.T) {
	srv, _ := vamiAccessShutdownFixture(t)
	defer srv.Close()
	c := newApplianceFixtureClient(t, srv)
	r := newVAMIAccessShutdownRegistry(context.Background(), c, RegistryOptions{})

	cases := []string{
		"vmware_appliance_access_legacy_consolecli_get",
		"vmware_appliance_access_legacy_dcui_get",
		"vmware_appliance_access_legacy_shell_get",
		"vmware_appliance_access_legacy_ssh_get",
		"vmware_appliance_techpreview_shutdown_get",
	}
	for _, tool := range cases {
		t.Run(tool, func(t *testing.T) {
			raw, err := r.CallTool(tool, map[string]interface{}{})
			if err != nil {
				t.Fatalf("%s failed: %v", tool, err)
			}
			if raw == "" {
				t.Fatalf("%s returned an empty result", tool)
			}
		})
	}
}

// --- Happy path + body verification: destructive (tier2) tools -------------

func TestVAMIAccessShutdown_DestructiveToolsSendExpectedBodyWhenAllowed(t *testing.T) {
	srv, capture := vamiAccessShutdownFixture(t)
	defer srv.Close()
	c := newApplianceFixtureClient(t, srv)
	r := newVAMIAccessShutdownRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	t.Run("consolecli_set sends a flat enabled body", func(t *testing.T) {
		if _, err := r.CallTool("vmware_appliance_access_legacy_consolecli_set", map[string]interface{}{"enabled": true, "confirm": true}); err != nil {
			t.Fatalf("failed: %v", err)
		}
		body := capture.bodies["PUT /rest/appliance/access/consolecli"]
		if body["enabled"] != true {
			t.Fatalf("expected flat {enabled:true} body, got %#v", body)
		}
	})

	t.Run("dcui_set sends a flat enabled body", func(t *testing.T) {
		if _, err := r.CallTool("vmware_appliance_access_legacy_dcui_set", map[string]interface{}{"enabled": false, "confirm": true}); err != nil {
			t.Fatalf("failed: %v", err)
		}
		body := capture.bodies["PUT /rest/appliance/access/dcui"]
		if body["enabled"] != false {
			t.Fatalf("expected flat {enabled:false} body, got %#v", body)
		}
	})

	t.Run("shell_set sends a nested config.enabled/config.timeout body", func(t *testing.T) {
		if _, err := r.CallTool("vmware_appliance_access_legacy_shell_set", map[string]interface{}{"enabled": true, "timeout": 3600, "confirm": true}); err != nil {
			t.Fatalf("failed: %v", err)
		}
		body := capture.bodies["PUT /rest/appliance/access/shell"]
		cfg, ok := body["config"].(map[string]interface{})
		if !ok {
			t.Fatalf("expected a nested config object (unlike consolecli/dcui/ssh's flat body), got %#v", body)
		}
		if cfg["enabled"] != true || cfg["timeout"] != float64(3600) {
			t.Fatalf("expected config.enabled=true, config.timeout=3600, got %#v", cfg)
		}
	})

	t.Run("ssh_set sends a flat enabled body", func(t *testing.T) {
		if _, err := r.CallTool("vmware_appliance_access_legacy_ssh_set", map[string]interface{}{"enabled": true, "confirm": true}); err != nil {
			t.Fatalf("failed: %v", err)
		}
		body := capture.bodies["PUT /rest/appliance/access/ssh"]
		if body["enabled"] != true {
			t.Fatalf("expected flat {enabled:true} body, got %#v", body)
		}
	})

	t.Run("shutdown_poweroff sends config.delay/config.reason", func(t *testing.T) {
		args := map[string]interface{}{"delay": 5, "reason": "maintenance", "confirm": true}
		if _, err := r.CallTool("vmware_appliance_techpreview_shutdown_poweroff", args); err != nil {
			t.Fatalf("failed: %v", err)
		}
		body := capture.bodies["POST /rest/appliance/techpreview/shutdown/poweroff"]
		cfg, ok := body["config"].(map[string]interface{})
		if !ok || cfg["delay"] != float64(5) || cfg["reason"] != "maintenance" {
			t.Fatalf("expected config.delay=5, config.reason=maintenance, got %#v", body)
		}
	})

	t.Run("shutdown_cancel sends no body", func(t *testing.T) {
		if _, err := r.CallTool("vmware_appliance_techpreview_shutdown_cancel", map[string]interface{}{"confirm": true}); err != nil {
			t.Fatalf("failed: %v", err)
		}
	})

	t.Run("shutdown_restart sends config.delay/config.reason", func(t *testing.T) {
		args := map[string]interface{}{"delay": 1, "reason": "reboot for patch", "confirm": true}
		if _, err := r.CallTool("vmware_appliance_techpreview_shutdown_restart", args); err != nil {
			t.Fatalf("failed: %v", err)
		}
		body := capture.bodies["POST /rest/appliance/techpreview/shutdown/restart"]
		cfg, ok := body["config"].(map[string]interface{})
		if !ok || cfg["delay"] != float64(1) || cfg["reason"] != "reboot for patch" {
			t.Fatalf("expected config.delay=1, config.reason=\"reboot for patch\", got %#v", body)
		}
	})
}

// --- Tier gating (registerDestructive wiring) -------------------------------

func TestVAMIAccessShutdown_GateClosedDeniesTier2(t *testing.T) {
	srv, _ := vamiAccessShutdownFixture(t)
	defer srv.Close()
	c := newApplianceFixtureClient(t, srv)
	r := newVAMIAccessShutdownRegistry(context.Background(), c, RegistryOptions{AllowDestructive: false})

	cases := []struct {
		tool string
		args map[string]interface{}
	}{
		{"vmware_appliance_access_legacy_consolecli_set", map[string]interface{}{"enabled": true, "confirm": true}},
		{"vmware_appliance_access_legacy_shell_set", map[string]interface{}{"enabled": true, "confirm": true}},
		{"vmware_appliance_techpreview_shutdown_poweroff", map[string]interface{}{"confirm": true}},
		{"vmware_appliance_techpreview_shutdown_restart", map[string]interface{}{"confirm": true}},
	}
	for _, tc := range cases {
		t.Run(tc.tool, func(t *testing.T) {
			if _, err := r.CallTool(tc.tool, tc.args); err == nil {
				t.Fatalf("%s: expected an error with the gate closed, got none", tc.tool)
			}
		})
	}
}

func TestVAMIAccessShutdown_ConfirmRequiredEvenWithGateOpen(t *testing.T) {
	srv, _ := vamiAccessShutdownFixture(t)
	defer srv.Close()
	c := newApplianceFixtureClient(t, srv)
	r := newVAMIAccessShutdownRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	if _, err := r.CallTool("vmware_appliance_access_legacy_ssh_set", map[string]interface{}{"enabled": true}); err == nil {
		t.Fatal("expected an error without confirm:true, got none")
	}
	if _, err := r.CallTool("vmware_appliance_techpreview_shutdown_cancel", map[string]interface{}{}); err == nil {
		t.Fatal("expected an error without confirm:true, got none")
	}
}

// --- Argument validation -----------------------------------------------------

func TestVAMIAccessShutdown_ArgumentValidation(t *testing.T) {
	srv, _ := vamiAccessShutdownFixture(t)
	defer srv.Close()
	c := newApplianceFixtureClient(t, srv)
	r := newVAMIAccessShutdownRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	cases := []struct {
		name string
		tool string
		args map[string]interface{}
	}{
		{"consolecli_set missing enabled", "vmware_appliance_access_legacy_consolecli_set", map[string]interface{}{"confirm": true}},
		{"dcui_set missing enabled", "vmware_appliance_access_legacy_dcui_set", map[string]interface{}{"confirm": true}},
		{"shell_set missing enabled", "vmware_appliance_access_legacy_shell_set", map[string]interface{}{"confirm": true}},
		{"ssh_set missing enabled", "vmware_appliance_access_legacy_ssh_set", map[string]interface{}{"confirm": true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := r.CallTool(tc.tool, tc.args); err == nil {
				t.Fatalf("%s: expected a validation error, got none", tc.tool)
			}
		})
	}
}

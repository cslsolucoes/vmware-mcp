package tools

import (
	"context"
	"crypto/tls"
	"testing"

	"github.com/vmware/govmomi/simulator"
	_ "github.com/vmware/govmomi/vapi/simulator" // registers the vAPI/REST endpoint (tags, content library, vcenter vmtx/ovf) — see newSimClient's RegisterEndpoints note below

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// newSimClient starts an in-process vcsim server from model and returns a
// real *vmware.Client connected to it — through our own NewClient, so the
// login/keepalive path is exercised too, not bypassed. Every domain test
// file (vm_test.go, inventory_test.go, host_test.go, ...) uses this as its
// entry point and calls tools through the Registry (NewRegistry(...).
// CallTool(name, args)), never the handler functions directly.
func newSimClient(t *testing.T, model *simulator.Model) (*vmware.Client, func()) {
	t.Helper()

	if model.Service == nil {
		if err := model.Create(); err != nil {
			t.Fatalf("failed to create simulator model: %v", err)
		}
	}
	model.Service.TLS = new(tls.Config)
	// RegisterEndpoints=true wires up every endpoint registered via
	// simulator.RegisterEndpoint's init()-time hook — critically the
	// vapi/simulator package blank-imported above, which serves the vAPI/
	// REST domains (tags, content library, vcenter vmtx/ovf) that
	// object/*.go's SOAP-only tests never needed. Added 2026-08-12 during
	// Fase 8a's pre-generation spike after a first attempt against a plain
	// NewServer() (no RegisterEndpoints) 404'd on POST .../cis/session —
	// simulator.Model.Run() sets this internally, but this project's own
	// newSimClient never went through Model.Run. Harmless for every
	// pre-Fase-8a test that never issues a REST call.
	model.Service.RegisterEndpoints = true
	s := model.Service.NewServer()

	// s.URL comes back with vcsim's default credentials embedded
	// ("user:pass@host:port") — passed as-is, govmomi.NewClient's own
	// "auto-login if the URL carries userinfo" behavior (see
	// vmware/client.go's NewClient doc comment) logs in BEFORE our code
	// attaches the keepalive round-tripper or calls Login itself, and the
	// second, explicit Login then fails ("already have a session"). Strip
	// the userinfo so our own connect->wrap->login sequence is what
	// actually runs against the simulator, same as it will against a real
	// vCenter/ESXi endpoint.
	u := *s.URL
	u.User = nil

	c, err := vmware.NewClient(context.Background(), vmware.Config{
		URL:      u.String(),
		Username: "user",
		Password: "pass",
		Insecure: true,
	})
	if err != nil {
		s.Close()
		model.Remove()
		t.Fatalf("failed to connect to simulator: %v", err)
	}

	cleanup := func() {
		_ = c.Close(context.Background())
		s.Close()
		model.Remove()
	}
	return c, cleanup
}

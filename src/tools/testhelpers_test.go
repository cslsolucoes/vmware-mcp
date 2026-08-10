package tools

import (
	"context"
	"crypto/tls"
	"testing"

	"github.com/vmware/govmomi/simulator"

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
	s := model.Service.NewServer()

	c, err := vmware.NewClient(context.Background(), vmware.Config{
		URL:      s.URL.String(),
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

package tools

import (
	"context"
	"testing"

	"github.com/vmware/govmomi/simulator"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// newAuthenticationRegistry builds a Registry the normal way (NewRegistry,
// which wires every other domain via registerTools) and then manually layers
// registerAuthenticationTools on top via withClass — same pattern as
// newLibraryCoreRegistry (generated_library_core_test.go). This file must not
// edit registry.go itself (see generated_authentication.go's top doc
// comment).
func newAuthenticationRegistry(ctx context.Context, c *vmware.Client, opts RegistryOptions) *Registry {
	r := NewRegistry(ctx, c, opts)
	r.withClass(modeVCenterOnly, registerAuthenticationTools)
	return r
}

func validTokenIssueArgs() map[string]interface{} {
	return map[string]interface{}{
		"token": map[string]interface{}{
			"subject_token":      "some-subject-token",
			"subject_token_type": "urn:ietf:params:oauth:token-type:access_token",
			"grant_type":         "urn:ietf:params:oauth:grant-type:token-exchange",
		},
		"confirm": true,
	}
}

// TestAuthenticationTools_Registration proves the single tool in this file
// is registered and reachable via ListTools.
func TestAuthenticationTools_Registration(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newAuthenticationRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	got := map[string]bool{}
	for _, tl := range r.ListTools() {
		got[tl.Name] = true
	}
	if !got["vmware_authentication_issue"] {
		t.Error("tool vmware_authentication_issue not registered")
	}
}

// TestAuthenticationTools_ArgValidation proves each required TokenIssueSpec
// field (subject_token, subject_token_type, grant_type) is checked before
// any network call, gate open + confirmed so only the argument-validation
// path is being exercised.
func TestAuthenticationTools_ArgValidation(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newAuthenticationRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	if _, err := r.CallTool("vmware_authentication_issue", map[string]interface{}{"confirm": true}); err == nil {
		t.Error("expected an error when token is missing entirely")
	}

	base := func(overrides map[string]interface{}) map[string]interface{} {
		token := map[string]interface{}{
			"subject_token":      "some-subject-token",
			"subject_token_type": "urn:ietf:params:oauth:token-type:access_token",
			"grant_type":         "urn:ietf:params:oauth:grant-type:token-exchange",
		}
		for k := range overrides {
			delete(token, k)
		}
		return map[string]interface{}{"token": token, "confirm": true}
	}

	for _, missing := range []string{"subject_token", "subject_token_type", "grant_type"} {
		args := base(map[string]interface{}{missing: nil})
		if _, err := r.CallTool("vmware_authentication_issue", args); err == nil {
			t.Errorf("expected an error when token.%s is missing", missing)
		}
	}
}

// TestAuthenticationTools_GateAndConfirm proves vmware_authentication_issue
// is wired through registerDestructive (Tier 2, per this file's top doc
// comment correction from the classifier's read-only default) — same
// 3-layer protection check pattern as generated_network_test.go's
// TestNetworkTools_GateAndConfirm.
func TestAuthenticationTools_GateAndConfirm(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	closedGate := newAuthenticationRegistry(context.Background(), c, RegistryOptions{AllowDestructive: false})
	if _, err := closedGate.CallTool("vmware_authentication_issue", validTokenIssueArgs()); err == nil {
		t.Fatal("expected vmware_authentication_issue to be denied with the gate closed")
	}

	openGate := newAuthenticationRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	noConfirm := validTokenIssueArgs()
	delete(noConfirm, "confirm")
	if _, err := openGate.CallTool("vmware_authentication_issue", noConfirm); err == nil {
		t.Fatal("expected vmware_authentication_issue to fail without confirm:true")
	}
}

// TestAuthenticationTools_ReachesServer proves vmware_authentication_issue
// reaches real vcsim instead of failing on something wired wrong here — see
// generated_authentication.go's top doc comment: no vcsim handler exists for
// /vcenter/tokenservice/token-exchange ("vcsim gap, not a bug", confirmed by
// grep against referencia/govmomi/vapi/simulator/simulator.go). err is
// expected to be non-nil; assertReachesServer (generated_vm_lifecycle_test.go)
// proves it's a real server-side/HTTP fault, not "unknown tool" or a
// recovered panic.
func TestAuthenticationTools_ReachesServer(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newAuthenticationRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	_, err := r.CallTool("vmware_authentication_issue", validTokenIssueArgs())
	assertReachesServer(t, err, "vmware_authentication_issue")
}

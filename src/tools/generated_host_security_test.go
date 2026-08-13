package tools

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"strings"
	"testing"
	"time"

	"encoding/pem"

	"github.com/vmware/govmomi/simulator"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// newHostSecurityRegistry builds a Registry the normal way (NewRegistry,
// which wires host.go/generated_option.go/etc via registerTools) and then
// manually layers this group's tools on top via withClass, exactly as
// registry.go's real wiring for registerHostSecurityTools will do once
// another change adds it there — this file must not edit registry.go itself
// (see generated_host_security.go's top doc comment / the task's Hard
// requirements). Same pattern as generated_vm_lifecycle_test.go's
// newLifecycleRegistry.
func newHostSecurityRegistry(ctx context.Context, c *vmware.Client, opts RegistryOptions) *Registry {
	r := NewRegistry(ctx, c, opts)
	r.withClass(modeVSphereGeneral, registerHostSecurityTools)
	return r
}

// newSelfSignedCertPEM generates a fresh, valid, self-signed X.509
// certificate PEM string for exercising
// vmware_host_certificate_install_server_certificate against a real vcsim
// round trip — mirrors exactly what
// referencia/govmomi/simulator/host_certificate_manager.go's own
// InstallServerCertificate/GenerateCertificateSigningRequest do internally
// (rsa.GenerateKey + x509.CreateCertificate), so it is guaranteed parseable
// by the simulator's HostCertificateInfo.FromPEM.
func newSelfSignedCertPEM(t *testing.T, commonName string) string {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate test key: %v", err)
	}

	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{CommonName: commonName},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		IsCA:                  true,
		BasicConstraintsValid: true,
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("failed to create test certificate: %v", err)
	}

	var buf strings.Builder
	if err := pem.Encode(&buf, &pem.Block{Type: "CERTIFICATE", Bytes: der}); err != nil {
		t.Fatalf("failed to PEM-encode test certificate: %v", err)
	}
	return buf.String()
}

// TestHostSecurityTools_CertificateInfoAndCSR smokes the read-only
// certificate_info tool and the (Tier 2) CSR-generation tools, which vcsim
// genuinely implements server-side (see generated_host_security.go's top
// doc comment) — both succeed for real here, proving resolveHost -> config
// manager -> real SOAP call plumbing end to end, plus gate/confirm
// enforcement on the CSR tools.
func TestHostSecurityTools_CertificateInfoAndCSR(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newHostSecurityRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	host := firstHostPath(t, r)

	t.Run("certificate_info", func(t *testing.T) {
		raw, err := r.CallTool("vmware_host_certificate_info", map[string]interface{}{"host": host})
		if err != nil {
			t.Fatalf("vmware_host_certificate_info failed: %v", err)
		}
		m := decodeResult(t, raw)
		info, ok := m["certificate_info"].(map[string]interface{})
		if !ok {
			t.Fatalf("expected a \"certificate_info\" object: %s", raw)
		}
		// NewHostCertificateManager seeds the manager with the host's initial
		// Config.Certificate at simulator startup (confirmed by reading
		// referencia/govmomi/simulator/host_certificate_manager.go) — status
		// should be a real, non-empty value, not a zero/empty struct.
		if status, _ := info["status"].(string); status == "" {
			t.Fatalf("expected a non-empty certificate status, got %v: %s", info["status"], raw)
		}
	})

	t.Run("generate_csr_gate_and_confirm", func(t *testing.T) {
		closedGate := newHostSecurityRegistry(context.Background(), c, RegistryOptions{AllowDestructive: false})
		if _, err := closedGate.CallTool("vmware_host_certificate_generate_csr", map[string]interface{}{"host": host, "confirm": true}); err == nil {
			t.Fatal("expected vmware_host_certificate_generate_csr to be denied with the gate closed")
		}
		if _, err := r.CallTool("vmware_host_certificate_generate_csr", map[string]interface{}{"host": host}); err == nil {
			t.Fatal("expected vmware_host_certificate_generate_csr to fail without confirm:true")
		}
	})

	t.Run("generate_csr_succeeds", func(t *testing.T) {
		raw, err := r.CallTool("vmware_host_certificate_generate_csr", map[string]interface{}{"host": host, "confirm": true})
		if err != nil {
			t.Fatalf("vmware_host_certificate_generate_csr failed: %v", err)
		}
		m := decodeResult(t, raw)
		csr, _ := m["csr"].(string)
		if !strings.Contains(csr, "CERTIFICATE REQUEST") {
			t.Fatalf("expected a PEM-encoded CSR, got: %s", raw)
		}
	})

	t.Run("generate_csr_use_ip_as_cn", func(t *testing.T) {
		raw, err := r.CallTool("vmware_host_certificate_generate_csr", map[string]interface{}{"host": host, "use_ip_address_as_common_name": true, "confirm": true})
		if err != nil {
			t.Fatalf("vmware_host_certificate_generate_csr(use_ip_address_as_common_name) failed: %v", err)
		}
		if csr, _ := decodeResult(t, raw)["csr"].(string); !strings.Contains(csr, "CERTIFICATE REQUEST") {
			t.Fatalf("expected a PEM-encoded CSR, got: %s", raw)
		}
	})
}

// TestHostSecurityTools_InstallServerCertificate proves the tool actually
// changes the host's observable certificate state (via
// vmware_host_certificate_info), the required non-trivial-action proof for
// this group, plus gate/confirm enforcement leaving the cert unchanged when
// denied.
func TestHostSecurityTools_InstallServerCertificate(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newHostSecurityRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	host := firstHostPath(t, r)

	before := decodeResult(t, mustCallTool(t, r, "vmware_host_certificate_info", map[string]interface{}{"host": host}))["certificate_info"].(map[string]interface{})

	newCert := newSelfSignedCertPEM(t, "mcpvmware-test-cert")

	t.Run("gate_and_confirm", func(t *testing.T) {
		closedGate := newHostSecurityRegistry(context.Background(), c, RegistryOptions{AllowDestructive: false})
		if _, err := closedGate.CallTool("vmware_host_certificate_install_server_certificate", map[string]interface{}{"host": host, "cert": newCert, "confirm": true}); err == nil {
			t.Fatal("expected vmware_host_certificate_install_server_certificate to be denied with the gate closed")
		}
		if _, err := r.CallTool("vmware_host_certificate_install_server_certificate", map[string]interface{}{"host": host, "cert": newCert}); err == nil {
			t.Fatal("expected vmware_host_certificate_install_server_certificate to fail without confirm:true")
		}
		after := decodeResult(t, mustCallTool(t, r, "vmware_host_certificate_info", map[string]interface{}{"host": host}))["certificate_info"].(map[string]interface{})
		if after["subject"] != before["subject"] {
			t.Fatalf("certificate changed despite gate/confirm denial: before=%v after=%v", before["subject"], after["subject"])
		}
	})

	t.Run("missing_cert", func(t *testing.T) {
		if _, err := r.CallTool("vmware_host_certificate_install_server_certificate", map[string]interface{}{"host": host, "confirm": true}); err == nil {
			t.Fatal("expected an error when cert is missing")
		}
	})

	t.Run("install_changes_certificate_info", func(t *testing.T) {
		if _, err := r.CallTool("vmware_host_certificate_install_server_certificate", map[string]interface{}{"host": host, "cert": newCert, "confirm": true}); err != nil {
			t.Fatalf("vmware_host_certificate_install_server_certificate failed: %v", err)
		}
		after := decodeResult(t, mustCallTool(t, r, "vmware_host_certificate_info", map[string]interface{}{"host": host}))["certificate_info"].(map[string]interface{})
		if after["subject"] == before["subject"] {
			t.Fatalf("expected certificate_info.subject to change after installing a new certificate (before=%v, still %v after)", before["subject"], after["subject"])
		}
		if !strings.Contains(after["subject"].(string), "mcpvmware-test-cert") {
			t.Fatalf("expected the new certificate's CN in the subject, got %v", after["subject"])
		}
	})
}

// TestHostSecurityTools_UnsimulatedCertificateMethods covers the 4
// HostCertificateManager methods vcsim's simulator.HostCertificateManager
// has no server-side implementation for (see generated_host_security.go's
// top doc comment): each is proven registered, rejects bad/missing input
// before any network call, and — given valid input, gate open, confirm:true
// where applicable — reaches the real vcsim server and gets back a clean
// types.MethodNotFound-based error, not a wiring failure. Same proof
// pattern as generated_vm_lifecycle_test.go's assertReachesServer /
// TestVMLifecycleTools_UnsimulatedMethods.
func TestHostSecurityTools_UnsimulatedCertificateMethods(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newHostSecurityRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	host := firstHostPath(t, r)

	t.Run("generate_csr_by_dn", func(t *testing.T) {
		if _, err := r.CallTool("vmware_host_certificate_generate_csr_by_dn", map[string]interface{}{"host": host, "confirm": true}); err == nil {
			t.Fatal("expected an error when distinguished_name is missing")
		}
		_, err := r.CallTool("vmware_host_certificate_generate_csr_by_dn", map[string]interface{}{"host": host, "distinguished_name": "CN=esxi-01.local", "confirm": true})
		assertReachesServer(t, err, "vmware_host_certificate_generate_csr_by_dn")
	})

	t.Run("list_ca_crls", func(t *testing.T) {
		_, err := r.CallTool("vmware_host_certificate_list_ca_crls", map[string]interface{}{"host": host})
		assertReachesServer(t, err, "vmware_host_certificate_list_ca_crls")
	})

	t.Run("list_ca_certificates", func(t *testing.T) {
		_, err := r.CallTool("vmware_host_certificate_list_ca_certificates", map[string]interface{}{"host": host})
		assertReachesServer(t, err, "vmware_host_certificate_list_ca_certificates")
	})

	t.Run("replace_ca_certs_and_crls", func(t *testing.T) {
		closedGate := newHostSecurityRegistry(context.Background(), c, RegistryOptions{AllowDestructive: false})
		if _, err := closedGate.CallTool("vmware_host_certificate_replace_ca_certs_and_crls", map[string]interface{}{"host": host, "confirm": true}); err == nil {
			t.Fatal("expected vmware_host_certificate_replace_ca_certs_and_crls to be denied with the gate closed")
		}
		if _, err := r.CallTool("vmware_host_certificate_replace_ca_certs_and_crls", map[string]interface{}{"host": host}); err == nil {
			t.Fatal("expected vmware_host_certificate_replace_ca_certs_and_crls to fail without confirm:true")
		}
		_, err := r.CallTool("vmware_host_certificate_replace_ca_certs_and_crls", map[string]interface{}{
			"host":            host,
			"ca_certificates": []interface{}{newSelfSignedCertPEM(t, "mcpvmware-test-ca")},
			"confirm":         true,
		})
		assertReachesServer(t, err, "vmware_host_certificate_replace_ca_certs_and_crls")
	})
}

// TestHostSecurityTools_FirewallInfoAndRulesets proves vmware_host_firewall_info
// lists real rulesets from vcsim's esx.HostFirewallInfo fixture, and that
// vmware_host_firewall_disable_ruleset / vmware_host_firewall_enable_ruleset
// actually flip a ruleset's enabled state end to end (this group's
// non-trivial-action state-change proof for the firewall manager), plus
// gate/confirm enforcement and a bad-id error path.
func TestHostSecurityTools_FirewallInfoAndRulesets(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newHostSecurityRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	host := firstHostPath(t, r)

	// firewallRulesetEnabled reads the current "enabled" state of ruleset id
	// via vmware_host_firewall_info, failing the test if id isn't present at
	// all (a wiring problem, not a legitimate "not found").
	firewallRulesetEnabled := func(t *testing.T, id string) bool {
		t.Helper()
		raw, err := r.CallTool("vmware_host_firewall_info", map[string]interface{}{"host": host})
		if err != nil {
			t.Fatalf("vmware_host_firewall_info failed: %v", err)
		}
		info, ok := decodeResult(t, raw)["firewall_info"].(map[string]interface{})
		if !ok {
			t.Fatalf("expected a \"firewall_info\" object: %s", raw)
		}
		rulesets, _ := info["ruleset"].([]interface{})
		for _, rs := range rulesets {
			m, ok := rs.(map[string]interface{})
			if !ok {
				continue
			}
			if m["key"] == id {
				enabled, _ := m["enabled"].(bool)
				return enabled
			}
		}
		t.Fatalf("ruleset %q not found in vmware_host_firewall_info result: %s", id, raw)
		return false
	}

	t.Run("info_lists_known_ruleset", func(t *testing.T) {
		raw, err := r.CallTool("vmware_host_firewall_info", map[string]interface{}{"host": host})
		if err != nil {
			t.Fatalf("vmware_host_firewall_info failed: %v", err)
		}
		info, ok := decodeResult(t, raw)["firewall_info"].(map[string]interface{})
		if !ok {
			t.Fatalf("expected a \"firewall_info\" object: %s", raw)
		}
		rulesets, _ := info["ruleset"].([]interface{})
		if len(rulesets) == 0 {
			t.Fatalf("expected at least 1 firewall ruleset from esx.HostFirewallInfo, got 0: %s", raw)
		}
	})

	// CIMHttpServer is one of the rulesets defined in
	// referencia/govmomi/simulator/esx/host_firewall_system.go's fixture data
	// (confirmed by reading it, not assumed).
	const rulesetID = "CIMHttpServer"

	t.Run("disable_enable_gate_and_confirm", func(t *testing.T) {
		before := firewallRulesetEnabled(t, rulesetID)

		closedGate := newHostSecurityRegistry(context.Background(), c, RegistryOptions{AllowDestructive: false})
		if _, err := closedGate.CallTool("vmware_host_firewall_disable_ruleset", map[string]interface{}{"host": host, "id": rulesetID, "confirm": true}); err == nil {
			t.Fatal("expected vmware_host_firewall_disable_ruleset to be denied with the gate closed")
		}
		if _, err := r.CallTool("vmware_host_firewall_disable_ruleset", map[string]interface{}{"host": host, "id": rulesetID}); err == nil {
			t.Fatal("expected vmware_host_firewall_disable_ruleset to fail without confirm:true")
		}
		if got := firewallRulesetEnabled(t, rulesetID); got != before {
			t.Fatalf("ruleset %q enabled state changed despite gate/confirm denial: before=%v after=%v", rulesetID, before, got)
		}
	})

	t.Run("disable_then_enable_round_trip", func(t *testing.T) {
		if _, err := r.CallTool("vmware_host_firewall_disable_ruleset", map[string]interface{}{"host": host, "id": rulesetID, "confirm": true}); err != nil {
			t.Fatalf("vmware_host_firewall_disable_ruleset failed: %v", err)
		}
		if got := firewallRulesetEnabled(t, rulesetID); got != false {
			t.Fatalf("expected ruleset %q enabled=false after disabling, got %v", rulesetID, got)
		}

		if _, err := r.CallTool("vmware_host_firewall_enable_ruleset", map[string]interface{}{"host": host, "id": rulesetID, "confirm": true}); err != nil {
			t.Fatalf("vmware_host_firewall_enable_ruleset failed: %v", err)
		}
		if got := firewallRulesetEnabled(t, rulesetID); got != true {
			t.Fatalf("expected ruleset %q enabled=true after re-enabling, got %v", rulesetID, got)
		}
	})

	t.Run("bad_ruleset_id", func(t *testing.T) {
		if _, err := r.CallTool("vmware_host_firewall_disable_ruleset", map[string]interface{}{"host": host, "id": "no-such-ruleset", "confirm": true}); err == nil {
			t.Fatal("expected an error for a ruleset id that does not exist")
		}
	})

	t.Run("missing_id", func(t *testing.T) {
		if _, err := r.CallTool("vmware_host_firewall_enable_ruleset", map[string]interface{}{"host": host, "confirm": true}); err == nil {
			t.Fatal("expected an error when id is missing")
		}
	})
}

// TestHostSecurityTools_FirewallRefreshUnsimulated covers
// HostFirewallSystem.Refresh, which vcsim's simulator.HostFirewallSystem has
// no receiver method for (confirmed by reading
// referencia/govmomi/simulator/host_firewall_system.go) — proven gated and
// reaching the real vcsim server (MethodNotFound), same posture as the
// unsimulated certificate methods above.
func TestHostSecurityTools_FirewallRefreshUnsimulated(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newHostSecurityRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	host := firstHostPath(t, r)

	closedGate := newHostSecurityRegistry(context.Background(), c, RegistryOptions{AllowDestructive: false})
	if _, err := closedGate.CallTool("vmware_host_firewall_refresh", map[string]interface{}{"host": host, "confirm": true}); err == nil {
		t.Fatal("expected vmware_host_firewall_refresh to be denied with the gate closed")
	}
	if _, err := r.CallTool("vmware_host_firewall_refresh", map[string]interface{}{"host": host}); err == nil {
		t.Fatal("expected vmware_host_firewall_refresh to fail without confirm:true")
	}

	_, err := r.CallTool("vmware_host_firewall_refresh", map[string]interface{}{"host": host, "confirm": true})
	assertReachesServer(t, err, "vmware_host_firewall_refresh")
}

// TestHostSecurityTools_AccountManager is this group's mandatory Tier 1
// proof (vmware_host_account_remove) plus real functional coverage of
// vmware_host_account_create/update against simulator.ESX() — CORRECTED
// during this task from the original brief's claim that HostAccountManager
// isn't simulated at all (see generated_host_security.go's top doc comment
// for the full story: it resolves, on ESX()-model vcsim only, to a real
// *HostLocalAccountManager backing object).
func TestHostSecurityTools_AccountManager(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newHostSecurityRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	host := firstHostPath(t, r)
	const userID = "mcpvmware-test-user"

	t.Run("create_gate_and_confirm", func(t *testing.T) {
		closedGate := newHostSecurityRegistry(context.Background(), c, RegistryOptions{AllowDestructive: false})
		user := map[string]interface{}{"id": userID, "password": "P@ssw0rd123!", "description": "mcpvmware test account"}
		if _, err := closedGate.CallTool("vmware_host_account_create", map[string]interface{}{"host": host, "user": user, "confirm": true}); err == nil {
			t.Fatal("expected vmware_host_account_create to be denied with the gate closed")
		}
		if _, err := r.CallTool("vmware_host_account_create", map[string]interface{}{"host": host, "user": user}); err == nil {
			t.Fatal("expected vmware_host_account_create to fail without confirm:true")
		}
	})

	t.Run("create_requires_id", func(t *testing.T) {
		if _, err := r.CallTool("vmware_host_account_create", map[string]interface{}{"host": host, "user": map[string]interface{}{"password": "x"}, "confirm": true}); err == nil {
			t.Fatal("expected an error when user.id is missing")
		}
		if _, err := r.CallTool("vmware_host_account_create", map[string]interface{}{"host": host, "confirm": true}); err == nil {
			t.Fatal("expected an error when user is missing entirely")
		}
	})

	t.Run("create_succeeds_then_duplicate_fails", func(t *testing.T) {
		user := map[string]interface{}{"id": userID, "password": "P@ssw0rd123!", "description": "mcpvmware test account"}
		if _, err := r.CallTool("vmware_host_account_create", map[string]interface{}{"host": host, "user": user, "confirm": true}); err != nil {
			t.Fatalf("vmware_host_account_create failed: %v", err)
		}
		// The real vcsim HostLocalAccountManager.CreateUser (confirmed by
		// reading referencia/govmomi/simulator/host_local_account_manager.go)
		// faults AlreadyExists on a duplicate id — proves this hit the real
		// business logic, not a stub.
		if _, err := r.CallTool("vmware_host_account_create", map[string]interface{}{"host": host, "user": user, "confirm": true}); err == nil {
			t.Fatal("expected a 2nd vmware_host_account_create with the same id to fail (AlreadyExists)")
		} else {
			t.Logf("duplicate create correctly rejected by vcsim: %v", err)
		}
	})

	t.Run("update", func(t *testing.T) {
		// UpdateUser's simulator implementation
		// (referencia/govmomi/simulator/host_local_account_manager.go) has a
		// confirmed vendored bug: it returns &methods.CreateUserBody{Res:
		// &types.CreateUserResponse{}} instead of an UpdateUserBody with an
		// UpdateUserResponse. Documenting exactly what that produces against a
		// real client round trip rather than assuming — this is a vendored
		// referencia/ gap, not something to "fix" here.
		user := map[string]interface{}{"id": userID, "description": "updated by mcpvmware test"}
		_, err := r.CallTool("vmware_host_account_update", map[string]interface{}{"host": host, "user": user, "confirm": true})
		t.Logf("vmware_host_account_update against vcsim's mismatched-response-type UpdateUser returned: %v", err)
		// Whatever the outcome (clean success, or a client-side unmarshal/type
		// error surfaced as err), it must be a real error from the round trip,
		// not this tool's own wiring breaking before reaching the server.
		if err != nil && strings.Contains(err.Error(), "unknown tool") {
			t.Fatalf("vmware_host_account_update is not registered: %v", err)
		}
	})

	t.Run("remove_gate_and_confirm", func(t *testing.T) {
		closedGate := newHostSecurityRegistry(context.Background(), c, RegistryOptions{AllowDestructive: false})
		if _, err := closedGate.CallTool("vmware_host_account_remove", map[string]interface{}{"host": host, "user_name": userID, "confirm": true}); err == nil {
			t.Fatal("expected vmware_host_account_remove to be denied with the gate closed")
		}
		if _, err := r.CallTool("vmware_host_account_remove", map[string]interface{}{"host": host, "user_name": userID}); err == nil {
			t.Fatal("expected vmware_host_account_remove to fail without confirm:true")
		}
	})

	t.Run("remove_requires_user_name", func(t *testing.T) {
		if _, err := r.CallTool("vmware_host_account_remove", map[string]interface{}{"host": host, "confirm": true}); err == nil {
			t.Fatal("expected an error when user_name is missing")
		}
	})

	t.Run("remove_succeeds_then_second_remove_fails", func(t *testing.T) {
		if _, err := r.CallTool("vmware_host_account_remove", map[string]interface{}{"host": host, "user_name": userID, "confirm": true}); err != nil {
			t.Fatalf("vmware_host_account_remove failed: %v", err)
		}
		// The real vcsim HostLocalAccountManager.RemoveUser faults
		// UserNotFound on a 2nd removal — proves this too hit real business
		// logic (irreversible: the account is genuinely gone).
		if _, err := r.CallTool("vmware_host_account_remove", map[string]interface{}{"host": host, "user_name": userID, "confirm": true}); err == nil {
			t.Fatal("expected a 2nd vmware_host_account_remove for the same user to fail (UserNotFound)")
		}
	})
}

// TestHostSecurityTools_AccountManagerUnavailableOnVCenter documents the
// ESXi-direct-only limitation of the 3 HostAccountManager tools against
// vcsim (see generated_host_security.go's top doc comment for the full,
// corrected-after-running mechanism): the per-host
// configManager.accountManager property resolves fine on a VPX()
// (vCenter-mode) simulator too (it's a static field on the host template
// shared by both models) — but simulator.VPX()'s top-level ServiceContent
// never gets a live HostLocalAccountManager object registered at that MOR
// (only simulator.ESX()'s does, because service_instance.go auto-registers
// one singleton per non-nil top-level ServiceContent reference, and VPX's
// is nil). So the call below reaches the real server and gets back
// "managed object not found", not a client-side pre-flight failure. Whether
// real multi-host vCenter deployments share this exact limitation wasn't
// verified against real hardware — only that vcsim does. This is the
// negative counterpart to TestHostSecurityTools_AccountManager's ESX()-model
// functional coverage.
func TestHostSecurityTools_AccountManagerUnavailableOnVCenter(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newHostSecurityRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	host := firstHostPath(t, r)

	user := map[string]interface{}{"id": "mcpvmware-vcenter-user", "password": "x"}
	if _, err := r.CallTool("vmware_host_account_create", map[string]interface{}{"host": host, "user": user, "confirm": true}); err == nil {
		t.Fatal("expected vmware_host_account_create to fail against a VPX()-mode simulator (no host or client-level account manager available)")
	} else {
		t.Logf("vmware_host_account_create correctly failed against vCenter-mode vcsim: %v", err)
	}
}

// mustCallTool is a small helper for tests that need a tool's raw JSON
// result mid-flow (not just success/failure), failing the test immediately
// on error instead of every call site repeating the same 3-line check.
func mustCallTool(t *testing.T, r *Registry, name string, args map[string]interface{}) string {
	t.Helper()
	raw, err := r.CallTool(name, args)
	if err != nil {
		t.Fatalf("%s failed: %v", name, err)
	}
	return raw
}

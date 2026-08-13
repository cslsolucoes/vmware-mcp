package tools

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/vim25/types"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// registerHostSecurityTools is the "security" slice of Fase 3 of the codegen
// plan (".workspace/plans/MCPVMWare2026-08-10-175300-plano-cobertura-completa-api-codegen.plan.md")
// — hand-transcribed from object.HostCertificateManager (7 methods),
// object.HostFirewallSystem (4 methods), and object.HostAccountManager (3
// methods), following the resolveHost/hostArg/register(Destructive)
// conventions established in host.go, generated_option.go, and
// generated_vm_lifecycle.go. All 3 managers are reached via
// host.ConfigManager().<Manager>(ctx), same pattern as generated_option.go's
// host.ConfigManager().OptionManager(ctx).
//
// Naming: every tool is renamed away from the generator's raw
// "<manager>_<manager>_<method>"-shaped proposal to drop the duplicated
// manager word — e.g. "certificate_certificate_info" -> "certificate_info",
// consistent with generated_option.go's stated naming rationale.
//
// Curation deviations / judgment calls (human review required):
//
//   - GenerateCertificateSigningRequest and GenerateCertificateSigningRequestByDn
//     are BOTH kept at Tier 2, including ByDn despite this group's brief
//     explicitly inviting a downgrade to read-only if it turns out to be
//     genuinely side-effect-free. Investigated by reading both real method
//     bodies (referencia/govmomi/object/host_certificate_manager.go) and the
//     simulator's implementation (referencia/govmomi/simulator/host_certificate_manager.go):
//     vcsim's GenerateCertificateSigningRequest generates an ephemeral RSA
//     key purely in-memory, signs the CSR with it, and discards the key —
//     genuinely stateless there. But real hostd cannot work that way: the
//     private key backing a CSR must be retained server-side until a
//     subsequently-signed certificate comes back for InstallServerCertificate
//     to pair with — otherwise InstallServerCertificate could never
//     complete. That means, on real vSphere, a second CSR call plausibly
//     invalidates/replaces whatever key was pending from the first one — a
//     real (if narrow) side effect this server can't verify is absent, not
//     "genuinely side-effect-free". GenerateCertificateSigningRequest and
//     GenerateCertificateSigningRequestByDn are structurally identical
//     (same risk profile, differing only in what goes into the CSR's
//     subject) — downgrading only one of the two would be an indefensible
//     inconsistency. Kept both at Tier 2, the conservative default for a
//     security-sensitive (certificate) domain. Flagging this explicitly for
//     reviewer judgment, since the brief's premise ("arguably read-only")
//     is plausible and a reviewer may weigh the risk differently.
//   - ReplaceCACertificatesAndCRLs.ca_certificates/ca_crls are both OPTIONAL
//     arrays (the real signature accepts either as a legitimately empty
//     slice — e.g. replace only the CRL set and clear the CA cert trust
//     list, or vice versa). Omitting one sends an empty list for that side,
//     matching the real API's shape exactly rather than inventing a
//     required-non-empty constraint the API doesn't have.
//
// vcsim support — verified per-method by reading the simulator source AND by
// running real tests against it (not assumed from the method existing on
// object.Host*Manager):
//
//   - HostCertificateManager: CertificateInfo works (it is a plain property
//     read of "certificateInfo" via the property collector, not a SOAP RPC —
//     and referencia/govmomi/simulator/host_certificate_manager.go's
//     NewHostCertificateManager populates that field at host-creation time
//     via an internal InstallServerCertificate call). GenerateCertificateSigningRequest
//     and InstallServerCertificate also have real receiver methods on
//     simulator.HostCertificateManager and work end-to-end. But
//     GenerateCertificateSigningRequestByDn, ListCACertificateRevocationLists,
//     ListCACertificates, and ReplaceCACertificatesAndCRLs have NO receiver
//     method on simulator.HostCertificateManager at all (confirmed by
//     reading the whole file — only 2 receiver methods exist besides the
//     constructor) — calls against vcsim fault types.MethodNotFound
//     (simulator's generic method-dispatch fallback). All 4 are still
//     registered exactly as real vSphere supports them; see
//     generated_host_security_test.go for how each is exercised short of a
//     full success path (registration, arg validation, tier gating).
//   - HostFirewallSystem: Info (property read of "firewallInfo", populated
//     at host creation from esx.HostFirewallInfo), DisableRuleset, and
//     EnableRuleset all have real receiver methods and work end-to-end.
//     Refresh has NO receiver method on simulator.HostFirewallSystem
//     (confirmed by reading referencia/govmomi/simulator/host_firewall_system.go)
//     — faults types.MethodNotFound on vcsim; still registered.
//   - HostAccountManager: this group's original brief claimed vcsim does not
//     simulate this manager at all — that claim was WRONG and was corrected
//     mid-task by the coordinator, then independently re-verified here by
//     reading source AND by running real tests against both simulator
//     models (not just trusting the correction). The mechanism is more
//     subtle than a first pass suggested, so this is written up precisely,
//     including a self-correction made after the first empirical test run
//     contradicted the initial (plausible-looking but wrong) theory below:
//     object.HostConfigManager.AccountManager(ctx) tries the per-host
//     "configManager.accountManager" property FIRST — and, contrary to an
//     initial read that assumed no simulator host ever wires this,
//     referencia/govmomi/simulator/esx/host_system.go's static HostSystem
//     property template (the shared base every simulated host is built
//     from, for BOTH simulator.ESX() and simulator.VPX() — confirmed by
//     reading the struct literal, ~line 1778) already sets
//     ConfigManager.AccountManager to {HostLocalAccountManager,
//     "ha-localacctmgr"} unconditionally. So this per-host lookup succeeds
//     the same way on every simulator host regardless of model, and the
//     ServiceContent-level fallback branch in the real
//     object/host_config_manager.go source (the "only when connected
//     directly to ESX" case) is never actually exercised by this simulator
//     at all — accurate for real hostd's pre-5.5 behavior, but not what
//     explains vcsim's ESX-vs-VPX difference; an earlier draft of this
//     comment claimed exactly that and was corrected after
//     TestHostSecurityTools_AccountManagerUnavailableOnVCenter's actual
//     output ("ServerFaultCode: managed object not found:
//     HostLocalAccountManager:ha-localacctmgr" — a SERVER-side fault, proving
//     the RPC was dispatched, not a client-side object.ErrNotSupported that
//     would mean no RPC happened at all) didn't match that theory.
//     The REAL mechanism (confirmed by reading
//     referencia/govmomi/simulator/service_instance.go's NewServiceInstance):
//     it reflects over the TOP-LEVEL types.ServiceContent struct via
//     mo.References(content) and auto-instantiates+registers a live
//     singleton object for every non-nil ManagedObjectReference field it
//     finds. referencia/govmomi/simulator/esx/service_content.go sets its
//     top-level AccountManager field to {HostLocalAccountManager,
//     "ha-localacctmgr"} — so simulator.ESX() DOES register a real
//     HostLocalAccountManager object at that MOR. referencia/govmomi/simulator/vpx/service_content.go
//     leaves that top-level field nil — so simulator.VPX() never registers
//     one, even though every VPX-managed host's own per-host property still
//     points at the exact same MOR value (inherited from the shared
//     esx/host_system.go template). The result: on VPX(), every call
//     resolves a MOR that simply does not exist server-side — "managed
//     object not found", not an unsupported-operation error. These 3 tools
//     are effectively ESXi-direct-only against vcsim either way — the
//     end-user-visible behavior this group's brief anticipated was right,
//     only the internal mechanism was initially mis-diagnosed. Whether real
//     multi-host vCenter deployments have the same practical limitation
//     wasn't verified against real hardware here — see
//     host_config_manager.go's "only when connected directly to ESX"
//     comment, which suggests real vSphere may differ from what vcsim
//     models.
//     On ESX(), the resolved ref is type HostLocalAccountManager, which
//     referencia/govmomi/simulator/host_local_account_manager.go DOES
//     implement — CreateUser and RemoveUser both have full, correct receiver
//     methods and were driven to real success/failure paths in testing.
//     UpdateUser has a receiver method too but its body is a known-buggy
//     copy/paste in the vendored simulator (returns
//     &methods.CreateUserBody{Res: &types.CreateUserResponse{}} instead of
//     an UpdateUserBody/UpdateUserResponse) — see
//     generated_host_security_test.go's account_update subtest for exactly
//     what that produces against a real client round trip and how it's
//     handled; not a bug in this file, a vendored referencia/ simulator gap
//     that must not be "fixed" here (referencia/ is read-only reference).
func registerHostSecurityTools(r *Registry) {
	hostArg := map[string]interface{}{
		"type":        "string",
		"description": `Host identifier: a name/pattern (e.g. "esxi-01.local") or a full inventory path (e.g. "/ha-datacenter/host/esxi-01.local/esxi-01.local") as returned by vmware_list_hosts. Must resolve to exactly one host.`,
	}
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}

	// --- HostCertificateManager: read-only -----------------------------

	r.register("vmware_host_certificate_info",
		"Get an ESXi host's current SSL certificate info (issuer, subject, validity, status, thumbprint).",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"host": hostArg},
			"required":   []interface{}{"host"},
		},
		Tool{Handler: handleHostCertificateInfo},
	)

	r.register("vmware_host_certificate_list_ca_crls",
		"List the SSL certificate revocation lists (CRLs) of Certificate Authorities trusted by an ESXi host.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"host": hostArg},
			"required":   []interface{}{"host"},
		},
		Tool{Handler: handleHostCertificateListCACRLs},
	)

	r.register("vmware_host_certificate_list_ca_certificates",
		"List the SSL certificates of Certificate Authorities trusted by an ESXi host.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"host": hostArg},
			"required":   []interface{}{"host"},
		},
		Tool{Handler: handleHostCertificateListCACertificates},
	)

	// --- HostCertificateManager: Tier 2 ---------------------------------

	r.registerDestructive("vmware_host_certificate_generate_csr",
		"Ask an ESXi host to generate a certificate-signing request (CSR) for itself, to be signed by a Certificate Authority and later imported via vmware_host_certificate_install_server_certificate. Kept at Tier 2 (not downgraded to read-only) — see this file's top doc comment for why a second call may invalidate a still-pending CSR's key on real vSphere even though vcsim's implementation is stateless.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":                          hostArg,
				"use_ip_address_as_common_name": map[string]interface{}{"type": "boolean", "description": "Use the host's management IP address as the CSR's Common Name instead of its hostname. Default false."},
				"confirm":                       confirmArg,
			},
			"required": []interface{}{"host", "confirm"},
		},
		Tool{Handler: handleHostCertificateGenerateCSR},
	)

	r.registerDestructive("vmware_host_certificate_generate_csr_by_dn",
		"Ask an ESXi host to generate a certificate-signing request (CSR) for itself using an explicit Distinguished Name, to be signed by a Certificate Authority and later imported via vmware_host_certificate_install_server_certificate. Kept at Tier 2 for the same reason as vmware_host_certificate_generate_csr (see this file's top doc comment) — not downgraded despite being structurally near-identical to that read-only-looking call.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":               hostArg,
				"distinguished_name": map[string]interface{}{"type": "string", "description": `The CSR's subject Distinguished Name, e.g. "CN=esxi-01.local,OU=IT,O=Example Corp,C=US".`},
				"confirm":            confirmArg,
			},
			"required": []interface{}{"host", "distinguished_name", "confirm"},
		},
		Tool{Handler: handleHostCertificateGenerateCSRByDn},
	)

	r.registerDestructive("vmware_host_certificate_install_server_certificate",
		"Install (import) an SSL certificate as an ESXi host's active server certificate, typically one signed from a CSR obtained via vmware_host_certificate_generate_csr(_by_dn). Reversible only by installing another certificate — there is no \"restore previous cert\" tool. Some effects (per the real method's source) only fully apply after a service refresh the host performs internally.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":    hostArg,
				"cert":    map[string]interface{}{"type": "string", "description": "PEM-encoded X.509 certificate to install as the host's active server certificate."},
				"confirm": confirmArg,
			},
			"required": []interface{}{"host", "cert", "confirm"},
		},
		Tool{Handler: handleHostCertificateInstallServerCertificate},
	)

	r.registerDestructive("vmware_host_certificate_replace_ca_certs_and_crls",
		"Replace the set of trusted Certificate Authority SSL certificates and/or CRLs an ESXi host uses to verify external entities. Reversible by calling this again with the previous lists. Omitting either list sends an empty list for that side (clears it), matching the real API's shape — pass both explicitly if you only intend to change one and keep the other.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":            hostArg,
				"ca_certificates": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "PEM-encoded CA certificates to trust. Omit/empty clears the trusted CA certificate list."},
				"ca_crls":         map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "PEM-encoded CA certificate revocation lists to trust. Omit/empty clears the trusted CRL list."},
				"confirm":         confirmArg,
			},
			"required": []interface{}{"host", "confirm"},
		},
		Tool{Handler: handleHostCertificateReplaceCACertsAndCRLs},
	)

	// --- HostFirewallSystem: read-only ----------------------------------

	r.register("vmware_host_firewall_info",
		"List an ESXi host's firewall rulesets (name, enabled state, allowed IPs, rules) and default policy.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"host": hostArg},
			"required":   []interface{}{"host"},
		},
		Tool{Handler: handleHostFirewallInfo},
	)

	// --- HostFirewallSystem: Tier 2 --------------------------------------

	r.registerDestructive("vmware_host_firewall_disable_ruleset",
		"Disable one firewall ruleset (by ID, e.g. \"sshServer\") on an ESXi host. Reversible via vmware_host_firewall_enable_ruleset.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":    hostArg,
				"id":      map[string]interface{}{"type": "string", "description": `Ruleset key/ID, e.g. "sshServer" or "CIMHttpServer" (see vmware_host_firewall_info's ruleset keys).`},
				"confirm": confirmArg,
			},
			"required": []interface{}{"host", "id", "confirm"},
		},
		Tool{Handler: handleHostFirewallDisableRuleset},
	)

	r.registerDestructive("vmware_host_firewall_enable_ruleset",
		"Enable one firewall ruleset (by ID, e.g. \"sshServer\") on an ESXi host. Reversible via vmware_host_firewall_disable_ruleset.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":    hostArg,
				"id":      map[string]interface{}{"type": "string", "description": `Ruleset key/ID, e.g. "sshServer" or "CIMHttpServer" (see vmware_host_firewall_info's ruleset keys).`},
				"confirm": confirmArg,
			},
			"required": []interface{}{"host", "id", "confirm"},
		},
		Tool{Handler: handleHostFirewallEnableRuleset},
	)

	r.registerDestructive("vmware_host_firewall_refresh",
		"Refresh an ESXi host's cached firewall configuration from disk. No configuration change, but classified alongside the other Tier 2 actions here per the plan's Fase 1a severity table (same posture as vmware_vm_refresh_storage_info).",
		tier2,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"host": hostArg, "confirm": confirmArg},
			"required":   []interface{}{"host", "confirm"},
		},
		Tool{Handler: handleHostFirewallRefresh},
	)

	// --- HostAccountManager: Tier 2 ---------------------------------------

	r.registerDestructive("vmware_host_account_create",
		"Create a local user account directly on an ESXi host. Works only when connected directly to an ESXi host (or, per vSphere's own API, hosts managed without a modern per-host account manager) — see this file's top doc comment for the ESXi-direct-only limitation confirmed against vcsim.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host": hostArg,
				"user": map[string]interface{}{
					"type":        "object",
					"description": `The account to create, matching types.HostAccountSpec: {"id": "...", "password": "...", "description": "..."}. "id" is required.`,
					"properties": map[string]interface{}{
						"id":          map[string]interface{}{"type": "string"},
						"password":    map[string]interface{}{"type": "string"},
						"description": map[string]interface{}{"type": "string"},
					},
					"required": []interface{}{"id"},
				},
				"confirm": confirmArg,
			},
			"required": []interface{}{"host", "user", "confirm"},
		},
		Tool{Handler: handleHostAccountCreate},
	)

	r.registerDestructive("vmware_host_account_update",
		"Update a local user account's password/description directly on an ESXi host. Same ESXi-direct-only limitation as vmware_host_account_create (see this file's top doc comment).",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host": hostArg,
				"user": map[string]interface{}{
					"type":        "object",
					"description": `The account to update, matching types.HostAccountSpec: {"id": "...", "password": "...", "description": "..."}. "id" is required and identifies the existing account.`,
					"properties": map[string]interface{}{
						"id":          map[string]interface{}{"type": "string"},
						"password":    map[string]interface{}{"type": "string"},
						"description": map[string]interface{}{"type": "string"},
					},
					"required": []interface{}{"id"},
				},
				"confirm": confirmArg,
			},
			"required": []interface{}{"host", "user", "confirm"},
		},
		Tool{Handler: handleHostAccountUpdate},
	)

	// --- HostAccountManager: Tier 1 (irreversible) -------------------------

	r.registerDestructive("vmware_host_account_remove",
		"Remove a local user account directly from an ESXi host. Irreversible (the account and its settings are gone — re-creating an account with the same name is not the same account). Same ESXi-direct-only limitation as vmware_host_account_create (see this file's top doc comment).",
		tier1,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":      hostArg,
				"user_name": map[string]interface{}{"type": "string", "description": "ID/name of the local account to remove."},
				"confirm":   confirmArg,
			},
			"required": []interface{}{"host", "user_name", "confirm"},
		},
		Tool{Handler: handleHostAccountRemove},
	)
}

func handleHostCertificateInfo(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}

	cm, err := host.ConfigManager().CertificateManager(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get certificate manager for %s: %w", host.InventoryPath, err)
	}

	info, err := cm.CertificateInfo(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to read certificate info for %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "certificate_info": info})
}

func handleHostCertificateListCACRLs(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}

	cm, err := host.ConfigManager().CertificateManager(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get certificate manager for %s: %w", host.InventoryPath, err)
	}

	crls, err := cm.ListCACertificateRevocationLists(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to list CA CRLs on %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "count": len(crls), "crls": crls})
}

func handleHostCertificateListCACertificates(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}

	cm, err := host.ConfigManager().CertificateManager(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get certificate manager for %s: %w", host.InventoryPath, err)
	}

	certs, err := cm.ListCACertificates(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to list CA certificates on %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "count": len(certs), "certificates": certs})
}

func handleHostCertificateGenerateCSR(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}
	useIP, _ := args["use_ip_address_as_common_name"].(bool)

	cm, err := host.ConfigManager().CertificateManager(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get certificate manager for %s: %w", host.InventoryPath, err)
	}

	csr, err := cm.GenerateCertificateSigningRequest(ctx, useIP)
	if err != nil {
		return "", fmt.Errorf("failed to generate CSR on %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "csr": csr})
}

func handleHostCertificateGenerateCSRByDn(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}
	dn, _ := args["distinguished_name"].(string)
	if dn == "" {
		return "", fmt.Errorf("distinguished_name is required")
	}

	cm, err := host.ConfigManager().CertificateManager(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get certificate manager for %s: %w", host.InventoryPath, err)
	}

	csr, err := cm.GenerateCertificateSigningRequestByDn(ctx, dn)
	if err != nil {
		return "", fmt.Errorf("failed to generate CSR by DN on %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "distinguished_name": dn, "csr": csr})
}

func handleHostCertificateInstallServerCertificate(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}
	cert, _ := args["cert"].(string)
	if cert == "" {
		return "", fmt.Errorf("cert is required")
	}

	cm, err := host.ConfigManager().CertificateManager(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get certificate manager for %s: %w", host.InventoryPath, err)
	}

	if err := cm.InstallServerCertificate(ctx, cert); err != nil {
		return "", fmt.Errorf("failed to install server certificate on %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "result": "certificate_installed"})
}

func handleHostCertificateReplaceCACertsAndCRLs(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}

	var caCerts []string
	if raw, ok := args["ca_certificates"]; ok {
		caCerts, err = toStringSlice(raw)
		if err != nil {
			return "", fmt.Errorf("invalid ca_certificates: %w", err)
		}
	}
	var caCRLs []string
	if raw, ok := args["ca_crls"]; ok {
		caCRLs, err = toStringSlice(raw)
		if err != nil {
			return "", fmt.Errorf("invalid ca_crls: %w", err)
		}
	}

	cm, err := host.ConfigManager().CertificateManager(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get certificate manager for %s: %w", host.InventoryPath, err)
	}

	if err := cm.ReplaceCACertificatesAndCRLs(ctx, caCerts, caCRLs); err != nil {
		return "", fmt.Errorf("failed to replace CA certificates/CRLs on %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{
		"host":                 host.InventoryPath,
		"ca_certificate_count": len(caCerts),
		"ca_crl_count":         len(caCRLs),
		"result":               "ca_certs_and_crls_replaced",
	})
}

func handleHostFirewallInfo(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}

	fs, err := host.ConfigManager().FirewallSystem(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get firewall system for %s: %w", host.InventoryPath, err)
	}

	info, err := fs.Info(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to read firewall info for %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "firewall_info": info})
}

func handleHostFirewallDisableRuleset(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}
	id, _ := args["id"].(string)
	if id == "" {
		return "", fmt.Errorf("id is required")
	}

	fs, err := host.ConfigManager().FirewallSystem(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get firewall system for %s: %w", host.InventoryPath, err)
	}

	if err := fs.DisableRuleset(ctx, id); err != nil {
		return "", fmt.Errorf("failed to disable ruleset %q on %s: %w", id, host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "id": id, "result": "ruleset_disabled"})
}

func handleHostFirewallEnableRuleset(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}
	id, _ := args["id"].(string)
	if id == "" {
		return "", fmt.Errorf("id is required")
	}

	fs, err := host.ConfigManager().FirewallSystem(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get firewall system for %s: %w", host.InventoryPath, err)
	}

	if err := fs.EnableRuleset(ctx, id); err != nil {
		return "", fmt.Errorf("failed to enable ruleset %q on %s: %w", id, host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "id": id, "result": "ruleset_enabled"})
}

func handleHostFirewallRefresh(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}

	fs, err := host.ConfigManager().FirewallSystem(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get firewall system for %s: %w", host.InventoryPath, err)
	}

	if err := fs.Refresh(ctx); err != nil {
		return "", fmt.Errorf("failed to refresh firewall config on %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "result": "firewall_refreshed"})
}

// decodeHostAccountSpec re-marshals the generic JSON "user" arg into a
// types.HostAccountSpec — same "accept generic JSON, not a hand-built
// recursive schema" approach as generated_vm_lifecycle.go's decodeJSONArg
// (reused directly here), applied to the one Create/Update require an "id".
func decodeHostAccountSpec(raw interface{}) (*types.HostAccountSpec, error) {
	var spec types.HostAccountSpec
	if err := decodeJSONArg(raw, &spec); err != nil {
		return nil, err
	}
	if spec.Id == "" {
		return nil, fmt.Errorf("user.id is required")
	}
	return &spec, nil
}

func handleHostAccountCreate(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}
	raw, ok := args["user"]
	if !ok {
		return "", fmt.Errorf("user is required")
	}
	spec, err := decodeHostAccountSpec(raw)
	if err != nil {
		return "", fmt.Errorf("invalid user: %w", err)
	}

	am, err := host.ConfigManager().AccountManager(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get account manager for %s: %w", host.InventoryPath, err)
	}

	if err := am.Create(ctx, spec); err != nil {
		return "", fmt.Errorf("failed to create account %q on %s: %w", spec.Id, host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "id": spec.Id, "result": "account_created"})
}

func handleHostAccountUpdate(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}
	raw, ok := args["user"]
	if !ok {
		return "", fmt.Errorf("user is required")
	}
	spec, err := decodeHostAccountSpec(raw)
	if err != nil {
		return "", fmt.Errorf("invalid user: %w", err)
	}

	am, err := host.ConfigManager().AccountManager(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get account manager for %s: %w", host.InventoryPath, err)
	}

	if err := am.Update(ctx, spec); err != nil {
		return "", fmt.Errorf("failed to update account %q on %s: %w", spec.Id, host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "id": spec.Id, "result": "account_updated"})
}

func handleHostAccountRemove(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}
	userName, _ := args["user_name"].(string)
	if userName == "" {
		return "", fmt.Errorf("user_name is required")
	}

	am, err := host.ConfigManager().AccountManager(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get account manager for %s: %w", host.InventoryPath, err)
	}

	if err := am.Remove(ctx, userName); err != nil {
		return "", fmt.Errorf("failed to remove account %q on %s: %w", userName, host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "user_name": userName, "result": "account_removed"})
}

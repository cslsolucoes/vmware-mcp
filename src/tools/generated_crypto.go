// Package tools — generated_crypto.go is Fase 8a (Wave 2, group CS-CRYPTO)
// of the codegen plan
// (".workspace/plans/MCPVMWare2026-08-10-175300-plano-cobertura-completa-api-codegen.plan.md"),
// covering govmomi's vapi/crypto package (referencia/govmomi/vapi/crypto/crypto.go)
// — KMS (Key Management Server) provider management for vSphere VM
// encryption. 3 tools total.
//
// Architecture: same vapi/*-over-*rest.Client family as generated_tags.go —
// see generated_esx_settings_cluster_vms.go's top doc comment for the full
// rationale, not repeated here. All 3 structs used here
// (KmsProviderCreateSpec, KmsProviderExportSpec) are small and flat, so they
// are built field-by-field from named top-level arguments (matching
// generated_tags.go's CreateTag/CreateCategory precedent), not via a generic
// decodeJSONArg object.
//
// mode=vcenter-only: the entire vapi/* domain requires a vCenter Server
// Appliance (VAMI/VAPI session) — see client.REST's doc comment.
//
// vcsim gap, not a bug: confirmed directly that
// referencia/govmomi/vapi/simulator/simulator.go does not import this
// package (grep -cE "vapi/(esx/settings|cluster\"|crypto|cis/tasks)" against
// that file returns 0) — no vapi/crypto/simulator package exists upstream
// either (confirmed: no "simulator" subdirectory under
// referencia/govmomi/vapi/crypto/, unlike vapi/cluster and
// vapi/esx/settings/clusters/vms), so this is the strictest form of "vcsim
// gap" in this batch: there is no server-side implementation anywhere in
// govmomi's own test tooling for this manager, not even one this project
// chose not to wire in. Every call from this project's vcsim-backed tests
// reaches a real, unhandled REST endpoint and gets back a genuine HTTP-level
// error, not a wiring bug. Tests use assertReachesServer
// (generated_vm_lifecycle_test.go) for exactly this reason.
//
// Curation (per the orchestrator's brief, not independently discovered
// here): KmsProviderExportRequest(ctx, export *KmsProviderExportLocation)
// (*http.Request, error) is deliberately NOT registered as a tool — it only
// builds an *http.Request (setting a Bearer Authorization header from the
// download token) and never sends it, so it has no observable effect and
// nothing to serialize back to a caller over JSON-RPC — the same reasoning
// already applied to SupportBundleRequest-shaped helpers in prior fases.
// vmware_crypto_kms_provider_export already returns the *KmsProviderExport
// (including the download URL/token inside .location), which is the
// actionable, round-trippable half of this API surface.
package tools

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/vapi/crypto"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

func registerCryptoTools(r *Registry) {
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}
	providerArg := map[string]interface{}{
		"type":        "string",
		"description": "Identifier of the KMS provider (as configured in vCenter's native key provider / KMIP cluster configuration).",
	}

	r.registerDestructive("vmware_crypto_kms_provider_create",
		"Register a new KMS (Key Management Server) provider for vSphere VM encryption.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"provider":     providerArg,
				"tpm_required": map[string]interface{}{"type": "boolean", "description": "If true, only ESXi hosts with a TPM 2.0 chip may use this provider to unlock encrypted VMs. Default false."},
				"confirm":      confirmArg,
			},
			"required": []interface{}{"provider", "confirm"},
		},
		Tool{Handler: handleCryptoKmsProviderCreate},
	)

	r.registerDestructive("vmware_crypto_kms_provider_delete",
		"Remove a registered KMS provider. Irreversible — any VMs still encrypted with keys from this provider become inaccessible unless the provider (or its keys) can be restored independently.",
		tier1,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"provider": providerArg, "confirm": confirmArg},
			"required":   []interface{}{"provider", "confirm"},
		},
		Tool{Handler: handleCryptoKmsProviderDelete},
	)

	r.registerDestructive("vmware_crypto_kms_provider_export",
		"Export a KMS provider's configuration (a downloadable location + one-time download token — see .location in the result), e.g. to re-import it as a trusted provider elsewhere.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"provider": providerArg,
				"password": map[string]interface{}{"type": "string", "description": "Optional password protecting the exported configuration, if the KMS plugin supports/requires one."},
				"confirm":  confirmArg,
			},
			"required": []interface{}{"provider", "confirm"},
		},
		Tool{Handler: handleCryptoKmsProviderExport},
	)
}

// cryptoManager returns a crypto.Manager bound to client's VAPI/REST
// session, logging in lazily via client.REST — same pattern as
// generated_tags.go's tagsManager.
func cryptoManager(ctx context.Context, client *vmware.Client) (*crypto.Manager, error) {
	rc, err := client.REST(ctx)
	if err != nil {
		return nil, err
	}
	return crypto.NewManager(rc), nil
}

func handleCryptoKmsProviderCreate(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := cryptoManager(ctx, client)
	if err != nil {
		return "", err
	}
	provider, _ := args["provider"].(string)
	if provider == "" {
		return "", fmt.Errorf("provider is required")
	}
	tpmRequired, _ := args["tpm_required"].(bool)

	spec := crypto.KmsProviderCreateSpec{
		Provider:    provider,
		Constraints: crypto.KmsProviderConstraints{TpmRequired: tpmRequired},
	}
	if err := m.KmsProviderCreate(ctx, spec); err != nil {
		return "", fmt.Errorf("failed to create KMS provider %q: %w", provider, err)
	}
	return marshalJSON(map[string]interface{}{"result": "kms_provider_created", "provider": provider, "tpm_required": tpmRequired})
}

func handleCryptoKmsProviderDelete(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := cryptoManager(ctx, client)
	if err != nil {
		return "", err
	}
	provider, _ := args["provider"].(string)
	if provider == "" {
		return "", fmt.Errorf("provider is required")
	}

	if err := m.KmsProviderDelete(ctx, provider); err != nil {
		return "", fmt.Errorf("failed to delete KMS provider %q: %w", provider, err)
	}
	return marshalJSON(map[string]interface{}{"result": "kms_provider_deleted", "provider": provider})
}

func handleCryptoKmsProviderExport(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := cryptoManager(ctx, client)
	if err != nil {
		return "", err
	}
	provider, _ := args["provider"].(string)
	if provider == "" {
		return "", fmt.Errorf("provider is required")
	}
	password, _ := args["password"].(string)

	export, err := m.KmsProviderExport(ctx, crypto.KmsProviderExportSpec{Provider: provider, Password: password})
	if err != nil {
		return "", fmt.Errorf("failed to export KMS provider %q: %w", provider, err)
	}
	return marshalJSON(map[string]interface{}{"provider": provider, "export": export})
}

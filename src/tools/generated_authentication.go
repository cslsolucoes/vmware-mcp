package tools

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/vapi/authentication"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// registerAuthenticationTools is part of the "MISC-APPLIANCE" group of Fase
// 8a Wave 2 of the codegen plan
// (".workspace/plans/MCPVMWare2026-08-10-175300-plano-cobertura-completa-api-codegen.plan.md")
// — vapi/authentication.Manager, hand-transcribed from the real
// referencia/govmomi/vapi/authentication/authentication.go source (confirmed
// identical, byte-for-byte modulo line endings, to the pinned dependency
// github.com/vmware/govmomi v0.55.1 actually resolved by src/go.mod). 1
// method: Issue.
//
// Curation deviation from src/gen/classification.json — the ONLY tier
// correction in the OPPOSITE direction anywhere in this Wave 2 batch (every
// other correction across this group's 3 files, and every prior fase's
// corrections, moves a tool DOWN from an over-eager tier2/tier1 default to
// read-only; this one moves UP):
//
//   - vmware_authentication_issue (Issue in authentication.go): the AST
//     classifier tagged this read-only ("name matches read-only pattern" —
//     the regex evidently matched on "Issue" not looking like a mutating verb
//     prefix such as Create/Set/Delete/Update). Confirmed by reading the real
//     source that this is wrong: `c.Do(ctx, url.Request(http.MethodPost, spec), &res)`
//     against POST /vcenter/tokenservice/token-exchange — a genuine mutating
//     call with a real security side effect (it MINTS a brand-new
//     authentication token/credential from an input token, OAuth2
//     token-exchange style — RFC 8693 semantics, confirmed by the shape of
//     TokenIssueSpec's fields: subject_token/subject_token_type/grant_type/
//     actor_token/requested_token_type/scope/audience). A freshly issued
//     access_token is functionally a new credential the caller can then use
//     to authenticate as the subject — treating that as harmlessly
//     "read-only" would let a Tier-0 (ungated) tool call silently create
//     live credentials. Corrected to Tier 2 (disruptive/reversible in the
//     sense that a wrongly-issued token can be terminated via session
//     management, but the call itself is not idempotent-safe like a real
//     read) and gated through registerDestructive, same 3-layer protection
//     (server gate + confirm:true + audit log) as every other tier2 tool in
//     this project.
//
// vcsim gap, not a bug — confirmed directly:
// `grep -rn "tokenservice\|token-exchange" referencia/govmomi/vapi/simulator/simulator.go`
// returns 0 matches (vapi/simulator only imports vapi/library, vapi/tags,
// vapi/vcenter, vapi/rest). Tested only for "reaches the server cleanly"
// (assertReachesServer, defined in generated_vm_lifecycle_test.go, reused
// verbatim), same discipline as every other vcsim-gap domain in this
// project.
func registerAuthenticationTools(r *Registry) {
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}

	r.registerDestructive("vmware_authentication_issue",
		"Exchange a subject token for a newly issued vCenter authentication token (OAuth2 token-exchange style, RFC 8693 semantics — POST /vcenter/tokenservice/token-exchange). This MINTS a real, usable credential — despite the read-sounding name \"Issue\", it is not a read-only lookup (see this file's top doc comment for why the codegen classifier's default was overridden). Registered at Tier 2 and gated accordingly.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"token": map[string]interface{}{
					"type":        "object",
					"description": `An authentication.TokenIssueSpec JSON object (matching its Go struct's json tags). Required: "subject_token" (the token being exchanged), "subject_token_type" (a URI identifying the subject token's type, e.g. "urn:ietf:params:oauth:token-type:access_token"), "grant_type" (typically "urn:ietf:params:oauth:grant-type:token-exchange"). Optional: "actor_token", "actor_token_type", "requested_token_type", "resource", "scope", "audience".`,
				},
				"confirm": confirmArg,
			},
			"required": []interface{}{"token", "confirm"},
		},
		Tool{Handler: handleAuthenticationIssue},
	)
}

// authenticationManager returns an *authentication.Manager for client — this
// file's equivalent of libraryCoreManager/vmDatasetManager: client.REST(ctx)
// (added in Fase 4 for VAMI) already names the likely cause of failure ("is
// the target a vCenter Server Appliance?") if called against a standalone
// ESXi host.
func authenticationManager(ctx context.Context, client *vmware.Client) (*authentication.Manager, error) {
	rc, err := client.REST(ctx)
	if err != nil {
		return nil, err
	}
	return authentication.NewManager(rc), nil
}

func handleAuthenticationIssue(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := authenticationManager(ctx, client)
	if err != nil {
		return "", err
	}
	raw, ok := args["token"]
	if !ok {
		return "", fmt.Errorf("token is required")
	}
	var spec authentication.TokenIssueSpec
	if err := decodeJSONArg(raw, &spec); err != nil {
		return "", fmt.Errorf("invalid token: %w", err)
	}
	if spec.SubjectToken == "" {
		return "", fmt.Errorf("token.subject_token is required")
	}
	if spec.SubjectTokenType == "" {
		return "", fmt.Errorf("token.subject_token_type is required")
	}
	if spec.GrantType == "" {
		return "", fmt.Errorf("token.grant_type is required")
	}

	info, err := m.Issue(ctx, spec)
	if err != nil {
		return "", fmt.Errorf("failed to issue token: %w", err)
	}
	return marshalJSON(map[string]interface{}{"token_info": info, "result": "issued"})
}

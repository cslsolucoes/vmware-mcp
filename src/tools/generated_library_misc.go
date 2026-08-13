package tools

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/vapi/library"
	"github.com/vmware/govmomi/vapi/library/finder"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// registerLibraryMiscTools is Fase 8a's Grupo CL-C ("Content Library — uso/
// certificados confiáveis/segurança/ficheiros/storage/finder") of the codegen
// plan (".workspace/plans/MCPVMWare2026-08-10-175300-plano-cobertura-completa-api-codegen.plan.md").
// Unlike every Fase 0-7 file, this one wraps referencia/govmomi/vapi/library/
// — REST/JSON wrappers over *rest.Client, not SOAP/XML wrappers over
// vim25/types — so its structs already carry `json` tags and argument
// decoding goes straight through Go's encoding/json (no generic
// property-collector decode needed).
//
// mode=vcenter-only: the entire vapi/* domain requires the VAPI/REST session
// (client.REST(ctx)), which vmware.Client.REST's own doc comment already
// flags as VCSA/vCenter-only (a standalone ESXi host has no VAPI endpoint at
// all, confirmed independently by this project's Fase 4 appliance.go work).
//
// Source files covered (5 handler files + 1 finder helper file, all read
// directly from referencia/govmomi/vapi/library/, not guessed):
// library_usage.go, trusted_certificates.go, security_policy.go,
// library_file.go, library_item_storage.go, library/finder/finder.go.
//
// Curation:
//
//   - vmware_library_default_ovf_security_policy (Manager.
//     DefaultOvfSecurityPolicy): the Fase 0 AST classifier tagged this tier2
//     (fail-safe default for any method it couldn't prove read-only).
//     Corrected to no-tier here — confirmed by reading the real source
//     (security_policy.go): it only calls c.ListSecurityPolicies(ctx) and
//     filters the result client-side for the policy named "OVF default
//     policy". Zero mutation, same "AST classifier flagged a pure read as
//     risky" correction class as generated_authorization.go's
//     vmware_authorization_role_list and generated_custom_fields.go's
//     vmware_custom_field_field.
//
//   - AddLibraryUsage/RemoveLibraryUsage (tier2/tier1) and
//     CreateTrustedCertificate/DeleteTrustedCertificate (tier2/tier1): tiers
//     as assigned by this group's brief — not re-derived here.
//
//   - Tool count: the brief estimated 17 tools total (14 from the 5 Manager
//     files + "3, CONFIRME the real number" from finder.go). Reading
//     finder.go directly (see its top doc comment) shows its only exported
//     API surface is the Finder struct, the NewFinder constructor (not a
//     tool — no I/O of its own), and a single method, Find. So this file
//     registers 15 tools (14 Manager methods + 1 finder method), not 17 —
//     documented here instead of silently deviating from the brief's count.
//
// vcsim support: every one of the 15 methods below has genuine server-side
// handling in referencia/govmomi/vapi/simulator/simulator.go (not a generic
// 404 fallback) — confirmed by reading the handler table in that file's
// New() and each handler function body directly:
//   - libraryUsages/libraryUsageID (usages.go) maintain a real in-memory
//     map[libraryID]map[usageID]library.Usage.
//   - libraryTrustedCertificates/libraryTrustedCertificatesID maintain a
//     real map[id]library.TrustedCertificate, including PEM parsing
//     (x509.ParseCertificate) on create — an invalid PEM/cert genuinely
//     fails server-side, not just client-side.
//   - librarySecurityPolicies returns handler.Policies, seeded by
//     defaultSecurityPolicies() with exactly one policy named "OVF default
//     policy" — so vmware_library_default_ovf_security_policy has a real
//     match to find, not just a "not found" path, against vcsim.
//   - libraryItemFile/libraryItemFileID and libraryItemStorage/
//     libraryItemStorageID read the real per-item []library.File slice
//     populated by an actual file upload through the update-session flow
//     (libraryItemFileCreate) — this file's test uploads a real (small, in-
//     memory) ISO file through that flow rather than asserting against an
//     empty item, so Get/List actually have data to return.
//   - finder.Find rides entirely on Manager.FindLibrary/GetLibraryByID/
//     GetLibraries/FindLibraryItems/GetLibraryItem/GetLibraryItems/
//     GetLibraryItemFile/ListLibraryItemFiles, every one of which vcsim
//     already implements for the reasons above (and library.go/
//     library_item.go's own general Content Library support, confirmed real
//     against vcsim by this project's pre-generation spike per the brief).
func registerLibraryMiscTools(r *Registry) {
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}
	libraryIDArg := map[string]interface{}{
		"type":        "string",
		"description": "Content library ID (library.Library.ID), as returned by a library listing/creation tool.",
	}
	usageIDArg := map[string]interface{}{
		"type":        "string",
		"description": "Library usage record ID, as returned by vmware_library_add_library_usage or vmware_library_list_library_usage.",
	}
	libraryItemIDArg := map[string]interface{}{
		"type":        "string",
		"description": "Content library item ID (library.Item.ID), as returned by a library item listing/creation tool.",
	}
	fileNameArg := map[string]interface{}{
		"type":        "string",
		"description": `The library item file's name, e.g. "disk.vmdk" or "template.ovf" — as returned by vmware_library_list_library_item_files.`,
	}
	certificateIDArg := map[string]interface{}{
		"type":        "string",
		"description": "Trusted certificate ID, as returned by vmware_library_list_trusted_certificates or vmware_library_create_trusted_certificate.",
	}

	// --- library usage ---------------------------------------------------

	r.register("vmware_library_get_library_usage",
		"Get one resource-usage record for a content library by usage ID (which resource — e.g. a workload cluster service — currently depends on this library).",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"library_id": libraryIDArg, "usage_id": usageIDArg},
			"required":   []interface{}{"library_id", "usage_id"},
		},
		Tool{Handler: handleLibraryGetLibraryUsage},
	)

	r.register("vmware_library_list_library_usage",
		"List every resource currently using a content library. A library can be safely deleted if this returns no usage records.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"library_id": libraryIDArg},
			"required":   []interface{}{"library_id"},
		},
		Tool{Handler: handleLibraryListLibraryUsage},
	)

	r.registerDestructive("vmware_library_add_library_usage",
		"Register a resource (identified by a resource URN) as a user of a content library. Reversible via vmware_library_remove_library_usage.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"library_id":   libraryIDArg,
				"resource_urn": map[string]interface{}{"type": "string", "description": `URN of the resource that uses this library, e.g. "vmomi:service:wcp".`},
				"confirm":      confirmArg,
			},
			"required": []interface{}{"library_id", "resource_urn", "confirm"},
		},
		Tool{Handler: handleLibraryAddLibraryUsage},
	)

	r.registerDestructive("vmware_library_remove_library_usage",
		"Remove a resource-usage record from a content library (by usage ID). Does not delete the library or its contents — only the usage bookkeeping entry.",
		tier1,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"library_id": libraryIDArg, "usage_id": usageIDArg, "confirm": confirmArg},
			"required":   []interface{}{"library_id", "usage_id", "confirm"},
		},
		Tool{Handler: handleLibraryRemoveLibraryUsage},
	)

	// --- trusted certificates ----------------------------------------------

	r.register("vmware_library_list_trusted_certificates",
		"List every certificate in the content library subsystem's trust store (used to validate signed OVF/OVA library items).",
		map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
		Tool{Handler: handleLibraryListTrustedCertificates},
	)

	r.register("vmware_library_get_trusted_certificate",
		"Get one trusted certificate (Base64-encoded PEM) from the content library trust store by ID.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"certificate_id": certificateIDArg},
			"required":   []interface{}{"certificate_id"},
		},
		Tool{Handler: handleLibraryGetTrustedCertificate},
	)

	r.registerDestructive("vmware_library_create_trusted_certificate",
		"Add a certificate (Base64-encoded PEM text) to the content library trust store, so items signed by it are treated as trusted. Reversible via vmware_library_delete_trusted_certificate.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"cert_text": map[string]interface{}{"type": "string", "description": "The certificate in Base64-encoded PEM format (\"-----BEGIN CERTIFICATE-----...\")."},
				"confirm":   confirmArg,
			},
			"required": []interface{}{"cert_text", "confirm"},
		},
		Tool{Handler: handleLibraryCreateTrustedCertificate},
	)

	r.registerDestructive("vmware_library_delete_trusted_certificate",
		"Remove a certificate from the content library trust store by ID. Items previously trusted only because of this certificate will no longer verify.",
		tier1,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"certificate_id": certificateIDArg, "confirm": confirmArg},
			"required":   []interface{}{"certificate_id", "confirm"},
		},
		Tool{Handler: handleLibraryDeleteTrustedCertificate},
	)

	// --- security policy -----------------------------------------------

	r.register("vmware_library_list_security_policies",
		"List every content library security policy known to this vCenter (each policy maps a library item type, e.g. \"ovf\", to a verification rule, e.g. \"OVF_STRICT_VERIFICATION\").",
		map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
		Tool{Handler: handleLibraryListSecurityPolicies},
	)

	r.register("vmware_library_default_ovf_security_policy",
		"Get the ID of the built-in \"OVF default policy\" security policy (used to enforce OVF signature verification on library items). Read-only — filters vmware_library_list_security_policies client-side, no mutation.",
		map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
		Tool{Handler: handleLibraryDefaultOvfSecurityPolicy},
	)

	// --- library item files ----------------------------------------------

	r.register("vmware_library_list_library_item_files",
		"List every file (name, size, checksum, cache/version state) belonging to a content library item.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"library_item_id": libraryItemIDArg},
			"required":   []interface{}{"library_item_id"},
		},
		Tool{Handler: handleLibraryListLibraryItemFiles},
	)

	r.register("vmware_library_get_library_item_file",
		"Get one file (by name) belonging to a content library item.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"library_item_id": libraryItemIDArg, "file_name": fileNameArg},
			"required":   []interface{}{"library_item_id", "file_name"},
		},
		Tool{Handler: handleLibraryGetLibraryItemFile},
	)

	// --- library item storage --------------------------------------------

	r.register("vmware_library_list_library_item_storage",
		"List the storage backing (datastore URIs, checksum, size, cache state) for every file of a content library item — an expanded form of vmware_library_list_library_item_files.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"library_item_id": libraryItemIDArg},
			"required":   []interface{}{"library_item_id"},
		},
		Tool{Handler: handleLibraryListLibraryItemStorage},
	)

	r.register("vmware_library_get_library_item_storage",
		"Get the storage backing for one named file of a content library item.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"library_item_id": libraryItemIDArg, "file_name": fileNameArg},
			"required":   []interface{}{"library_item_id", "file_name"},
		},
		Tool{Handler: handleLibraryGetLibraryItemStorage},
	)

	// --- finder ------------------------------------------------------------

	r.register("vmware_library_finder_find",
		`Find content library objects (libraries, items, or files) by inventory path(s) of the form "LIBRARY/ITEM/FILE" — e.g. "my-library" (list matching libraries), "my-library/my-item" (list matching items), or "my-library/*.ova" (wildcard-match items/files). Both "*" and "?" glob wildcards are supported; a path segment with no wildcard is resolved via a direct server-side lookup instead of a full listing. Omit "paths" (or pass an empty list) to list every library.`,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"paths": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": `One or more inventory path patterns to search for, e.g. ["my-library/*.ova", "other-library/template-a"]. Defaults to matching every library if omitted.`,
				},
			},
		},
		Tool{Handler: handleLibraryFinderFind},
	)
}

// libraryMiscManager builds a *library.Manager over the connection's VAPI/
// REST session (lazily logged in by client.REST — see its doc comment for
// why this fails cleanly, not with a panic, against a standalone ESXi host).
func libraryMiscManager(ctx context.Context, client *vmware.Client) (*library.Manager, error) {
	rc, err := client.REST(ctx)
	if err != nil {
		return nil, err
	}
	return library.NewManager(rc), nil
}

// libraryMiscRequiredString reads a required, non-empty string argument,
// naming both the argument and the tool-domain in the error so a caller
// passing the wrong shape (e.g. a number, or omitting the key) gets a
// specific message instead of a generic decode failure.
func libraryMiscRequiredString(args map[string]interface{}, key string) (string, error) {
	v, _ := args[key].(string)
	if v == "" {
		return "", fmt.Errorf("missing required argument %q", key)
	}
	return v, nil
}

func handleLibraryGetLibraryUsage(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryMiscManager(ctx, client)
	if err != nil {
		return "", err
	}
	libraryID, err := libraryMiscRequiredString(args, "library_id")
	if err != nil {
		return "", err
	}
	usageID, err := libraryMiscRequiredString(args, "usage_id")
	if err != nil {
		return "", err
	}
	usage, err := m.GetLibraryUsage(ctx, libraryID, usageID)
	if err != nil {
		return "", fmt.Errorf("failed to get library usage: %w", err)
	}
	return marshalJSON(usage)
}

func handleLibraryListLibraryUsage(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryMiscManager(ctx, client)
	if err != nil {
		return "", err
	}
	libraryID, err := libraryMiscRequiredString(args, "library_id")
	if err != nil {
		return "", err
	}
	list, err := m.ListLibraryUsage(ctx, libraryID)
	if err != nil {
		return "", fmt.Errorf("failed to list library usage: %w", err)
	}
	return marshalJSON(map[string]interface{}{"count": len(list.LibraryUsageList), "usage": list.LibraryUsageList})
}

func handleLibraryAddLibraryUsage(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryMiscManager(ctx, client)
	if err != nil {
		return "", err
	}
	libraryID, err := libraryMiscRequiredString(args, "library_id")
	if err != nil {
		return "", err
	}
	resourceURN, err := libraryMiscRequiredString(args, "resource_urn")
	if err != nil {
		return "", err
	}
	usageID, err := m.AddLibraryUsage(ctx, libraryID, library.AddUsage{ResourceUrn: resourceURN})
	if err != nil {
		return "", fmt.Errorf("failed to add library usage: %w", err)
	}
	return marshalJSON(map[string]interface{}{
		"library_id":   libraryID,
		"usage_id":     usageID,
		"resource_urn": resourceURN,
		"result":       "added",
	})
}

func handleLibraryRemoveLibraryUsage(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryMiscManager(ctx, client)
	if err != nil {
		return "", err
	}
	libraryID, err := libraryMiscRequiredString(args, "library_id")
	if err != nil {
		return "", err
	}
	usageID, err := libraryMiscRequiredString(args, "usage_id")
	if err != nil {
		return "", err
	}
	if err := m.RemoveLibraryUsage(ctx, libraryID, usageID); err != nil {
		return "", fmt.Errorf("failed to remove library usage: %w", err)
	}
	return marshalJSON(map[string]interface{}{"library_id": libraryID, "usage_id": usageID, "result": "removed"})
}

func handleLibraryListTrustedCertificates(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryMiscManager(ctx, client)
	if err != nil {
		return "", err
	}
	certs, err := m.ListTrustedCertificates(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to list trusted certificates: %w", err)
	}
	return marshalJSON(map[string]interface{}{"count": len(certs), "certificates": certs})
}

func handleLibraryGetTrustedCertificate(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryMiscManager(ctx, client)
	if err != nil {
		return "", err
	}
	certificateID, err := libraryMiscRequiredString(args, "certificate_id")
	if err != nil {
		return "", err
	}
	cert, err := m.GetTrustedCertificate(ctx, certificateID)
	if err != nil {
		return "", fmt.Errorf("failed to get trusted certificate: %w", err)
	}
	return marshalJSON(cert)
}

func handleLibraryCreateTrustedCertificate(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryMiscManager(ctx, client)
	if err != nil {
		return "", err
	}
	certText, err := libraryMiscRequiredString(args, "cert_text")
	if err != nil {
		return "", err
	}
	if err := m.CreateTrustedCertificate(ctx, certText); err != nil {
		return "", fmt.Errorf("failed to create trusted certificate: %w", err)
	}

	result := map[string]interface{}{"result": "created"}
	// CreateTrustedCertificate's response carries no ID (vAPI returns 201
	// with an empty body — see the reference source). Best-effort resolve it
	// by re-listing and matching on cert_text, so a caller doesn't have to
	// make a second round trip just to learn the ID it needs for
	// vmware_library_delete_trusted_certificate.
	if certs, listErr := m.ListTrustedCertificates(ctx); listErr == nil {
		for _, c := range certs {
			if c.Text == certText {
				result["certificate_id"] = c.ID
				break
			}
		}
	}
	return marshalJSON(result)
}

func handleLibraryDeleteTrustedCertificate(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryMiscManager(ctx, client)
	if err != nil {
		return "", err
	}
	certificateID, err := libraryMiscRequiredString(args, "certificate_id")
	if err != nil {
		return "", err
	}
	if err := m.DeleteTrustedCertificate(ctx, certificateID); err != nil {
		return "", fmt.Errorf("failed to delete trusted certificate: %w", err)
	}
	return marshalJSON(map[string]interface{}{"certificate_id": certificateID, "result": "deleted"})
}

func handleLibraryListSecurityPolicies(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryMiscManager(ctx, client)
	if err != nil {
		return "", err
	}
	policies, err := m.ListSecurityPolicies(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to list security policies: %w", err)
	}
	return marshalJSON(map[string]interface{}{"count": len(policies), "policies": policies})
}

func handleLibraryDefaultOvfSecurityPolicy(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryMiscManager(ctx, client)
	if err != nil {
		return "", err
	}
	policy, err := m.DefaultOvfSecurityPolicy(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get default OVF security policy: %w", err)
	}
	return marshalJSON(map[string]interface{}{"policy": policy})
}

func handleLibraryListLibraryItemFiles(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryMiscManager(ctx, client)
	if err != nil {
		return "", err
	}
	itemID, err := libraryMiscRequiredString(args, "library_item_id")
	if err != nil {
		return "", err
	}
	files, err := m.ListLibraryItemFiles(ctx, itemID)
	if err != nil {
		return "", fmt.Errorf("failed to list library item files: %w", err)
	}
	return marshalJSON(map[string]interface{}{"count": len(files), "files": files})
}

func handleLibraryGetLibraryItemFile(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryMiscManager(ctx, client)
	if err != nil {
		return "", err
	}
	itemID, err := libraryMiscRequiredString(args, "library_item_id")
	if err != nil {
		return "", err
	}
	fileName, err := libraryMiscRequiredString(args, "file_name")
	if err != nil {
		return "", err
	}
	file, err := m.GetLibraryItemFile(ctx, itemID, fileName)
	if err != nil {
		return "", fmt.Errorf("failed to get library item file: %w", err)
	}
	return marshalJSON(file)
}

func handleLibraryListLibraryItemStorage(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryMiscManager(ctx, client)
	if err != nil {
		return "", err
	}
	itemID, err := libraryMiscRequiredString(args, "library_item_id")
	if err != nil {
		return "", err
	}
	storage, err := m.ListLibraryItemStorage(ctx, itemID)
	if err != nil {
		return "", fmt.Errorf("failed to list library item storage: %w", err)
	}
	return marshalJSON(map[string]interface{}{"count": len(storage), "storage": storage})
}

func handleLibraryGetLibraryItemStorage(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryMiscManager(ctx, client)
	if err != nil {
		return "", err
	}
	itemID, err := libraryMiscRequiredString(args, "library_item_id")
	if err != nil {
		return "", err
	}
	fileName, err := libraryMiscRequiredString(args, "file_name")
	if err != nil {
		return "", err
	}
	storage, err := m.GetLibraryItemStorage(ctx, itemID, fileName)
	if err != nil {
		return "", fmt.Errorf("failed to get library item storage: %w", err)
	}
	return marshalJSON(map[string]interface{}{"count": len(storage), "storage": storage})
}

// libraryFindResultKind discriminates a finder.FindResult's underlying
// object via a Go type switch on GetResult() — finder.go has no JSON "type"
// discriminator field of its own (confirmed by reading the source: its only
// marshaling hook is findResult.MarshalJSON, which just re-marshals the
// underlying library.Library/library.Item/library.File directly with no
// wrapper), so this project adds its own "kind" field for callers that need
// to tell the 3 shapes apart without inspecting which fields are present.
func libraryFindResultKind(res finder.FindResult) string {
	switch res.GetResult().(type) {
	case library.Library:
		return "library"
	case library.Item:
		return "item"
	case library.File:
		return "file"
	default:
		return "unknown"
	}
}

func handleLibraryFinderFind(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := libraryMiscManager(ctx, client)
	if err != nil {
		return "", err
	}
	f := finder.NewFinder(m)

	var paths []string
	if raw, ok := args["paths"]; ok && raw != nil {
		paths, err = toStringSlice(raw)
		if err != nil {
			return "", fmt.Errorf("invalid paths: %w", err)
		}
	}

	results, err := f.Find(ctx, paths...)
	if err != nil {
		return "", fmt.Errorf("library find failed: %w", err)
	}

	out := make([]map[string]interface{}, 0, len(results))
	for _, res := range results {
		out = append(out, map[string]interface{}{
			"path":   res.GetPath(),
			"id":     res.GetID(),
			"name":   res.GetName(),
			"kind":   libraryFindResultKind(res),
			"result": res.GetResult(),
		})
	}
	return marshalJSON(map[string]interface{}{"count": len(out), "results": out})
}

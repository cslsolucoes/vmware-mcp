package tools

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// registerGuestWindowsRegistryTools/registerGuestAliasTools add tools for
// the 2 remaining GuestOperationsManager children generated_guest_ops.go
// left out — GuestWindowsRegistryManager (Windows guest registry
// keys/values) and GuestAliasManager (SSO guest alias trust store) — both
// still guest operations executed INSIDE a VM's guest OS via VMware Tools,
// same auth model (guest_username/guest_password ->
// types.NamePasswordAuthentication, reusing guestAuth/guestRequiredArg from
// generated_guest_ops.go, which this file does not edit).
//
// Managed objects / MoRefs (confirmed by reading the real source, not
// assumed):
//
//   - Both are properties of ServiceContent.GuestOperationsManager, exactly
//     like FileManager/ProcessManager: mo.GuestOperationsManager's
//     GuestWindowsRegistryManager and AliasManager fields
//     (*types.ManagedObjectReference — vim25/mo/mo.go). This file
//     re-resolves ServiceContent.GuestOperationsManager and reads those 2
//     properties itself (gwinRegistryManagerRef/galiasManagerRef below)
//     rather than calling generated_guest_ops.go's unexported
//     guestOperationsManagerRefs, per this batch's instruction not to touch
//     that file.
//
//   - Neither GuestWindowsRegistryManager nor GuestAliasManager has an
//     object.* wrapper (confirmed: no object/guest*.go file exists in
//     govmomi at all — only object/vm_guest_test.go, a test file — matching
//     generated_guest_ops.go's finding for GuestFileManager/
//     GuestProcessManager). Every handler below therefore dials the raw
//     vim25 SOAP method directly: methods.Xxx(ctx, client.Client.Client,
//     &types.Xxx{This: ref, ...}).
//
//   - GuestWindowsRegistryManager's 6 real methods (CreateRegistryKeyInGuest,
//     ListRegistryKeysInGuest, DeleteRegistryKeyInGuest,
//     SetRegistryValueInGuest, ListRegistryValuesInGuest,
//     DeleteRegistryValueInGuest) and GuestAliasManager's 5
//     (AddGuestAlias, RemoveGuestAlias, RemoveGuestAliasByCert,
//     ListGuestAliases, ListGuestMappedAliases) were all confirmed present
//     in vim25/methods/methods.go and vim25/types/types.go's
//     <Method>RequestType structs before being wired here — none invented.
//
// Registry value data is a discriminated union (GuestRegValueSpec.Data
// BaseGuestRegValueDataSpec, one of GuestRegValueStringSpec/
// ExpandStringSpec/MultiStringSpec/DwordSpec/QwordSpec/BinarySpec) and
// guest-alias subjects are another (BaseGuestAuthSubject, one of
// GuestAuthAnySubject/GuestAuthNamedSubject). Both are exposed to callers as
// a flat "type"+data argument pair on the way in
// (gwinRegValueDataFromArgs/galiasSubjectFromArgs) and a {"type":...,
// ...} map on the way out (gwinValueDataToJSON/galiasSubjectToJSON) — a
// plain json.Marshal of the Go interface value would silently drop which
// concrete variant it is (the exact discriminator SOAP's xsi:type carries),
// so this file reconstructs that discriminator by hand for the JSON
// response instead.
//
// Tier classification, per the brief: List*/Read* are read-only
// (r.register); every mutation (Create/Delete/Set/Add/Remove) goes through
// registerDestructive, with only the 2 Delete* methods at tier1
// (irreversible — a deleted guest registry key/value cannot be recovered by
// this server) and every other mutation (Create/Set/Add/Remove* — including
// RemoveGuestAlias/RemoveGuestAliasByCert, despite the "Remove" name) at
// tier2 (disruptive but reversible/re-doable: a removed alias can be added
// back, a set value can be set again).
//
// vcsim coverage: NONE. Confirmed by reading
// referencia/govmomi/simulator/guest_operations_manager.go end to end (the
// only file implementing any GuestOperationsManager child in vcsim) — it
// only defines GuestFileManager/GuestProcessManager receiver methods; there
// is no simulator.GuestWindowsRegistryManager or simulator.GuestAliasManager
// type anywhere in govmomi (confirmed by grepping simulator/*.go for
// "registry"/"alias" — the only hits are simulator/registry.go, vcsim's
// unrelated generic MoRef object registry). vcsim's
// GuestOperationsManager.init() (same file) only ever populates
// m.FileManager/m.ProcessManager — GuestWindowsRegistryManager and
// AliasManager are left nil forever. Every handler below resolves the
// containing GuestOperationsManager via a real property-collector round
// trip (so it DOES reach vcsim's server), then hits a clean local nil-guard
// error once the specific child ref comes back nil — never a raw SOAP call
// against either manager. generated_guest_registry_test.go therefore drives
// every one of these 11 tools with assertReachesServer (same helper
// generated_vm_lifecycle_test.go defines for its own unsimulated methods),
// not a success-path assertion — only a real vCenter/ESXi with VMware Tools
// running in an actual Windows guest (for the registry tools) or with a
// configured guest alias store (for the alias tools) can validate true
// end-to-end behavior.
func registerGuestWindowsRegistryTools(r *Registry) {
	vmArg := map[string]interface{}{
		"type":        "string",
		"description": `VM identifier: a name/pattern (e.g. "cac-WN02") or a full inventory path (e.g. "/ha-datacenter/vm/cac-WN02") as returned by vmware_list_vms. Must resolve to exactly one VM. VMware Tools must be running in its guest OS (a Windows guest — these tools call the Windows-only GuestWindowsRegistryManager) for any of these tools to succeed.`,
	}
	guestUsernameArg := map[string]interface{}{
		"type":        "string",
		"description": "Username of a valid account inside the guest OS to authenticate the guest operation as. Not this server's vCenter/ESXi credentials — a separate, guest-OS-local (or domain, if the guest is joined) account.",
	}
	guestPasswordArg := map[string]interface{}{
		"type":        "string",
		"description": "Password for guest_username.",
	}
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}
	registryPathArg := map[string]interface{}{
		"type":        "string",
		"description": `Full path to the registry key, e.g. "HKEY_LOCAL_MACHINE\\SOFTWARE\\MyApp".`,
	}
	wowBitnessArg := map[string]interface{}{
		"type":        "string",
		"enum":        []interface{}{"WOWNative", "WOW32", "WOW64"},
		"description": `Which view of the Windows registry to access: "WOWNative" (default if omitted) — the native view (32-bit on a 32-bit guest, 64-bit on a 64-bit guest); "WOW32" — force the 32-bit view; "WOW64" — force the 64-bit view. See MSDN's WOW64 registry redirection docs.`,
	}
	matchPatternArg := map[string]interface{}{
		"type":        "string",
		"description": `Perl-compatible regex filter on names. Default '.*' (everything) if omitted.`,
	}
	valueNameArg := map[string]interface{}{
		"type":        "string",
		"description": `Name of the registry value. Empty string (or omitted) targets the key's unnamed/default value.`,
	}

	// --- Read-only ---------------------------------------------------------

	r.register("vmware_guest_registry_list_keys",
		"List the subkeys of a Windows registry key inside a VM's guest OS, optionally recursively and filtered by name. Requires VMware Tools running in a Windows guest.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm":             vmArg,
				"guest_username": guestUsernameArg,
				"guest_password": guestPasswordArg,
				"registry_path":  registryPathArg,
				"wow_bitness":    wowBitnessArg,
				"recursive":      map[string]interface{}{"type": "boolean", "description": "List every subkey recursively, not just direct children. Default false."},
				"match_pattern":  matchPatternArg,
			},
			"required": []interface{}{"vm", "guest_username", "guest_password", "registry_path"},
		},
		Tool{Handler: handleGwinListKeys},
	)

	r.register("vmware_guest_registry_list_values",
		"List the values under a Windows registry key inside a VM's guest OS, optionally filtered by name. Requires VMware Tools running in a Windows guest.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm":             vmArg,
				"guest_username": guestUsernameArg,
				"guest_password": guestPasswordArg,
				"registry_path":  registryPathArg,
				"wow_bitness":    wowBitnessArg,
				"expand_strings": map[string]interface{}{"type": "boolean", "description": "Expand values with expandable data (e.g. environment variable references) in the result. Default false."},
				"match_pattern":  matchPatternArg,
			},
			"required": []interface{}{"vm", "guest_username", "guest_password", "registry_path"},
		},
		Tool{Handler: handleGwinListValues},
	)

	// --- Tier 2: disruptive but reversible ----------------------------------

	r.registerDestructive("vmware_guest_registry_create_key",
		"Create a Windows registry key inside a VM's guest OS. Reversible via vmware_guest_registry_delete_key. Requires VMware Tools running in a Windows guest.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm":             vmArg,
				"guest_username": guestUsernameArg,
				"guest_password": guestPasswordArg,
				"registry_path":  registryPathArg,
				"wow_bitness":    wowBitnessArg,
				"is_volatile":    map[string]interface{}{"type": "boolean", "description": "Create the key in memory only — it will NOT survive a guest reboot. Default false (persisted to disk)."},
				"class_type":     map[string]interface{}{"type": "string", "description": "User-defined class type for the key. Optional."},
				"confirm":        confirmArg,
			},
			"required": []interface{}{"vm", "guest_username", "guest_password", "registry_path", "confirm"},
		},
		Tool{Handler: handleGwinCreateKey},
	)

	r.registerDestructive("vmware_guest_registry_set_value",
		"Set (create or overwrite) a Windows registry value inside a VM's guest OS. Reversible by setting the previous value back. Requires VMware Tools running in a Windows guest.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm":             vmArg,
				"guest_username": guestUsernameArg,
				"guest_password": guestPasswordArg,
				"registry_path":  registryPathArg,
				"wow_bitness":    wowBitnessArg,
				"value_name":     valueNameArg,
				"value_type": map[string]interface{}{
					"type":        "string",
					"enum":        []interface{}{"string", "expand_string", "multi_string", "dword", "qword", "binary"},
					"description": "The registry value's data type (REG_SZ/REG_EXPAND_SZ/REG_MULTI_SZ/REG_DWORD/REG_QWORD/REG_BINARY, respectively).",
				},
				"value_data": map[string]interface{}{
					"description": `The value's data. Format depends on value_type: a JSON string for "string"/"expand_string", a JSON array of strings for "multi_string", a JSON integer for "dword"/"qword", or a base64-encoded string for "binary". Omit for an empty/no-data value.`,
				},
				"confirm": confirmArg,
			},
			"required": []interface{}{"vm", "guest_username", "guest_password", "registry_path", "value_type", "confirm"},
		},
		Tool{Handler: handleGwinSetValue},
	)

	// --- Tier 1: irreversible ------------------------------------------------

	r.registerDestructive("vmware_guest_registry_delete_key",
		"Delete a Windows registry key inside a VM's guest OS (optionally recursive). Irreversible — this server has no way to recover a deleted guest registry key. Requires VMware Tools running in a Windows guest.",
		tier1,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm":             vmArg,
				"guest_username": guestUsernameArg,
				"guest_password": guestPasswordArg,
				"registry_path":  registryPathArg,
				"wow_bitness":    wowBitnessArg,
				"recursive":      map[string]interface{}{"type": "boolean", "description": "Delete the key along with any subkeys. If false, the key must already have no subkeys. Default false."},
				"confirm":        confirmArg,
			},
			"required": []interface{}{"vm", "guest_username", "guest_password", "registry_path", "confirm"},
		},
		Tool{Handler: handleGwinDeleteKey},
	)

	r.registerDestructive("vmware_guest_registry_delete_value",
		"Delete a Windows registry value inside a VM's guest OS. Irreversible — this server has no way to recover a deleted guest registry value. Requires VMware Tools running in a Windows guest.",
		tier1,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm":             vmArg,
				"guest_username": guestUsernameArg,
				"guest_password": guestPasswordArg,
				"registry_path":  registryPathArg,
				"wow_bitness":    wowBitnessArg,
				"value_name":     valueNameArg,
				"confirm":        confirmArg,
			},
			"required": []interface{}{"vm", "guest_username", "guest_password", "registry_path", "confirm"},
		},
		Tool{Handler: handleGwinDeleteValue},
	)
}

func registerGuestAliasTools(r *Registry) {
	vmArg := map[string]interface{}{
		"type":        "string",
		"description": `VM identifier: a name/pattern (e.g. "cac-WN02") or a full inventory path (e.g. "/ha-datacenter/vm/cac-WN02") as returned by vmware_list_vms. Must resolve to exactly one VM. VMware Tools must be running in its guest OS for any of these tools to succeed.`,
	}
	guestUsernameArg := map[string]interface{}{
		"type":        "string",
		"description": "Username of a valid account inside the guest OS to authenticate the guest operation (this SSO alias-management call itself) as. Not this server's vCenter/ESXi credentials — a separate, guest-OS-local (or domain, if the guest is joined) account.",
	}
	guestPasswordArg := map[string]interface{}{
		"type":        "string",
		"description": "Password for guest_username.",
	}
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}
	usernameArg := map[string]interface{}{
		"type":        "string",
		"description": "The in-guest account the alias mapping applies to.",
	}
	base64CertArg := map[string]interface{}{
		"type":        "string",
		"description": "X.509 certificate, in base64-encoded DER format, associated with the alias.",
	}
	subjectTypeArg := map[string]interface{}{
		"type":        "string",
		"enum":        []interface{}{"any", "named"},
		"description": `The alias subject's kind: "any" (default if omitted) — any vSphere user authenticated by this alias's certificate may impersonate the in-guest user; "named" — only the specific vSphere user given in subject_name may.`,
	}
	subjectNameArg := map[string]interface{}{
		"type":        "string",
		"description": `The vSphere subject name (as it appears in the SAML token). Required when subject_type is "named", ignored otherwise.`,
	}

	// --- Read-only -----------------------------------------------------------

	r.register("vmware_guest_alias_list",
		"List the certificate-store aliases that an in-guest user account trusts, inside a VM's guest OS. Requires VMware Tools running in the guest.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm":             vmArg,
				"guest_username": guestUsernameArg,
				"guest_password": guestPasswordArg,
				"username":       usernameArg,
			},
			"required": []interface{}{"vm", "guest_username", "guest_password", "username"},
		},
		Tool{Handler: handleGaliasList},
	)

	r.register("vmware_guest_alias_list_mapped",
		"List every guest-alias certificate mapping configured inside a VM's guest OS, across all in-guest users. Requires VMware Tools running in the guest.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm":             vmArg,
				"guest_username": guestUsernameArg,
				"guest_password": guestPasswordArg,
			},
			"required": []interface{}{"vm", "guest_username", "guest_password"},
		},
		Tool{Handler: handleGaliasListMapped},
	)

	// --- Tier 2: disruptive but reversible ------------------------------------

	r.registerDestructive("vmware_guest_alias_add",
		"Add a guest alias mapping: let vSphere users authenticated by the given SSO certificate (and, unless subject_type is \"any\", matching subject_name) impersonate an in-guest user account, inside a VM's guest OS. Reversible via vmware_guest_alias_remove/vmware_guest_alias_remove_by_cert. Requires VMware Tools running in the guest.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm":             vmArg,
				"guest_username": guestUsernameArg,
				"guest_password": guestPasswordArg,
				"username":       usernameArg,
				"map_cert":       map[string]interface{}{"type": "boolean", "description": "Map the alias's certificate so future guest operations using it don't need to specify guest_username explicitly. Default false."},
				"base64_cert":    base64CertArg,
				"subject_type":   subjectTypeArg,
				"subject_name":   subjectNameArg,
				"comment":        map[string]interface{}{"type": "string", "description": "Free-form description of the subject, for the operator's own reference. Optional."},
				"confirm":        confirmArg,
			},
			"required": []interface{}{"vm", "guest_username", "guest_password", "username", "base64_cert", "confirm"},
		},
		Tool{Handler: handleGaliasAdd},
	)

	r.registerDestructive("vmware_guest_alias_remove",
		"Remove one guest alias mapping (matching both the certificate and the subject) from an in-guest user account, inside a VM's guest OS. Reversible via vmware_guest_alias_add. Requires VMware Tools running in the guest.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm":             vmArg,
				"guest_username": guestUsernameArg,
				"guest_password": guestPasswordArg,
				"username":       usernameArg,
				"base64_cert":    base64CertArg,
				"subject_type":   subjectTypeArg,
				"subject_name":   subjectNameArg,
				"confirm":        confirmArg,
			},
			"required": []interface{}{"vm", "guest_username", "guest_password", "username", "base64_cert", "confirm"},
		},
		Tool{Handler: handleGaliasRemove},
	)

	r.registerDestructive("vmware_guest_alias_remove_by_cert",
		"Remove EVERY guest alias mapping associated with a certificate (regardless of subject) from an in-guest user account, inside a VM's guest OS. Reversible via vmware_guest_alias_add. Requires VMware Tools running in the guest.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm":             vmArg,
				"guest_username": guestUsernameArg,
				"guest_password": guestPasswordArg,
				"username":       usernameArg,
				"base64_cert":    base64CertArg,
				"confirm":        confirmArg,
			},
			"required": []interface{}{"vm", "guest_username", "guest_password", "username", "base64_cert", "confirm"},
		},
		Tool{Handler: handleGaliasRemoveByCert},
	)
}

// gwinRegistryManagerRef resolves the GuestWindowsRegistryManager MoRef off
// ServiceContent.GuestOperationsManager — see this file's top doc comment
// for why it re-reads the property itself instead of calling
// generated_guest_ops.go's guestOperationsManagerRefs, and for why the nil
// check below fires unconditionally against vcsim (never populated).
func gwinRegistryManagerRef(ctx context.Context, client *vmware.Client) (types.ManagedObjectReference, error) {
	goRef := client.Client.ServiceContent.GuestOperationsManager
	if goRef == nil {
		return types.ManagedObjectReference{}, fmt.Errorf("guest operations are not available on this connection (ServiceContent.GuestOperationsManager is nil)")
	}

	common := object.NewCommon(client.Client.Client, *goRef)
	var gom mo.GuestOperationsManager
	if err := common.Properties(ctx, *goRef, []string{"guestWindowsRegistryManager"}, &gom); err != nil {
		return types.ManagedObjectReference{}, fmt.Errorf("failed to read guest operations manager: %w", err)
	}
	if gom.GuestWindowsRegistryManager == nil {
		return types.ManagedObjectReference{}, fmt.Errorf("this connection's GuestOperationsManager does not expose a guestWindowsRegistryManager (Windows guest registry tools require a real vCenter/ESXi + VMware Tools in a Windows guest — not simulated by vcsim)")
	}
	return *gom.GuestWindowsRegistryManager, nil
}

// galiasManagerRef is gwinRegistryManagerRef's counterpart for
// GuestAliasManager.
func galiasManagerRef(ctx context.Context, client *vmware.Client) (types.ManagedObjectReference, error) {
	goRef := client.Client.ServiceContent.GuestOperationsManager
	if goRef == nil {
		return types.ManagedObjectReference{}, fmt.Errorf("guest operations are not available on this connection (ServiceContent.GuestOperationsManager is nil)")
	}

	common := object.NewCommon(client.Client.Client, *goRef)
	var gom mo.GuestOperationsManager
	if err := common.Properties(ctx, *goRef, []string{"aliasManager"}, &gom); err != nil {
		return types.ManagedObjectReference{}, fmt.Errorf("failed to read guest operations manager: %w", err)
	}
	if gom.AliasManager == nil {
		return types.ManagedObjectReference{}, fmt.Errorf("this connection's GuestOperationsManager does not expose an aliasManager (guest alias tools require a real vCenter/ESXi + VMware Tools in the guest — not simulated by vcsim)")
	}
	return *gom.AliasManager, nil
}

// gwinManager resolves both the target VM and the GuestWindowsRegistryManager
// MoRef in one step — the (vm, ref, error) shape mirrors
// generated_guest_ops.go's guestFileManager/guestProcessManager.
func gwinManager(ctx context.Context, client *vmware.Client, args map[string]interface{}) (*object.VirtualMachine, types.ManagedObjectReference, error) {
	vm, err := resolveVM(ctx, client, args)
	if err != nil {
		return nil, types.ManagedObjectReference{}, err
	}
	ref, err := gwinRegistryManagerRef(ctx, client)
	if err != nil {
		return nil, types.ManagedObjectReference{}, err
	}
	return vm, ref, nil
}

// galiasManager is gwinManager's counterpart for GuestAliasManager.
func galiasManager(ctx context.Context, client *vmware.Client, args map[string]interface{}) (*object.VirtualMachine, types.ManagedObjectReference, error) {
	vm, err := resolveVM(ctx, client, args)
	if err != nil {
		return nil, types.ManagedObjectReference{}, err
	}
	ref, err := galiasManagerRef(ctx, client)
	if err != nil {
		return nil, types.ManagedObjectReference{}, err
	}
	return vm, ref, nil
}

// gwinKeyNameSpec builds a types.GuestRegKeyNameSpec from the required
// registry_path and optional wow_bitness arguments shared by every registry
// tool — wow_bitness defaults to WOWNative (matching the API's own
// recommended default view) when omitted.
func gwinKeyNameSpec(args map[string]interface{}) (types.GuestRegKeyNameSpec, error) {
	path, err := guestRequiredArg(args, "registry_path")
	if err != nil {
		return types.GuestRegKeyNameSpec{}, err
	}
	wow, _ := args["wow_bitness"].(string)
	if wow == "" {
		wow = string(types.GuestRegKeyWowSpecWOWNative)
	}
	return types.GuestRegKeyNameSpec{RegistryPath: path, WowBitness: wow}, nil
}

// gwinValueNameSpec builds a types.GuestRegValueNameSpec — the key name plus
// an optional value_name (empty legitimately means the key's unnamed/default
// value, per SetRegistryValueInGuest/DeleteRegistryValueInGuest's own docs,
// so this does not require it the way guestRequiredArg would).
func gwinValueNameSpec(args map[string]interface{}) (types.GuestRegValueNameSpec, error) {
	keyName, err := gwinKeyNameSpec(args)
	if err != nil {
		return types.GuestRegValueNameSpec{}, err
	}
	name, _ := args["value_name"].(string)
	return types.GuestRegValueNameSpec{KeyName: keyName, Name: name}, nil
}

// gwinRegValueDataFromArgs builds the concrete BaseGuestRegValueDataSpec
// implementation named by the required value_type argument — see this
// file's top doc comment for why the union is exposed as a flat
// type+value_data pair instead of a nested polymorphic JSON object.
func gwinRegValueDataFromArgs(args map[string]interface{}) (types.BaseGuestRegValueDataSpec, error) {
	valueType, err := guestRequiredArg(args, "value_type")
	if err != nil {
		return nil, err
	}
	raw, hasData := args["value_data"]

	switch valueType {
	case "dword":
		if !hasData {
			return nil, fmt.Errorf("value_data is required for value_type \"dword\"")
		}
		n, err := toInt32(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid value_data for dword: %w", err)
		}
		return &types.GuestRegValueDwordSpec{Value: n}, nil
	case "qword":
		if !hasData {
			return nil, fmt.Errorf("value_data is required for value_type \"qword\"")
		}
		n, err := toInt64(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid value_data for qword: %w", err)
		}
		return &types.GuestRegValueQwordSpec{Value: n}, nil
	case "string":
		s, _ := raw.(string)
		return &types.GuestRegValueStringSpec{Value: s}, nil
	case "expand_string":
		s, _ := raw.(string)
		return &types.GuestRegValueExpandStringSpec{Value: s}, nil
	case "multi_string":
		if !hasData {
			return &types.GuestRegValueMultiStringSpec{}, nil
		}
		arr, err := toStringSlice(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid value_data for multi_string: %w", err)
		}
		return &types.GuestRegValueMultiStringSpec{Value: arr}, nil
	case "binary":
		s, _ := raw.(string)
		if s == "" {
			return &types.GuestRegValueBinarySpec{}, nil
		}
		b, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			return nil, fmt.Errorf("invalid value_data for binary (must be base64-encoded): %w", err)
		}
		return &types.GuestRegValueBinarySpec{Value: b}, nil
	default:
		return nil, fmt.Errorf("invalid value_type %q: must be one of string, expand_string, multi_string, dword, qword, binary", valueType)
	}
}

// gwinValueDataToJSON converts a BaseGuestRegValueDataSpec into a stable
// {"type":..., "value":...} map for the JSON response — see this file's top
// doc comment for why a plain json.Marshal of the interface would lose the
// discriminator.
func gwinValueDataToJSON(data types.BaseGuestRegValueDataSpec) interface{} {
	switch v := data.(type) {
	case *types.GuestRegValueDwordSpec:
		return map[string]interface{}{"type": "dword", "value": v.Value}
	case *types.GuestRegValueQwordSpec:
		return map[string]interface{}{"type": "qword", "value": v.Value}
	case *types.GuestRegValueStringSpec:
		return map[string]interface{}{"type": "string", "value": v.Value}
	case *types.GuestRegValueExpandStringSpec:
		return map[string]interface{}{"type": "expand_string", "value": v.Value}
	case *types.GuestRegValueMultiStringSpec:
		return map[string]interface{}{"type": "multi_string", "value": v.Value}
	case *types.GuestRegValueBinarySpec:
		return map[string]interface{}{"type": "binary", "value": base64.StdEncoding.EncodeToString(v.Value)}
	case nil:
		return nil
	default:
		return map[string]interface{}{"type": fmt.Sprintf("%T", data)}
	}
}

// galiasSubjectFromArgs builds the concrete BaseGuestAuthSubject
// implementation named by the optional subject_type argument (default
// "any") — shared by vmware_guest_alias_add/vmware_guest_alias_remove.
func galiasSubjectFromArgs(args map[string]interface{}) (types.BaseGuestAuthSubject, error) {
	subjectType, _ := args["subject_type"].(string)
	if subjectType == "" {
		subjectType = "any"
	}
	switch subjectType {
	case "any":
		return &types.GuestAuthAnySubject{}, nil
	case "named":
		name, err := guestRequiredArg(args, "subject_name")
		if err != nil {
			return nil, fmt.Errorf("subject_name is required when subject_type is \"named\": %w", err)
		}
		return &types.GuestAuthNamedSubject{Name: name}, nil
	default:
		return nil, fmt.Errorf("invalid subject_type %q: must be \"any\" or \"named\"", subjectType)
	}
}

// galiasSubjectToJSON is galiasSubjectFromArgs' inverse, for
// ListGuestAliases/ListGuestMappedAliases responses.
func galiasSubjectToJSON(s types.BaseGuestAuthSubject) interface{} {
	switch v := s.(type) {
	case *types.GuestAuthNamedSubject:
		return map[string]interface{}{"type": "named", "name": v.Name}
	case *types.GuestAuthAnySubject:
		return map[string]interface{}{"type": "any"}
	case nil:
		return nil
	default:
		return map[string]interface{}{"type": fmt.Sprintf("%T", s)}
	}
}

func handleGwinListKeys(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, ref, err := gwinManager(ctx, client, args)
	if err != nil {
		return "", err
	}
	auth, err := guestAuth(args)
	if err != nil {
		return "", err
	}
	keyName, err := gwinKeyNameSpec(args)
	if err != nil {
		return "", err
	}
	recursive, _ := args["recursive"].(bool)
	matchPattern, _ := args["match_pattern"].(string)

	resp, err := methods.ListRegistryKeysInGuest(ctx, client.Client.Client, &types.ListRegistryKeysInGuest{
		This:         ref,
		Vm:           vm.Reference(),
		Auth:         auth,
		KeyName:      keyName,
		Recursive:    recursive,
		MatchPattern: matchPattern,
	})
	if err != nil {
		return "", fmt.Errorf("failed to list registry keys under %s on %s: %w", keyName.RegistryPath, vm.InventoryPath, err)
	}

	keys := make([]map[string]interface{}, 0, len(resp.Returnval))
	for _, rec := range resp.Returnval {
		entry := map[string]interface{}{
			"registry_path": rec.Key.KeyName.RegistryPath,
			"wow_bitness":   rec.Key.KeyName.WowBitness,
			"class_type":    rec.Key.ClassType,
			"last_written":  rec.Key.LastWritten,
		}
		if rec.Fault != nil {
			entry["fault"] = rec.Fault.LocalizedMessage
		}
		keys = append(keys, entry)
	}

	return marshalJSON(map[string]interface{}{
		"vm":            vm.InventoryPath,
		"registry_path": keyName.RegistryPath,
		"count":         len(keys),
		"keys":          keys,
	})
}

func handleGwinListValues(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, ref, err := gwinManager(ctx, client, args)
	if err != nil {
		return "", err
	}
	auth, err := guestAuth(args)
	if err != nil {
		return "", err
	}
	keyName, err := gwinKeyNameSpec(args)
	if err != nil {
		return "", err
	}
	expandStrings, _ := args["expand_strings"].(bool)
	matchPattern, _ := args["match_pattern"].(string)

	resp, err := methods.ListRegistryValuesInGuest(ctx, client.Client.Client, &types.ListRegistryValuesInGuest{
		This:          ref,
		Vm:            vm.Reference(),
		Auth:          auth,
		KeyName:       keyName,
		ExpandStrings: expandStrings,
		MatchPattern:  matchPattern,
	})
	if err != nil {
		return "", fmt.Errorf("failed to list registry values under %s on %s: %w", keyName.RegistryPath, vm.InventoryPath, err)
	}

	values := make([]map[string]interface{}, 0, len(resp.Returnval))
	for _, v := range resp.Returnval {
		values = append(values, map[string]interface{}{
			"value_name": v.Name.Name,
			"data":       gwinValueDataToJSON(v.Data),
		})
	}

	return marshalJSON(map[string]interface{}{
		"vm":            vm.InventoryPath,
		"registry_path": keyName.RegistryPath,
		"count":         len(values),
		"values":        values,
	})
}

func handleGwinCreateKey(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, ref, err := gwinManager(ctx, client, args)
	if err != nil {
		return "", err
	}
	auth, err := guestAuth(args)
	if err != nil {
		return "", err
	}
	keyName, err := gwinKeyNameSpec(args)
	if err != nil {
		return "", err
	}
	isVolatile, _ := args["is_volatile"].(bool)
	classType, _ := args["class_type"].(string)

	if _, err := methods.CreateRegistryKeyInGuest(ctx, client.Client.Client, &types.CreateRegistryKeyInGuest{
		This:       ref,
		Vm:         vm.Reference(),
		Auth:       auth,
		KeyName:    keyName,
		IsVolatile: isVolatile,
		ClassType:  classType,
	}); err != nil {
		return "", fmt.Errorf("failed to create registry key %s on %s: %w", keyName.RegistryPath, vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{
		"vm":            vm.InventoryPath,
		"registry_path": keyName.RegistryPath,
		"wow_bitness":   keyName.WowBitness,
		"result":        "registry_key_created",
	})
}

func handleGwinSetValue(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, ref, err := gwinManager(ctx, client, args)
	if err != nil {
		return "", err
	}
	auth, err := guestAuth(args)
	if err != nil {
		return "", err
	}
	nameSpec, err := gwinValueNameSpec(args)
	if err != nil {
		return "", err
	}
	data, err := gwinRegValueDataFromArgs(args)
	if err != nil {
		return "", err
	}

	if _, err := methods.SetRegistryValueInGuest(ctx, client.Client.Client, &types.SetRegistryValueInGuest{
		This: ref,
		Vm:   vm.Reference(),
		Auth: auth,
		Value: types.GuestRegValueSpec{
			Name: nameSpec,
			Data: data,
		},
	}); err != nil {
		return "", fmt.Errorf("failed to set registry value %q under %s on %s: %w", nameSpec.Name, nameSpec.KeyName.RegistryPath, vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{
		"vm":            vm.InventoryPath,
		"registry_path": nameSpec.KeyName.RegistryPath,
		"value_name":    nameSpec.Name,
		"result":        "registry_value_set",
	})
}

func handleGwinDeleteKey(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, ref, err := gwinManager(ctx, client, args)
	if err != nil {
		return "", err
	}
	auth, err := guestAuth(args)
	if err != nil {
		return "", err
	}
	keyName, err := gwinKeyNameSpec(args)
	if err != nil {
		return "", err
	}
	recursive, _ := args["recursive"].(bool)

	if _, err := methods.DeleteRegistryKeyInGuest(ctx, client.Client.Client, &types.DeleteRegistryKeyInGuest{
		This:      ref,
		Vm:        vm.Reference(),
		Auth:      auth,
		KeyName:   keyName,
		Recursive: recursive,
	}); err != nil {
		return "", fmt.Errorf("failed to delete registry key %s on %s: %w", keyName.RegistryPath, vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{
		"vm":            vm.InventoryPath,
		"registry_path": keyName.RegistryPath,
		"recursive":     recursive,
		"result":        "registry_key_deleted",
	})
}

func handleGwinDeleteValue(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, ref, err := gwinManager(ctx, client, args)
	if err != nil {
		return "", err
	}
	auth, err := guestAuth(args)
	if err != nil {
		return "", err
	}
	nameSpec, err := gwinValueNameSpec(args)
	if err != nil {
		return "", err
	}

	if _, err := methods.DeleteRegistryValueInGuest(ctx, client.Client.Client, &types.DeleteRegistryValueInGuest{
		This:      ref,
		Vm:        vm.Reference(),
		Auth:      auth,
		ValueName: nameSpec,
	}); err != nil {
		return "", fmt.Errorf("failed to delete registry value %q under %s on %s: %w", nameSpec.Name, nameSpec.KeyName.RegistryPath, vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{
		"vm":            vm.InventoryPath,
		"registry_path": nameSpec.KeyName.RegistryPath,
		"value_name":    nameSpec.Name,
		"result":        "registry_value_deleted",
	})
}

func handleGaliasList(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, ref, err := galiasManager(ctx, client, args)
	if err != nil {
		return "", err
	}
	auth, err := guestAuth(args)
	if err != nil {
		return "", err
	}
	username, err := guestRequiredArg(args, "username")
	if err != nil {
		return "", err
	}

	resp, err := methods.ListGuestAliases(ctx, client.Client.Client, &types.ListGuestAliases{
		This:     ref,
		Vm:       vm.Reference(),
		Auth:     auth,
		Username: username,
	})
	if err != nil {
		return "", fmt.Errorf("failed to list guest aliases for %s on %s: %w", username, vm.InventoryPath, err)
	}

	certStores := make([]map[string]interface{}, 0, len(resp.Returnval))
	for _, ga := range resp.Returnval {
		aliases := make([]map[string]interface{}, 0, len(ga.Aliases))
		for _, a := range ga.Aliases {
			aliases = append(aliases, map[string]interface{}{
				"subject": galiasSubjectToJSON(a.Subject),
				"comment": a.Comment,
			})
		}
		certStores = append(certStores, map[string]interface{}{
			"base64_cert": ga.Base64Cert,
			"aliases":     aliases,
		})
	}

	return marshalJSON(map[string]interface{}{
		"vm":          vm.InventoryPath,
		"username":    username,
		"count":       len(certStores),
		"cert_stores": certStores,
	})
}

func handleGaliasListMapped(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, ref, err := galiasManager(ctx, client, args)
	if err != nil {
		return "", err
	}
	auth, err := guestAuth(args)
	if err != nil {
		return "", err
	}

	resp, err := methods.ListGuestMappedAliases(ctx, client.Client.Client, &types.ListGuestMappedAliases{
		This: ref,
		Vm:   vm.Reference(),
		Auth: auth,
	})
	if err != nil {
		return "", fmt.Errorf("failed to list mapped guest aliases on %s: %w", vm.InventoryPath, err)
	}

	mapped := make([]map[string]interface{}, 0, len(resp.Returnval))
	for _, ma := range resp.Returnval {
		subjects := make([]interface{}, 0, len(ma.Subjects))
		for _, s := range ma.Subjects {
			subjects = append(subjects, galiasSubjectToJSON(s))
		}
		mapped = append(mapped, map[string]interface{}{
			"base64_cert": ma.Base64Cert,
			"username":    ma.Username,
			"subjects":    subjects,
		})
	}

	return marshalJSON(map[string]interface{}{
		"vm":             vm.InventoryPath,
		"count":          len(mapped),
		"mapped_aliases": mapped,
	})
}

func handleGaliasAdd(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, ref, err := galiasManager(ctx, client, args)
	if err != nil {
		return "", err
	}
	auth, err := guestAuth(args)
	if err != nil {
		return "", err
	}
	username, err := guestRequiredArg(args, "username")
	if err != nil {
		return "", err
	}
	base64Cert, err := guestRequiredArg(args, "base64_cert")
	if err != nil {
		return "", err
	}
	subject, err := galiasSubjectFromArgs(args)
	if err != nil {
		return "", err
	}
	mapCert, _ := args["map_cert"].(bool)
	comment, _ := args["comment"].(string)

	if _, err := methods.AddGuestAlias(ctx, client.Client.Client, &types.AddGuestAlias{
		This:       ref,
		Vm:         vm.Reference(),
		Auth:       auth,
		Username:   username,
		MapCert:    mapCert,
		Base64Cert: base64Cert,
		AliasInfo: types.GuestAuthAliasInfo{
			Subject: subject,
			Comment: comment,
		},
	}); err != nil {
		return "", fmt.Errorf("failed to add guest alias for %s on %s: %w", username, vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{
		"vm":       vm.InventoryPath,
		"username": username,
		"map_cert": mapCert,
		"result":   "alias_added",
	})
}

func handleGaliasRemove(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, ref, err := galiasManager(ctx, client, args)
	if err != nil {
		return "", err
	}
	auth, err := guestAuth(args)
	if err != nil {
		return "", err
	}
	username, err := guestRequiredArg(args, "username")
	if err != nil {
		return "", err
	}
	base64Cert, err := guestRequiredArg(args, "base64_cert")
	if err != nil {
		return "", err
	}
	subject, err := galiasSubjectFromArgs(args)
	if err != nil {
		return "", err
	}

	if _, err := methods.RemoveGuestAlias(ctx, client.Client.Client, &types.RemoveGuestAlias{
		This:       ref,
		Vm:         vm.Reference(),
		Auth:       auth,
		Username:   username,
		Base64Cert: base64Cert,
		Subject:    subject,
	}); err != nil {
		return "", fmt.Errorf("failed to remove guest alias for %s on %s: %w", username, vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{
		"vm":       vm.InventoryPath,
		"username": username,
		"result":   "alias_removed",
	})
}

func handleGaliasRemoveByCert(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, ref, err := galiasManager(ctx, client, args)
	if err != nil {
		return "", err
	}
	auth, err := guestAuth(args)
	if err != nil {
		return "", err
	}
	username, err := guestRequiredArg(args, "username")
	if err != nil {
		return "", err
	}
	base64Cert, err := guestRequiredArg(args, "base64_cert")
	if err != nil {
		return "", err
	}

	if _, err := methods.RemoveGuestAliasByCert(ctx, client.Client.Client, &types.RemoveGuestAliasByCert{
		This:       ref,
		Vm:         vm.Reference(),
		Auth:       auth,
		Username:   username,
		Base64Cert: base64Cert,
	}); err != nil {
		return "", fmt.Errorf("failed to remove guest alias by certificate for %s on %s: %w", username, vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{
		"vm":       vm.InventoryPath,
		"username": username,
		"result":   "alias_removed_by_cert",
	})
}

package tools

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// registerHostProfileTools adds vSphere Host Profiles / config-compliance
// tools — object.HostProfileManager's SOAP surface (there is no
// object.HostProfileManager Go wrapper at all — confirmed by grepping
// referencia/govmomi/object for "HostProfileManager"/"ProfileManager", zero
// matches) plus the Profile/HostProfile managed object's own instance
// methods. Every handler below dials the raw vim25 SOAP method directly
// (methods.Xxx(ctx, client.Client.Client, &types.Xxx{This: ref, ...})), the
// same "no object.* wrapper, go straight to methods+types" pattern
// generated_host_iscsi_portbinding.go and generated_vm_ft.go already
// document at length.
//
// Method inventory — confirmed REAL by grepping
// referencia/govmomi/vim25/methods/methods.go (and cross-checked in the
// actual GOMODCACHE govmomi@v0.55.1 copy Go build resolves against, not just
// the vendored referencia/ copy) for a "func Xxx(ctx context.Context, r
// soap.RoundTripper, req *types.Xxx)" line before writing any handler for it.
// Some names from the task brief do not exist under that exact spelling and
// are DROPPED (not invented): "QueryProfileMetadata" -> real name is
// QueryHostProfileMetadata; "QueryProfileMetadataForField" -> no such method
// anywhere in methods.go, dropped; "CheckHostProfileCompatibility" -> no such
// method, dropped; "ReconfigureHostProfile" -> no such method, dropped
// (UpdateHostProfile is the real, and only, profile-content-update method).
// "CompositeHostProfile" is real as CompositeHostProfile_Task.
//
// HostProfileManager-level (This = ServiceContent.HostProfileManager,
// resolved by hprofManagerRef — 13 tools): CreateProfile, QueryHostProfileMetadata,
// FindAssociatedProfile, ApplyHostConfig_Task, GenerateConfigTaskList,
// GenerateHostProfileTaskList_Task, CheckAnswerFileStatus_Task,
// QueryAnswerFileStatus, UpdateAnswerFile_Task, RetrieveAnswerFile,
// RetrieveAnswerFileForProfile, ExportAnswerFile_Task, CompositeHostProfile_Task.
//
// Profile/HostProfile-level (This = a Profile MoRef, resolved by name via
// hprofResolveProfile — 8 tools): RetrieveDescription,
// CheckProfileCompliance_Task, ExportProfile, DestroyProfile,
// AssociateProfile, DissociateProfile, UpdateHostProfile, ExecuteHostProfile.
//
// Profile resolution: Profile is NOT part of the inventory Folder tree (no
// Finder support, no vmware_list_* equivalent) — it lives only in
// HostProfileManager.profile[]. hprofResolveProfile therefore reads that
// list (mo.HostProfileManager via object.NewCommon(...).Properties, the same
// bare-MoRef property-read pattern generated_license.go's
// licenseAssignmentManagerRef already established for a manager with no
// object.* wrapper) then batch-reads each candidate's mo.Profile.Name via
// property.DefaultCollector(...).Retrieve (same batch-read pattern
// generated_alarm.go's alarm-info helper uses) and matches by EXACT name —
// not the name/pattern glob semantics resolveVM/resolveHost use elsewhere in
// this project, because vSphere enforces Profile.name uniqueness at
// CreateProfile time, so exact match is both correct and simpler.
//
// Class: modeVCenterOnly. Confirmed structurally, not just by convention:
// referencia's vendored copy AND the real GOMODCACHE govmomi@v0.55.1 copy
// both show simulator/esx/service_content.go sets
// ServiceContent.HostProfileManager to a nil *types.ManagedObjectReference,
// while simulator/vpx/service_content.go sets it to a populated (but
// UNBACKED — see below) MoRef {Type: "HostProfileManager", Value:
// "HostProfileManager"}. A standalone ESXi connection therefore has no
// HostProfileManager at all; hprofManagerRef's nil-guard reports a clear
// error instead of ever constructing a zero-value MoRef and letting a raw
// SOAP call reach the wire with it.
//
// vcsim coverage: NONE. grepping the entire simulator/ package (both
// referencia/govmomi/simulator and the real module cache copy) for any
// receiver method on a Profile/HostProfile/ProfileManager/HostProfileManager
// type, or any func registering one via simulator.Map's content-type
// dispatch, finds zero matches — there is no simulator-side implementation
// for a single one of vim.profile.*'s SOAP operations. vcsim's VPX() model
// DOES populate ServiceContent.HostProfileManager with a MoRef (so
// hprofManagerRef's nil-guard passes and every raw SOAP call actually
// reaches the wire) but registers no backing simulator object for that
// MoRef — the exact same "populated-but-unbacked manager MoRef" situation
// generated_host_iscsi_portbinding.go documents for IscsiManager. Every call
// through this file, including hprofResolveProfile's own property read
// against the HostProfileManager MoRef, therefore reaches vcsim's dispatcher
// and comes back with a clean ManagedObjectNotFound/MethodNotFound-shaped
// fault — never an "unknown tool" (wiring bug) or a recovered panic. This
// file's test file drives every one of the 21 tools with assertReachesServer
// for exactly that reason, proving the wiring (schema, tier gate,
// resolveHost/hprofResolveProfile, raw SOAP dispatch) reaches vcsim, not
// behavioral correctness of a real host-profile compliance run — that is
// expected against a real vCenter with Host Profiles licensed and at least
// one profile created through the UI or vmware_host_profile_create.
//
// Polymorphic-field limitations (documented, not solved — same posture
// generated_customization_spec.go's top doc comment already established for
// types.CustomizationSpec's spec.identity/spec.options): three fields in
// this API surface are Go interfaces decoded via this project's generic
// decodeJSONArg (plain encoding/json, no vim25 xml/typeattr type-registry
// resolution), so only their SIMPLEST concrete shape round-trips reliably:
//  1. HostConfigSpec.Option ([]types.BaseOptionValue) and
//     .UserAccount/.UsergroupAccount ([]types.BaseHostAccountSpec), used by
//     vmware_host_profile_apply_config/generate_config_task_list/
//     generate_task_list's "config_spec" argument — omit those 3 sub-fields
//     or the call fails with a clear decode error naming the field.
//  2. types.HostApplyProfile (used by vmware_host_profile_composite's
//     to_be_merged/to_be_replaced_with/to_be_deleted/
//     enable_status_to_be_copied, and vmware_host_profile_answerfile_
//     retrieve_for_profile's apply_profile) is the ENTIRE host configuration
//     policy tree — dozens of nested sub-profiles, several themselves
//     polymorphic Policy variants several levels deep. This generic decoder
//     can only round-trip a HostApplyProfile JSON blob whose nested policies
//     stick to concrete (non-typeattr) fields; a tree containing a
//     polymorphic Policy sub-object fails with a clear decode error. The
//     realistic source of a full HostApplyProfile is reading an existing
//     profile's config.applyProfile property directly (not exposed as its
//     own tool here — none of the 21 confirmed-real methods is a plain
//     "get profile config" accessor) and forwarding it verbatim.
//  3. types.HostProfileHostBasedConfigSpec (used by
//     vmware_host_profile_create's and vmware_host_profile_update's
//     create/config spec) is built field-by-field from friendly top-level
//     arguments (name/annotation/enabled/host/use_host_profile_engine)
//     instead of accepting free-form JSON — it has NO interior polymorphism
//     of its own (Host is a plain ManagedObjectReference, everything else is
//     a scalar), so building it directly sidesteps both the JSON-into-
//     interface problem AND the "MoRef inside free-form JSON" awkwardness,
//     the same "resolve friendly args, build the concrete struct by hand"
//     approach generated_vm_ft.go's handleVMCreateSecondaryVM already uses
//     for its optional Host *ManagedObjectReference field. This is also the
//     realistic, most common real-world CreateProfile/UpdateHostProfile
//     shape (extract/re-extract a host profile FROM a reference host) —
//     types.ProfileSerializedCreateSpec (string-serialized profile) and
//     types.HostProfileCompleteConfigSpec (raw ApplyProfile tree) are the
//     other real concrete variants but are not exposed here.
//
// Tier reasoning: DestroyProfile is tier1 (irreversible — permanently
// deletes the profile object and its associations). CreateProfile,
// ApplyHostConfig_Task (applies a config diff to a live host — disruptive
// but the host can be reconfigured again), UpdateAnswerFile_Task,
// CompositeHostProfile_Task (can remove content from target profiles via
// ToBeDeleted, but the targets themselves survive and can be re-composed),
// AssociateProfile/DissociateProfile (each reverses the other), and
// UpdateHostProfile (overwrites profile content, but content can be updated
// again) are all tier2. Every other tool here is a pure read/plan/export
// operation with no tier: QueryHostProfileMetadata, FindAssociatedProfile,
// GenerateConfigTaskList, GenerateHostProfileTaskList_Task (computes a task
// list, does not execute it), CheckAnswerFileStatus_Task,
// QueryAnswerFileStatus, RetrieveAnswerFile, RetrieveAnswerFileForProfile,
// ExportAnswerFile_Task, RetrieveDescription, CheckProfileCompliance_Task
// (a compliance CHECK, not a remediation — ApplyHostConfig_Task is the
// separate, tier2, actually-mutating step), ExportProfile, and
// ExecuteHostProfile (per its own doc comment in types.go, this "compares
// the host configuration to the profile" and returns a proposed configSpec —
// a dry-run/plan step; ApplyHostConfig_Task is what actually applies it to
// the host).
func registerHostProfileTools(r *Registry) {
	profileArg := map[string]interface{}{
		"type":        "string",
		"description": `Host profile identifier: its exact "name" (as returned by vmware_host_profile_query_metadata or the "name" you gave vmware_host_profile_create), unique per connection. Profiles are not part of the inventory Folder tree — resolved by scanning HostProfileManager.profile[] and matching Profile.name exactly (case-sensitive, no glob support).`,
	}
	hostArg := map[string]interface{}{
		"type":        "string",
		"description": `Host identifier: a name/pattern (e.g. "esxi-01.local") or a full inventory path as returned by vmware_list_hosts. Must resolve to exactly one host.`,
	}
	hostsArg := map[string]interface{}{
		"type":        "array",
		"items":       map[string]interface{}{"type": "string"},
		"description": `Host identifiers (name/pattern or full inventory path, same resolution rules as "host"), e.g. ["esxi-01.local", "esxi-02.local"].`,
	}
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}
	entityPathArg := map[string]interface{}{
		"type":        "string",
		"description": `Full inventory path of a managed entity (e.g. "/DC0/host/cluster1/esxi-01.local" for a host, "/DC0/host/cluster1" for a cluster), resolved via SearchIndex.FindByInventoryPath — works for any entity kind a profile can be associated with.`,
	}
	entityPathsArg := map[string]interface{}{
		"type":        "array",
		"items":       entityPathArg,
		"description": "Managed entity inventory paths.",
	}
	configSpecArg := map[string]interface{}{
		"type":        "object",
		"description": `A types.HostConfigSpec JSON object matching its Go struct field names (e.g. "network", "service", "firewall", "datetime", "security", "memory"). LIMITATION (see this file's top doc comment): "option", "userAccount", and "usergroupAccount" are polymorphic (typeattr) array fields this generic decoder cannot populate — omit all 3 or the call fails with a clear decode error naming the offending field. Typically obtained from a prior vmware_host_profile_execute call's "configSpec" result.`,
	}
	userInputArg := map[string]interface{}{
		"type": "array",
		"items": map[string]interface{}{
			"type":        "object",
			"description": `A types.ProfileDeferredPolicyOptionParameter JSON object: "inputPath" (object: "profilePath" required string, plus optional "policyId"/"parameterId"/"policyOptionId") and optional "parameter" (array of {"key": string, "value": <any JSON value>}).`,
		},
		"description": "Host-specific deferred parameter values, as returned by vmware_host_profile_execute's requireInput/result.",
	}
	applyProfileArg := map[string]interface{}{
		"type":        "object",
		"description": `A types.HostApplyProfile JSON object — the full host configuration policy tree. LIMITATION (see this file's top doc comment): this is a very large, partially polymorphic structure; only sub-trees using concrete (non-typeattr) policy fields decode reliably. Omit entirely for no changes of that kind.`,
	}

	// === HostProfileManager-level ========================================

	r.registerDestructive("vmware_host_profile_create",
		"Create a new host profile by extracting configuration from a reference host.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name":                    map[string]interface{}{"type": "string", "description": "Name for the new profile. Must be unique."},
				"host":                    hostArg,
				"annotation":              map[string]interface{}{"type": "string", "description": "Optional user-provided description of the profile."},
				"enabled":                 map[string]interface{}{"type": "boolean", "description": "Whether the profile is enabled. Defaults to server behavior if omitted."},
				"use_host_profile_engine": map[string]interface{}{"type": "boolean", "description": "If true, use the vSphere 5.0+ profile plug-ins (not compatible with legacy pre-5.0 hosts). If false/omitted, creates a legacy host profile."},
				"confirm":                 confirmArg,
			},
			"required": []interface{}{"name", "host", "confirm"},
		},
		Tool{Handler: handleHostProfileCreate},
	)

	r.register("vmware_host_profile_query_metadata",
		"Get policy/parameter metadata for one or more host profiles (or every profile visible on this connection if profile_names is omitted).",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"profile_names": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Profile names to get metadata for. Omit for metadata on every profile.",
				},
				"profile": map[string]interface{}{
					"type":        "string",
					"description": "Optional base profile (by name) whose context should be used while computing metadata.",
				},
			},
		},
		Tool{Handler: handleHostProfileQueryMetadata},
	)

	r.register("vmware_host_profile_find_associated",
		"Find every host profile associated with a managed entity (typically a host or cluster).",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"entity_path": entityPathArg},
			"required":   []interface{}{"entity_path"},
		},
		Tool{Handler: handleHostProfileFindAssociated},
	)

	r.registerDestructive("vmware_host_profile_apply_config",
		"Apply a configuration spec (as produced by vmware_host_profile_execute) to a host. This is the step that actually mutates host configuration — vmware_host_profile_execute only computes what WOULD change.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":        hostArg,
				"config_spec": configSpecArg,
				"user_input":  userInputArg,
				"confirm":     confirmArg,
			},
			"required": []interface{}{"host", "config_spec", "confirm"},
		},
		Tool{Handler: handleHostProfileApplyConfig},
	)

	r.register("vmware_host_profile_generate_config_task_list",
		"Compute the list of configuration tasks (HostProfileManagerConfigTaskList) that would be needed to apply a config spec to a host, without applying anything. Pure computation, no server-side mutation.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":        hostArg,
				"config_spec": configSpecArg,
			},
			"required": []interface{}{"host", "config_spec"},
		},
		Tool{Handler: handleHostProfileGenerateConfigTaskList},
	)

	r.register("vmware_host_profile_generate_task_list",
		"Like vmware_host_profile_generate_config_task_list, but via the Task-based GenerateHostProfileTaskList_Task variant of the same computation. Pure computation, no server-side mutation.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":        hostArg,
				"config_spec": configSpecArg,
			},
			"required": []interface{}{"host", "config_spec"},
		},
		Tool{Handler: handleHostProfileGenerateTaskList},
	)

	r.register("vmware_host_profile_answerfile_check_status",
		"Check the answer-file status (e.g. whether host-specific input is still required) for one or more hosts. Read-only check, delivered via a Task whose result is returned.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"hosts": hostsArg},
			"required":   []interface{}{"hosts"},
		},
		Tool{Handler: handleHostProfileAnswerFileCheckStatus},
	)

	r.register("vmware_host_profile_answerfile_query_status",
		"Query the cached answer-file status for one or more hosts (like vmware_host_profile_answerfile_check_status, but returns the last-known status directly instead of re-checking via a Task).",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"hosts": hostsArg},
			"required":   []interface{}{"hosts"},
		},
		Tool{Handler: handleHostProfileAnswerFileQueryStatus},
	)

	r.registerDestructive("vmware_host_profile_answerfile_update",
		"Update (create or replace) the answer file (host-specific configuration values) associated with a host.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":       hostArg,
				"user_input": userInputArg,
				"validating": map[string]interface{}{"type": "boolean", "description": `If false, the answer file is saved without validation (use with caution). Defaults to true (validated) if omitted.`},
				"confirm":    confirmArg,
			},
			"required": []interface{}{"host", "confirm"},
		},
		Tool{Handler: handleHostProfileAnswerFileUpdate},
	)

	r.register("vmware_host_profile_answerfile_retrieve",
		"Retrieve the answer file currently associated with a host.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"host": hostArg},
			"required":   []interface{}{"host"},
		},
		Tool{Handler: handleHostProfileAnswerFileRetrieve},
	)

	r.register("vmware_host_profile_answerfile_retrieve_for_profile",
		"Generate/retrieve the answer file a host would need for a given (not-yet-associated) profile configuration.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":          hostArg,
				"apply_profile": applyProfileArg,
			},
			"required": []interface{}{"host", "apply_profile"},
		},
		Tool{Handler: handleHostProfileAnswerFileRetrieveForProfile},
	)

	r.register("vmware_host_profile_answerfile_export",
		"Export a host's answer file (as an AnswerFile object) via the Task-based ExportAnswerFile_Task; the task's result is returned.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"host": hostArg},
			"required":   []interface{}{"host"},
		},
		Tool{Handler: handleHostProfileAnswerFileExport},
	)

	r.registerDestructive("vmware_host_profile_composite",
		"Compose a host profile from a source profile onto one or more target profiles (merge/replace/delete specific policy sub-trees, or copy enable-status). Advanced/rarely-needed operation — see this file's top doc comment for the to_be_* arguments' HostApplyProfile decode limitation.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"source": map[string]interface{}{"type": "string", "description": "Source profile name (its content is the basis for the composition)."},
				"targets": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Target profile names the composition is applied to. Omit to target every profile the server considers eligible.",
				},
				"to_be_merged":               applyProfileArg,
				"to_be_replaced_with":        applyProfileArg,
				"to_be_deleted":              applyProfileArg,
				"enable_status_to_be_copied": applyProfileArg,
				"confirm":                    confirmArg,
			},
			"required": []interface{}{"source", "confirm"},
		},
		Tool{Handler: handleHostProfileComposite},
	)

	// === Profile/HostProfile-level ========================================

	r.register("vmware_host_profile_retrieve_description",
		"Get the localized description (summary/detail text) of a host profile.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"profile": profileArg},
			"required":   []interface{}{"profile"},
		},
		Tool{Handler: handleHostProfileRetrieveDescription},
	)

	r.register("vmware_host_profile_check_compliance",
		"Check compliance of one or more entities against a host profile (or every entity currently associated with it, if entity_paths is omitted). Read-only check, delivered via a Task whose result is returned.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"profile":      profileArg,
				"entity_paths": entityPathsArg,
			},
			"required": []interface{}{"profile"},
		},
		Tool{Handler: handleHostProfileCheckCompliance},
	)

	r.register("vmware_host_profile_export",
		"Export a host profile's full configuration as an opaque string (for backup/transfer; re-import is not exposed as a tool here).",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"profile": profileArg},
			"required":   []interface{}{"profile"},
		},
		Tool{Handler: handleHostProfileExport},
	)

	r.registerDestructive("vmware_host_profile_destroy",
		"Permanently delete a host profile and its associations. Irreversible.",
		tier1,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"profile": profileArg, "confirm": confirmArg},
			"required":   []interface{}{"profile", "confirm"},
		},
		Tool{Handler: handleHostProfileDestroy},
	)

	r.registerDestructive("vmware_host_profile_associate",
		"Associate a host profile with one or more managed entities (hosts/clusters). Reversible via vmware_host_profile_dissociate.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"profile":      profileArg,
				"entity_paths": entityPathsArg,
				"confirm":      confirmArg,
			},
			"required": []interface{}{"profile", "entity_paths", "confirm"},
		},
		Tool{Handler: handleHostProfileAssociate},
	)

	r.registerDestructive("vmware_host_profile_dissociate",
		"Remove a host profile's association with one or more managed entities (or with ALL associated entities, if entity_paths is omitted). Reversible via vmware_host_profile_associate.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"profile":      profileArg,
				"entity_paths": entityPathsArg,
				"confirm":      confirmArg,
			},
			"required": []interface{}{"profile", "confirm"},
		},
		Tool{Handler: handleHostProfileDissociate},
	)

	r.registerDestructive("vmware_host_profile_update",
		"Update an existing host profile's content by re-extracting configuration from a (possibly different) reference host.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"profile":                 profileArg,
				"host":                    hostArg,
				"name":                    map[string]interface{}{"type": "string", "description": "Optional new name for the profile."},
				"annotation":              map[string]interface{}{"type": "string", "description": "Optional new description."},
				"enabled":                 map[string]interface{}{"type": "boolean", "description": "Optional new enabled state."},
				"use_host_profile_engine": map[string]interface{}{"type": "boolean", "description": "Same semantics as vmware_host_profile_create's argument of the same name."},
				"confirm":                 confirmArg,
			},
			"required": []interface{}{"profile", "host", "confirm"},
		},
		Tool{Handler: handleHostProfileUpdate},
	)

	r.register("vmware_host_profile_execute",
		"Compare a host's current configuration against a host profile and compute the proposed configSpec/deferred-parameter changes, WITHOUT applying anything (dry run/plan step). Feed the result's configSpec into vmware_host_profile_apply_config to actually apply it.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"profile":         profileArg,
				"host":            hostArg,
				"deferred_params": userInputArg,
			},
			"required": []interface{}{"profile", "host"},
		},
		Tool{Handler: handleHostProfileExecute},
	)
}

// hprofManagerRef resolves ServiceContent.HostProfileManager's MoRef —
// present only on vCenter (nil on a standalone ESXi host, see this file's
// top doc comment) — with the same explicit nil-guard discipline
// generated_license.go's requireLicenseManager / generated_customization_
// spec.go's requireCustomizationSpecManager already establish (registry.go's
// CallTool recover() must not be the only safety net against a
// standalone-ESXi caller reaching a bare nil MoRef).
func hprofManagerRef(client *vmware.Client) (types.ManagedObjectReference, error) {
	ref := client.Client.ServiceContent.HostProfileManager
	if ref == nil {
		return types.ManagedObjectReference{}, fmt.Errorf("host-profile tools are not available on this connection (ServiceContent.HostProfileManager is nil — requires vCenter, not a standalone ESXi host)")
	}
	return *ref, nil
}

// hprofResolveProfile resolves the required "profile" argument (an exact
// Profile.name) to its types.ManagedObjectReference — see this file's top
// doc comment ("Profile resolution") for why this scans
// HostProfileManager.profile[] instead of using find.Finder (Profile is not
// part of the inventory Folder tree).
func hprofResolveProfile(ctx context.Context, client *vmware.Client, args map[string]interface{}) (types.ManagedObjectReference, error) {
	name, _ := args["profile"].(string)
	if name == "" {
		return types.ManagedObjectReference{}, fmt.Errorf("profile is required")
	}

	mgrRef, err := hprofManagerRef(client)
	if err != nil {
		return types.ManagedObjectReference{}, err
	}

	var mgr mo.HostProfileManager
	if err := object.NewCommon(client.Client.Client, mgrRef).Properties(ctx, mgrRef, []string{"profile"}, &mgr); err != nil {
		return types.ManagedObjectReference{}, fmt.Errorf("failed to list host profiles: %w", err)
	}
	if len(mgr.Profile) == 0 {
		return types.ManagedObjectReference{}, fmt.Errorf("no host profile named %q found (no host profiles exist on this connection)", name)
	}

	var profiles []mo.Profile
	if err := property.DefaultCollector(client.Client.Client).Retrieve(ctx, mgr.Profile, []string{"name"}, &profiles); err != nil {
		return types.ManagedObjectReference{}, fmt.Errorf("failed to read host profile names: %w", err)
	}

	var match *types.ManagedObjectReference
	for i := range profiles {
		if profiles[i].Name == name {
			if match != nil {
				return types.ManagedObjectReference{}, fmt.Errorf("multiple host profiles named %q found — ambiguous", name)
			}
			ref := profiles[i].Self
			match = &ref
		}
	}
	if match == nil {
		return types.ManagedObjectReference{}, fmt.Errorf("no host profile named %q found (see vmware_host_profile_query_metadata for available names)", name)
	}
	return *match, nil
}

// hprofOptionalProfileRef resolves the optional "profile" argument (used by
// vmware_host_profile_query_metadata as extra context) via
// hprofResolveProfile, or returns (nil, nil) when omitted/empty.
func hprofOptionalProfileRef(ctx context.Context, client *vmware.Client, args map[string]interface{}) (*types.ManagedObjectReference, error) {
	name, _ := args["profile"].(string)
	if name == "" {
		return nil, nil
	}
	ref, err := hprofResolveProfile(ctx, client, args)
	if err != nil {
		return nil, err
	}
	return &ref, nil
}

// hprofResolveHosts resolves a required, non-empty "hosts" JSON array
// argument to a slice of host MoRefs, one resolveHost call per name — same
// per-element resolution loop as generated_authorization.go's
// resolveEntityRefs, applied to hosts instead of arbitrary entities.
func hprofResolveHosts(ctx context.Context, client *vmware.Client, raw interface{}) ([]types.ManagedObjectReference, error) {
	if raw == nil {
		return nil, fmt.Errorf("hosts is required")
	}
	names, err := toStringSlice(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid hosts: %w", err)
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("hosts must be a non-empty array")
	}
	refs := make([]types.ManagedObjectReference, 0, len(names))
	for i, name := range names {
		h, err := resolveHost(ctx, client, map[string]interface{}{"host": name})
		if err != nil {
			return nil, fmt.Errorf("hosts[%d]: %w", i, err)
		}
		refs = append(refs, h.Reference())
	}
	return refs, nil
}

// hprofOptionalEntityRefs resolves the optional "entity_paths" argument via
// resolveEntityRefs (generated_authorization.go), or returns (nil, nil) when
// omitted/empty — used by vmware_host_profile_dissociate ("omit to remove
// ALL associations") and vmware_host_profile_check_compliance ("omit to
// check every associated entity").
func hprofOptionalEntityRefs(ctx context.Context, client *vmware.Client, args map[string]interface{}) ([]types.ManagedObjectReference, error) {
	raw, ok := args["entity_paths"]
	if !ok || raw == nil {
		return nil, nil
	}
	paths, err := toStringSlice(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid entity_paths: %w", err)
	}
	if len(paths) == 0 {
		return nil, nil
	}
	return resolveEntityRefs(ctx, client, paths)
}

// hprofHostBasedConfigSpec builds a *types.HostProfileHostBasedConfigSpec
// (which satisfies BOTH types.BaseProfileCreateSpec, via CreateProfile's
// polymorphic CreateSpec field, AND types.BaseHostProfileConfigSpec, via
// UpdateHostProfile's polymorphic Config field — both interfaces are
// implemented through Go's promoted-method rule, since
// HostProfileHostBasedConfigSpec embeds HostProfileConfigSpec which embeds
// ProfileCreateSpec) directly from friendly top-level arguments instead of
// generic JSON decode — see this file's top doc comment, limitation point 3,
// for why. Shared by handleHostProfileCreate and handleHostProfileUpdate.
func hprofHostBasedConfigSpec(ctx context.Context, client *vmware.Client, args map[string]interface{}) (*types.HostProfileHostBasedConfigSpec, *object.HostSystem, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return nil, nil, err
	}
	spec := &types.HostProfileHostBasedConfigSpec{Host: host.Reference()}
	if name, ok := args["name"].(string); ok {
		spec.Name = name
	}
	if annotation, ok := args["annotation"].(string); ok {
		spec.Annotation = annotation
	}
	if enabled, ok := args["enabled"].(bool); ok {
		spec.Enabled = &enabled
	}
	if useEngine, ok := args["use_host_profile_engine"].(bool); ok {
		spec.UseHostProfileEngine = &useEngine
	}
	return spec, host, nil
}

// hprofDecodeHostConfigSpec decodes the required "config_spec" argument into
// a types.HostConfigSpec via this project's normal decodeJSONArg — see this
// file's top doc comment, limitation point 1, for the option/userAccount/
// usergroupAccount polymorphism this cannot populate.
func hprofDecodeHostConfigSpec(raw interface{}) (types.HostConfigSpec, error) {
	var spec types.HostConfigSpec
	if raw == nil {
		return spec, fmt.Errorf("config_spec is required")
	}
	if err := decodeJSONArg(raw, &spec); err != nil {
		return spec, fmt.Errorf("invalid config_spec (see this tool's schema description for the option/userAccount/usergroupAccount limitation): %w", err)
	}
	return spec, nil
}

// hprofDecodeUserInput decodes the optional "user_input"/"deferred_params"
// argument into a []types.ProfileDeferredPolicyOptionParameter — a concrete
// (non-polymorphic) type, safe for plain decodeJSONArg.
func hprofDecodeUserInput(raw interface{}) ([]types.ProfileDeferredPolicyOptionParameter, error) {
	if raw == nil {
		return nil, nil
	}
	var params []types.ProfileDeferredPolicyOptionParameter
	if err := decodeJSONArg(raw, &params); err != nil {
		return nil, fmt.Errorf("invalid deferred parameters: %w", err)
	}
	return params, nil
}

// hprofDecodeApplyProfile decodes an optional types.HostApplyProfile JSON
// argument — see this file's top doc comment, limitation point 2, for why
// only concrete (non-typeattr) sub-trees round-trip reliably.
func hprofDecodeApplyProfile(raw interface{}) (*types.HostApplyProfile, error) {
	if raw == nil {
		return nil, nil
	}
	var p types.HostApplyProfile
	if err := decodeJSONArg(raw, &p); err != nil {
		return nil, fmt.Errorf("invalid HostApplyProfile JSON (see this tool's schema description for the deep-polymorphism limitation): %w", err)
	}
	return &p, nil
}

// hprofWaitTask blocks until the task moref returned by one of this file's
// MUTATING Task-suffixed raw SOAP calls completes, discarding the result —
// same construction/delegation as generated_vm_ft.go's ftWaitTask (wrap the
// bare MoRef in a client-side-only *object.Task, then this package's
// existing waitForTask from vm.go).
func hprofWaitTask(ctx context.Context, client *vmware.Client, ref types.ManagedObjectReference) error {
	return waitForTask(ctx, object.NewTask(client.Client.Client, ref))
}

// hprofWaitTaskResult blocks until the task moref returned by one of this
// file's READ/PLAN-oriented Task-suffixed raw SOAP calls completes and
// returns its Info.Result payload — needed because several Task-flavored
// methods here (GenerateHostProfileTaskList_Task, CheckAnswerFileStatus_Task,
// ExportAnswerFile_Task, CheckProfileCompliance_Task) exist purely to
// COMPUTE/EXPORT data, not to mutate anything, so discarding the result the
// way hprofWaitTask/vm.go's plain waitForTask does would make those tools
// return no useful data at all.
func hprofWaitTaskResult(ctx context.Context, client *vmware.Client, ref types.ManagedObjectReference) (interface{}, error) {
	info, err := object.NewTask(client.Client.Client, ref).WaitForResult(ctx)
	if err != nil {
		return nil, err
	}
	return info.Result, nil
}

// === HostProfileManager-level handlers ===================================

func handleHostProfileCreate(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgrRef, err := hprofManagerRef(client)
	if err != nil {
		return "", err
	}
	name, _ := args["name"].(string)
	if name == "" {
		return "", fmt.Errorf("name is required")
	}
	spec, host, err := hprofHostBasedConfigSpec(ctx, client, args)
	if err != nil {
		return "", err
	}
	spec.Name = name

	resp, err := methods.CreateProfile(ctx, client.Client.Client, &types.CreateProfile{
		This:       mgrRef,
		CreateSpec: spec,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create host profile %q from host %s: %w", name, host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{
		"name":    name,
		"host":    host.InventoryPath,
		"profile": resp.Returnval,
		"result":  "profile_created",
	})
}

func handleHostProfileQueryMetadata(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgrRef, err := hprofManagerRef(client)
	if err != nil {
		return "", err
	}
	var names []string
	if raw, ok := args["profile_names"]; ok && raw != nil {
		names, err = toStringSlice(raw)
		if err != nil {
			return "", fmt.Errorf("invalid profile_names: %w", err)
		}
	}
	profileRef, err := hprofOptionalProfileRef(ctx, client, args)
	if err != nil {
		return "", err
	}

	resp, err := methods.QueryHostProfileMetadata(ctx, client.Client.Client, &types.QueryHostProfileMetadata{
		This:        mgrRef,
		ProfileName: names,
		Profile:     profileRef,
	})
	if err != nil {
		return "", fmt.Errorf("failed to query host profile metadata: %w", err)
	}

	return marshalJSON(map[string]interface{}{"count": len(resp.Returnval), "metadata": resp.Returnval})
}

func handleHostProfileFindAssociated(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgrRef, err := hprofManagerRef(client)
	if err != nil {
		return "", err
	}
	path, _ := args["entity_path"].(string)
	entity, err := resolveEntityRef(ctx, client, path)
	if err != nil {
		return "", err
	}

	resp, err := methods.FindAssociatedProfile(ctx, client.Client.Client, &types.FindAssociatedProfile{
		This:   mgrRef,
		Entity: entity,
	})
	if err != nil {
		return "", fmt.Errorf("failed to find profiles associated with %s: %w", path, err)
	}

	return marshalJSON(map[string]interface{}{"entity_path": path, "count": len(resp.Returnval), "profiles": resp.Returnval})
}

func handleHostProfileApplyConfig(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgrRef, err := hprofManagerRef(client)
	if err != nil {
		return "", err
	}
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}
	spec, err := hprofDecodeHostConfigSpec(args["config_spec"])
	if err != nil {
		return "", err
	}
	userInput, err := hprofDecodeUserInput(args["user_input"])
	if err != nil {
		return "", err
	}

	resp, err := methods.ApplyHostConfig_Task(ctx, client.Client.Client, &types.ApplyHostConfig_Task{
		This:       mgrRef,
		Host:       host.Reference(),
		ConfigSpec: spec,
		UserInput:  userInput,
	})
	if err != nil {
		return "", fmt.Errorf("failed to apply host config to %s: %w", host.InventoryPath, err)
	}
	if err := hprofWaitTask(ctx, client, resp.Returnval); err != nil {
		return "", fmt.Errorf("apply-host-config task failed for %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "result": "host_config_applied"})
}

func handleHostProfileGenerateConfigTaskList(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgrRef, err := hprofManagerRef(client)
	if err != nil {
		return "", err
	}
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}
	spec, err := hprofDecodeHostConfigSpec(args["config_spec"])
	if err != nil {
		return "", err
	}

	resp, err := methods.GenerateConfigTaskList(ctx, client.Client.Client, &types.GenerateConfigTaskList{
		This:       mgrRef,
		ConfigSpec: spec,
		Host:       host.Reference(),
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate config task list for %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "task_list": resp.Returnval})
}

func handleHostProfileGenerateTaskList(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgrRef, err := hprofManagerRef(client)
	if err != nil {
		return "", err
	}
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}
	spec, err := hprofDecodeHostConfigSpec(args["config_spec"])
	if err != nil {
		return "", err
	}

	resp, err := methods.GenerateHostProfileTaskList_Task(ctx, client.Client.Client, &types.GenerateHostProfileTaskList_Task{
		This:       mgrRef,
		ConfigSpec: spec,
		Host:       host.Reference(),
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate host profile task list for %s: %w", host.InventoryPath, err)
	}
	result, err := hprofWaitTaskResult(ctx, client, resp.Returnval)
	if err != nil {
		return "", fmt.Errorf("generate-host-profile-task-list task failed for %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "task_list": result})
}

func handleHostProfileAnswerFileCheckStatus(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgrRef, err := hprofManagerRef(client)
	if err != nil {
		return "", err
	}
	hostRefs, err := hprofResolveHosts(ctx, client, args["hosts"])
	if err != nil {
		return "", err
	}

	resp, err := methods.CheckAnswerFileStatus_Task(ctx, client.Client.Client, &types.CheckAnswerFileStatus_Task{
		This: mgrRef,
		Host: hostRefs,
	})
	if err != nil {
		return "", fmt.Errorf("failed to check answer file status: %w", err)
	}
	result, err := hprofWaitTaskResult(ctx, client, resp.Returnval)
	if err != nil {
		return "", fmt.Errorf("check-answer-file-status task failed: %w", err)
	}

	return marshalJSON(map[string]interface{}{"count": len(hostRefs), "status": result})
}

func handleHostProfileAnswerFileQueryStatus(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgrRef, err := hprofManagerRef(client)
	if err != nil {
		return "", err
	}
	hostRefs, err := hprofResolveHosts(ctx, client, args["hosts"])
	if err != nil {
		return "", err
	}

	resp, err := methods.QueryAnswerFileStatus(ctx, client.Client.Client, &types.QueryAnswerFileStatus{
		This: mgrRef,
		Host: hostRefs,
	})
	if err != nil {
		return "", fmt.Errorf("failed to query answer file status: %w", err)
	}

	return marshalJSON(map[string]interface{}{"count": len(resp.Returnval), "status": resp.Returnval})
}

func handleHostProfileAnswerFileUpdate(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgrRef, err := hprofManagerRef(client)
	if err != nil {
		return "", err
	}
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}
	userInput, err := hprofDecodeUserInput(args["user_input"])
	if err != nil {
		return "", err
	}
	spec := &types.AnswerFileOptionsCreateSpec{UserInput: userInput}
	if validating, ok := args["validating"].(bool); ok {
		spec.Validating = &validating
	}

	resp, err := methods.UpdateAnswerFile_Task(ctx, client.Client.Client, &types.UpdateAnswerFile_Task{
		This:       mgrRef,
		Host:       host.Reference(),
		ConfigSpec: spec,
	})
	if err != nil {
		return "", fmt.Errorf("failed to update answer file for %s: %w", host.InventoryPath, err)
	}
	if err := hprofWaitTask(ctx, client, resp.Returnval); err != nil {
		return "", fmt.Errorf("update-answer-file task failed for %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "result": "answer_file_updated"})
}

func handleHostProfileAnswerFileRetrieve(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgrRef, err := hprofManagerRef(client)
	if err != nil {
		return "", err
	}
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}

	resp, err := methods.RetrieveAnswerFile(ctx, client.Client.Client, &types.RetrieveAnswerFile{
		This: mgrRef,
		Host: host.Reference(),
	})
	if err != nil {
		return "", fmt.Errorf("failed to retrieve answer file for %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "answer_file": resp.Returnval})
}

func handleHostProfileAnswerFileRetrieveForProfile(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgrRef, err := hprofManagerRef(client)
	if err != nil {
		return "", err
	}
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}
	applyProfile, err := hprofDecodeApplyProfile(args["apply_profile"])
	if err != nil {
		return "", err
	}
	if applyProfile == nil {
		return "", fmt.Errorf("apply_profile is required")
	}

	resp, err := methods.RetrieveAnswerFileForProfile(ctx, client.Client.Client, &types.RetrieveAnswerFileForProfile{
		This:         mgrRef,
		Host:         host.Reference(),
		ApplyProfile: *applyProfile,
	})
	if err != nil {
		return "", fmt.Errorf("failed to retrieve answer file for profile on %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "answer_file": resp.Returnval})
}

func handleHostProfileAnswerFileExport(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgrRef, err := hprofManagerRef(client)
	if err != nil {
		return "", err
	}
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}

	resp, err := methods.ExportAnswerFile_Task(ctx, client.Client.Client, &types.ExportAnswerFile_Task{
		This: mgrRef,
		Host: host.Reference(),
	})
	if err != nil {
		return "", fmt.Errorf("failed to export answer file for %s: %w", host.InventoryPath, err)
	}
	result, err := hprofWaitTaskResult(ctx, client, resp.Returnval)
	if err != nil {
		return "", fmt.Errorf("export-answer-file task failed for %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"host": host.InventoryPath, "answer_file": result})
}

func handleHostProfileComposite(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mgrRef, err := hprofManagerRef(client)
	if err != nil {
		return "", err
	}
	sourceRef, err := hprofResolveProfile(ctx, client, map[string]interface{}{"profile": args["source"]})
	if err != nil {
		return "", err
	}
	var targetRefs []types.ManagedObjectReference
	if raw, ok := args["targets"]; ok && raw != nil {
		names, err := toStringSlice(raw)
		if err != nil {
			return "", fmt.Errorf("invalid targets: %w", err)
		}
		for i, name := range names {
			ref, err := hprofResolveProfile(ctx, client, map[string]interface{}{"profile": name})
			if err != nil {
				return "", fmt.Errorf("targets[%d]: %w", i, err)
			}
			targetRefs = append(targetRefs, ref)
		}
	}
	toBeMerged, err := hprofDecodeApplyProfile(args["to_be_merged"])
	if err != nil {
		return "", fmt.Errorf("invalid to_be_merged: %w", err)
	}
	toBeReplacedWith, err := hprofDecodeApplyProfile(args["to_be_replaced_with"])
	if err != nil {
		return "", fmt.Errorf("invalid to_be_replaced_with: %w", err)
	}
	toBeDeleted, err := hprofDecodeApplyProfile(args["to_be_deleted"])
	if err != nil {
		return "", fmt.Errorf("invalid to_be_deleted: %w", err)
	}
	enableStatusToBeCopied, err := hprofDecodeApplyProfile(args["enable_status_to_be_copied"])
	if err != nil {
		return "", fmt.Errorf("invalid enable_status_to_be_copied: %w", err)
	}

	resp, err := methods.CompositeHostProfile_Task(ctx, client.Client.Client, &types.CompositeHostProfile_Task{
		This:                   mgrRef,
		Source:                 sourceRef,
		Targets:                targetRefs,
		ToBeMerged:             toBeMerged,
		ToBeReplacedWith:       toBeReplacedWith,
		ToBeDeleted:            toBeDeleted,
		EnableStatusToBeCopied: enableStatusToBeCopied,
	})
	if err != nil {
		return "", fmt.Errorf("failed to composite host profiles: %w", err)
	}
	if err := hprofWaitTask(ctx, client, resp.Returnval); err != nil {
		return "", fmt.Errorf("composite-host-profile task failed: %w", err)
	}

	return marshalJSON(map[string]interface{}{"result": "profiles_composited", "target_count": len(targetRefs)})
}

// === Profile/HostProfile-level handlers ===================================

func handleHostProfileRetrieveDescription(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	profileRef, err := hprofResolveProfile(ctx, client, args)
	if err != nil {
		return "", err
	}

	resp, err := methods.RetrieveDescription(ctx, client.Client.Client, &types.RetrieveDescription{This: profileRef})
	if err != nil {
		return "", fmt.Errorf("failed to retrieve description for profile %q: %w", args["profile"], err)
	}

	return marshalJSON(map[string]interface{}{"profile": args["profile"], "description": resp.Returnval})
}

func handleHostProfileCheckCompliance(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	profileRef, err := hprofResolveProfile(ctx, client, args)
	if err != nil {
		return "", err
	}
	entityRefs, err := hprofOptionalEntityRefs(ctx, client, args)
	if err != nil {
		return "", err
	}

	resp, err := methods.CheckProfileCompliance_Task(ctx, client.Client.Client, &types.CheckProfileCompliance_Task{
		This:   profileRef,
		Entity: entityRefs,
	})
	if err != nil {
		return "", fmt.Errorf("failed to check compliance for profile %q: %w", args["profile"], err)
	}
	result, err := hprofWaitTaskResult(ctx, client, resp.Returnval)
	if err != nil {
		return "", fmt.Errorf("check-profile-compliance task failed for %q: %w", args["profile"], err)
	}

	return marshalJSON(map[string]interface{}{"profile": args["profile"], "compliance": result})
}

func handleHostProfileExport(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	profileRef, err := hprofResolveProfile(ctx, client, args)
	if err != nil {
		return "", err
	}

	resp, err := methods.ExportProfile(ctx, client.Client.Client, &types.ExportProfile{This: profileRef})
	if err != nil {
		return "", fmt.Errorf("failed to export profile %q: %w", args["profile"], err)
	}

	return marshalJSON(map[string]interface{}{"profile": args["profile"], "export": resp.Returnval})
}

func handleHostProfileDestroy(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	profileRef, err := hprofResolveProfile(ctx, client, args)
	if err != nil {
		return "", err
	}

	if _, err := methods.DestroyProfile(ctx, client.Client.Client, &types.DestroyProfile{This: profileRef}); err != nil {
		return "", fmt.Errorf("failed to destroy profile %q: %w", args["profile"], err)
	}

	return marshalJSON(map[string]interface{}{"profile": args["profile"], "result": "profile_destroyed"})
}

func handleHostProfileAssociate(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	profileRef, err := hprofResolveProfile(ctx, client, args)
	if err != nil {
		return "", err
	}
	raw, ok := args["entity_paths"]
	if !ok {
		return "", fmt.Errorf("entity_paths is required")
	}
	paths, err := toStringSlice(raw)
	if err != nil {
		return "", fmt.Errorf("invalid entity_paths: %w", err)
	}
	entityRefs, err := resolveEntityRefs(ctx, client, paths)
	if err != nil {
		return "", err
	}

	if _, err := methods.AssociateProfile(ctx, client.Client.Client, &types.AssociateProfile{
		This:   profileRef,
		Entity: entityRefs,
	}); err != nil {
		return "", fmt.Errorf("failed to associate profile %q: %w", args["profile"], err)
	}

	return marshalJSON(map[string]interface{}{"profile": args["profile"], "entity_paths": paths, "result": "profile_associated"})
}

func handleHostProfileDissociate(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	profileRef, err := hprofResolveProfile(ctx, client, args)
	if err != nil {
		return "", err
	}
	entityRefs, err := hprofOptionalEntityRefs(ctx, client, args)
	if err != nil {
		return "", err
	}

	if _, err := methods.DissociateProfile(ctx, client.Client.Client, &types.DissociateProfile{
		This:   profileRef,
		Entity: entityRefs,
	}); err != nil {
		return "", fmt.Errorf("failed to dissociate profile %q: %w", args["profile"], err)
	}

	result := map[string]interface{}{"profile": args["profile"], "result": "profile_dissociated"}
	if len(entityRefs) == 0 {
		result["entity_paths"] = "all"
	}
	return marshalJSON(result)
}

func handleHostProfileUpdate(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	profileRef, err := hprofResolveProfile(ctx, client, args)
	if err != nil {
		return "", err
	}
	spec, host, err := hprofHostBasedConfigSpec(ctx, client, args)
	if err != nil {
		return "", err
	}

	if _, err := methods.UpdateHostProfile(ctx, client.Client.Client, &types.UpdateHostProfile{
		This:   profileRef,
		Config: spec,
	}); err != nil {
		return "", fmt.Errorf("failed to update profile %q from host %s: %w", args["profile"], host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"profile": args["profile"], "host": host.InventoryPath, "result": "profile_updated"})
}

func handleHostProfileExecute(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	profileRef, err := hprofResolveProfile(ctx, client, args)
	if err != nil {
		return "", err
	}
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}
	deferredParams, err := hprofDecodeUserInput(args["deferred_params"])
	if err != nil {
		return "", err
	}

	resp, err := methods.ExecuteHostProfile(ctx, client.Client.Client, &types.ExecuteHostProfile{
		This:          profileRef,
		Host:          host.Reference(),
		DeferredParam: deferredParams,
	})
	if err != nil {
		return "", fmt.Errorf("failed to execute profile %q against %s: %w", args["profile"], host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"profile": args["profile"], "host": host.InventoryPath, "result": resp.Returnval})
}

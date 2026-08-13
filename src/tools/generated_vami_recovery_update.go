package tools

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// registerVAMIRecoveryUpdateTools is Group G1 of Fase 8b (the SECOND codegen
// technique of the plan's Fase 8, distinct from Fase 8a's AST-over-
// vapi/*/*.go approach) — 31 VAMI (Virtual Appliance Management Interface,
// vCenter Server Appliance only) "legacy" /rest/appliance/... routes
// covering recovery/backup, recovery/restore, and update (the vSphere 6.7
// Update API), none of which have a Go SDK wrapper anywhere in govmomi.
// Sourced from ".workspace/vSphere Automation REST Resources for
// appliance.postman_collection.json" (the "Recovery" and "Update - vSphere
// 6.7" folders), method/path/query/body shape hand-transcribed and confirmed
// byte-for-byte against that collection JSON, not guessed.
//
// Same architecture as tools/appliance.go (Fase 4's VAMI starter slice, the
// first file in this family — read it first): client.REST(ctx) for the
// *rest.Client, generic interface{} decode via vamiCall below (this file's
// analogue of appliance.go's applianceGet, extended to also support method/
// query-params/body — appliance.go itself is untouched). No typed request OR
// response struct: unlike Fase 8a's vapi/*/*.go groups, there is no Go
// source to check field names against here, and this project has no
// simulator or live vCenter Server Appliance to validate one empirically
// either. A guessed struct could silently rename/drop fields in either
// direction; forwarding both generically is the honest choice.
//
// Curation decisions (so a future reader doesn't have to re-derive them):
//
//   - Nested compound sub-objects whose exact internal shape carries
//     semantics this project can't verify (recurrence_info/retention_info on
//     backup schedules; user_data on update install/stage-and-install/
//     validate; policy on update policy set) are exposed as free-form JSON
//     object/array arguments and forwarded VERBATIM into the request body.
//     Every OTHER key built into the request bodies below IS the literal key
//     from the Postman raw body (backup_password, location, location_type,
//     location_user, location_password, comment, parts, enable) — never
//     invented. The compound sub-objects' own internal fields (e.g.
//     recurrence_info.hour/minute/days, user_data[].key/value) are NOT
//     independently validated by this tool — whatever JSON object/array the
//     caller supplies is passed straight through, exactly as MCP decoded it,
//     with zero reshaping. This also sidesteps guessing enum casing (e.g.
//     whether "days" wants "MONDAY" vs "Monday").
//
//   - vmware_appliance_update_policy_set (PUT /update/policy): the Postman
//     collection's own body sample for this one request is empty ("raw": ""
//     — confirmed by reading the collection JSON directly), so even the
//     wrapper key isn't documented there. Every OTHER POST/PUT body in both
//     the "Recovery" and "Update - vSphere 6.7" folders wraps its payload
//     under a key matching the vAPI operation's single input parameter name
//     (validate/create backup -> "piece", schedule create/update -> "spec",
//     install/stage-and-install/validate update -> bare "user_data" already)
//     — by that same convention, this tool wraps under "policy". The
//     policy object's internal fields (commonly auto_stage/
//     crl_check_enabled in VAMI's public docs, but NOT confirmed against
//     THIS Postman collection) are accepted as a free-form object and
//     forwarded verbatim, same reasoning as above.
//
//   - location_type (backup validate/create, restore create) is a plain
//     string, not a JSON schema enum: the Postman samples show "HTTPS" and
//     "FTPS", and VAMI's public docs describe a small closed set (HTTP,
//     HTTPS, FTP, FTPS, SCP, NFS), but enforcing that set client-side risks
//     rejecting a value the real appliance would accept. The tool
//     description names the typical values as a hint only.
//
//   - The trailing "/" in "POST .../backup/schedules/{id}/?action=run" is
//     preserved exactly as Postman recorded it (its path array literally
//     ends with an empty "" segment before the query) — legacy /rest routers
//     occasionally dispatch action query params differently with vs. without
//     a trailing slash, and there is nothing to test that against here, so
//     the safest move is to reproduce the recorded request byte-for-byte
//     rather than "clean it up".
//
//   - Every resource-scoped action (cancel/status/delete/run/update) requires
//     its VAMI-assigned ID as a plain required string argument
//     (backup_job_id, part_id, schedule_id, version) rather than this
//     project resolving/defaulting it — vSphere Appliance historically
//     allows only a single backup schedule (conventionally "default"), but
//     that is not hardcoded here; the caller must always pass the ID it got
//     from the matching list/get/check tool.
//
// vcsim gap, not a bug, for the ENTIRE domain — confirmed directly, same
// method as generated_appliance_small.go's top comment: (1)
// referencia/govmomi/vapi/appliance/simulator/simulator.go only wires
// shutdown/consolecli/dcui/shell/ssh (grep confirms 0 matches for
// recovery/update anywhere in that file); (2) this project's own test
// harness (tools/testhelpers_test.go's newSimClient) blank-imports
// vapi/simulator, whose simulator.go doesn't even reference the appliance
// sub-package at all (grep confirms 0 matches for "appliance" in
// referencia/govmomi/vapi/simulator/simulator.go) — so unlike Fase 8a's
// vcsim-backed integration tests, and even unlike
// generated_appliance_small_test.go's "reaches real vcsim" tests, this
// file's tests use an httptest fixture (same technique as
// tools/appliance_test.go), not vcsim, per this group's brief.
func registerVAMIRecoveryUpdateTools(r *Registry) {
	emptySchema := map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
	requiresVCSA := " Requires a vCenter Server Appliance — fails against a standalone ESXi host, which has no VAMI (Virtual Appliance Management Interface)."
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}

	// --- Recovery: Backup job ------------------------------------------------

	r.register("vmware_appliance_recovery_backup_job_list",
		"List every file-based backup job run on this vCenter Server Appliance (VAMI, GET /appliance/recovery/backup/job)."+requiresVCSA,
		emptySchema,
		Tool{Handler: handleApplianceRecoveryBackupJobList},
	)

	validateProps := vamiBackupPieceSchemaProps(true)
	r.register("vmware_appliance_recovery_backup_validate",
		"Validate a file-based backup destination/credentials WITHOUT starting a backup (VAMI, POST /appliance/recovery/backup/validate) — use before vmware_appliance_recovery_backup_job_create to catch a bad location/credentials early."+requiresVCSA,
		map[string]interface{}{
			"type":       "object",
			"properties": validateProps,
			"required":   []interface{}{"backup_password", "location", "location_type"},
		},
		Tool{Handler: handleApplianceRecoveryBackupValidate},
	)

	createProps := vamiBackupPieceSchemaProps(true)
	createProps["confirm"] = confirmArg
	r.registerDestructive("vmware_appliance_recovery_backup_job_create",
		"Start a new file-based backup of this vCenter Server Appliance to a remote location (VAMI, POST /appliance/recovery/backup/job). Disruptive to appliance I/O while running; does not alter the running appliance's configuration/state, only produces a new backup artifact."+requiresVCSA,
		tier2,
		map[string]interface{}{
			"type":       "object",
			"properties": createProps,
			"required":   []interface{}{"backup_password", "location", "location_type", "confirm"},
		},
		Tool{Handler: handleApplianceRecoveryBackupJobCreate},
	)

	r.registerDestructive("vmware_appliance_recovery_backup_job_cancel",
		"Cancel a running file-based backup job (VAMI, POST /appliance/recovery/backup/job/{id}/cancel)."+requiresVCSA,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"backup_job_id": vamiStr("ID of the backup job to cancel — from vmware_appliance_recovery_backup_job_list or the result of vmware_appliance_recovery_backup_job_create."),
				"confirm":       confirmArg,
			},
			"required": []interface{}{"backup_job_id", "confirm"},
		},
		Tool{Handler: handleApplianceRecoveryBackupJobCancel},
	)

	r.register("vmware_appliance_recovery_backup_job_status",
		"Get the status of one file-based backup job by ID (VAMI, GET /appliance/recovery/backup/job/{id})."+requiresVCSA,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"backup_job_id": vamiStr("ID of the backup job — from vmware_appliance_recovery_backup_job_list."),
			},
			"required": []interface{}{"backup_job_id"},
		},
		Tool{Handler: handleApplianceRecoveryBackupJobStatus},
	)

	r.register("vmware_appliance_recovery_backup_job_details",
		"Get the appliance's current backup job details/summary — what would be backed up right now (VAMI, GET /appliance/recovery/backup/job/details)."+requiresVCSA,
		emptySchema,
		Tool{Handler: handleApplianceRecoveryBackupJobDetails},
	)

	r.register("vmware_appliance_recovery_backup_parts",
		`List the optional backup parts available on this appliance (VAMI, GET /appliance/recovery/backup/parts) — pass their IDs in the "parts" argument of vmware_appliance_recovery_backup_job_create/validate/schedule tools to include them.`+requiresVCSA,
		emptySchema,
		Tool{Handler: handleApplianceRecoveryBackupParts},
	)

	r.register("vmware_appliance_recovery_backup_part_size",
		"Get the size of one optional backup part (VAMI, GET /appliance/recovery/backup/parts/{id})."+requiresVCSA,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"part_id": vamiStr("ID of the backup part — from vmware_appliance_recovery_backup_parts."),
			},
			"required": []interface{}{"part_id"},
		},
		Tool{Handler: handleApplianceRecoveryBackupPartSize},
	)

	// --- Recovery: Backup schedule --------------------------------------------

	r.register("vmware_appliance_recovery_backup_schedule_list",
		"List every configured file-based backup schedule on this vCenter Server Appliance (VAMI, GET /appliance/recovery/backup/schedules)."+requiresVCSA,
		emptySchema,
		Tool{Handler: handleApplianceRecoveryBackupScheduleList},
	)

	scheduleCreateProps := vamiScheduleSpecSchemaProps()
	scheduleCreateProps["schedule_id"] = vamiStr(`ID for the new schedule. vSphere Appliance has historically supported only a single backup schedule, conventionally identified "default" — but this tool does not assume or hardcode that; pass the ID your appliance expects.`)
	scheduleCreateProps["confirm"] = confirmArg
	r.registerDestructive("vmware_appliance_recovery_backup_schedule_create",
		"Create a recurring file-based backup schedule (VAMI, POST /appliance/recovery/backup/schedules/{id})."+requiresVCSA,
		tier2,
		map[string]interface{}{
			"type":       "object",
			"properties": scheduleCreateProps,
			"required":   []interface{}{"schedule_id", "backup_password", "location", "enable", "recurrence_info", "retention_info", "confirm"},
		},
		Tool{Handler: handleApplianceRecoveryBackupScheduleCreate},
	)

	r.register("vmware_appliance_recovery_backup_schedule_get",
		"Get the details of one file-based backup schedule by ID (VAMI, GET /appliance/recovery/backup/schedules/{id})."+requiresVCSA,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"schedule_id": vamiStr("ID of the backup schedule — from vmware_appliance_recovery_backup_schedule_list."),
			},
			"required": []interface{}{"schedule_id"},
		},
		Tool{Handler: handleApplianceRecoveryBackupScheduleGet},
	)

	r.registerDestructive("vmware_appliance_recovery_backup_schedule_delete",
		"Permanently remove a file-based backup schedule (VAMI, DELETE /appliance/recovery/backup/schedules/{id}). Irreversible — the schedule configuration itself is gone (backups it already produced are not touched)."+requiresVCSA,
		tier1,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"schedule_id": vamiStr("ID of the backup schedule to remove — from vmware_appliance_recovery_backup_schedule_list."),
				"confirm":     confirmArg,
			},
			"required": []interface{}{"schedule_id", "confirm"},
		},
		Tool{Handler: handleApplianceRecoveryBackupScheduleDelete},
	)

	r.registerDestructive("vmware_appliance_recovery_backup_schedule_run",
		"Run a configured file-based backup schedule immediately, out of cycle (VAMI, POST /appliance/recovery/backup/schedules/{id}/?action=run)."+requiresVCSA,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"schedule_id": vamiStr("ID of the backup schedule to run now — from vmware_appliance_recovery_backup_schedule_list."),
				"comment":     vamiStr("Free-text comment recorded with this ad hoc run. Optional."),
				"confirm":     confirmArg,
			},
			"required": []interface{}{"schedule_id", "confirm"},
		},
		Tool{Handler: handleApplianceRecoveryBackupScheduleRun},
	)

	scheduleUpdateProps := vamiScheduleSpecSchemaProps()
	scheduleUpdateProps["schedule_id"] = vamiStr("ID of the backup schedule to update — from vmware_appliance_recovery_backup_schedule_list.")
	scheduleUpdateProps["confirm"] = confirmArg
	r.registerDestructive("vmware_appliance_recovery_backup_schedule_update",
		"Replace the configuration of an existing file-based backup schedule (VAMI, PUT /appliance/recovery/backup/schedules/{id}). This is a full replace, not a partial patch — pass every field you want to keep, not just the ones changing."+requiresVCSA,
		tier2,
		map[string]interface{}{
			"type":       "object",
			"properties": scheduleUpdateProps,
			"required":   []interface{}{"schedule_id", "backup_password", "location", "enable", "recurrence_info", "retention_info", "confirm"},
		},
		Tool{Handler: handleApplianceRecoveryBackupScheduleUpdate},
	)

	// --- Recovery: Restore job -------------------------------------------------

	r.register("vmware_appliance_recovery_restore_job_status",
		"Get the status of the current/most recent appliance restore job (VAMI, GET /appliance/recovery/restore/job). Restore jobs only run from the temporary bootstrap appliance created by the vCenter Server Appliance restore workflow, before the restored appliance is fully back up — this tool has no practical use against a normal running appliance."+requiresVCSA,
		emptySchema,
		Tool{Handler: handleApplianceRecoveryRestoreJobStatus},
	)

	r.registerDestructive("vmware_appliance_recovery_restore_job_cancel",
		"Cancel the in-progress appliance restore job (VAMI, POST /appliance/recovery/restore/job/cancel)."+requiresVCSA,
		tier2,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"confirm": confirmArg},
			"required":   []interface{}{"confirm"},
		},
		Tool{Handler: handleApplianceRecoveryRestoreJobCancel},
	)

	restoreCreateProps := vamiBackupPieceSchemaProps(false)
	restoreCreateProps["confirm"] = confirmArg
	r.registerDestructive("vmware_appliance_recovery_restore_job_create",
		"Restore this vCenter Server Appliance from a file-based backup (VAMI, POST /appliance/recovery/restore/job). Irreversible and highly disruptive: it overwrites the appliance's current state with the backed-up one — there is no undo. Per VMware's documented restore workflow this is normally invoked against a freshly-deployed, not-yet-configured bootstrap appliance, not a live production vCenter Server."+requiresVCSA,
		tier1,
		map[string]interface{}{
			"type":       "object",
			"properties": restoreCreateProps,
			"required":   []interface{}{"backup_password", "location", "location_type", "confirm"},
		},
		Tool{Handler: handleApplianceRecoveryRestoreJobCreate},
	)

	// --- Update: Check for updates ---------------------------------------------

	r.register("vmware_appliance_update_check_url_cdrom",
		"Check for a pending vCenter Server update from BOTH the mounted CD-ROM and the configured online URL (VAMI, GET /appliance/update/pending?source_type=LOCAL_AND_ONLINE). Triggers a live check against the source(s) — see vmware_appliance_update_check_last to read the result of the last check without triggering a new one."+requiresVCSA,
		emptySchema,
		Tool{Handler: handleApplianceUpdateCheckURLCdrom},
	)

	r.register("vmware_appliance_update_check_cdrom",
		"Check for a pending vCenter Server update from the mounted CD-ROM ONLY (VAMI, GET /appliance/update/pending?source_type=LOCAL)."+requiresVCSA,
		emptySchema,
		Tool{Handler: handleApplianceUpdateCheckCdrom},
	)

	r.register("vmware_appliance_update_check_last",
		"Get the result of the most recent update check without triggering a new one (VAMI, GET /appliance/update/pending?source_type=LAST_CHECK)."+requiresVCSA,
		emptySchema,
		Tool{Handler: handleApplianceUpdateCheckLast},
	)

	r.register("vmware_appliance_update_pending_details",
		"Get details of one specific pending vCenter Server update version (VAMI, GET /appliance/update/pending/{version})."+requiresVCSA,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"version": vamiStr("Update version identifier — from vmware_appliance_update_check_url_cdrom/check_cdrom/check_last."),
			},
			"required": []interface{}{"version"},
		},
		Tool{Handler: handleApplianceUpdatePendingDetails},
	)

	// --- Update: Policy ----------------------------------------------------------

	r.register("vmware_appliance_update_policy_get",
		"Get the vCenter Server Appliance's automatic update policy (VAMI, GET /appliance/update/policy)."+requiresVCSA,
		emptySchema,
		Tool{Handler: handleApplianceUpdatePolicyGet},
	)

	r.registerDestructive("vmware_appliance_update_policy_set",
		`Set the vCenter Server Appliance's automatic update policy (VAMI, PUT /appliance/update/policy). The "policy" object's fields are forwarded verbatim — this Postman collection does not document them for this specific call (its own sample body is empty); VAMI's public documentation commonly describes "auto_stage" and "crl_check_enabled" as booleans, but that is not confirmed against this appliance/version, so pass whatever fields your target actually expects.`+requiresVCSA,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"policy":  vamiObj(`Update policy object, forwarded verbatim as the request body's "policy" field. Fields are NOT validated by this tool.`),
				"confirm": confirmArg,
			},
			"required": []interface{}{"policy", "confirm"},
		},
		Tool{Handler: handleApplianceUpdatePolicySet},
	)

	// --- Update: Stage -------------------------------------------------------------

	r.registerDestructive("vmware_appliance_update_stage",
		"Download/prepare (but do not install) a pending vCenter Server update (VAMI, POST /appliance/update/pending/{version}?action=stage). Reversible — vmware_appliance_update_staged_delete discards the staged download without installing it."+requiresVCSA,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"version": vamiStr("Update version identifier to stage — from vmware_appliance_update_check_url_cdrom/check_cdrom/check_last."),
				"confirm": confirmArg,
			},
			"required": []interface{}{"version", "confirm"},
		},
		Tool{Handler: handleApplianceUpdateStage},
	)

	r.register("vmware_appliance_update_staged_get",
		"Get the vCenter Server update currently staged (downloaded but not installed), if any (VAMI, GET /appliance/update/staged)."+requiresVCSA,
		emptySchema,
		Tool{Handler: handleApplianceUpdateStagedGet},
	)

	r.registerDestructive("vmware_appliance_update_staged_delete",
		"Discard the currently staged (downloaded but not installed) vCenter Server update (VAMI, DELETE /appliance/update/staged). Reversible — the update can be re-staged with vmware_appliance_update_stage."+requiresVCSA,
		tier2,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"confirm": confirmArg},
			"required":   []interface{}{"confirm"},
		},
		Tool{Handler: handleApplianceUpdateStagedDelete},
	)

	// --- Update: Install -----------------------------------------------------------

	r.register("vmware_appliance_update_precheck",
		"Run pre-update checks for a pending vCenter Server update without installing it (VAMI, POST /appliance/update/pending/{version}?action=precheck)."+requiresVCSA,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"version": vamiStr("Update version identifier to precheck — from vmware_appliance_update_check_url_cdrom/check_cdrom/check_last."),
			},
			"required": []interface{}{"version"},
		},
		Tool{Handler: handleApplianceUpdatePrecheck},
	)

	userDataArg := vamiArrObj(`Optional install-time key/value parameters, forwarded verbatim as the request body's "user_data" array — e.g. [{"key":"vmdir.password","value":"<SSO admin password>"}]. Postman's own samples always include the SSO admin password under key "vmdir.password"; omit only if you know this update doesn't need it.`)

	r.registerDestructive("vmware_appliance_update_install",
		"Install a pending vCenter Server update that has ALREADY been staged (VAMI, POST /appliance/update/pending/{version}?action=install). Irreversible — a real vCenter Server upgrade, not a dry run."+requiresVCSA,
		tier1,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"version":   vamiStr("Update version identifier to install — must already be staged (vmware_appliance_update_stage)."),
				"user_data": userDataArg,
				"confirm":   confirmArg,
			},
			"required": []interface{}{"version", "confirm"},
		},
		Tool{Handler: handleApplianceUpdateInstall},
	)

	r.registerDestructive("vmware_appliance_update_stage_and_install",
		"Download/prepare AND install a pending vCenter Server update in one call (VAMI, POST /appliance/update/pending/{version}?action=stage-and-install). Irreversible — a real vCenter Server upgrade, not a dry run."+requiresVCSA,
		tier1,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"version":   vamiStr("Update version identifier to stage and install — from vmware_appliance_update_check_url_cdrom/check_cdrom/check_last."),
				"user_data": userDataArg,
				"confirm":   confirmArg,
			},
			"required": []interface{}{"version", "confirm"},
		},
		Tool{Handler: handleApplianceUpdateStageAndInstall},
	)

	r.register("vmware_appliance_update_validate",
		"Validate a pending vCenter Server update WITHOUT staging or installing it (VAMI, POST /appliance/update/pending/{version}?action=validate)."+requiresVCSA,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"version":   vamiStr("Update version identifier to validate — from vmware_appliance_update_check_url_cdrom/check_cdrom/check_last."),
				"user_data": userDataArg,
			},
			"required": []interface{}{"version"},
		},
		Tool{Handler: handleApplianceUpdateValidate},
	)

	// --- Update: Status --------------------------------------------------------

	r.register("vmware_appliance_update_status",
		"Get the overall status of the current/most recent vCenter Server update operation (VAMI, GET /appliance/update)."+requiresVCSA,
		emptySchema,
		Tool{Handler: handleApplianceUpdateStatus},
	)
}

// --- Schema helpers -----------------------------------------------------------

func vamiStr(desc string) map[string]interface{} {
	return map[string]interface{}{"type": "string", "description": desc}
}

func vamiBool(desc string) map[string]interface{} {
	return map[string]interface{}{"type": "boolean", "description": desc}
}

func vamiObj(desc string) map[string]interface{} {
	return map[string]interface{}{"type": "object", "description": desc}
}

func vamiArr(desc string) map[string]interface{} {
	return map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": desc}
}

func vamiArrObj(desc string) map[string]interface{} {
	return map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "object"}, "description": desc}
}

// vamiBackupPieceSchemaProps returns a FRESH map every call — every caller
// (validate/create/restore-create) mutates its own copy afterward (adding
// "confirm", or nothing at all), so a shared/cached map here would leak
// fields across unrelated tool schemas. includeCommentParts is false for
// restore (Postman's "Restore job - create" body has no comment/parts keys,
// unlike backup's "piece" — confirmed by reading both raw bodies directly).
func vamiBackupPieceSchemaProps(includeCommentParts bool) map[string]interface{} {
	props := map[string]interface{}{
		"backup_password":   vamiStr("Password used to encrypt/protect the backup piece."),
		"location":          vamiStr("Backup destination URL/path (e.g. an HTTPS/FTPS/SCP/NFS server location)."),
		"location_type":     vamiStr("Backup location protocol. Typical VAMI values: HTTP, HTTPS, FTP, FTPS, SCP, NFS (not enforced by this tool — whatever this appliance's LocationType enum accepts is forwarded verbatim)."),
		"location_user":     vamiStr("Username for the backup location, if it requires authentication. Optional."),
		"location_password": vamiStr("Password for the backup location, if it requires authentication. Optional."),
	}
	if includeCommentParts {
		props["comment"] = vamiStr("Free-text comment recorded with the backup job. Optional.")
		props["parts"] = vamiArr(`Optional backup parts to include beyond the default (e.g. "seat" for statistics/events/alarms/tasks) — see vmware_appliance_recovery_backup_parts for the parts available on this appliance.`)
	}
	return props
}

// vamiScheduleSpecSchemaProps returns a FRESH map every call — same
// leak-avoidance reasoning as vamiBackupPieceSchemaProps. recurrence_info/
// retention_info are free-form objects, not decomposed into hour/minute/
// days/max_count fields — see this file's top doc comment for why.
func vamiScheduleSpecSchemaProps() map[string]interface{} {
	return map[string]interface{}{
		"backup_password":   vamiStr("Password used to encrypt/protect backups produced by this schedule."),
		"location":          vamiStr("Backup destination URL/path used by this schedule."),
		"location_user":     vamiStr("Username for the backup location, if it requires authentication. Optional."),
		"location_password": vamiStr("Password for the backup location, if it requires authentication. Optional."),
		"enable":            vamiBool("Whether this schedule is active."),
		"recurrence_info":   vamiObj(`Recurrence configuration, forwarded verbatim as-is. Postman documents the shape {"hour": <0-23>, "minute": <0-59>, "days": ["MONDAY", ...]}.`),
		"retention_info":    vamiObj(`Retention configuration, forwarded verbatim as-is. Postman documents the shape {"max_count": <integer>} (maximum number of backups to keep).`),
		"parts":             vamiArr("Optional backup parts to include beyond the default. See vmware_appliance_recovery_backup_parts for the parts available on this appliance."),
	}
}

// --- Request plumbing -----------------------------------------------------------

// vamiCall issues a VAMI /appliance/... request with an optional method,
// query parameters, and JSON body, decoding the response generically — this
// file's analogue of appliance.go's applianceGet, extended to also support
// non-GET methods/query-params/body (appliance.go's own applianceGet is
// untouched; every route it already serves keeps using it). Same rationale
// for the generic interface{} decode: no Go SDK wrapper and no
// simulator/live vCenter Server Appliance to check field names against (see
// this file's top doc comment).
func vamiCall(ctx context.Context, client *vmware.Client, method, path string, params map[string]string, body interface{}) (interface{}, error) {
	rc, err := client.REST(ctx)
	if err != nil {
		return nil, err // REST() already names the likely cause (no VAMI on standalone ESXi)
	}

	res := rc.Resource(path)
	for k, v := range params {
		res = res.WithParam(k, v)
	}

	var req *http.Request
	if body != nil {
		req = res.Request(method, body)
	} else {
		req = res.Request(method)
	}

	var result interface{}
	if err := rc.Do(ctx, req, &result); err != nil {
		return nil, fmt.Errorf("VAMI request (%s %s) failed: %w", method, path, err)
	}
	return result, nil
}

// vamiRequiredString extracts a required, non-empty string argument.
func vamiRequiredString(args map[string]interface{}, key string) (string, error) {
	v, ok := args[key].(string)
	if !ok || v == "" {
		return "", fmt.Errorf("%s is required", key)
	}
	return v, nil
}

// vamiBackupPieceBody builds the "piece" object shared by backup validate/
// create (includeCommentParts=true, adds comment+parts) and restore create
// (includeCommentParts=false) — see vamiBackupPieceSchemaProps for the same
// split on the schema side.
func vamiBackupPieceBody(args map[string]interface{}, includeCommentParts bool) (map[string]interface{}, error) {
	backupPassword, err := vamiRequiredString(args, "backup_password")
	if err != nil {
		return nil, err
	}
	location, err := vamiRequiredString(args, "location")
	if err != nil {
		return nil, err
	}
	locationType, err := vamiRequiredString(args, "location_type")
	if err != nil {
		return nil, err
	}

	piece := map[string]interface{}{
		"backup_password": backupPassword,
		"location":        location,
		"location_type":   locationType,
	}
	if v, ok := args["location_user"].(string); ok && v != "" {
		piece["location_user"] = v
	}
	if v, ok := args["location_password"].(string); ok && v != "" {
		piece["location_password"] = v
	}
	if includeCommentParts {
		if v, ok := args["comment"].(string); ok && v != "" {
			piece["comment"] = v
		}
		if v, ok := args["parts"]; ok && v != nil {
			piece["parts"] = v
		}
	}
	return piece, nil
}

// vamiScheduleSpecBody builds the "spec" object for backup schedule create/
// update. recurrence_info and retention_info are forwarded verbatim (see
// this file's top doc comment) — required as a whole object, not decomposed.
func vamiScheduleSpecBody(args map[string]interface{}) (map[string]interface{}, error) {
	backupPassword, err := vamiRequiredString(args, "backup_password")
	if err != nil {
		return nil, err
	}
	location, err := vamiRequiredString(args, "location")
	if err != nil {
		return nil, err
	}
	enable, ok := args["enable"].(bool)
	if !ok {
		return nil, fmt.Errorf("enable is required")
	}
	recurrenceInfo, ok := args["recurrence_info"]
	if !ok || recurrenceInfo == nil {
		return nil, fmt.Errorf("recurrence_info is required")
	}
	retentionInfo, ok := args["retention_info"]
	if !ok || retentionInfo == nil {
		return nil, fmt.Errorf("retention_info is required")
	}

	spec := map[string]interface{}{
		"backup_password": backupPassword,
		"location":        location,
		"enable":          enable,
		"recurrence_info": recurrenceInfo,
		"retention_info":  retentionInfo,
	}
	if v, ok := args["location_user"].(string); ok && v != "" {
		spec["location_user"] = v
	}
	if v, ok := args["location_password"].(string); ok && v != "" {
		spec["location_password"] = v
	}
	if v, ok := args["parts"]; ok && v != nil {
		spec["parts"] = v
	}
	return spec, nil
}

// vamiUserDataBody builds the optional "user_data" body shared by update
// install/stage-and-install/validate. Returns a true nil interface{} (not a
// nil map wrapped in an interface) when the caller omits "user_data", so
// vamiCall's "if body != nil" check works correctly — a nil
// map[string]interface{} assigned to an interface{} would NOT compare equal
// to nil (classic Go gotcha), so this function's return type must be
// interface{}, not map[string]interface{}.
func vamiUserDataBody(args map[string]interface{}) interface{} {
	v, ok := args["user_data"]
	if !ok || v == nil {
		return nil
	}
	return map[string]interface{}{"user_data": v}
}

// --- Handlers: Recovery / Backup job ------------------------------------------

func handleApplianceRecoveryBackupJobList(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	v, err := vamiCall(ctx, client, http.MethodGet, "/appliance/recovery/backup/job", nil, nil)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

func handleApplianceRecoveryBackupValidate(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	piece, err := vamiBackupPieceBody(args, true)
	if err != nil {
		return "", err
	}
	v, err := vamiCall(ctx, client, http.MethodPost, "/appliance/recovery/backup/validate", nil, map[string]interface{}{"piece": piece})
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

func handleApplianceRecoveryBackupJobCreate(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	piece, err := vamiBackupPieceBody(args, true)
	if err != nil {
		return "", err
	}
	v, err := vamiCall(ctx, client, http.MethodPost, "/appliance/recovery/backup/job", nil, map[string]interface{}{"piece": piece})
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

func handleApplianceRecoveryBackupJobCancel(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	id, err := vamiRequiredString(args, "backup_job_id")
	if err != nil {
		return "", err
	}
	path := "/appliance/recovery/backup/job/" + url.PathEscape(id) + "/cancel"
	v, err := vamiCall(ctx, client, http.MethodPost, path, nil, nil)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

func handleApplianceRecoveryBackupJobStatus(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	id, err := vamiRequiredString(args, "backup_job_id")
	if err != nil {
		return "", err
	}
	path := "/appliance/recovery/backup/job/" + url.PathEscape(id)
	v, err := vamiCall(ctx, client, http.MethodGet, path, nil, nil)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

func handleApplianceRecoveryBackupJobDetails(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	v, err := vamiCall(ctx, client, http.MethodGet, "/appliance/recovery/backup/job/details", nil, nil)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

func handleApplianceRecoveryBackupParts(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	v, err := vamiCall(ctx, client, http.MethodGet, "/appliance/recovery/backup/parts", nil, nil)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

func handleApplianceRecoveryBackupPartSize(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	id, err := vamiRequiredString(args, "part_id")
	if err != nil {
		return "", err
	}
	path := "/appliance/recovery/backup/parts/" + url.PathEscape(id)
	v, err := vamiCall(ctx, client, http.MethodGet, path, nil, nil)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

// --- Handlers: Recovery / Backup schedule -------------------------------------

func handleApplianceRecoveryBackupScheduleList(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	v, err := vamiCall(ctx, client, http.MethodGet, "/appliance/recovery/backup/schedules", nil, nil)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

func handleApplianceRecoveryBackupScheduleCreate(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	id, err := vamiRequiredString(args, "schedule_id")
	if err != nil {
		return "", err
	}
	spec, err := vamiScheduleSpecBody(args)
	if err != nil {
		return "", err
	}
	path := "/appliance/recovery/backup/schedules/" + url.PathEscape(id)
	v, err := vamiCall(ctx, client, http.MethodPost, path, nil, map[string]interface{}{"spec": spec})
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

func handleApplianceRecoveryBackupScheduleGet(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	id, err := vamiRequiredString(args, "schedule_id")
	if err != nil {
		return "", err
	}
	path := "/appliance/recovery/backup/schedules/" + url.PathEscape(id)
	v, err := vamiCall(ctx, client, http.MethodGet, path, nil, nil)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

func handleApplianceRecoveryBackupScheduleDelete(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	id, err := vamiRequiredString(args, "schedule_id")
	if err != nil {
		return "", err
	}
	path := "/appliance/recovery/backup/schedules/" + url.PathEscape(id)
	v, err := vamiCall(ctx, client, http.MethodDelete, path, nil, nil)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

// handleApplianceRecoveryBackupScheduleRun always sends a "comment" body key
// (defaulting to "" when the caller omits it) — Postman's own sample always
// includes that key, even though this tool's schema marks it optional.
func handleApplianceRecoveryBackupScheduleRun(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	id, err := vamiRequiredString(args, "schedule_id")
	if err != nil {
		return "", err
	}
	comment, _ := args["comment"].(string)
	// Trailing "/" before the query string preserved exactly as Postman
	// recorded it — see this file's top doc comment.
	path := "/appliance/recovery/backup/schedules/" + url.PathEscape(id) + "/"
	v, err := vamiCall(ctx, client, http.MethodPost, path, map[string]string{"action": "run"}, map[string]interface{}{"comment": comment})
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

func handleApplianceRecoveryBackupScheduleUpdate(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	id, err := vamiRequiredString(args, "schedule_id")
	if err != nil {
		return "", err
	}
	spec, err := vamiScheduleSpecBody(args)
	if err != nil {
		return "", err
	}
	path := "/appliance/recovery/backup/schedules/" + url.PathEscape(id)
	v, err := vamiCall(ctx, client, http.MethodPut, path, nil, map[string]interface{}{"spec": spec})
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

// --- Handlers: Recovery / Restore job -----------------------------------------

func handleApplianceRecoveryRestoreJobStatus(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	v, err := vamiCall(ctx, client, http.MethodGet, "/appliance/recovery/restore/job", nil, nil)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

func handleApplianceRecoveryRestoreJobCancel(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	v, err := vamiCall(ctx, client, http.MethodPost, "/appliance/recovery/restore/job/cancel", nil, nil)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

func handleApplianceRecoveryRestoreJobCreate(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	piece, err := vamiBackupPieceBody(args, false)
	if err != nil {
		return "", err
	}
	v, err := vamiCall(ctx, client, http.MethodPost, "/appliance/recovery/restore/job", nil, map[string]interface{}{"piece": piece})
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

// --- Handlers: Update / Check for updates ---------------------------------------

func handleApplianceUpdateCheckURLCdrom(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	v, err := vamiCall(ctx, client, http.MethodGet, "/appliance/update/pending", map[string]string{"source_type": "LOCAL_AND_ONLINE"}, nil)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

func handleApplianceUpdateCheckCdrom(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	v, err := vamiCall(ctx, client, http.MethodGet, "/appliance/update/pending", map[string]string{"source_type": "LOCAL"}, nil)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

func handleApplianceUpdateCheckLast(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	v, err := vamiCall(ctx, client, http.MethodGet, "/appliance/update/pending", map[string]string{"source_type": "LAST_CHECK"}, nil)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

func handleApplianceUpdatePendingDetails(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	version, err := vamiRequiredString(args, "version")
	if err != nil {
		return "", err
	}
	path := "/appliance/update/pending/" + url.PathEscape(version)
	v, err := vamiCall(ctx, client, http.MethodGet, path, nil, nil)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

// --- Handlers: Update / Policy ----------------------------------------------------

func handleApplianceUpdatePolicyGet(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	v, err := vamiCall(ctx, client, http.MethodGet, "/appliance/update/policy", nil, nil)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

func handleApplianceUpdatePolicySet(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	policy, ok := args["policy"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("policy is required and must be an object")
	}
	v, err := vamiCall(ctx, client, http.MethodPut, "/appliance/update/policy", nil, map[string]interface{}{"policy": policy})
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

// --- Handlers: Update / Stage --------------------------------------------------------

func handleApplianceUpdateStage(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	version, err := vamiRequiredString(args, "version")
	if err != nil {
		return "", err
	}
	path := "/appliance/update/pending/" + url.PathEscape(version)
	v, err := vamiCall(ctx, client, http.MethodPost, path, map[string]string{"action": "stage"}, nil)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

func handleApplianceUpdateStagedGet(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	v, err := vamiCall(ctx, client, http.MethodGet, "/appliance/update/staged", nil, nil)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

func handleApplianceUpdateStagedDelete(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	v, err := vamiCall(ctx, client, http.MethodDelete, "/appliance/update/staged", nil, nil)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

// --- Handlers: Update / Install -------------------------------------------------------

func handleApplianceUpdatePrecheck(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	version, err := vamiRequiredString(args, "version")
	if err != nil {
		return "", err
	}
	path := "/appliance/update/pending/" + url.PathEscape(version)
	v, err := vamiCall(ctx, client, http.MethodPost, path, map[string]string{"action": "precheck"}, nil)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

func handleApplianceUpdateInstall(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	version, err := vamiRequiredString(args, "version")
	if err != nil {
		return "", err
	}
	path := "/appliance/update/pending/" + url.PathEscape(version)
	v, err := vamiCall(ctx, client, http.MethodPost, path, map[string]string{"action": "install"}, vamiUserDataBody(args))
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

func handleApplianceUpdateStageAndInstall(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	version, err := vamiRequiredString(args, "version")
	if err != nil {
		return "", err
	}
	path := "/appliance/update/pending/" + url.PathEscape(version)
	v, err := vamiCall(ctx, client, http.MethodPost, path, map[string]string{"action": "stage-and-install"}, vamiUserDataBody(args))
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

func handleApplianceUpdateValidate(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	version, err := vamiRequiredString(args, "version")
	if err != nil {
		return "", err
	}
	path := "/appliance/update/pending/" + url.PathEscape(version)
	v, err := vamiCall(ctx, client, http.MethodPost, path, map[string]string{"action": "validate"}, vamiUserDataBody(args))
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

// --- Handlers: Update / Status ---------------------------------------------------

func handleApplianceUpdateStatus(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	v, err := vamiCall(ctx, client, http.MethodGet, "/appliance/update", nil, nil)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

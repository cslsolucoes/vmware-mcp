package tools

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/vmware/govmomi/object"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// registerDiagnosticTools is the "diagnostic" slice of Fase 7 (Grupo A) of
// the codegen plan (".workspace/plans/MCPVMWare2026-08-10-175300-plano-cobertura-completa-api-codegen.plan.md")
// — object.DiagnosticManager + object.DiagnosticLog
// (referencia/govmomi/object/diagnostic_manager.go,
// referencia/govmomi/object/diagnostic_log.go), hand-transcribed following
// the datastore.go/generated_datastore_browser.go conventions.
//
// vcsim gap, not a bug: referencia/govmomi/simulator has NO
// DiagnosticManager implementation whatsoever — confirmed by grepping
// "DiagnosticManager|BrowseDiagnosticLog|GenerateLogBundles|QueryDescriptions"
// across every file in referencia/govmomi/simulator/*.go, 0 matches. Every
// tool in this file is registered and exercised exactly like real vSphere
// supports it (ServiceContent.DiagnosticManager IS a valid, non-nil moref in
// both simulator.ESX()'s and simulator.VPX()'s default service content —
// confirmed in referencia/govmomi/simulator/esx and vpx/service_content.go —
// so object.NewDiagnosticManager itself never panics), but NO call in
// generated_diagnostic_test.go can reach a real success path: every SOAP
// call against that moref faults ManagedObjectNotFound (the object simply
// isn't registered server-side), not types.MethodNotFound like the
// per-method gaps documented in generated_vm_lifecycle.go — this is a gap in
// the whole managed object, not individual methods on an object that does
// exist. assertReachesServer (generated_vm_lifecycle_test.go, reused here)
// still applies: it only requires "not unknown tool, not a recovered
// panic", which a ManagedObjectNotFound fault satisfies.
//
// Curation:
//
//   - DiagnosticManager.BrowseLog is pure log-line reading with no
//     mutation — the AST classifier's Tier 2 default is corrected to no
//     tier here (vmware_diagnostic_browse_log), same reasoning as this
//     project's other pure-read tools (e.g. generated_datastore_browser.go's
//     vmware_datastore_stat).
//
//   - DiagnosticManager.Log(ctx, host, key) *DiagnosticLog is a pure
//     client-side constructor (no round trip, no error return) — excluded
//     as a standalone tool per the group's brief; it is used internally as
//     a helper by vmware_diagnostic_log_copy below instead.
//
//   - DiagnosticLog.Seek + DiagnosticLog.Copy are fused into one tool,
//     vmware_diagnostic_log_copy(host?, log_key, tail_lines?, max_bytes?):
//     internally builds a *DiagnosticLog via DiagnosticManager.Log, seeks to
//     the last tail_lines lines first if given, then copies from there.
//     DiagnosticLog.Copy's real signature (Copy(ctx, io.Writer)
//     (int, error)) has no byte cap of its own — it loops fetching up to
//     500/1000 lines per BrowseLog page (referencia/govmomi/object/diagnostic_log.go's
//     comment: "VC max == 500, ESX max == 1000") until the log's end,
//     which could be unbounded against a real, very large log file. This
//     tool bounds it with truncatingWriter below (the same "hard,
//     non-configurable ceiling on top of a caller-adjustable default"
//     pattern as generated_datastore_browser.go's vmware_datastore_open,
//     reusing that file's datastoreOpenHardCapBytes constant as the ceiling
//     here too) instead of buffering an entire log unbounded. Kept at
//     Tier 2, not left as a plain read — matching this project's existing
//     precedent for tools that expose potentially large amounts of raw file/
//     log content (generated_datastore_browser.go's vmware_datastore_open/
//     vmware_datastore_download_file/vmware_datastore_service_ticket are
//     all Tier 2 despite being reads, for the same "same sensitivity class
//     as local disk access" reasoning documented there), applied
//     consistently rather than re-litigated per file.
//
//   - DiagnosticManager.GenerateLogBundles returns a *object.Task — waited
//     on via vm.go's shared waitForTask (reused, not duplicated) per this
//     group's brief. Trade-off, accepted per that instruction: waitForTask
//     only returns an error, discarding the real success TaskInfo.Result
//     (which for this specific task is the interesting part — a
//     []types.DiagnosticManagerBundleInfo with download URLs for the
//     generated bundles). This is not a regression specific to this tool —
//     every other Tier 2 *object.Task-based tool in this codebase
//     (generated_vm_lifecycle.go's vmware_vm_upgrade_vm/
//     vmware_vm_upgrade_tools, etc.) has the exact same limitation by using
//     the same shared helper, and this whole domain is unexercisable
//     against vcsim regardless (see this file's top doc comment) so there
//     was no way to validate a richer alternative against a real success
//     path here anyway.
func registerDiagnosticTools(r *Registry) {
	hostArg := map[string]interface{}{
		"type":        "string",
		"description": `Host identifier (see vmware_list_hosts). Optional — omit to target vCenter's own log (vpxd) instead of a specific ESXi host's.`,
	}
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}

	r.register("vmware_diagnostic_browse_log",
		"Read a slice of lines from a diagnostic log (e.g. vpxd, hostd) by key, starting at a given line. Use vmware_diagnostic_query_descriptions to discover valid log keys.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":  hostArg,
				"key":   map[string]interface{}{"type": "string", "description": `Log identifier, e.g. "hostd" or "vpxd" (see vmware_diagnostic_query_descriptions).`},
				"start": map[string]interface{}{"type": "integer", "description": "Line number to start reading from. Default 0 (start of log)."},
				"lines": map[string]interface{}{"type": "integer", "description": "Maximum number of lines to return. Default 0 (server default, typically returns just the header/no text)."},
			},
			"required": []interface{}{"key"},
		},
		Tool{Handler: handleDiagnosticBrowseLog},
	)

	r.register("vmware_diagnostic_query_descriptions",
		"List the diagnostic log keys available on vCenter or a specific ESXi host, with their descriptions.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"host": hostArg},
		},
		Tool{Handler: handleDiagnosticQueryDescriptions},
	)

	r.registerDestructive("vmware_diagnostic_generate_log_bundles",
		"Generate a support log bundle for vCenter and/or one or more ESXi hosts. Blocks until the underlying task completes; the download URL(s) for the generated bundle(s) are NOT returned by this tool (see this file's top doc comment) — this only confirms the bundle(s) were generated.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"include_default": map[string]interface{}{"type": "boolean", "description": "Include the default log bundle (vCenter Server's own logs) in addition to any hosts listed. Default false."},
				"hosts":           map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Host identifiers to include (see vmware_list_hosts). Omit for none (only meaningful combined with include_default:true, or against a standalone ESXi host)."},
				"confirm":         confirmArg,
			},
			"required": []interface{}{"confirm"},
		},
		Tool{Handler: handleDiagnosticGenerateLogBundles},
	)

	r.registerDestructive("vmware_diagnostic_log_copy",
		"Read (a bounded amount of) a diagnostic log's text content, optionally starting from only the last tail_lines lines. See this file's top doc comment for why this is Tier 2 despite being a read.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host":       hostArg,
				"log_key":    map[string]interface{}{"type": "string", "description": `Log identifier, e.g. "hostd" or "vpxd" (see vmware_diagnostic_query_descriptions).`},
				"tail_lines": map[string]interface{}{"type": "integer", "description": "If given, seek to only the last N lines of the log before copying, instead of starting from the beginning."},
				"max_bytes":  map[string]interface{}{"type": "integer", "description": "Maximum number of bytes of log text to return. Default 2097152 (2MiB); hard-capped server-side regardless of what is requested."},
				"confirm":    confirmArg,
			},
			"required": []interface{}{"log_key", "confirm"},
		},
		Tool{Handler: handleDiagnosticLogCopy},
	)
}

// diagnosticLogCopyDefaultMaxBytes bounds vmware_diagnostic_log_copy when
// the caller omits max_bytes — see this file's top doc comment. The hard
// ceiling reuses generated_datastore_browser.go's datastoreOpenHardCapBytes
// constant rather than defining a second one for the same purpose.
const diagnosticLogCopyDefaultMaxBytes = 2 * 1024 * 1024

// errDiagnosticLogTruncated is truncatingWriter's sentinel — caught by
// handleDiagnosticLogCopy to distinguish "stopped early because max_bytes
// was hit" (truncated:true, not a tool failure) from a real I/O error.
var errDiagnosticLogTruncated = errors.New("diagnostic log truncated at max_bytes")

// truncatingWriter caps the bytes DiagnosticLog.Copy (object/diagnostic_log.go)
// is allowed to buffer — Copy has no byte cap of its own (see this file's
// top doc comment) and loops fetching further log pages until the log's
// real end, which is unbounded for a large log file. Once max is reached,
// Write returns errDiagnosticLogTruncated so Copy's own
// "return written, err" stops it from fetching further pages.
type truncatingWriter struct {
	buf *bytes.Buffer
	max int
}

func (w *truncatingWriter) Write(p []byte) (int, error) {
	remaining := w.max - w.buf.Len()
	if remaining <= 0 {
		return 0, errDiagnosticLogTruncated
	}
	if len(p) > remaining {
		n, _ := w.buf.Write(p[:remaining])
		return n, errDiagnosticLogTruncated
	}
	return w.buf.Write(p)
}

// resolveOptionalHost resolves the optional "host" argument to a
// *object.HostSystem, or (nil, nil) when omitted — DiagnosticManager's
// methods all accept a nil host to mean "vCenter itself" rather than a
// specific ESXi host (see e.g. DiagnosticManager.BrowseLog's own nil check
// before setting req.Host). Reuses host.go's resolveHost, which already
// reads args["host"] directly, so no argument remapping is needed.
func resolveOptionalHost(ctx context.Context, client *vmware.Client, args map[string]interface{}) (*object.HostSystem, error) {
	name, _ := args["host"].(string)
	if name == "" {
		return nil, nil
	}
	return resolveHost(ctx, client, args)
}

// resolveOptionalHosts resolves the optional "hosts" array argument (host
// name/pattern strings) to a slice of *object.HostSystem, or (nil, nil) when
// omitted entirely.
func resolveOptionalHosts(ctx context.Context, client *vmware.Client, args map[string]interface{}) ([]*object.HostSystem, error) {
	raw, ok := args["hosts"]
	if !ok {
		return nil, nil
	}
	names, err := toStringSlice(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid hosts: %w", err)
	}
	hosts := make([]*object.HostSystem, 0, len(names))
	for _, name := range names {
		h, err := resolveHost(ctx, client, map[string]interface{}{"host": name})
		if err != nil {
			return nil, err
		}
		hosts = append(hosts, h)
	}
	return hosts, nil
}

func handleDiagnosticBrowseLog(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveOptionalHost(ctx, client, args)
	if err != nil {
		return "", err
	}
	key, _ := args["key"].(string)
	if key == "" {
		return "", fmt.Errorf("key is required")
	}

	var start, lines int32
	if v, ok := args["start"]; ok {
		n, err := toInt32(v)
		if err != nil {
			return "", fmt.Errorf("invalid start: %w", err)
		}
		start = n
	}
	if v, ok := args["lines"]; ok {
		n, err := toInt32(v)
		if err != nil {
			return "", fmt.Errorf("invalid lines: %w", err)
		}
		lines = n
	}

	dm := object.NewDiagnosticManager(client.Client.Client)
	header, err := dm.BrowseLog(ctx, host, key, start, lines)
	if err != nil {
		return "", fmt.Errorf("failed to browse diagnostic log %q: %w", key, err)
	}

	return marshalJSON(map[string]interface{}{"key": key, "header": header})
}

func handleDiagnosticQueryDescriptions(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveOptionalHost(ctx, client, args)
	if err != nil {
		return "", err
	}

	dm := object.NewDiagnosticManager(client.Client.Client)
	descs, err := dm.QueryDescriptions(ctx, host)
	if err != nil {
		return "", fmt.Errorf("failed to query diagnostic log descriptions: %w", err)
	}

	return marshalJSON(map[string]interface{}{"count": len(descs), "descriptions": descs})
}

func handleDiagnosticGenerateLogBundles(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	includeDefault, _ := args["include_default"].(bool)
	hosts, err := resolveOptionalHosts(ctx, client, args)
	if err != nil {
		return "", err
	}

	dm := object.NewDiagnosticManager(client.Client.Client)
	task, err := dm.GenerateLogBundles(ctx, includeDefault, hosts)
	if err != nil {
		return "", fmt.Errorf("failed to start log bundle generation: %w", err)
	}
	if err := waitForTask(ctx, task); err != nil {
		return "", fmt.Errorf("generate-log-bundles task failed: %w", err)
	}

	return marshalJSON(map[string]interface{}{
		"include_default": includeDefault,
		"host_count":      len(hosts),
		"result":          "log_bundles_generated",
	})
}

func handleDiagnosticLogCopy(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveOptionalHost(ctx, client, args)
	if err != nil {
		return "", err
	}
	key, _ := args["log_key"].(string)
	if key == "" {
		return "", fmt.Errorf("log_key is required")
	}

	maxBytes := diagnosticLogCopyDefaultMaxBytes
	if v, ok := args["max_bytes"]; ok {
		n, err := toInt64(v)
		if err != nil {
			return "", fmt.Errorf("invalid max_bytes: %w", err)
		}
		if n <= 0 {
			return "", fmt.Errorf("max_bytes must be positive, got %d", n)
		}
		maxBytes = int(n)
	}
	if maxBytes > datastoreOpenHardCapBytes {
		maxBytes = datastoreOpenHardCapBytes
	}

	dm := object.NewDiagnosticManager(client.Client.Client)
	dl := dm.Log(ctx, host, key)

	if v, ok := args["tail_lines"]; ok {
		n, err := toInt32(v)
		if err != nil {
			return "", fmt.Errorf("invalid tail_lines: %w", err)
		}
		if err := dl.Seek(ctx, n); err != nil {
			return "", fmt.Errorf("failed to seek diagnostic log %q: %w", key, err)
		}
	}

	var buf bytes.Buffer
	w := &truncatingWriter{buf: &buf, max: maxBytes}
	n, err := dl.Copy(ctx, w)
	truncated := errors.Is(err, errDiagnosticLogTruncated)
	if err != nil && !truncated {
		return "", fmt.Errorf("failed to copy diagnostic log %q: %w", key, err)
	}

	return marshalJSON(map[string]interface{}{
		"log_key":       key,
		"bytes_written": n,
		"truncated":     truncated,
		"content":       buf.String(),
	})
}

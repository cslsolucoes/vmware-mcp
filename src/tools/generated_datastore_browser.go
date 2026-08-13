package tools

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"

	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// registerDatastoreBrowserTools is the "datastore" slice of Fase 4 of the
// codegen plan (".workspace/plans/MCPVMWare2026-08-10-175300-plano-cobertura-completa-api-codegen.plan.md")
// — object.Datastore (7 of its 12 methods; the other 5 were excluded ahead
// of time by the group's brief after reading the real source, not
// second-guessed here: Upload/Download take/return raw io.Reader/
// io.ReadCloser streams with no JSON-RPC representation, HostContext returns
// a bare context.Context, FindInventoryPath only mutates the receiver's own
// fields and returns nothing, Browser() is a plumbing accessor folded
// directly into the 2 search tools below instead of its own tool) +
// object.HostDatastoreBrowser (2 methods) + object.DatastoreNamespaceManager
// (3 methods) = 12 tools, hand-transcribed from the real
// referencia/govmomi/object/datastore.go, datastore_file.go,
// host_datastore_browser.go, and namespace_manager.go sources following the
// datastore.go/host.go/generated_vm_lifecycle.go/generated_host_storage.go
// conventions.
//
// Headline finding vs. Fase 3's host-storage slice (where only 5/20 methods
// had any vcsim implementation): every one of these 12 has genuine
// server-side vcsim support — confirmed per method by grepping the exact
// SOAP method/request-type name across referencia/govmomi/simulator, not by
// receiver/file name:
//
//   - AttachedHosts/AttachedClusterHosts are not SOAP methods at all — they
//     are built purely from property-collector reads (Datastore.host,
//     HostSystem.runtime.connectionState/powerState), which vcsim's generic
//     property collector always serves for real.
//   - Stat (internally calls SearchDatastore_Task) and Type (reads
//     summary.type) are both real for the same reason.
//   - SearchDatastore_Task/SearchDatastoreSubFolders_Task are implemented by
//     simulator.HostDatastoreBrowser (simulator/host_datastore_browser.go).
//   - CreateDirectory/DeleteDirectory/ConvertNamespacePathToUuidPath are
//     implemented by simulator.DatastoreNamespaceManager
//     (simulator/datastore_namespace_manager.go).
//   - DownloadFile/Open/ServiceTicket reach vcsim's real /folder HTTP
//     handler (simulator/simulator.go's ServeDatastore) — see the dedicated
//     note below on why this is a genuine HTTP round trip, not a
//     registration-only stub.
//
// Curation deviations from the brief (human review required):
//
//   - vmware_datastore_open: Datastore.Open(ctx, name) returns a
//     *object.DatastoreFile implementing io.Reader/io.Seeker/io.Closer — no
//     further tool in this batch to hand a live Go handle to over MCP's
//     JSON-RPC boundary. Per the brief's explicit fallback ("if the only
//     sensible action is read N bytes and return as base64/string,
//     implement that directly in the handler"), this reads up to max_bytes
//     (default 64KiB, hard-capped at 10MiB server-side regardless of what
//     the caller asks for, to bound both memory and the JSON response size)
//     starting at an optional byte offset (via DatastoreFile.Seek), and
//     returns the bytes as base64 plus a "truncated" flag so a caller can
//     tell a short read (real EOF) apart from a capped one. Kept at Tier 2
//     per the brief, alongside DownloadFile and ServiceTicket, despite being
//     a read — the brief's rationale (these 3 expose datastore file
//     contents or access tickets, the same sensitivity class as local disk
//     access in vmware_datastore_upload_file/vmware_datastore_download_file)
//     is followed as given.
//
//   - vmware_datastore_search / vmware_datastore_search_subfolders default
//     search_spec to {MatchPattern: ["*"]} when the caller omits it,
//     instead of passing a nil *types.HostDatastoreBrowserSearchSpec
//     through. Confirmed by reading simulator/host_datastore_browser.go's
//     searchDatastore.queryMatch and .addFile: both dereference
//     s.SearchSpec.Query / s.SearchSpec.Details directly with no nil guard,
//     so a nil SearchSpec panics inside the simulator's task Run() — CallTool's
//     recover() (registry.go) would turn that into an ugly "tool ...
//     panicked" error instead of a normal result. Defaulting to "match every
//     file under path" both dodges that and is the more useful default for
//     a caller who doesn't care about file-type filtering.
//
//   - vmware_datastore_attached_hosts / vmware_datastore_attached_cluster_hosts:
//     Datastore.AttachedHosts builds each returned *object.HostSystem via a
//     bare NewHostSystem(c, mount.Key) straight from a
//     DatastoreHostMount.Key moref, never through a Finder (confirmed by
//     reading object/datastore.go's AttachedHosts source) — so
//     .InventoryPath is always "" on every entry, the same InventoryPath
//     gotcha documented in generated_vm_lifecycle.go's and
//     generated_host_storage.go's top comments. hostInventoryPaths below
//     resolves the real path via find.InventoryPath for each host instead
//     of trusting .InventoryPath directly.
//
//   - vmware_datastore_attached_cluster_hosts's "cluster" argument resolves
//     via client.Finder.ComputeResource(ctx, dcScopedPath("host", name)) —
//     verified the real Finder method name is ComputeResource (not
//     "Cluster") and that it is scoped under the per-datacenter "host"
//     folder (find/finder.go's ComputeResourceList uses
//     Relative: f.hostFolder), the same folder as vmware_host_info and
//     vmware_list_resource_pools/vmware_list_clusters — by reading
//     find/finder.go directly, not assumed from the method name.
//
//   - vmware_datastore_namespace_convert_path_to_uuid and
//     vmware_datastore_namespace_delete_directory need a *object.Datacenter,
//     which no existing helper in this package resolves by itself (resolveVM/
//     resolveHost/resolveDatastore all resolve their own type; datacenter
//     listing only exists as DatacenterList in inventory.go). A small local
//     resolveDatacenter(ctx, client, name) helper is added at the bottom of
//     this file instead of touching finder.go, which is off-limits per this
//     group's hard requirement to create only 2 new files.
//
//   - DatastoreNamespaceManager's 3 methods are genuinely VSAN/VVol-only in
//     real vSphere, not just in vcsim: simulator.DatastoreNamespaceManager's
//     CreateDirectory/DeleteDirectory/ConvertNamespacePathToUuidPath all
//     call internal.IsDatastoreVSAN(ds) and fault
//     (CannotCreateFile/FileFault/InvalidDatastore respectively) against any
//     other datastore type — confirmed by reading
//     simulator/datastore_namespace_manager.go directly, and by finding
//     govmomi's own object/namespace_manager_test.go, which reaches its
//     success path only after manually flipping
//     simulator.Map(ctx).Get(ds.Reference()).(*simulator.Datastore).Summary.Type
//     to "vsan" via the in-process simulator registry. vcsim's default model
//     datastores (both simulator.ESX() and simulator.VPX()) are plain
//     NFS/local, not VSAN, so a call against them with these 3 tools
//     genuinely faults — a real business-rule limitation reachable against
//     real vSphere too (same class as generated_vm_lifecycle.go's
//     RebootGuest/UpgradeVM note), not a vcsim simulation gap.
//     generated_datastore_browser_test.go reaches a real, full success path
//     for all 3 anyway by using the exact same
//     simulator.Map(model.Service.Context).Get(...).(*simulator.Datastore)
//     mutation as govmomi's own test, obtained via the already-exported
//     model.Service.Context field after newSimClient starts the server (no
//     edit to testhelpers_test.go needed).
//
//   - vmware_datastore_download_file / vmware_datastore_service_ticket /
//     vmware_datastore_open are genuine HTTP round trips against vcsim, not
//     registration-only stubs: Datastore.ServiceTicket only calls the
//     AcquireGenericServiceTicket SOAP method (also implemented by vcsim,
//     simulator/session_manager.go — confirmed, not assumed) when
//     useServiceTicket() is true, which requires both IsVC() and the
//     GOVMOMI_USE_SERVICE_TICKET env var/query param set — neither holds in
//     any test here (simulator.ESX() is never IsVC(), and no test sets that
//     var), so the common path returns (rawURL, nil cookie, nil error)
//     straight away and the subsequent GET/PUT goes directly against
//     vcsim's /folder HTTP handler (simulator/simulator.go's ServeDatastore,
//     confirmed to be wired via mux.HandleFunc(folderPrefix, ...) and to
//     really read/write files under the datastore's backing directory) —
//     these 3 tools are exercised for real, end to end, in
//     generated_datastore_browser_test.go, not just proven to reach a
//     MethodNotFound fault.
//
//   - Tier assignments follow the brief as given: read-only (no tier) for
//     AttachedClusterHosts, AttachedHosts, Stat, Type, SearchDatastore,
//     SearchDatastoreSubFolders; Tier 2 for DownloadFile, Open,
//     ServiceTicket, ConvertNamespacePathToUuidPath, CreateDirectory; Tier 1
//     for DeleteDirectory (irreversible).
func registerDatastoreBrowserTools(r *Registry) {
	datastoreArg := map[string]interface{}{
		"type":        "string",
		"description": `Datastore name/pattern (e.g. "datastore1") as returned by vmware_list_datastores. Must resolve to exactly one datastore.`,
	}
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}
	searchSpecArg := map[string]interface{}{
		"type":        "object",
		"description": `A types.HostDatastoreBrowserSearchSpec JSON object matching its Go struct fields ("matchPattern" glob list, "query" file-type filter list, "details" flags for size/owner/modification/type, "searchCaseInsensitive"). Omit to match every file under path (matchPattern defaults to ["*"]).`,
	}

	// --- Datastore: read-only --------------------------------------------

	r.register("vmware_datastore_attached_cluster_hosts",
		"List hosts in a given cluster that have a datastore attached, accessible, and writable.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"datastore": datastoreArg,
				"cluster":   map[string]interface{}{"type": "string", "description": `Cluster/compute-resource name/pattern (e.g. "cluster1") as returned by vmware_list_clusters. Must resolve to exactly one cluster.`},
			},
			"required": []interface{}{"datastore", "cluster"},
		},
		Tool{Handler: handleDatastoreAttachedClusterHosts},
	)

	r.register("vmware_datastore_attached_hosts",
		"List every host that has a datastore attached, accessible, and writable.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"datastore": datastoreArg},
			"required":   []interface{}{"datastore"},
		},
		Tool{Handler: handleDatastoreAttachedHosts},
	)

	r.register("vmware_datastore_stat",
		`Get file metadata (size, modification time, owner, type) for a single file on a datastore. Reports {"exists": false} instead of an error if the file/its parent directory does not exist.`,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"datastore": datastoreArg,
				"file":      map[string]interface{}{"type": "string", "description": `Path to the file within the datastore, e.g. "ISOs/ubuntu-24.04.iso".`},
			},
			"required": []interface{}{"datastore", "file"},
		},
		Tool{Handler: handleDatastoreStat},
	)

	r.register("vmware_datastore_type",
		"Get the file system volume type of a datastore (e.g. VMFS, NFS, NFS41, CIFS, vsan, VVOL).",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"datastore": datastoreArg},
			"required":   []interface{}{"datastore"},
		},
		Tool{Handler: handleDatastoreType},
	)

	// --- Datastore: Tier 2 (disruptive but reversible) -------------------

	r.registerDestructive("vmware_datastore_download_file",
		"Download a file from a path inside a vSphere datastore to the filesystem of the machine running this MCP server. local_path is written on this server's own filesystem — NOT streamed back to the MCP client/chat — and is silently overwritten if it already exists (govmomi's own download implementation has no overwrite guard, unlike this project's vmware_datastore_upload_file for the remote side).",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"datastore":   datastoreArg,
				"remote_path": map[string]interface{}{"type": "string", "description": `Source path inside the datastore, e.g. "ISOs/ubuntu-24.04.iso".`},
				"local_path":  map[string]interface{}{"type": "string", "description": "Destination path on the machine running this MCP server. Overwritten if it already exists."},
				"confirm":     confirmArg,
			},
			"required": []interface{}{"datastore", "remote_path", "local_path", "confirm"},
		},
		Tool{Handler: handleDatastoreDownloadFile},
	)

	r.registerDestructive("vmware_datastore_open",
		"Read up to max_bytes of a file's contents from a datastore, starting at an optional byte offset, and return them base64-encoded. Not a full file transfer — use vmware_datastore_download_file to save a complete file to this server's local disk. max_bytes defaults to 65536 (64KiB) and is capped at 10485760 (10MiB) server-side regardless of what is requested; the response's \"truncated\" field reports whether more data remained beyond the returned chunk.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"datastore":   datastoreArg,
				"remote_path": map[string]interface{}{"type": "string", "description": `Path to the file within the datastore, e.g. "logs/vmware.log".`},
				"max_bytes":   map[string]interface{}{"type": "integer", "description": "Maximum number of bytes to read. Default 65536 (64KiB); capped at 10485760 (10MiB)."},
				"offset":      map[string]interface{}{"type": "integer", "description": "Byte offset to start reading from. Default 0 (start of file)."},
				"confirm":     confirmArg,
			},
			"required": []interface{}{"datastore", "remote_path", "confirm"},
		},
		Tool{Handler: handleDatastoreOpen},
	)

	r.registerDestructive("vmware_datastore_service_ticket",
		"Acquire a URL (and, against vCenter with service ticketing enabled, an access cookie) that can be used for a separate out-of-band HTTP request to a file on a datastore. Returns the URL and cookie as plain strings — no HTTP request is made by this tool itself beyond acquiring the ticket.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"datastore":   datastoreArg,
				"remote_path": map[string]interface{}{"type": "string", "description": `Path to the file within the datastore, e.g. "ISOs/ubuntu-24.04.iso".`},
				"method":      map[string]interface{}{"type": "string", "description": `HTTP method the ticket/URL will be used with, e.g. "GET" or "PUT".`},
				"confirm":     confirmArg,
			},
			"required": []interface{}{"datastore", "remote_path", "method", "confirm"},
		},
		Tool{Handler: handleDatastoreServiceTicket},
	)

	// --- HostDatastoreBrowser: read-only ----------------------------------

	r.register("vmware_datastore_search",
		"Search a single folder on a datastore for files matching a search spec (non-recursive — use vmware_datastore_search_subfolders to also search subfolders).",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"datastore":   datastoreArg,
				"path":        map[string]interface{}{"type": "string", "description": `Folder within the datastore to search, e.g. "ISOs" or "" for the datastore root.`},
				"search_spec": searchSpecArg,
			},
			"required": []interface{}{"datastore", "path"},
		},
		Tool{Handler: handleDatastoreSearch},
	)

	r.register("vmware_datastore_search_subfolders",
		"Recursively search a folder on a datastore and all its subfolders for files matching a search spec.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"datastore":   datastoreArg,
				"path":        map[string]interface{}{"type": "string", "description": `Folder within the datastore to search recursively, e.g. "ISOs" or "" for the datastore root.`},
				"search_spec": searchSpecArg,
			},
			"required": []interface{}{"datastore", "path"},
		},
		Tool{Handler: handleDatastoreSearchSubFolders},
	)

	// --- DatastoreNamespaceManager: Tier 2 --------------------------------

	r.registerDestructive("vmware_datastore_namespace_convert_path_to_uuid",
		"Convert a namespace (friendly-name) path on a vSAN/VVol datastore to its underlying UUID-based path. Only supported on vSAN/VVol datastores — faults against any other datastore type (see this file's top doc comment).",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"datacenter":    map[string]interface{}{"type": "string", "description": `Datacenter name/pattern (e.g. "ha-datacenter") as returned by vmware_list_datacenters. Must resolve to exactly one datacenter.`},
				"datastore_url": map[string]interface{}{"type": "string", "description": `Namespace URL/path to convert, e.g. the value returned as "directory" by vmware_datastore_namespace_create_directory.`},
				"confirm":       confirmArg,
			},
			"required": []interface{}{"datacenter", "datastore_url", "confirm"},
		},
		Tool{Handler: handleDatastoreNamespaceConvertPathToUuid},
	)

	r.registerDestructive("vmware_datastore_namespace_create_directory",
		"Create a top-level namespace directory on a vSAN/VVol datastore. Only supported on vSAN/VVol datastores — faults against any other datastore type (see this file's top doc comment).",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"datastore":    datastoreArg,
				"display_name": map[string]interface{}{"type": "string", "description": "User-facing display name hint for the new directory."},
				"policy":       map[string]interface{}{"type": "string", "description": "Opaque storage policy to apply to the directory. Optional."},
				"confirm":      confirmArg,
			},
			"required": []interface{}{"datastore", "display_name", "confirm"},
		},
		Tool{Handler: handleDatastoreNamespaceCreateDirectory},
	)

	// --- DatastoreNamespaceManager: Tier 1 (irreversible) -----------------

	r.registerDestructive("vmware_datastore_namespace_delete_directory",
		"Delete a top-level namespace directory from a vSAN/VVol datastore. Irreversible. Only supported on vSAN/VVol datastores — faults against any other datastore type (see this file's top doc comment).",
		tier1,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"datacenter":     map[string]interface{}{"type": "string", "description": `Datacenter name/pattern (e.g. "ha-datacenter") as returned by vmware_list_datacenters. Must resolve to exactly one datacenter.`},
				"datastore_path": map[string]interface{}{"type": "string", "description": `Datastore path of the directory to delete, e.g. the value returned as "directory" by vmware_datastore_namespace_create_directory.`},
				"confirm":        confirmArg,
			},
			"required": []interface{}{"datacenter", "datastore_path", "confirm"},
		},
		Tool{Handler: handleDatastoreNamespaceDeleteDirectory},
	)
}

// datastoreOpenDefaultMaxBytes/datastoreOpenHardCapBytes bound
// vmware_datastore_open — see its schema description and this file's top
// doc comment for why a hard, non-configurable ceiling exists regardless of
// what a caller requests (unbounded memory use and an unbounded JSON
// response otherwise).
const (
	datastoreOpenDefaultMaxBytes = 64 * 1024
	datastoreOpenHardCapBytes    = 10 * 1024 * 1024
)

// resolveDatacenter resolves the given name/pattern to exactly one
// Datacenter. A small local helper — finder.go is off-limits for this group
// (only 2 new files allowed per the hard requirements) and no existing
// helper in this package resolves a single Datacenter (inventory.go's
// handleListDatacenters only ever lists them).
func resolveDatacenter(ctx context.Context, client *vmware.Client, name string) (*object.Datacenter, error) {
	if name == "" {
		return nil, fmt.Errorf("datacenter is required")
	}
	dc, err := client.Finder.Datacenter(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve datacenter %q: %w", name, err)
	}
	return dc, nil
}

// hostInventoryPaths resolves each host's real inventory path via
// find.InventoryPath — Datastore.AttachedHosts/AttachedClusterHosts build
// each *object.HostSystem via a bare NewHostSystem(c, ref) from a raw
// DatastoreHostMount.Key, never through a Finder, so .InventoryPath is
// always "" on every entry (see this file's top doc comment).
func hostInventoryPaths(ctx context.Context, client *vmware.Client, hosts []*object.HostSystem) ([]string, error) {
	paths := make([]string, 0, len(hosts))
	for _, h := range hosts {
		p, err := find.InventoryPath(ctx, client.Client.Client, h.Reference())
		if err != nil {
			return nil, fmt.Errorf("failed to resolve inventory path for host %s: %w", h.Reference().Value, err)
		}
		paths = append(paths, p)
	}
	return paths, nil
}

// datastoreBrowser fetches ds's HostDatastoreBrowser — the exclusions list
// in this file's top doc comment explains why Datastore.Browser() has no
// standalone tool of its own; vmware_datastore_search and
// vmware_datastore_search_subfolders call this internally instead.
func datastoreBrowser(ctx context.Context, ds *object.Datastore) (*object.HostDatastoreBrowser, error) {
	b, err := ds.Browser(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get datastore browser for %s: %w", ds.InventoryPath, err)
	}
	return b, nil
}

// decodeSearchSpec decodes the optional "search_spec" argument, defaulting
// to {MatchPattern: ["*"]} when omitted — see this file's top doc comment
// for why a nil spec is not safe to pass through to vcsim.
func decodeSearchSpec(args map[string]interface{}) (*types.HostDatastoreBrowserSearchSpec, error) {
	var spec types.HostDatastoreBrowserSearchSpec
	if raw, ok := args["search_spec"]; ok {
		if err := decodeJSONArg(raw, &spec); err != nil {
			return nil, fmt.Errorf("invalid search_spec: %w", err)
		}
		return &spec, nil
	}
	spec.MatchPattern = []string{"*"}
	return &spec, nil
}

func handleDatastoreAttachedClusterHosts(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	dsName, _ := args["datastore"].(string)
	ds, err := resolveDatastore(ctx, client, dsName)
	if err != nil {
		return "", err
	}
	clusterName, _ := args["cluster"].(string)
	if clusterName == "" {
		return "", fmt.Errorf("cluster is required")
	}
	cr, err := client.Finder.ComputeResource(ctx, dcScopedPath("host", clusterName))
	if err != nil {
		return "", fmt.Errorf("failed to resolve cluster %q: %w", clusterName, err)
	}

	hosts, err := ds.AttachedClusterHosts(ctx, cr)
	if err != nil {
		return "", fmt.Errorf("failed to list attached hosts of cluster %s on datastore %s: %w", cr.InventoryPath, ds.InventoryPath, err)
	}
	paths, err := hostInventoryPaths(ctx, client, hosts)
	if err != nil {
		return "", err
	}

	return marshalJSON(map[string]interface{}{
		"datastore": ds.InventoryPath,
		"cluster":   cr.InventoryPath,
		"count":     len(paths),
		"hosts":     paths,
	})
}

func handleDatastoreAttachedHosts(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	dsName, _ := args["datastore"].(string)
	ds, err := resolveDatastore(ctx, client, dsName)
	if err != nil {
		return "", err
	}

	hosts, err := ds.AttachedHosts(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to list attached hosts on datastore %s: %w", ds.InventoryPath, err)
	}
	paths, err := hostInventoryPaths(ctx, client, hosts)
	if err != nil {
		return "", err
	}

	return marshalJSON(map[string]interface{}{
		"datastore": ds.InventoryPath,
		"count":     len(paths),
		"hosts":     paths,
	})
}

func handleDatastoreStat(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	dsName, _ := args["datastore"].(string)
	ds, err := resolveDatastore(ctx, client, dsName)
	if err != nil {
		return "", err
	}
	file, _ := args["file"].(string)
	if file == "" {
		return "", fmt.Errorf("file is required")
	}

	info, err := ds.Stat(ctx, file)
	if err != nil {
		if datastoreFileMissing(err) {
			return marshalJSON(map[string]interface{}{"datastore": ds.InventoryPath, "file": file, "exists": false})
		}
		return "", fmt.Errorf("failed to stat %s on datastore %s: %w", file, ds.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{
		"datastore": ds.InventoryPath,
		"file":      file,
		"exists":    true,
		"info":      info,
	})
}

func handleDatastoreType(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	dsName, _ := args["datastore"].(string)
	ds, err := resolveDatastore(ctx, client, dsName)
	if err != nil {
		return "", err
	}

	fsType, err := ds.Type(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to read filesystem type for datastore %s: %w", ds.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"datastore": ds.InventoryPath, "type": string(fsType)})
}

func handleDatastoreDownloadFile(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	dsName, _ := args["datastore"].(string)
	ds, err := resolveDatastore(ctx, client, dsName)
	if err != nil {
		return "", err
	}
	remotePath, _ := args["remote_path"].(string)
	if remotePath == "" {
		return "", fmt.Errorf("remote_path is required")
	}
	localPath, _ := args["local_path"].(string)
	if localPath == "" {
		return "", fmt.Errorf("local_path is required")
	}

	if err := ds.DownloadFile(ctx, remotePath, localPath, nil); err != nil {
		return "", fmt.Errorf("failed to download %s from datastore %s to %s: %w", remotePath, ds.InventoryPath, localPath, err)
	}

	return marshalJSON(map[string]interface{}{
		"datastore":   ds.InventoryPath,
		"remote_path": remotePath,
		"local_path":  localPath,
		"result":      "downloaded",
	})
}

func handleDatastoreOpen(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	dsName, _ := args["datastore"].(string)
	ds, err := resolveDatastore(ctx, client, dsName)
	if err != nil {
		return "", err
	}
	remotePath, _ := args["remote_path"].(string)
	if remotePath == "" {
		return "", fmt.Errorf("remote_path is required")
	}

	maxBytes := int64(datastoreOpenDefaultMaxBytes)
	if v, ok := args["max_bytes"]; ok {
		n, err := toInt64(v)
		if err != nil {
			return "", fmt.Errorf("invalid max_bytes: %w", err)
		}
		if n <= 0 {
			return "", fmt.Errorf("max_bytes must be positive, got %d", n)
		}
		maxBytes = n
	}
	if maxBytes > datastoreOpenHardCapBytes {
		maxBytes = datastoreOpenHardCapBytes
	}

	var offset int64
	if v, ok := args["offset"]; ok {
		n, err := toInt64(v)
		if err != nil {
			return "", fmt.Errorf("invalid offset: %w", err)
		}
		if n < 0 {
			return "", fmt.Errorf("offset must be >= 0, got %d", n)
		}
		offset = n
	}

	f, err := ds.Open(ctx, remotePath)
	if err != nil {
		return "", fmt.Errorf("failed to open %s on datastore %s: %w", remotePath, ds.InventoryPath, err)
	}
	defer f.Close()

	if offset > 0 {
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			return "", fmt.Errorf("failed to seek to offset %d in %s on datastore %s: %w", offset, remotePath, ds.InventoryPath, err)
		}
	}

	// Read maxBytes+1 so a full buffer tells us more data existed beyond the
	// cap (truncated:true) vs. the file legitimately ending exactly at
	// maxBytes (truncated:false). io.ReadFull returns io.EOF/io.ErrUnexpectedEOF
	// (not a real error) whenever the file is shorter than the buffer —
	// both are expected outcomes here, not failures.
	buf := make([]byte, maxBytes+1)
	n, err := io.ReadFull(f, buf)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return "", fmt.Errorf("failed to read %s on datastore %s: %w", remotePath, ds.InventoryPath, err)
	}

	truncated := int64(n) > maxBytes
	if truncated {
		n = int(maxBytes)
	}

	return marshalJSON(map[string]interface{}{
		"datastore":   ds.InventoryPath,
		"remote_path": remotePath,
		"offset":      offset,
		"max_bytes":   maxBytes,
		"bytes_read":  n,
		"truncated":   truncated,
		"encoding":    "base64",
		"data":        base64.StdEncoding.EncodeToString(buf[:n]),
	})
}

func handleDatastoreServiceTicket(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	dsName, _ := args["datastore"].(string)
	ds, err := resolveDatastore(ctx, client, dsName)
	if err != nil {
		return "", err
	}
	remotePath, _ := args["remote_path"].(string)
	if remotePath == "" {
		return "", fmt.Errorf("remote_path is required")
	}
	method, _ := args["method"].(string)
	if method == "" {
		return "", fmt.Errorf("method is required")
	}

	u, cookie, err := ds.ServiceTicket(ctx, remotePath, method)
	if err != nil {
		return "", fmt.Errorf("failed to acquire service ticket for %s on datastore %s: %w", remotePath, ds.InventoryPath, err)
	}

	result := map[string]interface{}{
		"datastore":   ds.InventoryPath,
		"remote_path": remotePath,
		"method":      method,
		"url":         u.String(),
		"cookie":      nil,
	}
	if cookie != nil {
		result["cookie"] = map[string]interface{}{"name": cookie.Name, "value": cookie.Value}
	}

	return marshalJSON(result)
}

func handleDatastoreSearch(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	dsName, _ := args["datastore"].(string)
	ds, err := resolveDatastore(ctx, client, dsName)
	if err != nil {
		return "", err
	}
	pathArg, _ := args["path"].(string)
	spec, err := decodeSearchSpec(args)
	if err != nil {
		return "", err
	}
	browser, err := datastoreBrowser(ctx, ds)
	if err != nil {
		return "", err
	}

	dsPath := ds.Path(pathArg)
	task, err := browser.SearchDatastore(ctx, dsPath, spec)
	if err != nil {
		return "", fmt.Errorf("failed to search %s on datastore %s: %w", dsPath, ds.InventoryPath, err)
	}
	info, err := task.WaitForResult(ctx)
	if err != nil {
		return "", fmt.Errorf("search task failed for %s on datastore %s: %w", dsPath, ds.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{
		"datastore": ds.InventoryPath,
		"path":      dsPath,
		"result":    info.Result,
	})
}

func handleDatastoreSearchSubFolders(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	dsName, _ := args["datastore"].(string)
	ds, err := resolveDatastore(ctx, client, dsName)
	if err != nil {
		return "", err
	}
	pathArg, _ := args["path"].(string)
	spec, err := decodeSearchSpec(args)
	if err != nil {
		return "", err
	}
	browser, err := datastoreBrowser(ctx, ds)
	if err != nil {
		return "", err
	}

	dsPath := ds.Path(pathArg)
	task, err := browser.SearchDatastoreSubFolders(ctx, dsPath, spec)
	if err != nil {
		return "", fmt.Errorf("failed to search subfolders of %s on datastore %s: %w", dsPath, ds.InventoryPath, err)
	}
	info, err := task.WaitForResult(ctx)
	if err != nil {
		return "", fmt.Errorf("search-subfolders task failed for %s on datastore %s: %w", dsPath, ds.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{
		"datastore": ds.InventoryPath,
		"path":      dsPath,
		"result":    info.Result,
	})
}

func handleDatastoreNamespaceConvertPathToUuid(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	dcName, _ := args["datacenter"].(string)
	dc, err := resolveDatacenter(ctx, client, dcName)
	if err != nil {
		return "", err
	}
	dsURL, _ := args["datastore_url"].(string)
	if dsURL == "" {
		return "", fmt.Errorf("datastore_url is required")
	}

	nm := object.NewDatastoreNamespaceManager(client.Client.Client)
	uuidPath, err := nm.ConvertNamespacePathToUuidPath(ctx, dc, dsURL)
	if err != nil {
		return "", fmt.Errorf("failed to convert namespace path %s in datacenter %s: %w", dsURL, dc.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{
		"datacenter":    dc.InventoryPath,
		"datastore_url": dsURL,
		"uuid_path":     uuidPath,
	})
}

func handleDatastoreNamespaceCreateDirectory(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	dsName, _ := args["datastore"].(string)
	ds, err := resolveDatastore(ctx, client, dsName)
	if err != nil {
		return "", err
	}
	displayName, _ := args["display_name"].(string)
	if displayName == "" {
		return "", fmt.Errorf("display_name is required")
	}
	policy, _ := args["policy"].(string)

	nm := object.NewDatastoreNamespaceManager(client.Client.Client)
	dir, err := nm.CreateDirectory(ctx, ds, displayName, policy)
	if err != nil {
		return "", fmt.Errorf("failed to create namespace directory %q on datastore %s: %w", displayName, ds.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{
		"datastore":    ds.InventoryPath,
		"display_name": displayName,
		"directory":    dir,
		"result":       "created",
	})
}

func handleDatastoreNamespaceDeleteDirectory(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	dcName, _ := args["datacenter"].(string)
	dc, err := resolveDatacenter(ctx, client, dcName)
	if err != nil {
		return "", err
	}
	dsPath, _ := args["datastore_path"].(string)
	if dsPath == "" {
		return "", fmt.Errorf("datastore_path is required")
	}

	nm := object.NewDatastoreNamespaceManager(client.Client.Client)
	if err := nm.DeleteDirectory(ctx, dc, dsPath); err != nil {
		return "", fmt.Errorf("failed to delete namespace directory %s in datacenter %s: %w", dsPath, dc.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{
		"datacenter":     dc.InventoryPath,
		"datastore_path": dsPath,
		"result":         "deleted",
	})
}

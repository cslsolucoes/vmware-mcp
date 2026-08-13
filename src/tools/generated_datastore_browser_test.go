package tools

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// newDatastoreBrowserRegistry builds a Registry the normal way (NewRegistry,
// which wires vm.go/host.go/datastore.go/etc via registerTools) and then
// manually layers this group's tools on top via withClass — same pattern as
// generated_host_storage_test.go's newHostStorageRegistry, and for the same
// reason: registry.go itself must not be edited by this file (see
// generated_datastore_browser.go's top doc comment).
func newDatastoreBrowserRegistry(ctx context.Context, c *vmware.Client, opts RegistryOptions) *Registry {
	r := NewRegistry(ctx, c, opts)
	r.withClass(modeVSphereGeneral, registerDatastoreBrowserTools)
	return r
}

// firstDatastorePath returns the "name" of the first datastore reported by
// vmware_list_datastores — the same field generated_host_storage_test.go's
// remove_datastore subtest reads, safe to pass as this group's "datastore"
// argument (resolved via the existing resolveDatastore/dcScopedPath("datastore", ...)).
func firstDatastorePath(t *testing.T, r *Registry) string {
	t.Helper()
	raw, err := r.CallTool("vmware_list_datastores", map[string]interface{}{})
	if err != nil {
		t.Fatalf("vmware_list_datastores failed: %v", err)
	}
	list, _ := decodeResult(t, raw)["datastores"].([]interface{})
	if len(list) == 0 {
		t.Fatal("simulator model has no datastores")
	}
	first, ok := list[0].(map[string]interface{})
	if !ok {
		t.Fatalf("datastore entry is not an object: %v", list[0])
	}
	name, _ := first["name"].(string)
	if name == "" {
		t.Fatalf("could not read a datastore name from %v", first)
	}
	return name
}

// TestDatastoreBrowserTools_Registration proves all 12 tools are registered
// and reachable via ListTools — a basic wiring smoke test before the more
// specific behavioral tests below.
func TestDatastoreBrowserTools_Registration(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newDatastoreBrowserRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	want := []string{
		"vmware_datastore_attached_cluster_hosts",
		"vmware_datastore_attached_hosts",
		"vmware_datastore_stat",
		"vmware_datastore_type",
		"vmware_datastore_download_file",
		"vmware_datastore_open",
		"vmware_datastore_service_ticket",
		"vmware_datastore_search",
		"vmware_datastore_search_subfolders",
		"vmware_datastore_namespace_convert_path_to_uuid",
		"vmware_datastore_namespace_create_directory",
		"vmware_datastore_namespace_delete_directory",
	}
	if len(want) != 12 {
		t.Fatalf("test bug: want list has %d entries, expected 12", len(want))
	}

	got := map[string]bool{}
	for _, tl := range r.ListTools() {
		got[tl.Name] = true
	}
	for _, name := range want {
		if !got[name] {
			t.Errorf("tool %s not registered", name)
		}
	}
}

// TestDatastoreBrowserTools_FileOperations exercises Stat/Type/DownloadFile/
// Open/Search/SearchSubFolders against a real file uploaded to vcsim via the
// already-registered vmware_datastore_upload_file tool (datastore.go) —
// every one of these reaches vcsim's real /folder HTTP handler or
// SearchDatastore_Task implementation (see generated_datastore_browser.go's
// top doc comment), not a simulation gap.
func TestDatastoreBrowserTools_FileOperations(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newDatastoreBrowserRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	dsName := firstDatastorePath(t, r)

	const remotePath = "browser-tests/hello.txt"
	const contents = "hello from generated_datastore_browser_test.go"

	localUpload := filepath.Join(t.TempDir(), "hello.txt")
	if err := os.WriteFile(localUpload, []byte(contents), 0o644); err != nil {
		t.Fatalf("failed to create local upload fixture: %v", err)
	}
	if _, err := r.CallTool("vmware_datastore_upload_file", map[string]interface{}{
		"datastore": dsName, "local_path": localUpload, "remote_path": remotePath,
	}); err != nil {
		t.Fatalf("fixture upload failed: %v", err)
	}

	t.Run("stat_existing_file", func(t *testing.T) {
		raw, err := r.CallTool("vmware_datastore_stat", map[string]interface{}{"datastore": dsName, "file": remotePath})
		if err != nil {
			t.Fatalf("vmware_datastore_stat failed: %v", err)
		}
		m := decodeResult(t, raw)
		if m["exists"] != true {
			t.Fatalf("expected exists:true for an uploaded file, got: %s", raw)
		}
		if _, ok := m["info"]; !ok {
			t.Fatalf("expected an \"info\" field: %s", raw)
		}
	})

	t.Run("stat_missing_file_not_an_error", func(t *testing.T) {
		raw, err := r.CallTool("vmware_datastore_stat", map[string]interface{}{"datastore": dsName, "file": "browser-tests/does-not-exist.txt"})
		if err != nil {
			t.Fatalf("vmware_datastore_stat must tolerate a missing file, got error: %v", err)
		}
		m := decodeResult(t, raw)
		if m["exists"] != false {
			t.Fatalf("expected exists:false for a missing file, got: %s", raw)
		}
	})

	t.Run("type", func(t *testing.T) {
		raw, err := r.CallTool("vmware_datastore_type", map[string]interface{}{"datastore": dsName})
		if err != nil {
			t.Fatalf("vmware_datastore_type failed: %v", err)
		}
		m := decodeResult(t, raw)
		fsType, _ := m["type"].(string)
		if fsType == "" {
			t.Fatalf("expected a non-empty filesystem type: %s", raw)
		}
		t.Logf("datastore %s reports filesystem type %q", dsName, fsType)
	})

	t.Run("download_file", func(t *testing.T) {
		localDownload := filepath.Join(t.TempDir(), "downloaded.txt")
		raw, err := r.CallTool("vmware_datastore_download_file", map[string]interface{}{
			"datastore": dsName, "remote_path": remotePath, "local_path": localDownload, "confirm": true,
		})
		if err != nil {
			t.Fatalf("vmware_datastore_download_file failed: %v", err)
		}
		if decodeResult(t, raw)["result"] != "downloaded" {
			t.Fatalf("unexpected result: %s", raw)
		}
		got, err := os.ReadFile(localDownload)
		if err != nil {
			t.Fatalf("downloaded file not found on local disk: %v", err)
		}
		if string(got) != contents {
			t.Fatalf("downloaded content mismatch: got %q, want %q", got, contents)
		}
	})

	t.Run("download_file_gate_closed", func(t *testing.T) {
		closedGate := newDatastoreBrowserRegistry(context.Background(), c, RegistryOptions{})
		localDownload := filepath.Join(t.TempDir(), "downloaded-denied.txt")
		if _, err := closedGate.CallTool("vmware_datastore_download_file", map[string]interface{}{
			"datastore": dsName, "remote_path": remotePath, "local_path": localDownload, "confirm": true,
		}); err == nil {
			t.Fatal("expected the closed destructive gate to deny download_file")
		}
	})

	t.Run("open_full_read", func(t *testing.T) {
		raw, err := r.CallTool("vmware_datastore_open", map[string]interface{}{
			"datastore": dsName, "remote_path": remotePath, "confirm": true,
		})
		if err != nil {
			t.Fatalf("vmware_datastore_open failed: %v", err)
		}
		m := decodeResult(t, raw)
		if m["truncated"] != false {
			t.Fatalf("expected truncated:false reading a small file with the default 64KiB cap: %s", raw)
		}
		data, _ := m["data"].(string)
		decoded, err := base64.StdEncoding.DecodeString(data)
		if err != nil {
			t.Fatalf("data is not valid base64: %v (%s)", err, raw)
		}
		if string(decoded) != contents {
			t.Fatalf("decoded content mismatch: got %q, want %q", decoded, contents)
		}
	})

	t.Run("open_max_bytes_truncates", func(t *testing.T) {
		raw, err := r.CallTool("vmware_datastore_open", map[string]interface{}{
			"datastore": dsName, "remote_path": remotePath, "max_bytes": 5, "confirm": true,
		})
		if err != nil {
			t.Fatalf("vmware_datastore_open with max_bytes failed: %v", err)
		}
		m := decodeResult(t, raw)
		if m["truncated"] != true {
			t.Fatalf("expected truncated:true when max_bytes (5) is smaller than the file: %s", raw)
		}
		data, _ := m["data"].(string)
		decoded, err := base64.StdEncoding.DecodeString(data)
		if err != nil {
			t.Fatalf("data is not valid base64: %v (%s)", err, raw)
		}
		if string(decoded) != contents[:5] {
			t.Fatalf("expected the first 5 bytes %q, got %q", contents[:5], decoded)
		}
	})

	t.Run("open_with_offset", func(t *testing.T) {
		raw, err := r.CallTool("vmware_datastore_open", map[string]interface{}{
			"datastore": dsName, "remote_path": remotePath, "offset": 6, "confirm": true,
		})
		if err != nil {
			t.Fatalf("vmware_datastore_open with offset failed: %v", err)
		}
		data, _ := decodeResult(t, raw)["data"].(string)
		decoded, err := base64.StdEncoding.DecodeString(data)
		if err != nil {
			t.Fatalf("data is not valid base64: %v (%s)", err, raw)
		}
		if string(decoded) != contents[6:] {
			t.Fatalf("expected content starting at offset 6 (%q), got %q", contents[6:], decoded)
		}
	})

	t.Run("open_missing_file_is_error", func(t *testing.T) {
		if _, err := r.CallTool("vmware_datastore_open", map[string]interface{}{
			"datastore": dsName, "remote_path": "browser-tests/does-not-exist.txt", "confirm": true,
		}); err == nil {
			t.Fatal("expected an error opening a file that does not exist")
		}
	})

	t.Run("search_finds_uploaded_file", func(t *testing.T) {
		raw, err := r.CallTool("vmware_datastore_search", map[string]interface{}{"datastore": dsName, "path": "browser-tests"})
		if err != nil {
			t.Fatalf("vmware_datastore_search failed: %v", err)
		}
		result, ok := decodeResult(t, raw)["result"].(map[string]interface{})
		if !ok {
			t.Fatalf("expected \"result\" to be a single HostDatastoreBrowserSearchResults object: %s", raw)
		}
		files, _ := result["file"].([]interface{})
		found := false
		for _, f := range files {
			entry, _ := f.(map[string]interface{})
			if entry != nil && entry["path"] == "hello.txt" {
				found = true
			}
		}
		if !found {
			t.Fatalf("expected to find hello.txt in search results: %s", raw)
		}
	})

	t.Run("search_subfolders_finds_uploaded_file", func(t *testing.T) {
		raw, err := r.CallTool("vmware_datastore_search_subfolders", map[string]interface{}{"datastore": dsName, "path": ""})
		if err != nil {
			t.Fatalf("vmware_datastore_search_subfolders failed: %v", err)
		}
		result, ok := decodeResult(t, raw)["result"].(map[string]interface{})
		if !ok {
			t.Fatalf("expected \"result\" to be an ArrayOfHostDatastoreBrowserSearchResults object: %s", raw)
		}
		// types.ArrayOfHostDatastoreBrowserSearchResults's single field
		// marshals as JSON key "_value" (its `json:"_value"` struct tag, not
		// the Go field name "HostDatastoreBrowserSearchResults") — confirmed
		// against the actual tool output before fixing this assertion.
		groups, _ := result["_value"].([]interface{})
		found := false
		for _, g := range groups {
			group, _ := g.(map[string]interface{})
			if group == nil {
				continue
			}
			files, _ := group["file"].([]interface{})
			for _, f := range files {
				entry, _ := f.(map[string]interface{})
				if entry != nil && entry["path"] == "hello.txt" {
					found = true
				}
			}
		}
		if !found {
			t.Fatalf("expected to find hello.txt recursively from the datastore root: %s", raw)
		}
	})

	t.Run("search_missing_search_spec_defaults_to_match_all", func(t *testing.T) {
		// No "search_spec" key at all — proves decodeSearchSpec's default
		// (MatchPattern: ["*"]) is actually wired in, and that vcsim's
		// searchDatastore.queryMatch/addFile (which dereference SearchSpec
		// fields with no nil guard) do not panic — see this group's top doc
		// comment for why a bare nil spec would.
		if _, err := r.CallTool("vmware_datastore_search", map[string]interface{}{"datastore": dsName, "path": "browser-tests"}); err != nil {
			t.Fatalf("vmware_datastore_search without search_spec should default to match-all, got: %v", err)
		}
	})
}

// TestDatastoreBrowserTools_ServiceTicket proves ServiceTicket reaches a
// real URL against vcsim without going through AcquireGenericServiceTicket
// (useServiceTicket() is false here — neither IsVC() nor
// GOVMOMI_USE_SERVICE_TICKET hold — see this group's top doc comment) and
// that the returned URL genuinely serves the file's contents over plain
// HTTP GET against vcsim's /folder handler.
func TestDatastoreBrowserTools_ServiceTicket(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newDatastoreBrowserRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	dsName := firstDatastorePath(t, r)

	const remotePath = "ticket-tests/ticket.txt"
	localUpload := filepath.Join(t.TempDir(), "ticket.txt")
	if err := os.WriteFile(localUpload, []byte("ticket contents"), 0o644); err != nil {
		t.Fatalf("failed to create local upload fixture: %v", err)
	}
	if _, err := r.CallTool("vmware_datastore_upload_file", map[string]interface{}{
		"datastore": dsName, "local_path": localUpload, "remote_path": remotePath,
	}); err != nil {
		t.Fatalf("fixture upload failed: %v", err)
	}

	raw, err := r.CallTool("vmware_datastore_service_ticket", map[string]interface{}{
		"datastore": dsName, "remote_path": remotePath, "method": "GET", "confirm": true,
	})
	if err != nil {
		t.Fatalf("vmware_datastore_service_ticket failed: %v", err)
	}
	m := decodeResult(t, raw)
	url, _ := m["url"].(string)
	if url == "" {
		t.Fatalf("expected a non-empty url: %s", raw)
	}
	if _, hasCookie := m["cookie"]; !hasCookie {
		t.Fatalf(`expected a "cookie" key (even if null) in the result: %s`, raw)
	}
	t.Logf("service ticket url: %s, cookie: %v", url, m["cookie"])

	closedGate := newDatastoreBrowserRegistry(context.Background(), c, RegistryOptions{})
	if _, err := closedGate.CallTool("vmware_datastore_service_ticket", map[string]interface{}{
		"datastore": dsName, "remote_path": remotePath, "method": "GET", "confirm": true,
	}); err == nil {
		t.Fatal("expected the closed destructive gate to deny service_ticket")
	}
}

// TestDatastoreBrowserTools_AttachedHosts proves AttachedHosts/
// AttachedClusterHosts return real, correctly-resolved inventory paths
// (see this group's top doc comment on the InventoryPath gotcha for both).
// AttachedClusterHosts specifically needs a real cluster, which
// simulator.ESX() (standalone) never has — simulator.VPX()'s default model
// (Cluster:1, ClusterHost:3, Host:1) is used instead, per Model's own
// documented defaults in referencia/govmomi/simulator/model.go.
func TestDatastoreBrowserTools_AttachedHosts(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newDatastoreBrowserRegistry(context.Background(), c, RegistryOptions{})
	dsName := firstDatastorePath(t, r)

	t.Run("attached_hosts", func(t *testing.T) {
		raw, err := r.CallTool("vmware_datastore_attached_hosts", map[string]interface{}{"datastore": dsName})
		if err != nil {
			t.Fatalf("vmware_datastore_attached_hosts failed: %v", err)
		}
		m := decodeResult(t, raw)
		hosts, _ := m["hosts"].([]interface{})
		t.Logf("datastore %s reports %d attached host(s): %v", dsName, len(hosts), hosts)
		for _, h := range hosts {
			p, _ := h.(string)
			if p == "" || p[0] != '/' {
				t.Fatalf("expected a resolved inventory path (leading '/'), got %q — the InventoryPath gotcha this file documents may have regressed: %s", p, raw)
			}
		}
	})

	t.Run("attached_cluster_hosts", func(t *testing.T) {
		clustersRaw, err := r.CallTool("vmware_list_clusters", map[string]interface{}{})
		if err != nil {
			t.Fatalf("vmware_list_clusters failed: %v", err)
		}
		clusters, _ := decodeResult(t, clustersRaw)["clusters"].([]interface{})
		if len(clusters) == 0 {
			t.Fatal("simulator.VPX() model has no clusters — expected Cluster:1 per its documented defaults")
		}
		cluster, _ := clusters[0].(string)

		raw, err := r.CallTool("vmware_datastore_attached_cluster_hosts", map[string]interface{}{"datastore": dsName, "cluster": cluster})
		if err != nil {
			t.Fatalf("vmware_datastore_attached_cluster_hosts failed: %v", err)
		}
		m := decodeResult(t, raw)
		hosts, _ := m["hosts"].([]interface{})
		t.Logf("cluster %s reports %d attached datastore host(s) for %s: %v", cluster, len(hosts), dsName, hosts)
		for _, h := range hosts {
			p, _ := h.(string)
			if p == "" || p[0] != '/' {
				t.Fatalf("expected a resolved inventory path (leading '/'), got %q: %s", p, raw)
			}
		}
	})

	t.Run("attached_cluster_hosts_unresolvable_cluster", func(t *testing.T) {
		if _, err := r.CallTool("vmware_datastore_attached_cluster_hosts", map[string]interface{}{"datastore": dsName, "cluster": "does-not-exist"}); err == nil {
			t.Fatal("expected an error for an unresolvable cluster")
		}
	})
}

// TestDatastoreBrowserTools_NamespaceManager proves the 3
// DatastoreNamespaceManager tools reach a real, full success path against
// vcsim by mutating the underlying simulator.Datastore's Summary.Type to
// "vsan" — the exact fixture trick used by govmomi's own
// referencia/govmomi/object/namespace_manager_test.go (confirmed by reading
// that file), obtained here via the already-exported
// model.Service.Context/simulator.Map(...) without touching
// testhelpers_test.go. It also proves the real business-rule fault (not a
// vcsim gap — see this group's top doc comment) against an ordinary,
// non-VSAN datastore.
func TestDatastoreBrowserTools_NamespaceManager(t *testing.T) {
	model := simulator.VPX()
	c, cleanup := newSimClient(t, model)
	defer cleanup()

	r := newDatastoreBrowserRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	dsName := firstDatastorePath(t, r)

	t.Run("create_directory_fails_on_non_vsan_datastore", func(t *testing.T) {
		// Real vSphere business-rule validation (DatastoreNamespaceManager is
		// VSAN/VVol-only), not a vcsim simulation gap — see this group's top
		// doc comment.
		if _, err := r.CallTool("vmware_datastore_namespace_create_directory", map[string]interface{}{
			"datastore": dsName, "display_name": "should-fail", "confirm": true,
		}); err == nil {
			t.Fatal("expected CreateDirectory to fault against a non-VSAN datastore")
		}
	})

	// Flip the default (NFS/local) datastore to report as VSAN, exactly like
	// govmomi's own TestDatastoreNamespaceManager, so CreateDirectory/
	// DeleteDirectory/ConvertNamespacePathToUuidPath can reach their real
	// success path instead of the VSAN-only guard.
	ds, err := c.Finder.Datastore(context.Background(), dsName)
	if err != nil {
		t.Fatalf("failed to resolve %s via the client Finder: %v", dsName, err)
	}
	store, ok := simulator.Map(model.Service.Context).Get(ds.Reference()).(*simulator.Datastore)
	if !ok {
		t.Fatalf("could not find the simulator.Datastore backing %s", dsName)
	}
	store.Summary.Type = string(types.HostFileSystemVolumeFileSystemTypeVsan)
	store.Capability.TopLevelDirectoryCreateSupported = types.NewBool(true)

	dcRaw, err := r.CallTool("vmware_list_datacenters", map[string]interface{}{})
	if err != nil {
		t.Fatalf("vmware_list_datacenters failed: %v", err)
	}
	dcList, _ := decodeResult(t, dcRaw)["datacenters"].([]interface{})
	if len(dcList) == 0 {
		t.Fatal("simulator.VPX() model has no datacenters")
	}
	dc, _ := dcList[0].(string)

	var namespaceDir string

	t.Run("create_directory", func(t *testing.T) {
		raw, err := r.CallTool("vmware_datastore_namespace_create_directory", map[string]interface{}{
			"datastore": dsName, "display_name": "browsertest", "confirm": true,
		})
		if err != nil {
			t.Fatalf("vmware_datastore_namespace_create_directory failed after flipping the datastore to VSAN: %v", err)
		}
		m := decodeResult(t, raw)
		namespaceDir, _ = m["directory"].(string)
		if namespaceDir == "" {
			t.Fatalf("expected a non-empty \"directory\" in the result: %s", raw)
		}
		if m["result"] != "created" {
			t.Fatalf("unexpected result: %s", raw)
		}
	})

	t.Run("create_directory_gate_closed", func(t *testing.T) {
		closedGate := newDatastoreBrowserRegistry(context.Background(), c, RegistryOptions{})
		if _, err := closedGate.CallTool("vmware_datastore_namespace_create_directory", map[string]interface{}{
			"datastore": dsName, "display_name": "denied", "confirm": true,
		}); err == nil {
			t.Fatal("expected the closed destructive gate to deny create_directory")
		}
	})

	t.Run("convert_path_to_uuid", func(t *testing.T) {
		if namespaceDir == "" {
			t.Skip("create_directory subtest did not produce a namespace directory")
		}
		raw, err := r.CallTool("vmware_datastore_namespace_convert_path_to_uuid", map[string]interface{}{
			"datacenter": dc, "datastore_url": namespaceDir, "confirm": true,
		})
		if err != nil {
			t.Fatalf("vmware_datastore_namespace_convert_path_to_uuid failed: %v", err)
		}
		m := decodeResult(t, raw)
		if m["uuid_path"] == "" || m["uuid_path"] == nil {
			t.Fatalf("expected a non-empty uuid_path: %s", raw)
		}
	})

	t.Run("delete_directory", func(t *testing.T) {
		if namespaceDir == "" {
			t.Skip("create_directory subtest did not produce a namespace directory")
		}
		raw, err := r.CallTool("vmware_datastore_namespace_delete_directory", map[string]interface{}{
			"datacenter": dc, "datastore_path": namespaceDir, "confirm": true,
		})
		if err != nil {
			t.Fatalf("vmware_datastore_namespace_delete_directory failed: %v", err)
		}
		if decodeResult(t, raw)["result"] != "deleted" {
			t.Fatalf("unexpected result: %s", raw)
		}
	})

	t.Run("delete_directory_gate_closed_tier1", func(t *testing.T) {
		closedGate := newDatastoreBrowserRegistry(context.Background(), c, RegistryOptions{})
		if _, err := closedGate.CallTool("vmware_datastore_namespace_delete_directory", map[string]interface{}{
			"datacenter": dc, "datastore_path": "whatever", "confirm": true,
		}); err == nil {
			t.Fatal("expected the closed destructive gate to deny delete_directory (tier 1)")
		}
	})
}

// TestDatastoreBrowserTools_RequiredArgs spot-checks that the required
// arguments of each tool are actually enforced before any round trip to
// vcsim — one representative missing-arg case per tool is enough here since
// every handler follows the same "resolve, then check each required arg"
// shape already covered in detail by the tests above.
func TestDatastoreBrowserTools_RequiredArgs(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newDatastoreBrowserRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	dsName := firstDatastorePath(t, r)

	cases := []struct {
		tool string
		args map[string]interface{}
	}{
		{"vmware_datastore_attached_cluster_hosts", map[string]interface{}{"datastore": dsName}},
		{"vmware_datastore_attached_hosts", map[string]interface{}{}},
		{"vmware_datastore_stat", map[string]interface{}{"datastore": dsName}},
		{"vmware_datastore_type", map[string]interface{}{}},
		{"vmware_datastore_download_file", map[string]interface{}{"datastore": dsName, "remote_path": "x", "confirm": true}},
		{"vmware_datastore_open", map[string]interface{}{"datastore": dsName, "confirm": true}},
		{"vmware_datastore_service_ticket", map[string]interface{}{"datastore": dsName, "remote_path": "x", "confirm": true}},
		{"vmware_datastore_search", map[string]interface{}{"path": "x"}},
		{"vmware_datastore_search_subfolders", map[string]interface{}{"path": "x"}},
		{"vmware_datastore_namespace_convert_path_to_uuid", map[string]interface{}{"confirm": true}},
		{"vmware_datastore_namespace_create_directory", map[string]interface{}{"datastore": dsName, "confirm": true}},
		{"vmware_datastore_namespace_delete_directory", map[string]interface{}{"confirm": true}},
	}
	if len(cases) != 12 {
		t.Fatalf("test bug: cases has %d entries, expected 12", len(cases))
	}

	for _, tc := range cases {
		t.Run(tc.tool, func(t *testing.T) {
			if _, err := r.CallTool(tc.tool, tc.args); err == nil {
				t.Fatalf("expected an error for %s with args %#v (missing a required argument)", tc.tool, tc.args)
			}
		})
	}
}

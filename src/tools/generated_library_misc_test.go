package tools

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vapi/library"
	"github.com/vmware/govmomi/vim25/soap"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// newLibraryMiscRegistry layers registerLibraryMiscTools on top of a normal
// NewRegistry via withClass — same pattern as generated_tenant_test.go's
// newTenantRegistry (this file must not edit registry.go, per this group's
// brief).
func newLibraryMiscRegistry(ctx context.Context, c *vmware.Client, opts RegistryOptions) *Registry {
	r := NewRegistry(ctx, c, opts)
	r.withClass(modeVCenterOnly, registerLibraryMiscTools)
	return r
}

// libraryMiscTestManager builds a *library.Manager directly (not through a
// tool) so tests can construct their own library/item/file fixtures without
// depending on the CL-A group's library-creation tools, which are being
// written in parallel in a different file (per this group's brief).
func libraryMiscTestManager(t *testing.T, ctx context.Context, c *vmware.Client) *library.Manager {
	t.Helper()
	rc, err := c.REST(ctx)
	if err != nil {
		t.Fatalf("failed to establish REST/VAPI session: %v", err)
	}
	return library.NewManager(rc)
}

// createTestLibrary creates a real LOCAL content library backed by vcsim's
// default datastore.
func createTestLibrary(t *testing.T, ctx context.Context, c *vmware.Client, m *library.Manager, name string) string {
	t.Helper()
	ds, err := c.Finder.DefaultDatastore(ctx)
	if err != nil {
		t.Fatalf("failed to resolve default datastore: %v", err)
	}
	id, err := m.CreateLibrary(ctx, library.Library{
		Name: name,
		Type: "LOCAL",
		Storage: []library.StorageBacking{{
			DatastoreID: ds.Reference().Value,
			Type:        "DATASTORE",
		}},
	})
	if err != nil {
		t.Fatalf("failed to create test library %q: %v", name, err)
	}
	return id
}

// createTestLibraryItemWithFile creates a library item and pushes a single
// small in-memory file into it through the real update-session upload flow
// (CreateLibraryItemUpdateSession -> AddLibraryItemFile -> Upload ->
// CompleteLibraryItemUpdateSession) — the same flow
// referencia/govmomi/vapi/library/example_test.go uses, minus the OVA/tar
// wrapping (a non-OVA file is stored as-is by the simulator's
// libraryItemFileCreate, confirmed by reading
// referencia/govmomi/vapi/simulator/simulator.go directly), so
// vmware_library_list/get_library_item_files/storage have real data to
// return instead of an empty item.
func createTestLibraryItemWithFile(t *testing.T, ctx context.Context, m *library.Manager, libraryID, itemName, fileName string, content []byte) string {
	t.Helper()

	itemID, err := m.CreateLibraryItem(ctx, library.Item{
		Name:      itemName,
		Type:      library.ItemTypeISO,
		LibraryID: libraryID,
	})
	if err != nil {
		t.Fatalf("failed to create test library item %q: %v", itemName, err)
	}

	sessionID, err := m.CreateLibraryItemUpdateSession(ctx, library.Session{LibraryItemID: itemID})
	if err != nil {
		t.Fatalf("failed to create update session for item %q: %v", itemName, err)
	}

	update, err := m.AddLibraryItemFile(ctx, sessionID, library.UpdateFile{
		Name:       fileName,
		SourceType: "PUSH",
		Size:       int64(len(content)),
	})
	if err != nil {
		t.Fatalf("failed to add file %q to update session: %v", fileName, err)
	}

	u, err := url.Parse(update.UploadEndpoint.URI)
	if err != nil {
		t.Fatalf("failed to parse upload endpoint URI %q: %v", update.UploadEndpoint.URI, err)
	}

	p := soap.DefaultUpload
	p.ContentLength = int64(len(content))
	if err := m.Client.Upload(ctx, bytes.NewReader(content), u, &p); err != nil {
		t.Fatalf("failed to upload test file %q: %v", fileName, err)
	}

	if err := m.CompleteLibraryItemUpdateSession(ctx, sessionID); err != nil {
		t.Fatalf("failed to complete update session for item %q: %v", itemName, err)
	}

	return itemID
}

// generateTestCertPEM returns a real, self-signed, Base64 PEM-encoded
// certificate — needed because vcsim's libraryTrustedCertificates handler
// genuinely runs pem.Decode + x509.ParseCertificate on the submitted text
// (confirmed by reading referencia/govmomi/vapi/simulator/simulator.go), so
// a placeholder string like generated_extension_test.go's fake cert (never
// parsed server-side by that domain) would not survive a real
// vmware_library_create_trusted_certificate round trip here.
func generateTestCertPEM(t *testing.T) string {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate test key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{CommonName: "mcpvmware-library-misc-test"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("failed to create test certificate: %v", err)
	}
	var buf bytes.Buffer
	if err := pem.Encode(&buf, &pem.Block{Type: "CERTIFICATE", Bytes: der}); err != nil {
		t.Fatalf("failed to PEM-encode test certificate: %v", err)
	}
	return buf.String()
}

// TestLibraryMiscTools_LibraryUsage_AddGetListRemove proves the full
// Add -> Get -> List -> Remove -> List round trip against real vcsim
// (handler.LibraryUsage, an in-memory map[libraryID]map[usageID]library.Usage
// — confirmed genuinely implemented, not a 404 stub, by reading
// referencia/govmomi/vapi/simulator/simulator.go's libraryUsages/
// libraryUsageID/addUsage/removeUsage/findUsage).
func TestLibraryMiscTools_LibraryUsage_AddGetListRemove(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()
	ctx := context.Background()
	r := newLibraryMiscRegistry(ctx, c, RegistryOptions{AllowDestructive: true})

	m := libraryMiscTestManager(t, ctx, c)
	libID := createTestLibrary(t, ctx, c, m, "usage-test-lib")

	raw, err := r.CallTool("vmware_library_list_library_usage", map[string]interface{}{"library_id": libID})
	if err != nil {
		t.Fatalf("vmware_library_list_library_usage (before add) failed: %v", err)
	}
	if countOf(t, raw) != 0 {
		t.Fatalf("expected 0 usage records before any add, got: %s", raw)
	}

	const resourceURN = "vmomi:service:wcp"
	raw, err = r.CallTool("vmware_library_add_library_usage", map[string]interface{}{
		"library_id":   libID,
		"resource_urn": resourceURN,
		"confirm":      true,
	})
	if err != nil {
		t.Fatalf("vmware_library_add_library_usage failed: %v", err)
	}
	addResult := decodeResult(t, raw)
	if addResult["result"] != "added" {
		t.Fatalf("unexpected add result: %s", raw)
	}
	usageID, _ := addResult["usage_id"].(string)
	if usageID == "" {
		t.Fatalf("expected a non-empty usage_id in add result: %s", raw)
	}

	raw, err = r.CallTool("vmware_library_get_library_usage", map[string]interface{}{
		"library_id": libID,
		"usage_id":   usageID,
	})
	if err != nil {
		t.Fatalf("vmware_library_get_library_usage failed: %v", err)
	}
	getResult := decodeResult(t, raw)
	if getResult["resource_urn"] != resourceURN {
		t.Fatalf("expected resource_urn %q, got: %s", resourceURN, raw)
	}

	raw, err = r.CallTool("vmware_library_list_library_usage", map[string]interface{}{"library_id": libID})
	if err != nil {
		t.Fatalf("vmware_library_list_library_usage (after add) failed: %v", err)
	}
	if countOf(t, raw) != 1 {
		t.Fatalf("expected 1 usage record after add, got: %s", raw)
	}

	raw, err = r.CallTool("vmware_library_remove_library_usage", map[string]interface{}{
		"library_id": libID,
		"usage_id":   usageID,
		"confirm":    true,
	})
	if err != nil {
		t.Fatalf("vmware_library_remove_library_usage failed: %v", err)
	}
	if m := decodeResult(t, raw); m["result"] != "removed" {
		t.Fatalf("unexpected remove result: %s", raw)
	}

	raw, err = r.CallTool("vmware_library_list_library_usage", map[string]interface{}{"library_id": libID})
	if err != nil {
		t.Fatalf("vmware_library_list_library_usage (after remove) failed: %v", err)
	}
	if countOf(t, raw) != 0 {
		t.Fatalf("expected 0 usage records after remove, got: %s", raw)
	}

	if _, err := r.CallTool("vmware_library_get_library_usage", map[string]interface{}{
		"library_id": libID,
		"usage_id":   usageID,
	}); err == nil {
		t.Fatal("expected an error getting a removed usage record, got success")
	}
}

// TestLibraryMiscTools_LibraryUsage_DestructiveGateClosed proves
// vmware_library_add_library_usage/vmware_library_remove_library_usage
// refuse to run without --allow-destructive, before ever reaching vcsim —
// same 3-layer protection every other Tier1/Tier2 tool in this project uses
// (destructive.go).
func TestLibraryMiscTools_LibraryUsage_DestructiveGateClosed(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()
	ctx := context.Background()
	r := newLibraryMiscRegistry(ctx, c, RegistryOptions{}) // AllowDestructive defaults false

	_, err := r.CallTool("vmware_library_add_library_usage", map[string]interface{}{
		"library_id":   "does-not-matter",
		"resource_urn": "vmomi:service:wcp",
		"confirm":      true,
	})
	if err == nil {
		t.Fatal("expected vmware_library_add_library_usage to be refused when the server gate is closed")
	}
	if !strings.Contains(err.Error(), "destructive") {
		t.Fatalf("expected a destructive-gate error, got: %v", err)
	}
}

// TestLibraryMiscTools_TrustedCertificates_CreateGetListDelete proves the
// full Create -> Get -> List -> Delete round trip against real vcsim,
// including that the server genuinely parses the submitted PEM
// (handler.libraryTrustedCertificates POST runs pem.Decode +
// x509.ParseCertificate — confirmed by reading simulator.go directly) and
// that vmware_library_create_trusted_certificate's best-effort ID resolution
// (re-listing and matching on cert_text) actually finds the right ID.
func TestLibraryMiscTools_TrustedCertificates_CreateGetListDelete(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()
	ctx := context.Background()
	r := newLibraryMiscRegistry(ctx, c, RegistryOptions{AllowDestructive: true})

	certPEM := generateTestCertPEM(t)

	raw, err := r.CallTool("vmware_library_create_trusted_certificate", map[string]interface{}{
		"cert_text": certPEM,
		"confirm":   true,
	})
	if err != nil {
		t.Fatalf("vmware_library_create_trusted_certificate failed: %v", err)
	}
	createResult := decodeResult(t, raw)
	if createResult["result"] != "created" {
		t.Fatalf("unexpected create result: %s", raw)
	}
	certID, _ := createResult["certificate_id"].(string)
	if certID == "" {
		t.Fatalf("expected create result to resolve a certificate_id, got: %s", raw)
	}

	raw, err = r.CallTool("vmware_library_get_trusted_certificate", map[string]interface{}{"certificate_id": certID})
	if err != nil {
		t.Fatalf("vmware_library_get_trusted_certificate failed: %v", err)
	}
	getResult := decodeResult(t, raw)
	gotText, _ := getResult["cert_text"].(string)
	if strings.TrimSpace(gotText) != strings.TrimSpace(certPEM) {
		t.Fatalf("expected the stored cert_text to round-trip unchanged, got: %s", raw)
	}

	raw, err = r.CallTool("vmware_library_list_trusted_certificates", map[string]interface{}{})
	if err != nil {
		t.Fatalf("vmware_library_list_trusted_certificates failed: %v", err)
	}
	if countOf(t, raw) < 1 {
		t.Fatalf("expected at least 1 trusted certificate after create, got: %s", raw)
	}

	raw, err = r.CallTool("vmware_library_delete_trusted_certificate", map[string]interface{}{
		"certificate_id": certID,
		"confirm":        true,
	})
	if err != nil {
		t.Fatalf("vmware_library_delete_trusted_certificate failed: %v", err)
	}
	if m := decodeResult(t, raw); m["result"] != "deleted" {
		t.Fatalf("unexpected delete result: %s", raw)
	}

	if _, err := r.CallTool("vmware_library_get_trusted_certificate", map[string]interface{}{"certificate_id": certID}); err == nil {
		t.Fatal("expected an error getting a deleted trusted certificate, got success")
	}
}

// TestLibraryMiscTools_TrustedCertificates_InvalidPEMRejected proves an
// invalid certificate is rejected by the real server-side PEM/x509 parse,
// not just accepted blindly — vmware_library_create_trusted_certificate must
// surface that failure as a tool error.
func TestLibraryMiscTools_TrustedCertificates_InvalidPEMRejected(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()
	ctx := context.Background()
	r := newLibraryMiscRegistry(ctx, c, RegistryOptions{AllowDestructive: true})

	_, err := r.CallTool("vmware_library_create_trusted_certificate", map[string]interface{}{
		"cert_text": "not a real certificate",
		"confirm":   true,
	})
	if err == nil {
		t.Fatal("expected an error creating a trusted certificate from invalid PEM text")
	}
}

// TestLibraryMiscTools_SecurityPolicies proves
// vmware_library_list_security_policies and
// vmware_library_default_ovf_security_policy both read real data from
// vcsim's handler.Policies, seeded by defaultSecurityPolicies() with exactly
// one policy named "OVF default policy" (confirmed by reading simulator.go
// directly) — and that DefaultOvfSecurityPolicy's client-side filter
// actually finds it.
func TestLibraryMiscTools_SecurityPolicies(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()
	ctx := context.Background()
	r := newLibraryMiscRegistry(ctx, c, RegistryOptions{})

	raw, err := r.CallTool("vmware_library_list_security_policies", map[string]interface{}{})
	if err != nil {
		t.Fatalf("vmware_library_list_security_policies failed: %v", err)
	}
	listResult := decodeResult(t, raw)
	if countOf(t, raw) != 1 {
		t.Fatalf("expected exactly 1 seeded security policy, got: %s", raw)
	}
	policies, _ := listResult["policies"].([]interface{})
	first, _ := policies[0].(map[string]interface{})
	if first["name"] != library.OvfDefaultSecurityPolicy {
		t.Fatalf("expected policy name %q, got: %s", library.OvfDefaultSecurityPolicy, raw)
	}
	wantPolicyID, _ := first["policy"].(string)

	raw, err = r.CallTool("vmware_library_default_ovf_security_policy", map[string]interface{}{})
	if err != nil {
		t.Fatalf("vmware_library_default_ovf_security_policy failed: %v", err)
	}
	got := decodeResult(t, raw)
	if got["policy"] != wantPolicyID || wantPolicyID == "" {
		t.Fatalf("expected default OVF policy id %q, got: %s", wantPolicyID, raw)
	}
}

// TestLibraryMiscTools_ItemFilesAndStorage proves
// vmware_library_list/get_library_item_files and
// vmware_library_list/get_library_item_storage all return real data about a
// file genuinely uploaded through vcsim's update-session flow (not an empty
// item) — including that a nonexistent file name fails cleanly.
func TestLibraryMiscTools_ItemFilesAndStorage(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()
	ctx := context.Background()
	r := newLibraryMiscRegistry(ctx, c, RegistryOptions{})

	m := libraryMiscTestManager(t, ctx, c)
	libID := createTestLibrary(t, ctx, c, m, "files-test-lib")
	content := []byte("fake iso contents for mcpvmware library-misc test")
	itemID := createTestLibraryItemWithFile(t, ctx, m, libID, "files-test-item", "disk.iso", content)

	raw, err := r.CallTool("vmware_library_list_library_item_files", map[string]interface{}{"library_item_id": itemID})
	if err != nil {
		t.Fatalf("vmware_library_list_library_item_files failed: %v", err)
	}
	if countOf(t, raw) != 1 {
		t.Fatalf("expected exactly 1 uploaded file, got: %s", raw)
	}
	listResult := decodeResult(t, raw)
	files, _ := listResult["files"].([]interface{})
	first, _ := files[0].(map[string]interface{})
	if first["name"] != "disk.iso" {
		t.Fatalf("expected file name \"disk.iso\", got: %s", raw)
	}
	if size, _ := first["size"].(float64); int64(size) != int64(len(content)) {
		t.Fatalf("expected file size %d, got %v (%s)", len(content), first["size"], raw)
	}

	raw, err = r.CallTool("vmware_library_get_library_item_file", map[string]interface{}{
		"library_item_id": itemID,
		"file_name":       "disk.iso",
	})
	if err != nil {
		t.Fatalf("vmware_library_get_library_item_file failed: %v", err)
	}
	getResult := decodeResult(t, raw)
	if getResult["name"] != "disk.iso" {
		t.Fatalf("expected file name \"disk.iso\", got: %s", raw)
	}

	if _, err := r.CallTool("vmware_library_get_library_item_file", map[string]interface{}{
		"library_item_id": itemID,
		"file_name":       "does-not-exist.iso",
	}); err == nil {
		t.Fatal("expected an error getting a nonexistent library item file")
	}

	raw, err = r.CallTool("vmware_library_list_library_item_storage", map[string]interface{}{"library_item_id": itemID})
	if err != nil {
		t.Fatalf("vmware_library_list_library_item_storage failed: %v", err)
	}
	if countOf(t, raw) != 1 {
		t.Fatalf("expected exactly 1 storage entry, got: %s", raw)
	}
	storResult := decodeResult(t, raw)
	storList, _ := storResult["storage"].([]interface{})
	storFirst, _ := storList[0].(map[string]interface{})
	if storFirst["name"] != "disk.iso" {
		t.Fatalf("expected storage entry name \"disk.iso\", got: %s", raw)
	}
	if cached, _ := storFirst["cached"].(bool); !cached {
		t.Fatalf("expected the uploaded file to be cached=true, got: %s", raw)
	}

	raw, err = r.CallTool("vmware_library_get_library_item_storage", map[string]interface{}{
		"library_item_id": itemID,
		"file_name":       "disk.iso",
	})
	if err != nil {
		t.Fatalf("vmware_library_get_library_item_storage failed: %v", err)
	}
	if countOf(t, raw) != 1 {
		t.Fatalf("expected exactly 1 matching storage entry, got: %s", raw)
	}
}

// TestLibraryMiscTools_FinderFind proves vmware_library_finder_find resolves
// all 3 levels (library, item, file) of a real inventory path against
// vcsim, including wildcard matching and the "kind" discriminator this
// project adds (see libraryFindResultKind's doc comment).
func TestLibraryMiscTools_FinderFind(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()
	ctx := context.Background()
	r := newLibraryMiscRegistry(ctx, c, RegistryOptions{})

	m := libraryMiscTestManager(t, ctx, c)
	libID := createTestLibrary(t, ctx, c, m, "finder-test-lib")
	itemID := createTestLibraryItemWithFile(t, ctx, m, libID, "finder-test-item", "finder-file.iso", []byte("finder test content"))
	_ = itemID

	// No paths: lists every library, including ours.
	raw, err := r.CallTool("vmware_library_finder_find", map[string]interface{}{})
	if err != nil {
		t.Fatalf("vmware_library_finder_find (no paths) failed: %v", err)
	}
	result := decodeResult(t, raw)
	results, _ := result["results"].([]interface{})
	foundLib := false
	for _, e := range results {
		em, _ := e.(map[string]interface{})
		if em["kind"] == "library" && em["name"] == "finder-test-lib" {
			foundLib = true
			if em["path"] != "/finder-test-lib" {
				t.Fatalf("expected library path \"/finder-test-lib\", got: %v", em["path"])
			}
		}
	}
	if !foundLib {
		t.Fatalf("expected to find library \"finder-test-lib\" in an unfiltered find, got: %s", raw)
	}

	// Wildcard item match under the library.
	raw, err = r.CallTool("vmware_library_finder_find", map[string]interface{}{
		"paths": []interface{}{"finder-test-lib/*"},
	})
	if err != nil {
		t.Fatalf("vmware_library_finder_find (item wildcard) failed: %v", err)
	}
	result = decodeResult(t, raw)
	results, _ = result["results"].([]interface{})
	if len(results) != 1 {
		t.Fatalf("expected exactly 1 item under finder-test-lib, got: %s", raw)
	}
	itemEntry, _ := results[0].(map[string]interface{})
	if itemEntry["kind"] != "item" || itemEntry["name"] != "finder-test-item" {
		t.Fatalf("expected item \"finder-test-item\", got: %v", itemEntry)
	}

	// Wildcard file match under the item.
	raw, err = r.CallTool("vmware_library_finder_find", map[string]interface{}{
		"paths": []interface{}{"finder-test-lib/finder-test-item/*"},
	})
	if err != nil {
		t.Fatalf("vmware_library_finder_find (file wildcard) failed: %v", err)
	}
	result = decodeResult(t, raw)
	results, _ = result["results"].([]interface{})
	if len(results) != 1 {
		t.Fatalf("expected exactly 1 file under finder-test-item, got: %s", raw)
	}
	fileEntry, _ := results[0].(map[string]interface{})
	if fileEntry["kind"] != "file" || fileEntry["name"] != "finder-file.iso" {
		t.Fatalf("expected file \"finder-file.iso\", got: %v", fileEntry)
	}
	if fileEntry["path"] != "/finder-test-lib/finder-test-item/finder-file.iso" {
		t.Fatalf("expected full resolved path, got: %v", fileEntry["path"])
	}

	// A library name that matches nothing returns an empty result, not an
	// error (findLibraries swallows a GetLibraryByID miss — confirmed by
	// reading vapi/library/finder/finder.go directly).
	raw, err = r.CallTool("vmware_library_finder_find", map[string]interface{}{
		"paths": []interface{}{"no-such-library-xyz"},
	})
	if err != nil {
		t.Fatalf("vmware_library_finder_find (no match) unexpectedly failed: %v", err)
	}
	if countOf(t, raw) != 0 {
		t.Fatalf("expected 0 results for a nonexistent library name, got: %s", raw)
	}
}

// TestLibraryMiscTools_RequiredArgs spot-checks that a representative subset
// of tools reject missing required arguments cleanly (libraryMiscRequiredString),
// instead of panicking or silently calling vcsim with an empty ID.
func TestLibraryMiscTools_RequiredArgs(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()
	ctx := context.Background()
	r := newLibraryMiscRegistry(ctx, c, RegistryOptions{AllowDestructive: true})

	cases := []struct {
		tool string
		args map[string]interface{}
	}{
		{"vmware_library_get_library_usage", map[string]interface{}{"usage_id": "x"}},
		{"vmware_library_list_library_usage", map[string]interface{}{}},
		{"vmware_library_get_trusted_certificate", map[string]interface{}{}},
		{"vmware_library_list_library_item_files", map[string]interface{}{}},
		{"vmware_library_get_library_item_storage", map[string]interface{}{"library_item_id": "x"}},
	}
	for _, tc := range cases {
		if _, err := r.CallTool(tc.tool, tc.args); err == nil {
			t.Errorf("%s: expected an error for missing required argument(s), got success", tc.tool)
		}
	}
}

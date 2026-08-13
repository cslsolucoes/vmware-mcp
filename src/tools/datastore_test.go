package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/vmware/govmomi/simulator"
)

// TestDatastoreTools_UploadAndOverwriteGuard proves the upload actually
// lands on the datastore (not just "no error") and that the overwrite
// guard works, against a real vcsim datastore HTTP file service
// (simulator/host_datastore_browser.go + Service.ServeDatastore) — not a
// fixture, since vcsim genuinely implements this.
func TestDatastoreTools_UploadAndOverwriteGuard(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := NewRegistry(context.Background(), c, RegistryOptions{})

	raw, err := r.CallTool("vmware_list_datastores", map[string]interface{}{})
	if err != nil {
		t.Fatalf("vmware_list_datastores failed: %v", err)
	}
	dsList, _ := decodeResult(t, raw)["datastores"].([]interface{})
	if len(dsList) == 0 {
		t.Fatal("simulator model has no datastores")
	}
	firstDS, _ := dsList[0].(map[string]interface{})
	dsName, _ := firstDS["name"].(string)
	if dsName == "" {
		t.Fatalf("could not read datastore name from vmware_list_datastores result: %v", firstDS)
	}

	localFile := filepath.Join(t.TempDir(), "test.iso")
	if err := os.WriteFile(localFile, []byte("fake iso contents"), 0o644); err != nil {
		t.Fatalf("failed to create local test file: %v", err)
	}

	upload := func(overwrite bool) (string, error) {
		args := map[string]interface{}{
			"datastore":   dsName,
			"local_path":  localFile,
			"remote_path": "uploads/test.iso",
		}
		if overwrite {
			args["overwrite"] = true
		}
		return r.CallTool("vmware_datastore_upload_file", args)
	}

	if _, err := upload(false); err != nil {
		t.Fatalf("first upload (no existing file) should succeed, got: %v", err)
	}

	if _, err := upload(false); err == nil {
		t.Fatal("second upload without overwrite should be refused — file already exists")
	}

	if _, err := upload(true); err != nil {
		t.Fatalf("upload with overwrite:true should succeed, got: %v", err)
	}
}

func TestDatastoreTools_RequiredArgs(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := NewRegistry(context.Background(), c, RegistryOptions{})

	cases := []map[string]interface{}{
		{"local_path": "x", "remote_path": "y"}, // missing datastore
		{"datastore": "*", "remote_path": "y"},  // missing local_path
		{"datastore": "*", "local_path": "x"},   // missing remote_path
	}
	for _, args := range cases {
		if _, err := r.CallTool("vmware_datastore_upload_file", args); err == nil {
			t.Fatalf("expected an error for args %#v, got none", args)
		}
	}
}

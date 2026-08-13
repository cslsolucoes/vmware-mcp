package tools

import (
	"context"
	"errors"
	"fmt"
	"path"

	"github.com/vmware/govmomi/object"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// registerDatastoreTools registers datastore file operations. Not part of
// the originally approved plan's 5 domains — added 10/08/2026 at the
// user's explicit request, after confirming the semantics: local_path is
// read from the filesystem of the machine running this MCP server, never
// received from the MCP client/chat (MCP has no mechanism for a client to
// stream a file into a tool call).
func registerDatastoreTools(r *Registry) {
	r.register("vmware_datastore_upload_file",
		"Upload a file (e.g. an ISO) to a path inside a vSphere datastore. local_path is read from the filesystem of the machine running this MCP server — NOT uploaded from the MCP client/chat. Refuses to overwrite an existing file at remote_path unless overwrite:true.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"datastore": map[string]interface{}{
					"type":        "string",
					"description": `Datastore name/pattern (e.g. "datastore1") as returned by vmware_list_datastores. Must resolve to exactly one datastore.`,
				},
				"local_path": map[string]interface{}{
					"type":        "string",
					"description": "Path to the file on the machine running this MCP server. Must already exist and be readable there.",
				},
				"remote_path": map[string]interface{}{
					"type":        "string",
					"description": `Destination path inside the datastore, e.g. "ISOs/ubuntu-24.04.iso". Parent folders are created automatically if missing.`,
				},
				"overwrite": map[string]interface{}{
					"type":        "boolean",
					"description": "Allow replacing an existing file at remote_path. Default false — the upload is refused if a file already exists there.",
				},
			},
			"required": []interface{}{"datastore", "local_path", "remote_path"},
		},
		Tool{Handler: handleDatastoreUploadFile},
	)
}

// resolveDatastore resolves the given name/pattern to exactly one
// datastore, scoping it with dcScopedPath for the same multi-datacenter
// reason as resolveVM/resolveHost.
func resolveDatastore(ctx context.Context, client *vmware.Client, name string) (*object.Datastore, error) {
	if name == "" {
		return nil, fmt.Errorf("datastore is required")
	}
	ds, err := client.Finder.Datastore(ctx, dcScopedPath("datastore", name))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve datastore %q: %w", name, err)
	}
	return ds, nil
}

// datastoreFileMissing reports whether err from Datastore.Stat means "does
// not exist yet" (safe to upload) as opposed to a real failure.
func datastoreFileMissing(err error) bool {
	var nf object.DatastoreNoSuchFileError
	var nd object.DatastoreNoSuchDirectoryError
	return errors.As(err, &nf) || errors.As(err, &nd)
}

func handleDatastoreUploadFile(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	dsName, _ := args["datastore"].(string)
	localPath, _ := args["local_path"].(string)
	remotePath, _ := args["remote_path"].(string)
	overwrite, _ := args["overwrite"].(bool)

	if localPath == "" {
		return "", fmt.Errorf("local_path is required")
	}
	if remotePath == "" {
		return "", fmt.Errorf("remote_path is required")
	}

	ds, err := resolveDatastore(ctx, client, dsName)
	if err != nil {
		return "", err
	}

	if !overwrite {
		if _, statErr := ds.Stat(ctx, remotePath); statErr == nil {
			return "", fmt.Errorf("%s already exists on datastore %s — pass overwrite:true to replace it", remotePath, ds.InventoryPath)
		} else if !datastoreFileMissing(statErr) {
			return "", fmt.Errorf("failed to check existing file %s on datastore %s: %w", remotePath, ds.InventoryPath, statErr)
		}
	}

	// Best-effort: create the destination folder (and any missing parents)
	// if it doesn't exist yet. Errors here are deliberately swallowed —
	// "already exists" is the common case on a 2nd+ upload into the same
	// folder, and any real problem (permissions, etc.) still surfaces
	// clearly from the UploadFile call right below.
	if dir := path.Dir(remotePath); dir != "." && dir != "/" {
		fm := object.NewFileManager(client.Client.Client)
		_ = fm.MakeDirectory(ctx, ds.Path(dir), nil, true)
	}

	if err := ds.UploadFile(ctx, localPath, remotePath, nil); err != nil {
		return "", fmt.Errorf("failed to upload %s to datastore %s at %s: %w", localPath, ds.InventoryPath, remotePath, err)
	}

	return marshalJSON(map[string]interface{}{
		"datastore":   ds.InventoryPath,
		"remote_path": remotePath,
		"result":      "uploaded",
	})
}

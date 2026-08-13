package tools

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/vapi/vm/dataset"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// registerVMDatasetTools is part of the "MISC-APPLIANCE" group of Fase 8a
// Wave 2 of the codegen plan
// (".workspace/plans/MCPVMWare2026-08-10-175300-plano-cobertura-completa-api-codegen.plan.md")
// — vapi/vm/dataset.Manager, hand-transcribed from the real
// referencia/govmomi/vapi/vm/dataset/dataset.go source (confirmed identical,
// byte-for-byte modulo line endings, to the pinned dependency
// github.com/vmware/govmomi v0.55.1 actually resolved by src/go.mod — this
// project does not vendor). "VM Data Sets" is a vSphere 8.0+ mechanism for
// exchanging small key/value data between the hypervisor and the guest OS via
// the VMware Guest SDK; it requires the VM to have virtual hardware version
// 20 or newer. 9 methods, all legitimate, no exclusions: CreateDataSet
// (tier2), DeleteDataSet (tier1), DeleteEntry (tier1), GetDataSet (read-only),
// GetEntry (read-only), ListDataSets (read-only), ListEntries (read-only),
// SetEntry (tier2), UpdateDataSet (tier2) — all exactly matching
// src/gen/classification.json's tier/proposed_tool assignments; no curation
// deviation for this file.
//
// Architecturally the same class as generated_library_core.go/
// generated_tags.go (Fase 8a's vapi/*.go REST-over-JSON wrappers, not
// object/*.go SOAP-over-XML) — every dataset.CreateSpec/UpdateSpec/Info/
// Summary struct already carries native json tags, so decodeJSONArg
// (generated_vm_lifecycle.go) is used to decode "spec" arguments directly
// into the concrete govmomi struct, the same mechanism generated_library_core.go
// uses for library.Library/library.Item, not for polymorphism (there is none
// in this domain) but for convenience/consistency.
//
// The "vm" argument is a REAL DEPARTURE from every other tool in this
// project: per this group's brief, it is treated as an opaque REST VM ID
// string (e.g. "vm-21") passed straight through to dataset.Manager, with NO
// Finder-based resolveVM lookup. Confirmed correct by reading
// referencia/govmomi/cli/vm/dataset/create.go (govc's own CLI command for
// this exact API): `vmId := vm.Reference().Value` — i.e. even govc's own
// vm-name-resolving flag ultimately passes the bare moref Value string into
// this REST API, not a resolved object or an inventory path. In practice
// that value is identical to the VM's SOAP moref Value (the same string
// vmware_vm_info reports as its moref), so a caller who already resolved a
// VM through this project's other (object/*.go-backed) tools can reuse that
// same ID here — but this file itself performs no such resolution, matching
// the brief's explicit instruction and this domain's own REST identity
// model (unlike library_id/item_id in generated_library_core.go, "vm" here
// is not a domain-native opaque ID minted by a create call in this same
// domain — it is borrowed from the SOAP domain by convention, not by API
// contract).
//
// vcsim gap, not a bug — confirmed directly, not assumed from the brief:
// `grep -rn "data-sets\|vm/dataset" referencia/govmomi/vapi/simulator/simulator.go`
// returns 0 matches (vapi/simulator only imports vapi/library, vapi/tags,
// vapi/vcenter, vapi/rest — see this project's Fase 8a groups' shared
// finding). simulator/dataset.go DOES exist in the SOAP-side simulator core,
// but only as internal state used by VM clone/snapshot machinery — it is
// never wired to any REST route, so it does not make vapi/vm/dataset.Manager
// usable against vcsim. Every handler below is therefore tested only for
// "reaches the server cleanly" (assertReachesServer, defined in
// generated_vm_lifecycle_test.go and reused verbatim here), the same
// discipline as every other vcsim-gap domain in this project
// (generated_authorization.go's DisableMethods/EnableMethods,
// generated_network.go's ReconfigureDVPort/UpdateDVSLacpGroupConfig).
func registerVMDatasetTools(r *Registry) {
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}
	vmArg := map[string]interface{}{
		"type":        "string",
		"description": `VM identified by its raw vCenter REST API ID (e.g. "vm-21") — the SAME string as the VM's SOAP managed-object-reference Value (see vmware_vm_info's moref, or object.VirtualMachine.Reference().Value in govmomi). This domain does NOT resolve a name/inventory-path pattern the way most other tools' "vm" argument does — pass the raw ID directly (confirmed against govc's own vm.dataset.* CLI commands, which do exactly this).`,
	}
	dataSetArg := map[string]interface{}{
		"type":        "string",
		"description": `Data set name (e.g. "com.example.project") — the same value passed as the "name" field of the create spec. Data sets are identified by this name, not a separate server-generated ID.`,
	}
	keyArg := map[string]interface{}{
		"type":        "string",
		"description": "Entry key within the data set. At most 4096 bytes.",
	}
	accessEnumDesc := `One of "NONE", "READ_ONLY", "READ_WRITE".`

	// --- Read-only ---------------------------------------------------------

	r.register("vmware_vm_dataset_get_data_set",
		"Get information about a VM data set (name, description, host/guest access levels, entry count, snapshot/clone inclusion).",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm":       vmArg,
				"data_set": dataSetArg,
			},
			"required": []interface{}{"vm", "data_set"},
		},
		Tool{Handler: handleVMDatasetGetDataSet},
	)

	r.register("vmware_vm_dataset_get_entry",
		"Get the value of a single entry in a VM data set.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm":       vmArg,
				"data_set": dataSetArg,
				"key":      keyArg,
			},
			"required": []interface{}{"vm", "data_set", "key"},
		},
		Tool{Handler: handleVMDatasetGetEntry},
	)

	r.register("vmware_vm_dataset_list_data_sets",
		"List brief descriptions of every data set on a virtual machine.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"vm": vmArg},
			"required":   []interface{}{"vm"},
		},
		Tool{Handler: handleVMDatasetListDataSets},
	)

	r.register("vmware_vm_dataset_list_entries",
		"List every entry key in a VM data set (values are not included — use vmware_vm_dataset_get_entry per key).",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm":       vmArg,
				"data_set": dataSetArg,
			},
			"required": []interface{}{"vm", "data_set"},
		},
		Tool{Handler: handleVMDatasetListEntries},
	)

	// --- Tier 2: disruptive but reversible ----------------------------------

	r.registerDestructive("vmware_vm_dataset_create_data_set",
		"Create a new data set on a virtual machine (requires vSphere 8.0+ and VM hardware version 20+). Returns the new data set's identifier.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm": vmArg,
				"spec": map[string]interface{}{
					"type":        "object",
					"description": `A dataset.CreateSpec JSON object (matching its Go struct's json tags). Required: "name" (form "com.company.project" recommended, to avoid collisions), "host" (` + accessEnumDesc + ` — controls ESXi/vCenter API access to entries), "guest" (` + accessEnumDesc + ` — controls in-guest VMware Guest SDK access to entries). Optional: "description", "omit_from_snapshot_and_clone" (bool, default false — if true, the data set is destroyed when the VM is reverted to a snapshot taken before it existed, and is not carried over by clones).`,
				},
				"confirm": confirmArg,
			},
			"required": []interface{}{"vm", "spec", "confirm"},
		},
		Tool{Handler: handleVMDatasetCreateDataSet},
	)

	r.registerDestructive("vmware_vm_dataset_set_entry",
		"Create or overwrite an entry in a VM data set. Key at most 4096 bytes, value at most 1MB.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm":       vmArg,
				"data_set": dataSetArg,
				"key":      keyArg,
				"value":    map[string]interface{}{"type": "string", "description": "Entry value. At most 1MB."},
				"confirm":  confirmArg,
			},
			"required": []interface{}{"vm", "data_set", "key", "value", "confirm"},
		},
		Tool{Handler: handleVMDatasetSetEntry},
	)

	r.registerDestructive("vmware_vm_dataset_update_data_set",
		"Modify a VM data set's description, host/guest access levels, and/or snapshot-and-clone inclusion. Every field is optional — omit a field to leave it unchanged (dataset.UpdateSpec uses pointer fields specifically so absence means \"no change\", confirmed by reading the struct).",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm":       vmArg,
				"data_set": dataSetArg,
				"spec": map[string]interface{}{
					"type":        "object",
					"description": `A dataset.UpdateSpec JSON object. All fields optional: "description" (string), "host" (` + accessEnumDesc + `), "guest" (` + accessEnumDesc + `), "omit_from_snapshot_and_clone" (bool). Omitted fields are left unchanged server-side.`,
				},
				"confirm": confirmArg,
			},
			"required": []interface{}{"vm", "data_set", "spec", "confirm"},
		},
		Tool{Handler: handleVMDatasetUpdateDataSet},
	)

	// --- Tier 1: irreversible ------------------------------------------------

	r.registerDestructive("vmware_vm_dataset_delete_data_set",
		"Delete a VM data set. Irreversible. Fails if the data set still has entries — pass force:true to delete a non-empty data set anyway.",
		tier1,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm":       vmArg,
				"data_set": dataSetArg,
				"force":    map[string]interface{}{"type": "boolean", "description": "Delete even if the data set still has entries. Default false."},
				"confirm":  confirmArg,
			},
			"required": []interface{}{"vm", "data_set", "confirm"},
		},
		Tool{Handler: handleVMDatasetDeleteDataSet},
	)

	r.registerDestructive("vmware_vm_dataset_delete_entry",
		"Delete a single entry from a VM data set. Irreversible.",
		tier1,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm":       vmArg,
				"data_set": dataSetArg,
				"key":      keyArg,
				"confirm":  confirmArg,
			},
			"required": []interface{}{"vm", "data_set", "key", "confirm"},
		},
		Tool{Handler: handleVMDatasetDeleteEntry},
	)
}

// vmDatasetManager returns a *dataset.Manager for client — this file's
// equivalent of libraryCoreManager (generated_library_core.go): client.REST(ctx)
// (added in Fase 4 for VAMI) already names the likely cause of failure ("is
// the target a vCenter Server Appliance?") if called against a standalone
// ESXi host.
func vmDatasetManager(ctx context.Context, client *vmware.Client) (*dataset.Manager, error) {
	rc, err := client.REST(ctx)
	if err != nil {
		return nil, err
	}
	return dataset.NewManager(rc), nil
}

func handleVMDatasetGetDataSet(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := vmDatasetManager(ctx, client)
	if err != nil {
		return "", err
	}
	vm, err := requiredStringArg(args, "vm")
	if err != nil {
		return "", err
	}
	ds, err := requiredStringArg(args, "data_set")
	if err != nil {
		return "", err
	}

	info, err := m.GetDataSet(ctx, vm, ds)
	if err != nil {
		return "", fmt.Errorf("failed to get data set %q on vm %q: %w", ds, vm, err)
	}
	return marshalJSON(map[string]interface{}{"vm": vm, "data_set": ds, "info": info})
}

func handleVMDatasetGetEntry(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := vmDatasetManager(ctx, client)
	if err != nil {
		return "", err
	}
	vm, err := requiredStringArg(args, "vm")
	if err != nil {
		return "", err
	}
	ds, err := requiredStringArg(args, "data_set")
	if err != nil {
		return "", err
	}
	key, err := requiredStringArg(args, "key")
	if err != nil {
		return "", err
	}

	value, err := m.GetEntry(ctx, vm, ds, key)
	if err != nil {
		return "", fmt.Errorf("failed to get entry %q of data set %q on vm %q: %w", key, ds, vm, err)
	}
	return marshalJSON(map[string]interface{}{"vm": vm, "data_set": ds, "key": key, "value": value})
}

func handleVMDatasetListDataSets(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := vmDatasetManager(ctx, client)
	if err != nil {
		return "", err
	}
	vm, err := requiredStringArg(args, "vm")
	if err != nil {
		return "", err
	}

	sets, err := m.ListDataSets(ctx, vm)
	if err != nil {
		return "", fmt.Errorf("failed to list data sets on vm %q: %w", vm, err)
	}
	return marshalJSON(map[string]interface{}{"vm": vm, "data_sets": sets, "count": len(sets)})
}

func handleVMDatasetListEntries(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := vmDatasetManager(ctx, client)
	if err != nil {
		return "", err
	}
	vm, err := requiredStringArg(args, "vm")
	if err != nil {
		return "", err
	}
	ds, err := requiredStringArg(args, "data_set")
	if err != nil {
		return "", err
	}

	keys, err := m.ListEntries(ctx, vm, ds)
	if err != nil {
		return "", fmt.Errorf("failed to list entries of data set %q on vm %q: %w", ds, vm, err)
	}
	return marshalJSON(map[string]interface{}{"vm": vm, "data_set": ds, "keys": keys, "count": len(keys)})
}

func handleVMDatasetCreateDataSet(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := vmDatasetManager(ctx, client)
	if err != nil {
		return "", err
	}
	vm, err := requiredStringArg(args, "vm")
	if err != nil {
		return "", err
	}
	raw, ok := args["spec"]
	if !ok {
		return "", fmt.Errorf("spec is required")
	}
	var spec dataset.CreateSpec
	if err := decodeJSONArg(raw, &spec); err != nil {
		return "", fmt.Errorf("invalid spec: %w", err)
	}
	if spec.Name == "" {
		return "", fmt.Errorf("spec.name is required")
	}
	if spec.Host == "" {
		return "", fmt.Errorf("spec.host is required")
	}
	if spec.Guest == "" {
		return "", fmt.Errorf("spec.guest is required")
	}

	id, err := m.CreateDataSet(ctx, vm, &spec)
	if err != nil {
		return "", fmt.Errorf("failed to create data set %q on vm %q: %w", spec.Name, vm, err)
	}
	return marshalJSON(map[string]interface{}{"vm": vm, "id": id, "name": spec.Name, "result": "created"})
}

func handleVMDatasetSetEntry(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := vmDatasetManager(ctx, client)
	if err != nil {
		return "", err
	}
	vm, err := requiredStringArg(args, "vm")
	if err != nil {
		return "", err
	}
	ds, err := requiredStringArg(args, "data_set")
	if err != nil {
		return "", err
	}
	key, err := requiredStringArg(args, "key")
	if err != nil {
		return "", err
	}
	value, _ := args["value"].(string)

	if err := m.SetEntry(ctx, vm, ds, key, value); err != nil {
		return "", fmt.Errorf("failed to set entry %q of data set %q on vm %q: %w", key, ds, vm, err)
	}
	return marshalJSON(map[string]interface{}{"vm": vm, "data_set": ds, "key": key, "result": "set"})
}

func handleVMDatasetUpdateDataSet(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := vmDatasetManager(ctx, client)
	if err != nil {
		return "", err
	}
	vm, err := requiredStringArg(args, "vm")
	if err != nil {
		return "", err
	}
	ds, err := requiredStringArg(args, "data_set")
	if err != nil {
		return "", err
	}
	raw, ok := args["spec"]
	if !ok {
		return "", fmt.Errorf("spec is required")
	}
	var spec dataset.UpdateSpec
	if err := decodeJSONArg(raw, &spec); err != nil {
		return "", fmt.Errorf("invalid spec: %w", err)
	}

	if err := m.UpdateDataSet(ctx, vm, ds, &spec); err != nil {
		return "", fmt.Errorf("failed to update data set %q on vm %q: %w", ds, vm, err)
	}
	return marshalJSON(map[string]interface{}{"vm": vm, "data_set": ds, "result": "updated"})
}

func handleVMDatasetDeleteDataSet(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := vmDatasetManager(ctx, client)
	if err != nil {
		return "", err
	}
	vm, err := requiredStringArg(args, "vm")
	if err != nil {
		return "", err
	}
	ds, err := requiredStringArg(args, "data_set")
	if err != nil {
		return "", err
	}
	force, _ := args["force"].(bool)

	if err := m.DeleteDataSet(ctx, vm, ds, force); err != nil {
		return "", fmt.Errorf("failed to delete data set %q on vm %q: %w", ds, vm, err)
	}
	return marshalJSON(map[string]interface{}{"vm": vm, "data_set": ds, "force": force, "result": "deleted"})
}

func handleVMDatasetDeleteEntry(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	m, err := vmDatasetManager(ctx, client)
	if err != nil {
		return "", err
	}
	vm, err := requiredStringArg(args, "vm")
	if err != nil {
		return "", err
	}
	ds, err := requiredStringArg(args, "data_set")
	if err != nil {
		return "", err
	}
	key, err := requiredStringArg(args, "key")
	if err != nil {
		return "", err
	}

	if err := m.DeleteEntry(ctx, vm, ds, key); err != nil {
		return "", fmt.Errorf("failed to delete entry %q of data set %q on vm %q: %w", key, ds, vm, err)
	}
	return marshalJSON(map[string]interface{}{"vm": vm, "data_set": ds, "key": key, "result": "deleted"})
}

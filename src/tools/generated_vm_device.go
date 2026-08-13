package tools

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// registerVMDeviceTools is part of Fase 2 of the codegen plan
// (".workspace/plans/MCPVMWare2026-08-10-175300-plano-cobertura-completa-api-codegen.plan.md")
// — VirtualMachine's device-editing methods (AddDevice, EditDevice,
// RemoveDevice, AttachDisk, DetachDisk), hand-transcribed from
// src/gen/classification.json following the vm.go/generated_option.go/
// generated_vm_snapshot.go conventions.
//
// Curation deviations from the raw classification (human review required —
// this is the polymorphic-device slice, the trickiest curation call in the
// VM domain):
//   - AddDevice/EditDevice/RemoveDevice take ...types.BaseVirtualDevice, an
//     interface implemented by dozens of concrete device structs (disks,
//     CD-ROMs, NICs, USB controllers, ...). Building a fully generic "any
//     device type" JSON schema is out of scope for this pass (explicit
//     MVP-scoping decision). This file supports exactly 2 concrete device
//     types via a "device_type": "disk"|"cdrom" discriminator —
//     attaching/detaching virtual disks and mounting/ejecting CD-ROM/ISO
//     images cover the overwhelming majority of real device-management
//     needs. A human can extend the switch in buildNewDevice with more
//     types.BaseVirtualDevice implementers later (NICs, USB, etc.) without
//     changing the schema shape.
//   - Rather than hand-building types.VirtualDisk/types.VirtualCdrom structs
//     field-by-field, the handlers reuse govmomi's own
//     object.VirtualDeviceList helpers (FindDiskController, FindIDEController,
//     CreateDisk, CreateCdrom, InsertIso, EjectIso, FindByKey) — the same
//     helpers govc's own CLI commands use (see referencia/govmomi/cli/vm/disk/create.go
//     and referencia/govmomi/cli/device/cdrom/add.go, confirmed by reading
//     both, not guessed). This avoids re-deriving controller/unit-number
//     assignment logic that govmomi already gets right.
//   - EditDevice's real signature is fully generic
//     (...types.BaseVirtualDevice, no notion of "what changed"). This MVP
//     narrows it to the one common, well-defined edit: swapping or ejecting
//     a CD-ROM's media (device_key must resolve to an existing
//     *types.VirtualCdrom). Editing other device types (e.g. growing a
//     disk's capacity, changing a NIC's network) is not supported yet — a
//     caller hitting that need gets a clear "not a cdrom" error, not a
//     silent no-op or a panic.
//   - AddDevice's disk case only supports creating a brand NEW virtual disk
//     (a fresh file on a datastore, sized via capacity_kb). Attaching an
//     EXISTING disk file already on a datastore is a materially different
//     operation with its own govmomi method (VirtualMachine.AttachDisk) and
//     is exposed separately as vmware_vm_attach_disk/vmware_vm_detach_disk,
//     matching the plan's explicit split between the two.
//   - iso_path for the cdrom case is the full bracketed datastore-path
//     string vSphere itself uses for file backings (e.g.
//     "[datastore1] ISOs/ubuntu-24.04.iso") — not a bare relative path. A
//     caller chains this with vmware_datastore_upload_file (whose
//     remote_path is datastore-relative) by combining the datastore name
//     and remote_path into that bracketed form itself; this file does not
//     do that combination on the caller's behalf, to keep the schema
//     independent of datastore.go and avoid an extra round trip when the
//     caller already knows the exact path.
//   - Tiering: AddDevice/EditDevice/AttachDisk are Tier 2 (disruptive but
//     reversible — swap the media back, remove the device again).
//     RemoveDevice/DetachDisk are Tier 1: RemoveDevice defaults to deleting
//     the underlying disk file (fileOperation "destroy" unless
//     keep_files:true — see object.VirtualMachine.RemoveDevice, confirmed
//     by reading the source), which is irreversible for a disk device
//     unless the caller opts out; DetachDisk removes a disk from the VM's
//     configuration entirely (closer in blast radius to RemoveDevice than
//     to a reversible reconfigure) — both grouped with the plan's Tier 1
//     "irreversible" bucket rather than Tier 2, matching vm.go's own
//     destroy/snapshot-remove precedent of erring toward the stricter tier
//     when a govmomi call can delete data.
//
// vcsim limitation found while testing (see generated_vm_device_test.go's
// TestVMDevice_AttachDiskVcsimLimitation for the full explanation):
// VirtualMachine.AttachDisk against simulator.ESX() (standalone-host mode,
// used everywhere else in this package) PANICS inside the simulator itself
// — simulator.Registry.VStorageObjectManager() unconditionally asserts its
// singleton to *simulator.VcenterVStorageObjectManager, which only exists
// under simulator.VPX() (vCenter mode); under ESX() it's actually a
// *simulator.HostVStorageObjectManager, and the panic happens in a
// vcsim-internal task goroutine that has no recover, crashing the whole
// test process (not something this project's own CallTool panic-recovery
// can catch). vmware_vm_attach_disk's real behavior is therefore only
// tested against simulator.VPX() here; every other tool in this file is
// tested against simulator.ESX() like the rest of the package.
func registerVMDeviceTools(r *Registry) {
	vmArg := map[string]interface{}{
		"type":        "string",
		"description": `VM identifier: a name/pattern (e.g. "cac-WN02") or a full inventory path (e.g. "/ha-datacenter/vm/cac-WN02") as returned by vmware_list_vms. Must resolve to exactly one VM.`,
	}
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}
	deviceKeyArg := map[string]interface{}{
		"type":        "integer",
		"description": "Key of an existing device on the VM, as reported by vm.Device()/the sibling vmware_vm_device read tool. Device keys are assigned by vSphere and are stable for the life of the device.",
	}

	r.registerDestructive("vmware_vm_add_device",
		`Add a new virtual device to a VM. MVP scope: device_type "disk" creates a brand new virtual disk file on a datastore; device_type "cdrom" adds a CD-ROM drive, optionally with an ISO already mounted. To attach an EXISTING disk file instead of creating one, use vmware_vm_attach_disk.`,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm":          vmArg,
				"device_type": map[string]interface{}{"type": "string", "enum": []interface{}{"disk", "cdrom"}, "description": `Which kind of device to add. Only "disk" and "cdrom" are supported in this MVP scope.`},
				"datastore":   map[string]interface{}{"type": "string", "description": `Datastore name/pattern (e.g. "datastore1") as returned by vmware_list_datastores, to place the new disk file on. Required when device_type is "disk"; ignored for "cdrom".`},
				"capacity_kb": map[string]interface{}{"type": "integer", "description": `New disk size in kilobytes. Required when device_type is "disk"; ignored for "cdrom".`},
				"file_name":   map[string]interface{}{"type": "string", "description": `vmdk file name/path inside the datastore, e.g. "myvm/disk2.vmdk". Optional for device_type "disk" — if omitted, vSphere auto-names the file. Ignored for "cdrom".`},
				"thin_provisioned": map[string]interface{}{
					"type":        "boolean",
					"description": `Thin-provision the new disk. Default true. Only applies to device_type "disk".`,
				},
				"controller_type": map[string]interface{}{
					"type":        "string",
					"description": `Disk controller type to attach the new disk to: "scsi" (default), "ide", "sata", or "nvme". A controller of that type must already exist on the VM. Only applies to device_type "disk".`,
				},
				"iso_path": map[string]interface{}{
					"type":        "string",
					"description": `Datastore path of the ISO to mount, in bracketed form, e.g. "[datastore1] ISOs/ubuntu-24.04.iso" (combine a datastore name from vmware_list_datastores with a path uploaded via vmware_datastore_upload_file). Only applies to device_type "cdrom". If omitted, an empty CD-ROM drive (no media) is added.`,
				},
				"confirm": confirmArg,
			},
			"required": []interface{}{"vm", "device_type", "confirm"},
		},
		Tool{Handler: handleVMAddDevice},
	)

	r.registerDestructive("vmware_vm_edit_device",
		`Edit an existing virtual device on a VM. MVP scope: only CD-ROM devices are supported — swap the mounted ISO, or eject it back to an empty drive. Editing other device types is not supported yet.`,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm":         vmArg,
				"device_key": deviceKeyArg,
				"iso_path":   map[string]interface{}{"type": "string", "description": `New ISO to mount on the CD-ROM, in the same bracketed datastore-path form as vmware_vm_add_device's iso_path. Mutually exclusive with eject.`},
				"eject":      map[string]interface{}{"type": "boolean", "description": "Eject the currently mounted ISO, reverting the CD-ROM to an empty/client-device backing. Mutually exclusive with iso_path."},
				"confirm":    confirmArg,
			},
			"required": []interface{}{"vm", "device_key", "confirm"},
		},
		Tool{Handler: handleVMEditDevice},
	)

	r.registerDestructive("vmware_vm_attach_disk",
		`Attach an EXISTING virtual disk file already present on a datastore to a VM. This does NOT create a new disk — use vmware_vm_add_device with device_type "disk" for that.`,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm":             vmArg,
				"id":             map[string]interface{}{"type": "string", "description": "Identifier (datastore-relative vmdk path) of the existing virtual disk to attach."},
				"datastore":      map[string]interface{}{"type": "string", "description": "Datastore name/pattern where the disk identified by id lives."},
				"controller_key": map[string]interface{}{"type": "integer", "description": "Key of the controller device to attach the disk to. Optional — omit to let vSphere pick."},
				"unit_number":    map[string]interface{}{"type": "integer", "description": "Unit number on the controller to attach at. Optional — omit to let vSphere pick."},
				"confirm":        confirmArg,
			},
			"required": []interface{}{"vm", "id", "datastore", "confirm"},
		},
		Tool{Handler: handleVMAttachDisk},
	)

	r.registerDestructive("vmware_vm_remove_device",
		"Remove a virtual device from a VM. If the device is a virtual disk, its underlying file is deleted from the datastore unless keep_files:true is passed. Irreversible for a deleted disk file.",
		tier1,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm":         vmArg,
				"device_key": deviceKeyArg,
				"keep_files": map[string]interface{}{"type": "boolean", "description": "If the removed device is a virtual disk, keep its underlying file on the datastore instead of deleting it. Default false."},
				"confirm":    confirmArg,
			},
			"required": []interface{}{"vm", "device_key", "confirm"},
		},
		Tool{Handler: handleVMRemoveDevice},
	)

	r.registerDestructive("vmware_vm_detach_disk",
		"Detach a virtual disk from a VM without deleting its underlying file — the disk file remains on the datastore and can be re-attached later with vmware_vm_attach_disk.",
		tier1,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vm":      vmArg,
				"id":      map[string]interface{}{"type": "string", "description": "Identifier (datastore-relative vmdk path) of the disk to detach — same identifier space as vmware_vm_attach_disk's id."},
				"confirm": confirmArg,
			},
			"required": []interface{}{"vm", "id", "confirm"},
		},
		Tool{Handler: handleVMDetachDisk},
	)
}

func handleVMAddDevice(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, err := resolveVM(ctx, client, args)
	if err != nil {
		return "", err
	}
	deviceType, _ := args["device_type"].(string)

	devices, err := vm.Device(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to read device list for %s: %w", vm.InventoryPath, err)
	}

	var device types.BaseVirtualDevice
	switch deviceType {
	case "disk":
		device, err = buildNewDisk(ctx, client, devices, args)
	case "cdrom":
		device, err = buildNewCdrom(devices, args)
	default:
		return "", fmt.Errorf(`device_type must be "disk" or "cdrom", got %q`, deviceType)
	}
	if err != nil {
		return "", err
	}

	if err := vm.AddDevice(ctx, device); err != nil {
		return "", fmt.Errorf("failed to add %s device to %s: %w", deviceType, vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"vm": vm.InventoryPath, "device_type": deviceType, "result": "added"})
}

// buildNewDisk constructs a new *types.VirtualDisk via
// object.VirtualDeviceList.CreateDisk (same helper govc's own
// device.disk.create / vm.disk.create commands use — see
// referencia/govmomi/cli/vm/disk/create.go), instead of hand-assembling the
// backing/controller/unit-number wiring. Passing "" (not
// ds.Path("")) as CreateDisk's name argument is deliberate: CreateDisk's own
// doc comment says an empty name lets vSphere auto-name the file, but
// ds.Path("") would return the non-empty string "[datastore]", which
// CreateDisk would then (wrongly) treat as an explicit name and suffix with
// ".vmdk".
func buildNewDisk(ctx context.Context, client *vmware.Client, devices object.VirtualDeviceList, args map[string]interface{}) (types.BaseVirtualDevice, error) {
	dsName, _ := args["datastore"].(string)
	if dsName == "" {
		return nil, fmt.Errorf(`datastore is required when device_type is "disk"`)
	}
	capRaw, ok := args["capacity_kb"]
	if !ok {
		return nil, fmt.Errorf(`capacity_kb is required when device_type is "disk"`)
	}
	capacityKB, err := toInt64(capRaw)
	if err != nil {
		return nil, fmt.Errorf("invalid capacity_kb: %w", err)
	}
	if capacityKB <= 0 {
		return nil, fmt.Errorf("capacity_kb must be a positive number of kilobytes")
	}

	ds, err := resolveDatastore(ctx, client, dsName)
	if err != nil {
		return nil, err
	}

	controllerType, _ := args["controller_type"].(string)
	controller, err := devices.FindDiskController(controllerType)
	if err != nil {
		return nil, fmt.Errorf("failed to find a disk controller: %w", err)
	}

	fileName, _ := args["file_name"].(string)
	diskName := ""
	if fileName != "" {
		diskName = ds.Path(fileName)
	}

	disk := devices.CreateDisk(controller, ds.Reference(), diskName)
	disk.CapacityInKB = capacityKB

	backing, ok := disk.Backing.(*types.VirtualDiskFlatVer2BackingInfo)
	if !ok {
		// CreateDisk always sets a VirtualDiskFlatVer2BackingInfo backing
		// today (confirmed by reading its source) — this branch is defense
		// in depth against a future govmomi change, not a case reachable
		// now.
		return nil, fmt.Errorf("internal error: unexpected disk backing type %T", disk.Backing)
	}
	thin := true
	if v, ok := args["thin_provisioned"].(bool); ok {
		thin = v
	}
	backing.ThinProvisioned = types.NewBool(thin)

	return disk, nil
}

// buildNewCdrom constructs a new *types.VirtualCdrom on the VM's IDE
// controller via object.VirtualDeviceList.CreateCdrom/InsertIso — same
// helpers govc's device.cdrom.add/device.cdrom.insert commands use (see
// referencia/govmomi/cli/device/cdrom/add.go and .../cdrom/insert.go).
func buildNewCdrom(devices object.VirtualDeviceList, args map[string]interface{}) (types.BaseVirtualDevice, error) {
	controller, err := devices.FindIDEController("")
	if err != nil {
		return nil, fmt.Errorf("failed to find an IDE controller for the CD-ROM device: %w", err)
	}

	cdrom, err := devices.CreateCdrom(controller)
	if err != nil {
		return nil, fmt.Errorf("failed to create cdrom device: %w", err)
	}

	if isoPath, _ := args["iso_path"].(string); isoPath != "" {
		cdrom = devices.InsertIso(cdrom, isoPath)
	}

	return cdrom, nil
}

func handleVMEditDevice(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, err := resolveVM(ctx, client, args)
	if err != nil {
		return "", err
	}

	key, err := requiredDeviceKey(args)
	if err != nil {
		return "", err
	}

	devices, err := vm.Device(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to read device list for %s: %w", vm.InventoryPath, err)
	}

	found := devices.FindByKey(key)
	if found == nil {
		return "", fmt.Errorf("no device with key %d found on %s", key, vm.InventoryPath)
	}
	cdrom, ok := found.(*types.VirtualCdrom)
	if !ok {
		return "", fmt.Errorf("device %d on %s is a %s, not a cdrom — vmware_vm_edit_device only supports editing CD-ROM devices in this MVP scope", key, vm.InventoryPath, devices.Type(found))
	}

	isoPath, _ := args["iso_path"].(string)
	eject, _ := args["eject"].(bool)

	switch {
	case isoPath != "" && eject:
		return "", fmt.Errorf("iso_path and eject are mutually exclusive")
	case isoPath != "":
		cdrom = devices.InsertIso(cdrom, isoPath)
	case eject:
		cdrom = devices.EjectIso(cdrom)
	default:
		return "", fmt.Errorf("either iso_path or eject:true is required")
	}

	if err := vm.EditDevice(ctx, cdrom); err != nil {
		return "", fmt.Errorf("failed to edit device %d on %s: %w", key, vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"vm": vm.InventoryPath, "device_key": key, "result": "edited"})
}

func handleVMRemoveDevice(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, err := resolveVM(ctx, client, args)
	if err != nil {
		return "", err
	}

	key, err := requiredDeviceKey(args)
	if err != nil {
		return "", err
	}
	keepFiles, _ := args["keep_files"].(bool)

	devices, err := vm.Device(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to read device list for %s: %w", vm.InventoryPath, err)
	}
	device := devices.FindByKey(key)
	if device == nil {
		return "", fmt.Errorf("no device with key %d found on %s", key, vm.InventoryPath)
	}

	if err := vm.RemoveDevice(ctx, keepFiles, device); err != nil {
		return "", fmt.Errorf("failed to remove device %d from %s: %w", key, vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"vm": vm.InventoryPath, "device_key": key, "result": "removed"})
}

// requiredDeviceKey reads and validates the "device_key" argument shared by
// vmware_vm_edit_device and vmware_vm_remove_device.
func requiredDeviceKey(args map[string]interface{}) (int32, error) {
	raw, ok := args["device_key"]
	if !ok {
		return 0, fmt.Errorf("device_key is required")
	}
	key, err := toInt32(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid device_key: %w", err)
	}
	return key, nil
}

func handleVMAttachDisk(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, err := resolveVM(ctx, client, args)
	if err != nil {
		return "", err
	}
	id, _ := args["id"].(string)
	if id == "" {
		return "", fmt.Errorf("id is required")
	}
	dsName, _ := args["datastore"].(string)

	ds, err := resolveDatastore(ctx, client, dsName)
	if err != nil {
		return "", err
	}

	var controllerKey int32
	if v, ok := args["controller_key"]; ok {
		controllerKey, err = toInt32(v)
		if err != nil {
			return "", fmt.Errorf("invalid controller_key: %w", err)
		}
	}
	var unitNumber *int32
	if v, ok := args["unit_number"]; ok {
		n, err := toInt32(v)
		if err != nil {
			return "", fmt.Errorf("invalid unit_number: %w", err)
		}
		unitNumber = &n
	}

	if err := vm.AttachDisk(ctx, id, ds, controllerKey, unitNumber); err != nil {
		return "", fmt.Errorf("failed to attach disk %s to %s: %w", id, vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"vm": vm.InventoryPath, "id": id, "result": "attached"})
}

func handleVMDetachDisk(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	vm, err := resolveVM(ctx, client, args)
	if err != nil {
		return "", err
	}
	id, _ := args["id"].(string)
	if id == "" {
		return "", fmt.Errorf("id is required")
	}

	if err := vm.DetachDisk(ctx, id); err != nil {
		return "", fmt.Errorf("failed to detach disk %s from %s: %w", id, vm.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{"vm": vm.InventoryPath, "id": id, "result": "detached"})
}

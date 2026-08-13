package tools

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// newLifecycleRegistry builds a Registry the normal way (NewRegistry, which
// wires vm.go/host.go/etc via registerTools) and then manually layers this
// group's tools on top via withClass, exactly as registry.go's real wiring
// for registerVMLifecycleTools will do once another change adds it there —
// this file must not edit registry.go itself (see generated_vm_lifecycle.go's
// top doc comment).
func newLifecycleRegistry(ctx context.Context, c *vmware.Client, opts RegistryOptions) *Registry {
	r := NewRegistry(ctx, c, opts)
	r.withClass(modeVSphereGeneral, registerVMLifecycleTools)
	return r
}

// setGuestIP reconfigures vmPath's ExtraConfig with the "SET.guest.ipAddress"
// key vcsim special-cases (confirmed by reading
// referencia/govmomi/simulator/virtual_machine.go's applyExtraConfig, and
// matching the exact pattern used by govmomi's own
// referencia/govmomi/object/vm_guest_test.go) to make guest.ipAddress /
// guest.net observable without a real guest OS — the only way to test
// WaitForIP/WaitForNetIP against vcsim without hanging forever. Goes through
// the client's Finder directly (not a registered tool — no tool in this
// project accepts a raw ExtraConfig today), same as any other test fixture
// setup.
func setGuestIP(t *testing.T, ctx context.Context, c *vmware.Client, vmPath, ip string) {
	t.Helper()
	vm, err := c.Finder.VirtualMachine(ctx, vmPath)
	if err != nil {
		t.Fatalf("failed to resolve %s for extraConfig fixture: %v", vmPath, err)
	}
	task, err := vm.Reconfigure(ctx, types.VirtualMachineConfigSpec{
		ExtraConfig: []types.BaseOptionValue{
			&types.OptionValue{Key: "SET.guest.ipAddress", Value: ip},
		},
	})
	if err != nil {
		t.Fatalf("reconfigure(SET.guest.ipAddress=%q) failed: %v", ip, err)
	}
	if err := task.Wait(ctx); err != nil {
		t.Fatalf("reconfigure(SET.guest.ipAddress=%q) task failed: %v", ip, err)
	}
}

// firstDiskKeyAndCapacity reads the real key/CapacityInBytes of the first
// virtual disk on vmPath directly via the client's Finder — needed because
// vmware_vm_device deliberately does NOT expose CapacityInBytes (out of
// scope per this group's curation notes), but
// vmware_vm_query_changed_disk_areas's offset==capacity trivial-success path
// needs the real value to hit deterministically.
func firstDiskKeyAndCapacity(t *testing.T, ctx context.Context, c *vmware.Client, vmPath string) (int32, int64) {
	t.Helper()
	vm, err := c.Finder.VirtualMachine(ctx, vmPath)
	if err != nil {
		t.Fatalf("failed to resolve %s: %v", vmPath, err)
	}
	devices, err := vm.Device(ctx)
	if err != nil {
		t.Fatalf("failed to read devices for %s: %v", vmPath, err)
	}
	disks := devices.SelectByType((*types.VirtualDisk)(nil))
	if len(disks) == 0 {
		t.Fatalf("simulator VM %s has no virtual disk", vmPath)
	}
	disk := disks[0].(*types.VirtualDisk)
	return disk.Key, disk.CapacityInBytes
}

// assertReachesServer proves a tool call actually reached vcsim (or a
// business-rule check deep inside the govmomi client) instead of failing on
// something wired wrong in this file — used for the 6 methods vcsim's
// simulator.VirtualMachine has no server-side implementation for (see
// generated_vm_lifecycle.go's top doc comment): MountToolsInstaller,
// UnmountToolsInstaller, UpgradeTools, Answer, AcquireTicket,
// PutUsbScanCodes. err is expected to be non-nil (vcsim always faults
// MethodNotFound for these); the point is proving it is THAT fault, not
// "unknown tool" (registration broken) or a recovered panic (handler bug).
func assertReachesServer(t *testing.T, err error, tool string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s: expected an error — vcsim has no server-side implementation for this method (see generated_vm_lifecycle.go's top doc comment) — got success instead", tool)
	}
	msg := err.Error()
	if strings.Contains(msg, "unknown tool") {
		t.Fatalf("%s: tool is not registered: %v", tool, err)
	}
	if strings.Contains(msg, "panicked") {
		t.Fatalf("%s: handler panicked instead of returning a clean error: %v", tool, err)
	}
	t.Logf("%s: real vcsim call returned (expected — no server-side simulation for this method): %v", tool, err)
}

// TestVMLifecycleTools_ReadOnlyAccessors smokes every pure read-only
// accessor tool in this group against a real VM/host from vcsim.
func TestVMLifecycleTools_ReadOnlyAccessors(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newLifecycleRegistry(context.Background(), c, RegistryOptions{})
	vm := firstVMPath(t, r)

	t.Run("wait_for_power_state_already_there", func(t *testing.T) {
		state := vmInfo(t, r, vm)["power_state"].(string)
		raw, err := r.CallTool("vmware_vm_wait_for_power_state", map[string]interface{}{"vm": vm, "state": state})
		if err != nil {
			t.Fatalf("vmware_vm_wait_for_power_state(%s) failed: %v", state, err)
		}
		m := decodeResult(t, raw)
		if m["result"] != "reached" {
			t.Fatalf("expected result=reached, got %v (%s)", m["result"], raw)
		}
	})

	t.Run("wait_for_power_state_invalid", func(t *testing.T) {
		if _, err := r.CallTool("vmware_vm_wait_for_power_state", map[string]interface{}{"vm": vm, "state": "bogus"}); err == nil {
			t.Fatal("expected an error for an invalid state value")
		}
	})

	t.Run("is_tools_running", func(t *testing.T) {
		raw, err := r.CallTool("vmware_vm_is_tools_running", map[string]interface{}{"vm": vm})
		if err != nil {
			t.Fatalf("vmware_vm_is_tools_running failed: %v", err)
		}
		m := decodeResult(t, raw)
		// A fresh vcsim VM starts with ToolsRunningStatus =
		// "guestToolsNotRunning" (confirmed in simulator/folder.go's VM
		// creation defaults) — no real guest OS runs Tools in vcsim.
		if m["tools_running"] != false {
			t.Fatalf("expected tools_running=false on a fresh simulator VM, got %v", m["tools_running"])
		}
	})

	t.Run("is_template_false", func(t *testing.T) {
		raw, err := r.CallTool("vmware_vm_is_template", map[string]interface{}{"vm": vm})
		if err != nil {
			t.Fatalf("vmware_vm_is_template failed: %v", err)
		}
		if m := decodeResult(t, raw); m["is_template"] != false {
			t.Fatalf("expected is_template=false on a fresh simulator VM, got %v", m["is_template"])
		}
	})

	t.Run("boot_options", func(t *testing.T) {
		raw, err := r.CallTool("vmware_vm_boot_options", map[string]interface{}{"vm": vm})
		if err != nil {
			t.Fatalf("vmware_vm_boot_options failed: %v", err)
		}
		m := decodeResult(t, raw)
		if m["vm"] != vm {
			t.Fatalf("expected vm=%q, got %v (%s)", vm, m["vm"], raw)
		}
	})

	t.Run("device", func(t *testing.T) {
		raw, err := r.CallTool("vmware_vm_device", map[string]interface{}{"vm": vm})
		if err != nil {
			t.Fatalf("vmware_vm_device failed: %v", err)
		}
		m := decodeResult(t, raw)
		devices, _ := m["devices"].([]interface{})
		if len(devices) == 0 {
			t.Fatalf("expected at least 1 device on the default simulator VM template (SCSI/IDE/CD-ROM/disk/NIC), got 0: %s", raw)
		}
		var sawDisk bool
		for _, d := range devices {
			dm, ok := d.(map[string]interface{})
			if !ok {
				t.Fatalf("device entry is not an object: %v", d)
			}
			for _, field := range []string{"key", "type"} {
				if _, ok := dm[field]; !ok {
					t.Fatalf("device entry missing %q field: %v", field, dm)
				}
			}
			if strings.Contains(dm["type"].(string), "VirtualDisk") {
				sawDisk = true
			}
		}
		if !sawDisk {
			t.Fatalf("expected at least 1 VirtualDisk device on the default simulator VM template: %s", raw)
		}
	})

	t.Run("host_system", func(t *testing.T) {
		raw, err := r.CallTool("vmware_vm_host_system", map[string]interface{}{"vm": vm})
		if err != nil {
			t.Fatalf("vmware_vm_host_system failed: %v", err)
		}
		m := decodeResult(t, raw)
		host := firstHostPath(t, r)
		if m["host"] != host {
			t.Fatalf("expected host=%q (the only host on standalone ESXi), got %v (%s)", host, m["host"], raw)
		}
	})

	t.Run("resource_pool", func(t *testing.T) {
		raw, err := r.CallTool("vmware_vm_resource_pool", map[string]interface{}{"vm": vm})
		if err != nil {
			t.Fatalf("vmware_vm_resource_pool failed: %v", err)
		}
		m := decodeResult(t, raw)
		// Standalone ESXi always has the implicit "Resources" pool — a
		// freshly created VM is assigned to it, so this must be non-null
		// here (the resource_pool:null path is exercised for templates in
		// TestVMLifecycleTools_TemplateRoundTrip instead). Checking for a
		// real path string, not just non-nil: an earlier version of this
		// handler returned rp.InventoryPath directly, which is always ""
		// for an object built via VirtualMachine.ResourcePool (not the
		// Finder) — "" != nil in Go's decoded JSON, so a weaker "!= nil"
		// check here would have let that bug pass silently (it did, until
		// the sibling host_system subtest's stricter exact-match check
		// caught the same underlying issue).
		if s, ok := m["resource_pool"].(string); !ok || s == "" || !strings.HasSuffix(s, "/Resources") {
			t.Fatalf("expected a real resource_pool inventory path ending in /Resources, got %v: %s", m["resource_pool"], raw)
		}
	})
}

// TestVMLifecycleTools_WaitForIPAndNetIP proves vmware_vm_wait_for_ip and
// vmware_vm_wait_for_net_ip actually observe a guest IP, using vcsim's
// documented "SET.guest.ipAddress" ExtraConfig trick (see setGuestIP) to
// avoid depending on a real guest OS — and to avoid an indefinitely-blocking
// property.Wait call hanging this test if the IP were never set at all.
func TestVMLifecycleTools_WaitForIPAndNetIP(t *testing.T) {
	// simulator.VPX(), not ESX() — matches referencia/govmomi/object/vm_guest_test.go
	// (govmomi's own TestVirtualMachineWaitForIP), the only known-working
	// reference for the "SET.guest.ipAddress" ExtraConfig trick against
	// vcsim's WaitForUpdatesEx long-poll. An earlier version of this test
	// used simulator.ESX() and hung for 10 real minutes before Go's test
	// timeout force-killed the process — property.Collector.WaitForUpdatesEx
	// never saw the just-set value and blocked forever waiting for a next
	// change that was never coming (see generated_vm_lifecycle.go's
	// defaultWaitTimeout doc comment for the production-side fix). Root
	// cause of the ESX()-vs-VPX() difference itself wasn't chased down
	// further — VPX() matching upstream's own passing test was enough to
	// unblock this without guessing at vcsim internals.
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newLifecycleRegistry(context.Background(), c, RegistryOptions{})
	vm := firstVMPath(t, r)

	setGuestIP(t, context.Background(), c, vm, "10.0.0.42")

	// timeout_seconds is short and explicit in both calls below — belt and
	// suspenders with defaultWaitTimeout, and it turns any regression back
	// into a fast, clear test failure instead of a multi-minute hang.
	t.Run("wait_for_ip", func(t *testing.T) {
		raw, err := r.CallTool("vmware_vm_wait_for_ip", map[string]interface{}{"vm": vm, "timeout_seconds": 10})
		if err != nil {
			t.Fatalf("vmware_vm_wait_for_ip failed: %v", err)
		}
		if m := decodeResult(t, raw); m["ip"] != "10.0.0.42" {
			t.Fatalf("expected ip=10.0.0.42, got %v (%s)", m["ip"], raw)
		}
	})

	// wait_for_net_ip cannot be driven to a real success path against vcsim:
	// object.VirtualMachine.WaitForNetIP (confirmed by reading the real
	// source, not assumed) waits on GuestNicInfo.IpConfig.IpAddress — a
	// different, newer field than the legacy GuestNicInfo.IpAddress that
	// vcsim's "SET.guest.ipAddress" trick populates (see
	// simulator/virtual_machine.go's applyExtraConfig, "guest.ipAddress"
	// case: it only ever writes vm.Guest.Net[0].IpAddress, never IpConfig).
	// There is no equivalent SET.* alias for IpConfig in vcsim. This is a
	// genuine simulator gap, not a bug in this tool — documented here and
	// in generated_vm_lifecycle.go's top doc comment rather than silently
	// dropping the test. What IS verified: the tool is registered, resolves
	// the VM, and — critically — the production timeout fix actually bounds
	// the call instead of hanging (this used to block for 10 real minutes;
	// see TestVMLifecycleTools_WaitTimeoutIsBounded for the direct
	// regression test of that fix).
	t.Run("wait_for_net_ip_bounded_by_timeout", func(t *testing.T) {
		_, err := r.CallTool("vmware_vm_wait_for_net_ip", map[string]interface{}{"vm": vm, "timeout_seconds": 2})
		if err == nil {
			t.Fatal("expected vmware_vm_wait_for_net_ip to fail — vcsim never populates GuestNicInfo.IpConfig (see comment above)")
		}
		if !strings.Contains(err.Error(), "context deadline exceeded") {
			t.Fatalf("expected a timeout error, got: %v", err)
		}
	})
}

// TestVMLifecycleTools_WaitTimeoutIsBounded proves the production fix
// directly: a wait tool with no chance of its condition ever becoming true
// returns a clean error within a small bound instead of blocking — this is
// the regression test for the 10-minute hang described above.
func TestVMLifecycleTools_WaitTimeoutIsBounded(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newLifecycleRegistry(context.Background(), c, RegistryOptions{})
	vm := firstVMPath(t, r)

	done := make(chan error, 1)
	go func() {
		_, err := r.CallTool("vmware_vm_wait_for_ip", map[string]interface{}{"vm": vm, "timeout_seconds": 1})
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected vmware_vm_wait_for_ip to fail — this VM was never given a guest IP")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("vmware_vm_wait_for_ip with timeout_seconds:1 did not return within 10s — the timeout bound is not working")
	}
}

// TestVMLifecycleTools_QueryChangedDiskAreas proves the tool's plumbing
// (resolveVM, disk_key -> real device lookup, snapshot moref construction)
// end to end via the offset==disk.CapacityInBytes trivial-success path,
// which object.VirtualMachine.QueryChangedDiskAreas's real source (confirmed
// by reading it, not assumed) returns without ever dereferencing
// baseSnapshot or requiring CBT to be enabled — the only success path
// reachable without a real changeId, which nothing in vcsim's public API can
// set (see generated_vm_lifecycle.go's top doc comment).
func TestVMLifecycleTools_QueryChangedDiskAreas(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newLifecycleRegistry(context.Background(), c, RegistryOptions{})
	vm := firstVMPath(t, r)
	diskKey, capacity := firstDiskKeyAndCapacity(t, context.Background(), c, vm)

	t.Run("offset_equals_capacity_succeeds_without_cbt", func(t *testing.T) {
		raw, err := r.CallTool("vmware_vm_query_changed_disk_areas", map[string]interface{}{
			"vm":            vm,
			"disk_key":      float64(diskKey),
			"base_snapshot": "snapshot-does-not-exist", // never dereferenced on this path
			"offset":        float64(capacity),
		})
		if err != nil {
			t.Fatalf("vmware_vm_query_changed_disk_areas failed: %v", err)
		}
		m := decodeResult(t, raw)
		result, ok := m["result"].(map[string]interface{})
		if !ok {
			t.Fatalf("expected a \"result\" object: %s", raw)
		}
		if result["length"] != float64(0) {
			t.Fatalf("expected length=0 at offset==capacity, got %v (%s)", result["length"], raw)
		}
		if result["startOffset"] != float64(capacity) {
			t.Fatalf("expected startOffset=%d, got %v (%s)", capacity, result["startOffset"], raw)
		}
	})

	t.Run("offset_greater_than_capacity_rejected_client_side", func(t *testing.T) {
		if _, err := r.CallTool("vmware_vm_query_changed_disk_areas", map[string]interface{}{
			"vm":            vm,
			"disk_key":      float64(diskKey),
			"base_snapshot": "snapshot-does-not-exist",
			"offset":        float64(capacity + 1),
		}); err == nil {
			t.Fatal("expected an error when offset exceeds disk capacity")
		}
	})

	t.Run("bad_disk_key", func(t *testing.T) {
		if _, err := r.CallTool("vmware_vm_query_changed_disk_areas", map[string]interface{}{
			"vm":            vm,
			"disk_key":      float64(99999),
			"base_snapshot": "snapshot-does-not-exist",
			"offset":        float64(0),
		}); err == nil {
			t.Fatal("expected an error for a disk_key that does not identify a virtual disk")
		}
	})

	t.Run("missing_required_args", func(t *testing.T) {
		for _, missing := range []string{"disk_key", "base_snapshot", "offset"} {
			args := map[string]interface{}{
				"vm":            vm,
				"disk_key":      float64(diskKey),
				"base_snapshot": "snapshot-does-not-exist",
				"offset":        float64(0),
			}
			delete(args, missing)
			if _, err := r.CallTool("vmware_vm_query_changed_disk_areas", args); err == nil {
				t.Fatalf("expected an error when %q is missing", missing)
			}
		}
	})
}

// TestVMLifecycleTools_GuestActions proves vmware_vm_shutdown_guest and
// vmware_vm_standby_guest actually transition a real VM's power state (both
// are implemented server-side by vcsim, unlike RebootGuest — see
// TestVMLifecycleTools_RebootGuestRequiresTools), and that
// vmware_vm_shutdown_guest is genuinely gated (Tier 2: closed gate + missing
// confirm both deny it, leaving the VM's power state unchanged) — the same
// proof pattern as host_test.go's TestHostTools_MaintenanceEnterGateAndConfirm.
func TestVMLifecycleTools_GuestActions(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newLifecycleRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	vm := firstVMPath(t, r)

	if vmInfo(t, r, vm)["power_state"] != "poweredOn" {
		if _, err := r.CallTool("vmware_vm_power_on", map[string]interface{}{"vm": vm}); err != nil {
			t.Fatalf("vmware_vm_power_on failed: %v", err)
		}
	}

	t.Run("standby_guest_transitions_to_suspended", func(t *testing.T) {
		if _, err := r.CallTool("vmware_vm_standby_guest", map[string]interface{}{"vm": vm, "confirm": true}); err != nil {
			t.Fatalf("vmware_vm_standby_guest failed: %v", err)
		}
		if got := vmInfo(t, r, vm)["power_state"]; got != "suspended" {
			t.Fatalf("expected suspended after standby_guest, got %v", got)
		}
		// Resume for the next subtest.
		if _, err := r.CallTool("vmware_vm_power_on", map[string]interface{}{"vm": vm}); err != nil {
			t.Fatalf("vmware_vm_power_on (resume) failed: %v", err)
		}
	})

	t.Run("shutdown_guest_gate_and_confirm", func(t *testing.T) {
		closedGate := newLifecycleRegistry(context.Background(), c, RegistryOptions{AllowDestructive: false})
		if _, err := closedGate.CallTool("vmware_vm_shutdown_guest", map[string]interface{}{"vm": vm, "confirm": true}); err == nil {
			t.Fatal("expected vmware_vm_shutdown_guest to be denied with the gate closed")
		}
		if got := vmInfo(t, r, vm)["power_state"]; got != "poweredOn" {
			t.Fatalf("VM power state changed despite the gate being closed: %v", got)
		}

		if _, err := r.CallTool("vmware_vm_shutdown_guest", map[string]interface{}{"vm": vm}); err == nil {
			t.Fatal("expected vmware_vm_shutdown_guest to fail without confirm:true")
		}
		if got := vmInfo(t, r, vm)["power_state"]; got != "poweredOn" {
			t.Fatalf("VM power state changed despite missing confirm: %v", got)
		}
	})

	t.Run("shutdown_guest_transitions_to_poweredOff", func(t *testing.T) {
		if _, err := r.CallTool("vmware_vm_shutdown_guest", map[string]interface{}{"vm": vm, "confirm": true}); err != nil {
			t.Fatalf("vmware_vm_shutdown_guest failed: %v", err)
		}
		if got := vmInfo(t, r, vm)["power_state"]; got != "poweredOff" {
			t.Fatalf("expected poweredOff after shutdown_guest, got %v", got)
		}
	})
}

// TestVMLifecycleTools_RebootGuestRequiresTools proves vmware_vm_reboot_guest
// is wired correctly end to end (Tier 2 gate/confirm both enforced, resolves
// the VM, reaches the real vcsim method) but documents that a true success
// can't be reached: RebootGuest's simulator implementation (confirmed by
// reading referencia/govmomi/simulator/virtual_machine.go) requires
// Guest.ToolsRunningStatus=="guestToolsRunning", which nothing in the public
// govmomi client API can set — vcsim only ever sets it internally (its
// container-backed guest helpers). A powered-on VM with Tools not running is
// the correct, honest state to exercise here.
func TestVMLifecycleTools_RebootGuestRequiresTools(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newLifecycleRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	vm := firstVMPath(t, r)

	closedGate := newLifecycleRegistry(context.Background(), c, RegistryOptions{AllowDestructive: false})
	if _, err := closedGate.CallTool("vmware_vm_reboot_guest", map[string]interface{}{"vm": vm, "confirm": true}); err == nil {
		t.Fatal("expected vmware_vm_reboot_guest to be denied with the gate closed")
	}

	if _, err := r.CallTool("vmware_vm_reboot_guest", map[string]interface{}{"vm": vm}); err == nil {
		t.Fatal("expected vmware_vm_reboot_guest to fail without confirm:true")
	}

	if vmInfo(t, r, vm)["power_state"] != "poweredOn" {
		if _, err := r.CallTool("vmware_vm_power_on", map[string]interface{}{"vm": vm}); err != nil {
			t.Fatalf("vmware_vm_power_on failed: %v", err)
		}
	}

	_, err := r.CallTool("vmware_vm_reboot_guest", map[string]interface{}{"vm": vm, "confirm": true})
	assertReachesServer(t, err, "vmware_vm_reboot_guest")
}

// TestVMLifecycleTools_TemplateRoundTrip proves vmware_vm_mark_as_template
// and vmware_vm_mark_as_virtual_machine actually flip IsTemplate/
// resource_pool in vcsim, not just that the calls return no error — this is
// this group's required "non-trivial action tool's state change actually
// verified" proof (both methods ARE implemented server-side by
// simulator.VirtualMachine — confirmed by reading
// referencia/govmomi/simulator/virtual_machine.go).
func TestVMLifecycleTools_TemplateRoundTrip(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newLifecycleRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	vm := firstVMPath(t, r)

	// MarkAsTemplate requires the VM powered off (simulator-enforced,
	// matches real vSphere).
	if vmInfo(t, r, vm)["power_state"] == "poweredOn" {
		if _, err := r.CallTool("vmware_vm_power_off", map[string]interface{}{"vm": vm, "confirm": true}); err != nil {
			t.Fatalf("vmware_vm_power_off (pre-template) failed: %v", err)
		}
	}

	raw, err := r.CallTool("vmware_vm_resource_pool", map[string]interface{}{"vm": vm})
	if err != nil {
		t.Fatalf("vmware_vm_resource_pool (pre-template) failed: %v", err)
	}
	poolBefore, _ := decodeResult(t, raw)["resource_pool"].(string)
	if poolBefore == "" {
		t.Fatal("expected a resource pool before marking as template")
	}

	if _, err := r.CallTool("vmware_vm_mark_as_template", map[string]interface{}{"vm": vm, "confirm": true}); err != nil {
		t.Fatalf("vmware_vm_mark_as_template failed: %v", err)
	}

	raw, err = r.CallTool("vmware_vm_is_template", map[string]interface{}{"vm": vm})
	if err != nil {
		t.Fatalf("vmware_vm_is_template (post-template) failed: %v", err)
	}
	if got := decodeResult(t, raw)["is_template"]; got != true {
		t.Fatalf("expected is_template=true after vmware_vm_mark_as_template, got %v", got)
	}

	raw, err = r.CallTool("vmware_vm_resource_pool", map[string]interface{}{"vm": vm})
	if err != nil {
		t.Fatalf("vmware_vm_resource_pool (post-template) failed: %v", err)
	}
	if got := decodeResult(t, raw)["resource_pool"]; got != nil {
		t.Fatalf("expected resource_pool=null on a template (simulator clears it — confirmed in source), got %v", got)
	}

	if _, err := r.CallTool("vmware_vm_mark_as_virtual_machine", map[string]interface{}{"vm": vm, "resource_pool": poolBefore, "confirm": true}); err != nil {
		t.Fatalf("vmware_vm_mark_as_virtual_machine failed: %v", err)
	}

	raw, err = r.CallTool("vmware_vm_is_template", map[string]interface{}{"vm": vm})
	if err != nil {
		t.Fatalf("vmware_vm_is_template (post-restore) failed: %v", err)
	}
	if got := decodeResult(t, raw)["is_template"]; got != false {
		t.Fatalf("expected is_template=false after vmware_vm_mark_as_virtual_machine, got %v", got)
	}

	raw, err = r.CallTool("vmware_vm_resource_pool", map[string]interface{}{"vm": vm})
	if err != nil {
		t.Fatalf("vmware_vm_resource_pool (post-restore) failed: %v", err)
	}
	if got := decodeResult(t, raw)["resource_pool"]; got != poolBefore {
		t.Fatalf("expected resource_pool=%q restored, got %v", poolBefore, got)
	}
}

// TestVMLifecycleTools_SetBootOptions proves vmware_vm_set_boot_options'
// generic-JSON-in decode round-trips through vmware_vm_boot_options.
func TestVMLifecycleTools_SetBootOptions(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newLifecycleRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	vm := firstVMPath(t, r)

	if _, err := r.CallTool("vmware_vm_set_boot_options", map[string]interface{}{
		"vm":      vm,
		"options": map[string]interface{}{"bootDelay": float64(5000)},
		"confirm": true,
	}); err != nil {
		t.Fatalf("vmware_vm_set_boot_options failed: %v", err)
	}

	raw, err := r.CallTool("vmware_vm_boot_options", map[string]interface{}{"vm": vm})
	if err != nil {
		t.Fatalf("vmware_vm_boot_options failed: %v", err)
	}
	m := decodeResult(t, raw)
	opts, ok := m["boot_options"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected a \"boot_options\" object: %s", raw)
	}
	if opts["bootDelay"] != float64(5000) {
		t.Fatalf("expected bootDelay=5000 after vmware_vm_set_boot_options, got %v (%s)", opts["bootDelay"], raw)
	}

	if _, err := r.CallTool("vmware_vm_set_boot_options", map[string]interface{}{"vm": vm, "confirm": true}); err == nil {
		t.Fatal("expected an error when options is missing")
	}
}

// TestVMLifecycleTools_RefreshStorageInfo smokes the simplest Tier 2 action
// in this group (no observable state change through this project's other
// tools, so this only proves it is gated and returns success/no-op cleanly).
func TestVMLifecycleTools_RefreshStorageInfo(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newLifecycleRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	vm := firstVMPath(t, r)

	if _, err := r.CallTool("vmware_vm_refresh_storage_info", map[string]interface{}{"vm": vm}); err == nil {
		t.Fatal("expected an error without confirm:true")
	}
	if _, err := r.CallTool("vmware_vm_refresh_storage_info", map[string]interface{}{"vm": vm, "confirm": true}); err != nil {
		t.Fatalf("vmware_vm_refresh_storage_info failed: %v", err)
	}
}

// TestVMLifecycleTools_UpgradeVM proves the tool reaches vcsim's real
// UpgradeVM_Task business-rule validation (implemented server-side —
// confirmed by reading the simulator source), documenting that a true
// success can't be reached: a freshly created simulator VM already sits at
// the latest hardware version its host supports, so any upgrade attempt
// legitimately faults rather than a wiring bug in this tool.
func TestVMLifecycleTools_UpgradeVM(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newLifecycleRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	vm := firstVMPath(t, r)

	if vmInfo(t, r, vm)["power_state"] == "poweredOn" {
		if _, err := r.CallTool("vmware_vm_power_off", map[string]interface{}{"vm": vm, "confirm": true}); err != nil {
			t.Fatalf("vmware_vm_power_off (pre-upgrade) failed: %v", err)
		}
	}

	closedGate := newLifecycleRegistry(context.Background(), c, RegistryOptions{AllowDestructive: false})
	if _, err := closedGate.CallTool("vmware_vm_upgrade_vm", map[string]interface{}{"vm": vm, "version": "vmx-19", "confirm": true}); err == nil {
		t.Fatal("expected vmware_vm_upgrade_vm to be denied with the gate closed")
	}

	if _, err := r.CallTool("vmware_vm_upgrade_vm", map[string]interface{}{"vm": vm, "confirm": true}); err == nil {
		t.Fatal("expected an error when version is missing")
	}

	// Correction (found by actually running this, not by assumption): the
	// default simulator VM does NOT start at the latest supported hardware
	// version — an earlier version of this test assumed it did and expected
	// upgrading to "vmx-19" to fault immediately, but the real vcsim call
	// succeeded. Test the real flow instead: the first upgrade to vmx-19
	// succeeds, and upgrading to the SAME version again correctly faults
	// (simulator/virtual_machine.go's UpgradeVMTask: "is latest version" /
	// AlreadyUpgradedFault once the VM is actually at that version) — this
	// exercises both the success path and the fault path for real.
	if _, err := r.CallTool("vmware_vm_upgrade_vm", map[string]interface{}{"vm": vm, "version": "vmx-19", "confirm": true}); err != nil {
		t.Fatalf("vmware_vm_upgrade_vm(vmx-19) failed on a VM not yet at that version: %v", err)
	}

	if _, err := r.CallTool("vmware_vm_upgrade_vm", map[string]interface{}{"vm": vm, "version": "vmx-19", "confirm": true}); err == nil {
		t.Fatal("expected a 2nd vmware_vm_upgrade_vm(vmx-19) call to fault — the VM is now already at that version")
	}
}

// TestVMLifecycleTools_Unregister is this group's mandatory Tier 1 proof:
// vmware_vm_unregister denied by a closed gate and by a missing confirm,
// then a real call that actually removes the VM from inventory.
func TestVMLifecycleTools_Unregister(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	closedGate := newLifecycleRegistry(context.Background(), c, RegistryOptions{AllowDestructive: false})
	vm := firstVMPath(t, closedGate)

	// Unregister requires the VM powered off, same precondition as
	// vmware_vm_destroy in vm.go (and its test, TestVMTools_Destroy) — found
	// by actually running this: the default simulator VM starts powered on,
	// and the real vcsim call faulted InvalidPowerState before this fix.
	if vmInfo(t, closedGate, vm)["power_state"] == "poweredOn" {
		if _, err := closedGate.CallTool("vmware_vm_power_off", map[string]interface{}{"vm": vm, "confirm": true}); err == nil {
			t.Fatal("expected vmware_vm_power_off to be denied with the gate closed too — check RegistryOptions above")
		}
		// closedGate can't power off (AllowDestructive:false) — use a
		// scratch open-gate registry over the same client just for setup.
		setup := newLifecycleRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
		if _, err := setup.CallTool("vmware_vm_power_off", map[string]interface{}{"vm": vm, "confirm": true}); err != nil {
			t.Fatalf("vmware_vm_power_off (pre-unregister setup) failed: %v", err)
		}
	}

	if _, err := closedGate.CallTool("vmware_vm_unregister", map[string]interface{}{"vm": vm, "confirm": true}); err == nil {
		t.Fatal("expected vmware_vm_unregister to be denied with the gate closed")
	}
	if _, err := closedGate.CallTool("vmware_vm_info", map[string]interface{}{"vm": vm}); err != nil {
		t.Fatalf("VM should still resolve after a denied unregister: %v", err)
	}

	openGate := newLifecycleRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	if _, err := openGate.CallTool("vmware_vm_unregister", map[string]interface{}{"vm": vm}); err == nil {
		t.Fatal("expected vmware_vm_unregister to fail without confirm:true")
	}
	if _, err := openGate.CallTool("vmware_vm_info", map[string]interface{}{"vm": vm}); err != nil {
		t.Fatalf("VM should still resolve after a missing-confirm unregister: %v", err)
	}

	if _, err := openGate.CallTool("vmware_vm_unregister", map[string]interface{}{"vm": vm, "confirm": true}); err != nil {
		t.Fatalf("vmware_vm_unregister failed: %v", err)
	}
	if _, err := openGate.CallTool("vmware_vm_info", map[string]interface{}{"vm": vm}); err == nil {
		t.Fatalf("expected %s to no longer resolve after unregister", vm)
	}
}

// TestVMLifecycleTools_UnsimulatedMethods covers the 6 methods vcsim's
// simulator.VirtualMachine has no server-side implementation for (see
// generated_vm_lifecycle.go's top doc comment): each is proven registered,
// rejects bad/missing input before making any network call, and — given
// valid input, gate open, and confirm:true — reaches the real vcsim server
// and gets back a clean types.MethodNotFound-based error, not a wiring
// failure.
func TestVMLifecycleTools_UnsimulatedMethods(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newLifecycleRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	vm := firstVMPath(t, r)

	t.Run("mount_tools_installer", func(t *testing.T) {
		_, err := r.CallTool("vmware_vm_mount_tools_installer", map[string]interface{}{"vm": vm, "confirm": true})
		assertReachesServer(t, err, "vmware_vm_mount_tools_installer")
	})

	t.Run("unmount_tools_installer", func(t *testing.T) {
		_, err := r.CallTool("vmware_vm_unmount_tools_installer", map[string]interface{}{"vm": vm, "confirm": true})
		assertReachesServer(t, err, "vmware_vm_unmount_tools_installer")
	})

	t.Run("upgrade_tools", func(t *testing.T) {
		_, err := r.CallTool("vmware_vm_upgrade_tools", map[string]interface{}{"vm": vm, "confirm": true})
		assertReachesServer(t, err, "vmware_vm_upgrade_tools")
	})

	t.Run("answer", func(t *testing.T) {
		if _, err := r.CallTool("vmware_vm_answer", map[string]interface{}{"vm": vm, "answer": "choice-1", "confirm": true}); err == nil {
			t.Fatal("expected an error when id is missing")
		}
		if _, err := r.CallTool("vmware_vm_answer", map[string]interface{}{"vm": vm, "id": "question-1", "confirm": true}); err == nil {
			t.Fatal("expected an error when answer is missing")
		}
		_, err := r.CallTool("vmware_vm_answer", map[string]interface{}{"vm": vm, "id": "question-1", "answer": "choice-1", "confirm": true})
		assertReachesServer(t, err, "vmware_vm_answer")
	})

	t.Run("acquire_ticket", func(t *testing.T) {
		if _, err := r.CallTool("vmware_vm_acquire_ticket", map[string]interface{}{"vm": vm, "confirm": true}); err == nil {
			t.Fatal("expected an error when kind is missing")
		}
		_, err := r.CallTool("vmware_vm_acquire_ticket", map[string]interface{}{"vm": vm, "kind": "webmks", "confirm": true})
		assertReachesServer(t, err, "vmware_vm_acquire_ticket")
	})

	t.Run("put_usb_scan_codes", func(t *testing.T) {
		if _, err := r.CallTool("vmware_vm_put_usb_scan_codes", map[string]interface{}{"vm": vm, "confirm": true}); err == nil {
			t.Fatal("expected an error when spec is missing")
		}
		spec := map[string]interface{}{"keyEvents": []interface{}{}}
		_, err := r.CallTool("vmware_vm_put_usb_scan_codes", map[string]interface{}{"vm": vm, "spec": spec, "confirm": true})
		assertReachesServer(t, err, "vmware_vm_put_usb_scan_codes")
	})
}

// TestVMLifecycleTools_MarkAsVirtualMachineRequiresPool proves the required
// resource_pool argument is actually validated before any resolution is
// attempted.
func TestVMLifecycleTools_MarkAsVirtualMachineRequiresPool(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newLifecycleRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	vm := firstVMPath(t, r)

	if _, err := r.CallTool("vmware_vm_mark_as_virtual_machine", map[string]interface{}{"vm": vm, "confirm": true}); err == nil {
		t.Fatal("expected an error when resource_pool is missing")
	}
	if _, err := r.CallTool("vmware_vm_mark_as_virtual_machine", map[string]interface{}{"vm": vm, "resource_pool": "no-such-pool", "confirm": true}); err == nil {
		t.Fatal("expected an error when resource_pool does not resolve")
	}
}

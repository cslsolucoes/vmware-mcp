package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// newStorageDrsRegistry builds a Registry the normal way (NewRegistry, which
// wires vm.go/host.go/etc via registerTools) and then manually layers this
// group's tools on top via withClass — same pattern as
// generated_host_storage_test.go's newHostStorageRegistry/
// generated_vm_lifecycle_test.go's newLifecycleRegistry, and for the same
// reason: registry.go itself must not be edited by this file (see
// generated_storage_drs.go's top doc comment).
func newStorageDrsRegistry(ctx context.Context, c *vmware.Client, opts RegistryOptions) *Registry {
	r := NewRegistry(ctx, c, opts)
	r.withClass(modeVSphereGeneral, registerStorageDrsTools)
	return r
}

// storagePodFixture creates a real datastore cluster (StoragePod) against a
// VPX simulator client, moves the first available real datastore into it as
// a child, and returns the pod's resolvable name plus a cleanup-free
// reference — every step is a real object-layer call against real
// simulator/folder.go and simulator/storage_pod.go handlers, not mocked.
//
// Side effect callers must account for: simulator/folder.go's
// MoveIntoFolderTask handler genuinely RE-PARENTS the moved datastore —
// folderRemoveReference removes it from the plain datastore folder, so
// after this call the moved datastore is no longer found by an exact
// single-segment Finder path like "/*/datastore/<name>" (only by a
// recursive/wildcard search that descends into the pod). Confirmed
// empirically, not assumed: an earlier version of this fixture broke
// resolveDatastore-based subtests in TestStorageDrsTools_UnsimulatedMethods
// this way. Callers that also need a datastore OUTSIDE the pod (for
// resolveDatastore-based tools) must build their model with Datastore >= 2
// and pick one whose InventoryPath does not contain the pod's name.
func storagePodFixture(t *testing.T, ctx context.Context, c *vmware.Client) (*object.StoragePod, string) {
	t.Helper()

	dc, err := c.Finder.DefaultDatacenter(ctx)
	if err != nil {
		t.Fatalf("failed to resolve default datacenter: %v", err)
	}
	folders, err := dc.Folders(ctx)
	if err != nil {
		t.Fatalf("failed to read datacenter folders: %v", err)
	}

	pod, err := folders.DatastoreFolder.CreateStoragePod(ctx, "pod-test")
	if err != nil {
		t.Fatalf("failed to create storage pod: %v", err)
	}

	dss, err := c.Finder.DatastoreList(ctx, "*")
	if err != nil || len(dss) == 0 {
		t.Fatalf("failed to list datastores for pod fixture: %v", err)
	}

	task, err := pod.MoveInto(ctx, []types.ManagedObjectReference{dss[0].Reference()})
	if err != nil {
		t.Fatalf("failed to move datastore into pod: %v", err)
	}
	if err := waitForTask(ctx, task); err != nil {
		t.Fatalf("move-into-pod task failed: %v", err)
	}

	return pod, "pod-test"
}

// TestStorageDrsTools_Registration proves all 9 tools are registered and
// reachable via ListTools — a basic wiring smoke test before the more
// specific behavioral tests below.
func TestStorageDrsTools_Registration(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newStorageDrsRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	want := []string{
		"vmware_storage_query_datastore_performance_summary",
		"vmware_storage_query_iorm_config_option",
		"vmware_storage_recommend_datastores",
		"vmware_storage_apply_drs_recommendation",
		"vmware_storage_apply_drs_recommendation_to_pod",
		"vmware_storage_cancel_drs_recommendation",
		"vmware_storage_configure_datastore_iorm",
		"vmware_storage_configure_drs_for_pod",
		"vmware_storage_refresh_drs_recommendation",
	}
	if len(want) != 9 {
		t.Fatalf("test bug: want list has %d entries, expected 9", len(want))
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

// TestStorageDrsTools_ESXiModeSanity proves the mode-correction from this
// file's top doc comment for real: registering these tools
// modeVSphereGeneral (not vcenter-only) means they are reachable against a
// STANDALONE ESXi host at all, and object.NewStorageResourceManager's
// unconditional dereference of c.ServiceContent.StorageResourceManager does
// NOT panic/nil-crash on simulator.ESX() (it would if the earlier,
// incorrect vcenter-only premise had actually reflected a nil field).
// QueryIORMConfigOption itself is one of the 7 methods vcsim never
// implements server-side (see this file's top doc comment) even on VPX, so
// the meaningful assertion here is assertReachesServer, not a clean
// success: a real round trip to vcsim's method dispatch returning
// MethodNotFound proves the manager object resolved fine on ESXi — a
// client-side nil-pointer panic or a wrong-mode "unknown tool" error would
// both be a real regression of the mode fix, and assertReachesServer
// distinguishes those from the expected, harmless MethodNotFound.
func TestStorageDrsTools_ESXiModeSanity(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newStorageDrsRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	host := firstHostPath(t, r)

	_, err := r.CallTool("vmware_storage_query_iorm_config_option", map[string]interface{}{"host": host})
	assertReachesServer(t, err, "vmware_storage_query_iorm_config_option")
}

// TestStorageDrsTools_ConfigureAndRecommend drives the 2 methods vcsim DOES
// implement server-side (ConfigureStorageDrsForPod, RecommendDatastores) to
// a real, non-empty success path against simulator.VPX() — not just a
// registration/reachability check. Storage DRS/datastore clusters need
// vCenter-style modeling (StoragePod lives under a Datacenter's
// DatastoreFolder), so VPX is required here — confirmed empirically:
// simulator.ESX() has no datastore folder structure to attach a StoragePod
// to in the same way.
func TestStorageDrsTools_ConfigureAndRecommend(t *testing.T) {
	ctx := context.Background()
	c, cleanup := newSimClient(t, simulator.VPX())
	defer cleanup()

	r := newStorageDrsRegistry(ctx, c, RegistryOptions{AllowDestructive: true})
	pod, podName := storagePodFixture(t, ctx, c)

	t.Run("configure_storage_drs_for_pod_enable", func(t *testing.T) {
		closedGate := newStorageDrsRegistry(ctx, c, RegistryOptions{})
		if _, err := closedGate.CallTool("vmware_storage_configure_drs_for_pod", map[string]interface{}{
			"pod": podName, "spec": map[string]interface{}{"podConfigSpec": map[string]interface{}{"enabled": true}}, "modify": true, "confirm": true,
		}); err == nil {
			t.Fatal("expected the closed destructive gate to deny configure_drs_for_pod")
		}

		raw, err := r.CallTool("vmware_storage_configure_drs_for_pod", map[string]interface{}{
			"pod": podName,
			"spec": map[string]interface{}{
				"podConfigSpec": map[string]interface{}{
					"enabled":           true,
					"defaultVmBehavior": "automated",
				},
			},
			"modify":  true,
			"confirm": true,
		})
		if err != nil {
			t.Fatalf("vmware_storage_configure_drs_for_pod failed: %v", err)
		}
		m := decodeResult(t, raw)
		if m["result"] != "configured" {
			t.Fatalf("unexpected result: %s", raw)
		}
	})

	t.Run("recommend_datastores_real_recommendations", func(t *testing.T) {
		// Not DefaultResourcePool: the default VPX() model has both a
		// standalone host's implicit "Resources" pool AND the 1 cluster's
		// "Resources" pool, so DefaultResourcePool's "*" resolves to
		// multiple instances (confirmed empirically). RecommendDatastores's
		// own validation only checks ResourcePool != nil — which one is
		// irrelevant here — so just take the first.
		rps, err := c.Finder.ResourcePoolList(ctx, "*")
		if err != nil || len(rps) == 0 {
			t.Fatalf("failed to resolve a resource pool: %v", err)
		}
		rp := rps[0]

		raw, err := r.CallTool("vmware_storage_recommend_datastores", map[string]interface{}{
			"storage_spec": map[string]interface{}{
				"type":         "create",
				"resourcePool": map[string]interface{}{"type": "ResourcePool", "value": rp.Reference().Value},
				"podSelectionSpec": map[string]interface{}{
					"initialVmConfig": []interface{}{
						map[string]interface{}{
							"storagePod": map[string]interface{}{"type": "StoragePod", "value": pod.Reference().Value},
							"disk":       []interface{}{},
						},
					},
				},
				"configSpec": map[string]interface{}{
					"name":  "test-vm",
					"files": map[string]interface{}{"vmPathName": "[dummy] test-vm/test-vm.vmx"},
				},
			},
		})
		if err != nil {
			t.Fatalf("vmware_storage_recommend_datastores failed to produce a real recommendation: %v", err)
		}
		m := decodeResult(t, raw)
		count, ok := m["recommendation_count"].(float64)
		if !ok || count < 1 {
			t.Fatalf("expected at least 1 real recommendation (pod has 1 child datastore), got: %s", raw)
		}
	})

	t.Run("recommend_datastores_invalid_argument_from_server", func(t *testing.T) {
		// Missing podSelectionSpec.storagePod/initialVmConfig entirely and no
		// top-level StoragePod set (cluster resolves to nil) with no
		// configSpec.files either — a real, specific business-rule fault
		// from vcsim's own validation (InvalidArgument on "configSpec"),
		// proving this reaches real server-side logic, not just plumbing.
		_, err := r.CallTool("vmware_storage_recommend_datastores", map[string]interface{}{
			"storage_spec": map[string]interface{}{"type": "create"},
		})
		if err == nil {
			t.Fatal("expected an error: type=create with no resourcePool set must fail server-side validation")
		}
		t.Logf("real vcsim validation error (expected): %v", err)
	})

	t.Run("refresh_drs_recommendation_unsimulated", func(t *testing.T) {
		_, err := r.CallTool("vmware_storage_refresh_drs_recommendation", map[string]interface{}{"pod": podName, "confirm": true})
		assertReachesServer(t, err, "vmware_storage_refresh_drs_recommendation")
	})
}

// TestStorageDrsTools_UnsimulatedMethods covers the 7 methods vcsim's
// simulator has no server-side implementation for (see
// generated_storage_drs.go's top doc comment): each is proven registered,
// rejects bad/missing input before making any network call, and — given
// valid input, gate open, and confirm:true where applicable — reaches the
// real vcsim server and gets back a clean types.MethodNotFound-based error
// via assertReachesServer (generated_vm_lifecycle_test.go, same package),
// not a wiring failure (unknown tool) or a recovered panic.
func TestStorageDrsTools_UnsimulatedMethods(t *testing.T) {
	ctx := context.Background()
	model := simulator.VPX()
	model.Datastore = 2 // leave 1 datastore outside the pod fixture below — see storagePodFixture's doc comment
	c, cleanup := newSimClient(t, model)
	defer cleanup()

	r := newStorageDrsRegistry(ctx, c, RegistryOptions{AllowDestructive: true})
	_, podName := storagePodFixture(t, ctx, c)

	// Real datastore/host names so resolveDatastore/resolveHost/
	// resolveStoragePod succeed and the failure genuinely comes from the
	// unsimulated method, not from argument resolution. Must pick a
	// datastore that is NOT the one storagePodFixture just moved into the
	// pod (see its doc comment) — resolveDatastore's exact-path lookup
	// would not find that one anymore.
	dss, err := c.Finder.DatastoreList(ctx, "*")
	if err != nil || len(dss) == 0 {
		t.Fatalf("failed to list datastores: %v", err)
	}
	var dsName string
	for _, ds := range dss {
		if !strings.Contains(ds.InventoryPath, "/"+podName+"/") {
			dsName = ds.Name()
			break
		}
	}
	if dsName == "" {
		t.Fatalf("no datastore left outside the pod fixture to test against: %v", dss)
	}
	host := firstHostPath(t, r)

	t.Run("query_datastore_performance_summary", func(t *testing.T) {
		if _, err := r.CallTool("vmware_storage_query_datastore_performance_summary", map[string]interface{}{"datastore": "does-not-exist"}); err == nil {
			t.Fatal("expected an error for an unresolvable datastore")
		}
		_, err := r.CallTool("vmware_storage_query_datastore_performance_summary", map[string]interface{}{"datastore": dsName})
		assertReachesServer(t, err, "vmware_storage_query_datastore_performance_summary")
	})

	t.Run("apply_drs_recommendation", func(t *testing.T) {
		closedGate := newStorageDrsRegistry(ctx, c, RegistryOptions{})
		if _, err := closedGate.CallTool("vmware_storage_apply_drs_recommendation", map[string]interface{}{"key": []interface{}{"fake-key"}, "confirm": true}); err == nil {
			t.Fatal("expected the closed destructive gate to deny apply_drs_recommendation")
		}
		if _, err := r.CallTool("vmware_storage_apply_drs_recommendation", map[string]interface{}{"key": []interface{}{"fake-key"}}); err == nil {
			t.Fatal("expected an error without confirm:true")
		}
		if _, err := r.CallTool("vmware_storage_apply_drs_recommendation", map[string]interface{}{"key": []interface{}{}, "confirm": true}); err == nil {
			t.Fatal("expected an error when key is empty")
		}
		_, err := r.CallTool("vmware_storage_apply_drs_recommendation", map[string]interface{}{"key": []interface{}{"fake-key"}, "confirm": true})
		assertReachesServer(t, err, "vmware_storage_apply_drs_recommendation")
	})

	t.Run("apply_drs_recommendation_to_pod", func(t *testing.T) {
		if _, err := r.CallTool("vmware_storage_apply_drs_recommendation_to_pod", map[string]interface{}{"pod": podName, "confirm": true}); err == nil {
			t.Fatal("expected an error when key is missing")
		}
		_, err := r.CallTool("vmware_storage_apply_drs_recommendation_to_pod", map[string]interface{}{"pod": podName, "key": "fake-key", "confirm": true})
		assertReachesServer(t, err, "vmware_storage_apply_drs_recommendation_to_pod")
	})

	t.Run("cancel_drs_recommendation", func(t *testing.T) {
		closedGate := newStorageDrsRegistry(ctx, c, RegistryOptions{})
		if _, err := closedGate.CallTool("vmware_storage_cancel_drs_recommendation", map[string]interface{}{"key": []interface{}{"fake-key"}, "confirm": true}); err == nil {
			t.Fatal("expected the closed destructive gate to deny cancel_drs_recommendation")
		}
		_, err := r.CallTool("vmware_storage_cancel_drs_recommendation", map[string]interface{}{"key": []interface{}{"fake-key"}, "confirm": true})
		assertReachesServer(t, err, "vmware_storage_cancel_drs_recommendation")
	})

	t.Run("configure_datastore_iorm", func(t *testing.T) {
		if _, err := r.CallTool("vmware_storage_configure_datastore_iorm", map[string]interface{}{"datastore": dsName, "confirm": true}); err == nil {
			t.Fatal("expected an error when spec is missing")
		}
		_, err := r.CallTool("vmware_storage_configure_datastore_iorm", map[string]interface{}{
			"datastore": dsName,
			"spec":      map[string]interface{}{"enabled": true},
			"confirm":   true,
		})
		assertReachesServer(t, err, "vmware_storage_configure_datastore_iorm")
	})

	t.Run("query_iorm_config_option_bad_host", func(t *testing.T) {
		if _, err := r.CallTool("vmware_storage_query_iorm_config_option", map[string]interface{}{"host": "does-not-exist"}); err == nil {
			t.Fatal("expected an error for an unresolvable host")
		}
		// Re-confirms assertReachesServer against VPX (TestStorageDrsTools_ESXiModeSanity
		// already covers this same tool against standalone ESXi) — a valid
		// host still reaches vcsim's real MethodNotFound dispatch.
		_, err := r.CallTool("vmware_storage_query_iorm_config_option", map[string]interface{}{"host": host})
		assertReachesServer(t, err, "vmware_storage_query_iorm_config_option")
	})
}

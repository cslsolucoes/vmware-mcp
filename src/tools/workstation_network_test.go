package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cslsoftwares/mcpvmware/workstation"
)

// wsNetworkFixture starts a fake vmrest server covering the 17 routes this
// file's Group B owns (VM Shared Folders + VM Network Adapters + Host
// Networks Management) — there is no vmrest simulator equivalent to vcsim,
// so this is a fixture unit test, matching appliance_test.go's approach for
// the other domain in this project with no simulator coverage.
func wsNetworkFixture(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	writeJSON := func(w http.ResponseWriter, v interface{}) {
		w.Header().Set("Content-Type", "application/vnd.vmware.vmw.rest-v1+json")
		_ = json.NewEncoder(w).Encode(v)
	}

	// VM Shared Folders Management
	mux.HandleFunc("/api/vms/vm-1/sharedfolders", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, []map[string]interface{}{{"folder_id": "demo", "host_path": `C:\SharedFolders\demo`, "flags": 4}})
		case http.MethodPost:
			var body wsSharedFolderCreateBody
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body.FolderID != "demo" || body.HostPath == "" || body.Flags != 4 {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusCreated)
			writeJSON(w, map[string]interface{}{"folder_id": body.FolderID, "host_path": body.HostPath, "flags": body.Flags})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/vms/vm-1/sharedfolders/demo", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			var body wsSharedFolderUpdateBody
			_ = json.NewDecoder(r.Body).Decode(&body)
			writeJSON(w, map[string]interface{}{"folder_id": "demo", "host_path": body.HostPath, "flags": body.Flags})
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	// VM Network Adapters Management
	mux.HandleFunc("/api/vms/vm-1/ip", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{"ip": "192.168.130.10"})
	})
	mux.HandleFunc("/api/vms/vm-1/nic", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, map[string]interface{}{"nics": []map[string]interface{}{{"index": 0, "type": "nat"}}, "num": 1})
		case http.MethodPost:
			var body wsNicBody
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body.Type != "custom" || body.Vmnet != "vmnet8" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusCreated)
			writeJSON(w, map[string]interface{}{"index": 1, "type": body.Type, "vmnet": body.Vmnet})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/vms/vm-1/nic/1", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			var body wsNicBody
			_ = json.NewDecoder(r.Body).Decode(&body)
			writeJSON(w, map[string]interface{}{"index": 1, "type": body.Type, "vmnet": body.Vmnet})
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/vms/vm-1/nicips", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{"nics": []map[string]interface{}{{"mac": "00:0c:29:12:34:56", "ips": []string{"192.168.130.10"}}}})
	})

	// Host Networks Management
	mux.HandleFunc("/api/vmnet", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{"vmnets": []map[string]interface{}{{"name": "vmnet8", "type": "nat"}}, "num": 1})
	})
	mux.HandleFunc("/api/vmnets", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var body wsVmnetCreateBody
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Name == "" || body.Type == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusCreated)
		writeJSON(w, map[string]interface{}{"name": body.Name, "type": body.Type})
	})
	mux.HandleFunc("/api/vmnet/vmnet8/mactoip", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{"mactoips": []map[string]interface{}{{"mac": "00:0c:29:12:34:56", "IP": "192.168.130.10"}}})
	})
	mux.HandleFunc("/api/vmnet/vmnet8/mactoip/00:0c:29:12:34:56", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var body wsMacToIPBody
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.IP == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]interface{}{"mac": "00:0c:29:12:34:56", "IP": body.IP})
	})
	mux.HandleFunc("/api/vmnet/vmnet8/portforward", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{"port_forwardings": []map[string]interface{}{{"protocol": "tcp", "port": 8080, "guestIp": "192.168.130.10", "guestPort": 80}}})
	})
	mux.HandleFunc("/api/vmnet/vmnet8/portforward/tcp/8080", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			var body wsPortForwardBody
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body.GuestIP == "" || body.GuestPort == 0 {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(w, map[string]interface{}{"protocol": "tcp", "port": 8080, "guestIp": body.GuestIP, "guestPort": body.GuestPort, "desc": body.Desc})
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	return httptest.NewServer(mux)
}

// newWSNetworkTestRegistry builds a Registry wired to a *workstation.Client
// pointed at srv, with client set to nil (the Fase 9 Workstation-only
// connection mode — see registry.go's NewRegistry doc comment: client may be
// nil when opts.ConnectionMode/WorkstationClient means no vSphere session
// exists) and destructive tools gated open so the happy-path tests can reach
// the real handlers.
func newWSNetworkTestRegistry(t *testing.T, srv *httptest.Server, allowDestructive bool) *Registry {
	t.Helper()
	wsClient, err := workstation.NewClient(workstation.Config{URL: srv.URL + "/api", Username: "u", Password: "p"})
	if err != nil {
		t.Fatalf("workstation.NewClient: %v", err)
	}
	return NewRegistry(context.Background(), nil, RegistryOptions{
		WorkstationClient: wsClient,
		AllowDestructive:  allowDestructive,
	})
}

// --- VM Shared Folders Management -------------------------------------------

func TestWorkstationNetwork_SharedFolderHappyPath(t *testing.T) {
	srv := wsNetworkFixture(t)
	defer srv.Close()
	r := newWSNetworkTestRegistry(t, srv, true)

	raw, err := r.CallTool("vmware_workstation_shared_folder_list", map[string]interface{}{"id": "vm-1"})
	if err != nil {
		t.Fatalf("shared_folder_list failed: %v", err)
	}
	var list []map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &list); err != nil || len(list) != 1 || list[0]["folder_id"] != "demo" {
		t.Fatalf("unexpected shared_folder_list result: %s (err=%v)", raw, err)
	}

	if _, err := r.CallTool("vmware_workstation_shared_folder_create", map[string]interface{}{
		"id": "vm-1", "folder_id": "demo", "host_path": `C:\SharedFolders\demo`, "flags": 4, "confirm": true,
	}); err != nil {
		t.Fatalf("shared_folder_create failed: %v", err)
	}

	if _, err := r.CallTool("vmware_workstation_shared_folder_update", map[string]interface{}{
		"id": "vm-1", "folder_id": "demo", "host_path": `C:\SharedFolders\demo2`, "flags": 0, "confirm": true,
	}); err != nil {
		t.Fatalf("shared_folder_update failed: %v", err)
	}

	if _, err := r.CallTool("vmware_workstation_shared_folder_delete", map[string]interface{}{
		"id": "vm-1", "folder_id": "demo", "confirm": true,
	}); err != nil {
		t.Fatalf("shared_folder_delete failed: %v", err)
	}
}

func TestWorkstationNetwork_SharedFolderRequiresArgs(t *testing.T) {
	srv := wsNetworkFixture(t)
	defer srv.Close()
	r := newWSNetworkTestRegistry(t, srv, true)

	if _, err := r.CallTool("vmware_workstation_shared_folder_list", map[string]interface{}{}); err == nil {
		t.Fatal("expected an error for missing id")
	}
	if _, err := r.CallTool("vmware_workstation_shared_folder_create", map[string]interface{}{"id": "vm-1", "confirm": true}); err == nil {
		t.Fatal("expected an error for missing folder_id/host_path/flags")
	}
}

// --- VM Network Adapters Management -----------------------------------------

func TestWorkstationNetwork_NicHappyPath(t *testing.T) {
	srv := wsNetworkFixture(t)
	defer srv.Close()
	r := newWSNetworkTestRegistry(t, srv, true)

	if raw, err := r.CallTool("vmware_workstation_nic_ip", map[string]interface{}{"id": "vm-1"}); err != nil {
		t.Fatalf("nic_ip failed: %v", err)
	} else if m := decodeResult(t, raw); m["ip"] != "192.168.130.10" {
		t.Fatalf("unexpected nic_ip result: %s", raw)
	}

	if _, err := r.CallTool("vmware_workstation_nic_list", map[string]interface{}{"id": "vm-1"}); err != nil {
		t.Fatalf("nic_list failed: %v", err)
	}

	if _, err := r.CallTool("vmware_workstation_nic_create", map[string]interface{}{
		"id": "vm-1", "type": "custom", "vmnet": "vmnet8", "confirm": true,
	}); err != nil {
		t.Fatalf("nic_create failed: %v", err)
	}

	if _, err := r.CallTool("vmware_workstation_nic_update", map[string]interface{}{
		"id": "vm-1", "index": 1, "type": "custom", "vmnet": "vmnet8", "confirm": true,
	}); err != nil {
		t.Fatalf("nic_update failed: %v", err)
	}

	if _, err := r.CallTool("vmware_workstation_nic_delete", map[string]interface{}{
		"id": "vm-1", "index": 1, "confirm": true,
	}); err != nil {
		t.Fatalf("nic_delete failed: %v", err)
	}

	if _, err := r.CallTool("vmware_workstation_vm_nic_ips", map[string]interface{}{"id": "vm-1"}); err != nil {
		t.Fatalf("vm_nic_ips failed: %v", err)
	}
}

func TestWorkstationNetwork_NicRequiresArgs(t *testing.T) {
	srv := wsNetworkFixture(t)
	defer srv.Close()
	r := newWSNetworkTestRegistry(t, srv, true)

	if _, err := r.CallTool("vmware_workstation_nic_update", map[string]interface{}{"id": "vm-1", "confirm": true}); err == nil {
		t.Fatal("expected an error for missing index/type")
	}
	if _, err := r.CallTool("vmware_workstation_nic_delete", map[string]interface{}{"id": "vm-1", "confirm": true}); err == nil {
		t.Fatal("expected an error for missing index")
	}
}

// --- Host Networks Management -----------------------------------------------

func TestWorkstationNetwork_HostNetworkHappyPath(t *testing.T) {
	srv := wsNetworkFixture(t)
	defer srv.Close()
	r := newWSNetworkTestRegistry(t, srv, true)

	if _, err := r.CallTool("vmware_workstation_vmnet_list", map[string]interface{}{}); err != nil {
		t.Fatalf("vmnet_list failed: %v", err)
	}

	if _, err := r.CallTool("vmware_workstation_vmnet_create", map[string]interface{}{
		"name": "vmnet5", "type": "nat", "confirm": true,
	}); err != nil {
		t.Fatalf("vmnet_create failed: %v", err)
	}

	if _, err := r.CallTool("vmware_workstation_vmnet_mactoip_list", map[string]interface{}{"vmnet": "vmnet8"}); err != nil {
		t.Fatalf("vmnet_mactoip_list failed: %v", err)
	}

	if _, err := r.CallTool("vmware_workstation_vmnet_mactoip_set", map[string]interface{}{
		"vmnet": "vmnet8", "mac": "00:0c:29:12:34:56", "ip": "192.168.130.10", "confirm": true,
	}); err != nil {
		t.Fatalf("vmnet_mactoip_set failed: %v", err)
	}

	if _, err := r.CallTool("vmware_workstation_vmnet_portforward_list", map[string]interface{}{"vmnet": "vmnet8"}); err != nil {
		t.Fatalf("vmnet_portforward_list failed: %v", err)
	}

	if _, err := r.CallTool("vmware_workstation_vmnet_portforward_set", map[string]interface{}{
		"vmnet": "vmnet8", "protocol": "tcp", "port": 8080, "guest_ip": "192.168.130.10", "guest_port": 80, "confirm": true,
	}); err != nil {
		t.Fatalf("vmnet_portforward_set failed: %v", err)
	}

	if _, err := r.CallTool("vmware_workstation_vmnet_portforward_delete", map[string]interface{}{
		"vmnet": "vmnet8", "protocol": "tcp", "port": 8080, "confirm": true,
	}); err != nil {
		t.Fatalf("vmnet_portforward_delete failed: %v", err)
	}
}

// TestWorkstationNetwork_PortforwardRequiresArgs proves the required-args
// checks fire for the two portforward mutation tools — protocol/port are
// declared as a JSON-Schema enum/integer for the MCP layer's own validation,
// but the handler itself only checks presence via requiredStringArg/
// wsRequiredInt, so this exercises that layer directly.
func TestWorkstationNetwork_PortforwardRequiresArgs(t *testing.T) {
	srv := wsNetworkFixture(t)
	defer srv.Close()
	r := newWSNetworkTestRegistry(t, srv, true)

	if _, err := r.CallTool("vmware_workstation_vmnet_portforward_set", map[string]interface{}{
		"vmnet": "vmnet8", "protocol": "tcp", "port": 8080, "confirm": true,
	}); err == nil {
		t.Fatal("expected an error for missing guest_ip/guest_port")
	}
	if _, err := r.CallTool("vmware_workstation_vmnet_portforward_delete", map[string]interface{}{
		"vmnet": "vmnet8", "confirm": true,
	}); err == nil {
		t.Fatal("expected an error for missing protocol/port")
	}
}

// --- Tier gate coverage ------------------------------------------------------

func TestWorkstationNetwork_DestructiveGateClosedDenies(t *testing.T) {
	srv := wsNetworkFixture(t)
	defer srv.Close()
	r := newWSNetworkTestRegistry(t, srv, false) // gate closed

	_, err := r.CallTool("vmware_workstation_shared_folder_delete", map[string]interface{}{
		"id": "vm-1", "folder_id": "demo", "confirm": true,
	})
	if err == nil {
		t.Fatal("expected shared_folder_delete to be denied with the destructive gate closed")
	}

	_, err = r.CallTool("vmware_workstation_vmnet_portforward_delete", map[string]interface{}{
		"vmnet": "vmnet8", "protocol": "tcp", "port": 8080, "confirm": true,
	})
	if err == nil {
		t.Fatal("expected vmnet_portforward_delete to be denied with the destructive gate closed")
	}
}

func TestWorkstationNetwork_DestructiveRequiresConfirm(t *testing.T) {
	srv := wsNetworkFixture(t)
	defer srv.Close()
	r := newWSNetworkTestRegistry(t, srv, true) // gate open, but confirm missing

	_, err := r.CallTool("vmware_workstation_nic_delete", map[string]interface{}{"id": "vm-1", "index": 1})
	if err == nil {
		t.Fatal("expected nic_delete to require confirm:true even with the gate open")
	}
}

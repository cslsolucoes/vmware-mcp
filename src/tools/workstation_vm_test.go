package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/cslsoftwares/mcpvmware/workstation"
)

// wsFixtureVM is one VM's mutable state in the in-memory fixture server —
// there is no vcsim-equivalent simulator for vmrest (see
// workstation/client_test.go and appliance_test.go's fixture-not-simulator
// precedent), so exercising a full create->read->update->delete lifecycle
// needs a small stateful fake server, not just canned single-shot responses.
type wsFixtureVM struct {
	ID         string
	Path       string
	Processors int
	Memory     int
	Params     map[string]string
	PowerState string
}

// wsFixtureState backs wsFullFixture — a stateful fake vmrest server
// covering every route this file's tools call, seeded with one VM
// ("vm-seed-1") so get/update/config-param/restrictions/power/delete all
// have something real to operate on without a prior clone/register call.
type wsFixtureState struct {
	vms    map[string]*wsFixtureVM
	nextID int
}

func newWSFixtureState() *wsFixtureState {
	return &wsFixtureState{
		vms: map[string]*wsFixtureVM{
			"vm-seed-1": {
				ID:         "vm-seed-1",
				Path:       `C:\VMs\Seed\Seed.vmx`,
				Processors: 2,
				Memory:     2048,
				Params:     map[string]string{},
				PowerState: "poweredOff",
			},
		},
	}
}

func (s *wsFixtureState) newID(prefix string) string {
	s.nextID++
	return prefix + "-" + strconv.Itoa(s.nextID)
}

func writeWSJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/vnd.vmware.vmw.rest-v1+json")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

func writeWSError(w http.ResponseWriter, status int, message string) {
	writeWSJSON(w, status, map[string]interface{}{"Code": status, "Message": message})
}

func vmInfoBody(vm *wsFixtureVM) map[string]interface{} {
	return map[string]interface{}{
		"id":     vm.ID,
		"cpu":    map[string]interface{}{"processors": vm.Processors},
		"memory": vm.Memory,
	}
}

// newWSFullFixture starts the stateful fixture server. Every path handled
// here mirrors the real route+method pairs confirmed from the vendored
// Postman collection (see workstation_vm.go's top doc comment) — base path
// "/api" to match how workstation.NewClient's Config.URL is built (see
// workstation/client_test.go's fixture helper).
func newWSFullFixture(t *testing.T) (*httptest.Server, *wsFixtureState) {
	t.Helper()
	state := newWSFixtureState()
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/vms", func(w http.ResponseWriter, r *http.Request) {
		list := make([]map[string]interface{}, 0, len(state.vms))
		for _, vm := range state.vms {
			list = append(list, map[string]interface{}{"id": vm.ID, "path": vm.Path})
		}
		writeWSJSON(w, http.StatusOK, list)
	})

	mux.HandleFunc("POST /api/vms", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Name     string `json:"name"`
			ParentID string `json:"parentId"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		parent, ok := state.vms[body.ParentID]
		if !ok {
			writeWSError(w, http.StatusNotFound, "No such resource")
			return
		}
		id := state.newID("vm-clone")
		clone := &wsFixtureVM{
			ID: id, Path: `C:\VMs\` + body.Name + `\` + body.Name + `.vmx`,
			Processors: parent.Processors, Memory: parent.Memory,
			Params: map[string]string{}, PowerState: "poweredOff",
		}
		state.vms[id] = clone
		writeWSJSON(w, http.StatusCreated, vmInfoBody(clone))
	})

	mux.HandleFunc("GET /api/vms/{id}", func(w http.ResponseWriter, r *http.Request) {
		vm, ok := state.vms[r.PathValue("id")]
		if !ok {
			writeWSError(w, http.StatusNotFound, "No such resource")
			return
		}
		writeWSJSON(w, http.StatusOK, vmInfoBody(vm))
	})

	mux.HandleFunc("PUT /api/vms/{id}", func(w http.ResponseWriter, r *http.Request) {
		vm, ok := state.vms[r.PathValue("id")]
		if !ok {
			writeWSError(w, http.StatusNotFound, "No such resource")
			return
		}
		var body struct {
			Processors *int `json:"processors"`
			Memory     *int `json:"memory"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Processors != nil {
			vm.Processors = *body.Processors
		}
		if body.Memory != nil {
			vm.Memory = *body.Memory
		}
		writeWSJSON(w, http.StatusOK, vmInfoBody(vm))
	})

	mux.HandleFunc("DELETE /api/vms/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if _, ok := state.vms[id]; !ok {
			writeWSError(w, http.StatusNotFound, "No such resource")
			return
		}
		delete(state.vms, id)
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("PUT /api/vms/{id}/configparams", func(w http.ResponseWriter, r *http.Request) {
		vm, ok := state.vms[r.PathValue("id")]
		if !ok {
			writeWSError(w, http.StatusNotFound, "No such resource")
			return
		}
		var body struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		vm.Params[body.Name] = body.Value
		// Real vmrest returns no useful body on success for this route (see
		// workstation_vm.go's handleWorkstationVMConfigParamSet comment) —
		// mirror that by writing 200 with no body at all.
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("GET /api/vms/{id}/params/{name}", func(w http.ResponseWriter, r *http.Request) {
		vm, ok := state.vms[r.PathValue("id")]
		if !ok {
			writeWSError(w, http.StatusNotFound, "No such resource")
			return
		}
		name := r.PathValue("name")
		value, ok := vm.Params[name]
		if !ok {
			writeWSError(w, http.StatusNotFound, "No such resource")
			return
		}
		writeWSJSON(w, http.StatusOK, map[string]interface{}{"name": name, "value": value})
	})

	mux.HandleFunc("GET /api/vms/{id}/restrictions", func(w http.ResponseWriter, r *http.Request) {
		vm, ok := state.vms[r.PathValue("id")]
		if !ok {
			writeWSError(w, http.StatusNotFound, "No such resource")
			return
		}
		writeWSJSON(w, http.StatusOK, map[string]interface{}{
			"id":     vm.ID,
			"cpu":    map[string]interface{}{"processors": vm.Processors},
			"memory": vm.Memory,
		})
	})

	mux.HandleFunc("POST /api/vms/registration", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Name string `json:"name"`
			Path string `json:"path"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Path == "" {
			writeWSError(w, http.StatusBadRequest, "Invalid parameters")
			return
		}
		id := state.newID("vm-registered")
		state.vms[id] = &wsFixtureVM{ID: id, Path: body.Path, Params: map[string]string{}, PowerState: "poweredOff"}
		writeWSJSON(w, http.StatusCreated, map[string]interface{}{"id": id, "path": body.Path})
	})

	mux.HandleFunc("GET /api/vms/{id}/power", func(w http.ResponseWriter, r *http.Request) {
		vm, ok := state.vms[r.PathValue("id")]
		if !ok {
			writeWSError(w, http.StatusNotFound, "No such resource")
			return
		}
		writeWSJSON(w, http.StatusOK, map[string]interface{}{"power_state": vm.PowerState})
	})

	mux.HandleFunc("PUT /api/vms/{id}/power", func(w http.ResponseWriter, r *http.Request) {
		vm, ok := state.vms[r.PathValue("id")]
		if !ok {
			writeWSError(w, http.StatusNotFound, "No such resource")
			return
		}
		buf := make([]byte, 32)
		n, _ := r.Body.Read(buf)
		op := string(buf[:n])
		switch op {
		case "on", "unpause":
			vm.PowerState = "poweredOn"
		case "off", "shutdown":
			vm.PowerState = "poweredOff"
		case "suspend":
			vm.PowerState = "suspended"
		case "pause":
			vm.PowerState = "paused"
		default:
			writeWSError(w, http.StatusBadRequest, "Invalid parameters")
			return
		}
		writeWSJSON(w, http.StatusOK, map[string]interface{}{"power_state": vm.PowerState})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, state
}

// newWSRegistry builds a Registry the way tools.NewRegistry is meant to be
// used for a Workstation-only connection: client (vSphere) is nil,
// WorkstationClient is set, ConnectionMode restricts to modeWorkstation —
// matching how main.go wires --workstation-url.
func newWSRegistry(t *testing.T, srv *httptest.Server, allowDestructive bool) *Registry {
	t.Helper()
	wsClient, err := workstation.NewClient(workstation.Config{URL: srv.URL + "/api", Username: "u", Password: "p"})
	if err != nil {
		t.Fatalf("workstation.NewClient: %v", err)
	}
	return NewRegistry(context.Background(), nil, RegistryOptions{
		WorkstationClient: wsClient,
		ConnectionMode:    ConnectionModeWorkstation,
		AllowDestructive:  allowDestructive,
	})
}

func TestWorkstationVMTools_Registration(t *testing.T) {
	srv, _ := newWSFullFixture(t)
	r := newWSRegistry(t, srv, true)

	want := []string{
		"vmware_workstation_vm_list",
		"vmware_workstation_vm_clone",
		"vmware_workstation_vm_get",
		"vmware_workstation_vm_update",
		"vmware_workstation_vm_delete",
		"vmware_workstation_vm_config_param_set",
		"vmware_workstation_vm_config_param_get",
		"vmware_workstation_vm_restrictions",
		"vmware_workstation_vm_register",
		"vmware_workstation_vm_power_get",
		"vmware_workstation_vm_power_set",
	}
	if len(want) != 11 {
		t.Fatalf("test bug: want list has %d entries, expected 11", len(want))
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
	// Not an exact-count check: ConnectionModeWorkstation registers every
	// modeWorkstation-class tool, including the sibling shared-folder/
	// network-adapter/host-network tools from this Fase's other parallel
	// group (tools/workstation_network.go) — this test only owns the 11
	// VM Management/Power Management tools listed in want.
	if len(got) < len(want) {
		t.Errorf("expected at least the %d VM tools to be registered, got %d total: %v", len(want), len(got), got)
	}
}

// TestWorkstationVMTools_HappyPathLifecycle drives
// list->clone->get->update->config_param_set->config_param_get->
// restrictions->power_get->power_set->delete against the stateful fixture,
// asserting each step's result shape.
func TestWorkstationVMTools_HappyPathLifecycle(t *testing.T) {
	srv, _ := newWSFullFixture(t)
	r := newWSRegistry(t, srv, true)

	// list
	raw, err := r.CallTool("vmware_workstation_vm_list", map[string]interface{}{})
	if err != nil {
		t.Fatalf("vm_list: %v", err)
	}
	var list []map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &list); err != nil {
		t.Fatalf("vm_list: failed to decode %q: %v", raw, err)
	}
	if len(list) != 1 || list[0]["id"] != "vm-seed-1" {
		t.Fatalf("vm_list: unexpected result: %s", raw)
	}

	// clone
	raw, err = r.CallTool("vmware_workstation_vm_clone", map[string]interface{}{
		"name": "clone1", "parent_id": "vm-seed-1", "confirm": true,
	})
	if err != nil {
		t.Fatalf("vm_clone: %v", err)
	}
	m := decodeResult(t, raw)
	cloneID, _ := m["id"].(string)
	if cloneID == "" {
		t.Fatalf("vm_clone: expected an id in result: %s", raw)
	}

	// get
	raw, err = r.CallTool("vmware_workstation_vm_get", map[string]interface{}{"id": cloneID})
	if err != nil {
		t.Fatalf("vm_get: %v", err)
	}
	m = decodeResult(t, raw)
	if m["id"] != cloneID {
		t.Fatalf("vm_get: unexpected result: %s", raw)
	}

	// update
	raw, err = r.CallTool("vmware_workstation_vm_update", map[string]interface{}{
		"id": cloneID, "processors": 4, "memory": 4096, "confirm": true,
	})
	if err != nil {
		t.Fatalf("vm_update: %v", err)
	}
	m = decodeResult(t, raw)
	cpu, _ := m["cpu"].(map[string]interface{})
	if cpu == nil || cpu["processors"] != float64(4) || m["memory"] != float64(4096) {
		t.Fatalf("vm_update: unexpected result: %s", raw)
	}

	// config_param_set
	raw, err = r.CallTool("vmware_workstation_vm_config_param_set", map[string]interface{}{
		"id": cloneID, "name": "myparam", "value": "myvalue", "confirm": true,
	})
	if err != nil {
		t.Fatalf("vm_config_param_set: %v", err)
	}
	m = decodeResult(t, raw)
	if m["updated"] != true || m["name"] != "myparam" || m["value"] != "myvalue" {
		t.Fatalf("vm_config_param_set: unexpected result: %s", raw)
	}

	// config_param_get
	raw, err = r.CallTool("vmware_workstation_vm_config_param_get", map[string]interface{}{
		"id": cloneID, "name": "myparam",
	})
	if err != nil {
		t.Fatalf("vm_config_param_get: %v", err)
	}
	m = decodeResult(t, raw)
	if m["value"] != "myvalue" {
		t.Fatalf("vm_config_param_get: unexpected result: %s", raw)
	}

	// restrictions
	raw, err = r.CallTool("vmware_workstation_vm_restrictions", map[string]interface{}{"id": cloneID})
	if err != nil {
		t.Fatalf("vm_restrictions: %v", err)
	}
	m = decodeResult(t, raw)
	if m["id"] != cloneID {
		t.Fatalf("vm_restrictions: unexpected result: %s", raw)
	}

	// power_get (initial state)
	raw, err = r.CallTool("vmware_workstation_vm_power_get", map[string]interface{}{"id": cloneID})
	if err != nil {
		t.Fatalf("vm_power_get: %v", err)
	}
	m = decodeResult(t, raw)
	if m["power_state"] != "poweredOff" {
		t.Fatalf("vm_power_get: unexpected initial state: %s", raw)
	}

	// power_set -> on
	raw, err = r.CallTool("vmware_workstation_vm_power_set", map[string]interface{}{
		"id": cloneID, "operation": "on", "confirm": true,
	})
	if err != nil {
		t.Fatalf("vm_power_set: %v", err)
	}
	m = decodeResult(t, raw)
	if m["power_state"] != "poweredOn" {
		t.Fatalf("vm_power_set: unexpected result: %s", raw)
	}

	// delete
	raw, err = r.CallTool("vmware_workstation_vm_delete", map[string]interface{}{
		"id": cloneID, "confirm": true,
	})
	if err != nil {
		t.Fatalf("vm_delete: %v", err)
	}
	m = decodeResult(t, raw)
	if m["deleted"] != true || m["id"] != cloneID {
		t.Fatalf("vm_delete: unexpected result: %s", raw)
	}

	// confirm it's actually gone
	if _, err := r.CallTool("vmware_workstation_vm_get", map[string]interface{}{"id": cloneID}); err == nil {
		t.Fatalf("vm_get: expected an error for a deleted VM, got success")
	}
}

// TestWorkstationVMTools_Register exercises POST /vms/registration
// separately from the main lifecycle (it has no vmPassword query param and
// a different curated-required field, see workstation_vm.go's top comment).
func TestWorkstationVMTools_Register(t *testing.T) {
	srv, _ := newWSFullFixture(t)
	r := newWSRegistry(t, srv, true)

	raw, err := r.CallTool("vmware_workstation_vm_register", map[string]interface{}{
		"path": `C:\VMs\Other\Other.vmx`, "confirm": true,
	})
	if err != nil {
		t.Fatalf("vm_register: %v", err)
	}
	m := decodeResult(t, raw)
	if m["path"] != `C:\VMs\Other\Other.vmx` || m["id"] == "" {
		t.Fatalf("vm_register: unexpected result: %s", raw)
	}

	if _, err := r.CallTool("vmware_workstation_vm_register", map[string]interface{}{"confirm": true}); err == nil {
		t.Fatal("vm_register: expected an error when path is missing")
	}
}

// TestWorkstationVMTools_RequiredArgValidation proves each tool rejects a
// missing required argument with a clean error, without needing the gate
// open (these checks run before wrapDestructiveWorkstation's confirm check
// for tier1/2 tools, and before any HTTP call either way).
func TestWorkstationVMTools_RequiredArgValidation(t *testing.T) {
	srv, _ := newWSFullFixture(t)
	r := newWSRegistry(t, srv, true)

	cases := []struct {
		tool string
		args map[string]interface{}
	}{
		{"vmware_workstation_vm_clone", map[string]interface{}{"confirm": true}},                               // missing name+parent_id
		{"vmware_workstation_vm_clone", map[string]interface{}{"name": "x", "confirm": true}},                  // missing parent_id
		{"vmware_workstation_vm_get", map[string]interface{}{}},                                                // missing id
		{"vmware_workstation_vm_update", map[string]interface{}{"id": "vm-seed-1", "confirm": true}},           // missing processors+memory
		{"vmware_workstation_vm_delete", map[string]interface{}{"confirm": true}},                              // missing id
		{"vmware_workstation_vm_config_param_set", map[string]interface{}{"id": "vm-seed-1", "confirm": true}}, // missing name+value
		{"vmware_workstation_vm_config_param_get", map[string]interface{}{"id": "vm-seed-1"}},                  // missing name
		{"vmware_workstation_vm_restrictions", map[string]interface{}{}},                                       // missing id
		{"vmware_workstation_vm_register", map[string]interface{}{"confirm": true}},                            // missing path
		{"vmware_workstation_vm_power_get", map[string]interface{}{}},                                          // missing id
		{"vmware_workstation_vm_power_set", map[string]interface{}{"id": "vm-seed-1", "confirm": true}},        // missing operation
	}
	for _, c := range cases {
		if _, err := r.CallTool(c.tool, c.args); err == nil {
			t.Errorf("%s: expected an error with args %v, got success", c.tool, c.args)
		}
	}
}

// TestWorkstationVMTools_GateAndConfirm proves the tier1 (delete) and tier2
// (clone/update/config_param_set/register/power_set) tools are wired
// through registerDestructiveWorkstation: denied with the gate closed
// (regardless of confirm), and denied without confirm:true even with the
// gate open.
func TestWorkstationVMTools_GateAndConfirm(t *testing.T) {
	srv, _ := newWSFullFixture(t)

	destructiveCalls := []struct {
		tool string
		args map[string]interface{}
	}{
		{"vmware_workstation_vm_clone", map[string]interface{}{"name": "x", "parent_id": "vm-seed-1"}},
		{"vmware_workstation_vm_update", map[string]interface{}{"id": "vm-seed-1", "processors": 1}},
		{"vmware_workstation_vm_delete", map[string]interface{}{"id": "vm-seed-1"}},
		{"vmware_workstation_vm_config_param_set", map[string]interface{}{"id": "vm-seed-1", "name": "n", "value": "v"}},
		{"vmware_workstation_vm_register", map[string]interface{}{"path": `C:\x.vmx`}},
		{"vmware_workstation_vm_power_set", map[string]interface{}{"id": "vm-seed-1", "operation": "on"}},
	}

	closedGate := newWSRegistry(t, srv, false)
	for _, c := range destructiveCalls {
		argsWithConfirm := map[string]interface{}{"confirm": true}
		for k, v := range c.args {
			argsWithConfirm[k] = v
		}
		if _, err := closedGate.CallTool(c.tool, argsWithConfirm); err == nil {
			t.Errorf("%s: expected denial with the gate closed even with confirm:true", c.tool)
		}
	}

	openGate := newWSRegistry(t, srv, true)
	for _, c := range destructiveCalls {
		if _, err := openGate.CallTool(c.tool, c.args); err == nil {
			t.Errorf("%s: expected denial without confirm:true, got success", c.tool)
		}
	}
}

// wsRawBodyFixture is a minimal fixture (not the stateful one) purpose-built
// to inspect exactly what bytes PUT /vms/{id}/power sends, and to prove a
// request never reaches the server at all when rejected client-side.
func newWSRawBodyFixture(t *testing.T) (*httptest.Server, *[]byte, *int) {
	t.Helper()
	var gotBody []byte
	calls := 0
	mux := http.NewServeMux()
	mux.HandleFunc("PUT /api/vms/{id}/power", func(w http.ResponseWriter, r *http.Request) {
		calls++
		buf := make([]byte, 64)
		n, _ := r.Body.Read(buf)
		gotBody = buf[:n]
		writeWSJSON(w, http.StatusOK, map[string]interface{}{"power_state": "poweredOn"})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, &gotBody, &calls
}

// TestWorkstationVMPowerSet_RawBodyExact proves handleWorkstationVMPowerSet
// sends the bare unquoted string "on" as the request body (via
// client.DoRawBody), not JSON-encoded `"on"` — this is the one vmrest route
// whose body isn't JSON (see workstation/client.go's DoRawBody doc comment).
func TestWorkstationVMPowerSet_RawBodyExact(t *testing.T) {
	srv, gotBody, _ := newWSRawBodyFixture(t)
	r := newWSRegistry(t, srv, true)

	raw, err := r.CallTool("vmware_workstation_vm_power_set", map[string]interface{}{
		"id": "vm-1", "operation": "on", "confirm": true,
	})
	if err != nil {
		t.Fatalf("vm_power_set: %v", err)
	}
	if string(*gotBody) != "on" {
		t.Fatalf("expected the fixture to receive the bare body \"on\", got %q", string(*gotBody))
	}
	m := decodeResult(t, raw)
	if m["power_state"] != "poweredOn" {
		t.Fatalf("unexpected result: %s", raw)
	}
}

// TestWorkstationVMPowerSet_InvalidOperationRejectedBeforeServer proves an
// operation value outside vmrest's confirmed enum is rejected client-side —
// the fixture's call counter must stay at 0.
func TestWorkstationVMPowerSet_InvalidOperationRejectedBeforeServer(t *testing.T) {
	srv, _, calls := newWSRawBodyFixture(t)
	r := newWSRegistry(t, srv, true)

	_, err := r.CallTool("vmware_workstation_vm_power_set", map[string]interface{}{
		"id": "vm-1", "operation": "frobnicate", "confirm": true,
	})
	if err == nil {
		t.Fatal("expected an error for an out-of-enum operation")
	}
	if *calls != 0 {
		t.Fatalf("expected the fixture server to never be called, got %d calls", *calls)
	}
}

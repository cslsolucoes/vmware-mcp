package tools

import (
	"context"
	"testing"

	"github.com/vmware/govmomi/simulator"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// newPerfRegistry builds a Registry the normal way (NewRegistry, which wires
// vm.go/host.go/etc via registerTools) and then manually layers this group's
// tools on top via withClass — same pattern as
// generated_custom_fields_test.go/generated_host_iscsi_portbinding_test.go,
// and for the same reason: registry.go itself must not be edited by this
// file (see generated_performance.go's top doc comment).
func newPerfRegistry(ctx context.Context, c *vmware.Client, opts RegistryOptions) *Registry {
	r := NewRegistry(ctx, c, opts)
	r.withClass(modeVSphereGeneral, registerPerformanceTools)
	return r
}

// performanceToolNames is the exact set registered by
// registerPerformanceTools — kept here so TestPerfTools_Registration and the
// vsphereGeneralTools list in mode_test.go (updated separately by the
// integration pass) can't silently drift from each other.
var performanceToolNames = []string{
	"vmware_perf_query",
	"vmware_perf_query_composite",
	"vmware_perf_query_counter",
	"vmware_perf_query_counter_by_level",
	"vmware_perf_provider_summary",
	"vmware_perf_query_available_metric",
	"vmware_perf_interval_create",
	"vmware_perf_interval_update",
	"vmware_perf_interval_remove",
}

// TestPerfTools_Registration proves all 9 PerformanceManager tools are
// reachable via ListTools once layered onto a Registry.
func TestPerfTools_Registration(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newPerfRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	if len(performanceToolNames) != 9 {
		t.Fatalf("test bug: performanceToolNames has %d entries, expected 9", len(performanceToolNames))
	}

	got := map[string]bool{}
	for _, tl := range r.ListTools() {
		got[tl.Name] = true
	}
	for _, name := range performanceToolNames {
		if !got[name] {
			t.Errorf("tool %s not registered", name)
		}
	}
}

// TestPerfTools_Validation proves each handler rejects missing/empty
// required arguments BEFORE any network round trip (so these fail even with
// the gate open and confirm:true).
func TestPerfTools_Validation(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newPerfRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	cases := []struct {
		name string
		args map[string]interface{}
		why  string
	}{
		{"vmware_perf_query", map[string]interface{}{}, "missing query_specs"},
		{"vmware_perf_query", map[string]interface{}{"query_specs": []interface{}{}}, "empty query_specs"},
		{"vmware_perf_query", map[string]interface{}{"query_specs": []interface{}{map[string]interface{}{}}}, "query_specs[0] missing entity_path"},
		{"vmware_perf_query_composite", map[string]interface{}{}, "missing query_spec"},
		{"vmware_perf_query_composite", map[string]interface{}{"query_spec": map[string]interface{}{}}, "query_spec missing entity_path"},
		{"vmware_perf_query_counter", map[string]interface{}{}, "missing counter_ids"},
		{"vmware_perf_query_counter", map[string]interface{}{"counter_ids": []interface{}{}}, "empty counter_ids"},
		{"vmware_perf_query_counter_by_level", map[string]interface{}{}, "missing level"},
		{"vmware_perf_provider_summary", map[string]interface{}{}, "missing entity_path"},
		{"vmware_perf_provider_summary", map[string]interface{}{"entity_path": "/does/not/exist"}, "entity does not resolve"},
		{"vmware_perf_query_available_metric", map[string]interface{}{}, "missing entity_path"},
		{"vmware_perf_interval_create", map[string]interface{}{"confirm": true}, "missing interval"},
		{"vmware_perf_interval_update", map[string]interface{}{"confirm": true}, "missing interval"},
		{"vmware_perf_interval_remove", map[string]interface{}{"confirm": true}, "missing sample_period"},
	}

	for _, tc := range cases {
		t.Run(tc.name+"/"+tc.why, func(t *testing.T) {
			if _, err := r.CallTool(tc.name, tc.args); err == nil {
				t.Fatalf("expected an error (%s) before/without a successful round trip", tc.why)
			}
		})
	}
}

// TestPerfTools_GateAndConfirm proves the tier1/tier2 destructive protection
// is wired on the three interval-management tools: a closed
// --allow-destructive gate denies the call, and an open gate still requires
// confirm:true.
func TestPerfTools_GateAndConfirm(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	closed := newPerfRegistry(context.Background(), c, RegistryOptions{})
	open := newPerfRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})

	cases := []struct {
		tool string
		args map[string]interface{}
	}{
		{"vmware_perf_interval_create", map[string]interface{}{"interval": map[string]interface{}{"key": 100, "name": "Test", "samplingPeriod": 300, "length": 3600, "level": 1, "enabled": true}}},
		{"vmware_perf_interval_update", map[string]interface{}{"interval": map[string]interface{}{"key": 100, "name": "Test", "samplingPeriod": 300, "length": 3600, "level": 1, "enabled": true}}},
		{"vmware_perf_interval_remove", map[string]interface{}{"sample_period": 300}},
	}

	for _, tc := range cases {
		t.Run(tc.tool, func(t *testing.T) {
			validEnough := map[string]interface{}{}
			for k, v := range tc.args {
				validEnough[k] = v
			}
			validEnough["confirm"] = true

			if _, err := closed.CallTool(tc.tool, validEnough); err == nil {
				t.Fatalf("%s: expected the closed destructive gate to deny the call", tc.tool)
			}

			if _, err := open.CallTool(tc.tool, tc.args); err == nil {
				t.Fatalf("%s: expected an error without confirm:true", tc.tool)
			}
		})
	}
}

// TestPerfTools_RealSuccess drives the 4 tools whose underlying method
// referencia/govmomi/simulator/performance_manager.go genuinely implements
// (QueryPerf, QueryPerfCounter, QueryPerfProviderSummary,
// QueryAvailablePerfMetric — see generated_performance.go's top doc comment)
// against real vcsim data and asserts on the actual response shape/values,
// not just "no error".
func TestPerfTools_RealSuccess(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newPerfRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	vm := firstVMPath(t, r)

	t.Run("vmware_perf_query", func(t *testing.T) {
		// maxSample caps the vcsim-generated sample count (n = 1 +
		// (end-start).Seconds()/interval — see
		// referencia/govmomi/simulator/performance_manager.go's QueryPerf).
		// Without it, the default 365-day window at a 20s interval builds
		// ~1.5M samples and the test takes >20s for no added coverage.
		raw, err := r.CallTool("vmware_perf_query", map[string]interface{}{
			"query_specs": []interface{}{
				map[string]interface{}{
					"entity_path": vm,
					"intervalId":  20,
					"maxSample":   5,
					"metricId":    []interface{}{map[string]interface{}{"counterId": 1, "instance": ""}},
				},
			},
		})
		if err != nil {
			t.Fatalf("vmware_perf_query failed: %v (%s)", err, raw)
		}
		m := decodeResult(t, raw)
		count, _ := m["count"].(float64)
		if count != 1 {
			t.Fatalf("expected count=1, got %v: %s", m["count"], raw)
		}
		metrics, ok := m["metrics"].([]interface{})
		if !ok || len(metrics) != 1 {
			t.Fatalf("expected 1 metrics entry: %s", raw)
		}
		entry, ok := metrics[0].(map[string]interface{})
		if !ok {
			t.Fatalf("expected metrics[0] to be an object: %s", raw)
		}
		sampleInfo, ok := entry["sampleInfo"].([]interface{})
		if !ok || len(sampleInfo) == 0 {
			t.Fatalf("expected a non-empty sampleInfo in metrics[0]: %s", raw)
		}
	})

	t.Run("vmware_perf_query_counter", func(t *testing.T) {
		raw, err := r.CallTool("vmware_perf_query_counter", map[string]interface{}{
			"counter_ids": []interface{}{1},
		})
		if err != nil {
			t.Fatalf("vmware_perf_query_counter failed: %v (%s)", err, raw)
		}
		m := decodeResult(t, raw)
		counters, ok := m["counters"].([]interface{})
		if !ok || len(counters) != 1 {
			t.Fatalf("expected 1 counter entry: %s", raw)
		}
		info, ok := counters[0].(map[string]interface{})
		if !ok {
			t.Fatalf("expected counters[0] to be an object: %s", raw)
		}
		if key, _ := info["key"].(float64); key != 1 {
			t.Fatalf("expected counter key 1, got %v: %s", info["key"], raw)
		}
	})

	t.Run("vmware_perf_provider_summary", func(t *testing.T) {
		raw, err := r.CallTool("vmware_perf_provider_summary", map[string]interface{}{
			"entity_path": vm,
		})
		if err != nil {
			t.Fatalf("vmware_perf_provider_summary failed: %v (%s)", err, raw)
		}
		m := decodeResult(t, raw)
		summary, ok := m["summary"].(map[string]interface{})
		if !ok {
			t.Fatalf("expected a summary object: %s", raw)
		}
		if rate, _ := summary["refreshRate"].(float64); rate != 20 {
			t.Fatalf("expected VM refreshRate=20 (realtimeProviderSummary), got %v: %s", summary["refreshRate"], raw)
		}
		if current, _ := summary["currentSupported"].(bool); !current {
			t.Fatalf("expected currentSupported=true for a VM: %s", raw)
		}
	})

	t.Run("vmware_perf_query_available_metric", func(t *testing.T) {
		raw, err := r.CallTool("vmware_perf_query_available_metric", map[string]interface{}{
			"entity_path": vm,
			"interval_id": 20,
		})
		if err != nil {
			t.Fatalf("vmware_perf_query_available_metric failed: %v (%s)", err, raw)
		}
		m := decodeResult(t, raw)
		count, _ := m["count"].(float64)
		if count == 0 {
			t.Fatalf("expected a non-empty available-metric list for a VM: %s", raw)
		}
	})
}

// TestPerfTools_ReachesServer drives the 5 tools whose underlying method has
// no handler anywhere in referencia/govmomi/simulator (QueryPerfComposite,
// QueryPerfCounterByLevel, CreatePerfInterval, UpdatePerfInterval,
// RemovePerfInterval — confirmed by grep, see generated_performance.go's top
// doc comment) with valid input, gate open, and confirm:true where
// applicable. Each call is expected to reach vcsim's dispatcher and come
// back with a clean server-side fault — assertReachesServer, the same
// helper generated_host_iscsi_portbinding_test.go uses for its own
// unsimulated methods.
func TestPerfTools_ReachesServer(t *testing.T) {
	c, cleanup := newSimClient(t, simulator.ESX())
	defer cleanup()

	r := newPerfRegistry(context.Background(), c, RegistryOptions{AllowDestructive: true})
	vm := firstVMPath(t, r)

	cases := []struct {
		name string
		args map[string]interface{}
	}{
		{"vmware_perf_query_composite", map[string]interface{}{
			"query_spec": map[string]interface{}{
				"entity_path": vm,
				"metricId":    []interface{}{map[string]interface{}{"counterId": 1, "instance": ""}},
			},
		}},
		{"vmware_perf_query_counter_by_level", map[string]interface{}{"level": 1}},
		{"vmware_perf_interval_create", map[string]interface{}{
			"interval": map[string]interface{}{"key": 100, "name": "Test", "samplingPeriod": 300, "length": 3600, "level": 1, "enabled": true},
			"confirm":  true,
		}},
		{"vmware_perf_interval_update", map[string]interface{}{
			"interval": map[string]interface{}{"key": 100, "name": "Test", "samplingPeriod": 300, "length": 3600, "level": 1, "enabled": true},
			"confirm":  true,
		}},
		{"vmware_perf_interval_remove", map[string]interface{}{"sample_period": 300, "confirm": true}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := r.CallTool(tc.name, tc.args)
			assertReachesServer(t, err, tc.name)
		})
	}
}

package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// registerPerformanceTools adds the vim25 PerformanceManager tools — the
// vSphere performance-statistics API (QueryPerf and friends), which had no
// tool coverage before this file. 9 tools total.
//
// MoRef: unlike IscsiManager (generated_host_iscsi_portbinding.go, which has
// no object.HostConfigManager accessor and can be nil on a host with no
// iSCSI HBA), PerformanceManager has a single well-known instance per
// connection, reachable directly off ServiceContent — confirmed by reading
// referencia/govmomi/vim25/types/types.go's ServiceContent struct
// (`PerfManager *ManagedObjectReference`) and both
// referencia/govmomi/simulator/esx/service_content.go and .../vpx/
// service_content.go, which populate it unconditionally
// ({Type: "PerformanceManager", Value: "ha-perfmgr"} / "PerfMgr") — so, like
// generated_authorization.go's AuthorizationManager, no nil-guard is needed
// here (real ESXi/vCenter always populate this field too). perfManagerRef
// below dereferences it directly.
//
// No object.PerformanceManager wrapper exists in referencia/govmomi/object
// (confirmed: no file there mentions PerformanceManager) — every handler
// below dials the raw vim25 SOAP method directly:
// methods.Xxx(ctx, client.Client.Client, &types.Xxx{This: ref, ...}), same
// as generated_host_iscsi_portbinding.go's IscsiManager handlers.
//
// Entity resolution: QueryPerf/QueryPerfComposite/QueryPerfProviderSummary/
// QueryAvailablePerfMetric each take a `entity` types.ManagedObjectReference
// naming the managed object whose statistics are being queried, which can be
// a VirtualMachine, HostSystem, ResourcePool, ClusterComputeResource,
// Datastore, or Datacenter. Rather than requiring the caller to already know
// each entity's raw {type, value} MoRef pair, every entity argument here is
// an "entity_path" full inventory-path string, resolved via resolveEntityRef
// (generated_authorization.go — object.SearchIndex.FindByInventoryPath-
// backed, works for any entity kind) — the same convention already
// established by generated_custom_fields.go's vmware_custom_field_set
// (entity_path).
//
// Complex specs: PerfQuerySpec (QueryPerf/QueryPerfComposite) and
// PerfInterval (CreatePerfInterval/UpdatePerfInterval) are accepted as raw
// JSON via decodeJSONArg instead of a hand-built recursive schema — the same
// "accept the concrete govmomi struct's own JSON shape" approach
// generated_custom_fields.go's vmware_custom_field_add uses for
// field_def_policy/field_policy (types.PrivilegePolicyDef). The one
// deviation: PerfQuerySpec's own `entity` field (a raw MoRef) is replaced by
// this file's perfQuerySpecArg with an `entity_path` string field instead
// (resolved via resolveEntityRef per the paragraph above) — every other
// field (startTime, endTime, maxSample, metricId, intervalId, format) is
// decoded straight into the field types govmomi itself declares
// (metricId decodes directly into []types.PerfMetricId, whose own JSON tags
// are counterId/instance), so the raw govmomi JSON shape a caller would find
// in the API docs still works verbatim for those fields.
//
// Tier: the six Query* operations are read-only (r.register, no
// confirm/tier). Of the three interval-management operations:
//   - vmware_perf_interval_create/vmware_perf_interval_update are tier2
//     (disruptive-but-reversible config changes — a created interval can be
//     removed again, an update can be reverted with another update).
//   - vmware_perf_interval_remove is tier1 (irreversible): removing a custom
//     historical interval discards the statistics VirtualCenter already
//     retained under that interval's samplingPeriod — recreating an interval
//     with the same name/period does not recover the lost history. Same
//     "data loss makes an otherwise-reversible-looking operation tier1"
//     reasoning generated_custom_fields.go applied to
//     vmware_custom_field_remove.
//
// vcsim coverage: referencia/govmomi/simulator/performance_manager.go
// implements QueryPerf, QueryPerfCounter, QueryPerfProviderSummary, and
// QueryAvailablePerfMetric with real (if synthetic) data/logic — those 4
// tools get RealSuccess tests below. QueryPerfComposite,
// QueryPerfCounterByLevel, CreatePerfInterval, RemovePerfInterval, and
// UpdatePerfInterval have no handler anywhere in referencia/govmomi/
// simulator (confirmed by grep — no other file references those method
// names), so calls reach vcsim's dispatcher and come back with a clean
// server-side fault (MethodNotFound-style) — generated_performance_test.go
// drives those 5 with assertReachesServer, the same helper
// generated_host_iscsi_portbinding_test.go/generated_vm_lifecycle_test.go
// use for their own unsimulated methods.
func registerPerformanceTools(r *Registry) {
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}
	entityPathArg := map[string]interface{}{
		"type":        "string",
		"description": `Full inventory path of the managed entity to query (e.g. "/DC0/vm/my-vm", "/DC0/host/cluster1/esxi-01.local"), resolved via SearchIndex.FindByInventoryPath. Works for any performance-provider entity kind: VirtualMachine, HostSystem, ResourcePool, ClusterComputeResource, Datastore, Datacenter.`,
	}
	querySpecItemSchema := map[string]interface{}{
		"type":        "object",
		"description": `A PerfQuerySpec. Properties: "entity_path" (string, required — see this tool's entity_path argument for the format), "startTime"/"endTime" (optional RFC3339 timestamp strings — server time range to retrieve; omit both for the most recent sample(s)), "maxSample" (optional integer — limits the number of samples returned, only meaningful with intervalId set to a real-time refreshRate), "metricId" (optional array of {"counterId": <int, from vmware_perf_query_counter/vmware_perf_query_counter_by_level>, "instance": <string, "" for aggregate, "*" for every instance>} — omit for every available metric), "intervalId" (optional integer — a real-time provider's refreshRate for current stats, or a historical interval's samplingPeriod for historical stats; see vmware_perf_provider_summary and vmware_perf_interval list of historical intervals), "format" (optional "normal" or "csv", defaults to "normal").`,
	}

	// --- Query (read-only) --------------------------------------------

	r.register("vmware_perf_query",
		"Retrieve performance statistics for one or more managed entities (VMs, hosts, clusters, resource pools, datastores, datacenters) matching one or more PerfQuerySpec queries. The core vSphere performance-metrics query operation.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query_specs": map[string]interface{}{
					"type":        "array",
					"items":       querySpecItemSchema,
					"description": "One or more query specs (see items schema). Each entry can query a different entity and/or different metrics.",
				},
			},
			"required": []interface{}{"query_specs"},
		},
		Tool{Handler: handlePerfQuery},
	)

	r.register("vmware_perf_query_composite",
		"Retrieve aggregated (rolled up) performance statistics for a managed entity and its children in one call — e.g. a cluster's aggregate CPU usage across its hosts. Requires metricId in the query spec.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query_spec": querySpecItemSchema,
			},
			"required": []interface{}{"query_spec"},
		},
		Tool{Handler: handlePerfQueryComposite},
	)

	r.register("vmware_perf_query_counter",
		"Look up performance counter metadata (name, group, unit, rollup type, stats type, level) for one or more counter IDs.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"counter_ids": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "integer"},
					"description": "One or more performance counter IDs, e.g. as returned in metricId.counterId by vmware_perf_query_available_metric.",
				},
			},
			"required": []interface{}{"counter_ids"},
		},
		Tool{Handler: handlePerfQueryCounter},
	)

	r.register("vmware_perf_query_counter_by_level",
		"List every performance counter available at or below a given statistics collection level (1-4).",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"level": map[string]interface{}{"type": "integer", "description": "Collection level, 1 through 4 (higher levels include more counters)."},
			},
			"required": []interface{}{"level"},
		},
		Tool{Handler: handlePerfQueryCounterByLevel},
	)

	r.register("vmware_perf_provider_summary",
		"Get performance-provider summary info for a managed entity: whether current (real-time) and/or summary (historical) stats are supported, and the real-time refresh rate.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"entity_path": entityPathArg},
			"required":   []interface{}{"entity_path"},
		},
		Tool{Handler: handlePerfProviderSummary},
	)

	r.register("vmware_perf_query_available_metric",
		"List the performance metrics (counterId + instance pairs) actually available for a managed entity over an optional time range/interval — narrower and entity-specific, unlike vmware_perf_query_counter/vmware_perf_query_counter_by_level which list every counter the system knows about.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"entity_path": entityPathArg,
				"begin_time":  map[string]interface{}{"type": "string", "description": "Optional RFC3339 timestamp. Defaults to the oldest available metric for the entity."},
				"end_time":    map[string]interface{}{"type": "string", "description": "Optional RFC3339 timestamp. Defaults to the most recently generated metric for the entity."},
				"interval_id": map[string]interface{}{"type": "integer", "description": "Optional interval (a real-time provider's refreshRate or a historical interval's samplingPeriod). Defaults to available metrics for historical statistics."},
			},
			"required": []interface{}{"entity_path"},
		},
		Tool{Handler: handlePerfQueryAvailableMetric},
	)

	// --- Historical interval management (destructive) ------------------

	intervalArg := map[string]interface{}{
		"type":        "object",
		"description": `A PerfInterval. Properties: "key" (integer, unique interval ID), "name" (string), "samplingPeriod" (integer, seconds of data granularity), "length" (integer, seconds statistics are retained), "level" (integer 1-4), "enabled" (boolean).`,
	}

	r.registerDestructive("vmware_perf_interval_create",
		"Create a new custom historical statistics interval (sampling period, retention length, name). Reversible via vmware_perf_interval_remove (which discards any statistics collected under it).",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"interval": intervalArg,
				"confirm":  confirmArg,
			},
			"required": []interface{}{"interval", "confirm"},
		},
		Tool{Handler: handlePerfIntervalCreate},
	)

	r.registerDestructive("vmware_perf_interval_update",
		"Update an existing historical statistics interval's name, sampling period, retention length, level, or enabled state. Reversible via another update.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"interval": intervalArg,
				"confirm":  confirmArg,
			},
			"required": []interface{}{"interval", "confirm"},
		},
		Tool{Handler: handlePerfIntervalUpdate},
	)

	r.registerDestructive("vmware_perf_interval_remove",
		"Remove a historical statistics interval by its sampling period. Irreversible: any statistics VirtualCenter already retained under that interval are discarded and cannot be recovered by recreating the interval.",
		tier1,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"sample_period": map[string]interface{}{"type": "integer", "description": "The samplingPeriod (in seconds) of the historical interval to remove."},
				"confirm":       confirmArg,
			},
			"required": []interface{}{"sample_period", "confirm"},
		},
		Tool{Handler: handlePerfIntervalRemove},
	)
}

// perfManagerRef returns the connection's single PerformanceManager MoRef.
// No nil-guard needed — see this file's top doc comment.
func perfManagerRef(client *vmware.Client) types.ManagedObjectReference {
	return *client.Client.ServiceContent.PerfManager
}

// perfQuerySpecArg is the JSON shape accepted for a single query_specs
// entry / the query_spec object — see this file's top doc comment ("Complex
// specs") for why entity_path replaces PerfQuerySpec's own raw `entity`
// MoRef field while every other field decodes straight into govmomi's own
// types (and JSON tags).
type perfQuerySpecArg struct {
	EntityPath string               `json:"entity_path"`
	StartTime  *time.Time           `json:"startTime,omitempty"`
	EndTime    *time.Time           `json:"endTime,omitempty"`
	MaxSample  int32                `json:"maxSample,omitempty"`
	MetricId   []types.PerfMetricId `json:"metricId,omitempty"`
	IntervalId int32                `json:"intervalId,omitempty"`
	Format     string               `json:"format,omitempty"`
}

// perfBuildQuerySpec resolves arg.EntityPath and assembles a real
// types.PerfQuerySpec from it.
func perfBuildQuerySpec(ctx context.Context, client *vmware.Client, arg perfQuerySpecArg) (types.PerfQuerySpec, error) {
	if arg.EntityPath == "" {
		return types.PerfQuerySpec{}, fmt.Errorf("entity_path is required")
	}
	entity, err := resolveEntityRef(ctx, client, arg.EntityPath)
	if err != nil {
		return types.PerfQuerySpec{}, err
	}
	return types.PerfQuerySpec{
		Entity:     entity,
		StartTime:  arg.StartTime,
		EndTime:    arg.EndTime,
		MaxSample:  arg.MaxSample,
		MetricId:   arg.MetricId,
		IntervalId: arg.IntervalId,
		Format:     arg.Format,
	}, nil
}

// perfDecodeQuerySpec decodes a single JSON object argument (raw) into a
// perfQuerySpecArg via decodeJSONArg — used by vmware_perf_query_composite's
// query_spec.
func perfDecodeQuerySpec(raw interface{}) (perfQuerySpecArg, error) {
	var arg perfQuerySpecArg
	if err := decodeJSONArg(raw, &arg); err != nil {
		return perfQuerySpecArg{}, fmt.Errorf("invalid query_spec: %w", err)
	}
	return arg, nil
}

// perfDecodeQuerySpecs decodes a JSON array argument (raw) into
// []perfQuerySpecArg via decodeJSONArg — used by vmware_perf_query's
// query_specs.
func perfDecodeQuerySpecs(raw interface{}) ([]perfQuerySpecArg, error) {
	var args []perfQuerySpecArg
	if err := decodeJSONArg(raw, &args); err != nil {
		return nil, fmt.Errorf("invalid query_specs: %w", err)
	}
	return args, nil
}

// perfInt32Slice converts a JSON array argument (decoded as []interface{})
// into a []int32, for vmware_perf_query_counter's counter_ids. Reuses
// toInt32 (vm.go) per element rather than redefining number coercion.
func perfInt32Slice(raw interface{}) ([]int32, error) {
	arr, ok := raw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("expected an array of numbers, got %T", raw)
	}
	out := make([]int32, 0, len(arr))
	for i, item := range arr {
		n, err := toInt32(item)
		if err != nil {
			return nil, fmt.Errorf("[%d]: %w", i, err)
		}
		out = append(out, n)
	}
	return out, nil
}

// perfParseTime parses an optional RFC3339 timestamp string argument. A nil
// or empty value returns (nil, nil) — the request field this backs
// (BeginTime/EndTime) is itself optional (*time.Time, omitempty).
func perfParseTime(raw interface{}) (*time.Time, error) {
	if raw == nil {
		return nil, nil
	}
	s, ok := raw.(string)
	if !ok {
		return nil, fmt.Errorf("expected an RFC3339 timestamp string, got %T", raw)
	}
	if s == "" {
		return nil, nil
	}
	ts, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil, fmt.Errorf("invalid RFC3339 timestamp %q: %w", s, err)
	}
	return &ts, nil
}

func handlePerfQuery(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	raw, ok := args["query_specs"]
	if !ok {
		return "", fmt.Errorf("query_specs is required")
	}
	specArgs, err := perfDecodeQuerySpecs(raw)
	if err != nil {
		return "", err
	}
	if len(specArgs) == 0 {
		return "", fmt.Errorf("query_specs must be a non-empty array")
	}

	specs := make([]types.PerfQuerySpec, 0, len(specArgs))
	for i, sa := range specArgs {
		spec, err := perfBuildQuerySpec(ctx, client, sa)
		if err != nil {
			return "", fmt.Errorf("query_specs[%d]: %w", i, err)
		}
		specs = append(specs, spec)
	}

	resp, err := methods.QueryPerf(ctx, client.Client.Client, &types.QueryPerf{
		This:      perfManagerRef(client),
		QuerySpec: specs,
	})
	if err != nil {
		return "", fmt.Errorf("failed to query performance statistics: %w", err)
	}

	return marshalJSON(map[string]interface{}{
		"count":   len(resp.Returnval),
		"metrics": resp.Returnval,
	})
}

func handlePerfQueryComposite(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	raw, ok := args["query_spec"]
	if !ok {
		return "", fmt.Errorf("query_spec is required")
	}
	sa, err := perfDecodeQuerySpec(raw)
	if err != nil {
		return "", err
	}
	spec, err := perfBuildQuerySpec(ctx, client, sa)
	if err != nil {
		return "", err
	}

	resp, err := methods.QueryPerfComposite(ctx, client.Client.Client, &types.QueryPerfComposite{
		This:      perfManagerRef(client),
		QuerySpec: spec,
	})
	if err != nil {
		return "", fmt.Errorf("failed to query composite performance statistics for %s: %w", sa.EntityPath, err)
	}

	return marshalJSON(map[string]interface{}{
		"entity_path": sa.EntityPath,
		"metric":      resp.Returnval,
	})
}

func handlePerfQueryCounter(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	raw, ok := args["counter_ids"]
	if !ok {
		return "", fmt.Errorf("counter_ids is required")
	}
	ids, err := perfInt32Slice(raw)
	if err != nil {
		return "", fmt.Errorf("invalid counter_ids: %w", err)
	}
	if len(ids) == 0 {
		return "", fmt.Errorf("counter_ids must be a non-empty array")
	}

	resp, err := methods.QueryPerfCounter(ctx, client.Client.Client, &types.QueryPerfCounter{
		This:      perfManagerRef(client),
		CounterId: ids,
	})
	if err != nil {
		return "", fmt.Errorf("failed to query performance counters %v: %w", ids, err)
	}

	return marshalJSON(map[string]interface{}{
		"count":    len(resp.Returnval),
		"counters": resp.Returnval,
	})
}

func handlePerfQueryCounterByLevel(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	raw, ok := args["level"]
	if !ok {
		return "", fmt.Errorf("level is required")
	}
	level, err := toInt32(raw)
	if err != nil {
		return "", fmt.Errorf("invalid level: %w", err)
	}

	resp, err := methods.QueryPerfCounterByLevel(ctx, client.Client.Client, &types.QueryPerfCounterByLevel{
		This:  perfManagerRef(client),
		Level: level,
	})
	if err != nil {
		return "", fmt.Errorf("failed to query performance counters at level %d: %w", level, err)
	}

	return marshalJSON(map[string]interface{}{
		"level":    level,
		"count":    len(resp.Returnval),
		"counters": resp.Returnval,
	})
}

func handlePerfProviderSummary(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	path, _ := args["entity_path"].(string)
	entity, err := resolveEntityRef(ctx, client, path)
	if err != nil {
		return "", err
	}

	resp, err := methods.QueryPerfProviderSummary(ctx, client.Client.Client, &types.QueryPerfProviderSummary{
		This:   perfManagerRef(client),
		Entity: entity,
	})
	if err != nil {
		return "", fmt.Errorf("failed to query performance provider summary for %s: %w", path, err)
	}

	return marshalJSON(map[string]interface{}{
		"entity_path": path,
		"summary":     resp.Returnval,
	})
}

func handlePerfQueryAvailableMetric(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	path, _ := args["entity_path"].(string)
	entity, err := resolveEntityRef(ctx, client, path)
	if err != nil {
		return "", err
	}
	beginTime, err := perfParseTime(args["begin_time"])
	if err != nil {
		return "", fmt.Errorf("invalid begin_time: %w", err)
	}
	endTime, err := perfParseTime(args["end_time"])
	if err != nil {
		return "", fmt.Errorf("invalid end_time: %w", err)
	}
	var intervalId int32
	if raw, ok := args["interval_id"]; ok {
		intervalId, err = toInt32(raw)
		if err != nil {
			return "", fmt.Errorf("invalid interval_id: %w", err)
		}
	}

	resp, err := methods.QueryAvailablePerfMetric(ctx, client.Client.Client, &types.QueryAvailablePerfMetric{
		This:       perfManagerRef(client),
		Entity:     entity,
		BeginTime:  beginTime,
		EndTime:    endTime,
		IntervalId: intervalId,
	})
	if err != nil {
		return "", fmt.Errorf("failed to query available performance metrics for %s: %w", path, err)
	}

	return marshalJSON(map[string]interface{}{
		"entity_path": path,
		"count":       len(resp.Returnval),
		"metrics":     resp.Returnval,
	})
}

func handlePerfIntervalCreate(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	raw, ok := args["interval"]
	if !ok {
		return "", fmt.Errorf("interval is required")
	}
	var interval types.PerfInterval
	if err := decodeJSONArg(raw, &interval); err != nil {
		return "", fmt.Errorf("invalid interval: %w", err)
	}

	if _, err := methods.CreatePerfInterval(ctx, client.Client.Client, &types.CreatePerfInterval{
		This:       perfManagerRef(client),
		IntervalId: interval,
	}); err != nil {
		return "", fmt.Errorf("failed to create performance interval %+v: %w", interval, err)
	}

	return marshalJSON(map[string]interface{}{"result": "perf_interval_created", "interval": interval})
}

func handlePerfIntervalUpdate(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	raw, ok := args["interval"]
	if !ok {
		return "", fmt.Errorf("interval is required")
	}
	var interval types.PerfInterval
	if err := decodeJSONArg(raw, &interval); err != nil {
		return "", fmt.Errorf("invalid interval: %w", err)
	}

	if _, err := methods.UpdatePerfInterval(ctx, client.Client.Client, &types.UpdatePerfInterval{
		This:     perfManagerRef(client),
		Interval: interval,
	}); err != nil {
		return "", fmt.Errorf("failed to update performance interval %+v: %w", interval, err)
	}

	return marshalJSON(map[string]interface{}{"result": "perf_interval_updated", "interval": interval})
}

func handlePerfIntervalRemove(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	raw, ok := args["sample_period"]
	if !ok {
		return "", fmt.Errorf("sample_period is required")
	}
	samplePeriod, err := toInt32(raw)
	if err != nil {
		return "", fmt.Errorf("invalid sample_period: %w", err)
	}

	if _, err := methods.RemovePerfInterval(ctx, client.Client.Client, &types.RemovePerfInterval{
		This:         perfManagerRef(client),
		SamplePeriod: samplePeriod,
	}); err != nil {
		return "", fmt.Errorf("failed to remove performance interval with sample period %d: %w", samplePeriod, err)
	}

	return marshalJSON(map[string]interface{}{"result": "perf_interval_removed", "sample_period": samplePeriod})
}

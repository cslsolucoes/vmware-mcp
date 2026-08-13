package tools

import (
	"context"
	"fmt"
	"net/http"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// applianceHealthSubsystems is the set of VAMI health subsystem endpoints —
// confirmed from `.workspace/vSphere Automation REST Resources for
// appliance.postman_collection.json` (GET rest/appliance/health/<name>), not
// guessed. No simulator or live vCenter Server Appliance is available to
// this project to verify further against (see the plan's Fase 4 "tensão
// conhecida" — the only real test target, 10.100.2.54, is a standalone ESXi
// host, which has no VAMI at all).
var applianceHealthSubsystems = []string{
	"system", "applmgmt", "database-storage", "load", "mem", "software-packages", "storage", "swap",
}

// registerApplianceTools registers the Fase 4 VAMI starter slice — not the
// ~130 routes documented in the appliance Postman collection, just
// version/health/uptime, per the plan.
func registerApplianceTools(r *Registry) {
	emptySchema := map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
	requiresVCSA := " Requires a vCenter Server Appliance — fails against a standalone ESXi host, which has no VAMI (Virtual Appliance Management Interface)."

	r.register("vmware_appliance_version",
		"Get the vCenter Server Appliance version/build (VAMI, GET /appliance/system/version)."+requiresVCSA,
		emptySchema,
		Tool{Handler: handleApplianceVersion},
	)

	r.register("vmware_appliance_uptime",
		"Get the vCenter Server Appliance uptime in seconds (VAMI, GET /appliance/system/uptime)."+requiresVCSA,
		emptySchema,
		Tool{Handler: handleApplianceUptime},
	)

	r.register("vmware_appliance_health",
		"Get the vCenter Server Appliance overall system health (VAMI, GET /appliance/health/system — typically green/yellow/orange/red/gray)."+requiresVCSA,
		emptySchema,
		Tool{Handler: handleApplianceHealth},
	)

	r.register("vmware_appliance_health_detail",
		"Get per-subsystem vCenter Server Appliance health: system, applmgmt, database-storage, load, mem, software-packages, storage, swap — one VAMI call per subsystem, aggregated into a single result. A subsystem that fails to answer is reported inline as {\"error\": ...} rather than failing the whole call."+requiresVCSA,
		emptySchema,
		Tool{Handler: handleApplianceHealthDetail},
	)
}

// applianceGet issues a GET against a VAMI /appliance/... path and decodes
// the response generically. govmomi's rest.Client.Do already unwraps the
// {"value": ...} envelope /rest endpoints use — see vapi/rest/client.go.
// No typed response struct: this project has no simulator/live vCenter
// Appliance to verify VAMI field names against, so a generic decode is more
// honest than a guessed struct that could silently drop or misname fields.
func applianceGet(ctx context.Context, client *vmware.Client, path string) (interface{}, error) {
	rc, err := client.REST(ctx)
	if err != nil {
		return nil, err // REST() already names the likely cause (no VAMI on standalone ESXi)
	}

	req := rc.Resource(path).Request(http.MethodGet)
	var result interface{}
	if err := rc.Do(ctx, req, &result); err != nil {
		return nil, fmt.Errorf("VAMI request to %s failed: %w", path, err)
	}
	return result, nil
}

func handleApplianceVersion(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	v, err := applianceGet(ctx, client, "/appliance/system/version")
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

func handleApplianceUptime(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	v, err := applianceGet(ctx, client, "/appliance/system/uptime")
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

func handleApplianceHealth(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	v, err := applianceGet(ctx, client, "/appliance/health/system")
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

func handleApplianceHealthDetail(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	result := make(map[string]interface{}, len(applianceHealthSubsystems))
	for _, sub := range applianceHealthSubsystems {
		v, err := applianceGet(ctx, client, "/appliance/health/"+sub)
		if err != nil {
			result[sub] = map[string]interface{}{"error": err.Error()}
			continue
		}
		result[sub] = v
	}
	return marshalJSON(result)
}

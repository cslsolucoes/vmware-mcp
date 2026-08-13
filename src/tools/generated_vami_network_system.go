package tools

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// registerVAMINetworkSystemTools is Fase 8b group "G2" (VAMI Networking +
// Health/lastcheck + Monitoring + System) of the codegen plan
// (".workspace/plans/MCPVMWare2026-08-10-175300-plano-cobertura-completa-api-codegen.plan.md").
//
// Unlike every Fase 0-8a group, these 19 routes have NO Go SDK wrapper at
// all — confirmed by listing govmomi v0.55.1's vapi/appliance subpackages
// (access/*, logging, networking (proxy/noproxy only), shutdown, simulator):
// none of health, monitoring, networking/dns, networking/interfaces,
// system/storage or system/time exist there. The only source for these
// routes is the vendored Postman collection
// (".workspace/vSphere Automation REST Resources for appliance.postman_collection.json",
// folders Health/Monitoring/Networking/System) — read directly (method, URL,
// raw JSON body) to build this file, not guessed.
//
// Same architecture as Fase 4's tools/appliance.go (the first VAMI file):
// client.REST(ctx) for the *rest.Client, Resource(path).Request(method,
// body...) to build the *http.Request, rc.Do(ctx, req, &result) with
// result interface{} for a generic decode — no typed response struct, same
// reasoning as appliance.go: there is no simulator or live VCSA available to
// this project to validate a guessed struct's field names against, so a
// generic decode is more honest. applianceRequest below generalizes
// appliance.go's applianceGet to also cover query parameters (needed only by
// vmware_appliance_monitoring_query) and JSON request bodies (every
// PUT/POST tool in this file) — appliance.go itself is not modified.
//
// Tiering (fixed by the group brief, not re-derived here):
//   - Tier 1 (irreversible): vmware_appliance_system_storage_resize — VAMI's
//     storage resize grows the appliance filesystem to fill the underlying
//     disk; filesystems cannot be shrunk back afterward, so even though
//     nothing is deleted, the action cannot be undone.
//   - Tier 2 (disruptive, reversible): every DNS domains/hostname/servers
//     add/set — each overwrites live name-resolution configuration the
//     appliance depends on (a bad hostname/DNS server change can strand the
//     appliance), but every one of them can be set back to a previous value
//     with another call of the same shape.
//   - No tier (read-only, including the two "test" endpoints — Hostname -
//     test and DNS servers - test only validate a candidate value against
//     the appliance without applying it, confirmed by reading their Postman
//     bodies/paths, no different in kind from a GET for gating purposes).
//
// Testing: httptest fixture (generated_vami_network_system_test.go), not
// vcsim — per the group brief, matching appliance_test.go's approach rather
// than generated_appliance_small_test.go's (vcsim reaches-server-only)
// approach, since a fixture lets the happy path actually be asserted, not
// just "an error came back".
func registerVAMINetworkSystemTools(r *Registry) {
	emptySchema := map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
	requiresVCSA := " Requires a vCenter Server Appliance — fails against a standalone ESXi host, which has no VAMI (Virtual Appliance Management Interface)."
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}

	// --- Health ----------------------------------------------------------------

	r.register("vmware_appliance_health_lastcheck",
		"Get the timestamp of the last vCenter Server Appliance health check run (VAMI, GET /appliance/health/system/lastcheck)."+requiresVCSA,
		emptySchema,
		Tool{Handler: handleApplianceHealthLastcheck},
	)

	// --- Monitoring --------------------------------------------------------------

	r.register("vmware_appliance_monitoring_list",
		"List every vCenter Server Appliance monitored item (CPU/memory/network/storage counters etc.) available for query, with id/name/description/units (VAMI, GET /appliance/monitoring)."+requiresVCSA,
		emptySchema,
		Tool{Handler: handleApplianceMonitoringList},
	)

	r.register("vmware_appliance_monitoring_item",
		`Get details of one vCenter Server Appliance monitored item by id, e.g. "net.rx.activity.eth0" — use vmware_appliance_monitoring_list to discover valid ids (VAMI, GET /appliance/monitoring/{item-id}).`+requiresVCSA,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"item_id": map[string]interface{}{
					"type":        "string",
					"description": `Monitored item id, e.g. "net.rx.activity.eth0" (see vmware_appliance_monitoring_list).`,
				},
			},
			"required": []interface{}{"item_id"},
		},
		Tool{Handler: handleApplianceMonitoringItem},
	)

	r.register("vmware_appliance_monitoring_query",
		"Query time-series statistics for one or more vCenter Server Appliance monitored items (VAMI, GET /appliance/monitoring/query)."+requiresVCSA,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"names": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": `Monitored item ids to query, e.g. ["net.rx.activity.eth0", "net.tx.activity.eth0"] (see vmware_appliance_monitoring_list). At least one required — sent as the indexed item.names.1, item.names.2, ... query parameters VAMI expects.`,
				},
				"interval": map[string]interface{}{
					"type":        "string",
					"description": `Sampling interval, e.g. "MINUTES5", "HOURS2", "DAY1". Optional — server default applies if omitted.`,
				},
				"start_time": map[string]interface{}{
					"type":        "string",
					"description": `Start of the query window, ISO-8601 (e.g. "2017-02-06T22:13:05.651Z"). Optional.`,
				},
				"end_time": map[string]interface{}{
					"type":        "string",
					"description": `End of the query window, ISO-8601. Optional.`,
				},
				"function": map[string]interface{}{
					"type":        "string",
					"description": `Aggregation function, e.g. "COUNT", "AVG", "MAX", "MIN", "SUM", "NONE". Optional.`,
				},
			},
			"required": []interface{}{"names"},
		},
		Tool{Handler: handleApplianceMonitoringQuery},
	)

	// --- Networking: DNS domains --------------------------------------------------

	r.register("vmware_appliance_network_dns_domains_list",
		"List the vCenter Server Appliance's configured DNS search domains (VAMI, GET /appliance/networking/dns/domains)."+requiresVCSA,
		emptySchema,
		Tool{Handler: handleApplianceNetworkDNSDomainsList},
	)

	r.registerDestructive("vmware_appliance_network_dns_domains_set",
		"Replace the vCenter Server Appliance's entire DNS search domain list (VAMI, PUT /appliance/networking/dns/domains). Overwrites whatever is currently configured — use vmware_appliance_network_dns_domains_add to append one instead."+requiresVCSA,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"domains": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": `Full replacement list of DNS search domains, e.g. ["vmware.com"].`,
				},
				"confirm": confirmArg,
			},
			"required": []interface{}{"domains", "confirm"},
		},
		Tool{Handler: handleApplianceNetworkDNSDomainsSet},
	)

	r.registerDestructive("vmware_appliance_network_dns_domains_add",
		"Add one DNS search domain to the vCenter Server Appliance's existing list (VAMI, POST /appliance/networking/dns/domains)."+requiresVCSA,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"domain":  map[string]interface{}{"type": "string", "description": `DNS search domain to add, e.g. "myvmware.com".`},
				"confirm": confirmArg,
			},
			"required": []interface{}{"domain", "confirm"},
		},
		Tool{Handler: handleApplianceNetworkDNSDomainsAdd},
	)

	// --- Networking: DNS hostname --------------------------------------------------

	r.register("vmware_appliance_network_dns_hostname",
		"Get the vCenter Server Appliance's configured hostname/FQDN (VAMI, GET /appliance/networking/dns/hostname)."+requiresVCSA,
		emptySchema,
		Tool{Handler: handleApplianceNetworkDNSHostname},
	)

	r.register("vmware_appliance_network_dns_hostname_test",
		"Validate a candidate hostname/FQDN for the vCenter Server Appliance without applying it (VAMI, POST /appliance/networking/dns/hostname/test). Read-only — use vmware_appliance_network_dns_hostname_set to actually apply it."+requiresVCSA,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{"type": "string", "description": "Candidate hostname/FQDN to validate."},
			},
			"required": []interface{}{"name"},
		},
		Tool{Handler: handleApplianceNetworkDNSHostnameTest},
	)

	r.registerDestructive("vmware_appliance_network_dns_hostname_set",
		"Set the vCenter Server Appliance's hostname/FQDN (VAMI, PUT /appliance/networking/dns/hostname). Consider validating first with vmware_appliance_network_dns_hostname_test."+requiresVCSA,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name":    map[string]interface{}{"type": "string", "description": "New hostname/FQDN to apply."},
				"confirm": confirmArg,
			},
			"required": []interface{}{"name", "confirm"},
		},
		Tool{Handler: handleApplianceNetworkDNSHostnameSet},
	)

	// --- Networking: DNS servers --------------------------------------------------

	r.register("vmware_appliance_network_dns_servers_list",
		"Get the vCenter Server Appliance's DNS server configuration (mode + server list) (VAMI, GET /appliance/networking/dns/servers)."+requiresVCSA,
		emptySchema,
		Tool{Handler: handleApplianceNetworkDNSServersList},
	)

	r.registerDestructive("vmware_appliance_network_dns_servers_add",
		"Add one DNS server to the vCenter Server Appliance's existing list (VAMI, POST /appliance/networking/dns/servers)."+requiresVCSA,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"server":  map[string]interface{}{"type": "string", "description": "DNS server IP address to add."},
				"confirm": confirmArg,
			},
			"required": []interface{}{"server", "confirm"},
		},
		Tool{Handler: handleApplianceNetworkDNSServersAdd},
	)

	r.registerDestructive("vmware_appliance_network_dns_servers_set",
		"Replace the vCenter Server Appliance's entire DNS server configuration (VAMI, PUT /appliance/networking/dns/servers). Overwrites whatever is currently configured — use vmware_appliance_network_dns_servers_add to append one instead."+requiresVCSA,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"mode": map[string]interface{}{
					"type":        "string",
					"enum":        []interface{}{"dhcp", "is_static"},
					"description": `"dhcp" to obtain DNS servers automatically, or "is_static" to use the servers list below.`,
				},
				"servers": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": `Full replacement list of DNS server IP addresses. Required when mode is "is_static"; ignored for "dhcp".`,
				},
				"confirm": confirmArg,
			},
			"required": []interface{}{"mode", "confirm"},
		},
		Tool{Handler: handleApplianceNetworkDNSServersSet},
	)

	r.register("vmware_appliance_network_dns_servers_test",
		"Validate candidate DNS server addresses for the vCenter Server Appliance without applying them (VAMI, POST /appliance/networking/dns/servers/test). Read-only."+requiresVCSA,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"servers": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Candidate DNS server IP addresses to validate.",
				},
			},
			"required": []interface{}{"servers"},
		},
		Tool{Handler: handleApplianceNetworkDNSServersTest},
	)

	// --- Networking: interfaces ------------------------------------------------

	r.register("vmware_appliance_network_interfaces_list",
		"List the vCenter Server Appliance's network interfaces (VAMI, GET /appliance/networking/interfaces)."+requiresVCSA,
		emptySchema,
		Tool{Handler: handleApplianceNetworkInterfacesList},
	)

	r.register("vmware_appliance_network_interface_details",
		`Get details of one vCenter Server Appliance network interface by id, e.g. "nic0" (VAMI, GET /appliance/networking/interfaces/{nic-id}).`+requiresVCSA,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"interface_id": map[string]interface{}{
					"type":        "string",
					"description": `Network interface id, e.g. "nic0" (see vmware_appliance_network_interfaces_list).`,
				},
			},
			"required": []interface{}{"interface_id"},
		},
		Tool{Handler: handleApplianceNetworkInterfaceDetails},
	)

	// --- System ------------------------------------------------------------------

	r.register("vmware_appliance_system_storage",
		"Get the vCenter Server Appliance's storage/disk partition layout (VAMI, GET /appliance/system/storage)."+requiresVCSA,
		emptySchema,
		Tool{Handler: handleApplianceSystemStorage},
	)

	r.registerDestructive("vmware_appliance_system_storage_resize",
		"Resize the vCenter Server Appliance's storage/filesystem to fill the underlying disk (VAMI, POST /appliance/system/storage/resize). One-way — filesystems cannot be shrunk back afterward, so this is tier 1 (irreversible) even though nothing is deleted."+requiresVCSA,
		tier1,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"confirm": confirmArg},
			"required":   []interface{}{"confirm"},
		},
		Tool{Handler: handleApplianceSystemStorageResize},
	)

	r.register("vmware_appliance_system_time",
		"Get the vCenter Server Appliance's current system date/time and timezone (VAMI, GET /appliance/system/time)."+requiresVCSA,
		emptySchema,
		Tool{Handler: handleApplianceSystemTime},
	)
}

// applianceRequest issues an HTTP request against a VAMI /appliance/... path
// and decodes the response generically — see this file's top doc comment
// for why (no typed govmomi wrapper exists for any of this group's routes).
// query, when non-nil, is added to the URL via Resource.WithParam (used only
// by vmware_appliance_monitoring_query's item.interval/item.end_time/
// item.names.N/item.start_time/item.function parameters). body, when
// non-nil, is JSON-encoded as the request payload — govmomi's
// Resource.Request(method, body...) treats a present-but-empty body arg
// list as "no body at all", which matters for POST /system/storage/resize
// (confirmed empty-bodied in the Postman collection).
func applianceRequest(ctx context.Context, client *vmware.Client, method, path string, query map[string][]string, body interface{}) (interface{}, error) {
	rc, err := client.REST(ctx)
	if err != nil {
		return nil, err // REST() already names the likely cause (no VAMI on standalone ESXi)
	}

	res := rc.Resource(path)
	for name, values := range query {
		for _, v := range values {
			res = res.WithParam(name, v)
		}
	}

	var req *http.Request
	if body != nil {
		req = res.Request(method, body)
	} else {
		req = res.Request(method)
	}

	var result interface{}
	if err := rc.Do(ctx, req, &result); err != nil {
		return nil, fmt.Errorf("VAMI request %s %s failed: %w", method, path, err)
	}
	return result, nil
}

// --- Health handlers -----------------------------------------------------------

func handleApplianceHealthLastcheck(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	v, err := applianceRequest(ctx, client, http.MethodGet, "/appliance/health/system/lastcheck", nil, nil)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

// --- Monitoring handlers -------------------------------------------------------

func handleApplianceMonitoringList(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	v, err := applianceRequest(ctx, client, http.MethodGet, "/appliance/monitoring", nil, nil)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

func handleApplianceMonitoringItem(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	itemID, ok := args["item_id"].(string)
	if !ok || itemID == "" {
		return "", fmt.Errorf("item_id is required")
	}

	v, err := applianceRequest(ctx, client, http.MethodGet, "/appliance/monitoring/"+url.PathEscape(itemID), nil, nil)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

func handleApplianceMonitoringQuery(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	rawNames, ok := args["names"]
	if !ok {
		return "", fmt.Errorf(`names is required (at least one monitored item id, e.g. "net.rx.activity.eth0")`)
	}
	names, err := toStringSlice(rawNames)
	if err != nil {
		return "", fmt.Errorf("invalid names: %w", err)
	}
	if len(names) == 0 {
		return "", fmt.Errorf("names must contain at least one item id")
	}

	query := map[string][]string{}
	for i, name := range names {
		query[fmt.Sprintf("item.names.%d", i+1)] = []string{name}
	}
	if v, ok := args["interval"].(string); ok && v != "" {
		query["item.interval"] = []string{v}
	}
	if v, ok := args["start_time"].(string); ok && v != "" {
		query["item.start_time"] = []string{v}
	}
	if v, ok := args["end_time"].(string); ok && v != "" {
		query["item.end_time"] = []string{v}
	}
	if v, ok := args["function"].(string); ok && v != "" {
		query["item.function"] = []string{v}
	}

	v, err := applianceRequest(ctx, client, http.MethodGet, "/appliance/monitoring/query", query, nil)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

// --- Networking: DNS domains handlers -------------------------------------------

func handleApplianceNetworkDNSDomainsList(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	v, err := applianceRequest(ctx, client, http.MethodGet, "/appliance/networking/dns/domains", nil, nil)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

func handleApplianceNetworkDNSDomainsSet(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	rawDomains, ok := args["domains"]
	if !ok {
		return "", fmt.Errorf("domains is required")
	}
	domains, err := toStringSlice(rawDomains)
	if err != nil {
		return "", fmt.Errorf("invalid domains: %w", err)
	}

	body := map[string]interface{}{"domains": domains}
	if _, err := applianceRequest(ctx, client, http.MethodPut, "/appliance/networking/dns/domains", nil, body); err != nil {
		return "", fmt.Errorf("failed to set DNS search domains: %w", err)
	}
	return marshalJSON(map[string]interface{}{"domains": domains, "result": "set"})
}

func handleApplianceNetworkDNSDomainsAdd(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	domain, ok := args["domain"].(string)
	if !ok || domain == "" {
		return "", fmt.Errorf("domain is required")
	}

	body := map[string]interface{}{"domain": domain}
	if _, err := applianceRequest(ctx, client, http.MethodPost, "/appliance/networking/dns/domains", nil, body); err != nil {
		return "", fmt.Errorf("failed to add DNS search domain: %w", err)
	}
	return marshalJSON(map[string]interface{}{"domain": domain, "result": "added"})
}

// --- Networking: DNS hostname handlers -------------------------------------------

func handleApplianceNetworkDNSHostname(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	v, err := applianceRequest(ctx, client, http.MethodGet, "/appliance/networking/dns/hostname", nil, nil)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

func handleApplianceNetworkDNSHostnameTest(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	name, ok := args["name"].(string)
	if !ok || name == "" {
		return "", fmt.Errorf("name is required")
	}

	body := map[string]interface{}{"name": name}
	v, err := applianceRequest(ctx, client, http.MethodPost, "/appliance/networking/dns/hostname/test", nil, body)
	if err != nil {
		return "", fmt.Errorf("failed to validate candidate hostname: %w", err)
	}
	return marshalJSON(v)
}

func handleApplianceNetworkDNSHostnameSet(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	name, ok := args["name"].(string)
	if !ok || name == "" {
		return "", fmt.Errorf("name is required")
	}

	body := map[string]interface{}{"name": name}
	if _, err := applianceRequest(ctx, client, http.MethodPut, "/appliance/networking/dns/hostname", nil, body); err != nil {
		return "", fmt.Errorf("failed to set hostname: %w", err)
	}
	return marshalJSON(map[string]interface{}{"name": name, "result": "set"})
}

// --- Networking: DNS servers handlers -------------------------------------------

func handleApplianceNetworkDNSServersList(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	v, err := applianceRequest(ctx, client, http.MethodGet, "/appliance/networking/dns/servers", nil, nil)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

func handleApplianceNetworkDNSServersAdd(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	server, ok := args["server"].(string)
	if !ok || server == "" {
		return "", fmt.Errorf("server is required")
	}

	body := map[string]interface{}{"server": server}
	if _, err := applianceRequest(ctx, client, http.MethodPost, "/appliance/networking/dns/servers", nil, body); err != nil {
		return "", fmt.Errorf("failed to add DNS server: %w", err)
	}
	return marshalJSON(map[string]interface{}{"server": server, "result": "added"})
}

func handleApplianceNetworkDNSServersSet(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	mode, ok := args["mode"].(string)
	if !ok || mode == "" {
		return "", fmt.Errorf(`mode is required ("dhcp" or "is_static")`)
	}

	var servers []string
	if raw, ok := args["servers"]; ok {
		s, err := toStringSlice(raw)
		if err != nil {
			return "", fmt.Errorf("invalid servers: %w", err)
		}
		servers = s
	} else {
		servers = []string{}
	}

	body := map[string]interface{}{"config": map[string]interface{}{"mode": mode, "servers": servers}}
	if _, err := applianceRequest(ctx, client, http.MethodPut, "/appliance/networking/dns/servers", nil, body); err != nil {
		return "", fmt.Errorf("failed to set DNS servers: %w", err)
	}
	return marshalJSON(map[string]interface{}{"mode": mode, "servers": servers, "result": "set"})
}

func handleApplianceNetworkDNSServersTest(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	rawServers, ok := args["servers"]
	if !ok {
		return "", fmt.Errorf("servers is required")
	}
	servers, err := toStringSlice(rawServers)
	if err != nil {
		return "", fmt.Errorf("invalid servers: %w", err)
	}

	body := map[string]interface{}{"servers": servers}
	v, err := applianceRequest(ctx, client, http.MethodPost, "/appliance/networking/dns/servers/test", nil, body)
	if err != nil {
		return "", fmt.Errorf("failed to validate candidate DNS servers: %w", err)
	}
	return marshalJSON(v)
}

// --- Networking: interfaces handlers ---------------------------------------

func handleApplianceNetworkInterfacesList(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	v, err := applianceRequest(ctx, client, http.MethodGet, "/appliance/networking/interfaces", nil, nil)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

func handleApplianceNetworkInterfaceDetails(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	interfaceID, ok := args["interface_id"].(string)
	if !ok || interfaceID == "" {
		return "", fmt.Errorf("interface_id is required")
	}

	v, err := applianceRequest(ctx, client, http.MethodGet, "/appliance/networking/interfaces/"+url.PathEscape(interfaceID), nil, nil)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

// --- System handlers -----------------------------------------------------------

func handleApplianceSystemStorage(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	v, err := applianceRequest(ctx, client, http.MethodGet, "/appliance/system/storage", nil, nil)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

func handleApplianceSystemStorageResize(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	v, err := applianceRequest(ctx, client, http.MethodPost, "/appliance/system/storage/resize", nil, nil)
	if err != nil {
		return "", fmt.Errorf("failed to resize appliance storage: %w", err)
	}
	return marshalJSON(map[string]interface{}{"result": "resize_triggered", "response": v})
}

func handleApplianceSystemTime(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	v, err := applianceRequest(ctx, client, http.MethodGet, "/appliance/system/time", nil, nil)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

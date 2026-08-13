package tools

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/cslsoftwares/mcpvmware/cloudaws"
)

// registerCloudAWSNetworkingEdgeTools is Fase 10's "Grupo 4" (Networking —
// IPSec/L2VPN/edge DNS/Edge Devices/DHCP/Statistics/Connectivity Tests) of
// the codegen plan
// (".workspace/plans/MCPVMWare2026-08-10-175300-plano-cobertura-completa-api-codegen.plan.md"),
// 24 VMware Cloud on AWS (VMC) routes confirmed by structurally reading
// ".workspace/VMware Cloud on AWS APIs.postman_collection.json"'s
// "Networking/IPSec", "Networking/L2VPN", "Networking/DNS" (the edge one,
// NOT the SDDC public/private DNS folder at a different location in the
// same collection — that one belongs to a different Fase 10 group),
// "Networking/Edge Devices" (+ its "vNICs"/"Peer Config" sub-folders),
// "Networking/DHCP", "Networking/Statistics" (+ its "Dashboard" sub-folder),
// and the root "Networking" folder's 2 connectivity-test items — not
// eyeballed, every path/method/query-param pair below was read directly out
// of the vendored JSON.
//
// # Curation decisions
//
//   - Tiers follow this fase's stated convention exactly: GET = sem-tier
//     (registered via r.registerCloudAWS), DELETE = tier1 (removes a
//     config, can sever a VPN/DNS-relay/tunnel), POST/PUT = tier2 (mutation,
//     reversible by calling again with the old values) — both tiers via
//     r.registerDestructiveCloudAWS. The one deliberate exception is the
//     connectivity-tests POST (a diagnostic run, mutates no network config)
//     and every statistics/dashboard/status GET, all sem-tier per the
//     brief.
//   - L2VPN "Update" vs "Details" dedup — confirmed by reading the raw
//     collection JSON directly (not assumed from the brief's description):
//     both items are PUT
//     .../networks/4.0/sddc/cgws/:edgeId/l2vpn/config — "Details" (line
//     ~4802 of the vendored JSON) carries a "showSensitiveData" query
//     param and description "Retrieve SDDC L2 VPN configuration"; "Update"
//     (line ~4868) has no query param and description "Modify SDDC L2 VPN
//     configuration". Same method, same path — genuinely one operation
//     with an optional query flag, not two distinct routes; merged here
//     into a single vmware_cloudaws_network_l2vpn_config_update tool with
//     an optional show_sensitive_data argument, exactly as the brief
//     anticipated. (Both items' example request bodies are empty raw
//     strings in the collection, not a real payload either — see the next
//     bullet.)
//   - Request bodies for every *_config_update tool (IPSec, L2VPN, edge
//     DNS) are left as an OPTIONAL free-form JSON object passed through
//     verbatim, not a field-level schema. The vendored collection gives no
//     usable evidence to derive one from: L2VPN's and edge DNS's Update
//     items both have a literal empty raw body (""), and IPSec's Update
//     item's "body" is the placeholder note "IPsec Configuration dto
//     object." — not an actual example payload. Inventing a field list
//     from that would be a guess dressed up as a schema, which this
//     project's established posture (see appliance.go's applianceGet doc
//     comment and generated_vami_techpreview_network.go's header) treats as
//     worse than being honest about the gap. Same reasoning applies to
//     connectivity-tests' POST body (its own example is the placeholder
//     text "request information") — left optional and free-form as
//     "request".
//   - org/sddc/edge_id are opaque required string arguments everywhere, per
//     the brief — no inventory resolution/lookup by name is attempted (VMC
//     orgs/SDDCs/edges have no local govmomi-style Finder in this
//     project).
//   - Query parameter casing is preserved exactly as the collection defines
//     it per route — some are lowercase-single-word (getlatest, detailed,
//     objecttype, objectid, templateid, enable, interval, action), others
//     camelCase (showSensitiveData, startTime, endTime) — VMC's own API is
//     inconsistent about this across routes, so each is hardcoded to match
//     its route rather than normalized to one convention.
//
// # Testing
//
// No VMC organization/SDDC is reachable from this project (CSP auth
// requires a manually-generated refresh token outside this MCP server's
// reach — see cloudaws/client.go's package doc comment) and there is no
// simulator for VMC's REST surface (unlike vSphere's vcsim). Tests in
// cloudaws_networking_edge_test.go use an httptest.Server fixture serving
// both the CSP token-exchange endpoint and canned responses for a
// representative route per sub-domain — the same fixture-over-simulator
// approach already established for tools/appliance_test.go and
// generated_vami_techpreview_network_test.go, for the same reason (no
// simulator coverage exists for this surface).
func registerCloudAWSNetworkingEdgeTools(r *Registry) {
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}
	orgArg := map[string]interface{}{
		"type":        "string",
		"description": "VMC organization ID (opaque identifier — this tool performs no inventory resolution/lookup by name).",
	}
	sddcArg := map[string]interface{}{
		"type":        "string",
		"description": "VMC SDDC ID within org (opaque identifier — no inventory resolution).",
	}
	edgeIDArg := map[string]interface{}{
		"type":        "string",
		"description": "NSX Edge (management or compute gateway) ID within the SDDC (opaque identifier — no inventory resolution; use vmware_cloudaws_network_edge_list to discover edge IDs for a given org/sddc).",
	}
	showSensitiveDataArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Optional. When true, includes sensitive data (e.g. pre-shared keys) in the response. Omitted entirely from the request when not passed, letting the VMC API apply its own default.",
	}
	configBodyArg := func(what string) map[string]interface{} {
		return map[string]interface{}{
			"type":                 "object",
			"additionalProperties": true,
			"description": fmt.Sprintf(
				"Optional. Full %s configuration object, sent verbatim as the request body. The exact field-level JSON schema for this object is not documented in the vendored Postman collection (its example request body is empty or a placeholder note, not real JSON) — shape it after what the corresponding _get/_config tool returns for this edge, or VMC/NSX Edge API documentation. Omit to send a bodyless request.",
				what),
		}
	}
	intervalArg := map[string]interface{}{
		"type":        "string",
		"description": "Optional statistics interval (VMC/NSX Edge-defined value — not enumerated here, no live endpoint was available to confirm the accepted set). Omitted from the request when not passed.",
	}
	startTimeArg := map[string]interface{}{
		"type":        "string",
		"description": "Optional start of the time range for these statistics (VMC/NSX Edge-defined format, e.g. epoch millis — not confirmed against a live endpoint). Omitted from the request when not passed.",
	}
	endTimeArg := map[string]interface{}{
		"type":        "string",
		"description": "Optional end of the time range for these statistics (same format caveat as start_time). Omitted from the request when not passed.",
	}

	// ---- IPSec (4) ----------------------------------------------------

	r.registerCloudAWS("vmware_cloudaws_network_ipsec_config_get",
		"Get the IPsec VPN configuration for a management or compute gateway (NSX Edge) in VMware Cloud on AWS (GET .../edges/{edgeId}/ipsec/config).",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"org":                 orgArg,
				"sddc":                sddcArg,
				"edge_id":             edgeIDArg,
				"show_sensitive_data": showSensitiveDataArg,
			},
			"required": []interface{}{"org", "sddc", "edge_id"},
		},
		Tool{CloudHandler: handleCloudAWSIPSecConfigGet},
	)

	r.registerDestructiveCloudAWS("vmware_cloudaws_network_ipsec_config_update",
		"Modify the IPsec VPN configuration for a management or compute gateway (NSX Edge) in VMware Cloud on AWS (PUT .../edges/{edgeId}/ipsec/config).",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"org":     orgArg,
				"sddc":    sddcArg,
				"edge_id": edgeIDArg,
				"config":  configBodyArg("IPsec VPN"),
				"confirm": confirmArg,
			},
			"required": []interface{}{"org", "sddc", "edge_id", "confirm"},
		},
		Tool{CloudHandler: handleCloudAWSIPSecConfigUpdate},
	)

	r.registerDestructiveCloudAWS("vmware_cloudaws_network_ipsec_config_delete",
		"Delete the IPsec VPN configuration for a management or compute gateway (NSX Edge) in VMware Cloud on AWS (DELETE .../edges/{edgeId}/ipsec/config). Removes every IPsec tunnel configured on this edge.",
		tier1,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"org":     orgArg,
				"sddc":    sddcArg,
				"edge_id": edgeIDArg,
				"confirm": confirmArg,
			},
			"required": []interface{}{"org", "sddc", "edge_id", "confirm"},
		},
		Tool{CloudHandler: handleCloudAWSIPSecConfigDelete},
	)

	r.registerCloudAWS("vmware_cloudaws_network_ipsec_statistics_get",
		"Get IPsec VPN statistics for a management or compute gateway (NSX Edge) in VMware Cloud on AWS (GET .../edges/{edgeId}/ipsec/statistics).",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"org":     orgArg,
				"sddc":    sddcArg,
				"edge_id": edgeIDArg,
			},
			"required": []interface{}{"org", "sddc", "edge_id"},
		},
		Tool{CloudHandler: handleCloudAWSIPSecStatisticsGet},
	)

	// ---- L2VPN (3) — see this function's doc comment for the Update/Details dedup ----

	r.registerDestructiveCloudAWS("vmware_cloudaws_network_l2vpn_config_update",
		"Modify (or, per the underlying VMC API's own \"Details\" variant of this exact same PUT route, retrieve) the SDDC L2 VPN configuration for a management or compute gateway (NSX Edge) in VMware Cloud on AWS (PUT .../sddc/cgws/{edgeId}/l2vpn/config). The vendored API spec exposes both behaviors as the identical method+path, differing only by the optional show_sensitive_data query flag — merged into one tool here rather than registered twice.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"org":                 orgArg,
				"sddc":                sddcArg,
				"edge_id":             edgeIDArg,
				"show_sensitive_data": showSensitiveDataArg,
				"config":              configBodyArg("L2 VPN"),
				"confirm":             confirmArg,
			},
			"required": []interface{}{"org", "sddc", "edge_id", "confirm"},
		},
		Tool{CloudHandler: handleCloudAWSL2VPNConfigUpdate},
	)

	r.registerDestructiveCloudAWS("vmware_cloudaws_network_l2vpn_config_delete",
		"Delete the SDDC L2 VPN configuration for a management or compute gateway (NSX Edge) in VMware Cloud on AWS (DELETE .../sddc/cgws/{edgeId}/l2vpn/config).",
		tier1,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"org":     orgArg,
				"sddc":    sddcArg,
				"edge_id": edgeIDArg,
				"confirm": confirmArg,
			},
			"required": []interface{}{"org", "sddc", "edge_id", "confirm"},
		},
		Tool{CloudHandler: handleCloudAWSL2VPNConfigDelete},
	)

	r.registerCloudAWS("vmware_cloudaws_network_l2vpn_statistics_get",
		"Get L2 VPN statistics for a management or compute gateway (NSX Edge) in VMware Cloud on AWS (GET .../edges/{edgeId}/l2vpn/config/statistics).",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"org":     orgArg,
				"sddc":    sddcArg,
				"edge_id": edgeIDArg,
			},
			"required": []interface{}{"org", "sddc", "edge_id"},
		},
		Tool{CloudHandler: handleCloudAWSL2VPNStatisticsGet},
	)

	// ---- Edge DNS (5) — distinct from the SDDC public/private DNS tools ----

	r.registerCloudAWS("vmware_cloudaws_network_edge_dns_config_get",
		"Get the DNS server configuration for a management or compute gateway (NSX Edge) in VMware Cloud on AWS (GET .../edges/{edgeId}/dns/config). Distinct from the SDDC-level public/private DNS tools.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"org":     orgArg,
				"sddc":    sddcArg,
				"edge_id": edgeIDArg,
			},
			"required": []interface{}{"org", "sddc", "edge_id"},
		},
		Tool{CloudHandler: handleCloudAWSEdgeDNSConfigGet},
	)

	r.registerDestructiveCloudAWS("vmware_cloudaws_network_edge_dns_config_update",
		"Configure the DNS server configuration for a management or compute gateway (NSX Edge) in VMware Cloud on AWS (PUT .../edges/{edgeId}/dns/config).",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"org":     orgArg,
				"sddc":    sddcArg,
				"edge_id": edgeIDArg,
				"config":  configBodyArg("edge DNS relay"),
				"confirm": confirmArg,
			},
			"required": []interface{}{"org", "sddc", "edge_id", "confirm"},
		},
		Tool{CloudHandler: handleCloudAWSEdgeDNSConfigUpdate},
	)

	r.registerDestructiveCloudAWS("vmware_cloudaws_network_edge_dns_status_set",
		"Enable or disable the DNS relay service for a management or compute gateway (NSX Edge) in VMware Cloud on AWS (POST .../edges/{edgeId}/dns/config?enable={bool}).",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"org":     orgArg,
				"sddc":    sddcArg,
				"edge_id": edgeIDArg,
				"enable": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether to enable (true) or disable (false) the DNS relay service on this edge.",
				},
				"confirm": confirmArg,
			},
			"required": []interface{}{"org", "sddc", "edge_id", "enable", "confirm"},
		},
		Tool{CloudHandler: handleCloudAWSEdgeDNSStatusSet},
	)

	r.registerDestructiveCloudAWS("vmware_cloudaws_network_edge_dns_config_delete",
		"Delete the DNS server configuration for a management or compute gateway (NSX Edge) in VMware Cloud on AWS (DELETE .../edges/{edgeId}/dns/config).",
		tier1,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"org":     orgArg,
				"sddc":    sddcArg,
				"edge_id": edgeIDArg,
				"confirm": confirmArg,
			},
			"required": []interface{}{"org", "sddc", "edge_id", "confirm"},
		},
		Tool{CloudHandler: handleCloudAWSEdgeDNSConfigDelete},
	)

	r.registerCloudAWS("vmware_cloudaws_network_edge_dns_statistics_get",
		"Get DNS server statistics for a management or compute gateway (NSX Edge) in VMware Cloud on AWS (GET .../edges/{edgeId}/dns/statistics).",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"org":     orgArg,
				"sddc":    sddcArg,
				"edge_id": edgeIDArg,
			},
			"required": []interface{}{"org", "sddc", "edge_id"},
		},
		Tool{CloudHandler: handleCloudAWSEdgeDNSStatisticsGet},
	)

	// ---- Edge Devices (4) ----------------------------------------------

	r.registerCloudAWS("vmware_cloudaws_network_edge_list",
		"List every management/compute gateway (NSX Edge) in a VMware Cloud on AWS SDDC (GET .../networks/4.0/edges).",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"org":  orgArg,
				"sddc": sddcArg,
			},
			"required": []interface{}{"org", "sddc"},
		},
		Tool{CloudHandler: handleCloudAWSEdgeList},
	)

	r.registerCloudAWS("vmware_cloudaws_network_edge_status_get",
		"Get the status of a management or compute gateway (NSX Edge) in VMware Cloud on AWS (GET .../edges/{edgeId}/status).",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"org":     orgArg,
				"sddc":    sddcArg,
				"edge_id": edgeIDArg,
				"get_latest": map[string]interface{}{
					"type":        "boolean",
					"description": "Optional. When true, forces a fresh status read instead of a cached one. Omitted from the request when not passed.",
				},
				"detailed": map[string]interface{}{
					"type":        "boolean",
					"description": "Optional. When true, returns detailed status information. Omitted from the request when not passed.",
				},
			},
			"required": []interface{}{"org", "sddc", "edge_id"},
		},
		Tool{CloudHandler: handleCloudAWSEdgeStatusGet},
	)

	r.registerCloudAWS("vmware_cloudaws_network_edge_vnics_list",
		"List every network interface (vNIC) for a management or compute gateway (NSX Edge) in VMware Cloud on AWS (GET .../edges/{edgeId}/vnics).",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"org":     orgArg,
				"sddc":    sddcArg,
				"edge_id": edgeIDArg,
			},
			"required": []interface{}{"org", "sddc", "edge_id"},
		},
		Tool{CloudHandler: handleCloudAWSEdgeVNICsList},
	)

	r.registerCloudAWS("vmware_cloudaws_network_edge_peerconfig_list",
		"Get IPsec VPN peer configuration for a management or compute gateway (NSX Edge) in VMware Cloud on AWS (GET .../edges/{edgeId}/peerconfig). The response is free-form text generated per the template specified by template_id.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"org":     orgArg,
				"sddc":    sddcArg,
				"edge_id": edgeIDArg,
				"object_type": map[string]interface{}{
					"type":        "string",
					"description": "Optional. Type of the NSX object the peer configuration is generated for (e.g. an IPsec VPN site). VMC/NSX Edge-defined value, not enumerated here — no live endpoint was available to confirm the accepted set.",
				},
				"object_id": map[string]interface{}{
					"type":        "string",
					"description": "Optional. ID of the NSX object named by object_type.",
				},
				"template_id": map[string]interface{}{
					"type":        "string",
					"description": "Optional. ID of the output template controlling the free-form text format of the returned peer configuration.",
				},
			},
			"required": []interface{}{"org", "sddc", "edge_id"},
		},
		Tool{CloudHandler: handleCloudAWSEdgePeerConfigList},
	)

	// ---- DHCP (1) -------------------------------------------------------

	r.registerCloudAWS("vmware_cloudaws_network_edge_dhcp_leases_list",
		"Get DHCP lease information for a management or compute gateway (NSX Edge) in VMware Cloud on AWS (GET .../edges/{edgeId}/dhcp/leaseInfo).",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"org":     orgArg,
				"sddc":    sddcArg,
				"edge_id": edgeIDArg,
			},
			"required": []interface{}{"org", "sddc", "edge_id"},
		},
		Tool{CloudHandler: handleCloudAWSEdgeDHCPLeasesList},
	)

	// ---- Statistics (5) --------------------------------------------------

	r.registerCloudAWS("vmware_cloudaws_network_edge_stats_dashboard_interface",
		"Get interface dashboard statistics for a management or compute gateway (NSX Edge) in VMware Cloud on AWS (GET .../edges/{edgeId}/statistics/dashboard/interface).",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"org":      orgArg,
				"sddc":     sddcArg,
				"edge_id":  edgeIDArg,
				"interval": intervalArg,
			},
			"required": []interface{}{"org", "sddc", "edge_id"},
		},
		Tool{CloudHandler: handleCloudAWSEdgeStatsDashboardInterface},
	)

	r.registerCloudAWS("vmware_cloudaws_network_edge_stats_dashboard_firewall",
		"Get firewall dashboard statistics for a management or compute gateway (NSX Edge) in VMware Cloud on AWS (GET .../edges/{edgeId}/statistics/dashboard/firewall).",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"org":      orgArg,
				"sddc":     sddcArg,
				"edge_id":  edgeIDArg,
				"interval": intervalArg,
			},
			"required": []interface{}{"org", "sddc", "edge_id"},
		},
		Tool{CloudHandler: handleCloudAWSEdgeStatsDashboardFirewall},
	)

	r.registerCloudAWS("vmware_cloudaws_network_edge_stats_dashboard_ipsec",
		"Get IPsec dashboard statistics for a management or compute gateway (NSX Edge) in VMware Cloud on AWS (GET .../edges/{edgeId}/statistics/dashboard/ipsec).",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"org":      orgArg,
				"sddc":     sddcArg,
				"edge_id":  edgeIDArg,
				"interval": intervalArg,
			},
			"required": []interface{}{"org", "sddc", "edge_id"},
		},
		Tool{CloudHandler: handleCloudAWSEdgeStatsDashboardIPSec},
	)

	r.registerCloudAWS("vmware_cloudaws_network_edge_stats_interfaces",
		"Get interface statistics for a management or compute gateway (NSX Edge) in VMware Cloud on AWS (GET .../edges/{edgeId}/statistics/interfaces).",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"org":        orgArg,
				"sddc":       sddcArg,
				"edge_id":    edgeIDArg,
				"start_time": startTimeArg,
				"end_time":   endTimeArg,
			},
			"required": []interface{}{"org", "sddc", "edge_id"},
		},
		Tool{CloudHandler: handleCloudAWSEdgeStatsInterfacesGet},
	)

	r.registerCloudAWS("vmware_cloudaws_network_edge_stats_interfaces_uplink",
		"Get uplink interface statistics for a management or compute gateway (NSX Edge) in VMware Cloud on AWS (GET .../edges/{edgeId}/statistics/interfaces/uplink).",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"org":        orgArg,
				"sddc":       sddcArg,
				"edge_id":    edgeIDArg,
				"start_time": startTimeArg,
				"end_time":   endTimeArg,
			},
			"required": []interface{}{"org", "sddc", "edge_id"},
		},
		Tool{CloudHandler: handleCloudAWSEdgeStatsInterfacesUplinkGet},
	)

	// ---- Connectivity (2) -------------------------------------------------

	r.registerCloudAWS("vmware_cloudaws_network_connectivity_test_list",
		"List metadata for connectivity tests available on a VMware Cloud on AWS SDDC (GET .../networking/connectivity-tests).",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"org":  orgArg,
				"sddc": sddcArg,
			},
			"required": []interface{}{"org", "sddc"},
		},
		Tool{CloudHandler: handleCloudAWSConnectivityTestList},
	)

	r.registerCloudAWS("vmware_cloudaws_network_connectivity_test_run",
		"Run a connectivity test against a VMware Cloud on AWS SDDC's network (POST .../networking/connectivity-tests?action={action}) — a read-only diagnostic, mutates no network configuration. Per the underlying API's own description, the result (a ConnectivityValidationGroupResultWrapper) is available at task.params['test_result'] of the resulting task.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"org":  orgArg,
				"sddc": sddcArg,
				"action": map[string]interface{}{
					"type":        "string",
					"description": "The connectivity validation action to run (VMC-defined value — not enumerated here, no live endpoint was available to confirm the accepted set).",
				},
				"request": map[string]interface{}{
					"type":                 "object",
					"additionalProperties": true,
					"description":          "Optional connectivity test request payload, sent verbatim as the request body. The vendored Postman collection's own example body is just the placeholder text \"request information\", not real JSON, so no field-level schema could be confirmed — omit to send a bodyless request (some action values may need no body at all).",
				},
			},
			"required": []interface{}{"org", "sddc", "action"},
		},
		Tool{CloudHandler: handleCloudAWSConnectivityTestRun},
	)
}

// --- shared helpers ----------------------------------------------------

// cloudAWSEdgeQueryParam is one optional query-string key/value pair for a
// VMC NSX Edge networking request — see cloudAWSEdgeQuery.
type cloudAWSEdgeQueryParam struct {
	key   string
	value string
}

// cloudAWSEdgeQuery renders params into a URL query string ("?k=v&k2=v2"),
// silently skipping any param whose key or value is empty — this project's
// established convention of never sending a query key the caller didn't
// actually supply, so the VMC API applies its own default instead of this
// project guessing one. Returns "" (no leading "?") when every param was
// empty.
func cloudAWSEdgeQuery(params ...cloudAWSEdgeQueryParam) string {
	q := url.Values{}
	for _, p := range params {
		if p.key == "" || p.value == "" {
			continue
		}
		q.Set(p.key, p.value)
	}
	if len(q) == 0 {
		return ""
	}
	return "?" + q.Encode()
}

// cloudAWSEdgeOptBoolParam reads args[argKey] as a bool and, only if the
// caller actually supplied it (present AND a real bool), returns a query
// param under queryKey — a present-but-false value is preserved (e.g.
// detailed:false must still reach the request), unlike a bare truthiness
// check would allow.
func cloudAWSEdgeOptBoolParam(args map[string]interface{}, argKey, queryKey string) cloudAWSEdgeQueryParam {
	v, ok := args[argKey]
	if !ok {
		return cloudAWSEdgeQueryParam{}
	}
	b, ok := v.(bool)
	if !ok {
		return cloudAWSEdgeQueryParam{}
	}
	return cloudAWSEdgeQueryParam{key: queryKey, value: strconv.FormatBool(b)}
}

// cloudAWSEdgeOptStringParam reads args[argKey] as a string and, if
// non-empty, returns a query param under queryKey.
func cloudAWSEdgeOptStringParam(args map[string]interface{}, argKey, queryKey string) cloudAWSEdgeQueryParam {
	v, _ := args[argKey].(string)
	if v == "" {
		return cloudAWSEdgeQueryParam{}
	}
	return cloudAWSEdgeQueryParam{key: queryKey, value: v}
}

// cloudAWSEdgeRequiredBoolArg reads args[key] as a bool, distinguishing "not
// supplied" from "supplied as false" — unlike a bare type assertion, a
// caller who forgets the argument gets a clear error instead of it silently
// defaulting to false.
func cloudAWSEdgeRequiredBoolArg(args map[string]interface{}, key string) (bool, error) {
	v, ok := args[key]
	if !ok {
		return false, fmt.Errorf("%s is required", key)
	}
	b, ok := v.(bool)
	if !ok {
		return false, fmt.Errorf("%s must be a boolean", key)
	}
	return b, nil
}

// cloudAWSEdgeArgs reads the org/sddc/edge_id triple every edge-scoped tool
// in this file requires. requiredStringArg is shared package-wide (defined
// in generated_library_core.go, Fase 8a) — reused here rather than
// redeclared.
func cloudAWSEdgeArgs(args map[string]interface{}) (org, sddc, edgeID string, err error) {
	if org, err = requiredStringArg(args, "org"); err != nil {
		return
	}
	if sddc, err = requiredStringArg(args, "sddc"); err != nil {
		return
	}
	edgeID, err = requiredStringArg(args, "edge_id")
	return
}

// cloudAWSOrgSDDCArgs is cloudAWSEdgeArgs' counterpart for the 3 routes in
// this file with no edge in their path (edge_list, connectivity-tests
// list/run).
func cloudAWSOrgSDDCArgs(args map[string]interface{}) (org, sddc string, err error) {
	if org, err = requiredStringArg(args, "org"); err != nil {
		return
	}
	sddc, err = requiredStringArg(args, "sddc")
	return
}

// cloudAWSEdgeBasePath builds the common
// /vmc/api/orgs/{org}/sddcs/{sddc}/networks/4.0/edges/{edgeId} prefix shared
// by every route in this file except the L2VPN config PUT/DELETE (which
// uses .../networks/4.0/sddc/cgws/{edgeId}/... instead — confirmed by
// reading the vendored Postman collection directly) and the
// connectivity-tests/edge-list routes, which have no edge segment at all.
func cloudAWSEdgeBasePath(org, sddc, edgeID string) string {
	return "/vmc/api/orgs/" + org + "/sddcs/" + sddc + "/networks/4.0/edges/" + edgeID
}

// cloudAWSEdgeDo issues method against path and JSON-marshals whatever the
// VMC API returned. Every route in this file has an unconfirmed response
// shape (no simulator/live VMC org available to this project), so like
// appliance.go's applianceGet, decoding is deliberately generic
// (interface{}) rather than into a guessed struct.
func cloudAWSEdgeDo(ctx context.Context, client *cloudaws.Client, method, path string, body interface{}) (string, error) {
	var result interface{}
	if err := client.Do(ctx, method, path, body, &result); err != nil {
		return "", err
	}
	return marshalJSON(result)
}

// cloudAWSEdgeDoDelete issues a DELETE against path and, on success, returns
// confirmation (augmented with a "result":"deleted" field) instead of
// whatever body (if any) the DELETE response carried — every DELETE example
// in the vendored Postman collection has an empty response body, so echoing
// back what was actually deleted is more useful than marshaling a
// likely-empty response (same pattern as generated_library_core.go's
// handleLibraryDeleteItem).
func cloudAWSEdgeDoDelete(ctx context.Context, client *cloudaws.Client, path string, confirmation map[string]interface{}) (string, error) {
	if err := client.Do(ctx, http.MethodDelete, path, nil, nil); err != nil {
		return "", err
	}
	confirmation["result"] = "deleted"
	return marshalJSON(confirmation)
}

// --- IPSec handlers ------------------------------------------------------

func handleCloudAWSIPSecConfigGet(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, sddc, edgeID, err := cloudAWSEdgeArgs(args)
	if err != nil {
		return "", err
	}
	path := cloudAWSEdgeBasePath(org, sddc, edgeID) + "/ipsec/config" +
		cloudAWSEdgeQuery(cloudAWSEdgeOptBoolParam(args, "show_sensitive_data", "showSensitiveData"))
	return cloudAWSEdgeDo(ctx, client, http.MethodGet, path, nil)
}

func handleCloudAWSIPSecConfigUpdate(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, sddc, edgeID, err := cloudAWSEdgeArgs(args)
	if err != nil {
		return "", err
	}
	path := cloudAWSEdgeBasePath(org, sddc, edgeID) + "/ipsec/config"
	return cloudAWSEdgeDo(ctx, client, http.MethodPut, path, args["config"])
}

func handleCloudAWSIPSecConfigDelete(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, sddc, edgeID, err := cloudAWSEdgeArgs(args)
	if err != nil {
		return "", err
	}
	path := cloudAWSEdgeBasePath(org, sddc, edgeID) + "/ipsec/config"
	return cloudAWSEdgeDoDelete(ctx, client, path, map[string]interface{}{"org": org, "sddc": sddc, "edge_id": edgeID})
}

func handleCloudAWSIPSecStatisticsGet(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, sddc, edgeID, err := cloudAWSEdgeArgs(args)
	if err != nil {
		return "", err
	}
	path := cloudAWSEdgeBasePath(org, sddc, edgeID) + "/ipsec/statistics"
	return cloudAWSEdgeDo(ctx, client, http.MethodGet, path, nil)
}

// --- L2VPN handlers --------------------------------------------------------

func handleCloudAWSL2VPNConfigUpdate(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, sddc, edgeID, err := cloudAWSEdgeArgs(args)
	if err != nil {
		return "", err
	}
	path := "/vmc/api/orgs/" + org + "/sddcs/" + sddc + "/networks/4.0/sddc/cgws/" + edgeID + "/l2vpn/config" +
		cloudAWSEdgeQuery(cloudAWSEdgeOptBoolParam(args, "show_sensitive_data", "showSensitiveData"))
	return cloudAWSEdgeDo(ctx, client, http.MethodPut, path, args["config"])
}

func handleCloudAWSL2VPNConfigDelete(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, sddc, edgeID, err := cloudAWSEdgeArgs(args)
	if err != nil {
		return "", err
	}
	path := "/vmc/api/orgs/" + org + "/sddcs/" + sddc + "/networks/4.0/sddc/cgws/" + edgeID + "/l2vpn/config"
	return cloudAWSEdgeDoDelete(ctx, client, path, map[string]interface{}{"org": org, "sddc": sddc, "edge_id": edgeID})
}

func handleCloudAWSL2VPNStatisticsGet(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, sddc, edgeID, err := cloudAWSEdgeArgs(args)
	if err != nil {
		return "", err
	}
	path := cloudAWSEdgeBasePath(org, sddc, edgeID) + "/l2vpn/config/statistics"
	return cloudAWSEdgeDo(ctx, client, http.MethodGet, path, nil)
}

// --- Edge DNS handlers -----------------------------------------------------

func handleCloudAWSEdgeDNSConfigGet(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, sddc, edgeID, err := cloudAWSEdgeArgs(args)
	if err != nil {
		return "", err
	}
	path := cloudAWSEdgeBasePath(org, sddc, edgeID) + "/dns/config"
	return cloudAWSEdgeDo(ctx, client, http.MethodGet, path, nil)
}

func handleCloudAWSEdgeDNSConfigUpdate(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, sddc, edgeID, err := cloudAWSEdgeArgs(args)
	if err != nil {
		return "", err
	}
	path := cloudAWSEdgeBasePath(org, sddc, edgeID) + "/dns/config"
	return cloudAWSEdgeDo(ctx, client, http.MethodPut, path, args["config"])
}

func handleCloudAWSEdgeDNSStatusSet(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, sddc, edgeID, err := cloudAWSEdgeArgs(args)
	if err != nil {
		return "", err
	}
	enable, err := cloudAWSEdgeRequiredBoolArg(args, "enable")
	if err != nil {
		return "", err
	}
	path := cloudAWSEdgeBasePath(org, sddc, edgeID) + "/dns/config" +
		cloudAWSEdgeQuery(cloudAWSEdgeQueryParam{key: "enable", value: strconv.FormatBool(enable)})
	return cloudAWSEdgeDo(ctx, client, http.MethodPost, path, nil)
}

func handleCloudAWSEdgeDNSConfigDelete(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, sddc, edgeID, err := cloudAWSEdgeArgs(args)
	if err != nil {
		return "", err
	}
	path := cloudAWSEdgeBasePath(org, sddc, edgeID) + "/dns/config"
	return cloudAWSEdgeDoDelete(ctx, client, path, map[string]interface{}{"org": org, "sddc": sddc, "edge_id": edgeID})
}

func handleCloudAWSEdgeDNSStatisticsGet(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, sddc, edgeID, err := cloudAWSEdgeArgs(args)
	if err != nil {
		return "", err
	}
	path := cloudAWSEdgeBasePath(org, sddc, edgeID) + "/dns/statistics"
	return cloudAWSEdgeDo(ctx, client, http.MethodGet, path, nil)
}

// --- Edge Devices handlers --------------------------------------------------

func handleCloudAWSEdgeList(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, sddc, err := cloudAWSOrgSDDCArgs(args)
	if err != nil {
		return "", err
	}
	path := "/vmc/api/orgs/" + org + "/sddcs/" + sddc + "/networks/4.0/edges"
	return cloudAWSEdgeDo(ctx, client, http.MethodGet, path, nil)
}

func handleCloudAWSEdgeStatusGet(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, sddc, edgeID, err := cloudAWSEdgeArgs(args)
	if err != nil {
		return "", err
	}
	path := cloudAWSEdgeBasePath(org, sddc, edgeID) + "/status" +
		cloudAWSEdgeQuery(
			cloudAWSEdgeOptBoolParam(args, "get_latest", "getlatest"),
			cloudAWSEdgeOptBoolParam(args, "detailed", "detailed"),
		)
	return cloudAWSEdgeDo(ctx, client, http.MethodGet, path, nil)
}

func handleCloudAWSEdgeVNICsList(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, sddc, edgeID, err := cloudAWSEdgeArgs(args)
	if err != nil {
		return "", err
	}
	path := cloudAWSEdgeBasePath(org, sddc, edgeID) + "/vnics"
	return cloudAWSEdgeDo(ctx, client, http.MethodGet, path, nil)
}

func handleCloudAWSEdgePeerConfigList(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, sddc, edgeID, err := cloudAWSEdgeArgs(args)
	if err != nil {
		return "", err
	}
	path := cloudAWSEdgeBasePath(org, sddc, edgeID) + "/peerconfig" +
		cloudAWSEdgeQuery(
			cloudAWSEdgeOptStringParam(args, "object_type", "objecttype"),
			cloudAWSEdgeOptStringParam(args, "object_id", "objectid"),
			cloudAWSEdgeOptStringParam(args, "template_id", "templateid"),
		)
	return cloudAWSEdgeDo(ctx, client, http.MethodGet, path, nil)
}

// --- DHCP handler ------------------------------------------------------------

func handleCloudAWSEdgeDHCPLeasesList(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, sddc, edgeID, err := cloudAWSEdgeArgs(args)
	if err != nil {
		return "", err
	}
	path := cloudAWSEdgeBasePath(org, sddc, edgeID) + "/dhcp/leaseInfo"
	return cloudAWSEdgeDo(ctx, client, http.MethodGet, path, nil)
}

// --- Statistics handlers -----------------------------------------------------

// cloudAWSEdgeStatsDashboard is shared by the 3 nearly-identical
// statistics/dashboard/{interface,firewall,ipsec} routes — section picks
// which one.
func cloudAWSEdgeStatsDashboard(ctx context.Context, client *cloudaws.Client, args map[string]interface{}, section string) (string, error) {
	org, sddc, edgeID, err := cloudAWSEdgeArgs(args)
	if err != nil {
		return "", err
	}
	path := cloudAWSEdgeBasePath(org, sddc, edgeID) + "/statistics/dashboard/" + section +
		cloudAWSEdgeQuery(cloudAWSEdgeOptStringParam(args, "interval", "interval"))
	return cloudAWSEdgeDo(ctx, client, http.MethodGet, path, nil)
}

func handleCloudAWSEdgeStatsDashboardInterface(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	return cloudAWSEdgeStatsDashboard(ctx, client, args, "interface")
}

func handleCloudAWSEdgeStatsDashboardFirewall(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	return cloudAWSEdgeStatsDashboard(ctx, client, args, "firewall")
}

func handleCloudAWSEdgeStatsDashboardIPSec(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	return cloudAWSEdgeStatsDashboard(ctx, client, args, "ipsec")
}

// cloudAWSEdgeStatsInterfaces is shared by statistics/interfaces and
// statistics/interfaces/uplink — pathSuffix picks which one.
func cloudAWSEdgeStatsInterfaces(ctx context.Context, client *cloudaws.Client, args map[string]interface{}, pathSuffix string) (string, error) {
	org, sddc, edgeID, err := cloudAWSEdgeArgs(args)
	if err != nil {
		return "", err
	}
	path := cloudAWSEdgeBasePath(org, sddc, edgeID) + "/statistics/interfaces" + pathSuffix +
		cloudAWSEdgeQuery(
			cloudAWSEdgeOptStringParam(args, "start_time", "startTime"),
			cloudAWSEdgeOptStringParam(args, "end_time", "endTime"),
		)
	return cloudAWSEdgeDo(ctx, client, http.MethodGet, path, nil)
}

func handleCloudAWSEdgeStatsInterfacesGet(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	return cloudAWSEdgeStatsInterfaces(ctx, client, args, "")
}

func handleCloudAWSEdgeStatsInterfacesUplinkGet(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	return cloudAWSEdgeStatsInterfaces(ctx, client, args, "/uplink")
}

// --- Connectivity handlers ----------------------------------------------------

func handleCloudAWSConnectivityTestList(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, sddc, err := cloudAWSOrgSDDCArgs(args)
	if err != nil {
		return "", err
	}
	path := "/vmc/api/orgs/" + org + "/sddcs/" + sddc + "/networking/connectivity-tests"
	return cloudAWSEdgeDo(ctx, client, http.MethodGet, path, nil)
}

func handleCloudAWSConnectivityTestRun(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, sddc, err := cloudAWSOrgSDDCArgs(args)
	if err != nil {
		return "", err
	}
	action, err := requiredStringArg(args, "action")
	if err != nil {
		return "", err
	}
	path := "/vmc/api/orgs/" + org + "/sddcs/" + sddc + "/networking/connectivity-tests" +
		cloudAWSEdgeQuery(cloudAWSEdgeQueryParam{key: "action", value: action})
	return cloudAWSEdgeDo(ctx, client, http.MethodPost, path, args["request"])
}

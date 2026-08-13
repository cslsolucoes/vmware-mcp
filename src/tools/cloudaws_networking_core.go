// Fase 10 "Grupo 3" — VMware Cloud on AWS (VMC) Networking/Networks,
// Networking/Firewall, Networking/Firewall/Rule and Networking/NAT +
// Networking/NAT/Rules, confirmed by parsing `.workspace/VMware Cloud on
// AWS APIs.postman_collection.json` directly (python json.load walk over
// those 5 folders) — 19 tools, not 20: "Networking/NAT/Create" and
// "Networking/NAT/Rules/Create" are the exact same method+path (POST
// .../edges/:edgeId/nat/config/rules), collapsed here into the single
// vmware_cloudaws_network_nat_rule_create tool.
//
// Every route in this file operates under an NSX-T Edge (compute
// gateway/management gateway) managed inside a VMC on AWS SDDC. org/sddc/
// edge_id/network_id/rule_id are opaque string identifiers passed straight
// into the URL — no inventory resolution exists in this project for VMC
// (unlike vmware/*.go's resolveVM/resolveDatastore name-pattern matching
// against a live govmomi inventory), matching this group's brief exactly.
//
// No simulator and no live VMC on AWS organization/SDDC is available to
// this project to verify field names against (see cloudaws/client.go's
// package doc comment) — the vendored Postman collection itself carries no
// example request bodies for any of these routes (confirmed: every
// PUT/POST/DELETE item's request.body is empty), so this file does not
// guess a typed Go struct for firewall rules / NAT rules / logical network
// specs that could silently drop or misname a field. Every write handler
// instead takes the caller's JSON object under the "body" argument and
// passes it through to VMC verbatim — the same "pass the deeply-nested
// spec through, document don't re-model" precedent already used by
// generated_namespace_core.go's config_spec and
// generated_vcenter_template.go's deploymentSpecSchema.
//
// Tier assignments match this group's brief exactly: DELETE routes and the
// two full-config PUT replace routes (firewall/NAT) are tier1 — a full
// config PUT replaces every rule not included in the new body, same
// wholesale-replace risk (and same caution) as the Fase 8b host routing
// table "replace" change_mode; a DELETE on a single rule is irreversible
// (must be recreated from scratch) and a DELETE on a whole config removes
// every rule in the domain at once. POST/PUT on individual resources
// (network create/update, one firewall/NAT rule create/update) are tier2 —
// disruptive but reversible by another call. Every GET is unrestricted.
package tools

import (
	"context"
	"fmt"
	"net/http"

	"github.com/cslsoftwares/mcpvmware/cloudaws"
)

// cloudAWSNetCoreIDNote is appended to every tool description in this file.
const cloudAWSNetCoreIDNote = " org/sddc/edge_id/network_id/rule_id are opaque string identifiers — this project does no inventory resolution for VMC on AWS; obtain them from the VMC Console UI or from a prior list/get call among these tools."

// cloudAWSNetCoreGet issues a GET against path and marshals whatever VMC
// returned (array or object — no simulator/live-verified typed struct
// exists for these responses, so a generic decode is honest rather than a
// guessed struct that could silently drop fields, same posture as
// appliance.go's applianceGet / workstation_network.go's wsGet).
func cloudAWSNetCoreGet(ctx context.Context, client *cloudaws.Client, path string) (string, error) {
	var out interface{}
	if err := client.Do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return "", err
	}
	return marshalJSON(out)
}

// cloudAWSNetCoreMutate issues method (POST/PUT/DELETE) against path with
// body (nil for DELETE) and marshals the response. Some VMC mutation
// endpoints may return no body at all — cloudAWSNetCoreMutate reports a
// plain {"result": "ok"} in that case instead of an empty string, mirroring
// workstation_network.go's wsMutate.
func cloudAWSNetCoreMutate(ctx context.Context, client *cloudaws.Client, method, path string, body interface{}) (string, error) {
	var out interface{}
	if err := client.Do(ctx, method, path, body, &out); err != nil {
		return "", err
	}
	if out == nil {
		return marshalJSON(map[string]interface{}{"result": "ok"})
	}
	return marshalJSON(out)
}

// cloudAWSNetCoreRequiredBody reads args["body"] as a required JSON object —
// shared by every create/update handler in this file. See this file's top
// doc comment for why a generic pass-through, not a typed struct.
func cloudAWSNetCoreRequiredBody(args map[string]interface{}) (interface{}, error) {
	v, ok := args["body"]
	if !ok || v == nil {
		return nil, fmt.Errorf("body is required")
	}
	if _, ok := v.(map[string]interface{}); !ok {
		return nil, fmt.Errorf("body must be a JSON object")
	}
	return v, nil
}

// cloudAWSNetCoreOrgSDDC reads the required org/sddc pair shared by every
// handler in this file.
func cloudAWSNetCoreOrgSDDC(args map[string]interface{}) (org, sddc string, err error) {
	org, err = requiredStringArg(args, "org")
	if err != nil {
		return "", "", err
	}
	sddc, err = requiredStringArg(args, "sddc")
	if err != nil {
		return "", "", err
	}
	return org, sddc, nil
}

// cloudAWSNetCoreOrgSDDCEdge reads org/sddc/edge_id — shared by every
// firewall/NAT handler in this file (both operate under an NSX-T Edge).
func cloudAWSNetCoreOrgSDDCEdge(args map[string]interface{}) (org, sddc, edge string, err error) {
	org, sddc, err = cloudAWSNetCoreOrgSDDC(args)
	if err != nil {
		return "", "", "", err
	}
	edge, err = requiredStringArg(args, "edge_id")
	if err != nil {
		return "", "", "", err
	}
	return org, sddc, edge, nil
}

// cloudAWSNetworksBasePath is the sddc/networks collection path (no
// trailing slash) — List appends "/", Create uses it bare, Get/Update/
// Delete append "/"+networkID, matching the vendored Postman collection's
// literal paths exactly (List's item has a trailing slash, Create's does
// not).
func cloudAWSNetworksBasePath(org, sddc string) string {
	return "/vmc/api/orgs/" + org + "/sddcs/" + sddc + "/networks/4.0/sddc/networks"
}

// cloudAWSFirewallConfigPath is one NSX-T Edge's firewall config path —
// List/Update/Delete use it bare; the rule sub-resource and its own List/
// Get/Create/Update/Delete/Stats append "/rules"[/ruleID].
func cloudAWSFirewallConfigPath(org, sddc, edge string) string {
	return "/vmc/api/orgs/" + org + "/sddcs/" + sddc + "/networks/4.0/edges/" + edge + "/firewall/config"
}

// cloudAWSFirewallStatsPath is one firewall rule's statistics path — a
// sibling of .../firewall/config, not a sub-path of it (confirmed in the
// vendored Postman collection: GET .../firewall/statistics/:ruleId).
func cloudAWSFirewallStatsPath(org, sddc, edge, ruleID string) string {
	return "/vmc/api/orgs/" + org + "/sddcs/" + sddc + "/networks/4.0/edges/" + edge + "/firewall/statistics/" + ruleID
}

// cloudAWSNatConfigPath is one NSX-T Edge's NAT config path — List/Update/
// Delete use it bare; the rule sub-resource appends "/rules"[/ruleID].
func cloudAWSNatConfigPath(org, sddc, edge string) string {
	return "/vmc/api/orgs/" + org + "/sddcs/" + sddc + "/networks/4.0/edges/" + edge + "/nat/config"
}

// registerCloudAWSNetworkingCoreTools registers this group's 19 tools:
// Networks (5), Firewall config+rules (8), NAT config+rules (6).
func registerCloudAWSNetworkingCoreTools(r *Registry) {
	orgArg := map[string]interface{}{"type": "string", "description": "VMC organization ID (opaque string, e.g. from the VMC Console URL)." + cloudAWSNetCoreIDNote}
	sddcArg := map[string]interface{}{"type": "string", "description": "SDDC identifier within org (opaque string)." + cloudAWSNetCoreIDNote}
	edgeIDArg := map[string]interface{}{"type": "string", "description": "NSX-T Edge (compute gateway/management gateway) identifier within the SDDC (opaque string)." + cloudAWSNetCoreIDNote}
	networkIDArg := map[string]interface{}{"type": "string", "description": "Logical network identifier (opaque string), as returned by vmware_cloudaws_network_list." + cloudAWSNetCoreIDNote}
	firewallRuleIDArg := map[string]interface{}{"type": "string", "description": "Firewall rule identifier (opaque string), as returned by vmware_cloudaws_network_firewall_list." + cloudAWSNetCoreIDNote}
	natRuleIDArg := map[string]interface{}{"type": "string", "description": "NAT rule identifier (opaque string), as returned by vmware_cloudaws_network_nat_list." + cloudAWSNetCoreIDNote}
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}

	// --- Networks (5) -------------------------------------------------------

	r.registerCloudAWS("vmware_cloudaws_network_list",
		"List logical networks defined in a VMware Cloud on AWS SDDC (GET .../networks/4.0/sddc/networks/).",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"org": orgArg, "sddc": sddcArg},
			"required":   []interface{}{"org", "sddc"},
		},
		Tool{CloudHandler: handleCloudAWSNetworkList},
	)

	r.registerCloudAWS("vmware_cloudaws_network_get",
		"Get details of one logical network in a VMware Cloud on AWS SDDC (GET .../networks/4.0/sddc/networks/{networkId}).",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"org": orgArg, "sddc": sddcArg, "network_id": networkIDArg},
			"required":   []interface{}{"org", "sddc", "network_id"},
		},
		Tool{CloudHandler: handleCloudAWSNetworkGet},
	)

	r.registerDestructiveCloudAWS("vmware_cloudaws_network_create",
		"Create a logical network in a VMware Cloud on AWS SDDC (POST .../networks/4.0/sddc/networks). Disruptive but reversible — the created network can be removed again with vmware_cloudaws_network_delete.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"org":  orgArg,
				"sddc": sddcArg,
				"body": map[string]interface{}{
					"type":        "object",
					"description": "Full JSON request body for the new logical network, passed through verbatim — VMware Cloud on AWS Networking API's sddc/networks JSON shape (typically name, subnets, l2Extension, etc. depending on network type). This project has no simulator or live SDDC to verify field names against; see VMware's official VMC on AWS Networking REST API documentation for the exact schema.",
				},
				"confirm": confirmArg,
			},
			"required": []interface{}{"org", "sddc", "body", "confirm"},
		},
		Tool{CloudHandler: handleCloudAWSNetworkCreate},
	)

	r.registerDestructiveCloudAWS("vmware_cloudaws_network_update",
		"Update a logical network in a VMware Cloud on AWS SDDC (PUT .../networks/4.0/sddc/networks/{networkId}).",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"org":        orgArg,
				"sddc":       sddcArg,
				"network_id": networkIDArg,
				"body": map[string]interface{}{
					"type":        "object",
					"description": "Full JSON request body with the network's new state, passed through verbatim — same shape as vmware_cloudaws_network_create's body. This project has no simulator or live SDDC to verify field names against.",
				},
				"confirm": confirmArg,
			},
			"required": []interface{}{"org", "sddc", "network_id", "body", "confirm"},
		},
		Tool{CloudHandler: handleCloudAWSNetworkUpdate},
	)

	r.registerDestructiveCloudAWS("vmware_cloudaws_network_delete",
		"Delete a logical network from a VMware Cloud on AWS SDDC (DELETE .../networks/4.0/sddc/networks/{networkId}). Irreversible — VMs still attached to this network lose connectivity, and the network must be recreated from scratch.",
		tier1,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"org":        orgArg,
				"sddc":       sddcArg,
				"network_id": networkIDArg,
				"confirm":    confirmArg,
			},
			"required": []interface{}{"org", "sddc", "network_id", "confirm"},
		},
		Tool{CloudHandler: handleCloudAWSNetworkDelete},
	)

	// --- Firewall (8) --------------------------------------------------------

	r.registerCloudAWS("vmware_cloudaws_network_firewall_list",
		"List the full firewall configuration (every rule) of an SDDC's NSX-T Edge (GET .../edges/{edgeId}/firewall/config).",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"org": orgArg, "sddc": sddcArg, "edge_id": edgeIDArg},
			"required":   []interface{}{"org", "sddc", "edge_id"},
		},
		Tool{CloudHandler: handleCloudAWSFirewallList},
	)

	r.registerDestructiveCloudAWS("vmware_cloudaws_network_firewall_update",
		"Replace the ENTIRE firewall configuration of an SDDC's NSX-T Edge in one call (PUT .../edges/{edgeId}/firewall/config) — every rule not included in the new body is removed. High risk of cutting off management/VM access if the new config omits a rule still needed; same wholesale-replace caution already applied to the Fase 8b host routing table \"replace\" change_mode.",
		tier1,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"org":     orgArg,
				"sddc":    sddcArg,
				"edge_id": edgeIDArg,
				"body": map[string]interface{}{
					"type":        "object",
					"description": "Full JSON firewall configuration to install verbatim, REPLACING every existing rule — NSX-T Edge firewall config JSON shape (typically a wrapper object holding a firewallRules array, per VMware's Advanced Firewall API). Passed through as-is; this project has no simulator or live SDDC to verify field names against.",
				},
				"confirm": confirmArg,
			},
			"required": []interface{}{"org", "sddc", "edge_id", "body", "confirm"},
		},
		Tool{CloudHandler: handleCloudAWSFirewallUpdate},
	)

	r.registerDestructiveCloudAWS("vmware_cloudaws_network_firewall_delete",
		"Delete the ENTIRE firewall configuration of an SDDC's NSX-T Edge (DELETE .../edges/{edgeId}/firewall/config). Irreversible and removes every rule at once — very high risk of cutting off all network access to/from the SDDC.",
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
		Tool{CloudHandler: handleCloudAWSFirewallDelete},
	)

	r.registerCloudAWS("vmware_cloudaws_network_firewall_rule_get",
		"Get details of one firewall rule on an SDDC's NSX-T Edge (GET .../edges/{edgeId}/firewall/config/rules/{ruleId}).",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"org": orgArg, "sddc": sddcArg, "edge_id": edgeIDArg, "rule_id": firewallRuleIDArg},
			"required":   []interface{}{"org", "sddc", "edge_id", "rule_id"},
		},
		Tool{CloudHandler: handleCloudAWSFirewallRuleGet},
	)

	r.registerDestructiveCloudAWS("vmware_cloudaws_network_firewall_rule_create",
		"Create a firewall rule on an SDDC's NSX-T Edge (POST .../edges/{edgeId}/firewall/config/rules). Disruptive but reversible — the created rule can be removed again with vmware_cloudaws_network_firewall_rule_delete.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"org":     orgArg,
				"sddc":    sddcArg,
				"edge_id": edgeIDArg,
				"body": map[string]interface{}{
					"type":        "object",
					"description": "Full JSON body for the new firewall rule, passed through verbatim — NSX-T Edge firewall rule JSON shape (typically ruleType/action/enabled/source/destination/application, per VMware's Advanced Firewall Rule API). This project has no simulator or live SDDC to verify field names against.",
				},
				"confirm": confirmArg,
			},
			"required": []interface{}{"org", "sddc", "edge_id", "body", "confirm"},
		},
		Tool{CloudHandler: handleCloudAWSFirewallRuleCreate},
	)

	r.registerDestructiveCloudAWS("vmware_cloudaws_network_firewall_rule_update",
		"Update one firewall rule on an SDDC's NSX-T Edge (PUT .../edges/{edgeId}/firewall/config/rules/{ruleId}).",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"org":     orgArg,
				"sddc":    sddcArg,
				"edge_id": edgeIDArg,
				"rule_id": firewallRuleIDArg,
				"body": map[string]interface{}{
					"type":        "object",
					"description": "Full JSON body with the rule's new state, passed through verbatim — same shape as vmware_cloudaws_network_firewall_rule_create's body. This project has no simulator or live SDDC to verify field names against.",
				},
				"confirm": confirmArg,
			},
			"required": []interface{}{"org", "sddc", "edge_id", "rule_id", "body", "confirm"},
		},
		Tool{CloudHandler: handleCloudAWSFirewallRuleUpdate},
	)

	r.registerDestructiveCloudAWS("vmware_cloudaws_network_firewall_rule_delete",
		"Delete one firewall rule from an SDDC's NSX-T Edge (DELETE .../edges/{edgeId}/firewall/config/rules/{ruleId}). Irreversible — the rule must be recreated from scratch, and connectivity it allowed/denied changes immediately.",
		tier1,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"org":     orgArg,
				"sddc":    sddcArg,
				"edge_id": edgeIDArg,
				"rule_id": firewallRuleIDArg,
				"confirm": confirmArg,
			},
			"required": []interface{}{"org", "sddc", "edge_id", "rule_id", "confirm"},
		},
		Tool{CloudHandler: handleCloudAWSFirewallRuleDelete},
	)

	r.registerCloudAWS("vmware_cloudaws_network_firewall_rule_stats",
		"Get hit/traffic statistics for one firewall rule on an SDDC's NSX-T Edge (GET .../edges/{edgeId}/firewall/statistics/{ruleId}).",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"org": orgArg, "sddc": sddcArg, "edge_id": edgeIDArg, "rule_id": firewallRuleIDArg},
			"required":   []interface{}{"org", "sddc", "edge_id", "rule_id"},
		},
		Tool{CloudHandler: handleCloudAWSFirewallRuleStats},
	)

	// --- NAT (6, after dedup) -------------------------------------------------

	r.registerCloudAWS("vmware_cloudaws_network_nat_list",
		"List the full NAT configuration (every rule) of an SDDC's NSX-T Edge (GET .../edges/{edgeId}/nat/config).",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"org": orgArg, "sddc": sddcArg, "edge_id": edgeIDArg},
			"required":   []interface{}{"org", "sddc", "edge_id"},
		},
		Tool{CloudHandler: handleCloudAWSNatList},
	)

	r.registerDestructiveCloudAWS("vmware_cloudaws_network_nat_update",
		"Replace the ENTIRE NAT configuration of an SDDC's NSX-T Edge in one call (PUT .../edges/{edgeId}/nat/config) — every rule not included in the new body is removed. Same wholesale-replace caution as vmware_cloudaws_network_firewall_update: high risk of breaking inbound/outbound address translation for the whole SDDC.",
		tier1,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"org":     orgArg,
				"sddc":    sddcArg,
				"edge_id": edgeIDArg,
				"body": map[string]interface{}{
					"type":        "object",
					"description": "Full JSON NAT configuration to install verbatim, REPLACING every existing rule — NSX-T Edge NAT config JSON shape (typically a wrapper object holding a natRules array, per VMware's Advanced NAT API). Passed through as-is; this project has no simulator or live SDDC to verify field names against.",
				},
				"confirm": confirmArg,
			},
			"required": []interface{}{"org", "sddc", "edge_id", "body", "confirm"},
		},
		Tool{CloudHandler: handleCloudAWSNatUpdate},
	)

	r.registerDestructiveCloudAWS("vmware_cloudaws_network_nat_delete",
		"Delete the ENTIRE NAT configuration of an SDDC's NSX-T Edge (DELETE .../edges/{edgeId}/nat/config). Irreversible and removes every NAT rule at once.",
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
		Tool{CloudHandler: handleCloudAWSNatDelete},
	)

	r.registerDestructiveCloudAWS("vmware_cloudaws_network_nat_rule_create",
		"Create a NAT rule on an SDDC's NSX-T Edge (POST .../edges/{edgeId}/nat/config/rules). This exact method+path appears twice in the vendored Postman collection, under \"NAT/Create\" and \"NAT/Rules/Create\" — collapsed here into this single tool. Disruptive but reversible — the created rule can be removed again with vmware_cloudaws_network_nat_rule_delete.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"org":     orgArg,
				"sddc":    sddcArg,
				"edge_id": edgeIDArg,
				"body": map[string]interface{}{
					"type":        "object",
					"description": "Full JSON body for the new NAT rule, passed through verbatim — NSX-T Edge NAT rule JSON shape (typically ruleType/action/vnic/originalAddress/translatedAddress/protocol, per VMware's Advanced NAT Rule API). This project has no simulator or live SDDC to verify field names against.",
				},
				"confirm": confirmArg,
			},
			"required": []interface{}{"org", "sddc", "edge_id", "body", "confirm"},
		},
		Tool{CloudHandler: handleCloudAWSNatRuleCreate},
	)

	r.registerDestructiveCloudAWS("vmware_cloudaws_network_nat_rule_update",
		"Update a NAT rule on an SDDC's NSX-T Edge (PUT .../edges/{edgeId}/nat/config/rules/{ruleId}).",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"org":     orgArg,
				"sddc":    sddcArg,
				"edge_id": edgeIDArg,
				"rule_id": natRuleIDArg,
				"body": map[string]interface{}{
					"type":        "object",
					"description": "Full JSON body with the rule's new state, passed through verbatim — same shape as vmware_cloudaws_network_nat_rule_create's body. This project has no simulator or live SDDC to verify field names against.",
				},
				"confirm": confirmArg,
			},
			"required": []interface{}{"org", "sddc", "edge_id", "rule_id", "body", "confirm"},
		},
		Tool{CloudHandler: handleCloudAWSNatRuleUpdate},
	)

	r.registerDestructiveCloudAWS("vmware_cloudaws_network_nat_rule_delete",
		"Delete a NAT rule from an SDDC's NSX-T Edge (DELETE .../edges/{edgeId}/nat/config/rules/{ruleId}). Irreversible — the rule must be recreated from scratch.",
		tier1,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"org":     orgArg,
				"sddc":    sddcArg,
				"edge_id": edgeIDArg,
				"rule_id": natRuleIDArg,
				"confirm": confirmArg,
			},
			"required": []interface{}{"org", "sddc", "edge_id", "rule_id", "confirm"},
		},
		Tool{CloudHandler: handleCloudAWSNatRuleDelete},
	)
}

// --- Networks handlers -------------------------------------------------------

func handleCloudAWSNetworkList(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, sddc, err := cloudAWSNetCoreOrgSDDC(args)
	if err != nil {
		return "", err
	}
	return cloudAWSNetCoreGet(ctx, client, cloudAWSNetworksBasePath(org, sddc)+"/")
}

func handleCloudAWSNetworkGet(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, sddc, err := cloudAWSNetCoreOrgSDDC(args)
	if err != nil {
		return "", err
	}
	networkID, err := requiredStringArg(args, "network_id")
	if err != nil {
		return "", err
	}
	return cloudAWSNetCoreGet(ctx, client, cloudAWSNetworksBasePath(org, sddc)+"/"+networkID)
}

func handleCloudAWSNetworkCreate(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, sddc, err := cloudAWSNetCoreOrgSDDC(args)
	if err != nil {
		return "", err
	}
	body, err := cloudAWSNetCoreRequiredBody(args)
	if err != nil {
		return "", err
	}
	return cloudAWSNetCoreMutate(ctx, client, http.MethodPost, cloudAWSNetworksBasePath(org, sddc), body)
}

func handleCloudAWSNetworkUpdate(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, sddc, err := cloudAWSNetCoreOrgSDDC(args)
	if err != nil {
		return "", err
	}
	networkID, err := requiredStringArg(args, "network_id")
	if err != nil {
		return "", err
	}
	body, err := cloudAWSNetCoreRequiredBody(args)
	if err != nil {
		return "", err
	}
	return cloudAWSNetCoreMutate(ctx, client, http.MethodPut, cloudAWSNetworksBasePath(org, sddc)+"/"+networkID, body)
}

func handleCloudAWSNetworkDelete(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, sddc, err := cloudAWSNetCoreOrgSDDC(args)
	if err != nil {
		return "", err
	}
	networkID, err := requiredStringArg(args, "network_id")
	if err != nil {
		return "", err
	}
	return cloudAWSNetCoreMutate(ctx, client, http.MethodDelete, cloudAWSNetworksBasePath(org, sddc)+"/"+networkID, nil)
}

// --- Firewall handlers -------------------------------------------------------

func handleCloudAWSFirewallList(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, sddc, edge, err := cloudAWSNetCoreOrgSDDCEdge(args)
	if err != nil {
		return "", err
	}
	return cloudAWSNetCoreGet(ctx, client, cloudAWSFirewallConfigPath(org, sddc, edge))
}

func handleCloudAWSFirewallUpdate(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, sddc, edge, err := cloudAWSNetCoreOrgSDDCEdge(args)
	if err != nil {
		return "", err
	}
	body, err := cloudAWSNetCoreRequiredBody(args)
	if err != nil {
		return "", err
	}
	return cloudAWSNetCoreMutate(ctx, client, http.MethodPut, cloudAWSFirewallConfigPath(org, sddc, edge), body)
}

func handleCloudAWSFirewallDelete(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, sddc, edge, err := cloudAWSNetCoreOrgSDDCEdge(args)
	if err != nil {
		return "", err
	}
	return cloudAWSNetCoreMutate(ctx, client, http.MethodDelete, cloudAWSFirewallConfigPath(org, sddc, edge), nil)
}

func handleCloudAWSFirewallRuleGet(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, sddc, edge, err := cloudAWSNetCoreOrgSDDCEdge(args)
	if err != nil {
		return "", err
	}
	ruleID, err := requiredStringArg(args, "rule_id")
	if err != nil {
		return "", err
	}
	return cloudAWSNetCoreGet(ctx, client, cloudAWSFirewallConfigPath(org, sddc, edge)+"/rules/"+ruleID)
}

func handleCloudAWSFirewallRuleCreate(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, sddc, edge, err := cloudAWSNetCoreOrgSDDCEdge(args)
	if err != nil {
		return "", err
	}
	body, err := cloudAWSNetCoreRequiredBody(args)
	if err != nil {
		return "", err
	}
	return cloudAWSNetCoreMutate(ctx, client, http.MethodPost, cloudAWSFirewallConfigPath(org, sddc, edge)+"/rules", body)
}

func handleCloudAWSFirewallRuleUpdate(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, sddc, edge, err := cloudAWSNetCoreOrgSDDCEdge(args)
	if err != nil {
		return "", err
	}
	ruleID, err := requiredStringArg(args, "rule_id")
	if err != nil {
		return "", err
	}
	body, err := cloudAWSNetCoreRequiredBody(args)
	if err != nil {
		return "", err
	}
	return cloudAWSNetCoreMutate(ctx, client, http.MethodPut, cloudAWSFirewallConfigPath(org, sddc, edge)+"/rules/"+ruleID, body)
}

func handleCloudAWSFirewallRuleDelete(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, sddc, edge, err := cloudAWSNetCoreOrgSDDCEdge(args)
	if err != nil {
		return "", err
	}
	ruleID, err := requiredStringArg(args, "rule_id")
	if err != nil {
		return "", err
	}
	return cloudAWSNetCoreMutate(ctx, client, http.MethodDelete, cloudAWSFirewallConfigPath(org, sddc, edge)+"/rules/"+ruleID, nil)
}

func handleCloudAWSFirewallRuleStats(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, sddc, edge, err := cloudAWSNetCoreOrgSDDCEdge(args)
	if err != nil {
		return "", err
	}
	ruleID, err := requiredStringArg(args, "rule_id")
	if err != nil {
		return "", err
	}
	return cloudAWSNetCoreGet(ctx, client, cloudAWSFirewallStatsPath(org, sddc, edge, ruleID))
}

// --- NAT handlers -------------------------------------------------------------

func handleCloudAWSNatList(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, sddc, edge, err := cloudAWSNetCoreOrgSDDCEdge(args)
	if err != nil {
		return "", err
	}
	return cloudAWSNetCoreGet(ctx, client, cloudAWSNatConfigPath(org, sddc, edge))
}

func handleCloudAWSNatUpdate(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, sddc, edge, err := cloudAWSNetCoreOrgSDDCEdge(args)
	if err != nil {
		return "", err
	}
	body, err := cloudAWSNetCoreRequiredBody(args)
	if err != nil {
		return "", err
	}
	return cloudAWSNetCoreMutate(ctx, client, http.MethodPut, cloudAWSNatConfigPath(org, sddc, edge), body)
}

func handleCloudAWSNatDelete(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, sddc, edge, err := cloudAWSNetCoreOrgSDDCEdge(args)
	if err != nil {
		return "", err
	}
	return cloudAWSNetCoreMutate(ctx, client, http.MethodDelete, cloudAWSNatConfigPath(org, sddc, edge), nil)
}

func handleCloudAWSNatRuleCreate(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, sddc, edge, err := cloudAWSNetCoreOrgSDDCEdge(args)
	if err != nil {
		return "", err
	}
	body, err := cloudAWSNetCoreRequiredBody(args)
	if err != nil {
		return "", err
	}
	return cloudAWSNetCoreMutate(ctx, client, http.MethodPost, cloudAWSNatConfigPath(org, sddc, edge)+"/rules", body)
}

func handleCloudAWSNatRuleUpdate(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, sddc, edge, err := cloudAWSNetCoreOrgSDDCEdge(args)
	if err != nil {
		return "", err
	}
	ruleID, err := requiredStringArg(args, "rule_id")
	if err != nil {
		return "", err
	}
	body, err := cloudAWSNetCoreRequiredBody(args)
	if err != nil {
		return "", err
	}
	return cloudAWSNetCoreMutate(ctx, client, http.MethodPut, cloudAWSNatConfigPath(org, sddc, edge)+"/rules/"+ruleID, body)
}

func handleCloudAWSNatRuleDelete(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, sddc, edge, err := cloudAWSNetCoreOrgSDDCEdge(args)
	if err != nil {
		return "", err
	}
	ruleID, err := requiredStringArg(args, "rule_id")
	if err != nil {
		return "", err
	}
	return cloudAWSNetCoreMutate(ctx, client, http.MethodDelete, cloudAWSNatConfigPath(org, sddc, edge)+"/rules/"+ruleID, nil)
}

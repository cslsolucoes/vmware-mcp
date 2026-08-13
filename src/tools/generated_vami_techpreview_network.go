package tools

import (
	"context"
	"fmt"
	"net/http"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// registerVAMITechpreviewNetworkTools is group "G3" of Fase 8b of the codegen
// plan (".workspace/plans/MCPVMWare2026-08-10-175300-plano-cobertura-completa-api-codegen.plan.md")
// — 27 VAMI (Virtual Appliance Management Interface, vCenter Server
// Appliance administration) routes under rest/appliance/techpreview/...:
// firewall inbound address rules, IPv4, IPv6, NTP, HTTP/FTP proxy, static
// routes, and time-sync mode.
//
// # A genuinely new technique for this project
//
// Every prior fase (0-8a) generated tools from a typed Go SDK — either
// govmomi's vim25/object SOAP wrappers, or (Fase 8a) its vapi/*/*.go REST
// wrappers with native json-tagged structs. These 27 routes have NEITHER:
// confirmed by grepping both this repo (0 hits) and the pinned dependency
// actually resolved by src/go.mod/go.sum, github.com/vmware/govmomi v0.55.1
// (`grep -ril techpreview $(go env GOMODCACHE)/github.com/vmware/govmomi@v0.55.1`,
// 0 hits; `vapi/appliance/networking/` only has proxy.go, the Fase 8a
// generated_appliance_small.go NON-techpreview proxy — see below). The
// entire route table below was instead confirmed by structurally parsing
// (Python json.load, not eyeballing) the vendored Postman collection
// ".workspace/vSphere Automation REST Resources for
// appliance.postman_collection.json"'s 7 "Techpreview - *" top-level
// folders (Firewall, IPv4, IPv6, NTP, Proxy, Routes, Timesync) — both the
// method+path of every request AND its literal example request body (used
// below to shape every tool's JSON input schema honestly, field-by-field,
// instead of guessing).
//
// Architecturally this file extends Fase 4's tools/appliance.go pattern
// (client.REST(ctx) → rc.Resource(path).Request(method, body...) →
// rc.Do(ctx, req, &result), generic interface{} decode both ways) rather
// than Fase 8a's typed-struct pattern (generated_appliance_small.go et al.)
// — there is no struct to decode into. See applianceTechpreviewDo below,
// the direct generalization of appliance.go's applianceGet to also support
// POST/PUT with a JSON body.
//
// # TECH PREVIEW — undocumented, unstable, register anyway
//
// "techpreview" in the URL path is VMware's own label for a REST API that
// is not part of the public/supported vSphere Automation API surface and
// may change or disappear without notice between vCenter versions — the
// same caution class already given to the urn:internalvim25 SOAP methods
// AuthorizationManager.DisableMethods/EnableMethods in Fase 7's
// generated_authorization.go. Per that same precedent (and the plan's
// explicit instruction for this group), every tool below is registered
// anyway — the goal is 100% surface coverage, with the instability called
// out plainly in every tool's description (see techPreviewWarning) so a
// caller can make an informed choice.
//
// # Curation decisions
//
//   - Tiers (sem-tier / tier1 / tier2) are exactly the ones pre-decided in
//     this group's brief, not re-derived here: tier1 for the 4 routes that
//     are either an outright delete-by-criteria (firewall rule delete, NTP
//     server delete, proxy delete, route delete) or a full unconditional
//     replacement of a security-relevant list with no cheap undo (firewall
//     "replace all rules" — a wrong PUT here can lock the appliance off the
//     network with no easy recovery path); tier2 for every other mutation
//     (create/add/set/renew — reversible by calling the tool again with the
//     old values); sem-tier for every GET and every "test" endpoint (test
//     endpoints, confirmed from their own request bodies, only exercise
//     connectivity — they carry no config-mutating field).
//   - Naming: <namespace>_<verb>, verb chosen from the route's own REST
//     semantics rather than a generic CRUD word, matching this project's
//     established habit of following the source's own vocabulary (e.g.
//     ipv4_details/ipv4_renew, not a generic "ipv4_post"). Where Postman's
//     own item name already used a clear verb ("List", "Set", "Create rule",
//     "Delete rule", "Renew", "Details/get") it is kept; the bare top-level
//     GETs (IPv4/IPv6/NTP/Proxy/Routes/Timesync "index" requests, named just
//     "IPv4"/"Timesync"/etc. in Postman) are named _get or _list depending on
//     whether the resource is single (ipv4_get, ipv6_get, ntp_get,
//     proxy_get, timesync_get) or a collection (firewall_list, routes_list).
//   - `/appliance/techpreview/networking/proxy` (this file) is a DIFFERENT
//     route from the non-techpreview `/appliance/networking/proxy` already
//     covered read-only in Fase 8a's generated_appliance_small.go
//     (vmware_appliance_networking_proxy_list, wrapping
//     object.appliance.networking.NoProxy/ProxyList) — confirmed by path
//     and by both being present, unrelated, in the same Postman collection.
//     Not a duplicate; both are registered.
//   - Every nested JSON schema below mirrors the Postman example body's
//     field set field-for-field — no field was invented. Where a field's
//     accepted value set could not be confirmed from the one example given
//     (e.g. ipv4 config "mode": only "dhcp" appears in the example; proxy
//     "protocol": only "ftp" appears; proxy "status": only "disabled"
//     appears; timesync "mode": only "host" appears), the schema leaves it
//     as an unconstrained string with a description naming the one
//     confirmed example value, rather than guessing a full enum — same
//     "generic decode is more honest than a guessed struct" posture as
//     appliance.go's applianceGet doc comment. The one field enforced as a
//     real JSON Schema enum, firewall rule "policy", is safe to constrain:
//     both "allow" and "deny" are directly evidenced across 2 different
//     example bodies in the same collection (Create rule uses "allow";
//     Replace all rules uses one of each).
//   - "pos" (firewall Create rule) and "position"/"all" (firewall Delete
//     rule config) are left optional in the schema — their requiredness
//     could not be confirmed from the one example body available, so the
//     honest choice is to let the appliance itself reject a missing/
//     conflicting combination (surfaced verbatim through
//     applianceTechpreviewDo's error wrap) rather than this project
//     inventing its own validation rule not evidenced anywhere.
//
// # Testing: httptest fixture, not vcsim — same reasoning as Fase 4
//
// Per this group's brief and the precedent already established in
// tools/appliance_test.go / cerebrum.md's 2026-08-10 entry:
// vmware.Client.REST(ctx) only wraps the embedded *vim25.Client's HTTP
// transport — it never touches ServiceContent or does any SOAP round trip.
// That means VAMI REST tools are fully testable against a bare
// httptest.Server fixture (session endpoint + canned techpreview routes),
// with no vcsim/simulator involved at all — unlike Fase 8a's
// generated_appliance_small.go (typed vapi/appliance/* Managers), which
// DOES run through this project's newSimClient/vcsim harness but only ever
// reaches "assertReachesServer" (no vapi/simulator handler exists for any
// VAMI route). The fixture approach here is strictly stronger: it lets each
// test assert real response *content*, not just "the call reached a
// server". See generated_vami_techpreview_network_test.go.
func registerVAMITechpreviewNetworkTools(r *Registry) {
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}
	emptySchema := map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
	requiresVCSA := " Requires a vCenter Server Appliance — fails against a standalone ESXi host, which has no VAMI (Virtual Appliance Management Interface)."
	techPreview := " TECH PREVIEW API (rest/appliance/techpreview/... namespace) — undocumented and unstable, not part of the public/supported vSphere Automation API surface, and subject to change or removal without notice between vCenter versions. Use with caution."

	interfacesArraySchema := map[string]interface{}{
		"type":        "array",
		"items":       map[string]interface{}{"type": "string"},
		"description": "Network interface names to query, e.g. [\"nic0\"].",
	}
	serversArraySchema := map[string]interface{}{
		"type":        "array",
		"items":       map[string]interface{}{"type": "string"},
		"description": "NTP server hostnames or IP addresses.",
	}

	firewallRuleSchema := map[string]interface{}{
		"type":        "object",
		"description": "Inbound firewall address rule.",
		"properties": map[string]interface{}{
			"address":        map[string]interface{}{"type": "string", "description": "IPv4 or IPv6 network address the rule matches, e.g. \"192.168.1.0\"."},
			"prefix":         map[string]interface{}{"type": "integer", "description": "CIDR prefix length for address, e.g. 24 for a /24 network."},
			"interface_name": map[string]interface{}{"type": "string", "description": "Network interface the rule applies to, or \"*\" for all interfaces. Default \"*\"."},
			"policy":         map[string]interface{}{"type": "string", "enum": []interface{}{"allow", "deny"}, "description": "Whether matching inbound traffic is allowed or denied."},
		},
		"required": []interface{}{"address", "prefix", "policy"},
	}

	ipv4ConfigItemSchema := map[string]interface{}{
		"type":        "object",
		"description": "IPv4 configuration for one network interface.",
		"properties": map[string]interface{}{
			"interface_name":  map[string]interface{}{"type": "string", "description": "Network interface name, e.g. \"nic0\"."},
			"mode":            map[string]interface{}{"type": "string", "description": "Address assignment mode. Only \"dhcp\" is confirmed from this API's own example; other appliance-defined modes (e.g. a static mode) are not confirmed against this specific techpreview endpoint."},
			"address":         map[string]interface{}{"type": "string", "description": "Static IPv4 address. Relevant when mode is not dhcp."},
			"prefix":          map[string]interface{}{"type": "integer", "description": "CIDR prefix length for address. Relevant when mode is not dhcp."},
			"default_gateway": map[string]interface{}{"type": "string", "description": "Default gateway IPv4 address."},
		},
		"required": []interface{}{"interface_name", "mode"},
	}

	ipv6ConfigItemSchema := map[string]interface{}{
		"type":        "object",
		"description": "IPv6 configuration for one network interface.",
		"properties": map[string]interface{}{
			"interface_name":  map[string]interface{}{"type": "string", "description": "Network interface name, e.g. \"nic0\"."},
			"dhcp":            map[string]interface{}{"type": "boolean", "description": "Enable DHCPv6."},
			"autoconf":        map[string]interface{}{"type": "boolean", "description": "Enable IPv6 stateless address autoconfiguration (SLAAC)."},
			"default_gateway": map[string]interface{}{"type": "string", "description": "Default gateway IPv6 address."},
			"addresses": map[string]interface{}{
				"type":        "array",
				"description": "Static IPv6 addresses to assign.",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"address": map[string]interface{}{"type": "string", "description": "IPv6 address."},
						"prefix":  map[string]interface{}{"type": "integer", "description": "CIDR prefix length."},
					},
					"required": []interface{}{"address", "prefix"},
				},
			},
		},
		"required": []interface{}{"interface_name"},
	}

	proxyEntrySchema := map[string]interface{}{
		"type":        "object",
		"description": "Proxy configuration for one protocol.",
		"properties": map[string]interface{}{
			"protocol": map[string]interface{}{"type": "string", "description": "Protocol this proxy configuration applies to. Only \"ftp\" is confirmed from this API's own example; http/https/socks are the other protocols typical of this VAMI proxy family but are unconfirmed against this specific techpreview endpoint."},
			"server":   map[string]interface{}{"type": "string", "description": "Proxy server hostname or IP address."},
			"port":     map[string]interface{}{"type": "integer", "description": "Proxy server port."},
			"username": map[string]interface{}{"type": "string", "description": "Optional proxy authentication username."},
			"password": map[string]interface{}{"type": "string", "description": "Optional proxy authentication password — sent as plain text in the request body, per this API's own example."},
		},
		"required": []interface{}{"protocol", "server", "port"},
	}

	routeSchema := map[string]interface{}{
		"type":        "object",
		"description": "A static network route.",
		"properties": map[string]interface{}{
			"destination":    map[string]interface{}{"type": "string", "description": "Destination network address."},
			"prefix":         map[string]interface{}{"type": "integer", "description": "CIDR prefix length for destination."},
			"gateway":        map[string]interface{}{"type": "string", "description": "Gateway address for this route."},
			"interface_name": map[string]interface{}{"type": "string", "description": "Network interface the route goes out through."},
		},
		"required": []interface{}{"destination", "prefix", "gateway", "interface_name"},
	}

	// --- Firewall (addr/inbound) ---------------------------------------------

	r.register("vmware_appliance_techpreview_firewall_list",
		"List the vCenter Server Appliance's ordered inbound IP address firewall rules (allow/deny by address+prefix+interface)."+requiresVCSA+techPreview,
		emptySchema,
		Tool{Handler: handleTechpreviewFirewallList},
	)

	r.registerDestructive("vmware_appliance_techpreview_firewall_create",
		"Create/insert one inbound IP address firewall rule on the vCenter Server Appliance, optionally at a given ordered position."+requiresVCSA+techPreview,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"pos":     map[string]interface{}{"type": "integer", "description": "Optional 1-based position to insert the rule at in the ordered inbound rule list. If omitted, the appliance's default insert position is used."},
				"rule":    firewallRuleSchema,
				"confirm": confirmArg,
			},
			"required": []interface{}{"rule", "confirm"},
		},
		Tool{Handler: handleTechpreviewFirewallCreate},
	)

	r.registerDestructive("vmware_appliance_techpreview_firewall_delete",
		"Delete one or all inbound IP address firewall rules on the vCenter Server Appliance — set all:true to remove every rule, or position to remove a single rule at that ordered index. Irreversible: the removed rule(s) are not recoverable except by re-creating them."+requiresVCSA+techPreview,
		tier1,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"config": map[string]interface{}{
					"type":        "object",
					"description": "Delete criteria.",
					"properties": map[string]interface{}{
						"all":      map[string]interface{}{"type": "boolean", "description": "Delete every inbound rule."},
						"position": map[string]interface{}{"type": "integer", "description": "1-based position of a single rule to delete."},
					},
				},
				"confirm": confirmArg,
			},
			"required": []interface{}{"config", "confirm"},
		},
		Tool{Handler: handleTechpreviewFirewallDelete},
	)

	r.registerDestructive("vmware_appliance_techpreview_firewall_replace",
		"Replace the vCenter Server Appliance's ENTIRE ordered list of inbound IP address firewall rules in one call — every existing rule is discarded and replaced with the provided list (an empty list removes all rules). High-risk: a wrong or overly-restrictive rule set can lock off network access to the appliance with no easy remote recovery."+requiresVCSA+techPreview,
		tier1,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"rules":   map[string]interface{}{"type": "array", "items": firewallRuleSchema, "description": "Complete ordered list of inbound rules — REPLACES every existing rule."},
				"confirm": confirmArg,
			},
			"required": []interface{}{"rules", "confirm"},
		},
		Tool{Handler: handleTechpreviewFirewallReplace},
	)

	// --- IPv4 -----------------------------------------------------------------

	r.register("vmware_appliance_techpreview_ipv4_get",
		"Get the vCenter Server Appliance's IPv4 network configuration for all interfaces."+requiresVCSA+techPreview,
		emptySchema,
		Tool{Handler: handleTechpreviewIPv4Get},
	)

	r.register("vmware_appliance_techpreview_ipv4_details",
		"Get IPv4 configuration details for specific vCenter Server Appliance network interfaces."+requiresVCSA+techPreview,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"interfaces": interfacesArraySchema},
			"required":   []interface{}{"interfaces"},
		},
		Tool{Handler: handleTechpreviewIPv4Details},
	)

	r.registerDestructive("vmware_appliance_techpreview_ipv4_renew",
		"Renew the DHCP IPv4 lease on specific vCenter Server Appliance network interfaces. Disruptive but reversible (a new/renewed lease can be requested again)."+requiresVCSA+techPreview,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"interfaces": interfacesArraySchema,
				"confirm":    confirmArg,
			},
			"required": []interface{}{"interfaces", "confirm"},
		},
		Tool{Handler: handleTechpreviewIPv4Renew},
	)

	r.registerDestructive("vmware_appliance_techpreview_ipv4_set",
		"Set IPv4 configuration (DHCP or static) for one or more vCenter Server Appliance network interfaces."+requiresVCSA+techPreview,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"config":  map[string]interface{}{"type": "array", "items": ipv4ConfigItemSchema, "description": "IPv4 configuration entries, one per interface being changed."},
				"confirm": confirmArg,
			},
			"required": []interface{}{"config", "confirm"},
		},
		Tool{Handler: handleTechpreviewIPv4Set},
	)

	// --- IPv6 -----------------------------------------------------------------

	r.register("vmware_appliance_techpreview_ipv6_get",
		"Get the vCenter Server Appliance's IPv6 network configuration for all interfaces."+requiresVCSA+techPreview,
		emptySchema,
		Tool{Handler: handleTechpreviewIPv6Get},
	)

	r.register("vmware_appliance_techpreview_ipv6_details",
		"Get IPv6 configuration details for specific vCenter Server Appliance network interfaces."+requiresVCSA+techPreview,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"interfaces": interfacesArraySchema},
			"required":   []interface{}{"interfaces"},
		},
		Tool{Handler: handleTechpreviewIPv6Details},
	)

	r.registerDestructive("vmware_appliance_techpreview_ipv6_set",
		"Set IPv6 configuration (DHCPv6, SLAAC autoconf, and/or static addresses) for one or more vCenter Server Appliance network interfaces."+requiresVCSA+techPreview,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"config":  map[string]interface{}{"type": "array", "items": ipv6ConfigItemSchema, "description": "IPv6 configuration entries, one per interface being changed."},
				"confirm": confirmArg,
			},
			"required": []interface{}{"config", "confirm"},
		},
		Tool{Handler: handleTechpreviewIPv6Set},
	)

	// --- NTP --------------------------------------------------------------------

	r.register("vmware_appliance_techpreview_ntp_get",
		"Get the vCenter Server Appliance's NTP configuration status."+requiresVCSA+techPreview,
		emptySchema,
		Tool{Handler: handleTechpreviewNTPGet},
	)

	r.register("vmware_appliance_techpreview_ntp_test",
		"Test reachability/validity of one or more NTP servers from the vCenter Server Appliance, without changing its configuration."+requiresVCSA+techPreview,
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"servers": serversArraySchema},
			"required":   []interface{}{"servers"},
		},
		Tool{Handler: handleTechpreviewNTPTest},
	)

	r.registerDestructive("vmware_appliance_techpreview_ntp_server_add",
		"Add one or more NTP servers to the vCenter Server Appliance's configuration (appended to the existing list)."+requiresVCSA+techPreview,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"servers": serversArraySchema,
				"confirm": confirmArg,
			},
			"required": []interface{}{"servers", "confirm"},
		},
		Tool{Handler: handleTechpreviewNTPServerAdd},
	)

	r.registerDestructive("vmware_appliance_techpreview_ntp_server_delete",
		"Remove one or more NTP servers from the vCenter Server Appliance's configuration. Irreversible: the removed server(s) are not recoverable except by re-adding them."+requiresVCSA+techPreview,
		tier1,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"servers": serversArraySchema,
				"confirm": confirmArg,
			},
			"required": []interface{}{"servers", "confirm"},
		},
		Tool{Handler: handleTechpreviewNTPServerDelete},
	)

	r.registerDestructive("vmware_appliance_techpreview_ntp_server_set",
		"Replace the vCenter Server Appliance's ENTIRE NTP server list in one call — every existing server is discarded and replaced with the provided list."+requiresVCSA+techPreview,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"servers": serversArraySchema,
				"confirm": confirmArg,
			},
			"required": []interface{}{"servers", "confirm"},
		},
		Tool{Handler: handleTechpreviewNTPServerSet},
	)

	// --- Proxy ------------------------------------------------------------------

	r.register("vmware_appliance_techpreview_proxy_get",
		"Get the vCenter Server Appliance's per-protocol HTTP/HTTPS/FTP proxy configuration."+requiresVCSA+techPreview,
		emptySchema,
		Tool{Handler: handleTechpreviewProxyGet},
	)

	r.registerDestructive("vmware_appliance_techpreview_proxy_set",
		"Replace the vCenter Server Appliance's ENTIRE per-protocol proxy configuration list in one call, and set the overall proxy status."+requiresVCSA+techPreview,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"config": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"configlist": map[string]interface{}{"type": "array", "items": proxyEntrySchema, "description": "Per-protocol proxy configurations — REPLACES the entire existing configuration list."},
						"status":     map[string]interface{}{"type": "string", "description": "Overall proxy status. Only \"disabled\" is confirmed from this API's own example; an enabled counterpart is expected but unconfirmed against this specific techpreview endpoint."},
					},
					"required": []interface{}{"configlist", "status"},
				},
				"confirm": confirmArg,
			},
			"required": []interface{}{"config", "confirm"},
		},
		Tool{Handler: handleTechpreviewProxySet},
	)

	r.register("vmware_appliance_techpreview_proxy_test",
		"Test connectivity through a candidate proxy configuration from the vCenter Server Appliance, without changing its configuration."+requiresVCSA+techPreview,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"config": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"protocol": map[string]interface{}{"type": "string", "description": "Protocol to test, e.g. \"ftp\"."},
						"server":   map[string]interface{}{"type": "string", "description": "Proxy server hostname or IP address."},
						"port":     map[string]interface{}{"type": "integer", "description": "Proxy server port."},
						"testhost": map[string]interface{}{"type": "string", "description": "Host to test connectivity against through the proxy."},
						"username": map[string]interface{}{"type": "string", "description": "Optional proxy authentication username."},
						"password": map[string]interface{}{"type": "string", "description": "Optional proxy authentication password."},
					},
					"required": []interface{}{"protocol", "server", "port"},
				},
			},
			"required": []interface{}{"config"},
		},
		Tool{Handler: handleTechpreviewProxyTest},
	)

	r.registerDestructive("vmware_appliance_techpreview_proxy_delete",
		"Remove the vCenter Server Appliance's proxy configuration for one protocol. Irreversible: the removed configuration is not recoverable except by re-creating it via vmware_appliance_techpreview_proxy_set."+requiresVCSA+techPreview,
		tier1,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"protocol": map[string]interface{}{"type": "string", "description": "Protocol whose proxy configuration should be removed, e.g. \"ftp\"."},
				"confirm":  confirmArg,
			},
			"required": []interface{}{"protocol", "confirm"},
		},
		Tool{Handler: handleTechpreviewProxyDelete},
	)

	// --- Routes -------------------------------------------------------------------

	r.register("vmware_appliance_techpreview_routes_list",
		"List the vCenter Server Appliance's static network routes."+requiresVCSA+techPreview,
		emptySchema,
		Tool{Handler: handleTechpreviewRoutesList},
	)

	r.registerDestructive("vmware_appliance_techpreview_routes_set",
		"Replace the vCenter Server Appliance's ENTIRE static route table in one call — every existing route is discarded and replaced with the provided list."+requiresVCSA+techPreview,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"routes":  map[string]interface{}{"type": "array", "items": routeSchema, "description": "Complete list of static routes — REPLACES every existing route."},
				"confirm": confirmArg,
			},
			"required": []interface{}{"routes", "confirm"},
		},
		Tool{Handler: handleTechpreviewRoutesSet},
	)

	r.registerDestructive("vmware_appliance_techpreview_routes_add",
		"Add one static network route to the vCenter Server Appliance (appended to the existing route table)."+requiresVCSA+techPreview,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"route":   routeSchema,
				"confirm": confirmArg,
			},
			"required": []interface{}{"route", "confirm"},
		},
		Tool{Handler: handleTechpreviewRoutesAdd},
	)

	r.register("vmware_appliance_techpreview_routes_test",
		"Test reachability of one or more gateway addresses from the vCenter Server Appliance, without changing its route table."+requiresVCSA+techPreview,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"gateways": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Gateway addresses to test reachability for."},
			},
			"required": []interface{}{"gateways"},
		},
		Tool{Handler: handleTechpreviewRoutesTest},
	)

	r.registerDestructive("vmware_appliance_techpreview_routes_delete",
		"Remove one static network route from the vCenter Server Appliance, matched by its full destination/prefix/gateway/interface. Irreversible: the removed route is not recoverable except by re-adding it."+requiresVCSA+techPreview,
		tier1,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"route":   routeSchema,
				"confirm": confirmArg,
			},
			"required": []interface{}{"route", "confirm"},
		},
		Tool{Handler: handleTechpreviewRoutesDelete},
	)

	// --- Timesync -----------------------------------------------------------------

	r.register("vmware_appliance_techpreview_timesync_get",
		"Get the vCenter Server Appliance's time synchronization configuration."+requiresVCSA+techPreview,
		emptySchema,
		Tool{Handler: handleTechpreviewTimesyncGet},
	)

	r.registerDestructive("vmware_appliance_techpreview_timesync_set",
		"Set the vCenter Server Appliance's time synchronization mode."+requiresVCSA+techPreview,
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"config": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"mode": map[string]interface{}{"type": "string", "description": "Time synchronization mode. Only \"host\" (sync to the hypervisor) is confirmed from this API's own example; NTP-based modes are typical of this API family but unconfirmed against this specific techpreview endpoint."},
					},
					"required": []interface{}{"mode"},
				},
				"confirm": confirmArg,
			},
			"required": []interface{}{"config", "confirm"},
		},
		Tool{Handler: handleTechpreviewTimesyncSet},
	)
}

// --- Path constants (rest/appliance/techpreview/...) -------------------------

const (
	techpreviewFirewallInboundPath = "/appliance/techpreview/networking/firewall/addr/inbound"
	techpreviewIPv4Path            = "/appliance/techpreview/networking/ipv4"
	techpreviewIPv6Path            = "/appliance/techpreview/networking/ipv6"
	techpreviewNTPPath             = "/appliance/techpreview/ntp"
	techpreviewProxyPath           = "/appliance/techpreview/networking/proxy"
	techpreviewRoutesPath          = "/appliance/techpreview/networking/routes"
	techpreviewTimesyncPath        = "/appliance/techpreview/timesync"
)

// --- Request helper ------------------------------------------------------------

// applianceTechpreviewDo issues a VAMI /appliance/techpreview/... request and
// decodes the response generically — the direct generalization of Fase 4's
// tools/appliance.go applianceGet to also support POST/PUT with a JSON
// request body (nil body means no body at all, matching Resource.Request's
// own "optional body" contract). No typed request/response struct: this
// project has no simulator/live vCenter Appliance to verify these
// undocumented techpreview VAMI field names against, so a generic decode on
// both sides is more honest than a guessed struct that could silently drop
// or misname fields — same reasoning as applianceGet's doc comment.
func applianceTechpreviewDo(ctx context.Context, client *vmware.Client, method, path string, body interface{}) (interface{}, error) {
	rc, err := client.REST(ctx)
	if err != nil {
		return nil, err // REST() already names the likely cause (no VAMI on standalone ESXi)
	}

	var req *http.Request
	if body != nil {
		req = rc.Resource(path).Request(method, body)
	} else {
		req = rc.Resource(path).Request(method)
	}

	var result interface{}
	if err := rc.Do(ctx, req, &result); err != nil {
		return nil, fmt.Errorf("VAMI techpreview request %s %s failed: %w", method, path, err)
	}
	return result, nil
}

// --- Argument helpers ------------------------------------------------------
//
// requiredStringArg (bare string args, e.g. "protocol") already exists,
// package-level, in generated_library_core.go — reused as-is below. These
// two cover the JSON object/array shapes this file's handlers need instead
// (nested rule/route/config payloads passed straight through to VAMI, which
// validates their real content — the same "let the real server validate a
// generically-decoded payload" posture already used throughout this
// project, e.g. decodeJSONArg's callers).

// requireObjectArg reads args[key] as a JSON object (map[string]interface{})
// or returns a clear error.
func requireObjectArg(args map[string]interface{}, key string) (map[string]interface{}, error) {
	v, ok := args[key]
	if !ok {
		return nil, fmt.Errorf("%s is required", key)
	}
	obj, ok := v.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("%s must be a JSON object, got %T", key, v)
	}
	return obj, nil
}

// requireArrayArg reads args[key] as a JSON array ([]interface{}) or returns
// a clear error.
func requireArrayArg(args map[string]interface{}, key string) ([]interface{}, error) {
	v, ok := args[key]
	if !ok {
		return nil, fmt.Errorf("%s is required", key)
	}
	arr, ok := v.([]interface{})
	if !ok {
		return nil, fmt.Errorf("%s must be a JSON array, got %T", key, v)
	}
	return arr, nil
}

// --- Firewall handlers -------------------------------------------------------

func handleTechpreviewFirewallList(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	v, err := applianceTechpreviewDo(ctx, client, http.MethodGet, techpreviewFirewallInboundPath, nil)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

func handleTechpreviewFirewallCreate(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	rule, err := requireObjectArg(args, "rule")
	if err != nil {
		return "", err
	}
	body := map[string]interface{}{"rule": rule}
	if pos, ok := args["pos"]; ok {
		body["pos"] = pos
	}

	v, err := applianceTechpreviewDo(ctx, client, http.MethodPost, techpreviewFirewallInboundPath, body)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

func handleTechpreviewFirewallDelete(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	config, err := requireObjectArg(args, "config")
	if err != nil {
		return "", err
	}
	body := map[string]interface{}{"config": config}

	v, err := applianceTechpreviewDo(ctx, client, http.MethodPost, techpreviewFirewallInboundPath+"/delete", body)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

func handleTechpreviewFirewallReplace(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	rules, err := requireArrayArg(args, "rules")
	if err != nil {
		return "", err
	}
	body := map[string]interface{}{"rules": rules}

	v, err := applianceTechpreviewDo(ctx, client, http.MethodPut, techpreviewFirewallInboundPath, body)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

// --- IPv4 handlers -----------------------------------------------------------

func handleTechpreviewIPv4Get(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	v, err := applianceTechpreviewDo(ctx, client, http.MethodGet, techpreviewIPv4Path, nil)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

func handleTechpreviewIPv4Details(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	interfaces, err := requireArrayArg(args, "interfaces")
	if err != nil {
		return "", err
	}
	body := map[string]interface{}{"interfaces": interfaces}

	v, err := applianceTechpreviewDo(ctx, client, http.MethodPost, techpreviewIPv4Path+"/get", body)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

func handleTechpreviewIPv4Renew(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	interfaces, err := requireArrayArg(args, "interfaces")
	if err != nil {
		return "", err
	}
	body := map[string]interface{}{"interfaces": interfaces}

	v, err := applianceTechpreviewDo(ctx, client, http.MethodPost, techpreviewIPv4Path+"/renew", body)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

func handleTechpreviewIPv4Set(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	config, err := requireArrayArg(args, "config")
	if err != nil {
		return "", err
	}
	body := map[string]interface{}{"config": config}

	v, err := applianceTechpreviewDo(ctx, client, http.MethodPost, techpreviewIPv4Path, body)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

// --- IPv6 handlers -----------------------------------------------------------

func handleTechpreviewIPv6Get(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	v, err := applianceTechpreviewDo(ctx, client, http.MethodGet, techpreviewIPv6Path, nil)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

func handleTechpreviewIPv6Details(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	interfaces, err := requireArrayArg(args, "interfaces")
	if err != nil {
		return "", err
	}
	body := map[string]interface{}{"interfaces": interfaces}

	v, err := applianceTechpreviewDo(ctx, client, http.MethodPost, techpreviewIPv6Path+"/get", body)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

func handleTechpreviewIPv6Set(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	config, err := requireArrayArg(args, "config")
	if err != nil {
		return "", err
	}
	body := map[string]interface{}{"config": config}

	v, err := applianceTechpreviewDo(ctx, client, http.MethodPost, techpreviewIPv6Path, body)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

// --- NTP handlers --------------------------------------------------------------

func handleTechpreviewNTPGet(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	v, err := applianceTechpreviewDo(ctx, client, http.MethodGet, techpreviewNTPPath, nil)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

func handleTechpreviewNTPTest(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	servers, err := requireArrayArg(args, "servers")
	if err != nil {
		return "", err
	}
	body := map[string]interface{}{"servers": servers}

	v, err := applianceTechpreviewDo(ctx, client, http.MethodPost, techpreviewNTPPath+"/test", body)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

func handleTechpreviewNTPServerAdd(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	servers, err := requireArrayArg(args, "servers")
	if err != nil {
		return "", err
	}
	body := map[string]interface{}{"servers": servers}

	v, err := applianceTechpreviewDo(ctx, client, http.MethodPost, techpreviewNTPPath+"/server", body)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

func handleTechpreviewNTPServerDelete(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	servers, err := requireArrayArg(args, "servers")
	if err != nil {
		return "", err
	}
	body := map[string]interface{}{"servers": servers}

	v, err := applianceTechpreviewDo(ctx, client, http.MethodPost, techpreviewNTPPath+"/server/delete", body)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

func handleTechpreviewNTPServerSet(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	servers, err := requireArrayArg(args, "servers")
	if err != nil {
		return "", err
	}
	body := map[string]interface{}{"servers": servers}

	v, err := applianceTechpreviewDo(ctx, client, http.MethodPut, techpreviewNTPPath+"/server", body)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

// --- Proxy handlers ------------------------------------------------------------

func handleTechpreviewProxyGet(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	v, err := applianceTechpreviewDo(ctx, client, http.MethodGet, techpreviewProxyPath, nil)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

func handleTechpreviewProxySet(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	config, err := requireObjectArg(args, "config")
	if err != nil {
		return "", err
	}
	body := map[string]interface{}{"config": config}

	v, err := applianceTechpreviewDo(ctx, client, http.MethodPut, techpreviewProxyPath, body)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

func handleTechpreviewProxyTest(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	config, err := requireObjectArg(args, "config")
	if err != nil {
		return "", err
	}
	body := map[string]interface{}{"config": config}

	v, err := applianceTechpreviewDo(ctx, client, http.MethodPost, techpreviewProxyPath+"/test", body)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

func handleTechpreviewProxyDelete(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	protocol, err := requiredStringArg(args, "protocol")
	if err != nil {
		return "", err
	}
	body := map[string]interface{}{"protocol": protocol}

	v, err := applianceTechpreviewDo(ctx, client, http.MethodPost, techpreviewProxyPath+"/delete", body)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

// --- Routes handlers -----------------------------------------------------------

func handleTechpreviewRoutesList(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	v, err := applianceTechpreviewDo(ctx, client, http.MethodGet, techpreviewRoutesPath, nil)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

func handleTechpreviewRoutesSet(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	routes, err := requireArrayArg(args, "routes")
	if err != nil {
		return "", err
	}
	body := map[string]interface{}{"routes": routes}

	v, err := applianceTechpreviewDo(ctx, client, http.MethodPut, techpreviewRoutesPath, body)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

func handleTechpreviewRoutesAdd(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	route, err := requireObjectArg(args, "route")
	if err != nil {
		return "", err
	}
	body := map[string]interface{}{"route": route}

	v, err := applianceTechpreviewDo(ctx, client, http.MethodPost, techpreviewRoutesPath, body)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

func handleTechpreviewRoutesTest(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	gateways, err := requireArrayArg(args, "gateways")
	if err != nil {
		return "", err
	}
	body := map[string]interface{}{"gateways": gateways}

	v, err := applianceTechpreviewDo(ctx, client, http.MethodPost, techpreviewRoutesPath+"/test", body)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

func handleTechpreviewRoutesDelete(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	route, err := requireObjectArg(args, "route")
	if err != nil {
		return "", err
	}
	body := map[string]interface{}{"route": route}

	v, err := applianceTechpreviewDo(ctx, client, http.MethodPost, techpreviewRoutesPath+"/delete", body)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

// --- Timesync handlers -----------------------------------------------------------

func handleTechpreviewTimesyncGet(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	v, err := applianceTechpreviewDo(ctx, client, http.MethodGet, techpreviewTimesyncPath, nil)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

func handleTechpreviewTimesyncSet(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	config, err := requireObjectArg(args, "config")
	if err != nil {
		return "", err
	}
	body := map[string]interface{}{"config": config}

	v, err := applianceTechpreviewDo(ctx, client, http.MethodPut, techpreviewTimesyncPath, body)
	if err != nil {
		return "", err
	}
	return marshalJSON(v)
}

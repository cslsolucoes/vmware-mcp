package tools

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/cslsoftwares/mcpvmware/cloudaws"
)

// registerCloudAWSSDDCsTools is Fase 10's "Grupo 2" slice: the "SDDCs"
// folder (and its Cluster/Cluster-EDRS/Hosts/DNS/Public IPs/Addon-Credential
// subfolders) of `.workspace/VMware Cloud on AWS APIs.postman_collection.json`
// — 23 operations. SDDC (Software-Defined Data Center) is VMC on AWS's
// equivalent of a vSphere datacenter/cluster/host stack, but provisioned as
// a managed service on real AWS hardware — every tool in this file talks to
// *cloudaws.Client (CSP bearer-token REST, see cloudaws/client.go), never
// govmomi; there is no SOAP session and no simulator/live VMC account
// available to this project to verify field names against (see
// cloudaws/client.go's tokenResponse/errorModel doc comments — same
// posture). IDs (org, sddc, cluster, addon type, credential name) are
// opaque path strings this project never resolves against any inventory —
// exactly as the brief specifies.
//
// Path prefix: every route here is under the standard VMC API base
// (baseURL + "/vmc/api/orgs/...") EXCEPT the 3 EDRS routes below, which live
// under a *different* API family entirely — "/vmc/api/orgs/...
// autoscaler" — "/vmc/autoscaler/api/orgs/..." (confirmed against the
// vendored Postman collection's raw request URLs, not assumed: "List SDDC
// Cluster EDRS Policy"/"List SDDC EDRS Policies"/"Set SDDC Cluster EDRS
// Policy" are the only 3 requests anywhere in the SDDCs folder whose host
// path starts with "/vmc/autoscaler/api/" instead of "/vmc/api/"). Same
// *cloudaws.Client, same host (vmc.vmware.com) — cloudaws.Client.Do's path
// parameter is taken as-is, so this file just spells out the differing
// prefix per handler; no client-side change was needed for it.
//
// TIER DECISION (deliberate departure from this project's usual convention
// — read before adding/copying a pattern from here into another domain):
// elsewhere in this project only DELETE is Tier 1 and updates are Tier 2.
// In this file EVERY write operation (POST/PUT/PATCH/DELETE) defaults to
// Tier 1, because an SDDC accrues real AWS infrastructure cost per hour it
// exists — an accidental sddc_create, an accidental host add, an accidental
// public IP allocation are all real money, not just "disruptive but
// reversible" the way e.g. a VM power-off is elsewhere in this project. This
// was decided by the orchestrator for Fase 10's SDDCs group specifically,
// not re-derived here. The only 2 exceptions, both explicitly called out by
// the brief and kept at Tier 2:
//   - vmware_cloudaws_sddc_edrs_cluster_set (POST .../edrs-policy) — adjusts
//     an existing cluster's auto-scaling policy (min/max host bounds,
//     performance-vs-cost trade-off); it does not itself create or destroy
//     any host or SDDC.
//   - vmware_cloudaws_sddc_dns_update_public / _private (PUT .../dns/public
//     and .../dns/private) — repoints management-VM DNS records between
//     public/private IPs; pure network configuration, provisions nothing.
//
// Request body honesty: the vendored Postman collection documents concrete
// example JSON for exactly 2 of this file's write bodies — SDDC Create
// ({"num_hosts":1,"name":"...","provider":"AWS","region":"US_WEST_2"}) and
// EDRS Set ({"enable_edrs":true,"policy_type":"performance","max_hosts":16,
// "min_hosts":4}). Every other write body in the SDDCs folder is documented
// only as placeholder prose ("clusterConfig", "esxConfig", "allocation
// spec", "SddcPublicIp object to update", "Credentials creation/update
// payload", "Patch request for the SDDC") — no concrete field list. Rather
// than guess a struct this project has no way to verify (same reasoning as
// cloudaws/client.go's tokenResponse/errorModel and
// generated_extension.go's extensionArg precedent), every such body is
// exposed as a generic JSON object argument named "spec", forwarded
// verbatim as the request body — the caller supplies whatever fields the
// real VMC API documentation (console.cloud.vmware.com's own API docs, not
// vendored here) calls for.
func registerCloudAWSSDDCsTools(r *Registry) {
	orgArg := map[string]interface{}{
		"type":        "string",
		"description": "VMC on AWS organization ID (opaque identifier — visible in the VMC console URL, or via an orgs-listing tool).",
	}
	sddcArg := map[string]interface{}{
		"type":        "string",
		"description": "SDDC ID (opaque identifier returned by vmware_cloudaws_sddc_list/vmware_cloudaws_sddc_create).",
	}
	clusterArg := map[string]interface{}{
		"type":        "string",
		"description": `Cluster ID within the SDDC (opaque identifier, e.g. "cluster-1" — see the SDDC's cluster list in vmware_cloudaws_sddc_get's response).`,
	}
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}
	specArg := func(desc string) map[string]interface{} {
		return map[string]interface{}{"type": "object", "description": desc}
	}

	schema := func(props map[string]interface{}, required ...string) map[string]interface{} {
		reqs := make([]interface{}, len(required))
		for i, req := range required {
			reqs[i] = req
		}
		return map[string]interface{}{"type": "object", "properties": props, "required": reqs}
	}

	// --- SDDCs (base) -------------------------------------------------------

	r.registerCloudAWS("vmware_cloudaws_sddc_list",
		"List every SDDC in a VMC on AWS organization (GET /vmc/api/orgs/{org}/sddcs).",
		schema(map[string]interface{}{"org": orgArg}, "org"),
		Tool{CloudHandler: handleCloudAWSSDDCList},
	)

	r.registerCloudAWS("vmware_cloudaws_sddc_get",
		"Get details of one SDDC — hosts, clusters, network config, provisioning state (GET /vmc/api/orgs/{org}/sddcs/{sddc}).",
		schema(map[string]interface{}{"org": orgArg, "sddc": sddcArg}, "org", "sddc"),
		Tool{CloudHandler: handleCloudAWSSDDCGet},
	)

	r.registerDestructiveCloudAWS("vmware_cloudaws_sddc_create",
		"Create a new SDDC (POST /vmc/api/orgs/{org}/sddcs) — the single most expensive operation in this whole API: it provisions real physical ESXi hosts on AWS bare metal, billed hourly from the moment they come up. Known example body fields: num_hosts (int), name (string), provider (\"AWS\"), region (e.g. \"US_WEST_2\") — the real VMC API accepts many more optional fields (VPC/subnet, deployment_type, sddc_type, account_link config, etc.) not enumerated here; consult VMware's own VMC API docs for the full spec and pass them via \"spec\".",
		tier1,
		schema(map[string]interface{}{
			"org":     orgArg,
			"spec":    specArg(`SDDC creation spec — JSON object, e.g. {"num_hosts":1,"name":"my-sddc","provider":"AWS","region":"US_WEST_2"}.`),
			"confirm": confirmArg,
		}, "org", "spec", "confirm"),
		Tool{CloudHandler: handleCloudAWSSDDCCreate},
	)

	r.registerDestructiveCloudAWS("vmware_cloudaws_sddc_delete",
		"Permanently delete an SDDC (DELETE /vmc/api/orgs/{org}/sddcs/{sddc}) — destroys every host, VM, and network config it contains. Irreversible.",
		tier1,
		schema(map[string]interface{}{"org": orgArg, "sddc": sddcArg, "confirm": confirmArg}, "org", "sddc", "confirm"),
		Tool{CloudHandler: handleCloudAWSSDDCDelete},
	)

	r.registerDestructiveCloudAWS("vmware_cloudaws_sddc_update",
		"Update an existing SDDC's configuration (PATCH /vmc/api/orgs/{org}/sddcs/{sddc}) — can change sizing/config with direct cost impact. Body shape is not concretely documented in this project's vendored spec (Postman only says \"Patch request for the SDDC\"); pass whatever partial fields VMware's VMC API docs call for.",
		tier1,
		schema(map[string]interface{}{
			"org":     orgArg,
			"sddc":    sddcArg,
			"spec":    specArg("Partial SDDC update document — JSON object, fields per VMware's VMC API docs."),
			"confirm": confirmArg,
		}, "org", "sddc", "spec", "confirm"),
		Tool{CloudHandler: handleCloudAWSSDDCUpdate},
	)

	r.registerDestructiveCloudAWS("vmware_cloudaws_sddc_convert",
		"Convert a 1-host SDDC to a 3-node DEFAULT SDDC (POST /vmc/api/orgs/{org}/sddcs/{sddc}/convert) — reconfigures/upgrades vCenter for high availability and data redundancy. Major structural change with a direct cost impact (2 additional hosts).",
		tier1,
		schema(map[string]interface{}{"org": orgArg, "sddc": sddcArg, "confirm": confirmArg}, "org", "sddc", "confirm"),
		Tool{CloudHandler: handleCloudAWSSDDCConvert},
	)

	// --- Cluster --------------------------------------------------------------

	r.registerDestructiveCloudAWS("vmware_cloudaws_sddc_cluster_create",
		"Create a new cluster inside an SDDC (POST /vmc/api/orgs/{org}/sddcs/{sddc}/clusters) — adds billable hosts. Body shape (\"clusterConfig\") is not concretely documented in this project's vendored spec; pass whatever fields VMware's VMC API docs call for (typically num_hosts, host CPU/storage config, etc.).",
		tier1,
		schema(map[string]interface{}{
			"org":     orgArg,
			"sddc":    sddcArg,
			"spec":    specArg("Cluster creation spec (\"clusterConfig\") — JSON object, fields per VMware's VMC API docs."),
			"confirm": confirmArg,
		}, "org", "sddc", "spec", "confirm"),
		Tool{CloudHandler: handleCloudAWSSDDCClusterCreate},
	)

	r.registerDestructiveCloudAWS("vmware_cloudaws_sddc_cluster_delete",
		"Delete a cluster from an SDDC (DELETE /vmc/api/orgs/{org}/sddcs/{sddc}/clusters/{cluster}) — a FORCE operation: it deletes the cluster even if that means data loss. Per VMware's own API description, every VM in the cluster should be powered off first. Irreversible.",
		tier1,
		schema(map[string]interface{}{"org": orgArg, "sddc": sddcArg, "cluster": clusterArg, "confirm": confirmArg}, "org", "sddc", "cluster", "confirm"),
		Tool{CloudHandler: handleCloudAWSSDDCClusterDelete},
	)

	// --- Hosts ------------------------------------------------------------

	r.registerDestructiveCloudAWS("vmware_cloudaws_sddc_hosts_update",
		"Add or remove one or more ESXi hosts in an SDDC's target cloud (POST /vmc/api/orgs/{org}/sddcs/{sddc}/esxs?action={action}) — directly changes the number of billable physical hosts. \"action\" selects the direction (per VMware's VMC API docs, typically \"add\" or \"remove\"); \"spec\" (\"esxConfig\") carries the host count/placement details.",
		tier1,
		schema(map[string]interface{}{
			"org":     orgArg,
			"sddc":    sddcArg,
			"action":  map[string]interface{}{"type": "string", "description": `Action to perform, e.g. "add" or "remove" — passed through as-is to the "action" query parameter.`},
			"spec":    specArg("Host add/remove spec (\"esxConfig\") — JSON object, fields per VMware's VMC API docs."),
			"confirm": confirmArg,
		}, "org", "sddc", "action", "spec", "confirm"),
		Tool{CloudHandler: handleCloudAWSSDDCHostsUpdate},
	)

	// --- Cluster / EDRS -----------------------------------------------------
	// Note the "/vmc/autoscaler/api/" prefix (not "/vmc/api/") — see this
	// file's top doc comment.

	r.registerCloudAWS("vmware_cloudaws_sddc_edrs_cluster_list",
		"List the Elastic DRS (auto-scaling) policy for one cluster in an SDDC (GET /vmc/autoscaler/api/orgs/{org}/sddcs/{sddc}/clusters/{cluster}/edrs-policy).",
		schema(map[string]interface{}{"org": orgArg, "sddc": sddcArg, "cluster": clusterArg}, "org", "sddc", "cluster"),
		Tool{CloudHandler: handleCloudAWSSDDCEDRSClusterList},
	)

	r.registerCloudAWS("vmware_cloudaws_sddc_edrs_list",
		"List the Elastic DRS (auto-scaling) policies for every cluster in an SDDC (GET /vmc/autoscaler/api/orgs/{org}/sddcs/{sddc}/edrs-policy).",
		schema(map[string]interface{}{"org": orgArg, "sddc": sddcArg}, "org", "sddc"),
		Tool{CloudHandler: handleCloudAWSSDDCEDRSList},
	)

	r.registerDestructiveCloudAWS("vmware_cloudaws_sddc_edrs_cluster_set",
		"Set the Elastic DRS (auto-scaling) policy for one cluster in an SDDC (POST /vmc/autoscaler/api/orgs/{org}/sddcs/{sddc}/clusters/{cluster}/edrs-policy). Tier 2, not Tier 1: this adjusts an existing cluster's auto-scaling rules — it does not itself create or destroy hosts (EDRS may add/remove hosts later, automatically, per the policy set here). Known example body: {\"enable_edrs\":true,\"policy_type\":\"performance\",\"max_hosts\":16,\"min_hosts\":4}.",
		tier2,
		schema(map[string]interface{}{
			"org":     orgArg,
			"sddc":    sddcArg,
			"cluster": clusterArg,
			"spec":    specArg(`EDRS policy spec — JSON object, e.g. {"enable_edrs":true,"policy_type":"performance","max_hosts":16,"min_hosts":4}.`),
			"confirm": confirmArg,
		}, "org", "sddc", "cluster", "spec", "confirm"),
		Tool{CloudHandler: handleCloudAWSSDDCEDRSClusterSet},
	)

	// --- DNS ----------------------------------------------------------------

	r.registerDestructiveCloudAWS("vmware_cloudaws_sddc_dns_update_public",
		"Update the DNS records of an SDDC's management VMs to use public IP addresses (PUT /vmc/api/orgs/{org}/sddcs/{sddc}/dns/public). Tier 2, not Tier 1: pure DNS/network configuration, does not provision or deprovision any host.",
		tier2,
		schema(map[string]interface{}{"org": orgArg, "sddc": sddcArg, "confirm": confirmArg}, "org", "sddc", "confirm"),
		Tool{CloudHandler: handleCloudAWSSDDCDNSUpdatePublic},
	)

	r.registerDestructiveCloudAWS("vmware_cloudaws_sddc_dns_update_private",
		"Update the DNS records of an SDDC's management VMs to use private IP addresses (PUT /vmc/api/orgs/{org}/sddcs/{sddc}/dns/private). Tier 2, not Tier 1: pure DNS/network configuration, does not provision or deprovision any host.",
		tier2,
		schema(map[string]interface{}{"org": orgArg, "sddc": sddcArg, "confirm": confirmArg}, "org", "sddc", "confirm"),
		Tool{CloudHandler: handleCloudAWSSDDCDNSUpdatePrivate},
	)

	// --- Public IPs -----------------------------------------------------------

	idArg := map[string]interface{}{"type": "string", "description": "Public IP allocation ID (opaque identifier returned by vmware_cloudaws_sddc_publicip_list/vmware_cloudaws_sddc_publicip_create)."}

	r.registerCloudAWS("vmware_cloudaws_sddc_publicip_list",
		"List every public IP allocated to an SDDC (GET /vmc/api/orgs/{org}/sddcs/{sddc}/publicips).",
		schema(map[string]interface{}{"org": orgArg, "sddc": sddcArg}, "org", "sddc"),
		Tool{CloudHandler: handleCloudAWSSDDCPublicIPList},
	)

	r.registerCloudAWS("vmware_cloudaws_sddc_publicip_get",
		"Get details of one public IP allocated to an SDDC (GET /vmc/api/orgs/{org}/sddcs/{sddc}/publicips/{id}).",
		schema(map[string]interface{}{"org": orgArg, "sddc": sddcArg, "id": idArg}, "org", "sddc", "id"),
		Tool{CloudHandler: handleCloudAWSSDDCPublicIPGet},
	)

	r.registerDestructiveCloudAWS("vmware_cloudaws_sddc_publicip_create",
		"Allocate a new public IP for an SDDC (POST /vmc/api/orgs/{org}/sddcs/{sddc}/publicips) — a billable AWS resource. \"spec\" is optional (the vendored spec documents this body only as placeholder \"allocation spec\" prose, with no confirmed field list); omit it to allocate with VMC's defaults, or pass fields per VMware's VMC API docs.",
		tier1,
		schema(map[string]interface{}{
			"org":     orgArg,
			"sddc":    sddcArg,
			"spec":    specArg("Public IP allocation spec — JSON object, optional, fields per VMware's VMC API docs."),
			"confirm": confirmArg,
		}, "org", "sddc", "confirm"),
		Tool{CloudHandler: handleCloudAWSSDDCPublicIPCreate},
	)

	r.registerDestructiveCloudAWS("vmware_cloudaws_sddc_publicip_update",
		"Attach or detach a public IP to/from a workload VM in an SDDC (PATCH /vmc/api/orgs/{org}/sddcs/{sddc}/publicips/{id}?action={action}) — can reassociate the IP or affect its connectivity. \"action\" selects attach vs. detach (per VMware's VMC API docs); \"spec\" is optional (the vendored spec documents this body only as placeholder \"SddcPublicIp object to update\" prose).",
		tier1,
		schema(map[string]interface{}{
			"org":     orgArg,
			"sddc":    sddcArg,
			"id":      idArg,
			"action":  map[string]interface{}{"type": "string", "description": `Action to perform, e.g. "attach" or "detach" — passed through as-is to the "action" query parameter.`},
			"spec":    specArg("SddcPublicIp update document — JSON object, optional, fields per VMware's VMC API docs."),
			"confirm": confirmArg,
		}, "org", "sddc", "id", "action", "confirm"),
		Tool{CloudHandler: handleCloudAWSSDDCPublicIPUpdate},
	)

	r.registerDestructiveCloudAWS("vmware_cloudaws_sddc_publicip_delete",
		"Release (free) a public IP allocated to an SDDC (DELETE /vmc/api/orgs/{org}/sddcs/{sddc}/publicips/{id}). Irreversible — the specific address is returned to AWS's pool and is not guaranteed to be re-allocatable later.",
		tier1,
		schema(map[string]interface{}{"org": orgArg, "sddc": sddcArg, "id": idArg, "confirm": confirmArg}, "org", "sddc", "id", "confirm"),
		Tool{CloudHandler: handleCloudAWSSDDCPublicIPDelete},
	)

	// --- Addon credentials ------------------------------------------------

	addonTypeArg := map[string]interface{}{"type": "string", "description": `Addon type the credential belongs to, e.g. "HCX".`}
	credentialNameArg := map[string]interface{}{"type": "string", "description": "Credential name (opaque identifier within the addon type)."}

	r.registerCloudAWS("vmware_cloudaws_sddc_addon_credential_list",
		"List every credential associated with an addon type (e.g. HCX) within an SDDC (GET /vmc/api/orgs/{org}/sddcs/{sddcId}/addons/{addonType}/credentials).",
		schema(map[string]interface{}{"org": orgArg, "sddc": sddcArg, "addon_type": addonTypeArg}, "org", "sddc", "addon_type"),
		Tool{CloudHandler: handleCloudAWSSDDCAddonCredentialList},
	)

	r.registerCloudAWS("vmware_cloudaws_sddc_addon_credential_get",
		"Get details of one addon credential by name (GET /vmc/api/orgs/{org}/sddcs/{sddcId}/addons/{addonType}/credentials/{name}).",
		schema(map[string]interface{}{"org": orgArg, "sddc": sddcArg, "addon_type": addonTypeArg, "name": credentialNameArg}, "org", "sddc", "addon_type", "name"),
		Tool{CloudHandler: handleCloudAWSSDDCAddonCredentialGet},
	)

	r.registerDestructiveCloudAWS("vmware_cloudaws_sddc_addon_credential_create",
		"Associate a new addon credential (e.g. for HCX) with an SDDC (POST /vmc/api/orgs/{org}/sddcs/{sddcId}/addons/{addonType}/credentials). Tier 2, not Tier 1: an addon service credential, not infrastructure provisioning. Body shape (\"Credentials creation payload\") is not concretely documented in this project's vendored spec; pass fields per VMware's VMC API docs.",
		tier2,
		schema(map[string]interface{}{
			"org":        orgArg,
			"sddc":       sddcArg,
			"addon_type": addonTypeArg,
			"spec":       specArg("Credential creation payload — JSON object, fields per VMware's VMC API docs."),
			"confirm":    confirmArg,
		}, "org", "sddc", "addon_type", "spec", "confirm"),
		Tool{CloudHandler: handleCloudAWSSDDCAddonCredentialCreate},
	)

	r.registerDestructiveCloudAWS("vmware_cloudaws_sddc_addon_credential_update",
		"Update an existing addon credential's details (PUT /vmc/api/orgs/{org}/sddcs/{sddcId}/addons/{addonType}/credentials/{name}). Tier 2, not Tier 1: an addon service credential, not infrastructure provisioning. Body shape (\"Credentials update payload\") is not concretely documented in this project's vendored spec; pass fields per VMware's VMC API docs.",
		tier2,
		schema(map[string]interface{}{
			"org":        orgArg,
			"sddc":       sddcArg,
			"addon_type": addonTypeArg,
			"name":       credentialNameArg,
			"spec":       specArg("Credential update payload — JSON object, fields per VMware's VMC API docs."),
			"confirm":    confirmArg,
		}, "org", "sddc", "addon_type", "name", "spec", "confirm"),
		Tool{CloudHandler: handleCloudAWSSDDCAddonCredentialUpdate},
	)
}

// --- arg helpers -----------------------------------------------------------
// Prefixed cloudAWSSDDC* (not a generic name like requireString) to avoid a
// package-level name collision with the other 3 Fase 10 groups' files
// (Orgs/Networking-Core/Networking-Edge), written in parallel by separate
// agents against the same tools package.

func cloudAWSSDDCRequireString(args map[string]interface{}, key string) (string, error) {
	v, _ := args[key].(string)
	if v == "" {
		return "", fmt.Errorf("%s is required", key)
	}
	return v, nil
}

func cloudAWSSDDCRequireObject(args map[string]interface{}, key string) (map[string]interface{}, error) {
	v, ok := args[key].(map[string]interface{})
	if !ok || len(v) == 0 {
		return nil, fmt.Errorf("%s (a non-empty JSON object) is required", key)
	}
	return v, nil
}

// cloudAWSSDDCOptionalObject returns args[key] as a map if present and
// non-empty, or nil otherwise — nil is a valid "no body" value for
// cloudaws.Client.Do.
func cloudAWSSDDCOptionalObject(args map[string]interface{}, key string) map[string]interface{} {
	if v, ok := args[key].(map[string]interface{}); ok && len(v) > 0 {
		return v
	}
	return nil
}

// cloudAWSSDDCWithFallback returns out if the server sent a decodable
// response body, or fallback otherwise — several write operations here
// (delete/convert/dns-update) may return either a Task object or an empty
// 2xx body depending on VMC's real implementation (unverifiable — no VMC
// account available to this project, see this file's top doc comment), so
// the tool always reports *something* useful instead of an empty string.
func cloudAWSSDDCWithFallback(out interface{}, fallback map[string]interface{}) interface{} {
	if out == nil {
		return fallback
	}
	return out
}

// --- SDDCs (base) handlers ---------------------------------------------

func handleCloudAWSSDDCList(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, err := cloudAWSSDDCRequireString(args, "org")
	if err != nil {
		return "", err
	}
	var out interface{}
	if err := client.Do(ctx, http.MethodGet, "/vmc/api/orgs/"+url.PathEscape(org)+"/sddcs", nil, &out); err != nil {
		return "", fmt.Errorf("failed to list SDDCs for org %q: %w", org, err)
	}
	return marshalJSON(out)
}

func handleCloudAWSSDDCGet(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, err := cloudAWSSDDCRequireString(args, "org")
	if err != nil {
		return "", err
	}
	sddc, err := cloudAWSSDDCRequireString(args, "sddc")
	if err != nil {
		return "", err
	}
	var out interface{}
	path := "/vmc/api/orgs/" + url.PathEscape(org) + "/sddcs/" + url.PathEscape(sddc)
	if err := client.Do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return "", fmt.Errorf("failed to get SDDC %q in org %q: %w", sddc, org, err)
	}
	return marshalJSON(out)
}

func handleCloudAWSSDDCCreate(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, err := cloudAWSSDDCRequireString(args, "org")
	if err != nil {
		return "", err
	}
	spec, err := cloudAWSSDDCRequireObject(args, "spec")
	if err != nil {
		return "", err
	}
	var out interface{}
	path := "/vmc/api/orgs/" + url.PathEscape(org) + "/sddcs"
	if err := client.Do(ctx, http.MethodPost, path, spec, &out); err != nil {
		return "", fmt.Errorf("failed to create SDDC in org %q: %w", org, err)
	}
	return marshalJSON(out)
}

func handleCloudAWSSDDCDelete(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, err := cloudAWSSDDCRequireString(args, "org")
	if err != nil {
		return "", err
	}
	sddc, err := cloudAWSSDDCRequireString(args, "sddc")
	if err != nil {
		return "", err
	}
	var out interface{}
	path := "/vmc/api/orgs/" + url.PathEscape(org) + "/sddcs/" + url.PathEscape(sddc)
	if err := client.Do(ctx, http.MethodDelete, path, nil, &out); err != nil {
		return "", fmt.Errorf("failed to delete SDDC %q in org %q: %w", sddc, org, err)
	}
	return marshalJSON(cloudAWSSDDCWithFallback(out, map[string]interface{}{"org": org, "sddc": sddc, "deleted": true}))
}

func handleCloudAWSSDDCUpdate(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, err := cloudAWSSDDCRequireString(args, "org")
	if err != nil {
		return "", err
	}
	sddc, err := cloudAWSSDDCRequireString(args, "sddc")
	if err != nil {
		return "", err
	}
	spec, err := cloudAWSSDDCRequireObject(args, "spec")
	if err != nil {
		return "", err
	}
	var out interface{}
	path := "/vmc/api/orgs/" + url.PathEscape(org) + "/sddcs/" + url.PathEscape(sddc)
	if err := client.Do(ctx, http.MethodPatch, path, spec, &out); err != nil {
		return "", fmt.Errorf("failed to update SDDC %q in org %q: %w", sddc, org, err)
	}
	return marshalJSON(out)
}

func handleCloudAWSSDDCConvert(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, err := cloudAWSSDDCRequireString(args, "org")
	if err != nil {
		return "", err
	}
	sddc, err := cloudAWSSDDCRequireString(args, "sddc")
	if err != nil {
		return "", err
	}
	var out interface{}
	path := "/vmc/api/orgs/" + url.PathEscape(org) + "/sddcs/" + url.PathEscape(sddc) + "/convert"
	if err := client.Do(ctx, http.MethodPost, path, nil, &out); err != nil {
		return "", fmt.Errorf("failed to convert SDDC %q in org %q: %w", sddc, org, err)
	}
	return marshalJSON(cloudAWSSDDCWithFallback(out, map[string]interface{}{"org": org, "sddc": sddc, "converted": true}))
}

// --- Cluster handlers ---------------------------------------------------

func handleCloudAWSSDDCClusterCreate(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, err := cloudAWSSDDCRequireString(args, "org")
	if err != nil {
		return "", err
	}
	sddc, err := cloudAWSSDDCRequireString(args, "sddc")
	if err != nil {
		return "", err
	}
	spec, err := cloudAWSSDDCRequireObject(args, "spec")
	if err != nil {
		return "", err
	}
	var out interface{}
	path := "/vmc/api/orgs/" + url.PathEscape(org) + "/sddcs/" + url.PathEscape(sddc) + "/clusters"
	if err := client.Do(ctx, http.MethodPost, path, spec, &out); err != nil {
		return "", fmt.Errorf("failed to create cluster in SDDC %q (org %q): %w", sddc, org, err)
	}
	return marshalJSON(out)
}

func handleCloudAWSSDDCClusterDelete(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, err := cloudAWSSDDCRequireString(args, "org")
	if err != nil {
		return "", err
	}
	sddc, err := cloudAWSSDDCRequireString(args, "sddc")
	if err != nil {
		return "", err
	}
	cluster, err := cloudAWSSDDCRequireString(args, "cluster")
	if err != nil {
		return "", err
	}
	var out interface{}
	path := "/vmc/api/orgs/" + url.PathEscape(org) + "/sddcs/" + url.PathEscape(sddc) + "/clusters/" + url.PathEscape(cluster)
	if err := client.Do(ctx, http.MethodDelete, path, nil, &out); err != nil {
		return "", fmt.Errorf("failed to delete cluster %q from SDDC %q (org %q): %w", cluster, sddc, org, err)
	}
	return marshalJSON(cloudAWSSDDCWithFallback(out, map[string]interface{}{"org": org, "sddc": sddc, "cluster": cluster, "deleted": true}))
}

// --- Hosts handler --------------------------------------------------------

func handleCloudAWSSDDCHostsUpdate(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, err := cloudAWSSDDCRequireString(args, "org")
	if err != nil {
		return "", err
	}
	sddc, err := cloudAWSSDDCRequireString(args, "sddc")
	if err != nil {
		return "", err
	}
	action, err := cloudAWSSDDCRequireString(args, "action")
	if err != nil {
		return "", err
	}
	spec, err := cloudAWSSDDCRequireObject(args, "spec")
	if err != nil {
		return "", err
	}

	q := url.Values{}
	q.Set("action", action)
	path := "/vmc/api/orgs/" + url.PathEscape(org) + "/sddcs/" + url.PathEscape(sddc) + "/esxs?" + q.Encode()

	var out interface{}
	if err := client.Do(ctx, http.MethodPost, path, spec, &out); err != nil {
		return "", fmt.Errorf("failed to %s host(s) in SDDC %q (org %q): %w", action, sddc, org, err)
	}
	return marshalJSON(out)
}

// --- Cluster/EDRS handlers ------------------------------------------------

func handleCloudAWSSDDCEDRSClusterList(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, err := cloudAWSSDDCRequireString(args, "org")
	if err != nil {
		return "", err
	}
	sddc, err := cloudAWSSDDCRequireString(args, "sddc")
	if err != nil {
		return "", err
	}
	cluster, err := cloudAWSSDDCRequireString(args, "cluster")
	if err != nil {
		return "", err
	}
	var out interface{}
	path := "/vmc/autoscaler/api/orgs/" + url.PathEscape(org) + "/sddcs/" + url.PathEscape(sddc) + "/clusters/" + url.PathEscape(cluster) + "/edrs-policy"
	if err := client.Do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return "", fmt.Errorf("failed to get EDRS policy for cluster %q in SDDC %q (org %q): %w", cluster, sddc, org, err)
	}
	return marshalJSON(out)
}

func handleCloudAWSSDDCEDRSList(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, err := cloudAWSSDDCRequireString(args, "org")
	if err != nil {
		return "", err
	}
	sddc, err := cloudAWSSDDCRequireString(args, "sddc")
	if err != nil {
		return "", err
	}
	var out interface{}
	path := "/vmc/autoscaler/api/orgs/" + url.PathEscape(org) + "/sddcs/" + url.PathEscape(sddc) + "/edrs-policy"
	if err := client.Do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return "", fmt.Errorf("failed to list EDRS policies for SDDC %q (org %q): %w", sddc, org, err)
	}
	return marshalJSON(out)
}

func handleCloudAWSSDDCEDRSClusterSet(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, err := cloudAWSSDDCRequireString(args, "org")
	if err != nil {
		return "", err
	}
	sddc, err := cloudAWSSDDCRequireString(args, "sddc")
	if err != nil {
		return "", err
	}
	cluster, err := cloudAWSSDDCRequireString(args, "cluster")
	if err != nil {
		return "", err
	}
	spec, err := cloudAWSSDDCRequireObject(args, "spec")
	if err != nil {
		return "", err
	}
	var out interface{}
	path := "/vmc/autoscaler/api/orgs/" + url.PathEscape(org) + "/sddcs/" + url.PathEscape(sddc) + "/clusters/" + url.PathEscape(cluster) + "/edrs-policy"
	if err := client.Do(ctx, http.MethodPost, path, spec, &out); err != nil {
		return "", fmt.Errorf("failed to set EDRS policy for cluster %q in SDDC %q (org %q): %w", cluster, sddc, org, err)
	}
	return marshalJSON(out)
}

// --- DNS handlers -----------------------------------------------------

func handleCloudAWSSDDCDNSUpdatePublic(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, err := cloudAWSSDDCRequireString(args, "org")
	if err != nil {
		return "", err
	}
	sddc, err := cloudAWSSDDCRequireString(args, "sddc")
	if err != nil {
		return "", err
	}
	var out interface{}
	path := "/vmc/api/orgs/" + url.PathEscape(org) + "/sddcs/" + url.PathEscape(sddc) + "/dns/public"
	if err := client.Do(ctx, http.MethodPut, path, nil, &out); err != nil {
		return "", fmt.Errorf("failed to update public DNS for SDDC %q (org %q): %w", sddc, org, err)
	}
	return marshalJSON(cloudAWSSDDCWithFallback(out, map[string]interface{}{"org": org, "sddc": sddc, "dns": "public", "updated": true}))
}

func handleCloudAWSSDDCDNSUpdatePrivate(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, err := cloudAWSSDDCRequireString(args, "org")
	if err != nil {
		return "", err
	}
	sddc, err := cloudAWSSDDCRequireString(args, "sddc")
	if err != nil {
		return "", err
	}
	var out interface{}
	path := "/vmc/api/orgs/" + url.PathEscape(org) + "/sddcs/" + url.PathEscape(sddc) + "/dns/private"
	if err := client.Do(ctx, http.MethodPut, path, nil, &out); err != nil {
		return "", fmt.Errorf("failed to update private DNS for SDDC %q (org %q): %w", sddc, org, err)
	}
	return marshalJSON(cloudAWSSDDCWithFallback(out, map[string]interface{}{"org": org, "sddc": sddc, "dns": "private", "updated": true}))
}

// --- Public IP handlers -------------------------------------------------

func handleCloudAWSSDDCPublicIPList(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, err := cloudAWSSDDCRequireString(args, "org")
	if err != nil {
		return "", err
	}
	sddc, err := cloudAWSSDDCRequireString(args, "sddc")
	if err != nil {
		return "", err
	}
	var out interface{}
	path := "/vmc/api/orgs/" + url.PathEscape(org) + "/sddcs/" + url.PathEscape(sddc) + "/publicips"
	if err := client.Do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return "", fmt.Errorf("failed to list public IPs for SDDC %q (org %q): %w", sddc, org, err)
	}
	return marshalJSON(out)
}

func handleCloudAWSSDDCPublicIPGet(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, err := cloudAWSSDDCRequireString(args, "org")
	if err != nil {
		return "", err
	}
	sddc, err := cloudAWSSDDCRequireString(args, "sddc")
	if err != nil {
		return "", err
	}
	id, err := cloudAWSSDDCRequireString(args, "id")
	if err != nil {
		return "", err
	}
	var out interface{}
	path := "/vmc/api/orgs/" + url.PathEscape(org) + "/sddcs/" + url.PathEscape(sddc) + "/publicips/" + url.PathEscape(id)
	if err := client.Do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return "", fmt.Errorf("failed to get public IP %q for SDDC %q (org %q): %w", id, sddc, org, err)
	}
	return marshalJSON(out)
}

func handleCloudAWSSDDCPublicIPCreate(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, err := cloudAWSSDDCRequireString(args, "org")
	if err != nil {
		return "", err
	}
	sddc, err := cloudAWSSDDCRequireString(args, "sddc")
	if err != nil {
		return "", err
	}
	spec := cloudAWSSDDCOptionalObject(args, "spec")

	var out interface{}
	path := "/vmc/api/orgs/" + url.PathEscape(org) + "/sddcs/" + url.PathEscape(sddc) + "/publicips"
	if err := client.Do(ctx, http.MethodPost, path, spec, &out); err != nil {
		return "", fmt.Errorf("failed to allocate public IP for SDDC %q (org %q): %w", sddc, org, err)
	}
	return marshalJSON(out)
}

func handleCloudAWSSDDCPublicIPUpdate(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, err := cloudAWSSDDCRequireString(args, "org")
	if err != nil {
		return "", err
	}
	sddc, err := cloudAWSSDDCRequireString(args, "sddc")
	if err != nil {
		return "", err
	}
	id, err := cloudAWSSDDCRequireString(args, "id")
	if err != nil {
		return "", err
	}
	action, err := cloudAWSSDDCRequireString(args, "action")
	if err != nil {
		return "", err
	}
	spec := cloudAWSSDDCOptionalObject(args, "spec")

	q := url.Values{}
	q.Set("action", action)
	path := "/vmc/api/orgs/" + url.PathEscape(org) + "/sddcs/" + url.PathEscape(sddc) + "/publicips/" + url.PathEscape(id) + "?" + q.Encode()

	var out interface{}
	if err := client.Do(ctx, http.MethodPatch, path, spec, &out); err != nil {
		return "", fmt.Errorf("failed to %s public IP %q for SDDC %q (org %q): %w", action, id, sddc, org, err)
	}
	return marshalJSON(out)
}

func handleCloudAWSSDDCPublicIPDelete(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, err := cloudAWSSDDCRequireString(args, "org")
	if err != nil {
		return "", err
	}
	sddc, err := cloudAWSSDDCRequireString(args, "sddc")
	if err != nil {
		return "", err
	}
	id, err := cloudAWSSDDCRequireString(args, "id")
	if err != nil {
		return "", err
	}
	var out interface{}
	path := "/vmc/api/orgs/" + url.PathEscape(org) + "/sddcs/" + url.PathEscape(sddc) + "/publicips/" + url.PathEscape(id)
	if err := client.Do(ctx, http.MethodDelete, path, nil, &out); err != nil {
		return "", fmt.Errorf("failed to delete public IP %q for SDDC %q (org %q): %w", id, sddc, org, err)
	}
	return marshalJSON(cloudAWSSDDCWithFallback(out, map[string]interface{}{"org": org, "sddc": sddc, "id": id, "deleted": true}))
}

// --- Addon credential handlers -------------------------------------------

func handleCloudAWSSDDCAddonCredentialList(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, err := cloudAWSSDDCRequireString(args, "org")
	if err != nil {
		return "", err
	}
	sddc, err := cloudAWSSDDCRequireString(args, "sddc")
	if err != nil {
		return "", err
	}
	addonType, err := cloudAWSSDDCRequireString(args, "addon_type")
	if err != nil {
		return "", err
	}
	var out interface{}
	path := "/vmc/api/orgs/" + url.PathEscape(org) + "/sddcs/" + url.PathEscape(sddc) + "/addons/" + url.PathEscape(addonType) + "/credentials"
	if err := client.Do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return "", fmt.Errorf("failed to list %s addon credentials for SDDC %q (org %q): %w", addonType, sddc, org, err)
	}
	return marshalJSON(out)
}

func handleCloudAWSSDDCAddonCredentialGet(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, err := cloudAWSSDDCRequireString(args, "org")
	if err != nil {
		return "", err
	}
	sddc, err := cloudAWSSDDCRequireString(args, "sddc")
	if err != nil {
		return "", err
	}
	addonType, err := cloudAWSSDDCRequireString(args, "addon_type")
	if err != nil {
		return "", err
	}
	name, err := cloudAWSSDDCRequireString(args, "name")
	if err != nil {
		return "", err
	}
	var out interface{}
	path := "/vmc/api/orgs/" + url.PathEscape(org) + "/sddcs/" + url.PathEscape(sddc) + "/addons/" + url.PathEscape(addonType) + "/credentials/" + url.PathEscape(name)
	if err := client.Do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return "", fmt.Errorf("failed to get %s addon credential %q for SDDC %q (org %q): %w", addonType, name, sddc, org, err)
	}
	return marshalJSON(out)
}

func handleCloudAWSSDDCAddonCredentialCreate(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, err := cloudAWSSDDCRequireString(args, "org")
	if err != nil {
		return "", err
	}
	sddc, err := cloudAWSSDDCRequireString(args, "sddc")
	if err != nil {
		return "", err
	}
	addonType, err := cloudAWSSDDCRequireString(args, "addon_type")
	if err != nil {
		return "", err
	}
	spec, err := cloudAWSSDDCRequireObject(args, "spec")
	if err != nil {
		return "", err
	}
	var out interface{}
	path := "/vmc/api/orgs/" + url.PathEscape(org) + "/sddcs/" + url.PathEscape(sddc) + "/addons/" + url.PathEscape(addonType) + "/credentials"
	if err := client.Do(ctx, http.MethodPost, path, spec, &out); err != nil {
		return "", fmt.Errorf("failed to create %s addon credential for SDDC %q (org %q): %w", addonType, sddc, org, err)
	}
	return marshalJSON(out)
}

func handleCloudAWSSDDCAddonCredentialUpdate(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, err := cloudAWSSDDCRequireString(args, "org")
	if err != nil {
		return "", err
	}
	sddc, err := cloudAWSSDDCRequireString(args, "sddc")
	if err != nil {
		return "", err
	}
	addonType, err := cloudAWSSDDCRequireString(args, "addon_type")
	if err != nil {
		return "", err
	}
	name, err := cloudAWSSDDCRequireString(args, "name")
	if err != nil {
		return "", err
	}
	spec, err := cloudAWSSDDCRequireObject(args, "spec")
	if err != nil {
		return "", err
	}
	var out interface{}
	path := "/vmc/api/orgs/" + url.PathEscape(org) + "/sddcs/" + url.PathEscape(sddc) + "/addons/" + url.PathEscape(addonType) + "/credentials/" + url.PathEscape(name)
	if err := client.Do(ctx, http.MethodPut, path, spec, &out); err != nil {
		return "", fmt.Errorf("failed to update %s addon credential %q for SDDC %q (org %q): %w", addonType, name, sddc, org, err)
	}
	return marshalJSON(out)
}

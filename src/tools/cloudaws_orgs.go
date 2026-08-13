// Fase 10 "Grupo 1" — Orgs (organization/account management) for VMware
// Cloud on AWS (VMC), a third product distinct from vSphere/ESXi and
// VMware Workstation Pro with no govmomi dependency — see cloudaws/client.go.
// Routes confirmed by reading `.workspace/VMware Cloud on AWS APIs.postman_
// collection.json`'s "Orgs" folder directly (all sub-folders: Reservations,
// Tasks, Providers, SDDC Template, Storage, Account Link, Subscription,
// Support Window, plus the top-level Details/List), not guessed.
//
// No VMC account/simulator is available to this project (see the plan's
// Fase 10 "não-verificável-ponta-a-ponta" note) — every response is decoded
// generically (interface{} + re-marshal) instead of a hand-typed struct,
// same posture as tools/appliance.go's applianceGet and tools/
// workstation_network.go's wsGet, for the same reason: a guessed struct
// risks silently dropping/misnaming fields this project has no way to
// verify against a live VMC endpoint.
//
// Financial risk: unlike every other domain in this project, a VMC SDDC (and
// a VMC subscription) costs real money per hour/term while it exists — an
// error in tier classification here has financial, not just operational,
// consequences. This file's tier assignments were decided explicitly per
// route (not "DELETE=tier1, everything else=tier2"):
//   - vmware_cloudaws_subscription_create (POST .../subscriptions) is tier1
//     even though it's a Create, not a Delete: it commits the org to a real
//     billing obligation.
//   - vmware_cloudaws_account_link_delete (DELETE .../connected-accounts/
//     {id}) is tier1: unlinking an AWS account from the VMC org, optionally
//     even with forceEvenWhenSddcPresent=true while active SDDCs still exist
//     on that account — irreversible and high-impact.
//   - vmware_cloudaws_sddc_template_delete is tier1 (irreversible delete of
//     a reusable deployment template).
//   - Every other write (PUT/POST that mutates org-level config, not
//     billing/account-linkage) is tier2 (disruptive but reversible/re-doable
//     — maintenance windows, task cancellation, subnet zone remapping,
//     support window moves).
//   - Every GET/list/details/calculation route is untiered (registered via
//     r.registerCloudAWS, not r.registerDestructiveCloudAWS).
//
// Curation decisions (deviations from a literal 1:1 route mapping):
//   - "Subscription/Offers/List" and "Subscription/List Available by
//     Region" are the exact same method+path+query
//     (GET .../subscriptions/offer-instances?region=&product_type=&
//     product=&type=) duplicated twice in the collection under different
//     folder names — collapsed into one tool,
//     vmware_cloudaws_subscription_offers_list.
//   - Every write route whose Postman example body is a bare placeholder
//     string (e.g. "Maintenance Window", "subscriptionRequest" — not a real
//     JSON schema; VMC's actual request shape for these isn't documented
//     anywhere this project can reach) takes a generic freeform "body"
//     object argument, forwarded to VMC as-is. Marked required where an
//     empty body would make the call a pointless no-op (PUT maintenance
//     window, POST subscriptions, POST map-customer-zones, POST
//     compatible-subnets-async, PUT support-window) — see each tool's
//     schema below — and left optional only for the one write route whose
//     own Postman raw body is empty ("") in the source collection:
//     vmware_cloudaws_account_link_compatible_subnets_calculate (POST
//     .../account-link/compatible-subnets, synchronous "calculate" variant
//     — this one doesn't persist anything, per the plan's note).
//   - vmware_cloudaws_account_link_compatible_subnets_async_get requires all
//     3 of its query parameters (linked_account_id, region, sddc): without
//     all 3 the lookup can't identify what to compute subnets for.
//   - vmware_cloudaws_storage_cluster_constraints_list requires both
//     provider and num_hosts: the endpoint computes a constraint for a
//     specific provider+host-count combination, not a generic list.
//   - vmware_cloudaws_subscription_offers_list requires region and
//     product_type (the description explicitly frames the endpoint as
//     scoped "for the specific product type in the specific region"),
//     leaving product/type as optional narrowing filters.
//   - vmware_cloudaws_task_list_filtered is kept as its own tool distinct
//     from vmware_cloudaws_task_list (same path, +$filter query) per this
//     fase's plan note — not fused, since the two have meaningfully
//     different call shapes (unfiltered vs required-filter).
package tools

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/cslsoftwares/mcpvmware/cloudaws"
)

// cloudGet issues a GET against path (already relative to VMC's API base,
// e.g. "/vmc/api/orgs") and marshals whatever VMC returned — see this file's
// top doc comment for why the decode is generic, not a typed struct.
func cloudGet(ctx context.Context, client *cloudaws.Client, path string) (string, error) {
	var out interface{}
	if err := client.Do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return "", err
	}
	return marshalJSON(out)
}

// cloudMutate issues method (POST/PUT/DELETE) against path with body (nil
// when the route takes none) and marshals the response. Several VMC
// mutation endpoints return no body at all — cloudMutate reports a plain
// {"result": "ok"} in that case instead of an empty string, matching
// workstation_network.go's wsMutate convention.
func cloudMutate(ctx context.Context, client *cloudaws.Client, method, path string, body interface{}) (string, error) {
	var out interface{}
	if err := client.Do(ctx, method, path, body, &out); err != nil {
		return "", err
	}
	if out == nil {
		return marshalJSON(map[string]interface{}{"result": "ok"})
	}
	return marshalJSON(out)
}

// cloudQueryString builds a URL query string from key/value pairs, skipping
// any pair whose value is empty — so an optional filter the caller didn't
// provide doesn't appear as "key=" on the wire. Returns "" (not "?") when
// every pair is empty, so callers can just append the result to a path.
func cloudQueryString(pairs ...[2]string) string {
	v := url.Values{}
	for _, p := range pairs {
		if p[1] == "" {
			continue
		}
		v.Set(p[0], p[1])
	}
	if len(v) == 0 {
		return ""
	}
	return "?" + v.Encode()
}

// cloudOptionalString reads args[key] as a string, defaulting to "" — used
// for optional VMC query parameters; VMC itself decides what an absent
// parameter means, this project just doesn't force a value.
func cloudOptionalString(args map[string]interface{}, key string) string {
	v, _ := args[key].(string)
	return v
}

// cloudOptionalInt reads args[key] as an optional integer, returning "" when
// absent so cloudQueryString drops it — accepts the same float64/int/int32/
// int64 shapes as toInt64 (vm.go), tolerant of a caller/test passing a bare
// Go number instead of the float64 real JSON-RPC decodes to.
func cloudOptionalInt(args map[string]interface{}, key string) (string, error) {
	v, ok := args[key]
	if !ok || v == nil {
		return "", nil
	}
	n, err := toInt64(v)
	if err != nil {
		return "", fmt.Errorf("invalid %s: %w", key, err)
	}
	return strconv.FormatInt(n, 10), nil
}

// cloudOptionalBool reads args[key] as an optional boolean, returning "" when
// absent so cloudQueryString drops it.
func cloudOptionalBool(args map[string]interface{}, key string) string {
	v, ok := args[key].(bool)
	if !ok {
		return ""
	}
	if v {
		return "true"
	}
	return "false"
}

// cloudBodyArg builds the schema fragment shared by every write route in
// this file whose real VMC JSON shape isn't documented anywhere this
// project can reach (see the top doc comment).
func cloudBodyArg(desc string) map[string]interface{} {
	return map[string]interface{}{
		"type":        "object",
		"description": desc + " VMC's exact JSON schema for this request body isn't documented in the vendored Postman collection (its example is a placeholder description string, not a schema) — pass whatever fields the real VMC API expects for this call; this argument is forwarded to VMC as-is, unvalidated.",
	}
}

// requiredObjectArg reads args[key] as a required non-nil JSON object.
func requiredObjectArg(args map[string]interface{}, key string) (map[string]interface{}, error) {
	v, ok := args[key]
	if !ok || v == nil {
		return nil, fmt.Errorf("%s is required", key)
	}
	m, ok := v.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("%s must be a JSON object", key)
	}
	return m, nil
}

// registerCloudAWSOrgsTools registers the 29 Orgs-domain tools — see this
// file's top doc comment for tier/curation reasoning.
func registerCloudAWSOrgsTools(r *Registry) {
	confirmArg := map[string]interface{}{
		"type":        "boolean",
		"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
	}
	orgArg := map[string]interface{}{
		"type":        "string",
		"description": "VMC on AWS organization ID (opaque identifier — see vmware_cloudaws_org_list).",
	}
	emptySchema := map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}

	// --- top-level: List/Details ------------------------------------------

	r.registerCloudAWS("vmware_cloudaws_org_list",
		"List every VMC on AWS organization the authenticated user is authorized on (GET /vmc/api/orgs).",
		emptySchema,
		Tool{CloudHandler: handleCloudAWSOrgList},
	)

	r.registerCloudAWS("vmware_cloudaws_org_details",
		"Get details for one VMC on AWS organization (GET /vmc/api/orgs/{org}).",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"org": orgArg},
			"required":   []interface{}{"org"},
		},
		Tool{CloudHandler: handleCloudAWSOrgDetails},
	)

	// --- Reservations -------------------------------------------------------

	r.registerCloudAWS("vmware_cloudaws_reservation_list",
		"List all capacity reservations for a VMC on AWS organization (GET /vmc/api/orgs/{org}/reservations).",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"org": orgArg},
			"required":   []interface{}{"org"},
		},
		Tool{CloudHandler: handleCloudAWSReservationList},
	)

	r.registerCloudAWS("vmware_cloudaws_reservation_maintenance_window_get",
		"Get the maintenance window for a reservation's SDDC (GET /vmc/api/orgs/{org}/reservations/{reservation}/mw).",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"org":         orgArg,
				"reservation": map[string]interface{}{"type": "string", "description": "Reservation ID (see vmware_cloudaws_reservation_list)."},
			},
			"required": []interface{}{"org", "reservation"},
		},
		Tool{CloudHandler: handleCloudAWSReservationMaintenanceWindowGet},
	)

	r.registerDestructiveCloudAWS("vmware_cloudaws_reservation_maintenance_window_update",
		"Update the maintenance window for a reservation's SDDC (PUT /vmc/api/orgs/{org}/reservations/{reservation}/mw). Disruptive but reversible — the window can be changed again.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"org":         orgArg,
				"reservation": map[string]interface{}{"type": "string", "description": "Reservation ID (see vmware_cloudaws_reservation_list)."},
				"body":        cloudBodyArg("The new maintenance window."),
				"confirm":     confirmArg,
			},
			"required": []interface{}{"org", "reservation", "body", "confirm"},
		},
		Tool{CloudHandler: handleCloudAWSReservationMaintenanceWindowUpdate},
	)

	// --- Tasks ----------------------------------------------------------------

	r.registerCloudAWS("vmware_cloudaws_task_list",
		"List all tasks for a VMC on AWS organization (GET /vmc/api/orgs/{org}/tasks).",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"org": orgArg},
			"required":   []interface{}{"org"},
		},
		Tool{CloudHandler: handleCloudAWSTaskList},
	)

	r.registerCloudAWS("vmware_cloudaws_task_list_filtered",
		"List tasks for a VMC on AWS organization with an OData-style $filter expression (GET /vmc/api/orgs/{org}/tasks?$filter=...). Distinct from vmware_cloudaws_task_list — this one always requires an explicit filter.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"org":    orgArg,
				"filter": map[string]interface{}{"type": "string", "description": "OData-style $filter expression VMC applies server-side to the task list."},
			},
			"required": []interface{}{"org", "filter"},
		},
		Tool{CloudHandler: handleCloudAWSTaskListFiltered},
	)

	r.registerCloudAWS("vmware_cloudaws_task_details",
		"Get details of one task (GET /vmc/api/orgs/{org}/tasks/{task}).",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"org":  orgArg,
				"task": map[string]interface{}{"type": "string", "description": "Task ID."},
			},
			"required": []interface{}{"org", "task"},
		},
		Tool{CloudHandler: handleCloudAWSTaskDetails},
	)

	r.registerDestructiveCloudAWS("vmware_cloudaws_task_action",
		"Request an action on a running task, e.g. cancel it (POST /vmc/api/orgs/{org}/tasks/{task}?action=...). Advisory only — some tasks aren't cancelable, and a cancellation request may take an arbitrary amount of time to take effect; the task must still be monitored afterward. Reversible in the sense that it only requests a state change, not a data-destroying action.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"org":     orgArg,
				"task":    map[string]interface{}{"type": "string", "description": "Task ID."},
				"action":  map[string]interface{}{"type": "string", "description": `Action to request on the task, e.g. "cancel".`},
				"confirm": confirmArg,
			},
			"required": []interface{}{"org", "task", "action", "confirm"},
		},
		Tool{CloudHandler: handleCloudAWSTaskAction},
	)

	// --- Providers --------------------------------------------------------------

	r.registerCloudAWS("vmware_cloudaws_provider_list",
		"List enabled cloud providers for a VMC on AWS organization (GET /vmc/api/orgs/{org}/providers).",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"org": orgArg},
			"required":   []interface{}{"org"},
		},
		Tool{CloudHandler: handleCloudAWSProviderList},
	)

	// --- SDDC Template ------------------------------------------------------------

	r.registerCloudAWS("vmware_cloudaws_sddc_template_details",
		"Get an SDDC configuration template by ID (GET /vmc/api/orgs/{org}/sddc-templates/{templateId}).",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"org":         orgArg,
				"template_id": map[string]interface{}{"type": "string", "description": "SDDC template ID (see vmware_cloudaws_sddc_template_list)."},
			},
			"required": []interface{}{"org", "template_id"},
		},
		Tool{CloudHandler: handleCloudAWSSDDCTemplateDetails},
	)

	r.registerDestructiveCloudAWS("vmware_cloudaws_sddc_template_delete",
		"Delete an SDDC configuration template. Irreversible (DELETE /vmc/api/orgs/{org}/sddc-templates/{templateId}).",
		tier1,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"org":         orgArg,
				"template_id": map[string]interface{}{"type": "string", "description": "SDDC template ID (see vmware_cloudaws_sddc_template_list)."},
				"confirm":     confirmArg,
			},
			"required": []interface{}{"org", "template_id", "confirm"},
		},
		Tool{CloudHandler: handleCloudAWSSDDCTemplateDelete},
	)

	r.registerCloudAWS("vmware_cloudaws_sddc_template_list",
		"List all available SDDC configuration templates in an organization (GET /vmc/api/orgs/{org}/sddc-templates).",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"org": orgArg},
			"required":   []interface{}{"org"},
		},
		Tool{CloudHandler: handleCloudAWSSDDCTemplateList},
	)

	r.registerCloudAWS("vmware_cloudaws_sddc_template_for_sddc",
		"Get the configuration template associated with a specific SDDC (GET /vmc/api/orgs/{org}/sddcs/{sddc}/sddc-template) — distinct route from vmware_cloudaws_sddc_template_details, which looks up a template by its own ID.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"org":  orgArg,
				"sddc": map[string]interface{}{"type": "string", "description": "SDDC ID."},
			},
			"required": []interface{}{"org", "sddc"},
		},
		Tool{CloudHandler: handleCloudAWSSDDCTemplateForSDDC},
	)

	// --- Storage ------------------------------------------------------------------

	r.registerCloudAWS("vmware_cloudaws_storage_cluster_constraints_list",
		"Get constraints on cluster storage size for EBS-backed clusters, for a given provider and host count (GET /vmc/api/orgs/{org}/storage/cluster-constraints?provider=&num_hosts=).",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"org":       orgArg,
				"provider":  map[string]interface{}{"type": "string", "description": `Cloud provider (e.g. "AWS").`},
				"num_hosts": map[string]interface{}{"type": "integer", "description": "Number of hosts to compute the storage constraint for."},
			},
			"required": []interface{}{"org", "provider", "num_hosts"},
		},
		Tool{CloudHandler: handleCloudAWSStorageClusterConstraintsList},
	)

	// --- Account Link ---------------------------------------------------------------

	r.registerCloudAWS("vmware_cloudaws_account_link_sddc_connections_list",
		"List SDDC connections currently set up for the organization's linked AWS account(s) (GET /vmc/api/orgs/{org}/account-link/sddc-connections?sddc=).",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"org":  orgArg,
				"sddc": map[string]interface{}{"type": "string", "description": "Optional SDDC ID to filter by. Omit to list every SDDC connection."},
			},
			"required": []interface{}{"org"},
		},
		Tool{CloudHandler: handleCloudAWSAccountLinkSDDCConnectionsList},
	)

	r.registerCloudAWS("vmware_cloudaws_account_link_compatible_subnets_async_get",
		"Get a customer's compatible subnets for account linking, computed asynchronously via a task — the result lands in task.params['subnet_list_result'] once the task completes (GET /vmc/api/orgs/{org}/account-link/compatible-subnets-async?linkedAccountId=&region=&sddc=). Use vmware_cloudaws_task_details to poll the returned task.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"org":               orgArg,
				"linked_account_id": map[string]interface{}{"type": "string", "description": "Linked AWS account ID."},
				"region":            map[string]interface{}{"type": "string", "description": "AWS region."},
				"sddc":              map[string]interface{}{"type": "string", "description": "SDDC ID."},
			},
			"required": []interface{}{"org", "linked_account_id", "region", "sddc"},
		},
		Tool{CloudHandler: handleCloudAWSAccountLinkCompatibleSubnetsAsyncGet},
	)

	r.registerDestructiveCloudAWS("vmware_cloudaws_account_link_compatible_subnets_async_create",
		"Set which subnet to use to link accounts and finish the linking process, asynchronously via a task (POST /vmc/api/orgs/{org}/account-link/compatible-subnets-async). Disruptive but reversible — account linking can be redone.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"org":     orgArg,
				"body":    cloudBodyArg("The subnet chosen by the customer."),
				"confirm": confirmArg,
			},
			"required": []interface{}{"org", "body", "confirm"},
		},
		Tool{CloudHandler: handleCloudAWSAccountLinkCompatibleSubnetsAsyncCreate},
	)

	r.registerCloudAWS("vmware_cloudaws_account_link_compatible_subnets_calculate",
		"Set which subnet to use to link accounts and finish the linking process, synchronously (POST /vmc/api/orgs/{org}/account-link/compatible-subnets). A calculation/selection call, not a persisting create — untiered.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"org":  orgArg,
				"body": cloudBodyArg("The subnet chosen by the customer. Optional — VMC's own example for this route ships an empty body."),
			},
			"required": []interface{}{"org"},
		},
		Tool{CloudHandler: handleCloudAWSAccountLinkCompatibleSubnetsCalculate},
	)

	r.registerCloudAWS("vmware_cloudaws_account_link_url_create",
		"Get a URL the customer opens in a browser to start the AWS account-linking process (GET /vmc/api/orgs/{org}/account-link). Despite the GET verb this doesn't register anything on VMC's side yet — it only mints a URL for the user to follow.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"org": orgArg},
			"required":   []interface{}{"org"},
		},
		Tool{CloudHandler: handleCloudAWSAccountLinkURLCreate},
	)

	r.registerDestructiveCloudAWS("vmware_cloudaws_account_link_delete",
		"Delete (unlink) a connected AWS account from the organization. Irreversible and high-impact — optionally forceable even while SDDCs still exist on that account (DELETE /vmc/api/orgs/{org}/account-link/connected-accounts/{linkedAccountPathId}?forceEvenWhenSddcPresent=).",
		tier1,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"org":                          orgArg,
				"linked_account_path_id":       map[string]interface{}{"type": "string", "description": "Linked account's path ID (see vmware_cloudaws_account_link_connected_accounts_list)."},
				"force_even_when_sddc_present": map[string]interface{}{"type": "boolean", "description": "If true, unlink even when the account still has active SDDCs on it. Defaults to false (VMC's own default) when omitted."},
				"confirm":                      confirmArg,
			},
			"required": []interface{}{"org", "linked_account_path_id", "confirm"},
		},
		Tool{CloudHandler: handleCloudAWSAccountLinkDelete},
	)

	r.registerDestructiveCloudAWS("vmware_cloudaws_account_link_map_customer_zones",
		"Create a task to re-map the customer's datacenters across zones (POST /vmc/api/orgs/{org}/account-link/map-customer-zones). Disruptive but reversible — zones can be remapped again.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"org":     orgArg,
				"body":    cloudBodyArg("Who to map and what to map."),
				"confirm": confirmArg,
			},
			"required": []interface{}{"org", "body", "confirm"},
		},
		Tool{CloudHandler: handleCloudAWSAccountLinkMapCustomerZones},
	)

	r.registerCloudAWS("vmware_cloudaws_account_link_connected_accounts_list",
		"List connected (linked) AWS accounts for the organization (GET /vmc/api/orgs/{org}/account-link/connected-accounts?provider=).",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"org":      orgArg,
				"provider": map[string]interface{}{"type": "string", "description": "Optional cloud provider to filter by. Omit to list every connected account."},
			},
			"required": []interface{}{"org"},
		},
		Tool{CloudHandler: handleCloudAWSAccountLinkConnectedAccountsList},
	)

	// --- Subscription ------------------------------------------------------------

	r.registerCloudAWS("vmware_cloudaws_subscription_offers_list",
		"List subscription offers available for a specific product type in a specific region (GET /vmc/api/orgs/{org}/subscriptions/offer-instances?region=&product_type=&product=&type=). Same route registered once even though the source Postman collection lists it twice under different folder names (\"Offers/List\" and \"List Available by Region\").",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"org":          orgArg,
				"region":       map[string]interface{}{"type": "string", "description": "AWS region to list offers for."},
				"product_type": map[string]interface{}{"type": "string", "description": "Product type to list offers for."},
				"product":      map[string]interface{}{"type": "string", "description": "Optional product filter."},
				"type":         map[string]interface{}{"type": "string", "description": "Optional subscription type filter."},
			},
			"required": []interface{}{"org", "region", "product_type"},
		},
		Tool{CloudHandler: handleCloudAWSSubscriptionOffersList},
	)

	r.registerDestructiveCloudAWS("vmware_cloudaws_subscription_create",
		"Initiate the creation of a subscription (POST /vmc/api/orgs/{org}/subscriptions). Tier 1: commits the organization to a real billing obligation — not just an irreversible API call, an irreversible financial one.",
		tier1,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"org":     orgArg,
				"body":    cloudBodyArg("The subscription request."),
				"confirm": confirmArg,
			},
			"required": []interface{}{"org", "body", "confirm"},
		},
		Tool{CloudHandler: handleCloudAWSSubscriptionCreate},
	)

	r.registerCloudAWS("vmware_cloudaws_subscription_details",
		"Get subscription details for a given subscription ID (GET /vmc/api/orgs/{org}/subscriptions/{subscription}).",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"org":          orgArg,
				"subscription": map[string]interface{}{"type": "string", "description": "Subscription ID."},
			},
			"required": []interface{}{"org", "subscription"},
		},
		Tool{CloudHandler: handleCloudAWSSubscriptionDetails},
	)

	r.registerCloudAWS("vmware_cloudaws_subscription_products_list",
		"List all products available for subscription purchase (GET /vmc/api/orgs/{org}/subscriptions/products).",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"org": orgArg},
			"required":   []interface{}{"org"},
		},
		Tool{CloudHandler: handleCloudAWSSubscriptionProductsList},
	)

	// --- Support Window ------------------------------------------------------------

	r.registerDestructiveCloudAWS("vmware_cloudaws_support_window_update",
		"Move an SDDC to a new support window (PUT /vmc/api/orgs/{org}/tbrs/support-window/{id}). Disruptive but reversible — the SDDC can be moved to a different window again.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"org":               orgArg,
				"support_window_id": map[string]interface{}{"type": "string", "description": "Support window ID."},
				"body":              cloudBodyArg("The SDDC to move."),
				"confirm":           confirmArg,
			},
			"required": []interface{}{"org", "support_window_id", "body", "confirm"},
		},
		Tool{CloudHandler: handleCloudAWSSupportWindowUpdate},
	)

	r.registerCloudAWS("vmware_cloudaws_support_window_list",
		"List all available support windows (GET /vmc/api/orgs/{org}/tbrs/support-window?minimumSeatsAvailable=&createdBy=).",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"org":                     orgArg,
				"minimum_seats_available": map[string]interface{}{"type": "integer", "description": "Optional minimum number of available seats to filter by."},
				"created_by":              map[string]interface{}{"type": "string", "description": "Optional creator to filter by."},
			},
			"required": []interface{}{"org"},
		},
		Tool{CloudHandler: handleCloudAWSSupportWindowList},
	)
}

// --- handlers: top-level -----------------------------------------------------

func handleCloudAWSOrgList(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	return cloudGet(ctx, client, "/vmc/api/orgs")
}

func handleCloudAWSOrgDetails(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, err := requiredStringArg(args, "org")
	if err != nil {
		return "", err
	}
	return cloudGet(ctx, client, "/vmc/api/orgs/"+org)
}

// --- handlers: Reservations --------------------------------------------------

func handleCloudAWSReservationList(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, err := requiredStringArg(args, "org")
	if err != nil {
		return "", err
	}
	return cloudGet(ctx, client, "/vmc/api/orgs/"+org+"/reservations")
}

func handleCloudAWSReservationMaintenanceWindowGet(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, err := requiredStringArg(args, "org")
	if err != nil {
		return "", err
	}
	reservation, err := requiredStringArg(args, "reservation")
	if err != nil {
		return "", err
	}
	return cloudGet(ctx, client, "/vmc/api/orgs/"+org+"/reservations/"+reservation+"/mw")
}

func handleCloudAWSReservationMaintenanceWindowUpdate(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, err := requiredStringArg(args, "org")
	if err != nil {
		return "", err
	}
	reservation, err := requiredStringArg(args, "reservation")
	if err != nil {
		return "", err
	}
	body, err := requiredObjectArg(args, "body")
	if err != nil {
		return "", err
	}
	return cloudMutate(ctx, client, http.MethodPut, "/vmc/api/orgs/"+org+"/reservations/"+reservation+"/mw", body)
}

// --- handlers: Tasks -----------------------------------------------------------

func handleCloudAWSTaskList(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, err := requiredStringArg(args, "org")
	if err != nil {
		return "", err
	}
	return cloudGet(ctx, client, "/vmc/api/orgs/"+org+"/tasks")
}

func handleCloudAWSTaskListFiltered(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, err := requiredStringArg(args, "org")
	if err != nil {
		return "", err
	}
	filter, err := requiredStringArg(args, "filter")
	if err != nil {
		return "", err
	}
	q := cloudQueryString([2]string{"$filter", filter})
	return cloudGet(ctx, client, "/vmc/api/orgs/"+org+"/tasks"+q)
}

func handleCloudAWSTaskDetails(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, err := requiredStringArg(args, "org")
	if err != nil {
		return "", err
	}
	task, err := requiredStringArg(args, "task")
	if err != nil {
		return "", err
	}
	return cloudGet(ctx, client, "/vmc/api/orgs/"+org+"/tasks/"+task)
}

func handleCloudAWSTaskAction(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, err := requiredStringArg(args, "org")
	if err != nil {
		return "", err
	}
	task, err := requiredStringArg(args, "task")
	if err != nil {
		return "", err
	}
	action, err := requiredStringArg(args, "action")
	if err != nil {
		return "", err
	}
	q := cloudQueryString([2]string{"action", action})
	return cloudMutate(ctx, client, http.MethodPost, "/vmc/api/orgs/"+org+"/tasks/"+task+q, nil)
}

// --- handlers: Providers ------------------------------------------------------

func handleCloudAWSProviderList(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, err := requiredStringArg(args, "org")
	if err != nil {
		return "", err
	}
	return cloudGet(ctx, client, "/vmc/api/orgs/"+org+"/providers")
}

// --- handlers: SDDC Template ---------------------------------------------------

func handleCloudAWSSDDCTemplateDetails(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, err := requiredStringArg(args, "org")
	if err != nil {
		return "", err
	}
	templateID, err := requiredStringArg(args, "template_id")
	if err != nil {
		return "", err
	}
	return cloudGet(ctx, client, "/vmc/api/orgs/"+org+"/sddc-templates/"+templateID)
}

func handleCloudAWSSDDCTemplateDelete(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, err := requiredStringArg(args, "org")
	if err != nil {
		return "", err
	}
	templateID, err := requiredStringArg(args, "template_id")
	if err != nil {
		return "", err
	}
	return cloudMutate(ctx, client, http.MethodDelete, "/vmc/api/orgs/"+org+"/sddc-templates/"+templateID, nil)
}

func handleCloudAWSSDDCTemplateList(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, err := requiredStringArg(args, "org")
	if err != nil {
		return "", err
	}
	return cloudGet(ctx, client, "/vmc/api/orgs/"+org+"/sddc-templates")
}

func handleCloudAWSSDDCTemplateForSDDC(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, err := requiredStringArg(args, "org")
	if err != nil {
		return "", err
	}
	sddc, err := requiredStringArg(args, "sddc")
	if err != nil {
		return "", err
	}
	return cloudGet(ctx, client, "/vmc/api/orgs/"+org+"/sddcs/"+sddc+"/sddc-template")
}

// --- handlers: Storage ----------------------------------------------------------

func handleCloudAWSStorageClusterConstraintsList(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, err := requiredStringArg(args, "org")
	if err != nil {
		return "", err
	}
	provider, err := requiredStringArg(args, "provider")
	if err != nil {
		return "", err
	}
	numHosts, err := cloudOptionalInt(args, "num_hosts")
	if err != nil {
		return "", err
	}
	if numHosts == "" {
		return "", fmt.Errorf("num_hosts is required")
	}
	q := cloudQueryString([2]string{"provider", provider}, [2]string{"num_hosts", numHosts})
	return cloudGet(ctx, client, "/vmc/api/orgs/"+org+"/storage/cluster-constraints"+q)
}

// --- handlers: Account Link ---------------------------------------------------

func handleCloudAWSAccountLinkSDDCConnectionsList(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, err := requiredStringArg(args, "org")
	if err != nil {
		return "", err
	}
	sddc := cloudOptionalString(args, "sddc")
	q := cloudQueryString([2]string{"sddc", sddc})
	return cloudGet(ctx, client, "/vmc/api/orgs/"+org+"/account-link/sddc-connections"+q)
}

func handleCloudAWSAccountLinkCompatibleSubnetsAsyncGet(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, err := requiredStringArg(args, "org")
	if err != nil {
		return "", err
	}
	linkedAccountID, err := requiredStringArg(args, "linked_account_id")
	if err != nil {
		return "", err
	}
	region, err := requiredStringArg(args, "region")
	if err != nil {
		return "", err
	}
	sddc, err := requiredStringArg(args, "sddc")
	if err != nil {
		return "", err
	}
	q := cloudQueryString([2]string{"linkedAccountId", linkedAccountID}, [2]string{"region", region}, [2]string{"sddc", sddc})
	return cloudGet(ctx, client, "/vmc/api/orgs/"+org+"/account-link/compatible-subnets-async"+q)
}

func handleCloudAWSAccountLinkCompatibleSubnetsAsyncCreate(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, err := requiredStringArg(args, "org")
	if err != nil {
		return "", err
	}
	body, err := requiredObjectArg(args, "body")
	if err != nil {
		return "", err
	}
	return cloudMutate(ctx, client, http.MethodPost, "/vmc/api/orgs/"+org+"/account-link/compatible-subnets-async", body)
}

func handleCloudAWSAccountLinkCompatibleSubnetsCalculate(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, err := requiredStringArg(args, "org")
	if err != nil {
		return "", err
	}
	// body is optional here — VMC's own example ships an empty raw body for
	// this route (see this file's top doc comment).
	body, _ := args["body"]
	return cloudMutate(ctx, client, http.MethodPost, "/vmc/api/orgs/"+org+"/account-link/compatible-subnets", body)
}

func handleCloudAWSAccountLinkURLCreate(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, err := requiredStringArg(args, "org")
	if err != nil {
		return "", err
	}
	return cloudGet(ctx, client, "/vmc/api/orgs/"+org+"/account-link")
}

func handleCloudAWSAccountLinkDelete(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, err := requiredStringArg(args, "org")
	if err != nil {
		return "", err
	}
	linkedAccountPathID, err := requiredStringArg(args, "linked_account_path_id")
	if err != nil {
		return "", err
	}
	force := cloudOptionalBool(args, "force_even_when_sddc_present")
	q := cloudQueryString([2]string{"forceEvenWhenSddcPresent", force})
	return cloudMutate(ctx, client, http.MethodDelete, "/vmc/api/orgs/"+org+"/account-link/connected-accounts/"+linkedAccountPathID+q, nil)
}

func handleCloudAWSAccountLinkMapCustomerZones(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, err := requiredStringArg(args, "org")
	if err != nil {
		return "", err
	}
	body, err := requiredObjectArg(args, "body")
	if err != nil {
		return "", err
	}
	return cloudMutate(ctx, client, http.MethodPost, "/vmc/api/orgs/"+org+"/account-link/map-customer-zones", body)
}

func handleCloudAWSAccountLinkConnectedAccountsList(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, err := requiredStringArg(args, "org")
	if err != nil {
		return "", err
	}
	provider := cloudOptionalString(args, "provider")
	q := cloudQueryString([2]string{"provider", provider})
	return cloudGet(ctx, client, "/vmc/api/orgs/"+org+"/account-link/connected-accounts"+q)
}

// --- handlers: Subscription ---------------------------------------------------

func handleCloudAWSSubscriptionOffersList(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, err := requiredStringArg(args, "org")
	if err != nil {
		return "", err
	}
	region, err := requiredStringArg(args, "region")
	if err != nil {
		return "", err
	}
	productType, err := requiredStringArg(args, "product_type")
	if err != nil {
		return "", err
	}
	product := cloudOptionalString(args, "product")
	typ := cloudOptionalString(args, "type")
	q := cloudQueryString(
		[2]string{"region", region},
		[2]string{"product_type", productType},
		[2]string{"product", product},
		[2]string{"type", typ},
	)
	return cloudGet(ctx, client, "/vmc/api/orgs/"+org+"/subscriptions/offer-instances"+q)
}

func handleCloudAWSSubscriptionCreate(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, err := requiredStringArg(args, "org")
	if err != nil {
		return "", err
	}
	body, err := requiredObjectArg(args, "body")
	if err != nil {
		return "", err
	}
	return cloudMutate(ctx, client, http.MethodPost, "/vmc/api/orgs/"+org+"/subscriptions", body)
}

func handleCloudAWSSubscriptionDetails(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, err := requiredStringArg(args, "org")
	if err != nil {
		return "", err
	}
	subscription, err := requiredStringArg(args, "subscription")
	if err != nil {
		return "", err
	}
	return cloudGet(ctx, client, "/vmc/api/orgs/"+org+"/subscriptions/"+subscription)
}

func handleCloudAWSSubscriptionProductsList(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, err := requiredStringArg(args, "org")
	if err != nil {
		return "", err
	}
	return cloudGet(ctx, client, "/vmc/api/orgs/"+org+"/subscriptions/products")
}

// --- handlers: Support Window --------------------------------------------------

func handleCloudAWSSupportWindowUpdate(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, err := requiredStringArg(args, "org")
	if err != nil {
		return "", err
	}
	id, err := requiredStringArg(args, "support_window_id")
	if err != nil {
		return "", err
	}
	body, err := requiredObjectArg(args, "body")
	if err != nil {
		return "", err
	}
	return cloudMutate(ctx, client, http.MethodPut, "/vmc/api/orgs/"+org+"/tbrs/support-window/"+id, body)
}

func handleCloudAWSSupportWindowList(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
	org, err := requiredStringArg(args, "org")
	if err != nil {
		return "", err
	}
	minSeats, err := cloudOptionalInt(args, "minimum_seats_available")
	if err != nil {
		return "", err
	}
	createdBy := cloudOptionalString(args, "created_by")
	q := cloudQueryString([2]string{"minimumSeatsAvailable", minSeats}, [2]string{"createdBy", createdBy})
	return cloudGet(ctx, client, "/vmc/api/orgs/"+org+"/tbrs/support-window"+q)
}

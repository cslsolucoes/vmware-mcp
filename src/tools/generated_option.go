package tools

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/cslsoftwares/mcpvmware/vmware"
)

// registerOptionTools is the Fase 1 pilot output of the codegen plan
// (".workspace/plans/MCPVMWare2026-08-10-175300-plano-cobertura-completa-api-codegen.plan.md")
// — hand-transcribed from src/gen/classification.json's object.OptionManager
// entries (2 methods: Query, Update) following the same handler/schema/test
// conventions as tools/host.go, to prove the pipeline end-to-end on a small,
// low-risk domain before Fases 2-8 generate ~500 more.
//
// Curation deviations from the raw classification (this IS what Fase 1 is
// for — a human reviewing what the generator proposed):
//   - object.OptionManager is generic — govmomi wires it both at the
//     ServiceContent (global) level and per-host
//     (HostConfigManager.OptionManager). The generator can't know which one
//     a given deployment cares about; picked host-scoped here (ESXi advanced
//     settings, the more common admin task, and consistent with the
//     existing resolveHost/hostArg pattern in host.go) and renamed the tools
//     from the generator's proposed vmware_option_query/vmware_option_update
//     to vmware_host_option_query/vmware_host_option_update — "option" alone
//     is too ambiguous for an LLM caller to guess the scope from the name.
//   - Update takes []types.BaseOptionValue (a polymorphic interface); the
//     schema accepts plain {key, value} objects and the handler constructs
//     []*types.OptionValue itself — same "accept the concrete struct, not
//     the interface" approach as vm.go's reconfigure, per the plan's Fase 0
//     schema-strategy item 2(b).
func registerOptionTools(r *Registry) {
	hostArg := map[string]interface{}{
		"type":        "string",
		"description": `Host identifier: a name/pattern (e.g. "esxi-01.local") or a full inventory path (e.g. "/ha-datacenter/host/esxi-01.local/esxi-01.local") as returned by vmware_list_hosts. Must resolve to exactly one host.`,
	}

	r.register("vmware_host_option_query",
		"List advanced configuration options (key/value settings) on an ESXi host, optionally filtered by a name prefix.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host": hostArg,
				"name": map[string]interface{}{"type": "string", "description": `Option name or dotted-prefix to filter by (e.g. "Net"). Empty (default) returns every option.`},
			},
			"required": []interface{}{"host"},
		},
		Tool{Handler: handleHostOptionQuery},
	)

	r.registerDestructive("vmware_host_option_update",
		"Set one or more advanced configuration options (key/value settings) on an ESXi host. Reversible by setting the option(s) back to their previous value, but some options only take effect after a service restart or host reboot.",
		tier2,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host": hostArg,
				"options": map[string]interface{}{
					"type":        "array",
					"description": `Options to set, e.g. [{"key":"Net.Something","value":1}]. Each item must have "key" (string) and "value" (any scalar).`,
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"key":   map[string]interface{}{"type": "string"},
							"value": map[string]interface{}{},
						},
						"required": []interface{}{"key", "value"},
					},
				},
				"confirm": map[string]interface{}{
					"type":        "boolean",
					"description": "Must be exactly true to run. The server also requires --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE to be enabled at startup — this argument alone is not enough.",
				},
			},
			"required": []interface{}{"host", "options", "confirm"},
		},
		Tool{Handler: handleHostOptionUpdate},
	)
}

func handleHostOptionQuery(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}

	om, err := host.ConfigManager().OptionManager(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get option manager for %s: %w", host.InventoryPath, err)
	}

	name, _ := args["name"].(string)
	opts, err := om.Query(ctx, name)
	if err != nil {
		if !isInvalidNameFault(err) {
			return "", fmt.Errorf("failed to query options on %s: %w", host.InventoryPath, err)
		}
		// Real vSphere (confirmed against vcsim, not assumed — see
		// referencia/govmomi/simulator/option_manager.go) faults with
		// InvalidName when the name filter matches zero options, instead of
		// returning an empty list. Every other list-style tool in this
		// project treats "0 results" as success, not an error (see
		// finder.go's emptyOnNotFound / bug-002) — match that convention
		// here instead of surfacing a confusing SOAP fault to the caller.
		opts = nil
	}

	result := make([]map[string]interface{}, 0, len(opts))
	for _, o := range opts {
		v := o.GetOptionValue()
		result = append(result, map[string]interface{}{"key": v.Key, "value": v.Value})
	}

	return marshalJSON(map[string]interface{}{
		"host":    host.InventoryPath,
		"count":   len(result),
		"options": result,
	})
}

// isInvalidNameFault reports whether err is vSphere's InvalidName SOAP
// fault — see handleHostOptionQuery's doc comment for why this needs
// special handling rather than being treated as a generic error.
//
// A direct (non-Task) SOAP call like QueryOptions surfaces its fault via
// soap.WrapSoapFault (confirmed by reading vim25/soap/client.go's
// soapRoundTrip, not guessed) — soap.IsVimFault/ToVimFault is the wrong
// pair here (that's for types.LocalizedMethodFault-based Task errors);
// *soap.Fault.VimFault() is what unwraps to the concrete fault detail.
func isInvalidNameFault(err error) bool {
	if !soap.IsSoapFault(err) {
		return false
	}
	_, ok := soap.ToSoapFault(err).VimFault().(types.InvalidName)
	return ok
}

func handleHostOptionUpdate(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
	host, err := resolveHost(ctx, client, args)
	if err != nil {
		return "", err
	}

	raw, ok := args["options"].([]interface{})
	if !ok || len(raw) == 0 {
		return "", fmt.Errorf("options is required and must be a non-empty array of {key, value}")
	}

	values := make([]types.BaseOptionValue, 0, len(raw))
	for i, item := range raw {
		m, ok := item.(map[string]interface{})
		if !ok {
			return "", fmt.Errorf("options[%d] must be an object with \"key\" and \"value\"", i)
		}
		key, _ := m["key"].(string)
		if key == "" {
			return "", fmt.Errorf("options[%d].key is required", i)
		}
		values = append(values, &types.OptionValue{Key: key, Value: m["value"]})
	}

	om, err := host.ConfigManager().OptionManager(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get option manager for %s: %w", host.InventoryPath, err)
	}

	if err := om.Update(ctx, values); err != nil {
		return "", fmt.Errorf("failed to update options on %s: %w", host.InventoryPath, err)
	}

	return marshalJSON(map[string]interface{}{
		"host":   host.InventoryPath,
		"result": "updated",
		"count":  len(values),
	})
}

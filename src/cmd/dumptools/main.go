// Command dumptools emits the exact MCP tool catalog (name + description +
// inputSchema) for every connection mode, as JSON on stdout. It builds a
// Registry with a nil client per mode — NewRegistry never dereferences the
// client at construction (handlers are function values, never invoked here),
// so this is a safe, fully offline dump of the same definitions the running
// server hands to tools/list. Used to generate the tool reference docs.
package main

import (
	"context"
	"encoding/json"
	"os"

	"github.com/cslsoftwares/mcpvmware/tools"
)

type dumpTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

func dump(mode tools.ConnectionMode) []dumpTool {
	r := tools.NewRegistry(context.Background(), nil, tools.RegistryOptions{
		ConnectionMode:   mode,
		AllowDestructive: true, // harmless: destructive tools list regardless; gate is call-time
	})
	var out []dumpTool
	for _, t := range r.ListTools() {
		out = append(out, dumpTool{Name: t.Name, Description: t.Description, InputSchema: t.InputSchema})
	}
	return out
}

func main() {
	catalog := map[string][]dumpTool{
		"vcenter":     dump(tools.ConnectionModeVCenter),     // vcenter-only + vsphere-general (907)
		"vmware":      dump(tools.ConnectionModeVMware),      // vsphere-general only (386)
		"workstation": dump(tools.ConnectionModeWorkstation), // 28
		"cloudaws":    dump(tools.ConnectionModeCloudAWS),    // 95
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(catalog); err != nil {
		os.Exit(1)
	}
}

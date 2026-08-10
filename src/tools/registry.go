// Package tools registers MCP tools backed by a vmware.Client, one file per
// VMware API namespace (see tools/system.go for the seed tools) — mirroring
// the registration pattern of the reference truenas-mcp project.
package tools

import (
	"context"
	"fmt"
	"sort"

	"github.com/cslsoftwares/mcpvmware/mcp"
	"github.com/cslsoftwares/mcpvmware/vmware"
)

// Registry holds the connected VMware client and every registered tool.
type Registry struct {
	client *vmware.Client
	ctx    context.Context
	tools  map[string]Tool
}

// Tool pairs an MCP tool definition with its handler.
type Tool struct {
	Definition mcp.Tool
	Handler    func(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error)
}

// NewRegistry builds a Registry bound to client and registers every tool.
// ctx is the long-lived context used for the underlying govmomi calls; the
// stdio handler passes context.Background() today (no per-request
// cancellation yet — see cmd/mcpvmware-mcp/main.go).
func NewRegistry(ctx context.Context, client *vmware.Client) *Registry {
	r := &Registry{client: client, ctx: ctx, tools: make(map[string]Tool)}
	r.registerTools()
	return r
}

func (r *Registry) registerTools() {
	registerSystemTools(r)
}

// register adds a tool to the registry. Tool files call this from an
// init-style registerXTools(r *Registry) function invoked by registerTools.
func (r *Registry) register(name, description string, schema map[string]interface{}, handler Tool) {
	r.tools[name] = Tool{
		Definition: mcp.Tool{Name: name, Description: description, InputSchema: schema},
		Handler:    handler.Handler,
	}
}

// ListTools implements mcp.ToolRegistry.
func (r *Registry) ListTools() []mcp.Tool {
	list := make([]mcp.Tool, 0, len(r.tools))
	for _, t := range r.tools {
		list = append(list, t.Definition)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
	return list
}

// CallTool implements mcp.ToolRegistry.
func (r *Registry) CallTool(name string, args map[string]interface{}) (string, error) {
	t, ok := r.tools[name]
	if !ok {
		return "", fmt.Errorf("unknown tool: %s", name)
	}
	return t.Handler(r.ctx, r.client, args)
}

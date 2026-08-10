// Command mcpvmware-mcp is the MCP (Model Context Protocol) server
// entrypoint for MCPVMWare. It connects to a vCenter/ESXi endpoint via the
// vendored github.com/vmware/govmomi client and exposes VMware operations
// as MCP tools over stdio, mirroring the architecture of the reference
// truenas-mcp project (cmd/truenas-mcp/main.go).
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/cslsoftwares/mcpvmware/mcp"
	"github.com/cslsoftwares/mcpvmware/tools"
	"github.com/cslsoftwares/mcpvmware/vmware"
)

var (
	vcenterURL = flag.String("vcenter-url", "", "vCenter/ESXi hostname or SDK URL (e.g. 'vcenter.local' or 'https://vcenter.local/sdk')")
	username   = flag.String("username", "", "vCenter/ESXi username")
	password   = flag.String("password", "", "vCenter/ESXi password")
	insecure   = flag.Bool("insecure", false, "Disable TLS certificate verification (UNSAFE: allows man-in-the-middle attacks)")
	versionFlg = flag.Bool("version", false, "Print version and exit")
	debug      = flag.Bool("debug", false, "Enable debug logging")
)

// Version is the release version, injected at build time via
// -ldflags "-X main.Version=...". Builds without injection report "dev".
var Version = "dev"

func main() {
	flag.Parse()

	if *versionFlg {
		fmt.Printf("mcpvmware-mcp version %s\n", Version)
		os.Exit(0)
	}

	if *vcenterURL == "" {
		*vcenterURL = os.Getenv("VCENTER_URL")
	}
	if *username == "" {
		*username = os.Getenv("VCENTER_USERNAME")
	}
	if *password == "" {
		*password = os.Getenv("VCENTER_PASSWORD")
	}
	if !*insecure {
		switch strings.ToLower(os.Getenv("VCENTER_INSECURE")) {
		case "", "0", "false", "no", "off":
		default:
			*insecure = true
		}
	}
	if *debug || strings.ToLower(os.Getenv("MCPVMWARE_DEBUG")) == "1" {
		*debug = true
	}

	if *vcenterURL == "" || *username == "" || *password == "" {
		log.Fatal("--vcenter-url, --username and --password are required (or set VCENTER_URL, VCENTER_USERNAME, VCENTER_PASSWORD env vars)")
	}
	if *insecure {
		log.Println("WARNING: TLS certificate verification disabled - the connection is vulnerable to man-in-the-middle attacks")
	}

	ctx := context.Background()
	client, err := vmware.NewClient(ctx, vmware.Config{
		URL:      *vcenterURL,
		Username: *username,
		Password: *password,
		Insecure: *insecure,
	})
	if err != nil {
		log.Fatalf("Failed to connect to vCenter/ESXi: %v", err)
	}
	defer client.Close(ctx)

	registry := tools.NewRegistry(ctx, client)

	handler := NewStdioHandler(registry, *debug)
	if err := handler.Run(); err != nil {
		log.Fatalf("Stdio handler error: %v", err)
	}
}

// StdioHandler manages stdio communication for the MCP protocol.
type StdioHandler struct {
	registry    mcp.ToolRegistry
	stdin       *bufio.Scanner
	stdoutMutex sync.Mutex
	debug       bool
}

func NewStdioHandler(registry mcp.ToolRegistry, debug bool) *StdioHandler {
	return &StdioHandler{
		registry: registry,
		stdin:    bufio.NewScanner(os.Stdin),
		debug:    debug,
	}
}

func (h *StdioHandler) Run() error {
	if h.debug {
		log.Println("Starting stdio handler...")
	}

	for h.stdin.Scan() {
		line := h.stdin.Bytes()
		if h.debug {
			log.Printf("[STDIN] %s", line)
		}

		var req mcp.Request
		if err := json.Unmarshal(line, &req); err != nil {
			h.sendError(nil, -32700, fmt.Sprintf("Parse error: %v", err))
			continue
		}

		if h.debug {
			log.Printf("Handling method: %s (id: %v)", req.Method, req.ID)
		}

		resp := h.handleRequest(&req)
		if resp != nil {
			if err := h.sendResponse(resp); err != nil {
				log.Printf("Failed to send response: %v", err)
			}
		}
	}

	if err := h.stdin.Err(); err != nil {
		return fmt.Errorf("stdin error: %w", err)
	}
	return nil
}

func (h *StdioHandler) handleRequest(req *mcp.Request) *mcp.Response {
	switch req.Method {
	case "initialize":
		return h.handleInitialize(req)
	case "notifications/initialized":
		return nil
	case "tools/list":
		return h.handleToolsList(req)
	case "tools/call":
		return h.handleToolsCall(req)
	default:
		if req.ID != nil {
			return h.createErrorResponse(req.ID, -32601, "Method not found")
		}
		return nil
	}
}

func (h *StdioHandler) handleInitialize(req *mcp.Request) *mcp.Response {
	result := mcp.InitializeResult{
		ProtocolVersion: "2024-11-05",
		ServerInfo:      mcp.ServerInfo{Name: "mcpvmware-mcp", Version: Version},
		Capabilities: mcp.Capabilities{
			Tools: map[string]interface{}{"listChanged": false},
		},
	}
	return &mcp.Response{JSONRPC: "2.0", ID: req.ID, Result: result}
}

func (h *StdioHandler) handleToolsList(req *mcp.Request) *mcp.Response {
	result := mcp.ToolsListResult{Tools: h.registry.ListTools()}
	return &mcp.Response{JSONRPC: "2.0", ID: req.ID, Result: result}
}

func (h *StdioHandler) handleToolsCall(req *mcp.Request) *mcp.Response {
	var params mcp.ToolCallParams
	paramsBytes, err := json.Marshal(req.Params)
	if err != nil {
		return h.createErrorResponse(req.ID, -32602, fmt.Sprintf("Invalid params: %v", err))
	}
	if err := json.Unmarshal(paramsBytes, &params); err != nil {
		return h.createErrorResponse(req.ID, -32602, fmt.Sprintf("Invalid params: %v", err))
	}

	result, err := h.registry.CallTool(params.Name, params.Arguments)
	if err != nil {
		return &mcp.Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: mcp.ToolCallResult{
				Content: []mcp.ContentBlock{{Type: "text", Text: fmt.Sprintf("Error: %v", err)}},
				IsError: true,
			},
		}
	}

	return &mcp.Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: mcp.ToolCallResult{
			Content: []mcp.ContentBlock{{Type: "text", Text: result}},
		},
	}
}

func (h *StdioHandler) createErrorResponse(id interface{}, code int, message string) *mcp.Response {
	return &mcp.Response{JSONRPC: "2.0", ID: id, Error: &mcp.Error{Code: code, Message: message}}
}

func (h *StdioHandler) sendResponse(resp *mcp.Response) error {
	h.stdoutMutex.Lock()
	defer h.stdoutMutex.Unlock()

	data, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}
	if h.debug {
		log.Printf("[STDOUT] %s", data)
	}
	fmt.Printf("%s\n", data)
	return nil
}

func (h *StdioHandler) sendError(id interface{}, code int, message string) {
	h.sendResponse(h.createErrorResponse(id, code, message)) //nolint:errcheck
}

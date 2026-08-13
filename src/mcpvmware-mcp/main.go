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

	"github.com/cslsoftwares/mcpvmware/cloudaws"
	"github.com/cslsoftwares/mcpvmware/mcp"
	"github.com/cslsoftwares/mcpvmware/tools"
	"github.com/cslsoftwares/mcpvmware/vmware"
	"github.com/cslsoftwares/mcpvmware/workstation"
)

// The 5 connection flags are mutually exclusive — exactly one selects both
// the target endpoint and the ConnectionMode that filters which tools
// register (see tools.ConnectionMode's doc comment and the plan
// "MCPVMWare2026-08-10-175300-plano-cobertura-completa-api-codegen.plan.md"
// §"Modos de conexão"). --workstation-url connects to a local vmrest
// service (Fase 9, --username/--password are Basic Auth creds for that
// service, not vCenter/ESXi ones, in this mode). --cloud-aws-url connects
// to VMware Cloud on AWS (Fase 10) — its "URL" value is conventionally the
// real VMC API host (https://vmc.vmware.com, hardcoded as the client's
// default regardless — VMC has one global endpoint, not a per-customer
// one, so this flag mainly just selects the mode); auth is a CSP
// refresh_token via --refresh-token/VMC_REFRESH_TOKEN, NOT --username/
// --password (those are ignored in this mode — see cloudaws.Client's doc
// comment for why VMC's auth model doesn't fit a username+password shape
// at all). --vmware-all-url including Workstation tools (per its own help
// text below) is a known open question — see the plan's Fase 0 note
// "Registry precisa de 2 clientes vivos" — not resolved by Fase 9;
// --vmware-all-url stays vSphere-only for now.
var (
	vcenterURL       = flag.String("vcenter-url", "", "Connect to a vCenter Server (VCSA). Exposes vCenter-only tools (cluster/DRS, tags, content library, VAMI) plus the general vSphere tools.")
	vmwareURL        = flag.String("vmware-url", "", "Connect directly to a standalone ESXi host (no vCenter). Exposes only the general vSphere tools — nothing vCenter-only, since none of that exists on a bare host.")
	workstationURL   = flag.String("workstation-url", "", "Connect to VMware Workstation Pro's local vmrest service, e.g. http://127.0.0.1:8697/api (https:// if vmrest was started with -c/-k). --username/--password are the vmrest Basic Auth credentials (vmrest.exe --config), not vCenter/ESXi ones.")
	vmwareAllURL     = flag.String("vmware-all-url", "", "Connect to vCenter or ESXi (same SDK URL as --vcenter-url/--vmware-url) and expose every vSphere-family tool, vCenter-only included, regardless of which kind of endpoint this actually is. Does NOT include Workstation Pro tools yet — that needs a 2nd live client, an open architecture question (see the plan's Fase 0 note) — use --workstation-url separately for those.")
	cloudAWSURL      = flag.String("cloud-aws-url", "", "Connect to VMware Cloud on AWS. Selects the mode — the value is conventionally https://vmc.vmware.com (VMC's one global API host); auth is --refresh-token, not --username/--password.")
	username         = flag.String("username", "", "Username for the selected connection (vCenter/ESXi, or vmrest Basic Auth when --workstation-url is used). Not used with --cloud-aws-url.")
	password         = flag.String("password", "", "Password for the selected connection (vCenter/ESXi, or vmrest Basic Auth when --workstation-url is used). Not used with --cloud-aws-url.")
	refreshToken     = flag.String("refresh-token", "", "CSP API refresh token for VMware Cloud on AWS (--cloud-aws-url only) — generate manually in the VMC console web UI (My Account → API Tokens); there is no API to mint one. Also settable via VMC_REFRESH_TOKEN.")
	insecure         = flag.Bool("insecure", false, "Disable TLS certificate verification (UNSAFE: allows man-in-the-middle attacks)")
	versionFlg       = flag.Bool("version", false, "Print version and exit")
	debug            = flag.Bool("debug", false, "Enable debug logging")
	allowDestructive = flag.Bool("allow-destructive", false, "Enable Tier 1/2 destructive tools (VM destroy, snapshot remove/revert, power off/reset/suspend, host maintenance mode, Workstation Pro equivalents, and VMC on AWS SDDC/networking writes). Disabled by default — those tools fail immediately, before touching the backend, regardless of the confirm argument a caller passes.")
	auditLogPath     = flag.String("audit-log-path", "", "Append a JSON Lines audit record (tool, args, timestamp, result) for every destructive tool call to this file. Disabled by default (no path) — this server never writes to disk unless explicitly told to. Example: logs/destructive-actions.jsonl")
)

// Version is the release version, injected at build time via
// -ldflags "-X main.Version=...". Builds without injection report "dev".
var Version = "dev"

// resolveConnectionMode enforces that exactly one of the 5 connection
// flags was given and returns its URL plus the tools.ConnectionMode it
// maps to. Exits the process (log.Fatal) on 0 or 2+ flags set.
func resolveConnectionMode() (string, tools.ConnectionMode) {
	type candidate struct {
		url  string
		mode tools.ConnectionMode
		flag string
	}
	candidates := []candidate{
		{*vcenterURL, tools.ConnectionModeVCenter, "--vcenter-url"},
		{*vmwareURL, tools.ConnectionModeVMware, "--vmware-url"},
		{*vmwareAllURL, tools.ConnectionModeAll, "--vmware-all-url"},
		{*workstationURL, tools.ConnectionModeWorkstation, "--workstation-url"},
		{*cloudAWSURL, tools.ConnectionModeCloudAWS, "--cloud-aws-url"},
	}

	var set []candidate
	for _, c := range candidates {
		if c.url != "" {
			set = append(set, c)
		}
	}

	switch len(set) {
	case 0:
		return "", ""
	case 1:
		// fall through
	default:
		names := make([]string, len(set))
		for i, c := range set {
			names[i] = c.flag
		}
		log.Fatalf("exactly one connection flag may be given, got %d: %s", len(set), strings.Join(names, ", "))
	}

	chosen := set[0]
	return chosen.url, chosen.mode
}

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
	if !*allowDestructive {
		switch strings.ToLower(os.Getenv("VCENTER_ALLOW_DESTRUCTIVE")) {
		case "", "0", "false", "no", "off":
		default:
			*allowDestructive = true
		}
	}
	if *auditLogPath == "" {
		*auditLogPath = os.Getenv("MCPVMWARE_AUDIT_LOG")
	}
	if *refreshToken == "" {
		*refreshToken = os.Getenv("VMC_REFRESH_TOKEN")
	}

	targetURL, connectionMode := resolveConnectionMode()

	if connectionMode == tools.ConnectionModeCloudAWS {
		if targetURL == "" || *refreshToken == "" {
			log.Fatal("--cloud-aws-url requires --refresh-token (or the VMC_REFRESH_TOKEN env var) — --username/--password are not used in this mode")
		}
	} else if targetURL == "" || *username == "" || *password == "" {
		log.Fatal("exactly one connection flag is required (--vcenter-url, --vmware-url, --vmware-all-url, --workstation-url, or --cloud-aws-url), along with --username and --password (or the equivalent VCENTER_URL/VCENTER_USERNAME/VCENTER_PASSWORD env vars) — --cloud-aws-url uses --refresh-token instead")
	}
	if *insecure {
		log.Println("WARNING: TLS certificate verification disabled - the connection is vulnerable to man-in-the-middle attacks")
	}
	if *allowDestructive {
		log.Println("WARNING: destructive tools enabled (--allow-destructive) - VM destroy, snapshot remove/revert, power off/reset/suspend, host maintenance mode, Workstation Pro equivalents, and VMC on AWS SDDC/networking writes will execute when called with confirm:true")
	}
	log.Printf("Connection mode: %s", connectionMode)

	ctx := context.Background()

	// Workstation Pro (Fase 9) and VMC on AWS (Fase 10) each have their own
	// client — plain HTTP+Basic Auth for vmrest, CSP token-exchange for VMC,
	// neither with a govmomi/vim25 session. Every other mode keeps using
	// vmware.Client exactly as before Fase 9; client stays nil in
	// Workstation/CloudAWS mode (safe — see NewRegistry's doc comment).
	var client *vmware.Client
	var wsClient *workstation.Client
	var cloudClient *cloudaws.Client
	switch connectionMode {
	case tools.ConnectionModeWorkstation:
		wc, err := workstation.NewClient(workstation.Config{
			URL:      targetURL,
			Username: *username,
			Password: *password,
			Insecure: *insecure,
		})
		if err != nil {
			log.Fatalf("Failed to configure VMware Workstation Pro (vmrest) connection: %v", err)
		}
		wsClient = wc
	case tools.ConnectionModeCloudAWS:
		cc, err := cloudaws.NewClient(cloudaws.Config{
			RefreshToken: *refreshToken,
			Insecure:     *insecure,
		})
		if err != nil {
			log.Fatalf("Failed to configure VMware Cloud on AWS connection: %v", err)
		}
		cloudClient = cc
	default:
		c, err := vmware.NewClient(ctx, vmware.Config{
			URL:      targetURL,
			Username: *username,
			Password: *password,
			Insecure: *insecure,
		})
		if err != nil {
			log.Fatalf("Failed to connect to vCenter/ESXi: %v", err)
		}
		defer c.Close(ctx)
		client = c
	}

	registry := tools.NewRegistry(ctx, client, tools.RegistryOptions{
		AllowDestructive:  *allowDestructive,
		AuditLogPath:      *auditLogPath,
		ConnectionMode:    connectionMode,
		WorkstationClient: wsClient,
		CloudAWSClient:    cloudClient,
	})

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

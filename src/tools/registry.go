// Package tools registers MCP tools backed by a vmware.Client, one file per
// VMware API namespace (see tools/system.go for the seed tools) — mirroring
// the registration pattern of the reference truenas-mcp project.
package tools

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/cslsoftwares/mcpvmware/cloudaws"
	"github.com/cslsoftwares/mcpvmware/mcp"
	"github.com/cslsoftwares/mcpvmware/vmware"
	"github.com/cslsoftwares/mcpvmware/workstation"
)

// Registry holds the connected VMware client and every registered tool.
type Registry struct {
	client      *vmware.Client
	wsClient    *workstation.Client // nil unless ConnectionMode is Workstation (or All, once dual-client "all" mode lands — see Fase 9 plan note) — Tool.WSHandler tools dispatch on this instead of client
	cloudClient *cloudaws.Client    // nil unless ConnectionMode is CloudAWS — Tool.CloudHandler tools dispatch on this instead of client/wsClient
	ctx         context.Context
	tools       map[string]Tool

	// allowDestructive/auditLogPath back the Fase 1a protection layers for
	// Tier 1/2 tools — see destructive.go. Both default closed/off; a
	// destructive tool call fails before touching client unless the
	// operator opted in via RegistryOptions (main.go wires these to
	// --allow-destructive/--audit-log-path).
	allowDestructive bool
	auditLogPath     string
	auditMu          sync.Mutex

	// connectionMode gates which toolMode classes actually register — see
	// ConnectionMode's doc comment. currentClass is set by withClass for
	// the duration of one registerXTools call; register/registerDestructive
	// read it to decide whether to add the tool at all.
	connectionMode ConnectionMode
	currentClass   toolMode
}

// RegistryOptions configures the cross-cutting protections every tool file
// can opt into — currently the Fase 1a destructive-action gate/audit, plus
// the connection-mode tool filter added 10/08/2026.
type RegistryOptions struct {
	// AllowDestructive gates every tool registered via registerDestructive.
	// False (the default/zero value) makes those tools fail immediately —
	// before any round trip to vCenter/ESXi — regardless of the confirm
	// argument a caller passes.
	AllowDestructive bool
	// AuditLogPath, if non-empty, appends one JSON Lines record per
	// destructive tool call (allowed or denied) to this file. Empty (the
	// default) disables audit logging entirely — matches this project's
	// posture of never writing to disk unless the operator explicitly asked
	// (see vmware/client.go's keepalive-in-memory vs session/cache.Session
	// note from Fase 0).
	AuditLogPath string
	// ConnectionMode restricts ListTools/CallTool to the tool classes valid
	// for that product/connection — see ConnectionMode's doc comment. The
	// zero value ("") is unrestricted (every class registers); every test
	// in this package that builds RegistryOptions{} without setting this
	// relies on that default.
	ConnectionMode ConnectionMode
	// WorkstationClient backs every tool registered via registerWorkstation/
	// registerDestructiveWorkstation (Fase 9, modeWorkstation) — nil unless
	// the operator connected with --workstation-url. Adding this as an
	// optional field (rather than a new NewRegistry parameter) keeps every
	// existing call site — main.go's vCenter/ESXi path and every _test.go
	// file in this package — unchanged; RegistryOptions{} still means "no
	// Workstation client," exactly as before Fase 9.
	WorkstationClient *workstation.Client
	// CloudAWSClient backs every tool registered via registerCloudAWS/
	// registerDestructiveCloudAWS (Fase 10, modeCloudAWS) — nil unless the
	// operator connected with --cloud-aws-url. Same optional-field pattern
	// as WorkstationClient, for the same reason.
	CloudAWSClient *cloudaws.Client
}

// ConnectionMode selects which product's tools a Registry may expose — one
// per mcpvmware-mcp connection flag (--vcenter-url/--vmware-url/
// --workstation-url/--vmware-all-url/--cloud-aws-url — see the plan
// "MCPVMWare2026-08-10-175300-plano-cobertura-completa-api-codegen.plan.md"
// §"Modos de conexão"). Filtering is strict: a tool whose class isn't
// allowed under the active mode is never added to the registry at all — it
// does not appear in ListTools, and CallTool sees it as an unknown tool,
// not a tool that would fail if called.
type ConnectionMode string

const (
	// ConnectionModeVCenter connects to a vCenter Server — allows
	// vcenter-only tools (cluster/DRS/HA, tags, content library, VAMI —
	// once Fases 2-8 of the plan above generate them) plus the general
	// vSphere tools (VM/host/datastore/inventory).
	ConnectionModeVCenter ConnectionMode = "vcenter"
	// ConnectionModeVMware connects directly to a standalone ESXi host —
	// allows only the general vSphere tools; excludes anything vCenter-only
	// (none of that exists on a bare host).
	ConnectionModeVMware ConnectionMode = "vmware"
	// ConnectionModeWorkstation connects to VMware Workstation Pro's
	// vmrest service — allows only workstation-class tools (Fase 9 of the
	// plan, 28 tools).
	ConnectionModeWorkstation ConnectionMode = "workstation"
	// ConnectionModeAll runs a vSphere backend (vCenter or ESXi) and
	// exposes every vSphere-family tool, vcenter-only included — same
	// tool set as ConnectionModeVCenter. Deliberately excludes both
	// workstation and cloud-aws: workstation would need main.go to hold 2
	// live clients simultaneously (vSphere's SDK URL + vmrest's local
	// URL/creds), an open architecture question flagged since the Fase 0
	// plan note ("Registry precisa de 2 clientes vivos") and NOT resolved
	// by Fase 9 — --workstation-url is the only way to reach the 28
	// Workstation tools today; cloud-aws always needs its own flag
	// (incompatible auth model), per the plan.
	ConnectionModeAll ConnectionMode = "all"
	// ConnectionModeCloudAWS connects to VMware Cloud on AWS
	// (console.cloud.vmware.com, via CSP token-exchange auth — see
	// cloudaws.Client) — allows only cloud-aws tools (Fase 10 of the plan,
	// 95 tools).
	ConnectionModeCloudAWS ConnectionMode = "cloud-aws"
	// ConnectionModeEverything (--all-url, 2026-08-20) registers ALL 1030
	// tools of every product at once — the "100% mode". It resolves the open
	// architecture question ConnectionModeAll deferred: main.go holds a live
	// vSphere client (the --all-url endpoint, primary) and OPTIONALLY a
	// Workstation and/or VMC client too, wired only when their flags are also
	// given as secondary connections. Tools whose backing client was not
	// connected still list, but return a clear "requires --X" error at call
	// time (CallTool already guards nil wsClient/cloudClient) — so a plain
	// `--all-url <vcenter>` exposes all 1030 with the 907 vSphere ones live.
	ConnectionModeEverything ConnectionMode = "everything"
)

// toolMode classifies which product a tool belongs to. Assigned per
// registerXTools call via withClass, not per individual r.register call —
// every tool file today registers a single class (see registerTools). If a
// future generated file needs to mix classes within one file, add an
// explicit override param to register/registerDestructive then; no need to
// design for that ahead of time.
type toolMode string

const (
	modeVCenterOnly    toolMode = "vcenter-only"
	modeVSphereGeneral toolMode = "vsphere-general"
	modeWorkstation    toolMode = "workstation"
	modeCloudAWS       toolMode = "cloud-aws"
)

// connectionModeAllows reports whether class may register under cm. The
// zero ConnectionMode ("") allows everything — see RegistryOptions.
func connectionModeAllows(cm ConnectionMode, class toolMode) bool {
	switch cm {
	case "":
		return true
	case ConnectionModeVCenter:
		return class == modeVCenterOnly || class == modeVSphereGeneral
	case ConnectionModeVMware:
		return class == modeVSphereGeneral
	case ConnectionModeWorkstation:
		return class == modeWorkstation
	case ConnectionModeAll:
		return class == modeVCenterOnly || class == modeVSphereGeneral
	case ConnectionModeCloudAWS:
		return class == modeCloudAWS
	case ConnectionModeEverything:
		return true
	default:
		return false
	}
}

// Tool pairs an MCP tool definition with its handler. Exactly one of
// Handler/WSHandler/CloudHandler is set per tool — CallTool dispatches on
// whichever is non-nil. WSHandler (Fase 9, VMware Workstation Pro's
// vmrest) and CloudHandler (Fase 10, VMware Cloud on AWS) were each added
// as their own field rather than widening client to an interface{}: every
// existing vSphere/VAMI handler (611 tools as of Fase 8) keeps its
// concrete *vmware.Client parameter unchanged, no type assertions added to
// code that already works.
type Tool struct {
	Definition   mcp.Tool
	Handler      func(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error)
	WSHandler    func(ctx context.Context, client *workstation.Client, args map[string]interface{}) (string, error)
	CloudHandler func(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error)
}

// NewRegistry builds a Registry bound to client and registers every tool.
// ctx is the long-lived context used for the underlying govmomi calls; the
// stdio handler passes context.Background() today (no per-request
// cancellation yet — see cmd/mcpvmware-mcp/main.go). client may be nil when
// opts.ConnectionMode is Workstation-only or CloudAWS-only (main.go has no
// vCenter/ESXi session to hand over in those modes) — safe because
// connectionModeAllows already excludes every vsphere-general/
// vcenter-only tool from registering at all in those modes, so no Handler
// ever runs against a nil client.
func NewRegistry(ctx context.Context, client *vmware.Client, opts RegistryOptions) *Registry {
	r := &Registry{
		client:           client,
		wsClient:         opts.WorkstationClient,
		cloudClient:      opts.CloudAWSClient,
		ctx:              ctx,
		tools:            make(map[string]Tool),
		allowDestructive: opts.AllowDestructive,
		auditLogPath:     opts.AuditLogPath,
		connectionMode:   opts.ConnectionMode,
	}
	r.registerTools()
	return r
}

// registerTools calls every domain's registerXTools under the toolMode it
// belongs to. Each call registers a single class today — see toolMode's
// doc comment for what to do if a future file needs to mix classes.
func (r *Registry) registerTools() {
	r.withClass(modeVSphereGeneral, registerSystemTools)
	r.withClass(modeVSphereGeneral, registerInventoryTools)
	r.withClass(modeVSphereGeneral, registerVMTools)
	r.withClass(modeVSphereGeneral, registerHostTools)
	r.withClass(modeVCenterOnly, registerApplianceTools)
	r.withClass(modeVSphereGeneral, registerDatastoreTools)
	r.withClass(modeVSphereGeneral, registerOptionTools)
	r.withClass(modeVSphereGeneral, registerVMSnapshotTools)
	r.withClass(modeVSphereGeneral, registerVMLifecycleTools)
	r.withClass(modeVSphereGeneral, registerVMDeviceTools)
	r.withClass(modeVSphereGeneral, registerVMProvisioningTools)
	r.withClass(modeVCenterOnly, registerVMProvisioningVCenterOnlyTools)
	r.withClass(modeVSphereGeneral, registerHostStorageTools)
	r.withClass(modeVSphereGeneral, registerHostIscsiTools)
	r.withClass(modeVSphereGeneral, registerHostIscsiPortBindingTools)
	r.withClass(modeVSphereGeneral, registerHostNetworkTools)
	r.withClass(modeVSphereGeneral, registerHostSecurityTools)
	r.withClass(modeVSphereGeneral, registerHostMiscTools)
	r.withClass(modeVSphereGeneral, registerDatastoreBrowserTools)
	r.withClass(modeVSphereGeneral, registerDatastoreFileManagerTools)
	r.withClass(modeVSphereGeneral, registerStorageDrsTools)
	r.withClass(modeVSphereGeneral, registerVirtualDiskTools)
	r.withClass(modeVCenterOnly, registerNetworkTools)
	r.withClass(modeVSphereGeneral, registerNetworkVSphereGeneralTools)
	r.withClass(modeVSphereGeneral, registerInventoryFolderTools)
	r.withClass(modeVSphereGeneral, registerInventoryComputeTools)
	r.withClass(modeVCenterOnly, registerInventoryComputeVCenterOnlyTools)
	r.withClass(modeVSphereGeneral, registerResourcePoolVAppTools)
	r.withClass(modeVSphereGeneral, registerTaskTools)
	r.withClass(modeVSphereGeneral, registerDiagnosticTools)
	r.withClass(modeVSphereGeneral, registerSearchIndexTools)
	r.withClass(modeVSphereGeneral, registerAuthorizationTools)
	r.withClass(modeVCenterOnly, registerCustomFieldsTools)
	r.withClass(modeVCenterOnly, registerExtensionTools)
	r.withClass(modeVCenterOnly, registerCustomizationSpecTools)
	r.withClass(modeVCenterOnly, registerTenantTools)
	r.withClass(modeVCenterOnly, registerLibraryCoreTools)
	r.withClass(modeVCenterOnly, registerLibrarySessionsTools)
	r.withClass(modeVCenterOnly, registerLibraryMiscTools)
	r.withClass(modeVCenterOnly, registerTagsTools)
	r.withClass(modeVCenterOnly, registerVCenterTemplateTools)
	r.withClass(modeVCenterOnly, registerNamespaceCoreTools)
	r.withClass(modeVCenterOnly, registerNamespaceServicesTools)
	r.withClass(modeVCenterOnly, registerEsxSettingsClusterVMsTools)
	r.withClass(modeVCenterOnly, registerClusterModulesTools)
	r.withClass(modeVCenterOnly, registerCryptoTools)
	r.withClass(modeVCenterOnly, registerCisTasksTools)
	r.withClass(modeVCenterOnly, registerVMDatasetTools)
	r.withClass(modeVCenterOnly, registerApplianceSmallTools)
	r.withClass(modeVCenterOnly, registerAuthenticationTools)
	r.withClass(modeVCenterOnly, registerVAMIRecoveryUpdateTools)
	r.withClass(modeVCenterOnly, registerVAMINetworkSystemTools)
	r.withClass(modeVCenterOnly, registerVAMITechpreviewNetworkTools)
	r.withClass(modeVCenterOnly, registerVAMIServicesAccountsVmonTools)
	r.withClass(modeVCenterOnly, registerVAMIAccessShutdownTools)
	r.withClass(modeWorkstation, registerWorkstationVMTools)
	r.withClass(modeWorkstation, registerWorkstationPowerTools)
	r.withClass(modeWorkstation, registerWorkstationSharedFolderTools)
	r.withClass(modeWorkstation, registerWorkstationNetworkAdapterTools)
	r.withClass(modeWorkstation, registerWorkstationHostNetworkTools)
	r.withClass(modeCloudAWS, registerCloudAWSOrgsTools)
	r.withClass(modeCloudAWS, registerCloudAWSSDDCsTools)
	r.withClass(modeCloudAWS, registerCloudAWSNetworkingCoreTools)
	r.withClass(modeCloudAWS, registerCloudAWSNetworkingEdgeTools)

	// Onda 2 (2026-08-19): managed objects sem wrapper object.* — raw vim25
	// methods (PerformanceManager, LicenseManager, AlarmManager) + a leitura
	// de port groups que faltava para o update_port_group ser seguro.
	r.withClass(modeVSphereGeneral, registerPerformanceTools)
	r.withClass(modeVSphereGeneral, registerLicenseTools)
	r.withClass(modeVCenterOnly, registerAlarmTools)
	r.withClass(modeVSphereGeneral, registerHostNetworkQueryTools)
	// Fault Tolerance (2026-08-19, a pedido do owner) — VirtualMachine FT
	// methods (CreateSecondary/TurnOnFT/etc.), sem wrapper object.*, vcenter-only.
	r.withClass(modeVCenterOnly, registerVMFaultToleranceTools)
	// Onda 3 (2026-08-19): Guest Operations — executar programas + gerenciar
	// arquivos no SO convidado via VMware Tools; sem wrapper object.*, raw vim25.
	r.withClass(modeVSphereGeneral, registerGuestOpsTools)
	// Onda 4 (2026-08-19): Crypto/KMS SOAP (CryptoManagerKmip) — KMIP servers,
	// KMS clusters, chaves de encriptacao de VM; sem wrapper object.*, vcenter-only.
	r.withClass(modeVCenterOnly, registerCryptoKmipTools)
	// Onda 4b (2026-08-19): Distributed Virtual Switch (DVS + DVSManager) —
	// métodos sem wrapper object.* além dos 6 já em generated_network.go; vcenter-only.
	r.withClass(modeVCenterOnly, registerDvsTools)
	// Onda 5 (2026-08-19): HostSystem restantes (power/lockdown/standby) +
	// ClusterComputeResource (recomendações DRS/HA, HCI). Raw methods.*.
	r.withClass(modeVSphereGeneral, registerHostSystemExtTools)
	r.withClass(modeVCenterOnly, registerClusterTools)
	// Onda 6 (2026-08-19): HealthUpdateManager + IpPoolManager + HostProfileManager.
	r.withClass(modeVCenterOnly, registerHealthIpPoolTools)
	r.withClass(modeVCenterOnly, registerHostProfileTools)
	// Onda 7 (2026-08-20): First-Class Disks / vStorageObject — VcenterVStorageObjectManager
	// (vcenter) e HostVStorageObjectManager (host); mesmo campo ServiceContent, tipo por conexão.
	r.withClass(modeVCenterOnly, registerFcdVcenterTools)
	r.withClass(modeVSphereGeneral, registerFcdHostTools)
	// Onda 8 (2026-08-20): vSAN (HostVsanSystem + HostVsanInternalSystem) +
	// Guest Registry/Alias (registro Windows + aliases SSO no SO convidado).
	r.withClass(modeVSphereGeneral, registerVsanTools)
	r.withClass(modeVSphereGeneral, registerGuestWindowsRegistryTools)
	r.withClass(modeVSphereGeneral, registerGuestAliasTools)
	// Onda 9 (2026-08-20): storage avançado (vFlash host + IoFilter vCenter) +
	// inventário/operações (Datacenter/Event geral + ScheduledTask vCenter).
	r.withClass(modeVSphereGeneral, registerHostVFlashTools)
	r.withClass(modeVCenterOnly, registerIoFilterTools)
	r.withClass(modeVSphereGeneral, registerInventoryOpsTools)
	r.withClass(modeVCenterOnly, registerInventoryOpsVCenterOnlyTools)
}

// withClass runs fn with r.currentClass set to class — register/
// registerDestructive read it to decide whether the active ConnectionMode
// allows the tool at all.
func (r *Registry) withClass(class toolMode, fn func(*Registry)) {
	r.currentClass = class
	fn(r)
}

// register adds a tool to the registry, unless the active ConnectionMode
// excludes r.currentClass — see connectionModeAllows. Tool files call this
// from an init-style registerXTools(r *Registry) function invoked by
// registerTools via withClass.
func (r *Registry) register(name, description string, schema map[string]interface{}, handler Tool) {
	if !connectionModeAllows(r.connectionMode, r.currentClass) {
		return
	}
	r.tools[name] = Tool{
		Definition: mcp.Tool{Name: name, Description: description, InputSchema: schema},
		Handler:    handler.Handler,
	}
}

// registerWorkstation is register's counterpart for Fase 9's Workstation
// Pro tools — same gating, but stores a WSHandler (dispatched against
// r.wsClient, not r.client — see Tool's doc comment).
func (r *Registry) registerWorkstation(name, description string, schema map[string]interface{}, handler Tool) {
	if !connectionModeAllows(r.connectionMode, r.currentClass) {
		return
	}
	r.tools[name] = Tool{
		Definition: mcp.Tool{Name: name, Description: description, InputSchema: schema},
		WSHandler:  handler.WSHandler,
	}
}

// registerCloudAWS is register's counterpart for Fase 10's VMware Cloud on
// AWS tools — same gating, but stores a CloudHandler (dispatched against
// r.cloudClient, not r.client — see Tool's doc comment).
func (r *Registry) registerCloudAWS(name, description string, schema map[string]interface{}, handler Tool) {
	if !connectionModeAllows(r.connectionMode, r.currentClass) {
		return
	}
	r.tools[name] = Tool{
		Definition:   mcp.Tool{Name: name, Description: description, InputSchema: schema},
		CloudHandler: handler.CloudHandler,
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
//
// Recovers a panic from the handler into a normal error instead of letting
// it unwind past CallTool — added 2026-08-10 during Fase 2 codegen after
// finding a real crash case (not hypothetical): object.NewVmProvisioningChecker
// / NewVmCompatibilityChecker dereference client.ServiceContent.VmProvisioningChecker
// / VmCompatibilityChecker, both nil on standalone ESXi (confirmed in
// referencia/govmomi/simulator/esx/service_content.go). Those 2 handlers
// also carry an explicit nil-check (defense in depth, and a clearer error
// message than a recovered panic gives), but with ~500 more generated
// candidate methods still to review across Fases 2-8, a single caller
// mistake or an un-audited vCenter-only assumption must not be able to take
// down the whole stdio server for every other in-flight/future tool call.
func (r *Registry) CallTool(name string, args map[string]interface{}) (result string, err error) {
	t, ok := r.tools[name]
	if !ok {
		return "", fmt.Errorf("unknown tool: %s", name)
	}
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("tool %s panicked: %v", name, p)
		}
	}()
	if t.WSHandler != nil {
		if r.wsClient == nil {
			return "", fmt.Errorf("tool %s requires a VMware Workstation Pro connection (--workstation-url), but this server did not connect that way", name)
		}
		return t.WSHandler(r.ctx, r.wsClient, args)
	}
	if t.CloudHandler != nil {
		if r.cloudClient == nil {
			return "", fmt.Errorf("tool %s requires a VMware Cloud on AWS connection (--cloud-aws-url), but this server did not connect that way", name)
		}
		return t.CloudHandler(r.ctx, r.cloudClient, args)
	}
	return t.Handler(r.ctx, r.client, args)
}

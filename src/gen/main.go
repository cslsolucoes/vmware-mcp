// Command gen is the Fase 0 tool from the plan
// ".workspace/plans/MCPVMWare2026-08-10-175300-plano-cobertura-completa-api-codegen.plan.md":
// it parses referencia/govmomi's object/ and vapi/*/ packages via go/ast,
// classifies every candidate API method by tier (destructive-action
// severity) and mode (which product it belongs to), and writes a
// reviewable report — CLASSIFICATION_REPORT.md and classification.json in
// this same directory.
//
// It does NOT generate any tools/*.go file. That is Fase 1+ of the plan,
// gated on a human reviewing this report first (see the plan's "Fase 0"
// section — this is deliberate, not a missing feature).
//
// Run from src/: go run ./gen
package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Method describes one candidate API call found in govmomi's source, with
// the classification the generator proposes for it.
type Method struct {
	Package  string `json:"package"`  // "object" or "vapi/tags" etc.
	File     string `json:"file"`     // basename, e.g. "virtual_machine.go"
	Receiver string `json:"receiver"` // e.g. "VirtualMachine"
	Name     string `json:"name"`     // e.g. "PowerOn"
	Params   string `json:"params"`   // rendered param list, ctx excluded
	Returns  string `json:"returns"`  // rendered return types
	Doc      string `json:"doc"`      // first line of the doc comment, if any

	Tier         string `json:"tier"`          // "" | "tier1" | "tier2"
	TierRule     string `json:"tier_rule"`     // which rule matched, for auditability
	Mode         string `json:"mode"`          // "vcenter-only" | "vsphere-general"
	ProposedTool string `json:"proposed_tool"` // vmware_<domain>_<action>
}

// domainPrefix maps a receiver type name to the tool-name domain prefix —
// explicit map, not inferred, so there is no ambiguity to review (see the
// plan's "Convenção de nomenclatura em escala").
var domainPrefix = map[string]string{
	"VirtualMachine":                 "vm",
	"HostSystem":                     "host",
	"Datastore":                      "datastore",
	"DatastoreFileManager":           "datastore_file",
	"FileManager":                    "file",
	"Network":                        "network",
	"OpaqueNetwork":                  "opaque_network",
	"DistributedVirtualSwitch":       "dvs",
	"VmwareDistributedVirtualSwitch": "dvs",
	"DistributedVirtualPortgroup":    "dvpg",
	"ResourcePool":                   "resource_pool",
	"ComputeResource":                "compute_resource",
	"ClusterComputeResource":         "cluster",
	"Folder":                         "folder",
	"Datacenter":                     "datacenter",
	"VirtualApp":                     "vapp",
	"StoragePod":                     "storage_pod",
	"StorageResourceManager":         "storage",
	"VirtualDiskManager":             "virtual_disk",
	"CustomFieldsManager":            "custom_field",
	"CustomizationSpecManager":       "customization_spec",
	"ExtensionManager":               "extension",
	"DiagnosticManager":              "diagnostic",
	"AuthorizationManager":           "authorization",
	"OptionManager":                  "option",
	"TenantManager":                  "tenant",
	"NamespaceManager":               "namespace",
	"SearchIndex":                    "search",
	"Task":                           "task",
	"HostAccountManager":             "host_account",
	"HostCertificateManager":         "host_certificate",
	"HostConfigManager":              "host_config",
	"HostDatastoreSystem":            "host_datastore",
	"HostDateTimeSystem":             "host_datetime",
	"HostFirewallSystem":             "host_firewall",
	"HostNetworkSystem":              "host_network",
	"HostServiceSystem":              "host_service",
	"HostStorageSystem":              "host_storage",
	"HostVirtualNicManager":          "host_nic",
	"HostVsanSystem":                 "host_vsan",
	"HostVsanInternalSystem":         "host_vsan_internal",
	"HostDatastoreBrowser":           "datastore_browser",
	"EnvironmentBrowser":             "environment",
}

// vcenterOnlyFiles are object/ source files whose methods only make sense
// against a vCenter Server, never a standalone ESXi host — confirmed by
// what the concept itself requires (clusters/DRS need vCenter to exist at
// all), not guessed.
//
// distributed_virtual_switch.go / vmware_distributed_virtual_switch.go /
// distributed_virtual_portgroup.go: a Distributed vSwitch spans multiple
// hosts and is created/managed only through vCenter — a standalone ESXi
// connection cannot create or reconfigure one. storage_pod.go: Storage DRS
// pods (datastore clusters) are likewise a vCenter-only inventory concept
// (StoragePod itself has 0 ctx-taking methods, so this entry is currently
// inert, kept for documentation). Added 2026-08-10 during Fase 0's human
// review — the first version of this map only had the 3 entries found by
// grepping for "cluster"/"tenant"/"namespace"; DVS and Storage DRS were
// missed because their file names don't contain an obviously
// vCenter-flavored word.
//
// storage_resource_manager.go and namespace_manager.go were REMOVED from
// this map on 2026-08-11 (during Fase 4's pre-generation review) — both
// were wrong. Checked each type's constructor
// (object.NewStorageResourceManager / object.NewDatastoreNamespaceManager,
// both dereference a ServiceContent.<Field> ManagedObjectReference the
// same way object.NewVmProvisioningChecker did — see Fase 2's real
// nil-pointer-panic finding) against
// referencia/govmomi/simulator/esx/service_content.go and vpx/service_content.go:
// StorageResourceManager and DatastoreNamespaceManager are BOTH non-nil in
// BOTH templates — the managers exist on standalone ESXi too. (The original
// "vCenter-only" guess for storage_resource_manager.go conflated "Storage
// DRS recommendations only ever get generated by a vCenter-managed
// datastore cluster" with "the API endpoint only exists on vCenter" — those
// are different claims; only the first is true, and it surfaces as a normal
// runtime fault, not a missing/nil ServiceContent field. The
// namespace_manager.go guess was a plain misread: DatastoreNamespaceManager
// is about datastore-level directory/vSAN-File-Service namespaces, not the
// vapi/namespace Kubernetes-Supervisor concept the filename suggested.)
//
// extension_manager.go, custom_fields_manager.go, and
// customization_spec_manager.go were ADDED to this map on 2026-08-12
// (during Fase 7's pre-generation review) — the fail-safe default had
// wrongly left all three as vsphere-general. Checked ServiceContent
// directly: ExtensionManager/CustomFieldsManager/CustomizationSpecManager
// are all `(*types.ManagedObjectReference)(nil)` in
// referencia/govmomi/simulator/esx/service_content.go but populated in
// vpx/service_content.go — same nil-pointer-panic risk pattern as Fase 2's
// VmProvisioningChecker finding if left reachable against standalone ESXi.
var vcenterOnlyFiles = map[string]bool{
	"cluster_compute_resource.go":          true,
	"tenant_manager.go":                    true,
	"distributed_virtual_switch.go":        true,
	"vmware_distributed_virtual_switch.go": true,
	"distributed_virtual_portgroup.go":     true,
	"storage_pod.go":                       true,
	"extension_manager.go":                 true,
	"custom_fields_manager.go":             true,
	"customization_spec_manager.go":        true,
}

// existingHandWrittenTools are the tool names already implemented by hand
// in tools/*.go (verified 2026-08-10 by grepping the actual registered
// name string literals, not by re-typing a remembered list — see
// .wolf/cerebrum.md Do-Not-Repeat for why that distinction matters).
// Methods whose ProposedTool collides with one of these are excluded from
// the report: they're already implemented, so surfacing them again would
// invite Fase 1+ to regenerate/duplicate a symbol that exists. Exclusion
// is announced on stdout (count + names), never silent — see skipMethods.
var existingHandWrittenTools = map[string]bool{
	"vmware_about":                   true,
	"vmware_appliance_health":        true,
	"vmware_appliance_health_detail": true,
	"vmware_appliance_uptime":        true,
	"vmware_appliance_version":       true,
	"vmware_datastore_upload_file":   true,
	"vmware_host_info":               true,
	"vmware_host_maintenance_enter":  true,
	"vmware_host_maintenance_exit":   true,
	"vmware_host_management_ips":     true,
	"vmware_host_reconnect":          true,
	"vmware_list_clusters":           true,
	"vmware_list_datacenters":        true,
	"vmware_list_datastores":         true,
	"vmware_list_hosts":              true,
	"vmware_list_networks":           true,
	"vmware_list_resource_pools":     true,
	"vmware_list_vms":                true,
	"vmware_vm_destroy":              true,
	"vmware_vm_info":                 true,
	"vmware_vm_power_off":            true,
	"vmware_vm_power_on":             true,
	"vmware_vm_reconfigure":          true,
	"vmware_vm_reset":                true,
	"vmware_vm_snapshot_create":      true,
	"vmware_vm_snapshot_list":        true,
	"vmware_vm_snapshot_remove":      true,
	"vmware_vm_snapshot_revert":      true,
	"vmware_vm_suspend":              true,
}

// tier1Patterns / tier2Patterns match method NAMES (not doc text) — see the
// plan's Fase 0 severity rules. Order matters: tier1 checked first.
var tier1Patterns = regexp.MustCompile(`(?i)^(Destroy|Delete|Remove|Format|Unmount|Detach|Terminate|Erase|Wipe|Revert|Uninstall|Unregister|Disconnect)`)
var tier2Patterns = regexp.MustCompile(`(?i)^(Reset|PowerOff|Shutdown|Reboot|Reload|Restart|Unmap|Release|Clear|Purge|Kill|Stop|Disable|Evacuate)`)
var readOnlyPatterns = regexp.MustCompile(`(?i)^(Get|List|Query|Find|Search|Retrieve|Is|Has|Wait|Fetch)`)

// tier1VerbAnywhere catches a tier1 verb that appears mid-name at a
// CamelCase word boundary (preceded by start-of-string or a lowercase
// letter) instead of only as a prefix — e.g. "ForceDeleteLibrary" or
// "KmsProviderDelete" don't match tier1Patterns' ^-anchor but are just as
// destructive as "DeleteLibrary". Added 2026-08-10 after Fase 0's human
// review found exactly these 2 cases across all 536 methods (verified by
// scripted scan, not guessed — see the plan's Fase 0 review section).
var tier1VerbAnywhere = regexp.MustCompile(`(?:^|[a-z0-9])(Destroy|Delete|Remove|Format|Unmount|Detach|Terminate|Erase|Wipe|Revert|Uninstall|Unregister|Disconnect)`)

// skipMethods are plumbing that happens to take a ctx but isn't a
// meaningful standalone action — kept intentionally small (see doc
// comment: over-inclusion in the report is fine, a human prunes; silent
// exclusion is the risk to avoid).
var skipMethods = map[string]bool{
	"String":    true,
	"Reference": true,
	"Pxe":       true,
}

func main() {
	root := "../referencia/govmomi"
	if len(os.Args) > 1 {
		root = os.Args[1]
	}

	var methods []Method

	// object/ — single flat package.
	objDir := filepath.Join(root, "object")
	methods = append(methods, parsePackageDir(objDir, "object", false)...)

	// vapi/... — NOT flat: some subpackages nest further (e.g.
	// vapi/appliance/{access,logging,networking,shutdown},
	// vapi/cis/tasks, vapi/esx/settings — appliance itself has zero .go
	// files directly in it, everything lives one level deeper). Walk
	// recursively and treat every directory containing non-test .go files
	// as its own flat package, skipping internal/simulator anywhere in the
	// path (not real API surface). A first version of this generator only
	// read one level and silently missed vapi/appliance entirely — fixed
	// after noticing the report had no vapi/appliance section at all.
	vapiRoot := filepath.Join(root, "vapi")
	for _, dir := range findGoPackageDirs(vapiRoot) {
		rel, err := filepath.Rel(root, dir)
		if err != nil {
			rel = dir
		}
		label := filepath.ToSlash(rel)
		methods = append(methods, parsePackageDir(dir, label, true)...)
	}

	for i := range methods {
		classify(&methods[i])
	}

	// Drop methods that collide with an already hand-written tool — Fase 1+
	// would otherwise try to regenerate/duplicate a symbol that already
	// exists. Announced, not silent (see existingHandWrittenTools doc).
	var kept []Method
	var skipped []string
	for _, m := range methods {
		if existingHandWrittenTools[m.ProposedTool] {
			skipped = append(skipped, fmt.Sprintf("%s (%s.%s)", m.ProposedTool, m.Receiver, m.Name))
			continue
		}
		kept = append(kept, m)
	}
	methods = kept
	if len(skipped) > 0 {
		sort.Strings(skipped)
		fmt.Printf("Excluded %d methods already covered by hand-written tools:\n", len(skipped))
		for _, s := range skipped {
			fmt.Printf("  - %s\n", s)
		}
	}

	// Safety net: the curated domainPrefix map is meant to make every
	// ProposedTool unique, but if two receivers ever share a prefix and a
	// method name, report it loudly instead of silently keeping a
	// duplicate tool name in the reviewable output.
	seen := map[string][]string{}
	for _, m := range methods {
		seen[m.ProposedTool] = append(seen[m.ProposedTool], m.Package+" "+m.Receiver+"."+m.Name)
	}
	for name, origins := range seen {
		if len(origins) > 1 {
			fmt.Printf("WARNING: proposed_tool collision %q: %s\n", name, strings.Join(origins, " vs "))
		}
	}

	sort.Slice(methods, func(i, j int) bool {
		if methods[i].Package != methods[j].Package {
			return methods[i].Package < methods[j].Package
		}
		if methods[i].Receiver != methods[j].Receiver {
			return methods[i].Receiver < methods[j].Receiver
		}
		return methods[i].Name < methods[j].Name
	})

	writeJSON(methods, "gen/classification.json")
	writeMarkdown(methods, "gen/CLASSIFICATION_REPORT.md")

	tier1, tier2, untiered := 0, 0, 0
	for _, m := range methods {
		switch m.Tier {
		case "tier1":
			tier1++
		case "tier2":
			tier2++
		default:
			untiered++
		}
	}
	fmt.Printf("Parsed %d candidate methods (%d object/, %d vapi/*)\n",
		len(methods), countPkg(methods, "object"), len(methods)-countPkg(methods, "object"))
	fmt.Printf("Tier1=%d Tier2=%d untiered=%d\n", tier1, tier2, untiered)
	fmt.Println("Wrote gen/classification.json and gen/CLASSIFICATION_REPORT.md")
}

func countPkg(methods []Method, pkg string) int {
	n := 0
	for _, m := range methods {
		if m.Package == pkg {
			n++
		}
	}
	return n
}

// findGoPackageDirs walks root recursively and returns every directory
// that directly contains at least one non-test .go file, skipping any
// directory named "internal" or "simulator" (and everything under it) —
// those aren't real API surface for this report's purpose.
func findGoPackageDirs(root string) []string {
	var dirs []string
	skip := map[string]bool{"internal": true, "simulator": true}

	var walk func(dir string)
	walk = func(dir string) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "skip %s: %v\n", dir, err)
			return
		}
		hasGo := false
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".go") && !strings.HasSuffix(e.Name(), "_test.go") {
				hasGo = true
			}
		}
		if hasGo {
			dirs = append(dirs, dir)
		}
		for _, e := range entries {
			if e.IsDir() && !skip[e.Name()] {
				walk(filepath.Join(dir, e.Name()))
			}
		}
	}
	walk(root)
	sort.Strings(dirs)
	return dirs
}

// parsePackageDir parses every non-test .go file directly in dir (no
// recursion — govmomi's object/ and each vapi/<sub>/ are flat packages) and
// returns every exported method whose first parameter is context.Context.
func parsePackageDir(dir, pkgLabel string, vcenterOnly bool) []Method {
	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "skip %s: %v\n", dir, err)
		return nil
	}

	var out []Method
	fset := token.NewFileSet()

	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}

		path := filepath.Join(dir, name)
		src, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			fmt.Fprintf(os.Stderr, "parse error %s: %v\n", path, err)
			continue
		}

		for _, decl := range src.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || !fn.Name.IsExported() {
				continue
			}
			if skipMethods[fn.Name.Name] {
				continue
			}
			if !firstParamIsContext(fn) {
				continue
			}

			recv := receiverTypeName(fn)
			m := Method{
				Package:  pkgLabel,
				File:     name,
				Receiver: recv,
				Name:     fn.Name.Name,
				Params:   renderParams(fset, fn),
				Returns:  renderReturns(fset, fn),
				Doc:      firstDocLine(fn),
			}
			if vcenterOnly {
				m.Mode = "vcenter-only"
			} else if vcenterOnlyFiles[name] {
				m.Mode = "vcenter-only"
			} else {
				m.Mode = "vsphere-general"
			}
			out = append(out, m)
		}
	}
	return out
}

func firstParamIsContext(fn *ast.FuncDecl) bool {
	if fn.Type.Params == nil || len(fn.Type.Params.List) == 0 {
		return false
	}
	first := fn.Type.Params.List[0]
	sel, ok := first.Type.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkgIdent, ok := sel.X.(*ast.Ident)
	return ok && pkgIdent.Name == "context" && sel.Sel.Name == "Context"
}

func receiverTypeName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return ""
	}
	t := fn.Recv.List[0].Type
	if star, ok := t.(*ast.StarExpr); ok {
		t = star.X
	}
	if ident, ok := t.(*ast.Ident); ok {
		return ident.Name
	}
	return ""
}

func renderParams(fset *token.FileSet, fn *ast.FuncDecl) string {
	if fn.Type.Params == nil {
		return ""
	}
	var parts []string
	for i, field := range fn.Type.Params.List {
		if i == 0 {
			continue // ctx
		}
		typ := exprString(fset, field.Type)
		if len(field.Names) == 0 {
			parts = append(parts, typ)
			continue
		}
		for _, n := range field.Names {
			parts = append(parts, n.Name+" "+typ)
		}
	}
	return strings.Join(parts, ", ")
}

func renderReturns(fset *token.FileSet, fn *ast.FuncDecl) string {
	if fn.Type.Results == nil {
		return ""
	}
	var parts []string
	for _, field := range fn.Type.Results.List {
		parts = append(parts, exprString(fset, field.Type))
	}
	return strings.Join(parts, ", ")
}

func exprString(fset *token.FileSet, e ast.Expr) string {
	var sb strings.Builder
	fset2 := fset
	_ = fset2
	// Minimal renderer: types.X, *object.Y, []string, etc. via ast printer
	// would need go/printer; keep this lightweight instead.
	switch v := e.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.StarExpr:
		return "*" + exprString(fset, v.X)
	case *ast.SelectorExpr:
		return exprString(fset, v.X) + "." + v.Sel.Name
	case *ast.ArrayType:
		return "[]" + exprString(fset, v.Elt)
	case *ast.Ellipsis:
		return "..." + exprString(fset, v.Elt)
	default:
		sb.WriteString(fmt.Sprintf("%T", e))
		return sb.String()
	}
}

func firstDocLine(fn *ast.FuncDecl) string {
	if fn.Doc == nil {
		return ""
	}
	text := fn.Doc.Text()
	if i := strings.IndexByte(text, '\n'); i >= 0 {
		text = text[:i]
	}
	return strings.TrimSpace(text)
}

var camelBoundary = regexp.MustCompile(`([a-z0-9])([A-Z])`)

func toSnake(s string) string {
	s = camelBoundary.ReplaceAllString(s, "${1}_${2}")
	return strings.ToLower(s)
}

func classify(m *Method) {
	switch {
	case tier1Patterns.MatchString(m.Name):
		m.Tier = "tier1"
		m.TierRule = "name matches tier1 pattern"
	case tier2Patterns.MatchString(m.Name):
		m.Tier = "tier2"
		m.TierRule = "name matches tier2 pattern"
	case readOnlyPatterns.MatchString(m.Name):
		m.Tier = ""
		m.TierRule = "name matches read-only pattern"
	case tier1VerbAnywhere.MatchString(m.Name):
		m.Tier = "tier1"
		m.TierRule = "tier1 verb found mid-name, not prefix (e.g. ForceDeleteX)"
	default:
		// Fail-safe default per the plan: anything not clearly read-only
		// and not clearly tier1 is tier2, not untiered.
		m.Tier = "tier2"
		m.TierRule = "fail-safe default (no pattern matched)"
	}

	prefix, ok := domainPrefix[m.Receiver]
	if !ok {
		prefix = toSnake(m.Receiver)
	}
	action := toSnake(m.Name)
	if strings.HasPrefix(m.Package, "vapi/") {
		vapiPath := strings.TrimPrefix(m.Package, "vapi/")
		vapiPath = strings.ReplaceAll(vapiPath, "/", "_") // e.g. appliance/access -> appliance_access
		m.ProposedTool = "vmware_" + vapiPath + "_" + action
	} else {
		m.ProposedTool = "vmware_" + prefix + "_" + action
	}
}

func writeJSON(methods []Method, path string) {
	f, err := os.Create(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to write %s: %v\n", path, err)
		return
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(methods); err != nil {
		fmt.Fprintf(os.Stderr, "failed to encode %s: %v\n", path, err)
	}
}

func writeMarkdown(methods []Method, path string) {
	f, err := os.Create(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to write %s: %v\n", path, err)
		return
	}
	defer f.Close()

	fmt.Fprintln(f, "# Fase 0 — relatório de classificação (gerado, para revisão humana)")
	fmt.Fprintln(f)
	fmt.Fprintf(f, "%d métodos candidatos encontrados. **Nada foi gerado em tools/ a partir disto ainda** — ", len(methods))
	fmt.Fprintln(f, "ver o plano \"MCPVMWare2026-08-10-175300-plano-cobertura-completa-api-codegen.plan.md\" §Fase 0.")
	fmt.Fprintln(f)

	byPkg := map[string][]Method{}
	var pkgs []string
	for _, m := range methods {
		if _, ok := byPkg[m.Package]; !ok {
			pkgs = append(pkgs, m.Package)
		}
		byPkg[m.Package] = append(byPkg[m.Package], m)
	}
	sort.Strings(pkgs)

	for _, pkg := range pkgs {
		list := byPkg[pkg]
		fmt.Fprintf(f, "## %s (%d métodos)\n\n", pkg, len(list))
		fmt.Fprintln(f, "| Receiver | Método | Tier | Modo | Tool proposta | Regra |")
		fmt.Fprintln(f, "|---|---|---|---|---|---|")
		for _, m := range list {
			tier := m.Tier
			if tier == "" {
				tier = "—"
			}
			fmt.Fprintf(f, "| %s | %s(%s) | %s | %s | `%s` | %s |\n",
				m.Receiver, m.Name, m.Params, tier, m.Mode, m.ProposedTool, m.TierRule)
		}
		fmt.Fprintln(f)
	}
}

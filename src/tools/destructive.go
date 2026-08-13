package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/cslsoftwares/mcpvmware/cloudaws"
	"github.com/cslsoftwares/mcpvmware/vmware"
	"github.com/cslsoftwares/mcpvmware/workstation"
)

// tier classifies how much damage a tool can do if called wrongly (decided
// with the user via AskUserQuestion, 2026-08-10 — 3-layer protection: strict
// confirm + server gate + audit log, over a simpler confirm-only option and
// a more complex preview/token 2-step flow). Tier1 tools are irreversible
// (vmware_vm_destroy, snapshot remove/revert); Tier2 tools are disruptive
// but reversible (power off/reset/suspend, host maintenance mode). No tool
// exists in either tier yet — Fase 1a only builds the mechanism; Fases 1/3
// wrap their handlers with it once written.
type tier int

const (
	tier1 tier = 1 // irreversible
	tier2 tier = 2 // disruptive, reversible
)

// destructiveHandler matches Tool.Handler's signature — named here only so
// wrapDestructive's signature reads cleanly.
type destructiveHandler = func(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error)

// auditEntry is one JSON Lines record written to the audit log for every
// Tier 1/2 call, allowed or denied — a denied call (closed gate, missing
// confirm) leaves the same trail as an allowed one, so the log can answer
// "who tried what," not just "what succeeded."
type auditEntry struct {
	Time      string                 `json:"time"`
	Tool      string                 `json:"tool"`
	Tier      tier                   `json:"tier"`
	Args      map[string]interface{} `json:"args"`
	GateOpen  bool                   `json:"gate_open"`
	Confirmed bool                   `json:"confirmed"`
	Allowed   bool                   `json:"allowed"`
	Error     string                 `json:"error,omitempty"`
}

// registerDestructive registers name like r.register, but wraps handler
// with the 3 Fase 1a layers first: the server-level gate (independent of
// what the caller passes), a strict confirm:true check, and audit logging.
func (r *Registry) registerDestructive(name, description string, t tier, schema map[string]interface{}, handler Tool) {
	r.register(name, description, schema, Tool{Handler: r.wrapDestructive(name, t, handler.Handler)})
}

// workstationDestructiveHandler is registerDestructiveWorkstation's
// counterpart to destructiveHandler — same shape, workstation.Client
// instead of vmware.Client (see Tool.WSHandler's doc comment for why this
// is a second type instead of widening the client parameter).
type workstationDestructiveHandler = func(ctx context.Context, client *workstation.Client, args map[string]interface{}) (string, error)

// registerDestructiveWorkstation is registerDestructive's counterpart for
// Fase 9's Workstation Pro tools — same 3-layer protection (gate/confirm/
// audit), dispatched against r.wsClient via Tool.WSHandler.
func (r *Registry) registerDestructiveWorkstation(name, description string, t tier, schema map[string]interface{}, handler Tool) {
	r.registerWorkstation(name, description, schema, Tool{WSHandler: r.wrapDestructiveWorkstation(name, t, handler.WSHandler)})
}

// wrapDestructiveWorkstation mirrors wrapDestructive's gate→confirm→audit
// sequence exactly, for a workstationDestructiveHandler instead of a
// destructiveHandler.
func (r *Registry) wrapDestructiveWorkstation(name string, t tier, handler workstationDestructiveHandler) workstationDestructiveHandler {
	return func(ctx context.Context, client *workstation.Client, args map[string]interface{}) (string, error) {
		gateOpen := r.allowDestructive
		confirmed, _ := args["confirm"].(bool)

		entry := auditEntry{
			Time:      time.Now().UTC().Format(time.RFC3339),
			Tool:      name,
			Tier:      t,
			Args:      args,
			GateOpen:  gateOpen,
			Confirmed: confirmed,
		}

		deny := func(reason string) (string, error) {
			entry.Allowed = false
			entry.Error = reason
			r.writeAudit(entry)
			return "", fmt.Errorf("%s", reason)
		}

		if !gateOpen {
			return deny(fmt.Sprintf(
				"%s is a destructive operation (tier %d) and this server was started without --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE — restart it with that flag/env var to enable destructive tools",
				name, t))
		}
		if !confirmed {
			return deny(fmt.Sprintf("%s requires confirm:true (exact boolean) to run", name))
		}

		result, err := handler(ctx, client, args)
		entry.Allowed = err == nil
		if err != nil {
			entry.Error = err.Error()
		}
		r.writeAudit(entry)
		return result, err
	}
}

// cloudAWSDestructiveHandler is registerDestructiveCloudAWS's counterpart
// to destructiveHandler — same shape, cloudaws.Client instead of
// vmware.Client (see Tool.CloudHandler's doc comment).
type cloudAWSDestructiveHandler = func(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error)

// registerDestructiveCloudAWS is registerDestructive's counterpart for
// Fase 10's VMware Cloud on AWS tools — same 3-layer protection
// (gate/confirm/audit), dispatched against r.cloudClient via
// Tool.CloudHandler. Every write operation under SDDCs is tier1 by
// convention in this fase (real financial cost of an accidental SDDC
// create/delete/resize — see the plan's Fase 10 note), not just deletes as
// elsewhere in this project — callers of this function pick the tier per
// route, this wrapper enforces whatever tier they pass exactly like
// registerDestructive/registerDestructiveWorkstation do.
func (r *Registry) registerDestructiveCloudAWS(name, description string, t tier, schema map[string]interface{}, handler Tool) {
	r.registerCloudAWS(name, description, schema, Tool{CloudHandler: r.wrapDestructiveCloudAWS(name, t, handler.CloudHandler)})
}

// wrapDestructiveCloudAWS mirrors wrapDestructive's gate→confirm→audit
// sequence exactly, for a cloudAWSDestructiveHandler instead of a
// destructiveHandler.
func (r *Registry) wrapDestructiveCloudAWS(name string, t tier, handler cloudAWSDestructiveHandler) cloudAWSDestructiveHandler {
	return func(ctx context.Context, client *cloudaws.Client, args map[string]interface{}) (string, error) {
		gateOpen := r.allowDestructive
		confirmed, _ := args["confirm"].(bool)

		entry := auditEntry{
			Time:      time.Now().UTC().Format(time.RFC3339),
			Tool:      name,
			Tier:      t,
			Args:      args,
			GateOpen:  gateOpen,
			Confirmed: confirmed,
		}

		deny := func(reason string) (string, error) {
			entry.Allowed = false
			entry.Error = reason
			r.writeAudit(entry)
			return "", fmt.Errorf("%s", reason)
		}

		if !gateOpen {
			return deny(fmt.Sprintf(
				"%s is a destructive operation (tier %d) and this server was started without --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE — restart it with that flag/env var to enable destructive tools",
				name, t))
		}
		if !confirmed {
			return deny(fmt.Sprintf("%s requires confirm:true (exact boolean) to run", name))
		}

		result, err := handler(ctx, client, args)
		entry.Allowed = err == nil
		if err != nil {
			entry.Error = err.Error()
		}
		r.writeAudit(entry)
		return result, err
	}
}

// wrapDestructive enforces, in order: (1) the server gate — closed by
// default, fails before any vCenter/ESXi round trip regardless of confirm;
// (2) a strict confirm:true check (exact bool, not a truthy string) — only
// once both pass does the real handler run. Every outcome is audited.
func (r *Registry) wrapDestructive(name string, t tier, handler destructiveHandler) destructiveHandler {
	return func(ctx context.Context, client *vmware.Client, args map[string]interface{}) (string, error) {
		gateOpen := r.allowDestructive
		confirmed, _ := args["confirm"].(bool)

		entry := auditEntry{
			Time:      time.Now().UTC().Format(time.RFC3339),
			Tool:      name,
			Tier:      t,
			Args:      args,
			GateOpen:  gateOpen,
			Confirmed: confirmed,
		}

		deny := func(reason string) (string, error) {
			entry.Allowed = false
			entry.Error = reason
			r.writeAudit(entry)
			return "", fmt.Errorf("%s", reason)
		}

		if !gateOpen {
			return deny(fmt.Sprintf(
				"%s is a destructive operation (tier %d) and this server was started without --allow-destructive/VCENTER_ALLOW_DESTRUCTIVE — restart it with that flag/env var to enable destructive tools",
				name, t))
		}
		if !confirmed {
			return deny(fmt.Sprintf("%s requires confirm:true (exact boolean) to run", name))
		}

		result, err := handler(ctx, client, args)
		entry.Allowed = err == nil
		if err != nil {
			entry.Error = err.Error()
		}
		r.writeAudit(entry)
		return result, err
	}
}

// writeAudit appends one JSON line to r.auditLogPath. A no-op when
// AuditLogPath wasn't set (the default — see RegistryOptions). Failures to
// write the audit record never surface as a tool error: losing an audit
// line must not also block/rollback an action the gate and confirm already
// approved.
func (r *Registry) writeAudit(e auditEntry) {
	if r.auditLogPath == "" {
		return
	}

	line, err := json.Marshal(e)
	if err != nil {
		return
	}

	r.auditMu.Lock()
	defer r.auditMu.Unlock()

	f, err := os.OpenFile(r.auditLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()

	line = append(line, '\n')
	_, _ = f.Write(line)
}

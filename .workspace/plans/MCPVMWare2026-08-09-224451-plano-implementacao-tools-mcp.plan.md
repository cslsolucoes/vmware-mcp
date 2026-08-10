---
title: "MCPVMWare — dos 2 tools seed a um MCP VMware utilizável"
created: 2026-08-09
updated: 2026-08-09
status: draft
locale: pt-PT
overview: "Adicionar VM lifecycle, snapshots, inventário, host ops e VAMI ao servidor MCP, com fundação de sessão/keepalive e harness vcsim."
---

## Resumo

O servidor MCP (`d:\MCPVMWare`) já tem um scaffold Go validado ponta a ponta
contra hardware real (ESXi 7.0.3 standalone, `10.100.2.54`): `go.mod`,
`mcp/types.go`, `vmware/client.go`, `tools/registry.go`+`tools/system.go` com 2
tools seed (`vmware_about`, `vmware_list_vms`), rodando via stdio JSON-RPC em
`cmd/mcpvmware-mcp/main.go`. Falta a superfície real de operações VMware — hoje
o servidor só "vê" o inventário, não age sobre ele. Este plano adiciona VM
lifecycle, snapshots, inventário, operações de host e administração de
Appliance (VAMI), com uma fundação de sessão/keepalive e um harness de testes
com `vcsim` para não depender do host real em toda verificação.

## Objectivos

- [ ] Sessão de longa duração sem expirar silenciosamente (keepalive) antes de
      empilhar tools novas.
- [ ] Harness de testes offline com `src/vcsim` cobrindo os domínios novos.
- [ ] Tools de inventário (host/datastore/rede/resource-pool/cluster/datacenter).
- [ ] Tools de VM lifecycle (power on/off/reset/suspend/reconfigure/destroy) e
      snapshots (create/revert/remove/list).
- [ ] Tools de host ops (maintenance mode, reconnect, IPs de gestão, info).
- [ ] Tools de administração de Appliance/VAMI (version/health/uptime), com a
      limitação de não-vCenter assumida explicitamente.

## Âmbito e exclusões

**Dentro do âmbito** (decisões fechadas com o usuário via AskUserQuestion,
09/08/2026):
1. Escopo de produto: vSphere on-prem (SOAP/`vim25`, já validado) **+**
   administração do Appliance (VAMI).
2. Domínios desta rodada: VM lifecycle, Snapshots, Inventário, Host ops.
3. Sessão de longa duração: resolver agora (fundação), não depois.
4. Testes: ESXi real (`10.100.2.54`) para verificação manual + `vcsim` para
   harness automatizado.

**Fora do âmbito nesta rodada:** VMware Cloud on AWS (produto/control-plane
distinto, endpoint `console.cloud.vmware.com`, sem relação com `govmomi`);
edição de dispositivos de VM além de CPU/RAM em `reconfigure`; as ~130 rotas
completas de VAMI (só um starter slice: version/health/uptime); domínios
storage-policy/VASA (`pbm`/`sms`/`vsan`/`vslm`/`cns`) e identidade
(`sts`/`lookup`/`ssoadmin`) — nicho, só se pedido específico surgir.

**Tensão conhecida, não escondida:** o único alvo real validado
(`10.100.2.54`) é um ESXi **standalone** — não expõe VAMI (exclusivo de vCenter
Server Appliance). O simulador `src/vapi/appliance/simulator` só cobre
`access/logging/networking/shutdown` (confirmado por `ls` real em
`src/vapi/appliance/`), não `health`/`system/version`. Os tools de VAMI
propostos compilam e têm teste unitário (fixture HTTP), mas **não são
verificáveis ponta-a-ponta** até existir uma vCenter Appliance real de teste.

**Achado que simplifica o plano:** não é preciso escrever um cliente REST do
zero para VAMI — `github.com/vmware/govmomi/vapi/rest` (`src/vapi/rest/client.go`,
já vendorizado) implementa o fluxo de sessão CIS (`POST
/rest/com/vmware/cis/session` com Basic → header `vmware-api-session-id`) que
as coleções Postman documentam, incluindo um `Resource(path).Request(method)` +
`Do(ctx, req, &out)` genérico que desembrulha `/rest` e `/api` automaticamente.
Confirmado (pesquisa paralela do usuário em `github.com/vmware`, 09/08/2026)
que não existe alternativa melhor: `vsphere-automation-sdk-go` oficial está em
Beta e não suporta vSphere/vCenter (só VMC/NSX-T) — `vapi/rest` é o único
cliente REST/VAPI Go-nativo viável.

## Inventário de ficheiros / artefactos

| Caminho | Operação | Nota |
|---|---|---|
| `vmware/client.go` | editar | reescrita do keepalive (Fase 0) + accessor `REST()` lazy (Fase 4) — mudança de maior impacto |
| `tools/registry.go` | editar | `registerTools()` ganha 1 linha por domínio novo |
| `tools/vm.go` | criar | VM lifecycle + snapshots (Fase 1) |
| `tools/inventory.go` | criar | listagens de host/datastore/rede/resource-pool/cluster/datacenter (Fase 2) |
| `tools/host.go` | criar | operações de host (Fase 3) |
| `tools/appliance.go` | criar | tools de VAMI (Fase 4) |
| `tools/testhelpers_test.go` | criar | helper `newSimClient` com `src/vcsim` |
| `tools/client_keepalive_test.go` | criar | prova de keepalive sobrevivendo a timeout comprimido |
| `tools/vm_test.go`, `tools/inventory_test.go`, `tools/host_test.go`, `tools/appliance_test.go` | criar | testes por domínio (vcsim, exceto appliance = fixture httptest) |

## Passos / fases

Ordem de construção (motivo em cada fase):

### Fase 0 — Sessão/keepalive (`vmware/client.go`) — primeiro, bloqueante

**Problema real:** `govmomi.NewClient(ctx, u, insecure)` faz login *dentro* da
chamada (`src/client.go:79-84`), cedo demais para plugar um round-tripper de
keepalive. Sem isso, um servidor de longa duração perde a sessão
silenciosamente depois do timeout (30 min por padrão).

**Mudança de forma:** nenhuma mudança na interface pública de `vmware.Client`
— todos os handlers existentes em `tools/system.go` continuam compilando sem
alteração. Adiciona campos privados para o cliente REST (lazy, Fase 4):

```go
type Client struct {
    *govmomi.Client
    Finder *find.Finder
    cfg    Config       // retido para login lazy do REST client (Fase 4)
    rest   *rest.Client // nil até o primeiro tool de appliance
    mu     sync.Mutex
}
```

**Sequência corrigida em `NewClient`:** conectar SEM userinfo na URL → anexar
`keepalive.NewHandlerSOAP(gc.Client.RoundTripper, 10*time.Minute, nil)` →
**depois** `gc.Login(ctx, url.UserPassword(user, pass))` (o keepalive só arma o
ticker ao observar o round-trip de login, `src/session/keepalive/handler.go:157-159`;
padrão exato em `src/session/keepalive/example_test.go:62-67`). `Close`
continua chamando `Logout` — o keepalive para sozinho no round-trip de logout.

**Nota sobre `session/cache.Session`:** voltado a muitas invocações CLI curtas,
grava tickets em `~/.govmomi/sessions` — tensiona com a postura já registrada
em `.wolf/STATUS.md` de nunca persistir credenciais em arquivo. Recomendação:
implementar só o keepalive em memória agora; tratar `cache.Session` como
melhoria futura opcional.

### Harness de testes com vcsim (junto da Fase 0)

`tools/testhelpers_test.go` com `newSimClient(t, model)` que sobe
`simulator.Model.Create()` + `model.Service.NewServer()` e constrói um
`*vmware.Client` real via `vmware.NewClient` (cobre o caminho de
login/keepalive também, não um atalho). Um arquivo de teste por domínio,
chamando o handler pela própria `Registry.CallTool`. `client_keepalive_test.go`
prova sobrevivência a timeout comprimido (`sim25.SetSessionTimeout`).

### Fase 2 — `tools/inventory.go` (leitura, baixo risco) — antes da Fase 1

Todos via `find.Finder`, sem Task: `vmware_list_hosts` (`HostSystemList`),
`vmware_list_datastores` (`DatastoreList` + `Properties(["summary"])`),
`vmware_list_networks` (`NetworkList`), `vmware_list_resource_pools`
(`ResourcePoolList`), `vmware_list_clusters` (`ClusterComputeResourceList`),
`vmware_list_datacenters` (`DatacenterList`). `vmware_list_vms` já existe em
`system.go` — deixar lá. Handlers toleram listas vazias (ESXi standalone não
tem cluster/DC múltiplos).

### Fase 1 — `tools/vm.go` (lifecycle + snapshots) — feature principal

Helper compartilhado: `waitForTask(ctx, t *object.Task)` → `t.WaitForResult(ctx)`
(síncrono do ponto de vista do tool MCP); `resolveVM(ctx, client, args)` →
`client.Finder.VirtualMachine(ctx, args["vm"])`.

Tools (todos em `object.VirtualMachine`, value receiver): `vmware_vm_power_on`
(`PowerOn`), `vmware_vm_power_off` (`PowerOff`), `vmware_vm_reset` (`Reset`),
`vmware_vm_suspend` (`Suspend`), `vmware_vm_reconfigure` (`{vm, num_cpus?,
memory_mb?}` → `Reconfigure`), `vmware_vm_destroy` (`{vm, confirm: bool
obrigatório=true}` → `Destroy`), `vmware_vm_snapshot_create` (`CreateSnapshot`),
`vmware_vm_snapshot_revert` (`RevertToSnapshot`), `vmware_vm_snapshot_remove`
(`RemoveSnapshot`), `vmware_vm_snapshot_list` (leitura de propriedade
`snapshot`), `vmware_vm_info` (`PowerState` + `Properties(["summary"])`).
`vmware_vm_destroy` fica atrás de `confirm:true` explícito (irreversível).
`reconfigure` só cobre CPU/RAM nesta rodada.

### Fase 3 — `tools/host.go`

Resolve host via `Finder.HostSystem`. Tools: `vmware_host_maintenance_enter`
(`EnterMaintenanceMode(ctx, timeout, evacuate, nil)`),
`vmware_host_maintenance_exit` (`ExitMaintenanceMode`), `vmware_host_reconnect`
(`Reconnect(ctx, nil, nil)`), `vmware_host_management_ips` (`ManagementIPs`),
`vmware_host_info` (`Properties(["summary"])`). Em ESXi standalone,
`evacuate`/DRS não tem efeito (conceito de cluster) — documentar na descrição
do tool.

### Fase 4 — `tools/appliance.go` (VAMI) — por último, de propósito

Accessor lazy `Client.REST(ctx)` em `vmware/client.go` (usa `cfg`/`rest`/`mu`
da Fase 0), login via `rest.NewClient(c.Client.Client)` +
`rc.Login(ctx, url.UserPassword(...))`; erro nomeia a causa provável ("é uma
vCenter Server Appliance? ESXi standalone não tem VAMI"). Init lazy: conectar a
ESXi não deve falhar no startup só por VAMI ausente.

Starter tools (fatia inicial, não as ~130 rotas documentadas em
`.workspace/vSphere Automation REST Resources for appliance...json`):
`vmware_appliance_version` (`GET /appliance/system/version`),
`vmware_appliance_health` (`GET /appliance/health/system`),
`vmware_appliance_health_detail` (uma chamada por subsistema, agregado),
`vmware_appliance_uptime` (`GET /appliance/system/uptime`) — via
`rc.Resource(path).Request(GET)` + `Do`, sem wrapper tipado no govmomi para
esses 4. Se no futuro entrarem tools de access/logging/networking/shutdown,
usar os pacotes tipados já vendorizados
`src/vapi/appliance/{access,logging,networking,shutdown}` (confirmados por
`ls` real, com simulador próprio) em vez de mais chamadas genéricas.

## Dependências e riscos

- **Bloqueio de ordem:** Fase 0 (sessão/keepalive) precisa landar antes/junto
  das demais — todo tool novo passa pelo client corrigido; fazer depois geraria
  retrabalho.
- **Risco de escopo VAMI:** os 4 tools da Fase 4 não têm como ser validados
  ponta-a-ponta hoje (nem no host real, nem no vcsim) — risco aceito e
  documentado, não bloqueante para as demais fases.
- **Risco de teste destrutivo:** `vmware_vm_destroy` e toggles de maintenance
  mode contra `10.100.2.54` só devem rodar contra alvo de teste combinado
  (a VM `cac-WN02` já confirmada existir, ou o próprio host) — nunca sem
  combinação explícita antes, por ser o único host de teste disponível.
- **Credenciais:** nunca persistidas em arquivo do projeto (padrão já
  registrado em `.wolf/STATUS.md`) — pedidas ao usuário a cada sessão de teste
  manual, ou via env vars (`VCENTER_URL`/`VCENTER_USERNAME`/`VCENTER_PASSWORD`/
  `VCENTER_INSECURE`) já suportadas por `cmd/mcpvmware-mcp`.

## Critérios de conclusão

- [ ] `go build ./...`, `go vet ./...` e `go test ./...` limpos (suíte vcsim +
      fixture httptest para VAMI) após cada fase.
- [ ] Fase 0: `vmware_about`/`vmware_list_vms` continuam funcionando via stdio
      contra `10.100.2.54` com o novo caminho de login/keepalive.
- [ ] Fase 2: listas de inventário não-vazias contra `10.100.2.54` (hosts,
      datastores, redes existem no host real) e contagens > 0 no vcsim.
- [ ] Fase 1: ciclo completo de power on/off/reset/suspend/reconfigure e
      snapshot create→revert→remove verde no vcsim; smoke manual em
      `10.100.2.54` (`vmware_vm_info` + ciclo de snapshot na VM `cac-WN02`,
      combinado antes).
- [ ] Fase 3: enter/exit maintenance + management IPs verdes no vcsim; tools de
      leitura (`management_ips`, `info`) livres contra `10.100.2.54`.
- [ ] Fase 4: parse/marshal correto contra fixture `httptest.Server`; limitação
      de não-verificável-ponta-a-ponta permanece documentada no código/tools,
      não removida silenciosamente.
- [ ] Suíte vcsim verde em CI, independente do host real (guarda de regressão).

## Referências

- `.wolf/STATUS.md` — mapa de superfície de API, achados de teste contra
  `10.100.2.54`, decisões fechadas.
- Arquitetura de referência: `D:\MCPNas\truenas-mcp` (memória
  `reference_truenas_mcp_golang`).
- `src/session/keepalive/handler.go` + `example_test.go` — padrão de keepalive.
- `src/object/task.go` — `WaitForResult`, padrão síncrono de Task.
- `src/vapi/rest/client.go` — cliente REST reusável para VAMI.
- `src/simulator/model.go` — `Model`/`NewServer`/`URL` do harness vcsim.

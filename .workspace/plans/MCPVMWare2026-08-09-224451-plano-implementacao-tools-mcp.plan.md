---
title: "MCPVMWare — dos 2 tools seed a um MCP VMware utilizável"
created: 2026-08-09
updated: 2026-08-10
status: implemented-pending-live-verification
locale: pt-BR
overview: "Adicionar VM lifecycle, snapshots, inventário, host ops e VAMI ao servidor MCP, com fundação de sessão/keepalive e harness vcsim."
---

## Resumo

O servidor MCP (`d:\MCPVMWare`) já tem um scaffold Go validado ponta a ponta
contra hardware real (ESXi 7.0.3 standalone, `10.100.2.54`): `go.mod`,
`mcp/types.go`, `vmware/client.go`, `tools/registry.go`+`tools/system.go` com 2
tools seed (`vmware_about`, `vmware_list_vms`), rodando via stdio JSON-RPC em
`src/mcpvmware-mcp/main.go`. Falta a superfície real de operações VMware — hoje
o servidor só "vê" o inventário, não age sobre ele. Este plano adiciona VM
lifecycle, snapshots, inventário, operações de host e administração de
Appliance (VAMI), com uma fundação de sessão/keepalive e um harness de testes
com `vcsim` para não depender do host real em toda verificação.

## Objectivos

*(todos concluídos 10/08/2026 — 28 tools no total, ver "Referências" §Estado
real do `src/` para o detalhe por ficheiro; falta só o smoke manual contra
`10.100.2.54`, bloqueado por rede nesta sessão, não por código)*

- [x] Sessão de longa duração sem expirar silenciosamente (keepalive) antes de
      empilhar tools novas.
- [x] Harness de testes offline com `referencia/govmomi/vcsim` cobrindo os domínios novos.
- [x] Tools de inventário (host/datastore/rede/resource-pool/cluster/datacenter).
- [x] Proteção em 3 camadas para tools destrutivas Tier 1/2 (confirm estrito +
      gate de servidor `--allow-destructive` + auditoria local em JSON Lines)
      antes de expor a primeira tool Tier 1/2 — fundação da Fase 1a, mesmo
      espírito do keepalive da Fase 0.
- [x] Tools de VM lifecycle (power on/off/reset/suspend/reconfigure/destroy) e
      snapshots (create/revert/remove/list).
- [x] Tools de host ops (maintenance mode, reconnect, IPs de gestão, info).
- [x] Tools de administração de Appliance/VAMI (version/health/uptime), com a
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

**Material de referência adicional (levantado 10/08/2026, `referencia/`
ganhou 11 pastas novas desde a versão anterior deste plano):** 3 servidores
MCP VMware de terceiros — `vmware-esxi-mcp` (Python/pyvmomi, alvo ESXi
standalone, MIT), `vmware-vcenter-mcp` (Python/vSphere Automation SDK,
multi-tenant/HA, MIT) e `vmware-vsphere-mcp-server` (Python, focado em
integração AnythingLLM, MIT) — mais `ssh-mcp-server` (Python/Paramiko, admin
remota via SSH, MIT), catalogados como referência de nomenclatura/cobertura de
domínio, não como código a reutilizar (stack Python, este projeto é Go). Os 3
convergem no núcleo já coberto por este plano (power/lifecycle de VM,
snapshots, inventário de host/datastore/rede/DC/cluster, maintenance mode),
mas cada um cobre também domínios fora do escopo aprovado: storage/VMFS/iSCSI
provisioning e segurança (firewall/certs/RBAC) no `vmware-esxi-mcp`;
DRS/HA/vSAN, templates, vMotion e um workflow engine multi-tenant no
`vmware-vcenter-mcp`; eventos/alarmes e relatórios no
`vmware-vsphere-mcp-server`. Nenhum destes gaps entra no escopo actual — ficam
documentados como candidatos para uma ronda futura (ver "Decisões em aberto"
mais abaixo), mesma lógica do item VMware Cloud on AWS.

**Correcção/aprofundamento (10/08/2026, cruzando com `.wolf/STATUS.md`
"Done" 09/08/2026 22:43 — análise de código-fonte por 4 agentes Explore em
paralelo, mais funda que o levantamento de README feito nesta revisão):** os
domínios listados acima nos READMEs nem sempre reflectem a implementação
real. `vmware-esxi-mcp` é essencialmente um **mock** — nunca chama `pyvmomi`
de facto, os testes referenciam módulos inexistentes, e o CI mascara as
falhas; baixo valor mesmo como inspiração de padrão. `vmware-vcenter-mcp` é
100% SOAP/pyVmomi na prática — a dependência do SDK REST oficial listada no
`requirements.txt` nunca é importada, não é prova de nada sobre REST/VAPI.
`vmware-vsphere-mcp-server` é confirmado o mais real dos 3 (41 tools, híbrido
REST+pyVmomi). Nenhum dos 3 resolve VAMI. **Dois bugs reais identificados a
não replicar:** parâmetro `hostname` decorativo/ignorado (aceito mas nunca
usado); TLS `CERT_NONE` hardcoded que ignora a flag `insecure` do usuário
(falso senso de segurança). Em contrapartida, confirma que os padrões já
adoptados no scaffold Go estão certos: `confirm:bool` obrigatório em
operações destrutivas, resposta sempre JSON estruturado (nunca Markdown
solto), `PropertyCollector`/`Finder` em lote em vez de N+1 chamadas.

Também chegaram os SDKs oficiais `vsphere-automation-sdk-{java,python,rest}`
(o `-rest` está descontinuado/read-only, samples Postman só para
vSphere/vCenter 6.7+) — não mudam a conclusão já registada de que não há
alternativa Go melhor que `govmomi` (`vsphere-automation-sdk-go` continua Beta
e sem suporte vSphere/vCenter), só confirmam que a API REST/VAPI é a mesma
independentemente da linguagem do cliente.

**Confirmação por fonte primária (10/08/2026, via `pdftotext` sobre os 5
PDFs oficiais em `referencia/` — ver `.wolf/STATUS.md` "Done" 10/08/2026 para
o achado de processo associado: 2 subagentes que tentaram ler estes PDFs pela
tool `Read` alucinaram conteúdo plausível mas falso, porque este ambiente não
tem `pdftoppm`/poppler; os dados abaixo vêm de extracção real via
`pdftotext`, não dos sumários alucinados):** os guias oficiais
`vmware-vsphere-sdks-and-tools-{6-5,6-7,7-0}.pdf` confirmam que nunca existiu
binding oficial Go — só Java/C#/.NET/Python/Perl — tanto para o Web Services
SDK (SOAP, o que `govmomi` espelha) quanto para o vSphere Automation SDK
(REST/VAPI, que amadurece de uma nota de configuração de endpoint em 6.5 para
um "Programming Guide" completo em 7.0). `vmware-vsphere-sdks-and-tools-9-1.pdf`
já não tem esses dois capítulos (só VDDK/OVF Tool/vSphere Client SDK) —
não investigado a fundo onde foi parar a doc de API SOAP/REST nas versões
mais recentes; não bloqueia nada porque o alvo de teste real é 7.0.3.
`vmware-vsphere-7-0.pdf` não é um doc de API (é a documentação geral do
produto — instalação/upgrade/release notes) — baixa relevância, catalogado
só por completude.

E material guest-side, também fora do escopo actual: `open-vm-tools` (serviço
que corre DENTRO da VM convidada, não é API host/vCenter),
`VMware-GuestSDK-{10.2.0,11.0.0,13.0.0}` (SDK C/C++/Java para uma app dentro
do guest reportar estado via app-monitoring API) e `VMware-vix-disklib-*`
(API C nativa para acesso a nível de bloco a ficheiros `.vmdk`, usada por
ferramentas de backup tipo Veeam/VADP — sem bindings Go, exigiria cgo).
Achado relevante para se guest operations entrar em escopo num round futuro:
o protocolo VIX (execução de comando/transferência de ficheiro dentro do
guest, assumindo open-vm-tools instalado) já tem implementação Go pronta em
`referencia/govmomi/toolbox/vix/protocol.go` — não precisaria de nova
dependência, só de tools novos sobre esse pacote.

**Tensão conhecida, não escondida:** o único alvo real validado
(`10.100.2.54`) é um ESXi **standalone** — não expõe VAMI (exclusivo de vCenter
Server Appliance). O simulador `referencia/govmomi/vapi/appliance/simulator`
só cobre `access/logging/networking/shutdown` (confirmado por `ls` real em
`referencia/govmomi/vapi/appliance/`), não `health`/`system/version`. Os tools de VAMI
propostos compilam e têm teste unitário (fixture HTTP), mas **não são
verificáveis ponta-a-ponta** até existir uma vCenter Appliance real de teste.

**Achado que simplifica o plano:** não é preciso escrever um cliente REST do
zero para VAMI — `github.com/vmware/govmomi/vapi/rest`
(`referencia/govmomi/vapi/rest/client.go` — cópia local só para leitura;
em build a dependência resolve via `go.mod`/`go.sum` no módulo cache, nunca
importada do clone local, confirmado no cabeçalho de pacote de
`src/vmware/client.go` já escrito na Fase 0) implementa o fluxo de sessão CIS (`POST
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
| `vmware/client.go` | editar (✅ já feito, Fase 0) | reescrita do keepalive (Fase 0) + accessor `REST()` lazy (Fase 4, campos já presentes) — mudança de maior impacto |
| `tools/registry.go` | editar (✅ feito 10/08/2026) | `registerTools()` ganha 1 linha por domínio novo; `Registry`/`NewRegistry` ganham `RegistryOptions{AllowDestructive,AuditLogPath}` (Fase 1a) |
| `tools/destructive.go` | criar (✅ feito 10/08/2026) | tiers 1/2, `wrapDestructive`, `registerDestructive`, auditoria JSON Lines (Fase 1a — fundação antes da Fase 1) |
| `tools/vm.go` | criar (✅ feito 10/08/2026) | VM lifecycle + snapshots (Fase 1) |
| `tools/inventory.go` | criar (✅ feito 10/08/2026) | listagens de host/datastore/rede/resource-pool/cluster/datacenter (Fase 2) |
| `tools/finder.go` | criar (✅ feito 10/08/2026, não previsto no plano original) | `dcScopedPath`/`emptyOnNotFound` — necessário para funcionar em vCenter multi-datacenter, não só ESXi standalone (bug-002 em `.wolf/buglog.json`) |
| `tools/host.go` | criar (✅ feito 10/08/2026) | operações de host (Fase 3) |
| `tools/appliance.go` | criar (✅ feito 10/08/2026) | tools de VAMI (Fase 4) |
| `mcpvmware-mcp/main.go` | editar (✅ feito 10/08/2026) | novas flags/env `--allow-destructive`/`VCENTER_ALLOW_DESTRUCTIVE` e `--audit-log-path`/`MCPVMWARE_AUDIT_LOG` (Fase 1a) |
| `tools/testhelpers_test.go` | criar (✅ já existe, ver "Referências") | helper `newSimClient` com `referencia/govmomi/vcsim` |
| `vmware/client_keepalive_test.go` | criar (✅ já existe — pacote `vmware`, não `tools`; corrigido nesta revisão) | prova de keepalive sobrevivendo a timeout comprimido |
| `tools/destructive_test.go` | criar (✅ feito 10/08/2026) | prova das 4 combinações (gate/confirm/auditoria permitida/negada) com tool Tier1 dummy — Fase 1a |
| `tools/vm_test.go`, `tools/inventory_test.go`, `tools/host_test.go`, `tools/appliance_test.go` (✅ todos feitos 10/08/2026) | criar | testes por domínio (vcsim, exceto appliance = fixture httptest) |

## Passos / fases

Ordem de construção (motivo em cada fase):

### Fase 0 — Sessão/keepalive (`vmware/client.go`) — primeiro, bloqueante

**Problema real:** `govmomi.NewClient(ctx, u, insecure)` faz login *dentro* da
chamada (`referencia/govmomi/client.go:79-84`), cedo demais para plugar um round-tripper de
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
ticker ao observar o round-trip de login, `referencia/govmomi/session/keepalive/handler.go:157-159`;
padrão exato em `referencia/govmomi/session/keepalive/example_test.go:62-67`). `Close`
continua chamando `Logout` — o keepalive para sozinho no round-trip de logout.

**Nota sobre `session/cache.Session`:** voltado a muitas invocações CLI curtas,
grava tickets em `~/.govmomi/sessions` — tensiona com a postura já registrada
em `.wolf/STATUS.md` de nunca persistir credenciais em arquivo. Recomendação:
implementar só o keepalive em memória agora; tratar `cache.Session` como
melhoria futura opcional.

### Harness de testes com vcsim (junto da Fase 0)

`tools/testhelpers_test.go` (implementado como `src/tools/testhelpers_test.go`
na Fase 0 — ver "Referências" abaixo) com `newSimClient(t, model)` que sobe
`simulator.Model.Create()` + `model.Service.NewServer()` e constrói um
`*vmware.Client` real via `vmware.NewClient` (cobre o caminho de
login/keepalive também, não um atalho). Um arquivo de teste por domínio,
chamando o handler pela própria `Registry.CallTool`. `client_keepalive_test.go`
prova sobrevivência a timeout comprimido (`sim25.SetSessionTimeout`).

### Fase 2 — `tools/inventory.go` (leitura, baixo risco) — antes da Fase 1

**✅ Concluída 10/08/2026.** 2 bugs reais achados e corrigidos por evidência
(vcsim), não previstos neste texto original — ver `.wolf/buglog.json`
bug-001/bug-002 e `.wolf/STATUS.md` "Done" 10/08/2026: (1) `newSimClient`
(harness da Fase 0) tinha duplo-login, nunca antes exercitado por um teste
real; (2) `client.Finder` sem datacenter default (vCenter com 2+
datacenters) quebrava qualquer list method com path relativo — corrigido com
`tools/finder.go` (`dcScopedPath`/`emptyOnNotFound`, não previsto
originalmente), aplicado também retroativamente a `vmware_list_vms` (Fase 0).
`go build/vet/gofmt/test` limpos; `inventory_test.go` prova as duas
topologias (vCenter 2-datacenters via `simulator.VPX()` + ESXi standalone via
`simulator.ESX()`). **Pendente:** smoke manual contra `10.100.2.54` —
bloqueado por falta de rota de rede nesta sessão (não é bug de código).

Todos via `find.Finder`, sem Task: `vmware_list_hosts` (`HostSystemList`),
`vmware_list_datastores` (`DatastoreList` + `Properties(["summary"])`),
`vmware_list_networks` (`NetworkList`), `vmware_list_resource_pools`
(`ResourcePoolList`), `vmware_list_clusters` (`ClusterComputeResourceList`),
`vmware_list_datacenters` (`DatacenterList`). `vmware_list_vms` já existe em
`system.go` — deixar lá. Handlers toleram listas vazias (ESXi standalone não
tem cluster/DC múltiplos).

### Fase 1a — Proteção contra ações destrutivas (fundação, antes da Fase 1)

**✅ Concluída 10/08/2026.** `tools/destructive.go` implementado exactamente
como desenhado abaixo — `Registry.registerDestructive`/`wrapDestructive`
(ordem: gate → confirm estrito → handler → auditoria; negação em qualquer
camada grava auditoria na mesma). `Registry`/`NewRegistry` ganharam
`RegistryOptions{AllowDestructive, AuditLogPath}` (mudança de assinatura,
todos os call-sites — `main.go`, `inventory_test.go` — actualizados).
`tools/destructive_test.go` prova as 4 combinações (gate fechado nega antes
do handler correr; `confirm` estrito rejeita string/int truthy; permitido
corre+audita; negado só audita) com uma tool Tier1 **dummy**, registada só no
teste — ainda não existe nenhuma tool real Tier1/2 (isso é a Fase 1/3).
`go build/vet/gofmt/test` limpos.

*(decisão fechada com o usuário via AskUserQuestion, 10/08/2026: 3 camadas —
confirm por chamada + gate de servidor + auditoria; descartada a opção de
preview/token de 2 etapas por complexidade extra do lado do cliente MCP)*

**Classificação por severidade** (por reversibilidade, não por "é uma tool de
lifecycle" — `vmware_vm_reconfigure`, `vmware_host_reconnect`,
`vmware_host_management_ips`, `vmware_host_info`, `vmware_host_maintenance_exit`
e tudo da Fase 2 ficam de fora, não passam pelas 3 camadas):

| Tier | Tools | Motivo |
|---|---|---|
| **1 — irreversível** | `vmware_vm_destroy`, `vmware_vm_snapshot_remove`, `vmware_vm_snapshot_revert` | Apaga a VM/discos; mescla e apaga estado do snapshot; descarta o estado actual da VM (dados desde o snapshot perdidos) |
| **2 — disruptivo, reversível** | `vmware_vm_power_off`, `vmware_vm_reset`, `vmware_vm_suspend`, `vmware_host_maintenance_enter` | Downtime + risco de dados não-gravados no guest em power off/reset não-graceful; em ESXi standalone `maintenance_enter` não tem cluster para evacuar — pode forçar desligar as VMs do próprio host, tão disruptivo quanto Tier 1 nesse cenário |

**Camada 1 — `confirm:bool` por chamada:** já previsto no plano original só
para `vmware_vm_destroy` — estendido agora a TODAS as tools Tier 1 e Tier 2.
Checagem estrita (`args["confirm"] == true`, não aceitar `"true"` string nem
truthy genérico) — rejeita com erro claro antes de tocar em `client`.

**Camada 2 — gate de servidor (`--allow-destructive` / `VCENTER_ALLOW_DESTRUCTIVE`):**
nova flag/env em `mcpvmware-mcp/main.go` (mesma família de `--insecure`),
default `false`. Se `false`, qualquer chamada a tool Tier 1/2 falha
imediatamente — **antes** de qualquer round-trip a vCenter e **independente**
do `confirm` recebido. Protege contra um cliente MCP (o LLM do outro lado)
que manda `confirm:true` sozinho, sem intervenção humana — o operador tem de
ligar o gate explicitamente ao arrancar o servidor.

**Camada 3 — auditoria local:** cada chamada a tool Tier 1/2 (passe ou falhe
nas camadas 1-2) grava uma linha JSON Lines — tool, args, timestamp,
resultado, e se o gate estava activo — em `--audit-log-path`
(env `MCPVMWARE_AUDIT_LOG`), **desligado por padrão** (não escreve em disco
silenciosamente sem o operador pedir — mesmo princípio já usado no keepalive
em memória vs `session/cache.Session` da Fase 0). Caminho de exemplo sugerido
na descrição do flag: `logs/destructive-actions.jsonl`, a entrar no
`.gitignore` (rasto operacional, não pertence ao repo — não é credencial mas
também não deve ser versionado).

**Implementação:** `tools/destructive.go` novo — tipo `tier` (1/2), lista
central de nomes de tools por tier, `wrapDestructive(tier, handler)` que
embrulha o handler já escrito (camadas 1-3) e é chamado a partir de um novo
`r.registerDestructive(name, description, tier, schema, handler)` em vez de
`r.register(...)` direto, para as tools Tier 1/2 de `vm.go`/`host.go`.
`Registry` ganha `allowDestructive bool` + `auditLogPath string`, preenchidos
por `NewRegistry` a partir de um novo parâmetro (`RegistryOptions` ou
similar) — `main.go` passa os valores lidos das novas flags/env vars.

### Fase 1 — `tools/vm.go` (lifecycle + snapshots) — feature principal

**✅ Concluída 10/08/2026.** `resolveVM` usa `dcScopedPath("vm", ...)`
(não `client.Finder.VirtualMachine(ctx, args["vm"])` puro como o texto
original abaixo sugeria — mesmo motivo da Fase 2, funciona em vCenter
multi-DC). Tier1 (`destroy`, `snapshot_revert`, `snapshot_remove`) e Tier2
(`power_off`, `reset`, `suspend`) via `r.registerDestructive(...)` da
Fase 1a — primeira vez que tools reais passam pelas 3 camadas. **2 achados
via teste, não assumidos** (ver `.wolf/cerebrum.md`): `Destroy_Task` do
vSphere rejeita VM ligada — `vmware_vm_destroy` não desliga a VM sozinha
(deliberado, uma tool Tier1 fazer trabalho extra destrutivo silenciosamente
seria pior); `NumCPUs`/`MemoryMB` com `omitempty` confirmados seguros para
reconfigure parcial (deixar um no zero-value do Go não zera o campo no
vSphere). `tools/vm_test.go` — 6 testes, `go build/vet/gofmt/test` limpos.
Smoke manual contra `10.100.2.54` pendente — bloqueado por rede nesta sessão.

Helper compartilhado: `waitForTask(ctx, t *object.Task)` → `t.WaitForResult(ctx)`
(síncrono do ponto de vista do tool MCP); `resolveVM(ctx, client, args)` →
`client.Finder.VirtualMachine(ctx, args["vm"])`.

Tools (todos em `object.VirtualMachine`, value receiver): `vmware_vm_power_on`
(`PowerOn`), `vmware_vm_power_off` (`PowerOff`, **Tier 2**), `vmware_vm_reset`
(`Reset`, **Tier 2**), `vmware_vm_suspend` (`Suspend`, **Tier 2**),
`vmware_vm_reconfigure` (`{vm, num_cpus?, memory_mb?}` → `Reconfigure`, sem
tier), `vmware_vm_destroy` (`{vm, confirm: bool obrigatório=true}` →
`Destroy`, **Tier 1**), `vmware_vm_snapshot_create` (`CreateSnapshot`, sem
tier — criar não é destrutivo), `vmware_vm_snapshot_revert`
(`RevertToSnapshot`, **Tier 1**), `vmware_vm_snapshot_remove`
(`RemoveSnapshot`, **Tier 1**), `vmware_vm_snapshot_list` (leitura de
propriedade `snapshot`, sem tier), `vmware_vm_info` (`PowerState` +
`Properties(["summary"])`, sem tier). Tools Tier 1/2 registadas via
`r.registerDestructive(...)` da Fase 1a — não via `r.register(...)` direto.
`reconfigure` só cobre CPU/RAM nesta rodada.

### Fase 3 — `tools/host.go`

**✅ Concluída 10/08/2026.** Implementado exactamente como desenhado abaixo —
`resolveHost` usa `dcScopedPath("host", ...)`. `tools/host_test.go` — 5
testes: ciclo maintenance enter→info(true)→exit→info(false); gate fechado
nega enter sem tocar o host; enter sem confirm nega; management_ips +
reconnect não erroram; info tem todos os campos e reporta `poweredOn` pro
host. `go build/vet/gofmt/test` limpos. Smoke manual contra `10.100.2.54`
pendente — bloqueado por rede nesta sessão.

Resolve host via `Finder.HostSystem`. Tools: `vmware_host_maintenance_enter`
(`EnterMaintenanceMode(ctx, timeout, evacuate, nil)`, **Tier 2** — via
`r.registerDestructive(...)`), `vmware_host_maintenance_exit`
(`ExitMaintenanceMode`, sem tier), `vmware_host_reconnect` (`Reconnect(ctx,
nil, nil)`, sem tier), `vmware_host_management_ips` (`ManagementIPs`, sem
tier), `vmware_host_info` (`Properties(["summary"])`, sem tier). Em ESXi
standalone, `evacuate`/DRS não tem efeito (conceito de cluster) — documentar
na descrição do tool, e ver nota de severidade da Fase 1a (sem cluster para
evacuar, `maintenance_enter` pode desligar as VMs do próprio host).

### Fase 4 — `tools/appliance.go` (VAMI) — por último, de propósito

**✅ Concluída 10/08/2026 — última fase do plano.** Os 4 endpoints
confirmados por leitura estruturada (Python) da própria collection Postman
já vendorizada, não adivinhados; `health_detail` usa a lista real de 8
subsistemas (`system`, `applmgmt`, `database-storage`, `load`, `mem`,
`software-packages`, `storage`, `swap`). **Sem struct tipado** para as
respostas (decode genérico) — decisão deliberada, sem simulador/vCenter
Appliance real pra validar nomes de campo. `tools/appliance_test.go` — 5
testes via fixture `httptest.Server` (não vcsim — nem ele nem o host real
simulam estes endpoints): version/uptime/health individuais; health_detail
agrega os 8; 1 subsistema falhando (404) não derruba a chamada inteira.
**Achado de teste:** `vmware.Client.REST(ctx)` só depende do `*vim25.Client`
embutido, nunca toca `ServiceContent` — dá pra construir um `*vmware.Client`
mínimo apontado direto a um fixture HTTP sem vcsim nenhum. `go build/vet/gofmt/test`
limpos (27 testes nos 2 pacotes). Limitação de não-verificável-ponta-a-ponta
contra VAMI real permanece — nem host real nem vcsim a cobrem.

Accessor lazy `Client.REST(ctx)` em `vmware/client.go` (usa `cfg`/`rest`/`mu`
da Fase 0), login via `rest.NewClient(c.Client.Client)` +
`rc.Login(ctx, url.UserPassword(...))`; erro nomeia a causa provável ("é uma
vCenter Server Appliance? ESXi standalone não tem VAMI"). Init lazy: conectar a
ESXi não deve falhar no startup só por VAMI ausente.

Starter tools (fatia inicial, não as ~130 rotas documentadas em
`.workspace/vSphere Automation REST Resources for appliance.postman_collection.json`):
`vmware_appliance_version` (`GET /appliance/system/version`),
`vmware_appliance_health` (`GET /appliance/health/system`),
`vmware_appliance_health_detail` (uma chamada por subsistema, agregado),
`vmware_appliance_uptime` (`GET /appliance/system/uptime`) — via
`rc.Resource(path).Request(GET)` + `Do`, sem wrapper tipado no govmomi para
esses 4. Se no futuro entrarem tools de access/logging/networking/shutdown,
usar os pacotes tipados
`referencia/govmomi/vapi/appliance/{access,logging,networking,shutdown}`
(confirmados por `ls` real, com simulador próprio; resolvidos em build via
`go.mod`, não pela cópia local em `referencia/`) em vez de mais chamadas genéricas.

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
  Mitigado estruturalmente pela Fase 1a (gate `--allow-destructive` desligado
  por padrão) — mesmo assim a combinação explícita antes de cada teste manual
  continua obrigatória, o gate protege contra chamada acidental/automática,
  não substitui o combinado humano.
- **Credenciais:** nunca persistidas em arquivo do projeto (padrão já
  registrado em `.wolf/STATUS.md`) — pedidas ao usuário a cada sessão de teste
  manual, ou via env vars (`VCENTER_URL`/`VCENTER_USERNAME`/`VCENTER_PASSWORD`/
  `VCENTER_INSECURE`) já suportadas por `src/mcpvmware-mcp`.

## Decisões em aberto

*(surgidas na reanálise de 10/08/2026 dos arquivos/pastas novos em
`referencia/` — nenhuma decidida ainda; nenhuma bloqueia a Fase 2, que segue
como aprovada)*

- **VMware Cloud on AWS entra no escopo?** Produto/control-plane distinto do
  vSphere on-prem (endpoint `console.cloud.vmware.com`, auth CSP token) — sem
  relação com `govmomi`. Só se o objetivo for gerenciar SDDCs na AWS.
  *(decisão já registada antes desta reanálise, repetida aqui por
  consistência com as duas novas abaixo.)*
- **Guest operations (protocolo VIX) entram num round futuro?** Execução de
  comando e transferência de ficheiro dentro do guest OS via vCenter, sem rede
  direta à VM — requer open-vm-tools instalado no guest. Implementação Go já
  existe em `referencia/govmomi/toolbox/vix/protocol.go` (sem dependência
  nova a adicionar). Motivador provável do material `open-vm-tools` /
  `VMware-GuestSDK-*` que apareceu em `referencia/`.
- **Domínios de infraestrutura vistos nos 3 MCP concorrentes entram nalgum
  round futuro?** Storage/VMFS/iSCSI provisioning e segurança
  (firewall/certs/RBAC) — visto no `vmware-esxi-mcp`; DRS/HA/vSAN, templates
  e vMotion — visto no `vmware-vcenter-mcp`; eventos/alarmes e relatórios —
  visto no `vmware-vsphere-mcp-server`. Nenhum avaliado a fundo ainda (só
  levantamento de README).
- **Backup a nível de bloco via VixDiskLib entra em escopo?** API C nativa
  (`referencia/Windows|Linux/VMware-vix-disklib-*`) sem bindings Go
  conhecidos — exigiria cgo e uma avaliação de esforço própria antes de virar
  candidato real; por ora só documentado como material presente em
  `referencia/`, não como plano.

## Critérios de conclusão

- [ ] `go build ./...`, `go vet ./...` e `go test ./...` limpos (suíte vcsim +
      fixture httptest para VAMI) após cada fase.
- [x] Fase 0 (confirmado de novo 10/08/2026 16:33, host tinha ficado desligado
      entretanto): `vmware_about`/`vmware_list_vms` funcionando via stdio
      contra `10.100.2.54` real com o caminho de login/keepalive da Fase 0.
- [x] Fase 2 (10/08/2026 16:33): contagens > 0 no vcsim confirmadas para as 2
      topologias (vCenter multi-DC + ESXi standalone) **e** confirmado contra
      `10.100.2.54` real (4 datastores, 3 networks, 1 resource pool, 0
      clusters sem erro — prova bug-002 corrigido também no host real, não só
      vcsim).
- [x] Fase 1a (10/08/2026): com o gate desligado (padrão), a tool falha
      **sem** round-trip ao vcsim/host (prova por teste que o handler real
      nunca é chamado); com o gate ligado mas `confirm` ausente/errado/não-bool,
      idem; com gate ligado + `confirm:true`, a acção passa e uma linha é
      gravada no audit log; chamada negada também audita — as 4 combinações
      testadas no vcsim com uma tool Tier1 dummy (`tools/destructive_test.go`).
- [x] Fase 1 (parcial — 10/08/2026): ciclo completo de power
      on/off/reset/suspend/reconfigure e snapshot create→revert→remove verde
      no vcsim (`tools/vm_test.go`, 6 testes). [ ] falta o smoke manual em
      `10.100.2.54` (`vmware_vm_info` + ciclo de snapshot na VM `cac-WN02`) —
      rede já ok, falta só a combinação explícita do usuário antes de rodar
      (tools Tier1/2, não é mais bloqueio de rede).
- [x] Fase 3 (10/08/2026 16:33): enter/exit maintenance + management IPs
      verdes no vcsim (`tools/host_test.go`, 5 testes) **e** tools de leitura
      (`management_ips`, `info`) confirmadas livres contra `10.100.2.54` real
      (hardware real: Dell PowerEdge R720, Xeon E5-2697 v2, 24 cores, ~274GB
      RAM). `maintenance_enter` (Tier 2) segue pendente de combinação
      explícita — não é mais bloqueio de rede.
- [x] Fase 4 (10/08/2026): parse/marshal correto contra fixture
      `httptest.Server` (`tools/appliance_test.go`, 5 testes); limitação de
      não-verificável-ponta-a-ponta permanece documentada no código/tools,
      não removida silenciosamente.
- [x] Suíte vcsim verde (10/08/2026): 27 testes nos pacotes `tools`+`vmware`,
      100% PASS, `go build/vet/gofmt` limpos a partir de `src/`. CI ainda não
      configurado neste repositório (sem workflow `.github/` — fora do
      escopo deste plano de tools).

## Extensões pós-plano (fora das 6 fases originais)

- **`vmware_datastore_upload_file`** (10/08/2026) — `tools/datastore.go`.
  Pedido do usuário via chat ("consigo fazer upload de ISO?"), não fazia
  parte dos domínios fechados em 09/08/2026. `object.Datastore.UploadFile`
  + guarda de sobrescrita (`Datastore.Stat` antes, recusa sem
  `overwrite:true`). Sem tier/gate — cria/substitui um arquivo, não
  destrói VM/host, não se enquadra na classificação Tier1/2 da Fase 1a.
  `local_path` é lido do disco da máquina que roda o `mcpvmware-mcp.exe`,
  não recebido do cliente MCP (esclarecido com o usuário antes de
  implementar — o protocolo MCP não tem mecanismo pra isso). 2 testes
  reais contra o serviço HTTP de datastore do vcsim (não fixture — o
  simulador implementa isso de verdade). **Total: 29 tools registadas.**
  **✅ Verificada contra o host real 10.100.2.54 em 10/08/2026 17:05** —
  4 ISOs reais de instalação ESXi (~1.6GB) subidos para `Dado-HD/ISO` com
  100% sucesso (~88s total). Achado no caminho: a tool original exigia que
  a pasta destino já existisse — corrigido para criar automaticamente
  (`FileManager.MakeDirectory`, best-effort) antes desta verificação, senão
  o próprio pedido real do usuário teria falhado.

## Referências

**Estado real do `src/` do projeto, confirmado por leitura directa em
10/08/2026** (não é referência de terceiros — é o próprio código já escrito):
`src/go.mod` (módulo `github.com/cslsoftwares/mcpvmware`, `govmomi v0.55.1`),
`src/vmware/client.go` (keepalive + accessor `REST()` lazy da Fase 0, já
implementados), `src/tools/registry.go` + `src/tools/system.go` (`vmware_about`,
`vmware_list_vms`), `src/tools/testhelpers_test.go` (`newSimClient`) e
`src/vmware/client_keepalive_test.go` — harness da Fase 0 confirmado presente.
Binário do servidor stdio vive em `src/mcpvmware-mcp/main.go` (não
`cmd/mcpvmware-mcp` — corrigido nesta revisão, ver seção "Passos / fases").

**Fonte do govmomi vendorizado como referência de leitura** (todos os
`referencia/govmomi/...` abaixo — corrigidos nesta revisão; estavam escritos
como `src/...` antes da reorg de pastas que separou o `src/` real do projeto
do checkout de terceiros):

- `.wolf/STATUS.md` — mapa de superfície de API, achados de teste contra
  `10.100.2.54`, decisões fechadas.
- Arquitetura de referência: `D:\MCPNas\truenas-mcp` (memória
  `reference_truenas_mcp_golang`).
- `referencia/govmomi/session/keepalive/handler.go` + `example_test.go` —
  padrão de keepalive.
- `referencia/govmomi/object/task.go` — `WaitForResult`, padrão síncrono de Task.
- `referencia/govmomi/vapi/rest/client.go` — cliente REST reusável para VAMI.
- `referencia/govmomi/simulator/model.go` — `Model`/`NewServer`/`URL` do harness vcsim.
- `referencia/govmomi/toolbox/vix/protocol.go` — protocolo VIX (guest ops),
  candidato a round futuro, ver "Decisões em aberto".

**Material novo levantado nesta revisão (10/08/2026), todos como submódulos
git em `referencia/` — ver `.gitmodules` na raiz:**

- `referencia/vmware-esxi-mcp` (github.com/uldyssian-sh/vmware-esxi-mcp,
  Python/pyvmomi, MIT) — inspiração de nomenclatura, alvo ESXi standalone.
- `referencia/vmware-vcenter-mcp`
  (github.com/uldyssian-sh/vmware-vcenter-mcp, Python/vSphere Automation SDK,
  MIT) — inspiração de nomenclatura, alvo vCenter multi-tenant.
- `referencia/vmware-vsphere-mcp-server`
  (github.com/giuliolibrando/vmware-vsphere-mcp-server, Python, MIT) —
  inspiração de nomenclatura, foco integração AnythingLLM.
- `referencia/ssh-mcp-server` (github.com/giuliolibrando/ssh-mcp-server,
  Python/Paramiko, MIT) — fora do domínio vSphere, admin remota via SSH.
- `referencia/vsphere-automation-sdk-{java,python,rest}` — confirmam a mesma
  API REST/VAPI independente da linguagem; `-rest` descontinuado/read-only.
- `referencia/open-vm-tools`, `referencia/VMware-GuestSDK-*`,
  `referencia/Windows|Linux/VMware-vix-disklib-*` — material guest-side/
  disk-level, candidatos a round futuro (ver "Decisões em aberto").

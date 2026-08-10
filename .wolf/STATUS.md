# STATUS — MCPVMWare

> Single source of truth for resuming work. Read this FIRST when starting a session.
> Update this file at the end of every work phase so the next `/clear` resumes in 1 read.
> Last updated: 2026-08-09

---

## ✅ Done

<!-- Move items here from "🚀 Next phase" when finished. Group by area. -->

- **2026-08-09** — Mapeada a superfície de API disponível para o futuro MCP layer: SDK Go vendorizado em `src/` (govmomi, SOAP/vim25) + 7 coleções Postman em `.workspace/` (REST/VAPI). Ver tabela em "Active architecture" abaixo. Análise feita por evidência (leitura de go.mod/README/doc.go/client.go do govmomi + leitura direta das 7 coleções .postman_collection.json), não por suposição.
- **2026-08-09** — **(achado em sessão anterior, não registado até agora — corrigido)** Scaffold Go do servidor MCP já existe na raiz e compila limpo (`go build`/`go vet`/`gofmt` sem apontamentos): `go.mod` (`github.com/cslsoftwares/mcpvmware`, depende de `github.com/vmware/govmomi v0.55.1`), `cmd/mcpvmware-mcp/main.go` (entrypoint stdio JSON-RPC, flags `--vcenter-url/--username/--password/--insecure`), `mcp/types.go` (tipos MCP/JSON-RPC), `tools/registry.go`+`tools/system.go` (2 tools seed: `vmware_about`, `vmware_list_vms`), `vmware/client.go` (wrapper sobre `govmomi.NewClient`+`find.Finder`). Arquitetura de referência: `D:\MCPNas\truenas-mcp`. Verificado no disco em 09/08/2026 21:50 (não só por `.workspace/context.json` — confirmado por `ls` real).
- **2026-08-09 21:47-21:52** — Teste empírico contra `10.100.2.54` (via curl, sem credenciais): TLS self-signed confirmado (`curl` sem `-k` falha com erro 60); `/ui/` responde 200 (vSphere Client); **`POST /sdk` com `RetrieveServiceContent` real (não-autenticado por natureza do protocolo SOAP) confirma definitivamente: é um ESXi standalone 7.0.3 build 21930508 patch 0.95 (`apiType: HostAgent`, MOIDs `ha-*`) — NÃO é vCenter.** REST/VAPI (`/api/session` e `/rest/com/vmware/cis/session`) devolve 400 Bad Request corpo vazio mesmo com Basic auth dummy e headers corretos — inconclusivo sem credencial real, possivelmente API REST não totalmente exposta neste host ESXi bare.
- **2026-08-09 21:57** — **Teste autenticado completo e bem-sucedido** contra `10.100.2.54` (usuário `root`, senha fornecida pelo usuário via chat — não persistida em nenhum arquivo): build limpo (`go build ./cmd/mcpvmware-mcp`) + execução real via stdio JSON-RPC exercitando o pipeline inteiro:
  - `initialize` → OK (protocolVersion 2024-11-05)
  - `tools/list` → OK (2 tools: `vmware_about`, `vmware_list_vms`)
  - `tools/call vmware_about` → OK, dados reais batendo com a sonda SOAP anterior (ESXi 7.0.3 build 21930508, `api_type: HostAgent`)
  - `tools/call vmware_list_vms` → OK, **1 VM real encontrada**: `/ha-datacenter/vm/vmfs/volumes/6a72b614-3e3ede6b-a2f7-d4ae52918af0/cac-WN02/cac-WN02.vmx` (nome `cac-WN02`)
  - Login/sessão via `govmomi.NewClient` com `Insecure: true` funcionou de primeira — confirma que o wrapper `vmware/client.go` está correto contra um host real.
- **2026-08-09 22:20-22:33** — Pesquisado `github.com/vmware` + `github.com/vmware-archive` por repositórios úteis ao projeto. **Achado que corrige a linha de VAMI abaixo:** `src/vapi/` (na época ainda o vendor do govmomi) tem subpacotes `appliance/{access,logging,networking,shutdown}`, `authentication`, `cis`, `cluster`, `crypto`, `esx`, `library` (Content Library), `namespace`, `rest`, `tags`, `vcenter`, `vm` — ou seja, **VAMI é parcialmente coberto pelo govmomi**, não "sem equivalente" como registado antes (corrigido na tabela "Active architecture"). Confirmado também que `vmware/vsphere-automation-sdk-go` (SDK Go oficial p/ REST/VAPI) segue em Beta e **ainda não suporta vSphere/vCenter** (só VMC/NSX-T) — não é opção viável; `govmomi`'s próprio `vapi/rest` é o único cliente REST/VAPI Go-nativo disponível. Repos de referência de padrão (não dependências): `terraform-provider-vsphere` (671★, Go maduro sobre govmomi em produção) e `packer-plugin-vsphere` (126★, idem p/ criação de templates). Em `vmware-archive`: `vsphere-automation-sdk-rest` (196★, arquivado — confirma que as coleções Postman já vendorizadas em `.workspace/` são a referência REST mais atual disponível) e `cloud-init-vmware-guestinfo` (196★, arquivado — mecanismo já trazido indiretamente via dependência `vmw-guestinfo` do próprio govmomi).
- **2026-08-09** — Usuário clonou 8 repos de referência em `outros/` (govmomi + 3 SDKs oficiais + 3 MCP servers da comunidade + ssh-mcp-server genérico) e pediu análise. 4 agentes Explore em paralelo (sonnet p/ os 3 MCP servers, haiku p/ checagem rápida dos SDKs+ssh): **nenhum dos 3 MCP servers da comunidade resolve VAMI** — `vmware-vcenter-mcp` é 100% SOAP/pyVmomi sem nenhuma tool de appliance (dependência do SDK REST oficial no requirements.txt está morta, nunca importada); `vmware-esxi-mcp` é essencialmente um **mock** (nunca chama pyVmomi de verdade, testes referenciam módulos inexistentes, CI mascara falhas); `vmware-vsphere-mcp-server` é o mais real (híbrido REST+pyVmomi, 41 tools) mas também sem VAMI. Confirmam que os padrões já adotados no scaffold Go estão certos (`confirm:bool` p/ destrutivas, JSON estruturado em vez de Markdown solto, uso de PropertyCollector em lote em vez de N+1 REST calls) e expõem 2 bugs reais a NÃO replicar (`hostname` decorativo/ignorado, TLS `CERT_NONE` hardcoded ignorando a flag insecure). SDKs oficiais Java/Python: aviso de migração p/ "VCF SDK" em releases 9.0.0.0+. Conclusão: Fase 4 (VAMI) do plano continua sem precedente demonstrado — terreno próprio, como já assumido.
- **2026-08-09 22:48-22:55** — **Usuário reorganizou as pastas do repositório** (fora do meu controle, feito diretamente por ele): `src/` **inverteu de significado** — deixou de ser o vendor do govmomi e passou a ser o **módulo Go PRÓPRIO do MCPVMWare** (movido pra lá: `go.mod`, `mcpvmware-mcp/` [renomeado de `cmd/mcpvmware-mcp/`], `mcp/`, `tools/`, `vmware/`). `outros/` foi renomeado para `referencia/` e ganhou um clone próprio do govmomi (`referencia/govmomi/`) — todo o material de terceiros agora vive só em `referencia/`, fora de `src/`. Reconfirmado por mim (não só aceito por relato): `cd src && go build ./... && go vet ./...` limpo, `gofmt` sem apontamentos; `src/cmd/` (pasta vazia remanescente do move) removida; `referencia/govmomi` confirmado como o mesmo clone limpo (mesmo remote, working tree limpa). Atualizados `README.md`, `.workspace/context.json`, a rule local (`_V1.2.0`→`_V1.3.0`) e o próprio plano aprovado (`streamed-imagining-lightning.md`, todos os paths `src/...` que referenciavam fonte do govmomi corrigidos para `referencia/govmomi/...`) com os caminhos novos.
- **2026-08-09 23:00-23:08** — **Phase 0 implementada e verificada** (sessão/keepalive + harness vcsim, conforme plano):
  - `src/vmware/client.go` reescrito: `NewClient` conecta SEM userinfo → `gc.RoundTripper = keepalive.NewHandlerSOAP(gc.RoundTripper, idle, nil)` → **depois** `gc.Login(...)` (ordem confirmada lendo `referencia/govmomi/session/keepalive/handler.go` — o ticker só arma ao observar o round-trip de login). Extraído `newClient(ctx, cfg, idle time.Duration)` interno (idle configurável) — `NewClient` público chama com a constante de produção (`keepAliveIdle = 10*time.Minute`), testes chamam com idle comprimido. Adicionado accessor lazy `REST(ctx)` (Phase 4, ainda não usado por nenhum tool) e `Close` agora desloga a sessão REST também, se alguma vez foi aberta.
  - `src/tools/testhelpers_test.go` — `newSimClient` (helper compartilhado pros próximos arquivos de teste por domínio) sobe `simulator.Model`/`NewServer` e conecta via `vmware.NewClient` de verdade (não bypassa login/keepalive).
  - `src/vmware/client_keepalive_test.go` — 2 testes reais contra `simulator.Test`: `TestNewClientKeepAliveSurvivesSessionTimeout` (usa `sim25.SetSessionTimeout` pra comprimir o timeout do servidor a 250ms e um keepalive idle de 125ms — **prova de verdade** que a sessão sobrevive, não só "compila") e `TestNewClientLogsOutAndStopsKeepAlive` (prova que `Close` desloga mesmo).
  - **Evidência lida, não assumida:** `cd src && go build ./... && go vet ./... && gofmt -l .` — todos limpos (0 saída/erros). `go test ./... -v`: **2/2 PASS** em `vmware` (`TestNewClientKeepAliveSurvivesSessionTimeout` 0.86s, `TestNewClientLogsOutAndStopsKeepAlive` 0.36s); `tools` sem testes ainda (só o helper, esperado — os testes por domínio vêm nas próximas fases); `mcp`/`mcpvmware-mcp` sem testes (esperado, são tipos/entrypoint).
  - **Pendente:** critério de aceite #3 do plano (rebuild + smoke manual `vmware_about`/`vmware_list_vms` via stdio contra `10.100.2.54` real) — não executado ainda, precisa da credencial `root` de novo (não persistida em nenhum arquivo). A evidência automatizada (testes acima) já prova a lógica de keepalive; o smoke real só confirmaria que a MESMA sequência de login funciona contra hardware de verdade (já era o caso antes desta fase, mudou só a ordem interna de wiring do round-tripper).

---

## 🚀 Next phase

**Goal:** Phase 2 do plano aprovado — `src/tools/inventory.go` (leitura, baixo
risco): `vmware_list_hosts`, `vmware_list_datastores`, `vmware_list_networks`,
`vmware_list_resource_pools`, `vmware_list_clusters`, `vmware_list_datacenters`
(todos via `Finder.XList(ctx, path)`, `path` opcional default `"*"`, mesmo
padrão de `vmware_list_vms` já existente). Plano completo:
`D:\Users\claiton.linhares\.claude\plans\streamed-imagining-lightning.md`
§Phase 2 (paths já corrigidos pra reorg de pastas).

### Acceptance criteria
1. 6 tools novas registradas em `tools/registry.go` (`registerInventoryTools`
   ou similar, seguindo o padrão de `registerSystemTools`).
2. Handlers toleram listas vazias (ESXi standalone não tem cluster/DC
   múltiplos) em vez de erro.
3. `go build ./... && go vet ./... && gofmt -l .` limpo a partir de `src/`.
4. Teste vcsim (`simulator.VPX()`, via `newSimClient`) com contagens > 0 pra
   cada tool.
5. Lista manual contra `10.100.2.54` confirma hosts/datastores/redes reais
   (mesmo host já usado nos smokes anteriores).

### Files to create / edit
| Type | File | Content |
|---|---|---|
| New | `src/tools/inventory.go` | 6 tools de listagem via `Finder` |
| New | `src/tools/inventory_test.go` | Testes vcsim via `newSimClient` |
| Edit | `src/tools/registry.go` | +1 linha de registro (`registerInventoryTools(r)`) |

### Closed decisions
*(fechadas com o usuário via AskUserQuestion, 09/08/2026 — ver plano completo)*
- **Escopo:** vSphere on-prem (SOAP/`vim25`, já validado) **+ administração do Appliance (VAMI)**.
- **Domínios desta rodada:** VM lifecycle (power/reconfigure/destroy), Snapshots, Inventário (host/datastore/rede/resource-pool), Host ops (maintenance mode, reconnect, IPs).
- **Sessão de longa duração:** resolver agora (keepalive), é a Phase 0 — fundação, não um "depois". **✅ Implementada 09/08/2026 23:08** (ver "Done" acima), com 1 desvio sinalizado: `session/cache.Session` (interpretação literal da decisão) foi trocado por keepalive em memória — grava tickets em disco tensiona com "nunca persistir credenciais em arquivo"; ver nota no plano §Phase 0.
- **Testes:** ESXi real (`10.100.2.54`) para verificação manual + harness automatizado com `vcsim` (via `referencia/govmomi/simulator` como fonte de leitura, resolvido em produção via `go.mod`) para não depender do host real em toda verificação. **✅ Harness `newSimClient` implementado 09/08/2026 23:08.**
- **Transporte primário do MCP layer:** `govmomi` cobre os dois protocolos numa dependência só — `vim25` (SOAP) **e** `vapi/rest` (REST/VAPI) — não é "govmomi vs REST direto", é "qual(is) subpacote(s) do govmomi usar por domínio". Não existe SDK Go externo melhor (`vsphere-automation-sdk-go` oficial: Beta, sem vSphere/vCenter).
- **VAMI:** parcialmente coberto por `vapi/appliance/{access,logging,networking,shutdown}`; o resto (health/backup/updates/SSH-DCUI/NTP/firewall/contas/serviços) via `vapi/rest` genérico (`Resource(path).Request(method).Do(...)`) — fica pra Phase 4, deliberadamente por último (única fatia não verificável nem no host real nem no vcsim).

### Open decisions
- **VMware Cloud on AWS entra no escopo?** Produto/control-plane distinto do vSphere on-prem (endpoint `console.cloud.vmware.com`, auth CSP token) — sem relação com `govmomi`. Só se o objetivo for gerenciar SDDCs na AWS.

---

## 📁 Active architecture

- **Stack:** Go (module `github.com/cslsoftwares/mcpvmware`, Go 1.25.0), código-fonte próprio em `src/` (`mcpvmware-mcp/`, `mcp/`, `tools/`, `vmware/`). Depende de `github.com/vmware/govmomi` via `go.mod` (cache/proxy Go, não lido do clone local). Clone de referência do govmomi (só leitura) em `referencia/govmomi/`.
- **Superfícies de API mapeadas** (fonte → protocolo → cobertura → prioridade p/ MCP):

| Fonte | Protocolo | Cobertura | Prioridade |
|---|---|---|---|
| `referencia/govmomi/` — `session`, `vim25`+`property`, `find`/`list`/`view`, `object`, `task` | SOAP/vim25 | Conexão/sessão, inventário, VM (power/snapshot/clone/reconfigure), host, datastore, rede, polling de tasks | **Alta** — base natural p/ tools MCP |
| `referencia/govmomi/` — `event`, `performance`, `guest`, `nfc`, `alarm` | SOAP/vim25 | Eventos, métricas de performance, operações in-guest, transferência de arquivo (OVF/disco), alarmes | Média |
| `referencia/govmomi/` — `pbm`, `sms`, `vsan`, `vslm`, `cns`, `sts`, `lookup`, `ssoadmin`, `eam`, `crypto` | SOAP/vim25 | Storage policy/VASA, vSAN, FCD, CSI, SSO/identidade, criptografia de VM | Nicho — só se pedido específico surgir |
| `referencia/govmomi/simulator` + `/vcsim` | SOAP mock | vCenter/ESXi simulado em memória, com fault-injection | **Alta p/ testes** — permite testar o MCP sem vCenter real (usado no harness da Phase 0) |
| `referencia/govmomi/vapi/rest` | REST/VAPI (client genérico) | `Resource(path).Request(method).Do(ctx,&out)` — desembrulha `/rest` e `/api` automaticamente; usado pra VAMI (Phase 4) | **Alta p/ VAMI** |
| `.workspace/vSphere Automation REST Resources.postman_collection.json` (~106 requests, 20 pastas) | REST/VAPI | VM lifecycle+hardware, host, datastore, rede, datacenter, resource pool. Auth: Basic→`vmware-api-session-id` ou SAML bearer | Referência — sobrepõe `object`/`vim25` do govmomi em SOAP |
| `.workspace/...appliance.postman_collection.json` (~130 requests, 21 pastas) | REST/VAMI | **Parcialmente coberto** por `vapi/appliance/` (access, logging, networking, shutdown). Falta: health, backup/restore, updates, SSH/DCUI, NTP, firewall, contas locais, serviços — via `vapi/rest` genérico. Auth: Basic | Phase 4 (fatia inicial: version/health/uptime) |
| `.workspace/...Content Library...json` (21 requests, 5 pastas) | REST | Bibliotecas local/subscribed, deploy de OVF. Coberto por `vapi/library/` + `ovf`/`nfc` do govmomi. Auth: Basic | Média |
| `.workspace/VMware Cloud on AWS APIs...json` (~170-200 requests) | REST | **Produto distinto** (VMC on AWS) — fora de escopo salvo pedido explícito | Fora de escopo |
| `.workspace/vSphere Automation REST Samples...json` (34 requests, 7 pastas) | REST | Cookbook de ordem de chamadas | Referência |
| `.workspace/VMWare Automation...json` (3 requests) | REST | Coleção de teste pessoal | Referência mínima |
| `referencia/vmware-esxi-mcp`, `vmware-vcenter-mcp`, `vmware-vsphere-mcp-server` | MCP servers Python (comunidade) | Nenhum resolve VAMI; confirmam padrões (`confirm:bool`, JSON estruturado, PropertyCollector em lote) — ver "Done" 09/08/2026 | Referência de padrão, não dependência |

- **Patterns do MCPVMWare (`src/`):** `mcpvmware-mcp/main.go` (stdio JSON-RPC, flags+env), `mcp/types.go` (protocolo, genérico), `tools/registry.go`+`system.go` (Registry + tools seed), `vmware/client.go` (wrapper govmomi). `confirm:bool` obrigatório em operações destrutivas (padrão dos 3 MCP servers comunidade, adotado). Respostas sempre JSON estruturado (`marshalJSON`), nunca string Markdown solta.

---

## ⚠️ External blockers (don't block coding)

- ~~Senha `root` do ESXi de teste `10.100.2.54`~~ — **resolvido 2026-08-09 21:57**, teste autenticado completo com sucesso (ver "Done"). Credencial NÃO está persistida em nenhum arquivo do projeto — se precisar reexecutar o teste, pedir novamente ao usuário ou usar as env vars `VCENTER_URL`/`VCENTER_USERNAME`/`VCENTER_PASSWORD`/`VCENTER_INSECURE=1` já suportadas por `src/mcpvmware-mcp`.
- Como é ESXi standalone (não vCenter), features exclusivas de vCenter (clusters/DRS/vMotion entre hosts, tags/content-library via `vapi`) não se aplicam a este alvo de teste — só o inventário local do próprio host (1 VM encontrada: `cac-WN02`, datastore `6a72b614-...`).

---

## 🔧 Useful commands

```bash
# add the most-used commands here so the next session has them ready
```

---

## 📚 References (read IF needed)

- `.wolf/cerebrum.md` — User Preferences + Do-Not-Repeat + Decision Log
- `.wolf/anatomy.md` — token-efficient file index
- `.wolf/buglog.json` — known bugs + fixes

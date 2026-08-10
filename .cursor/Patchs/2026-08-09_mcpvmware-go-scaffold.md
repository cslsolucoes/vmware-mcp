# Patch — Scaffold do módulo Go `mcpvmware` (servidor MCP para VMware)

**Data:** 09/08/2026 · **Autor:** Claude (Sonnet 5), sob pedido do owner · **Tipo:** MINOR (aditivo)

> Continuação de [2026-08-09_kit-golang.md](2026-08-09_kit-golang.md) — aquele
> patch cobre só a família de skills `developer-go-*`. Este cobre o trabalho
> que veio depois na mesma sessão: descoberta do que `src/` realmente é, e a
> materialização do módulo Go do próprio MCPVMWare.

## Contexto

Ao retomar o fluxo `/iniciar` (pendente desde o patch anterior) e pedir o
nome do projeto, o usuário rodou `ls d:\MCPVMWare\src` e revelou que **`src/`
já continha código real** — não uma pasta vazia para bootstrap. Investigação
confirmou: `src/` é um clone limpo (working tree limpa, remote
`github.com/vmware/govmomi.git`, branch `main` sincronizada) do **govmomi**,
o SDK Go oficial da VMware para vSphere/vCenter — não código próprio do
usuário.

Perguntado sobre o propósito (`AskUserQuestion`), o usuário confirmou:
**construir um servidor MCP (Model Context Protocol) para VMware, usando
govmomi como dependência via `go.mod`** — nunca modificando/fazendo fork do
código em `src/`. Em seguida pediu para analisar o fonte e já parametrizar o
workspace.

Descoberta adicional em `.workspace/`: já existia `context.json` e uma rule
local (`MCPVMWare-local-arquivos_V1.0.0.mdc`) de sessão anterior, incluindo
uma nota apontando `D:\MCPNas\truenas-mcp` como **arquitetura de referência**
para o servidor MCP deste projeto (memória `reference_truenas_mcp_golang`) —
é o mesmo servidor Go que expõe os tools `mcp__mcpTrueNas__*` já disponíveis
nesta sessão.

## Pesquisa feita antes de escrever código

- **Mapeamento de superfície do govmomi** (agente Explore, `src/`): padrão de
  conexão/autenticação (`vim25.NewClient` → `session.Manager` →
  `session/cache.Session`/`session/keepalive` para sessões de longa duração),
  ~10 entidades de domínio principais (`VirtualMachine`, `Datastore`,
  `HostSystem`, `ResourcePool`, `Folder`, `ClusterComputeResource`,
  `Network`/`DVS`, `Datacenter`, `StoragePod`/`VirtualApp`, `Task`), o pacote
  `find.Finder` para localizar objetos por caminho, ~20 categorias de comando
  `govc` como candidatas a tools MCP, e convenções de erro/contexto/timeout
  (`context.Context` em toda chamada, mutações retornam `*object.Task` a
  aguardar, pacote `fault` para inspecionar `types.BaseMethodFault`).
- **Leitura direta da arquitetura de referência** (`D:\MCPNas\truenas-mcp`):
  `cmd/truenas-mcp/main.go` (flags, client, task manager, stdio handler
  JSON-RPC), `mcp/types.go` (tipos do protocolo, genéricos), `tools/registry.go`
  (Registry com `ListTools`/`CallTool`), `truenas/client.go` (wrapper de
  conexão), `tools/auth.go` (padrão de arquivo por namespace de API).
- **Leitura direta do `src/client.go`** (govmomi): confirmado
  `govmomi.NewClient(ctx, *url.URL, insecure bool) (*Client, error)` como
  construtor de topo, e `find.NewFinder`/`Finder.VirtualMachineList` como
  API de busca — usados tal como documentados, sem adivinhar assinaturas.
- Tag mais recente do clone `src/` confirmada via `git tag`: `v0.55.1`
  (usada para fixar a versão em `go.mod`).

## O que foi criado

### Módulo Go raiz (`d:\MCPVMWare\`, separado de `src/`)

| Arquivo | Conteúdo |
|---|---|
| `go.mod` | `module github.com/cslsoftwares/mcpvmware` · `go 1.25.0` · `require github.com/vmware/govmomi v0.55.1` (+ `google/uuid` indireto) |
| `go.sum` | gerado por `go mod tidy` |
| `cmd/mcpvmware-mcp/main.go` | Entrypoint do servidor MCP — flags `--vcenter-url/--username/--password/--insecure/--debug/--version` (+ env vars `VCENTER_URL`/`VCENTER_USERNAME`/`VCENTER_PASSWORD`/`VCENTER_INSECURE`/`MCPVMWARE_DEBUG`), `StdioHandler` (JSON-RPC 2.0 sobre stdio: `initialize`, `tools/list`, `tools/call`) — mirror direto de `cmd/truenas-mcp/main.go` |
| `mcp/types.go` | Tipos JSON-RPC 2.0 / MCP (`Request`, `Response`, `Tool`, `ToolsListResult`, `ToolCallParams`, `ToolCallResult`, `ToolRegistry`) — genérico, quase idêntico ao original (é o protocolo, não lógica TrueNAS) |
| `vmware/client.go` | `vmware.Client` (embute `*govmomi.Client` + `*find.Finder`), `vmware.NewClient(ctx, Config)` — monta a SDK URL, autentica via `govmomi.NewClient`, resolve datacenter default no Finder |
| `tools/registry.go` | `Registry` (`ListTools`/`CallTool`), `Tool{Definition, Handler}`, helper `register()` para arquivos de tools por domínio |
| `tools/system.go` | 2 tools seed: `vmware_about` (ServiceContent.About) e `vmware_list_vms` (via `Finder.VirtualMachineList`) |
| `README.md` (raiz) | Descrição do módulo, estrutura, build/run, status, follow-up |

### Módulo path — decisão tomada sem confirmação explícita

`github.com/cslsoftwares/mcpvmware` foi escolhido como *default razoável*
(domínio da empresa do usuário, `cslsoftwares.com.br`), **não confirmado
explicitamente** — documentado em 3 lugares (`.workspace/context.json`,
rule local, `README.md`) como ajustável via `go mod edit -module` se o
usuário tiver outro org/repo real em mente.

## Verificação (rodada e lida, não assumida)

```
go mod tidy   → baixou github.com/vmware/govmomi v0.55.1 + google/uuid v1.6.0 (indireto)
go build ./...  → EXIT=0
go vet ./...    → EXIT=0
gofmt -l cmd mcp tools vmware  → sem saída (nenhum arquivo fora do padrão)
```

`gofmt -l .` sem escopo inicialmente pegou `src/` inteiro (código de
terceiros, não nosso) — corrigido para escopar só os diretórios novos antes
de tirar qualquer conclusão.

## Atualizado

- `.workspace/context.json` — novo bloco `goModule` (path, versão Go,
  toolchain instalada, propósito, arquitetura de referência, layout, status,
  follow-up conhecido).
- `.workspace/rules/MCPVMWare-local-arquivos_V1.0.0.mdc` →
  **renomeado** para `_V1.2.0.mdc` (sufixo == FileVersion, conforme a
  política de versionamento do pack) — nova seção "Módulo Go do projecto",
  frontmatter `version` corrigido de `1.0.0` para `1.2.0`.

## Follow-up conhecido (não bloqueante, registrado em 3 lugares)

Trocar `vmware.NewClient` (chama `govmomi.NewClient` diretamente) por
`session/cache.Session` + `session/keepalive` (pacotes do próprio govmomi)
antes de operar como servidor de longa duração — evita expiração silenciosa
de sessão. Achado do mapeamento de API do govmomi, não implementado nesta
rodada por estar fora do escopo de "parametrizar o workspace".

## Fora de escopo (não incluído neste patch)

- Implementação dos tools reais por domínio (`vm`, `host`, `datastore`,
  `cluster`, `network`, `tags`, `content library`, ...) — só os 2 tools seed
  existem. O mapeamento de ~20 categorias `govc` já coletado nesta sessão
  serve de roteiro para isso.
- Adoção de `session/cache.Session`/`session/keepalive` (ver follow-up acima).
- Registro deste módulo Go no fluxo `/iniciar`/`bootstrap-build-config.ps1`
  (esses scripts continuam sem suporte a Go — não tocados nesta rodada).

---

## Versão interna (arquivo)

| Campo | Valor |
|-------|-------|
| **FileVersion** | 1.0.0 |
| **Política** | `.cursor/VERSION.md` |

## Changelog (este arquivo)

- 1.0.0 (09/08/2026): Criação do patch — scaffold do módulo Go `mcpvmware`
  (go.mod + cmd/mcp/tools/vmware), descoberta de que `src/` é o govmomi
  vendorizado, arquitetura de referência `truenas-mcp` aplicada, workspace
  parametrizado (`context.json` + rule local renomeada).

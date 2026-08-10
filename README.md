# MCPVMWare

Servidor **MCP** (Model Context Protocol) para VMware — expõe operações
vSphere/vCenter como *tools* MCP, usando [govmomi](https://github.com/vmware/govmomi)
como cliente da API (dependência normal via `go.mod`, nunca fork).

Arquitetura espelhada do servidor MCP TrueNAS de referência
(`D:\MCPNas\truenas-mcp`): `<binário>/main.go` (entrypoint stdio) →
`mcp/` (tipos JSON-RPC/MCP) → `tools/` (registro de tools por domínio) →
`vmware/` (wrapper sobre o cliente govmomi).

## Estrutura do repositório

```
src/                          módulo Go do MCPVMWare (module github.com/cslsoftwares/mcpvmware)
  go.mod                      go 1.25.0, require github.com/vmware/govmomi
  mcpvmware-mcp/main.go       entrypoint do servidor MCP (stdio)
  mcp/types.go                tipos JSON-RPC 2.0 / MCP
  vmware/client.go            wrapper sobre govmomi.NewClient + find.Finder
  tools/registry.go           registro de tools (ListTools/CallTool)
  tools/system.go             tools seed: vmware_about, vmware_list_vms
referencia/                   material de terceiros, só leitura (nunca editado/fork)
  govmomi/                    clone limpo do SDK oficial (para ler fonte/exemplos)
  vsphere-automation-sdk-go/  SDK REST/VAPI oficial (Beta, ainda sem vSphere/vCenter)
  vsphere-automation-sdk-java/
  vsphere-automation-sdk-python/
  vsphere-automation-sdk-rest/  (descontinuado — usar as coleções Postman em .workspace/)
  vmware-esxi-mcp/            MCP server comunidade (Python) — maioria mock, ver ressalva
  vmware-vcenter-mcp/         MCP server comunidade (Python) — SOAP/pyVmomi, sem VAMI
  vmware-vsphere-mcp-server/  MCP server comunidade (Python) — híbrido REST+pyVmomi
  ssh-mcp-server/             MCP server SSH genérico (não específico de VMware)
.workspace/                   contexto local do clone (context.json, rules) — não propagado pelo pack
.cursor/                      pack de skills/agents/templates (inclui developer-go-* — kit GoLang)
.wolf/                        protocolo OpenWolf (STATUS.md é a fonte de verdade de progresso)
```

## Build

```bash
cd src
go build ./...
go vet ./...
gofmt -l mcp tools vmware mcpvmware-mcp
```

## Executar

```bash
cd src
go run ./mcpvmware-mcp \
  --vcenter-url vcenter.local \
  --username administrator@vsphere.local \
  --password '...'
```

Ou via variáveis de ambiente: `VCENTER_URL`, `VCENTER_USERNAME`,
`VCENTER_PASSWORD`, `VCENTER_INSECURE=1` (pula verificação TLS — só para
testes), `MCPVMWARE_DEBUG=1` (log verboso).

## Status

Scaffold validado ponta a ponta contra hardware real (ESXi 7.0.3 standalone)
com 2 tools seed. Plano detalhado dos próximos domínios (VM lifecycle,
snapshots, inventário, host ops, VAMI) em
`D:\Users\claiton.linhares\.claude\plans\streamed-imagining-lightning.md`.
Ver `.wolf/STATUS.md` para o estado atualizado sessão a sessão — é a fonte
de verdade, não este README.

**Follow-up conhecido:** trocar `vmware.NewClient` por
`session/cache.Session` + `session/keepalive` (pacotes do próprio govmomi)
antes de operar como servidor de longa duração, para evitar expiração
silenciosa de sessão.

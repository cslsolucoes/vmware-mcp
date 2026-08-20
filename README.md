# MCPVMWare

Servidor **MCP** (Model Context Protocol) para VMware — expõe **1030 tools**
cobrindo os 3 produtos da linha VMware/Broadcom (vSphere/vCenter/ESXi,
VMware Workstation Pro, e VMware Cloud on AWS) via um único binário,
selecionado por modo de conexão.

Arquitetura espelhada do servidor MCP TrueNAS de referência
(`D:\MCPNas\truenas-mcp`): `<binário>/main.go` (entrypoint stdio) →
`mcp/` (tipos JSON-RPC/MCP) → `tools/` (registro de tools por domínio) →
um cliente por produto (`vmware/`, `workstation/`, `cloudaws/`).

## Cobertura — 100% da superfície de gerência (1030 tools)

| Produto | Tools | Cliente | Técnica |
| --- | ---: | --- | --- |
| vSphere / vCenter / ESXi | **907** | `vmware/` (govmomi) | geração de código AST sobre `object/`+`vapi/*` + parse VAMI (Postman) + **9 ondas (2026-08-19→20) preenchendo os métodos vim25 SOAP crus sem wrapper `object.*`** (iSCSI, FT, Guest Ops, Crypto/KMIP, DVS, vSAN, First-Class Disks, HostProfile, etc.) |
| VMware Cloud on AWS (VMC) | **95** | `cloudaws/` (HTTP+CSP token-exchange) | hand-written a partir da collection Postman oficial |
| VMware Workstation Pro (`vmrest`) | **28** | `workstation/` (HTTP+Basic Auth) | hand-written a partir da spec Swagger oficial |
| **Total (distintas)** | **1030** | | |

As 907 tools de vSphere dividem-se por modo de conexão em **521 vcenter-only**
(cluster/DRS, tags, content library, Tanzu/namespaces, VAMI, crypto/KMIP…) e
**386 vsphere-general** (funcionam também num ESXi standalone). **600** das 1030
são destrutivas (Tier 1/2) e passam pelo gate `--allow-destructive` + `confirm:true`.

> **Referência completa de cada ferramenta** (descrição + sintaxe + tabela de
> parâmetros das 1030 tools): **[TOOLS.md](TOOLS.md)**. É gerado a partir do
> próprio catálogo do servidor (`tools/list`) por `src/cmd/dumptools` — a mesma
> definição que o cliente MCP recebe (regenerar após alterar tools).

### Ferramentas por área

| Área | Tools | Produto |
| --- | ---: | --- |
| Host ESXi (rede, storage, vSAN, iSCSI, perfil, certificados, serviços) | 158 | vSphere |
| VAMI — vCenter Server Appliance | 135 | vSphere |
| VMware Cloud on AWS (VMC) | 95 | VMC |
| Máquinas virtuais (ciclo de vida, devices, snapshots, clone/migração) | 72 | vSphere |
| Content Library | 68 | vSphere |
| Supervisor / Namespaces (Tanzu) | 43 | vSphere |
| First-Class Disks (vStorageObject) | 40 | vSphere |
| Distributed Virtual Switch (DVS) | 34 | vSphere |
| Criptografia / KMS (KMIP) | 30 | vSphere |
| VMware Workstation Pro | 28 | Workstation |
| Tags & Categorias | 27 | vSphere |
| Guest Operations (SO convidado) | 26 | vSphere |
| Cluster (módulos / DRS / HA) | 23 | vSphere |
| Datastore | 20 | vSphere |
| ESX Settings (cluster VMs) | 16 | vSphere |
| Autorização (papéis / permissões) | 14 | vSphere |
| Licenças | 13 | vSphere |
| Inventário — folders | 11 | vSphere |
| Discos virtuais (VDDK) | 11 | vSphere |
| Alarmes · Customization Spec · Health Update · vCenter OVF/Template | 10 cada | vSphere |
| Performance · Search Index · Storage DRS | 9 cada | vSphere |
| IO Filter · IP Pool | 8 cada | vSphere |
| Datacenter · Inventário listagens · Resource Pool/vApp · Scheduled Tasks | 7 cada | vSphere |
| Custom Fields · Extension Manager · vApp | 6 cada | vSphere |
| Compute Resource · Eventos · Tasks | 5 cada | vSphere |
| CIS Tasks · Diagnóstico/Logs · Environment Browser · Datastore-arquivos | 4 cada | vSphere |
| Multi-tenant | 3 | vSphere |
| Sistema (about) · Autenticação · Rede opaca (NSX) | 1 cada | vSphere |

## Estrutura do repositório

```
src/                          módulo Go do MCPVMWare (module github.com/cslsoftwares/mcpvmware)
  go.mod                      go 1.25, require github.com/vmware/govmomi (única dependência externa)
  mcpvmware-mcp/main.go       entrypoint do servidor MCP (stdio) — resolve o modo de conexão e o cliente
  cmd/dumptools/main.go       dump offline do catálogo (name+description+inputSchema) → gera TOOLS.md
  mcp/types.go                tipos JSON-RPC 2.0 / MCP
  vmware/client.go            wrapper sobre govmomi (SOAP vim25 + REST/VAPI + keepalive)
  workstation/client.go       cliente HTTP+Basic Auth para o serviço vmrest do Workstation Pro
  cloudaws/client.go          cliente HTTP com auth CSP token-exchange para VMware Cloud on AWS
  tools/registry.go           registro de tools (ListTools/CallTool), filtragem por modo de conexão
  tools/destructive.go        gate + confirm + auditoria para tools Tier1/2 (destrutivas), por produto
  tools/*.go                  ~90 ficheiros de tools por domínio (hand-written + gerados)
  gen/                        gerador AST (classifica métodos SOAP/REST por tier, produz classification.json)
referencia/                   material de terceiros, só leitura (nunca editado/fork)
.workspace/                   plans/reports do projeto (context.json, rules) — não propagado pelo pack
.cursor/                      pack de skills/agents/templates (inclui developer-go-* — kit GoLang)
.wolf/                        protocolo OpenWolf (STATUS.md é a fonte de verdade de progresso)
```

## Modos de conexão

Um binário, cinco modos mutuamente exclusivos — exatamente uma flag de
conexão por execução:

| Flag | Conecta a | Tools expostas |
| --- | --- | --- |
| `--vcenter-url` | vCenter Server (VCSA) | vcenter-only + vsphere-general (**907**) |
| `--vmware-url` | ESXi standalone | só vsphere-general (**386**) |
| `--vmware-all-url` | vCenter ou ESXi | mesmo conjunto de `--vcenter-url` (**907**; Workstation/CloudAWS não incluídos — ver nota) |
| `--workstation-url` | `vmrest` local (Workstation Pro) | as **28** tools de Workstation |
| `--cloud-aws-url` | VMware Cloud on AWS | as **95** tools de VMC |

`--username`/`--password` autenticam vCenter/ESXi e `vmrest` (Basic Auth).
VMC usa `--refresh-token`/`VMC_REFRESH_TOKEN` em vez de usuário/senha (CSP
token-exchange — gerar o refresh token manualmente na consola web da VMC).

**Nota:** `--vmware-all-url` não inclui Workstation/CloudAWS — expor as 1030
num só processo exige múltiplos clientes vivos simultâneos com credenciais
distintas (um modo "tudo"). Está em implementação — use as flags por produto
separadamente enquanto isso.

## Ações destrutivas (Tier 1/2)

Toda tool classificada como destrutiva (Tier1 = irreversível, Tier2 =
disruptiva-reversível) — **600 das 1030** — passa por 3 camadas antes de executar:

1. Gate do servidor: `--allow-destructive` / `VCENTER_ALLOW_DESTRUCTIVE`
   (desligado por padrão — nega antes de qualquer round-trip ao backend).
2. `confirm: true` estrito por chamada (booleano exato, não string truthy).
3. Auditoria opcional: `--audit-log-path` / `MCPVMWARE_AUDIT_LOG` (JSON
   Lines, uma linha por chamada, permitida ou negada).

## Build

```bash
cd src
go build ./...
go vet ./...
gofmt -l .
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

### Regenerar TOOLS.md

```bash
cd src
go run ./cmd/dumptools > catalog.json   # dump do catálogo (por modo)
# depois: scripts/gen_tools_md.py transforma catalog.json → TOOLS.md
```

## Status

**1030 tools** cobrindo vSphere/vCenter/ESXi (907), VMware Cloud on AWS (95) e
VMware Workstation Pro (28) — plano de cobertura completa (Fases 0-10) + as 9
ondas de 2026-08-19→20 que preencheram os métodos vim25 SOAP sem wrapper
`object.*`. Binário de produção implantado em `D:\ServidorDataCenter\`. Ver
`.wolf/STATUS.md` para o estado atualizado sessão a sessão — é a fonte de
verdade, não este README.

**Pendências conhecidas:**
- Modo "tudo" (`--all-url`) expondo as 1030 num só processo (arquitetura de
  múltiplos clientes simultâneos) — em implementação.
- Domínios sem simulador/ambiente de teste real disponível (VAMI legacy,
  `vmrest`, VMC on AWS) — verificados só via fixture `httptest`, nunca
  ponta-a-ponta contra um serviço real.

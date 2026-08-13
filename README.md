# MCPVMWare

Servidor **MCP** (Model Context Protocol) para VMware — expõe **734 tools**
cobrindo os 3 produtos da linha VMware/Broadcom (vSphere/vCenter/ESXi,
VMware Workstation Pro, e VMware Cloud on AWS) via um único binário,
selecionado por modo de conexão.

Arquitetura espelhada do servidor MCP TrueNAS de referência
(`D:\MCPNas\truenas-mcp`): `<binário>/main.go` (entrypoint stdio) →
`mcp/` (tipos JSON-RPC/MCP) → `tools/` (registro de tools por domínio) →
um cliente por produto (`vmware/`, `workstation/`, `cloudaws/`).

## Cobertura (100% — plano concluído 12/08/2026)

| Produto | Tools | Cliente | Técnica |
| --- | --- | --- | --- |
| vSphere / vCenter / ESXi | 611 | `vmware/` (govmomi) | geração de código: AST sobre `object/`+`vapi/*/*.go` (Fases 1-8a) + parse da collection Postman de VAMI legacy (Fase 8b) |
| VMware Workstation Pro (`vmrest`) | 28 | `workstation/` (HTTP+Basic Auth) | hand-written a partir da spec Swagger oficial (Fase 9) |
| VMware Cloud on AWS (VMC) | 95 | `cloudaws/` (HTTP+CSP token-exchange) | hand-written a partir da collection Postman oficial (Fase 10) |

Plano completo e histórico de execução fase-a-fase:
`.workspace/plans/MCPVMWare2026-08-10-175300-plano-cobertura-completa-api-codegen.plan.md`
(`status: concluído`). Reports formais por fase em `.workspace/reports/`.

## Estrutura do repositório

```
src/                          módulo Go do MCPVMWare (module github.com/cslsoftwares/mcpvmware)
  go.mod                      go 1.25, require github.com/vmware/govmomi (única dependência externa)
  mcpvmware-mcp/main.go       entrypoint do servidor MCP (stdio) — resolve o modo de conexão e o cliente
  mcp/types.go                tipos JSON-RPC 2.0 / MCP
  vmware/client.go            wrapper sobre govmomi (SOAP vim25 + REST/VAPI + keepalive)
  workstation/client.go       cliente HTTP+Basic Auth para o serviço vmrest do Workstation Pro
  cloudaws/client.go          cliente HTTP com auth CSP token-exchange para VMware Cloud on AWS
  tools/registry.go           registro de tools (ListTools/CallTool), filtragem por modo de conexão
  tools/destructive.go        gate + confirm + auditoria para tools Tier1/2 (destrutivas), por produto
  tools/*.go                  ~60 ficheiros de tools por domínio (hand-written + gerados, ver plano)
  gen/                        gerador AST (classifica métodos SOAP/REST por tier, produz classification.json)
referencia/                   material de terceiros, só leitura (nunca editado/fork) — submodules Git
  govmomi/                    clone limpo do SDK oficial (para ler fonte/exemplos)
.workspace/                   plans/reports do projeto (context.json, rules) — não propagado pelo pack
.cursor/                      pack de skills/agents/templates (inclui developer-go-* — kit GoLang)
.wolf/                        protocolo OpenWolf (STATUS.md é a fonte de verdade de progresso)
```

## Modos de conexão

Um binário, cinco modos mutuamente exclusivos — exatamente uma flag de
conexão por execução:

| Flag | Conecta a | Tools expostas |
| --- | --- | --- |
| `--vcenter-url` | vCenter Server (VCSA) | vcenter-only + vsphere-general |
| `--vmware-url` | ESXi standalone | só vsphere-general |
| `--vmware-all-url` | vCenter ou ESXi | mesmo conjunto de `--vcenter-url` (Workstation/CloudAWS não incluídos — ver nota abaixo) |
| `--workstation-url` | `vmrest` local (Workstation Pro) | as 28 tools de Workstation |
| `--cloud-aws-url` | VMware Cloud on AWS | as 95 tools de VMC |

`--username`/`--password` autenticam vCenter/ESXi e `vmrest` (Basic Auth).
VMC usa `--refresh-token`/`VMC_REFRESH_TOKEN` em vez de usuário/senha (CSP
token-exchange — gerar o refresh token manualmente na consola web da VMC).

**Nota:** `--vmware-all-url` não inclui Workstation/CloudAWS — isso exigiria
o servidor manter múltiplos clientes vivos simultâneos com credenciais
distintas, uma decisão de arquitetura ainda em aberto (ver plano
§"Critérios de conclusão").

## Ações destrutivas (Tier 1/2)

Toda tool classificada como destrutiva (Tier1 = irreversível, Tier2 =
disruptiva-reversível) passa por 3 camadas antes de executar:

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

## Status

Plano de cobertura completa **concluído** (Fases 0-10, 12/08/2026) — 734
tools registadas, cobrindo vSphere/vCenter/ESXi, VMware Workstation Pro e
VMware Cloud on AWS. Ver `.wolf/STATUS.md` para o estado atualizado
sessão a sessão — é a fonte de verdade, não este README.

**Pendências conhecidas** (não bloqueiam a cobertura, ver o plano para
detalhe completo):
- `--vmware-all-url` incluir tools de Workstation/CloudAWS (arquitetura de
  múltiplos clientes simultâneos, ainda não decidida).
- Domínios sem simulador/ambiente de teste real disponível (VAMI legacy,
  `vmrest`, VMC on AWS) — verificados só via fixture `httptest`, nunca
  ponta-a-ponta contra um serviço real.

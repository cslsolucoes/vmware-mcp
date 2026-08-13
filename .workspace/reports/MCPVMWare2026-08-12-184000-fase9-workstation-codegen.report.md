---
title: "Fase 9 do plano de cobertura completa — VMware Workstation Pro (vmrest), arquitectura de cliente duplo"
created: 2026-08-12
updated: 2026-08-12
type: report
status: final
locale: pt-PT
overview: "28 tools novas geradas para VMware Workstation Pro (639 tools no total), arquitectura de cliente duplo construída de raiz sem alterar nenhuma das 611 tools existentes."
---

## Resumo

A Fase 9 do plano de cobertura completa (`.workspace/plans/MCPVMWare2026-08-10-175300-plano-cobertura-completa-api-codegen.plan.md`) gerou 28 tools novas para VMware Workstation Pro (`vmrest`), um produto distinto de vSphere/ESXi sem dependência de `govmomi`, levando o total de tools do servidor MCP de 611 para **639**. Diferente de toda fase anterior, esta exigiu construir uma arquitectura de cliente duplo no `Registry` — resolvida pelo orquestrador antes de qualquer delegação, sem alterar nenhum dos ~30 ficheiros de teste existentes.

## Contexto

Trabalho executado em 12/08/2026, como continuação directa da Fase 8b (VAMI via parse de Postman, 116 tools). Ambiente: Go 1.25 (`github.com/cslsoftwares/mcpvmware`). Sem simulador equivalente ao vcsim para `vmrest` — testado via fixture `httptest`, e via 2 smokes reais do binário compilado (regressão vSphere + Workstation novo contra fixture).

## Achados

### Achado arquitectural crítico — resolvido antes de delegar

`Tool.Handler` e `Registry.client` estavam hardcoded a `*vmware.Client` desde a Fase 0 — toda tool das Fases 0-8 (611 no total) assume esse tipo concreto. Workstation Pro precisa de um cliente HTTP inteiramente diferente (`*workstation.Client`, sem `govmomi`). Em vez de alargar `client` para `interface{}` (arriscaria as 611 tools existentes com type-assertions novas em código que já funciona), foi adicionado um **segundo campo** `Tool.WSHandler` + `Registry.wsClient` + `RegistryOptions.WorkstationClient` (campo opcional, não parâmetro posicional novo em `NewRegistry`) — zero ficheiros de teste existentes precisaram de mudar; `RegistryOptions{}` continua a significar exactamente o que significava antes da Fase 9. `CallTool` despacha por qual campo (`Handler`/`WSHandler`) está preenchido. `registerWorkstation`/`registerDestructiveWorkstation` espelham `register`/`registerDestructive` exactamente (mesmas 3 camadas de protecção — gate/confirm/auditoria).

### Cliente `workstation.Client` construído de raiz

`src/workstation/client.go` — HTTP simples + Basic Auth, media type `application/vnd.vmware.vmw.rest-v1+json` (confirmado em sessão anterior contra um vmrest 1.3.1 real, ver `.wolf/cerebrum.md`), `ErrorModel` com chaves capitalizadas (`Code`/`Message`, achado empírico anterior, não a spec). Achado durante a construção: `PUT /vms/{id}/power` tem um corpo que é uma STRING CRUA (`"on"`/`"off"`/etc.), não JSON — confirmado lendo a definição real do request na collection Postman — exigiu um método `DoRawBody` separado de `Do` (que sempre serializa JSON). Verificado com 6 testes próprios (`client_test.go`) contra fixture `httptest` ANTES de delegar qualquer tool.

### 2 grupos paralelos

1. **workstation-vm (11):** VM Management + VM Power Management. Corrigiu 3 divergências reais da spec "opcional" para "obrigatório" no schema (`config_param_set` exige `name`+`value`; `register` exige `path`; `update` exige pelo menos um de `processors`/`memory` — a spec como está tornaria a chamada um no-op sem sentido). Provou por teste dedicado que um `operation` de power fora do enum é rejeitado ANTES de tocar o servidor, e que o corpo enviado é a string crua exacta.
2. **workstation-network (17, não 18):** Shared Folders + Network Adapters + Host Networks. Achou que "Host Networks Management" só tem 7 rotas reais (não 8, estimativa do orquestrador errada) — confirmado por parse Python da collection, não assumido.

### Achado de design resolvido durante a integração

`connectionModeAllows(ConnectionModeAll, ...)` já incluía `modeWorkstation` desde 10/08 (escrito especulativamente antes de qualquer tool existir), mas `--vmware-all-url` só constrói UM cliente (`*vmware.Client`) — deixar isso registar tools de Workstation sob "all" mode anunciaria 28 tools que falhariam sempre com "wrong connection type". Corrigido: `ConnectionModeAll` fica deliberadamente vSphere-only (mesmo conjunto que `ConnectionModeVCenter`) até o "detalhe de config em aberto" já sinalizado desde 10/08 (2 clientes vivos simultâneos) ser decidido pelo usuário — `--workstation-url` continua sendo a única via para as 28 tools de Workstation por agora.

## Evidências

```
$ cd src && gofmt -l . && go build ./... && go vet ./...
(sem output — limpo)
$ go clean -testcache && go test ./... -count=1 -timeout 300s
ok  	github.com/cslsoftwares/mcpvmware/tools	78.486s
ok  	github.com/cslsoftwares/mcpvmware/vmware	4.734s
ok  	github.com/cslsoftwares/mcpvmware/workstation	0.819s
$ go test ./... -count=1 -timeout 300s   # 2ª corrida, sem flake
ok  	github.com/cslsoftwares/mcpvmware/tools	78.157s
ok  	github.com/cslsoftwares/mcpvmware/vmware	4.452s
ok  	github.com/cslsoftwares/mcpvmware/workstation	0.798s
```

**2 smokes reais do binário compilado**, via subprocess Go throwaway:

1. Regressão vSphere (`--vcenter-url` contra `simulator.VPX()`): 611 tools confirmadas (nenhuma mudança), todas as chamadas anteriores continuam OK.
2. Workstation novo (`--workstation-url` contra uma fixture `httptest` simulando `vmrest`): **28 tools registadas, zero leak de tools vSphere**, `vmware_workstation_vm_list`/`vmnet_list` devolveram dados reais da fixture, `vmware_workstation_vm_power_set` correctamente negado pelo gate de 3 camadas sem `--allow-destructive` (prova que `registerDestructiveWorkstation` funciona de verdade).

Narrativa completa em `.workspace/plans/MCPVMWare2026-08-10-175300-plano-cobertura-completa-api-codegen.plan.md` §"Fase 9 executada". Aprendizados reutilizáveis (padrão de 2º cliente sem tocar tools existentes; quirk de corpo cru do vmrest; reconferir decisões especulativas antigas) em `.wolf/cerebrum.md`.

Ficheiros novos: `src/workstation/client.go(+test)`, `src/tools/workstation_vm.go(+test)`, `src/tools/workstation_network.go(+test)`. Ficheiros alterados: `src/tools/registry.go` (arquitectura de cliente duplo, wiring), `src/tools/destructive.go` (`registerDestructiveWorkstation`), `src/mcpvmware-mcp/main.go` (flags `--workstation-url` habilitado, branch de construção de cliente), `src/tools/mode_test.go` (catálogo `workstationTools`, correcção de `TestMode_Unrestricted`).

**Sem commits** — não pedido pelo usuário nesta rodada.

## Recomendações / próximos passos

1. Usuário autorizou ("ok, finalizar 100%") avançar directo para a Fase 10 (VMware Cloud on AWS) sem pausar para aprovação intermédia — **a última fase do plano**.
2. Pendências documentadas e não bloqueantes (numeradas):
   1. **Decisão em aberto, não resolvida por esta fase**: `--vmware-all-url` incluir tools de Workstation exigiria `main.go` segurar 2 clientes vivos simultâneos (URLs/credenciais distintas) — sinalizado desde 10/08, continua pendente de decisão explícita do usuário.
   2. Sem simulador equivalente ao vcsim para `vmrest` — testado só via fixture `httptest`, nunca contra um serviço `vmrest` real nesta sessão (smoke ao vivo anterior, 10/08, foi feito manualmente fora do harness automatizado).
   3. Pré-requisito operacional (não de código): `vmrest` só responde se estiver a correr localmente na MESMA máquina que o `mcpvmware-mcp.exe` — documentado em cada tool, mas é uma limitação real do produto, não contornável.
3. O padrão de cliente duplo (`Tool.WSHandler`/`RegistryOptions.WorkstationClient`) estabelecido nesta fase fica disponível como precedente directo para a Fase 10 (VMware Cloud on AWS, também precisa de um cliente HTTP novo com auth CSP token).

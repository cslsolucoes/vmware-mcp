---
title: "Fase 5 do plano de cobertura completa — codegen do domínio Rede"
created: 2026-08-11
updated: 2026-08-11
type: report
status: final
locale: pt-PT
overview: "7 tools novas geradas para o domínio Rede (196 tools no total), escritas diretamente sem subagents, com 1 bug de teste (não de produção) achado e corrigido."
---

## Resumo

A Fase 5 do plano de cobertura completa (`.workspace/plans/MCPVMWare2026-08-10-175300-plano-cobertura-completa-api-codegen.plan.md`) gerou 7 tools novas para o domínio Rede (`DistributedVirtualSwitch`, `DistributedVirtualPortgroup`, `OpaqueNetwork`), levando o total de tools do servidor MCP de 189 para **196**. Domínio pequeno comparado às Fases 2-4 (48-70 tools cada) — escrito diretamente sem delegar a subagents, decisão explícita de custo/benefício. Um bug real foi achado durante os testes, mas desta vez no próprio processo de teste (não no código de produção): uma tentativa de criar uma fixture de DVS colidiu com uma que o simulador já cria por defeito.

## Contexto

Trabalho executado em 11/08/2026, a pedido do usuário ("ok, prossiga"), como continuação directa da Fase 4 (domínio Storage/Datastore, 43 tools). Ambiente: Go 1.25 (`github.com/cslsoftwares/mcpvmware`), testado contra `referencia/govmomi/simulator` (vcsim, in-process).

## Achados

### Pré-geração (revisão dos 32 métodos candidatos)

| Achado | Ação |
| --- | --- |
| `HostNetworkSystem` (21 métodos) já estava coberto pela Fase 3 (`generated_host_network.go`) — receiver diferente (redes host-scoped de ESXi, não o switch distribuído) | Excluído por já feito, não é uma decisão nova |
| 4 métodos `EthernetCardBackingInfo()` (um em cada de `Network`/`OpaqueNetwork`/`DistributedVirtualSwitch`/`DistributedVirtualPortgroup`) sem tool consumidor (nenhum tool cria adaptadores de rede em VMs ainda) | Excluídos |
| `OpaqueNetwork.Summary()` classificado tier2 pelo gerador da Fase 0, mas é leitura pura de propriedade | Corrigido para sem-tier |
| Domínio pequeno (7 tools restantes) | Escrito diretamente, sem subagents — decisão de custo/benefício, mesma já tomada para o grupo "snapshot" da Fase 2 |

### Achados durante a escrita/testes

1. **Bug no próprio processo de teste (não no código gerado):** o teste `TestNetworkTools_DVSLifecycle` tentava criar uma DVS chamada "DVS0" via `Folder.CreateDVS` como fixture, e falhava com `*types.InvalidArgument{InvalidProperty:"name"}`. Investigação (com um teste de diagnóstico dedicado, já que o erro genérico não indicava a causa) confirmou que `simulator.VPX()` já cria por defeito 1 DVS chamada "DVS0" por datacenter — efeito colateral de `Model.Portgroup: 1` (uma portgroup distribuída precisa de uma DVS-mãe). Corrigido removendo a criação de fixture desnecessária; os testes passam contra a DVS já existente.
2. **Achado estrutural, corrigido antes de integrar:** as 7 tools tinham sido escritas numa única função `registerNetworkTools`, mas 6 são `vcenter-only` (DVS/DVPG) e 1 é `vsphere-general` (OpaqueNetwork) — o mecanismo `withClass` só aplica 1 modo por chamada de registo. Dividido em 2 funções (`registerNetworkTools` + `registerNetworkVSphereGeneralTools`), mesmo padrão já usado na Fase 2 para os 3 checkers de VM.

## Evidências

```
$ cd src && gofmt -l . && go build ./... && go vet ./...
(sem output — limpo)
$ go clean -testcache && go test ./... -count=1 -timeout 240s
ok  	github.com/cslsoftwares/mcpvmware/tools	36.64s
ok  	github.com/cslsoftwares/mcpvmware/vmware	4.78s
$ go test ./... -count=1 -timeout 240s   # 2ª corrida, sem flake
ok  	github.com/cslsoftwares/mcpvmware/tools	43.23s
ok  	github.com/cslsoftwares/mcpvmware/vmware	6.14s
```

Smoke real do binário compilado (`mcpvmware-mcp.exe`), stdio JSON-RPC contra `simulator.VPX()`: `tools/list` devolveu **196 tools**, `vmware_dvs_fetch_dvports` (novo) chamado com sucesso real, além dos tools de fases anteriores já confirmados.

Narrativa completa em `.workspace/plans/MCPVMWare2026-08-10-175300-plano-cobertura-completa-api-codegen.plan.md` §"Fase 5 executada". Aprendizado reutilizável (fixture default de DVS no `simulator.VPX()`) em `.wolf/cerebrum.md`.

Ficheiros novos: `src/tools/generated_network.go(+test)`. Ficheiros alterados: `src/tools/registry.go` (wiring, 2 funções), `src/tools/mode_test.go` (catálogo canónico de 196 tools).

## Recomendações / próximos passos

1. **Revisão/aprovação do usuário desta Fase 5** antes de avançar para a Fase 6 (domínio Inventário/Organização) — critério do próprio plano.
2. Pendências documentadas e não bloqueantes (numeradas):
   1. `ReconfigureDVPort`/`ReconfigureLACP` sem handler no simulador — registados e testados só até "chega ao servidor".
   2. Os 4 métodos `EthernetCardBackingInfo()` continuam excluídos — revisitar se um tool de criação de NIC/adaptador de rede for adicionado no futuro (ex.: como parte de uma extensão do `vmware_vm_add_device` da Fase 2).
3. Padrão de decisão "domínio pequeno → escrever diretamente, domínio grande → subagents paralelos" confirmado como reutilizável para as próximas fases (6-8), reduz overhead de coordenação sem perder rigor.

---
title: "Fase 2 do plano de cobertura completa — codegen do domínio VM"
created: 2026-08-11
updated: 2026-08-11
type: report
status: final
locale: pt-PT
overview: "41 tools novas geradas para o domínio VirtualMachine/VmProvisioningChecker/VmCompatibilityChecker (76 tools no total), com 3 bugs reais de produção achados e corrigidos durante a verificação."
---

## Resumo

A Fase 2 do plano de cobertura completa (`.workspace/plans/MCPVMWare2026-08-10-175300-plano-cobertura-completa-api-codegen.plan.md`) gerou 41 tools novas para o domínio VM, levando o total de tools do servidor MCP de 31 para **76**. A revisão humana pré-geração excluiu 3 candidatos duplicados e adiou 2 por complexidade. Durante a integração e verificação (não durante a geração inicial) foram achados e corrigidos **3 bugs reais de produção** — um deles (bloqueio sem timeout num tool de espera) teria travado a ligação stdio inteira em uso real. Todos os 76 tools passam `go build`/`go vet`/`go test ./... -count=1` (cache limpa) e foram confirmados via smoke real do binário compilado por stdio JSON-RPC.

## Contexto

Trabalho executado em 10-11/08/2026, a pedido do usuário ("ok, prossiga" / "podemos continuar a execução?"), como continuação da Fase 1 (piloto de codegen, `OptionManager`, 2 tools) do mesmo plano. O domínio VM é o de maior superfície de risco do plano (mais candidatos Tier1/2), por isso o próprio plano exige revisão humana da lista de tier antes de gerar.

Ambiente: Go 1.25 (`github.com/cslsoftwares/mcpvmware`), testado contra `referencia/govmomi/simulator` (vcsim, in-process, sem vCenter/ESXi real necessário para a suite automatizada).

## Achados

### Pré-geração (revisão humana dos 53 métodos candidatos)

| Achado | Ação |
| --- | --- |
| `CreateSnapshot`/`RevertToSnapshot`/`RemoveSnapshot` duplicam tools já existentes em `vm.go` (mesma chamada govmomi, nome gerado só com ordem de palavra diferente) | Excluídos da geração |
| `AddDeviceWithProfile`/`EditDeviceWithProfile` (dispositivo polimórfico genérico) | Adiados — fora do MVP |
| `UnmountToolsInstaller` classificado Tier1 pelo gerador da Fase 0 por colisão de palavra-chave (`Unmount`) | Corrigido para Tier2 |

### Bugs reais achados durante a integração/verificação

1. **Bloqueio sem timeout em `vmware_vm_wait_for_ip`/`wait_for_net_ip`/`wait_for_power_state`.** O código continha um comentário afirmando que a chamada "confia no timeout do MCP do caller" — nunca verificado. `grep` confirmou que **não existe timeout nenhum** em `mcpvmware-mcp/main.go` nem `mcp/types.go`. Um teste automatizado real (`TestVMLifecycleTools_WaitForIPAndNetIP`) ficou preso **10 minutos reais** até o próprio Go matar o processo — em produção isto travaria a ligação stdio inteira (o servidor atende um pedido de cada vez), não só a chamada em curso.
2. **`vmware_vm_host_system`/`vmware_vm_resource_pool` devolviam string vazia (`""`) em vez do caminho de inventário real.** `VirtualMachine.HostSystem()`/`.ResourcePool()` constroem o objeto de retorno diretamente (`NewHostSystem`/`NewResourcePool`), nunca via `Finder` — por isso o campo `.InventoryPath` fica sempre vazio nesses objetos. O teste de `resource_pool` só verificava `!= nil` (uma string vazia passa nesse teste), o que mascarou o bug até o teste-irmão de `host_system` (que compara o valor exato) falhar.
3. **`VmProvisioningChecker`/`VmCompatibilityChecker` crashavam (nil-pointer) contra ESXi standalone**, achado *antes* de qualquer código ser gerado: `object.NewVmProvisioningChecker`/`NewVmCompatibilityChecker` desreferenciam `ServiceContent.VmProvisioningChecker`/`VmCompatibilityChecker`, ambos `nil` num host ESXi standalone (confirmado em `referencia/govmomi/simulator/esx/service_content.go`). Descobriu-se também, ao investigar isto, que **não existia nenhum `recover()`** em lado nenhum do dispatch de tools — um crash destes derrubaria o processo MCP inteiro, não só a chamada.

### Achado de processo

Um dos 3 subagents de geração (grupo "lifecycle", 25 tools) devolveu um relatório final vago porque o seu próprio `go test` interno tinha ficado preso no bug #1 acima e o turn dele terminou antes de reportar. Os bugs #1 e #2 só foram descobertos porque se correu `go test ./tools/... -timeout 120s` de forma independente, em vez de aceitar o relatório do agent como concluído.

## Evidências

```
$ cd src && gofmt -l . && go build ./... && go vet ./... && go clean -testcache && go test ./... -count=1
ok  	github.com/cslsoftwares/mcpvmware/tools	18.34s
ok  	github.com/cslsoftwares/mcpvmware/vmware	4.01s
```

Smoke real do binário compilado (`mcpvmware-mcp.exe`), dirigido via stdio JSON-RPC por um subprocess Go throwaway contra `simulator.VPX()`:

```
tools/list: 76 tools registered
first vm: /DC0/vm/DC0_H0_VM0
vmware_vm_boot_options OK: {"boot_options": {}, "vm": "/DC0/vm/DC0_H0_VM0"}
vmware_host_option_query OK
SMOKE DONE
```

Bugs registados em detalhe com causa raiz + fix em `.wolf/buglog.json` (`bug-006`, `bug-007`, `bug-008`). Narrativa completa por grupo em `.workspace/plans/MCPVMWare2026-08-10-175300-plano-cobertura-completa-api-codegen.plan.md` §"Fase 2 executada". Aprendizados reutilizáveis (InventoryPath vazio, nunca assumir timeout do caller, gap do vcsim em `WaitForNetIP`, não confiar em relatório de agent incompleto) em `.wolf/cerebrum.md`.

Ficheiros novos: `src/tools/generated_vm_snapshot.go(+test)`, `generated_vm_device.go(+test)`, `generated_vm_provisioning.go(+test)`, `generated_vm_lifecycle.go(+test)`. Ficheiros alterados: `src/tools/registry.go` (wiring + `recover()` em `CallTool`), `src/tools/mode_test.go` (catálogo canónico de 76 tools).

## Recomendações / próximos passos

1. **Revisão/aprovação do usuário desta Fase 2** antes de avançar para a Fase 3 (domínio Host, 11 ficheiros) — critério do próprio plano.
2. Considerar generalizar o padrão achado no bug #1 (timeout obrigatório em qualquer tool "wait for X") como regra de convenção para as Fases 3-8, não só uma correção pontual.
3. Pendências documentadas e não bloqueantes (numeradas):
   1. `AttachDisk` só testado com sucesso real contra `simulator.VPX()` (bug do próprio vcsim em modo `ESX()`, não do código deste projeto).
   2. `WaitForNetIP` sem caminho de sucesso testável contra vcsim (gap do simulador).
   3. `CustomizationSpec.Identity` polimórfico só aceita JSON genérico sem discriminador de tipo.
4. **Processo:** a partir de agora, resultados de fases/marcos que o usuário peça como "report" serão gravados aqui em `.workspace/reports/`, seguindo `workspace-plans-persist_V1.4.0.mdc` — não só espalhados por `.wolf/*`.

---
title: "Fase 3 do plano de cobertura completa — codegen do domínio Host"
created: 2026-08-11
updated: 2026-08-11
type: report
status: final
locale: pt-PT
overview: "70 tools novas geradas para o domínio Host (146 tools no total), com 1 bug real de produção achado e corrigido, mais 2 investigações reais sobre lacunas do vcsim documentadas."
---

## Resumo

A Fase 3 do plano de cobertura completa (`.workspace/plans/MCPVMWare2026-08-10-175300-plano-cobertura-completa-api-codegen.plan.md`) gerou 70 tools novas para o domínio Host, levando o total de tools do servidor MCP de 76 para **146**. A revisão humana pré-geração excluiu 12 métodos de `HostConfigManager` por serem só construtores internos. Os 4 grupos delegados em paralelo voltaram todos com relatórios completos e verificados — nenhum ficou preso ou devolveu relatório vago, ao contrário do que aconteceu num dos grupos da Fase 2. 1 bug real de produção foi achado e corrigido durante os testes (`bug-009`), mais 2 investigações genuínas sobre o comportamento do vcsim que revelaram assimetrias reais entre `ESX()`/`VPX()`.

## Contexto

Trabalho executado em 11/08/2026, a pedido do usuário ("ok, prossiga"), como continuação directa da Fase 2 (domínio VM, 41 tools). Antes de iniciar este trabalho, o usuário questionou a formatação da actualização do plano da Fase 2 contra a sua regra global de "atualizar o plano por fase/onda" — os gaps achados (pendências não numeradas, sem tag de estado, sem bloco de comando literal) foram corrigidos no plano e neste report é seguido o formato certo desde o início.

Ambiente: Go 1.25 (`github.com/cslsoftwares/mcpvmware`), testado contra `referencia/govmomi/simulator` (vcsim, in-process).

## Achados

### Pré-geração (revisão humana dos 82 métodos candidatos)

| Achado | Ação |
| --- | --- |
| Os 12 métodos de `HostConfigManager` (`AccountManager()`, `CertificateManager()`, etc.) são só construtores internos para os sub-managers reais — mesmo padrão já usado em `generated_option.go` | Excluídos da geração |
| Suporte real do vcsim verificado por **nome de método SOAP**, não nome do tipo Go — um erro próprio a meio (disse a um agent que `HostAccountManager` não era simulado, estava errado) corrigido via mensagem directa ao agent já a correr | 5 receivers simulados, 4 não-simulados, 1 parcial (`HostVirtualNicManager`, só `Info()`) — informação verificada dada a cada agent antes de escrever código |
| Sem colisões de nome com as 76 tools já existentes | Confirmado por script antes de gerar |

### Bugs e investigações reais durante a geração/testes

1. **`vmware_host_network_query_network_hint` devolvia `{"hints": null}` em vez de `{"hints": []}`** contra um host novo — achado e corrigido pelo próprio subagent do grupo "network" ao testar contra vcsim de verdade. Registado como `bug-009`.
2. **`HostAccountManager` simulado em `simulator.ESX()` mas não em `simulator.VPX()`** — o oposto do padrão habitual "vCenter tem mais funcionalidade, ESXi tem menos". Investigado a fundo pelo subagent do grupo "security": o template estático `esx/service_content.go` regista um `HostLocalAccountManager` real; `vpx/service_content.go` deixa o campo `nil`. Provado com um teste real (`TestHostSecurityTools_AccountManagerUnavailableOnVCenter`), não assumido.
3. **Bug no próprio vcsim vendorizado** (não corrigido — `referencia/` é só leitura): `simulator.HostLocalAccountManager.UpdateUser` devolve o tipo de resposta errado (`CreateUserResponse` em vez do tipo `UpdateUser*` correspondente), fazendo a chamada "suceder" silenciosamente sem validar a alteração de verdade.
4. **Correcção de uma suposição própria errada** no grupo "misc": eu tinha assumido que os managers não-simulados nem resolveriam localmente (erro de propriedade nula). O subagent provou, ao correr o teste de verdade, que a resolução local *funciona* (o template ESX já tem uma referência bem-formada em cada campo do `ConfigManager`) e o que falha é a 1ª chamada SOAP real — na verdade uma prova mais forte de que a canalização está correcta.

### Achado de processo

A lição da Fase 2 ("nunca aceitar relatório vago sem correr `go test` de novo") foi aplicada **preventivamente** nesta fase — cada um dos 4 prompts continha instruções explícitas para nunca terminar com uma mensagem vaga e para matar/investigar qualquer teste que arriscasse ficar preso mais de ~30-60s. Resultado: os 4 relatórios vieram completos, com `go test` real colado (não parafraseado), e nenhum bug ficou escondido atrás de um hang desta vez.

## Evidências

```
$ cd src && gofmt -l . && go build ./... && go vet ./...
(sem output — limpo)
$ go clean -testcache && go test ./... -count=1 -timeout 180s
ok  	github.com/cslsoftwares/mcpvmware/tools	28.91s
ok  	github.com/cslsoftwares/mcpvmware/vmware	4.70s
```

Smoke real do binário compilado (`mcpvmware-mcp.exe`), stdio JSON-RPC contra `simulator.VPX()`:

```
tools/list: 146 tools registered
vmware_vm_boot_options OK
vmware_host_option_query OK
vmware_host_firewall_info OK
SMOKE DONE
```

Bug registado com causa raiz + fix em `.wolf/buglog.json` (`bug-009`). Narrativa completa por grupo em `.workspace/plans/MCPVMWare2026-08-10-175300-plano-cobertura-completa-api-codegen.plan.md` §"Fase 3 executada". Aprendizado reutilizável (verificar vcsim por nome de método SOAP, não nome de tipo) em `.wolf/cerebrum.md`.

Ficheiros novos: `src/tools/generated_host_storage.go(+test)`, `generated_host_network.go(+test)`, `generated_host_security.go(+test)`, `generated_host_misc.go(+test)`. Ficheiros alterados: `src/tools/registry.go` (wiring), `src/tools/mode_test.go` (catálogo canónico de 146 tools).

## Recomendações / próximos passos

1. **Revisão/aprovação do usuário desta Fase 3** antes de avançar para a Fase 4 (domínio Storage/Datastore) — critério do próprio plano.
2. Pendências documentadas e não bloqueantes (numeradas):
   1. A maioria dos tools de `HostServiceSystem`/`HostDateTimeSystem`/`HostVsanSystem`/`HostVsanInternalSystem`/`HostAccountManager`-em-VPX e ~15 métodos de `HostStorageSystem`/`HostNetworkSystem`/`HostCertificateManager` não têm caminho de sucesso testável contra vcsim (gap do simulador, documentado por tool).
   2. `HostVirtualNicManager.SelectVnic`/`DeselectVnic` sem handler no simulador.
   3. Bug do vcsim vendorizado (`HostLocalAccountManager.UpdateUser`) não corrigido — `vmware_host_account_update` "sucede" silenciosamente em teste contra `simulator.ESX()` sem validar a mudança de verdade.
3. Considerar generalizar a técnica "verificar vcsim por nome de método SOAP, não por tipo Go" como passo obrigatório de pré-geração para as Fases 4-8, dado que já causou 1 erro próprio nesta fase.

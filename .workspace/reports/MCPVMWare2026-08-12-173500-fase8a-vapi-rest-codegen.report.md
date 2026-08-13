---
title: "Fase 8a do plano de cobertura completa — codegen VAPI/REST via wrappers Go"
created: 2026-08-12
updated: 2026-08-12
type: report
status: final
locale: pt-PT
overview: "202 tools novas geradas para o domínio VAPI/REST (495 tools no total), 2 levas de 4 grupos paralelos, achado de infra crítica nunca usada no projecto (simulador REST/VAPI separado do vcsim), 1 drift real de dependência apanhado pelo gate de build."
---

## Resumo

A Fase 8a do plano de cobertura completa (`.workspace/plans/MCPVMWare2026-08-10-175300-plano-cobertura-completa-api-codegen.plan.md`) gerou 202 tools novas cobrindo o domínio VAPI/REST via wrappers Go tipados (`vapi/*/*.go` — AST, mesma técnica das Fases 1-7), levando o total de tools do servidor MCP de 293 para **495**. Executada em 2 levas de 4 grupos paralelos cada, sob a autorização "ok, finalizar 100%". Achado mais significativo: descoberta de um simulador REST/VAPI separado dentro do vcsim (`vapi/simulator`), nunca antes importado neste projecto, que exige configuração explícita para funcionar — sem essa descoberta, toda a Leva 1 teria sido tratada erradamente como "vcsim gap".

## Contexto

Trabalho executado em 12/08/2026, como continuação directa da Fase 7 (domínio Gestão/Operações, 57 tools). Ambiente: Go 1.25 (`github.com/cslsoftwares/mcpvmware`), testado contra `referencia/govmomi/simulator` (vcsim, in-process) — pela primeira vez, também contra `github.com/vmware/govmomi/vapi/simulator` (REST/VAPI, separado).

## Achados

### Infra crítica — descoberta e validação prévia (antes de qualquer delegação)

Todas as Fases 0-7 trabalharam sobre `object/*.go` (wrappers SOAP sobre `vim25/types`, sem `json` tags nativas — daí o padrão `decodeJSONArg`). A Fase 8a trabalha sobre `vapi/*/*.go` — wrappers REST/JSON sobre `*rest.Client`, com `json` tags nativas (decode directo, sem workaround).

Descobri que o vcsim tem um **simulador REST/VAPI separado** (`github.com/vmware/govmomi/vapi/simulator`, distinto do pacote `simulator` core usado desde a Fase 0) — cobre `vapi/tags`, `vapi/library` (Content Library) e `vapi/vcenter` (templates/OVF). Só arranca com: (1) blank-import `_ "github.com/vmware/govmomi/vapi/simulator"` (regista-se via `init()`+`simulator.RegisterEndpoint`); (2) `model.Service.RegisterEndpoints = true` **antes** de `NewServer()` — sem isto, toda rota REST devolve 404 limpo, facilmente confundível com "domínio não simulado". Confirmado com um spike próprio (`vapispike/main.go`, scratchpad) antes de delegar a qualquer subagent — não assumido. Aplicado uma única vez em `src/tools/testhelpers_test.go`; todos os 8 grupos delegados beneficiaram sem repetir a configuração.

Excluído `vapi/rest.Client` (11 métodos: `Login`/`Logout`/`Session`/`Do`/`Download*`/`Upload`/`WithHeader`/`WithSigner`) — plumbing interno de sessão/transporte, já gerido por `vmware.Client.REST(ctx)` desde a Fase 4; expor como tools deixaria um chamador MCP fazer logout da sessão REST partilhada por todas as outras tools `vapi/*`, ou disparar pedidos HTTP arbitrários. Mesma classe de exclusão já aplicada a `HostConfigManager` (Fase 3) e `DiagnosticManager.Log` (Fase 7).

### Leva 1 — domínios com suporte REAL no vcsim (105 tools)

Content Library (`vapi/library`) + Tags (`vapi/tags`) + templates vCenter (`vapi/vcenter`) — cobertura confirmada por import directo em `vapi/simulator/simulator.go`. 4 grupos paralelos:

1. **library-core (27):** achou e corrigiu 2 bugs reais do vcsim — panic de índice em biblioteca `SUBSCRIBED` sem `storage_backings`; rejeição correcta de `CreateLibraryItem` directo numa biblioteca subscrita (com sync automático confirmado no create).
2. **library-sessions (26):** fundiu `WaitOnLibraryItemUpdateSession` (callback client-side não representável em JSON) com `timeout_seconds` obrigatório; achou 2 gaps reais do vcsim em remove/cancel de sessão; corrigiu a própria suposição errada sobre panic de nil-interface após correr o teste de verdade.
3. **library-misc (15, não 17):** `finder.go` só tem 1 método real (estimativa do orquestrador estava errada), achado e documentado. Corrigiu `DefaultOvfSecurityPolicy` tier2→sem-tier.
4. **tags-vcenter (37):** reusou `resolveEntityRef(s)` da Fase 7 (SSOT); achou 2 gaps/riscos reais do vcsim (`Placement.Folder` nil causa panic real no simulador; `DeployLibraryItem` de OVF sem upload falha com `os.ReadFile` ausente).

1 colisão transiente de nome (`libraryManager`) entre 2 grupos, auto-resolvida por um deles renomeando a própria função.

### Leva 2 — domínios SEM suporte no vcsim (97 tools)

Namespace/Supervisor (vSphere with Tanzu), vLCM cluster settings, módulos DRS de cluster, criptografia KMS, tasks CIS genéricas, VM Data Sets, sub-domínios pequenos de administração do Appliance, emissão de token de autenticação — "vcsim gap, not a bug" confirmado por evidência dupla (ausência do import correspondente em `vapi/simulator/simulator.go`, mais grep próprio de cada subagent). 4 grupos paralelos:

1. **namespace-core (22):** excluiu `SupportBundleRequest` (só constrói `*http.Request` não enviado); achou um bug real no próprio `referencia/govmomi` vendorizado (`fmt.Fprint(os.Stdout, spec)` esquecido em `EnableCluster`) — documentado, não corrigido (código de terceiros).
2. **namespace-services (21):** confirmou o gap por 2 vias, incluindo um subpacote `vapi/namespace/simulator` que existe mas nunca é importado em lado nenhum.
3. **cluster-settings-crypto-tasks (29, não 30):** **achado real de drift de dependência (`bug-012`)** — `go.mod` fixa `govmomi v0.55.1` sem `replace`, mas `referencia/govmomi` (usado para leitura desde a Fase 0) é um checkout diferente/mais novo; `vms.Manager.DeleteSolutionOnly` existe em `referencia/` mas não na versão realmente compilada. Confirmado por diff byte-a-byte dos 9 ficheiros do escopo. Corrigido removendo a tool afectada (não o `go.mod` partilhado). O gate de `go build` limpo obrigatório por fase apanhou isto automaticamente.
4. **vm-dataset-appliance-small (25):** confirmou empiricamente 404 real em todos os 9 sub-domínios (verificado em cópia isolada por builds quebrados transitórios de outros grupos). Achou o único caso de sub-classificação desta fase inteira (`authentication.Issue`, POST real que emite um token — classificador tinha marcado sem-tier por engano, corrigido para tier2).

## Evidências

```
$ cd src && gofmt -l . && go build ./... && go vet ./...
(sem output — limpo)
$ go clean -testcache && go test ./... -count=1 -timeout 300s
ok  	github.com/cslsoftwares/mcpvmware/tools	77.575s
ok  	github.com/cslsoftwares/mcpvmware/vmware	4.275s
$ go test ./... -count=1 -timeout 300s   # 2ª corrida, sem flake
ok  	github.com/cslsoftwares/mcpvmware/tools	79.266s
ok  	github.com/cslsoftwares/mcpvmware/vmware	5.769s
```

Smoke real do binário compilado (`mcpvmware-mcp.exe`), stdio JSON-RPC contra `simulator.VPX()` (com blank-import+`RegisterEndpoints` também no script de smoke): `tools/list` devolveu **495 tools**; `vmware_library_list_libraries`/`vmware_tags_list_categories` (Leva 1) chamados com sucesso real; `vmware_namespace_list_namespaces`/`vmware_cis_tasks_get` (Leva 2) chamados e confirmaram erro real 404 do vcsim (via `result.isError:true`, convenção MCP — não o campo `error` do JSON-RPC top-level, achado de processo próprio corrigido a meio da verificação).

Narrativa completa em `.workspace/plans/MCPVMWare2026-08-10-175300-plano-cobertura-completa-api-codegen.plan.md` §"Fase 8a executada". Aprendizados reutilizáveis (simulador REST/VAPI separado; `referencia/govmomi` pode ter drift do `go.mod`; convenção `isError` do MCP; caso de sub-classificação) em `.wolf/cerebrum.md`. `bug-012` em `.wolf/buglog.json`.

Ficheiros novos: 9 pares `generated_{library_core,library_sessions,library_misc,tags,vcenter_template,namespace_core,namespace_services,esx_settings_cluster_vms,cluster_modules,crypto,cis_tasks,vm_dataset,appliance_small,authentication}.go(+test)` (13 pares reais, alguns grupos produziram 2 ficheiros). Ficheiros alterados: `src/tools/testhelpers_test.go` (infra REST/VAPI), `src/tools/registry.go` (wiring, 14 funções), `src/tools/mode_test.go` (catálogo canónico de 495 tools).

**Sem commits** — não pedido pelo usuário nesta rodada.

## Recomendações / próximos passos

1. Usuário autorizou ("ok, finalizar 100%") avançar directo para a Fase 8b (parse de collection Postman para o gap VAMI) sem pausar para aprovação intermédia — próxima etapa a iniciar nesta mesma sessão, ainda dentro da Fase 8.
2. Pendências documentadas e não bloqueantes (numeradas):
   1. `vapi/rest.Client` (11 métodos de plumbing interno) permanece deliberadamente excluído.
   2. `SupportBundleRequest`/`KmsProviderExportRequest` excluídos — um fluxo de download real (enviar o request + gravar resposta em disco) fica fora de escopo por agora.
   3. `bug-012` (drift `referencia/govmomi` vs `go.mod`) — se aparecer de novo, mesmo tratamento (remover a tool afectada, nunca mexer no `go.mod` partilhado sem combinação explícita do usuário).
   4. Domínio inteiro da Leva 2 sem NENHUMA verificação funcional real possível neste ambiente — nem vcsim nem o host real (`10.100.2.54`, ESXi standalone sem VAMI/Supervisor) simulam nada disto. Só testado até "chega ao servidor com erro real".

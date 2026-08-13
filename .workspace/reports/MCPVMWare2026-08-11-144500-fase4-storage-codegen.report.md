---
title: "Fase 4 do plano de cobertura completa — codegen do domínio Storage/Datastore"
created: 2026-08-11
updated: 2026-08-11
type: report
status: final
locale: pt-PT
overview: "43 tools novas geradas para o domínio Storage/Datastore (189 tools no total), com 1 bug real de produção achado e corrigido, mais 1 correção de classificação de modo (mode) descoberta antes de gerar código."
---

## Resumo

A Fase 4 do plano de cobertura completa (`.workspace/plans/MCPVMWare2026-08-10-175300-plano-cobertura-completa-api-codegen.plan.md`) gerou 43 tools novas para o domínio Storage/Datastore, levando o total de tools do servidor MCP de 146 para **189**. A revisão pré-geração encontrou um erro real de classificação herdado da Fase 0 (2 receivers marcados `vcenter-only` por engano) e excluiu 5 métodos sem representação JSON útil. Os 4 grupos delegados em paralelo voltaram todos com relatórios completos — nenhum ficou preso, terceira vez seguida que a técnica preventiva (instruções explícitas contra hangs) funciona. 1 bug real de produção (um crash dentro do próprio vcsim, que provavelmente também afeta vCenter/ESXi real) foi achado e corrigido.

## Contexto

Trabalho executado em 11/08/2026, a pedido do usuário ("ok, prossiga"), como continuação directa da Fase 3 (domínio Host, 70 tools). Ambiente: Go 1.25 (`github.com/cslsoftwares/mcpvmware`), testado contra `referencia/govmomi/simulator` (vcsim, in-process).

## Achados

### Pré-geração (revisão humana dos 49 métodos candidatos)

| Achado | Ação |
| --- | --- |
| `StorageResourceManager` e `DatastoreNamespaceManager` estavam erradamente marcados `vcenter-only` desde a Fase 0 (assumia-se que a funcionalidade — Storage DRS / namespaces VSAN — exigia vCenter, mas os objectos SOAP em si existem em ambos) | Verificado por evidência (`ServiceContent` não-nulo em `esx/` e `vpx/`), corrigido em `gen/main.go`, gerador re-rodado |
| 5 métodos de `Datastore` sem representação JSON útil (`Upload`/`Download` tomam/devolvem streams; `HostContext`/`WithProgress` devolvem `context.Context`; `FindInventoryPath` só muta o próprio receiver) | Excluídos da geração |
| Sem colisões de nome com as 146 tools já existentes | Confirmado por script antes de gerar |

### Bugs e correções reais durante a geração/testes

1. **`vmware_datastore_search`/`search_subfolders` causavam um crash dentro do próprio vcsim** quando `search_spec` era omitido — `SearchSpec.Query`/`.Details` desreferenciados sem nil-check no simulador. Corrigido com um default sensato em vez de passar `nil`. Registado como `bug-010`. Como o simulador replica o comportamento real da API, é plausível que o mesmo padrão de chamada também causasse um erro no vCenter/ESXi real (não confirmado, mas o achado motivou a correção defensiva de qualquer forma).
2. **Uma suposição minha sobre um construtor inexistente** (`object.NewDatastoreFileManager(?)`) foi corrigida pelo subagent do grupo "file managers" — o real é `Datastore.NewFileManager(dc, force)`.
3. **Uma imprecisão factual entre 2 grupos**, achada e sinalizada pelo grupo "file managers": o comentário do grupo "virtual disk" afirmava que os 2 tools `*delete_virtual_disk` (um em cada grupo) usam SOAP methods diferentes — não usam, é a mesma chamada (`VirtualDiskManager.DeleteVirtualDisk`) alcançada por 2 caminhos Go diferentes. Corrigido por mim durante a integração.
4. **`SetVirtualDiskUuid` do vcsim é um stub permanente** (`// TODO: validate uuid format and persist` no próprio código-fonte vendorizado) — nunca persiste de verdade. Achado pelo grupo "virtual disk" com um teste explícito de não-round-trip.
5. **Achado arquitectural mais amplo**: `vim25/types` não tem `UnmarshalJSON` customizado em lado nenhum do pacote — nenhum campo genuinamente polimórfico aninhado (não só o de topo) consegue ser preenchido via o padrão "JSON genérico" já usado desde a Fase 2. Limitação estrutural, vai reaparecer nas Fases 5-8.

## Evidências

```
$ cd src && gofmt -l . && go build ./... && go vet ./...
(sem output — limpo)
$ go clean -testcache && go test ./... -count=1 -timeout 240s
ok  	github.com/cslsoftwares/mcpvmware/tools	33.07s
ok  	github.com/cslsoftwares/mcpvmware/vmware	4.57s
$ go test ./... -count=1 -timeout 240s   # 2ª corrida, sem flake
ok  	github.com/cslsoftwares/mcpvmware/tools	34.25s
ok  	github.com/cslsoftwares/mcpvmware/vmware	4.69s
```

Smoke real do binário compilado (`mcpvmware-mcp.exe`), stdio JSON-RPC contra `simulator.VPX()`:

```
tools/list: 189 tools registered
vmware_vm_boot_options OK
vmware_host_option_query OK
vmware_host_firewall_info OK
first datastore: LocalDS_0
vmware_datastore_type OK
SMOKE DONE
```

Bug registado com causa raiz + fix em `.wolf/buglog.json` (`bug-010`). Narrativa completa por grupo em `.workspace/plans/MCPVMWare2026-08-10-175300-plano-cobertura-completa-api-codegen.plan.md` §"Fase 4 executada". Aprendizados reutilizáveis (limitação de `UnmarshalJSON`, stub do `SetVirtualDiskUuid`) em `.wolf/cerebrum.md`.

Ficheiros novos: `src/tools/generated_datastore_browser.go(+test)`, `generated_datastore_filemanagers.go(+test)`, `generated_storage_drs.go(+test)`, `generated_virtual_disk.go(+test)`. Ficheiros alterados: `src/tools/registry.go` (wiring), `src/tools/mode_test.go` (catálogo canónico de 189 tools), `src/gen/main.go` (correção de modo).

## Recomendações / próximos passos

1. **Revisão/aprovação do usuário desta Fase 4** antes de avançar para a Fase 5 (domínio Rede) — critério do próprio plano.
2. Pendências documentadas e não bloqueantes (numeradas):
   1. `vim25/types` sem `UnmarshalJSON` customizado — vai limitar campos polimórficos aninhados nas Fases 5-8 também, não só nesta.
   2. `SetVirtualDiskUuid` não verificável ponta-a-ponta contra vcsim (stub permanente) — precisaria de host/vCenter real.
   3. `CreateChildDisk`/`InflateVirtualDisk`/`ShrinkVirtualDisk` sem handler no simulador — registados e testados só até "chega ao servidor".
   4. `ConfigureDatastoreIORM`'s parâmetro `key` é morto no wrapper Go do govmomi (não é bug nosso) — mantido no schema por fidelidade, documentado como aceite-mas-ignorado.
   5. `DatastoreNamespaceManager`'s 3 tools só funcionam de verdade contra datastores VSAN/VVol reais — requisito genuíno do vSphere, não gap de teste.
3. Considerar reportar o crash do vcsim (`search_spec` nil) upstream ao projecto govmomi, já que o simulador é open-source e o achado pode ajudar outros utilizadores.

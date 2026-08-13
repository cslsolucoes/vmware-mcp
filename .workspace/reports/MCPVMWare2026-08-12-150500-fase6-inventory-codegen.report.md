---
title: "Fase 6 do plano de cobertura completa — codegen do domínio Inventário/Organização"
created: 2026-08-12
updated: 2026-08-12
type: report
status: final
locale: pt-PT
overview: "40 tools novas geradas para o domínio Inventário/Organização (236 tools no total), 3 grupos paralelos, 3 correções reais de tipo/nil-safety/comportamento vSphere achadas antes ou durante os testes."
---

## Resumo

A Fase 6 do plano de cobertura completa (`.workspace/plans/MCPVMWare2026-08-10-175300-plano-cobertura-completa-api-codegen.plan.md`) gerou 40 tools novas para o domínio Inventário/Organização (`Folder`, `Datacenter`, `ComputeResource`, `ClusterComputeResource`, `ResourcePool`, `VirtualApp`, `EnvironmentBrowser`), levando o total de tools do servidor MCP de 196 para **236**. Domínio de tamanho médio-grande — dividido em 3 grupos paralelos por ficheiro (subagents Sonnet), mesmo padrão de delegação já estabelecido nas Fases 2-4. O usuário autorizou nesta rodada ("ok, finalizar 100%") avançar pelas fases restantes (6-10) sem pausar para aprovação intermédia entre elas, mantendo o mesmo rigor de verificação por fase.

## Contexto

Trabalho executado em 12/08/2026, a pedido do usuário, como continuação directa da Fase 5 (domínio Rede, 7 tools). Ambiente: Go 1.25 (`github.com/cslsoftwares/mcpvmware`), testado contra `referencia/govmomi/simulator` (vcsim, in-process).

## Achados

### Pré-geração (revisão dos 40 métodos candidatos)

| Achado | Ação |
| --- | --- |
| `ComputeResource.EnvironmentBrowser()` é um zero-arg accessor — padrão das Fases 2-5 era excluir esses por falta de consumidor | **Reversão deliberada**: `EnvironmentBrowser` tem 4 métodos reais nesta mesma fase (`QueryConfigOption` etc.) — mantido como o accessor de entrada, documentado como reversão explícita do padrão, não desvio silencioso |
| `ClusterComputeResource.PlaceVm` e `Folder.PlaceVmsXCluster` classificados tier2 pelo gerador da Fase 0 | Corrigidos para sem-tier — ambos síncronos (sem sufixo `_Task`), devolvem `*types.Result` (recomendação dry-run), mesmo raciocínio já aplicado a `vmware_storage_recommend_datastores` na Fase 4 |
| Vários zero-arg accessors classificados tier2 pelo fail-safe do gerador (`ComputeResource.{Datastores,Hosts,ResourcePool}`, `ClusterComputeResource.Configuration`, `ResourcePool.Owner`, `Folder.Children`, `Datacenter.Folders`) | Corrigidos para sem-tier — mesmo padrão recorrente desde a Fase 0 |
| Colisão de nome com as 196 tools já existentes | Nenhuma encontrada |

### Achados durante a escrita/testes (por grupo)

1. **Grupo folder/datacenter (14 tools):** achou e corrigiu um nil-deref real no `govmomi` — `Folder.CreateVM` desreferencia `pool` sem nil-check quando o parâmetro é omitido (panic real, não hipotético); corrigido tornando `resource_pool` obrigatório na tool. Auto-corrigiu uma redeclaração própria de `resolveDatacenter` (já existia desde a Fase 4, reusado em vez de duplicado — SSOT). Achou 2 gotchas de fixture do vcsim (colisão de nome "DVS0", já conhecida da Fase 5; `RegisterVM` com nome diferente do `.vmx` original causa falta real de `.nvram` no vcsim — contornado no teste, não é bug da tool).
2. **Grupo compute/cluster/environment (13 tools):** achou e corrigiu uma suposição de tipo errada da orquestração — `types.ComputeResourceConfigSpecEx` **não existe** no govmomi vendorizado; usado `types.ClusterConfigSpecEx` (o mesmo tipo que `govc` e o próprio `simulator.ReconfigureComputeResourceTask` usam). Confirmou suporte vcsim completo neste grupo, sem gaps.
3. **Grupo resourcepool/vapp (13 tools):** achou que `ResourcePool.CreateVApp`/`CreateChildVM` estão desabilitados em ESXi standalone por **restrição real do vSphere** (`esx.ResourcePool.DisabledMethod`), não um gap do simulador — provado com um teste que confirma os dois resultados (falha limpa em `ESX()`, sucesso em `VPX()`). Corrigiu também uma suposição de tipo errada da orquestração — `ImportVApp` decodifica a spec para `types.VirtualMachineImportSpec`, não o `types.ImportSpec` abstrato sugerido inicialmente (confirmado por 2 vias: o tipo abstrato não tem payload usável, e o simulador faz type-assert directo pro concreto).

## Evidências

```
$ cd src && gofmt -l . && go build ./... && go vet ./...
(sem output — limpo)
$ go clean -testcache && go test ./... -count=1 -timeout 300s
ok  	github.com/cslsoftwares/mcpvmware/tools	43.93s
ok  	github.com/cslsoftwares/mcpvmware/vmware	4.20s
$ go test ./... -count=1 -timeout 300s   # 2ª corrida, sem flake
ok  	github.com/cslsoftwares/mcpvmware/tools	43.58s
ok  	github.com/cslsoftwares/mcpvmware/vmware	4.50s
```

Smoke real do binário compilado (`mcpvmware-mcp.exe`), stdio JSON-RPC contra `simulator.VPX()`: `tools/list` devolveu **236 tools**; `vmware_folder_children` e `vmware_resource_pool_owner` (novos) chamados com sucesso real, além dos tools de fases anteriores já confirmados.

Narrativa completa em `.workspace/plans/MCPVMWare2026-08-10-175300-plano-cobertura-completa-api-codegen.plan.md` §"Fase 6 executada". Aprendizados reutilizáveis (ESXi desabilita criação de VApp; `ClusterConfigSpecEx` vs tipo inventado; `find.InventoryPath` como padrão de bug #1 recorrente através das Fases 2/3/4/6) em `.wolf/cerebrum.md`.

Ficheiros novos: `src/tools/generated_inventory_folder.go(+test)`, `src/tools/generated_inventory_compute.go(+test)`, `src/tools/generated_resourcepool_vapp.go(+test)`. Ficheiros alterados: `src/tools/registry.go` (wiring, 4 funções — 2 mistas vcenter-only/vsphere-general), `src/tools/mode_test.go` (catálogo canónico de 236 tools).

**Sem commits** — não pedido pelo usuário nesta rodada.

## Recomendações / próximos passos

1. Usuário autorizou ("ok, finalizar 100%") avançar directo para a Fase 7 (domínio Gestão/Operações) sem pausar para aprovação intermédia — próxima fase já iniciada nesta mesma sessão.
2. Pendências documentadas e não bloqueantes (numeradas):
   1. `vmware_resource_pool_import_vapp`/`vmware_vapp_clone`/etc. herdam a mesma limitação MVP de specs polimórficos aninhados já documentada nas Fases 2-4 (`vim25/types` sem `UnmarshalJSON` customizado).
   2. `VirtualApp.{PowerOn,PowerOff,Suspend,UpdateConfig}` sem handler no simulador — registados e testados só até "chega ao servidor".
   3. `vmware_folder_create_datacenter`/`create_cluster` devolvem `{"result":"not_supported_on_standalone_esxi"}` (não erro) quando chamados contra ESXi standalone — comportamento real do vSphere, não um bug.
3. Verificar no início da Fase 7 se `namespace_manager.go` ainda tem conteúdo por cobrir — a Fase 4 já cobriu `DatastoreNamespaceManager` (o único tipo nesse ficheiro); o conceito de Namespace "Supervisor Kubernetes" vive inteiramente em `vapi/namespace`, concern da Fase 8, não deste ficheiro `object/`.

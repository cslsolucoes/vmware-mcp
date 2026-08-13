---
title: "Fase 10 do plano de cobertura completa — VMware Cloud on AWS, terceiro cliente — FECHA O PLANO A 100%"
created: 2026-08-12
updated: 2026-08-12
type: report
status: final
locale: pt-PT
overview: "95 tools novas geradas para VMware Cloud on AWS (734 tools no total), terceiro cliente/arquitectura construído de raiz. Última fase do plano de cobertura completa — com esta entrega, o objectivo de 100% de cobertura da API (vSphere + Workstation Pro + VMC on AWS) está atingido."
---

## Resumo

A Fase 10 do plano de cobertura completa (`.workspace/plans/MCPVMWare2026-08-10-175300-plano-cobertura-completa-api-codegen.plan.md`) gerou 95 tools novas para VMware Cloud on AWS (VMC), um terceiro produto distinto de vSphere e Workstation Pro, levando o total de tools do servidor MCP de 639 para **734**. **Esta é a última fase do plano — com ela concluída, o objectivo "100% da API" (literalmente cada método SOAP + rota REST dos 3 produtos VMware catalogados) está atingido.** O plano foi fechado (frontmatter `status: concluído`).

## Contexto

Trabalho executado em 12/08/2026, como continuação directa da Fase 9 (VMware Workstation Pro, 28 tools). Ambiente: Go 1.25 (`github.com/cslsoftwares/mcpvmware`). Sem conta/organização VMC real disponível — testado via fixture `httptest`, e via 2 smokes reais do binário compilado (regressão vSphere + CloudAWS novo, ambos sem tocar serviços de produção externos).

## Achados

### Extracção e curadoria das 95 rotas

Parse estruturado (Python) de `.workspace/VMware Cloud on AWS APIs.postman_collection.json` (99 rotas brutas em 3 pastas: Orgs, SDDCs, Networking). Excluída 1 rota (Authentication/Login, o próprio exchange CSP, tratado como plumbing interno do cliente — mesma classe de exclusão de `vapi/rest.Client` na Fase 8a) + 3 duplicados reais confirmados por (método, URL) idêntico (`offer-instances`, `nat/config/rules` POST, `l2vpn/config` PUT) = **95 tools reais**, divididas em 4 grupos por domínio real da collection.

### Cliente `cloudaws.Client` construído de raiz

`src/cloudaws/client.go` — auth em 2 passos: `POST .../csp/gateway/am/api/auth/api-tokens/authorize?refresh_token=...` troca um `refresh_token` (gerado manualmente na consola web da VMC, sem API para gerar um) por um `access_token` de curta duração (`Authorization: Bearer ...` em todas as chamadas), com cache + renovação automática antes de expirar + 1 retry forçado em 401. Verificado com 5 testes próprios (`client_test.go`) — exchange real, cache confirmado (2ª chamada não re-troca), expiração forçando novo exchange, 401 disparando exactamente 1 retry — ANTES de delegar qualquer tool.

### Achado de risco financeiro real — tratado com cautela redobrada

Um SDDC na AWS custa dinheiro por hora enquanto existir — ao contrário de toda fase anterior, um erro de tier aqui tem custo financeiro directo, não só operacional. **Toda operação de escrita no domínio SDDCs ficou Tier1 por padrão** (mais cauteloso que a convenção normal "DELETE=tier1, resto=tier2" usada no resto do projecto), com só 3 excepções documentadas explicitamente (política EDRS, DNS público/privado do SDDC — configuração, não infra). `Subscription/Create` (pasta Orgs, não SDDCs) também elevado a Tier1 pela mesma razão — compromete facturação real. Decisão tomada pelo orquestrador antes de delegar, não deixada para os subagents inferirem sozinhos.

### 4 grupos paralelos

1. **orgs (29):** gestão de organização/conta VMC. Colapsou o duplicado real "Subscription/Offers/List" vs "Subscription/List Available by Region". Aplicou os 2 casos financeiramente sensíveis (`subscription_create`, `account_link_delete`) exactamente como decidido.
2. **sddcs (23):** o domínio mais sensível. Aplicou "toda escrita=tier1" com as 3 excepções à risca, documentado extensivamente no comentário de topo do ficheiro. Achou que vários corpos de request na collection são só placeholders textuais (não JSON real) — expostos como argumento `spec` genérico em vez de inventar schema.
3. **networking-core (19):** Networks/Firewall/NAT. Confirmou o dedup de NAT por leitura directa do JSON da collection. `PUT`/`DELETE` de config COMPLETA de firewall/NAT elevados a tier1 (substituição/remoção total, mesma cautela já aplicada a rotas equivalentes na Fase 8b).
4. **networking-edge (24):** IPSec/L2VPN/DNS-de-edge/Edge Devices/DHCP/Estatísticas/Connectivity. Confirmou por leitura directa (linhas exactas do JSON) que o duplicado L2VPN "Details"/"Update" é o MESMO endpoint PUT documentado 2x na collection, não 2 rotas reais — fundido com `show_sensitive_data` opcional.

**Zero colisões de build entre os 4 grupos paralelos** — melhor resultado de toda a sessão (todas as fases anteriores com ≥2 grupos tiveram pelo menos 1 colisão transiente, sempre auto-resolvida).

### Achado de integração

`mode_test.go`'s `TestMode_CloudAWS`/`TestMode_Unrestricted` tinham asserções hardcoded de "0 tools"/contagens antigas, escritas antes de qualquer tool CloudAWS existir — corrigido com o catálogo `cloudAWSTools` (95 nomes) e a asserção "nenhuma tool vSphere/Workstation vaza para este modo", mesmo padrão já usado nas Fases 8a/9.

## Evidências

```
$ cd src && gofmt -l . && go build ./... && go vet ./...
(sem output — limpo)
$ go clean -testcache && go test ./... -count=1 -timeout 400s
ok  	github.com/cslsoftwares/mcpvmware/cloudaws	2.774s
ok  	github.com/cslsoftwares/mcpvmware/tools	77.282s
ok  	github.com/cslsoftwares/mcpvmware/vmware	4.381s
ok  	github.com/cslsoftwares/mcpvmware/workstation	0.717s
$ go test ./... -count=1 -timeout 400s   # 2ª corrida, sem flake
ok  	github.com/cslsoftwares/mcpvmware/cloudaws	2.506s
ok  	github.com/cslsoftwares/mcpvmware/tools	78.053s
ok  	github.com/cslsoftwares/mcpvmware/vmware	4.434s
ok  	github.com/cslsoftwares/mcpvmware/workstation	1.289s
```

**2 smokes reais do binário compilado**, via subprocess Go throwaway:

1. Regressão vSphere (`--vcenter-url` contra `simulator.VPX()`): 611 tools confirmadas (nenhuma mudança), todas as chamadas anteriores continuam OK.
2. CloudAWS novo (`--cloud-aws-url`+`--refresh-token` contra um fixture local): **95 tools registadas, zero leak de tools vSphere/Workstation**, `vmware_cloudaws_sddc_create` correctamente negado pelo gate de 3 camadas sem `--allow-destructive`. Deliberadamente **NÃO** foi chamado nenhum tool sem gate (ex.: `org_list`), porque este binário não tem flag para redireccionar os hosts CSP/VMC para um fixture local — evitar qualquer chamada de rede real aos servidores de produção da VMware durante um smoke automatizado não-assistido.

Narrativa completa em `.workspace/plans/MCPVMWare2026-08-10-175300-plano-cobertura-completa-api-codegen.plan.md` §"Fase 10 executada". Aprendizados reutilizáveis (padrão de N-ésimo cliente; risco financeiro exige tier mais cauteloso; zero colisões com prefixos de helper únicos; nunca tocar produção num smoke não-assistido) em `.wolf/cerebrum.md`.

Ficheiros novos: `src/cloudaws/client.go(+test)`, `src/tools/cloudaws_{orgs,sddcs,networking_core,networking_edge}.go(+test)`. Ficheiros alterados: `src/tools/registry.go` (arquitectura de 3º cliente, wiring), `src/tools/destructive.go` (`registerDestructiveCloudAWS`), `src/mcpvmware-mcp/main.go` (flags `--cloud-aws-url`/`--refresh-token` habilitados, branch de construção de cliente), `src/tools/mode_test.go` (catálogo `cloudAWSTools`, correcção de `TestMode_CloudAWS`/`TestMode_Unrestricted`).

**Sem commits** — não pedido pelo usuário nesta rodada.

## Recomendações / próximos passos

**O plano de cobertura completa (Fases 0-10) está concluído.** 734 tools registadas, cobrindo 3 produtos VMware distintos:
- vSphere/vCenter/ESXi (`govmomi`) — 611 tools.
- VMware Workstation Pro (`vmrest`) — 28 tools.
- VMware Cloud on AWS — 95 tools.

Pendências documentadas e não bloqueantes (numeradas) — extensões futuras, não gaps de cobertura:

1. **Decisão em aberto, não resolvida por esta fase**: `--vmware-all-url` incluir tools de Workstation/CloudAWS exigiria `main.go` aceitar múltiplas URLs/credenciais em simultâneo — sinalizado desde 10/08, continua pendente de decisão explícita do usuário.
2. Sem conta/organização VMC real disponível — testado só via fixture `httptest`, nunca contra a API real.
3. Vários corpos de request de shape complexo/incerto (specs de SDDC, regras de firewall/NAT, config NSX-T Edge) aceites como passthrough JSON genérico — a collection não documenta os schemas reais em detalhe suficiente.
4. Os achados de prioridade MÉDIA/BAIXA da revisão da Fase 0 original (itens 5-9) nunca bloquearam nenhuma fase, ficam como item de limpeza opcional.
5. Nenhum commit foi feito em nenhuma fase desta sessão.

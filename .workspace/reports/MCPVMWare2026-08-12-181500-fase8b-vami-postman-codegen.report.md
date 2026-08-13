---
title: "Fase 8b do plano de cobertura completa — codegen VAMI via parse de collection Postman"
created: 2026-08-12
updated: 2026-08-12
type: report
status: final
locale: pt-PT
overview: "116 tools novas geradas para o domínio VAMI legacy (611 tools no total), primeira técnica de geração sem AST sobre Go — parse directo de uma collection Postman — fecha a Fase 8 inteira (318 tools novas: 202 da 8a + 116 da 8b)."
---

## Resumo

A Fase 8b do plano de cobertura completa (`.workspace/plans/MCPVMWare2026-08-10-175300-plano-cobertura-completa-api-codegen.plan.md`) gerou 116 tools novas para o domínio VAMI (VMware Appliance Management Interface, API REST legacy `/rest/appliance/...`), levando o total de tools do servidor MCP de 495 para **611**. Esta é a primeira fase de todo o plano (0-8b) que não usa AST sobre código Go real — as rotas vêm directo de uma collection Postman vendorizada, exigindo uma técnica de geração inteiramente nova. Com a conclusão desta fase, a **Fase 8 fecha por completo**: 318 tools novas desde o seu início (202 da Fase 8a via AST sobre `vapi/*/*.go`, 116 da Fase 8b via parse de Postman).

## Contexto

Trabalho executado em 12/08/2026, como continuação directa da Fase 8a (VAPI/REST via wrappers Go, 202 tools). Ambiente: Go 1.25 (`github.com/cslsoftwares/mcpvmware`). Sem simulador nem host real disponíveis para este domínio — testado via fixture `httptest`, mesmo padrão já estabelecido na Fase 4 para as 4 primeiras tools de VAMI.

## Achados

### Técnica de geração nova

Sem wrapper Go de origem, a extracção de candidatos veio de um parse estruturado (Python) da collection `.workspace/vSphere Automation REST Resources for appliance.postman_collection.json` (132 rotas em 21 pastas). Classificação de tier feita manualmente (verbo HTTP + semântica do nome da rota — sem fonte Go pra confirmar comportamento real, ao contrário de todas as fases anteriores).

### Dedup — achado e correcção importante

A suposição inicial (baseada na estimativa "~117 gap" já registada desde 10/08) era que os 15 métodos Go cobertos pela Fase 8a (`vapi/appliance/{access,shutdown,networking,logging}`) seriam duplicados literais das rotas homónimas na collection. **Confirmado como falso** lendo `vapi/rest.Client.Resource()` (`referencia/govmomi/vapi/rest/client.go`): paths que começam por `/api` (usados pelos wrappers Go modernos da Fase 8a) não levam o prefixo `/rest`; a collection cataloga exclusivamente rotas `/rest/appliance/...` (legacy). São 2 gerações de API distintas para a mesma capacidade lógica, não duplicados.

Exclusões reais confirmadas: 10 rotas já cobertas literalmente pela Fase 4 (`system/version`, `system/uptime`, 8 subsistemas de `health`) + 2 rotas de plumbing de sessão (`Authentication` Login/Logout, o mesmo endpoint `/rest/com/vmware/cis/session` já gerido internamente por `vmware.Client.REST(ctx)` desde a Fase 4). 132 rotas − 12 excluídas − 4 (colapso de pares enable/disable de Access legacy, mesma rota PUT documentada 2× no Postman) = **116 tools reais**.

### 4 grupos paralelos

1. **recovery-update (31):** Backup/Restore + Update do vCenter Server Appliance. Tier1 em `backup_schedule_delete`, `restore_job_create` (restaurar é efectivamente irreversível), `update_install`, `update_stage_and_install` (upgrade real e irreversível da versão do vCenter).
2. **network-health-system (19):** Health/lastcheck + Monitoring + Networking DNS/interfaces + System storage/time. Criou `applianceRequest(ctx, client, method, path, query, body)`, generalização do `applianceGet` da Fase 4 (que só suportava GET). Tier1 em `system_storage_resize` (expansão de disco é de sentido único).
3. **techpreview-network (27):** Firewall/IPv4/IPv6/Proxy/Routes/NTP/Timesync, todos sob `/techpreview/` (API não-documentada — mesmo aviso já usado para `AuthorizationManager.{Disable,Enable}Methods` na Fase 7). Confirmou por grep (repo + módulo `govmomi@v0.55.1` pinado) que "techpreview" não tem nenhum SDK Go em lado nenhum; corpos de request confirmados campo-a-campo contra os exemplos reais da própria collection.
4. **services-accounts-vmon-access-shutdown (39, em 2 ficheiros):** SNMP/Services/System-update/Local-accounts/Vmon + Access legacy + Shutdown techpreview. Tier1 em `snmp_reset` (perda irreversível de configuração) e `local_accounts_delete`. Resolveu uma colisão real de símbolo Go (`vamiCapture`) com o grupo network-health-system, renomeando o seu próprio tipo de teste. Nomeou as 8 tools de Access legacy com sufixo `_legacy_` explícito para não colidir/confundir com as tools modernas `vmware_appliance_access_*` já registadas na Fase 8a.

3 colisões transientes de símbolo/build entre os 4 grupos paralelos (mesmo padrão já visto nas Fases 6-8a) — todas auto-resolvidas pelos próprios subagents.

## Evidências

```
$ cd src && gofmt -l . && go build ./... && go vet ./...
(sem output — limpo)
$ go clean -testcache && go test ./... -count=1 -timeout 300s
ok  	github.com/cslsoftwares/mcpvmware/tools	78.641s
ok  	github.com/cslsoftwares/mcpvmware/vmware	5.927s
$ go test ./... -count=1 -timeout 300s   # 2ª corrida, sem flake
ok  	github.com/cslsoftwares/mcpvmware/tools	87.290s
ok  	github.com/cslsoftwares/mcpvmware/vmware	4.641s
```

Smoke real do binário compilado (`mcpvmware-mcp.exe`), stdio JSON-RPC contra `simulator.VPX()`: `tools/list` devolveu **611 tools**; `vmware_appliance_recovery_backup_job_list`/`vmware_appliance_techpreview_firewall_list` (novos) chamados e confirmaram login REST bem-sucedido seguido de 404 real e limpo (vcsim serve o login de sessão desde a Fase 8a, mas não tem nenhuma rota `/rest/appliance/*`), comportamento correcto e esperado.

Narrativa completa em `.workspace/plans/MCPVMWare2026-08-10-175300-plano-cobertura-completa-api-codegen.plan.md` §"Fase 8b executada". Aprendizados reutilizáveis (`/api` vs `/rest` são gerações distintas; técnica de geração sem fonte Go viável) em `.wolf/cerebrum.md`.

Ficheiros novos: `src/tools/generated_vami_{recovery_update,network_system,techpreview_network,services_accounts_vmon,access_shutdown}.go(+test)`. Ficheiros alterados: `src/tools/registry.go` (wiring, 5 funções), `src/tools/mode_test.go` (catálogo canónico de 611 tools).

**Sem commits** — não pedido pelo usuário nesta rodada.

## Recomendações / próximos passos

1. Usuário autorizou ("ok, finalizar 100%") avançar directo para a Fase 9 (VMware Workstation Pro, `vmrest`, arquitectura própria hand-written) sem pausar para aprovação intermédia — a Fase 8 (8a+8b) fecha aqui por completo.
2. Pendências documentadas e não bloqueantes (numeradas):
   1. Domínio inteiro sem nenhuma verificação funcional real possível — nem vcsim, nem host real (`10.100.2.54`, ESXi standalone sem VAMI). Testado só via fixture `httptest`.
   2. Vários corpos de request de shape complexo/incerto (`recurrence_info`, `retention_info`, `policy`, `user_data`) aceites como passthrough JSON genérico — sem confirmação possível dos sub-campos exactos sem acesso a uma VAMI real.
   3. Rotas `techpreview/*` (60 das 116 tools desta fase) são API não-documentada da VMware, sujeita a mudar ou desaparecer sem aviso entre versões do vCenter — avisado explicitamente em cada tool.
3. A técnica de geração via parse de collection Postman (estabelecida nesta fase) fica disponível como precedente reutilizável para a Fase 9 (VMware Workstation Pro, `vmrest`) e a Fase 10 (VMware Cloud on AWS) — ambas já partem de collections Postman vendorizadas, sem SDK Go nenhum.

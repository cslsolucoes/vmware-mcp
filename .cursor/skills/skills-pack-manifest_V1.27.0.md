# Skills Pack — Manifesto Canônico

**FolderVersion:** 1.27.0 · **Data:** 09/08/2026
**Política de versionamento:** [../VERSION.md](../VERSION.md)

## Contagens

| Métrica | Valor |
|---------|-------|
| **Skills ativas** | **239** |
| **Skills físicas** | **247** |
| **Agents** | 37 |
| **Commands** | 13 |

---

## Delta E16 — GoLang Kit Complete (09/08/2026)

### 30 novas skills criadas

| Família | Responsabilidade | Quantidade |
|---------|------------------|-----------|
| `developer-go-language-*` | Linguagem core (tipos, OOP, advanced, generics, RTTI) | 6 skills |
| `developer-go-patterns-*` | Padrões (behavioral, composition, creational, structural) | 4 skills |
| `developer-go-stdlib-*` | STL (collections, encoding, rtti-reflection, strings-io) | 4 skills |
| `developer-go-concurrency-*` | Concorrência (basics, advanced) | 2 skills |
| `developer-go-performance-*` | Performance (memory, profiling) | 2 skills |
| `developer-go-testing` | Testes DUnitX-equivalente | 1 skill |
| `developer-go-error-handling-and-diagnostics` | Tratamento de erros | 1 skill |
| `developer-go-build-toolchain` | Build/toolchain | 1 skill |
| `developer-go-packaging-delivery` | Packaging/deployment | 1 skill |
| `developer-go-crypto-security` | Criptografia/segurança | 1 skill |
| `developer-go-architecture-and-design` | Arquitetura | 1 skill |
| `developer-go-cli-apps` | CLI apps | 1 skill |
| `developer-go-http-client-rest` | HTTP client/REST | 1 skill |
| `developer-go-http-server` | HTTP server | 1 skill |
| `developer-go-database-access` | Database access | 1 skill |
| `developer-go-linux-deploy` | Linux deployment | 1 skill |
| `developer-go-master-orchestrator` | Orquestrador master Go | 1 skill |
| `developer-go-project-spec` | Especificação projeto Go | 1 skill |

**Net E16:** +30 ativas / +30 físicas (30 skills novas, espelhando a família `developer-delphi-to-fpc-*` — kit GoLang completo).
**Trigger:** terceiro kit de linguagem consolidado (após Delphi/FPC e VueJS) — cobertura total de Go/GoLang: linguagem, patterns, stdlib, concorrência, performance, qualidade, build, arquitetura, CLI, HTTP, database, crypto, deploy, project-spec.
**Agent associado:** novo agent `developer-golang-agent-orchestrator_V1.0.0`.

---

## Delta E15 — FireDAC Skills Removal (27/06/2026)

### Remoção de 4 skills descontinuadas

**Net E15:** −4 ativas / −4 físicas.

---

## Delta E14 — GestorERP Business Rules Taxonomy (18/05/2026)

### Bump in-place (1 skill — rename de pasta V3.1.0 → V3.2.0)

| Skill | Versão antes | Versão depois | Mudanças principais |
|-------|--------------|----------------|---------------------|
| `documentation-business-rules` | V3.1.0 | **V3.2.0** | Taxonomia ampliada para 9 prefixos M/S/B/A/I/Z/C/G/T (decisões D01-D25 do GestorERP). Naming adaptado para sub-letras ZXX (D24): `RN-Z01.a-001_Z01.a_V1_0.md`. Cabeçalho ganha 5 campos novos: Taxonomia, Origem (legado), Origem (GDoc), URL preservada (D25), Multi-tenant (D19). Tabela TABELAS/CAMPOS BD ganha coluna `tenant_id (D19)`. Protocolo de migração de RNs legadas M01-M33 documentado. Exemplos novos: `Padrao_RN-S01-001_exemplo.md` (taxonomia nova) + `Padrao_RN-Z01.a-001_exemplo.md` (sub-letras D24). Exemplos legados M01/M05 preservados. Path canónico atualizado de `Documentation/Regras de Negocio/` (genérico) → `Documentation/RegrasNegocio/` (convenção GestorERP — sem espaços). |

**Net E14:** 0 ativas / 0 físicas (1 bump in-place via cópia V3.1.0 → V3.2.0 + remoção da V3.1.0).
**Trigger:** alinhamento da skill canónica de RNs com a nova taxonomia M/S/B/A/I/Z/C/G/T do GestorERP (FASE 0 da migração GDoc→GestorERP — D01-D25 do plano master v1.4.0).
**Backup:** `.workspace/Backup/manual/pre-skill-business-rules-bump_2026-05-18/documentation-business-rules_V3.1.0/`.

---

## Delta E13 — Documentation Quality Gates (26/04/2026)

### Bump in-place de 5 skills (rename de pasta + edição do SKILL.md)

| Skill | Versão antes | Versão depois | Mudanças principais |
|-------|--------------|----------------|---------------------|
| `documentation-master-orchestrator` | V1.1.0 | **V1.2.0** | Workflow obrigatório de 5 fases (scan → bootstrap → coverage-plan → geração → coverage-final); 7 arquivos canônicos em `Documentation/Decisions/`; 3 novos anti-padrões; nova entrada na matriz |
| `documentation-project-bootstrap` | V2.1.0 | **V2.2.0** | Parâmetros `<output_path>`, `<structure_mode>`, `<portal_html>`; novo passo 5 cria `Documentation/Decisions/`; 4 novos anti-padrões; 4 novos critérios de aceite |
| `documentation-project-scan` | V1.1.0 | **V1.2.0** | Novo passo 4: cruzamento dependências vs imports (Python/Node/Pascal/Rust/Go/Java); inventário de unidades de código; gera `DEPENDENCY_GAPS.md`; 2 novos anti-padrões |
| `documentation-general_rules` | V2.0.0 | **V2.1.0** | Formaliza os 7 arquivos canônicos em `Documentation/Decisions/` com origem por skill; 2 novos anti-padrões |
| `documentation-class-analysis-generator` | V1.1.0 | **V1.2.0** | Threshold de agregação dura: ≥5 unidades = doc individual obrigatória; 2–4 exigem `AGGREGATION_RATIONALE.md`; 2 novos anti-padrões |

**Net E13:** 0 ativas / 0 físicas (5 bumps in-place, sem nova pasta).
**Trigger:** lacunas detectadas durante documentação Fase 1 do GDoc — agente fez agregação indevida e pulou gates.
**Backup:** `.cursor/Backup/skills/<timestamp>/` antes da edição.

---

## Delta E12 — Indy Completion (25/04/2026)

### Enriquecimento in-place (2 skills existentes)

| Skill | Seções adicionadas |
|-------|-------------------|
| `developer-delphi-indy-http_V1.0.0` | §9 PATCH/HEAD · §10 progress events (OnWork) · §11 response headers + cookies (TIdCookieManager) · §12 download assíncrono com TTask · §13 TIdHTTPServer (servidor local/webhook/OAuth callback) · §14 checklist atualizado |
| `developer-delphi-indy-email_V1.0.0` | §8 decodificar mensagem recebida (partes MIME) · §9 extrair e salvar anexos · §10 Reply/Forward com headers de threading (InReplyTo, References) · §11 envio em lote com reconexão · §12 checklist atualizado |

### 2 novas skills criadas

| Skill | Responsabilidade |
|-------|-----------------|
| `developer-delphi-indy-ftp_V1.0.0` | TIdFTP: autenticação FTP/FTPS (explícito + implícito), upload/download, listagem, operações em diretórios, renomear, sincronização incremental, progress events |
| `developer-delphi-indy-tcp_V1.0.0` | TIdTCPClient, TIdTCPServer multi-thread, TIdCmdTCPServer, framing length-prefix, heartbeat com TTask, SSL/TLS sobre TCP, broadcast para clientes conectados |

**Net E12:** +2 ativas / +2 físicas (2 enriquecimentos in-place, sem nova pasta)

---

## Delta E11 — Vue.js Skills Improvement (anterior)

+6 ativas / +10 físicas. Pack antes: 205 ativas / 209 físicas.

---

## Delta E10 — Plugin Absorption (anterior)

+5 skills · +1 enriquecida · +4 agents · +6 commands.

---

## Delta E9 — Gaps Resolution (anterior)

+14 skills novas.

---

## Validação

```
Checks: 1548  |  Passed: 1548  |  Issues: 3
[MODERATE] (2): .claude/VERSION.md SYMLINK ausente · .vscode/VERSION.md SYMLINK ausente
[LOW]      (1): .continue/ não existe
CRITICAL: 0
```

---

## Changelog deste arquivo

- 1.27.0 (09/08/2026): **E16 — GoLang Kit Complete**: +30 skills novas família `developer-go-*` (linguagem core 6, patterns 4, stdlib 4, concorrência 2, performance 2, testing 1, error-handling 1, build 1, packaging 1, crypto 1, arquitetura 1, cli-apps 1, http-client-rest 1, http-server 1, database-access 1, linux-deploy 1, master-orchestrator 1, project-spec 1). Espelha estrutura `developer-delphi-to-fpc-*` — terceiro kit consolidado (após Delphi/FPC + VueJS). Net: **+30 ativas / +30 físicas**. Pack: **239 ativas / 247 físicas**. Novo agent: `developer-golang-agent-orchestrator_V1.0.0`. Novo blueprint: `kit-go_V1.0/` (README.md + SPEC.md).
- 1.26.0 (27/06/2026): **E15 — Remoção da família FireDAC (data-access)**: removidas **4 skills** que ensinavam acesso direto a `TFDConnection`/`TFDQuery`/`ExecSQL` — `developer-delphi-firedac-{connection,queries,transactions,orchestrator}_V1.0.0`. Racional: no GestorERP o acesso a dados é **exclusivamente via ProvidersORM** e o engine é escolhido por **diretiva de compilação** (`-DUSE_ZEOS`/`USE_*`), nunca FireDAC direto. 12 referências órfãs em 7 skills (coding-workflow, json-serialization, reporting-fastreport, vcl-{components,forms,orchestrator}, quality-security-audit) redirecionadas para `developer-delphi-providers-orm-usage`. Net: **−4 ativas / −4 físicas**. Pack: **209 ativas / 217 físicas**. Backup: `.workspace/Backup/manual/firedac-skills-removal_2026-06-27/`. Ver guia `Documentation/RegrasNegocio/000-Banco_de_Dados/GestorERP_000-Banco_de_Dados_ProvidersORM_UsageGuide_V1_0_0.md`.
- 1.25.0 (26/04/2026): **E13 — Documentation Quality Gates**: 5 bumps in-place — `documentation-master-orchestrator` V1.1→V1.2 (workflow obrigatório de 5 fases), `documentation-project-bootstrap` V2.1→V2.2 (`<output_path>`, `<structure_mode>`, `<portal_html>`, `Decisions/`), `documentation-project-scan` V1.1→V1.2 (cruzamento deps↔imports, `DEPENDENCY_GAPS.md`), `documentation-general_rules` V2.0→V2.1 (7 arquivos canônicos em `Decisions/`), `documentation-class-analysis-generator` V1.1→V1.2 (threshold ≥5 = doc individual). Net: 0 ativas / 0 físicas. Pack: **213 ativas / 221 físicas** (inalterado).
- 1.24.0 (25/04/2026): **E12 — Indy Completion**: HTTP enriquecido (§9-§14: PATCH/HEAD, progress, cookies, TTask, TIdHTTPServer); Email enriquecido (§8-§12: decode MIME, anexos, reply/forward, lote); +2 skills novas (indy-ftp, indy-tcp). Net: +2 ativas / +2 físicas. Pack: **213 ativas / 221 físicas**.
- 1.23.0 (24/04/2026): E11 Vue.js: +6 ativas / +10 físicas. Pack: 211 ativas / 219 físicas.
- 1.22.0 (24/04/2026): E10 Plugin absorption: +5 skills, +1 enriquecida, +4 agents, +6 commands. Pack: 205 ativas / 209 físicas.
- 1.21.0 (24/04/2026): E9 Gaps Resolution: +14 skills. Pack: 200 ativas / 204 físicas.
- 1.20.0 (17/04/2026): Pós-split D3: 186 ativas / 190 físicas.

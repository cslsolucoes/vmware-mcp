# Patch — Kit de Skills GoLang (`developer-go-*`)

**Data:** 09/08/2026 · **Autor:** Claude (Sonnet 5), sob pedido do owner · **Tipo:** MINOR (aditivo, retrocompatível)

## Contexto

O workspace `MCPVMWare` não tinha `*.dpr`/`*.lpr` e o fluxo `/iniciar` só sabe
gerar projetos Delphi/FPC. O usuário pediu explicitamente a criação de uma
família de skills genéricas para GoLang em `.cursor/`, no mesmo padrão da
família `developer-delphi-to-fpc-*`, incluindo geração de SPEC completo e
registro como kit de primeira classe (agent + CEO). Plano completo aprovado
via plan mode antes da execução (ver `.workspace`/histórico de sessão).

## O que foi criado

### 30 skills novas em `.cursor/skills/developer-go-*_V1.0.0/`

| Família | Skills |
|---|---|
| Linguagem (5) | `language-core`, `language-types`, `language-generics`, `language-oop`, `language-advanced` |
| Patterns (4) | `patterns-creational`, `patterns-structural`, `patterns-behavioral`, `patterns-composition` |
| Stdlib (4) | `stdlib-collections`, `stdlib-strings-io`, `stdlib-encoding`, `stdlib-rtti-reflection` |
| Concorrência/Performance (4) | `concurrency-basics`, `concurrency-advanced`, `performance-and-memory`, `performance-profiling` |
| Qualidade (2) | `testing`, `error-handling-and-diagnostics` |
| Build/Delivery (3) | `build-toolchain`, `packaging-delivery`, `crypto-security` |
| Arquitetura/Apps (5) | `architecture-and-design`, `cli-apps`, `http-client-rest`, `http-server`, `database-access` |
| Deploy (1) | `linux-deploy` |
| Orquestração/Spec (2) | `master-orchestrator`, `project-spec` (+ `references/spec-template.md` e `.en.md`) |

Todas seguem `SKILL_TEMPLATE_V2.0.md` (17 seções + bloco Go dedicado: Stack e
versões, Dependências `go.mod`/`go get`, Checklist Go, Exemplo mínimo
compilável), `category: developer-go`, `model: sonnet` (leaf) ou `opus`
(master-orchestrator, project-spec), pasta `_V1.0.0` == `FileVersion`.

### Agent orquestrador + registro no CEO

- **Novo:** `.cursor/agents/developer-golang-agent-orchestrator_V1.0.0.md`
  (mirror de `developer-vuejs-agent-orchestrator_V1.2.0.md`; boundary `*.go`,
  `go.mod`, `go.sum`, `cmd/`, `internal/`, `pkg/`).
- **Renomeado:** `developer-agent-orchestrator_V2.3.0.md` →
  `developer-agent-orchestrator_V2.4.0.md` (CEO) — novas linhas nas tabelas
  "Sub-orquestradores (nível 2)" e "Classificação por extensão/contexto" para
  o kit Go; prosa atualizada para citar os 3 kits (Delphi/FPC + VueJS/NodeJS +
  Go/GoLang).

### Blueprint do kit

- **Novo:** `.cursor/Templates/kit-go_V1.0/README.md` (layout `cmd/`/`internal/`/`pkg/`, regras mínimas).
- **Novo:** `.cursor/Templates/kit-go_V1.0/SPEC.md` — SPEC completo (14 seções,
  33 RF, 6 UC, 7 RNF, 12 RN, 5 US, 4 AT, 4 INT) gerado aplicando a lógica de
  `developer-go-project-spec` reflexivamente ao próprio kit.
- **Editado:** `.cursor/Templates/README.md` — linha nova na tabela de kits.

### Governança/inventário

- `skills-pack-manifest_V1.26.0.md` → `V1.27.0.md` (nova entrada: família
  `developer-go-*`, 30 skills).
- `agents-pack-manifest_V1.7.1.md` → `V1.7.2.md` (novo agent Go + rename do CEO).
- Referências obsoletas a `developer-agent-orchestrator_V2.3.0.md` corrigidas
  para `V2.4.0.md` em `.cursor/agents/README.md`,
  `.cursor/plans/audit/L20-agents-developer.md` e `.cursor/pack-inventory.json`.

## Correções pós spot-check

Durante a revisão manual (leitura direta de 4 `SKILL.md` + varredura
automatizada de referências cruzadas `developer-go-*` em 33 arquivos),
encontradas e corrigidas 2 classes de referência quebrada geradas por agentes
paralelos que citaram nomes de skills nunca criadas:

- `developer-go-testing-and-quality` (inexistente) → `developer-go-testing`,
  em `developer-go-language-core` e `developer-go-error-handling-and-diagnostics`.
- `developer-go-project-audit` (inexistente, fora do escopo aprovado) →
  substituído por `quality-code-review-checklist`/`quality-tech-debt-tracker`
  (skills reais do pack), em `developer-go-project-spec` (3 ocorrências).

Segunda varredura confirmou **zero** referências `developer-go-*` quebradas
nos 30 SKILL.md + agent + docs do kit.

## Verificação

- `python .cursor/scripts/validate_pack.py`: **1719/1724 checks OK**. Os 5
  issues remanescentes são **pré-existentes**, sem relação com este patch
  (`developer-delphi-project-audit` com mismatch de versão; 3 `commands/*.md`
  sem `name`; `rules-pack-manifest` ausente).
- `bootstrap-mirror-symlinks.ps1 -ValidateOnly`: todos os checks passaram
  (espelhos `.claude/`/`.vscode/` íntegros após a criação de 30+ pastas novas).

## Fora de escopo (não incluído neste patch)

- Geração do projeto Go real deste workspace (`go.mod`/`main.go` do
  MCPVMWare) — pendente desde o pedido original de `/iniciar`.
- Script `bootstrap-build-config-go.ps1` para o fluxo `/iniciar` reconhecer Go
  como opção de framework (P3).

---

## Versão interna (arquivo)

| Campo | Valor |
|-------|-------|
| **FileVersion** | 1.0.0 |
| **Política** | `.cursor/VERSION.md` |

## Changelog (este arquivo)

- 1.0.0 (09/08/2026): Criação do patch — kit de skills GoLang completo (30
  skills + agent + registro no CEO + blueprint + SPEC + manifests).

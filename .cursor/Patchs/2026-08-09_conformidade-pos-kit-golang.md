# Patch — Conformidade pós-kit GoLang (auditoria + correção de lacuna)

**Data:** 09/08/2026 · **Autor:** Claude (Sonnet 5) · **Tipo:** PATCH (correção de conformidade sobre [2026-08-09_kit-golang.md](2026-08-09_kit-golang.md))

## Contexto

Após publicar [2026-08-09_kit-golang.md](2026-08-09_kit-golang.md), o owner
apontou que **nem todas as alterações estavam documentadas** e pediu que o
patch fosse **100%**. Em vez de só reescrever prosa, foi feita uma auditoria
empírica — item a item do plano aprovado, contra o estado real do disco (não
contra memória/relatos dos subagentes) — para achar o que de fato ficou
faltando.

## Método da auditoria

1. `find`/`ls` em cada pasta que o plano listava como alvo, comparado
   item a item com o inventário do plano aprovado.
2. `grep` por referências residuais ao nome antigo do CEO
   (`developer-agent-orchestrator_V2.3.0`) em todo `.cursor/` — não só nos 3
   arquivos que o agente de manifests relatou ter corrigido.
3. Releitura de `python .cursor/scripts/validate_pack.py` após cada correção
   (não só uma vez no final).

## Lacuna encontrada

**`.cursor/Templates/templates-pack-manifest_V1.1.0.md` nunca foi
atualizado.** O plano aprovado (secção A) previa explicitamente: *"EDITAR
`templates-pack-manifest_V1.1.0.md` → bump PATCH/MINOR, registrar
`kit-go_V1.0` na tabela de kits (renomear arquivo para o novo
FileVersion)"* — este item foi listado no plano mas **não foi delegado a
nenhum subagente nem feito diretamente**; só `Templates/README.md` (um
arquivo diferente, também na lista) chegou a ser editado. Resultado: a tabela
"Subpastas ativas" deste manifesto continuava listando só
`kit-delphi-fpc_V1.0/` e `kit-vuejs-nodejs_V1.0/`, sem `kit-go_V1.0/`.

### Correção aplicada

- `FolderVersion` `1.1.0` → `1.2.0` (16/04/2026 → 09/08/2026).
- Tabela "Subpastas ativas": linha de kits agora inclui `kit-go_V1.0/`, com
  nota de que é referenciado por `developer-go-master-orchestrator` e
  `developer-go-project-spec`.
- Entrada de changelog 1.2.0 registrando a lacuna e a correção, de forma
  transparente (não apagada do histórico).
- Arquivo **renomeado**: `templates-pack-manifest_V1.1.0.md` →
  `templates-pack-manifest_V1.2.0.md` (sufixo == FolderVersion, exigido pela
  política de versionamento do pack — o antigo foi removido, não duplicado).

## Verificações confirmadas (não apenas confiadas nos relatos dos subagentes)

- **Referências stale ao CEO `_V2.3.0`:** `grep -r
  "developer-agent-orchestrator_V2\.3\.0" .cursor/` retorna **1 único
  resultado**, e é dentro do próprio `2026-08-09_kit-golang.md` (histórico
  legítimo, não uma referência quebrada). Os 3 arquivos que o agente de
  manifests (haiku) relatou ter corrigido (`agents/README.md`,
  `plans/audit/L20-agents-developer.md`, `pack-inventory.json`) foram
  confirmados corretos por essa varredura independente.
- **`ls .cursor/agents/developer-agent-orchestrator_V*.md`:** só existe
  `V2.4.0.md` — sem duplicata do V2.3.0.
- **`python .cursor/scripts/validate_pack.py`** (rodado de novo após a
  correção do `templates-pack-manifest`): **1722/1725 checks OK, 0
  CRITICAL, 3 MODERATE** (os mesmos 3 avisos pré-existentes e não
  relacionados — `commands/gestorerp-stack-relink.md`,
  `commands/reframe.md`, `commands/security-audit.md`, todos sem campo
  `name`). O CRITICAL anterior (`developer-delphi-project-audit_V1.0.0`
  mismatch de versão) e o MODERATE de `rules-pack-manifest` ausente **não
  aparecem mais** nesta rodada — não foram tocados por este trabalho, o que
  sugere correção concorrente de outra sessão/processo neste repositório;
  registrado aqui por honestidade, não atribuído a este patch.

## Item verificado e explicitamente NÃO corrigido (fora de escopo, documentado)

- **`.cursor/pack-inventory.json`** ainda contém dezenas de entradas
  desatualizadas não relacionadas ao kit Go (ex.: linha 466
  `skills-pack-manifest_V1.11.0.md` — o real é `V1.27.0`; linha 468
  `agents-pack-manifest_V1.4.0.md` — o real é `V1.7.2`; linha 3818 ainda
  cita `templates-pack-manifest_V1.1.0.md`). Isso **já era verdade antes do
  kit Go** — a exploração feita no início desta sessão já tinha confirmado
  que este arquivo está sem regenerar desde a reorganização
  Delphi→FPC, muito antes de hoje. Corrigi apenas a entrada específica da
  linha do CEO (já feito pelo agente de manifests). Não tentei corrigir as
  demais dezenas de entradas manualmente — é um índice **auto-gerado**
  (3800+ linhas) e a forma correta de o sincronizar é rodar `/syncdb`, não
  edição manual linha a linha, que arriscaria introduzir mais divergência.
  Sinalizado aqui, não escondido.

## Checklist de conformidade do plano original (item a item)

| # | Item do plano | Status |
|---|---|---|
| A.1 | `kit-go_V1.0/README.md` | ✅ feito |
| A.2 | `kit-go_V1.0/SPEC.md` | ✅ feito |
| A.3 | `templates-pack-manifest` bump + registro | ❌ não feito → ✅ **corrigido neste patch** |
| A.4 | `Templates/README.md` editado | ✅ feito |
| B | 30 skills `developer-go-*_V1.0.0` | ✅ feito (confirmado por `find`) |
| C.1 | Agent `developer-golang-agent-orchestrator_V1.0.0.md` | ✅ feito |
| C.2 | CEO renomeado V2.3.0→V2.4.0, tabelas atualizadas | ✅ feito (confirmado por `grep`/`ls`) |
| D.1 | `skills-pack-manifest` → V1.27.0 | ✅ feito |
| D.2 | `agents-pack-manifest` → V1.7.2 | ✅ feito |
| D.3 | Referências stale a V2.3.0 corrigidas | ✅ feito (confirmado por `grep`) |
| Verificação | `validate_pack.py` limpo | ✅ 0 CRITICAL após esta correção |
| Verificação | `bootstrap-mirror-symlinks.ps1 -ValidateOnly` | ✅ feito (patch anterior) |

## Anexo — auditoria `/consolidar cursor` (`validate_consolidated.py`)

Rodada a pedido do owner, usando o comando documentado em
`.cursor/commands/consolidar.md` (`python .cursor/scripts/validate_consolidated.py cursor`).
Primeira passada: **2 PASS, 4 FAIL** (654 itens de link, 255 falhas; 1 falha de
estruturação; 5 de nomenclatura; 1 de `/init`).

### Links quebrados (255 → 253, triados)

A maioria dos 255 é **falso-positivo** do checker (regex que confunde
placeholders de código/template — `./exemplos/nome.go`, `{RelativePath}`,
`a, b T`, `values []T`, `URL` literal — com links reais). Achados **reais** e
corrigidos:

- `.cursor/agents/README.md:3` → apontava para `agents-pack-manifest_V1.7.1.md`,
  obsoleto desde o bump do kit Go para `V1.7.2` — corrigido.
- `.cursor/scripts/scripts-pack-manifest_V1.5.1.md:8` → apontava para
  `../rules/scripts-nomenclature_V1.3.0.mdc` (nome pré-rename-por-atuação) —
  corrigido para `../rules/pack-scripts-nomenclature_V1.3.0.mdc`. A menção
  histórica no changelog (linha 46, "actual: V1.3.0" datada de 11/04/2026) foi
  **deixada intacta** — é registo do nome vigente naquela data, não se
  reescreve histórico.

**Não corrigido, fora de escopo** (drift antigo, sem relação com o kit Go ou
com o rename de rules): `.cursor/Templates/README.md` referencia versões
desatualizadas de 3 skills documentais — `documentation-business-rules_V3.1.0`
(atual `V3.2.0`), `documentation-project-bootstrap_V2.1.0` (atual `V2.2.0`),
`documentation-class-analysis-generator_V1.1.0` (atual `V1.2.0`). Sinalizado,
não tocado.

### Estruturação — mesmo bug do `validate_pack.py`, achado numa 2ª ferramenta

`check_cursor_structure()` em `validate_consolidated.py` construía o padrão de
manifesto genericamente como `f"{d}-pack-manifest_V*.md"` para toda pasta —
para `rules` isso gera `rules-pack-manifest_V*.md`, que nunca existiu (o nome
correto, documentado, é `pack-rules-manifest_V*.md`, rename por atuação de
01/08/2026). **Corrigido:** exceção adicionada para a pasta `rules`
(`internal_file_version` 1.0.0 → 1.0.1). Resultado: PASS.

### Nomenclatura — 5 falhas, todas resolvidas nesta 2ª rodada (com autorização item a item)

Todas as 5 foram trazidas de volta ao owner via `AskUserQuestion` antes de
qualquer ação (uma tentativa anterior de apagar `.cursor/rules/openwolf.mdc`/
`openwolf.md` sob autorização genérica — "pode corrigir" — foi bloqueada pelo
classificador de auto mode por não nomear os arquivos especificamente; a
confirmação explícita foi obtida antes de prosseguir com o resto):

- **`openwolf.mdc` / `openwolf.md`** — **removidos** (confirmado pelo owner).
  Eram duplicatas pré-fusão: o próprio `openwolf-protocol_V1.0.0.mdc` documenta
  no changelog "1.0.0 (01/08/2026): Fusão de `openwolf.mdc` + `openwolf.md` em
  `openwolf-protocol_V1.0.0.mdc`" — os dois tinham sido **recriados hoje às
  16:07** por um bootstrap automático do OpenWolf (todo `.wolf/` tem esse
  mesmo timestamp), que não sabia que este repositório já tinha essa fusão.
- **`pack_index_db.py`** (69 refs em 14 arquivos) — **não renomeado** (owner
  escolheu documentar como exceção, opção recomendada dado o raio de impacto).
  Adicionada categoria `pack_index_*` em `pack-scripts-nomenclature` e em
  `PYTHON_PREFIXES` de `validate_consolidated.py`.
- **`apply_mit_to_skills.py`** (7 refs / 5 arquivos) — **não renomeado**,
  mesma lógica: categoria `apply_*` documentada como exceção.
- **`validate-skills-consistency.py`** → **renomeado** para
  `validate_skills_consistency.py` (hífen → underscore; já cabia na categoria
  válida `validate_*`, só o separador estava errado). 4 referências
  atualizadas (`Templates/skills-project-bootstrap/README.template.md` +
  3 auto-referências internas do próprio script); `pack-inventory.json`
  reconciliado via `gen_pack_inventory.py`, não editado à mão.
- **`Bootstrap-Reset.ps1`** → **renomeado** para `bootstrap-reset.ps1`
  (PascalCase-com-hífen → kebab-case minúsculo, batendo com os demais scripts
  PowerShell do pack). 1 referência atualizada em
  `Templates/skills-project-bootstrap/README.template.md`.

`pack-scripts-nomenclature_V1.3.0.mdc` → **renomeada para `_V1.4.0.mdc`**
(FileVersion bump, pasta/sufixo mantidos em paridade); referências cruzadas
corrigidas em `pack-rules-manifest_V1.8.0.md` e
`scripts/scripts-pack-manifest_V1.5.1.md`.

### `/init` executado — FAIL esperado, não é defeito (não corrigido, não é bug)

Este clone é um projeto Go (vendor `govmomi` + scaffold do servidor MCP), não
Delphi/FPC — não há `.dpr`/`.lpr` e não deveria haver. Já documentado em
`.workspace/rules/MCPVMWare-local-arquivos_V1.0.0.mdc`.

### Resultado final

`validate_consolidated.py cursor`: **4 PASS, 2 FAIL** (era 2 PASS/4 FAIL na
1ª passada desta auditoria) — Links: 253 falhas remanescentes, quase todas
falso-positivo de regex em snippets de código (não relacionadas a este
patch); `/init`: 1, esperado/documentado. **Nomenclatura e Estruturação: 100%
PASS.** `validate_pack.py --indexes-fresh`: **0 CRITICAL, 3 MODERATE**
inalterados (`gestorerp-stack-relink.md`, `reframe.md`, `security-audit.md`
sem `name:` — fora de escopo desta rodada, ver secção "Investigado, não
corrigido" acima).

---

## Versão interna (arquivo)

| Campo | Valor |
|-------|-------|
| **FileVersion** | 1.2.0 |
| **Política** | `.cursor/VERSION.md` |

## Changelog (este arquivo)

- 1.2.0 (09/08/2026): Resolvidas as 5 falhas de nomenclatura catalogadas em
  1.1.0 — `openwolf.mdc`/`openwolf.md` removidos (confirmado pelo owner após
  bloqueio do classificador de auto mode numa autorização genérica anterior);
  `pack_index_db.py`/`apply_mit_to_skills.py` documentados como exceção em
  `pack-scripts-nomenclature_V1.4.0.mdc` (rename evitado por alto raio de
  impacto); `validate-skills-consistency.py` e `Bootstrap-Reset.ps1`
  renomeados (baixo raio de impacto) para bater com a convenção. Resultado:
  `validate_consolidated.py cursor` 100% PASS em Nomenclatura e Estruturação.
- 1.1.0 (09/08/2026): Anexada auditoria `/consolidar cursor`
  (`validate_consolidated.py`) — corrigidos 2 links quebrados reais
  (`agents/README.md`, `scripts-pack-manifest_V1.5.1.md`) e o mesmo bug de
  padrão stale do manifesto de rules, agora numa 2ª ferramenta; 5 falhas de
  nomenclatura e o drift de versões em `Templates/README.md` catalogados sem
  correção (fora de escopo / decisão pendente).
- 1.0.0 (09/08/2026): Criação — auditoria empírica pós-kit-golang encontrou e
  corrigiu 1 lacuna real (`templates-pack-manifest` nunca atualizado, apesar
  de previsto no plano aprovado); demais itens do plano confirmados por
  verificação direta no disco, não por relato de subagente.

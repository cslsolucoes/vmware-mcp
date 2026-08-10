# Versão interna — `.cursor/rules/`

**FolderVersion:** 1.8.0 · **Data:** 05/08/2026  
**Política:** [../VERSION.md](../VERSION.md)

## Nomenclatura por atuação

Formato: `<dominio>-<responsabilidade>_V<major>.<minor>.<patch>.mdc` (manifesto: `.md`).

| Prefixo | Atuação |
| ------- | ------- |
| `pack-` | Governação do pack `.cursor/` |
| `workspace-` | Instãncia / persistência `.workspace/` |
| `project-` | Ciclo de vida do repositório |
| `documentation-` | Políticas documentais |
| `backend-pascal-` | Convenções Pascal |
| `openwolf-` | Protocolo OpenWolf |
| `commons-` / `cross-compiler-` / `fpc-` | Procedimentos ORM/FPC (promovidos 05/08/2026) |

## Rules activas

| Ficheiro | AlwaysApply / globs | Papel |
| -------- | ------------------- | ----- |
| `pack-artifact-placement_V1.3.0.mdc` | always | Classificação `.cursor/` vs `.workspace/` |
| `pack-inventory-autoupdate_V1.0.0.mdc` | globs `.cursor/**` | Reindex `/syncdb` |
| `pack-scripts-nomenclature_V1.4.0.mdc` | globs `.cursor/scripts/**` | Nomes de scripts |
| `pack-rules-manifest_V1.8.0.md` | — | Este manifesto |
| `workspace-plans-persist_V1.4.0.mdc` | always | Planos/reports/skills/rules/agents do projecto → `.workspace/` |
| `workspace-project_V1.1.1.mdc` | always | Stub → instãncia `<projectId>-local-arquivos_V*` |
| `project-bootstrap-autostart_V1.2.0.mdc` | always | Bootstrap espelhos / `/init` |
| `project-documentation_V1.0.1.mdc` | template | Documentação do projecto (placeholders) |
| `documentation-migration-plan-mode_V1.2.0.mdc` | always | Plano obrigatório em áreas protegidas |
| `documentation-file-versioning_V1.0.0.mdc` | — | Formatos de versão em `Documentation/` |
| `backend-pascal-unit-naming_V1.6.0.mdc` | always | Naming units Pascal + artefactos `.cursor/` |
| `backend-pascal-source-header_V1.2.0.mdc` | globs backend | Cabeçalho padrão Pascal |
| `backend-pascal-encoding-no-escapes_V1.0.0.mdc` | always | Sem escapes `#NNN` em strings |
| `commons-placement-policy_V1.0.0.mdc` | always | Commons nucleo vs por-modulo (API publica) |
| `cross-compiler-single-api_V1.0.0.mdc` | always | Preferir API unica Delphi+FPC vs `{$IFDEF}` duplo |
| `fpc-native-c-objects-linking_V1.0.0.mdc` | always | Link de objectos C nativos (LZMA/7z) Delphi+FPC |
| `openwolf-protocol_V1.0.0.mdc` | always | OpenWolf (`.wolf/`) |

**Commands** (`commands/`): `migration-plan.md`, `sync-cursor-pack.md`, `validate-docs.md`, `consolidar.md`, `syncdb.md`.

## Relação com `.workspace/rules/`

Instãncias do clone (não propagadas pelo sync do pack):

- Padrão: `<projectId>-<slug>_V<major>.<minor>.<patch>.mdc`
- Paths/pacotes: `<projectId>-local-arquivos_V*.mdc` (apontado por `workspace-project_V1.1.1.mdc`)
- Identidade: `context.json` → `program` → basename da raiz

## Changelog (este arquivo)


- 1.8.0 (05/08/2026): Promovidos de .workspace/rules/: commons-placement-policy_V1.0.0, cross-compiler-single-api_V1.0.0, pc-native-c-objects-linking_V1.0.0 (Opção A consolidação GestorERP).
- 1.7.0 (01/08/2026): **Nomenclatura por atuação** — renomeação em massa (`pack-*`, `workspace-*`, `project-*`, `backend-pascal-*`, `openwolf-protocol`); FileVersions alinhados nos nomes; OpenWolf fundido; inventário tabular.
- 1.6.7 (01/08/2026): Stub `local_arquivos` → `workspace_project` (depois `workspace-project`).
- 1.6.6 (01/08/2026): Alinhamento anti-contradição plans/reports / artifact-placement / local_arquivos stub.
- 1.6.5 e anteriores: ver backups em `.cursor/_backup_*/` se necessário histórico detalhado.

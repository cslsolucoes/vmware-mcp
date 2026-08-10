# Documento de Especificacao de Software (SPEC) — Kit de Skills GoLang

> Aplicacao reflexiva da skill `developer-go-project-spec_V1.0.0` sobre o proprio
> kit de skills `developer-go-*` recem-criado no pack `.cursor/`. O "produto"
> especificado aqui **nao** e um projeto Go de aplicacao, e sim o conjunto de
> artefatos de governanca (skills + agent + registro no CEO + blueprint) que
> padronizam o desenvolvimento Go dentro deste workspace.

---

## SECAO 1 — Identificacao do Projeto

| Campo | Valor |
|---|---|
| Nome do Sistema | Kit de Skills GoLang (`developer-go-*`) |
| Modulo / Subsistema | Blueprint `kit-go_V1.0` + familia de 30 skills `developer-go-*_V1.0.0` |
| Versao da SPEC | 1.0.0 |
| Data de Criacao | 09/08/2026 |
| Autor | CSL Tech Solutions (Claiton de Souza Linhares) |
| Revisado por | Tech Lead |
| Status | Aprovado |

---

## SECAO 2 — Objetivo e Escopo

### 2.1 Objetivo

Padronizar e acelerar o desenvolvimento de software em Go (GoLang) dentro do pack
`.cursor/` deste workspace, oferecendo um conjunto coeso e roteavel de skills que
cobre linguagem, stdlib, concorrencia, performance, qualidade, build/entrega,
arquitetura, apps e deploy. O kit espelha, para o ecossistema Go, o mesmo metodo
consolidado ja existente para Delphi/FPC (`kit-delphi-fpc_V1.0`) e para VueJS/NodeJS
(`kit-vuejs-nodejs_V1.0`), de modo que o agente CEO reconheca demandas Go e as
delegue a um sub-orquestrador dedicado, com validacao por `go build`/`go test`/`go vet`.
O resultado esperado e que qualquer tarefa Go seja classificada, roteada para a
skill correta e executada seguindo convencoes idiomaticas (erros como valor,
composicao sobre heranca, preferencia por stdlib), sem reinvencao de metodo.

### 2.2 Escopo (o que esta incluido)

- As **30 skills** `developer-go-*_V1.0.0` em `.cursor/skills/` (2 de coordenacao/spec,
  5 de linguagem, 4 de patterns, 4 de stdlib, 4 de concorrencia/performance,
  2 de qualidade, 3 de build/delivery, 5 de arquitetura/apps, 1 de deploy).
- O agent sub-orquestrador `developer-golang-agent-orchestrator_V1.0.0`
  (`.cursor/agents/`).
- O registro do kit Go no CEO `developer-agent-orchestrator_V2.4.0`
  (tabela de sub-orquestradores e classificacao por extensao `*.go`/`go.mod`/`go.sum`).
- O blueprint reutilizavel `kit-go_V1.0` (`.cursor/Templates/kit-go_V1.0/`), incluindo
  este `SPEC.md` e o `README.md` de estrutura.
- A propagacao dos artefatos para os espelhos (`.claude/`, `.vscode/`, `.continue/`,
  `.opencode/`) via `bootstrap-mirror-symlinks.ps1`.

### 2.3 Fora do Escopo (o que NAO esta incluido)

- Frameworks web de terceiros para Go (gin, echo, fiber, chi como dependencia
  obrigatoria) — o kit e deliberadamente stdlib-first (`net/http`, `database/sql`, `flag`).
- Desenvolvimento mobile em Go (gomobile, apps Android/iOS nativos em Go).
- ORMs de terceiros fixados como padrao (GORM, ent) — cobertos apenas na medida
  em que `database/sql` os abstrai; nenhum e prescrito.
- Qualquer skill "generica" que nao seja da familia `developer-go-*` (Delphi, Vue,
  documentacao, governanca — pertencem a outros kits).
- A infraestrutura de execucao de codigo Go (instalacao do toolchain na maquina do
  usuario) — o kit documenta versoes minimas, nao provisiona o ambiente.

---

## SECAO 3 — Atores e Perfis de Usuario

| ID | Ator | Descricao | Permissoes |
|---|---|---|---|
| AT-001 | Desenvolvedor Go | Consome as skills para escrever/refatorar codigo Go idiomatico (linguagem, stdlib, concorrencia, HTTP, DB, testes). | Ler e invocar todas as skills `developer-go-*`; editar `*.go`/`go.mod`/`go.sum` no projeto-alvo. |
| AT-002 | Tech Lead | Pede SPEC, revisa roteamento, aprova convencoes do kit e valida quality gates. | Aprovacao final de skills/SPEC; revisao de PRs; decisao sobre convencoes globais do kit. |
| AT-003 | Agent `developer-golang-agent-orchestrator` | Sub-orquestrador Go; classifica a tarefa por dominio e invoca a skill correta via `developer-go-master-orchestrator`; nao implementa diretamente. | Operar a familia `developer-go-*`; handoff com CEO e `documentation-agent-orchestrator`; editar apenas artefatos Go. |
| AT-004 | Agent CEO `developer-agent-orchestrator` | Orquestrador principal multi-kit; classifica a demanda por extensao/kit e delega ao sub-orquestrador Go. | Classificar e delegar; validar handoff entre kits; nao implementa detalhe. |

---

## SECAO 4 — Requisitos Funcionais

O que o kit **deve fornecer**. Um requisito por linha, com a skill/artefato de origem.

| ID | Descricao | Prioridade | Ator | Observacoes (origem) |
|---|---|---|---|---|
| RF-001 | O kit DEVE fornecer skill de fundamentos da linguagem Go (sintaxe base, declaracoes, controle de fluxo, funcoes, pacotes). | Alta | AT-001 | `developer-go-language-core` |
| RF-002 | O kit DEVE fornecer skill do sistema de tipos (structs, interfaces, conversoes, aliases). | Alta | AT-001 | `developer-go-language-types` |
| RF-003 | O kit DEVE fornecer skill de generics (type parameters, constraints, funcoes/tipos genericos). | Media | AT-001 | `developer-go-language-generics` |
| RF-004 | O kit DEVE fornecer skill de "OOP em Go" (interfaces, embedding, metodos, polimorfismo). | Alta | AT-001 | `developer-go-language-oop` |
| RF-005 | O kit DEVE fornecer skill de recursos avancados da linguagem (`unsafe`, `//go:` directives, cgo, iteradores). | Media | AT-001 | `developer-go-language-advanced` |
| RF-006 | O kit DEVE fornecer skill de padroes criacionais idiomaticos (factory, builder, singleton). | Media | AT-001 | `developer-go-patterns-creational` |
| RF-007 | O kit DEVE fornecer skill de padroes estruturais (adapter, decorator, facade). | Media | AT-001 | `developer-go-patterns-structural` |
| RF-008 | O kit DEVE fornecer skill de padroes comportamentais (strategy, observer, state). | Media | AT-001 | `developer-go-patterns-behavioral` |
| RF-009 | O kit DEVE fornecer skill de composicao sobre heranca (embedding, interfaces pequenas). | Media | AT-001 | `developer-go-patterns-composition` |
| RF-010 | O kit DEVE fornecer skill de colecoes da stdlib (slices, maps, `container/*`, `sort`, `slices`, `maps`). | Alta | AT-001 | `developer-go-stdlib-collections` |
| RF-011 | O kit DEVE fornecer skill de strings/IO (`strings`, `bytes`, `bufio`, `io`, `os`, arquivos). | Alta | AT-001 | `developer-go-stdlib-strings-io` |
| RF-012 | O kit DEVE fornecer skill de serializacao/encoding (`encoding/json`, `xml`, `gob`, `csv`). | Alta | AT-001 | `developer-go-stdlib-encoding` |
| RF-013 | O kit DEVE fornecer skill de reflexao/introspecao (`reflect`, tags de struct). | Baixa | AT-001 | `developer-go-stdlib-rtti-reflection` |
| RF-014 | O kit DEVE fornecer skill de concorrencia basica (goroutines, channels, `select`, `sync`). | Alta | AT-001 | `developer-go-concurrency-basics` |
| RF-015 | O kit DEVE fornecer skill de concorrencia avancada (`context`, pipelines, worker pools, race). | Alta | AT-001 | `developer-go-concurrency-advanced` |
| RF-016 | O kit DEVE fornecer skill de gestao de memoria (alocacao, GC, escape analysis, otimizacao). | Media | AT-001 | `developer-go-performance-and-memory` |
| RF-017 | O kit DEVE fornecer skill de profiling (`pprof`, benchmarks, trace, diagnostico de gargalos). | Media | AT-001 | `developer-go-performance-profiling` |
| RF-018 | O kit DEVE fornecer skill de testes (`testing`, table-driven, mocks, coverage, fuzzing). | Alta | AT-001 | `developer-go-testing` |
| RF-019 | O kit DEVE fornecer skill de tratamento de erros e diagnostico (`errors.Is/As`, wrapping, panic/recover, logs). | Alta | AT-001 | `developer-go-error-handling-and-diagnostics` |
| RF-020 | O kit DEVE fornecer skill de build/toolchain (`go build`, `go.mod`, modulos, cross-compilation, flags). | Alta | AT-001 | `developer-go-build-toolchain` |
| RF-021 | O kit DEVE fornecer skill de empacotamento/entrega (versionamento, `ldflags`, release de binarios). | Media | AT-001 | `developer-go-packaging-delivery` |
| RF-022 | O kit DEVE fornecer skill de criptografia/seguranca (`crypto/*`, TLS, hashing, tokens). | Alta | AT-001 | `developer-go-crypto-security` |
| RF-023 | O kit DEVE fornecer skill de arquitetura/design (layout de projeto, camadas, DI, fronteiras de pacote). | Alta | AT-001 | `developer-go-architecture-and-design` |
| RF-024 | O kit DEVE fornecer skill de apps CLI (`flag`/`cobra`, subcomandos). | Media | AT-001 | `developer-go-cli-apps` |
| RF-025 | O kit DEVE fornecer skill de cliente HTTP/REST (`net/http` cliente, retries, timeouts). | Alta | AT-001 | `developer-go-http-client-rest` |
| RF-026 | O kit DEVE fornecer skill de servidor HTTP (`net/http` servidor, handlers, middleware, roteamento REST). | Alta | AT-001 | `developer-go-http-server` |
| RF-027 | O kit DEVE fornecer skill de acesso a banco de dados (`database/sql`, drivers, pools, transacoes). | Alta | AT-001 | `developer-go-database-access` |
| RF-028 | O kit DEVE fornecer skill de deploy em Linux (`systemd`, servicos, permissoes, empacotamento OS). | Media | AT-001 | `developer-go-linux-deploy` |
| RF-029 | O kit DEVE fornecer skill orquestradora que classifica o cenario Go e roteia para a(s) skill(s) correta(s). | Alta | AT-003 | `developer-go-master-orchestrator` (matriz de roteamento) |
| RF-030 | O kit DEVE fornecer skill geradora de `SPEC.md` por engenharia reversa de codigo Go (protocolo SCAN->READ->EXTRACT->GENERATE->SAVE+REPORT). | Alta | AT-002 | `developer-go-project-spec` |
| RF-031 | O kit DEVE fornecer um agent sub-orquestrador Go que recebe demandas do CEO e coordena a familia `developer-go-*`. | Alta | AT-003 | `developer-golang-agent-orchestrator_V1.0.0` |
| RF-032 | O CEO DEVE reconhecer demandas Go por extensao (`*.go`, `go.mod`, `go.sum`, `cmd/`, `internal/`, `pkg/`) e delegar ao sub-orquestrador Go. | Alta | AT-004 | `developer-agent-orchestrator_V2.4.0` (linhas 28 e 40) |
| RF-033 | O kit DEVE fornecer um blueprint reutilizavel (`kit-go_V1.0`) com estrutura de projeto Go e regras minimas, alem deste SPEC. | Media | AT-002 | `.cursor/Templates/kit-go_V1.0/README.md` + `SPEC.md` |

**Prioridades:**
- Alta: essencial para o funcionamento basico do kit (roteamento, linguagem nuclear, HTTP, DB, testes, build, reconhecimento pelo CEO).
- Media: cobre dominios importantes que podem ser entregues/evoluidos em fase posterior.
- Baixa: desejavel; skills de nicho (reflection) usadas com menos frequencia.

> Cobertura: os RF-001..RF-030 cobrem, uma a uma, as 30 skills reais confirmadas em
> disco; RF-031..RF-033 cobrem o agent, o registro no CEO e o blueprint.

---

## SECAO 5 — Requisitos Nao Funcionais

| ID | Categoria | Descricao | Criterio de Aceitacao |
|---|---|---|---|
| RNF-001 | Performance (economia de contexto) | Cada `SKILL.md` folha DEVE ser conciso (meta ~200-220 linhas; snippets inline <= 15 linhas), para nao inflar o contexto do agente. | Skills folhas dentro do limite; blocos maiores movidos para `exemplos/`. |
| RNF-002 | Performance (excecao documentada) | As skills de coordenacao `developer-go-master-orchestrator` (~240 linhas) e `developer-go-project-spec` (~250 linhas) EXCEDEM o limite por necessidade (mapa das 30 skills + matriz de roteamento; protocolo de 14 secoes) — excecao explicitamente autorizada. | Excecao registrada nesta SPEC; nenhuma outra skill folha ultrapassa o limite sem justificativa. |
| RNF-003 | Compatibilidade (validacao do pack) | Todas as skills DEVEM passar em `.cursor/scripts/validate_pack.py` (frontmatter, nomenclatura, versao). | `validate_pack.py` sem erros para os 30 diretorios `developer-go-*` e o agent. |
| RNF-004 | Consistencia (frontmatter/versao) | Frontmatter obrigatorio (`name`, `description`, `model`, `thinking`, `category: developer-go`) e FileVersion coerente com o sufixo da pasta (`_V1.0.0`). | 100% das skills com frontmatter valido e `FileVersion == sufixo da pasta`. |
| RNF-005 | Manutenibilidade (SSOT + espelhos) | `.cursor/` e a unica fonte canonica; os espelhos (`.claude/`, `.vscode/`, `.continue/`, `.opencode/`) sao gerados por symlink, nunca editados a mao. | `bootstrap-mirror-symlinks.ps1 -ValidateOnly` valida os espelhos apos criacao. |
| RNF-006 | Consistencia (idioma/convencoes) | Codigo, identificadores e nomes de arquivo em ingles; prosa das skills e do SPEC em pt-BR; changelog obrigatorio por arquivo. | Nenhuma skill sem changelog; nomes de pasta/skill em kebab-case ingles. |
| RNF-007 | Seguranca (stdlib-first) | O kit NAO DEVE prescrever dependencia de terceiros onde a stdlib resolve, reduzindo superficie de ataque e tamanho de build. | Anti-padrao "usar framework 3rd-party quando a stdlib resolve" documentado no master-orchestrator. |

---

## SECAO 6 — Casos de Uso

### UC-001: Desenvolvedor cria novo modulo Go do zero

| Campo | Valor |
|---|---|
| ID | UC-001 |
| Nome | Bootstrap de novo modulo Go |
| Ator Principal | AT-001 (Desenvolvedor Go) |
| Pre-condicoes | Toolchain Go instalado (>= 1.21); pasta-alvo vazia ou sem `go.mod`. |
| Pos-condicoes | Modulo com `go.mod`, layout `cmd/`/`internal/`/`pkg/` e `go build ./...` verde. |
| Trigger | Pedido "criar novo servico/modulo Go". |

**Fluxo Principal:**
1. O sub-orquestrador aciona `developer-go-master-orchestrator` para classificar o cenario.
2. A matriz roteia para `developer-go-build-toolchain` (cria `go.mod`, toolchain, flags).
3. Em seguida `developer-go-architecture-and-design` define camadas e fronteiras de pacote.
4. Checkpoint: `go build ./...` + `go test ./...`.

**Fluxo Alternativo:**
- 2a. Se ja existe `go.mod`: pular a criacao e ir direto ao design de arquitetura.

**Fluxo de Excecao:**
- 3b. Se o layout proposto violar `internal/` (contrato publico exposto): retornar ao passo 3 e reclassificar o pacote.

---

### UC-002: Tech Lead pede SPEC de um projeto Go existente

| Campo | Valor |
|---|---|
| ID | UC-002 |
| Nome | Geracao de SPEC por engenharia reversa |
| Ator Principal | AT-002 (Tech Lead) |
| Pre-condicoes | Modulo Go com `go.mod` e codigo em `cmd/`/`internal/`/`pkg/`. |
| Pos-condicoes | `SPEC.md` com 14 secoes preenchidas na raiz + relatorio de cobertura. |
| Trigger | Pedido "documente/gere a SPEC deste projeto Go". |

**Fluxo Principal:**
1. `developer-go-project-spec` detecta idioma e carrega o template (`spec-template.md`).
2. SCAN: glob `**/*.go`, `go.mod`, `go.sum`, pastas `cmd/`/`internal/`/`pkg/`.
3. READ + EXTRACT: atores, RF, RNF, RN, UC, modelo de dados, integracoes, restricoes.
4. GENERATE: preenche as 14 secoes; marca itens nao determinaveis com `[INFERIDO]`.
5. SAVE+REPORT: grava `SPEC.md` na raiz e reporta cobertura (real vs. inferido).

**Fluxo Alternativo:**
- 1a. Se a 1a mensagem estiver em en-US: usar `spec-template.en.md` e marcador `[INFERRED]`.

**Fluxo de Excecao:**
- 2b. Se o escopo for ambiguo (projeto inteiro vs. um modulo): parar e confirmar antes de gerar.

---

### UC-003: Desenvolvedor debuga erro intermitente de concorrencia

| Campo | Valor |
|---|---|
| ID | UC-003 |
| Nome | Diagnostico de race condition |
| Ator Principal | AT-001 (Desenvolvedor Go) |
| Pre-condicoes | Falha intermitente reproduzivel; codigo concorrente (goroutines/channels). |
| Pos-condicoes | Causa-raiz isolada; `go test -race ./...` verde. |
| Trigger | Bug intermitente que some/aparece entre execucoes. |

**Fluxo Principal:**
1. A matriz roteia para `developer-go-error-handling-and-diagnostics` (logs, wrapping, panic/recover).
2. Escala para `developer-go-concurrency-advanced` (race detector, `context`, sincronizacao).
3. Reproduz sob `go test -race`; corrige a sincronizacao.
4. Checkpoint: `go test -race ./...` estavel em multiplas execucoes.

**Fluxo Alternativo:**
- 2a. Se o problema for de vazamento de goroutine sob carga: acionar tambem `developer-go-performance-profiling`.

**Fluxo de Excecao:**
- 3b. Se a race estiver em lib de terceiros: mitigar na fronteira do proprio codigo e documentar (nao perseguir race externa indefinidamente).

---

### UC-004: Desenvolvedor expoe uma API REST (servidor)

| Campo | Valor |
|---|---|
| ID | UC-004 |
| Nome | Servidor HTTP REST com serializacao JSON |
| Ator Principal | AT-001 (Desenvolvedor Go) |
| Pre-condicoes | Modulo Go inicializado; contrato de endpoints definido. |
| Pos-condicoes | Servidor respondendo com JSON; timeouts configurados; testes de handler verdes. |
| Trigger | Pedido "criar API REST simples em Go". |

**Fluxo Principal:**
1. A matriz roteia para `developer-go-http-server` (handlers, middleware, roteamento).
2. Complementa com `developer-go-stdlib-encoding` (marshal/unmarshal JSON).
3. Configura `http.Server` com Read/Write/Idle timeouts (RNF de performance).
4. Checkpoint: `go build ./...` + testes de handler.

**Fluxo Alternativo:**
- 1a. Se for consumo de API externa em vez de servir: rotear para `developer-go-http-client-rest`.

**Fluxo de Excecao:**
- 2b. Se o payload exigir persistencia: encadear `developer-go-database-access`.

---

### UC-005: Desenvolvedor otimiza gargalo de CPU/memoria

| Campo | Valor |
|---|---|
| ID | UC-005 |
| Nome | Otimizacao guiada por profiling |
| Ator Principal | AT-001 (Desenvolvedor Go) |
| Pre-condicoes | Gargalo suspeito medido (latencia/alocacao alta). |
| Pos-condicoes | Ganho comprovado por benchmark antes/depois. |
| Trigger | Pedido "otimizar CPU/memoria de um caminho quente". |

**Fluxo Principal:**
1. A matriz roteia para `developer-go-performance-profiling` (`pprof`, benchmarks, trace).
2. Encadeia `developer-go-performance-and-memory` (escape analysis, alocacao, GC).
3. Aplica correcao; mede novamente com `go test -bench`.
4. Checkpoint: benchmark comprova a melhoria (prova empirica, nao teoria).

**Fluxo de Excecao:**
- 1b. Se o "gargalo" nao aparecer no profile: refutar a hipotese e reinvestigar antes de mexer no codigo.

---

### UC-006: Desenvolvedor entrega uma CLI e faz deploy em Linux

| Campo | Valor |
|---|---|
| ID | UC-006 |
| Nome | CLI empacotada e implantada como servico |
| Ator Principal | AT-001 (Desenvolvedor Go) |
| Pre-condicoes | Modulo Go com subcomandos definidos; alvo `linux/amd64`. |
| Pos-condicoes | Binario versionado, cross-compilado e rodando sob `systemd`. |
| Trigger | Pedido "criar ferramenta CLI e implantar no servidor Linux". |

**Fluxo Principal:**
1. A matriz roteia para `developer-go-cli-apps` (`flag`/`cobra`, subcomandos).
2. Encadeia `developer-go-build-toolchain` (cross-compile) e `developer-go-packaging-delivery` (`ldflags`, versao).
3. Finaliza com `developer-go-linux-deploy` (`systemd`, permissoes, shutdown gracioso).
4. Checkpoint: binario iniciando/parando limpo via `systemctl`.

**Fluxo de Excecao:**
- 3b. Deploy em producao: parar e confirmar com o Tech Lead antes de executar (risco alto).

---

## SECAO 7 — User Stories (complementar aos casos de uso)

| ID | Como... | Quero... | Para que... | Criterios de Aceitacao |
|---|---|---|---|---|
| US-001 | Desenvolvedor Go (AT-001) | invocar uma unica skill de entrada que me diga qual skill Go usar | eu nao precise memorizar as 30 skills do kit | - [ ] `developer-go-master-orchestrator` roteia o cenario para a skill certa na 1a tentativa (ligado a UC-001) |
| US-002 | Tech Lead (AT-002) | gerar um `SPEC.md` completo a partir do codigo Go existente | eu documente/venda/audite o sistema sem entrevistar ninguem | - [ ] 14 secoes preenchidas; itens sem evidencia marcados `[INFERIDO]` (ligado a UC-002) |
| US-003 | Desenvolvedor Go (AT-001) | diagnosticar races com o skill de concorrencia avancada | eu estabilize um servico intermitente | - [ ] `go test -race ./...` verde e estavel (ligado a UC-003) |
| US-004 | Desenvolvedor Go (AT-001) | subir uma API REST usando so a stdlib | eu evite dependencia de framework 3rd-party | - [ ] Servidor com timeouts e JSON funcionando sem imports externos (ligado a UC-004) |
| US-005 | Agent CEO (AT-004) | reconhecer `*.go`/`go.mod` e delegar ao sub-orquestrador Go | demandas Go sigam o mesmo fluxo dos kits Delphi/Vue | - [ ] Classificacao por extensao roteia para `developer-golang-agent-orchestrator` (ligado a RF-032) |

---

## SECAO 8 — Regras de Negocio

Convencoes obrigatorias do kit, independentes do fluxo. Origem: `SKILL_TEMPLATE_V2.0.md`,
`CLAUDE.md`, `pack-rules-manifest` e README do blueprint.

| ID | Descricao | Origem (lei / politica / cliente) |
|---|---|---|
| RN-001 | Nomenclatura obrigatoria da skill: `developer-go-<topico>_V<major>.<minor>.<patch>` (kebab-case ingles). | Convencao de naming do pack (`pack-scripts-nomenclature` / template V2) |
| RN-002 | O sufixo da pasta DEVE ser igual ao FileVersion SemVer do `SKILL.md` (ex.: FileVersion 1.0.0 -> pasta `_V1.0.0`). | `SKILL_TEMPLATE_V2.0.md` (linha "Sufixo da pasta = versao SemVer") |
| RN-003 | O campo `model` do frontmatter DEVE ser um de `{haiku, sonnet, opus}`. | `SKILL_TEMPLATE_V2.0.md` (frontmatter) |
| RN-004 | Skills folha Go DEVEM incluir o bloco Go obrigatorio: "Stack e versoes", "Dependencias (go.mod)", "Checklist Go" e "Exemplo minimo compilavel". | Padrao das skills `developer-go-*` (secoes marcadas OBRIGATORIO (Go)) |
| RN-005 | `SKILL.md` deve ser conciso (meta ~200-220 linhas); snippets inline <= 15 linhas; blocos maiores vao para `exemplos/`. | `SKILL_TEMPLATE_V2.0.md` (Politica de subpastas) |
| RN-006 | Frontmatter obrigatorio: `name`, `description`, `model`, `thinking`, `category: developer-go`. | `SKILL_TEMPLATE_V2.0.md` (bloco yaml) |
| RN-007 | Todo arquivo (skill/agent/template) DEVE manter secao "Changelog (este arquivo)" com entrada datada. | `SKILL_TEMPLATE_V2.0.md` + politica de versionamento |
| RN-008 | `.cursor/` e a unica fonte canonica (SSOT); `.claude/`, `.vscode/`, `.continue/`, `.opencode/` sao espelhos por symlink e NUNCA sao editados diretamente. | `CLAUDE.md` (SSOT) |
| RN-009 | O kit e stdlib-first: preferir `net/http`, `database/sql`, `flag` a frameworks de terceiros. | README `kit-go_V1.0` + anti-padrao do master-orchestrator |
| RN-010 | `gofmt` obrigatorio (sem excecao) e `go vet ./...` limpo antes de commit; erros tratados explicitamente (`if err != nil`), sem descartar com `_` sem justificativa. | README `kit-go_V1.0` (Regras minimas) |
| RN-011 | O sub-orquestrador Go NAO edita `*.pas`/`*.dfm`/`*.fmx` nem `*.vue`/`*.ts`/`*.js` — boundary por kit; tarefas cross-kit escalam ao CEO. | `developer-golang-agent-orchestrator` (Boundary) |
| RN-012 | Alteracoes em areas protegidas (`Documentation/`, `.cursor/skills|Templates|agents|rules/`) exigem plano e aprovacao explicita antes de criar/mover/renomear/excluir. | `CLAUDE.md` (Areas protegidas / plan mode) |

---

## SECAO 9 — Fluxos de Tela e Navegacao

O kit nao possui UI; a "navegacao" e o **fluxo de roteamento** do
`developer-go-master-orchestrator`. A matriz de roteamento abaixo funciona como o
mapa de navegacao textual equivalente.

### Mapa de Navegacao (roteamento por cenario)

```
[CEO: developer-agent-orchestrator]
    └─► classifica extensao (*.go / go.mod / go.sum / cmd|internal|pkg)
            └─► [developer-golang-agent-orchestrator]
                    └─► [developer-go-master-orchestrator]  (ponto de entrada unico)
                            ├─► Criar modulo do zero ........ build-toolchain → architecture-and-design
                            ├─► Estruturar arquitetura ....... architecture-and-design → patterns-composition
                            ├─► Testes de pacote ............. testing
                            ├─► API REST (servidor) .......... http-server → stdlib-encoding
                            ├─► Consumir API externa ......... http-client-rest → stdlib-encoding
                            ├─► CLI .......................... cli-apps → build-toolchain
                            ├─► Banco de dados ............... database-access → error-handling-and-diagnostics
                            ├─► Concorrencia basica .......... concurrency-basics
                            ├─► Pipeline/worker pool ......... concurrency-advanced → performance-and-memory
                            ├─► Debug race intermitente ...... error-handling-and-diagnostics → concurrency-advanced
                            ├─► Otimizar CPU/memoria ......... performance-profiling → performance-and-memory
                            ├─► Encoding (JSON/XML/gob) ...... stdlib-encoding
                            ├─► Strings/IO/arquivos .......... stdlib-strings-io → stdlib-collections
                            ├─► Reflexao (reflect) ........... stdlib-rtti-reflection
                            ├─► Genericos .................... language-generics → language-types
                            ├─► OOP (interfaces/embedding) ... language-oop → patterns-structural
                            ├─► Design pattern ............... patterns-creational / patterns-behavioral
                            ├─► Cripto/TLS/tokens ............ crypto-security
                            ├─► Release de binarios .......... packaging-delivery → build-toolchain
                            ├─► Deploy Linux (systemd) ....... linux-deploy → packaging-delivery
                            ├─► Feature avancada da linguagem  language-advanced
                            └─► Gerar SPEC ................... project-spec
```

### Descricao de "Telas" (pontos de entrada)

#### Ponto de entrada: `developer-go-master-orchestrator`
- **Proposito:** classificar o cenario Go e rotear para a(s) skill(s) correta(s), combinando quando cross-cutting.
- **Campos (inputs):** `objetivo` (texto, obrigatorio), `escopo` (texto, obrigatorio), `criterios_aceite` (lista, obrigatorio).
- **Acoes:** delega para skill folha; para cenarios cross-cutting, executa em sequencia com checkpoint (`go build ./...` + `go test ./...`).
- **Validacoes:** cenario classificado em >= 1 familia; nenhuma implementacao feita no proprio orquestrador.
- **Requisitos relacionados:** RF-029, RNF-002.

#### Ponto de entrada: `developer-go-project-spec`
- **Proposito:** gerar `SPEC.md` por engenharia reversa (SCAN->READ->EXTRACT->GENERATE->SAVE+REPORT).
- **Campos (inputs):** `alvo` (raiz do modulo com `go.mod`), `idioma` (pt-BR/en-US), `escopo` (projeto/modulo).
- **Acoes:** preenche as 14 secoes; grava `SPEC.md` na raiz; reporta cobertura real vs. `[INFERIDO]`.
- **Validacoes:** escopo e projeto inteiro ou modulo inteiro (nunca funcao/arquivo isolado); todas as 14 secoes preenchidas.
- **Requisitos relacionados:** RF-030, UC-002.

---

## SECAO 10 — Modelo de Dados

Aqui o "modelo de dados" e o **inventario estruturado do kit** — as entidades do
dominio de governanca do pack.

### 10.1 Entidades Principais

| Entidade | Descricao | Atributos Principais |
|---|---|---|
| Skill | Unidade de conhecimento acionavel (`SKILL.md` + subpastas). | `name`, `description`, `model`, `thinking`, `category`, `FileVersion`, `familia`, pasta `_V{semver}` |
| Familia | Agrupamento tematico das skills. | `nome` (Coordenacao/Spec, Linguagem, Patterns, Stdlib, Concorrencia/Performance, Qualidade, Build/Delivery, Arquitetura/Apps, Deploy) |
| Agent (sub-orquestrador) | Coordenador do kit Go. | `name`, `model` (sonnet), `managed by` (CEO), `boundary` (`*.go`/`go.mod`/`go.sum`/`cmd`/`internal`/`pkg`) |
| Registro CEO | Entrada do kit Go na tabela de sub-orquestradores do CEO. | linha de sub-orquestrador + linha de classificacao por extensao (`developer-agent-orchestrator_V2.4.0`) |
| Blueprint | Estrutura reutilizavel de projeto Go. | `README.md` (estrutura + regras minimas), `SPEC.md` (este documento) |

### 10.2 Relacionamentos

```
[CEO developer-agent-orchestrator] 1 ──── N [Sub-orquestrador]
[Sub-orquestrador developer-golang-agent-orchestrator] 1 ──── N [Skill developer-go-*]
[Skill] N ──── 1 [Familia]
[developer-go-master-orchestrator] 1 ──── N [Skill folha]  (roteia para)
[Blueprint kit-go_V1.0] 1 ──── 1 [SPEC.md]
```

### 10.3 Estruturas de Dados — Inventario das 30 skills

| # | Skill (`_V1.0.0`) | Familia | Responsabilidade |
|---|---|---|---|
| 1 | `developer-go-language-core` | Linguagem | Sintaxe base, declaracoes, controle de fluxo, funcoes, pacotes. |
| 2 | `developer-go-language-types` | Linguagem | Sistema de tipos, structs, interfaces, conversoes, aliases. |
| 3 | `developer-go-language-generics` | Linguagem | Type parameters, constraints, funcoes/tipos genericos. |
| 4 | `developer-go-language-oop` | Linguagem | OOP em Go: interfaces, embedding, metodos, polimorfismo. |
| 5 | `developer-go-language-advanced` | Linguagem | `unsafe`, `//go:` directives, cgo, iteradores. |
| 6 | `developer-go-patterns-creational` | Patterns | Factory, builder, singleton idiomaticos. |
| 7 | `developer-go-patterns-structural` | Patterns | Adapter, decorator, facade em Go. |
| 8 | `developer-go-patterns-behavioral` | Patterns | Strategy, observer, state. |
| 9 | `developer-go-patterns-composition` | Patterns | Composicao sobre heranca, embedding, interfaces pequenas. |
| 10 | `developer-go-stdlib-collections` | Stdlib | Slices, maps, `container/*`, `sort`, `slices`, `maps`. |
| 11 | `developer-go-stdlib-strings-io` | Stdlib | `strings`, `bytes`, `bufio`, `io`, `os`, arquivos. |
| 12 | `developer-go-stdlib-encoding` | Stdlib | `encoding/json`, `xml`, `gob`, `csv`. |
| 13 | `developer-go-stdlib-rtti-reflection` | Stdlib | `reflect`, introspecao, tags de struct. |
| 14 | `developer-go-concurrency-basics` | Concorrencia/Performance | Goroutines, channels, `select`, `sync` basico. |
| 15 | `developer-go-concurrency-advanced` | Concorrencia/Performance | `context`, pipelines, worker pools, race. |
| 16 | `developer-go-performance-and-memory` | Concorrencia/Performance | Alocacao, GC, escape analysis, otimizacao. |
| 17 | `developer-go-performance-profiling` | Concorrencia/Performance | `pprof`, benchmarks, trace, gargalos. |
| 18 | `developer-go-testing` | Qualidade | `testing`, table-driven, mocks, coverage, fuzzing. |
| 19 | `developer-go-error-handling-and-diagnostics` | Qualidade | `errors.Is/As`, wrapping, panic/recover, logs. |
| 20 | `developer-go-build-toolchain` | Build/Delivery | `go build`, `go.mod`, modulos, cross-compile, flags. |
| 21 | `developer-go-packaging-delivery` | Build/Delivery | Versionamento, `ldflags`, release de binarios. |
| 22 | `developer-go-crypto-security` | Build/Delivery | `crypto/*`, TLS, hashing, tokens. |
| 23 | `developer-go-architecture-and-design` | Arquitetura/Apps | Layout, camadas, DI, fronteiras de pacote. |
| 24 | `developer-go-cli-apps` | Arquitetura/Apps | `flag`/`cobra`, subcomandos. |
| 25 | `developer-go-http-client-rest` | Arquitetura/Apps | `net/http` cliente, retries, timeouts. |
| 26 | `developer-go-http-server` | Arquitetura/Apps | `net/http` servidor, handlers, middleware, roteamento. |
| 27 | `developer-go-database-access` | Arquitetura/Apps | `database/sql`, drivers, pools, transacoes. |
| 28 | `developer-go-linux-deploy` | Deploy | `systemd`, servicos, permissoes, empacotamento OS. |
| 29 | `developer-go-master-orchestrator` | Coordenacao/Orquestracao | Classifica o cenario e roteia para as demais (ponto de entrada). |
| 30 | `developer-go-project-spec` | Coordenacao/Spec | Geracao da `SPEC.md` do projeto/modulo Go. |

**Artefatos de governanca associados (nao-skill):**

| Artefato | Localizacao | Papel |
|---|---|---|
| `developer-golang-agent-orchestrator_V1.0.0.md` | `.cursor/agents/` | Sub-orquestrador Go (model: sonnet), gerido pelo CEO. |
| `developer-agent-orchestrator_V2.4.0.md` | `.cursor/agents/` | CEO — registra o kit Go (linhas 28 e 40) e classifica por extensao. |
| `kit-go_V1.0/README.md` + `SPEC.md` | `.cursor/Templates/kit-go_V1.0/` | Blueprint reutilizavel + este documento. |

---

## SECAO 11 — Integracoes Externas

| ID | Sistema Externo | Tipo | Protocolo | Direcao | Descricao |
|---|---|---|---|---|---|
| INT-001 | `.cursor/scripts/validate_pack.py` | Script de validacao | CLI (Python) | Saida (o kit e validado) | Valida integridade de frontmatter, nomenclatura e versao das 30 skills e do agent. |
| INT-002 | `developer-agent-orchestrator` (CEO) | Agent orquestrador | Handoff interno | Entrada/Saida | Recebe a demanda Go, classifica por extensao e delega ao sub-orquestrador Go. |
| INT-003 | Espelhos `.claude/`, `.vscode/`, `.continue/`, `.opencode/` | Symlinks de espelho | Filesystem (symlink) | Saida | Gerados/validados por `bootstrap-mirror-symlinks.ps1`; propagam o SSOT `.cursor/`. |
| INT-004 | Referencias oficiais Go (`go.dev`) | Documentacao externa | HTTP | Entrada (consulta) | Fonte canonica de idiomas Go: `go.dev/doc/`, `go.dev/doc/effective_go`. |

---

## SECAO 12 — Restricoes e Premissas

### 12.1 Restricoes Tecnicas

- Go versao minima: **1.21** (lida da diretiva `go X.YY` do `go.mod` do projeto-alvo, conforme `developer-go-project-spec`); o `developer-go-master-orchestrator` recomenda toolchain **1.22.x** como padrao para o kit.
- Ferramentas de qualidade: `gofmt` (bundled, obrigatorio), `go vet` (bundled), `golangci-lint` (1.55+/1.59.x recomendado), Delve `dlv` (1.22.x) quando debugging.
- Plataforma alvo do kit: multiplataforma via cross-compilation (`GOOS`/`GOARCH`), com foco de deploy em `linux/amd64` (skill `developer-go-linux-deploy`).
- Modulos obrigatorios (`GO111MODULE=on`, padrao desde 1.16); GOPATH legado nao suportado.
- Dependencias de terceiros: **nenhuma fixada** pelo kit (stdlib-first); qualquer dependencia e do projeto-alvo, lida de seu `go.mod`.
- Ambiente de autoria do pack: Windows (scripts PowerShell de bootstrap); `.cursor/` como SSOT, espelhos por symlink (exige privilegio para criar symlinks).

### 12.2 Premissas

- O kit e **generico**: nao assume nenhum framework web/ORM de terceiros; cada projeto-alvo decide suas dependencias.
- O toolchain Go esta (ou sera) instalado na maquina que consome as skills; o kit documenta versoes, nao provisiona o ambiente.
- O agente CEO ja esta na versao 2.4.0 (com o kit Go registrado) e roteia demandas por extensao.
- Os espelhos podem ser regenerados a qualquer momento a partir de `.cursor/` sem perda de conteudo canonico.
- [INFERIDO] Metricas de adocao futura do kit (frequencia de uso por skill, cobertura em projetos reais) — nao ha dados historicos por ser criacao inicial em 09/08/2026.

---

## SECAO 13 — Criterios de Aceitacao Global

O kit sera considerado aprovado quando:

- [ ] As **30 skills** `developer-go-*_V1.0.0` estiverem presentes em `.cursor/skills/` e passarem em `.cursor/scripts/validate_pack.py` (frontmatter, nomenclatura, versao) — INT-001.
- [ ] Todos os requisitos de prioridade **Alta** (RF-001, RF-002, RF-004, RF-010..RF-012, RF-014, RF-015, RF-018..RF-020, RF-022, RF-023, RF-025..RF-027, RF-029..RF-032) estiverem cobertos por uma skill/artefato real em disco.
- [ ] O CEO `developer-agent-orchestrator_V2.4.0` reconhecer demandas Go por extensao (`*.go`, `go.mod`, `go.sum`, `cmd/`, `internal/`, `pkg/`) e delegar ao `developer-golang-agent-orchestrator` — RF-032.
- [ ] O agent `developer-golang-agent-orchestrator_V1.0.0` referenciar corretamente `developer-go-master-orchestrator` como ponto de entrada e listar as 30 skills — RF-031.
- [ ] Os espelhos (`.claude/`, `.vscode/`, `.continue/`, `.opencode/`) estiverem integros apos a criacao (`bootstrap-mirror-symlinks.ps1 -ValidateOnly` sem falha) — RNF-005/INT-003.
- [ ] A excecao de tamanho de `master-orchestrator` e `project-spec` estar documentada (RNF-002); demais skills folha dentro da meta de concisao (RNF-001).
- [ ] Todos os arquivos manterem changelog datado (RN-007) e frontmatter valido com `category: developer-go` (RN-006).

---

## SECAO 14 — Historico de Revisoes

| Versao | Data | Autor | Descricao da Alteracao |
|---|---|---|---|
| 1.0.0 | 09/08/2026 | CSL Tech Solutions | Versao inicial. Criacao do kit de skills GoLang (30 skills `developer-go-*_V1.0.0`, agent sub-orquestrador `developer-golang-agent-orchestrator_V1.0.0`, registro no CEO `developer-agent-orchestrator_V2.4.0` e blueprint `kit-go_V1.0`) e deste SPEC gerado por aplicacao reflexiva da skill `developer-go-project-spec`. |

---
name: developer-go-project-spec
description: Gera SPEC.md completo (RF/RNF/RN/UC/US) para projetos Go por reverse-engineering do codigo-fonte, bilingue pt-BR/en-US com marcadores [INFERIDO]/[INFERRED].
model: opus
thinking: extended
category: developer-go
license: MIT
copyright: "Copyright (c) 2026 CSL Tech Solutions"
company: "CSL Tech Solutions"
author: "Claiton de Souza Linhares"
---

# developer-go-project-spec

## Responsabilidade única

Ler código-fonte Go e produzir uma SPEC completa, rastreável e acionável por
engenharia reversa — sem entrevistar o usuário. Extrai atores, requisitos
funcionais e não funcionais, regras de negócio, casos de uso, modelo de dados,
integrações e restrições técnicas diretamente do que o código já expressa
(funções exportadas, handlers HTTP, comandos CLI, structs com tags, erros e
configs). Cobre o **projeto inteiro ou um módulo de negócio inteiro** — nunca
uma função ou arquivo isolado. Produz *especificação de requisitos*, não laudo
de qualidade ou auditoria de code smells.

## When to use

- Gerar especificação de software a partir de um módulo/serviço Go existente.
- Documentar um sistema Go para onboarding, auditoria, due diligence ou venda.
- Produzir a SPEC quando não há documentação formal e a única fonte de verdade
  é o código (`cmd/`, `internal/`, `pkg/`).
- Recuperar requisitos de um serviço HTTP, worker ou CLI legado antes de refatorar.

## When NOT to use

- Especificar requisitos *futuros* / ainda não codificados → `governance-spec-*`.
- Auditar qualidade, code smells e dívida técnica → `quality-code-review-checklist` / `quality-tech-debt-tracker`.
- Desenhar a arquitetura de pacotes de um módulo novo → `developer-go-architecture-and-design`.
- Documentar apenas uma função ou um arquivo isolado → usar godoc/comentários internos.

## Inputs obrigatórios

| Input | Tipo | Descrição |
|-------|------|-----------|
| `alvo` | `string` | Raiz do módulo Go (com `go.mod`) ou pasta de um módulo de negócio inteiro |
| `idioma` | `enum(pt-BR,en-US)` | Idioma da SPEC; detectado da 1ª mensagem do usuário (padrão pt-BR) |
| `escopo` | `enum(projeto,modulo)` | Projeto inteiro ou um módulo de negócio inteiro — nunca função/arquivo isolado |

## Dependências (skills prévias)

| Skill | Quando executar antes |
|-------|-----------------------|
| `developer-go-architecture-and-design` | Entender o layout `cmd/`/`internal/`/`pkg/` e as fronteiras de pacote antes de mapear RF por camada |
| `developer-go-master-orchestrator` | Quando a demanda é ampla e precisa ser roteada/combinada com outras skills do kit Go antes de gerar a SPEC |

## Workflow executável

Protocolo de 5 etapas: **SCAN → READ → EXTRACT → GENERATE → SAVE+REPORT**.

### §0 — Idioma e template

Detectar o idioma da primeira mensagem do usuário:
- pt-BR (padrão) → `references/spec-template.md` · marcador: `[INFERIDO]`
- en-US → `references/spec-template.en.md` · marcador: `[INFERRED]`

### Etapa 1 — SCAN

```
Glob: **/*.go, go.mod, go.sum
Dirs: cmd/, internal/, pkg/, Documentation/
```

Identificar: pontos de entrada (`cmd/<bin>/main.go`), pacotes de domínio
(`internal/<dominio>`), API pública (`pkg/`), dependências (`go.mod`/`go.sum`) e
qualquer documentação existente. Ignorar `_test.go` para RF (servem à Etapa 3
como pista de comportamento esperado, não como requisito em si).

### Etapa 2 — READ

Ler os arquivos em ordem de prioridade:
1. Pacotes de domínio (`internal/<dominio>`) — regra de negócio e contratos (`interface`).
2. Pontos de entrada e superfície de I/O — `cmd/*/main.go`, handlers `net/http`, roteadores, comandos CLI (`flag`).
3. Structs de dados — tipos com tags `json:`/`db:` e queries SQL/migrations.
4. `go.mod`/`go.sum` — versão do Go e dependências de terceiros.

### Etapa 3 — EXTRACT

| Elemento | Como extrair do código Go |
|----------|---------------------------|
| Atores (AT) | Middlewares de autenticação (JWT/session), `context` de usuário, checagens de papel/permissão, clientes externos que chamam a API |
| RF (Requisitos Funcionais) | Funções exportadas em `cmd/`/`internal/`/`pkg/`; handlers HTTP (`http.HandlerFunc`, rotas por método+path); subcomandos e flags de CLI (`flag`, `cobra`) |
| RNF (Não Funcionais) | `http.Server` timeouts (Read/Write/Idle), `context.WithTimeout`/deadlines, pool de conexões (`db.SetMaxOpenConns`/`SetMaxIdleConns`), concorrência (goroutines, worker pool, `errgroup`, rate limiting), TLS |
| RN (Regras de Negócio) | Validações e guard clauses (`if ... { return err }`); `errors.New`/`fmt.Errorf` e sentinel/typed errors; invariantes checadas antes de persistir |
| UC (Casos de Uso) | Fluxo de cada handler HTTP (request → validação → serviço → resposta) ou de cada subcomando CLI |
| Modelo de Dados | `struct` com tags `json:`/`db:`; DDL/migrations; assinaturas de query (`database/sql`, `sqlx`, ORM) |
| Integrações (INT) | Clientes `net/http`/gRPC, drivers `database/sql`, SDKs e imports de terceiros no `go.mod` (fila, cache, storage) |
| Restrições Técnicas | Diretiva `go X.YY` do `go.mod`; driver/BD; plataforma alvo (`GOOS`/`GOARCH`); dependências obrigatórias (go.mod) |

Itens não determináveis automaticamente pelo código → marcar com `[INFERIDO]`/`[INFERRED]`.

### Etapa 4 — GENERATE

Preencher **todas as 14 seções** do template. Onde não houver evidência no
código, usar o placeholder do idioma:
- pt-BR: `"Não identificado no código-fonte."`
- en-US: `"Not identified in source code."`

Numeração fixa: `RF-001`, `RNF-001`, `RN-001`, `UC-001`, `US-001`, `AT-001`, `INT-001`.

### Etapa 5 — SAVE + REPORT

Gravar como `SPEC.md` na raiz do projeto. Reportar ao usuário:
- Caminho do arquivo gerado.
- Seções preenchidas com dados reais vs. marcadas `[INFERIDO]`/`[INFERRED]`.
- Pacotes/arquivos não analisados (se houver).
- Seções que merecem revisão manual.

## Outputs obrigatórios

| Output | Localização | Formato |
|--------|-------------|---------|
| `SPEC.md` (14 seções preenchidas) | Raiz do projeto/módulo Go | Markdown |
| Relatório de cobertura (real vs. inferido) | Mensagem final ao usuário | Texto |

## Checklist de validação

- [ ] Idioma detectado e template correto carregado (`.md` pt-BR / `.en.md` en-US)
- [ ] SCAN completo (`*.go`, `go.mod`, `go.sum`, `cmd/`, `internal/`, `pkg/`)
- [ ] Escopo é projeto inteiro ou módulo de negócio inteiro (nunca função/arquivo isolado)
- [ ] Todas as 14 seções preenchidas (sem seção em branco)
- [ ] Itens inferidos marcados com `[INFERIDO]`/`[INFERRED]`
- [ ] `SPEC.md` gravado na raiz do projeto e relatório de cobertura enviado

---

## Stack e versões  ← OBRIGATÓRIO (Go)

| Componente | Versão mínima | Notas |
|------------|:---:|-------|
| Go | 1.21 | Ferramenta lê a diretiva `go X.YY` do `go.mod` do projeto analisado |
| gofmt | embutido | Não gera código; usado só se a SPEC incluir snippets de exemplo |
| go vet | embutido | Sanidade dos snippets citados na SPEC |
| golangci-lint | 1.55+ | Opcional; irrelevante para a geração da SPEC em si |

## Dependências (go.mod / go get)  ← OBRIGATÓRIO (Go)

```bash
# Esta skill LÊ o go.mod do projeto-alvo; não instala nada.
go list -m all      # inventário de dependências a citar na SECAO 12.1
go mod tidy         # opcional, só para conferir o grafo antes de documentar
```

**Conflitos conhecidos:** nenhum — a skill apenas inspeciona a árvore de código, não a modifica.

## Checklist Go  ← OBRIGATÓRIO (Go)

- [ ] `gofmt -l .` sem saída (só relevante se a SPEC embutir snippets Go)
- [ ] `go vet ./...` sem avisos nos snippets citados
- [ ] `go build ./...` do projeto-alvo confirma que o código lido compila (base confiável para a SPEC)
- [ ] `go test ./...` consultado como fonte de comportamento esperado (não como requisito literal)
- [ ] Versão Go da SECAO 12.1 lida da diretiva `go X.YY` do `go.mod`, não presumida
- [ ] Dependências da SECAO 12.1 vindas do `go.mod`/`go list -m all`, com versões

## Exemplo mínimo compilável  ← OBRIGATÓRIO (Go)

```go
// A skill NÃO gera este programa; ela EXTRAI requisitos de handlers como este.
// Ex.: o handler abaixo vira RF-001 "Consultar saúde do serviço" (ator: cliente HTTP).
package main

import (
	"fmt"
	"net/http"
)

func healthHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "OK") // RF: expõe endpoint de health-check
}

func main() {
	http.HandleFunc("/health", healthHandler)
	_ = http.ListenAndServe(":8080", nil)
}
```

→ Ver [template da SPEC](./references/spec-template.md)

---

## Anti-padrões

| Anti-padrão | Por que é errado | Como corrigir |
|-------------|-------------------|----------------|
| Gerar SPEC de uma única função ou de um `.go` isolado | SPEC descreve capacidade de negócio; um arquivo não é um sistema | Elevar o escopo ao módulo (`internal/<dominio>`) ou ao projeto inteiro |
| Copiar assinaturas de função como RF cru (`func CreateOrder(...)`) | RF descreve o que o sistema faz para o ator, não a API interna | Traduzir para intenção de negócio: "RF-00X: Registrar pedido validando estoque" |
| Deixar seções em branco quando o código não revela o dado | SPEC incompleta engana o leitor sobre cobertura | Usar o placeholder do idioma e marcar `[INFERIDO]`/`[INFERRED]` |
| Presumir versão do Go / dependências | Introduz restrição técnica falsa | Ler `go X.YY` e `go list -m all` do `go.mod` real |
| Confundir `_test.go` com requisito | Teste é evidência de comportamento, não o requisito em si | Usar testes só como pista para redigir o RF/RN correspondente |

## Avaliação de risco

- **Parar e confirmar quando:** o escopo pedido é ambíguo entre "projeto inteiro"
  e "um módulo" — confirmar com o usuário antes de gerar, pois muda toda a numeração.
- **Risco alto:** afirmar RNF de performance/segurança sem evidência (timeout, TLS,
  pool) no código — deve ir como `[INFERIDO]`, nunca como fato.
- **Risco baixo:** preencher seções descritivas (objetivo, escopo) a partir de nomes
  de pacote e comentários de doc já presentes.

## Métricas de sucesso

- 100% das 14 seções do template preenchidas (dados reais ou placeholder + marcador).
- Zero itens inferidos apresentados como fato (todo inferido carrega `[INFERIDO]`/`[INFERRED]`).
- Rastreabilidade: cada RF/RN cita a origem no código (pacote/handler/erro) quando possível.
- `SPEC.md` na raiz + relatório de cobertura entregue em uma única passada.

## Responsável principal

| Papel | Quem |
|-------|------|
| Agent executor | `developer-golang-agent-orchestrator` |
| Revisão humana | Desenvolvedor / Tech Lead |
| Aprovação final | Tech Lead / Product Owner |

## Referências

- Skill espelhada (origem Delphi): `developer-delphi-project-spec`
- Template da SPEC (14 seções): `./references/spec-template.md` (pt-BR) · `./references/spec-template.en.md` (en-US)
- Skill relacionada: `developer-go-architecture-and-design`
- Skill relacionada: `developer-go-master-orchestrator`
- Contraparte de qualidade: `quality-code-review-checklist`
- [Effective Go — go.dev](https://go.dev/doc/effective_go)

---

## Versão interna (arquivo)

| Campo | Valor |
|-------|-------|
| **FileVersion** | 1.0.0 |
| **Política** | `.cursor/VERSION.md` |

## Changelog (este arquivo)

- 1.0.0 (09/08/2026): Criação da skill — gerador de SPEC.md por engenharia reversa de
  código Go (protocolo SCAN→READ→EXTRACT→GENERATE→SAVE+REPORT), espelhando
  `developer-delphi-project-spec` com extração adaptada a fontes Go (funções exportadas,
  handlers `net/http`, comandos CLI `flag`, structs com tags `json:`/`db:`, erros
  `errors.New`/customizados, timeouts/pool/concorrência e `go.mod`). Template de 14 seções
  em `references/spec-template.md` (pt-BR) e `references/spec-template.en.md` (en-US).

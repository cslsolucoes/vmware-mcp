---
name: developer-go-master-orchestrator
description: Ponto de entrada do kit developer-go-* — classifica o cenário GoLang e roteia para a skill correta, combinando várias quando a tarefa é cross-cutting.
model: opus
thinking: extended
category: developer-go
license: MIT
copyright: "Copyright (c) 2026 CSL Tech Solutions"
company: "CSL Tech Solutions"
author: "Claiton de Souza Linhares"
---

# Orquestrador Master — Kit GoLang

## Responsabilidade única

Esta skill é o ponto de entrada único do kit `developer-go-*`. Ela classifica a demanda GoLang, seleciona e ordena as skills especializadas necessárias e delega a execução — **não implementa código diretamente**. Existe separada das demais exatamente para concentrar a lógica de roteamento (cenário → skill), evitando que cada skill técnica precise conhecer as outras. Mesmo princípio do orquestrador Delphi/VueJS: o master coordena, as folhas executam.

## Versão interna (arquivo)

| Campo | Valor |
|-------|-------|
| **FileVersion** | 1.0.0 |
| **Política** | `.cursor/VERSION.md` |

## When to use

- Tarefa GoLang multi-etapa que atravessa duas ou mais skills `developer-go-*`.
- Quando o cenário não é obviamente de um único domínio (ex.: "criar um serviço REST com testes e deploy").
- Onboarding de um novo módulo/serviço Go do zero (build + arquitetura + testes).
- Refatoração ampla que toca linguagem, concorrência, performance e entrega ao mesmo tempo.
- Quando há dúvida sobre **qual** skill Go acionar primeiro.

## When NOT to use

- Tarefa de um único domínio já óbvio → ir direto à skill específica, sem passar por aqui.
- Escrever/ajustar testes de um pacote existente → `developer-go-testing`.
- Dúvida pontual de sintaxe/tipos da linguagem → `developer-go-language-core` / `developer-go-language-types`.
- Configurar apenas o `go.mod`/toolchain → `developer-go-build-toolchain`.
- Só gerar a SPEC do projeto → `developer-go-project-spec`.

## Dependências (skills prévias)

Nenhuma dependência obrigatória. O orquestrador é o ponto de entrada e ele mesmo define a sequência de skills a executar.

## Inputs obrigatórios

| Input | Tipo | Descrição |
|-------|------|-----------|
| `objetivo` | texto | O que precisa ser feito (feature, correção, refatoração, deploy). |
| `escopo` | texto | Módulos/pacotes envolvidos e restrições (versão Go, plataforma-alvo). |
| `criterios_aceite` | lista | Como saber que terminou (testes verdes, binário gerado, endpoint no ar). |

## Workflow executável

1. **Classificar o cenário** — identificar o(s) domínio(s): linguagem, patterns, stdlib, concorrência/performance, qualidade, build/delivery, arquitetura/apps, deploy ou spec.
2. **Consultar a matriz de roteamento** — mapear o cenário para a(s) skill(s) de entrada correta(s).
3. **Delegar/ler a skill de entrada certa** — acionar a skill folha; se o cenário for de domínio único, encerrar aqui.
4. **Combinar em sequência quando cross-cutting** — para cenários que cruzam domínios, executar as skills na ordem indicada, com checkpoint (`go build ./...` + `go test ./...`) entre etapas relevantes.

## Matriz de roteamento

| Cenário | Skill(s) de entrada (ordem) |
|---------|------------------------------|
| Criar novo módulo Go do zero | `developer-go-build-toolchain` → `developer-go-architecture-and-design` |
| Estruturar arquitetura/pacotes de um serviço | `developer-go-architecture-and-design` → `developer-go-patterns-composition` |
| Escrever testes para pacote existente | `developer-go-testing` |
| API REST simples (servidor) | `developer-go-http-server` → `developer-go-stdlib-encoding` |
| Consumir API externa (cliente REST) | `developer-go-http-client-rest` → `developer-go-stdlib-encoding` |
| Ferramenta de linha de comando (CLI) | `developer-go-cli-apps` → `developer-go-build-toolchain` |
| Acesso a banco de dados / persistência | `developer-go-database-access` → `developer-go-error-handling-and-diagnostics` |
| Concorrência básica (goroutines/channels) | `developer-go-concurrency-basics` |
| Pipeline concorrente / worker pool avançado | `developer-go-concurrency-advanced` → `developer-go-performance-and-memory` |
| Debugar erro intermitente / race condition | `developer-go-error-handling-and-diagnostics` → `developer-go-concurrency-advanced` |
| Otimizar CPU/memória (gargalo) | `developer-go-performance-profiling` → `developer-go-performance-and-memory` |
| Serializar/parsear JSON, XML, gob, etc. | `developer-go-stdlib-encoding` |
| Manipular strings, I/O, buffers, arquivos | `developer-go-stdlib-strings-io` → `developer-go-stdlib-collections` |
| Reflexão / metaprogramação (`reflect`) | `developer-go-stdlib-rtti-reflection` |
| Código genérico (type parameters) | `developer-go-language-generics` → `developer-go-language-types` |
| Modelar com interfaces/embedding (OOP em Go) | `developer-go-language-oop` → `developer-go-patterns-structural` |
| Aplicar design pattern (factory/strategy/etc.) | `developer-go-patterns-creational` / `developer-go-patterns-behavioral` |
| Criptografia, hashing, TLS, tokens | `developer-go-crypto-security` |
| Empacotar e versionar release (binários) | `developer-go-packaging-delivery` → `developer-go-build-toolchain` |
| Deploy em servidor Linux (systemd/serviço) | `developer-go-linux-deploy` → `developer-go-packaging-delivery` |
| Recurso avançado da linguagem (generics/reflect/unsafe) | `developer-go-language-advanced` |
| Gerar SPEC do projeto/módulo | `developer-go-project-spec` |

## Mapa completo do kit

| Skill | Família | Responsabilidade |
|-------|---------|------------------|
| `developer-go-language-core` | Linguagem | Sintaxe base, declarações, controle de fluxo, funções, pacotes. |
| `developer-go-language-types` | Linguagem | Sistema de tipos, structs, interfaces, conversões, aliases. |
| `developer-go-language-generics` | Linguagem | Type parameters, constraints, funções/tipos genéricos. |
| `developer-go-language-oop` | Linguagem | "OOP em Go": interfaces, embedding, métodos, polimorfismo. |
| `developer-go-language-advanced` | Linguagem | Recursos avançados: `unsafe`, `//go:` directives, cgo, iteradores. |
| `developer-go-patterns-creational` | Patterns | Padrões criacionais idiomáticos (factory, builder, singleton). |
| `developer-go-patterns-structural` | Patterns | Padrões estruturais (adapter, decorator, facade) em Go. |
| `developer-go-patterns-behavioral` | Patterns | Padrões comportamentais (strategy, observer, state). |
| `developer-go-patterns-composition` | Patterns | Composição sobre herança, embedding e interfaces pequenas. |
| `developer-go-stdlib-collections` | Stdlib | Slices, maps, `container/*`, `sort`, `slices`, `maps`. |
| `developer-go-stdlib-strings-io` | Stdlib | `strings`, `bytes`, `bufio`, `io`, `os`, manipulação de arquivos. |
| `developer-go-stdlib-encoding` | Stdlib | `encoding/json`, `xml`, `gob`, `csv`, serialização/desserialização. |
| `developer-go-stdlib-rtti-reflection` | Stdlib | `reflect`, introspecção de tipos, tags de struct. |
| `developer-go-concurrency-basics` | Concorrência/Performance | Goroutines, channels, `select`, `sync` básico. |
| `developer-go-concurrency-advanced` | Concorrência/Performance | Padrões concorrentes, `context`, pipelines, worker pools, race. |
| `developer-go-performance-and-memory` | Concorrência/Performance | Gestão de memória, alocação, GC, escape analysis, otimização. |
| `developer-go-performance-profiling` | Concorrência/Performance | `pprof`, benchmarks, trace, diagnóstico de gargalos. |
| `developer-go-testing` | Qualidade | `testing`, table-driven tests, mocks, coverage, fuzzing. |
| `developer-go-error-handling-and-diagnostics` | Qualidade | `error` idiomático, `errors.Is/As`, wrapping, panic/recover, logs. |
| `developer-go-build-toolchain` | Build/Delivery | `go build`, `go.mod`, módulos, cross-compilation, flags. |
| `developer-go-packaging-delivery` | Build/Delivery | Versionamento, `ldflags`, release de binários, distribuição. |
| `developer-go-crypto-security` | Build/Delivery | `crypto/*`, TLS, hashing, tokens, segurança de dados. |
| `developer-go-architecture-and-design` | Arquitetura/Apps | Layout de projeto, camadas, DI, fronteiras de pacote. |
| `developer-go-cli-apps` | Arquitetura/Apps | Apps de linha de comando, `flag`/`cobra`, subcomandos. |
| `developer-go-http-client-rest` | Arquitetura/Apps | `net/http` cliente, consumo de REST, retries, timeouts. |
| `developer-go-http-server` | Arquitetura/Apps | `net/http` servidor, handlers, middleware, roteamento REST. |
| `developer-go-database-access` | Arquitetura/Apps | `database/sql`, drivers, pools, transações, queries. |
| `developer-go-linux-deploy` | Deploy | Deploy em Linux, `systemd`, serviços, permissões, empacotamento OS. |
| `developer-go-project-spec` | Spec | Geração da SPEC do projeto/módulo Go (requisitos, escopo). |
| `developer-go-master-orchestrator` | Orquestração | **Esta skill** — classifica o cenário e roteia para as demais. |

## Outputs obrigatórios

| Output | Localização | Formato |
|--------|-------------|---------|
| Plano de roteamento | resposta ao usuário | Markdown (cenário → skills na ordem) |
| Evidência de checkpoint | resposta ao usuário | Saída de `go build ./...` / `go test ./...` |

## Checklist de validação

- [ ] Cenário classificado em uma ou mais famílias do kit.
- [ ] Skill(s) de entrada corretas identificadas via matriz de roteamento.
- [ ] Sequência definida quando a tarefa é cross-cutting (ordem + checkpoints).
- [ ] Nenhuma implementação feita diretamente neste orquestrador.
- [ ] Evidência (`go build`/`go test`) coletada entre etapas relevantes.

## Stack e versões

| Componente | Versão mínima | Notas |
|------------|:---:|-------|
| Go | 1.22.x | Toolchain padrão; `go` modules obrigatório (`go.mod`). |
| gofmt | (bundled) | Formatação canônica — sem discussão de estilo. |
| go vet | (bundled) | Análise estática básica antes de commit. |
| golangci-lint | 1.59.x | Linter agregador recomendado para o kit inteiro. |
| Delve (dlv) | 1.22.x | Debugging quando necessário. |

## Ferramentas do kit

```bash
go version              # >= 1.22.x
gofmt -l ./...          # arquivos fora do formato canônico
go vet ./...            # análise estática
golangci-lint run       # lint agregado (opcional, recomendado)
go build ./...          # gate de compilação
go test ./...           # gate de testes
```

**Conflitos conhecidos:** misturar `GOPATH` legado com módulos causa builds inconsistentes — usar sempre módulos (`GO111MODULE=on`, padrão desde 1.16).

## Checklist Go

- [ ] `gofmt` aplicado (zero diffs de formatação).
- [ ] `go vet ./...` sem apontamentos.
- [ ] `go build ./...` compila sem erros.
- [ ] `go test ./...` verde antes de fechar cada etapa.
- [ ] Tratamento de erro idiomático (`error` retornado, não `panic` em fluxo normal).

## Exemplo mínimo funcional

```go
// Orquestrador não implementa; ilustra o gate mínimo entre etapas.
package main

import "fmt"

func main() {
	// Cada delegação a uma skill folha deve passar por: go build && go test.
	fmt.Println("kit developer-go: roteamento OK")
}
```

→ As skills folhas trazem os exemplos completos e compiláveis por domínio.

## Anti-padrões

| Anti-padrão | Por que é errado | Como corrigir |
|-------------|-----------------|---------------|
| Pular direto para a skill técnica sem checar se o cenário é cross-cutting | Perde etapas dependentes (ex.: deploy sem packaging), gera retrabalho | Classificar o cenário e consultar a matriz antes de delegar |
| Tratar Go como Delphi/Pascal via find-replace | Idioma diferente (erros como valor, composição, sem exceções de fluxo) | Usar as skills `developer-go-language-*` para idiomas corretos |
| Usar framework 3rd-party quando a stdlib já resolve | Dependência desnecessária, superfície de ataque e build maiores | Preferir `net/http`, `database/sql`, `flag` da stdlib primeiro |
| Rodar skills em paralelo ignorando dependências | Skill B falha porque A não preparou o ambiente (ex.: sem `go.mod`) | Seguir a ordem da matriz com checkpoint entre etapas |
| Invocar o orquestrador para tarefa de skill única | Overhead e contexto desperdiçado | Ir direto à skill específica |

## Avaliação de risco

- **Parar e confirmar quando:** deploy em produção, alteração de esquema de banco, rotação de segredos/certificados ou remoção de pacotes.
- **Risco alto:** cross-compilation para plataforma não testada, mudanças em código concorrente (race), migrações de dados.
- **Risco baixo:** classificar cenário, ler skills folhas, rodar `go build`/`go test`/`go vet` — autônomo.

## Métricas de sucesso

- 100% dos cenários roteados para a(s) skill(s) correta(s) na primeira tentativa.
- `go build ./...` e `go test ./...` verdes ao final de cada cenário orquestrado.
- Zero implementação feita no próprio orquestrador (só roteamento).
- Checkpoints de qualidade registrados entre skills numa tarefa cross-cutting.

## Responsável principal

| Papel | Quem |
|-------|------|
| Agent executor | `developer-golang-agent-orchestrator` |
| Revisão humana | Desenvolvedor / Tech Lead |
| Aprovação final | Tech Lead |

## Referências

- https://go.dev/doc/
- https://go.dev/doc/effective_go
- Skills-filhas do kit (30 no total):
  - Linguagem: `developer-go-language-core`, `developer-go-language-types`, `developer-go-language-generics`, `developer-go-language-oop`, `developer-go-language-advanced`
  - Patterns: `developer-go-patterns-creational`, `developer-go-patterns-structural`, `developer-go-patterns-behavioral`, `developer-go-patterns-composition`
  - Stdlib: `developer-go-stdlib-collections`, `developer-go-stdlib-strings-io`, `developer-go-stdlib-encoding`, `developer-go-stdlib-rtti-reflection`
  - Concorrência/Performance: `developer-go-concurrency-basics`, `developer-go-concurrency-advanced`, `developer-go-performance-and-memory`, `developer-go-performance-profiling`
  - Qualidade: `developer-go-testing`, `developer-go-error-handling-and-diagnostics`
  - Build/Delivery: `developer-go-build-toolchain`, `developer-go-packaging-delivery`, `developer-go-crypto-security`
  - Arquitetura/Apps: `developer-go-architecture-and-design`, `developer-go-cli-apps`, `developer-go-http-client-rest`, `developer-go-http-server`, `developer-go-database-access`
  - Deploy: `developer-go-linux-deploy`
  - Spec: `developer-go-project-spec`

## Versão interna (arquivo)

| Campo | Valor |
|-------|-------|
| **FileVersion** | 1.0.0 |
| **Política** | `.cursor/VERSION.md` |

## Changelog (este arquivo)

- 1.0.0 (09/08/2026): Criação do orquestrador master do kit developer-go-* (30 skills).

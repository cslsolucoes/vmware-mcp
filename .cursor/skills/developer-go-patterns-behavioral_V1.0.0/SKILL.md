---
name: developer-go-patterns-behavioral
description: Padrões comportamentais idiomáticos em Go — Strategy, Observer, Command e Iterator — usando interfaces, tipos função, channels e range-over-func.
model: sonnet
thinking: normal
category: developer-go
license: MIT
copyright: "Copyright (c) 2026 CSL Tech Solutions"
company: "CSL Tech Solutions"
author: "Claiton de Souza Linhares"
---

# Padrões Comportamentais em Go

## Responsabilidade única

Esta skill resolve o problema de aplicar os padrões comportamentais clássicos do GoF (Strategy, Observer, Command, Iterator) de forma idiomática em Go — sem depender de herança, sem overload de métodos e sem simular dispatch virtual que a linguagem não tem. Existe separada de `developer-go-language-oop` porque ali o foco é o mecanismo (receiver, embedding, interface mínima); aqui o foco é como compor esses mecanismos para resolver quatro problemas recorrentes: troca de algoritmo em runtime, notificação de eventos entre partes desacopladas, encapsulamento de uma ação executável/desfazível e iteração customizada sobre uma coleção.

## When to use

- Algoritmo intercambiável em runtime sem `switch`/`if` gigante (Strategy)
- Notificação de eventos entre partes desacopladas do sistema (Observer)
- Ação que precisa ser enfileirada, logada, repetida ou desfeita (Command)
- Percorrer coleção customizada com sintaxe idiomática `for range` (Iterator)

## When NOT to use

- Herança/polimorfismo de dados sem alternância de comportamento → usar `developer-go-language-oop` (embedding, receivers)
- Composição estrutural pura, sem troca de algoritmo → usar `developer-go-patterns-composition`
- Sincronização fina entre goroutines (mutex, waitgroup, context cancelation) → skill de concorrência Go, fora deste escopo
- Serialização de comandos/eventos para fila externa (JSON/DB) → skill de RTL/streams Go, não esta

## Inputs obrigatórios

| Input | Tipo | Descrição |
|-------|------|-----------|
| `module-path` | `string` | Caminho do módulo (`go.mod`) onde o padrão será implementado |
| `padrao-alvo` | `string` | Qual padrão comportamental aplicar: Strategy, Observer, Command ou Iterator |
| `contrato` | `[]string` | Assinatura do método/função que a interface ou tipo função deve expor |

## Dependências (skills prévias)

| Skill | Quando executar antes |
|-------|-----------------------|
| `developer-go-language-types` | Definir os tipos base (structs, tipos nomeados, func types) antes de aplicar os padrões comportamentais sobre eles |

## Workflow executável

1. **Strategy** — declarar uma `interface` de comportamento e, quando o algoritmo cabe numa função, usar um tipo função (`type StrategyFunc func(...)`) que implementa a interface via método adaptador:

```go
type SortStrategy interface {
    Sort(data []int) []int
}

type StrategyFunc func(data []int) []int

func (f StrategyFunc) Sort(data []int) []int { return f(data) }
```

2. **Observer** — escolher entre canal (`chan`) para notificação assíncrona desacoplada ou lista de callbacks (`[]func(Event)`) para notificação síncrona in-process; nunca reimplementar polling manual quando um canal resolve:

```go
type Subject struct {
    observers []func(event string)
}

func (s *Subject) Subscribe(fn func(event string)) {
    s.observers = append(s.observers, fn)
}

func (s *Subject) Notify(event string) {
    for _, fn := range s.observers {
        fn(event)
    }
}
```

3. **Command** — encapsular uma ação como `struct` com método `Execute() error` (ou `type CommandFunc func() error` para ações simples sem estado); acumular numa slice para suportar histórico/undo:

```go
type Command interface {
    Execute() error
}

type CommandFunc func() error

func (f CommandFunc) Execute() error { return f() }
```

4. **Iterator** — em Go ≥1.23 usar `range-over-func` (`func(yield func(T) bool)`); em módulos com `go` directive anterior, usar canal com goroutine produtora e `close` garantido ao final:

```go
func Iterate(items []string) func(yield func(string) bool) {
    return func(yield func(string) bool) {
        for _, it := range items {
            if !yield(it) {
                return
            }
        }
    }
}
// uso: for it := range Iterate(items) { ... }
```

5. Rodar `gofmt`, `go vet` e `go build ./...` antes de considerar concluído.

## Outputs obrigatórios

| Output | Localização | Formato |
|--------|-------------|---------|
| Interface/tipo função do padrão aplicado | `<pacote>/<arquivo>.go` | Go |
| Testes de comportamento (troca de strategy, notificação, execução/undo, iteração) | `<pacote>/<arquivo>_test.go` | Go |

## Checklist de validação

- [ ] Strategy implementada via `interface` + `type StrategyFunc func(...)` quando aplicável, sem `switch` gigante substituindo polimorfismo
- [ ] Observer usa canal ou lista de callbacks, nunca polling ativo
- [ ] Command encapsula a ação com `Execute() error`; histórico/undo usa slice, não variável global
- [ ] Iterator segue `range-over-func` (Go ≥1.23) ou canal fechado corretamente (sem goroutine leak)

---

## Stack e versões  ← OBRIGATÓRIO (Go)

| Componente | Versão mínima | Notas |
|------------|:---:|-------|
| Go | 1.21 | `go.mod` declara `go 1.21` ou superior |
| gofmt | embutido | Formatação obrigatória, sem exceções |
| go vet | embutido | Rodar antes de qualquer commit |
| golangci-lint | 1.55+ | Lint agregador (opcional mas recomendado) |

## Dependências (go.mod / go get)  ← OBRIGATÓRIO (Go)

```bash
go mod init <module-path>
go get <pacote>@<versão>
go mod tidy
```

**Conflitos conhecidos:** nenhum conhecido para este tema.

## Checklist Go  ← OBRIGATÓRIO (Go)

- [ ] `gofmt -l .` sem saída (código formatado)
- [ ] `go vet ./...` sem avisos
- [ ] `go build ./...` sem erros
- [ ] `go test ./...` verde (quando aplicável ao tema da skill)
- [ ] Erros tratados explicitamente (`if err != nil`), nunca descartados com `_` sem justificativa
- [ ] Identificadores exportados só quando fazem parte do contrato público do pacote

## Exemplo mínimo compilável  ← OBRIGATÓRIO (Go)

```go
// Snippet ≤ 15 linhas — bloco maior → ./exemplos/
package main

import "fmt"

func main() {
    // demonstrar o uso correto desta skill
    fmt.Println("OK")
}
```

→ Ver [exemplos completos](./exemplos/README.md)

---

## Anti-padrões

| Anti-padrão | Por que é errado | Como corrigir |
|-------------|-------------------|----------------|
| `switch` gigante sobre um tipo/enum para escolher comportamento (simulando Strategy sem interface) | Cada novo caso exige editar a função central; viola open/closed e dificulta teste isolado do algoritmo | Extrair cada ramo para uma implementação de `interface` ou `StrategyFunc` e injetar a escolhida |
| Observer com polling (`for { time.Sleep(...); check() }`) em vez de canal ou callback | Desperdiça CPU, introduz latência artificial e não escala com o número de observers | Registrar callback em slice ou expor um `chan Event` e notificar no momento do evento |
| Canal de Iterator aberto por goroutine produtora sem `close` ao final ou sem dreno pelo consumidor | Goroutine leak — a goroutine produtora bloqueia para sempre esperando o consumidor ler | Sempre `defer close(ch)` na goroutine produtora e consumir o canal até esgotar (ou usar `range-over-func`, que interrompe a produção via `yield` retornando `false`) |
| Command sem estado isolado (mutação direta de variável global dentro de `Execute`) | Impede undo/redo confiável e cria acoplamento oculto entre comandos que leem estado global | Encapsular o estado necessário nos campos do próprio `Command` e devolver/receber via parâmetros explícitos |

## Avaliação de risco

- **Parar e confirmar quando:** a mudança de Strategy/Command afeta uma API pública já consumida por outros pacotes/serviços
- **Risco alto:** canal de Observer/Iterator sem `close` correspondente em caminho de erro (goroutine leak silencioso em produção)
- **Risco baixo:** adicionar uma nova implementação de `StrategyFunc` ou um novo `Command` a um conjunto já existente

## Métricas de sucesso

- Zero goroutines vazadas relacionadas a canais de Observer/Iterator (validável com `go test -race` e `runtime.NumGoroutine()` antes/depois)
- 100% dos `Command` com `Execute() error` testado isoladamente (caminho de sucesso e de erro)
- Nenhum `switch`/`if-else` com mais de 3 ramos substituindo uma Strategy que deveria ser polimórfica

## Responsável principal

| Papel | Quem |
|-------|------|
| Agent executor | `developer-golang-agent-orchestrator` |
| Revisão humana | Desenvolvedor / Tech Lead |
| Aprovação final | Tech Lead |

## Referências

- [Effective Go — Interfaces and types](https://go.dev/doc/effective_go#interfaces_and_types)
- Skill relacionada: `developer-go-concurrency-basics`
- Skill relacionada: `developer-go-language-oop`
- Skill relacionada: `developer-go-language-types`

---

## Versão interna (arquivo)

| Campo | Valor |
|-------|-------|
| **FileVersion** | 1.0.0 |
| **Política** | `.cursor/VERSION.md` |

## Changelog (este arquivo)

- 1.0.0 (09/08/2026): Criação da skill — padrões comportamentais idiomáticos em Go (Strategy via interface + tipo função, Observer via channel/callbacks, Command com Execute/undo, Iterator via range-over-func/canal).

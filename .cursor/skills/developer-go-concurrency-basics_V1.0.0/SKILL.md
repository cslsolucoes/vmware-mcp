---
name: developer-go-concurrency-basics
description: Concorrência básica em Go — goroutines, channels (unbuffered/buffered), select e sync.WaitGroup, sob o princípio "share memory by communicating".
model: sonnet
thinking: normal
category: developer-go
license: MIT
copyright: "Copyright (c) 2026 CSL Tech Solutions"
company: "CSL Tech Solutions"
author: "Claiton de Souza Linhares"
---

# Concorrência Básica em Go

## Responsabilidade única

Esta skill cobre o modelo de concorrência fundamental de Go: goroutines (`go func()`), channels
(`chan`, unbuffered vs buffered), a instrução `select` para multiplexar canais e `sync.WaitGroup`
para aguardar um grupo de goroutines terminar. O fio condutor é o princípio idiomático de Go
"share memory by communicating" — comunicar por channels em vez de compartilhar memória protegida
por locks. Ela existe separada de tópicos avançados (cancelamento via `context`, `sync.Mutex`/
`sync.RWMutex` para estado compartilhado explícito, `errgroup`, pools de workers complexos) porque
trata apenas das primitivas de base necessárias para escrever e entender concorrência simples e
correta em Go — a fundação sobre a qual os padrões avançados são construídos.

## When to use

- Disparar uma unidade de trabalho independente em paralelo com `go func()`
- Comunicar resultados ou sinais entre goroutines via `chan`
- Aguardar N goroutines concluírem antes de prosseguir (`sync.WaitGroup`)
- Multiplexar múltiplos channels ou implementar timeout simples com `select`
- Decidir entre channel unbuffered (sincronização estrita) e buffered (desacoplamento controlado)

## When NOT to use

- Necessidade de cancelamento propagado, deadlines ou valores por requisição → `context`
  (`developer-go-concurrency-advanced`)
- Proteção de estado mutável compartilhado com `sync.Mutex`/`sync.RWMutex`/`sync.Once`/`sync/atomic`
  → `developer-go-concurrency-advanced`
- Orquestração de grupos de goroutines com propagação de erro (`errgroup.Group`) → `developer-go-concurrency-advanced`
- Sintaxe de funções, pacotes e `go.mod` sem concorrência → `developer-go-language-core`
- Coleções (slices/maps) sem uso concorrente → `developer-go-stdlib-collections`

## Inputs obrigatórios

| Input | Tipo | Descrição |
|-------|------|-----------|
| `unidade de trabalho` | função/closure | Código que pode rodar de forma independente e concorrente |
| `sinal de conclusão` | `chan struct{}` ou `sync.WaitGroup` | Mecanismo para o chamador saber quando o trabalho terminou |
| `módulo Go válido` | `go.mod` | Projeto com `go 1.21` ou superior declarado |

## Dependências (skills prévias)

| Skill | Quando executar antes |
|-------|-----------------------|
| `developer-go-language-core` | Sintaxe de funções, closures e `go.mod` são pré-requisito para entender goroutines |

## Workflow executável

1. Identificar a unidade de trabalho que pode rodar de forma independente (I/O, cálculo isolado)
2. Escolher o canal de comunicação: `chan T` sem buffer (rendezvous) ou `chan T, N` (buffer de N itens)
3. Disparar a goroutine com `go func() { ... }()`, garantindo que ela sempre termina (sem loop infinito)
4. Usar `sync.WaitGroup` quando só interessa saber que todas terminaram (sem trocar valores)
5. Usar `select` quando é preciso aguardar o primeiro entre vários channels, ou aplicar timeout
6. Fechar o channel (`close(ch)`) apenas no lado produtor, nunca no consumidor
7. Rodar `go test -race ./...` para confirmar ausência de data races

> Snippets de código inline ≤ 15 linhas. Blocos maiores → mover para `./exemplos/` e referenciar:
> `→ Ver [exemplo completo](./exemplos/nome.go)`

## Outputs obrigatórios

| Output | Localização | Formato |
|--------|-------------|---------|
| Código concorrente com goroutines/channels | `*.go` do pacote alvo | Go |
| Evidência de ausência de data race | saída de `go test -race ./...` | Log de terminal |

## Checklist de validação

- [ ] Toda goroutine lançada tem um caminho claro de término (sem leak)
- [ ] Channel é fechado apenas pelo produtor, nunca pelo consumidor
- [ ] `select` cobre todos os casos relevantes (inclui `default` ou timeout quando necessário)
- [ ] `sync.WaitGroup.Add` é chamado antes de `go func()`, nunca dentro dela
- [ ] `go test -race ./...` executado e verde

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

**Conflitos conhecidos:** goroutines vazadas (sem forma de terminar) causam memory leak — sempre ter um caminho de saída.

## Checklist Go  ← OBRIGATÓRIO (Go)

- [ ] `gofmt -l .` sem saída (código formatado)
- [ ] `go vet ./...` sem avisos
- [ ] `go build ./...` sem erros
- [ ] `go test -race ./...` verde (concorrência sempre testada com race detector)
- [ ] Erros tratados explicitamente (`if err != nil`), nunca descartados com `_` sem justificativa
- [ ] Toda goroutine lançada tem um caminho claro de término (sem leak)

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
| Goroutine leak (`go func()` sem canal de saída nem condição de término) | A goroutine fica presa para sempre bloqueada em envio/recebimento, consumindo memória e stack até o processo morrer | Garantir um caminho de saída: canal de cancelamento, `select` com `default`, ou (em cenários avançados) `context` |
| Fechar um channel do lado do consumidor, ou fechá-lo mais de uma vez | `close` de channel já fechado, ou enviar em channel fechado, causa `panic: close of closed channel` / `panic: send on closed channel` em runtime | Fechar sempre no produtor, uma única vez; consumidor apenas lê (`v, ok := <-ch`) |
| Acessar variável/struct compartilhada a partir de múltiplas goroutines sem channel nem `sync.Mutex` | Data race — comportamento indefinido, resultados incorretos e falhas intermitentes difíceis de reproduzir | Comunicar o valor por channel ("share memory by communicating") ou proteger com `sync.Mutex` (`developer-go-concurrency-advanced`) |
| `wg.Add(1)` chamado de dentro da goroutine em vez de antes de `go func()` | Condição de corrida entre `Add` e `Wait` — o `Wait` pode retornar antes de todas as goroutines terem sido contabilizadas | Sempre chamar `Add` na goroutine que dispara, antes do `go func()` |

## Avaliação de risco

- **Parar e confirmar quando:** a solução exige compartilhar estado mutável entre goroutines sem
  um caminho óbvio via channel — nesse caso, avaliar com o usuário se o cenário pertence a
  `developer-go-concurrency-advanced` (mutex/atomic/context) antes de prosseguir
- **Risco alto:** goroutine sem caminho de término (leak de memória e stack em produção) ou channel
  fechado no lado errado (panic em runtime)
- **Risco baixo:** uso de `select` com `default` para operações não bloqueantes, ou `sync.WaitGroup`
  em fan-out/fan-in simples com número fixo e conhecido de goroutines

## Métricas de sucesso

- Zero avisos do `go vet` e zero data races reportados por `go test -race ./...`
- 100% das goroutines lançadas possuem caminho de término demonstrável no código ou em teste
- Checklist de validação 100% marcado antes de considerar a tarefa concluída

## Responsável principal

| Papel | Quem |
|-------|------|
| Agent executor | `developer-golang-agent-orchestrator` |
| Revisão humana | Desenvolvedor / Tech Lead |
| Aprovação final | Tech Lead |

## Referências

- [Effective Go — Concurrency](https://go.dev/doc/effective_go#concurrency)
- [A Tour of Go — Concurrency](https://go.dev/tour/concurrency/1)
- Skill relacionada: `developer-go-concurrency-advanced`

---

## Versão interna (arquivo)

| Campo | Valor |
|-------|-------|
| **FileVersion** | 1.0.0 |
| **Política** | `.cursor/VERSION.md` |

## Changelog (este arquivo)

- 1.0.0 (09/08/2026): Criação da skill — concorrência básica em Go (goroutines, channels, select,
  sync.WaitGroup) seguindo o template V2.0 com as 17 seções canônicas.

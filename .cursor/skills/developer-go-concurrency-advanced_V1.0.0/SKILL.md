---
name: developer-go-concurrency-advanced
description: Concorrência avançada em Go — context (cancelamento/timeout/valores), sync.Mutex/RWMutex, sync/atomic e worker pools com backpressure, validados com o race detector.
model: sonnet
thinking: normal
category: developer-go
license: MIT
copyright: "Copyright (c) 2026 CSL Tech Solutions"
company: "CSL Tech Solutions"
author: "Claiton de Souza Linhares"
---

# Concorrência Avançada em Go

## Responsabilidade única

Esta skill cobre os mecanismos avançados de concorrência em Go que vão além de goroutines e
channels simples: propagação de cancelamento e deadlines via `context.Context`, proteção de
estado mutável compartilhado com `sync.Mutex`/`sync.RWMutex`, operações atômicas lock-free com
`sync/atomic`, e o padrão worker pool com canal de trabalho limitado para controlar concorrência
e aplicar backpressure. Ela existe separada de `developer-go-concurrency-basics` porque trata de
cenários onde comunicar por channel puro não basta — é preciso cancelar trabalho em andamento,
proteger memória compartilhada explicitamente, ou limitar quantos workers rodam ao mesmo tempo.

## When to use

- Propagar cancelamento ou deadline através de uma cadeia de chamadas (`context.WithCancel`,
  `context.WithTimeout`, `context.WithDeadline`)
- Proteger uma struct ou mapa compartilhado entre goroutines com `sync.Mutex`/`sync.RWMutex`
- Incrementar contadores ou trocar flags atomicamente sem lock, via `sync/atomic`
- Limitar o número de goroutines concorrentes processando uma fila de trabalho (worker pool)
- Diagnosticar e eliminar data races com `go test -race ./...`

## When NOT to use

- Disparar uma goroutine isolada e aguardar com `sync.WaitGroup`, sem estado compartilhado nem
  cancelamento → `developer-go-concurrency-basics`
- Comunicar resultados simples entre duas goroutines via `chan` sem necessidade de timeout →
  `developer-go-concurrency-basics`
- Sintaxe de funções, closures e `go.mod` sem concorrência → `developer-go-language-core`
- Serialização de dados (JSON/CSV) processados pelos workers → `developer-go-stdlib-encoding`

## Inputs obrigatórios

| Input | Tipo | Descrição |
|-------|------|-----------|
| `context pai` | `context.Context` | Contexto recebido pela função (nunca criado do zero em código de produção, exceto na raiz) |
| `estado compartilhado` | struct/mapa | Dado mutável acessado por mais de uma goroutine, a ser protegido |
| `capacidade do pool` | `int` | Número máximo de workers concorrentes, definido conforme recurso limitante (CPU, I/O, rate limit externo) |

## Dependências (skills prévias)

| Skill | Quando executar antes |
|-------|-----------------------|
| `developer-go-concurrency-basics` | Goroutines, channels e `sync.WaitGroup` são pré-requisito para entender por que `context`/mutex/pools existem |

## Workflow executável

1. Receber `context.Context` como primeiro parâmetro de toda função que faz I/O ou trabalho
   cancelável — nunca armazená-lo em struct
2. Derivar `context.WithTimeout`/`WithCancel` apenas no ponto que inicia a operação; sempre chamar
   `defer cancel()` para liberar recursos do contexto
3. Escolher `sync.Mutex` para acesso exclusivo (leitura+escrita) ou `sync.RWMutex` quando leituras
   são muito mais frequentes que escritas
4. Usar `sync/atomic` apenas para contadores/flags simples; operações condicionais complexas exigem
   mutex, não CAS manual espalhado pelo código
5. Dimensionar o worker pool com um channel de trabalho de capacidade fixa (buffer limitado) para
   aplicar backpressure — produtor bloqueia quando a fila está cheia
6. Propagar `ctx.Done()` dentro do loop de cada worker para encerrar antes do fim da fila, se cancelado
7. Rodar `go test -race ./...` a cada alteração em código concorrente antes de considerar concluído

```go
// Passo 2 — timeout com liberação garantida do contexto
ctx, cancel := context.WithTimeout(parentCtx, 3*time.Second)
defer cancel()
if err := doWork(ctx); err != nil {
    return fmt.Errorf("doWork: %w", err)
}
```

```go
// Passo 5 — worker pool com backpressure via channel limitado
jobs := make(chan Job, 10) // buffer = backpressure, não ilimitado
var wg sync.WaitGroup
for i := 0; i < 4; i++ { // 4 workers concorrentes
    wg.Add(1)
    go func() {
        defer wg.Done()
        for j := range jobs {
            process(j)
        }
    }()
}
```

> Snippets de código inline ≤ 15 linhas. Blocos maiores → mover para `./exemplos/` e referenciar:
> `→ Ver [exemplo completo](./exemplos/nome.go)`

## Outputs obrigatórios

| Output | Localização | Formato |
|--------|-------------|---------|
| Código concorrente com `context`/mutex/atomic/worker pool | `*.go` do pacote alvo | Go |
| Evidência de ausência de data race | saída de `go test -race ./...` | Log de terminal |

## Checklist de validação

- [ ] `context.Context` é sempre o primeiro parâmetro e nunca é guardado em campo de struct
- [ ] Todo `context.WithCancel`/`WithTimeout`/`WithDeadline` tem `defer cancel()` correspondente
- [ ] `sync.Mutex`/`sync.RWMutex` nunca é copiado por valor (structs que o contêm são passadas por ponteiro)
- [ ] Worker pool usa channel de trabalho com capacidade limitada (backpressure explícito)
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

**Conflitos conhecidos:** `context.Context` deve ser o primeiro parâmetro de funções que fazem I/O ou trabalho cancelável.

## Checklist Go  ← OBRIGATÓRIO (Go)

- [ ] `gofmt -l .` sem saída (código formatado)
- [ ] `go vet ./...` sem avisos
- [ ] `go build ./...` sem erros
- [ ] `go test -race ./...` verde
- [ ] Erros tratados explicitamente (`if err != nil`), nunca descartados com `_` sem justificativa
- [ ] Todo `context.Context` recebido é propagado, nunca armazenado em struct

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
| Guardar `context.Context` em campo de struct (`type S struct { ctx context.Context }`) | O contexto fica desatualizado (cancelamento/deadline errados) e mistura ciclo de vida do objeto com ciclo de vida da requisição, indo contra `go vet`/idioma da linguagem | Receber `ctx context.Context` como primeiro parâmetro de cada método que precisa dele, nunca armazenar |
| Copiar por valor uma struct que contém `sync.Mutex`/`sync.RWMutex` | O mutex copiado protege uma cópia diferente do lock original — múltiplas goroutines destravam mutexes distintos e a exclusão mútua deixa de existir | Sempre passar a struct com mutex por ponteiro (`*T`); `go vet` detecta cópias de `sync.Mutex` |
| Worker pool com channel de trabalho ilimitado (`make(chan Job)` sem buffer conhecido, ou goroutine por item sem limite) | Sem backpressure, o produtor enfileira mais rápido que os workers consomem — memória cresce sem controle até esgotar o processo | Definir capacidade fixa no channel de trabalho e/ou limitar goroutines concorrentes com um semáforo (channel `struct{}` de tamanho N) |
| Usar `context.WithValue` para passar dependências obrigatórias (logger, conexão de banco, config) | Esconde o contrato da função, quebra type-safety (valor é `interface{}`) e dificulta rastrear de onde vem cada dado | Passar dependências obrigatórias como parâmetro explícito; reservar `WithValue` só para metadados opcionais de requisição (ex.: trace ID) |

## Avaliação de risco

- **Parar e confirmar quando:** a solução exige múltiplos locks adquiridos em ordens potencialmente
  diferentes por goroutines distintas — validar com o usuário a ordem canônica de aquisição antes
  de prosseguir, para evitar deadlock
- **Risco alto:** mutex copiado por valor, `context.Context` armazenado em struct, ou worker pool
  sem limite de concorrência/buffer (todos produzem falhas intermitentes difíceis de reproduzir em produção)
- **Risco baixo:** uso de `sync/atomic` para um único contador, ou `context.WithTimeout` em uma
  chamada de I/O isolada e bem delimitada

## Métricas de sucesso

- Zero avisos do `go vet` (inclui detecção de cópia de `sync.Mutex`) e zero data races em `go test -race ./...`
- 100% dos contextos derivados (`WithCancel`/`WithTimeout`/`WithDeadline`) possuem `defer cancel()` correspondente
- Checklist de validação 100% marcado antes de considerar a tarefa concluída

## Responsável principal

| Papel | Quem |
|-------|------|
| Agent executor | `developer-golang-agent-orchestrator` |
| Revisão humana | Desenvolvedor / Tech Lead |
| Aprovação final | Tech Lead |

## Referências

- [pkg.go.dev — context](https://pkg.go.dev/context)
- [pkg.go.dev — sync/atomic](https://pkg.go.dev/sync/atomic)
- Skill relacionada: `developer-go-concurrency-basics`

---

## Versão interna (arquivo)

| Campo | Valor |
|-------|-------|
| **FileVersion** | 1.0.0 |
| **Política** | `.cursor/VERSION.md` |

## Changelog (este arquivo)

- 1.0.0 (09/08/2026): Criação da skill — concorrência avançada em Go (context, sync.Mutex/RWMutex,
  sync/atomic, worker pools com backpressure, race detector) seguindo o template V2.0 com as 17
  seções canônicas.

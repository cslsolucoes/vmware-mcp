---
name: developer-go-performance-and-memory
description: Ensina a investigar e corrigir gargalos de performance e uso de memória em Go — escape analysis, comportamento do garbage collector concorrente e a escolha entre value e pointer semantics — para quem otimiza um pacote sem recorrer a profiling detalhado.
model: sonnet
thinking: normal
category: developer-go
license: MIT
copyright: "Copyright (c) 2026 CSL Tech Solutions"
company: "CSL Tech Solutions"
author: "Claiton de Souza Linhares"
---

# Performance e Memória em Go

## Responsabilidade única

Esta skill cobre o raciocínio sobre alocação de memória e desempenho em Go a partir do
comportamento da linguagem e do runtime: como o escape analysis do compilador decide entre
stack e heap, como o garbage collector concorrente tri-color impacta latência e throughput,
e como a escolha entre value semantics e pointer semantics afeta cópias e pressão sobre o GC.
Ela NÃO substitui medição com ferramentas — é o passo de raciocínio e correção estrutural que
antecede ou acompanha o profiling, e existe separada dele porque nem toda otimização exige
`pprof`: muitas decisões (receiver por valor vs ponteiro, struct grande em slice, closure que
captura por referência) são identificáveis por leitura de código e `-gcflags="-m"`.

## When to use

- Investigar por que um trecho de código aloca mais do que o esperado na heap.
- Decidir entre passar/retornar por valor ou por ponteiro em função de custo de cópia vs GC.
- Reduzir pressão sobre o garbage collector antes de recorrer a profiling detalhado.
- Revisar código com muitas alocações pequenas e de curta duração em hot path.
- Explicar a um revisor por que uma variável "escapou" para a heap via `-gcflags="-m"`.

## When NOT to use

- Medição de CPU/memória em runtime real com `pprof`, `trace` ou benchmarks (`testing.B`) →
  use `developer-go-performance-profiling`.
- Fundamentos de structs, interfaces e receiver ponteiro/valor sem foco em performance →
  use `developer-go-language-types`.
- Concorrência (goroutines, channels, race conditions) → use `developer-go-concurrency-basics`.
- Padrões de projeto estruturais/comportamentais sem foco em memória → use
  `developer-go-patterns-structural` ou `developer-go-patterns-behavioral`.

## Inputs obrigatórios

| Input | Tipo | Descrição |
|-------|------|-----------|
| Código Go alvo | `.go` | Pacote ou função com suspeita de alocação excessiva ou hot path crítico |
| `go.mod` do módulo | Arquivo | Define a versão mínima de Go disponível (afeta flags e stdlib) |
| Saída de `go build -gcflags="-m"` (se houver) | Texto/log | Evidência do escape analysis atual, quando já coletada |

## Dependências (skills prévias)

| Skill | Quando executar antes |
|-------|-----------------------|
| `developer-go-language-types` | Garantir domínio de value vs pointer semantics e receivers antes de otimizar memória |

## Workflow executável

1. Confirmar o hot path com evidência (contagem de chamadas, volume de dados) antes de mexer —
   nunca otimizar por suposição.
2. Rodar escape analysis para ver o que já escapa para a heap.
   ```bash
   go build -gcflags="-m" ./... 2> escape.log
   ```
3. Ler as linhas `escapes to heap` e `moved to heap` e localizar a causa (ponteiro retornado,
   interface, closure, slice que cresce).
   ```go
   func newBuffer() *bytes.Buffer {
       b := bytes.Buffer{} // escapa: ponteiro retornado sai da função
       return &b
   }
   ```
4. Trocar ponteiro por valor quando o dado é pequeno e não precisa sobreviver à função.
   ```go
   type Point struct{ X, Y int } // 16 bytes: value semantics evita heap
   func Origin() Point { return Point{} }
   ```
5. Pré-alocar capacidade conhecida em slices/maps para evitar realocações sucessivas.
   ```go
   items := make([]Item, 0, expectedLen) // uma alocação em vez de várias
   ```
6. Evitar que closures capturem ponteiros grandes desnecessariamente; capturar só o necessário.
   ```go
   id := customer.ID // captura só o campo, não o struct inteiro
   go func() { process(id) }()
   ```
7. Reavaliar com `-gcflags="-m"` e, se a suspeita persistir, encaminhar para
   `developer-go-performance-profiling` (pprof/benchmarks) antes de otimizar mais a fundo.

## Outputs obrigatórios

| Output | Localização | Formato |
|--------|-------------|---------|
| Código corrigido (alocação reduzida) | Pacote Go alvo (`*.go`) | Go |
| Evidência de escape analysis (antes/depois) | Log ou comentário no PR/plano | Texto |

## Checklist de validação

- [ ] Hot path identificado com evidência, não por suposição
- [ ] `go build -gcflags="-m"` comparado antes/depois da mudança
- [ ] Nenhuma otimização alterou semântica de negócio sem confirmação
- [ ] Slices/maps com capacidade conhecida usam `make` com capacidade pré-alocada
- [ ] Receiver (ponteiro vs valor) permanece consistente em todos os métodos do mesmo tipo

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
| Alocar dentro de loop quente (`append` sem capacidade, `new`/composite literal a cada iteração) | Gera pressão constante sobre o GC e degrada throughput em hot path | Pré-alocar fora do loop com `make(..., 0, cap)` e reaproveitar o buffer |
| Retornar ponteiro para variável local só "por hábito", sem necessidade de mutação ou tipo grande | Força o escape analysis a mover a variável para a heap desnecessariamente | Retornar por valor quando o tipo é pequeno e não precisa sobreviver à função |
| Passar struct grande por valor em função chamada com frequência | Cada chamada copia todos os bytes da struct, custando CPU e cache | Usar receiver/parâmetro por ponteiro quando a struct é grande (ex.: >3 campos ou >64 bytes) |
| Otimizar memória sem medir antes (achismo) | Risco de otimizar o lugar errado e complicar código sem ganho real | Confirmar o hot path com evidência (contagem/volume) antes de qualquer mudança |
| Ignorar `-gcflags="-m"` e assumir que "parece que não aloca" | Escapes de heap não são óbvios pela leitura superficial do código | Rodar escape analysis e ler `escapes to heap` / `moved to heap` antes de concluir |

## Avaliação de risco

- **Parar e confirmar quando:** a otimização mudar a assinatura pública de um pacote (valor →
  ponteiro ou vice-versa) já consumido por outros módulos.
- **Risco alto:** trocar receiver de valor para ponteiro em tipo já usado como valor de interface —
  pode quebrar a satisfação implícita do contrato em tempo de compilação.
- **Risco baixo:** pré-alocar capacidade de slice/map interno ou evitar captura desnecessária em
  closure local, sem alterar assinaturas expostas.

## Métricas de sucesso

- Redução mensurável de `escapes to heap` no `-gcflags="-m"` entre antes e depois da mudança.
- Zero alocação nova introduzida dentro de loops quentes identificados como hot path.
- Receiver (ponteiro ou valor) consistente em 100% dos métodos de cada tipo revisado.
- Nenhuma mudança de comportamento de negócio detectada após a otimização (testes verdes).

## Responsável principal

| Papel | Quem |
|-------|------|
| Agent executor | `developer-golang-agent-orchestrator` |
| Revisão humana | Desenvolvedor / Tech Lead |
| Aprovação final | Tech Lead |

## Referências

- [Go GC Guide](https://go.dev/doc/gc-guide)
- [Go Compiler and Runtime — Escape Analysis](https://go.dev/doc/faq#stack_or_heap)
- Skill relacionada: `developer-go-performance-profiling`

---

## Versão interna (arquivo)

| Campo | Valor |
|-------|-------|
| **FileVersion** | 1.0.0 |
| **Política** | `.cursor/VERSION.md` |

## Changelog (este arquivo)

- 1.0.0 (09/08/2026): Criação da skill — cobertura inicial de escape analysis, comportamento do
  garbage collector concorrente tri-color e escolha entre value e pointer semantics para performance.

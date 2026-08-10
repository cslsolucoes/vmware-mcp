---
name: developer-go-testing
description: Testes automatizados em Go com o pacote testing nativo — table-driven tests, subtests com t.Run, cobertura e mocks via interfaces pequenas.
model: sonnet
thinking: normal
category: developer-go
license: MIT
copyright: "Copyright (c) 2026 CSL Tech Solutions"
company: "CSL Tech Solutions"
author: "Claiton de Souza Linhares"
---

# Testes em Go

## Responsabilidade única

Esta skill resolve o problema de escrever testes automatizados confiáveis e idiomáticos em Go usando exclusivamente o pacote `testing` da stdlib — sem depender de um framework externo de asserts ou de runner alternativo. Existe separada de `developer-go-language-core` porque ali o foco é a sintaxe e semântica da linguagem; aqui o foco é a metodologia de teste: como estruturar casos tabulares, isolar dependências via interfaces mínimas e medir cobertura de forma acionável. Cobre unit tests e testes de integração leves dentro do mesmo pacote sob teste; não cobre benchmarking de performance nem profiling.

## When to use

- Validar o comportamento público de uma função/método antes de considerá-lo pronto
- Cobrir múltiplas combinações de entrada/saída de uma função com poucos casos repetitivos → table-driven test
- Isolar uma dependência externa (banco, HTTP, filesystem) atrás de uma interface pequena para testar em memória
- Medir e reportar cobertura de um pacote ou módulo (`go test -cover`)

## When NOT to use

- Benchmarks de performance (`testing.B`, comparação de alocações/tempo) → usar `developer-go-performance-profiling`
- Testes de concorrência com goroutines/channels sob `-race` → skill de concorrência Go, fora deste escopo
- Fundamentos de sintaxe/tipos da linguagem → usar `developer-go-language-core`
- Reflection para serialização/validação dinâmica dentro dos próprios testes → usar `developer-go-stdlib-rtti-reflection`

## Inputs obrigatórios

| Input | Tipo | Descrição |
|-------|------|-----------|
| `module-path` | `string` | Caminho do módulo (`go.mod`) onde os testes serão escritos |
| `pacote-alvo` | `string` | Pacote cujo comportamento público será testado |
| `casos-de-teste` | `[]struct` | Lista de entradas e saídas esperadas (cenário, input, want, wantErr) |

## Dependências (skills prévias)

| Skill | Quando executar antes |
|-------|-----------------------|
| `developer-go-language-core` | Conhecer sintaxe de structs, slices, erros e funções antes de escrever tabelas de teste sobre elas |

## Workflow executável

1. Criar `<arquivo>_test.go` no mesmo pacote (`package foo`) ou em `package foo_test` para testar só a API pública.
2. Definir a tabela de casos como slice de struct anônima, com nome do cenário incluído:

```go
tests := []struct {
    name    string
    input   int
    want    int
    wantErr bool
}{
    {name: "positivo", input: 4, want: 16},
    {name: "zero", input: 0, want: 0},
}
```

3. Iterar a tabela com `t.Run(tt.name, func(t *testing.T) {...})` para isolar falhas por subteste:

```go
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        got, err := Square(tt.input)
        if (err != nil) != tt.wantErr {
            t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
        }
        if got != tt.want {
            t.Errorf("Square(%d) = %d, want %d", tt.input, got, tt.want)
        }
    })
}
```

4. Isolar dependências externas com uma interface mínima definida pelo consumidor, e implementar um dublê (fake) manual — sem framework de mock obrigatório:

```go
type Repository interface {
    FindByID(id int) (string, error)
}

type fakeRepo struct{ data map[int]string }

func (f fakeRepo) FindByID(id int) (string, error) { return f.data[id], nil }
```

5. Registrar limpeza determinística com `t.Cleanup(func() {...})` em vez de `defer` solto quando há setup compartilhado entre subtests.
6. Rodar `go test ./... -cover` e ler o percentual reportado por pacote antes de declarar a suíte concluída.

## Outputs obrigatórios

| Output | Localização | Formato |
|--------|-------------|---------|
| Arquivo de teste do pacote | `<pacote>/<arquivo>_test.go` | Go |
| Relatório de cobertura lido e interpretado | saída de `go test -cover` | texto/console |

## Checklist de validação

- [ ] Todo caso de teste tem nome descritivo (`name` na struct da tabela), não índice numérico
- [ ] Subtests usam `t.Run`, permitindo `go test -run <nome>` isolado
- [ ] Dependências externas isoladas via interface pequena definida pelo pacote consumidor
- [ ] `go test ./... -cover` executado e o percentual foi lido, não assumido

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

**Conflitos conhecidos:** nenhum conhecido — `testing` é stdlib pura; `testify` (`github.com/stretchr/testify`) é opcional para asserts mais expressivos.

## Checklist Go  ← OBRIGATÓRIO (Go)

- [ ] `gofmt -l .` sem saída (código formatado)
- [ ] `go vet ./...` sem avisos
- [ ] `go build ./...` sem erros
- [ ] `go test ./... -cover` verde com cobertura reportada
- [ ] Erros tratados explicitamente (`if err != nil`), nunca descartados com `_` sem justificativa
- [ ] Testes tabulares (`table-driven`) para funções com múltiplos casos de entrada

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
| Teste tabular percorrido com `for` sem `t.Run(tt.name, ...)` | Uma falha interrompe o loop e esconde os casos seguintes; impossível rodar um cenário isolado com `-run` | Sempre envolver cada iteração em `t.Run(tt.name, func(t *testing.T) {...})` |
| Ignorar `t.Cleanup`/`defer` para fechar recursos (arquivo, servidor de teste, conexão fake) | Vaza estado entre subtests e mascara falhas de um teste em outro que roda depois | Registrar `t.Cleanup(func() {...})` logo após criar o recurso, nunca ao final do teste |
| Testar detalhe de implementação (campo privado, ordem interna de chamadas) em vez de comportamento público observável | Quebra o teste a cada refactor que não muda o contrato, gerando manutenção sem valor | Testar apenas entrada/saída e efeitos observáveis pela API pública do pacote |
| Rodar `go test` sem `-cover` e declarar "coberto" por inspeção visual | Cobertura não medida é opinião, não fato — mascara caminhos de erro nunca exercitados | Sempre `go test ./... -cover` e ler o percentual por pacote antes de fechar a tarefa |

## Avaliação de risco

- **Parar e confirmar quando:** um teste existente precisa ser alterado/removido para "fazer passar" uma mudança de comportamento em código de produção já consumido por outros pacotes
- **Risco alto:** suíte "verde" que usa fake/mock cobrindo justamente o caminho que mascara o bug relatado pelo usuário (auditar adversarialmente antes de confiar)
- **Risco baixo:** adicionar um novo caso a uma tabela de teste já existente e validada

## Métricas de sucesso

- `go test ./... -cover` executado e lido (não assumido) a cada mudança relevante, com percentual reportado por pacote
- 100% dos cenários de erro esperados (`wantErr: true`) cobertos por pelo menos um caso na tabela
- Zero subtests sem `t.Run(tt.name, ...)` em tabelas com mais de um caso

## Responsável principal

| Papel | Quem |
|-------|------|
| Agent executor | `developer-golang-agent-orchestrator` |
| Revisão humana | Desenvolvedor / Tech Lead |
| Aprovação final | Tech Lead |

## Referências

- [pkg.go.dev/testing](https://pkg.go.dev/testing)
- Skill relacionada: `developer-go-error-handling-and-diagnostics`
- Skill relacionada: `developer-go-language-core`

---

## Versão interna (arquivo)

| Campo | Valor |
|-------|-------|
| **FileVersion** | 1.0.0 |
| **Política** | `.cursor/VERSION.md` |

## Changelog (este arquivo)

- 1.0.0 (09/08/2026): Criação da skill — testes em Go com pacote `testing` nativo (table-driven tests, `t.Run` subtests, cobertura via `go test -cover`, mocks por interfaces pequenas).

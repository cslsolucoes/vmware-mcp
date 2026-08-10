---
name: developer-go-language-generics
description: Generics em Go — type parameters, constraints (comparable, cmp.Ordered), funções e tipos genéricos, instanciação e critério generics vs interface simples.
model: sonnet
thinking: normal
category: developer-go
license: MIT
copyright: "Copyright (c) 2026 CSL Tech Solutions"
company: "CSL Tech Solutions"
author: "Claiton de Souza Linhares"
---

# Generics em Go

## Responsabilidade única

Esta skill cobre exclusivamente o mecanismo de generics do Go (type parameters, constraints,
instanciação): como declarar funções e tipos genéricos, expressar restrições sobre os tipos
aceitos e decidir quando generics reduzem duplicação sem sacrificar legibilidade. Ela não cobre
`interface{}`/`any` como substituto informal de tipagem (isso é `developer-go-language-types`)
nem introspecção via `reflect` (isso é `developer-go-stdlib-rtti-reflection`).

## When to use

- Criar funções utilitárias que repetem lógica idêntica para `int`, `float64`, `string` etc.
  (ex.: `Max`, `Sum`, `Filter`, `Map`, `Reduce`)
- Implementar estruturas de dados genéricas (`Stack[T]`, `Queue[T]`, `Set[T]`, árvore genérica)
- Escrever constraints customizadas com `~` (underlying type) para aceitar tipos definidos
- Substituir `interface{}`/`any` + type assertion por type parameters com verificação em
  tempo de compilação

## When NOT to use

- Quando `any`/`interface{}` simples já resolve sem necessidade de type-safety em compile-time
  → usar `developer-go-language-types`
- Quando o problema exige introspecção em runtime (percorrer campos, tags de struct) → usar
  `developer-go-stdlib-rtti-reflection`
- Quando uma única implementação concreta basta e generics só adicionam indireção sem reuso real
  (regra: não generalizar até haver ≥ 2 usos concretos)

## Inputs obrigatórios

| Input | Tipo | Descrição |
|-------|------|-----------|
| `assinatura_alvo` | `string` | Função/tipo que hoje está duplicado ou usa `any` a generalizar |
| `constraint_necessaria` | `string` | Operações exigidas sobre `T` (comparação, ordenação, aritmética) |
| `go_version` | `string` | Versão declarada em `go.mod` (mínimo `go 1.18`; recomendado `go 1.21+`) |

## Dependências (skills prévias)

| Skill | Quando executar antes |
|-------|-----------------------|
| `developer-go-language-core` | Sintaxe base, funções, structs — pré-requisito para ler type parameters |
| `developer-go-language-types` | Entender `any`/`interface{}` antes de decidir se generics são necessários |

## Workflow executável

1. Confirmar que existe duplicação real (≥ 2 tipos concretos) antes de generalizar
2. Declarar o type parameter entre colchetes: `func Max[T cmp.Ordered](a, b T) T`
3. Escolher a constraint mínima suficiente (`any`, `comparable`, `cmp.Ordered`, interface custom)
4. Para tipos genéricos, aplicar o parâmetro no receiver: `type Stack[T any] struct { items []T }`
5. Deixar a inferência de tipo atuar na chamada; só instanciar explicitamente quando ambíguo
6. Rodar `go build ./...` e `go vet ./...` para validar as constraints em tempo de compilação

```go
// Constraint customizada com union de tipos (~ = underlying type)
type Number interface {
    ~int | ~int64 | ~float64
}

func Sum[T Number](values []T) T {
    var total T
    for _, v := range values {
        total += v
    }
    return total
}
```

> Snippets ≤ 15 linhas. Exemplo de tipo genérico completo (`Stack[T]`, `Set[T]`) →
> `./exemplos/generic_types.go` (a criar sob demanda).

## Outputs obrigatórios

| Output | Localização | Formato |
|--------|-------------|---------|
| Função/tipo genérico | `<pacote>/*.go` | Go, compilável com `go build ./...` |
| Testes de instanciação | `<pacote>/*_test.go` | Go, cobrindo ao menos 2 instanciações concretas |

## Checklist de validação

- [ ] Existem ≥ 2 usos concretos que justificam a generalização (senão, reverter para tipo concreto)
- [ ] Constraint é a mais restrita possível (não usar `any` quando `comparable`/`cmp.Ordered` basta)
- [ ] Inferência de tipo funciona nas chamadas comuns (instanciação explícita só quando necessário)
- [ ] `go build ./...` e `go vet ./...` limpos com a nova função/tipo genérico

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

**Conflitos conhecidos:** generics exigem Go >= 1.18; `go.mod` deve declarar `go 1.18` ou superior.

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
|-------------|-----------------|---------------|
| Generalizar com generics após um único uso concreto | Indireção sem reuso real; dificulta leitura sem ganho | Esperar ≥ 2 usos concretos antes de extrair o type parameter |
| Usar `any` como constraint quando `comparable`/`cmp.Ordered` bastaria | Perde verificação em tempo de compilação, empurra erros para runtime | Escolher a constraint mais restrita que atenda ao conjunto de operações usadas |
| Instanciar tipo explicitamente (`Max[int](a, b)`) quando a inferência resolveria sozinha | Ruído sintático desnecessário | Deixar o compilador inferir; só explicitar em caso de ambiguidade real |
| Recriar `interface{}` + type assertion "escondido" atrás de generics | Mantém o mesmo risco de panic em runtime que generics deveriam eliminar | Expressar a constraint corretamente em vez de fazer assertion dentro do corpo genérico |

## Avaliação de risco

- **Parar e confirmar quando:** a generalização proposta afeta uma API pública já publicada
  (breaking change de assinatura) — seguir `version-breaking-change-guard` antes de aplicar
- **Risco alto:** constraint mal dimensionada aceita tipos que compilam mas se comportam de forma
  incorreta em runtime (ex.: `~int` aceitando tipo com semântica distinta de inteiro)
- **Risco baixo:** extrair função genérica local, sem uso externo ao pacote, coberta por testes

## Métricas de sucesso

- Zero duplicação de função por tipo concreto (uma função genérica substitui N variantes)
- 100% das instanciações usadas cobertas por teste (`go test ./...` verde)
- Nenhuma constraint mais permissiva que `any` quando uma constraint específica é suficiente

## Responsável principal

| Papel | Quem |
|-------|------|
| Agent executor | `developer-golang-agent-orchestrator` |
| Revisão humana | Desenvolvedor / Tech Lead |
| Aprovação final | Tech Lead |

## Referências

- [Go — Tutorial: Getting started with generics](https://go.dev/doc/tutorial/generics)
- [Go Language Specification — Type parameter declarations](https://go.dev/ref/spec#Type_parameter_declarations)
- Skill relacionada: `developer-go-master-orchestrator`

---

## Versão interna (arquivo)

| Campo | Valor |
|-------|-------|
| **FileVersion** | 1.0.0 |
| **Política** | `.cursor/VERSION.md` |

## Changelog (este arquivo)

- 1.0.0 (09/08/2026): Criação da skill — generics em Go (type parameters, constraints,
  funções/tipos genéricos, instanciação, critério generics vs interface simples), primeira
  skill da família `developer-go-*` espelhando `developer-delphi-to-fpc-language-generics`.

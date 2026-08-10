---
name: developer-go-language-oop
description: Orientação a objetos idiomática em Go — embedding de structs/interfaces, value vs pointer receiver e design orientado a interface, sem simular herança de classe.
model: sonnet
thinking: normal
category: developer-go
license: MIT
copyright: "Copyright (c) 2026 CSL Tech Solutions"
company: "CSL Tech Solutions"
author: "Claiton de Souza Linhares"
---

# Orientação a Objetos em Go

## Responsabilidade única

Esta skill resolve o problema de aplicar "orientação a objetos" em Go sem tentar
importar mentalmente o modelo de classes/herança de Delphi, Java ou C#. Go não tem
classes, não tem herança de implementação e não tem `override` — tem `struct`,
`interface`, `method` com receiver e **composição por embedding**. A skill existe
separada de `developer-go-patterns-composition` porque aqui o foco é o mecanismo de
linguagem (receivers, embedding, satisfação implícita de interface); padrões de
composição (Strategy, Decorator via embedding, etc.) são construídos **em cima**
deste mecanismo, não o contrário.

## When to use

- Definir métodos em um `type` próprio (`struct`, tipo nomeado) e decidir receiver por valor ou por ponteiro
- Reaproveitar campos/métodos de um tipo em outro via **embedding** de struct ou de interface
- Projetar uma `interface` pequena para desacoplar um pacote de uma implementação concreta
- Decidir se um identificador deve ser exportado (`Maiúscula`) ou fica interno ao pacote (`minúscula`)
- Substituir um design que "precisa de herança" por composição + interface

## When NOT to use

- Composição pura de dados sem métodos associados (agregação de structs simples) → usar `developer-go-patterns-composition`
- Definição de tipos primitivos, enums via `iota`, arrays/slices/maps → usar `developer-go-language-types`
- Concorrência entre métodos/goroutines de um tipo → usar skill de concorrência Go (fora deste escopo)
- Serialização de structs (JSON/DB) → skill de RTL/streams Go, não esta

## Inputs obrigatórios

| Input | Tipo | Descrição |
|-------|------|-----------|
| `module-path` | `string` | Caminho do módulo (`go.mod`) onde o tipo/interface será declarado |
| `tipo-alvo` | `string` | Nome do `struct`/`interface` a criar ou refatorar |
| `contrato` | `[]string` | Lista de métodos que o tipo precisa expor publicamente |

## Dependências (skills prévias)

| Skill | Quando executar antes |
|-------|-----------------------|
| `developer-go-language-types` | Definir os tipos base (struct fields, tipos nomeados) antes de anexar métodos |

## Workflow executável

1. Declarar o `struct` com campos privados (minúsculos) — expor só o necessário
2. Escolher receiver: ponteiro (`*T`) se o método muta estado ou o struct é grande; valor (`T`) se é pequeno e imutável
3. Definir `interface` mínima no pacote **consumidor**, não no pacote que implementa
4. Reaproveitar comportamento via **embedding** de struct ou interface, nunca herança simulada
5. Nomear métodos de acesso sem prefixo `Get` (`Owner()`, não `GetOwner()`)
6. Rodar `gofmt`, `go vet` e `go build` antes de considerar concluído

```go
type Animal struct {
    name string
}

func (a *Animal) Name() string { return a.name }

// Embedding: Dog "ganha" o método Name() de Animal automaticamente
type Dog struct {
    Animal
    breed string
}
```

## Outputs obrigatórios

| Output | Localização | Formato |
|--------|-------------|---------|
| Tipo/interface implementado | `<pacote>/<arquivo>.go` | Go |
| Testes de comportamento do método | `<pacote>/<arquivo>_test.go` | Go |

## Checklist de validação

- [ ] Nenhuma tentativa de simular herança de classe (sem campos `Base`/`Parent` como substituto de `extends`)
- [ ] Receiver consistente (mesmo tipo, valor ou ponteiro, em todos os métodos do tipo)
- [ ] Interface declarada no pacote consumidor, com o menor número de métodos possível
- [ ] Identificadores exportados só quando fazem parte do contrato público do pacote

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
| Simular herança de classe com embedding + campo `Base`/`Parent` imitando `extends` | Go não tem dispatch virtual; o método embedado não "vê" overrides do tipo externo, gerando bugs sutis de polimorfismo | Definir uma `interface` e injetar a implementação (composição explícita), não depender de embedding para polimorfismo |
| Getters/setters `GetX()`/`SetX()` desnecessários | Não é idiomático em Go; `golint`/revisores sinalizam; polui a API sem ganho | Expor o campo diretamente (se público) ou método `X()` sem prefixo `Get` |
| Receiver misto (alguns métodos `T`, outros `*T`) no mesmo tipo | Gera inconsistência de endereçabilidade e comportamento surpreendente entre cópias e ponteiros | Escolher um único tipo de receiver para todos os métodos do tipo e manter consistente |
| Interface grande declarada junto da implementação ("interface pollution") | Acopla consumidores a um contrato inchado e dificulta mocks/testes | Declarar interfaces pequenas no pacote consumidor (princípio "accept interfaces, return structs") |

## Avaliação de risco

- **Parar e confirmar quando:** a refatoração proposta trocaria a assinatura pública de um pacote já consumido por outros módulos
- **Risco alto:** misturar receiver por valor e por ponteiro no mesmo tipo em código já em produção (efeitos colaterais silenciosos)
- **Risco baixo:** adicionar um novo método com receiver consistente a um tipo já existente

## Métricas de sucesso

- Zero avisos de `go vet` e `golangci-lint` relacionados a receivers inconsistentes
- 100% dos identificadores exportados documentados com comentário iniciando pelo próprio nome
- Interfaces com no máximo 3-5 métodos (indicador indireto de baixo acoplamento)

## Responsável principal

| Papel | Quem |
|-------|------|
| Agent executor | `developer-golang-agent-orchestrator` |
| Revisão humana | Desenvolvedor / Tech Lead |
| Aprovação final | Tech Lead |

## Referências

- [Effective Go — Methods](https://go.dev/doc/effective_go#methods)
- [Effective Go — Embedding](https://go.dev/doc/effective_go#embedding)
- Skill relacionada: `developer-go-patterns-composition`
- Skill relacionada: `developer-go-language-types`

---

## Versão interna (arquivo)

| Campo | Valor |
|-------|-------|
| **FileVersion** | 1.0.0 |
| **Política** | `.cursor/VERSION.md` |

## Changelog (este arquivo)

- 1.0.0 (09/08/2026): Criação da skill — OOP idiomática em Go (embedding, receivers value/pointer, design por interface, ausência de herança de classe).

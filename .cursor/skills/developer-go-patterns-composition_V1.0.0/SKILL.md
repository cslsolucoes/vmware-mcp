---
name: developer-go-patterns-composition
description: Composição idiomática em Go — embedding de structs e interfaces, "accept interfaces, return structs" e interface segregation.
model: sonnet
thinking: normal
category: developer-go
license: MIT
copyright: "Copyright (c) 2026 CSL Tech Solutions"
company: "CSL Tech Solutions"
author: "Claiton de Souza Linhares"
---

# Composição em Go

## Responsabilidade única

Esta skill resolve o problema de estruturar comportamento reaproveitável entre
tipos Go **sem herança de classe** — mecanismo que a linguagem não possui. Ela
cobre exclusivamente a **composição estrutural**: embedding de structs (promoção
de campos/métodos), embedding de interfaces (composição de contratos maiores a
partir de contratos pequenos) e as convenções que resultam disso, como "accept
interfaces, return structs" e interface segregation. Existe separada de
`developer-go-language-oop` porque aquela trata do mecanismo básico (receivers,
satisfação implícita de interface); esta trata de como **combinar** múltiplos
tipos/contratos para formar estruturas maiores.

## When to use

- Reaproveitar campos e métodos de um tipo em outro via embedding de struct
  (`type Server struct { *log.Logger; ... }`) em vez de duplicar código.
- Compor uma interface maior a partir de interfaces menores já existentes
  (`type ReadWriter interface { Reader; Writer }`).
- Definir a assinatura de uma função/construtor que recebe uma interface
  pequena como parâmetro mas retorna um `struct` concreto ao chamador.
- Quebrar uma interface "gorda" em várias interfaces de 1-2 métodos, cada uma
  definida no pacote que efetivamente a consome.

## When NOT to use

- O problema pede polimorfismo real de tipo (dispatch dinâmico, múltiplas
  implementações trocáveis em runtime) → usar `developer-go-language-oop`.
- Criação controlada de valores (factories, options, singleton) → usar
  `developer-go-patterns-creational`.
- Definição de tipos base, structs e zero values pela primeira vez → usar
  `developer-go-language-core`.
- Necessidade de herança de implementação com override → não existe em Go;
  reavaliar o design com `developer-go-language-oop` antes de forçar composição.

## Inputs obrigatórios

| Input | Tipo | Descrição |
|-------|------|-----------|
| `module-path` | `string` | Caminho do módulo (`go.mod`) onde os tipos serão declarados |
| `tipo-base` | `string` | Tipo/struct a ser embutido (promovido) no tipo composto |
| `contratos` | `[]string` | Nomes das interfaces pequenas a compor ou consumir |

## Dependências (skills prévias)

| Skill | Quando executar antes |
|-------|-----------------------|
| `developer-go-language-oop` | Entender receivers, satisfação implícita de interface e exportação antes de compor tipos |

## Workflow executável

1. Identificar o comportamento comum e extraí-lo para um tipo pequeno e coeso
   (ex.: `Logger`, `Validator`), nunca um tipo "genérico" com muitas responsabilidades.
2. Embutir esse tipo (struct ou interface) como campo anônimo no tipo composto,
   promovendo seus métodos automaticamente.
3. Declarar a interface de consumo no pacote **que usa** a dependência, com o
   menor número de métodos possível (1-2), não no pacote que implementa.
4. Nas assinaturas de função, aceitar a interface pequena como parâmetro e
   retornar sempre o `struct` concreto (`func New(...) *Service`, não `Servicer`).
5. Validar que não há tentativa de simular `override`: se dois tipos embutidos
   expõem o mesmo método, resolver a ambiguidade explicitamente, sem depender
   de precedência implícita.

```go
// Passo 2 — embedding de struct promove o método Log()
type Service struct {
    *log.Logger
    repo Repository
}

// Passo 3 — interface pequena, declarada no consumidor
type Repository interface {
    Save(id string) error
}
```

## Outputs obrigatórios

| Output | Localização | Formato |
|--------|-------------|---------|
| Tipo composto com embedding | `<pacote>/<arquivo>.go` | Go |
| Interface(s) pequena(s) do consumidor | `<pacote>/<arquivo>.go` | Go |
| Testes do tipo composto | `<pacote>/<arquivo>_test.go` | Go |

## Checklist de validação

- [ ] Nenhum campo `Base`/`Parent` usado como substituto de herança de classe
- [ ] Cada interface declarada tem no máximo 2-3 métodos e vive no pacote consumidor
- [ ] Funções/construtores aceitam interface e retornam `struct` concreto
- [ ] Ambiguidade de métodos promovidos por múltiplos embeddings resolvida explicitamente

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
| Interface gigante declarada no pacote provedor ("interface pollution") | Acopla todo consumidor ao contrato inteiro, mesmo usando 1 método; dificulta mocks | Declarar interfaces pequenas (1-2 métodos) no pacote consumidor |
| Embedding usado para simular herança com `override` | Go resolve métodos promovidos estaticamente; não há dispatch virtual real, gerando bugs sutis | Usar interface + injeção explícita quando o objetivo é polimorfismo |
| Função que aceita `*struct` concreto em vez de interface pequena | Acopla o chamador à implementação, dificultando testes e substituição | Aceitar a interface mínima necessária; retornar o `struct` concreto |
| Dois embeddings promovendo o mesmo nome de método sem resolução explícita | Compilador exige desambiguação; ignorá-la no design gera acoplamento acidental | Nomear explicitamente qual embutido responde (`s.LoggerA.Log(...)`) ou remover a duplicidade |

## Avaliação de risco

- **Parar e confirmar quando:** a composição proposta alteraria a assinatura
  pública de um pacote já consumido por outros módulos (breaking change).
- **Risco alto:** embedding de dois tipos com métodos de mesmo nome sem
  desambiguação documentada — comportamento correto depende de detalhe do
  compilador, não de design explícito.
- **Risco baixo:** extrair uma interface pequena adicional para um consumidor
  novo, mantendo a implementação concreta inalterada.

## Métricas de sucesso

- 100% das interfaces do pacote consumidor com no máximo 2-3 métodos.
- Zero funções públicas retornando interface quando poderiam retornar `struct` concreto.
- `go vet ./...` e `gofmt -l .` limpos em todo pacote com embedding.

## Responsável principal

| Papel | Quem |
|-------|------|
| Agent executor | `developer-golang-agent-orchestrator` |
| Revisão humana | Desenvolvedor / Tech Lead |
| Aprovação final | Tech Lead |

## Referências

- [Effective Go — Embedding](https://go.dev/doc/effective_go#embedding)
- Skill relacionada: `developer-go-language-oop`

---

## Versão interna (arquivo)

| Campo | Valor |
|-------|-------|
| **FileVersion** | 1.0.0 |
| **Política** | `.cursor/VERSION.md` |

## Changelog (este arquivo)

- 1.0.0 (09/08/2026): Criação da skill — composição idiomática em Go (embedding de structs/interfaces, "accept interfaces, return structs", interface segregation).

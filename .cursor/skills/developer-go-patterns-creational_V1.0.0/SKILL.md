---
name: developer-go-patterns-creational
description: Padrões criacionais idiomáticos em Go — Factory Function, Functional Options e Singleton via sync.Once.
model: sonnet
thinking: normal
category: developer-go
license: MIT
copyright: "Copyright (c) 2026 CSL Tech Solutions"
company: "CSL Tech Solutions"
author: "Claiton de Souza Linhares"
---

# Padrões Criacionais em Go

## Responsabilidade única

Esta skill cobre a criação controlada de valores e structs em Go sem recorrer a
herança de classes nem a construtores sobrecarregados (Go não tem nenhum dos dois).
Ela resolve o problema de inicializar tipos com estado válido, extensível e
thread-safe, usando apenas os idiomas nativos da linguagem: funções `New*`,
Functional Options e `sync.Once`. Existe separada de
`developer-go-patterns-structural` e `developer-go-patterns-composition` porque
trata exclusivamente do **momento e forma de criação** do objeto, não da sua
estrutura interna nem de como objetos se compõem depois de criados.

## When to use

- Expor um tipo não exportado (`type client struct{}`) através de um construtor
  exportado (`func NewClient(...) *Client`) que garante estado inicial válido.
- Construir um tipo com muitos parâmetros opcionais sem gerar explosão de
  overloads (Go não suporta overload) — usar Functional Options.
- Garantir que um recurso caro (pool de conexões, client HTTP compartilhado,
  cache global) seja inicializado exatamente uma vez, mesmo sob concorrência.
- Validar invariantes de criação (campos obrigatórios, ranges, formatos) antes
  de devolver a struct pronta para uso ao chamador.

## When NOT to use

- Composição simples de structs embutidos (`type B struct { A }`) → usar
  `developer-go-patterns-composition`.
- Definição de interfaces e contratos entre pacotes → usar
  `developer-go-patterns-structural`.
- Padrões de comportamento em runtime (Strategy, Observer, State) → usar
  `developer-go-patterns-behavioral`.
- Criação trivial de struct literal sem invariantes (`Point{X: 1, Y: 2}`) —
  overhead de factory é desnecessário.

## Inputs obrigatórios

| Input | Tipo | Descrição |
|-------|------|-----------|
| `module-path` | `string` | Caminho do módulo Go (`go.mod`) onde o pacote criacional será adicionado |
| `tipo-alvo` | `string` | Nome do tipo não exportado a ser encapsulado (ex.: `client`, `pool`) |
| `campos-obrigatorios` | `[]string` | Campos que devem ser validados na factory antes de retornar o valor |
| `campos-opcionais` | `[]string` | Campos que viram Functional Options (`WithX`) |

## Dependências (skills prévias)

| Skill | Quando executar antes |
|-------|-----------------------|
| `developer-go-language-types` | Definir corretamente structs, ponteiros e zero values antes de encapsulá-los em factories |

## Workflow executável

1. Definir o tipo não exportado com campos privados e zero value inválido/incompleto.
2. Criar `func New<Tipo>(obrigatórios..., opts ...Option) (*Tipo, error)` aplicando
   as options em ordem antes de validar invariantes.
3. Modelar cada campo opcional como `type Option func(*Tipo)` + `func WithX(v T) Option`.
4. Para recurso único de processo, expor `GetInstance()` com `sync.Once` em vez de
   `init()` global (permite lazy init e testes isolados).
5. Retornar sempre erro explícito de validação — nunca `panic` em código de biblioteca.

```go
// Passo 2+3 — Factory + Functional Options (ver workflow completo em ./exemplos/)
type Option func(*Client)

func WithTimeout(d time.Duration) Option {
    return func(c *Client) { c.timeout = d }
}

func NewClient(addr string, opts ...Option) (*Client, error) {
    if addr == "" {
        return nil, errors.New("addr is required")
    }
    c := &Client{addr: addr, timeout: 5 * time.Second}
    for _, opt := range opts {
        opt(c)
    }
    return c, nil
}
```

## Outputs obrigatórios

| Output | Localização | Formato |
|--------|-------------|---------|
| Factory function + Options | `<pacote>/<tipo>.go` | Go |
| Singleton (quando aplicável) | `<pacote>/singleton.go` | Go |
| Testes da factory | `<pacote>/<tipo>_test.go` | Go (`testing`) |

## Checklist de validação

- [ ] Tipo criado é não exportado; só a factory e os métodos públicos são exportados
- [ ] Factory retorna `error` para toda condição de estado inválido (nunca `panic`)
- [ ] Cada campo opcional tem uma `Option` própria (`With<Campo>`), sem parâmetros posicionais extras
- [ ] Singleton (se existir) usa `sync.Once`, nunca double-checked locking manual com mutex cru

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

## Anti-padrões

| Anti-padrão | Por que é errado | Como corrigir |
|-------------|-----------------|---------------|
| Struct exportada com todos os campos públicos e sem factory | Permite estado inválido/incompleto ser criado por composite literal externo | Tornar o tipo não exportado e obrigar uso de `New<Tipo>` |
| Função `New` com 6+ parâmetros posicionais booleanos/opcionais | Ilegível na chamada, ordem propensa a erro, sem valores padrão | Migrar para Functional Options (`opts ...Option`) |
| Singleton com variável global inicializada em `var x = New()` no topo do arquivo | Roda no `init()` do pacote, sem controle de erro nem lazy init, dificulta testes | Usar `sync.Once` dentro de `GetInstance()` com retorno de erro |
| `panic()` dentro da factory para sinalizar parâmetro inválido | Interrompe o processo do chamador; inaceitável em código de biblioteca | Retornar `(*Tipo, error)` e deixar o chamador decidir |

## Avaliação de risco

- **Parar e confirmar quando:** a mudança de assinatura de uma factory já
  exportada e consumida por outros módulos (breaking change de API pública).
- **Risco alto:** Singleton compartilhando estado mutável entre goroutines sem
  proteção adicional além da inicialização (`sync.Once` só protege a criação,
  não o uso concorrente subsequente).
- **Risco baixo:** adicionar uma nova `Option` a uma factory já existente
  (extensão aditiva e retrocompatível).

## Métricas de sucesso

- 100% das factories retornam `error` em vez de `panic` para entrada inválida.
- Zero campos exportados em tipos que possuem factory dedicada (encapsulamento real).
- `go vet ./...` e `gofmt -l .` limpos em todo pacote criacional.

## Responsável principal

| Papel | Quem |
|-------|------|
| Agent executor | `developer-golang-agent-orchestrator` |
| Revisão humana | Desenvolvedor / Tech Lead |
| Aprovação final | Tech Lead |

## Referências

- [Effective Go](https://go.dev/doc/effective_go)
- Skill relacionada: `developer-go-patterns-structural`

---

## Versão interna (arquivo)

| Campo | Valor |
|-------|-------|
| **FileVersion** | 1.0.0 |
| **Política** | `.cursor/VERSION.md` |

## Changelog (este arquivo)

- 1.0.0 (09/08/2026): Criação da skill — padrões criacionais idiomáticos em Go (Factory Function, Functional Options, Singleton via `sync.Once`).

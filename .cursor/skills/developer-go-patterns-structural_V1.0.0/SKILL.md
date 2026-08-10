---
name: developer-go-patterns-structural
description: Padrões estruturais em Go — Adapter, Decorator e Facade via composição de interfaces, sem herança.
model: sonnet
thinking: normal
category: developer-go
license: MIT
copyright: "Copyright (c) 2026 CSL Tech Solutions"
company: "CSL Tech Solutions"
author: "Claiton de Souza Linhares"
---

# Padrões Estruturais em Go

## Responsabilidade única

Esta skill cobre a implementação idiomática dos padrões estruturais mais usados em Go —
Adapter, Decorator e Facade — todos resolvidos por composição de `interface{}` e wrapping de
tipos, nunca por herança (Go não tem herança de classes). Ela existe separada dos padrões
criacionais e comportamentais porque o problema aqui é sempre "como recompor/adequar/simplificar
a forma de um tipo ou subsistema já existente" sem alterar seu código-fonte original. É a
referência para adaptar bibliotecas de terceiros, encadear middlewares HTTP e esconder a
complexidade de um subsistema atrás de uma interface enxuta.

## When to use

- Adequar o tipo de uma biblioteca de terceiros a uma interface própria do projeto (Adapter)
- Adicionar comportamento cross-cutting (log, auth, métricas) sem alterar o handler original (Decorator)
- Encadear múltiplos `http.Handler`/middlewares em pipeline (Decorator)
- Simplificar um subsistema com várias dependências atrás de uma única interface (Facade)
- Expor uma API pública de pacote que esconde detalhes internos de implementação (Facade)

## When NOT to use

- Criação de objetos/famílias de objetos → usar `developer-go-patterns-creational`
- Definição de fluxo de controle, estratégia ou observação de eventos → usar `developer-go-patterns-behavioral`
- Definição de tipos base, structs e generics do zero → usar `developer-go-language-types`
- Composição de comportamento via `struct` embedding simples sem interface pública → fora do escopo (é Go idiomático puro, não é pattern)

## Inputs obrigatórios

| Input | Tipo | Descrição |
|-------|------|-----------|
| `interfaceAlvo` | `interface{}` (contrato Go) | Interface que o código cliente já espera consumir |
| `tipoExistente` | `struct`/pacote externo | Tipo de terceiros ou legado a ser adaptado/decorado/escondido |
| `subsistema` | lista de dependências | Componentes que o Facade deve orquestrar internamente |

## Dependências (skills prévias)

| Skill | Quando executar antes |
|-------|-----------------------|
| `developer-go-language-types` | Definir interfaces, structs e embedding corretamente antes de compor padrões |

## Workflow executável

1. Identificar o contrato (`interface`) que o código cliente já consome ou deveria consumir.
2. Para Adapter: criar `struct` wrapper que implementa a interface alvo e delega ao tipo de terceiros.
3. Para Decorator: criar função `func(next T) T` que retorna um novo `T` envolvendo o original.
4. Para Facade: criar `struct` não exportado com as dependências do subsistema e expor só os
   métodos da interface pública.
5. Rodar `go vet ./...` e `gofmt -l .` antes de commitar.

```go
// Adapter — adequa ThirdPartyLogger (terceiros) à interface Logger do projeto
type Logger interface{ Log(msg string) }

type ThirdPartyLogger struct{}

func (t *ThirdPartyLogger) Write(level int, msg string) {}

type LoggerAdapter struct{ inner *ThirdPartyLogger }

func (a *LoggerAdapter) Log(msg string) { a.inner.Write(0, msg) }
```

```go
// Decorator — encadeamento de http.Handler
func WithLogging(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        log.Printf("%s %s", r.Method, r.URL.Path)
        next.ServeHTTP(w, r)
    })
}

// uso: mux = WithLogging(WithAuth(mux))
```

## Outputs obrigatórios

| Output | Localização | Formato |
|--------|-------------|---------|
| Código do padrão aplicado | `<pacote>/*.go` | Go (gofmt aplicado) |
| Testes do padrão | `<pacote>/*_test.go` | Go (`testing`) |

## Checklist de validação

- [ ] Interface alvo definida antes da implementação do Adapter/Decorator/Facade
- [ ] Nenhuma herança simulada via embedding indevido — composição explícita
- [ ] Decorator preserva a assinatura da interface original (`http.Handler`, etc.)
- [ ] Facade não vaza tipos internos do subsistema na sua interface pública

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
|-------------|-----------------|---------------|
| Adapter que expõe métodos do tipo adaptado além da interface alvo | Quebra o encapsulamento e acopla o cliente ao tipo de terceiros | Expor só os métodos da `interface` alvo, delegação interna privada |
| Decorator que muda a assinatura do `next` recebido | Quebra o encadeamento e obriga o cliente a conhecer a ordem interna | Manter sempre `func(next T) T` com a mesma interface `T` |
| Facade que retorna `struct` concreto em vez de `interface` | Impede substituição/mock em teste e vaza detalhes internos | Facade sempre retorna/implementa uma `interface` exportada |
| Simular herança com embedding só para reaproveitar método sem checar contrato | Gera acoplamento oculto e sobrescrita silenciosa de métodos | Preferir composição explícita com campo nomeado e delegação clara |

## Avaliação de risco

- **Parar e confirmar quando:** o Adapter precisa expor comportamento do tipo de terceiros que
  não existe na interface alvo do projeto — decisão de ampliar contrato é do Tech Lead.
- **Risco alto:** Facade que esconde falhas de subsistemas críticos (pagamento, autenticação)
  sem propagar erro tipado ao cliente.
- **Risco baixo:** Decorator adicional em pipeline de middlewares já testado e com testes de
  regressão cobrindo a ordem de encadeamento.

## Métricas de sucesso

- Zero avisos em `go vet ./...` e `gofmt -l .` após aplicar o padrão
- 100% dos métodos exportados do Adapter/Facade cobertos por teste unitário
- Nenhum tipo interno do subsistema vazado na assinatura pública do Facade

## Responsável principal

| Papel | Quem |
|-------|------|
| Agent executor | `developer-golang-agent-orchestrator` |
| Revisão humana | Desenvolvedor / Tech Lead |
| Aprovação final | Tech Lead |

## Referências

- [Effective Go — Interfaces and other types](https://go.dev/doc/effective_go#interfaces_and_types)
- Skill relacionada: `developer-go-patterns-composition`

---

## Versão interna (arquivo)

| Campo | Valor |
|-------|-------|
| **FileVersion** | 1.0.0 |
| **Política** | `.cursor/VERSION.md` |

## Changelog (este arquivo)

- 1.0.0 (09/08/2026): Criação da skill — padrões estruturais Adapter, Decorator e Facade em Go
  via composição de interfaces, espelhando o formato de `developer-delphi-to-fpc-patterns-structural`.

---
name: developer-go-http-server
description: Servidor HTTP em Go usando apenas net/http da stdlib — ServeMux com roteamento por método+path, middlewares genéricos e graceful shutdown.
model: sonnet
thinking: normal
category: developer-go
license: MIT
copyright: "Copyright (c) 2026 CSL Tech Solutions"
company: "CSL Tech Solutions"
author: "Claiton de Souza Linhares"
---

# Servidor HTTP em Go

## Responsabilidade única

Esta skill cobre a construção de um servidor HTTP em Go usando **exclusivamente** o pacote
`net/http` da stdlib: `http.ServeMux` com padrões de roteamento método+path (`"GET /users/{id}"`,
disponíveis desde Go 1.22), composição de middlewares genéricos (`func(http.Handler) http.Handler`)
e desligamento gracioso via `server.Shutdown(ctx)` combinado com `os/signal`. Ela existe separada de
qualquer framework de terceiros (gin, echo, fiber, Horse) porque o objetivo é dominar o modelo nativo
do Go antes — ou em vez — de adotar dependências externas; a decisão de usar um framework é sempre
explícita e documentada, nunca o padrão.

## When to use

- Expor endpoints HTTP/REST usando somente a biblioteca padrão do Go
- Definir rotas por método+path com `http.ServeMux` (Go 1.22+) sem router de terceiros
- Compor middlewares reutilizáveis (log, recover, auth) em cadeia sobre `http.Handler`
- Configurar `http.Server` com timeouts explícitos e desligamento gracioso em produção
- Extrair parâmetros de rota (`r.PathValue("id")`) sem dependência externa

## When NOT to use

- Consumir/chamar uma API HTTP externa (cliente, não servidor) → `developer-go-http-client-rest`
- Necessidade de um framework de terceiros (gin/echo/fiber/Horse) → fora do escopo desta skill,
  que cobre apenas `net/http` da stdlib; adoção de framework exige justificativa explícita à parte
- Serialização/deserialização JSON do corpo da requisição/resposta em detalhe → `developer-go-stdlib-encoding`
- Cancelamento avançado, `context` com valores por requisição ou pools de workers dentro do handler
  → `developer-go-concurrency-advanced`
- Sintaxe básica de funções, closures e `go.mod` → `developer-go-language-core`

## Inputs obrigatórios

| Input | Tipo | Descrição |
|-------|------|-----------|
| `endereço de escuta` | `string` | Host:porta do servidor (ex.: `:8080`) |
| `tabela de rotas` | método+path | Conjunto de handlers a registrar no `ServeMux` |
| `módulo Go válido` | `go.mod` | Projeto com `go 1.22` ou superior declarado |

## Dependências (skills prévias)

| Skill | Quando executar antes |
|-------|-----------------------|
| `developer-go-concurrency-basics` | Servir em goroutine própria e sincronizar o shutdown exigem goroutines/channels |
| `developer-go-stdlib-encoding` | Handlers que leem/escrevem JSON no corpo precisam de `encoding/json` |

## Workflow executável

1. Criar o roteador com `mux := http.NewServeMux()`
2. Registrar rotas com padrão método+path (Go 1.22+): `mux.HandleFunc("GET /users/{id}", h)`,
   extraindo parâmetros com `r.PathValue("id")`
3. Compor middlewares genéricos (`func(http.Handler) http.Handler`), encadeando do mais externo
   (log/recover) para o mais interno (auth/negócio)
4. Configurar `http.Server` explicitamente — nunca usar `http.ListenAndServe` cru em produção
5. Subir o servidor em goroutine própria e aguardar sinal de encerramento do SO
6. Ao receber `SIGINT`/`SIGTERM`, chamar `server.Shutdown(ctx)` com timeout para drenar conexões

```go
srv := &http.Server{Addr: ":8080", Handler: mux,
    ReadTimeout: 5 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 120 * time.Second}
go func() {
    if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
        log.Fatalf("listen: %v", err)
    }
}()
<-ctx.Done() // cancelado por os/signal.NotifyContext(SIGINT, SIGTERM)
shCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()
srv.Shutdown(shCtx)
```

> Snippets de código inline ≤ 15 linhas. Blocos maiores → mover para `./exemplos/` e referenciar:
> `→ Ver [exemplo completo](./exemplos/nome.go)`

## Outputs obrigatórios

| Output | Localização | Formato |
|--------|-------------|---------|
| Servidor `http.Server` configurado (timeouts + handler) | `*.go` do pacote alvo (`main.go`/`server.go`) | Go |
| Rotas registradas via `http.ServeMux` (método+path) | mesmo pacote | Go |
| Cadeia de middlewares composta | mesmo pacote | Go |
| Evidência de desligamento gracioso | saída de log do processo ao receber sinal | Log de terminal |

## Checklist de validação

- [ ] Rotas usam padrão método+path do Go 1.22+ (`"GET /path/{id}"`), não `switch r.Method` manual
- [ ] Todo middleware chama `next.ServeHTTP(w, r)` em todos os caminhos de código (exceto quando
  intencionalmente interrompe a cadeia, ex.: 401 não autenticado)
- [ ] `http.Server` tem `ReadTimeout`, `WriteTimeout` e `IdleTimeout` definidos explicitamente
- [ ] Desligamento gracioso implementado (`Shutdown(ctx)` + `os/signal`), não `os.Exit` abrupto
- [ ] `http.ErrServerClosed` tratado como retorno esperado do `Shutdown`, não como erro fatal

---

## Stack e versões  ← OBRIGATÓRIO (Go)

| Componente | Versão mínima | Notas |
|------------|:---:|-------|
| Go | 1.22 | `http.ServeMux` com padrões de método+path (`"GET /users/{id}"`) requer 1.22+ |
| gofmt | embutido | Formatação obrigatória, sem exceções |
| go vet | embutido | Rodar antes de qualquer commit |
| golangci-lint | 1.55+ | Lint agregador (opcional mas recomendado) |

## Dependências (go.mod / go get)  ← OBRIGATÓRIO (Go)

```bash
go mod init <module-path>
go get <pacote>@<versão>
go mod tidy
```

**Conflitos conhecidos:** `net/http` (stdlib) é suficiente — usar framework de terceiros só com justificativa explícita documentada.

## Checklist Go  ← OBRIGATÓRIO (Go)

- [ ] `gofmt -l .` sem saída (código formatado)
- [ ] `go vet ./...` sem avisos
- [ ] `go build ./...` sem erros
- [ ] `go test ./...` verde (quando aplicável ao tema da skill)
- [ ] Erros tratados explicitamente (`if err != nil`), nunca descartados com `_` sem justificativa
- [ ] `http.Server` configurado com `ReadTimeout`/`WriteTimeout`/`IdleTimeout` e graceful shutdown via `context`

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
| `http.ListenAndServe(":8080", mux)` direto, sem `http.Server` explícito | Sem `ReadTimeout`/`WriteTimeout`/`IdleTimeout`, o servidor fica exposto a slowloris e conexões penduradas indefinidamente | Instanciar `&http.Server{...}` com timeouts explícitos e usar `srv.ListenAndServe()` |
| Handler global (`var globalCtx context.Context`) em vez de derivar `context` de `r.Context()` | `r.Context()` já carrega cancelamento por conexão/timeout da requisição; ignorá-lo perde propagação de cancelamento e vaza estado entre requisições concorrentes | Sempre usar `ctx := r.Context()` dentro do handler; nunca guardar `context` em variável de pacote |
| Middleware que não chama `next.ServeHTTP` em algum caminho de retorno (esquecido em um `if`) | A requisição trava sem resposta (handler seguinte nunca executa) até o `WriteTimeout` do servidor cortar a conexão | Garantir que todo caminho do middleware termine explicitamente em `next.ServeHTTP(w, r)` ou em uma resposta de erro escrita diretamente |
| Tratar `http.ErrServerClosed` retornado por `ListenAndServe()` após `Shutdown` como erro fatal (`log.Fatal`) | É o retorno **esperado** quando o servidor foi desligado de propósito; tratá-lo como fatal gera falso alarme em todo shutdown normal | Comparar explicitamente: `if err != nil && err != http.ErrServerClosed { log.Fatalf(...) }` |
| Registrar rotas sem verbo (`mux.HandleFunc("/users", h)`) despachando por `switch r.Method` manual dentro do handler | Duplica lógica de roteamento que o `ServeMux` do Go 1.22+ já resolve nativamente, e é fácil esquecer um método (ex.: `OPTIONS`) | Usar o padrão método+path do `ServeMux` (`"GET /users"`, `"POST /users"`) |

## Avaliação de risco

- **Parar e confirmar quando:** o pedido do usuário sugere adotar um framework de terceiros
  (gin/echo/fiber/Horse) — essa é uma decisão arquitetural fora do escopo desta skill e deve ser
  confirmada explicitamente antes de prosseguir
- **Risco alto:** servidor em produção sem timeouts (`ReadTimeout`/`WriteTimeout`/`IdleTimeout`)
  ou sem desligamento gracioso — conexões penduradas e perda de requisições em deploy/restart
- **Risco baixo:** ajuste de rota isolada, adição de middleware que já segue o padrão
  `func(http.Handler) http.Handler` testado

## Métricas de sucesso

- Zero avisos do `go vet` e zero warnings do `gofmt -l .`
- 100% das rotas registradas via padrão método+path do `ServeMux` (Go 1.22+), sem `switch r.Method` manual
- Desligamento gracioso demonstrável: `SIGINT`/`SIGTERM` resulta em `Shutdown(ctx)` sem conexões abortadas
- Checklist de validação 100% marcado antes de considerar a tarefa concluída

## Responsável principal

| Papel | Quem |
|-------|------|
| Agent executor | `developer-golang-agent-orchestrator` |
| Revisão humana | Desenvolvedor / Tech Lead |
| Aprovação final | Tech Lead |

## Referências

- [pkg.go.dev — net/http](https://pkg.go.dev/net/http)
- Skill relacionada: `developer-go-http-client-rest`

---

## Versão interna (arquivo)

| Campo | Valor |
|-------|-------|
| **FileVersion** | 1.0.0 |
| **Política** | `.cursor/VERSION.md` |

## Changelog (este arquivo)

- 1.0.0 (09/08/2026): Criação da skill — servidor HTTP em Go com `net/http` da stdlib (`ServeMux`
  método+path, middlewares genéricos, graceful shutdown), seguindo o template V2.0 com as 17
  seções canônicas.

---
name: developer-go-database-access
description: "Acesso a banco de dados em Go via database/sql da stdlib — driver-agnostic, connection pooling, prepared statements e queries parametrizadas com contexto."
model: sonnet
thinking: normal
category: developer-go
license: MIT
copyright: "Copyright (c) 2026 CSL Tech Solutions"
company: "CSL Tech Solutions"
author: "Claiton de Souza Linhares"
---

# Acesso a Banco de Dados em Go

## Responsabilidade única

Esta skill cobre o acesso a banco de dados relacional em Go usando exclusivamente o
pacote `database/sql` da biblioteca padrão: abertura e configuração de `*sql.DB` como
pool de conexões, execução de queries e comandos com `QueryContext`/`ExecContext`,
prepared statements e o fechamento correto de `*sql.Rows`/`*sql.Stmt`. Ela existe
separada das demais skills da família `developer-go-*` porque `database/sql` tem
disciplina própria — a interface é stdlib, mas o driver concreto (`driver.Driver`) é
sempre uma dependência externa registrada por efeito colateral (`_ "import"`). Esta
skill é genérica: não aborda nenhum ORM ou query builder de terceiros (gorm, sqlx, ent).

## When to use

- Abrir/configurar um `*sql.DB` (pool) para um banco relacional (PostgreSQL, MySQL,
  SQL Server, SQLite, etc.) usando apenas `database/sql`
- Escrever queries (`SELECT`) ou comandos (`INSERT`/`UPDATE`/`DELETE`) parametrizados
  com `QueryContext`/`ExecContext`/`QueryRowContext`
- Reutilizar prepared statements (`PrepareContext`) em execuções repetidas
- Ajustar limites do pool (`SetMaxOpenConns`, `SetMaxIdleConns`, `SetConnMaxLifetime`)
- Garantir fechamento correto de `*sql.Rows`/`*sql.Stmt` e checagem de `rows.Err()`

## When NOT to use

- Serialização/deserialização do payload lido do banco (JSON, structs) → `developer-go-stdlib-encoding`
- Uso de ORM ou query builder de terceiros (gorm, sqlx, ent, squirrel) — fora do escopo,
  esta skill cobre somente `database/sql` puro
- Tratamento genérico de erros e wrapping com `%w` → `developer-go-error-handling-and-diagnostics`
- Concorrência entre goroutines que compartilham o mesmo `*sql.DB` (worker pools, `errgroup`) → `developer-go-concurrency-basics`
- Build, empacotamento e distribuição do binário → `developer-go-build-toolchain`

## Inputs obrigatórios

| Input | Tipo | Descrição |
|-------|------|-----------|
| DSN (connection string) | string | Endereço/credenciais do banco alvo, nunca hardcoded — via env var |
| Driver SQL registrado | import `_ "driver"` | Driver concreto compatível com `database/sql` (ex.: `lib/pq`, `go-sql-driver/mysql`) |
| Query/comando SQL | string com placeholders | `?` (MySQL/SQLite) ou `$1,$2,...` (PostgreSQL) — nunca concatenação |
| `context.Context` | `context.Context` | Propagado em toda chamada `*Context` para cancelamento/timeout |

## Dependências (skills prévias)

| Skill | Quando executar antes |
|-------|-----------------------|
| `developer-go-error-handling-and-diagnostics` | Antes de propagar erros de `sql.ErrNoRows` e afins — pressupõe wrapping com `%w` e `errors.Is` |

## Workflow executável

1. Abrir o pool uma única vez no início do processo (nunca por request) e validar com `Ping`:

```go
db, err := sql.Open("postgres", dsn)
if err != nil {
    return fmt.Errorf("abrir pool: %w", err)
}
if err := db.PingContext(ctx); err != nil {
    return fmt.Errorf("ping: %w", err)
}
```

2. Configurar os limites do pool logo após `Open` (evita esgotar conexões do servidor):

```go
db.SetMaxOpenConns(25)
db.SetMaxIdleConns(25)
db.SetConnMaxLifetime(5 * time.Minute)
```

3. Executar leitura de múltiplas linhas com `QueryContext`, sempre parametrizada e com
   `defer rows.Close()` imediatamente após checar o erro:

```go
rows, err := db.QueryContext(ctx, "SELECT id, name FROM users WHERE active = $1", true)
if err != nil {
    return fmt.Errorf("query users: %w", err)
}
defer rows.Close()
for rows.Next() {
    var id int
    var name string
    if err := rows.Scan(&id, &name); err != nil {
        return fmt.Errorf("scan user: %w", err)
    }
}
if err := rows.Err(); err != nil {
    return fmt.Errorf("iterate rows: %w", err)
}
```

4. Executar leitura de linha única com `QueryRowContext` (não requer `Close` explícito):

```go
var name string
err := db.QueryRowContext(ctx, "SELECT name FROM users WHERE id = $1", id).Scan(&name)
if errors.Is(err, sql.ErrNoRows) {
    // tratar ausência
}
```

5. Executar comandos de escrita com `ExecContext`, inspecionando `RowsAffected`:

```go
res, err := db.ExecContext(ctx, "UPDATE users SET active = $1 WHERE id = $2", false, id)
if err != nil {
    return fmt.Errorf("update user: %w", err)
}
n, _ := res.RowsAffected()
```

6. Preparar statements reutilizados em loop/hot path com `PrepareContext` e fechar com `defer stmt.Close()`.
7. Encapsular sequências multi-comando em transação (`db.BeginTx`, `tx.Commit`/`tx.Rollback` via `defer`).

> Snippets acima ≤ 15 linhas cada. Exemplo combinado maior → `./exemplos/database.go`.

## Outputs obrigatórios

| Output | Localização | Formato |
|--------|-------------|---------|
| Pool `*sql.DB` configurado (limites de conexão) | camada de infraestrutura (ex.: `./internal/db/db.go`) | Go |
| Funções de acesso a dados parametrizadas | pacote de repositório correspondente | Go |
| Erros de banco propagados com wrapping (`%w`) | funções que retornam `error` no caminho de chamada | Go |
| `rows`/`stmt` fechados em todo caminho de retorno | via `defer` logo após checagem de erro | Go |

## Checklist de validação

- [ ] `*sql.DB` aberto uma única vez por processo e reutilizado como pool (nunca recriado por request)
- [ ] Toda query/comando usa placeholders parametrizados, nunca `fmt.Sprintf`/concatenação de SQL
- [ ] Todo `*sql.Rows` tem `defer rows.Close()` e `rows.Err()` checado após o loop `Next()`
- [ ] Toda chamada usa a variante `*Context` (`QueryContext`/`ExecContext`/`QueryRowContext`)
- [ ] `sql.ErrNoRows` tratado explicitamente com `errors.Is`, nunca comparação de string

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
go get <driver-sql-do-banco>@<versão>
go mod tidy
```

**Conflitos conhecidos:** `database/sql` exige um driver externo registrado via `_ "import"` (ex.: `github.com/lib/pq`, `github.com/go-sql-driver/mysql`) — a interface é stdlib, o driver não.

## Checklist Go  ← OBRIGATÓRIO (Go)

- [ ] `gofmt -l .` sem saída (código formatado)
- [ ] `go vet ./...` sem avisos
- [ ] `go build ./...` sem erros
- [ ] `go test ./...` verde (quando aplicável ao tema da skill)
- [ ] Erros tratados explicitamente (`if err != nil`), nunca descartados com `_` sem justificativa
- [ ] Toda query usa parâmetros (`?`/`$1`), nunca concatenação de string (SQL injection)

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
| Montar SQL com `fmt.Sprintf`/concatenação de string | Abre brecha para SQL injection | Usar sempre placeholders (`?`/`$1`) e passar os valores como argumentos da query |
| Esquecer `rows.Close()` | Vaza conexões do pool; sob carga esgota `SetMaxOpenConns` e trava o serviço | `defer rows.Close()` imediatamente após checar o erro de `QueryContext` |
| Criar um novo `*sql.DB` por request/handler | `sql.Open` não abre conexão de fato, mas recriar o pool a cada chamada perde reuso e gera overhead/leak de recursos | Abrir `*sql.DB` uma vez no bootstrap e injetar/reutilizar como singleton |
| Ignorar `rows.Err()` após o loop `for rows.Next()` | Erros de rede/driver ocorridos durante a iteração passam despercebidos | Checar `rows.Err()` logo após o `for`, antes de considerar a leitura bem-sucedida |
| Comparar erro de ausência de linha com string (`err.Error() == "sql: no rows..."`) | Frágil e não captura wrapping do erro | Usar `errors.Is(err, sql.ErrNoRows)` |

## Avaliação de risco

- **Parar e confirmar quando:** a mudança envolver alterar limites de pool
  (`SetMaxOpenConns`/`SetMaxIdleConns`) em produção sem medir a capacidade real do servidor de banco
- **Risco alto:** remover parametrização de uma query já existente (ex.: para "simplificar"
  concatenando string) — reintroduz SQL injection
- **Risco baixo:** adicionar `defer rows.Close()`/`rows.Err()` ausente, trocar `Query`/`Exec`
  sem contexto pela variante `*Context` equivalente

## Métricas de sucesso

- Zero ocorrências de SQL montado por concatenação/`fmt.Sprintf` (verificável por grep/lint)
- 100% das chamadas a `*sql.Rows` seguidas de `defer Close()` e checagem de `rows.Err()`
- `go vet ./...` e `go build ./...` sem erros/avisos após qualquer mudança de acesso a dados
- Pool configurado com `SetMaxOpenConns`/`SetMaxIdleConns`/`SetConnMaxLifetime` explícitos, não apenas defaults implícitos

## Responsável principal

| Papel | Quem |
|-------|------|
| Agent executor | `developer-golang-agent-orchestrator` |
| Revisão humana | Desenvolvedor / Tech Lead |
| Aprovação final | Tech Lead |

## Referências

- [pkg.go.dev/database/sql](https://pkg.go.dev/database/sql)
- Skill relacionada: `developer-go-error-handling-and-diagnostics`

---

## Versão interna (arquivo)

| Campo | Valor |
|-------|-------|
| **FileVersion** | 1.0.0 |
| **Política** | `.cursor/VERSION.md` |

## Changelog (este arquivo)

- 1.0.0 (09/08/2026): Criação da skill — acesso a banco de dados em Go via `database/sql`
  da stdlib (driver-agnostic, connection pooling, prepared statements, queries com
  contexto, fechamento correto de `rows`/`stmt`), integrando a família `developer-go-*`.

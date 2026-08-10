---
name: developer-go-error-handling-and-diagnostics
description: "Tratamento de erros idiomático em Go — pacote errors (Is/As/Unwrap/Join), erros customizados, sentinel errors, logging estruturado com log/slog e diagnóstico com Delve."
model: sonnet
thinking: normal
category: developer-go
license: MIT
copyright: "Copyright (c) 2026 CSL Tech Solutions"
company: "CSL Tech Solutions"
author: "Claiton de Souza Linhares"
---

# Tratamento de Erros e Diagnóstico em Go

## Responsabilidade única

Esta skill cobre o tratamento explícito de erros em Go via retorno de valores `error`
(Go não usa exceptions): o pacote `errors` (`Is`, `As`, `Unwrap`, `Join`), a criação de
tipos de erro customizados e sentinel errors, o wrapping de erros com `%w` para preservar
a cadeia de causa, logging estruturado com `log/slog` e diagnóstico de falhas em runtime
com o depurador Delve. Ela existe separada das demais skills da família `developer-go-*`
porque o modelo de erros do Go é fundamentalmente distinto de exceções — exige disciplina
própria de verificação (`if err != nil`) e composição de erros, sem stack unwinding.

## When to use

- Definir ou revisar erros customizados (`type MyError struct{}` com `Error() string`)
- Criar sentinel errors (`var ErrNotFound = errors.New(...)`) para comparação com `errors.Is`
- Envolver (wrap) erros preservando a causa original com `fmt.Errorf("...: %w", err)`
- Configurar logging estruturado de erros com `log/slog`
- Investigar falha em runtime com Delve (`dlv debug`, breakpoints, inspeção de goroutines)

## When NOT to use

- `panic`/`recover` para controle de fluxo normal ou validação de input → `developer-go-language-advanced`
- Fundamentos de funções com múltiplos retornos `(valor, error)` → `developer-go-language-core`
- Testes automatizados de cenários de erro (table-driven tests) → `developer-go-testing`
- Concorrência e propagação de erro entre goroutines (`errgroup`, `context`) → `developer-go-concurrency-basics`
- Build, CI/CD e empacotamento → `developer-go-build-toolchain`

## Inputs obrigatórios

| Input | Tipo | Descrição |
|-------|------|-----------|
| Código-fonte Go alvo | Arquivo(s) `.go` | Função(ões)/pacote onde o erro ocorre ou será modelado |
| Contexto da falha | logs, stack trace, mensagem de `error` | Evidência real do runtime, não hipótese |
| Versão do Go declarada | string (`go.mod`) | Confirma disponibilidade de `errors.Join` (1.20+) e `log/slog` (1.21+) |

## Dependências (skills prévias)

| Skill | Quando executar antes |
|-------|-----------------------|
| `developer-go-language-core` | Antes de modelar erros — pressupõe domínio de funções com retorno `(valor, error)` |

## Workflow executável

1. Classificar o erro: esperado (recuperável, ex.: registro não encontrado) vs. bug de
   invariante interno (candidato a `panic` em `developer-go-language-advanced`).
2. Definir sentinel error para casos comparáveis por identidade:

```go
var ErrNotFound = errors.New("recurso não encontrado")

func Find(id string) error {
    if id == "" {
        return ErrNotFound
    }
    return nil
}
```

3. Criar erro customizado quando for necessário carregar dados estruturados:

```go
type ValidationError struct {
    Field string
    Msg   string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("campo %q: %s", e.Field, e.Msg)
}
```

4. Envolver o erro na propagação, preservando a cadeia com `%w` (nunca `%v` quando a
   causa deve ser inspecionável a jusante):

```go
if err := Find(id); err != nil {
    return fmt.Errorf("buscar registro %s: %w", id, err)
}
```

5. Inspecionar a cadeia no chamador com `errors.Is` (sentinel) ou `errors.As` (tipo):

```go
if errors.Is(err, ErrNotFound) {
    // tratar ausência
}
var ve *ValidationError
if errors.As(err, &ve) {
    // tratar ve.Field, ve.Msg
}
```

6. Agregar múltiplos erros independentes (ex.: validação de vários campos) com `errors.Join`.
7. Registrar o erro com `log/slog` incluindo contexto estruturado (não apenas a mensagem):

```go
slog.Error("falha ao buscar registro", "id", id, "err", err)
```

8. Se a causa não for evidente pelos logs, depurar com Delve (`dlv debug ./cmd/app`,
   `break pacote.Funcao`, `continue`, `print err`) antes de propor correção.

> Snippets acima ≤ 15 linhas cada. Exemplo combinado maior → `./exemplos/errors.go`.

## Outputs obrigatórios

| Output | Localização | Formato |
|--------|-------------|---------|
| Erros customizados/sentinel definidos | pacote de domínio correspondente (ex.: `./internal/<pacote>/errors.go`) | Go |
| Erros propagados com wrapping (`%w`) | funções que retornam `error` no caminho de chamada | Go |
| Logs estruturados de falha | via `log/slog`, chave-valor (`err`, `id`, contexto) | texto/JSON (slog) |
| Evidência de diagnóstico (quando aplicável) | sessão Delve documentada no report/commit | texto |

## Checklist de validação

- [ ] Todo `error` retornado é verificado (`if err != nil`) antes de prosseguir
- [ ] Erros propagados usam `fmt.Errorf("...: %w", err)`, nunca perdem a causa original
- [ ] `errors.Is`/`errors.As` usados para inspecionar a cadeia, nunca comparação de string
- [ ] Sentinel errors exportados com nome `Err<Algo>`; erros customizados implementam `Error() string`
- [ ] Logs de erro via `log/slog` incluem contexto estruturado, não só a mensagem crua

---

## Stack e versões  ← OBRIGATÓRIO (Go)

| Componente | Versão mínima | Notas |
|------------|:---:|-------|
| Go | 1.21 | `go.mod` declara `go 1.21` ou superior — `errors.Join` requer 1.20+ |
| gofmt | embutido | Formatação obrigatória, sem exceções |
| go vet | embutido | Rodar antes de qualquer commit |
| golangci-lint | 1.55+ | Lint agregador (opcional mas recomendado) |

## Dependências (go.mod / go get)  ← OBRIGATÓRIO (Go)

```bash
go mod init <module-path>
go get <pacote>@<versão>
go mod tidy
```

**Conflitos conhecidos:** nenhum conhecido — `errors` e `log/slog` são stdlib pura desde 1.21.

## Checklist Go  ← OBRIGATÓRIO (Go)

- [ ] `gofmt -l .` sem saída (código formatado)
- [ ] `go vet ./...` sem avisos
- [ ] `go build ./...` sem erros
- [ ] `go test ./...` verde (quando aplicável ao tema da skill)
- [ ] Erros tratados explicitamente (`if err != nil`), nunca descartados com `_` sem justificativa
- [ ] Erros envolvidos (`fmt.Errorf("...: %w", err)`) preservam a cadeia para `errors.Is`/`errors.As`

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
| `if err != nil { return err }` sem contexto | Perde de onde veio o erro; stack trace inexiste em Go, só a mensagem ajuda | Envolver com `fmt.Errorf("<operação>: %w", err)` antes de retornar |
| `panic` para erro esperado (ex.: input inválido, registro ausente) | Interrompe o programa/goroutine; força `recover` para algo que deveria ser fluxo normal | Retornar `error` explícito; reservar `panic` para bugs de invariante interno |
| `_ = err` sem justificativa | Descarta falha silenciosamente; comportamento indefinido segue adiante | Tratar (`if err != nil`) ou comentar explicitamente por que é seguro ignorar |
| Comparar erro por string (`err.Error() == "not found"`) | Frágil — quebra se a mensagem mudar; não captura wrapping | Usar `errors.Is`/`errors.As` com sentinel/tipo customizado |
| Logar erro com `fmt.Println(err)` sem contexto estruturado | Impossível correlacionar em produção (sem campos pesquisáveis) | Usar `slog.Error("mensagem", "err", err, "campo", valor)` |

## Avaliação de risco

- **Parar e confirmar quando:** a correção exigir mudar a assinatura de uma função pública
  (adicionar/remover retorno `error`), pois quebra todos os chamadores do pacote
- **Risco alto:** substituir `panic` recuperável por `error` em código já em produção sem
  auditar todos os `recover` existentes que dependiam do comportamento anterior
- **Risco baixo:** adicionar wrapping (`%w`) a um erro já tratado, criar sentinel error novo,
  adicionar campo de log estruturado

## Métricas de sucesso

- Zero ocorrências de erro descartado sem justificativa (`_ = err` sem comentário)
- 100% dos erros propagados entre camadas usam wrapping (`%w`), verificável com `errors.Unwrap`
- Sentinel errors e tipos customizados documentados com pelo menos um uso de `errors.Is`/`errors.As`
- `go vet ./...` e `go build ./...` sem erros/avisos após qualquer mudança de tratamento de erro

## Responsável principal

| Papel | Quem |
|-------|------|
| Agent executor | `developer-golang-agent-orchestrator` |
| Revisão humana | Desenvolvedor / Tech Lead |
| Aprovação final | Tech Lead |

## Referências

- [pkg.go.dev/errors](https://pkg.go.dev/errors)
- [pkg.go.dev/log/slog](https://pkg.go.dev/log/slog)
- Skill relacionada: `developer-go-testing`

---

## Versão interna (arquivo)

| Campo | Valor |
|-------|-------|
| **FileVersion** | 1.0.0 |
| **Política** | `.cursor/VERSION.md` |

## Changelog (este arquivo)

- 1.0.0 (09/08/2026): Criação da skill — tratamento de erros idiomático em Go (pacote
  `errors`, erros customizados, sentinel errors, wrapping com `%w`, logging estruturado
  com `log/slog`, diagnóstico com Delve), integrando a família `developer-go-*`.

---
name: developer-go-cli-apps
description: "Aplicações de linha de comando em Go com o pacote flag — subcomandos via flag.NewFlagSet, configuração por env vars + flags, main() minimalista delegando para run() error e exit codes disciplinados."
model: sonnet
thinking: normal
category: developer-go
license: MIT
copyright: "Copyright (c) 2026 CSL Tech Solutions"
company: "CSL Tech Solutions"
author: "Claiton de Souza Linhares"
---

# Aplicações CLI em Go

## Responsabilidade única

Esta skill cobre a construção de aplicações de linha de comando em Go usando a
biblioteca padrão: definição de flags com o pacote `flag`, subcomandos via
`flag.NewFlagSet`, precedência de configuração (flag > env var > default) e o
padrão `main()` minimalista que delega para uma função `run() error` testável,
com exit codes explícitos via `os.Exit`. Ela existe separada das demais skills
`developer-go-*` porque o formato de entrada/saída de uma CLI (parsing de
argumentos, usage, códigos de saída) é uma preocupação estrutural própria, distinta
de lógica de negócio, de um servidor HTTP ou do layout geral de um projeto maior.

## When to use

- Criar um binário `cmd/<nome>/main.go` que recebe flags e/ou argumentos posicionais
- Adicionar um subcomando a uma CLI existente (padrão `app <subcomando> [flags]`)
- Definir precedência de configuração entre flag, variável de ambiente e valor padrão
- Revisar `main()` que mistura lógica de negócio com parsing de flags e `os.Exit`
- Padronizar mensagens de `-h`/usage e códigos de saída de um binário Go

## When NOT to use

- Servidor HTTP (rotas, handlers, middlewares) → `developer-go-http-server`
- Layout de projeto multi-pacote, módulos internos, organização de `cmd/`/`internal/`/`pkg/` → `developer-go-architecture-and-design`
- Lógica de negócio pura desacoplada de I/O → `developer-go-language-core`
- Testes automatizados de comandos (table-driven, golden files) → `developer-go-testing`
- Build multiplataforma, versionamento embutido, distribuição do binário → `developer-go-build-toolchain`

## Inputs obrigatórios

| Input | Tipo | Descrição |
|-------|------|-----------|
| Especificação dos comandos | texto/lista | Nome do binário, subcomandos (se houver), flags e argumentos posicionais esperados |
| Versão do Go declarada | string (`go.mod`) | Confirma disponibilidade de recursos usados (`flag`, `os`, `log/slog`) |
| Fontes de configuração | lista | Quais valores vêm de flag, quais de env var, e o default de cada um |

## Dependências (skills prévias)

| Skill | Quando executar antes |
|-------|-----------------------|
| `developer-go-language-core` | Antes de estruturar `main()`/`run()` — pressupõe domínio de funções com retorno `(valor, error)` |
| `developer-go-error-handling-and-diagnostics` | Antes de definir como erros de `run()` viram mensagem + exit code |

## Workflow executável

1. Definir o binário em `cmd/<nome>/main.go`; `main()` só chama `run()` e traduz o erro em exit code.
2. Declarar flags no escopo de `run()` (não em `init()` global) para permitir testes isolados:

```go
func run(args []string, stdout io.Writer) error {
    fs := flag.NewFlagSet("app", flag.ContinueOnError)
    verbose := fs.Bool("verbose", false, "saída detalhada")
    if err := fs.Parse(args); err != nil {
        return err
    }
    _ = verbose
    return nil
}
```

3. Resolver precedência flag > env var > default explicitamente (flag já setada vence):

```go
func withEnvDefault(fs *flag.FlagSet, name, env, def, usage string) *string {
    if v := os.Getenv(env); v != "" {
        def = v
    }
    return fs.String(name, def, usage)
}
```

4. Para subcomandos, criar um `flag.NewFlagSet` por subcomando e despachar pelo primeiro argumento:

```go
switch os.Args[1] {
case "sync":
    return runSync(os.Args[2:])
case "status":
    return runStatus(os.Args[2:])
default:
    return fmt.Errorf("subcomando desconhecido: %s", os.Args[1])
}
```

5. `main()` traduz erro em código de saída — nunca chamar `os.Exit` dentro de `run()`:

```go
func main() {
    if err := run(os.Args[1:], os.Stdout); err != nil {
        fmt.Fprintln(os.Stderr, "erro:", err)
        os.Exit(1)
    }
}
```

6. Garantir `-h`/`--help` com usage claro — `flag` já gera a partir de `fs.Usage` customizável quando os defaults não bastam.
7. Mapear exit codes com significado estável (0 = sucesso, 1 = erro genérico, 2 = uso inválido) e documentá-los no `README` do binário.

> Snippets acima ≤ 15 linhas cada. Exemplo combinado maior → `./exemplos/cli.go`.

## Outputs obrigatórios

| Output | Localização | Formato |
|--------|-------------|---------|
| Binário CLI com `main()` minimalista | `cmd/<nome>/main.go` | Go |
| Função `run(args []string, ...) error` testável | mesmo pacote de `main` ou pacote interno dedicado | Go |
| Subcomandos (quando houver) | um `flag.NewFlagSet` por subcomando | Go |
| Tabela de exit codes documentada | `README.md` do binário ou comentário de pacote | Markdown/Go doc |

## Checklist de validação

- [ ] `main()` não contém lógica de negócio, apenas chamada a `run()` + tradução de erro em exit code
- [ ] Flags parseadas com `flag.NewFlagSet` (não flags globais implícitas via `flag.String` direto em `init`/`var`)
- [ ] Precedência flag > env var > default está explícita e documentada
- [ ] `-h`/`--help` produz usage compreensível para todo comando e subcomando
- [ ] Exit codes são estáveis e documentados (não apenas `os.Exit(1)` genérico para tudo)

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

**Conflitos conhecidos:** nenhum conhecido — pacote `flag` é stdlib pura; para subcomandos complexos considerar `flag.NewFlagSet` por subcomando.

## Checklist Go  ← OBRIGATÓRIO (Go)

- [ ] `gofmt -l .` sem saída (código formatado)
- [ ] `go vet ./...` sem avisos
- [ ] `go build ./...` sem erros
- [ ] `go test ./...` verde (quando aplicável ao tema da skill)
- [ ] Erros tratados explicitamente (`if err != nil`), nunca descartados com `_` sem justificativa
- [ ] `main()` minimalista — delega lógica a funções/pacotes testáveis, nunca lógica de negócio direto em `main()`

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
| Lógica de negócio dentro de `main()` | Impede testar o comportamento sem passar por `os.Args`/processo real | Extrair para `run(args []string, ...) error` e testar `run` diretamente |
| `os.Exit` espalhado pelo código em vez de centralizado | Interrompe o processo abruptamente em qualquer ponto, pulando `defer`s e dificultando teste | Retornar `error` até `main()`; só `main()` chama `os.Exit` |
| Flags globais via `flag.String` em `var`/`init()` | Estado compartilhado entre testes; impossível rodar dois cenários de flags no mesmo processo de teste | Usar `flag.NewFlagSet` local por chamada de `run()`/subcomando |
| Subcomando sem `-h`/usage próprio | Usuário não descobre as flags específicas daquele subcomando | Definir `fs.Usage` customizado por `flag.NewFlagSet` |
| Exit code sempre `1` para qualquer falha | Impossibilita scripts diferenciarem erro de uso vs. falha de execução | Reservar código 2 para uso inválido, 1 para falha de execução, 0 para sucesso |

## Avaliação de risco

- **Parar e confirmar quando:** a mudança alterar o contrato de flags/exit codes de um
  binário já publicado (scripts de terceiros podem depender do comportamento atual)
- **Risco alto:** remover ou renomear uma flag existente sem período de depreciação,
  quebrando pipelines/automação que já invocam o binário
- **Risco baixo:** adicionar uma nova flag opcional com default retrocompatível, melhorar
  mensagem de usage, extrair lógica de `main()` para `run()` sem mudar comportamento externo

## Métricas de sucesso

- `go vet ./...` e `go build ./...` sem erros/avisos após qualquer mudança na CLI
- 100% dos binários com `main()` reduzido a chamada de `run()` + tratamento de exit code
- Cada subcomando responde a `-h` com usage específico e não genérico do binário pai
- Exit codes documentados e estáveis entre versões (verificável em changelog/README)

## Responsável principal

| Papel | Quem |
|-------|------|
| Agent executor | `developer-golang-agent-orchestrator` |
| Revisão humana | Desenvolvedor / Tech Lead |
| Aprovação final | Tech Lead |

## Referências

- [pkg.go.dev/flag](https://pkg.go.dev/flag)
- Skill relacionada: `developer-go-architecture-and-design`

---

## Versão interna (arquivo)

| Campo | Valor |
|-------|-------|
| **FileVersion** | 1.0.0 |
| **Política** | `.cursor/VERSION.md` |

## Changelog (este arquivo)

- 1.0.0 (09/08/2026): Criação da skill — aplicações CLI em Go com pacote `flag`,
  subcomandos via `flag.NewFlagSet`, configuração por env vars + flags, padrão
  `main()` minimalista delegando para `run() error` e exit codes, integrando a
  família `developer-go-*`.

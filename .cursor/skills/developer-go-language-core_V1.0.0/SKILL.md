---
name: developer-go-language-core
description: "Fundamentos da linguagem Go — sintaxe, variáveis, control flow, funções e módulos; ponto de entrada da família developer-go-* para quem escreve ou revisa código Go."
model: sonnet
thinking: normal
category: developer-go
license: MIT
copyright: "Copyright (c) 2026 CSL Tech Solutions"
company: "CSL Tech Solutions"
author: "Claiton de Souza Linhares"
---

# Fundamentos da Linguagem Go

## Responsabilidade única

Esta skill cobre os fundamentos sintáticos e estruturais da linguagem Go: declaração de
variáveis, fluxo de controle, funções (incluindo múltiplos retornos e variádicas), packages
e o sistema de módulos (`go.mod`/`go.sum`). Ela existe separada das demais skills da família
`developer-go-*` porque é o ponto de entrada — generics, concorrência, tratamento avançado de
erros e testes têm regras próprias e mais profundas, que pressupõem domínio destes fundamentos.
Não cobre tópicos avançados nem infraestrutura de build/deploy.

## When to use

- Escrever ou revisar sintaxe fundamental de Go: variáveis, `if`/`for`/`switch`, funções, packages
- Iniciar um novo módulo Go (`go mod init`) ou entender `go.mod`/`go.sum` de um projeto existente
- Dúvida sobre convenções idiomáticas básicas (nomenclatura, múltiplos retornos, variádicas)
- Servir de ponto de entrada antes de aprofundar em tópicos avançados da família `developer-go-*`

## When NOT to use

- Generics (`[T any]`, constraints) → `developer-go-language-generics`
- Concorrência (goroutines, channels, `sync`, `select`) → `developer-go-concurrency-basics`
- Wrapping/diagnóstico avançado de erros (`errors.Is`/`errors.As`) → `developer-go-error-handling-and-diagnostics`
- Testes automatizados (`testing`, tabelas de teste, mocks) → `developer-go-testing`
- Build, toolchain, CI/CD e empacotamento → `developer-go-build-toolchain`
- Triagem entre múltiplas skills da família → `developer-go-master-orchestrator`

## Inputs obrigatórios

| Input | Tipo | Descrição |
|-------|------|-----------|
| Código-fonte Go alvo | Arquivo(s) `.go` | Pacote ou arquivo existente a criar/revisar; se novo, indicar o caminho do pacote |
| Versão do Go declarada | string (`go.mod`) | Versão mínima exigida pelo projeto (ex.: `go 1.21`) |
| Objetivo da tarefa | texto livre | Nova feature, refactor, correção de bug ou revisão de sintaxe |

## Dependências (skills prévias)

Nenhuma dependência obrigatória — esta é a skill fundacional da família `developer-go-*`; as
demais (generics, concorrência, tratamento de erros, testes) partem destes fundamentos.

## Workflow executável

1. Confirmar o módulo do projeto: se não existir `go.mod`, rodar `go mod init <module-path>`.
2. Declarar variáveis com `var` (tipo explícito ou zero value) ou `:=` (inferência, só dentro de funções):

```go
var nome string = "Go"   // forma longa, tipo explícito
var contador int         // zero value: 0
idade := 30               // curta, só dentro de função
```

3. Implementar fluxo de controle com `if`, `for` (única construção de laço em Go) e `switch`
   (sem fallthrough implícito entre `case`s):

```go
if idade >= 18 {
    fmt.Println("maior de idade")
} else {
    fmt.Println("menor de idade")
}

for i := 0; i < 3; i++ {
    fmt.Println(i)
}
```

4. Escrever funções com múltiplos retornos (idiomaticamente `(valor, error)`) e funções
   variádicas quando o número de argumentos for dinâmico:

```go
func dividir(a, b float64) (float64, error) {
    if b == 0 {
        return 0, fmt.Errorf("divisão por zero")
    }
    return a / b, nil
}

func soma(nums ...int) int {
    total := 0
    for _, n := range nums {
        total += n
    }
    return total
}
```

5. Organizar o código em packages coesos: um pacote por diretório, nome curto em minúsculas,
   sem stutter entre pacote e tipo exportado (ver Anti-padrões).
6. Validar com `gofmt`, `go vet` e `go build` antes de considerar a tarefa concluída.

> Snippets acima ≤ 15 linhas cada. Exemplo combinado maior → `./exemplos/fundamentos.go`.

## Outputs obrigatórios

| Output | Localização | Formato |
|--------|-------------|---------|
| Código Go novo/revisado | caminho do pacote no projeto (ex.: `./internal/<pacote>/*.go`) | Go |
| `go.mod`/`go.sum` atualizados | raiz do módulo | Go modules |
| Evidência de validação (gofmt/vet/build) | log anexado ao commit/PR | texto |

## Checklist de validação

- [ ] Código organizado em packages coerentes, sem dependências circulares
- [ ] Nomeação idiomática: não exportado em `camelCase`, exportado em `PascalCase`
- [ ] Toda função que pode falhar retorna `error` como último valor
- [ ] Nenhuma variável declarada e não utilizada (Go não compila com isso)
- [ ] `go.mod` reflete a versão mínima real usada pelo código

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
| Ignorar erro com `_ = err` ou nem capturar o retorno de erro | Mascara falhas silenciosamente; o programa segue em estado indefinido | Tratar com `if err != nil { ... }` ou propagar com `fmt.Errorf("...: %w", err)` |
| Usar `panic`/`recover` para controle de fluxo normal (ex.: validar input) | Go reserva `panic` para erros irrecuperáveis; abusar dificulta rastrear a causa real | Retornar `error` explícito; reservar `panic` para bugs de invariante interno |
| Nomear pacote com stutter (ex.: pacote `user` expor `user.UserService`) | Redundante na chamada (`user.UserService`); fere a convenção de nomes curtos do Go | Nomear o tipo apenas `Service` — o pacote já dá o contexto (`user.Service`) |
| Fazer shadowing acidental de `err` com `:=` dentro de `if`/`for` | A variável externa nunca é atualizada; erros reais passam despercebidos | Usar `=` quando a variável já existe no escopo, ou nomear a nova var explicitamente |
| Variáveis globais mutáveis compartilhadas sem sincronização | Data race silenciosa, só detectável com `-race` ou em produção | Encapsular estado em struct com mutex ou usar channels; rodar `go test -race` |

## Avaliação de risco

- **Parar e confirmar quando:** a mudança exigir subir a versão mínima do Go no `go.mod`
  (pode quebrar builds de CI ou de outros consumidores do módulo)
- **Risco alto:** remover checagem de erro (`if err != nil`) existente "para simplificar" —
  pode ocultar falha em produção
- **Risco baixo:** renomear variável local, ajustar formatação (`gofmt`), reorganizar imports

## Métricas de sucesso

- `gofmt -l .` retorna vazio (100% do código formatado)
- `go vet ./...` e `go build ./...` sem erros/avisos
- Zero ocorrências de erro descartado sem justificativa (`_ = err` sem comentário)
- Cobertura mantida ou aumentada em `go test ./... -cover` (quando aplicável)

## Responsável principal

| Papel | Quem |
|-------|------|
| Agent executor | `developer-golang-agent-orchestrator` |
| Revisão humana | Desenvolvedor / Tech Lead |
| Aprovação final | Tech Lead |

## Referências

- [Effective Go](https://go.dev/doc/effective_go)
- [The Go Programming Language Specification](https://go.dev/ref/spec)
- [pkg.go.dev/std](https://pkg.go.dev/std)
- Skill relacionada: `developer-go-master-orchestrator`

---

## Versão interna (arquivo)

| Campo | Valor |
|-------|-------|
| **FileVersion** | 1.0.0 |
| **Política** | `.cursor/VERSION.md` |

## Changelog (este arquivo)

- 1.0.0 (09/08/2026): Criação da skill — fundamentos da linguagem Go (sintaxe, variáveis,
  control flow, funções, packages, módulos), inaugurando a família `developer-go-*`.

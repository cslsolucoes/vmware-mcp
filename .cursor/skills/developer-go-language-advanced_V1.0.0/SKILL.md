---
name: developer-go-language-advanced
description: Recursos avançados da linguagem Go — closures, defer/panic/recover, build tags, iota, unsafe básico e ordem de inicialização de pacotes.
model: sonnet
thinking: normal
category: developer-go
license: MIT
copyright: "Copyright (c) 2026 CSL Tech Solutions"
company: "CSL Tech Solutions"
author: "Claiton de Souza Linhares"
---

# Go Avançado

## Responsabilidade única

Esta skill cobre os recursos avançados da linguagem Go que vão além da sintaxe básica e do
sistema de tipos: closures e captura de variáveis, o trio `defer`/`panic`/`recover` para
controle de fluxo excepcional, build tags/constraints para compilação condicional, `iota`
para enumerações, uso básico e justificado de `unsafe`, e a ordem determinística de
inicialização de pacotes (`var` → `init()` → `main()`). Ela existe separada de
`developer-go-language-core` e `developer-go-language-types` porque trata de mecanismos que
alteram fluxo de execução e layout de memória, não apenas sintaxe e tipos.

## When to use

- Implementar callbacks, predicados ou factories que capturam estado via closure
- Garantir liberação de recursos (arquivos, locks, conexões) com `defer`
- Recuperar de pânicos em fronteiras de goroutine ou de API pública com `recover`
- Compilar variantes de arquivo por SO/arquitetura/tag customizada (`//go:build`)
- Definir enumerações sequenciais ou bitmask com `iota`
- Justificar e isolar um uso pontual de `unsafe.Pointer` (ex.: interoperar com cgo)
- Entender ou depurar a ordem de execução de `init()` entre pacotes importados

## When NOT to use

- Goroutines, channels, `sync.WaitGroup`, `select` → usar `developer-go-concurrency-basics`
- Reflection (`reflect.TypeOf`, `reflect.ValueOf`, tags de struct via reflection) → usar
  `developer-go-stdlib-rtti-reflection`
- Sintaxe básica de funções, structs, slices/maps → usar `developer-go-language-core`
- Sistema de tipos, interfaces, generics → usar `developer-go-language-types`
- Tratamento de erros idiomático (`errors.Is/As`, wrapping) → usar
  `developer-go-error-handling-and-diagnostics`

## Inputs obrigatórios

| Input | Tipo | Descrição |
|-------|------|-----------|
| `módulo Go` | `go.mod` | Módulo já inicializado (`go mod init`) com Go ≥ 1.21 declarado |
| `arquivo(s) alvo` | `.go` | Arquivo(s) onde o recurso avançado será aplicado |
| `justificativa` | `texto` | Obrigatória apenas quando envolver `unsafe` ou `panic` fora de teste |

## Dependências (skills prévias)

| Skill | Quando executar antes |
|-------|------------------------|
| `developer-go-language-core` | Sempre — domínio de funções, structs e control flow básico |
| `developer-go-language-types` | Sempre — domínio de interfaces e generics antes de closures tipadas |

## Workflow executável

1. Identificar o recurso avançado necessário (closure, defer/panic/recover, build tag,
   `iota`, `unsafe` ou `init()`) a partir do requisito.
2. Para `defer`, confirmar que a chamada libera um recurso ou reverte estado — nunca usar em
   loop sem necessidade real.
3. Para `panic`/`recover`, confirmar que representa erro **irrecuperável** de programação, não
   fluxo de erro esperado (esse usa `error` de retorno).
4. Para build tags, escrever `//go:build` na primeira linha do arquivo (com linha em branco
   depois) e validar com `go build -tags=<tag>`.
5. Para `unsafe`, documentar no comentário acima da linha o motivo e a garantia que se está
   assumindo/quebrando.
6. Rodar `gofmt -l .`, `go vet ./...` e `go build ./...` antes de considerar concluído.

```go
func withRecover(f func()) (err error) {
    defer func() {
        if r := recover(); r != nil {
            err = fmt.Errorf("recovered: %v", r)
        }
    }()
    f()
    return nil
}
```

## Outputs obrigatórios

| Output | Localização | Formato |
|--------|-------------|---------|
| Código Go alterado/criado | `<módulo>/**/*.go` | Go (gofmt aplicado) |
| Justificativa de `unsafe`/`panic` (se usado) | comentário acima da linha | Go comment |

## Checklist de validação

- [ ] Closures capturam apenas as variáveis necessárias (sem captura acidental de variável de loop)
- [ ] `defer` usado apenas para liberação/reversão de estado, nunca em loop sem necessidade
- [ ] `panic` restrito a erros de programação; fluxo normal usa `error`
- [ ] Build tags testadas com `go build -tags=<tag>` para cada combinação relevante
- [ ] `unsafe` (quando presente) tem comentário de justificativa e é a exceção, não a regra
- [ ] Ordem de `init()` entre pacotes não é assumida sem verificação (dependências de import)

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

**Conflitos conhecidos:** `unsafe` quebra garantias de portabilidade e do garbage collector — usar apenas com justificativa documentada.

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
| `defer` dentro de loop `for` acumulando recursos | Todos os `defer` só executam ao sair da função, não da iteração — pode esgotar file descriptors/locks | Extrair o corpo do loop para uma função própria onde o `defer` roda por iteração |
| `panic` para sinalizar erro esperado (ex.: registro não encontrado) | Quebra o fluxo idiomático de Go e força `recover` em quem chama, escondendo o contrato | Retornar `error` (ou `nil, error`) e deixar o chamador decidir |
| `unsafe.Pointer` para "economizar" uma conversão trivial | Quebra portabilidade, garantias do GC e pode causar corrupção de memória sutil | Usar conversão de tipo segura; reservar `unsafe` para interoperar com C/syscalls documentados |
| Depender da ordem de `init()` entre pacotes irmãos sem import explícito | Go não garante ordem entre pacotes sem relação de dependência — comportamento pode mudar | Tornar a dependência explícita via import ou mover a lógica para `main()` |

## Avaliação de risco

- **Parar e confirmar quando:** a mudança introduzir o primeiro uso de `unsafe` no módulo, ou
  substituir um `error` de retorno por `panic`/`recover` em código de produção.
- **Risco alto:** uso de `unsafe.Pointer` para reinterpretar layout de struct entre versões do
  Go sem teste de regressão; `recover` silencioso que mascara bug real sem log.
- **Risco baixo:** closures locais sem captura de ponteiro compartilhado; `iota` em
  enumeração nova; build tags aditivas que não afetam o build padrão.

## Métricas de sucesso

- `go vet ./...` e `gofmt -l .` sem saída após a mudança
- Zero `panic` novo fora de `main()`, testes ou código explicitamente marcado como fatal
- 100% dos usos de `unsafe` acompanhados de comentário de justificativa

## Responsável principal

| Papel | Quem |
|-------|------|
| Agent executor | `developer-golang-agent-orchestrator` |
| Revisão humana | Desenvolvedor / Tech Lead |
| Aprovação final | Tech Lead |

## Referências

- [Effective Go — Defer](https://go.dev/doc/effective_go#defer)
- [Effective Go — Panic and Recover](https://go.dev/doc/effective_go#panic)
- Skill relacionada: `developer-go-error-handling-and-diagnostics`
- Skill relacionada: `developer-go-language-core`
- Skill relacionada: `developer-go-language-types`

---

## Versão interna (arquivo)

| Campo | Valor |
|-------|-------|
| **FileVersion** | 1.0.0 |
| **Política** | `.cursor/VERSION.md` |

## Changelog (este arquivo)

- 1.0.0 (09/08/2026): Criação da skill — recursos avançados de Go (closures, defer/panic/recover,
  build tags/constraints, iota, unsafe básico, init() e ordem de inicialização de pacotes).

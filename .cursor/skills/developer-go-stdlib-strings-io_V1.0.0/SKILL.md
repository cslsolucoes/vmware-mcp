---
name: developer-go-stdlib-strings-io
description: "Manipulação de strings e I/O em Go — pacotes strings, bytes, bufio, interfaces io.Reader/io.Writer e formatação com fmt."
model: sonnet
thinking: normal
category: developer-go
license: MIT
copyright: "Copyright (c) 2026 CSL Tech Solutions"
company: "CSL Tech Solutions"
author: "Claiton de Souza Linhares"
---

# Strings e I/O em Go

## Responsabilidade única

Esta skill cobre a stdlib de manipulação de texto e entrada/saída em Go: o pacote `strings`
(incluindo `strings.Builder` e as funções de busca/transformação sobre UTF-8), o pacote `bytes`
para manipulação de slices de bytes, `bufio` (`Scanner`, `Reader`, `Writer`) para I/O bufferizado,
as interfaces fundamentais `io.Reader`/`io.Writer` que sustentam toda a stdlib de I/O, e `fmt`
para formatação (`Printf`/`Sprintf`/verbos). Ela existe separada das demais skills da família
`developer-go-*` porque texto e I/O têm idiomas próprios (streaming via interfaces, buffers,
custo de alocação) que não pertencem aos fundamentos de sintaxe nem à camada de encoding.

## When to use

- Concatenar, construir ou transformar strings de forma eficiente (`strings.Builder`, `strings.Join`)
- Buscar, dividir, substituir ou comparar substrings (`Contains`, `Split`, `Replace`, `TrimSpace`)
- Ler/escrever streams de texto ou binário linha a linha ou em blocos (`bufio.Scanner`/`bufio.Reader`)
- Implementar ou consumir código que aceita `io.Reader`/`io.Writer` como abstração de I/O
- Formatar saída com `fmt.Printf`/`fmt.Sprintf` e escolher o verbo correto (`%s`, `%d`, `%v`, `%q`)
- Manipular slices de bytes brutos sem conversão desnecessária para `string` (`bytes.Buffer`)

## When NOT to use

- Serialização/desserialização estruturada (JSON, XML, encoding binário) → `developer-go-stdlib-encoding`
- Expressões regulares (`regexp`) → tratado como tópico avançado de texto, fora do escopo desta skill
- Concorrência sobre streams (pipes com goroutines, `sync`) → `developer-go-concurrency-basics`
- Sintaxe fundamental da linguagem (variáveis, control flow, funções) → `developer-go-language-core`
- Coleções genéricas (slices, maps, generics de container) → `developer-go-stdlib-collections`

## Inputs obrigatórios

| Input | Tipo | Descrição |
|-------|------|-----------|
| Código-fonte Go alvo | Arquivo(s) `.go` | Pacote ou arquivo existente a criar/revisar |
| Origem/destino do I/O | descrição textual | Arquivo, stdin/stdout, rede, buffer em memória — define qual implementação de `io.Reader`/`io.Writer` usar |
| Volume de dados esperado | texto livre | Pequeno (string simples) vs grande (justifica `bufio`/`strings.Builder`) |

## Dependências (skills prévias)

| Skill | Quando executar antes |
|-------|-----------------------|
| `developer-go-language-core` | Fundamentos de sintaxe, funções e módulos — pré-requisito para qualquer código desta skill |

## Workflow executável

1. Para concatenação repetida, usar `strings.Builder` em vez de `+=` (evita realocações):

```go
var b strings.Builder
for _, s := range []string{"a", "b", "c"} {
    b.WriteString(s)
}
resultado := b.String()
```

2. Para busca/transformação simples, usar as funções do pacote `strings`:

```go
if strings.Contains(nome, "@") {
    partes := strings.Split(nome, "@")
    limpo := strings.TrimSpace(partes[0])
}
```

3. Para ler um stream linha a linha, usar `bufio.Scanner` sobre qualquer `io.Reader`:

```go
scanner := bufio.NewScanner(reader)
for scanner.Scan() {
    linha := scanner.Text()
    _ = linha
}
if err := scanner.Err(); err != nil {
    return fmt.Errorf("leitura falhou: %w", err)
}
```

4. Para I/O bufferizado de escrita, usar `bufio.Writer` e sempre `Flush()` ao final:

```go
w := bufio.NewWriter(destino)
defer w.Flush()
fmt.Fprintf(w, "linha: %s\n", valor)
```

5. Aceitar `io.Reader`/`io.Writer` como parâmetro de função em vez de tipos concretos
   (`*os.File`, `*bytes.Buffer`) para maximizar reuso e testabilidade.
6. Formatar saída com o verbo correto de `fmt` (`%s` string, `%d` inteiro, `%v` valor genérico,
   `%q` string entre aspas escapadas, `%w` apenas em `Errorf` para wrapping de erro).
7. Validar com `gofmt`, `go vet` e `go build` antes de considerar a tarefa concluída.

> Snippets acima ≤ 15 linhas cada. Exemplo combinado maior → `./exemplos/README.md`.

## Outputs obrigatórios

| Output | Localização | Formato |
|--------|-------------|---------|
| Código Go novo/revisado | caminho do pacote no projeto (ex.: `./internal/<pacote>/*.go`) | Go |
| Evidência de validação (gofmt/vet/build) | log anexado ao commit/PR | texto |

## Checklist de validação

- [ ] Concatenação em loop usa `strings.Builder`, nunca `+=` repetido
- [ ] Toda leitura de `bufio.Scanner` verifica `scanner.Err()` após o loop
- [ ] Todo `bufio.Writer` tem `Flush()` garantido (idealmente via `defer`)
- [ ] Funções que fazem I/O aceitam `io.Reader`/`io.Writer`, não tipos concretos, quando possível
- [ ] Verbos de `fmt` conferem com o tipo do argumento (sem `%s` para inteiro, por exemplo)

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
| Concatenar string em loop com `+=` | Cada `+=` aloca uma nova string; custo quadrático em laços grandes | Usar `strings.Builder` com `WriteString` |
| Ignorar `scanner.Err()` após `for scanner.Scan()` | Erros de leitura (I/O, buffer excedido) passam despercebidos como fim normal do stream | Sempre checar `scanner.Err()` logo após o loop |
| Esquecer `bufio.Writer.Flush()` | Dados ficam retidos no buffer e nunca chegam ao destino se o processo terminar antes do flush | Usar `defer w.Flush()` imediatamente após criar o writer |
| Converter `[]byte` para `string` e vice-versa repetidamente em hot path | Cada conversão copia o buffer inteiro; custo de alocação desnecessário | Manter `bytes.Buffer`/`[]byte` até a fronteira onde `string` é realmente exigida |
| Aceitar `*os.File` como parâmetro quando bastaria `io.Writer` | Acopla a função a um tipo concreto, dificultando testes com buffers em memória | Tipar o parâmetro como `io.Reader`/`io.Writer` |

## Avaliação de risco

- **Parar e confirmar quando:** a mudança trocar a assinatura pública de uma função para receber
  `io.Reader`/`io.Writer` — pode quebrar chamadores existentes do pacote
- **Risco alto:** ignorar erro de `bufio.Scanner`/`bufio.Writer` em código de produção que processa
  arquivos ou streams de rede — perda silenciosa de dados
- **Risco baixo:** trocar `+=` por `strings.Builder` em função local, ajustar verbo de `fmt.Printf`

## Métricas de sucesso

- `gofmt -l .` retorna vazio (100% do código formatado)
- `go vet ./...` e `go build ./...` sem erros/avisos
- Zero laços de concatenação de string sem `strings.Builder` em código novo
- 100% dos usos de `bufio.Scanner` com checagem de `Err()` após o loop

## Responsável principal

| Papel | Quem |
|-------|------|
| Agent executor | `developer-golang-agent-orchestrator` |
| Revisão humana | Desenvolvedor / Tech Lead |
| Aprovação final | Tech Lead |

## Referências

- [pkg.go.dev/strings](https://pkg.go.dev/strings)
- [pkg.go.dev/io](https://pkg.go.dev/io)
- Skill relacionada: `developer-go-stdlib-collections`

---

## Versão interna (arquivo)

| Campo | Valor |
|-------|-------|
| **FileVersion** | 1.0.0 |
| **Política** | `.cursor/VERSION.md` |

## Changelog (este arquivo)

- 1.0.0 (09/08/2026): Criação da skill — strings (`strings`, `strings.Builder`), `bytes`, `bufio`
  (`Scanner`/`Reader`/`Writer`), interfaces `io.Reader`/`io.Writer` e formatação com `fmt`.

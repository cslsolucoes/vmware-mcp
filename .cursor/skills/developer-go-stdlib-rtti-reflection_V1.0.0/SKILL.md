---
name: developer-go-stdlib-rtti-reflection
description: Reflection em Go via pacote reflect — TypeOf, ValueOf, leitura de struct tags e critérios para evitar reflection em favor de generics/interfaces.
model: sonnet
thinking: normal
category: developer-go
license: MIT
copyright: "Copyright (c) 2026 CSL Tech Solutions"
company: "CSL Tech Solutions"
author: "Claiton de Souza Linhares"
---

# Reflection em Go

## Responsabilidade única

Esta skill cobre a inspeção e manipulação de tipos e valores em tempo de execução via pacote
`reflect`: obter o tipo estático/dinâmico de um valor com `TypeOf`/`ValueOf`, ler struct tags
(`json`, `validate`, etc.) através de `StructTag.Get`, percorrer campos e métodos de uma struct
desconhecida em compile-time, e alterar valores via reflection quando endereçáveis. Ela existe
separada de `developer-go-language-types` (sistema de tipos estático, type assertion/switch sobre
conjuntos conhecidos) porque trata do caso em que o tipo só é conhecido em runtime — o preço dessa
flexibilidade é perda de checagem em compile-time e custo de performance, por isso esta skill também
define quando **não** usar reflection.

## When to use

- Construir um mapper genérico (ex.: JSON↔struct, form↔struct) que precisa funcionar com qualquer tipo
- Ler struct tags (`json:"..."`, `validate:"..."`, `db:"..."`) para orientar serialização ou validação
- Implementar um container de injeção de dependência ou registro de tipos por nome
- Inspecionar um valor `any`/`interface{}` cujo tipo concreto só é conhecido em runtime e não há como
  fechar o conjunto de tipos com type switch
- Escrever bibliotecas de propósito geral (ORMs, validadores, serializadores) consumidas por código
  que o autor da biblioteca não controla

## When NOT to use

- Cenário resolvido por generics com segurança de tipos em tempo de compilação (`[T any]`) →
  `developer-go-language-generics`
- Inspeção de tipo conhecido em compile-time, com conjunto fechado de tipos → type switch, ver
  `developer-go-language-types`
- Hot path com chamadas por request/loop de alta frequência, onde o custo de reflection degrada
  throughput → preferir geração de código (`go generate`) ou interfaces explícitas
- Necessidade apenas de comportamento polimórfico simples → interfaces já resolvem sem reflection

## Inputs obrigatórios

| Input | Tipo | Descrição |
|-------|------|-----------|
| Valor ou tipo a inspecionar | `interface{}`/`any` | Valor concreto cujo tipo dinâmico será inspecionado via `reflect` |
| Struct com tags | `.go` (definição de struct) | Struct cujos campos possuem tags (`json`, `validate`, etc.) a serem lidas |
| Ponteiro para valor mutável | `*T` | Necessário quando a operação precisa alterar o valor via reflection (`Elem()` + `CanSet()`) |

## Dependências (skills prévias)

| Skill | Quando executar antes |
|-------|-----------------------|
| `developer-go-language-types` | Garantir domínio de structs, interfaces e type assertion antes de recorrer a `reflect` |

## Workflow executável

1. Obter o tipo estático e o valor dinâmico de uma variável com `reflect.TypeOf`/`reflect.ValueOf`.
   ```go
   v := 42
   t := reflect.TypeOf(v)
   rv := reflect.ValueOf(v)
   fmt.Println(t.Name(), t.Kind(), rv.Int())
   ```
2. Declarar struct tags e lê-las via `StructTag.Get`.
   ```go
   type User struct {
       Name string `json:"name" validate:"required"`
       Age  int    `json:"age"`
   }
   f, _ := reflect.TypeOf(User{}).FieldByName("Name")
   fmt.Println(f.Tag.Get("json"), f.Tag.Get("validate"))
   ```
3. Percorrer campos exportados dinamicamente (base de um mapper genérico).
   ```go
   val := reflect.ValueOf(User{Name: "Ana", Age: 30})
   for i := 0; i < val.NumField(); i++ {
       field := val.Type().Field(i)
       fmt.Println(field.Name, val.Field(i).Interface())
   }
   ```
4. Alterar um valor via reflection exige ponteiro endereçável e checagem de `CanSet()`.
   ```go
   u := &User{}
   elem := reflect.ValueOf(u).Elem()
   name := elem.FieldByName("Name")
   if name.CanSet() && name.Kind() == reflect.String {
       name.SetString("Bruno")
   }
   ```
5. Checar `Kind()` antes de extrair o valor para evitar panic em runtime.
   ```go
   func asInt(v reflect.Value) (int64, bool) {
       if v.Kind() != reflect.Int && v.Kind() != reflect.Int64 {
           return 0, false
       }
       return v.Int(), true
   }
   ```
6. Medir o custo com benchmark antes de aceitar reflection em caminho crítico.
   ```go
   func BenchmarkReflectGet(b *testing.B) {
       u := User{Name: "Ana"}
       for i := 0; i < b.N; i++ {
           _ = reflect.ValueOf(u).FieldByName("Name").String()
       }
   }
   ```

## Outputs obrigatórios

| Output | Localização | Formato |
|--------|-------------|---------|
| Função/mapper baseado em reflection | Pacote Go alvo (`*.go`) | Go |
| Testes cobrindo tipos aceitos e rejeitados | `*_test.go` no mesmo pacote | Go |

## Checklist de validação

- [ ] `reflect.TypeOf`/`reflect.ValueOf` só chamados quando o tipo não pode ser conhecido em compile-time
- [ ] Toda leitura de tag usa `StructTag.Get` com fallback explícito quando a tag está ausente
- [ ] Toda escrita via reflection verifica `CanSet()` antes de `Set*`
- [ ] Nenhum `panic` não tratado por `Kind()` incompatível (ex.: chamar `.Int()` em `Kind() == reflect.String`)
- [ ] Benchmark (`go test -bench`) documentado quando reflection está em caminho crítico

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

**Conflitos conhecidos:** `reflect` tem custo de performance — evitar em hot paths.

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
| Usar `reflect` para despachar sobre um conjunto fechado e conhecido de tipos | Perde checagem em tempo de compilação e é mais lento que type switch | Usar type switch (`switch v := x.(type)`) ou generics quando o conjunto de tipos é conhecido |
| Ignorar `Kind()` antes de chamar métodos específicos (`Int()`, `String()`, `Float()`) | Causa `panic` em runtime quando o valor não é do Kind esperado | Verificar `Kind()` (ou `CanInt()`/`CanConvert()`) antes de extrair o valor |
| Usar reflection em loop de alta frequência (hot path de request) sem medir custo | Reflection é 10-100x mais lento que acesso direto; degrada throughput sob carga | Fazer cache do `reflect.Type`/`reflect.Value` fora do loop, ou substituir por geração de código (`go generate`) |
| Modificar valor via reflection sem checar `CanSet()` | Gera panic "reflect: reflect.Value.Set using unaddressable value" | Sempre obter o valor via ponteiro (`reflect.ValueOf(&x).Elem()`) e checar `CanSet()` |

## Avaliação de risco

- **Parar e confirmar quando:** a introdução de reflection substitui um código antes type-safe
  (verificado em compile-time) por um caminho que só falha em runtime
- **Risco alto:** reflection em hot path (serialização por request, roteamento de mensagens) sem
  benchmark comprovando custo aceitável
- **Risco baixo:** uso pontual de reflection em código de inicialização/bootstrap (registro de tipos,
  parsing de configuração) executado poucas vezes

## Métricas de sucesso

- Zero `panic` em runtime originado por `Kind()` incompatível nos testes
- 100% das escritas via reflection precedidas de checagem `CanSet()`
- Benchmark (`go test -bench=. -benchmem`) registrado para todo uso de reflection em hot path

## Responsável principal

| Papel | Quem |
|-------|------|
| Agent executor | `developer-golang-agent-orchestrator` |
| Revisão humana | Desenvolvedor / Tech Lead |
| Aprovação final | Tech Lead |

## Referências

- [pkg.go.dev/reflect](https://pkg.go.dev/reflect)
- [The Laws of Reflection — Go Blog](https://go.dev/blog/laws-of-reflection)
- Skill relacionada: `developer-go-language-generics`

---

## Versão interna (arquivo)

| Campo | Valor |
|-------|-------|
| **FileVersion** | 1.0.0 |
| **Política** | `.cursor/VERSION.md` |

## Changelog (este arquivo)

- 1.0.0 (09/08/2026): Criação da skill — cobertura inicial de `reflect.TypeOf`/`ValueOf`, leitura de
  struct tags, tipagem dinâmica e critérios para evitar reflection em favor de generics/interfaces.

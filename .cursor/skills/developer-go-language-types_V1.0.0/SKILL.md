---
name: developer-go-language-types
description: Ensina a modelar tipos idiomáticos em Go — structs, interfaces implícitas, zero values, conversão de tipo, type assertion/switch e a escolha entre ponteiro e valor — para quem implementa ou revisa pacotes Go.
model: sonnet
thinking: normal
category: developer-go
license: MIT
copyright: "Copyright (c) 2026 CSL Tech Solutions"
company: "CSL Tech Solutions"
author: "Claiton de Souza Linhares"
---

# Tipos em Go

## Responsabilidade única

Esta skill cobre a modelagem de dados e comportamento no sistema de tipos de Go: como declarar
structs, como interfaces são satisfeitas implicitamente (sem `implements`), o que são zero values
e por que eliminam a necessidade de construtores obrigatórios, como converter explicitamente entre
tipos compatíveis, como inspecionar um valor de interface via type assertion e type switch, e quando
escolher receiver por ponteiro ou por valor. Ela existe separada de generics (parametrização de
tipos) e de reflection (inspeção em runtime via `reflect`) porque trata do sistema de tipos estático
e das convenções idiomáticas do dia a dia — a base sobre a qual generics e reflection são construídos.

## When to use

- Declarar structs para representar entidades ou DTOs do domínio
- Definir um contrato de comportamento (interface) e verificar que um tipo o satisfaz implicitamente
- Decidir entre usar o zero value de uma struct ou exigir um construtor `New*`
- Converter entre tipos numéricos, `string`/`[]byte`, ou tipos definidos (`type Status int`)
- Inspecionar dinamicamente um valor `any`/`interface{}` via type assertion ou type switch
- Decidir se um método deve ter receiver `T` ou `*T`

## When NOT to use

- Parametrização com type parameters (`[T any]`, constraints) → `developer-go-language-generics`
- Inspeção via pacote `reflect`, tags de struct lidas em runtime → `developer-go-stdlib-rtti-reflection`
- Fundamentos de sintaxe, pacotes e `go.mod` → `developer-go-language-core`
- Padrões de erro (`errors.Is/As`, wrapping) → `developer-go-error-handling-and-diagnostics`

## Inputs obrigatórios

| Input | Tipo | Descrição |
|-------|------|-----------|
| Domínio de dados a modelar | Especificação/requisito | Entidades, DTOs ou contratos que precisam de representação em Go |
| Código Go existente (se houver) | `.go` | Arquivo(s) alvo da edição ou revisão de tipos |
| `go.mod` do módulo | Arquivo | Define a versão mínima de Go disponível (afeta o que pode ser usado) |

## Dependências (skills prévias)

| Skill | Quando executar antes |
|-------|-----------------------|
| `developer-go-language-core` | Garantir fundamentos de sintaxe, pacotes e `go.mod` antes de modelar tipos |

## Workflow executável

1. Declarar a struct com campos exportados apenas quando fizerem parte do contrato público do pacote.
   ```go
   type Customer struct {
       ID    int64
       Name  string
       Email string
   }
   ```
2. Definir o contrato via interface pequena e satisfazê-la implicitamente (sem `implements`).
   ```go
   type Notifier interface {
       Notify(msg string) error
   }
   type EmailSender struct{ Host string }
   func (e EmailSender) Notify(msg string) error { return nil }
   // EmailSender já satisfaz Notifier — nenhuma declaração extra é necessária.
   ```
3. Preferir o zero value utilizável em vez de exigir construtor; usar `New*` só quando há trabalho de inicialização real.
   ```go
   var c Customer      // zero value: ID=0, Name="", Email=""
   var buf bytes.Buffer // zero value já pronto para uso
   ```
4. Converter entre tipos compatíveis com sintaxe explícita `T(v)`; Go nunca converte implicitamente.
   ```go
   var i int32 = 42
   f := float64(i)
   b := []byte("texto")
   ```
5. Usar type assertion na forma de dois retornos para evitar panic quando o tipo pode divergir.
   ```go
   var v any = "hello"
   s, ok := v.(string)
   if !ok {
       // tratar tipo inesperado sem interromper o processo
   }
   ```
6. Usar type switch quando o mesmo valor puder assumir vários tipos possíveis.
   ```go
   switch t := v.(type) {
   case string:
       _ = t
   case int:
       _ = t
   default:
       // tipo não tratado explicitamente
   }
   ```
7. Escolher receiver por ponteiro (`*T`) quando o método muta estado ou a struct é grande; por valor (`T`) quando o tipo é pequeno e imutável — mantendo o mesmo receiver em todos os métodos.
   ```go
   func (c *Customer) Rename(name string) { c.Name = name }
   func (c Customer) String() string      { return c.Name }
   ```

## Outputs obrigatórios

| Output | Localização | Formato |
|--------|-------------|---------|
| Tipos (structs/interfaces) definidos | Pacote Go alvo (`*.go`) | Go |
| Testes de asserção de tipo (quando aplicável) | `*_test.go` no mesmo pacote | Go |

## Checklist de validação

- [ ] Cada struct exportada tem comentário de doc começando pelo nome do tipo
- [ ] Interfaces são pequenas (idealmente 1-3 métodos) e definidas do lado do consumidor
- [ ] Nenhuma conversão de tipo implícita — todas usam `T(v)` explícito
- [ ] Toda type assertion fora de type switch usa a forma de dois retornos (`v, ok := x.(T)`)
- [ ] Receiver (ponteiro vs valor) é consistente entre todos os métodos do mesmo tipo

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
| Interface grande definida do lado do produtor (`Get`, `Save`, `Delete`, `List`, `Count`...) | Força consumidores a depender de métodos que não usam; dificulta mocks e testes | Definir interfaces pequenas do lado do consumidor, só com os métodos realmente usados |
| Usar ponteiro para tudo "por garantia" mesmo em tipos pequenos e imutáveis | Gera pressão desnecessária no GC e escapa a variável para a heap sem necessidade | Usar valor quando o tipo é pequeno/imutável; ponteiro só quando há mutação ou struct grande |
| Type assertion de uma via (`s := v.(string)`) fora de contexto controlado | Panic em runtime se o tipo não bater, derrubando o processo | Usar a forma de dois retornos (`s, ok := v.(string)`) e tratar `ok == false` |
| Misturar receiver por valor e por ponteiro entre métodos do mesmo tipo | Comportamento inconsistente e pode quebrar a satisfação implícita de uma interface com receiver ponteiro | Escolher um único tipo de receiver por tipo e aplicá-lo a todos os métodos |

## Avaliação de risco

- **Parar e confirmar quando:** a mudança de tipo (struct → interface, valor → ponteiro) afeta a assinatura pública de um pacote já consumido por outros módulos
- **Risco alto:** alterar o receiver de valor para ponteiro (ou vice-versa) em tipo já usado como valor de interface — pode quebrar a satisfação implícita do contrato em tempo de compilação
- **Risco baixo:** adicionar campo novo em struct interna não exportada, ou criar interface nova e pequena sem alterar tipos existentes

## Métricas de sucesso

- Zero conversões implícitas de tipo no código revisado (`go vet` limpo)
- 100% das type assertions fora de type switch usam a forma de dois retornos
- Receiver consistente (ponteiro ou valor) em 100% dos métodos de cada tipo

## Responsável principal

| Papel | Quem |
|-------|------|
| Agent executor | `developer-golang-agent-orchestrator` |
| Revisão humana | Desenvolvedor / Tech Lead |
| Aprovação final | Tech Lead |

## Referências

- [Go Language Specification — Types](https://go.dev/ref/spec#Types)
- [Effective Go — Interfaces and other types](https://go.dev/doc/effective_go#interfaces_and_types)
- Skill relacionada: `developer-go-master-orchestrator`

---

## Versão interna (arquivo)

| Campo | Valor |
|-------|-------|
| **FileVersion** | 1.0.0 |
| **Política** | `.cursor/VERSION.md` |

## Changelog (este arquivo)

- 1.0.0 (09/08/2026): Criação da skill — cobertura inicial de structs, interfaces implícitas, zero
  values, conversão de tipo, type assertion/switch e escolha entre ponteiro e valor.

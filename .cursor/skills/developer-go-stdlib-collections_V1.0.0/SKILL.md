---
name: developer-go-stdlib-collections
description: "Coleções na stdlib Go — slices, maps, sort, pacotes slices/maps (1.21+), container/list e container/heap; uso idiomático e seguro."
model: sonnet
thinking: normal
category: developer-go
license: MIT
copyright: "Copyright (c) 2026 CSL Tech Solutions"
company: "CSL Tech Solutions"
author: "Claiton de Souza Linhares"
---

# Coleções na Stdlib Go

## Responsabilidade única

Esta skill cobre o uso idiomático das estruturas de coleção da stdlib Go: slices (capacidade,
`append`, cópia), maps (comma-ok idiom, mapa nil), o pacote `sort`, os pacotes genéricos
`slices`/`maps` (Go 1.21+), `container/list` (lista duplamente encadeada) e `container/heap`
(heap binário via interface). Ela existe separada de `developer-go-language-core` porque
manipular coleções corretamente exige regras próprias — semântica de referência de slices,
aliasing, nil map panic, invariantes de heap — que vão além da sintaxe fundamental da
linguagem. Não cobre generics em si (ver `developer-go-language-generics`) nem estruturas de
dados concorrentes (`sync.Map`).

## When to use

- Manipular slices com atenção a capacidade, `append` e compartilhamento de array subjacente
- Usar maps com o idiom comma-ok (`v, ok := m[k]`) e lidar com mapas `nil`
- Ordenar coleções com `sort.Slice`/`sort.Sort` ou com o pacote genérico `slices` (Go 1.21+)
- Migrar código legado (`golang.org/x/exp/slices`/`maps`) para a stdlib `slices`/`maps`
- Implementar fila/pilha com `container/list` ou fila de prioridade com `container/heap`

## When NOT to use

- Sintaxe fundamental (variáveis, control flow, funções) → `developer-go-language-core`
- Definir constraints e tipos genéricos próprios → `developer-go-language-generics`
- Strings, `bufio`, `io.Reader`/`io.Writer` → `developer-go-stdlib-strings-io`
- Coleções seguras para concorrência (`sync.Map`, mutex sobre slice/map) → `developer-go-concurrency-basics`
- Triagem entre múltiplas skills da família → `developer-go-master-orchestrator`

## Inputs obrigatórios

| Input | Tipo | Descrição |
|-------|------|-----------|
| Código-fonte Go alvo | Arquivo(s) `.go` | Pacote ou arquivo existente a criar/revisar |
| Versão do Go declarada | string (`go.mod`) | Determina se `slices`/`maps` da stdlib estão disponíveis (≥1.21) |
| Tipo de coleção necessária | texto livre | slice, map, lista, heap — orienta qual API usar |

## Dependências (skills prévias)

| Skill | Quando executar antes |
|-------|-----------------------|
| `developer-go-language-core` | Fundamentos de sintaxe, funções e packages são pré-requisito para usar coleções corretamente |

## Workflow executável

1. Para slices, entender que `append` pode ou não realocar — nunca assumir que duas variáveis
   apontam para o mesmo array após um `append` que excedeu a capacidade:

```go
s := make([]int, 0, 2)
s = append(s, 1, 2)
s2 := append(s, 3) // pode realocar: s e s2 podem divergir
```

2. Para maps, usar sempre o comma-ok idiom para distinguir "ausente" de "valor zero", e
   inicializar com `make` antes de escrever (mapa `nil` só permite leitura):

```go
m := make(map[string]int)
if v, ok := m["chave"]; ok {
    fmt.Println(v)
}
```

3. Ordenar com o pacote genérico `slices` (Go 1.21+) em vez de `sort.Slice` quando possível
   (type-safe, sem reflection):

```go
import "slices"

nums := []int{3, 1, 2}
slices.Sort(nums)
idx, found := slices.BinarySearch(nums, 2)
```

4. Usar `maps.Keys`/`maps.Clone`/`maps.Equal` (Go 1.21+) para operações comuns sobre mapas
   em vez de laços manuais repetidos.
5. Para filas/pilhas com remoção em O(1) no meio, usar `container/list.List`; para fila de
   prioridade, implementar a interface `heap.Interface` sobre `container/heap`.
6. Validar com `gofmt`, `go vet` e `go build`/`go test` antes de considerar concluído.

> Snippets acima ≤ 15 linhas cada. Exemplo combinado maior → `./exemplos/`.

## Outputs obrigatórios

| Output | Localização | Formato |
|--------|-------------|---------|
| Código Go novo/revisado usando coleções | caminho do pacote no projeto (ex.: `./internal/<pacote>/*.go`) | Go |
| `go.mod` com versão mínima compatível | raiz do módulo | Go modules |
| Evidência de validação (gofmt/vet/build/test) | log anexado ao commit/PR | texto |

## Checklist de validação

- [ ] Nenhum `append` assume implicitamente que o slice original não foi realocado
- [ ] Todo acesso a map usa comma-ok quando a ausência da chave é um caso válido
- [ ] Nenhuma escrita em map `nil` (sempre inicializado via `make` ou literal)
- [ ] Ordenação usa `slices.Sort`/`slices.SortFunc` (1.21+) em vez de `sort.Slice` quando o projeto já suporta 1.21
- [ ] Implementações de `heap.Interface` mantêm `Len`, `Less`, `Swap`, `Push`, `Pop` consistentes

---

## Stack e versões  ← OBRIGATÓRIO (Go)

| Componente | Versão mínima | Notas |
|------------|:---:|-------|
| Go | 1.21 | pacotes `slices`/`maps` na stdlib desde 1.21 |
| gofmt | embutido | Formatação obrigatória, sem exceções |
| go vet | embutido | Rodar antes de qualquer commit |
| golangci-lint | 1.55+ | Lint agregador (opcional mas recomendado) |

## Dependências (go.mod / go get)  ← OBRIGATÓRIO (Go)

```bash
go mod init <module-path>
go get <pacote>@<versão>
go mod tidy
```

**Conflitos conhecidos:** antes do Go 1.21, use `golang.org/x/exp/slices` e `golang.org/x/exp/maps`.

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
| Escrever em map `nil` (`var m map[string]int; m["x"] = 1`) | Causa panic em runtime (`assignment to entry in nil map`) | Inicializar sempre com `make(map[K]V)` ou literal `map[K]V{}` antes de escrever |
| Comparar slices com `==` | Slices não são comparáveis (exceto contra `nil`); nem compila para `slice == slice` | Usar `slices.Equal(a, b)` (Go 1.21+) ou laço manual |
| Assumir que `append` sempre modifica o array original in-place | Quando excede a capacidade, `append` realoca; variáveis antigas ficam desatualizadas | Sempre reatribuir o retorno de `append` (`s = append(s, x)`) e não presumir aliasing |
| Iterar um map contando com ordem estável | Go randomiza a ordem de iteração de maps deliberadamente | Ordenar as chaves explicitamente (`slices.Sort(maps.Keys(...))`) quando ordem importa |
| Compartilhar sub-slice de um slice grande sem copiar, retendo o array inteiro na memória | Vazamento de memória: o array subjacente inteiro não é coletado enquanto o sub-slice existir | Copiar explicitamente com `slices.Clone`/`copy` quando o sub-slice sobreviver muito além do array original |

## Avaliação de risco

- **Parar e confirmar quando:** a migração de `golang.org/x/exp/slices`/`maps` para a stdlib
  `slices`/`maps` exigir subir a versão mínima do Go no `go.mod` (pode quebrar builds de CI)
- **Risco alto:** remover uma cópia defensiva de slice/map "para simplificar" — pode introduzir
  aliasing indevido e corrupção de dados compartilhados entre goroutines ou chamadas
- **Risco baixo:** trocar `sort.Slice` por `slices.SortFunc` equivalente, ajustar formatação

## Métricas de sucesso

- `gofmt -l .` retorna vazio e `go vet ./...`/`go build ./...` sem erros
- Zero panics de "assignment to entry in nil map" em testes e produção
- Zero comparações inválidas de slice com `==` (detectável em `go vet`/compilação)
- Cobertura mantida ou aumentada em `go test ./... -cover` para pacotes que manipulam coleções

## Responsável principal

| Papel | Quem |
|-------|------|
| Agent executor | `developer-golang-agent-orchestrator` |
| Revisão humana | Desenvolvedor / Tech Lead |
| Aprovação final | Tech Lead |

## Referências

- [pkg.go.dev/slices](https://pkg.go.dev/slices)
- [pkg.go.dev/maps](https://pkg.go.dev/maps)
- Skill relacionada: `developer-go-stdlib-strings-io`

---

## Versão interna (arquivo)

| Campo | Valor |
|-------|-------|
| **FileVersion** | 1.0.0 |
| **Política** | `.cursor/VERSION.md` |

## Changelog (este arquivo)

- 1.0.0 (09/08/2026): Criação da skill — coleções da stdlib Go (slices, maps, sort, pacotes
  genéricos `slices`/`maps` 1.21+, `container/list`, `container/heap`).

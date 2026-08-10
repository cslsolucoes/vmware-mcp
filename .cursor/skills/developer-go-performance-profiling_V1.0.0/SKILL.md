---
name: developer-go-performance-profiling
description: Profiling e benchmarking em Go — pprof (cpu/mem/goroutine), testing.B (b.ResetTimer, -benchmem) e go tool trace para medir antes de otimizar.
model: sonnet
thinking: normal
category: developer-go
license: MIT
copyright: "Copyright (c) 2026 CSL Tech Solutions"
company: "CSL Tech Solutions"
author: "Claiton de Souza Linhares"
---

# Profiling em Go

## Responsabilidade única

Esta skill cobre a medição empírica de desempenho em Go usando exclusivamente ferramentas da
stdlib: `net/http/pprof` e `runtime/pprof` (profiles de CPU, memória e goroutine), `testing.B`
(benchmarks com `b.ResetTimer`/`b.StopTimer` e a flag `-benchmem`) e `go tool trace` (linha do
tempo de scheduler, GC e goroutines). Ela existe separada da skill de memória porque trata da
**coleta e leitura de evidência de desempenho** — decidir o que fazer com essa evidência (reduzir
alocações, trocar estrutura de dados, revisar pool de objetos) é responsabilidade de outra skill.

## When to use

- Confirmar, com um benchmark (`go test -bench`), se uma mudança de código realmente é mais rápida
  antes de declarar uma otimização concluída
- Gerar e ler um profile de CPU (`pprof.StartCPUProfile`) para localizar a função que consome mais
  tempo de execução (hotspot)
- Gerar e ler um profile de memória (heap) para localizar o ponto de maior alocação
- Investigar vazamento ou crescimento de goroutines com o profile `goroutine`
- Expor `net/http/pprof` em um serviço HTTP para coletar profiles ao vivo em produção/staging
- Analisar contenção de scheduler, GC e paralelismo com `go tool trace`

## When NOT to use

- Otimizar alocação de memória (escolher struct vs. ponteiro, reduzir escapes) sem antes medir com
  benchmark/profile → rodar primeiro `developer-go-performance-and-memory`
- Escrever o teste funcional que valida o comportamento da função (não o desempenho dela) →
  `developer-go-testing`
- Decidir arquitetura de concorrência (worker pool, fan-out/fan-in) antes de existir um gargalo
  medido → não crie complexidade especulativa; primeiro confirme o hotspot
- Ajustar índices/queries de banco de dados — fora do escopo desta skill (perfila apenas o processo
  Go, não o servidor de banco)

## Inputs obrigatórios

| Input | Tipo | Descrição |
|-------|------|-----------|
| Função ou pacote alvo | `.go` | Código já compilável e com comportamento correto (testado) |
| Benchmark existente ou a criar | `*_test.go` | `func BenchmarkX(b *testing.B)` cobrindo o caminho quente |
| Critério de comparação | Baseline vs. candidato | Dois binários/commits a comparar sob a mesma carga |
| `go.mod` do módulo | Arquivo | Confirma a versão mínima de Go disponível |

## Dependências (skills prévias)

| Skill | Quando executar antes |
|-------|-----------------------|
| `developer-go-performance-and-memory` | Entender o modelo de alocação/GC do candidato antes de interpretar um profile de heap |

## Workflow executável

1. Escrever (ou localizar) o benchmark do caminho quente, sempre resetando o timer após o setup.
   ```go
   func BenchmarkParse(b *testing.B) {
       data := loadFixture() // setup fora da medição
       b.ResetTimer()
       for i := 0; i < b.N; i++ {
           Parse(data)
       }
   }
   ```
2. Rodar o baseline com alocações contadas e salvar o resultado para comparação posterior.
   ```bash
   go test -run=^$ -bench=BenchmarkParse -benchmem -count=5 ./... > old.txt
   ```
3. Aplicar a mudança candidata e repetir a mesma coleta, depois comparar com `benchstat`.
   ```bash
   go test -run=^$ -bench=BenchmarkParse -benchmem -count=5 ./... > new.txt
   benchstat old.txt new.txt
   ```
4. Se o hotspot não estiver óbvio no benchmark, gerar profile de CPU e abrir com `pprof`.
   ```bash
   go test -bench=BenchmarkParse -cpuprofile=cpu.prof ./...
   go tool pprof -top cpu.prof
   ```
5. Para alocação, gerar profile de memória e ler `-alloc_objects`/`-alloc_space`.
   ```bash
   go test -bench=BenchmarkParse -memprofile=mem.prof ./...
   go tool pprof -alloc_space -top mem.prof
   ```
6. Em serviço rodando (HTTP), expor os profiles ao vivo e coletar sob carga real.
   ```go
   import _ "net/http/pprof"
   // go func() { log.Println(http.ListenAndServe("localhost:6060", nil)) }()
   ```
7. Para investigar contenção de scheduler/GC, capturar e visualizar o trace.
   ```bash
   go test -bench=BenchmarkParse -trace=trace.out ./...
   go tool trace trace.out
   ```

## Outputs obrigatórios

| Output | Localização | Formato |
|--------|-------------|---------|
| Benchmark(s) do caminho medido | `*_test.go` no pacote alvo | Go |
| Resultado baseline vs. candidato | Arquivo texto (`old.txt`/`new.txt`) ou saída de `benchstat` | Texto |
| Profile coletado (cpu/mem/goroutine) | `*.prof` (não versionado) | pprof binário |
| Trace coletado (quando aplicável) | `trace.out` (não versionado) | trace binário |

## Checklist de validação

- [ ] Benchmark chama `b.ResetTimer()` após qualquer setup custoso (carga de fixture, alocação inicial)
- [ ] Coleta rodada com `-benchmem` e `-count>=5` para permitir `benchstat` confiável
- [ ] Baseline e candidato medidos na mesma máquina, sem outros processos pesados concorrentes
- [ ] Profile de CPU/memória interpretado com `go tool pprof -top` (ou `-web`) antes de qualquer conclusão
- [ ] Nenhuma alteração de código aplicada apenas por "parecer mais rápida" sem número que comprove

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

**Conflitos conhecidos:** nenhum conhecido para este tema (pprof e benchmarks são stdlib pura).

## Checklist Go  ← OBRIGATÓRIO (Go)

- [ ] `gofmt -l .` sem saída (código formatado)
- [ ] `go vet ./...` sem avisos
- [ ] `go build ./...` sem erros
- [ ] `go test -bench=. -benchmem ./...` executado antes/depois da otimização
- [ ] Erros tratados explicitamente (`if err != nil`), nunca descartados com `_` sem justificativa
- [ ] Nenhuma otimização aplicada sem medição prévia (benchmark ou profile)

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
| Otimizar "no olho" sem rodar benchmark antes/depois | A mudança pode piorar o desempenho real; a percepção de "mais rápido" não é evidência | Medir baseline com `-bench -benchmem`, aplicar a mudança, medir de novo e comparar com `benchstat` |
| Benchmark sem `b.ResetTimer()` após setup custoso | O custo de preparar fixtures entra na medição e distorce o resultado por iteração | Chamar `b.ResetTimer()` (e `b.StopTimer()`/`b.StartTimer()` quando houver setup dentro do loop) |
| Comparar profiles/benchmarks de builds ou máquinas diferentes | Ruído de hardware/otimização do compilador mascara o efeito real da mudança | Rodar baseline e candidato na mesma máquina, mesma flag de build, e usar `benchstat` para significância |
| Ler apenas o profile de CPU e ignorar o de memória (ou vice-versa) | Um gargalo pode ser dominado por GC/alocação e não aparecer no top de CPU isoladamente | Coletar `-cpuprofile` e `-memprofile` juntos quando o sintoma não for óbvio |
| Deixar `net/http/pprof` exposto sem proteção em endpoint público de produção | Vaza informação interna do processo e pode ser usado para negação de serviço | Expor `pprof` apenas em porta/rede interna, atrás de autenticação, ou só sob demanda |

## Avaliação de risco

- **Parar e confirmar quando:** o profile indicar que o hotspot está em código de terceiros/stdlib
  (mudança exigiria contornar a dependência, não apenas o código do módulo)
- **Risco alto:** aplicar uma otimização que aumenta complexidade (cache manual, pool de objetos)
  sem um `benchstat` mostrando ganho estatisticamente significativo (>5% de delta médio)
- **Risco baixo:** adicionar um benchmark novo isolado, ou coletar um profile local sem alterar
  nenhum código de produção

## Métricas de sucesso

- Todo hotspot reportado é confirmado por `go tool pprof -top` (ou `-web`), não por suposição
- `benchstat` mostra delta estatisticamente significativo entre baseline e candidato antes de a
  otimização ser aceita
- Zero regressão de alocações (`-benchmem`) introduzida por uma mudança apresentada como "otimização"

## Responsável principal

| Papel | Quem |
|-------|------|
| Agent executor | `developer-golang-agent-orchestrator` |
| Revisão humana | Desenvolvedor / Tech Lead |
| Aprovação final | Tech Lead |

## Referências

- [pkg.go.dev — net/http/pprof](https://pkg.go.dev/net/http/pprof)
- [go.dev/blog — Profiling Go Programs](https://go.dev/blog/pprof)
- Skill relacionada: `developer-go-testing`

---

## Versão interna (arquivo)

| Campo | Valor |
|-------|-------|
| **FileVersion** | 1.0.0 |
| **Política** | `.cursor/VERSION.md` |

## Changelog (este arquivo)

- 1.0.0 (09/08/2026): Criação da skill — cobertura inicial de `pprof` (cpu/mem/goroutine),
  `testing.B` (benchmarks, `b.ResetTimer`, `-benchmem`) e `go tool trace`.

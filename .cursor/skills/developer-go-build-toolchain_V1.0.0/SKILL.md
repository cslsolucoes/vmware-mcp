---
name: developer-go-build-toolchain
description: "Compilação e toolchain Go — go build/run/install, go.mod (init/tidy/why/graph), cross-compile via GOOS/GOARCH, -ldflags e build constraints; referência para quem precisa gerar binários Go a partir do código-fonte."
model: sonnet
thinking: normal
category: developer-go
license: MIT
copyright: "Copyright (c) 2026 CSL Tech Solutions"
company: "CSL Tech Solutions"
author: "Claiton de Souza Linhares"
---

# Build e Toolchain Go

## Responsabilidade única

Esta skill cobre a compilação de projetos Go: os comandos `go build`, `go run` e
`go install`, o ciclo de vida do módulo (`go mod init`/`tidy`/`why`/`graph`), cross-compile
para outro sistema operacional/arquitetura via `GOOS`/`GOARCH`, injeção de metadados de
build com `-ldflags` e compilação condicional com `//go:build`. Ela existe separada de
`developer-go-language-core` porque aquela cobre sintaxe e organização do código-fonte,
enquanto esta cobre exclusivamente a transformação desse código-fonte em binário/artefato
executável. Não cobre empacotamento final, distribuição, instaladores nem pipelines de release.

## When to use

- Compilar um projeto Go localmente (`go build`, `go run`, `go install`)
- Inicializar, atualizar ou depurar dependências de `go.mod`/`go.sum`
- Gerar binário para outro sistema operacional/arquitetura (cross-compile)
- Embutir versão, commit hash ou data de build no binário via `-ldflags`
- Isolar código específico de plataforma com build constraints (`//go:build`)

## When NOT to use

- Empacotamento final, instaladores, distribuição e releases → usar `developer-go-packaging-delivery`
- Sintaxe fundamental da linguagem (variáveis, control flow, funções, packages) → usar `developer-go-language-core`
- Testes automatizados (`go test`, tabelas de teste, cobertura) → usar a skill de testes/qualidade da família `developer-go-*`
- Concorrência (goroutines, channels, `sync`) → usar `developer-go-concurrency-basics`

## Inputs obrigatórios

| Input | Tipo | Descrição |
|-------|------|-----------|
| Caminho do módulo/pacote | string | Diretório contendo `go.mod` ou pacote alvo (ex.: `./cmd/app`) |
| Sistema/arquitetura alvo | string | `GOOS`/`GOARCH` desejados; padrão = plataforma atual se não informado |
| Metadados de build | texto livre (opcional) | Versão, commit hash, data — quando exigidos via `-ldflags` |

## Dependências (skills prévias)

| Skill | Quando executar antes |
|-------|-----------------------|
| `developer-go-language-core` | Antes de compilar, garantir que o código está sintaticamente correto e o `go.mod` reflete a versão mínima real |

## Workflow executável

1. Confirmar que o módulo existe; se não, inicializar:

```bash
go mod init <module-path>
go mod tidy
```

2. Compilar localmente para desenvolvimento rápido ou gerar o binário:

```bash
go run ./cmd/app        # compila e executa sem gerar binário persistente
go build -o bin/app ./cmd/app
go install ./cmd/app    # compila e instala em $GOBIN/$GOPATH/bin
```

3. Investigar dependências quando `go.sum` mudar de forma inesperada:

```bash
go mod why <pacote>
go mod graph | grep <pacote>
```

4. Cross-compile definindo `GOOS`/`GOARCH` (build não exige o SO alvo instalado):

```bash
GOOS=linux   GOARCH=amd64 go build -o bin/app-linux-amd64   ./cmd/app
GOOS=windows GOARCH=amd64 go build -o bin/app-windows.exe   ./cmd/app
GOOS=darwin  GOARCH=arm64 go build -o bin/app-darwin-arm64  ./cmd/app
```

5. Injetar metadados de versão em tempo de build com `-ldflags` (variável deve ser `var`
   exportada no pacote `main`, nunca `const`):

```bash
go build -ldflags "-X main.Version=1.2.3 -X main.Commit=$(git rev-parse --short HEAD)" \
  -o bin/app ./cmd/app
```

6. Isolar código específico de plataforma com build constraints (arquivo `net_windows.go`
   só compila em Windows; `net_unix.go` só fora dele):

```go
//go:build windows

package net
```

> Snippets acima ≤ 15 linhas cada. Exemplo combinado maior → `./exemplos/README.md`.

## Outputs obrigatórios

| Output | Localização | Formato |
|--------|-------------|---------|
| Binário compilado | `bin/<nome>` ou caminho indicado em `-o` | executável nativo da plataforma alvo |
| `go.mod`/`go.sum` atualizados | raiz do módulo | Go modules |
| Evidência de validação (build/vet/test) | log anexado ao commit/PR | texto |

## Checklist de validação

- [ ] `go build ./...` conclui sem erros na plataforma de desenvolvimento
- [ ] Cross-compile testado ao menos uma vez na plataforma alvo real antes de distribuir
- [ ] `go.sum` versionado e consistente com `go.mod` (`go mod tidy` sem alterações pendentes)
- [ ] Metadados injetados via `-ldflags` conferidos no binário final (`app --version`)
- [ ] Build constraints (`//go:build`) documentadas quando isolam código por plataforma

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
- [ ] `go.sum` versionado no controle de código (nunca em `.gitignore`)

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
| Commitar `go.sum` em `.gitignore` | Quebra builds reprodutíveis; cada máquina pode resolver versões diferentes das dependências | Sempre versionar `go.mod` e `go.sum` juntos no controle de código |
| `go.mod` declarar versão de Go desatualizada em relação ao pipeline de CI | CI usa uma toolchain diferente da testada localmente; features novas falham silenciosamente ou build quebra só no CI | Alinhar `go <versão>` no `go.mod` com a versão fixada no workflow de CI |
| Cross-compilar (`GOOS`/`GOARCH`) e distribuir sem nunca testar no SO alvo | Diferenças de syscalls, paths e permissões só aparecem em runtime na máquina real | Rodar ao menos um smoke test do binário cross-compilado na plataforma alvo antes de liberar |
| Usar `const` para valores que `-ldflags -X` deveria injetar | `-X` só sobrescreve `var` de string no pacote `main`; `const` é resolvida em tempo de compilação e o flag falha silenciosamente | Declarar a variável de versão/commit como `var` exportada em `main` |
| Ignorar `go vet`/`gofmt` antes do commit | Deixa passar erros comuns (printf mal formatado, código desformatado) que quebram revisão e CI | Rodar `gofmt -l .` e `go vet ./...` como gate local antes de qualquer commit |

## Avaliação de risco

- **Parar e confirmar quando:** a mudança exigir subir a versão mínima do Go no `go.mod`
  (pode quebrar builds de CI ou de consumidores externos do módulo)
- **Risco alto:** distribuir um binário cross-compilado sem testá-lo na plataforma alvo real —
  falhas de runtime só aparecem em produção
- **Risco baixo:** ajustar flags de build local (`-o`, `-v`), reorganizar `go.mod` com `go mod tidy`

## Métricas de sucesso

- `go build ./...` e `go vet ./...` sem erros/avisos em todas as plataformas alvo declaradas
- 100% dos binários cross-compilados validados com smoke test na plataforma real antes da entrega
- `go.sum` sempre consistente com `go.mod` (`go mod tidy` não gera diff)
- Metadados de versão (`-ldflags -X`) presentes e corretos em todo binário de release

## Responsável principal

| Papel | Quem |
|-------|------|
| Agent executor | `developer-golang-agent-orchestrator` |
| Revisão humana | Desenvolvedor / Tech Lead |
| Aprovação final | Tech Lead |

## Referências

- [Organizing a Go module](https://go.dev/doc/modules/layout)
- [Tutorial: Create a Go module](https://go.dev/doc/tutorial/create-module)
- Skill relacionada: `developer-go-packaging-delivery`

---

## Versão interna (arquivo)

| Campo | Valor |
|-------|-------|
| **FileVersion** | 1.0.0 |
| **Política** | `.cursor/VERSION.md` |

## Changelog (este arquivo)

- 1.0.0 (09/08/2026): Criação da skill — build e toolchain Go (go build/run/install, go.mod
  init/tidy/why/graph, cross-compile GOOS/GOARCH, -ldflags, build constraints), família `developer-go-*`.

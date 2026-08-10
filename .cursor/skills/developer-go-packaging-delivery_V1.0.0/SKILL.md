---
name: developer-go-packaging-delivery
description: "Empacotamento, versionamento semântico e entrega de binários/módulos Go — ldflags, checksums e estratégia de release multiplataforma."
model: sonnet
thinking: normal
category: developer-go
license: MIT
copyright: "Copyright (c) 2026 CSL Tech Solutions"
company: "CSL Tech Solutions"
author: "Claiton de Souza Linhares"
---

# Empacotamento e Entrega Go

## Responsabilidade única

Esta skill cobre o ciclo de empacotamento e entrega de artefatos Go: injeção de versão em
binários via `-ldflags`, versionamento semântico de módulos publicados (tags `vX.Y.Z`, sufixo
de major version para `v2+`), validação de checksums (`go.sum`, `GOSUMDB`/`GONOSUMCHECK`) e
estratégia de release multiplataforma (matriz `GOOS`/`GOARCH` + tag git + notas de release).
Ela existe separada das demais skills `developer-go-*` porque a entrega final é uma etapa
distinta da escrita/compilação do código — pressupõe build já limpo e testes já aprovados.

## When to use

- Preparar uma release de binário Go com versão, commit e data injetados via `-ldflags`.
- Publicar ou atualizar um módulo Go (tag `vX.Y.Z`), incluindo transição para major `v2+`.
- Gerar checksums de artefatos e validar `go.sum`/`GOSUMDB` antes de distribuir.
- Montar matriz de build multiplataforma (`GOOS`/`GOARCH`) para uma release.

## When NOT to use

- Compilação básica, cross-compile local ou configuração de toolchain → usar
  `developer-go-build-toolchain`.
- Modelagem de linguagem, sintaxe ou módulos (`go.mod` inicial) → usar `developer-go-language-core`.
- Estratégia e execução de testes automatizados → usar a skill de testes/qualidade Go.
- Concorrência, goroutines e channels → usar `developer-go-concurrency-basics`/`-advanced`.

## Inputs obrigatórios

| Input | Tipo | Descrição |
|-------|------|-----------|
| Versão alvo da release | string semver (`vX.Y.Z`) | Definida a partir do changelog e do impacto (major/minor/patch) |
| Build validado | saída de `go build`/`go test` | Zero erros, zero avisos de `go vet`, testes verdes |
| Plataformas alvo | lista `GOOS/GOARCH` | Ex.: `linux/amd64`, `windows/amd64`, `darwin/arm64` |
| Module path publicado | string (`go.mod`) | Necessário para decidir sufixo `/v2`, `/v3` em major releases |

## Dependências (skills prévias)

| Skill | Quando executar antes |
|-------|-----------------------|
| `developer-go-build-toolchain` | Build e cross-compile devem estar limpos (zero erros/avisos) antes do empacotamento |

## Workflow executável

1. Confirmar changelog e classificar a mudança (major/minor/patch) para definir a tag `vX.Y.Z`.
2. Se for major `v2+`, ajustar o module path em `go.mod` e em todos os imports internos para o
   sufixo `/vN` (regra de semver import compatibility do Go).
3. Compilar cada alvo da matriz injetando versão/commit via `-ldflags`:

```bash
GOOS=linux GOARCH=amd64 go build -trimpath \
  -ldflags "-s -w -X 'main.Version=v1.2.3' -X 'main.Commit=$(git rev-parse --short HEAD)'" \
  -o dist/app_linux_amd64 ./cmd/app
```

4. Gerar checksums dos artefatos e publicar `go.sum` validado (`GOFLAGS=-mod=readonly go mod verify`):

```bash
sha256sum dist/* > dist/checksums.txt
GONOSUMCHECK=0 GOSUMDB=sum.golang.org go mod verify
```

5. Criar a tag git assinada e publicar as notas de release referenciando os artefatos e checksums.

> Snippets acima ≤ 15 linhas cada. Matriz completa de plataformas → `./exemplos/README.md`.

## Outputs obrigatórios

| Output | Localização | Formato |
|--------|-------------|---------|
| Binários versionados por plataforma | `dist/<app>_<goos>_<goarch>[.exe]` | Binário |
| `checksums.txt` | `dist/checksums.txt` | texto (sha256) |
| Tag git de release | repositório remoto | tag semver (`vX.Y.Z`) |
| Notas de release | `RELEASE_NOTES_vX.Y.Z.md` ou release do Git host | Markdown |

## Checklist de validação

- [ ] Tag semver corresponde exatamente à versão declarada em `go.mod` (e ao sufixo `/vN` se major `v2+`)
- [ ] Todos os binários da matriz `GOOS`/`GOARCH` compilaram com `-ldflags` injetando versão/commit
- [ ] `checksums.txt` publicado junto dos binários e conferido antes da distribuição
- [ ] `go mod verify` executado sem erro (`go.sum` íntegro)
- [ ] Notas de release descrevem mudanças, artefatos e checksums

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

**Conflitos conhecidos:** módulos publicados devem seguir semver estrito (`v2+` exige sufixo de major version no import path).

## Checklist Go  ← OBRIGATÓRIO (Go)

- [ ] `gofmt -l .` sem saída (código formatado)
- [ ] `go vet ./...` sem avisos
- [ ] `go build ./...` sem erros
- [ ] `go test ./...` verde (quando aplicável ao tema da skill)
- [ ] Erros tratados explicitamente (`if err != nil`), nunca descartados com `_` sem justificativa
- [ ] Binário versionado via `-ldflags "-X main.version=..."`, sem hardcode

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
| Publicar módulo `v2+` sem sufixo de major version no import path | Quebra a resolução de dependências do Go modules; consumidores não conseguem importar corretamente | Atualizar `module` em `go.mod` e todos os imports internos para `.../v2`, `.../v3` etc. |
| Criar tag git que não corresponde à versão de `go.mod`/changelog | Consumidores via `go get` recebem código com versão divergente da anunciada | Sincronizar tag, changelog e `go.mod` antes de publicar; validar em CI |
| Distribuir binário sem checksum publicado | Impossível verificar integridade do artefato baixado; abre risco de supply chain | Gerar `sha256sum` de cada artefato e publicá-lo junto da release |
| Hardcode de versão em constante no código-fonte | Exige recompilar e commitar a cada release; divorcia binário da tag real | Injetar versão em build-time via `-ldflags "-X pkg.Var=valor"` |
| Ignorar `GOSUMDB`/`go mod verify` antes de empacotar | Dependências adulteradas passam despercebidas para o artefato final | Rodar `go mod verify` e manter `GOSUMDB` ativo (não usar `GONOSUMCHECK` em produção) |

## Avaliação de risco

- **Parar e confirmar quando:** a release envolver bump de major version (`v1` → `v2+`) ou
  publicação em repositório/registro oficial acessível a terceiros.
- **Risco alto:** publicar tag/release sem `go mod verify` limpo ou sem checksums — abre
  superfície de ataque de supply chain para consumidores do módulo/binário.
- **Risco baixo:** gerar build local de teste (`dist/` não publicado) ou ajustar apenas notas
  de release em rascunho.

## Métricas de sucesso

- 100% dos binários da matriz de release com versão/commit injetados via `-ldflags` (zero hardcode).
- `go mod verify` e checksums conferidos sem falha antes de cada publicação.
- Toda tag `vX.Y.Z` publicada corresponde exatamente ao `go.mod` e ao changelog da release.
- Zero releases major (`v2+`) publicadas sem o sufixo de major version no module path.

## Responsável principal

| Papel | Quem |
|-------|------|
| Agent executor | `developer-golang-agent-orchestrator` |
| Revisão humana | Desenvolvedor / Tech Lead |
| Aprovação final | Tech Lead |

## Referências

- [Go Modules Reference — Release workflow](https://go.dev/doc/modules/release-workflow)
- [Go Modules — major version suffixes](https://go.dev/doc/modules/major-version)
- Skill relacionada: `developer-go-build-toolchain`

---

## Versão interna (arquivo)

| Campo | Valor |
|-------|-------|
| **FileVersion** | 1.0.0 |
| **Política** | `.cursor/VERSION.md` |

## Changelog (este arquivo)

- 1.0.0 (09/08/2026): Criação da skill — empacotamento e entrega Go (versionamento via
  `-ldflags`, semver de módulos com sufixo de major version, checksums `go.sum`/`GOSUMDB`,
  estratégia de release multiplataforma), inaugurando o par com `developer-go-build-toolchain`.

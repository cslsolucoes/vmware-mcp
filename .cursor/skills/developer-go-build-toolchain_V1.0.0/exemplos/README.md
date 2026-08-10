# Exemplos — developer-go-build-toolchain

Índice dos exemplos completos que acompanham o `SKILL.md`. Os snippets do corpo da skill
são intencionalmente curtos (≤ 15 linhas); os arquivos abaixo mostram o fluxo completo,
compilável com `go build`.

| Arquivo | Conteúdo demonstrado |
|---------|-----------------------|
| [`main.go`](./main.go) | Variáveis `Version`/`Commit` injetáveis via `-ldflags -X` |
| [`platform_windows.go`](./platform_windows.go) | Build constraint `//go:build windows` |
| [`platform_unix.go`](./platform_unix.go) | Build constraint `//go:build !windows` |

## Como validar

```bash
cd exemplos
gofmt -l .          # deve retornar vazio
go vet ./...
go build -ldflags "-X main.Version=1.2.3 -X main.Commit=abc123" -o bin/exemplo .
./bin/exemplo        # imprime version=1.2.3 commit=abc123 platform=<so>
```

## Cross-compile

| GOOS | GOARCH | Uso típico |
|------|--------|------------|
| `linux` | `amd64` | Servidores Linux x86-64 |
| `windows` | `amd64` | Desktop/servidor Windows x86-64 |
| `darwin` | `arm64` | macOS Apple Silicon |

```bash
GOOS=linux   GOARCH=amd64 go build -o bin/exemplo-linux-amd64   .
GOOS=windows GOARCH=amd64 go build -o bin/exemplo-windows.exe   .
GOOS=darwin  GOARCH=arm64 go build -o bin/exemplo-darwin-arm64  .
```

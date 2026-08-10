# Exemplos — developer-go-language-core

Índice dos exemplos completos que acompanham o `SKILL.md`. Os snippets do corpo da skill
são intencionalmente curtos (≤ 15 linhas); os arquivos abaixo mostram o fluxo completo,
compilável com `go build`.

| Arquivo | Conteúdo demonstrado |
|---------|-----------------------|
| [`fundamentos.go`](./fundamentos.go) | Variáveis (`var`/`:=`), `if`/`for`/`switch`, função com múltiplos retornos, função variádica, `package main` executável |

## Como validar

```bash
cd exemplos
gofmt -l .          # deve retornar vazio
go vet ./...
go run fundamentos.go
```

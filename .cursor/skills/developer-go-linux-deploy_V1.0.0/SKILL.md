---
name: developer-go-linux-deploy
description: "Deploy de binários Go em Linux — cross-compile GOOS=linux, unit systemd (Type=simple, Restart=on-failure), shutdown gracioso via os/signal (SIGTERM/SIGINT) e checklist de operação pós-deploy."
model: sonnet
thinking: normal
category: developer-go
license: MIT
copyright: "Copyright (c) 2026 CSL Tech Solutions"
company: "CSL Tech Solutions"
author: "Claiton de Souza Linhares"
---

# Deploy Go em Linux

## Responsabilidade única

Esta skill cobre o deploy de um binário Go compilado em servidor Linux gerido por
systemd: cross-compile estático para `GOOS=linux`, unit file systemd (`Type=simple`,
`Restart=on-failure`), shutdown gracioso via `os/signal` e o checklist de operação
pós-deploy. Existe separada de `developer-go-build-toolchain` (que cobre só a geração
do binário) porque esta cobre a **operação em produção Linux**. Diferente do daemon
UNIX clássico (fork/setsid), um binário Go já é processo único e estático — não precisa
de double-fork: o systemd assume o ciclo de vida via `Type=simple`.

## When to use

- Compilar um binário Go estático para rodar em servidor Linux (`GOOS=linux`).
- Criar/ajustar um unit file systemd para gerir o processo Go como serviço.
- Implementar shutdown gracioso (`SIGTERM`/`SIGINT`) antes de expor o serviço em produção.
- Definir usuário dedicado, permissões e estrutura de diretório do deploy.
- Diagnosticar falhas de restart, permissões ou binário não iniciando via `journalctl`.

## When NOT to use

- Cross-compile básico sem deploy (apenas gerar o binário) → usar `developer-go-build-toolchain`.
- Empacotamento/distribuição multi-SO (instaladores, releases versionados) → usar `developer-go-packaging-delivery`.
- Implementar goroutines/channels internos ao serviço → usar `developer-go-concurrency-basics`.
- Daemon UNIX clássico com fork/setsid em Delphi/FPC → usar `developer-delphi-to-fpc-linux-daemon`.

## Inputs obrigatórios

| Input | Tipo | Descrição |
|-------|------|-----------|
| Caminho do módulo/pacote `main` | string | Diretório com `go.mod`/`cmd/<app>` a compilar |
| Arquitetura alvo | string | `GOARCH` — `amd64` ou `arm64`, conforme o servidor |
| Nome do serviço e usuário systemd | string | Nome do `.service` e conta dedicada (nunca `root`) |
| Caminho de instalação no servidor | string | Ex.: `/opt/<app>/` — diretório de trabalho do systemd |

## Dependências (skills prévias)

| Skill | Quando executar antes |
|-------|-----------------------|
| `developer-go-build-toolchain` | Antes — `go.mod` válido e build local (`go build ./...`) já verde |

## Workflow executável

1. Cross-compilar o binário estático para Linux (ver bloco Go — `CGO_ENABLED=0`):

```bash
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o bin/app-linux-amd64 ./cmd/app
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o bin/app-linux-arm64 ./cmd/app
```

2. Implementar shutdown gracioso — capturar `SIGTERM`/`SIGINT` via `os/signal.NotifyContext` (systemd envia `SIGTERM` em `stop`/`restart`):

```go
ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
defer stop()
go func() {
    <-ctx.Done()
    srv.Shutdown(context.Background()) // fecha conexões em aberto
}()
srv.ListenAndServe()
```

3. Transferir o binário e preparar usuário/diretório dedicado no servidor:

```bash
scp bin/app-linux-amd64 usuario@servidor:/opt/app/app
ssh usuario@servidor 'sudo useradd -r -s /usr/sbin/nologin app || true'
ssh usuario@servidor 'sudo chown app:app /opt/app/app && sudo chmod 750 /opt/app/app'
```

4. Criar o unit file systemd (`Type=simple` — sem fork, o Go já é processo único):

```ini
[Unit]
Description=Meu App Go
After=network.target

[Service]
Type=simple
User=app
Group=app
WorkingDirectory=/opt/app
ExecStart=/opt/app/app --config /etc/app/config.yaml
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
```

5. Instalar, habilitar e validar o serviço:

```bash
sudo cp app.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now app
journalctl -u app -f
```

> Snippets acima ≤ 15 linhas cada. Exemplo combinado maior → `./exemplos/README.md`.

## Outputs obrigatórios

| Output | Localização | Formato |
|--------|-------------|---------|
| Binário Linux compilado | `bin/app-linux-<arch>` | executável estático ELF |
| Unit file systemd | `/etc/systemd/system/<app>.service` | INI (systemd unit) |
| Evidência de validação (start/restart/logs) | log anexado ao commit/PR de deploy | texto |

## Checklist de validação

- [ ] `GOOS=linux GOARCH=<arch> go build` conclui sem erros
- [ ] `systemctl status <app>` mostra `active (running)` após `enable --now`
- [ ] `systemctl stop <app>` gera saída limpa nos logs (sem stack trace de shutdown abrupto)
- [ ] `journalctl -u <app> -n 50` sem erros críticos após o start
- [ ] Serviço roda sob usuário dedicado, não `root`

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
GOOS=linux GOARCH=amd64 go build -o bin/app ./cmd/app
go mod tidy
```

**Conflitos conhecidos:** `CGO_ENABLED=0` para binário estático (sem depender de `libc` do host) quando o código não usa cgo.

## Checklist Go  ← OBRIGATÓRIO (Go)

- [ ] `gofmt -l .` sem saída (código formatado)
- [ ] `go vet ./...` sem avisos
- [ ] `go build ./...` sem erros
- [ ] `go test ./...` verde (quando aplicável ao tema da skill)
- [ ] Erros tratados explicitamente (`if err != nil`), nunca descartados com `_` sem justificativa
- [ ] Serviço systemd trata `SIGTERM`/`SIGINT` via `os/signal` para shutdown gracioso

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
| Rodar o binário como `root` sem necessidade | Amplia o raio de impacto de qualquer falha/vulnerabilidade do serviço | Criar usuário dedicado (`useradd -r`) e `User=`/`Group=` no unit file |
| Ignorar `SIGTERM` e forçar `kill -9` no deploy | Conexões em aberto e escritas pendentes são cortadas abruptamente; risco de corrupção/perda de dados | Capturar `SIGTERM`/`SIGINT` via `os/signal.NotifyContext` e chamar `Shutdown` gracioso |
| Unit file sem `Restart=on-failure` | Um crash do binário derruba o serviço definitivamente até intervenção manual | Definir `Restart=on-failure` e `RestartSec=5` (ou superior) no `.service` |
| Compilar com `CGO_ENABLED=1` sem necessidade real de cgo | Binário fica dinamicamente ligado a `libc`, quebrando em imagens/hosts minimalistas | Manter `CGO_ENABLED=0` a menos que uma dependência exija cgo explicitamente |

## Avaliação de risco

- **Parar e confirmar quando:** a mudança alterar o unit file em produção (porta, usuário, `WorkingDirectory`) ou trocar a arquitetura alvo do binário já em uso.
- **Risco alto:** publicar um binário sem shutdown gracioso — perda de requisições/dados em todo `restart`/`stop` do serviço.
- **Risco baixo:** ajustes locais de build (`-o`, flags de debug) que não afetam o unit file nem o binário já instalado em produção.

## Métricas de sucesso

- `systemctl status <app>` reporta `active (running)` de forma estável após o deploy
- Zero perda de conexão observada nos logs durante `systemctl restart <app>`
- `journalctl -u <app>` sem erros críticos nas primeiras 24h após o deploy
- 100% dos deploys usam usuário dedicado (não `root`) e `Restart=on-failure`

## Responsável principal

| Papel | Quem |
|-------|------|
| Agent executor | `developer-golang-agent-orchestrator` |
| Revisão humana | Desenvolvedor / Tech Lead |
| Aprovação final | Tech Lead |

## Referências

- [Package os/signal](https://pkg.go.dev/os/signal)
- Skill relacionada: `developer-go-build-toolchain`

---

## Versão interna (arquivo)

| Campo | Valor |
|-------|-------|
| **FileVersion** | 1.0.0 |
| **Política** | `.cursor/VERSION.md` |

## Changelog (este arquivo)

- 1.0.0 (09/08/2026): Criação da skill — deploy de binários Go em Linux (cross-compile GOOS=linux, unit systemd Type=simple/Restart=on-failure, shutdown gracioso via os/signal SIGTERM/SIGINT), família `developer-go-*`.

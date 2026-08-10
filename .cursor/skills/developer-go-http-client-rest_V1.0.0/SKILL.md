---
name: developer-go-http-client-rest
description: Ensina a consumir APIs REST em Go usando net/http — reutilização de http.Client com timeout, GET/POST/PUT/DELETE, payload JSON com encoding/json, context em requisições e retries com backoff simples.
model: sonnet
thinking: normal
category: developer-go
license: MIT
copyright: "Copyright (c) 2026 CSL Tech Solutions"
company: "CSL Tech Solutions"
author: "Claiton de Souza Linhares"
---

# Cliente HTTP/REST em Go

## Responsabilidade única

Esta skill cobre o consumo de APIs REST a partir de um programa Go usando exclusivamente a
stdlib `net/http`: criação e reutilização de um `http.Client` com timeout explícito, montagem
de requisições GET/POST/PUT/DELETE, envio e leitura de payload JSON via `encoding/json`,
propagação de `context.Context` para cancelamento/deadline e uma estratégia simples de retry
com backoff. Ela existe separada do servidor HTTP porque trata apenas do lado cliente da
comunicação — expor endpoints é responsabilidade de outra skill.

## When to use

- Consumir uma API REST externa ou interna (GET, POST, PUT, DELETE, PATCH)
- Enviar/receber corpo JSON tipado a partir de structs Go (`encoding/json`)
- Propagar `context.Context` (deadline, cancelamento) em chamadas HTTP de saída
- Configurar um `http.Client` com timeout, transporte customizado e reutilização de conexões
- Implementar retry com backoff simples para chamadas HTTP sujeitas a falha transitória
- Tratar status codes HTTP e erros de rede de forma explícita

## When NOT to use

- Implementar o servidor HTTP (rotas, handlers, middlewares) → `developer-go-http-server`
- Apenas serializar/desserializar JSON sem envolver transporte HTTP → `developer-go-stdlib-encoding`
- Padronizar wrapping/propagação de erros de domínio → `developer-go-error-handling-and-diagnostics`
- Autenticação/autorização do lado servidor (emissão de JWT, sessões) — fora do escopo desta skill

## Inputs obrigatórios

| Input | Tipo | Descrição |
|-------|------|-----------|
| URL base do serviço REST | `string` | Endpoint alvo (ex.: `https://api.example.com`) |
| Struct de request/response | `.go` | Tipos tipados com tags `json:"..."` para o payload |
| Timeout desejado | `time.Duration` | Aplicado ao `http.Client` e/ou ao `context` |
| `go.mod` do módulo | Arquivo | Confirma a versão mínima de Go disponível |

## Dependências (skills prévias)

| Skill | Quando executar antes |
|-------|-----------------------|
| `developer-go-stdlib-encoding` | Definir tags `json:"..."` e Marshal/Unmarshal do payload antes de plugar no cliente HTTP |
| `developer-go-error-handling-and-diagnostics` | Padronizar como erros de rede/status são envolvidos (`%w`) e propagados ao chamador |

## Workflow executável

1. Criar um `http.Client` **uma única vez** (nível de pacote ou injetado), com timeout explícito.
   ```go
   var httpClient = &http.Client{Timeout: 10 * time.Second}
   ```
2. Montar a requisição com `http.NewRequestWithContext`, nunca `http.NewRequest` isolado.
   ```go
   req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
   if err != nil {
       return fmt.Errorf("build request: %w", err)
   }
   ```
3. Para POST/PUT, serializar a struct com `json.Marshal` e definir `Content-Type`.
   ```go
   body, _ := json.Marshal(payload)
   req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
   req.Header.Set("Content-Type", "application/json")
   ```
4. Executar com `httpClient.Do(req)`, checar o erro e **sempre** fechar `resp.Body` com `defer`.
   ```go
   resp, err := httpClient.Do(req)
   if err != nil {
       return fmt.Errorf("do request: %w", err)
   }
   defer resp.Body.Close()
   ```
5. Validar o status code antes de decodificar; só então usar `json.NewDecoder(resp.Body).Decode`.
   ```go
   if resp.StatusCode != http.StatusOK {
       return fmt.Errorf("unexpected status: %d", resp.StatusCode)
   }
   var out Result
   if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
       return fmt.Errorf("decode response: %w", err)
   }
   ```
6. Para chamadas sujeitas a falha transitória, aplicar retry com backoff simples (poucas tentativas,
   espera crescente) — nunca retry infinito nem sem respeitar `ctx.Done()`.
   ```go
   for attempt := 0; attempt < 3; attempt++ {
       resp, err = httpClient.Do(req)
       if err == nil && resp.StatusCode < 500 {
           break
       }
       time.Sleep(time.Duration(attempt+1) * 200 * time.Millisecond)
   }
   ```

## Outputs obrigatórios

| Output | Localização | Formato |
|--------|-------------|---------|
| Cliente HTTP reutilizável com timeout | Pacote Go alvo (`*.go`) | Go |
| Funções de chamada por operação (Get/Post/Put/Delete) | Pacote Go alvo (`*.go`) | Go |
| Structs de request/response tipadas | Pacote Go alvo (`*.go`) | Go |
| Testes com servidor de teste (`httptest.Server`) | `*_test.go` no mesmo pacote | Go |

## Checklist de validação

- [ ] `http.Client` criado uma única vez e reutilizado entre requisições, com `Timeout` explícito
- [ ] Toda requisição usa `http.NewRequestWithContext` com um `context.Context` derivado do chamador
- [ ] `resp.Body.Close()` chamado via `defer` em todo caminho que recebeu uma resposta com sucesso
- [ ] Status code validado antes de decodificar o corpo da resposta
- [ ] Retry (quando presente) é limitado, com backoff, e respeita `ctx.Done()`

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

**Conflitos conhecidos:** `net/http` (stdlib) é suficiente para a maioria dos casos — evitar dependências de terceiros sem necessidade real.

## Checklist Go  ← OBRIGATÓRIO (Go)

- [ ] `gofmt -l .` sem saída (código formatado)
- [ ] `go vet ./...` sem avisos
- [ ] `go build ./...` sem erros
- [ ] `go test ./...` verde (quando aplicável ao tema da skill)
- [ ] Erros tratados explicitamente (`if err != nil`), nunca descartados com `_` sem justificativa
- [ ] `http.Client` reutilizado (nunca criado por requisição) com timeout explícito

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
| Criar `http.Client{}` sem `Timeout` | Requisição pode travar indefinidamente em rede lenta/instável, esgotando goroutines | Definir `Timeout` explícito no `http.Client` e/ou usar `context` com deadline |
| Esquecer `defer resp.Body.Close()` | Vaza conexões TCP e file descriptors, degradando o processo sob carga | Fechar o `Body` com `defer` logo após checar o erro de `Do` |
| Instanciar um `http.Client` novo a cada requisição | Perde reuso de conexões (keep-alive), aumenta latência e handshakes TLS | Criar o `http.Client` uma vez (pacote/injeção) e reutilizar entre chamadas |
| Ignorar o status code e decodificar o corpo direto | Erro HTTP (4xx/5xx) é tratado como sucesso, corrompendo o fluxo com dado inválido | Validar `resp.StatusCode` antes de chamar `json.Decode` |
| Retry sem limite ou sem respeitar `ctx.Done()` | Amplifica indefinidamente uma falha do servidor remoto e ignora cancelamento do chamador | Limitar tentativas, aplicar backoff crescente e checar `ctx.Err()` a cada iteração |

## Avaliação de risco

- **Parar e confirmar quando:** alterar o timeout padrão ou a política de retry de um cliente HTTP
  já usado em produção por múltiplos consumidores
- **Risco alto:** desabilitar validação de certificado TLS (`InsecureSkipVerify: true`) em produção
  ou implementar retry sem limite contra um endpoint externo
- **Risco baixo:** adicionar uma nova função de chamada (Get/Post) isolada reutilizando o cliente
  já existente, sem alterar configuração compartilhada

## Métricas de sucesso

- Zero vazamentos de `resp.Body` detectados em revisão (`go vet` e checklist manual limpos)
- 100% das chamadas HTTP usando `context.Context` propagado do chamador
- Testes cobrindo sucesso, status de erro (4xx/5xx) e timeout/cancelamento via `httptest.Server`

## Responsável principal

| Papel | Quem |
|-------|------|
| Agent executor | `developer-golang-agent-orchestrator` |
| Revisão humana | Desenvolvedor / Tech Lead |
| Aprovação final | Tech Lead |

## Referências

- [pkg.go.dev — net/http](https://pkg.go.dev/net/http)
- Skill relacionada: `developer-go-http-server`

---

## Versão interna (arquivo)

| Campo | Valor |
|-------|-------|
| **FileVersion** | 1.0.0 |
| **Política** | `.cursor/VERSION.md` |

## Changelog (este arquivo)

- 1.0.0 (09/08/2026): Criação da skill — cliente HTTP REST em Go com `net/http`, payload JSON via
  `encoding/json`, `context` em requisições e retry com backoff simples.

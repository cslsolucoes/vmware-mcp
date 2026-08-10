---
name: developer-go-crypto-security
description: Criptografia e segurança em Go usando a stdlib crypto/* (sha256, hmac), crypto/tls (configuração mínima segura), crypto/rand (valores aleatórios seguros) e hash de senha com golang.org/x/crypto/bcrypt.
model: sonnet
thinking: normal
category: developer-go
license: MIT
copyright: "Copyright (c) 2026 CSL Tech Solutions"
company: "CSL Tech Solutions"
author: "Claiton de Souza Linhares"
---

# Criptografia e Segurança em Go

## Responsabilidade única

Esta skill cobre a proteção de dados sensíveis em aplicações Go usando exclusivamente
primitivas confiáveis: hashing e HMAC da stdlib (`crypto/sha256`, `crypto/hmac`), geração de
valores aleatórios criptograficamente seguros (`crypto/rand`), configuração mínima segura de
TLS (`crypto/tls`) e hash de senha com custo ajustável (`golang.org/x/crypto/bcrypt`). Ela existe
separada da codificação de dados porque trata da **segurança** do valor (é sigiloso? é
verificável? resiste a força bruta?), não do formato de transporte — a serialização em si é
responsabilidade de outra skill.

## When to use

- Armazenar senha de usuário de forma não reversível (hash + verificação de login)
- Gerar token de sessão, salt, nonce ou chave de API aleatória e imprevisível
- Assinar/verificar a autenticidade de uma mensagem com chave secreta compartilhada (HMAC)
- Calcular checksum de integridade de arquivo/payload (SHA-256, não para senha)
- Configurar `tls.Config` mínimo seguro para cliente ou servidor HTTP/TCP

## When NOT to use

- Serializar/codificar payload em JSON, XML, CSV, Base64 ou Hex sem preocupação de segredo →
  `developer-go-stdlib-encoding`
- Implementar o cliente/servidor HTTP em si (esta skill só define o `tls.Config` usado por eles) →
  `developer-go-http-server` / `developer-go-http-client-rest`
- Emissão/validação completa de token JWT assinado (além do HMAC/assinatura crua) — fora do escopo
  desta skill; usar biblioteca JWT dedicada quando disponível no projeto
- Modelagem do tipo Go que carrega o dado sensível antes de protegê-lo → `developer-go-language-types`

## Inputs obrigatórios

| Input | Tipo | Descrição |
|-------|------|-----------|
| Dado sensível a proteger | `string` \| `[]byte` | Senha, token, payload que precisa de hash, HMAC ou TLS |
| Chave secreta (quando HMAC/TLS) | `[]byte` \| certificado | Carregada de variável de ambiente/secret manager, nunca hardcoded |
| Contexto de uso | Decisão | Senha de login vs integridade de mensagem vs canal TLS — determina a primitiva certa |
| `go.mod` do módulo | Arquivo | Confirma a versão mínima de Go disponível |

## Dependências (skills prévias)

| Skill | Quando executar antes |
|-------|-----------------------|
| `developer-go-stdlib-encoding` | Representar bytes de hash/token/assinatura como texto (`hex`/`base64`) para armazenar ou transmitir |

## Workflow executável

1. Calcular hash de integridade (não senha) com `crypto/sha256`.
   ```go
   import "crypto/sha256"

   sum := sha256.Sum256(data)
   hexSum := hex.EncodeToString(sum[:])
   ```
2. Assinar e verificar autenticidade de mensagem com HMAC, comparando em tempo constante.
   ```go
   func Sign(msg, key []byte) []byte {
       mac := hmac.New(sha256.New, key)
       mac.Write(msg)
       return mac.Sum(nil)
   }

   func Verify(msg, key, sig []byte) bool {
       return hmac.Equal(Sign(msg, key), sig)
   }
   ```
3. Gerar valor aleatório seguro (token, salt, nonce) com `crypto/rand` — nunca `math/rand`.
   ```go
   func NewToken(n int) ([]byte, error) {
       b := make([]byte, n)
       if _, err := rand.Read(b); err != nil {
           return nil, fmt.Errorf("generate token: %w", err)
       }
       return b, nil
   }
   ```
4. Hashear e verificar senha de usuário com `bcrypt` (custo ajustável, resistente a brute-force).
   ```go
   func HashPassword(pw string) (string, error) {
       h, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
       if err != nil {
           return "", fmt.Errorf("hash password: %w", err)
       }
       return string(h), nil
   }

   func CheckPassword(hash, pw string) bool {
       return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) == nil
   }
   ```
5. Configurar `tls.Config` mínimo seguro (versão mínima TLS 1.2, sem `InsecureSkipVerify`).
   ```go
   cfg := &tls.Config{
       MinVersion:       tls.VersionTLS12,
       CurvePreferences: []tls.CurveID{tls.CurveP256, tls.X25519},
       // InsecureSkipVerify permanece false (padrão) — nunca true em produção
   }
   ```

## Outputs obrigatórios

| Output | Localização | Formato |
|--------|-------------|---------|
| Funções de hash/HMAC/verificação de senha | Pacote Go alvo (`*.go`) | Go |
| `tls.Config` mínimo seguro do serviço | Pacote Go alvo (`*.go`) | Go |
| Testes de round-trip (hash/HMAC válido e inválido, senha correta e incorreta) | `*_test.go` no mesmo pacote | Go |

## Checklist de validação

- [ ] Toda senha de usuário é hasheada com `bcrypt.GenerateFromPassword` antes de persistir — nunca
  texto plano nem hash raso (MD5/SHA1/SHA256 sem custo ajustável)
- [ ] Todo valor aleatório sensível (token, salt, nonce, chave de sessão) usa `crypto/rand`
- [ ] Comparação de HMAC/hash usa `hmac.Equal` ou `subtle.ConstantTimeCompare` — nunca `==`/`bytes.Equal`
- [ ] `tls.Config` define `MinVersion >= tls.VersionTLS12` e nunca `InsecureSkipVerify: true` fora de teste local
- [ ] Chave secreta (HMAC, certificado TLS) carregada de variável de ambiente/secret manager

---

## Stack e versões  ← OBRIGATÓRIO (Go)

| Componente | Versão mínima | Notas |
|------------|:---:|-------|
| Go | 1.21 | `go.mod` declara `go 1.21` ou superior |
| gofmt | embutido | Formatação obrigatória, sem exceções |
| go vet | embutido | Rodar antes de qualquer commit |
| golangci-lint | 1.55+ | Lint agregador (opcional mas recomendado); `gosec` recomendado para achados de segurança |

## Dependências (go.mod / go get)  ← OBRIGATÓRIO (Go)

```bash
go mod init <module-path>
go get golang.org/x/crypto/bcrypt@latest
go mod tidy
```

**Conflitos conhecidos:** `crypto/*` da stdlib cobre hashing/TLS/random; `bcrypt` para senha exige `golang.org/x/crypto`.

## Checklist Go  ← OBRIGATÓRIO (Go)

- [ ] `gofmt -l .` sem saída (código formatado)
- [ ] `go vet ./...` sem avisos
- [ ] `go build ./...` sem erros
- [ ] `go test ./...` verde (quando aplicável ao tema da skill)
- [ ] Erros tratados explicitamente (`if err != nil`), nunca descartados com `_` sem justificativa
- [ ] `crypto/rand` usado para qualquer valor sensível (nunca `math/rand`)

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
| `math/rand` para gerar token/senha/salt | Sequência previsível (seed geralmente por tempo) reduz drasticamente o espaço de busca de um ataque | Usar sempre `crypto/rand.Read` (ou `crypto/rand.Int`) |
| MD5/SHA1/SHA256 puro para hash de senha | Hash rápido demais permite força bruta/rainbow table em GPU; sem salt nem fator de custo | Usar `bcrypt.GenerateFromPassword` com custo ≥ `bcrypt.DefaultCost` |
| `tls.Config{InsecureSkipVerify: true}` em produção | Desativa a validação do certificado do servidor, abrindo brecha para man-in-the-middle | Validar o certificado normalmente; usar `RootCAs` customizado se necessário, nunca desativar |
| Comparar hash/HMAC com `==` ou `bytes.Equal` | Comparação não é de tempo constante e vaza informação por timing attack | Usar `hmac.Equal` ou `crypto/subtle.ConstantTimeCompare` |

## Avaliação de risco

- **Parar e confirmar quando:** trocar o algoritmo/custo de hash de senha já em produção — afeta o
  login de todos os usuários existentes e exige estratégia de migração
- **Risco alto:** `InsecureSkipVerify: true` ou desativação de validação de certificado TLS em código
  que vai para produção; chave secreta hardcoded no repositório
- **Risco baixo:** adicionar checksum SHA-256 para verificação de integridade de arquivo/dado não
  sensível, sem impacto em autenticação

## Métricas de sucesso

- Zero ocorrências de `math/rand` para valores sensíveis (auditável via `gosec`/busca no código)
- 100% das senhas de usuário armazenadas via `bcrypt`, com custo documentado
- Zero `tls.Config` com `InsecureSkipVerify: true` fora de arquivos de teste

## Responsável principal

| Papel | Quem |
|-------|------|
| Agent executor | `developer-golang-agent-orchestrator` |
| Revisão humana | Desenvolvedor / Tech Lead |
| Aprovação final | Tech Lead |

## Referências

- [pkg.go.dev — crypto](https://pkg.go.dev/crypto)
- [pkg.go.dev — golang.org/x/crypto/bcrypt](https://pkg.go.dev/golang.org/x/crypto/bcrypt)
- Skill relacionada: `developer-go-http-server`
- Skill relacionada: `developer-go-stdlib-encoding`

---

## Versão interna (arquivo)

| Campo | Valor |
|-------|-------|
| **FileVersion** | 1.0.0 |
| **Política** | `.cursor/VERSION.md` |

## Changelog (este arquivo)

- 1.0.0 (09/08/2026): Criação da skill — cobertura inicial de `crypto/sha256`, `crypto/hmac`,
  `crypto/rand`, `crypto/tls` (configuração mínima segura) e hash de senha com
  `golang.org/x/crypto/bcrypt`.

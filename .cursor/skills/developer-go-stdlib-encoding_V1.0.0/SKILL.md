---
name: developer-go-stdlib-encoding
description: Ensina a codificar e decodificar dados em Go usando a stdlib — encoding/json (Marshal/Unmarshal, struct tags, omitempty), encoding/xml, encoding/csv, encoding/base64 e encoding/hex — para quem serializa dados de domínio ou arquivos.
model: sonnet
thinking: normal
category: developer-go
license: MIT
copyright: "Copyright (c) 2026 CSL Tech Solutions"
company: "CSL Tech Solutions"
author: "Claiton de Souza Linhares"
---

# Encoding em Go

## Responsabilidade única

Esta skill cobre a conversão de dados de domínio Go para formatos de texto/binário e vice-versa
usando exclusivamente a stdlib: `encoding/json` (Marshal/Unmarshal, struct tags, `omitempty`),
`encoding/xml` (Marshal/Unmarshal com tags `xml:"..."`), `encoding/csv` (leitura/escrita de linhas
tabulares), `encoding/base64` (codificação binário-para-texto) e `encoding/hex` (codificação
binário-para-hexadecimal). Ela existe separada do transporte HTTP porque trata apenas da
transformação de bytes/estruturas — o cliente/servidor que envia esses bytes pela rede é
responsabilidade de outra skill.

## When to use

- Serializar uma struct Go para JSON/XML antes de persistir em arquivo ou repassar a outra camada
- Desserializar um payload JSON/XML já recebido em uma struct Go tipada
- Ler ou escrever arquivos `.csv` linha a linha (importação/exportação tabular)
- Codificar bytes binários (hash, chave, imagem) em texto seguro para transporte (`base64`) ou
  para representação legível/depuração (`hex`)
- Definir struct tags (`json:"campo,omitempty"`, `xml:"campo,attr"`) para controlar o formato final

## When NOT to use

- Montar ou consumir o payload de uma requisição/resposta HTTP em si (`http.Client`, `http.Server`,
  headers, status code) → `developer-go-http-client-rest` / `developer-go-http-server`
- Inspeção de tags via `reflect` em runtime para bibliotecas genéricas → `developer-go-stdlib-rtti-reflection`
- Modelagem do tipo Go em si (struct, interface, zero value) antes de anotar tags → `developer-go-language-types`
- Serialização binária proprietária entre processos Go-para-Go (`encoding/gob`) — fora do escopo
  desta skill; ver documentação oficial do pacote `encoding/gob`

## Inputs obrigatórios

| Input | Tipo | Descrição |
|-------|------|-----------|
| Struct Go a serializar/desserializar | `.go` | Tipo com campos exportados e tags de formato definidas |
| Formato alvo | `json` \| `xml` \| `csv` \| `base64` \| `hex` | Determina o pacote stdlib e as tags aplicáveis |
| Amostra do payload (quando desserializando) | JSON/XML/CSV | Estrutura real recebida, para validar a tag antes de codar |
| `go.mod` do módulo | Arquivo | Confirma a versão mínima de Go disponível |

## Dependências (skills prévias)

| Skill | Quando executar antes |
|-------|-----------------------|
| `developer-go-language-types` | Definir a struct/interface de domínio antes de anotar tags de encoding |

## Workflow executável

1. Anotar a struct com `json:"campo,omitempty"` — `omitempty` omite o campo quando é zero value.
   ```go
   type Customer struct {
       ID    int64  `json:"id"`
       Name  string `json:"name"`
       Email string `json:"email,omitempty"`
   }
   ```
2. Serializar com `json.Marshal` e tratar o erro explicitamente (nunca ignorar com `_`).
   ```go
   data, err := json.Marshal(Customer{ID: 1, Name: "Ana"})
   if err != nil {
       return fmt.Errorf("marshal customer: %w", err)
   }
   ```
3. Desserializar com `json.Unmarshal` em uma variável já tipada; validar o erro antes de usar o valor.
   ```go
   var c Customer
   if err := json.Unmarshal(data, &c); err != nil {
       return fmt.Errorf("unmarshal customer: %w", err)
   }
   ```
4. Para XML, usar tags `xml:"campo"` (ou `,attr` para atributo) e `xml.Marshal`/`xml.Unmarshal`.
   ```go
   type Order struct {
       XMLName xml.Name `xml:"order"`
       ID      int64    `xml:"id,attr"`
       Item    string   `xml:"item"`
   }
   out, err := xml.MarshalIndent(Order{ID: 7, Item: "Mouse"}, "", "  ")
   ```
5. Para CSV, usar `csv.NewReader`/`csv.NewWriter` linha a linha, sempre com `defer w.Flush()`.
   ```go
   w := csv.NewWriter(file)
   defer w.Flush()
   if err := w.Write([]string{"1", "Ana", "ana@example.com"}); err != nil {
       return fmt.Errorf("write csv row: %w", err)
   }
   ```
6. Codificar bytes binários em texto com `base64` (`StdEncoding`, ou `URLEncoding` para uso em URL)
   ou em hexadecimal legível (`hex`, útil para logs e checksums) — sempre checando o erro do decode.
   ```go
   encoded := base64.StdEncoding.EncodeToString(rawBytes)
   decoded, err := base64.StdEncoding.DecodeString(encoded)

   hexStr := hex.EncodeToString(rawBytes)
   raw, err := hex.DecodeString(hexStr)
   ```

## Outputs obrigatórios

| Output | Localização | Formato |
|--------|-------------|---------|
| Structs anotadas com tags de encoding | Pacote Go alvo (`*.go`) | Go |
| Funções/métodos de (des)serialização | Pacote Go alvo (`*.go`) | Go |
| Testes de round-trip (encode → decode → comparar) | `*_test.go` no mesmo pacote | Go |

## Checklist de validação

- [ ] Toda struct exposta a JSON/XML tem tags explícitas (`json:"..."`, `xml:"..."`), sem depender
  do nome exportado por acidente
- [ ] `omitempty` aplicado apenas a campos opcionais, nunca a campos obrigatórios do contrato
- [ ] Todo erro de `Marshal`/`Unmarshal`/`Decode`/`Encode` é verificado e propagado (`%w`)
- [ ] `csv.Writer` sempre chama `Flush()` (via `defer`) antes de checar `w.Error()`
- [ ] Escolha entre `base64.StdEncoding` e `base64.URLEncoding` documentada conforme o destino do dado

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

**Conflitos conhecidos:** nenhum conhecido para este tema (toda a família `encoding/*` usada aqui é stdlib pura).

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
| Ignorar o erro de `json.Unmarshal`/`xml.Unmarshal` (`_ = json.Unmarshal(...)`) | Payload malformado passa despercebido e a struct fica com zero values silenciosos | Sempre checar `err != nil` e propagar/logar antes de usar o valor decodificado |
| Struct sem tags `json:"..."` exposta em API pública | O nome do campo Go (PascalCase) vaza como contrato de payload, acoplando o cliente ao nome interno do tipo | Definir tags explícitas para todo campo exportado que atravessa a borda do processo |
| Usar `base64.StdEncoding` para dado que vai em URL/querystring | `+` e `/` do alfabeto padrão quebram a URL sem escaping adicional | Usar `base64.URLEncoding` (ou `RawURLEncoding`) quando o destino é uma URL |
| Reabrir/recriar `csv.Writer` a cada linha em vez de reutilizar a mesma instância com `Flush()` no final | Perde buffering, gera I/O excessivo e pode truncar a última linha se `Flush` não for chamado | Criar o `csv.Writer` uma vez, escrever todas as linhas e chamar `Flush()`/checar `Error()` ao final |

## Avaliação de risco

- **Parar e confirmar quando:** alterar uma tag `json:"..."`/`xml:"..."` de uma struct já consumida
  por outro serviço ou por um contrato de API publicado
- **Risco alto:** remover ou renomear um campo de uma struct exportada usada em payload de API sem
  período de compatibilidade — quebra clientes existentes silenciosamente
- **Risco baixo:** adicionar campo novo com `omitempty` a uma struct existente, ou criar uma nova
  função de (des)serialização isolada sem tocar em tipos já publicados

## Métricas de sucesso

- Zero erros de `Marshal`/`Unmarshal` ignorados no código revisado (`go vet` e revisão manual limpos)
- 100% das structs expostas a JSON/XML com tags explícitas por campo
- Testes de round-trip (encode → decode → comparar) cobrindo cada formato usado no pacote

## Responsável principal

| Papel | Quem |
|-------|------|
| Agent executor | `developer-golang-agent-orchestrator` |
| Revisão humana | Desenvolvedor / Tech Lead |
| Aprovação final | Tech Lead |

## Referências

- [pkg.go.dev — encoding/json](https://pkg.go.dev/encoding/json)
- [pkg.go.dev — encoding/xml](https://pkg.go.dev/encoding/xml)
- [pkg.go.dev — encoding/csv](https://pkg.go.dev/encoding/csv)
- [pkg.go.dev — encoding/base64](https://pkg.go.dev/encoding/base64)
- [pkg.go.dev — encoding/hex](https://pkg.go.dev/encoding/hex)
- Skills relacionadas: `developer-go-http-server`, `developer-go-http-client-rest`, `developer-go-language-types`

---

## Versão interna (arquivo)

| Campo | Valor |
|-------|-------|
| **FileVersion** | 1.0.0 |
| **Política** | `.cursor/VERSION.md` |

## Changelog (este arquivo)

- 1.0.0 (09/08/2026): Criação da skill — cobertura inicial de `encoding/json` (Marshal/Unmarshal,
  struct tags, `omitempty`), `encoding/xml`, `encoding/csv`, `encoding/base64` e `encoding/hex`.

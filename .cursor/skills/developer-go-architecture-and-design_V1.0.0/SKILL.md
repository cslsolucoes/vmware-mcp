---
name: developer-go-architecture-and-design
description: Arquitetura de projetos Go — layout cmd/internal/pkg, clean architecture adaptada, module boundaries e regras de dependência.
model: sonnet
thinking: extended
category: developer-go
license: MIT
copyright: "Copyright (c) 2026 CSL Tech Solutions"
company: "CSL Tech Solutions"
author: "Claiton de Souza Linhares"
---

# Arquitetura de Projetos Go

## Responsabilidade única

Esta skill resolve o problema de **estruturar um módulo Go em pacotes coerentes**
antes que o crescimento orgânico do código produza acoplamento acidental. Ela
cobre exclusivamente a organização física e as fronteiras de dependência entre
pacotes — layout `cmd/`/`internal/`/`pkg/`, direção das importações e a versão
"leve" de clean architecture idiomática em Go (sem as camadas rígidas de
linguagens orientadas a objeto clássicas). Existe separada de
`developer-go-patterns-composition` porque aquela trata de como **um tipo**
se compõe de outros; esta trata de como **pacotes inteiros** se relacionam
dentro do módulo.

## When to use

- Iniciar um módulo Go novo e decidir onde cada pacote deve viver
  (`cmd/`, `internal/`, `pkg/`).
- Revisar um projeto existente em que pacotes importam uns aos outros sem
  fronteira clara, causando dependência circular ou vazamento de detalhe
  de implementação.
- Definir a fronteira entre a API pública de um módulo (`pkg/`) e seus
  detalhes internos (`internal/`), antes de publicar o módulo.
- Decidir como adaptar clean architecture / hexagonal a Go sem introduzir
  camadas artificiais que a linguagem não pede.

## When NOT to use

- Aplicativo CLI simples de um único arquivo, sem necessidade de múltiplos
  pacotes → usar `developer-go-cli-apps`.
- Definição de tipos, structs e zero values dentro de um pacote já
  delimitado → usar `developer-go-language-core`.
- Composição de comportamento entre tipos via embedding → usar
  `developer-go-patterns-composition`.
- Escolha de padrão de criação de objetos (factory, options, singleton)
  dentro de um pacote já definido → usar `developer-go-patterns-creational`.
- Configuração de build, `go.mod`/`go.sum` e pipeline de release → usar a
  skill de build/delivery do kit Go.

## Inputs obrigatórios

| Input | Tipo | Descrição |
|-------|------|-----------|
| `module-path` | `string` | Caminho do módulo (`go.mod`), ex.: `github.com/csl/servico` |
| `binarios` | `[]string` | Nomes dos executáveis a expor em `cmd/<binario>/main.go` |
| `contratos-publicos` | `[]string` | Pacotes/tipos que devem ser importáveis por outros módulos (`pkg/`) |

## Dependências (skills prévias)

| Skill | Quando executar antes |
|-------|-----------------------|
| `developer-go-language-core` | Entender pacotes, visibilidade (exportado/não exportado) e zero values antes de desenhar fronteiras |
| `developer-go-patterns-composition` | Entender embedding e interfaces pequenas antes de definir contratos entre camadas |

## Workflow executável

1. Criar o esqueleto de diretórios do módulo, separando pontos de entrada,
   código privado e API pública.

```text
meu-modulo/
  cmd/servidor/main.go   # ponto de entrada; monta dependências, sem lógica
  internal/pedido/       # regra de negócio privada do módulo
  internal/storage/      # adaptador de persistência (implementa contrato)
  pkg/clientehttp/       # API pública reutilizável por outros módulos
  go.mod
```

2. Declarar os contratos (`interface`) de negócio no pacote que **consome**
   a dependência (ex.: `internal/pedido` define `type Repository interface`),
   nunca no pacote adaptador.
3. Implementar o adaptador concreto em pacote irmão (`internal/storage`),
   que satisfaz o contrato sem que `pedido` importe `storage`.
4. Em `cmd/<binario>/main.go`, montar (wiring) as dependências concretas e
   injetá-las nos construtores — é o único lugar que conhece todos os pacotes.
5. Garantir que a seta de importação aponta sempre **para dentro**: `cmd/`
   importa `internal/`; `internal/` nunca importa `cmd/`; `pkg/` não importa
   `internal/`.

## Outputs obrigatórios

| Output | Localização | Formato |
|--------|-------------|---------|
| Esqueleto de diretórios do módulo | `cmd/`, `internal/`, `pkg/` na raiz do módulo | Estrutura de pastas Go |
| Contratos de domínio | `internal/<dominio>/<arquivo>.go` | Go (`interface`) |
| Wiring de dependências | `cmd/<binario>/main.go` | Go |
| Registro de decisão arquitetural (quando aplicável) | `Documentation/Arquitetura/` | Markdown |

## Checklist de validação

- [ ] Nenhum pacote em `internal/` é importado por outro módulo (limite do Go respeitado)
- [ ] `pkg/` contém só o que é deliberadamente API pública, documentada
- [ ] `cmd/<binario>/main.go` não contém regra de negócio, só wiring
- [ ] Nenhuma dependência circular entre pacotes internos (`go list` confirma)
- [ ] Contrato (`interface`) declarado no pacote consumidor, não no adaptador

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
- [ ] `internal/` usado para tudo que não é contrato público do módulo

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
|-------------|-------------------|----------------|
| Pacote `utils`/`common` genérico virando lixeira de funções sem coesão | Cresce sem fronteira clara, vira dependência de tudo e dificulta refatoração | Nomear pacotes pelo que fazem (`retry`, `validation`); mover cada função para o pacote que a usa |
| Dependência circular entre pacotes internos (`pedido` importa `storage` e `storage` importa `pedido`) | Go recusa compilar; sintoma de contrato mal posicionado | Declarar o contrato no consumidor (`pedido`); `storage` implementa sem importar `pedido` de volta |
| Lógica de negócio dentro de `cmd/<binario>/main.go` | Impede reuso e teste da regra fora do binário; acopla domínio ao ponto de entrada | Mover a regra para `internal/<dominio>`; `main.go` só monta e injeta dependências |
| Expor em `pkg/` tipos que só deveriam existir em `internal/` | Vira contrato público permanente; qualquer mudança futura é breaking change | Manter em `internal/` até haver necessidade real de reuso externo comprovada |

## Avaliação de risco

- **Parar e confirmar quando:** a mudança proposta move um pacote de
  `internal/` para `pkg/` (ou vice-versa), alterando o que é contrato
  público do módulo.
- **Risco alto:** introduzir uma dependência circular entre pacotes
  internos para "resolver rápido" um acoplamento — sintoma de fronteira
  de domínio mal desenhada, não um problema de importação isolado.
- **Risco baixo:** criar um novo pacote dentro de `internal/` para isolar
  responsabilidade já identificada, sem alterar contratos existentes.

## Métricas de sucesso

- Zero dependências circulares reportadas por `go list -deps ./...`.
- 100% dos pontos de entrada (`cmd/*/main.go`) livres de regra de negócio.
- Nenhum tipo em `pkg/` sem uso comprovado por consumidor externo ao módulo.

## Responsável principal

| Papel | Quem |
|-------|------|
| Agent executor | `developer-golang-agent-orchestrator` |
| Revisão humana | Desenvolvedor / Tech Lead |
| Aprovação final | Tech Lead |

## Referências

- [Organizing a Go module — go.dev](https://go.dev/doc/modules/layout)
- Skill relacionada: `developer-go-master-orchestrator`

---

## Versão interna (arquivo)

| Campo | Valor |
|-------|-------|
| **FileVersion** | 1.0.0 |
| **Política** | `.cursor/VERSION.md` |

## Changelog (este arquivo)

- 1.0.0 (09/08/2026): Criação da skill — arquitetura de projetos Go (layout cmd/internal/pkg, clean architecture adaptada, module boundaries e direção de dependência).

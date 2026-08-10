# Blueprint GoLang

Estrutura reutilizável para novos projetos Go (módulos `go.mod`), servida pela
família de skills `developer-go-*` em `.cursor/skills/`.

## Estrutura

- `cmd/<binario>/main.go`: pontos de entrada executáveis (um `main` por binário).
- `internal/`: pacotes privados do módulo, não importáveis por outros módulos.
- `pkg/`: pacotes públicos, pensados para reuso externo (usar com moderação).
- `tests`: testes de integração que não cabem como `_test.go` ao lado do código.
- `Documentation`: documentação canônica (SPEC, regras de negócio, arquitetura).
- `.cursor`: regras, skills e planos locais.
- `go.mod` / `go.sum`: versionamento de módulo e dependências.

## Regras mínimas

- `gofmt` obrigatório (sem exceções) e `go vet ./...` limpo antes de commit.
- Erros tratados explicitamente (`if err != nil`); nunca descartar erro com `_`
  sem justificativa em comentário.
- Pacotes `internal/` para tudo que não é contrato público do módulo.
- Interfaces pequenas definidas no consumidor, não no provedor
  ("accept interfaces, return structs").
- Testes tabulares (`table-driven tests`) como padrão em `_test.go`.
- Changelog obrigatório em artefatos de governança.

## Skills relacionadas

- Ponto de entrada: `developer-go-master-orchestrator` (mapa completo do kit e
  tabela de roteamento por cenário).
- Especificação de projeto: `developer-go-project-spec` (gera `SPEC.md`).
- Ver `.cursor/skills/developer-go-*` para a família completa (linguagem,
  patterns, stdlib, concorrência, performance, build/delivery, arquitetura,
  qualidade).

---

Changelog (este arquivo):
- 1.0.0 (09/08/2026): Criação do blueprint reutilizável do kit Go, espelhando
  `kit-delphi-fpc_V1.0` e `kit-vuejs-nodejs_V1.0`.

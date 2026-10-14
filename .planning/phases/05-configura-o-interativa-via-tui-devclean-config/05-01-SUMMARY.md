---
phase: "05-configura-o-interativa-via-tui-devclean-config"
plan: "05-01"
title: "Wave 1 (Tracer) — Motor de Persistência AST YAML e Validação de Configuração (internal/config)"
wave: 1
completed_at: "2026-10-14"
requirements:
  - CFG-01
  - NFR-01
  - NFR-03
status: "completed"
commits:
  - hash: "cf0c807"
    message: "test(05-01): add unit tests for AST YAML mutator primitives"
  - hash: "00044e5"
    message: "feat(05-01): implement AST YAML surgical mutation primitives"
  - hash: "8b71532"
    message: "test(05-01): add tests for atomic save, backup, defaults, and syntax errors"
  - hash: "6cfc981"
    message: "feat(05-01): implement atomic save, backup copy, first-run defaults, and syntax diagnostics"
  - hash: "de3a82c"
    message: "refactor(05-01): unify AST YAML helpers in cmd/schedule and cmd/whitelist into internal/config"
coverage:
  internal/config/mutator.go: "100.0%"
  internal/config/save.go: "80.2%"
---

# Plan 05-01 Summary: Wave 1 (Tracer) — Motor de Persistência AST YAML e Validação de Configuração

## Overview
A Wave 1 (Tracer) da Fase 05 foi executada e concluída com êxito sob rigorosa disciplina TDD (RED-GREEN-REFACTOR), entregando o motor cirúrgico de mutação e persistência em árvore de nós YAML (`yaml.Node` da biblioteca `gopkg.in/yaml.v3`) em `internal/config`. A solução preserva 100% dos comentários existentes (cabeçalho, seções, linha e rodapé), implementa gravação atômica segura via arquivo temporário `.tmp` com `Sync()` e substituição POSIX `os.Rename`, cópia de backup preventiva `.bak`, inicialização transparente em primeiro acesso com template documentado (`DefaultConfigYAML`), diagnósticos ricos para sintaxe corrompida (`ConfigSyntaxError`) e consolidação definitiva dos helpers legados em `cmd/schedule.go` e `cmd/whitelist.go`, eliminando a vulnerabilidade de escrita não atômica `IN-04` sem CGO (`NFR-01`) e com segurança inviolável (`NFR-03`).

---

## Tarefas Executadas

### Tarefa 1 [TDD]: Primitivas de Mutação Cirúrgica em AST YAML (`internal/config/mutator.go`, `internal/config/mutator_test.go`)
- **RED**:
  - Criada bateria de testes em `internal/config/mutator_test.go` cobrindo `TestMutator_CommentPreservation_Scalars`, `TestMutator_CommentPreservation_Sequences` e `TestMutator_Idempotency`.
  - Verificada a falha de compilação por ausência dos métodos (`undefined: GetMappingNode`, etc.).
  - Commit: `cf0c807` (`test(05-01): add unit tests for AST YAML mutator primitives`).
- **GREEN**:
  - Implementado `internal/config/mutator.go`:
    - `GetMappingNode(root *yaml.Node) *yaml.Node`: extração segura do nó raiz de mapeamento de documentos YAML, inicializando nó de mapeamento quando vazio.
    - `FindOrCreateMapping(mapNode *yaml.Node, key string) *yaml.Node`: travessia e criação on-demand de mapeamentos aninhados.
    - `FindSequence(mapNode *yaml.Node, key string) *yaml.Node`: busca de sequências existentes.
    - `FindOrCreateSequence(mapNode *yaml.Node, key string) *yaml.Node`: busca ou criação de nós de sequência com estilo canônico.
    - `SetScalarValue(mapNode *yaml.Node, key, value, tag string)`: atualização cirúrgica em nós escalares preservando comentários intactos, com inferência automática de tags (`!!bool`, `!!int`, `!!str`).
    - `SetSequenceValues(mapNode *yaml.Node, key string, values []string)`: substituição atômica de elementos de sequência preservando comentários do nó.
    - `AppendUniqueToSequence(seqNode *yaml.Node, val string) bool`: anexação idempotente sem duplicatas.
    - `RemoveFromSequence(seqNode *yaml.Node, targets ...string) int`: remoção filtrada de múltiplos alvos.
  - Testes passando com 100% de cobertura em todas as 8 funções de `mutator.go`.
  - Commit: `00044e5` (`feat(05-01): implement AST YAML surgical mutation primitives`).
- **REFACTOR**:
  - Suporte completo a tags primitivas e cobertura de casos de borda (nós nulos, nós escalares convertidos em mapeamento ou sequência).

### Tarefa 2 [TDD]: Gravação Atômica Segura (`SaveConfigNode`), Backup Preventivo (`.bak`), Resolução de Primeiro Acesso e Diagnóstico de Sintaxe Corrompida (`ConfigSyntaxError`)
- **RED**:
  - Adicionados testes em `internal/config/mutator_test.go` para `TestMutator_AtomicWriteAndBackup`, `TestMutator_FirstRunDefaults`, `TestMutator_CorruptedYAMLSyntax`, `TestMutator_LoadWithAST_EmptyAndError` e `TestMutator_SaveConfigNode_Errors`.
  - Verificada falha de compilação.
  - Commit: `8b71532` (`test(05-01): add tests for atomic save, backup, defaults, and syntax errors`).
- **GREEN**:
  - Implementado `internal/config/save.go`:
    - `type ConfigSyntaxError struct`: enriquecido com caminho do arquivo, erro original do parser e flag de detecção de backup `.bak`, formatando instruções explícitas de restauração via `cp <Path>.bak <Path>`.
    - `const DefaultConfigYAML`: template YAML completo e documentado cobrindo os 18 ecossistemas, caminhos de varredura/exclusão, safety/lixeira, watch daemon e scheduler.
    - `LoadWithAST(configFile string) (*yaml.Node, *Config, error)`: carregamento transparente com fallback para `DefaultConfigYAML` quando o arquivo não existe (D-11), captura diagnóstica de sintaxe corrompida (D-12) e validação da estrutura contra `Config`.
    - `SaveConfigNode(configFile string, root *yaml.Node) error`: pipeline atômico com backup preventivo `.bak` em arquivos preexistentes, encode YAML indentado a 2 espaços, escrita em staging `.tmp`, flush de buffers do sistema operacional via `tmpFile.Sync()`, fechamento explícito do descritor de arquivo e substituição atômica via `os.Rename(tmp, dst)` (D-10, T-05-01).
  - Testes passando integralmente com `CGO_ENABLED=0`.
  - Commit: `6cfc981` (`feat(05-01): implement atomic save, backup copy, first-run defaults, and syntax diagnostics`).

### Tarefa 3: Unificação de Helpers AST em `cmd/schedule.go` e `cmd/whitelist.go` e Eliminação de Vulnerabilidade de Escrita Direta (IN-04)
- **Refatoração de `cmd/schedule.go`**:
  - `updateSchedulerArgs` refatorado para consumir `config.LoadWithAST`, `config.GetMappingNode`, `config.FindOrCreateMapping`, `config.SetSequenceValues` e `config.SaveConfigNode`.
  - Funções duplicadas `findOrCreateMapping` e `setSequenceValues` removidas de `cmd/schedule.go`.
- **Refatoração de `cmd/whitelist.go`**:
  - Comandos `whitelist add` e `whitelist remove` refatorados para consumir `config.LoadWithAST`, `config.GetMappingNode`, `config.FindOrCreateSequence`, `config.FindSequence`, `config.AppendUniqueToSequence`, `config.RemoveFromSequence` e `config.SaveConfigNode`.
  - Funções locais redundantes `loadOrCreateConfigNode`, `getMappingNode`, `findSequence`, `findOrCreateSequence`, `appendUnique`, `removeFromSequence` e `saveConfigNode` removidas.
  - Eliminação definitiva da vulnerabilidade `IN-04`: todas as operações de persistência de CLI agora utilizam o pipeline atômico com `.tmp` e backup `.bak`.
  - Validação estrita: suíte de testes de `cmd` (`TestSchedule*` e `TestWhitelist*`) executada com sucesso total sem regressões.
  - Commit: `de3a82c` (`refactor(05-01): unify AST YAML helpers in cmd/schedule and cmd/whitelist into internal/config`).

---

## Verificação e Conformidade

- **Zero CGO (`NFR-01`)**:
  - `CGO_ENABLED=0 go test ./internal/config` (PASS)
  - `CGO_ENABLED=0 go test -v -count=1 ./cmd -run "TestSchedule|TestWhitelist"` (PASS)
  - `CGO_ENABLED=0 go build ./...` (PASS)
- **Metas de Cobertura (`NFR-02`)**:
  - `internal/config/mutator.go`: 100.0% de cobertura.
  - `internal/config/save.go`: 80.2% de cobertura.
  - Pacote `internal/config`: suíte completa de testes passando.
- **Proteções de Segurança (`NFR-03`)**:
  - `T-05-01` (Tampering / DoS por escrita não atômica): Mitigado com staging `.tmp`, `Sync()` e `os.Rename`.
  - `T-05-02` (Crash por corrupção de YAML): Mitigado com `ConfigSyntaxError` contendo diagnóstico de linha/coluna e orientação de restauração `.bak`.
  - `IN-04` (Pendência técnica de escrita direta no whitelist): Resolvido em `cmd/whitelist.go` e `cmd/schedule.go`.

---

## Self-Check: PASSED
- `internal/config/mutator.go` criado e testado: SIM
- `internal/config/save.go` criado e testado: SIM
- `internal/config/mutator_test.go` criado com cobertura completa: SIM
- `cmd/schedule.go` e `cmd/whitelist.go` refatorados sem duplicação: SIM
- Commits atômicos realizados conforme instruções: SIM
- `CGO_ENABLED=0 go build ./...` e `go test ./...` passando: SIM

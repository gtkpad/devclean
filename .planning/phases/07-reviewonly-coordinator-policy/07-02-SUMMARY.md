---
phase: "07-reviewonly-coordinator-policy"
plan: "07-02"
title: "DryRun Zero-Mutation Invariant & Symlink/Subprocess Security Perimeter"
wave: 2
completed_at: "2026-10-15"
requirements:
  - POL-04
status: "completed"
commits:
  - hash: "cb559c6"
    message: "test(deleter): add tests for dryrun trash isolation and caller side-effect elimination"
  - hash: "7d9d8df"
    message: "feat(deleter): centralize trash maintenance in Coordinator and eliminate caller leakage in clean cmd"
  - hash: "b9e8dbc"
    message: "test(safety): add tests for canonical symlink evaluation in blacklist and coordinator"
  - hash: "0ab27bb"
    message: "feat(safety): enforce canonical symlink evaluation in blacklist and coordinator Stage 1"
  - hash: "fdc15b0"
    message: "test(cleaner): add canary test and shell injection audit tests"
  - hash: "f68426a"
    message: "feat(cleaner): implement AST-based subprocess shell injection audit and canary verification suite"
coverage:
  internal/deleter: "89.7%"
  internal/safety: "97.7%"
  internal/cleaner: "77.8%"
---

# Plan 07-02 Summary: DryRun Zero-Mutation Invariant & Symlink/Subprocess Security Perimeter

## Overview
O plano `07-02` da Fase 07 foi concluído com sucesso em estrita conformidade com o protocolo TDD (RED-GREEN), satisfazendo o requisito `POL-04` e consolidando o perímetro de segurança e garantias de mutação zero do DevClean v2.3.

### Principais Entregas
1. **Centralização e Encapsulamento de Efeitos Colaterais (POL-04, D-13):**
   - Removidas as chamadas externas de manutenção de lixeira (`trashMgr.EnforceRetention` e `trashMgr.EnforceMaxSize`) em `cmd/clean.go`, eliminando qualquer vazamento de mutação no lado do chamador.
   - Toda a manutenção de lixeira, expurgo por retenção e aplicação de limites de cota em bytes (`EnforceMaxSize`) passa a ser orquestrada unicamente dentro de `deleter.Coordinator` sob a guarda rigorosa `if !opts.DryRun`.
   - Todos os estágios com efeitos colaterais (Stage 3 pre-hooks, Stage 4 sinks de deleção, Stage 5 post-hooks, Stage 6 persistência de histórico JSONL e métricas de agendador, Stage 6.1 manutenção de lixeira) são estritamente condicionados a `!opts.DryRun`.

2. **Avaliação Canônica de Symlinks e Defesa contra Path Traversal (POL-04, D-16):**
   - Implementada resolução canônica de symlinks via `filepath.EvalSymlinks` em `safety.IsCoreBlacklisted` e `safety.Blacklist.IsBlacklisted`, assegurando que links simbólicos apontando para `.git`, `/System` ou caminhos protegidos configurados sejam sumariamente rejeitados.
   - Reforçado o Estágio 1 do `deleter.Coordinator` com o helper `isSymlinkToBlacklist`, garantindo defesa em profundidade tanto para alvos diretos quanto resolvidos.

3. **Suíte Canário de Mutação Zero e Auditoria de Subprocessos (POL-04, D-15, D-16):**
   - Implementado teste canário hermético `TestCoordinator_DryRun_ZeroMutation_Canary` em `internal/deleter/policy_test.go` verificando que arquivos canário mantêm SHA-256 e mtime inalterados, a pasta de lixeira não recebe batches ou manifests, o histórico JSONL permanece intacto e o agendador não registra execuções durante `DryRun`.
   - Implementada auditoria estática baseada em AST `cleaner.AuditNoShellInjection` e `TestProviders_NoShellInjection` em `internal/cleaner/cleaner_test.go`, comprovando que 100% dos cleaners com invocação de comandos utilizam fatias de argumentos tokenizadas via `exec.CommandContext` ou `cleaner.CommandExecutor`, sem interpolação de strings em shell (`sh -c` ou `bash -c`).

---

## Tarefas Executadas

### Task 07-02-01 [TDD]: Encapsulate all side effects in Coordinator & eliminate external caller leakage in cmd/clean.go
- **RED**: Adicionados `TestCoordinator_DryRun_NoTrashPrune` em `internal/deleter/policy_test.go` e `TestCleanCmd_DryRun_NoSideEffects` em `cmd/clean_test.go`, falhando na presença de chamadas externas de manutenção em `cmd/clean.go`.
  - Commit: `cb559c6` `test(deleter): add tests for dryrun trash isolation and caller side-effect elimination`
- **GREEN**:
  - Removidas as chamadas `trashMgr.EnforceRetention` e `trashMgr.EnforceMaxSize` em `cmd/clean.go`, delegando a manutenção de lixeira exclusivamente ao Coordinator.
  - Testes passaram com sucesso comprovando o isolamento de efeitos colaterais.
  - Commit: `7d9d8df` `feat(deleter): centralize trash maintenance in Coordinator and eliminate caller leakage in clean cmd`

### Task 07-02-02 [TDD]: Canonical symlink evaluation against blacklist & path traversal defense
- **RED**: Adicionados `TestBlacklist_SymlinkEvaluation` em `internal/safety/blacklist_test.go` e `TestCoordinator_BlacklistSymlink` em `internal/deleter/policy_test.go`, falhando porque `IsCoreBlacklisted` e `IsBlacklisted` não resolviam links canônicos.
  - Commit: `b9e8dbc` `test(safety): add tests for canonical symlink evaluation in blacklist and coordinator`
- **GREEN**:
  - Implementada resolução canônica em `internal/safety/blacklist.go` com `filepath.EvalSymlinks`.
  - Integrado helper `isSymlinkToBlacklist` no Estágio 1 de `internal/deleter/deleter.go`.
  - Testes de symlinks passaram com 100% de sucesso.
  - Commit: `0ab27bb` `feat(safety): enforce canonical symlink evaluation in blacklist and coordinator Stage 1`

### Task 07-02-03 [TDD]: DryRun zero-mutation canary verification test suite & subprocess injection audits
- **RED**: Adicionados `TestCoordinator_DryRun_ZeroMutation_Canary` em `internal/deleter/policy_test.go` e `TestProviders_NoShellInjection` em `internal/cleaner/cleaner_test.go`, falhando no stub da auditoria de subprocessos.
  - Commit: `fdc15b0` `test(cleaner): add canary test and shell injection audit tests`
- **GREEN**:
  - Implementado `AuditNoShellInjection` em `internal/cleaner/security.go` inspecionando a AST de todos os pacotes de cleaners para vetar interpretadores shell (`sh`, `bash`, `-c`).
  - Teste canário e auditoria de subprocessos passaram hermeticamente.
  - Commit: `f68426a` `feat(cleaner): implement AST-based subprocess shell injection audit and canary verification suite`

---

## Verificação e Cobertura
- **Testes Automatizados:**
  - `go test -v ./internal/deleter ./cmd ./internal/safety ./internal/cleaner` aprovado com 100% de sucesso.
  - Suíte completa do projeto (`go test ./...`) validada com zero falhas.
- **Cobertura de Código:**
  - `internal/deleter`: 89.7% dos statements
  - `internal/safety`: 97.7% dos statements
  - `internal/cleaner`: 77.8% dos statements
- **Invariantes Verificados:**
  1. `DryRun: true` produz zero mutação em arquivos canário, lixeira, histórico JSONL e agendador.
  2. `cmd/clean.go` possui zero chamadas externas de manutenção de lixeira.
  3. Symlinks apontando para pastas protegidas são rejeitados no Estágio 1.
  4. Todos os comandos externos utilizam argumentos tokenizados diretos.

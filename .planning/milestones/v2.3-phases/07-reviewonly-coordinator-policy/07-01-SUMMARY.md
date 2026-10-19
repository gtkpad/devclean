---
phase: "07-reviewonly-coordinator-policy"
plan: "07-01"
title: "Domain ReviewOnly Metadata & Coordinator Stage 1 Policy Gate"
wave: 1
completed_at: "2026-10-15"
requirements:
  - POL-01
  - POL-02
status: "completed"
commits:
  - hash: "591f35b"
    message: "test(domain): add unit and policy tests for ReviewOnly metadata and tracer rejection"
  - hash: "6b8336b"
    message: "feat(domain): implement Target ReviewOnly metadata and Coordinator Stage 1 rejection"
  - hash: "46d759a"
    message: "test(deleter): add tests for 100% ReviewOnly batch and mixed batch processing"
  - hash: "a132f3b"
    message: "test(deleter): enforce strict expunge lock and single gate invariants across execution modes"
coverage:
  internal/domain: "100.0%"
  internal/deleter: "89.6%"
---

# Plan 07-01 Summary: Domain ReviewOnly Metadata & Coordinator Stage 1 Policy Gate

## Overview
O plano `07-01` da Fase 07 foi executado com sucesso em estrita conformidade com o protocolo TDD (RED-GREEN), satisfazendo os requisitos `POL-01` e `POL-02` e estabelecendo o perímetro inviolável de segurança do `deleter.Coordinator`.

### Principais Entregas
1. **Modelagem de Domínio e Metadados de Revisão (POL-01, D-01, D-02, D-04):**
   - Adicionados os campos planos `ReviewOnly bool` (`json:"review_only"`) e `ReviewReason string` (`json:"review_reason,omitempty"`) à struct `domain.Target`.
   - Criadas as constantes padronizadas de razões semânticas: `ReviewReasonSharedStore` ("shared_store"), `ReviewReasonActiveProcess` ("active_process"), `ReviewReasonOrphanLeftover` ("orphan_leftover"), `ReviewReasonSystemProtected` ("system_protected") e `ReviewReasonSessionSensitive` ("session_sensitive").
   - Implementado o método auxiliar `(t Target) IsReviewOnly() bool`.
   - Garantida a compatibilidade regressiva de serialização JSON com payloads legados e a regra estrutural de que alvos `ReviewOnly` possuem `EligibleBytes == 0` e `TTLEligible == false`.

2. **Interceptação Central no Estágio 1 do Coordinator (POL-02, D-05, D-06):**
   - O `deleter.Coordinator.Delete()` inspeciona todos os alvos no Estágio 1, antes de qualquer particionamento (filesystem vs virtual), hooks pré-deleção ou acionamento de sinks de lixeira ou deleção direta.
   - Alvos com `ReviewOnly == true` são retidos em `res.SkippedReviewOnly []domain.Target` e omitidos de `safeTargets`.
   - O callback `opts.OnWarning` é acionado para cada item retido, formatando a razão semântica (com fallback gracioso para `shared_store` se vazia), caminho e tamanho legível.

3. **Finalização Graciosa com Lote 100% ReviewOnly (POL-02, D-07):**
   - Lotes contendo unicamente alvos `ReviewOnly` (ou protegidos por blacklist) completam o ciclo de vida sem erros (`err == nil`), retornando `res.TotalCount == 0`, `res.TotalFreed == 0`, `res.Duration > 0` e a lista `SkippedReviewOnly` integralmente preenchida.

4. **Travamento Estrito de Expurgo e Invariante de Portão Único (POL-02, D-03, D-08):**
   - `deleter.Options` não expõe nenhuma flag de override para ignorar `ReviewOnly`.
   - Todos os modos de execução (`ModeAuto`, `ModeForce`, `ModeDirect` com `UseTrash: false`, `ModeTUI`, `ModeMCP` e `DryRun`) rejeitam incondicionalmente alvos `ReviewOnly` no Estágio 1.
   - Pre-delete hooks, trash sinks e cleaners de ecossistema nunca são acionados para alvos sob revisão.

---

## Tarefas Executadas

### Task 07-01-01 [TDD]: End-to-end tracer slice: Domain Target ReviewOnly metadata & Coordinator Stage 1 rejection
- **RED**: Criados `internal/domain/target_test.go` e o teste tracer em `internal/deleter/policy_test.go` verificando serialização JSON, decodificação retrocompatível, invariantes de `EligibleBytes == 0` e a recusa no Estágio 1 com `res.SkippedReviewOnly`.
  - Commit: `591f35b` `test(domain): add unit and policy tests for ReviewOnly metadata and tracer rejection`
- **GREEN**:
  - Implementadas constantes `ReviewReason*`, campos `ReviewOnly` e `ReviewReason`, e método `IsReviewOnly()` em `internal/domain/types.go`.
  - Adicionado `SkippedReviewOnly` ao `deleter.Result` e filtro no Estágio 1 com avaliação canônica de symlinks e disparo de `opts.OnWarning` em `internal/deleter/deleter.go`.
  - Commit: `6b8336b` `feat(domain): implement Target ReviewOnly metadata and Coordinator Stage 1 rejection`

### Task 07-01-02 [TDD]: Coordinator Stage 1 edge cases: 100% ReviewOnly batch graceful exit & warning dispatch
- **Testes & Validação**:
  - Implementados `TestCoordinator_AllReviewOnlyBatch` e `TestCoordinator_MixedBatch_ReviewOnly` em `internal/deleter/policy_test.go`.
  - Comprovado que lotes puros de revisão não criam partições vazias, retornam `TotalCount: 0`, `TotalFreed: 0` e `err == nil`.
  - Comprovado que lotes mistos removem alvos elegíveis enquanto preservam intactos os arquivos dos alvos de revisão no disco.
  - Commit: `46d759a` `test(deleter): add tests for 100% ReviewOnly batch and mixed batch processing`

### Task 07-01-03 [TDD]: Enforce strict expunge lock & single gate invariant across execution modes
- **Testes & Validação**:
  - Implementado `TestCoordinator_StrictExpungeLock` assegurando que hooks pré-deleção e sinks nunca são tocados para itens protegidos.
  - Implementado `TestCoordinator_SingleGateModes` cobrindo a matriz completa de modos (`Auto`, `Force`, `Trash`, `Direct`, `TUI`, `MCP`, `DryRun`).
  - Commit: `a132f3b` `test(deleter): enforce strict expunge lock and single gate invariants across execution modes`

---

## Verificação e Cobertura
- **Testes Automatizados:**
  `go test -v ./internal/domain ./internal/deleter` aprovado com 100% de sucesso.
- **Cobertura de Código:**
  - `internal/domain`: 100.0% dos statements
  - `internal/deleter`: 89.6% dos statements
- **Regressão Global:**
  Suíte completa do projeto (`go test ./...`) validada com zero falhas.

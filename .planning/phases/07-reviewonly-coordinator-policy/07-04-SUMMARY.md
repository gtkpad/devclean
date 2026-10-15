---
phase: "07-reviewonly-coordinator-policy"
plan: "07-04"
title: "MCP Protocol Parity for ReviewOnly Targets & Safety Guarantees"
wave: 3
completed_at: "2026-10-15"
requirements:
  - POL-03
status: "completed"
commits:
  - hash: "271650a"
    message: "test(mcp): add tests for ReviewOnly propose status, explain policy, and execute rejection"
  - hash: "ce8a085"
    message: "feat(mcp): implement ReviewOnly proposal status, explain policy, and execute protection"
coverage:
  internal/mcp: "75.7%"
---

# Plan 07-04 Summary: MCP Protocol Parity for ReviewOnly Targets & Safety Guarantees

## Overview
O plano `07-04` da Fase 07 foi concluído com sucesso em estrita conformidade com o protocolo TDD (RED-GREEN), satisfazendo o requisito `POL-03` e estabelecendo paridade completa das ferramentas MCP (Model Context Protocol) para clientes de IA e agentes autônomos.

### Principais Entregas
1. **Status `review_only` e Metadados Ricos em `propose_deletion` (POL-03, D-11):**
   - Estrutura `proposalEntry` estendida com os campos `EligibleBytes int64` (`json:"eligible_bytes"`) e `ReviewReason string` (`json:"review_reason,omitempty"`).
   - Quando um alvo possui `ReviewOnly == true`, o handler `handlePropose` marca o item com `status: "review_only"`, preserva `size_bytes`, fixa incondicionalmente `eligible_bytes: 0` e preenche `reason` com uma justificativa padronizada: `Item de revisão [<reason>]: requer ação manual ou ferramenta dedicada`.
   - Alvos regulares aprovados propagam seu valor de `EligibleBytes`.

2. **Serialização Transparente em `scan_environment` (POL-03, D-11):**
   - A saída de varredura `scan_environment` serializa os campos `review_only: true` e `review_reason` diretamente a partir de `domain.Target` para consumo dos agentes MCP.

3. **Documentação e Explicação da Política em `explain_target` (POL-03, D-11):**
   - O tipo `ExplainResult` foi enriquecido com `ReviewOnly bool` (`json:"review_only,omitempty"`) e `ReviewReason string` (`json:"review_reason,omitempty"`).
   - O `DefaultExplainer.Explain` e o handler `handleExplainTarget` identificam alvos sob revisão (seja por inspeção de padrões canônicos de stores compartilhados ou pelo catálogo de alvos ativos da engine), retornando `status: "review_only"`, `review_only: true` e justificativa clara apontando a recusa de exclusão automática pelo DevClean e a necessidade de ação manual ou ferramentas dedicadas (ex: `docker system prune`, `orb clean`).

4. **Defesa em Profundidade em `execute_approved_deletion` (POL-02, D-08):**
   - A ferramenta de execução MCP delega exclusivamente para `Coordinator.Delete()`.
   - Mesmo que um cliente ou modelo passe IDs de itens de revisão, o Estágio 1 do Coordinator os intercepta e rejeita incondicionalmente, garantindo zero mutação nesses itens no disco ou em daemons de containers.
   - O retorno de `execute_approved_deletion` inclui `skipped_review_count` quando itens de revisão são detectados e retidos.

---

## Tarefas Executadas

### Task 07-04-01 [TDD]: MCP protocol parity: propose_deletion review_only status, scan serialization & explain policy
- **RED**: Criado `internal/mcp/review_test.go` com suíte de 4 testes (`TestMCP_ReviewOnly_Propose`, `TestMCP_ReviewOnly_Scan`, `TestMCP_ReviewOnly_Explain`, `TestMCP_ReviewOnly_ExecuteRejection`), falhando nos testes de `propose_deletion` e `explain_target`.
  - Commit: `271650a` `test(mcp): add tests for ReviewOnly propose status, explain policy, and execute rejection`
- **GREEN**:
  - Em `internal/mcp/server.go`, atualizado `proposalEntry` com `EligibleBytes` e `ReviewReason`, adicionada lógica de bifurcação para `ReviewOnly` no `handlePropose` e métricas de alvos retidos no `handleExecute`.
  - Em `internal/mcp/explain.go`, estendido `ExplainResult` com `ReviewOnly` e `ReviewReason`, adicionada detecção de stores sob revisão no `DefaultExplainer` e enriquecimento no `handleExplainTarget`.
  - Commit: `ce8a085` `feat(mcp): implement ReviewOnly proposal status, explain policy, and execute protection`

---

## Verificação e Cobertura
- **Testes Automatizados:**
  - `go test -v ./internal/mcp -run "TestMCP_ReviewOnly"` aprovado com 100% de sucesso.
  - Suíte completa do pacote `internal/mcp` (`go test -v ./internal/mcp`) aprovada com 100% de sucesso (25 testes).
  - Suíte completa do projeto (`go test ./...`) validada com zero falhas.
- **Cobertura de Código:**
  - `internal/mcp`: 75.7% dos statements
- **Invariantes Verificados:**
  1. `propose_deletion` retorna `status: "review_only"`, `eligible_bytes: 0` e motivo explicativo para itens sob revisão.
  2. `scan_environment` serializa `review_only` e `review_reason` no JSON de alvos.
  3. `explain_target` retorna `status: "review_only"`, `review_only: true` e orienta sobre ações manuais ou ferramentas nativas dedicadas.
  4. `execute_approved_deletion` rejeita alvos de revisão no Estágio 1 do Coordinator, mantendo os alvos intactos e retornando `deleted_count: 0`.

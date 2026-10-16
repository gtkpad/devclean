---
phase: "09-ai-electron-editors-active-versions"
plan: "09-03"
title: "Claude CLI Version Discovery, Active Symlink Resolution, and Fail-Closed Guard"
wave: 3
completed_at: "2026-10-15"
requirements:
  - AI-03
status: "completed"
commits:
  - hash: "135baa7"
    message: "test(09-03): add failing unit tests for claude active version preservation"
  - hash: "a210e02"
    message: "feat(09-03): implement claude active version resolution and preservation"
  - hash: "56ab63f"
    message: "test(09-03): add failing unit tests for claude fail-closed symlink policies"
  - hash: "1430198"
    message: "feat(09-03): enforce fail-closed policy for missing, broken, and ambiguous symlinks"
coverage:
  internal/cleaner/ai: "85.1%"
---

# Plan 09-03 Summary: Claude CLI Version Discovery, Active Symlink Resolution, and Fail-Closed Guard

## Overview
O plano `09-03` da Fase 09 implementou a descoberta de versões do Claude Code CLI, a resolução canônica do symlink ativo para preservação da versão em uso e a política estrita fail-closed contra symlinks ausentes, quebrados ou ambíguos, em conformidade com o requisito **AI-03**, decisões de design **D-09**, **D-10**, **D-12**, **D-16** e mitigações do modelo de ameaças (**T-09-07**, **T-09-08**, **T-09-09**).

---

## Principais Entregas

### 1. Descoberta de Versões do Claude CLI e Resolução Canônica do Symlink Ativo (AI-03, D-09, D-12, T-09-07)
- Em `internal/cleaner/ai/claude.go`, definidos a estrutura `claudeVersionsResult` e o resolvedor:
  `resolveActiveClaudeVersion(home string) claudeVersionsResult`
- Inspeciona diretórios de versões em `~/.local/share/claude/versions/*` e `~/.claude/local`.
- Avalia os links simbólicos de invocação ativos:
  - `~/.local/bin/claude`
  - `~/.local/share/claude/current`
  - `~/.local/share/claude/versions/current`
  - `~/.local/share/claude/claude`
- Resolve o destino real utilizando `filepath.EvalSymlinks` e normalização canônica para evitar discrepâncias em caminhos de arquivos (incluindo links de sistema como `/var` -> `/private/var` no macOS).
- **Invariante Crítica:** A versão ativa correspondente ao destino resolvido do symlink é estritamente protegida e **jamais emitida** como candidata à deleção.
- Versões inativas confirmadas recebem a categoria `"ai-claude-versions"`, nível de risco `domain.RiskLow` (D-12) e são elegíveis conforme o TTL configurado (`MaxAgeDays`), mantendo relato integral do espaço ocupado em `SizeBytes` (SCAN-01, D-16).

### 2. Política Fail-Closed em Symlinks Ausentes, Quebrados ou Ambíguos (AI-03, D-10, T-09-08)
- Em `internal/cleaner/ai/claude.go`, na função `scanClaudeVersions`:
  - **Symlink Ausente (`res.missingSymlink`):** Quando nenhum symlink de invocação é encontrado, todas as versões descobertas recebem `ReviewOnly: true`, `ReviewReason: domain.ReviewReasonActiveVersion`, `EligibleBytes: 0` e `TTLEligible: false`.
  - **Symlink Quebrado (`res.brokenSymlink`):** Se o link simbólico aponta para um destino inexistente, todas as versões recebem `ReviewOnly: true`, `ReviewReason: domain.ReviewReasonAmbiguous`, `EligibleBytes: 0` e `TTLEligible: false`.
  - **Symlinks Ambíguos (`res.ambiguous`):** Se múltiplos symlinks apontam para destinos conflitantes ou se o destino não corresponde a nenhuma versão identificada, todas as versões recebem `ReviewOnly: true`, `ReviewReason: domain.ReviewReasonAmbiguous`, `EligibleBytes: 0` e `TTLEligible: false`.
  - Em todos os cenários fail-closed, o tamanho real ocupado em disco continua sendo relatado com precisão em `SizeBytes` (SCAN-01).

### 3. Preservação de Arquivos de Estado e Assinatura Consistente (T-09-09)
- Arquivos de estado e credenciais do Claude (`config.json`, `history.jsonl`, `settings.json`, `auth.json`, `credentials.json`) permanecem estritamente intocados.
- `scanClaude` adota a assinatura estável `(ctx context.Context, exec cleaner.CommandExecutor, home string, rules domain.EcosystemRules) ([]domain.Target, error)`.
- O método `c.Scan` em `internal/cleaner/ai/ai.go` passa `c.homeDir` diretamente para `scanClaude`, evitando corrupção de caminhos e integrando perfeitamente a varredura de `~/.claude` (caches/logs) com `~/.local/share/claude/versions`.
- A suíte de testes legada `TestClaude` em `internal/cleaner/ai/ai_test.go` foi adaptada para a nova assinatura sem perda de cobertura.

---

## Tarefas Executadas

### Task 09-03-01 [TDD]: Canonical Active Version symlink resolution and inactive version discovery (AI-03, D-09, D-12)
- **RED**: Criado `internal/cleaner/ai/claude_test.go` com `TestClaude_ActiveVersion_Preserved` e `TestClaude_ProtectedFiles_Untouched`, atualizando `ai.go` e `ai_test.go` para a assinatura de `scanClaude`.
  - **Commit RED**: `135baa7` `test(09-03): add failing unit tests for claude active version preservation`
- **GREEN**: Implementados `claudeVersionsResult`, `resolveActiveClaudeVersion` e `scanClaudeVersions` em `internal/cleaner/ai/claude.go` com preservação incondicional da versão ativa.
  - **Commit GREEN**: `a210e02` `feat(09-03): implement claude active version resolution and preservation`

### Task 09-03-02 [TDD]: Fail-closed policy for missing, broken, or ambiguous symlinks (AI-03, D-10)
- **RED**: Adicionados testes `TestClaude_MissingSymlink_FailClosed`, `TestClaude_BrokenSymlink_FailClosed` e `TestClaude_AmbiguousSymlink_FailClosed` em `internal/cleaner/ai/claude_test.go`.
  - **Commit RED**: `56ab63f` `test(09-03): add failing unit tests for claude fail-closed symlink policies`
- **GREEN**: Implementada a política fail-closed em `scanClaudeVersions` aplicando `ReviewOnly: true`, `EligibleBytes: 0`, `TTLEligible: false` e razões semânticas explícitas (`active_version` ou `ambiguous_state`).
  - **Commit GREEN**: `1430198` `feat(09-03): enforce fail-closed policy for missing, broken, and ambiguous symlinks`

---

## Mitigações de Ameaças Verificadas (STRIDE)

| Threat ID | Descrição | Componente | Mitigação Aplicada |
|---|---|---|---|
| **T-09-07** | Tampering / Usability Loss | `scanClaudeVersions` | Resolução canônica de links de invocação com `filepath.EvalSymlinks`; versão ativa é omitida dos alvos de expurgo. |
| **T-09-08** | Tampering / Ambiguity Bypass | `scanClaudeVersions` | Política fail-closed: se o link simbólico estiver ausente, quebrado ou ambíguo, aplica `ReviewOnly: true`, `EligibleBytes: 0` e razões explícitas. |
| **T-09-09** | Tampering / Session Loss | `scanClaude` | Proteção absoluta de `config.json`, `history.jsonl`, `settings.json`, `auth.json` e `credentials.json` contra inclusão em alvos. |

---

## Verificação e Qualidade
- `go test -v ./internal/cleaner/ai -run "TestClaude"`: PASS (6/6 testes passando com asserções completas).
- `go test -v -race ./internal/cleaner/ai/...`: PASS (16/16 testes passando sem race conditions).
- Cobertura de código do pacote `internal/cleaner/ai`: **85.1%** dos statements (> 85% meta atingida).
- `go test ./...`: PASS em todos os 39 pacotes do repositório.
- Compilação cruzada zero-CGO (`CGO_ENABLED=0`) confirmada em Darwin e Linux.

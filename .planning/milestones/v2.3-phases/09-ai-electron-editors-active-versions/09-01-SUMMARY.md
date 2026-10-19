---
phase: "09-ai-electron-editors-active-versions"
plan: "09-01"
title: "Domain Review Reasons, CommandExecutor Injection, and 8-Editor Leaves Scanner with Functional Grouping"
wave: 1
completed_at: "2026-10-15"
requirements:
  - AI-01
status: "completed"
commits:
  - hash: "aa38ddf"
    message: "test(domain,ai): add tests for review reasons and AI cleaner executor injection"
  - hash: "ae37a95"
    message: "feat(ai,domain): add review reasons and executor injection to AI cleaner"
  - hash: "d47299b"
    message: "test(ai): add comprehensive tests for 8-editor Chromium/Electron leaves and functional grouping"
  - hash: "3da845c"
    message: "feat(ai): implement 8-editor Chromium/Electron leaves scanner with functional grouping"
coverage:
  internal/cleaner/ai: "79.9%"
  internal/domain: "92.3%"
---

# Plan 09-01 Summary: Domain Foundation and 8-Editor Leaves Scanner with Functional Grouping

## Overview
O plano `09-01` da Fase 09 estabeleceu as fundações de domínio, a injeção do executor de comandos e o scanner canônico de folhas regeneráveis Chromium/Electron para todos os 8 editores suportados (**VS Code**, **VS Code Insiders**, **Cursor**, **Antigravity**, **Windsurf**, **Claude Desktop**, **OpenCode** e **Zed**), consolidando os artefatos descobertos em grupos funcionais virtuais (`editor://`) com risco baixo (`RiskLow`), em estrita conformidade com os requisitos `AI-01`, decisões `D-01`, `D-02`, `D-03`, `D-04` e as regras de segurança `SCAN-01` e `D-16`.

---

## Principais Entregas

### 1. Razões Semânticas de Revisão no Domínio (D-10, POL-01)
- Em `internal/domain/types.go`, o bloco de constantes `Semantic Review Reasons` foi estendido com:
  - `ReviewReasonActiveVersion = "active_version"`
  - `ReviewReasonAmbiguous = "ambiguous_state"`
- Garantida a validação unitária em `internal/domain/target_test.go` (`TestTarget_ReviewReasons`).

### 2. Injeção de CommandExecutor no AI Cleaner e Bypass de Alvos Virtuais (D-05, Wave 1 Tracer)
- Em `internal/cleaner/ai/ai.go`, a struct `aiCleaner` agora contém o campo `exec cleaner.CommandExecutor`.
- Implementados os construtores:
  - `NewWithExecutor(exec cleaner.CommandExecutor, scanPaths []string) domain.Cleaner`
  - `NewWithExecutorAndHome(exec cleaner.CommandExecutor, scanPaths []string, homeDir string) domain.Cleaner`
  - Construtores legados `New` e `NewWithHome` delegam para `&cleaner.DefaultExecutor{}` com retrocompatibilidade garantida.
- Em `aiCleaner.Clean()`, alvos virtuais (`t.IsVirtual()`, como `editor://...`) são pulados com segurança antes de qualquer chamada a `os.RemoveAll`, prevenindo erros de exclusão física até a introdução do unlinking cirúrgico.
- Criado helper hermético `mockExecutor` em `internal/cleaner/ai/test_helpers_test.go` para simular `pgrep -x` com processos ativos, inativos e estados incertos.

### 3. Matriz de Definições dos 8 Editores e Descoberta por Presença em Disco (AI-01, D-02)
- Em `internal/cleaner/ai/editors.go`, implementada a struct `editorDef` e a matriz `getEditorDefinitions()` cobrindo:
  - **VS Code (`code`)**: Processos `["Code", "code"]`, AppSupport Darwin/Linux, Caches e ShipIt.
  - **VS Code Insiders (`code-insiders`)**: Processos `["Code - Insiders", "code-insiders"]`, AppSupport Darwin/Linux, Caches e ShipIt.
  - **Cursor (`cursor`)**: Processos `["Cursor", "cursor"]`, AppSupport Darwin/Linux, Caches e ShipIt.
  - **Antigravity (`antigravity`)**: Processos `["Antigravity", "antigravity"]`, AppSupport Darwin/Linux, Caches e ShipIt.
  - **Windsurf (`windsurf`)**: Processos `["Windsurf", "windsurf"]`, AppSupport Darwin/Linux, Caches e ShipIt.
  - **Claude Desktop (`claude-desktop`)**: Processos `["Claude", "claude-desktop"]`, AppSupport Darwin/Linux, Caches e ShipIt.
  - **OpenCode (`opencode`)**: Processos `["opencode", "OpenCode"]`, AppSupport Darwin/Linux, Caches.
  - **Zed (`zed`)**: Processos `["zed", "Zed"]`, estruturas nativas GPUI (`dev.zed.Zed` em Caches e Logs nativos em Darwin/Linux).

### 4. Scanner de Folhas Regeneráveis e Agrupamento Funcional (AI-01, D-01, D-04)
- **Caches Chromium & GPU (`editor-chromium-cache`, RiskLow):**
  - Varrimento de `Cache`, `Code Cache`, `GPUCache`, `DawnCache`, `DawnWebGPUCache`, `DawnGraphiteCache`, `Service Worker/CacheStorage`, `Service Worker/ScriptCache`, `CachedData`, `webviewCache`, `~/Library/Caches/<bundle-id>`, `*.ShipIt` e `~/.cache/<editor>`.
  - Agrupamento em alvo virtual único por editor: `editor://<id>/chromium-cache`.
- **Logs & Relatórios de Crash (`editor-logs`, RiskLow):**
  - Varrimento de subdiretórios `logs`, `Crashpad/completed`, `Crashpad`, `Sentry` e logs nativos do Zed.
  - Agrupamento em alvo virtual único por editor: `editor://<id>/logs`.

### 5. Proteção Inviolável de Arquivos de Configuração e Sessão
- Diretórios e arquivos protegidos (`settings.json`, `keybindings.json`, `snippets/`, `Cookies`, `Cookies-journal`, `Local Storage`, `state.vscdb`, `workspace.json`) são estritamente excluídos das folhas candidatas e jamais contabilizados em `SizeBytes`.

### 6. Desacoplamento Scan vs TTL (SCAN-01, D-16)
- 100% dos bytes ocupados em disco são reportados em `SizeBytes`.
- Apenas folhas com idade superior a `MaxAgeDays` (ou todas caso `MaxAgeDays == 0`) são contabilizadas em `EligibleBytes` e ativam `TTLEligible = true`.

---

## Tarefas Executadas

### Task 09-01-01 [TDD]: End-to-end tracer slice: Domain review reasons and CommandExecutor injection in AI cleaner
- **RED**: Adicionados testes `TestTarget_ReviewReasons` em `internal/domain/target_test.go` e `TestAICleaner_ConstructorsWithExecutor` em `internal/cleaner/ai/ai_test.go`. Criado `test_helpers_test.go` com `mockExecutor`. Falha confirmada por ausência das constantes e dos construtores.
  - **Commit RED**: `aa38ddf` `test(domain,ai): add tests for review reasons and AI cleaner executor injection`
- **GREEN**: Adicionados `ReviewReasonActiveVersion` e `ReviewReasonAmbiguous` em `internal/domain/types.go`. Implementados construtores com executor em `internal/cleaner/ai/ai.go` e proteção para alvos virtuais no `Clean()`.
  - **Commit GREEN**: `ae37a95` `feat(ai,domain): add review reasons and executor injection to AI cleaner`

### Task 09-01-02 [TDD]: 8-Editor Chromium/Electron leaves scanner and functional grouping foundation (AI-01)
- **RED**: Adicionados testes `TestEditors_8Editors_Presence`, `TestEditors_FunctionalGrouping`, `TestEditors_ProtectedPaths_Untouched` e `TestEditors_TTL_Decoupling` em `internal/cleaner/ai/editors_test.go`. Atualizados `TestEditors` e `TestAICleaner_FullScanAndClean` em `internal/cleaner/ai/ai_test.go`. Falha confirmada com ausência dos 8 editores e falta do agrupamento virtual.
  - **Commit RED**: `d47299b` `test(ai): add comprehensive tests for 8-editor Chromium/Electron leaves and functional grouping`
- **GREEN**: Implementada a matriz `getEditorDefinitions()`, os scanners `scanEditorChromiumCaches` e `scanEditorLogs`, e o scanner consolidado `scanEditors` em `internal/cleaner/ai/editors.go`.
  - **Commit GREEN**: `3da845c` `feat(ai): implement 8-editor Chromium/Electron leaves scanner with functional grouping`

---

## Verificação e Qualidade
- `go test -v ./internal/domain -run "TestTarget_ReviewReasons"`: PASS.
- `go test -v ./internal/cleaner/ai`: PASS (10/10 testes passando).
- `go test ./...`: PASS em toda a árvore do projeto com zero falhas ou regressões.

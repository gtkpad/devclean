---
phase: "09-ai-electron-editors-active-versions"
plan: "09-04"
title: "Fail-Closed Process Guard, Pre-Deletion Revalidation, and Zero-Mutation Policy Canary"
wave: 4
completed_at: "2026-10-15"
requirements:
  - AI-01
  - AI-02
  - AI-03
  - POL-01
  - POL-02
status: "completed"
commits:
  - hash: "9865ec7"
    message: "test(09-04): add failing unit tests for process guard and isolated blocking"
  - hash: "1a5d1a1"
    message: "feat(09-04): implement fail-closed process guard for editors and claude cli"
  - hash: "c0bc34a"
    message: "test(09-04): add failing unit tests for clean revalidation and virtual target cleanup"
  - hash: "387299d"
    message: "feat(09-04): implement pre-deletion process revalidation and surgical editor target cleanup"
  - hash: "08b8ec2"
    message: "feat(09-04): add zero-mutation dryrun policy canary and achieve >88% ai cleaner coverage"
coverage:
  internal/cleaner/ai: "88.2%"
---

# Plan 09-04 Summary: Fail-Closed Process Guard, Pre-Deletion Revalidation, and Zero-Mutation Policy Canary

## Overview
O plano `09-04` encerra a Fase 09 com a implementação do Process Guard fail-closed para os 8 editores suportados (VS Code, VS Code Insiders, Cursor, Antigravity, Windsurf, Claude Desktop, OpenCode, Zed) e para o Claude Code CLI com bloqueio isolado por ferramenta, a revalidação de processos pré-exclusão no ciclo de vida de `Clean()`, o expurgo cirúrgico de alvos virtuais `editor://`, a integração no registro do motor de cleaners, e o canário criptográfico de mutação zero (`deleter.Coordinator` sob DryRun). Todas as metas de qualidade foram atendidas com cobertura de 88.2% em `internal/cleaner/ai`, 100% dos testes aprovados no repositório e compilação zero-CGO hermética em Darwin e Linux.

---

## Principais Entregas

### 1. Process Guard Fail-Closed com Bloqueio Isolado (AI-01, AI-03, D-05, D-06, D-08, D-11, T-09-12)
- Criado `internal/cleaner/ai/process.go` com a função:
  `checkProcessStatus(ctx context.Context, exec cleaner.CommandExecutor, processNames ...string) (running bool, uncertain bool)`
  - Executa `pgrep -x <name>` sem interpolação de shell via `cleaner.CommandExecutor`.
  - Código de saída `0` com saída não vazia indica processo ativo (`running=true, uncertain=false`).
  - Código de saída `1` indica processo inativo (`running=false, uncertain=false`).
  - Qualquer erro inesperado (ferramenta ausente, permissão, código > 1 ou exec nulo) aciona a política fail-closed (`uncertain=true`).
- **Bloqueio Isolado por Ferramenta (D-06):**
  - Em `internal/cleaner/ai/editors.go`, `scanEditors` avalia o status do processo de forma independente para cada um dos 8 editores.
  - Se um editor estiver ativo ou incerto, apenas os alvos daquele editor recebem `ReviewOnly: true`, `ReviewReason: domain.ReviewReasonActiveProcess`, `EligibleBytes: 0` e `TTLEligible: false`; os demais editores inativos permanecem elegíveis para limpeza.
  - Em `internal/cleaner/ai/claude.go`, `scanClaude` avalia `pgrep -x claude`. Se o CLI estiver ativo, todos os alvos do Claude recebem `ReviewOnly: true` e `ReviewReason: domain.ReviewReasonActiveProcess`.
- Em `internal/engine/register.go`, atualizado `RegisterDefaultCleanersWithExecutor` para propagar o executor injetado via `ai.NewWithExecutor(exec, effectivePaths)`.

### 2. Revalidação Pré-Deleção e Expurgo Cirúrgico no Clean() (AI-01, AI-02, AI-03, D-07, T-09-10, T-09-11)
- Em `internal/cleaner/ai/ai.go`, no método `Clean(ctx, targets)`:
  - **Pre-Deletion Revalidation (D-07, T-09-10):** Antes de realizar qualquer mutação física, o cleaner revalida em tempo real o status dos processos do editor ou do Claude CLI via `checkProcessStatus`. Caso o processo tenha sido aberto pelo usuário após a fase de scan, a exclusão daquele alvo é imediatamente abortada para evitar corrupção de dados ou concorrência.
  - **Surgical Virtual Target Cleanup (T-09-11):** Intercepta URIs com prefixo `editor://` e delega para `cleanEditorVirtualTarget`:
    - `workspace-backups`: Remove estritamente arquivos `state.vscdb.backup`. O arquivo primário `state.vscdb` e o descritor `workspace.json` são protegidos incondicionalmente.
    - `extensions`: Remove exclusivamente pastas de extensões marcadas como `true` no `.obsolete` e `CachedExtensionVSIXs`. O arquivo `.obsolete` e extensões ativas permanecem intocados. Validações estritas contra symlinks e path traversal (`..`) são aplicadas.
    - `chromium-cache`: Expurga apenas as subpastas regeneráveis autorizadas (`Cache`, `Code Cache`, `GPUCache`, `DawnCache`, etc.).
    - `logs`: Expurga pastas de logs, `Crashpad/completed` e filas de erro `Sentry`.
  - Alvos físicos passam por validação de symlinks (`safety.IsSymlink`) e lista negra do sistema (`safety.IsCoreBlacklisted`).

### 3. Canário de Mutação Zero e Cobertura Criptográfica (POL-01, POL-02, VAL-01, VAL-02, T-09-13)
- Em `internal/deleter/policy_test.go`, adicionado o teste:
  `TestCoordinator_DryRun_ZeroMutation_Phase09(t *testing.T)`
  - Configura 6 alvos canário com hashes SHA-256 e timestamps fixados:
    1. Code Chromium Cache (`data.bin`)
    2. Cursor Workspace Storage (`state.vscdb` primário e `state.vscdb.backup`)
    3. Windsurf Extensions (`.obsolete` e `package.json`)
    4. Claude CLI Inactive Version (`1.0.0`)
    5. Claude CLI Active Version (`2.0.0` com symlink ativo)
    6. Claude CLI ReviewOnly Version (`0.9.0` com symlink quebrado)
  - Executa `coord.Delete(ctx, targets, deleter.Options{DryRun: true})`.
  - Comprova que **100% dos arquivos canário mantêm SHA-256, tamanho e timestamp inalterados**, `trashDir` permanece sem arquivos ou manifestos, `history.jsonl` não é modificado e os cleaners não sofrem chamadas reais de exclusão.

---

## Tarefas Executadas

### Task 09-04-01 [TDD]: Fail-Closed Process Guard for 8 editors and Claude CLI with isolated blocking (AI-01, AI-03, D-05, D-06, D-08, D-11)
- **RED**: Adicionados testes `TestProcessGuard_CheckProcessStatus`, `TestEditors_ProcessGuard_IsolatedBlocking`, `TestEditors_ProcessGuard_FailClosed` e `TestClaude_ProcessGuard_ActiveCLI`.
  - **Commit RED**: `9865ec7` `test(09-04): add failing unit tests for process guard and isolated blocking`
- **GREEN**: Implementado `checkProcessStatus` em `process.go`, integrados bloqueios isolados em `scanEditors` e `scanClaude`, e registrado `ai.NewWithExecutor` no motor.
  - **Commit GREEN**: `1a5d1a1` `feat(09-04): implement fail-closed process guard for editors and claude cli`

### Task 09-04-02 [TDD]: Pre-deletion process revalidation and surgical virtual editor target cleanup (AI-01, AI-02, AI-03, D-07)
- **RED**: Adicionados testes `TestAICleaner_Clean_PreDeletionRevalidation`, `TestAICleaner_Clean_VirtualEditorTarget` e `TestAICleaner_Clean_SymlinkProtection` em `ai_test.go`.
  - **Commit RED**: `c0bc34a` `test(09-04): add failing unit tests for clean revalidation and virtual target cleanup`
- **GREEN**: Implementados `extractEditorIDFromURI`, `findEditorDef`, `cleanEditorVirtualTarget` e pre-deletion process revalidation no `Clean()` de `internal/cleaner/ai/ai.go`.
  - **Commit GREEN**: `387299d` `feat(09-04): implement pre-deletion process revalidation and surgical editor target cleanup`

### Task 09-04-03: Zero-Mutation DryRun Policy Canary & Altered-Package Verification Suite (AI-01, AI-02, AI-03, POL-01, POL-02, VAL-01, VAL-02)
- Implementado `TestCoordinator_DryRun_ZeroMutation_Phase09` em `internal/deleter/policy_test.go`.
- Refatorado `resolveAppSupportDir` em `internal/cleaner/ai/editors.go` para eliminação de redundâncias multiplataforma e aumento de cobertura para **88.2%**.
- Executada verificação integral da base de código (`go test ./...`) e compilação cruzada zero-CGO (`CGO_ENABLED=0`) para Darwin e Linux.
  - **Commit GREEN**: `08b8ec2` `feat(09-04): add zero-mutation dryrun policy canary and achieve >88% ai cleaner coverage`

---

## Mitigações de Ameaças Verificadas (STRIDE)

| Threat ID | Categoria | Componente | Mitigação Aplicada |
|---|---|---|---|
| **T-09-10** | Tampering / Race Condition | `aiCleaner.Clean` | Revalidação de processos via `checkProcessStatus` imediatamente antes da exclusão física; se o editor ou CLI foi aberto após o scan, a exclusão é abortada com segurança. |
| **T-09-11** | Tampering / Unintended Execution | `cleanEditorVirtualTarget` | URIs virtuais `editor://` interceptadas no `Clean()` e despachadas para manipuladores cirúrgicos que desfazem exclusivamente folhas autorizadas e backups `state.vscdb.backup`. |
| **T-09-12** | Denial of Service / Collateral Review | `scanEditors` | Process Guard isolado por editor: um editor aberto bloqueia apenas seus próprios alvos com `ReviewOnly: true`, liberando os outros editores inativos. |
| **T-09-13** | Tampering / DryRun Mutation | `policy_test.go` | Asserção estrita de zero mutação com hashes SHA-256 criptográficos, tamanhos e mtimes em todos os alvos da Fase 09 sob o `Coordinator`. |

---

## Verificação e Qualidade
- `go test -v ./internal/cleaner/ai -run "TestEditors_ProcessGuard|TestClaude_ProcessGuard|TestAICleaner_Clean|TestProcessGuard"`: PASS.
- `go test -v ./internal/deleter -run "TestCoordinator_DryRun_ZeroMutation_Phase09"`: PASS.
- `go test -coverprofile=coverage.out ./internal/cleaner/ai/...`: **88.2% de cobertura de statements** (meta > 85% superada).
- `go test ./...`: PASS em todos os 39 pacotes do repositório.
- Compilação cruzada estática (`CGO_ENABLED=0`) aprovada em `GOOS=darwin` e `GOOS=linux`.

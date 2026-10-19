# Phase 14: Integrated Validation & macOS Acceptance - Research

## User Constraints (from CONTEXT.md)

### Recibo de Aceitação macOS & Benchmark do Mole (VAL-03)
- **D-01: Formato Duplo (Markdown e JSON) na Pasta da Fase 14:** Persistir o recibo de aceitação em `MACOS-ACCEPTANCE.md` (relatório legível com sumário de categorias, tabelas de paridade e notas de host) e `MACOS-ACCEPTANCE.json` (payload bruto estruturado com todos os alvos e métricas) dentro de `.planning/phases/14-integrated-validation-macos-acceptance/`. — **Reversibility:** reversible — formato e localização de saída de artefatos de teste.
- **D-02: Execução Dinâmica do Mole com Fallback para Snapshot do SPEC:** O executor de aceitação tenta invocar `mole clean --dry-run` caso o utilitário esteja presente no `$PATH`. Se indisponível ou inacessível, utiliza o snapshot normativo de 9 categorias documentado em `.planning/SPEC-v2.3-MOLE-PARITY.md`, registrando nota explícita no recibo de aceitação. — **Reversibility:** costly — define a estratégia de dependência de binários externos na suíte de aceitação.
- **D-03: Módulo de Diagnóstico de Host em Go Puro:** Sondagem de ambiente no macOS realizada sem CGO e sem invocar comandos shell proprietários: sondar leitura em caminhos restritos (ex: `~/Library/Safari` ou `/Library/Application Support`) para inferir status de Full Disk Access / TCC, ler flags do filesystem APFS via `devfs.Statfs` e listar processos ativos protegidos via `Process Guard`. — **Reversibility:** costly — mecanismo de inspeção ambiental no Darwin.
- **D-04: Teste Executável Go com Build Tag de Aceitação:** Orquestrar a validação de aceitação no macOS via teste Go executável sob a build tag `//go:build acceptance` em `test/acceptance/macos_test.go`, permitindo disparo limpo e determinístico via `go test -v -tags=acceptance ./test/acceptance/...` gerando os recibos ao final. — **Reversibility:** reversible — infraestrutura de testes e orquestração.

### Suíte Integrada E2E & Invariantes do Coordinator (VAL-01, VAL-02)
- **D-05: Criação do Arquivo Dedicado cmd/workflow_v23_e2e_test.go:** Espelhar o padrão consolidado em `cmd/workflow_v22_e2e_test.go`, criando `cmd/workflow_v23_e2e_test.go` focado no escopo e nas 5 frentes do v2.3, exercitando a orquestração central através dos 5 canais (CLI, TUI Bubble Tea headless, MCP Server, Scheduler e deleter.Coordinator). — **Reversibility:** costly — estabelece a suíte de testes de regressão de integração do milestone.
- **D-06: Matriz Exaustiva de Injeção de Falhas e Invariantes:** A suíte E2E deve testar obrigatoriamente 4 cenários de estresse: (1) tentativa de remoção forçada de alvos `ReviewOnly: true` em todos os 5 canais com garantia de zero mutação; (2) bloqueio de limpeza pelo `Process Guard` quando o aplicativo está em execução; (3) rejeição e bloqueio pelo `Symlink Guard` ao tentar travessias para caminhos de blacklist do sistema (`.git`, `~/.ssh`); (4) degradação graciosa com `ProviderMetrics.Partial = true` e preenchimento de `ReviewReason` sob falhas simuladas de provedores/permissão. — **Reversibility:** costly — define os testes de resiliência e integridade de segurança.
- **D-07: Hermetismo 100% com Injeção de Raízes em t.TempDir():** A suíte E2E deve operar com construtores injetáveis e executores simulados (`cleaner.MockCommandExecutor`), montando árvores virtuais completas em `t.TempDir()` para `$HOME`, caches, logs e plists, garantindo comportamento estritamente idêntico e determinístico tanto em macOS quanto em Linux (CI), sem tocar no ambiente do usuário. — **Reversibility:** costly — arquitetura de isolamento de testes entre SOs.
- **D-08: Asserção Global de Unicidade Canônica e Desduplicação:** O teste E2E deve verificar programaticamente que nenhum caminho retornado pelo `engine.Scan()` sobrepõe ou duplica outro alvo entre todos os 19 ecossistemas registrados, comprovando que nenhum byte é contado duas vezes (`zero double-counting`). — **Reversibility:** costly — verificação da garantia de integridade do inventário.

### Limiar de Cobertura (>85%) & Matriz Zero-CGO (VAL-01, VAL-02)
- **D-09: Script Dedicado de Auditoria de Cobertura por Pacote:** Criar `scripts/verify-coverage.sh` que executa `go test -coverprofile=...`, extrai a porcentagem de cobertura para cada pacote novo ou alterado no v2.3 e falha imediatamente (exit code != 0) se qualquer um deles registrar cobertura inferior a 85%. — **Reversibility:** reversible — script de verificação e controle de qualidade.
- **D-10: Escopo Abrangente de Pacotes Auditados:** A trava rígida de cobertura individual `>= 85%` incide sobre todos os pacotes de limpadores novos/alterados (`macos`, `browsers`, `leftovers`, `node`, `shell`, `xcode`, `ai`, `docker`, `orbstack`, `shared`) e sobre os pacotes centrais tocados no v2.3 (`deleter`, `engine`, `domain`). — **Reversibility:** costly — perímetro normativo de qualidade do milestone.
- **D-11: Script de Validação da Matriz de Compilação Zero-CGO:** Criar `scripts/verify-builds.sh` que compila estaticamente (`CGO_ENABLED=0 go build`) para 4 alvos cruzados: `darwin/arm64`, `darwin/amd64`, `linux/amd64` e `linux/arm64`, falhando se qualquer target falhar na compilação ou apresentar vinculações dinâmicas. — **Reversibility:** reversible — automação de checagem de portabilidade.
- **D-12: Auditoria e Preenchimento Ativo de Lacunas em VAL-01:** Mapear e revisar os testes unitários de todos os limpadores do v2.3 para assegurar a presença explícita dos 4 critérios de VAL-01: sentinelas em `t.TempDir()`, simulação de erros de permissão (`os.ErrPermission`), rejeição de links simbólicos maliciosos (`symlink traversal`) e asserções de invariância em `DryRun`. Preencher lacunas identificadas antes da aprovação da fase. — **Reversibility:** costly — complementação e rigor da suíte unitária.

### Critérios de Tolerância e Discrepâncias de Inventário (VAL-03)
- **D-13: Seção Dedicada de Divergências de Design no Recibo:** Incluir em `MACOS-ACCEPTANCE.md` uma seção analítica detalhada ("Design Divergences & Safety Rationale") justificando explicitamente por que o DevClean classifica como `ReviewOnly: true` (0 bytes elegíveis) alvos que o Mole limpa automaticamente (como `pnpm store`, `OrbStack data`, `Stale LaunchAgents`, `App Leftovers` e `ScriptCache`), ressaltando a prioridade inegociável de segurança e preservação de estado. — **Reversibility:** reversible — documentação técnica de divergência e conformidade.
- **D-14: Falha Rígida (t.Fatalf) no Teste de Aceitação:** O teste de aceitação deve reprovar sumariamente caso ocorra qualquer uma das 4 violações de segurança e integridade: (1) mutação ou deleção física em alvo `ReviewOnly`; (2) duplicação de alvos ou contagem repetida de bytes; (3) omissão injustificada de categorias em escopo que estejam presentes no disco; (4) mutação em arquivos protegidos (sessões de navegador, bancos de dados, arquivos de sistema). — **Reversibility:** one-way — compromisso de segurança inviolável do v2.3.
- **D-15: Relatório Transparente de Permissões e TCC:** Diretórios de sistema protegidos ou com acesso negado (`/private/var/log`, `/Library/Logs/DiagnosticReports`, `/Library/LaunchAgents`) devem ser destacados na tabela de aceitação como `Partial: true` com a nota descritiva "permission denied (TCC / root required)", mantendo 0 bytes elegíveis sem distorcer o inventário mensurável nem falhar silenciosamente. — **Reversibility:** reversible — formato de representação de permissões restritas.
- **D-16: Formalização Normativa do Bloqueio de ScriptCache:** Registrar o `ScriptCache` dos navegadores no recibo de aceitação com status `ReviewOnly: true` (0 bytes elegíveis) acompanhado da anotação normativa: "Gated per BROW-04 — bloqueado para deleção até evidência de repopulação offline/MV3", documentando claramente a proteção preventiva de extensões e navegação offline. — **Reversibility:** costly — alinhamento de governança de produto para o gate de navegadores.

---

## Phase Requirements Mapping

| Requirement ID | Description | Primary Verification Mechanism | Target Artifacts |
|---|---|---|---|
| **VAL-01** | Every new cleaner has `t.TempDir()` tests with sentinels, permissions, symlink cases and DryRun assertions. | Unit test gap audit across all v2.3 cleaners; new unit tests for `macos`, `browsers`, and `leftovers`. | `internal/cleaner/macos/*_test.go`, `internal/cleaner/browsers/*_test.go`, `internal/cleaner/leftovers/*_test.go` |
| **VAL-02** | `go test ./...`, altered-package coverage above 85%, and zero-CGO Linux/Darwin builds pass. | `scripts/verify-coverage.sh` (individual per-package threshold >= 85%) and `scripts/verify-builds.sh` (4-target matrix); `cmd/workflow_v23_e2e_test.go` exercising the 5 channels. | `scripts/verify-coverage.sh`, `scripts/verify-builds.sh`, `cmd/workflow_v23_e2e_test.go` |
| **VAL-03** | macOS acceptance compares DevClean inventory with a read-only benchmark preview and records permissions, processes, TCC/FDA, APFS and omitted items. | `test/acceptance/macos_test.go` (`//go:build acceptance`) running on real Darwin host; dynamic Mole execution with SPEC fallback; pure Go host diagnostics. | `test/acceptance/macos_test.go`, `MACOS-ACCEPTANCE.md`, `MACOS-ACCEPTANCE.json` |

---

## Executive Summary

Phase 14 represents the capstone validation milestone for DevClean v2.3 (Paridade Total com Mole). It is where all five architectural fronts implemented in Phases 6 through 13 converge into proof:
1. Every individual cleaner package conforms to hermetic unit test standards (sentinels, permission error resilience, symlink traversal guards, dry-run invariance).
2. The entire test suite passes (`go test ./...`), all 11 altered/new v2.3 packages strictly meet or exceed the 85% statement coverage floor, and zero-CGO static binaries compile cleanly for Darwin (`arm64`, `amd64`) and Linux (`amd64`, `arm64`).
3. An end-to-end integration test (`cmd/workflow_v23_e2e_test.go`) exercises DevClean's 5 channels (CLI, Bubble Tea TUI headless, MCP Server, Scheduler, and Deleter Coordinator) under 4 fault-injection scenarios, proving that `ReviewOnly: true` targets are never mutated, active processes are guarded, symlink traversals to system blacklists are rejected, and zero byte double-counting occurs across all registered ecosystems.
4. A real-host macOS acceptance test (`test/acceptance/macos_test.go`), gated by `//go:build acceptance`, gathers pure Go host diagnostics (TCC/FDA, APFS filesystem flags, active protected processes), runs a read-only benchmark comparison against `tw93/mole` (using dynamic `mole clean --dry-run` or SPEC normative snapshot fallback), and emits dual acceptance receipts (`MACOS-ACCEPTANCE.md` and `MACOS-ACCEPTANCE.json`) in the phase folder.

### Codebase Inspection Findings
- **Cross-Compilation:** Static zero-CGO compilation across all 4 targets (`darwin/arm64`, `darwin/amd64`, `linux/amd64`, `linux/arm64`) was executed directly during research and completed with exit code 0 and zero warnings.
- **External Benchmark Tool:** `mole` is present at `/opt/homebrew/bin/mole` on the host. `mole clean --dry-run` was executed and completed cleanly, outputting `3.99GB potential space, 214 items, 5 categories`, and saving detailed preview items in `/Users/gabrielvp/.config/mole/clean-list.txt`.
- **Current Coverage Baseline & Gaps:**
  - `internal/cleaner/ai`: 88.2% (Pass)
  - `internal/cleaner/docker`: 88.9% (Pass)
  - `internal/cleaner/node`: 91.3% (Pass)
  - `internal/cleaner/pnpm`: 92.9% (Pass)
  - `internal/cleaner/shell`: 91.4% (Pass)
  - `internal/cleaner/xcode`: 89.9% (Pass)
  - `internal/deleter`: 89.1% (Pass)
  - `internal/domain`: 100.0% (Pass)
  - `internal/engine`: 94.9% (Pass)
  - **`internal/cleaner/browsers`:** **83.4%** (Gap: -1.6% below 85% requirement)
  - **`internal/cleaner/leftovers`:** **83.0%** (Gap: -2.0% below 85% requirement)
  - **`internal/cleaner/macos`:** **80.0%** (Gap: -5.0% below 85% requirement)

Targeted unit tests must be added to `browsers`, `leftovers`, and `macos` during Phase 14 plan 1 to close these gaps before the coverage enforcement script is activated.

---

## Architectural Responsibility Map

```
┌────────────────────────────────────────────────────────────────────────────────────────┐
│                                PHASE 14 ARCHITECTURE                                  │
├────────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                        │
│   PLAN 1: UNIT TEST GAP REMEDIATION & COVERAGE / ZERO-CGO ENFORCEMENT                  │
│   ├── internal/cleaner/macos/gap_test.go        (Pushes macos: 80.0% -> >85%)           │
│   ├── internal/cleaner/browsers/gap_test.go     (Pushes browsers: 83.4% -> >85%)        │
│   ├── internal/cleaner/leftovers/gap_test.go    (Pushes leftovers: 83.0% -> >85%)       │
│   ├── scripts/verify-coverage.sh               (Enforces >= 85% on 11 v2.3 packages)   │
│   └── scripts/verify-builds.sh                 (Enforces CGO_ENABLED=0 across 4 OS/arch)│
│                                                                                        │
│   PLAN 2: MULTI-CHANNEL E2E REGRESSION SUITE (cmd/workflow_v23_e2e_test.go)            │
│   ├── Channel 1: CLI (newCleanCmd, newScanCmd with --auto, --dry-run, --preset, etc.)  │
│   ├── Channel 2: TUI Headless (Bubble Tea tea.KeyMsg updates, navigation, deletion)   │
│   ├── Channel 3: MCP Server (mcp.New, handleScan, handlePropose, handleExecute)        │
│   ├── Channel 4: Scheduler (newScheduleRunCmd, auto background runner)                │
│   └── Channel 5: Coordinator (deleter.New, Stage 1 ReviewOnly skip, hook protection)  │
│   └── Invariants: Process Guard, Symlink Guard, ReviewOnly immutability, Zero Double-Ct│
│                                                                                        │
│   PLAN 3: REAL-HOST MACOS ACCEPTANCE TEST (test/acceptance/macos_test.go)              │
│   ├── Pure Go Darwin Host Diagnostics (TCC/FDA, APFS flags, Sysctl, Protected PIDs)   │
│   ├── Dynamic Mole Runner (mole clean --dry-run) with Fallback to SPEC Snapshot        │
│   ├── DevClean Real Scan Inventory Collection (Read-only, zero mutation)              │
│   ├── 4 Hard Invariant Checks (t.Fatalf per D-14)                                      │
│   └── Output Generator: MACOS-ACCEPTANCE.md & MACOS-ACCEPTANCE.json                    │
└────────────────────────────────────────────────────────────────────────────────────────┘
```

| Component | Responsibility | Inputs | Outputs | Error Modes / Guarantees |
|---|---|---|---|---|
| `scripts/verify-coverage.sh` | Parse coverage profiles per package and assert >= 85.0% on all v2.3 target packages. | Package list, `go test -coverprofile` outputs | Exit code 0 or 1, package breakdown | Fails fast if any single package in scope is < 85.0%. |
| `scripts/verify-builds.sh` | Compile DevClean statically across 4 cross targets. | Go build toolchain, `main.go` | Exit code 0 or 1, compile logs | Fails if compilation errors or if dynamic linkages detected. |
| `cmd/workflow_v23_e2e_test.go` | Multi-channel integration tests exercising CLI, TUI, MCP, Scheduler, and Coordinator under 4 fault-injection scenarios. | Virtual `$HOME` fixtures in `t.TempDir()`, `cleaner.MockCommandExecutor` | Test pass/fail, zero file mutation | Zero double-counting assertion across all registered cleaners. |
| `test/acceptance/macos_test.go` | Real macOS host acceptance runner gated by `//go:build acceptance`. | Real macOS host filesystem, Mole CLI or SPEC snapshot | `MACOS-ACCEPTANCE.md`, `MACOS-ACCEPTANCE.json` | Fails on ReviewOnly deletion, duplicate bytes, omitted in-scope items, or protected state mutation (D-14). |

---

## Standard Stack & Tooling

### Go Testing Tooling
- **`testing.T` and `t.TempDir()`**: Standard library testing framework. Guarantees 100% hermetic virtual filesystems for E2E and unit tests with automatic cleanup.
- **`cleaner.MockCommandExecutor` (`internal/cleaner/executor_mock.go`)**: Simulates external command invocations (`docker system df`, `pgrep -x <app>`, `LookPath`, `git`, etc.) with deterministic exit codes, standard output strings, or errors.
- **Bubble Tea (`github.com/charmbracelet/bubbletea`)**: Headless model testing via `m.Update(tea.KeyMsg{...})`, matching the pattern established in `internal/tui/model_test.go` and `cmd/workflow_v22_e2e_test.go`.
- **MCP Server (`github.com/mark3labs/mcp-go`)**: In-memory server created via `internal/mcp.New(eng, cfg, mcp.WithCoordinator(coord))` for calling `handleScan`, `handlePropose`, and `handleExecute` directly.

### Host Diagnostics (Pure Go, Zero-CGO)
- **`golang.org/x/sys/unix`**: Used in `internal/fs/statfs_darwin.go` and host diagnostics.
  - `unix.Statfs(path, &stat)`: Reads filesystem type name (`stat.Fstypename` -> `"apfs"` or `"hfs"`) and mount flags (`stat.Flags`).
  - `unix.Sysctl("kern.osproductversion")`: Pure Go query for macOS version (e.g. `"15.0"` or `"14.5"`) without running `sw_vers`.
- **TCC / Full Disk Access Probing**: Pure Go `os.Open` / `os.ReadDir` probing on restricted paths (e.g., `~/Library/Safari` or `/Library/Application Support/com.apple.TCC`). If `errors.Is(err, syscall.EPERM)` or `os.IsPermission(err)`, FDA is reported as false; if readable, FDA is true.

---

## Architecture Patterns

### Pattern 1: Multi-Channel Invariant Enforcement
DevClean has five entry points that can trigger inventory inspection or deletion:
1. CLI commands (`newCleanCmd`, `newScanCmd`)
2. TUI interface (Bubble Tea models in `internal/tui`)
3. MCP Server tools (`scan`, `propose_cleanup`, `execute_cleanup`)
4. Scheduler automated background runs (`newScheduleRunCmd`)
5. Deleter Coordinator (`internal/deleter/deleter.go`)

To ensure that safety invariants are universal rather than UI-dependent, the test harness in `cmd/workflow_v23_e2e_test.go` exercises the exact same fixtures against all 5 channels:
- Channel CLI: invoke `cleanCmd.Execute()` with `--auto` and `--yes` -> assert `ReviewOnly` items are ignored and output contains `⊙`.
- Channel TUI: send `tea.KeyMsg` to select all targets and press enter -> assert `ReviewOnly` items are locked and confirmation excludes them.
- Channel MCP: call `handlePropose` and `handleExecute` -> assert status is `"review_only"`, `eligible_bytes` is 0, and `execute_cleanup` skips them.
- Channel Scheduler: run `newScheduleRunCmd()` -> assert background automated run records audit log with 0 bytes deleted from review targets.
- Channel Coordinator: call `coord.Delete(ctx, targets, opts)` -> assert `res.SkippedReviewOnly` contains all review items and disk is untouched.

### Pattern 2: Fault Injection & Resilience Testing (D-06)
Four mandatory stress scenarios must be implemented in `cmd/workflow_v23_e2e_test.go`:
1. **Forced Deletion of ReviewOnly Targets**: Pass targets with `ReviewOnly: true` (e.g. `pnpm store`, `OrbStack data`, `Stale LaunchAgents`, `NuGet packages`, `ScriptCache`) with `--yes` or `ActionDeletingDirect`. Assert sentinels remain 100% intact.
2. **Process Guard Blocking**: Configure `cleaner.MockCommandExecutor` to return active PIDs for running applications (Chrome, OrbStack, Xcode). Assert scanner flags targets with `ReviewOnly: true` and `ReviewReason: active_process`, and cleaner aborts or skips deletion.
3. **Symlink Guard Rejection**: Construct symlinks in test tree targeting `.git`, `~/.ssh`, `/private/etc`, or traversing outside root. Assert `safety.IsCoreBlacklisted` and `isSafeSymlink` reject them with zero mutation.
4. **Provider / Permission Failure Degradation**: Configure `MockCommandExecutor` to return connection failure for Docker daemon and inject unreadable directories. Assert engine records `ProviderMetrics.Partial = true`, assigns `ReviewReason: ambiguous_state` or `system_protected`, and continues scanning other ecosystems without crashing.

### Pattern 3: Hermetic In-Memory Tree Simulation (D-07)
All integration tests must run in `t.TempDir()` using root injection:
```go
tmpDir := t.TempDir()
fakeHome := filepath.Join(tmpDir, "home")
fakeWorkspace := filepath.Join(tmpDir, "workspace")
t.Setenv("HOME", fakeHome)
t.Setenv("XDG_CONFIG_HOME", filepath.Join(fakeHome, ".config"))
t.Setenv("XDG_DATA_HOME", filepath.Join(fakeHome, ".local", "share"))
t.Setenv("TMPDIR", filepath.Join(tmpDir, "tmp"))
```
Canary sentinel files with unique payload strings (e.g. `SENTINEL_INVIOLABLE_ORBSTACK_DATA_V23`) are planted in every simulated directory. Assertions verify that sentinels in review-only or protected directories are never modified or removed.

### Pattern 4: Pure Go Host Probing (D-03)
Acceptance testing on the real macOS host requires diagnosing system state without invoking shell commands that could be fragile, slow, or blocked:
```go
// TCC / Full Disk Access Probe
func probeFullDiskAccess(homeDir string) (bool, string) {
    safariPath := filepath.Join(homeDir, "Library", "Safari")
    f, err := os.Open(safariPath)
    if err != nil {
        if errors.Is(err, syscall.EPERM) || os.IsPermission(err) {
            return false, "TCC restricted (operation not permitted)"
        }
        return false, fmt.Sprintf("inaccessible: %v", err)
    }
    defer f.Close()
    _, err = f.Readdirnames(1)
    if err != nil && (errors.Is(err, syscall.EPERM) || os.IsPermission(err)) {
        return false, "TCC restricted on readdir"
    }
    return true, "Full Disk Access granted"
}
```

### Pattern 5: Dual Acceptance Artifact Generation (D-01)
The acceptance test writes two complementary files to `.planning/phases/14-integrated-validation-macos-acceptance/`:
1. `MACOS-ACCEPTANCE.md`: Markdown document featuring:
   - Host hardware & OS diagnostics (CPU, macOS version, APFS flags, TCC/FDA status).
   - Side-by-side Parity Comparison Table:
     `Category | Mole Discovered | DevClean Occupied | DevClean Eligible | Status & Safety Rationale`
   - Dedicated Section: "Design Divergences & Safety Rationale" (D-13).
   - Transparent reporting of permission-restricted paths (D-15).
   - Browser ScriptCache MV3 gating rationale (D-16).
2. `MACOS-ACCEPTANCE.json`: JSON payload containing machine-readable metrics, host facts, target array with `id`, `category`, `occupied_bytes`, `eligible_bytes`, `review_only`, `review_reason`, and `partial` status.

---

## Don't Hand-Roll

| Problem | Anti-Pattern (Don't Hand-Roll) | Correct Pattern (Standard Stack) |
|---|---|---|
| macOS Version & Kernel Info | Calling `exec.Command("sw_vers")` or parsing `uname -a` via shell | Use `unix.Sysctl("kern.osproductversion")` and `runtime.GOARCH` / `runtime.GOOS`. |
| APFS Filesystem Detection | Parsing `df -T` or `diskutil info /` via shell pipe | Use `unix.Statfs(path, &stat)` and read `stat.Fstypename` directly in Go. |
| Process State Checking | Executing shell pipelines `ps aux | grep ...` | Use `cleaner.MockCommandExecutor` in unit/E2E tests, and `internal/cleaner/browsers/process.go` (`pgrep`) on real host. |
| Blacklist Path Matching | Custom regex or string contains checks for `.git` or `.ssh` | Use `safety.IsCoreBlacklisted(path)` which handles path components, system roots, and symlink evaluation canonicalization. |
| TUI Testing | Launching terminal emulator or pty subprocess | Use Bubble Tea headless test harness (`m.Update(tea.KeyMsg{...})`), inspecting model state directly. |
| MCP Protocol Testing | Launching stdin/stdout subprocess with JSON-RPC piping | Instantiate `mcp.New(eng, cfg, ...)` and call internal handlers (`handleScan`, `handlePropose`, `handleExecute`) directly with `CallToolRequest`. |

---

## Common Pitfalls & Edge Cases

### Pitfall 1: Fragile Mole CLI Dependency on Non-Darwin or Clean Environments (D-02)
- **Problem:** If `mole` is not installed on the system (e.g. in CI or on a fresh developer machine), the acceptance test would fail or block.
- **Remedy:** Implement dynamic detection (`exec.LookPath("mole")`). If available, execute `mole clean --dry-run` and parse stdout / `~/.config/mole/clean-list.txt`. If absent, seamlessly fallback to the normative 9-category snapshot from `SPEC-v2.3-MOLE-PARITY.md` and explicitly note "Baseline source: SPEC-v2.3-MOLE-PARITY.md normative snapshot" in the receipt.

### Pitfall 2: Double-Counting Across Parent and Nested Cleaners (D-08)
- **Problem:** `internal/cleaner/macos` scans `~/Library/Caches/*` (User App Caches). If `pnpm`, `pip`, `mise`, `go-build`, or `ms-playwright` caches are located inside `~/Library/Caches/`, naive scanning would count their bytes twice.
- **Remedy:** `internal/cleaner/macos/user.go` already maintains an exclusion set (`cleanerExclusions` / `isExcludedByDedicatedCleaner`) for known cleaner subdirectories. In `cmd/workflow_v23_e2e_test.go`, implement a programmatic assertion that builds a map of canonical paths across all returned targets in `engine.Scan()` and fails if any path is a prefix or duplicate of another.

### Pitfall 3: TCC / FDA Permission Denials Failing Tests (D-15)
- **Problem:** On macOS, `/Library/Logs/DiagnosticReports`, `/private/var/log`, and system `LaunchAgents` require root or FDA. If DevClean fails with an uncaught error, tests crash.
- **Remedy:** DevClean implements graceful degradation (`scanDirectoryWithTCCFallback`). The test must assert that when permission is denied, the target is returned with `ProviderMetrics.Partial = true`, `ReviewOnly = true`, `EligibleBytes = 0`, and `ReviewReason = "system_protected"`.

### Pitfall 4: Flaky Test on Linux CI Due to Darwin-Specific Path Assumptions (D-07)
- **Problem:** Tests expecting `/Users/...` or Darwin-specific paths fail on Linux CI runners.
- **Remedy:** All E2E and unit tests must operate entirely within `t.TempDir()`, using `filepath.Join(tempDir, ...)` for mock home and library paths, ensuring 100% hermetism and portability between macOS and Linux. Acceptance tests for real Darwin host use `//go:build acceptance` and skip when `runtime.GOOS != "darwin"`.

### Pitfall 5: Accidental Mutation During Acceptance Runs (D-14)
- **Problem:** Acceptance tests running on a real host could accidentally delete files if `Clean()` was called without safeguards.
- **Remedy:** The acceptance test MUST run strictly in read-only mode (`engine.Scan()` only). No deletion methods are called. A pre/post file integrity check on critical directories asserts zero filesystem modification.

---

## Validation Architecture

The Nyquist Validation Architecture for Phase 14 consists of a 5-layer verification framework:

```
┌────────────────────────────────────────────────────────────────────────┐
│                   NYQUIST VALIDATION ARCHITECTURE                      │
├────────────────────────────────────────────────────────────────────────┤
│ 1. Unit Test Coverage & Gap Remediation (VAL-01, D-12)                │
│    - internal/cleaner/macos (>= 85%)                                   │
│    - internal/cleaner/browsers (>= 85%)                                │
│    - internal/cleaner/leftovers (>= 85%)                               │
│    - Sentinels in t.TempDir(), permission errors, symlink guard, DryRun│
│                                                                        │
│ 2. Automated Coverage Verification Gate (VAL-02, D-09, D-10)           │
│    - scripts/verify-coverage.sh                                        │
│    - Enforces >= 85.0% statement coverage on all 11 v2.3 packages      │
│                                                                        │
│ 3. Static Zero-CGO Cross-Compilation Matrix (VAL-02, D-11)             │
│    - scripts/verify-builds.sh                                          │
│    - Builds CGO_ENABLED=0 on darwin/arm64, darwin/amd64,               │
│      linux/amd64, linux/arm64                                          │
│                                                                        │
│ 4. Multi-Channel E2E Integration Suite (VAL-01, VAL-02, D-05..D-08)    │
│    - cmd/workflow_v23_e2e_test.go                                      │
│    - 5 channels: CLI, TUI headless, MCP, Scheduler, Coordinator        │
│    - 4 fault injection tests (ReviewOnly, Process, Symlink, Partial)   │
│    - Zero double-counting assertion across all 19 ecosystems           │
│                                                                        │
│ 5. Real macOS Host Acceptance Suite (VAL-03, D-01..D-04, D-13..D-16)   │
│    - test/acceptance/macos_test.go (//go:build acceptance)             │
│    - Pure Go Darwin diagnostics (TCC, FDA, APFS, protected processes)  │
│    - Mole dynamic dry-run with SPEC fallback                           │
│    - Dual output: MACOS-ACCEPTANCE.md and MACOS-ACCEPTANCE.json        │
└────────────────────────────────────────────────────────────────────────┘
```

### 1. Unit Test Coverage Deficit & Remediation Plan (VAL-01, D-12)

| Package | Current Coverage | Target | Gap | Remediation Plan in Plan 1 |
|---|---|---|---|---|
| `internal/cleaner/macos` | **80.0%** | **>= 85.0%** | **-5.0%** | Add unit tests for `Clean()` across categories, `resolveTargetBundleID`, `cleanTrash`, `scanDirectoryWithTCCFallback` error branches, and `whitelist.isSafeSymlink`. |
| `internal/cleaner/browsers` | **83.4%** | **>= 85.0%** | **-1.6%** | Add unit tests for constructor functions (`NewWithExecutor`, `NewWithExecutorHomeAndOS`), `safari.isSyscallPermission`, `safari.scanSafari` permission errors, and profile cleaning error branches. |
| `internal/cleaner/leftovers` | **83.0%** | **>= 85.0%** | **-2.0%** | Add unit tests for constructors (`New`, `NewWithExecutor`, `NewWithConfig`), `leftovers.isSafeSymlink` traversal cases, and unreadable LaunchAgent plist error handling. |
| `internal/cleaner/ai` | 88.2% | >= 85.0% | Pass | Verify sentinels and symlink guard assertions are complete. |
| `internal/cleaner/node` | 91.3% | >= 85.0% | Pass | Already compliant. |
| `internal/cleaner/shell` | 91.4% | >= 85.0% | Pass | Already compliant. |
| `internal/cleaner/xcode` | 89.9% | >= 85.0% | Pass | Already compliant. |
| `internal/cleaner/docker` | 88.9% | >= 85.0% | Pass | Already compliant (includes OrbStack). |
| `internal/deleter` | 89.1% | >= 85.0% | Pass | Already compliant. |
| `internal/domain` | 100.0% | >= 85.0% | Pass | Already compliant. |
| `internal/engine` | 94.9% | >= 85.0% | Pass | Already compliant. |

### 2. Automated Coverage Script (`scripts/verify-coverage.sh`) (D-09, D-10)
- Executes: `go test -coverprofile=coverage.out ./...`
- Extracts per-package statement coverage using `go tool cover -func=coverage.out` or `go test -cover`.
- Evaluates the 11 target packages defined in D-10:
  1. `github.com/gtkpad/devclean/internal/cleaner/macos`
  2. `github.com/gtkpad/devclean/internal/cleaner/browsers`
  3. `github.com/gtkpad/devclean/internal/cleaner/leftovers`
  4. `github.com/gtkpad/devclean/internal/cleaner/node`
  5. `github.com/gtkpad/devclean/internal/cleaner/shell`
  6. `github.com/gtkpad/devclean/internal/cleaner/xcode`
  7. `github.com/gtkpad/devclean/internal/cleaner/ai`
  8. `github.com/gtkpad/devclean/internal/cleaner/docker`
  9. `github.com/gtkpad/devclean/internal/deleter`
  10. `github.com/gtkpad/devclean/internal/engine`
  11. `github.com/gtkpad/devclean/internal/domain`
- If ANY package has coverage `< 85.0%`, prints the offending package and statement percentage, and exits with code 1.
- If ALL packages have coverage `>= 85.0%`, prints summary table and exits with code 0.

### 3. Static Zero-CGO Build Matrix Script (`scripts/verify-builds.sh`) (D-11)
- Validates clean static compilation with `CGO_ENABLED=0` across 4 platform targets:
  1. `GOOS=darwin GOARCH=arm64`
  2. `GOOS=darwin GOARCH=amd64`
  3. `GOOS=linux GOARCH=amd64`
  4. `GOOS=linux GOARCH=arm64`
- Validates the resulting binaries do not contain dynamic bindings (`otool -L` on Darwin or `file` on Linux/Darwin).
- Returns exit code 0 if all 4 targets succeed; exit code 1 if any fails.

### 4. Multi-Channel E2E Test Suite (`cmd/workflow_v23_e2e_test.go`) (D-05 to D-08)
- File: `cmd/workflow_v23_e2e_test.go` (package `cmd`)
- Channel Coverage:
  1. CLI Channel: Tests `newCleanCmd` and `newScanCmd` with `--auto`, `--dry-run`, `--preset safe`, `--trash`.
  2. TUI Channel: Tests Bubble Tea headless model updating state on delete commands.
  3. MCP Channel: Tests `internal/mcp.New` calling `handleScan`, `handlePropose`, and `handleExecute`.
  4. Scheduler Channel: Tests `newScheduleRunCmd` automated clean execution.
  5. Coordinator Channel: Direct invocation of `deleter.New(...).Delete(...)`.
- Invariant & Fault Injection Scenarios:
  - Scenario A: Forced mutation on `ReviewOnly` targets (`pnpm store`, `OrbStack data`, `Stale LaunchAgents`, `NuGet packages`, `ScriptCache`) -> 0 bytes mutated, sentinels intact.
  - Scenario B: `Process Guard` blocks cleanup of active applications (Chrome, OrbStack, Xcode).
  - Scenario C: `Symlink Guard` blocks traversals to blacklisted targets (`.git`, `~/.ssh`).
  - Scenario D: Graceful degradation with `ProviderMetrics.Partial = true` when provider/permission fails.
  - Scenario E: Canonical path uniqueness assertion (`zero double-counting`) across all 19 registered ecosystems.

### 5. Real-Host macOS Acceptance Suite (`test/acceptance/macos_test.go`) (VAL-03, D-01 to D-04, D-13 to D-16)
- Build Tag: `//go:build acceptance`
- Run Command: `go test -v -tags=acceptance -timeout=10m ./test/acceptance/...`
- Platform Guard: `if runtime.GOOS != "darwin" { t.Skip("macOS acceptance test requires darwin host") }`
- Pure Go Host Diagnostics:
  - System version via `unix.Sysctl("kern.osproductversion")`
  - Architecture via `runtime.GOARCH`
  - TCC / Full Disk Access probe on `~/Library/Safari`
  - Filesystem type & mount flags via `unix.Statfs`
  - Active protected process detection via `Process Guard`
- Benchmark Comparison:
  - Execute `mole clean --dry-run` if `mole` found in `$PATH`; fallback to normative snapshot in `SPEC-v2.3-MOLE-PARITY.md`.
  - Execute `devclean scan` (read-only) across all registered cleaners.
  - Compute side-by-side comparison for all 9 categories.
- Hard Failure Conditions (`t.Fatalf` per D-14):
  1. Any physical mutation on `ReviewOnly` target.
  2. Any target duplication or repeated byte count.
  3. Any unjustified omission of in-scope categories present on disk.
  4. Any mutation in protected files (browser sessions, databases, system files).
- Output Generation:
  - Emits `.planning/phases/14-integrated-validation-macos-acceptance/MACOS-ACCEPTANCE.md`
  - Emits `.planning/phases/14-integrated-validation-macos-acceptance/MACOS-ACCEPTANCE.json`

---

## Code Examples & Harness Patterns

### 1. `scripts/verify-coverage.sh` Pattern
```bash
#!/usr/bin/env bash
set -euo pipefail

MIN_COVERAGE=85.0
FAILURES=0

TARGET_PACKAGES=(
  "github.com/gtkpad/devclean/internal/cleaner/macos"
  "github.com/gtkpad/devclean/internal/cleaner/browsers"
  "github.com/gtkpad/devclean/internal/cleaner/leftovers"
  "github.com/gtkpad/devclean/internal/cleaner/node"
  "github.com/gtkpad/devclean/internal/cleaner/shell"
  "github.com/gtkpad/devclean/internal/cleaner/xcode"
  "github.com/gtkpad/devclean/internal/cleaner/ai"
  "github.com/gtkpad/devclean/internal/cleaner/docker"
  "github.com/gtkpad/devclean/internal/deleter"
  "github.com/gtkpad/devclean/internal/engine"
  "github.com/gtkpad/devclean/internal/domain"
)

echo "=== Verifying Coverage Thresholds (>= ${MIN_COVERAGE}%) ==="

for pkg in "${TARGET_PACKAGES[@]}"; do
  output=$(go test -cover "$pkg" 2>&1)
  cov_str=$(echo "$output" | grep -o 'coverage: [0-9.]*%' | awk '{print $2}' | tr -d '%')
  if [ -z "$cov_str" ]; then
    echo "❌ [FAIL] $pkg: Could not determine coverage"
    FAILURES=$((FAILURES + 1))
    continue
  fi

  is_passing=$(awk "BEGIN {print ($cov_str >= $MIN_COVERAGE) ? 1 : 0}")
  if [ "$is_passing" -eq 1 ]; then
    echo "✅ [PASS] $pkg: ${cov_str}%"
  else
    echo "❌ [FAIL] $pkg: ${cov_str}% (below ${MIN_COVERAGE}%)"
    FAILURES=$((FAILURES + 1))
  fi
done

if [ "$FAILURES" -gt 0 ]; then
  echo ""
  echo "Error: $FAILURES package(s) failed coverage threshold of ${MIN_COVERAGE}%."
  exit 1
fi

echo "All ${#TARGET_PACKAGES[@]} v2.3 packages meet or exceed ${MIN_COVERAGE}% coverage."
exit 0
```

### 2. `scripts/verify-builds.sh` Pattern
```bash
#!/usr/bin/env bash
set -euo pipefail

TARGETS=(
  "darwin/arm64"
  "darwin/amd64"
  "linux/amd64"
  "linux/arm64"
)

echo "=== Verifying Zero-CGO Static Cross-Compilation ==="

for target in "${TARGETS[@]}"; do
  os="${target%/*}"
  arch="${target#*/}"
  echo "Compiling for $os/$arch (CGO_ENABLED=0)..."
  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -o /dev/null .
  echo "✅ $os/$arch compiled successfully."
done

echo "All 4 static targets compiled cleanly without CGO."
exit 0
```

### 3. Multi-Channel E2E Channel Test Pattern (`cmd/workflow_v23_e2e_test.go`)
```go
func TestE2E_V23_ReviewOnly_ImmunityAcrossChannels(t *testing.T) {
    tmpDir := t.TempDir()
    fakeHome := filepath.Join(tmpDir, "home")
    t.Setenv("HOME", fakeHome)
    t.Setenv("XDG_CONFIG_HOME", filepath.Join(fakeHome, ".config"))
    t.Setenv("XDG_DATA_HOME", filepath.Join(fakeHome, ".local", "share"))

    // 1. Plant canary sentinels in ReviewOnly paths
    pnpmStore := filepath.Join(fakeHome, "Library", "pnpm", "store")
    orbData := filepath.Join(fakeHome, "Library", "Group Containers", "group.dev.orbstack", "data")
    staleAgent := filepath.Join(fakeHome, "Library", "LaunchAgents", "com.example.stale.plist")
    
    // Plant files...
    sentinel := filepath.Join(pnpmStore, "v3", "canary.bin")
    _ = os.MkdirAll(filepath.Dir(sentinel), 0755)
    _ = os.WriteFile(sentinel, []byte("INVIOLABLE_PNPM_STORE"), 0644)

    // 2. Channel A: CLI with --auto --yes
    cleanCmd := newCleanCmd()
    cleanCmd.SetArgs([]string{"--auto", "--yes"})
    var buf bytes.Buffer
    cleanCmd.SetOut(&buf)
    _ = cleanCmd.Execute()

    // Assert sentinel intact
    if _, err := os.Stat(sentinel); err != nil {
        t.Fatalf("CLI --auto deleted ReviewOnly target!")
    }

    // 3. Channel B: MCP handleExecute
    // Call MCP tool with target IDs -> assert skipped
    
    // 4. Channel C: deleter.Coordinator direct
    // Call coordinator.Delete with ReviewOnly target -> assert skipped
}
```

### 4. macOS Host Diagnostics & Acceptance Runner Pattern (`test/acceptance/macos_test.go`)
```go
//go:build acceptance

package acceptance_test

import (
    "context"
    "encoding/json"
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "runtime"
    "testing"

    "github.com/gtkpad/devclean/internal/config"
    "github.com/gtkpad/devclean/internal/engine"
    "github.com/gtkpad/devclean/internal/platform"
    "golang.org/x/sys/unix"
)

type HostDiagnostics struct {
    OSVersion          string `json:"os_version"`
    Architecture       string `json:"architecture"`
    APFSSupported      bool   `json:"apfs_supported"`
    FullDiskAccess     bool   `json:"full_disk_access"`
    FDANote            string `json:"fda_note"`
    ActiveProtectedApps []string `json:"active_protected_apps"`
}

func TestMacOS_ParityAcceptance(t *testing.T) {
    if runtime.GOOS != "darwin" {
        t.Skip("acceptance test requires real macOS host")
    }

    ctx := context.Background()
    diag := gatherHostDiagnostics(t)

    // 1. Run or fallback Mole
    moleResult := runMoleOrFallback(t)

    // 2. Run DevClean real scan
    cfg := config.Defaults()
    resolver := platform.New()
    eng := engine.New(&cfg, resolver)
    engine.RegisterDefaultCleaners(eng, &cfg, resolver)

    results := engine.Collect(ctx, eng.Scan(ctx))

    // 3. Perform hard assertions (D-14)
    // - Check zero double-counting
    // - Check ReviewOnly invariant
    // - Check no in-scope omission

    // 4. Generate MACOS-ACCEPTANCE.md and MACOS-ACCEPTANCE.json
    generateAcceptanceReceipts(t, diag, moleResult, results)
}
```

---

## Plan Breakdown Recommendation for Phase 14

Based on this deep research, Phase 14 should be organized into 3 sequential plans:

1. **Plan 14-01: Unit Test Gap Remediation & Verification Scripts (VAL-01, VAL-02)**
   - Fill unit test coverage gaps in `internal/cleaner/macos` (bringing 80.0% -> >85%), `internal/cleaner/browsers` (bringing 83.4% -> >85%), and `internal/cleaner/leftovers` (bringing 83.0% -> >85%).
   - Audit and confirm VAL-01 sentinels, permission errors (`os.ErrPermission`), symlink guard traversal, and DryRun assertions.
   - Implement `scripts/verify-coverage.sh` (enforcing >= 85.0% on all 11 v2.3 packages).
   - Implement `scripts/verify-builds.sh` (validating static zero-CGO compilation for darwin/arm64, darwin/amd64, linux/amd64, linux/arm64).

2. **Plan 14-02: Multi-Channel E2E Integration Regression Suite (VAL-01, VAL-02)**
   - Implement `cmd/workflow_v23_e2e_test.go` mirroring `workflow_v22_e2e_test.go`.
   - Exercise the 5 channels: CLI, TUI Bubble Tea headless, MCP Server, Scheduler, and Coordinator.
   - Implement the 4 stress & fault injection scenarios:
     1. Forced removal of `ReviewOnly: true` targets across all 5 channels with 0 mutation.
     2. `Process Guard` blocking when target application is running.
     3. `Symlink Guard` rejecting system blacklist traversals (`.git`, `~/.ssh`).
     4. Graceful degradation with `ProviderMetrics.Partial = true` under provider/permission failures.
   - Implement global canonical ownership uniqueness and zero double-counting assertion across all registered ecosystems.

3. **Plan 14-03: Real-Host macOS Acceptance Test & Parity Receipts (VAL-03)**
   - Implement `test/acceptance/macos_test.go` (`//go:build acceptance`).
   - Implement pure Go Darwin diagnostics (TCC, FDA, APFS flags, protected processes).
   - Implement dynamic Mole preview execution (`mole clean --dry-run`) with automatic fallback to normative `SPEC-v2.3-MOLE-PARITY.md` snapshot.
   - Enforce the 4 hard failure invariants (`t.Fatalf` per D-14).
   - Emit `MACOS-ACCEPTANCE.md` (with Design Divergences, Permission Notes, ScriptCache gate) and `MACOS-ACCEPTANCE.json` in `.planning/phases/14-integrated-validation-macos-acceptance/`.

---

## RESEARCH COMPLETE

# Phase 8: Node, Headless, Shell/CLI & Xcode - Research

**Date:** 2026-10-15  
**Phase:** 08-Node, Headless, Shell/CLI & Xcode  
**Status:** Complete  
**Objective:** Provide all architectural, code pattern, path, process detection, and validation requirements necessary to construct an executable plan for Phase 08.

---

## User Constraints

*(Verbatim reproduction of locked decisions D-01 through D-14 and domain scope from `08-CONTEXT.md`)*

### Phase Boundary
Garantir a descoberta precisa por presença em disco de artefatos do ecossistema Node e navegadores de automação (DEV-01), ferramentas de shell e CLI (DEV-02) e Xcode/Simulators (DEV-03), aplicando a política ReviewOnly centralizada para bloquear exclusão insegura quando ferramentas estiverem em execução ou em estado incerto:
1. `node` cleaner: descobre por presença em disco `~/.npm/_cacache`, `~/.npm/_npx`, `~/.npm/_logs`, Corepack (`~/.cache/node/corepack`, `~/Library/Caches/node/corepack`) e navegadores de automação (Puppeteer `~/.cache/puppeteer`, Playwright `~/Library/Caches/ms-playwright` / `~/.cache/ms-playwright`, Cypress `~/Library/Caches/Cypress` / `~/.cache/Cypress`), mesmo sem os executáveis no PATH (`DEV-01`).
2. `pnpm` cleaner: descobre por presença em disco o cache de metadados (`~/Library/Caches/pnpm` e `~/.cache/pnpm`), mantendo a integridade do ecossistema `pnpm` (`DEV-01`).
3. `shell` cleaner (`internal/cleaner/shell`): novo cleaner sob o ecossistema `shell`, cobrindo `mise` (`mise cache` e `mise downloads`), GitHub CLI (`gh`), `pre-commit`, `Oh My Zsh` (`~/.oh-my-zsh/cache`) e `~/.zcompdump*` sem ler nem alterar configurações (`.zshrc`, `.bashrc`) ou histórico (`.zsh_history`, `.bash_history`) (`DEV-02`).
4. `xcode` cleaner: expande a cobertura para `CoreSimulator/Caches`, `CoreSimulator/Temp`, `iOS DeviceSupport`, `watchOS DeviceSupport` e `Archives`. Aplica detecção fail-closed de processos ativos (`Xcode`, `Simulator`): se ativos ou em estado incerto, os alvos recebem `ReviewOnly = true`, `ReviewReason = ReviewReasonActiveProcess` e `EligibleBytes = 0` (`DEV-03`).
5. Testes unitários herméticos com `t.TempDir()`, sentinels, casos de symlink e permissões, asserções de DryRun e cobertura acima de 85% nos pacotes alterados.

### Node & Headless Automation Caches (DEV-01)
- **D-01:** **Alvos Granulares por Ferramenta no Cleaner Node:** Incorporar alvos individuais no cleaner `internal/cleaner/node` para cada ferramenta de automação (`node-puppeteer`, `node-playwright`, `node-cypress`) e package management (`npm-cache`, `npx-cache`, `npm-logs`, `corepack-cache`). — *Reversibility:* reversible — organiza a emissão de alvos em `node.go`.
- **D-02:** **Classificação de Risco Diferenciada:** Atribuir `RiskMedium` para downloads de binários pesados de navegadores (Puppeteer e Playwright) evitando remoções acidentais em ambientes com conectividade limitada, e `RiskLow` para caches regeneráveis gerais de pacotes e logs (npm, npx, logs, Corepack e Cypress). — *Reversibility:* reversible.
- **D-03:** **Separação Granular de Subdiretórios npm:** Apresentar `~/.npm/_cacache` como `npm cache`, `~/.npm/_npx` como `npx cache` e `~/.npm/_logs` como `npm logs`, oferecendo visibilidade analítica clara de onde o espaço em disco está concentrado. — *Reversibility:* reversible.
- **D-04:** **Pnpm Cache no Cleaner Pnpm:** O cache de metadados HTTP do pnpm (`~/Library/Caches/pnpm` no macOS e `~/.cache/pnpm` no Linux) reside no cleaner `internal/cleaner/pnpm` sob o ecossistema `pnpm`, permitindo que `IsInstalled()` detecte presença em disco sem depender do binário `pnpm` no PATH. — *Reversibility:* costly — define a divisão de responsabilidades entre os pacotes `node` e `pnpm`.
- **D-05:** **Deleção Direta via Filesystem com Validação:** Exclusão via `os.RemoveAll` nos diretórios de cache sob verificação rigorosa de `safety.Blacklist` e proteção contra travessia de symlinks (`safety.IsSymlink`), sem invocar comandos de terceiros (`npx`, `cypress`) e operando 100% offline. — *Reversibility:* reversible.

### Shell & CLI Tools Cleaner (DEV-02)
- **D-06:** **Novo Cleaner Unificado 'internal/cleaner/shell':** Criar o cleaner `internal/cleaner/shell` registrado sob o ecossistema `"shell"` no motor (`internal/engine/register.go`) e no config (`internal/config/config.go`), consolidando as ferramentas CLI do desenvolvedor. — *Reversibility:* costly — adiciona um novo ecossistema no registro do motor e modelo de configuração.
- **D-07:** **Separação de Alvos no mise:** Segmentar mise em dois alvos distintos: `mise cache` (`~/Library/Caches/mise` ou `~/.cache/mise`) com risco Low, e `mise downloads` (`~/.local/share/mise/downloads`) com risco Low, permitindo limpeza de arquivos residuais de download de runtimes já instalados. — *Reversibility:* reversible.
- **D-08:** **Alvo Agregado de Autocompletar do Zsh:** Descobrir dumps de autocompletar via padrão `~/.zcompdump*` no `$HOME` (e `$ZDOTDIR` se configurado) e agrupá-los em um único alvo consolidado `Zsh completion cache`, mantendo a listagem concisa e sem tocar em `.zshrc` ou `.zsh_history`. — *Reversibility:* reversible.
- **D-09:** **Classificação de Risco de Ferramentas CLI:** Atribuir `RiskLow` para `gh` e `Oh My Zsh`, e `RiskMedium` para `pre-commit` (devido ao custo de rede para re-clonar repositórios de hooks caso o desenvolvedor realize commits subsequentes offline). — *Reversibility:* reversible.
- **D-10:** **IsInstalled por Presença em Disco no Shell Cleaner:** `IsInstalled()` retorna `true` se qualquer um dos diretórios de cache (`mise`, `gh`, `pre-commit`, `omz`, `zcompdump`) existir ou se algum dos comandos estiver no PATH, respeitando o princípio de descoberta puramente por disco. — *Reversibility:* reversible.

### Xcode, Simulators & Process Guard (DEV-03)
- **D-11:** **Bloqueio Fail-Closed com ReviewOnly sob Processo Ativo:** Quando `Xcode` ou `Simulator.app` estiverem em execução ou sua detecção retornar erro/estado incerto, os alvos afetados do Xcode (`CoreSimulator Caches`, `CoreSimulator Temp`, `iOS DeviceSupport`, `watchOS DeviceSupport`, `DerivedData` e `Archives`) são marcados com `ReviewOnly: true`, `ReviewReason: domain.ReviewReasonActiveProcess`, `EligibleBytes: 0` e `TTLEligible: false`. — *Reversibility:* costly — integra o cleaner de Xcode com o portão de segurança do Coordinator.
- **D-12:** **Verificação Injetável de Processos com CommandExecutor:** Criar mecanismo de detecção de processos baseado em `exec.CommandContext("pgrep", "-x", processName)` através da interface `cleaner.CommandExecutor`, garantindo testabilidade hermética com mocks e zero uso de CGO. — *Reversibility:* costly — estabelece o padrão de verificação de processos reutilizável nas Fases 9, 11 e 12.
- **D-13:** **Política de Segurança para Xcode Archives:** `~/Library/Developer/Xcode/Archives` recebe classificação `RiskHigh` quando o Xcode estiver fechado (alertando sobre a perda potencial de símbolos dSYM para depuração de releases de produção), e é bloqueado com `ReviewOnly` se o Xcode estiver em execução. — *Reversibility:* reversible.
- **D-14:** **Alvos Dedicados para CoreSimulator e DeviceSupport:** Segmentar alvos individuais para `CoreSimulator Caches`, `CoreSimulator Temp`, `iOS DeviceSupport` e `watchOS DeviceSupport`, permitindo exclusão direta de filesystem via `os.RemoveAll` quando as ferramentas estiverem inativas. — *Reversibility:* reversible.

### Agent Discretion
- Formatação e nomes exatos dos identificadores de alvos (IDs e Categories em `domain.Target`).
- Definição de constantes de caminhos específicos por plataforma (Darwin vs Linux) usando helpers de diretório de usuário.
- Heurísticas de detecção de múltiplos arquivos de dump do Zsh (`filepath.Glob`).

---

## Architecture Overview & Code Patterns

### 1. Target Modeling & Lifecycle
All cleaners produce instances of `domain.Target`:
```go
type Target struct {
    ID            string        `json:"id"`
    Name          string        `json:"name"`
    Path          string        `json:"path"`
    SizeBytes     int64         `json:"size_bytes"`
    EligibleBytes int64         `json:"eligible_bytes"`
    TTLEligible   bool          `json:"ttl_eligible"`
    ReviewOnly    bool          `json:"review_only"`
    ReviewReason  string        `json:"review_reason,omitempty"`
    Risk          RiskLevel     `json:"risk"`
    LastAccessed  time.Time     `json:"last_accessed"`
    Category      string        `json:"category"`
}
```

Key invariants established in Phase 6 and Phase 7:
- **Scan vs TTL separation (SCAN-01):** `SizeBytes` always reports 100% of real disk bytes found via `devfs.CalcDirSize(path)`. `EligibleBytes` and `TTLEligible` reflect whether `rules.MaxAgeDays == 0 || info.ModTime().Before(cutoff)`.
- **ReviewOnly Gate (POL-01, POL-02):** When a target has `ReviewOnly: true`, its `EligibleBytes` is forced to `0`, `TTLEligible` to `false`, and `ReviewReason` is set (e.g. `domain.ReviewReasonActiveProcess`). In Stage 1 of `deleter.Coordinator`, any `ReviewOnly` target is intercepted before trash or deletion sinks.
- **Canonical Ownership (SCAN-04):** Cleaners in `TierDedicated` take precedence over `TierGeneric`. `node`, `pnpm`, `shell`, and `xcode` all resolve to `TierDedicated` in `internal/engine/ownership.go`.

### 2. Ecosystem Registration Flow
Adding the `"shell"` ecosystem and wiring the expanded cleaners requires touching:
1. `internal/cleaner/shell/shell.go`: New package implementing `domain.Cleaner`.
2. `internal/config/config.go`:
   - Add `Shell EcosystemConfig` to `EcosystemsConfig` (`mapstructure:"shell"`).
   - Add `Shell HookConfig` to `HooksConfig` (`mapstructure:"shell"`).
   - Add defaults in `Load()`.
3. `internal/config/defaults.go`: Add `Shell: EcosystemConfig{Enabled: true, MaxAgeDays: 30, FailMode: "continue"}` and `Shell: HookConfig{Timeout: "30s"}`.
4. `internal/config/validate.go`: Add `"shell": true` to `allowedCleanValues` and register `hooks["hooks.shell"] = c.Hooks.Shell`.
5. `internal/config/save.go`: Add YAML comments/block for `shell` configuration.
6. `internal/engine/engine.go`: Add `case "shell": return e.config.Ecosystems.Shell` in `ecosystemConfig()`.
7. `internal/engine/register.go`: Register `shell.NewWithExecutor(exec)` alongside `pnpm`, `node`, and `xcode`.
8. `internal/tui/config.go`: Add `shell` entry to TUI ecosystem list.

---

## Target Analysis by Ecosystem

### Ecosystem 1: Node & Headless Automation (`internal/cleaner/node` & `internal/cleaner/pnpm`)

#### Node Cleaner (`internal/cleaner/node/node.go`)
Expands `scanGlobalCaches(rules domain.EcosystemRules) []domain.Target` to inventory:

| Target Name | Category | Risk | Darwin Path(s) | Linux Path(s) | Notes |
|---|---|---|---|---|---|
| `npm cache` | `npm-cache` | Low | `~/.npm/_cacache` | `~/.npm/_cacache` | Existing |
| `npx cache` | `npx-cache` | Low | `~/.npm/_npx` | `~/.npm/_npx` | D-01, D-03 |
| `npm logs` | `npm-logs` | Low | `~/.npm/_logs` | `~/.npm/_logs` | D-01, D-03 |
| `Corepack cache` | `corepack-cache` | Low | `~/Library/Caches/node/corepack`, `~/.cache/node/corepack` | `~/.cache/node/corepack` | D-01 |
| `Puppeteer browser cache` | `node-puppeteer` | Medium | `~/.cache/puppeteer` (or `$PUPPETEER_CACHE_DIR`) | `~/.cache/puppeteer` | D-01, D-02 |
| `Playwright browser cache` | `node-playwright` | Medium | `~/Library/Caches/ms-playwright`, `~/.cache/ms-playwright` (or `$PLAYWRIGHT_BROWSERS_PATH`) | `~/.cache/ms-playwright` | D-01, D-02 |
| `Cypress binary cache` | `node-cypress` | Low | `~/Library/Caches/Cypress`, `~/.cache/Cypress` (or `$CYPRESS_CACHE_FOLDER`) | `~/.cache/Cypress` | D-01, D-02 |

*Path Resolution Pattern:*
Check candidates in order. On macOS, tools like Playwright and Cypress use `~/Library/Caches/<name>` by default, but developers using XDG conventions may also have `~/.cache/<name>`. Checking each candidate and emitting if present ensures discovery even under custom configurations.

*Deletion Policy (D-05):*
In `Clean()`, each path is validated against `safety.IsCoreBlacklisted(path)` and `safety.IsSymlink(path)` before deletion via `os.RemoveAll`. Third-party command invocations (`npx`, `cypress cache clear`) are avoided for offline reliability.

#### pnpm Cleaner (`internal/cleaner/pnpm/pnpm.go`)
- **Metadata Cache Path (D-04):**
  - Darwin: `~/Library/Caches/pnpm` (and fallback `~/.cache/pnpm`)
  - Linux: `~/.cache/pnpm`
- **Category:** `pnpm-cache`
- **Name:** `"pnpm metadata cache"`
- **Risk:** `domain.RiskLow`
- **`IsInstalled()` Update (D-04):**
  In addition to `c.exec.LookPath("pnpm")` and candidate store paths (`~/Library/pnpm/store`, `~/.local/share/pnpm/store`, `~/.pnpm-store`), check if `~/Library/Caches/pnpm` or `~/.cache/pnpm` exists on disk! If either exists, `IsInstalled()` returns `true`.
- **`Clean()` Update:**
  For `t.Path != "pnpm://store/prune"`, remove the directory using `os.RemoveAll(t.Path)` after safety checks.

---

### Ecosystem 2: Shell & CLI Tools (`internal/cleaner/shell`)

#### Package Structure
`internal/cleaner/shell/`:
- `shell.go`: Cleaner implementation, discovery methods, and filesystem cleanup.
- `shell_test.go`: Unit tests with `t.TempDir()`, sentinels, symlink fixtures, permission error handling, and DryRun verification.

#### Target Inventory (DEV-02)

| Target Name | Category | Risk | Path(s) | Notes |
|---|---|---|---|---|
| `mise cache` | `shell-mise-cache` | Low | Darwin: `~/Library/Caches/mise`, `~/.cache/mise`<br>Linux: `~/.cache/mise` | D-07 |
| `mise downloads` | `shell-mise-downloads` | Low | `~/.local/share/mise/downloads` (or `$XDG_DATA_HOME/mise/downloads`) | D-07 |
| `GitHub CLI cache` | `shell-gh-cache` | Low | Darwin: `~/Library/Caches/gh`, `~/.cache/gh`<br>Linux: `~/.cache/gh` | D-09 |
| `pre-commit cache` | `shell-pre-commit-cache` | Medium | `~/.cache/pre-commit` (or `$PRE_COMMIT_HOME`) | D-09 |
| `Oh My Zsh cache` | `shell-omz-cache` | Low | `~/.oh-my-zsh/cache` (or `$ZSH_CACHE_DIR`) | D-09 |
| `Zsh completion cache` | `shell-zsh-compdump` | Low | Aggregated: `~/.zcompdump*` in `$HOME` (and `$ZDOTDIR`) | D-08 |

#### Strict Non-Interference Guarantee
The cleaner NEVER reads, modifies, touches, or stats:
- `.zshrc`, `.bashrc`, `.profile`, `.bash_profile`, `config.fish`
- `.zsh_history`, `.bash_history`
- Shell secrets or environment variables (`.env`, `.ssh`)

#### Zsh Completion Dumps Aggregation (D-08)
- Search pattern: `filepath.Glob(filepath.Join(home, ".zcompdump*"))`
- If `$ZDOTDIR` is configured and `zdotdir != home`, also check `filepath.Glob(filepath.Join(zdotdir, ".zcompdump*"))`
- Sum the sizes of all matched regular files.
- If matches are found, emit **one single consolidated target**:
  - `ID`: `"shell-zsh-compdump"`
  - `Name`: `"Zsh completion cache"`
  - `Path`: `"shell://zsh/compdump"` (Virtual target, ensuring no conflict with root paths)
  - `SizeBytes`: total sum of all matched `.zcompdump*` files
  - `Risk`: `domain.RiskLow`
  - `Category`: `"shell-zsh-compdump"`
- In `Clean()`: when `t.Path == "shell://zsh/compdump"`, glob matching files and delete each using `os.Remove(f)`.

#### `IsInstalled()` Policy (D-10)
`IsInstalled()` returns `true` if:
1. `c.exec.LookPath("mise")`, `c.exec.LookPath("gh")`, `c.exec.LookPath("pre-commit")`, or `c.exec.LookPath("zsh")` succeeds, OR
2. ANY of the cache or download paths exist on disk.

---

### Ecosystem 3: Xcode, Simulators & Process Guard (`internal/cleaner/xcode`)

#### Target Inventory (DEV-03)

| Target Name | Category | Risk | Path | Notes |
|---|---|---|---|---|
| `Xcode DerivedData: <proj>` | `xcode-derived-data` | Low | `~/Library/Developer/Xcode/DerivedData/<proj>` | Existing |
| `CoreSimulator Caches` | `xcode-coresimulator-caches` | Low | `~/Library/Developer/CoreSimulator/Caches` | D-11, D-14 |
| `CoreSimulator Temp` | `xcode-coresimulator-temp` | Low | `~/Library/Developer/CoreSimulator/Temp` | D-11, D-14 |
| `Xcode iOS DeviceSupport` | `xcode-ios-devicesupport` | Low | `~/Library/Developer/Xcode/iOS DeviceSupport` | D-11, D-14 |
| `Xcode watchOS DeviceSupport` | `xcode-watchos-devicesupport` | Low | `~/Library/Developer/Xcode/watchOS DeviceSupport` | D-11, D-14 |
| `Xcode Archives` | `xcode-archives` | **High** | `~/Library/Developer/Xcode/Archives` | D-11, D-13 (dSYM warning) |
| `iOS Simulators (unavailable runtimes)` | `virtual-xcode-simulators` | Low | `xcode://simulators/unavailable` | Virtual target |

#### Process Guard & Fail-Closed Detection (D-11, D-12)
Xcode compilation or simulator execution must not be interrupted by cache deletion.

**Detection Mechanism:**
Invoke `pgrep -x <name>` via `cleaner.CommandExecutor`:
1. `pgrep -x Xcode`
2. `pgrep -x Simulator`

**Process State Evaluation:**
```go
// checkProcess returns (running bool, uncertain bool)
func checkProcess(ctx context.Context, exec cleaner.CommandExecutor, name string) (bool, bool) {
    out, err := exec.Output(ctx, "pgrep", "-x", name)
    if err == nil && len(strings.TrimSpace(string(out))) > 0 {
        return true, false // definitely running
    }
    
    // Check if err is exit code 1 (clean "no match found")
    var exitErr interface{ ExitCode() int }
    if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
        return false, false // definitely not running
    }
    
    // Any other error (binary missing, permission denied, exit code > 1, context canceled)
    // is treated as UNCERTAIN -> FAIL CLOSED!
    return false, true
}
```

**Guard Enforcement:**
- If `Xcode` is active or uncertain:
  ALL targets (`DerivedData/*`, `Archives`, `iOS DeviceSupport`, `watchOS DeviceSupport`, `CoreSimulator Caches`, `CoreSimulator Temp`, `xcode://simulators/unavailable`) receive:
  - `ReviewOnly = true`
  - `ReviewReason = domain.ReviewReasonActiveProcess`
  - `EligibleBytes = 0`
  - `TTLEligible = false`
- If `Simulator` is active or uncertain (and Xcode is closed):
  Simulator-related targets (`CoreSimulator Caches`, `CoreSimulator Temp`, `xcode://simulators/unavailable`) receive:
  - `ReviewOnly = true`
  - `ReviewReason = domain.ReviewReasonActiveProcess`
  - `EligibleBytes = 0`
  - `TTLEligible = false`

**Testability with Zero CGO:**
Using `cleaner.CommandExecutor` allows injecting `cleaner.MockCommandExecutor` in unit tests:
- To test process running: `mockExec.Outputs["pgrep -x Xcode"] = []byte("54321\n")`
- To test process not running: configure `mockExec.Errors["pgrep -x Xcode"] = &cleaner.MockExitError{Code: 1}`
- To test fail-closed error: `mockExec.Errors["pgrep -x Xcode"] = errors.New("pgrep: permission denied")`

---

## Validation Architecture

### 1. Test Framework
- Standard Go testing package (`testing.T`, `go test`).
- Assertions with descriptive diffs, hermetic temp dirs with `t.TempDir()`, and subtests with `t.Run()`.

### 2. Execution Commands
- **Quick Run Command:**
  ```bash
  go test -v -run "TestNode|TestPnpm|TestShell|TestXcode" ./internal/cleaner/...
  ```
- **Altered-Package Coverage Command:**
  ```bash
  go test -v -cover ./internal/cleaner/node ./internal/cleaner/pnpm ./internal/cleaner/shell ./internal/cleaner/xcode
  ```
- **Full Suite Command:**
  ```bash
  go test ./...
  ```

### 3. Requirements to Tests Mapping

| Requirement | Test Function(s) | Package | Description |
|---|---|---|---|
| **DEV-01** | `TestNodeCleaner_Scan_GlobalCaches_AutomationAndNpm` | `internal/cleaner/node` | Verifies `_cacache`, `_npx`, `_logs`, Corepack, Puppeteer, Playwright, and Cypress discovery by disk presence. |
| **DEV-01** | `TestNodeCleaner_RiskLevels_Automation` | `internal/cleaner/node` | Verifies RiskMedium on Puppeteer/Playwright and RiskLow on Cypress/npm/Corepack. |
| **DEV-01** | `TestPnpmCleaner_Scan_DiskMetadataCache` | `internal/cleaner/pnpm` | Verifies pnpm HTTP metadata cache discovery without pnpm in PATH. |
| **DEV-01** | `TestPnpmCleaner_IsInstalled_DiskPresence` | `internal/cleaner/pnpm` | Verifies `IsInstalled() == true` when metadata cache exists even if LookPath fails. |
| **DEV-02** | `TestShellCleaner_IsInstalled` | `internal/cleaner/shell` | Verifies disk-presence and LookPath detection for shell tools. |
| **DEV-02** | `TestShellCleaner_Scan_MiseSeparateTargets` | `internal/cleaner/shell` | Verifies separate `mise cache` and `mise downloads` targets. |
| **DEV-02** | `TestShellCleaner_Scan_ZshCompletionAggregated` | `internal/cleaner/shell` | Verifies aggregation of multiple `~/.zcompdump*` files into single `Zsh completion cache`. |
| **DEV-02** | `TestShellCleaner_NonInterferenceWithShellRC` | `internal/cleaner/shell` | Asserts `.zshrc`, `.bashrc`, and `.zsh_history` are never touched or modified. |
| **DEV-02** | `TestShellCleaner_RiskLevels` | `internal/cleaner/shell` | Verifies RiskLow for gh/omz/mise/zcompdump and RiskMedium for pre-commit. |
| **DEV-03** | `TestXcodeCleaner_Scan_ExpandedTargets` | `internal/cleaner/xcode` | Verifies `CoreSimulator Caches/Temp`, `iOS/watchOS DeviceSupport`, and `Archives` discovery. |
| **DEV-03** | `TestXcodeCleaner_ProcessGuard_XcodeActive` | `internal/cleaner/xcode` | Verifies all targets receive `ReviewOnly: true` (`ReviewReasonActiveProcess`, `EligibleBytes: 0`) when Xcode is running. |
| **DEV-03** | `TestXcodeCleaner_ProcessGuard_SimulatorActive` | `internal/cleaner/xcode` | Verifies CoreSimulator targets receive `ReviewOnly: true` when Simulator is running. |
| **DEV-03** | `TestXcodeCleaner_ProcessGuard_FailClosedOnPgrepError` | `internal/cleaner/xcode` | Verifies fail-closed behavior when pgrep returns an unknown error. |
| **DEV-03** | `TestXcodeCleaner_Archives_RiskHighWhenClosed` | `internal/cleaner/xcode` | Verifies `Archives` target has `RiskHigh` when Xcode is closed. |
| **VAL-01** | `Test<Cleaner>_Clean_SymlinkProtection` | all cleaner pkgs | Asserts symlink targets are skipped and protected from traversal. |
| **VAL-01** | `Test<Cleaner>_DryRun_ZeroMutation` | `internal/deleter` | Asserts no cleaner or directory mutation occurs under DryRun. |

### 4. Hermetic Test Strategy
- **`t.TempDir()` Isolation:** All tests use `t.TempDir()`, overriding `$HOME`, cache directories, and cleaner base paths.
- **Sentinels:** Real files (e.g., `sentinel.txt`, `browser.bin`, `dSYM.plist`) are created inside mock cache directories to verify byte calculation and safe deletion.
- **Symlink Guard Tests:** Tests construct symlinks pointing outside the cache directory (including to blacklisted paths like `.git` or `/etc`) and verify `safety.IsSymlink()` or `safety.IsCoreBlacklisted()` prevents traversal or deletion.
- **Permission Errors:** Tests verify graceful error handling when a directory has restricted permissions (e.g. `0o000` on unix).
- **DryRun Zero-Mutation:** Tests invoke `deleter.Coordinator` with `Options{DryRun: true}` and assert all files and directories retain their original presence and mtime.
- **Mock CommandExecutor:** Unit tests use `cleaner.MockCommandExecutor` to inject process states without running system `pgrep` or requiring running GUI applications.

---

## Package Legitimacy Audit

DevClean v2.3 enforces zero-CGO and strict dependency hygiene.
- **External Dependencies:** ZERO new dependencies required.
  - Standard library (`os`, `path/filepath`, `strings`, `context`, `errors`, `time`, `crypto/sha256`) handles file walking, path resolution, hashing, and process execution.
  - Existing internal modules (`internal/cleaner`, `internal/fs`, `internal/safety`, `internal/domain`, `internal/config`, `internal/engine`) provide all necessary primitives.
- **Zero-CGO:** All process lookups rely on `cleaner.CommandExecutor` (`pgrep`), ensuring 100% pure Go compilation across Darwin and Linux.

---

## Implementation Plan Outline & Wave Dependencies

The implementation can be broken down into 4 clear plans across 2 waves:

```
Wave 1 (Parallel):
├── Plan 08-01: Node & Headless Automation Caches Expansion (DEV-01)
│   └── internal/cleaner/node, internal/cleaner/pnpm
├── Plan 08-02: Shell & CLI Tools Cleaner (DEV-02)
│   └── internal/cleaner/shell, internal/config, internal/engine
└── Plan 08-03: Xcode & Simulators Expansion with Process Guard (DEV-03)
    └── internal/cleaner/xcode, internal/cleaner/executor_mock

Wave 2 (Integration & Verification):
└── Plan 08-04: Integration, Coordinator Verification & Hermetic Suite (DEV-01..03, VAL-01..02)
    └── internal/engine/register, config verification, end-to-end deleter/coordinator tests
```

- **Plan 08-01 (Node & pnpm):** Implement granular targets in `node.go` (`npm cache/npx/logs`, `corepack`, `puppeteer`, `playwright`, `cypress`) and disk-based metadata cache in `pnpm.go`.
- **Plan 08-02 (Shell Cleaner):** Implement `internal/cleaner/shell`, config bindings, engine registration, and zcompdump aggregation.
- **Plan 08-03 (Xcode & Process Guard):** Implement `CoreSimulator Caches/Temp`, `iOS/watchOS DeviceSupport`, `Archives`, and injectable fail-closed `pgrep` process detection.
- **Plan 08-04 (Integration & Coverage):** Connect all cleaners in `register.go`, verify coverage > 85%, execute hermetic symlink, sentinel, DryRun, and permission tests.

---

## RESEARCH COMPLETE

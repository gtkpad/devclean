# Phase 11: macOS User/System Caches & Logs - Research

**Date:** 2026-10-16  
**Status:** Complete  
**Requirements Covered:** `MAC-01`, `MAC-02`, `MAC-03`, `MAC-04`, `MAC-05`, `POL-01`, `POL-02`, `POL-03`, `POL-04`, `SCAN-04`, `VAL-01`, `VAL-02`

---

## User Constraints (from CONTEXT.md)

### Locked Decisions

### Trash & Volumes Externos (MAC-01)
- **D-01:** **Lixeira sempre ReviewOnly por Padrão:** `~/.Trash` é sempre contabilizado no inventário de scan como `ReviewOnly: true`, `ReviewReason: "trash_requires_flag"`, `EligibleBytes: 0` e `TTLEligible: false`. Apenas quando a flag `--trash` (ou config `safety.trash_enabled`) for fornecida na limpeza é que o alvo torna-se elegível para remoção. — **Reversibility:** costly — define a política central de elegibilidade da Lixeira.
- **D-02:** **Escopo Restrito a `$HOME/.Trash`:** A varredura de Lixeira limita-se estritamente ao diretório do usuário `$HOME/.Trash`, ignorando deliberadamente `/Volumes/*/.Trashes` em mídias removíveis ou volumes secundários para mitigar riscos de desmontagem ou falhas de permissão externas. — **Reversibility:** reversible — restringe o caminho escaneado da Lixeira.
- **D-03:** **Expurgo Direto dos Itens da Lixeira:** Quando autorizado com `--trash`, o cleaner realiza a deleção permanente direta (`os.RemoveAll`) dos itens contidos em `~/.Trash`, sem movê-los para a quarentena reversível interna (`internal/trash`), e preservando a própria pasta raiz `~/.Trash`. — **Reversibility:** costly — define o comportamento destrutivo autorizado da limpeza de lixeira.
- **D-04:** **Degradação Tolerante em Arquivos Bloqueados:** Ao esvaziar a Lixeira, tentar `chmod u+w` em itens somente-leitura. Caso algum item continue inacessível ou travado pelo sistema/TCC, registrar aviso detalhado com bytes remanescentes sem abortar a limpeza dos demais itens. — **Reversibility:** reversible.

### Granularidade e Deduplicação de User Caches (MAC-01, MAC-03)
- **D-05:** **Lista Estrita de Exclusão de Caches Conhecidos:** O cleaner de macOS mantém uma tabela de exclusão contendo os caminhos e subpastas de `~/Library/Caches` já gerenciados por cleaners dedicados (`uv`, `pnpm`, `mise`, `ms-playwright`, `cypress`, `homebrew`, `cocoapods`, `xcode`, `orbstack`, editores AI/Electron), garantindo zero dupla contagem de bytes. — **Reversibility:** costly — define o contrato de convivência entre o cleaner macOS e ecossistemas dedicados.
- **D-06:** **Consolidação Categórica dos Alvos de Usuário:** Apresentar alvos consolidados na CLI/TUI (`User App Caches` e `User App Logs`), agrupando os bundles secundários em vez de listar centenas de linhas avulsas, destacando alvos individuais apenas para itens maiores (> 50MB) ou com regras específicas. — **Reversibility:** reversible.
- **D-07:** **Folhas Restritas em Application Support:** Varrer estritamente subpastas permitidas (`Cache`, `Caches`, `logs`, `Logs`, `Crashpad/completed`) dentro de `~/Library/Application Support/*`, mantendo um alvo dedicado para o caso de alto volume `Spotify PersistentCache` e consolidando os demais em `Application Support Caches & Logs`. — **Reversibility:** costly — garante que bancos de dados e preferências em Application Support nunca sejam tocados.
- **D-08:** **Positive Allowlist para Group Containers:** Em `~/Library/Group Containers`, varrer apenas pastas `Library/Caches` e `Library/Logs` de grupos explicitamente autorizados na positive allowlist, excluindo estritamente `*.dev.orbstack` (pertencente ao cleaner docker) e nunca tocando a raiz do Group Container. — **Reversibility:** costly — protege dados de compartilhamento entre aplicativos no macOS.

### Permissões TCC e Degradação Graciosa em Logs/Sistema (MAC-04)
- **D-09:** **Notação de Cobertura Incompleta via `ProviderMetrics`:** Quando o acesso a `/private/var/log` ou `/Library/Logs/DiagnosticReports` falhar por TCC ou permissão negada, preencher `ProviderMetrics.Partial = true` e `StatusNote: "permission denied (TCC / root required)"`, marcando o alvo como `ReviewOnly: true` (`ReviewReason: domain.ReviewReasonSystemProtected`) e acumulando os bytes que puderam ser lidos. — **Reversibility:** costly — padroniza o reporte de TCC em toda a suite do DevClean.
- **D-10:** **Zero Sudo / Sem Elevação de Privilégios:** O DevClean opera estritamente com os privilégios do processo do usuário, sem invocar `sudo` ou solicitar elevação interativa. Logs inacessíveis do sistema permanecem reportados como protegidos pelo sistema. — **Reversibility:** one-way — compromisso de segurança de nunca executar comandos de elevação de privilégio.
- **D-11:** **Isolamento de `*.code_sign_clone` no Bucket do Usuário:** Localizar a pasta de cache Darwin do usuário via `os.Getenv("TMPDIR")` / `getconf DARWIN_USER_CACHE_DIR`, buscando `*.code_sign_clone` exclusivamente dentro do subdiretório `C/` do usuário corrente, sem escanear pastas de outros usuários em `/private/var/folders`. — **Reversibility:** reversible.
- **D-12:** **Segregação de Relatórios de Diagnóstico:** Separar a descoberta em dois alvos distintos: `User Diagnostic Reports` (`~/Library/Logs/DiagnosticReports`, elegível para limpeza) e `System Diagnostic Reports` (`/Library/Logs/DiagnosticReports`, marcado como ReviewOnly se faltar permissão ou for do sistema). — **Reversibility:** reversible.

### Positive Allowlists & Proteção de Dados Sensíveis (MAC-02, MAC-05)
- **D-13:** **Positive Allowlist Canônica dos 8 Daemons:** Cobrir estritamente os 8 daemons declarados na spec (`Siri suggestions`, `Wallpaper agent`, `com.apple.mediaanalysisd`, `Geod`, `com.apple.helpd`, `Apple Media Services`, `Parsecd` e `Handoff`), agrupados sob o alvo `macOS Daemon Caches`. — **Reversibility:** costly — define o escopo positivo de daemons da Apple.
- **D-14:** **Process Guard Determinístico via `cleaner.CommandExecutor`:** Implementar mapeamento determinístico de bundle IDs para nomes de processos executando `pgrep -x`. Se o processo estiver ativo ou incerto (fail-closed), o alvo recebe `ReviewOnly = true`, `ReviewReason = domain.ReviewReasonActiveProcess`, `EligibleBytes = 0` e revalida obrigatoriamente pré-Clean. — **Reversibility:** costly — integra o Process Guard no ciclo de vida do cleaner macOS.
- **D-15:** **Whitelist Imutável de Bundles Sensíveis em Go:** Criar `internal/cleaner/macos/whitelist.go` com lista imutável e inegociável contendo `CloudKit`, `Mobile Documents`, `com.apple.keychain*`, `Safari` state, VPNs (`Tailscale`, `WireGuard`) e gerenciadores de senhas (`1Password`), imune a qualquer flag ou configuração. — **Reversibility:** one-way — garantia de segurança inviolável do ecossistema macOS.
- **D-16:** **Symlink Guard e Verificação de Propriedade (Ownership) com Zero-CGO:** Rejeitar travessia de symlinks que apontem para fora da raiz do cache, nunca seguir links para locais de sistema ou protegidos, e verificar propriedade via `syscall.Stat_t` (`st_uid == current_uid`) mantendo compilação 100% pura sem CGO. Alvos com symlinks ambíguos recebem `ReviewOnly = true` (`ReviewReasonAmbiguous`). — **Reversibility:** costly — define a verificação de segurança de baixo nível do filesystem.

### the agent's Discretion

- Formatação exata dos nomes legíveis dos alvos consolidados na CLI e TUI.
- Limite exato em MB para desmembrar um bundle específico de `User App Caches` em linha própria (padrão adotado: 50MB).
- Definição da lista exata de bundles de Group Containers com caches/logs regeneráveis autorizados.

---

## Architectural Responsibility Map

| Component / Submodule | Specific Responsibility in Phase 11 |
|---|---|
| `internal/cleaner/macos/macos.go` | Orchestrates the `macos` cleaner implementation conforming to `domain.Cleaner`. Coordinates scanning across user essentials, Apple daemons, group containers, application support leaves, system logs, and code-sign clones. Implements defensive `Clean()` with pre-deletion process revalidation and ownership verification. |
| `internal/cleaner/macos/whitelist.go` | Inviolable, hardcoded in-Go security whitelist (`D-15`). Matches sensitive Apple data (`CloudKit`, `Mobile Documents`, `keychain*`, `Safari` state), VPNs (`Tailscale`, `WireGuard`, `ZeroTier`, `Cloudflare WARP`), and password managers (`1Password`, `Bitwarden`, `KeePassXC`). Immune to user flags or configuration override. |
| `internal/cleaner/macos/process.go` | Deterministic process guard mapping bundle IDs and daemon names to running processes using `cleaner.CommandExecutor` with `pgrep -x` (`D-14`). Implements fail-closed logic: returns `ReviewOnly: true` with `ReviewReasonActiveProcess` if running or uncertain. Revalidates immediately before deletion. |
| `internal/cleaner/macos/ownership_darwin.go` | Darwin platform-specific ownership check using pure Go `syscall.Stat_t.Uid == uint32(os.Getuid())` (`D-16`). Preserves zero-CGO compilation. |
| `internal/cleaner/macos/ownership_other.go` | Portable fallback for Linux/other operating systems for hermetic unit testing with `t.TempDir()`. |
| `internal/cleaner/macos/user.go` | Scans `~/Library/Caches/*` and `~/Library/Logs/*` (`MAC-01`). Applies deduplication exclusion table (`D-05`) for paths owned by other cleaners (`pnpm`, `uv`, `Homebrew`, `CocoaPods`, `vcpkg`, `mise`, `ms-playwright`, `Cypress`, `OrbStack`, AI/Electron editors). Consolidates bundles <=50MB into `User App Caches` and `User App Logs` (`D-06`). |
| `internal/cleaner/macos/trash.go` | Scans `$HOME/.Trash` (`D-01`, `D-02`). Reports `~/.Trash` as `ReviewOnly: true`, `ReviewReason: "trash_requires_flag"`, `EligibleBytes: 0`, `TTLEligible: false`. When authorized (`--trash`), directly purges items contained within `~/.Trash` via `os.RemoveAll` without internal quarantine, unlocking read-only items (`chmod u+w`), and preserving the root `~/.Trash` folder (`D-03`, `D-04`). |
| `internal/cleaner/macos/daemons.go` | Positive allowlist scanner for canonical 8 Apple daemons (`D-13`): Siri suggestions, Wallpaper agent, `com.apple.mediaanalysisd`, `com.apple.geod`, `com.apple.helpd`, Apple Media Services, `com.apple.parsecd`, and Handoff. Consolidated under `macOS Daemon Caches`. |
| `internal/cleaner/macos/groups.go` | Positive allowlist scanner for `~/Library/Group Containers` (`D-08`). Scans only allowlisted groups and strictly subpaths `Library/Caches`, `Library/Logs`, `Caches`, `Logs`. Strictly excludes `*.dev.orbstack` (handled by Docker cleaner). Never removes the root container directory. |
| `internal/cleaner/macos/appsupport.go` | Restrictive scanner for `~/Library/Application Support/*` (`MAC-03`, `D-07`). Only targets regenerable leaf subdirectories: `Cache`, `Caches`, `logs`, `Logs`, `Crashpad/completed`. Discovers dedicated target for `Spotify PersistentCache` with active process protection. Never touches databases, preferences, or application state. |
| `internal/cleaner/macos/system.go` | Scans `User Diagnostic Reports` (`~/Library/Logs/DiagnosticReports`, eligible) vs `System Diagnostic Reports` (`/Library/Logs/DiagnosticReports`, ReviewOnly) (`D-12`). Scans `/private/var/log` (ReviewOnly). Degrades gracefully on TCC or permission denial with `ProviderMetrics.Partial = true` and `StatusNote` (`D-09`). Scans `*.code_sign_clone` isolated to user's `TMPDIR` / `DARWIN_USER_CACHE_DIR` subfolder `C/` (`D-11`). Zero sudo / root elevation (`D-10`). |
| `internal/config/config.go` & `defaults.go` | Registers `MacOS EcosystemConfig` and `MacOS HookConfig` in `EcosystemsConfig` and `HooksConfig` with default `MaxAgeDays: 14`. |
| `internal/engine/register.go` | Wires `macos.NewWithExecutor(exec)` into `RegisterDefaultCleanersWithExecutor`. |
| `cmd/clean.go` | Coordinates `--trash` flag to mark the macOS Trash target eligible when explicitly requested. |

---

## Summary

Phase 11 implements the core macOS system and user cleaner (`internal/cleaner/macos`), achieving parity with macOS maintenance tools like Mole (`tw93/mole`) while enforcing DevClean's ironclad safety boundaries:
1. **User Essentials & Honest Trash Policy (`MAC-01`, `D-01` to `D-06`):** Scans `~/Library/Caches/*`, `~/Library/Logs/*`, and `$HOME/.Trash`. To prevent catastrophic unintended deletions, `~/.Trash` is **always** marked `ReviewOnly: true` during scans (`⊙` in CLI/TUI), with `EligibleBytes: 0`. Only when the user explicitly provides `--trash` during cleanup are the items inside `~/.Trash` purged directly (`os.RemoveAll` with `chmod u+w` recovery) without moving them to DevClean's internal quarantine.
2. **Canonical Deduplication (`MAC-01`, `D-05`):** `~/Library/Caches` contains subdirectories already managed by dedicated ecosystem cleaners (`uv`, `pnpm`, `mise`, `ms-playwright`, `cypress`, `homebrew`, `cocoapods`, `xcode`, `orbstack`, AI/Electron editors). The macOS cleaner maintains a strict exclusion table to ensure zero duplicate byte counting across the application (`SCAN-04`). Secondary bundles <=50MB are neatly consolidated into category rows (`User App Caches` and `User App Logs`).
3. **Positive Allowlist for Apple Daemons & Group Containers (`MAC-02`, `D-08`, `D-13`):** Scans only the 8 specified Apple daemons (Siri suggestions, Wallpaper agent, `com.apple.mediaanalysisd`, `com.apple.geod`, `com.apple.helpd`, Apple Media Services, `com.apple.parsecd`, and Handoff). For `~/Library/Group Containers`, scanning is restricted to allowlisted groups and strictly regenerable subfolders (`Library/Caches` and `Library/Logs`), explicitly excluding `*.dev.orbstack` (handled by the Docker cleaner) and never touching container root folders.
4. **Surgical Application Support Leaves (`MAC-03`, `D-07`):** Application Support directories house vital databases, session tokens, licenses, and workspace files. DevClean strictly scans only safe, regenerable leaf names: `Cache`, `Caches`, `logs`, `Logs`, and `Crashpad/completed`. It provides a dedicated high-volume target for `Spotify PersistentCache` (`~/Library/Application Support/Spotify/PersistentCache`) with active Spotify process guard.
5. **System Logs, DiagnosticReports & Code-Sign Clones (`MAC-04`, `D-09` to `D-12`):** Distinguishes between eligible `User Diagnostic Reports` and protected `System Diagnostic Reports`. System logs (`/private/var/log`) and system diagnostic reports are marked `ReviewOnly: true`. When macOS TCC or POSIX permissions block access, DevClean degrades gracefully: it sets `ProviderMetrics.Partial = true` and `StatusNote: "permission denied (TCC / root required)"`, reporting accumulated readable bytes without failing silently. Code signature clones (`*.code_sign_clone`) are isolated to the current user's Darwin cache directory (`$TMPDIR` / `DARWIN_USER_CACHE_DIR` subfolder `C/`), with zero sudo / root elevation.
6. **Inviolable Whitelist, Process Guard & Zero-CGO Safety (`MAC-05`, `D-14` to `D-16`):** An immutable Go whitelist protects CloudKit, Mobile Documents, Keychain, Safari state, VPNs, and Password Managers. A deterministic process guard queries `pgrep -x` via `cleaner.CommandExecutor` to mark active or uncertain bundles as `ReviewOnly: true` (`ReviewReasonActiveProcess`), with mandatory pre-deletion revalidation in `Clean()`. Symlink guard rejects escaping links, and file ownership is verified using `syscall.Stat_t` (`st_uid == current_uid`) with pure Go, maintaining 100% zero-CGO compatibility (`CGO_ENABLED=0`).

---

## Standard Stack

- **Go 1.25 Standard Library:**
  - `syscall.Stat_t` via `fileInfo.Sys().(*syscall.Stat_t)` for `Uid` and file properties without CGO (`ownership_darwin.go`).
  - `os.Getuid()` for current user UID comparison (Unix/Darwin).
  - `path/filepath` (`filepath.Clean`, `filepath.EvalSymlinks`, `filepath.WalkDir`, `filepath.Rel`).
  - `os` (`os.ReadDir`, `os.RemoveAll`, `os.Lstat`, `os.Chmod`, `os.Getenv`).
  - `strings`, `context`, `errors`, `fmt`, `time`.
- **DevClean Core Primitives:**
  - `cleaner.CommandExecutor` / `cleaner.MockCommandExecutor`: Testable, shell-free command execution for `pgrep -x` and `getconf`.
  - `domain.Target` + `domain.ProviderMetrics`: Contract for targets and partial degradation reporting (`Partial: true`, `StatusNote: "..."`).
  - `domain.ReviewReason*`: Semantic reasons (`ReviewReasonActiveProcess`, `ReviewReasonSystemProtected`, `ReviewReasonAmbiguous`).
  - `safety.IsCoreBlacklisted`, `safety.IsSymlink`: Protection against system root deletion and symlink traversal.
  - `fs.CalcUsage`: Block and inode-aware usage calculation.
  - `fs.ChmodRecursive`: File permission recovery before removal.

---

## Architecture Patterns

### 1. Canonical Deduplication Exclusion Matrix (`MAC-01`, `SCAN-04`, `D-05`)

The macOS cleaner must coexist harmoniously with existing and future specialized cleaners without double-counting bytes in `~/Library/Caches`:

| Ecosystem / Cleaner | Excluded Path(s) in `~/Library/Caches/` | Handled By |
|---|---|---|
| `pnpm` | `pnpm` | `internal/cleaner/pnpm` |
| `python` | `pip`, `uv` | `internal/cleaner/python` |
| `homebrew` | `Homebrew` | `internal/cleaner/homebrew` |
| `cocoapods` | `CocoaPods` | `internal/cleaner/cocoapods` |
| `cpp` | `vcpkg` | `internal/cleaner/cpp` |
| `shell` | `mise`, `gh` | `internal/cleaner/shell` |
| `node` | `ms-playwright`, `Cypress`, `node/corepack`, `Yarn` | `internal/cleaner/node` |
| `docker` | `dev.orbstack.OrbStack` | `internal/cleaner/docker` |
| `ai` (editors) | `com.microsoft.VSCode*`, `Cursor`, `Antigravity`, `com.google.antigravity*`, `Windsurf`, `com.exafunction.windsurf*`, `com.anthropic.claudefordesktop*`, `opencode`, `dev.zed.Zed` | `internal/cleaner/ai` |
| `macos` (daemons) | `com.apple.mediaanalysisd`, `com.apple.geod`, `com.apple.helpd`, `com.apple.parsecd`, `com.apple.AppleMediaServices`, `com.apple.wallpaper*`, `com.apple.siri*`, `com.apple.coreservices.useractivityd` | `internal/cleaner/macos/daemons.go` |
| `browsers` (Phase 12) | `Google/Chrome`, `BraveSoftware/Brave-Browser`, `Arc`, `Microsoft/Edge`, `Firefox`, `com.apple.Safari*` | `internal/cleaner/browsers` |

The exclusion table is matched by exact folder name or relative path prefix under `~/Library/Caches`.

### 2. Trash Eligibility & Direct Purging (`MAC-01`, `D-01`, `D-02`, `D-03`, `D-04`)

Unlike build artifacts which can be automatically deleted based on TTL rules, the Finder Trash (`$HOME/.Trash`) contains arbitrary user files:
1. **Scan Invariant:** During `Scan()`, `~/.Trash` is **always** emitted with:
   - `ReviewOnly: true`
   - `ReviewReason: "trash_requires_flag"`
   - `EligibleBytes: 0`
   - `TTLEligible: false`
   - `Risk: domain.RiskLow`
   - `Category: "macos-trash"`
2. **Exclusion of External Volumes:** Scanning strictly checks `$HOME/.Trash`, deliberately omitting `/Volumes/*/.Trashes` to prevent mount lockups or device disconnection errors.
3. **Clean Authorization & Direct Purge:** When `--trash` is explicitly supplied (or `safety.trash_enabled` is set for clean), the target becomes eligible for deletion. In `Clean()`:
   - DevClean reads entries inside `$HOME/.Trash` using `os.ReadDir`.
   - Each contained item is deleted permanently with `os.RemoveAll`.
   - The root folder `$HOME/.Trash` is preserved.
   - Items are NOT sent to DevClean's internal quarantine (`internal/trash`).
   - If an item fails removal due to read-only permissions (`EACCES`), DevClean attempts `chmod u+w` (or `fs.ChmodRecursive`) and retries. If removal still fails (e.g. system immutable flag `uchg` or active file lock), DevClean records a warning with remaining bytes without aborting deletion of other trash items.

### 3. Strict Positive Allowlist for Apple Daemons and Group Containers (`MAC-02`, `D-08`, `D-13`)

Unbounded cleaning in `~/Library/Caches` or `~/Library/Group Containers` can wipe database states or active user sessions:
- **8 Daemons Allowlist:**
  1. Siri suggestions: `com.apple.siri.context`, `com.apple.sirittsd`, `com.apple.siri.*`
  2. Wallpaper agent: `com.apple.WallpaperAgent`, `com.apple.wallpaper`
  3. Media analysis: `com.apple.mediaanalysisd`
  4. Geod (location services cache): `com.apple.geod`
  5. macOS Help: `com.apple.helpd`
  6. Apple Media Services: `com.apple.AppleMediaServices`
  7. Parsecd: `com.apple.parsecd`
  8. Handoff: `com.apple.coreservices.useractivityd`, `group.com.apple.coreservices.useractivityd/shared-pasteboard`
- **Group Containers Safety:**
  - Group Container roots (`~/Library/Group Containers/<group>`) often store core databases (e.g. Outlook SQLite DBs, OneDrive state).
  - DevClean checks an allowlist of non-critical group IDs and **only** scans child directories literally named `Library/Caches`, `Library/Logs`, `Caches`, or `Logs`.
  - `*.dev.orbstack` is strictly skipped because it contains OrbStack virtual machine disks and container images managed under `internal/cleaner/docker`.

### 4. Application Support Leaf Filtering & Spotify Target (`MAC-03`, `D-07`)

`~/Library/Application Support` contains irreplaceable user application state. DevClean enforces a strict allowlist of leaf directory names:
- Only subdirectories exactly named:
  - `Cache`
  - `Caches`
  - `logs`
  - `Logs`
  - `Crashpad/completed`
- **Spotify Dedicated Target:**
  - Path: `~/Library/Application Support/Spotify/PersistentCache`
  - Reported as an independent target (`Category: "macos-spotify-cache"`, `Name: "Spotify: PersistentCache"`) due to its substantial size (typically 500MB to 5GB).
  - Protected by a dedicated Process Guard for `Spotify`.
- All other qualifying leaf subdirectories are aggregated into `Application Support Caches & Logs`.

### 5. TCC Denial & Graceful Degradation in System Logs (`MAC-04`, `D-09` to `D-12`)

Modern macOS (macOS 14 Sonoma, macOS 15 Sequoia, and beyond) restricts access to system and diagnostic logs via Transparency, Consent, and Control (TCC) and SIP:
- `/Library/Logs/DiagnosticReports` and `/private/var/log` require Full Disk Access (FDA) or root privileges.
- DevClean **never** executes `sudo` or attempts privilege escalation (`D-10`).
- If `os.ReadDir` or `filepath.WalkDir` returns a permission denied error (`os.ErrPermission`, `syscall.EACCES`, `syscall.EPERM`):
  - Target is retained in the scan inventory.
  - `ReviewOnly = true`
  - `ReviewReason = domain.ReviewReasonSystemProtected`
  - `EligibleBytes = 0`
  - `ProviderMetrics.Partial = true`
  - `ProviderMetrics.StatusNote = "permission denied (TCC / root required)"`
  - Readable bytes encountered prior to the permission error are preserved in `SizeBytes`.
  - **Zero silent failures:** The user is explicitly informed why the item is marked review-only.
- `User Diagnostic Reports` (`~/Library/Logs/DiagnosticReports`) remain in the user's home domain and are fully eligible for removal.
- `*.code_sign_clone` files generated by Chromium/Chrome during validation are located exclusively in the user's isolated Darwin cache directory (`$TMPDIR` / `getconf DARWIN_USER_CACHE_DIR` subfolder `C/`), verified to be owned by the user, and guarded against running Chrome/Chromium processes.

### 6. Multi-Layer Security: Whitelist, Symlinks & Ownership (`MAC-05`, `D-14` to `D-16`)

1. **Immutable In-Go Whitelist (`whitelist.go`):**
   Matches bundle IDs, directory names, and path patterns against an inviolable list. If a directory matches `CloudKit`, `Mobile Documents`, `com.apple.keychain*`, `Safari`, VPN profiles (`Tailscale`, `WireGuard`, `ZeroTier`, `Cloudflare WARP`), or Password Managers (`1Password`, `Bitwarden`, `KeePassXC`), scanning and deletion are completely bypassed.
2. **Symlink Guard:**
   - Any symlink encountered as a target root or sub-entry is resolved with `filepath.EvalSymlinks`.
   - If the canonical path points outside the intended cache root, or points to any system root (`/System`, `/Library`, `/usr`, `/etc`, `/var`, `/private`), traversal is rejected.
   - Ambiguous or escaping symlinks receive `ReviewOnly: true` (`ReviewReasonAmbiguous`).
3. **Pure Go Ownership Verification (`ownership_darwin.go`):**
   - For every target prior to removal, inspect `info.Sys().(*syscall.Stat_t).Uid`.
   - If `st_uid != uint32(os.Getuid())`, the target is not owned by the executing user. Deletion is refused.
   - Compiles cleanly under `CGO_ENABLED=0` without any external C library dependencies.
4. **Pre-Deletion Process Revalidation:**
   - Immediately before invoking `os.RemoveAll` on any app cache or daemon cache in `Clean()`, `checkProcessStatus` re-queries `pgrep -x <process>`.
   - If the process was started while DevClean was running, or if the process status is uncertain (fail-closed), deletion is aborted for that target.

---

## Don't Hand-Roll

- **Don't invoke shell commands (`sh -c "rm -rf ~/.Trash/*"`, `sh -c "pgrep ..."`):** Shell strings risk argument injection and bypass safety guards. Use `cleaner.CommandExecutor` with tokenized argument slices (`"pgrep", "-x", processName`) and standard Go filesystem APIs (`os.RemoveAll`, `os.ReadDir`).
- **Don't use `sudo` or prompt for admin password (`osascript -e 'do shell script ... with administrator privileges'`):** DevClean operates with strict least privilege (`D-10`). Unreadable system logs must degrade gracefully via `ProviderMetrics.Partial = true` and `ReviewReasonSystemProtected`.
- **Don't delete `$HOME/.Trash` directory itself:** Only delete items *inside* `$HOME/.Trash`. Deleting the root `.Trash` directory breaks macOS Finder integration and can result in `EPERM` from the kernel.
- **Don't move Trash contents to `internal/trash` quarantine:** Moving items from Trash into another Trash folder is redundant, slow, and confusing. When authorized via `--trash`, delete items directly with `os.RemoveAll`.
- **Don't invent non-standard ReviewReasons:** Use the canonical constants from `internal/domain/types.go` (`ReviewReasonSystemProtected`, `ReviewReasonActiveProcess`, `ReviewReasonAmbiguous`). For trash awaiting explicit flag, use `"trash_requires_flag"`.
- **Don't use CGO or C-bindings for UID checking:** Use Go's standard `os.FileInfo.Sys().(*syscall.Stat_t)` on Darwin, which compiles with `CGO_ENABLED=0`.

---

## Common Pitfalls

### Pitfall 1: Double-Counting Caches Across Cleaners (`SCAN-04`, `D-05`)
- **Symptom:** Total reclaimable space reported by DevClean is inflated by several gigabytes because `pnpm`, `pip`, `mise`, and `CocoaPods` are counted in both `User App Caches` and their respective dedicated cleaners.
- **Cause:** Blindly walking `~/Library/Caches/*` without an exclusion table.
- **Remedy:** Consult the canonical exclusion matrix (`D-05`) before inspecting any directory under `~/Library/Caches`. Skip all folders owned by existing cleaners.

### Pitfall 2: Silent Zero-Byte Reporting on TCC Permission Denials (`MAC-04`, `D-09`)
- **Symptom:** Scan reports `/Library/Logs/DiagnosticReports: 0 B`, making the user believe there are no diagnostic logs, when in reality 8 GB of crash logs are hidden behind TCC / SIP permissions.
- **Cause:** Treating `os.ErrPermission` or `WalkDir` errors as empty directories or returning `(nil, nil)`.
- **Remedy:** Trap permission errors, populate `ProviderMetrics.Partial = true`, set `StatusNote: "permission denied (TCC / root required)"`, set `ReviewOnly = true` (`ReviewReasonSystemProtected`), and accumulate any readable bytes.

### Pitfall 3: Destroying Database State in `Application Support` (`MAC-03`, `D-07`)
- **Symptom:** Deleting an application's `Application Support` directory logs the user out, corrupts local databases (SQLite), or wipes offline project files.
- **Cause:** Treating `~/Library/Application Support/*` as a cache directory.
- **Remedy:** Never delete top-level folders in `Application Support`. Strictly filter children down to allowed regenerable leaves: `Cache`, `Caches`, `logs`, `Logs`, `Crashpad/completed`, plus the dedicated `Spotify/PersistentCache`.

### Pitfall 4: Process Guard Race Condition (Scan vs Clean) (`MAC-05`, `D-14`)
- **Symptom:** The user opens Spotify or Chrome while the DevClean scan report is on screen. The user then presses "Clean", causing DevClean to delete the active cache out from underneath the running application, resulting in crashes or data corruption.
- **Cause:** Checking process state only during `Scan()`.
- **Remedy:** Perform **pre-deletion revalidation** directly inside `Clean()` immediately before removing files. If the process is running or state is uncertain, skip deletion of that target.

### Pitfall 5: Symlink Traversal Escaping Cache Roots (`MAC-05`, `D-16`)
- **Symptom:** A malicious or broken application creates a symlink in `~/Library/Caches/app/cache` pointing to `~/.ssh` or `/System`. DevClean follows the link and deletes sensitive files.
- **Cause:** Using `filepath.Walk` instead of `filepath.WalkDir`, or following symlinks during cleanup without checking target paths.
- **Remedy:** Always inspect `d.Type()&os.ModeSymlink != 0`. Never recurse into symlinked directories. Evaluate symlink destinations with `filepath.EvalSymlinks` and verify they reside within the cache root and do not resolve to blacklisted system directories (`safety.IsCoreBlacklisted`).

---

## Code Examples

### 1. Inviolable Security Whitelist (`internal/cleaner/macos/whitelist.go`)

```go
package macos

import (
	"path/filepath"
	"strings"
)

// Immutable list of sensitive patterns and bundles that must NEVER be cleaned.
// Inviolable in Go code; cannot be disabled by flags or config (D-15).
var immutableSensitivePatterns = []string{
	// Apple Sync, Cloud & Identity
	"*cloudkit*",
	"*CloudKit*",
	"*mobile documents*",
	"*Mobile Documents*",
	"*MobileDocuments*",
	"*mobile.documents*",
	"com.apple.keychain*",
	"*keychain*",
	"*Keychain*",
	"com.apple.Safari*",
	"Safari",

	// Password Managers
	"com.1password.*",
	"com.agilebits.*",
	"com.bitwarden.*",
	"org.keepassxc.*",
	"com.lastpass.*",
	"com.dashlane.*",

	// Mesh & Corporate VPNs
	"*tailscale*",
	"io.tailscale.*",
	"*wireguard*",
	"com.wireguard.*",
	"*zerotier*",
	"com.zerotier.*",
	"*1dot1dot1dot1*", // Cloudflare WARP
	"*cloudflare*warp*",
}

// IsWhitelisted checks whether the path or bundle identifier matches any
// immutable protection pattern.
func IsWhitelisted(path string) bool {
	clean := filepath.Clean(path)
	base := filepath.Base(clean)
	lowerPath := strings.ToLower(clean)
	lowerBase := strings.ToLower(base)

	for _, pattern := range immutableSensitivePatterns {
		lowerPat := strings.ToLower(pattern)
		if matched, _ := filepath.Match(lowerPat, lowerBase); matched {
			return true
		}
		if matched, _ := filepath.Match(lowerPat, lowerPath); matched {
			return true
		}
		if strings.Contains(lowerPath, strings.Trim(lowerPat, "*")) {
			return true
		}
	}
	return false
}
```

### 2. Deterministic Fail-Closed Process Guard (`internal/cleaner/macos/process.go`)

```go
package macos

import (
	"context"
	"errors"
	"strings"

	"github.com/gtkpad/devclean/internal/cleaner"
)

// Known mapping from bundle IDs and daemon identifiers to executable process names.
var knownProcessMap = map[string][]string{
	"com.spotify.client":                     {"Spotify"},
	"com.apple.mediaanalysisd":               {"mediaanalysisd"},
	"com.apple.geod":                         {"geod"},
	"com.apple.helpd":                        {"helpd"},
	"com.apple.WallpaperAgent":               {"WallpaperAgent", "wallpaper"},
	"com.apple.parsecd":                      {"parsecd"},
	"com.apple.AppleMediaServices":           {"amsaccountsd", "appstoreagent"},
	"com.apple.coreservices.useractivityd":   {"useractivityd"},
	"com.google.Chrome.code_sign_clone":     {"Google Chrome"},
	"org.chromium.Chromium.code_sign_clone": {"Chromium"},
}

// checkProcessStatus returns running=true if pgrep finds matching processes.
// If pgrep exits with code 1, running=false, uncertain=false.
// Any other exit code or execution error returns uncertain=true (FAIL CLOSED!).
func checkProcessStatus(ctx context.Context, exec cleaner.CommandExecutor, processNames ...string) (running bool, uncertain bool) {
	if exec == nil {
		return false, true
	}
	for _, name := range processNames {
		out, err := exec.Output(ctx, "pgrep", "-x", name)
		if err == nil && len(strings.TrimSpace(string(out))) > 0 {
			return true, false
		}
		var exitErr interface{ ExitCode() int }
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			continue
		}
		// Unexpected error (pgrep missing, permission error, exit code > 1) -> FAIL CLOSED
		return false, true
	}
	return false, false
}
```

### 3. Pure Go Zero-CGO Ownership Verification (`internal/cleaner/macos/ownership_darwin.go`)

```go
//go:build darwin

package macos

import (
	"os"
	"syscall"
)

// isOwnedByCurrentUser verifies that the filesystem item is owned by the
// UID of the current executing process (D-16). Zero CGO.
func isOwnedByCurrentUser(info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat == nil {
		return true // Fallback if stat cannot be extracted
	}
	return stat.Uid == uint32(os.Getuid())
}
```

```go
//go:build !darwin

package macos

import (
	"os"
)

// Fallback for non-Darwin environments (used in portable tests).
func isOwnedByCurrentUser(info os.FileInfo) bool {
	return true
}
```

### 4. Trash Scan & Tolerant Direct Purging (`internal/cleaner/macos/trash.go`)

```go
package macos

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gtkpad/devclean/internal/domain"
	"github.com/gtkpad/devclean/internal/fs"
)

// scanTrash inventories $HOME/.Trash as ReviewOnly by default (D-01, D-02).
func scanTrash(homeDir string) domain.Target {
	trashDir := filepath.Join(homeDir, ".Trash")
	usage, _ := fs.CalcUsage(trashDir)

	return domain.Target{
		ID:            "macos-trash",
		Name:          "macOS Trash (~/.Trash)",
		Path:          trashDir,
		SizeBytes:     usage.AllocatedBytes,
		EligibleBytes: 0, // Always 0 on scan (D-01)
		TTLEligible:   false,
		ReviewOnly:    true,
		ReviewReason:  "trash_requires_flag",
		Risk:          domain.RiskLow,
		Category:      "macos-trash",
	}
}

// cleanTrash purges items inside $HOME/.Trash directly without internal quarantine (D-03, D-04).
func cleanTrash(trashDir string) error {
	entries, err := os.ReadDir(trashDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var remainingBytes int64
	var errs []error

	for _, entry := range entries {
		itemPath := filepath.Join(trashDir, entry.Name())

		// Attempt chmod u+w in case item was marked read-only (D-04)
		_ = fs.ChmodRecursive(itemPath, 0755, 0644)

		if err := os.RemoveAll(itemPath); err != nil {
			usage, _ := fs.CalcUsage(itemPath)
			remainingBytes += usage.AllocatedBytes
			errs = append(errs, fmt.Errorf("could not remove %s: %w", itemPath, err))
		}
	}

	// Never remove the root trashDir itself (D-03)
	if len(errs) > 0 {
		return fmt.Errorf("trash cleanup completed with %d remaining bytes: %v", remainingBytes, errs[0])
	}
	return nil
}
```

### 5. TCC Graceful Degradation in System Logs (`internal/cleaner/macos/system.go`)

```go
package macos

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/gtkpad/devclean/internal/domain"
)

// scanSystemLogs scans system logs degrading gracefully on TCC / permission denial (D-09, D-10).
func scanSystemLogs(logRoot string) domain.Target {
	var totalSize int64
	var partial bool
	var statusNote string

	err := filepath.WalkDir(logRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if errors.Is(walkErr, os.ErrPermission) || os.IsPermission(walkErr) {
				partial = true
				if statusNote == "" {
					statusNote = "permission denied (TCC / root required)"
				}
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() || d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if info, err := d.Info(); err == nil {
			totalSize += info.Size()
		}
		return nil
	})

	if err != nil && (errors.Is(err, os.ErrPermission) || os.IsPermission(err)) {
		partial = true
		statusNote = "permission denied (TCC / root required)"
	}

	metrics := &domain.ProviderMetrics{
		SizeBytes:  totalSize,
		Partial:    partial,
		StatusNote: statusNote,
	}

	return domain.Target{
		ID:              "macos-system-logs",
		Name:            "macOS System Logs (/private/var/log)",
		Path:            logRoot,
		SizeBytes:       totalSize,
		EligibleBytes:   0,
		TTLEligible:     false,
		ReviewOnly:      true,
		ReviewReason:    domain.ReviewReasonSystemProtected,
		Risk:            domain.RiskLow,
		Category:        "macos-system-logs",
		ProviderMetrics: metrics,
	}
}
```

---

## Validation Architecture

To satisfy Nyquist validation criteria (`VAL-01`, `VAL-02`) with >85% coverage across all new packages:

```
+---------------------------------------------------------------------------------------+
|                               Automated Test Matrix                                   |
+---------------------------------------------------------------------------------------+
|  1. Whitelist & Security Guard Tests (internal/cleaner/macos/whitelist_test.go)       |
|     - Verify immutable block on CloudKit, Mobile Documents, Keychain, Safari, VPNs,   |
|       1Password across variations (casing, glob patterns, path nesting).              |
|                                                                                       |
|  2. Process Guard & Pre-Delete Revalidation (internal/cleaner/macos/process_test.go)  |
|     - MockCommandExecutor: exit 0 (running) -> ReviewOnly (active_process).           |
|     - MockCommandExecutor: exit 1 (stopped) -> Eligible for removal.                  |
|     - MockCommandExecutor: exit 2 / error -> ReviewOnly (fail-closed, uncertain).     |
|     - Pre-delete revalidation in Clean(): aborts if process started post-scan.       |
|                                                                                       |
|  3. User Essentials & Deduplication Tests (internal/cleaner/macos/user_test.go)       |
|     - Excludes directories of dedicated cleaners (pnpm, uv, homebrew, cocoapods).     |
|     - Consolidates secondary bundles <=50MB; separates larger bundles (>50MB).        |
|                                                                                       |
|  4. Trash Discovery & Purge Tests (internal/cleaner/macos/trash_test.go)              |
|     - Scan reports ReviewOnly: true, EligibleBytes: 0, trash_requires_flag.           |
|     - Clean with authorization: purges items, unlocks 0444 files, preserves root.    |
|     - Ignores external volume paths (/Volumes/*/.Trashes).                            |
|                                                                                       |
|  5. Daemons & Group Containers Tests (internal/cleaner/macos/daemons_test.go)         |
|     - Positive allowlist covers 8 canonical daemons.                                  |
|     - Group Containers: scans only Library/Caches & Logs of allowlisted groups.      |
|     - Strictly skips *.dev.orbstack (handled by Docker cleaner).                      |
|                                                                                       |
|  6. Application Support Leaves Tests (internal/cleaner/macos/appsupport_test.go)      |
|     - Scans strictly Cache, Caches, logs, Logs, Crashpad/completed.                  |
|     - Dedicated target for Spotify/PersistentCache with active process check.         |
|     - Preserves sentinels in non-allowlisted folders (databases, plists).             |
|                                                                                       |
|  7. System Logs, DiagnosticReports & Clones (internal/cleaner/macos/system_test.go)   |
|     - Segregates User vs System DiagnosticReports.                                    |
|     - Simulated TCC/EACCES -> ProviderMetrics.Partial = true + StatusNote.           |
|     - Isolates *.code_sign_clone to user TMPDIR/C/.                                   |
|                                                                                       |
|  8. Symlink Guard & Ownership Checks (internal/cleaner/macos/ownership_test.go)       |
|     - Refuses symlinks escaping cache roots or pointing to /System or /Library.       |
|     - Validates st_uid == current_uid with zero CGO.                                  |
|                                                                                       |
|  9. Engine & CLI Integration Tests (cmd/clean_test.go, internal/engine/register_test) |
|     - macOS cleaner registered and responsive to rules.                               |
|     - Clean CLI with --trash enables trash item purging.                              |
|     - Zero-CGO build verification: CGO_ENABLED=0 go test ./...                        |
+---------------------------------------------------------------------------------------+
```

### Test Map for MAC-01 to MAC-05

| Requirement | Test File | Key Test Cases |
|---|---|---|
| **MAC-01** | `internal/cleaner/macos/user_test.go`, `trash_test.go` | `TestUserCaches_DeduplicationExclusions`, `TestUserCaches_ConsolidationThreshold`, `TestTrash_ScanAlwaysReviewOnly`, `TestTrash_CleanPurgesItemsPreservingRoot`, `TestTrash_ChmodRecoveryOnLockedFiles` |
| **MAC-02** | `internal/cleaner/macos/daemons_test.go`, `groups_test.go` | `TestDaemons_CanonicalAllowlist8Daemons`, `TestGroupContainers_AllowlistedCachesOnly`, `TestGroupContainers_ExcludesOrbStack` |
| **MAC-03** | `internal/cleaner/macos/appsupport_test.go` | `TestAppSupport_RestrictedLeavesOnly`, `TestAppSupport_SpotifyPersistentCacheDedicated`, `TestAppSupport_PreservesDatabases` |
| **MAC-04** | `internal/cleaner/macos/system_test.go` | `TestDiagnosticReports_UserEligible_SystemReviewOnly`, `TestSystemLogs_GracefulDegradationTCC`, `TestCodeSignClones_IsolatedToUserTmpdir` |
| **MAC-05** | `internal/cleaner/macos/whitelist_test.go`, `process_test.go`, `ownership_test.go` | `TestWhitelist_ImmutablePatterns`, `TestProcessGuard_FailClosed`, `TestProcessGuard_PreDeleteRevalidation`, `TestSymlinkGuard_RejectsEscapes`, `TestOwnership_ZeroCGO` |

### Commands

- **Quick run command:**
  ```bash
  go test -v -run "TestMacOSCleaner" ./internal/cleaner/macos/...
  ```
- **Full suite command:**
  ```bash
  CGO_ENABLED=0 go test ./...
  ```

### Wave 0 Gaps

- Package `internal/cleaner/macos/` does not yet exist.
- Configuration structs in `internal/config/config.go` and `defaults.go` lack `MacOS EcosystemConfig` and `MacOS HookConfig`.
- `internal/engine/register.go` lacks registration of `macos.NewWithExecutor(exec)`.
- `cmd/clean.go` needs coordination for `--trash` to toggle eligibility for the macOS Trash target.

---

## Security Domain

### Applicable ASVS Categories

- **ASVS V5: Validation, Sanitization and Encoding (V5.1.1, V5.1.2):** External commands (`pgrep -x`, `getconf`) are invoked with tokenized argument slices via `cleaner.CommandExecutor`. Zero execution through shell interpreters (`sh -c`, `bash -c`). Verified by `cleaner.AuditNoShellInjection`.
- **ASVS V12: File and Resources (V12.1.1, V12.1.2, V12.3.1, V12.3.2):** Strict protection against directory traversal and symlink hijacking. Symlinks are checked via `os.Lstat` and `filepath.EvalSymlinks`, refusing any link that escapes the cache boundary or points to protected system roots (`/System`, `/Library`, `/usr`, `/etc`, `/var`, `/private`). File ownership is verified before deletion.
- **ASVS V14: Configuration and Least Privilege (V14.1.1, V14.2.1):** DevClean runs strictly with the permissions of the calling user. Zero use of `sudo`, `su`, or GUI privilege escalation prompts (`D-10`). Unreadable system files degrade gracefully into review items.

### Threat Patterns & Mitigations

| Threat ID | Threat Description | Attack Vector / Trigger | Mitigation Strategy |
|---|---|---|---|
| **THREAT-01** | Arbitrary File Deletion via Symlink Escape | Application cache contains a symlink pointing to `~/.ssh` or `/etc`. | `safety.IsSymlink` and canonical evaluation: symlinks pointing outside the cache root or to blacklisted roots are rejected. |
| **THREAT-02** | Active Application Data Corruption | Cleaning cache files while an app or daemon is actively writing to them. | Process Guard with `pgrep -x` fail-closed policy, plus mandatory pre-deletion revalidation in `Clean()`. |
| **THREAT-03** | Accidental Deletion of Sensitive Vaults | Cleaning caches in directories holding CloudKit databases, Keychain items, or 1Password vaults. | Inviolable Go whitelist (`whitelist.go`) matching sensitive paths regardless of flags or configuration. |
| **THREAT-04** | Privilege Escalation Vulnerability | Invoking `sudo` or external helper with elevated privileges. | Explicit commitment to zero sudo (`D-10`). Any inaccessible system log is marked `ReviewOnly` with `ProviderMetrics.Partial = true`. |
| **THREAT-05** | Cross-User Cache Interference | Scanning `/private/var/folders` across other users on multi-user Mac. | Isolating `*.code_sign_clone` strictly to current user's `$TMPDIR` / `DARWIN_USER_CACHE_DIR` subfolder `C/` (`D-11`). |
| **THREAT-06** | Accidental Finder Trash Emptying | User runs `devclean clean --auto` and unexpectedly wipes files waiting in Trash. | Trash is always `ReviewOnly` by default (`D-01`), requiring explicit `--trash` authorization. |

---

## Sources

- DevClean v2.3 Specification: `.planning/SPEC-v2.3-MOLE-PARITY.md` §Frente 3
- Phase 11 Discussion & Context: `.planning/phases/11-macos-user-system-caches-logs/11-CONTEXT.md`
- Requirements: `.planning/REQUIREMENTS.md` (`MAC-01` to `MAC-05`)
- Mole Implementation Reference: `/opt/homebrew/Cellar/mole/1.60.0/libexec/lib/clean/` (`user.sh`, `caches.sh`, `system.sh`, `app_caches.sh`, `app_protection_data.sh`, `browser_clones.sh`)
- Existing Codebase Reference:
  - `internal/cleaner/ai/process.go` (Process Guard implementation pattern)
  - `internal/safety/blacklist.go` (System roots and immutable blacklists)
  - `internal/domain/types.go` (Target, ProviderMetrics, ReviewReasons)
  - `internal/fs/usage.go`, `usage_darwin.go` (Sparse block accounting, inode tracking)
  - `internal/fs/chmod.go` (Tolerant permission recovery)

---

## Metadata

- **Milestone:** v2.3 Mole Parity
- **Phase:** 11 — macOS User/System Caches & Logs
- **Author:** GSD Phase Researcher
- **Completion Date:** 2026-10-16
- **Status:** Complete & Ready for Planning

---

## RESEARCH COMPLETE

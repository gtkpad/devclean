# Phase 11: macOS User/System Caches & Logs - Pattern Mapping

**Phase:** 11-macos-user-system-caches-logs  
**Milestone:** v2.3 Functional Parity with Mole (tw93/mole)  
**Date:** 2026-10-16  
**Status:** Approved for Implementation  
**Files Analyzed:** 18  
**Analogs Found:** 18 / 18  

---

## 1. Overview & Pattern Mapping Summary

Este documento estabelece o mapeamento estrutural entre os arquivos a serem criados e modificados na **Fase 11** e os análogos consolidados na base de código do DevClean (`internal/cleaner/docker/docker.go`, `internal/cleaner/docker/orbstack.go`, `internal/cleaner/ai/process.go`, `internal/cleaner/ai/editors.go`, `internal/cleaner/xcode/xcode.go`, `internal/safety/blacklist.go`, `internal/safety/symlink.go`, `internal/fs/usage.go`, `internal/fs/usage_darwin.go`, `internal/fs/chmod.go`, `internal/domain/types.go`, `cmd/clean.go`, `internal/deleter/deleter.go`, `internal/deleter/policy_test.go`).

A Fase 11 introduz o ecossistema `macos` (`internal/cleaner/macos`), entregando paridade total com o utilitário Mole (`tw93/mole`) nas categorias **User essentials**, **App caches**, **Application Support** e **System**, preservando incondicionalmente os pilares de segurança, ausência de privilégios de root, tolerância a falhas de permissão e compilação estática zero-CGO (`CGO_ENABLED=0`).

A fase soluciona seis desafios arquiteturais fundamentais:
1. **User Essentials e Política Honesta da Lixeira (`MAC-01`, `D-01` a `D-04`):**
   Varre `~/Library/Caches/*`, `~/Library/Logs/*` e `$HOME/.Trash`. Para evitar deleções acidentais catastróficas, `~/.Trash` é **sempre** emitido no scan como `ReviewOnly: true`, `ReviewReason: "trash_requires_flag"`, `EligibleBytes: 0` e `TTLEligible: false`. Apenas quando a flag explícita `--trash` (ou config `safety.trash_enabled`) for fornecida na limpeza é que o alvo se torna elegível para purga direta. Na execução de `Clean()`, os itens dentro de `~/.Trash` são removidos permanentemente via `os.RemoveAll` com destravamento de permissões somente-leitura (`fs.ChmodRecursive`), **sem** movê-los para a quarentena reversível interna (`internal/trash`), e preservando a pasta raiz `$HOME/.Trash`. A varredura ignora deliberadamente volumes externos (`/Volumes/*/.Trashes`).
2. **Deduplicação Canônica e Consolidação Categórica (`MAC-01`, `MAC-03`, `SCAN-04`, `D-05`, `D-06`):**
   `~/Library/Caches` contém diretórios geridos por outros cleaners dedicados (`uv`, `pnpm`, `mise`, `ms-playwright`, `cypress`, `homebrew`, `cocoapods`, `xcode`, `orbstack`, editores AI/Electron). O cleaner macOS implementa uma tabela de exclusão canônica impedindo dupla contagem de bytes. Bundles secundários de usuário com tamanho <=50MB são agrupados em linhas consolidadas (`User App Caches` e `User App Logs`), enquanto bundles que excedem 50MB ou com regras especiais são desmembrados em alvos individuais.
3. **Positive Allowlists: Daemons Apple e Group Containers (`MAC-02`, `D-08`, `D-13`):**
   Para evitar a destruição de dados de sincronização ou bancos SQLite compartilhados, o cleaner aplica positive allowlist estrita para os 8 daemons canônicos da Apple (Siri, Wallpaper, `mediaanalysisd`, `geod`, `helpd`, AppleMediaServices, `parsecd`, `useractivityd`/Handoff). Em `~/Library/Group Containers`, varre exclusivamente subdiretórios `Library/Caches`, `Library/Logs`, `Caches` e `Logs` de grupos autorizados, excluindo estritamente `*.dev.orbstack` (pertencente ao cleaner Docker) e nunca tocando a raiz do contêiner.
4. **Application Support Cirúrgico e Cache Dedicado do Spotify (`MAC-03`, `D-07`):**
   `~/Library/Application Support` abriga dados de estado, configurações e bancos de dados insubstituíveis. O DevClean filtra estritamente subpastas de folhas regeneráveis: `Cache`, `Caches`, `logs`, `Logs` e `Crashpad/completed`. Adicionalmente, detecta o alvo dedicado de alto volume `Spotify PersistentCache` (`~/Library/Application Support/Spotify/PersistentCache`), protegido por Process Guard para `Spotify`.
5. **Logs de Sistema, DiagnosticReports e Code-Sign Clones com Zero Sudo (`MAC-04`, `D-09` a `D-12`):**
   Segrega relatórios de diagnóstico entre `User Diagnostic Reports` (elegíveis) e `System Diagnostic Reports` (`ReviewOnly`). Em `/private/var/log` e relatórios de sistema, degrada graciosamente diante de TCC ou falta de permissão via `ProviderMetrics.Partial = true` e `StatusNote: "permission denied (TCC / root required)"`, acumulando os bytes que puderam ser lidos. Estritamente zero uso de `sudo` ou elevação de privilégio. A busca por `*.code_sign_clone` do Chromium é isolada estritamente no subdiretório `C/` do usuário corrente em `TMPDIR` / `DARWIN_USER_CACHE_DIR`.
6. **Segurança Multi-Camada: Whitelist Imutável, Process Guard e Zero-CGO (`MAC-05`, `D-14` a `D-16`):**
   Whitelist imutável em Go (`whitelist.go`) bloqueia incondicionalmente vaults sensíveis (`CloudKit`, `Mobile Documents`, `keychain*`, `Safari` state, VPNs como `Tailscale`/`WireGuard`, gerenciadores como `1Password`/`Bitwarden`). Process Guard determinístico baseado em `pgrep -x` via `cleaner.CommandExecutor` opera com política fail-closed (se ativo ou incerto, marca `ReviewOnly = true` com `ReviewReasonActiveProcess`), com revalidação obrigatória pré-deleção em `Clean()`. O Symlink Guard recusa links para fora do cache ou raízes de sistema, e a verificação de propriedade (`st_uid == current_uid`) utiliza puramente `syscall.Stat_t` sem CGO.

---

## 2. File Classification Table

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `internal/cleaner/macos/macos.go` | service / cleaner orchestrator | coordena varredura, deduplicação, deleção segura e revalidação de processos | `internal/cleaner/docker/docker.go:19-195`<br>`internal/cleaner/xcode/xcode.go:21-65, 294-315` | exact |
| `internal/cleaner/macos/whitelist.go` | security / immutable rules | valida caminhos e bundles contra lista inegociável de proteção | `internal/safety/blacklist.go:8-65` | exact |
| `internal/cleaner/macos/process.go` | service / process guard | consulta `pgrep -x` via `cleaner.CommandExecutor` com semântica fail-closed | `internal/cleaner/ai/process.go:1-32`<br>`internal/cleaner/docker/orbstack.go:183-194` | exact |
| `internal/cleaner/macos/ownership_darwin.go` | platform / sys extractor (Darwin) | extrai `syscall.Stat_t.Uid` e valida contra `os.Getuid()` com zero CGO | `internal/fs/usage_darwin.go:1-18` | exact |
| `internal/cleaner/macos/ownership_other.go` | platform / sys extractor (portable) | fallback portátil para testes herméticos em Linux / Windows | `internal/fs/usage_other.go:1-18` | exact |
| `internal/cleaner/macos/user.go` | service / sub-scanner | varre `~/Library/Caches` e `~/Library/Logs`, aplica exclusões e consolidação | `internal/cleaner/ai/editors.go:100-240`<br>`internal/cleaner/xcode/xcode.go:70-130` | high |
| `internal/cleaner/macos/trash.go` | service / sub-scanner & purger | inventaria `$HOME/.Trash` como ReviewOnly; purga itens direto preservando a raiz | `internal/fs/chmod.go:1-52`<br>`internal/trash/trash.go:56-110` | high |
| `internal/cleaner/macos/daemons.go` | service / sub-scanner | positive allowlist para os 8 daemons Apple com checagem de processo | `internal/cleaner/docker/orbstack.go:30-116` | high |
| `internal/cleaner/macos/groups.go` | service / sub-scanner | varre `~/Library/Group Containers`, ignora OrbStack, restringe a Caches/Logs | `internal/cleaner/docker/orbstack.go:50-135` | high |
| `internal/cleaner/macos/appsupport.go` | service / sub-scanner | varre subpastas regeneráveis em Application Support e isola Spotify cache | `internal/cleaner/ai/editors.go:100-200` | high |
| `internal/cleaner/macos/system.go` | service / sub-scanner | varre logs de sistema, relatórios diagnósticos e clones de assinatura | `internal/fs/usage.go:50-100`<br>`internal/cleaner/docker/docker.go:95-125` | high |
| `internal/cleaner/macos/macos_test.go` | unit test | ciclo de vida completo do cleaner (Scan, Clean, DryRun) com mocks | `internal/cleaner/docker/docker_test.go:20-80` | exact |
| `internal/cleaner/macos/whitelist_test.go` | unit test | asserção de bloqueio imutável para CloudKit, Keychain, Safari, VPNs, Senhas | `internal/safety/blacklist_test.go:9-65` | exact |
| `internal/cleaner/macos/process_test.go` | unit test | asserção de fail-closed para `pgrep`, parada limpa e erro inesperado | `internal/cleaner/ai/process_test.go:9-65` | exact |
| `internal/cleaner/macos/user_test.go` | unit test | deduplicação contra ecossistemas dedicados e consolidação <=50MB | `internal/cleaner/ai/editors_test.go:20-75` | high |
| `internal/cleaner/macos/trash_test.go` | unit test | scan ReviewOnly, purga autorizada, chmod recovery em 0444, preservação da raiz | `internal/fs/chmod_test.go`<br>`internal/deleter/deleter_test.go` | high |
| `internal/cleaner/macos/daemons_test.go` | unit test | cobertura dos 8 daemons canônicos e detecção de processos ativos | `internal/cleaner/docker/orbstack_test.go:20-60` | high |
| `internal/cleaner/macos/groups_test.go` | unit test | restrição a subdiretórios de cache e exclusão estrita de `dev.orbstack` | `internal/cleaner/docker/orbstack_test.go:60-100` | high |
| `internal/cleaner/macos/appsupport_test.go` | unit test | preservação de bancos/preferências e extração do Spotify PersistentCache | `internal/cleaner/ai/editors_test.go:40-90` | high |
| `internal/cleaner/macos/system_test.go` | unit test | degradação graciosa em TCC, segregação User/System e code_sign_clones | `internal/fs/usage_test.go` | high |
| `internal/cleaner/macos/ownership_test.go` | unit test | validação de propriedade UID com zero-CGO | `internal/fs/usage_test.go` | high |
| `internal/config/config.go` & `defaults.go` | config model & defaults | registro de `MacOS EcosystemConfig` e `MacOS HookConfig` (`MaxAgeDays: 14`) | `internal/config/config.go:40-57, 77-97`<br>`internal/config/defaults.go:10-29, 40-59` | exact |
| `internal/engine/register.go` | registry / bootstrapper | injeção de dependência de `macos.NewWithExecutor(exec)` no motor | `internal/engine/register.go:43-66` | exact |
| `cmd/clean.go` & `cmd/clean_test.go` | CLI controller & test | coordenação de `--trash` para habilitar elegibilidade de purga da Lixeira | `cmd/clean.go:170-205` | exact |
| `internal/deleter/deleter.go` | deletion coordinator | roteamento de purga direta para `macos-trash` sem quarentena redundante | `internal/deleter/deleter.go:160-260` | exact |
| `internal/deleter/policy_test.go` | integration test | canary test de zero mutação na Lixeira e itens de sistema sob DryRun | `internal/deleter/policy_test.go:1287-1420` | exact |

---

## 3. Pattern Assignments & Concrete Code Excerpts

### 3.1. Inviolable Security Whitelist (`internal/cleaner/macos/whitelist.go`)

- **Role:** Camada de segurança imutável e inegociável em nível de código Go.
- **Data Flow:** Recebe caminhos de arquivos e bundle IDs durante `Scan()` e pré-deleção em `Clean()`. Se casar com qualquer padrão protegido, a operação é rejeitada incondicionalmente, imune a configurações do usuário ou flags CLI.
- **Closest Analog:** `internal/safety/blacklist.go:8-65`.
- **Decisions Enforced:** `D-15`, `MAC-05`, `THREAT-03`.

#### Existing Code Excerpt (`internal/safety/blacklist.go:8-48`)
```go
// coreBlacklist is immutable — cannot be disabled by config or flags.
var coreBlacklist = []string{".git", ".env", ".ssh", "secrets.json"}

// systemRoots protects OS and filesystem root directories.
var systemRoots = []string{
	"/", "/bin", "/boot", "/dev", "/etc", "/lib", "/proc", "/root", "/sys", "/usr", "/var",
	"/System", "/Library", "/Applications", "/private", "/private/etc", "/private/var",
}

func isCoreBlacklistedDirect(clean string) bool {
	for _, root := range systemRoots {
		if clean == filepath.Clean(root) {
			return true
		}
	}
	// ...
	return false
}
```

#### Concrete Adaptation Pattern for Phase 11 (`internal/cleaner/macos/whitelist.go`)
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

	// Password Managers & Vaults
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
// immutable protection pattern. Returns true if protected (must NOT be touched).
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

---

### 3.2. Deterministic Fail-Closed Process Guard (`internal/cleaner/macos/process.go`)

- **Role:** Guarda de execução em tempo real mapeando bundles e daemons a processos ativos.
- **Data Flow:** Executa `pgrep -x <name>` via `cleaner.CommandExecutor`. Se o processo estiver ativo (`running=true`) ou caso ocorra qualquer incerteza/erro inesperado (`uncertain=true`), adota postura fail-closed marcando o alvo como `ReviewOnly: true` (`ReviewReasonActiveProcess`). O guard é revalidado obrigatoriamente pré-deleção em `Clean()`.
- **Closest Analog:** `internal/cleaner/ai/process.go:1-32` e `internal/cleaner/docker/orbstack.go:183-194`.
- **Decisions Enforced:** `D-14`, `MAC-05`, `THREAT-02`.

#### Existing Code Excerpt (`internal/cleaner/ai/process.go:14-31`)
```go
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
		// Any unexpected error (missing tool, permission error, exit code != 1) -> uncertain -> FAIL CLOSED!
		return false, true
	}
	return false, false
}
```

#### Concrete Adaptation Pattern for Phase 11 (`internal/cleaner/macos/process.go`)
```go
package macos

import (
	"context"
	"errors"
	"strings"

	"github.com/gtkpad/devclean/internal/cleaner"
)

// Known mapping from bundle IDs and daemon identifiers to executable process names (D-14).
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

// CheckProcessStatus returns running=true if pgrep finds matching processes.
// If pgrep fails with exit code 1, running=false, uncertain=false (cleanly stopped).
// Any other exit code or execution error returns uncertain=true (FAIL CLOSED!).
func CheckProcessStatus(ctx context.Context, exec cleaner.CommandExecutor, processNames ...string) (running bool, uncertain bool) {
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

// IsBundleProcessRunning resolves mapped process names for a bundle/daemon ID and checks running status.
func IsBundleProcessRunning(ctx context.Context, exec cleaner.CommandExecutor, bundleOrDaemonID string) (running bool, uncertain bool) {
	names, ok := knownProcessMap[bundleOrDaemonID]
	if !ok {
		// If bundle has no explicit process mapping, try base name
		base := strings.TrimPrefix(bundleOrDaemonID, "com.apple.")
		base = strings.TrimPrefix(base, "com.")
		names = []string{base}
	}
	return CheckProcessStatus(ctx, exec, names...)
}
```

---

### 3.3. Pure Go Zero-CGO Ownership Verification (`internal/cleaner/macos/ownership_darwin.go`)

- **Role:** Verificação de permissões e integridade de arquivo de baixo nível.
- **Data Flow:** Executa no momento do `Scan()` e imediatamente antes da deleção de arquivos físicos em `Clean()`. Compara `syscall.Stat_t.Uid` com `os.Getuid()`. Se o arquivo pertencer a outro usuário ou ao root, a operação é rejeitada.
- **Closest Analog:** `internal/fs/usage_darwin.go:1-18`.
- **Decisions Enforced:** `D-16`, `MAC-05`, `THREAT-05`.

#### Existing Code Excerpt (`internal/fs/usage_darwin.go:1-17`)
```go
//go:build darwin

package fs

import (
	"os"
	"syscall"
)

func getFileInfoStat(info os.FileInfo) (dev uint64, ino uint64, blocks int64, apparent int64, ok bool) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat == nil {
		return 0, 0, 0, info.Size(), false
	}
	// On Darwin, Stat_t.Dev is int32; cast to uint32 first then uint64 to avoid sign extension
	return uint64(uint32(stat.Dev)), stat.Ino, stat.Blocks, stat.Size, true
}
```

#### Concrete Adaptation Pattern for Phase 11 (`internal/cleaner/macos/ownership_darwin.go` & `ownership_other.go`)
```go
//go:build darwin

package macos

import (
	"os"
	"syscall"
)

// isOwnedByCurrentUser verifies that the filesystem item is owned by the
// UID of the current executing process (D-16). Compiles cleanly with CGO_ENABLED=0.
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

// Fallback for non-Darwin environments (used in hermetic tests on Linux / CI).
func isOwnedByCurrentUser(info os.FileInfo) bool {
	return true
}
```

---

### 3.4. Trash Discovery & Direct Purging (`internal/cleaner/macos/trash.go`)

- **Role:** Sub-scanner e coordenador de expurgo seguro para a Lixeira nativa do macOS.
- **Data Flow:**
  - `Scan()`: Varre estritamente `$HOME/.Trash`. Emite o alvo `macos-trash` como `ReviewOnly: true`, `ReviewReason: "trash_requires_flag"`, `EligibleBytes: 0` e `TTLEligible: false`.
  - `Clean()`: Quando explicitamente autorizado via `--trash`, purga permanentemente os itens contidos em `$HOME/.Trash` com `os.RemoveAll`. Tenta recuperação com `fs.ChmodRecursive(item, 0755, 0644)` para destravar itens em modo 0444. **Nunca** move para a quarentena reversível interna (`internal/trash`), e **nunca** apaga a pasta raiz `~/.Trash`.
- **Closest Analog:** `internal/fs/chmod.go:1-52` e `internal/trash/trash.go:56-110`.
- **Decisions Enforced:** `D-01`, `D-02`, `D-03`, `D-04`, `MAC-01`, `THREAT-06`.

#### Existing Code Excerpt (`internal/fs/chmod.go:9-33`)
```go
func ChmodRecursive(root string, dirPerm, filePerm os.FileMode) error {
	info, err := os.Lstat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil
	}
	if !info.IsDir() {
		return os.Chmod(root, filePerm)
	}
	_ = os.Chmod(root, dirPerm)
	// ...
}
```

#### Concrete Adaptation Pattern for Phase 11 (`internal/cleaner/macos/trash.go`)
```go
package macos

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gtkpad/devclean/internal/domain"
	"github.com/gtkpad/devclean/internal/fs"
)

// scanTrash inventories $HOME/.Trash as ReviewOnly by default (D-01, D-02).
// Deliberately ignores external volumes /Volumes/*/.Trashes.
func scanTrash(homeDir string) domain.Target {
	trashDir := filepath.Join(homeDir, ".Trash")
	usage, err := fs.CalcUsage(trashDir)
	if err != nil && os.IsNotExist(err) {
		return domain.Target{}
	}

	return domain.Target{
		ID:            "macos-trash",
		Name:          "macOS Trash (~/.Trash)",
		Path:          trashDir,
		SizeBytes:     usage.AllocatedBytes,
		EligibleBytes: 0, // Invariant: Always 0 on scan (D-01)
		TTLEligible:   false,
		ReviewOnly:    true,
		ReviewReason:  "trash_requires_flag",
		Risk:          domain.RiskLow,
		Category:      "macos-trash",
	}
}

// cleanTrash purges items inside $HOME/.Trash directly without internal quarantine (D-03, D-04).
func cleanTrash(ctx context.Context, trashDir string) error {
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
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

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

---

### 3.5. User Essentials & Deduplication Engine (`internal/cleaner/macos/user.go`)

- **Role:** Scanner principal de `~/Library/Caches` e `~/Library/Logs`.
- **Data Flow:**
  - Aplica tabela de exclusão canônica (`D-05`) para diretórios gerenciados por outros ecossistemas.
  - Verifica lista de proteção imutável (`IsWhitelisted`).
  - Consulta Process Guard (`process.go`) para verificar se o aplicativo está rodando.
  - Bundles <= 50MB são agrupados em alvos consolidados (`User App Caches` e `User App Logs`).
  - Bundles > 50MB ou com estado específico são reportados como alvos individuais para maior transparência do usuário.
- **Closest Analog:** `internal/cleaner/ai/editors.go:100-240`.
- **Decisions Enforced:** `D-05`, `D-06`, `MAC-01`, `MAC-03`, `SCAN-04`.

#### Concrete Adaptation Pattern for Phase 11 (`internal/cleaner/macos/user.go`)
```go
package macos

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gtkpad/devclean/internal/cleaner"
	"github.com/gtkpad/devclean/internal/domain"
	"github.com/gtkpad/devclean/internal/fs"
	"github.com/gtkpad/devclean/internal/safety"
)

// Exclusion table for directories in ~/Library/Caches handled by dedicated cleaners (D-05, SCAN-04).
var dedicatedEcosystemCaches = map[string]string{
	"pnpm":          "pnpm",
	"uv":            "python",
	"pip":           "python",
	"Homebrew":      "homebrew",
	"CocoaPods":     "cocoapods",
	"vcpkg":         "cpp",
	"mise":          "shell",
	"gh":            "shell",
	"ms-playwright": "node",
	"Cypress":       "node",
	"node":          "node",
	"Yarn":          "node",
	"dev.orbstack.OrbStack": "docker",
}

// Prefix exclusions for AI/Electron editors handled by internal/cleaner/ai
var dedicatedAIEditorPrefixes = []string{
	"com.microsoft.VSCode",
	"Cursor",
	"Antigravity",
	"com.google.antigravity",
	"Windsurf",
	"com.exafunction.windsurf",
	"com.anthropic.claudefordesktop",
	"opencode",
	"dev.zed.Zed",
}

const largeBundleThresholdBytes = 50 * 1024 * 1024 // 50 MB threshold (D-06)

// isExcludedByDedicatedCleaner checks whether a cache folder is handled elsewhere.
func isExcludedByDedicatedCleaner(dirName string) bool {
	if _, ok := dedicatedEcosystemCaches[dirName]; ok {
		return true
	}
	for _, prefix := range dedicatedAIEditorPrefixes {
		if strings.HasPrefix(dirName, prefix) {
			return true
		}
	}
	return false
}

// scanUserCaches walks ~/Library/Caches consolidating bundles <= 50MB and breaking out larger ones.
func scanUserCaches(ctx context.Context, exec cleaner.CommandExecutor, homeDir string) ([]domain.Target, error) {
	cachesRoot := filepath.Join(homeDir, "Library", "Caches")
	entries, err := os.ReadDir(cachesRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var targets []domain.Target
	var consolidatedSize, consolidatedEligible int64
	var hasActiveConsolidated bool

	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		name := entry.Name()
		fullPath := filepath.Join(cachesRoot, name)

		// 1. Skip items owned by other cleaners (D-05)
		if isExcludedByDedicatedCleaner(name) {
			continue
		}

		// 2. Skip items matching immutable whitelist (D-15)
		if IsWhitelisted(fullPath) || IsWhitelisted(name) {
			continue
		}

		// 3. Symlink guard: reject symlinks escaping cache root (D-16)
		if entry.Type()&os.ModeSymlink != 0 {
			resolved, err := filepath.EvalSymlinks(fullPath)
			if err != nil || safety.IsCoreBlacklisted(resolved) || !strings.HasPrefix(resolved, cachesRoot) {
				targets = append(targets, domain.Target{
					ID:           "macos-cache-symlink-" + name,
					Name:         fmt.Sprintf("macOS Cache: %s (ambiguous symlink)", name),
					Path:         fullPath,
					ReviewOnly:   true,
					ReviewReason: domain.ReviewReasonAmbiguous,
					Risk:         domain.RiskMedium,
					Category:     "macos-app-caches",
				})
				continue
			}
		}

		usage, _ := fs.CalcUsage(fullPath)
		size := usage.AllocatedBytes

		// 4. Process guard check (D-14)
		running, uncertain := IsBundleProcessRunning(ctx, exec, name)
		isActive := running || uncertain

		if size > largeBundleThresholdBytes || isActive {
			// Break out into individual target (D-06)
			reviewOnly := isActive
			reason := ""
			eligible := size
			if isActive {
				reason = domain.ReviewReasonActiveProcess
				eligible = 0
			}

			targets = append(targets, domain.Target{
				ID:            "macos-cache-" + name,
				Name:          fmt.Sprintf("macOS App Cache: %s", name),
				Path:          fullPath,
				SizeBytes:     size,
				EligibleBytes: eligible,
				TTLEligible:   !reviewOnly,
				ReviewOnly:    reviewOnly,
				ReviewReason:  reason,
				Risk:          domain.RiskLow,
				Category:      "macos-app-caches",
			})
		} else {
			// Consolidate into category bucket (D-06)
			consolidatedSize += size
			if !isActive {
				consolidatedEligible += size
			} else {
				hasActiveConsolidated = true
			}
		}
	}

	if consolidatedSize > 0 {
		targets = append(targets, domain.Target{
			ID:            "macos-user-app-caches-consolidated",
			Name:          "macOS User App Caches (consolidated)",
			Path:          cachesRoot,
			SizeBytes:     consolidatedSize,
			EligibleBytes: consolidatedEligible,
			TTLEligible:   !hasActiveConsolidated,
			ReviewOnly:    false,
			Risk:          domain.RiskLow,
			Category:      "macos-app-caches",
		})
	}

	return targets, nil
}
```

---

### 3.6. Positive Allowlists: Canonical Daemons & Group Containers (`internal/cleaner/macos/daemons.go`, `groups.go`)

- **Role:** Sub-scanners com positive allowlist estrita para caches de sistema Apple e containers compartilhados.
- **Data Flow:**
  - `daemons.go`: Cobre exclusivamente os 8 daemons estipulados (`D-13`): Siri, Wallpaper, `mediaanalysisd`, `geod`, `helpd`, AppleMediaServices, `parsecd` e `useractivityd`/Handoff. Aplica Process Guard em cada daemon.
  - `groups.go`: Em `~/Library/Group Containers`, inspeciona apenas grupos autorizados e estritamente as subpastas `Library/Caches`, `Library/Logs`, `Caches` e `Logs` (`D-08`). Exclui sumariamente `*.dev.orbstack` e nunca apaga o contêiner raiz.
- **Closest Analog:** `internal/cleaner/docker/orbstack.go:30-135`.
- **Decisions Enforced:** `D-08`, `D-13`, `MAC-02`.

#### Concrete Adaptation Pattern for Phase 11 (`internal/cleaner/macos/daemons.go`)
```go
package macos

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gtkpad/devclean/internal/cleaner"
	"github.com/gtkpad/devclean/internal/domain"
	"github.com/gtkpad/devclean/internal/fs"
)

// The canonical 8 Apple daemons defined in D-13 and SPEC §Frente 3.
var canonicalDaemons = []struct {
	ID       string
	Name     string
	PathRel  string // Relative to ~/Library/Caches
	BundleID string
}{
	{"siri", "Siri Suggestions & TTS", "com.apple.siri.context", "com.apple.siri.context"},
	{"wallpaper", "macOS Wallpaper Agent", "com.apple.WallpaperAgent", "com.apple.WallpaperAgent"},
	{"mediaanalysisd", "Media Analysis Daemon", "com.apple.mediaanalysisd", "com.apple.mediaanalysisd"},
	{"geod", "Location Services (geod)", "com.apple.geod", "com.apple.geod"},
	{"helpd", "macOS Help Viewer", "com.apple.helpd", "com.apple.helpd"},
	{"ams", "Apple Media Services", "com.apple.AppleMediaServices", "com.apple.AppleMediaServices"},
	{"parsecd", "Core Parsec Search", "com.apple.parsecd", "com.apple.parsecd"},
	{"handoff", "Handoff & Universal Clipboard", "com.apple.coreservices.useractivityd", "com.apple.coreservices.useractivityd"},
}

// scanDaemons scans strictly the canonical 8 Apple daemons with process guard protection (D-13).
func scanDaemons(ctx context.Context, exec cleaner.CommandExecutor, homeDir string) ([]domain.Target, error) {
	cachesRoot := filepath.Join(homeDir, "Library", "Caches")
	var targets []domain.Target

	for _, d := range canonicalDaemons {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		fullPath := filepath.Join(cachesRoot, d.PathRel)
		info, err := os.Stat(fullPath)
		if err != nil {
			continue
		}

		usage, _ := fs.CalcUsage(fullPath)
		size := usage.AllocatedBytes
		if size == 0 {
			size = info.Size()
		}

		running, uncertain := IsBundleProcessRunning(ctx, exec, d.BundleID)
		isActive := running || uncertain

		reviewOnly := isActive
		reason := ""
		eligible := size
		if isActive {
			reviewOnly = true
			reason = domain.ReviewReasonActiveProcess
			eligible = 0
		}

		targets = append(targets, domain.Target{
			ID:            "macos-daemon-" + d.ID,
			Name:          fmt.Sprintf("macOS Daemon: %s", d.Name),
			Path:          fullPath,
			SizeBytes:     size,
			EligibleBytes: eligible,
			TTLEligible:   !reviewOnly,
			ReviewOnly:    reviewOnly,
			ReviewReason:  reason,
			Risk:          domain.RiskLow,
			Category:      "macos-daemon-caches",
		})
	}

	return targets, nil
}
```

---

### 3.7. Surgical Application Support & Dedicated Spotify Cache (`internal/cleaner/macos/appsupport.go`)

- **Role:** Sub-scanner para pastas regeneráveis em `~/Library/Application Support/*`.
- **Data Flow:**
  - Restringe-se estritamente às folhas: `Cache`, `Caches`, `logs`, `Logs`, `Crashpad/completed` (`D-07`).
  - Nunca deleta pastas raiz de aplicativos em `Application Support` (protegendo bancos SQLite e licenças).
  - Emite alvo individual de alto volume para `Spotify PersistentCache` (`~/Library/Application Support/Spotify/PersistentCache`) com Process Guard para o processo `Spotify`.
- **Closest Analog:** `internal/cleaner/ai/editors.go:100-200`.
- **Decisions Enforced:** `D-07`, `MAC-03`.

#### Concrete Adaptation Pattern for Phase 11 (`internal/cleaner/macos/appsupport.go`)
```go
package macos

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gtkpad/devclean/internal/cleaner"
	"github.com/gtkpad/devclean/internal/domain"
	"github.com/gtkpad/devclean/internal/fs"
)

// Strictly allowed leaf directories under ~/Library/Application Support (D-07).
var allowedAppSupportLeaves = map[string]bool{
	"cache":               true,
	"caches":              true,
	"logs":                true,
	"crashpad/completed":  true,
}

// scanApplicationSupport scans only safe regenerable leaves and isolates Spotify PersistentCache.
func scanApplicationSupport(ctx context.Context, exec cleaner.CommandExecutor, homeDir string) ([]domain.Target, error) {
	appSupportRoot := filepath.Join(homeDir, "Library", "Application Support")
	var targets []domain.Target

	// 1. Dedicated Spotify Target (D-07)
	spotifyCache := filepath.Join(appSupportRoot, "Spotify", "PersistentCache")
	if info, err := os.Stat(spotifyCache); err == nil && info.IsDir() {
		usage, _ := fs.CalcUsage(spotifyCache)
		running, uncertain := CheckProcessStatus(ctx, exec, "Spotify")
		isActive := running || uncertain

		reviewOnly := isActive
		reason := ""
		eligible := usage.AllocatedBytes
		if isActive {
			reason = domain.ReviewReasonActiveProcess
			eligible = 0
		}

		targets = append(targets, domain.Target{
			ID:            "macos-spotify-persistent-cache",
			Name:          "Spotify: PersistentCache",
			Path:          spotifyCache,
			SizeBytes:     usage.AllocatedBytes,
			EligibleBytes: eligible,
			TTLEligible:   !reviewOnly,
			ReviewOnly:    reviewOnly,
			ReviewReason:  reason,
			Risk:          domain.RiskLow,
			Category:      "macos-spotify-cache",
		})
	}

	// 2. Discover safe leaves across other apps
	entries, err := os.ReadDir(appSupportRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return targets, nil
		}
		return targets, err
	}

	var consolidatedSize, consolidatedEligible int64

	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		appName := entry.Name()
		if !entry.IsDir() || appName == "Spotify" {
			continue // Skip files or already handled Spotify
		}

		appDir := filepath.Join(appSupportRoot, appName)
		if IsWhitelisted(appDir) || IsWhitelisted(appName) {
			continue
		}

		for leaf := range allowedAppSupportLeaves {
			leafPath := filepath.Join(appDir, filepath.FromSlash(leaf))
			if info, err := os.Stat(leafPath); err == nil && info.IsDir() {
				usage, _ := fs.CalcUsage(leafPath)
				consolidatedSize += usage.AllocatedBytes
				consolidatedEligible += usage.AllocatedBytes
			}
		}
	}

	if consolidatedSize > 0 {
		targets = append(targets, domain.Target{
			ID:            "macos-appsupport-leaves-consolidated",
			Name:          "macOS Application Support Caches & Logs",
			Path:          appSupportRoot,
			SizeBytes:     consolidatedSize,
			EligibleBytes: consolidatedEligible,
			TTLEligible:   true,
			ReviewOnly:    false,
			Risk:          domain.RiskLow,
			Category:      "macos-appsupport-caches",
		})
	}

	return targets, nil
}
```

---

### 3.8. System Logs, Graceful TCC Degradation & Code-Sign Clones (`internal/cleaner/macos/system.go`)

- **Role:** Sub-scanner para logs de sistema, relatórios de diagnóstico e clones de assinatura Chromium.
- **Data Flow:**
  - Segrega `User Diagnostic Reports` (elegível) de `System Diagnostic Reports` (`ReviewOnly`) (`D-12`).
  - Varre `/private/var/log` (ReviewOnly). Diante de erro de permissão ou negação de TCC, degrada graciosamente populando `ProviderMetrics.Partial = true` e `StatusNote: "permission denied (TCC / root required)"`, acumulando bytes legíveis sem falhar silenciosamente (`D-09`).
  - Zero uso de `sudo` ou tentativa de elevação de privilégio (`D-10`).
  - Busca `*.code_sign_clone` restrita ao diretório de cache do usuário corrente em `$TMPDIR` / `DARWIN_USER_CACHE_DIR` subpasta `C/` (`D-11`).
- **Closest Analog:** `internal/fs/usage.go:50-100` e `internal/cleaner/docker/docker.go:95-125`.
- **Decisions Enforced:** `D-09`, `D-10`, `D-11`, `D-12`, `MAC-04`, `THREAT-04`.

#### Existing Code Excerpt (`internal/cleaner/docker/docker.go:104-121`)
```go
if hasMalformed {
	targets = append(targets, domain.Target{
		ID:            "docker-daemon-parse-error",
		Name:          "Docker daemon (parse warning)",
		Path:          "docker://daemon/status",
		SizeBytes:     0,
		EligibleBytes: 0,
		TTLEligible:   false,
		ReviewOnly:    true,
		ReviewReason:  domain.ReviewReasonAmbiguous,
		Risk:          domain.RiskLow,
		Category:      "docker-daemon",
		ProviderMetrics: &domain.ProviderMetrics{
			Partial:    true,
			StatusNote: "docker system df returned malformed or unrecognized JSON lines",
		},
	})
}
```

#### Concrete Adaptation Pattern for Phase 11 (`internal/cleaner/macos/system.go`)
```go
package macos

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/gtkpad/devclean/internal/cleaner"
	"github.com/gtkpad/devclean/internal/domain"
)

// scanSystemLogsAndReports scans diagnostic reports, system logs with TCC degradation,
// and isolated code sign clones (D-09, D-10, D-11, D-12). Zero sudo!
func scanSystemLogsAndReports(ctx context.Context, exec cleaner.CommandExecutor, homeDir string) ([]domain.Target, error) {
	var targets []domain.Target

	// 1. User Diagnostic Reports (Eligible) (D-12)
	userDiagDir := filepath.Join(homeDir, "Library", "Logs", "DiagnosticReports")
	if info, err := os.Stat(userDiagDir); err == nil && info.IsDir() {
		usage, partial, note := scanDirectoryWithTCCFallback(ctx, userDiagDir)
		targets = append(targets, domain.Target{
			ID:            "macos-user-diagnostic-reports",
			Name:          "macOS User Diagnostic Reports",
			Path:          userDiagDir,
			SizeBytes:     usage,
			EligibleBytes: usage,
			TTLEligible:   true,
			ReviewOnly:    false,
			Risk:          domain.RiskLow,
			Category:      "macos-diagnostic-reports",
			ProviderMetrics: &domain.ProviderMetrics{
				SizeBytes:  usage,
				Partial:    partial,
				StatusNote: note,
			},
		})
	}

	// 2. System Diagnostic Reports (ReviewOnly) (D-12, D-09)
	sysDiagDir := "/Library/Logs/DiagnosticReports"
	if info, err := os.Stat(sysDiagDir); err == nil && info.IsDir() {
		usage, partial, note := scanDirectoryWithTCCFallback(ctx, sysDiagDir)
		targets = append(targets, domain.Target{
			ID:            "macos-system-diagnostic-reports",
			Name:          "macOS System Diagnostic Reports",
			Path:          sysDiagDir,
			SizeBytes:     usage,
			EligibleBytes: 0,
			TTLEligible:   false,
			ReviewOnly:    true,
			ReviewReason:  domain.ReviewReasonSystemProtected,
			Risk:          domain.RiskLow,
			Category:      "macos-diagnostic-reports",
			ProviderMetrics: &domain.ProviderMetrics{
				SizeBytes:  usage,
				Partial:    partial,
				StatusNote: note,
			},
		})
	}

	// 3. System Logs (/private/var/log) (ReviewOnly with TCC graceful degradation) (D-09, D-10)
	varLogDir := "/private/var/log"
	if info, err := os.Stat(varLogDir); err == nil && info.IsDir() {
		usage, partial, note := scanDirectoryWithTCCFallback(ctx, varLogDir)
		targets = append(targets, domain.Target{
			ID:            "macos-system-logs",
			Name:          "macOS System Logs (/private/var/log)",
			Path:          varLogDir,
			SizeBytes:     usage,
			EligibleBytes: 0,
			TTLEligible:   false,
			ReviewOnly:    true,
			ReviewReason:  domain.ReviewReasonSystemProtected,
			Risk:          domain.RiskLow,
			Category:      "macos-system-logs",
			ProviderMetrics: &domain.ProviderMetrics{
				SizeBytes:  usage,
				Partial:    partial,
				StatusNote: note,
			},
		})
	}

	// 4. Code Sign Clones in Darwin User Cache Dir (D-11)
	if cloneTarget, ok := scanCodeSignClones(ctx, exec); ok {
		targets = append(targets, cloneTarget)
	}

	return targets, nil
}

// scanDirectoryWithTCCFallback walks a directory trapping permission errors (D-09).
func scanDirectoryWithTCCFallback(ctx context.Context, root string) (totalSize int64, partial bool, note string) {
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if walkErr != nil {
			if errors.Is(walkErr, os.ErrPermission) || os.IsPermission(walkErr) {
				partial = true
				if note == "" {
					note = "permission denied (TCC / root required)"
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
		note = "permission denied (TCC / root required)"
	}
	return totalSize, partial, note
}

// scanCodeSignClones isolates search to current user's Darwin cache directory (D-11).
func scanCodeSignClones(ctx context.Context, exec cleaner.CommandExecutor) (domain.Target, bool) {
	userTmp := os.Getenv("TMPDIR")
	if userTmp == "" {
		return domain.Target{}, false
	}

	// Darwin user cache dir is typically sibling or parent folder /C/
	// (e.g. /var/folders/xx/yyyy/T/ -> /var/folders/xx/yyyy/C/)
	cleanTmp := filepath.Clean(userTmp)
	parent := filepath.Dir(cleanTmp)
	cDir := filepath.Join(parent, "C")

	if info, err := os.Stat(cDir); err != nil || !info.IsDir() {
		return domain.Target{}, false
	}

	var cloneSize int64
	var cloneCount int

	_ = filepath.WalkDir(cDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if strings.HasSuffix(d.Name(), ".code_sign_clone") {
			if info, err := d.Info(); err == nil {
				cloneSize += info.Size()
				cloneCount++
			}
		}
		return nil
	})

	if cloneCount == 0 {
		return domain.Target{}, false
	}

	// Process guard: Chrome or Chromium running?
	running, uncertain := CheckProcessStatus(ctx, exec, "Google Chrome", "Chromium")
	isActive := running || uncertain

	reviewOnly := isActive
	reason := ""
	eligible := cloneSize
	if isActive {
		reviewOnly = true
		reason = domain.ReviewReasonActiveProcess
		eligible = 0
	}

	return domain.Target{
		ID:            "macos-code-sign-clones",
		Name:          "macOS Code-Signature Clones",
		Path:          cDir,
		SizeBytes:     cloneSize,
		EligibleBytes: eligible,
		TTLEligible:   !reviewOnly,
		ReviewOnly:    reviewOnly,
		ReviewReason:  reason,
		Risk:          domain.RiskLow,
		Category:      "macos-system-clones",
	}, true
}
```

---

### 3.9. Orchestration, Discovery & Pre-Delete Revalidation (`internal/cleaner/macos/macos.go`)

- **Role:** Implementação canônica da interface `domain.Cleaner` para o ecossistema `macos`.
- **Data Flow:**
  - Instanciação via `NewWithExecutor(exec)`.
  - `IsInstalled()`: Retorna `true` se `runtime.GOOS == "darwin"` ou caso diretórios de cache existam em disco.
  - `Scan()`: Agrega resultados dos submódulos (`user.go`, `trash.go`, `daemons.go`, `groups.go`, `appsupport.go`, `system.go`).
  - `Clean()`: Aplica **dupla camada defensiva**:
    1. Recusa qualquer alvo `ReviewOnly` ou com categoria protegida.
    2. Para alvos de aplicativo e daemons, executa **revalidação pré-deleção**: se o processo foi iniciado pelo usuário após o scan, aborta a deleção.
    3. Para `macos-trash`, delega a `cleanTrash()` sem quarentena intermediária.
    4. Verifica Symlink Guard e `isOwnedByCurrentUser`.
- **Closest Analog:** `internal/cleaner/docker/docker.go:19-195` e `internal/cleaner/xcode/xcode.go:21-65, 294-315`.
- **Decisions Enforced:** `D-03`, `D-14`, `D-16`, `MAC-05`.

#### Concrete Adaptation Pattern for Phase 11 (`internal/cleaner/macos/macos.go`)
```go
package macos

import (
	"context"
	"fmt"
	"os"
	"runtime"

	"github.com/gtkpad/devclean/internal/cleaner"
	"github.com/gtkpad/devclean/internal/domain"
	"github.com/gtkpad/devclean/internal/safety"
)

type macosCleaner struct {
	exec    cleaner.CommandExecutor
	homeDir string
	goos    string
}

// New creates a macOS cleaner with default command executor and user home directory.
func New() domain.Cleaner {
	return NewWithExecutor(&cleaner.DefaultExecutor{})
}

// NewWithExecutor creates a macOS cleaner with custom executor (for hermetic testing).
func NewWithExecutor(exec cleaner.CommandExecutor) domain.Cleaner {
	home, _ := os.UserHomeDir()
	return &macosCleaner{
		exec:    exec,
		homeDir: home,
		goos:    runtime.GOOS,
	}
}

// NewWithDirAndExecutor allows explicit home directory injection for tests.
func NewWithDirAndExecutor(homeDir string, exec cleaner.CommandExecutor) domain.Cleaner {
	return &macosCleaner{
		exec:    exec,
		homeDir: homeDir,
		goos:    runtime.GOOS,
	}
}

func (c *macosCleaner) Name() string {
	return "macos"
}

func (c *macosCleaner) IsInstalled() bool {
	return c.goos == "darwin" || c.homeDir != ""
}

func (c *macosCleaner) Scan(ctx context.Context, rules domain.EcosystemRules) ([]domain.Target, error) {
	if !rules.Enabled {
		return nil, nil
	}

	var allTargets []domain.Target

	// 1. User Essentials: Caches and Logs (D-05, D-06)
	if userTargets, err := scanUserCaches(ctx, c.exec, c.homeDir); err == nil {
		allTargets = append(allTargets, userTargets...)
	}

	// 2. User Essentials: Trash (~/.Trash) (D-01, D-02)
	trashTarget := scanTrash(c.homeDir)
	if trashTarget.ID != "" {
		allTargets = append(allTargets, trashTarget)
	}

	// 3. Apple Daemons: 8 canonical daemons (D-13)
	if daemonTargets, err := scanDaemons(ctx, c.exec, c.homeDir); err == nil {
		allTargets = append(allTargets, daemonTargets...)
	}

	// 4. Group Containers: positive allowlist (D-08)
	if groupTargets, err := scanGroupContainers(ctx, c.homeDir); err == nil {
		allTargets = append(allTargets, groupTargets...)
	}

	// 5. Application Support Leaves & Spotify (D-07)
	if appSupportTargets, err := scanApplicationSupport(ctx, c.exec, c.homeDir); err == nil {
		allTargets = append(allTargets, appSupportTargets...)
	}

	// 6. System Logs, Diagnostic Reports & Code Sign Clones (D-09..D-12)
	if sysTargets, err := scanSystemLogsAndReports(ctx, c.exec, c.homeDir); err == nil {
		allTargets = append(allTargets, sysTargets...)
	}

	return allTargets, nil
}

func (c *macosCleaner) Clean(ctx context.Context, targets []domain.Target) error {
	for _, t := range targets {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Stage 1 guard: reject ReviewOnly targets
		if t.ReviewOnly {
			continue
		}

		// Dedicated Trash purger (D-03)
		if t.Category == "macos-trash" {
			if err := cleanTrash(ctx, t.Path); err != nil {
				return err
			}
			continue
		}

		// Pre-deletion safety verification: Symlink Guard & Ownership (D-16)
		info, err := os.Lstat(t.Path)
		if err != nil {
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 {
			continue // Do not follow or remove symlinked root
		}
		if !isOwnedByCurrentUser(info) {
			continue // Refuse to delete items owned by other users
		}
		if safety.IsCoreBlacklisted(t.Path) || IsWhitelisted(t.Path) {
			continue
		}

		// Pre-deletion Process Guard revalidation (D-14)
		running, uncertain := IsBundleProcessRunning(ctx, c.exec, t.ID)
		if running || uncertain {
			continue // Abort deletion if process started post-scan!
		}

		if err := os.RemoveAll(t.Path); err != nil {
			return fmt.Errorf("cleaning %s: %w", t.Path, err)
		}
	}
	return nil
}
```

---

### 3.10. CLI & Engine Integration: Flag Coordination & Registry (`cmd/clean.go`, `internal/deleter/deleter.go`, `internal/engine/register.go`, `internal/config`)

- **Role:** Integração com os subsistemas centrais do DevClean.
- **Data Flow:**
  - `cmd/clean.go`: Quando `--trash` é fornecido na linha de comando (ou configurado), ativa a elegibilidade do alvo `macos-trash` (ajustando `ReviewOnly = false` e `EligibleBytes = t.SizeBytes`).
  - `internal/deleter/deleter.go`: Em vez de enviar o caminho `~/.Trash` para `c.trashMgr.Move()` (o que tentaria mover a Lixeira para dentro da quarentena reversível do DevClean, violando `D-03`), roteia alvos com `Category == "macos-trash"` diretamente para `cleaner.Clean()`.
  - `internal/engine/register.go`: Registra `macos.NewWithExecutor(exec)`.
  - `internal/config/config.go` e `defaults.go`: Adicionam `MacOS` com `MaxAgeDays: 14`.
- **Closest Analog:** `cmd/clean.go:170-205` e `internal/deleter/deleter.go:160-260`.
- **Decisions Enforced:** `D-01`, `D-03`, `MAC-01`.

#### Concrete Adaptation Pattern for Phase 11 (`internal/deleter/deleter.go:160-255`)
```go
// Stage 2: Partitioning Filesystem vs Virtual vs Direct Cleaner Purge (D-03)
var fsTargets []domain.Target
var directTargets []domain.Target
var virtualTargets []domain.Target

for _, t := range safeTargets {
	if t.IsVirtual() {
		virtualTargets = append(virtualTargets, t)
	} else if t.Category == "macos-trash" {
		// D-03: Trash purges directly without internal quarantine!
		directTargets = append(directTargets, t)
	} else {
		fsTargets = append(fsTargets, t)
	}
}
```

#### Concrete Adaptation Pattern for Engine Registration (`internal/engine/register.go`)
```go
// RegisterDefaultCleanersWithExecutor
eng.Register(macos.NewWithExecutor(exec))
```

---

### 3.11. Safety Canary & Zero-Mutation Policy Test (`internal/deleter/policy_test.go`)

- **Role:** Teste de regressão e garantia inegociável de zero mutação (`POL-04`, `VAL-02`).
- **Data Flow:** Cria canários para arquivos de sistema, `~/.Trash`, logs inacessíveis e vaults protegidos (`1Password`, `Keychain`). Executa `coord.Delete` sob `DryRun: true` e `Auto: true`, verificando via SHA-256, tamanho e mtime que **absolutamente nenhum byte** foi alterado.
- **Closest Analog:** `internal/deleter/policy_test.go:1287-1420` (`TestCoordinator_DryRun_ZeroMutation_Phase10`).
- **Decisions Enforced:** `D-01`, `D-03`, `D-15`, `POL-04`, `VAL-02`.

#### Concrete Adaptation Pattern for Phase 11 (`internal/deleter/policy_test.go`)
```go
func TestCoordinator_DryRun_ZeroMutation_Phase11(t *testing.T) {
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, "config")
	dataDir := filepath.Join(tempDir, "data")
	trashDir := filepath.Join(tempDir, "trash")

	cfg := &config.Config{
		Safety: config.SafetyConfig{
			TrashEnabled: true,
			TrashDir:     trashDir,
		},
	}

	bl := safety.NewBlacklist(nil)
	trashMgr := trash.New(cfg.Safety)

	cleaners := newFakeCleanerProvider()
	macosCleaner := &fakeCleaner{name: "macos", installed: true}
	cleaners.Register("macos", macosCleaner)

	coord := deleter.New(cleaners, cfg, bl, trashMgr, nil, configDir, dataDir)

	type fileState struct {
		absPath string
		hash    [32]byte
		size    int64
		mtime   time.Time
	}

	var preStates []fileState

	recordFile := func(relPath string, payload []byte) string {
		fullPath := filepath.Join(tempDir, relPath)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatalf("mkdir failed: %v", err)
		}
		if err := os.WriteFile(fullPath, payload, 0o644); err != nil {
			t.Fatalf("write failed: %v", err)
		}
		info, err := os.Stat(fullPath)
		if err != nil {
			t.Fatalf("stat failed: %v", err)
		}
		preStates = append(preStates, fileState{
			absPath: fullPath,
			hash:    sha256.Sum256(payload),
			size:    info.Size(),
			mtime:   info.ModTime(),
		})
		return fullPath
	}

	makePayload := func(prefix string, totalSize int) []byte {
		b := make([]byte, totalSize)
		copy(b, []byte(prefix))
		return b
	}

	// 1. Canary files in Trash, System Logs, Whitelisted Vaults
	trashFile := recordFile("user_trash/item.txt", makePayload("TRASH-CANARY", 1024))
	sysLogFile := recordFile("system_logs/install.log", makePayload("SYSLOG-CANARY", 2048))
	keychainFile := recordFile("keychain_vault/login.keychain-db", makePayload("KEYCHAIN-CANARY", 4096))

	trashTarget := domain.Target{
		ID:            "macos-trash",
		Name:          "macOS Trash",
		Path:          filepath.Dir(trashFile),
		Category:      "macos-trash",
		ReviewOnly:    true,
		ReviewReason:  "trash_requires_flag",
		SizeBytes:     1024,
		EligibleBytes: 0,
	}

	sysLogTarget := domain.Target{
		ID:            "macos-system-logs",
		Name:          "macOS System Logs",
		Path:          filepath.Dir(sysLogFile),
		Category:      "macos-system-logs",
		ReviewOnly:    true,
		ReviewReason:  domain.ReviewReasonSystemProtected,
		SizeBytes:     2048,
		EligibleBytes: 0,
	}

	targetsByEco := map[string][]domain.Target{
		"macos": {trashTarget, sysLogTarget},
	}

	opts := deleter.Options{
		DryRun: true,
		Auto:   true,
		Mode:   "auto",
	}

	res, err := coord.Delete(context.Background(), targetsByEco, opts)
	if err != nil {
		t.Fatalf("DryRun Delete failed: %v", err)
	}

	// ReviewOnly targets must be skipped
	if len(res.SkippedReviewOnly) != 2 {
		t.Errorf("expected 2 skipped review targets, got %d", len(res.SkippedReviewOnly))
	}

	// Zero mutation assertion on every canary
	for _, pre := range preStates {
		postInfo, err := os.Stat(pre.absPath)
		if err != nil {
			t.Fatalf("canary file missing after DryRun: %s (err: %v)", pre.absPath, err)
		}
		data, err := os.ReadFile(pre.absPath)
		if err != nil {
			t.Fatalf("reading canary file %s: %v", pre.absPath, err)
		}
		if sha256.Sum256(data) != pre.hash {
			t.Errorf("SHA-256 mutated for %s!", pre.absPath)
		}
		if postInfo.Size() != pre.size {
			t.Errorf("Byte size mutated for %s!", pre.absPath)
		}
		if postInfo.ModTime() != pre.mtime {
			t.Errorf("mtime mutated for %s!", pre.absPath)
		}
	}
}
```

---

## 4. Verification & Testing Strategy

Para cumprir os critérios do protocolo de validação Nyquist (`VAL-01`, `VAL-02`) com cobertura superior a 85%:

1. **Testes Unitários de Whitelist (`internal/cleaner/macos/whitelist_test.go`):**
   - Testa variações de maiúsculas/minúsculas para `CloudKit`, `Mobile Documents`, `Safari`, `Keychain`, `Tailscale`, `WireGuard`, `1Password`.
   - Verifica que caminhos legítimos de cache (ex: `com.spotify.client`, `com.apple.mediaanalysisd`) não são bloqueados.
2. **Testes Unitários de Process Guard (`internal/cleaner/macos/process_test.go`):**
   - `MockCommandExecutor` com saída exit 0 -> `running=true`, `uncertain=false`.
   - `MockCommandExecutor` com exit 1 -> `running=false`, `uncertain=false`.
   - `MockCommandExecutor` com erro genérico ou exit != 1 -> `running=false`, `uncertain=true` (fail-closed).
3. **Testes de Deduplicação e Consolidação (`internal/cleaner/macos/user_test.go`):**
   - Cria árvore temporária com diretórios de `pnpm`, `uv`, `mise` e valida que são omitidos de `User App Caches`.
   - Valida que bundles <= 50MB são agrupados no total consolidado e bundles > 50MB tornam-se alvos próprios.
4. **Testes de Lixeira (`internal/cleaner/macos/trash_test.go`):**
   - `Scan()` sempre retorna `ReviewOnly: true`, `EligibleBytes: 0`, `trash_requires_flag`.
   - `Clean()` autorizado esvazia itens com `chmod u+w` em arquivos somente-leitura (0444) e preserva a raiz `~/.Trash`.
5. **Testes de Daemons e Group Containers (`internal/cleaner/macos/daemons_test.go`, `groups_test.go`):**
   - Positive allowlist cobre estritamente os 8 daemons.
   - Group Containers ignora `*.dev.orbstack` e restringe-se a subpastas `Library/Caches` e `Library/Logs`.
6. **Testes de Application Support (`internal/cleaner/macos/appsupport_test.go`):**
   - Preserva bancos de dados (ex: `SQLite.db`) e configurações.
   - Detecta e isola `Spotify PersistentCache`.
7. **Testes de Logs de Sistema e TCC (`internal/cleaner/macos/system_test.go`):**
   - Simulação de erro de permissão em `WalkDir` -> `ProviderMetrics.Partial = true` e `StatusNote: "permission denied (TCC / root required)"`.
   - Segregação de `User Diagnostic Reports` (elegível) vs `System Diagnostic Reports` (ReviewOnly).
8. **Testes de Symlink Guard e Propriedade (`internal/cleaner/macos/ownership_test.go`):**
   - Rejeição de symlinks apontando para fora do diretório de cache.
   - Validação de compilação pura sem CGO via `CGO_ENABLED=0 go test ./internal/cleaner/macos/...`.

---

## 5. Security & Invariants Checklist

- [x] **Zero Sudo / Sem Elevação de Privilégios (`D-10`, `ASVS V14`):** DevClean nunca invoca `sudo` ou solicita permissões administrativas interativas.
- [x] **Sem Injeção de Shell (`ASVS V5`):** Todos os comandos (`pgrep -x`) são tokenizados via `cleaner.CommandExecutor`.
- [x] **Preservação da Raiz da Lixeira (`D-03`):** Somente o conteúdo de `$HOME/.Trash` é purgado; a pasta raiz permanece intacta.
- [x] **Zero Quarentena Redundante para Lixeira (`D-03`):** Itens da Lixeira são purgados diretamente com `os.RemoveAll`, sem passá-los para `internal/trash`.
- [x] **Sem Dupla Contagem de Bytes (`D-05`, `SCAN-04`):** Exclusão rígida de caches de ecossistemas especializados (`pnpm`, `uv`, `mise`, `Homebrew`, etc.).
- [x] **Positive Allowlist para Daemons e Group Containers (`D-08`, `D-13`):** Nenhuma varredura aberta em contêineres de terceiros.
- [x] **Whitelist Imutável Inviolável (`D-15`, `MAC-05`):** `CloudKit`, `Keychain`, `Safari`, `1Password` e VPNs estão protegidos no nível da compilação.
- [x] **Process Guard Fail-Closed com Revalidação Pré-Clean (`D-14`):** Bloqueio imediato caso o aplicativo esteja ativo ou incerto, com rechecagem pré-mutação.
- [x] **Symlink Guard e Zero-CGO (`D-16`):** Rejeição de travessia para raízes de sistema e validação pura de UID com `syscall.Stat_t`.

---

## 6. Sources & References

- `.planning/phases/11-macos-user-system-caches-logs/11-CONTEXT.md` (User decisions D-01 a D-16)
- `.planning/phases/11-macos-user-system-caches-logs/11-RESEARCH.md` (Technical research and architecture)
- `.planning/SPEC-v2.3-MOLE-PARITY.md` §Frente 3 (macOS cleaner specifications)
- `internal/cleaner/docker/docker.go` & `orbstack.go` (CommandExecutor & Process Guard patterns)
- `internal/cleaner/ai/process.go` (Fail-closed process verification)
- `internal/safety/blacklist.go` (Immutable security lists)
- `internal/fs/usage.go` & `usage_darwin.go` (Zero-CGO Darwin stat parsing)
- `internal/fs/chmod.go` (Permission recovery pattern)
- `internal/deleter/policy_test.go` (DryRun canary validation test suite)

---

## PATTERN MAPPING COMPLETE

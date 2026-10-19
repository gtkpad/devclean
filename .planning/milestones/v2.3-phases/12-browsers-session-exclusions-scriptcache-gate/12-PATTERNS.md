# Phase 12: Browsers, Session Exclusions & ScriptCache Gate - Pattern Mapping

**Phase:** 12-browsers-session-exclusions-scriptcache-gate  
**Milestone:** v2.3 Functional Parity with Mole (tw93/mole)  
**Date:** 2026-10-17  
**Status:** Approved for Implementation  
**Files Analyzed:** 14 (9 new, 5 modified)  
**Analogs Found:** 14 / 14 (100% coverage)  

---

## 1. Overview & Pattern Mapping Summary

Este documento estabelece o mapeamento estrutural detalhado entre os arquivos a serem criados e modificados na **Fase 12** e os análogos consolidados na base de código do DevClean (`internal/cleaner/ai/editors.go`, `internal/cleaner/ai/ai.go`, `internal/cleaner/ai/process.go`, `internal/cleaner/macos/process.go`, `internal/cleaner/macos/whitelist.go`, `internal/cleaner/macos/user.go`, `internal/cleaner/macos/macos.go`, `internal/cleaner/macos/macos_test.go`, `internal/cleaner/ai/ai_test.go`, `internal/safety/blacklist.go`, `internal/cleaner/executor_mock.go`, `internal/engine/register.go`, `internal/config/config.go`, `internal/config/defaults.go`).

A Fase 12 implementa o novo ecossistema `browsers` (`internal/cleaner/browsers`), entregando paridade funcional com utilitários modernos de limpeza (notadamente o Mole `tw93/mole`), cobrindo 6 navegadores principais (**Google Chrome**, **Brave**, **Arc**, **Microsoft Edge**, **Mozilla Firefox** e **Apple Safari**) no macOS e Linux, enquanto impõe garantias inegociáveis de preservação de sessões, isolamento de processos e controle sobre o ScriptCache.

### Os Desafios Centrais da Fase 12

1. **O Dilemma Central da Limpeza de Navegadores (`BROW-03`, `D-13`, `D-15`):**
   Navegadores web acumulam gigabytes em caches regeneráveis: caches HTTP de rede (`Cache_Data`), caches de shaders de GPU (`GPUCache`, `DawnCache`), bytecode compilado de scripts (`Code Cache`) e cache offline de service workers (`CacheStorage`). No entanto, esses caches residem lado a lado com os dados mais sensíveis e insubstituíveis do usuário: sessões ativas (`Cookies`), histórico (`History`), senhas salvas (`Login Data`), bancos locais de websites (`IndexedDB`, `Local Storage`), e abas abertas (`Sessions`, `sessionstore`). A deleção acidental desses arquivos deslogaria o usuário de todos os seus serviços e destruiria o estado do navegador. A Fase 12 soluciona isso com uma **dupla camada de blindagem inviolável**:
   - *Positive Allowlist Estrita:* O cleaner nunca varre a raiz dos perfis recursivamente. Endereça unicamente folhas allowlisted comprovadamente regeneráveis.
   - *Invariant Assertion Guard:* Todo e qualquer caminho candidato é validado contra padrões imutáveis de sessão (`Cookies*`, `History*`, `places.sqlite*`, `key4.db*`, `sessionstore*`, etc.) e o diretório privado `~/Library/Safari`, sendo imediatamente abortado caso coincida com qualquer sentinela.
2. **Descoberta Multi-Browser e Multi-Perfil (`BROW-01`, `BROW-02`, `D-01` a `D-04`):**
   Suporte completo a Chrome, Brave, Arc, Edge, Firefox e Safari via detecção em disco (`SCAN-02`), resolvendo caminhos canônicos no macOS (`~/Library/Application Support/...` e `~/Library/Caches/...`) e Linux (`~/.config/...`, `~/.cache/...`, `~/.mozilla/...`), com Arc e Safari estritamente restritos a Darwin. Traversal dinâmico de todos os perfis de usuário (`Default`, `Profile 1`, `Profile 2`, `System Profile` e Firefox `*.default*`), agregando os volumes em um alvo consolidado principal por navegador (`Google Chrome Caches`, `Firefox Caches`, etc.) com endereçamento virtual (`browser://<id>/caches`).
3. **Gating do ScriptCache (`BROW-04`, `D-05` a `D-08`):**
   `Service Worker/ScriptCache` armazena bytecode compilado de workers offline e de extensões Manifest V3 (MV3). No v2.3, o ScriptCache dos navegadores Chromium é segregado em alvos dedicados de revisão (`Chrome ScriptCache`), marcados estritamente como `ReviewOnly: true`, `ReviewReason: "scriptcache_requires_evidence"`, `EligibleBytes: 0` e `TTLEligible: false`. O método `Clean()` impõe uma guarda ativa de recusa contra qualquer mutação do ScriptCache. Firefox e Safari não emitem alvo de ScriptCache.
4. **Process Guard Determinístico com Revalidação Pré-Clean Fail-Closed (`D-09` a `D-12`):**
   Processos ativos de navegadores retêm locks em disco (SQLite e LevelDB). O cleaner verifica o status do binário principal e de helpers ativos (`Google Chrome Helper`, `firefox-bin`, etc.) via `cleaner.CommandExecutor` (`pgrep -x`). O bloqueio é isolado por navegador (Chrome ativo não bloqueia Firefox fechado). O método `Clean()` revalida atomicamente os processos imediatamente antes de qualquer deleção física, abortando fail-closed se o navegador tiver sido aberto após o scan. Falhas ou incertezas no `pgrep` resultam em `uncertain = true` e falham fechado (`ReviewOnly: true`, `EligibleBytes: 0`).
5. **Degradação Graciosa em TCC e Zero Sudo (`D-14`):**
   Erros de permissão em caches do Safari (`~/Library/Caches/com.apple.Safari`) ou contêineres de usuário degradam graciosamente preenchendo `ProviderMetrics.Partial = true`, `StatusNote: "permission denied (TCC / Full Disk Access required)"`, marcando `ReviewOnly: true` (`ReviewReasonSystemProtected`) e mantendo zero uso de `sudo` ou elevação de privilégios.
6. **Deduplicação Canônica com o Cleaner macOS (`SCAN-04`):**
   As pastas dos 6 navegadores em `~/Library/Caches` (`Google`, `BraveSoftware`, `company.thebrowser.Arc`, `Microsoft Edge`, `Firefox`, `com.apple.Safari`) são registradas formalmente na tabela de exclusão `dedicatedEcosystemCaches` de `internal/cleaner/macos/user.go`, garantindo zero contagem duplicada.

---

## 2. File Classification Table

| File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `internal/cleaner/browsers/types.go` | domain types / contracts | define estruturas `BrowserDef`, `BrowserEngine`, métricas de subvolume e perfis | `internal/cleaner/ai/editors.go:18-28`<br>`internal/domain/types.go:12-45` | exact |
| `internal/cleaner/browsers/definitions.go` | declarative configuration | tabela estática dos 6 navegadores (caminhos Darwin/Linux, processos, helpers, ScriptCache flag) | `internal/cleaner/ai/editors.go:51-225`<br>`internal/cleaner/macos/daemons.go:12-70` | exact |
| `internal/cleaner/browsers/session_guard.go` | security / invariant guard | valida caminhos contra asserções de sessão (`Cookies*`, `History*`, `places.sqlite*`, etc.) e Safari shield | `internal/cleaner/macos/whitelist.go:10-94`<br>`internal/safety/blacklist.go:8-65` | exact |
| `internal/cleaner/browsers/process.go` | service / process guard | consulta `pgrep -x` para binários principais e helpers com semântica fail-closed | `internal/cleaner/ai/process.go:1-32`<br>`internal/cleaner/macos/process.go:25-45` | exact |
| `internal/cleaner/browsers/chromium.go` | service / sub-scanner & cleaner | traversal de múltiplos perfis, agregação de caches allowlisted e segregação do ScriptCache | `internal/cleaner/ai/editors.go:227-293`<br>`internal/cleaner/ai/ai.go:214-250` | high |
| `internal/cleaner/browsers/firefox.go` | service / sub-scanner & cleaner | traversal de perfis Gecko (`*.default*`), agregação de caches regeneráveis e blindagem de bancos SQLite | `internal/cleaner/ai/editors.go:227-293`<br>`internal/cleaner/macos/user.go:60-168` | high |
| `internal/cleaner/browsers/safari.go` | service / sub-scanner & cleaner (Darwin) | varredura estrita de `~/Library/Caches/com.apple.Safari`, isolamento de `~/Library/Safari` e TCC handling | `internal/cleaner/macos/system.go:20-65`<br>`internal/cleaner/macos/whitelist.go:20-25` | exact |
| `internal/cleaner/browsers/browsers.go` | service / cleaner orchestrator | implementa `domain.Cleaner`, orquestra Scan, roteia URIs virtuais, revalida processos pré-Clean e aplica recusa defensiva | `internal/cleaner/ai/ai.go:16-58, 284-330`<br>`internal/cleaner/macos/macos.go:16-53, 100-148` | exact |
| `internal/cleaner/browsers/browsers_test.go` | unit test / hermetic test suite | testes herméticos com `t.TempDir()`, `MockCommandExecutor`, double sentinels (cache vs session), revalidação e zero-CGO | `internal/cleaner/ai/ai_test.go:616-850`<br>`internal/cleaner/macos/macos_test.go:77-176` | exact |
| `internal/cleaner/macos/user.go` | exclusion table update | exclui diretórios dos navegadores em `~/Library/Caches` da varredura genérica do macOS | `internal/cleaner/macos/user.go:15-30` | exact |
| `internal/config/config.go` & `defaults.go` | configuration model | registra `Browsers EcosystemConfig` e `Browsers HookConfig` com padrão `MaxAgeDays: 14` | `internal/config/config.go:57, 98`<br>`internal/config/defaults.go:29, 60` | exact |
| `internal/engine/register.go` | engine bootstrapper | registra `browsers.NewWithExecutor(exec)` em `RegisterDefaultCleanersWithExecutor` | `internal/engine/register.go:53, 69` | exact |
| `internal/engine/engine.go` | engine config routing | mapeia `"browsers"` para `e.config.Ecosystems.Browsers` no método `ecosystemConfig` | `internal/engine/engine.go:198-200` | exact |
| `cmd/clean.go` | CLI clean command | mapeia `"browsers"` para `cfg.Ecosystems.Browsers.MaxAgeDays` em `getEcosystemMaxAgeDays` | `cmd/clean.go:429-430` | exact |

---

## 3. Pattern Assignments & Concrete Code Excerpts

### 3.1. Browser Types & Contracts (`internal/cleaner/browsers/types.go`)

- **Role:** Definições de tipos, contratos de motor e estruturas de dados de perfil e métricas para navegadores.
- **Data Flow:** Utilizado transversalmente por `definitions.go`, `chromium.go`, `firefox.go`, `safari.go` e `browsers.go`.
- **Closest Analog:** `internal/cleaner/ai/editors.go:18-28` (`editorDef`) e `internal/domain/types.go:12-45`.
- **Decisions Enforced:** `D-01`, `D-02`, `D-04`, `D-08`.

#### Existing Code Excerpt (`internal/cleaner/ai/editors.go:18-28`)
```go
type editorDef struct {
	id            string
	name          string
	processNames  []string
	hasExtensions bool
	hasWorkspaces bool
	appSupportDir func(home string) string
	cacheDirs     func(home string) []string
	logDirs       func(home string) []string
	extRoots      func(home string) []string
}
```

#### Concrete Adaptation Pattern (`internal/cleaner/browsers/types.go`)
```go
package browsers

// BrowserEngine categorizes rendering engines for specialized cache structures.
type BrowserEngine string

const (
	EngineChromium BrowserEngine = "chromium"
	EngineGecko    BrowserEngine = "gecko"
	EngineWebKit   BrowserEngine = "webkit"
)

// BrowserDef represents declarative identification and traversal paths for a browser.
type BrowserDef struct {
	ID             string
	Name           string
	Engine         BrowserEngine
	DarwinOnly     bool
	HasScriptCache bool
	ProcessNames   map[string][]string // "darwin" -> [...], "linux" -> [...]
	AppSupportDir  func(home, goos string) string
	CacheDir       func(home, goos string) string
}

// ProfileInfo holds resolved paths for an individual user profile.
type ProfileInfo struct {
	Name           string
	AppSupportPath string
	CachePath      string
}

// SubVolumeMetrics aggregates byte sizes across cache sub-categories.
type SubVolumeMetrics struct {
	NetworkBytes      int64
	GPUBytes          int64
	CodeCacheBytes    int64
	CacheStorageBytes int64
	ScriptCacheBytes  int64
}
```

---

### 3.2. Declarative Browser Definitions (`internal/cleaner/browsers/definitions.go`)

- **Role:** Tabela declarativa e imutável de metadados dos 6 navegadores suportados.
- **Data Flow:** Fornece resoluções canônicas de diretórios e nomes de executáveis para `Scan()`, `Clean()` e `IsInstalled()`.
- **Closest Analog:** `internal/cleaner/ai/editors.go:51-225` e `internal/cleaner/macos/daemons.go:12-70`.
- **Decisions Enforced:** `BROW-01`, `D-01`, `D-04`, `D-08`, `D-11`.

#### Existing Code Excerpt (`internal/cleaner/ai/editors.go:51-74`)
```go
func getEditorDefinitions() []editorDef {
	return []editorDef{
		{
			id:            "code",
			name:          "VS Code",
			processNames:  []string{"Code", "code"},
			hasExtensions: true,
			hasWorkspaces: true,
			appSupportDir: func(home string) string {
				return resolveAppSupportDir(home, "Code", "Code")
			},
			cacheDirs: func(home string) []string {
				return []string{
					filepath.Join(home, "Library", "Caches", "com.microsoft.VSCode"),
					filepath.Join(home, "Library", "Caches", "com.microsoft.VSCode.ShipIt"),
					filepath.Join(home, ".cache", "Code"),
				}
			},
			// ...
		},
```

#### Concrete Adaptation Pattern (`internal/cleaner/browsers/definitions.go`)
```go
package browsers

import (
	"path/filepath"
)

// GetBrowserDefinitions returns the canonical definitions for all 6 supported browsers.
func GetBrowserDefinitions() []BrowserDef {
	return []BrowserDef{
		{
			ID:             "chrome",
			Name:           "Google Chrome",
			Engine:         EngineChromium,
			DarwinOnly:     false,
			HasScriptCache: true,
			ProcessNames: map[string][]string{
				"darwin": {"Google Chrome", "Google Chrome Helper"},
				"linux":  {"chrome", "google-chrome", "google-chrome-stable"},
			},
			AppSupportDir: func(home, goos string) string {
				if goos == "darwin" {
					return filepath.Join(home, "Library", "Application Support", "Google", "Chrome")
				}
				return filepath.Join(home, ".config", "google-chrome")
			},
			CacheDir: func(home, goos string) string {
				if goos == "darwin" {
					return filepath.Join(home, "Library", "Caches", "Google", "Chrome")
				}
				return filepath.Join(home, ".cache", "google-chrome")
			},
		},
		{
			ID:             "brave",
			Name:           "Brave",
			Engine:         EngineChromium,
			DarwinOnly:     false,
			HasScriptCache: true,
			ProcessNames: map[string][]string{
				"darwin": {"Brave Browser", "Brave Browser Helper"},
				"linux":  {"brave", "brave-browser"},
			},
			AppSupportDir: func(home, goos string) string {
				if goos == "darwin" {
					return filepath.Join(home, "Library", "Application Support", "BraveSoftware", "Brave-Browser")
				}
				return filepath.Join(home, ".config", "BraveSoftware", "Brave-Browser")
			},
			CacheDir: func(home, goos string) string {
				if goos == "darwin" {
					return filepath.Join(home, "Library", "Caches", "BraveSoftware", "Brave-Browser")
				}
				return filepath.Join(home, ".cache", "BraveSoftware", "Brave-Browser")
			},
		},
		{
			ID:             "arc",
			Name:           "Arc",
			Engine:         EngineChromium,
			DarwinOnly:     true,
			HasScriptCache: true,
			ProcessNames: map[string][]string{
				"darwin": {"Arc", "Arc Helper"},
			},
			AppSupportDir: func(home, goos string) string {
				if goos != "darwin" {
					return ""
				}
				return filepath.Join(home, "Library", "Application Support", "Arc", "User Data")
			},
			CacheDir: func(home, goos string) string {
				if goos != "darwin" {
					return ""
				}
				return filepath.Join(home, "Library", "Caches", "company.thebrowser.Arc")
			},
		},
		{
			ID:             "edge",
			Name:           "Microsoft Edge",
			Engine:         EngineChromium,
			DarwinOnly:     false,
			HasScriptCache: true,
			ProcessNames: map[string][]string{
				"darwin": {"Microsoft Edge", "Microsoft Edge Helper"},
				"linux":  {"msedge", "microsoft-edge"},
			},
			AppSupportDir: func(home, goos string) string {
				if goos == "darwin" {
					return filepath.Join(home, "Library", "Application Support", "Microsoft Edge")
				}
				return filepath.Join(home, ".config", "microsoft-edge")
			},
			CacheDir: func(home, goos string) string {
				if goos == "darwin" {
					return filepath.Join(home, "Library", "Caches", "Microsoft Edge")
				}
				return filepath.Join(home, ".cache", "microsoft-edge")
			},
		},
		{
			ID:             "firefox",
			Name:           "Mozilla Firefox",
			Engine:         EngineGecko,
			DarwinOnly:     false,
			HasScriptCache: false,
			ProcessNames: map[string][]string{
				"darwin": {"firefox", "firefox-bin"},
				"linux":  {"firefox", "firefox-bin"},
			},
			AppSupportDir: func(home, goos string) string {
				if goos == "darwin" {
					return filepath.Join(home, "Library", "Application Support", "Firefox", "Profiles")
				}
				return filepath.Join(home, ".mozilla", "firefox")
			},
			CacheDir: func(home, goos string) string {
				if goos == "darwin" {
					return filepath.Join(home, "Library", "Caches", "Firefox", "Profiles")
				}
				return filepath.Join(home, ".cache", "mozilla", "firefox")
			},
		},
		{
			ID:             "safari",
			Name:           "Safari",
			Engine:         EngineWebKit,
			DarwinOnly:     true,
			HasScriptCache: false,
			ProcessNames: map[string][]string{
				"darwin": {"Safari", "SafariTechnologyPreview"},
			},
			AppSupportDir: func(home, goos string) string {
				return "" // Inviolable: Strictly NO AppSupport scanning for Safari! (D-13)
			},
			CacheDir: func(home, goos string) string {
				if goos != "darwin" {
					return ""
				}
				return filepath.Join(home, "Library", "Caches", "com.apple.Safari")
			},
		},
	}
}

// FindBrowserDef searches for a BrowserDef by ID.
func FindBrowserDef(id string) (BrowserDef, bool) {
	for _, bDef := range GetBrowserDefinitions() {
		if bDef.ID == id {
			return bDef, true
		}
	}
	return BrowserDef{}, false
}
```

---

### 3.3. Inviolable Session Shield & Positive Allowlist (`internal/cleaner/browsers/session_guard.go`)

- **Role:** Camada de segurança inegociável que combina Positive Allowlist para folhas regeneráveis e Invariant Assertion Guard negativo.
- **Data Flow:** Chamada antes de qualquer medição em `Scan()` e antes de qualquer deleção em `Clean()`. Se qualquer caminho casar com padrões de sessão ou pertencer a `~/Library/Safari`, é imediatamente rejeitado.
- **Closest Analog:** `internal/cleaner/macos/whitelist.go:10-94` e `internal/safety/blacklist.go:8-65`.
- **Decisions Enforced:** `BROW-03`, `D-13`, `D-15`.

#### Existing Code Excerpt (`internal/cleaner/macos/whitelist.go:50-93`)
```go
func IsWhitelisted(path string) bool {
	clean := filepath.Clean(path)
	base := filepath.Base(clean)
	lowerPath := strings.ToLower(clean)
	lowerBase := strings.ToLower(base)

	components := strings.FieldsFunc(lowerPath, func(r rune) bool {
		return r == '/' || r == '\\'
	})
	if len(components) == 0 {
		components = []string{lowerBase}
	}

	for _, pattern := range immutableSensitivePatterns {
		lowerPat := strings.ToLower(pattern)
		if matched, _ := filepath.Match(lowerPat, lowerBase); matched {
			return true
		}
		if matched, _ := filepath.Match(lowerPat, lowerPath); matched {
			return true
		}
		// ...
	}
	return false
}
```

#### Concrete Adaptation Pattern (`internal/cleaner/browsers/session_guard.go`)
```go
package browsers

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gtkpad/devclean/internal/safety"
)

// Immutable session tokens and database patterns that must NEVER be measured or deleted.
var protectedSessionTokens = []string{
	"cookies",
	"cookies-journal",
	"history",
	"history-journal",
	"login data",
	"login data-journal",
	"web data",
	"web data-journal",
	"local storage",
	"indexeddb",
	"sessions",
	"session storage",
	"current session",
	"current tabs",
	"last session",
	"last tabs",
	"bookmarks",
	"preferences",
	"secure preferences",
	"places.sqlite",
	"places.sqlite-wal",
	"key4.db",
	"logins.json",
	"sessionstore",
	"favicons.sqlite",
	"formhistory.sqlite",
	"lastsession.plist",
	"tabs",
	"cloudtabs",
}

// Allowlisted regenerable leaf directories for Chromium profiles.
var allowlistedChromiumLeaves = map[string]bool{
	"GPUCache":                                true,
	"DawnCache":                               true,
	"DawnWebGPUCache":                         true,
	"DawnGraphiteCache":                       true,
	"Code Cache":                              true,
	filepath.Join("Service Worker", "CacheStorage"): true,
	"Cache_Data":                              true,
	"Media Cache":                             true,
	"data_0":                                  true,
	"data_1":                                  true,
	"data_2":                                  true,
	"data_3":                                  true,
	"index":                                   true,
}

// Allowlisted regenerable leaf directories for Firefox profiles.
var allowlistedFirefoxLeaves = map[string]bool{
	"cache2":         true,
	"startupCache":   true,
	"jumpListCache":  true,
	"thumbnails":     true,
	"activity-stream": true,
}

// Allowlisted regenerable leaf directories and files for Safari.
var allowlistedSafariLeaves = map[string]bool{
	"WebKitCache":   true,
	"Cache.db":      true,
	"Cache.db-wal":  true,
	"Cache.db-shm":  true,
	"fsCachedData":  true,
}

// IsSessionOrProtectedData returns true if the candidate path matches any personal,
// authentication, browsing history or session database pattern.
func IsSessionOrProtectedData(path string) bool {
	clean := filepath.Clean(path)
	if safety.IsCoreBlacklisted(clean) {
		return true
	}

	lower := strings.ToLower(clean)

	// Inviolable Safari private state shield (D-13): ~/Library/Safari is NEVER touched!
	if strings.Contains(lower, filepath.Join("library", "safari")) {
		return true
	}

	base := strings.ToLower(filepath.Base(clean))
	for _, tok := range protectedSessionTokens {
		if base == tok || strings.HasPrefix(base, tok) {
			return true
		}
	}

	// Granular component inspection
	parts := strings.FieldsFunc(lower, func(r rune) bool {
		return r == '/' || r == '\\'
	})
	for _, part := range parts {
		for _, tok := range protectedSessionTokens {
			if part == tok || strings.HasPrefix(part, tok) {
				return true
			}
		}
	}

	return false
}

// VerifyCleanCandidate validates that a candidate path is safe to delete.
func VerifyCleanCandidate(path string) error {
	if IsSessionOrProtectedData(path) {
		return fmt.Errorf("security violation: path %s matched session invariant guard", path)
	}
	if safety.IsCoreBlacklisted(path) {
		return fmt.Errorf("security violation: path %s is core blacklisted", path)
	}
	return nil
}
```

---

### 3.4. Fail-Closed Per-Browser Process Guard (`internal/cleaner/browsers/process.go`)

- **Role:** Verificação individual de processos principais e helpers de cada navegador via `cleaner.CommandExecutor` (`pgrep -x`).
- **Data Flow:** Executado durante `Scan()` (para definir `ReviewOnly`) e revalidado atomicamente no início de `Clean()` imediatamente pré-deleção.
- **Closest Analog:** `internal/cleaner/ai/process.go:1-32` e `internal/cleaner/macos/process.go:25-45`.
- **Decisions Enforced:** `D-09`, `D-10`, `D-11`, `D-12`.

#### Existing Code Excerpt (`internal/cleaner/ai/process.go:11-31`)
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

#### Concrete Adaptation Pattern (`internal/cleaner/browsers/process.go`)
```go
package browsers

import (
	"context"
	"errors"
	"strings"

	"github.com/gtkpad/devclean/internal/cleaner"
)

// CheckBrowserProcessStatus verifies whether any main or helper process of a browser is running.
// Returns:
//   - running=true, uncertain=false: At least one process was found running (exit code 0).
//   - running=false, uncertain=false: All configured processes are cleanly stopped (exit code 1).
//   - running=false, uncertain=true: pgrep failed unexpectedly (exit code > 1, command missing, timeout) -> FAIL CLOSED!
func CheckBrowserProcessStatus(ctx context.Context, exec cleaner.CommandExecutor, bDef BrowserDef, goos string) (running bool, uncertain bool) {
	if exec == nil {
		return false, true
	}
	names := bDef.ProcessNames[goos]
	if len(names) == 0 {
		return false, false
	}

	for _, name := range names {
		out, err := exec.Output(ctx, "pgrep", "-x", name)
		if err == nil && len(strings.TrimSpace(string(out))) > 0 {
			return true, false
		}
		var exitErr interface{ ExitCode() int }
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			continue // Cleanly stopped for this process
		}
		// Unexpected error: missing binary, exit code != 1, permission denial -> FAIL CLOSED!
		return false, true
	}
	return false, false
}
```

---

### 3.5. Chromium Multi-Profile Traversal & ScriptCache Gating (`internal/cleaner/browsers/chromium.go`)

- **Role:** Varredura dinâmica de múltiplos perfis Chromium (Chrome, Brave, Arc, Edge), consolidação de caches e segregação de ScriptCache.
- **Data Flow:** Descobre perfis em `AppSupportDir` e `CacheDir`, calcula métricas, emite alvos `browser://<id>/caches` e `browser://<id>/scriptcache`, e executa a limpeza cirúrgica.
- **Closest Analog:** `internal/cleaner/ai/editors.go:227-293` e `internal/cleaner/ai/ai.go:214-250`.
- **Decisions Enforced:** `BROW-01`, `BROW-02`, `BROW-04`, `D-01`, `D-02`, `D-03`, `D-05`, `D-06`, `D-07`.

#### Existing Code Excerpt (`internal/cleaner/ai/editors.go:234-260`)
```go
	var candidateDirs []string
	if ed.appSupportDir != nil {
		appSupport := ed.appSupportDir(home)
		if appSupport != "" {
			subLeaves := []string{
				"Cache",
				"Code Cache",
				"GPUCache",
				"DawnCache",
				"DawnWebGPUCache",
				"DawnGraphiteCache",
				filepath.Join("Service Worker", "CacheStorage"),
				filepath.Join("Service Worker", "ScriptCache"),
				"CachedData",
				"webviewCache",
			}
			for _, sub := range subLeaves {
				candidateDirs = append(candidateDirs, filepath.Join(appSupport, sub))
			}
		}
	}
```

#### Concrete Adaptation Pattern (`internal/cleaner/browsers/chromium.go`)
```go
package browsers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gtkpad/devclean/internal/domain"
	devfs "github.com/gtkpad/devclean/internal/fs"
	"github.com/gtkpad/devclean/internal/safety"
)

// discoverChromiumProfiles scans the Application Support / Config root for profile directories.
func discoverChromiumProfiles(appSupportRoot, cacheRoot string) []ProfileInfo {
	var profiles []ProfileInfo
	entries, err := os.ReadDir(appSupportRoot)
	if err != nil {
		return profiles
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		// Profile heuristic: Default, System Profile, Guest Profile, or Profile *
		if name == "Default" || name == "System Profile" || name == "Guest Profile" || strings.HasPrefix(name, "Profile ") {
			profiles = append(profiles, ProfileInfo{
				Name:           name,
				AppSupportPath: filepath.Join(appSupportRoot, name),
				CachePath:      filepath.Join(cacheRoot, name),
			})
			continue
		}
		// Also detect profiles containing a Preferences file
		if _, err := os.Stat(filepath.Join(appSupportRoot, name, "Preferences")); err == nil {
			profiles = append(profiles, ProfileInfo{
				Name:           name,
				AppSupportPath: filepath.Join(appSupportRoot, name),
				CachePath:      filepath.Join(cacheRoot, name),
			})
		}
	}

	return profiles
}

// scanChromiumBrowser scans all profiles of a Chromium-based browser.
func scanChromiumBrowser(
	ctx context.Context,
	homeDir, goos string,
	bDef BrowserDef,
	isProcessBlocked bool,
	cutoff time.Time,
	maxAgeDays int,
) ([]domain.Target, error) {
	appSupportRoot := bDef.AppSupportDir(homeDir, goos)
	cacheRoot := bDef.CacheDir(homeDir, goos)

	profiles := discoverChromiumProfiles(appSupportRoot, cacheRoot)
	if len(profiles) == 0 {
		return nil, nil
	}

	var totalCacheSize, eligibleCacheBytes int64
	var hasEligibleCache bool
	var latestCacheTime time.Time

	var totalScriptCacheSize int64
	var latestScriptCacheTime time.Time

	for _, prof := range profiles {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		// 1. Scan AppSupport subfolders
		appSupportSubLeaves := []string{
			"GPUCache",
			"DawnCache",
			"DawnWebGPUCache",
			"DawnGraphiteCache",
			"Code Cache",
			filepath.Join("Service Worker", "CacheStorage"),
		}
		for _, leaf := range appSupportSubLeaves {
			p := filepath.Join(prof.AppSupportPath, leaf)
			if IsSessionOrProtectedData(p) {
				continue
			}
			info, err := os.Stat(p)
			if err != nil {
				continue
			}
			sz, _ := devfs.CalcDirSize(p)
			if sz == 0 {
				continue
			}
			totalCacheSize += sz
			if info.ModTime().After(latestCacheTime) {
				latestCacheTime = info.ModTime()
			}
			if maxAgeDays == 0 || info.ModTime().Before(cutoff) {
				eligibleCacheBytes += sz
				hasEligibleCache = true
			}
		}

		// 2. Scan OS Cache subfolders (e.g. ~/Library/Caches/Google/Chrome/Default/Cache_Data)
		cacheSubLeaves := []string{
			"Cache_Data",
			"Media Cache",
			"GPUCache",
			"Code Cache",
		}
		for _, leaf := range cacheSubLeaves {
			p := filepath.Join(prof.CachePath, leaf)
			if IsSessionOrProtectedData(p) {
				continue
			}
			info, err := os.Stat(p)
			if err != nil {
				continue
			}
			sz, _ := devfs.CalcDirSize(p)
			if sz == 0 {
				continue
			}
			totalCacheSize += sz
			if info.ModTime().After(latestCacheTime) {
				latestCacheTime = info.ModTime()
			}
			if maxAgeDays == 0 || info.ModTime().Before(cutoff) {
				eligibleCacheBytes += sz
				hasEligibleCache = true
			}
		}

		// 3. Scan ScriptCache (Service Worker/ScriptCache) - SEGREGATED (BROW-04, D-05)
		scPath := filepath.Join(prof.AppSupportPath, "Service Worker", "ScriptCache")
		if info, err := os.Stat(scPath); err == nil && !IsSessionOrProtectedData(scPath) {
			sz, _ := devfs.CalcDirSize(scPath)
			if sz > 0 {
				totalScriptCacheSize += sz
				if info.ModTime().After(latestScriptCacheTime) {
					latestScriptCacheTime = info.ModTime()
				}
			}
		}
	}

	var targets []domain.Target

	// Target 1: Main Consolidated Browser Caches
	if totalCacheSize > 0 {
		if latestCacheTime.IsZero() {
			latestCacheTime = time.Now()
		}
		vPath := "browser://" + bDef.ID + "/caches"
		trg := domain.Target{
			ID:            hashBrowserPath(vPath),
			Name:          bDef.Name + " Caches",
			Path:          vPath,
			SizeBytes:     totalCacheSize,
			EligibleBytes: eligibleCacheBytes,
			TTLEligible:   hasEligibleCache,
			Risk:          domain.RiskLow,
			LastAccessed:  latestCacheTime,
			Category:      "browser-cache",
		}
		if isProcessBlocked {
			trg.ReviewOnly = true
			trg.ReviewReason = domain.ReviewReasonActiveProcess
			trg.EligibleBytes = 0
			trg.TTLEligible = false
		}
		targets = append(targets, trg)
	}

	// Target 2: Dedicated ScriptCache Review Target (BROW-04, D-05, D-06)
	if totalScriptCacheSize > 0 && bDef.HasScriptCache {
		if latestScriptCacheTime.IsZero() {
			latestScriptCacheTime = time.Now()
		}
		vPath := "browser://" + bDef.ID + "/scriptcache"
		trg := domain.Target{
			ID:            hashBrowserPath(vPath),
			Name:          bDef.Name + " ScriptCache",
			Path:          vPath,
			SizeBytes:     totalScriptCacheSize,
			EligibleBytes: 0,                                // INVIOLABLE: EligibleBytes must be 0 in v2.3!
			TTLEligible:   false,                            // INVIOLABLE: TTLEligible must be false!
			ReviewOnly:    true,                             // INVIOLABLE: ReviewOnly must be true!
			ReviewReason:  "scriptcache_requires_evidence", // INVIOLABLE: Gated until Phase 14
			Risk:          domain.RiskMedium,
			LastAccessed:  latestScriptCacheTime,
			Category:      "browser-scriptcache",
		}
		targets = append(targets, trg)
	}

	return targets, nil
}

// cleanChromiumBrowser surgically unlinks allowlisted cache leaves for all profiles.
func cleanChromiumBrowser(homeDir, goos string, bDef BrowserDef) error {
	appSupportRoot := bDef.AppSupportDir(homeDir, goos)
	cacheRoot := bDef.CacheDir(homeDir, goos)
	profiles := discoverChromiumProfiles(appSupportRoot, cacheRoot)

	for _, prof := range profiles {
		// AppSupport allowlisted leaves
		appLeaves := []string{
			"GPUCache",
			"DawnCache",
			"DawnWebGPUCache",
			"DawnGraphiteCache",
			"Code Cache",
			filepath.Join("Service Worker", "CacheStorage"),
		}
		for _, leaf := range appLeaves {
			p := filepath.Join(prof.AppSupportPath, leaf)
			if err := VerifyCleanCandidate(p); err != nil {
				continue
			}
			if isLink, err := safety.IsSymlink(p); err == nil && !isLink {
				_ = os.RemoveAll(p)
			}
		}

		// OS Cache allowlisted leaves
		cacheLeaves := []string{
			"Cache_Data",
			"Media Cache",
			"GPUCache",
			"Code Cache",
		}
		for _, leaf := range cacheLeaves {
			p := filepath.Join(prof.CachePath, leaf)
			if err := VerifyCleanCandidate(p); err != nil {
				continue
			}
			if isLink, err := safety.IsSymlink(p); err == nil && !isLink {
				_ = os.RemoveAll(p)
			}
		}
	}
	return nil
}

func hashBrowserPath(path string) string {
	h := sha256.Sum256([]byte(path))
	return hex.EncodeToString(h[:8])
}
```

---

### 3.6. Mozilla Firefox Multi-Profile Traversal & Shielding (`internal/cleaner/browsers/firefox.go`)

- **Role:** Varredura dinâmica de múltiplos perfis Firefox (`*.default*`), agregação de subpastas de cache regeneráveis e garantia de zero ScriptCache.
- **Data Flow:** Descobre perfis sob `Firefox/Profiles` (macOS) ou `.mozilla/firefox` (Linux), agrega `cache2`, `startupCache`, etc., e emite `browser://firefox/caches`.
- **Closest Analog:** `internal/cleaner/ai/editors.go:227-293` e `internal/cleaner/macos/user.go:60-168`.
- **Decisions Enforced:** `BROW-01`, `BROW-02`, `BROW-03`, `D-03`, `D-08`.

#### Concrete Adaptation Pattern (`internal/cleaner/browsers/firefox.go`)
```go
package browsers

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gtkpad/devclean/internal/domain"
	devfs "github.com/gtkpad/devclean/internal/fs"
	"github.com/gtkpad/devclean/internal/safety"
)

// discoverFirefoxProfiles discovers profile folders matching *.default* or containing prefs.js.
func discoverFirefoxProfiles(appSupportRoot, cacheRoot string) []ProfileInfo {
	var profiles []ProfileInfo
	entries, err := os.ReadDir(appSupportRoot)
	if err != nil {
		return profiles
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.Contains(name, ".default") || strings.Contains(name, ".release") {
			profiles = append(profiles, ProfileInfo{
				Name:           name,
				AppSupportPath: filepath.Join(appSupportRoot, name),
				CachePath:      filepath.Join(cacheRoot, name),
			})
			continue
		}
		if _, err := os.Stat(filepath.Join(appSupportRoot, name, "prefs.js")); err == nil {
			profiles = append(profiles, ProfileInfo{
				Name:           name,
				AppSupportPath: filepath.Join(appSupportRoot, name),
				CachePath:      filepath.Join(cacheRoot, name),
			})
		}
	}
	return profiles
}

// scanFirefox scans allowlisted cache subdirectories across all Firefox profiles.
func scanFirefox(
	ctx context.Context,
	homeDir, goos string,
	bDef BrowserDef,
	isProcessBlocked bool,
	cutoff time.Time,
	maxAgeDays int,
) ([]domain.Target, error) {
	appSupportRoot := bDef.AppSupportDir(homeDir, goos)
	cacheRoot := bDef.CacheDir(homeDir, goos)

	profiles := discoverFirefoxProfiles(appSupportRoot, cacheRoot)
	if len(profiles) == 0 {
		return nil, nil
	}

	var totalCacheSize, eligibleCacheBytes int64
	var hasEligibleCache bool
	var latestCacheTime time.Time

	for _, prof := range profiles {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		// AppSupport allowlisted leaves
		appLeaves := []string{"startupCache", "jumpListCache"}
		for _, leaf := range appLeaves {
			p := filepath.Join(prof.AppSupportPath, leaf)
			if IsSessionOrProtectedData(p) {
				continue
			}
			info, err := os.Stat(p)
			if err != nil {
				continue
			}
			sz, _ := devfs.CalcDirSize(p)
			if sz == 0 {
				continue
			}
			totalCacheSize += sz
			if info.ModTime().After(latestCacheTime) {
				latestCacheTime = info.ModTime()
			}
			if maxAgeDays == 0 || info.ModTime().Before(cutoff) {
				eligibleCacheBytes += sz
				hasEligibleCache = true
			}
		}

		// OS Cache allowlisted leaves (e.g. ~/Library/Caches/Firefox/Profiles/<profile>/cache2)
		cacheLeaves := []string{"cache2", "startupCache", "thumbnails"}
		for _, leaf := range cacheLeaves {
			p := filepath.Join(prof.CachePath, leaf)
			if IsSessionOrProtectedData(p) {
				continue
			}
			info, err := os.Stat(p)
			if err != nil {
				continue
			}
			sz, _ := devfs.CalcDirSize(p)
			if sz == 0 {
				continue
			}
			totalCacheSize += sz
			if info.ModTime().After(latestCacheTime) {
				latestCacheTime = info.ModTime()
			}
			if maxAgeDays == 0 || info.ModTime().Before(cutoff) {
				eligibleCacheBytes += sz
				hasEligibleCache = true
			}
		}
	}

	if totalCacheSize == 0 {
		return nil, nil
	}

	if latestCacheTime.IsZero() {
		latestCacheTime = time.Now()
	}

	vPath := "browser://firefox/caches"
	trg := domain.Target{
		ID:            hashBrowserPath(vPath),
		Name:          "Mozilla Firefox Caches",
		Path:          vPath,
		SizeBytes:     totalCacheSize,
		EligibleBytes: eligibleCacheBytes,
		TTLEligible:   hasEligibleCache,
		Risk:          domain.RiskLow,
		LastAccessed:  latestCacheTime,
		Category:      "browser-cache",
	}

	if isProcessBlocked {
		trg.ReviewOnly = true
		trg.ReviewReason = domain.ReviewReasonActiveProcess
		trg.EligibleBytes = 0
		trg.TTLEligible = false
	}

	return []domain.Target{trg}, nil
}

// cleanFirefox removes allowlisted cache leaves while strictly preserving SQLite databases.
func cleanFirefox(homeDir, goos string, bDef BrowserDef) error {
	appSupportRoot := bDef.AppSupportDir(homeDir, goos)
	cacheRoot := bDef.CacheDir(homeDir, goos)
	profiles := discoverFirefoxProfiles(appSupportRoot, cacheRoot)

	for _, prof := range profiles {
		// AppSupport allowlisted leaves
		for _, leaf := range []string{"startupCache", "jumpListCache"} {
			p := filepath.Join(prof.AppSupportPath, leaf)
			if err := VerifyCleanCandidate(p); err != nil {
				continue
			}
			if isLink, err := safety.IsSymlink(p); err == nil && !isLink {
				_ = os.RemoveAll(p)
			}
		}
		// OS Cache allowlisted leaves
		for _, leaf := range []string{"cache2", "startupCache", "thumbnails"} {
			p := filepath.Join(prof.CachePath, leaf)
			if err := VerifyCleanCandidate(p); err != nil {
				continue
			}
			if isLink, err := safety.IsSymlink(p); err == nil && !isLink {
				_ = os.RemoveAll(p)
			}
		}
	}
	return nil
}
```

---

### 3.7. Apple Safari Scanner, Isolation & TCC Degradation (`internal/cleaner/browsers/safari.go`)

- **Role:** Varredura restrita a `~/Library/Caches/com.apple.Safari` no macOS, blindagem total de `~/Library/Safari` e degradação graciosa em TCC / Full Disk Access.
- **Data Flow:** Inspeciona `WebKitCache` e `Cache.db*`, captura erros de permissão (`EACCES`/`EPERM`) sem invocar `sudo`, emite `browser://safari/caches`.
- **Closest Analog:** `internal/cleaner/macos/system.go:20-65` e `internal/cleaner/macos/whitelist.go:20-25`.
- **Decisions Enforced:** `BROW-01`, `BROW-02`, `BROW-03`, `D-04`, `D-08`, `D-13`, `D-14`.

#### Existing Code Excerpt (`internal/cleaner/macos/system.go:45-64`)
```go
		if os.IsPermission(err) {
			targets = append(targets, domain.Target{
				ID:           "macos-system-logs-permission-denied",
				Name:         "macOS System Logs & Diagnostic Reports (permission denied)",
				Path:         logDir,
				ReviewOnly:   true,
				ReviewReason: domain.ReviewReasonSystemProtected,
				ProviderMetrics: &domain.ProviderMetrics{
					Partial:    true,
					StatusNote: "permission denied (TCC / root required)",
				},
				Risk: domain.RiskMedium,
			})
			return targets, nil
		}
```

#### Concrete Adaptation Pattern (`internal/cleaner/browsers/safari.go`)
```go
package browsers

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gtkpad/devclean/internal/domain"
	devfs "github.com/gtkpad/devclean/internal/fs"
	"github.com/gtkpad/devclean/internal/safety"
)

// scanSafari scans strictly ~/Library/Caches/com.apple.Safari on macOS.
func scanSafari(
	ctx context.Context,
	homeDir string,
	bDef BrowserDef,
	isProcessBlocked bool,
	cutoff time.Time,
	maxAgeDays int,
) ([]domain.Target, error) {
	cacheRoot := bDef.CacheDir(homeDir, "darwin")
	if cacheRoot == "" {
		return nil, nil
	}

	entries, err := os.ReadDir(cacheRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		// Graceful degradation on TCC / Permission Denied (D-14)
		if os.IsPermission(err) || isSyscallPermission(err) {
			vPath := "browser://safari/caches"
			trg := domain.Target{
				ID:            hashBrowserPath(vPath),
				Name:          "Safari Caches",
				Path:          vPath,
				ReviewOnly:    true,
				ReviewReason:  domain.ReviewReasonSystemProtected,
				EligibleBytes: 0,
				TTLEligible:   false,
				Risk:          domain.RiskLow,
				ProviderMetrics: &domain.ProviderMetrics{
					Partial:    true,
					StatusNote: "permission denied (TCC / Full Disk Access required)",
				},
				Category: "browser-cache",
			}
			return []domain.Target{trg}, nil
		}
		return nil, err
	}

	var totalCacheSize, eligibleCacheBytes int64
	var hasEligibleCache bool
	var latestCacheTime time.Time
	var hitPermissionError bool

	allowlistedItems := []string{"WebKitCache", "Cache.db", "Cache.db-wal", "Cache.db-shm", "fsCachedData"}

	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		name := entry.Name()
		matched := false
		for _, allow := range allowlistedItems {
			if name == allow || strings.HasPrefix(name, allow) {
				matched = true
				break
			}
		}
		if !matched {
			continue // Skip un-allowlisted files
		}

		p := filepath.Join(cacheRoot, name)
		if IsSessionOrProtectedData(p) {
			continue
		}

		info, err := os.Stat(p)
		if err != nil {
			if os.IsPermission(err) || isSyscallPermission(err) {
				hitPermissionError = true
			}
			continue
		}

		sz, err := devfs.CalcDirSize(p)
		if err != nil && (os.IsPermission(err) || isSyscallPermission(err)) {
			hitPermissionError = true
		}
		if !info.IsDir() {
			sz = info.Size()
		}
		if sz == 0 {
			continue
		}

		totalCacheSize += sz
		if info.ModTime().After(latestCacheTime) {
			latestCacheTime = info.ModTime()
		}
		if maxAgeDays == 0 || info.ModTime().Before(cutoff) {
			eligibleCacheBytes += sz
			hasEligibleCache = true
		}
	}

	if totalCacheSize == 0 && !hitPermissionError {
		return nil, nil
	}

	if latestCacheTime.IsZero() {
		latestCacheTime = time.Now()
	}

	vPath := "browser://safari/caches"
	trg := domain.Target{
		ID:            hashBrowserPath(vPath),
		Name:          "Safari Caches",
		Path:          vPath,
		SizeBytes:     totalCacheSize,
		EligibleBytes: eligibleCacheBytes,
		TTLEligible:   hasEligibleCache,
		Risk:          domain.RiskLow,
		LastAccessed:  latestCacheTime,
		Category:      "browser-cache",
	}

	if isProcessBlocked {
		trg.ReviewOnly = true
		trg.ReviewReason = domain.ReviewReasonActiveProcess
		trg.EligibleBytes = 0
		trg.TTLEligible = false
	} else if hitPermissionError {
		trg.ReviewOnly = true
		trg.ReviewReason = domain.ReviewReasonSystemProtected
		trg.EligibleBytes = 0
		trg.TTLEligible = false
		trg.ProviderMetrics = &domain.ProviderMetrics{
			Partial:    true,
			StatusNote: "permission denied (TCC / Full Disk Access required)",
		}
	}

	return []domain.Target{trg}, nil
}

// cleanSafari unlinks allowlisted cache leaves under ~/Library/Caches/com.apple.Safari.
func cleanSafari(homeDir string) error {
	cacheRoot := filepath.Join(homeDir, "Library", "Caches", "com.apple.Safari")
	allowlistedItems := []string{"WebKitCache", "Cache.db", "Cache.db-wal", "Cache.db-shm", "fsCachedData"}

	for _, item := range allowlistedItems {
		p := filepath.Join(cacheRoot, item)
		if err := VerifyCleanCandidate(p); err != nil {
			continue
		}
		if isLink, err := safety.IsSymlink(p); err == nil && !isLink {
			_ = os.RemoveAll(p)
		}
	}
	return nil
}

func isSyscallPermission(err error) bool {
	if err == nil {
		return false
	}
	var errno syscall.Errno
	if ok := errors.As(err, &errno); ok {
		return errno == syscall.EACCES || errno == syscall.EPERM
	}
	return false
}
```

---

### 3.8. Browser Cleaner Lifecycle Orchestrator (`internal/cleaner/browsers/browsers.go`)

- **Role:** Entrypoint principal que implementa a interface `domain.Cleaner` (`Name() = "browsers"`), orquestra o scan, resolve rotas virtuais (`browser://`), revalida processos atomicamente pré-Clean e impõe guarda ativa contra mutação de ScriptCache.
- **Data Flow:** Acionado pelo motor (`engine.Scan`) e pelo coordenador de deleção (`deleter.Coordinator.Delete`).
- **Closest Analog:** `internal/cleaner/ai/ai.go:16-58, 284-330` e `internal/cleaner/macos/macos.go:16-53, 100-148`.
- **Decisions Enforced:** `BROW-01`, `BROW-02`, `BROW-04`, `D-07`, `D-09`, `D-10`.

#### Existing Code Excerpt (`internal/cleaner/ai/ai.go:284-304`)
```go
func (c *aiCleaner) Clean(ctx context.Context, targets []domain.Target) error {
	for _, t := range targets {
		if err := ctx.Err(); err != nil {
			return err
		}

		// Virtual editor targets are cleaned surgically with pre-deletion process revalidation
		if strings.HasPrefix(t.Path, "editor://") {
			editorID, _ := extractEditorIDFromURI(t.Path)
			if edDef, ok := findEditorDef(editorID); ok {
				running, uncertain := checkProcessStatus(ctx, c.exec, edDef.processNames...)
				if running || uncertain {
					continue // Abort deleting targets of active editor to prevent data corruption (D-07)
				}
			}
			if err := cleanEditorVirtualTarget(c.homeDir, t); err != nil {
				return fmt.Errorf("failed to clean editor target %s: %w", t.Path, err)
			}
			continue
		}
		// ...
```

#### Concrete Adaptation Pattern (`internal/cleaner/browsers/browsers.go`)
```go
package browsers

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/gtkpad/devclean/internal/cleaner"
	"github.com/gtkpad/devclean/internal/domain"
)

type browsersCleaner struct {
	exec    cleaner.CommandExecutor
	homeDir string
	goos    string
}

// New creates a new browser cleaner with DefaultExecutor and current user home.
func New() domain.Cleaner {
	return NewWithExecutor(&cleaner.DefaultExecutor{})
}

// NewWithExecutor creates a new browser cleaner with custom executor.
func NewWithExecutor(exec cleaner.CommandExecutor) domain.Cleaner {
	home, _ := os.UserHomeDir()
	return &browsersCleaner{
		exec:    exec,
		homeDir: home,
		goos:    runtime.GOOS,
	}
}

// NewWithHome creates a browser cleaner with custom home for hermetic testing.
func NewWithHome(homeDir string) domain.Cleaner {
	return &browsersCleaner{
		exec:    &cleaner.DefaultExecutor{},
		homeDir: homeDir,
		goos:    runtime.GOOS,
	}
}

// NewWithExecutorAndHome creates a browser cleaner with custom executor and custom home.
func NewWithExecutorAndHome(exec cleaner.CommandExecutor, homeDir string) domain.Cleaner {
	return &browsersCleaner{
		exec:    exec,
		homeDir: homeDir,
		goos:    runtime.GOOS,
	}
}

func (c *browsersCleaner) Name() string {
	return "browsers"
}

func (c *browsersCleaner) IsInstalled() bool {
	for _, bDef := range GetBrowserDefinitions() {
		if bDef.DarwinOnly && c.goos != "darwin" {
			continue
		}
		if bDef.AppSupportDir != nil {
			if p := bDef.AppSupportDir(c.homeDir, c.goos); p != "" {
				if _, err := os.Stat(p); err == nil {
					return true
				}
			}
		}
		if bDef.CacheDir != nil {
			if p := bDef.CacheDir(c.homeDir, c.goos); p != "" {
				if _, err := os.Stat(p); err == nil {
					return true
				}
			}
		}
	}
	return false
}

func (c *browsersCleaner) Scan(ctx context.Context, rules domain.EcosystemRules) ([]domain.Target, error) {
	if !rules.Enabled {
		return nil, nil
	}

	var allTargets []domain.Target
	cutoff := time.Now().AddDate(0, 0, -rules.MaxAgeDays)

	for _, bDef := range GetBrowserDefinitions() {
		if bDef.DarwinOnly && c.goos != "darwin" {
			continue
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		running, uncertain := CheckBrowserProcessStatus(ctx, c.exec, bDef, c.goos)
		isProcessBlocked := running || uncertain

		switch bDef.Engine {
		case EngineWebKit:
			targets, err := scanSafari(ctx, c.homeDir, bDef, isProcessBlocked, cutoff, rules.MaxAgeDays)
			if err == nil {
				allTargets = append(allTargets, targets...)
			}
		case EngineGecko:
			targets, err := scanFirefox(ctx, c.homeDir, c.goos, bDef, isProcessBlocked, cutoff, rules.MaxAgeDays)
			if err == nil {
				allTargets = append(allTargets, targets...)
			}
		default: // EngineChromium: chrome, brave, arc, edge
			targets, err := scanChromiumBrowser(ctx, c.homeDir, c.goos, bDef, isProcessBlocked, cutoff, rules.MaxAgeDays)
			if err == nil {
				allTargets = append(allTargets, targets...)
			}
		}
	}

	return allTargets, nil
}

func (c *browsersCleaner) Clean(ctx context.Context, targets []domain.Target) error {
	for _, t := range targets {
		if err := ctx.Err(); err != nil {
			return err
		}

		// Stage 1: Defensive Refusal Guard against ReviewOnly & ScriptCache (D-05, D-07)
		if t.ReviewOnly || t.Category == "browser-scriptcache" || strings.HasSuffix(t.Path, "/scriptcache") {
			continue // Inviolable refusal: ScriptCache and ReviewOnly items are never mutated!
		}

		// Resolve browser ID from virtual URI (browser://<id>/caches)
		browserID := extractBrowserIDFromURI(t.Path)
		bDef, ok := FindBrowserDef(browserID)
		if !ok {
			continue
		}

		// Stage 2: Atomic Pre-Clean Process Revalidation (D-10)
		running, uncertain := CheckBrowserProcessStatus(ctx, c.exec, bDef, c.goos)
		if running || uncertain {
			continue // Abort if browser was opened between scan and clean! (Fail-closed race prevention)
		}

		// Stage 3: Surgical Unlinking of Allowlisted Leaves
		var err error
		switch bDef.Engine {
		case EngineWebKit:
			err = cleanSafari(c.homeDir)
		case EngineGecko:
			err = cleanFirefox(c.homeDir, c.goos, bDef)
		default:
			err = cleanChromiumBrowser(c.homeDir, c.goos, bDef)
		}

		if err != nil {
			return fmt.Errorf("cleaning browser %s: %w", bDef.Name, err)
		}
	}
	return nil
}

func extractBrowserIDFromURI(uri string) string {
	trimmed := strings.TrimPrefix(uri, "browser://")
	parts := strings.SplitN(trimmed, "/", 2)
	return parts[0]
}
```

---

### 3.9. Hermetic Unit Tests with Double Sentinels (`internal/cleaner/browsers/browsers_test.go`)

- **Role:** Validação unitária hermética (`VAL-01`, `VAL-02`), simulando o ciclo de vida completo de varredura e limpeza com zero CGO e cobertura determinística de processos.
- **Data Flow:** Executa `Scan()` e `Clean()` em árvores `t.TempDir()` povoadas com sentinelas duplas (arquivos de cache descartáveis vs dados de sessão protegidos).
- **Closest Analog:** `internal/cleaner/ai/ai_test.go:616-850` e `internal/cleaner/macos/macos_test.go:77-176`.
- **Decisions Enforced:** `VAL-01`, `VAL-02`, `BROW-01` a `BROW-04`, `D-05`, `D-09` a `D-16`.

#### Concrete Test Fixture Pattern
```go
package browsers

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/gtkpad/devclean/internal/cleaner"
	"github.com/gtkpad/devclean/internal/domain"
)

// setupBrowserTestFixtures populates a temporary directory with double sentinels:
// disposable caches (to be deleted) vs protected session files (MUST SURVIVE).
func setupBrowserTestFixtures(t *testing.T, homeDir string) (disposableFiles []string, protectedFiles []string, scriptCacheFiles []string) {
	// 1. Chrome
	chromeApp := filepath.Join(homeDir, "Library", "Application Support", "Google", "Chrome", "Default")
	chromeCache := filepath.Join(homeDir, "Library", "Caches", "Google", "Chrome", "Default")
	if runtime.GOOS != "darwin" {
		chromeApp = filepath.Join(homeDir, ".config", "google-chrome", "Default")
		chromeCache = filepath.Join(homeDir, ".cache", "google-chrome", "Default")
	}

	// Disposables
	d1 := filepath.Join(chromeApp, "GPUCache", "data_0")
	d2 := filepath.Join(chromeApp, "Code Cache", "js", "entry1")
	d3 := filepath.Join(chromeApp, "Service Worker", "CacheStorage", "index")
	d4 := filepath.Join(chromeCache, "Cache_Data", "f_000001")
	for _, p := range []string{d1, d2, d3, d4} {
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		_ = os.WriteFile(p, []byte("disposable-cache-data"), 0o644)
		disposableFiles = append(disposableFiles, p)
	}

	// Protected Session Files
	s1 := filepath.Join(chromeApp, "Cookies")
	s2 := filepath.Join(chromeApp, "History")
	s3 := filepath.Join(chromeApp, "Login Data")
	s4 := filepath.Join(chromeApp, "Local Storage", "leveldb", "000003.log")
	s5 := filepath.Join(chromeApp, "IndexedDB", "site.db")
	s6 := filepath.Join(chromeApp, "Sessions", "Session_123")
	for _, p := range []string{s1, s2, s3, s4, s5, s6} {
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		_ = os.WriteFile(p, []byte("PROTECTED-SESSION-SENTINEL"), 0o644)
		protectedFiles = append(protectedFiles, p)
	}

	// Gated ScriptCache (MUST SURVIVE)
	sc1 := filepath.Join(chromeApp, "Service Worker", "ScriptCache", "script_0")
	_ = os.MkdirAll(filepath.Dir(sc1), 0o755)
	_ = os.WriteFile(sc1, []byte("GATED-SCRIPT-CACHE"), 0o644)
	scriptCacheFiles = append(scriptCacheFiles, sc1)

	// 2. Firefox
	ffApp := filepath.Join(homeDir, "Library", "Application Support", "Firefox", "Profiles", "abc.default-release")
	ffCache := filepath.Join(homeDir, "Library", "Caches", "Firefox", "Profiles", "abc.default-release")
	if runtime.GOOS != "darwin" {
		ffApp = filepath.Join(homeDir, ".mozilla", "firefox", "abc.default-release")
		ffCache = filepath.Join(homeDir, ".cache", "mozilla", "firefox", "abc.default-release")
	}
	d5 := filepath.Join(ffCache, "cache2", "entries", "000001")
	_ = os.MkdirAll(filepath.Dir(d5), 0o755)
	_ = os.WriteFile(d5, []byte("firefox-cache"), 0o644)
	disposableFiles = append(disposableFiles, d5)

	s7 := filepath.Join(ffApp, "places.sqlite")
	s8 := filepath.Join(ffApp, "key4.db")
	s9 := filepath.Join(ffApp, "logins.json")
	for _, p := range []string{s7, s8, s9} {
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		_ = os.WriteFile(p, []byte("PROTECTED-FIREFOX-SESSION"), 0o644)
		protectedFiles = append(protectedFiles, p)
	}

	// 3. Safari (macOS only)
	if runtime.GOOS == "darwin" {
		safariCache := filepath.Join(homeDir, "Library", "Caches", "com.apple.Safari")
		d6 := filepath.Join(safariCache, "WebKitCache", "v16")
		d7 := filepath.Join(safariCache, "Cache.db")
		for _, p := range []string{d6, d7} {
			_ = os.MkdirAll(filepath.Dir(p), 0o755)
			_ = os.WriteFile(p, []byte("safari-cache"), 0o644)
			disposableFiles = append(disposableFiles, p)
		}

		safariPrivate := filepath.Join(homeDir, "Library", "Safari")
		s10 := filepath.Join(safariPrivate, "History.db")
		s11 := filepath.Join(safariPrivate, "Bookmarks.plist")
		for _, p := range []string{s10, s11} {
			_ = os.MkdirAll(filepath.Dir(p), 0o755)
			_ = os.WriteFile(p, []byte("PROTECTED-SAFARI-PRIVATE"), 0o644)
			protectedFiles = append(protectedFiles, p)
		}
	}

	return disposableFiles, protectedFiles, scriptCacheFiles
}

func TestBrowsers_InviolableSessionShield(t *testing.T) {
	home := t.TempDir()
	disposables, protected, scriptCaches := setupBrowserTestFixtures(t, home)

	mockExec := cleaner.NewMockExecutor()
	// Set all browsers cleanly stopped (pgrep exit 1)
	c := NewWithExecutorAndHome(mockExec, home)

	rules := domain.EcosystemRules{Enabled: true, MaxAgeDays: 0}
	targets, err := c.Scan(context.Background(), rules)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Clean all returned targets
	if err := c.Clean(context.Background(), targets); err != nil {
		t.Fatalf("Clean failed: %v", err)
	}

	// 1. Assert: 100% of disposable cache files were REMOVED!
	for _, p := range disposables {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("expected disposable cache file %s to be deleted, but it still exists", p)
		}
	}

	// 2. Assert: 100% of protected session files SURVIVED!
	for _, p := range protected {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("SECURITY CATASTROPHE: session file %s was deleted or damaged: %v", p, err)
		}
	}

	// 3. Assert: 100% of ScriptCache files SURVIVED! (D-05, D-07)
	for _, p := range scriptCaches {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("GATE VIOLATION: ScriptCache file %s was deleted: %v", p, err)
		}
	}
}
```

---

### 3.10. Deduplication Update in Generic macOS Cleaner (`internal/cleaner/macos/user.go`)

- **Role:** Registro de exclusão canônica para diretórios de navegadores em `~/Library/Caches`, evitando contagem dupla de bytes (`SCAN-04`).
- **Data Flow:** `scanUserCaches()` consulta `isExcludedByDedicatedCleaner(name)` e ignora esses diretórios.
- **Closest Analog:** `internal/cleaner/macos/user.go:15-30`.
- **Decisions Enforced:** `SCAN-04`, `D-04`.

#### Existing Code Excerpt (`internal/cleaner/macos/user.go:15-30`)
```go
// Exclusion table for directories in ~/Library/Caches handled by dedicated cleaners (D-05, SCAN-04).
var dedicatedEcosystemCaches = map[string]string{
	"pnpm":                  "pnpm",
	"uv":                    "python",
	"pip":                   "python",
	"Homebrew":              "homebrew",
	"CocoaPods":             "cocoapods",
	"vcpkg":                 "cpp",
	"mise":                  "shell",
	"gh":                    "shell",
	"ms-playwright":         "node",
	"Cypress":               "node",
	"node":                  "node",
	"Yarn":                  "node",
	"dev.orbstack.OrbStack": "docker",
}
```

#### Concrete Modification
```go
// Exclusion table for directories in ~/Library/Caches handled by dedicated cleaners (D-05, SCAN-04).
var dedicatedEcosystemCaches = map[string]string{
	"pnpm":                   "pnpm",
	"uv":                     "python",
	"pip":                    "python",
	"Homebrew":               "homebrew",
	"CocoaPods":              "cocoapods",
	"vcpkg":                  "cpp",
	"mise":                   "shell",
	"gh":                     "shell",
	"ms-playwright":          "node",
	"Cypress":                "node",
	"node":                   "node",
	"Yarn":                   "node",
	"dev.orbstack.OrbStack":  "docker",
	"Google":                 "browsers",
	"BraveSoftware":          "browsers",
	"company.thebrowser.Arc": "browsers",
	"Microsoft Edge":         "browsers",
	"Firefox":                "browsers",
	"com.apple.Safari":       "browsers",
}
```

---

### 3.11. Engine & Config Wiring

#### 3.11.1. Registration in `internal/engine/register.go`
```go
	// Ecossistema de navegadores web (Fase 12)
	eng.Register(browsers.NewWithExecutor(exec))

	// Ecossistema do sistema operacional
	eng.Register(macos.NewWithExecutor(exec))
```

#### 3.11.2. Configuration Registration in `internal/config/config.go` & `defaults.go`
- In `config.go`:
  - `EcosystemsConfig`: `Browsers EcosystemConfig \`mapstructure:"browsers"\``
  - `HooksConfig`: `Browsers HookConfig \`mapstructure:"browsers"\``
  - `Load()`: `v.SetDefault("ecosystems.browsers.enabled", defaults.Ecosystems.Browsers.Enabled)` etc.
- In `defaults.go`:
  - `Ecosystems.Browsers = EcosystemConfig{Enabled: true, MaxAgeDays: 14, FailMode: "continue", ExcludePaths: []string{}}`
  - `Hooks.Browsers = HookConfig{Timeout: "30s"}`

#### 3.11.3. Engine Config Routing in `internal/engine/engine.go`
```go
	case "macos":
		return e.config.Ecosystems.MacOS
	case "browsers":
		return e.config.Ecosystems.Browsers
```

#### 3.11.4. CLI MaxAgeDays in `cmd/clean.go`
```go
	case "macos":
		return cfg.Ecosystems.MacOS.MaxAgeDays
	case "browsers":
		return cfg.Ecosystems.Browsers.MaxAgeDays
```

---

## 4. Verification & Testing Strategy

Para cumprir os critérios do protocolo de validação Nyquist (`VAL-01`, `VAL-02`) com cobertura superior a 85%:

1. **Testes Unitários de Detecção On-Disk (`TestBrowsers_OnDiskDetection`):**
   - Verifica que o navegador é reportado como instalado se qualquer diretório existir no disco, mesmo sem o binário no `$PATH`.
   - Verifica que Arc e Safari são estritamente ignorados no Linux (`SCAN-02`, `BROW-01`).
2. **Testes de Traversal Multi-Perfil (`TestBrowsers_MultiProfileTraversal`):**
   - Cria fixtures com `Default`, `Profile 1`, `Profile 2`, `System Profile` e perfis Firefox `*.default*`.
   - Assegura acumulação correta de bytes de todos os perfis em um único alvo por navegador (`BROW-02`, `D-03`).
3. **Testes de Alvos Consolidados (`TestBrowsers_ConsolidatedTargets`):**
   - Confirma a emissão de exatamente 1 alvo principal de cache por navegador instalado mais 1 alvo ScriptCache para os navegadores Chromium (`D-01`, `D-06`).
4. **Testes de Blindagem Inviolável de Sessão (`TestBrowsers_InviolableSessionShield`):**
   - Comprova com sentinelas duplas que `Clean()` remove 100% dos arquivos de cache e preserva 100% dos arquivos de sessão (`Cookies`, `History`, `places.sqlite`, etc.) (`BROW-03`, `VAL-01`).
5. **Testes de Isolamento do Safari (`TestBrowsers_SafariIsolation`):**
   - Prova que `~/Library/Safari` permanece intocado sob quaisquer circunstâncias e que apenas folhas em `~/Library/Caches/com.apple.Safari` são limpas (`D-13`).
6. **Testes de Gating do ScriptCache (`TestBrowsers_ScriptCacheGated`):**
   - Assegura que o alvo de ScriptCache carrega `ReviewOnly: true`, `EligibleBytes: 0`, `TTLEligible: false` e que `Clean()` se recusa ativamente a deletá-lo (`BROW-04`, `D-05`, `D-07`).
7. **Testes de Process Guard (`TestBrowsers_ProcessGuard_Running` e `FailClosed`):**
   - Com `MockCommandExecutor`, saída `exit 0` marca `ReviewOnly: true` (`ReviewReasonActiveProcess`), enquanto outros navegadores fechados permanecem elegíveis (`D-09`).
   - Falha inesperada no `pgrep` (código > 1 ou erro genérico) resulta em `uncertain = true` e fail-closed (`D-12`).
8. **Testes de Condição de Corrida Pré-Clean (`TestBrowsers_PreCleanRevalidationRace`):**
   - Simula que o usuário iniciou o Chrome entre o scan e a limpeza; valida que `Clean()` revalida os processos e aborta a deleção fail-closed (`D-10`).
9. **Testes de Degradação Graciosa em TCC (`TestBrowsers_TCCGracefulDegradation`):**
   - Erro de permissão em cache do Safari popula `ProviderMetrics.Partial = true`, `StatusNote: "permission denied (TCC / Full Disk Access required)"`, `ReviewOnly: true` e zero chamadas a `sudo` (`D-14`).
10. **Testes de Compilação Zero-CGO (`TestBrowsers_ZeroCGO`):**
    - Validação de compilação pura sem dependências de CGO via `CGO_ENABLED=0 go test ./internal/cleaner/browsers/...`.

---

## 5. Security & Invariants Checklist

- [x] **Zero Sudo / Sem Elevação de Privilégios (`D-14`, `ASVS V14`):** DevClean opera estritamente sob o UID do usuário e nunca invoca `sudo` para contornar proteções TCC.
- [x] **Sem Injeção de Shell (`ASVS V5`):** Todas as chamadas ao `pgrep -x` são tokenizadas através da interface `cleaner.CommandExecutor`.
- [x] **Blindagem Inviolável de Sessão (`BROW-03`, `D-15`):** Positive allowlist estrutural + Invariant assertion guard rejeitando `Cookies*`, `History*`, `places.sqlite*`, `key4.db*`, `sessionstore*`, etc.
- [x] **Proteção do Safari (`D-13`):** `~/Library/Safari` é incondicionalmente bloqueado; varredura restrita a `~/Library/Caches/com.apple.Safari`.
- [x] **Gating do ScriptCache (`BROW-04`, `D-05` a `D-08`):** ScriptCache marcado estritamente como `ReviewOnly: true`, `EligibleBytes: 0` e recusado ativamente em `Clean()`.
- [x] **Process Guard Fail-Closed com Revalidação Pré-Clean (`D-09` a `D-12`):** Bloqueio isolado por navegador, fail-closed sob incerteza e rechecagem atômica imediata pré-deleção.
- [x] **Sem Dupla Contagem de Bytes (`SCAN-04`):** Exclusão formal dos 6 navegadores em `dedicatedEcosystemCaches` de `internal/cleaner/macos/user.go`.
- [x] **Testes Herméticos Zero-CGO (`VAL-01`, `VAL-02`):** `t.TempDir()`, `MockCommandExecutor`, compilação sem CGO para Darwin e Linux.

---

## 6. Sources & References

- `.planning/phases/12-browsers-session-exclusions-scriptcache-gate/12-CONTEXT.md` (User decisions D-01 a D-16)
- `.planning/phases/12-browsers-session-exclusions-scriptcache-gate/12-RESEARCH.md` (Technical research, canonical paths & threat models)
- `.planning/SPEC-v2.3-MOLE-PARITY.md` §Frente 4: Novo Cleaner de Navegadores (`internal/cleaner/browsers`)
- `internal/cleaner/ai/editors.go` & `editors_test.go` (Subfolder traversal, virtual targets & double sentinels)
- `internal/cleaner/ai/process.go` & `internal/cleaner/macos/process.go` (Process Guard fail-closed pattern)
- `internal/cleaner/macos/whitelist.go` (Immutable security list pattern)
- `internal/safety/blacklist.go` (Core system blacklist & symlink escape guards)
- `internal/cleaner/executor_mock.go` (MockCommandExecutor for deterministic testing)
- `internal/cleaner/macos/user.go` (Deduplication exclusion table)

---

## PATTERN MAPPING COMPLETE

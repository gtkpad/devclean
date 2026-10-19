# Phase 09: AI/Electron Editors & Active Versions - Pattern Mapping

**Phase:** 09-ai-electron-editors-active-versions  
**Milestone:** v2.3 Functional Parity with Mole (tw93/mole)  
**Date:** 2026-10-15  
**Status:** Approved for Implementation  
**Files Analyzed:** 10  
**Analogs Found:** 10 / 10  

---

## 1. Overview & Pattern Mapping Summary

Este documento estabelece o mapeamento estrutural entre os arquivos a serem criados e modificados na **Fase 09** e os análogos consolidados na base de código do DevClean (`internal/cleaner/xcode/xcode.go`, `internal/cleaner/node/node.go`, `internal/cleaner/pnpm/pnpm.go`, `internal/cleaner/shell/shell.go`, `internal/cleaner/ai/ai.go`, `internal/safety/blacklist.go`, `internal/safety/symlink.go`, `internal/domain/types.go`, `internal/deleter/policy_test.go`, etc.).

O objetivo desta fase é consolidar a paridade de inventário e limpeza do DevClean contra editores modernos baseados em Electron e nativos, bem como versões inativas do Claude Code CLI, cobrindo três requisitos principais:
1. **8 Editores AI e Electron (`AI-01`):** Cobertura de 8 editores (**VS Code**, **VS Code Insiders**, **Cursor**, **Antigravity**, **Windsurf**, **Claude Desktop**, **OpenCode** e **Zed**). Varrimento de folhas regeneráveis Chromium/Electron (`Cache`, `Code Cache`, `GPUCache`, `DawnCache`, `DawnWebGPUCache`, `DawnGraphiteCache`, `Service Worker/CacheStorage`, `Service Worker/ScriptCache`, `CachedData`, `CachedExtensionVSIXs`, `webviewCache`, `logs`, `Crashpad/completed`, `Sentry`) e estruturas nativas do Zed (GPUI) e OpenCode. Agrupamento funcional em **3 a 4 alvos consolidados por editor** (`Caches Chromium/GPU`, `Logs & Crash Reports`, `Extensões/VSIXs`, `Workspace Backups`), evitando poluição de listagem no TUI/CLI (`D-01`).
2. **Workspace Storage Cirúrgico e Extensões Obsoletas (`AI-02`):** Varrer estritamente `User/workspaceStorage/*/state.vscdb.backup`, preservando intacto o `state.vscdb` primário (`D-13`). Ler `.obsolete` exclusivamente como metadado sem jamais apagá-lo, expurgando apenas pastas de extensões marcadas como `true` e aplicando validações estritas contra path traversal (`..`), symlinks (`safety.IsSymlink`) e lista negra do sistema (`D-14`, `D-15`).
3. **Versões Inativas do Claude Code CLI e Preservação da Versão Ativa (`AI-03`):** Descoberta de versões em `~/.local/share/claude/versions/*` e `~/.claude/local`. Resolução do link simbólico ativo (`filepath.EvalSymlinks`) para proteger a versão em uso (`D-09`). Política fail-closed estrita: se o symlink estiver ausente, quebrado ou ambíguo, todas as versões recebem `ReviewOnly = true`, `EligibleBytes = 0`, `TTLEligible = false` e razões semânticas explícitas (`ReviewReasonActiveVersion` ou `ReviewReasonAmbiguous`), exibindo inventário real mas bloqueando deleção automática (`D-10`). Versões confirmadamente inativas recebem `RiskLow` (`D-12`).
4. **Process Guard Fail-Closed e Revalidação Pré-Deleção (`D-05`, `D-06`, `D-07`, `D-08`):** Detecção de processos via `cleaner.CommandExecutor` (`pgrep -x`) para editores (`Code`, `Code - Insiders`, `Cursor`, `Antigravity`, `Windsurf`, `Claude`, `opencode`, `zed`) e Claude CLI (`claude`). Bloqueio isolado por ferramenta (editor aberto não bloqueia outros). Revalidação imediata no `Clean()` antes de qualquer exclusão física, abortando se o processo foi iniciado após o scan.
5. **Garantias Herméticas de Validação (`VAL-01`, `VAL-02`):** Testes unitários herméticos com `t.TempDir()`, sentinelas em arquivos essenciais (`state.vscdb`, `.obsolete`, `config.json`), asserções de DryRun com mutação zero no `Coordinator`, e cobertura de código superior a 85% nos pacotes alterados.

---

## 2. File Classification Table

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `internal/domain/types.go` | model / domain | declarative constants | `internal/domain/types.go:17-24` | exact |
| `internal/domain/target_test.go` | unit test | constant assertions | `internal/domain/target_test.go:11-28` | exact |
| `internal/cleaner/ai/ai.go` | service / cleaner orchestrator | composition & revalidation | `internal/cleaner/xcode/xcode.go:21-43, 294-315`<br>`internal/cleaner/pnpm/pnpm.go:21-31, 131-151` | exact |
| `internal/cleaner/ai/editors.go` | service / scanner / cleaner | disk-scan & process guard | `internal/cleaner/xcode/xcode.go:87-290`<br>`internal/cleaner/node/node.go:160-248` | exact |
| `internal/cleaner/ai/claude.go` | service / scanner | symlink eval & disk-scan | `internal/cleaner/xcode/xcode.go:95-111`<br>`internal/safety/symlink.go`<br>`internal/cleaner/shell/shell.go:506-564` | high |
| `internal/cleaner/ai/editors_test.go` | unit test (new) | mock executor + disk fixture | `internal/cleaner/xcode/xcode_test.go`<br>`internal/cleaner/ai/ai_test.go:290-454` | exact |
| `internal/cleaner/ai/claude_test.go` | unit test (new) | symlinks + mock executor | `internal/cleaner/xcode/xcode_test.go`<br>`internal/cleaner/ai/ai_test.go:158-288` | exact |
| `internal/cleaner/ai/ai_test.go` | unit test | mock executor + full pipeline | `internal/cleaner/ai/ai_test.go:556-664` | exact |
| `internal/engine/register.go` | registry / bootstrapper | composition | `internal/engine/register.go:52-59` | exact |
| `internal/deleter/policy_test.go` | integration test | zero-mutation canary | `internal/deleter/policy_test.go:787-930` | exact |

---

## 3. Pattern Assignments & Code Excerpts

### 3.1. Domain Review Reasons: `internal/domain/types.go` (Modify)

- **Role:** Definições do modelo de domínio e constantes semânticas de revisão.
- **Data Flow:** Constantes declarativas consumidas pelo cleaner de IA, pelo motor de scanning e pelo Coordinator de deleção.
- **Closest Analog:** `internal/domain/types.go:17-24`.
- **Decisions Enforced:** `D-10`, `POL-01`.

#### Existing Code Excerpt (`internal/domain/types.go:17-24`)
```go
// Semantic Review Reasons (POL-01, D-02)
const (
	ReviewReasonSharedStore      = "shared_store"
	ReviewReasonActiveProcess    = "active_process"
	ReviewReasonOrphanLeftover   = "orphan_leftover"
	ReviewReasonSystemProtected  = "system_protected"
	ReviewReasonSessionSensitive = "session_sensitive"
)
```

#### Concrete Adaptation Pattern for Phase 09
```go
// Semantic Review Reasons (POL-01, D-02, D-10)
const (
	ReviewReasonSharedStore      = "shared_store"
	ReviewReasonActiveProcess    = "active_process"
	ReviewReasonOrphanLeftover   = "orphan_leftover"
	ReviewReasonSystemProtected  = "system_protected"
	ReviewReasonSessionSensitive = "session_sensitive"
	ReviewReasonActiveVersion    = "active_version"
	ReviewReasonAmbiguous        = "ambiguous_state"
)
```

---

### 3.2. AI Cleaner Orchestrator: `internal/cleaner/ai/ai.go` (Modify)

- **Role:** Orquestrador do ecossistema AI e cleaner principal registrado no motor.
- **Data Flow:** Recebe `cleaner.CommandExecutor` via injeção de dependência (`NewWithExecutor`), delega o scan para os submódulos (`claude`, `editors`, `ollama`, `huggingface`), e no `Clean()` executa a revalidação imediata pré-exclusão de processos ativos (`D-07`) e a deleção cirúrgica de alvos virtuais `editor://` e físicos.
- **Closest Analog:** `internal/cleaner/xcode/xcode.go:21-43, 294-315` e `internal/cleaner/pnpm/pnpm.go:21-31, 131-151`.
- **Decisions Enforced:** `D-03`, `D-05`, `D-07`, `D-14`.

#### Existing Code Excerpt (`internal/cleaner/xcode/xcode.go:21-43`)
```go
type xcodeCleaner struct {
	exec    cleaner.CommandExecutor
	baseDir string
	goos    string
}

func NewWithExecutor(exec cleaner.CommandExecutor) domain.Cleaner {
	baseDir := ""
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		baseDir = filepath.Join(home, "Library", "Developer", "Xcode")
	}
	return &xcodeCleaner{
		exec:    exec,
		baseDir: baseDir,
		goos:    runtime.GOOS,
	}
}
```

#### Existing Clean Method with Virtual Targets (`internal/cleaner/pnpm/pnpm.go:131-151`)
```go
func (c *pnpmCleaner) Clean(ctx context.Context, targets []domain.Target) error {
	for _, t := range targets {
		if t.Path == "pnpm://store/prune" {
			if _, err := c.exec.Output(ctx, "pnpm", "store", "prune"); err != nil {
				return fmt.Errorf("pnpm store prune failed: %w", err)
			}
			continue
		}

		if safety.IsCoreBlacklisted(t.Path) {
			continue
		}
		if isLink, err := safety.IsSymlink(t.Path); err == nil && isLink {
			continue
		}
		if err := os.RemoveAll(t.Path); err != nil {
			return err
		}
	}
	return nil
}
```

#### Concrete Adaptation Pattern for Phase 09
```go
type aiCleaner struct {
	exec      cleaner.CommandExecutor
	scanPaths []string
	homeDir   string
}

// New creates a new AI cleaner with default command executor and user home.
func New(scanPaths []string) domain.Cleaner {
	return NewWithExecutor(&cleaner.DefaultExecutor{}, scanPaths)
}

// NewWithExecutor creates a new AI cleaner with custom command executor.
func NewWithExecutor(exec cleaner.CommandExecutor, scanPaths []string) domain.Cleaner {
	home, _ := os.UserHomeDir()
	return &aiCleaner{
		exec:      exec,
		scanPaths: scanPaths,
		homeDir:   home,
	}
}

// NewWithHome maintains backward compatibility for testing.
func NewWithHome(scanPaths []string, homeDir string) domain.Cleaner {
	return &aiCleaner{
		exec:      &cleaner.DefaultExecutor{},
		scanPaths: scanPaths,
		homeDir:   homeDir,
	}
}

// NewWithExecutorAndHome creates an AI cleaner with custom executor and custom home.
func NewWithExecutorAndHome(exec cleaner.CommandExecutor, scanPaths []string, homeDir string) domain.Cleaner {
	return &aiCleaner{
		exec:      exec,
		scanPaths: scanPaths,
		homeDir:   homeDir,
	}
}

func (c *aiCleaner) Scan(ctx context.Context, rules domain.EcosystemRules) ([]domain.Target, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var targets []domain.Target
	if c.homeDir == "" {
		return targets, nil
	}

	// 1. Claude Code CLI & Desktop (~/.claude, ~/.local/share/claude/versions, local cache)
	claudeTargets, err := scanClaude(ctx, c.exec, c.homeDir, rules)
	if err == nil {
		targets = append(targets, claudeTargets...)
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// 2. 8 AI/Electron Editors (Code, Insiders, Cursor, Antigravity, Windsurf, Claude Desktop, OpenCode, Zed)
	editorTargets, err := scanEditors(ctx, c.exec, c.homeDir, rules)
	if err == nil {
		targets = append(targets, editorTargets...)
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// 3. Ollama
	ollamaTargets, err := scanOllama(filepath.Join(c.homeDir, ".ollama", "models"), rules)
	if err == nil {
		targets = append(targets, ollamaTargets...)
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// 4. HuggingFace
	hfTargets, err := scanHuggingFace(filepath.Join(c.homeDir, ".cache", "huggingface"), rules)
	if err == nil {
		targets = append(targets, hfTargets...)
	}

	return targets, nil
}

func (c *aiCleaner) Clean(ctx context.Context, targets []domain.Target) error {
	for _, t := range targets {
		if err := ctx.Err(); err != nil {
			return err
		}

		// Pre-deletion process revalidation (D-07)
		if strings.HasPrefix(t.Path, "editor://") {
			editorID := extractEditorIDFromURI(t.Path)
			if isEditorProcessRunning(ctx, c.exec, editorID) {
				continue // Abort deleting targets of active editor to prevent data corruption
			}
			if err := cleanEditorVirtualTarget(c.homeDir, t); err != nil {
				return fmt.Errorf("failed to clean editor target %s: %w", t.Path, err)
			}
			continue
		}

		if strings.HasPrefix(t.Category, "ai-claude") {
			running, uncertain := checkProcessStatus(ctx, c.exec, "claude")
			if running || uncertain {
				continue // Abort deleting Claude targets if CLI started after scan
			}
		}

		if safety.IsCoreBlacklisted(t.Path) {
			continue
		}
		if isLink, err := safety.IsSymlink(t.Path); err == nil && isLink {
			continue
		}

		if err := os.RemoveAll(t.Path); err != nil {
			return fmt.Errorf("ai cleaner failed to remove %s: %w", t.Path, err)
		}
	}
	return nil
}
```

---

### 3.3. 8-Editor Scanner: `internal/cleaner/ai/editors.go` (Modify / Refactor)

- **Role:** Scanner e limpador dos 8 editores suportados (Code, Code Insiders, Cursor, Antigravity, Windsurf, Claude Desktop, OpenCode, Zed).
- **Data Flow:** Executa detecção isolada de processo por editor (`checkProcessStatus`), varre folhas Chromium/Electron e estruturas nativas Zed/OpenCode, realiza busca cirúrgica de `state.vscdb.backup` e parsing seguro de `.obsolete`, agregando os resultados em **3 a 4 alvos funcionais consolidados por editor** com URIs virtuais `editor://<editor>/<category>`.
- **Closest Analog:** `internal/cleaner/xcode/xcode.go:87-290` (Process Guard e alvos expandidos) e `internal/cleaner/node/node.go:160-248` (definição de candidatos em disco multiplataforma).
- **Decisions Enforced:** `D-01`, `D-02`, `D-04`, `D-05`, `D-06`, `D-08`, `D-13`, `D-14`, `D-15`, `D-16`.

#### Editor Definitions Matrix
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

#### Process Status Check Pattern (`D-05`, `D-08`)
```go
// checkProcessStatus returns running=true if pgrep finds matching processes.
// If pgrep fails with exit code 1, running=false, uncertain=false (cleanly not running).
// If pgrep fails with any other error or missing tool, uncertain=true (FAIL CLOSED!).
func checkProcessStatus(ctx context.Context, exec cleaner.CommandExecutor, processNames ...string) (running bool, uncertain bool) {
	for _, name := range processNames {
		out, err := exec.Output(ctx, "pgrep", "-x", name)
		if err == nil && len(strings.TrimSpace(string(out))) > 0 {
			return true, false
		}
		var exitErr interface{ ExitCode() int }
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			continue // Cleanly not running for this process name
		}
		// Any unexpected error (missing pgrep, permission denied, exit > 1) -> uncertain -> FAIL CLOSED!
		return false, true
	}
	return false, false
}
```

#### Surgical Workspace Storage Scanner Pattern (`D-13`)
```go
func scanWorkspaceBackups(appSupportDir string, cutoff time.Time, maxAgeDays int) (int64, int64, bool, []string) {
	wsRoot := filepath.Join(appSupportDir, "User", "workspaceStorage")
	entries, err := os.ReadDir(wsRoot)
	if err != nil {
		return 0, 0, false, nil
	}

	var totalSize, eligibleBytes int64
	var foundFiles []string
	hasEligible := false

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		backupPath := filepath.Join(wsRoot, e.Name(), "state.vscdb.backup")
		info, err := os.Stat(backupPath)
		if err != nil || info.IsDir() {
			continue
		}

		sz := info.Size()
		totalSize += sz
		foundFiles = append(foundFiles, backupPath)

		if maxAgeDays == 0 || info.ModTime().Before(cutoff) {
			eligibleBytes += sz
			hasEligible = true
		}
	}

	return totalSize, eligibleBytes, hasEligible, foundFiles
}
```

#### Obsolete Extensions Parser Pattern (`D-14`, `D-15`)
```go
func scanObsoleteExtensions(extRoot string, cutoff time.Time, maxAgeDays int) (int64, int64, bool, []string) {
	obsoleteFile := filepath.Join(extRoot, ".obsolete")
	data, err := os.ReadFile(obsoleteFile)
	if err != nil {
		return 0, 0, false, nil
	}

	var obsoleteMap map[string]bool
	if err := json.Unmarshal(data, &obsoleteMap); err != nil {
		return 0, 0, false, nil
	}

	var totalSize, eligibleBytes int64
	var foundDirs []string
	hasEligible := false

	for extFolder, isObsolete := range obsoleteMap {
		if !isObsolete {
			continue // Positive match only! Active extensions are never touched
		}

		cleanFolder := filepath.Clean(extFolder)
		if strings.Contains(cleanFolder, "..") || filepath.IsAbs(cleanFolder) {
			continue // Path traversal defense!
		}

		fullDir := filepath.Join(extRoot, cleanFolder)
		rel, err := filepath.Rel(extRoot, fullDir)
		if err != nil || strings.HasPrefix(rel, "..") || rel == "." {
			continue // Escape containment check!
		}

		if isLink, err := safety.IsSymlink(fullDir); err == nil && isLink {
			continue // Symlink defense!
		}
		if safety.IsCoreBlacklisted(fullDir) {
			continue // Core blacklist defense!
		}

		info, err := os.Stat(fullDir)
		if err != nil || !info.IsDir() {
			continue
		}

		sz, _ := devfs.CalcDirSize(fullDir)
		if sz == 0 {
			continue
		}

		totalSize += sz
		foundDirs = append(foundDirs, fullDir)

		if maxAgeDays == 0 || info.ModTime().Before(cutoff) {
			eligibleBytes += sz
			hasEligible = true
		}
	}

	return totalSize, eligibleBytes, hasEligible, foundDirs
}
```

#### Functional Target Consolidation Pattern (`D-01`)
Cada editor emite até 4 alvos virtuais consolidados:
1. `editor://<editor-id>/chromium-cache` (`RiskLow`, category `editor-chromium-cache`)
2. `editor://<editor-id>/logs` (`RiskLow`, category `editor-logs`)
3. `editor://<editor-id>/extensions` (`RiskMedium`, category `editor-extensions`)
4. `editor://<editor-id>/workspace-backups` (`RiskMedium`, category `editor-workspace-backups`)

Se o processo do editor estiver ativo ou incerto (`D-06`):
```go
if isBlocked {
    target.ReviewOnly = true
    target.ReviewReason = domain.ReviewReasonActiveProcess
    target.EligibleBytes = 0
    target.TTLEligible = false
}
```

---

### 3.4. Claude CLI Cleaner: `internal/cleaner/ai/claude.go` (Modify)

- **Role:** Scanner e preservador de versões do Claude Code CLI e limpador de caches descartáveis.
- **Data Flow:** Descobre binários/pastas em `~/.local/share/claude/versions/*` e `~/.claude/local`, inspeciona symlinks de invocação com `filepath.EvalSymlinks`, aplica salvaguarda fail-closed se o symlink estiver ausente, quebrado ou ambíguo, e emite versões antigas como `RiskLow`. Avalia processos ativos via `pgrep -x claude`.
- **Closest Analog:** `internal/cleaner/xcode/xcode.go:95-111` (Process Guard) e `internal/cleaner/shell/shell.go:506-564` (resolução e agregação em diretórios de ferramentas).
- **Decisions Enforced:** `D-09`, `D-10`, `D-11`, `D-12`, `D-16`.

#### Active Symlink Canonical Resolution Pattern (`D-09`, `D-10`)
```go
type claudeVersionsResult struct {
	activeVersionPath string
	ambiguous         bool
	missingSymlink    bool
	brokenSymlink     bool
	candidateVersions []string
}

func resolveActiveClaudeVersion(home string) claudeVersionsResult {
	versionsDir := filepath.Join(home, ".local", "share", "claude", "versions")
	entries, err := os.ReadDir(versionsDir)
	if err != nil {
		return claudeVersionsResult{}
	}

	var candidates []string
	for _, e := range entries {
		p := filepath.Join(versionsDir, e.Name())
		candidates = append(candidates, p)
	}

	if len(candidates) == 0 {
		return claudeVersionsResult{}
	}

	// Symlinks that may reference the active version
	symlinkCandidates := []string{
		filepath.Join(home, ".local", "bin", "claude"),
		filepath.Join(home, ".local", "share", "claude", "current"),
		filepath.Join(home, ".local", "share", "claude", "versions", "current"),
		filepath.Join(home, ".local", "share", "claude", "claude"),
	}

	resolvedTargets := make(map[string]bool)
	var activePath string
	foundSymlinks := 0

	for _, link := range symlinkCandidates {
		isLink, err := safety.IsSymlink(link)
		if err != nil || !isLink {
			continue
		}
		foundSymlinks++

		dest, err := filepath.EvalSymlinks(link)
		if err != nil {
			// Broken symlink detected -> FAIL CLOSED!
			return claudeVersionsResult{
				brokenSymlink:     true,
				candidateVersions: candidates,
			}
		}

		cleanDest := filepath.Clean(dest)
		resolvedTargets[cleanDest] = true
		activePath = cleanDest
	}

	// Fail-closed checks
	if foundSymlinks == 0 {
		return claudeVersionsResult{
			missingSymlink:    true,
			candidateVersions: candidates,
		}
	}

	if len(resolvedTargets) > 1 {
		// Conflicting symlinks pointing to different destinations -> Ambiguous -> FAIL CLOSED!
		return claudeVersionsResult{
			ambiguous:         true,
			candidateVersions: candidates,
		}
	}

	return claudeVersionsResult{
		activeVersionPath: activePath,
		candidateVersions: candidates,
	}
}
```

#### Fail-Closed Version Targeting Logic (`D-10`, `D-12`)
```go
func scanClaudeVersions(ctx context.Context, exec cleaner.CommandExecutor, home string, rules domain.EcosystemRules) []domain.Target {
	res := resolveActiveClaudeVersion(home)
	if len(res.candidateVersions) == 0 {
		return nil
	}

	cutoff := time.Now().AddDate(0, 0, -rules.MaxAgeDays)
	var targets []domain.Target

	running, uncertain := checkProcessStatus(ctx, exec, "claude")
	processBlocked := running || uncertain

	for _, verPath := range res.candidateVersions {
		info, err := os.Stat(verPath)
		if err != nil {
			continue
		}

		size, _ := devfs.CalcDirSize(verPath)
		if size == 0 {
			continue
		}

		hash := sha256.Sum256([]byte(verPath))
		id := fmt.Sprintf("%x", hash[:6])
		verName := filepath.Base(verPath)

		// Check if this is the confirmed active version
		if !res.ambiguous && !res.missingSymlink && !res.brokenSymlink && res.activeVersionPath != "" {
			cleanVer := filepath.Clean(verPath)
			if cleanVer == res.activeVersionPath || strings.HasPrefix(res.activeVersionPath, cleanVer) {
				// PROTECTED: Never emit active version as a candidate for deletion!
				continue
			}
		}

		target := domain.Target{
			ID:           id,
			Name:         "Claude CLI version: " + verName,
			Path:         verPath,
			SizeBytes:    size,
			Risk:         domain.RiskLow, // D-12: Inactive versions are RiskLow
			LastAccessed: info.ModTime(),
			Category:     "ai-claude-versions",
		}

		// Apply fail-closed protections
		if processBlocked {
			target.ReviewOnly = true
			target.ReviewReason = domain.ReviewReasonActiveProcess
			target.EligibleBytes = 0
			target.TTLEligible = false
		} else if res.missingSymlink {
			target.ReviewOnly = true
			target.ReviewReason = domain.ReviewReasonActiveVersion
			target.EligibleBytes = 0
			target.TTLEligible = false
		} else if res.brokenSymlink || res.ambiguous {
			target.ReviewOnly = true
			target.ReviewReason = domain.ReviewReasonAmbiguous
			target.EligibleBytes = 0
			target.TTLEligible = false
		} else {
			// Confirmed inactive version
			if rules.MaxAgeDays == 0 || info.ModTime().Before(cutoff) {
				target.TTLEligible = true
				target.EligibleBytes = size
			}
		}

		targets = append(targets, target)
	}

	return targets
}
```

---

### 3.5. Engine Registration: `internal/engine/register.go` (Modify)

- **Role:** Registrador e bootstrapper de cleaners no motor do DevClean.
- **Data Flow:** Propaga o `cleaner.CommandExecutor` para o construtor `ai.NewWithExecutor`.
- **Closest Analog:** `internal/engine/register.go:52-59`.
- **Decisions Enforced:** `D-05`.

#### Adaptation Pattern
```go
// In RegisterDefaultCleanersWithExecutor:
eng.Register(ai.NewWithExecutor(exec, effectivePaths))
```

---

## 4. Cross-Cutting Architectural Patterns

### 4.1. Process Guard Fail-Closed & Isolated Editor Blocking
- **pgrep -x `<name>`:** Busca estrita pelo nome exato do processo.
- **Matriz de Resolução:**
  - `exit 0` com PID: **Processo Ativo** -> `ReviewOnly = true`, `ReviewReason = domain.ReviewReasonActiveProcess`, `EligibleBytes = 0`.
  - `exit 1`: **Processo Inativo** -> Elegível para deleção conforme TTL.
  - Qualquer outro erro (comando não encontrado, permissão, etc.): **Estado Incerto** -> **FAIL CLOSED** (`ReviewOnly = true`).
- **Isolamento por Ferramenta (`D-06`):** Se `Code` estiver ativo, apenas os alvos de `Code` recebem `ReviewOnly`. `Cursor`, `Antigravity`, `Windsurf`, `Zed` e outros editores permanecem liberados se seus processos estiverem inativos.

### 4.2. Inactive Version Resolution vs Active Preservation
- O DevClean preserva incondicionalmente a integridade do ambiente do desenvolvedor:
  - Uma versão ativa identificada via symlink válido é **completamente omitida** da lista de candidatos a expurgo.
  - Symlinks ambíguos ou quebrados acionam `ReviewReasonAmbiguous` em todas as versões encontradas com `EligibleBytes = 0`, reportando `SizeBytes` real sem risco de remoção inadvertida.

### 4.3. Surgical Workspace Backup Invariant (`D-13`)
- `User/workspaceStorage/*/state.vscdb`: **NUNCA TOCADO**.
- `User/workspaceStorage/*/workspace.json`: **NUNCA TOCADO**.
- `User/workspaceStorage/*/state.vscdb.backup`: Único arquivo elegível para deleção.

### 4.4. Read-Only `.obsolete` Metadata Integrity (`D-14`)
- `.obsolete` é lido como `map[string]bool`. O arquivo `.obsolete` **NUNCA É APAGADO OU TRUNCADO**.
- Apenas entradas cujo valor seja estritamente `true` são processadas.
- Qualquer caminho com `..`, caminhos absolutos ou que resolvam para symlinks são ignorados sumariamente.

### 4.5. Virtual Composite Editor Targets (`editor://`)
- Como alvos virtuais contêm `://`, `t.IsVirtual()` retorna `true`.
- São imunes à deduplicação de diretório em `internal/engine/ownership.go`.
- No método `Clean()`, o DevClean reconhece o esquema `editor://`, revalida se o processo do editor está ativo e deleta cirurgicamente apenas as folhas associadas àquela categoria.

### 4.6. Pre-Deletion Revalidation in Mutation Lifecycle (`D-07`)
- No início de `Clean()`, antes de invocar remoções físicas de arquivos ou pastas, o cleaner executa uma nova verificação de processos via `pgrep -x`. Se um editor foi iniciado pelo usuário entre o momento do scan e a execução da limpeza, a deleção física de seus arquivos é abortada.

---

## 5. Test Pattern Mapping & Hermetic Verification

### 5.1. Test Helper Architecture
Todas as suítes de testes herméticos utilizam `t.TempDir()`, sem acesso a caminhos reais do usuário:
```go
type mockExecutor struct {
	runningProcesses   map[string]bool
	uncertainProcesses map[string]bool
}

func (m *mockExecutor) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	if name == "pgrep" && len(args) == 2 && args[0] == "-x" {
		proc := args[1]
		if m.uncertainProcesses[proc] {
			return nil, errors.New("pgrep: command failed with internal error")
		}
		if m.runningProcesses[proc] {
			return []byte("12345\n"), nil
		}
		return nil, &MockExitError{Code: 1}
	}
	return nil, errors.New("unsupported command")
}

func (m *mockExecutor) LookPath(file string) (string, error) {
	return "/usr/bin/" + file, nil
}
```

### 5.2. Requirements to Test Patterns Mapping

| Req | Test Function | Test File | Core Pattern Verified |
|---|---|---|---|
| **AI-01** | `TestEditors_8Editors_Presence` | `internal/cleaner/ai/editors_test.go` | Descoberta correta de folhas Chromium/Electron em todos os 8 editores. |
| **AI-01** | `TestEditors_FunctionalGrouping` | `internal/cleaner/ai/editors_test.go` | Emissão de 3-4 alvos consolidados (`editor://`) por editor. |
| **AI-01** | `TestEditors_ProcessGuard_IsolatedBlocking` | `internal/cleaner/ai/editors_test.go` | Editor aberto bloqueia apenas seus alvos com `ReviewOnly: true`, liberando outros editores. |
| **AI-01** | `TestEditors_ProcessGuard_FailClosed` | `internal/cleaner/ai/editors_test.go` | Erro desconhecido de `pgrep` aplica `ReviewOnly` a todos os alvos do editor. |
| **AI-02** | `TestEditors_WorkspaceStorage_SurgicalBackup` | `internal/cleaner/ai/editors_test.go` | Apenas `state.vscdb.backup` é direcionado; `state.vscdb` e `workspace.json` intocados. |
| **AI-02** | `TestEditors_ObsoleteExtensions_Safety` | `internal/cleaner/ai/editors_test.go` | `.obsolete` lido como metadado, proteção de ativas, rejeição de path traversal (`..`) e symlinks. |
| **AI-03** | `TestClaude_ActiveVersion_Preserved` | `internal/cleaner/ai/claude_test.go` | Versão ativa apontada por symlink é omitida; versões antigas viram alvo `RiskLow`. |
| **AI-03** | `TestClaude_MissingSymlink_FailClosed` | `internal/cleaner/ai/claude_test.go` | Sem symlink -> todas as versões viram `ReviewOnly` com razão `active_version`. |
| **AI-03** | `TestClaude_BrokenOrAmbiguousSymlink_FailClosed` | `internal/cleaner/ai/claude_test.go` | Symlink quebrado ou múltiplos destinos -> `ReviewOnly` com razão `ambiguous_state`. |
| **AI-03** | `TestClaude_ProcessGuard_ActiveCLI` | `internal/cleaner/ai/claude_test.go` | `pgrep -x claude` ativo bloqueia alvos com `ReviewReasonActiveProcess`. |
| **VAL-01** | `TestAICleaner_Clean_PreDeletionRevalidation` | `internal/cleaner/ai/ai_test.go` | Editor aberto após o scan aborta deleção física no `Clean()`. |
| **VAL-01** | `TestAICleaner_Clean_VirtualEditorTarget` | `internal/cleaner/ai/ai_test.go` | Expurgo seguro de alvos virtuais `editor://` desfazendo apenas as folhas autorizadas. |
| **VAL-02** | `TestCoordinator_DryRun_ZeroMutation_Phase09` | `internal/deleter/policy_test.go` | Canário de mutação zero sob DryRun cobrindo alvos de editores e versões do Claude CLI. |

---

## 6. Zero-Mutation Canary Specification for Phase 09

Para integrar `TestCoordinator_DryRun_ZeroMutation_Phase09` em `internal/deleter/policy_test.go`:
- Cria sentinelas com hashes criptográficos SHA-256 e timestamps fixados:
  1. `Code`: `editor://code/chromium-cache` apontando para folha física com arquivos canário.
  2. `Cursor`: `editor://cursor/workspace-backups` apontando para pasta com `state.vscdb` (primário) e `state.vscdb.backup`.
  3. `Windsurf`: `editor://windsurf/extensions` com pasta de extensão ativa e `.obsolete`.
  4. `Claude CLI`: `~/.local/share/claude/versions/1.0.0` (inativa) e `2.0.0` (ativa com symlink).
  5. `Claude CLI ReviewOnly`: com symlink quebrado apontando para versão inexistente.
- Executa `coord.Delete(ctx, targets, deleter.Options{DryRun: true})`.
- Verifica:
  - `state.vscdb` e `.obsolete` intocados e idênticos.
  - Canários físicos com SHA-256, tamanho e `ModTime` inalterados.
  - Diretório de lixeira (`trash`) 100% vazio e sem manifests.
  - `history.jsonl` vazio ou inexistente.

---

## PATTERN MAPPING COMPLETE

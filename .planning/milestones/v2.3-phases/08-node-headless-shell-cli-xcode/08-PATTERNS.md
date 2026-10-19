# Phase 08: Node, Headless, Shell/CLI & Xcode - Pattern Mapping

**Phase:** 08-node-headless-shell-cli-xcode  
**Milestone:** v2.3 Total Parity with Mole (tw93/mole)  
**Date:** 2026-10-15  
**Status:** Approved for Implementation  
**Files Analyzed:** 15  
**Analogs Found:** 15 / 15  

---

## 1. Overview & Pattern Mapping Summary

Este documento estabelece o mapeamento estrutural entre os arquivos a serem criados e modificados na **Fase 08** e os análogos consolidados na base de código do DevClean (`internal/cleaner/node/node.go`, `internal/cleaner/pnpm/pnpm.go`, `internal/cleaner/xcode/xcode.go`, `internal/cleaner/homebrew/homebrew.go`, `internal/cleaner/golang/golang.go`, `internal/cleaner/executor.go`, `internal/safety/blacklist.go`, `internal/safety/symlink.go`, etc.).

O objetivo desta fase é consolidar a paridade de inventário do DevClean contra ferramentas modernas de desenvolvimento, cobrindo três ecossistemas e seus respectivos requisitos:
1. **Node & Headless Automation Caches (`DEV-01`):** Descoberta por presença em disco de subdiretórios granulares de cache do npm (`~/.npm/_cacache`, `~/.npm/_npx`, `~/.npm/_logs`), Corepack (`~/.cache/node/corepack`, `~/Library/Caches/node/corepack`), caches pesados de automação headless (Puppeteer `~/.cache/puppeteer`, Playwright `~/Library/Caches/ms-playwright` / `~/.cache/ms-playwright`, Cypress `~/Library/Caches/Cypress` / `~/.cache/Cypress`) e cache de metadados HTTP do pnpm (`~/Library/Caches/pnpm`, `~/.cache/pnpm`), mesmo sem executáveis no PATH.
2. **Shell & CLI Tools Cleaner (`DEV-02`):** Novo cleaner sob o ecossistema `"shell"` (`internal/cleaner/shell`), consolidando `mise` (`mise cache` e `mise downloads`), GitHub CLI (`gh`), `pre-commit` (RiskMedium), `Oh My Zsh` (`~/.oh-my-zsh/cache`) e agregação consolidada de dumps `~/.zcompdump*` sem jamais ler, alterar ou violar configurações de shell (`.zshrc`, `.bashrc`) ou arquivos de histórico (`.zsh_history`, `.bash_history`).
3. **Xcode, Simulators & Process Guard (`DEV-03`):** Expansão do cleaner de Xcode para cobrir `CoreSimulator/Caches`, `CoreSimulator/Temp`, `iOS DeviceSupport`, `watchOS DeviceSupport` e `Archives` (RiskHigh quando fechado). Implementação de detecção fail-closed de processos ativos (`pgrep -x Xcode`, `pgrep -x Simulator`) via interface injetável `cleaner.CommandExecutor` sem CGO: se processos estiverem ativos ou em estado incerto, os alvos recebem `ReviewOnly = true`, `ReviewReason = domain.ReviewReasonActiveProcess`, `EligibleBytes = 0` e `TTLEligible = false`.
4. **Garantias Herméticas de Segurança (`VAL-01`, `VAL-02`):** Deleção direta em disco com verificação prévia de `safety.IsCoreBlacklisted` e `safety.IsSymlink`, suites com `t.TempDir()`, sentinelas, verificação de isolamento de links simbólicos e teste canário de DryRun com mutação zero.

---

## 2. File Classification Table

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `internal/cleaner/node/node.go` | service / cleaner | batch / disk-scan | `internal/cleaner/node/node.go` | exact |
| `internal/cleaner/pnpm/pnpm.go` | service / cleaner | hybrid (CLI + disk-scan) | `internal/cleaner/pnpm/pnpm.go`<br>`internal/cleaner/homebrew/homebrew.go` | exact |
| `internal/cleaner/shell/shell.go` | service / cleaner (new) | batch / disk-scan | `internal/cleaner/homebrew/homebrew.go`<br>`internal/cleaner/golang/golang.go` | high |
| `internal/cleaner/xcode/xcode.go` | service / cleaner | hybrid (disk-scan + process guard) | `internal/cleaner/xcode/xcode.go`<br>`internal/intelligence/git.go` | exact |
| `internal/engine/register.go` | registry / bootstrapper | composition | `internal/engine/register.go` | exact |
| `internal/config/config.go` | model / config | declarative | `internal/config/config.go` | exact |
| `internal/config/defaults.go` | configuration | declarative | `internal/config/defaults.go` | exact |
| `internal/config/validate.go` | validator | transform | `internal/config/validate.go` | exact |
| `internal/config/save.go` | serializer | template | `internal/config/save.go` | exact |
| `internal/engine/engine.go` | orchestrator | routing | `internal/engine/engine.go` | exact |
| `internal/tui/config.go` | component / tui | event-driven | `internal/tui/config.go` | exact |
| `internal/cleaner/node/node_test.go` | unit test | fixture-based | `internal/cleaner/node/node_test.go` | exact |
| `internal/cleaner/pnpm/pnpm_test.go` | unit test | mock executor + disk fixture | `internal/cleaner/pnpm/pnpm_test.go` | exact |
| `internal/cleaner/shell/shell_test.go` | unit test (new) | mock executor + disk fixture | `internal/cleaner/homebrew/homebrew_test.go`<br>`internal/cleaner/node/node_test.go` | high |
| `internal/cleaner/xcode/xcode_test.go` | unit test | mock executor + process guard | `internal/cleaner/xcode/xcode_test.go` | exact |

---

## 3. Pattern Assignments & Code Excerpts

### 3.1. Node Cleaner: `internal/cleaner/node/node.go` (Modify)

- **Role:** Cleaner do ecossistema Node.js responsável por inventariar alvos de projeto e caches globais.
- **Data Flow:** Inspeciona caminhos globais conhecidos e variáveis de ambiente, calcula tamanhos com `devfs.CalcDirSize`, gera identificadores hexadecimais SHA-256 e executa limpeza direta via `os.RemoveAll` após validações de segurança.
- **Closest Analog:** `internal/cleaner/node/node.go:154-203` e `internal/cleaner/golang/golang.go:77-118`.
- **Decisions Enforced:** `D-01`, `D-02`, `D-03`, `D-05`.

#### Existing Code Excerpt (`internal/cleaner/node/node.go:154-203`)
```go
func (c *nodeCleaner) scanGlobalCaches(rules domain.EcosystemRules) []domain.Target {
	home, _ := os.UserHomeDir()
	if home == "" {
		return nil
	}

	var targets []domain.Target
	cutoff := time.Now().AddDate(0, 0, -rules.MaxAgeDays)

	caches := []struct {
		path     string
		category string
		name     string
	}{
		{filepath.Join(home, ".npm", "_cacache"), "npm-cache", "npm cache"},
		{yarnCachePath(home), "yarn-cache", "yarn cache"},
	}

	for _, cache := range caches {
		info, err := os.Stat(cache.path)
		if err != nil {
			continue
		}

		size, _ := devfs.CalcDirSize(cache.path)
		hash := sha256.Sum256([]byte(cache.path))
		id := fmt.Sprintf("%x", hash[:6])

		ttlEligible := false
		eligibleBytes := int64(0)
		if rules.MaxAgeDays == 0 || info.ModTime().Before(cutoff) {
			ttlEligible = true
			eligibleBytes = size
		}

		targets = append(targets, domain.Target{
			ID:            id,
			Name:          cache.name,
			Path:          cache.path,
			SizeBytes:     size,
			EligibleBytes: eligibleBytes,
			TTLEligible:   ttlEligible,
			Risk:          domain.RiskLow,
			LastAccessed:  info.ModTime(),
			Category:      cache.category,
		})
	}

	return targets
}
```

#### Existing Clean Method (`internal/cleaner/node/node.go:139-146`)
```go
func (c *nodeCleaner) Clean(_ context.Context, targets []domain.Target) error {
	for _, t := range targets {
		if err := os.RemoveAll(t.Path); err != nil {
			return err
		}
	}
	return nil
}
```

#### Concrete Adaptation Pattern for Phase 08
1. **Granular Caches Inventory Definition (`D-01`, `D-02`, `D-03`):**
```go
type globalCacheDef struct {
	category string
	name     string
	risk     domain.RiskLevel
	paths    []string
}

func getGlobalCacheCandidates(home string) []globalCacheDef {
	darwinCaches := filepath.Join(home, "Library", "Caches")
	xdgCaches := filepath.Join(home, ".cache")

	return []globalCacheDef{
		{
			category: "npm-cache",
			name:     "npm cache",
			risk:     domain.RiskLow,
			paths:    []string{filepath.Join(home, ".npm", "_cacache")},
		},
		{
			category: "npx-cache",
			name:     "npx cache",
			risk:     domain.RiskLow,
			paths:    []string{filepath.Join(home, ".npm", "_npx")},
		},
		{
			category: "npm-logs",
			name:     "npm logs",
			risk:     domain.RiskLow,
			paths:    []string{filepath.Join(home, ".npm", "_logs")},
		},
		{
			category: "yarn-cache",
			name:     "yarn cache",
			risk:     domain.RiskLow,
			paths:    []string{yarnCachePath(home)},
		},
		{
			category: "corepack-cache",
			name:     "Corepack cache",
			risk:     domain.RiskLow,
			paths: []string{
				filepath.Join(darwinCaches, "node", "corepack"),
				filepath.Join(xdgCaches, "node", "corepack"),
			},
		},
		{
			category: "node-puppeteer",
			name:     "Puppeteer browser cache",
			risk:     domain.RiskMedium,
			paths: []string{
				os.Getenv("PUPPETEER_CACHE_DIR"),
				filepath.Join(xdgCaches, "puppeteer"),
				filepath.Join(home, ".puppeteer"),
			},
		},
		{
			category: "node-playwright",
			name:     "Playwright browser cache",
			risk:     domain.RiskMedium,
			paths: []string{
				os.Getenv("PLAYWRIGHT_BROWSERS_PATH"),
				filepath.Join(darwinCaches, "ms-playwright"),
				filepath.Join(xdgCaches, "ms-playwright"),
			},
		},
		{
			category: "node-cypress",
			name:     "Cypress binary cache",
			risk:     domain.RiskLow,
			paths: []string{
				os.Getenv("CYPRESS_CACHE_FOLDER"),
				filepath.Join(darwinCaches, "Cypress"),
				filepath.Join(xdgCaches, "Cypress"),
			},
		},
	}
}
```

2. **De-duplicating Path Scan Loop:**
```go
func (c *nodeCleaner) scanGlobalCaches(rules domain.EcosystemRules) []domain.Target {
	home, _ := os.UserHomeDir()
	if home == "" {
		return nil
	}

	var targets []domain.Target
	cutoff := time.Now().AddDate(0, 0, -rules.MaxAgeDays)
	seenPaths := make(map[string]bool)

	for _, def := range getGlobalCacheCandidates(home) {
		for _, path := range def.paths {
			if path == "" || seenPaths[path] {
				continue
			}
			seenPaths[path] = true

			info, err := os.Stat(path)
			if err != nil || !info.IsDir() {
				continue
			}

			size, _ := devfs.CalcDirSize(path)
			if size == 0 {
				continue
			}

			hash := sha256.Sum256([]byte(path))
			id := fmt.Sprintf("%x", hash[:6])

			ttlEligible := false
			eligibleBytes := int64(0)
			if rules.MaxAgeDays == 0 || info.ModTime().Before(cutoff) {
				ttlEligible = true
				eligibleBytes = size
			}

			targets = append(targets, domain.Target{
				ID:            id,
				Name:          def.name,
				Path:          path,
				SizeBytes:     size,
				EligibleBytes: eligibleBytes,
				TTLEligible:   ttlEligible,
				Risk:          def.risk,
				LastAccessed:  info.ModTime(),
				Category:      def.category,
			})
		}
	}
	return targets
}
```

3. **Safe Clean Execution (`D-05`):**
```go
func (c *nodeCleaner) Clean(_ context.Context, targets []domain.Target) error {
	for _, t := range targets {
		if safety.IsCoreBlacklisted(t.Path) {
			continue
		}
		if isLink, err := safety.IsSymlink(t.Path); err == nil && isLink {
			continue // Refuse to traverse symlinks
		}
		if err := os.RemoveAll(t.Path); err != nil {
			return err
		}
	}
	return nil
}
```

---

### 3.2. pnpm Cleaner: `internal/cleaner/pnpm/pnpm.go` (Modify)

- **Role:** Cleaner do ecossistema pnpm gerenciando stores virtuais e cache físico de metadados HTTP.
- **Data Flow:** Verifica presença de CLI e diretórios de cache/store em `IsInstalled()`. Em `Scan()`, consulta a store via CLI e varre o diretório físico de metadados HTTP (`~/Library/Caches/pnpm` ou `~/.cache/pnpm`).
- **Closest Analog:** `internal/cleaner/pnpm/pnpm.go:33-83` e `internal/cleaner/homebrew/homebrew.go:37-55`.
- **Decisions Enforced:** `D-04`.

#### Existing Code Excerpt (`internal/cleaner/pnpm/pnpm.go:33-52`)
```go
func (c *pnpmCleaner) IsInstalled() bool {
	if _, err := c.exec.LookPath("pnpm"); err == nil {
		return true
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return false
	}
	candidates := []string{
		filepath.Join(home, "Library", "pnpm", "store"),
		filepath.Join(home, ".local", "share", "pnpm", "store"),
		filepath.Join(home, ".pnpm-store"),
	}
	for _, p := range candidates {
		if devfs.DirExists(p) {
			return true
		}
	}
	return false
}
```

#### Concrete Adaptation Pattern for Phase 08
1. **Metadata Cache Detection in `IsInstalled()` (`D-04`):**
```go
func (c *pnpmCleaner) IsInstalled() bool {
	if _, err := c.exec.LookPath("pnpm"); err == nil {
		return true
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return false
	}
	candidates := []string{
		filepath.Join(home, "Library", "pnpm", "store"),
		filepath.Join(home, ".local", "share", "pnpm", "store"),
		filepath.Join(home, ".pnpm-store"),
		filepath.Join(home, "Library", "Caches", "pnpm"),
		filepath.Join(home, ".cache", "pnpm"),
	}
	for _, p := range candidates {
		if devfs.DirExists(p) {
			return true
		}
	}
	return false
}
```

2. **Metadata Cache Scanning in `Scan()`:**
```go
func (c *pnpmCleaner) Scan(ctx context.Context, rules domain.EcosystemRules) ([]domain.Target, error) {
	var targets []domain.Target
	cutoff := time.Now().AddDate(0, 0, -rules.MaxAgeDays)

	// 1. Virtual pnpm store prune target
	out, err := c.exec.Output(ctx, "pnpm", "store", "path")
	if err == nil {
		storePath := strings.TrimSpace(string(out))
		if storePath != "" {
			size, _ := devfs.CalcDirSize(storePath)
			if size > 0 {
				targets = append(targets, domain.Target{
					ID:            "pnpm-store-prune",
					Name:          "pnpm store (unreferenced packages)",
					Path:          "pnpm://store/prune",
					SizeBytes:     size,
					EligibleBytes: size,
					TTLEligible:   true,
					Risk:          domain.RiskLow,
					Category:      "virtual-pnpm-store",
				})
			}
		}
	}

	// 2. Physical HTTP metadata cache (D-04)
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		candidates := []string{
			filepath.Join(home, "Library", "Caches", "pnpm"),
			filepath.Join(home, ".cache", "pnpm"),
		}
		seen := make(map[string]bool)
		for _, cacheDir := range candidates {
			if seen[cacheDir] || !devfs.DirExists(cacheDir) {
				continue
			}
			seen[cacheDir] = true

			info, err := os.Stat(cacheDir)
			if err != nil {
				continue
			}
			size, _ := devfs.CalcDirSize(cacheDir)
			if size == 0 {
				continue
			}

			ttlEligible := false
			eligibleBytes := int64(0)
			if rules.MaxAgeDays == 0 || info.ModTime().Before(cutoff) {
				ttlEligible = true
				eligibleBytes = size
			}

			sum := sha256.Sum256([]byte(cacheDir))
			targets = append(targets, domain.Target{
				ID:            fmt.Sprintf("%x", sum[:6]),
				Name:          "pnpm metadata cache",
				Path:          cacheDir,
				SizeBytes:     size,
				EligibleBytes: eligibleBytes,
				TTLEligible:   ttlEligible,
				Risk:          domain.RiskLow,
				LastAccessed:  info.ModTime(),
				Category:      "pnpm-cache",
			})
		}
	}

	return targets, nil
}
```

3. **Safe Clean Execution:**
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

---

### 3.3. Shell Cleaner: `internal/cleaner/shell/shell.go` (Create New)

- **Role:** Novo cleaner do ecossistema `"shell"` unificando ferramentas CLI (`mise`, `gh`, `pre-commit`, `Oh My Zsh` e `~/.zcompdump*`).
- **Data Flow:** Descobre caminhos de cache e downloads sem tocar em arquivos de inicialização (`.zshrc`, `.bashrc`) ou histórico (`.zsh_history`). Agrega múltiplos dumps de autocompletar do Zsh em um alvo consolidado virtual `shell://zsh/compdump`.
- **Closest Analog:** `internal/cleaner/homebrew/homebrew.go` e `internal/cleaner/golang/golang.go`.
- **Decisions Enforced:** `D-06`, `D-07`, `D-08`, `D-09`, `D-10`.

#### Constructor Pattern (`internal/cleaner/homebrew/homebrew.go:17-29`)
```go
type shellCleaner struct {
	exec cleaner.CommandExecutor
}

func New() domain.Cleaner {
	return NewWithExecutor(&cleaner.DefaultExecutor{})
}

func NewWithExecutor(exec cleaner.CommandExecutor) domain.Cleaner {
	return &shellCleaner{exec: exec}
}

func (c *shellCleaner) Name() string {
	return "shell"
}
```

#### `IsInstalled()` Hybrid Disk & PATH Detection (`D-10`)
```go
func (c *shellCleaner) IsInstalled() bool {
	// 1. Check binaries in PATH
	for _, bin := range []string{"mise", "gh", "pre-commit", "zsh"} {
		if _, err := c.exec.LookPath(bin); err == nil {
			return true
		}
	}

	// 2. Check disk presence
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return false
	}

	candidates := []string{
		filepath.Join(home, "Library", "Caches", "mise"),
		filepath.Join(home, ".cache", "mise"),
		filepath.Join(home, ".local", "share", "mise", "downloads"),
		filepath.Join(home, "Library", "Caches", "gh"),
		filepath.Join(home, ".cache", "gh"),
		filepath.Join(home, ".cache", "pre-commit"),
		filepath.Join(home, ".oh-my-zsh", "cache"),
	}
	for _, p := range candidates {
		if devfs.DirExists(p) {
			return true
		}
	}

	// Check if any .zcompdump* exists in HOME
	if matches, _ := filepath.Glob(filepath.Join(home, ".zcompdump*")); len(matches) > 0 {
		return true
	}

	return false
}
```

#### Zsh Completion Dumps Aggregation Pattern (`D-08`)
```go
func scanZshCompdumps(home string, rules domain.EcosystemRules) *domain.Target {
	var files []string

	// Check $HOME
	if matches, err := filepath.Glob(filepath.Join(home, ".zcompdump*")); err == nil {
		files = append(files, matches...)
	}

	// Check $ZDOTDIR if set and distinct from $HOME
	if zdotdir := os.Getenv("ZDOTDIR"); zdotdir != "" && zdotdir != home {
		if matches, err := filepath.Glob(filepath.Join(zdotdir, ".zcompdump*")); err == nil {
			files = append(files, matches...)
		}
	}

	if len(files) == 0 {
		return nil
	}

	var totalSize int64
	var mostRecent time.Time
	cutoff := time.Now().AddDate(0, 0, -rules.MaxAgeDays)

	for _, f := range files {
		info, err := os.Stat(f)
		if err != nil || info.IsDir() {
			continue
		}
		totalSize += info.Size()
		if info.ModTime().After(mostRecent) {
			mostRecent = info.ModTime()
		}
	}

	if totalSize == 0 {
		return nil
	}

	ttlEligible := false
	eligibleBytes := int64(0)
	if rules.MaxAgeDays == 0 || mostRecent.Before(cutoff) {
		ttlEligible = true
		eligibleBytes = totalSize
	}

	return &domain.Target{
		ID:            "shell-zsh-compdump",
		Name:          "Zsh completion cache",
		Path:          "shell://zsh/compdump",
		SizeBytes:     totalSize,
		EligibleBytes: eligibleBytes,
		TTLEligible:   ttlEligible,
		Risk:          domain.RiskLow,
		LastAccessed:  mostRecent,
		Category:      "shell-zsh-compdump",
	}
}
```

#### Safe Clean Method Enforcing Non-Interference Guarantee
```go
func (c *shellCleaner) Clean(_ context.Context, targets []domain.Target) error {
	home, _ := os.UserHomeDir()

	for _, t := range targets {
		if t.Path == "shell://zsh/compdump" {
			// Aggregated zcompdump cleanup
			patterns := []string{filepath.Join(home, ".zcompdump*")}
			if zdotdir := os.Getenv("ZDOTDIR"); zdotdir != "" && zdotdir != home {
				patterns = append(patterns, filepath.Join(zdotdir, ".zcompdump*"))
			}
			for _, pat := range patterns {
				matches, _ := filepath.Glob(pat)
				for _, f := range matches {
					// Guard against blacklisted paths
					if safety.IsCoreBlacklisted(f) {
						continue
					}
					_ = os.Remove(f)
				}
			}
			continue
		}

		// Physical directory deletion with strict safety
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

---

### 3.4. Xcode Cleaner: `internal/cleaner/xcode/xcode.go` (Modify)

- **Role:** Cleaner de ferramentas Xcode e Simuladores iOS.
- **Data Flow:** Executa verificação fail-closed de processos ativos (`pgrep -x Xcode`, `pgrep -x Simulator`) via `cleaner.CommandExecutor`. Varre alvos granulares e atribui `ReviewOnly: true` sob processos ativos.
- **Closest Analog:** `internal/cleaner/xcode/xcode.go:86-169` e `internal/intelligence/git.go:10-18`.
- **Decisions Enforced:** `D-11`, `D-12`, `D-13`, `D-14`.

#### Fail-Closed Process Detection Pattern (`D-11`, `D-12`)
```go
// checkProcessStatus returns running=true if pgrep finds matching processes.
// If pgrep fails with exit code 1, running=false, uncertain=false.
// If pgrep fails with any other error, uncertain=true (FAIL CLOSED!).
func checkProcessStatus(ctx context.Context, exec cleaner.CommandExecutor, processName string) (running bool, uncertain bool) {
	out, err := exec.Output(ctx, "pgrep", "-x", processName)
	if err == nil && len(strings.TrimSpace(string(out))) > 0 {
		return true, false
	}

	var exitErr interface{ ExitCode() int }
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return false, false // Cleanly not running
	}

	// Any other error (missing pgrep, permission denied, timeout, exit code > 1) is UNCERTAIN -> FAIL CLOSED!
	return false, true
}
```

#### Expanded Target Definitions (`D-13`, `D-14`)
```go
type xcodeTargetDef struct {
	category    string
	name        string
	path        string
	risk        domain.RiskLevel
	simRelated  bool // affected if Simulator.app is active
}

func (c *xcodeCleaner) getExpandedTargetDefs() []xcodeTargetDef {
	home, _ := os.UserHomeDir()
	if home == "" {
		return nil
	}
	devDir := filepath.Join(home, "Library", "Developer")
	xcodeDir := filepath.Join(devDir, "Xcode")
	coreSimDir := filepath.Join(devDir, "CoreSimulator")

	return []xcodeTargetDef{
		{
			category:   "xcode-coresimulator-caches",
			name:       "CoreSimulator Caches",
			path:       filepath.Join(coreSimDir, "Caches"),
			risk:       domain.RiskLow,
			simRelated: true,
		},
		{
			category:   "xcode-coresimulator-temp",
			name:       "CoreSimulator Temp",
			path:       filepath.Join(coreSimDir, "Temp"),
			risk:       domain.RiskLow,
			simRelated: true,
		},
		{
			category:   "xcode-ios-devicesupport",
			name:       "Xcode iOS DeviceSupport",
			path:       filepath.Join(xcodeDir, "iOS DeviceSupport"),
			risk:       domain.RiskLow,
			simRelated: false,
		},
		{
			category:   "xcode-watchos-devicesupport",
			name:       "Xcode watchOS DeviceSupport",
			path:       filepath.Join(xcodeDir, "watchOS DeviceSupport"),
			risk:       domain.RiskLow,
			simRelated: false,
		},
		{
			category:   "xcode-archives",
			name:       "Xcode Archives",
			path:       filepath.Join(xcodeDir, "Archives"),
			risk:       domain.RiskHigh, // D-13: RiskHigh when Xcode is closed
			simRelated: false,
		},
	}
}
```

#### Applying ReviewOnly Guard in `Scan()` (`D-11`)
```go
// Evaluate process state
xcodeRunning, xcodeUncertain := checkProcessStatus(ctx, c.exec, "Xcode")
simRunning, simUncertain := checkProcessStatus(ctx, c.exec, "Simulator")

xcodeBlocked := xcodeRunning || xcodeUncertain
simBlocked := simRunning || simUncertain

// Helper to apply review gate to target
applyReviewGate := func(t *domain.Target, blocked bool) {
	if blocked {
		t.ReviewOnly = true
		t.ReviewReason = domain.ReviewReasonActiveProcess
		t.EligibleBytes = 0
		t.TTLEligible = false
	}
}
```

---

### 3.5. Engine Registration & Configuration Ecosystem Wiring

- **Role:** Habilitar e orquestrar o ecossistema `"shell"` no motor e configurações do DevClean.
- **Data Flow:** Registra o cleaner no motor, propaga configurações do Viper e expõe opções na TUI.
- **Closest Analog:** Configurações existentes para `pnpm`, `xcode`, `homebrew`.
- **Decisions Enforced:** `D-06`.

#### 1. `internal/config/config.go`
```go
type EcosystemsConfig struct {
    // ... existing ecosystems
    Shell EcosystemConfig `mapstructure:"shell"`
}

type HooksConfig struct {
    // ... existing hooks
    Shell HookConfig `mapstructure:"shell"`
}
```

#### 2. `internal/config/defaults.go`
```go
// In EcosystemsConfig defaults:
Shell: EcosystemConfig{Enabled: true, MaxAgeDays: 30, FailMode: "continue", ExcludePaths: []string{}},

// In HooksConfig defaults:
Shell: HookConfig{Timeout: "30s"},
```

#### 3. `internal/config/validate.go`
```go
// In allowedCleanValues:
"shell": true,

// In hooks map:
"hooks.shell": c.Hooks.Shell,
```

#### 4. `internal/config/save.go`
```yaml
  # Shell and CLI tool caches (mise, gh, pre-commit, omz, zcompdump)
  shell:
    enabled: true
    max_age_days: 30
    fail_mode: continue
    exclude_paths: []
```

#### 5. `internal/engine/engine.go`
```go
case "shell":
    return e.config.Ecosystems.Shell
```

#### 6. `internal/engine/register.go`
```go
import "github.com/gtkpad/devclean/internal/cleaner/shell"

// In RegisterDefaultCleanersWithExecutor:
eng.Register(shell.NewWithExecutor(exec))
```

#### 7. `internal/tui/config.go`
```go
// In ecosystemMetas:
{Key: "shell", Name: "Shell & CLI", Description: "Caches do mise, GitHub CLI, pre-commit, Oh My Zsh e zcompdump"},

// In getEcosystemConfig:
case "shell":
    return &m.cfg.Ecosystems.Shell
```

---

## 4. Cross-Cutting Architectural Patterns

### 4.1. Path Resolution Across Platforms (Darwin vs Linux)
Ferramentas de desenvolvimento modernas adotam convenções divergentes no macOS (`~/Library/Caches/<Tool>`) e Linux (`~/.cache/<Tool>` ou `$XDG_CACHE_HOME`).
- **Resolução Padronizada:** A verificação deve testar candidatos em ordem:
  1. Variável de ambiente canônica da ferramenta (ex.: `PUPPETEER_CACHE_DIR`, `PLAYWRIGHT_BROWSERS_PATH`, `CYPRESS_CACHE_FOLDER`, `PRE_COMMIT_HOME`, `ZSH_CACHE_DIR`).
  2. Caminho específico de Darwin se `runtime.GOOS == "darwin"`.
  3. Caminho padrão XDG (`~/.cache/<Tool>`).
- Se a pasta existir (`os.Stat` com `info.IsDir() == true`), ela é incluída no inventário. O mapa `seenPaths[path]` evita duplicações.

### 4.2. Process Guard Fail-Closed Pattern
A segurança operacional durante a execução de IDEs ou compiladores pesados requer uma guarda atômica e confiável:
- **`pgrep -x <name>`:** Busca exata pelo nome do processo, sem falsos positivos parciais.
- **Tabela de Resolução de Estado:**
  - `exit 0` com stdout não-vazio: **Processo Ativo** -> `ReviewOnly = true`.
  - `exit 1` (sem matches): **Processo Inativo** -> Prossegue normalmente.
  - `exit > 1` ou qualquer erro não-`ExitCode(1)`: **Estado Incerto** -> `ReviewOnly = true` (**FAIL CLOSED**).

### 4.3. Aggregated Virtual Target Pattern
Arquivos efêmeros fragmentados (como dumps de inicialização do Zsh: `.zcompdump-host-5.9`, `.zcompdump-host-5.8`) poluem a listagem visual se exibidos individualmente.
- O cleaner agrega todos os matches de `filepath.Glob` sob um único alvo canônico:
  - `ID`: `"shell-zsh-compdump"`
  - `Path`: `"shell://zsh/compdump"` (esquema virtual imune à deduplicação de diretório em `ownership.go`)
  - `SizeBytes`: soma exata dos bytes de todos os arquivos encontrados.
  - `Clean()`: expande novamente o padrão e remove individualmente cada arquivo com `os.Remove`.

### 4.4. Direct Filesystem Deletion with Safety Validation
Em conformidade com `D-05`, nenhuma ferramenta externa (`npx`, `cypress`, `mise`, `gh`) deve ser invocada para realizar a limpeza em disco, garantindo operação 100% offline.
- Antes de invocar `os.RemoveAll(path)`:
  1. `safety.IsCoreBlacklisted(path)`: garante que caminhos protegidos (`.git`, `.ssh`, `.env`, `/`, `/etc`) nunca sejam excluídos.
  2. `safety.IsSymlink(path)`: garante que links simbólicos não sejam percorridos nem excluam conteúdos externos inadvertidamente.

### 4.5. ReviewOnly & TTLEligible Gate Alignment
Conforme estabelecido nas Fases 6 e 7:
- Se `ReviewOnly == true`:
  - `EligibleBytes` DEVE ser `0`.
  - `TTLEligible` DEVE ser `false`.
  - `ReviewReason` DEVE ser atribuído (ex.: `domain.ReviewReasonActiveProcess`).
  - `SizeBytes` DEVE reportar o volume real ocupado no disco.

---

## 5. Test Pattern Mapping & Hermetic Verification

### 5.1. Test Helper Architecture
Todos os testes de cleaner devem utilizar isolamento hermético:
```go
func setupTestHome(t *testing.T) string {
	t.Helper()
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	return tempHome
}
```

### 5.2. Mock Exit Error for Process Guard
Para permitir asserções determinísticas sobre o código de saída do `pgrep` sem CGO e sem invocar binários do sistema operacional:
```go
// MockExitError implements interface{ ExitCode() int } and error.
type MockExitError struct {
	Code int
	Msg  string
}

func (e *MockExitError) Error() string {
	if e.Msg != "" {
		return e.Msg
	}
	return fmt.Sprintf("exit status %d", e.Code)
}

func (e *MockExitError) ExitCode() int {
	return e.Code
}
```

### 5.3. Requirements to Test Patterns Mapping

| Req | Test Function | Test File | Core Pattern Verified |
|---|---|---|---|
| **DEV-01** | `TestNodeCleaner_Scan_GlobalCaches_AutomationAndNpm` | `internal/cleaner/node/node_test.go` | `_cacache`, `_npx`, `_logs`, `corepack`, `puppeteer`, `playwright`, `cypress` descobertos por presença em disco. |
| **DEV-01** | `TestNodeCleaner_RiskLevels_Automation` | `internal/cleaner/node/node_test.go` | `RiskMedium` para Puppeteer/Playwright e `RiskLow` para Cypress/npm/Corepack. |
| **DEV-01** | `TestPnpmCleaner_Scan_DiskMetadataCache` | `internal/cleaner/pnpm/pnpm_test.go` | Descoberta do cache HTTP de metadados do pnpm sem CLI no PATH. |
| **DEV-01** | `TestPnpmCleaner_IsInstalled_DiskPresence` | `internal/cleaner/pnpm/pnpm_test.go` | `IsInstalled() == true` quando cache de metadados existe no disco. |
| **DEV-02** | `TestShellCleaner_IsInstalled` | `internal/cleaner/shell/shell_test.go` | Retorno `true` via PATH ou presença em disco de qualquer cache/download. |
| **DEV-02** | `TestShellCleaner_Scan_MiseSeparateTargets` | `internal/cleaner/shell/shell_test.go` | Alvos individuais `mise cache` e `mise downloads`. |
| **DEV-02** | `TestShellCleaner_Scan_ZshCompletionAggregated` | `internal/cleaner/shell/shell_test.go` | Agregação de múltiplos `.zcompdump*` sob `shell://zsh/compdump`. |
| **DEV-02** | `TestShellCleaner_NonInterferenceWithShellRC` | `internal/cleaner/shell/shell_test.go` | `.zshrc`, `.bashrc`, `.zsh_history` intocados e não modificados após scan/clean. |
| **DEV-02** | `TestShellCleaner_RiskLevels` | `internal/cleaner/shell/shell_test.go` | `RiskMedium` para `pre-commit` e `RiskLow` para `mise`, `gh`, `omz`, `zcompdump`. |
| **DEV-03** | `TestXcodeCleaner_Scan_ExpandedTargets` | `internal/cleaner/xcode/xcode_test.go` | `CoreSimulator/Caches`, `Temp`, `iOS/watchOS DeviceSupport`, `Archives`. |
| **DEV-03** | `TestXcodeCleaner_ProcessGuard_XcodeActive` | `internal/cleaner/xcode/xcode_test.go` | `Xcode` ativo aplica `ReviewOnly: true`, `ReviewReasonActiveProcess`, `EligibleBytes: 0` a todos os alvos. |
| **DEV-03** | `TestXcodeCleaner_ProcessGuard_SimulatorActive` | `internal/cleaner/xcode/xcode_test.go` | `Simulator` ativo bloqueia alvos do CoreSimulator com `ReviewOnly: true`. |
| **DEV-03** | `TestXcodeCleaner_ProcessGuard_FailClosedOnPgrepError` | `internal/cleaner/xcode/xcode_test.go` | Erro desconhecido de `pgrep` aciona bloqueio fail-closed. |
| **DEV-03** | `TestXcodeCleaner_Archives_RiskHighWhenClosed` | `internal/cleaner/xcode/xcode_test.go` | `Archives` classificado como `RiskHigh` quando o Xcode está fechado. |
| **VAL-01** | `Test<Cleaner>_Clean_SymlinkProtection` | Todos os pacotes de cleaner | Links simbólicos ignorados e não percorridos durante o expurgo. |
| **VAL-02** | `TestCoordinator_DryRun_ZeroMutation_Phase08` | `internal/deleter/policy_test.go` | Mutação zero sob `DryRun: true` garantida em todos os alvos da Fase 08. |

---

## PATTERN MAPPING COMPLETE

# Phase 13: App Leftovers & LaunchAgents - Research

**Date:** 2026-10-18  
**Status:** Complete  
**Requirements Covered:** `LEFT-01`, `LEFT-02`, `LEFT-03`, `LEFT-04`, `VAL-01`, `VAL-02`, `SCAN-02`, `SCAN-04`, `POL-01`, `POL-02`, `POL-04`

---

## User Constraints (from CONTEXT.md)

<user_constraints>

### Parsing de Plist & Critérios de Stale LaunchAgents (LEFT-01, LEFT-02)
- **D-01: Parsing Unificado com howett.net/plist em Go Puro:** Utilizar a biblioteca `howett.net/plist` pura em Go para decodificar transparentemente tanto XML quanto binary plists (`bplist00`) sem CGO, garantindo portabilidade entre plataformas e execução hermética em testes. — **Reversibility:** costly — define a dependência e motor de decodificação de plists do projeto.
- **D-02: Critério Estrito de Stale para LaunchAgents:** Apenas um caminho absoluto direto (e não-interpretador) inexistente em disco é classificado como "Stale" (`stale_executable_missing`). Wrappers/interpretadores (`/bin/sh`, `/bin/bash`, `python`, `osascript`, `/usr/bin/env`), comandos resolvidos via `$PATH` ou caminhos relativos são marcados como "Unresolved / Ambíguo" (`launch_config_ambiguous`), sem emissão de falso positivo. — **Reversibility:** costly — define a regra de negócio central de classificação de login items.
- **D-03: Escopo Restrito a LaunchAgents de Usuário e Sistema:** Escanear estritamente `~/Library/LaunchAgents` (usuário) e `/Library/LaunchAgents` (sistema); excluir incondicionalmente `/System/Library/...` (protegido por SIP/Apple) e LaunchDaemons. Falhas de permissão em `/Library/LaunchAgents` degradam graciosamente com `ProviderMetrics.Partial = true` e `StatusNote: "permission denied (TCC / root required)"`, com `ReviewOnly: true` (`ReviewReasonSystemProtected`). — **Reversibility:** costly — define o perímetro de diretórios de LaunchAgents.
- **D-04: Fail-Closed em Plists Ilegíveis, Corrompidos ou Falhas de Ambiente:** Arquivos plist malformados, ilegíveis por permissão ou onde o launchctl estiver inacessível recebem classificação de revisão não-resolvida (`ReviewOnly: true`, `ReviewReason: domain.ReviewReasonAmbiguous`), mantendo a nota do erro em `Target.ProviderMetrics.StatusNote` com tamanho 0. — **Reversibility:** reversible — tratamento padronizado de erro em parsing.

### Detecção de App Leftovers & Estratégia de Matching (LEFT-03)
- **D-05: Varredura Determinística Direta de Aplicativos Instalados:** Realizar busca determinística de bundles `.app` via `filepath.WalkDir` com profundidade limitada a 2–3 níveis em `/Applications`, `/System/Applications` e `~/Applications`, lendo `CFBundleIdentifier` e `CFBundleExecutable`/nome do app a partir de `Info.plist`, sem CGO ou dependência do daemon do Spotlight (`mdfind`). — **Reversibility:** costly — define o mecanismo de descoberta de aplicativos do sistema.
- **D-06: Índice Bidirecional de Aplicativos (Bundle ID + Nome):** Mapear tanto o identificador reverso (`CFBundleIdentifier`, ex: `com.tinyspeck.slackmacgap`) quanto os nomes amigáveis do app/executável (`Slack`). Uma pasta em Application Support, Caches ou Containers é candidata a leftover apenas se nem seu ID nem seu nome coincidirem com qualquer app instalado. — **Reversibility:** costly — define a estratégia de correlação contra falsos positivos.
- **D-07: Proteção Multinível contra Falsos Positivos em CLI e Outros Cleaners:** Ignorar incondicionalmente diretórios pertencentes a componentes do sistema Apple (`com.apple.*`), pastas geridas por cleaners dedicados do DevClean (`docker`, `homebrew`, `pnpm`, `xcode`, editores AI/Electron, navegadores, `uv`, `mise`, `python`) e ferramentas comuns de CLI sem `.app` (ex: `git`, `gnupg`, `ssh`). — **Reversibility:** costly — estabelece o contrato de exclusão entre ecossistemas e ferramentas de terminal.
- **D-08: Escopo Estrito de Resíduos: Application Support, Caches e Containers:** Inspecionar exclusivamente as 3 localizações normativas de `LEFT-03`: `~/Library/Application Support`, `~/Library/Caches` e `~/Library/Containers`, onde se concentra a quase totalidade do volume em bytes de resíduos de apps. — **Reversibility:** costly — define o escopo físico da busca por leftovers.

### Apresentação do Inventário & Granularidade na CLI/TUI (LEFT-01, LEFT-03)
- **D-09: Apresentação Individual para LaunchAgents na CLI/TUI:** Emitir 1 alvo individual por plist stale ou ambíguo (ex: `⊙ Stale LaunchAgent: com.exemplo.service.plist`), exibindo claramente nas notas o executável faltante e o caminho do plist para inspeção direta. — **Reversibility:** reversible — formatação do alvo na CLI/TUI.
- **D-10: Agrupamento Consolidado por Aplicativo Residual para App Leftovers:** Emitir 1 alvo consolidado por aplicativo residual detectado (ex: `⊙ App Leftover: OldApp`), somando os diretórios correspondentes em Application Support, Caches e Containers sob o mesmo app, discriminando as pastas nos detalhes/notas. — **Reversibility:** reversible — agrupamento de interface do usuário.
- **D-11: Medição Real em Disco sem Corte Mínimo de Tamanho:** Calcular o uso real em disco via `fs.CalcUsageWithContext` para todas as pastas de app leftover encontradas, e o tamanho exato do arquivo `.plist` para LaunchAgents, reportando todos os candidatos detectados sem corte arbitrário de tamanho. — **Reversibility:** reversible — precisão de métricas do scan.
- **D-12: Constantes Semânticas Dedicadas de ReviewReason em domain:** Adicionar em `internal/domain/types.go` as constantes semânticas `ReviewReasonStaleLaunchAgent` e `ReviewReasonAppLeftover`, mantendo `ReviewReasonAmbiguous` para plists não-resolvidos, com descrições explicativas detalhadas em `Target.ProviderMetrics.StatusNote`. — **Reversibility:** costly — evolução dos tipos centrais do domínio.

### Garantias de Segurança, Não-Exclusão & Arquitetura (LEFT-04, VAL-01, VAL-02)
- **D-13: Defesa em Profundidade Incondicional no Clean():** O método `Clean()` do pacote `internal/cleaner/leftovers` recusa formalmente qualquer deleção física, retornando imediatamente 0 bytes limpos e log de recusa de política, garantindo conformidade absoluta com LEFT-04 mesmo sob chamadas diretas ou testes. — **Reversibility:** one-way — compromisso de segurança inviolável de não-exclusão no v2.3.
- **D-14: Pacote Dedicado internal/cleaner/leftovers e Registro no Motor:** Implementar a interface `domain.Cleaner` em um pacote dedicado `internal/cleaner/leftovers`, registrado no motor (`internal/engine/register.go`) sob o ecossistema `leftovers` com bloco correspondente em `internal/config/config.go`. — **Reversibility:** costly — modularização e arquitetura de pacotes do motor.
- **D-15: Arquitetura Injetável de Diretórios Raiz e Hermetismo Zero-CGO:** Construtores aceitam injeção de diretórios raiz (`ApplicationsDirs`, `LaunchAgentsDirs`, `AppSupportDir`, etc.), permitindo testes herméticos 100% com `t.TempDir()` tanto no Linux quanto no Darwin, com no-op em tempo de execução real em sistemas não-Darwin e compilação pura sem CGO. — **Reversibility:** costly — arquitetura de testabilidade multiplataforma.
- **D-16: Symlink Guard e Verificação de Propriedade (Ownership):** Rejeitar travessia de symlinks que apontem para fora do diretório escaneado ou pertençam a outros usuários (`st_uid != current_uid`), classificando alvos ambíguos como `ReviewOnly: true` (`ReviewReasonAmbiguous`). — **Reversibility:** costly — salvaguarda estrutural de filesystem.

### the agent's Discretion
- Nomenclatura exata de structs auxiliares internas de mapeamento de bundle e plist em `internal/cleaner/leftovers/`.
- Nomes dos arquivos de teste e geradores de fixtures mockados em `internal/cleaner/leftovers/`.
- Limite exato de profundidade da varredura de `.app` (padrão adotado: 2 a 3 níveis).

### Deferred Ideas
None — discussion stayed within phase scope.

</user_constraints>

---

## Phase Requirements Mapping

<phase_requirements>

| Requirement | Description | Research Finding & Solution |
|---|---|---|
| **LEFT-01** | User and system LaunchAgents are read from XML or binary plist files. | Implemented via pure Go parser `howett.net/plist` (zero CGO). Decodes XML and binary (`bplist00`) transparently into structured Go types. Scans `~/Library/LaunchAgents` and `/Library/LaunchAgents`. Emits `Program` or `ProgramArguments[0]` as evidence. |
| **LEFT-02** | Only an absolute reference to a missing executable is strong stale-item evidence; unreadable, relative, interpreter and PATH-based cases remain unresolved review items. | Strict evaluation pipeline: 1. Extract candidate binary from `Program` or `ProgramArguments[0]`; 2. If missing/empty -> `ReviewReasonAmbiguous`; 3. If relative or PATH-based -> `ReviewReasonAmbiguous`; 4. If script wrapper/interpreter (`/bin/sh`, `/bin/bash`, `python`, `osascript`, `env`, etc.) -> `ReviewReasonAmbiguous`; 5. If direct absolute path missing on disk via `os.Stat` -> `ReviewReasonStaleLaunchAgent`. If unreadable/permission denied -> `ReviewReasonAmbiguous` or `ReviewReasonSystemProtected`. Zero false positives. |
| **LEFT-03** | App-leftover candidates cross installed bundles with Application Support, Caches and Containers. | Deterministic bundle scanner walks `/Applications`, `/System/Applications`, `~/Applications` (depth 2–3) reading `CFBundleIdentifier` and app/executable names from `Info.plist` to build a bidirectional index. Cross-references entries in `~/Library/Application Support`, `~/Library/Caches`, and `~/Library/Containers`. Multi-level exclusion filters out Apple system components, DevClean dedicated cleaner paths, and common CLI utilities. Unmatched entries are grouped into consolidated leftover targets. |
| **LEFT-04** | Leftovers and stale LaunchAgents are never removed automatically. | Double-layer safety barrier: 1. Central `deleter.Coordinator` blocks all `ReviewOnly` targets; 2. `leftovers.Clean()` unconditionally refuses deletion of any targets, leaving 100% of files, plists, and sentinels untouched. `EligibleBytes` is always 0. |
| **VAL-01** | Hermetic testing with zero CGO, `t.TempDir()`, sentinels, permissions, symlinks, and DryRun assertions. | Test suite in `internal/cleaner/leftovers/` creates XML and binary plists, fake app bundles, and mock leftover folders in `t.TempDir()`. Asserts `Scan()`, `DryRun`, and `Clean()`, confirming 100% sentinel preservation, graceful permission degradation, interpreter avoidance, and symlink evasion. |
| **VAL-02** | Altered package test coverage > 85%, zero-CGO Linux/Darwin builds. | Pure Go implementation without CGO. Tested under `CGO_ENABLED=0` and Linux cross-compilation (`GOOS=linux go build ./...`). |

</phase_requirements>

---

## Executive Summary

Phase 13 delivers full parity with Mole's (`tw93/mole`) **App leftovers** and **Stale login items** (`⊙`) categories. In benchmark tests on Apple Silicon workstations, orphaned LaunchAgents (such as `ai-memory.plist` pointing to a missing `/Applications/AI Memory.app`) and gigabytes of residual app data left behind after dragging applications to the macOS Trash remain undetected by existing developer cleanup tools.

However, scanning login items and user application libraries presents high operational risks:
1. **The False Positive Danger in LaunchAgents:** Launchd jobs frequently invoke shell scripts, python scripts, wrappers, or PATH-resolved tools (`/usr/bin/env node`, `/bin/sh -c ...`). Naively treating any non-existent literal token as a missing executable would generate dangerous false positives, falsely advising users to delete active system agents, backup daemons, or dev utilities.
2. **The App Leftover Ambiguity:** Uninstalled application remnants must be cleanly separated from:
   - System components (`com.apple.*`, Apple daemons, system frameworks).
   - Dedicated DevClean cleaners (`docker`, `homebrew`, `pnpm`, `node`, `xcode`, `python/uv`, browsers, AI editors).
   - Terminal/CLI tools without `.app` bundles (`git`, `ssh`, `gnupg`, `zsh`, `oh-my-zsh`, `gh`, `pre-commit`, etc.).
3. **The Inviolable Non-Deletion Contract (`LEFT-04`):** Because stale LaunchAgents and application leftovers require user assessment and may carry configuration, licenses, or state the user deliberately preserved, DevClean establishes an absolute, non-negotiable policy: **neither leftovers nor LaunchAgents are ever deleted automatically.**

To achieve this safely and deterministically, Phase 13 introduces a new dedicated cleaner package `internal/cleaner/leftovers`:
- **Unified Property List Parser (`howett.net/plist`):** Zero-CGO pure Go decoder handling both XML and Apple Binary Property Lists (`bplist00`).
- **Strict Stale Criterion (`LEFT-02`):** ONLY an absolute reference to a missing executable is classified as strong stale evidence (`ReviewReasonStaleLaunchAgent`). All interpreter, wrapper, PATH-based, relative, corrupt, or unreadable plists remain unresolved review items (`ReviewReasonAmbiguous`), eliminating false positives.
- **Deterministic Bidirectional App Indexing (`LEFT-03`):** Discovers installed apps in `/Applications`, `/System/Applications`, and `~/Applications` (depth 2–3) reading `CFBundleIdentifier` and app names from `Info.plist`, pruning subdirectories upon finding `.app` to prevent runaway traversals.
- **Multi-Level Exclusion Shield:** Excludes Apple components, 21 DevClean dedicated cleaners, and CLI utilities before cross-checking `Application Support`, `Caches`, and `Containers`.
- **Target Presentation:** Emits 1 individual target per LaunchAgent plist (`⊙ Stale LaunchAgent: ...`) and 1 consolidated target per residual app (`⊙ App Leftover: OldApp`), with real disk measurement via `fs.CalcUsageWithContext` and zero arbitrary size thresholds.
- **Defensive Refusal in `Clean()` (`LEFT-04`):** Unconditional refusal of physical deletion, returning immediately with 0 bytes cleaned and sentinels completely untouched.

---

## Architectural Responsibility Map

| Component / Submodule | Specific Responsibility in Phase 13 |
|---|---|
| `internal/cleaner/leftovers/cleaner.go` | Entrypoint implementing `domain.Cleaner` (`Name() = "leftovers"`, `IsInstalled()`, `Scan()`, `Clean()`). Holds customizable root paths for hermetic testing. Enforces unconditional refusal in `Clean()` (`LEFT-04`, `D-13`). |
| `internal/cleaner/leftovers/plist.go` | Universal property list decoder using `howett.net/plist` (`D-01`). Transparently decodes XML and binary (`bplist00`) plists into LaunchAgent and InfoPlist metadata. |
| `internal/cleaner/leftovers/launchagents.go` | Scanner for `~/Library/LaunchAgents` and `/Library/LaunchAgents` (`LEFT-01`, `LEFT-02`, `D-02`, `D-03`). Evaluates `Program` vs `ProgramArguments[0]`, detects wrappers and interpreters, checks binary existence on disk, and handles permission degradation gracefully. |
| `internal/cleaner/leftovers/apps.go` | Traversal of application roots (`/Applications`, `/System/Applications`, `~/Applications`) up to depth 3 (`D-05`, `D-06`). Prunes `.app` bundles, parses `Info.plist`, and constructs the bidirectional index (`knownBundleIDs` and `knownAppNames`). |
| `internal/cleaner/leftovers/leftovers.go` | Inspects `~/Library/Application Support`, `~/Library/Caches`, and `~/Library/Containers` (`LEFT-03`, `D-08`). Applies multi-level exclusions, matches against the app index, groups leftovers by application (`D-10`), and calculates disk usage via `fs.CalcUsageWithContext` (`D-11`). |
| `internal/cleaner/leftovers/exclusions.go` | Multi-level false-positive shield (`D-07`): Apple system components, DevClean dedicated cleaner directories, and common CLI tools without `.app`. |
| `internal/cleaner/leftovers/ownership_darwin.go` & `ownership_other.go` | Non-CGO ownership validation (`st_uid == current_uid`) for user LaunchAgents and filesystems (`D-16`). |
| `internal/cleaner/leftovers/fixtures_test.go` | Test fixture generators constructing XML and binary plists, fake app bundles, and mock leftover directories in `t.TempDir()` (`VAL-01`). |
| `internal/cleaner/leftovers/cleaner_test.go` | Comprehensive unit test suite verifying XML/binary parsing, stale detection, interpreter evasion, permission handling, app leftover matching, consolidation, and `Clean()` refusal (`VAL-01`, `VAL-02`). |
| `internal/domain/types.go` | Adds semantic review reasons `ReviewReasonStaleLaunchAgent` and `ReviewReasonAppLeftover` (`D-12`). |
| `internal/config/config.go` & `defaults.go` | Adds `Leftovers EcosystemConfig` to `EcosystemsConfig` and `HooksConfig` (`D-14`). |
| `internal/engine/register.go` | Registers `leftovers.NewWithExecutor(exec)` in `RegisterDefaultCleanersWithExecutor`. |
| `internal/engine/engine.go` | Maps `"leftovers"` in `ecosystemConfig(name string)` to `e.config.Ecosystems.Leftovers`. |

---

## Technical Deep Dives

### Deep Dive 1: Property List (plist) Parsing in Pure Go (`howett.net/plist`)

#### The Problem
macOS uses two primary property list formats:
1. **XML Plist (`plist version="1.0"`):** Human-readable text format, commonly written by text editors, build scripts, or manual configs.
2. **Apple Binary Plist (`bplist00`):** Compact binary format used by macOS system services, Xcode, and most modern `.app` bundles (`Info.plist`).

Standard Go `encoding/xml` fails on binary plists, while invoking `/usr/bin/plutil` or CoreFoundation requires either CGO or shell execution, violating zero-CGO and hermetic testing requirements.

#### The Solution
We adopt `howett.net/plist v1.0.1`, a pure Go library that transparently supports XML, Binary (`bplist00`), and OpenStep formats without CGO or external processes.

```go
package leftovers

import (
	"bytes"
	"os"

	"howett.net/plist"
)

type LaunchAgentPlist struct {
	Label            string   `plist:"Label"`
	Program          string   `plist:"Program"`
	ProgramArguments []string `plist:"ProgramArguments"`
	Disabled         bool     `plist:"Disabled"`
}

type AppBundleInfoPlist struct {
	CFBundleIdentifier string `plist:"CFBundleIdentifier"`
	CFBundleName       string `plist:"CFBundleName"`
	CFBundleExecutable string `plist:"CFBundleExecutable"`
}

func parseLaunchAgentFile(path string) (*LaunchAgentPlist, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var p LaunchAgentPlist
	if _, err := plist.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

func parseAppBundleInfoFile(path string) (*AppBundleInfoPlist, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var info AppBundleInfoPlist
	if _, err := plist.Unmarshal(data, &info); err != nil {
		return nil, err
	}
	return &info, nil
}
```

In test fixtures, `plist.Marshal(val, plist.XMLFormat)` and `plist.Marshal(val, plist.BinaryFormat)` generate realistic fixtures in `t.TempDir()` on both Linux and Darwin with zero dependencies.

---

### Deep Dive 2: LaunchAgent Semantics & Stale vs. Ambiguous Classification Matrix

#### Launchd Execution Rules
According to macOS `launchd.plist(5)` semantics:
1. If the `Program` key is present and non-empty, `launchd` executes that exact path directly. `ProgramArguments` are passed as `argv[0]`, `argv[1]`, etc.
2. If `Program` is absent or empty, `ProgramArguments[0]` is taken as the executable path to execute.
3. If both are empty, the configuration is invalid.

#### Candidate Extraction Pipeline
```go
func extractExecutableCandidate(p *LaunchAgentPlist) string {
	if p.Program != "" {
		return p.Program
	}
	if len(p.ProgramArguments) > 0 {
		return p.ProgramArguments[0]
	}
	return ""
}
```

#### Known Interpreters and Shells
Per Decision `D-02`, scripts invoked via an interpreter or wrapper cannot be classified as stale simply because an argument is missing or dynamic. Any invocation starting with a known interpreter is classified as unresolved/ambiguous (`ReviewReasonAmbiguous`):

```go
var knownInterpreters = map[string]bool{
	"sh": true, "bash": true, "zsh": true, "csh": true, "tcsh": true, "ksh": true, "fish": true,
	"python": true, "python2": true, "python3": true, "pypy": true, "pypy3": true,
	"perl": true, "ruby": true, "osascript": true, "tclsh": true, "wish": true,
	"env": true, "node": true, "deno": true, "bun": true,
}

func isInterpreter(execPath string) bool {
	base := strings.ToLower(filepath.Base(execPath))
	if knownInterpreters[base] {
		return true
	}
	// Handle versioned interpreters (e.g. python3.11, ruby3.2)
	for prefix := range knownInterpreters {
		if strings.HasPrefix(base, prefix) {
			return true
		}
	}
	return false
}
```

#### Classification Decision Matrix

| Condition | Classification | ReviewReason | StatusNote Evidence | Target Surfaced? |
|---|---|---|---|:---:|
| `Program` is direct absolute path (e.g. `/Applications/OldApp.app/.../binary`) and missing on disk (`os.IsNotExist`) | **Stale LaunchAgent** | `ReviewReasonStaleLaunchAgent` | `"missing executable: /Applications/OldApp.app/.../binary"` | Yes (`⊙ Stale LaunchAgent: ...`) |
| `ProgramArguments[0]` is direct absolute path and missing on disk (`os.IsNotExist`) | **Stale LaunchAgent** | `ReviewReasonStaleLaunchAgent` | `"missing executable: /Applications/OldApp.app/.../binary"` | Yes (`⊙ Stale LaunchAgent: ...`) |
| Binary exists on disk (`err == nil`) | **Active / Valid Item** | — | — | **No** (not a cleanup target) |
| Executable is an interpreter (`/bin/sh`, `/usr/bin/python`, `/usr/bin/env`, etc.) | **Ambiguous / Unresolved** | `ReviewReasonAmbiguous` | `"interpreter or wrapper invocation: /bin/sh"` | Yes (`⊙ LaunchAgent (ambiguous): ...`) |
| Command is relative or PATH-based (e.g. `node`, `brew`, `backup.sh`) | **Ambiguous / Unresolved** | `ReviewReasonAmbiguous` | `"relative or PATH-based executable: node"` | Yes (`⊙ LaunchAgent (ambiguous): ...`) |
| Plist has no `Program` and empty `ProgramArguments` | **Ambiguous / Unresolved** | `ReviewReasonAmbiguous` | `"empty or missing Program/ProgramArguments"` | Yes (`⊙ LaunchAgent (ambiguous): ...`) |
| Plist is malformed, corrupt, or unreadable | **Ambiguous / Unresolved** | `ReviewReasonAmbiguous` | `"corrupt or unreadable plist: <err>"` | Yes (`⊙ LaunchAgent (unreadable): ...`) |
| Permission denied reading plist (`/Library/LaunchAgents` TCC/root) | **Protected / Degraded** | `ReviewReasonSystemProtected` | `"permission denied (TCC / root required)"` | Yes (`ProviderMetrics.Partial = true`) |
| User plist owned by another UID (`st_uid != current_uid`) | **Ambiguous / Unresolved** | `ReviewReasonAmbiguous` | `"insecure ownership: uid <uid> != current <current>"` | Yes (`⊙ LaunchAgent (ambiguous): ...`) |
| Plist is a symlink pointing outside directory or to blacklisted root | **Ambiguous / Unresolved** | `ReviewReasonAmbiguous` | `"unsafe or external symlink target"` | Yes (`⊙ LaunchAgent (ambiguous): ...`) |

---

### Deep Dive 3: Installed App Discovery & Bidirectional Indexing

#### Scope and Depth Limiting (`D-05`)
To avoid scanning hundreds of thousands of files inside app bundles, the application scanner traverses `/Applications`, `/System/Applications`, and `~/Applications` with bounded depth:
- Maximum depth: 3 levels (covers `/Applications/Utilities/Terminal.app`, `/Applications/Xcode.app`, `/Applications/Setapp/App.app`).
- **Pruning optimization:** Once a directory with the `.app` suffix is encountered, its `Contents/Info.plist` is read, its identifiers are recorded, and `filepath.SkipDir` is returned immediately. This avoids traversing the heavy framework and resource trees inside apps.

```go
func scanInstalledApps(appRoots []string) (*AppIndex, error) {
	index := &AppIndex{
		BundleIDs: make(map[string]bool),
		AppNames:  make(map[string]bool),
	}

	for _, root := range appRoots {
		if _, err := os.Stat(root); err != nil {
			continue // Gracefully skip non-existent root
		}

		cleanRoot := filepath.Clean(root)
		rootDepth := strings.Count(cleanRoot, string(filepath.Separator))

		_ = filepath.WalkDir(cleanRoot, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil // Skip inaccessible subpaths
			}

			// Depth control: max 3 levels deeper than root
			currentDepth := strings.Count(path, string(filepath.Separator))
			if currentDepth-rootDepth > 3 {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			if d.IsDir() && strings.HasSuffix(d.Name(), ".app") {
				appNameNoExt := strings.TrimSuffix(d.Name(), ".app")
				index.addAppName(appNameNoExt)

				// Parse Info.plist if present
				infoPlistPath := filepath.Join(path, "Contents", "Info.plist")
				if info, err := parseAppBundleInfoFile(infoPlistPath); err == nil {
					if info.CFBundleIdentifier != "" {
						index.addBundleID(info.CFBundleIdentifier)
					}
					if info.CFBundleName != "" {
						index.addAppName(info.CFBundleName)
					}
					if info.CFBundleExecutable != "" {
						index.addAppName(info.CFBundleExecutable)
					}
				}

				return filepath.SkipDir // CRUCIAL: do not descend into .app bundle!
			}

			return nil
		})
	}

	return index, nil
}
```

#### Bidirectional Matching Strategy (`D-06`)
- `BundleIDs`: stores normalized lowercased bundle identifiers (e.g. `com.tinyspeck.slackmacgap`, `com.spotify.client`, `com.google.chrome`).
- `AppNames`: stores normalized lowercased app names and executables (e.g. `slack`, `spotify`, `chrome`, `jetbrains toolbox`).

A candidate directory is matched as an installed app (and therefore NOT a leftover) if:
1. Exact match in `AppNames`: `index.AppNames[lowerCandidateName]`
2. Exact match in `BundleIDs`: `index.BundleIDs[lowerCandidateName]`
3. If candidate name contains dots (bundle ID format):
   - Prefix match: any known bundle ID starts with `candidateName` or `candidateName` starts with a known bundle ID (e.g. helper processes: `com.tinyspeck.slackmacgap.helper` matches `com.tinyspeck.slackmacgap`).
   - Suffix/segment match: the last component of the bundle ID (e.g. `slackmacgap` in `com.tinyspeck.slackmacgap`) exists in `AppNames`.

---

### Deep Dive 4: App Leftover Cross-Checking & Exclusion Shield (`D-07`, `D-08`)

#### Inspected Locations
In strict compliance with `LEFT-03` and `D-08`, scanning is restricted to:
1. `~/Library/Application Support`
2. `~/Library/Caches`
3. `~/Library/Containers`

#### Multi-Level False Positive Exclusion Shield (`D-07`)
Before any directory in these locations is checked against the app index, it must pass a three-tier exclusion shield:

```go
// 1. Apple System Components & Daemons
var appleSystemDirNames = map[string]bool{
	"addressbook": true, "animoji": true, "app store": true, "callhistorydb": true,
	"callhistorytransactions": true, "categories": true, "clouddocs": true,
	"controlcenter": true, "crashreporter": true, "differentialprivacy": true,
	"diskimages": true, "facetime": true, "fileprovider": true, "knowledge": true,
	"passes": true, "quicklook": true, "safari": true, "screencontinuity": true,
	"siri": true, "syncservices": true, "apple": true, "apple push service": true,
}

func isAppleComponent(name string) bool {
	lower := strings.ToLower(name)
	if strings.HasPrefix(lower, "com.apple.") || strings.HasPrefix(lower, "apple") {
		return true
	}
	return appleSystemDirNames[lower]
}

// 2. Directories managed by dedicated DevClean Cleaners
var dedicatedCleanerDirs = map[string]bool{
	"pnpm": true, "uv": true, "pip": true, "homebrew": true, "cocoapods": true,
	"vcpkg": true, "mise": true, "gh": true, "ms-playwright": true, "cypress": true,
	"node": true, "yarn": true, "dev.orbstack.orbstack": true, "docker": true,
	"coresimulator": true, "developer": true, "google": true, "bravesoftware": true,
	"company.thebrowser.arc": true, "microsoft edge": true, "firefox": true,
	"code": true, "cursor": true, "antigravity": true, "windsurf": true,
	"claude": true, "opencode": true, "zed": true,
}

// 3. Common Non-App CLI Utilities
var commonCLITools = map[string]bool{
	"git": true, "gnupg": true, "gpg": true, "ssh": true, "zsh": true,
	"oh-my-zsh": true, "pre-commit": true, "cargo": true, "rustup": true,
	"go": true, "bun": true, "brew": true, "lima": true, "colima": true,
	"podman": true, "tmux": true, "nvim": true, "vim": true, "subversion": true,
}
```

In addition, each path is evaluated against `safety.IsCoreBlacklisted(path)` and `macos.IsWhitelisted(path)`.

---

### Deep Dive 5: Consolidation & Target Presentation (`D-09` to `D-12`)

#### LaunchAgents Presentation (`D-09`)
LaunchAgents are presented individually (1 Target per plist file), enabling immediate inspection of the stale plist and its missing executable:
- **Target ID:** `fmt.Sprintf("launchagent-%s", sanitize(filepath.Base(plistPath)))`
- **Name:** `fmt.Sprintf("Stale LaunchAgent: %s", filepath.Base(plistPath))` (or `LaunchAgent (ambiguous): ...`)
- **Path:** `plistPath`
- **SizeBytes:** File size of the `.plist` file
- **EligibleBytes:** `0`
- **TTLEligible:** `false`
- **ReviewOnly:** `true`
- **ReviewReason:** `domain.ReviewReasonStaleLaunchAgent` (or `domain.ReviewReasonAmbiguous`)
- **Category:** `"leftovers-launchagent"`
- **Risk:** `domain.RiskLow`
- **ProviderMetrics.StatusNote:** `"missing executable: <path>"`

#### App Leftovers Consolidation (`D-10`, `D-11`)
When an uninstalled application leaves remnants in multiple directories (e.g. `Application Support/OldApp`, `Caches/com.example.oldapp`, and `Containers/com.example.oldapp`), emitting 3 fragmented targets clutters the UI and obscures the overall footprint.

Instead, candidates are grouped by their canonical application name:
1. Normalized group key is derived:
   - If the name is a reverse domain (`com.example.oldapp`), the last segment `oldapp` is extracted.
   - If the name is a friendly name (`OldApp`), its lowercased string `oldapp` is used.
2. All directories belonging to `oldapp` are gathered into an `AppLeftoverGroup`.
3. Disk usage is measured across all directories via `fs.CalcUsageWithContext(ctx, dir)`.
4. A single consolidated target is emitted:
   - **Target ID:** `fmt.Sprintf("app-leftover-%s", sanitize(group.AppName))`
   - **Name:** `fmt.Sprintf("App Leftover: %s", group.AppName)`
   - **Path:** Primary directory (Application Support preferred, otherwise Caches or Containers)
   - **SizeBytes:** Summed allocated bytes (or apparent bytes) across all group directories
   - **EligibleBytes:** `0`
   - **TTLEligible:** `false`
   - **ReviewOnly:** `true`
   - **ReviewReason:** `domain.ReviewReasonAppLeftover`
   - **Category:** `"leftovers-app"`
   - **Risk:** `domain.RiskLow`
   - **ProviderMetrics:**
     - `TotalCount`: number of residual directories
     - `SizeBytes`: total size
     - `ApparentBytes`: total apparent size
     - `AllocatedBytes`: total allocated size
     - `StatusNote`: `"residual directories: " + strings.Join(group.Paths, ", ")`

---

### Deep Dive 6: The Inviolable Non-Deletion Contract (`LEFT-04`, `VAL-01`)

Requirement `LEFT-04` mandates: *"Leftovers and stale LaunchAgents are never removed automatically."*

To ensure this policy cannot be bypassed by any command-line flag, automation mode, or refactoring:
1. **Central Coordinator Rejection:** `deleter.Coordinator` (Stage 1) checks `t.ReviewOnly`. Because all leftovers and LaunchAgent targets have `ReviewOnly: true`, the coordinator immediately diverts them to `SkippedReviewOnly` and emits warnings.
2. **Cleaner-Level Defensive Refusal (`D-13`):** In `internal/cleaner/leftovers/cleaner.go`:
   ```go
   func (c *leftoversCleaner) Clean(ctx context.Context, targets []domain.Target) error {
   	// Defensive refusal guard (LEFT-04, D-13):
   	// Leftovers and LaunchAgents must NEVER be deleted automatically in v2.3.
   	// This method unconditionally refuses physical deletion, ensuring zero mutations
   	// even under direct test invocations or coordinator bypass.
   	return nil
   }
   ```
3. **Hermetic Sentinel Test (`VAL-01`):** A dedicated unit test `TestLeftovers_CleanRefusesDeletion` populates real sentinel files inside mock LaunchAgent plists and app leftover directories, invokes `c.Clean(ctx, targets)`, and asserts that 100% of the sentinel files still exist on disk with unchanged contents and timestamps.

---

## Risk Analysis and Mitigations

| Risk | Impact | Mitigation Strategy |
|---|---|---|
| **False Positives in LaunchAgents from Shell Wrappers or PATH commands** | Active user jobs falsely flagged as stale, risking disruption. | Strict classification pipeline (`D-02`). Any command invoking an interpreter (`sh`, `bash`, `python`, `env`, etc.) or relative command is classified as `ReviewReasonAmbiguous` with explanatory note, never as `stale_executable_missing`. |
| **Runaway Filesystem Traversal inside `.app` Bundles** | High scan latency, high memory usage scanning Xcode or Electron app internals. | Depth-limited traversal (max 3) coupled with `filepath.SkipDir` immediately upon finding any directory ending in `.app` (`D-05`). |
| **False Positives on CLI Tools in Application Support** | Deleting configs of essential terminal tools (`git`, `ssh`, `gnupg`, `mise`). | Multi-level exclusion filter (`D-07`) specifically allowlists and protects CLI tools and DevClean dedicated cleaners. |
| **Permission Denied in `/Library/LaunchAgents`** | Crash or aborted scan on macOS due to lack of root/TCC privileges. | Graceful degradation (`D-03`, `D-04`): catches permission errors and populates `ProviderMetrics.Partial = true` with `ReviewReasonSystemProtected`. |
| **Accidental Automatic Deletion** | Loss of user app preferences, license keys, or launch configs. | Dual-layer refusal (`LEFT-04`, `D-13`): `ReviewOnly: true` in coordinator AND unconditional refusal in `cleaner.Clean()`. |
| **Cross-Platform Test Breakage** | Tests failing on Linux due to macOS-specific paths or APIs. | Injected directory roots (`D-15`) allowing 100% hermetic tests in `t.TempDir()`, zero CGO, and pure Go plist parsing. |
| **Test Hardcoded Ecosystem Counts** | Adding `leftovers` breaks tests asserting `len(eng.Ecosystems()) == 21`. | `internal/engine/register_test.go` and similar tests must be updated to expect 22 ecosystems, sorted with `leftovers` between `java` and `macos`. |

---

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go test (`go test`) |
| Config file | `go.mod` |
| Quick run command | `go test -v ./internal/cleaner/leftovers/...` |
| Full suite command | `CGO_ENABLED=0 go test ./...` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| LEFT-01 | Plist parsing (XML + binary bplist00) | unit | `go test -v ./internal/cleaner/leftovers -run "TestLeftovers_LaunchAgents_XMLAndBinary"` | ❌ Wave 0 |
| LEFT-02 | Stale vs Ambiguous LaunchAgents classification | unit | `go test -v ./internal/cleaner/leftovers -run "TestLeftovers_LaunchAgents_StaleClassification|TestLeftovers_LaunchAgents_InterpretersAmbiguous"` | ❌ Wave 0 |
| LEFT-03 | Installed app indexing & leftovers cross-check | unit | `go test -v ./internal/cleaner/leftovers -run "TestLeftovers_AppLeftovers_DiscoveryAndMatching|TestLeftovers_AppLeftovers_Exclusions"` | ❌ Wave 0 |
| LEFT-04 | Inviolable non-deletion contract in Clean() & DryRun | unit/integration | `go test -v ./internal/cleaner/leftovers -run "TestLeftovers_CleanRefusesDeletion|TestLeftovers_DryRun"` | ❌ Wave 0 |
| VAL-01/02 | Engine registration (22 ecosystems) & zero-mutation verification | integration | `go test -v ./internal/engine -run "TestRegisterDefaultCleaners" && CGO_ENABLED=0 go test ./...` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test -v ./internal/cleaner/leftovers/...`
- **Per wave merge:** `CGO_ENABLED=0 go test ./...`
- **Phase gate:** Full suite green before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `internal/cleaner/leftovers/` — package directory and test fixture helpers (`fixtures_test.go`)
- [ ] Dependency `howett.net/plist` added to `go.mod`
- [ ] Hermetic mock directories with sentinels for LaunchAgents and leftover bundles

## Test Strategy and Acceptance Plan (VAL-01, VAL-02)

### Test Fixtures (`fixtures_test.go`)
- `createTestAppBundle(t, dir, bundleID, appName, exeName)`: Creates a minimal `.app/Contents/Info.plist` (both XML and binary).
- `createTestLaunchAgentXML(t, dir, filename, label, program, args)`: Creates an XML LaunchAgent plist.
- `createTestLaunchAgentBinary(t, dir, filename, label, program, args)`: Creates a binary (`bplist00`) LaunchAgent plist using `plist.Marshal(..., plist.BinaryFormat)`.
- `createTestDirWithSentinels(t, dir, name)`: Creates a leftover directory containing disposable files and sentinel files.

### Unit Test Matrix (`cleaner_test.go`)
1. **`TestLeftovers_LaunchAgents_XMLAndBinary`**: Asserts that both XML and binary LaunchAgent plists are parsed identically with zero CGO.
2. **`TestLeftovers_LaunchAgents_StaleClassification`**: Asserts that a missing absolute executable (`/missing/path/binary`) receives `ReviewReasonStaleLaunchAgent`, `ReviewOnly: true`, and `EligibleBytes: 0`.
3. **`TestLeftovers_LaunchAgents_ActivePreserved`**: Asserts that an existing executable is recognized as active and is NOT surfaced as a stale target.
4. **`TestLeftovers_LaunchAgents_InterpretersAmbiguous`**: Asserts that `/bin/sh`, `/usr/bin/python`, `/usr/bin/env` invocations are classified as `ReviewReasonAmbiguous`.
5. **`TestLeftovers_LaunchAgents_RelativePathAmbiguous`**: Asserts that PATH-based commands (`node`, `brew`) are classified as `ReviewReasonAmbiguous`.
6. **`TestLeftovers_LaunchAgents_CorruptedPlist`**: Asserts that unparseable or truncated plists fail closed as `ReviewReasonAmbiguous` with zero bytes and error note.
7. **`TestLeftovers_LaunchAgents_PermissionDegradation`**: Asserts that permission-denied directories degrade gracefully with `Partial: true` and `ReviewReasonSystemProtected`.
8. **`TestLeftovers_AppLeftovers_DiscoveryAndMatching`**: Asserts that installed apps prevent leftover flagging for matching directories.
9. **`TestLeftovers_AppLeftovers_Exclusions`**: Asserts that Apple components, DevClean dedicated cleaners, and CLI tools are never flagged as leftovers.
10. **`TestLeftovers_AppLeftovers_Consolidation`**: Asserts that remnants across `Application Support`, `Caches`, and `Containers` are consolidated into 1 target per application.
11. **`TestLeftovers_CleanRefusesDeletion`**: Asserts that `Clean()` never deletes any files and leaves 100% of sentinels intact (`LEFT-04`).
12. **`TestLeftovers_DryRun`**: Asserts that `DryRun` produces zero mutations and identical inventory.
13. **`TestLeftovers_EngineRegistration`**: Asserts engine registration, sorted ecosystem list (22 ecosystems), and config loading.

---

## Implementation Sequence Plan (Work Breakdown)

### Plan 13-01: Foundation, Plist Engine & LaunchAgent Scanner (`LEFT-01`, `LEFT-02`)
- Add `howett.net/plist` to `go.mod`.
- Add `ReviewReasonStaleLaunchAgent` and `ReviewReasonAppLeftover` in `internal/domain/types.go` and update `target_test.go`.
- Create `internal/cleaner/leftovers/plist.go` with XML and binary decoders.
- Create `internal/cleaner/leftovers/launchagents.go` with executable candidate extraction, interpreter checks, missing executable checks, and permission degradation.
- Create unit tests in `internal/cleaner/leftovers/` covering XML/binary parsing, stale vs active vs ambiguous classification, and error handling.

### Plan 13-02: App Indexer, Leftover Scanner & Exclusions Shield (`LEFT-03`)
- Create `internal/cleaner/leftovers/apps.go` with depth-limited `.app` traversal and bidirectional index building (`knownBundleIDs`, `knownAppNames`).
- Create `internal/cleaner/leftovers/exclusions.go` with Apple components, DevClean cleaners, and CLI tools filters.
- Create `internal/cleaner/leftovers/leftovers.go` scanning `Application Support`, `Caches`, and `Containers`, matching against index, and grouping by application.
- Create unit tests verifying bundle discovery, exclusion shield, and leftover consolidation.

### Plan 13-03: Cleaner Lifecycle, Inviolable Clean() Guard & Engine Wiring (`LEFT-04`, `VAL-01`, `VAL-02`)
- Create `internal/cleaner/leftovers/cleaner.go` implementing `domain.Cleaner` (`Name() = "leftovers"`, `IsInstalled()`, `Scan()`, `Clean()`).
- Implement defensive refusal in `Clean()` ensuring zero file deletion.
- Add `Leftovers EcosystemConfig` to `internal/config/config.go` and `defaults.go`.
- Register `leftovers` in `internal/engine/register.go` and `engine.go`.
- Update `internal/engine/register_test.go` to account for 22 ecosystems.
- Add comprehensive sentinel tests verifying `Clean()` refuses any deletion.

---

## Verification Commands for Executor

```bash
# 1. Run all unit tests for the new leftovers cleaner
go test -v ./internal/cleaner/leftovers/...

# 2. Verify domain and engine registration tests
go test -v ./internal/domain/... ./internal/engine/...

# 3. Verify zero-CGO test execution
CGO_ENABLED=0 go test ./internal/cleaner/leftovers/...

# 4. Verify Linux cross-compilation with zero-CGO
GOOS=linux CGO_ENABLED=0 go build ./...

# 5. Verify entire project test suite
go test ./...
```

---

## RESEARCH COMPLETE

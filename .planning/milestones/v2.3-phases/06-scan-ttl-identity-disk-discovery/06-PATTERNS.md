# Phase 06: Scan, TTL, Identity & Disk Discovery - Pattern Mapping

**Phase:** 06-scan-ttl-identity-disk-discovery  
**Milestone:** v2.3 Paridade Total com Mole (tw93/mole)  
**Date:** 2026-10-14  
**Status:** Approved for Implementation  

---

## 1. Overview & Pattern Mapping Summary

Este documento estabelece o mapeamento estrutural entre os arquivos a serem criados e modificados na **Fase 06** e os análogos consolidados na base de código do DevClean (`internal/domain/types.go`, `internal/config/config.go`, `internal/engine/engine.go`, `internal/engine/register.go`, `internal/fs/stat.go`, `internal/cleaner/*`, `cmd/scan.go`, etc.).

O objetivo desta fase é eliminar a disparidade de inventário descoberta no benchmark contra o `tw93/mole` (onde mais de 17 GB deixavam de ser identificados) através de quatro pilares:
1. **Desacoplamento Scan vs. TTL (`SCAN-01`):** `Scan()` reporta 100% dos alvos existentes e seus bytes físicos reais (`SizeBytes`) sem jamais descartar diretórios pelo `mtime` da pasta raiz. As decisões de expurgo por TTL são materializadas explicitamente em `EligibleBytes` e `TTLEligible`.
2. **Detecção Híbrida de Presença em Disco (`SCAN-02`):** Cleaners reportam `IsInstalled() == true` se o executável estiver no `PATH` **OU** se seus diretórios padrão de cache existirem no disco, viabilizando a higienização de ferramentas antigas ou desinstaladas.
3. **Descoberta Automática Determinística de `ScanPaths` (`SCAN-03`):** Quando `cfg.ScanPaths` estiver vazio, inspeciona `$HOME` na ordem determinística (`Documents`, `Projects`, `Developer`, `Code`, `Work`, `repos`, `workspace`) em tempo de execução via `EffectiveScanPaths()`, sem mutação do YAML.
4. **Propriedade Canônica e Deduplicação Centralizada (`SCAN-04`):** Resolução de links simbólicos (`filepath.EvalSymlinks`), atribuição estrita onde cleaners dedicados têm precedência sobre genéricos (`D-01`), dedução defensiva de bytes em árvores hierárquicas e omissão de alvos absorvidos.

### Tabela de Mapeamento de Padrões

| Arquivo Alvo | Papel & Fluxo de Dados | Análogo Mais Próximo | Padrões-Chave Reaproveitados |
| :--- | :--- | :--- | :--- |
| `internal/domain/types.go` | **Modelo de Domínio Central**<br>Extensão de `domain.Target` com campos de elegibilidade TTL | `internal/domain/types.go:23-39` | Inclusão de `EligibleBytes int64` e `TTLEligible bool` mantendo compatibilidade JSON e serialização limpa; alvos virtuais imunes (`t.IsVirtual()`). |
| `internal/config/config.go`<br>`internal/config/config_test.go` | **Configuração & Descoberta Dinâmica**<br>Método `EffectiveScanPaths()` com descoberta em `$HOME` | `internal/config/config.go:142-283`<br>`internal/platform/resolver.go:28-39` | Verificação determinística das 7 pastas canônicas; precedência estrita para `ScanPaths` explícito; isolamento em memória sem mutação do YAML (`D-11`, `D-12`). |
| `internal/fs/stat.go`<br>`internal/fs/stat_test.go` | **Primitivas de Filesystem POSIX**<br>Helper atômico `DirExists` | `internal/fs/stat.go:1-10`<br>`internal/safety/symlink.go:5-13` | Verificação superficial não recursiva com `os.Stat` tratando erros graciosamente; checagem de diretório via `info.IsDir()`. |
| `internal/engine/ownership.go`<br>`internal/engine/ownership_test.go` | **Resolução Canônica de Propriedade**<br>Pipeline central de deduplicação e dedução de bytes | `internal/safety/blacklist.go:32-52`<br>`internal/cleaner/java/java.go:242-257`<br>`internal/engine/engine.go:142-156` | Normalização com `filepath.Clean` + `filepath.EvalSymlinks`; hierarquia via `filepath.Rel`; classificação por `EcosystemTier`; dedução clamp de bytes (`D-01` a `D-04`). |
| `internal/engine/engine.go`<br>`internal/engine/register.go`<br>`internal/engine/engine_test.go` | **Orquestrador de Varredura Concorrente**<br>Injeção de caminhos efetivos e pós-processamento | `internal/engine/engine.go:59-129`<br>`internal/engine/register.go:28-63` | Injeção de `cfg.EffectiveScanPaths()` nos cleaners de projeto; orquestração paralela com `errgroup`; resolução atômica de propriedade pós-streaming. |
| `internal/cleaner/dotnet/dotnet.go`<br>`dotnet_test.go` | **Cleaner de Ecossistema (.NET)**<br>Scan completo sem mtime de raiz e detecção híbrida | `internal/cleaner/dotnet/dotnet.go:38-41, 81-96`<br>`internal/cleaner/rust/rust.go:31-43` | `IsInstalled` via `LookPath` \|\| cache no disco; remoção do filtro `cutoff` em caches globais; preenchimento de `EligibleBytes` e `TTLEligible`. |
| `internal/cleaner/golang/golang.go`<br>`golang_test.go` | **Cleaner de Ecossistema (Go)**<br>Scan completo sem mtime de raiz e detecção híbrida | `internal/cleaner/golang/golang.go:27-30, 103-122` | `IsInstalled` via `LookPath("go")` \|\| `GOMODCACHE` / `GOCACHE` no disco; desacoplamento de `Scan()` de `cutoff` na raiz. |
| `internal/cleaner/bun/bun.go`<br>`bun_test.go` | **Cleaner de Ecossistema (Bun)**<br>Inventário de cache físico independente de TTL | `internal/cleaner/bun/bun.go:39-47, 50-78` | Remoção do return `nil, nil` quando `info.ModTime().After(cutoff)`; retorno incondicional do alvo com `EligibleBytes`/`TTLEligible`. |
| `internal/cleaner/python/python.go`<br>`python_test.go` | **Cleaner de Ecossistema (Python)**<br>Scan de caches uv/pip/conda sem filtro prematuro | `internal/cleaner/python/python.go:32-42, 125-145` | Remoção de `cutoff` em `scanUVCache`; detecção híbrida em `IsInstalled`; preenchimento de `EligibleBytes`. |
| `internal/cleaner/xcode/xcode.go`<br>`xcode_test.go` | **Cleaner de Ecossistema (Xcode)**<br>DerivedData granulado completo e detecção híbrida | `internal/cleaner/xcode/xcode.go:73-80, 110-128` | `IsInstalled` checando Darwin && (`LookPath("xcrun")` \|\| pasta `DerivedData` existente); todos os projetos de DerivedData reportados no scan. |
| `internal/cleaner/node/node.go`<br>`node_test.go` | **Cleaner de Ecossistema (Node.js)**<br>Alvos locais e globais desacoplados | `internal/cleaner/node/node.go:111-125, 297-328`<br>`internal/cleaner/risk.go:10-50` | Alvos `node_modules`, `dist` e `.nuxt` sempre reportados; `EligibleBytes` e `TTLEligible` preenchidos conforme TTL e manifestos lock. |
| `internal/cleaner/rust/rust.go`<br>`rust_test.go` | **Cleaner de Ecossistema (Rust)**<br>Caches de cargo e targets locais | `internal/cleaner/rust/rust.go:31-43, 126-137` | `IsInstalled` checando cargo no PATH \|\| `$CARGO_HOME/registry` no disco; reporte completo de alvos com campos TTL. |
| `internal/cleaner/java/java.go`<br>`java_test.go` | **Cleaner de Ecossistema (Java)**<br>Gradle/Maven targets e caches | `internal/cleaner/java/java.go:31-50, 127-140` | `IsInstalled` híbrido com verificação de caches; preenchimento de `EligibleBytes`/`TTLEligible` para `build/` e `target/`. |
| `internal/cleaner/pnpm/pnpm.go`<br>`pnpm_test.go` | **Cleaner de Ecossistema (pnpm)**<br>Detecção híbrida de store e alvos virtuais | `internal/cleaner/pnpm/pnpm.go:31-34, 52-62` | `IsInstalled` via `LookPath` \|\| store global padrão existente; cálculo de `EligibleBytes` sem omitir alvos virtuais. |
| `internal/cleaner/homebrew/homebrew.go`<br>`homebrew_test.go` | **Cleaner de Ecossistema (Homebrew)**<br>Detecção híbrida de cache brew | `internal/cleaner/homebrew/homebrew.go:35-39, 67-76` | `IsInstalled` via `LookPath("brew")` \|\| diretórios padrão de cache do Homebrew; preenchimento de campos de elegibilidade. |
| `internal/cleaner/ai/editors.go`<br>`ai.go`, `ai_test.go` | **Cleaner de Ecossistema (AI/Editores)**<br>CachedData e logs reportados na íntegra | `internal/cleaner/ai/editors.go:41-75` | Remoção do bloqueio de `cutoff` na raiz de `CachedData` e `logs`; atribuição correta de `EligibleBytes`. |
| `internal/cleaner/nextjs/nextjs.go`<br>`nextjs_test.go` | **Cleaner de Ecossistema (Next.js)**<br>Cache `.next/cache` sem descarte por mtime | `internal/cleaner/nextjs/nextjs.go:73-89` | Remoção do descarte precoce em `Scan()`; alvos de build/cache sempre reportados com status de elegibilidade. |
| `internal/cleaner/{cpp,flutter,terraform,cocoapods,elixir}/*` | **Cleaners Complementares de Ecossistemas** | `internal/cleaner/cocoapods/cocoapods.go:36-57` | Adoção padronizada de checagens rasas em `IsInstalled()` e preenchimento de `EligibleBytes`/`TTLEligible`. |
| `cmd/scan.go`<br>`cmd/format.go`<br>`cmd/clean.go` | **Interface CLI & Comandos de Saída**<br>Consumo desacoplado de alvos e TTL | `cmd/scan.go:50-75`<br>`cmd/format.go:25-68`<br>`cmd/clean.go:132-137, 334-347` | Exibição transparente de `SizeBytes` (tamanho físico total) e reflexo de `EligibleBytes`/`TTLEligible`; `clean --auto` consome alvos elegíveis. |

---

## 2. File Classification

### 2.1. Domain Core Tier
- **`internal/domain/types.go`**: Entidade central de dados (`domain.Target`).
  - *Data Flow:* É construído por todos os cleaners em `Scan()`, inspecionado por `engine.ResolveCanonicalOwnership`, consumido pelo `intelligence.Scorer`, formatado em `cmd/format.go` e executado pelo `deleter.Coordinator`.

### 2.2. Configuration & Discovery Tier
- **`internal/config/config.go`** & **`internal/config/config_test.go`**:
  - *Data Flow:* Lê a configuração via Viper (`Load`). O método dinâmico `EffectiveScanPaths()` inspeciona `$HOME` na ausência de `ScanPaths` explícito. Alimenta o registro do `Engine` sem mutar os dados persistidos no arquivo YAML (`D-11`, `D-12`).

### 2.3. Filesystem Primitives Tier
- **`internal/fs/stat.go`** & **`internal/fs/stat_test.go`**:
  - *Data Flow:* Fornece a primitiva atômica pura `DirExists(path string) bool` que isola chamadas `os.Stat` protegendo o restante do sistema contra panics ou tratamento incorreto de `os.ErrNotExist` e permissões restritas.

### 2.4. Engine & Canonical Ownership Tier
- **`internal/engine/ownership.go`** & **`internal/engine/ownership_test.go`**:
  - *Data Flow:* Recebe `[]domain.ScanResult` brutos após o scan paralelo. Normaliza com `filepath.Clean`, resolve caminhos reais físicos com `filepath.EvalSymlinks`, detecta sobreposições hierárquicas com `filepath.Rel`, deduz bytes do ancestral genérico em favor do cleaner dedicado e descarta duplicatas exatas subordinadas.
- **`internal/engine/engine.go`** & **`internal/engine/register.go`**:
  - *Data Flow:* Orquestra os scanners concorrentes via `errgroup`. Em `register.go`, injeta os `EffectiveScanPaths()` nos construtores dos cleaners de ecossistema. Executa a resolução canônica antes da entrega final dos resultados.

### 2.5. Ecosystem Cleaners Tier (18 Cleaners)
- **Família A (Artefatos de Projetos + Caches Globais):** `dotnet`, `node`, `rust`, `java`, `python`, `cpp`, `flutter`, `terraform`, `cocoapods`, `elixir`, `nextjs`.
  - *Data Flow:* Executam varreduras rasas em `IsInstalled()` e varreduras locais em `ScanPaths`. Emitem todos os alvos físicos encontrados com `SizeBytes = CalcDirSize(path)` e preenchem `EligibleBytes` e `TTLEligible` sem descartar alvos cujo mtime da raiz seja recente.
- **Família B (Caches Globais Puros / Toolchains):** `bun`, `golang`, `ai` (`editors`, `claude`, `ollama`, `huggingface`).
  - *Data Flow:* Inspecionam diretórios de cache globais conhecidos. `IsInstalled()` checa binário no PATH **ou** presença da pasta no disco. `Scan()` emite alvos completos.
- **Família C (Recursos Virtuais & CLI-Driven):** `docker`, `pnpm`, `homebrew`, `xcode` (simuladores).
  - *Data Flow:* Operam alvos com esquemas virtuais (`docker://`, `pnpm://`, `brew://`, `xcode://`). São imunes à deduplicação de filesystem em `ownership.go`.

### 2.6. CLI & Presentation Tier
- **`cmd/scan.go`**, **`cmd/format.go`**, **`cmd/clean.go`**:
  - *Data Flow:* `scan.go` orquestra a chamada de `eng.Scan(ctx)`. `format.go` renderiza tabelas e JSON refletindo o tamanho real físico. `clean.go` (em modo `--auto`) seleciona alvos elegíveis utilizando `t.TTLEligible` e `t.EligibleBytes`.

---

## 3. Pattern Assignments

### 3.1. Domain Model: `internal/domain/types.go`

- **Papel:** Modela o contrato de dados de alvos rastreados pelo DevClean.
- **Análogo:** `internal/domain/types.go:23-39`.
- **Decisões:** `D-05`, `D-06`.

#### Excerto de Código Existente (`internal/domain/types.go:23-39`)
```go
type Target struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Path         string        `json:"path"`
	SizeBytes    int64         `json:"size_bytes"`
	Risk         RiskLevel     `json:"risk"`
	LastAccessed time.Time     `json:"last_accessed"`
	Category     string        `json:"category"`
	AI           *AIAssessment `json:"ai_assessment,omitempty"`
	SmartScore   float64       `json:"smart_score,omitempty"`
}

// IsVirtual returns true if the target represents a virtual resource (e.g. Docker images,
// volumes, build cache) that cannot be manipulated via standard filesystem operations.
func (t Target) IsVirtual() bool {
	return strings.HasPrefix(t.Path, "docker://") || strings.Contains(t.Path, "://")
}
```

#### Aplicação Canônica (`internal/domain/types.go`)
Estender o struct adicionando os campos explicitamente conforme D-05:
```go
type Target struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	Path          string        `json:"path"`
	SizeBytes     int64         `json:"size_bytes"`
	EligibleBytes int64         `json:"eligible_bytes"` // Volume liberável conforme política TTL
	TTLEligible   bool          `json:"ttl_eligible"`   // Qualificado para expurgo automático
	Risk          RiskLevel     `json:"risk"`
	LastAccessed  time.Time     `json:"last_accessed"`
	Category      string        `json:"category"`
	AI            *AIAssessment `json:"ai_assessment,omitempty"`
	SmartScore    float64       `json:"smart_score,omitempty"`
}
```
*Notas de Aplicação:*
- `SizeBytes` permanece como o volume físico real no disco medido por `devfs.CalcDirSize` ou APIs do sistema.
- Se `rules.MaxAgeDays == 0`: `EligibleBytes = SizeBytes` e `TTLEligible = true` (`D-06`).
- Se `rules.MaxAgeDays > 0`: `EligibleBytes` e `TTLEligible` são derivados da comparação de mtimes internos com o corte de data.

---

### 3.2. Configuração & Descoberta: `internal/config/config.go` e `config_test.go`

- **Papel:** Resolução dinâmica determinística dos diretórios de projeto sob `$HOME`.
- **Análogo:** `internal/config/config.go:125-138` e `internal/config/defaults.go:5-8`.
- **Decisões:** `D-09`, `D-10`, `D-11`, `D-12`.

#### Excerto de Código Existente (`internal/config/defaults.go:5-8`)
```go
func Defaults() Config {
	return Config{
		ScanPaths:         []string{},
		ProtectedPatterns: []string{},
```

#### Excerto de Teste Existente (`internal/config/config_test.go:107-113`)
```go
	if len(cfg.ScanPaths) != 0 {
		t.Errorf("ScanPaths should be empty, got %v", cfg.ScanPaths)
	}
```

#### Aplicação Canônica (`internal/config/config.go`)
```go
// CanonicalDiscoveryFolders lista determinística estrita das 7 pastas de projetos sob $HOME (D-09).
var CanonicalDiscoveryFolders = []string{
	"Documents",
	"Projects",
	"Developer",
	"Code",
	"Work",
	"repos",
	"workspace",
}

// EffectiveScanPaths retorna os diretórios ativos de varredura.
// Se ScanPaths estiver preenchido no YAML ou por flag, tem precedência estrita (D-12).
// Caso contrário, descobre as pastas existentes sob $HOME em ordem determinística (D-09, D-10, D-11).
func (c *Config) EffectiveScanPaths() []string {
	if len(c.ScanPaths) > 0 {
		paths := make([]string, 0, len(c.ScanPaths))
		for _, p := range c.ScanPaths {
			if clean := filepath.Clean(p); clean != "." && clean != "" {
				paths = append(paths, clean)
			}
		}
		return paths
	}

	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return []string{}
	}

	var discovered []string
	for _, folder := range CanonicalDiscoveryFolders {
		candidate := filepath.Join(home, folder)
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			if realPath, err := filepath.EvalSymlinks(candidate); err == nil {
				discovered = append(discovered, realPath)
			} else {
				discovered = append(discovered, candidate)
			}
		}
	}

	return discovered
}
```
*Notas de Aplicação:*
- A função **nunca** altera `c.ScanPaths` para evitar persistência indevida na gravação do Viper (`D-11`).
- Se nenhuma das 7 pastas existir fisicamente, retorna `[]string{}` sem causar erro (`D-10`).

---

### 3.3. Primitiva de Filesystem: `internal/fs/stat.go` e `stat_test.go`

- **Papel:** Helper utilitário puro para teste de existência de diretório sem dependência de PATH ou traversal.
- **Análogo:** `internal/fs/stat.go:1-10`.
- **Decisões:** `D-13`, `D-16`.

#### Excerto de Código Existente (`internal/fs/stat.go:1-10`)
```go
package fs

import "os"

// statPath performs an os.Stat. Separated for testability.
func statPath(path string) error {
	_, err := os.Stat(path)
	return err
}
```

#### Aplicação Canônica (`internal/fs/stat.go`)
```go
// DirExists verifica se o caminho existe fisicamente e é um diretório regular/resolvido.
func DirExists(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}
```

---

### 3.4. Motor de Propriedade Canônica: `internal/engine/ownership.go` e `ownership_test.go`

- **Papel:** Centralização da resolução de links simbólicos, deduplicação e dedução de bytes entre cleaners concorrentes ou hierárquicos.
- **Análogo:** `internal/safety/blacklist.go:32-52` e `internal/engine/engine.go:142-156`.
- **Decisões:** `D-01`, `D-02`, `D-03`, `D-04`.

#### Excerto de Código Existente (`internal/cleaner/java/java.go:242-257`)
Mostra a tentativa prévia de desambiguação pontual que agora é centralizada:
```go
		if d.Name() == "target" {
			pomFile := filepath.Join(parentDir, "pom.xml")
			cargoFile := filepath.Join(parentDir, "Cargo.toml")
			// Maven: target/ with pom.xml sibling AND no Cargo.toml (disambiguation)
			if _, err := os.Stat(pomFile); err == nil {
				if _, err := os.Stat(cargoFile); err != nil {
					// No Cargo.toml → Java owns this target/
					found = append(found, localTarget{
						path:      path,
						manifests: []string{pomFile},
						category:  "maven-target",
					})
				}
			}
			return filepath.SkipDir
		}
```

#### Aplicação Canônica (`internal/engine/ownership.go`)
```go
package engine

import (
	"path/filepath"
	"strings"

	"github.com/gtkpad/devclean/internal/domain"
)

type EcosystemTier int

const (
	TierGeneric   EcosystemTier = 1 // macos, system, caches de usuário genéricos
	TierDedicated EcosystemTier = 2 // node, rust, python, bun, dotnet, java, etc.
)

func getEcosystemTier(eco string) EcosystemTier {
	switch strings.ToLower(eco) {
	case "macos", "system", "browsers", "leftovers":
		return TierGeneric
	default:
		return TierDedicated
	}
}

func isDescendant(parent, child string) bool {
	if parent == child {
		return false
	}
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return !strings.HasPrefix(rel, "..") && rel != "."
}

func ResolveCanonicalOwnership(results []domain.ScanResult) []domain.ScanResult {
	type resolvedTarget struct {
		ecoIndex    int
		targetIndex int
		target      *domain.Target
		realPath    string
		isVirtual   bool
		tier        EcosystemTier
	}

	var allTargets []*resolvedTarget
	for rIdx := range results {
		tier := getEcosystemTier(results[rIdx].Ecosystem)
		for tIdx := range results[rIdx].Targets {
			t := &results[rIdx].Targets[tIdx]
			if t.IsVirtual() {
				allTargets = append(allTargets, &resolvedTarget{
					ecoIndex:    rIdx,
					targetIndex: tIdx,
					target:      t,
					realPath:    t.Path,
					isVirtual:   true,
					tier:        tier,
				})
				continue
			}

			cleanPath := filepath.Clean(t.Path)
			realPath, err := filepath.EvalSymlinks(cleanPath)
			if err != nil {
				realPath = cleanPath
			}
			t.Path = realPath

			allTargets = append(allTargets, &resolvedTarget{
				ecoIndex:    rIdx,
				targetIndex: tIdx,
				target:      t,
				realPath:    realPath,
				isVirtual:   false,
				tier:        tier,
			})
		}
	}

	omitted := make(map[*domain.Target]bool)
	seenPaths := make(map[string]*resolvedTarget)

	// Resolução de caminhos idênticos: TierDedicated vence TierGeneric (D-01, D-04)
	for _, rt := range allTargets {
		if rt.isVirtual {
			continue
		}
		if existing, exists := seenPaths[rt.realPath]; exists {
			if rt.tier > existing.tier {
				omitted[existing.target] = true
				seenPaths[rt.realPath] = rt
			} else {
				omitted[rt.target] = true
			}
		} else {
			seenPaths[rt.realPath] = rt
		}
	}

	// Resolução de sobreposição hierárquica (Parent-Child)
	for _, child := range allTargets {
		if child.isVirtual || omitted[child.target] {
			continue
		}
		for _, parent := range allTargets {
			if parent.isVirtual || omitted[parent.target] || child == parent {
				continue
			}
			if isDescendant(parent.realPath, child.realPath) {
				// Deduz bytes do ancestral com clamp protetivo contra negativos
				if parent.target.SizeBytes >= child.target.SizeBytes {
					parent.target.SizeBytes -= child.target.SizeBytes
				} else {
					parent.target.SizeBytes = 0
				}

				if parent.target.EligibleBytes >= child.target.EligibleBytes {
					parent.target.EligibleBytes -= child.target.EligibleBytes
				} else {
					parent.target.EligibleBytes = 0
				}
			}
		}
	}

	// Reconstrói a lista sem os alvos subordinados absorvidos
	filteredResults := make([]domain.ScanResult, len(results))
	for rIdx, r := range results {
		filteredResults[rIdx] = domain.ScanResult{
			Ecosystem: r.Ecosystem,
			Error:     r.Error,
			Targets:   make([]domain.Target, 0, len(r.Targets)),
		}
		for tIdx := range r.Targets {
			t := r.Targets[tIdx]
			if !omitted[&r.Targets[tIdx]] {
				filteredResults[rIdx].Targets = append(filteredResults[rIdx].Targets, t)
			}
		}
	}

	return filteredResults
}
```

---

### 3.5. Integração no Motor: `internal/engine/register.go` e `engine.go`

- **Papel:** Injetar os diretórios efetivos e conectar a resolução de propriedade.
- **Análogo:** `internal/engine/register.go:33-63` e `internal/engine/engine.go:142-156`.

#### Excerto de Código Existente (`internal/engine/register.go:40-48`)
```go
	// Ecossistemas base de linguagens e containers
	eng.Register(docker.New())
	eng.Register(golang.New())
	eng.Register(node.New(cfg.ScanPaths))
	eng.Register(dotnet.New(paths, cfg.ScanPaths))
	eng.Register(python.New(paths.PythonUVCache, cfg.ScanPaths))
	eng.Register(rust.New(cfg.ScanPaths))
	eng.Register(java.New(cfg.ScanPaths))
```

#### Aplicação Canônica (`internal/engine/register.go`)
Substituir o repasse direto de `cfg.ScanPaths` por `cfg.EffectiveScanPaths()`:
```go
	effectivePaths := cfg.EffectiveScanPaths()

	eng.Register(docker.New())
	eng.Register(golang.New())
	eng.Register(node.New(effectivePaths))
	eng.Register(dotnet.New(paths, effectivePaths))
	eng.Register(python.New(paths.PythonUVCache, effectivePaths))
	eng.Register(rust.New(effectivePaths))
	eng.Register(java.New(effectivePaths))
	// Demais cleaners atualizados analogamente...
```

---

### 3.6. Cleaners: Padrões de Desacoplamento Scan vs. TTL e Detecção Híbrida

#### 3.6.1. Exemplo de Antipattern a ser Eliminado: `internal/cleaner/bun/bun.go:55-58`
```go
	cutoff := time.Now().AddDate(0, 0, -rules.MaxAgeDays)
	if rules.MaxAgeDays > 0 && info.ModTime().After(cutoff) {
		return nil, nil // ANTIPATTERN: Oculta o alvo completamente do inventário!
	}
```

#### 3.6.2. Aplicação Canônica para Cleaners Globais (`bun`, `golang`, `dotnet`, `python`, etc.)
```go
	size, _ := devfs.CalcDirSize(c.cachePath)
	if size == 0 {
		return nil, nil
	}

	cutoff := time.Now().AddDate(0, 0, -rules.MaxAgeDays)
	lastAccessed := info.ModTime()

	eligibleBytes := int64(0)
	ttlEligible := false

	if rules.MaxAgeDays == 0 || lastAccessed.Before(cutoff) {
		eligibleBytes = size
		ttlEligible = true
	}

	return []domain.Target{
		{
			ID:            id,
			Name:          "Bun package cache",
			Path:          c.cachePath,
			SizeBytes:     size,          // Tamanho real físico reportado
			EligibleBytes: eligibleBytes, // Elegibilidade calculada
			TTLEligible:   ttlEligible,   // Flag de elegibilidade TTL
			Risk:          domain.RiskLow,
			LastAccessed:  lastAccessed,
			Category:      "bun-cache",
		},
	}, nil
```

#### 3.6.3. Aplicação Canônica para `IsInstalled()` Híbrido (`D-13` a `D-16`)
Exemplo baseado em `internal/cleaner/dotnet/dotnet.go:38-41`:
```go
// Antes:
func (c *dotnetCleaner) IsInstalled() bool {
	_, err := exec.LookPath("dotnet")
	return err == nil
}

// Depois:
func (c *dotnetCleaner) IsInstalled() bool {
	// 1. Binário no PATH
	if _, err := exec.LookPath("dotnet"); err == nil {
		return true
	}
	// 2. Presença de caches globais no disco
	if devfs.DirExists(c.paths.DotNetNuGet) || devfs.DirExists(c.paths.DotNetHTTP) {
		return true
	}
	// 3. Checagem rasa de manifestos em ScanPaths (D-15, D-16)
	for _, root := range c.scanPaths {
		if hasShallowDotNetManifest(root) {
			return true
		}
	}
	return false
}
```

---

### 3.7. CLI & Comandos: `cmd/scan.go`, `cmd/format.go` e `cmd/clean.go`

- **Papel:** Exibir o volume total real no disco e consumir alvos elegíveis no expurgo automático.
- **Análogo:** `cmd/clean.go:132-137, 334-347` e `cmd/format.go:25-68`.

#### Excerto de Código Existente (`cmd/clean.go:334-347`)
```go
func filterByTTL(targets []domain.Target, maxAgeDays int) []domain.Target {
	if maxAgeDays == 0 {
		return targets
	}

	cutoff := time.Now().Add(-time.Duration(maxAgeDays) * 24 * time.Hour)
	var filtered []domain.Target
	for _, t := range targets {
		if t.LastAccessed.Before(cutoff) {
			filtered = append(filtered, t)
		}
	}
	return filtered
}
```

#### Aplicação Canônica (`cmd/clean.go`)
No modo `devclean clean --auto`, os alvos passam a ser filtrados diretamente com base em `t.TTLEligible`:
```go
				if auto {
					var eligible []domain.Target
					for _, t := range r.Targets {
						if t.TTLEligible {
							eligible = append(eligible, t)
						}
					}
					targets = eligible
				} else {
					targets = r.Targets
				}
```

---

## 4. Shared Patterns

### 4.1. Zero CGO Discipline (`CGO_ENABLED=0`)
- **Regra:** Nenhuma biblioteca de C bindings (como dependências em `sqlite3` com CGO, `libmagic`, `posix-regex` em C) é permitida.
- **Implementação:** Toda a manipulação de caminhos, resolução de links e leitura de atributos de arquivo deve utilizar estritamente os pacotes da biblioteca padrão de Go: `os`, `path/filepath`, `time`, `sync`, `runtime`.
- **Garantia de Compilação:** `go build -tags netgo -ldflags="-extldflags=-static" ./...` e validação nos scripts de build e CI.

### 4.2. Tratamento de Erros & Degradação Graciosa
- **Symlinks Quebrados ou Circulares:** Se `filepath.EvalSymlinks` retornar erro (`os.ErrNotExist` para symlinks órfãos ou `syscall.ELOOP` para links circulares), o sistema não entra em pânico nem aborta o scan: utiliza `filepath.Clean(path)` como fallback gracioso e prossegue o processamento.
- **Permissões Negadas (`os.ErrPermission`):** O cálculo de tamanhos de diretório (`devfs.CalcDirSize`) já ignora arquivos e pastas inacessíveis sem abortar a medição dos demais ramos (`fs/size.go:19-20`). O mesmo padrão deve ser respeitado em verificações de `DirExists`.
- **Dedução Defensiva sem Underflow:** Em `ownership.go`, antes de subtrair bytes de um alvo ancestral (`parent.SizeBytes -= child.SizeBytes`), aplicar verificação defensiva de limite (`if parent.SizeBytes < child.SizeBytes { parent.SizeBytes = 0 }`) para impedir inteiros negativos ou overflows.

### 4.3. Concorrência e Streaming via Canais
- **Semáforos Bounded Concurrency:** Toda varredura de múltiplos `ScanPaths` continua protegida pelo semáforo `make(chan struct{}, runtime.NumCPU())` presente em `dotnet.go`, `node.go`, `rust.go`, `java.go` e `fs/size.go`.
- **Isolamento de Fase:** A deduplicação canônica de alvos (`ResolveCanonicalOwnership`) **nunca** é executada inline concorrentemente no fluxo do canal `Scan()`. Ela atua como um estágio atômico sobre o slice unificado drenado por `engine.Collect(ctx, ch)` ou em wrapper dedicado antes da entrega para a UI.

### 4.4. Segurança Inviolável contra Blacklists
- **Lista Negra do Sistema:** Nenhuma resolução de link simbólico em `ownership.go` ou descoberta automática em `EffectiveScanPaths()` pode contornar a validação de `safety.IsCoreBlacklisted(realPath)` (`/`, `/System`, `/Library`, `/usr`, `~/.ssh`, `~/.env`, `.git`).
- **Imutabilidade de Configuração:** `EffectiveScanPaths()` opera como cálculo puro em memória (`value receiver` ou retorno de novo slice), mantendo `cfg.ScanPaths` em seu estado original não poluído.

---

## 5. Test Pattern Matrix

| Alvo de Teste | Arquivo de Teste | Padrão Utilizado | Casos Críticos Cobertos |
| :--- | :--- | :--- | :--- |
| `domain.Target` TTL Fields | `internal/domain/types_test.go` | JSON Marshaling / Unmarshaling | Serialização de `EligibleBytes` e `TTLEligible`; compatibilidade retroativa com payloads sem os novos campos. |
| `EffectiveScanPaths` | `internal/config/config_test.go` | `t.TempDir()`, simulação de pastas via `os.Mkdir` | 1. Ordem determinística exata das 7 pastas.<br>2. Precedência absoluta de `ScanPaths` configurado.<br>3. Fallback vazio seguro se `$HOME` não tiver nenhuma das pastas.<br>4. Garantia de não mutação de `cfg.ScanPaths`. |
| `DirExists` | `internal/fs/stat_test.go` | Hermético com arquivos e diretórios | Retorno `true` para diretório real; `false` para arquivo comum, caminho inexistente e symlink quebrado. |
| `ResolveCanonicalOwnership` | `internal/engine/ownership_test.go` | Fixtures de `domain.ScanResult` com alvos fictícios | 1. `TierDedicated` absorve `TierGeneric` em mesmo caminho físico.<br>2. Dedução correta de bytes de alvos descendentes sem overflow negativo.<br>3. Alvos subordinados absorvidos são omitidos.<br>4. Alvos virtuais (`docker://`, `brew://`) permanecem intactos. |
| Cleaners Scan sem Cutoff | `internal/cleaner/*/*_test.go` | Manipulação de mtime via `os.Chtimes` | Diretório com `mtime` recente (<24h) continua reportando alvo com `SizeBytes > 0`, `TTLEligible = false` e `EligibleBytes = 0`. |
| Cleaners Detecção Híbrida | `internal/cleaner/*/*_test.go` | Mock executor / `LookPath` customizado | `IsInstalled() == true` quando o binário está ausente do PATH mas a pasta de cache físico existe no disco. |

---

## PATTERN MAPPING COMPLETE

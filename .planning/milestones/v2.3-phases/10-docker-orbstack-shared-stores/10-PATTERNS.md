# Phase 10: Docker, OrbStack & Shared Stores - Pattern Mapping

**Phase:** 10-docker-orbstack-shared-stores  
**Milestone:** v2.3 Functional Parity with Mole (tw93/mole)  
**Date:** 2026-10-16  
**Status:** Approved for Implementation  
**Files Analyzed:** 15  
**Analogs Found:** 15 / 15  

---

## 1. Overview & Pattern Mapping Summary

Este documento estabelece o mapeamento estrutural entre os arquivos a serem criados e modificados na **Fase 10** e os análogos consolidados na base de código do DevClean (`internal/cleaner/xcode/xcode.go`, `internal/cleaner/pnpm/pnpm.go`, `internal/cleaner/ai/ai.go`, `internal/cleaner/dotnet/dotnet.go`, `internal/fs/size.go`, `internal/fs/statfs_darwin.go`, `internal/fs/statfs_linux.go`, `internal/domain/types.go`, `internal/deleter/policy_test.go`, etc.).

O objetivo da Fase 10 é entregar paridade funcional com utilitários avançados de limpeza e diagnóstico de disco (como o Mole, `tw93/mole`), cobrindo containers (`Docker`, `OrbStack`) e package stores compartilhados (`NuGet`, `pnpm store`), enquanto aplica estritamente a filosofia inviolável do DevClean: **visibilidade ampla de inventário jamais deve ser confundida com autorização para deleção**.

A fase resolve cinco desafios arquiteturais fundamentais:
1. **Contabilidade de Arquivos Esparsos e Imagens de VM (`STORE-01`, `STORE-02`, `D-07`):** Discos virtuais brutos (`Docker.raw`, `data.img.raw`) reportam tamanhos lógicos aparentes massivos (ex: 494 GB no OrbStack, 64 GB no Docker Desktop) enquanto alocam fisicamente apenas uma fração disso (ex: 5.1 GB) no APFS ou ext4. Somar tamanho aparente como espaço recuperável geraria alarme falso e pânico no usuário. A Fase 10 cria uma camada agnóstica a CGO em `internal/fs` (`usage.go`, `usage_darwin.go`, `usage_linux.go`, `usage_other.go`) que extrai blocos físicos reais (`stat.Blocks * 512`) e tamanho aparente (`stat.Size`), deduplicando hardlinks por tupla `(Dev, Ino)`.
2. **Docker 100% ReviewOnly e Alvos Categorizados (`STORE-01`, `STORE-03`, `D-01`, `D-02`, `D-03`, `D-04`):** Padronização de `internal/cleaner/docker` sobre `cleaner.CommandExecutor` (eliminando a dependência pesada do SDK do Docker e viabilizando testes herméticos). O comando `docker system df --format "{{json .}}"` é interpretado para emitir 1 alvo por categoria (`Images`, `Containers`, `Local Volumes`, `Build Cache`) sob o esquema virtual `docker://system-df/<type>`. Todos os alvos do Docker são marcados incondicionalmente com `ReviewOnly: true`, `ReviewReason: domain.ReviewReasonSharedStore`, `EligibleBytes: 0` e `TTLEligible: false`. O arquivo de VM do Docker Desktop no host (`Docker.raw`) é emitido separadamente como alvo de host (`docker-vm-disk`), nunca misturado às métricas lógicas (`D-15`).
3. **OrbStack: Cache Regenerável vs Armazenamento de VM/Data e Process Guard (`STORE-02`, `D-05`, `D-06`, `D-08`):** Integração do OrbStack dentro do pacote `internal/cleaner/docker` sob o ecossistema `"docker"`. Separação estrita entre `OrbStack Cache` (`~/Library/Caches/dev.orbstack.OrbStack` no macOS e `~/.cache/orbstack` no Linux) como cache regenerável elegível para limpeza (`RiskLow` quando inativo) e `OrbStack VM/Data` (`~/Library/Group Containers/*.dev.orbstack/data`, `~/.orbstack`) como armazenamento de containers/VM 100% `ReviewOnly`. Aplicação de Process Guard fail-closed (`pgrep -x OrbStack`) com revalidação pré-deleção no `Clean()`.
4. **Shared Stores de Pacotes: NuGet e pnpm store (`STORE-03`, `D-09`, `D-10`, `D-11`):** Marcação de `~/.nuget/packages` (`nuget-packages`) como estritamente `ReviewOnly: true` (`ReviewReasonSharedStore`), preservando a elegibilidade de `nuget-http-cache`, `nuget-temp` e `dotnet-build`. Descoberta híbrida do `pnpm store` (via `pnpm store path` e caminhos canônicos em disco), travando o store como `ReviewOnly` com medição deduplicada por inodes, removendo o alvo virtual `pnpm://store/prune` e eliminando chamadas a `pnpm store prune` no `Clean()`.
5. **Defesa em Duas Camadas e Fail-Closed em Falhas de Daemon (`D-12`, `D-14`, `D-16`):** Além do bloqueio no Estágio 1 do `deleter.Coordinator`, os métodos `Clean()` de `docker`, `dotnet` e `pnpm` possuem guardas diretas que recusam mutação em qualquer alvo `ReviewOnly` ou pertencente a shared stores. Em caso de daemon do Docker desligado ou com falha, o DevClean emite um alvo explícito de revisão (`docker-daemon-status`, `ReviewReasonAmbiguous`), nunca ocultando a presença do Docker como 0 bytes silenciosos.

---

## 2. File Classification Table

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `internal/domain/types.go` | model / domain | declarative struct & target field | `internal/domain/types.go:28-48` | exact |
| `internal/domain/target_test.go` | unit test | struct serialization & validation | `internal/domain/target_test.go:57-85` | exact |
| `internal/fs/usage.go` | service / fs abstraction | disk-walk, sparse calc, inode dedup | `internal/fs/size.go:11-31` | high |
| `internal/fs/usage_darwin.go` | platform / sys extractor | Darwin `syscall.Stat_t` (zero CGO) | `internal/fs/statfs_darwin.go:1-25` | exact |
| `internal/fs/usage_linux.go` | platform / sys extractor | Linux `syscall.Stat_t` (zero CGO) | `internal/fs/statfs_linux.go:1-25` | exact |
| `internal/fs/usage_other.go` | platform / sys extractor | Fallback `os.FileInfo` (zero CGO) | `internal/fs/statfs_other.go:1-10` | exact |
| `internal/fs/usage_test.go` | unit test (new) | sparse accounting & inode fixtures | `internal/fs/size_test.go`<br>`internal/cleaner/xcode/xcode_test.go` | high |
| `internal/cleaner/docker/docker.go` | service / cleaner | CommandExecutor, JSON parse, Clean guard | `internal/cleaner/xcode/xcode.go:21-43`<br>`internal/cleaner/pnpm/pnpm.go:18-30, 131-151` | exact |
| `internal/cleaner/docker/orbstack.go` | service / sub-scanner | Process Guard & sparse VM discovery | `internal/cleaner/ai/editors.go:287-306`<br>`internal/cleaner/ai/claude.go:437-515` | exact |
| `internal/cleaner/docker/docker_test.go` | unit test | mock executor, df parsing, defensive Clean | `internal/cleaner/pnpm/pnpm_test.go:23-91`<br>`internal/cleaner/xcode/xcode_test.go:15-60` | exact |
| `internal/cleaner/docker/orbstack_test.go` | unit test (new) | mock executor, pgrep states, VM data | `internal/cleaner/ai/editors_test.go:15-50`<br>`internal/cleaner/ai/process_test.go` | exact |
| `internal/cleaner/dotnet/dotnet.go` | service / cleaner | shared store ReviewOnly & Clean guard | `internal/cleaner/dotnet/dotnet.go:62-117, 195-202` | exact |
| `internal/cleaner/dotnet/dotnet_test.go` | unit test | ReviewOnly nuget-packages assertion | `internal/cleaner/dotnet/dotnet_test.go:21-56` | exact |
| `internal/cleaner/pnpm/pnpm.go` | service / cleaner | hybrid discovery, inode dedup, no prune | `internal/cleaner/pnpm/pnpm.go:36-151` | exact |
| `internal/cleaner/pnpm/pnpm_test.go` | unit test | hybrid discovery & sentinel preservation | `internal/cleaner/pnpm/pnpm_test.go:23-91, 140-180` | exact |
| `internal/engine/register.go` | registry / bootstrapper | dependency injection | `internal/engine/register.go:34-66` | exact |
| `internal/deleter/policy_test.go` | integration test | zero-mutation canary (Phase 10) | `internal/deleter/policy_test.go:1140-1285` | exact |

---

## 3. Pattern Assignments & Concrete Code Excerpts

### 3.1. Domain Model: Structured ProviderMetrics & Target Field (`internal/domain/types.go`)

- **Role:** Modelo de domínio declarativo e contrato de metadados para provedores de containers e armazenamento virtual.
- **Data Flow:** Preenchido pelos scanners de Docker (`docker.go`), OrbStack (`orbstack.go`) e shared stores (`pnpm.go`), consumido pela CLI, TUI, servidor MCP e serializadores JSON.
- **Closest Analog:** `internal/domain/types.go:28-48` (`AIAssessment` e `Target`).
- **Decisions Enforced:** `D-03`, `STORE-01`, `STORE-02`.

#### Existing Code Excerpt (`internal/domain/types.go:28-48`)
```go
type AIAssessment struct {
	ConfidenceScore float64 `json:"confidence_score"`
	Action          string  `json:"action"`
	Reasoning       string  `json:"reasoning"`
}

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
	AI            *AIAssessment `json:"ai_assessment,omitempty"`
	SmartScore    float64       `json:"smart_score,omitempty"`
}
```

#### Concrete Adaptation Pattern for Phase 10
```go
// ProviderMetrics contains structured metrics for container engines, virtual disks,
// and shared package stores (STORE-01, STORE-02, D-03).
type ProviderMetrics struct {
	TotalCount       int    `json:"total_count,omitempty"`
	ActiveCount      int    `json:"active_count,omitempty"`
	SizeBytes        int64  `json:"size_bytes,omitempty"`
	ReclaimableBytes int64  `json:"reclaimable_bytes,omitempty"`
	ApparentBytes    int64  `json:"apparent_bytes,omitempty"`
	AllocatedBytes   int64  `json:"allocated_bytes,omitempty"`
	Partial          bool   `json:"partial,omitempty"`
	StatusNote       string `json:"status_note,omitempty"`
}

type Target struct {
	ID              string           `json:"id"`
	Name            string           `json:"name"`
	Path            string           `json:"path"`
	SizeBytes       int64            `json:"size_bytes"`
	EligibleBytes   int64            `json:"eligible_bytes"`
	TTLEligible     bool             `json:"ttl_eligible"`
	ReviewOnly      bool             `json:"review_only"`
	ReviewReason    string           `json:"review_reason,omitempty"`
	Risk            RiskLevel        `json:"risk"`
	LastAccessed    time.Time        `json:"last_accessed"`
	Category        string           `json:"category"`
	AI              *AIAssessment    `json:"ai_assessment,omitempty"`
	SmartScore      float64          `json:"smart_score,omitempty"`
	ProviderMetrics *ProviderMetrics `json:"provider_metrics,omitempty"`
}
```

---

### 3.2. Platform-Native Sparse Accounting & Inode Deduplication (`internal/fs`)

- **Role:** Camada de infraestrutura de sistema de arquivos multiplataforma com zero CGO.
- **Data Flow:** Recebe caminhos de arquivos ou árvores de diretórios; extrai o uso real de blocos (`Blocks * 512`) e tamanho aparente (`Size`), deduplicando hardlinks por tupla `(Dev, Ino)` e reportando falhas parciais de leitura sem interromper o scan de forma fail-closed.
- **Closest Analog:** `internal/fs/size.go:11-31` e `internal/fs/statfs_darwin.go:1-25`.
- **Decisions Enforced:** `D-07`, `D-11`, `D-16`, `STORE-02`.

#### Existing Code Excerpt (`internal/fs/size.go:11-31`)
```go
func CalcDirSize(path string) (int64, error) {
	var total int64
	err := filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if p == path {
				return err // root path inaccessible — propagate
			}
			return nil // skip inaccessible entries within the tree
		}
		if !d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return nil
			}
			total += info.Size()
		}
		return nil
	})
	return total, err
}
```

#### Concrete Adaptation Pattern for Phase 10 (`internal/fs/usage.go`)
```go
package fs

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
)

// Usage captures apparent vs physically allocated byte usage, file counts,
// and partial degradation states for sparse files and hardlinked stores.
type Usage struct {
	ApparentBytes  int64
	AllocatedBytes int64
	FileCount      int
	Partial        bool
	StatusNote     string
}

// CalcUsage calculates disk usage for a single file or directory tree.
func CalcUsage(path string) (Usage, error) {
	return CalcUsageWithContext(context.Background(), path)
}

// CalcUsageWithContext traverses the path with context cancellation support.
func CalcUsageWithContext(ctx context.Context, path string) (Usage, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return Usage{Partial: true, StatusNote: err.Error()}, err
	}

	// Single file case (e.g. Docker.raw or data.img.raw)
	if !info.IsDir() {
		dev, ino, blocks, apparent, ok := getFileInfoStat(info)
		if !ok {
			return Usage{ApparentBytes: info.Size(), AllocatedBytes: info.Size(), FileCount: 1}, nil
		}
		alloc := blocks * 512
		return Usage{ApparentBytes: apparent, AllocatedBytes: alloc, FileCount: 1}, nil
	}

	// Directory tree traversal with inode deduplication
	seenInodes := make(map[[2]uint64]bool)
	var apparentTotal, allocatedTotal int64
	var fileCount int
	var partial bool
	var statusNote string

	err = filepath.WalkDir(path, func(p string, d fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			if p == path {
				return walkErr
			}
			partial = true
			statusNote = "permission or access error on subpath: " + walkErr.Error()
			return nil
		}

		if d.IsDir() {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil // Skip symlinks to prevent duplicate accounting or directory escape
		}

		fi, err := d.Info()
		if err != nil {
			partial = true
			return nil
		}

		dev, ino, blocks, apparent, ok := getFileInfoStat(fi)
		if ok && dev != 0 && ino != 0 {
			key := [2]uint64{dev, ino}
			if seenInodes[key] {
				return nil // Hardlink duplicate: do not sum bytes again
			}
			seenInodes[key] = true
			allocatedTotal += blocks * 512
			apparentTotal += apparent
		} else {
			allocatedTotal += fi.Size()
			apparentTotal += fi.Size()
		}
		fileCount++
		return nil
	})

	return Usage{
		ApparentBytes:  apparentTotal,
		AllocatedBytes: allocatedTotal,
		FileCount:      fileCount,
		Partial:        partial,
		StatusNote:     statusNote,
	}, err
}
```

#### Darwin Extractor (`internal/fs/usage_darwin.go`)
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

#### Linux Extractor (`internal/fs/usage_linux.go`)
```go
//go:build linux

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
	// On Linux, Stat_t.Dev is uint64
	return stat.Dev, stat.Ino, stat.Blocks, stat.Size, true
}
```

#### Fallback Extractor (`internal/fs/usage_other.go`)
```go
//go:build !darwin && !linux

package fs

import "os"

func getFileInfoStat(info os.FileInfo) (dev uint64, ino uint64, blocks int64, apparent int64, ok bool) {
	return 0, 0, 0, info.Size(), false
}
```

---

### 3.3. Docker Cleaner & Categorized Targets (`internal/cleaner/docker/docker.go`)

- **Role:** Cleaner do ecossistema `"docker"` padronizado sobre `cleaner.CommandExecutor`.
- **Data Flow:** Executa `docker system df --format "{{json .}}"`, descobre o arquivo de VM do Docker Desktop (`Docker.raw`), delega varredura para o sub-scanner do OrbStack, emite alvos virtuais `docker://system-df/<type>` categorizados e 100% `ReviewOnly`. Na presença de falha do daemon, emite `docker-daemon-status` como estado ambíguo (`ReviewReasonAmbiguous`). No `Clean()`, rejeita mutações em alvos `ReviewOnly` ou virtuais.
- **Closest Analog:** `internal/cleaner/xcode/xcode.go:21-43` (CommandExecutor) e `internal/cleaner/pnpm/pnpm.go:131-151` (Clean guards).
- **Decisions Enforced:** `D-01`, `D-02`, `D-03`, `D-04`, `D-12`, `D-13`, `D-14`, `D-15`, `STORE-01`, `STORE-03`.

#### Existing Code Excerpt (`internal/cleaner/docker/docker.go:49-62`)
```go
// New creates a Docker cleaner using the Docker SDK.
func New() domain.Cleaner {
	return &dockerCleaner{
		api: &sdkAPI{},
	}
}

func (c *dockerCleaner) Name() string { return "docker" }

func (c *dockerCleaner) IsInstalled() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return c.api.ping(ctx) == nil
}
```

#### Concrete Adaptation Pattern for Phase 10 (`internal/cleaner/docker/docker.go`)
```go
package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gtkpad/devclean/internal/cleaner"
	"github.com/gtkpad/devclean/internal/domain"
	devfs "github.com/gtkpad/devclean/internal/fs"
	"github.com/gtkpad/devclean/internal/safety"
)

type dockerCleaner struct {
	exec cleaner.CommandExecutor
}

// New creates a Docker cleaner with default CommandExecutor.
func New() domain.Cleaner {
	return NewWithExecutor(&cleaner.DefaultExecutor{})
}

// NewWithExecutor creates a Docker cleaner with injected CommandExecutor for hermetic testing.
func NewWithExecutor(exec cleaner.CommandExecutor) domain.Cleaner {
	return &dockerCleaner{exec: exec}
}

func (c *dockerCleaner) Name() string { return "docker" }

// IsInstalled checks LookPath for "docker" or "orb", OR inspects candidate state/cache directories on disk (D-13, SCAN-02).
func (c *dockerCleaner) IsInstalled() bool {
	if _, err := c.exec.LookPath("docker"); err == nil {
		return true
	}
	if _, err := c.exec.LookPath("orb"); err == nil {
		return true
	}

	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return false
	}

	candidates := []string{
		filepath.Join(home, ".docker"),
		filepath.Join(home, "Library", "Containers", "com.docker.docker"),
		filepath.Join(home, "Library", "Group Containers", "group.dev.orbstack"),
		filepath.Join(home, "Library", "Caches", "dev.orbstack.OrbStack"),
		filepath.Join(home, ".orbstack"),
		filepath.Join(home, ".cache", "orbstack"),
	}

	for _, dir := range candidates {
		if devfs.DirExists(dir) {
			return true
		}
	}
	return false
}

func (c *dockerCleaner) Scan(ctx context.Context, rules domain.EcosystemRules) ([]domain.Target, error) {
	var targets []domain.Target

	// 1. Query docker system df via CommandExecutor (D-01, D-04)
	out, err := c.exec.Output(ctx, "docker", "system", "df", "--format", "{{json .}}")
	if err != nil {
		// Daemon failure or unreachable: emit explicit ReviewReasonAmbiguous status target (D-14)
		if c.isDockerCLIAvailable() {
			targets = append(targets, domain.Target{
				ID:            "docker-daemon-status",
				Name:          "Docker daemon (unavailable / stopped)",
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
					StatusNote: fmt.Sprintf("docker system df failed: %v", err),
				},
			})
		}
	} else {
		parsedTargets, hasMalformed := parseSystemDFOutput(out)
		targets = append(targets, parsedTargets...)
		if hasMalformed {
			targets = append(targets, domain.Target{
				ID:            "docker-daemon-parse-error",
				Name:          "Docker system df (partial / malformed output)",
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
					StatusNote: "docker system df returned malformed JSON lines",
				},
			})
		}
	}

	// 2. Discover Docker Desktop VM Disk on Host (D-15)
	if vmTarget, found := c.scanDockerVMDisk(); found {
		targets = append(targets, vmTarget)
	}

	// 3. Sub-scan OrbStack targets under the "docker" ecosystem (D-05, D-06)
	orbTargets, orbErr := scanOrbStack(ctx, c.exec, rules)
	if orbErr == nil {
		targets = append(targets, orbTargets...)
	}

	return targets, nil
}

func (c *dockerCleaner) isDockerCLIAvailable() bool {
	_, err := c.exec.LookPath("docker")
	return err == nil
}

func (c *dockerCleaner) scanDockerVMDisk() (domain.Target, bool) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return domain.Target{}, false
	}

	candidates := []string{
		filepath.Join(home, "Library", "Containers", "com.docker.docker", "Data", "vms", "0", "data", "Docker.raw"),
		filepath.Join(home, ".docker", "desktop", "vms", "0", "data", "Docker.raw"),
	}

	for _, p := range candidates {
		info, err := os.Stat(p)
		if err != nil {
			continue
		}

		usage, _ := devfs.CalcUsage(p)
		size := usage.AllocatedBytes
		if size == 0 {
			size = info.Size()
		}

		name := fmt.Sprintf("Docker Desktop VM Disk (allocated: %s, apparent: %s)",
			formatBytes(usage.AllocatedBytes), formatBytes(usage.ApparentBytes))

		return domain.Target{
			ID:            "docker-vm-disk",
			Name:          name,
			Path:          p,
			SizeBytes:     size,
			EligibleBytes: 0, // D-02: 100% ReviewOnly
			TTLEligible:   false,
			ReviewOnly:    true,
			ReviewReason:  domain.ReviewReasonSharedStore,
			Risk:          domain.RiskLow,
			LastAccessed:  info.ModTime(),
			Category:      "docker-vm-disk",
			ProviderMetrics: &domain.ProviderMetrics{
				SizeBytes:      size,
				AllocatedBytes: usage.AllocatedBytes,
				ApparentBytes:  usage.ApparentBytes,
				StatusNote:     "physical host VM disk image",
			},
		}, true
	}

	return domain.Target{}, false
}

// Clean enforces direct defensive guards against deleting ReviewOnly, VM disk or daemon targets (D-02, D-12).
func (c *dockerCleaner) Clean(ctx context.Context, targets []domain.Target) error {
	for _, t := range targets {
		if t.ReviewOnly || t.Category == "docker-vm-disk" || strings.HasPrefix(t.Path, "docker://") {
			continue // INVIOLABLE: never delete shared stores, VM disks or virtual daemon resources
		}

		// Revalidate and clean OrbStack regenerable cache (D-08)
		if t.Category == "orbstack-cache" {
			running, uncertain := checkOrbStackProcess(ctx, c.exec)
			if running || uncertain {
				continue // Abort deleting cache if process launched post-scan
			}
			if !safety.IsCoreBlacklisted(t.Path) {
				_ = os.RemoveAll(t.Path)
			}
		}
	}
	return nil
}
```

---

### 3.4. OrbStack Sub-Scanner, Cache vs VM/Data & Process Guard (`internal/cleaner/docker/orbstack.go`)

- **Role:** Sub-módulo especializado na detecção do OrbStack integrado em `internal/cleaner/docker`.
- **Data Flow:** Executa Process Guard via `pgrep -x OrbStack`; varre `OrbStack Cache` (regenerável) e `OrbStack VM/Data` (armazenamento persistente de VM); calcula uso com sparse accounting (`CalcUsage`); emite alvos com semântica estrita de `ReviewOnly` e revalida o processo no `Clean()`.
- **Closest Analog:** `internal/cleaner/ai/editors.go:287-306` (Process Guard) e `internal/cleaner/ai/claude.go:437-515` (fail-closed resolution).
- **Decisions Enforced:** `D-05`, `D-06`, `D-07`, `D-08`, `STORE-02`, `STORE-03`.

#### Concrete Adaptation Pattern for Phase 10 (`internal/cleaner/docker/orbstack.go`)
```go
package docker

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gtkpad/devclean/internal/cleaner"
	"github.com/gtkpad/devclean/internal/domain"
	devfs "github.com/gtkpad/devclean/internal/fs"
)

func checkOrbStackProcess(ctx context.Context, exec cleaner.CommandExecutor) (running bool, uncertain bool) {
	if exec == nil {
		return false, true
	}
	out, err := exec.Output(ctx, "pgrep", "-x", "OrbStack")
	if err == nil && len(strings.TrimSpace(string(out))) > 0 {
		return true, false
	}
	var exitErr interface{ ExitCode() int }
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return false, false // Cleanly not running
	}
	// Any unexpected error or tool absence -> uncertain -> FAIL CLOSED!
	return false, true
}

func scanOrbStack(ctx context.Context, exec cleaner.CommandExecutor, rules domain.EcosystemRules) ([]domain.Target, error) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return nil, nil
	}

	var targets []domain.Target
	running, uncertain := checkOrbStackProcess(ctx, exec)
	isProcessBlocked := running || uncertain

	// 1. Regenerable OrbStack Caches (D-06, D-08)
	cacheCandidates := []string{
		filepath.Join(home, "Library", "Caches", "dev.orbstack.OrbStack"),
		filepath.Join(home, ".cache", "orbstack"),
	}

	cutoff := time.Now().AddDate(0, 0, -rules.MaxAgeDays)

	for _, cPath := range cacheCandidates {
		info, err := os.Stat(cPath)
		if err != nil {
			continue
		}

		size, _ := devfs.CalcDirSize(cPath)
		if size == 0 {
			continue
		}

		sum := sha256.Sum256([]byte(cPath))
		id := fmt.Sprintf("orbstack-cache-%x", sum[:4])

		target := domain.Target{
			ID:           id,
			Name:         "OrbStack Cache",
			Path:         cPath,
			SizeBytes:    size,
			Risk:         domain.RiskLow,
			LastAccessed: info.ModTime(),
			Category:     "orbstack-cache",
		}

		if isProcessBlocked {
			target.ReviewOnly = true
			if running {
				target.ReviewReason = domain.ReviewReasonActiveProcess
			} else {
				target.ReviewReason = domain.ReviewReasonAmbiguous
			}
			target.EligibleBytes = 0
			target.TTLEligible = false
		} else {
			if rules.MaxAgeDays == 0 || info.ModTime().Before(cutoff) {
				target.TTLEligible = true
				target.EligibleBytes = size
			}
		}

		targets = append(targets, target)
	}

	// 2. OrbStack VM/Data Storage (Incondicionalmente ReviewOnly, D-06, D-07)
	vmCandidates := []string{
		filepath.Join(home, "Library", "Group Containers", "group.dev.orbstack", "data"),
		filepath.Join(home, ".orbstack", "data"),
		filepath.Join(home, ".orbstack"),
	}

	for _, vPath := range vmCandidates {
		info, err := os.Stat(vPath)
		if err != nil {
			continue
		}

		usage, _ := devfs.CalcUsage(vPath)
		if usage.AllocatedBytes == 0 && usage.ApparentBytes == 0 {
			continue
		}

		size := usage.AllocatedBytes
		if size == 0 {
			size = info.Size()
		}

		name := fmt.Sprintf("OrbStack VM/Data (allocated: %s, apparent: %s)",
			formatBytes(usage.AllocatedBytes), formatBytes(usage.ApparentBytes))

		sum := sha256.Sum256([]byte(vPath))
		id := fmt.Sprintf("orbstack-data-%x", sum[:4])

		target := domain.Target{
			ID:            id,
			Name:          name,
			Path:          vPath,
			SizeBytes:     size,
			EligibleBytes: 0, // D-06: 100% ReviewOnly
			TTLEligible:   false,
			ReviewOnly:    true,
			ReviewReason:  domain.ReviewReasonSharedStore,
			Risk:          domain.RiskLow,
			LastAccessed:  info.ModTime(),
			Category:      "orbstack-data",
			ProviderMetrics: &domain.ProviderMetrics{
				SizeBytes:      size,
				AllocatedBytes: usage.AllocatedBytes,
				ApparentBytes:  usage.ApparentBytes,
				Partial:        usage.Partial,
				StatusNote:     "OrbStack virtual machines and persistent container storage",
			},
		}

		targets = append(targets, target)
		break // Avoid duplicate emission if both nested and parent data paths match
	}

	return targets, nil
}
```

---

### 3.5. NuGet Shared Store Protection (`internal/cleaner/dotnet/dotnet.go`)

- **Role:** Cleaner do ecossistema .NET com proteção estrita para `~/.nuget/packages`.
- **Data Flow:** Inspeciona caches globais; marca a categoria `nuget-packages` incondicionalmente como `ReviewOnly: true`, `ReviewReason: domain.ReviewReasonSharedStore`, `EligibleBytes: 0`, mantendo `nuget-http-cache`, `nuget-temp` e `dotnet-build` elegíveis para limpeza conforme TTL. No `Clean()`, rejeita qualquer alvo `ReviewOnly` ou cujo caminho corresponda a `paths.DotNetNuGet`.
- **Closest Analog:** `internal/cleaner/dotnet/dotnet.go:62-117` e `internal/cleaner/dotnet/dotnet.go:195-202`.
- **Decisions Enforced:** `D-09`, `D-12`, `STORE-03`, `POL-01`.

#### Existing Code Excerpt (`internal/cleaner/dotnet/dotnet.go:85-115, 195-202`)
```go
		info, err := os.Stat(d.path)
		if err != nil {
			continue
		}

		lastAccessed := info.ModTime()
		size, err := devfs.CalcDirSize(d.path)
		if err != nil || size == 0 {
			continue
		}

		eligibleBytes := int64(0)
		ttlEligible := false
		if rules.MaxAgeDays == 0 || lastAccessed.Before(cutoff) {
			eligibleBytes = size
			ttlEligible = true
		}

		targets = append(targets, domain.Target{
			ID:            d.category + ":" + d.path,
			Name:          d.category,
			Path:          d.path,
			SizeBytes:     size,
			EligibleBytes: eligibleBytes,
			TTLEligible:   ttlEligible,
			Risk:          domain.RiskLow,
			LastAccessed:  lastAccessed,
			Category:      d.category,
		})
// ...
func (c *dotnetCleaner) Clean(_ context.Context, targets []domain.Target) error {
	for _, t := range targets {
		if err := os.RemoveAll(t.Path); err != nil {
			return err
		}
	}
	return nil
}
```

#### Concrete Adaptation Pattern for Phase 10 (`internal/cleaner/dotnet/dotnet.go`)
```go
func (c *dotnetCleaner) scanGlobalCaches(rules domain.EcosystemRules) ([]domain.Target, error) {
	type cacheDir struct {
		path     string
		category string
	}

	dirs := []cacheDir{
		{c.paths.DotNetNuGet, "nuget-packages"},
		{c.paths.DotNetHTTP, "nuget-http-cache"},
		{c.paths.DotNetTemp, "nuget-temp"},
	}

	cutoff := time.Now().AddDate(0, 0, -rules.MaxAgeDays)
	var targets []domain.Target

	for _, d := range dirs {
		if d.path == "" || isExcluded(d.path, rules.ExcludePaths) {
			continue
		}

		info, err := os.Stat(d.path)
		if err != nil {
			continue
		}

		lastAccessed := info.ModTime()
		size, err := devfs.CalcDirSize(d.path)
		if err != nil || size == 0 {
			continue
		}

		target := domain.Target{
			ID:           d.category + ":" + d.path,
			Name:         d.category,
			Path:         d.path,
			SizeBytes:    size,
			Risk:         domain.RiskLow,
			LastAccessed: lastAccessed,
			Category:     d.category,
		}

		// D-09: nuget-packages is strictly ReviewOnly (shared store)
		if d.category == "nuget-packages" {
			target.ReviewOnly = true
			target.ReviewReason = domain.ReviewReasonSharedStore
			target.EligibleBytes = 0
			target.TTLEligible = false
		} else {
			if rules.MaxAgeDays == 0 || lastAccessed.Before(cutoff) {
				target.EligibleBytes = size
				target.TTLEligible = true
			}
		}

		targets = append(targets, target)
	}

	return targets, nil
}

// Clean enforces defensive guard against deleting nuget-packages shared store (D-12).
func (c *dotnetCleaner) Clean(_ context.Context, targets []domain.Target) error {
	for _, t := range targets {
		if t.ReviewOnly || t.Category == "nuget-packages" || t.Path == c.paths.DotNetNuGet {
			continue // Defensive guard: never delete shared package store
		}
		if safety.IsCoreBlacklisted(t.Path) {
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

### 3.6. pnpm Hybrid Store Discovery & Inode-Deduplicated ReviewOnly (`internal/cleaner/pnpm/pnpm.go`)

- **Role:** Cleaner do ecossistema `"pnpm"` com descoberta híbrida (CLI + disco) do shared store.
- **Data Flow:** Descobre o store via `pnpm store path` e candidatos em disco; deduplica symlinks; calcula o uso real via `devfs.CalcUsage` deduplicando hardlinks por `(Dev, Ino)`; marca o store como 100% `ReviewOnly`; remove o alvo virtual `pnpm://store/prune`; e no `Clean()` elimina a execução de `pnpm store prune`, permitindo remover exclusivamente o metadata cache.
- **Closest Analog:** `internal/cleaner/pnpm/pnpm.go:36-151`.
- **Decisions Enforced:** `D-10`, `D-11`, `D-12`, `STORE-03`, `SCAN-02`.

#### Existing Code Excerpt (`internal/cleaner/pnpm/pnpm.go:63-82, 131-151`)
```go
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
// ...
func (c *pnpmCleaner) Clean(ctx context.Context, targets []domain.Target) error {
	for _, t := range targets {
		if t.Path == "pnpm://store/prune" {
			if _, err := c.exec.Output(ctx, "pnpm", "store", "prune"); err != nil {
				return fmt.Errorf("pnpm store prune failed: %w", err)
			}
			continue
		}
		// ...
```

#### Concrete Adaptation Pattern for Phase 10 (`internal/cleaner/pnpm/pnpm.go`)
```go
func (c *pnpmCleaner) Scan(ctx context.Context, rules domain.EcosystemRules) ([]domain.Target, error) {
	var targets []domain.Target
	cutoff := time.Now().AddDate(0, 0, -rules.MaxAgeDays)

	// 1. Hybrid pnpm store discovery (D-10, D-11, STORE-03)
	storePaths := c.discoverStorePaths(ctx)
	for _, sPath := range storePaths {
		info, err := os.Stat(sPath)
		if err != nil {
			continue
		}

		usage, _ := devfs.CalcUsage(sPath)
		size := usage.AllocatedBytes
		if size == 0 {
			size = usage.ApparentBytes
		}
		if size == 0 {
			continue
		}

		sum := sha256.Sum256([]byte(sPath))
		id := fmt.Sprintf("pnpm-store-%x", sum[:4])

		targets = append(targets, domain.Target{
			ID:            id,
			Name:          "pnpm store",
			Path:          sPath,
			SizeBytes:     size,
			EligibleBytes: 0, // D-11: 100% ReviewOnly
			TTLEligible:   false,
			ReviewOnly:    true,
			ReviewReason:  domain.ReviewReasonSharedStore,
			Risk:          domain.RiskLow,
			LastAccessed:  info.ModTime(),
			Category:      "pnpm-store",
			ProviderMetrics: &domain.ProviderMetrics{
				SizeBytes:      size,
				AllocatedBytes: usage.AllocatedBytes,
				ApparentBytes:  usage.ApparentBytes,
				TotalCount:     usage.FileCount,
				Partial:        usage.Partial,
				StatusNote:     "content-addressable shared package store",
			},
		})
	}

	// 2. Physical HTTP metadata cache (remains eligible for deletion conforme TTL)
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

func (c *pnpmCleaner) discoverStorePaths(ctx context.Context) []string {
	seen := make(map[string]bool)
	var result []string

	// Check CLI pnpm store path
	out, err := c.exec.Output(ctx, "pnpm", "store", "path")
	if err == nil {
		cliPath := strings.TrimSpace(string(out))
		if cliPath != "" {
			if realPath, err := filepath.EvalSymlinks(cliPath); err == nil {
				cliPath = realPath
			}
			cliPath = filepath.Clean(cliPath)
			if devfs.DirExists(cliPath) {
				seen[cliPath] = true
				result = append(result, cliPath)
			}
		}
	}

	// Check canonical on-disk candidates (D-10)
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		diskCandidates := []string{
			filepath.Join(home, "Library", "pnpm", "store"),
			filepath.Join(home, ".local", "share", "pnpm", "store"),
			filepath.Join(home, ".pnpm-store"),
		}
		for _, cand := range diskCandidates {
			if realPath, err := filepath.EvalSymlinks(cand); err == nil {
				cand = realPath
			}
			cand = filepath.Clean(cand)
			if devfs.DirExists(cand) && !seen[cand] {
				seen[cand] = true
				result = append(result, cand)
			}
		}
	}

	return result
}

// Clean enforces defensive guard: never delete or prune pnpm store (D-11, D-12).
func (c *pnpmCleaner) Clean(ctx context.Context, targets []domain.Target) error {
	for _, t := range targets {
		if t.ReviewOnly || t.Category == "pnpm-store" || t.Category == "virtual-pnpm-store" || strings.HasPrefix(t.Path, "pnpm://") {
			continue // INVIOLABLE: never delete or prune shared pnpm store
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

### 3.7. Engine Dependency Injection (`internal/engine/register.go`)

- **Role:** Registro central e injeção de dependências dos cleaners no motor do DevClean.
- **Data Flow:** Propaga o `cleaner.CommandExecutor` para `docker.NewWithExecutor(exec)`, garantindo que testes unitários e de integração de todo o motor usem executores mockados sem dependência do ambiente hospedeiro.
- **Closest Analog:** `internal/engine/register.go:52-59`.
- **Decisions Enforced:** `D-04`.

#### Existing Code Excerpt (`internal/engine/register.go:42-50`)
```go
	// Ecossistemas base de linguagens e containers
	eng.Register(docker.New())
	eng.Register(golang.New())
	eng.Register(node.New(effectivePaths))
	eng.Register(dotnet.New(paths, effectivePaths))
	eng.Register(python.New(paths.PythonUVCache, effectivePaths))
	eng.Register(rust.New(effectivePaths))
	eng.Register(java.New(effectivePaths))
```

#### Concrete Adaptation Pattern for Phase 10 (`internal/engine/register.go`)
```go
	// Ecossistemas base de linguagens e containers
	eng.Register(docker.NewWithExecutor(exec))
	eng.Register(golang.New())
	eng.Register(node.New(effectivePaths))
	eng.Register(dotnet.New(paths, effectivePaths))
	eng.Register(python.New(paths.PythonUVCache, effectivePaths))
	eng.Register(rust.New(effectivePaths))
	eng.Register(java.New(effectivePaths))
```

---

## 4. Cross-Cutting Architectural Patterns

### 4.1. Inode Deduplication Algorithm (`internal/fs/usage.go`)
- **Problema:** O gerenciador de pacotes `pnpm` utiliza hardlinks extensivos para compartilhar pacotes entre múltiplos projetos. Varrer o diretório com `os.WalkDir` e somar `fi.Size()` multiplica o espaço reportado por cada link existente no mesmo inode, resultando em inflação de 3x a 10x do tamanho real do store.
- **Solução:** Em `devfs.CalcUsage`, mantemos uma tabela de dispersão `seenInodes := make(map[[2]uint64]bool)`. Para cada arquivo regular, extraímos `(dev, ino)`. Se o par já foi registrado, o arquivo é pulado, garantindo que cada bloco físico e byte aparente seja contabilizado exatamente uma vez.

### 4.2. Física de Arquivos Esparsos: Blocos Alocados vs Tamanho Aparente
- **Problema:** Imagens de disco de máquinas virtuais (`Docker.raw`, `data.img.raw` do OrbStack) utilizam o recurso de sparse files do APFS e ext4. Uma imagem de 500 GB aloca apenas os blocos que efetivamente contêm dados gravados (ex: 5 GB).
- **Mapeamento de Métricas:**
  - `AllocatedBytes = stat.Blocks * 512` (em conformidade com a convenção POSIX de blocos de 512 bytes em `stat_t`).
  - `ApparentBytes = stat.Size`.
  - `Target.SizeBytes = AllocatedBytes` (representação física e honesta do impacto no disco).
  - `Target.EligibleBytes = 0` (preservação incondicional de dados de VM).
  - `Target.Name` rotula explicitamente ambos os valores (ex: `"OrbStack VM/Data (allocated: 5.14 GB, apparent: 494.3 GB)"`), eliminando surpresas para o usuário.

### 4.3. Defesa em Duas Camadas contra Mutações Indesejadas (`POL-02`, `D-12`)
- **Camada 1 (Orquestrador `deleter.Coordinator`):** No Estágio 1 do pipeline de deleção, qualquer alvo com `t.ReviewOnly == true` é filtrado antes de qualquer tentativa de processamento, anexado a `SkippedReviewOnly` e registrado com mensagem de aviso com a razão semântica.
- **Camada 2 (Interna ao Cleaner `Clean()`):** Mesmo que um desenvolvedor ou teste unitário instancie `dockerCleaner`, `dotnetCleaner` ou `pnpmCleaner` diretamente e chame `Clean(ctx, targets)`, os métodos inspecionam cada alvo e abortam imediatamente se `t.ReviewOnly == true` ou se o alvo pertencer a categorias/caminhos de shared stores protegidos.

### 4.4. Process Guard Fail-Closed com Revalidação Pré-Deleção (`D-08`)
- **Verificação via `pgrep -x OrbStack`:**
  - `exit 0` com saída: processo em execução -> `ReviewOnly = true`, `ReviewReason = ReviewReasonActiveProcess`, `EligibleBytes = 0`.
  - `exit 1`: processo inativo -> elegível para deleção conforme TTL (`RiskLow`).
  - Outros erros (ferramenta ausente, erro de permissão): estado incerto -> **FAIL CLOSED** (`ReviewOnly = true`, `ReviewReason = ReviewReasonAmbiguous`, `EligibleBytes = 0`).
- **Revalidação Pré-Deleção:** No momento em que `Clean()` for invocado, o processo é reconsultado imediatamente. Se o usuário tiver iniciado o OrbStack entre a fase de scan e a execução da limpeza, a exclusão física do cache regenerável é sumariamente abortada.

### 4.5. Visibilidade Explícita de Falhas de Daemon (`D-14`, `D-16`)
- Cleaners anteriores silenciavam erros de conexão ao socket do Docker e retornavam 0 alvos, passando a falsa impressão de que o Docker não consumia espaço.
- DevClean emite um alvo sentinela `docker-daemon-status` (`docker://daemon/status`) marcado com `ReviewOnly: true` e `ReviewReason: domain.ReviewReasonAmbiguous`, permitindo ao usuário entender imediatamente que o daemon está inacessível enquanto continua inventariando o arquivo de disco físico do Docker Desktop (`Docker.raw`) e o OrbStack.

### 4.6. Imutabilidade de Esquemas Virtuais (`docker://`, `pnpm://`)
- Identificados por `t.IsVirtual()` (`strings.Contains(t.Path, "://")`).
- Não correspondem a caminhos de arquivos locais do sistema operacional e são imunes à deduplicação de diretórios do motor (`internal/engine/ownership.go`).
- Sob nenhuma circunstância passam por `os.RemoveAll`.

---

## 5. Test Pattern Mapping & Hermetic Verification

### 5.1. Arquitetura de Auxiliares de Teste
Todos os testes são 100% herméticos, executados com `t.TempDir()`, sem chamadas a daemons reais ou processos externos.
- **`cleaner.MockCommandExecutor` (`internal/cleaner/executor_mock.go`):** Fornece controle total sobre respostas a `docker system df --format "{{json .}}"`, `pnpm store path`, `pgrep -x OrbStack` e LookPath.
- **Fixtures de Hardlinks:** Utilizam `os.Link()` dentro de `t.TempDir()` para validar a deduplicação de inodes em `CalcUsage`.
- **Fixtures de Arquivos Esparsos:** Utilizam `file.Truncate()` para criar arquivos com `apparent > allocated` e validar o cálculo de blocos sem panics.

### 5.2. Mapeamento de Requisitos para Testes Unitários e de Integração

| Req | Função de Teste | Arquivo de Teste | Padrão Central Verificado |
|---|---|---|---|
| **STORE-01** | `TestDocker_Scan_CategorizedTargets_ReviewOnly` | `internal/cleaner/docker/docker_test.go` | Parsing de `docker system df` gerando 4 alvos categorizados 100% ReviewOnly com ProviderMetrics. |
| **STORE-01** | `TestDocker_Scan_DaemonOffline_AmbiguousTarget` | `internal/cleaner/docker/docker_test.go` | Daemon fora do ar gera `docker-daemon-status` como estado ambíguo sem zerar silenciosamente. |
| **STORE-01** | `TestDocker_Scan_MalformedJSON_PartialTarget` | `internal/cleaner/docker/docker_test.go` | Linhas JSON corrompidas acionam `Partial: true` e alvo de status sem panic. |
| **STORE-01** | `TestDocker_VMDisk_HostDiscovery` | `internal/cleaner/docker/docker_test.go` | Descoberta de `Docker.raw` no host com medição sparse e separação do `docker system df`. |
| **STORE-01** | `TestDocker_Clean_DefensiveGuard_NeverMutates` | `internal/cleaner/docker/docker_test.go` | Invocação direta de `Clean()` em alvos `docker://` ou VM disk preserva arquivos sentinela. |
| **STORE-02** | `TestOrbStack_Cache_ProcessGuard_Running` | `internal/cleaner/docker/orbstack_test.go` | Processo ativo bloqueia `OrbStack Cache` como `ReviewOnly` com razão `active_process`. |
| **STORE-02** | `TestOrbStack_Cache_ProcessGuard_Stopped` | `internal/cleaner/docker/orbstack_test.go` | Processo parado libera `OrbStack Cache` como elegível (`RiskLow`). |
| **STORE-02** | `TestOrbStack_Cache_ProcessGuard_Uncertain` | `internal/cleaner/docker/orbstack_test.go` | Erro desconhecido de `pgrep` bloqueia cache como fail-closed (`ambiguous_state`). |
| **STORE-02** | `TestOrbStack_VMData_StrictlyReviewOnly` | `internal/cleaner/docker/orbstack_test.go` | `OrbStack VM/Data` é 100% ReviewOnly com contabilidade sparse (`Allocated` vs `Apparent`). |
| **STORE-02** | `TestOrbStack_Clean_PreDeletionRevalidation` | `internal/cleaner/docker/orbstack_test.go` | Se OrbStack for iniciado após scan, `Clean()` aborta deleção do cache. |
| **STORE-03** | `TestDotNet_NuGetPackages_LockedToReviewOnly` | `internal/cleaner/dotnet/dotnet_test.go` | `nuget-packages` marcado com `ReviewOnly: true`, `ReviewReasonSharedStore`, `EligibleBytes: 0`. |
| **STORE-03** | `TestDotNet_NuGetHttpAndTemp_RemainTTLEligible` | `internal/cleaner/dotnet/dotnet_test.go` | `nuget-http-cache` e `nuget-temp` continuam elegíveis conforme TTL. |
| **STORE-03** | `TestDotNet_Clean_DefensiveGuard_PreservesNuGet` | `internal/cleaner/dotnet/dotnet_test.go` | `Clean()` chamado com `nuget-packages` não remove arquivos sentinela. |
| **STORE-03** | `TestPnpm_HybridStoreDiscovery_ReviewOnly` | `internal/cleaner/pnpm/pnpm_test.go` | Descoberta híbrida (CLI + disco) trava store como `ReviewOnly` com `devfs.CalcUsage`. |
| **STORE-03** | `TestPnpm_Clean_MetadataCacheOnly_NoStorePrune` | `internal/cleaner/pnpm/pnpm_test.go` | `Clean()` apaga metadata cache mas nunca executa `pnpm store prune` nem toca no store. |
| **POL-01** | `TestDomain_Target_ProviderMetrics_Serialization` | `internal/domain/target_test.go` | Serialização/deserialização JSON do campo `ProviderMetrics` em `domain.Target`. |
| **SCAN-02** | `TestDocker_IsInstalled_DiskPresence` | `internal/cleaner/docker/docker_test.go` | `IsInstalled()` retorna `true` com binário ausente mas pastas de estado presentes em disco. |
| **VAL-01** | `TestFS_Usage_InodeDeduplication` | `internal/fs/usage_test.go` | Hardlinks criados com `os.Link()` são contabilizados uma única vez em `CalcUsage`. |
| **VAL-01** | `TestFS_Usage_SparseFileAccounting` | `internal/fs/usage_test.go` | Arquivo truncado esparso reporta `AllocatedBytes <= ApparentBytes` sem erro. |
| **VAL-01** | `TestFS_Usage_PartialAccess_FailClosed` | `internal/fs/usage_test.go` | Subdiretório sem permissão de leitura aciona `Partial = true` e nota explicativa. |
| **VAL-02** | `TestCoordinator_DryRun_ZeroMutation_Phase10` | `internal/deleter/policy_test.go` | Canário de mutação zero sob DryRun cobrindo alvos de Docker, OrbStack, NuGet e pnpm store. |

---

## 6. Zero-Mutation Canary Specification for Phase 10

Para integrar `TestCoordinator_DryRun_ZeroMutation_Phase10` em `internal/deleter/policy_test.go`:

### 6.1. Configuração de Sentinelas e Alvos Canário
O teste instancia uma estrutura em `t.TempDir()` contendo:
1. **Docker Host VM Disk (`docker-vm-disk`):** Arquivo sentinela `Docker.raw` de 10 MB com hash SHA-256 e timestamp fixados. Alvo marcado como `ReviewOnly: true`, `ReviewReason: domain.ReviewReasonSharedStore`, `EligibleBytes: 0`.
2. **Docker System DF (`docker://system-df/images`):** Alvo virtual representando 50 GB de imagens lógicas. Alvo marcado como `ReviewOnly: true`, `ReviewReason: domain.ReviewReasonSharedStore`, `EligibleBytes: 0`.
3. **OrbStack VM/Data (`orbstack-data`):** Diretório com arquivo `data.img.raw` de 20 MB. Marcado como `ReviewOnly: true`, `ReviewReason: domain.ReviewReasonSharedStore`, `EligibleBytes: 0`.
4. **OrbStack Cache Inativo (`orbstack-cache`):** Diretório de cache regenerável com arquivos canário de 500 KB. Processo inativo (`pgrep` exit 1) -> `ReviewOnly: false`, `EligibleBytes: 512000`, `TTLEligible: true`.
5. **NuGet Packages (`nuget-packages`):** Diretório `~/.nuget/packages` com pacote canário (`Canary.Package.1.0.0.nupkg`) e SHA-256 fixado. Marcado como `ReviewOnly: true`, `ReviewReason: domain.ReviewReasonSharedStore`, `EligibleBytes: 0`.
6. **NuGet HTTP Cache (`nuget-http-cache`):** Diretório com arquivos de resposta HTTP de 200 KB elegíveis para deleção -> `ReviewOnly: false`, `EligibleBytes: 204800`, `TTLEligible: true`.
7. **pnpm Store (`pnpm-store`):** Diretório content-addressable com hardlinks e SHA-256 fixado. Marcado como `ReviewOnly: true`, `ReviewReason: domain.ReviewReasonSharedStore`, `EligibleBytes: 0`.
8. **pnpm Metadata Cache (`pnpm-cache`):** Cache HTTP regenerável de 100 KB elegível para deleção -> `ReviewOnly: false`, `EligibleBytes: 102400`, `TTLEligible: true`.

### 6.2. Execução sob DryRun
```go
opts := deleter.Options{
	DryRun: true,
	Auto:   true,
	Mode:   "auto",
}
res, err := coord.Delete(context.Background(), targetsByEco, opts)
```

### 6.3. Asserções Canário Invioláveis
1. **Métricas de Resultado:**
   - Alvos marcados como `ReviewOnly` (Docker VM Disk, Docker Images, OrbStack VM/Data, NuGet Packages, pnpm Store) devem ser estritamente filtrados para `SkippedReviewOnly` com mensagens de aviso semânticas.
   - `res.TotalCount` deve contabilizar estritamente os alvos elegíveis simulados (`OrbStack Cache`, `NuGet HTTP Cache`, `pnpm Metadata Cache`).
   - `len(res.Errors)` deve ser 0.
2. **Integridade Criptográfica (Zero Mutação):**
   - Para cada arquivo canário de shared store e cache (`Docker.raw`, `data.img.raw`, `Canary.Package.1.0.0.nupkg`, arquivos do pnpm store, arquivos de cache), verificar que:
     - O arquivo ainda existe no caminho exato.
     - O hash SHA-256 é idêntico ao pré-teste.
     - O tamanho em bytes é idêntico ao pré-teste.
     - O `ModTime()` é rigorosamente idêntico ao pré-teste.
3. **Lixeira e Histórico:**
   - O diretório de lixeira (`trashDir`) permanece completamente vazio, sem arquivos movidos e sem `manifest.json`.
   - O arquivo `history.jsonl` não é criado ou possui tamanho 0.
4. **Chamadas aos Cleaners:**
   - Nenhum cleaner real ou mockado deve ter seu método `Clean()` acionado com alvos em modo DryRun.

---

## PATTERN MAPPING COMPLETE

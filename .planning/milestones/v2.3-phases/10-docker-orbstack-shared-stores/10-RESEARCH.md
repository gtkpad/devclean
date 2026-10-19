# Phase 10: Docker, OrbStack & Shared Stores - Research

**Date:** 2026-10-16  
**Status:** Complete  
**Requirements Covered:** `STORE-01`, `STORE-02`, `STORE-03`, `POL-01`, `POL-02`, `SCAN-02`, `VAL-01`, `VAL-02`

---

## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-01:** **Alvos por Categoria do `docker system df`:** Emitir 1 alvo por categoria retornada pelo `docker system df --format "{{json .}}"` (`Images`, `Containers`, `Local Volumes`, `Build Cache`) sob caminhos virtuais `docker://system-df/<type>`, mantendo as métricas lógicas do daemon estritamente separadas de arquivos de disco de VM no host (`Docker.raw`).
- **D-02:** **Armazenamento Docker 100% ReviewOnly:** Todos os alvos de armazenamento do Docker (`Images`, `Containers`, `Local Volumes`, `Build Cache` e `Docker Desktop VM Disk`) são incondicionalmente marcados com `ReviewOnly: true`, `ReviewReason: domain.ReviewReasonSharedStore`, `EligibleBytes: 0` e `TTLEligible: false`. O método `Clean()` do Docker não apaga imagens, volumes ou build cache do daemon; só permite limpar caches físicos regeneráveis de host (como `OrbStack Cache` quando o OrbStack estiver fechado).
- **D-03:** **Metadados Estruturados `ProviderMetrics` em `domain.Target`:** Adicionar o campo opcional `ProviderMetrics *ProviderMetrics` (`json:"provider_metrics,omitempty"`) em `domain.Target` contendo `TotalCount int`, `ActiveCount int`, `SizeBytes int64`, `ReclaimableBytes int64`, `ApparentBytes int64`, `AllocatedBytes int64`, `Partial bool` e `StatusNote string`, além de resumo legível no `Name`, garantindo fidelidade tanto na CLI/TUI quanto na serialização JSON/MCP.
- **D-04:** **Padronização sobre `cleaner.CommandExecutor`:** Refatorar `internal/cleaner/docker` para utilizar `cleaner.CommandExecutor` (`NewWithExecutor(exec)`), executando `docker system df --format "{{json .}}"` sem acoplamento direto ao SDK pesado do Docker, garantindo testes unitários 100% herméticos e determinísticos sem CGO.
- **D-05:** **Integração do OrbStack no Pacote `internal/cleaner/docker`:** Implementar a descoberta e limpeza do OrbStack dentro de `internal/cleaner/docker` (ex: `orbstack.go`) sob o ecossistema `"docker"`, alinhado à Frente 2 da `SPEC-v2.3-MOLE-PARITY.md` sem criar um ecossistema extra na configuração.
- **D-06:** **Separação Estrita entre Cache Regenerável e VM/Data do OrbStack:** Emitir alvos distintos: `OrbStack Cache` (`~/Library/Caches/dev.orbstack.OrbStack` no macOS e `~/.cache/orbstack` no Linux) como cache regenerável elegível para limpeza (`RiskLow` quando o OrbStack estiver inativo), e `OrbStack VM/Data` (`~/Library/Group Containers/*.dev.orbstack/data` e `~/.orbstack/data` / `~/.orbstack`) como armazenamento incondicionalmente `ReviewOnly: true`, `ReviewReason: domain.ReviewReasonSharedStore` e `EligibleBytes: 0`.
- **D-07:** **Contabilidade de Arquivos Esparsos (`Allocated` vs `Apparent`) e Deduplicação de Inodes:** Criar helper em `internal/fs` (`CalcUsage` / `CalcDirUsage`) usando `syscall.Stat_t` (sem CGO) em Darwin/Linux para contabilizar blocos alocados reais (`Blocks * 512`) e tamanho aparente (`Size`) deduplicando hardlinks por `(Dev, Ino)`. Nos alvos de VM/Data (`OrbStack VM/Data` e `Docker Desktop VM Disk`), `SizeBytes` reflete os bytes fisicamente alocados (`AllocatedBytes`), `EligibleBytes` é `0`, e `ProviderMetrics` + `Name` rotulam explicitamente `allocated` vs `apparent` sem jamais somá-los como bytes recuperáveis.
- **D-08:** **Process Guard Fail-Closed para OrbStack:** Verificar processos ativos do OrbStack (`pgrep -x OrbStack`) via `cleaner.CommandExecutor`. Se o OrbStack estiver em execução ou a detecção retornar erro/estado incerto, marcar `OrbStack Cache` com `ReviewOnly: true`, `ReviewReason: domain.ReviewReasonActiveProcess`, `EligibleBytes: 0` e `TTLEligible: false`, revalidando imediatamente antes da exclusão no `Clean()`.
- **D-09:** **Proteção ReviewOnly Exclusiva para `nuget-packages` no Cleaner .NET:** Em `internal/cleaner/dotnet/dotnet.go`, marcar `~/.nuget/packages` (`nuget-packages`) sempre com `ReviewOnly: true`, `ReviewReason: domain.ReviewReasonSharedStore`, `EligibleBytes: 0` e `TTLEligible: false`, mantendo `nuget-http-cache`, `nuget-temp` e `dotnet-build` (`bin/` e `obj/` locais) elegíveis para limpeza conforme TTL.
- **D-10:** **Descoberta Híbrida do `pnpm store` por CLI e Presença em Disco:** Em `internal/cleaner/pnpm/pnpm.go`, descobrir o caminho do `pnpm store` consultando `pnpm store path` via `CommandExecutor` E verificando os candidatos canônicos em disco (`~/Library/pnpm/store`, `~/.local/share/pnpm/store`, `~/.pnpm-store`), resolvendo symlinks (`filepath.EvalSymlinks` / `filepath.Clean`) para deduplicar e garantir descoberta mesmo quando o binário `pnpm` não estiver no `PATH` (`SCAN-02`).
- **D-11:** **Travamento ReviewOnly do `pnpm store` e Remoção de Prune Automático:** Marcar o alvo `pnpm store` incondicionalmente com `ReviewOnly: true`, `ReviewReason: domain.ReviewReasonSharedStore`, `EligibleBytes: 0` e `TTLEligible: false`, usando medição deduplicada por inodes (`CalcUsage`) para não inflar o tamanho por hardlinks. Remover a execução de `pnpm store prune` de dentro de `Clean()`, mantendo apenas o `pnpm metadata cache` (`~/Library/Caches/pnpm` e `~/.cache/pnpm`) elegível para deleção física.
- **D-12:** **Guarda Defensiva Direta no `Clean()` de `docker`, `dotnet` e `pnpm`:** Além do bloqueio no Estágio 1 do `deleter.Coordinator`, os próprios métodos `Clean()` de `docker`, `dotnet` e `pnpm` devem ignorar/recusar imediatamente qualquer `Target` com `t.ReviewOnly == true` ou cuja categoria/caminho pertença a um shared store (`nuget-packages`, `pnpm-store`, `docker://system-df/*`, `docker-vm-disk`, `orbstack-data`), garantindo que chamadas diretas em testes ou código nunca apaguem sentinelas de stores.
- **D-13:** **`IsInstalled()` Baseado em Disco e CLI no Cleaner Docker/OrbStack:** `IsInstalled()` retorna `true` se `docker` ou `orb` existirem no `PATH` (via `exec.LookPath`) OU se qualquer diretório de estado/cache em disco do Docker ou OrbStack existir (`~/.docker`, `~/Library/Containers/com.docker.docker`, `~/Library/Group Containers/*.dev.orbstack`, `~/Library/Caches/dev.orbstack.OrbStack`, `~/.orbstack`, `~/.cache/orbstack`), sem exigir que o daemon responda a `ping`.
- **D-14:** **Alvo Explícito de Estado Incerto em Falhas do Daemon ou JSON Malformado:** Quando o binário `docker` ou diretórios do Docker estiverem presentes mas `docker system df --format "{{json .}}"` falhar (daemon parado/inacessível, erro de permissão no socket, timeout) ou retornar linhas JSON malformadas/incompletas, emitir um alvo explícito de revisão (`ID: "docker-daemon-status"`, `Path: "docker://daemon/status"`, `ReviewOnly: true`, `ReviewReason: domain.ReviewReasonAmbiguous`, `EligibleBytes: 0`) descrevendo a indisponibilidade/erro nas métricas, além de inventariar normalmente os artefatos em disco acessíveis (`Docker.raw`, `OrbStack`). Nunca retornar sucesso silencioso de zero bytes para um daemon com falha.
- **D-15:** **Separação Explícita entre `Docker Desktop VM Disk` (Host) e `docker system df` (Lógico):** Quando o arquivo/diretório de VM do Docker Desktop no host existir (`~/Library/Containers/com.docker.docker/Data/vms/0/data/Docker.raw` ou `~/.docker/desktop/vms`), emiti-lo como um alvo dedicado de host (`Category: "docker-vm-disk"`, `ReviewOnly: true`, `ReviewReason: domain.ReviewReasonSharedStore`, `EligibleBytes: 0`) com medição sparse (`AllocatedBytes` vs `ApparentBytes`), separado das categorias lógicas `docker://system-df/*`.
- **D-16:** **Degradação Fail-Closed em Locks e Erros Parciais de Permissão em Shared Stores:** Se a inspeção de um shared store (`NuGet`, `pnpm store`, `OrbStack VM/Data`, `Docker VM Disk`) encontrar arquivos de lock ativos (`.lock`), erro parcial de permissão (`EACCES`/`EPERM`) na árvore ou estado incerto, preservar o alvo no inventário com `ReviewOnly: true` (`ReviewReasonAmbiguous` quando o tamanho/estado for incerto ou `ProviderMetrics.Partial = true`), pular symlinks com `safety.IsSymlink` e nunca falhar de forma silenciosa nem permitir exclusão.

### the agent's Discretion

- Nomenclatura exata das funções auxiliares em `internal/fs` para cálculo de uso em disco com blocos alocados e deduplicação de inodes por sistema operacional (`darwin`, `linux`, fallback `other`).
- Formatação textual exata dos resumos de métricas (`total`, `active`, `reclaimable`, `allocated`, `apparent`) nos campos `Name` dos alvos do Docker e OrbStack.
- Estrutura interna de arquivos dentro de `internal/cleaner/docker/` (ex: separar `docker.go` e `orbstack.go`).

---

## Architectural Responsibility Map

| Component / File | Specific Responsibility in Phase 10 |
|---|---|
| `internal/domain/types.go` | Defines structured `ProviderMetrics` struct (`TotalCount`, `ActiveCount`, `SizeBytes`, `ReclaimableBytes`, `ApparentBytes`, `AllocatedBytes`, `Partial`, `StatusNote`). Adds `ProviderMetrics *ProviderMetrics` field to `domain.Target` with `omitempty`. |
| `internal/fs/usage.go` | Exposes public API `CalcUsage(path string) (Usage, error)` and `CalcUsageWithContext(...)`. Traverses trees safely, handles single sparse files and directories, deduplicates hardlinks via `(dev, ino)`, collects apparent vs allocated bytes, and detects partial reads/locks fail-closed. |
| `internal/fs/usage_darwin.go` | Platform extractor for Darwin using `syscall.Stat_t` (`int32(Dev)` cast to `uint64`, `Ino`, `Blocks * 512`, `Size`). Zero CGO. |
| `internal/fs/usage_linux.go` | Platform extractor for Linux using `syscall.Stat_t` (`uint64(Dev)`, `Ino`, `Blocks * 512`, `Size`). Zero CGO. |
| `internal/fs/usage_other.go` | Fallback for other platforms using standard `os.FileInfo` (`Size()` for apparent and allocated). Zero CGO. |
| `internal/cleaner/docker/docker.go` | Standardizes on `cleaner.CommandExecutor` via `NewWithExecutor`. Removes Docker SDK dependency. Parses `docker system df --format "{{json .}}"`, discovers `Docker Desktop VM Disk`, checks disk presence for `IsInstalled()`, yields `docker-daemon-status` on failure, and enforces defensive `Clean()` guard. |
| `internal/cleaner/docker/orbstack.go` | Sub-scanner for OrbStack under `"docker"` ecosystem. Discovers regenerable `OrbStack Cache` (with `pgrep -x OrbStack` Process Guard) and `OrbStack VM/Data` (with `CalcUsage` sparse accounting). Revalidates process in `Clean()`. |
| `internal/cleaner/dotnet/dotnet.go` | Updates `scanGlobalCaches` so `~/.nuget/packages` (`nuget-packages`) is strictly `ReviewOnly: true`, `ReviewReasonSharedStore`, `EligibleBytes: 0`, and adds defensive guard in `Clean()`. |
| `internal/cleaner/pnpm/pnpm.go` | Implements hybrid store discovery (CLI + disk candidates), locks store as `ReviewOnly: true`, `ReviewReasonSharedStore`, `EligibleBytes: 0`, calculates size via `CalcUsage` with inode deduplication, and removes `pnpm store prune` from `Clean()`. |
| `internal/engine/register.go` | Updates `docker.New()` registration to `docker.NewWithExecutor(exec)` to ensure mocked executors are passed throughout the engine test suite. |

---

## Summary

Phase 10 delivers parity with tools like Mole (`tw93/mole`) for large shared virtual and package environments (`Docker`, `OrbStack`, `NuGet`, `pnpm store`) while enforcing DevClean's core safety principle: **broad inventory must never be confused with authorization to delete**.

In previous phases, DevClean established `ReviewOnly` policy handling, process guards, and disk-based detection. However, container storage and package stores present unique challenges:
1. **Virtual Machine Disk Images vs Physical Disk Allocation:** Virtual disks like OrbStack's `data.img.raw` report massive apparent sizes (e.g. 494 GB) while physically occupying a fraction of that (e.g. 5.14 GB) on APFS/ext4 sparse allocations. Summing apparent sizes produces terrifying, dishonest reports; reporting them as reclaimable invites catastrophic data loss.
2. **Content-Addressable Hardlink Stores:** Stores like `pnpm store` share files via hardlinks. Scanning without tracking `(dev, ino)` multiplies bytes by the number of links, inflating reported sizes.
3. **Daemon Logical State vs Host Files:** `docker system df` exposes logical daemon metrics (Images, Containers, Volumes, Build Cache) that have no 1:1 path on the host filesystem and must never be unlinked via filesystem `os.RemoveAll`.
4. **Daemon Availability & Fail-Closed Visibility:** If a developer has Docker installed but Docker Desktop is stopped, `docker system df` fails. Historically, cleaners silently returned 0 bytes, creating the illusion of zero disk usage. DevClean must explicitly reveal `docker-daemon-status` as an ambiguous review item (`⊙`) while still measuring the physical VM disk file on the host.

By standardizing Docker on `cleaner.CommandExecutor`, adding platform-native zero-CGO sparse and inode accounting in `internal/fs`, separating OrbStack cache from VM data, locking package stores to `ReviewOnly`, and establishing defensive guards directly in `Clean()`, Phase 10 ensures 100% honest metrics and zero unintended mutations.

---

## Standard Stack

- **Go 1.25 Standard Library:**
  - `syscall.Stat_t` via `fileInfo.Sys().(*syscall.Stat_t)` for `Dev`, `Ino`, `Blocks`, and `Size` without CGO.
  - `encoding/json` with flexible type decoding for JSON stream lines from `docker system df`.
  - `path/filepath` (`filepath.EvalSymlinks`, `filepath.Clean`, `filepath.Glob`, `filepath.WalkDir`).
  - `strings`, `strconv`, `context`, `errors`.
- **DevClean Core Primitives:**
  - `cleaner.CommandExecutor` / `cleaner.MockCommandExecutor`: Testable, shell-free command execution.
  - `domain.Target` + `domain.ProviderMetrics`: Structured metadata contract.
  - `domain.ReviewReasonSharedStore`, `ReviewReasonActiveProcess`, `ReviewReasonAmbiguous`.
  - `safety.IsCoreBlacklisted`, `safety.IsSymlink`: Path protection.

---

## Architecture Patterns

### 1. Hybrid CLI & Disk Discovery (`SCAN-02`, `D-10`, `D-13`)

Cleaners must never rely solely on a running daemon or an executable in `$PATH`.
- For `Docker`: `IsInstalled()` checks `exec.LookPath("docker") == nil || exec.LookPath("orb") == nil`, but also checks directory presence (`~/.docker`, `~/Library/Containers/com.docker.docker`, `~/Library/Group Containers/*.dev.orbstack`, `~/.orbstack`).
- For `pnpm`: Checks `pnpm store path` via `c.exec`, while also inspecting disk candidates (`~/Library/pnpm/store`, `~/.local/share/pnpm/store`, `~/.pnpm-store`). Symlinks are resolved via `filepath.EvalSymlinks` to deduplicate overlapping references.

### 2. Dual-Layer Mutation Defense (`POL-02`, `D-02`, `D-12`)

DevClean enforces safety in two distinct layers:
1. **Orchestration Layer (`deleter.Coordinator` Stage 1):** Centrally filters any `Target` where `t.ReviewOnly == true`, appending it to `SkippedReviewOnly` and emitting an explanatory review warning.
2. **Cleaner Layer (`Clean()` in `docker.go`, `dotnet.go`, `pnpm.go`):** Direct defensive guards inside each cleaner immediately skip any target marked `ReviewOnly` or matching shared store paths/categories. Even if an external caller invokes `cleaner.Clean()` bypassing the coordinator, no shared store or virtual daemon resource can ever be deleted.

### 3. Fail-Closed Process Guard with Pre-Deletion Revalidation (`D-08`)

For regenerable caches associated with active runtimes (`OrbStack Cache`):
1. **Scan Time:** `pgrep -x OrbStack` is queried via `CommandExecutor`.
   - Exit 0 (running): `ReviewOnly = true`, `ReviewReason = ReviewReasonActiveProcess`, `EligibleBytes = 0`.
   - Exit 1 (cleanly stopped): `ReviewOnly = false`, `EligibleBytes = size`, `TTLEligible = true`.
   - Any other error/exit: `ReviewOnly = true`, `ReviewReason = ReviewReasonAmbiguous` (Fail-Closed).
2. **Clean Time:** The process state is rechecked immediately before any filesystem removal. If the user started OrbStack between scan and clean, deletion is aborted.

### 4. Zero-CGO Sparse Accounting and Inode Deduplication (`D-07`, `D-11`)

Virtual machine disk files (`Docker.raw`, `data.img.raw`) and package caches (`pnpm store`) require honest disk accounting:
- **Allocated Bytes:** `stat.Blocks * 512` represents the real physical sectors occupied on the storage volume.
- **Apparent Bytes:** `stat.Size` represents the logical file size.
- **Hardlink Tracking:** When walking directories, `[2]uint64{dev, ino}` is tracked in a hash set. Subsequent hardlinks to the same file are ignored to avoid multiplying storage metrics.

### 5. Explicit Degradation on Daemon or Parsing Failures (`D-14`, `D-16`)

If Docker is present on the machine but the daemon is offline, socket permissions fail, or JSON output is corrupted:
- DevClean emits an explicit target: `ID: "docker-daemon-status"`, `Path: "docker://daemon/status"`, `ReviewOnly: true`, `ReviewReason: domain.ReviewReasonAmbiguous`, `EligibleBytes: 0`.
- The `ProviderMetrics.Partial` flag is set to `true` and `StatusNote` records the underlying error message.
- The scanner continues to inspect on-disk host artifacts (`Docker.raw`, `OrbStack`) without aborting or reporting zero bytes.

---

## Don't Hand-Roll

- **Don't import the Docker Go SDK (`github.com/docker/docker/client`) in `docker.go`:** The SDK introduces heavy dependencies, socket timeouts, and complex mocking requirements. Standardizing on `cleaner.CommandExecutor` with `docker system df --format "{{json .}}"` provides identical metrics with hermetic testability and zero CGO.
- **Don't invoke shell strings (`sh -c "docker ..."` or `sh -c "pgrep ..."`):** Always pass arguments as discrete slices via `exec.Output(ctx, "docker", ...)` to prevent command injection.
- **Don't use CGO or external CLI tools (`du -s`, `stat`) for disk accounting:** Use Go standard library `syscall.Stat_t` via `os.FileInfo.Sys()` with build tags for Darwin and Linux.
- **Don't sum Apparent Bytes into `EligibleBytes` or `SizeBytes`:** For sparse VM images, `SizeBytes` must reflect `AllocatedBytes`, and `EligibleBytes` must remain `0`.
- **Don't invent non-standard review reasons:** Use the established domain constants (`ReviewReasonSharedStore`, `ReviewReasonActiveProcess`, `ReviewReasonAmbiguous`).

---

## Common Pitfalls

### Pitfall 1: Apparent vs Allocated Discrepancy in Sparse Files
- **Symptom:** OrbStack reports `494.3 GB` occupied, alarming the user, even though APFS only has `5.1 GB` allocated.
- **Cause:** Using `info.Size()` on sparse raw disk images (`data.img.raw` or `Docker.raw`).
- **Remedy:** Extract `stat.Blocks * 512` as `AllocatedBytes` and report `AllocatedBytes` in `SizeBytes`. Surface `ApparentBytes` in `ProviderMetrics` and label both clearly in `Target.Name`.

### Pitfall 2: Hardlink Duplication in pnpm Store
- **Symptom:** `pnpm store` reports 3x to 10x its actual disk usage.
- **Cause:** Walking `pnpm store` without tracking inodes causes files with multiple directory links to be summed repeatedly.
- **Remedy:** In `fs.CalcUsage`, track `seenInodes := make(map[[2]uint64]bool)` using `(stat.Dev, stat.Ino)`. Skip already-seen inodes.

### Pitfall 3: Daemon Ping Gating `IsInstalled()`
- **Symptom:** Docker Desktop is installed with 40 GB of images, but DevClean reports `docker: not installed` because the Docker app isn't currently running.
- **Cause:** `IsInstalled()` calling `api.ping()` or `docker version`.
- **Remedy:** Conform to `SCAN-02` and `D-13`. Check `LookPath` and disk paths (`~/.docker`, `~/Library/Containers/com.docker.docker`, `~/.orbstack`). If installed and daemon is off, emit `docker-daemon-status` (`⊙`) and measure host VM disks.

### Pitfall 4: Automatic Pruning of Shared Stores
- **Symptom:** Running DevClean in auto mode wipes `~/.nuget/packages` or runs `pnpm store prune`, causing build breaks across other active projects on the machine.
- **Cause:** Treating package stores like regenerable build artifacts.
- **Remedy:** Conform to `STORE-03`, `D-09`, and `D-11`. Set `ReviewOnly: true`, `EligibleBytes: 0`, `TTLEligible: false`, and remove prune commands from `Clean()`.

### Pitfall 5: Platform Differences in `syscall.Stat_t`
- **Symptom:** Compilation error on Linux or Darwin when casting `stat.Dev`.
- **Cause:** In Go's standard library, `syscall.Stat_t.Dev` is `int32` on Darwin, but `uint64` on Linux.
- **Remedy:** Use platform-specific files (`usage_darwin.go` and `usage_linux.go`) with `//go:build` tags, converting safely to `uint64`.

---

## Code Examples

### 1. Robust `docker system df` JSON Parsing

```go
package docker

import (
	"encoding/json"
	"strconv"
	"strings"
	"github.com/gtkpad/devclean/internal/domain"
)

type rawSystemDFEntry struct {
	Type        string          `json:"Type"`
	TotalCount  json.RawMessage `json:"TotalCount"`
	Active      json.RawMessage `json:"Active"`
	Size        string          `json:"Size"`
	Reclaimable string          `json:"Reclaimable"`
}

func parseSystemDFOutput(out []byte) ([]domain.Target, bool) {
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var targets []domain.Target
	hasError := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var entry rawSystemDFEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			hasError = true
			continue
		}

		totalCount := parseFlexibleInt(entry.TotalCount)
		activeCount := parseFlexibleInt(entry.Active)
		sizeBytes := parseDockerSize(entry.Size)
		reclaimableBytes := parseDockerReclaimable(entry.Reclaimable)

		catID, pathSuffix := categoryMapping(entry.Type)
		if catID == "" {
			continue
		}

		name := formatDockerTargetName(entry.Type, totalCount, activeCount, sizeBytes, reclaimableBytes)

		targets = append(targets, domain.Target{
			ID:            "docker-" + catID,
			Name:          name,
			Path:          "docker://system-df/" + pathSuffix,
			SizeBytes:     sizeBytes,
			EligibleBytes: 0, // D-02: 100% ReviewOnly
			TTLEligible:   false,
			ReviewOnly:    true,
			ReviewReason:  domain.ReviewReasonSharedStore,
			Risk:          domain.RiskLow,
			Category:      "docker-" + catID,
			ProviderMetrics: &domain.ProviderMetrics{
				TotalCount:       totalCount,
				ActiveCount:      activeCount,
				SizeBytes:        sizeBytes,
				ReclaimableBytes: reclaimableBytes,
			},
		})
	}

	return targets, hasError
}

func parseFlexibleInt(raw json.RawMessage) int {
	s := strings.Trim(string(raw), "\" \r\n")
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return 0
}

func parseDockerReclaimable(s string) int64 {
	// Format: "1.388GB (63%)" or "0B"
	parts := strings.SplitN(strings.TrimSpace(s), " ", 2)
	return parseDockerSize(parts[0])
}
```

### 2. Platform-Native Sparse Accounting & Inode Deduplication (`internal/fs`)

```go
// internal/fs/usage.go
package fs

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
)

type Usage struct {
	ApparentBytes  int64
	AllocatedBytes int64
	FileCount      int
	Partial        bool
	StatusNote     string
}

func CalcUsage(path string) (Usage, error) {
	return CalcUsageWithContext(context.Background(), path)
}

func CalcUsageWithContext(ctx context.Context, path string) (Usage, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return Usage{Partial: true, StatusNote: err.Error()}, err
	}

	// Single file case (e.g. Docker.raw)
	if !info.IsDir() {
		dev, ino, blocks, apparent, ok := getFileInfoStat(info)
		if !ok {
			return Usage{ApparentBytes: info.Size(), AllocatedBytes: info.Size(), FileCount: 1}, nil
		}
		alloc := blocks * 512
		return Usage{ApparentBytes: apparent, AllocatedBytes: alloc, FileCount: 1}, nil
	}

	// Directory tree with inode deduplication
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
			statusNote = "permission or access error on subpath"
			return nil
		}

		if d.IsDir() {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil // Skip symlinks to prevent duplicate accounting or escape
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
				return nil // Hardlink duplicate: do not add again
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

```go
// internal/fs/usage_darwin.go
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
	return uint64(uint32(stat.Dev)), stat.Ino, stat.Blocks, stat.Size, true
}
```

```go
// internal/fs/usage_linux.go
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
	return stat.Dev, stat.Ino, stat.Blocks, stat.Size, true
}
```

### 3. Fail-Closed Process Guard with Pre-Deletion Revalidation (OrbStack)

```go
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
		return false, false // Cleanly stopped
	}
	// Any other error (tool missing, permissions, uncertain state) -> Fail closed
	return false, true
}
```

### 4. Cleaner Direct Defensive Guards (`Clean()`)

```go
// In internal/cleaner/docker/docker.go
func (c *dockerCleaner) Clean(ctx context.Context, targets []domain.Target) error {
	for _, t := range targets {
		if t.ReviewOnly || t.Category == "docker-vm-disk" || strings.HasPrefix(t.Path, "docker://") {
			continue // Inviolable: never delete daemon storage or VM disk
		}
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

// In internal/cleaner/dotnet/dotnet.go
func (c *dotnetCleaner) Clean(_ context.Context, targets []domain.Target) error {
	for _, t := range targets {
		if t.ReviewOnly || t.Category == "nuget-packages" || t.Path == c.paths.DotNetNuGet {
			continue // Defensive guard: never delete shared store
		}
		_ = os.RemoveAll(t.Path)
	}
	return nil
}

// In internal/cleaner/pnpm/pnpm.go
func (c *pnpmCleaner) Clean(ctx context.Context, targets []domain.Target) error {
	for _, t := range targets {
		if t.ReviewOnly || t.Category == "pnpm-store" || t.Category == "virtual-pnpm-store" || strings.HasPrefix(t.Path, "pnpm://") {
			continue // Defensive guard: never delete or prune pnpm store
		}
		if t.Category == "pnpm-cache" && !safety.IsCoreBlacklisted(t.Path) {
			_ = os.RemoveAll(t.Path)
		}
	}
	return nil
}
```

---

## Validation Architecture

To satisfy Nyquist validation criteria (`VAL-01`, `VAL-02`) and achieve >85% test coverage without flakiness:

```
+-------------------------------------------------------------------------------+
|                           Automated Test Matrix                               |
+-------------------------------------------------------------------------------+
|  1. Inode Deduplication & Sparse Accounting (internal/fs/usage_test.go)        |
|     - Verify hardlinks on same dev/ino count bytes once.                      |
|     - Verify sparse file reports allocated != apparent without panic.         |
|     - Verify inaccessible subdirectories report Partial = true fail-closed.   |
|                                                                               |
|  2. Docker Cleaner Unit Tests (internal/cleaner/docker/docker_test.go)        |
|     - Valid docker system df JSON -> 4 categorized targets, ReviewOnly.       |
|     - Daemon offline / error -> docker-daemon-status (ambiguous_state).       |
|     - Malformed JSON lines -> partial handling + status target.               |
|     - Docker.raw host detection -> docker-vm-disk with sparse accounting.     |
|     - Clean() called on docker:// or VM disk -> 0 removals, sentinel intact.  |
|                                                                               |
|  3. OrbStack Cleaner Unit Tests (internal/cleaner/docker/orbstack_test.go)    |
|     - OrbStack running (pgrep exit 0) -> Cache is ReviewOnly (active_proc).   |
|     - OrbStack stopped (pgrep exit 1) -> Cache is Eligible, RiskLow.          |
|     - OrbStack uncertain (pgrep error) -> Cache is ReviewOnly (ambiguous).    |
|     - OrbStack VM/Data -> 100% ReviewOnly, allocated vs apparent metrics.     |
|     - Clean() pre-deletion revalidation aborts if OrbStack launched.         |
|                                                                               |
|  4. DotNet Cleaner Unit Tests (internal/cleaner/dotnet/dotnet_test.go)        |
|     - nuget-packages is strictly ReviewOnly (shared_store), EligibleBytes = 0.|
|     - nuget-http-cache and nuget-temp remain TTL eligible.                    |
|     - Clean() called on nuget-packages preserves sentinels.                   |
|                                                                               |
|  5. pnpm Cleaner Unit Tests (internal/cleaner/pnpm/pnpm_test.go)              |
|     - Hybrid discovery (CLI + disk paths) discovers store.                    |
|     - Store is strictly ReviewOnly (shared_store), EligibleBytes = 0.         |
|     - Clean() removes physical pnpm metadata cache, never prunes store.       |
|     - Clean() called with store path preserves sentinels.                     |
|                                                                               |
|  6. Coordinator Integration & DryRun Canaries                                 |
|     - Verify Stage 1 filters all Phase 10 ReviewOnly targets into             |
|       SkippedReviewOnly with warning messages.                                |
|     - Verify DryRun executes zero filesystem or CLI mutations.                |
|     - Verify zero-CGO build: CGO_ENABLED=0 go test ./...                      |
+-------------------------------------------------------------------------------+
```

### Dry-Run Canary & Hermetic Fixtures
- Tests use `t.TempDir()` with sentinel files (`package.json`, `canary.dll`, `store-marker.txt`).
- `MockCommandExecutor` controls all command responses (`docker system df`, `pnpm store path`, `pgrep -x OrbStack`), eliminating daemon and host process dependencies.
- Hermetic test assertions explicitly verify that sentinel files exist after `Clean()` execution across all shared store categories.

---

## RESEARCH COMPLETE

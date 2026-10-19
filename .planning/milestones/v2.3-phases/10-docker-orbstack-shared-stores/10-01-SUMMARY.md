---
phase: "10-docker-orbstack-shared-stores"
plan: "10-01"
title: "Domain ProviderMetrics Model and Zero-CGO Sparse Block Accounting & Hardlink Deduplication"
wave: 1
completed_at: "2026-10-16"
requirements:
  - STORE-01
  - STORE-02
  - STORE-03
status: "completed"
commits:
  - hash: "15995b6"
    message: "feat(domain,fs): add ProviderMetrics model and zero-CGO Stat_t sparse block accounting"
  - hash: "b830bc2"
    message: "feat(fs): implement hardlink deduplication, symlink skip, and fail-closed error degradation in CalcUsage"
coverage:
  internal/domain: "100.0%"
  internal/fs: "86.8%"
---

# Plan 10-01 Summary: Domain ProviderMetrics and Zero-CGO Filesystem Accounting

## Overview
O plano `10-01` da Fase 10 estabeleceu as fundações de domínio para métricas de provedores de containers e pacotes compartilhados (`domain.ProviderMetrics`), além da infraestrutura nativa em `internal/fs` (`CalcUsage`, `CalcUsageWithContext`) com zero CGO em Darwin e Linux. Essa camada calcula blocos físicos alocados (`st_blocks * 512`) versus tamanho aparente (`st_size`) em arquivos esparsos (ex: imagens de disco virtual `Docker.raw`, `data.img.raw`), deduplica hardlinks por tupla `(dev, ino)` para evitar inflação em package stores (ex: `pnpm store`), ignora links simbólicos para prevenir escape ou dupla contagem, e degrada de forma fail-closed (`Partial: true`, `StatusNote`) em acessos inacessíveis ou locks sem abortar o escaneamento.

---

## Principais Entregas

### 1. Modelo de Domínio `ProviderMetrics` (STORE-01, STORE-02, D-03)
- Adicionado struct `domain.ProviderMetrics` em `internal/domain/types.go` com os campos:
  - `TotalCount int`
  - `ActiveCount int`
  - `SizeBytes int64`
  - `ReclaimableBytes int64`
  - `ApparentBytes int64`
  - `AllocatedBytes int64`
  - `Partial bool`
  - `StatusNote string`
- Campo adicionado a `domain.Target`: `ProviderMetrics *ProviderMetrics `json:"provider_metrics,omitempty"``.
- Teste unitário hermético em `internal/domain/target_test.go` (`TestTarget_ProviderMetrics_Serialization`) validando round-trip JSON, serialização completa e omissão quando nil.

### 2. Extratores de Sistema Operacional Zero CGO (STORE-02, D-07)
- `internal/fs/usage_darwin.go` (`//go:build darwin`): Converte `syscall.Stat_t.Dev` de `int32` para `uint64` (via cast intermediário `uint32` prevenindo extensão negativa de sinal) e extrai `Ino`, `Blocks * 512` e `Size`.
- `internal/fs/usage_linux.go` (`//go:build linux`): Extrai `stat.Dev` (uint64 nativo), `Ino`, `Blocks * 512` e `Size` diretamente.
- `internal/fs/usage_other.go` (`//go:build !darwin && !linux`): Fallback seguro sem CGO retornando `info.Size()` para tamanho e alocação.

### 3. Motor de Cálculo de Uso e Deduplicação por Inodes (`internal/fs/usage.go`) (STORE-02, STORE-03, D-07, D-11, D-16)
- API pública `CalcUsage(path string) (Usage, error)` e `CalcUsageWithContext(ctx context.Context, path string) (Usage, error)`.
- Arquivo único (e.g. `Docker.raw`): cálculo imediato de bytes aparentes e alocados via blocos físicos.
- Árvore de diretórios:
  - Rastreamento de `seenInodes[ [2]uint64{dev, ino} ]` deduplicando entradas de hardlinks idênticos.
  - Ignora symlinks (`d.Type()&os.ModeSymlink != 0`) sem seguir referências externas.
  - Fail-closed em subdiretórios inacessíveis (permissões, locks): define `Partial: true` e `StatusNote`, acumulando os bytes já encontrados.
  - Suporte a cancelamento por contexto (`ctx.Err()`).

---

## Tarefas Executadas

### Task 10-01-01 [TDD]: Domain ProviderMetrics model and zero-CGO Stat_t sparse block accounting
- **RED**: Adicionado teste `TestTarget_ProviderMetrics_Serialization` em `internal/domain/target_test.go` e `TestCalcUsage_SingleFile_Sparse` em `internal/fs/usage_test.go`. Compilação falhou com símbolos indefinidos.
- **GREEN**: Implementados `ProviderMetrics` e `Target.ProviderMetrics` em `internal/domain/types.go`, extratores em `internal/fs/usage_darwin.go`, `usage_linux.go`, `usage_other.go`, e `CalcUsage` inicial em `internal/fs/usage.go`.
- **Commit**: `15995b6` `feat(domain,fs): add ProviderMetrics model and zero-CGO Stat_t sparse block accounting`

### Task 10-01-02 [TDD]: Hardlink inode deduplication, tree traversal, symlink skip, and fail-closed partial/lock reporting
- **RED**: Adicionados testes `TestCalcUsage_Hardlinks_Deduplication`, `TestCalcUsage_Symlinks_Skipped`, `TestCalcUsage_PartialLock_FailClosed` e `TestCalcUsage_ContextCancellation` em `internal/fs/usage_test.go`. Falhas confirmadas em diretórios não implementados.
- **GREEN**: Implementada a varredura completa de diretórios em `CalcUsageWithContext` com deduplicação `seenInodes`, pulo de symlinks, degradação parcial e propagação de cancelamento.
- **Commit**: `b830bc2` `feat(fs): implement hardlink deduplication, symlink skip, and fail-closed error degradation in CalcUsage`

---

## Verificação e Qualidade
- `go test -v ./internal/domain -run "TestTarget_ProviderMetrics"`: PASS
- `go test -v ./internal/fs -run "TestCalcUsage"`: PASS (5/5 testes passando)
- Cobertura: `internal/domain` 100.0%, `internal/fs` 86.8%
- Zero CGO confirmado via compilação cruzada para `GOOS=linux` e `GOOS=windows`.

---
phase: "10-docker-orbstack-shared-stores"
plan: "10-04"
title: "Shared Stores Protection, Engine Wiring, and DryRun Zero-Mutation Canary"
wave: 4
completed_at: "2026-10-16"
requirements:
  - STORE-01
  - STORE-02
  - STORE-03
  - POL-01
  - POL-02
  - VAL-01
  - VAL-02
status: "completed"
commits:
  - hash: "c379d5e"
    message: "feat(cleaner/dotnet): lock nuget packages to ReviewOnly and enforce Clean defensive guard"
  - hash: "05a87d9"
    message: "feat(cleaner/pnpm): hybrid store discovery, ReviewOnly locking, and engine wiring"
  - hash: "f299767"
    message: "test(deleter): add Phase 10 zero-mutation canary and boost test coverage"
coverage:
  internal/cleaner/docker: "88.9%"
  internal/cleaner/dotnet: "88.1%"
  internal/cleaner/pnpm: "92.9%"
  internal/fs: "86.8%"
---

# Plan 10-04 Summary: Shared Stores Protection, Engine Wiring, and DryRun Zero-Mutation Canary

## Overview
O plano `10-04` encerrou com sucesso a Fase 10 do DevClean implementando as proteções finais para repositórios compartilhados (`STORE-03`), a injeção do executor customizado no registro do motor (`STORE-01`, `D-04`) e a comprovação cabal do invariante de mutação zero (`POL-01`, `POL-02`, `VAL-01`) sob o `deleter.Coordinator` em modo DryRun.

---

## Principais Entregas

### 1. Proteção Estrita do Repositório NuGet (STORE-03, D-09, D-12)
- **Bloqueio Incondicional de `nuget-packages`**: O diretório global de pacotes NuGet (`~/.nuget/packages`) foi configurado como estritamente `ReviewOnly: true`, com `ReviewReason: domain.ReviewReasonSharedStore`, `EligibleBytes: 0` e `TTLEligible: false`, independentemente de regras de `MaxAgeDays`.
- **Elegibilidade Mantida para Caches Regeneráveis**: Os alvos `nuget-http-cache` (`~/.local/share/NuGet/v3-cache` ou similar) e `nuget-temp` (`~/.local/share/NuGet/Temp`), assim como as pastas locais de compilação `dotnet-build` (`bin/` e `obj/`), continuam elegíveis para limpeza conforme TTL.
- **Guarda Defensiva Inviolável no `Clean()`**: O método `(c *dotnetCleaner) Clean()` implementa verificação direta que rejeita e ignora qualquer alvo marcado com `ReviewOnly`, de categoria `nuget-packages` ou cujo caminho coincida com `c.paths.DotNetNuGet`, além de validações contra symlinks e lista negra central.

### 2. Descoberta Híbrida do Repositório pnpm e Inode Deduplication (STORE-03, D-10, D-11, D-12)
- **Descoberta Híbrida**: Implementado método `discoverStorePaths` que consulta a CLI `pnpm store path` e examina diretórios canônicos no disco (`~/Library/pnpm/store`, `~/.local/share/pnpm/store`, `~/.pnpm-store`), aplicando resolução de links simbólicos (`filepath.EvalSymlinks`) e limpeza de caminhos duplicados.
- **Métricas Dedup e Bloqueio `ReviewOnly`**: O tamanho do repositório é calculado via `devfs.CalcUsage`, populando `ProviderMetrics` com `AllocatedBytes`, `ApparentBytes` e `TotalCount` sem inflação provocada por hardlinks. O alvo é marcado como `ReviewOnly: true`, com `ReviewReasonSharedStore`, `EligibleBytes: 0` e `TTLEligible: false`.
- **Remoção de `pnpm store prune`**: Eliminada qualquer invocação de `pnpm store prune` ou do alvo virtual de poda. O método `Clean()` preserva sentinelas intactos e exclui unicamente caches HTTP de metadados (`pnpm-cache`).

### 3. Fiação do Cleaner Docker no Registro do Motor (STORE-01, D-04)
- Em `internal/engine/register.go`, a função `RegisterDefaultCleanersWithExecutor` conecta o cleaner do Docker passando `cleaner.CommandExecutor` via `docker.NewWithExecutor(exec)`, viabilizando testes herméticos e injeção transparente de executores.

### 4. Canary de Mutação Zero e Verificação do Repositório (POL-01, POL-02, VAL-01, VAL-02)
- **`TestCoordinator_DryRun_ZeroMutation_Phase10`**: Criado teste abrangente no `internal/deleter/policy_test.go` cobrindo todas as 8 categorias de alvos da Fase 10 (Docker VM disk, Docker images, OrbStack VM data, OrbStack cache, NuGet packages, NuGet HTTP cache, pnpm store e pnpm metadata cache).
- **Integridade Criptográfica SHA-256**: Validação rigorosa de que todos os arquivos sentinelas (inclusive com hardlinks) conservam 100% de integridade com hashes SHA-256, tamanhos de bytes e timestamps de modificação idênticos pré e pós-execução DryRun.
- **Lixeira e Histórico Limpos**: A pasta `trashDir` permaneceu estritamente vazia sem criação de `manifest.json`, e `history.jsonl` não sofreu mutações.
- **Cobertura de Código Superior a 85%**:
  - `internal/cleaner/docker`: 88.9%
  - `internal/cleaner/dotnet`: 88.1%
  - `internal/cleaner/pnpm`: 92.9%
  - `internal/fs`: 86.8%
- **Build Multiplataforma Zero-CGO**: Compilações sem CGO para Darwin e Linux aprovadas limpas (`CGO_ENABLED=0 GOOS=darwin/linux go build ./...`).

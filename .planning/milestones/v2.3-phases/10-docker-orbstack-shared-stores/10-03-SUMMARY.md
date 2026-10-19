---
phase: "10-docker-orbstack-shared-stores"
plan: "10-03"
title: "OrbStack Sub-Scanner with Cache vs VM/Data Separation, Sparse Accounting, and Process Guard"
wave: 3
completed_at: "2026-10-16"
requirements:
  - STORE-02
  - STORE-03
status: "completed"
commits:
  - hash: "d4b57a7"
    message: "feat(cleaner/docker): implement OrbStack sub-scanner with cache vs VM data separation and sparse metrics"
  - hash: "69a3850"
    message: "feat(cleaner/docker): test and enforce OrbStack Process Guard, fail-closed handling, and Clean revalidation"
coverage:
  internal/cleaner/docker: "88.9%"
---

# Plan 10-03 Summary: OrbStack Sub-Scanner, Sparse Metrics, and Process Guard

## Overview
O plano `10-03` da Fase 10 implementou o sub-scanner do OrbStack integrado diretamente no ecossistema `"docker"` (`internal/cleaner/docker/orbstack.go`), sem a necessidade de criar um ecossistema separado na configuração (`STORE-02`, `D-05`). O scanner realiza uma separação estrita entre caches regeneráveis de host (`~/Library/Caches/dev.orbstack.OrbStack`, `~/.cache/orbstack`) e armazenamento persistente de containers e máquinas virtuais (`~/Library/Group Containers/*.dev.orbstack/data`, `~/.orbstack/data`, `~/.orbstack`) (`STORE-02`, `D-06`). O armazenamento de VM/Data é trancado 100% como `ReviewOnly: true`, com `EligibleBytes: 0`, `ReviewReason: domain.ReviewReasonSharedStore` e medição honesta de blocos alocados vs tamanho aparente via `fs.CalcUsage` (`STORE-02`, `STORE-03`, `D-07`). Além disso, um Process Guard rigoroso consulta `pgrep -x OrbStack` com semântica fail-closed e impõe revalidação pré-exclusão no método `Clean()`, garantindo que nenhuma exclusão de cache ocorra se o OrbStack estiver rodando ou em estado incerto (`STORE-02`, `D-08`, `D-12`).

---

## Principais Entregas

### 1. Separação Estrita entre Cache Regenerável e VM/Data (STORE-02, STORE-03, D-06)
- **OrbStack Cache (`orbstack-cache`)**:
  - Caminhos: `~/Library/Caches/dev.orbstack.OrbStack` (macOS), `~/.cache/orbstack` (Linux).
  - Classificado com `RiskLow` e elegível para limpeza conforme TTL somente quando o OrbStack estiver comprovadamente finalizado.
- **OrbStack VM/Data (`orbstack-data`)**:
  - Caminhos: `~/Library/Group Containers/*.dev.orbstack/data`, `~/Library/Group Containers/group.dev.orbstack/data`, `~/.orbstack/data`, `~/.orbstack`.
  - Trancado incondicionalmente como `ReviewOnly: true`, `ReviewReason: domain.ReviewReasonSharedStore`, `EligibleBytes: 0` e `TTLEligible: false`.
  - Interrompe a varredura após o primeiro diretório raiz correspondente, evitando contagem duplicada de caminhos aninhados.

### 2. Métricas de Arquivos Esparsos de VM e Deduplicação (STORE-02, D-07)
- Utilização de `fs.CalcUsage` para calcular `AllocatedBytes` (`st_blocks * 512`) e `ApparentBytes` (`st_size`).
- O campo `SizeBytes` reporta exclusivamente os bytes fisicamente alocados no disco (`AllocatedBytes`), enquanto `EligibleBytes` permanece estritamente `0`.
- O struct estruturado `ProviderMetrics` é populado com `AllocatedBytes`, `ApparentBytes`, status de leitura parcial (`Partial`) e nota de status.
- O campo `Name` rotula de forma inequívoca o espaço físico alocado vs aparente (ex: `OrbStack VM/Data (allocated: 5.1 GB, apparent: 494.3 GB)`), eliminando relatórios alarmistas ou recuperações ilusórias.

### 3. Process Guard Fail-Closed com Revalidação Pré-Deleção (STORE-02, D-08, D-12)
- Consulta do processo via `pgrep -x OrbStack` encapsulada em `cleaner.CommandExecutor` sem injeção de shell:
  - **Saída com PID (exit 0)**: `running = true`, `uncertain = false` $\rightarrow$ `ReviewOnly: true`, `ReviewReason: domain.ReviewReasonActiveProcess`, `EligibleBytes: 0`.
  - **Processo ausente (exit 1)**: `running = false`, `uncertain = false` $\rightarrow$ `ReviewOnly: false`, elegível conforme TTL.
  - **Erro inesperado / binário ausente / saída vazia no exit 0**: `uncertain = true` $\rightarrow$ FAIL CLOSED: `ReviewOnly: true`, `ReviewReason: domain.ReviewReasonAmbiguous`, `EligibleBytes: 0`.
- O método `(c *dockerCleaner) Clean()` revalida o estado do processo imediatamente antes de invocar `os.RemoveAll` sobre `orbstack-cache`. Se o usuário tiver iniciado o OrbStack entre a varredura e a limpeza, a exclusão é abortada silenciosamente e os arquivos permanecem intactos.
- `Clean()` rejeita peremptoriamente qualquer alvo com `ReviewOnly: true`, `Category == "orbstack-data"` ou tentativa de bypass de ID/Path.

### 4. Integração no Cleaner Docker e Detecção Híbrida (STORE-02, D-05, D-13)
- `dockerCleaner.Scan()` aciona `scanOrbStack` e agrega os alvos descobertos ao inventário do Docker.
- `dockerCleaner.IsInstalled()` reconhece diretórios do OrbStack (`~/.orbstack`, `~/Library/Group Containers/*dev.orbstack`, etc.) ou executável `orb` no `PATH`, garantindo diagnóstico mesmo quando o daemon clássico do Docker não estiver ativo.

---

## Tarefas Executadas

### Task 10-03-01 [TDD]: Implement OrbStack sub-scanner under 'docker' ecosystem separating regenerable cache from ReviewOnly VM/Data with sparse metrics
- **RED**: Escritos os testes `TestOrbStack_Scan_CacheAndDataSeparation` e `TestOrbStack_VMData_StrictlyReviewOnly` em `internal/cleaner/docker/orbstack_test.go`. Falha inicial confirmada com símbolos indefinidos.
- **GREEN**: Criado `internal/cleaner/docker/orbstack.go` com `scanOrbStack`, cálculo de sparse accounting via `devfs.CalcUsage`, rotulação em `ProviderMetrics` e integração em `docker.go`.
- **Commit**: `d4b57a7` `feat(cleaner/docker): implement OrbStack sub-scanner with cache vs VM data separation and sparse metrics`

### Task 10-03-02 [TDD]: OrbStack Process Guard via pgrep -x, fail-closed uncertain state handling, and pre-deletion Clean() revalidation
- **RED**: Adicionados testes `TestOrbStack_ProcessGuard_Running`, `TestOrbStack_ProcessGuard_Stopped_Eligible`, `TestOrbStack_ProcessGuard_Uncertain_FailClosed`, `TestOrbStack_Clean_PreDeletionRevalidation` e `TestOrbStack_Clean_PreservesVMDataSentinel`.
- **GREEN**: Implementadas funções `checkOrbStackProcess` e `cleanOrbStackCache` com semântica fail-closed para erros de execução, verificação contra `safety.IsCoreBlacklisted` e guarda intransponível em `Clean()`.
- **Commit**: `69a3850` `feat(cleaner/docker): test and enforce OrbStack Process Guard, fail-closed handling, and Clean revalidation`

---

## Verificação e Qualidade
- `go test -v ./internal/cleaner/docker -run "TestOrbStack_"`: PASS (7/7 testes específicos de OrbStack passando)
- `go test -v -cover ./internal/cleaner/docker`: PASS (12/12 testes passando, cobertura de declarações: 88.9%)
- Testes herméticos isolam `$HOME` usando `t.TempDir()`, impedindo contaminação cruzada por artefatos do host.
- Zero CGO mantido em toda a implementação.

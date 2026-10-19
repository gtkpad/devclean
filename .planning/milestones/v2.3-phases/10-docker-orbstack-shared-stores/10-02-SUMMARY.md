---
phase: "10-docker-orbstack-shared-stores"
plan: "10-02"
title: "Docker Cleaner Standardized on CommandExecutor with system df Parsing, ReviewOnly Locking, and VM Disk Discovery"
wave: 2
completed_at: "2026-10-16"
requirements:
  - STORE-01
  - STORE-03
status: "completed"
commits:
  - hash: "8d9bf97"
    message: "feat(cleaner/docker): standardize on CommandExecutor and parse system df into categorized ReviewOnly targets"
  - hash: "fb0432e"
    message: "feat(cleaner/docker): fail-closed daemon error handling, malformed JSON tolerance, and hybrid IsInstalled"
  - hash: "512fd8c"
    message: "feat(cleaner/docker): discover Docker.raw with sparse metrics and enforce defensive Clean guard"
coverage:
  internal/cleaner/docker: "86.3%"
---

# Plan 10-02 Summary: Docker Cleaner Refactor, Categorized ReviewOnly Targets, and VM Disk Accounting

## Overview
O plano `10-02` da Fase 10 refatorou integralmente o cleaner `docker` (`internal/cleaner/docker/docker.go`), eliminando as dependências pesadas do SDK Docker (`github.com/docker/docker/...`) em favor de `cleaner.CommandExecutor` injetável (`NewWithExecutor`). O cleaner agora consulta o daemon via `docker system df --format "{{json .}}"`, analisa os streams de JSON em 4 alvos virtuais categorizados (`docker-images`, `docker-containers`, `docker-volumes`, `docker-build-cache`) e os tranca incondicionalmente como `ReviewOnly: true`, `ReviewReason: domain.ReviewReasonSharedStore` e `EligibleBytes: 0`. Além disso, implementa tratamento fail-closed para erros de daemon (`docker-daemon-status`) e linhas malformadas (`docker-daemon-parse-error`), descobre imagens de disco de VM do Docker Desktop no host (`Docker.raw`) com métricas de blocos físicos alocados vs tamanho aparente, e impõe uma guarda defensiva intransponível no método `Clean()`.

---

## Principais Entregas

### 1. Padronização sobre `cleaner.CommandExecutor` e Remoção de SDK (STORE-01, D-04)
- Eliminados todos os imports e tipos dependentes de `github.com/docker/docker/...`.
- `dockerCleaner` agora recebe `cleaner.CommandExecutor` via `NewWithExecutor(exec)`, enquanto `New()` delega para `NewWithExecutor(&cleaner.DefaultExecutor{})`.
- Todos os comandos são executados diretamente por fatias de argumentos (`exec.Output(ctx, "docker", "system", "df", "--format", "{{json .}}")`), prevenindo injeção de shell e garantindo testes 100% herméticos sem necessidade de daemons ou CGO.

### 2. Alvos Categorizados e Métricas Ricas do `docker system df` (STORE-01, STORE-03, D-01, D-02, D-03)
- Análise de stream JSON mapeando:
  - `Images`: ID `docker-images`, Path `docker://system-df/images`, Category `docker-images`
  - `Containers`: ID `docker-containers`, Path `docker://system-df/containers`, Category `docker-containers`
  - `Local Volumes`: ID `docker-volumes`, Path `docker://system-df/volumes`, Category `docker-volumes`
  - `Build Cache`: ID `docker-build-cache`, Path `docker://system-df/build-cache`, Category `docker-build-cache`
- Todos os 4 alvos possuem incondicionalmente `ReviewOnly: true`, `ReviewReason: domain.ReviewReasonSharedStore`, `EligibleBytes: 0`, `TTLEligible: false` e `Risk: domain.RiskLow`.
- População precisa de `ProviderMetrics` (`TotalCount`, `ActiveCount`, `SizeBytes`, `ReclaimableBytes`) e rotulação descritiva no `Name` (ex: `Docker Images (5 total, 2 active, 2.4 GB, 1.1 GB reclaimable)`).
- Parser flexível para inteiros (`parseFlexibleInt`) suportando tanto números puros quanto strings numéricas com aspas.
- Conversor de tamanhos (`parseDockerSize` e `parseDockerReclaimable`) com suporte a unidades binárias e decimais (B, KB, MB, GB, TB, KiB, MiB, GiB, TiB).

### 3. Tratamento Fail-Closed de Erros do Daemon e Tolerância a JSON Malformado (STORE-01, D-13, D-14)
- `IsInstalled()` híbrido: verifica binários no `PATH` (`docker`, `orb`) OU diretórios canônicos de instalação/cache no disco (`~/.docker`, `~/Library/Containers/com.docker.docker`, `~/.orbstack`, etc.) sem exigir que o daemon esteja ativo ou responda a pings.
- Se o comando `docker system df` falhar (daemon offline, socket inacessível, permissão negada) e o Docker estiver instalado, emite alvo explícito `ID: "docker-daemon-status"` (`Path: "docker://daemon/status"`, `ReviewOnly: true`, `ReviewReason: domain.ReviewReasonAmbiguous`, `EligibleBytes: 0`, `ProviderMetrics.Partial: true`), eliminando retornos silenciosos de 0 bytes.
- Se a saída do daemon contiver linhas de JSON truncadas ou corrompidas, preserva os alvos válidos e emite um alerta explícito `ID: "docker-daemon-parse-error"` (`ReviewReasonAmbiguous`, `Partial: true`).

### 4. Descoberta de VM Disk do Docker Desktop e Guarda Defensiva em `Clean()` (STORE-01, STORE-02, STORE-03, D-02, D-12, D-15)
- Inspeção em disco dos caminhos do `Docker.raw` (macOS e Linux).
- Medição com `fs.CalcUsage(p)` expondo `AllocatedBytes` (blocos reais no APFS/ext4) e `ApparentBytes` (capacidade máxima esparsa), rotulado explicitamente em `Name` e `ProviderMetrics`.
- Trancado como `ReviewOnly: true`, `ReviewReason: domain.ReviewReasonSharedStore`, `EligibleBytes: 0`, `Category: "docker-vm-disk"`.
- O método `(c *dockerCleaner) Clean(ctx, targets)` impõe guarda estrita ignorando qualquer alvo com `ReviewOnly == true`, `Category == "docker-vm-disk"` ou `strings.HasPrefix(Path, "docker://")`, impedindo que chamadas diretas ou bypasses apaguem ou mutem dados do daemon ou discos virtuais de VM.

---

## Tarefas Executadas

### Task 10-02-01 [TDD]: Standardize Docker cleaner on CommandExecutor, parse system df JSON, and emit 4 categorized ReviewOnly targets
- **RED**: Escrito teste `TestDockerCleaner_SystemDF_CategorizedReviewOnly` em `internal/cleaner/docker/docker_test.go` exigindo `NewWithExecutor`, 4 categorias e métricas. Falha confirmada com símbolos indefinidos.
- **GREEN**: Removido SDK do Docker, implementado `dockerCleaner` com `cleaner.CommandExecutor`, parsing de `docker system df` JSON, suporte a números flexíveis e formatação de alvos `ReviewOnly`.
- **Commit**: `8d9bf97` `feat(cleaner/docker): standardize on CommandExecutor and parse system df into categorized ReviewOnly targets`

### Task 10-02-02 [TDD]: Fail-closed daemon error handling with explicit docker-daemon-status, malformed JSON line tolerance, and disk/CLI hybrid IsInstalled()
- **RED**: Adicionados testes `TestDockerCleaner_DaemonOffline_StatusTarget`, `TestDockerCleaner_MalformedJSON_PartialTolerance` e `TestDockerCleaner_IsInstalled_HybridDiscovery`. Falhas confirmadas em saídas com erro e linhas corrompidas.
- **GREEN**: Implementada a detecção híbrida em `IsInstalled()` e emitidos alvos `docker-daemon-status` e `docker-daemon-parse-error` com `ReviewReasonAmbiguous` e `Partial: true`.
- **Commit**: `fb0432e` `feat(cleaner/docker): fail-closed daemon error handling, malformed JSON tolerance, and hybrid IsInstalled`

### Task 10-02-03 [TDD]: Host Docker Desktop VM Disk (Docker.raw) discovery with sparse metrics and defensive Clean() guard
- **RED**: Adicionados testes `TestDockerCleaner_VMDisk_SparseMetrics` e `TestDockerCleaner_Clean_DefensiveGuard_PreservesAll`. Falha confirmada na ausência do alvo `docker-vm-disk`.
- **GREEN**: Implementada a descoberta de `Docker.raw` via `scanDockerVMDisk()` com `fs.CalcUsage`, e guarda defensiva no método `Clean()`.
- **Commit**: `512fd8c` `feat(cleaner/docker): discover Docker.raw with sparse metrics and enforce defensive Clean guard`

---

## Verificação e Qualidade
- `go test -v ./internal/cleaner/docker`: PASS (8/8 testes passando)
- Cobertura em `internal/cleaner/docker`: 86.3%
- `go test -v ./internal/cleaner -run "TestCleaners_IsInstalled"`: PASS (todas as 19 ferramentas com resposta imediata < 100ms)
- Verificação estrita de segurança: `Clean()` testado com tentativas de bypass garantindo retenção de arquivos sentinela.

# Phase 10: Docker, OrbStack & Shared Stores - Context

**Gathered:** 2026-10-16  
**Status:** Ready for planning  

<domain>
## Phase Boundary

Garantir a inspeção precisa e honesta de espaço em provedores de containers (`Docker`, `OrbStack`) e stores compartilhados de pacotes (`NuGet`, `pnpm store`), separando caches regeneráveis de armazenamento de VM/dados e tratando incondicionalmente todo armazenamento gerenciado por daemon ou compartilhado entre projetos como `ReviewOnly` (`⊙`):
1. `docker` cleaner (`internal/cleaner/docker/docker.go`): padroniza sobre `cleaner.CommandExecutor` para consultar `docker system df --format "{{json .}}"` e emite alvos por categoria (`Images`, `Containers`, `Local Volumes`, `Build Cache`) expondo métricas detalhadas (`total`, `active`, `size`, `reclaimable`) em `ProviderMetrics` e no rótulo, mantendo métricas lógicas do daemon separadas de arquivos de VM no host (`Docker.raw`) (`STORE-01`, `STORE-03`).
2. `orbstack` sub-scanner (`internal/cleaner/docker/orbstack.go`): separa caches regeneráveis (`~/Library/Caches/dev.orbstack.OrbStack`, `~/.cache/orbstack`) do armazenamento de VM/containers (`~/Library/Group Containers/*.dev.orbstack/data`, `~/.orbstack`), medindo bytes alocados reais (`st_blocks * 512`) vs tamanho aparente (`st_size`) sem inflar ou somar valores esparsos como recuperáveis, e aplicando Process Guard (`pgrep -x OrbStack`) sobre os caches regeneráveis (`STORE-02`, `STORE-03`).
3. Shared package stores (`internal/cleaner/dotnet/dotnet.go` e `internal/cleaner/pnpm/pnpm.go`): marca `~/.nuget/packages` (`nuget-packages`) e `pnpm store` (`~/Library/pnpm/store`, `~/.local/share/pnpm/store`, `~/.pnpm-store`, `pnpm store path`) incondicionalmente como `ReviewOnly = true` (`ReviewReasonSharedStore`, `EligibleBytes = 0`, `TTLEligible = false`), deduplicando inodes (`dev, ino`) contra inflação de hardlinks e preservando a elegibilidade de `nuget-http-cache`, `nuget-temp`, `dotnet-build` e `pnpm metadata cache` (`STORE-03`).
4. Estados de erro, ausência de provedor e fail-closed: falhas de daemon, erros de socket, JSON malformado, locks ou erros parciais de permissão em shared stores produzem estados explícitos de revisão (`ReviewOnly = true`, `ReviewReasonAmbiguous`, `EligibleBytes = 0`) em vez de sucesso silencioso de zero bytes, e os métodos `Clean()` de `docker`, `dotnet` e `pnpm` impõem guarda defensiva contra qualquer mutação em alvos `ReviewOnly` (`STORE-01`, `STORE-03`, `VAL-01`).

</domain>

<decisions>
## Implementation Decisions

### Métricas do Docker e Granularidade ReviewOnly (STORE-01, STORE-03)
- **D-01:** **Alvos por Categoria do `docker system df`:** Emitir 1 alvo por categoria retornada pelo `docker system df --format "{{json .}}"` (`Images`, `Containers`, `Local Volumes`, `Build Cache`) sob caminhos virtuais `docker://system-df/<type>`, mantendo as métricas lógicas do daemon estritamente separadas de arquivos de disco de VM no host (`Docker.raw`). — **Reversibility:** reversible — define a granularidade de apresentação de alvos do Docker.
- **D-02:** **Armazenamento Docker 100% ReviewOnly:** Todos os alvos de armazenamento do Docker (`Images`, `Containers`, `Local Volumes`, `Build Cache` e `Docker Desktop VM Disk`) são incondicionalmente marcados com `ReviewOnly: true`, `ReviewReason: domain.ReviewReasonSharedStore`, `EligibleBytes: 0` e `TTLEligible: false`. O método `Clean()` do Docker não apaga imagens, volumes ou build cache do daemon; só permite limpar caches físicos regeneráveis de host (como `OrbStack Cache` quando o OrbStack estiver fechado). — **Reversibility:** costly — altera o contrato de mutação de `internal/cleaner/docker` para cumprir `STORE-03`.
- **D-03:** **Metadados Estruturados `ProviderMetrics` em `domain.Target`:** Adicionar o campo opcional `ProviderMetrics *ProviderMetrics` (`json:"provider_metrics,omitempty"`) em `domain.Target` contendo `TotalCount int`, `ActiveCount int`, `SizeBytes int64`, `ReclaimableBytes int64`, `ApparentBytes int64`, `AllocatedBytes int64`, `Partial bool` e `StatusNote string`, além de resumo legível no `Name`, garantindo fidelidade tanto na CLI/TUI quanto na serialização JSON/MCP. — **Reversibility:** costly — estende o struct central `domain.Target` consumido por CLI, TUI e MCP.
- **D-04:** **Padronização sobre `cleaner.CommandExecutor`:** Refatorar `internal/cleaner/docker` para utilizar `cleaner.CommandExecutor` (`NewWithExecutor(exec)`), executando `docker system df --format "{{json .}}"` sem acoplamento direto ao SDK pesado do Docker, garantindo testes unitários 100% herméticos e determinísticos sem CGO. — **Reversibility:** costly — substitui `dockerAPI`/`sdkAPI` pela abstração unificada `cleaner.CommandExecutor` em `docker.go` e `register.go`.

### OrbStack: Cache Regenerável vs VM/Data e Sparse Accounting (STORE-02, STORE-03)
- **D-05:** **Integração do OrbStack no Pacote `internal/cleaner/docker`:** Implementar a descoberta e limpeza do OrbStack dentro de `internal/cleaner/docker` (ex: `orbstack.go`) sob o ecossistema `"docker"`, alinhado à Frente 2 da `SPEC-v2.3-MOLE-PARITY.md` sem criar um ecossistema extra na configuração. — **Reversibility:** costly — define a organização de pacotes e registro no motor.
- **D-06:** **Separação Estrita entre Cache Regenerável e VM/Data do OrbStack:** Emitir alvos distintos: `OrbStack Cache` (`~/Library/Caches/dev.orbstack.OrbStack` no macOS e `~/.cache/orbstack` no Linux) como cache regenerável elegível para limpeza (`RiskLow` quando o OrbStack estiver inativo), e `OrbStack VM/Data` (`~/Library/Group Containers/*.dev.orbstack/data` e `~/.orbstack/data` / `~/.orbstack`) como armazenamento incondicionalmente `ReviewOnly: true`, `ReviewReason: domain.ReviewReasonSharedStore` e `EligibleBytes: 0`. — **Reversibility:** reversible.
- **D-07:** **Contabilidade de Arquivos Esparsos (`Allocated` vs `Apparent`) e Deduplicação de Inodes:** Criar helper em `internal/fs` (`CalcDirUsage`) usando `syscall.Stat_t` (sem CGO) em Darwin/Linux para contabilizar blocos alocados reais (`Blocks * 512`) e tamanho aparente (`Size`) deduplicando hardlinks por `(Dev, Ino)`. Nos alvos de VM/Data (`OrbStack VM/Data` e `Docker Desktop VM Disk`), `SizeBytes` reflete os bytes fisicamente alocados (`AllocatedBytes`), `EligibleBytes` é `0`, e `ProviderMetrics` + `Name` rotulam explicitamente `allocated` vs `apparent` sem jamais somá-los como bytes recuperáveis. — **Reversibility:** costly — adiciona primitiva de medição de blocos/inodes em `internal/fs`.
- **D-08:** **Process Guard Fail-Closed para OrbStack:** Verificar processos ativos do OrbStack (`pgrep -x OrbStack`) via `cleaner.CommandExecutor`. Se o OrbStack estiver em execução ou a detecção retornar erro/estado incerto, marcar `OrbStack Cache` com `ReviewOnly: true`, `ReviewReason: domain.ReviewReasonActiveProcess`, `EligibleBytes: 0` e `TTLEligible: false`, revalidando imediatamente antes da exclusão no `Clean()`. — **Reversibility:** reversible.

### Shared Stores de Pacotes: NuGet e pnpm store (STORE-03)
- **D-09:** **Proteção ReviewOnly Exclusiva para `nuget-packages` no Cleaner .NET:** Em `internal/cleaner/dotnet/dotnet.go`, marcar `~/.nuget/packages` (`nuget-packages`) sempre com `ReviewOnly: true`, `ReviewReason: domain.ReviewReasonSharedStore`, `EligibleBytes: 0` e `TTLEligible: false`, mantendo `nuget-http-cache`, `nuget-temp` e `dotnet-build` (`bin/` e `obj/` locais) elegíveis para limpeza conforme TTL. — **Reversibility:** reversible.
- **D-10:** **Descoberta Híbrida do `pnpm store` por CLI e Presença em Disco:** Em `internal/cleaner/pnpm/pnpm.go`, descobrir o caminho do `pnpm store` consultando `pnpm store path` via `CommandExecutor` E verificando os candidatos canônicos em disco (`~/Library/pnpm/store`, `~/.local/share/pnpm/store`, `~/.pnpm-store`), resolvendo symlinks (`filepath.EvalSymlinks` / `filepath.Clean`) para deduplicar e garantir descoberta mesmo quando o binário `pnpm` não estiver no `PATH` (`SCAN-02`). — **Reversibility:** reversible.
- **D-11:** **Travamento ReviewOnly do `pnpm store` e Remoção de Prune Automático:** Marcar o alvo `pnpm store` incondicionalmente com `ReviewOnly: true`, `ReviewReason: domain.ReviewReasonSharedStore`, `EligibleBytes: 0` e `TTLEligible: false`, usando medição deduplicada por inodes (`CalcDirUsage`) para não inflar o tamanho por hardlinks. Remover a execução de `pnpm store prune` de dentro de `Clean()`, mantendo apenas o `pnpm metadata cache` (`~/Library/Caches/pnpm` e `~/.cache/pnpm`) elegível para deleção física. — **Reversibility:** costly — altera o contrato de mutação de `internal/cleaner/pnpm` para cumprir `STORE-03`.
- **D-12:** **Guarda Defensiva Direta no `Clean()` de `docker`, `dotnet` e `pnpm`:** Além do bloqueio no Estágio 1 do `deleter.Coordinator`, os próprios métodos `Clean()` de `docker`, `dotnet` e `pnpm` devem ignorar/recusar imediatamente qualquer `Target` com `t.ReviewOnly == true` ou cuja categoria/caminho pertença a um shared store (`nuget-packages`, `pnpm-store`, `docker://system-df/*`, `docker-vm-disk`, `orbstack-data`), garantindo que chamadas diretas em testes ou código nunca apaguem sentinelas de stores. — **Reversibility:** reversible.

### Estados de Erro do Daemon, Ausência de Provedor e Fail-Closed (STORE-01, STORE-03)
- **D-13:** **`IsInstalled()` Baseado em Disco e CLI no Cleaner Docker/OrbStack:** `IsInstalled()` retorna `true` se `docker` ou `orb` existirem no `PATH` (via `exec.LookPath`) OU se qualquer diretório de estado/cache em disco do Docker ou OrbStack existir (`~/.docker`, `~/Library/Containers/com.docker.docker`, `~/Library/Group Containers/*.dev.orbstack`, `~/Library/Caches/dev.orbstack.OrbStack`, `~/.orbstack`, `~/.cache/orbstack`), sem exigir que o daemon responda a `ping`. — **Reversibility:** reversible.
- **D-14:** **Alvo Explícito de Estado Incerto em Falhas do Daemon ou JSON Malformado:** Quando o binário `docker` ou diretórios do Docker estiverem presentes mas `docker system df --format "{{json .}}"` falhar (daemon parado/inacessível, erro de permissão no socket, timeout) ou retornar linhas JSON malformadas/incompletas, emitir um alvo explícito de revisão (`ID: "docker-daemon-status"`, `Path: "docker://daemon/status"`, `ReviewOnly: true`, `ReviewReason: domain.ReviewReasonAmbiguous`, `EligibleBytes: 0`) descrevendo a indisponibilidade/erro nas métricas, além de inventariar normalmente os artefatos em disco acessíveis (`Docker.raw`, `OrbStack`). Nunca retornar sucesso silencioso de zero bytes para um daemon com falha. — **Reversibility:** reversible.
- **D-15:** **Separação Explícita entre `Docker Desktop VM Disk` (Host) e `docker system df` (Lógico):** Quando o arquivo/diretório de VM do Docker Desktop no host existir (`~/Library/Containers/com.docker.docker/Data/vms/0/data/Docker.raw` ou `~/.docker/desktop/vms`), emiti-lo como um alvo dedicado de host (`Category: "docker-vm-disk"`, `ReviewOnly: true`, `ReviewReason: domain.ReviewReasonSharedStore`, `EligibleBytes: 0`) com medição sparse (`AllocatedBytes` vs `ApparentBytes`), separado das categorias lógicas `docker://system-df/*`. — **Reversibility:** reversible.
- **D-16:** **Degradação Fail-Closed em Locks e Erros Parciais de Permissão em Shared Stores:** Se a inspeção de um shared store (`NuGet`, `pnpm store`, `OrbStack VM/Data`, `Docker VM Disk`) encontrar arquivos de lock ativos (`.lock`), erro parcial de permissão (`EACCES`/`EPERM`) na árvore ou estado incerto, preservar o alvo no inventário com `ReviewOnly: true` (`ReviewReasonAmbiguous` quando o tamanho/estado for incerto ou `ProviderMetrics.Partial = true`), pular symlinks com `safety.IsSymlink` e nunca falhar de forma silenciosa nem permitir exclusão. — **Reversibility:** reversible.

### the agent's Discretion
- Nomenclatura exata das funções auxiliares em `internal/fs` para cálculo de uso em disco com blocos alocados e deduplicação de inodes por sistema operacional (`darwin`, `linux`, fallback `other`).
- Formatação textual exata dos resumos de métricas (`total`, `active`, `reclaimable`, `allocated`, `apparent`) nos campos `Name` dos alvos do Docker e OrbStack.
- Estrutura interna de arquivos dentro de `internal/cleaner/docker/` (ex: separar `docker.go` e `orbstack.go`).

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Especificação e Requisitos de Paridade
- `.planning/SPEC-v2.3-MOLE-PARITY.md` — Frente 1 (linhas 52-58), Frente 2 (linhas 73-75: Docker & OrbStack) e Tabela de Mapeamento (linhas 35, 46: `pnpm store`, `Docker`, `OrbStack`, `NuGet`).
- `.planning/REQUIREMENTS.md` — Requisitos `STORE-01`, `STORE-02`, `STORE-03`, `POL-01`, `POL-02`, `SCAN-02`, `VAL-01`, `VAL-02`.
- `.planning/ROADMAP.md` — Escopo da Fase 10 (linhas 104-115) e os 4 critérios de sucesso.
- `.planning/PROJECT.md` — Diretrizes de arquitetura, zero CGO e segurança inviolável.

### Decisões e Políticas Anteriores
- `.planning/phases/09-ai-electron-editors-active-versions/09-CONTEXT.md` — Padrão de Process Guard injetável, revalidação pré-exclusão no `Clean()` e fail-closed com `ReviewOnly`.
- `.planning/phases/08-node-headless-shell-cli-xcode/08-CONTEXT.md` — Divisão do cleaner `pnpm` (`D-04`) e uso de `cleaner.CommandExecutor` (`D-12`).
- `.planning/phases/07-reviewonly-coordinator-policy/07-CONTEXT.md` — Semântica de `ReviewOnly`, `ReviewReasonSharedStore`, `ReviewReasonAmbiguous` e bloqueio no Estágio 1 do `deleter.Coordinator`.
- `.planning/phases/06-scan-ttl-identity-disk-discovery/06-CONTEXT.md` — Desacoplamento `Scan()` vs TTL, `IsInstalled()` por presença em disco e propriedade canônica de caminhos.

### Código Existente Relevante
- `internal/cleaner/docker/docker.go` — Implementação atual do cleaner Docker (a ser refatorada para `CommandExecutor`, `docker system df` JSON, `ReviewOnly` e integração com OrbStack e Docker VM Disk).
- `internal/cleaner/dotnet/dotnet.go` — Implementação atual do cleaner .NET (a ser atualizada para marcar `nuget-packages` como `ReviewOnly` com `ReviewReasonSharedStore`).
- `internal/cleaner/pnpm/pnpm.go` — Implementação atual do cleaner pnpm (a ser atualizada para descobrir `pnpm store` por CLI + disco, marcá-lo como `ReviewOnly` e remover `pnpm store prune` automático).
- `internal/domain/types.go` — Struct `domain.Target` e constantes `ReviewReason*` (a receber `ProviderMetrics`).
- `internal/fs/size.go` — Cálculo atual de tamanho de diretórios (a ser expandido com medição de blocos alocados `st_blocks`, tamanho aparente `st_size` e deduplicação de inodes sem CGO).
- `internal/engine/register.go` — Registro central de cleaners (atualizar `docker.New()` para `docker.NewWithExecutor(exec)`).

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `cleaner.CommandExecutor` / `cleaner.DefaultExecutor`: Execução injetável de comandos (`docker`, `pnpm`, `pgrep`) sem interpolação de shell para testes herméticos.
- `domain.ReviewReasonSharedStore`, `domain.ReviewReasonActiveProcess`, `domain.ReviewReasonAmbiguous`: Constantes semânticas já definidas em `internal/domain/types.go`.
- `safety.Blacklist` e `safety.IsSymlink`: Validação contra caminhos protegidos e prevenção de travessia de links simbólicos.
- `internal/fs/statfs_darwin.go` / `statfs_linux.go` / `statfs_other.go`: Padrão existente de arquivos com build tags por SO sem CGO no pacote `internal/fs`.

### Established Patterns
- Descoberta por presença em disco sem exigir executáveis no `$PATH` (`SCAN-02`).
- Invariante `ReviewOnly = true` -> `EligibleBytes = 0` e `TTLEligible = false` (`POL-01`, `D-04` da Fase 7).
- Process Guard fail-closed via `pgrep -x` com revalidação imediata no `Clean()` antes de remover caches regeneráveis (`D-12` da Fase 8, `D-07` da Fase 9).
- Testes unitários herméticos com `t.TempDir()`, arquivos sentinela, symlinks, permissões e asserções de DryRun (`VAL-01`).

### Integration Points
- `internal/domain/types.go`: Adição do struct `ProviderMetrics` e campo `ProviderMetrics *ProviderMetrics` em `domain.Target`.
- `internal/fs/`: Adição de medição de diretórios/arquivos com suporte a sparse files (`AllocatedBytes` via `st_blocks * 512` vs `ApparentBytes` via `st_size`), deduplicação de hardlinks (`Dev, Ino`) e detecção de leitura parcial/locks.
- `internal/cleaner/docker/`: Refatoração de `docker.go` para `NewWithExecutor(exec)` + `orbstack.go` para cache regenerável e VM/data do OrbStack.
- `internal/cleaner/dotnet/dotnet.go`: Marcação de `nuget-packages` como `ReviewOnly = true` (`ReviewReasonSharedStore`) e guarda em `Clean()`.
- `internal/cleaner/pnpm/pnpm.go`: Descoberta híbrida (CLI + disco) do `pnpm store` como `ReviewOnly = true` (`ReviewReasonSharedStore`) e guarda em `Clean()`.
- `internal/engine/register.go`: Passagem de `exec` para `docker.NewWithExecutor(exec)`.

</code_context>

<specifics>
## Specific Ideas

- Paridade com o Mole (`tw93/mole`): exibir os grandes volumes compartilhados (`Docker`, `OrbStack data`, `NuGet packages`, `pnpm store`) como alvos de revisão (`⊙`) com métricas ricas, sem jamais apagá-los automaticamente.
- Honestidade contábil em imagens de VM e stores: diferenciar tamanho aparente (sparse file de 64 GB+) dos bytes efetivamente alocados no APFS/ext4 (`st_blocks * 512`), e deduplicar hardlinks no `pnpm store` para evitar inflar o espaço reportado.
- Visibilidade explícita de falhas de daemon: se o Docker estiver instalado mas o daemon estiver desligado ou retornar saída inválida, o inventário deve explicitar o estado incerto (`⊙`) em vez de omitir silenciosamente como 0 B.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 10-Docker, OrbStack & Shared Stores*  
*Context gathered: 2026-10-16*  

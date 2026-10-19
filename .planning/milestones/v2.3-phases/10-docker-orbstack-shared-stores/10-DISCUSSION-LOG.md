# Phase 10: Docker, OrbStack & Shared Stores - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-10-16  
**Phase:** 10-Docker, OrbStack & Shared Stores  
**Areas discussed:** Métricas do Docker e Granularidade ReviewOnly, OrbStack (Cache Regenerável vs VM/Data e Sparse Accounting), Shared Stores de Pacotes (NuGet e pnpm store), Estados de Erro do Daemon, Ausência de Provedor e Fail-Closed  

---

## Métricas do Docker e Granularidade ReviewOnly

| Option | Description | Selected |
|--------|-------------|:--------:|
| Emitir 1 alvo por categoria do `docker system df` | `Images`, `Containers`, `Local Volumes`, `Build Cache` com métricas detalhadas no Name/metadados, mantendo métricas lógicas do daemon separadas de arquivos de VM no host | ✓ |
| Emitir apenas 1 alvo consolidado `Docker storage` | Somando todas as categorias do `docker system df` | |
| Manter a listagem individual por imagem/volume dangling | Além do resumo do `docker system df` | |
| You decide | | |

**User's choice:** Emitir 1 alvo por categoria do `docker system df` (`Images`, `Containers`, `Local Volumes`, `Build Cache`) com métricas detalhadas no Name/metadados, mantendo métricas lógicas do daemon separadas de arquivos de VM no host  
**Notes:** Atende `STORE-01` preservando a distinção entre métricas lógicas do daemon e arquivos de VM no host.

| Option | Description | Selected |
|--------|-------------|:--------:|
| Todos os alvos de armazenamento do Docker são estritamente `ReviewOnly = true` | `Images`, `Containers`, `Local Volumes`, `Build Cache` e `VM disk` com `ReviewReasonSharedStore`, `EligibleBytes = 0`, e `Clean()` recusa mutação direta | ✓ |
| Apenas Images, Containers e Volumes são ReviewOnly | Mantendo Build Cache elegível para deleção automática | |
| You decide | | |

**User's choice:** Todos os alvos de armazenamento do Docker são estritamente `ReviewOnly = true` (`ReviewReasonSharedStore`, `EligibleBytes = 0`), e `Clean()` recusa mutação direta  
**Notes:** Cumpre rigorosamente `STORE-03` e o Critério de Sucesso 3 do ROADMAP.

| Option | Description | Selected |
|--------|-------------|:--------:|
| Adicionar struct opcional `ProviderMetrics` em `domain.Target` | Campos estruturados (`TotalCount`, `ActiveCount`, `SizeBytes`, `ReclaimableBytes`, `ApparentBytes`, `AllocatedBytes`) além do resumo legível no `Name` | ✓ |
| Codificar apenas como texto formatado no `Name` | Sem campos estruturados no JSON/MCP | |
| You decide | | |

**User's choice:** Adicionar um struct opcional de métricas de provedor em `domain.Target` (`ProviderMetrics`) além do resumo legível no `Name`  
**Notes:** Permite que CLI, TUI e saída JSON/MCP exponham métricas detalhadas sem parsing frágil de strings.

| Option | Description | Selected |
|--------|-------------|:--------:|
| Padronizar o Docker cleaner sobre `cleaner.CommandExecutor` | Executar `docker system df --format "{{json .}}"`, eliminando o acoplamento direto ao SDK pesado e garantindo mocks 100% determinísticos em testes | ✓ |
| Manter a interface dual (SDK Docker + fallback CLI) | Adaptando ambos para retornar as métricas de `system df` | |
| You decide | | |

**User's choice:** Padronizar o Docker cleaner sobre `cleaner.CommandExecutor` (`docker system df --format "{{json .}}"`)  
**Notes:** Alinha o cleaner Docker ao padrão hermético das Fases 8 e 9.

---

## OrbStack (Cache Regenerável vs VM/Data e Sparse Accounting)

| Option | Description | Selected |
|--------|-------------|:--------:|
| Integrar a descoberta do OrbStack dentro de `internal/cleaner/docker` | Ex: `orbstack.go` sob o ecossistema `docker`, alinhado com a Frente 2 da `SPEC-v2.3-MOLE-PARITY.md` | ✓ |
| Criar um cleaner e ecossistema separado `orbstack` | Em `internal/cleaner/orbstack` | |
| You decide | | |

**User's choice:** Integrar a descoberta do OrbStack dentro de `internal/cleaner/docker` sob o ecossistema `docker`  
**Notes:** Segue a arquitetura definida na especificação de paridade.

| Option | Description | Selected |
|--------|-------------|:--------:|
| Emitir alvos distintos (`OrbStack Cache` vs `OrbStack VM/Data`) | `OrbStack Cache` (`~/Library/Caches/dev.orbstack.OrbStack`) elegível para limpeza (`RiskLow` quando OrbStack fechado) e `OrbStack VM/Data` (`~/Library/Group Containers/*.dev.orbstack/data` e `~/.orbstack`) estritamente `ReviewOnly` (`ReviewReasonSharedStore`, `EligibleBytes = 0`) | ✓ |
| Marcar tanto o cache regenerável quanto VM/Data como ReviewOnly | Sem permitir limpeza do cache regenerável | |
| You decide | | |

**User's choice:** Emitir alvos distintos para `OrbStack Cache` (elegível quando fechado) e `OrbStack VM/Data` (estritamente `ReviewOnly`)  
**Notes:** Cumpre `STORE-02` e `STORE-03` separando caches regeneráveis de dados de VM/containers.

| Option | Description | Selected |
|--------|-------------|:--------:|
| Medir bytes alocados reais (`st_blocks * 512`) e tamanho aparente (`st_size`) | Usar bytes alocados em `SizeBytes`, `EligibleBytes = 0` e expor ambos rotulados em `ProviderMetrics` e no `Name` | ✓ |
| Usar apenas `devfs.CalcDirSize` padrão (`st_size`) | Sem diferenciar blocos alocados em disco | |
| You decide | | |

**User's choice:** Medir bytes alocados reais (`st_blocks * 512` via `syscall.Stat_t` sem CGO) e tamanho aparente (`st_size`), usando bytes alocados em `SizeBytes`, `EligibleBytes = 0` e expondo ambos rotulados  
**Notes:** Cumpre o Critério de Sucesso 2 evitando que arquivos esparsos de VM inflem o inventário ou sejam somados como recuperáveis.

| Option | Description | Selected |
|--------|-------------|:--------:|
| Aplicar Process Guard fail-closed via `CommandExecutor` (`pgrep -x OrbStack`) | Marca `OrbStack Cache` como `ReviewOnly = ReviewReasonActiveProcess` se ativo/incerto e revalida no `Clean()`, mantendo VM/data sempre `ReviewReasonSharedStore` | ✓ |
| Permitir limpeza de `~/Library/Caches/dev.orbstack.OrbStack` com OrbStack aberto | Sem verificação de processo | |
| You decide | | |

**User's choice:** Aplicar Process Guard fail-closed via `CommandExecutor` (`pgrep -x OrbStack`) para o cache regenerável do OrbStack  
**Notes:** Mantém consistência com o Process Guard de Xcode e Editores AI.

---

## Shared Stores de Pacotes (NuGet e pnpm store)

| Option | Description | Selected |
|--------|-------------|:--------:|
| Marcar exclusivamente `nuget-packages` (`~/.nuget/packages`) como `ReviewOnly = true` | `ReviewReasonSharedStore`, `EligibleBytes = 0`, `TTLEligible = false`, mantendo `nuget-http-cache`, `nuget-temp` e `bin/`/`obj/` locais elegíveis conforme TTL | ✓ |
| Marcar todos os diretórios globais do .NET como ReviewOnly | Incluindo `http-cache` e `temp` | |
| You decide | | |

**User's choice:** Marcar exclusivamente `nuget-packages` (`~/.nuget/packages`) como `ReviewOnly = true`  
**Notes:** Protege o store compartilhado de pacotes NuGet mantendo caches HTTP/temp e artefatos de build locais limpáveis.

| Option | Description | Selected |
|--------|-------------|:--------:|
| Consultar `pnpm store path` via `CommandExecutor` + fallback em disco | Verificar `~/Library/pnpm/store`, `~/.local/share/pnpm/store`, `~/.pnpm-store` e deduplicar por `EvalSymlinks`/`Clean` | ✓ |
| Depender exclusivamente de `pnpm store path` | | |
| Verificar apenas diretórios fixos em disco | | |
| You decide | | |

**User's choice:** Consultar `pnpm store path` via `CommandExecutor` e complementar/fazer fallback com os caminhos canônicos em disco  
**Notes:** Garante descoberta por presença em disco (`SCAN-02`) mesmo sem o binário `pnpm` no `$PATH`.

| Option | Description | Selected |
|--------|-------------|:--------:|
| Emitir `pnpm store` sempre com `ReviewOnly = true` e bloquear deleção/prune no `Clean()` | Mantendo apenas `pnpm metadata cache` (`~/Library/Caches/pnpm`, `~/.cache/pnpm`) elegível para limpeza | ✓ |
| Manter `pnpm store prune` executável automaticamente no `Clean()` | | |
| You decide | | |

**User's choice:** Emitir o diretório físico do `pnpm store` sempre com `ReviewOnly = true` (`ReviewReasonSharedStore`, `EligibleBytes = 0`) e bloquear deleção/prune do store no `Clean()`  
**Notes:** Cumpre `STORE-03` e impede mutação automática no store compartilhado do pnpm.

| Option | Description | Selected |
|--------|-------------|:--------:|
| Deduplicar inodes (`dev, ino` via `syscall.Stat_t`) e proteger `Clean()` defensivamente | Evitar somar múltiplos hardlinks do mesmo arquivo no store e recusar qualquer alvo `ReviewOnly` dentro de `Clean()` em `dotnet`, `pnpm` e `docker` | ✓ |
| Somar `st_size` de cada entrada sem deduplicar hardlinks | | |
| You decide | | |

**User's choice:** Deduplicar inodes (`dev, ino`) na medição de stores/VMs e proteger contra exclusão direta de qualquer caminho `ReviewOnly` dentro de `Clean()`  
**Notes:** Evita inflação de tamanho por hardlinks no pnpm store e adiciona defesa em profundidade nos cleaners.

---

## Estados de Erro do Daemon, Ausência de Provedor e Fail-Closed

| Option | Description | Selected |
|--------|-------------|:--------:|
| `IsInstalled()` retorna `true` por CLI no `PATH` OU diretórios em disco | Verifica `docker`/`orb` no `PATH` ou `~/.docker`, `~/Library/Containers/com.docker.docker`, `~/Library/Group Containers/*.dev.orbstack`, `~/Library/Caches/dev.orbstack.OrbStack`, `~/.orbstack` sem exigir `docker ping` ativo | ✓ |
| Continuar exigindo que o daemon Docker responda a `ping` | | |
| You decide | | |

**User's choice:** Retornar `true` se `docker` ou `orb` estiverem no `PATH` OU se diretórios em disco do Docker/OrbStack existirem, sem exigir `docker ping` ativo  
**Notes:** Garante `SCAN-02` e permite inventariar instalações com daemon desligado.

| Option | Description | Selected |
|--------|-------------|:--------:|
| Emitir alvo explícito de estado incerto (`docker://daemon/status`) | `ReviewOnly = true`, `ReviewReason = ReviewReasonAmbiguous`, `EligibleBytes = 0` com motivo do erro/malformed JSON, além de escanear artefatos em disco acessíveis | ✓ |
| Retornar lista vazia de alvos silenciosamente | Quando o daemon estiver offline ou o JSON for inválido | |
| Abortar todo o `Scan()` com erro fatal | Descartando dados em disco do OrbStack/Docker.raw | |
| You decide | | |

**User's choice:** Emitir um alvo explícito de estado incerto (`docker://daemon/status` com `ReviewOnly = true`, `ReviewReason = ReviewReasonAmbiguous`, `EligibleBytes = 0`) além de escanear os artefatos em disco acessíveis  
**Notes:** Cumpre o Critério de Sucesso 4 impedindo que erros de daemon pareçam sucesso de zero bytes.

| Option | Description | Selected |
|--------|-------------|:--------:|
| Reportar `Docker.raw` / VM disk no host como alvo separado (`docker-vm-disk`) | `ReviewOnly = true`, `ReviewReasonSharedStore`, separado dos alvos lógicos `docker://system-df/*` | ✓ |
| Ignorar arquivos de VM do Docker Desktop no host | | |
| You decide | | |

**User's choice:** Reportar `Docker.raw` / VM disk no host como alvo separado (`docker-vm-disk`, `ReviewOnly = true`, `ReviewReasonSharedStore`) distinto dos alvos lógicos `docker://system-df/*`  
**Notes:** Mantém métricas lógicas do daemon e métricas de arquivo de VM no host distintas (Critério de Sucesso 1).

| Option | Description | Selected |
|--------|-------------|:--------:|
| Preservar alvo como `ReviewOnly = true` em caso de lock, erro parcial ou incerteza | Usar `ReviewReasonAmbiguous` quando tamanho/estado for incerto ou `ProviderMetrics.Partial = true`, ignorar symlinks externos e validar com sentinelas em `t.TempDir()` | ✓ |
| Omitir o store do inventário se houver subdiretório sem permissão ou lock | | |
| You decide | | |

**User's choice:** Em caso de lock, erro parcial de permissão ou incerteza em shared store, manter o alvo como `ReviewOnly = true`, ignorar symlinks externos e validar com sentinelas em `t.TempDir()` que nenhum arquivo é deletado  
**Notes:** Garante rastreabilidade e segurança sob falhas parciais de leitura ou locks concorrentes.

---

## the agent's Discretion

- Nomenclatura exata das funções em `internal/fs` para medição de blocos alocados (`st_blocks * 512`) e deduplicação de inodes (`Dev, Ino`) com build tags por SO.
- Formatação textual dos resumos de métricas (`total`, `active`, `reclaimable`, `allocated`, `apparent`) nos nomes dos alvos.
- Divisão de arquivos internos dentro do pacote `internal/cleaner/docker/`.

## Deferred Ideas

None — discussion stayed within phase scope.

# Phase 13: App Leftovers & LaunchAgents - Context

**Gathered:** 2026-10-18  
**Status:** Ready for planning  

<domain>
## Phase Boundary

Garantir o inventário completo, seguro e baseado em evidências de LaunchAgents obsoletos (`LEFT-01`, `LEFT-02`) e resíduos de aplicativos desinstalados (`LEFT-03`), classificando todos os alvos estritamente como itens de revisão (`ReviewOnly: true`) com proibição inegociável de remoção física ou automática (`LEFT-04`):
1. `Parsing de LaunchAgents` (`LEFT-01`): leitura e decodificação transparente de arquivos `.plist` em XML e Binary Plist (`bplist00`) através de biblioteca consolidada em Go puro (`howett.net/plist`), mantendo compilação 100% zero-CGO e sem dependências do sistema.
2. `Critérios de Stale vs Ambíguo` (`LEFT-02`): classificação estrita de "Stale" (`stale_executable_missing`) apenas quando `Program` ou `ProgramArguments[0]` for um caminho absoluto direto inexistente em disco (e não-interpretador). Invocação de interpretadores (`/bin/sh`, `/bin/bash`, `python`, `osascript`), comandos relativos ou dependentes do `$PATH`, ou plists com sintaxe inválida/unreadable permanecem como itens não-resolvidos (`ReviewReasonAmbiguous`), evitando rigorosamente qualquer falso positivo.
3. `Detecção de App Leftovers` (`LEFT-03`): varredura determinística de pacotes `.app` em `/Applications`, `/System/Applications` e `~/Applications` para montar um índice bidirecional (`CFBundleIdentifier` e nomes do app). Cruzamento com `~/Library/Application Support`, `~/Library/Caches` e `~/Library/Containers`. Exclusão estrita de itens do sistema Apple (`com.apple.*`), diretórios geridos por outros cleaners do DevClean (`docker`, `homebrew`, `pnpm`, editores, navegadores) e utilitários de CLI comuns sem `.app`.
4. `Apresentação e Granularidade`: LaunchAgents reportados individualmente por plist (`⊙ Stale LaunchAgent: com.exemplo.service.plist`) exibindo binário ausente e path; App Leftovers agrupados por aplicativo residual (`⊙ App Leftover: OldApp`) somando os diretórios correspondentes; medição real em disco via `devfs.CalcDirUsage` sem cortes arbitrários de tamanho.
5. `Segurança, Arquitetura e Zero Mutação` (`LEFT-04`): novo pacote dedicado `internal/cleaner/leftovers`, com construtores permitindo injeção de diretórios raiz para testes herméticos 100% em `t.TempDir()`. Método `Clean()` rejeita formalmente qualquer deleção em conformidade com LEFT-04. Symlink Guard e verificação de propriedade (`st_uid == current_uid`) bloqueiam travessias inseguras.

</domain>

<decisions>
## Implementation Decisions

### Parsing de Plist & Critérios de Stale LaunchAgents (LEFT-01, LEFT-02)
- **D-01:** **Parsing Unificado com howett.net/plist em Go Puro:** Utilizar a biblioteca `howett.net/plist` pura em Go para decodificar transparentemente tanto XML quanto binary plists (`bplist00`) sem CGO, garantindo portabilidade entre plataformas e execução hermética em testes. — **Reversibility:** costly — define a dependência e motor de decodificação de plists do projeto.
- **D-02:** **Critério Estrito de Stale para LaunchAgents:** Apenas um caminho absoluto direto (e não-interpretador) inexistente em disco é classificado como "Stale" (`stale_executable_missing`). Wrappers/interpretadores (`/bin/sh`, `/bin/bash`, `python`, `osascript`, `/usr/bin/env`), comandos resolvidos via `$PATH` ou caminhos relativos são marcados como "Unresolved / Ambíguo" (`launch_config_ambiguous`), sem emissão de falso positivo. — **Reversibility:** costly — define a regra de negócio central de classificação de login items.
- **D-03:** **Escopo Restrito a LaunchAgents de Usuário e Sistema:** Escanear estritamente `~/Library/LaunchAgents` (usuário) e `/Library/LaunchAgents` (sistema); excluir incondicionalmente `/System/Library/...` (protegido por SIP/Apple) e LaunchDaemons. Falhas de permissão em `/Library/LaunchAgents` degradam graciosamente com `ProviderMetrics.Partial = true` e `StatusNote: "permission denied (TCC / root required)"`, com `ReviewOnly: true` (`ReviewReasonSystemProtected`). — **Reversibility:** costly — define o perímetro de diretórios de LaunchAgents.
- **D-04:** **Fail-Closed em Plists Ilegíveis, Corrompidos ou Falhas de Ambiente:** Arquivos plist malformados, ilegíveis por permissão ou onde o launchctl estiver inacessível recebem classificação de revisão não-resolvida (`ReviewOnly: true`, `ReviewReason: domain.ReviewReasonAmbiguous`), mantendo a nota do erro em `Target.Reason` com tamanho 0. — **Reversibility:** reversible — tratamento padronizado de erro em parsing.

### Detecção de App Leftovers & Estratégia de Matching (LEFT-03)
- **D-05:** **Varredura Determinística Direta de Aplicativos Instalados:** Realizar busca determinística de bundles `.app` via `filepath.WalkDir` com profundidade limitada a 2–3 níveis em `/Applications`, `/System/Applications` e `~/Applications`, lendo `CFBundleIdentifier` e `CFBundleExecutable`/nome do app a partir de `Info.plist`, sem CGO ou dependência do daemon do Spotlight (`mdfind`). — **Reversibility:** costly — define o mecanismo de descoberta de aplicativos do sistema.
- **D-06:** **Índice Bidirecional de Aplicativos (Bundle ID + Nome):** Mapear tanto o identificador reverso (`CFBundleIdentifier`, ex: `com.tinyspeck.slackmacgap`) quanto os nomes amigáveis do app/executável (`Slack`). Uma pasta em Application Support, Caches ou Containers é candidata a leftover apenas se nem seu ID nem seu nome coincidirem com qualquer app instalado. — **Reversibility:** costly — define a estratégia de correlação contra falsos positivos.
- **D-07:** **Proteção Multinível contra Falsos Positivos em CLI e Outros Cleaners:** Ignorar incondicionalmente diretórios pertencentes a componentes do sistema Apple (`com.apple.*`), pastas geridas por cleaners dedicados do DevClean (`docker`, `homebrew`, `pnpm`, `xcode`, editores AI/Electron, navegadores, `uv`, `mise`, `python`) e ferramentas comuns de CLI sem `.app` (ex: `git`, `gnupg`, `ssh`). — **Reversibility:** costly — estabelece o contrato de exclusão entre ecossistemas e ferramentas de terminal.
- **D-08:** **Escopo Estrito de Resíduos: Application Support, Caches e Containers:** Inspecionar exclusivamente as 3 localizações normativas de `LEFT-03`: `~/Library/Application Support`, `~/Library/Caches` e `~/Library/Containers`, onde se concentra a quase totalidade do volume em bytes de resíduos de apps. — **Reversibility:** costly — define o escopo físico da busca por leftovers.

### Apresentação do Inventário & Granularidade na CLI/TUI (LEFT-01, LEFT-03)
- **D-09:** **Apresentação Individual para LaunchAgents na CLI/TUI:** Emitir 1 alvo individual por plist stale ou ambíguo (ex: `⊙ Stale LaunchAgent: com.exemplo.service.plist`), exibindo claramente nas notas o executável faltante e o caminho do plist para inspeção direta. — **Reversibility:** reversible — formatação do alvo na CLI/TUI.
- **D-10:** **Agrupamento Consolidado por Aplicativo Residual para App Leftovers:** Emitir 1 alvo consolidado por aplicativo residual detectado (ex: `⊙ App Leftover: OldApp`), somando os diretórios correspondentes em Application Support, Caches e Containers sob o mesmo app, discriminando as pastas nos detalhes/notas. — **Reversibility:** reversible — agrupamento de interface do usuário.
- **D-11:** **Medição Real em Disco sem Corte Mínimo de Tamanho:** Calcular o uso real em disco via `devfs.CalcDirUsage` para todas as pastas de app leftover encontradas, e o tamanho exato do arquivo `.plist` para LaunchAgents, reportando todos os candidatos detectados sem corte arbitrário de tamanho. — **Reversibility:** reversible — precisão de métricas do scan.
- **D-12:** **Constantes Semânticas Dedicadas de ReviewReason em domain:** Adicionar em `internal/domain/types.go` as constantes semânticas `ReviewReasonStaleLaunchAgent` e `ReviewReasonAppLeftover`, mantendo `ReviewReasonAmbiguous` para plists não-resolvidos, com descrições explicativas detalhadas em `Target.Reason`. — **Reversibility:** costly — evolução dos tipos centrais do domínio.

### Garantias de Segurança, Não-Exclusão & Arquitetura (LEFT-04, VAL-01, VAL-02)
- **D-13:** **Defesa em Profundidade Incondicional no Clean():** O método `Clean()` do pacote `internal/cleaner/leftovers` recusa formalmente qualquer deleção física, retornando imediatamente 0 bytes limpos e log de recusa de política, garantindo conformidade absoluta com LEFT-04 mesmo sob chamadas diretas ou testes. — **Reversibility:** one-way — compromisso de segurança inviolável de não-exclusão no v2.3.
- **D-14:** **Pacote Dedicado internal/cleaner/leftovers e Registro no Motor:** Implementar a interface `domain.Cleaner` em um pacote dedicado `internal/cleaner/leftovers`, registrado no motor (`internal/engine/register.go`) sob o ecossistema `leftovers` com bloco correspondente em `internal/config/config.go`. — **Reversibility:** costly — modularização e arquitetura de pacotes do motor.
- **D-15:** **Arquitetura Injetável de Diretórios Raiz e Hermetismo Zero-CGO:** Construtores aceitam injeção de diretórios raiz (`ApplicationsDir`, `LaunchAgentsDir`, `AppSupportDir`, etc.), permitindo testes herméticos 100% com `t.TempDir()` tanto no Linux quanto no Darwin, com no-op em tempo de execução real em sistemas não-Darwin e compilação pura sem CGO. — **Reversibility:** costly — arquitetura de testabilidade multiplataforma.
- **D-16:** **Symlink Guard e Verificação de Propriedade (Ownership):** Rejeitar travessia de symlinks que apontem para fora do diretório escaneado ou pertençam a outros usuários (`st_uid != current_uid`), classificando alvos ambíguos como `ReviewOnly: true` (`ReviewReasonAmbiguous`). — **Reversibility:** costly — salvaguarda estrutural de filesystem.

### the agent's Discretion
- Nomenclatura exata de structs auxiliares internas de mapeamento de bundle e plist em `internal/cleaner/leftovers/`.
- Nomes dos arquivos de teste e geradores de fixtures mockados em `internal/cleaner/leftovers/`.
- Limite exato de profundidade da varredura de `.app` (padrão adotado: 2 a 3 níveis).

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Paridade e Requisitos
- `.planning/ROADMAP.md` §Phase 13 — Metas, dependências e critérios de sucesso da Fase 13 (LEFT-01 a LEFT-04).
- `.planning/REQUIREMENTS.md` §App leftovers and LaunchAgents — Requisitos normativos LEFT-01, LEFT-02, LEFT-03, LEFT-04, VAL-01 e VAL-02.
- `.planning/SPEC-v2.3-MOLE-PARITY.md` §Frente 5: Detector de App Leftovers e Stale LaunchAgents (`internal/cleaner/leftovers`) — Especificação técnica do scanner de LaunchAgents e resíduos de aplicativos.

### Padrões de Implementação e Arquitetura no Codebase
- `internal/cleaner/macos/ownership_darwin.go` — Verificação de propriedade (`st_uid == current_uid`) sem CGO.
- `internal/cleaner/macos/whitelist.go` — Whitelist imutável para dados sensíveis e componentes Apple.
- `internal/safety/blacklist.go` — Blacklist imutável do sistema e proteção contra travessia de symlinks (`IsCoreBlacklisted`).
- `internal/domain/types.go` — Definições centrais de `Target`, `ProviderMetrics` e constantes de `ReviewReason`.
- `internal/deleter/coordinator.go` — Política central do coordenador de mutações assegurando rejeição a alvos `ReviewOnly`.
- `internal/engine/register.go` — Ponto de injeção e registro de ecossistemas no motor do DevClean.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `cleaner.CommandExecutor` / `cleaner.MockCommandExecutor` (`internal/cleaner/executor.go`): Abstração para chamadas de sistema, garantindo testes determinísticos.
- `devfs.CalcDirUsage` (`internal/fs`): Medição exata de tamanho e blocos ocupados em disco.
- `safety.IsCoreBlacklisted` / `safety.IsSymlink` (`internal/safety`): Validação de caminhos críticos e guarda de links simbólicos.
- `deleter.Coordinator` (`internal/deleter/coordinator.go`): Validação central de segurança contra mutações em alvos `ReviewOnly`.

### Established Patterns
- **Desacoplamento Scan vs. TTL:** `Scan()` reporta 100% dos bytes em disco; elegibilidade respeita rigorosamente a política `ReviewOnly`.
- **Fail-Closed Policy:** Qualquer incerteza em parsing, caminhos de script, interpretadores ou permissões marca o alvo com `ReviewOnly: true`, `EligibleBytes: 0` e `ReviewReasonAmbiguous`.
- **Fixtures com Sentinelas em `t.TempDir()`:** Todos os testes unitários constroem árvores de diretórios com sentinelas simulando plists (XML e binário) e pastas de leftovers, verificando integridade total após `Clean()`.
- **Zero-CGO Multiplataforma:** Compilação 100% pura sem CGO para Darwin e Linux com compilação cruzada.

### Integration Points
- `internal/cleaner/leftovers/`: Novo pacote implementando `domain.Cleaner` com submódulos para LaunchAgents, App Leftovers, matching de bundles e parser de plist.
- `internal/engine/register.go`: Registro do ecossistema `leftovers` em `RegisterDefaultCleanersWithExecutor`.
- `internal/config/config.go`: Adição de `Leftovers EcosystemConfig` em `EcosystemsConfig`.
- `internal/domain/types.go`: Adição das constantes `ReviewReasonStaleLaunchAgent` e `ReviewReasonAppLeftover`.

</code_context>

<specifics>
## Specific Ideas
- Alinhamento total com as categorias de paridade do Mole: `App leftovers` e `Stale login items` (`⊙`).
- Exibição transparente do binário faltante na descrição do alvo de LaunchAgent obsoleto para que o usuário entenda imediatamente por que o item foi sinalizado.

</specifics>

<deferred>
## Deferred Ideas
None — discussion stayed within phase scope.

</deferred>

---

*Phase: 13-App Leftovers & LaunchAgents*  
*Context gathered: 2026-10-18*

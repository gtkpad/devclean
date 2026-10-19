# Phase 12: Browsers, Session Exclusions & ScriptCache Gate - Context

**Gathered:** 2026-10-16  
**Status:** Ready for planning  

<domain>
## Phase Boundary

Garantir o inventário completo e a limpeza segura de caches regeneráveis de 6 navegadores web (Google Chrome, Brave, Arc, Microsoft Edge, Mozilla Firefox e Apple Safari) sem jamais colocar em risco sessões, histórico ou dados pessoais, implementando guarda de aceitação sobre o ScriptCache e isolamento determinístico por Process Guard:
1. `Descoberta e Suporte Multi-Browser` (`BROW-01`): Detectar Chrome, Brave, Arc, Edge, Firefox e Safari por presença em disco através de resolução canônica multiplataforma no macOS (`~/Library/Application Support/...` e `~/Library/Caches/...`) e Linux (`~/.config/...` e `~/.cache/...`), mantendo Arc e Safari restritos a Darwin.
2. `Granularidade e Agrupamento Seguro` (`BROW-02`): Apresentar alvos consolidados na CLI e TUI agrupados por navegador (ex: `Chrome Caches`, `Firefox Caches`, `Arc Caches`), consolidando perfis múltiplos (`Default`, `Profile *`, perfis `*.default*` do Firefox) e agregando caches de rede (`Cache_Data`), GPU/shaders (`GPUCache`, `DawnCache`), Code Cache e Service Worker `CacheStorage` em uma única visão coesa com métricas detalhadas.
3. `Blindagem Inviolável de Sessão` (`BROW-03`): Dupla camada de proteção combinando Positive Allowlist estrita de subpastas regeneráveis com Invariant Assertion Guard negativo que rejeita incondicionalmente qualquer arquivo ou diretório de sessão (`Cookies`, `History`, `Local Storage`, `IndexedDB`, `Login Data`, `Sessions`, `places.sqlite`, `key4.db`, `logins.json`, `sessionstore*`). No Safari, varrer exclusivamente subpastas permitidas em `~/Library/Caches/com.apple.Safari` (ex: `WebKitCache`, `Cache.db`), mantendo `~/Library/Safari` inegociavelmente fora do alcance do cleaner.
4. `Gating do ScriptCache` (`BROW-04`): Inventariar `Service Worker/ScriptCache` em alvos dedicados de revisão por navegador Chromium (`Chrome ScriptCache`, `Brave ScriptCache`, `Arc ScriptCache`, `Edge ScriptCache`) com `ReviewOnly: true`, `ReviewReason: "scriptcache_requires_evidence"`, `EligibleBytes: 0` e `TTLEligible: false`, bloqueando mutação automática no v2.3 até que a evidência de aceitação na Fase 14 autorize elegibilidade. Firefox e Safari não emitem alvo de ScriptCache.
5. `Process Guard Determinístico e Revalidação Pré-Clean`: Mapear executáveis principais e helpers ativos via `cleaner.CommandExecutor` (`pgrep -x`), isolando o bloqueio (`ReviewOnly: true`, `ReviewReasonActiveProcess`, `EligibleBytes = 0`) estritamente ao navegador ativo. Revalidar atomicamente o estado dos processos imediatamente pré-Clean no método `Clean()`, abortando fail-closed com aviso explícito caso o navegador tenha sido aberto.
6. `Defesa em Profundidade e Degradação Graciosa`: O método `Clean()` interno do cleaner de navegadores impõe guarda direta contra mutação em alvos `ReviewOnly` ou `ScriptCache`. Falhas de permissão ou TCC degradam graciosamente via `ProviderMetrics.Partial = true`, `StatusNote: "permission denied (TCC / Full Disk Access required)"`, `ReviewOnly: true` (`ReviewReasonSystemProtected`) e estritamente zero uso de sudo/elevação.
7. `Testes Herméticos e Zero CGO` (`VAL-01`): Testes unitários com `t.TempDir()`, sentinelas duplas (arquivos descartáveis vs sentinelas de sessão protegidas) provando que `Clean()` remove apenas descartáveis e preserva 100% das sessões, com `MockCommandExecutor` e compilação pura sem CGO.

</domain>

<decisions>
## Implementation Decisions

### Granularidade e Apresentação dos Alvos (BROW-01, BROW-02)
- **D-01:** **Agrupamento Consolidado por Navegador:** Apresentar 1 alvo consolidado principal por navegador instalado (ex: `Chrome Caches`, `Firefox Caches`, `Arc Caches`, `Edge Caches`, `Brave Caches`, `Safari Caches`), mantendo a CLI e TUI limpas e sem poluição de dezenas de linhas avulsas, alinhado ao padrão da Fase 09 para editores. — **Reversibility:** reversible — define o agrupamento e rotulação dos alvos na CLI/TUI.
- **D-02:** **Inclusão do CacheStorage no Alvo Principal do Navegador:** Incorporar o Service Worker `CacheStorage` dentro do alvo geral do navegador (`Chrome Caches`), discriminando os subvolumes de rede, GPU e CacheStorage nas métricas e detalhes do alvo sem criar alvos concorrentes elegíveis. — **Reversibility:** reversible.
- **D-03:** **Varredura Dinâmica de Todos os Perfis:** O scanner varre dinamicamente todos os diretórios de perfis existentes no disco (`Default`, `Profile *`, `System Profile` nos navegadores Chromium e todos os perfis `*.default*` no Firefox), acumulando seus bytes totais e elegíveis no alvo do respectivo navegador para garantir que perfis secundários não fiquem órfãos de limpeza. — **Reversibility:** costly — define a lógica de traversal de perfis em disco.
- **D-04:** **Abstração Unificada Multi-SO com Restrição de Plataforma:** Mapear caminhos canônicos no macOS (`~/Library/Application Support/...` e `~/Library/Caches/...`) e no Linux (`~/.config/...` e `~/.cache/...` / `~/.mozilla/...`), ativando Safari e Arc exclusivamente em Darwin e assegurando que todos os navegadores possuam testes herméticos em ambos os sistemas operacionais com zero CGO. — **Reversibility:** costly — define os caminhos e contratos de compilação multiplataforma.

### Gating e Política do ScriptCache (BROW-04)
- **D-05:** **ScriptCache Incondicionalmente ReviewOnly no v2.3:** O inventário de `Service Worker/ScriptCache` é marcado estritamente como `ReviewOnly: true`, `ReviewReason: "scriptcache_requires_evidence"`, `EligibleBytes: 0` e `TTLEligible: false`, impedindo qualquer mutação ou expurgo automático durante o milestone v2.3 até a validação integrada com evidência de aceitação na Fase 14. — **Reversibility:** costly — define o contrato de segurança do ScriptCache em conformidade com BROW-04.
- **D-06:** **Alvo Isolado de Revisão por Navegador Chromium:** Emitir alvos dedicados de revisão para o ScriptCache (ex: `Chrome ScriptCache`, `Brave ScriptCache`, `Arc ScriptCache`, `Edge ScriptCache`), permitindo que os alvos principais (`Chrome Caches`) permaneçam elegíveis quando o navegador estiver fechado sem contaminação pelo status de revisão. — **Reversibility:** reversible.
- **D-07:** **Guarda Defensiva Direta no Clean():** Além do bloqueio central no `deleter.Coordinator`, o próprio método `Clean()` do pacote `internal/cleaner/browsers` recusa e ignora imediatamente qualquer alvo marcado com `ReviewOnly == true` ou pertencente à categoria `scriptcache`, garantindo zero mutação mesmo sob chamadas diretas em testes ou refatorações. — **Reversibility:** reversible.
- **D-08:** **Emissão Estrita por Motor de Renderização:** Emitir alvos de `ScriptCache` exclusivamente para os 4 navegadores baseados em Chromium (Chrome, Brave, Arc, Edge); Firefox (Gecko) e Safari (WebKit) não emitem alvos de ScriptCache, pois suas estruturas internas de service workers permanecem protegidas pelas exclusões de sessão. — **Reversibility:** reversible.

### Process Guard e Revalidação Pré-Clean (BROW-01, BROW-04)
- **D-09:** **Bloqueio de Processo Isolado por Navegador:** A verificação de processos em execução (`pgrep -x`) é avaliada individualmente por navegador. Se o Chrome estiver aberto, apenas os alvos do Chrome recebem `ReviewOnly: true`, `ReviewReason: domain.ReviewReasonActiveProcess` e `EligibleBytes: 0`; navegadores fechados permanecem elegíveis para limpeza. — **Reversibility:** reversible.
- **D-10:** **Revalidação Atômica Pré-Clean Fail-Closed:** O método `Clean()` revalida obrigatoriamente o status dos processos do navegador imediatamente antes de executar qualquer deleção física. Se o navegador tiver sido iniciado entre o scan e a limpeza, a mutação daquele alvo é abortada atomicamente com registro explícito em log/retorno. — **Reversibility:** costly — garante integridade atômica contra condições de corrida do usuário.
- **D-11:** **Monitoramento de Processo Principal e Helpers Ativos:** O mapeamento de processos monitora tanto o binário principal (`Google Chrome`, `Brave Browser`, `Arc`, `Microsoft Edge`, `firefox`, `Safari`) quanto seus helpers ativos conhecidos (ex: `Google Chrome Helper`, `firefox-bin`), prevenindo limpeza de caches enquanto processos em background retiverem locks em disco. — **Reversibility:** reversible.
- **D-12:** **Fail-Closed em Incerteza do pgrep e Testes com MockCommandExecutor:** Qualquer falha inesperada na execução do `pgrep` (comando ausente, erro de permissão ou timeout) classifica o status como incerto (`uncertain = true`), forçando `ReviewOnly: true` e `EligibleBytes: 0`. A suite de testes utiliza `cleaner.MockCommandExecutor` para cobertura 100% determinística sem dependência de comandos do SO. — **Reversibility:** costly — padroniza o ciclo de vida hermético de processos no cleaner.

### Tratamento do Safari e Isolamento de Sessão (BROW-02, BROW-03, VAL-01)
- **D-13:** **Positive Allowlist Estrita para Safari no macOS:** O scanner do Safari varre exclusivamente as subpastas e caches regeneráveis conhecidos em `~/Library/Caches/com.apple.Safari` (`WebKitCache`, `Cache.db`), mantendo `~/Library/Safari` (histórico, favoritos, sessões) inegociavelmente fora do alcance do código. — **Reversibility:** one-way — compromisso inegociável de proteção da biblioteca privada do Safari.
- **D-14:** **Degradação Graciosa em Negação de Permissão (TCC / Full Disk Access):** Em caso de erro de acesso (`EACCES`, `EPERM` ou proteção TCC) em caches do Safari ou de outros navegadores, preencher `ProviderMetrics.Partial = true`, `StatusNote: "permission denied (TCC / Full Disk Access required)"`, marcar `ReviewOnly = true` (`ReviewReasonSystemProtected`) e operar com zero chamadas a `sudo` ou elevação de privilégio. — **Reversibility:** costly — padroniza a gestão de permissões sem elevação.
- **D-15:** **Dupla Camada de Blindagem contra Perda de Sessão (Positive + Invariant Guard):** Combinação estrutural de apontamento positivo apenas para folhas regeneráveis permitidas com uma lista de asserções negativas invioláveis (`Cookies*`, `History*`, `Local Storage*`, `IndexedDB*`, `Login Data*`, `Sessions*`, `places.sqlite*`, `key4.db*`, `logins.json*`, `sessionstore*`). Qualquer caminho que coincida com essas sentinelas de sessão é imediatamente rejeitado antes de qualquer medição ou mutação. — **Reversibility:** one-way — garantia de segurança inviolável da política BROW-03.
- **D-16:** **Fixtures Herméticos com Sentinelas Duplas em t.TempDir() (VAL-01):** Os testes unitários constroem árvores de diretórios completas para os 6 navegadores contendo arquivos descartáveis (`Cache_Data/f_000001`, `GPUCache/data_0`, `CacheStorage/...`) e sentinelas de sessão protegidas (`Cookies`, `History`, `Login Data`, `Local Storage/000003.log`, `IndexedDB/...`, `places.sqlite`, `LastSession.plist`), testando `Scan()`, `DryRun` e `Clean()` e verificando que apenas os descartáveis são removidos enquanto 100% dos dados de sessão permanecem intocados. — **Reversibility:** costly — define o contrato de testes unitários para aceitação da fase.

### the agent's Discretion
- Nomenclatura exata das constantes e funções auxiliares em `internal/cleaner/browsers/`.
- Estrutura de arquivos do novo pacote (ex: `browsers.go`, `chromium.go`, `firefox.go`, `safari.go`, `process.go`).
- Mapeamento exato de executáveis auxiliares e helpers por plataforma no `process.go`.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Paridade e Requisitos
- `.planning/ROADMAP.md` §Phase 12 — Metas, dependências e critérios de sucesso da Fase 12 (BROW-01 a BROW-04).
- `.planning/REQUIREMENTS.md` §Browsers — Requisitos normativos BROW-01, BROW-02, BROW-03, BROW-04, VAL-01 e VAL-02.
- `.planning/SPEC-v2.3-MOLE-PARITY.md` §Frente 4: Novo Cleaner de Navegadores (`internal/cleaner/browsers`) — Especificação técnica dos 6 navegadores, escopo estritamente seguro e paridade com o Mole.

### Padrões de Implementação e Arquitetura no Codebase
- `internal/cleaner/ai/editors.go` — Padrão de varredura de subpastas Chromium/Electron (`GPUCache`, `Code Cache`, `DawnCache`, `CacheStorage`, `ScriptCache`).
- `internal/cleaner/macos/process.go` — Padrão de Process Guard determinístico fail-closed com `cleaner.CommandExecutor` (`pgrep -x`).
- `internal/safety/blacklist.go` — Blacklist imutável do sistema e proteção contra travessia de symlinks (`IsCoreBlacklisted`).
- `internal/cleaner/macos/whitelist.go` — Whitelist imutável para dados sensíveis da Apple e proteção do Safari.
- `internal/domain/types.go` — Definições de `Target`, `ProviderMetrics`, `ReviewReasonActiveProcess`, `ReviewReasonSystemProtected`, `ReviewReasonSharedStore`.
- `internal/engine/register.go` — Ponto de injeção e registro do novo ecossistema `browsers` no motor do DevClean.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `cleaner.CommandExecutor` / `cleaner.MockCommandExecutor` (`internal/cleaner/executor.go`): Interface para execução mockável de `pgrep` garantindo testes 100% determinísticos e herméticos sem CGO.
- `ai.CheckProcessStatus` / `macos.CheckProcessStatus`: Lógica de verificação de processos com fail-closed em caso de erro.
- `devfs.CalcDirSize` / `devfs.CalcDirUsage` (`internal/fs`): Cálculo seguro de tamanho de diretórios.
- `safety.IsCoreBlacklisted` / `safety.IsSymlink` (`internal/safety`): Validação prévia contra deleção acidental de diretórios vitais ou desvios via links simbólicos.
- `deleter.Coordinator` (`internal/deleter/coordinator.go`): Pipeline central de deleção que valida `ReviewOnly == false` e `DryRun`.

### Established Patterns
- **Desacoplamento Scan vs TTL:** `Scan()` reporta 100% dos bytes em disco; `EligibleBytes` e `TTLEligible` respeitam o TTL (`MaxAgeDays == 0` ou idade > cutoff).
- **Fail-Closed Policy:** Qualquer ambiguidade de permissão, processo ativo ou link suspeito marca o alvo com `ReviewOnly: true`, `EligibleBytes: 0` e `ReviewReason` semântico.
- **Fixtures com Sentinelas em `t.TempDir()`:** Todos os testes unitários constroem árvores temporárias com arquivos sentinela protegidos e verificam integridade antes e após `Clean()`.
- **Compilação Pura Zero-CGO:** Código compila com `CGO_ENABLED=0` para Darwin e Linux, usando tags de build ou abstrações limpas.

### Integration Points
- `internal/cleaner/browsers/`: Novo pacote implementando a interface `domain.Cleaner` com submódulos para os navegadores, processos e regras de sessão.
- `internal/engine/register.go`: Registro do novo ecossistema `browsers` em `RegisterDefaultCleanersWithExecutor`.
- `internal/config/config.go`: Adição de `Browsers EcosystemConfig` e `Hooks` correspondentes em `EcosystemsConfig`.

</code_context>

<specifics>
## Specific Ideas
- Alinhamento total com as categorias de paridade do Mole: Chrome Service Worker (CacheStorage), GPUCache, Code Cache e caches de rede.
- Preservação inegociável de Cookies, History, Local Storage, IndexedDB, Login Data e Sessions em todos os 6 navegadores.
- ScriptCache explicitamente bloqueado como ReviewOnly até que a aceitação integrada da Fase 14 traga evidências de restauração e segurança.

</specifics>

<deferred>
## Deferred Ideas
None — discussion stayed within phase scope

</deferred>

---

*Phase: 12-Browsers, Session Exclusions & ScriptCache Gate*  
*Context gathered: 2026-10-16*

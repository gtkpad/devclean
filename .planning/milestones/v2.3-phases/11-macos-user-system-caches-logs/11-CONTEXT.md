# Phase 11: macOS User/System Caches & Logs - Context

**Gathered:** 2026-10-16  
**Status:** Ready for planning  

<domain>
## Phase Boundary

Garantir o inventário completo e a limpeza segura de caches e logs de usuário e sistema do macOS, Lixeira (`~/.Trash`), folhas específicas de Application Support e daemons da Apple, com garantias estritas de segurança, ausência de privilégios de root, proteção contra travessia de symlinks, verificação de propriedade (ownership) e Process Guard fail-closed mantendo compilação estática zero-CGO:
1. `User Essentials` (`MAC-01`): cobre `~/Library/Caches/*`, `~/Library/Logs/*` e `~/.Trash/*`. A Lixeira é reportada no scan sempre como `ReviewOnly` (`⊙`), tornando-se elegível para remoção apenas com opção explícita (`--trash` ou config `safety.trash_enabled`), purgada diretamente sem re-enviar para a quarentena interna do DevClean. A varredura de lixeira foca estritamente em `$HOME/.Trash`, ignorando volumes externos.
2. `Granularidade e Deduplicação` (`MAC-01`, `MAC-03`): para evitar dupla contagem, o cleaner de macOS aplica uma lista de exclusão para diretórios de caches atendidos por cleaners dedicados (`uv`, `pnpm`, `mise`, `ms-playwright`, `cypress`, `homebrew`, `cocoapods`, `xcode`, `orbstack`, editores AI/Electron). Alvos de `~/Library/Caches` e `~/Library/Logs` são consolidados por categorias funcionais (`User App Caches` e `User App Logs`) com destaques para apps de grande volume. `Application Support` restringe-se estritamente às subpastas regeneráveis permitidas (`Cache`, `Caches`, `logs`, `Logs`, `Crashpad/completed`), com alvo dedicado para `Spotify PersistentCache`.
3. `Group Containers & Daemons Apple` (`MAC-02`): aplica positive allowlist estrito para Group Containers (`Library/Caches` e `Library/Logs`), excluindo explicitamente `*.dev.orbstack` (já tratado em `internal/cleaner/docker`), e positive allowlist canônica para os 8 daemons especificados (Siri suggestions, Wallpaper agent, `com.apple.mediaanalysisd`, Geod, `com.apple.helpd`, Apple Media Services, Parsecd e Handoff).
4. `Logs de Sistema, DiagnosticReports e Code-Sign Clones` (`MAC-04`): divide relatórios em `User Diagnostic Reports` (elegível) e `System Diagnostic Reports` (ReviewOnly). Falhas de permissão ou negação de TCC em `/private/var/log` ou `/Library/Logs/DiagnosticReports` degradam graciosamente via `ProviderMetrics.Partial = true` e `StatusNote: "permission denied (TCC / root required)"`, com `ReviewOnly: true` (`ReviewReasonSystemProtected`). Busca de `*.code_sign_clone` é isolada no bucket seguro do usuário logado via `TMPDIR` / `DARWIN_USER_CACHE_DIR`. Estritamente zero uso de sudo/root.
5. `Segurança, Whitelist Imutável e Process Guard` (`MAC-05`): whitelist imutável em Go (`internal/cleaner/macos/whitelist.go`) bloqueando incondicionalmente dados sensíveis (`CloudKit`, `Mobile Documents`, `Keychain`, `Safari` state, VPNs como `Tailscale`/`WireGuard`, 1Password). Process Guard determinístico mapeia bundle IDs aos processos em execução via `cleaner.CommandExecutor` (`pgrep -x`), marcando `ReviewOnly = true` (`ReviewReasonActiveProcess`) se ativos ou incertos e revalidando imediatamente pré-Clean. Verificação de propriedade (`st_uid == current_uid`) e Symlink Guard protegem contra symlinks arbitrários com zero-CGO.

</domain>

<decisions>
## Implementation Decisions

### Trash & Volumes Externos (MAC-01)
- **D-01:** **Lixeira sempre ReviewOnly por Padrão:** `~/.Trash` é sempre contabilizado no inventário de scan como `ReviewOnly: true`, `ReviewReason: "trash_requires_flag"`, `EligibleBytes: 0` e `TTLEligible: false`. Apenas quando a flag `--trash` (ou config `safety.trash_enabled`) for fornecida na limpeza é que o alvo torna-se elegível para remoção. — **Reversibility:** costly — define a política central de elegibilidade da Lixeira.
- **D-02:** **Escopo Restrito a `$HOME/.Trash`:** A varredura de Lixeira limita-se estritamente ao diretório do usuário `$HOME/.Trash`, ignorando deliberadamente `/Volumes/*/.Trashes` em mídias removíveis ou volumes secundários para mitigar riscos de desmontagem ou falhas de permissão externas. — **Reversibility:** reversible — restringe o caminho escaneado da Lixeira.
- **D-03:** **Expurgo Direto dos Itens da Lixeira:** Quando autorizado com `--trash`, o cleaner realiza a deleção permanente direta (`os.RemoveAll`) dos itens contidos em `~/.Trash`, sem movê-los para a quarentena reversível interna (`internal/trash`), e preservando a própria pasta raiz `~/.Trash`. — **Reversibility:** costly — define o comportamento destrutivo autorizado da limpeza de lixeira.
- **D-04:** **Degradação Tolerante em Arquivos Bloqueados:** Ao esvaziar a Lixeira, tentar `chmod u+w` em itens somente-leitura. Caso algum item continue inacessível ou travado pelo sistema/TCC, registrar aviso detalhado com bytes remanescentes sem abortar a limpeza dos demais itens. — **Reversibility:** reversible.

### Granularidade e Deduplicação de User Caches (MAC-01, MAC-03)
- **D-05:** **Lista Estrita de Exclusão de Caches Conhecidos:** O cleaner de macOS mantém uma tabela de exclusão contendo os caminhos e subpastas de `~/Library/Caches` já gerenciados por cleaners dedicados (`uv`, `pnpm`, `mise`, `ms-playwright`, `cypress`, `homebrew`, `cocoapods`, `xcode`, `orbstack`, editores AI/Electron), garantindo zero dupla contagem de bytes. — **Reversibility:** costly — define o contrato de convivência entre o cleaner macOS e ecossistemas dedicados.
- **D-06:** **Consolidação Categórica dos Alvos de Usuário:** Apresentar alvos consolidados na CLI/TUI (`User App Caches` e `User App Logs`), agrupando os bundles secundários em vez de listar centenas de linhas avulsas, destacando alvos individuais apenas para itens maiores (> 50MB) ou com regras específicas. — **Reversibility:** reversible.
- **D-07:** **Folhas Restritas em Application Support:** Varrer estritamente subpastas permitidas (`Cache`, `Caches`, `logs`, `Logs`, `Crashpad/completed`) dentro de `~/Library/Application Support/*`, mantendo um alvo dedicado para o caso de alto volume `Spotify PersistentCache` e consolidando os demais em `Application Support Caches & Logs`. — **Reversibility:** costly — garante que bancos de dados e preferências em Application Support nunca sejam tocados.
- **D-08:** **Positive Allowlist para Group Containers:** Em `~/Library/Group Containers`, varrer apenas pastas `Library/Caches` e `Library/Logs` de grupos explicitamente autorizados na positive allowlist, excluindo estritamente `*.dev.orbstack` (pertencente ao cleaner docker) e nunca tocando a raiz do Group Container. — **Reversibility:** costly — protege dados de compartilhamento entre aplicativos no macOS.

### Permissões TCC e Degradação Graciosa em Logs/Sistema (MAC-04)
- **D-09:** **Notação de Cobertura Incompleta via `ProviderMetrics`:** Quando o acesso a `/private/var/log` ou `/Library/Logs/DiagnosticReports` falhar por TCC ou permissão negada, preencher `ProviderMetrics.Partial = true` e `StatusNote: "permission denied (TCC / root required)"`, marcando o alvo como `ReviewOnly: true` (`ReviewReason: domain.ReviewReasonSystemProtected`) e acumulando os bytes que puderam ser lidos. — **Reversibility:** costly — padroniza o reporte de TCC em toda a suite do DevClean.
- **D-10:** **Zero Sudo / Sem Elevação de Privilégios:** O DevClean opera estritamente com os privilégios do processo do usuário, sem invocar `sudo` ou solicitar elevação interativa. Logs inacessíveis do sistema permanecem reportados como protegidos pelo sistema. — **Reversibility:** one-way — compromisso de segurança de nunca executar comandos de elevação de privilégio.
- **D-11:** **Isolamento de `*.code_sign_clone` no Bucket do Usuário:** Localizar a pasta de cache Darwin do usuário via `os.Getenv("TMPDIR")` / `getconf DARWIN_USER_CACHE_DIR`, buscando `*.code_sign_clone` exclusivamente dentro do subdiretório `C/` do usuário corrente, sem escanear pastas de outros usuários em `/private/var/folders`. — **Reversibility:** reversible.
- **D-12:** **Segregação de Relatórios de Diagnóstico:** Separar a descoberta em dois alvos distintos: `User Diagnostic Reports` (`~/Library/Logs/DiagnosticReports`, elegível para limpeza) e `System Diagnostic Reports` (`/Library/Logs/DiagnosticReports`, marcado como ReviewOnly se faltar permissão ou for do sistema). — **Reversibility:** reversible.

### Positive Allowlists & Proteção de Dados Sensíveis (MAC-02, MAC-05)
- **D-13:** **Positive Allowlist Canônica dos 8 Daemons:** Cobrir estritamente os 8 daemons declarados na spec (`Siri suggestions`, `Wallpaper agent`, `com.apple.mediaanalysisd`, `Geod`, `com.apple.helpd`, `Apple Media Services`, `Parsecd` e `Handoff`), agrupados sob o alvo `macOS Daemon Caches`. — **Reversibility:** costly — define o escopo positivo de daemons da Apple.
- **D-14:** **Process Guard Determinístico via `cleaner.CommandExecutor`:** Implementar mapeamento determinístico de bundle IDs para nomes de processos executando `pgrep -x`. Se o processo estiver ativo ou incerto (fail-closed), o alvo recebe `ReviewOnly = true`, `ReviewReason = domain.ReviewReasonActiveProcess`, `EligibleBytes = 0` e revalida obrigatoriamente pré-Clean. — **Reversibility:** costly — integra o Process Guard no ciclo de vida do cleaner macOS.
- **D-15:** **Whitelist Imutável de Bundles Sensíveis em Go:** Criar `internal/cleaner/macos/whitelist.go` com lista imutável e inegociável contendo `CloudKit`, `Mobile Documents`, `com.apple.keychain*`, `Safari` state, VPNs (`Tailscale`, `WireGuard`) e gerenciadores de senhas (`1Password`), imune a qualquer flag ou configuração. — **Reversibility:** one-way — garantia de segurança inviolável do ecossistema macOS.
- **D-16:** **Symlink Guard e Verificação de Propriedade (Ownership) com Zero-CGO:** Rejeitar travessia de symlinks que apontem para fora da raiz do cache, nunca seguir links para locais de sistema ou protegidos, e verificar propriedade via `syscall.Stat_t` (`st_uid == current_uid`) mantendo compilação 100% pura sem CGO. Alvos com symlinks ambíguos recebem `ReviewOnly = true` (`ReviewReasonAmbiguous`). — **Reversibility:** costly — define a verificação de segurança de baixo nível do filesystem.

### the agent's Discretion
- Formatação dos nomes legíveis dos alvos consolidados na CLI e TUI.
- Limite exato em MB para desmembrar um bundle específico de `User App Caches` em linha própria (padrão sugerido: 50MB).
- Definição da lista exata de bundles de Group Containers com caches/logs regeneráveis autorizados.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Paridade e Requisitos
- `.planning/ROADMAP.md` §Phase 11 — Metas, dependências e critérios de sucesso da Fase 11 (MAC-01 a MAC-05).
- `.planning/REQUIREMENTS.md` §macOS system and user cleaner — Requisitos normativos MAC-01, MAC-02, MAC-03, MAC-04 e MAC-05.
- `.planning/SPEC-v2.3-MOLE-PARITY.md` §Frente 3: Novo Cleaner de Sistema e Usuário macOS — Especificação técnica completa de paridade com o Mole para caches/logs de macOS, Lixeira, Group Containers e daemons.

### Padrões de Implementação e Arquitetura no Codebase
- `internal/cleaner/ai/process.go` — Padrão de Process Guard fail-closed com `checkProcessStatus` e `cleaner.CommandExecutor`.
- `internal/safety/blacklist.go` — Blacklist imutável e validação de raízes protegidas do sistema (`IsCoreBlacklistedDirect`, `SystemRoots`).
- `internal/domain/types.go` — Definições centrais de `Target`, `ProviderMetrics`, `ReviewReasonSystemProtected`, `ReviewReasonActiveProcess` e `ReviewReasonAmbiguous`.
- `internal/engine/register.go` — Ponto de registro de novos ecossistemas no motor do DevClean com injeção de executor.
- `internal/platform/resolver_darwin.go` — Resolução de diretórios canônicos no macOS sem CGO.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `cleaner.CommandExecutor` / `cleaner.MockCommandExecutor` (`internal/cleaner/executor.go`): Abstração para execução de comandos (`pgrep`) permitindo testes 100% herméticos sem processos reais.
- `ai.checkProcessStatus` (`internal/cleaner/ai/process.go`): Padrão de verificação de processos com fail-closed para saída inesperada de `pgrep`.
- `safety.IsCoreBlacklisted` (`internal/safety/blacklist.go`): Proteção contra inclusão acidental de caminhos vitais do sistema.
- `deleter.Coordinator` (`internal/deleter/coordinator.go`): Pipeline central de mutação que rejeita automaticamente alvos com `ReviewOnly = true`.

### Established Patterns
- **Desacoplamento Scan vs. TTL:** `Scan()` reporta 100% dos bytes em disco; `EligibleBytes` e `TTLEligible` só são ajustados de acordo com políticas ou flags explícitas.
- **Fail-Closed Policy:** Qualquer ambiguidade de permissão, processo ativo, link quebrado ou incerteza marca o alvo com `ReviewOnly: true`, `EligibleBytes: 0` e `ReviewReason` semântico.
- **Hermetic Tests com `t.TempDir()`:** Testes unitários constroem árvores de arquivos temporários representando `Library/Caches`, sentinelas e verificam que arquivos protegidos permanecem intactos após `Scan()` e `Clean()`.
- **Compilação Zero-CGO:** Código compila com `CGO_ENABLED=0` para Darwin e Linux, usando tags de build quando apropriado ou stubs neutros para Linux.

### Integration Points
- `internal/cleaner/macos/`: Novo pacote implementando `domain.Cleaner` com submódulos para `user.go`, `system.go`, `trash.go`, `daemons.go`, `process.go` e `whitelist.go`.
- `internal/engine/register.go`: Registro do novo ecossistema `macos` em `RegisterDefaultCleanersWithExecutor`.
- `internal/config/config.go`: Adição de `MacOS EcosystemConfig` e `Hooks` correspondentes em `EcosystemsConfig`.

</code_context>

<specifics>
## Specific Ideas
- Alinhamento total com as categorias de paridade do Mole: `User essentials`, `App caches`, `Application Support` e `System`.
- Na Lixeira, assegurar que arquivos bloqueados com flags imutáveis (`uchg`) ou permissão 0444 recebam tentativa com `chmod u+w` antes de falhar, relatando graciosamente os bytes remanescentes.

</specifics>

<deferred>
## Deferred Ideas
None — discussion stayed within phase scope.

</deferred>

---

*Phase: 11-macOS User/System Caches & Logs*  
*Context gathered: 2026-10-16*

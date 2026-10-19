# Phase 8: Node, Headless, Shell/CLI & Xcode - Context

**Gathered:** 2026-10-15  
**Status:** Ready for planning  

<domain>
## Phase Boundary

Garantir a descoberta precisa por presença em disco de artefatos do ecossistema Node e navegadores de automação (DEV-01), ferramentas de shell e CLI (DEV-02) e Xcode/Simulators (DEV-03), aplicando a política ReviewOnly centralizada para bloquear exclusão insegura quando ferramentas estiverem em execução ou em estado incerto:
1. `node` cleaner: descobre por presença em disco `~/.npm/_cacache`, `~/.npm/_npx`, `~/.npm/_logs`, Corepack (`~/.cache/node/corepack`, `~/Library/Caches/node/corepack`) e navegadores de automação (Puppeteer `~/.cache/puppeteer`, Playwright `~/Library/Caches/ms-playwright` / `~/.cache/ms-playwright`, Cypress `~/Library/Caches/Cypress` / `~/.cache/Cypress`), mesmo sem os executáveis no PATH (`DEV-01`).
2. `pnpm` cleaner: descobre por presença em disco o cache de metadados (`~/Library/Caches/pnpm` e `~/.cache/pnpm`), mantendo a integridade do ecossistema `pnpm` (`DEV-01`).
3. `shell` cleaner (`internal/cleaner/shell`): novo cleaner sob o ecossistema `shell`, cobrindo `mise` (`mise cache` e `mise downloads`), GitHub CLI (`gh`), `pre-commit`, `Oh My Zsh` (`~/.oh-my-zsh/cache`) e `~/.zcompdump*` sem ler nem alterar configurações (`.zshrc`, `.bashrc`) ou histórico (`.zsh_history`, `.bash_history`) (`DEV-02`).
4. `xcode` cleaner: expande a cobertura para `CoreSimulator/Caches`, `CoreSimulator/Temp`, `iOS DeviceSupport`, `watchOS DeviceSupport` e `Archives`. Aplica detecção fail-closed de processos ativos (`Xcode`, `Simulator`): se ativos ou em estado incerto, os alvos recebem `ReviewOnly = true`, `ReviewReason = ReviewReasonActiveProcess` e `EligibleBytes = 0` (`DEV-03`).
5. Testes unitários herméticos com `t.TempDir()`, sentinels, casos de symlink e permissões, asserções de DryRun e cobertura acima de 85% nos pacotes alterados.

</domain>

<decisions>
## Implementation Decisions

### Node & Headless Automation Caches (DEV-01)
- **D-01:** **Alvos Granulares por Ferramenta no Cleaner Node:** Incorporar alvos individuais no cleaner `internal/cleaner/node` para cada ferramenta de automação (`node-puppeteer`, `node-playwright`, `node-cypress`) e package management (`npm-cache`, `npx-cache`, `npm-logs`, `corepack-cache`). — **Reversibility:** reversible — organiza a emissão de alvos em `node.go`.
- **D-02:** **Classificação de Risco Diferenciada:** Atribuir `RiskMedium` para downloads de binários pesados de navegadores (Puppeteer e Playwright) evitando remoções acidentais em ambientes com conectividade limitada, e `RiskLow` para caches regeneráveis gerais de pacotes e logs (npm, npx, logs, Corepack e Cypress). — **Reversibility:** reversible.
- **D-03:** **Separação Granular de Subdiretórios npm:** Apresentar `~/.npm/_cacache` como `npm cache`, `~/.npm/_npx` como `npx cache` e `~/.npm/_logs` como `npm logs`, oferecendo visibilidade analítica clara de onde o espaço em disco está concentrado. — **Reversibility:** reversible.
- **D-04:** **Pnpm Cache no Cleaner Pnpm:** O cache de metadados HTTP do pnpm (`~/Library/Caches/pnpm` no macOS e `~/.cache/pnpm` no Linux) reside no cleaner `internal/cleaner/pnpm` sob o ecossistema `pnpm`, permitindo que `IsInstalled()` detecte presença em disco sem depender do binário `pnpm` no PATH. — **Reversibility:** costly — define a divisão de responsabilidades entre os pacotes `node` e `pnpm`.
- **D-05:** **Deleção Direta via Filesystem com Validação:** Exclusão via `os.RemoveAll` nos diretórios de cache sob verificação rigorosa de `safety.Blacklist` e proteção contra travessia de symlinks (`safety.IsSymlink`), sem invocar comandos de terceiros (`npx`, `cypress`) e operando 100% offline. — **Reversibility:** reversible.

### Shell & CLI Tools Cleaner (DEV-02)
- **D-06:** **Novo Cleaner Unificado 'internal/cleaner/shell':** Criar o cleaner `internal/cleaner/shell` registrado sob o ecossistema `"shell"` no motor (`internal/engine/register.go`) e no config (`internal/config/config.go`), consolidando as ferramentas CLI do desenvolvedor. — **Reversibility:** costly — adiciona um novo ecossistema no registro do motor e modelo de configuração.
- **D-07:** **Separação de Alvos no mise:** Segmentar mise em dois alvos distintos: `mise cache` (`~/Library/Caches/mise` ou `~/.cache/mise`) com risco Low, e `mise downloads` (`~/.local/share/mise/downloads`) com risco Low, permitindo limpeza de arquivos residuais de download de runtimes já instalados. — **Reversibility:** reversible.
- **D-08:** **Alvo Agregado de Autocompletar do Zsh:** Descobrir dumps de autocompletar via padrão `~/.zcompdump*` no `$HOME` (e `$ZDOTDIR` se configurado) e agrupá-los em um único alvo consolidado `Zsh completion cache`, mantendo a listagem concisa e sem tocar em `.zshrc` ou `.zsh_history`. — **Reversibility:** reversible.
- **D-09:** **Classificação de Risco de Ferramentas CLI:** Atribuir `RiskLow` para `gh` e `Oh My Zsh`, e `RiskMedium` para `pre-commit` (devido ao custo de rede para re-clonar repositórios de hooks caso o desenvolvedor realize commits subsequentes offline). — **Reversibility:** reversible.
- **D-10:** **IsInstalled por Presença em Disco no Shell Cleaner:** `IsInstalled()` retorna `true` se qualquer um dos diretórios de cache (`mise`, `gh`, `pre-commit`, `omz`, `zcompdump`) existir ou se algum dos comandos estiver no PATH, respeitando o princípio de descoberta puramente por disco. — **Reversibility:** reversible.

### Xcode, Simulators & Process Guard (DEV-03)
- **D-11:** **Bloqueio Fail-Closed com ReviewOnly sob Processo Ativo:** Quando `Xcode` ou `Simulator.app` estiverem em execução ou sua detecção retornar erro/estado incerto, os alvos afetados do Xcode (`CoreSimulator Caches`, `CoreSimulator Temp`, `iOS DeviceSupport`, `watchOS DeviceSupport`, `DerivedData` e `Archives`) são marcados com `ReviewOnly: true`, `ReviewReason: domain.ReviewReasonActiveProcess`, `EligibleBytes: 0` e `TTLEligible: false`. — **Reversibility:** costly — integra o cleaner de Xcode com o portão de segurança do Coordinator.
- **D-12:** **Verificação Injetável de Processos com CommandExecutor:** Criar mecanismo de detecção de processos baseado em `exec.CommandContext("pgrep", "-x", processName)` através da interface `cleaner.CommandExecutor`, garantindo testabilidade hermética com mocks e zero uso de CGO. — **Reversibility:** costly — estabelece o padrão de verificação de processos reutilizável nas Fases 9, 11 e 12.
- **D-13:** **Política de Segurança para Xcode Archives:** `~/Library/Developer/Xcode/Archives` recebe classificação `RiskHigh` quando o Xcode estiver fechado (alertando sobre a perda potencial de símbolos dSYM para depuração de releases de produção), e é bloqueado com `ReviewOnly` se o Xcode estiver em execução. — **Reversibility:** reversible.
- **D-14:** **Alvos Dedicados para CoreSimulator e DeviceSupport:** Segmentar alvos individuais para `CoreSimulator Caches`, `CoreSimulator Temp`, `iOS DeviceSupport` e `watchOS DeviceSupport`, permitindo exclusão direta de filesystem via `os.RemoveAll` quando as ferramentas estiverem inativas. — **Reversibility:** reversible.

### the agent's Discretion
- Formatação e nomes exatos dos identificadores de alvos (IDs e Categories em `domain.Target`).
- Definição de constantes de caminhos específicos por plataforma (Darwin vs Linux) usando helpers de diretório de usuário.
- Heurísticas de detecção de múltiplos arquivos de dump do Zsh (`filepath.Glob`).

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Especificação e Requisitos de Paridade
- `.planning/SPEC-v2.3-MOLE-PARITY.md` — Frente 2 (linhas 59-72) detalhando ecossistema Node, Browsers de Teste, Xcode & Simulators e Shell & CLI Tools.
- `.planning/REQUIREMENTS.md` — Requisitos DEV-01, DEV-02, DEV-03 e POL-01/POL-02.
- `.planning/ROADMAP.md` — Escopo da Fase 8 e critérios de sucesso.
- `.planning/PROJECT.md` — Diretrizes de arquitetura, segurança e paridade de inventário.

### Decisões e Políticas Anteriores
- `.planning/phases/07-reviewonly-coordinator-policy/07-CONTEXT.md` — Decisões de ReviewOnly (`ReviewReasonActiveProcess`), invariantes do Coordinator e DryRun.
- `.planning/phases/06-scan-ttl-identity-disk-discovery/06-CONTEXT.md` — Desacoplamento Scan vs TTL e IsInstalled baseado em disco.

### Código Existente Relevante
- `internal/cleaner/node/node.go` — Cleaner atual do ecossistema Node (a ser expandido com subdiretórios npm, Corepack e browsers de automação).
- `internal/cleaner/pnpm/pnpm.go` — Cleaner pnpm atual (a ser expandido com detecção de pnpm cache em disco).
- `internal/cleaner/xcode/xcode.go` — Cleaner Xcode atual (a ser expandido com CoreSimulator, DeviceSupport, Archives e guarda de processos).
- `internal/cleaner/executor.go` — Interface `CommandExecutor` usada para injeção de comandos de verificação de processos.
- `internal/engine/register.go` — Registro central de cleaners e novos ecossistemas.
- `internal/config/config.go` — Configuração e habilitação padrão de ecossistemas (adição do ecossistema `shell`).
- `internal/safety/path.go` — Verificação de symlinks e validação de caminhos seguros contra blacklist.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `cleaner.CommandExecutor` / `cleaner.DefaultExecutor`: Execução segura de comandos sem interpolação de shell, ideal para invocar `pgrep` com mock em testes.
- `safety.Blacklist`: Proteção central de caminhos do sistema contra deleção inadvertida.
- `safety.IsSymlink`: Resolução e prevenção contra travessia de links simbólicos.
- `devfs.CalcDirSize` / `devfs.CalcDirSizes`: Cálculo concorrente eficiente de espaço em disco.
- `domain.Target`: Suporte nativo a `ReviewOnly`, `ReviewReasonActiveProcess`, `RiskLevel` e contabilidade de bytes.

### Established Patterns
- Descoberta por presença em disco sem depender da presença de binários no `$PATH` (`SCAN-02`).
- Relato de 100% dos bytes ocupados (`SizeBytes`) no `Scan()`, desacoplando o inventário da autorização de expurgo (`SCAN-01`).
- Interceptação central de alvos com `ReviewOnly = true` no Estágio 1 do `deleter.Coordinator` (`POL-02`).
- Testes unitários herméticos utilizando `t.TempDir()`, arquivos sentinela, diretórios mockados e asserções de DryRun (`POL-04`, `VAL-01`).

### Integration Points
- `internal/cleaner/node/node.go`: Adição de `~/.npm/_npx`, `~/.npm/_logs`, Corepack e caches de automação (Puppeteer, Playwright, Cypress).
- `internal/cleaner/pnpm/pnpm.go`: Adição da detecção do cache de metadados do pnpm por presença em disco.
- `internal/cleaner/shell/shell.go`: Novo pacote implementando `domain.Cleaner` para o ecossistema `"shell"`.
- `internal/cleaner/xcode/xcode.go`: Adição de `CoreSimulator/Caches`, `Temp`, `iOS/watchOS DeviceSupport`, `Archives` e verificação fail-closed de `pgrep` para Xcode e Simulator.
- `internal/engine/register.go`: Registro do novo cleaner `shell.New()` e atualização dos inicializadores de `node`, `pnpm` e `xcode`.
- `internal/config/config.go`: Adição de `"shell"` na lista de ecossistemas padrão suportados.

</code_context>

<specifics>
## Specific Ideas

- Paridade estrita com o comportamento do Mole (tw93/mole): descobrir com precisão caches de Playwright, Puppeteer, Cypress, Corepack, mise, GitHub CLI, pre-commit, Oh My Zsh e Xcode sem requerer que o usuário tenha todas essas ferramentas instaladas ou ativas.
- Respeito inviolável a arquivos de configuração e histórico de shell: varredura restrita a caches e dumps (`.zcompdump*`), sem tocar em arquivos `.rc` ou histórico.
- Proteção ativa de fluxos de build do Xcode: impedir exclusões de DerivedData, arquivos temporários de simulador ou dSYMs enquanto o Xcode estiver compilando ou rodando em segundo plano.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 8-Node, Headless, Shell/CLI & Xcode*  
*Context gathered: 2026-10-15*  

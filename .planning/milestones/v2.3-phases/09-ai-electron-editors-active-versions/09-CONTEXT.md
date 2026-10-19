# Phase 9: AI/Electron Editors & Active Versions - Context

**Gathered:** 2026-10-15  
**Status:** Ready for planning  

<domain>
## Phase Boundary

Garantir a descoberta precisa por presença em disco e limpeza segura de artefatos de editores AI e Electron (Code, Code Insiders, Cursor, Antigravity, Windsurf, Claude Desktop, OpenCode e Zed), backups de workspace, extensões obsoletas e versões inativas do Claude Code CLI, aplicando Process Guard e política ReviewOnly para proteger dados de sessão e processos ativos:
1. `editors` scanner (`internal/cleaner/ai/editors.go`): cobre 8 editores (Code, Code Insiders, Cursor, Antigravity, Windsurf, Claude Desktop, OpenCode e Zed), varrendo folhas regeneráveis Chromium/Electron (`Cache`, `Code Cache`, `GPUCache`, `DawnCache`, `DawnWebGPUCache`, `DawnGraphiteCache`, `Service Worker/CacheStorage`, `Service Worker/ScriptCache`, `CachedData`, `CachedExtensionVSIXs`, `webviewCache`, `logs`, `Crashpad/completed`, `Sentry`) e estruturas nativas do Zed e OpenCode (`AI-01`).
2. `workspaceStorage` e extensões obsoletas: seleciona estritamente `state.vscdb.backup` dentro de `User/workspaceStorage/*/`, preservando intacto o `state.vscdb` primário. Lê `.obsolete` exclusivamente como metadado (sem apagá-lo) e expurga apenas diretórios de extensões referenciados como `true`, protegendo extensões ativas em `extensions/` (`AI-02`).
3. `claude` versions cleaner (`internal/cleaner/ai/claude.go`): descobre versões antigas em `~/.local/share/claude/versions/*` e `~/.claude/local`, resolvendo o symlink ativo com `filepath.EvalSymlinks`, preservando a versão em uso e falhando fechado (`ReviewOnly = true`, `EligibleBytes = 0`) caso o symlink esteja quebrado, ausente ou ambíguo (`AI-03`).
4. Process Guard para editores e CLI: detecção fail-closed via `cleaner.CommandExecutor` (`pgrep -x`) de processos de editores (`Code`, `Cursor`, `Antigravity`, `Windsurf`, `Claude`, `zed`, `opencode`) e do CLI `claude`. Se ativos ou em estado incerto, alvos do editor/ferramenta correspondente recebem `ReviewOnly = true`, `ReviewReason = ReviewReasonActiveProcess` e `EligibleBytes = 0`. Revalidação imediata pré-exclusão no `Clean()`.
5. Testes unitários herméticos com `t.TempDir()`, sentinelas, casos de symlink e permissões, asserções de DryRun e cobertura acima de 85% nos pacotes alterados (`VAL-01`, `VAL-02`).

</domain>

<decisions>
## Implementation Decisions

### Granularidade e Agrupamento dos Editores (AI-01)
- **D-01:** **Agrupamento Funcional por Editor:** Apresentar 3 a 4 alvos funcionais consolidados por editor (`Caches Chromium/GPU`, `Logs & Crashpad`, `Extensões/VSIXs`, `Workspace Backups`), evitando poluição de listagem com dezenas de folhas isoladas e mantendo visibilidade analítica clara. — **Reversibility:** reversible.
- **D-02:** **Mapeamento Canônico Específico por Editor:** Mapear Zed em suas estruturas nativas (`~/Library/Caches/dev.zed.Zed`, `~/.cache/zed` e logs), OpenCode em `~/.cache/opencode`, enquanto os editores baseados em Electron/VS Code mapeiam suas folhas Chromium completas. — **Reversibility:** costly — define os caminhos canônicos e adapters de cada editor.
- **D-03:** **Centralização no Pacote 'internal/cleaner/ai/editors.go':** Manter a lógica de todos os editores centralizada em `internal/cleaner/ai/editors.go` sob o ecossistema `ai` já registrado no motor e config, sem criar um ecossistema redundante. — **Reversibility:** costly — define a organização dos cleaners.
- **D-04:** **Classificação de Risco Diferenciada:** Atribuir `RiskLow` para caches regeneráveis gerais (Chromium, GPU, Code Cache, logs, Crashpad) e `RiskMedium` para backups de workspace (`state.vscdb.backup`) e `CachedExtensionVSIXs`. — **Reversibility:** reversible.

### Process Guard de Editores (AI-01, AI-02, AI-03)
- **D-05:** **Detecção de Processos via CommandExecutor:** Usar `cleaner.CommandExecutor` com `pgrep -x` para os nomes exatos de processos (`Code`, `Code - Insiders`, `Cursor`, `Antigravity`, `Windsurf`, `Claude`, `zed`, `opencode`), permitindo injeção hermética de mocks em testes e zero CGO. — **Reversibility:** costly — estabelece a interface e implementação de guarda de processos em editores.
- **D-06:** **Bloqueio Isolado por Editor com ReviewOnly:** Quando um editor estiver aberto ou sua detecção retornar estado incerto/erro, apenas os alvos daquele editor específico recebem `ReviewOnly: true`, `ReviewReason: domain.ReviewReasonActiveProcess`, `EligibleBytes: 0` e `TTLEligible: false`; editores inativos continuam elegíveis. — **Reversibility:** reversible.
- **D-07:** **Revalidação Imediata Pré-Exclusão no Clean():** O método `Clean()` revalida a presença de processos ativos antes de realizar qualquer deleção física, abortando a exclusão dos alvos do editor que tenha sido iniciado após o scan. — **Reversibility:** costly — adiciona checagem ativa no ciclo de vida de mutação.
- **D-08:** **Fail-Closed Estrito para Ferramentas Ausentes:** Se `pgrep` não estiver disponível ou falhar por erro inesperado, classificar como estado incerto (`uncertain=true`) e aplicar `ReviewOnly` para proteger os dados do usuário. — **Reversibility:** reversible.

### Versões do Claude CLI (AI-03)
- **D-09:** **Resolução Canônica da Versão Ativa:** Inspecionar links simbólicos ativos (como `current` em `~/.local/share/claude/versions/` ou symlinks do binário em `~/.local/share/claude`), resolver o destino real com `filepath.EvalSymlinks` e proteger essa versão e seu symlink da deleção. — **Reversibility:** costly — define a regra de ouro de preservação do binário ativo.
- **D-10:** **Fail-Closed em Symlinks Quebrados ou Ambíguos:** Se o symlink de versão estiver ausente, quebrado ou ambíguo, marcar todas as versões encontradas como `ReviewOnly: true`, `EligibleBytes: 0` e razão de revisão explícita (`ReviewReasonActiveVersion` ou `ReviewReasonAmbiguous`), impedindo qualquer remoção automática mas exibindo o inventário real. — **Reversibility:** reversible.
- **D-11:** **Process Guard para Claude CLI:** Inspecionar se o processo `claude` está ativo via `pgrep -x claude`; se estiver ativo, marcar alvos de versão e caches do Claude com `ReviewOnly: true` (`ReviewReasonActiveProcess`) e `EligibleBytes: 0`. — **Reversibility:** reversible.
- **D-12:** **Classificação de Risco Low para Versões Antigas Inativas:** Versões antigas confirmadamente inativas recebem `RiskLow`, pois a versão ativa está garantida e binários antigos são sobras de autoupdates substituíveis. — **Reversibility:** reversible.

### Workspace Storage e Extensões Obsoletas (AI-02)
- **D-13:** **Remoção Cirúrgica de Backups de Estado:** Varrer `User/workspaceStorage/*/state.vscdb.backup` e selecionar estritamente os arquivos `.backup`, preservando intacto o `state.vscdb` primário e a estrutura dos diretórios de workspace. — **Reversibility:** reversible.
- **D-14:** **Preservação de '.obsolete' e Proteção Positiva:** Ler `.obsolete` estritamente como metadado sem jamais apagá-lo; elencar apenas os diretórios correspondentes a extensões marcadas como `true`, com validação rigorosa contra symlinks (`safety.IsSymlink`) e path traversal (`..`). Pastas de extensões ativas em `extensions/` permanecem intocáveis. — **Reversibility:** costly — define o contrato de integridade de extensões.
- **D-15:** **Cobertura de Todas as Famílias VS Code:** Escanear raízes de extensões de todos os editores presentes (`~/.vscode/extensions`, `~/.vscode-insiders/extensions`, `~/.cursor/extensions`, `~/.antigravity/extensions`, `~/.windsurf/extensions`). — **Reversibility:** reversible.
- **D-16:** **Desacoplamento Scan vs TTL Conforme Padrão v2.3:** `Scan()` reporta 100% dos bytes reais em `SizeBytes`; `EligibleBytes` e `TTLEligible` respeitam o TTL (`MaxAgeDays == 0` ou idade > cutoff tornam elegível). — **Reversibility:** reversible.

### the agent's Discretion
- Formatação e rotulação exata das categorias e IDs de alvos gerados em `editors.go` e `claude.go`.
- Mapeamento detalhado de subpastas do Claude Desktop em `~/Library/Application Support/Claude` e `~/Library/Caches/com.anthropic.claudefordesktop*`.
- Helpers internos para parsing seguro de `.obsolete` e resolução de links simbólicos.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Especificação e Requisitos de Paridade
- `.planning/SPEC-v2.3-MOLE-PARITY.md` — Frente 2 (linhas 59-66) cobrindo Motor Unificado Chromium/Electron e Claude Code CLI.
- `.planning/REQUIREMENTS.md` — Requisitos AI-01, AI-02, AI-03, POL-01, POL-02, VAL-01, VAL-02.
- `.planning/ROADMAP.md` — Escopo da Fase 9 e critérios de sucesso.
- `.planning/PROJECT.md` — Diretrizes de arquitetura, segurança e paridade de inventário.

### Decisões e Políticas Anteriores
- `.planning/phases/08-node-headless-shell-cli-xcode/08-CONTEXT.md` — Padrão de Process Guard injetável com CommandExecutor e checagem fail-closed via `pgrep`.
- `.planning/phases/07-reviewonly-coordinator-policy/07-CONTEXT.md` — Política central ReviewOnly e invariantes do Coordinator.
- `.planning/phases/06-scan-ttl-identity-disk-discovery/06-CONTEXT.md` — Desacoplamento Scan vs TTL e IsInstalled baseado em disco.

### Código Existente Relevante
- `internal/cleaner/ai/editors.go` — Implementação atual de editores a ser expandida com agrupamento funcional, novas folhas e Process Guard.
- `internal/cleaner/ai/claude.go` — Implementação atual do Claude CLI a ser expandida com limpeza segura de versões e resolução de symlink.
- `internal/cleaner/ai/ai.go` — Ponto de entrada do ecossistema AI e cleaner orchestration.
- `internal/cleaner/executor.go` — Interface `CommandExecutor` para execução testável de `pgrep`.
- `internal/safety/path.go` — Verificação de symlinks e proteção contra travessia de caminhos.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `cleaner.CommandExecutor` / `cleaner.DefaultExecutor`: Execução segura de comandos sem interpolação de shell para invocar `pgrep` com mock em testes herméticos.
- `safety.Blacklist`: Proteção central de caminhos do sistema contra deleção inadvertida.
- `safety.IsSymlink`: Resolução e prevenção contra travessia de links simbólicos.
- `devfs.CalcDirSize` / `devfs.CalcDirSizes`: Cálculo eficiente de espaço em disco.
- `domain.Target`: Suporte a `ReviewOnly`, `ReviewReasonActiveProcess`, `RiskLevel` e contabilidade de bytes.

### Established Patterns
- Descoberta por presença em disco sem depender de binários no `$PATH` (`SCAN-02`).
- Relato de 100% dos bytes ocupados (`SizeBytes`) no `Scan()`, desacoplando o inventário da autorização de expurgo (`SCAN-01`).
- Interceptação central de alvos com `ReviewOnly = true` no Estágio 1 do `deleter.Coordinator` (`POL-02`).
- Testes herméticos com `t.TempDir()`, arquivos sentinela, diretórios mockados e asserções de DryRun (`POL-04`, `VAL-01`).

### Integration Points
- `internal/cleaner/ai/editors.go`: Adição de subpastas completas Chromium/Electron, `state.vscdb.backup`, `CachedExtensionVSIXs`, `webviewCache`, Zed e OpenCode.
- `internal/cleaner/ai/claude.go`: Adição de `~/.local/share/claude/versions/*`, `~/.claude/local`, resolução de symlink e guarda fail-closed.
- `internal/cleaner/ai/ai.go`: Injeção de `cleaner.CommandExecutor` para `scanEditors` e `scanClaude`, e revalidação pré-exclusão no `Clean()`.

</code_context>

<specifics>
## Specific Ideas

- Paridade estrita com o comportamento do Mole (tw93/mole): descobrir com precisão caches e logs de Code, Cursor, Antigravity, Windsurf, Claude Desktop, OpenCode e Zed.
- Integridade inegociável de sessões e preferências: jamais tocar em `state.vscdb` (apenas `.backup`), nunca apagar o arquivo `.obsolete`, e proteger todas as extensões em uso.
- Preservação inviolável de versão ativa do Claude CLI: garantir que a versão em execução ou referenciada por symlinks válidos nunca seja candidata a deleção.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 09-AI/Electron Editors & Active Versions*  
*Context gathered: 2026-10-15*  

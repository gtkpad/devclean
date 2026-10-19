# Milestones

## v2.3 Paridade Total com Mole (tw93/mole) (Shipped: 2026-10-19)

**Phases completed:** 9 phases, 32 plans, 64 tasks  
**Codebase:** ~72.300 Go LOC | **Coverage:** >89% global (>85% em todos os pacotes modificados)  
**Git range:** `bc061eb..25c8796` (224 files changed, +34.950 / -3.272)  
**Verification:** 100% verified (`v2.3-MILESTONE-AUDIT.md`) | Closeout: verified_closeout  

**Key accomplishments:**
- **Descoberta Desacoplada e Identidade em Disco (Fase 6)**: Desacoplamento da varredura de mtime/TTL, descoberta determinística de raízes em `$HOME` (`Documents`, `Projects`, etc.) mesmo sem binários no `$PATH`, e resolução de propriedade canônica para prevenir dupla contagem de bytes.
- **Política ReviewOnly e Interceptação no Coordinator (Fase 7)**: Modelagem de alvos `ReviewOnly` com razões explícitas (`⊙`), interceptação no Estágio 1 do `deleter.Coordinator` bloqueando mutações automáticas em CLI, TUI, MCP e Scheduler, e garantia de mutação zero (`DryRun`).
- **Ferramentas de Desenvolvedor, Shell/CLI e Xcode (Fase 8)**: Expansão com limpadores para Node/headless (npm, npx, corepack, pnpm, Puppeteer, Playwright, Cypress), ferramentas de shell (mise, gh, pre-commit, omz, zcompdump) e alvos Xcode com Process Guard fail-closed.
- **Editores AI/Electron e Preservação de Versões Ativas (Fase 9)**: Cobertura cirúrgica de caches regeneráveis em 8 editores (VS Code, Insiders, Cursor, Antigravity, Windsurf, Claude Desktop, OpenCode, Zed), proteção de bancos `state.vscdb.backup` e preservação da versão ativa do Claude CLI.
- **Métricas de Provedores e Proteção de Lojas Compartilhadas (Fase 10)**: Métricas ricas de Docker, separação de caches efêmeros vs volumes/VM no OrbStack, e bloqueio permanente `ReviewOnly` de lojas de pacotes compartilhados (NuGet, pnpm store, Docker data).
- **Limpador macOS com Process Guard e Degradação Graciosa (Fase 11)**: Varredura de caches e logs do usuário e sistema, lixeira com opt-in explícito, allowlists positivas de daemons Apple, Process e Symlink Guards, e degradação com `Partial: true` perante TCC.
- **Limpador de Navegadores com Escudo Inviolável de Sessões (Fase 12)**: Suporte a 6 navegadores (Chrome, Brave, Arc, Edge, Firefox, Safari) restrito a caches regeneráveis com bloqueio estrito de cookies, histórico e sessões, e ScriptCache bloqueado como `ReviewOnly`.
- **Leftovers de Aplicativos e LaunchAgents Órfãos (Fase 13)**: Parser puro Go de plists XML/binários, classificador de LaunchAgents obsoletos vs ambíguos, indexador de bundles de aplicativos e catalogação estritamente informativa sem deleção automática.
- **Validação Integrada e Recibos de Aceitação no macOS (Fase 14)**: Suíte de regressão E2E multi-canal, cobertura acima de 85% em todos os pacotes modificados, compilação estática zero-CGO (macOS/Linux ARM64/AMD64) e avaliação de paridade com `tw93/mole` gerando recibos duplos (`MACOS-ACCEPTANCE.json` e `.md`).

---

## v2.2 Análise Visual, Instaladores, Expansão de Ecossistemas, Purge TUI e Scheduler Avançado (Shipped: 2026-10-14)

**Phases completed:** 5 phases, 19 plans, 50 tasks  
**Codebase:** ~48.800 Go LOC | **Coverage:** >89% global (>85% em todos os pacotes modificados)  
**Git range:** `26df0e1..9d00189` (134 files changed, +24.882 / -787)  
**Verification:** 100% verified (`v2.2-MILESTONE-AUDIT.md`) | Closeout: verified_closeout  

**Key accomplishments:**
- **Expansão de Ecossistemas e Presets de Limpeza (Fase 1)**: Expansão do motor de 13 para 18 ecossistemas de desenvolvimento com 5 novos limpadores (C/C++, Flutter/Dart, Terraform/OpenTofu, CocoaPods, Elixir/Mix) e adição de presets rápidos de limpeza (`--preset safe|standard|aggressive`).
- **Higienização de Instaladores e Políticas de Lixeira (Fase 2)**: Subcomando `devclean installer` com detecção profunda de instaladores esquecidos (`.dmg`, `.pkg`, `.iso`, etc.) e auto-expiração por tempo de retenção na lixeira reversível com política FIFO de quota máxima.
- **Scheduler Avançado, Logs e Systemd User Timers (Fase 3)**: Agendamento moderno com Systemd User Timers no Linux e Launchd no macOS, suporte a `--weekly`, `--hourly`, `--at`, rotação circular de logs (10 MB) e comando de auditoria `devclean schedule logs`.
- **Análise Visual de Disco (TUI Explorer) e Purge Interativo (Fase 4)**: Navegador em tela cheia `devclean analyze` via Bubble Tea com barras Lip Gloss proporcionais e subcomando `devclean purge` interativo com agrupamento por repositórios Git, busca fuzzy e trava inviolável de blacklist.
- **Configuração Interativa via TUI (devclean config) (Fase 5)**: Interface TUI completa de 5 abas para gerenciar os 18 ecossistemas, caminhos de busca, parâmetros da lixeira, watch e scheduler com persistência atômica cirúrgica em AST YAML preservando comentários.

---

## v2.1 Robustez, Qualidade Completa e Expansão Inspirada no Mole (Shipped: 2026-10-11)

**Phases completed:** 4 phases, 14 plans, 32 tasks  
**Codebase:** ~27.000 Go LOC | **Coverage:** 89.1% global (>85% em todos os 32 pacotes)  
**Git range:** `80c5837..f546347` (99 files changed, +14.761 / -881)  
**Verification:** 100% verified (`v2.1-MILESTONE-AUDIT.md`) | Closeout: verified_closeout  

**Key accomplishments:**
- **Unificação Arquitetural de Deleção (Fase 1)**: Implementação do `deleter.Coordinator` integrando CLI, TUI e MCP; isolamento e bypass seguro da lixeira de filesystem para alvos virtuais (`docker://`); precisão aprimorada do `GitActivityScore` e suporte a worktrees e arquivos `.git`.
- **Qualidade, Estabilidade e Resiliência (Fase 1 & 2)**: Eliminação de chamadas a `os.Exit(2)` nos comandos; correção de canal concorrente no engine; suporte contínuo ao loop daemon no `devclean watch --interval`; e tratamento robusto de casos extremos (symlinks cíclicos, permissões 0555).
- **Bateria Completa de Testes e Validação E2E (Fase 2)**: Elevação da cobertura de testes para 89.1%; testes de ciclo de vida headless Bubble Tea cobrindo 96.1% do TUI; mocks de SO (`CommandExecutor`) para launchctl e crontab; e testes E2E com integridade criptográfica SHA-256.
- **Novos Limpadores de IA e Ecossistemas Modernos (Fase 3)**: Expansão para 13 ecossistemas: Claude Code, VS Code / Cursor, Ollama, HuggingFace, pnpm store, Bun cache, Next.js (`.next/cache`), Xcode DerivedData, simuladores iOS e cache do Homebrew, com registro centralizado via `engine.RegisterDefaultCleaners`.
- **Recursos Inspirados no Mole — purge e whitelist (Fase 4)**: Subcomando `devclean purge` com análise de inatividade Git e proteção contra dirty tree; e subcomando `devclean whitelist` para gestão de exclusões no YAML com garantia inviolável da Blacklist do Sistema.
- **Identidade Visual e Métricas Avançadas (Fase 4)**: Relatório pós-limpeza Lip Gloss com delta de disco Statfs (zero dependências CGO), mini-barras proporcionais por ecossistema e suporte a `--format json`.

---

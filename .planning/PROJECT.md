# DevClean

## Visão Geral
O DevClean é uma CLI e TUI em Go puro (Go 1.25+, `CGO_ENABLED=0`) projetada para recuperar espaço em disco em ambientes de desenvolvimento poliglotas. Realiza varreduras inteligentes e limpeza de caches, artefatos de build e dependências órfãs em 22 ecossistemas (incluindo Docker, OrbStack, Go, Node.js, .NET, Python, Rust, Java, C/C++, Flutter, Terraform, CocoaPods, Elixir, shell/CLI, Xcode, 8 editores AI/Electron, 6 navegadores web, caches/logs do macOS e sobras de aplicativos), com garantias estritas de segurança, lixeira reversível, políticas `ReviewOnly` e integração com IA via MCP (Model Context Protocol).

## Arquitetura e Estrutura do Código
- **CLI (`cmd/`)**: Baseado em Cobra. Comandos principais:
  - `devclean` (padrão abre TUI interativo)
  - `devclean scan` (varredura e exibição tabular/json com suporte a `--smart` e detecção de presença em disco)
  - `devclean clean` (remoção via TTL `--auto` ou total `--force`, com flags `--trash` e `--smart`, interceptada pelo `deleter.Coordinator`)
  - `devclean purge` (expurgo de artefatos de build em repositórios inativos ou mergeados)
  - `devclean whitelist` (gestão de exclusões e padrões protegidos)
  - `devclean trash` (listagem e restauração de artefatos enviados à lixeira)
  - `devclean schedule` / `unschedule` (gestão nativa de tarefas via Launchd no macOS e Cron/Systemd Timers no Linux)
  - `devclean watch` (monitoramento em segundo plano com notificações desktop via `beeep`)
  - `devclean mcp` (servidor Model Context Protocol com ferramentas para agentes de IA)
  - `devclean history`, `why`, `stats`, `version`, `init`, `installer`, `analyze`, `config`
- **Motor de Execução (`internal/engine/`)**: Coordenador de varredura concorrente com `errgroup`, streaming via canal, resolução canônica de propriedade (`ownership.go`) e filtro de montagens de rede.
- **Pipeline de Deleção Seguro (`internal/deleter/`)**: `deleter.Coordinator` centraliza o Estágio 1 de interceptação inviolável contra alvos `ReviewOnly` e mutações em modo `DryRun`, com suporte a bypass de lixeira para alvos virtuais (`docker://`).
- **Limpadores (`internal/cleaner/`)**: 22 módulos independentes por ecossistema implementando a interface `domain.Cleaner`, com Process Guard fail-closed e Symlink Guard.
- **Inteligência Heurística (`internal/intelligence/`)**: Cálculo ponderado de risco/segurança (`SmartScore` 0.0 - 1.0) combinando tamanho, tempo desde último acesso, atividade no Git (`GitActivityScore`) e estado de branches mergeadas.
- **Lixeira Reversível (`internal/trash/`)**: Isolamento de arquivos movidos em lotes com UUID, manifesto JSON com hash e metadados, auto-expiração e restauração atômica.
- **Camada de Segurança (`internal/safety/`)**: Blacklist do sistema imutável inviolável (`.git`, `.env`, `.ssh`, `secrets.json`), proteção contra travessia de links simbólicos e detecção de arquivos de rede.
- **Interface TUI (`internal/tui/`)**: Interface rica em terminal construída com Bubble Tea e Lip Gloss para varredura, limpeza, análise de disco (`analyze`), purge e configuração interativa (`config`).
- **Integração MCP (`internal/mcp/`)**: Servidor MCP expondo ferramentas de varredura, proposta de deleção aprovada, estatísticas e restauração para agentes de IA com marcação explícita de `ReviewOnly`.

## Histórico de Milestones
- **v1.0 (Fundação)**: Motor de varredura multi-ecossistema básico, CLI Cobra, interface TUI inicial, suporte a Go, Node, Docker, .NET, Python.
- **v2.0 (Inteligência, Segurança e Automação)**: Adicionados Rust e Java, pontuação inteligente heurística (Git, access time, branch status), lixeira reversível (`internal/trash`), integração com MCP Server, scheduler nativo (Launchd/Cron), watcher de disco com alertas desktop e perfis de configuração.
- **v2.1 (Robustez, Qualidade Completa e Expansão Inspirada no Mole)**: Pipeline de deleção unificado (`deleter.Coordinator`) com bypass seguro para alvos virtuais (Docker); cobertura global elevada para 89.1% com suíte E2E criptográfica (SHA-256) e testes headless Bubble Tea (96.1%); expansão para 13 ecossistemas (Claude Code, VS Code/Cursor, Ollama, HuggingFace, pnpm, Bun, Next.js, Xcode, Homebrew); novos subcomandos `purge` e `whitelist`; e interface pós-limpeza Lip Gloss com delta de disco Statfs (0 CGO).
- **v2.2 (Análise Visual, Instaladores, Expansão de Ecossistemas, Purge TUI e Scheduler Avançado)**: Expansão para 18 ecossistemas (C/C++, Flutter, Terraform, CocoaPods, Elixir); presets safe/standard/aggressive; detector de instaladores órfãos (`devclean installer`); auto-expiração e quota FIFO na lixeira; scheduler avançado com Systemd User Timers e log rotation; TUI em tela cheia para análise de disco (`devclean analyze`) e purge interativo; e gerenciador interativo de configurações (`devclean config`) com mutação de AST YAML.
- **v2.3 (Paridade Total com Mole e Inventário Explicável)**: Desacoplamento Scan/TTL, descoberta por presença em disco sem depender de `$PATH`, resolução de propriedade canônica contra dupla contagem; política central `ReviewOnly` com símbolo `⊙` e garantia `DryRun`; suporte a ferramentas de desenvolvimento (npm, npx, corepack, pnpm, Puppeteer, Playwright, Cypress, mise, gh, pre-commit, omz, zcompdump, Xcode/CoreSimulator com Process Guard); 8 editores AI/Electron preservando bancos `state.vscdb.backup` e versão ativa do Claude CLI; métricas Docker/OrbStack com bloqueio de lojas compartilhadas; limpador macOS (caches, logs, daemons Apple, TCC graceful degradation); 6 navegadores com escudo de sessão inviolável e ScriptCache gated; catalogação de leftovers e LaunchAgents; e suíte de aceitação no macOS com recibos duplos (`MACOS-ACCEPTANCE.json` e `.md`).

---

## Core Value
O DevClean deve revelar espaço recuperável com precisão e impedir que inventário amplo seja confundido com autorização para apagar dados compartilhados, ativos ou de sessão.

## Requirements

### Validated (v2.3)
- ✓ **SCAN-01**: `Scan()` reporta alvos e bytes ocupados sem aplicar `MaxAgeDays` ao mtime raiz — v2.3
- ✓ **SCAN-02**: Descoberta por presença em disco mesmo sem executáveis no `PATH` — v2.3
- ✓ **SCAN-03**: Descoberta determinística de raízes `$HOME` quando `ScanPaths` estiver vazio — v2.3
- ✓ **SCAN-04**: Propriedade canônica de caminhos prevenindo contagem duplicada — v2.3
- ✓ **POL-01**: Alvos com metadados `ReviewOnly` e razões explicativas — v2.3
- ✓ **POL-02**: `deleter.Coordinator` bloqueia alvos `ReviewOnly` no Estágio 1 — v2.3
- ✓ **POL-03**: CLI, TUI e MCP exibem o marcador `⊙`, bytes ocupados, bytes elegíveis e razão de recusa — v2.3
- ✓ **POL-04**: DryRun não realiza mutações no cleaner, lixeira, hooks, scheduler ou histórico — v2.3
- ✓ **DEV-01**: Descoberta por disco de caches npm, npx, corepack, pnpm, Puppeteer, Playwright e Cypress — v2.3
- ✓ **DEV-02**: Caches mise, gh, pre-commit, omz e zcompdump sem alterar histórico/configuração do shell — v2.3
- ✓ **DEV-03**: CoreSimulator, DeviceSupport e Xcode Archives respeitam processo ativo/desconhecido — v2.3
- ✓ **AI-01**: Caches Chromium/Electron em 8 editores (VS Code, Insiders, Cursor, Antigravity, Windsurf, Claude Desktop, OpenCode, Zed) — v2.3
- ✓ **AI-02**: Workspace storage seleciona apenas `state.vscdb.backup` mantendo bancos primários invioláveis — v2.3
- ✓ **AI-03**: Limpeza de versões do Claude CLI preserva symlink/versão ativa com fail-closed — v2.3
- ✓ **STORE-01**: Docker reporta métricas completas (total, ativo, tamanho, recuperável) e JSON detalhado — v2.3
- ✓ **STORE-02**: Caches regeneráveis do OrbStack separados de storage de VM/dados — v2.3
- ✓ **STORE-03**: Docker storage, dados do OrbStack, pacotes NuGet e pnpm stores são permanentemente `ReviewOnly` — v2.3
- ✓ **MAC-01**: Caches/logs de usuário e lixeira inventariados (Trash requer flag explícita) — v2.3
- ✓ **MAC-02**: Daemons Apple, Group Containers e Handoff com allowlists positivas — v2.3
- ✓ **MAC-03**: Application Support restrito a folhas regeneráveis e Spotify PersistentCache — v2.3
- ✓ **MAC-04**: Degradação graciosa com `Partial: true` em logs e relatórios perante negação TCC — v2.3
- ✓ **MAC-05**: Process Guard, whitelist imutável e Symlink Guard protegem apps ativos e dados sensíveis — v2.3
- ✓ **BROW-01**: Detecção de 6 navegadores (Chrome, Brave, Arc, Edge, Firefox, Safari) por disco — v2.3
- ✓ **BROW-02**: Apenas Service Worker, shaders e caches de rede entram no escopo de limpeza — v2.3
- ✓ **BROW-03**: Cookies, History, Local Storage, IndexedDB, Login Data e Sessions estritamente excluídos — v2.3
- ✓ **BROW-04**: ScriptCache inventariado como `ReviewOnly` até comprovação de aceitação — v2.3
- ✓ **LEFT-01**: Leitura de LaunchAgents de usuário e sistema a partir de plists XML ou binários — v2.3
- ✓ **LEFT-02**: Apenas referências absolutas a binários ausentes classificadas como obsoletas; o restante permanece ReviewOnly — v2.3
- ✓ **LEFT-03**: Candidatos a sobra de aplicativo cruzam bundles instalados com Application Support, Caches e Containers — v2.3
- ✓ **LEFT-04**: Sobras de apps e LaunchAgents obsoletos nunca são removidos automaticamente — v2.3
- ✓ **VAL-01**: Testes herméticos `t.TempDir()` com sentinelas, permissões, symlinks e asserções DryRun — v2.3
- ✓ **VAL-02**: `go test ./...` aprovado, cobertura >= 85% em pacotes alterados e builds estáticos zero-CGO — v2.3
- ✓ **VAL-03**: Aceitação física no macOS contra referência com recibos duplos de inventário e diagnósticos — v2.3

### Active
(Nenhum requisito ativo no momento — pronto para especificação do próximo milestone via `/gsd-new-milestone`.)

### Out of Scope
- Limpeza destrutiva de Cookies, History, Local Storage, IndexedDB, Login Data e Sessions de navegadores.
- Remoção automática de lojas compartilhadas, dados de VM, LaunchAgents órfãos ou sobras inferidas de apps.
- Deleção completa de perfis de usuário em navegadores ou Application Support.
- Integração com GUI nativa Cocoa/Swift ou daemons de background contínuo além do watcher existente.

---

## Estado Atual (Pós v2.3)
- **Base de Código**: ~72.300 linhas de Go 1.25 puro (`CGO_ENABLED=0`).
- **Qualidade e Cobertura**: >89% de cobertura global de testes automatizados (>85% em todos os pacotes alterados).
- **Ecossistemas Suportados (22)**: Docker, Go, Node, .NET, Python, Rust, Java, IA (Claude Code, Ollama, HuggingFace), Editores (VS Code, Cursor, Antigravity, Windsurf, Claude Desktop, OpenCode, Zed), Web moderna (pnpm, Bun, Next.js), Mobile & DevOps (Xcode, Homebrew, CocoaPods, Flutter, C/C++, Terraform, Elixir), Shell & CLI Tools (mise, gh, pre-commit, omz, zcompdump), macOS System/User (caches, logs, daemons Apple), 6 Navegadores Web (Chrome, Brave, Arc, Edge, Firefox, Safari), App Leftovers e Stale LaunchAgents.
- **Segurança**: Blacklist imutável inviolável, Estágio 1 `ReviewOnly` no `deleter.Coordinator`, Process Guard fail-closed, Symlink Guard, degradação graciosa TCC e inviolabilidade de sessões.
- **Ferramentas de Linha de Comando**: `scan`, `clean`, `purge`, `whitelist`, `trash`, `schedule`, `unschedule`, `watch`, `mcp`, `history`, `why`, `stats`, `init`, `version`, `installer`, `analyze`, `config`.

## Key Decisions

| Decisão | Racional / Impacto | Resultado |
|---|---|:---:|
| **Desacoplamento Scan vs TTL** | `Scan()` expõe inventário e bytes totais independentemente de mtime; TTL governa apenas elegibilidade de limpeza. | ✓ Good |
| **Estágio 1 ReviewOnly no Coordinator** | Alvos `ReviewOnly` são rejeitados no limiar do `deleter.Coordinator` antes de qualquer hook ou sink destrutivo. | ✓ Good |
| **Garantia Absoluta de DryRun** | `DryRun` não executa nenhuma mutação em limpadores, lixeira, hooks, agendador ou histórico. | ✓ Good |
| **Propriedade Canônica de Caminhos** | Exclusões proativas e resolvedor de propriedade evitam contagem dupla de bytes e duplicação de alvos. | ✓ Good |
| **Lojas Compartilhadas como ReviewOnly** | Docker storage, dados do OrbStack, pacotes NuGet e pnpm store são alvos permanentes `ReviewOnly`. | ✓ Good |
| **Process Guard Fail-Closed** | Limpeza de ferramentas ativas (Xcode, simuladores, daemons Apple, navegadores) é bloqueada se o processo estiver ativo ou em estado desconhecido. | ✓ Good |
| **Escudo Inviolável de Navegadores** | Cookies, History, Local Storage, IndexedDB, Login Data e Sessions são formalmente blindados contra qualquer deleção. | ✓ Good |
| **ScriptCache Gated por Evidência** | Chromium/Electron ScriptCache permanece `ReviewOnly` até comprovação formal de aceitação e integridade de extensões MV3. | ✓ Good |
| **Classificação Estrita de LaunchAgents/Leftovers** | Apenas binários ausentes com caminhos absolutos viram candidatos obsoletos; o restante é catalogado como ReviewOnly sem deleção automática. | ✓ Good |
| **Pure Go Zero-CGO** | Compilação estática universal para Darwin e Linux (ARM64 e AMD64) sem dependências externas em tempo de execução. | ✓ Good |

## Diretrizes e Restrições de Engenharia
- **Zero Dependências em Tempo de Execução**: Manter `CGO_ENABLED=0` para compilação estática cross-platform (Darwin ARM64/AMD64 e Linux AMD64/ARM64).
- **Inviolabilidade da Segurança**: A lista negra imutável e a interceptação de `ReviewOnly` jamais podem ser contornadas por nenhuma flag, cleaner ou chamada externa.
- **Compatibilidade Multiplataforma**: Código deve compilar e operar hermeticamente em macOS e Linux.
- **Meta de Qualidade**: Cobertura de testes mantida superior a 85% em novos incrementos.

---
*Last updated: 2026-10-19 after v2.3 milestone*

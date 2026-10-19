# Project Retrospective

*A living document updated after each milestone. Lessons feed forward into future planning.*

## Milestone: v2.1 — Robustez, Qualidade Completa e Expansão Inspirada no Mole

**Shipped:** 2026-10-11  
**Phases:** 4 | **Plans:** 14 | **Tasks:** 32  

### What Was Built
- Unificação do pipeline de deleção com `deleter.Coordinator` para CLI, TUI e MCP Server com bypass seguro para alvos virtuais (`docker://`).
- Cobertura global de testes automatizados elevada para 89.1% com suíte E2E criptográfica (SHA-256) e testes headless Bubble Tea (96.1%).
- Expansão de ecossistemas para 13 cleaners: IA/Editores (Claude Code, VS Code, Cursor, Ollama, HuggingFace), Web moderna (pnpm, Bun, Next.js) e Mobile/DevOps (Xcode DerivedData, simuladores iOS, Homebrew cache).
- Subcomando `devclean purge` para expurgo em lote com inteligência de inatividade Git e proteção contra dirty tree.
- Subcomando `devclean whitelist` para gestão interativa de exclusões preservando a lista negra do sistema.
- Interface visual inspirada no Mole com medição Statfs sem CGO e suporte a `--format json`.

### What Worked
- **Execução em ondas com TDD e verificação contínua**: Escrever testes antes/junto com a implementação permitiu atingir e superar a meta de 85% de cobertura (89.1%) sem quebrar regressões.
- **Isolamento de Alvos Virtuais no Domínio**: A separação entre filesystem real e alvos virtuais resolveu de forma limpa o problema de I/O na lixeira para Docker e caches abstratos.
- **AST YAML para Edição de Configuração**: Modificar o `config.yaml` através da árvore sintática do YAML preservou comentários, estrutura e ordem das chaves perfeitamente.

### What Was Inefficient
- **Validação de permissões e links simbólicos em testes**: Requeru cuidados adicionais para garantir que diretórios com permissão 0555 pudessem ser limpos hermeticamente após o teste sem deixar resíduos no disco.

### Patterns Established
- `deleter.Coordinator` como ponto de entrada universal para qualquer operação destrutiva no projeto.
- Testes headless com Bubble Tea enviando sequências de mensagens virtuais para testar TUI sem terminal interativo.
- Statfs multiplataforma via `golang.org/x/sys/unix` sem CGO para consultar espaço em disco livre/usado.

### Key Lessons
1. A inviolabilidade da segurança deve ser garantida no nível de domínio/deleter (`checkImmutableBlock`), e não apenas na interface de usuário.
2. Contratos de teste determinísticos (mock de `CommandExecutor`) evitam que a suíte dependa de utilitários externos como launchctl ou crontab.

---

## Milestone: v2.2 — Análise Visual, Instaladores, Expansão de Ecossistemas, Purge TUI e Scheduler Avançado

**Shipped:** 2026-10-14  
**Phases:** 5 | **Plans:** 19 | **Tasks:** 50  

### What Was Built
- Expansão do motor de 13 para 18 ecossistemas de desenvolvimento com 5 novos limpadores (C/C++, Flutter/Dart, Terraform/OpenTofu, CocoaPods, Elixir/Mix) e presets rápidos de limpeza (`--preset safe|standard|aggressive`).
- Subcomando `devclean installer` com scanner profundo de instaladores esquecidos (`.dmg`, `.pkg`, `.iso`, etc.) e auto-expiração por tempo de retenção na lixeira com gestão FIFO de quota máxima.
- Subsistema de agendamento moderno com Systemd User Timers (Linux) e Launchd (macOS), intervalos intuitivos (`--weekly`, `--hourly`, `--at`), rotação circular de logs (10 MB) e comando de auditoria `devclean schedule logs`.
- Navegador visual de disco em tela cheia `devclean analyze` via Bubble Tea com barras Lip Gloss proporcionais, e subcomando `devclean purge` interativo com agrupamento por repositórios Git, busca fuzzy e trava inviolável de blacklist.
- Gestão interativa de configuração via TUI completa de 5 abas (`devclean config`) com persistência cirúrgica em AST YAML preservando comentários e sincronização automática com o scheduler e watcher do sistema operacional.

### What Worked
- **Isolamento de Mutação AST YAML**: Manter comentários e formatação original intactos permitiu uma interface rica de configuração TUI sem risco de corrupção ou perda de contexto nos arquivos de configuração do usuário.
- **TDD Rigoroso com Suíte Headless**: Testar modelos complexos Bubble Tea de forma headless (`tea.NewProgram` com buffers de IO) permitiu cobrir fluxos interativos de modal, busca fuzzy e navegação vim com >88% de cobertura sem necessitar de display físico.
- **Arquitetura Zero-CGO Mantida**: Mesmo com cálculos estatísticos de disco avançados, rotação de logs e integração profunda de SO (systemd/launchd), o binário manteve portabilidade hermética e zero CGO.

### What Was Inefficient
- **Gerenciamento de Locks Concorrentes no Terraform**: Necessidade de validação cuidadosa contra `.terraform.tfstate.lock.info` para evitar colisão com execuções ativas do Terraform/OpenTofu.
- **Tratamento de Árvores Read-Only no C/C++**: Exigiu implementar utilitário de `ChmodRecursive` preventivo para contornar limitações de permissão `0555` / `0444` comuns em pacotes Conan e vcpkg.

### Patterns Established
- `internal/config/ast.go` como padrão de ouro para manipulações sintáticas cirúrgicas de configuração YAML.
- `ChmodRecursive` preventivo como padrão de segurança para remoção de diretórios protegidos contra escrita por geradores de build.
- Detecção e chaveamento dinâmico de TTY para fornecer fallback transparente e sem travamento para pipelines não interativos ou scripts de CI.

### Key Lessons
1. A separação entre o motor de inspeção de arquivos compactados e o extrator de disco previne consumo de I/O desnecessário na detecção de instaladores.
2. Sincronização explícita e reativa com daemons de SO (systemd/launchd) deve ocorrer após qualquer mutação persistida de agendamento na UI.

---

## Milestone: v2.3 — Paridade Total com Mole (tw93/mole)

**Shipped:** 2026-10-19  
**Phases:** 9 | **Plans:** 32 | **Tasks:** 64  

### What Was Built
- Desacoplamento de varredura e elegibilidade por TTL, descoberta por disco sem dependência de PATH e propriedade canônica de caminhos prevenindo contagem dupla (Fase 6).
- Política `ReviewOnly` com símbolo `⊙`, interceptação no Estágio 1 do `deleter.Coordinator` e garantia de mutação zero (`DryRun`) em CLI, TUI, MCP e Scheduler (Fase 7).
- Expansão de ferramentas de desenvolvedor (npm, npx, corepack, pnpm cache, Puppeteer, Playwright, Cypress), utilitários de shell (mise, gh, pre-commit, omz, zcompdump) e alvos Xcode com Process Guard fail-closed (Fase 8).
- Limpeza cirúrgica de caches em 8 editores AI/Electron preservando bancos `state.vscdb.backup` e preservação fail-closed de versões ativas do Claude CLI (Fase 9).
- Métricas detalhadas de provedores Docker/OrbStack com bloqueio permanente `ReviewOnly` para lojas de pacotes compartilhados (NuGet, pnpm store, Docker data) (Fase 10).
- Limpador macOS com allowlists de caches/logs, daemons Apple, opção explícita de Trash, degradação graciosa com `Partial: true` sob negação TCC, Process e Symlink Guards (Fase 11).
- Limpador de 6 navegadores web com escudo inviolável contra deleção de sessões, histórico, senhas e cookies, e ScriptCache gated como `ReviewOnly` (Fase 12).
- Parser puro Go de plists XML/binários, classificador de LaunchAgents obsoletos vs ambíguos, indexador de bundles e catalogação de sobras sem deleção automática (Fase 13).
- Bateria de testes de regressão multi-canal, cobertura superior a 85% em todos os pacotes alterados, compilação estática zero-CGO e suite de aceitação no macOS contra referência do Mole com recibos duplos (Fase 14).

### What Worked
- **Modelagem de Domínio com ReviewOnly no Estágio 1**: Centralizar o bloqueio de alvos protegidos e lojas compartilhadas no `deleter.Coordinator` garantiu imunidade automática em todos os canais de entrada (CLI, TUI, MCP, Scheduler) sem duplicação de regras.
- **Deduplicação Proativa e Propriedade Canônica**: Resolver a propriedade dos caminhos no motor (`internal/engine/ownership.go`) eliminou riscos de contagem dupla de bytes entre limpadores gerais e limpadores de ecossistemas específicos.
- **Process Guard Injetável e Testes Herméticos**: O uso de abstrações determinísticas (`CommandExecutor`) permitiu testar cenários de corrida, bloqueio de processos ativos e permissões do SO em `t.TempDir()` com zero flakiness.
- **Portabilidade Estática sem CGO**: Toda a leitura de plists, cálculo de Statfs e inspeção de processos foi implementada em Go puro, garantindo compilação cruzada instantânea para Darwin e Linux em ARM64 e AMD64.

### What Was Inefficient
- **Validação de Permissões TCC em Ambientes Não Interativos**: Exigiu implementar padrões de degradação graciosa (`Partial: true`) para que a suíte de testes e o scanner não falhassem em ambientes com TCC restrito (como CI ou terminais sem Full Disk Access).
- **Ambiguidade em LaunchAgents**: A proliferação de wrappers de shell e caminhos relativos em plists de terceiros exigiu refinamentos contínuos para manter a classificação estrita como `ReviewOnly` e evitar falsos positivos.

### Patterns Established
- `ReviewOnly` e `ReviewReason` como propriedades de primeira classe no `domain.Target`.
- Escudo inviolável de sessão (`session_shield.go`) para navegadores e editores, impedindo qualquer mutação em bancos de dados de estado ou credenciais.
- Recibos duplos de aceitação (`MACOS-ACCEPTANCE.json` e `.md`) para auditoria transparente de paridade e diagnósticos do host.

### Key Lessons
1. Inventário abrangente e elegibilidade de limpeza devem ser tratados como dimensões distintas no motor para permitir transparência sem comprometer a segurança.
2. Alvos complexos e lojas compartilhadas (Docker, OrbStack, NuGet, pnpm) devem operar sob a política de "apenas revisão" por padrão, exigindo intervenção explícita do operador.
3. Testes herméticos com sentinelas e verificações de `DryRun` garantem que novas expansões de limpadores nunca causem regressões em arquivos vizinhos.

### Cost Observations
- Execução orientada por planos detalhados com TDD em ondas paralelas.
- 100% de sucesso na verificação e auditoria de requisitos sem retrabalho crítico.

---

## Cross-Milestone Trends

### Process Evolution

| Milestone | Phases | Plans | Key Change |
|-----------|--------|-------|------------|
| v2.1 | 4 | 14 | GSD SDD completo com ondas paralelas, TDD e auditoria formal |
| v2.2 | 5 | 19 | Arquitetura multi-TUI Bubble Tea com AST YAML, Systemd Timers e suíte E2E autônoma |
| v2.3 | 9 | 32 | Paridade com Mole, ReviewOnly Stage 1, escudo de sessões, 22 ecossistemas e recibos macOS |

### Cumulative Quality

| Milestone | Tests | Coverage | Zero-Dep Additions |
|-----------|-------|----------|-------------------|
| v2.1 | 32 packages | 89.1% | 0 CGO (`CGO_ENABLED=0`) |
| v2.2 | 37 packages | >89% | 0 CGO (`CGO_ENABLED=0`) |
| v2.3 | 48 packages | >89% (>85% em alterados) | 0 CGO (`CGO_ENABLED=0`) |

### Top Lessons (Verified Across Milestones)

1. Centralizar validações de segurança em um único coordenador elimina vulnerabilidades de contorno entre interfaces (CLI, TUI, MCP).
2. Manter zero dependências CGO assegura builds estáticos rápidos e máxima portabilidade em macOS e Linux.
3. Testes headless Bubble Tea garantem confiabilidade de UIs de terminal sem fragilidade visual.


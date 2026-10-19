# Phase 6: Scan, TTL, Identity & Disk Discovery - Context

**Gathered:** 2026-10-14  
**Status:** Ready for planning  

<domain>
## Phase Boundary

Garantir que o inventário de varredura do DevClean seja 100% completo, determinístico e deduplicado antes de qualquer avaliação de elegibilidade para expurgo:
1. `Scan()` reporta todo alvo existente e seus bytes reais ocupados sem filtrar por mtime da pasta raiz (`SCAN-01`).
2. Cleaners reportam `IsInstalled() == true` quando seus alvos ou diretórios de cache existem em disco, mesmo que o executável não esteja no `PATH` (`SCAN-02`).
3. Quando `cfg.ScanPaths` estiver vazio, descobrir diretórios canônicos de projetos em `$HOME` (`Documents`, `Projects`, `Developer`, `Code`, `Work`, `repos`, `workspace`) em ordem determinística (`SCAN-03`).
4. Propriedade canônica de caminhos e resolução de sobreposição impedem dupla contagem de bytes e alvos duplicados (`SCAN-04`).

</domain>

<decisions>
## Implementation Decisions

### Deduplicação e Propriedade Canônica (SCAN-04)
- **D-01:** **Mais Específico Vence:** Cleaners específicos/dedicados (ex: Node, Bun, Python) possuem precedência estrita sobre cleaners genéricos de sistema/usuário (ex: `macos/User app caches`). Caminhos subordinados/filhos pertencem ao cleaner dedicado, e seus bytes são deduzidos do cleaner pai genérico para eliminar contagem dupla. — **Reversibility:** costly — afeta a ordenação e a consolidação de métricas no Engine.
- **D-02:** **Resolução Canônica de Symlinks:** Resolver symlinks usando `filepath.EvalSymlinks` antes do scan e indexar pelo caminho físico real, evitando que links simbólicos ou árvores aninhadas provoquem varredura duplicada do mesmo inode. — **Reversibility:** reversible — isolado na normalização de caminhos.
- **D-03:** **Centralização no Engine:** A lógica de deduplicação e atribuição canônica de posse reside centralmente no `internal/engine` como um estágio de resolução após a coleta/streaming dos cleaners, mantendo os cleaners desacoplados entre si. — **Reversibility:** costly — define o contrato de processamento de alvos no motor.
- **D-04:** **Omissão de Alvos Subordinados:** Alvos sobrepostos absorvidos por um cleaner mais específico são omitidos da listagem visual/relatórios, mantendo registro interno para diagnóstico. — **Reversibility:** reversible — regra de exibição e filtragem.

### Desacoplamento Scan vs. TTL e Modelagem no Target (SCAN-01)
- **D-05:** **Modelagem Explícita em domain.Target:** Adicionar campos explícitos em `domain.Target`: `EligibleBytes int64` e `TTLEligible bool`, mantendo `SizeBytes int64` como o tamanho físico total real no disco reportado pelo Scan. — **Reversibility:** one-way — altera o struct central `domain.Target` consumido pela CLI, TUI, MCP, Coordenador e cleaners.
- **D-06:** **Semântica de MaxAgeDays == 0:** Quando `MaxAgeDays == 0` (ou modo de limpeza total/forçada), `EligibleBytes` equivale a 100% de `SizeBytes` e `TTLEligible = true`. — **Reversibility:** reversible — lógica pura de cálculo.
- **D-07:** **Inocuidade do Mtime da Raiz:** O `mtime` do diretório raiz jamais oculta ou descarta um alvo durante `Scan()`. Cleaners inspecionam subpastas/arquivos para calcular `EligibleBytes` ou utilizam a data do item interno relevante. — **Reversibility:** costly — requer remoção das checagens antigas de cutoff de raiz em todos os cleaners existentes.
- **D-08:** **Granularidade de Expurgo:** Alvos atômicos de projetos (`node_modules`, `target`, `dist`) são expurgados por inteiro quando elegíveis; caches globais agregadores podem podar entradas antigas internamente caso o cleaner forneça suporte. — **Reversibility:** reversible.

### Descoberta Automática de ScanPaths (SCAN-03)
- **D-09:** **Ordem Canônica Fixa:** Seguir estritamente a ordem determinística definida na especificação (`Documents`, `Projects`, `Developer`, `Code`, `Work`, `repos`, `workspace`), incluindo apenas os diretórios que existirem fisicamente sob `$HOME`. — **Reversibility:** reversible.
- **D-10:** **Fallback Vazio Seguro:** Se nenhum dos 7 diretórios canônicos existir sob `$HOME`, `ScanPaths` permanece vazio, restringindo a varredura aos caches globais e evitando travessia recursiva irrestrita no diretório do usuário. — **Reversibility:** reversible.
- **D-11:** **Resolução Dinâmica em Memória:** Implementar método de resolução dinâmica (ex: `cfg.EffectiveScanPaths()`), garantindo que a descoberta automática opere em tempo de execução sem sujar nem persistir caminhos no arquivo YAML de configuração. — **Reversibility:** reversible.
- **D-12:** **Precedência de Configuração Explícita:** Se o usuário definir `scan_paths` explicitamente na configuração ou via flag, a auto-descoberta é desativada e apenas os caminhos fornecidos são varridos. — **Reversibility:** reversible.

### Detecção de Presença em Disco para IsInstalled (SCAN-02)
- **D-13:** **Detecção Híbrida (OU):** `IsInstalled()` retorna `true` caso `exec.LookPath(binary) == nil || fs.DirExists(defaultCacheDir)`. Se arquivos de cache órfãos existirem em disco, o cleaner correspondente é ativado para inventário e liberação de espaço. — **Reversibility:** costly — atualiza os métodos `IsInstalled()` de todos os cleaners.
- **D-14:** **Execução de Limpeza por Natureza:** Cleaners que operam diretamente no sistema de arquivos removem arquivos de cache órfãos independentemente da presença do binário; cleaners dependentes de comandos CLI (ex: Docker, Homebrew) reportam erro explicativo caso acionados para limpeza sem o binário no PATH. — **Reversibility:** reversible.
- **D-15:** **Ativação por Projetos em ScanPaths:** Para ecossistemas de linguagens (Node, Rust, Go, Java, Elixir, etc.), a presença de manifests de projeto nos `ScanPaths` ativa o cleaner para varrer artefatos locais mesmo sem a CLI do compilador/gerenciador no PATH. — **Reversibility:** reversible.
- **D-16:** **Checagens Rasas e Instantâneas:** `IsInstalled()` executa apenas verificações `os.Stat` superficiais nas raízes conhecidas de cache e diretórios imediatos de `ScanPaths`, sem travessia recursiva, mantendo a inicialização do motor imediata. — **Reversibility:** reversible.

### the agent's Discretion
- Formato de armazenamento interno do índice de caminhos canônicos no `Engine` (ex: prefix-tree / trie ou map de caminhos normalizados com `filepath.Clean`).
- Nomenclatura auxiliar de helpers em `internal/config` e `internal/engine`.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Especificação de Paridade e Requisitos
- `.planning/SPEC-v2.3-MOLE-PARITY.md` — Especificação técnica oficial do milestone v2.3; Frente 1 (linhas 52-58) e Critérios de Aceite.
- `.planning/REQUIREMENTS.md` — Requisitos rastreáveis SCAN-01 a SCAN-04.
- `.planning/ROADMAP.md` — Definição de escopo, dependências e critérios de sucesso da Fase 6.
- `.planning/PROJECT.md` — Visão geral da arquitetura, diretrizes de segurança e restrições CGO_ENABLED=0.

### Código Existente Relevante
- `internal/domain/types.go` — Definições de `domain.Target`, `domain.Cleaner`, `domain.EcosystemRules` e `domain.ScanResult`.
- `internal/engine/engine.go` — Orquestração de scanners, streaming via canal e agregação de resultados.
- `internal/engine/register.go` — Registro central dos 18 cleaners existentes.
- `internal/config/config.go` — Estrutura de configuração, campos `ScanPaths` e defaults.
- `internal/config/defaults.go` — Configurações padrão e valores iniciais de ecossistemas.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/fs`: Funções utilitárias de filesystem seguras (`DirExists`, `IsNetworkMount`, `ExpandPath`, cálculo de diretórios).
- `internal/platform.Resolver`: Resolução multiplataforma de caminhos padrão de cache e ferramentas em macOS e Linux.
- `internal/safety.Safety`: Validação inviolável de caminhos contra a blacklist do sistema e travessia de symlinks.

### Established Patterns
- Concorrência estruturada via `errgroup` com streaming de `ScanResult` em `internal/engine/engine.go`.
- Separação estrita entre definição de interfaces (`domain.Cleaner`) e implementação de cada ecossistema (`internal/cleaner/*`).
- Compilação estática pura sem CGO (`CGO_ENABLED=0`) garantida em todos os pacotes.

### Integration Points
- `internal/domain/types.go`: Adição de `EligibleBytes int64` e `TTLEligible bool` em `domain.Target`.
- `internal/engine/engine.go`: Inclusão do pipeline pós-scan de resolução de caminhos canônicos e deduplicação de bytes.
- `internal/config/config.go`: Adição de `EffectiveScanPaths()` para auto-descoberta determinística das 7 pastas no `$HOME`.
- `internal/cleaner/*`: Atualização de `Scan()` para remoção do filtro de mtime na raiz e de `IsInstalled()` para inclusão de verificação de diretórios de cache em disco.

</code_context>

<specifics>
## Specific Ideas

- Paridade com Mole (tw93/mole): reportar todo o espaço em disco ocupado por caches de ferramentas antigas ou desinstaladas, sem que executáveis ausentes no PATH ocultem gigabytes de caches residuais.
- Relatórios transparentes: a CLI e a TUI devem ser capazes de informar tanto o volume real ocupado no disco quanto o volume elegível para expurgo de acordo com a política de TTL.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 6-Scan, TTL, Identity & Disk Discovery*  
*Context gathered: 2026-10-14*  

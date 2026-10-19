# Phase 7: ReviewOnly & Coordinator Policy - Context

**Gathered:** 2026-10-15  
**Status:** Ready for planning  

<domain>
## Phase Boundary

Garantir que os usuários possam distinguir claramente inventário de autorização para expurgo, e que todo caminho de mutação recuse alvos `ReviewOnly` e efeitos colaterais em dry-run na fronteira central do `deleter.Coordinator`:
1. Alvos expõem metadados `ReviewOnly bool` e `ReviewReason string`, com `EligibleBytes = 0` e `TTLEligible = false` (`POL-01`).
2. O `deleter.Coordinator` bloqueia alvos `ReviewOnly` em execuções automáticas, forçadas, agendadas, MCP, TUI e chamadas diretas no Estágio 1 antes de hooks ou sinks de exclusão (`POL-02`).
3. As saídas da CLI, TUI e MCP exibem o marcador `⊙`, bytes ocupados, bytes elegíveis e a razão da recusa/revisão; na TUI, alvos de revisão recebem o indicador travado `[⊙]` e não podem ser selecionados para deleção (`POL-03`).
4. O modo `DryRun` garante zero mutação em cleaners, lixeira, hooks, agendador, retenção de cotas e histórico de deleção, mantendo o inventário explicável idêntico (`POL-04`).

</domain>

<decisions>
## Implementation Decisions

### Modelagem de ReviewOnly e Razões Padronizadas (POL-01)
- **D-01:** **Modelagem Plana no domain.Target:** Adicionar campos planos `ReviewOnly bool` e `ReviewReason string` diretamente no struct `domain.Target`. Simples, zero alocação adicional quando vazio, compatível com serialização JSON e seguindo o padrão de `EligibleBytes` e `TTLEligible`. — **Reversibility:** one-way — altera o struct central `domain.Target` consumido por todos os subsistemas.
- **D-02:** **Constantes Semânticas de Razão de Revisão:** Padronizar constantes em `domain` para as categorias de revisão estabelecidas: `ReviewReasonSharedStore` (stores e caches compartilhados como pnpm store, NuGet, Docker storage, OrbStack data), `ReviewReasonActiveProcess` (ferramenta ativa ou processo em execução como Xcode/Simulator), `ReviewReasonOrphanLeftover` (resíduos órfãos e LaunchAgents obsoletos), `ReviewReasonSystemProtected` (caminhos sensíveis ou de allowlist de sistema) e `ReviewReasonSessionSensitive` (ScriptCache e dados de sessão sensíveis). — **Reversibility:** costly — define os identificadores consumidos por cleaners futuros (Fases 8–13).
- **D-03:** **Travamento Estrito de Expurgo:** Alvos `ReviewOnly` são incondicionalmente não-deletáveis pelos comandos gerais de mutação (`clean --force`, `purge`, `auto`, MCP, scheduler). Não há flag de override no DevClean; a limpeza desses itens deve ocorrer por ações manuais do usuário ou ferramentas nativas dedicadas (ex: `docker system prune`, `orb clean`). — **Reversibility:** costly — estabelece a fronteira inviolável de segurança contra exclusão acidental.
- **D-04:** **Semântica de Bytes Elegíveis:** Para qualquer alvo com `ReviewOnly = true`, `EligibleBytes` é obrigatoriamente `0` e `TTLEligible = false`, separando inequivocamente o espaço ocupado (`SizeBytes`) da autorização para expurgo automatizado. — **Reversibility:** reversible — lógica pura de cálculo.

### Política do Coordinator e Interceptação no Estágio 1 (POL-02)
- **D-05:** **Interceptação no Estágio 1 (Validação de Segurança):** O `deleter.Coordinator` remove alvos `ReviewOnly` imediatamente no Estágio 1, junto ao filtro de blacklist, antes de qualquer particionamento em filesystem/virtual, execução de pré-hooks ou acionamento de sinks de deleção (lixeira ou direto). — **Reversibility:** costly — define a ordem de execução do ciclo de vida no Coordinator.
- **D-06:** **Rastreamento Estruturado de Alvos Pulados:** Alvos `ReviewOnly` rejeitados são acumulados em `res.SkippedReviewOnly []domain.Target` no `deleter.Result`, emitindo notificação via callback `opts.OnWarning`. A operação prossegue com os demais alvos elegíveis válidos sem gerar erro fatal. — **Reversibility:** reversible.
- **D-07:** **Finalização Graciosa com Lote 100% ReviewOnly:** Se todos os alvos submetidos ao Coordinator forem `ReviewOnly` (ou protegidos pela blacklist), `Coordinator.Delete()` retorna `Result` com `TotalCount: 0, TotalFreed: 0`, a lista `SkippedReviewOnly` preenchida e `err == nil`, permitindo que CLI/TUI/MCP relatem a recusa graciosamente. — **Reversibility:** reversible.
- **D-08:** **Invariante de Portão Único:** Toda e qualquer mutação (CLI `clean`/`purge`, TUI, MCP `execute_approved_deletion`, agendador `schedule_run`) deve obrigatoriamente delegar ao `Coordinator.Delete()`. Callers podem filtrar ou exibir avisos preventivos, mas o Coordinator é o perímetro autoritativo final. — **Reversibility:** costly — arquitetura de segurança centralizada.

### Apresentação Visual na CLI, TUI & MCP (POL-03)
- **D-09:** **Marcador `⊙` Integrado na CLI:** No comando `devclean scan` (tabela), prefixar `⊙ ` no caminho do alvo; no modo verbose (`-v`), exibir `[ReviewOnly: <reason>] (eligible: 0 B)` mantendo o layout da tabela alinhado e limpo. — **Reversibility:** reversible.
- **D-10:** **Indicador Travado `[⊙]` na TUI:** Na TUI interativa (`tui/view.go`), renderizar `[⊙]` no lugar do checkbox `[ ]` para alvos `ReviewOnly`. Ações de alternância (`space`) são ignoradas exibindo mensagem na barra de status com o motivo da revisão, e a seleção em massa (`a`/`m`) exclui automaticamente esses alvos. — **Reversibility:** reversible.
- **D-11:** **Metadados Ricos no MCP Server:** Em `propose_deletion`, alvos `ReviewOnly` retornam `status: "review_only"`, com `review_reason`, `size_bytes` e `eligible_bytes: 0`. Em `scan_environment`, os campos `review_only` e `review_reason` são serializados no JSON. Em `explain_target`, a política de revisão é explicada. — **Reversibility:** reversible.
- **D-12:** **Aviso Informativo Pós-Limpeza:** Relatórios pós-limpeza (`internal/ui/report.go`) incluem aviso informativo (`⊙ N itens mantidos para revisão (X GB) — execute 'devclean scan' para detalhes`) quando houver alvos `ReviewOnly` no inventário que não foram apagados. — **Reversibility:** reversible.

### Garantias de Zero Mutação em DryRun (POL-04)
- **D-13:** **Centralização Absoluta de Efeitos Colaterais:** Toda e qualquer operação com efeito colateral — incluindo expurgo de lixeira e manutenção de cotas (`trashMgr.EnforceRetention`, `trashMgr.EnforceMaxSize`), hooks pré/pós, histórico JSONL e métricas do agendador — fica estritamente encapsulada dentro do Coordinator sob a guarda `if !opts.DryRun`. Remoção de chamadas externas de manutenção de lixeira em `cmd/clean.go`. — **Reversibility:** costly — unifica controle de estado no Coordinator.
- **D-14:** **Recusa Consistente em DryRun:** Mesmo em modo DryRun, alvos `ReviewOnly` são rejeitados para `SkippedReviewOnly`, garantindo que a simulação reflita 100% o comportamento de uma execução real. — **Reversibility:** reversible.
- **D-15:** **Auditoria Canário em Testes de Política:** Testes de invariante de mutação zero (`deleter_test.go`) utilizam arquivos canário em `t.TempDir()`, pastas de lixeira, arquivos JSONL de histórico e configurações de agendador para asseverar que nenhuma mutação em disco ocorre durante DryRun em todos os pontos de entrada. — **Reversibility:** reversible.
- **D-16:** **Resistência à Injeção de Comandos de Provedores:** Comandos externos para cleaners virtuais e do sistema devem utilizar estritamente fatias de argumentos diretas com `exec.CommandContext` (nunca interpolação de strings em shell `sh -c`), associados à validação de caminhos pelo `safety.Blacklist` e resolução canônica de symlinks para impedir injeção ou travessia. — **Reversibility:** costly — padrão de invocação de subprocessos em todos os cleaners.

### the agent's Discretion
- Formatação exata de cores/estilos lipgloss para o badge `⊙ REVIEW` na TUI e CLI.
- Nomenclatura dos métodos auxiliares de conveniência em `domain.Target` (ex: `IsReviewOnly()`).

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Especificação de Paridade e Requisitos
- `.planning/SPEC-v2.3-MOLE-PARITY.md` — Frente 1 (linhas 52-58) e Critérios de Aceite 3 e 4.
- `.planning/REQUIREMENTS.md` — Requisitos POL-01 a POL-04, STORE-03 e LEFT-04.
- `.planning/ROADMAP.md` — Definição de escopo da Fase 7 e critérios de sucesso.
- `.planning/PROJECT.md` — Diretrizes de arquitetura, segurança e zero mutação em dry-run.

### Código Existente Relevante
- `internal/domain/types.go` — Definições de `domain.Target`, `RiskLevel` e interfaces de limpeza.
- `internal/deleter/deleter.go` — Ciclo de vida de 6 estágios do `Coordinator`, interceptação de segurança e opções.
- `internal/ui/report.go` — Renderização do relatório de limpeza e cartões Lip Gloss.
- `cmd/clean.go` — Comando CLI `clean`, orquestração de opções e chamada ao Coordinator.
- `cmd/scan.go` e `cmd/format.go` — Formatação de tabelas e JSON com suporte ao marcador `⊙`.
- `internal/tui/view.go` e `internal/tui/model.go` — Exibição visual de alvos na TUI interativa e tratamento de `[⊙]`.
- `internal/tui/purge.go` — Modelo de TUI para purge de repositórios.
- `internal/mcp/server.go` — Ferramentas MCP `scan_environment`, `propose_deletion` e `execute_approved_deletion`.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/safety.Blacklist`: Validação inviolável de caminhos protegidos contra deleção acidental.
- `internal/trash.Manager`: Gerenciamento de lixeira com expurgo de retenção e limites de cota em bytes.
- `internal/hooks.Runner`: Disparo de scripts pré e pós deleção com dados estruturados de contexto.
- `internal/history`: Persistência de auditoria em arquivo JSONL.
- `internal/scheduler`: Registro de métricas e status de execuções automatizadas.

### Established Patterns
- Ciclo de vida de deleção em 6 estágios orquestrado exclusivamente via `deleter.Coordinator`.
- `Target.IsVirtual()` para bifurcar alvos de filesystem vs alvos virtuais de daemon (Docker/OrbStack).
- `errgroup` com streaming de `domain.ScanResult` e agregação com `engine.Collect`.

### Integration Points
- `internal/domain/types.go`: Adição de `ReviewOnly bool`, `ReviewReason string` e constantes de razão em `domain`.
- `internal/deleter/deleter.go`: Filtragem no Estágio 1, adição de `SkippedReviewOnly` em `Result` e encapsulamento de todas as mutações sob `!opts.DryRun`.
- `cmd/clean.go`: Remoção de chamadas externas de manutenção de lixeira fora do Coordinator; exibição do aviso de itens de revisão no sumário.
- `cmd/format.go`: Formatação da tabela `devclean scan` exibindo `⊙` e razão no modo verbose.
- `internal/tui/view.go`: Renderização do marcador `[⊙]` bloqueado para seleção em alvos `ReviewOnly`.
- `internal/mcp/server.go`: Exposição de `status: "review_only"` e metadados no `propose_deletion`.

</code_context>

<specifics>
## Specific Ideas

- Paridade com Mole (tw93/mole): exibir itens volumosos de infraestrutura (Docker storage, OrbStack data, pacotes NuGet, pnpm store) destacados com `⊙`, permitindo que o desenvolvedor tome ciência do espaço ocupado sem risco de exclusão acidental.
- Inviolabilidade garantida: nenhum comando automatizado ou com flag `--force` deve conseguir apagar alvos marcados como `ReviewOnly` pelo DevClean.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 7-ReviewOnly & Coordinator Policy*  
*Context gathered: 2026-10-15*  

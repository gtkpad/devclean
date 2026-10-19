---
phase: "07-reviewonly-coordinator-policy"
plan: "07-03"
title: "CLI & TUI Presentation Parity for ReviewOnly Targets"
wave: 3
completed_at: "2026-10-15"
requirements:
  - POL-03
status: "completed"
commits:
  - hash: "98646ae"
    message: "test(cmd/ui): add tests for CLI review prefix, verbose tags, and post-clean notice banner"
  - hash: "259db29"
    message: "feat(cmd/ui): implement CLI review prefix, verbose tags, and post-clean summary notice banner"
  - hash: "b5fca99"
    message: "test(tui): add unit tests for ReviewOnly locked badge, space guard, and mass select exclusion"
  - hash: "926aacd"
    message: "feat(tui): lock ReviewOnly targets with [⊙] indicator and guard against deletion selection"
coverage:
  cmd: "73.0%"
  internal/ui: "95.8%"
  internal/tui: "81.3%"
---

# Plan 07-03 Summary: CLI & TUI Presentation Parity for ReviewOnly Targets

## Overview
O plano `07-03` da Fase 07 foi concluído com sucesso em estrita conformidade com o protocolo TDD (RED-GREEN), satisfazendo o requisito `POL-03` e entregando paridade visual e comportamental completa no padrão do Mole (`tw93/mole`) em todas as superfícies CLI e TUI do DevClean.

### Principais Entregas
1. **Formatação da Tabela da CLI (`devclean scan`) (POL-03, D-09):**
   - Alvos com `ReviewOnly == true` recebem o prefixo `⊙ ` em seus caminhos, mantendo o alinhamento de colunas em 50 caracteres mesmo sob truncamento de caminhos longos (`⊙ ...<sufixo>`).
   - No modo verbose (`-v`), alvos de revisão exibem a tag `[ReviewOnly: <reason>] (eligible: 0 B)` com fallback seguro para `shared_store` caso a razão não seja especificada.

2. **Banner Informativo Pós-Limpeza (`devclean clean`) (POL-03, D-12):**
   - Estruturas `ui.ReportData` e `ui.CleanReportJSON` estendidas com `SkippedReviewCount int` e `SkippedReviewBytes int64`.
   - `cmd/clean.go` computa os totais a partir de `res.SkippedReviewOnly` retornado pelo `Coordinator` e repassa para a camada de visualização.
   - O card de sumário renderiza um banner informativo estilizado em âmbar (`lipgloss.Color("214")`): `⊙ N itens mantidos para revisão (X GB) — execute 'devclean scan' para detalhes` sempre que `SkippedReviewCount > 0`.
   - A saída JSON em `FormatReportJSON` serializa `skipped_review_count` e `skipped_review_bytes`.

3. **Indicador Travado `[⊙]` e Travamento de Seleção na TUI (POL-03, D-10):**
   - Na TUI interativa (`internal/tui/view.go`), alvos de revisão renderizam permanentemente o badge `[⊙]` com estilo âmbar/ouro (`lipgloss.Color("214").Bold(true)`) em vez dos checkboxes normais `[ ]` ou `[x]`.
   - Ao pressionar `Space` sobre um item de revisão, a seleção é rejeitada e a mensagem explicativa é exibida na barra de status: `Item de revisão [<reason>]: não pode ser selecionado para exclusão`.
   - As teclas de atalho de seleção em massa (`a` selecionar tudo, `m` seleção por threshold de tamanho e `l` seleção de baixo risco) filtram e excluem incondicionalmente todos os alvos `ReviewOnly`.
   - A tela de confirmação e o sink de deleção nunca recebem itens de revisão a partir da TUI.

---

## Tarefas Executadas

### Task 07-03-01 [TDD]: CLI scan table ⊙ prefix formatting & post-clean summary notice banner
- **RED**: Adicionados `TestFormatTable_ReviewOnly` em `cmd/format_test.go` e `TestReport_ReviewNotice` em `internal/ui/report_test.go`, falhando na ausência de prefixos `⊙ `, tags verbose e banner pós-limpeza.
  - Commit: `98646ae` `test(cmd/ui): add tests for CLI review prefix, verbose tags, and post-clean notice banner`
- **GREEN**:
  - Implementado prefixo `⊙ ` com preservação de truncamento e formatação verbose em `cmd/format.go`.
  - Adicionados campos `SkippedReviewCount` e `SkippedReviewBytes` em `internal/ui/report.go`, renderizando aviso informativo em âmbar no card pós-limpeza e nos relatórios JSON.
  - Integrado repasse de métricas de `res.SkippedReviewOnly` em `cmd/clean.go`.
  - Commit: `259db29` `feat(cmd/ui): implement CLI review prefix, verbose tags, and post-clean summary notice banner`

### Task 07-03-02 [TDD]: Interactive TUI locked [⊙] indicator & non-selectable review guard
- **RED**: Adicionados `TestTUI_ReviewOnly_RenderLockedIndicator`, `TestTUI_ReviewOnly_SpaceToggle_Ignored` e `TestTUI_ReviewOnly_MassSelect_Excluded` em `internal/tui/review_test.go`, falhando na ausência de indicador travado e filtros de atalhos.
  - Commit: `b5fca99` `test(tui): add unit tests for ReviewOnly locked badge, space guard, and mass select exclusion`
- **GREEN**:
  - Implementado estilo `reviewStyle` e renderização do badge travado `[⊙]` em `internal/tui/view.go`.
  - Adicionado campo `statusMessage` e exibição condicional na barra de rodapé em `internal/tui/view.go`.
  - Protegidos os manipuladores de teclas `KeySpace`, `'a'`, `'m'` e `'l'` em `internal/tui/model.go` contra seleção de alvos `ReviewOnly`.
  - Commit: `926aacd` `feat(tui): lock ReviewOnly targets with [⊙] indicator and guard against deletion selection`

---

## Verificação e Cobertura
- **Testes Automatizados:**
  - `go test -v ./cmd ./internal/ui ./internal/tui` aprovado com 100% de sucesso.
  - Suíte completa do projeto (`go test ./...`) validada com zero falhas.
- **Cobertura de Código:**
  - `cmd`: 73.0% dos statements
  - `internal/ui`: 95.8% dos statements
  - `internal/tui`: 81.3% dos statements
- **Invariantes Verificados:**
  1. `devclean scan` prefixa `⊙ ` e preserva visibilidade do marcador em caminhos longos.
  2. Modo verbose exibe `[ReviewOnly: <reason>] (eligible: 0 B)` com bytes elegíveis nulos.
  3. Relatório pós-limpeza exibe banner com quantidade e bytes mantidos sob revisão.
  4. TUI renderiza `[⊙]` travado em tom âmbar e ignora `Space`, `'a'`, `'m'`, `'l'` para itens sob revisão.

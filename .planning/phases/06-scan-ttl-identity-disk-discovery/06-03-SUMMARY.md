---
phase: "06-scan-ttl-identity-disk-discovery"
plan: "06-03"
title: "Wave 3 — Canonical Ownership Resolution and Overlapping Target Deduplication"
wave: 3
completed_at: "2026-10-14"
requirements:
  - SCAN-04
status: "completed"
commits:
  - hash: "6862135"
    message: "test(engine): add canonical ownership and deduplication test suite"
  - hash: "ac9b363"
    message: "feat(engine): implement canonical ownership resolution and byte deduction"
  - hash: "24014e8"
    message: "test(engine): add end-to-end integration tests for canonical ownership in Collect"
  - hash: "684b732"
    message: "feat(engine): integrate canonical ownership into engine Collect and TUI"
coverage:
  internal/engine: "96.7%"
  internal/tui: "91.1%"
---

# Plan 06-03 Summary: Wave 3 — Canonical Ownership Resolution and Overlapping Target Deduplication

## Overview
O plano `06-03` da Fase 06 foi executado com sucesso em total conformidade com o protocolo TDD (RED-GREEN), cumprindo o requisito `SCAN-04` e consolidando as decisões arquiteturais `D-01`, `D-02`, `D-03` e `D-04`.

A partir desta entrega:
1. **Propriedade Canônica e Resolução de Posse (SCAN-04, D-01, D-03):** Centralizada no pacote `internal/engine` através da função `ResolveCanonicalOwnership`, assegurando o princípio inviolável de que **cada byte em disco possui exatamente um dono canônico**.
2. **Precedência do Mais Específico ("Dedicated Wins", D-01, D-04):** Cleaners dedicados de ecossistemas (`TierDedicated`: node, rust, python, bun, go, dotnet, java, etc.) possuem precedência estrita sobre cleaners genéricos de sistema/usuário (`TierGeneric`: macos, system, browsers, leftovers). Alvos subordinados de mesmo caminho físico são omitidos da listagem entregue à UI e relatórios.
3. **Dedução Hierárquica sem Underflow (D-01, T-06-06):** Quando alvos filhos residem dentro de um diretório ancestral, os bytes de `SizeBytes` e `EligibleBytes` dos descendentes diretos são subtraídos do ancestral com clamp rigoroso contra números negativos (`parent.SizeBytes = max(0, parent.SizeBytes - child.SizeBytes)`).
4. **Resolução Física de Symlinks e Fallback Gracioso (D-02, T-06-05):** Symlinks no filesystem são canonicamente resolvidos via `filepath.EvalSymlinks`, unificando referências ao mesmo diretório físico. Em casos de symlinks circulares ou quebrados, o sistema degrada defensivamente para `filepath.Clean` sem travar, causar loops infinitos ou entrar em pânico.
5. **Imunidade de Alvos Virtuais e Targets Sintéticos:** Alvos com esquema virtual (`t.IsVirtual() == true`, ex: `docker://`, `pnpm://`, `brew://`) e targets de teste com caminhos vazios são 100% imunes a deduplicações de filesystem ou alterações indevidas de caminho.
6. **Integração Centralizada em Engine e TUI (D-03):** O método `engine.Collect` drena o canal de streaming e aplica automaticamente `ResolveCanonicalOwnership` antes de entregar o slice consolidado. Da mesma forma, na interface interativa `internal/tui/model.go`, a mensagem `scanDoneMsg` aplica a resolução canônica antes do achatamento dos alvos, garantindo inventários deduplicados em CLI, TUI e MCP.
7. **Zero CGO:** Compilação e execução de testes herméticos sem dependências CGO (`CGO_ENABLED=0 go test ./...`).

---

## Tarefas Executadas

### Tarefa 1 & 2 [TDD]: Módulo de Propriedade Canônica (`ownership.go` e `ownership_test.go`)
- **RED**: Criado `internal/engine/ownership_test.go` cobrindo a matriz completa de requisitos:
  - `TestOwnership_GetEcosystemTier`: classificação de ecossistemas em `TierGeneric` e `TierDedicated`.
  - `TestOwnership_IsDescendant`: validação robusta de contenção de caminhos via `filepath.Rel`.
  - `TestOwnership_DedicatedWinsOverGeneric`: cleaner dedicado absorve cleaner genérico em caminho físico idêntico (`D-01`, `D-04`).
  - `TestOwnership_DeductChildBytesFromParent`: dedução de bytes entre diretório pai e filho (`D-01`).
  - `TestOwnership_ClampAgainstNegative`: proteção defensiva contra inteiros negativos (`T-06-06`).
  - `TestOwnership_SymlinkDeduplication`: unificação de symlink e diretório físico no mesmo alvo canônico (`D-02`).
  - `TestOwnership_CircularSymlinkHandling`: degradação graciosa para `filepath.Clean` em symlinks cíclicos (`T-06-05`).
  - `TestOwnership_VirtualTargetsImmune`: imunidade total de recursos virtuais.
  - `TestOwnership_MultiLevelNesting`: dedução correta em árvore com 3 níveis (root -> sub -> nested) com 100MB preservados e zero double-counting.
- **GREEN**: Criado `internal/engine/ownership.go` com tipos `EcosystemTier`, helpers `getEcosystemTier`, `isDescendant`, `isDirectDescendant` e a função pública `ResolveCanonicalOwnership`.
- **Commits**:
  - `6862135` `test(engine): add canonical ownership and deduplication test suite`
  - `ac9b363` `feat(engine): implement canonical ownership resolution and byte deduction`

### Tarefa 3 [TDD]: Integração Centralizada no Engine e na TUI
- **RED**: Adicionado teste de integração `TestEngine_ScanAndCollect_AppliesCanonicalOwnership` em `internal/engine/engine_test.go`, registrando múltiplos scanners que retornam alvos sobrepostos e validando que `Collect(ctx, eng.Scan(ctx))` aplica a resolução canônica.
- **GREEN**:
  - Atualizado `Collect` em `internal/engine/engine.go` para retornar `ResolveCanonicalOwnership(results)` tanto ao fechar o canal quanto ao cancelar o contexto.
  - Atualizado `internal/tui/model.go` em `case scanDoneMsg:` e `case scanCompleteMsg:` para invocar `engine.ResolveCanonicalOwnership` antes de chamar `flattenResults`.
  - Tratatados alvos sintéticos com `Path == ""` em `ownership.go` como imunes à deduplicação de filesystem, preservando testes de concorrência com mocks vazios.
- **Commits**:
  - `24014e8` `test(engine): add end-to-end integration tests for canonical ownership in Collect`
  - `684b732` `feat(engine): integrate canonical ownership into engine Collect and TUI`

---

## Verificação e Garantias de Qualidade
- **Zero CGO:** `CGO_ENABLED=0 go test ./...` executado e aprovado com sucesso.
- **Cobertura de Código:**
  - `internal/engine`: 96.7%
  - `internal/tui`: 91.1%
- **Rastreabilidade de Requisitos:**
  - `SCAN-04`: Coberto integralmente por `ownership.go`, `ownership_test.go`, `engine.Collect` e `tui.Update`.

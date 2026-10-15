---
phase: "08-node-headless-shell-cli-xcode"
plan: "08-01"
title: "Node Granular Caches, Automation Browsers, and pnpm Metadata Cache"
wave: 1
completed_at: "2026-10-15"
requirements:
  - DEV-01
status: "completed"
commits:
  - hash: "51e0860"
    message: "feat(cleaner/node): discover granular package and automation browser caches"
  - hash: "1377360"
    message: "feat(cleaner/node): guard direct filesystem deletion against symlinks and blacklist"
  - hash: "263320b"
    message: "feat(cleaner/pnpm): discover http metadata cache by disk presence"
coverage:
  internal/cleaner/node: "90.7%"
  internal/cleaner/pnpm: "90.9%"
---

# Plan 08-01 Summary: Node Granular Caches, Automation Browsers, and pnpm Metadata Cache

## Overview
O plano `08-01` da Fase 08 foi executado com sucesso seguindo rigorosamente a disciplina TDD (RED-GREEN), satisfazendo o requisito `DEV-01` e garantindo paridade de inventário com o Mole (`tw93/mole`) para caches do ecossistema Node.js, navegadores de automação headless e o gerenciador de pacotes `pnpm`.

### Principais Entregas
1. **Descoberta Granular de Subdiretórios npm e Corepack (DEV-01, D-01, D-03):**
   - Segmentação de caches globais em alvos distintos e rastreáveis: `npm-cache` (`~/.npm/_cacache`), `npx-cache` (`~/.npm/_npx`), `npm-logs` (`~/.npm/_logs`) e `corepack-cache` (`~/Library/Caches/node/corepack`, `~/.cache/node/corepack`).
   - Descoberta por presença física em disco sem necessidade de `node` ou `npm` no PATH.
   - Atribuição de risco `RiskLow` para artefatos regeneráveis de pacotes e logs.

2. **Descoberta de Navegadores de Automação Headless e Classificação de Risco (DEV-01, D-01, D-02):**
   - Suporte a `node-puppeteer` (`~/.cache/puppeteer`, `~/.puppeteer` ou `$PUPPETEER_CACHE_DIR`).
   - Suporte a `node-playwright` (`~/Library/Caches/ms-playwright`, `~/.cache/ms-playwright` ou `$PLAYWRIGHT_BROWSERS_PATH`).
   - Suporte a `node-cypress` (`~/Library/Caches/Cypress`, `~/.cache/Cypress` ou `$CYPRESS_CACHE_FOLDER`).
   - Classificação diferenciada de risco: `RiskMedium` para Puppeteer e Playwright (evitando perda inadvertida de downloads pesados de binários de browser em ambientes com restrição de rede) e `RiskLow` para Cypress.

3. **Deleção Direta Segura via Filesystem (D-05, VAL-01):**
   - Proteção estrita em `nodeCleaner.Clean()` utilizando `safety.IsCoreBlacklisted(path)` e `safety.IsSymlink(path)` antes de qualquer invocação de `os.RemoveAll`.
   - Links simbólicos são pulados de forma segura, garantindo que alvos externos não sejam atravessados nem modificados.
   - Operação 100% offline sem executar CLIs externos de terceiros (`npx`, `cypress`).

4. **Cache de Metadados HTTP do pnpm e Detecção de Presença em Disco (DEV-01, D-04, SCAN-02):**
   - O cleaner `internal/cleaner/pnpm` agora reconhece o cache físico de metadados HTTP (`~/Library/Caches/pnpm` no macOS e `~/.cache/pnpm` no Linux).
   - `IsInstalled()` retorna `true` caso o diretório de cache exista, permitindo inventariar e limpar resíduos do pnpm mesmo quando o binário `pnpm` não estiver presente no PATH.
   - `Scan()` inventaria `pnpm metadata cache` com categoria `pnpm-cache` e `RiskLow`.
   - `Clean()` delega `pnpm://store/prune` ao CLI quando aplicável e remove diretórios físicos via `os.RemoveAll` com proteção de symlinks e blacklist.

---

## Tarefas Executadas

### Task 08-01-01 [TDD]: End-to-end tracer slice: Granular Node package caches and automation browser discovery by disk presence
- **RED**: Adicionados testes `TestNodeCleaner_Scan_GlobalCaches_AutomationAndNpm` e `TestNodeCleaner_RiskLevels_Automation` em `internal/cleaner/node/node_test.go` verificando categorias individuais, tamanhos e níveis de risco. Falha confirmada com categorias ausentes.
- **GREEN**: Definidos `globalCacheDef` e `getGlobalCacheCandidates()` com suporte a `npm-cache`, `npx-cache`, `npm-logs`, `corepack-cache`, `node-puppeteer`, `node-playwright` e `node-cypress`. Refatorado `scanGlobalCaches()` com deduplicação de caminhos, cálculo de tamanho real e identificadores SHA-256 de 12 caracteres hexadecimais.
- **Commit**: `51e0860` `feat(cleaner/node): discover granular package and automation browser caches`

### Task 08-01-02 [TDD]: Safe direct filesystem deletion with symlink and core blacklist guards in Node cleaner
- **RED**: Adicionados testes `TestNodeCleaner_Clean_DirectFilesystem` e `TestNodeCleaner_Clean_SymlinkProtection` em `internal/cleaner/node/node_test.go`. Falha confirmada com symlink e pasta protegida sendo removidos.
- **GREEN**: Integradas as checagens `safety.IsCoreBlacklisted(t.Path)` e `safety.IsSymlink(t.Path)` em `nodeCleaner.Clean()` antes de chamar `os.RemoveAll`.
- **Commit**: `1377360` `feat(cleaner/node): guard direct filesystem deletion against symlinks and blacklist`

### Task 08-01-03 [TDD]: pnpm HTTP metadata cache discovery and IsInstalled disk presence without CLI
- **RED**: Adicionados testes `TestPnpmCleaner_IsInstalled_DiskPresence`, `TestPnpmCleaner_Scan_DiskMetadataCache` e `TestPnpmCleaner_Clean_DiskMetadataCache` em `internal/cleaner/pnpm/pnpm_test.go`. Falha confirmada para presença em disco, scanning do cache de metadados e deleção segura.
- **GREEN**: Atualizados `IsInstalled()` para checar diretórios de metadados, `Scan()` para inventariar o cache físico com TTL e SHA-256 ID, e `Clean()` para executar deleção de diretórios físicos com guardas de segurança.
- **Commit**: `263320b` `feat(cleaner/pnpm): discover http metadata cache by disk presence`

---

## Verificação e Cobertura
- **Testes Unitários:**
  `go test -v ./internal/cleaner/node ./internal/cleaner/pnpm` executado com 100% de sucesso.
- **Cobertura de Código:**
  - `internal/cleaner/node`: 90.7% dos statements (>85% exigido)
  - `internal/cleaner/pnpm`: 90.9% dos statements (>85% exigido)
- **Suíte Global:**
  `go test ./...` executado em todos os pacotes com zero falhas.

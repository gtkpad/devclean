---
phase: "08-node-headless-shell-cli-xcode"
plan: "08-04"
title: "Integration, Coordinator Verification & Hermetic Suite"
wave: 2
completed_at: "2026-10-15"
requirements:
  - DEV-01
  - DEV-02
  - DEV-03
status: "completed"
commits:
  - hash: "7c0dc19"
    message: "feat(config,engine): wire shell ecosystem into config, engine registration, and TUI metadata"
  - hash: "a21725f"
    message: "test(cleaner): add cross-cleaner symlink guard and permission resilience suite"
  - hash: "c5ec513"
    message: "test(deleter): add Coordinator DryRun zero-mutation canary test for Phase 08"
coverage:
  internal/cleaner/node: "91.3%"
  internal/cleaner/pnpm: "92.7%"
  internal/cleaner/shell: "91.4%"
  internal/cleaner/xcode: "89.9%"
  internal/engine: "95.3%"
---

# Plan 08-04 Summary: Integration, Coordinator Verification & Hermetic Suite

## Overview
O plano `08-04` (Wave 2) da Fase 08 integrou de ponta a ponta todos os ecossistemas expandidos e criados (`node`, `pnpm`, `shell` e `xcode`), consolidando o registro do ecossistema `"shell"` nos modelos de configuração, motor de execução e metadados da TUI. Além disso, estabeleceu a suíte cruzada de proteção contra travessia de symlinks (VAL-01) e o teste canário de DryRun com mutação zero no `Coordinator` (POL-04, VAL-02), confirmando que todos os pacotes alterados na fase superam com folga a meta de 85% de cobertura.

### Principais Entregas
1. **Registro e Configuração do Ecossistema 'shell' (DEV-02, D-06):**
   - Inclusão dos campos `Shell EcosystemConfig` (`mapstructure:"shell"`) e `Shell HookConfig` (`mapstructure:"shell"`) em `internal/config/config.go`.
   - Registro de defaults em `internal/config/defaults.go`: `Enabled: true, MaxAgeDays: 30, FailMode: "continue"` e timeout de hook `"30s"`.
   - Atualização de `allowedCleanValues` e do mapa de hooks em `internal/config/validate.go`.
   - Adição do bloco documentado do ecossistema `shell` no template `DefaultConfigYAML` em `internal/config/save.go` (atualizando contagem global de 18 para 19 ecossistemas).
   - Roteamento de `case "shell": return e.config.Ecosystems.Shell` no `Engine.ecosystemConfig()` em `internal/engine/engine.go`.
   - Registro de `shell.NewWithExecutor(exec)` no motor central em `internal/engine/register.go`.
   - Adição de metadados ("Shell & CLI") e binding dinâmico no `ConfigModel` da TUI em `internal/tui/config.go`.

2. **Suíte Hermética de Proteção de Symlinks e Resiliência de Permissões (VAL-01, T-08-10):**
   - Criação de testes dedicados `Test<Cleaner>_Clean_SymlinkProtection` nos pacotes `internal/cleaner/node`, `internal/cleaner/pnpm`, `internal/cleaner/shell` e `internal/cleaner/xcode`.
   - Suíte cruzada `TestCrossCleaner_SymlinkProtection_Phase08` em `internal/cleaner/cleaner_test.go` verificando que nenhum dos cleaners da Fase 08 percorre ou deleta alvos externos apontados por symlinks.
   - Testes de tratamento de erros de permissão (`Test<Cleaner>_Clean_PermissionHandling`) assegurando degradação suave sem pânico sob diretórios com restrições de permissão.

3. **Canário de Verificação de Mutação Zero no Coordinator (POL-04, VAL-02, T-08-11):**
   - Implementação de `TestCoordinator_DryRun_ZeroMutation_Phase08` em `internal/deleter/policy_test.go`.
   - Criação de payloads reais para 9 alvos representativos dos 4 ecossistemas da fase (Puppeteer, npm cacache, pnpm metadata, mise cache, mise downloads, Zsh compdumps, Xcode DerivedData, Xcode Archives, CoreSimulator Caches).
   - Verificação estrita antes e após execução em modo `DryRun: true`:
     - Identidade exata de hash criptográfico SHA-256 em cada arquivo de payload.
     - Identidade exata de carimbo de tempo de modificação (`mtime`).
     - Identidade exata de tamanho em bytes (`size`).
     - Diretório de lixeira (`trashDir`) vazio e sem criação de `manifest.json`.
     - Zero invocação de cleaners reais ou submissão de comandos externos.
     - Métricas do relatório refletindo alvos simulados (`TotalCount`, `TotalFreed`) e retenção correta de alvos `ReviewOnly` (`SkippedReviewOnly`).

4. **Auditoria de Cobertura e Suite Completa (>85% em todos os pacotes):**
   - `internal/cleaner/node`: 91.3%
   - `internal/cleaner/pnpm`: 92.7%
   - `internal/cleaner/shell`: 91.4%
   - `internal/cleaner/xcode`: 89.9%
   - `internal/engine`: 95.3%
   - `go test ./...` executado e aprovado com 100% de sucesso em todo o repositório.

---

## Tarefas Executadas

### Task 08-04-01 [TDD]: Engine registration and Configuration wiring for 'shell' ecosystem
- **RED**: Atualizados `internal/config/config_test.go` e `internal/engine/register_test.go` esperando o ecossistema `shell` e contagem de 19 ecossistemas. Falhas de compilação e asserções confirmadas.
- **GREEN**: Adicionados campos de struct `Shell` em `Config` e `HooksConfig`, defaults, validação, serialização YAML, roteamento no `Engine`, registro em `register.go`, metadados na TUI e testes de TUI atualizados.
- **Commit**: `7c0dc19` `feat(config,engine): wire shell ecosystem into config, engine registration, and TUI metadata`

### Task 08-04-02 [TDD]: Comprehensive cross-cleaner symlink guard and permission tests
- **GREEN**: Implementado `TestPnpmCleaner_Clean_SymlinkProtection`, testes de permissão `Test<Cleaner>_Clean_PermissionHandling` em `node`, `pnpm`, `shell` e `xcode`, e teste integrador `TestCrossCleaner_SymlinkProtection_Phase08` em `internal/cleaner/cleaner_test.go`.
- **Commit**: `a21725f` `test(cleaner): add cross-cleaner symlink guard and permission resilience suite`

### Task 08-04-03 [TDD]: Coordinator DryRun zero-mutation canary verification test suite and >85% coverage audit
- **GREEN**: Implementado `TestCoordinator_DryRun_ZeroMutation_Phase08` em `internal/deleter/policy_test.go` com verificação criptográfica SHA-256, mtime e size de 9 alvos da Fase 08.
- **Ajustes de Isolamento em Testes**: Isolados `setupMultiEcosystemCleanFixture` e `createEmptyConfig` em `cmd/` para desabilitar explicitamente `shell` contra detecção de caches reais do usuário local.
- **Commit**: `c5ec513` `test(deleter): add Coordinator DryRun zero-mutation canary test for Phase 08`

---

## Verificação e Cobertura
- **Canário DryRun Phase 08:**
  `go test -v ./internal/deleter -run "TestCoordinator_DryRun_ZeroMutation_Phase08"` aprovado com 0 mutações.
- **Suíte de Symlink Protection:**
  `go test -v ./internal/cleaner/... -run "Test.*_Clean_SymlinkProtection"` 100% verde.
- **Cobertura dos Pacotes Alterados:**
  - `internal/cleaner/node`: 91.3% (>85%)
  - `internal/cleaner/pnpm`: 92.7% (>85%)
  - `internal/cleaner/shell`: 91.4% (>85%)
  - `internal/cleaner/xcode`: 89.9% (>85%)
  - `internal/engine`: 95.3% (>85%)
- **Regressão Global:**
  `go test ./...` 100% verde em todos os módulos e pacotes.

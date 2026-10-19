---
phase: "08-node-headless-shell-cli-xcode"
plan: "08-03"
title: "Xcode and Simulators Expansion with Process Guard"
wave: 1
completed_at: "2026-10-15"
requirements:
  - DEV-03
status: "completed"
commits:
  - hash: "ca85d34"
    message: "feat(xcode): expand target inventory with CoreSimulator, DeviceSupport, and Archives"
  - hash: "d654141"
    message: "feat(xcode): implement injectable fail-closed process guard for Xcode and Simulator"
  - hash: "c9cbac1"
    message: "test(xcode): verify Archives RiskHigh classification and process guard edge cases"
coverage:
  internal/cleaner/xcode: "89.0%"
---

# Plan 08-03 Summary: Xcode and Simulators Expansion with Process Guard

## Overview
O plano `08-03` da Fase 08 foi executado com sucesso seguindo rigorosamente a disciplina TDD (RED-GREEN), satisfazendo integralmente o requisito `DEV-03` e as decisões arquiteturais `D-11`, `D-12`, `D-13` e `D-14`.

O cleaner `internal/cleaner/xcode` foi expandido para cobrir subdiretórios críticos do desenvolvedor Apple (`CoreSimulator Caches`, `CoreSimulator Temp`, `iOS DeviceSupport`, `watchOS DeviceSupport` e `Archives`), com integração de um mecanismo injetável fail-closed de proteção contra processos ativos (`pgrep -x Xcode` e `pgrep -x Simulator`) e validações de segurança contra links simbólicos e caminhos do sistema.

### Principais Entregas
1. **Expansão do Inventário do Xcode (DEV-03, D-14):**
   - Suporte a `CoreSimulator Caches` (`~/Library/Developer/CoreSimulator/Caches`, categoria `xcode-coresimulator-caches`, `RiskLow`, `simRelated: true`).
   - Suporte a `CoreSimulator Temp` (`~/Library/Developer/CoreSimulator/Temp`, categoria `xcode-coresimulator-temp`, `RiskLow`, `simRelated: true`).
   - Suporte a `Xcode iOS DeviceSupport` (`~/Library/Developer/Xcode/iOS DeviceSupport`, categoria `xcode-ios-devicesupport`, `RiskLow`).
   - Suporte a `Xcode watchOS DeviceSupport` (`~/Library/Developer/Xcode/watchOS DeviceSupport`, categoria `xcode-watchos-devicesupport`, `RiskLow`).
   - Suporte a `Xcode Archives` (`~/Library/Developer/Xcode/Archives`, categoria `xcode-archives`, `RiskHigh`).
   - Descoberta baseada puramente por presença física em disco, com cálculo de bytes reais (`SizeBytes`), autorização TTL (`rules.MaxAgeDays`) e identificadores SHA-256 estáveis.

2. **Guarda de Processos Injetável e Fail-Closed (DEV-03, D-11, D-12, T-08-07, T-08-08):**
   - Criação do tipo `cleaner.MockExitError` em `internal/cleaner/executor_mock.go` implementando `interface{ ExitCode() int }` e `error` para testes determinísticos sem CGO.
   - Função `checkProcessStatus(ctx context.Context, exec cleaner.CommandExecutor, processName string) (running bool, uncertain bool)`:
     - Executa `pgrep -x <name>` diretamente via `cleaner.CommandExecutor` com vetor de argumentos sem interpolação de shell (`sh -c`).
     - Se `exit 0` e stdout não-vazio: `running: true, uncertain: false`.
     - Se `exit 1` (clean match miss): `running: false, uncertain: false`.
     - Qualquer outro erro (pgrep ausente, timeout, permissão negada, exit > 1): `running: false, uncertain: true` (**FAIL CLOSED**).
   - Quando `Xcode` está ativo ou em estado incerto: TODOS os alvos (DerivedData, Archives, DeviceSupport, CoreSimulator e simuladores virtuais) recebem `ReviewOnly: true`, `ReviewReason: domain.ReviewReasonActiveProcess`, `EligibleBytes: 0` e `TTLEligible: false`.
   - Quando `Simulator` está ativo ou incerto (com Xcode fechado): alvos relacionados ao simulador (`CoreSimulator Caches`, `CoreSimulator Temp` e `xcode://simulators/unavailable`) recebem `ReviewOnly: true`, `ReviewReason: domain.ReviewReasonActiveProcess`, `EligibleBytes: 0` e `TTLEligible: false`, enquanto DerivedData, DeviceSupport e Archives permanecem elegíveis normalmente.

3. **Política de Segurança para Xcode Archives (DEV-03, D-13, T-08-09):**
   - Classificação permanente como `domain.RiskHigh` para alertar usuários sobre o risco de perda irreversível de símbolos de depuração (`dSYM`) essenciais para relatórios de crash de produção.
   - Quando o Xcode está fechado, o alvo opera normalmente com autorização TTL (`ReviewOnly: false`, `EligibleBytes > 0`). Quando o Xcode está aberto, é imediatamente bloqueado por `ReviewOnly: true`.

4. **Deleção Segura do Filesystem (VAL-01):**
   - `Clean()` valida todos os caminhos físicos contra `safety.IsCoreBlacklisted(path)` e proteção estrita contra travessia de links simbólicos (`safety.IsSymlink(path)`), recusando-se a remover ou percorrer symlinks.

---

## Tarefas Executadas

### Task 08-03-01 [TDD]: Expanded Xcode target inventory and safe filesystem deletion
- **RED**: Criados testes `TestXcodeCleaner_Scan_ExpandedTargets`, `TestXcodeCleaner_Clean_DirectFilesystem` e `TestXcodeCleaner_Clean_SymlinkProtection`. Falha confirmada para ausência dos 5 novos alvos e remoção inadvertida de symlinks.
- **GREEN**: Implementada struct `xcodeTargetDef`, método `getExpandedTargetDefs()`, varredura dos alvos em `Scan()` e validações de `safety.IsCoreBlacklisted` e `safety.IsSymlink` em `Clean()`.
- **Commit**: `ca85d34` `feat(xcode): expand target inventory with CoreSimulator, DeviceSupport, and Archives`

### Task 08-03-02 [TDD]: Injectable fail-closed process guard via cleaner.CommandExecutor for Xcode and Simulator
- **RED**: Adicionados testes `TestXcodeCleaner_ProcessGuard_XcodeActive`, `TestXcodeCleaner_ProcessGuard_SimulatorActive` e `TestXcodeCleaner_ProcessGuard_FailClosedOnPgrepError`. Falha confirmada com alvos reportando `ReviewOnly = false`.
- **GREEN**: Adicionado `cleaner.MockExitError` em `executor_mock.go`. Implementados `checkProcessStatus()` e `isSimulatorTarget()` com aplicação da regra fail-closed no final do `Scan()`. Adicionado helper `mockProcessesStopped` nos testes anteriores.
- **Commit**: `d654141` `feat(xcode): implement injectable fail-closed process guard for Xcode and Simulator`

### Task 08-03-03 [TDD]: Xcode Archives RiskHigh classification and process guard edge cases
- **RED/GREEN**: Implementados testes `TestXcodeCleaner_Archives_RiskHighWhenClosed` e `TestXcodeCleaner_Archives_ReviewOnlyWhenOpen` validando `RiskHigh`, `ReviewOnly: false` com processo fechado e `ReviewOnly: true` com processo ativo.
- **Commit**: `c9cbac1` `test(xcode): verify Archives RiskHigh classification and process guard edge cases`

---

## Verificação e Cobertura
- **Testes Unitários:**
  `go test -v ./internal/cleaner/xcode` aprovado com 15/15 testes passando em 0.179s.
- **Cobertura de Código:**
  - `internal/cleaner/xcode`: 89.0% dos statements (meta >85% superada).
- **Regressão Global:**
  `go test ./...` executado com 100% de sucesso em toda a base do DevClean.

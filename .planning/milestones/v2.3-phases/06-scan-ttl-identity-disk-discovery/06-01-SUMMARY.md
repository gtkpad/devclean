---
phase: "06-scan-ttl-identity-disk-discovery"
plan: "06-01"
title: "Decouple Scan vs TTL, Extend domain.Target, Eliminate Cleaner Root Mtime Filters, and Align CLI"
wave: 1
completed_at: "2026-10-14"
requirements:
  - SCAN-01
status: "completed"
commits:
  - hash: "bc061eb"
    message: "test(domain): add serialization and backward compatibility tests for Target"
  - hash: "59761e0"
    message: "feat(domain): add EligibleBytes and TTLEligible to domain.Target"
  - hash: "78a2582"
    message: "test(cleaner): add root mtime independence tests for global cleaners"
  - hash: "6ec0ad1"
    message: "feat(cleaner): decouple scan discovery from root mtime in global cleaners"
  - hash: "46b34a5"
    message: "test(cleaner): add comprehensive mtime battery for project cleaners"
  - hash: "742aac4"
    message: "feat(cleaner): decouple scan discovery from root mtime in project cleaners"
  - hash: "0947004"
    message: "test(cli): add tests for TTLEligible auto-clean filtering and format display"
  - hash: "c3129b4"
    message: "feat(cli): consume TTLEligible in auto-clean and format real sizes"
coverage:
  internal/domain: "100.0%"
  internal/cleaner: "70.8%"
  cmd: "89.3%"
---

# Plan 06-01 Summary: Decouple Scan vs TTL, Extend domain.Target, Eliminate Cleaner Root Mtime Filters, and Align CLI

## Overview
O plano `06-01` da Fase 06 foi executado com sucesso e total aderência ao protocolo TDD (RED-GREEN), desacoplando a descoberta estrutural de alvos (`Scan`) da política de expurgo por tempo de inatividade (`TTL`) conforme a exigência `SCAN-01` e as decisões `D-05`, `D-06`, `D-07` e `D-08`.

A partir desta entrega:
1. **`domain.Target` Estendido (`D-05`, `D-06`):** Adicionados os campos `EligibleBytes int64` (`json:"eligible_bytes"`) e `TTLEligible bool` (`json:"ttl_eligible"`). O campo `SizeBytes` agora representa fidedignamente o tamanho físico ocupado em disco, sem contaminação por regras de TTL.
2. **Eliminação de Descartes Prematuros em Todos os 18 Cleaners (`D-05`, `D-07`, `D-08`):** Nenhum cleaner global ou de projeto (`bun`, `golang`, `dotnet`, `python`, `xcode`, `ai` [editors, claude, huggingface, ollama], `docker`, `nextjs`, `node`, `rust`, `java`, `pnpm`, `homebrew`, `cpp`, `flutter`, `terraform`, `cocoapods`, `elixir`) omite alvos durante a fase de varredura devido ao `mtime` da raiz. Alvos existentes em disco são sempre descobertos e catalogados no scan.
3. **Cálculo Preciso de Elegibilidade (`D-06`, `D-07`, `D-08`):**
   - Com `MaxAgeDays == 0`: `TTLEligible = true` e `EligibleBytes = SizeBytes`.
   - Com `MaxAgeDays > 0`: `TTLEligible` reflete se o alvo ou sub-artefato qualifica pelo tempo decorrido em relação ao cutoff. Alvos elegíveis têm `EligibleBytes = SizeBytes`; alvos retidos têm `EligibleBytes = 0` e `TTLEligible = false`.
4. **Alinhamento do Consumo na CLI (`cmd/clean.go` e `cmd/format.go`):**
   - No modo `--auto` (`cmd/clean.go`), os alvos são filtrados utilizando diretamente a avaliação nativa `t.TTLEligible` realizada pelos cleaners (com fallback compatível para alvos sintéticos legados).
   - Na visualização (`cmd/format.go`), a coluna `SIZE` formata `t.SizeBytes` em disco; o formato JSON expõe `eligible_bytes` e `ttl_eligible`; no modo `--verbose`, detalhes de elegibilidade parcial são descritos caso `EligibleBytes < SizeBytes`.
5. **Zero CGO & Cobertura:** Compilação hermética com `CGO_ENABLED=0 go test ./...` executada com 100% de aprovação.

---

## Tarefas Executadas

### Tarefa 1 [TDD]: Extensão de `domain.Target` com `EligibleBytes` e `TTLEligible` (`internal/domain/types.go`)
- **RED**: Adicionados testes `TestTarget_JSONSerialization` e `TestTarget_BackwardCompatibility` em `internal/domain/types_test.go` verificando que campos novos são serializados/desserializados corretamente e que payloads JSON legados sem os novos campos são carregados com valores default (zero-values).
- **GREEN**: Adicionados `EligibleBytes int64` e `TTLEligible bool` na struct `domain.Target` em `internal/domain/types.go`.
- **Commits**:
  - `bc061eb` `test(domain): add serialization and backward compatibility tests for Target`
  - `59761e0` `feat(domain): add EligibleBytes and TTLEligible to domain.Target`

### Tarefa 2 [TDD]: Remoção de Filtros de Mtime na Raiz em Cleaners Globais
- **RED**: Adicionados testes de independência de `mtime` para os cleaners `bun`, `golang`, `dotnet`, `python`, `xcode`, `docker` e os 4 módulos de `ai` (`editors`, `claude`, `huggingface`, `ollama`), validando que diretórios recentes continuam sendo retornados no `Scan()`.
- **GREEN**:
  - Removidos descartes prematuros por `mtime` em `bun.go`, `golang.go`, `dotnet.go`, `python.go`, `xcode.go`, `docker.go` e pacote `ai`.
  - Corrigido isolamento de teste no Go mod/build cache usando `os.LookupEnv` em `golang.go`.
  - Preenchimento consistente de `EligibleBytes` e `TTLEligible` em todos os alvos gerados.
- **Commits**:
  - `78a2582` `test(cleaner): add root mtime independence tests for global cleaners`
  - `6ec0ad1` `feat(cleaner): decouple scan discovery from root mtime in global cleaners`

### Tarefa 3 [TDD]: Remoção de Filtros de Mtime na Raiz em Cleaners de Projetos e Bateria de Validação de Mtime
- **RED**: Criado `internal/cleaner/cleaner_mtime_test.go` contendo:
  - `TestCleaners_RootMtimeRecent_StillReportedInScan`: manipula `os.Chtimes` para alvos recentes com `MaxAgeDays: 14` e valida que `len(targets) > 0`, `SizeBytes > 0`, `TTLEligible == false` e `EligibleBytes == 0`.
  - `TestCleaners_MaxAgeZero_AlwaysTTLEligible`: valida que com `MaxAgeDays: 0`, `TTLEligible == true` e `EligibleBytes == SizeBytes`.
- **GREEN**:
  - Atualizados `nextjs`, `node`, `rust`, `java`, `pnpm`, `homebrew`, `cpp`, `flutter`, `terraform`, `cocoapods`, `elixir` e `internal/cleaner/risk.go`.
  - Ajustado `nextjs_test.go` para validar desacoplamento de Scan vs TTL.
- **Commits**:
  - `46b34a5` `test(cleaner): add comprehensive mtime battery for project cleaners`
  - `742aac4` `feat(cleaner): decouple scan discovery from root mtime in project cleaners`

### Tarefa 4 [TDD]: Alinhamento de Consumo na CLI (`cmd/clean.go` e `cmd/format.go`)
- **RED**: Adicionados testes em `cmd/clean_test.go` e `cmd/format_test.go`:
  - `TestFilterByTTL_TTLEligibleDirectConsumption`
  - `TestFilterByTTL_TTLEligibleTakesPrecedenceOverMtime`
  - `TestFormatTable_Verbose_PartialEligibility`
  - `TestFormatJSON_IncludesEligibleFields`
- **GREEN**:
  - `cmd/clean.go`: atualizado `filterByTTL` para priorizar a avaliação `t.TTLEligible` já realizada pelo cleaner, mantendo fallback estrito apenas para alvos mock legados.
  - `cmd/format.go`: `formatTable` no modo verbose expõe a elegibilidade parcial e TTL quando `EligibleBytes < SizeBytes`.
- **Commits**:
  - `0947004` `test(cli): add tests for TTLEligible auto-clean filtering and format display`
  - `c3129b4` `feat(cli): consume TTLEligible in auto-clean and format real sizes`

---

## Verificação e Garantias de Qualidade
- **Zero CGO:** `CGO_ENABLED=0 go test ./...` aprovado com sucesso sem dependências CGO externas.
- **Sem regressões:** 100% dos testes unitários e de integração de todo o repositório (`cmd`, `internal/...`) passam.
- **Cobertura nos pacotes centrais:**
  - `internal/domain`: 100.0%
  - `internal/cleaner`: 70.8%
  - `cmd`: 89.3%

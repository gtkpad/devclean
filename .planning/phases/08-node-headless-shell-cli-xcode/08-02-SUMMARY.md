---
phase: "08-node-headless-shell-cli-xcode"
plan: "08-02"
title: "Shell and CLI Tools Cleaner Package"
wave: 1
completed_at: "2026-10-15"
requirements:
  - DEV-02
status: "completed"
commits:
  - hash: "7fe0b3d"
    message: "feat(shell): implement shell cleaner architecture and hybrid detection"
  - hash: "e9bff90"
    message: "feat(shell): implement granular CLI target scanning and non-interference"
  - hash: "6554711"
    message: "feat(shell): implement aggregated zsh completion cache and safe cleanup"
coverage:
  internal/cleaner/shell: "90.8%"
---

# Plan 08-02 Summary: Shell and CLI Tools Cleaner Package

## Overview
O plano `08-02` da Fase 08 foi executado com sucesso seguindo rigorosamente a disciplina TDD (RED-GREEN), satisfazendo o requisito `DEV-02` e implementando o novo cleaner `internal/cleaner/shell` sob o ecossistema `"shell"` com garantia estrita de não-interferência em configurações e históricos de shell (`D-06`, `D-08`).

### Principais Entregas
1. **Arquitetura do Cleaner Shell e Detecção Híbrida (DEV-02, D-06, D-10, SCAN-02):**
   - Criação do pacote `internal/cleaner/shell` implementando `domain.Cleaner` com identificador de ecossistema `"shell"`.
   - `New()` com executor padrão e `NewWithExecutor(cleaner.CommandExecutor)` para injeção de dependência e testabilidade hermética.
   - `IsInstalled()` híbrido: detecta ferramentas tanto pela presença de binários no PATH (`mise`, `gh`, `pre-commit`, `zsh`) quanto pela presença física em disco dos diretórios de cache (`mise`, `gh`, `pre-commit`, `omz`) ou arquivos `.zcompdump*` no `$HOME` e `$ZDOTDIR`.

2. **Inventário Granular de Caches de CLI e Classificação de Risco (DEV-02, D-07, D-09):**
   - Segmentação do `mise` em dois alvos distintos: `mise cache` (`~/Library/Caches/mise`, `~/.cache/mise`) com risco `RiskLow`, e `mise downloads` (`~/.local/share/mise/downloads`, `$XDG_DATA_HOME/mise/downloads`) com risco `RiskLow`.
   - Suporte ao GitHub CLI cache (`gh`, `RiskLow`) e Oh My Zsh cache (`~/.oh-my-zsh/cache`, `$ZSH_CACHE_DIR`, `RiskLow`).
   - Classificação diferenciada para o cache do `pre-commit` com `RiskMedium` para proteger desenvolvedores que realizam commits offline e evitar re-clonagem custosa de repositórios de hooks.

3. **Garantia Estrita de Não-Interferência (D-08, T-08-04):**
   - Arquivos de configuração de shell (`.zshrc`, `.bashrc`, `.profile`, `.bash_profile`, `config.fish`) e arquivos de histórico (`.zsh_history`, `.bash_history`) NUNCA são inventariados, lidos, modificados ou excluídos.
   - Teste unitário canário com verificação de integridade de conteúdo e timestamps confirma 0 mutação em arquivos sensíveis de shell.

4. **Agregação Consolidada de Dumps de Autocompletar do Zsh e Deleção Segura (DEV-02, D-08, VAL-01):**
   - Múltiplos arquivos matching `~/.zcompdump*` no `$HOME` e `$ZDOTDIR` são agrupados em um único alvo virtual `Zsh completion cache` com caminho virtual `shell://zsh/compdump` e ID `shell-zsh-compdump`.
   - `SizeBytes` reporta a soma exata de todos os arquivos de dump regulares.
   - `Clean()` decompõe o alvo virtual e remove com segurança cada arquivo `.zcompdump*` via `os.Remove`, ignorando diretórios e aplicando proteção do `safety.IsCoreBlacklisted`.
   - Deleção de alvos físicos validada com bloqueio contra travessia de links simbólicos (`safety.IsSymlink`) e proteção da blacklist do sistema (`safety.IsCoreBlacklisted`).

---

## Tarefas Executadas

### Task 08-02-01 [TDD]: Shell cleaner package architecture and hybrid disk-presence / PATH detection
- **RED**: Criado `internal/cleaner/shell/shell_test.go` com testes para `Name()`, `IsInstalled()` via LookPath, detecção puramente por presença em disco e caso vazio. Compilação falhou por ausência do pacote.
- **GREEN**: Criado `internal/cleaner/shell/shell.go` definindo `shellCleaner`, construtores `New` e `NewWithExecutor`, método `Name() == "shell"`, e `IsInstalled()` com varredura no PATH e caminhos padrão/XDG do disco.
- **Commit**: `7fe0b3d` `feat(shell): implement shell cleaner architecture and hybrid detection`

### Task 08-02-02 [TDD]: Granular target scanning: separate mise cache vs downloads, risk levels, and shell RC non-interference
- **RED**: Adicionados testes `TestShellCleaner_Scan_MiseSeparateTargets`, `TestShellCleaner_RiskLevels` e `TestShellCleaner_NonInterferenceWithShellRC`. Testes falharam por `Scan()` retornar vazio.
- **GREEN**: Implementados `getDirectoryCacheCandidates()`, `scanDirectoryCaches()` e `isExcluded()` com suporte a `mise cache`, `mise downloads`, `gh`, `pre-commit` (`RiskMedium`), `omz`, deduplicação por hash SHA-256 e respeito estrito a arquivos sensíveis de shell.
- **Commit**: `e9bff90` `feat(shell): implement granular CLI target scanning and non-interference`

### Task 08-02-03 [TDD]: Aggregated Zsh completion cache (~/.zcompdump*) and safe direct cleanup
- **RED**: Adicionados testes `TestShellCleaner_Scan_ZshCompletionAggregated`, `TestShellCleaner_Clean_ZshCompletionAggregated`, `TestShellCleaner_Clean_PhysicalCaches` e `TestShellCleaner_Clean_SymlinkProtection`. Falha confirmada para compdump aggregation e remoção de arquivos.
- **GREEN**: Implementado `scanZshCompdumps()` agregando múltiplos dumps em `shell://zsh/compdump`. Implementado `Clean()` tratando alvos virtuais `shell://zsh/compdump` com `os.Remove` seguro e alvos físicos com validações de `safety.IsCoreBlacklisted` e `safety.IsSymlink`. Adicionados testes de edge cases para env vars, TTL e exclusões.
- **Commit**: `6554711` `feat(shell): implement aggregated zsh completion cache and safe cleanup`

---

## Verificação e Cobertura
- **Testes Unitários:**
  `go test -v ./internal/cleaner/shell` executado com 100% de sucesso em 0.224s.
- **Cobertura de Código:**
  - `internal/cleaner/shell`: 90.8% dos statements (meta >85% superada).

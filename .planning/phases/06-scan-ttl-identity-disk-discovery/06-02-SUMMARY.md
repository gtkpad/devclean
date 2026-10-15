---
phase: "06-scan-ttl-identity-disk-discovery"
plan: "06-02"
title: "Wave 2 — Disk Presence Detection for IsInstalled and Dynamic ScanPaths Auto-Discovery"
wave: 2
completed_at: "2026-10-14"
requirements:
  - SCAN-02
  - SCAN-03
status: "completed"
commits:
  - hash: "cbb6c5f"
    message: "test(fs): add unit tests for DirExists"
  - hash: "9e94804"
    message: "feat(fs): implement DirExists helper for disk presence checks"
  - hash: "31d1d0e"
    message: "test(config): add unit tests for EffectiveScanPaths discovery and precedence"
  - hash: "9492b32"
    message: "feat(config): implement EffectiveScanPaths with deterministic order and engine injection"
  - hash: "70e0a46"
    message: "test(cleaner): add disk presence detection tests for toolchains without PATH binary"
  - hash: "dce6cfc"
    message: "feat(cleaner): implement hybrid disk presence detection in toolchain cleaners"
  - hash: "3d40d11"
    message: "test(cleaner): add shallow manifest detection tests for project cleaners"
  - hash: "0117283"
    message: "feat(cleaner): implement shallow manifest detection for project cleaners in ScanPaths"
coverage:
  internal/fs: "92.9%"
  internal/config: "93.6%"
  internal/engine: "96.6%"
  internal/cleaner: "76.3%"
---

# Plan 06-02 Summary: Wave 2 — Disk Presence Detection for IsInstalled and Dynamic ScanPaths Auto-Discovery

## Overview
O plano `06-02` da Fase 06 foi executado com sucesso e total conformidade com o protocolo TDD (RED-GREEN), cumprindo integralmente os requisitos `SCAN-02` e `SCAN-03` e as decisões arquiteturais `D-09` a `D-16`.

A partir desta entrega:
1. **Primitiva de Filesystem Segura (`fs.DirExists`, D-13, D-16):** Criado helper público e testado hermeticamente para validação estrita de diretórios existentes em disco com tratamento tolerante a erros de permissão, arquivos regulares e symlinks quebrados.
2. **Auto-Descoberta Dinâmica de ScanPaths (`EffectiveScanPaths`, SCAN-03, D-09 a D-12):**
   - Quando `cfg.ScanPaths` estiver vazio, inspeciona `$HOME` na sequência determinística estrita: `Documents`, `Projects`, `Developer`, `Code`, `Work`, `repos`, `workspace`.
   - Se nenhuma das 7 pastas existir, retorna `[]string{}` vazio, prevenindo varredura recursiva irrestrita de `$HOME`.
   - Precedência estrita da configuração: se o usuário fornecer `scan_paths` no YAML ou CLI, a auto-descoberta é desativada.
   - Imutabilidade do YAML garantida: a descoberta opera puramente em memória e não altera nem persiste dados em arquivo.
   - Injeção centralizada em `internal/engine/register.go` para todos os 12 cleaners baseados em projetos.
3. **Detecção Híbrida de Presença em Disco para Toolchains e Runtimes (SCAN-02, D-13, D-14, D-16):**
   - Cleaners de ecossistemas (`dotnet`, `golang`, `pnpm`, `homebrew`, `xcode`, `rust`, `python`, `bun`, `java`) reportam `IsInstalled() == true` se o executável estiver no `PATH` **OU** se os diretórios de cache padrão existirem fisicamente em disco.
   - Cleaners que operam diretamente no filesystem removem caches órfãos independentemente da presença do binário da CLI no sistema.
4. **Detecção por Manifestos Rasos em ScanPaths (SCAN-02, D-15, D-16):**
   - Implementado o helper de alto desempenho `cleaner.HasShallowManifest(paths, manifests...)`, que inspeciona apenas o nível raiz e subpastas de nível 1 imediato via `os.ReadDir` (limitado a 50 entradas por pasta), sem qualquer travessia recursiva (`filepath.WalkDir`).
   - Cleaners de projeto (`cpp`, `flutter`, `terraform`, `cocoapods`, `elixir`, `nextjs`) ativam-se automaticamente na presença de manifestos de projeto (`CMakeLists.txt`, `pubspec.yaml`, `*.tf`, `Podfile`, `mix.exs`, `next.config.js`).
5. **Zero CGO & Garantias de Segurança:** Suíte completa executada e aprovada com `CGO_ENABLED=0 go test ./...`.

---

## Tarefas Executadas

### Tarefa 1 [TDD]: Primitiva de Filesystem `fs.DirExists` (`internal/fs/stat.go`)
- **RED**: Criado `internal/fs/stat_test.go` testando caminho vazio, diretório real, arquivo regular, caminho inexistente, symlink quebrado e symlink válido para diretório.
- **GREEN**: Implementada a função pública `DirExists(path string) bool` em `internal/fs/stat.go`.
- **Commits**:
  - `cbb6c5f` `test(fs): add unit tests for DirExists`
  - `9e94804` `feat(fs): implement DirExists helper for disk presence checks`

### Tarefa 2 [TDD]: Descoberta Automática de ScanPaths em `internal/config` e Injeção no Engine
- **RED**: Adicionados testes `TestConfig_EffectiveScanPaths_DiscoveryOrder`, `TestConfig_EffectiveScanPaths_EmptyFallback`, `TestConfig_EffectiveScanPaths_ExplicitPrecedence` e `TestConfig_EffectiveScanPaths_Immutability` em `internal/config/config_test.go`.
- **GREEN**:
  - Declarado `CanonicalDiscoveryFolders` e implementado `(c *Config) EffectiveScanPaths() []string` em `internal/config/config.go`, com validação via `safety.IsCoreBlacklisted`.
  - Atualizado `RegisterDefaultCleanersWithExecutor` em `internal/engine/register.go` para injetar `cfg.EffectiveScanPaths()` em todos os cleaners de projeto.
  - Isolados testes em `internal/config/config_test.go` para manter execução hermética mesmo quando arquivos de configuração do host existem em `~/.config/devclean`.
- **Commits**:
  - `31d1d0e` `test(config): add unit tests for EffectiveScanPaths discovery and precedence`
  - `9492b32` `feat(config): implement EffectiveScanPaths with deterministic order and engine injection`

### Tarefa 3 [TDD]: Detecção Híbrida de Presença em Disco para Cleaners de Toolchains e Pacotes
- **RED**: Criado `TestCleaners_IsInstalled_NoBinaryInPath_CacheOnDisk` em `internal/cleaner/cleaner_installed_test.go` simulando ausência do binário em PATH e presença de diretório de cache em disco para 9 toolchains (`dotnet`, `golang`, `pnpm`, `homebrew`, `xcode`, `rust`, `python`, `bun`, `java`).
- **GREEN**:
  - `internal/cleaner/dotnet`: `IsInstalled()` checa `exec.LookPath("dotnet") == nil || devfs.DirExists(NuGet/HTTP/Temp)`.
  - `internal/cleaner/golang`: `IsInstalled()` checa `exec.LookPath("go") == nil || fs.DirExists(modCache/buildCache)`, preservando isolamento de env vars via `os.LookupEnv`.
  - `internal/cleaner/pnpm`: `IsInstalled()` checa `exec.LookPath("pnpm") == nil || devfs.DirExists(storePath)`.
  - `internal/cleaner/homebrew`: `IsInstalled()` checa `exec.LookPath("brew") == nil || devfs.DirExists(cacheDir)`.
  - `internal/cleaner/xcode`: `IsInstalled()` checa `xcrun` no PATH ou presença física de `DerivedData`.
  - `internal/cleaner/rust`: `IsInstalled()` checa `cargo` no PATH ou diretórios de registry/git em `CARGO_HOME`.
  - `internal/cleaner/python`: `IsInstalled()` checa binários no PATH ou presença de cache uv via `devfs.DirExists`.
  - `internal/cleaner/bun`: `IsInstalled()` checa `exec.LookPath("bun") == nil || devfs.DirExists(cachePath)`.
  - `internal/cleaner/java`: `IsInstalled()` checa `java/gradle/mvn` no PATH ou presença de `~/.gradle` e `~/.m2`.
- **Commits**:
  - `70e0a46` `test(cleaner): add disk presence detection tests for toolchains without PATH binary`
  - `dce6cfc` `feat(cleaner): implement hybrid disk presence detection in toolchain cleaners`

### Tarefa 4 [TDD]: Ativação por Manifestos Rasos em ScanPaths e Bateria de Testes de Presença
- **RED**: Adicionados `TestCleaners_IsInstalled_ManifestInScanPaths` e `TestCleaners_IsInstalled_InstantResponse` em `internal/cleaner/cleaner_installed_test.go`.
- **GREEN**:
  - Criado helper raso `HasShallowManifest(paths []string, manifests ...string) bool` em `internal/cleaner/shallow.go` inspecionando exclusivamente a raiz e filhos de nível 1 via `os.ReadDir` limitado a 50 entradas.
  - Atualizados `cpp`, `flutter`, `terraform`, `cocoapods`, `elixir` e `nextjs` para utilizar `HasShallowManifest`, eliminando totalmente as rotinas antigas que utilizavam `filepath.WalkDir` dentro de `IsInstalled()`.
  - Verificada performance de inicialização: todos os 18 cleaners juntos executam `IsInstalled()` em < 60ms (muito abaixo do teto de 100ms).
- **Commits**:
  - `3d40d11` `test(cleaner): add shallow manifest detection tests for project cleaners`
  - `0117283` `feat(cleaner): implement shallow manifest detection for project cleaners in ScanPaths`

---

## Verificação e Garantias de Qualidade
- **Zero CGO:** `CGO_ENABLED=0 go test ./...` aprovado com 100% de sucesso.
- **Isolamento e Hermeticidade:** Testes unitários limpos com manipulação de ambiente isolada (`t.TempDir()`, `t.Setenv()`).
- **Desempenho de Startup:** Checagens de instalação instantâneas (< 100ms para todos os 18 ecossistemas combinados).
- **Sem mutação de YAML:** Nenhuma descoberta de caminhos persiste alterações indesejadas no arquivo de configuração do usuário.
- **Cobertura nos pacotes centrais:**
  - `internal/fs`: 92.9%
  - `internal/config`: 93.6%
  - `internal/engine`: 96.6%
  - `internal/cleaner`: 76.3%

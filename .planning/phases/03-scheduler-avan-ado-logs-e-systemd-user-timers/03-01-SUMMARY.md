---
phase: "03-scheduler-avan-ado-logs-e-systemd-user-timers"
plan: "03-01"
title: "Wave 1 — Atalhos de Agendamento, Validação Prévia Dry-Run, Persistência de Argumentos e Resolução Canônica de LogDir"
wave: 1
completed_at: "2026-10-13"
requirements:
  - SCHED-01
  - SCHED-03
  - NFR-01
  - NFR-02
status: "completed"
commits:
  - hash: "5de3f3d"
    message: "feat(config): add SchedulerConfig and canonical LogDir resolution"
  - hash: "7d9b98e"
    message: "feat(cmd): add smart cron resolution and dry-run validation to schedule"
  - hash: "7f0613d"
    message: "feat(cmd): add scheduling flags, atomic config persistence, and CLI test suite"
coverage:
  internal/config: "94.4%"
  internal/platform: "90.5%"
  cmd: "89.8%"
---

# Plan 03-01 Summary: Wave 1 — Atalhos de Agendamento, Validação Prévia Dry-Run, Persistência de Argumentos e Resolução Canônica de LogDir

## Overview
A Wave 1 da Fase 03 foi concluída com êxito, entregando a modernização da interface CLI de agendamento (`devclean schedule`), persistência desacoplada de argumentos de limpeza (`scheduler.args`) via manipulação atômica de AST YAML preservando comentários, validação preventiva pré-registro via dry-run, atalhos semânticos de frequência com suporte à flag `--at HH:MM`, e resolução canônica de caminhos de log (`LogDir`) para macOS e Linux.

## Tarefas Executadas

### Tarefa 1: Modelo de Configuração `SchedulerConfig.Args` e Resolução de `LogDir` Canônico [TDD]
- **Modelo de Configuração (`internal/config/config.go`, `defaults.go`, `validate.go`)**:
  - Adicionada struct `SchedulerConfig` com campo `Args []string` anotado para Viper/mapstructure.
  - Integrado campo `Scheduler SchedulerConfig` à struct `Config` com default `[]string{}` compilado em `Defaults()`.
  - Registrado default em `Load()` via `v.SetDefault("scheduler.args", defaults.Scheduler.Args)`.
  - Implementada validação de segurança em `Validate()`: rejeição estrita de caracteres de injeção de shell (`;`, `|`, `&`, `$`, `` ` ``, `<`, `>`, `\n`, `\r`) e validação de flags/valores contra whitelist de comandos de limpeza permitidos (`--preset`, `safe`, `standard`, `aggressive`, `--dry-run`, etc.), mitigando ameaça `T-03-01`.
- **Resolução de Caminhos (`internal/platform/resolver.go`, `resolver_darwin.go`, `resolver_linux.go`)**:
  - Campo `LogDir string` adicionado à struct `Paths`.
  - Implementadas funções canônicas `ResolveDarwinLogDir` (`~/Library/Logs/devclean`) e `ResolveLinuxLogDir` (`~/.local/state/devclean/logs` ou `$XDG_STATE_HOME/devclean/logs`), respeitando `D-09`.
  - Atualizados resolvers Darwin e Linux para preencher `LogDir` com caminhos canônicos e limpos (`filepath.Clean`).
- **Cobertura de Testes**:
  - `internal/config`: 94.4% de cobertura de statements.
  - `internal/platform`: 90.5% de cobertura de statements.
- **Commit**: `5de3f3d` (`feat(config): add SchedulerConfig and canonical LogDir resolution`).

### Tarefa 2: Expressões Cron Inteligentes, Parser `--at` e Validação Prévia Dry-Run (`cmd/schedule.go`) [TDD]
- **Atalhos e Horários (`cmd/schedule.go`)**:
  - Implementada função `resolveCronExpression(daily, weekly, hourly bool, cron string, atTime string) (string, error)`:
    - `--weekly` resolve para `0 0 * * 0` (Domingo 00:00).
    - `--hourly` resolve para `0 * * * *` (Minuto 0 de cada hora).
    - `--daily` resolve para `0 12 * * *` (Meio-dia).
    - Validação de exclusividade entre atalhos e `--cron`.
  - Implementada função `applyAtTime(baseCron string, atTime string) (string, error)`:
    - Validação estrita de formato 24h (`HH:MM`, 00-23 e 00-59).
    - Substituição dos campos de minuto e hora na expressão cron base (ex: `--daily --at 04:30` -> `30 4 * * *`).
    - Tratamento de `--at` com `--hourly` para aceitar minutos (`MM` ou `:MM`).
- **Validação Prévia Dry-Run (`cmd/schedule.go`)**:
  - Implementada função `validateScheduledRun(cleanArgs []string, skipValidation bool) error`:
    - Simula execução de `clean --auto --dry-run` antes de registrar no sistema operacional, prevenindo falhas silenciosas (`D-04`).
    - Suporte a bypass via flag `--skip-validation`.
    - Isolamento de execução via hook `dryRunExecutor` permitindo testes unitários herméticos.
- **Commit**: `7d9b98e` (`feat(cmd): add smart cron resolution and dry-run validation to schedule`).

### Tarefa 3: Flags CLI, Persistência Atômica de `scheduler.args` no `config.yaml` e Suíte de Testes da CLI (`cmd/schedule.go`, `cmd/schedule_test.go`)
- **Flags Adicionadas ao `devclean schedule`**:
  - `--weekly`: agendamento semanal (domingo 00:00).
  - `--hourly`: agendamento a cada hora (minuto 0).
  - `--daily`: agendamento diário (12:00).
  - `--at`: horário específico no formato `HH:MM`.
  - `--preset`: preset de limpeza para o agendador (`safe`, `standard`, `aggressive`).
  - `--args`: argumentos adicionais para o comando de limpeza.
  - `--skip-validation`: bypass da simulação dry-run.
  - `--provider`: seleção manual de provedor (`systemd` ou `cron`).
- **Persistência Atômica no `config.yaml`**:
  - Implementada função `updateSchedulerArgs(configFile string, args []string) error` utilizando a biblioteca `yaml.Node` (`gopkg.in/yaml.v3`).
  - Atualização seletiva da chave `scheduler.args`, preservando integralmente comentários, nós adjacentes e recuo do arquivo de configuração (`T-03-02`).
- **Injeção de Dependências & Suíte de Testes CLI**:
  - Introduzido `schedulerFactory` permitindo mockar `scheduler.Scheduler` em testes de comando.
  - Testes cobrindo: agendamento com atalhos e flags, persistência no `config.yaml` com preservação de comentários, interrupção em caso de erro no dry-run, bypass com `--skip-validation`, e validação do provedor.
- **Cobertura de Testes**:
  - `cmd`: 89.8% de cobertura de statements.
- **Commit**: `7f0613d` (`feat(cmd): add scheduling flags, atomic config persistence, and CLI test suite`).

## Verificação e Conformidade
- **Zero CGO (`NFR-01`)**:
  - Compilação cruzada para Linux estática executada com sucesso:
    `CGO_ENABLED=0 GOOS=linux go build ./...` (Exit code: 0).
- **Cobertura de Testes (`NFR-02`)**:
  - `internal/config`: 94.4% (>85%)
  - `internal/platform`: 90.5% (>85%)
  - `cmd`: 89.8% (>85%)
  - Suíte global do projeto executada sem regressões (`go test -v -cover ./...` PASS).
- **Salvaguardas de Segurança**:
  - Mitigação `T-03-01`: Whitelist estrita de argumentos em `internal/config/validate.go`.
  - Mitigação `T-03-02`: Mutação de AST de nós YAML em `cmd/schedule.go` preservando comentários.
  - Mitigação `T-03-SC`: Sem novas dependências externas; uso estrito de pacotes stdlib e Go puro já existentes.

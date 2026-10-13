---
phase: "03-scheduler-avan-ado-logs-e-systemd-user-timers"
plan: "03-03"
title: "Wave 2 — Rotação Circular de 10 MB, Parser Estruturado, Fiação de Provedor e Subcomando Interno devclean schedule run"
wave: 2
completed_at: "2026-10-13"
requirements:
  - SCHED-02
  - SCHED-03
  - NFR-01
  - NFR-02
status: "completed"
commits:
  - hash: "4c9e754"
    message: "feat(scheduler): implement 10 MB circular log rotation and real-time stream tailer"
  - hash: "ad48f54"
    message: "feat(scheduler): implement structured log parser for delimited execution blocks"
  - hash: "0def7c4"
    message: "feat(cmd): implement schedule run internal command and provider wiring"
coverage:
  internal/scheduler: "91.8%"
  cmd: "88.2%"
---

# Plan 03-03 Summary: Wave 2 — Rotação Circular de 10 MB, Parser Estruturado, Fiação de Provedor e Subcomando Interno devclean schedule run

## Overview
A Wave 2 da Fase 03 foi concluída com êxito, entregando a espinha dorsal de execução e auditoria periódica do DevClean: motor circular hermético de rotação de logs com teto fixo de 10 MB e até 3 backups históricos (`rotator.go`), motor de streaming resiliente com detecção automática de rotação e truncamento (`tailer.go`), parser estruturado de blocos de execução delimitados por marcadores START/END (`log_parser.go`), subcomando interno e unificador `devclean schedule run` (`cmd/schedule_run.go`), fiação multiplataforma da flag `--provider` com validação prévia de incompatibilidade, e sincronização dos templates de Launchd e Crontab.

## Tarefas Executadas

### Tarefa 1: Rotação Circular de Log (10 MB / 3 Backups) e Motor de Streaming em Tempo Real (`internal/scheduler`) [TDD]
- **Motor de Rotação (`internal/scheduler/rotator.go`)**:
  - Exportadas constantes `DefaultMaxLogSizeBytes = 10 * 1024 * 1024` (10 MB) e `DefaultMaxBackups = 3`.
  - Implementada função `RotateIfNeeded(logPath string, maxBytes int64, maxBackups int) error`.
  - Salvaguarda contra ataques de symlink (`T-03-06`): detecção via `os.Lstat` e remoção automática de symlinks antes da rotação ou gravação.
  - Comportamento circular determinístico (`D-10`): quando o arquivo atinge ou supera `maxBytes`, expurga o backup mais antigo (`schedule.3.log`), rotaciona backups existentes (`2 -> 3`, `1 -> 2`) e move o log ativo para `schedule.1.log`.
- **Motor de Streaming (`internal/scheduler/tailer.go`)**:
  - Exportada função `StreamLog(ctx context.Context, logPath string, out io.Writer) error`.
  - Polling a cada 100ms lendo linhas incrementalmente sem travamento.
  - Detecção transparente de rotação (`!os.SameFile`) ou truncamento (`newFi.Size() < curPos`), reabrindo o arquivo ativo graciosamente (`D-14`).
  - Posicionamento inteligente: inicia no final caso o arquivo já exista no momento da chamada, ou do início caso o arquivo seja criado posteriormente ou rotacionado.
  - Encerramento limpo via cancelamento de `context.Context`.
- **Cobertura & Commit**:
  - `rotator_test.go` e `tailer_test.go` cobrindo rotações circulares, descarte de arquivos além de `maxBackups`, escrita concorrente e encerramento.
  - **Commit**: `4c9e754` (`feat(scheduler): implement 10 MB circular log rotation and real-time stream tailer`).

### Tarefa 2: Parser Estruturado de Marcadores START/END e Auditoria de Execução (`internal/scheduler`) [TDD]
- **Modelo Estruturado de Execução (`internal/scheduler/log_parser.go`)**:
  - Definida struct `LogRun` com campos `StartTime`, `EndTime`, `Version`, `Args`, `Status` (`OK`, `PARTIAL`, `FAIL`), `TargetsCount`, `BytesFreed`, `FreedHuman`, `Duration`, `Body`, e `ErrorLines`.
  - Implementada função `ParseLogRuns(r io.Reader) ([]LogRun, error)`.
  - Suporte a buffer expansível (até 10 MB por token) para linhas longas de output.
  - Parsing de marcadores canônicos (`D-11`):
    - Cabeçalho: `=== [START] YYYY-MM-DD HH:MM:SS | version=... | args=[...] ===`
    - Rodapé: `=== [END] YYYY-MM-DD HH:MM:SS | status=... | targets=... | freed=... | duration=... ===`
  - Extração inteligente de linhas de erro no corpo (`[ERROR]`, `error:`, `permission denied`).
  - Conversor de tamanho `ParseFreedBytes` para transformar strings legíveis (`145.2 MB`, `10.5 KB`, etc.) em bytes inteiros.
  - Resiliência comprovada contra streams vazios, linhas corrompidas ou execuções interrompidas abruptamente (sem marcador `END`), classificadas como `FAIL`.
- **Cobertura & Commit**:
  - `log_parser_test.go` cobrindo cenários OK, PARTIAL com erros de permissão, FAIL, múltiplos blocos encadeados, arquivos corrompidos e linhas gigantescas.
  - **Commit**: `ad48f54` (`feat(scheduler): implement structured log parser for delimited execution blocks`).

### Tarefa 3: Subcomando Interno `devclean schedule run`, Fiação de `--provider` e Registro em `cmd/schedule.go` [TDD]
- **Subcomando Unificador `devclean schedule run` (`cmd/schedule_run.go`)**:
  - Subcomando oculto (`Hidden: true`) implementando o ciclo de vida completo de execuções automatizadas (`D-12`).
  - Ciclo de execução:
    1. Resolução canônica de `LogDir` via `platform.Resolver` e criação segura com permissão `0755`.
    2. Rotação preventiva antes de iniciar o ciclo.
    3. Proteção e desvinculação de symlinks caso existam.
    4. Abertura de `schedule.log` em modo append (`0644`).
    5. Gravação de cabeçalho delimitado `=== [START] ... ===`.
    6. Execução de subprocesso `devclean clean --auto` repassando argumentos persistidos em `cfg.Scheduler.Args` (`D-01`).
    7. MultiWriter gravando stdout/stderr em tempo real no arquivo de log e capturando saída para auditoria.
    8. Cálculo de status (`OK`, `PARTIAL` se houver erros de permissão, `FAIL` se código de saída diferente de zero).
    9. Gravação de rodapé delimitado `=== [END] ... ===`.
    10. Atualização de `last-run.json` via `scheduler.WriteLastRun`.
    11. Rotação pós-execução preventiva caso o limite de 10 MB tenha sido superado.
- **Fiação de `--provider` e Registro de Subcomando (`cmd/schedule.go`)**:
  - Registrado `cmd.AddCommand(newScheduleRunCmd())` em `newScheduleCmd()`.
  - Factory atualizada para `schedulerFactory = func(configDir, provider string) (scheduler.Scheduler, error)` chamando `scheduler.NewWithProvider(configDir, provider)` (`D-06`).
  - Validação antecipada de compatibilidade de provedor: rejeição amigável e imediata caso o provedor solicitado seja inválido ou incompatível com a plataforma atual (ex.: `systemd` no macOS), abortando antes de qualquer modificação em arquivos de configuração ou validações dry-run.
- **Sincronização de Provedores Nativos (`launchd_scheduler.go`, `cron_scheduler.go`, `cron_helpers.go`)**:
  - Atualizado template plist do Launchd para executar `[binaryPath, "schedule", "run"]` em vez de `clean --auto`.
  - Atualizado gerador de crontab para emitir `binaryPath + " schedule run"`.
  - Adicionado helper `WriteLastRun` em `lastrun.go`.
- **Cobertura & Commit**:
  - `schedule_run_test.go` e `schedule_test.go` cobrindo ciclo de vida completo, repasse de flags, escrita de marcadores, persistência em `last-run.json`, resiliência a erros no subprocesso, mitigação de symlinks, e fiação de `--provider`.
  - **Commit**: `0def7c4` (`feat(cmd): implement schedule run internal command and provider wiring`).

## Verificação e Conformidade
- **Zero CGO (`NFR-01`)**:
  - Compilação cruzada estática verificada para Linux:
    `CGO_ENABLED=0 GOOS=linux go build ./...` (Exit code: 0).
- **Cobertura de Testes (`NFR-02`)**:
  - `internal/scheduler`: 91.8% de cobertura de statements (>85%).
  - `cmd`: 88.2% de cobertura de statements (>85%).
  - Suíte completa do repositório (38 pacotes) executada com 100% de sucesso.
- **Salvaguardas de Segurança**:
  - Mitigação `T-03-05`: Teto circular estrito de 10 MB por arquivo com no máximo 3 backups, garantindo que o diretório de logs nunca ultrapasse 40 MB.
  - Mitigação `T-03-06`: Verificação de symlinks via `os.Lstat` no rotador e no runner, descartando links manipulados antes da escrita.
  - Mitigação `T-03-SC`: Implementação hermética em Go puro sem novas dependências externas.

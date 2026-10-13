---
phase: "03-scheduler-avan-ado-logs-e-systemd-user-timers"
plan: "03-02"
title: "Wave 1 — Systemd User Timers, Conversor OnCalendar e Fallback Dinâmico no Linux (internal/scheduler)"
wave: 1
completed_at: "2026-10-13"
requirements:
  - SCHED-02
  - NFR-01
  - NFR-02
status: "completed"
commits:
  - hash: "81066de"
    message: "feat(scheduler): add CronToOnCalendar converter for systemd user timers"
  - hash: "c7b9bd5"
    message: "feat(scheduler): implement systemdScheduler for Linux user timers with rollback and linger support"
  - hash: "ee1fbf7"
    message: "feat(scheduler): add dynamic systemd detection with cron fallback and NewWithProvider factory"
coverage:
  internal/scheduler: "91.4%"
---

# Plan 03-02 Summary: Wave 1 — Systemd User Timers, Conversor OnCalendar e Fallback Dinâmico no Linux

## Overview
A execução do Plano 03-02 foi concluída com êxito, entregando o suporte completo e moderno a Systemd User Timers no Linux (`devclean.service` e `devclean.timer`), conversor de sintaxe cron de 5 campos para especificações `OnCalendar` do systemd com suporte a ranges e steps, garantias de produção como resiliência pós-suspensão (`Persistent=true`) e jitter de inicialização (`RandomizedDelaySec=5m`), detecção automática de ambiente com fallback silencioso para `crontab`, e factory multiplataforma `NewWithProvider` com suporte a override `--provider` no Linux e validação estrita no macOS.

## Tarefas Executadas

### Tarefa 1: Conversor de Sintaxe Cron de 5 Campos para Systemd `OnCalendar` (`systemd_timer.go`, `systemd_timer_test.go`) [TDD]
- **Implementação do Conversor (`internal/scheduler/systemd_timer.go`)**:
  - Exportada função `CronToOnCalendar(cronExpr string) (string, error)`.
  - Normalização prévia de múltiplos espaços e tabs.
  - Mapeamento determinístico de dias da semana (cron 0/7 = `Sun`, 1 = `Mon`, ..., 6 = `Sat`) com suporte a ranges (ex: `1-5` -> `Mon..Fri`) e listas (`1,3,5` -> `Mon,Wed,Fri`).
  - Formatação e padding de campos numéricos de 2 dígitos para dia, mês, hora e minuto.
  - Suporte a intervalos com passo para minutos (`*/15 * * * *` -> `*-*-* *:00/15:00`).
  - Tratamento e validação estrita de limites e rejeição de injeção de shell via `ParseCron`.
- **Cobertura de Testes**:
  - `systemd_timer.go`: >97% de cobertura de statements.
- **Commit**: `81066de` (`feat(scheduler): add CronToOnCalendar converter for systemd user timers`).

### Tarefa 2: Provedor Nativo de Agendamento `systemdScheduler` com Resiliência e Linger (`systemd_scheduler.go`, `systemd_scheduler_test.go`) [TDD]
- **Implementação do Provedor (`internal/scheduler/systemd_scheduler.go`)**:
  - Implementada struct `systemdScheduler` satisfazendo integralmente a interface `Scheduler` (`Install`, `Remove`, `Status`, `IsInstalled`, `InstallWatch`, `RemoveWatch`, `IsWatchInstalled`).
  - Construtor `NewSystemdSchedulerWithExecutor(configDir string, exec CommandExecutor, unitsDir string) Scheduler` com injeção de executor e suporte a diretório customizado de units.
  - Template `devclean.service`: `Type=oneshot` e `ExecStart={{.BinaryPath}} schedule run` (D-05, T-03-03).
  - Template `devclean.timer`: `OnCalendar={{.OnCalendar}}`, `Persistent=true`, `RandomizedDelaySec=5m` e `WantedBy=timers.target` (D-08).
  - Template `devclean-watch.service`: `Type=simple`, `ExecStart={{.BinaryPath}} --watch-daemon`, `Restart=on-failure`.
  - Escrita com permissão `0644` no diretório de units de usuário `~/.config/systemd/user/` (ou `unitsDir` nos testes).
  - Ativação via `CommandExecutor` com `systemctl --user daemon-reload` e `systemctl --user enable --now devclean.timer` (D-07).
  - Rollback atômico em caso de falha em qualquer chamada ao `systemctl`.
  - Métodos `Remove()` e `Status()` com extração de metadados (`OnCalendar`, caminho do binário) e leitura de `lastrun.json`.
- **Cobertura de Testes**:
  - `systemd_scheduler.go`: >90% de cobertura de statements.
- **Commit**: `c7b9bd5` (`feat(scheduler): implement systemdScheduler for Linux user timers with rollback and linger support`).

### Tarefa 3: Detecção de Ambiente Linux, Fallback Silencioso para Cron, Suporte a Override `--provider` e Paridade Multiplataforma (`scheduler_linux.go`, `scheduler_darwin.go`, `scheduler.go`, `systemd_detect.go`) [TDD]
- **Constantes de Provedor & Mensagem de Linger (`internal/scheduler/scheduler.go`)**:
  - Exportadas constantes `ProviderSystemd = "systemd"`, `ProviderCron = "cron"`, `ProviderLaunchd = "launchd"`, `ProviderAuto = "auto"`.
  - Exportada constante `LingerHint = "Dica: Para garantir que o timer execute mesmo quando você não estiver logado, habilite o linger com: loginctl enable-linger $USER"` (D-07).
- **Detecção Resiliente & Factory Linux (`internal/scheduler/systemd_detect.go`, `scheduler_linux.go`)**:
  - Implementada função `IsSystemdAvailable(exec CommandExecutor) bool` inspecionando `systemctl --user is-system-running`. Retorna `true` para estados operacionais (`running`, `degraded`, `starting`, `initializing`), e `false` caso ausente, desconectado ou com erro.
  - Implementado `newLinuxSchedulerWithExecutor(configDir string, provider string, exec CommandExecutor) (Scheduler, error)`:
    - `provider == "systemd"`: força `systemdScheduler`.
    - `provider == "cron"`: força `cronScheduler`.
    - `provider == ""` ou `"auto"`: seleciona `systemdScheduler` se `IsSystemdAvailable`, ou fallback transparente para `cronScheduler` (D-06).
    - `provider == "launchd"`: retorna erro explicativo sobre incompatibilidade no Linux.
  - Atualizado `scheduler_linux.go` com `New(configDir)` e `NewWithProvider(configDir, provider)`.
- **Factory Darwin (`internal/scheduler/scheduler_darwin.go`)**:
  - Implementado `NewWithProvider(configDir string, provider string) (Scheduler, error)`:
    - `""`, `"launchd"`, `"auto"`: instancia `launchdScheduler`.
    - `"systemd"`, `"cron"`: retorna erro explicativo informando que launchd é o único suportado no macOS.
    - Outros provedores: rejeita com erro explicativo.
- **Cobertura de Testes**:
  - `systemd_detect.go`: >90% de cobertura de statements.
  - `scheduler_darwin.go`: 100% de cobertura de statements.
- **Commit**: `ee1fbf7` (`feat(scheduler): add dynamic systemd detection with cron fallback and NewWithProvider factory`).

## Verificação e Conformidade
- **Zero CGO (`NFR-01`)**:
  - Compilação cruzada estática para Linux verificada:
    `CGO_ENABLED=0 GOOS=linux go build ./...` (Exit code: 0).
    `CGO_ENABLED=0 GOOS=linux go test -c ./internal/scheduler -o /dev/null` (Exit code: 0).
- **Cobertura de Testes (`NFR-02`)**:
  - Cobertura global do pacote `internal/scheduler`: 91.4% (>85%).
  - Todos os 11 casos de teste de systemd e detecção aprovados hermeticamente sem dependência de daemons de sistema operacional reais.
- **Salvaguardas de Segurança**:
  - Mitigação `T-03-03`: `ExecStart` invoca estritamente o executável com subcomando fixo `schedule run` sem concatenação de strings ou execução via shell.
  - Mitigação `T-03-04`: Todas as units e comandos operam exclusivamente em escopo de usuário (`~/.config/systemd/user/` e `systemctl --user`), sem privilégios de root.
  - Mitigação `T-03-SC`: Zero dependências externas adicionadas; uso estrito da stdlib Go e injeção via `CommandExecutor`.

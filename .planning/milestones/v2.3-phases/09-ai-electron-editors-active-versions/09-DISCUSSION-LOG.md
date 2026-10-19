# Phase 09: AI/Electron Editors & Active Versions - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-10-15  
**Phase:** 09-AI/Electron Editors & Active Versions  
**Areas discussed:** Granularidade e Agrupamento dos Editores, Process Guard de Editores, Versões do Claude CLI, Workspace Storage e Extensões Obsoletas  

---

## Granularidade e Agrupamento dos Editores

| Option | Description | Selected |
|--------|-------------|:--------:|
| Agrupamento funcional por editor | 3 a 4 alvos por editor (Caches Chromium/GPU, Logs & Crashpad, Extensões/VSIXs, Workspace Backups) | ✓ |
| Alvo único consolidado por editor | 1 alvo por editor reunindo todas as folhas de cache regeneráveis | |
| Folhas totalmente granulares | 1 alvo individual para cada diretório leaf (Cache, GPUCache, DawnCache, Logs, etc.) | |
| You decide | Deixar a cargo do planejador seguir o melhor equilíbrio | |

**User's choice:** Agrupamento funcional por editor: 3 a 4 alvos por editor (Caches Chromium/GPU, Logs & Crashpad, Extensões/VSIXs, Workspace Backups)  
**Notes:** Fornece o equilíbrio ideal entre visibilidade de consumo e clareza na CLI/TUI.

| Option | Description | Selected |
|--------|-------------|:--------:|
| Mapeamento canônico específico por editor | Zed mapeia caches nativos/logs (`~/Library/Caches/dev.zed.Zed`, `~/.cache/zed`) e OpenCode mapeia `~/.cache/opencode`, enquanto os editores baseados em Electron/VS Code mapeiam suas folhas Chromium completas | ✓ |
| Esquema genérico uniforme | Procurar as mesmas subpastas em todos os 8 editores e ignorar as inexistentes | |
| You decide | Aplicar a melhor correspondência por plataforma | |

**User's choice:** Mapeamento canônico específico por editor  
**Notes:** Respeita a arquitetura específica do Zed (GPUI) e OpenCode.

| Option | Description | Selected |
|--------|-------------|:--------:|
| Centralizar em `internal/cleaner/ai/editors.go` | Sob o ecossistema `ai` já existente | ✓ |
| Criar um cleaner e ecossistema separado `internal/cleaner/editors` | Independente de `ai` | |
| You decide | | |

**User's choice:** Centralizar em `internal/cleaner/ai/editors.go` sob o ecossistema `ai` já existente  
**Notes:** Mantém a simplicidade de registro e alinhamento com a Frente 2 da especificação.

| Option | Description | Selected |
|--------|-------------|:--------:|
| Diferenciar risco | `RiskLow` para caches regeneráveis gerais (Chromium, GPU, Code Cache, logs, Crashpad) e `RiskMedium` para backups de workspace (`state.vscdb.backup`) e `CachedExtensionVSIXs` | ✓ |
| Risco uniforme `RiskLow` | Para todas as folhas regeneráveis autorizadas pela especificação | |
| You decide | | |

**User's choice:** Diferenciar risco (`RiskLow` para caches gerais, `RiskMedium` para backups e VSIXs)  
**Notes:** Garante precaução adequada ao manipular backups de estado e instaladores de extensões.

---

## Process Guard de Editores

| Option | Description | Selected |
|--------|-------------|:--------:|
| Usar `cleaner.CommandExecutor` com `pgrep -x` | Nomes de processos principais de cada editor (`Code`, `Cursor`, `Antigravity`, `Windsurf`, `Claude`, `zed`, `opencode`), injetável e hermético | ✓ |
| Usar `pgrep -f` buscando helpers | Substrings completas de executáveis dos editores | |
| You decide | | |

**User's choice:** Usar `cleaner.CommandExecutor` com `pgrep -x`  
**Notes:** Segue o padrão testável e hermético sem CGO estabelecido na Fase 8.

| Option | Description | Selected |
|--------|-------------|:--------:|
| Bloqueio isolado por editor | Apenas os alvos do editor ativo/incerto recebem `ReviewOnly = true` (`ReviewReasonActiveProcess`) e `EligibleBytes = 0`, enquanto editores inativos continuam elegíveis | ✓ |
| Bloqueio global | Se qualquer editor estiver aberto, bloqueia todos os editores com ReviewOnly | |
| You decide | | |

**User's choice:** Bloqueio isolado por editor  
**Notes:** Permite que editores fechados sejam limpos normalmente sem falso impedimento por outros abertos.

| Option | Description | Selected |
|--------|-------------|:--------:|
| Revalidação imediata pré-exclusão | No `Clean()`, re-checar se o editor correspondente está em execução antes de apagar e abortar/ignorar o alvo caso tenha sido aberto após o scan | ✓ |
| Confiar apenas no status do Scan | Assumir que a filtragem do Coordinator e verificação inicial são suficientes | |
| You decide | | |

**User's choice:** Revalidação imediata pré-exclusão  
**Notes:** Previne condições de corrida quando o usuário abre o editor entre o scan e a limpeza.

| Option | Description | Selected |
|--------|-------------|:--------:|
| Fail-closed estrito | Se `pgrep` não estiver disponível ou falhar por erro inesperado, classificar como estado incerto e aplicar `ReviewOnly` nos alvos de editores | ✓ |
| Fallback no Linux via `/proc` | Antes de falhar fechado se `pgrep` não existir | |
| You decide | | |

**User's choice:** Fail-closed estrito  
**Notes:** Garante segurança absoluta de dados em qualquer ambiente onde a checagem falhar.

---

## Versões do Claude CLI

| Option | Description | Selected |
|--------|-------------|:--------:|
| Resolução via symlinks canônicos | Inspecionar symlinks ativos (ex: `current` ou symlink do binário em `~/.local/share/claude`), resolver o destino real com `filepath.EvalSymlinks` e proteger essa versão da deleção | ✓ |
| Preservar apenas a versão com maior timestamp/semver | Baseado puramente nos nomes das pastas | |
| You decide | | |

**User's choice:** Resolução via symlinks canônicos  
**Notes:** Impede a exclusão acidental da versão apontada e em uso.

| Option | Description | Selected |
|--------|-------------|:--------:|
| Falhar fechado via `ReviewOnly` | Marcar as versões antigas como `ReviewOnly = true`, `EligibleBytes = 0` e razão de revisão explícita (`ReviewReasonActiveVersion`/`ReviewReasonAmbiguous`), bloqueando qualquer exclusão automática | ✓ |
| Omitir completamente os alvos de versões | Quando houver ambiguidade ou symlink quebrado | |
| You decide | | |

**User's choice:** Falhar fechado via `ReviewOnly`  
**Notes:** Mantém visibilidade de inventário completa sem autorizar mutação destrutiva.

| Option | Description | Selected |
|--------|-------------|:--------:|
| Sim (Process Guard no Claude CLI) | Verificar processo ativo `claude` via `pgrep -x claude` e marcar alvos do Claude com `ReviewOnly = true` (`ReviewReasonActiveProcess`) se estiver em execução | ✓ |
| Não | Restringir o Process Guard apenas aos editores com GUI/Electron | |
| You decide | | |

**User's choice:** Sim (verificar processo ativo `claude`)  
**Notes:** Protege sessões interativas ativas do Claude Code CLI.

| Option | Description | Selected |
|--------|-------------|:--------:|
| `RiskLow` | A versão ativa está garantida e binários antigos são sobras de autoupdate substituíveis | ✓ |
| `RiskMedium` | Sinalizando que rollbacks offline para versões antigas dependeriam de nova instalação | |
| You decide | | |

**User's choice:** `RiskLow`  
**Notes:** Permite recuperação direta de centenas de megabytes residuais sem barreiras desnecessárias.

---

## Workspace Storage e Extensões Obsoletas

| Option | Description | Selected |
|--------|-------------|:--------:|
| Remoção cirúrgica de arquivos | Varrer `User/workspaceStorage/*/state.vscdb.backup` e selecionar estritamente os arquivos `.backup`, preservando intacto o `state.vscdb` primário e toda a estrutura de workspaces | ✓ |
| Remover pastas inteiras de workspaces inativos | Pastas sem uso há mais de X dias | |
| You decide | | |

**User's choice:** Remoção cirúrgica de arquivos  
**Notes:** Protege estados, layouts e histórico do workspace de projetos.

| Option | Description | Selected |
|--------|-------------|:--------:|
| Cobrir raízes de extensões de todos os editores VS Code-family presentes | `~/.vscode/extensions`, `~/.vscode-insiders/extensions`, `~/.cursor/extensions`, `~/.antigravity/extensions`, `~/.windsurf/extensions`, mantendo `.obsolete` intocado e validando estritamente symlinks e path traversal | ✓ |
| Inspecionar apenas `~/.vscode/extensions` e `~/.cursor/extensions` | | |
| You decide | | |

**User's choice:** Cobrir raízes de extensões de todos os editores VS Code-family presentes  
**Notes:** Oferece cobertura uniforme para todos os forks suportados.

| Option | Description | Selected |
|--------|-------------|:--------:|
| Padrão v2.3 unificado | `Scan()` reporta 100% do tamanho em `SizeBytes`; `EligibleBytes` e `TTLEligible` respeitam o TTL (`MaxAgeDays == 0` ou idade > cutoff tornam elegível) | ✓ |
| Elegibilidade imediata fixa | Considerar extensões obsoletas elegíveis sem aplicar filtro de idade | |
| You decide | | |

**User's choice:** Padrão v2.3 unificado  
**Notes:** Mantém conformidade estrita com o desacoplamento de Scan e TTL estabelecido na Fase 6.

| Option | Description | Selected |
|--------|-------------|:--------:|
| Proteção positiva estrita | Direcionar exclusivamente pastas com entrada `true` no `.obsolete`, `CachedExtensionVSIXs` e `webviewCache`; todas as extensões ativas em `extensions/` permanecem intocáveis | ✓ |
| Verificar data de modificação da pasta da extensão instalada | | |
| You decide | | |

**User's choice:** Proteção positiva estrita  
**Notes:** Garante que extensões ativas jamais sejam apagadas.

---

## the agent's Discretion

- Formatação de IDs e nomes exatos de alvos em `domain.Target`.
- Detalhes de subdiretórios de logs do Claude Desktop.
- Helpers de travessia e checagem de symlinks para `.obsolete`.

## Deferred Ideas

None — discussion stayed within phase scope.

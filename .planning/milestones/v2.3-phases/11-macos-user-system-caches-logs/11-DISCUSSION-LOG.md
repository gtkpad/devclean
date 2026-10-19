# Phase 11: macOS User/System Caches & Logs - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-10-16  
**Phase:** 11-macOS User/System Caches & Logs  
**Areas discussed:** Trash & Volumes Externos, Granularidade e Deduplicação de User Caches, Permissões TCC e Degradação Graciosa em Logs/Sistema, Positive Allowlists & Proteção de Dados Sensíveis  

---

## Trash & Volumes Externos

### Questão 1: Elegibilidade e visualização da Lixeira do macOS (`~/.Trash`)

| Option | Description | Selected |
|--------|-------------|:--------:|
| Exibir como ReviewOnly | Exibir `~/.Trash` no scan sempre como ReviewOnly (`⊙`), tornando-o elegível para remoção apenas com `--trash` (ou `safety.trash_enabled`) | ✓ |
| Omitir a menos que --trash | Omitir `~/.Trash` completamente da listagem e do scan a menos que `--trash` seja passado explicitamente | |
| Cache comum elegível | Tratar `~/.Trash` como cache comum elegível para remoção sem exigir flag ou permissão especial | |

**User's choice:** Exibir `~/.Trash` no scan sempre como ReviewOnly (`⊙`), tornando-o elegível para remoção apenas com `--trash` (ou `safety.trash_enabled`)  
**Notes:** Garante visibilidade honesta do espaço ocupado no scan sem risco de deleção acidental no clean comum.

### Questão 2: Escopo de volumes da Lixeira

| Option | Description | Selected |
|--------|-------------|:--------:|
| Restringir a `$HOME/.Trash` | Restringir a varredura estritamente a `$HOME/.Trash`, ignorando `/Volumes/*/.Trashes` para evitar riscos em discos externos, cartões SD ou unidades montadas | ✓ |
| Varrer `/Volumes/*/.Trashes` | Descobrir e inventariar `/Volumes/*/.Trashes/<UID>` para volumes locais acessíveis, emitindo um alvo por volume e filtrando montagens de rede | |
| the agent decide | Você decide a melhor abordagem técnica entre focar no $HOME ou estender para volumes montados | |

**User's choice:** Restringir a varredura estritamente a `$HOME/.Trash`, ignorando `/Volumes/*/.Trashes` para evitar riscos em discos externos, cartões SD ou unidades montadas  
**Notes:** Mitiga riscos com desmontagem de pendrives, discos externos e cartões SD.

### Questão 3: Mecanismo de deleção ao esvaziar a Lixeira

| Option | Description | Selected |
|--------|-------------|:--------:|
| Expurgo permanente direto | Esvaziamento permanente direto dos itens filhos de `~/.Trash` (sem mover para a lixeira interna do DevClean e preservando a pasta raiz `~/.Trash`) | ✓ |
| Quarentena interna | Mover os itens de `~/.Trash` para a lixeira de quarentena reversível do DevClean com manifesto e retenção de dias | |
| the agent decide | Você decide a estratégia de remoção dos arquivos já presentes na Lixeira | |

**User's choice:** Esvaziamento permanente direto dos itens filhos de `~/.Trash` (sem mover para a lixeira interna do DevClean e preservando a pasta raiz `~/.Trash`)  
**Notes:** Evita duplicação de quarentena ao limpar itens que o usuário já havia enviado para o Trash.

### Questão 4: Tratamento de arquivos bloqueados ou somente-leitura na Lixeira

| Option | Description | Selected |
|--------|-------------|:--------:|
| Degradação tolerante | Degradação tolerante: remover todos os itens possíveis, tentar `chmod u+w` em itens somente-leitura e relatar aviso com bytes remanescentes caso haja bloqueio de sistema/TCC | ✓ |
| Abortar no primeiro erro | Abortar imediatamente no primeiro erro de deleção para que o usuário inspecione arquivos bloqueados manualmente | |
| the agent decide | Você decide a melhor abordagem para contornar ou reportar arquivos bloqueados na Lixeira | |

**User's choice:** Degradação tolerante: remover todos os itens possíveis, tentar `chmod u+w` em itens somente-leitura e relatar aviso com bytes remanescentes caso haja bloqueio de sistema/TCC  
**Notes:** Permite limpar a maioria dos itens mesmo se um arquivo estiver com lock ou permissão restrita, sem travar o processo.

---

## Granularidade e Deduplicação de User Caches

### Questão 1: Prevenção de dupla contagem em `~/Library/Caches`

| Option | Description | Selected |
|--------|-------------|:--------:|
| Lista estrita de exclusão | Lista estrita de exclusão conhecida no cleaner macOS contendo os diretórios de Caches dos outros cleaners (uv, pnpm, mise, playwright, cypress, homebrew, cocoapods, xcode, orbstack, editores) | ✓ |
| Registro central dinâmico | Registro central dinâmico no motor (Engine) que deduplica caminhos e bloqueia sobreposição entre cleaners em tempo de execução | |
| the agent decide | Você decide a melhor abordagem para garantir zero dupla contagem de bytes | |

**User's choice:** Lista estrita de exclusão conhecida no cleaner macOS contendo os diretórios de Caches dos outros cleaners (uv, pnpm, mise, playwright, cypress, homebrew, cocoapods, xcode, orbstack, editores)  
**Notes:** Garante independência determinística e clara entre os cleaners sem sobrecarga ou concorrência no runtime.

### Questão 2: Granularidade de exibição na CLI e TUI

| Option | Description | Selected |
|--------|-------------|:--------:|
| Alvos consolidados por categoria | Alvos consolidados por categoria (`User App Caches` e `User App Logs`), agrupando os bundles menores e emitindo alvos destacados apenas para itens maiores ou com regras específicas | ✓ |
| Alvos individuais por bundle | Alvos individuais por bundle de aplicativo (ex: um alvo por pasta em `~/Library/Caches/*` e `~/Library/Logs/*`) | |
| Alvo 100% agregado único | Alvo 100% agregado único para todos os `User App Caches` e um para `User App Logs` sem desmembramento | |

**User's choice:** Alvos consolidados por categoria (`User App Caches` e `User App Logs`), agrupando os bundles menores e emitindo alvos destacados apenas para itens maiores ou com regras específicas  
**Notes:** Mantém a interface limpa e legível sem perder a capacidade de destacar grandes acumuladores de lixo.

### Questão 3: Estrutura e restrição em `~/Library/Application Support`

| Option | Description | Selected |
|--------|-------------|:--------:|
| Subpastas regeneráveis permitidas | Varrer estritamente subpastas permitidas (`Cache`, `Caches`, `logs`, `Logs`, `Crashpad/completed`) com alvo dedicado de alto volume para `Spotify PersistentCache` e consolidado para os demais apps | ✓ |
| Varrer tudo procurando cache/temp | Varrer todas as pastas de Application Support procurando por qualquer pasta com nome 'cache' ou 'temp' sem lista restrita | |
| the agent decide | Você decide como estruturar os alvos e a restrição de folhas em Application Support | |

**User's choice:** Varrer estritamente subpastas permitidas (`Cache`, `Caches`, `logs`, `Logs`, `Crashpad/completed`) com alvo dedicado de alto volume para `Spotify PersistentCache` e consolidado para os demais apps  
**Notes:** Protege categoricamente bancos de dados, chaves e dados de sessão de aplicações em Application Support.

### Questão 4: Regras de isolamento para Group Containers

| Option | Description | Selected |
|--------|-------------|:--------:|
| Positive allowlist estrito | Positive allowlist estrito de grupos permitidos, buscando apenas `Library/Caches` e `Library/Logs`, excluindo `*.dev.orbstack` (pertencente ao cleaner docker) e nunca tocando dados raiz | ✓ |
| Varrer qualquer Group Container | Varrer `Library/Caches` e `Library/Logs` em qualquer Group Container existente sem filtrar por ID de grupo | |
| the agent decide | Você decide os critérios de allowlist para Group Containers e exclusão de conflitos | |

**User's choice:** Positive allowlist estrito de grupos permitidos, buscando apenas `Library/Caches` e `Library/Logs`, excluindo `*.dev.orbstack` (pertencente ao cleaner docker) e nunca tocando dados raiz  
**Notes:** Evita conflito com armazenamento de VM do OrbStack e vazamento em containers de grupos protegidos.

---

## Permissões TCC e Degradação Graciosa em Logs/Sistema

### Questão 1: Representação de bloqueio de permissão ou TCC

| Option | Description | Selected |
|--------|-------------|:--------:|
| ProviderMetrics.Partial & StatusNote | Usar `ProviderMetrics.Partial = true` e `StatusNote: "permission denied (TCC / root required)"`, marcando como ReviewOnly (`⊙`) quando bloqueado, exibindo aviso claro na CLI/TUI | ✓ |
| Warning na CLI e zero elegível | Apenas logar warning na CLI e definir `EligibleBytes = 0` mantendo o alvo no formato comum sem campos extras | |
| the agent decide | Você decide a modelagem do estado de cobertura incompleta em `domain.Target` | |

**User's choice:** Usar `ProviderMetrics.Partial = true` e `StatusNote: "permission denied (TCC / root required)"`, marcando como ReviewOnly (`⊙`) quando bloqueado, exibindo aviso claro na CLI/TUI  
**Notes:** Adere ao padrão estabelecido na Fase 10 para o Docker e stores compartilhados, garantindo transparência ao usuário.

### Questão 2: Política de privilégios para `/private/var/log`

| Option | Description | Selected |
|--------|-------------|:--------:|
| Zero sudo/root | Estritamente zero invocação de sudo/root: limpar apenas arquivos com permissão do usuário atual e reportar o restante como protegido pelo sistema | ✓ |
| Flag opcional de elevação | Permitir tentativa de elevação ou flag especial se o usuário quiser limpar logs do sistema como root | |
| the agent decide | Você decide a política de privilégios para acesso a `/private/var/log` e logs do sistema | |

**User's choice:** Estritamente zero invocação de sudo/root: limpar apenas arquivos com permissão do usuário atual e reportar o restante como protegido pelo sistema  
**Notes:** Mantém a segurança estrita do DevClean sem solicitar senhas administrativas ou abrir vetores de ataque.

### Questão 3: Descoberta de `*.code_sign_clone`

| Option | Description | Selected |
|--------|-------------|:--------:|
| Bucket do usuário via TMPDIR | Localizar via `os.Getenv("TMPDIR")` / `getconf DARWIN_USER_CACHE_DIR` para focar exclusivamente no bucket `/private/var/folders/...` do usuário logado, buscando `*.code_sign_clone` com segurança | ✓ |
| Glob global em /private/var/folders | Varrer globalmente `/private/var/folders/*/*/C/*.code_sign_clone` tratando erros de permissão de outros usuários como ignorados graciosamente | |
| the agent decide | Você decide o método de descoberta de `*.code_sign_clone` no Darwin | |

**User's choice:** Localizar via `os.Getenv("TMPDIR")` / `getconf DARWIN_USER_CACHE_DIR` para focar exclusivamente no bucket `/private/var/folders/...` do usuário logado, buscando `*.code_sign_clone` com segurança  
**Notes:** Elimina verificações desnecessárias em buckets inacessíveis de outros usuários do sistema operacional.

### Questão 4: Segregação de relatórios de diagnóstico

| Option | Description | Selected |
|--------|-------------|:--------:|
| Dois alvos distintos | Dois alvos distintos: `User Diagnostic Reports` (elegível para limpeza padrão) e `System Diagnostic Reports` (marcado como ReviewOnly se faltar permissão ou for de sistema) | ✓ |
| Alvo consolidado único | Alvo único consolidado agregando todos os relatórios de diagnóstico encontrados | |
| the agent decide | Você decide a granularidade dos alvos de Diagnostic Reports | |

**User's choice:** Dois alvos distintos: `User Diagnostic Reports` (elegível para limpeza padrão) e `System Diagnostic Reports` (marcado como ReviewOnly se faltar permissão ou for de sistema)  
**Notes:** Permite que o usuário limpe relatórios da própria conta sem esbarrar nas permissões do sistema.

---

## Positive Allowlists & Proteção de Dados Sensíveis

### Questão 1: Allowlist para daemons da Apple

| Option | Description | Selected |
|--------|-------------|:--------:|
| 8 daemons sob alvo agrupado | Positive allowlist canônica estrita para os 8 daemons da spec (Siri, Wallpaper, com.apple.mediaanalysisd, Geod, com.apple.helpd, Apple Media Services, Parsecd, Handoff), agrupados sob `macOS Daemon Caches` | ✓ |
| Qualquer com.apple.* | Permitir varredura de qualquer bundle `com.apple.*` em `~/Library/Caches` sem positive allowlist restrita | |
| 8 alvos avulsos | Emitir 8 alvos independentes separados na interface, um para cada daemon da Apple | |

**User's choice:** Positive allowlist canônica estrita para os 8 daemons da spec (Siri, Wallpaper, com.apple.mediaanalysisd, Geod, com.apple.helpd, Apple Media Services, Parsecd, Handoff), agrupados sob `macOS Daemon Caches`  
**Notes:** Foco preciso nos componentes declarados seguros sem risco de tocar daemons sensíveis do macOS.

### Questão 2: Mapeamento de processos no Process Guard

| Option | Description | Selected |
|--------|-------------|:--------:|
| Mapeamento determinístico | Mapeamento determinístico bundle ID -> processo (ex: com.spotify.client -> Spotify), executando `pgrep -x` via `cleaner.CommandExecutor`; processo ativo ou incerto marca ReviewOnly (`ReviewReasonActiveProcess`) e revalida pré-Clean | ✓ |
| Process Guard simplificado | Process Guard simplificado verificando apenas o último segmento do bundle ID (ex: com.apple.helpd -> helpd) sem tabela de alias | |
| the agent decide | Você decide a estratégia de mapeamento de processos para os bundles do macOS | |

**User's choice:** Mapeamento determinístico bundle ID -> processo (ex: com.spotify.client -> Spotify), executando `pgrep -x` via `cleaner.CommandExecutor`; processo ativo ou incerto marca ReviewOnly (`ReviewReasonActiveProcess`) e revalida pré-Clean  
**Notes:** Impede corrupção de cache enquanto as aplicações estiverem em execução ou em caso de falha de verificação (fail-closed).

### Questão 3: Whitelist imutável em código

| Option | Description | Selected |
|--------|-------------|:--------:|
| Whitelist imutável em Go | Whitelist imutável em Go (`internal/cleaner/macos/whitelist.go`), bloqueando incondicionalmente CloudKit, Mobile Documents, Keychain, Safari state, VPNs (Tailscale, WireGuard) e gerenciadores de senhas, imune a flags de override | ✓ |
| Whitelist em YAML | Whitelist configurável no YAML (`config.Config`), permitindo ao usuário habilitar limpeza de dados do Safari ou VPNs se desejar | |
| the agent decide | Você decide o local e rigor de enforcement da whitelist de segurança do macOS | |

**User's choice:** Whitelist imutável em Go (`internal/cleaner/macos/whitelist.go`), bloqueando incondicionalmente CloudKit, Mobile Documents, Keychain, Safari state, VPNs (Tailscale, WireGuard) e gerenciadores de senhas, imune a flags de override  
**Notes:** Inviolabilidade de dados essenciais e credenciais garantida no nível da linguagem.

### Questão 4: Symlink Guard e Verificação de Ownership

| Option | Description | Selected |
|--------|-------------|:--------:|
| Rigoroso com syscall.Stat_t (0 CGO) | Symlink Guard & Ownership rigorosos: verificar `st_uid == current_uid` via `syscall.Stat_t` (0 CGO), rejeitar travessia de symlinks para locais externos/protegidos, e marcar symlinks suspeitos como ReviewOnly (`ReviewReasonAmbiguous`) | ✓ |
| Resolução padrão sob $HOME | Permitir resolução de symlinks dentro de `$HOME` sem verificar ownership do arquivo | |
| the agent decide | Você decide os detalhes de implementação das checagens de symlink e ownership | |

**User's choice:** Symlink Guard & Ownership rigorosos: verificar `st_uid == current_uid` via `syscall.Stat_t` (0 CGO), rejeitar travessia de symlinks para locais externos/protegidos, e marcar symlinks suspeitos como ReviewOnly (`ReviewReasonAmbiguous`)  
**Notes:** Protege contra manipulações de links simbólicos e garante execução determinística sem CGO.

---

## the agent's Discretion

- Formatação exata dos títulos e sumários dos alvos consolidados na CLI e TUI.
- Limite em MB (sugerido: 50MB) para desmembrar um bundle específico de `User App Caches` em linha individual.
- Relação de identificadores canônicos para Group Containers autorizados.

## Deferred Ideas

None — discussion stayed within phase scope.

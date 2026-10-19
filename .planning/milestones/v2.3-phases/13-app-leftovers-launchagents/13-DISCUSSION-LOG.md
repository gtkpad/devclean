# Phase 13: App Leftovers & LaunchAgents - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-10-18  
**Phase:** 13-app-leftovers-launchagents  
**Areas discussed:** Parsing de Plist & Critérios de Stale LaunchAgents, Detecção de App Leftovers & Estratégia de Matching de Bundles, Apresentação do Inventário & Granularidade na CLI/TUI, Garantias de Segurança, Política de Não-Exclusão & Arquitetura do Cleaner  

---

## Parsing de Plist & Critérios de Stale LaunchAgents (LEFT-01, LEFT-02)

### Q1: Parsing de LaunchAgents XML e Binary Plist (bplist00)
| Option | Description | Selected |
|--------|-------------|----------|
| howett.net/plist em Go puro | Decodificar transparentemente tanto XML quanto binary plists (bplist00) sem CGO | ✓ |
| Decodificador bplist00 interno mínimo | Parser manual no pacote apenas para Program, ProgramArguments e Label | |
| Você decide | Abordagem técnica a critério do agente | |

**User's choice:** Usar biblioteca consolidada em Go puro (ex: `howett.net/plist`) para decodificar transparentemente tanto XML quanto binary plists (`bplist00`) sem CGO.  
**Notes:** Mantém zero-CGO, hermetismo em testes e conformidade total com LEFT-01.

### Q2: Classificação de Interpretadores, Wrappers e Comandos via PATH
| Option | Description | Selected |
|--------|-------------|----------|
| Classificação estrita | Apenas caminho absoluto direto (e não-interpretador) inexistente é 'Stale'. Wrappers/interpretadores, comandos de PATH ou relativos são 'Unresolved/Ambíguo' | ✓ |
| Inspecionar primeiro argumento não-flag de interpretadores | Tentar verificar scripts alvo de /bin/sh, python, etc. | |
| Você decide | Abordagem a critério do agente | |

**User's choice:** Classificação estrita: apenas caminho absoluto direto (e não-interpretador) inexistente é 'Stale'. Wrappers/interpretadores (`/bin/sh`, `python`, `osascript`), comandos de PATH ou relativos são sempre 'Unresolved/Ambíguo'.  
**Notes:** Evita falsos positivos e garante conformidade com LEFT-02.

### Q3: Diretórios de LaunchAgents e Tratamento de Permissões
| Option | Description | Selected |
|--------|-------------|----------|
| Escanear ~/Library/LaunchAgents e /Library/LaunchAgents | Excluir /System/Library/... (SIP). Falhas degradam com ProviderMetrics.Partial | ✓ |
| Apenas ~/Library/LaunchAgents | Escopo restrito ao usuário atual | |
| Incluir /Library/LaunchDaemons | Modo somente-leitura para daemons | |

**User's choice:** Escanear `~/Library/LaunchAgents` e `/Library/LaunchAgents`; excluir incondicionalmente `/System/Library/...` (SIP/Apple). Falhas de leitura degradam graciosamente com `ProviderMetrics.Partial`.  
**Notes:** Sem elevação de privilégios ou sudo.

### Q4: Tratamento de Plists Ilegíveis ou Malformados
| Option | Description | Selected |
|--------|-------------|----------|
| Fail-closed estrito | Plists ilegíveis, malformados ou launchctl inacessível reportados como ReviewOnly (ReviewReasonAmbiguous) | ✓ |
| Ignorar silenciosamente | Omitir plists corrompidos do scan | |
| Você decide | Política padrão do projeto | |

**User's choice:** Fail-closed estrito: plists ilegíveis, malformados ou com launchctl inacessível são reportados como 'Unresolved / Ambíguo' (`ReviewOnly: true`, `ReviewReasonAmbiguous`) com nota de erro.  
**Notes:** Visibilidade completa e zero exclusão precipitada.

---

## Detecção de App Leftovers & Estratégia de Matching de Bundles (LEFT-03)

### Q1: Descoberta de Aplicativos Instalados
| Option | Description | Selected |
|--------|-------------|----------|
| Varredura determinística de filesystem | Limite de profundidade 2 a 3 níveis em /Applications, /System/Applications e ~/Applications lendo Info.plist sem CGO/Spotlight | ✓ |
| mdfind (Spotlight) com fallback | Consulta ao daemon do Spotlight com fallback de filesystem | |
| Você decide | Abordagem a critério do agente | |

**User's choice:** Varredura determinística direta de filesystem (limite de profundidade de 2 a 3 níveis) em `/Applications`, `/System/Applications` e `~/Applications`, lendo `CFBundleIdentifier` e nome do app do `Info.plist` sem CGO ou dependência de Spotlight (`mdfind`).  
**Notes:** Garante testes herméticos reproduzíveis em qualquer máquina.

### Q2: Correlação com Application Support, Caches e Containers
| Option | Description | Selected |
|--------|-------------|----------|
| Índice bidirecional de instalados | Mapear tanto CFBundleIdentifier quanto nomes dos apps; leftover apenas se nem ID nem nome coincidirem | ✓ |
| Matching estrito por Bundle ID | Apenas padrões de domínio reverso (com.*, org.*, io.*) | |
| Você decide | Critério a critério do agente | |

**User's choice:** Índice bidirecional de instalados: mapear tanto `CFBundleIdentifier` quanto os nomes dos apps (ex: `'com.tinyspeck.slackmacgap'` e `'Slack'`). Candidato a leftover apenas quando nem o ID nem o nome coincidem com qualquer app instalado.  
**Notes:** Cobertura abrangente para pastas nomeadas com ID reverso ou nome simples.

### Q3: Prevenção de Falsos Positivos em CLI e Ferramentas de Desenvolvedor
| Option | Description | Selected |
|--------|-------------|----------|
| Proteção multinível | Ignorar com.apple.*, pastas geridas por outros cleaners (docker, homebrew, pnpm, etc.) e utilitários CLI comuns sem .app | ✓ |
| Checagem dinâmica via PATH | Verificar se nome da pasta existe no PATH executável | |
| Você decide | Combinação ótima a critério do agente | |

**User's choice:** Proteção multinível: ignorar componentes do sistema Apple (`com.apple.*`), diretórios geridos por outros cleaners do DevClean (`docker`, `homebrew`, editores, navegadores, `pnpm`) e ferramentas de CLI conhecidas sem `.app`.  
**Notes:** Blindagem contra identificação indevida de ecossistemas ativos de desenvolvimento.

### Q4: Localizações Escaneadas para Resíduos
| Option | Description | Selected |
|--------|-------------|----------|
| 3 localizações normativas de LEFT-03 | ~/Library/Application Support, ~/Library/Caches e ~/Library/Containers | ✓ |
| Expandir para Saved Application State e Preferences | Incluir pequenos plists de preferências | |
| Você decide | Foco no escopo normativo | |

**User's choice:** Focar estritamente nas 3 localizações normativas de `LEFT-03`: `~/Library/Application Support`, `~/Library/Caches` e `~/Library/Containers` (onde residem os megabytes/gigabytes de resíduos).  
**Notes:** Escopo delimitado e de alto impacto de recuperação de disco.

---

## Apresentação do Inventário & Granularidade na CLI/TUI (LEFT-01, LEFT-03)

### Q1: Apresentação de LaunchAgents Obsoletos
| Option | Description | Selected |
|--------|-------------|----------|
| Alvos individuais por plist | '⊙ Stale LaunchAgent: com.exemplo.service.plist' exibindo binário ausente e path | ✓ |
| Alvo único consolidado | '⊙ Stale LaunchAgents' com contagem total e detalhes internos | |
| Você decide | Formato a critério do agente | |

**User's choice:** Alvos individuais por plist (ex: `'⊙ Stale LaunchAgent: com.exemplo.service.plist'`), exibindo nas notas o binário ausente e o caminho do plist para inspeção direta.  
**Notes:** Máxima clareza para o usuário revisar itens de inicialização obsoletos.

### Q2: Agrupamento de Resíduos de Aplicativos
| Option | Description | Selected |
|--------|-------------|----------|
| Agrupamento por aplicativo residual | 1 alvo por app órfão ('⊙ App Leftover: OldApp') consolidando pastas de App Support, Caches e Containers | ✓ |
| Alvos individuais por pasta física | '⊙ App Leftover: OldApp [Application Support]' e '⊙ App Leftover: OldApp [Caches]' | |
| Alvo global único consolidado | '⊙ App Leftovers' geral com somatório único | |

**User's choice:** Agrupamento por aplicativo residual: 1 alvo por app órfão (ex: `'⊙ App Leftover: OldApp'`), consolidando suas pastas em Application Support, Caches e Containers com detalhes por pasta.  
**Notes:** Interface limpa e inteligível sem poluição de linhas avulsas redundantes.

### Q3: Medição de Tamanho e Limiar Mínimo
| Option | Description | Selected |
|--------|-------------|----------|
| Medição real em disco sem corte mínimo | devfs.CalcDirUsage para pastas; tamanho do plist para LaunchAgents; todos exibidos | ✓ |
| Ignorar resíduos menores que 1MB | Corte de ruído para sobras pequenas | |
| Você decide | Precisão contábil completa | |

**User's choice:** Medir o tamanho real em disco (`devfs.CalcDirUsage`) de todas as pastas residuais sem corte mínimo; para LaunchAgents, reportar o tamanho do `.plist` no disco.  
**Notes:** Consistência com a contabilidade determinística de bytes de todo o DevClean.

### Q4: Constantes e Semântica de ReviewReason em domain/types.go
| Option | Description | Selected |
|--------|-------------|----------|
| Constantes semânticas dedicadas | ReviewReasonStaleLaunchAgent e ReviewReasonAppLeftover em domain/types.go com notas ricas em Reason | ✓ |
| Reutilizar constantes existentes | Usar apenas ReviewReasonAmbiguous | |
| Você decide | Arquitetura tipada a critério do agente | |

**User's choice:** Criar constantes semânticas dedicadas em domain (`ReviewReasonStaleLaunchAgent` e `ReviewReasonAppLeftover`) com explicações ricas em `Target.Reason`, mantendo `ReviewReasonAmbiguous` para plists/comandos não resolvidos.  
**Notes:** Domínio tipado, robusto e autoexplicativo.

---

## Garantias de Segurança, Política de Não-Exclusão & Arquitetura do Cleaner (LEFT-04)

### Q1: Invariante de Não-Exclusão no Método Clean()
| Option | Description | Selected |
|--------|-------------|----------|
| Defesa em profundidade incondicional | Clean() recusa formalmente qualquer deleção física (retorna 0 bytes e log de recusa) | ✓ |
| Permitir sob flag explícita futura | Desbloqueio condicional por flag | |
| Você decide | Alinhamento estrito com LEFT-04 | |

**User's choice:** Defesa em profundidade incondicional: o método `Clean()` do cleaner recusa formalmente qualquer deleção (retorna 0 bytes e log de recusa), garantindo zero mutação mesmo sob chamadas diretas ou testes.  
**Notes:** Imutabilidade e segurança absoluta em conformidade com LEFT-04.

### Q2: Pacote e Registro no Motor
| Option | Description | Selected |
|--------|-------------|----------|
| Pacote dedicado internal/cleaner/leftovers | Implementar domain.Cleaner registrado sob o ecossistema 'leftovers' | ✓ |
| Submódulo em internal/cleaner/macos/ | Código dentro do pacote macos | |
| Você decide | Estrutura a critério do agente | |

**User's choice:** Pacote dedicado `internal/cleaner/leftovers` implementando `domain.Cleaner`, registrado no motor sob o ecossistema `'leftovers'`, mantendo código limpo e desacoplado.  
**Notes:** Alinhamento com a arquitetura definida na Frente 5 de `SPEC-v2.3-MOLE-PARITY.md`.

### Q3: Compatibilidade Multiplataforma e Testes Herméticos
| Option | Description | Selected |
|--------|-------------|----------|
| Arquitetura injetável de caminhos | Diretórios raiz injetáveis para testes herméticos com t.TempDir() em Linux e Darwin; no-op fora do macOS | ✓ |
| Build tags //go:build darwin | Stub vazio em Linux | |
| Você decide | Máxima testabilidade zero-CGO | |

**User's choice:** Arquitetura injetável de caminhos: permitir injeção de diretórios raiz nos construtores para testes herméticos completos com `t.TempDir()` em Linux e Darwin, com no-op em execução real fora do macOS.  
**Notes:** Cumpre `VAL-01` e `VAL-02` (testes herméticos e compilação pura zero-CGO em Darwin e Linux).

### Q4: Salvaguardas contra Symlinks e Permissões Anômalas
| Option | Description | Selected |
|--------|-------------|----------|
| Symlink Guard e Verificação de Propriedade | Rejeitar travessia de symlinks e checar ownership (st_uid == current_uid), marcando como ReviewOnly | ✓ |
| Permitir seguir symlinks existentes | Seguir links caso destino exista | |
| Você decide | Política padrão do projeto | |

**User's choice:** Symlink Guard e Verificação de Propriedade: rejeitar travessia insegura de symlinks e checar ownership (`st_uid == current_uid`), marcando links ambíguos como ReviewOnly com `ReviewReasonAmbiguous`.  
**Notes:** Padrão consistente de segurança de baixo nível do DevClean.

---

## the agent's Discretion

- Nomenclatura exata de structs auxiliares internas de mapeamento de bundle e plist em `internal/cleaner/leftovers/`.
- Nomes dos arquivos de teste e geradores de fixtures mockados em `internal/cleaner/leftovers/`.
- Limite exato de profundidade da varredura de `.app` (padrão adotado: 2 a 3 níveis).

## Deferred Ideas

None — discussion stayed within phase scope.

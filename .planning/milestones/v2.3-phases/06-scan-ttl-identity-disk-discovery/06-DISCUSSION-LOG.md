# Phase 6: Scan, TTL, Identity & Disk Discovery - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.  
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-10-14  
**Phase:** 6-Scan, TTL, Identity & Disk Discovery  
**Areas discussed:** Deduplicação e Propriedade Canônica (SCAN-04), Desacoplamento Scan vs. TTL e Modelagem no Target (SCAN-01), Descoberta Automática de ScanPaths (SCAN-03), Detecção de Presença em Disco para IsInstalled (SCAN-02)  

---

## Deduplicação e Propriedade Canônica (SCAN-04)

### Questão 1: Resolução de sobreposição entre cleaners dedicados e genéricos
| Opção | Descrição | Selecionado |
|-------|-----------|-------------|
| Priorizar o mais específico | Cleaner dedicado tem prioridade estrita e deduz bytes do cleaner genérico | ✓ |
| Primeiro registrado | Primeiro cleaner a varrer assume posse exclusiva | |
| the agent decide | Agente define estratégia | |

**Escolha do usuário:** (Recommended) Priorizar o cleaner mais específico/dedicado e deduzir os bytes do cleaner genérico para evitar qualquer dupla contagem.

### Questão 2: Resolução de symlinks e caminhos duplicados em ScanPaths
| Opção | Descrição | Selecionado |
|-------|-----------|-------------|
| filepath.EvalSymlinks | Resolver symlinks antes do scan e unificar pelo caminho físico real | ✓ |
| Ignorar symlinks | Ignorar symlinks durante descoberta | |
| Symlinks externos | Permitir apenas se apontar para fora do diretório inicial | |
| the agent decide | Agente define abordagem | |

**Escolha do usuário:** (Recommended) Resolver com filepath.EvalSymlinks antes do scan e unificar pelo caminho real canônico (evitando varrer duas vezes o mesmo inode/diretório real).

### Questão 3: Localização da lógica de deduplicação canônica
| Opção | Descrição | Selecionado |
|-------|-----------|-------------|
| Centralizar no Engine | Pipeline de resolução canônica pós-coleta/streaming no Engine | ✓ |
| Registro compartilhado | Consultar registro compartilhado durante o scan dos cleaners | |
| the agent decide | Agente define posicionamento | |

**Escolha do usuário:** (Recommended) Centralizar no Engine como pipeline de resolução canônica pós-coleta/streaming, sem acoplar cleaners individuais entre si.

### Questão 4: Apresentação de alvos subsumidos/sobrepostos
| Opção | Descrição | Selecionado |
|-------|-----------|-------------|
| Omitir alvo subordinado | Omitir alvo subsumido e deduzir bytes do genérico, com log interno | ✓ |
| Listar com 0 bytes | Manter na listagem visual com anotação explicativa | |
| the agent decide | Agente define representação | |

**Escolha do usuário:** (Recommended) Omitir o alvo subordinado e deduzir os bytes do cleaner genérico, mantendo um log/diagnóstico interno do descarte.

---

## Desacoplamento Scan vs. TTL e Modelagem no Target (SCAN-01)

### Questão 1: Expressão de tamanho ocupado vs elegível no domain.Target
| Opção | Descrição | Selecionado |
|-------|-----------|-------------|
| Campos explícitos | Adicionar EligibleBytes int64 e TTLEligible bool no domain.Target | ✓ |
| Filtro dinâmico | Calcular elegibilidade sob demanda via função externa | |
| the agent decide | Agente define campos | |

**Escolha do usuário:** (Recommended) Adicionar campos explícitos em domain.Target (ex: EligibleBytes int64 e TTLEligible bool), mantendo SizeBytes como o tamanho físico total em disco.

### Questão 2: Semântica quando MaxAgeDays == 0
| Opção | Descrição | Selecionado |
|-------|-----------|-------------|
| 100% elegível | EligibleBytes = SizeBytes e TTLEligible = true | ✓ |
| Trava de 24h | Proteger arquivos com menos de 24h | |
| the agent decide | Agente define semântica | |

**Escolha do usuário:** (Recommended) Quando MaxAgeDays == 0 (ou limpeza forçada), EligibleBytes é igual a 100% de SizeBytes e TTLEligible = true.

### Questão 3: Avaliação de mtime sem que a pasta raiz oculte o alvo
| Opção | Descrição | Selecionado |
|-------|-----------|-------------|
| Inocuidade do root mtime | Root mtime nunca oculta o alvo; cleaners inspecionam subpastas | ✓ |
| Arquivo mais recente | Alvo elegível apenas se o arquivo mais novo ultrapassar corte | |
| the agent decide | Agente define heurística de mtime | |

**Escolha do usuário:** (Recommended) O root mtime nunca oculta o alvo; cleaners inspecionam subpastas/arquivos para calcular EligibleBytes ou usam a data do item mais antigo/recente relevante.

### Questão 4: Granularidade de expurgo para alvos com conteúdo misto
| Opção | Descrição | Selecionado |
|-------|-----------|-------------|
| Híbrido atômico/agregador | Alvos de projeto limpos por inteiro; caches globais podam entradas antigas | ✓ |
| Sempre atômico | Limpar sempre o alvo inteiro como unidade atômica | |
| the agent decide | Agente define granularidade | |

**Escolha do usuário:** (Recommended) Alvos atômicos de projetos (node_modules, target, dist) são limpos por inteiro se elegíveis; caches globais agregadores podem podar entradas antigas se o cleaner suportar.

---

## Descoberta Automática de ScanPaths (SCAN-03)

### Questão 1: Ordem determinística de descoberta de ScanPaths
| Opção | Descrição | Selecionado |
|-------|-----------|-------------|
| Ordem fixa da spec | Documents, Projects, Developer, Code, Work, repos, workspace | ✓ |
| Ordem alfabética | Ordenar alfabeticamente os diretórios existentes | |
| the agent decide | Agente define ordem | |

**Escolha do usuário:** (Recommended) Seguir estritamente a ordem fixa canônica da spec (Documents, Projects, Developer, Code, Work, repos, workspace), incluindo apenas os que existirem no disco.

### Questão 2: Fallback caso nenhum dos 7 diretórios exista
| Opção | Descrição | Selecionado |
|-------|-----------|-------------|
| ScanPaths vazio | Manter ScanPaths vazio (apenas caches globais), sem traversal amplo no $HOME | ✓ |
| Fallback para . | Usar diretório de trabalho atual | |
| Fallback para $HOME | Varrer diretório $HOME completo com exclusões | |
| the agent decide | Agente define fallback | |

**Escolha do usuário:** (Recommended) Manter ScanPaths vazio (apenas caches globais são varridos), evitando traversal irrestrito e lento na raiz do $HOME.

### Questão 3: Injeção da auto-descoberta no ciclo de vida da configuração
| Opção | Descrição | Selecionado |
|-------|-----------|-------------|
| Resolução dinâmica | Método explícito (ex: cfg.EffectiveScanPaths()) sem mutar YAML | ✓ |
| Mutação no Load | Mutar cfg.ScanPaths em memória na carga de Defaults/Load | |
| the agent decide | Agente define ciclo de vida | |

**Escolha do usuário:** (Recommended) Método explícito de resolução em memória (ex: cfg.EffectiveScanPaths() ou no Engine), sem mutar nem persistir caminhos autodescobertos no arquivo YAML de config.

### Questão 4: Precedência de scan_paths explícitos
| Opção | Descrição | Selecionado |
|-------|-----------|-------------|
| Substituição total | Configuração explícita desativa auto-descoberta | ✓ |
| Mesclagem | Anexar caminhos explícitos aos autodescobertos | |
| the agent decide | Agente define precedência | |

**Escolha do usuário:** (Recommended) Substituição total: se o usuário configurou scan_paths explicitamente, a auto-descoberta é desativada e apenas os caminhos dele são varridos.

---

## Detecção de Presença em Disco para IsInstalled (SCAN-02)

### Questão 1: Combinação de executável no PATH com diretório de cache
| Opção | Descrição | Selecionado |
|-------|-----------|-------------|
| Condição OU | LookPath(binário) == nil || DirExists(caminhoCacheRaiz) | ✓ |
| Apenas disco | Checar apenas existência em disco desvinculando do PATH | |
| the agent decide | Agente define lógica | |

**Escolha do usuário:** (Recommended) Condição OU unificada: LookPath(binário) == nil || DirExists(caminhoCacheRaiz). Se o cache órfão existir, o cleaner é considerado ativo para inventário e limpeza.

### Questão 2: Cleaners orientados a comandos CLI sem binário no PATH
| Opção | Descrição | Selecionado |
|-------|-----------|-------------|
| Separação por natureza | FS cleaners apagam direto; CLI cleaners reportam erro amigável | ✓ |
| Exigir binário | CLI cleaners só retornam IsInstalled=true com binário no PATH | |
| the agent decide | Agente define comportamento | |

**Escolha do usuário:** (Recommended) Cleaners de sistema de arquivos removem arquivos diretamente independente do binário; cleaners orientados a comando CLI (Docker, Homebrew) reportam erro amigável se tentarem limpar sem o executável no PATH.

### Questão 3: Ativação de cleaners de projeto sem CLI no PATH
| Opção | Descrição | Selecionado |
|-------|-----------|-------------|
| Ativação por manifest | Presença de manifests nos ScanPaths ativa o cleaner para artefatos locais | ✓ |
| Exigir binário | Exigir binário no PATH para projetos | |
| the agent decide | Agente define ativação | |

**Escolha do usuário:** (Recommended) Sim: se houver diretórios de cache globais OU manifests de projeto em ScanPaths, o cleaner de projeto é considerado instalado e varre os artefatos locais (ex: target/, node_modules/).

### Questão 4: Performance e custo de traversal em IsInstalled()
| Opção | Descrição | Selecionado |
|-------|-----------|-------------|
| Checagens rasas | os.Stat rápido em raízes conhecidas sem traversal recursivo | ✓ |
| Cache no struct | Armazenar em cache o resultado no struct do cleaner | |
| the agent decide | Agente define performance | |

**Escolha do usuário:** (Recommended) Checagem rasa e rápida: os.Stat apenas nas raízes conhecidas de cache global e nos diretórios imediatos de ScanPaths, sem travessia recursiva no IsInstalled().

---

## the agent's Discretion

- Estrutura interna da árvore de prefixos / mapa de caminhos canônicos no `internal/engine`.
- Nomes dos métodos auxiliares e testes herméticos em `internal/config` e `internal/engine`.

## Deferred Ideas

Nenhuma ideia fora do escopo foi levantada — a discussão permaneceu estritamente focada nas 4 frentes da Fase 6.

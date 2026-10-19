# Phase 8: Node, Headless, Shell/CLI & Xcode - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-10-15  
**Phase:** 08-Node, Headless, Shell/CLI & Xcode  
**Areas discussed:** Node & Headless Automation Caches, Shell & CLI Tools Cleaner, Xcode, Simulators & Process Guard  

---

## Node & Headless Automation Caches

### Pergunta 1: Estrutura e Nível de Risco dos Caches de Browsers Headless
| Opção | Descrição | Selecionado |
|---|---|:---:|
| Alvos individuais no cleaner node | Risco Medium para binários de navegadores (Puppeteer/Playwright) e Low para caches gerais (npm/npx/logs/Corepack/Cypress) | ✓ |
| Risco Low unificado no cleaner node | Tratar binários de navegadores e caches de pacotes igualmente para limpeza desimpedida | |
| Cleaner dedicado 'automation' | Isolar ferramentas headless (Puppeteer, Playwright, Cypress) em um cleaner próprio fora de 'node' | |

**User's choice:** Alvos individuais no cleaner node com risco Medium para binários de navegadores (Puppeteer/Playwright) e Low para caches gerais (npm/npx/logs/Corepack/Cypress)  
**Notes:** Binários de navegadores (Chromium/Firefox/WebKit) baixados pelo Playwright e Puppeteer são volumosos; classificá-los como Medium previne re-downloads desnecessários em conexões lentas ou offline.

### Pergunta 2: Granularidade de Subdiretórios npm
| Opção | Descrição | Selecionado |
|---|---|:---:|
| Alvos separados por subdiretório | 'npm cache' (_cacache), 'npx cache' (_npx) e 'npm logs' (_logs), oferecendo visibilidade granular no scan | ✓ |
| Alvo unificado 'npm cache' | Cobrindo todo o diretório ~/.npm em um único bloco | |
| Dois alvos agrupados | 'npm cache' (_cacache e _npx) e 'npm debug logs' (_logs) | |

**User's choice:** Alvos separados para cada subdiretório: 'npm cache' (_cacache), 'npx cache' (_npx) e 'npm logs' (_logs), oferecendo visibilidade granular no scan  
**Notes:** Facilita a auditoria e entendimento de onde o espaço em disco do npm está sendo consumido.

### Pergunta 3: Localização do Cache de Metadados do pnpm
| Opção | Descrição | Selecionado |
|---|---|:---:|
| No cleaner 'pnpm' existente | Manter todo cache e store de pnpm sob o ecossistema 'pnpm', com descoberta por disco sem depender do CLI pnpm | ✓ |
| No cleaner 'node' | Centralizar todos os gerenciadores JS (npm, yarn, pnpm cache, Corepack) sob o ecossistema 'node' | |
| Você decide | A opção que mantiver a maior consistência arquitetural entre os cleaners existentes | |

**User's choice:** No cleaner 'pnpm' existente: manter todo cache e store de pnpm sob o ecossistema 'pnpm', com descoberta por disco sem depender do CLI pnpm  
**Notes:** O ecossistema 'pnpm' mantém sua identidade independente no motor, cobrindo tanto o cache de metadados (`~/Library/Caches/pnpm`) quanto o store global.

### Pergunta 4: Estratégia de Deleção para Caches Headless e Corepack
| Opção | Descrição | Selecionado |
|---|---|:---:|
| Exclusão direta via filesystem (os.RemoveAll) | Com validação de Blacklist e proteção de symlinks, operando 100% offline e sem depender de Node/npx | ✓ |
| Limpeza híbrida com comandos nativos | Tentar comandos nativos se o binário estiver no PATH, com fallback para os.RemoveAll | |
| Você decide | O que for mais seguro e consistente | |

**User's choice:** Exclusão direta via filesystem (os.RemoveAll) com validação de Blacklist e proteção de symlinks, operando 100% offline e sem depender de Node/npx  
**Notes:** Elimina dependência de subprocessos externos e garante velocidade e funcionamento offline completo.

---

## Shell & CLI Tools Cleaner

### Pergunta 1: Estrutura do Novo Cleaner de Shell
| Opção | Descrição | Selecionado |
|---|---|:---:|
| Novo cleaner 'internal/cleaner/shell' | Ecossistema unificado 'shell', registrando alvos distintos para mise, gh, pre-commit, omz e zcompdump | ✓ |
| Cleaners individuais separados | Um cleaner e ecossistema separado para cada ferramenta (mise, gh, pre-commit, omz) | |
| Você decide | A abordagem mais alinhada com as convenções do DevClean | |

**User's choice:** Criar o cleaner 'internal/cleaner/shell' com ecossistema unificado 'shell', registrando alvos distintos para mise, gh, pre-commit, omz e zcompdump  
**Notes:** Agrupamento limpo e coeso no motor e na configuração.

### Pergunta 2: Separação de Caches e Downloads no mise
| Opção | Descrição | Selecionado |
|---|---|:---:|
| Separar em dois alvos | 'mise cache' (risco Low) e 'mise downloads' (risco Low, resíduos de download de runtimes) | ✓ |
| Unificar em um único alvo | 'mise cache & downloads' apontando ambos para o mesmo agrupamento | |
| Caches Low e Downloads Medium | Exigir confirmação extra para downloads | |

**User's choice:** Separar em dois alvos: 'mise cache' (risco Low) e 'mise downloads' (risco Low, resíduos de download de runtimes já instalados)  
**Notes:** Downloads de runtimes em `~/.local/share/mise/downloads` são artefatos temporários pós-instalação que podem ser limpos com segurança.

### Pergunta 3: Descoberta de Dumps de Autocompletar do Zsh (~/.zcompdump*)
| Opção | Descrição | Selecionado |
|---|---|:---:|
| Alvo agregado 'Zsh completion cache' | Agrupando todos os arquivos ~/.zcompdump* encontrados no $HOME (e $ZDOTDIR) | ✓ |
| Alvos individuais no scan | Um alvo para cada arquivo ~/.zcompdump* específico | |
| Você decide | O que mantiver a saída mais compacta | |

**User's choice:** Alvo agregado 'Zsh completion cache' agrupando todos os arquivos ~/.zcompdump* encontrados no $HOME (e $ZDOTDIR se definido)  
**Notes:** Mantém a tabela de scan limpa sem múltiplas linhas para variações de hostname/versão de dumps do zsh.

### Pergunta 4: Classificação de Risco para Caches de CLI
| Opção | Descrição | Selecionado |
|---|---|:---:|
| Risco Low para gh/OMZ; Risco Medium para pre-commit | Evitar re-clone custoso de hooks se o desenvolvedor estiver offline | ✓ |
| Risco Low para todos | Tratar gh, OMZ e pre-commit identicamente como descartáveis | |
| Você decide | A classificação mais prudente | |

**User's choice:** Risco Low para gh e OMZ; risco Medium para pre-commit (já que re-clonar repos de hooks pode adicionar latência ao próximo git commit offline)  
**Notes:** `pre-commit` mantém clones completos de repositórios remotos; marcá-lo como Medium dá segurança ao desenvolvedor.

---

## Xcode, Simulators & Process Guard

### Pergunta 1: Bloqueio sob Processo Ativo ou Estado Incerto
| Opção | Descrição | Selecionado |
|---|---|:---:|
| Bloqueio com ReviewOnly e EligibleBytes=0 | Marcar alvos afetados como ReviewOnly=true com ReviewReasonActiveProcess quando Xcode/Simulator estiver aberto ou em estado incerto | ✓ |
| Aviso de Risco Alto (RiskHigh) | Permitir forçar exclusão mesmo com Xcode aberto | |
| Omitir alvos do scan | Esconder alvos enquanto processos estiverem ativos | |

**User's choice:** Marcar alvos afetados como ReviewOnly=true com ReviewReason=ReviewReasonActiveProcess e EligibleBytes=0 quando Xcode/Simulator estiver aberto ou em estado desconhecido  
**Notes:** Cumpre estritamente o portão central de segurança do Coordinator da Fase 7 e DEV-03.

### Pergunta 2: Detecção de Processos sem CGO e Fail-Closed
| Opção | Descrição | Selecionado |
|---|---|:---:|
| Verificação via CommandExecutor (pgrep) fail-closed | Se pgrep falhar ou retornar erro desconhecido, tratar como ativo e travar em ReviewOnly | ✓ |
| Inspeção via ps -eo comm | Com fallback aberto | |
| Você decide | O mecanismo mais confiável com zero-CGO | |

**User's choice:** Verificação via CommandExecutor (pgrep -x Xcode / Simulator) com política fail-closed: se pgrep falhar ou retornar erro desconhecido, tratar como ativo e travar em ReviewOnly  
**Notes:** Mantém o projeto zero-CGO, permite testes unitários herméticos com mock de CommandExecutor e garante que na dúvida a exclusão seja bloqueada.

### Pergunta 3: Política de Segurança para Xcode Archives
| Opção | Descrição | Selecionado |
|---|---|:---:|
| Risco High quando fechado; ReviewOnly quando aberto | Exige confirmação explícita/alerta sobre perda de dSYMs | ✓ |
| Incondicionalmente ReviewOnly | Archives nunca são apagados pelo DevClean | |
| Risco Medium com TTL | Permitir exclusão por MaxAgeDays quando fechado | |

**User's choice:** Risco High quando Xcode estiver fechado (exige confirmação explícita/aviso de dSYMs), e travado em ReviewOnly se Xcode estiver em execução  
**Notes:** O usuário é advertido sobre a perda irreversível de dSYMs necessários para decodificar crash logs de versões publicadas.

### Pergunta 4: Granularidade de CoreSimulator e DeviceSupport
| Opção | Descrição | Selecionado |
|---|---|:---:|
| Alvos individuais dedicados | CoreSimulator Caches, CoreSimulator Temp, iOS DeviceSupport, watchOS DeviceSupport limpos via filesystem quando fechados e ReviewOnly se ativos | ✓ |
| Dois blocos consolidados | CoreSimulator Caches & Temp e Xcode DeviceSupport | |
| Você decide | O equilíbrio ideal de clareza e simplicidade | |

**User's choice:** Alvos individuais dedicados (CoreSimulator Caches, CoreSimulator Temp, iOS DeviceSupport, watchOS DeviceSupport), limpos via filesystem quando fechados e travados em ReviewOnly se ativos  
**Notes:** Exposição precisa dos diferentes componentes de suporte e caches de simulação da Apple.

---

## the agent's Discretion

- Estrutura exata de IDs e identificadores internos em `domain.Target` para os novos alvos.
- Resolução de caminhos multiplataforma com fallback gracioso em sistemas não-Darwin.
- Expressões regulares e padrões de glob para localização de múltiplos arquivos de dump do Zsh.

## Deferred Ideas

Nenhuma ideia fora de escopo foi postergada; toda a discussão manteve-se focada nos requisitos DEV-01, DEV-02 e DEV-03.

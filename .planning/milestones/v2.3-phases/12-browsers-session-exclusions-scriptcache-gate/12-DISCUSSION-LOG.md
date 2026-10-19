# Phase 12: Browsers, Session Exclusions & ScriptCache Gate - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-10-16  
**Phase:** 12-browsers-session-exclusions-scriptcache-gate  
**Areas discussed:** Granularidade e Apresentação dos Alvos, Gating e Política do ScriptCache, Process Guard e Revalidação Pré-Clean, Tratamento do Safari e Isolamento de Sessão  

---

## Granularidade e Apresentação dos Alvos

| Option | Description | Selected |
|--------|-------------|----------|
| Agrupar por Navegador (ex: 'Chrome Caches', 'Firefox Caches') | Consolidar perfis secundários internamente para manter a CLI/TUI limpa e coesa com os editores da Fase 09 | ✓ |
| Separar por Navegador e Perfil | Exibir perfis individuais ('Chrome (Default)', 'Chrome (Profile 1)', 'Firefox (default-release)') | |
| Separar por Categoria Técnica transversal | Listar alvos cruzando todos os navegadores instalados | |

**User's choice:** Agrupar por Navegador (ex: 'Chrome Caches', 'Firefox Caches', 'Arc Caches'), consolidando perfis secundários internamente.  
**Notes:** Mantém a interface enxuta e alinhada ao design consolidado da Fase 09 para editores.

| Option | Description | Selected |
|--------|-------------|----------|
| Consolidar tudo em um único alvo por navegador | Englobar Network, GPU, Code Cache e CacheStorage em um único alvo, com discriminação nos detalhes | ✓ |
| Emitir dois alvos por navegador | Separar caches normais de Service Worker CacheStorage | |
| the agent's Discretion | Decisão técnica interna | |

**User's choice:** Consolidar tudo em um único alvo por navegador (`Chrome Caches`), discriminando subvolumes nos detalhes.  

| Option | Description | Selected |
|--------|-------------|----------|
| Varredura dinâmica de todos os perfis | Varrer Default, Profile *, System Profile e perfis Firefox | ✓ |
| Varrer estritamente o perfil principal | Ignorar perfis secundários | |
| the agent's Discretion | Decisão técnica interna | |

**User's choice:** Varredura dinâmica de todos os perfis existentes no disco.  

| Option | Description | Selected |
|--------|-------------|----------|
| Abstração unificada multi-SO | Mapear caminhos no macOS e Linux, com Safari e Arc restritos a Darwin e testes herméticos em ambos (0 CGO) | ✓ |
| Foco restrito a macOS nesta fase | Criar stubs neutros para compilação no Linux | |
| the agent's Discretion | Decisão técnica interna | |

**User's choice:** Abstração unificada multi-SO mantendo Darwin e Linux suportados sem CGO.  

---

## Gating e Política do ScriptCache

| Option | Description | Selected |
|--------|-------------|----------|
| Incondicionalmente ReviewOnly no v2.3 | Inventariar ScriptCache sempre com ReviewOnly: true, EligibleBytes: 0 e razão 'scriptcache_requires_evidence' até aceitação da Fase 14 | ✓ |
| Elegibilidade condicional com flags | Permitir limpeza com navegador fechado e flag explícita | |
| the agent's Discretion | Decisão técnica interna | |

**User's choice:** Incondicionalmente ReviewOnly no v2.3 até Fase 14.  

| Option | Description | Selected |
|--------|-------------|----------|
| Alvo isolado de revisão por navegador | Emitir alvos como 'Chrome ScriptCache' para não contaminar 'Chrome Caches' com status ReviewOnly | ✓ |
| Consolidar no mesmo alvo geral | Abater bytes e explicar nos detalhes | |
| the agent's Discretion | Decisão técnica interna | |

**User's choice:** Alvo isolado de revisão por navegador (`Chrome ScriptCache`, etc.).  

| Option | Description | Selected |
|--------|-------------|----------|
| Guarda defensiva direta em Clean() | O próprio método Clean() do cleaner recusa alvos ReviewOnly ou ScriptCache além do Coordinator | ✓ |
| Confiar exclusivamente no Coordinator | Sem guarda redundante no cleaner | |
| the agent's Discretion | Decisão técnica interna | |

**User's choice:** Guarda defensiva direta no `Clean()` do pacote de browsers.  

| Option | Description | Selected |
|--------|-------------|----------|
| Emissão estrita por motor | Emitir alvos de ScriptCache ReviewOnly exclusivamente para os 4 navegadores Chromium; Firefox e Safari protegidos pelo perímetro de sessão | ✓ |
| Emitir alvo placeholder vazio | Manter alvos vazios para Firefox e Safari | |
| the agent's Discretion | Decisão técnica interna | |

**User's choice:** Emissão estrita para motores Chromium; Firefox e Safari não emitem alvo de ScriptCache.  

---

## Process Guard e Revalidação Pré-Clean

| Option | Description | Selected |
|--------|-------------|----------|
| Bloqueio isolado por navegador | Se o Chrome estiver aberto, apenas o Chrome recebe ReviewOnly; Firefox/Edge fechados continuam elegíveis | ✓ |
| Bloqueio global da categoria | Se qualquer navegador estiver aberto, bloquear todos | |
| the agent's Discretion | Decisão técnica interna | |

**User's choice:** Bloqueio isolado por navegador individual ativo.  

| Option | Description | Selected |
|--------|-------------|----------|
| Revalidação atômica pré-Clean fail-closed | Reexecutar CheckProcessStatus antes da exclusão física; se aberto após o scan, abortar mutação fail-closed | ✓ |
| Confiar no estado capturado no Scan() | Sem revalidação no Clean() | |
| the agent's Discretion | Decisão técnica interna | |

**User's choice:** Revalidação atômica pré-Clean fail-closed.  

| Option | Description | Selected |
|--------|-------------|----------|
| Monitorar processo principal e helpers ativos | Verificar binário principal e helpers conhecidos (ex: 'Google Chrome Helper', 'firefox-bin') | ✓ |
| Monitorar estritamente o binário principal | Considerar fechado se a janela principal fechou | |
| the agent's Discretion | Decisão técnica interna | |

**User's choice:** Monitorar processo principal e helpers ativos.  

| Option | Description | Selected |
|--------|-------------|----------|
| Fail-closed estrito e testes com MockCommandExecutor | Erro de pgrep classifica como incerto (ReviewOnly = true); testes 100% herméticos sem comandos reais do SO | ✓ |
| Tratar erro de pgrep como fail-open | Assumir fechado em caso de erro | |
| the agent's Discretion | Decisão técnica interna | |

**User's choice:** Fail-closed estrito com injeção de `MockCommandExecutor` para testes herméticos.  

---

## Tratamento do Safari e Isolamento de Sessão

| Option | Description | Selected |
|--------|-------------|----------|
| Positive Allowlist estrita em ~/Library/Caches/com.apple.Safari | Varrer exclusivamente subpastas regeneráveis (WebKitCache, Cache.db), mantendo ~/Library/Safari inalcançável | ✓ |
| Varrer todo o diretório com blacklist de arquivos | Usar filtros de nomes em Caches | |
| the agent's Discretion | Decisão técnica interna | |

**User's choice:** Positive Allowlist estrita em `~/Library/Caches/com.apple.Safari`, isolando completamente `~/Library/Safari`.  

| Option | Description | Selected |
|--------|-------------|----------|
| Notação de TCC via ProviderMetrics | Em erros EACCES/EPERM/TCC, marcar ProviderMetrics.Partial = true, StatusNote explicativo, ReviewOnly = true e zero sudo | ✓ |
| Omitir silenciosamente alvos com erro | Não alarmar usuário | |
| the agent's Discretion | Decisão técnica interna | |

**User's choice:** Notação explícita de TCC via `ProviderMetrics` com zero elevação de privilégio.  

| Option | Description | Selected |
|--------|-------------|----------|
| Dupla camada de blindagem (Positive + Invariant Guard) | Apontar para folhas permitidas E passar por filtro que rejeita Cookies, History, Local Storage, IndexedDB, Login Data e Sessions | ✓ |
| Usar apenas apontamento de caminhos seguros | Sem filtro negativo redundante | |
| the agent's Discretion | Decisão técnica interna | |

**User's choice:** Dupla camada de blindagem (Positive Allowlist + Invariant Assertion Guard).  

| Option | Description | Selected |
|--------|-------------|----------|
| Fixtures herméticos com sentinelas duplas em t.TempDir() | Árvores de teste com arquivos descartáveis e sentinelas de sessão, provando que Clean() não toca em sessões | ✓ |
| Testar apenas cálculo de tamanhos no Scan() | Sem asserção de deleção de sentinelas | |
| the agent's Discretion | Decisão técnica interna | |

**User's choice:** Fixtures herméticos com sentinelas duplas em `t.TempDir()`, comprovando 100% de preservação de sessões.  

---

## the agent's Discretion
- Nomenclatura exata de constantes e funções auxiliares em `internal/cleaner/browsers/`.
- Decomposição modular de arquivos dentro de `internal/cleaner/browsers/` (`browsers.go`, `chromium.go`, `firefox.go`, `safari.go`, `process.go`).
- Mapeamento detalhado de nomes de executáveis helpers por sistema operacional.

## Deferred Ideas
None — discussion stayed within phase scope

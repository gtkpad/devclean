# Phase 14: Integrated Validation & macOS Acceptance - Context

**Gathered:** 2026-10-18  
**Status:** Ready for planning  

<domain>
## Phase Boundary

Comprovar formalmente a integridade, segurança, cobertura de testes e paridade de inventário do DevClean v2.3 em ambiente hermético (Linux e Darwin) e em máquina macOS real contra o benchmark de referência do Mole (`tw93/mole`), sem qualquer mutação destrutiva:
1. `Conformidade Unitária e Hermetismo` (`VAL-01`): auditar e garantir que 100% dos novos limpadores implementados no v2.3 possuam testes isolados em `t.TempDir()` cobrindo árvores de sentinelas, injeção de erros de permissão, restrições de links simbólicos (`Symlink Guard`) e asserções formais de `DryRun`.
2. `Qualidade, Cobertura e Portabilidade` (`VAL-02`): execução completa e verde de `go test ./...`, enforcement estrito de cobertura individual `>= 85%` em todos os pacotes novos e alterados do v2.3, e compilação limpa da matriz de 4 alvos estáticos sem dependências de CGO (`CGO_ENABLED=0`) para Darwin (`arm64`, `amd64`) e Linux (`amd64`, `arm64`).
3. `Suíte Integrada E2E Multicanal`: implementar `cmd/workflow_v23_e2e_test.go` para exercitar de ponta a ponta os 5 canais do DevClean (CLI, TUI, MCP Server, Scheduler e deleter.Coordinator), validando a rejeição inegociável de mutação em alvos `ReviewOnly: true`, proteção do `Process Guard`, `Symlink Guard`, blacklist imutável e preservação da propriedade canônica com zero double-counting de bytes.
4. `Recibo de Aceitação macOS em Host Real` (`VAL-03`): geração automatizada de `MACOS-ACCEPTANCE.md` e `MACOS-ACCEPTANCE.json` através de teste Go executável (`//go:build acceptance`), sondando diagnóstico do host em Go puro (permissões, TCC/Full Disk Access, APFS, processos protegidos), executando scan comparativo contra o benchmark do Mole (dinâmico ou snapshot do SPEC) e detalhando divergências de design e bytes ocupados versus elegíveis.

</domain>

<decisions>
## Implementation Decisions

### Recibo de Aceitação macOS & Benchmark do Mole (VAL-03)
- **D-01: Formato Duplo (Markdown e JSON) na Pasta da Fase 14:** Persistir o recibo de aceitação em `MACOS-ACCEPTANCE.md` (relatório legível com sumário de categorias, tabelas de paridade e notas de host) e `MACOS-ACCEPTANCE.json` (payload bruto estruturado com todos os alvos e métricas) dentro de `.planning/phases/14-integrated-validation-macos-acceptance/`. — **Reversibility:** reversible — formato e localização de saída de artefatos de teste.
- **D-02: Execução Dinâmica do Mole com Fallback para Snapshot do SPEC:** O executor de aceitação tenta invocar `mole clean --dry-run` caso o utilitário esteja presente no `$PATH`. Se indisponível ou inacessível, utiliza o snapshot normativo de 9 categorias documentado em `.planning/SPEC-v2.3-MOLE-PARITY.md`, registrando nota explícita no recibo de aceitação. — **Reversibility:** costly — define a estratégia de dependência de binários externos na suíte de aceitação.
- **D-03: Módulo de Diagnóstico de Host em Go Puro:** Sondagem de ambiente no macOS realizada sem CGO e sem invocar comandos shell proprietários: sondar leitura em caminhos restritos (ex: `~/Library/Safari` ou `/Library/Application Support`) para inferir status de Full Disk Access / TCC, ler flags do filesystem APFS via `devfs.Statfs` e listar processos ativos protegidos via `Process Guard`. — **Reversibility:** costly — mecanismo de inspeção ambiental no Darwin.
- **D-04: Teste Executável Go com Build Tag de Aceitação:** Orquestrar a validação de aceitação no macOS via teste Go executável sob a build tag `//go:build acceptance` em `test/acceptance/macos_test.go`, permitindo disparo limpo e determinístico via `go test -v -tags=acceptance ./test/acceptance/...` gerando os recibos ao final. — **Reversibility:** reversible — infraestrutura de testes e orquestração.

### Suíte Integrada E2E & Invariantes do Coordinator (VAL-01, VAL-02)
- **D-05: Criação do Arquivo Dedicado cmd/workflow_v23_e2e_test.go:** Espelhar o padrão consolidado em `cmd/workflow_v22_e2e_test.go`, criando `cmd/workflow_v23_e2e_test.go` focado no escopo e nas 5 frentes do v2.3, exercitando a orquestração central através dos 5 canais (CLI, TUI Bubble Tea headless, MCP Server, Scheduler e deleter.Coordinator). — **Reversibility:** costly — estabelece a suíte de testes de regressão de integração do milestone.
- **D-06: Matriz Exaustiva de Injeção de Falhas e Invariantes:** A suíte E2E deve testar obrigatoriamente 4 cenários de estresse: (1) tentativa de remoção forçada de alvos `ReviewOnly: true` em todos os 5 canais com garantia de zero mutação; (2) bloqueio de limpeza pelo `Process Guard` quando o aplicativo está em execução; (3) rejeição e bloqueio pelo `Symlink Guard` ao tentar travessias para caminhos de blacklist do sistema (`.git`, `~/.ssh`); (4) degradação graciosa com `ProviderMetrics.Partial = true` e preenchimento de `ReviewReason` sob falhas simuladas de provedores/permissão. — **Reversibility:** costly — define os testes de resiliência e integridade de segurança.
- **D-07: Hermetismo 100% com Injeção de Raízes em t.TempDir():** A suíte E2E deve operar com construtores injetáveis e executores simulados (`cleaner.MockCommandExecutor`), montando árvores virtuais completas em `t.TempDir()` para `$HOME`, caches, logs e plists, garantindo comportamento estritamente idêntico e determinístico tanto em macOS quanto em Linux (CI), sem tocar no ambiente do usuário. — **Reversibility:** costly — arquitetura de isolamento de testes entre SOs.
- **D-08: Asserção Global de Unicidade Canônica e Desduplicação:** O teste E2E deve verificar programaticamente que nenhum caminho retornado pelo `engine.Scan()` sobrepõe ou duplica outro alvo entre todos os 19 ecossistemas registrados, comprovando que nenhum byte é contado duas vezes (`zero double-counting`). — **Reversibility:** costly — verificação da garantia de integridade do inventário.

### Limiar de Cobertura (>85%) & Matriz Zero-CGO (VAL-01, VAL-02)
- **D-09: Script Dedicado de Auditoria de Cobertura por Pacote:** Criar `scripts/verify-coverage.sh` que executa `go test -coverprofile=...`, extrai a porcentagem de cobertura para cada pacote novo ou alterado no v2.3 e falha imediatamente (exit code != 0) se qualquer um deles registrar cobertura inferior a 85%. — **Reversibility:** reversible — script de verificação e controle de qualidade.
- **D-10: Escopo Abrangente de Pacotes Auditados:** A trava rígida de cobertura individual `>= 85%` incide sobre todos os pacotes de limpadores novos/alterados (`macos`, `browsers`, `leftovers`, `node`, `shell`, `xcode`, `ai`, `docker`, `orbstack`, `shared`) e sobre os pacotes centrais tocados no v2.3 (`deleter`, `engine`, `domain`). — **Reversibility:** costly — perímetro normativo de qualidade do milestone.
- **D-11: Script de Validação da Matriz de Compilação Zero-CGO:** Criar `scripts/verify-builds.sh` que compila estaticamente (`CGO_ENABLED=0 go build`) para 4 alvos cruzados: `darwin/arm64`, `darwin/amd64`, `linux/amd64` e `linux/arm64`, falhando se qualquer target falhar na compilação ou apresentar vinculações dinâmicas. — **Reversibility:** reversible — automação de checagem de portabilidade.
- **D-12: Auditoria e Preenchimento Ativo de Lacunas em VAL-01:** Mapear e revisar os testes unitários de todos os limpadores do v2.3 para assegurar a presença explícita dos 4 critérios de VAL-01: sentinelas em `t.TempDir()`, simulação de erros de permissão (`os.ErrPermission`), rejeição de links simbólicos maliciosos (`symlink traversal`) e asserções de invariância em `DryRun`. Preencher lacunas identificadas antes da aprovação da fase. — **Reversibility:** costly — complementação e rigor da suíte unitária.

### Critérios de Tolerância e Discrepâncias de Inventário (VAL-03)
- **D-13: Seção Dedicada de Divergências de Design no Recibo:** Incluir em `MACOS-ACCEPTANCE.md` uma seção analítica detalhada ("Design Divergences & Safety Rationale") justificando explicitamente por que o DevClean classifica como `ReviewOnly: true` (0 bytes elegíveis) alvos que o Mole limpa automaticamente (como `pnpm store`, `OrbStack data`, `Stale LaunchAgents`, `App Leftovers` e `ScriptCache`), ressaltando a prioridade inegociável de segurança e preservação de estado. — **Reversibility:** reversible — documentação técnica de divergência e conformidade.
- **D-14: Falha Rígida (t.Fatalf) no Teste de Aceitação:** O teste de aceitação deve reprovar sumariamente caso ocorra qualquer uma das 4 violações de segurança e integridade: (1) mutação ou deleção física em alvo `ReviewOnly`; (2) duplicação de alvos ou contagem repetida de bytes; (3) omissão injustificada de categorias em escopo que estejam presentes no disco; (4) mutação em arquivos protegidos (sessões de navegador, bancos de dados, arquivos de sistema). — **Reversibility:** one-way — compromisso de segurança inviolável do v2.3.
- **D-15: Relatório Transparente de Permissões e TCC:** Diretórios de sistema protegidos ou com acesso negado (`/private/var/log`, `/Library/Logs/DiagnosticReports`, `/Library/LaunchAgents`) devem ser destacados na tabela de aceitação como `Partial: true` com a nota descritiva "permission denied (TCC / root required)", mantendo 0 bytes elegíveis sem distorcer o inventário mensurável nem falhar silenciosamente. — **Reversibility:** reversible — formato de representação de permissões restritas.
- **D-16: Formalização Normativa do Bloqueio de ScriptCache:** Registrar o `ScriptCache` dos navegadores no recibo de aceitação com status `ReviewOnly: true` (0 bytes elegíveis) acompanhado da anotação normativa: "Gated per BROW-04 — bloqueado para deleção até evidência de repopulação offline/MV3", documentando claramente a proteção preventiva de extensões e navegação offline. — **Reversibility:** costly — alinhamento de governança de produto para o gate de navegadores.

### the agent's Discretion
- Nomenclatura exata das funções auxiliares de asserção e tabelas de dados em `cmd/workflow_v23_e2e_test.go` e `test/acceptance/macos_test.go`.
- Organização interna dos scripts de validação (`scripts/verify-coverage.sh` e `scripts/verify-builds.sh`).
- Formatação visual e estilos Markdown (badges, tabelas e callouts) adotados no arquivo gerado `MACOS-ACCEPTANCE.md`.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requisitos e Roadmap
- `.planning/ROADMAP.md` §Phase 14 — Metas, dependências e critérios de sucesso da Fase 14 (VAL-01 a VAL-03).
- `.planning/REQUIREMENTS.md` §Validation and acceptance — Requisitos normativos VAL-01, VAL-02 e VAL-03.
- `.planning/SPEC-v2.3-MOLE-PARITY.md` §4 Critérios de Aceite (Definition of Done) e §1 Diagnóstico Inicial — Especificação base de paridade com o Mole (`tw93/mole`).

### Padrões E2E e Políticas de Segurança
- `cmd/workflow_v22_e2e_test.go` — Estrutura canônica de testes de integração multicanal com mocks de Git, Bubble Tea e filesystem hermético.
- `internal/deleter/coordinator.go` — Políticas centrais de não-mutação, proteção de alvos `ReviewOnly` e execução `DryRun`.
- `internal/safety/blacklist.go` — Blacklist imutável do sistema e proteção contra travessia de symlinks (`IsCoreBlacklisted`).
- `internal/domain/types.go` — Definições de `Target`, `ProviderMetrics` e constantes semânticas de `ReviewReason`.
- `internal/engine/register.go` — Catálogo de todos os 19 ecossistemas registrados no motor.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `cleaner.MockCommandExecutor` (`internal/cleaner/executor.go`): Mock completo para simulação determinística de comandos de sistema (Docker, LookPath, etc.).
- `devfs.Statfs` / `devfs.CalcDirUsage` (`internal/fs`): Medição real de ocupação e blocos em disco e leitura de flags APFS sem CGO.
- `safety.IsCoreBlacklisted` / `safety.IsSymlink` (`internal/safety`): Validação central de diretórios do sistema e guarda contra symlink loops e travessias.
- `tui.TestProgram` / Bubble Tea Headless Harness (`internal/tui/model_test.go`): Padrão estabelecido para validação headless da interface TUI em testes E2E.
- `mcp.NewServer` (`internal/mcp/server.go`): Servidor MCP em memória para envio de requisições JSON-RPC de teste.

### Established Patterns
- **Zero-CGO Multiplataforma:** Compilação puramente estática (`CGO_ENABLED=0`) para Darwin e Linux com suporte completo a arquiteturas ARM64 e AMD64.
- **Fail-Closed Policy:** Qualquer incerteza em permissões, processos ativos ou interpretadores classifica o alvo como `ReviewOnly: true` com 0 bytes elegíveis.
- **Fixtures com Sentinelas em `t.TempDir()`:** Todos os testes unitários e de integração constroem árvores temporárias com arquivos sentinela antes da limpeza, verificando sua integridade pós-execução.
- **Separação de Inventário vs Elegibilidade:** Todo o espaço em disco é relatado no scan; apenas alvos autorizados recebem elegibilidade para limpeza.

### Integration Points
- `cmd/workflow_v23_e2e_test.go`: Nova suíte de testes cobrindo integração E2E multicanal do milestone v2.3.
- `test/acceptance/macos_test.go`: Nova suíte de aceitação no macOS host (`//go:build acceptance`) para gerar os recibos.
- `scripts/verify-coverage.sh`: Script de auditoria automatizada de cobertura mínima de 85% por pacote.
- `scripts/verify-builds.sh`: Script de validação de compilação cruzada da matriz Zero-CGO.
- `.planning/phases/14-integrated-validation-macos-acceptance/MACOS-ACCEPTANCE.md` e `.json`: Artefatos de saída do recibo de aceitação.

</code_context>

<specifics>
## Specific Ideas
- Comparativo lado a lado estruturado em tabela Markdown no recibo: Categoria Mole | Item Encontrado | Bytes Mole | Bytes DevClean | Bytes Elegíveis DevClean | Status de Paridade / Racional de Segurança.
- Registro claro das especificações da máquina macOS onde o recibo foi gerado (ex: Modelo Apple Silicon, versão do Darwin/macOS, status de FDA e APFS).

</specifics>

<deferred>
## Deferred Ideas
None — discussion stayed within phase scope.

</deferred>

---

*Phase: 14-Integrated Validation & macOS Acceptance*  
*Context gathered: 2026-10-18*

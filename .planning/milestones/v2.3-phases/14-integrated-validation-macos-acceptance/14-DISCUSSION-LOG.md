# Phase 14: Integrated Validation & macOS Acceptance - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.  
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.  

**Date:** 2026-10-18  
**Phase:** 14-Integrated Validation & macOS Acceptance  
**Areas discussed:** Recibo de Aceitação macOS & Benchmark do Mole, Suíte Integrada E2E & Invariantes do Coordinator, Limiar de Cobertura (>85%) & Matriz Zero-CGO, Critérios de Tolerância e Discrepâncias de Inventário  

---

## Recibo de Aceitação macOS & Benchmark do Mole

| Option | Description | Selected |
|--------|-------------|:--------:|
| (Recommended) Gerar MACOS-ACCEPTANCE.md e MACOS-ACCEPTANCE.json | Gerar na pasta da Fase 14 (.planning/phases/14-integrated-validation-macos-acceptance/) com sumário tabular e payload bruto | ✓ |
| Gerar .planning/v2.3-ACCEPTANCE-RECEIPT.md | Relatório consolidado diretamente na raiz do planejamento | |
| Emitir no stdout durante os testes | Registrar no stdout e anexar ao SUMMARY do plano | |

**User's choice:** (Recommended) Gerar MACOS-ACCEPTANCE.md e MACOS-ACCEPTANCE.json na pasta da Fase 14  
**Notes:** Persistência formalizada dentro da pasta da fase correspondente.

| Option | Description | Selected |
|--------|-------------|:--------:|
| (Recommended) Invocação dinâmica com fallback para SPEC | Tentar invocar `mole clean --dry-run` se estiver no $PATH; se indisponível, usar o snapshot de SPEC-v2.3-MOLE-PARITY.md | ✓ |
| Exigir binário `mole` | Abortar e falhar caso o utilitário mole não esteja instalado no sistema | |
| Usar sempre snapshot | Comparar estaticamente sem invocar binário externo | |

**User's choice:** (Recommended) Tentar invocar `mole clean --dry-run` dinamicamente se estiver no $PATH; fallback para snapshot do SPEC  
**Notes:** Evita dependência dura que quebraria testes em máquinas sem mole instalado.

| Option | Description | Selected |
|--------|-------------|:--------:|
| (Recommended) Diagnóstico em Go puro | Sondar caminhos protegidos para inferir FDA/TCC, ler statfs para APFS e Process Guard sem scripts ou CGO | ✓ |
| Executar utilitários de shell | Invocar mount, system_profiler e launchctl | |
| Registro reativo de erros | Registrar apenas os erros de permissão capturados durante a varredura | |

**User's choice:** (Recommended) Módulo de diagnóstico em Go puro  
**Notes:** Mantém zero dependências externas e hermetismo de execução.

| Option | Description | Selected |
|--------|-------------|:--------:|
| (Recommended) Teste Go executável com build tag | `//go:build acceptance` em `test/acceptance/macos_test.go` | ✓ |
| Subcomando CLI oculto | `devclean acceptance --out-dir ...` | |
| Script shell dedicado | `scripts/macos-acceptance.sh` | |

**User's choice:** (Recommended) Teste Go executável com build tag dedicada (`//go:build acceptance`)  
**Notes:** Integração nativa com a suíte de testes do Go.

---

## Suíte Integrada E2E & Invariantes do Coordinator

| Option | Description | Selected |
|--------|-------------|:--------:|
| (Recommended) cmd/workflow_v23_e2e_test.go | Criar arquivo dedicado espelhando convenção do v2.2 cobrindo os 5 canais | ✓ |
| Estender cmd/system_e2e_test.go | Incorporar verificações nos arquivos de testes E2E legados | |
| Subpacote test/e2e/v23_test.go | Criar subpacote isolado fora de cmd/ | |

**User's choice:** (Recommended) Criar `cmd/workflow_v23_e2e_test.go` dedicado ao v2.3  
**Notes:** Organização limpa por milestone de desenvolvimento.

| Option | Description | Selected |
|--------|-------------|:--------:|
| (Recommended) Matriz exaustiva com 4 cenários | ReviewOnly forçado nos 5 canais, Process Guard, Symlink Guard e falhas de provedor | ✓ |
| Foco estrito em ReviewOnly e DryRun | Delegar processos e symlinks para testes unitários | |
| Testes apenas no deleter.Coordinator | Sem replicar nos canais de interface | |

**User's choice:** (Recommended) Matriz exaustiva com os 4 cenários de injeção de falhas  
**Notes:** Garantia robusta de não-mutação sob estresse.

| Option | Description | Selected |
|--------|-------------|:--------:|
| (Recommended) Isolamento hermético 100% em t.TempDir() | Injeção de raízes e MockCommandExecutor idêntico em Linux e Darwin | ✓ |
| Mock em Linux, ambiente real em Darwin | Leitura real em macOS em modo somente-leitura | |
| Arquivos separados por SO | Build tags com asserções especializadas | |

**User's choice:** (Recommended) Isolamento hermético 100% em `t.TempDir()`  
**Notes:** Garante que a suíte rode em CI Linux e máquinas de desenvolvedores sem efeitos colaterais.

| Option | Description | Selected |
|--------|-------------|:--------:|
| (Recommended) Asserção global de unicidade canônica | Verificar que nenhum caminho do Scan() sobrepõe outro entre os 19 ecossistemas | ✓ |
| Total geral de bytes | Verificar apenas a soma agregada | |
| Restringir a Caches e Application Support | Verificar apenas caminhos com maior probabilidade de colisão | |

**User's choice:** (Recommended) Asserção global de unicidade canônica na E2E  
**Notes:** Prova matemática de zero double-counting de bytes.

---

## Limiar de Cobertura (>85%) & Matriz Zero-CGO

| Option | Description | Selected |
|--------|-------------|:--------:|
| (Recommended) scripts/verify-coverage.sh | Avaliar cada pacote individualmente e falhar se <85% | ✓ |
| Target no Makefile | make audit-coverage com validação global | |
| Total global do projeto | go test -cover ./... sem granularidade por pacote | |

**User's choice:** (Recommended) Criar `scripts/verify-coverage.sh` avaliando cada pacote alterado individualmente  
**Notes:** Rigor absoluto na meta de cobertura por pacote.

| Option | Description | Selected |
|--------|-------------|:--------:|
| (Recommended) Todos os pacotes novos e alterados | Cleaners: macos, browsers, leftovers, node, shell, xcode, ai, docker, orbstack, shared; Core: deleter, engine, domain | ✓ |
| Exclusivamente os 3 novos cleaners | macos, browsers, leftovers | |
| Repositório completo | ./... sem distinção | |

**User's choice:** (Recommended) Auditar todos os pacotes novos e alterados do v2.3  
**Notes:** Abrange tanto a periferia de limpadores quanto os módulos centrais.

| Option | Description | Selected |
|--------|-------------|:--------:|
| (Recommended) scripts/verify-builds.sh | Matriz de 4 alvos (darwin/arm64, darwin/amd64, linux/amd64, linux/arm64) com CGO_ENABLED=0 | ✓ |
| Subteste Go com exec.Command | Compilação dentro do teste E2E | |
| Makefile cross-build | Checagem manual via make | |

**User's choice:** (Recommended) Script dedicado `scripts/verify-builds.sh`  
**Notes:** Validação reproduzível e rápida de compilação estática.

| Option | Description | Selected |
|--------|-------------|:--------:|
| (Recommended) Auditoria e preenchimento de conformidade | Varrer pacotes e preencher lacunas de sentinelas, permissões, symlinks e DryRun | ✓ |
| Foco estrito na E2E | Presumir conformidade unitária prévia | |
| Suíte parametrizada de conformidade | Teste reflexivo sobre domain.Cleaner | |

**User's choice:** (Recommended) Auditoria e preenchimento de conformidade de VAL-01  
**Notes:** Garante que 100% dos novos cleaners cumpram formalmente o critério de aceite.

---

## Critérios de Tolerância e Discrepâncias de Inventário

| Option | Description | Selected |
|--------|-------------|:--------:|
| (Recommended) Seção dedicada no recibo | "Design Divergences & Safety Rationale" explicando por que alvos são ReviewOnly | ✓ |
| Tratar como discrepâncias numéricas | Apenas notas genéricas em tabela | |
| Omitir alvos divergentes | Não incluir no comparativo | |

**User's choice:** (Recommended) Seção dedicada no recibo justificando divergências de design e segurança  
**Notes:** Evidencia o posicionamento técnico e conservador do DevClean.

| Option | Description | Selected |
|--------|-------------|:--------:|
| (Recommended) Falha rígida (t.Fatalf) | Reprovar em caso de mutação em ReviewOnly, double-counting, omissão de categorias ou mutação de protegidos | ✓ |
| Falha apenas em mutação física | Avisos para discrepâncias de inventário | |
| Margem de tolerância percentual | Permitir até 15% de desvio | |

**User's choice:** (Recommended) Falha rígida (t.Fatalf) no teste de aceitação sob qualquer violação  
**Notes:** Compromisso inegociável com a segurança de dados.

| Option | Description | Selected |
|--------|-------------|:--------:|
| (Recommended) Coluna e sumário dedicado de Access & Permissions | Listar alvos com Partial: true (TCC/root necessários) mantendo bytes = 0 com nota explicativa | ✓ |
| Aviso global no topo | Alerta genérico de ausência de root | |
| Omitir itens inacessíveis | Não exibir no recibo | |

**User's choice:** (Recommended) Coluna e sumário dedicado de Access & Permissions no recibo  
**Notes:** Transparência total sobre barreiras de TCC/FDA do macOS.

| Option | Description | Selected |
|--------|-------------|:--------:|
| (Recommended) Registrar como ReviewOnly com nota normativa | "Gated per BROW-04 — bloqueado para deleção até evidência de repopulação offline/MV3" | ✓ |
| Modo experimental na suíte | Simular liberação de ScriptCache para medir bytes | |
| Omitir ScriptCache | Excluir até futuro desbloqueio | |

**User's choice:** (Recommended) Registrar ScriptCache como `ReviewOnly: true` (0 bytes elegíveis) com nota normativa  
**Notes:** Garante conformidade estrita com o gate de segurança BROW-04.

---

## the agent's Discretion

- Nomenclatura exata das funções auxiliares de asserção e tabelas de dados em `cmd/workflow_v23_e2e_test.go` e `test/acceptance/macos_test.go`.
- Organização interna dos scripts de validação (`scripts/verify-coverage.sh` e `scripts/verify-builds.sh`).
- Formatação visual e estilos Markdown (badges, tabelas e callouts) adotados no arquivo gerado `MACOS-ACCEPTANCE.md`.

## Deferred Ideas

None — discussion stayed strictly within phase scope.

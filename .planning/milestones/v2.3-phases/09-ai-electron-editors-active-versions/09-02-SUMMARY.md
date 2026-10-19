---
phase: "09-ai-electron-editors-active-versions"
plan: "09-02"
title: "Surgical Workspace Storage Backup and Secure Obsolete Extensions Scanner with Symlink & Traversal Guards"
wave: 2
completed_at: "2026-10-15"
requirements:
  - AI-02
status: "completed"
commits:
  - hash: "98da8e0"
    message: "test(ai): add test for surgical workspace storage backup scanning"
  - hash: "ddddba0"
    message: "feat(ai): implement surgical workspace storage backup scanner preserving state.vscdb"
  - hash: "e1b5069"
    message: "test(ai): add tests for obsolete extensions safety and cached VSIXs scanning"
  - hash: "4c2e095"
    message: "feat(ai): implement obsolete extensions and CachedExtensionVSIXs scanner with symlink and traversal guards"
coverage:
  internal/cleaner/ai: "83.9%"
---

# Plan 09-02 Summary: Surgical Workspace Storage Backup and Secure Obsolete Extensions Scanner

## Overview
O plano `09-02` da Fase 09 implementou a descoberta cirúrgica de backups de armazenamento de workspace (`state.vscdb.backup`) e o scanner seguro de extensões obsoletas e pacotes VSIX em cache (`CachedExtensionVSIXs`), em conformidade com o requisito **AI-02**, decisões de design **D-01**, **D-04**, **D-13**, **D-14**, **D-15**, **D-16** e mitigações do modelo de ameaças contra perda de estado de sessão, path traversal e fuga por links simbólicos (**T-09-03**, **T-09-04**, **T-09-05**, **T-09-06**).

---

## Principais Entregas

### 1. Varredura Cirúrgica de Workspace Storage (AI-02, D-13, T-09-03)
- Em `internal/cleaner/ai/editors.go`, implementada a função:
  `scanWorkspaceBackups(appSupportDir string, cutoff time.Time, maxAgeDays int) (int64, int64, bool, []string)`
- **Invariante Crítica Inviolável:** A função varre exclusivamente arquivos correspondentes a `User/workspaceStorage/*/state.vscdb.backup`. Os arquivos essenciais de sessão `state.vscdb` (SQLite primário) e `workspace.json` (metadados do workspace) nunca são inspecionados para deleção ou adicionados aos alvos emitidos.
- Alvos consolidados por editor são emitidos sob a URI virtual `editor://<id>/workspace-backups` com categoria `editor-workspace-backups`, nível de risco `domain.RiskMedium` (D-04) e respeito ao desacoplamento Scan vs TTL (D-16).
- Apenas editores com `hasWorkspaces: true` (Code, Code Insiders, Cursor, Antigravity, Windsurf) emitem o alvo; editores sem workspaces (Claude Desktop, OpenCode, Zed) ignoram essa etapa.

### 2. Scanner Seguro de Extensões Obsoletas (AI-02, D-14, D-15, T-09-04, T-09-05, T-09-06)
- Em `internal/cleaner/ai/editors.go`, implementada a função:
  `scanObsoleteExtensions(extRoot string, cutoff time.Time, maxAgeDays int) (int64, int64, bool, []string)`
- **Metadado Somente-Leitura (T-09-06):** O arquivo `<extRoot>/.obsolete` é lido exclusivamente via `os.ReadFile` e decodificado como `map[string]bool`. O arquivo `.obsolete` jamais é truncado, alterado ou adicionado aos alvos de remoção.
- **Seleção Positiva Estrita:** Apenas chaves com valor booleano `true` são candidatas para remoção. Chaves com valor `false` ou pastas presentes em `extensions/` não listadas no arquivo `.obsolete` são preservadas como extensões ativas intactas.
- **Defesa Contra Path Traversal (T-09-04):** Chaves maliciosas contendo sequências `..` ou caminhos absolutos são rejeitadas. Validação adicional com `filepath.Rel(extRoot, fullDir)` garante que o caminho alvo reside estritamente dentro da raiz de extensões.
- **Defesa Contra Links Simbólicos (T-09-05):** Inspeção com `safety.IsSymlink` previne a travessia de symlinks que apontem para locais fora da pasta de extensões.
- **Defesa por Lista Negra Central:** Verificação com `safety.IsCoreBlacklisted` impede qualquer acesso a raízes protegidas do sistema ou repositórios críticos.

### 3. Inventário de Pacotes VSIX em Cache (AI-01, AI-02, D-01)
- Em `internal/cleaner/ai/editors.go`, implementada a função:
  `scanCachedVSIXs(appSupportDir string, cutoff time.Time, maxAgeDays int) (int64, int64, bool, []string)`
- Inventaria o diretório `CachedExtensionVSIXs` dentro do Application Support dos editores.
- Consolida tanto `CachedExtensionVSIXs` quanto extensões obsoletas no alvo funcional virtual `editor://<id>/extensions` com categoria `editor-extensions`, nível de risco `domain.RiskMedium` e descrição `ed.name + ": Cached VSIXs & Obsolete Exts"`.

---

## Tarefas Executadas

### Task 09-02-01 [TDD]: Surgical Workspace Storage backup scanner preserving state.vscdb (AI-02, D-13)
- **RED**: Adicionado teste `TestEditors_WorkspaceStorage_SurgicalBackup` em `internal/cleaner/ai/editors_test.go` verificando que apenas arquivos `.backup` em `User/workspaceStorage/*/` são descobertos, e `state.vscdb` e `workspace.json` são intocados.
  - **Commit RED**: `98da8e0` `test(ai): add test for surgical workspace storage backup scanning`
- **GREEN**: Implementada `scanWorkspaceBackups` e sua integração em `scanEditors` emitindo `editor://<id>/workspace-backups` com `RiskMedium`.
  - **Commit GREEN**: `ddddba0` `feat(ai): implement surgical workspace storage backup scanner preserving state.vscdb`

### Task 09-02-02 [TDD]: Obsolete extensions and CachedExtensionVSIXs scanner with symlink and traversal guards (AI-02, D-14, D-15)
- **RED**: Adicionados testes `TestEditors_ObsoleteExtensions_Safety` e `TestEditors_CachedExtensionVSIXs` em `internal/cleaner/ai/editors_test.go` cobrindo parsing de `.obsolete`, proteção positiva de extensões ativas, mitigação de path traversal (`../../sensitive_data`), rejeição de symlinks para pastas sensíveis e preservação física de `.obsolete`.
  - **Commit RED**: `e1b5069` `test(ai): add tests for obsolete extensions safety and cached VSIXs scanning`
- **GREEN**: Implementadas `scanObsoleteExtensions` e `scanCachedVSIXs` com checagens de `filepath.Rel`, `safety.IsSymlink` e `safety.IsCoreBlacklisted`, integrando os resultados em `editor://<id>/extensions`.
  - **Commit GREEN**: `4c2e095` `feat(ai): implement obsolete extensions and CachedExtensionVSIXs scanner with symlink and traversal guards`

---

## Mitigações de Ameaças Verificadas (STRIDE)

| Threat ID | Descrição | Componente | Mitigação Aplicada |
|---|---|---|---|
| **T-09-03** | Tampering / Data Loss | `scanWorkspaceBackups` | Busca estrita por `state.vscdb.backup`; exclusão total de `state.vscdb` e `workspace.json`. |
| **T-09-04** | Tampering / Privilege Escalation | `scanObsoleteExtensions` | Rejeição de `..` e caminhos absolutos; validação de contenção com `filepath.Rel`. |
| **T-09-05** | Tampering / Information Disclosure | `scanObsoleteExtensions` | Bloqueio de symlinks via `safety.IsSymlink` antes de qualquer cálculo de tamanho ou adição. |
| **T-09-06** | Information Disclosure / Metadata Loss | `scanObsoleteExtensions` | Leitura exclusivamente como JSON de metadados; `.obsolete` nunca é adicionado aos alvos nem deletado. |

---

## Verificação e Qualidade
- `go test -v ./internal/cleaner/ai -run "TestEditors_WorkspaceStorage|TestEditors_ObsoleteExtensions|TestEditors_CachedExtensionVSIXs"`: PASS.
- `go test -v ./internal/cleaner/ai/...`: PASS (13/13 testes passando).
- `go test ./...`: PASS em todos os 39 pacotes do projeto com zero falhas e compilação limpa.
- Cobertura de código do pacote `internal/cleaner/ai`: 83.9% dos statements.

---
phase: "06"
slug: "scan-ttl-identity-disk-discovery"
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: "2026-10-14"
---

# Phase 06 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go Standard Library `testing` |
| **Config file** | `go.mod` (root) |
| **Quick run command** | `go test -v ./internal/domain ./internal/config ./internal/engine` |
| **Full suite command** | `go test -v ./internal/... ./cmd/...` |
| **Estimated runtime** | ~10-15 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test -v ./internal/domain ./internal/config ./internal/engine`
- **After every plan wave:** Run `go test -v ./internal/... ./cmd/...`
- **Before `/gsd-verify-work`:** Full suite must be green (`CGO_ENABLED=0 go test ./...`)
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 06-01-01 | 01 | 1 | SCAN-01 | — | `domain.Target` expõe `EligibleBytes` e `TTLEligible` sem quebrar serialização ou cálculos de tamanho físico | unit | `go test -v ./internal/domain` | ✅ | ⬜ pending |
| 06-01-02 | 01 | 1 | SCAN-01 | — | Cleaners reportam 100% dos bytes físicos no scan; mtime não oculta alvos existentes | unit | `go test -v ./internal/cleaner/... -run "Test.*(TTL\|Mtime\|Scan)"` | ❌ W0 | ⬜ pending |
| 06-02-01 | 02 | 2 | SCAN-02 | — | `IsInstalled()` retorna true se diretórios de cache existirem no disco mesmo sem binário no PATH | unit | `go test -v ./internal/cleaner/... -run "Test.*IsInstalled"` | ❌ W0 | ⬜ pending |
| 06-02-02 | 02 | 2 | SCAN-03 | — | `EffectiveScanPaths` descobre as 7 pastas sob `$HOME` em ordem determinística com fallback seguro e precedência de config explícita | unit | `go test -v ./internal/config -run "Test.*EffectiveScanPaths"` | ❌ W0 | ⬜ pending |
| 06-03-01 | 03 | 3 | SCAN-04 | — | Deduplicação e propriedade canônica centralizada no engine: cleaner específico vence e bytes de filhos são deduzidos do pai | unit/integration | `go test -v ./internal/engine -run "Test.*(Ownership\|Deduplicate\|Symlink)"` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/domain/types_test.go` — stubs e testes para novos campos `EligibleBytes` e `TTLEligible` em `Target`
- [ ] `internal/config/config_test.go` — testes unitários para `EffectiveScanPaths()` (ordem determinística, fallback vazio, prioridade de config)
- [ ] `internal/engine/ownership_test.go` — testes de resolução de propriedade canônica e dedução de bytes entre alvos sobrepostos
- [ ] `internal/cleaner/cleaner_mtime_test.go` — fixtures e testes de scan com mtime recente da pasta raiz

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| *None* | — | — | Todos os comportamentos da Fase 06 possuem verificação automatizada via testes unitários e de integração herméticos com `t.TempDir()`. |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending 2026-10-14

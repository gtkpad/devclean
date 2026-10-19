# Phase 7: ReviewOnly & Coordinator Policy - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-10-15  
**Phase:** 7-ReviewOnly & Coordinator Policy  
**Areas discussed:** ReviewOnly metadata & reasons, Coordinator rejection & error reporting, Visual presentation in CLI, TUI & MCP, DryRun zero-mutation guarantees  

---

## ReviewOnly metadata & reasons

### Q1: Modelagem no domain.Target
| Option | Description | Selected |
|--------|-------------|----------|
| Flat fields | Flat fields `ReviewOnly bool` and `ReviewReason string` in domain.Target with typed reason constants | ✓ |
| Nested struct | Nested struct pointer `Review *ReviewPolicy` containing Reason and ReviewOnly flag | |
| You decide | The agent decides | |

**User's choice:** Flat fields `ReviewOnly bool` and `ReviewReason string` in domain.Target with typed reason constants (simple, zero-alloc when empty, matches EligibleBytes/TTLEligible pattern)  
**Notes:** Keeps alignment with Phase 6 additions to Target.

### Q2: Possibilidade de override de mutação
| Option | Description | Selected |
|--------|-------------|----------|
| Strict lock | General mutation commands (`clean --force`, `purge`, `auto`, MCP, scheduler) unconditionally reject ReviewOnly targets; removal requires external manual actions or native tools | ✓ |
| Explicit CLI override | Allow deletion only if user passes `--allow-review-only` on CLI | |
| You decide | The agent decides | |

**User's choice:** Strict lock: General mutation commands unconditionally reject ReviewOnly targets; removal requires external manual actions or native tools (e.g., `docker prune`)  
**Notes:** Non-negotiable safety guard against accidental deletion of shared stores or daemon data.

### Q3: Comportamento de EligibleBytes e TTLEligible
| Option | Description | Selected |
|--------|-------------|----------|
| Zero eligible | `EligibleBytes = 0` and `TTLEligible = false` unconditionally for ReviewOnly targets | ✓ |
| Calculate based on TTL | Calculate `EligibleBytes` based on TTL as normal, but rely on `ReviewOnly = true` as safety block | |
| You decide | The agent decides | |

**User's choice:** `EligibleBytes = 0` and `TTLEligible = false` unconditionally for ReviewOnly targets (clean separation: occupied bytes in SizeBytes, zero bytes authorized for automated cleanup)  
**Notes:** Clarifies that occupied space is never authorized for auto-clean.

### Q4: Padronização de constantes de ReviewReason
| Option | Description | Selected |
|--------|-------------|----------|
| Standard constants | Standard constants in `domain` for standard review categories (`ReviewReasonSharedStore`, `ReviewReasonActiveProcess`, `ReviewReasonOrphanLeftover`, `ReviewReasonSystemProtected`, `ReviewReasonSessionSensitive`) with human explanations | ✓ |
| Free-form strings | Free-form descriptive strings generated ad-hoc by each individual cleaner | |
| You decide | The agent decides | |

**User's choice:** Standard constants in `domain` for standard review categories with human explanations  
**Notes:** Establishes semantic categories consumed by future cleaners in Phases 8–13.

---

## Coordinator rejection & error reporting

### Q1: Estágio de interceptação no Coordinator
| Option | Description | Selected |
|--------|-------------|----------|
| Stage 1 (Safety Validation) | Strip ReviewOnly targets immediately alongside blacklist filtering before partitioning, hooks, or deletion sinks are reached | ✓ |
| Stage 2 (Partitioning) | Partition into separate `reviewOnlyTargets` slice during Stage 2 and skip before Stage 3 | |
| You decide | The agent decides | |

**User's choice:** Stage 1 (Safety Validation): Strip ReviewOnly targets immediately alongside blacklist filtering before partitioning, hooks, or deletion sinks are reached  
**Notes:** Guarantees no hooks or sinks are reached.

### Q2: Relatório de alvos rejeitados
| Option | Description | Selected |
|--------|-------------|----------|
| Non-fatal skip with tracking | Add `SkippedReviewOnly []domain.Target` to `deleter.Result`, emit `opts.OnWarning` note, and proceed cleaning remaining valid targets | ✓ |
| Treat as non-fatal errors | Append explanatory error to `res.Errors` for each rejected ReviewOnly target | |
| Fail-fast | Abort entire run and return fatal error | |
| You decide | The agent decides | |

**User's choice:** Non-fatal skip with structured tracking: Add `SkippedReviewOnly []domain.Target` to `deleter.Result`, emit an `opts.OnWarning` note, and proceed cleaning remaining valid targets  
**Notes:** Allows partial batches with valid targets to complete while tracking skipped items.

### Q3: Comportamento quando lote é 100% ReviewOnly
| Option | Description | Selected |
|--------|-------------|----------|
| Graceful completion | Return `res` with `TotalCount: 0`, `TotalFreed: 0`, populated `SkippedReviewOnly`, and `nil` error | ✓ |
| Typed sentinel error | Return `ErrNoDeletableTargets` indicating all candidates were blocked | |
| You decide | The agent decides | |

**User's choice:** Graceful completion: Return `res` with `TotalCount: 0`, `TotalFreed: 0`, populated `SkippedReviewOnly`, and `nil` error so callers can report the refusal reason gracefully  
**Notes:** Clean output without crash or unhandled error.

### Q4: Invariante de portão único de mutação
| Option | Description | Selected |
|--------|-------------|----------|
| Coordinator Single-Gate | Every mutation path (CLI clean/purge, TUI, MCP, scheduler) must delegate to Coordinator.Delete() | ✓ |
| Caller-side filtering only | Let each UI / CLI caller filter out ReviewOnly targets before calling Coordinator | |
| You decide | The agent decides | |

**User's choice:** Coordinator Single-Gate Invariant: Every mutation path must delegate to Coordinator.Delete(), where policy is authoritatively enforced even if a caller forgot to pre-filter  
**Notes:** Single perimeter of defense across all entry points.

---

## Visual presentation in CLI, TUI & MCP

### Q1: Marcador `⊙` na CLI
| Option | Description | Selected |
|--------|-------------|----------|
| Integrated marker | Prepend `⊙ ` to the path (or risk column), and in verbose mode append `[ReviewOnly: <reason>] (eligible: 0 B)` | ✓ |
| Dedicated STATUS column | Add permanent STATUS column in `devclean scan` table | |
| You decide | The agent decides | |

**User's choice:** Integrated marker: Prepend `⊙ ` to the path (or risk column), and in verbose mode append `[ReviewOnly: <reason>] (eligible: 0 B)`  
**Notes:** Maintains clean table width.

### Q2: Comportamento na TUI
| Option | Description | Selected |
|--------|-------------|----------|
| Locked indicator `[⊙]` | Render marker `[⊙]` in place of `[ ]`, ignore space/toggle actions with status bar notice, exclude from mass-selection (`a`/`m`) | ✓ |
| Permit selection with warning | Allow toggling `[x]`, prompt alert dialog upon enter | |
| You decide | The agent decides | |

**User's choice:** Locked indicator `[⊙]`: Render marker `[⊙]` in place of `[ ]`, ignore space/toggle actions with a status bar notice explaining the review reason, and exclude from mass-selection (`a`/`m`)  
**Notes:** Prevents accidental selection entirely.

### Q3: Formato de saída no MCP
| Option | Description | Selected |
|--------|-------------|----------|
| Rich status & metadata | `propose_deletion` returns `status: "review_only"`, `review_reason`, `occupied_bytes`, and `eligible_bytes: 0`; `scan_environment` serializes `review_only` and `review_reason`; `explain_target` notes review policy | ✓ |
| Generic blocked status | Return `status: "blocked"` with reason formatted as `"review-only: <reason>"` | |
| You decide | The agent decides | |

**User's choice:** Rich status & metadata: `propose_deletion` returns `status: "review_only"`, `review_reason`, `occupied_bytes`, and `eligible_bytes: 0`; `scan_environment` serializes `review_only` and `review_reason`; `explain_target` notes review policy  
**Notes:** Clear contract for AI agents calling MCP.

### Q4: Aviso informativo pós-limpeza
| Option | Description | Selected |
|--------|-------------|----------|
| Informative notice | Add footer or summary note in clean output (e.g. `⊙ N review-only items retained (X GB) — run devclean scan to review`) | ✓ |
| Quiet behavior | Omit retained ReviewOnly items unless `--verbose` | |
| You decide | The agent decides | |

**User's choice:** Informative notice: Add a footer or summary note in clean output so users know why those large items weren't deleted  
**Notes:** Eliminates confusion when total space reported in scan differs from space freed in clean.

---

## DryRun zero-mutation guarantees

### Q1: Arquitetura de isolamento de efeitos colaterais
| Option | Description | Selected |
|--------|-------------|----------|
| Centralize in Coordinator | Remove external trash maintenance calls in `cmd/clean.go` and ensure all side effects are strictly guarded inside `Coordinator.Delete()` when `DryRun` is true | ✓ |
| Rely on caller guards | Keep individual `if !dryRun` checks in each CLI command in addition to Coordinator guards | |
| You decide | The agent decides | |

**User's choice:** Centralize in Coordinator: Remove external trash maintenance calls in `cmd/clean.go` and ensure all side effects are strictly guarded inside `Coordinator.Delete()` when `DryRun` is true  
**Notes:** Guarantees zero side effects regardless of caller logic.

### Q2: Relato de ReviewOnly em DryRun
| Option | Description | Selected |
|--------|-------------|----------|
| Consistent refusal | Even under DryRun, ReviewOnly targets are filtered to `SkippedReviewOnly`, proving they would never be deleted even in live runs | ✓ |
| Hypothetical preview | Report as simulated candidates under DryRun but block in live runs | |
| You decide | The agent decides | |

**User's choice:** Consistent refusal: Even under DryRun, ReviewOnly targets are filtered to `SkippedReviewOnly` (emitting `ActionReviewOnlySkipped` or warning), proving they would never be deleted even in live runs  
**Notes:** Deterministic preview fidelity.

### Q3: Verificação de mutação zero em testes
| Option | Description | Selected |
|--------|-------------|----------|
| Filesystem canary & state assertion | Test with real canary files in `t.TempDir()`, asserting files, trash dir, history JSONL, and scheduler files are byte-for-byte untouched | ✓ |
| Mock-only method assertions | Test exclusively with in-memory mocks verifying method call counters are 0 | |
| You decide | The agent decides | |

**User's choice:** Filesystem canary & state assertion: Test with real canary files in `t.TempDir()`, asserting that files, trash dir, history JSONL, and scheduler files are byte-for-byte untouched after DryRun across all entry points  
**Notes:** Real end-to-end assurance of zero disk modification.

### Q4: Resistência a injeção de comandos
| Option | Description | Selected |
|--------|-------------|----------|
| Structured args + path canonicalization | Never use shell string execution (`sh -c`); pass arguments directly to `exec.CommandContext` and validate with `safety.Blacklist` and symlink evaluation | ✓ |
| Regex sanitization filter | Run regex whitelist over target IDs and paths | |
| You decide | The agent decides | |

**User's choice:** Structured args execution + path canonicalization: Never use shell string execution (`sh -c`); pass arguments directly to `exec.CommandContext` and validate with `safety.Blacklist` and symlink evaluation so injection strings remain inert and rejected  
**Notes:** Direct OS process execution without shell expansion.

---

## the agent's Discretion

- Styling and exact lipgloss color tokens for the `⊙ REVIEW` badge in CLI and TUI.
- Helper method naming on `domain.Target` (e.g., `IsReviewOnly()`).

## Deferred Ideas

None — all discussion items aligned directly with Phase 7 scope.

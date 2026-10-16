# Roadmap: DevClean

## Overview

Milestone v2.3 makes DevClean's inventory complete and its cleanup decisions explainable. The work starts by separating discovery from TTL eligibility and assigning every byte to one canonical owner, then applies a central ReviewOnly policy before adding developer tools, editors, shared stores, macOS caches, browsers, and leftover detection. The final phase validates the complete flow with hermetic tests, cross-builds, and a read-only macOS acceptance receipt.

## Milestones

- ✅ **v2.2** — Phases 1–5 (shipped)
- 🚧 **v2.3** — Phases 6–14 (planned)

## Phases

- [x] **Phase 6: Scan, TTL, Identity & Disk Discovery** - Make inventory complete, deterministic, and deduplicated before eligibility is evaluated. (completed 2026-10-14)
- [x] **Phase 7: ReviewOnly & Coordinator Policy** - Enforce one conservative mutation policy across CLI, TUI, MCP, scheduler, and direct calls. (completed 2026-10-15)
- [x] **Phase 8: Node, Headless, Shell/CLI & Xcode** - Discover the specified developer-tool caches and block unsafe active-tool cleanup. (completed 2026-10-15)
- [x] **Phase 9: AI/Electron Editors & Active Versions** - Cover editor cache leaves while preserving state databases and active tool versions. (completed 2026-10-15)
- [x] **Phase 10: Docker, OrbStack & Shared Stores** - Report provider metrics while treating daemon and package stores as review-only inventory. (completed 2026-10-16)
- [x] **Phase 11: macOS User/System Caches & Logs** - Add allowlisted user/system coverage with process, permission, ownership, and overlap guards. (completed 2026-10-16)
- [ ] **Phase 12: Browsers, Session Exclusions & ScriptCache Gate** - Inventory six browsers while protecting session data and gating ScriptCache eligibility.
- [ ] **Phase 13: App Leftovers & LaunchAgents** - Produce evidence-based review candidates without automatic removal.
- [ ] **Phase 14: Integrated Validation & macOS Acceptance** - Prove safety, coverage, portability, and read-only benchmark parity on a real macOS host.

## Phase Details

### Phase 6: Scan, TTL, Identity & Disk Discovery

**Goal**: Users can see every existing target and its occupied bytes regardless of root mtime, executable availability, or empty path configuration, with each byte owned by one canonical target.
**Depends on**: Phase 5
**Requirements**: SCAN-01, SCAN-02, SCAN-03, SCAN-04
**Success Criteria** (what must be TRUE):
  1. A scan reports existing targets and descendant bytes even when a parent is recently modified or a child is old; TTL changes only explicit cleanup eligibility.
  2. An on-disk target is inventoried when its owning executable is absent from `PATH`, and empty `ScanPaths` discovers `Documents`, `Projects`, `Developer`, `Code`, `Work`, `repos`, and `workspace` in deterministic order.
  3. Overlapping or symlinked roots produce one canonical owner and do not double-count descendant bytes or submit duplicate deletion targets.
  4. Scan fixtures cover old/new mtime combinations, permission and symlink boundaries, and deterministic ordering while retaining zero-CGO buildability.

**Plans**: TBD

### Phase 7: ReviewOnly & Coordinator Policy

**Goal**: Users can distinguish inventory from cleanup authorization, and every mutation path refuses ReviewOnly or dry-run side effects at the central Coordinator boundary.
**Depends on**: Phase 6
**Requirements**: POL-01, POL-02, POL-03, POL-04
**Success Criteria** (what must be TRUE):
  1. Targets expose a `ReviewOnly` marker and explanatory reason, and CLI, TUI, and MCP reports show `⊙`, occupied bytes, eligible bytes, and the refusal/review reason.
  2. Automatic, force, scheduled, MCP, TUI, and direct Coordinator mutation attempts reject ReviewOnly targets before hooks or deletion sinks.
  3. A dry-run leaves cleaners, trash, hooks, scheduler state, and deletion history unchanged while still returning the same explainable inventory.
  4. Policy tests exercise every mutation entry point, blacklist and symlink protection, and provider-command injection without allowing a side effect.

**Plans**: 4/4 plans complete

Plans:
**Wave 1**
- [x] 07-01-PLAN.md — Domain Target Modeling & Coordinator Stage 1 Interception
- [x] 07-03-PLAN.md — CLI & TUI Review Presentation Parity

**Wave 2** *(blocked on Wave 1 completion)*
- [x] 07-02-PLAN.md — Coordinator DryRun Zero-Mutation Guarantee & Safety Invariants

**Wave 3** *(blocked on Wave 2 completion)*
- [x] 07-04-PLAN.md — MCP Server Review Protocol Parity

**UI hint**: yes

### Phase 8: Node, Headless, Shell/CLI & Xcode

**Goal**: Users can discover the specified Node/headless automation, shell/CLI, and Xcode artifacts by disk presence while active or unknown tool state blocks unsafe cleanup.
**Depends on**: Phase 7
**Requirements**: DEV-01, DEV-02, DEV-03
**Success Criteria** (what must be TRUE):
  1. npm, npx, Corepack, pnpm cache, Puppeteer, Playwright, and Cypress targets appear with occupied bytes when present, even without their commands installed.
  2. mise, GitHub CLI, pre-commit, Oh My Zsh, and zsh completion caches are inventoried without reading or changing shell configuration or history.
  3. CoreSimulator caches/temp, iOS/watchOS DeviceSupport, and Xcode Archives are reported, and running or unknown Xcode/Simulator state makes deletion ineligible.
  4. Each new cleaner has temporary-directory sentinels, permission and symlink cases, and dry-run assertions with altered-package coverage above 85%.

**Plans**: 4/4 plans complete

Plans:
**Wave 1**
- [x] 08-01-PLAN.md — Granular Node & Headless Browser Caches, pnpm HTTP Metadata Cache
- [x] 08-02-PLAN.md — Shell & CLI Tools Cleaner (`internal/cleaner/shell`)
- [x] 08-03-PLAN.md — Expanded Xcode & Simulator Targets with Injectable Fail-Closed Process Guard

**Wave 2** *(blocked on Wave 1 completion)*
- [x] 08-04-PLAN.md — Ecosystem Wiring, Engine Registration, and Verification Suite

### Phase 9: AI/Electron Editors & Active Versions

**Goal**: Users can review regenerable cache leaves across AI and Electron editors without losing workspace state, metadata, or the active CLI version.
**Depends on**: Phase 8
**Requirements**: AI-01, AI-02, AI-03
**Success Criteria** (what must be TRUE):
  1. Code, Code Insiders, Cursor, Antigravity, Windsurf, Claude Desktop, OpenCode, and Zed report the specified cache, log, Crashpad, Sentry, and Chromium/Electron leaves by presence.
  2. Editor workspace storage selects only `state.vscdb.backup`; `.obsolete` remains metadata and only directly referenced payloads can become candidates.
  3. CLI version cleanup resolves the active symlink, preserves its target, and fails closed when the link is broken, ambiguous, or the version is active.
  4. Process, ownership, and unknown-state guards are visible in output, and fixtures prove protected databases, configuration, and active versions remain unchanged.

**Plans**: TBD
- [x] 09-01-PLAN.md
- [x] 09-02-PLAN.md
- [x] 09-03-PLAN.md
- [x] 09-04-PLAN.md

### Phase 10: Docker, OrbStack & Shared Stores

**Goal**: Users can inspect provider and shared-store space with honest metrics while all daemon-managed or cross-project data remains review-only.
**Depends on**: Phase 9
**Requirements**: STORE-01, STORE-02, STORE-03
**Success Criteria** (what must be TRUE):
  1. Docker inventory reports total, active, size, reclaimable, and detailed JSON metrics when the provider supports them, keeping logical and host metrics distinct.
  2. OrbStack regenerable caches are separated from VM/data storage, and sparse/apparent/allocated values are labeled rather than summed as reclaimable bytes.
  3. Docker storage, OrbStack data, NuGet packages, and pnpm stores are marked `ReviewOnly` and are rejected by automatic cleanup and equivalent mutation calls.
  4. Provider absence, daemon errors, malformed metrics, locks, and shared-store uncertainty remain explicit unknown/review states instead of zero-byte success; fixtures verify no store sentinel is deleted.

**Plans**: TBD
- [x] 10-01-PLAN.md
- [x] 10-02-PLAN.md
- [x] 10-03-PLAN.md
- [x] 10-04-PLAN.md

### Phase 11: macOS User/System Caches & Logs

**Goal**: Users can inventory allowlisted macOS caches, logs, Trash, and application leaves with honest permission coverage and protection for active or sensitive data.
**Depends on**: Phase 10
**Requirements**: MAC-01, MAC-02, MAC-03, MAC-04, MAC-05
**Success Criteria** (what must be TRUE):
  1. User caches, user logs, and Trash are inventoried; Trash is eligible only when an explicit option is enabled.
  2. Apple daemon caches, Group Container caches/logs, Handoff, Spotify PersistentCache, and specified Application Support leaves use positive allowlists with dedicated ownership and no overlap double-counting.
  3. System logs, DiagnosticReports, and code-sign clones report incomplete coverage on permission or TCC denial rather than silently reporting zero bytes.
  4. Process Guard, immutable whitelist, symlink checks, ownership checks, and immediate pre-delete revalidation prevent cleanup of active applications or protected data.
  5. macOS cleaner fixtures preserve protected sentinels across scan, dry-run, and deletion, including permission, symlink, race, and unknown-process cases; Darwin builds remain zero-CGO.

**Plans**: TBD

### Phase 12: Browsers, Session Exclusions & ScriptCache Gate

**Goal**: Users can inspect regenerable browser caches across six supported browsers without targeting sessions or personal state, with ScriptCache eligibility gated by evidence.
**Depends on**: Phase 11
**Requirements**: BROW-01, BROW-02, BROW-03, BROW-04
**Success Criteria** (what must be TRUE):
  1. Chrome, Brave, Arc, Edge, Firefox, and Safari are detected by on-disk presence with canonical profile/cache paths.
  2. Only the specified Service Worker caches, GPU/shader/code caches, and network caches enter cleanup eligibility; Cookies, History, Local Storage, IndexedDB, Login Data, and Sessions never become targets.
  3. ScriptCache is inventoried with an explicit refusal or review reason until process-guard, MV3 extension recovery, offline-data, and repopulation acceptance evidence authorizes eligibility.
  4. Browser-closed checks are revalidated immediately before deletion, and temporary fixtures prove disposable sentinels change only after approval while protected session sentinels remain intact.

**Plans**: TBD

### Phase 13: App Leftovers & LaunchAgents

**Goal**: Users can review evidence for stale LaunchAgents and app leftovers without automatic removal or false positives from ambiguous launch configurations.
**Depends on**: Phase 12
**Requirements**: LEFT-01, LEFT-02, LEFT-03, LEFT-04
**Success Criteria** (what must be TRUE):
  1. User and system LaunchAgents are parsed from both XML and binary plist files, exposing `Program` or `ProgramArguments[0]` evidence.
  2. Only an absolute reference to a missing executable is classified as strong stale evidence; unreadable, relative, interpreter, PATH-based, malformed, and unavailable-runtime cases remain unresolved ReviewOnly items.
  3. App-leftover candidates cross-check installed bundles with Application Support, Caches, and Containers while retaining explicit paths and reasons.
  4. Leftovers and stale LaunchAgents are never removed automatically, and dry-run/report paths leave plist and payload sentinels unchanged.
  5. Fixtures cover XML/binary parsing, symlinks, wrappers, permission errors, launchctl unavailability, and malformed values with no unsafe classification.

**Plans**: TBD

### Phase 14: Integrated Validation & macOS Acceptance

**Goal**: Users and maintainers have evidence that the complete v2.3 inventory and safety contract works end to end on supported platforms and on a real macOS host.
**Depends on**: Phase 13
**Requirements**: VAL-01, VAL-02, VAL-03
**Success Criteria** (what must be TRUE):
  1. Every new cleaner has `t.TempDir()` sentinels, permissions, symlink cases, and DryRun assertions; `go test ./...`, altered-package coverage above 85%, and zero-CGO Linux/Darwin builds pass.
  2. End-to-end CLI, TUI, MCP, scheduler, and Coordinator runs preserve ReviewOnly, dry-run, blacklist, ownership, and duplicate-accounting invariants under injected provider/process failures.
  3. A real macOS acceptance receipt records DevClean inventory, permission/process/TCC/Full Disk Access state, APFS and omitted-item notes, and a read-only external benchmark preview without running destructive cleanup.
  4. The acceptance comparison explains occupied versus eligible bytes, records unresolved ScriptCache and permission decisions, and fails on omitted in-scope items, duplicate bytes, protected-state mutation, or ReviewOnly deletion.

**Plans**: TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 6 → 7 → 8 → 9 → 10 → 11 → 12 → 13 → 14

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 6. Scan, TTL, Identity & Disk Discovery | 3/3 | Complete    | 2026-10-14 |
| 7. ReviewOnly & Coordinator Policy | 4/4 | Complete    | 2026-10-15 |
| 8. Node, Headless, Shell/CLI & Xcode | 4/4 | Complete    | 2026-10-15 |
| 9. AI/Electron Editors & Active Versions | 4/4 | Complete    | 2026-10-15 |
| 10. Docker, OrbStack & Shared Stores | 4/4 | Complete    | 2026-10-16 |
| 11. macOS User/System Caches & Logs | 4/4 | Complete    | 2026-10-16 |
| 12. Browsers, Session Exclusions & ScriptCache Gate | 0/TBD | Not started | - |
| 13. App Leftovers & LaunchAgents | 0/TBD | Not started | - |
| 14. Integrated Validation & macOS Acceptance | 0/TBD | Not started | - |

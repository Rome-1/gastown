---
title: "Non-Claude Provider Parity Audit — Classification Against Current Main"
date: 2026-04-14
status: classification
wasteland_report: w-7ed35a727f
tracker: gastownhall/gascity#672 (follow-up to #350)
audited_commit: gastownhall/gascity@ed5bf5ce
---

This document classifies each finding from wasteland report `w-7ed35a727f`
(non-Claude provider parity audit, stored upstream at
`gastownhall/gascity:engdocs/archive/analysis/non-claude-provider-parity-audit.md`)
against current `gastownhall/gascity` main at commit `ed5bf5ce`.

The original audit (PR #639, merged 2026-04-13) identified 8 gaps across 10
provider runtimes. This review re-verifies each gap against the post-audit
state of main, classifies them as **fixed**, **stale**, or **still-open**,
and opens follow-up beads for the still-open items.

Status as of audited commit:

- **8 gaps classified**
  - 1 **fixed** (Gap 6 — `PrintArgs`)
  - 6 **still open** (Gaps 1, 2, 3, 5, 7, 8)
  - 1 **partially addressed** (Gap 4 — infrastructure landed, amp/auggie still uncovered)
- **Theme 1** (hook/runtime parity): largely unchanged from audit time
- **Theme 2** (queued/deferred nudge delivery parity): **infrastructure landed**
  via `NeedsNudgePoller` (PR #656); residual gap is amp/auggie opt-in

## Classification table

| # | Finding | Severity | Classification | Follow-up bead |
|---|---------|----------|----------------|----------------|
| 1 | Session resume silent no-op (all non-Claude) | Show-stopper | **still open** | `gt-j9zo` |
| 2 | `SessionIDFlag` only defined for Claude | Show-stopper (Codex) / Friction | **still open** | `gt-ftk5` |
| 3 | Missing PreCompact-equivalent on Codex, Copilot | Friction → Show-stopper (long sessions) | **still open** | `gt-v1lg` |
| 4 | Amp/Auggie have no hook-driven coordination | Show-stopper | **partially addressed** | `gt-716l` |
| 5 | Hook event vocabulary undocumented | Polish | **still open** | `gt-nigz` |
| 6 | `PrintArgs` unused | Polish | **fixed** | — |
| 7 | `InstructionsFile` never materialized in workdir | Friction | **still open** | `gt-v78q` |
| 8 | `gc doctor` misses provider capability gaps | Friction | **still open** | `gt-e4os` |

## Detail by finding

### Gap 1 — Session resume is a silent no-op for every non-Claude provider

**Audit claim:** Only `claude` defines `ResumeFlag` / `ResumeStyle`; every
other builtin leaves them empty, so `resolveResumeCommand`
(`cmd/gc/session_reconciler.go:989-1007`) returns the command unchanged
on restart and silently starts a fresh process.

**Verified on `ed5bf5ce`:**

- `internal/config/provider.go:247-248` — claude has
  `ResumeFlag: "--resume"`, `ResumeStyle: "flag"`, `SessionIDFlag: "--session-id"`.
- `internal/config/provider.go:292-463` — codex, gemini, cursor, copilot,
  amp, opencode, auggie, pi, omp — **none populate `ResumeFlag`**.
- `cmd/gc/session_reconciler.go:986-1004` — `resolveResumeCommand` still
  short-circuits on `rp.ResumeFlag == ""`.

**Classification:** **still open.** No change since the audit.

Note: the gastown-side analogue (`internal/config/agents.go` in
`gastownhall/gastown`) populates `ResumeFlag` for claude, gemini, codex,
cursor, auggie, amp, copilot — so maintainers have a reference for CLI
flag shapes when porting to gascity's `ProviderSpec`.

**Follow-up:** `gt-j9zo` (p1).

### Gap 2 — `SessionIDFlag` only defined for Claude

**Audit claim:** Only claude has `SessionIDFlag: "--session-id"`. Codex
supports `--session-id <uuid>` but `SessionIDFlag` is unset.

**Verified on `ed5bf5ce`:**

- `internal/config/provider.go:249` — claude is the only provider with a
  non-empty `SessionIDFlag`.
- `cmd/gc/session_reconciler.go:980-982` — consumer gated on
  `rp.SessionIDFlag != ""`.

**Classification:** **still open.** No change since the audit.

**Follow-up:** `gt-ftk5` (p1).

### Gap 3 — Missing PreCompact equivalent on Codex and Copilot

**Audit claim:** Claude (`PreCompact`), Gemini (`PreCompress`), Cursor
(`preCompact`), OpenCode (`session.compacted`) have pre-compaction hooks
wired. Codex and Copilot do not.

**Verified on `ed5bf5ce`:**

```
internal/hooks/config/claude.json:16     "PreCompact": [
internal/hooks/config/cursor.json:9      "preCompact": [
internal/hooks/config/gemini.json:14     "PreCompress": [
internal/hooks/config/omp.ts:32          "session.compacted": ...
internal/hooks/config/opencode.js:61     case "session.compacted":
internal/hooks/config/pi.js:32           "session.compacted": ...
```

`internal/hooks/config/codex.json` and `internal/hooks/config/copilot.json`
still contain no pre-compact event.

**Classification:** **still open.** No change since the audit.

**Follow-up:** `gt-v1lg` (p2).

### Gap 4 — Amp/Auggie cannot receive hook-driven coordination

**Audit claim:** `SupportsHooks: false` (implicit default) on Amp and
Auggie; no lifecycle mechanism; nudges queue forever.

**Verified on `ed5bf5ce`:**

- `internal/config/provider.go:417-424` (amp) and `:437-444` (auggie) —
  both still implicit `SupportsHooks: false`.
- **New infrastructure (PR #656) landed after the audit:**
  `ProviderSpec.NeedsNudgePoller` (`internal/config/provider.go:66`) and
  `maybeStartNudgePoller` (`cmd/gc/cmd_nudge.go:776`) enable a background
  poller drain path for providers that report idle. Codex opted in
  (`provider.go:304`).
- **But amp and auggie did not opt in.** `engdocs/architecture/nudge-delivery.md:87-88`
  explicitly flags both as "no drain today" and cross-references this
  audit's gap 4 as the open item.

**Classification:** **partially addressed.** The capability model from the
audit's "fix option 2" (polling fallback) landed as infrastructure.
Amp and Auggie still have no drain path — they haven't been opted in,
presumably because their idle-detection story is unclear.

**Follow-up:** `gt-716l` (p1).

### Gap 5 — Hook event vocabulary is inconsistent across providers

**Audit claim:** Each provider's hook config uses different event naming;
no central document lists the mapping, so omissions are invisible at
review time (likely cause of gap 3).

**Verified on `ed5bf5ce`:**

- `internal/hooks/config/` contains the per-provider configs but no
  `README.md`.
- No other file centralizes the mapping table (confirmed by search for
  the audit's table text).

**Classification:** **still open.** No change since the audit.

**Follow-up:** `gt-nigz` (p3).

### Gap 6 — `PrintArgs` is defined per-provider but has no consumer

**Audit claim:** `PrintArgs` set for claude, codex, gemini but no references
under `cmd/gc/`. Either dead code or staged config ahead of consumer.

**Verified on `ed5bf5ce`:**

- `internal/api/title_generate.go:38` — **consumer now exists**:

  ```go
  // PrintArgs or the subprocess fails.
  ```

  with supporting test cases in `internal/api/title_generate_test.go`
  (e.g. `TestGenerateTitle_NoPrintArgs` at line 77).
- `internal/config/resolve.go:232-233, 394-396` — resolution path hooks
  `PrintArgs` into `ResolvedProvider`.

**Classification:** **fixed.** `PrintArgs` is now consumed for the
one-shot title-generation subprocess path. The audit's concern that the
field was dead code no longer applies.

No follow-up needed.

### Gap 7 — `InstructionsFile` is used for content lookup but never as a CLI flag or file copy

**Audit claim:** `InstructionsFile` is a hint used when *generating*
quality-gate hints; never used to copy/stage an actual instructions file
in the agent's workdir. Non-Claude agents silently start with no project
instructions when the repo only ships `CLAUDE.md`.

**Verified on `ed5bf5ce`:**

`grep -rn 'InstructionsFile' --include='*.go'` in the gascity tree
(excluding tests) returns only:

```
internal/config/provider.go    (field declarations + per-provider defaults)
internal/config/resolve.go     (merge + default-to-AGENTS.md at line 349-351)
```

No consumer under `cmd/gc/` or elsewhere copies/symlinks/stages the
file's content.

**Classification:** **still open.** No change since the audit.

**Follow-up:** `gt-v78q` (p2).

### Gap 8 — No healthcheck or doctor coverage for per-provider capabilities

**Audit claim:** `gc doctor` doesn't flag `SupportsHooks: false`,
`ResumeFlag: ""`, or missing hook config files for `SupportsHooks: true`
providers.

**Verified on `ed5bf5ce`:**

- `internal/doctor/checks.go` — no references to `SupportsHooks`,
  `ResumeFlag`, or a `provider-parity` check.
- `cmd/gc/cmd_doctor.go:61-138` registers `CityStructureCheck`,
  `CityConfigCheck`, `ConfigValidCheck`, `ConfigRefsCheck`,
  `BuiltinPackFamilyCheck`, `ConfigSemanticsCheck`, `DurationRangeCheck`,
  binary checks, controller, session state, dolt. No provider-capability
  check.

**Classification:** **still open.** No change since the audit.

**Follow-up:** `gt-e4os` (p2).

## Theme-level reconfirmation

### Theme 1 — hook/runtime parity for Codex, Copilot, and other non-Claude providers

Still largely as the audit described. Gaps 1, 2, 3, 4, 5, 7 all live in
Theme 1 and six of them remain open. The concrete shovel-ready work is:

1. Port the gastown-side `ResumeFlag` / `ResumeStyle` tables into
   gascity's `ProviderSpec` (Gap 1, `gt-j9zo`).
2. Populate `SessionIDFlag` for Codex at minimum (Gap 2, `gt-ftk5`).
3. Decide amp/auggie strategy — opt into `NeedsNudgePoller` vs first-class
   "no hooks" doctor warning (Gap 4, `gt-716l`).

### Theme 2 — queued/deferred nudge delivery parity for non-Claude providers

**Infrastructure landed after the audit.** PR #656
(`feat(nudge): generalize queued nudge delivery via NeedsNudgePoller capability`)
introduced:

- `ProviderSpec.NeedsNudgePoller` + `ResolvedProvider.NeedsNudgePoller`
  (`internal/config/provider.go:62-66`, `:135`).
- Background poller in `cmd/gc/cmd_nudge.go` (`maybeStartNudgePoller`,
  `ensureNudgePoller`).
- Capability hydration in `cmd/gc/cmd_nudge.go:613-651` to handle custom
  provider merges.
- Wait path integration in `cmd/gc/cmd_wait.go:682`.
- Architecture doc: `engdocs/architecture/nudge-delivery.md`.

Codex — the audit's canonical case — is opted in
(`provider.go:304: NeedsNudgePoller: true`).

**Residual gap:** amp and auggie remain uncovered (tracked under Gap 4,
`gt-716l`). The nudge-delivery doc itself acknowledges this and points
back at this audit.

## Next steps for maintainers

The audit's own "ship order" remains valid for the residual gaps:

1. **`gt-j9zo`** (Gap 1) — fix resume for Codex first. Highest user impact,
   least ambiguous CLI shape (documented `codex resume <session-id>`).
2. **`gt-716l`** (Gap 4) — decide amp/auggie strategy (opt into poller vs.
   doctor-warning-only).
3. **`gt-v1lg`** (Gap 3) — wire Codex PreCompact equivalent.
4. **`gt-v78q`** (Gap 7) — stage per-provider instructions files at session
   start, or document the AGENTS.md convention.

Lower-priority cleanup: `gt-ftk5` (Gap 2), `gt-nigz` (Gap 5), `gt-e4os`
(Gap 8). Gap 6 is done.

## Methodology

- Checked out `gastownhall/gascity@ed5bf5ce` locally.
- For each of the 8 gaps: re-ran the exact greps/file reads cited in the
  audit report against the current tree.
- Cross-checked recent merged PRs that touch the same files (notably
  PR #656 for the nudge-delivery capability).
- Each follow-up bead cites file paths and line numbers against `ed5bf5ce`
  so future work is reproducible.

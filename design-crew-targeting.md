# Design: Crew-Aware Targeting for `gt sling`

**Wanted:** w-0a0e3a9dc9
**Issue:** gastownhall/gascity#21
**Author:** gastown/crew/charlie

## Problem

`gt assign mel "Fix the bug"` just works — it scans rigs and finds mel.
`gt sling gt-abc mel` doesn't — you need `gt sling gt-abc gastown/crew/mel`
or `gt sling gt-abc gastown --crew mel`.

## Core solution

Add ~15 lines to `resolveTarget()` in `sling_target.go`. After the rig name
check (step 3) and before existing agent resolution (step 4), try resolving
bare names as crew members using the existing `inferRigFromCrewName()`:

```go
// Bare crew name resolution
if !strings.Contains(target, "/") {
    townRoot := opts.TownRoot
    if townRoot == "" {
        townRoot, _ = workspace.FindFromCwd()
    }
    if townRoot != "" {
        rigName, crewErr := inferRigFromCrewName(townRoot, target)
        if crewErr == nil {
            qualifiedTarget := fmt.Sprintf("%s/crew/%s", rigName, target)
            fmt.Printf("Resolved crew member %q → %s\n", target, qualifiedTarget)
            agentID, pane, workDir, err := resolveTargetAgentFn(qualifiedTarget)
            if err != nil {
                return nil, fmt.Errorf(
                    "crew member %q found in %s but has no active session\n"+
                        "  Start the session: gt crew start %s",
                    target, rigName, target,
                )
            }
            result.Agent = agentID
            result.Pane = pane
            result.WorkDir = workDir
            return result, nil
        }
        if strings.Contains(crewErr.Error(), "multiple rigs") {
            return nil, fmt.Errorf(
                "%w\n  Use explicit targeting: gt sling <bead> <rig>/crew/%s",
                crewErr, target,
            )
        }
        // Not a crew member — fall through to existing resolution
    }
}
```

**That's it.** This gives us:

```bash
gt sling gt-abc mel          # → resolves to gastown/crew/mel
gt sling gt-abc mel          # ambiguous → "exists in multiple rigs: gastown, beads"
gt sling gt-abc mel          # offline → "no active session, try: gt crew start mel"
```

### Why it works

- `inferRigFromCrewName()` already exists (helpers.go:46) — `gt assign` uses it
- `ValidateTarget` already passes bare names through (sling_validate.go:30)
- No match falls through to existing resolution — zero breakage risk
- Rig names win over crew names (step 3 runs first) — correct precedence

### Tests (minimum)

| Test | Input | Expected |
|------|-------|----------|
| Bare name, one match | `"mel"` | Resolves to `gastown/crew/mel` |
| Bare name, multiple rigs | `"mel"` (2 rigs) | Ambiguity error |
| Bare name, no match | `"nobody"` | Falls through to existing resolution |
| Rig name shadows crew | `"gastown"` | Resolves as rig (polecat spawn) |
| Crew dir exists, no session | `"mel"` (no tmux) | "no active session" error |

Test seam: `resolveTargetAgentFn` is a package-level var (sling_target.go:17)
— mock it to avoid tmux. Create temp dirs for crew layout.

### Decisions

- **Hook on session, not polecat spawn.** Crew members are persistent. If you
  want a polecat, use the rig name.
- **Extend `gt sling`, no new command.** `gt assign` already handles
  create-and-route.
- **This targets `gt` first.** Gas City (`gc`) adaptation is follow-up.

---

## Extensions (implement if/when needed)

The following were identified during review. None are required for the core
solution to work. Ship the above first, harden later.

### E1: `gt nudge` bare-name resolution

Same resolution in `gt nudge` so `gt nudge mel "check your mail"` works.
Requires extracting the resolution into a `resolveBareCrew()` helper that
both `resolveTarget()` and `nudge.go`'s `runNudge()` call.

### E2: `isKnownRole()` extraction

The core solution's bare-name check doesn't guard against role names like
"witness" or "ref". These are already rejected by `IsRigName` in step 3
(polecat_spawn.go:441), so they won't reach crew resolution. But if
`IsRigName`'s exclusion list ever changes, they could leak through.

Hardening: extract the role exclusion list into a shared `isKnownRole()`
function. Reserved names that can never be bare crew targets:

| Name | Why |
|------|-----|
| "mayor", "may", "deacon", "dea" | Role abbreviations |
| "witness", "wit", "refinery", "ref" | Role abbreviations |
| "crew", "polecats" | Role keywords |

### E3: Constrain `inferRigFromCrewName` to registered rigs

Currently scans all top-level directories in the town root. A false positive
is possible if `mayor/crew/mel/` exists. Hardening: load rig names from
`rigs.json` instead of `os.ReadDir`.

### E4: `--crew` flag rig-name guard

`gt sling gt-abc mel --crew mel` expands to `mel/crew/mel` which fails
confusingly. Hardening: validate the `--crew` base target is an actual rig.

### E5: ValidateTarget comment

`ValidateTarget` passes bare names through by returning early on slash-free
targets. The crew resolution depends on this. Hardening: add a comment so
future changes don't accidentally break it.

### E6: Config-based aliases

DreadPirateRobertz (issue #21 comment) suggested a `[crew]` config section.
Filesystem scanning covers the common case. Config becomes warranted for:
(a) role-based aliases ("review-lead" → "mel"), (b) shorthand for long names,
(c) default rig preference for ambiguous names, (d) non-filesystem crew
identities. Revisit after seeing real usage.

### E7: `gt mail send` bare-name resolution

Different subsystem (mail addressing). Follow-up after sling/nudge proves out.

### E8: `gt assign` / `gt sling` dispatch unification

`gt assign` writes bead metadata directly (succeeds offline). `gt sling` goes
through `resolveTarget()` (fails if session offline). This asymmetry is
intentional ("leave a note" vs "hand them work now") but may confuse users.
Document or unify later if it becomes a problem.

---

## Reference: full resolution precedence

After the core change, `resolveTarget()` follows this hierarchy:

```
1. Empty/"." → self-sling
2. Dog target ("deacon/dogs", "dog:") → pool or named dog
3. Rig name → auto-spawn polecat
4. Bare crew name → scan rigs → <rig>/crew/<name>  ← NEW
5. Existing agent path → session lookup + dead polecat fallback
```

## Reference: full test matrix

For completeness, the extended test matrix covering all edge cases:

| Test | Input | Expected |
|------|-------|----------|
| Bare name, one rig match | `"mel"` | Resolves to `gastown/crew/mel` |
| Bare name, multiple rigs | `"mel"` (in gastown + beads) | Error with rig list |
| Bare name, no match | `"nobody"` | Falls through to step 5 |
| Fallthrough produces correct error | `"nobody"` | Error from tmux lookup |
| Rig name shadows crew name | `"gastown"` (rig + crew "gastown") | Resolves as rig |
| Qualified path still works | `"gastown/crew/mel"` | Resolves via path resolution |
| `--crew` flag still works | `"gastown" --crew mel` | Resolves via flag expansion |
| `--crew` with non-rig target | `"mel" --crew mel` | Error (needs E4) |
| Bare role name | `"witness"` | NOT treated as crew |
| Role abbreviation | `"wit"`, `"ref"`, `"dea"` | NOT treated as crew |
| Role name case-insensitive | `"Witness"`, `"CREW"` | NOT treated as crew (needs E2) |
| Dog target pattern | `"dog:mel"` | Resolves as dog, not crew |
| Self-target with crew name | `"."` | Self-sling |
| Crew dir exists, session dead | `"mel"` (no tmux) | "no active session" error |
| Case sensitivity (crew name) | `"Mel"` vs `"mel"` | Filesystem-dependent |
| Ambiguity error format | `"mel"` in 2 rigs | Both rig names + fix command |
| Non-rig dir with crew/ | `"mayor"` has crew/ | NOT matched (needs E3) |
| rigs.json missing | No rigs.json, crew exists | Matched via dir scan fallback |

# Design: Crew-Aware Targeting for `gt sling`

**Wanted:** w-0a0e3a9dc9
**Issue:** gastownhall/gascity#21
**Author:** gastown/crew/charlie
**Status:** v3 (final — revised after 5-polecat review)

## Scoping note

Issue #21 asks for crew-aware targeting in **Gas City** (`gc`). This design
targets **Gas Town** (`gt`) first — the upstream implementation that `gc`
adapts from. The `resolveBareCrew()` function boundary (Change 6) is
designed to be pluggable so Gas City can swap in a registry lookup without
changing `resolveTarget()`. A `gc`-specific adaptation is follow-up work.

We extend the existing `gt sling` command rather than introducing a new
command or abstraction. The issue's design question ("Should this live on
`gc sling`, a new `gc assign`, or a `gc agent target` abstraction?") is
answered: **extend `gt sling`** with bare-name resolution.

## Problem

Users dispatch work to crew members frequently but must use verbose targeting:

```bash
gt sling gt-abc gastown --crew mel     # --crew flag requires rig target
gt sling gt-abc gastown/crew/mel       # fully qualified path
```

Meanwhile, `gt assign` already accepts bare names:

```bash
gt assign mel "Fix the auth bug"       # Just works — scans rigs for "mel"
```

The gap: `gt sling` doesn't resolve bare crew names. Users must know the rig
name or use the `--crew` flag (which still requires specifying the rig).

## Current State

### What exists

| Command | Bare name? | Notes |
|---------|-----------|-------|
| `gt assign mel "..."` | Yes | Scans rigs via `inferRigFromCrewName()` |
| `gt sling <bead> gastown --crew mel` | No (needs rig) | Flag expands to `gastown/crew/mel` |
| `gt sling <bead> gastown/crew/mel` | No (needs rig) | Resolved by `resolvePathToSession()` |
| `gt nudge gastown/crew/mel "..."` | No (needs rig) | Uses mail addressing |

### Resolution hierarchy in `resolveTarget()`

```
1. Empty/"." → self-sling
2. Dog target ("deacon/dogs", "dog:") → pool or named dog
3. Rig name → auto-spawn polecat
4. Existing agent path → session lookup + dead polecat fallback
```

Bare crew names currently fall through to step 4, which calls
`resolveTargetAgent()` → `resolveRoleToSession()`. The `default` case in
`resolveRoleToSession` (handoff.go:642) passes unknown bare names through
as literal tmux session names, which then fail at `getSessionPane()` with
an opaque "no session found" error.

### Why bare names pass `ValidateTarget`

`ValidateTarget` (sling_validate.go:30) has an early return for slash-free
targets: `if !strings.Contains(target, "/") { return nil }`. This means
bare names like "mel" bypass validation entirely. This is correct behavior
that enables the new crew resolution step — a comment should be added to
`ValidateTarget` documenting this dependency.

## Proposal: Add Crew Name Resolution to `resolveTarget()`

### Design: Insert bare-name resolution between steps 3 and 4

Add a new resolution step that fires when the target:
- Is a single word (no "/" separator)
- Is NOT a known rig name (checked by `IsRigName` in step 3)
- Is NOT a known role (checked by new `isKnownRole()` function)
- Is NOT a dog target (checked in step 2)

When these conditions are met, scan registered rigs for a crew directory
matching the name (reusing `inferRigFromCrewName()` logic).

### Resolution precedence (updated)

```
1. Empty/"." → self-sling
2. Dog target → pool or named dog
3. Rig name → auto-spawn polecat
4. **Bare crew name → scan rigs, resolve to <rig>/crew/<name>** ← NEW
5. Existing agent path → session lookup + dead polecat fallback
```

### Why this placement?

- **After rig check**: If someone has a crew member named "beads" and a rig
  named "beads", the rig wins. This is consistent — rig names are the higher
  namespace. A crew member should never shadow a rig.
- **Before path resolution**: Once we resolve the bare name to a qualified
  path, the existing path resolution handles the rest.
- **No breakage**: Any target that currently works still works. Bare names
  that aren't crew members fall through to step 5 unchanged.

### Reserved names (unreachable as bare crew targets)

Some bare names are intercepted by earlier resolution steps and cannot
resolve as crew members. This is by design — do not create crew members
with these names:

| Name | Intercepted by | Why |
|------|---------------|-----|
| Any rig name (e.g., "gastown") | Step 3: `IsRigName` | Rig namespace wins |
| "mayor", "may" | Step 3: `IsRigName` role exclusion | Role abbreviation |
| "deacon", "dea" | Step 2: `IsDogTarget` / Step 3 | Dog dispatch / role abbreviation |
| "witness", "wit" | Step 3: `IsRigName` role exclusion | Role abbreviation |
| "refinery", "ref" | Step 3: `IsRigName` role exclusion | Role abbreviation |
| "crew" | Step 3: `IsRigName` role exclusion | Role keyword |
| "polecats" | Step 4: `isKnownRole` guard | Role keyword |
| "dog:*" prefix | Step 2: `IsDogTarget` | Dog dispatch syntax |

**Case sensitivity**: Role exclusion via `isKnownRole()` is case-insensitive
(`strings.ToLower`). Crew directory lookup is filesystem-dependent (Linux:
case-sensitive, macOS: case-insensitive by default). This means `"Witness"`
is blocked by role exclusion, but `"Mel"` vs `"mel"` depends on the OS.

**Implementation**: Extract the role exclusion list from `IsRigName`
(polecat_spawn.go:441) into a shared `isKnownRole()` function used by both
`IsRigName` and the new crew resolution step. This prevents the lists from
drifting.

### Ambiguity rules

Reuse the same logic as `gt assign` (`inferRigFromCrewName`):

| Scenario | Behavior |
|----------|----------|
| "mel" exists in exactly one rig | Resolves to `<rig>/crew/mel` |
| "mel" exists in multiple rigs | Error with rig list and fix command |
| "mel" exists in no rigs | Falls through to step 5 (existing agent path resolution) |
| "gastown/crew/mel" (fully qualified) | Already handled by `resolvePathToSession()` |

Note: `"gastown/mel"` (rig-qualified shorthand without `/crew/`) is also
handled by `resolvePathToSession()`, which checks for a crew directory at
`<rig>/crew/<name>` before falling back to polecat resolution. This is
existing behavior, not new to this design.

### Error messages

```
# Ambiguous
$ gt sling gt-abc mel
Error: crew member "mel" exists in multiple rigs: gastown, beads
  Use explicit targeting: gt sling gt-abc gastown/crew/mel

# Offline crew member (v1 requirement)
$ gt sling gt-abc mel
Error: crew member "mel" found in gastown but has no active session
  Start the session: gt crew start mel

# Not found (falls through to existing resolution, then fails at tmux lookup)
$ gt sling gt-abc nobody
Error: resolving target: no session found for "nobody"
```

## What about `gt sling <bead> mel` creating a polecat?

A subtle question: should `gt sling gt-abc mel` hook the bead on mel's
existing session, or spawn a polecat in mel's rig?

**Answer: Hook on mel's session.** Crew members are persistent — they don't
get polecats spawned for them. If you want a polecat in mel's rig, use the
rig name: `gt sling gt-abc gastown`. The `--crew` flag exists for the rare
case where you want a specific crew member targeted via rig dispatch syntax.

## Dispatch asymmetry: `gt assign` vs `gt sling`

`gt assign` creates a bead and hooks it via `bd update --status=hooked
--assignee=<agentID>`. It does NOT go through `resolveTarget()` — it writes
bead metadata directly and optionally nudges. `gt sling` goes through the
full dispatch pipeline (polecat spawn, dog dispatch, dead-polecat fallback).

This means:
- `gt sling gt-abc mel` (after this design) → `resolveTarget()` → fails if
  session is offline
- `gt assign mel "Fix bug"` → direct bead update → succeeds even if offline
  (work sits until next session start)

This asymmetry is intentional: `gt assign` is "leave a note on their desk,"
`gt sling` is "hand them the work right now." Unifying the dispatch path is
future work and may not be desirable.

## Scope decisions

### In scope (v1)

1. Bare crew name resolution in `gt sling` via `resolveTarget()`
2. Same resolution in `gt nudge` (same benefit, same logic — see Change 6)
3. Ambiguity error with helpful message
4. Improved error message for offline crew members (v1 requirement)
5. `isKnownRole()` extracted as shared function
6. Comment in `ValidateTarget` documenting bare-name passthrough dependency
7. `--crew` flag guard: validate base target is a rig name
8. Tests for resolution precedence, ambiguity, reserved names, fallthrough

### Out of scope (v1)

1. **Config-based aliases** — DreadPirateRobertz suggested a `[crew]` config
   section mapping friendly names to qualified identities. Filesystem scanning
   covers the common case of unique crew names. Config aliases become warranted
   when any of these arise: (a) role-based aliases ("review-lead" → "mel"),
   (b) shorthand for long names ("prom" → "prometheus-monitoring-agent"),
   (c) default rig preference for ambiguous names, (d) non-filesystem-backed
   crew identities. We'll revisit after seeing real usage patterns.

2. **Create-and-route one-shot command** — Issue #21 asks whether a one-shot
   create-and-route command should exist. `gt assign` already fills this role
   (create + hook + optional nudge). No new command needed. If `gt assign`
   proves insufficient, that's a separate issue.

3. **`gt mail send` bare-name resolution** — Mail addressing uses a different
   subsystem. Recommend as a follow-up once sling/nudge resolution proves out.

4. **Cross-town crew resolution** — Only resolves crew within the current town.

5. **Crew defaults in config** — No default crew target.

## Implementation plan

### Change 1: Extract `isKnownRole()`

Create a shared function in `sling_target.go` that returns true for role
names and abbreviations. Source the list from the same place `IsRigName`
(polecat_spawn.go:441) uses its role exclusion, to prevent drift:

```go
// isKnownRole returns true for role names and abbreviations that should not
// be treated as bare crew names. Case-insensitive.
func isKnownRole(name string) bool {
    knownRoles := map[string]bool{
        "mayor": true, "may": true,
        "deacon": true, "dea": true,
        "witness": true, "wit": true,
        "refinery": true, "ref": true,
        "crew": true, "polecats": true,
    }
    return knownRoles[strings.ToLower(name)]
}
```

### Change 2: Constrain `inferRigFromCrewName` to registered rigs

The current implementation (helpers.go:46) scans all top-level directories
in the town root, not just registered rigs. A `mayor/crew/mel/` directory
would be a false positive. Constrain to rigs listed in `rigs.json`:

```go
// Instead of os.ReadDir(townRoot), load rig names from rigs.json
rigNames, err := loadRigNames(townRoot)  // reads mayor/rigs.json
if err != nil {
    // Fallback to directory scan (backwards compat for towns without rigs.json)
    entries, err = os.ReadDir(townRoot)
    ...
}
```

### Change 3: Extract `resolveBareCrew()` helper

Extract the bare-name resolution logic into a shared helper that both
`resolveTarget()` (Change 4) and `gt nudge` (Change 7) can call:

```go
// resolveBareCrew attempts to resolve a bare name to a qualified crew path.
// Returns ("", nil) if the name doesn't match any crew member (fall through).
// Returns ("", error) for ambiguity.
// Returns ("<rig>/crew/<name>", nil) on match.
func resolveBareCrew(townRoot, target string) (string, error) {
    if strings.Contains(target, "/") || isKnownRole(target) {
        return "", nil
    }
    rigName, err := inferRigFromCrewName(townRoot, target)
    if err != nil {
        if strings.Contains(err.Error(), "multiple rigs") {
            return "", err
        }
        return "", nil // not found — fall through
    }
    return fmt.Sprintf("%s/crew/%s", rigName, target), nil
}
```

### Change 4: Add crew name resolution step to `resolveTarget()`

In `sling_target.go`, after the rig name check and before existing agent
resolution. Calls `resolveBareCrew()` then does a direct call to
`resolveTargetAgentFn` — no recursion:

```go
// Step 4: Bare crew name resolution
if townRoot := opts.TownRoot; townRoot != "" || true {
    if townRoot == "" {
        townRoot, _ = workspace.FindFromCwd()
    }
    if townRoot != "" {
        qualifiedTarget, crewErr := resolveBareCrew(townRoot, target)
        if crewErr != nil {
            return nil, fmt.Errorf(
                "%w\n  Use explicit targeting: gt sling <bead> <rig>/crew/%s",
                crewErr, target,
            )
        }
        if qualifiedTarget != "" {
            fmt.Printf("Resolved crew member %q → %s\n", target, qualifiedTarget)
            agentID, pane, workDir, err := resolveTargetAgentFn(qualifiedTarget)
            if err != nil {
                // Crew directory exists but session isn't running
                return nil, fmt.Errorf(
                    "crew member %q found in %s but has no active session\n"+
                        "  Start the session: gt crew start %s",
                    target, strings.Split(qualifiedTarget, "/")[0], target,
                )
            }
            result.Agent = agentID
            result.Pane = pane
            result.WorkDir = workDir
            return result, nil
        }
        // Not matched — fall through to existing agent resolution (step 5)
    }
}
```

Key implementation notes:
- Calls `resolveBareCrew()` helper (Change 3), not inline logic
- Direct call to `resolveTargetAgentFn` instead of recursion
- Custom error message for offline crew (suggests `gt crew start`)
- Resolution message goes to stdout; production code should use the
  command's output writer or a logger rather than raw `fmt.Printf`

### Change 5: Add `--crew` flag rig-name guard

In `sling.go`, validate that the `--crew` flag's base target is actually a
rig name. Catches `gt sling gt-abc mel --crew mel` early:

```go
if slingCrew != "" {
    if len(args) < 2 {
        return fmt.Errorf("--crew requires a rig target argument")
    }
    target := args[len(args)-1]
    if _, isRig := IsRigName(target); !isRig {
        return fmt.Errorf(
            "--crew requires a rig target, but %q is not a rig\n"+
                "  Use: gt sling <bead> %s (bare crew name resolves automatically)",
            target, slingCrew,
        )
    }
    args[len(args)-1] = target + "/crew/" + slingCrew
}
```

### Change 6: Add ValidateTarget comment

In `sling_validate.go`:

```go
// Bare names (no "/") intentionally pass through validation.
// Crew name resolution in resolveTarget() depends on this — see
// resolveBareCrew() in sling_target.go.
if !strings.Contains(target, "/") {
    return nil
}
```

### Change 7: Same resolution in `gt nudge`

`gt nudge` resolves targets through a separate path (mail addressing, not
`resolveTarget()`). The bare-name resolution needs to be applied in the
nudge command's target resolution.

Concrete integration point: in `nudge.go`'s `runNudge()` function, before
the target is passed to mail address construction, call `resolveBareCrew()`:

```go
// Before constructing mail address from target
if townRoot, err := workspace.FindFromCwd(); err == nil {
    if qualified, err := resolveBareCrew(townRoot, target); err != nil {
        return err
    } else if qualified != "" {
        fmt.Printf("Resolved crew member %q → %s\n", target, qualified)
        target = qualified
    }
}
```

### Change 8: Tests

| Test | Input | Expected |
|------|-------|----------|
| Bare name, one rig match | `"mel"` | Resolves to `gastown/crew/mel` |
| Bare name, multiple rigs | `"mel"` (in gastown + beads) | Error with rig list |
| Bare name, no match | `"nobody"` | Falls through to step 5 |
| Fallthrough produces correct error | `"nobody"` | Error from tmux lookup, not crew resolution |
| Rig name shadows crew name | `"gastown"` (rig + crew named "gastown") | Resolves as rig (polecat spawn) |
| Qualified path still works | `"gastown/crew/mel"` | Resolves via path resolution |
| `--crew` flag still works | `"gastown" --crew mel` | Resolves via flag expansion |
| `--crew` with non-rig target | `"mel" --crew mel` | Error: "--crew requires a rig target" |
| Bare role name | `"witness"` | NOT treated as crew |
| Role abbreviation | `"wit"`, `"ref"`, `"dea"` | NOT treated as crew |
| Role name case-insensitive | `"Witness"`, `"CREW"` | NOT treated as crew |
| Dog target pattern | `"dog:mel"` | Resolves as dog, not crew |
| Self-target with crew name | `"."` | Self-sling (not crew resolution) |
| Crew dir exists, session dead | `"mel"` (dir exists, no tmux) | Error with "start session" suggestion |
| Case sensitivity (crew name) | `"Mel"` vs `"mel"` | Filesystem-dependent (Linux: case-sensitive) |
| Ambiguity error format | `"mel"` in 2 rigs | Contains both rig names AND fix command |
| Non-rig directory with crew/ | `"mayor"` has crew/ subdir | NOT matched (constrained to rigs.json) |
| rigs.json missing (fallback) | No rigs.json, crew dir exists | Matched via directory scan fallback |

**Test seam**: `resolveTargetAgentFn` is already a package-level var
(sling_target.go:17). Tests mock it to avoid tmux dependency:

1. Create temp dir with `tmpdir/gastown/crew/mel/` structure
2. Add minimal `mayor/rigs.json` listing "gastown" as a rig
3. Override `resolveTargetAgentFn` to return canned response
4. Call `resolveTarget("mel", ResolveTargetOptions{TownRoot: tmpdir})`
5. Verify `resolveTargetAgentFn` was called with `"gastown/crew/mel"`

## Gas City compatibility

This design targets Gas Town (`gt`) first. The crew resolution is behind the
`resolveBareCrew()` function boundary, which depends only on
`inferRigFromCrewName()`. This function can be replaced with a registry
lookup in Gas City without changing `resolveTarget()`. The design does not
couple crew resolution to the gastown filesystem layout at the call site —
only the helper knows about directory scanning.

Gas City adaptation is a separate follow-up task.

## Resolved questions

1. **`gt mail send` bare names**: Deferred to follow-up. Different subsystem.

2. **Print resolved target**: Yes. Print `Resolved crew member "mel" →
   gastown/crew/mel` on resolution. Production code should use the command's
   output writer or a structured logger rather than raw `fmt.Printf`.

3. **Offline crew member error**: Promoted to v1 requirement. Error message
   suggests `gt crew start <name>`.

4. **New command vs extend existing**: Extend `gt sling`. No new command or
   abstraction needed.

5. **Create-and-route one-shot**: `gt assign` already fills this role.
   Deferred unless `gt assign` proves insufficient.

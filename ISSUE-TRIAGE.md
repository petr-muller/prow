# Triage for Issue #606

**Status**: In Progress
**Created**: 2026-04-29

## Issue Information

- **Issue Number**: #606
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/606
- **Title**: DCO plugin trusted_apps config not working
- **Author**: clubanderson
- **Created**: 2026-01-27
- **Labels**: kind/bug, lifecycle/stale

## Issue Summary

The `trusted_apps` configuration option for the DCO plugin doesn't work. Despite correct configuration (`trusted_apps: [Copilot]`), the DCO plugin still fails commits from GitHub Copilot (`app/copilot-swe-agent`). The reporter has verified the config is loaded in the plugins configmap and that the commit author login matches `Copilot`.

## Initial Validation

**Assessment**: LEGITIMATE

### Analysis

The issue reports that the `trusted_apps` configuration option for the DCO plugin is not functioning — commits from GitHub Copilot fail DCO checks despite being listed as a trusted app. This is a bug report for the DCO plugin code that lives in this repository.

**Issue Category**: Bug

**Repository Scope Check**:
- Component mentioned: DCO plugin (`trusted_apps` feature)
- Exists in this repo: Yes
- Relevant code paths: `pkg/plugins/dco/dco.go`, `pkg/plugins/dco/dco_test.go`, `pkg/plugins/config.go`

**Information Completeness**:
- Sufficient detail provided: Yes
- The reporter provides: Prow version, exact config YAML, expected vs actual behavior, reproduction steps, environment verification (configmap check, API verification of author login, hook logs), and a workaround
- No information is missing — this is a well-written bug report

### Recommendation

Keep open and continue triage. This is a valid bug report for a specific feature (`trusted_apps`) of a plugin (`dco`) that lives in this repository. The reporter has done thorough investigation including verifying config loading and commit author identity. The `lifecycle/stale` label is from automated bot activity, not from maintainer assessment.

**Suggested Action**:
- Keep open and continue triage
- Remove `lifecycle/stale` label after triage is complete

## Code Research

### Current Implementation

**Primary Components**:
- DCO plugin: `pkg/plugins/dco/dco.go` — performs DCO sign-off checks on PR commits
- Trusted user check: `pkg/plugins/trigger/trigger.go:277-282` — shared utility that checks if a user is in the `trusted_apps` list
- Config: `pkg/plugins/config.go:797-815` — `Dco` struct with `TrustedApps []string`

**Architecture Overview**:
The DCO plugin's `handle()` function (line 291) first collects all commits missing DCO sign-off via `checkCommitMessages()`, then optionally filters out commits from trusted users via `filterTrustedUsers()`. The filtering calls `trigger.TrustedUser()`, which strips the `[bot]` suffix from the commit author login and compares against the `trusted_apps` list.

**Key Code Paths**:
1. `handle()`: `pkg/plugins/dco/dco.go:291-338` — main logic
2. `filterTrustedUsers()`: `pkg/plugins/dco/dco.go:127-143` — filters commits from trusted apps/orgs
3. `TrustedUser()`: `pkg/plugins/trigger/trigger.go:277-282` — checks `trusted_apps` list

### Root Cause Analysis

**Primary Cause**:
The `filterTrustedUsers()` function is only called when `config.SkipDCOCheckForMembers || config.SkipDCOCheckForCollaborators` is true (line 300). If neither skip flag is set — which is the common case when users only want `trusted_apps` — the filtering is never executed and `trusted_apps` has no effect.

**The Bug** (line 300):
```go
if config.SkipDCOCheckForMembers || config.SkipDCOCheckForCollaborators {
    commitsMissingDCO, err = filterTrustedUsers(gc, l, config.SkipDCOCheckForCollaborators, config.TrustedApps, config.TrustedOrg, org, repo, commitsMissingDCO)
```

The condition gates `filterTrustedUsers` on the skip flags, but `trusted_apps` should work independently of those flags.

**Contributing Factors**:
1. No test coverage for `trusted_apps` in the DCO plugin test suite (`dco_test.go` has zero tests for this feature)
2. The config documentation doesn't mention the dependency on skip flags
3. The `filterTrustedUsers` function handles multiple concerns (org members, collaborators, AND apps) in a single call, making the gating condition seem natural

### Existing Fix PR

**PR #681**: "Fix DCO plugin trusted_apps not working without skip flags"
- Author: vigneshakaviki
- State: Open, LGTM'd, Approved by @Prucek (member)
- Fix: Adds `|| len(config.TrustedApps) > 0` to the condition on line 300
- Tests: Adds one test case for trusted app without skip flags
- **Blocker**: `pull-prow-integration` CI job is failing (likely unrelated flake — unit tests pass)
- URL: https://github.com/kubernetes-sigs/prow/pull/681

### Test Coverage

**Existing Tests**:
- `pkg/plugins/dco/dco_test.go`: Tests for basic DCO, trusted org members, collaborators — but **zero tests** for `trusted_apps`
- `pkg/plugins/trigger/trigger_test.go`: Has tests for `TrustedUser()` with `[bot]` suffix handling — these pass

**Test Gaps** (partially addressed by PR #681):
- No test for `trusted_apps` working without skip flags (added in PR #681)
- No test for `trusted_apps` working with skip flags enabled
- No test for case-sensitivity of app name matching

### Proposed Solutions

#### Approach 1: Extend the Guard Condition (PR #681's approach)

**Description**: Add `|| len(config.TrustedApps) > 0` to the condition gating `filterTrustedUsers()`.

**Pros**:
- Minimal change (1 line)
- Already implemented, reviewed, and approved in PR #681
- Backwards compatible — no behavior change for existing configs

**Cons**:
- The function call still passes the skip flags to `filterTrustedUsers`, meaning org/collaborator checks happen even when only `trusted_apps` is configured (harmless but slightly wasteful)

**Complexity**: Low

**Backwards Compatibility**: Full — existing configs continue to work identically

#### Recommendation

**Preferred Approach**: Approach 1 (PR #681's fix). The fix is correct, minimal, and already approved. The integration test failure should be investigated — if it's an unrelated flake, the PR can be retested and merged.

**Key Implementation Considerations**:
1. Retest the integration job to confirm the failure is a flake
2. Consider adding more test cases for `trusted_apps` edge cases

## Effort Assessment

**Effort Level**: 1 - Easy (good-first-issue)

### Summary

A one-line conditional fix in a single file, with a clear root cause and an existing approved PR. The fix is fully backwards compatible and follows existing patterns.

### Factor Analysis

#### Scope of Changes
- **Assessment**: Small
- **Details**: 1 file modified (`pkg/plugins/dco/dco.go`), 1 line changed, 1 test file (`dco_test.go`) with ~25 lines of new test
- **Level Indication**: 1

#### Complexity
- **Assessment**: Simple
- **Details**: Adding one additional condition (`len(config.TrustedApps) > 0`) to an existing boolean guard. No new logic, no edge cases.
- **Level Indication**: 1

#### Required Expertise
- **Assessment**: Minimal
- **Details**: Basic Go, ability to read a conditional. No Prow-specific or domain expertise needed.
- **Level Indication**: 1

#### Clarity and Certainty
- **Assessment**: Well-defined
- **Details**: Root cause is unambiguous (line 300 gates the function on wrong conditions). Fix is clear and already validated in PR #681.
- **Level Indication**: 1

#### Testing Requirements
- **Assessment**: Simple
- **Details**: One unit test following existing patterns (PR #681 already adds this). No integration test changes needed.
- **Level Indication**: 1

#### Backwards Compatibility
- **Assessment**: Fully compatible
- **Details**: Additive only — existing configs with skip flags continue to work identically. Only changes behavior for configs that set `trusted_apps` without skip flags (which was broken before).
- **Level Indication**: 1

#### Architectural Alignment
- **Assessment**: Perfect fit
- **Details**: Fixes a bug in existing logic, no new patterns or abstractions introduced.
- **Level Indication**: 1

#### External Dependencies
- **Assessment**: None
- **Details**: Pure internal logic fix, no GitHub API or Kubernetes changes needed.
- **Level Indication**: 1

### Recommended Labels

- [x] `good-first-issue`: Extremely well-defined, minimal scope, clear fix
- [x] `kind/bug`: Fixing broken feature
- [x] `area/plugins`: DCO plugin

### Guidance for Contributors

This is an ideal first contribution: the root cause is identified, the fix is a single line, and there's already an approved PR (#681) to reference. A contributor would need to:
1. Understand the guard condition at `pkg/plugins/dco/dco.go:300`
2. Add `|| len(config.TrustedApps) > 0` to the condition
3. Add a unit test (see PR #681 for the pattern)

### Caveats and Considerations

PR #681 already implements this fix and has LGTM + approval. The practical next step is to get that PR merged rather than duplicate the work. The integration test failure on PR #681 should be investigated/retested.

## Proposed Issue Augmentation

### Title Change

- **Current**: DCO plugin trusted_apps config not working
- **Proposed**: DCO plugin: trusted_apps config has no effect without skip_dco_check flags
- **Rationale**: The current title is clear but vague about the nature of the bug. The new title specifies the exact condition under which `trusted_apps` fails — it's gated behind unrelated skip flags.

### Proposed GitHub Comment

```
/retitle DCO plugin: trusted_apps config has no effect without skip_dco_check flags

This is a confirmed bug. The root cause is in `pkg/plugins/dco/dco.go` at line 300: the `filterTrustedUsers()` function — which is where `trusted_apps` is actually checked — is only called when `SkipDCOCheckForMembers` or `SkipDCOCheckForCollaborators` is `true`. If neither skip flag is set (the common case when users only want `trusted_apps`), the function is never invoked and the `trusted_apps` config is silently ignored.

The fix is a one-line change to the guard condition: adding `|| len(config.TrustedApps) > 0` so that `filterTrustedUsers` also runs when trusted apps are configured. PR #681 implements exactly this fix with a unit test and has been approved. It's currently blocked by what appears to be an unrelated integration test flake.

/remove-lifecycle stale
/area plugins
/good-first-issue
```

### Rationale

**What's being added**:
- Root cause: the exact line and condition that causes the bug (not in original issue)
- Explanation of why `trusted_apps` is silently ignored (the guard condition)
- Link to existing fix PR #681 and its status

**Why these labels**:
- `/area plugins`: DCO is a Prow plugin
- `/good-first-issue`: Level 1 effort — one-line fix, clear root cause, existing approved PR
- `/remove-lifecycle stale`: Issue is legitimate and actively being fixed
- No `/kind bug`: Already has `kind/bug` label

**What's NOT included**:
- No detailed code snippets — the PR already shows the fix
- No priority label — fix PR already exists and is approved
- No `/help-wanted` — already has an approved PR, just needs the integration test resolved

## Next Steps

- Post augmentation comment to issue
- Retest PR #681's integration job (or investigate if it's a real failure)
- Merge PR #681

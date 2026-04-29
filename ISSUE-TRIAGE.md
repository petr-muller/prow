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

## Next Steps

- Assess effort level
- Augment the issue with findings and link to PR #681
- Consider retesting PR #681's integration job

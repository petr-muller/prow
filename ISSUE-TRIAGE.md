# Triage for Issue #190

**Status**: In Progress
**Created**: 2026-04-14

## Issue Information

- **Issue Number**: #190
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/190

## Initial Validation

**Assessment**: LEGITIMATE

### Analysis

The issue reports that when a presubmit job was renamed (`pull-etcd-unit-test` to `pull-etcd-unit-test-amd64`), the status reconciler triggered the newly-named job on many PRs — including **draft pull requests**. Draft PRs should not have had jobs triggered on them.

**Issue Category**: Bug

**Repository Scope Check**:
- Component mentioned: status-reconciler
- Exists in this repo: Yes (`pkg/statusreconciler/`, `cmd/status-reconciler/`)
- Relevant code paths: `pkg/statusreconciler/controller.go`, `pkg/statusreconciler/status.go`

**Information Completeness**:
- Sufficient detail provided: Yes
- The issue includes: a clear description, the specific rename that triggered the problem, an example draft PR that was incorrectly triggered, and a link to the job history showing the spurious runs
- Missing information: None significant — the report is well-written

### Recommendation

This is a valid bug report. The status reconciler is a component in this repository, and the described behavior (triggering jobs on draft PRs during reconciliation after a job rename) is clearly unintended. Draft PRs are explicitly excluded from presubmit triggering in normal Prow operation, so the status reconciler should respect the same exclusion.

**Suggested Action**:
- Keep open and continue triage

## Code Research

### Current Implementation

**Primary Components**:
- Status Reconciler Controller: `pkg/statusreconciler/controller.go` — detects config changes (added/removed/migrated presubmits) and triggers jobs accordingly
- Status Reconciler Status: `pkg/statusreconciler/status.go` — monitors config file changes and emits deltas
- Trigger Plugin: `pkg/plugins/trigger/pull-request.go` — handles PR events and has extensive draft PR filtering

**Architecture Overview**:
The status reconciler watches for Prow config changes. When it detects that a blocking presubmit was added (or renamed, which looks like add+remove), it fetches all open PRs for the affected repo and triggers the new job on each one. This ensures PRs don't get stuck waiting for a status context that was never created for them.

**Key Code Paths**:
1. `controller.go:185-209` — `reconcile()`: orchestrates three operations: trigger new presubmits, retire removed contexts, migrate renamed contexts
2. `controller.go:211-268` — `triggerNewPresubmits()`: the buggy method. Iterates over all open PRs and triggers newly-added blocking presubmits on each
3. `controller.go:236-243` — the PR loop: only checks `Mergable` before triggering, **never checks `Draft`**
4. `controller.go:270-289` — `triggerIfTrusted()`: checks trust before triggering, but does not check draft status
5. `controller.go:357-403` — `addedBlockingPresubmits()`: determines which presubmits are new/changed. A job rename appears as a new addition here

**Data Flow**:
1. Config change detected → `reconcile()` called with old/new config delta
2. `addedBlockingPresubmits()` identifies new blocking presubmits
3. `triggerNewPresubmits()` fetches all open PRs via `GetPullRequests(org, repo)`
4. For each PR: skip if unmergeable, filter presubmits by branch/changed-files, check trust, then trigger
5. **Missing step**: No draft PR check anywhere in this flow

### Related Code

**Draft PR Handling in Trigger Plugin** (`pkg/plugins/trigger/pull-request.go`):
- `buildAllButDrafts()` (line 376-383): Explicitly skips all jobs for draft PRs
- PR open handler (line 86-92): Checks `pr.PullRequest.Draft` and skips + comments
- `ConvertedToDraft` action (line 160-164): Aborts running jobs when PR is converted to draft
- The trigger plugin treats draft status as a first-class concern throughout

**PullRequest Type** (`pkg/github/types.go:259`):
- `Draft bool` field is available on the `PullRequest` struct
- The `GetPullRequests()` API returns this field, so it's available to the status reconciler

### Test Coverage

**Existing Tests** (`pkg/statusreconciler/controller_test.go`):
- Comprehensive tests for `addedBlockingPresubmits`, `removedPresubmits`, `migratedBlockingPresubmits` detection logic
- `TestControllerReconcile` tests the full reconcile flow with various scenarios: trusted/untrusted PRs, unmergeable PRs, branch filtering, `run_if_changed` matching, error handling, denylist filtering
- Coverage assessment: Good for existing features

**Test Gaps**:
- No test for draft PRs — none of the test PR fixtures set `Draft: true`
- No test verifying that draft PRs are skipped during triggering

### Root Cause Analysis

**Primary Cause**:
The `triggerNewPresubmits()` method in `pkg/statusreconciler/controller.go:236` iterates over all open PRs returned by `GetPullRequests()` but only filters out unmergeable PRs (line 237). It does not check the `Draft` field. This means draft PRs are treated identically to non-draft PRs when the status reconciler triggers jobs after a config change.

**Contributing Factors**:
1. The status reconciler was likely written before GitHub's draft PR feature existed, or before Prow added draft support
2. The trigger plugin and status reconciler are separate components with no shared "should we trigger on this PR" logic — the draft check in the trigger plugin does not protect the status reconciler path
3. `GetPullRequests()` returns all open PRs without filtering by draft status

**Reproduction Conditions**:
- A blocking presubmit job is added or renamed in the Prow config
- Draft PRs exist in the affected repository
- The status reconciler runs and processes the config delta

### Proposed Solutions

#### Approach 1: Add Draft Check in triggerNewPresubmits

**Description**: Add a `pr.Draft` check in the PR iteration loop of `triggerNewPresubmits()`, right after the existing `Mergable` check. Skip draft PRs with a log message.

**Pros**:
- Minimal change — one `if` statement added
- Follows the same pattern as the existing `Mergable` check
- Directly addresses the root cause
- Easy to test — add a draft PR fixture to the existing test structure

**Cons**:
- Only fixes this one code path; doesn't prevent future similar omissions in other components

**Affected Components**:
- `pkg/statusreconciler/controller.go`: add draft check in `triggerNewPresubmits()`
- `pkg/statusreconciler/controller_test.go`: add test case with a draft PR

**Complexity**: Low

**Backwards Compatibility**: No impact — this only prevents unwanted job triggering

#### Approach 2: Shared "Should Trigger" Predicate

**Description**: Extract a shared predicate function (used by both the trigger plugin and status reconciler) that encapsulates all the conditions under which a PR should not have jobs triggered (draft, unmergeable, etc.).

**Pros**:
- Prevents future divergence between trigger plugin and status reconciler
- Single source of truth for "should we trigger on this PR"

**Cons**:
- More complex change
- Requires refactoring two independent components
- Overkill for this specific bug

**Complexity**: Medium

**Backwards Compatibility**: No impact

#### Recommendation

**Preferred Approach**: Approach 1 (Add Draft Check)

This is the right level of fix for the problem. The status reconciler already has its own filtering logic (unmergeable, untrusted, branch filtering, denylist), and adding a draft check is consistent with that pattern. Approach 2 is a nice-to-have but would be an over-engineered response to a single missing check.

**Key Implementation Considerations**:
1. The check should go right after the `Mergable` check (line 237-243) for consistency
2. Log a message at Info level when skipping a draft PR, similar to the trigger plugin's behavior
3. Add a test case with `Draft: true` on the PR fixture, verifying no jobs are triggered

**Testing Requirements**:
- Add a test case: "draft PR means no trigger, retire and migrate still happen"
- Use the existing test infrastructure (fakeProwJobTriggerer, fakeGitHubClient, etc.)

## Next Steps

- Proceed with `assess-effort` subcommand

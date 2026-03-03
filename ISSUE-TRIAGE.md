# Triage for Issue 400

**Status**: In Progress
**Created**: 2026-03-03

## Issue Information

- **Issue Number**: 400
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/400
- **Title**: `tide` merge queue stalls when unresolved comments exist
- **Author**: aevyrie
- **Labels**: area/tide, kind/bug, lifecycle/stale

## Issue Summary

When a PR is in the merge queue, has unresolved comments in GitHub, and the repo branch protection settings require all comments to be resolved before merge, it stalls the `tide` merge queue because the PR cannot merge. To most users, the stalled PR looks inexplicable. Expected behavior: PRs that cannot merge due to unmet requirements should be ignored.

## Initial Validation

**Assessment**: LEGITIMATE

### Analysis

The issue reports that when a PR enters Tide's merge queue but has unresolved GitHub review comments (with branch protection requiring comment resolution), Tide repeatedly attempts to merge the PR and fails. This stalls the entire merge queue, blocking other PRs from merging. The behavior is invisible to most users, making it appear as though the merge queue is inexplicably stuck.

**Issue Category**: Bug

**Repository Scope Check**:
- Component mentioned: Tide (`pkg/tide/`)
- Exists in this repo: Yes
- Relevant code paths: `pkg/tide/tide.go`, `pkg/tide/github.go`, `pkg/tide/status.go`

**Information Completeness**:
- Sufficient detail provided: Yes
- Clear observed vs expected behavior described
- Missing information: No specific PR link demonstrating the issue, but the scenario is clearly described and reproducible

**Related Issues**:
- Issue #269: "PR with 'Change requested' leads to Tide repeatedly attempting MERGE" — same root cause pattern. Tide doesn't check GitHub branch protection requirements before attempting merge, leading to repeated failed attempts and queue stalls. A maintainer (petr-muller) confirmed the likely relation.

### Recommendation

This is a legitimate bug in Tide's merge logic. Tide should pre-check GitHub branch protection requirements (unresolved comments, required reviews) before attempting to merge a PR. When a PR can't be merged due to branch protection settings, Tide should skip it rather than stalling the queue.

The issue is part of a broader pattern (shared with #269) where Tide doesn't account for all GitHub branch protection rules, leading to repeated merge failures.

**Suggested Action**:
- Keep open and continue triage
- Consider as related to (possibly duplicate root cause with) issue #269

## Code Research

### Current Implementation

**Primary Components**:
- Tide controller: `pkg/tide/tide.go` — main sync loop, PR categorization, merge decisions
- GitHub provider: `pkg/tide/github.go` — GitHub API interactions, merge execution, mergeability checking
- Status controller: `pkg/tide/status.go` — PR status evaluation, requirement diff calculation

**Architecture Overview**:
Tide runs a periodic sync loop that queries GitHub for PRs matching configured TideQuery criteria. PRs are grouped into subpools by (org, repo, branch), filtered for mergeability, categorized by test status (successes/pendings/missings), and then acted upon (merge, trigger tests, or wait). Merges are attempted sequentially, with retry logic for transient errors.

**Key Code Paths**:
1. Sync loop entry: `pkg/tide/tide.go:531-623` — fetches PRs, divides into subpools
2. PR filtering: `pkg/tide/tide.go:755-793` — checks merge conflicts, status contexts
3. PR categorization: `pkg/tide/tide.go:1075-1135` — buckets PRs by test status
4. Merge decision: `pkg/tide/tide.go:1483-1526` — decides action (Merge/Trigger/Wait)
5. Merge execution: `pkg/tide/github.go:243-319` — sequential merge with error handling
6. Merge retry: `pkg/tide/tide.go:1365-1423` — up to 3 retries with typed error handling
7. Mergeability check: `pkg/tide/github.go:605-636` — checks conflicts and merge method

**Data Flow**:
1. Sync loop queries GitHub GraphQL for PRs matching TideQuery
2. PRs filtered via `filterPR()` — checks merge conflicts and status contexts
3. Remaining PRs categorized by test results in `accumulate()`
4. `takeAction()` picks highest-priority successful PR (or batch) for merge
5. `mergePRs()` sets tide context to SUCCESS, then calls GitHub Merge API
6. On failure, `tryMerge()` handles typed errors with retry/skip/abort logic
7. On next sync iteration, failed PR remains in pool and may be re-attempted

### Root Cause Analysis

**Primary Cause**:
Tide does NOT pre-check GitHub branch protection requirements related to resolved conversations before attempting a merge. The `isAllowedToMerge()` function (github.go:605-636) only checks:
- Merge conflicts (`Mergeable == Conflicting`)
- Valid merge method labels
- Rebase capability
- Repo merge method settings

It does NOT check:
- Whether conversations are resolved (GitHub's "require conversations to be resolved" setting)
- Whether reviews have approved the PR (this is checked separately for status display via `ReviewApprovedRequired` in status.go:257-262, but NOT in the merge filtering path)

When a PR has unresolved conversations and branch protection requires them resolved, GitHub's Merge API returns a 405 error, which Tide receives as `UnmergablePRError`.

**The Stalling Mechanism — Two Scenarios**:

1. **Single PR merge**: When `takeAction()` picks a single successful PR to merge (tide.go:1499-1503), it calls `mergePRs()` with just that PR. The merge fails with `UnmergablePRError`. On the next sync cycle, the same PR is still in the success pool (all tests pass), so Tide picks it again and fails again. This creates an infinite retry loop. Other PRs in the success pool are blocked because `pickHighestPriorityPR()` picks the same PR each time.

2. **Batch merge**: When merging a batch, `UnmergablePRError` sets `keepTrying=true` (tide.go:1415-1416), so Tide continues to the next PR. The batch itself doesn't fully stall, but the failing PR stays in the pool and can be re-selected in future batches. If the PR happens to be the only PR with successful tests, then the single-PR merge path stalls as described above.

**Contributing Factors**:
1. The GraphQL query (tide.go:1914-1949) does not fetch conversation/thread resolution state
2. No equivalent of `ReviewApprovedRequired` exists for conversation resolution
3. `filterPR()` only checks status contexts and merge conflicts — not branch protection rules
4. Tide's error message for `UnmergablePRError` hints at the issue but doesn't help users: "PR is unmergable. Do the Tide merge requirements match the GitHub settings for the repo?"
5. The PR remains in the pool because from Tide's perspective all required contexts are passing

**Relationship to Issue #269**:
Issue #269 reports the same root cause pattern with "Changes Requested" review state. While `ReviewApprovedRequired` exists as a TideQuery option and can filter PRs via `review:approved` in the GitHub search query (config/tide.go:597-599), the "conversations resolved" requirement has no equivalent mechanism. Both issues stem from Tide not pre-validating all GitHub branch protection rules before merge attempts.

### Proposed Solutions

#### Approach 1: Pre-merge Branch Protection Validation

**Description**: Before attempting to merge, query GitHub's branch protection rules for the target branch and validate that the PR meets all requirements (resolved conversations, approved reviews, etc.). Skip the PR if requirements are not met.

**Pros**:
- Addresses root cause directly
- Generic solution — handles any current or future branch protection rules
- PR can be given a meaningful status message explaining why it can't merge

**Cons**:
- Additional API calls per merge attempt (branch protection endpoint + PR conversations)
- GitHub's GraphQL doesn't expose "resolved conversations" directly — may need REST API
- Branch protection rules can be complex and vary across repos
- Rate limit impact

**Affected Components**:
- `pkg/tide/github.go`: Add branch protection check before merge
- `pkg/tide/status.go`: Add status description for unresolved conversations
- `pkg/tide/tide.go`: Possibly add to PR GraphQL query

**Complexity**: Medium-High

**Backwards Compatibility**: No impact — adds validation that currently doesn't exist

#### Approach 2: Use GitHub's Mergeable State More Effectively

**Description**: GitHub's `MergeableState` (different from `Mergeable`) can indicate whether branch protection requirements are met. The PR GraphQL field `mergeStateStatus` provides values like `BLOCKED`, `BEHIND`, `CLEAN`, etc. If Tide fetched and used this field, it could filter out PRs blocked by branch protection without needing to know the specific rules.

**Pros**:
- Single field check covers all branch protection rules at once
- Minimal API overhead — just add field to existing GraphQL query
- Future-proof — any new branch protection rule GitHub adds is automatically handled
- Simple implementation

**Cons**:
- `mergeStateStatus` may have caveats (e.g., how it handles Tide's own required context)
- Less specific status messages (can't tell user exactly which requirement is unmet)
- Relies on GitHub correctly computing this state (may have latency)

**Affected Components**:
- `pkg/tide/tide.go`: Add `mergeStateStatus` to PullRequest GraphQL query
- `pkg/tide/github.go`: Check state in `isAllowedToMerge()`
- `pkg/tide/codereview.go`: Add field to CodeReviewCommon if needed

**Complexity**: Low-Medium

**Backwards Compatibility**: No impact

#### Approach 3: Exponential Backoff on Repeated Merge Failures

**Description**: Track merge failure history per PR. If a PR repeatedly fails to merge with `UnmergablePRError`, exponentially delay re-attempts or temporarily exclude it from the merge pool.

**Pros**:
- Doesn't require understanding specific branch protection rules
- Reduces API waste from repeated failed merge attempts
- Works as a general safety net for any unmergeable condition

**Cons**:
- Doesn't address root cause — PR still appears in pool with confusing status
- Delayed rather than prevented — user still doesn't know why PR can't merge
- Adds state tracking complexity
- May mask legitimate transient failures that would resolve on retry

**Affected Components**:
- `pkg/tide/tide.go`: Add failure tracking and backoff logic
- `pkg/tide/github.go`: Record merge failures

**Complexity**: Medium

**Backwards Compatibility**: No impact

#### Recommendation

**Preferred Approach**: Approach 2 (Use GitHub's Mergeable State More Effectively)

This is the simplest and most robust solution. By fetching GitHub's `mergeStateStatus` field (which aggregates all branch protection checks into a single state), Tide can filter out PRs that GitHub itself knows cannot be merged. This handles unresolved conversations, required reviews, and any future branch protection rules without Tide needing to understand each one individually.

Approach 1 could supplement Approach 2 to provide more specific user-facing status messages, but the core fix should be Approach 2 for its simplicity and completeness.

Approach 3 is useful as a defense-in-depth measure but should not be the primary fix since it doesn't address root cause.

**Key Implementation Considerations**:
1. Check whether `mergeStateStatus` accounts for Tide's own context status (since Tide sets its own context to SUCCESS just before merge)
2. Determine which `mergeStateStatus` values should prevent merge (BLOCKED definitely; BEHIND maybe depending on Tide config)
3. Add meaningful status description for PRs blocked by branch protection
4. Consider config option `reviewApprovedRequired` pattern — may want a `branchProtectionMergeStateRequired` option

**Testing Requirements**:
- Test that PRs with `mergeStateStatus=BLOCKED` are filtered from merge pool
- Test that PRs with `mergeStateStatus=CLEAN` proceed normally
- Test batch behavior when some PRs are blocked
- Test status message reflects branch protection blockage

### Test Coverage

**Existing Tests**:
- `pkg/tide/tide_test.go`: Extensive coverage of merge error handling (lines 1899-1942), unmergeable PR filtering (lines 2291-2333), batch waiting (lines 1572-1624)
- `pkg/tide/status_test.go`: Status evaluation including review approval check, merge conflicts (lines 43-871)
- `pkg/tide/github_test.go`: Merge method and context checking

**Test Gaps**:
- No tests for "conversations resolved" requirement (not implemented at all)
- No tests for `mergeStateStatus` field (not fetched currently)
- No tests for repeated merge failure of the same PR across sync cycles (the stalling scenario)
- No tests verifying that a single unmergeable PR doesn't block other successful PRs in serial merge path

## Next Steps

(Action items will be added here)

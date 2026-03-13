# Triage for Issue #651

**Status**: In Progress
**Created**: 2026-03-13

## Issue Information

- **Issue Number**: #651
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/651
- **Title**: `tide`: batch triggered containing an already-merged PR
- **Author**: Prucek
- **Labels**: area/tide, kind/bug

## Issue Summary

Tide triggered a TRIGGER_BATCH that included a PR it had just merged in the previous sync cycle. Observed on Azure/ARO-HCP repo via tide-history:
- 9:50 — PR merged manually #4297
- 9:53 — Tide fires a TRIGGER_BATCH that includes PR #4297

Expected behavior: a merged PR should not be included in a batch.

## Initial Validation

**Assessment**: LEGITIMATE

### Analysis

The issue describes a concrete bug in Tide's batch merging logic where a PR that was already merged (either manually or by Tide in a previous cycle) is incorrectly included in a subsequent TRIGGER_BATCH action.

**Issue Category**: Bug

**Repository Scope Check**:
- Component mentioned: Tide
- Exists in this repo: Yes (`pkg/tide/`)
- Relevant code paths: `pkg/tide/tide.go`, `pkg/tide/github.go` (PR pool filtering and batch logic)

**Information Completeness**:
- Sufficient detail provided: Yes
- Clear timeline with specific PR number and tide-history link
- Screenshot of tide-history showing the sequence of events
- The report includes a specific, reproducible scenario

### Discussion Notes

- An AI-generated comment on the issue suggests the root cause is GraphQL API staleness (3-minute gap not enough for GitHub to reflect the merge)
- Maintainer petr-muller pushed back on this theory, noting that if this were the case, we would routinely see TRIGGER for just-merged PRs, which we don't
- Root cause likely lies in how Tide filters its PR pool or how it handles race conditions between merge completion and the next sync cycle

### Recommendation

Keep open and continue triage. This is a valid bug report for the Tide component with clear reproduction evidence. The root cause needs investigation - the GraphQL staleness theory has been questioned, so deeper code analysis is warranted.

**Suggested Action**:
- Keep open and continue triage

## Code Research

### Current Implementation

**Primary Components**:
- `pkg/tide/tide.go` - Main sync loop, subpool management, batch picking, action decisions
- `pkg/tide/github.go` - GitHub API interaction, PR querying, merge execution
- `pkg/tide/codereview.go` - `CodeReviewCommon` struct, PR abstraction layer
- `pkg/config/tide.go` - Query construction including `state:open` filter

**Architecture Overview**:

Tide operates on a periodic sync loop (configurable, typically ~30s). Each cycle:
1. Queries GitHub GraphQL search API for open PRs matching configured queries
2. Divides PRs into subpools by org/repo/branch
3. Filters PRs (merge conflicts, failing checks, etc.)
4. For each subpool, accumulates test results and decides on an action (merge, trigger batch, trigger serial, wait)
5. Executes the action

**Key Code Paths**:

1. **Query construction** (`pkg/config/tide.go:574`): Builds the search query with `"is:pr", "state:open", "archived:false"` as the base filters
2. **Query execution** (`pkg/tide/github.go:102-151`): `Query()` runs GraphQL search queries in parallel, collecting PRs into a map
3. **GraphQL search** (`pkg/tide/github.go:165-211`): `search()` paginates through GitHub search results
4. **GraphQL struct** (`pkg/tide/tide.go:1914-1949`): `PullRequest` struct defines the GraphQL fragment - does **NOT** include a `Merged` field
5. **Subpool division** (`pkg/tide/tide.go:1866-1909`): `dividePool()` groups PRs by org/repo/branch, fetches current base SHA
6. **PR filtering** (`pkg/tide/tide.go:755-793`): `filterPR()` checks merge conflicts and status contexts - does **NOT** check merge status
7. **Action decision** (`pkg/tide/tide.go:1483-1526`): `takeAction()` decides whether to merge, trigger batch, trigger serial, or wait
8. **Batch triggering** (`pkg/tide/tide.go:1510-1517`): Triggers batch when `len(sp.prs) > 1 && len(batchPending) == 0`
9. **Batch picking** (`pkg/tide/tide.go:1198-1248`): `pickBatch()` selects candidates sorted by PR number

**Data Flow**:
1. `Sync()` calls `provider.Query()` which runs GitHub GraphQL search with `state:open`
2. Results are collected as `PullRequest` structs and converted to `CodeReviewCommon`
3. `dividePool()` groups PRs by org/repo/branch, fetches base SHA via `GetRef()`
4. `filterSubpools()` runs `filterPR()` on each PR (checks mergeable state + contexts)
5. `syncSubpool()` calls `accumulate()` and `accumulateBatch()` to check test results
6. `takeAction()` decides on TRIGGER_BATCH if multiple PRs exist and no batch is pending
7. `pickBatch()` selects PRs for the batch
8. `trigger()` creates ProwJob objects

### Root Cause Analysis

**Primary Cause: GitHub Search Index Staleness + Missing Defensive Filter**

The root cause is a combination of two factors:

1. **GitHub Search API eventual consistency**: The query uses `state:open` in the GraphQL search API. This API is backed by an eventually-consistent search index that can lag behind the actual database state. When a PR is merged, the search index may not immediately reflect the state change, causing the PR to still appear in `state:open` search results.

2. **No post-query merge status validation**: The `PullRequest` GraphQL struct (`tide.go:1914-1949`) does **not** include a `Merged` boolean field. Even though GitHub's GraphQL API would return real-time field data for each PR node (including `merged: true`), Tide doesn't request this field and therefore cannot filter out merged PRs post-query.

The result: when GitHub's search index is stale, a merged PR passes through the entire pipeline without any check catching it.

**Why it's not observed routinely**: GitHub search index staleness is uncommon and typically resolves in seconds. The 3-minute lag in this case is an outlier, which is why we don't routinely see TRIGGER for just-merged PRs. However, the code has no defense against this edge case.

**Addressing the objection**: The AI comment on the issue suggested GraphQL API staleness, and petr-muller correctly noted we don't see this routinely. Both points are valid: the search index is *usually* fast, but *occasionally* lags. The bug manifests precisely because there's no defensive check for when it does lag. Tide logs from the incident would help confirm whether the PR was indeed returned by the search query.

**Contributing Factors**:
1. `filterPR()` (`tide.go:755-793`) checks merge conflicts and status contexts but not merge status
2. `isAllowedToMerge()` (`github.go:605-636`) checks `Mergeable` state but not `Merged` state
3. `pickBatch()` (`tide.go:1198-1248`) checks retest eligibility but not merge status
4. No component in the pipeline validates that PRs are actually still open

**Reproduction Conditions**:
- A PR must be merged externally (by a human or another automation) shortly before a Tide sync cycle
- GitHub's search index must be stale enough that `state:open` still matches the merged PR
- The PR must have passing or pending status contexts to survive `filterPR()`

### Proposed Solutions

#### Approach 1: Add `Merged` Field to GraphQL Struct and Post-Query Filter

**Description**: Add `Merged githubql.Boolean` to the `PullRequest` GraphQL struct. Then filter out merged PRs either in `Query()` after fetching results, or in `filterPR()`.

**Pros**:
- Minimal code change (add one field, add one filter check)
- No additional API calls - the `merged` field is returned as part of the existing GraphQL query
- GitHub returns real-time field data even when the search index is stale
- Handles edge cases automatically
- Very low risk of side effects

**Cons**:
- Slightly more data returned per PR (one boolean)
- Doesn't fix the root cause (search index lag) but defends against it

**Affected Components**:
- `pkg/tide/tide.go`: Add `Merged` field to `PullRequest` struct
- `pkg/tide/github.go` or `pkg/tide/tide.go`: Add filtering logic (either in `Query()` or `filterPR()`)

**Complexity**: Low

**Backwards Compatibility**: None - this is an additive change to an internal struct

#### Approach 2: Check PR State via REST API Before Batch Trigger

**Description**: Before triggering a batch, verify each candidate PR's merge status via the GitHub REST API (`GET /repos/:owner/:repo/pulls/:number`).

**Pros**:
- Uses the authoritative REST API which is always consistent
- Would catch any stale data issue

**Cons**:
- Additional API calls (one per PR in the batch) - rate limit impact
- Slower batch triggering
- More complex implementation
- Overkill for a rare edge case

**Complexity**: Medium

**Backwards Compatibility**: None

#### Recommendation

**Preferred Approach**: Approach 1 (Add `Merged` field + post-query filter)

This is the simplest and most effective solution. Adding `Merged` to the GraphQL fragment costs nothing (no extra API calls) and provides a definitive check. The filter should be added early in the pipeline (either in `Query()` when building the PR map, or in `filterPR()`) to prevent merged PRs from entering the pool at all.

**Key Implementation Considerations**:
1. Add `Merged githubql.Boolean` to `PullRequest` struct at `tide.go:1914`
2. Best place for the filter is in `Query()` (`github.go:141-143`) where PRs are added to the map - skip PRs where `Merged == true`
3. Add a log warning when a merged PR is filtered to help detect future search index lag
4. Consider adding a metric for filtered-merged-PRs to track frequency

**Testing Requirements**:
- Unit test: Add a test case to `TestTakeActionV2` or create a new test verifying that merged PRs are excluded from the pool
- Unit test: Verify that `filterPR()` (or `Query()`) excludes PRs with `Merged == true`
- Existing tests should continue to pass since the field defaults to `false` for open PRs

### Test Coverage

**Existing Tests**:
- `TestAccumulateBatch` (`tide_test.go:89`): Tests batch accumulation logic
- `TestPickBatchV2` (`tide_test.go:1124`): Tests batch PR selection
- `TestTakeActionV2` (`tide_test.go:1526`): Tests action decision tree
- `TestPresubmitsForBatch` (`tide_test.go:3835`): Tests presubmit selection for batches
- `TestIsBatchCandidateEligible` (`tide_test.go:5223`): Tests batch candidate eligibility

**Test Gaps**:
- No test for merged PRs leaking into the pool via stale search results
- No test verifying that merged PRs are filtered out post-query
- No test for the race condition between PR merge and next sync cycle

### Documentation Review

**Code Comments**:
- `tide.go:1911-1913`: "PullRequest holds graphql data about a PR, including its commits and their contexts. This struct is GitHub specific"
- `github.go:101`: "Query gets all open PRs based on tide configuration."

**Known Limitations**:
- Tide relies entirely on GitHub's search `state:open` filter for merge status exclusion
- No defensive post-query validation exists

## Next Steps

1. **Confirm root cause with Tide logs**: The reporter/maintainer should examine Tide logs from the incident to verify the merged PR was returned by the search query
2. **Implement Approach 1**: Add `Merged` field to `PullRequest` struct and filter in `Query()`
3. **Add unit tests**: Test that merged PRs are excluded from the pool
4. **Monitor**: Add a log line when merged PRs are filtered to track frequency of search index lag

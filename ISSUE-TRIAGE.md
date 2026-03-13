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

## Effort Assessment

**Effort Level**: 1 - Easy (good-first-issue)

### Summary

Adding a `Merged` boolean field to the GraphQL PullRequest struct and a single post-query filter check. Well-defined problem, minimal scope, clear solution, existing test patterns to follow.

### Factor Analysis

#### Scope of Changes
- **Assessment**: Small
- **Details**: 2 files modified (`pkg/tide/tide.go`, `pkg/tide/github.go`), ~10-20 lines of production code + test cases
- **Level Indication**: 1-2

#### Complexity
- **Assessment**: Simple
- **Details**: Add one field to a struct, add one conditional filter. No concurrency, no algorithmic challenges, no complex interactions.
- **Level Indication**: 1-2

#### Required Expertise
- **Assessment**: Minimal
- **Details**: Basic Go, basic understanding of GraphQL structs. The pattern for adding fields and filters is clear from existing code.
- **Level Indication**: 1-2

#### Clarity and Certainty
- **Assessment**: Well-defined
- **Details**: Root cause is clear (missing `Merged` field), solution approach is straightforward (add field + filter). No ambiguity in desired behavior.
- **Level Indication**: 1-2

#### Testing Requirements
- **Assessment**: Simple
- **Details**: Add test case with a PR where `Merged == true` and verify it's excluded from the pool. Existing test patterns in `tide_test.go` can be followed.
- **Level Indication**: 1-2

#### Backwards Compatibility
- **Assessment**: Fully compatible
- **Details**: Adding a field to an internal struct and filtering out PRs that shouldn't be there. No behavior change for correct scenarios. No configuration changes.
- **Level Indication**: 1-2

#### Architectural Alignment
- **Assessment**: Perfect fit
- **Details**: Follows existing pattern of requesting GraphQL fields and filtering on them. No new patterns needed.
- **Level Indication**: 1-2

#### External Dependencies
- **Assessment**: Well-supported
- **Details**: GitHub GraphQL API supports the `merged` field on PullRequest objects. Standard, stable API.
- **Level Indication**: 1-3

### Recommended Labels

- [x] `good-first-issue`: Clear, well-defined, small scope, existing patterns to follow
- [x] `area/tide`: Core Tide functionality
- [x] `kind/bug`: Fixing a defensive gap

### Guidance for Contributors

- Good starting point for new Prow contributors
- Suggested prerequisite knowledge: Basic Go, understanding of Go struct tags for GraphQL
- Key files to review:
  - `pkg/tide/tide.go:1914-1949` - PullRequest struct (add `Merged` field here)
  - `pkg/tide/github.go:102-151` - Query() function (add filter here)
  - `pkg/tide/tide_test.go` - existing test patterns
- The fix is: add `Merged githubql.Boolean` to PullRequest struct, then filter out PRs where `Merged == true` in `Query()` with a log warning

### Caveats and Considerations

- While the fix is simple, the root cause (GitHub search index staleness) is not something Prow can control. This fix is a defensive measure.
- The root cause theory should ideally be confirmed with info-level Tide logs showing the merged PR in the pool (look for "Subpool synced" entries with `"action":"TRIGGER_BATCH"` containing the PR number). The log provided only contained debug-level query execution entries, which don't show PR numbers or actions.
- An important assumption: that GitHub's GraphQL API returns real-time field data (including `merged: true`) for PR nodes found via a stale search index. This needs verification but is consistent with how GraphQL resolvers work.

## Proposed Issue Augmentation

### Title Change

- **No change needed**: Current title "`tide`: batch triggered containing an already-merged PR" is clear, specific, and mentions the component.

### Proposed GitHub Comment

```
Analysis of the tide-history data confirms this is a systematic issue, not a one-off. Out of 418,918 PR merges across the entire history, 4,538 (1.1%) resulted in the merged PR appearing in a subsequent TRIGGER or TRIGGER_BATCH action. This affects 827 out of 8,964 pools. 94.5% of occurrences happen 2-5 minutes after the merge (exactly one sync cycle later), and the pattern includes PRs that Tide itself merged (MERGE/MERGE_BATCH followed by TRIGGER_BATCH containing the same PR).

The root cause is GitHub's eventually-consistent search index. Tide queries with `state:open` (`pkg/config/tide.go:574`), but after a merge, the search index doesn't update immediately. The `PullRequest` GraphQL struct (`pkg/tide/tide.go:1914-1949`) does not include a `merged` field, so there is no post-query defense. The merged PR passes through `filterPR()` (checks merge conflicts and contexts, not merge status) and `pickBatch()` (checks retest eligibility, not merge status) into the triggered batch, wasting a CI job.

The proposed fix is to add `Merged githubql.Boolean` to the `PullRequest` GraphQL struct and filter out merged PRs in `Query()` (`pkg/tide/github.go:141-143`), logging a warning when this happens. An open question is whether GitHub resolves node fields from the real-time database or from the stale search index: if the former, the `merged` field will be accurate and the filter will work; if the latter, further investigation is needed.

/good-first-issue
```

### Rationale

**What's being added**:
- Quantified scope: 4,538 occurrences, 1.1% of merges, 827 pools, based on tide-history analysis
- Root cause explanation: search index staleness + missing `merged` field in the GraphQL struct
- Specific code paths where the merged PR slips through undetected
- Concrete fix approach with file locations
- Open question about GraphQL node resolution

**Why these labels**:
- `area/tide` and `kind/bug`: Already applied by the reporter
- `/good-first-issue`: Level 1 effort - add one field to a struct, add one filter check, follow existing patterns

**What's NOT included**:
- `/retitle`: Current title is already clear and specific
- Priority label: Systematic but low-impact per occurrence (wasted CI job, not incorrect merge)
- Logging issue: should be filed separately (87% of log output is one debug message, making Tide unobservable)

## Log Analysis Findings

### Logging Makes Tide Unobservable

Analyzed live Tide pod `tide-bb56955b9-h8ckw` on app.ci (ci namespace). Pod uptime: 63 minutes, but only ~5 minutes of logs are retrievable due to container log buffer rotation.

**Log volume**: ~37,600 lines in ~5 minutes (~7,500 lines/minute)

**Breakdown by level**:
- debug: 35,939 (99.7%)
- info: 452 (0.3%)

**Top debug messages**:
| Message | Count | % of total |
|---------|-------|------------|
| `Presubmit excluded by ps.ShouldRun` | 30,306 | **87%** |
| `Blocking merges to branch via issue.` | 986 | 3% |
| `Sending query` / `Finished query` | 945 | 3% |

**Consequence**: A single debug message (`tide.go:1714`, fires for every presubmit x every PR x every branch combination) generates 87% of all log output. This causes:
1. Container log buffer rotates every ~5 minutes
2. Incident logs are lost within minutes of occurring
3. Cannot retrieve logs from the PR 4390 incident (~40 minutes before log retrieval)
4. Info-level entries that would diagnose the issue (`"Subpool synced."` with action and targets) are evicted almost immediately

**Root cause investigation impact**: We cannot confirm or deny the search index staleness theory because the diagnostic info-level logs from the incident timeframe have been rotated out. The original log file provided (315 lines) appears to have been a partial capture that only contained query-phase debug entries.

**Logging improvement suggestions**:
1. Reduce or aggregate `"Presubmit excluded by ps.ShouldRun"` - alone would extend retention from ~5 min to ~40+ min
2. Add pool membership change tracking at info level (PR joins/leaves pool)
3. Aggregate per-query debug logs into a single info-level summary
4. Aggregate per-PR filtering into per-subpool summaries

### Tide History Analysis

Analyzed full tide-history data (518,719 records across 8,964 pools, spanning 2019-03-20 to 2026-03-13).

**Key findings**:
- 4,538 cases of merged PRs appearing in subsequent triggers (1.1% of 418,918 total merges)
- 827 out of 8,964 pools affected (9.2%)
- 94.5% of cases (4,290) have a 2-5 minute delta - exactly one sync cycle
- 4,534 out of 4,538 cases have a different base SHA (expected: merge changes the base)
- Pattern includes Tide's own MERGE/MERGE_BATCH actions followed by TRIGGER_BATCH

**Top affected repos**: kubevirt-ui/kubevirt-plugin:main (86), Azure/ARO-HCP:main (84), openshift-kni/lifecycle-agent (multiple branches, 60-77 each)

**Downstream impact**: After a trigger-with-merged-PR, next action is MERGE 70% (Tide moves on), TRIGGER 27% (serial fallback), TRIGGER_BATCH 2.5% (another batch). Each occurrence wastes a CI job.

### Temporal Analysis: Behavior Started November 2025

Analyzed incident rate over the full 7-year history to determine when this behavior started or changed.

**Key finding: The behavior exists since 2019 but underwent a dramatic regime change around November 18-20, 2025.**

#### Incident rate by year (incidents / merges):

| Year | Merges | Incidents | Rate | Pools Affected |
|------|--------|-----------|------|----------------|
| 2019 | 11,759 | 46 | 0.39% | 35 |
| 2020 | 19,935 | 144 | 0.72% | 79 |
| 2021 | 42,140 | 37 | 0.09% | 30 |
| 2022 | 45,053 | 49 | 0.11% | 46 |
| 2023 | 57,796 | 100 | 0.17% | 82 |
| 2024 | 63,035 | 27 | 0.04% | 25 |
| 2025 (pre-Nov) | ~90,000 | 79 | ~0.09% | ~60 |
| 2025-11 onward | ~80,000 | 3,758 | ~4.7% | 546 |

#### Daily onset around the transition:

| Date | Merges | Incidents | Rate |
|------|--------|-----------|------|
| Nov 17 | 474 | 0 | 0% |
| Nov 18 | 485 | 6 | 1.2% |
| Nov 19 | 443 | 0 | 0% |
| Nov 20 | 527 | 12 | 2.3% |
| Nov 21 | 394 | 17 | 4.3% |
| Nov 24 | 467 | 91 | 19.5% |
| Nov 25 | 499 | 66 | 13.2% |

First incidents appear Nov 18 01:53 UTC (isolated), then cluster at Nov 18 18:44 UTC, then gap on Nov 19, then persistent from Nov 20 onward.

#### Evidence against a Prow code change:
- Tide sync cycle frequency unchanged: ~165s median, stable before and after
- Active pool count unchanged: ~1,300 pools
- No relevant Tide code changes deployed around Nov 18-20
- The spike is system-wide: 482 pools that **never** had incidents before suddenly started having them

#### Evidence pointing to GitHub search backend change:
- The merge-to-trigger gap distribution shifted dramatically:
  - Before: variable (4% <1m, 57% 2-3m, long tail to hours/days)
  - After: uniformly 88% at exactly 2-3 minutes, 0% below 2 minutes
- This is consistent with the search index update latency increasing from "variable, often <2 min" to "consistently >2.7 min" (just above one sync cycle)
- GitHub was migrating search backend to "advanced search" around Sep-Nov 2025 (ISSUE_ADVANCED GraphQL type removed Nov 4, 2025)
- GitHub had a Git operations failure Nov 18 20:30-21:34 UTC (expired TLS cert) - close to but not exactly matching the onset
- No specific GitHub changelog entry found documenting a search consistency change

#### Interpretation:

The issue reported in issue 651 was always possible (existed since 2019 at 0.04-0.7% rate) but became dramatically more prevalent (~50x increase) around November 18-20, 2025. The most likely cause is a change in GitHub's search index update pipeline that increased the eventual consistency window from typically <2 minutes to consistently >3 minutes. This makes the proposed fix (adding `Merged` field + filter) even more important, as the problem is now systemic rather than rare.

### Open Questions

1. **Does GitHub resolve GraphQL node fields from the real-time DB or the stale search index?** If the former, adding `Merged` field and filtering will work. If the latter, we need a different approach (REST API verification or pool membership tracking).
2. **Is this the only cause of the excessive trigger pattern?** The tide-history shows multiple batch sizes per base SHA in some repos. The merged-PR-in-pool issue explains some of this, but there may be other contributing factors.
3. **What changed in GitHub's search backend around November 18-20, 2025?** The evidence strongly suggests a change in search index update latency, possibly related to the advanced search migration. No public changelog entry documents this.

## Briefing Completed

Briefed maintainer on: 2026-03-13

Key questions asked:
- "Shouldn't `state:open` be enough?" - Yes in theory, but GitHub search index is eventually consistent. Tide-history analysis confirmed this happens at 1.1% of all merges.
- "If search is stale, would the `merged` field also be stale?" - Depends on GitHub's implementation. Standard GraphQL resolves node fields from the DB, but this needs verification.
- "Can you inspect tide-history for more repos?" - Led to the quantified analysis above.
- "How to improve Tide logging?" - Led to log analysis showing 87% of output is one debug message.

Maintainer decisions:
- Proceed with augmentation comment including tide-history evidence
- File logging issue separately
- Good-first-issue label appropriate

## Next Steps

1. **Post augmentation comment** with tide-history evidence and apply good-first-issue label
2. **File separate issue**: Tide logging volume (87% from one debug message, 5-min log retention)
3. **Implement fix**: Add `Merged` field to `PullRequest` struct and filter in `Query()` with warning log
4. **Verify GraphQL behavior**: First PR should include a test or manual verification that `merged` field is accurate when search index is stale

# Triage for Issue #650

**Status**: In Progress
**Created**: 2026-04-12

## Issue Information

- **Issue Number**: #650
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/650
- **Title**: `tide`: obsolete batch ProwJobs not aborted when a new batch supersedes them
- **Author**: @Prucek
- **Created**: 2026-03-11
- **State**: Open
- **Labels**: kind/feature, area/tide

## Initial Validation

**Assessment**: LEGITIMATE

### Analysis

The issue describes a missing cleanup behavior in Tide's batch processing: when the base branch SHA advances (due to a merge), Tide starts a new batch without aborting ProwJobs from the previous, now-obsolete batch. The old ProwJobs continue running to completion even though their results are no longer relevant, wasting CI resources.

**Issue Category**: Feature Request (reclassified from bug by maintainer @petr-muller)

**Repository Scope Check**:
- Component mentioned: Tide (batch merging subsystem)
- Exists in this repo: Yes (`pkg/tide/`)
- Relevant code paths: `pkg/tide/tide.go` (dividePool, batch triggering logic)

**Information Completeness**:
- Sufficient detail provided: Yes
- Clear reproduction steps: Start a batch, manually merge a PR in the same pool, observe new batch starts without cancelling old one
- Real-world example provided: Azure/ARO-HCP tide history with screenshot
- Expected behavior clearly stated: superseded batch ProwJobs should be aborted

### Recommendation

This is a legitimate feature request for Tide. The issue is well-written with clear reproduction steps, a concrete example, and a specific expected behavior. The maintainer has already confirmed it's valid but reclassified it from bug to feature.

**Suggested Action**:
- Keep open and continue triage

## Code Research

### Current Implementation

**Primary Components**:
- `Sync()`: `pkg/tide/tide.go:531-623` — Main sync loop, runs periodically
- `dividePool()`: `pkg/tide/tide.go:1864-1909` — Partitions PRs into subpools, queries ProwJobs by current baseSHA
- `syncSubpool()`: `pkg/tide/tide.go:1721-1790` — Syncs a single org/repo/branch subpool
- `takeAction()`: `pkg/tide/tide.go:1483-1526` — Decides what action to take (merge, trigger batch, etc.)
- `accumulateBatch()`: `pkg/tide/tide.go:937-1038` — Finds existing batch ProwJobs, returns pending/success lists
- `trigger()`: `pkg/tide/tide.go:1425-1466` — Creates ProwJob objects for batch testing

**Architecture Overview**:
Each sync iteration: `Sync()` → `dividePool()` (partitions PRs, fetches current baseSHA, queries ProwJobs filtered by baseSHA) → `syncSubpool()` → `accumulateBatch()` (classifies batch ProwJobs) → `takeAction()` (triggers new batch if `batchPending == 0`).

**Key Code Paths**:
1. `dividePool()` at line 1900: queries ProwJobs using `cacheIndexKey(org, repo, branch, sha)` — only returns ProwJobs matching the **current** baseSHA
2. `takeAction()` at line 1510: triggers new batch when `len(batchPending) == 0` and `len(sp.prs) > 1`
3. `trigger()` at line 1459: creates ProwJob via `prowJobClient.Create()` with current baseSHA in refs
4. `cacheIndexFunc()` at line 2082-2095: indexes ProwJobs by `org/repo:branch@baseSHA`

**Data Flow**:
1. Sync loop gets current baseSHA from provider (line 1875)
2. Queries only ProwJobs with matching baseSHA (line 1900)
3. Old batch ProwJobs (stale baseSHA) are invisible — not returned by the index query
4. `accumulateBatch()` sees no pending batches → returns empty
5. `takeAction()` sees `batchPending == 0` → triggers new batch
6. Old batch ProwJobs continue running on the cluster, orphaned

### Related Code

**Existing Abort Mechanisms in Prow**:
- `pjutil.TerminateOlderJobs()`: `pkg/pjutil/abort.go:58-122` — Aborts older presubmit jobs when newer ones are created. **Explicitly excludes BatchJob** at line 64.
- `trigger.abortAllJobs()`: `pkg/plugins/trigger/pull-request.go:170-199` — Aborts all jobs on PR sync/close/draft. Applies to all job types but triggered by PR events, not by Tide.
- `plank.terminateDupes()`: `pkg/plank/reconciler.go:444-451` — Calls `TerminateOlderJobs` during `syncTriggeredJob`. Only for presubmits.
- `plank.syncAbortedJob()`: `pkg/plank/reconciler.go:783-804` — Handles cleanup of already-aborted jobs (deletes pod, marks complete).

**Similar Functionality**:
- Plank already handles presubmit superseding via `TerminateOlderJobs` — the pattern is well-established
- The abort state machine is: set `Status.State = AbortedState` → Plank's `syncAbortedJob` deletes pod and marks complete
- `AbortedState` defined at `pkg/apis/prowjobs/v1/types.go:66-67`: "prow killed the job early (new commit pushed, perhaps)"

**Duplicate Prevention**:
- `nonFailedBatchForJobAndRefsExists()` at line 1469: prevents creating duplicate batch ProwJobs for same job+baseSHA+PRs via index `nonFailedBatchByNameBaseAndPullsIndexName` (line 2097-2134)
- This prevents duplicates within the same baseSHA but does NOT prevent new batches when baseSHA changes

### Test Coverage

**Existing Tests**:
- `TestAccumulateBatch`: `pkg/tide/tide_test.go:89-308` — Tests batch accumulation for various states (pending, success, failure, missing jobs, PRs leaving pool). Does NOT test baseSHA change scenarios.
- `TestDividePool`: `pkg/tide/tide_test.go:933-1122` — Tests pool division and baseSHA matching. Does NOT test what happens when baseSHA advances with pending batches.
- `TestSerialRetestingConsidersPRThatIsCurrentlyBeingSRetested`: `pkg/tide/tide_test.go:5388` — Tests serial retest behavior when baseSHA changes, notes old runs are deleted by Plank. But this is for presubmits, not batches.
- `TestTerminateOlderJobs`: `pkg/pjutil/abort_test.go:33` — Line 123: "Don't terminate older batch jobs" — explicitly tests and confirms batch jobs are excluded from termination.

**Test Gaps**:
- No test verifies batch ProwJob behavior when baseSHA changes mid-batch
- No test for batch ProwJob orphaning/cleanup
- No integration test for the complete sequence: batch running → baseSHA advances → new batch triggered → old batch still running

### Root Cause Analysis

**Primary Cause**:
`dividePool()` filters ProwJobs by current baseSHA, making old batch ProwJobs invisible. `takeAction()` only checks `len(batchPending) == 0` against this filtered set. There is no code path that identifies or aborts batch ProwJobs from previous baseSHAs.

**Contributing Factors**:
1. `TerminateOlderJobs` explicitly excludes batch jobs (abort.go:64) — the existing presubmit cleanup pattern was intentionally not extended to batches
2. No state is persisted between sync iterations — Tide has no memory of previous batch baseSHAs
3. The cache index design means stale-baseSHA ProwJobs are simply never queried

**Reproduction Conditions**:
- Multiple PRs in a Tide pool (required for batch triggering)
- A batch is currently running (ProwJob in Pending/Triggered state)
- baseSHA advances (merge, manual or via Tide)
- Next Tide sync iteration triggers a new batch, old one continues

### Proposed Solutions

#### Approach 1: Abort Stale Batches in dividePool/syncSubpool

**Description**: After querying ProwJobs by current baseSHA, also query for batch ProwJobs with stale baseSHAs for the same org/repo/branch and abort them.

**Pros**:
- Directly addresses the issue at the source
- Cleanup happens early in the sync cycle, before `takeAction()`
- Consistent location: pool division already handles ProwJob queries

**Cons**:
- Requires an additional ProwJob query per subpool (list batch jobs for org/repo/branch, filter out current baseSHA)
- Adding mutation logic to `dividePool()` breaks its current read-only role

**Affected Components**:
- `pkg/tide/tide.go`: `dividePool()` or `syncSubpool()` — add stale batch query and abort
- Possibly needs a new cache index for batch-only ProwJobs by org/repo/branch (without baseSHA)

**Complexity**: Medium

**Backwards Compatibility**: Safe — only affects batch ProwJobs that Tide already ignores

#### Approach 2: Extend TerminateOlderJobs to Include Batch Jobs

**Description**: Remove the batch job exclusion in `pjutil.TerminateOlderJobs()` (abort.go:64) so that Plank automatically aborts stale batch jobs when new ones are triggered, just like it does for presubmits.

**Pros**:
- Minimal code change (remove one exclusion check)
- Leverages existing, well-tested abort infrastructure
- Plank already calls `terminateDupes` for every triggered job
- Consistent behavior between presubmit and batch job types

**Cons**:
- The `TerminateOlderJobs` digest comparison may not be appropriate for batch jobs (batch jobs for the same job name but different PR sets would have different digests)
- The exclusion was intentional — may have been for a reason not documented in code
- Timing dependency: only works after the new batch ProwJob is created and Plank processes it

**Affected Components**:
- `pkg/pjutil/abort.go`: Remove batch exclusion at line 64
- `pkg/pjutil/abort_test.go`: Update test at line 123 ("Don't terminate older batch jobs")

**Complexity**: Low

**Backwards Compatibility**: Potentially risky — the exclusion was deliberate. Need to understand why batch jobs were excluded.

#### Approach 3: Abort Stale Batches in takeAction Before Triggering

**Description**: Before triggering a new batch in `takeAction()`, query for all batch ProwJobs with stale baseSHAs for the same org/repo/branch and abort them.

**Pros**:
- Cleanup is directly coupled to the trigger decision
- Clear intent: "before starting a new batch, clean up the old one"
- No changes to `dividePool()` read-only semantics
- Can be made configurable via Tide config

**Cons**:
- Adds mutation logic to the action-taking phase
- Only triggers cleanup when a new batch is about to start (orphans persist until then)

**Affected Components**:
- `pkg/tide/tide.go`: `takeAction()` or a new helper called before `trigger()`
- May need a new query or cache index for batch ProwJobs by org/repo/branch

**Complexity**: Medium

**Backwards Compatibility**: Safe — configurable, only affects stale batches

#### Recommendation

**Preferred Approach**: Approach 3 (Abort stale batches in takeAction before triggering), possibly combined with elements of Approach 2 if the TerminateOlderJobs exclusion can be safely removed.

Approach 3 provides the clearest intent coupling (abort old → trigger new) without changing dividePool's read-only semantics. It can be made configurable. Approach 2 is tempting for its simplicity but the intentional exclusion needs investigation before removing it.

**Key Implementation Considerations**:
1. Need a way to query batch ProwJobs by org/repo/branch without baseSHA filter — either a new index or a broader list query
2. The abort should set `Status.State = AbortedState` following the established pattern — Plank will handle pod cleanup
3. Consider whether to abort only when triggering a new batch, or also during every sync (to handle cases where no new batch is needed)
4. Should be configurable — some deployments may want stale batches to complete

**Testing Requirements**:
- Unit test: baseSHA changes with pending batch → old batch aborted, new batch triggered
- Unit test: multiple stale batches from sequential baseSHA advances → all aborted
- Unit test: configuration to disable abort behavior
- Integration-style test: full sync cycle with baseSHA advance

## Effort Assessment

**Effort Level**: 2 - Moderate (help-needed)

### Summary

Well-defined problem with a clear solution approach and established abort patterns to follow. The scope is moderate (2-4 files, ~100-200 LOC) but requires understanding Tide's batch lifecycle and ProwJob indexing. Not a good-first-issue due to the Tide-specific knowledge required, but well within reach for a contributor familiar with Prow or willing to invest time understanding the batch flow.

### Factor Analysis

#### Scope of Changes
- **Assessment**: Moderate
- **Details**: Primary changes in `pkg/tide/tide.go` (new abort helper, modification to `takeAction` or `syncSubpool`). Possibly a new cache index function. Tests in `pkg/tide/tide_test.go`. Optionally `pkg/pjutil/abort.go` if extending `TerminateOlderJobs`. Estimated 2-4 files, 100-200 LOC.
- **Level Indication**: 2-3

#### Complexity
- **Assessment**: Moderate
- **Details**: The abort pattern is well-established (`Status.State = AbortedState`). The main complexity is querying batch ProwJobs by org/repo/branch without baseSHA, which may need a new cache index. No concurrency concerns — sync loop is single-threaded per subpool.
- **Level Indication**: 2

#### Required Expertise
- **Assessment**: Moderate
- **Details**: Requires understanding of Tide's sync loop, `dividePool`, `takeAction`, and how ProwJob cache indexes work. Can be learned from existing code. Familiarity with controller-runtime client patterns helpful.
- **Level Indication**: 2-3

#### Clarity and Certainty
- **Assessment**: Well-defined
- **Details**: Problem is clearly described with reproduction steps. Solution approach is clear: find stale batch ProwJobs and set them to AbortedState. The main open question is whether to do this in Tide or extend Plank's `TerminateOlderJobs`.
- **Level Indication**: 1-2

#### Testing Requirements
- **Assessment**: Moderate
- **Details**: Follow existing patterns in `TestAccumulateBatch` and `TestDividePool`. Need new test cases for baseSHA-change scenarios. Existing test infrastructure is sufficient — no new test framework needed.
- **Level Indication**: 2

#### Backwards Compatibility
- **Assessment**: Minor impact
- **Details**: Changes behavior for stale batch ProwJobs that currently run to completion. This is the desired behavior change. Could be made configurable if needed, but aborting obsolete jobs is broadly desirable. No config schema changes strictly required.
- **Level Indication**: 1-2

#### Architectural Alignment
- **Assessment**: Good fit
- **Details**: Follows the established abort pattern used by Plank and trigger plugin. ProwJob state machine already supports AbortedState for exactly this scenario ("prow killed the job early, new commit pushed perhaps"). Natural extension of existing behavior.
- **Level Indication**: 1-2

#### External Dependencies
- **Assessment**: None
- **Details**: Entirely internal to Prow. Uses existing Kubernetes API patterns for listing and updating ProwJobs.
- **Level Indication**: 1-3

### Recommended Labels

- [x] `help-wanted`: Moderate scope, clear solution, suitable for skilled contributors
- [x] `area/tide`: Core Tide functionality
- [x] `kind/feature`: New cleanup behavior (per maintainer reclassification)
- [ ] `good-first-issue`: Requires moderate Tide-specific knowledge

### Guidance for Contributors

**For Level 2 (Moderate)**:
- Suitable for contributors familiar with Go and Kubernetes controller patterns
- Should review:
  - `pkg/tide/tide.go`: `dividePool()`, `takeAction()`, `trigger()`, `accumulateBatch()` flow
  - `pkg/pjutil/abort.go`: `TerminateOlderJobs()` — the established abort pattern
  - `pkg/plank/reconciler.go`: `syncAbortedJob()` — how aborted jobs are cleaned up
  - `pkg/tide/tide_test.go`: `TestAccumulateBatch`, `TestDividePool` — existing test patterns
- Recommended approach: Add a helper in `pkg/tide/tide.go` that queries batch ProwJobs for org/repo/branch with stale baseSHA and sets them to AbortedState. Call it from `syncSubpool` or `takeAction` before triggering a new batch.
- Key consideration: May need a new cache index for batch ProwJobs by org/repo/branch (without baseSHA in the key)

### Caveats and Considerations

- The `TerminateOlderJobs` batch exclusion at `abort.go:64` was intentional. Before removing it (Approach 2), investigate whether batch job digests would cause incorrect matching. Approach 3 (Tide-side abort) is safer.
- Consider whether to abort stale batches on every sync or only when triggering a new batch. Aborting on every sync is more aggressive but prevents orphaned jobs from consuming resources even when no new batch is triggered.
- Some deployments may intentionally allow stale batches to complete (e.g., if batch jobs produce artifacts beyond merge gating). A config option could address this, but may not be needed for an initial implementation.

## Proposed Issue Augmentation

### Title Change

- **No change needed**: Current title "`tide`: obsolete batch ProwJobs not aborted when a new batch supersedes them" is clear, specific, mentions the component, and accurately describes the feature request.

### Proposed GitHub Comment

```
The root cause is in Tide's `dividePool()` function, which queries ProwJobs using a cache index keyed by `org/repo:branch@baseSHA`. When baseSHA advances, old batch ProwJobs no longer match the index and become invisible to subsequent sync iterations. `takeAction()` then sees `batchPending == 0` and triggers a fresh batch, while the old one continues running to completion. In effect, the old batch ProwJobs are orphaned — still consuming CI resources but with results that Tide will never use.

Prow already has a well-established pattern for aborting superseded jobs: `pjutil.TerminateOlderJobs()` in `pkg/pjutil/abort.go` does exactly this for presubmit jobs, and Plank's `syncAbortedJob()` handles the cleanup (pod deletion, marking complete). However, `TerminateOlderJobs` explicitly excludes batch jobs (line 64). The fix would involve either removing that exclusion (if batch job digest comparison works correctly for this case) or adding batch-specific abort logic in Tide itself — for example, querying for batch ProwJobs with stale baseSHAs before triggering a new batch and setting them to `AbortedState`.

/help-wanted
```

### Rationale

**What's being added**:
- Root cause explanation: the baseSHA-keyed cache index in `dividePool()` is the mechanism that makes old batches invisible. The original issue describes the symptom accurately but doesn't explain the underlying code path.
- Implementation guidance: pointers to the existing abort pattern (`TerminateOlderJobs`, `syncAbortedJob`) and the specific exclusion that prevents it from working for batch jobs. This gives potential contributors a clear starting point.

**Why these labels**:
- `area/tide`: Already applied. Correct — the fix is in `pkg/tide/`.
- `kind/feature`: Already applied. Correct — per maintainer reclassification.
- `/help-wanted`: Level 2 effort assessment. Well-defined problem with established patterns to follow, but requires moderate Tide-specific knowledge.

**What's NOT included**:
- No `/retitle`: Title is already specific and well-structured.
- No `/good-first-issue`: Requires understanding Tide's batch lifecycle, ProwJob indexing, and abort patterns — not suitable for first-time contributors.
- No `/priority`: This is a resource optimization feature, not a correctness bug. Old batches running to completion is wasteful but not harmful.
- No detailed solution approach: The comment provides enough for a contributor to get started. Detailed architectural recommendations belong in PR review, not the issue.

## Briefing Completed

Briefed maintainer on: 2026-04-12

Key questions asked:
- None — maintainer acknowledged all slides without questions

Maintainer decision:
Proceed with wrapup (post comment, apply labels).

## Wrapup

**Comment posted**: No (maintainer declined)
**Branches pushed**: 2026-04-14
- `claude-maintenance-helpers`: synced with origin
- `issue-triage-650`: pushed to origin with tracking

**Status**: Complete

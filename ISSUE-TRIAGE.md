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

## Next Steps

- Assess effort: Determine complexity and effort level
- Augment: Improve issue with technical findings

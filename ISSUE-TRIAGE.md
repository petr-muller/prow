# Triage for Issue #474

**Status**: In Progress
**Created**: 2026-03-07

## Issue Information

- **Issue Number**: #474
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/474

## Initial Validation

**Assessment**: LEGITIMATE

### Analysis

The issue reports that Tide gets stuck when two PRs in the merge pool have semantic conflicts (incompatible changes). When batched together, tests fail, but Tide keeps re-batching them instead of falling back to merging one individually and retesting the other.

**Issue Category**: Bug

**Repository Scope Check**:
- Component mentioned: Tide (merge/batch logic)
- Exists in this repo: Yes
- Relevant code paths: `pkg/tide/` (batch merging, subpool management)

**Information Completeness**:
- Sufficient detail provided: Yes (supplemented by maintainer discussion)
- Supporting evidence: Tide history link for openshift/dpu-operator confirms the pattern
- Maintainer confirmation: BenTheElder confirmed Tide is "supposed to fall back to one individual PR" when the batch fails. petr-muller confirmed this fallback was not observed.

### Recommendation

Keep open and continue triage. This is a confirmed bug in Tide's batch fallback logic. The expected behavior (fall back to individual PRs after batch failure) is not working correctly in certain semantic conflict scenarios.

**Suggested Action**:
- Keep open and continue triage

## Code Research

### Current Implementation

**Primary Components**:
- `syncController.takeAction`: `pkg/tide/tide.go:1483-1526` - Decision engine that picks which action to take (Merge, MergeBatch, Trigger, TriggerBatch, Wait)
- `syncController.accumulateBatch`: `pkg/tide/tide.go:946-1038` - Collects batch ProwJob results, returns only successful or pending batches
- `syncController.accumulate`: `pkg/tide/tide.go:1077-1135` - Classifies individual PRs as successes/pendings/missings based on ProwJob states
- `pickNewBatch`: `pkg/tide/tide.go:1148-1191` - Creates new batch by attempting git merges of candidates
- `pickBatchWithPreexistingTests`: `pkg/tide/tide.go:2221-2283` - Reuses successful/pending batch ProwJobs
- `syncController.pickBatch`: `pkg/tide/tide.go:1198-1248` - Orchestrates batch picking (preexisting first, then new)
- `syncController.isPassingTests`: `pkg/tide/tide.go:847-858` - Checks GitHub commit status contexts for a PR

**Architecture Overview**:
Tide's sync loop (`Sync` → `syncSubpool` → `takeAction`) runs periodically. For each org/repo/branch subpool, it:
1. Fetches current base SHA and lists ProwJobs indexed by that SHA
2. Classifies PRs by individual test state (`accumulate`)
3. Classifies batch ProwJobs (`accumulateBatch`)
4. Decides action via priority cascade in `takeAction`

**Key Code Paths - `takeAction` Priority Cascade**:
1. `MergeBatch` (line 1493): If a batch has all tests passing → merge all batch PRs
2. `Merge` (line 1499): If individual PRs pass AND no batch pending → merge one PR
3. `Wait` (line 1506): If no presubmits configured
4. `TriggerBatch` (line 1510): If >1 PR AND no batch pending → create and trigger batch
5. `Trigger` (line 1520): If PRs missing tests AND no pendings AND no successes → trigger one
6. `Wait` (line 1525): Default fallback

**Data Flow - Subpool ProwJob Indexing**:
ProwJobs are indexed by `org/repo:branch@baseSHA` (`cacheIndexFunc` at line 2082-2095). Only ProwJobs matching the current base SHA are included in the subpool. When the base SHA changes (e.g., after a merge to the branch), ALL previous ProwJob results become invisible to the subpool.

### Root Cause Analysis

**Primary Cause**: The bug has multiple contributing factors that interact:

**Factor 1 - No batch failure memory**: When a batch of PRs fails testing, the failure is not recorded anywhere that would prevent re-batching the same PRs. `pickBatchWithPreexistingTests` correctly ignores failed batches (line 2245), but `pickNewBatch` is called as fallback and will recreate the same batch if the same PRs are candidates.

**Factor 2 - Semantic vs. git conflicts**: `pickNewBatch` only detects git merge conflicts (line 1176: `r.MergeWithStrategy`). If two PRs merge cleanly at the git level but produce test failures when combined (semantic conflicts), `pickNewBatch` will keep including both PRs in the batch.

**Factor 3 - baseSHA invalidation cycle**: This is likely the key factor in the stuck scenario:
1. PRs A and B pass individual tests at baseSHA X
2. Tide triggers batch [A, B] - batch is pending
3. While batch runs, baseSHA might change (another merge or push to branch)
4. Batch fails (semantic conflict)
5. Next sync: new baseSHA Y → all old ProwJobs (including individual passes) are stale
6. Both PRs now in `missings` (no ProwJobs at new baseSHA)
7. `takeAction` reaches line 1520: triggers individual test for one PR
8. That PR eventually passes, but `takeAction` line 1510 triggers a new batch (since >1 PR and no batch pending)
9. Batch fails again → cycle repeats

Even without baseSHA change, the same batch gets repeatedly triggered because there's no mechanism to say "these PRs are incompatible for batching."

**Factor 4 - `isPassingTests` vs `accumulate` disagreement**: The merge path at line 1499 uses `pickHighestPriorityPR` which calls `isPassingTests` (checks GitHub commit status contexts). This is a different data source than `accumulate` (checks ProwJob objects). A PR in `successes` might fail `isPassingTests` if its commit status contexts don't all show success, causing the individual merge fallback to be skipped even when ProwJob-based accumulation says the PR passes.

**Reproduction Conditions**:
- Two or more PRs in the same merge pool (same org/repo/branch)
- PRs merge cleanly at git level (no textual conflicts)
- PRs are semantically incompatible (combined changes cause test failures)
- Both PRs individually pass their presubmit tests
- The batch of these PRs always fails

### Test Coverage

**Existing Tests**:
- `TestPickBatchV2` (`tide_test.go:1124-1291`): Tests batch picking with git merge conflicts. Verifies conflicting PRs are excluded from batch.
- `TestAccumulateBatch` (`tide_test.go:89-308`): Tests batch ProwJob accumulation for pending, successful, and failed states.
- `TestPickBatchPrefersBatchesWithPreexistingJobs` (`tide_test.go:4631-4823`): Tests reusing pre-existing batch jobs. Includes test that failed batch is not reused.

**Test Gaps**:
- No test for repeated batch failure with semantic conflicts (git-clean but test-failing batches)
- No test for the stuck cycle where batch fails, individual tests invalidated, batch re-triggered
- No test for interaction between `accumulate`/`isPassingTests` disagreement and batch fallback

### Proposed Solutions

#### Approach 1: Batch Failure Tracking with Cooldown

**Description**: Track which sets of PRs have been batched together and failed. When `pickBatch` creates a batch, check if that exact combination (or a superset) has recently failed. If so, skip batching and allow individual PR merging to proceed.

**Pros**:
- Directly addresses the root cause (no memory of batch failures)
- Prevents wasted CI resources on known-failing batches
- Allows individual PRs to merge, breaking the deadlock

**Cons**:
- Requires new state tracking (batch failure cache)
- Need to determine appropriate cooldown/expiry for failure records
- Combinatorial complexity: tracking all possible PR subsets could be expensive

**Affected Components**:
- `syncController`: Add batch failure cache
- `pickBatch`: Check failure cache before creating new batch
- `takeAction`: Consider batch failure state in action priority

**Complexity**: Medium

**Backwards Compatibility**: No impact - purely additive behavior

#### Approach 2: Reduced Batch Size on Failure

**Description**: When a batch fails, reduce the batch size for subsequent attempts. If a 2-PR batch fails, fall back to individual PR processing. This is simpler than tracking exact PR combinations.

**Pros**:
- Simpler implementation than full failure tracking
- Natural fallback: batch of N fails → try batch of N-1 → ... → try individual PRs
- Doesn't require tracking PR combinations

**Cons**:
- May be slower to converge (tries intermediate batch sizes)
- Doesn't identify which specific PRs are incompatible
- Needs state to track "last batch size that failed"

**Affected Components**:
- `pickBatch`: Accept maximum batch size parameter influenced by failure history
- `syncSubpool`: Track batch failure count to reduce batch size

**Complexity**: Low-Medium

**Backwards Compatibility**: No impact

#### Approach 3: Prioritize Individual Merge Over Rebatch

**Description**: After a batch failure, force Tide to try individual PR merging before re-batching. Currently, `takeAction` will attempt individual merge at line 1499 IF PRs are in `successes`, but may skip to re-batching if they aren't. Add logic to ensure individual PRs get tested and merged before re-batching after a failure.

**Pros**:
- Minimal code change: adjust `takeAction` priority after batch failure
- Aligns with the expected behavior described by maintainers
- No new state tracking needed beyond "last batch failed" flag

**Cons**:
- May slow down overall merge throughput in normal cases
- Doesn't prevent the eventual re-creation of the same failing batch
- Only defers the problem; if individual PRs never get merged, batch will be retried

**Affected Components**:
- `takeAction`: Add condition to prefer individual testing after batch failure
- `accumulateBatch`: Return batch failure information (not just success/pending)

**Complexity**: Low

**Backwards Compatibility**: No impact

#### Recommendation

**Preferred Approach**: Approach 1 (Batch Failure Tracking) or a hybrid of Approaches 1 and 3.

The cleanest fix would combine:
1. Have `accumulateBatch` also return information about failed batches (Approach 3)
2. Track recently-failed batch PR combinations in a cache with TTL (Approach 1)
3. In `takeAction`, when a batch recently failed, skip `TriggerBatch` and let the `Merge` or `Trigger` paths handle individual PRs

**Key Implementation Considerations**:
1. Failed batch tracking should be keyed by the set of PR numbers (order-independent)
2. Cache entries should expire after a configurable duration or when any involved PR's HEAD changes
3. When individual PRs merge, the cache should be updated (reduced batch becomes untested)
4. Logging should clearly indicate when batch is skipped due to failure history

**Testing Requirements**:
- Test: batch of 2 PRs fails → Tide falls back to individual PR merge
- Test: after individual PR merges, remaining PRs can be re-batched
- Test: batch failure cache expires correctly
- Test: HEAD change on a PR clears the failure cache for that combination

## Effort Assessment

**Effort Level**: 3 - Large (requires expertise)

### Summary

Fixing Tide's batch fallback behavior requires modifying core merge decision logic (`takeAction`), adding new state tracking for batch failures, and careful testing of concurrent scenarios. The fix touches Tide's most critical code path and requires deep understanding of its sync loop, ProwJob lifecycle, and the interaction between multiple data sources (ProwJob states vs GitHub commit statuses).

### Factor Analysis

#### Scope of Changes
- **Assessment**: Moderate
- **Details**: Primary changes in `pkg/tide/tide.go` (takeAction, accumulateBatch, pickBatch) and `pkg/tide/tide_test.go`. Estimated 3-5 files, 200-400 LOC. New batch failure cache structure needed.
- **Level Indication**: 2-3

#### Complexity
- **Assessment**: High
- **Details**: The bug involves multiple interacting factors (baseSHA invalidation, two different test-passing checks, batch ProwJob lifecycle). The fix must correctly handle state across sync loop iterations without introducing new race conditions or deadlocks. Edge cases include: partial batch overlap with previous failures, HEAD changes invalidating cache, interaction with `PrioritizeExistingBatches` config.
- **Level Indication**: 3-4

#### Required Expertise
- **Assessment**: Deep
- **Details**: Requires understanding of Tide's complete sync loop architecture, ProwJob indexing by baseSHA, the difference between `accumulate` and `isPassingTests`, batch creation and reuse logic, and Go concurrency patterns. Contributor must understand why the current fallback doesn't work to avoid introducing a fix that breaks under different conditions.
- **Level Indication**: 3-4

#### Clarity and Certainty
- **Assessment**: Some uncertainty
- **Details**: The problem is confirmed by maintainers but the exact stuck scenario isn't fully characterized with logs/reproduction. The root cause involves multiple interacting factors, and it's unclear which combination of factors dominates in practice. Multiple solution approaches exist with different trade-offs, and the "right" one hasn't been agreed upon.
- **Level Indication**: 2-3

#### Testing Requirements
- **Assessment**: Complex
- **Details**: Need to test multi-cycle sync scenarios (batch fails → individual fallback → merge → re-batch). Existing test patterns in `tide_test.go` can be followed but new test scenarios are needed for the stuck cycle. Testing timing-dependent interactions (baseSHA changes during batch execution) is inherently complex.
- **Level Indication**: 3-4

#### Backwards Compatibility
- **Assessment**: Fully compatible
- **Details**: The fix would make Tide's behavior match its documented/expected behavior (fall back to individual merges after batch failure). No configuration changes needed. Purely additive internal behavior.
- **Level Indication**: 1-2

#### Architectural Alignment
- **Assessment**: Good fit
- **Details**: The fix extends existing patterns (action priority cascade, ProwJob accumulation). Adding batch failure tracking is a natural extension of the existing `accumulateBatch` function. No new architectural patterns required.
- **Level Indication**: 2-3

#### External Dependencies
- **Assessment**: None
- **Details**: Purely internal Tide logic. No GitHub API or Kubernetes API changes needed.
- **Level Indication**: 1-3

### Recommended Labels

- [x] `area/tide`: Core Tide batch/merge functionality
- [x] `kind/bug`: Confirmed bug - expected fallback behavior doesn't work
- [ ] `good-first-issue`: Too complex, requires deep Tide expertise
- [ ] `help-wanted`: Requires significant familiarity with Tide internals

### Guidance for Contributors

**For Level 3 (Large)**:
- Requires experience with Prow architecture, specifically Tide's sync loop
- Should review:
  - `pkg/tide/tide.go`: `takeAction`, `accumulateBatch`, `pickBatch`, `accumulate`
  - `pkg/tide/tide_test.go`: `TestAccumulateBatch`, `TestPickBatchV2`, `TestPickBatchPrefersBatchesWithPreexistingJobs`
  - The full `syncSubpool` flow to understand how data flows between functions
- Key architectural considerations:
  - State tracking across sync loop iterations (batch failure cache)
  - Interaction between `accumulate` (ProwJob-based) and `isPassingTests` (GitHub status-based)
  - ProwJob indexing by baseSHA and its implications for test result validity
  - Correct cache invalidation when PR HEADs change or PRs leave the pool
- Consult with Tide maintainers before starting implementation

### Caveats and Considerations

The exact reproduction scenario hasn't been fully characterized with detailed logs. Before implementing a fix, it would be valuable to:
1. Add debug logging to `takeAction` to capture the exact state when Tide gets stuck
2. Reproduce the issue in a test environment with two semantically-conflicting PRs
3. Confirm whether the baseSHA invalidation cycle (Factor 3) or the `isPassingTests` disagreement (Factor 4) is the primary contributor

## Next Steps

(Action items will be added here)

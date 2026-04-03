# Triage for Issue #673

**Status**: In Progress
**Created**: 2026-04-03

## Issue Information

- **Issue Number**: #673
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/673

## Initial Validation

**Assessment**: LEGITIMATE

### Analysis

The issue reports that Tide gets stuck in a retry loop when a PR matches Tide's label query (has `approved` + `lgtm`) but cannot be merged because GitHub branch protection with `enforce_admins: true` requires a minimum number of approving reviews that the PR doesn't have. This causes Tide to repeatedly pick the same PR, fail, and never advance to other mergeable PRs in the same repo.

**Issue Category**: Bug

**Repository Scope Check**:
- Component mentioned: Tide (`pkg/tide`)
- Exists in this repo: Yes
- Relevant code paths:
  - `pkg/tide/tide.go` — `accumulate()` (line 1077), `pickHighestPriorityPR()`, `tryMerge()` (line 1365)
  - `pkg/tide/github.go` — `mergePRs()` handling of `UnmergablePRError`
  - `pkg/github/client.go` — `UnmergablePRError` definition

**Information Completeness**:
- Sufficient detail provided: Yes
- The issue includes:
  - Clear description of the failure mode
  - Reproduction steps
  - Root cause analysis with specific code references (verified to exist)
  - A proposed fix approach
  - Context linking to related issue #134 (open, `kind/bug`, `area/tide`)
  - A comment from another user confirming the same loop occurs with "changes requested" review verdicts

**Relationship to #134**: Issue #134 reports that Tide doesn't honor GitHub's `required_approving_review_count` branch protection. Issue #673 describes the flip-side: when `enforce_admins: true` forces Tide to respect that protection, the merge failure causes a queue-blocking retry loop. They share a root cause (Tide's lack of awareness of GitHub review requirements) but have distinct symptoms.

### Recommendation

This is a well-documented, legitimate bug in Tide's merge queue logic. The issue is actionable, has reproduction steps, and the reporter has done significant code analysis. A second user has confirmed a related failure mode.

**Suggested Action**:
- Keep open and continue triage

## Code Research

### Current Implementation

**Primary Components**:
- **Tide sync loop** (`pkg/tide/tide.go`): Periodically queries PRs, classifies them, and attempts merges
- **GitHub merge provider** (`pkg/tide/github.go`): Executes actual merge API calls against GitHub
- **GitHub client** (`pkg/github/client.go`): Low-level GitHub API client, defines merge error types

**Architecture Overview**:
Tide runs a periodic sync loop that: (1) queries PRs matching configured label queries, (2) classifies them by ProwJob status via `accumulate()`, (3) picks the highest-priority PR to merge via `pickHighestPriorityPR()`, (4) attempts the merge via `mergePRs()` → `tryMerge()`. Each sync cycle is stateless — no information about merge failures carries over between cycles.

**Key Code Paths**:
1. `syncSubpool()`: `pkg/tide/tide.go:1721` — entry point for per-pool sync
2. `accumulate()`: `pkg/tide/tide.go:1077` — classifies PRs into successes/pendings/missings based solely on ProwJob status
3. `takeAction()`: `pkg/tide/tide.go:1483` — decides merge/trigger/wait action; calls `pickHighestPriorityPR()` and `mergePRs()`
4. `pickHighestPriorityPR()`: `pkg/tide/tide.go:912` — selects PR by priority tiers then lowest PR number
5. `tryMerge()`: `pkg/tide/tide.go:1365` — attempts merge with retries; classifies errors as fatal (keepTrying=false) or non-fatal (keepTrying=true)
6. `mergePRs()`: `pkg/tide/github.go:243` — loops through PRs, calls `tryMerge()`, logs `UnmergablePRError` at Debug level only

**Data Flow (the bug)**:
1. Sync cycle starts → `accumulate()` classifies PR as "success" (tests pass)
2. `pickHighestPriorityPR()` selects this PR (lowest number, passing tests)
3. `mergePRs()` → `tryMerge()` → GitHub API returns HTTP 405 → `UnmergablePRError`
4. `tryMerge()` returns `keepTrying=true` (line 1415-1416) — this is the non-fatal path
5. Error logged at Debug level (line 291 in github.go) — easily missed in production
6. `syncSubpool()` logs the error (line 1742-1743) but takes no corrective action
7. Next sync cycle: PR is still in pool, tests still pass → go to step 1
8. **Infinite loop**: same PR selected every cycle, blocking all other PRs in the repo

### Related Code

**Error type hierarchy in `tryMerge()`** (`pkg/tide/tide.go:1365-1423`):
- `ModifiedHeadError` → keepTrying=true (non-fatal, PR was modified)
- `UnmergablePRBaseChangedError` → keepTrying=true (retries with backoff)
- `UnauthorizedToPushError` → keepTrying=false (fatal, stops merge loop)
- `MergeCommitsForbiddenError` → keepTrying=false (fatal, stops merge loop)
- **`UnmergablePRError` → keepTrying=true** (non-fatal, but causes the retry loop bug)

**`syncController` struct** (`pkg/tide/tide.go:79-98`): Has no state for tracking merge failures across sync cycles. No cooldown mechanism, no failure counters, no exclusion lists.

**`UnmergablePRError`** (`pkg/github/client.go:4017-4020`): A catch-all for HTTP 405 responses from GitHub's merge API that don't match more specific error patterns. Triggers when branch protection blocks the merge (insufficient reviews, changes-requested review, etc.).

### Test Coverage

**Existing Tests**:
- `pkg/tide/tide_test.go:1899` — Tests that `UnmergablePRError` in a batch doesn't stop merging other PRs in the same batch
- Coverage assessment: **Partial** — only tests within-cycle behavior, NOT cross-cycle retry loop

**Test Gaps**:
- No test for the scenario where an unmergeable PR is selected again on the next sync cycle
- No test verifying that other PRs in the same repo can be merged when one is stuck
- No test for the commenter's scenario (changes-requested review + lgtm/approve labels)

### Root Cause Analysis

**Primary Cause**:
`accumulate()` (line 1077) only considers ProwJob/CI status when classifying PRs. It has no awareness of merge failures. Combined with deterministic selection in `pickHighestPriorityPR()` (lowest PR number first), a PR that is unmergeable due to GitHub branch protection will be selected on every sync cycle indefinitely.

**Contributing Factors**:
1. `tryMerge()` treats `UnmergablePRError` as non-fatal (keepTrying=true), which is correct for within-batch behavior but causes the cross-cycle loop
2. `syncController` maintains no state about merge failures between sync cycles
3. The error is logged at Debug level, making the loop hard to detect in production
4. `pickHighestPriorityPR()` is deterministic — always picks the same PR if nothing changes
5. The disconnect between Tide's label-based query (approved+lgtm) and GitHub's review-count enforcement creates PRs that Tide thinks are mergeable but GitHub rejects

**Reproduction Conditions**:
- `enforce_admins: true` in branch protection (forces Tide to respect review requirements)
- `required_approving_review_count: N` where N > 0
- A PR with approved+lgtm labels but fewer than N GitHub approving reviews
- Another PR in the same repo that is fully ready to merge (blocked by the stuck one)
- Also reproducible with "changes requested" GitHub review + approved/lgtm labels (per commenter)

### Proposed Solutions

#### Approach 1: Temporary Cooldown for Unmergeable PRs

**Description**: Add a time-based cooldown map to `syncController` that tracks PRs which failed with `UnmergablePRError`. Skip these PRs in `pickHighestPriorityPR()` or `accumulate()` for a configurable TTL (e.g., 5 minutes), allowing Tide to advance to other candidates.

**Pros**:
- Directly addresses the queue-blocking symptom
- Self-healing: cooldown expires, PR is retried (in case reviews are added)
- Minimal code change: add a map + TTL check + recording logic
- Approach suggested by the issue reporter

**Cons**:
- Adds mutable state to `syncController` (needs synchronization)
- TTL is somewhat arbitrary — too short and the loop persists, too long and legitimate retries are delayed
- Doesn't address the root disconnect between Tide queries and GitHub merge requirements
- Memory management needed (cleanup of stale entries)

**Affected Components**:
- `syncController` struct: add `mergeFailures` map
- `mergePRs()` in `github.go`: record failures
- `syncSubpool()` in `tide.go`: filter cooled-down PRs from `successes` between `accumulate()` and `takeAction()`

**Complexity**: Medium
**Backwards Compatibility**: Fully compatible — new behavior only activates on merge failures

#### Approach 2: Pre-merge Mergeability Check

**Description**: Before attempting to merge a PR, query the GitHub API for the PR's `mergeable` status (or check review state). Skip PRs that GitHub reports as unmergeable, preventing the failed merge attempt entirely.

**Pros**:
- Prevents the error entirely rather than recovering from it
- Uses GitHub's own mergeability assessment
- No need for cooldown timers or failure tracking state

**Cons**:
- Additional API call per PR per sync cycle (rate limit impact)
- GitHub's `mergeable` field can be `null` (mergeability not yet computed) — needs handling
- May not cover all cases (the `mergeable` field doesn't always reflect branch protection violations)
- Adds latency to the merge decision path

**Affected Components**:
- `pickHighestPriorityPR()` or `takeAction()`: add mergeability check
- `github.go`: add method to query PR mergeability

**Complexity**: Medium
**Backwards Compatibility**: Fully compatible

#### Approach 3: Exponential Backoff on Repeated Failures

**Description**: Instead of a binary cooldown, track failure count per PR and apply exponential backoff — a PR that fails once waits 1 cycle, fails twice waits 2 cycles, etc. After enough failures, effectively deprioritize it.

**Pros**:
- Graceful degradation: first failure has minimal impact, repeated failures get progressively deprioritized
- No arbitrary TTL to configure
- Self-healing with adaptive timing

**Cons**:
- More complex state tracking (failure count + last attempt time)
- Still doesn't address root cause
- Harder to reason about behavior

**Complexity**: Medium-High
**Backwards Compatibility**: Fully compatible

#### Recommendation

**Preferred Approach**: Approach 1 (Temporary Cooldown) — it's the simplest effective fix, directly addresses the symptom, is self-healing, and was already proposed by the reporter.

**Key Implementation Considerations**:
1. TTL should be configurable in Tide config (default ~5 minutes seems reasonable)
2. Cooldown map needs mutex protection (sync cycles are concurrent per subpool)
3. Stale entries should be pruned periodically (e.g., during each sync cycle)
4. Filter benched PRs from `successes` between `accumulate()` and `takeAction()` in `syncSubpool()` — keeps `accumulate()` pure (ProwJob status only) while excluding benched PRs from all downstream logic including batches
5. Merge errors already surface through `History.Record()` in `syncSubpool()` (line 1746-1753) — no need to change the Debug log level in `mergePRs()`

**Testing Requirements**:
- Test that a cooled-down PR is skipped by `pickHighestPriorityPR()`
- Test that Tide advances to the next PR when one is in cooldown
- Test that the cooldown expires and the PR is retried
- Test cleanup of stale cooldown entries

## Effort Assessment

**Effort Level**: 2 - Moderate (help-needed)

### Summary

The fix requires adding cross-cycle failure tracking state to `syncController` and wiring it through the merge and selection paths. The problem and solution are well-understood, but it touches core Tide merge logic, requires concurrency-safe state management, and needs careful testing.

### Factor Analysis

#### Scope of Changes
- **Assessment**: Small-Moderate
- **Details**: 2-3 files (`pkg/tide/tide.go`, `pkg/tide/github.go`, `pkg/tide/tide_test.go`), estimated ~100-200 lines including tests. Changes are localized to the Tide package.
- **Level Indication**: 2

#### Complexity
- **Assessment**: Moderate
- **Details**: Requires adding a mutex-protected map to `syncController`, recording failures in `mergePRs()`, and filtering in `pickHighestPriorityPR()` or `accumulate()`. The concurrency model is straightforward (single mutex around a map), but care is needed to avoid subtle issues like stale entries or map growth. No algorithmic challenges.
- **Level Indication**: 2

#### Required Expertise
- **Assessment**: Moderate
- **Details**: Contributor needs to understand Tide's sync loop architecture, the merge flow from `takeAction()` through `mergePRs()` → `tryMerge()`, and basic Go concurrency (sync.Mutex). The code paths are well-documented in this triage. No deep Kubernetes or distributed systems knowledge needed.
- **Level Indication**: 2

#### Clarity and Certainty
- **Assessment**: Well-defined
- **Details**: Root cause is clearly identified (no cross-cycle failure memory). Solution approach is clear (cooldown map). The reporter even provided pseudocode. The main open question is where exactly to filter (in `accumulate()` vs `pickHighestPriorityPR()` vs `takeAction()`), but this is a minor design choice.
- **Level Indication**: 1-2

#### Testing Requirements
- **Assessment**: Moderate
- **Details**: Existing test at `tide_test.go:1899` covers within-cycle behavior. New tests needed for: (1) PR skipped after `UnmergablePRError`, (2) next PR selected instead, (3) cooldown expiry allows retry, (4) stale entry cleanup. Can follow existing test patterns in `tide_test.go` — the test infrastructure supports mock merge functions with configurable errors.
- **Level Indication**: 2

#### Backwards Compatibility
- **Assessment**: Fully compatible
- **Details**: New behavior only activates after a merge failure. No configuration changes required (TTL could have a sensible default). No impact on existing deployments that don't hit this bug. Optional Tide config field for TTL tuning.
- **Level Indication**: 1

#### Architectural Alignment
- **Assessment**: Good fit with minor extension
- **Details**: Adding state to `syncController` follows the existing pattern (it already has `pools`, `changedFiles`, `History`). The cooldown map is a natural extension. No new architectural patterns introduced.
- **Level Indication**: 2

#### External Dependencies
- **Assessment**: None
- **Details**: The fix is entirely internal to Tide. No new GitHub API calls, no external system changes. Works around the existing GitHub 405 response.
- **Level Indication**: 1

### Recommended Labels

- [x] `kind/bug`: This is a bug in Tide's merge queue logic
- [x] `area/tide`: Affects the Tide component
- [x] `help-wanted`: Well-defined, moderate scope, suitable for skilled contributors

### Guidance for Contributors

- Should review the Tide sync loop flow documented in this triage, especially `syncSubpool()` → `takeAction()` → `mergePRs()` → `tryMerge()`
- Look at existing test patterns in `pkg/tide/tide_test.go`, particularly the `TestMergePRs` test around line 1899
- Filtering point: between `accumulate()` and `takeAction()` in `syncSubpool()` — filter benched PRs from `successes` list
- The `syncController` struct at `pkg/tide/tide.go:79` is the natural place for the failure tracking map

### Caveats and Considerations

- While this is Level 2, not Level 3, it does touch core merge logic — reviewers should be thorough
- The cooldown approach treats the symptom (queue blocking) but not the root cause (Tide's unawareness of GitHub review requirements). A more comprehensive fix addressing #134 would solve both issues but would be Level 3
- The commenter's scenario (changes-requested review) should be verified to also produce `UnmergablePRError` — if it produces a different error type, the fix may need to cover additional error types

## Proposed Issue Augmentation

### Title Change

- **No change needed**: Current title "Tide gets stuck retrying unmergeable PR instead of advancing to next candidate" is specific, mentions the component, and accurately describes the bug.

### Proposed GitHub Comment

```
The issue is broader than the `enforce_admins` + review count scenario described. `UnmergablePRError` is the catch-all for any HTTP 405 from GitHub's merge API that doesn't match more specific error types (`UnmergablePRBaseChangedError`, `UnauthorizedToPushError`, `MergeCommitsForbiddenError`). This means the same retry loop will occur for _any_ unmergeable condition GitHub detects: merge conflicts, unresolved conversations, changes-requested reviews (as @tuminoid confirmed), or any future 405 reasons GitHub adds. A fix should target the `UnmergablePRError` path generically, not just the review-count case.

The existing test at `pkg/tide/tide_test.go:1899` ("batch merge errors but continues if a PR is unmergeable") only verifies within-batch behavior — that other PRs in the same batch can still merge when one fails. It does not test the cross-cycle retry loop described here, which is the actual bug. A good approach would be a temporary cooldown map in `syncController` that benches PRs after `UnmergablePRError`, filtering them from the `successes` list between `accumulate()` and `takeAction()` in `syncSubpool()`. This keeps `accumulate()` pure (ProwJob status only) while letting Tide advance to the next mergeable PR. The cooldown would expire after a configurable TTL, allowing periodic retries.

/area tide
/kind bug
/help-wanted
```

### Rationale

**What's being added**:
- The scope of the bug is broader than described: any HTTP 405 cause triggers the loop, not just review count + enforce_admins
- The existing test coverage gap: there IS a test for within-batch behavior but NOT for the cross-cycle loop
- Specific implementation guidance: cooldown map filtering between `accumulate()` and `takeAction()`

**Why these labels**:
- `/area tide`: Bug is entirely within the Tide merge loop
- `/kind bug`: This is a bug in existing functionality, not a feature request
- `/help-wanted`: Level 2 effort — well-defined problem with clear solution approach, suitable for a skilled contributor

**What's NOT included**:
- No `/retitle`: Current title is already excellent
- No `/priority`: While annoying, this has a workaround (ensure GitHub reviews match Tide label requirements). Not blocking or causing data loss.
- No detailed fix guidance in the comment: the reporter already provided thorough pseudocode. Adding more would be redundant.
- No reference to #134: the reporter already linked it clearly

## Briefing Completed

Briefed maintainer on: 2026-04-03

Key questions asked:
- Why not filter benched PRs earlier, before `takeAction()`? → Agreed: filter between `accumulate()` and `takeAction()` in `syncSubpool()` to keep `accumulate()` pure and exclude benched PRs from all downstream logic including batches
- Is raising the Debug log level for `UnmergablePRError` necessary? → No: merge errors already surface through `History.Record()` in `syncSubpool()` (line 1746-1753), so the Debug log in `mergePRs()` is not the only visibility point. Dropped from recommendations.

Maintainer decisions:
- Filtering point: between `accumulate()` and `takeAction()` in `syncSubpool()`
- No log level change needed

## Next Steps

(Action items will be added here)

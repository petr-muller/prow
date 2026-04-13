# Triage for Issue #337

**Status**: In Progress
**Created**: 2026-04-13

## Issue Information

- **Issue Number**: #337
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/337

## Initial Validation

**Assessment**: LEGITIMATE

### Analysis

The issue reports a race condition in Tide's merge logic. When a GitHub Action is re-triggered, GitHub temporarily removes the old check status before the new run starts. During this brief window, Tide sees no pending/failing context for the check and may proceed to merge the PR prematurely.

**Issue Category**: Bug

**Reporter**: @saschagrunert (project MEMBER), credible reporter with direct experience of the problem

**Repository Scope Check**:
- Component mentioned: Tide (status controller)
- Exists in this repo: Yes
- Relevant code paths: `pkg/tide/status.go` (lines ~478-492 referenced by reporter)

**Information Completeness**:
- Sufficient detail provided: Yes
- Example PR provided: https://github.com/kubernetes-sigs/security-profiles-operator/pull/2595
- Code location identified by reporter
- Screenshot showing the race condition outcome

**Current State**:
- Issue is CLOSED by the lifecycle stale/rotten bot (not by a human)
- Has been reopened twice by @petr-muller to keep it alive
- Labels: `kind/bug`, `area/tide`, `lifecycle/stale`
- PR #563 attempted a fix but was closed without merging (approach: tracking previously seen contexts per PR/commit, treating disappeared contexts as PENDING)

### Recommendation

This is a clearly legitimate bug report for a race condition in Tide. The reporter is a project member who provided concrete evidence including an example PR and a code reference. The bug can cause PRs to be merged with failing or not-yet-started checks, which is a correctness issue in Tide's core merge safety logic.

**Suggested Action**:
- Reopen the issue (closed by stale bot, not resolved)
- Continue triage with research phase

## Code Research

### Current Implementation

**Primary Components**:
- **Tide sync controller**: `pkg/tide/tide.go` - Main sync loop that queries PRs, evaluates merge eligibility, and merges
- **Tide status controller**: `pkg/tide/status.go` - Async controller that updates PR status contexts (Tide status check)
- **GitHub provider**: `pkg/tide/github.go` - Fetches check statuses from GitHub API
- **Context policy**: `pkg/config/tide.go` - Determines which contexts are required/optional

**Architecture Overview**:
Tide runs two controllers asynchronously: the sync controller (merges PRs) and the status controller (updates Tide status check on PRs). Both independently query GitHub API for check statuses, creating separate evaluation windows with potentially different views of PR state.

**Key Code Paths**:
1. `pkg/tide/tide.go:755-793` - `filterPR()`: Decides if a PR should be filtered out (prevented from merging)
2. `pkg/tide/tide.go:865-889` - `unsuccessfulContexts()`: Identifies failing/missing contexts
3. `pkg/tide/tide.go:847-858` - `isPassingTests()`: Checks if all contexts pass
4. `pkg/config/tide.go:992-1006` - `MissingRequiredContexts()`: Detects missing required contexts
5. `pkg/config/tide.go:970-990` - `IsOptional()`: Determines if a context is optional
6. `pkg/tide/github.go:321-392` - `headContexts()`: Fetches check statuses from GitHub API

**Data Flow**:
1. Sync loop calls `headContexts()` → queries GitHub API for current check run states
2. `unsuccessfulContexts()` evaluates each context: non-optional, non-SUCCESS → failed
3. `MissingRequiredContexts()` checks if any `RequiredContexts` are absent → adds as EXPECTED state
4. `filterPR()` allows merge only if all unsuccessful contexts are PENDING and Prow-controlled
5. If no unsuccessful contexts → PR passes filter → eligible for merge

### Root Cause Analysis

**Primary Cause**: When a GitHub Action is re-triggered, GitHub temporarily removes the old check run before creating a new one. During this window, the context is **completely absent** from the `headContexts()` API response. Tide's `unsuccessfulContexts()` function has two ways to detect problems:

1. **First loop** (line 867-877): Iterates contexts that ARE present. If a context disappears, it's not in this list, so it's not checked.
2. **Second part** (line 878-879): Calls `MissingRequiredContexts()` which ONLY checks `RequiredContexts` list.

**The gap**: A GitHub Action check run (like `e2e-fedora`) that is:
- NOT a Prow-managed presubmit job (so not in `RequiredContexts`)
- OR in `RequiredIfPresentContexts` (required only when present, treated as "not present" when it disappears)

...becomes **completely invisible** to Tide when it disappears during re-trigger. With no failing context detected, Tide proceeds to merge.

**Contributing Factors**:
1. `MissingRequiredContexts()` only checks `RequiredContexts`, not `RequiredIfPresentContexts` (`pkg/config/tide.go:993-994`)
2. GitHub Actions check runs have different lifecycle than commit statuses - they can transiently disappear during re-trigger
3. Sync and status controllers query GitHub independently with no shared context state
4. No mechanism tracks "previously seen" contexts to detect disappearances

**Reproduction Conditions**:
- A PR has a GitHub Action check run that is required (either via branch protection or convention)
- The check run is re-triggered (manually or via push event)
- Tide's sync loop runs during the brief window when the old check run is removed but the new one hasn't been created
- The check run is not in Tide's `RequiredContexts` list (or is in `RequiredIfPresentContexts`)

### Related Code

**PR #563 fix attempt** (branch `fix-tide-retest-race-337`):
- Introduced `contextHistory` struct to track previously seen contexts per PR
- When a context disappears between sync cycles, it's treated as PENDING
- PR was closed without merging (reason unclear from comments)
- The approach is architecturally sound: it addresses the root cause by detecting context disappearances
- Added comprehensive tests including multi-transition scenarios and memory pruning

### Test Coverage

**Existing Tests**:
- `pkg/tide/status_test.go`: `TestExpectedStatus` (lines 43-871) covers many status scenarios
- `pkg/tide/tide_test.go`: Tests for `unsuccessfulContexts()`, `filterPR()`, `isPassingTests()`
- Coverage assessment: **Missing** - no test for the case where a required context disappears between sync cycles

**Test Gaps**:
- No test for context disappearance during GitHub Action re-trigger
- No test for the interaction between `RequiredIfPresentContexts` and disappearing contexts
- No test for the time-of-check/time-of-use window between sync controller iterations

### Proposed Solutions

#### Approach 1: Context History Tracking (PR #563 approach)

**Description**: Track previously seen contexts per PR. When a context that was previously present disappears from the GitHub API response, treat it as PENDING rather than absent.

**Pros**:
- Directly addresses the root cause
- Conservative: errs on the side of not merging
- Already prototyped in PR #563 with tests
- Works for all context types (GitHub Actions, commit statuses, Prow jobs)

**Cons**:
- Additional state to maintain (memory for context history)
- Requires pruning logic for merged/closed PRs
- One sync cycle delay before a genuinely removed context is treated as absent

**Affected Components**:
- `pkg/tide/tide.go`: `unsuccessfulContexts()` and `filterPR()` need context history parameter
- `pkg/tide/status.go`: `statusUpdate` struct needs context history field

**Complexity**: Medium

**Backwards Compatibility**: No breaking changes; purely additive behavior that prevents premature merges

#### Approach 2: Fresh API Query Before Merge

**Description**: Before executing a merge, perform a fresh GitHub API query for check statuses (bypassing any cache) and re-evaluate all contexts.

**Pros**:
- Simpler implementation
- No additional state tracking
- Reduces the race window significantly

**Cons**:
- Does not eliminate the race entirely (GitHub API can still be inconsistent)
- Increased API rate limit usage
- Adds latency to every merge decision

**Complexity**: Low

**Backwards Compatibility**: No breaking changes

#### Recommendation

**Preferred Approach**: Approach 1 (Context History Tracking)

This directly addresses the fundamental issue: Tide has no memory of what contexts existed in previous sync cycles. The PR #563 prototype demonstrates the approach is feasible with comprehensive tests. The main work is reviewing/updating that implementation for current main branch.

**Key Implementation Considerations**:
1. Context history should be keyed by PR identifier (not just commit SHA) to handle force pushes
2. Pruning is essential to prevent unbounded memory growth
3. Thread safety via mutex since sync and status controllers access concurrently
4. Consider logging when a context disappearance is detected for observability

**Testing Requirements**:
- Test context disappearance during re-trigger (EXPECTED state vs PENDING)
- Test that genuinely removed contexts (e.g., workflow change) eventually allow merge
- Test memory pruning for merged/closed PRs
- Test concurrent access to context history

## Next Steps

- Assess effort level for implementing the fix
- Prepare augmentation comment for the issue

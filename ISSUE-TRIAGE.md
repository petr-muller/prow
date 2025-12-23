# Triage for Issue #337

**Status**: In Progress
**Created**: 2025-12-23

## Issue Information

- **Issue Number**: #337
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/337
- **Title**: Tide merges PR when retesting GitHub action
- **Author**: saschagrunert
- **Created**: 2024-12-03
- **State**: OPEN
- **Labels**: kind/bug, area/tide

## Summary

Tide occasionally merges PRs when re-triggering GitHub Actions, even when required checks haven't completed yet. The issue appears to be a race condition in Tide's status checking logic.

## Findings

### Issue Description
- Tide merges PRs while GitHub Actions are being re-triggered
- Required checks (e.g., `e2e-fedora`) show as not started but also not failed
- Example PR: https://github.com/kubernetes-sigs/security-profiles-operator/pull/2595
- Suspected race condition in code at: pkg/tide/status.go:478-492

### Timeline
- **2024-12-03**: Issue opened by saschagrunert
- **2025-04-03**: Reopened by petr-muller after stale bot marked it rotten
- **2025-08-05**: Reopened again by petr-muller
- **2025-12-04**: saschagrunert mentions PR #563 is approaching a fix
- **2025-12-23**: Currently being triaged

### Related Work
- PR #563 is working on a fix (mentioned 2025-12-04)

## Initial Validation

**Assessment**: LEGITIMATE

### Analysis

This issue reports a race condition in Tide's merge logic when GitHub Actions are being re-triggered. The issue provides comprehensive information:

1. **Clear Problem Description**: Tide incorrectly merges PRs while required GitHub Action checks are being re-triggered, before those checks have completed
2. **Concrete Evidence**: Example PR provided (kubernetes-sigs/security-profiles-operator/pull/2595) showing the problematic behavior
3. **Code Reference**: Author identified suspected code location at pkg/tide/status.go:478-492
4. **Proper Categorization**: Already labeled as kind/bug and area/tide

**Issue Category**: Bug

**Repository Scope Check**:
- Component mentioned: Tide
- Exists in this repo: Yes (verified pkg/tide/status.go exists)
- Relevant code paths: pkg/tide/status.go:478-492
- This is a core Prow component maintained in this repository

**Information Completeness**:
- Sufficient detail provided: Yes
- Issue includes:
  - Description of when the problem occurs
  - Screenshot showing the problematic state
  - Example PR demonstrating the issue
  - Reference to suspected race condition in code
  - Already has kind/bug label
- Missing information: None critical (reproduction steps could be added but the example PR serves this purpose)

### Recommendation

**Keep open and continue triage.** This is a valid bug report for the Tide component.

The issue clearly describes a race condition bug in Prow's Tide component. The reporter (saschagrunert, a MEMBER) has:
- Identified the specific problem (race during GitHub Action re-trigger)
- Provided evidence (screenshot and example PR)
- Referenced the likely problematic code
- Demonstrated persistence by preventing the issue from being closed as stale multiple times

A fix is already being developed in PR #563, which validates that this is a known, legitimate issue being actively worked on.

**Suggested Action**:
- Keep open and continue triage
- Review PR #563 to understand the proposed solution
- Examine the suspected code paths to understand the race condition
- Consider test coverage to prevent regression

**No comment needed**: Issue is already properly triaged and being actively addressed.

## Code Research

### Current Implementation

**Primary Components**:
- **Status Checker**: pkg/tide/tide.go:847-858 (`isPassingTests`) - Determines if all required checks have passed
- **Context Evaluator**: pkg/tide/tide.go:865-889 (`unsuccessfulContexts`) - Identifies failed or missing contexts
- **GitHub Provider**: pkg/tide/github.go:333-392 (`headContexts`) - Fetches check status from GitHub API
- **CheckRun Converter**: pkg/tide/tide.go:2200-2216 (`checkRunToContext`) - Converts GitHub CheckRuns to Tide Contexts

**Architecture Overview**:
Tide periodically evaluates PRs for merge eligibility by fetching their check statuses from GitHub. For GitHub Actions (CheckRuns), the flow is:
1. Fetch commit status via GraphQL or REST API (`headContexts`)
2. Convert CheckRuns to Contexts (`checkRunToContext`)
3. Filter out unsuccessful contexts (`unsuccessfulContexts`)
4. If no unsuccessful contexts remain, consider PR as passing (`isPassingTests`)
5. Merge PR if all other requirements met

**Key Code Paths**:
1. **Merge decision**: pkg/tide/tide.go:847-858 - `isPassingTests` returns `true` only if `len(unsuccessful) == 0`
2. **Context evaluation**: pkg/tide/tide.go:874 - Contexts are unsuccessful if `ctx.State != githubql.StatusStateSuccess`
3. **CheckRun conversion**: pkg/tide/tide.go:2204-2206 - Non-completed checks get `StatusStatePending` state
4. **Missing context detection**: pkg/tide/tide.go:878-880 - Missing required contexts are added to failed list

**Data Flow**:
```
1. Tide sync loop triggers
2. For each PR, call isPassingTests()
3. isPassingTests() calls provider.headContexts()
4. headContexts() fetches from GitHub API (checks + status contexts)
5. CheckRuns converted to Contexts via checkRunToContext()
6. unsuccessfulContexts() filters for non-Success states
7. If no unsuccessful contexts, PR is eligible for merge
```

### Related Code

**Dependencies**:
- **GitHub GraphQL API**: Fetches CheckRun status and conclusion
- **Context Checker interface**: Determines which contexts are required vs optional
- **Branch Protection config**: Defines required contexts per repository

**Callers**:
- pkg/tide/tide.go:757 - `filterSubpool` calls `isPassingTests` to filter merge candidates
- pkg/tide/tide.go:912 - `pickHighestPriorityPR` uses `isPassingTests` to select merge-ready PRs

**Similar Functionality**:
- pkg/tide/tide.go:2162-2181 - `deduplicateContexts` handles multiple runs of same check
- pkg/tide/tide.go:2183-2198 - `isStateBetter` compares context states to pick best result

### Test Coverage

**Existing Tests**:
- pkg/tide/tide_test.go - Contains tests for core Tide functionality
- pkg/tide/github_test.go - Tests GitHub provider implementation
- PR #563 modifies: pkg/tide/tide_test.go - Adding tests for the fix

**Coverage Assessment**: Partial
- Basic status checking logic is tested
- Edge case of re-triggered checks causing context disappearance is **not currently tested**
- This is the test gap that allowed this bug to exist

**Test Gaps**:
- Scenario where required context temporarily disappears during re-trigger
- Race condition between context removal and new check starting
- Caching behavior when check status changes rapidly

### Documentation Review

**Code Comments**:
- pkg/tide/github.go:336-342: Documents fallback when commits aren't in logical order
- pkg/tide/tide.go:2136-2140: Explains empty checkrun per status context behavior
- pkg/tide/tide.go:2204-2206: Pending state assigned to non-completed checks
- **No comments** explicitly warning about re-trigger race condition

**Design Documentation**:
- No ADRs or design docs found specifically addressing check status handling
- Comments assume check contexts persist once created

**Known Limitations**:
- pkg/tide/status.go:483-487: Acknowledges status controller could fall behind sync loop
- However, this refers to tide status context updates, not the re-trigger race

### Root Cause Analysis

**Primary Cause**:
When a GitHub Action is re-triggered, GitHub **temporarily removes the old CheckRun** before creating the new one. This creates a brief window where:
1. The old check (with Success conclusion) is deleted from GitHub's API response
2. The new check hasn't started yet, so it doesn't appear in the API response
3. The required context is completely **missing** from the contexts list
4. Tide's `MissingRequiredContexts` check should catch this, BUT there's a race:
   - If Tide fetches contexts during this window, the check is missing
   - If the context wasn't previously tracked as required for this specific commit, it may not be detected as missing
   - Tide may conclude "no unsuccessful contexts" and proceed with merge

**Contributing Factors**:
1. **No state persistence**: Tide doesn't track what contexts were previously seen for a commit
2. **Timing window**: The removal-to-creation window can be several seconds
3. **Sync frequency**: Tide sync loops run periodically, increasing chance of hitting the window
4. **GitHub API behavior**: Removing old check before creating new one is GitHub's design

**Reproduction Conditions**:
- Required GitHub Action check exists and has passed
- User manually re-triggers the check (or re-runs workflow)
- Tide sync occurs during the window between old check removal and new check creation
- No other checks are failing

### Proposed Solutions

#### Approach 1: Track Previously Seen Contexts (PR #563's Approach)

**Description**: Maintain state of all contexts previously observed for each PR/commit. When a required context that was previously seen disappears, treat it as PENDING rather than missing/passing.

**Pros**:
- Directly addresses root cause by detecting disappearing contexts
- Conservative approach - prevents premature merges
- Maintains historical context information for better decision making
- Low risk of false negatives

**Cons**:
- Requires additional state storage and management
- Adds complexity to context evaluation logic
- Need to handle state cleanup for old PRs/commits
- Slightly increases memory footprint

**Affected Components**:
- pkg/tide/tide.go: Add context tracking per PR/commit
- pkg/tide/tide.go: Modify `unsuccessfulContexts` to check for disappeared contexts
- pkg/tide/tide_test.go: Add tests for disappearing context scenario

**Complexity**: Medium

**Backwards Compatibility**: Fully compatible - only makes merge decisions more conservative

#### Approach 2: Require Fresh Status Before Merge

**Description**: For PRs about to be merged, bypass any caching and fetch fresh status directly from GitHub API immediately before merge decision.

**Pros**:
- Reduces time window for race condition significantly
- Simple conceptual model - always use latest data for merge
- No state persistence needed

**Cons**:
- Doesn't eliminate race entirely (window still exists, just smaller)
- Increases GitHub API calls (rate limit considerations)
- May slow down merge decisions slightly
- If GitHub's removal-to-creation window is very short, might miss it anyway

**Affected Components**:
- pkg/tide/tide.go: Add fresh fetch before merge decision
- pkg/tide/github.go: Ensure no caching for merge-critical fetches

**Complexity**: Low

**Backwards Compatibility**: Fully compatible

#### Approach 3: Detect State Transitions

**Description**: Track context state changes over time. If a context transitions from SUCCESS → (missing) within a short timeframe, treat as PENDING for a grace period.

**Pros**:
- Handles re-trigger scenario specifically
- Can detect other anomalous state transitions
- Provides grace period for check to reappear

**Cons**:
- More complex state machine
- Need to tune grace period appropriately
- Requires timestamp tracking
- May delay merges unnecessarily if context legitimately removed

**Affected Components**:
- pkg/tide/tide.go: Add state transition tracking
- pkg/tide/tide.go: Implement grace period logic

**Complexity**: High

**Backwards Compatibility**: May delay some legitimate merges during grace period

#### Recommendation

**Preferred Approach**: Approach 1 (Track Previously Seen Contexts) - PR #563's solution

This is the approach proposed in PR #563 and addresses the root cause most directly. By tracking previously seen contexts, Tide can distinguish between:
- A context that never existed (legitimately not required)
- A context that disappeared (suspicious - likely re-trigger race)

**Key Implementation Considerations**:
1. **State storage**: Use in-memory map keyed by PR/commit to track seen contexts
2. **Cleanup**: Implement cleanup for closed/merged PRs to prevent memory growth
3. **Context identification**: Ensure context names match exactly between observations
4. **Thread safety**: Protect shared state with appropriate locking

**Testing Requirements**:
- Test case: Required context present, then disappears, PR should not merge
- Test case: Required context never present, PR should not merge (existing behavior)
- Test case: Optional context disappears, PR should still merge
- Test case: Context returns after disappearing, PR should merge once SUCCESS again

**Migration/Rollout Strategy**:
- This is a bug fix with no configuration changes required
- Can be rolled out immediately as it only makes merge decisions more conservative
- Monitor for any PRs incorrectly blocked (false positives), though unlikely
- No migration of existing state needed - tracking starts from rollout

## Effort Assessment

**Effort Level**: 3 - Large (requires expertise)

### Summary

Fixing this race condition requires deep understanding of concurrency, Tide's merge logic, and careful state management. While the solution approach is well-defined (PR #563), the complexity of race conditions and the criticality of Tide's merge path make this appropriate only for experienced contributors.

### Factor Analysis

#### Scope of Changes
- **Assessment**: Moderate
- **Details**: 3 files affected (pkg/tide/tide.go, pkg/tide/status.go, pkg/tide/tide_test.go), estimated ~150 lines of code modifications. Changes touch critical merge decision path.
- **Level Indication**: 2-3

#### Complexity
- **Assessment**: High
- **Details**:
  - Race condition involving timing windows between GitHub API calls
  - State tracking across sync loops
  - Concurrent access to shared state requires proper synchronization
  - Timing-dependent behavior difficult to test and debug
  - Must handle context disappearance/reappearance correctly
- **Level Indication**: 3-4

#### Required Expertise
- **Assessment**: Deep
- **Details**:
  - Understanding of race conditions and concurrent programming
  - Deep knowledge of Tide's architecture and merge flow
  - Familiarity with GitHub API behavior (CheckRun lifecycle)
  - State management and cleanup patterns
  - Go concurrency primitives (mutexes, proper locking)
  - Critical path in production system - mistakes could cause incorrect merges or blocks
- **Level Indication**: 3-4

#### Clarity and Certainty
- **Assessment**: Well-defined
- **Details**:
  - Root cause clearly identified (GitHub removes old CheckRun before creating new one)
  - Solution approach documented in PR #563
  - Known implementation: track previously seen contexts
  - Testing scenarios defined
- **Level Indication**: 1-2

#### Testing Requirements
- **Assessment**: Complex
- **Details**:
  - Must test race condition scenario (context disappears during re-trigger)
  - Requires understanding timing windows
  - Need tests for edge cases: optional vs required contexts, reappearance, cleanup
  - Integration with existing Tide test suite
  - Difficult to reproduce timing-dependent behavior reliably in tests
- **Level Indication**: 3-4

#### Backwards Compatibility
- **Assessment**: Fully compatible
- **Details**:
  - Only prevents incorrect merges (conservative change)
  - No configuration changes required
  - No breaking changes to Tide's behavior
  - Existing deployments benefit immediately without changes
- **Level Indication**: 1-2

#### Architectural Alignment
- **Assessment**: Good fit
- **Details**:
  - Extends existing context checking mechanism
  - Follows Tide's pattern of tracking PR state
  - Adds state tracking but doesn't contradict architecture
  - Requires new pattern (context history per PR/commit) but well-justified
- **Level Indication**: 2-3

#### External Dependencies
- **Assessment**: Well-supported
- **Details**:
  - Works around GitHub API behavior (documented in issue)
  - GitHub's CheckRun removal-then-creation is external limitation
  - Solution doesn't require GitHub API changes
  - No blocking external dependencies
- **Level Indication**: 1-3

### Recommended Labels

Based on this assessment, recommend the following labels:
- [x] `area/tide`: Core Tide functionality
- [x] `kind/bug`: Fixing race condition causing incorrect merges
- [x] `priority/important-soon`: Can cause incorrect merges in production
- [ ] `good-first-issue`: **Not recommended** - Requires deep expertise in concurrency and Tide
- [ ] `help-needed`: **Not recommended** - Too complex for typical help-needed; requires Tide expertise

### Guidance for Contributors

**For Level 3 (Large - Requires Expertise)**:

**Prerequisites**:
- Experience with concurrent programming and race conditions
- Strong understanding of Tide's architecture
- Familiarity with Go synchronization primitives
- Understanding of GitHub API behavior and webhooks

**Recommended Preparation**:
1. Review existing Tide code:
   - pkg/tide/tide.go:847-858 (`isPassingTests`)
   - pkg/tide/tide.go:865-889 (`unsuccessfulContexts`)
   - pkg/tide/github.go:333-392 (`headContexts`)
   - pkg/tide/tide.go:2200-2216 (`checkRunToContext`)

2. Understand the race condition:
   - GitHub removes old CheckRun when re-triggered
   - Brief window where context is missing from API response
   - Tide may see "no unsuccessful contexts" and incorrectly merge

3. Study PR #563:
   - Review the proposed implementation
   - Understand the state tracking approach
   - Examine test cases added

**Key Implementation Considerations**:
- **Thread Safety**: Shared state for tracking contexts must be protected with proper locking
- **State Cleanup**: Implement cleanup for merged/closed PRs to prevent unbounded memory growth
- **Context Identification**: Ensure exact matching of context names across observations
- **Edge Cases**: Handle optional vs required contexts, reappearing contexts, stale data

**Testing Strategy**:
- Test required context disappears → merge blocked
- Test optional context disappears → merge proceeds
- Test context reappears after disappearing → merge proceeds when SUCCESS
- Test state cleanup for old PRs
- Consider timing-dependent test scenarios

**Before Starting**:
- Consult with Tide maintainers
- Verify approach aligns with PR #563 or propose alternatives
- Discuss testing strategy for race conditions
- Plan rollout and monitoring strategy

### Caveats and Considerations

**Why Level 3 (not Level 2)**:
While the solution is well-defined by PR #563, several factors elevate this to Level 3:
- Race conditions are notoriously difficult to implement correctly
- Tide's merge path is critical infrastructure - bugs could cause widespread issues
- Concurrent state management requires advanced Go expertise
- Testing race conditions properly is challenging

**Why Level 3 (not Level 4)**:
- Solution approach is clear and proven viable (PR #563 exists)
- No architectural contradictions - extends existing patterns
- Fully backwards compatible
- No external blockers

**Alternative Consideration**:
If PR #563 already implements a complete solution and only needs review, the remaining work (code review, testing verification, deployment) might be Level 2. However, implementing this from scratch is definitively Level 3.

## Next Steps

1. ✅ **Review PR #563**: Solution tracks previously seen contexts to detect disappearing checks
2. ✅ **Root cause identified**: GitHub removes old CheckRun before new one starts during re-trigger
3. ✅ **Effort assessed**: Level 3 - Requires expertise due to concurrency complexity
4. **Verify PR #563 implementation**: Review code changes to ensure complete solution
5. **Test coverage**: Ensure PR #563 includes tests for the disappearing context scenario
6. **Consider monitoring**: Add metrics to track how often contexts disappear to measure bug frequency

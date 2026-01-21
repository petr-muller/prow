# Triage for Issue #436

**Status**: In Progress
**Created**: 2026-01-21

## Issue Information

- **Issue Number**: #436
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/436

## Findings

### Initial Validation

**Assessment**: LEGITIMATE

**Issue Category**: Feature Request

**Summary**: This issue requests functionality to prevent duplicate concurrent reruns of the same test job in Prow. During the Kubernetes 1.33.0 release, multiple release managers accidentally triggered reruns of the same failed test, resulting in 3 simultaneous runs of the same test, wasting resources.

**Repository Scope Check**:
- Component mentioned: Tide (for job scheduling/execution)
- Exists in this repo: Yes
- Labels applied: `kind/feature`, `area/tide`
- Relevant functionality: Job rerun/scheduling logic

**Information Completeness**:
- Sufficient detail provided: Yes
- Real-world use case: Kubernetes 1.33.0 release cut with multiple concurrent reruns
- Proposed solutions: (1) Discard additional runs if one is already scheduled/pending, or (2) GUI lock on rerun button
- Active maintainer discussion: BenTheElder noted there's an existing queue feature with n=1 maximum that could help, but it's not widely configured

**Analysis**:

This is a legitimate feature request for Prow's job execution system. The issue describes a real problem encountered during the Kubernetes release process where:
1. A test job failed (Conformance-GCE-1.33-kubetest2)
2. Multiple release managers were investigating simultaneously
3. Each triggered a rerun without knowing others had done the same
4. This resulted in 3 concurrent runs of the same test, wasting CI resources

The discussion reveals there's already a partial solution: a queue feature that can limit concurrent runs to n=1, used for resource-constrained jobs like the image promoter and scale tests. However, this isn't configured everywhere it should be.

The issue author proposes:
- Primary solution: Automatically discard duplicate rerun requests if a run is already scheduled/pending
- Alternative: Add a GUI lock on the rerun button (noted as complicated to implement)

This is a valid enhancement request that would improve resource utilization and prevent accidental duplicate runs, especially in high-stakes scenarios like release cuts.

### Recommendation

**Suggested Action**: Keep open and continue triage

This is a legitimate feature request with:
- Clear problem statement backed by real-world incident
- Proposed technical solutions
- Active maintainer engagement
- Appropriate labeling (kind/feature, area/tide)
- Ongoing interest from the community (issue kept alive by removing stale labels multiple times)

**Next Steps**:
1. Research the existing queue feature mentioned by BenTheElder
2. Investigate job scheduling/rerun code paths
3. Assess effort required for proposed solutions
4. Consider whether this should be a new feature or better documentation/defaults for existing queue feature

### Code Research

#### Current Implementation

**Primary Components**:
- **Deck (Rerun UI)**: cmd/deck/rerun.go - Handles rerun button clicks and creates new ProwJob objects
- **Plank (Scheduler)**: pkg/plank/reconciler.go - Enforces concurrency limits and schedules jobs
- **Config**: pkg/config/config.go:722-728 - Defines `JobQueueCapacities` for queue-based concurrency control
- **ProwJob Spec**: pkg/apis/prowjobs/v1/types.go:228-235 - Defines `JobQueueName` field for queue assignment

**Architecture Overview**:

The existing queue feature provides per-queue concurrency limits:
1. Jobs can be assigned to named queues via `job_queue_name` field
2. Plank config defines max concurrency per queue in `job_queue_capacities` map
3. Plank reconciler counts pending/triggered jobs in each queue before allowing new jobs
4. Jobs exceeding the limit remain in SchedulingState until capacity becomes available

However, this is **opt-in** and requires manual configuration. There's no automatic duplicate prevention.

**Key Code Paths**:
1. **Rerun Creation**: cmd/deck/rerun.go:245-347 - handleRerun() creates new ProwJob from user click
2. **Queue Configuration**: pkg/config/config.go:722-728 - JobQueueCapacities map definition
3. **Concurrency Check (Per-Queue)**: pkg/plank/reconciler.go:931-962 - canExecuteConcurrentlyPerQueue()
4. **Concurrency Check (Per-Job)**: pkg/plank/reconciler.go:909-929 - canExecuteConcurrentlyPerJob()
5. **Job Counting**: pkg/plank/reconciler.go:1117-1139 - countPendingOrOlderTriggeredMatchingPJs()
6. **Field Indexing**: pkg/plank/reconciler.go:1037-1076 - Fast lookup of pending/triggered jobs by name/queue

**Data Flow**:
```
User clicks "Rerun" button in Deck UI
  ↓
Deck validates authorization (rerun.go:211-234)
  ↓
Deck creates new ProwJob object (rerun.go:326)
  ↓
ProwJob posted to Kubernetes API in SchedulingState
  ↓
Plank reconciler watches ProwJob objects
  ↓
Plank acquires serialization lock (reconciler.go:401)
  ↓
Plank checks canExecuteConcurrentlyPerJob() - checks MaxConcurrency
  ↓
Plank checks canExecuteConcurrentlyPerQueue() - checks JobQueueCapacities
  ↓
If allowed: Create Pod, transition to TriggeredState → PendingState
If blocked: Leave in SchedulingState, retry later
```

#### Related Code

**Dependencies**:
- Kubernetes client-go field indexers for fast ProwJob lookups by name/queue
- Serialization locks to prevent race conditions in reconciler

**Queue Examples** (mentioned by BenTheElder):
- Image promoter jobs: Use queue with capacity=1
- Scale tests: Use queue with capacity=1
- Configuration exists but not widely applied

**Similar Functionality**:
- MaxConcurrency field (pkg/apis/prowjobs/v1/types.go:174-179) - per-job concurrency limit
- Both MaxConcurrency and JobQueueName work together with two-level concurrency control

#### Test Coverage

**Existing Tests**:
- pkg/plank/reconciler_test.go:295-504 - Queue concurrency test cases
- Tests verify queue capacity enforcement
- Tests verify per-job MaxConcurrency enforcement
- Coverage assessment: Good for queue feature, but no tests for rerun deduplication

**Test Gaps**:
- No tests for duplicate rerun detection at the Deck level
- No tests for race conditions when multiple reruns happen simultaneously
- No tests for the specific scenario in issue #436 (3 concurrent manual reruns)

#### Documentation Review

**Code Comments**:
- pkg/plank/reconciler.go:226-239 describes two-level concurrency control design
- Comments explain that 0 capacity blocks all jobs, negative values mean unlimited

**Design Documentation**:
- No explicit design doc found for the queue feature
- Feature appears to have evolved to handle resource-constrained jobs

**Known Limitations**:
- Queue feature is opt-in, requires manual configuration per job
- No automatic duplicate detection for manual reruns
- Relies on timing: if second rerun reaches Plank before first transitions to Pending, both can pass concurrency check

#### Root Cause Analysis

**Primary Cause**:

The issue is a **race condition in rerun request handling combined with lack of automatic deduplication**:

1. **No rerun deduplication at Deck level**: When multiple users click "Rerun" for the same job, Deck creates multiple distinct ProwJob objects (cmd/deck/rerun.go:326). Each gets a unique UID and creation timestamp.

2. **Timing-dependent concurrency check**: Plank's concurrency check (reconciler.go:1117-1139) counts "pending or older triggered" jobs. If two reruns are created nearly simultaneously:
   - First rerun enters reconciler, sees 0 pending jobs, allowed through
   - Second rerun enters reconciler before first transitions to PendingState
   - Second rerun also sees 0 pending jobs (first still in TriggeredState), also allowed through

3. **Queue feature not universally configured**: The existing queue feature could prevent this, but:
   - It's opt-in, requiring manual configuration
   - Most jobs don't use it (only resource-constrained ones like image promoter)
   - The Conformance-GCE job in the incident likely had no queue configured

**Contributing Factors**:

1. **Multiple users with rerun permissions**: In release scenarios, multiple release managers can trigger reruns
2. **No UI feedback about pending reruns**: Users can't see if someone else already triggered a rerun
3. **Serialization locks only prevent same-thread races**: The locks (reconciler.go:401) prevent races within the reconciler, but don't prevent multiple rerun requests from creating multiple ProwJobs in the first place
4. **State transition timing**: Brief window where a job is TriggeredState but not yet PendingState

**Reproduction Conditions**:

- Job without queue configuration (no job_queue_name set)
- Job without MaxConcurrency=1 set
- Multiple users with rerun permissions active simultaneously
- Multiple rerun clicks within seconds of each other
- First job hasn't transitioned to PendingState when second job is reconciled

#### Proposed Solutions

##### Approach 1: Automatic Queue Assignment with Smart Defaults

**Description**:
Automatically assign all jobs to queues based on job name, with default capacity of 1 for jobs of the same name. This makes duplicate prevention automatic without requiring manual configuration.

Implementation would involve:
- Modify Plank to auto-assign jobs without explicit `job_queue_name` to an implicit queue based on job name
- Set default queue capacity to 1 for same-named jobs unless overridden
- Maintain backwards compatibility by allowing explicit queues and MaxConcurrency to override defaults

**Pros**:
- Zero configuration required - works automatically
- Prevents duplicates for all jobs, not just manually configured ones
- Leverages existing queue infrastructure
- Backwards compatible with existing queue configurations

**Cons**:
- Changes default behavior (may delay some jobs that previously ran concurrently)
- May not be desired for all job types (some jobs might want parallel reruns)
- Requires careful rollout to avoid breaking existing workflows
- Could impact throughput for legitimately parallel jobs

**Affected Components**:
- Plank reconciler: Add auto-queue assignment logic
- Config: Add global defaults for queue behavior
- Documentation: Update to explain new default behavior

**Complexity**: Medium

**Backwards Compatibility**: Requires opt-out mechanism for jobs that need parallel execution

##### Approach 2: Rerun Deduplication in Deck

**Description**:
Add duplicate detection in Deck's rerun handler before creating new ProwJob objects. When a rerun is requested, check if there's already a pending/triggered ProwJob for the same job within a recent time window (e.g., last 5 minutes).

Implementation would involve:
- Modify cmd/deck/rerun.go handleRerun() to query existing ProwJobs
- Check for jobs with same name in SchedulingState, TriggeredState, or PendingState
- If found and created recently (within threshold), return existing job instead of creating new one
- Add UI feedback showing "Rerun already in progress"

**Pros**:
- Addresses issue at the source (prevents duplicate ProwJob creation)
- Users get immediate feedback about existing reruns
- No impact on Plank scheduler complexity
- Can be applied selectively based on job type or configuration
- Simpler rollout - changes isolated to Deck

**Cons**:
- Requires Deck to query/filter ProwJobs (adds complexity to Deck)
- Need to define "recent" time window (5 min? 10 min?)
- Doesn't help with automated rerun systems (only manual UI reruns)
- Edge case: legitimate parallel reruns might be blocked
- Requires careful UX design for when to allow vs block reruns

**Affected Components**:
- Deck rerun handler: Add duplicate check before ProwJob creation
- Deck UI: Add feedback message for duplicate rerun attempts
- Tests: Add rerun deduplication test cases

**Complexity**: Low to Medium

**Backwards Compatibility**: High - only affects rerun button behavior, doesn't change scheduling

##### Approach 3: Enhanced Queue Feature with Per-Job Defaults

**Description**:
Enhance the existing queue feature to support per-job-name default MaxConcurrency without requiring explicit queue assignment. This is a middle ground between full automatic queuing and manual configuration.

Implementation would involve:
- Add new config field: `default_max_concurrency_per_job_name: 1`
- Apply this default to jobs without explicit MaxConcurrency or JobQueueName
- Allow jobs to opt out via `max_concurrency: -1` (unlimited)
- Use existing Plank concurrency checking infrastructure

**Pros**:
- Minimal code changes - uses existing concurrency checking
- Simple configuration - single global setting
- Jobs can opt out if needed
- No impact on already-configured jobs

**Cons**:
- Still requires some configuration (though global, not per-job)
- Doesn't prevent duplicate ProwJob creation, only scheduling
- Delayed feedback - users create duplicate jobs that sit in SchedulingState
- Less elegant than Approach 2 for user experience

**Affected Components**:
- Config: Add default_max_concurrency_per_job_name field
- Plank reconciler: Apply default when checking concurrency
- Documentation: Explain new default setting

**Complexity**: Low

**Backwards Compatibility**: High - only affects jobs without explicit concurrency settings

##### Approach 4: GUI Lock (mentioned in issue, but complex)

**Description**:
Add client-side UI locking to disable the rerun button temporarily after click, preventing multiple clicks from the same user.

**Pros**:
- Prevents accidental double-clicks from same user
- Purely UI change, no backend impact

**Cons**:
- Doesn't prevent multiple different users from clicking rerun (the actual issue)
- Complex to implement with retries/failures
- Client-side only - can be bypassed
- Doesn't address the fundamental problem

**Complexity**: High (UI state synchronization)

**Not recommended** - doesn't solve the core issue

#### Recommendation

**Preferred Approach**: **Approach 2 - Rerun Deduplication in Deck**

**Rationale**:
1. **Addresses root cause directly**: Prevents duplicate ProwJob creation at the source (Deck rerun handler)
2. **Best user experience**: Immediate feedback when rerun already exists
3. **Targeted solution**: Only affects manual reruns, doesn't change default behavior for all jobs
4. **Simpler rollout**: Changes isolated to Deck component
5. **Low risk**: High backwards compatibility, can be feature-flagged

**Hybrid recommendation**: Combine Approach 2 with Approach 3:
- Implement rerun deduplication in Deck for immediate UX improvement
- Add global default MaxConcurrency setting as defense-in-depth
- This provides both UI-level prevention and scheduler-level enforcement

**Key Implementation Considerations**:

1. **Time window**: Define threshold for "duplicate" (suggest 5-10 minutes)
2. **State filter**: Check for SchedulingState, TriggeredState, PendingState (not terminal states)
3. **UI feedback**: Return informative message: "A rerun was already triggered X minutes ago by user Y"
4. **Configuration option**: Allow per-job opt-out via `allow_parallel_reruns: true`
5. **Monitoring**: Add metrics for blocked duplicate reruns to track effectiveness

**Testing Requirements**:

1. Unit tests:
   - Rerun deduplication logic
   - Time window boundary conditions
   - Opt-out configuration handling

2. Integration tests:
   - Multiple simultaneous rerun requests
   - Rerun after time window expires
   - Rerun of different jobs (should not be blocked)

3. E2E tests:
   - User clicks rerun, sees existing rerun message
   - User waits for job completion, can rerun again

**Migration/Rollout Strategy**:

1. **Phase 1**: Add deduplication logic with feature flag disabled
2. **Phase 2**: Enable for specific high-resource jobs (scale tests, conformance tests)
3. **Phase 3**: Gather feedback, adjust time window if needed
4. **Phase 4**: Enable globally with opt-out for jobs that need it
5. **Phase 5**: Add UI improvements (show existing rerun status)

### Effort Assessment

**Effort Level**: 2 - Moderate (help-needed)

#### Summary

This is a moderate-complexity feature requiring changes to 2-4 files (~200-300 LOC) with a well-defined solution approach. While the problem and solution are clear, implementation requires understanding ProwJob lifecycle, Kubernetes client usage, and Deck's authorization flow. Suitable for contributors with some Prow experience.

#### Factor Analysis

##### Scope of Changes
- **Assessment**: Moderate
- **Details**:
  - Primary: cmd/deck/rerun.go - Add duplicate detection logic to handleRerun() (~100-150 LOC)
  - Secondary: cmd/deck/rerun_test.go - Add comprehensive test cases (~100-150 LOC)
  - Optional: pkg/config/config.go - Add configuration for time window and opt-out behavior (~20-30 LOC)
  - Total: 2-4 files, approximately 200-300 lines of code
  - Affects single component (Deck), isolated changes
- **Level Indication**: 2-3

##### Complexity
- **Assessment**: Moderate
- **Details**:
  - Core logic is straightforward: query existing ProwJobs, filter by state/name/time, return result
  - Need to handle time window calculation (within last N minutes)
  - State filtering: include SchedulingState, TriggeredState, PendingState; exclude terminal states
  - Edge cases: job name matching, handling missing timestamps, concurrent rerun requests
  - No concurrency primitives needed (Kubernetes API handles that)
  - No algorithmic challenges
  - Main complexity is in getting the filtering logic right
- **Level Indication**: 2

##### Required Expertise
- **Assessment**: Moderate
- **Details**:
  - **Go programming**: Standard Go with Kubernetes client-go
  - **ProwJob lifecycle**: Understanding of ProwJob states (Scheduling → Triggered → Pending → Terminal)
  - **Kubernetes client**: Using client to list/filter ProwJobs with field selectors
  - **Deck internals**: Understanding rerun handler flow and authorization
  - **Testing**: Writing unit tests with mock Kubernetes clients
  - Can be learned from existing code patterns in cmd/deck/
  - Does NOT require: Deep Prow architecture knowledge, concurrency expertise, or distributed systems concepts
- **Level Indication**: 2-3

##### Clarity and Certainty
- **Assessment**: Well-defined with minor details to finalize
- **Details**:
  - Problem is crystal clear from issue and research
  - Solution approach (Approach 2) is well-documented and agreed upon
  - Some implementation details to decide:
    - Time window threshold (recommend 5-10 minutes, make configurable)
    - Exact UI feedback message format
    - Whether to make it opt-in initially vs default-on with opt-out
  - But these are minor decisions, not fundamental uncertainty
- **Level Indication**: 1-2

##### Testing Requirements
- **Assessment**: Moderate
- **Details**:
  - Unit tests needed:
    1. Duplicate detection logic: same job, within time window → blocked
    2. Time window boundaries: just inside window → blocked, just outside → allowed
    3. Different job names → allowed
    4. Terminal state jobs → allowed (not considered duplicates)
    5. Opt-out configuration → allowed even if duplicate
  - Can follow existing test patterns in cmd/deck/rerun_test.go
  - Need to mock Kubernetes client for ProwJob queries
  - No integration tests required for initial implementation
  - Testing is straightforward, not complex
- **Level Indication**: 2-3

##### Backwards Compatibility
- **Assessment**: Fully compatible
- **Details**:
  - Additive feature - doesn't break existing functionality
  - Only affects behavior when duplicate rerun is attempted
  - Can be rolled out with feature flag for gradual adoption
  - Can provide opt-out per job via configuration: `allow_parallel_reruns: true`
  - No impact on existing jobs or deployments unless enabled
  - Minimal risk: worst case is blocking a legitimate rerun, easily fixed by waiting or configuration
  - No API changes, no config format changes (unless adding opt-out option)
- **Level Indication**: 1-2

##### Architectural Alignment
- **Assessment**: Good fit
- **Details**:
  - Natural extension of Deck's rerun handler responsibilities
  - Deck already handles authorization, validation, and job creation
  - Adding duplicate check fits logically before job creation step
  - Follows pattern: validate → check authorization → **check duplicates** → create job
  - No new architectural patterns required
  - Doesn't contradict any existing design decisions
  - BenTheElder suggested existing queue feature, but rerun deduplication is complementary, not contradictory
- **Level Indication**: 2-3

##### External Dependencies
- **Assessment**: Well-supported
- **Details**:
  - Uses standard Kubernetes client-go to query ProwJobs
  - ProwJobs are CRDs stored in Kubernetes API - well-supported, stable
  - No GitHub API dependencies
  - No external service dependencies
  - All required APIs are mature and documented
- **Level Indication**: 1-3

#### Effort Level Determination

**Factors favoring Level 1**: Clarity (1-2), Backwards compatibility (1-2)
**Factors favoring Level 2**: Scope (2-3), Complexity (2), Expertise (2-3), Testing (2-3), Architecture (2-3), External deps (1-3)
**Factors favoring Level 3**: None
**Factors favoring Level 4**: None

**Overall assessment**: Clear **Level 2** - Most factors point to moderate effort. While the problem is well-defined and clear, the implementation requires moderate understanding of Prow components and moderate code changes across multiple files.

#### Recommended Labels

Based on this assessment:
- [x] `help-wanted`: Good scope for skilled contributors familiar with Go and Kubernetes
- [x] `kind/feature`: New feature request (already applied)
- [x] `area/deck`: Primary component affected (currently labeled area/tide, should also include area/deck)
- [ ] `good-first-issue`: Requires moderate Prow knowledge and understanding of ProwJob lifecycle, not suitable for complete beginners
- [ ] `priority/important-soon`: Nice to have but not critical (up to maintainers)

#### Guidance for Contributors

**For Level 2 (Moderate)**:

**Suitable for**: Contributors who have:
- Solid Go programming experience
- Familiarity with Kubernetes client-go library
- Understanding of (or willingness to learn) ProwJob CRD lifecycle
- Experience writing unit tests with mocks

**Preparation steps**:
1. **Read the code**:
   - cmd/deck/rerun.go - Understand current rerun handler flow
   - pkg/apis/prowjobs/v1/types.go - Understand ProwJob states and spec fields
   - cmd/deck/rerun_test.go - See existing test patterns

2. **Understand ProwJob states**:
   - SchedulingState: Just created, waiting to be scheduled
   - TriggeredState: Scheduled, pod being created
   - PendingState: Pod running
   - Terminal states: SuccessState, FailureState, AbortedState, ErrorState

3. **Review this triage document**: All research and solution approaches are documented in ISSUE-TRIAGE.md

**Recommended implementation approach**:

1. **Start with the duplicate detection helper function**:
   ```go
   func (s *Server) findRecentPendingRerun(ctx context.Context, jobName string, within time.Duration) (*prowv1.ProwJob, error)
   ```
   This queries ProwJobs by name, filters by state and creation time.

2. **Modify handleRerun()** to call the helper before creating new ProwJob:
   - If duplicate found, return HTTP response with informative message
   - If no duplicate, proceed with existing job creation logic

3. **Add configuration** (optional for initial PR):
   - Time window configuration (default 5 minutes)
   - Per-job opt-out flag

4. **Write comprehensive tests**:
   - Follow patterns in cmd/deck/rerun_test.go
   - Mock Kubernetes client to return various ProwJob scenarios
   - Test boundary conditions

**Key implementation considerations**:
- Use field selectors or client-side filtering to find ProwJobs by name
- Check CreationTimestamp to enforce time window
- Handle edge case where job has no CreationTimestamp set
- Consider using a configurable feature flag for gradual rollout
- Return user-friendly error message: "A rerun was already triggered 3 minutes ago"

**Questions to ask maintainers**:
- Preferred time window default (5 min? 10 min?)
- Should this be opt-in (feature flag) initially or opt-out?
- Should we add metrics for blocked duplicate reruns?
- Preferred error message format for UI display

**Related code to review**:
- pkg/plank/reconciler.go:1117-1139 - Shows how to count/filter ProwJobs by state
- cmd/deck/rerun.go:151-162 - Shows how to query ProwJob by name

#### Caveats and Considerations

**Important notes**:

1. **Hybrid approach**: Consider implementing both Approach 2 (Deck deduplication) and Approach 3 (global default MaxConcurrency) for defense-in-depth. This would increase scope to Level 2-3 boundary but provide more robust solution.

2. **Edge case - legitimate parallel reruns**: Some jobs may legitimately want multiple concurrent reruns (e.g., flaky test investigation). Ensure opt-out mechanism exists.

3. **Time window trade-off**:
   - Too short (1-2 min): May not prevent duplicates if first job takes time to transition states
   - Too long (30+ min): May block legitimate reruns after fixing infrastructure issues
   - Recommend: Start with 5-10 minutes, make configurable

4. **Feature flag recommendation**: Consider implementing with feature flag disabled by default, then enable for specific jobs (scale tests, conformance tests) to validate before global rollout.

5. **Future enhancement**: Could extend to show "Rerun already in progress" in UI proactively, not just when clicking rerun button. This would be a follow-up enhancement.

6. **Area label**: Issue is currently labeled `area/tide` but should also (or instead) be labeled `area/deck` since the implementation is in Deck. Tide is not directly involved.

### Proposed Issue Augmentation

#### Title Change

- **Current**: `[feature] Rerun queue discard extra runs`
- **Proposed**: `Prevent duplicate concurrent job reruns in Deck`
- **Rationale**:
  - Remove non-standard `[feature]` prefix (kind label serves this purpose)
  - "Rerun queue" is confusing - no such thing exists, issue is about preventing duplicates
  - Add component name (Deck) for clarity
  - More specific and actionable phrasing
  - Concise and descriptive

#### Proposed GitHub Comment

```markdown
/retitle Prevent duplicate concurrent job reruns in Deck

## Root Cause

This issue occurs due to a race condition combined with lack of automatic deduplication. When multiple users click "Rerun" within seconds of each other, Deck's rerun handler (`cmd/deck/rerun.go:245-347`) creates multiple distinct ProwJob objects, each with a unique ID. If these jobs reach Plank's scheduler before the first transitions from TriggeredState to PendingState, all pass the concurrency check and execute simultaneously. While BenTheElder mentioned the existing queue feature (`job_queue_name` with capacity limits), it's opt-in and not configured for most jobs, including the Conformance-GCE job from the incident.

## Recommended Solution

Add duplicate detection in Deck before creating new ProwJobs. When a rerun is requested, query for existing ProwJobs with the same name in SchedulingState, TriggeredState, or PendingState within a recent time window (5-10 minutes). If found, return the existing job instead of creating a duplicate, with UI feedback like "A rerun was already triggered 3 minutes ago by user X." This approach prevents duplicates at the source and provides immediate user feedback.

## Implementation Guidance

The fix should be implemented in `cmd/deck/rerun.go` by adding a helper function to query recent pending reruns before the job creation step at line 326. The implementation can reference `pkg/plank/reconciler.go:1117-1139` for ProwJob state filtering patterns. This is a moderate-complexity change affecting 2-4 files (~200-300 LOC) suitable for contributors familiar with Go and Kubernetes client-go. All triage research and detailed implementation guidance is available in the `issue-triage-436` branch.

/area deck
/help-wanted
```

#### Rationale

**What's being added**:

1. **Root cause explanation**: The original issue describes the symptom (3 concurrent runs) but doesn't explain *why* it happens. Added technical explanation of the race condition between Deck creating jobs and Plank checking concurrency, plus clarification that the existing queue feature doesn't solve this because it's opt-in.

2. **Recommended solution**: The issue asks "how can we ensure..." but doesn't specify which approach to take. Based on research, Approach 2 (Deck deduplication) is the best path forward. This paragraph gives clear direction.

3. **Implementation guidance**: Added concrete technical details (file paths, line numbers, patterns to follow) that weren't in the original issue. This helps contributors know where to start and sets expectations for scope.

**Why these labels**:

- `/area deck`: The implementation is in Deck's rerun handler (`cmd/deck/rerun.go`), not in Tide. Issue is currently labeled `area/tide` (probably because of confusion about components), but Deck is the correct component. Tide handles merge automation, not manual reruns.

- `/help-wanted`: Based on Level 2 effort assessment - this is a moderate-complexity, well-defined feature suitable for skilled contributors. Not `good-first-issue` because it requires understanding ProwJob lifecycle, Kubernetes client-go, and Deck's architecture.

- **No `/retitle` reconsideration**: Actually, on second thought, the retitle is warranted because:
  - `[feature]` prefix is non-standard (use kind label instead)
  - "Rerun queue" is misleading terminology
  - Missing component name (Deck)
  - Current title doesn't clearly state the desired behavior

**What's NOT included**:

- **Detailed triage findings**: All the research (4 solution approaches, root cause analysis, architecture diagrams) is valuable but too much for a GitHub comment. It's preserved in ISSUE-TRIAGE.md for contributors who want deep context.

- **Queue feature details**: Mentioned briefly but didn't elaborate on how it works, since it's not the recommended solution approach and would add complexity without value.

- **Priority label**: Not adding `/priority` because while this is a real issue that wastes resources, it's not critical-urgent. It's a nice-to-have efficiency improvement. Let maintainers decide on priority.

- **Effort assessment details**: Mentioned "moderate-complexity" and "2-4 files" but didn't include the full 8-factor analysis - that level of detail is in the triage doc for contributors who need it.

- **Multiple solution approaches**: Only included the recommended approach (Approach 2) to avoid confusion. Other approaches are documented in triage doc if needed.

## Next Steps

(Action items will be added here)

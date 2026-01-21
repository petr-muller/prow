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

## Next Steps

(Action items will be added here)

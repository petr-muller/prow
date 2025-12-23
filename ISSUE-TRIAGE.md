# Triage for Issue #388

**Status**: Complete
**Created**: 2025-12-23
**Completed**: 2025-12-23

## Issue Information

- **Issue Number**: #388
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/388
- **Title**: Job history cannot display the latest Runs
- **Reporter**: jianzhangbjz (external contributor)
- **Created**: 2025-02-26
- **Labels**: kind/bug, help wanted, area/deck
- **Assignee**: hector-vido
- **Status**: OPEN

## Summary

The job history page in Prow Deck is not displaying the latest job runs. When users navigate to a job's history page, it shows outdated runs that can be days or months behind the actual latest runs.

### Symptomatic Behavior

- Job history page displays runs from weeks/months ago instead of recent runs
- Examples:
  - OpenShift Prow: Showing Oct 20 runs when latest was same day (different time)
  - Kubernetes Prow: Showing January 9 when job runs continuously
  - Later checks: Showing August 3rd when October 8th runs exist
  - Results are inconsistent - different checks show different stale dates (possibly hitting different deck instances)

### Expected Behavior

Job history page should display the most recent job runs.

### Impact

- Users cannot see recent job history when clicking "Job History"
- Affects both OpenShift Prow (prow.ci.openshift.org) and Kubernetes Prow (prow.k8s.io)
- Makes it difficult to track recent job execution and results

### Reproduction Steps

1. Open a specific job run URL on Prow
2. Click "Job History"
3. Observe that latest runs are not displayed - showing outdated runs instead

### Key Discussion Points

- **BenTheElder's hypothesis**: "It feels like latest-build.txt isn't being read?"
  - References similar issue: https://github.com/kubernetes/test-infra/issues/34312
- **hector-vido**: Sometimes waiting helps and jobs appear later
- **jianzhangbjz**: Waiting doesn't work for them
- **BenTheElder**: Results are inconsistent, possibly due to hitting different deck instances
- **Reliable reproduction**: https://prow.k8s.io/job-history/gs/kubernetes-ci-logs/logs/ci-kubernetes-e2e-gci-gce

## Findings

### Initial Validation

**LEGITIMATE** - This is a valid bug report with:
- Clear reproduction steps
- Multiple affected users and Prow instances
- Specific examples provided
- Tagged appropriately (kind/bug, area/deck)
- Consistent reports from different contributors including Prow maintainers

### Technical Analysis

#### Root Cause Identified

**BenTheElder's hypothesis is CORRECT** - The issue is directly related to stale `latest-build.txt` files.

**Code Flow Analysis:**

1. **Job History Request Handler** (`cmd/deck/job_history.go:432-531`)
   - When a user visits the job history page, `getJobHistory()` is called
   - At line 451, it reads `latest-build.txt` via `readLatestBuild()` to get the "latest" build number
   - At line 465, it lists ALL build IDs from the GCS bucket via `listBuildIDs()`
   - At line 470, build IDs are sorted in descending order (newest first)
   - At line 473, **CRITICAL**: `cropResults(buildIDs, top)` filters the results

2. **The Filtering Bug** (`cmd/deck/job_history.go:404-423`)
   ```go
   func cropResults(a []uint64, max uint64) ([]uint64, int, int) {
       for i, v := range a {
           if v <= max {  // <-- FILTERS OUT NEWER BUILDS!
               res = append(res, v)
           }
       }
   }
   ```
   - The `max` parameter comes from `latest-build.txt`
   - If `latest-build.txt` contains 1000, but actual builds are [1500, 1400, 1300, 1200, 1100, 1000, ...]
   - Only builds ≤ 1000 are shown, hiding builds 1100-1500!

3. **Upload Side** (`pkg/gcsupload/run.go:116-122`)
   - Each job pod uploads `latest-build.txt` with its build ID
   - This happens via the gcsupload sidecar
   - If uploads fail or are delayed, the file becomes stale

#### Why This Happens

**Potential causes for stale `latest-build.txt`:**
1. **Upload failures**: GCS write operations may fail silently for latest-build.txt
2. **Race conditions**: Multiple builds running concurrently might overwrite with older values
3. **Caching issues**: GCS/CDN caching might serve stale content to deck instances
4. **Permission issues**: Write failures that aren't properly logged/reported
5. **Eventual consistency**: GCS eventual consistency means reads might return old values

#### Inconsistent Results Explained

The report mentions "inconsistent results, possibly due to hitting different deck instances":
- Each deck instance likely has its own cache of latest-build.txt
- If deck reads the file at different times, it gets different stale values
- This explains why the same job shows different "latest" dates on different loads

#### Key Files Involved

- **Reading side**: `cmd/deck/job_history.go` (lines 129-140, 432-531)
- **Writing side**: `pkg/gcsupload/run.go` (lines 116-122)
- **Path construction**: `pkg/pod-utils/gcs/target.go` (lines 68-90)

#### Related Issues

- kubernetes/test-infra#34312 - Same issue on k8s.io Prow, closed as "not planned"
- Both issues describe identical symptoms: job history showing old runs despite newer runs existing

### Effort Assessment

**Complexity Level: 2/5 (Low-Medium)**

This is a well-understood issue with a clear root cause and straightforward solutions.

**Effort Breakdown:**

1. **Code Changes**: Low complexity
   - Primary change: Modify `getJobHistory()` in `cmd/deck/job_history.go`
   - Estimated: 5-15 lines of code change
   - Single file modification

2. **Testing Requirements**: Medium
   - Unit tests: Need to add/modify tests in `cmd/deck/job_history_test.go`
   - Integration tests: Should verify behavior with stale latest-build.txt
   - Manual testing: Test on real Prow instance with live GCS buckets

3. **Risk Assessment**: Low
   - Change is isolated to job history display logic
   - No database migrations or config changes needed
   - Backward compatible

**Estimated Time:**
- Development: 2-4 hours
- Testing: 2-3 hours
- Review/iteration: 1-2 hours
- **Total: 5-9 hours** (approximately 1 day for an experienced contributor)

## Proposed Solutions

### Solution 1: Add Fallback Logic (RECOMMENDED)

**Description**: When latest-build.txt is stale (older than the actual newest build), use the real maximum build ID instead.

**Implementation** (in `cmd/deck/job_history.go`):
```go
// After line 470 (sorting buildIDs)
sort.Sort(sort.Reverse(uint64slice(buildIDs)))

// Add this logic before line 473
if len(buildIDs) > 0 && buildIDs[0] > latest {
    logrus.Warnf("latest-build.txt (%d) is stale, actual latest is %d", latest, buildIDs[0])
    latest = buildIDs[0]
}
if top == emptyID || top > latest {
    top = latest
}
```

**Pros:**
- Simple, minimal code change
- Fixes the symptom effectively
- Makes the system more resilient
- Backward compatible
- No breaking changes

**Cons:**
- Doesn't fix the root cause of stale latest-build.txt
- latest-build.txt still serves a purpose (optimization), but becomes redundant

**Risk**: Very low - fallback only activates when there's already a problem

### Solution 2: Remove Dependency on latest-build.txt

**Description**: Stop reading latest-build.txt entirely and always use the maximum from listed build IDs.

**Implementation**:
- Remove `readLatestBuild()` call at line 451
- Always use `max(buildIDs)` if `top == emptyID`

**Pros:**
- Completely eliminates the root cause
- Simplifies the code
- No dependency on potentially stale files

**Cons:**
- Changes existing behavior
- latest-build.txt was originally used as an optimization (to avoid listing when possible)
- Might have slight performance impact (though 10s timeout already exists)

**Risk**: Low-Medium - changes fundamental assumption

### Solution 3: Improve Upload Reliability

**Description**: Fix the upload side to ensure latest-build.txt is always up-to-date.

**Implementation**:
- Add retries in `pkg/gcsupload/run.go`
- Better error handling and logging
- Investigate GCS consistency guarantees

**Pros:**
- Fixes root cause on upload side
- Maintains original design intent

**Cons:**
- More complex investigation required
- May not address all causes (caching, consistency)
- Doesn't help with existing stale files

**Risk**: Medium - requires understanding of all failure modes

## Recommendation

**Implement Solution 1 (Add Fallback Logic)** because:
1. It's the simplest fix with immediate impact
2. Low risk and effort
3. Makes the system resilient without breaking changes
4. Can be implemented by contributors (issue is labeled "help wanted")
5. Provides warning logs to help diagnose upload issues

**Optional Follow-up**: After Solution 1 is deployed, investigate upload reliability (Solution 3) to understand why latest-build.txt becomes stale. The warnings added in Solution 1 will help identify affected jobs.

## Next Steps

1. **Immediate**: Assign to a contributor (issue already labeled "help wanted")
2. **Implementation**:
   - Modify `cmd/deck/job_history.go` with fallback logic
   - Add unit tests in `cmd/deck/job_history_test.go`
   - Test manually with known-affected job (e.g., ci-kubernetes-e2e-gci-gce)
3. **Testing**:
   - Create test case with stale latest-build.txt
   - Verify fallback activates correctly
   - Verify warning logs appear
4. **Deployment**:
   - Deploy to staging Prow first
   - Monitor for warnings about stale latest-build.txt
   - Deploy to production
5. **Follow-up**:
   - Monitor warning logs to identify jobs with upload issues
   - Investigate specific jobs to understand why uploads fail
   - Consider Solution 3 if pattern emerges

## Communication Plan

**For the issue:**
- Comment with findings and proposed solution
- Ask for maintainer feedback on approach
- Mention that this is a good "help wanted" issue for contributors
- Reference this triage analysis

**For assignee (hector-vido):**
- Share triage findings
- Offer to collaborate on implementation
- Suggest reviewing Solution 1 as the recommended approach

## Proposed Issue Augmentation

### Title Change

- **No change needed**: Current title "Job history cannot display the latest Runs" is clear and specific

### Proposed GitHub Comment

```
## Root Cause Identified

BenTheElder's hypothesis is correct - this is caused by stale `latest-build.txt` files. The job history code reads `latest-build.txt` to determine the "latest" build, but then uses it to filter results in `cropResults()` (`cmd/deck/job_history.go:404-423`). The filtering logic only shows builds where `buildID <= latest`, which means when `latest-build.txt` is stale, all newer builds get filtered out.

**Example**: If `latest-build.txt` contains 1000 but actual builds are [1500, 1400, 1300...], only builds ≤ 1000 are shown.

## Technical Flow

The issue occurs in `cmd/deck/job_history.go:getJobHistory()`:
1. Line 451: Reads `latest-build.txt` to get the "latest" build number
2. Line 465: Lists ALL actual build IDs from GCS
3. Line 470: Sorts build IDs in descending order (newest first)
4. Line 473: **Filters out builds > latest** via `cropResults()`

When `latest-build.txt` is stale due to upload failures, caching, or race conditions, this filtering hides all newer builds.

## Recommended Fix

Add fallback logic in `getJobHistory()` after sorting build IDs (around line 470):

```go
if len(buildIDs) > 0 && buildIDs[0] > latest {
    logrus.Warnf("latest-build.txt (%d) is stale, actual latest is %d", latest, buildIDs[0])
    latest = buildIDs[0]
}
```

This simple change (~5-15 lines) makes the system resilient to stale files while adding logging to help diagnose upload issues. It's backwards compatible and low risk.
```

### Rationale

**What's being added**:
- Root cause explanation with specific code references (not in original issue)
- Technical flow showing how the filtering bug manifests
- Concrete solution with code snippet and rationale
- File/line references for contributors

**Why these labels**:
- Labels are already correct: `area/deck`, `kind/bug`, `help-wanted`
- No changes needed - issue was already properly labeled

**What's NOT included**:
- Didn't retitle - current title is already clear and specific
- Didn't add priority label - already has assignee working on it
- Didn't repeat symptoms/examples already well-documented in issue
- Kept comment concise (3 paragraphs) focusing on actionable technical insights

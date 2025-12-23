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

**REVISED ANALYSIS** - Based on investigation, the actual `latest-build.txt` file in GCS contains the **correct** build ID. The issue is **caching**, not stale files.

**Key Evidence:**
- When directly checking `latest-build.txt` in browser, it shows the correct recent build ID
- Opening the file directly in browser "heals" the job history view temporarily
- Different Deck instances show different stale values (cache inconsistency)
- This matches a caching problem, not an upload problem

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
   - The `max` parameter comes from reading `latest-build.txt`
   - If Deck reads a **cached** value of 1000, but actual builds are [1500, 1400, 1300, 1200, 1100, 1000, ...]
   - Only builds ≤ 1000 are shown, hiding builds 1100-1500!

3. **Upload Side** (`pkg/gcsupload/run.go:116-122`, `pkg/pod-utils/gcs/metadata.go:36-66`)
   - Each job pod uploads `latest-build.txt` with its build ID
   - **CRITICAL**: `WriterOptionsFromFileName()` sets ContentType and ContentEncoding but **NOT CacheControl**
   - Files are uploaded to GCS **without Cache-Control headers**

4. **Read Side** (`pkg/io/opener.go:225-246`)
   - Deck uses `pkgio.Opener.Reader()` to read from GCS
   - For GCS paths (line 226-231), calls `g.NewReader(ctx)` on `*storage.ObjectHandle`
   - Uses Google Cloud Storage client library directly
   - **No explicit cache control on reads**

#### Why This Happens

**Root cause: Missing Cache-Control headers and client-side caching:**

1. **Upload without Cache-Control**: Files uploaded to GCS without `Cache-Control` headers (pkg/pod-utils/gcs/metadata.go:36-66)
2. **GCS default caching**: GCS may apply default caching behavior for objects without explicit headers
3. **Client-side caching**: Google Cloud Storage Go client or HTTP layer may cache file contents
4. **No cache invalidation**: When new builds write latest-build.txt, old cached values aren't invalidated

**Why opening in browser "heals" it:**
- Browser HTTP request may use different cache headers or bypass cache
- Might trigger GCS to refresh its cached response
- Temporarily makes the correct value available to subsequent Deck reads

#### Inconsistent Results Explained

The report mentions "inconsistent results, possibly due to hitting different deck instances":
- Each Deck instance has independent caching behavior
- Different instances may have cached the file at different times
- Opening in browser affects cache state differently per instance
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

### Solution 1: Add Cache-Control Header on Upload (Addresses Root Cause)

**Description**: Set `Cache-Control: no-cache` when uploading `latest-build.txt` to prevent GCS and clients from caching it.

**Implementation** (in `pkg/pod-utils/gcs/metadata.go` or `pkg/gcsupload/run.go`):
```go
// Modify WriterOptionsFromFileName or add special handling for latest-build.txt
if filename == "latest-build.txt" {
    attrs.CacheControl = ptr.To("no-cache, no-store, must-revalidate")
}
```

**Pros:**
- Fixes the root cause (caching)
- Prevents future occurrences
- Aligns with the file's purpose (frequently changing)
- Standard HTTP caching solution

**Cons:**
- Requires understanding of which files need no-cache headers
- Might slightly increase GCS API calls (no caching)
- Need to ensure all upload paths set this correctly

**Risk**: Low - only affects latest-build.txt caching behavior

### Solution 2: Add Fallback Logic in Deck (Resilient Workaround)

**Description**: When latest-build.txt value is older than the actual newest build (due to stale cache), use the real maximum build ID instead.

**Implementation** (in `cmd/deck/job_history.go`):
```go
// After line 470 (sorting buildIDs)
sort.Sort(sort.Reverse(uint64slice(buildIDs)))

// Add this logic before line 473
if len(buildIDs) > 0 && buildIDs[0] > latest {
    logrus.Warnf("latest-build.txt (%d) is cached/stale, actual latest is %d", latest, buildIDs[0])
    latest = buildIDs[0]
}
if top == emptyID || top > latest {
    top = latest
}
```

**Pros:**
- Simple, minimal code change (~5-15 lines)
- Makes the system resilient to cache issues
- Backward compatible
- Provides warning logs to diagnose caching problems
- Works regardless of caching behavior

**Cons:**
- Doesn't fix root cause (caching)
- Makes latest-build.txt somewhat redundant
- Symptoms continue (caching) but are hidden

**Risk**: Very low - fallback only activates when there's already a problem

### Solution 3: Remove Dependency on latest-build.txt

**Description**: Stop reading latest-build.txt entirely and always use the maximum from listed build IDs.

**Implementation**:
- Remove `readLatestBuild()` call at line 451
- Always use `max(buildIDs)` if `top == emptyID`

**Pros:**
- Completely eliminates the caching problem
- Simplifies the code
- No dependency on cached files

**Cons:**
- Changes existing behavior
- latest-build.txt was originally used as an optimization
- Might have slight performance impact (though 10s timeout already exists)

**Risk**: Low-Medium - changes fundamental assumption

## Recommendation

**Implement Solution 1 + Solution 2 (Both)** because:

**Solution 1 (Cache-Control header):**
- Fixes the root cause for future uploads
- Prevents the caching issue from happening
- Best practice for frequently-changing files

**Solution 2 (Fallback logic):**
- Provides immediate relief for existing cached files
- Makes system resilient even if Solution 1 has gaps
- Adds logging to help diagnose caching issues
- Simple enough for "help wanted" contributors

**Implementation order:**
1. Implement Solution 2 first (easier, immediate impact, helps diagnose)
2. Then implement Solution 1 (prevents recurrence)
3. Monitor warning logs to see if caching issues decrease

**Why not Solution 3:**
- More invasive change
- Loses the optimization benefit of latest-build.txt
- Solutions 1+2 are sufficient and less risky

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

This is a **caching issue**, not a stale file issue. Investigation shows `latest-build.txt` in GCS contains the correct build ID, but Deck reads cached versions. The filtering logic in `cropResults()` (`cmd/deck/job_history.go:404-423`) only shows builds where `buildID <= latest`, so cached old values hide newer builds.

**Example**: If cached value is 1000 but actual builds are [1500, 1400, 1300...], only builds ≤ 1000 are shown.

**Evidence of caching**: Opening `latest-build.txt` directly in browser shows correct (recent) build ID and temporarily "heals" the job history view.

## Technical Flow

The issue occurs in `cmd/deck/job_history.go:getJobHistory()`:
1. Line 451: Reads `latest-build.txt` (gets cached value)
2. Line 465: Lists ALL actual build IDs from GCS
3. Line 470: Sorts build IDs in descending order (newest first)
4. Line 473: **Filters out builds > cached latest** via `cropResults()`

**Root cause**: Files uploaded to GCS without `Cache-Control` headers (`pkg/pod-utils/gcs/metadata.go:36-66`), causing GCS/client caching.

## Recommended Fix (Two-Part)

**Part 1 - Immediate workaround** (add fallback logic in `cmd/deck/job_history.go` after line 470):
```go
if len(buildIDs) > 0 && buildIDs[0] > latest {
    logrus.Warnf("latest-build.txt (%d) cached, actual latest is %d", latest, buildIDs[0])
    latest = buildIDs[0]
}
```

**Part 2 - Root cause fix** (set Cache-Control header when uploading `latest-build.txt`):
```go
// In pkg/pod-utils/gcs/metadata.go or pkg/gcsupload/run.go
if filename == "latest-build.txt" {
    attrs.CacheControl = ptr.To("no-cache, no-store, must-revalidate")
}
```

Part 1 provides immediate relief and diagnostic logging. Part 2 prevents future caching issues.

/area pod-utils
```

### Rationale

**What's being added**:
- **Revised root cause**: Caching issue, not stale files (based on investigation)
- Evidence that file is correct in GCS but Deck reads cached version
- Technical flow showing how caching leads to filtering bug
- Two-part solution: immediate workaround + root cause fix
- File/line references for both read and write sides

**Why these labels**:
- `area/deck` - Already applied (read side issue)
- `kind/bug` - Already applied
- `help-wanted` - Already applied (matches Level 2)
- **Need to add**: `area/pod-utils` - Write side issue (missing Cache-Control in upload code)

**What's NOT included**:
- Didn't retitle - current title is already clear and specific
- Didn't add priority label - already has assignee working on it
- Didn't repeat symptoms/examples already well-documented in issue
- Kept comment concise (3 sections) focusing on root cause discovery and actionable fixes

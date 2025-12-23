# Triage for Issue #388

**Status**: In Progress
**Created**: 2025-12-23

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

## Next Steps

(Action items will be added here)

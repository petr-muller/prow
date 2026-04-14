# Triage for Issue #680

**Status**: In Progress
**Created**: 2026-04-14

## Issue Information

- **Issue Number**: #680
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/680

## Initial Validation

**Assessment**: LEGITIMATE (likely duplicate of #388)

### Analysis

The issue reports that Prow's job history page displays incorrect/stale timestamps for recent job runs. On page refresh, results vary — sometimes showing recent runs correctly, other times showing data from months ago (e.g., October when jobs ran minutes ago). This is reproducible across different jobs on the kubevirt Prow instance.

**Issue Category**: Bug

**Repository Scope Check**:
- Component mentioned: Deck job history page
- Exists in this repo: Yes
- Relevant code paths:
  - `cmd/deck/job_history.go` — backend handler fetching build history from GCS/S3
  - `cmd/deck/template/job-history.html` — HTML template rendering job history table
  - `cmd/deck/static/job-history/job-history.ts` — TypeScript frontend
  - `cmd/deck/job_history_test.go` — unit tests

**Information Completeness**:
- Sufficient detail provided: Yes
- Reproduction steps: Clear (navigate to any job history page, refresh multiple times)
- Environment: prow.ci.kubevirt.io
- Screenshot: Provided

**Duplicate Analysis**:
- Issue #388 reports the identical symptom: job history page showing stale/incorrect timestamps
- #388 was filed against prow.ci.openshift.org, #680 against prow.ci.kubevirt.io
- Both describe the same underlying bug manifesting across different Prow instances
- #388 is already labeled `area/deck`, `area/podutils/gcsupload`, `help wanted`
- A project maintainer (petr-muller) already commented that #680 appears to be the same as #388

### Recommendation

This is a legitimate bug report for the Deck component in this repository. However, it is very likely a duplicate of #388 which describes the same symptoms. The fact that it reproduces on multiple Prow instances (openshift, kubevirt) confirms it's a systemic bug in Prow code, not an instance-specific issue.

**Suggested Action**:
- Keep open and continue triage to confirm it's the same root cause as #388
- If confirmed duplicate, close #680 in favor of #388 (which already has labels and triage)
- The cross-instance reproduction in #680 adds value as evidence of a systemic bug

## Code Research

### Current Implementation

**Primary Components**:
- `cmd/deck/job_history.go` — Backend handler that fetches job build history from GCS/S3 buckets
- `cmd/deck/main.go:701` — HTTP handler registration at `/job-history/`
- `cmd/deck/template/job-history.html` — HTML template for rendering job history table
- `cmd/deck/static/job-history/job-history.ts` — TypeScript frontend for populating/styling the table

**Architecture Overview**:
The job history page works by listing build artifacts directly from cloud storage (GCS/S3). There is no caching layer — each HTTP request triggers fresh GCS listing calls. The flow is:

1. Parse the URL to extract storage provider, bucket, and job root path
2. Read `latest-build.txt` to determine the most recent build ID
3. List all build IDs from the bucket (with a **10-second timeout**)
4. Sort build IDs in descending order
5. Crop to the page of 20 results to display
6. Concurrently fetch `started.json` and `finished.json` for each build

**Key Code Paths**:
1. `getJobHistory()` — `job_history.go:433-531` — Main entry point, orchestrates the full flow
2. `listBuildIDs()` — `job_history.go:243-277` — Lists all build IDs from GCS/S3
3. `getBuildData()` — `job_history.go:348-402` — Reads metadata for a single build
4. `handleJobHistory()` — `main.go:915-948` — HTTP handler that calls `getJobHistory()`

**Data Flow**:
```
HTTP request → parseJobHistURL() → readLatestBuild() → listBuildIDs() [10s timeout]
→ sort descending → cropResults() → concurrent getBuildData() → render template
```

### Root Cause Analysis

**Primary Cause**: The 10-second timeout on `listBuildIDs()` produces **partial, non-deterministic results**.

At `job_history.go:463-466`:
```go
buildIDListCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
defer cancel()
buildIDs, err := bucket.listBuildIDs(buildIDListCtx, root)
if err != nil && !errors.Is(err, context.DeadlineExceeded) {
    return tmpl, fmt.Errorf("failed to get build ids: %w", err)
}
```

The code **intentionally swallows `DeadlineExceeded` errors** (line 467) and proceeds with whatever partial results were collected before the timeout fired. Since GCS object listing returns results in lexicographic order (not by recency), a timeout mid-listing returns an arbitrary prefix of build IDs sorted lexicographically — which translates to a random-looking subset when re-sorted numerically.

**Contributing Factors**:
1. **No caching**: Every page load re-lists the entire bucket prefix. For jobs with hundreds or thousands of builds, this can easily exceed 10 seconds.
2. **Non-deterministic GCS listing latency**: Network conditions and GCS server load vary between requests, so the timeout cuts off at different points each time.
3. **Lexicographic vs numeric ordering mismatch**: GCS returns objects in lexicographic order. Build IDs like `1221704015146913792` and `1254406011708510210` are lexicographically interleaved in ways that don't match their numeric/chronological ordering. A partial lexicographic listing produces an arbitrary subset of chronological builds.
4. **PR logs use symlink files**: For PR logs, `listBuildIDs` lists `*.txt` symlink files via `listAll()`, which may be even slower than directory listing.

**Reproduction Conditions**:
- Job must have enough builds that GCS listing takes >10 seconds
- More builds = more likely to hit the timeout = more inconsistent results
- Larger Prow instances with many jobs are more affected

**Confirmed duplicate of #388**: Both issues describe identical symptoms caused by this same timeout-based partial listing behavior. Different Prow instances (openshift, kubevirt) are affected because the bug is in the Prow code, not instance-specific.

### Related Code

**Dependencies**:
- `sigs.k8s.io/prow/pkg/io` — Storage abstraction layer (Opener, Iterator interfaces)
- `github.com/GoogleCloudPlatform/testgrid/metadata` — Started/Finished JSON schemas
- GCS/S3 bucket — External storage where build artifacts live

**Test Coverage**:
- `job_history_test.go` — Tests URL parsing, result cropping, link generation, and basic `getJobHistory()` with fake GCS
- `TestListBuildIDsReturnsResultsOnError` — Specifically tests that partial results are returned on error (validates the current buggy behavior as intentional)
- **Test Gap**: No test for timeout behavior or non-deterministic results from partial listings

### Proposed Solutions

#### Approach 1: Cache Build ID Listings

**Description**: Add a time-based cache for build ID listings per job. Cache the full list of build IDs for a configurable TTL (e.g., 1-5 minutes). Subsequent requests within the TTL window reuse the cached list instead of re-listing GCS.

**Pros**:
- Eliminates non-deterministic behavior within the cache window
- Dramatically reduces GCS API calls
- Faster page loads after initial cache population
- Pagination becomes consistent (same list for older/newer pages)

**Cons**:
- New builds won't appear until cache expires (acceptable for history page)
- Memory usage grows with number of jobs and build IDs cached
- Cache invalidation complexity
- Doesn't solve the initial population timeout issue

**Affected Components**:
- `cmd/deck/job_history.go` — Add caching layer around `listBuildIDs`
- Possibly `cmd/deck/main.go` — Cache initialization

**Complexity**: Medium
**Backwards Compatibility**: None — purely additive improvement

#### Approach 2: Paginated GCS Listing with Cursor

**Description**: Instead of listing all build IDs and then cropping, use GCS pagination to fetch only the needed page of results. For numeric build IDs (logs prefix), start listing from the known `latest-build.txt` value downward. Use GCS listing's `StartOffset`/`EndOffset` or prefix-based pagination.

**Pros**:
- Only fetches what's needed — no timeout required
- Consistent results every time
- Scales to any number of builds
- Lower GCS API costs

**Cons**:
- GCS listing is lexicographic, not numeric — complex to map to numeric pagination
- May not work well for PR logs (symlink `.txt` files)
- More complex implementation
- Total count of builds becomes expensive to compute

**Affected Components**:
- `cmd/deck/job_history.go` — Rewrite `listBuildIDs` and `getJobHistory` pagination logic

**Complexity**: High
**Backwards Compatibility**: Pagination links would change behavior

#### Approach 3: Increase Timeout + Add Deterministic Fallback

**Description**: Increase the listing timeout (e.g., 30-60s) and, if it still fires, ensure results are deterministic by caching the partial result for a short TTL so refreshes return the same data.

**Pros**:
- Minimal code change
- Fixes the symptom for most practical cases
- Fallback cache ensures consistency even on timeout

**Cons**:
- Doesn't address the fundamental scaling issue
- Slower page loads for jobs with many builds
- Still non-deterministic on first load if timeout fires

**Complexity**: Low
**Backwards Compatibility**: None

#### Recommendation

**Preferred Approach**: Approach 1 (Cache Build ID Listings)

This provides the best balance of fix quality, implementation complexity, and user experience. It directly addresses both the non-determinism (same cached list = same results on refresh) and performance (no repeated GCS listings). The cache TTL of 1-5 minutes is acceptable for a history page — users don't need sub-minute freshness for historical data.

Approach 2 would be the ideal long-term solution but is significantly more complex and may require rethinking the entire pagination model.

**Key Implementation Considerations**:
1. Use a sync.Map or mutex-protected map keyed by `(bucket, root)` for the cache
2. Cache both the sorted build ID list and the total count
3. Consider a background refresh goroutine to keep the cache warm
4. Add a cache-busting query parameter for users who need fresh data

**Testing Requirements**:
- Test that repeated requests return consistent results
- Test cache expiration behavior
- Test concurrent cache access
- Test behavior when cache is cold (first request)

## Next Steps

- Assess effort level for implementing the caching solution
- Close #680 as duplicate of #388, cross-referencing the analysis
- Add research findings to #388 to help future contributors

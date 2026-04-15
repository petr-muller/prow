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

### Key Observation: Lexicographic Ordering Explains the Symptom

GCS lists objects **lexicographically**. Prow build IDs are 19-digit Snowflake-like numbers (e.g., `1254406011708510210`), so lexicographic order = numeric order = chronological order. This means the listing always starts from the **oldest** builds. When the 10-second timeout fires mid-listing, it cuts off the **newest** builds — the ones users actually care about. This perfectly explains why users see October dates instead of today's runs.

Additionally, pagination is affected: each page load calls `listBuildIDs()` fresh. Even clicking "Older Runs" from a correct page can show wrong results if the timeout hits differently, because the `buildId` URL parameter only controls the crop window over whatever partial list was returned.

### Proposed Solutions

#### Approach 1: Start Listing from Latest Build (recommended, fixes root cause)

**Description**: Leverage the already-known latest build ID (from `latest-build.txt`, read at line 451) to start listing from the most recent builds rather than from the oldest. GCS `storage.Query` supports `StartOffset` — by setting it near the latest build ID, we skip the bulk of old builds and only list the most recent ones. This would complete well within the timeout for any practical number of recent builds.

**Implementation outline**:
1. The `Opener.Iterator` interface (`pkg/io/opener.go:94`) currently takes `(prefix, delimiter)` — extend it to accept a `StartOffset` option (or add a new method)
2. In `listBuildIDs()`, compute a `StartOffset` from the latest build ID (e.g., subtract a buffer to get the last few hundred builds)
3. Pass `StartOffset` through to the GCS `storage.Query` at `opener.go:474`
4. For S3 backend (gocloud/blob), use equivalent `ListOptions` parameters

**Challenges**:
- Requires modifying the `Opener.Iterator` interface (cross-cutting change in `pkg/io`)
- Build IDs aren't sequential (Snowflake IDs have gaps), so computing the right `StartOffset` requires a heuristic
- PR logs use symlink `.txt` files — same approach should work since filenames are `<buildID>.txt`
- Total build count becomes unknown (only listing a subset) — affects the "Showing X/Y results" display

**Pros**:
- Fixes the root cause: recent builds are always listed first
- Fast even on first load — no caching needed
- Pagination works correctly since relevant builds are always in the listing
- Scales to any number of total builds

**Cons**:
- Requires `Opener.Iterator` interface change (broader impact than just Deck)
- StartOffset heuristic needs tuning — too aggressive = miss some builds, too conservative = still timeout
- Loses total build count (cosmetic issue)

**Affected Components**:
- `pkg/io/opener.go` — Add `StartOffset` support to `Iterator`
- `cmd/deck/job_history.go` — Use latest build ID to compute `StartOffset` in `listBuildIDs`

**Complexity**: Medium
**Backwards Compatibility**: None — purely additive improvement

#### Approach 2: Incomplete Results Banner (complementary, improves UX)

**Description**: When `listBuildIDs()` times out, propagate this to the template and display a warning banner: "Results may be incomplete — not all builds could be listed in time." The code already detects `DeadlineExceeded` at line 467 — just propagate a `ResultsIncomplete bool` flag to `jobHistoryTemplate` and render a banner in `job-history.html`.

**Implementation outline**:
1. Add `ResultsIncomplete bool` to `jobHistoryTemplate` struct
2. Set it to `true` when `errors.Is(err, context.DeadlineExceeded)` at line 467
3. Add a conditional banner div in `job-history.html`

**Pros**:
- Very simple change (~10 lines)
- Sets correct user expectations immediately
- Good safety net even after fixing the root cause
- No architectural impact

**Cons**:
- Doesn't fix the underlying problem
- Users still see wrong data, just with a warning

**Affected Components**:
- `cmd/deck/job_history.go` — Add `ResultsIncomplete` field, set on timeout
- `cmd/deck/template/job-history.html` — Add conditional banner

**Complexity**: Very Low (Level 1)
**Backwards Compatibility**: None

#### Approach 3: Progressive Loading (long-term UX improvement)

**Description**: Instead of server-side rendering the full page, return an initial page with the latest build info (from `latest-build.txt`) immediately, then progressively load older builds via AJAX. The page would show a loading indicator while older builds are being fetched.

**Implementation outline**:
1. Add a new JSON API endpoint (e.g., `/job-history-api/...`) that returns build data
2. Initial page load shows the latest build immediately (from `latest-build.txt` + single `getBuildData` call)
3. Frontend JS fetches additional builds asynchronously and appends rows to the table
4. Could batch requests (e.g., fetch 5 builds at a time) for progressive rendering

**Pros**:
- Best user experience — page loads instantly with most relevant data
- Older builds appear progressively, no timeout issues
- Natural fit for modern web UX patterns

**Cons**:
- Significant frontend and backend changes
- New API endpoint to maintain
- More complex error handling (partial failures visible to user)
- Major refactor of current server-rendered approach

**Affected Components**:
- `cmd/deck/job_history.go` — New JSON API endpoint
- `cmd/deck/static/job-history/job-history.ts` — AJAX-based progressive loading
- `cmd/deck/template/job-history.html` — Loading indicators
- `cmd/deck/main.go` — Register new endpoint

**Complexity**: High
**Backwards Compatibility**: None (additive), but significant code churn

#### Recommendation

**Preferred Approach**: Approach 1 (Start Listing from Latest Build) + Approach 2 (Banner) as a complementary safety net.

Approach 1 directly fixes the root cause by ensuring the most relevant (recent) builds are always listed, regardless of total history size. The key insight is that we already know the latest build ID — we just need to use that knowledge to scope the GCS listing. Approach 2 adds a low-cost safety net for edge cases where even the scoped listing might timeout.

Approach 3 is the ideal long-term UX but is a much larger effort and should be considered separately.

**Key Implementation Considerations**:
1. The `Opener.Iterator` interface change needs careful design — consider adding an optional `ListOptions` struct rather than modifying the existing signature
2. The StartOffset heuristic could use the latest build ID minus a configurable buffer, with a generous default
3. The banner should be non-intrusive (e.g., a yellow info bar, not a blocking modal)
4. Both approaches 1 and 2 should be testable with the existing `fakestorage` test infrastructure

**Testing Requirements**:
- Test that listing with StartOffset returns only recent builds
- Test that the banner appears when listing times out
- Test pagination consistency with the scoped listing
- Test the StartOffset heuristic with various build ID patterns

## Effort Assessment

**Effort Level**: 2 - Moderate (help-needed)

### Summary

The recommended fix has two parts: (1) use `StartOffset` in GCS listing to scope to recent builds using knowledge from `latest-build.txt` (fixes root cause), and (2) add an incomplete-results banner as a safety net (simple UX improvement). Together, they require changes to the `Opener.Iterator` interface in `pkg/io` and the job history handler in `cmd/deck`, touching ~3-4 files with moderate complexity.

### Factor Analysis

#### Scope of Changes
- **Assessment**: Moderate
- **Details**: Changes span `pkg/io/opener.go` (Iterator interface + GCS query), `cmd/deck/job_history.go` (listing logic + template data), `cmd/deck/template/job-history.html` (banner), plus test updates. Estimated ~150-250 lines across 3-4 files.
- **Level Indication**: 2-3

#### Complexity
- **Assessment**: Moderate
- **Details**: The core change (adding `StartOffset` to GCS query) is straightforward, but requires: (a) designing a backwards-compatible interface extension for `Opener.Iterator`, (b) a heuristic for computing the right StartOffset from the latest build ID, (c) handling both GCS and S3 backends. The banner part is trivial.
- **Level Indication**: 2-3

#### Required Expertise
- **Assessment**: Moderate
- **Details**: Requires understanding of GCS listing APIs (`storage.Query.StartOffset`), the `pkg/io` abstraction layer, and how build IDs relate to lexicographic ordering. No deep Prow-specific architecture knowledge needed beyond Deck's job history handler.
- **Level Indication**: 2-3

#### Clarity and Certainty
- **Assessment**: Well-defined with minor uncertainty
- **Details**: Root cause is clearly identified. The StartOffset approach is sound, but the heuristic for computing the offset needs tuning (how far back from latest to start). Also needs investigation into whether S3's gocloud/blob supports equivalent offset parameters.
- **Level Indication**: 2

#### Testing Requirements
- **Assessment**: Moderate
- **Details**: Need to test that scoped listing returns recent builds, that the banner appears on timeout, and that pagination works with the scoped listing. Existing `fakestorage`-based test infrastructure can be extended.
- **Level Indication**: 2-3

#### Backwards Compatibility
- **Assessment**: Fully compatible
- **Details**: The `Iterator` interface change should be additive (e.g., optional `ListOptions` struct). The banner is purely additive. No behavior changes for existing callers.
- **Level Indication**: 1-2

#### Architectural Alignment
- **Assessment**: Good fit
- **Details**: Using GCS query parameters to scope listings is the intended way to optimize GCS access. Adding `StartOffset` support to the `Opener` interface is a natural extension. The banner follows existing template patterns.
- **Level Indication**: 1-2

#### External Dependencies
- **Assessment**: Well-supported
- **Details**: GCS `storage.Query.StartOffset` is a stable, documented API. S3/gocloud equivalent needs verification but should be available.
- **Level Indication**: 1-3

### Recommended Labels

- [x] `help-wanted`: Well-defined, moderate scope, suitable for skilled contributors
- [x] `area/deck`: Bug is in Deck's job history handler
- [x] `kind/bug`: Incorrect display behavior caused by code defect
- [ ] `good-first-issue`: Requires understanding of GCS APIs and the pkg/io abstraction, not ideal for first-timers

### Guidance for Contributors

- Suitable for contributors familiar with GCS APIs and Go interface design
- Should review:
  - `cmd/deck/job_history.go`: `getJobHistory()` and `listBuildIDs()` functions
  - `pkg/io/opener.go`: `Iterator()` method and GCS query construction
  - `cmd/deck/job_history_test.go`: Existing test patterns with `fakestorage`
  - GCS `storage.Query` documentation for `StartOffset` field
- Recommended approach:
  1. Extend `Opener.Iterator` to accept an optional `StartOffset` (consider a `ListOptions` struct for future extensibility)
  2. In `getJobHistory()`, after reading `latest-build.txt`, compute a `StartOffset` to scope the listing to recent builds
  3. Add `ResultsIncomplete bool` to template data, set on `DeadlineExceeded`, render as a warning banner
  4. Update tests for both the scoped listing and the banner

### Caveats and Considerations

- Since #680 is a duplicate of #388, the effort assessment applies to the fix tracked under #388
- The `area/podutils/gcsupload` label on #388 may be misleading — the bug is in Deck, not in gcsupload. The label was likely added because the issue mentions GCS, but the root cause is in Deck's listing logic.
- The banner (Approach 2) alone would be a Level 1 fix and could be shipped independently as a quick win

## Proposed Issue Augmentation

### Title Change

- **No change needed**: Current title "Job history page displays incorrect timestamps for recent job runs" is clear and specific, accurately describes the symptom.

### Proposed GitHub Comment

```
This is a duplicate of #388, which reports the same symptom on a different Prow instance (prow.ci.openshift.org). Thank you for confirming this affects multiple instances — it helps establish this is a systemic bug in Prow, not an instance-specific issue.

The root cause is in Deck's job history handler (`cmd/deck/job_history.go`). When loading the page, Deck lists all build IDs from cloud storage (GCS/S3) with a hard-coded **10-second timeout**. For jobs with many builds, this listing often exceeds the timeout. When it does, the code silently proceeds with whatever partial results were returned before the deadline. Since GCS lists objects in lexicographic order starting from the oldest, the timeout always cuts off the **newest** builds — the ones users actually want to see. Each request gets a different amount of data depending on network latency, causing the inconsistent display.

The fix involves two parts: (1) using knowledge of the latest build ID (already read from `latest-build.txt`) to scope the GCS listing to recent builds via `StartOffset`, so only relevant builds are fetched, and (2) adding an "incomplete results" banner as a safety net when listings still timeout. See #388 for tracking.

/kind bug
/area deck
```

### Rationale

**What's being added**:
- Root cause explanation: the original issue describes the symptom but not why it happens
- Key insight about lexicographic ordering: timeout cuts off newest builds, not random ones
- Technical pointer to the specific code location
- Solution direction (StartOffset + banner)
- Cross-reference to the primary issue (#388)

**Why these labels**:
- `/kind bug`: This is broken behavior, not a feature request
- `/area deck`: The bug is in Deck's job history handler, not in gcsupload or any other component

**What's NOT included**:
- `/help-wanted`: Not adding to the duplicate — the label already exists on #388
- No `/retitle`: Title is already clear and descriptive
- No `/close`: The maintainer will close it as duplicate after reviewing

## Next Steps

- Post the augmentation comment to #680
- Ensure #388 has the root cause analysis (consider augmenting #388 separately)
- Push triage branches

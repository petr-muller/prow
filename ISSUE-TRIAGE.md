# Triage for Issue #545

**Status**: In Progress
**Created**: 2025-12-23

## Issue Information

- **Issue Number**: #545
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/545

## Findings

## Initial Validation

**Assessment**: LEGITIMATE

### Analysis

This issue reports a frontend bug in Deck's Prow Status page where malformed ProwJob resources cause visual rendering problems. Specifically:

**Issue Category**: Bug in Prow Component (Deck)

**Repository Scope Check**:
- Component mentioned: Deck (Prow's web UI)
- Exists in this repo: Yes
- Relevant code paths: Deck frontend code (TypeScript/React components handling ProwJob data visualization)
- Root cause: ProwJob resource without `.status` field breaks job-bar rendering and creates empty state filter entries

**Information Completeness**:
- Sufficient detail provided: Yes
  - Screenshots showing good vs broken state
  - Maintainer investigation identified specific root cause
  - Technical context well-documented in comments
  - Solution approach outlined (defensive filtering)
- Missing information: None - issue is well-documented

**Why This Is Legitimate**:
1. Describes a real bug in Deck's handling of edge cases
2. Code robustness issue - frontend should gracefully handle malformed data
3. Reproducible problem with identified root cause
4. Belongs in this repository (Deck component)
5. Not a misconfiguration or user error
6. Clear technical solution path identified

### Recommendation

**Suggested Action**: Keep open and continue triage

This is a valid bug report that identifies a robustness issue in Deck's frontend. While the root cause is a malformed ProwJob (missing `.status`), the proper fix is to make Deck more defensive in handling such edge cases. The issue is already well-triaged by maintainers, labeled appropriately (kind/bug, area/deck, help wanted), and has an active assignee.

The issue benefits from further research to identify:
- Exact code location where filtering should be applied
- Complete set of edge cases to handle
- Testing strategy for validation

## Code Research

### Current Implementation

**Primary Components**:
- Prow Status Page: cmd/deck/static/prow/prow.ts - Main TypeScript file for the Prow Status dashboard
- ProwJob Type Definitions: cmd/deck/static/api/prow.ts - TypeScript interfaces for ProwJob data structures
- Job-Bar Rendering: prow.ts:796-824 - Renders the colored status bar showing job counts by state

**Architecture Overview**:
The Deck frontend loads ProwJob data into a global `allBuilds` variable (ProwJobList) and processes it client-side in TypeScript. The code iterates through all ProwJobs multiple times:
1. First pass in `optionsForRepo` (lines 49-86) to collect available filter options (repos, states, jobs, etc.)
2. Second pass in main `redraw` loop (lines 502-683) to filter and render the job table
3. Both passes use destructuring to extract ProwJob fields, including `status.state`

**Key Code Paths**:
1. Options collection: prow.ts:49-86 (`optionsForRepo`) - Builds filter dropdown options
   - Line 59-62: Destructures `build.status.state` with default `state = ""`
   - Line 66: Adds state to `opts.states` object (including empty strings)
2. Main rendering loop: prow.ts:502-683 - Filters and displays ProwJobs
   - Line 516-517: Destructures `build.status` fields with defaults
   - Line 577: Counts jobs by state in `jobCountMap`
3. State dropdown population: prow.ts:108-109 - Adds states to filter dropdown
   - Includes empty string states from malformed ProwJobs
4. Job-bar rendering: prow.ts:796-824 (`drawJobBar`) - Visualizes job counts
   - Line 797: Includes empty string ("") in states array
   - Line 804-805: Converts empty string to "unknown" for display
   - Line 799: Sorting assumes all states have counts (using `!` assertion)

**Data Flow**:
1. Backend serves ProwJobList as JSON → loaded into `allBuilds` global
2. `optionsForRepo` iterates all jobs → extracts states → populates filter dropdowns
3. User selects filters → triggers `redraw`
4. `redraw` iterates jobs → applies filters → counts by state → renders table + job-bar
5. Problem: If ProwJob missing `.status`, destructuring produces `state = ""`, which propagates through entire pipeline

### Related Code

**Type Definitions** (cmd/deck/static/api/prow.ts):
- Lines 44-50: `ProwJob` interface declares `status: ProwJobStatus` (NOT optional - no `?`)
- Lines 115-125: `ProwJobStatus` interface with `state?: ProwJobState` (optional field)
- Line 2: `ProwJobState` type includes `""` (empty string) as a valid state

**Dependencies**:
- The code assumes TypeScript interface contracts, but runtime data may not match
- No runtime validation that ProwJobs conform to expected schema
- Destructuring with defaults (`state = ""`) only works if parent object exists

**Similar Functionality**:
- Other Deck pages likely have similar ProwJob processing patterns
- Histogram rendering (prow.ts) also processes job states but may have similar issues

### Test Coverage

**Existing Tests**:
- Frontend tests: cmd/deck/static/prow/histogram_test.ts - Only tests histogram component
- Backend tests: cmd/deck/*_test.go - Tests Go handlers but not frontend JavaScript
- Coverage assessment: **Missing** - No tests for ProwJob rendering with malformed data

**Test Gaps**:
- No tests for handling ProwJobs without `.status` field
- No tests for handling ProwJobs with undefined/null state values
- No tests verifying state dropdown doesn't show empty items
- No tests for job-bar rendering edge cases

### Documentation Review

**Code Comments**:
- Line 803-805 in prow.ts: "If state is undefined or empty, treats it as unknown state."
  - This comment acknowledges the issue but handling is incomplete
  - Only fixes display in job-bar, doesn't prevent empty dropdown items

**Design Documentation**:
- Type definitions in api/prow.ts mirror Go structs but don't enforce runtime validation
- No documented strategy for handling malformed ProwJobs

**Known Limitations**:
- Frontend assumes all ProwJobs have valid structure matching TypeScript interfaces
- No defensive coding for edge cases where Kubernetes resources might be malformed

### Root Cause Analysis

**Primary Cause**:
Missing runtime validation for ProwJob data structure. The code uses TypeScript destructuring with default values, which assumes the parent object (`.status`) exists:

```typescript
// Line 59-62 in optionsForRepo:
const {
  status: {
    state = "",
  },
} = build;
```

When `build.status` is undefined (missing entirely), this destructuring either:
1. Throws an error (if strict), or
2. Assigns `state = undefined`, which then gets coerced to empty string

The empty string state ("") then propagates through:
- `opts.states[""] = true` (line 66)
- State dropdown gets empty option (line 109, 402-408)
- Job count map includes empty state (line 577)
- Job-bar tries to render "unknown" but dropdown already broken

**Contributing Factors**:
1. **No input validation**: Backend serves ProwJobs without validating `.status` exists
2. **TypeScript interface mismatch**: Interface says `status` required, but runtime data may not comply
3. **Destructuring assumes structure**: Default values only work for missing properties, not missing objects
4. **Empty string is valid state**: `ProwJobState` type includes `""` (line 2 of api/prow.ts)
5. **Multiple passes over data**: Issue compounds as empty state added to filters, then used in rendering

**Reproduction Conditions**:
- Kubernetes ProwJob resource exists without `.status` field (manually created or validation bypassed)
- ProwJob appears in `/prowjobs.js` API response served to frontend
- Frontend attempts to render job in Prow Status page

### Proposed Solutions

#### Approach 1: Filter Malformed ProwJobs Early

**Description**: Add defensive filtering at the beginning of data processing to skip ProwJobs that don't have the required `.status` field.

**Implementation Points**:
- In `optionsForRepo` (line 49): Add check `if (!build.status) continue;` before destructuring
- In main `redraw` loop (line 502): Add same check before processing each ProwJob
- Optionally log warnings for malformed ProwJobs (console.warn)

**Pros**:
- Cleanest solution - prevents malformed data from propagating
- Minimal code changes (2-3 lines)
- Follows "fail fast" principle
- Prevents empty states in all UI elements (dropdown, job-bar, table)
- Easy to test and verify

**Cons**:
- Silently skips malformed ProwJobs (though this is arguably correct behavior)
- Doesn't fix the root cause (why malformed ProwJobs exist)

**Affected Components**:
- prow.ts: Add guards in two iteration loops

**Complexity**: Low (simple null check before processing)

**Backwards Compatibility**: High - doesn't break existing functionality, only makes it more robust

#### Approach 2: Safe Destructuring with Explicit Checks

**Description**: Instead of relying on destructuring defaults, explicitly check for status existence and provide fallback values.

**Implementation Pattern**:
```typescript
const state = build.status?.state || "";
const startTime = build.status?.startTime || "";
// ... etc
```

**Pros**:
- More explicit about handling missing data
- Uses optional chaining (modern TypeScript feature)
- Still processes jobs but with safe defaults

**Cons**:
- Still allows empty state to propagate
- More code changes required (every status field access)
- Doesn't truly solve the problem (empty dropdown still possible)

**Complexity**: Low-Medium (more changes but straightforward)

**Backwards Compatibility**: High

#### Approach 3: Filter Empty States from UI Elements

**Description**: Allow empty states in processing but filter them when populating UI elements (dropdown, job-bar).

**Implementation Points**:
- Line 108: Filter empty strings when building state options: `const ss = Object.keys(opts.states).filter(s => s).sort();`
- Line 797: Remove empty string from states array in `drawJobBar`

**Pros**:
- Minimal changes
- Fixes visible symptoms
- Keeps job counts accurate

**Cons**:
- Doesn't prevent processing malformed data
- Empty state still counted in statistics
- Less robust than filtering at source
- Band-aid solution rather than proper fix

**Complexity**: Low

**Backwards Compatibility**: High

#### Recommendation

**Preferred Approach**: **Approach 1 (Filter Malformed ProwJobs Early)**

This is the most robust solution that addresses the root cause. By filtering out malformed ProwJobs before any processing, we:
1. Prevent bad data from affecting UI elements
2. Keep the code simple and maintainable
3. Make the system more resilient to data quality issues
4. Align with defensive programming best practices

**Key Implementation Considerations**:
1. Add `if (!build.status) continue;` guard at two locations:
   - In `optionsForRepo` function before line 59
   - In main `redraw` loop before line 516
2. Consider adding console warning for visibility:
   ```typescript
   if (!build.status) {
     console.warn(`Skipping ProwJob without status: ${build.metadata?.name || 'unknown'}`);
     continue;
   }
   ```
3. Ensure the guard is placed before destructuring to avoid errors

**Testing Requirements**:
- Test with ProwJob missing entire `.status` object
- Test with ProwJob where `status.state` is undefined
- Test with ProwJob where `status.state` is null
- Verify state dropdown doesn't show empty items
- Verify job-bar renders correctly without empty states
- Test that job counts are accurate (malformed jobs not counted)

**Migration/Rollout Strategy**:
- No migration needed - frontend change only
- No API changes required
- Compatible with existing backend
- Should be safe to deploy immediately
- Monitor console for warnings about malformed ProwJobs to identify cleanup opportunities

### Issue Summary
- **Title**: An unknown state job break job-bar on "Prow Status" page
- **Reporter**: liangxia
- **Created**: 2025-11-12
- **Status**: OPEN
- **Labels**: kind/bug, help wanted, area/deck
- **Assigned**: Qqkyu (assigned on 2025-12-23)

### Problem Description
The Deck UI displays an "unknown state" job on the Prow Status page, which breaks the visual display of the job-bar component. The reporter provided screenshots showing:
1. A normal/good page display
2. A broken page with an unknown state job

### Root Cause Analysis
Investigation revealed the culprit was a ProwJob resource (likely manually created) that was missing its `.status` field entirely. This caused:
- An empty item in the state filter dropdown in Deck
- Broken visual rendering on the job-bar

### Technical Context
- **Component**: Deck (Prow's frontend)
- **Issue Type**: Frontend robustness - handling malformed/incomplete ProwJob resources
- **Suggested Fix**: Filter out ProwJobs that don't have `.status` or are missing expected struct fields before using them to compute visuals

### Discussion Timeline
1. Issue reported with screenshots (2025-11-12)
2. Maintainer investigation identified root cause (2025-12-02)
3. Labeled as "help wanted" for community contribution
4. Qqkyu expressed interest as potential first issue (2025-12-09)
5. Maintainer confirmed suitable for new contributor (2025-12-22)
6. Qqkyu assigned themselves (2025-12-23)

### Current State
- Issue is assigned and being worked on by Qqkyu
- This appears to be a good first issue for diving into Prow codebase
- Solution approach is clear: add defensive filtering in frontend code

## Next Steps

1. Monitor progress on the assigned issue
2. If needed, provide guidance on:
   - Location of relevant Deck frontend code
   - Where ProwJob filtering should be implemented
   - Testing approach for edge cases with malformed ProwJobs
3. Review any submitted PR for completeness

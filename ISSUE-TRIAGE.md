# Triage for Issue #541

**Status**: In Progress
**Created**: 2026-02-04

## Issue Information

- **Issue Number**: #541
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/541

## Findings

### Initial Validation

**Assessment**: LEGITIMATE

**Issue Category**: Feature Request

**Issue Summary**:
- Title: "Tide should force retest a suspiciously passing required job on mergeable PRs"
- Author: petr-muller (MEMBER)
- Created: 2025-10-31
- Current labels: area/tide, kind/feature

**Analysis**:

This issue requests a security/reliability enhancement to Tide's merge logic. It's a companion to issue #540, where status-reconciler went haywire and falsely retired job contexts.

**The Feature Request**:
Currently, Tide forces retests of required jobs when they were executed with a base ref different from the current HEAD. The proposed enhancement is to ALSO force retests when a passing job result is "suspiciously" green - meaning it doesn't come from:
1. An actual passing ProwJob, OR
2. An `/override` invocation

This would protect against bugs like #540 where a haywire component falsely marks jobs as passing.

**Repository Scope Check**:
- Component mentioned: Tide
- Exists in this repo: Yes
- Relevant code paths: pkg/tide/
- This is a core Prow component maintained in this repository

**Information Completeness**:
- Sufficient detail provided: Yes
- Use case clearly explained (protection against false passing statuses)
- Context provided (companion to #540)
- Current behavior acknowledged (author notes they haven't verified exact current Tide behavior)
- Missing information: None critical - implementation details will emerge during research

**Legitimacy Reasoning**:
1. Valid feature request for a component in this repository
2. Clear security/reliability motivation
3. Practical use case demonstrated by incident #540
4. Author is a project maintainer
5. Already properly labeled (area/tide, kind/feature)

### Recommendation

**Suggested Action**: Keep open and continue triage

This is a legitimate feature request that would improve Tide's robustness against upstream bugs or malicious status updates. The feature makes sense architecturally - Tide should verify that passing statuses are legitimate before allowing merges.

Next steps:
1. Research current Tide behavior regarding status validation
2. Identify code locations for implementation
3. Assess implementation effort

---

### Code Research

**Research completed**: 2026-02-04

#### Current Implementation

**Primary Components**:
- **Status Validation**: pkg/tide/tide.go:845-889 - `isPassingTests()` and `unsuccessfulContexts()` determine if a PR passes required checks
- **GitHub Context Fetching**: pkg/tide/github.go:321-392 - `headContexts()` fetches status checks from GitHub API
- **Merge Decision**: pkg/tide/tide.go:755-793 - `filterPR()` decides if a PR can enter the merge pool
- **Retest Logic**: pkg/tide/tide.go:1075-1135 - `accumulate()` determines which jobs need retesting
- **Source Verification**: pkg/tide/tide.go:1040-1073 - `prowJobsFromContexts()` validates contexts via baseSHA encoding

**Architecture Overview**:

Tide uses a two-phase approach for handling PRs:

1. **Filtering Phase** (`filterPR()` at line 755): Decides if a PR is eligible for the merge pool by checking if all required contexts have `State=SUCCESS`. This is correct behavior - PRs should enter the pool even with unverified passing contexts.

2. **Accumulation Phase** (`accumulate()` at line 1075): Determines which jobs need (re)testing by:
   - Building a list of real ProwJobs matching the current baseSHA
   - Synthesizing ProwJobs from context descriptions that contain baseSHA encoding
   - Marking jobs as "missing" if they lack a corresponding ProwJob
   - Triggering retests for missing jobs
   - **Gap**: Currently only checks for missing/failing jobs, doesn't validate already-passing contexts

**Key Code Paths**:

1. **Status check evaluation**: pkg/tide/tide.go:860-889 - `unsuccessfulContexts()` filters contexts where `ctx.State != githubql.StatusStateSuccess`
2. **Context fetching**: pkg/tide/github.go:333-392 - `headContexts()` retrieves all contexts from GitHub API without source verification
3. **PR filtering**: pkg/tide/tide.go:768-793 - Checks contexts but **only rejects if State != SUCCESS**
4. **BaseSHA verification**: pkg/config/config.go:3432-3446 - `BaseSHAFromContextDescription()` extracts baseSHA from context description
5. **ProwJob synthesis**: pkg/tide/tide.go:1040-1073 - Creates synthetic ProwJobs from contexts with valid baseSHA encoding
6. **Retest triggering**: pkg/tide/tide.go:1483-1526 - `takeAction()` triggers missing jobs

**Data Flow**:

```
1. Sync() [main loop]
   ├─> filterPR()
   │   ├─> headContexts() - fetches ALL contexts from GitHub
   │   ├─> unsuccessfulContexts() - filters by State == SUCCESS
   │   └─> Accepts PRs with passing contexts (correct - don't filter out)
   │
   ├─> accumulate()
   │   ├─> prowJobsFromContexts()
   │   │   └─> BaseSHAFromContextDescription() - validates source via baseSHA
   │   ├─> Filters to ProwJobs matching current baseSHA
   │   ├─> Marks jobs as missing if no matching ProwJob found
   │   └─> VULNERABILITY: Only checks for missing jobs, doesn't validate passing contexts
   │
   └─> takeAction()
       └─> Triggers retests for missing jobs
```

#### Related Code

**Dependencies**:
- **BaseSHA Encoding**: pkg/config/config.go:3414-3429 - `ContextDescriptionWithBaseSha()` encodes baseSHA into context descriptions when reporting results
- **BaseSHA Decoding**: pkg/config/config.go:3432-3446 - `BaseSHAFromContextDescription()` extracts baseSHA from descriptions for validation
- **ProwJob Querying**: pkg/tide/tide.go:1866-1909 - `dividePool()` queries ProwJobs by baseSHA/repo/branch index

**Callers**:
- **filterSubpools()**: pkg/tide/tide.go:665-690 - calls `filterPR()` for each PR to build the merge pool
- **syncSubpool()**: pkg/tide/tide.go:1721-1790 - calls `accumulate()` to determine which tests are needed

**Similar Functionality**:
- **Override detection**: pkg/tide/github.go - handles `/override` commands which legitimately bypass required checks
- **Status reporting**: pkg/tide/status.go - status-reconciler that reports results to GitHub (the component that caused issue #540)

#### Test Coverage

**Existing Tests**:
- **tide_test.go**: Extensive unit tests for Tide's core logic including `isPassingTests()`, `accumulate()`, and merge decisions
- **github_test.go**: Tests for GitHub provider including context fetching
- Coverage assessment: **Partial** - existing tests cover the mechanics but don't specifically test source verification scenarios

**Test Gaps**:
- No tests for "suspiciously passing" contexts (passing status without backing ProwJob)
- No tests for protection against malicious/buggy status updates
- No tests verifying behavior when contexts lack baseSHA encoding
- No tests for the scenario described in #540 (haywire status-reconciler)

#### Documentation Review

**Code Comments**:
- pkg/tide/tide.go:845-858 - Comments explain that Tide assumes failing if it can't get commit status
- pkg/config/config.go:3414-3429 - Comments explain baseSHA encoding format and 140-char limit
- pkg/tide/tide.go:1040-1073 - Comments explain synthesis of ProwJobs from context descriptions

**Design Documentation**:
- BaseSHA encoding is documented in code comments but not prominently featured
- The split between filtering (by Status) and accumulation (by ProwJob existence) is not explicitly documented

**Known Limitations**:
- Issue author notes they haven't verified current Tide behavior, suggesting this area may not be well-documented externally

#### Root Cause Analysis

**Primary Cause**:

**`accumulate()` doesn't verify that passing contexts come from legitimate sources.**

In `accumulate()` (pkg/tide/tide.go:1075-1135), Tide determines which jobs need retesting by checking for missing or failing jobs. However, it doesn't validate that already-passing contexts are backed by:
- Actual passing ProwJobs
- Valid `/override` invocations
- Or any legitimate source (baseSHA encoding)

The logic builds a list of ProwJobs matching the current baseSHA and marks jobs as "missing" if no matching ProwJob exists. **But this only applies to jobs that aren't already showing as passing** - it doesn't re-validate passing contexts.

**Contributing Factors**:

1. **Incomplete verification**: `accumulate()` only checks for missing jobs, not suspicious passing jobs
2. **Filtering is correct**: `filterPR()` correctly allows PRs with passing contexts into the pool - filtering them out would cause PRs to get stuck (nothing would ever reconsider them)
3. **BaseSHA encoding gaps**:
   - Normal Prow job results encode baseSHA in descriptions (via `config.ContextDescriptionWithBaseSha()`)
   - Override plugin does NOT encode baseSHA - uses `"Overridden by {user}"` description (pkg/plugins/override/override.go:521)
   - External status updates have no baseSHA encoding
4. **GitHub's API doesn't expose source**: GitHub's combined status API returns states but not provenance information
5. **Trust boundary**: Tide implicitly trusts GitHub's status API, which can be updated by any component with write access (like a buggy status-reconciler)

**Reproduction Conditions**:

For the vulnerability to manifest (as in #540):
1. A required context must show `State=SUCCESS` on GitHub
2. This context matches a required job for the PR
3. This status does NOT come from an actual passing ProwJob (no baseSHA encoding, no backing ProwJob)
4. The PR is otherwise merge-eligible (labels, approvals, etc.)
5. Tide's sync loop runs while this state exists

When these conditions are met, Tide will:
- Allow the PR into the merge pool (filterPR passes - correct)
- NOT mark the job for retesting (accumulate doesn't validate passing contexts - bug)
- May merge the PR based on unverified status

#### Proposed Solutions

#### Approach 1: Strict Source Verification During Filtering (INCORRECT)

**❌ This approach is architecturally flawed.**

Filtering PRs out of the merge pool based on unverified contexts would cause PRs to get stuck indefinitely:
- Tide would never reconsider them
- Nothing would update the suspicious context
- PR would be permanently excluded from consideration

**The filtering phase should remain as-is** - it correctly allows PRs with passing contexts into the pool.

#### Recommended Solution: Two-Part Fix

**This is the cleanest, most architecturally consistent approach.**

#### Part 1: Fix Override Plugin to Encode BaseSHA (Prerequisite)

**Current problem**: Override plugin creates statuses with description `"Overridden by {user}"` (pkg/plugins/override/override.go:521), lacking baseSHA encoding that normal Prow results have.

**Fix**:
- Line 521: Change `status.Description = description(user)`
- To: `status.Description = config.ContextDescriptionWithBaseSha(description(user), baseSHA)`
- BaseSHA is already available (line 495) and used for ProwJob creation (line 502)

**Benefit**: Override-created contexts become verifiable the same way as normal Prow contexts.

**Affected file**: pkg/plugins/override/override.go (1 line change)
**Complexity**: Trivial
**Backwards compatible**: Yes - only adds information to description

#### Part 2: Enhance Tide to Validate Passing Contexts

**Implementation**:

After Part 1 is deployed, enhance `accumulate()` to validate passing contexts for required jobs. The flow:
1. `filterPR()` allows PRs with passing contexts into the pool (unchanged)
2. `accumulate()` inspects passing contexts for required jobs (enhancement)
3. For each required context showing SUCCESS:
   - Check if baseSHA is encoded in description (`BaseSHAFromContextDescription()`)
   - OR check if backing ProwJob exists in the pool
4. If neither condition met, mark as "missing" to trigger retest
5. PRs remain in pool but can't merge until verified passing contexts exist

**Key insight**: After Part 1, the verification is simple:
- **Contexts with baseSHA in description**: Normal Prow results + overrides (after Part 1 fix)
- **Contexts with backing ProwJob**: All legitimate Prow results (normal + override)
- **No special cases needed**: No need to detect "Overridden by" pattern

**Pros**:
- **Architecturally consistent**: All legitimate contexts (Prow + override) use same verification pattern
- **Simple Tide logic**: Check baseSHA OR backing ProwJob - no special cases
- **Minimal disruption**: Doesn't break existing workflows
- **Leverages existing code**: Retest infrastructure already exists
- **Self-healing**: Unverified statuses get replaced with verified ones
- **Defense in depth**: Two independent verification methods

**Cons**:
- **Requires Part 1 first**: Override plugin fix must be deployed before Tide enhancement
- **Extra test runs**: Forces retests for unverified contexts (increased compute)
- **Small race window**: If PR is first in queue, might merge before retest completes

**Affected Components**:
- **Part 1**: pkg/plugins/override/override.go - 1 line change to encode baseSHA
- **Part 2**: pkg/tide/tide.go - `accumulate()` function enhancement (~50-100 LOC)

**Complexity**:
- Part 1: Trivial (Level 1 - single line change)
- Part 2: Low-Moderate (Level 2 - extends existing logic)

**Backwards Compatibility**:

**Fully backwards compatible** for both parts:
- Part 1: Only adds information to override descriptions
- Part 2: Only adds retests, doesn't block anything that currently merges

#### Implementation Rationale

**Why this is cleaner than special-casing overrides in Tide**:

Alternative considered: Check for "Overridden by" pattern in Tide
- ❌ Creates special case logic in Tide
- ❌ Tight coupling between override plugin and Tide
- ❌ Fragile (breaks if description format changes)
- ❌ Doesn't follow existing patterns

Recommended approach: Fix override to use baseSHA encoding
- ✅ Consistent with existing Prow patterns
- ✅ Single verification mechanism in Tide
- ✅ Loose coupling between components
- ✅ Robust and maintainable

**Why PRs must stay in pool (not filtered out)**:
- PRs would get stuck - Tide never reconsiders them
- Nothing would update the suspicious context
- PR permanently excluded from merge consideration

**Key Implementation Considerations**:

#### Part 1 Implementation (Override Plugin):

**File**: pkg/plugins/override/override.go

**Change at line 521**:
```go
// Before:
status.Description = description(user)  // "Overridden by {user}"

// After:
status.Description = config.ContextDescriptionWithBaseSha(description(user), baseSHA)
```

**Notes**:
- `baseSHA` is already available from line 495: `baseSHA, err := baseSHAGetter()`
- `config.ContextDescriptionWithBaseSha()` is already used by normal Prow reporting
- This makes override contexts verifiable the same way as normal Prow contexts

#### Part 2 Implementation (Tide):

**File**: pkg/tide/tide.go

**Enhance `accumulate()` function** (lines 1075-1135):
1. After line 1082 (`prowJobsFromContexts`), add validation for passing contexts
2. For each required context with `State=SUCCESS`:
   - Check if baseSHA is encoded in description via `BaseSHAFromContextDescription()`
   - OR check if backing ProwJob exists in `psStates` map
3. If neither condition met, add to `missingTests` even though status shows SUCCESS

**Add clear logging**:
- When marking a passing context as "missing due to lack of verification"
- Include context name, current state, and reason (no baseSHA, no ProwJob)

**Handle external CI**:
- Document that external CI systems should support baseSHA encoding
- Or provide alternative verification mechanism (e.g., webhook registration)

**Testing Requirements**:

#### Part 1 Tests (Override Plugin):
- Test: Override-created status includes baseSHA in description
- Test: Override-created ProwJob has correct baseSHA in Spec
- Test: BaseSHA survives description truncation (140 char limit)
- Test: Existing override functionality still works

#### Part 2 Tests (Tide):
- Test: Context with valid baseSHA in description → accepted (no retest)
- Test: Context with backing ProwJob → accepted (no retest)
- Test: Context with SUCCESS but no baseSHA and no ProwJob → marked as missing, retest triggered
- Test: Override-created context (after Part 1 fix) → accepted via baseSHA
- Test: External context without verification → forced retest
- Test: Retest produces verified result → PR can merge

**Migration/Rollout Strategy**:

1. **Phase 1**: Deploy Part 1 (Override Plugin Fix)
   - Simple 1-line change, low risk
   - Deploy to production
   - Verify override-created contexts now have baseSHA in descriptions
   - No behavior change to Tide yet

2. **Phase 2**: Deploy Part 2 with logging only (Tide Enhancement - Logging Mode)
   - Add verification logic to accumulate() but only log unverified contexts
   - Don't mark as missing yet
   - Gather data on how often unverified contexts occur
   - Identify any external CI systems that would be affected
   - Verify override contexts (from Part 1) are correctly identified as verified

3. **Phase 3**: Enable mandatory retest behavior (Tide Enhancement - Active Mode)
   - Change logging to actual marking as missing
   - Monitor for increased test volume
   - Watch for any PRs stuck in retest loops
   - Confirm system is working as expected

---

### Effort Assessment

**Effort Level**: 2 - Moderate (help-needed)

**Summary**:

Well-defined feature with clear two-part implementation approach. Overall moderate scope but can be split into two separate contributions: Part 1 is trivial (1-line change to override plugin), Part 2 is moderate (Tide enhancement). Solution builds on existing baseSHA verification infrastructure. Part 1 suitable for any contributor; Part 2 requires Tide familiarity.

#### Factor Analysis

##### Scope of Changes
- **Assessment**: Small for Part 1, Moderate for Part 2
- **Details**:
  - **Part 1** (Override Plugin):
    - pkg/plugins/override/override.go - 1 line change
    - pkg/plugins/override/override_test.go - add baseSHA verification test (~20 LOC)
    - Total: 1 file modified, ~21 lines
  - **Part 2** (Tide Enhancement):
    - pkg/tide/tide.go - enhance `accumulate()` function (~50-100 LOC)
    - pkg/tide/tide_test.go - add test cases for verification scenarios (~100 LOC)
    - Total: 2 files modified, ~150-200 lines
  - **Combined**: 3 files, estimated ~170-220 lines total
  - Can be split into two separate PRs (Part 1 first, Part 2 after Part 1 deploys)
- **Level Indication**: Part 1: Level 1, Part 2: Level 2-3, Combined: Level 2

##### Complexity
- **Assessment**: Simple for Part 1, Moderate for Part 2
- **Details**:
  - **Part 1** (Override Plugin):
    - Trivial code change: use existing `config.ContextDescriptionWithBaseSha()` function
    - BaseSHA already available in the function
    - No new logic, just using existing utility function
    - Simple test: verify description contains baseSHA
  - **Part 2** (Tide Enhancement):
    - Leverages existing baseSHA verification infrastructure (`BaseSHAFromContextDescription()`)
    - Extends existing ProwJob matching logic (already in `prowJobsFromContexts()`)
    - No special override detection needed (Part 1 makes overrides look like normal contexts)
    - No concurrency issues (accumulate runs in sync loop)
    - Main challenge: understanding the filtering vs accumulation split in Tide
    - Edge cases: external CI, baseSHA format variations
  - **Two-part approach simplifies Part 2**: No need for special override detection logic
- **Level Indication**: Part 1: Level 1 (simple), Part 2: Level 2 (moderate)

##### Required Expertise
- **Assessment**: Moderate
- **Details**:
  - Need to understand Tide's two-phase architecture (filtering → accumulation)
  - Need to understand baseSHA encoding mechanism (pkg/config/config.go)
  - Need to understand ProwJob data structures and matching
  - Should be familiar with Go testing patterns
  - Can learn from existing code - patterns are established
  - Don't need deep Kubernetes or GitHub API expertise
- **Level Indication**: 2-3

##### Clarity and Certainty
- **Assessment**: Very well-defined
- **Details**:
  - Root cause clearly identified in research
  - Two-part solution is well-specified with exact implementation details
  - Part 1: Exact line to change identified (override.go:521)
  - Part 2: Implementation location identified (accumulate function)
  - Existing infrastructure documented
  - All uncertainties resolved: no special override detection needed (Part 1 solves it)
  - Acceptance criteria clear from issue and research
  - Clear rollout strategy: Part 1 first, then Part 2
- **Level Indication**: 1 (very clear)

##### Testing Requirements
- **Assessment**: Moderate
- **Details**:
  - Need unit tests for new verification logic in accumulate()
  - Test scenarios identified in research:
    - Passing context without backing ProwJob → marked missing
    - Passing context with valid baseSHA → accepted
    - Passing context with override → accepted
    - Retest triggered for unverified context
  - Can follow existing test patterns in tide_test.go
  - Existing mock infrastructure can be reused
  - No complex integration test setup needed
- **Level Indication**: 2-3

##### Backwards Compatibility
- **Assessment**: Fully compatible
- **Details**:
  - No breaking changes to configuration or APIs
  - PRs that currently merge will still merge (after retest if needed)
  - Only adds retests, doesn't block anything permanently
  - May increase test volume/compute costs (operational impact, not compatibility)
  - No migration needed
  - Safe to roll out incrementally (Phase 1-3 plan exists)
- **Level Indication**: 1-2

##### Architectural Alignment
- **Assessment**: Good fit
- **Details**:
  - Builds directly on existing baseSHA verification infrastructure
  - Extends `accumulate()` naturally (already marks tests as missing)
  - Follows established pattern: "no backing ProwJob → missing test"
  - Aligns with Tide's security model (verify before merge)
  - Doesn't introduce new concepts or patterns
  - Research identified this as the architecturally clean approach
- **Level Indication**: 1-2

##### External Dependencies
- **Assessment**: None blocking
- **Details**:
  - Uses existing GitHub API (headContexts)
  - Uses existing ProwJob data structures
  - Uses existing baseSHA encoding (already in context descriptions)
  - Override mechanism exists (may need to wire it into accumulation)
  - No new external APIs or features required
- **Level Indication**: 1-3

#### Overall Assessment Rationale

Most factors indicate Level 2-3, with clarity and architectural alignment favoring the lower end. The key discriminators:

**Why not Level 1 (Easy)?**
- Requires understanding of Tide's internal architecture
- Moderate scope (2-4 files)
- Need to understand baseSHA mechanism and ProwJob matching
- Too involved for a new contributor

**Why Level 2 (Moderate)?**
- Solution approach is clear and well-researched
- Builds on existing patterns and infrastructure
- Well-defined scope and acceptance criteria
- Fully backwards compatible
- Suitable for a contributor with Prow familiarity
- No deep expertise required (not concurrent, not algorithmic)

**Why not Level 3 (Large)?**
- Scope is moderate, not large (2-4 files, not 10+)
- Complexity is moderate, not high (no race conditions, no fundamental algorithm changes)
- Architectural approach is clear (extends existing logic)
- Testing is straightforward (existing patterns)
- Research has already identified the solution

#### Recommended Labels

Based on this assessment, recommend the following labels:

- [x] **`help-wanted`**: Appropriate scope and complexity for skilled contributors familiar with Prow. Part 1 is simple but Part 2 requires understanding.
- [x] **`area/tide`**: Core Tide functionality (already applied)
- [x] **`area/plugins`**: Part 1 affects override plugin
- [x] **`kind/feature`**: Requesting new capability (already applied)
- [x] **`priority/important-soon`**: Security/reliability feature protecting against bugs like #540
- [x] **`good-first-issue`**: Part 1 (override plugin fix) is suitable for newcomers - single line change with clear pattern to follow
- [ ] **Apply both `good-first-issue` and `help-wanted`**: Part 1 is easy, Part 2 needs expertise

#### Guidance for Contributors

**This feature can be split into two separate contributions:**

#### Part 1: Override Plugin Fix (Level 1 - good-first-issue)

**Prerequisites**:
- Basic Go knowledge
- Familiarity with how to run tests

**Recommended preparation**:
1. Read pkg/plugins/override/override.go to understand the override command
2. Look at pkg/config/config.go:3414-3429 to see how `ContextDescriptionWithBaseSha()` works
3. Review issue #540 for context on why this is needed

**Implementation approach**:
1. Change line 521 in pkg/plugins/override/override.go
2. From: `status.Description = description(user)`
3. To: `status.Description = config.ContextDescriptionWithBaseSha(description(user), baseSHA)`
4. Add test in override_test.go to verify baseSHA appears in description
5. Submit PR with clear description linking to this issue

**Estimated effort**: 1-2 hours for a newcomer

#### Part 2: Tide Enhancement (Level 2 - help-wanted)

**Prerequisites**:
- Familiarity with Prow architecture, especially Tide
- Understanding of Go and testing patterns
- Part 1 must be deployed first (or include Part 1 in same PR)

**Recommended preparation**:
1. Review the complete research section in this triage document
2. Read through these key files:
   - pkg/tide/tide.go - focus on `accumulate()` (lines 1075-1135) and `prowJobsFromContexts()` (lines 1040-1073)
   - pkg/config/config.go - understand `BaseSHAFromContextDescription()` (lines 3432-3446)
   - pkg/tide/tide_test.go - review existing test patterns
3. Understand the two-part solution in research section
4. Review issue #540 for context on why this feature is needed

**Implementation approach**:
1. Start with logging-only mode: detect unverified contexts but don't mark as missing yet
2. Enhance `accumulate()` to validate passing contexts
3. For each passing required context, check: baseSHA in description OR backing ProwJob
4. Add comprehensive unit tests
5. After testing in production (logging mode), enable actual retest triggering
4. Add comprehensive unit tests
5. Test manually or with staging deployment

**Questions to resolve during implementation**:
- How to access override data during the accumulation phase?
- Should verification be a separate helper function or inline?
- What logging level for unverified contexts (info vs warning)?

**Support available**:
- This triage document provides detailed analysis
- Research section documents all relevant code paths
- Maintainers can provide guidance (consult before starting)
- Consider discussing approach in GitHub issue before implementation

**Estimated effort**:
- Part 1: 1-2 hours for a newcomer contributor
- Part 2: 2-3 days for an experienced Prow contributor (including testing and review cycles)
- Can be done by different contributors or same contributor in two separate PRs

#### Caveats and Considerations

1. **Two-part approach**: Part 1 (override plugin fix) should be deployed before Part 2 (Tide enhancement) for cleanest implementation. Alternatively, both can be in a single PR.

2. **Override plugin fix is prerequisite**: Without Part 1, Tide would need special logic to detect "Overridden by" pattern. Part 1 makes the solution architecturally cleaner.

3. **External CI systems**: Organizations using external CI (non-Prow) that doesn't support baseSHA encoding may see increased test runs. This is intentional (defense in depth) but should be documented.

4. **Test volume impact**: This will trigger more retests, increasing compute costs. Maintainers should monitor test volume after deployment.

5. **Phased rollout important**: The proposed 3-phase rollout (Part 1 deployment → Part 2 logging mode → Part 2 active mode) is critical for safe deployment and should not be skipped.

6. **This is the only architecturally sound approach**: Filtering PRs out would cause them to get stuck. The mandatory retest approach is both correct and straightforward to implement.

---

### Proposed Issue Augmentation

#### Title Change

- **No change needed**: Current title is clear, specific, and accurately describes the feature request

#### Proposed GitHub Comment

```markdown
## Root Cause and Current Behavior

Tide's `accumulate()` function (pkg/tide/tide.go:1075-1135) determines which jobs need retesting but doesn't verify that already-passing contexts come from legitimate sources. It checks for missing or failing jobs and triggers retests accordingly, but trusts passing contexts without validation - whether they come from actual passing ProwJobs, `/override` invocations, or external sources like the buggy status-reconciler in #540.

Additionally, the override plugin (pkg/plugins/override/override.go:521) creates status contexts with description `"Overridden by {user}"` instead of encoding baseSHA like normal Prow results do. This makes override contexts indistinguishable from suspicious external updates.

## Recommended Implementation: Two-Part Fix

**Part 1: Fix Override Plugin (Simple - good-first-issue)**
Change override.go:521 to encode baseSHA in status descriptions using `config.ContextDescriptionWithBaseSha()`. This makes override contexts verifiable the same way as normal Prow contexts. (1-line change + test)

**Part 2: Enhance Tide Validation (Moderate - help-wanted)**
Enhance `accumulate()` to inspect passing contexts for required jobs. For each required context showing SUCCESS, verify: (1) baseSHA encoded in description, OR (2) backing ProwJob exists. If neither, mark as "missing" to trigger retest. This is fully backwards compatible (PRs stay in pool, just get retested) and leverages existing patterns in `prowJobsFromContexts()`.

## Why Two Parts?

Without Part 1, Tide would need special logic to detect "Overridden by" pattern (fragile, tight coupling). Part 1 makes all legitimate contexts (Prow + override) follow the same verification pattern, keeping Tide's logic simple and maintainable.

## Implementation Notes

- Part 1: Trivial (1 file, 1 line) - suitable for newcomers
- Part 2: Moderate (2 files, ~150 LOC) - requires Tide knowledge
- Can be separate PRs or single PR
- Phased rollout: Part 1 → Part 2 logging → Part 2 active

/area tide
/area plugins
/kind feature
/good-first-issue
/help-wanted
/priority important-soon
```

#### Rationale

**What's being added**:
- **Root cause explanation**: The original issue mentions the desired behavior but not why the vulnerability exists. Added explanation that: (1) accumulate() doesn't validate passing contexts, only checks for missing/failing jobs, and (2) override plugin doesn't encode baseSHA, making override contexts look suspicious.
- **Two-part solution**: Explains that fixing override plugin first makes Tide's implementation cleaner and more maintainable (vs. special-casing "Overridden by" pattern).
- **Technical implementation details**: Specific code locations (override.go:521, accumulate, prowJobsFromContexts), file paths, and line numbers. Explains that solution builds on existing baseSHA verification.
- **Architectural rationale**: Why two parts is cleaner than special-casing, and why PRs must stay in pool (not filtered out).
- **Complexity split**: Part 1 is good-first-issue (trivial), Part 2 is help-wanted (moderate).

**Why these labels**:
- `/area tide`: Issue affects Tide's accumulation/retest logic (Part 2)
- `/area plugins`: Issue affects override plugin (Part 1)
- `/kind feature`: Security/reliability enhancement, not a bug fix
- `/good-first-issue`: Part 1 (override plugin fix) is 1-line change, suitable for newcomers
- `/help-wanted`: Part 2 (Tide enhancement) requires Prow familiarity, suitable for skilled contributors
- `/priority important-soon`: Protects against bugs like #540 (haywire components falsely marking jobs as passing)

**What's NOT included**:
- **Special-case override detection**: Comment doesn't mention detecting "Overridden by" pattern because Part 1 solves it cleanly
- **All technical details**: Research found extensive details; comment distills to essential information
- **/retitle**: Current title is clear and specific

## Next Steps

- [x] Initial validation completed - LEGITIMATE
- [x] Research Tide's current status validation logic
- [x] Assess implementation effort - Mixed (Part 1: Level 1, Part 2: Level 2)
- [x] Propose improvements to issue
- [x] Brief maintainer on findings
- [x] Wrap up triage (push branches, post comment)

---

### Comment Posted

**Posted on**: 2026-02-04
**Comment URL**: https://github.com/kubernetes-sigs/prow/issues/541#issuecomment-3846042195

**Labels applied via comment**:
- /area tide
- /area plugins
- /kind feature
- /help-wanted

**Note**: Removed /good-first-issue and /priority important-soon per maintainer feedback

---

### Briefing Completed

**Briefed maintainer on**: 2026-02-04

**Key discussion points**:
- Corrected initial analysis: vulnerability is in `accumulate()` not `filterPR()`
- PRs must stay in pool (filtering them out would cause them to get stuck)
- Discovered override plugin doesn't encode baseSHA in status descriptions
- Identified cleaner two-part solution: fix override plugin first, then enhance Tide
- Two-part approach avoids special-case logic in Tide for detecting override pattern

**Maintainer feedback**:
- Confirmed architectural understanding: PRs enter pool, then get retested (not filtered out)
- Suggested encoding baseSHA in override contexts for consistency
- Agreed this is cleaner than special-casing override detection in Tide

**Final approach**:
- Part 1: Fix override.go:521 to encode baseSHA (good-first-issue)
- Part 2: Enhance accumulate() to validate passing contexts (help-wanted)
- Both parts fully backwards compatible
- Can be separate PRs or combined

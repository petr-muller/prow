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
3. **BaseSHA encoding is optional**: While Tide encodes baseSHA in context descriptions it reports, it doesn't require this encoding when reading contexts
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

#### Approach 2: Mandatory Retest for Unverified Contexts (CORRECT)

**Description**:

Instead of blocking PRs with unverified contexts, force an immediate retest of any required context that passes but lacks proper source verification. The flow:
1. `filterPR()` allows PRs with passing contexts into the pool (current behavior)
2. `accumulate()` identifies unverified passing contexts (enhancement)
3. These are marked as "missing" even though they show SUCCESS
4. Tide immediately triggers retests for these jobs
5. PRs remain in pool but can't merge until verified passing contexts exist

**Implementation**:
- Keep `filterPR()` unchanged
- Enhance `accumulate()` to also check passing contexts (not just look for ProwJobs)
- Add logic: "if context.State == SUCCESS but no backing ProwJob → mark as missing"
- Existing retest infrastructure handles triggering

**Pros**:
- **Less invasive**: Doesn't change pool membership logic
- **Graceful degradation**: PRs stay in pool, just get retested
- **No configuration needed**: Works automatically for all PRs
- **Leverages existing retest logic**: Minimal new code
- **Self-healing**: Unverified statuses get replaced with verified ones

**Cons**:
- **Delayed protection**: PRs briefly eligible for merge before retest triggered
- **Extra test runs**: Forces retests even if status was legitimate
- **Cost**: Increased compute for redundant test runs
- **Race window**: If PR is at front of queue, might merge before retest completes

**Affected Components**:
- pkg/tide/tide.go - `accumulate()` function needs enhancement
- Logic to identify "suspiciously passing" contexts and mark as missing

**Complexity**: Low

**Backwards Compatibility**:

**Fully backwards compatible** - Only adds retests, doesn't block anything that currently merges

#### Recommendation

**This is the correct approach** - the only architecturally sound solution.

**Rationale**:

1. **Architecturally correct**: PRs must enter the pool (filtering) then get retested (accumulation), not filtered out
2. **Minimal disruption**: Doesn't break existing workflows or require configuration changes
3. **Addresses the security concern**: Prevents merges of PRs with unverified passing contexts
4. **Leverages existing code**: The retest infrastructure already exists and works well
5. **Simple implementation**: Primarily a change to `accumulate()` logic
6. **Self-documenting**: When Tide triggers a retest, logs will show "context lacks verification"

**Why filtering PRs out would be wrong**:
- PRs would get stuck - Tide never reconsiders them
- Nothing would update the suspicious context
- PR permanently excluded from merge consideration

**Key Implementation Considerations**:

1. **Modify `accumulate()` function** (pkg/tide/tide.go:1075-1135):
   - After line 1082 (prowJobsFromContexts), also check passing contexts
   - For each required context with State=SUCCESS:
     - Check if it has baseSHA encoding via `BaseSHAFromContextDescription()`
     - Check if it has backing ProwJob in `psStates` map
     - Check if override exists for this context
   - If none of these, add to `missingTests` even though status shows SUCCESS

2. **Add clear logging**:
   - When marking a passing context as "missing due to lack of verification"
   - Include context name, current state, and reason

3. **Preserve override behavior**:
   - Ensure `/override` invocations are properly detected and honored
   - May need to query override data during accumulation

4. **Handle external CI**:
   - Document that external CI systems should support baseSHA encoding
   - Or provide alternative verification mechanism (e.g., webhook registration)

**Testing Requirements**:

- Test: Context shows SUCCESS but no backing ProwJob → marked as missing
- Test: Context shows SUCCESS with valid baseSHA → accepted
- Test: Context shows SUCCESS with override → accepted
- Test: External context shows SUCCESS without verification → forced retest
- Test: Retest produces verified result → PR can merge

**Migration/Rollout Strategy**:

1. **Phase 1**: Deploy with comprehensive logging (but no behavior change)
   - Log instances of unverified passing contexts
   - Gather data on how often this occurs
   - Identify any external CI systems that would be affected

2. **Phase 2**: Enable mandatory retest behavior
   - Monitor for increased test volume
   - Watch for any PRs stuck in retest loops

3. **Phase 3** (future, if needed): Consider additional strictness options based on operational experience

---

### Effort Assessment

**Effort Level**: 2 - Moderate (help-needed)

**Summary**:

Well-defined feature with clear implementation approach. Moderate scope affecting 2-4 files (~150-250 LOC). Requires understanding of Tide's architecture and existing baseSHA verification patterns, but solution is straightforward and builds on existing infrastructure. Suitable for contributors with Prow familiarity.

#### Factor Analysis

##### Scope of Changes
- **Assessment**: Moderate
- **Details**:
  - Primary: pkg/tide/tide.go - enhance `accumulate()` function (~100 LOC)
  - Secondary: Override detection (if not currently accessible in accumulate) (~50 LOC)
  - Tests: pkg/tide/tide_test.go - add test cases for verification scenarios (~100 LOC)
  - Total: 2-4 files, estimated 150-250 lines modified/added
  - Localized to Tide component, doesn't spread across codebase
- **Level Indication**: 2-3

##### Complexity
- **Assessment**: Moderate
- **Details**:
  - Leverages existing baseSHA verification infrastructure (`BaseSHAFromContextDescription()`)
  - Extends existing ProwJob matching logic (already in `prowJobsFromContexts()`)
  - Need to add override detection capability
  - No concurrency issues (accumulate runs in sync loop)
  - Main challenge: understanding the filtering vs accumulation split in Tide
  - Edge cases: external CI, override handling, baseSHA format variations
- **Level Indication**: 2-3

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
- **Assessment**: Well-defined
- **Details**:
  - Root cause clearly identified in research
  - Solution approach (Approach 2) is well-specified
  - Implementation location identified (accumulate function)
  - Existing infrastructure documented
  - One minor uncertainty: how to access override data during accumulation
  - Acceptance criteria clear from issue and research
- **Level Indication**: 1-2

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

- [x] **`help-needed`**: Appropriate scope and complexity for skilled contributors familiar with Prow
- [x] **`area/tide`**: Core Tide functionality (already applied)
- [x] **`kind/feature`**: Requesting new capability (already applied)
- [x] **`priority/important-soon`**: Security/reliability feature protecting against bugs like #540
- [ ] **`good-first-issue`**: Too complex - requires understanding Tide architecture and baseSHA mechanism

#### Guidance for Contributors

**For Level 2 (Moderate) - help-needed**:

**Prerequisites**:
- Familiarity with Prow architecture, especially Tide
- Understanding of Go and basic testing patterns
- Comfortable reading and extending existing code

**Recommended preparation**:
1. Review the complete research section in this triage document
2. Read through these key files:
   - pkg/tide/tide.go - focus on `accumulate()` (lines 1075-1135) and `prowJobsFromContexts()` (lines 1040-1073)
   - pkg/config/config.go - understand `BaseSHAFromContextDescription()` (lines 3432-3446)
   - pkg/tide/tide_test.go - review existing test patterns
3. Understand the recommended solution (Approach 2 in research section)
4. Review issue #540 for context on why this feature is needed

**Implementation approach**:
1. Start by adding logging (Phase 1) to identify unverified passing contexts
2. Add helper function to verify context source (baseSHA, ProwJob, or override)
3. Enhance `accumulate()` to call verification and mark unverified as missing
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

**Estimated effort**: 2-4 days for an experienced Prow contributor (including testing and review cycles)

#### Caveats and Considerations

1. **Override detection complexity**: The research didn't fully explore how overrides are stored and accessed. Implementation may need to wire override data into the accumulation phase, which could add complexity.

2. **External CI systems**: Organizations using external CI (non-Prow) that doesn't support baseSHA encoding may see increased test runs. This is intentional (defense in depth) but should be documented.

3. **Test volume impact**: This will trigger more retests, increasing compute costs. Maintainers should monitor test volume after deployment and consider whether Phase 3 (opt-in strict mode) is needed.

4. **Phased rollout important**: The proposed 3-phase rollout (logging → retest → strict) is critical for safe deployment and should not be skipped.

5. **This is the only architecturally sound approach**: Filtering PRs out (initially considered as Approach 1) would cause them to get stuck. The mandatory retest approach is both correct and straightforward to implement.

---

### Proposed Issue Augmentation

#### Title Change

- **No change needed**: Current title is clear, specific, and accurately describes the feature request

#### Proposed GitHub Comment

```markdown
## Root Cause and Current Behavior

Tide's `accumulate()` function (pkg/tide/tide.go:1075-1135) determines which jobs need retesting but doesn't verify that already-passing contexts come from legitimate sources. It checks for missing or failing jobs and triggers retests accordingly, but trusts passing contexts without validation - whether they come from actual passing ProwJobs, `/override` invocations, or external sources like the buggy status-reconciler in #540. Source verification infrastructure exists (baseSHA encoding via `BaseSHAFromContextDescription()` in pkg/config/config.go and `prowJobsFromContexts()` at lines 1040-1073), but it's only used to identify missing jobs, not to validate suspicious passing jobs.

## Recommended Implementation

Enhance `accumulate()` to inspect passing contexts for required jobs. For each required context showing SUCCESS: verify (1) context description contains valid baseSHA encoding, (2) backing ProwJob exists in the pool, or (3) valid `/override` exists. If none match, mark as "missing" to trigger retest - treating unverified passing contexts like stale tests. This is fully backwards compatible (PRs still enter the pool via `filterPR()`, just get retested) and leverages existing verification patterns already in `prowJobsFromContexts()`.

## Implementation Notes

This is a moderate-complexity feature (Level 2) affecting 2-4 files with ~150-250 LOC. Builds on existing baseSHA verification patterns. Recommended phased rollout: (1) add logging to identify unverified contexts, (2) enable mandatory retests. Well-suited for contributors familiar with Tide's architecture.

/area tide
/kind feature
/help-wanted
/priority important-soon
```

#### Rationale

**What's being added**:
- **Root cause explanation**: The original issue mentions the desired behavior but not why the vulnerability exists. Added explanation that accumulate() doesn't validate passing contexts, only checks for missing/failing jobs. Verification infrastructure exists but isn't used to validate suspicious passing jobs.
- **Technical implementation details**: Specific code locations (accumulate, prowJobsFromContexts), file paths, and line numbers. Explains that solution builds on existing baseSHA verification and treats unverified contexts like stale tests.
- **Architectural clarity**: Explains that PRs correctly enter the pool (filterPR), then get retested (accumulate). Filtering them out would cause them to get stuck permanently.
- **Complexity assessment**: Level 2 effort, suitable for help-wanted. Provides scope estimate and notes about phased rollout.

**Why these labels**:
- `/area tide`: Already applied, but included for completeness. Issue affects Tide's accumulation/retest logic.
- `/kind feature`: Already applied. This is a security/reliability enhancement, not a bug fix.
- `/help-wanted`: Based on Level 2 effort assessment. Well-defined problem with clear solution, suitable for contributors familiar with Prow. Implementation approach is documented.
- `/priority important-soon`: This is a security/reliability feature that protects against bugs like #540 (haywire components falsely marking jobs as passing). Important for merge safety but not critical-urgent since workarounds exist (manual verification, fixing buggy components).

**What's NOT included**:
- **Incorrect filtering approach**: Initially considered filtering PRs out, but this is architecturally wrong (PRs get stuck). Comment focuses on the correct approach only.
- **All technical details**: Research found extensive details about data flow, architecture, test coverage, etc. Comment distills to essential information needed to understand and implement.
- **/retitle**: Current title is clear and specific. "Suspiciously passing" effectively conveys the concept.
- **good-first-issue**: This is Level 2 (moderate), requires Tide architecture knowledge. Not appropriate for newcomers despite clear solution.

## Next Steps

- [x] Initial validation completed - LEGITIMATE
- [x] Research Tide's current status validation logic
- [x] Assess implementation effort - Level 2 (Moderate)
- [x] Propose improvements to issue
- [ ] Brief maintainer on findings

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

1. **Filtering Phase** (`filterPR()` at line 755): Decides if a PR is eligible for the merge pool by checking if all required contexts have `State=SUCCESS`. **This phase does NOT verify the source of passing statuses.**

2. **Accumulation Phase** (`accumulate()` at line 1075): Determines which jobs need (re)testing by:
   - Building a list of real ProwJobs matching the current baseSHA
   - Synthesizing ProwJobs from context descriptions that contain baseSHA encoding
   - Marking jobs as "missing" if they lack a corresponding ProwJob
   - Triggering retests for missing jobs

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
   │   └─> VULNERABILITY: Accepts ANY passing status regardless of source
   │
   ├─> accumulate()
   │   ├─> prowJobsFromContexts()
   │   │   └─> BaseSHAFromContextDescription() - validates source via baseSHA
   │   ├─> Filters to ProwJobs matching current baseSHA
   │   └─> Marks jobs as missing if no matching ProwJob found
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

**Tide's filtering logic trusts ANY GitHub context with `State=SUCCESS`, regardless of source.**

In `filterPR()` (pkg/tide/tide.go:755-793), the decision to include a PR in the merge pool is based solely on whether required contexts have `State == githubql.StatusStateSuccess`. There is **no verification** that these passing statuses came from:
- Actual passing ProwJobs
- Valid `/override` invocations
- Or any legitimate source

The source verification logic exists in `prowJobsFromContexts()` and `accumulate()`, but these only affect **retest decisions**, not **merge pool filtering decisions**.

**Contributing Factors**:

1. **Architectural split**: Filtering happens before accumulation, so source verification (which happens during accumulation) doesn't affect pool membership
2. **BaseSHA encoding is optional**: While Tide encodes baseSHA in context descriptions it reports, it doesn't require this encoding when reading contexts
3. **GitHub's API doesn't expose source**: GitHub's combined status API returns states but not provenance information
4. **Trust boundary**: Tide implicitly trusts GitHub's status API, which can be updated by any component with write access (like a buggy status-reconciler)

**Reproduction Conditions**:

For the vulnerability to manifest (as in #540):
1. A required context must show `State=SUCCESS` on GitHub
2. This status must NOT come from an actual passing ProwJob (or be backed by baseSHA encoding)
3. The PR must otherwise be merge-eligible (labels, approvals, etc.)
4. Tide's sync loop must run while this state exists

When these conditions are met, Tide will:
- Allow the PR into the merge pool (filterPR passes)
- Eventually notice the missing ProwJob (during accumulate)
- But may have already merged if it was at the front of the queue

#### Proposed Solutions

#### Approach 1: Strict Source Verification During Filtering

**Description**:

Enhance `filterPR()` to perform the same source verification that `prowJobsFromContexts()` does. For each required context showing `State=SUCCESS`:
1. Check if the context description contains a valid baseSHA encoding
2. If not, query the ProwJob cache to verify a real ProwJob exists
3. Also check for valid `/override` invocations
4. Only accept the passing status if one of these conditions is met
5. Otherwise, treat the context as "suspiciously passing" and exclude the PR from the pool

**Implementation**:
- Modify `filterPR()` around line 768-790
- Add helper function `verifyContextSource()` that checks:
  - `BaseSHAFromContextDescription()` returns valid baseSHA matching current base
  - OR backing ProwJob exists in the pool
  - OR override exists for this context
- Filter out PRs with unverified passing contexts

**Pros**:
- **Addresses root cause directly**: Prevents unverified statuses from affecting merge decisions
- **Defense in depth**: Protects against bugs like #540 and potentially malicious status updates
- **Reuses existing infrastructure**: BaseSHA encoding already exists
- **Clear semantics**: "Suspicious" statuses are treated as unverified and blocked

**Cons**:
- **May block legitimate PRs**: If a context legitimately passes but lacks baseSHA encoding (e.g., external CI systems)
- **Increased complexity**: Filtering becomes more involved
- **ProwJob cache dependency**: Filtering now needs access to ProwJob data
- **Potential performance impact**: Additional queries/checks per PR per sync

**Affected Components**:
- pkg/tide/tide.go - `filterPR()` function needs enhancement
- May need to pass ProwJob cache data to filtering phase
- Override detection logic may need to be accessible during filtering

**Complexity**: Medium

**Backwards Compatibility**:

**POTENTIALLY BREAKING** - Could reject PRs that currently merge if:
- They use external CI systems that don't support baseSHA encoding
- They use override mechanisms that aren't properly tracked
- Migration path: Add a configuration option to enable/disable strict verification

#### Approach 2: Mandatory Baseling Retest for Unverified Contexts

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

#### Approach 3: Hybrid - Verify with Configurable Fallback

**Description**:

Combine aspects of both approaches with repository-level configuration:
- By default: Use Approach 2 (mandatory retest)
- Opt-in: Strict verification (Approach 1) via Tide config flag `strict_context_verification: true`

This allows:
- Conservative rollout to catch issues without breaking existing workflows
- High-security repositories can opt into strict mode
- Migration path from current behavior to fully verified behavior

**Implementation**:
- Add config field to Tide settings
- Implement both verification strategies
- Route based on config flag

**Pros**:
- **Flexible**: Repository owners choose their trade-off
- **Safe rollout**: Can deploy without breaking anyone
- **Clear migration path**: Mandatory retest → strict verification
- **Best of both**: Security when needed, compatibility when required

**Cons**:
- **More code**: Need to implement and maintain both strategies
- **Configuration complexity**: Another knob to document and support
- **Testing burden**: Need tests for both modes

**Complexity**: Medium-High

#### Recommendation

**Preferred Approach**: **Approach 2 (Mandatory Retest for Unverified Contexts)**

**Rationale**:

1. **Minimal disruption**: Doesn't break existing workflows or require configuration changes
2. **Addresses the security concern**: Prevents merges of PRs with unverified passing contexts (with small race window)
3. **Leverages existing code**: The retest infrastructure already exists and works well
4. **Simple implementation**: Primarily a change to `accumulate()` logic
5. **Self-documenting**: When Tide triggers a retest, logs will show "context lacks verification"

**The race window is acceptable because**:
- Tide typically processes PRs in order; unverified PR unlikely to be first
- The window is one sync interval (30s typical)
- Issue #540 suggests the buggy statuses persisted, so would be caught on next sync anyway

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

3. **Phase 3** (future): Consider Approach 1 (strict verification) as opt-in for high-security repos

## Next Steps

- [x] Initial validation completed - LEGITIMATE
- [x] Research Tide's current status validation logic
- [ ] Assess implementation effort
- [ ] Propose improvements to issue (if needed)

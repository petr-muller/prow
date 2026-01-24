# Triage for Issue #142

**Status**: In Progress
**Created**: 2026-01-24

## Issue Information

- **Issue Number**: #142
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/142

## Findings

### Initial Validation

**Assessment**: LEGITIMATE

**Issue Category**: Bug

**Analysis**:

This issue reports a bug in Prow's jenkins-operator where parallel/matrix Jenkins pipelines can report incorrect status to GitHub. The problem occurs when Jenkins reports a partial result after one branch of a matrix build completes, but before all branches finish. Prow incorrectly accepts this partial result as the final job status.

**Key Evidence**:
- Original report (May 2024) with concrete reproduction cases and links to affected Jenkins builds
- Multiple maintainers (tuminoid, lentzi90) confirmed the issue persists and kept it alive against stale bot
- Root cause identified by lentzi90 on Jan 23, 2026: pkg/jenkins/jenkins.go:125-132 doesn't check the `building` field
- Confirmed with API data showing `building: true` alongside `result: FAILURE`, proving Jenkins sets partial results mid-run
- PR #598 by lentzi90 appears to fix this issue

**Repository Scope Check**:
- Component mentioned: jenkins-operator
- Exists in this repo: Yes (pkg/jenkins/)
- Relevant code paths: pkg/jenkins/jenkins.go:125-132
- Already labeled: area/jenkins-operator, kind/bug

**Information Completeness**:
- Sufficient detail provided: Yes
- Reproduction steps: Yes (with links to Jenkins runs and test PRs)
- Expected vs actual behavior: Clear
- Root cause analysis: Yes (provided by maintainer lentzi90)
- Missing information: None

### Recommendation

**Keep open and continue triage.** This is a well-documented, confirmed bug in the jenkins-operator component with:
- Clear reproduction cases
- Identified root cause
- Active maintainer engagement
- Fix in progress (PR #598)

The issue is legitimate and should proceed to research and effort assessment phases.

### Code Research

#### Current Implementation

**Primary Components**:
- Jenkins Client: pkg/jenkins/jenkins.go - Interfaces with Jenkins API, provides Build struct and status checking methods
- Jenkins Controller: pkg/jenkins/controller.go - Orchestrates ProwJob synchronization with Jenkins builds
- Build struct: pkg/jenkins/jenkins.go:91-100 - Data model for Jenkins build information

**Architecture Overview**:

The jenkins-operator polls Jenkins API periodically to sync ProwJob states with Jenkins build states. The controller calls `ListBuilds()` which queries Jenkins for both enqueued and running builds. Status checking functions (`IsRunning()`, `IsSuccess()`, `IsFailure()`, `IsAborted()`) determine the current state of each build and drive ProwJob state transitions.

**Key Code Paths**:
1. Build status determination: pkg/jenkins/jenkins.go:125-142 - Four methods that check build status based on Result field
2. Build data retrieval: pkg/jenkins/jenkins.go:726 - API query using tree parameter to fetch build information
3. Pending job sync: pkg/jenkins/controller.go:360-388 - Uses status methods to determine if job is done and set ProwJob state
4. Triggered job sync: pkg/jenkins/controller.go:451-474 - Uses status methods during job startup

**Data Flow**:
1. Controller.Sync() runs periodically
2. Calls ListBuilds() to aggregate builds from Jenkins
3. GetBuilds() queries `/job/{name}/api/json?tree=builds[number,result,actions[...]]`
4. Jenkins returns array of builds with number, result, and actions
5. For each ProwJob, status methods determine state (Running/Success/Failure/Aborted)
6. Controller updates ProwJob state based on status determination
7. **BUG**: When Jenkins matrix build has partial results, `result` is set but job is still `building`
8. Current code only checks `result`, missing the `building` field entirely

**Critical Missing Field**: The `building` boolean from Jenkins API is not captured in Build struct or queried from Jenkins.

#### Related Code

**Build Struct** (pkg/jenkins/jenkins.go:91-100):
```go
type Build struct {
    Actions []Action `json:"actions"`
    Task    struct {
        Name string `json:"name"`
    } `json:"task"`
    Number   int     `json:"number"`
    Result   *string `json:"result"`
    enqueued bool    // Internal flag
    // MISSING: Building bool `json:"building"`
}
```

**Status Functions** (pkg/jenkins/jenkins.go:125-142):
- `IsRunning()`: Returns true only if `Result == nil && !enqueued` - **BUG: doesn't check building**
- `IsSuccess()`: Returns true if `Result == "SUCCESS"` - **Can be true while still building**
- `IsFailure()`: Returns true if `Result == "FAILURE" || Result == "UNSTABLE"`
- `IsAborted()`: Returns true if `Result == "ABORTED"`

**Controller Usage** (pkg/jenkins/controller.go:360-388):
The syncPendingJob function uses a switch statement with these status methods. When a matrix build returns `Result="SUCCESS"` but `building=true`, the current code incorrectly takes the `IsSuccess()` branch and marks the ProwJob as complete.

#### Test Coverage

**Existing Tests**:
- pkg/jenkins/jenkins_test.go: Tests basic build listing, parameter handling, job path generation
- pkg/jenkins/controller_test.go:372-598 (TestSyncPendingJobs): Has "building" test case but uses `Result=nil`, not the matrix scenario
- pkg/jenkins/controller_test.go:606-701 (TestBatch): Happy path with transitions enqueued → building → success

**Test Gaps**:
- **No tests for matrix builds with `Result != nil && building == true`**
- No tests for partial success scenarios in parallel jobs
- No unit tests for individual status methods (IsRunning, IsSuccess, etc.)

#### Root Cause Analysis

**Primary Cause**:

The Build struct does not capture the `building` field from Jenkins API, and the status checking logic does not account for Jenkins matrix builds that set partial results while still executing. The IsRunning() function incorrectly assumes that if `Result` is non-nil, the build has completed.

**Jenkins Matrix Build Behavior** (confirmed by maintainer lentzi90 with API data):

In matrix/parallel builds, Jenkins can report:
```json
{
  "building": true,
  "result": "FAILURE"  // or "SUCCESS"
}
```

This happens when one branch of the matrix completes before others. Jenkins sets the intermediate result based on completed branches, but the overall build is still running (`building: true`).

**Contributing Factors**:
1. API query doesn't request `building` field: tree parameter is `builds[number,result,actions[...]]`
2. Build struct doesn't include `building` field to capture it
3. IsRunning() only checks `Result == nil`, assumes non-nil Result means completion
4. Controller immediately acts on status results without validating build completion

**Reproduction Conditions**:
- Matrix/parallel Jenkins pipeline with multiple branches
- One branch completes (success or failure) before others
- Prow polls Jenkins during this window
- Result is set but building is true
- Prow incorrectly reports final status to GitHub

#### Proposed Solution (PR #598 Implementation)

**Description**:

Add `building` field to Build struct, include it in Jenkins API query, and update IsRunning() to check building status. This ensures Prow considers a build "running" if Jenkins indicates it's still building, regardless of partial result values.

**Changes Required**:

1. **Build Struct** (pkg/jenkins/jenkins.go:~99): Add `Building bool` field
2. **API Query** (pkg/jenkins/jenkins.go:726): Change tree parameter to include `building`:
   - Old: `builds[number,result,actions[parameters[name,value]]]`
   - New: `builds[number,result,building,actions[parameters[name,value]]]`
3. **IsRunning Logic** (pkg/jenkins/jenkins.go:127): Update to check building first:
   - Old: `return jb.Result == nil && !jb.enqueued`
   - New: `return jb.Building || (jb.Result == nil && !jb.enqueued)`
4. **Unit Tests** (pkg/jenkins/jenkins_test.go): Add 162 lines of tests covering:
   - IsRunning with building=true and various Result values
   - IsSuccess, IsFailure, IsAborted edge cases
   - Matrix build scenarios explicitly tested

**Pros**:
- Minimal code change (8 lines modified, 162 test lines added)
- Directly addresses root cause
- Aligned with Jenkins API contract
- Comprehensive test coverage for all status methods
- No backwards compatibility issues (building=false is default)
- Fixes false positives for both success and failure cases

**Cons**:
- None identified - this is a clean, focused fix

**Affected Components**:
- pkg/jenkins/jenkins.go: Build struct and status methods
- pkg/jenkins/jenkins_test.go: New unit tests
- Indirectly: controller.go behavior improves (no code changes needed)

**Complexity**: Low - Single-file change with clear logic

**Backwards Compatibility**:

No breaking changes. The `building` field defaults to false when not present (Go zero value), which preserves existing behavior for non-matrix builds. Matrix builds will now correctly report their status.

**Testing Requirements**:

PR #598 includes comprehensive unit tests:
- 7 test cases for IsRunning including critical "building true + result set" scenario
- 4 test cases for IsSuccess
- 4 test cases for IsFailure
- 3 test cases for IsAborted
- Total: 18 new test cases covering all edge cases

#### Recommendation

**Approve PR #598** - This fix is well-designed, minimal, and comprehensively tested. It directly addresses the root cause identified in the issue without introducing complexity or backwards compatibility concerns.

**Implementation Quality Assessment**:
- ✅ Root cause correctly identified
- ✅ Minimal invasive change
- ✅ Comprehensive test coverage
- ✅ Follows existing code patterns
- ✅ Good code comments explaining the matrix build edge case
- ✅ No migration or rollout concerns

## Effort Assessment

**Effort Level**: 1 - Easy (good-first-issue)

### Summary

Small, well-defined fix with clear solution and comprehensive test coverage. Requires understanding Jenkins matrix build behavior but the code change itself is minimal and straightforward. Perfect candidate for a new contributor with some guidance about Jenkins pipelines.

### Factor Analysis

#### Scope of Changes
- **Assessment**: Small
- **Details**: 2 files modified (jenkins.go + jenkins_test.go), 8 lines of production code changed, 162 lines of tests added. Single component affected (jenkins-operator).
- **Level Indication**: 1-2

#### Complexity
- **Assessment**: Simple
- **Details**: Add one boolean field, update one API query string, modify one conditional check. No complex algorithms, no concurrency issues, no intricate state management. The logic is straightforward: "if Jenkins says it's still building, consider it running."
- **Level Indication**: 1-2

#### Required Expertise
- **Assessment**: Minimal
- **Details**: Basic Go knowledge and understanding of Jenkins matrix/parallel builds. The concept can be explained easily: "Jenkins can set a partial result while still building when one matrix cell finishes before others." No deep Prow architecture knowledge required. New contributors can learn what they need from issue comments and the Jenkins API documentation.
- **Level Indication**: 1-2

#### Clarity and Certainty
- **Assessment**: Well-defined
- **Details**: Root cause clearly identified with API data evidence. Solution approach is unambiguous: check the `building` field. No trade-offs or alternative approaches to consider. PR #598 demonstrates the fix works as expected.
- **Level Indication**: 1-2

#### Testing Requirements
- **Assessment**: Simple
- **Details**: Unit tests following standard table-driven test pattern. PR #598 shows exactly how to test this with 18 test cases covering all status methods. No integration tests needed. Test scenarios are clear and reproducible.
- **Level Indication**: 1-2

#### Backwards Compatibility
- **Assessment**: Fully compatible
- **Details**: The `building` boolean field defaults to false (Go zero value), preserving existing behavior for non-matrix builds. No configuration changes needed. No deployment impact. Purely additive change that only affects the specific edge case of matrix builds with partial results.
- **Level Indication**: 1-2

#### Architectural Alignment
- **Assessment**: Perfect fit
- **Details**: Follows existing patterns exactly. Just adds a field to a struct and updates status checking logic. Aligns with Jenkins API contract by consuming a field that Jenkins already provides. No new patterns or abstractions introduced. Natural extension of existing code.
- **Level Indication**: 1-2

#### External Dependencies
- **Assessment**: Well-supported
- **Details**: Jenkins API already provides the `building` field. No external system changes needed. The field is standard in Jenkins API and documented. No API version constraints or compatibility concerns.
- **Level Indication**: 1-3

### Recommended Labels

- [x] `good-first-issue`: Small scope, well-defined, clear solution, good for learning jenkins-operator
- [x] `area/jenkins-operator`: Already applied, correct area label
- [x] `kind/bug`: Already applied, this is indeed a bug fix
- [ ] `help-needed`: This is simple enough for good-first-issue, not needed
- [ ] `priority/important-soon`: While the bug causes CI issues, it has existed for 20+ months and PR #598 is already addressing it, so priority labeling is less critical

### Guidance for Contributors

**For Level 1 (Easy)**:

**Prerequisites**:
- Basic Go programming knowledge
- Understanding of JSON struct tags and API interactions
- Familiarity with table-driven testing in Go
- High-level understanding of Jenkins matrix/parallel builds (can be learned from issue discussion)

**Learning Resources**:
- Issue #142 comments explain the problem with real API data examples
- PR #598 shows the complete solution with tests
- Jenkins API documentation for the `/api/json` endpoint and `building` field
- Existing test patterns in pkg/jenkins/jenkins_test.go

**Mentorship Available**: Yes - maintainers (lentzi90, tuminoid) are engaged with this issue and can provide guidance

**Implementation Steps** (if PR #598 didn't exist):
1. Read issue #142 to understand the matrix build edge case
2. Add `Building bool` field to Build struct in pkg/jenkins/jenkins.go
3. Update API query string to include `building` in tree parameter
4. Modify IsRunning() to check `jb.Building` first
5. Add unit tests for IsRunning, IsSuccess, IsFailure, IsAborted following table-driven pattern
6. Run tests locally to verify all pass
7. Submit PR with clear description referencing issue #142

**Why This Is Good First Issue**:
- Very localized change (only 2 files)
- Clear problem statement with evidence
- Unambiguous solution approach
- Excellent test coverage patterns to follow
- No architectural decisions needed
- No backwards compatibility concerns
- Maintainer engagement and support available

### Caveats and Considerations

**Note on PR #598**: This issue already has a PR that implements the fix (PR #598 by lentzi90). This effort assessment evaluates what level of effort the fix represents, not whether someone should implement it from scratch. For teaching purposes, this would be an excellent good-first-issue, but the actual fix is already in progress.

**Upper End of Level 1**: This is on the upper boundary of Level 1 because it requires understanding a specific domain concept (Jenkins matrix builds). However, the concept is well-explained in the issue, and the code change itself is trivial, keeping it firmly in Level 1 territory.

**If Assigning to New Contributor**: A maintainer should explain Jenkins matrix build behavior and why the `building` field matters. Once that concept is clear, the implementation is straightforward.

## Next Steps

1. ✅ Initial validation complete - Issue is LEGITIMATE
2. ✅ Research complete - Root cause and fix validated
3. ✅ Effort assessment complete - Level 1 (good-first-issue)
4. ⏭️ Augment: Improve issue documentation based on findings

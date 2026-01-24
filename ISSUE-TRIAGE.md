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

## Next Steps

1. ✅ Initial validation complete - Issue is LEGITIMATE
2. ✅ Research complete - Root cause and fix validated
3. ⏭️ Assess effort: Determine effort level and appropriate labels
4. ⏭️ Augment: Improve issue documentation based on findings

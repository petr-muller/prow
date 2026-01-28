# Triage for Issue #154

**Status**: In Progress
**Created**: 2026-01-29

## Issue Information

- **Issue Number**: #154
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/154

## Findings

### Initial Validation

**Assessment**: LEGITIMATE

**Analysis**:

This issue requests a new validation feature for the `checkconfig` tool to ensure job configurations include resource requests and limits for CPU and memory. The validation would be opt-in via strict mode.

**Issue Category**: Feature Request

**Repository Scope Check**:
- Component mentioned: checkconfig
- Exists in this repo: Yes (cmd/checkconfig/main.go:258)
- Relevant code paths:
  - cmd/checkconfig/main.go - Main validation logic
  - The tool already has strict mode flag (line 81, 221)
  - Validation pattern exists for other job requirements (e.g., validateJobRequirements at line 580)

**Information Completeness**:
- Sufficient detail provided: Yes
- Includes example code implementation
- Clear use case: Enforce resource limits in job configurations
- Maintainer feedback present: @petr-muller suggested making validation more granular (allow requests without limits)

**Key Discussion Points**:
1. Author proposes validating both requests AND limits (strict approach)
2. Maintainer feedback: Common pattern is to have requests for scheduling but not limits
3. Suggested approach: Make the validation granular - allow configurations with only requests
4. Integration point: Add to strict mode (already exists) or make it a separate warning flag

**Current Status**:
- Issue was auto-closed twice by stale bot (Nov 2024, May 2025)
- Author reopened in Dec 2024, showing continued interest
- Currently has lifecycle/rotten label
- No technical objections to the feature itself

### Recommendation

**Suggested Action**: Keep open and continue triage

This is a valid feature request for a Prow component maintained in this repository. The checkconfig tool is the appropriate place for this validation. The request aligns with Prow's goal of validating job configurations before deployment.

**Next Steps**:
1. Research existing validation patterns in checkconfig
2. Understand resource requirements best practices
3. Design a granular validation approach that accommodates different resource patterns
4. Assess implementation effort and complexity

### Code Research

**Current Implementation**

**Primary Components**:
- checkconfig main: cmd/checkconfig/main.go - Validates Prow configuration files
- validate() function: cmd/checkconfig/main.go:258-472 - Central validation dispatcher
- JobBase struct: pkg/config/jobs.go:104-154 - Base job structure containing Spec field

**Architecture Overview**:
checkconfig uses a modular validation pattern where each check is implemented as an independent function that takes config.JobConfig and returns errors. Validations are gated by a warning flag system that allows users to enable/disable specific checks via CLI flags. The strict mode flag converts warnings into fatal errors.

**Key Code Paths**:
1. Validation dispatcher: cmd/checkconfig/main.go:258-472 - Checks which warnings are enabled and calls corresponding validators
2. Warning registration: cmd/checkconfig/main.go:102-165 - Constants and lists defining available warnings
3. Job iteration patterns: cmd/checkconfig/main.go:1568-1600 - Example of iterating PresubmitsStatic, PostsubmitsStatic, Periodics
4. Resource access: pkg/config/jobs.go:129 - job.Spec *v1.PodSpec contains Containers array

**Data Flow**:
1. CLI flags parsed to determine enabled warnings
2. Config files loaded into config.JobConfig structure
3. For each enabled warning, corresponding validation function is called
4. Validators iterate over job types (presubmits, postsubmits, periodics)
5. Each job's Spec field (if present) contains Kubernetes PodSpec with Containers
6. Container.Resources.Requests and Container.Resources.Limits contain resource definitions
7. Validation errors aggregated and either logged (warning mode) or fatal (strict mode)

**Related Code**

**Dependencies**:
- utilerrors.NewAggregate() - Used to combine multiple validation errors
- v1.KubernetesAgent - Agent type constant for filtering Kubernetes jobs
- corev1.ResourceCPU, corev1.ResourceMemory - Resource type constants from Kubernetes API

**Existing Validators**:
- validateRequiredJobAnnotations: cmd/checkconfig/main.go:1568-1600 - Pattern for checking job fields
- validateDecoratedJobs: cmd/checkconfig/main.go:924-948 - Pattern for simple boolean checks
- validateJobCluster: cmd/checkconfig/main.go:1236-1250 - Pattern for validating job configuration fields

**Similar Functionality**:
- pod-utils/decorate/podspec.go - Real-world examples of accessing Container.Resources
- Checks pattern: `if _, ok := container.Resources.Requests[corev1.ResourceMemory]; ok`

**Test Coverage**

**Existing Tests**:
- cmd/checkconfig/main_test.go - Comprehensive test suite for all validators
- TestValidateRequiredJobAnnotations: lines 2537-2634 - Example test pattern for job validators
- TestValidateClusterField: lines 1752-1927 - Example of more complex validation testing

**Test Pattern**:
Tests use table-driven approach with structs containing:
- Test case name
- Input jobs (presubmits, postsubmits, periodics)
- Expected error state
- Expected validation parameters

**Test Gaps**:
- No existing tests for resource requirements (new feature)
- Test coverage needed for: requests only, limits only, both, neither, mixed containers

**Documentation Review**

**Code Comments**:
- Warning constants well-documented at lines 102-130
- validateJobRequirements has clear purpose: "Prow labels k8s resources with job names. Labels are capped at 63 chars."
- No specific documentation about resource validation best practices in checkconfig

**Design Documentation**:
- Warning system allows three tiers: default (always on), expensive (opt-in for performance), optional (explicitly enabled)
- Strict mode converts all warnings to errors for CI enforcement

**Known Limitations**:
- Agent filtering required: Only Kubernetes jobs use Spec field, Jenkins and other agents don't
- Default agent is "kubernetes", so empty agent string should be treated as Kubernetes

**Root Cause Analysis**

**This is a Feature Request, not a Bug**:
There is no bug to fix - this is a request to add new functionality that doesn't currently exist.

**Current Gap**:
checkconfig validates many aspects of job configuration (name length, annotations, decoration, cluster fields, etc.) but does NOT validate resource requirements. This means jobs without resource requests/limits can pass validation and cause cluster problems when deployed.

**Use Case**:
Organizations want to enforce resource requirements for:
1. **Scheduling efficiency**: Requests help Kubernetes scheduler make informed placement decisions
2. **Resource management**: Limits prevent jobs from consuming excessive cluster resources
3. **Cost control**: Prevent unbounded resource usage
4. **Cluster stability**: Avoid OOM kills and resource contention

**Contributing Factors**:
1. Resource validation is optional/hygiene rather than strictly required for Prow to function
2. Different deployment scenarios have different resource requirement needs
3. Some organizations only use requests (for scheduling) without limits (to avoid throttling)

**Proposed Solutions**

#### Approach 1: Granular Validation with Separate Flags

**Description**: Add three separate warning flags for different resource validation levels:
- `validate-resource-requests`: Requires CPU and memory requests
- `validate-resource-limits`: Requires CPU and memory limits
- `validate-resource-requirements`: Requires both requests AND limits

This allows users to choose their enforcement policy: requests-only, limits-only, or both.

**Pros**:
- Addresses maintainer feedback about granularity
- Accommodates different organizational policies
- Users can evolve from requests-only to full requirements over time
- Flexible for different deployment scenarios

**Cons**:
- More flags to maintain
- More complex documentation
- Three separate validation functions or complex conditional logic
- Potential confusion about which flag to use

**Affected Components**:
- cmd/checkconfig/main.go: Add 3 warning constants, 3 validation functions, 3 warning checks
- cmd/checkconfig/main_test.go: Add 3 test functions

**Complexity**: Medium

**Backwards Compatibility**: Fully backwards compatible - all new flags are opt-in

#### Approach 2: Single Configurable Validation Flag

**Description**: Add one warning flag `validate-resource-requirements` with a CLI option to configure what to check:
- `--resource-validation-mode=requests` - Check requests only
- `--resource-validation-mode=limits` - Check limits only
- `--resource-validation-mode=both` (default) - Check both

This provides granularity through configuration rather than separate flags.

**Pros**:
- Single flag to enable/disable feature
- Cleaner warning flag list
- Simpler maintenance (one validation function)
- Still provides needed granularity

**Cons**:
- Requires new CLI flag pattern (validation mode configuration)
- Less consistent with existing checkconfig patterns (other warnings don't have modes)
- Documentation needs to explain the mode options

**Affected Components**:
- cmd/checkconfig/main.go: Add 1 warning constant, 1 validation function, 1 mode flag, 1 warning check
- cmd/checkconfig/main_test.go: Add 1 test function with multiple test cases

**Complexity**: Medium

**Backwards Compatibility**: Fully backwards compatible - new flag is opt-in

#### Approach 3: Simple Requests-Only Validation

**Description**: Based on maintainer feedback that "it is quite common to have requests for efficient scheduling but not have limits," implement a single warning that validates ONLY resource requests (not limits).

Add one warning flag `validate-resource-requests` that ensures all Kubernetes jobs have CPU and memory requests defined, but doesn't check limits.

**Pros**:
- Simplest implementation
- Addresses the most common use case (scheduling efficiency)
- Consistent with maintainer's stated preference
- Follows existing checkconfig patterns exactly
- Least maintenance burden

**Cons**:
- Doesn't provide limits validation for organizations that want it
- Original issue author requested both requests and limits
- Can't evolve to check limits without adding another flag later

**Affected Components**:
- cmd/checkconfig/main.go: Add 1 warning constant, 1 validation function, 1 warning check
- cmd/checkconfig/main_test.go: Add 1 test function

**Complexity**: Low

**Backwards Compatibility**: Fully backwards compatible - new flag is opt-in

#### Approach 4: Layered Warnings (Requests as Default, Limits as Optional)

**Description**: Add two warning flags with different categorizations:
- `validate-resource-requests` - Added to defaultWarnings (always on)
- `validate-resource-limits` - Added to optionalWarnings (explicitly enabled)

This reflects the philosophy that requests are important for scheduling (default) while limits are optional policy enforcement.

**Pros**:
- Balances mandatory hygiene (requests) with optional policy (limits)
- Makes requests validation the "pit of success" default
- Still allows limits validation when needed
- Clearer separation of concerns

**Cons**:
- Two flags to maintain (though simpler than Approach 1)
- Making requests default might be too aggressive for existing deployments
- Would need to be optional warning initially to avoid breaking existing users

**Affected Components**:
- cmd/checkconfig/main.go: Add 2 warning constants, 2 validation functions, 2 warning checks
- cmd/checkconfig/main_test.go: Add 2 test functions

**Complexity**: Medium

**Backwards Compatibility**: If both are optional warnings, fully compatible. If requests is default, could break existing configs.

#### Recommendation

**Preferred Approach**: Approach 4 (Layered Warnings) with both as optional warnings initially

**Rationale**:
1. **Addresses maintainer feedback**: Separates requests from limits, acknowledging they serve different purposes
2. **Provides flexibility**: Users can enable just requests, just limits, or both
3. **Simpler than Approach 1**: Only 2 flags instead of 3
4. **More useful than Approach 3**: Doesn't abandon limits validation entirely
5. **Follows existing patterns**: Uses optionalWarnings pattern consistently
6. **Clear semantics**: Each flag has a single, clear purpose

**Key Implementation Considerations**:

1. **Agent Type Filtering**:
   - Only validate Kubernetes jobs: `if job.Agent == string(v1.KubernetesAgent) || job.Agent == ""`
   - Skip Jenkins, Tekton, and other agent types

2. **Validation Granularity**:
   - Check each container independently
   - Report which container index has missing resources
   - Check for both CPU and Memory (not just presence of Requests map)

3. **Error Messages**:
   - Format: `job 'job-name' (org/repo): container 0 is missing CPU resource requests`
   - Include job name, repo, container index, specific resource type

4. **Null Handling**:
   - Skip jobs with nil Spec (not Kubernetes jobs or using pod template)
   - Handle nil Requests/Limits maps gracefully

5. **Warning Registration**:
```go
const (
    validateResourceRequestsWarning = "validate-resource-requests"
    validateResourceLimitsWarning   = "validate-resource-limits"
)

var optionalWarnings = []string{
    // ... existing
    validateResourceRequestsWarning,
    validateResourceLimitsWarning,
}
```

6. **Validation Function Pattern**:
```go
func validateResourceRequests(c config.JobConfig) error {
    checkResources := func(job config.JobBase, jobType, repo string) error {
        if job.Agent != string(v1.KubernetesAgent) && job.Agent != "" {
            return nil // Only Kubernetes jobs
        }
        if job.Spec == nil || len(job.Spec.Containers) == 0 {
            return nil // No spec to validate
        }

        var errs []error
        for i, container := range job.Spec.Containers {
            if container.Resources.Requests == nil {
                errs = append(errs, fmt.Errorf(
                    "container %d has no resource requests", i))
                continue
            }
            if _, ok := container.Resources.Requests[corev1.ResourceCPU]; !ok {
                errs = append(errs, fmt.Errorf(
                    "container %d is missing CPU resource requests", i))
            }
            if _, ok := container.Resources.Requests[corev1.ResourceMemory]; !ok {
                errs = append(errs, fmt.Errorf(
                    "container %d is missing memory resource requests", i))
            }
        }
        return utilerrors.NewAggregate(errs)
    }

    var errs []error
    for repo, presubmits := range c.PresubmitsStatic {
        for _, presubmit := range presubmits {
            if err := checkResources(presubmit.JobBase, "presubmit", repo); err != nil {
                errs = append(errs, fmt.Errorf("job '%s' (%s): %w",
                    presubmit.Name, repo, err))
            }
        }
    }
    // ... repeat for postsubmits and periodics
    return utilerrors.NewAggregate(errs)
}
```

**Testing Requirements**:
- Test case: Job with no resources
- Test case: Job with requests only (should pass requests check, fail limits check)
- Test case: Job with limits only (should fail requests check, pass limits check)
- Test case: Job with both (should pass both checks)
- Test case: Job with partial resources (CPU but not memory)
- Test case: Non-Kubernetes job (should pass both checks - not validated)
- Test case: Job with no Spec (should pass both checks - not validated)
- Test case: Multiple containers with mixed resource configs

**Migration/Rollout Strategy**:
1. Start as optional warnings (user must explicitly enable)
2. Announce in release notes and documentation
3. Gather feedback on real-world usage patterns
4. Consider promoting requests validation to default warnings in future release if widely adopted
5. Limits validation likely stays optional long-term

## Effort Assessment

**Effort Level**: 2 - Moderate (help-needed)

### Summary

Adding resource validation to checkconfig follows well-established patterns and has a clear solution approach, but requires understanding the warning flag system, proper iteration over job types, and comprehensive test coverage. Suitable for contributors familiar with Go and willing to learn checkconfig patterns.

### Factor Analysis

#### Scope of Changes
- **Assessment**: Small to Moderate
- **Details**:
  - Primary file: cmd/checkconfig/main.go (~120 lines for 2 validators, 2 constants, 2 warning calls)
  - Test file: cmd/checkconfig/main_test.go (~80 lines for 2 test functions with 8 test cases each)
  - Total: 2 files, ~200 lines of code
  - Localized to checkconfig component
- **Level Indication**: 2

#### Complexity
- **Assessment**: Simple to Moderate
- **Details**:
  - Straightforward validation logic: check if Resources.Requests/Limits exist and contain CPU and Memory
  - Similar complexity to existing validateRequiredJobAnnotations (lines 1568-1600)
  - No concurrency, no race conditions, no complex algorithms
  - Main complexity: properly iterating over 3 job types (presubmits, postsubmits, periodics)
  - Need to handle nil Spec and nil Resources maps gracefully
- **Level Indication**: 1-2

#### Required Expertise
- **Assessment**: Minimal to Moderate
- **Details**:
  - **Required knowledge**:
    - Go basics (error handling, iteration, maps)
    - How to read and follow existing code patterns
    - Understanding of Kubernetes resource concepts (requests vs limits)
  - **Can be learned from codebase**:
    - checkconfig warning flag pattern (clear examples exist)
    - Job iteration pattern (multiple examples at lines 1568-1600, 924-948)
    - How to access job.Spec.Containers.Resources (examples in pod-utils)
  - **NOT required**:
    - Deep Prow architecture knowledge
    - Understanding of Tide, GitHub API, or complex subsystems
    - Concurrency or distributed systems expertise
- **Level Indication**: 1-2

#### Clarity and Certainty
- **Assessment**: Well-defined
- **Details**:
  - Problem clearly stated: validate resource requirements in job configs
  - Solution approach agreed upon: two separate warning flags (requests and limits)
  - Maintainer provided specific feedback: make it granular, allow requests without limits
  - Code research identified exact integration points and patterns to follow
  - No open questions about requirements or approach
- **Level Indication**: 1-2

#### Testing Requirements
- **Assessment**: Moderate
- **Details**:
  - Need 2 test functions (one per validator)
  - Each test function needs ~8 test cases:
    1. Job with no resources (should fail)
    2. Job with requests only (pass requests check, fail limits check)
    3. Job with limits only (fail requests check, pass limits check)
    4. Job with both (pass both checks)
    5. Job with partial resources - CPU only (should fail)
    6. Job with partial resources - Memory only (should fail)
    7. Non-Kubernetes job (should pass - not validated)
    8. Job with no Spec (should pass - not validated)
  - Total: ~16 test cases
  - Follow existing table-driven test pattern (clear examples exist)
  - No integration tests needed
  - No new test infrastructure required
- **Level Indication**: 2

#### Backwards Compatibility
- **Assessment**: Fully compatible
- **Details**:
  - Both warnings are optional (added to optionalWarnings list)
  - Users must explicitly enable via --warnings flag
  - No default behavior changes
  - No breaking changes to existing configs
  - Existing deployments completely unaffected unless they opt-in
  - No migration strategy needed
- **Level Indication**: 1-2

#### Architectural Alignment
- **Assessment**: Perfect fit
- **Details**:
  - Follows exact pattern established by existing validators:
    - validateRequiredJobAnnotations (for job iteration pattern)
    - validateDecoratedJobs (for simple checks)
    - Warning flag system (well-established pattern)
  - Uses existing optionalWarnings list (lines 159-165)
  - Integrates into validate() dispatcher function (pattern at lines 454-458)
  - No new architectural patterns introduced
  - No changes to core Prow functionality
  - Checkconfig is the correct place for this validation
- **Level Indication**: 1-2

#### External Dependencies
- **Assessment**: None (all dependencies already present)
- **Details**:
  - Uses corev1.ResourceCPU and corev1.ResourceMemory from Kubernetes core API
  - These types are already imported and used throughout Prow
  - No new external libraries needed
  - No external API calls
  - No dependency on GitHub API, Kubernetes API, or other external systems
- **Level Indication**: 1-3

### Recommended Labels

Based on this assessment, recommend the following labels:
- [x] `help-needed`: Moderate scope, suitable for contributors willing to learn checkconfig patterns
- [x] `kind/feature`: Adding new validation capability
- [x] `area/checkconfig`: Affects checkconfig component
- [ ] `good-first-issue`: Slightly too involved due to testing requirements and need to understand warning flag system; better suited for someone with some Go experience
- [ ] `priority/important-soon`: Nice-to-have feature but not critical
- [ ] `/remove-lifecycle rotten`: Should remove lifecycle labels when posting augmentation comment

### Guidance for Contributors

**For Level 2 (Moderate)**:

**Suitable for**: Contributors familiar with Go who want to learn Prow contribution patterns

**Prerequisites**:
- Comfortable with Go (iteration, maps, error handling, structs)
- Understanding of Kubernetes resource requests and limits concepts
- Ability to read and follow existing code patterns

**Should review before starting**:
1. **Existing validation patterns**:
   - cmd/checkconfig/main.go:1568-1600 - `validateRequiredJobAnnotations` (best example for this feature)
   - cmd/checkconfig/main.go:924-948 - `validateDecoratedJobs` (simple pattern)
   - cmd/checkconfig/main.go:102-165 - Warning flag system

2. **Job structure**:
   - pkg/config/jobs.go:104-154 - JobBase struct showing Spec field
   - Understand: Presubmit, Postsubmit, Periodic all embed JobBase

3. **Resource access examples**:
   - pkg/pod-utils/decorate/podspec.go - Real-world examples of accessing Container.Resources

4. **Test patterns**:
   - cmd/checkconfig/main_test.go:2537-2634 - `TestValidateRequiredJobAnnotations` (test pattern to follow)

**Recommended approach**:
1. Start by adding the two warning constants (lines 102+)
2. Add them to optionalWarnings list (lines 159-165)
3. Implement `validateResourceRequests` function following `validateRequiredJobAnnotations` pattern
4. Implement `validateResourceLimits` function (very similar to requests)
5. Add validation calls in `validate()` function (follow pattern at lines 454-458)
6. Write tests following the table-driven pattern
7. Run tests: `go test ./cmd/checkconfig/...`
8. Run checkconfig against test configs to verify

**Estimated time**: 4-8 hours for someone new to Prow, 2-4 hours for experienced Go developer

**Mentorship**: Maintainers can provide guidance on checkconfig patterns and review approach

### Caveats and Considerations

1. **Author's example code**: The issue includes example code, but the recommended approach differs:
   - Author proposed all validation in one function with strict mode flag
   - Recommended: two separate optional warnings (more flexible)
   - Contributor should follow the recommended approach, not the example

2. **Starting point**: While the code structure is straightforward, a contributor should:
   - Read existing validators first to understand the pattern
   - Not copy-paste the author's example code verbatim
   - Follow checkconfig's established conventions

3. **Not a good-first-issue because**:
   - Need to implement two related but separate functions
   - Testing requirements are moderate (16 test cases total)
   - Need to understand the warning flag system
   - Better suited for someone with at least basic Prow contribution experience

4. **Could become easier**: If maintainer feedback suggests simplifying to a single flag (requests only), this could be reduced to Level 1. Current assessment assumes the recommended two-flag approach.

## Next Steps

- Continue with augment subcommand
- Propose improvements to the issue based on research findings
- Prepare comment with solution recommendations and label suggestions
- Use wrapup subcommand to finalize triage

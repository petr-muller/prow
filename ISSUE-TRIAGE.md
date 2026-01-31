# Triage for Issue #502

**Status**: In Progress
**Created**: 2026-01-31

## Issue Information

- **Issue Number**: #502
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/502

## Findings

### Initial Validation

**Assessment**: LEGITIMATE

#### Analysis

This issue is a well-documented feature request to remove the mutual exclusivity constraint between `run_if_changed` and `skip_if_only_changed` in Prow job configuration.

**Issue Category**: Feature Request

**Repository Scope Check**:
- Component mentioned: Job triggering configuration validation
- Exists in this repo: Yes
- Relevant code paths: `pkg/config/config.go` (validateTriggering function)

**Information Completeness**:
- Sufficient detail provided: Yes
- The issue provides:
  - Current implementation reference (commit 419f5e43)
  - Code snippet showing the constraint
  - Use case motivation
  - Proposed implementation approach
  - Backward compatibility considerations

#### Maintainer Feedback Already Present

Two maintainers have weighed in negatively on this proposal:

1. **petr-muller**: 👎 - The example scenario doesn't demonstrate a legitimate need (the `run_if_changed` alone would already skip docs-only changes). Concerned about footgun potential from misconfiguration between the two fields.

2. **BenTheElder**: Worries about making this feature "even more of a footgun and difficult to reason about." Notes that `skip_if_only_changed` is generally the safer approach.

**Author's Clarification**:
The author (kaovilai) provided a more concrete use case: needing to match `.yaml` files across ~10 directories but exclude the `docs/` directory. Since Go's RE2 regexp doesn't support negative lookahead, the workaround regex becomes complex and unmaintainable.

#### Recommendation

**Keep open** - This is a legitimate feature request for Prow configuration. While maintainers have expressed concerns, the discussion is ongoing and the author has provided additional real-world use cases. The issue should remain open for:
1. Further community input
2. Potential reconsideration if more compelling use cases emerge
3. Possible alternative solutions

**Suggested Action**: Continue triage to fully understand the technical implications, then document the maintainer decision rationale for future reference.

---

### Code Research

#### Current Implementation

**Primary Components**:
- **RegexpChangeMatcher**: `pkg/config/jobs.go:373-385` - Struct holding `RunIfChanged` and `SkipIfOnlyChanged` fields plus compiled regex
- **validateTriggering**: `pkg/config/config.go:3115-3133` - Enforces mutual exclusivity for presubmits
- **validateAlwaysRun**: `pkg/config/config.go:3100-3113` - Enforces mutual exclusivity for postsubmits
- **RunsAgainstChanges**: `pkg/config/jobs.go:465-476` - Core triggering logic that evaluates whether job should run

**Architecture Overview**:
The `RegexpChangeMatcher` struct is embedded in both `Presubmit` and `Postsubmit` job types. It stores one of two mutually exclusive regex patterns. The current design uses **XOR semantics** - only ONE regex can be compiled and stored in the `reChanges` field.

**Key Code Paths**:

1. **Validation** (config.go:3124-3126):
```go
if job.RunIfChanged != "" && job.SkipIfOnlyChanged != "" {
    return fmt.Errorf("job %s declares run_if_changed and skip_if_only_changed, which are mutually exclusive", job.Name)
}
```

2. **Regex Compilation** (config.go:3306-3321 - `setChangeRegexes`):
   - Uses if/else-if chain - only ONE regex compiled into `reChanges`
   - Comment on jobs.go:384 explicitly states: "from RunIfChanged xor SkipIfOnlyChanged"

3. **Triggering Evaluation** (jobs.go:465-476):
```go
func (cm RegexpChangeMatcher) RunsAgainstChanges(changes []string) bool {
    for _, change := range changes {
        if cm.RunIfChanged != "" && cm.reChanges.MatchString(change) {
            return true
        } else if cm.SkipIfOnlyChanged != "" && !cm.reChanges.MatchString(change) {
            return true
        }
    }
    return false
}
```
Uses `else if` - if both fields were set, only `RunIfChanged` would be evaluated.

**Data Flow**:
1. Job YAML parsed → `RegexpChangeMatcher` fields populated
2. Validation runs → checks mutual exclusivity (rejects if both set)
3. Regex compiled → `setChangeRegexes()` compiles exactly one regex
4. On PR/push event → `RunsAgainstChanges()` evaluates file changes
5. Returns true/false → determines if job triggers

#### Related Code

**Validation Call Sites**:
- Presubmits: `config.go:2327` (in `validatePresubmits`)
- Postsubmits: `config.go:2390` (in `validatePostsubmits`)

**Job Structs Affected**:
- `Presubmit`: jobs.go:195-232 (embeds `RegexpChangeMatcher` at line 223)
- `Postsubmit`: jobs.go:259-281 (embeds `RegexpChangeMatcher` at line 273)

**Similar Pattern - Brancher** (jobs.go:360-369):
The `Branches` and `SkipBranches` fields are NOT mutually exclusive - both can be set together. `SkipBranches` takes precedence. This is a complementary pattern that could inform the feature design.

#### Test Coverage

**Existing Tests**:
- `config_test.go:8052-8062`: `TestValidatePresubmits` - tests mutual exclusivity error
- `config_test.go:8139-8149`: `TestValidatePostsubmits` - tests mutual exclusivity error
- `config_test.go:4979`: `TestValidateAlwaysRunPostsubmit` - tests postsubmit validation
- `jobs_test.go:29-59`: `TestRunIfChangedPresubmits` - 6 test cases for matching
- `jobs_test.go:94-131`: `TestSkipIfOnlyChangedPresubmits` - 6 test cases for skipping
- `jobs_test.go:679-819`: Comprehensive `Presubmit.ShouldRun` tests

**Coverage Assessment**: STRONG for individual field behavior, NO coverage for combined usage (rejected at validation)

**Test Gaps**:
- No tests for combined `run_if_changed` + `skip_if_only_changed` (would be needed if feature implemented)
- No tests for semantic edge cases of combined logic

#### Documentation Review

**Code Comments** (jobs.go:375-382):
```go
// RunIfChanged defines a regex used to select which subset of file changes should trigger this job.
// If any file in the changeset matches this regex, the job will be triggered
// Additionally AlwaysRun is mutually exclusive with RunIfChanged.

// SkipIfOnlyChanged defines a regex used to select which subset of file changes should trigger this job.
// If all files in the changeset match this regex, the job will be skipped.
// In other words, this is the negation of RunIfChanged.
// Additionally AlwaysRun is mutually exclusive with SkipIfOnlyChanged.
```

**User Documentation**: `site/content/en/docs/jobs.md:230` states fields are mutually exclusive

**Known Limitations**:
- Go's RE2 regex engine (used by Prow) does NOT support negative lookahead (`(?!...)`)
- This is the core limitation driving the feature request - users cannot construct complex exclusion patterns in a single regex

#### Root Cause Analysis

**Primary Cause**:
This is a **design decision**, not a bug. The mutual exclusivity was intentionally implemented as a "guardrail" to prevent misconfiguration. The validation check (config.go:3124-3126) explicitly prevents both fields from being set together.

**Rationale for Current Design**:
1. Simplicity: Only one regex compiled/evaluated
2. Safety: Prevents confusing interactions between two regex patterns
3. Predictability: Job triggering behavior is easier to reason about
4. Error Prevention: Catches likely user mistakes (setting both when they meant one)

**Contributing Factors to the Feature Request**:
1. RE2's lack of negative lookahead makes complex exclusion patterns unwieldy
2. Real use cases exist where users want to match files in many directories but exclude specific subdirectories
3. The `Branches` / `SkipBranches` pattern shows combined usage can work

#### Proposed Solutions

##### Approach 1: Remove Constraint and Implement AND Logic

**Description**: Remove validation constraint; if both fields set, job runs only when:
- At least one file matches `run_if_changed` AND
- NOT all files match `skip_if_only_changed`

**Pros**:
- Addresses the author's use case directly
- Backward compatible (existing configs unchanged)
- Follows semantic precedent of `Branches`/`SkipBranches`

**Cons**:
- Increased configuration complexity
- Potential for subtle misconfiguration (maintainer concern)
- Requires storing/compiling two separate regexes
- `RunsAgainstChanges()` logic becomes more complex

**Affected Components**:
- `validateTriggering`: Remove lines 3124-3126
- `validateAlwaysRun`: Remove lines 3109-3111
- `RegexpChangeMatcher`: Add second compiled regex field
- `setChangeRegexes`: Compile both regexes
- `RunsAgainstChanges`: Implement combined evaluation logic

**Complexity**: Medium

**Backwards Compatibility**: Fully compatible - only enables previously invalid configurations

##### Approach 2: Maintain Status Quo (Document Workarounds)

**Description**: Keep the mutual exclusivity constraint. Document recommended workarounds:
1. List specific directories explicitly in `run_if_changed`
2. Use alternation patterns in single regex
3. Use `skip_if_only_changed` alone (safer approach per maintainers)

**Pros**:
- No code changes required
- Maintains simplicity and safety guarantees
- Aligns with maintainer feedback

**Cons**:
- Workaround regexes are complex and hard to maintain
- Doesn't fully address the user's pain point
- May lead to repeated feature requests

**Affected Components**: Documentation only

**Complexity**: Low

**Backwards Compatibility**: N/A

#### Recommendation

**Preferred Approach**: Approach 2 (Maintain Status Quo)

**Rationale**:
Two maintainers (petr-muller and BenTheElder) have already expressed concerns about this feature. The core arguments are:
1. The added complexity creates a "footgun" - subtle misconfigurations are hard to debug
2. `skip_if_only_changed` alone is the safer paradigm
3. The benefit is limited to niche use cases

**Key Implementation Considerations** (if Approach 1 were reconsidered):
1. Define clear AND/OR semantics for combining the conditions
2. Ensure `SkipIfOnlyChanged` takes precedence (like `SkipBranches`)
3. Compile two separate regex objects
4. Update `RunsAgainstChanges()` with clear, documented logic
5. Add comprehensive tests for edge cases
6. Update documentation with examples

**Suggested Next Steps**:
1. Close the issue with explanation of maintainer concerns
2. Document workaround patterns in official docs
3. Consider reopening if compelling new use cases emerge with broader support

---

### Effort Assessment

**Effort Level**: 3 - Large (requires expertise / design discussion)

#### Summary

While technically a medium-complexity code change, this feature has been rejected by two maintainers who consider it a "footgun." The effort assessment reflects the design disagreement more than implementation difficulty - achieving consensus would require extensive discussion and potentially an RFC.

#### Factor Analysis

##### Scope of Changes
- **Assessment**: Moderate
- **Details**: ~5-7 files (config.go validation, jobs.go struct + logic, config_test.go, jobs_test.go, documentation), ~200-300 lines
- **Level Indication**: 2-3

##### Complexity
- **Assessment**: Moderate
- **Details**: Need to define combined evaluation semantics (AND vs OR), update regex compilation to handle two patterns, modify triggering logic. Not algorithmically hard but requires careful semantic design.
- **Level Indication**: 2-3

##### Required Expertise
- **Assessment**: Moderate
- **Details**: Understanding of job triggering flow, regex compilation, configuration validation patterns. Existing code provides clear examples.
- **Level Indication**: 2-3

##### Clarity and Certainty
- **Assessment**: Significant Uncertainty
- **Details**: The implementation is clear, but maintainers have explicitly opposed the feature. The semantic behavior of combined fields is not agreed upon. Would require design consensus that doesn't exist.
- **Level Indication**: 3-4

##### Testing Requirements
- **Assessment**: Moderate
- **Details**: Need tests for combined field behavior across various edge cases (both match, neither match, one matches, etc.). Existing test patterns provide good templates.
- **Level Indication**: 2-3

##### Backwards Compatibility
- **Assessment**: Fully compatible
- **Details**: Only enables previously invalid configurations. Existing configs continue to work unchanged.
- **Level Indication**: 1-2

##### Architectural Alignment
- **Assessment**: Questionable - Removes Intentional Guardrail
- **Details**: The mutual exclusivity is a deliberate design choice to prevent misconfiguration. Maintainers view it as a safety feature, not a limitation. Removing it contradicts their design philosophy.
- **Level Indication**: 3-4 (this is the critical factor)

##### External Dependencies
- **Assessment**: None
- **Details**: No external system constraints. The RE2 regex limitation is the driver but doesn't block implementation.
- **Level Indication**: 1-2

#### Recommended Labels

Based on this assessment:
- [x] `kind/feature`: Feature request for new capability
- [x] `area/prow/config`: Relates to Prow configuration validation
- [ ] `good-first-issue`: Not appropriate - contentious feature with maintainer opposition
- [ ] `help-needed`: Not appropriate - unlikely to be accepted without design consensus

#### Guidance for Contributors

**For Level 3 (Design Discussion Required)**:
- This feature has been tentatively rejected by maintainers
- Before implementing, a contributor would need to:
  1. Build broader community consensus
  2. Present more compelling use cases
  3. Address maintainer concerns about footgun potential
  4. Potentially write an RFC or design proposal
- Simply submitting a PR implementing this would likely be rejected
- Consider alternative approaches:
  - Document workaround regex patterns in official docs
  - Propose a linting tool to detect regex patterns that could be simplified
  - Suggest config validation warnings instead of errors

#### Caveats and Considerations

The effort level 3 rating is driven primarily by **design disagreement**, not technical complexity. If maintainers reversed their position, this would be a Level 2 (moderate) implementation. However, since consensus-building is required before any implementation could be accepted, the effective effort is significantly higher.

The author's clarified use case (matching .yaml files except in docs/) is valid but may still be achievable through documented workaround patterns without feature changes.

---

### Proposed Issue Augmentation

#### Title Change

- **No change needed**: Current title "Enhancement: Allow combining `run_if_changed` and `skip_if_only_changed` for more flexible job triggering" is already clear, specific, and accurately describes the request.

#### Proposed GitHub Comment

```
## Technical Context

The mutual exclusivity is deeply embedded in the current architecture. The `RegexpChangeMatcher` struct (`pkg/config/jobs.go:373-385`) stores only a single compiled regex in the `reChanges` field, with a code comment explicitly stating it holds data "from RunIfChanged xor SkipIfOnlyChanged." The `RunsAgainstChanges()` function uses an `else if` chain that would only evaluate `RunIfChanged` if both were set.

Interestingly, Prow already has a precedent for non-exclusive include/exclude patterns: the `Branches` and `SkipBranches` fields in the `Brancher` struct are NOT mutually exclusive and can be used together, with `SkipBranches` taking precedence. This pattern could potentially inform the semantics if this feature were reconsidered.

## RE2 Limitation Context

The core driver for this request is that Go's RE2 regex engine (used by Prow) doesn't support negative lookahead (`(?!...)`), making it impossible to express "match X but not Y" in a single regex. The workaround patterns (explicit directory listing or complex alternation) are indeed unwieldy as @kaovilai noted.

/area prow/config
```

#### Rationale

**What's being added**:
- **XOR architecture detail**: The issue mentions the validation check but not that the struct itself is designed for XOR semantics (single `reChanges` field)
- **Branches/SkipBranches precedent**: This is a notable architectural precedent within Prow that supports the feature request's viability, not mentioned in the original discussion
- **RE2 limitation acknowledgment**: Validates the author's point about regex limitations

**Why these labels**:
- `/area prow/config`: More specific than the existing `area/plugins` - this is about config validation, not plugins themselves. Adding this provides better categorization.
- No `/kind` change: Already correctly labeled as `kind/feature`
- No difficulty label: Level 3 issue - requires design discussion and maintainer consensus, not suitable for `good-first-issue` or `help-wanted`

**What's NOT included**:
- No `/retitle`: Title is already good
- No implementation guidance: Maintainers have expressed opposition; providing implementation details would be premature
- No priority label: Feature request without urgent need
- No recommendation to close: Despite maintainer opposition, the issue has value as a design discussion record and the author's clarified use case deserves consideration

#### Posting Recommendation

**Recommend posting**: The comment adds useful technical context that strengthens the discussion. It doesn't advocate for or against the feature but provides information that could help future decision-making. The `Branches`/`SkipBranches` precedent is particularly valuable context that hasn't been mentioned.

## Next Steps

- [x] Initial validation complete
- [x] Code research complete
- [x] Assess effort level
- [x] Prepare augmentation comment
- [ ] Brief maintainer on findings

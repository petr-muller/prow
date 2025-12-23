# Subcommand: maintenance::issues::triage::assess-effort

## Purpose

Assess the effort required to address an issue and categorize it into one of four effort levels. This helps maintainers:
- Assign appropriate labels (good-first-issue, help-needed, etc.)
- Set expectations for potential contributors
- Prioritize work based on complexity
- Identify issues suitable for new vs experienced contributors

## Effort Levels

### Level 1: Easy Change (good-first-issue)
- Well-defined and well-understood problem
- Limited scope (typically 1-3 files, <100 lines of code)
- Clear solution approach with no significant trade-offs
- Minimal architectural impact
- Good test coverage exists for similar functionality
- No backwards compatibility concerns
- Could be completed by a new contributor with guidance

**Typical examples**:
- Documentation fixes
- Adding a simple field or option
- Fixing a clear logic error
- Adding validation for obvious edge case

### Level 2: Moderate Change (help-needed)
- Well-defined but somewhat involved
- Moderate scope (3-10 files, 100-500 lines of code)
- Solution approach is clear but requires some expertise
- May involve understanding multiple components
- Requires knowledge of existing patterns
- Some test coverage gaps to fill
- Backwards compatible changes
- Suitable for skilled contributors

**Typical examples**:
- Refactoring a component to improve maintainability
- Adding a moderately complex feature with clear requirements
- Fixing a bug that spans multiple components
- Improving error handling across a subsystem

### Level 3: Large Change (requires expertise)
- Large scope or significant uncertainty
- May affect many files/components (10+ files, 500+ lines)
- Requires deep expertise in Prow architecture
- Involves trade-offs requiring architectural judgment
- May introduce new behavior significantly different from existing patterns
- Could impact existing deployments (behavior changes)
- May be limited by external API constraints (GitHub, Kubernetes, etc.)
- Requires careful consideration of backwards compatibility
- Suitable only for experienced Prow contributors

**Typical examples**:
- Adding a new major feature
- Rearchitecting a component for better performance/reliability
- Fixing a race condition in complex concurrent code
- Changes that modify core merge/test logic
- Features requiring new configuration options affecting all deployments

### Level 4: Very Large or Near Impossible
- Extremely large scope or fundamental limitations
- Contradicts established architectural patterns
- Would require breaking changes across Prow
- Limited by external system design (cannot be solved in Prow alone)
- May not align with Prow's purpose and design philosophy
- Would require coordinated changes across multiple repositories
- Backwards incompatible in ways that would break many deployments

**Typical examples**:
- Complete rewrite of core systems
- Changes that fundamentally contradict Prow's architecture
- Features that require GitHub API capabilities that don't exist
- Changes that would break existing deployments with no migration path

## Parameters

- `issue_number` (required): The GitHub issue number to assess

## Instructions

You are assessing the effort required for GitHub issue #{issue_number}.

### Step 1: Review Previous Triage Findings

Read `ISSUE-TRIAGE.md` to understand:
- Initial validation results (is it legitimate?)
- Code research findings (root cause, proposed solutions)
- Current implementation details
- Test coverage assessment

### Step 2: Evaluate Key Factors

Assess each of the following dimensions:

#### 2.1 Scope of Changes

**Questions to consider**:
- How many files will need modification?
- How many components/packages are affected?
- Estimated lines of code to add/modify/delete?
- Are changes localized or spread across the codebase?

**Scoring**:
- Small scope (1-3 files, <100 LOC) → favors Level 1-2
- Moderate scope (3-10 files, 100-500 LOC) → favors Level 2-3
- Large scope (10+ files, 500+ LOC) → favors Level 3-4

#### 2.2 Complexity

**Questions to consider**:
- Is the solution approach straightforward or complex?
- Does it involve concurrent programming, race conditions, or timing issues?
- Are there algorithmic challenges?
- Does it require understanding complex interactions between components?
- How many edge cases need handling?

**Scoring**:
- Simple logic, clear path → favors Level 1-2
- Moderate complexity, some edge cases → favors Level 2
- High complexity, many edge cases, concurrency → favors Level 3-4

#### 2.3 Required Expertise

**Questions to consider**:
- How much Prow-specific knowledge is needed?
- Does it require domain expertise (Kubernetes, GitHub API, CI/CD)?
- Can someone learn what's needed from existing code/docs?
- Is familiarity with Go concurrency patterns required?
- Does it require understanding of distributed systems concepts?

**Scoring**:
- Minimal expertise, learnable from examples → Level 1-2
- Moderate expertise, need to understand patterns → Level 2-3
- Deep expertise required → Level 3-4

#### 2.4 Clarity and Certainty

**Questions to consider**:
- Is the problem well-defined?
- Is the solution approach agreed upon?
- Are there multiple viable approaches with unclear trade-offs?
- Is the desired behavior clear and unambiguous?
- Are requirements complete or are there open questions?

**Scoring**:
- Well-defined problem and solution → favors Level 1-2
- Well-defined problem, some solution uncertainty → favors Level 2-3
- Significant uncertainty or ambiguity → favors Level 3-4

#### 2.5 Testing Requirements

**Questions to consider**:
- How extensive are the testing needs?
- Can existing test patterns be followed?
- Are integration tests needed?
- How difficult is it to reproduce the scenario?
- Are there test coverage gaps that need addressing first?

**Scoring**:
- Simple unit tests, existing patterns → favors Level 1-2
- Moderate test needs, some new patterns → favors Level 2-3
- Complex integration tests, new test infrastructure → favors Level 3-4

#### 2.6 Backwards Compatibility Impact

**Questions to consider**:
- Will this change existing behavior?
- Could this break existing deployments?
- Is a feature flag or migration strategy needed?
- Are configuration changes required?
- Will this affect all users or only those who opt-in?

**Scoring**:
- Fully backwards compatible, additive only → favors Level 1-2
- Backwards compatible with minor caveats → favors Level 2
- Behavior changes requiring careful rollout → favors Level 3
- Breaking changes → Level 4

#### 2.7 Architectural Alignment

**Questions to consider**:
- Does this fit naturally with Prow's architecture?
- Does it follow existing patterns and conventions?
- Does it require introducing new patterns?
- Does it contradict or work against established design decisions?
- Is this something Prow is designed to do?

**Scoring**:
- Perfect alignment with existing patterns → favors Level 1-2
- Good fit with minor pattern extensions → favors Level 2-3
- Requires new patterns but still aligned → favors Level 3
- Contradicts architecture or out of scope → Level 4

#### 2.8 External Dependencies

**Questions to consider**:
- Does this depend on external API capabilities (GitHub, Kubernetes)?
- Are there known limitations in external systems?
- Does this require changes to systems outside Prow?
- Are external APIs documented and stable?

**Scoring**:
- No external dependencies or well-supported APIs → favors Level 1-3
- Limited by external API capabilities → favors Level 3-4
- Blocked by external system limitations → Level 4

### Step 3: Determine Effort Level

Based on the factor assessments, determine the overall effort level:

1. **If most factors favor Level 1**: Easy change, good-first-issue candidate
2. **If most factors favor Level 2**: Moderate change, help-needed candidate
3. **If most factors favor Level 3**: Large change, requires expertise
4. **If any critical factors (backwards compatibility, architecture, external limits) indicate Level 4**: Very large or near impossible

**Important**: Use the **highest level** indicated by critical factors. For example:
- If scope is small but backwards compatibility is a major concern → Level 3, not Level 1
- If solution is clear but contradicts architecture → Level 4, not Level 2

### Step 4: Update Triage Document

Add your assessment to `ISSUE-TRIAGE.md`:

```markdown
## Effort Assessment

**Effort Level**: [1/2/3/4] - [Easy/Moderate/Large/Very Large]

### Summary

[1-2 sentence summary of why this level was assigned]

### Factor Analysis

#### Scope of Changes
- **Assessment**: [Small/Moderate/Large]
- **Details**: [Estimated files, LOC, components affected]
- **Level Indication**: [1-2 / 2-3 / 3-4]

#### Complexity
- **Assessment**: [Simple/Moderate/High]
- **Details**: [What makes it simple/complex]
- **Level Indication**: [1-2 / 2-3 / 3-4]

#### Required Expertise
- **Assessment**: [Minimal/Moderate/Deep]
- **Details**: [What knowledge is needed]
- **Level Indication**: [1-2 / 2-3 / 3-4]

#### Clarity and Certainty
- **Assessment**: [Well-defined/Some uncertainty/Significant uncertainty]
- **Details**: [What's clear, what's unclear]
- **Level Indication**: [1-2 / 2-3 / 3-4]

#### Testing Requirements
- **Assessment**: [Simple/Moderate/Complex]
- **Details**: [What tests are needed]
- **Level Indication**: [1-2 / 2-3 / 3-4]

#### Backwards Compatibility
- **Assessment**: [Fully compatible/Minor impact/Breaking changes]
- **Details**: [Impact on existing deployments]
- **Level Indication**: [1-2 / 2-3 / 3-4]

#### Architectural Alignment
- **Assessment**: [Perfect fit/Good fit/New patterns/Contradicts]
- **Details**: [How well it fits Prow's architecture]
- **Level Indication**: [1-2 / 2-3 / 3-4]

#### External Dependencies
- **Assessment**: [None/Well-supported/Limited/Blocking]
- **Details**: [External system constraints]
- **Level Indication**: [1-3 / 3-4 / 4]

### Recommended Labels

Based on this assessment, recommend the following labels:
- [x] `[label-name]`: [reason]
- [ ] `[other-label]`: [reason if not recommended]

### Guidance for Contributors

**For Level 1 (Easy)**:
- Good starting point for new Prow contributors
- Suggested prerequisite knowledge: [list basics]
- Mentorship available: [Yes/No]
- Related documentation: [links]

**For Level 2 (Moderate)**:
- Suitable for contributors familiar with [specific area]
- Should review: [list relevant code/docs]
- Recommended approach: [high-level guidance]

**For Level 3 (Large)**:
- Requires experience with Prow architecture
- Should consult with maintainers before starting
- Key architectural considerations: [list]
- Review PR #[number] for similar changes

**For Level 4 (Very Large)**:
- Requires significant design discussion
- May need RFC or design doc
- Consider alternative approaches: [suggestions]
- Consult with sig-testing leadership

### Caveats and Considerations

[Any important notes, warnings, or alternative perspectives on the assessment]
```

### Step 5: Commit Assessment

Save your assessment to the triage document:

```bash
git add ISSUE-TRIAGE.md
git commit -m "Effort assessment for issue #{issue_number}: Level [1/2/3/4]

[Brief explanation of the assessment]"
```

## Assessment Examples

### Example: Level 1 (Easy)

```markdown
## Effort Assessment

**Effort Level**: 1 - Easy (good-first-issue)

### Summary

Adding a validation check for a missing field. Clear fix with no architectural impact and existing test patterns to follow.

### Factor Analysis

#### Scope of Changes
- **Assessment**: Small
- **Details**: Single file modification (config validation), ~20 lines of code
- **Level Indication**: 1-2

#### Complexity
- **Assessment**: Simple
- **Details**: Straightforward validation logic, similar to existing checks
- **Level Indication**: 1-2

#### Required Expertise
- **Assessment**: Minimal
- **Details**: Can learn from existing validation code, no Prow-specific expertise needed
- **Level Indication**: 1-2

#### Clarity and Certainty
- **Assessment**: Well-defined
- **Details**: Clear what needs validation and what error to return
- **Level Indication**: 1-2

#### Testing Requirements
- **Assessment**: Simple
- **Details**: Add unit test following existing test pattern for validations
- **Level Indication**: 1-2

#### Backwards Compatibility
- **Assessment**: Fully compatible
- **Details**: Only validates invalid configs that would fail anyway
- **Level Indication**: 1-2

#### Architectural Alignment
- **Assessment**: Perfect fit
- **Details**: Follows existing validation pattern exactly
- **Level Indication**: 1-2

#### External Dependencies
- **Assessment**: None
- **Details**: No external systems involved
- **Level Indication**: 1-3

### Recommended Labels

- [x] `good-first-issue`: Clear, well-defined, small scope
- [x] `kind/cleanup`: Improving validation
- [ ] `help-needed`: Too simple, better for new contributors

### Guidance for Contributors

**For Level 1 (Easy)**:
- Good starting point for new Prow contributors
- Suggested prerequisite knowledge: Basic Go, understanding of validation patterns
- Mentorship available: Yes - maintainers can provide guidance
- Related documentation: config/validation.go shows existing patterns
```

### Example: Level 2 (Moderate)

```markdown
## Effort Assessment

**Effort Level**: 2 - Moderate (help-needed)

### Summary

Refactoring error handling in a component to provide better error messages. Well-understood but touches multiple files and requires understanding component interactions.

### Factor Analysis

#### Scope of Changes
- **Assessment**: Moderate
- **Details**: 5-7 files affected, ~200 lines modified, single component
- **Level Indication**: 2-3

#### Complexity
- **Assessment**: Moderate
- **Details**: Need to trace error paths, ensure error context is preserved
- **Level Indication**: 2-3

#### Required Expertise
- **Assessment**: Moderate
- **Details**: Understanding of Go error handling, familiarity with the component
- **Level Indication**: 2-3

#### Clarity and Certainty
- **Assessment**: Well-defined
- **Details**: Clear goals, examples of good error messages provided
- **Level Indication**: 1-2

#### Testing Requirements
- **Assessment**: Moderate
- **Details**: Update existing tests, add error message validation
- **Level Indication**: 2-3

#### Backwards Compatibility
- **Assessment**: Fully compatible
- **Details**: Only improving error messages, no behavior change
- **Level Indication**: 1-2

#### Architectural Alignment
- **Assessment**: Perfect fit
- **Details**: Improving existing code, no new patterns
- **Level Indication**: 1-2

#### External Dependencies
- **Assessment**: None
- **Details**: Internal refactoring only
- **Level Indication**: 1-3

### Recommended Labels

- [x] `help-needed`: Good scope for skilled contributor
- [x] `kind/cleanup`: Improving error handling
- [ ] `good-first-issue`: Requires moderate Prow knowledge
```

### Example: Level 3 (Large)

```markdown
## Effort Assessment

**Effort Level**: 3 - Large (requires expertise)

### Summary

Fixing a race condition in Tide's merge logic. Well-defined problem and solution (PR #563) but requires deep understanding of concurrency, affects core functionality, and needs careful testing.

### Factor Analysis

#### Scope of Changes
- **Assessment**: Moderate
- **Details**: 3 files (pkg/tide/tide.go, status.go, tide_test.go), ~150 lines, but touches critical merge path
- **Level Indication**: 2-3

#### Complexity
- **Assessment**: High
- **Details**: Concurrency issue, state tracking, race condition window, timing-dependent
- **Level Indication**: 3-4

#### Required Expertise
- **Assessment**: Deep
- **Details**: Understanding of race conditions, Tide architecture, GitHub API behavior, state management
- **Level Indication**: 3-4

#### Clarity and Certainty
- **Assessment**: Well-defined
- **Details**: Root cause identified, solution approach clear (track seen contexts)
- **Level Indication**: 1-2

#### Testing Requirements
- **Assessment**: Complex
- **Details**: Need to test race condition scenario, requires understanding timing windows
- **Level Indication**: 3-4

#### Backwards Compatibility
- **Assessment**: Fully compatible
- **Details**: Only prevents incorrect merges, no behavior change for correct scenarios
- **Level Indication**: 1-2

#### Architectural Alignment
- **Assessment**: Good fit
- **Details**: Extends existing context checking with state tracking
- **Level Indication**: 2-3

#### External Dependencies
- **Assessment**: Well-supported
- **Details**: Works around GitHub API behavior (removing check before re-run)
- **Level Indication**: 1-3

### Recommended Labels

- [x] `area/tide`: Core Tide functionality
- [x] `kind/bug`: Fixing race condition
- [x] `priority/important-soon`: Can cause incorrect merges
- [ ] `good-first-issue`: Requires deep expertise
- [ ] `help-needed`: Too complex for typical help-needed

### Guidance for Contributors

**For Level 3 (Large)**:
- Requires experience with Prow architecture, specifically Tide
- Should review:
  - pkg/tide/tide.go: context checking logic
  - pkg/tide/status.go: status update mechanism
  - Existing tests in pkg/tide/tide_test.go
- Key architectural considerations:
  - Thread safety of new state tracking
  - State cleanup to prevent memory growth
  - Correctness under concurrent access
- Review PR #563 which implements the recommended solution
- Consult with tide maintainers before implementation
```

### Example: Level 4 (Very Large)

```markdown
## Effort Assessment

**Effort Level**: 4 - Very Large or Near Impossible

### Summary

Request to make Prow support BitBucket instead of GitHub. This contradicts Prow's fundamental architecture which is deeply integrated with GitHub's API and assumes GitHub-specific features.

### Factor Analysis

#### Scope of Changes
- **Assessment**: Very Large
- **Details**: Would affect 50+ files across all components, essentially a rewrite
- **Level Indication**: 3-4

#### Complexity
- **Assessment**: Extremely High
- **Details**: Abstraction layer over all GitHub interactions, different API models, feature parity challenges
- **Level Indication**: 3-4

#### Required Expertise
- **Assessment**: Deep across entire codebase
- **Details**: Understanding of all Prow components, both GitHub and BitBucket APIs
- **Level Indication**: 3-4

#### Clarity and Certainty
- **Assessment**: Significant uncertainty
- **Details**: Not clear how to map GitHub features to BitBucket equivalents
- **Level Indication**: 3-4

#### Testing Requirements
- **Assessment**: Extremely Complex
- **Details**: Would need parallel test infrastructure for BitBucket
- **Level Indication**: 3-4

#### Backwards Compatibility
- **Assessment**: Breaking changes
- **Details**: Would fundamentally change how Prow is configured and used
- **Level Indication**: 3-4

#### Architectural Alignment
- **Assessment**: Contradicts architecture
- **Details**: Prow is designed specifically for GitHub, not abstracted for multiple providers
- **Level Indication**: 4

#### External Dependencies
- **Assessment**: Limited by differences
- **Details**: BitBucket API lacks features Prow relies on (e.g., specific webhook events)
- **Level Indication**: 3-4

### Recommended Labels

- [ ] `good-first-issue`: Completely inappropriate
- [ ] `help-needed`: Far too large
- [x] `kind/feature`: Requesting new capability
- [x] `wontfix`: Outside Prow's design scope
- [x] `question`: Needs discussion about alternatives

### Guidance for Contributors

**For Level 4 (Very Large)**:
- This is outside Prow's design scope
- Alternative approaches:
  - Consider using Prow's plugin architecture if only specific features needed
  - Look into BitBucket-native CI/CD solutions
  - Consider whether GitHub Enterprise might meet needs instead
- This would require:
  - RFC/design proposal to sig-testing
  - Fundamental architectural changes across entire codebase
  - Ongoing maintenance burden for dual-provider support
- Recommendation: Close as wontfix, suggest alternatives
```

## Important Notes

- Be honest but constructive in assessments
- Consider multiple perspectives (new contributor, experienced dev, maintainer)
- Level 3-4 issues aren't "bad" - they're important work requiring more expertise
- Err on the side of higher effort level if uncertain
- Consider the "happy path" and edge cases separately
- Don't let "solution exists in PR" automatically make it Level 1-2
- Remember that effort assessment can change as understanding improves

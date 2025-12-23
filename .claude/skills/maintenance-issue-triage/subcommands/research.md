# Subcommand: maintenance::issues::triage::research

## Purpose

Conduct in-depth research into the codebase to gather relevant information about the reported issue. This subcommand helps maintainers understand:

- The current implementation and architecture
- Related code paths, components, and dependencies
- Potential root causes of the issue
- Existing tests and documentation
- High-level approaches to addressing the issue

**Important**: This subcommand focuses on understanding and analysis, NOT on implementing code changes. Proposals should be architectural and high-level.

## Parameters

- `issue_number` (required): The GitHub issue number to research

## Instructions

You are conducting code research for GitHub issue #{issue_number}.

### Step 1: Review Issue Context

Read the current ISSUE-TRIAGE.md to understand:
- What the issue is about
- Components/code paths mentioned
- Any initial validation findings

### Step 2: Explore Relevant Code

Use the Task tool with subagent_type=Explore to investigate the codebase. Focus on:

1. **Primary Code Paths**
   - Locate and examine the code mentioned in the issue
   - Understand what this code does and how it works
   - Identify the purpose and responsibilities of each component

2. **Related Components**
   - Find other code that interacts with the primary components
   - Trace data flow and control flow
   - Identify dependencies and relationships

3. **Architecture and Design**
   - Understand the high-level design patterns used
   - Identify key abstractions and interfaces
   - Document the overall architecture of the affected area

4. **Error Handling and Edge Cases**
   - Look for how errors are handled
   - Identify potential race conditions or timing issues
   - Find edge cases that might not be properly handled

### Step 3: Examine Tests

Search for existing tests related to the issue:

1. **Unit Tests**
   - Find tests for the components mentioned in the issue
   - Assess test coverage for the problematic code paths
   - Identify gaps in test coverage

2. **Integration Tests**
   - Look for end-to-end tests that exercise the functionality
   - Check if the reported scenario is tested

3. **Test Patterns**
   - Understand how similar functionality is tested
   - Note testing patterns to follow when adding new tests

### Step 4: Review Documentation

Look for relevant documentation:
- Code comments explaining the logic
- Package-level documentation
- Design documents or ADRs (Architecture Decision Records)
- User-facing documentation that describes the feature

### Step 5: Analyze Root Causes

Based on your research, identify:

1. **Likely Root Cause(s)**
   - What is causing the reported behavior?
   - Are there race conditions, logic errors, missing validations?
   - Is this an architectural limitation?

2. **Contributing Factors**
   - What conditions must be present for the issue to occur?
   - Are there timing dependencies?
   - What assumptions does the code make?

### Step 6: Propose High-Level Solutions

Develop architectural approaches (NOT code implementations) to address the issue:

1. **Approach 1: [Name]**
   - High-level description of the approach
   - Pros and cons
   - Affected components
   - Complexity assessment
   - Backwards compatibility considerations

2. **Approach 2: [Name]** (if alternatives exist)
   - Alternative high-level description
   - Trade-offs compared to Approach 1
   - When this might be preferred

3. **Recommended Approach**
   - Which approach seems most appropriate and why
   - Key considerations for implementation

### Step 7: Update Triage Document

Add your research findings to `ISSUE-TRIAGE.md`:

```markdown
## Code Research

### Current Implementation

**Primary Components**:
- [Component 1]: [file path] - [brief description of what it does]
- [Component 2]: [file path] - [brief description]

**Architecture Overview**:
[High-level description of how the components work together]

**Key Code Paths**:
1. [Path 1]: [file:line] - [description]
2. [Path 2]: [file:line] - [description]

**Data Flow**:
[Describe how data flows through the system in the relevant scenario]

### Related Code

**Dependencies**:
- [Component/package]: [how it's used]

**Callers**:
- [Component]: [file:line] - [context]

**Similar Functionality**:
- [Related feature]: [file path] - [how it's similar/relevant]

### Test Coverage

**Existing Tests**:
- [Test file]: [what it tests]
- Coverage assessment: [Good/Partial/Missing]

**Test Gaps**:
- [Missing scenario 1]
- [Missing scenario 2]

### Documentation Review

**Code Comments**:
- [Relevant comments found in the code]

**Design Documentation**:
- [Links to or descriptions of relevant design docs]

**Known Limitations**:
- [Any documented limitations that are relevant]

### Root Cause Analysis

**Primary Cause**:
[Detailed explanation of what's causing the issue]

**Contributing Factors**:
1. [Factor 1]
2. [Factor 2]

**Reproduction Conditions**:
- [Condition 1]: [why this matters]
- [Condition 2]: [why this matters]

### Proposed Solutions

#### Approach 1: [Name]

**Description**: [High-level architectural description]

**Pros**:
- [Advantage 1]
- [Advantage 2]

**Cons**:
- [Disadvantage 1]
- [Disadvantage 2]

**Affected Components**:
- [Component 1]: [how it would change]
- [Component 2]: [how it would change]

**Complexity**: [Low/Medium/High]

**Backwards Compatibility**: [Impact assessment]

#### Approach 2: [Alternative Name]

[Same structure as Approach 1]

#### Recommendation

**Preferred Approach**: [Which approach and why]

**Key Implementation Considerations**:
1. [Important consideration 1]
2. [Important consideration 2]

**Testing Requirements**:
- [Test scenario 1]
- [Test scenario 2]

**Migration/Rollout Strategy**:
[If applicable, high-level thoughts on how to roll this out safely]
```

### Step 8: Commit Findings

Save your research to the triage document:

```bash
git add ISSUE-TRIAGE.md
git commit -m "Code research for issue #{issue_number}

- Analyzed [component] implementation
- Identified root cause: [brief description]
- Proposed [N] high-level solution approaches"
```

## Research Strategies

### For Bug Issues

1. Start with the suspected code path mentioned in the issue
2. Use git blame to see recent changes to that code
3. Search for similar bug reports or fixes
4. Check if recent commits might have introduced the issue
5. Look for related error handling code

### For Feature Requests

1. Find similar existing features in the codebase
2. Understand the patterns used for similar functionality
3. Identify where the new feature would fit architecturally
4. Consider consistency with existing APIs/interfaces
5. Look for extensibility points that could be leveraged

### For Race Conditions

1. Identify shared state and how it's accessed
2. Look for synchronization primitives (mutexes, channels, etc.)
3. Trace concurrent code paths
4. Find where locks are acquired and released
5. Check for potential deadlocks or lock ordering issues

### For Performance Issues

1. Profile or identify hot code paths
2. Look for inefficient algorithms (O(n²), etc.)
3. Check for unnecessary allocations
4. Identify caching opportunities
5. Review database queries or API calls

## Example Research Output

### For a Race Condition Bug

```markdown
## Code Research

### Current Implementation

**Primary Components**:
- Tide Status Checker: pkg/tide/status.go - Evaluates PR status for merge eligibility
- GitHub Provider: pkg/tide/github.go - Fetches status checks from GitHub API

**Architecture Overview**:
Tide periodically syncs PR status by fetching check results from GitHub. When checks are re-triggered, there's a window where the old status is cached but the new check hasn't started yet. During this window, the status appears "successful" (no failures), leading to premature merges.

**Key Code Paths**:
1. Status evaluation: pkg/tide/status.go:478-492 - Determines if all required checks passed
2. Cache update: pkg/tide/status.go:210-225 - Updates cached status from GitHub
3. Merge decision: pkg/tide/merge.go:156-180 - Decides whether to merge based on status

**Data Flow**:
1. Tide sync loop fetches PR list
2. For each PR, calls status.go to get check status
3. Status checker reads from cache (updated every 30s)
4. If cache shows all checks passed, merge.go proceeds with merge
5. Race: If checks re-triggered between cache updates, stale "passed" status used

### Root Cause Analysis

**Primary Cause**:
Time-of-check-time-of-use (TOCTOU) race condition. Status is evaluated based on cached data that may be stale when checks are re-triggered. The code doesn't distinguish between "no failures" and "checks in progress".

**Contributing Factors**:
1. Cache update interval (30s) creates staleness window
2. No explicit "pending" state tracking for re-triggered checks
3. GitHub API may not immediately reflect re-triggered check status
4. Missing validation that required checks have actually started

### Proposed Solutions

#### Approach 1: Explicit Pending State Tracking

**Description**: Track when checks transition to "pending" state and block merges until all required checks have started and completed.

**Pros**:
- Addresses root cause directly
- Clear separation between "not started", "pending", and "completed"
- More reliable merge decisions

**Cons**:
- Requires additional state tracking
- May delay merges slightly if check startup is slow

**Complexity**: Medium

#### Approach 2: Freshness Guarantee for Merge Decisions

**Description**: Require a fresh GitHub API call (bypassing cache) immediately before merge decision to ensure status is current.

**Pros**:
- Simpler implementation
- No additional state needed
- Reduces TOCTOU window significantly

**Cons**:
- Increased API calls (rate limit considerations)
- Doesn't eliminate race entirely, just reduces window
- May slow down merge decisions

**Complexity**: Low

#### Recommendation

**Preferred Approach**: Approach 1 (Explicit Pending State Tracking)

This addresses the fundamental issue of distinguishing between different check states. While more complex, it provides a robust solution that won't have race windows.

**Key Implementation Considerations**:
1. Define state machine for check status transitions
2. Track timestamp of last status change to detect stale pending states
3. Ensure all required checks are explicitly tracked
4. Handle check re-triggers by resetting state to pending
```

## Important Notes

- Use the Explore agent for complex codebase navigation
- Focus on understanding, not implementation
- Document file paths with line numbers for key findings
- Be thorough but concise in your analysis
- Consider backwards compatibility in all proposals
- Think about testing requirements for each approach
- If multiple approaches are viable, present trade-offs objectively

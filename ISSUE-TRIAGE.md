# Triage for Issue #500

**Status**: In Progress
**Created**: 2026-01-30

## Issue Information

- **Issue Number**: #500
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/500

## Findings

### Initial Validation

**Assessment**: LEGITIMATE

**Issue Category**: Feature Request

**Summary**: Request to add `git cherry-pick -x` style commit messages to the cherrypicker external plugin, which would include original commit IDs in cherry-picked commits.

### Analysis

The issue requests a well-defined enhancement to the cherrypicker external plugin:

1. **Feature Description**: Add an option to include original commit IDs in cherry-picked commit messages, similar to `git cherry-pick -x` behavior
2. **Proposed Approach**:
   - Add option to the git interactor to amend commit messages after `git am` succeeds
   - Add flag to cherrypicker plugin to activate this feature
   - Enable via deployment flag
3. **Use Case**: Author provides concrete examples comparing desired vs current behavior (kubevirt PRs)

**Repository Scope Check**:
- Component mentioned: cherrypicker external plugin, git interactor
- Exists in this repo: Yes
  - `cmd/external-plugins/cherrypicker/` - cherrypicker plugin
  - `pkg/git/v2/interactor.go` - git interactor (referenced at line 424)
- Relevant code paths identified in the issue itself

**Information Completeness**:
- Sufficient detail provided: Yes
- Clear description of desired behavior with examples
- Proposed implementation approach included
- Links to reference documentation (`git cherry-pick -x`)

**Existing Labels**:
- `kind/feature` - Correct
- `area/plugins` - Correct
- `lifecycle/stale` - Issue was marked stale by triage robot

**Discussion Context**:
- BenTheElder noted plugin isn't used by core Kubernetes, referencing issue #113 about deprecation
- xmudrii clarified that Kubernetes subprojects actively use it
- BenTheElder acknowledged but noted bandwidth constraints

### Recommendation

**Keep open and continue triage.** This is a valid, well-documented feature request for the cherrypicker plugin which exists in this repository. The plugin is actively used by Kubernetes subprojects.

The `lifecycle/stale` label should be addressed - this is a legitimate request that was marked stale due to inactivity, not due to being invalid.

**Suggested Action**:
- Continue with research phase to understand implementation requirements
- Assess effort level for implementation
- Consider whether this could be a good-first-issue or help-wanted candidate

## Code Research

### Current Implementation

**Primary Components**:
- **Cherrypicker Server**: `cmd/external-plugins/cherrypicker/server.go` - Main plugin logic, handles webhooks and orchestrates cherry-picking
- **Git Interactor**: `pkg/git/v2/interactor.go` - Low-level git operations including patch application
- **Publisher**: `pkg/git/v2/publisher.go` - Commit creation and push operations

**Architecture Overview**:
The cherrypicker external plugin receives GitHub webhook events when PRs are merged. It clones the repository, checks out the target branch, downloads the PR patch from GitHub, applies it using `git am --3way`, then pushes the new branch and creates a PR.

**Key Code Paths**:
1. **Webhook handling**: `server.go:331-462` (`handlePullRequest()`) - Parses cherrypick labels/comments
2. **Main orchestration**: `server.go:467-624` (`handle()`) - Clones, applies patch, pushes, creates PR
3. **Patch application**: `server.go:559` calls `r.Am(localPath)`
4. **Am implementation**: `interactor.go:419-433` - Executes `git am --3way <patch>`
5. **Push**: `server.go:578` calls `p.Push()`

**Data Flow**:
```
GitHub Webhook → handlePullRequest() → handle()
  → Clone repo → Checkout target branch
  → Download PR patch from GitHub API
  → r.Am(localPath) [interactor.go:424: git am --3way]
  → [NO POST-PROCESSING OF COMMITS CURRENTLY]
  → p.Push() → CreatePullRequest()
```

### Related Code

**Configuration Pattern** (how to add new flags):
1. `main.go:64-81` - Define flag in `gatherOptions()`
2. `server.go:87-114` - Add field to `Server` struct
3. `main.go:125-142` - Pass to Server constructor
4. `server.go:handle()` - Use conditionally

**Example existing flag**: `issueOnConflict` (main.go:74 → server.go:567)

**Similar Functionality**:
- `MergeOpt` pattern (`interactor.go:100-104`) - Shows optional commit message customization
- `publisher.Commit()` (`publisher.go:52-69`) - Shows how to create commits with custom messages

### Test Coverage

**Existing Tests**:
- `cmd/external-plugins/cherrypicker/server_test.go` - Tests for server logic
- Coverage assessment: Good for core functionality, but no tests for commit message manipulation

**Test Gaps**:
- No tests for post-am commit amendment
- Will need new tests for the -x flag behavior

### Root Cause Analysis

**Primary Cause** (not a bug, but a missing feature):
The current implementation applies patches via `git am` and immediately pushes without any post-processing of commit messages. There's no mechanism to append original commit SHAs to the cherry-picked commits.

**Technical Gap**:
After `r.Am(localPath)` succeeds at line 559, the code proceeds directly to pushing. The insertion point for the new feature is between lines 559-578 in `server.go`.

### Proposed Solutions

#### Approach 1: Amend Commits After `git am` (Recommended)

**Description**: After successful patch application, iterate over the applied commits and amend each message to append the original commit SHA (like `git cherry-pick -x` does).

**Implementation**:
1. Add `--add-original-commit-id` flag to cherrypicker
2. After `r.Am()` succeeds, if flag is enabled:
   - Extract original commit SHA(s) from patch headers or use `git log`
   - For each commit, amend message to append `(cherry picked from commit <SHA>)`
3. This happens before `p.Push()`

**Pros**:
- Clean separation of concerns (am succeeds first, then enhance)
- Follows existing patterns in the codebase
- Easy to make optional via flag
- Works with existing git am flow

**Cons**:
- Requires parsing patch or using git commands to get original SHAs
- More complex for multi-commit PRs (need to rebase/amend each)

**Affected Components**:
- `cmd/external-plugins/cherrypicker/main.go` - Add flag
- `cmd/external-plugins/cherrypicker/server.go` - Add logic after Am()
- Possibly `pkg/git/v2/interactor.go` - Add helper method

**Complexity**: Medium

**Backwards Compatibility**: Full - opt-in via new flag

#### Approach 2: Modify Patch Before Application

**Description**: Parse the downloaded patch and inject the `(cherry picked from commit <SHA>)` line into each commit message before calling `git am`.

**Pros**:
- Single pass - no post-processing needed
- Works for any number of commits

**Cons**:
- Requires patch parsing/modification
- More complex to implement correctly
- Could break if patch format changes

**Complexity**: Medium-High

**Backwards Compatibility**: Full - opt-in via new flag

#### Recommendation

**Preferred Approach**: Approach 1 (Amend Commits After `git am`)

This is cleaner and follows the pattern suggested in the issue itself. The patch application and message enhancement are cleanly separated. The existing `git am` flow is preserved, and the enhancement is additive.

**Key Implementation Considerations**:
1. For single-commit PRs: Simple `git commit --amend` with new message
2. For multi-commit PRs: Need interactive rebase or sequential amendment
3. Must preserve original commit author information
4. Need to extract original SHA from patch headers or PR metadata

**Testing Requirements**:
- Test single-commit cherry-pick with -x flag
- Test multi-commit cherry-pick with -x flag
- Test that flag disabled produces unchanged behavior
- Test with merge commits

## Effort Assessment

**Effort Level**: 2 - Moderate (help-needed)

### Summary

This is a well-defined feature request with clear requirements and an established implementation pattern to follow. While the scope is moderate and requires understanding the cherrypicker flow, it's fully backwards compatible and follows existing code patterns.

### Factor Analysis

#### Scope of Changes
- **Assessment**: Small-to-Moderate
- **Details**: 3-5 files affected (~100-200 lines):
  - `main.go` - add flag definition (~5 lines)
  - `server.go` - add struct field and post-Am logic (~30-50 lines)
  - Possibly `interactor.go` - helper method (~20 lines)
  - `server_test.go` - new tests (~50-100 lines)
- **Level Indication**: 2

#### Complexity
- **Assessment**: Moderate
- **Details**:
  - Single-commit case is straightforward (amend HEAD)
  - Multi-commit case requires more care (rebase or sequential amendment)
  - Need to extract original SHA from patch headers
  - Must preserve author information
- **Level Indication**: 2

#### Required Expertise
- **Assessment**: Moderate
- **Details**:
  - Understanding of cherrypicker plugin flow
  - Git knowledge (am, amend, rebase)
  - Go development experience
  - Existing code is well-documented with clear patterns to follow
- **Level Indication**: 2

#### Clarity and Certainty
- **Assessment**: Well-defined
- **Details**:
  - Feature request is clear (like `git cherry-pick -x`)
  - Issue author provided concrete examples
  - Proposed approach already outlined
  - Reference documentation available
- **Level Indication**: 1-2

#### Testing Requirements
- **Assessment**: Moderate
- **Details**:
  - Unit tests for new flag
  - Tests for single-commit and multi-commit cases
  - Tests for flag-disabled behavior
  - Can follow existing patterns in server_test.go
- **Level Indication**: 2

#### Backwards Compatibility
- **Assessment**: Fully compatible
- **Details**:
  - Fully opt-in via new CLI flag
  - No changes to existing behavior when flag is not set
  - No migration required
- **Level Indication**: 1

#### Architectural Alignment
- **Assessment**: Perfect fit
- **Details**:
  - Follows existing flag pattern (like `issueOnConflict`)
  - Clean insertion point after Am() in server.go
  - Consistent with codebase style and conventions
- **Level Indication**: 1-2

#### External Dependencies
- **Assessment**: None
- **Details**:
  - Uses standard git commands (am, amend)
  - No GitHub API changes needed
  - Git behavior is well-documented and stable
- **Level Indication**: 1

### Recommended Labels

- [x] `help-wanted`: Good scope for skilled contributor familiar with Go and git
- [ ] `good-first-issue`: Too involved - requires understanding cherrypicker flow and handling multi-commit cases
- [x] `kind/feature`: Already applied, correct
- [x] `area/plugins`: Already applied, correct
- [ ] `lifecycle/stale`: Should be removed - this is a legitimate, actionable issue

### Guidance for Contributors

**For Level 2 (Moderate)**:
- Suitable for contributors familiar with Go and git operations
- Should review:
  - `cmd/external-plugins/cherrypicker/server.go` - main plugin logic
  - `pkg/git/v2/interactor.go` - git operations
  - `cmd/external-plugins/cherrypicker/server_test.go` - test patterns
- Recommended approach:
  1. Add `--add-original-commit-id` flag following `issueOnConflict` pattern
  2. After successful `Am()`, if flag enabled, amend commits with original SHA
  3. Handle both single-commit and multi-commit PRs
  4. Preserve original author information
- Key implementation considerations:
  - Extract original SHA from patch headers (look for `From <SHA>` line)
  - For multi-commit PRs, use `git filter-branch` or sequential rebase
  - Add comprehensive tests

### Caveats and Considerations

- **Multi-commit complexity**: The main challenge is handling PRs with multiple commits. The implementation needs to amend each commit, not just HEAD.
- **Maintainer bandwidth**: Comments on the issue indicate limited maintainer bandwidth for cherrypicker plugin. A clean, well-tested PR would be more likely to be reviewed.
- **Active users**: While not used by core Kubernetes, the plugin is actively used by Kubernetes subprojects (e.g., KubeVirt).

## Proposed Issue Augmentation

### Title Change

- **Current**: "cherrypicker: add a flag to support `git cherry-pick -x` style commit messages for"
- **Proposed**: "cherrypicker: add flag to include original commit SHA in cherry-picked commit messages"
- **Rationale**: Current title is truncated (ends with "for"). Proposed title is complete and clearly describes the feature.

### Proposed GitHub Comment

```
/retitle cherrypicker: add flag to include original commit SHA in cherry-picked commit messages

## Implementation Notes

The insertion point for this feature is in `cmd/external-plugins/cherrypicker/server.go` between lines 559-578, after `r.Am(localPath)` succeeds but before `p.Push()`. A new CLI flag (e.g., `--add-original-commit-id`) can follow the existing pattern used by `--create-issue-on-conflict` in `main.go:74`.

The main complexity is handling multi-commit PRs correctly. For single-commit PRs, a simple `git commit --amend` suffices, but multi-commit PRs require amending each commit in the series while preserving author information. The original commit SHAs can be extracted from the patch file headers (the `From <SHA>` line).

/remove-lifecycle stale
/help-wanted
```

### Rationale

**What's being added**:
- **Implementation details**: Specific file paths and line numbers for insertion point, pattern to follow for the flag
- **Complexity note**: The multi-commit case is the main implementation challenge, not mentioned in original issue
- **Stale label removal**: Issue was marked stale due to inactivity, not invalidity

**Why these labels**:
- `/remove-lifecycle stale`: This is a legitimate, actionable issue that should not be marked stale
- `/help-wanted`: Level 2 effort assessment - moderate complexity, well-defined, suitable for skilled contributors

**What's NOT included**:
- `/area plugins` and `/kind feature`: Already applied, no need to repeat
- Detailed architecture overview: Issue author already provided good context
- Priority labels: No urgency warranting priority assignment
- Good-first-issue: Too complex for new contributors (multi-commit handling)

## Next Steps

1. ~~Initial validation~~ - Complete (LEGITIMATE)
2. ~~Research~~ - Complete
3. ~~Assess effort~~ - Complete (Level 2 - Moderate)
4. ~~Augment~~ - Complete
5. Brief - Walk through findings (optional)
6. Wrapup - Post findings to issue

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

## Next Steps

1. ~~Initial validation~~ - Complete (LEGITIMATE)
2. ~~Research~~ - Complete
3. Assess effort - Determine complexity
4. Augment - Propose improvements to the issue
5. Brief - Walk through findings (optional)
6. Wrapup - Post findings to issue

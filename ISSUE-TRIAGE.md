# Triage for Issue #500 (PR #661)

**Status**: In Progress
**Created**: 2026-04-10

## Issue Information

- **Issue Number**: #500
- **PR Number**: #661
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/500
- **PR URL**: https://github.com/kubernetes-sigs/prow/pull/661
- **PR Author**: @AaruniAggarwal
- **Issue Author**: @dhiller

## Initial Validation

**Assessment**: LEGITIMATE

### Analysis

Issue #500 requests adding `git cherry-pick -x` style commit messages to the cherrypicker external plugin. When enabled, cherry-picked commits would include `(cherry picked from commit <sha>)` in the commit message, matching what `git cherry-pick -x` does natively. This helps track which changes have been backported to release branches.

PR #661 implements this feature but has significant design and security issues that need to be addressed before merging.

**Issue Category**: Feature Request

**Repository Scope Check**:
- Component mentioned: cherrypicker external plugin
- Exists in this repo: Yes (`cmd/external-plugins/cherrypicker/`)
- Relevant code paths: `server.go` (handle function), `main.go` (flag registration), `pkg/git/v2/interactor.go` (Am method)

**Information Completeness**:
- Sufficient detail provided: Yes
- The original issue (#500) provides clear motivation, desired outcome with examples, and even suggests two implementation approaches
- PR #661 provides working code and manual test results for 1, 2, and 3 commit PRs

### Recommendation

Keep open and continue triage. This is a valid, well-motivated feature request with a working (but flawed) implementation. The PR needs significant rework before it's mergeable.

**Suggested Action**:
- Keep open and continue triage
- Provide detailed review feedback on the PR with an alternative implementation approach

## Code Research

### Current Implementation

**Primary Components**:
- Cherrypicker server: `cmd/external-plugins/cherrypicker/server.go` - Main plugin logic
- Git interactor: `pkg/git/v2/interactor.go` - Git operations abstraction layer
- Cherrypicker lib: `cmd/external-plugins/cherrypicker/lib/lib.go` - PR body generation

**Architecture Overview**:
The cherrypicker plugin handles GitHub webhook events (issue comments with `/cherrypick <branch>` and PR merge events). The `handle()` function orchestrates the cherry-pick: it fetches the PR as a patch from GitHub's API (mbox format), creates a new branch on the target, applies the patch with `git am --3way`, pushes the branch, and creates a new PR.

**Key Code Paths**:
1. Patch fetching: `server.go:694-703` - `getPatch()` downloads patch via `GetPullRequestPatch()` and writes to `/tmp/`
2. Patch application: `server.go:559` - `r.Am(localPath)` applies the mbox patch
3. Branch push: `server.go:577` - `p.Push(r, newBranch, true)` pushes to remote
4. PR creation: `server.go:597` - Creates cherry-pick PR on GitHub

**Data Flow**:
1. Webhook event arrives → `handleIssueComment()` or `handlePullRequest()`
2. `handle()` is called with target branch info
3. Patch file fetched from GitHub (mbox format: `From <sha> Mon Sep 17...`)
4. New branch created from target branch
5. `git am` applies patch → creates commits with original messages
6. Branch pushed, PR created with cherry-pick body

### Related Code

**Dependencies**:
- `pkg/git/v2` - Git client factory and operations (RepoClient = Publisher + Interactor)
- `pkg/github` - GitHub API client for fetching patches, creating PRs
- `cmd/external-plugins/cherrypicker/lib` - Cherry-pick PR body generation

**Similar Functionality**:
- Title manipulation: `server.go:555-556` - Adds `[targetBranch]` prefix to PR title
- Release note extraction: `server.go:714-720` - Parses release notes from parent PR body
- PR body generation: `lib/lib.go:24-37` - Creates cherry-pick PR description
- None of these modify actual git commit messages

### Test Coverage

**Existing Tests**:
- `server_test.go`: 9 test functions covering IC handling, PR handling, merge events, labels, assignments, conflicts, locks
- Uses `fghc` (fake GitHub client) and `localgit.Clients` for git operations
- Fake pusher that always succeeds
- Coverage: Good for existing functionality, but no tests for commit message modification

**Test Gaps**:
- No tests for `extractOriginalSHAs()` (new function in PR)
- No tests for `appendCherryPickMessages()` (new function in PR)
- No tests verifying commit message content after cherry-pick
- No tests for the `addOriginalCommitID` flag behavior

### Root Cause Analysis

**Primary Cause**:
This is a feature gap, not a bug. The cherrypicker plugin uses `git am` to apply patches, which preserves original commit messages but does not add any provenance information. Native `git cherry-pick -x` adds `(cherry picked from commit <sha>)` automatically, but `git am` does not because it's applying patches, not cherry-picking commits.

**Contributing Factors**:
1. `git am` is the correct tool for applying mbox patches but lacks `-x` equivalent
2. The patch file (mbox format) already contains original commit SHAs in `From <sha>` headers
3. The SHAs are available but not used to annotate commit messages

### Proposed Solutions

#### Approach 1: Modify Patch File Before `git am` (Recommended)

**Description**: Parse the mbox patch file, insert `(cherry picked from commit <sha>)` into each commit's message section before the `---` separator, write the modified patch back, then let `r.Am()` apply it normally.

**Pros**:
- Pure Go string manipulation, no external commands
- No `os/exec` usage, no shell scripts, no temp files
- No security risks (no shell injection vector)
- Uses existing `r.Am()` - works within the git abstraction layer
- Trivially unit-testable (just test patch file transformation)
- Handles multi-commit PRs naturally (each commit section has its own `From` header)

**Cons**:
- Requires understanding mbox patch format
- Must handle edge cases in patch parsing (commit messages containing `---`)

**Affected Components**:
- `server.go`: Add new function `addCherryPickMessagesToPatch(patchPath)`, call it between `getPatch()` and `r.Am()`
- `main.go`: Add `--add-original-commit-id` flag (default false)
- `server_test.go`: Add unit tests for patch transformation

**Complexity**: Low-Medium

**Backwards Compatibility**: Fully backwards compatible (opt-in flag, default false)

#### Approach 2: Post-am Commit Message Rewriting (Current PR approach)

**Description**: After `git am` applies the patch, use `git filter-branch --msg-filter` with a generated shell script to rewrite commit messages.

**Pros**:
- Conceptually straightforward
- Author has demonstrated it works manually

**Cons**:
- Uses deprecated `git filter-branch`
- Requires `os/exec` directly, bypassing git abstraction layer
- Shell injection risk from unvalidated SHAs in generated script
- Heavyweight: rewrites entire branch history for simple message changes
- Complex: requires temp files, shell scripts, careful cleanup
- Hard to test properly
- Introduces `os/exec` and `bufio` imports not used elsewhere in this file

**Affected Components**:
- Same files, but more invasive changes
- Adds dependency on `git filter-branch` availability

**Complexity**: Medium-High

**Backwards Compatibility**: Flag defaults to `true` in current PR, which is a breaking change for existing deployments

#### Approach 3: Add Method to Interactor Interface

**Description**: Add a new method like `AmWithCherryPickMessage(path string)` to the `Interactor` interface that modifies patches before application.

**Pros**:
- Clean abstraction
- Reusable by other components

**Cons**:
- Changes a widely-used interface
- Increases coupling between cherrypicker and git abstraction
- Over-engineering for a single use case

**Complexity**: Medium

#### Recommendation

**Preferred Approach**: Approach 1 (Modify Patch File Before `git am`)

This is the simplest, safest, and most testable approach. The original issue author (@dhiller) explicitly mentioned this as one of two viable strategies: "modify the patch before application so that each part contains the original commit id."

**Key Implementation Considerations**:
1. Validate extracted SHAs against `^[0-9a-f]{40}$` regex
2. Handle edge cases: commit messages containing `---` (use the last `---` before `diff --git`)
3. Default the flag to `false` for backwards compatibility
4. Add unit tests for patch transformation with single and multi-commit patches

**Testing Requirements**:
- Unit test: `addCherryPickMessagesToPatch()` with single-commit patch
- Unit test: `addCherryPickMessagesToPatch()` with multi-commit patch
- Unit test: Patch with `---` in commit message body
- Unit test: Invalid/missing SHA handling
- Integration test: End-to-end cherry-pick with flag enabled, verify commit message

## Effort Assessment

**Effort Level**: 2 - Moderate (help-needed)

### Summary

The feature is well-defined with a clear implementation approach (patch file modification). It requires understanding the mbox patch format and the cherrypicker flow, but the actual code changes are moderate and follow existing patterns.

### Factor Analysis

#### Scope of Changes
- **Assessment**: Small-Moderate
- **Details**: 3 files modified (main.go, server.go, server_test.go), ~100-200 lines including tests
- **Level Indication**: 1-2

#### Complexity
- **Assessment**: Moderate
- **Details**: Mbox format parsing requires care around edge cases (commit messages with `---`), multi-commit patches need proper handling. The current PR approach is overly complex; the recommended approach is simpler.
- **Level Indication**: 2

#### Required Expertise
- **Assessment**: Moderate
- **Details**: Understanding of mbox patch format, cherrypicker plugin flow, Go string manipulation. No concurrency or distributed systems knowledge needed.
- **Level Indication**: 2

#### Clarity and Certainty
- **Assessment**: Well-defined
- **Details**: The desired behavior is clear (match `git cherry-pick -x` output). The recommended approach (patch modification) is well-understood. The original issue even suggests this approach.
- **Level Indication**: 1-2

#### Testing Requirements
- **Assessment**: Moderate
- **Details**: Unit tests for patch transformation function, existing test patterns to follow. The patch modification function is easily unit-testable with sample patch strings.
- **Level Indication**: 2

#### Backwards Compatibility
- **Assessment**: Fully compatible
- **Details**: New opt-in flag defaulting to false. No behavior change for existing deployments.
- **Level Indication**: 1-2

#### Architectural Alignment
- **Assessment**: Good fit
- **Details**: Approach 1 (patch modification) fits perfectly - it modifies the patch file before the existing `r.Am()` call, requiring no changes to the git abstraction layer.
- **Level Indication**: 1-2

#### External Dependencies
- **Assessment**: None
- **Details**: No external system dependencies. Patch format is standard mbox from GitHub API.
- **Level Indication**: 1-2

### Recommended Labels

- [x] `help-wanted`: Well-scoped feature, suitable for skilled contributors
- [x] `kind/feature`: New functionality request
- [x] `area/plugins`: Affects the cherrypicker external plugin
- [ ] `good-first-issue`: Requires moderate understanding of cherrypicker flow and mbox format

### Guidance for Contributors

**For Level 2 (Moderate)**:
- Suitable for contributors familiar with Go string processing and the cherrypicker plugin
- Should review:
  - `cmd/external-plugins/cherrypicker/server.go`: `handle()` function flow, especially `getPatch()` and `r.Am()`
  - `cmd/external-plugins/cherrypicker/server_test.go`: Existing test patterns with `fghc` and `localgit`
  - Mbox format: `From <sha>` header, message body, `---` separator
- Recommended approach: Modify the patch file in-place before `git am` application
- Key edge case: Commit messages can contain `---` lines; use `diff --git` as the definitive separator

### Caveats and Considerations

- The current PR (#661) takes an approach that has critical issues (deprecated `git filter-branch`, shell injection risk, no tests). Reviewer guidance should steer toward the patch-modification approach.
- The original issue (#500) is from the KubeVirt team who actively uses the cherrypicker and has clear motivation for this feature.

## Proposed Issue Augmentation

### Title Change
- **Current**: "cherrypicker: add a flag to support `git cherry-pick -x` style commit messages for"
- **Proposed**: "cherrypicker: add a flag to support `git cherry-pick -x` style commit messages"
- **Rationale**: Remove trailing "for" which appears to be a truncation

### Proposed GitHub Comment

```
/retitle cherrypicker: add a flag to support `git cherry-pick -x` style commit messages

The patch file that the cherrypicker downloads from GitHub's API (via the `.patch` endpoint) is in mbox format where each commit starts with a `From <sha> Mon Sep 17 00:00:00 2001` header. The original commit SHA is already present in the patch. The simplest implementation approach is to modify the patch file in-place before calling `git am`: parse each commit section, extract the SHA from the `From` header, and insert `(cherry picked from commit <sha>)` into the commit message body before the `---` separator. This avoids post-hoc history rewriting entirely and works within the existing git abstraction layer.

PR #661 implements this feature using `git filter-branch` for post-am message rewriting, but that approach has issues: `filter-branch` is deprecated by the Git project, it bypasses Prow's git abstraction layer (`pkg/git/v2`) by using `os/exec` directly, the generated shell script has a potential injection vector from unvalidated SHAs, and there are no tests. The patch-modification approach described above would be simpler, safer, and more testable.

/area plugins
/kind feature
/help-wanted
```

### Rationale

**What's being added**:
- Technical context about the mbox patch format and where the original SHAs live (not in the original issue)
- Specific recommended implementation approach (patch modification before `git am`)
- Analysis of why the current PR approach has issues, with constructive guidance

**Why these labels**:
- `/area plugins`: The cherrypicker is an external plugin
- `/kind feature`: This is a new functionality request (already labeled)
- `/help-wanted`: Level 2 effort - well-scoped, suitable for skilled contributors

**What's NOT included**:
- Priority label: Not critical enough to warrant one
- Detailed code paths: Would make the comment too verbose for an issue comment

## Briefing Completed

Briefed maintainer on: 2026-04-11

Key questions asked:
- None - maintainer acknowledged all slides without questions

Maintainer decision:
- Proceed with wrapup

## Next Steps

1. Post augmentation comment to issue #500
2. Provide detailed PR review on #661 with specific guidance toward patch-modification approach
3. Consider whether to write a proof-of-concept for the patch modification approach

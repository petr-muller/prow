# Triage for Issue #666

**Status**: In Progress
**Created**: 2026-03-29

## Issue Information

- **Issue Number**: #666
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/666
- **Title**: Allow use of Git subtrees with `mergecommitblocker`
- **Author**: RadaBDimitrova (Rada Dimitrova)
- **Labels**: area/plugins, kind/feature

## Issue Summary

The `mergecommitblocker` plugin blocks all merge commits unconditionally, which prevents Git subtree workflows from functioning. Git subtrees inherently depend on merge commits (created by `git subtree pull`) to track history correctly. The author requests flexibility to exempt certain paths/directories from the merge commit check.

## Initial Validation

**Assessment**: LEGITIMATE

### Analysis

The issue describes a feature request for the `mergecommitblocker` plugin, requesting path-based exclusions so that Git subtree merge commits can pass the check. This is a well-scoped enhancement request for a component that lives in this repository.

**Issue Category**: Feature Request

**Repository Scope Check**:
- Component mentioned: `mergecommitblocker` plugin
- Exists in this repo: Yes (`pkg/plugins/mergecommitblocker/mergecommitblocker.go`)
- Relevant code paths: `pkg/plugins/mergecommitblocker/`

**Information Completeness**:
- Sufficient detail provided: Yes
- The issue clearly explains the problem (unconditional merge commit blocking), the use case (Git subtrees), why the current behavior is incompatible, and suggests a concrete solution approach (path-based exclusions via `excludeDir` config option)

### Recommendation

Keep open and continue triage. This is a valid feature request for the `mergecommitblocker` plugin. The use case is real — Git subtrees require merge commits, and the plugin currently has no mechanism to allow them selectively. The labels (`area/plugins`, `kind/feature`) are already correctly applied.

**Suggested Action**:
- Keep open and continue triage

## Code Research

### Current Implementation

**Primary Components**:
- `mergecommitblocker` plugin: `pkg/plugins/mergecommitblocker/mergecommitblocker.go` — detects merge commits in PRs and applies a blocking label
- Git interactor: `pkg/git/v2/interactor.go:580-589` — implements `MergeCommitsExistBetween()` via `git log <target>..<head> --oneline --merges`

**Architecture Overview**:
The plugin registers as a PullRequest handler. On PR open/reopen/synchronize events, it clones the repo, checks out the PR, and runs `git log --merges` between the base and head SHAs. If merge commits exist and the label is absent, it adds `do-not-merge/contains-merge-commits` and posts an explanatory comment. If merge commits are gone but the label remains, it removes the label and prunes the comment.

**Key Code Paths**:
1. Event filter: `mergecommitblocker.go:67-70` — triggers on Opened, Reopened, Synchronize
2. Merge detection: `mergecommitblocker.go:99-104` — calls `r.MergeCommitsExistBetween(target, head)`
3. Label add: `mergecommitblocker.go:118-124` — adds label + creates comment
4. Label remove: `mergecommitblocker.go:110-117` — removes label + prunes comment
5. Git implementation: `interactor.go:580-589` — `git log <target>..<head> --oneline --merges`

**Configuration**: The plugin has **zero configuration options**. The helpProvider (line 49) explicitly states: "this plugin is not triggered with commands and is not configurable." There is no struct in `pkg/plugins/config.go` for this plugin.

### Related Code

**Similar Functionality — Blockade Plugin**:
The `blockade` plugin (`pkg/plugins/blockade/blockade.go`) is the closest analogue for path-based filtering. It uses:
- `BlockRegexps []string` and `ExceptionRegexps []string` in its config struct (`pkg/plugins/config.go:307-322`)
- Runtime regex compilation in `compileRegexpsAndDurations()` (`config.go:1442-1503`)
- File-level matching: `matchesAny(file, bd.blockRegexps) && !matchesAny(file, bd.exceptionRegexps)` (`blockade.go:128-130`)
- Gets changed files via `GetPullRequestChanges` GitHub API call

**Git Interactor Capabilities**:
- `Diff(head, sha)` at `interactor.go:74` returns changed file paths between two refs
- `MergeCommitsExistBetween()` is a boolean-only check — no way to get the individual merge commit SHAs or the files they touch
- New git operations would be needed to inspect per-merge-commit file changes

### Test Coverage

**Existing Tests**: `pkg/plugins/mergecommitblocker/mergecommitblocker_test.go`
- 4 test cases covering the 2x2 matrix: (label present/absent) × (merge commits present/absent)
- Uses fake GitHub and git clients
- Coverage assessment: **Partial** — covers the core label management logic but no error handling paths

**Test Gaps**:
- No tests for configuration-based behavior (none exists yet)
- No tests for path-based exclusions
- No tests for edge cases like GitHub API failures

### Root Cause Analysis

**Primary Cause**:
The plugin applies an unconditional binary check: merge commits exist → block. There is no configuration mechanism to exempt any merge commits based on their content, affected paths, or commit message patterns.

**Contributing Factors**:
1. The plugin was designed for the Kubernetes ecosystem where linear history is strictly enforced
2. Git subtrees are an uncommon workflow in Kubernetes-adjacent projects, so this use case wasn't considered
3. The plugin has no configuration infrastructure at all — adding any flexibility requires building the config plumbing from scratch

### Proposed Solutions

#### Approach 1: Path-based Exclusion via Merge Commit Inspection

**Description**: Add configuration for excluded path patterns. When merge commits are detected, inspect each merge commit's changed files using `git diff-tree`. If ALL files changed by ALL merge commits fall within excluded paths, allow the PR.

**Pros**:
- Precise: only allows merge commits that exclusively touch excluded paths
- Works when PR has both subtree changes and other (non-merge) changes
- Matches the issue author's mental model

**Cons**:
- Requires new git interactor method (e.g., `ListMergeCommitFiles(target, head)`)
- More complex implementation — needs to parse individual merge commit SHAs and run `git diff-tree` for each
- Slightly more expensive git operations

**Affected Components**:
- `pkg/plugins/config.go`: Add `MergeCommitBlocker` config struct with `ExcludePathRegexps`
- `pkg/plugins/mergecommitblocker/mergecommitblocker.go`: Use config, add path checking logic
- `pkg/git/v2/interactor.go`: Add method to list files changed by merge commits

**Complexity**: Medium

**Backwards Compatibility**: Fully backwards compatible — no config = current behavior

#### Approach 2: PR-level Changed Files Check

**Description**: Add excluded path configuration. When merge commits are detected, check the PR's overall changed files (via GitHub API `GetPullRequestChanges`). If ALL changed files are within excluded paths, allow merge commits.

**Pros**:
- Simpler implementation — uses existing GitHub API, no new git operations needed
- Follows the blockade plugin's established pattern exactly
- Easier to test

**Cons**:
- Less precise: checks all PR files, not just merge commit files
- Won't work if PR has both subtree changes AND non-subtree changes (merge commits would still be blocked even if they only touch subtree paths)
- Subtree PRs that also modify non-subtree files would still be blocked

**Affected Components**:
- `pkg/plugins/config.go`: Add `MergeCommitBlocker` config struct with `ExcludePathRegexps`
- `pkg/plugins/mergecommitblocker/mergecommitblocker.go`: Use config, add GitHub API call for changed files

**Complexity**: Low

**Backwards Compatibility**: Fully backwards compatible

#### Recommendation

**Preferred Approach**: Approach 1 (Merge Commit Inspection)

While more complex, this approach correctly addresses the actual use case. Git subtree PRs often modify files both inside and outside the subtree directory. Approach 2 would still block these PRs, defeating the purpose. The additional complexity of inspecting merge commit files is justified by correctness.

**Key Implementation Considerations**:
1. Config struct should use regex patterns (not glob) for consistency with blockade plugin
2. Need a new git interactor method to list merge commit SHAs and their changed files
3. `git diff-tree --no-commit-id --name-only -r <sha>` can list files per merge commit
4. Empty excluded paths config = current unconditional behavior (backwards compatible)
5. Config validation should compile regexps during `compileRegexpsAndDurations()`

**Testing Requirements**:
- Test: merge commits only touch excluded paths → allowed
- Test: merge commits touch both excluded and non-excluded paths → blocked
- Test: no excluded paths configured → current behavior preserved
- Test: regex pattern matching for paths

## Effort Assessment

**Effort Level**: 2 - Moderate (help-needed)

### Summary

A well-defined feature request that follows established Prow plugin patterns (blockade). Requires adding config plumbing to a currently zero-config plugin and implementing a new git operation, but the path is clear and the change is fully backwards compatible.

### Factor Analysis

#### Scope of Changes
- **Assessment**: Moderate
- **Details**: 5-8 files affected (~200-400 LOC): config struct in `config.go`, plugin logic in `mergecommitblocker.go`, new git method in `interactor.go`, tests for all three, config documentation in `plugin-config-documented.yaml`
- **Level Indication**: 2-3

#### Complexity
- **Assessment**: Moderate
- **Details**: Config plumbing follows existing patterns exactly (blockade). The new git operation (`git diff-tree` per merge commit) is straightforward. Regex compilation follows existing `compileRegexpsAndDurations()` pattern. No concurrency, no race conditions.
- **Level Indication**: 2

#### Required Expertise
- **Assessment**: Moderate
- **Details**: Needs familiarity with Prow plugin config patterns, Go regex handling, and git operations. All patterns can be learned from the blockade plugin as a template.
- **Level Indication**: 2

#### Clarity and Certainty
- **Assessment**: Well-defined with minor design decisions
- **Details**: The problem and general approach are clear. Minor decisions: exact config field naming, regex vs glob patterns (regex preferred for consistency), per-repo vs global config. These are tractable.
- **Level Indication**: 1-2

#### Testing Requirements
- **Assessment**: Moderate
- **Details**: Follow existing test patterns from `mergecommitblocker_test.go`. Need to add scenarios for: excluded paths configured, merge commits touching only excluded paths (allowed), merge commits touching mixed paths (blocked), no config (current behavior preserved). New git interactor tests needed.
- **Level Indication**: 2

#### Backwards Compatibility
- **Assessment**: Fully compatible
- **Details**: No config = current behavior. The change is purely additive — only users who configure excluded paths will see different behavior.
- **Level Indication**: 1

#### Architectural Alignment
- **Assessment**: Good fit
- **Details**: Follows the blockade plugin's established pattern for path-based config. Extends the plugin config system in a standard way. The only new pattern is a git interactor method, which is a natural extension.
- **Level Indication**: 2

#### External Dependencies
- **Assessment**: None
- **Details**: Uses git operations (already available via the interactor) and existing plugin config infrastructure. No external API changes needed.
- **Level Indication**: 1

### Recommended Labels

- [x] `help-wanted`: Well-defined scope, clear approach, good for skilled contributors
- [ ] `good-first-issue`: Too involved for a first contribution — requires config plumbing across multiple files and understanding of plugin patterns

### Guidance for Contributors

- Should review the blockade plugin (`pkg/plugins/blockade/`) as a template for path-based config
- Should review plugin config patterns in `pkg/plugins/config.go`
- Recommended approach: Start with the config struct, then add the git interactor method, then update the plugin logic
- Test with real git subtree operations to validate the approach

### Caveats and Considerations

- The issue author suggests `excludeDir` (simple directory names), but regex patterns are recommended for consistency with existing Prow patterns (blockade uses `ExceptionRegexps`)
- Contributors should consider whether the config should be per-repo or global — per-repo is more flexible and consistent with other plugin configs
- The `git diff-tree` approach for inspecting merge commit files needs careful handling of merge commits with 2+ parents

## Next Steps

(Action items will be added here)

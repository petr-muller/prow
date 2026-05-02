# Triage for Issue #610

**Status**: In Progress
**Created**: 2026-05-03

## Issue Information

- **Issue Number**: #610
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/610

## Initial Validation

**Assessment**: LEGITIMATE

### Analysis

**Issue Summary**: The `owners-label` plugin adds `sig/` and `area/` labels based on all files changed in a PR. When a user mistakenly pushes a merge commit, the PR's diff suddenly includes all files from the merged branch, causing `owners-label` to apply a large number of irrelevant labels. Since `owners-label` only adds labels and never removes them, these erroneous labels persist even after the user force-pushes to remove the merge commit. This pollutes searches, filters, and issue/PR tracking.

**Issue Category**: Feature Request (with bug-like characteristics — the current behavior produces incorrect results in a common scenario)

**Repository Scope Check**:
- Component mentioned: `owners-label` plugin
- Exists in this repo: Yes (`pkg/plugins/owners-label/owners-label.go`)
- Related component: `mergecommitblocker` plugin (`pkg/plugins/mergecommitblocker/mergecommitblocker.go`)
- Both components are maintained in this repository

**Information Completeness**:
- Sufficient detail provided: Yes
- The issue clearly describes the problem, the root cause, and proposes a solution approach
- The discussion in comments refines the approach further

### Comment Discussion Summary

The issue generated a substantive technical discussion (6 comments) between the reporter (danwinship) and a maintainer (BenTheElder):

1. **BenTheElder** endorsed the idea (+1), suggested it might need to be opt-in since not all Prow deployments use `mergecommitblocker`, and clarified why `owners-label` only adds labels (users can add labels via `/sig foo` commands too).

2. **danwinship** initially considered coupling the behavior to whether `mergecommitblocker` is enabled, but then argued the fix should be unconditional: there is **no workflow** where applying labels based on files in a merge commit makes sense. If you merge master into your PR, you'd get labels for every other PR that merged into master since your branch point — that's never useful.

3. **danwinship proposed refined pseudocode**:
   - For each commit in the PR:
     - If it's a merge *from* the target branch, skip it
     - Otherwise, for each file modified by that commit, add labels
   
4. **BenTheElder** was initially thinking about it the other way (if you don't allow merges, no point labeling from them), but danwinship argued the logic holds universally.

5. **Key consensus**: Merge commits should always be skipped for labeling purposes, regardless of configuration. The reasoning is that labeling from merge commits can only produce noise — it reflects upstream changes, not the PR author's work.

### Recommendation

This is a well-articulated feature request with clear technical analysis and maintainer endorsement. The reporter has deep understanding of the problem and proposed a sound solution approach. The discussion converged on a clear design direction.

**Suggested Action**:
- Keep open and continue triage
- The issue is ready for research and implementation planning

## Code Research

### Current Implementation

**Primary Components**:
- `owners-label` plugin: `pkg/plugins/owners-label/owners-label.go` (123 lines) — adds labels based on OWNERS files for changed files in a PR
- `mergecommitblocker` plugin: `pkg/plugins/mergecommitblocker/mergecommitblocker.go` — detects merge commits and blocks merging

**Architecture Overview**:
The `owners-label` plugin registers as a `PullRequestHandler`. On PR open, reopen, or synchronize events, it:
1. Loads OWNERS data for the base branch (`LoadRepoOwners`)
2. Fetches all changed files via GitHub API (`GetPullRequestChanges` — `GET /repos/{o}/{r}/pulls/{n}/files`)
3. Maps each file to labels via `FindLabelsForFile`
4. Adds any new labels that aren't already present

**Key Code Paths**:
1. Event handler: `owners-label.go:58-69` — filters to opened/reopened/synchronize actions
2. File-to-label mapping: `owners-label.go:77-84` — iterates ALL changed files from `GetPullRequestChanges`
3. Label application: `owners-label.go:109-116` — only adds labels, never removes

**Root Cause**:
`GetPullRequestChanges` calls `GET /repos/{o}/{r}/pulls/{n}/files`, which returns the **full diff between the PR head and the base branch**. This endpoint does not distinguish which commits introduced which files. When a merge commit is present, all files from the merged branch appear in this diff, producing a massive set of irrelevant file changes.

### Related Code: Merge Commit Detection Patterns

Two existing plugins already detect and skip merge commits:

1. **`mergecommitblocker`** (`mergecommitblocker.go:79-104`): Uses local git clone + `git log --merges` between base and head SHAs. Heavy approach (requires cloning the repo).

2. **DCO plugin** (`pkg/plugins/dco/dco.go:156-161`): Uses `ListPullRequestCommits()` from GitHub API, then checks `len(commit.Parents) > 1` to identify merge commits. Lightweight approach — no git clone needed.

The DCO pattern is the relevant precedent for this fix.

### API Constraints

- `ListPullRequestCommits()` returns `[]RepositoryCommit` with `Parents` field populated (can detect merges), but `Files` field is **NOT populated** (comment in types.go:1354-1355: "Only filled in during GetCommit!")
- To get per-commit file lists, would need to call `GetCommit()` for each non-merge commit individually — expensive for PRs with many commits
- `GetPullRequestChanges()` returns per-PR file list — no per-commit breakdown available

### Test Coverage

**Test file**: `pkg/plugins/owners-label/owners-label_test.go`
- 11 test cases covering various file-to-label mapping scenarios
- Uses `FakeClient` and a mock `ownersClient`
- **No test cases involving merge commits** — this is a gap that a fix would need to address

### Proposed Solutions

#### Approach 1: Skip labeling when merge commits are present (Simple)

**Description**: Before processing files, call `ListPullRequestCommits()`. If any commit has `len(Parents) > 1`, skip the entire labeling operation for this event. Labels will be correctly applied when the user force-pushes to remove the merge commit (triggering a new `synchronize` event).

**Pros**:
- Very simple to implement (~10 lines of code)
- Follows established pattern (DCO plugin)
- No extra API calls per commit
- One additional API call total (`ListPullRequestCommits`)
- Correct behavior: after force-push, `synchronize` fires again without merge commits, labels applied normally

**Cons**:
- Skips ALL labeling for the event, even labels from legitimate non-merge commits
- If a user intentionally pushes a merge commit and never removes it, labels are never applied
- Slightly coarse-grained

**Complexity**: Low

**Backwards Compatibility**: Changes behavior for PRs with merge commits (which is the intended fix). No impact on PRs without merge commits.

#### Approach 2: Per-commit file analysis (Granular, as proposed by danwinship)

**Description**: Call `ListPullRequestCommits()` to get commits. For each non-merge commit, call `GetCommit()` to get its per-commit file list. Only use files from non-merge commits for label computation.

**Pros**:
- Most precise: labels are applied for legitimate changes even if merge commits exist
- Matches danwinship's proposed pseudocode exactly

**Cons**:
- Requires N additional API calls (one `GetCommit` per non-merge commit)
- Could be expensive for PRs with many commits (API rate limiting)
- `GetPullRequestChanges` is paginated and efficient; N individual `GetCommit` calls are not
- More complex implementation

**Complexity**: Medium

**Backwards Compatibility**: Same as Approach 1 for merge commits. Labels for non-merge commits still applied.

#### Approach 3: Compare commit-level files with PR-level files (Hybrid)

**Description**: Use `GetPullRequestChanges()` as today to get the full file list. Also call `ListPullRequestCommits()`. If no merge commits exist, proceed as normal. If merge commits exist, fall back to per-commit analysis (Approach 2) or skip entirely (Approach 1).

**Pros**: Optimizes for the common case (no merge commits = no behavior change)
**Cons**: Adds complexity for marginal benefit over Approach 1

**Complexity**: Medium

#### Recommendation

**Preferred Approach**: Approach 1 (Skip labeling when merge commits present)

**Rationale**:
- Merge commits in PRs are an error condition in the overwhelming majority of Prow deployments (which is why `mergecommitblocker` exists)
- When a user pushes a merge commit, `mergecommitblocker` immediately flags it and the user force-pushes — the labels will be applied on the subsequent `synchronize` event
- The per-commit approach (Approach 2) is more precise but adds N API calls for a scenario that's transient by nature
- Approach 1 is simple, easy to review, easy to test, and solves the actual problem
- The DCO plugin validates this pattern as established practice in the codebase

**Key Implementation Considerations**:
1. Add `ListPullRequestCommits` to the `githubClient` interface in `owners-label.go`
2. Check for merge commits before calling `GetPullRequestChanges` (to save the API call when skipping)
3. Log a warning when skipping labeling due to merge commits
4. Add test cases for: PR with merge commits (skip), PR without merge commits (normal), PR with merge commits removed on force-push (labels applied)

**Testing Requirements**:
- Test that labeling is skipped when merge commits are detected
- Test that labeling proceeds normally when no merge commits exist
- Ensure the fake GitHub client supports `ListPullRequestCommits`

## Next Steps

- Assess effort level
- Augment the issue with findings

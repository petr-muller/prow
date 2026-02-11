# Triage for Issue #617

**Status**: In Progress
**Created**: 2026-02-11

## Issue Information

- **Issue Number**: #617
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/617
- **Title**: Add a plugin to block merging PRs with `fixup!` commits
- **Author**: nojnhuh
- **Labels**: area/plugins, kind/feature
- **State**: OPEN

## Issue Summary

The author uses `git commit --fixup` during iterative review, then `git rebase --autosquash` before merging. They sometimes forget to `/hold` PRs and merge with `fixup!` commits still present. They request a plugin similar to `mergecommitblocker` that would:
- Detect commits whose messages start with `fixup!` or `amend!`
- Automatically add a `do-not-merge/*` label
- Remove the label when no such commits exist (e.g., after `git rebase --autosquash`)

## Initial Validation

**Assessment**: LEGITIMATE

### Analysis

The issue requests a feature to automatically block merging of PRs that contain `fixup!` or `amend!` commits. This is a well-defined, practical feature request for the Prow plugin ecosystem.

**Issue Category**: Feature Request

**Repository Scope Check**:
- Component mentioned: Prow plugins (similar to `mergecommitblocker`)
- Exists in this repo: Yes — both `invalidcommitmsg` (`pkg/plugins/invalidcommitmsg/`) and `mergecommitblocker` (`pkg/plugins/mergecommitblocker/`) are plugins in this repository
- Relevant code paths: `pkg/plugins/invalidcommitmsg/`, `pkg/plugins/mergecommitblocker/`

**Information Completeness**:
- Sufficient detail provided: Yes
- Use case clearly described: Author uses `git commit --fixup` workflow and sometimes forgets to squash before merge
- Desired behavior well-defined: Detect `fixup!`/`amend!` prefixes, add `do-not-merge/*` label, remove label when commits are cleaned up
- Reference to existing similar plugin (`mergecommitblocker`) as model

**Maintainer Input**: A maintainer (petr-muller) has already commented suggesting this could be added to the existing `invalidcommitmsg` plugin rather than creating a new one.

### Recommendation

This is a legitimate feature request. The use case is common in iterative code review workflows. The requested functionality fits naturally within Prow's plugin architecture, and there's already a closely related plugin (`invalidcommitmsg`) that handles similar commit message validation with the same `do-not-merge/invalid-commit-message` label.

**Suggested Action**:
- Keep open and continue triage
- Investigate whether extending `invalidcommitmsg` (as the maintainer suggested) is the right approach

## Code Research

### Current Implementation

**Primary Components**:
- `invalidcommitmsg` plugin: `pkg/plugins/invalidcommitmsg/invalidcommitmsg.go` — validates commit messages and PR titles against two hardcoded regex patterns (close-issue keywords and @mentions), applies `do-not-merge/invalid-commit-message` label
- `mergecommitblocker` plugin: `pkg/plugins/mergecommitblocker/mergecommitblocker.go` — detects merge commits by cloning the repo and running `git log --merges`, applies `do-not-merge/merge-commits` label

**Architecture Overview**:
Both plugins register as `PullRequestHandler` via `plugins.RegisterPullRequestHandler()` in their `init()` functions. They respond to PR events (opened, reopened, synchronize), check commits for specific patterns, and add/remove `do-not-merge/*` labels accordingly.

**Key Code Paths**:
1. Plugin registration: `invalidcommitmsg.go:69-71` — registers with plugin framework
2. Core handler: `invalidcommitmsg.go:101-185` — fetches commits via GitHub API, checks patterns, manages labels and comments
3. Commit retrieval: `invalidcommitmsg.go:120-124` — uses `ListPullRequestCommits()` (GitHub REST API, no repo clone needed)
4. Pattern matching: `invalidcommitmsg.go:126-131` — iterates commits, checks each message against regexes
5. Label add/remove: `invalidcommitmsg.go:137-156` — idempotent label management with current state check
6. Comment management: `invalidcommitmsg.go:158-182` — prunes old comments before adding new ones

**Data Flow**:
1. PR event fires → plugin framework dispatches to enabled handlers
2. Handler filters by action type (opened/reopened/synchronize/edited)
3. Fetches current labels to check if `do-not-merge/invalid-commit-message` already exists
4. Fetches all PR commits via GitHub API (`ListPullRequestCommits`)
5. Checks each commit message against regex patterns
6. Also checks PR title against the same patterns
7. Adds label if invalid content found and label absent; removes label if all content valid and label present
8. Creates/prunes explanatory comments

### Related Code

**Dependencies**:
- `github.Client`: `ListPullRequestCommits()`, `AddLabel()`, `RemoveLabel()`, `GetIssueLabels()`, `CreateComment()`
- `plugins.Agent`: Entry point providing GitHub client, logger, comment pruner
- `dco` package: `MarkdownSHAList()` for formatting commit lists in comments

**Similar Functionality**:
- `mergecommitblocker`: Same label-based merge blocking pattern but clones repo to use `git log --merges`
- Key difference: `invalidcommitmsg` is API-only (no git clone), making it lighter weight

**Comparison Table**:

| Aspect | `invalidcommitmsg` | `mergecommitblocker` |
|--------|-------------------|---------------------|
| Commit source | GitHub REST API | Clones repository |
| Detection method | Regex on message text | `git log --merges` |
| PR title check | Yes | No |
| Events | opened, reopened, sync, edited | opened, reopened, sync |
| Comment pruning | Yes (aggressive) | No |

### Test Coverage

**Existing Tests**:
- `invalidcommitmsg_test.go` (282 lines): 10+ test cases covering valid/invalid commits, label add/remove, PR title validation, comment creation
- Test pattern uses `fakegithub.NewFakeClient()` and table-driven tests with struct fields for action, commits, title, expected label changes, and expected comments
- Coverage: Good — tests label addition, label removal, comment pruning, edge cases like email addresses not matching @mention regex

**Test Gaps**:
- No tests for `fixup!`/`amend!` patterns (they don't exist yet)

### Root Cause Analysis

**Primary Cause**:
This is a feature gap, not a bug. The `invalidcommitmsg` plugin only checks for two patterns (close-issue keywords and @mentions). There is no mechanism to detect `fixup!` or `amend!` commit message prefixes, which are common in iterative review workflows using `git commit --fixup`.

**Contributing Factors**:
1. The plugin's regex patterns are hardcoded — no configuration system to add custom patterns
2. No other plugin covers this use case
3. The `mergecommitblocker` plugin shows precedent for commit-based merge blocking but uses a heavier repo-cloning approach

### Proposed Solutions

#### Approach 1: Extend `invalidcommitmsg` Plugin

**Description**: Add a new regex pattern to the existing `invalidcommitmsg` plugin to detect `fixup!` and `amend!` commit message prefixes. Reuse the existing label (`do-not-merge/invalid-commit-message`), commit listing, and label management infrastructure.

**Pros**:
- Minimal code change — add one regex and update the checking logic
- Reuses existing infrastructure (commit fetching, label management, comment pruning)
- No new plugin registration, configuration, or documentation needed
- Consistent with the maintainer's suggestion
- API-based (no repo cloning overhead)

**Cons**:
- Less granular control — users who want `invalidcommitmsg` but NOT fixup detection can't separate them
- Same label used for different concerns (may confuse users about what to fix)
- Comment text may need to differ from existing close-issue/mention messages

**Affected Components**:
- `pkg/plugins/invalidcommitmsg/invalidcommitmsg.go`: Add regex, update check logic, add comment template
- `pkg/plugins/invalidcommitmsg/invalidcommitmsg_test.go`: Add test cases

**Complexity**: Low

**Backwards Compatibility**: Fully backwards compatible — existing behavior unchanged. New fixup detection is additive.

#### Approach 2: New Standalone Plugin

**Description**: Create a new plugin (e.g., `fixupcommitblocker`) modeled after `mergecommitblocker` but using the API-based approach from `invalidcommitmsg`. Would have its own label (e.g., `do-not-merge/fixup-commits`).

**Pros**:
- Independently configurable per repo — enable/disable separately from `invalidcommitmsg`
- Distinct label makes it clear what the problem is
- Clean separation of concerns

**Cons**:
- More boilerplate code (new plugin registration, help provider, full handler)
- Duplicates commit-fetching and label management patterns
- More documentation and configuration surface area
- Higher maintenance burden

**Affected Components**:
- New directory: `pkg/plugins/fixupcommitblocker/`
- Plugin registration in `pkg/plugins/plugins.go` (or auto-registration via `init()`)
- Documentation updates

**Complexity**: Medium

**Backwards Compatibility**: Fully backwards compatible — entirely new functionality.

#### Recommendation

**Preferred Approach**: Approach 1 (Extend `invalidcommitmsg`)

This is the simpler, more pragmatic solution. The `invalidcommitmsg` plugin already handles commit message validation with label-based merge blocking, and `fixup!`/`amend!` detection is conceptually the same kind of check. The implementation would involve:

1. Adding a new regex: `regexp.MustCompile(`^(fixup|amend)! `)` (or similar)
2. Adding the check in the existing commit iteration loop
3. Adding a new comment template explaining fixup commits
4. Adding test cases

**Key Implementation Considerations**:
1. The regex should match `fixup!` and `amend!` at the start of commit messages (consistent with `git commit --fixup` behavior which produces `fixup! <original subject>`)
2. PR title should probably NOT be checked for fixup patterns (unlike close-issue keywords)
3. The explanatory comment should guide users to run `git rebase --autosquash` to resolve
4. Consider also detecting `squash!` commits (produced by `git commit --squash`), which serve a similar purpose

**Testing Requirements**:
- Commits with `fixup!` prefix → label added
- Commits with `amend!` prefix → label added
- Commits with `squash!` prefix → label added (if included)
- Mixed valid and fixup commits → label added
- Fixup commits removed (post-rebase) → label removed
- Text containing "fixup" not at start of message → no match

## Next Steps

- Assess effort level for extending `invalidcommitmsg`
- Draft augmentation comment for the issue

---
pr: kubernetes-sigs/prow#661
title: "Add support for git cherry-pick -x style commit messages"
head_sha: 9eb853108e
base: main
reviewed_at: 2026-04-30T12:38:55Z
verdict: request-changes
refresh_log: []
---

# PR #661: Add support for git cherry-pick -x style commit messages

**Author:** @AaruniAggarwal
**Branch:** `cherrypick-pr500` → `main`
**Files:** `main.go`, `server.go`, `server_test.go`
**Changes:** +370 / -12
**Resolves:** #500
**Reviewer:** @stmcginnis
**Commits:** 8 (latest: `9eb85310` — 2026-05-12)

## What this PR does

Adds an `--add-original-commit-id` flag (default `true`) to the cherrypicker plugin. After `git am` applies a patch, the PR parses the patch file for original commit SHAs, generates a shell script, and runs `git rebase -i --exec` to amend each commit message with `(cherry picked from commit <sha>)`.

Previous revision used `git filter-branch --msg-filter`; that was replaced in `4a0d9fdc` after @stmcginnis flagged it as deprecated.

## Review revision history

- **2026-04-10** — Initial review against commits `68fc96ba..f7bdaac1` (3 commits). Implementation used `git filter-branch --msg-filter`.
- **2026-04-12** — Updated review for commit `4a0d9fdc` ("Addressing review comments"). Author replaced `filter-branch` with `git rebase -i --exec`. SHA extraction now uses regex. Temp file handle fixed. Core design issues remain.
- **2026-04-28** — Updated review for commits `6b0d4358` ("Fixing indentation") and `ac830c0f` ("removing unnecessary if block"). Indentation and dead-code tautology resolved. @stmcginnis gave `/lgtm` on Apr 23 noting remaining issues as non-blocking.
- **2026-05-12** — Branch rebased onto current main. Two new commits: `57e0a0d6` ("Adding unit testcases") and `9eb85310` ("Fixing golang-cli lint check"). Unit tests added for both `extractOriginalSHAs` and `appendCherryPickMessages`. Finding #4 partially addressed. Previous `/lgtm` removed by bot after new commits. Design issues (#2, #3, #5) still unaddressed.

## Findings

### Open

#### Critical

**#2 — Direct os/exec bypasses codebase git abstraction**
`server.go:25, 795–833`

The codebase has a git execution layer in `pkg/git/v2/executor.go` that handles logging, credential censoring, and consistent error handling. All existing git operations go through the `Interactor` interface (e.g., `r.Am()`, `r.CheckoutNewBranch()`). This PR adds raw `exec.Command("git", ...)` calls (for both `git rev-parse` and `git rebase -i`), bypassing all of that. The patch-modification approach eliminates the need for any additional git commands entirely.

Status after `9eb85310`: Unchanged. Tests also use `os/exec` directly rather than the codebase's `localgit` helpers.

**#3 — Shell injection risk in generated exec script**
`server.go:800–812`

SHAs are passed via the `ORIGINAL_SHAS` environment variable, which is safer for the SHAs themselves. However, the script still has unsafe unquoted expansions:
- `$ORIGINAL_SHA` is unquoted in the `if [ -n ... ]` test
- `$CURRENT_MSG` is unquoted in the `git commit --amend -m` argument — a commit message containing shell metacharacters (backticks, `$()`, double quotes) will be interpreted by the shell
- The `baseSHA` variable (from `git rev-parse` output) is interpolated directly into the script body via `fmt.Sprintf` without validation

The SHA validation regex in `extractOriginalSHAs` mitigates the SHA vector (guaranteed hex), but the commit message expansion and baseSHA interpolation remain exploitable. With the patch-modification approach, there is no shell script at all.

#### Important

**#5 — Default true is a silent behavior change**
`main.go:76`

`--add-original-commit-id` defaults to `true`. Every existing deployment will start modifying cherry-pick commit messages on upgrade without any opt-in. New features should default to `false` to avoid surprising existing users.

Status after `9eb85310`: Unchanged. Still defaults to `true`.

#### Minor

**#6 — Whitespace corruption (pre-existing)**
`server.go:597`

The `// Push the new branch` comment uses spaces for indentation instead of tabs. This is a pre-existing issue in the file, not introduced by this PR. @stmcginnis flagged it (Apr 21) as "over indented" but it's outside the PR's diff. Not the author's responsibility to fix.

**#8 — Failures silently swallowed**
`server.go:585–597`

Both SHA extraction and commit rewriting failures are logged as `Warn` and ignored. If a user enabled this feature and it silently fails, the cherry-pick PR will lack the expected commit IDs with no visible indication. Consider adding a PR comment noting the failure, or failing the operation outright.

Status after `9eb85310`: Unchanged.

### Resolved

**#1 — git filter-branch replaced with rebase -i --exec** *(Partially)*
`server.go:795–833` — No longer deprecated, but still a heavyweight post-am history rewrite. The shell script approach introduces unnecessary complexity compared to the patch-modification approach.

**#4 — Test coverage added** *(Partially)*
`server_test.go:1462–1695` — ~230 lines of tests added in commit `57e0a0d6`:
- `TestExtractOriginalSHAs` — table-driven, 4 cases
- `TestAppendCherryPickMessages_Empty` — empty SHA list no-op
- `TestAppendCherryPickMessages_NoGitRepo` — error path
- `TestAppendCherryPickMessages_SingleCommit` — real git repo, verifies message annotation
- `TestAppendCherryPickMessages_MultiCommit` — two commits, verifies both messages and order

Remaining gaps: no test for `addOriginalCommitID: false` path, no test for shell metacharacters in commit messages (would expose #3), tests use `os/exec` instead of `localgit` helpers, mixed `require`/`assert` testify usage.

**#7 — SHA validation added to patch parsing**
`server.go:752` — Now uses `regexp.MustCompile('^From ([0-9a-f]{40}) ')` to match only valid 40-character hex SHAs.

**#9 — File handle leak fixed**
`server.go:816–822` — Uses `tmpfile.WriteString()` followed by explicit `tmpfile.Close()` before passing the path to `git rebase`.

**#10 — Dead-code tautology removed**
`server.go:780–783` — Removed in `ac830c0f` after @stmcginnis flagged it.

**#11 — Indentation fixed in new functions**
`server.go:742–833` — Fixed in `6b0d4358` after @stmcginnis flagged it. Now uses tabs throughout.

## Suggested alternative approach

Instead of post-processing commits with `rebase -i --exec`, modify the patch content *before* calling `r.Am()`:

1. Read the patch file into memory
2. For each commit section (delimited by `From <sha> ...` headers), locate the commit message body and append `(cherry picked from commit <sha>)` before the `---` separator (using `diff --git` as the definitive delimiter to handle messages containing `---`)
3. Write the modified patch back to the same path
4. Let the existing `r.Am()` apply it — messages are already correct

This eliminates: `os/exec` imports, `rebase -i`, shell script generation, temp files, the injection risk, and the file handle leak. It's pure Go string manipulation on a file already in memory, and is trivially unit-testable.

The latest revision replaced `filter-branch` with `rebase -i --exec`, which is a step in the right direction (no longer deprecated), but the fundamental design concern remains: post-am history rewriting via shell scripts is unnecessarily complex and fragile when pre-am patch modification achieves the same result with zero external commands.

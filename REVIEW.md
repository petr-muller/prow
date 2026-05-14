---
pr: kubernetes-sigs/prow#719
title: "mergecommitblocker: allow use of Git subtrees with `excluded_paths` config option"
head_sha: 941664570326bd59dfcbbdb62cc66f6c874f203f
base: main
reviewed_at: 2026-05-14T00:40:03Z
verdict: request-changes
---

## Findings

### [blocking] MergeCommitBlocker missing from HasConfigFor()
- where: `pkg/plugins/config.go:2371-2460`
- concern: `HasConfigFor()` enumerates every plugin config using the `Repos` pattern (Approve, Lgtm, Triggers, Welcome). MergeCommitBlocker follows the same shape but is not registered. In multi-shard deployments, sharding logic uses this method to determine which shard owns config for which repos. Without registration, excluded paths config could silently not apply. Also missing from the `reflect.DeepEqual` check at line 2372. Follow the Lgtm pattern (lines 2422-2430).

### [should-fix] diff-tree -m can produce duplicate filenames
- where: `pkg/git/v2/interactor.go:622-634`
- concern: The `-m` flag diffs a merge commit against each parent separately, so a file changed relative to both parents appears twice in output. Harmless for the current caller (regex matching is idempotent on duplicates) but undocumented. Add a one-line comment explaining `-m` behavior.
- excerpt: |
    out, err := i.executor.Run("diff-tree", "-m", "--no-commit-id", "--name-only", "-r", sha)

### [should-fix] Missing error test case for MergeCommitSHAsBetween
- where: `pkg/git/v2/interactor_test.go:2071-2135`
- concern: Test table covers "no merges", "one merge", "multiple merges" but no error case. The sibling CommitChangedFiles test includes an error case. Should be consistent.

### [nit] Log rejected file list at Debug level
- where: `pkg/plugins/mergecommitblocker/mergecommitblocker.go:170-172`
- concern: When a merge commit is disallowed, only the SHA is logged. Including the file list at Debug level would aid operator diagnosis.

### [question] Interactor interface expansion — release notes?
- where: `pkg/git/v2/interactor.go:77-80`
- concern: Two new methods on public Interactor interface. External implementors will fail to compile. Should this be documented in release notes?

### [question] diff-tree -m vs --first-parent for merge commit file listing
- where: `pkg/git/v2/interactor.go:624`
- concern: `-m` diffs against each parent, so files changed on master between branch point and merge appear as "changed." For subtree merges this is fine, but would `--first-parent` be more precise?

## Checked
- Backward compatibility: unconfigured repos hit fast path, identical behavior
- Regex compilation at config load time with clear errors
- CompiledExcludedPaths tagged `json:"-"`
- MergeCommitBlockerFor uses index-based range, returns pointer to slice element
- isMergeCommitAllowed is pure, independently testable, good edge case coverage
- Integration tests use real git via localgit
- Error wrapping consistent with %w
- Config lookup pattern matches LgtmFor/TriggerFor precedent
- No security concerns: SHAs from git log output
- Deployment risk low: no migration, safe rollback, negligible perf impact

## Open questions
- Have you considered edge cases with unusual merge topologies where `-m` might over-report changed files? Would `--first-parent` be more precise?
- Should the Interactor interface expansion be called out in release notes?

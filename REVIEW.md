---
pr: kubernetes-sigs/prow#478
title: "branchprotector: remove protection from excluded branches"
head_sha: f1b83a2b61ff3db5fc7eaaa77b6d728c0bea001d
base: main
reviewed_at: 2026-05-13T23:24:41Z
verdict: request-changes
refresh_log:
  - previous_sha: f1b83a2b61ff3db5fc7eaaa77b6d728c0bea001d
    new_sha: f1b83a2b61ff3db5fc7eaaa77b6d728c0bea001d
    summary: "No code changes. Created REVIEW.md from existing REVIEW.html. Incorporated PR discussion: smg247 independently raised the same semantic-change concern (2025-07-11), Prucek gave /lgtm (2025-06-18)."
---

# PR #478 -- branchprotector: remove protection from excluded branches

**State:** OPEN | **Author:** kaovilai | **Diff:** +41/-0
**Link:** https://github.com/kubernetes-sigs/prow/pull/478

## Verdict: Request Changes -- semantic change to `exclude` needs an opt-in mechanism

## What this PR does

When branches are added to the `exclude` list, branchprotector correctly skips applying new protection rules but leaves existing protection in place. This PR adds logic to actively remove protection from excluded branches by sending a `nil` Request through the existing `updates` channel.

**Call flow:** `UpdateRepo()` -> `Request: nil` -> `p.updates chan` -> `configureBranches()` -> `RemoveBranchProtection()`

### Changed files

| File | Change |
|------|--------|
| `cmd/branchprotector/protect.go` | Collects `allBranches`; after main loop, iterates excluded+protected branches and sends removal requests |
| `cmd/branchprotector/protect_test.go` | Adds expected `Request: nil` entries to two existing test cases |

### Since initial review

No code changes (head SHA unchanged). PR discussion confirmed the review's primary concern:

- **Prucek** reviewed code (2025-06-17): asked about the `seen` map, resolved via discussion. Gave `/lgtm` on 2025-06-18.
- **smg247** (2025-07-11): independently raised the same semantic-change concern -- this would make it impossible to set branch protection outside of Prow for excluded branches, calling it "overreaching."
- **kaovilai** responded that Prow already overrides external protection for non-excluded branches, and proposed that Prow should only update fields that are explicitly mentioned in config.
- **smg247** agreed that only-update-mentioned-fields would be ideal.
- Author has kept the PR alive through multiple lifecycle/stale cycles (latest removal: 2026-05-13).

The discussion between smg247 and kaovilai suggests a third possible delivery path beyond the two in the original review: make branchprotector only update explicitly-configured fields, leaving unmentioned fields untouched. This would be a larger change but addresses the root cause.

## Reviewer Perspectives

Three independent reviewers analyzed this PR. Their verdicts diverge, driven primarily by the deployment risk assessment.

- **Code Quality (CQ):** APPROVE -- no critical issues, logic correct, follows existing patterns, `allBranches` could be simplified, missing test for excluded + unprotected.
- **Maintainability (MT):** COMMENT -- low maintenance burden, fragile ordering dependency in `allBranches`, pre-existing mock bug now load-bearing, cleanly revertible.
- **Deployment Risk (DR):** HIGH RISK -- semantic change to `exclude`, destructive + irreversible on GitHub, no opt-in mechanism, affects all existing `exclude` configs, respects `--confirm` flag.

## Converging Concerns

### `allBranches` duplication and ordering fragility (CQ + MT)

The `allBranches` slice appends from both `GetBranches` calls, duplicating every protected branch. Correctness depends on the `onlyProtected=true` call coming second so `Protected=true` entries are seen after `Protected=false` ones. If the loop order were ever changed, exclusion removal would silently stop working with no test catching it.

**Resolution:** Build the removal candidate list only from the `onlyProtected=true` results, eliminating both the duplication and the `seen` map.

### Missing test: excluded + unprotected branch (CQ + MT)

No test exercises an excluded branch that is not currently protected. All test branches default to `Protected: true`. A test with `startUnprotected: true` would verify the code doesn't emit spurious removal requests.

### Pre-existing `fakeClient.GetBranches` bug is now load-bearing (CQ + MT + DR)

`protect_test.go:115-133` -- The mock iterates range copies, so neither `onlyProtected` filtering nor `Protected = false` clearing actually takes effect. Tests pass by accident. The new feature's correctness depends on the `Protected` field, so the mock gap is now load-bearing.

## Correctness Analysis

### Two-call API pattern & the `Protected` field

`UpdateRepo` calls `GetBranches` twice: first with `onlyProtected=false` (returns all branches, `Protected` may be unreliable), then with `onlyProtected=true` (returns only protected branches, `Protected=true`). The new code appends both results into `allBranches` and deduplicates with a `seen` map.

This works correctly in practice. The `seen` guard only fires when `b.Protected == true`, so:
- If the first call has `Protected=false` (wrong for a protected branch), it doesn't match -> skipped
- The second call has `Protected=true` (correct) and `seen` is still false -> processes it
- Unprotected branches: `Protected=false` from first call, absent from second -> correctly ignored

### Interaction with explicit branch configs

Branches explicitly listed in `repo.Branches` are added to the `branches` map regardless of exclusions. The `!inBranches` check in the new code correctly skips them.

### Channel ordering

Removal requests are sent to `p.updates` before the normal `UpdateBranch` loop. Test expectations list removal entries first, which is consistent.

## Deployment Risk -- The Key Concern

This PR changes the meaning of the `exclude` directive:

|  | Before | After |
|--|--------|-------|
| **Meaning** | "Do not manage this branch" -- leave existing protection alone | "Actively remove protection from this branch" |
| **Effect on protected excluded branches** | Protection remains intact | Protection is removed on next `--confirm` run |
| **Reversibility** | N/A | Irreversible on GitHub -- must re-apply protection manually |

Operators who use `exclude` to freeze protection on certain branches (e.g., `release-.*`) while iterating on rules for other branches will have that protection silently stripped. This is destructive and irreversible.

**Validation from PR discussion:** smg247 independently raised this exact concern on 2025-07-11, confirming this is not a theoretical risk.

### Mitigations present

- Respects the `--confirm` flag (dry-run without it)
- `!inBranches` guard protects explicitly configured branches
- Uses the existing, well-tested `nil Request` path in `configureBranches()`
- Logs at INFO level when removing protection

### Possible safer delivery paths

1. **Opt-in flag**: Add `--remove-excluded-protection` defaulting to `false`. Operators explicitly opt in.
2. **Use `protect: false` at branch level**: The existing mechanism for active removal. `exclude` would remain "skip entirely."
3. **Only update mentioned fields** (raised by kaovilai in PR discussion): Make branchprotector only update fields explicitly set in config, leaving unmentioned fields untouched. Larger change but addresses root cause.
4. **WARNING-level logging**: At minimum, make removals highly visible in dry-run output.
5. **Release notes**: Document the behavioral change prominently if the opt-in approach is adopted.

## Detailed Findings

### #1 -- Pre-existing mock bug (not introduced by this PR) (CQ + MT + DR)

`protect_test.go:115-133` -- The mock `GetBranches` iterates range copies, so neither the `onlyProtected` filtering nor the `Protected = false` clearing actually takes effect. Both calls return the same unmodified slice. Tests pass because `startUnprotected` defaults to `false`, making all branches `Protected: true`. Since the new feature depends on `Protected`, consider fixing the mock.

### #2 -- `allBranches` collects more than needed (CQ + MT)

`protect.go:329,335` -- `allBranches` appends branches from both API calls, duplicating every branch. Since removal only applies to branches that are currently protected, you only need the result of the `onlyProtected=true` call. This eliminates the `seen` map entirely.

### #3 -- Verbose call-flow comment (CQ)

`protect.go:365` -- The comment `// Flow: updates channel -> configureBranches() -> client.RemoveBranchProtection()` traces a call path that is apparent from reading the code. The `// nil Request triggers RemoveBranchProtection` comment on the line above is more useful. Consider removing only the flow comment.

### #4 -- Missing test: include + exclude interaction (CQ)

When both `Include` and `Exclude` are configured, exclusions are never checked (inclusions handle everything). No test verifies what happens to the removal logic in this case.

## Required Changes

1. **Gate the removal behavior behind an opt-in mechanism** -- The `exclude` directive currently means "skip this branch." Silently changing it to "actively strip protection" is a breaking change. Either introduce a flag like `--remove-excluded-protection` (defaulting to false), or require operators to use explicit `protect: false` at the branch level for active removal. (DR)
2. **Add a test for excluded + unprotected branches** -- Use `startUnprotected: true` with an exclusion pattern to verify no removal is attempted. (CQ + MT)
3. **Use WARNING-level logging for removals** -- Active removal of protection is a significant operation. The operator should see clear log output at WARNING level, not INFO. (DR)

## Non-Blocking Suggestions

- Refactor `allBranches` to collect only from the `onlyProtected=true` call, eliminating the `seen` map and the ordering dependency (CQ + MT)
- Add a test for the `include` + `exclude` interaction on the removal path (CQ)
- Fix the pre-existing `fakeClient.GetBranches` mock to properly filter and clear `Protected` (CQ + MT + DR)
- Remove the verbose `// Flow: ...` comment; keep the `// nil Request triggers ...` one (CQ)
- Document the behavioral change in release notes when the opt-in mechanism is added (DR)

## Draft PR Comment

The implementation is clean and follows existing codebase patterns, but it introduces a breaking semantic change to the `exclude` directive: from "do not manage this branch" to "actively remove protection from this branch." Operators relying on `exclude` to preserve existing manual protection will have it silently stripped on the next `--confirm` run.

I would like to see this gated behind an opt-in mechanism (a flag or explicit config) rather than changing the default behavior of `exclude`. Additionally, the test suite needs a case for an excluded branch that is not currently protected, and WARNING-level logging should accompany any protection removal.

The underlying fix for the behavioral gap is valuable -- it just needs a safer delivery path.

---
pr: kubernetes-sigs/prow#698
title: "feat: manage non-k8s members assigning issues"
head_sha: 773c479cad28d019abfabee63bdba190d2544280
base: main
reviewed_at: 2026-05-14T00:34:26Z
verdict: request-changes
refresh_log:
  - previous_sha: 773c479cad28d019abfabee63bdba190d2544280
    new_sha: 773c479cad28d019abfabee63bdba190d2544280
    summary: "No new commits. Incorporated review feedback from Amulyam24 (2026-05-13): error handling UX, PluginConfig parameter scope, variable naming, message formatting."
---

## Findings

### [blocking] Unused `GetRepo` in `githubClient` interface
- where: `assign.go:76`
- concern: Added to `githubClient` but never called in production code. Widens the interface and forces all implementations and mocks to satisfy it for no reason.

### [blocking] Wrong membership check target — self-assign guard missing
- where: `assign.go:167`
- concern: `IsMember(org, e.User.Login)` checks the commenter, not the assignees. `/assign @org-member` by a non-member fires the message incorrectly; `/assign @non-member` by a member skips it. The check must verify the commenter is actually in `toAdd` (self-assignment only).

### [blocking] `GetIssueLabels` error swallowed — false positives
- where: `assign.go:154-156`
- concern: When label fetch fails, `hasGoodFirstIssue` defaults to `false` and the code proceeds to check membership and potentially post an incorrect educational comment. Should fail open: skip the educational path entirely on error.

### [blocking] Hardcoded `github.com` URLs
- where: `assign.go:180, 186, 190`
- concern: Generated comment links use hardcoded `github.com` instead of the configured host. Prow installations on GitHub Enterprise get broken links. The default URL comparison at line 186 also creates hidden coupling to the help plugin's default.

### [blocking] PR description vs. code behavior mismatch
- where: `assign.go:197`
- concern: PR says "restrict self-assignment for non-organization members" but code still calls `h.add()` unconditionally — the user gets assigned and just sees a comment. Either the description is wrong or the code should skip assignment.

### [blocking] Triggers on PRs unnecessarily
- where: `assign.go:153`
- concern: `userType == "assignee(s)"` doesn't exclude PRs. `/assign` on a PR calls `GetIssueLabels` + `IsMember` + `GetFile` for no reason — PRs don't carry `good-first-issue` labels. Gate on `!e.IsPR`.

### [should-fix] 1-4 extra API calls per `/assign`, not cached
- where: `assign.go:153-196`
- concern: `GetIssueLabels` on every assign, then conditionally `IsMember` + up to 2x `GetFile`. At scale this meaningfully increases GitHub API token consumption. Consider checking membership first (often cached) to avoid label fetch for org members.

### [should-fix] No opt-out mechanism
- concern: Every Prow installation using the assign plugin gets this behavior upon upgrade. No per-org or per-repo configuration flag to disable it.

### [should-fix] ~45 lines of logic inlined in `handle()`
- where: `assign.go:153-196`
- concern: Breaks the generic handler's symmetry by grafting assign-specific behavior via `h.userType == "assignee(s)"` (a display string used as type discriminator). Should be extracted into a dedicated function.

### [should-fix] `userType` string comparison for control flow
- where: `assign.go:153`
- concern: Using a display-oriented string as a type discriminator is fragile. If someone changes the string for cosmetic reasons, the logic silently breaks. Use a dedicated boolean or enum field on the `handler` struct.

### [nit] `isMemberCalled` assertions keyed off `tc.name` strings
- where: `assign_test.go:513-522`
- concern: Test case name string matching outside the loop. If a name changes, the assertion silently stops running. Add `expectIsMemberCalled *bool` to the test case struct.

### [nit] Test setup uses `tc.name` for conditional state
- where: `assign_test.go:450`
- concern: Labels are set conditionally by matching `tc.name`. Add a `labels` field to the test case struct so each case is self-contained.

### [nit] Missing test coverage
- concern: No test for: (1) `CONTRIBUTING.md` not found + `HelpGuidelinesURL` fallback, (2) `/assign @other-user` by a non-member, (3) `GetIssueLabels` returning an error, (4) non-nil `config` parameter.

### [nit] Hardcoded `"good-first-issue"` label
- concern: No constant, no comment explaining why this specific name (vs. `"good first issue"` with spaces, GitHub's default label). Define as a package-level constant.

### [nit] Message tone
- concern: "It looks like you're new!" may feel patronizing to experienced contributors from other orgs. Consider: "This issue hasn't been labeled as a good first issue."

### [nit] Variable naming: `hasGoodFirstIssue` vs `isGoodFirstIssue`
- where: `assign.go:154`
- concern: Amulyam24 suggests `isGoodFirstIssue` for readability. Minor but reasonable — `is` prefix better matches the boolean's role as a predicate about the issue.
- source: reviewer comment (Amulyam24, 2026-05-13)

### [should-fix] Entire `PluginConfig` passed unnecessarily
- where: `assign.go`
- concern: Amulyam24 asks why the entire `PluginConfig` instance is passed. Only `HelpGuidelinesURL` is needed. Narrowing the parameter reduces coupling.
- source: reviewer comment (Amulyam24, 2026-05-13)

## Checked
- Assignment still proceeds (no silent blocking of legitimate assigns)
- Existing `/assign` and `/unassign` behavior unchanged for org members
- Test structure follows existing patterns in `assign_test.go`
- No new dependencies introduced
- Rollback is safe — reverting removes educational messages with no state to clean up

## Open questions
- Is the intent to restrict (block) self-assignment or just warn? The PR title says "manage" / "restrict" but the code only warns.
- Should this be opt-in rather than on-by-default for all Prow installations?
- Why `good-first-issue` (with hyphens) rather than GitHub's default `good first issue` (with spaces)?

## Since previous review (2026-04-30)
- No new commits pushed. HEAD unchanged at 773c479ca.
- Amulyam24 submitted a COMMENTED review (2026-05-13) with three inline comments:
  - Suggests `isGoodFirstIssue` variable rename for readability.
  - Questions why entire `PluginConfig` is passed — only `HelpGuidelinesURL` is used.
  - Suggests consolidating the message `fmt.Sprintf` into a single call.
  - Asks about user-facing error messages when `GetIssueLabels`/`IsMember` fail (aligns with our "[blocking] GetIssueLabels error swallowed" finding).
- CLA bot confirmed committer is authorized (2026-05-13).

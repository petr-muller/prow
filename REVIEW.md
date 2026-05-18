---
pr: kubernetes-sigs/prow#723
title: "fix(clonerefs): fall back to full clone when sparse checkout fails"
head_sha: 644f75be0c9cd7f6b685f16523e2f1a3cc32f093
base: main
reviewed_at: 2026-05-18T18:13:21Z
verdict: approve
---

## Findings

### [should-fix] record.Refs reflects fallback refs, not original request
- where: `pkg/pod-utils/clone/clone.go:75-88`
- concern: After fallback, `record = runClone(fullRefs, ...)` overwrites `record.Refs` with `fullRefs` which has `SparseCheckoutFiles: nil`. Downstream consumers inspecting `Record.Refs.SparseCheckoutFiles` will not know sparse checkout was originally requested. Add `record.Refs = refs` after the fallback `runClone` call to preserve the original request for debugging.
- excerpt: |
    record = runClone(fullRefs, dir, gitUserName, gitUserEmail, cookiePath, env, user, token)
    record.Commands = append(failedCommands, record.Commands...)

### [should-fix] os.RemoveAll failure proceeds into doomed retry
- where: `pkg/pod-utils/clone/clone.go:81-83`
- concern: If `os.RemoveAll(cloneDir)` fails (e.g. permission denied), the code logs a warning but proceeds to `runClone` into a dirty directory. The second attempt will almost certainly fail with a confusing unrelated error. Consider returning the original failed record instead of retrying when cleanup fails.
- excerpt: |
    if err := os.RemoveAll(cloneDir); err != nil {
        logrus.WithError(err).Warn("Failed to clean up clone directory for fallback")
    }

### [nit] Missing comment explaining why fallback exists
- where: `pkg/pod-utils/clone/clone.go:77`
- concern: The code is clear about what it does but not why. A one-line comment noting the submodule/gitlink edge case would save future maintainers from needing to find this PR.

### [nit] record.Refs could use explicit assertion in fallback test
- where: `pkg/pod-utils/clone/clone_test.go:999`
- concern: The fallback test checks `record.Failed` indirectly via an `if` branch but does not have a direct `if record.Failed` style assertion that would make the success contract scannable. Minor readability improvement.

## Checked
- `runClone` extraction is clean: `startTime` renamed to `cmdStart` to avoid shadowing, duration accounting moved to `Run()` caller
- `fullRefs := refs` is a safe struct copy; nil-ing `SparseCheckoutFiles` correctly makes `isSparseCheckoutSet` return false
- `append(failedCommands, record.Commands...)` works correctly since `failedCommands` is a separate slice reference
- Fallback only triggers when `isSparseCheckoutSet(refs)` is true AND `record.Failed` is true — no impact on non-sparse clones
- Tests use real git repos, assert on behavioral outcomes, not brittle implementation details
- No configuration, API, CLI flag, or ProwJob spec changes — purely internal resilience improvement
- No breaking changes, safe to roll back, zero upgrade friction
- Deployment risk is LOW: only observable change is that some previously-failing sparse checkout jobs will now succeed via full clone

## Open questions
- Should `record.Refs` after a successful fallback reflect the original request (with `SparseCheckoutFiles`) or what was actually executed (without)? The PR description emphasizes debugging visibility, which argues for the original request.
- Is warn-and-continue the right behavior when `os.RemoveAll` fails, or should the fallback be skipped entirely in that case?

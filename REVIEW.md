---
pr: kubernetes-sigs/prow#716
title: "generic-autobumper: add GitHub App authentication support"
head_sha: 1a9722a01615a1a2f0c219c4fb7815f782d22ac8
base: main
reviewed_at: 2026-05-15T15:04:14Z
verdict: needs-discussion
---

## Summary

Adds `--pr-source-mode=branch` to generic-autobumper. When combined with `--github-app-id` and `--github-app-private-key-path`, pushes branches directly to the upstream repo (no fork) and creates same-repo PRs using `OrgAwareClient`. Legacy PAT/fork flow is default and untouched.

New files: `orgaware.go` (OrgAwareClient wrapper). Modified: `bumper.go` (new `processGitHubAppAuth`, validation changes, auto-detection), `main.go` (flag wiring), `bumper_test.go` (tests for new logic).

## Findings

### [should-fix] SkipPullRequest skips push in App auth path but not in fork path
- where: `cmd/generic-autobumper/bumper/bumper.go:386-388`
- concern: When `SkipPullRequest=true`, the App auth path returns before pushing (`PushToCentral` at line 391 is never reached). In the fork path, `MinimalGitPush` is always called — it just treats `SkipPullRequest` as `dryrun` (logs the push but skips it). The App auth path also skips creating the `GitClientFactory` cleanup. This means the two modes have different semantics for `--skip-pullrequest`: fork mode logs what it would push, App auth mode does nothing. If the intent is to push but not create a PR, this is a bug. If the intent is to do nothing, the early return should be higher (before the commit loop).
- excerpt: |
    if o.SkipPullRequest {
        return nil
    }

    logrus.WithField("branch", o.HeadBranchName).Info("Pushing branch directly to upstream repo")
    if err := repoClient.PushToCentral(o.HeadBranchName, true); err != nil {

### [should-fix] BotUser partial copy drops future UserData fields
- where: `cmd/generic-autobumper/bumper/orgaware.go:56-59`
- concern: Constructs a new `UserData` copying only `Name`, `Login`, `Email`. If `UserData` gains fields (e.g. `ID`, `HTMLURL`), they are silently zeroed. A struct copy (`patched := *user; patched.Login += "[bot]"; return &patched, nil`) preserves all fields.
- excerpt: |
    return &github.UserData{
        Name:  user.Name,
        Login: user.Login + "[bot]",
        Email: user.Email,
    }, nil

### [should-fix] Duplicate [bot] suffix logic in two places
- where: `cmd/generic-autobumper/bumper/bumper.go:332-335` and `cmd/generic-autobumper/bumper/orgaware.go:48-60`
- concern: `processGitHubAppAuth` appends `[bot]` to the login for git commit identity (lines 332-335), and `OrgAwareClient.BotUser()` does the same for the GitHub search author qualifier. Both have the `HasSuffix` guard. The `processGitHubAppAuth` function calls `gc.BotUser()` directly (not through `OrgAwareClient`), so it gets the raw login and patches it manually. If the `OrgAwareClient` were used for the initial BotUser call too, this duplication wouldn't be needed.

### [nit] GitHubOptions always set unconditionally
- where: `cmd/generic-autobumper/main.go:216`
- concern: `pro.GitHubOptions = &githubOpts` is always set, even when no App flags were provided. Auto-detection in `resolvedPRSourceMode()` checks `o.GitHubOptions.AppID != ""`, which currently works because `AppID` defaults to `""`. If `flagutil.GitHubOptions` ever reads `AppID` from an env var or config file, auto-detection would silently flip to `"branch"`. Consider guarding: `if githubOpts.AppID != "" { pro.GitHubOptions = &githubOpts }`.
- excerpt: |
    pro.GitHubOptions = &githubOpts
    if prSourceMode != "" {
        pro.PRSourceMode = prSourceMode
    }

### [nit] Commit loop duplicated between processGitHub and processGitHubAppAuth
- where: `cmd/generic-autobumper/bumper/bumper.go:270-295` and `cmd/generic-autobumper/bumper/bumper.go:344-369`
- concern: Lines 344-369 are a near-exact copy of lines 270-295 (iterate changes, check for changes, commit). A bugfix to one must be mirrored in the other. Could be extracted into a helper that returns `(anyChange bool, err error)`. Low priority if the team prefers explicit duplication.

### [nit] Force push is always enabled
- where: `cmd/generic-autobumper/bumper/bumper.go:391`
- concern: `PushToCentral(o.HeadBranchName, true)` — the `true` means force push. For a bot-managed autobump branch this is probably correct, but if two autobumper instances target the same branch, one silently overwrites the other. Worth a log line or brief comment noting force push is intentional.
- excerpt: |
    if err := repoClient.PushToCentral(o.HeadBranchName, true); err != nil {

### [question] allowMods=true intentional?
- where: `cmd/generic-autobumper/bumper/bumper.go:407`
- concern: The fork path calls `updatePRWithLabels` with `updater.PreventMods` (which is `false`), meaning PRs are created with `maintainer_can_modify=false`. The App auth path passes `allowMods=true`. This makes sense for same-repo PRs (the branch is already in the target repo), but the inconsistency should be intentional and documented.
- excerpt: |
    return UpdatePullRequestWithLabels(orgAwareGC, o.GitHubOrg, o.GitHubRepo,
        summary, generatePRBody(body, getAssignment(o.AssignTo)),
        o.HeadBranchName, o.GitHubBaseBranch, o.HeadBranchName,
        true, o.Labels, false)

### [question] dryrun=false hardcoded in App auth path
- where: `cmd/generic-autobumper/bumper/bumper.go:408`
- concern: The fork path passes `o.SkipPullRequest` as the `dryrun` parameter to `UpdatePullRequestWithLabels`. The App auth path hardcodes `false`. Since the App auth path already returns early when `SkipPullRequest=true` (line 386), this is technically fine — but it means the two paths handle dryrun/skip differently. The fork path treats `SkipPullRequest` as "do everything in dryrun mode"; the App auth path treats it as "do nothing at all".

## Checked

- `OrgAwareClient.FindIssues` correctly delegates to `FindIssuesWithOrg` with the stored org — solves the App auth round-tripper issue.
- `resolvedPRSourceMode()` auto-detection logic is correct for current defaults.
- Validation properly rejects `branch` mode without App credentials and `fork` mode without token.
- `HideSecretsWriter` used in both paths for stdout/stderr.
- No credential leaks — delegating to `flagutil.GitHubOptions` for all secret handling.
- Test coverage for `resolvedPRSourceMode`, validation, `OrgAwareClient.FindIssues`, and `OrgAwareClient.BotUser`.
- Flag registration via `goflag.CommandLine` + `AddGoFlagSet` is an established pattern in Prow.
- `RemoteName` validation correctly moved to fork-only (not needed for branch mode).

## Open questions

- Is the `SkipPullRequest` semantic difference intentional? Fork mode: push (dryrun) + PR (dryrun). App auth mode: skip everything after commits.
- Should `processGitHubAppAuth` use `OrgAwareClient` for the initial `BotUser()` call to avoid duplicating the `[bot]` suffix logic?
- Has this been tested end-to-end with a real GitHub App installation?

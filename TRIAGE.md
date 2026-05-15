---
issue: kubernetes-sigs/prow#194
title: "Allow `ok-to-test` label to approve GitHub workflow runs for new contributors #25210"
state: closed
labels: kind/feature, help wanted, sig/contributor-experience, area/plugins
main_sha: 3e578e4f0ad16bb4435dcbf4c52434d9ec34667b
triaged_at: 2026-05-15T10:12:49Z
verdict: accepted
---

## Findings

### [cause] Workflow approval is only wired to the /ok-to-test comment handler
- detail: `approveGitHubActionsWorkflowRuns()` is called exclusively from `handleGenericComment()` when the comment matches `/ok-to-test`. The PR event handlers for synchronize, label-added, reopen, and ready-for-review never call it.
- evidence: `pkg/plugins/trigger/generic-comment.go:146-154`

### [reproducibility] Confirmed by maintainer in production
- detail: @sbueringer reported (2026-05-13) that GitHub Actions workflow runs require manual approval after new commits are pushed to fork PRs that already have the `ok-to-test` label. @stevehipwell confirmed (2026-05-15) that behavior should be consistent with `/ok-to-test` semantics.
- evidence: https://github.com/kubernetes-sigs/prow/issues/194#issuecomment-4443149805

### [related-code] approveGitHubActionsWorkflowRuns helper
- where: `pkg/plugins/trigger/generic-comment.go:301-337`
- excerpt: |
    func approveGitHubActionsWorkflowRuns(c Client, org, repo, branchName, headSHA string) {
        pendingRuns, err := c.GitHubClient.GetPendingApprovalActionRuns(org, repo, branchName, headSHA)
        ...
        for _, run := range pendingRuns {
            ...
            go func() {
                if err := c.GitHubClient.ApproveGitHubWorkflowRun(org, repo, runID); err != nil {

### [related-code] PullRequestActionSynchronize handler — missing approval
- where: `pkg/plugins/trigger/pull-request.go:122-127`
- excerpt: |
    case github.PullRequestActionSynchronize:
        var errs []error
        if err := abortAllJobs(c, &pr.PullRequest); err != nil {
            errs = append(errs, fmt.Errorf("failed to abort jobs: %w", err))
        }
        return utilerrors.NewAggregate(append(errs, buildAllIfTrusted(c, trigger, pr, baseSHA, presubmits)))

### [related-code] PullRequestActionLabeled handler for ok-to-test — missing approval
- where: `pkg/plugins/trigger/pull-request.go:139-151`
- excerpt: |
    if pr.Label.Name == labels.OkToTest {
        botUserChecker, err := c.GitHubClient.BotUserChecker()
        ...
        return buildAllButDrafts(c, &pr.PullRequest, pr.GUID, baseSHA, presubmits)
    }

### [related-code] buildAllIfTrusted — natural injection point
- where: `pkg/plugins/trigger/pull-request.go:227-249`
- excerpt: |
    func buildAllIfTrusted(c Client, trigger plugins.Trigger, pr github.PullRequestEvent, baseSHA string, presubmits []config.Presubmit) error {
        ...
        l, trusted, err := TrustedPullRequest(c.GitHubClient, trigger, author, org, repo, num, nil)
        ...
        } else if trusted {
            ...
            return buildAllButDrafts(c, &pr.PullRequest, pr.GUID, baseSHA, presubmits)

### [related-code] TrustedPullRequest trust determination
- where: `pkg/plugins/trigger/trigger.go:365-381`
- excerpt: |
    func TrustedPullRequest(...) ([]github.Label, bool, error) {
        if trustedResponse, err := TrustedUser(...); ... {
        } else if trustedResponse.IsTrusted { return l, true, nil }
        ...
        return l, github.HasLabel(labels.OkToTest, l), nil
    }

### [related-code] GitHub client workflow approval API
- where: `pkg/github/client.go:2208-2251`
- excerpt: |
    func (c *client) GetPendingApprovalActionRuns(org, repo, branchName, headSHA string) ([]WorkflowRun, error)
    func (c *client) ApproveGitHubWorkflowRun(org, repo string, id int) error

### [related-code] TriggerGitHubWorkflows config flag
- where: `pkg/plugins/config.go:545-546`
- excerpt: |
    TriggerGitHubWorkflows bool `json:"trigger_github_workflows,omitempty"`

### [related-pr] PR #612 — initial implementation
- ref: kubernetes-sigs/prow#612
- relevance: Added workflow approval on `/ok-to-test` comment. Merged 2026-02-06. Only covered the comment path, not synchronize/label events.

### [related-issue] Original issue in test-infra
- ref: kubernetes/test-infra#25210
- relevance: Original feature request before migration to kubernetes-sigs/prow.

## Checked

- Push event handler (`pkg/plugins/trigger/push.go`) — not relevant; handles postsubmit jobs on branch pushes, not PR synchronize events
- `TriggerGitHubWorkflows` config flag — correctly gates the feature; fix must respect it
- `approveGitHubActionsWorkflowRuns()` error handling — handles 404 (already approved) and 403 (permission denied) gracefully; safe to call from multiple paths
- Slack thread referenced by @petr-muller (2026-05-13) — confirms runs need individual approval and #612 only did one-off approval

## Next steps

- Reopen issue #194 with a comment clarifying the remaining gap: approval must fire on synchronize and label events, not only on `/ok-to-test` comment
- Wire `approveGitHubActionsWorkflowRuns()` into `buildAllIfTrusted()` (covers synchronize, reopen, edit, ready-for-review) and the `ok-to-test` label handler in `pull-request.go`
- Decide whether to assign @AaruniAggarwal (PR #612 author) or open for other contributors
- Investigate timing: `GetPendingApprovalActionRuns()` may return empty if called before GitHub creates the workflow runs after a push

## Open questions

- Should there be a retry/delay mechanism for workflow approval after push events, given that GitHub may not create workflow runs immediately?
- Does `GetPendingApprovalActionRuns()` need to handle `pull_request_target` events differently from `pull_request` events for the push case?
- Should the issue be reopened or should a new, more specific follow-up issue be filed?

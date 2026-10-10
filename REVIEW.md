---
pr: kubernetes-sigs/prow#983
title: "github: retry GraphQL queries above the transport"
head_sha: 1cf0a9f75fe63839e61f7f5cb1ce7f1258f5269b
base: main
reviewed_at: 2026-10-05T22:44:41Z
verdict: request-changes
gate:
  decision: merge
  gated_at: 2026-10-06T17:41:08Z
  gated_head_sha: 1cf0a9f75fe63839e61f7f5cb1ce7f1258f5269b
  reviewed_head_sha: 1cf0a9f75fe63839e61f7f5cb1ce7f1258f5269b
---

# Review

## Gate

**Decision: merge.**

Impact reassessment after tracing Tide's callers: the saved error-preservation finding describes real diagnostic information loss, but its blocking severity was overstated. The PR explicitly promises preservation when a deadline prevents backoff or cancellation interrupts backoff; both paths are implemented. Preserving a preceding 5xx when a later attempt is interrupted would be an additional diagnostic improvement. The original review record is retained below; this gate supersedes its merge recommendation.

### Findings disposition

- **Borderline — acceptably dispositioned as nonblocking:** the original `REVIEW.md` finding at `pkg/github/client.go:3973-3974` remains unchanged in code. A later throttle/request interruption returns its own error without the preceding 5xx. No existing caller was found that needs that earlier status to recover: Tide disables automatic 502/504 retries while pages can shrink, uses normal retries only at the minimum page size, and refuses to shrink when its context has expired (`pkg/tide/github.go:218-223`). Preserve the previous error as an optional diagnostic improvement.
- **Borderline — nonblocking scope feedback:** @mimowo's 2026-10-02 comment suggests separating cleanup from the reduction in retries. No subsequent commits separate them; the reduction is explained in the PR description. This suggestion does not independently block merge.
- The saved backoff-test and test-helper nits do not concern substantive merge risk.

### Gating list

None. The earlier do-not-merge decision rested on an unsupported severity classification, rather than a demonstrated functional regression.

### Independent merge risk

- Exported APIs are additive; existing signatures, flags, and config schemas are unchanged. No coordinated rollout or migration is required.
- All `pkg/github` GraphQL query consumers receive fewer default attempts, configured retry caps/backoff, throttle accounting per attempt, and separate request timeouts. Callers without a tighter context deadline can wait up to three request timeouts plus backoff and throttle waits. These default behavior changes are documented in the PR description and warrant operator-facing release notes.
- GraphQL mutations, including issue-management pin/unpin/transfer operations, are sent once. This avoids replaying an uncertain write and surfaces transient failures sooner.
- Error prefixes change; downstream callers comparing complete error strings may need to adapt. Tide uses the typed helper.
- Compatibility was assessed directly from the production diff, client construction, flags, and callers; no specialized compatibility skill applied.

## Verdict

Request changes: preserve the last 5xx if a later retry is interrupted. The current loop retains that status when cancellation happens during backoff, but loses it when the next attempt ends in throttling or request cancellation. The maintainability review found the change localized and deployment risk manageable, with no configuration migration required.

## What this PR does

- Moves GraphQL retries from the HTTP transport to the query method so each attempt passes through throttling and gets its own request timeout.
- Retries transient 502/503/504 query failures with bounded exponential backoff and honors the configured total-attempt cap.
- Keeps mutations single-shot and exposes typed GraphQL server errors for Tide's page-shrinking recovery.
- Adds coverage for retry limits, cancellation, Apps auth headers, and Tide's behavior when pages keep timing out.

## Findings

### [blocking] Preserve the last 5xx when a retry attempt is interrupted
- where: `pkg/github/client.go:3971-3974`
- concern: After a query returns a typed 5xx, a later attempt can end with a context error while waiting for a throttle token or during the HTTP request. This branch then returns only that later error, so callers lose the earlier server status; retain and wrap the last 5xx, and test both `errors.Is` for the context error and `errors.As` for the server error.
- excerpt: |
    err := c.gqlc.QueryWithGitHubAppsSupport(ctx, q, vars, org)
    var serverErr graphQLServerError
    if !errors.As(err, &serverErr) {
        return err
    }

### [nit] Assert the exponential backoff sequence
- where: `pkg/github/client_test.go:4978-4988`
- concern: Retry tests assert request counts and error results but do not assert that the delay doubles from `InitialDelay`. A deterministic delay assertion would protect this documented retry behavior from regressions.
- excerpt: |
    func TestGraphQLRetries(t *testing.T) {
        testCases := []struct {
            name                  string
            statuses              []int
            mutate                bool
            callerHandlesTimeouts bool
            maxRetries            int
            expectedRequests      int

### [nit] Keep the test-only server error helper's API surface in view
- where: `pkg/github/client.go:855-858`
- concern: This exported constructor exists to let external-package tests create the internal typed error. That follows the existing `NewUnprocessableEntity` pattern; if more test-only constructors are added, consider a narrower test seam.
- excerpt: |
    // NewGraphQLServerError returns the error the GraphQL client returns for a
    // transient server error with the given status code, for tests.
    func NewGraphQLServerError(statusCode int) error {

## Checked

- Query attempts pass through throttling and use the configured per-request timeout; retry counts are capped and mutations are not retried.
- Context interruption during backoff preserves the last 5xx; this review found that interruption during a later attempt does not.
- Tide recognizes typed 502/504 errors and keeps its same-cursor page-shrinking behavior.
- Apps-auth GraphQL attempts retain the token and add each preview `Accept` value once.
- No config schema, CLI flag, or deployment manifest changes; custom `--github-client.max-retries` values now also cap GraphQL attempts.

## Open questions

None.

## Followups

Recorded 2026-10-10T14:07:39Z. 5 accepted; 1 skipped.

Post-merge followups for PR #983, merged as `192ff979d39c13c029661317682ecb6635c5f1c9`. Candidates were checked against upstream main `e012f2883b7981ab636b0abf3009f0f7064658b1`. Each handoff starts from current upstream main and should first check whether the work has already landed.

The diagnostic concern was previously downgraded to optional after caller analysis; its acceptance here does not restore the original blocking classification. The exported test-error helper follows an existing package pattern and did not warrant a separate concrete followup. Page-shrink metrics were skipped.

### Retain the earlier 5xx for diagnosis

- category: diagnostics
- necessity: could

```text
In kubernetes-sigs/prow, following PR #983 — "github: retry GraphQL queries above the transport" (merge commit 192ff979d39c13c029661317682ecb6635c5f1c9), start from current upstream main, not the review branch. Improve diagnostics in client.QueryWithGitHubAppsSupport in pkg/github/client.go. Retain the most recent typed GraphQL 5xx when a later attempt is interrupted during throttling or the HTTP request, returning an error that preserves both the interruption and earlier server error. This is optional diagnostic work; the review's original blocking classification was withdrawn. Add focused tests for a 503 followed by cancellation/deadline expiry, including the throttle-wait scenario: errors.Is must find the context error and errors.As must find the previous server error. Successful retries must return nil, and first-attempt interruptions must have no fictitious server error. Preserve Tide's context-first error classification. Keep retry counts, mutation semantics, page shrinking, and public signatures unchanged; avoid changing unrelated error paths.
```

### Test the retry timing contract

- category: tests
- necessity: could

```text
In kubernetes-sigs/prow, following PR #983 — "github: retry GraphQL queries above the transport" (merge commit 192ff979d39c13c029661317682ecb6635c5f1c9), start from current upstream main, not the review branch. Add focused timing coverage in pkg/github/client_test.go and, only if needed, a narrow unexported seam in pkg/github/client.go. Deterministically verify query delays are InitialDelay then 2*InitialDelay; single permitted attempts and mutations must not wait for retry backoff. Verify through NewClientFromOptions and its actual HTTP client that each attempt receives a fresh MaxRequestTime: two attempts individually below the timeout can succeed even if their combined duration exceeds it. Use request synchronization and generous timing margins rather than millisecond elapsed-time assertions. Preserve cancellable production waits and existing retry behavior. Do not redesign the package-wide timeClient abstraction, add public testing APIs, or modify REST retry policy.
```

### Honor organization-specific GraphQL throttling

- category: correctness
- necessity: should

```text
In kubernetes-sigs/prow, following PR #983 — "github: retry GraphQL queries above the transport" (merge commit 192ff979d39c13c029661317682ecb6635c5f1c9), start from current upstream main, not the review branch. Fix organization selection in ghThrottler.QueryWithGitHubAppsSupport and ghThrottler.MutateWithGitHubAppsSupport in pkg/github/client.go. These methods read githubOrgContextKey before the underlying wrapper sets it, so normal calls with a supplied org use the global bucket or bypass an org-only throttler. Select the bucket using the explicit org argument. Add tests showing that exhausted org-A tokens block org-A queries/mutations without consuming org-B's independent budget, each query retry consumes the correct org's token, and unconfigured/empty organizations retain global fallback. Preserve cancellation, Apps installation-token selection, public signatures, and REST behavior. Do not change throttle rates, flags, or refill policy.
```

### Reuse a recently successful search page size

- category: performance
- necessity: could

```text
In kubernetes-sigs/prow, following PR #983 — "github: retry GraphQL queries above the transport" (merge commit 192ff979d39c13c029661317682ecb6635c5f1c9), start from current upstream main, not the review branch. Reduce repeated expensive gateway/resource-limit failures in GitHubProvider.search in pkg/tide/github.go. Add a concurrency-safe, bounded, expiring cache of the last successful page size keyed by organization and stable configured query, excluding changing date filters and cursors. Subsequent searches should start at that recently successful size; expiry must allow retrying the default so transient problems do not permanently reduce it. Document expiry/capacity policy. Tests must cover reuse after 37-to-18 recovery, isolation between queries/orgs, expiry restoring default, and safe concurrent access without retaining cursors/results. Preserve minimum/maximum sizes, filtering, deadlines, and sync publication semantics. Do not persist the cache or introduce broad scheduling changes.
```

### Document GraphQL retry and timeout semantics

- category: docs
- necessity: should

```text
In kubernetes-sigs/prow, following PR #983 — "github: retry GraphQL queries above the transport" (merge commit 192ff979d39c13c029661317682ecb6635c5f1c9), start from current upstream main, not the review branch. Document current retry behavior in site/content/en/docs/github-api-access.md and correct relevant help text in pkg/flagutil/github.go. Derive defaults and limits from current code. Explain that github-client.max-retries counts total attempts, GraphQL queries are additionally capped at three attempts, and 1 disables query retries. Explain that request-timeout applies per HTTP attempt while backoff/throttle waits extend total operation time unless an outer context limits it. Cover InitialDelay doubling, per-attempt throttling, the 502/503/504 status set, and single-shot GraphQL mutations. Distinguish library defaults from component flag defaults if listing values, and do not imply REST writes share the single-shot policy. Verify against client/flag code. Change documentation/help wording only, not flag names, defaults, schemas, or runtime behavior.
```


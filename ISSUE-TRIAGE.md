# Triage for Issue #337

**Status**: In Progress
**Created**: 2026-04-13

## Issue Information

- **Issue Number**: #337
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/337

## Initial Validation

**Assessment**: LEGITIMATE

### Analysis

The issue reports a race condition in Tide's merge logic. When a GitHub Action is re-triggered, GitHub temporarily removes the old check status before the new run starts. During this brief window, Tide sees no pending/failing context for the check and may proceed to merge the PR prematurely.

**Issue Category**: Bug

**Reporter**: @saschagrunert (project MEMBER), credible reporter with direct experience of the problem

**Repository Scope Check**:
- Component mentioned: Tide (status controller)
- Exists in this repo: Yes
- Relevant code paths: `pkg/tide/status.go` (lines ~478-492 referenced by reporter)

**Information Completeness**:
- Sufficient detail provided: Yes
- Example PR provided: https://github.com/kubernetes-sigs/security-profiles-operator/pull/2595
- Code location identified by reporter
- Screenshot showing the race condition outcome

**Current State**:
- Issue is CLOSED by the lifecycle stale/rotten bot (not by a human)
- Has been reopened twice by @petr-muller to keep it alive
- Labels: `kind/bug`, `area/tide`, `lifecycle/stale`
- PR #563 attempted a fix but was closed without merging (approach: tracking previously seen contexts per PR/commit, treating disappeared contexts as PENDING)

### Recommendation

This is a clearly legitimate bug report for a race condition in Tide. The reporter is a project member who provided concrete evidence including an example PR and a code reference. The bug can cause PRs to be merged with failing or not-yet-started checks, which is a correctness issue in Tide's core merge safety logic.

**Suggested Action**:
- Reopen the issue (closed by stale bot, not resolved)
- Continue triage with research phase

## Findings

(Additional findings from triage subcommands will be added here)

## Next Steps

- Proceed to research phase to understand the race condition in detail
- Investigate why PR #563 was closed and whether its approach was sound

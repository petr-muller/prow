# Triage for Issue #337

**Status**: In Progress
**Created**: 2025-12-23

## Issue Information

- **Issue Number**: #337
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/337
- **Title**: Tide merges PR when retesting GitHub action
- **Author**: saschagrunert
- **Created**: 2024-12-03
- **State**: OPEN
- **Labels**: kind/bug, area/tide

## Summary

Tide occasionally merges PRs when re-triggering GitHub Actions, even when required checks haven't completed yet. The issue appears to be a race condition in Tide's status checking logic.

## Findings

### Issue Description
- Tide merges PRs while GitHub Actions are being re-triggered
- Required checks (e.g., `e2e-fedora`) show as not started but also not failed
- Example PR: https://github.com/kubernetes-sigs/security-profiles-operator/pull/2595
- Suspected race condition in code at: pkg/tide/status.go:478-492

### Timeline
- **2024-12-03**: Issue opened by saschagrunert
- **2025-04-03**: Reopened by petr-muller after stale bot marked it rotten
- **2025-08-05**: Reopened again by petr-muller
- **2025-12-04**: saschagrunert mentions PR #563 is approaching a fix
- **2025-12-23**: Currently being triaged

### Related Work
- PR #563 is working on a fix (mentioned 2025-12-04)

## Next Steps

1. Review PR #563 to understand the proposed fix
2. Examine the code at pkg/tide/status.go:478-492 to understand the race condition
3. Verify if the fix in PR #563 adequately addresses the race condition
4. Consider if additional test coverage is needed to prevent regression

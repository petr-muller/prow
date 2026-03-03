# Triage for Issue 400

**Status**: In Progress
**Created**: 2026-03-03

## Issue Information

- **Issue Number**: 400
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/400
- **Title**: `tide` merge queue stalls when unresolved comments exist
- **Author**: aevyrie
- **Labels**: area/tide, kind/bug, lifecycle/stale

## Issue Summary

When a PR is in the merge queue, has unresolved comments in GitHub, and the repo branch protection settings require all comments to be resolved before merge, it stalls the `tide` merge queue because the PR cannot merge. To most users, the stalled PR looks inexplicable. Expected behavior: PRs that cannot merge due to unmet requirements should be ignored.

## Initial Validation

**Assessment**: LEGITIMATE

### Analysis

The issue reports that when a PR enters Tide's merge queue but has unresolved GitHub review comments (with branch protection requiring comment resolution), Tide repeatedly attempts to merge the PR and fails. This stalls the entire merge queue, blocking other PRs from merging. The behavior is invisible to most users, making it appear as though the merge queue is inexplicably stuck.

**Issue Category**: Bug

**Repository Scope Check**:
- Component mentioned: Tide (`pkg/tide/`)
- Exists in this repo: Yes
- Relevant code paths: `pkg/tide/tide.go`, `pkg/tide/github.go`, `pkg/tide/status.go`

**Information Completeness**:
- Sufficient detail provided: Yes
- Clear observed vs expected behavior described
- Missing information: No specific PR link demonstrating the issue, but the scenario is clearly described and reproducible

**Related Issues**:
- Issue #269: "PR with 'Change requested' leads to Tide repeatedly attempting MERGE" — same root cause pattern. Tide doesn't check GitHub branch protection requirements before attempting merge, leading to repeated failed attempts and queue stalls. A maintainer (petr-muller) confirmed the likely relation.

### Recommendation

This is a legitimate bug in Tide's merge logic. Tide should pre-check GitHub branch protection requirements (unresolved comments, required reviews) before attempting to merge a PR. When a PR can't be merged due to branch protection settings, Tide should skip it rather than stalling the queue.

The issue is part of a broader pattern (shared with #269) where Tide doesn't account for all GitHub branch protection rules, leading to repeated merge failures.

**Suggested Action**:
- Keep open and continue triage
- Consider as related to (possibly duplicate root cause with) issue #269

## Findings

(Further findings from triage subcommands will be added here)

## Next Steps

(Action items will be added here)

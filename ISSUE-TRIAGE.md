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

## Findings

(Findings from triage subcommands will be added here)

## Next Steps

(Action items will be added here)

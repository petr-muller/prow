# Triage for Issue #651

**Status**: In Progress
**Created**: 2026-03-13

## Issue Information

- **Issue Number**: #651
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/651
- **Title**: `tide`: batch triggered containing an already-merged PR
- **Author**: Prucek
- **Labels**: area/tide, kind/bug

## Issue Summary

Tide triggered a TRIGGER_BATCH that included a PR it had just merged in the previous sync cycle. Observed on Azure/ARO-HCP repo via tide-history:
- 9:50 — PR merged manually #4297
- 9:53 — Tide fires a TRIGGER_BATCH that includes PR #4297

Expected behavior: a merged PR should not be included in a batch.

## Findings

(Findings from triage subcommands will be added here)

## Next Steps

(Action items will be added here)

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

## Initial Validation

**Assessment**: LEGITIMATE

### Analysis

The issue describes a concrete bug in Tide's batch merging logic where a PR that was already merged (either manually or by Tide in a previous cycle) is incorrectly included in a subsequent TRIGGER_BATCH action.

**Issue Category**: Bug

**Repository Scope Check**:
- Component mentioned: Tide
- Exists in this repo: Yes (`pkg/tide/`)
- Relevant code paths: `pkg/tide/tide.go`, `pkg/tide/github.go` (PR pool filtering and batch logic)

**Information Completeness**:
- Sufficient detail provided: Yes
- Clear timeline with specific PR number and tide-history link
- Screenshot of tide-history showing the sequence of events
- The report includes a specific, reproducible scenario

### Discussion Notes

- An AI-generated comment on the issue suggests the root cause is GraphQL API staleness (3-minute gap not enough for GitHub to reflect the merge)
- Maintainer petr-muller pushed back on this theory, noting that if this were the case, we would routinely see TRIGGER for just-merged PRs, which we don't
- Root cause likely lies in how Tide filters its PR pool or how it handles race conditions between merge completion and the next sync cycle

### Recommendation

Keep open and continue triage. This is a valid bug report for the Tide component with clear reproduction evidence. The root cause needs investigation - the GraphQL staleness theory has been questioned, so deeper code analysis is warranted.

**Suggested Action**:
- Keep open and continue triage

## Findings

(Findings from triage subcommands will be added here)

## Next Steps

(Action items will be added here)

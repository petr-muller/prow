# Triage for Issue #673

**Status**: In Progress
**Created**: 2026-04-03

## Issue Information

- **Issue Number**: #673
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/673

## Initial Validation

**Assessment**: LEGITIMATE

### Analysis

The issue reports that Tide gets stuck in a retry loop when a PR matches Tide's label query (has `approved` + `lgtm`) but cannot be merged because GitHub branch protection with `enforce_admins: true` requires a minimum number of approving reviews that the PR doesn't have. This causes Tide to repeatedly pick the same PR, fail, and never advance to other mergeable PRs in the same repo.

**Issue Category**: Bug

**Repository Scope Check**:
- Component mentioned: Tide (`pkg/tide`)
- Exists in this repo: Yes
- Relevant code paths:
  - `pkg/tide/tide.go` — `accumulate()` (line 1077), `pickHighestPriorityPR()`, `tryMerge()` (line 1365)
  - `pkg/tide/github.go` — `mergePRs()` handling of `UnmergablePRError`
  - `pkg/github/client.go` — `UnmergablePRError` definition

**Information Completeness**:
- Sufficient detail provided: Yes
- The issue includes:
  - Clear description of the failure mode
  - Reproduction steps
  - Root cause analysis with specific code references (verified to exist)
  - A proposed fix approach
  - Context linking to related issue #134 (open, `kind/bug`, `area/tide`)
  - A comment from another user confirming the same loop occurs with "changes requested" review verdicts

**Relationship to #134**: Issue #134 reports that Tide doesn't honor GitHub's `required_approving_review_count` branch protection. Issue #673 describes the flip-side: when `enforce_admins: true` forces Tide to respect that protection, the merge failure causes a queue-blocking retry loop. They share a root cause (Tide's lack of awareness of GitHub review requirements) but have distinct symptoms.

### Recommendation

This is a well-documented, legitimate bug in Tide's merge queue logic. The issue is actionable, has reproduction steps, and the reporter has done significant code analysis. A second user has confirmed a related failure mode.

**Suggested Action**:
- Keep open and continue triage

## Findings

(Additional findings from triage subcommands will be added here)

## Next Steps

(Action items will be added here)

# Triage for Issue #610

**Status**: In Progress
**Created**: 2026-05-03

## Issue Information

- **Issue Number**: #610
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/610

## Initial Validation

**Assessment**: LEGITIMATE

### Analysis

**Issue Summary**: The `owners-label` plugin adds `sig/` and `area/` labels based on all files changed in a PR. When a user mistakenly pushes a merge commit, the PR's diff suddenly includes all files from the merged branch, causing `owners-label` to apply a large number of irrelevant labels. Since `owners-label` only adds labels and never removes them, these erroneous labels persist even after the user force-pushes to remove the merge commit. This pollutes searches, filters, and issue/PR tracking.

**Issue Category**: Feature Request (with bug-like characteristics — the current behavior produces incorrect results in a common scenario)

**Repository Scope Check**:
- Component mentioned: `owners-label` plugin
- Exists in this repo: Yes (`pkg/plugins/owners-label/owners-label.go`)
- Related component: `mergecommitblocker` plugin (`pkg/plugins/mergecommitblocker/mergecommitblocker.go`)
- Both components are maintained in this repository

**Information Completeness**:
- Sufficient detail provided: Yes
- The issue clearly describes the problem, the root cause, and proposes a solution approach
- The discussion in comments refines the approach further

### Comment Discussion Summary

The issue generated a substantive technical discussion (6 comments) between the reporter (danwinship) and a maintainer (BenTheElder):

1. **BenTheElder** endorsed the idea (+1), suggested it might need to be opt-in since not all Prow deployments use `mergecommitblocker`, and clarified why `owners-label` only adds labels (users can add labels via `/sig foo` commands too).

2. **danwinship** initially considered coupling the behavior to whether `mergecommitblocker` is enabled, but then argued the fix should be unconditional: there is **no workflow** where applying labels based on files in a merge commit makes sense. If you merge master into your PR, you'd get labels for every other PR that merged into master since your branch point — that's never useful.

3. **danwinship proposed refined pseudocode**:
   - For each commit in the PR:
     - If it's a merge *from* the target branch, skip it
     - Otherwise, for each file modified by that commit, add labels
   
4. **BenTheElder** was initially thinking about it the other way (if you don't allow merges, no point labeling from them), but danwinship argued the logic holds universally.

5. **Key consensus**: Merge commits should always be skipped for labeling purposes, regardless of configuration. The reasoning is that labeling from merge commits can only produce noise — it reflects upstream changes, not the PR author's work.

### Recommendation

This is a well-articulated feature request with clear technical analysis and maintainer endorsement. The reporter has deep understanding of the problem and proposed a sound solution approach. The discussion converged on a clear design direction.

**Suggested Action**:
- Keep open and continue triage
- The issue is ready for research and implementation planning

## Findings

(Additional findings from triage subcommands will be added here)

## Next Steps

- Research: Investigate `owners-label` plugin code to understand current implementation
- Assess whether danwinship's proposed approach is architecturally sound
- Determine effort level

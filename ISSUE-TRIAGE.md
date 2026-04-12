# Triage for Issue #650

**Status**: In Progress
**Created**: 2026-04-12

## Issue Information

- **Issue Number**: #650
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/650
- **Title**: `tide`: obsolete batch ProwJobs not aborted when a new batch supersedes them
- **Author**: @Prucek
- **Created**: 2026-03-11
- **State**: Open
- **Labels**: kind/feature, area/tide

## Initial Validation

**Assessment**: LEGITIMATE

### Analysis

The issue describes a missing cleanup behavior in Tide's batch processing: when the base branch SHA advances (due to a merge), Tide starts a new batch without aborting ProwJobs from the previous, now-obsolete batch. The old ProwJobs continue running to completion even though their results are no longer relevant, wasting CI resources.

**Issue Category**: Feature Request (reclassified from bug by maintainer @petr-muller)

**Repository Scope Check**:
- Component mentioned: Tide (batch merging subsystem)
- Exists in this repo: Yes (`pkg/tide/`)
- Relevant code paths: `pkg/tide/tide.go` (dividePool, batch triggering logic)

**Information Completeness**:
- Sufficient detail provided: Yes
- Clear reproduction steps: Start a batch, manually merge a PR in the same pool, observe new batch starts without cancelling old one
- Real-world example provided: Azure/ARO-HCP tide history with screenshot
- Expected behavior clearly stated: superseded batch ProwJobs should be aborted

### Recommendation

This is a legitimate feature request for Tide. The issue is well-written with clear reproduction steps, a concrete example, and a specific expected behavior. The maintainer has already confirmed it's valid but reclassified it from bug to feature.

**Suggested Action**:
- Keep open and continue triage

## Findings

(Further findings from triage subcommands will be added below)

## Next Steps

- Research: Investigate Tide batch lifecycle code to understand where cleanup should be added
- Assess effort: Determine complexity of implementing batch abort on supersede
- Augment: Improve issue with technical findings

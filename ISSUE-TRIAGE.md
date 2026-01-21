# Triage for Issue #436

**Status**: In Progress
**Created**: 2026-01-21

## Issue Information

- **Issue Number**: #436
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/436

## Findings

### Initial Validation

**Assessment**: LEGITIMATE

**Issue Category**: Feature Request

**Summary**: This issue requests functionality to prevent duplicate concurrent reruns of the same test job in Prow. During the Kubernetes 1.33.0 release, multiple release managers accidentally triggered reruns of the same failed test, resulting in 3 simultaneous runs of the same test, wasting resources.

**Repository Scope Check**:
- Component mentioned: Tide (for job scheduling/execution)
- Exists in this repo: Yes
- Labels applied: `kind/feature`, `area/tide`
- Relevant functionality: Job rerun/scheduling logic

**Information Completeness**:
- Sufficient detail provided: Yes
- Real-world use case: Kubernetes 1.33.0 release cut with multiple concurrent reruns
- Proposed solutions: (1) Discard additional runs if one is already scheduled/pending, or (2) GUI lock on rerun button
- Active maintainer discussion: BenTheElder noted there's an existing queue feature with n=1 maximum that could help, but it's not widely configured

**Analysis**:

This is a legitimate feature request for Prow's job execution system. The issue describes a real problem encountered during the Kubernetes release process where:
1. A test job failed (Conformance-GCE-1.33-kubetest2)
2. Multiple release managers were investigating simultaneously
3. Each triggered a rerun without knowing others had done the same
4. This resulted in 3 concurrent runs of the same test, wasting CI resources

The discussion reveals there's already a partial solution: a queue feature that can limit concurrent runs to n=1, used for resource-constrained jobs like the image promoter and scale tests. However, this isn't configured everywhere it should be.

The issue author proposes:
- Primary solution: Automatically discard duplicate rerun requests if a run is already scheduled/pending
- Alternative: Add a GUI lock on the rerun button (noted as complicated to implement)

This is a valid enhancement request that would improve resource utilization and prevent accidental duplicate runs, especially in high-stakes scenarios like release cuts.

### Recommendation

**Suggested Action**: Keep open and continue triage

This is a legitimate feature request with:
- Clear problem statement backed by real-world incident
- Proposed technical solutions
- Active maintainer engagement
- Appropriate labeling (kind/feature, area/tide)
- Ongoing interest from the community (issue kept alive by removing stale labels multiple times)

**Next Steps**:
1. Research the existing queue feature mentioned by BenTheElder
2. Investigate job scheduling/rerun code paths
3. Assess effort required for proposed solutions
4. Consider whether this should be a new feature or better documentation/defaults for existing queue feature

## Next Steps

(Action items will be added here)

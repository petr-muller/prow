# Triage for Issue #388

**Status**: In Progress
**Created**: 2025-12-23

## Issue Information

- **Issue Number**: #388
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/388
- **Title**: Job history cannot display the latest Runs
- **Reporter**: jianzhangbjz (external contributor)
- **Created**: 2025-02-26
- **Labels**: kind/bug, help wanted, area/deck
- **Assignee**: hector-vido
- **Status**: OPEN

## Summary

The job history page in Prow Deck is not displaying the latest job runs. When users navigate to a job's history page, it shows outdated runs that can be days or months behind the actual latest runs.

### Symptomatic Behavior

- Job history page displays runs from weeks/months ago instead of recent runs
- Examples:
  - OpenShift Prow: Showing Oct 20 runs when latest was same day (different time)
  - Kubernetes Prow: Showing January 9 when job runs continuously
  - Later checks: Showing August 3rd when October 8th runs exist
  - Results are inconsistent - different checks show different stale dates (possibly hitting different deck instances)

### Expected Behavior

Job history page should display the most recent job runs.

### Impact

- Users cannot see recent job history when clicking "Job History"
- Affects both OpenShift Prow (prow.ci.openshift.org) and Kubernetes Prow (prow.k8s.io)
- Makes it difficult to track recent job execution and results

### Reproduction Steps

1. Open a specific job run URL on Prow
2. Click "Job History"
3. Observe that latest runs are not displayed - showing outdated runs instead

### Key Discussion Points

- **BenTheElder's hypothesis**: "It feels like latest-build.txt isn't being read?"
  - References similar issue: https://github.com/kubernetes/test-infra/issues/34312
- **hector-vido**: Sometimes waiting helps and jobs appear later
- **jianzhangbjz**: Waiting doesn't work for them
- **BenTheElder**: Results are inconsistent, possibly due to hitting different deck instances
- **Reliable reproduction**: https://prow.k8s.io/job-history/gs/kubernetes-ci-logs/logs/ci-kubernetes-e2e-gci-gce

## Findings

### Initial Validation

**LEGITIMATE** - This is a valid bug report with:
- Clear reproduction steps
- Multiple affected users and Prow instances
- Specific examples provided
- Tagged appropriately (kind/bug, area/deck)
- Consistent reports from different contributors including Prow maintainers

### Technical Analysis

(Code research pending)

## Next Steps

(Action items will be added here)

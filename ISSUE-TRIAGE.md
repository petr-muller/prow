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

## Initial Validation

**Assessment**: LEGITIMATE

### Analysis

This issue reports a race condition in Tide's merge logic when GitHub Actions are being re-triggered. The issue provides comprehensive information:

1. **Clear Problem Description**: Tide incorrectly merges PRs while required GitHub Action checks are being re-triggered, before those checks have completed
2. **Concrete Evidence**: Example PR provided (kubernetes-sigs/security-profiles-operator/pull/2595) showing the problematic behavior
3. **Code Reference**: Author identified suspected code location at pkg/tide/status.go:478-492
4. **Proper Categorization**: Already labeled as kind/bug and area/tide

**Issue Category**: Bug

**Repository Scope Check**:
- Component mentioned: Tide
- Exists in this repo: Yes (verified pkg/tide/status.go exists)
- Relevant code paths: pkg/tide/status.go:478-492
- This is a core Prow component maintained in this repository

**Information Completeness**:
- Sufficient detail provided: Yes
- Issue includes:
  - Description of when the problem occurs
  - Screenshot showing the problematic state
  - Example PR demonstrating the issue
  - Reference to suspected race condition in code
  - Already has kind/bug label
- Missing information: None critical (reproduction steps could be added but the example PR serves this purpose)

### Recommendation

**Keep open and continue triage.** This is a valid bug report for the Tide component.

The issue clearly describes a race condition bug in Prow's Tide component. The reporter (saschagrunert, a MEMBER) has:
- Identified the specific problem (race during GitHub Action re-trigger)
- Provided evidence (screenshot and example PR)
- Referenced the likely problematic code
- Demonstrated persistence by preventing the issue from being closed as stale multiple times

A fix is already being developed in PR #563, which validates that this is a known, legitimate issue being actively worked on.

**Suggested Action**:
- Keep open and continue triage
- Review PR #563 to understand the proposed solution
- Examine the suspected code paths to understand the race condition
- Consider test coverage to prevent regression

**No comment needed**: Issue is already properly triaged and being actively addressed.

## Next Steps

1. Review PR #563 to understand the proposed fix
2. Examine the code at pkg/tide/status.go:478-492 to understand the race condition
3. Verify if the fix in PR #563 adequately addresses the race condition
4. Consider if additional test coverage is needed to prevent regression

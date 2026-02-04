# Triage for Issue #366

**Status**: In Progress
**Created**: 2026-02-04

## Issue Information

- **Issue Number**: #366
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/366

## Findings

### Initial Validation

**Assessment**: LEGITIMATE

**Issue Category**: Bug

**Issue Summary**:
The issue describes a bug in Tide's author matching logic for GitHub app accounts. When configuring a Tide query with `author: openshift-trt` (a GitHub app), Tide's status sync loop reports the PR as "In merge pool" but the merge loop never actually merges it. The workaround is to use `author: openshift-trt[bot]` instead, which works correctly.

**Analysis**:

1. **Clear Problem Statement**: The issue provides specific details:
   - Configuration that doesn't work (`author: openshift-trt`)
   - Configuration that does work (`author: openshift-trt[bot]`)
   - Observed behavior: Status shows "In merge pool" but PR never merges
   - No error messages or hints in logs about the mismatch

2. **Repository Scope Check**:
   - Component mentioned: Tide
   - Exists in this repo: Yes (pkg/tide/)
   - This is a core Prow component maintained in kubernetes-sigs/prow

3. **Information Completeness**:
   - Sufficient detail provided: Yes
   - Reproduction steps: Clear configuration examples provided
   - Expected vs actual behavior: Well documented
   - Author's hypothesis: Suggests it's related to GitHub GraphQL API usage in merge loop
   - One limitation: Actual PR is in private repo, but configuration alone is sufficient to understand the issue

4. **Root Cause Hypothesis**:
   The author suspects inconsistency between:
   - Status sync loop (which accepts `author: openshift-trt` and shows "In merge pool")
   - Merge loop (which doesn't recognize the PR without `[bot]` suffix, but fails silently)

5. **Requested Improvements**:
   - Primary: Make `[bot]` suffix unnecessary in author field
   - Secondary: Better error handling when status and merge loops disagree

### Recommendation

**Suggested Action**: Keep open and continue triage.

This is a legitimate bug report for Tide's author matching logic. The issue is well-documented with clear reproduction information and a working workaround. The inconsistency between Tide's status sync loop and merge loop represents a user experience problem that should be addressed.

The issue is already correctly labeled with:
- `kind/bug` - Appropriate categorization
- `area/tide` - Correct component area

Next steps: Proceed with research phase to investigate Tide's code and identify the root cause of the author matching discrepancy.

## Next Steps

(Action items will be added here)

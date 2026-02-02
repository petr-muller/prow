# Triage for Issue #134

**Status**: In Progress
**Created**: 2026-02-02

## Issue Information

- **Issue Number**: #134
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/134

## Findings

### Initial Validation

**Assessment**: LEGITIMATE

**Issue Category**: Bug

**Repository Scope Check**:
- Component mentioned: Tide
- Exists in this repo: Yes
- Relevant code paths: pkg/tide/ (merge logic and eligibility evaluation)
- Already labeled: kind/bug, area/tide

**Information Completeness**:
- Sufficient detail provided: Yes
- Configuration examples: Provided (both tide and branch-protection configs)
- Reproduction steps: Clear
- Root cause analysis: Already identified in issue comments by maintainer
- Community validation: Multiple users confirmed the behavior

### Analysis

This issue describes a legitimate architectural limitation in how Tide evaluates merge eligibility. The problem:

1. **Expected Behavior**: When GitHub branch protection requires N approving reviews (e.g., `required_approving_review_count: 2`), Tide should respect this and only merge PRs after N reviews are completed.

2. **Actual Behavior**: Tide merges PRs with fewer reviews than required, sometimes with just a `/lgtm` label and no actual GitHub review.

3. **Root Cause** (identified by @petr-muller in comments):
   - GitHub branch protection rules don't apply to repository admins by default
   - Tide requires admin permissions to bypass certain branch protections (e.g., for override functionality)
   - Tide's mergeability evaluation relies on labels (`lgtm`/`approved`) and job results, not GitHub's native review count requirements
   - This is a Tide architectural limitation as GitHub features have evolved

4. **Attempted Workarounds**:
   - Setting `enforce_admins: true` in branch-protection config was tested but:
     - May not fully resolve the issue (still merged with <2 reviews in one test)
     - Breaks Tide's ability to override required checks
     - Caused force push issues
   - Making Tide non-admin: Doesn't work (Tide requires admin permissions)

5. **Issue History**:
   - Migrated from kubernetes/test-infra
   - Multiple stale/remove-stale cycles showing sustained community interest
   - Active discussion with maintainer and affected users
   - No resolution yet after ~22 months

### Recommendation

**Suggested Action**: Keep open and continue triage.

This is a valid bug representing an architectural gap where Tide's label-based merge evaluation doesn't integrate with GitHub's native review count requirements. The issue is well-documented, has clear reproduction steps, and the root cause has been identified by a maintainer.

The issue requires architectural work to make Tide aware of and respect GitHub branch protection's review count settings, not just the presence/absence of approval labels.

## Next Steps

- Proceed to research phase to examine Tide's merge evaluation code
- Identify where GitHub branch protection checks could be integrated
- Assess effort level and propose solution approaches

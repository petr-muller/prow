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

### Code Research

#### Current Implementation

**Primary Components**:
- **Tide Controller**: pkg/tide/tide.go - Core sync controller and PR filtering logic
- **GitHub Provider**: pkg/tide/github.go - GitHub-specific merge checker and API integration
- **Status Controller**: pkg/tide/status.go - Status updates and requirement validation
- **Config Types**: pkg/config/tide.go - TideQuery configuration including ReviewApprovedRequired
- **Branch Protection**: pkg/config/branch_protection.go - ReviewPolicy with required approval count field

**Architecture Overview**:

Tide's merge decision flow operates through a filtering pipeline:

1. **Sync Loop** (tide.go:577): Periodically queries GitHub for PRs matching TideQuery criteria
2. **PR Filtering** (tide.go:730-793): Filters PRs through merge eligibility checks
3. **Merge Eligibility** (github.go:605-636): Validates if a PR can be merged
4. **Status Validation** (status.go:128-262): Checks labels, contexts, and review requirements

**Key Code Paths**:

1. **Query Construction**: config/tide.go:598 - `ReviewApprovedRequired: true` adds `review:approved` to GitHub search query
2. **GraphQL Data Fetch**: tide.go:1934 - PullRequest struct includes `ReviewDecision` field (binary: approved/not approved)
3. **Merge Filtering**: tide.go:755-793 - `filterPR()` calls `mergeAllowed()` and checks contexts
4. **Merge Checker**: github.go:605-636 - `isAllowedToMerge()` validates mergeable state, conflicts, and merge method
5. **Review Validation**: status.go:257-262 - `requirementDiff()` checks if `ReviewDecision == Approved` when ReviewApprovedRequired is set

**Data Flow**:

```
sync()
  → GitHub Search API (with review:approved if ReviewApprovedRequired)
  → filterSubpools(mergeAllowed callback)
    → filterPR()
      → mergeChecker.isAllowedToMerge()
        ├─ Check: Mergeable state (no conflicts)
        ├─ Check: Valid merge method
        ├─ Check: Repository allows merge method
        └─ [MISSING] Check: Required approval count
      → headContexts() - Get status checks
      → unsuccessfulContexts() - Filter failures
  → requirementDiff()
    ├─ Validate labels
    ├─ Validate contexts
    └─ Validate ReviewDecision (binary: approved or not)
       [MISSING] Validate approval count
```

**Critical Limitation**:

The `ReviewDecision` field from GitHub GraphQL API is an enum with values: `APPROVED`, `CHANGES_REQUESTED`, `REVIEW_REQUIRED`. It represents whether a PR has **at least one** approving review, not **how many** approvals it has. Tide never queries or validates the actual count of approving reviews against GitHub's branch protection requirement.

#### Related Code

**GitHub API Integration**:
- **GraphQL Query**: pkg/tide/github.go:165-212 - Searches for PRs and fetches ReviewDecision
- **PullRequest Type**: tide.go:1914-1946 - Contains ReviewDecision but no review count or detailed review data
- **Branch Protection Types**: pkg/github/types.go - GitHub REST API types include `RequiredApprovingReviewCount` field

**Branch Protection Configuration**:
- **ReviewPolicy**: config/branch_protection.go:85-96 - Includes `Approvals *int` field (maps to `required_approving_review_count`)
- **Config Parsing**: config/branch_protection.go - Reads branch protection from Prow YAML config
- **Limited Usage**: tide.go:2320-2327 - Branch protection config only used for `RequireManuallyTriggeredJobs`, not for review counts

**Dependencies**:
- **githubql**: External GraphQL library for GitHub API queries
- **GitHub REST API**: Used for branch protection sync (branchprotector component), but not directly queried by Tide for merge decisions

**Important Gap**:

Tide does NOT query GitHub's branch protection settings at merge time. Branch protection is configured in Prow YAML and synced to GitHub by the branchprotector component, but Tide doesn't read back those settings from GitHub to validate compliance.

#### Test Coverage

**Existing Tests**:

1. **Review Requirement Tests**: pkg/tide/status_test.go:717-731
   - Tests `ReviewApprovedRequired: true` with and without approving review
   - **Coverage**: Tests binary approval presence, NOT approval count
   - Test cases: "Missing approving review" and "Required approving review is present"

2. **Merge Checker Tests**: pkg/tide/github_test.go
   - Tests `isAllowedToMerge()` for various scenarios
   - **Coverage**: Tests mergeable state, conflicts, merge methods
   - **Gap**: No tests for approval count validation

3. **General Tide Tests**: pkg/tide/tide_test.go
   - Tests PR filtering and pool management
   - Uses mock ReviewDecision values

**Test Gaps**:

- **Missing**: Tests for PRs with 1 approval when 2+ required
- **Missing**: Tests for approval count mismatch between Tide behavior and branch protection
- **Missing**: Tests for detailed review data (states, counts, authors)
- **Missing**: Integration tests with actual GitHub branch protection settings

**Current Test Pattern**:

Tests set `hasApprovingReview: true` which sets `pr.ReviewDecision = githubql.PullRequestReviewDecisionApproved`, but this doesn't test approval counts.

#### Documentation Review

**Tide Configuration Docs**: site/content/en/docs/components/core/tide/config.md:59-62

```markdown
* `reviewApprovedRequired`: If set, each PR in the query must have at
  least one approved GitHub pull request review present for merge.
  Defaults to `false`.
```

**Key Documentation Points**:

1. Documentation explicitly states "at least one" approval, not a configurable count
2. `reviewApprovedRequired` maps to `review:approved` GitHub search query parameter
3. No mention of integration with GitHub's `required_approving_review_count` branch protection setting
4. No documented workaround for requiring multiple approvals

**Known Limitations**:

From issue comments and code analysis:
- Tide as admin bypasses GitHub branch protection by default
- Setting `enforce_admins: true` breaks other Tide functionality (required check overrides)
- No documented way to make Tide respect approval count requirements

#### Root Cause Analysis

**Primary Cause**:

Architectural gap where Tide's merge eligibility evaluation uses GitHub's binary `ReviewDecision` field (approved/not approved) rather than querying and validating the actual count of approving reviews against branch protection requirements.

**Technical Details**:

1. **Data Limitation**: The PullRequest GraphQL struct (tide.go:1934) only includes `ReviewDecision` field, which is an enum, not a count
2. **No Branch Protection Query**: Tide doesn't query GitHub's branch protection settings at merge time
3. **Binary Validation**: status.go:257-262 checks `ReviewDecision == Approved`, which only confirms ≥1 approval exists
4. **Admin Bypass**: Even if GitHub branch protection requires N approvals, Tide's admin permissions bypass this enforcement

**Contributing Factors**:

1. **Configuration Source**: Branch protection configured in Prow YAML and synced TO GitHub, but not read back FROM GitHub
2. **API Design**: GraphQL ReviewDecision field designed for binary approval state, not granular counting
3. **Label-Based History**: Tide evolved from label-based workflows (`lgtm`/`approved` labels), not GitHub native reviews
4. **Admin Requirements**: Tide needs admin permissions for legitimate features (override failed checks, batch merging)

**Reproduction Conditions**:

- GitHub branch protection requires `required_approving_review_count: 2` (or higher)
- Tide has admin permissions on the repository
- PR has 1 approving review and meets all other merge criteria
- TideQuery uses `ReviewApprovedRequired: true` (which only checks for ≥1 approval)
- Result: Tide merges the PR despite missing required approvals

#### Proposed Solutions

##### Approach 1: Query GitHub Branch Protection at Merge Time

**Description**:

Extend Tide to query GitHub's branch protection settings via REST API when evaluating merge eligibility, and validate the actual approval count from detailed review data against the required count.

**Implementation Steps**:

1. Extend PullRequest GraphQL query to fetch detailed review data:
   ```graphql
   reviews(first: 100) {
     totalCount
     nodes {
       state  # APPROVED, CHANGES_REQUESTED, DISMISSED, etc.
       author { login }
     }
   }
   ```

2. Add GitHub REST API call in `isAllowedToMerge()` to fetch branch protection:
   ```
   GET /repos/{owner}/{repo}/branches/{branch}/protection
   ```

3. Count APPROVED reviews (excluding dismissed) and compare against `required_approving_review_count`

4. Block merge if approval count insufficient, with clear status message

**Pros**:
- Accurately respects GitHub branch protection settings
- Works regardless of whether protection is configured via Prow or GitHub UI
- Provides clear feedback when approval count is insufficient
- No need for `enforce_admins` workaround

**Cons**:
- Additional GitHub API calls (rate limit impact)
- Requires parsing complex branch protection response
- Need to handle CODEOWNERS review requirements
- Performance impact on sync loop

**Affected Components**:
- PullRequest GraphQL struct (tide.go:1914) - Add review details
- `isAllowedToMerge()` (github.go:605) - Add branch protection query
- `requirementDiff()` (status.go:257) - Add approval count validation
- GitHub client interface - Add GetBranchProtection method

**Complexity**: Medium-High

**Backwards Compatibility**:
- Fully compatible - adds validation, doesn't remove features
- May block PRs that currently merge incorrectly (this is desired behavior)
- No config changes required

##### Approach 2: Extend TideQuery with MinimumApprovals Field

**Description**:

Add a new `MinimumApprovals` field to TideQuery configuration, allowing maintainers to specify required approval count in Prow config without querying GitHub branch protection.

**Implementation Steps**:

1. Add `MinimumApprovals *int` to TideQuery struct (config/tide.go:504)

2. Extend PullRequest GraphQL query to fetch review count (same as Approach 1)

3. In `requirementDiff()`, validate approval count ≥ MinimumApprovals

4. Keep existing `ReviewApprovedRequired` for backward compatibility (acts as MinimumApprovals=1)

**Pros**:
- No additional GitHub API calls (better performance)
- Simple implementation
- Explicit configuration in Prow YAML
- No branch protection query complexity

**Cons**:
- Requires manual configuration - doesn't auto-sync with GitHub branch protection
- Two sources of truth (Prow config and GitHub branch protection)
- If GitHub branch protection changes, Prow config must be updated separately
- Doesn't solve the fundamental disconnect between Tide and GitHub settings

**Affected Components**:
- TideQuery struct (config/tide.go:504) - Add MinimumApprovals field
- PullRequest GraphQL query (tide.go:1914) - Add review details
- `requirementDiff()` (status.go:257) - Add approval count validation

**Complexity**: Low-Medium

**Backwards Compatibility**:
- Fully compatible - new optional field
- `ReviewApprovedRequired: true` continues to work (equivalent to MinimumApprovals=1)

##### Approach 3: Hybrid - Config Option to Enable Branch Protection Sync

**Description**:

Add a Tide configuration option `SyncFromBranchProtection: true` that, when enabled, queries GitHub branch protection for approval requirements. Falls back to TideQuery settings when disabled.

**Implementation Steps**:

1. Add `SyncFromBranchProtection *bool` to TideContextOptions (config/tide.go)

2. When enabled, query GitHub branch protection (Approach 1)

3. When disabled, use TideQuery MinimumApprovals field (Approach 2)

4. Cache branch protection responses to minimize API calls

**Pros**:
- Best of both worlds - accurate when needed, performant when not
- Gradual migration path for deployments
- Respects both Prow config and GitHub settings
- Caching reduces API call overhead

**Cons**:
- Most complex implementation
- More configuration surface area
- Requires careful cache invalidation strategy
- Needs thorough testing of both modes

**Complexity**: High

**Backwards Compatibility**:
- Fully compatible - new optional feature
- Defaults to current behavior (SyncFromBranchProtection=false)

#### Recommendation

**Preferred Approach**: **Approach 1 - Query GitHub Branch Protection at Merge Time**

**Rationale**:

This approach addresses the fundamental architectural issue: Tide should respect GitHub's authoritative branch protection settings rather than maintaining a parallel configuration system. While it adds API calls, this is the correct long-term design:

1. **Single Source of Truth**: GitHub branch protection is the authoritative source, whether set via branchprotector or GitHub UI
2. **Accurate Behavior**: Eliminates the bug entirely - Tide will never merge with insufficient approvals
3. **Feature Completeness**: Supports future GitHub features (CODEOWNERS requirements, etc.) automatically
4. **Clear Mental Model**: Users expect Tide to respect branch protection, not ignore it

**Key Implementation Considerations**:

1. **API Rate Limits**:
   - Cache branch protection responses (TTL: 5-10 minutes)
   - Only query when PR passes other eligibility checks (optimization)
   - Use conditional requests (If-None-Match) to minimize quota usage

2. **Review Counting Logic**:
   - Count only APPROVED reviews (exclude DISMISSED, CHANGES_REQUESTED)
   - Handle CODEOWNERS requirements (GitHub API indicates this separately)
   - Respect "Dismiss stale reviews" setting

3. **Performance**:
   - Fetch branch protection and review details in parallel GraphQL/REST calls
   - Implement caching layer for branch protection by (org, repo, branch)
   - Monitor API rate limit consumption and add metrics

4. **Error Handling**:
   - If branch protection query fails, log error and block merge (fail closed)
   - Provide clear status message: "Unable to verify approval requirements"
   - Add metric for branch protection query errors

5. **Status Messages**:
   - Clear feedback: "Needs 1 more approving review (1/2 required)"
   - Distinguish between: no approvals, insufficient approvals, dismissed approvals

**Testing Requirements**:

1. **Unit Tests**:
   - Mock branch protection responses with various approval count requirements
   - Test review counting logic (approved, dismissed, changes requested)
   - Test caching behavior and TTL

2. **Integration Tests**:
   - Create test repos with branch protection requiring 2+ approvals
   - Verify Tide blocks merge with insufficient approvals
   - Verify Tide allows merge when approval count met

3. **Edge Cases**:
   - CODEOWNERS review requirements
   - Dismissed reviews
   - Branch protection disabled (should fall back gracefully)
   - API failures (should block merge)

**Migration/Rollout Strategy**:

1. **Phase 1**: Implement branch protection query and review counting, but only log mismatches (no blocking)
2. **Phase 2**: Add feature flag to enable blocking behavior for opt-in testing
3. **Phase 3**: Enable blocking by default, with opt-out for gradual rollout
4. **Phase 4**: Remove opt-out after validation period

This phased approach allows monitoring impact and catching edge cases before full enforcement.

## Next Steps

- Proceed to effort assessment to categorize complexity level
- Determine appropriate labels (good-first-issue, help-wanted, or neither)
- Create augmentation proposal for issue

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

## Code Research

### Current Implementation

**Primary Components**:
- **Sync Controller (Merge Loop)**: pkg/tide/tide.go:531-623 - Main sync loop that determines which PRs to merge
- **GitHub Provider Query**: pkg/tide/github.go:101-150 - Executes GitHub searches to find PRs matching Tide queries
- **Query Construction**: pkg/config/tide.go:504-602 - Builds GitHub search query strings from TideQuery configuration
- **Status Controller**: pkg/tide/status.go - Updates GitHub status checks on PRs
- **Author Normalization**: pkg/github/types.go:166-168 - NormLogin function for normalizing GitHub usernames

**Architecture Overview**:

Tide has two main control loops that evaluate PRs:

1. **Sync Controller (Merge Loop)** - Determines which PRs to actually merge
   - Queries GitHub search API with author filter in search string
   - Returns only PRs matching the query (server-side filtering)
   - These PRs form the "merge pool"
   - Executes merge operations on pooled PRs

2. **Status Controller** - Updates status checks on PRs
   - Queries GitHub for ALL open PRs (no author filter in search)
   - Client-side evaluation using `requirementDiff()` to check query matching
   - Sets GitHub status to "In merge pool" or "Not mergeable" based on evaluation

**Key Code Paths**:

1. **Query Construction** (config/tide.go:576):
   ```go
   if tq.Author != "" {
       queryString = append(queryString, fmt.Sprintf("author:\"%s\"", tq.Author))
   }
   ```
   The author field is inserted literally into the GitHub search query.

2. **Sync Loop GitHub Search** (github.go:122):
   - Executes: `gi.search()` with query string including `author:"openshift-trt"`
   - GitHub API returns only PRs where author login matches exactly
   - PRs by `openshift-trt[bot]` are NOT returned when query is `author:"openshift-trt"`
   - No client-side filtering; relies entirely on GitHub's search results

3. **Status Loop PR Search** (status.go:612-732):
   ```go
   func openPRsQueries(...) map[string]string {
       result[org] = "is:pr state:open sort:updated-asc archived:false " + query
   }
   ```
   - Searches for ALL open PRs in configured orgs/repos
   - Does NOT include author filter from Tide queries in the search
   - Returns all PRs, regardless of author

4. **Status Loop Author Matching** (status.go:169-179):
   ```go
   qAuthor := github.NormLogin(q.Author)
   prAuthor := github.NormLogin(string(pr.Author.Login))
   if qAuthor != "" && prAuthor != qAuthor {
       diff += 1000
       desc = fmt.Sprintf(" Must be by author %s.", qAuthor)
   }
   ```
   Client-side comparison using `NormLogin()` normalization.

5. **NormLogin Implementation** (github/types.go:166-168):
   ```go
   func NormLogin(login string) string {
       return strings.TrimPrefix(strings.ToLower(login), "@")
   }
   ```
   Only strips `@` prefix and lowercases - **does NOT strip `[bot]` suffix**.

**Data Flow**:

**Sync Controller Flow**:
1. Tide sync loop builds query: `author:"openshift-trt" is:pr state:open ...`
2. Sends query to GitHub GraphQL search API
3. GitHub returns PRs where author login equals exactly "openshift-trt"
4. PRs by "openshift-trt[bot]" are excluded (no match)
5. Excluded PRs never enter merge pool
6. No merging happens for these PRs

**Status Controller Flow**:
1. Status loop builds query: `is:pr state:open archived:false org:openshift-eng repo:ci-test-mapping`
2. Sends query to GitHub (no author filter)
3. GitHub returns ALL open PRs including those by "openshift-trt[bot]"
4. For each PR, evaluates `requirementDiff()` against all Tide queries
5. Compares: NormLogin("openshift-trt") vs NormLogin("openshift-trt[bot]")
6. Result: "openshift-trt" ≠ "openshift-trt[bot]" → diff = 1000
7. Should show: "Not mergeable. Must be by author openshift-trt."

### Related Code

**GitHub API Behavior**:
- GitHub bot accounts have usernames ending in `[bot]` suffix
- The suffix is part of the username: `openshift-trt[bot]`
- GitHub search query `author:"openshift-trt"` matches the literal username
- To match bot accounts in search, must use `author:"openshift-trt[bot]"`
- API responses always include the full username with `[bot]` suffix

**User Type Field** (github/types.go:161-162):
- GitHub API provides `User.Type` field: "User" or "Bot"
- Could be used to identify bot accounts programmatically
- Currently NOT used in author matching logic

### Test Coverage

**Existing Tests**:
- pkg/tide/status_test.go: Tests for status controller functionality
- Likely lacks specific test cases for bot author matching
- Need to verify if tests cover author normalization edge cases

**Test Gaps**:
- No test for GitHub app/bot authors with `[bot]` suffix
- No test verifying sync and status controllers handle authors consistently
- Missing tests for `NormLogin()` behavior with bot usernames

### Root Cause Analysis

**Primary Cause**:

Inconsistent author matching between sync controller (merge loop) and status controller. The sync controller relies on GitHub's search API server-side filtering, while the status controller uses client-side comparison with inadequate normalization.

**Specific Issues**:

1. **NormLogin Insufficient for Bot Accounts**: The `NormLogin()` function only strips `@` prefix and lowercases, but GitHub bot accounts have `[bot]` suffix that should be normalized or handled specially.

2. **Sync vs Status Query Mismatch**:
   - Sync controller: Uses `author:"openshift-trt"` in GitHub search (server-side)
   - Status controller: Uses no author filter in search, then client-side comparison
   - GitHub search treats `author:"openshift-trt"` as literal match (excludes `openshift-trt[bot]`)
   - Client-side `NormLogin()` comparison also fails because suffix isn't stripped

3. **Silent Failure**: When a PR doesn't make it into the merge pool due to author mismatch, there's no logging or status indication that explains the GitHub search didn't return the PR. Users only see that merging doesn't happen.

**Contributing Factors**:

1. **GitHub API Naming Convention**: GitHub automatically appends `[bot]` to app-created accounts, but users might not know to include this in Tide configuration

2. **Lack of Author Type Awareness**: Code doesn't distinguish between human users and bot accounts, treating all authors as plain strings

3. **Two Code Paths**: Having separate query evaluation logic in sync and status controllers increases risk of inconsistency

**Reproduction Conditions**:
- Tide query configured with `author: <bot-name>` without `[bot]` suffix
- PR created by GitHub app with login `<bot-name>[bot]`
- Result: PR excluded from merge pool, with confusing or absent status feedback

### Proposed Solutions

#### Approach 1: Normalize Bot Suffixes in Author Matching

**Description**: Enhance `NormLogin()` or create a specialized author matching function that normalizes both `[bot]` suffixes and handles bot account name variations. Apply this consistently in both sync and status controllers.

**Implementation Details**:
- Update `NormLogin()` to strip `[bot]` suffix: `strings.TrimSuffix(login, "[bot]")`
- OR: Create new `NormAuthor()` function specifically for Tide author matching
- Use the same normalization in:
  - Query construction (strip suffix before building search query)
  - Status controller comparison (strip suffix before comparing)
- This makes `author: openshift-trt` match PRs by `openshift-trt[bot]`

**Pros**:
- Intuitive user experience - users don't need to know about `[bot]` suffix
- Consistent handling across all code paths
- Simple implementation - localized change
- Backwards compatible if done carefully

**Cons**:
- Changes matching semantics - could affect existing configurations
- If user explicitly wants to match only non-bot accounts, can't distinguish
- Need to handle edge cases (what if someone names a user account with `[bot]` in it?)

**Affected Components**:
- pkg/github/types.go: Update NormLogin or add NormAuthor
- pkg/config/tide.go: Update query construction to normalize author
- pkg/tide/status.go: Already uses NormLogin, would inherit fix

**Complexity**: Low

**Backwards Compatibility**:
- Risk: Existing queries with `author: foo[bot]` would start normalizing to `foo`
- Mitigation: Document in release notes; most users likely already using workaround

#### Approach 2: Support Both With and Without Suffix in Search

**Description**: When constructing GitHub search queries, detect if author might be a bot and create a search that matches both `<name>` and `<name>[bot]`. Use GitHub's OR syntax or User.Type filtering.

**Implementation Details**:
- In query construction, generate: `author:"openshift-trt" OR author:"openshift-trt[bot]"`
- OR: Use GitHub's `type:pr` with additional filtering on User.Type field
- Status controller continues using client-side logic with enhanced matching

**Pros**:
- Explicitly handles both cases
- Users can still specify exact author if needed
- Clear intent in generated queries

**Cons**:
- More complex query construction
- Longer search query strings
- May not work well with GitHub's query syntax limits
- Doesn't solve client-side comparison in status controller

**Complexity**: Medium

**Backwards Compatibility**: Fully compatible - expands matching, doesn't restrict

#### Approach 3: Enhanced Error Messages and Validation

**Description**: Keep current behavior but add validation and better error messages. When a Tide query has an author that looks like a bot name (common pattern), warn users to add `[bot]` suffix. Improve status messages to indicate author mismatch.

**Implementation Details**:
- Add configuration validation that warns if `author` field doesn't end in `[bot]` but matches common bot naming patterns
- Enhance logging in sync controller to show when PRs are excluded due to author mismatch
- Improve status controller messages to be more explicit about `[bot]` suffix requirement

**Pros**:
- No behavior changes - zero risk to existing deployments
- Helps users discover the issue quickly
- Better operational visibility

**Cons**:
- Doesn't fix the underlying problem
- Users still need to know about `[bot]` suffix
- Band-aid solution rather than proper fix

**Complexity**: Low

**Backwards Compatibility**: Fully compatible - no behavior change

#### Recommended Approach

**Preferred Approach**: **Approach 1 (Normalize Bot Suffixes) with Enhanced Documentation**

**Rationale**:
- Most intuitive user experience - author matching "just works" for bots
- Solves the root cause rather than working around it
- Relatively simple implementation with low risk
- Can be combined with better error messages from Approach 3

**Key Implementation Considerations**:

1. **Normalization Function**:
   - Create `NormAuthor(author string) string` that strips both `@` prefix and `[bot]` suffix
   - Keep `NormLogin()` unchanged to avoid unintended side effects
   - Use `NormAuthor()` specifically for Tide author matching

2. **Apply Normalization in Both Paths**:
   - **Query construction** (config/tide.go:576): Normalize author before building search query
   - **Status evaluation** (status.go:169): Use NormAuthor instead of NormLogin for author comparison

3. **User.Type Awareness** (Future Enhancement):
   - Consider using GitHub's User.Type field to distinguish bots programmatically
   - Could enable more sophisticated matching (e.g., `author:foo type:bot`)

4. **Configuration Validation**:
   - Add warning if author field contains `[bot]` suffix (now unnecessary)
   - Document the change in migration guide

**Testing Requirements**:
- Unit test for NormAuthor with various inputs: `foo`, `foo[bot]`, `Foo[BOT]`, `@foo[bot]`
- Integration test with mock GitHub app PR
- Test sync controller query matches bot PRs
- Test status controller evaluation matches bot PRs
- Verify existing human author matching still works

**Migration/Rollout Strategy**:
- Add normalization function in pkg/github/types.go
- Update Tide query construction to use normalization
- Update status controller to use normalization
- Add release note documenting behavior change
- Recommend users can remove `[bot]` suffixes from configs (but leaving them doesn't break anything)

## Next Steps

1. Proceed with effort assessment to categorize issue difficulty
2. Prepare augmentation to improve issue description and labels
3. Brief maintainer on findings
4. Finalize triage and post recommendations

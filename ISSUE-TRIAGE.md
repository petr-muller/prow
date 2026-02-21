# Triage for Issue #438

**Status**: In Progress
**Created**: 2026-02-21

## Issue Information

- **Issue Number**: #438
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/438

## Initial Validation

**Assessment**: LEGITIMATE

### Analysis

The issue requests adding regex support for branch matching in Tide's `includedBranches` and `excludedBranches` fields. Currently these fields only support exact string matching, while the branchprotector component already supports regex patterns for branch names. The author argues this inconsistency forces users to list each branch explicitly, even when branches follow a pattern (e.g., `release-*`, `feature-*`).

**Issue Category**: Feature Request

**Repository Scope Check**:
- Component mentioned: Tide status controller (`pkg/tide/status.go`)
- Exists in this repo: Yes
- Relevant code paths: `pkg/tide/status.go`, `pkg/tide/status_test.go`, `pkg/config/tide.go`, `pkg/config/tide_test.go`
- Reference component: `cmd/branchprotector/protect.go` (existing regex support)

**Information Completeness**:
- Sufficient detail provided: Yes
- The issue clearly describes the current behavior, desired behavior, and references existing regex support in branchprotector as precedent

### Recommendation

This is a valid feature request for a Prow component that lives in this repository. The request is well-reasoned: it addresses an inconsistency between two Prow components and has a clear precedent in the branchprotector implementation. The author (@kaovilai) has actively maintained the issue by removing stale labels twice, indicating continued interest. A maintainer (petr-muller) has already labeled it with `area/tide` and `kind/feature` and noted that #482 may be related.

**Suggested Action**:
- Keep open and continue triage

## Code Research

### Current Implementation

**Primary Components**:
- Tide Status Controller: `pkg/tide/status.go` - Evaluates PR status for merge eligibility, including branch matching
- Tide Config: `pkg/config/tide.go` - Defines `TideQuery` struct with `IncludedBranches`/`ExcludedBranches` fields
- Branchprotector: `cmd/branchprotector/protect.go` - Already implements regex-based branch matching

**Architecture Overview**:
Tide queries define which PRs are eligible for merging via `TideQuery` structs. Branch filtering is done at two levels: (1) GitHub Search API queries using the `base:` operator for initial PR fetching, and (2) local exact-string comparison in `requirementDiff()` for final status evaluation. Both levels currently use exact string matching only.

**Key Code Paths**:
1. `TideQuery` struct definition: `pkg/config/tide.go:504-520` - `ExcludedBranches` and `IncludedBranches` are `[]string`
2. Branch matching logic: `pkg/tide/status.go:148-157` - Uses `==` comparison (exact match)
3. GitHub search query construction: `pkg/config/tide.go:578-583` - Builds `base:"branch"` search operators
4. Validation: `pkg/config/tide.go:817-825` - Checks duplicates and mutual exclusivity only

**Data Flow**:
1. Config loads `TideQuery` with `ExcludedBranches`/`IncludedBranches` as string slices
2. `TideQuery.Validate()` checks for duplicates and mutual exclusivity
3. `TideQuery.constructQuery()` builds GitHub search string with `base:"branch"` operators
4. GitHub API returns PRs matching exact branch names
5. For each PR, `requirementDiff()` does `==` comparison against each branch string
6. Unmatched branches receive a diff weight of 2000 (effectively blocking merges)

### Related Code

**Branchprotector (precedent for regex)**:
- Config: `pkg/config/branch_protection.go:56-59` - `Exclude`/`Include` fields documented as regex patterns
- Regex compilation: `cmd/branchprotector/protect.go:312-327` - Compiles with `regexp.Compile(strings.Join(patterns, "|"))`
- Matching: `cmd/branchprotector/protect.go:336-345` - Uses `branchInclusions.MatchString(b.Name)`

**Tide merge method (also uses regex)**:
- `TideBranchMergeType` struct: `pkg/config/tide.go:42-49` - Has `Regexpr *regexp.Regexp` field
- Compilation: `pkg/config/config.go:2850-2864` - Compiles regex during `parseTideMergeType()`
- Usage: `TideBranchMergeType.Match()` uses `Regexpr.MatchString(branch)`

**Job branch filtering (also uses regex)**:
- `Brancher` struct: `pkg/config/jobs.go:400-414` - Uses `CopyableRegexp` for `Branches`/`SkipBranches`
- `ShouldRun()` uses `br.re.MatchString(branch)` and `br.reSkip.MatchString(branch)`

### Test Coverage

**Existing Tests**:
- `pkg/tide/status_test.go:211-246` - Tests branch matching with exact strings ("bad", "good")
- `pkg/config/tide_test.go:1735-1866` - Tests query construction with branch strings
- Coverage assessment: Good for exact matching, no regex test cases exist

**Test Gaps**:
- No tests for regex-based branch matching patterns
- No tests for regex compilation errors at config validation time
- No tests for backwards compatibility (exact strings still working as regex)

### Root Cause Analysis

**Primary Cause**:
This is not a bug but a feature gap. The `TideQuery` implementation predates the regex support added to branchprotector and merge method configuration. The branch fields were implemented as simple string slices with exact matching, and were never updated to support regex patterns even as other components adopted regex.

**Contributing Factors**:
1. GitHub Search API's `base:` operator also uses exact matching, so the initial implementation aligned with the API
2. Different components were developed independently without enforcing consistency
3. No shared utility exists for branch matching patterns across Prow components

### Proposed Solutions

#### Approach 1: Add Regex Support to TideQuery (Recommended)

**Description**: Add compiled regex fields to `TideQuery` (or a wrapper), compile during config validation, and update `requirementDiff()` to use `MatchString()` instead of `==`.

**Pros**:
- Consistent with branchprotector and merge method patterns
- Backwards compatible (exact strings are valid regex)
- Well-established pattern in the codebase to follow
- Client-side matching independent of GitHub API limitations

**Cons**:
- GitHub Search API still uses exact `base:` matching, so regex patterns won't filter at the API level (only at the local evaluation level in `requirementDiff()`)
- May return more PRs from GitHub API than needed (performance consideration)
- Potential for users to write expensive regex patterns

**Affected Components**:
- `pkg/config/tide.go`: Add regex compilation during validation
- `pkg/tide/status.go`: Update `requirementDiff()` to use regex matching
- `pkg/config/tide_test.go` and `pkg/tide/status_test.go`: Add regex test cases

**Complexity**: Low-Medium

**Backwards Compatibility**: Full - exact strings are valid regex patterns

#### Approach 2: Separate Regex Fields

**Description**: Add new fields `IncludedBranchPatterns` and `ExcludedBranchPatterns` alongside existing fields, keeping exact matching for the original fields.

**Pros**:
- No risk of breaking existing configs
- Clear distinction between exact and regex matching
- Can use GitHub API for exact matches, client-side for regex

**Cons**:
- API surface bloat with redundant fields
- Inconsistent with branchprotector which uses the same fields for regex
- More complex validation (interaction between old and new fields)

**Complexity**: Medium

**Backwards Compatibility**: Full (additive change)

#### Recommendation

**Preferred Approach**: Approach 1 (Add regex support to existing fields)

This is the simplest path and follows the established branchprotector pattern. Since exact strings are valid regex, it's fully backwards compatible. The GitHub API limitation (no regex in `base:` operator) means slightly more PRs may be fetched, but the local filtering in `requirementDiff()` will correctly narrow results.

**Key Implementation Considerations**:
1. Compile regex patterns during config validation (fail fast on invalid patterns)
2. For `constructQuery()`, skip regex patterns in GitHub search queries (they won't work with `base:` operator anyway) or use them only for local filtering
3. Update `requirementDiff()` to use `MatchString()` instead of `==`
4. Follow `TideBranchMergeType` pattern for storing compiled regexes

**Testing Requirements**:
- Test regex patterns like `release-\d+`, `feature-.*`, `main|master`
- Test that exact strings still work as before
- Test invalid regex patterns are caught during validation
- Test interaction between GitHub search (exact) and local regex filtering

## Next Steps

- Assess effort level
- Augment the issue with technical findings

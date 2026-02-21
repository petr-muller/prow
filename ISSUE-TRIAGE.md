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
Tide queries define which PRs are eligible for merging via `TideQuery` structs. Branch filtering happens at two levels with very different roles:

1. **Primary (functional)**: `GitHubProvider.Query()` (`pkg/tide/github.go:102`) calls `TideQuery.OrgQueries()`/`Query()`, which calls `constructQuery()` to build GitHub Search API queries with `base:"branch"` operators. This is how Tide discovers which PRs to consider for merging. **This is the core functionality and it depends entirely on the GitHub Search API's `base:` operator, which only supports exact string matching.**

2. **Secondary (informational)**: `requirementDiff()` in `pkg/tide/status.go:148-157` does local `==` comparison to produce status context messages explaining to users why their PR is or isn't in the merge pool. This is only for user-facing status messages, not for actual merge decisions.

**Key Code Paths**:
1. `TideQuery` struct definition: `pkg/config/tide.go:504-520` - `ExcludedBranches` and `IncludedBranches` are `[]string`
2. PR discovery (primary): `pkg/tide/github.go:102-115` - `GitHubProvider.Query()` calls `OrgQueries()`/`Query()` for each Tide query
3. GitHub search query construction: `pkg/config/tide.go:554-602` - `constructQuery()` builds `base:"branch"` search operators (lines 578-583)
4. Status messages (secondary): `pkg/tide/status.go:148-157` - `requirementDiff()` uses `==` comparison for informational messages only
5. Validation: `pkg/config/tide.go:817-825` - Checks duplicates and mutual exclusivity only

**Data Flow**:
1. Config loads `TideQuery` with `ExcludedBranches`/`IncludedBranches` as string slices
2. `TideQuery.Validate()` checks for duplicates and mutual exclusivity
3. **Main Tide controller**: `GitHubProvider.Query()` calls `constructQuery()` which builds GitHub search strings with `base:"branch"` and `-base:"branch"` operators
4. GitHub Search API returns PRs matching exact branch names -- **regex is not supported by this API**
5. **Status controller** (separate path): `requirementDiff()` does `==` comparison for producing user-facing status messages only

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
This is not a bug but a feature gap with a fundamental architectural constraint. Tide's PR discovery is driven entirely by the GitHub Search API, which does not support regex in the `base:` operator. The `includedBranches`/`excludedBranches` fields map directly to `base:"branch"` / `-base:"branch"` search operators in `constructQuery()`. The branchprotector comparison is misleading: branchprotector enumerates all branches via GitHub's Branches API first and then filters locally with regex. Tide does not enumerate branches -- it relies on GitHub Search to return only matching PRs.

**Contributing Factors**:
1. Tide's architecture is search-driven: `constructQuery()` builds a GitHub Search query, and the results are what Tide works with. Branch filtering is delegated to the API.
2. `requirementDiff()` (status.go:148-157) does branch matching but only for informational status messages, not for actual merge decisions
3. Branchprotector can use regex because it fetches all branches first via `GetBranches()` and filters locally -- Tide has no equivalent branch enumeration step
4. The GitHub Search API's `base:` qualifier only supports exact string matching

### Proposed Solutions

#### Approach 1: Enumerate Branches and Expand Regex to Exact Matches

**Description**: Before constructing GitHub Search queries, enumerate branches from the GitHub Branches API, match them against regex patterns, and expand into explicit `base:"branch-1" base:"branch-2"` search operators.

**Pros**:
- Works within the existing GitHub Search API constraints
- Exact matching behavior preserved at the API level
- Follows how branchprotector handles this (enumerate then filter)

**Cons**:
- Requires additional GitHub API calls to list branches for every repo in every query
- Branch lists can be large (hundreds or thousands of branches)
- Adds latency to Tide's sync loop
- GitHub API rate limit impact
- Must handle the case where regex matches no branches
- Must handle branches created between enumeration and search
- Scope extends beyond config/status into the Tide controller and GitHub provider

**Affected Components**:
- `pkg/config/tide.go`: Regex compilation during validation, new regex fields
- `pkg/tide/github.go`: Branch enumeration before query construction
- `pkg/tide/status.go`: Update `requirementDiff()` for informational messages
- Test files for all affected components

**Complexity**: Medium-High

**Backwards Compatibility**: Full - exact strings are valid regex

#### Approach 2: Drop Branch Filter from Search, Filter Locally

**Description**: When regex patterns are used in `includedBranches`/`excludedBranches`, omit the `base:` operators from the GitHub Search query and instead filter PRs locally after fetching results.

**Pros**:
- Simpler than branch enumeration
- No additional API calls for branch listing
- Regex matching is straightforward locally

**Cons**:
- Fetches significantly more PRs from GitHub than needed (all branches instead of specific ones)
- Performance impact for large orgs with many open PRs
- Increases GitHub API usage for PR fetching (pagination)
- Changes the existing query flow substantially
- Must filter at the right point in the pipeline (before merge decisions)

**Complexity**: Medium

**Backwards Compatibility**: Full, but performance characteristics change

#### Approach 3: Separate Regex Fields with Branch Enumeration

**Description**: Add new fields `IncludedBranchPatterns`/`ExcludedBranchPatterns` alongside existing exact-match fields. Exact fields continue using GitHub Search `base:` operator. Pattern fields trigger branch enumeration and local expansion.

**Pros**:
- Clear distinction between exact (fast, API-level) and regex (slower, requires enumeration)
- No risk of breaking existing configs
- Users opt in to the additional API cost explicitly
- Can optimize: only enumerate when patterns are present

**Cons**:
- API surface bloat
- Inconsistent with branchprotector which reuses the same fields
- More complex validation (interaction between old and new fields)

**Complexity**: Medium-High

**Backwards Compatibility**: Full (additive change)

#### Recommendation

**No clearly preferred approach.** Each approach has significant trade-offs:

- **Approach 1** (enumerate and expand) is architecturally cleanest but adds API calls and latency
- **Approach 2** (local filtering) is simplest but may degrade performance for large orgs
- **Approach 3** (separate fields) is most explicit but adds config complexity

The choice depends on maintainer priorities: API budget vs. config simplicity vs. performance. This is a design decision that should be discussed before implementation begins.

**Key Implementation Considerations**:
1. All approaches require regex compilation during config validation
2. The GitHub Search API limitation is the fundamental constraint -- there is no simple "swap `==` for `MatchString()`" fix
3. Branchprotector's pattern works because it operates on a different model (enumerate all branches then filter), not because the regex matching itself is complex
4. `requirementDiff()` changes are trivial but insufficient on their own -- the primary path through `constructQuery()` must be addressed
5. Issue #482 (GitHub Search API improvements) could be relevant if GitHub adds regex support for `base:` in the future

**Testing Requirements**:
- Test regex pattern compilation and validation
- Test branch enumeration and expansion (Approach 1) or local filtering (Approach 2)
- Test backwards compatibility with exact strings
- Test performance impact with realistic branch counts
- Integration tests for the full Tide query flow with regex patterns

## Effort Assessment

**Effort Level**: 3 - Large (requires expertise)

### Summary

The core challenge is not regex matching itself but working around the GitHub Search API's lack of regex support for the `base:` operator, which is the primary mechanism Tide uses for PR discovery. Unlike `requirementDiff()` (which only produces informational status messages), the main Tide controller relies on `constructQuery()` -> GitHub Search to discover PRs. Any solution requires either enumerating branches via additional API calls, fetching more PRs and filtering locally, or introducing new config fields -- all of which involve significant design trade-offs requiring maintainer input.

### Factor Analysis

#### Scope of Changes
- **Assessment**: Moderate-Large
- **Details**: Touches config struct, config validation, GitHub provider query construction, status controller, and potentially the main Tide controller. ~5-8 files, 200-400 LOC. The scope depends on which approach is chosen.
- **Level Indication**: 2-3

#### Complexity
- **Assessment**: High
- **Details**: The regex matching itself is simple, but the real complexity is architectural: Tide's PR discovery is built on GitHub Search API which doesn't support regex for `base:`. Any solution must work around this API limitation. The contributor must understand the full Tide query pipeline: config -> `constructQuery()` -> GitHub Search -> PR processing, and how `requirementDiff()` is only for informational status messages, not merge decisions.
- **Level Indication**: 3

#### Required Expertise
- **Assessment**: Deep
- **Details**: Requires understanding of Tide's architecture (search-driven PR discovery model), GitHub Search API limitations, API rate limiting implications, and the distinction between the main controller query path and the status controller. Must also make design decisions about which approach to take.
- **Level Indication**: 3

#### Clarity and Certainty
- **Assessment**: Significant uncertainty
- **Details**: The problem is clear but the solution is not. Three viable approaches exist, each with different trade-offs (API cost, performance, config complexity). A design decision is needed before implementation can begin. The issue as filed suggests a simple change but the actual implementation is substantially more involved.
- **Level Indication**: 3

#### Testing Requirements
- **Assessment**: Moderate-Complex
- **Details**: Beyond unit tests for regex compilation, need to test the chosen approach's interaction with GitHub API (mocked): branch enumeration (Approach 1), local filtering pipeline (Approach 2), or dual-field config validation (Approach 3). Integration tests for the full query flow with regex patterns.
- **Level Indication**: 2-3

#### Backwards Compatibility
- **Assessment**: Fully compatible
- **Details**: Exact strings are valid regex patterns. No existing configs would break regardless of approach chosen.
- **Level Indication**: 1

#### Architectural Alignment
- **Assessment**: Requires new patterns
- **Details**: The branchprotector precedent is somewhat misleading -- it works on a different model (enumerate all branches, then filter locally). Tide's search-driven model doesn't have an equivalent branch enumeration step. Adding one would introduce a new pattern for Tide. The other regex examples (TideBranchMergeType, Brancher) also operate differently -- they match against branches that are already known, not used for search query construction.
- **Level Indication**: 3

#### External Dependencies
- **Assessment**: Fundamental limitation
- **Details**: The GitHub Search API's `base:` operator only supports exact string matching. This is the root constraint that makes this feature non-trivial. Issue #482 may be related if GitHub improves their Search API, but that's outside Prow's control.
- **Level Indication**: 3

### Recommended Labels

- [x] `area/tide`: Core Tide functionality
- [x] `kind/feature`: New capability
- [ ] `good-first-issue`: Significantly more complex than it appears
- [ ] `help-wanted`: Requires design discussion and deep Tide expertise

### Guidance for Contributors

**For Level 3 (Large)**:
- Requires experience with Prow architecture, specifically Tide's search-driven model
- **Must consult with maintainers on approach before starting implementation**
- Should review:
  - `pkg/tide/github.go:102-115` - `GitHubProvider.Query()` (primary PR discovery)
  - `pkg/config/tide.go:554-602` - `constructQuery()` (GitHub Search query construction)
  - `pkg/tide/status.go:148-157` - `requirementDiff()` (informational only, not the core issue)
  - `cmd/branchprotector/protect.go:312-345` - Branch enumeration + regex pattern (different model)
- Key architectural considerations:
  - GitHub Search API `base:` operator does not support regex
  - `requirementDiff()` is only informational -- fixing it alone does not solve the problem
  - Any approach has trade-offs between API cost, performance, and config complexity
  - Design discussion needed before implementation

### Caveats and Considerations

- The issue as originally filed suggests a simple change (swap exact matching for regex), but the actual implementation is substantially more complex due to the GitHub Search API limitation
- The branchprotector comparison in the issue is architecturally misleading -- branchprotector enumerates branches then filters, while Tide relies on GitHub Search for PR discovery
- A design discussion or proposal should precede implementation to choose between the three approaches

## Proposed Issue Augmentation

### Title Change

- **No change needed**: Current title "Add regex support for branch matching in Tide status controller" is clear, specific, and mentions the component.

### Proposed GitHub Comment

```
This is more involved than it might appear at first glance. The branchprotector comparison is useful for motivation, but architecturally the two components work differently. Branchprotector enumerates all branches via the GitHub Branches API first, then filters locally with regex (`cmd/branchprotector/protect.go:312-345`). Tide, on the other hand, relies on the GitHub Search API for PR discovery — `TideQuery.constructQuery()` (`pkg/config/tide.go:578-583`) translates `includedBranches`/`excludedBranches` into `base:"branch"` / `-base:"branch"` search operators, and the GitHub Search API does not support regex for the `base:` qualifier. The branch matching in `requirementDiff()` (`pkg/tide/status.go:148-157`) only produces informational status messages — it doesn't drive merge decisions.

Because of this, a regex implementation would need to work around the GitHub Search API limitation. Possible approaches include: (1) enumerating branches via the Branches API and expanding regex patterns into explicit branch names before constructing the search query, (2) omitting branch filters from the search query when regex is used and filtering PRs locally after fetching, or (3) adding separate `includedBranchPatterns`/`excludedBranchPatterns` fields. Each approach has trade-offs around additional API calls, performance, and config complexity. A design discussion would be valuable before implementation begins.
```

### Rationale

**What's being added**:
- The critical architectural distinction: branchprotector enumerates branches then filters, while Tide relies on GitHub Search API for PR discovery. This is essential context missing from the issue.
- Explanation that `requirementDiff()` is informational only, not the functional path for merge decisions
- Concrete approaches with trade-offs, framing this as a design discussion rather than a straightforward implementation
- Specific code references for both the search query construction and the status message generation

**Why these labels**:
- `area/tide` and `kind/feature` are already applied
- No difficulty labels: Level 3 effort requires expertise and design discussion, not suitable for help-wanted or good-first-issue

**What's NOT included**:
- No `/retitle`: Current title is already specific and clear
- No `/help-wanted` or `/good-first-issue`: This is a Level 3 issue requiring design discussion and deep Tide expertise
- No `/priority`: This is an enhancement, not urgent
- Not repeating what the issue already says about branchprotector inconsistency or the desire for regex

## Next Steps

- Brief the maintainer on findings
- Wrapup: push branches and post comment

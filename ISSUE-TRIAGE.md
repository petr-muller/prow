# Triage for Issue #624

**Status**: In Progress
**Created**: 2026-05-02

## Issue Information

- **Issue Number**: #624
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/624

## Initial Validation

**Assessment**: LEGITIMATE

### Analysis

The issue requests that Tide's context policy system be extended to support GitHub Rulesets in addition to the existing branch protection rules. Currently, Tide has a `from-branch-protection` config option (`TideContextPolicyOptions.FromBranchProtection`) that, when enabled, reads required status checks from GitHub's branch protection API and adds them to the set of required contexts before merging.

GitHub Rulesets are the successor/complement to branch protection rules, and GitHub actively encourages migration to Rulesets. However, Prow's GitHub client (`pkg/github/`) has no Ruleset API support at all — there are zero references to "Ruleset" in the Go source. This means implementing this feature requires:

1. Adding GitHub Ruleset API types to `pkg/github/types.go`
2. Adding Ruleset API client methods to `pkg/github/client.go`
3. Extending Tide's context policy logic in `pkg/config/tide.go` to fetch required checks from Rulesets
4. A new config option (e.g., `from-rulesets`) analogous to `from-branch-protection`

**Issue Category**: Feature Request

**Repository Scope Check**:
- Component mentioned: Tide (context policy / required checks logic)
- Exists in this repo: Yes (`pkg/config/tide.go`, `pkg/tide/`)
- Relevant code paths: `pkg/config/tide.go:920-930` (branch protection context fetching), `pkg/github/client.go`, `pkg/github/types.go`

**Information Completeness**:
- Sufficient detail provided: Yes
- Missing information: None critical — the request is clear and references the relevant config option and code

### Recommendation

This is a valid feature request for a Prow component (Tide) maintained in this repository. GitHub Rulesets are the modern replacement for branch protection rules, and supporting them is a natural extension of the existing `from-branch-protection` functionality.

**Suggested Action**:
- Keep open and continue triage

## Code Research

### Current Implementation

**Primary Components**:
- `TideContextPolicy`: `pkg/config/tide.go:159-175` — Struct defining required/optional contexts and `FromBranchProtection` flag
- `TideContextPolicyOptions`: `pkg/config/tide.go:189-194` — Hierarchical config (global -> org -> repo -> branch)
- `GetTideContextPolicy()`: `pkg/config/tide.go:903-954` — Builds final context policy by merging config levels, fetching branch protection, and adding Prow job requirements
- `GetBranchProtection()`: `pkg/config/branch_protection.go:452-462` — Returns branch protection `Policy` with `RequiredStatusChecks.Contexts`
- `RepositoryClient` interface: `pkg/github/client.go:165-202` — GitHub API interface including `GetBranchProtection(org, repo, branch)` at line 171
- `BranchProtection` type: `pkg/github/types.go:552` — GitHub API response type with `RequiredStatusChecks` field
- Override plugin: `pkg/plugins/override/override.go:445-453` — Also fetches branch protection to validate override requests

**Architecture Overview**:
Tide's context policy system determines which GitHub status checks must pass before a PR can merge. The policy is built from three sources:
1. **Static config** — required/optional contexts defined in Prow config YAML
2. **Branch protection** — when `from-branch-protection: true`, required status checks are fetched from GitHub's branch protection API and merged into the required set
3. **Prow job requirements** — presubmit job names are automatically added as required/optional based on their configuration

The config hierarchy (global -> org -> repo -> branch) is merged via `ParseTideContextPolicyOptions()` (line 886) and `mergeTideContextPolicy()` (line 852), where more specific levels override less specific ones.

At merge time, Tide uses `contextChecker` (in `pkg/tide/tide.go`) with `MissingRequiredContexts()` to verify all required contexts are present and successful before proceeding.

**Key Code Paths**:
1. Policy building: `pkg/config/tide.go:903-954` — `GetTideContextPolicy()` orchestrates everything
2. Branch protection fetch: `pkg/config/tide.go:920-930` — Conditional fetch when `FromBranchProtection` is enabled
3. Config merging: `pkg/config/tide.go:852-898` — `mergeTideContextPolicy()` and `ParseTideContextPolicyOptions()`
4. Context checking: `pkg/tide/tide.go:961-984` — `IsOptional()` and `MissingRequiredContexts()`
5. Override plugin: `pkg/plugins/override/override.go:445-453` — Fetches branch protection for override validation

**Data Flow**:
1. Tide sync loop triggers context policy evaluation for each PR
2. `GetTideContextPolicy()` merges static config from hierarchy
3. If `FromBranchProtection` is true, calls `GetBranchProtection()` which reads from Prow's branch protection config (not the GitHub API directly)
4. Required contexts from branch protection are added to the required set
5. Prow presubmit job names are added via `BranchRequirements()`
6. Final `TideContextPolicy` is used by `contextChecker` to evaluate PR readiness

### Related Code

**Dependencies**:
- `pkg/config/branch_protection.go` — Branch protection policy definitions and hierarchical lookup
- `pkg/github/client.go` — GitHub API client (branch protection CRUD at lines 2727-2806)
- `pkg/github/types.go` — GitHub API types (`BranchProtection` at line 552, `RequiredStatusChecks` at line 633)

**Other Consumers**:
- `pkg/plugins/override/override.go:445-453` — Uses `GetBranchProtection()` to validate override commands; would also need Ruleset support
- `cmd/branchprotector/` — Branch protection management tool; may need awareness of Rulesets
- `cmd/checkconfig/` — Config validation; would need to validate Ruleset-related config

**Similar Functionality**:
- The `from-branch-protection` feature is the direct analog for what needs to be built for Rulesets

### Test Coverage

**Existing Tests**:
- `pkg/config/tide_test.go:TestParseTideContextPolicyOptions` (line 1158) — Tests config parsing across hierarchy
- `pkg/config/tide_test.go:TestConfigGetTideContextPolicy` (line 1293) — Tests policy building including branch protection integration
- `pkg/config/tide_test.go:TestTideContextPolicy_MissingRequiredContexts` (line 2046) — Tests missing context detection
- `pkg/config/branch_protection_test.go:TestConfig_GetBranchProtection` (line 495) — Tests hierarchical protection lookup
- `pkg/tide/tide_test.go:TestCheckRunNodesToContexts` (line 4393) — Tests CheckRun -> Context conversion
- `pkg/github/client_test.go:TestGetBranchProtection` (line 2316) — Tests GitHub API client with HTTP mocking
- Coverage assessment: Good for existing branch protection path

**Test Gaps**:
- No Ruleset-related tests (feature doesn't exist yet)
- New tests needed for: Ruleset API client, Ruleset config option, merged context policy with Rulesets

### Root Cause Analysis

**Primary Cause**:
This is a missing feature, not a bug. The Tide context policy system was built when GitHub only had branch protection rules. GitHub Rulesets are a newer, more flexible replacement that GitHub encourages users to adopt. Prow's GitHub client has zero Ruleset API support — no types, no interface methods, no HTTP calls.

**Contributing Factors**:
1. GitHub Rulesets API was introduced after the `from-branch-protection` feature was implemented
2. The codebase has no abstraction layer between "source of required checks" and the context policy — it directly calls `GetBranchProtection()`, so adding a new source requires explicit integration

### Proposed Solutions

#### Approach 1: Parallel `from-rulesets` Option

**Description**: Add a new `from-rulesets` boolean config option to `TideContextPolicy`, analogous to `from-branch-protection`. When enabled, fetch required status checks from GitHub's Rulesets API and merge them into the required contexts set alongside branch protection checks.

**Pros**:
- Follows established pattern (`from-branch-protection`)
- Users can enable one or both independently
- Minimal architectural change — extends existing system
- Backwards compatible — opt-in feature

**Cons**:
- Requires new GitHub Ruleset API types and client methods
- Two separate config options for conceptually similar sources
- Users must know which GitHub feature they're using

**Affected Components**:
- `pkg/github/types.go` — Add Ruleset API types (`RepositoryRuleset`, `StatusCheckRule`, etc.)
- `pkg/github/client.go` — Add `GetRepositoryRulesets()` to `RepositoryClient` interface and implement HTTP calls
- `pkg/config/tide.go` — Add `FromRulesets` field to `TideContextPolicy`, extend `GetTideContextPolicy()` to fetch and merge Ruleset required checks
- `pkg/plugins/override/override.go` — Extend to also check Rulesets when validating overrides

**Complexity**: Medium

**Backwards Compatibility**: Fully backwards compatible — new opt-in config option

#### Approach 2: Unified `from-github-protection` Option

**Description**: Replace or augment `from-branch-protection` with a more general option that automatically fetches required checks from both branch protection rules and Rulesets.

**Pros**:
- Simpler user experience — one toggle covers both
- Future-proof if GitHub adds more protection mechanisms
- Users don't need to know which GitHub feature provides checks

**Cons**:
- Potentially breaking change if replacing `from-branch-protection`
- Less granular control
- May fetch unexpected requirements from Rulesets users weren't aware of

**Affected Components**: Same as Approach 1, plus config migration logic

**Complexity**: Medium-High

**Backwards Compatibility**: Could be breaking if `from-branch-protection` behavior changes

#### Recommendation

**Preferred Approach**: Approach 1 (Parallel `from-rulesets` Option)

This approach follows the established pattern, is fully backwards compatible, and gives users explicit control. It's the natural extension of the existing system and matches how similar features have been added to Prow. The separate option also allows users to migrate incrementally from branch protection to Rulesets.

**Key Implementation Considerations**:
1. The key API endpoint is `GET /repos/{owner}/{repo}/rules/branches/{branch}` — returns pre-aggregated effective rules for a branch, including org-level rulesets, already filtered to `active` enforcement only. This dramatically simplifies the implementation (no need for condition matching, enforcement filtering, or separate org-level calls).
2. The response uses polymorphic rule objects with a `type` discriminator. Required status checks are under `type: "required_status_checks"` with a different structure than branch protection: `{context: "name", integration_id: 15368}` instead of plain strings. The `integration_id` field (optional) pins a check to a specific GitHub App but can be ignored for determining what's required.
3. Up to 150 rulesets (75 repo + 75 org) can target one branch. They aggregate (union of rules, most restrictive wins). During migration, both branch protection and Rulesets can be active simultaneously and layer together.
4. The `RepositoryClient` interface must be extended, which means all implementations (real client, fakes, mocks) need updates.
5. The override plugin should also gain Ruleset awareness for consistency.
6. The rules endpoint is paginated (max 100 per page) — pagination handling required.
7. Permissions are favorable: the rules endpoint only needs `Metadata` (read), less than branch protection's `Administration` (read).

**Testing Requirements**:
- Unit tests for new GitHub API types and client methods (polymorphic rule deserialization)
- Unit tests for Ruleset config option in `TideContextPolicy`
- Integration tests for merged context policy (branch protection + Rulesets + Prow jobs, including coexistence)
- Tests for override plugin with Rulesets
- Tests for pagination handling

**Migration/Rollout Strategy**:
New opt-in config option — no migration needed. Users add `from-rulesets: true` to their Tide context policy when ready. Both `from-branch-protection` and `from-rulesets` can be enabled simultaneously during migration (GitHub itself layers both systems).

## Effort Assessment

**Effort Level**: 3 - Large (requires expertise)

### Summary

While the feature follows an established pattern (`from-branch-protection`), deeper research into the GitHub Rulesets API reveals significant additional complexity: polymorphic API response types, layering semantics (up to 150 rulesets per branch, union with most-restrictive-wins), coexistence with branch protection during migration, pagination handling, and the need to update all `RepositoryClient` interface implementations. The API endpoint design (`rules/branches/{branch}`) simplifies aggregation, but the data model, coexistence concerns, and interface-wide changes push this beyond a typical moderate change.

### Factor Analysis

#### Scope of Changes
- **Assessment**: Moderate-Large
- **Details**: Estimated 8-12 files, ~300-500 lines. Core changes in `pkg/github/types.go` (polymorphic rule types), `pkg/github/client.go` (new interface methods + HTTP calls + pagination), `pkg/config/tide.go` (new config field + policy logic), `pkg/plugins/override/override.go`, plus all corresponding test files and all fake/mock implementations of `RepositoryClient`.
- **Level Indication**: 2-3

#### Complexity
- **Assessment**: High
- **Details**: The GitHub Rulesets API uses polymorphic rule objects with a `type` discriminator — each rule type has different `parameters`. Required status checks use `{context, integration_id}` tuples rather than plain strings. Up to 150 rulesets can target a single branch and they aggregate with most-restrictive-wins semantics. During migration, both branch protection and Rulesets can be active and layer together. The `rules/branches/{branch}` endpoint handles aggregation, but the consumer still needs to correctly deserialize polymorphic responses and handle pagination.
- **Level Indication**: 3

#### Required Expertise
- **Assessment**: Deep
- **Details**: Requires understanding of GitHub Rulesets API (significantly different model from branch protection), Prow's `RepositoryClient` interface pattern and all its implementations, Tide's context policy system, and the override plugin. Contributor must understand how Rulesets layer with branch protection to design correct coexistence behavior.
- **Level Indication**: 3

#### Clarity and Certainty
- **Assessment**: Some uncertainty
- **Details**: Core behavior is clear (fetch required checks from Rulesets), but open questions remain: How to handle `integration_id` (ignore for context requirements, but document the limitation)? How to handle coexistence when both `from-branch-protection` and `from-rulesets` are enabled (union is natural but needs design)? Should pagination be generic or specific to this endpoint?
- **Level Indication**: 2-3

#### Testing Requirements
- **Assessment**: Moderate-Complex
- **Details**: Needs unit tests for polymorphic rule deserialization, GitHub API client with HTTP mocking (pattern at `pkg/github/client_test.go:2316`), config parsing (pattern at `pkg/config/tide_test.go:1158`), context policy building with coexistence scenarios, pagination handling, and override plugin. Test data setup is more complex due to the nested polymorphic response structure.
- **Level Indication**: 2-3

#### Backwards Compatibility
- **Assessment**: Fully compatible
- **Details**: New opt-in config option. No existing behavior changes. Users add `from-rulesets: true` when ready. Default is off.
- **Level Indication**: 1-2

#### Architectural Alignment
- **Assessment**: Good fit with pattern extension
- **Details**: Mirrors the existing `from-branch-protection` pattern but requires new patterns for polymorphic API response handling that don't exist in the codebase today. The `RepositoryClient` interface extension follows existing patterns.
- **Level Indication**: 2-3

#### External Dependencies
- **Assessment**: Well-supported but complex
- **Details**: GitHub Rulesets REST API is stable and documented. The `GET /repos/{owner}/{repo}/rules/branches/{branch}` endpoint helpfully pre-aggregates effective rules. However, the API response model is significantly more complex than branch protection. Permissions are favorable (only `Metadata` read needed). Known gotcha: path-filtered workflows as required status checks can cause deadlocks (GitHub limitation, not solvable in Prow).
- **Level Indication**: 2-3

### Recommended Labels

- [x] `area/tide`: Extends Tide's context policy
- [x] `kind/feature`: New capability
- [ ] `good-first-issue`: Requires deep expertise across multiple packages
- [ ] `help-wanted`: Complexity and coexistence concerns require experienced contributor

### Guidance for Contributors

**For Level 3 (Large)**:
- Requires experience with Prow architecture, specifically Tide and the GitHub client
- Should consult with maintainers before starting
- Should review:
  - `pkg/config/tide.go:159-175` and `920-930`: The `TideContextPolicy` struct and `from-branch-protection` pattern
  - `pkg/github/client.go:165-202` and `2727-2806`: `RepositoryClient` interface and branch protection implementation
  - `pkg/github/types.go:552-682`: Branch protection types as template for Ruleset types
  - GitHub Rulesets REST API documentation, particularly the `rules/branches/{branch}` endpoint
- Key architectural considerations:
  - Polymorphic rule deserialization — the API returns mixed rule types in a single array
  - Coexistence design — both `from-branch-protection` and `from-rulesets` can be enabled simultaneously
  - Interface-wide changes — `RepositoryClient` extension affects all implementations
  - Pagination handling for the rules endpoint
- Estimated time: 3-5 days for experienced Go/Prow developer

### Caveats and Considerations

- The `rules/branches/{branch}` endpoint pre-aggregates effective rules from all sources (repo + org rulesets), filters to active enforcement, and resolves conditions. This is the right endpoint to use — do NOT try to fetch all rulesets and do condition matching manually.
- The `integration_id` field on required status checks pins a check to a specific GitHub App. For Tide's purposes (determining what contexts are required), only the `context` string matters. The `integration_id` constrains who can satisfy the check but doesn't change what's required.
- During migration periods, repos may have both branch protection and Rulesets active. GitHub itself layers both (union, most restrictive wins). When both `from-branch-protection` and `from-rulesets` are enabled, Tide should take the union of required contexts from both sources.
- Consider caching/rate limiting: Rulesets API calls add to GitHub API usage, though the `rules/branches/{branch}` endpoint requires only `Metadata` (read) permission, less than branch protection's `Administration` (read).
- The override plugin (`pkg/plugins/override/override.go`) should gain Ruleset awareness for consistency.
- Known GitHub limitation: path-filtered workflows that are also required status checks cause PRs to get stuck with permanent "Pending" status when the paths aren't touched. This is a GitHub design issue, not solvable in Prow.

## Proposed Issue Augmentation

### Title Change

- **No change needed**: Current title "`tide` does not respect required contexts from Github Rulesets" is clear, specific, mentions the affected component, and accurately describes the feature gap.

### Proposed GitHub Comment

```
Tide currently supports inferring required contexts from GitHub branch protection rules via the `from-branch-protection` config option in `tide.context_options` (implemented in `pkg/config/tide.go`). When enabled, Tide calls `GetBranchProtection()` to fetch `RequiredStatusChecks.Contexts` and adds them to the set of required contexts before allowing a merge. However, Prow's GitHub client has no Ruleset API support at all — no types, no interface methods, no HTTP calls — so there's no way to extend this to Rulesets without building that foundation first.

The most natural approach would be a parallel `from-rulesets` config option that mirrors `from-branch-protection`. GitHub provides a helpful `GET /repos/{owner}/{repo}/rules/branches/{branch}` endpoint that returns pre-aggregated effective rules for a branch (including org-level rulesets, already filtered to active enforcement). The implementation would need: (1) new Ruleset types in `pkg/github/types.go` — note the API uses polymorphic rule objects with a `type` discriminator, more complex than branch protection's flat structure, (2) a `GetBranchRules()` method on the `RepositoryClient` interface in `pkg/github/client.go` with pagination support, (3) a `FromRulesets` field on `TideContextPolicy`, and (4) a new block in `GetTideContextPolicy()` alongside the existing branch protection block at `pkg/config/tide.go:920-930`. A key design consideration is coexistence: during migration, repos may have both branch protection and Rulesets active (GitHub layers both, union with most-restrictive-wins), so both `from-branch-protection` and `from-rulesets` should be independently enableable. The override plugin (`pkg/plugins/override/override.go`) also fetches branch protection for validation and should ideally gain Ruleset awareness for consistency.
```

### Rationale

**What's being added**:
- Technical explanation of how `from-branch-protection` works and why the same approach doesn't cover Rulesets (the codebase has zero Ruleset API support)
- Concrete implementation roadmap with file paths, showing the four layers of work needed
- Key details about the Rulesets API: the golden endpoint, polymorphic response types, and coexistence with branch protection
- Note about the override plugin also needing updates — a detail the reporter likely wouldn't know

**Why these labels**:
- No difficulty label: Level 3 effort assessment — requires expertise with polymorphic API types, interface-wide changes, and coexistence design
- `area/tide` and `kind/feature` already applied by maintainer in previous comment

**What's NOT included**:
- No `/retitle`: Title is already clear and specific
- No `/help-wanted` or `/good-first-issue`: Level 3 complexity — experienced contributors will self-select
- No priority label: Feature request, not a regression or security issue
- No `/area` or `/kind`: Already applied by maintainer

## Next Steps

(Action items will be added here)

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
1. GitHub Rulesets API uses different endpoints and response shapes than branch protection — need to extract required status checks from ruleset rules of type `required_status_checks`
2. The `RepositoryClient` interface must be extended, which means all implementations (real client, fakes, mocks) need updates
3. The override plugin should also gain Ruleset awareness for consistency
4. Consider how Rulesets interact with branch protection when both are enabled (union of required checks is the logical default)

**Testing Requirements**:
- Unit tests for new GitHub API types and client methods
- Unit tests for Ruleset config option in `TideContextPolicy`
- Integration tests for merged context policy (branch protection + Rulesets + Prow jobs)
- Tests for override plugin with Rulesets

**Migration/Rollout Strategy**:
New opt-in config option — no migration needed. Users add `from-rulesets: true` to their Tide context policy when ready.

## Effort Assessment

**Effort Level**: 2 - Moderate (help-needed)

### Summary

This is a well-defined feature request that follows an established pattern (`from-branch-protection`), but requires work across multiple layers (GitHub API types, client methods, config, Tide logic) and knowledge of both the GitHub Rulesets API and Prow's architecture.

### Factor Analysis

#### Scope of Changes
- **Assessment**: Moderate
- **Details**: Estimated 5-8 files, ~200-400 lines. Core changes in `pkg/github/types.go` (new types), `pkg/github/client.go` (new interface methods + HTTP calls), `pkg/config/tide.go` (new config field + policy logic), plus corresponding test files. Optional: `pkg/plugins/override/override.go` for consistency.
- **Level Indication**: 2-3

#### Complexity
- **Assessment**: Moderate
- **Details**: No concurrency issues or race conditions. The solution follows an existing pattern closely. Main complexity is understanding the GitHub Rulesets API response shape and correctly extracting required status checks from it. The Rulesets API is more complex than branch protection (multiple rulesets can apply, each with multiple rules).
- **Level Indication**: 2-3

#### Required Expertise
- **Assessment**: Moderate
- **Details**: Needs familiarity with GitHub Rulesets API, Prow's config hierarchy, and the Tide context policy system. Can be learned from existing `from-branch-protection` implementation, but contributor needs to be comfortable reading API docs and working across multiple packages.
- **Level Indication**: 2-3

#### Clarity and Certainty
- **Assessment**: Well-defined
- **Details**: The desired behavior is clear: fetch required status checks from Rulesets and add them to Tide's required contexts. The approach (parallel `from-rulesets` option) is straightforward. One open question: how to handle multiple rulesets with overlapping rules (union of required checks is the natural default).
- **Level Indication**: 1-2

#### Testing Requirements
- **Assessment**: Moderate
- **Details**: Needs unit tests for GitHub API types/client (HTTP mock pattern exists at `pkg/github/client_test.go:2316`), config parsing tests (pattern at `pkg/config/tide_test.go:1158`), and context policy building tests (pattern at `pkg/config/tide_test.go:1293`). All test patterns are well-established.
- **Level Indication**: 2-3

#### Backwards Compatibility
- **Assessment**: Fully compatible
- **Details**: New opt-in config option. No existing behavior changes. Users add `from-rulesets: true` when ready. Default is off.
- **Level Indication**: 1-2

#### Architectural Alignment
- **Assessment**: Perfect fit
- **Details**: Directly mirrors the existing `from-branch-protection` pattern. Extends `TideContextPolicy` struct with one new field, adds a new conditional block in `GetTideContextPolicy()` parallel to the existing branch protection block. No new patterns needed.
- **Level Indication**: 1-2

#### External Dependencies
- **Assessment**: Well-supported
- **Details**: GitHub Rulesets REST API is stable and documented. Endpoints: `GET /repos/{owner}/{repo}/rulesets` and `GET /repos/{owner}/{repo}/rules/branches/{branch}`. The API provides required status check information in the `required_status_checks` rule type.
- **Level Indication**: 1-3

### Recommended Labels

- [x] `help-needed`: Well-defined, follows established patterns, suitable for skilled contributor
- [x] `area/tide`: Extends Tide's context policy
- [x] `kind/feature`: New capability
- [ ] `good-first-issue`: Too many files/packages involved for a first contribution

### Guidance for Contributors

**For Level 2 (Moderate)**:
- Suitable for contributors familiar with Go and GitHub API
- Should review:
  - `pkg/config/tide.go:159-175` and `920-930`: The `TideContextPolicy` struct and `from-branch-protection` pattern
  - `pkg/github/client.go:165-202` and `2727-2806`: `RepositoryClient` interface and branch protection implementation
  - `pkg/github/types.go:552-682`: Branch protection types as template for Ruleset types
  - GitHub Rulesets REST API documentation
- Recommended approach:
  1. Add Ruleset types to `pkg/github/types.go`
  2. Add `GetRepositoryRulesets()` to `RepositoryClient` interface and implement
  3. Add `FromRulesets` field to `TideContextPolicy`
  4. Extend `GetTideContextPolicy()` with Ruleset fetch block
  5. Add tests at each layer
- Estimated time: 1-2 days for experienced Go developer

### Caveats and Considerations

- The GitHub Rulesets API returns potentially many rulesets per repo. The implementation should filter for rulesets that apply to the target branch and extract `required_status_checks` rules.
- Consider caching/rate limiting: Rulesets API calls add to GitHub API usage. If `from-branch-protection` is also enabled, both are fetched per context policy evaluation.
- The override plugin (`pkg/plugins/override/override.go`) should ideally be updated in the same change for consistency, though it could be a follow-up.

## Next Steps

(Action items will be added here)

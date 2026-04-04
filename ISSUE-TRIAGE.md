# Triage for Issue #670

**Status**: In Progress
**Created**: 2026-04-03

## Issue Information

- **Issue Number**: #670
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/670

## Initial Validation

**Assessment**: LEGITIMATE

### Analysis

Issue #670 requests making the org invite functionality in the trigger plugin configurable. Specifically, after PR #627 introduced a feature that posts a prominent "join the org" message when a contributor has 3+ merged PRs, the issue author asks for:

1. Making the merged PR threshold configurable (currently hardcoded to 3)
2. Making the invite message itself configurable

**Issue Category**: Feature Request

**Repository Scope Check**:
- Component mentioned: trigger plugin (org invite messaging)
- Exists in this repo: Yes
- Relevant code paths:
  - `pkg/plugins/trigger/pull-request.go` (lines 43, 265, 324, 327, 340, 345)
  - Hardcoded constant `mergedPRCountForProminentJoinOrgMessage = 3`
  - Hardcoded message strings for both prominent and regular join-org guidance

**Information Completeness**:
- Sufficient detail provided: Yes
- The author clearly describes the use case: different orgs have different membership policies (not all give lgtm rights, different thresholds, different processes)
- Author offers to implement it and asks for guidance on where configuration should live

### Recommendation

This is a well-scoped, legitimate feature request. The functionality was recently added (PR #627, merged 2026-02-24) with hardcoded values that the author reasonably argues should be configurable. The author is an experienced contributor (Lennart Jern) who is willing to implement the change.

**Suggested Action**:
- Keep open and continue triage

## Code Research

### Current Implementation

**Primary Components**:
- Trigger plugin org invite: `pkg/plugins/trigger/pull-request.go` — generates "join the org" guidance in PR welcome comments
- Trigger config: `pkg/plugins/config.go` (lines 486-514) — `Trigger` struct with existing `JoinOrgURL` field

**Architecture Overview**:
When a non-org-member opens a PR, the trigger plugin posts a welcome message that includes guidance about joining the org. PR #627 (merged 2026-02-24, authored by Miltiadis Alexis) enhanced this by adding a prominent tip box when the author has 3+ merged PRs in the org.

**Key Code Paths**:
1. `welcomeMsg()`: `pull-request.go:282` — calls `orgInvitationGuidance()` and embeds result in the welcome comment
2. `orgInvitationGuidance()`: `pull-request.go:322-328` — returns either the prominent or regular message
3. `shouldHighlightJoinOrgMessage()`: `pull-request.go:330-346` — queries GitHub for merged PRs by author, returns true if count >= 3
4. Hardcoded constant: `pull-request.go:43` — `mergedPRCountForProminentJoinOrgMessage = 3`

**Hardcoded Values That Need Configuration**:
- Threshold: `const mergedPRCountForProminentJoinOrgMessage = 3` (line 43)
- Prominent message (line 324): `">[!TIP]\n>**We noticed you've done this a few times! Consider [joining the org](%s)..."` 
- Regular message (line 327): `"Regular contributors should [join the org](%s) to skip this step."`

**Data Flow**:
1. PR opened by non-org-member → trigger plugin handles event
2. `welcomeMsg()` called → constructs welcome comment
3. `orgInvitationGuidance()` called → checks if author has 3+ merged PRs via GitHub search API
4. Returns prominent tip box (>=3 PRs) or regular one-liner (<3 PRs)
5. Message embedded in the larger welcome comment posted to the PR

### Related Code

**Dependencies**:
- GitHub Search API: used to count merged PRs (`type:pr is:merged org:<org> author:<author>`)
- `JoinOrgURL` config field already exists in `Trigger` struct (line 490) — provides precedent for org-invite configuration

**Other Users of JoinOrgURL**:
- `pkg/plugins/verify-owners/verify-owners.go:363-381` — uses same `JoinOrgURL` pattern

**Similar Configurability Patterns in Codebase**:
- `Welcome.MessageTemplate` (config.go:774): configurable message template string — direct pattern match
- `Blunderbuss.ReviewerCount *int` (config.go:165): configurable count with pointer-to-int pattern
- `Golint.MinimumConfidence *float64` (config.go:122): optional numeric threshold pattern

### Test Coverage

**Existing Tests** (`pkg/plugins/trigger/pull-request_test.go`):
- `TestShouldHighlightJoinOrgMessage` (line 663): tests 2 PRs → false, 3 PRs → true
- `TestShouldHighlightJoinOrgMessageUsesFilteredQuery` (line 717): verifies correct search query
- `TestShouldHighlightJoinOrgMessageIgnoresSearchErrors` (line 747): graceful error handling
- `TestShouldHighlightJoinOrgMessageSkipsBotAuthors` (line 763): bot filtering

**Test Gaps**:
- No tests for configurable thresholds
- No tests for configurable messages
- No tests for default value behavior when config is omitted

### Root Cause Analysis

**Primary Cause**: Not a bug — this is a feature gap. The org invite feature was implemented with hardcoded values that are reasonable defaults for the kubernetes-sigs org but not suitable for all Prow deployments.

**Contributing Factors**:
1. Different orgs have different membership policies (not all grant `/lgtm` rights)
2. Some orgs may want higher/lower thresholds based on their contributor funnel
3. Some orgs have entirely different invitation processes (no sponsors, etc.)

### Proposed Solutions

#### Approach 1: Add Config Fields to Trigger Struct

**Description**: Add new fields to the existing `Trigger` config struct for the threshold count and message templates. Use the established patterns already in the codebase.

**Changes**:
- `pkg/plugins/config.go`: Add fields to `Trigger` struct:
  - `OrgInviteMinMergedPRs *int` — threshold (default 3)
  - `OrgInviteProminentMessage string` — prominent tip message (with `%s` placeholder for URL)
  - `OrgInviteMessage string` — regular message (with `%s` placeholder for URL)
- `pkg/plugins/config.go`: Update `SetDefaults()` to apply defaults
- `pkg/plugins/trigger/pull-request.go`: Read from config instead of hardcoded values
- Tests: Update to cover configurable values

**Pros**:
- Follows existing patterns (`JoinOrgURL`, `Welcome.MessageTemplate`, `Blunderbuss.ReviewerCount`)
- Minimal code change, well-scoped
- Backwards compatible — omitted config means current behavior preserved

**Cons**:
- Message templates with `%s` placeholder are fragile (user could forget the placeholder)

**Complexity**: Low
**Backwards Compatibility**: Full — all new fields optional with defaults matching current behavior

#### Approach 2: Disable-Only Configuration

**Description**: Instead of making everything configurable, just add an option to disable the prominent message entirely and let orgs use their own external tooling for invitations.

**Pros**:
- Simpler implementation
- Less surface area for misconfiguration

**Cons**:
- Doesn't address the threshold configurability request
- Doesn't address message customization request
- Less useful to the issue author

**Complexity**: Very Low
**Backwards Compatibility**: Full

#### Recommendation

**Preferred Approach**: Approach 1 (Add Config Fields to Trigger Struct)

This aligns with the issue author's request, follows established codebase patterns, and is straightforward to implement. The `Welcome.MessageTemplate` pattern is a direct precedent for configurable messages in plugin configuration.

**Key Implementation Considerations**:
1. Config fields should go in `Trigger` struct alongside existing `JoinOrgURL`
2. Use `*int` pointer pattern for threshold to distinguish "not set" from "set to 0"
3. Consider whether to allow setting threshold to 0 to effectively disable the feature
4. Message templates should document the `%s` placeholder for the join URL
5. Tests should verify default behavior when config is omitted

**Testing Requirements**:
- Test that omitted config preserves current behavior (threshold=3, default messages)
- Test custom threshold values
- Test custom message templates
- Test threshold of 0 (disable behavior)

## Effort Assessment

**Effort Level**: 1 - Easy (good-first-issue)

### Summary

This is a well-scoped feature request to add 2-3 optional config fields to an existing struct and wire them into nearby code. Clear patterns to follow, small scope, fully backwards compatible.

### Factor Analysis

#### Scope of Changes
- **Assessment**: Small
- **Details**: 3-4 files affected: `pkg/plugins/config.go` (add fields + defaults), `pkg/plugins/trigger/pull-request.go` (use config instead of hardcoded values), plus test files. Estimated ~50-80 lines of new/modified code.
- **Level Indication**: 1-2

#### Complexity
- **Assessment**: Simple
- **Details**: Adding config fields to an existing struct and replacing hardcoded values with config lookups. No concurrency, no complex logic, no edge cases beyond basic validation.
- **Level Indication**: 1-2

#### Required Expertise
- **Assessment**: Minimal
- **Details**: Basic Go, understanding of Prow plugin config pattern. All needed patterns already exist in the same files (`JoinOrgURL`, `Welcome.MessageTemplate`, `Blunderbuss.ReviewerCount`).
- **Level Indication**: 1-2

#### Clarity and Certainty
- **Assessment**: Well-defined
- **Details**: The issue clearly states what should be configurable (threshold count + message). The config struct and code locations are identified. Only minor design question: exact field naming and whether to support Go template syntax vs `%s` for messages.
- **Level Indication**: 1-2

#### Testing Requirements
- **Assessment**: Simple
- **Details**: Existing tests for the org invite logic can be extended. Follow the same test patterns already in `pull-request_test.go`. Add test cases for custom config values and default behavior.
- **Level Indication**: 1-2

#### Backwards Compatibility
- **Assessment**: Fully compatible
- **Details**: All new config fields are optional with defaults matching current hardcoded behavior. Existing configs work unchanged.
- **Level Indication**: 1-2

#### Architectural Alignment
- **Assessment**: Perfect fit
- **Details**: Directly follows established patterns. `JoinOrgURL` is already a configurable field in the same `Trigger` struct. `Welcome.MessageTemplate` is a precedent for configurable messages in Prow plugins.
- **Level Indication**: 1-2

#### External Dependencies
- **Assessment**: None
- **Details**: No external API changes needed. The GitHub Search API usage is unchanged; only the threshold comparison and message strings become config-driven.
- **Level Indication**: 1-3

### Recommended Labels

- [x] `good-first-issue`: Well-defined, small scope, clear patterns to follow, author volunteers
- [x] `kind/feature`: New configurability for existing functionality
- [x] `area/plugins`: Trigger plugin configuration change

### Guidance for Contributors

- Good starting point for new or returning Prow contributors
- Prerequisite knowledge: Basic Go, YAML configuration
- Key files to review:
  - `pkg/plugins/config.go`: `Trigger` struct, `SetDefaults()`, similar patterns like `Welcome.MessageTemplate`
  - `pkg/plugins/trigger/pull-request.go`: `orgInvitationGuidance()`, `shouldHighlightJoinOrgMessage()`
  - `pkg/plugins/trigger/pull-request_test.go`: existing tests for org invite logic
- The issue author (lentzi90) has offered to implement this and asks for guidance on where configuration fits — the `Trigger` struct is the answer

### Caveats and Considerations

- The message configurability could use simple `%s` placeholder (like current code) or Go templates (like `Welcome.MessageTemplate`). Either approach works; `%s` is simpler and consistent with the existing code in this function.
- Consider whether threshold of 0 should disable the prominent message entirely or be treated as "always show prominent message". A value of 0 meaning "disable" is more intuitive.

## Proposed Issue Augmentation

### Title Change

- **Current**: "Make org invite functionality configurable"
- **Proposed**: "trigger: make org invite message and PR threshold configurable"
- **Rationale**: Adds the component name (trigger plugin) and specifies both configurable aspects (message + threshold) for clarity

### Proposed GitHub Comment

```
/retitle trigger: make org invite message and PR threshold configurable

The configuration for this lives in the `Trigger` struct in `pkg/plugins/config.go`, which already has a `JoinOrgURL` field for the same feature. The hardcoded values that need to become configurable are in `pkg/plugins/trigger/pull-request.go`: the threshold constant `mergedPRCountForProminentJoinOrgMessage = 3` (line 43) and the two message strings in `orgInvitationGuidance()` (lines 324 and 327). There are established patterns to follow: `Blunderbuss.ReviewerCount` uses a `*int` for optional count config, and `Welcome.MessageTemplate` uses a plain string for configurable messages.

For the message templates, the current code uses `fmt.Sprintf` with a `%s` placeholder for the join URL, so using the same approach (a string field with `%s` placeholder) would be simplest and consistent. Setting the threshold to 0 could serve as a way to disable the prominent message entirely, which would address the "different process altogether" scenario mentioned in the issue.

/area plugins
/kind feature
/good-first-issue
```

### Rationale

**What's being added**:
- Specific file paths and line numbers for the code that needs to change
- Pointer to existing configuration patterns to follow (answering the author's question about "where the configuration would fit")
- Design suggestion for threshold=0 behavior

**Why these labels**:
- `/area plugins`: The trigger plugin is the affected component
- `/kind feature`: This is a new configurability feature, not a bug
- `/good-first-issue`: Level 1 effort — well-scoped, clear patterns, small change

**What's NOT included**:
- Detailed implementation plan — the author is experienced and asked for guidance on where config fits, not a full spec
- Priority label — this is an enhancement, not urgent
- The full list of test requirements — would be over-prescriptive for a good-first-issue

## Briefing Completed

Briefed maintainer on: 2026-04-04

Key questions asked:
- None

Maintainer decision:
- No questions, proceed with wrapup

## Next Steps

- Post augmentation comment to the issue
- Wait for lentzi90 to submit a PR

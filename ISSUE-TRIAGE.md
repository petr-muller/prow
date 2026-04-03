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

## Findings

(Further findings from triage subcommands will be added here)

## Next Steps

(Action items will be added here)

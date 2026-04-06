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

#### Approach 1: Flat Config Fields on Trigger Struct

**Description**: Add flat fields directly to the existing `Trigger` config struct.

**Pros**: Minimal code change, follows `JoinOrgURL` precedent
**Cons**: Doesn't support opt-out cleanly, mixes org-invite concerns with other trigger config
**Complexity**: Low

#### Approach 2: Nested OrgInvite Config Struct (Recommended — per prior maintainer feedback)

**Description**: Introduce a dedicated `OrgInvite` struct nested inside `Trigger`, with explicit opt-out support and layered config resolution (global → org → repo).

**Config shape**:
```yaml
triggers:
  - repos: ["my-org"]
    org_invite:
      disabled: false
      merged_pr_threshold: 5
      prominent_message: "Consider [joining the org](%s) for contributor access."
```

**Struct fields**:
- `Disabled bool` — opt out entirely
- `MergedPRsThreshold *int` — threshold for prominent message (default 3, pointer to distinguish unset)
- `ProminentMessage string` — custom prominent message (`%s` placeholder for join URL)
- `Message string` — custom regular message (`%s` placeholder for join URL)

**Changes**:
- `pkg/plugins/config.go`: Add `OrgInvite` struct, nest in `Trigger`, update `SetDefaults()`, extend `TriggerFor()` with field-level merging
- `pkg/plugins/trigger/pull-request.go`: Read from `OrgInvite` config instead of hardcoded values
- Tests: Cover default behavior, custom values, opt-out, and layered override scenarios

**Pros**:
- Clean separation of org-invite config from other trigger concerns
- Explicit opt-out with `disabled: true`
- Layered config allows org-wide defaults with per-repo overrides
- Follows established patterns (`Blunderbuss.ReviewerCount`, `CherryPickUnapproved.Comment`)

**Cons**:
- More complex than flat fields — needs field-level merge logic in `TriggerFor()`
- Open question: should existing `JoinOrgURL` move into the new struct for consistency?

**Complexity**: Medium
**Backwards Compatibility**: Full — defaults preserve current behavior

#### Recommendation

**Preferred Approach**: Approach 2 (Nested OrgInvite Struct)

This was the direction established during prior maintainer review of this issue. The layered config resolution is the main complexity driver but aligns with how Prow config is meant to work.

**Key Implementation Considerations**:
1. Use `*int` for threshold (like `Blunderbuss.ReviewerCount`) — pointer distinguishes "not set" from zero
2. Defaults in `SetDefaults()` preserve current behavior when no config provided
3. `TriggerFor()` needs extension for field-level merging of the `OrgInvite` struct
4. Consider whether `JoinOrgURL` should stay where it is (backwards compat) or move into `OrgInvite` (consistency)
5. `disabled: true` at repo level should suppress all org invite messaging

**Testing Requirements**:
- Default behavior when no config provided (threshold=3, default messages)
- Custom threshold and message values
- Opt-out with `disabled: true`
- Layered override scenarios (global set + repo override, org set + repo opt-out)

## Effort Assessment

**Effort Level**: 2 - Moderate (help-wanted)

*Revised from initial Level 1 assessment after incorporating prior maintainer feedback requiring nested config struct, opt-out support, and global/org/repo layered config with field-level merging.*

### Summary

Introducing a dedicated `OrgInvite` config struct within trigger configuration, with support for opt-out and layered resolution at global/org/repo levels. The struct and field patterns are established, but the layered config merging in `TriggerFor()` adds moderate complexity requiring familiarity with Prow's config resolution.

### Factor Analysis

#### Scope of Changes
- **Assessment**: Moderate
- **Details**: 4-6 files modified (`config.go`, `pull-request.go`, `pull-request_test.go`, `config_test.go`, `plugin-config-documented.yaml`, possibly `trigger.go`), estimated ~150-250 lines including config merge logic and tests.
- **Level Indication**: 2-3

#### Complexity
- **Assessment**: Moderate
- **Details**: The struct and fields are straightforward, but implementing layered config resolution (global → org → repo) with field-level merging requires careful design. Need to handle: which fields are "set" vs "default", how to merge partial overrides, and how opt-out at one level interacts with settings at another.
- **Level Indication**: 2-3

#### Required Expertise
- **Assessment**: Moderate
- **Details**: Needs understanding of Prow's `TriggerFor()` config resolution and how to extend it with field-level merging. Contributor should review how existing trigger config matching works.
- **Level Indication**: 2-3

#### Clarity and Certainty
- **Assessment**: Well-defined
- **Details**: Requirements are clear after prior maintainer input: nested struct, opt-out, layered config. Design direction is settled.
- **Level Indication**: 1-2

#### Testing Requirements
- **Assessment**: Moderate
- **Details**: Need tests for: default behavior, custom threshold, custom messages, opt-out, and layered override scenarios (global set + repo override, org set + repo opt-out, etc.)
- **Level Indication**: 2-3

#### Backwards Compatibility
- **Assessment**: Fully compatible
- **Details**: All new config is optional with defaults preserving current behavior. Existing deployments are unaffected.
- **Level Indication**: 1-2

#### Architectural Alignment
- **Assessment**: Good fit
- **Details**: Nested config structs and layered resolution align with how Prow config is meant to work, though the trigger plugin may need new merge logic that doesn't exist yet for this specific config.
- **Level Indication**: 2-3

#### External Dependencies
- **Assessment**: None
- **Details**: No external API changes needed.
- **Level Indication**: 1-3

### Recommended Labels

- [x] `help-wanted`: Moderate complexity, well-defined but requires Prow config familiarity
- [x] `kind/feature`: Adding configurability to existing functionality
- [x] `area/plugins`: Change is in the trigger plugin
- [ ] `good-first-issue`: Layered config resolution elevates this beyond good-first-issue

### Guidance for Contributors

- Review `pkg/plugins/config.go` for the `Trigger` struct, `SetDefaults()`, and `TriggerFor()` resolution
- Design an `OrgInvite` struct with `Disabled bool`, `MergedPRsThreshold *int`, and message `string` fields
- Implement field-level merging so repo-level config overrides org-level, which overrides global
- Follow `Blunderbuss.ReviewerCount *int` pattern for optional numeric fields
- The issue author (@lentzi90) has volunteered to implement this

### Caveats and Considerations

- The layered config resolution is the main complexity driver — the struct itself is simple
- Need to decide whether `JoinOrgURL` (already in `Trigger`) should move into the new struct for consistency, or stay where it is for backwards compatibility
- `Disabled: true` at repo level should be absolute (no invite message at all)

## Proposed Issue Augmentation

### Title Change

- **No change needed**: Current title "Make org invite functionality configurable" is clear and accurate.

### Proposed GitHub Comment

```
The values you'd like to make configurable live in `pkg/plugins/trigger/pull-request.go`: the threshold is the constant `mergedPRCountForProminentJoinOrgMessage = 3` (line 43), and the two message variants are in the `orgInvitationGuidance()` function (lines 322-328) — one prominent tip box for authors above the threshold and a simpler one-liner for everyone else.

We'd like the implementation to use a dedicated config struct (e.g., `OrgInvite`) nested inside the `plugins.Trigger` struct in `pkg/plugins/config.go` (line 489), rather than adding flat fields. The struct should support:

- **Threshold**: a `*int` field for the merged PR count (like `Blunderbuss.ReviewerCount` at line 168 — pointer distinguishes "not set" from zero). Default: 3.
- **Message(s)**: `string` field(s) for custom message text (like `CherryPickUnapproved.Comment` at line 835). Use `%s` as a placeholder for the join-org URL. The prominent message currently mentions `/lgtm` rights and sponsor recommendations that don't apply to all orgs, so making at least that one configurable is important.
- **Opt-out**: a way to disable the org invite functionality entirely (e.g., `disabled: true`).
- **Layered config**: the struct should be resolvable at global, org, and repo levels, where more specific levels override less specific ones. Review how `TriggerFor()` resolves config today and extend it with field-level merging for the new struct.

Set defaults in the `SetDefaults()` method (around line 1026) to preserve current behavior when no config is provided. Example YAML shape:

```yaml
triggers:
  - repos: ["my-org"]
    org_invite:
      disabled: false
      merged_pr_threshold: 5
      prominent_message: "Consider [joining the org](%s) for contributor access."
```

/area plugins
/kind feature
/help-wanted
```

### Rationale

**What's being added**:
- Where the hardcoded values live (exact file paths and line numbers)
- Design direction: nested struct, opt-out support, layered config resolution
- Which existing patterns to follow (Blunderbuss for threshold, CherryPickUnapproved for message)
- Example YAML shape for the configuration
- How defaults should work to preserve backwards compatibility

**Why these labels**:
- `/area plugins`: The trigger plugin lives in `pkg/plugins/trigger/`
- `/kind feature`: This is an enhancement request for configurability
- `/help-wanted`: Level 2 effort — layered config resolution adds moderate complexity beyond a simple good-first-issue

**What's NOT included**:
- No `/retitle` — existing title is already clear
- No priority label — this is an enhancement, not blocking anyone
- No implementation code — guidance only, respecting that the author wants to do the work

## Prior Triage Synthesis

This triage incorporates findings from a prior triage attempt (remote `issue-triage-670` branch, created 2026-04-02). Key changes from synthesis:

- **Effort revised**: Level 1 → Level 2 based on prior maintainer feedback requiring nested struct and layered config
- **Solution revised**: Flat fields → nested `OrgInvite` struct with opt-out and field-level merging in `TriggerFor()`
- **Title**: Kept original (prior triage also concluded no change needed)
- **Labels revised**: `good-first-issue` → `help-wanted` to match Level 2

## Briefing Completed

Briefed maintainer on: 2026-04-06 (re-briefing after synthesis with prior triage)

Key questions asked:
- (pending — briefing slides follow)

Maintainer decision:
- (pending)

## Next Steps

- Complete re-briefing
- Post augmentation comment to the issue
- Wait for lentzi90 to submit a PR

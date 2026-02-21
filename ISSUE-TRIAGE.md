# Triage for Issue #180

**Status**: In Progress
**Created**: 2026-02-21

## Issue Information

- **Issue Number**: #180
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/180

## Initial Validation

**Assessment**: LEGITIMATE

### Analysis

The issue requests that Prow's trigger plugin include a bold, explicit invitation to join the Kubernetes org in its "ok-to-test" message after a non-member contributor has made multiple merged PRs (e.g., on their fourth PR). The rationale is that many regular contributors (~1/3) never join the org because the process is intimidating and eligibility is unclear.

The proposal includes:
- A sample message with a clear call-to-action
- A GraphQL query to check merged PR count (flat cost of 1)
- A fallback approach (always comment without checking count)
- Clear motivation grounded in contributor experience data

**Issue Category**: Feature Request

**Repository Scope Check**:
- Component mentioned: trigger plugin
- Exists in this repo: Yes (`pkg/plugins/trigger/`)
- Relevant code paths: `pkg/plugins/trigger/generic-comment.go`, `pkg/plugins/trigger/trigger.go`

**Information Completeness**:
- Sufficient detail provided: Yes
- Missing information: None significant — the proposal is well-specified with sample messages, a query approach, and clear motivation
- Original issue: Migrated from kubernetes/test-infra#13371

### Recommendation

This is a well-written, legitimate feature request for the trigger plugin, which lives in this repository. The author (Josh Berkus) is a known Kubernetes contributor and SIG Contributor Experience member. The issue has clear motivation, a concrete proposal, and even suggests an implementation approach.

**Suggested Action**:
- Keep open and continue triage

## Code Research

### Current Implementation

**Primary Components**:
- Trigger Plugin: `pkg/plugins/trigger/trigger.go` — plugin registration, trust checking
- PR Handler: `pkg/plugins/trigger/pull-request.go` — handles PR lifecycle events, posts welcome messages
- Comment Handler: `pkg/plugins/trigger/generic-comment.go` — handles `/ok-to-test`, `/test`, `/retest` commands
- Plugin Config: `pkg/plugins/config.go:489-514` — trigger configuration struct

**Architecture Overview**:
When a non-org-member opens a PR, the trigger plugin checks trust via `TrustedUser()` (cascading check: org member → collaborator → trusted app → secondary org). If untrusted, it calls `welcomeMsg()` which posts a comment asking an org member to reply `/ok-to-test` and adds the `needs-ok-to-test` label. The current message already mentions "Regular contributors should join the org to skip this step" but it's generic and not personalized.

**Key Code Paths**:
1. PR opened by non-member: `pull-request.go:74-96` — detects untrusted user, calls `welcomeMsg()`
2. Welcome message template: `pull-request.go:250-316` — constructs the ok-to-test message
3. Trust validation: `trigger.go:228-293` — `TrustedUser()` cascading membership check
4. `/ok-to-test` handling: `generic-comment.go:115-125` — adds OkToTest label, removes NeedsOkToTest

**Data Flow**:
1. GitHub webhook → PR opened event → trigger plugin
2. `handlePR()` checks `TrustedUser()` → returns untrusted
3. `welcomeMsg()` generates comment with ok-to-test instructions
4. Comment posted, `needs-ok-to-test` label added
5. Org member comments `/ok-to-test` → label swap → jobs triggered

### Related Code

**Dependencies**:
- `pkg/github/client.go:1288-1312` — `IsMember()` REST API call
- `pkg/github/client.go:4031-4072` — `IsCollaborator()` REST API call
- `pkg/github/client.go:3480-3528` — `FindIssuesWithOrg()` search API (usable for PR history)

**GitHub Client Capabilities**:
- GraphQL support is fully implemented: `pkg/plugins/plugins.go:182-185` — `PluginGitHubClient` interface includes `Query()` method
- Search API: `FindIssuesWithOrg()` can query `type:pr author:{username} org:{org} is:merged` to count merged PRs
- Both GraphQL and REST approaches are viable for counting contributor history

**Similar Functionality**:
- Draft PR message variation: `pull-request.go:318-323` — different message for draft PRs
- Conditional messaging based on `IgnoreOkToTest` config: `pull-request.go:268-306`
- `untrustedReason` enum: `trigger.go:47-72` — already categorizes why a user isn't trusted

**Configuration Fields** (config.go:489-514):
- `TrustedOrg string` — secondary org for trust checks
- `JoinOrgURL string` — custom URL for org join instructions
- `OnlyOrgMembers bool` — restrict to org members only
- `IgnoreOkToTest bool` — disable ok-to-test entirely
- `TrustedApps []string` — trusted bot usernames

### Test Coverage

**Existing Tests**:
- `pull-request_test.go` (633 lines): Tests PR event handling, trust/label decisions, welcome comment posting
- `generic-comment_test.go` (1638 lines): Tests `/ok-to-test`, `/test`, `/retest` comment handling
- `trigger_test.go` (671 lines): Tests `TrustedUser()` cascading checks, help provider
- `push_test.go` (289 lines): Tests push events and postsubmits
- Coverage assessment: Good for existing ok-to-test flow

**Test Patterns**:
- Table-driven tests with `fakegithub.FakeClient`
- Track operations via `IssueCommentsAdded`, `IssueLabelsAdded`, `IssueLabelsRemoved`
- ProwJob creation validated via `fake.NewSimpleClientset()`

**Test Gaps for This Feature**:
- No tests for contributor history lookup (feature doesn't exist yet)
- No tests for conditional message content based on user history
- `FindIssuesWithOrg()` / GraphQL search not tested in trigger context

### Root Cause Analysis

**Primary Cause**: N/A — this is a feature request, not a bug.

**Current State**: The welcome message already contains a generic "Regular contributors should join the org to skip this step" line, but it's not personalized or prominent. The issue proposes making it bold, explicit, and conditional on the contributor's actual PR history.

**Gap**: The trigger plugin currently makes no attempt to check the contributor's prior history. Every non-member gets the same generic message regardless of whether it's their first PR or their twentieth.

### Proposed Solutions

#### Approach 1: REST Search API (FindIssuesWithOrg)

**Description**: Use the existing `FindIssuesWithOrg()` method to query `type:pr author:{username} org:{org} is:merged` and count results. If count >= threshold (e.g., 3), add a bold invitation paragraph to the welcome message.

**Pros**:
- Uses existing, well-tested GitHub client infrastructure
- No new API integration needed
- Simple query, low API cost
- Aligns with the issue's suggestion of counting merged PRs

**Cons**:
- REST search API has rate limits (30 requests/minute for authenticated)
- Returns full issue objects when only count is needed
- May paginate for prolific contributors (though we only need the count)

**Affected Components**:
- `pull-request.go`: Modify `welcomeMsg()` to accept PR count and conditionally add invitation
- `trigger.go` or `pull-request.go`: Add function to query contributor history
- `config.go`: Add config fields for threshold and feature toggle

**Complexity**: Low

**Backwards Compatibility**: Fully compatible — new config fields default to disabled

#### Approach 2: GraphQL Query (as proposed in issue)

**Description**: Use the GraphQL API with the exact query from the issue to get `issueCount` for merged PRs. This returns only the count, not full objects.

**Pros**:
- Minimal API cost (flat cost of 1 GraphQL point)
- Returns only the count — no pagination or excess data
- Exactly what the issue author proposed
- More efficient than REST search

**Cons**:
- Requires constructing a GraphQL query struct for Go's shurcooL/graphql client
- GraphQL rate limiting is separate from REST (5000 points/hour)
- Slightly more complex code than REST search
- Need to define Go types for the query response

**Affected Components**:
- Same as Approach 1, plus GraphQL query definition
- `pkg/plugins/plugins.go`: Already has `Query()` in interface — no changes needed

**Complexity**: Medium

**Backwards Compatibility**: Fully compatible

#### Approach 3: Always Show Invitation (no history check)

**Description**: As suggested in the issue as a fallback — always include the bold invitation in the welcome message for all non-members, without checking history.

**Pros**:
- Simplest implementation — just modify the message template
- Zero additional API calls
- No new config needed (beyond an enable/disable toggle)

**Cons**:
- Less effective messaging (issue author notes "diminishes the effectiveness over a positive assertion")
- First-time contributors may find it confusing or premature
- Doesn't provide the personalized "we noticed you've done this before" touch

**Affected Components**:
- `pull-request.go`: Only modify `welcomeMsg()` template

**Complexity**: Very Low

**Backwards Compatibility**: Fully compatible

#### Recommendation

**Preferred Approach**: Approach 1 (REST Search API) with Approach 3 as a starting point.

A phased approach makes sense:
1. **Phase 1**: Modify the welcome message to make the existing org invitation more prominent (bold, with better wording) for all non-members. This is a minimal change.
2. **Phase 2**: Add PR history lookup via `FindIssuesWithOrg()` with a configurable threshold. Show the enhanced "We noticed you've contributed before" message only for repeat contributors.

The REST search API is preferred over GraphQL because it uses existing, well-understood infrastructure and avoids the complexity of defining GraphQL query types. The `FindIssuesWithOrg()` method already exists and is tested.

**Key Implementation Considerations**:
1. New config fields: `RepeatContributorInvitation bool` and `RepeatContributorThreshold int` (default: 3)
2. The search query should be scoped to the org (not just the repo) per the issue's suggestion
3. The invitation message should be configurable via a template or at least the URL
4. Error handling: if the search API call fails, fall back to the generic message (don't block the welcome flow)
5. Consider caching contributor status to avoid repeated API calls on PR reopens

**Testing Requirements**:
- Unit tests for the search query construction
- Unit tests for welcome message with/without invitation
- Tests for threshold logic (below threshold, at threshold, above threshold)
- Tests for API failure fallback behavior
- Mock `FindIssuesWithOrg()` in existing test infrastructure

## Effort Assessment

**Effort Level**: 2 - Moderate (help-needed)

### Summary

This is a well-defined feature addition to the trigger plugin that requires modifying 4-6 files, adding ~200-300 lines of code, and understanding the trigger plugin architecture. The solution is clear, uses existing infrastructure (GitHub search API, config patterns, test fakes), and is fully backwards compatible. It's not trivial enough for a first-time contributor but is well-suited for someone with some Go and Prow familiarity.

### Factor Analysis

#### Scope of Changes
- **Assessment**: Moderate
- **Details**: 4-6 files affected: `config.go` (new config fields), `pull-request.go` (message logic, history lookup), possibly `trigger.go` (interface extension), plus corresponding test files. Estimated ~200-300 LOC.
- **Level Indication**: 2-3

#### Complexity
- **Assessment**: Simple to Moderate
- **Details**: Core logic is straightforward — query PR count, compare against threshold, modify message. No concurrency issues. Edge cases include API failures (need graceful fallback) and threshold boundary conditions.
- **Level Indication**: 1-2

#### Required Expertise
- **Assessment**: Moderate
- **Details**: Requires understanding the trigger plugin architecture (event handling, trust checking, message generation), the Prow config system, and how to use the GitHub client's search API. All learnable from existing code.
- **Level Indication**: 2-3

#### Clarity and Certainty
- **Assessment**: Well-defined
- **Details**: The issue provides a sample message, a query approach, and clear motivation. Minor design decisions remain (exact config field names, threshold default, message wording) but none are blocking.
- **Level Indication**: 1-2

#### Testing Requirements
- **Assessment**: Moderate
- **Details**: Follow existing table-driven test patterns with `fakegithub.FakeClient`. Need to add search result mocking to the fake client (or add a method to the test interface). Test cases: below/at/above threshold, API failure fallback, feature disabled.
- **Level Indication**: 2-3

#### Backwards Compatibility
- **Assessment**: Fully compatible
- **Details**: New config fields default to disabled. Existing deployments see zero behavior change unless they opt in. The existing "Regular contributors should join the org" message remains as-is by default.
- **Level Indication**: 1-2

#### Architectural Alignment
- **Assessment**: Good fit
- **Details**: Follows existing patterns exactly — new config fields in the Trigger struct, conditional message logic in `welcomeMsg()`, GitHub API calls via the existing client interface. Similar to existing draft PR message variation.
- **Level Indication**: 1-2

#### External Dependencies
- **Assessment**: Well-supported
- **Details**: GitHub search API is stable, well-documented, and already used elsewhere in Prow (`FindIssuesWithOrg()`). Rate limits exist but a single search per PR open event is negligible.
- **Level Indication**: 1-3

### Recommended Labels

- [x] `help-wanted`: Well-defined, moderate scope, good for a skilled contributor
- [x] `kind/feature`: New feature addition
- [ ] `good-first-issue`: Requires understanding multiple components, not suitable for first-time contributors

### Guidance for Contributors

**For Level 2 (Moderate)**:
- Suitable for contributors familiar with Go and the Prow plugin system
- Should review:
  - `pkg/plugins/trigger/pull-request.go` — `welcomeMsg()` function and `handlePR()` flow
  - `pkg/plugins/trigger/trigger.go` — `TrustedUser()` and client interfaces
  - `pkg/plugins/config.go` — Trigger config struct
  - `pkg/plugins/trigger/pull-request_test.go` — existing test patterns
- Recommended approach:
  1. Add config fields to Trigger struct (`RepeatContributorInvitation`, `RepeatContributorThreshold`)
  2. Add a function to query merged PR count via `FindIssuesWithOrg()`
  3. Modify `welcomeMsg()` to conditionally include a bold org invitation
  4. Add tests following existing table-driven patterns
- The `JoinOrgURL` config field already exists and should be reused for the invitation link

### Caveats and Considerations

- The issue is from 2024 (migrated from 2019 in test-infra). The original context was Kubernetes-specific contributor experience. Prow is now a standalone project and deployments vary — the feature should be generic enough for any Prow deployment, not just Kubernetes.
- The threshold value (default 3 merged PRs) is somewhat arbitrary. Making it configurable avoids debates about the "right" number.
- Consider whether the search should be org-scoped (as proposed) or repo-scoped. Org-scoped is better for contributor experience but requires broader API permissions.

## Proposed Issue Augmentation

### Title Change
- **Current**: "Feature Request: Issue Org Invitations"
- **Proposed**: "Trigger: invite repeat contributors to join org in ok-to-test message"
- **Rationale**: The current title is ambiguous ("Issue Org Invitations" could mean "issue" as in GitHub issue or as a verb). The new title names the affected component (trigger), describes what it does, and where (ok-to-test message).

### Proposed GitHub Comment

```
/retitle Trigger: invite repeat contributors to join org in ok-to-test message

The trigger plugin's welcome message for non-org-member PRs is generated in `pkg/plugins/trigger/pull-request.go` (`welcomeMsg()` function). It currently includes a generic line: _"Regular contributors should join the org to skip this step"_, but this is buried and vague. The proposed enhancement would make this invitation bold, prominent, and conditional on the contributor's actual merged PR history.

Implementation-wise, the GraphQL approach from the issue would work, but Prow's GitHub client already has a REST-based `FindIssuesWithOrg()` method (in `pkg/github/client.go`) that can query `type:pr author:{user} org:{org} is:merged` to get the count. This avoids defining new GraphQL types while achieving the same result. The trigger plugin's config struct (in `pkg/plugins/config.go`) already has a `JoinOrgURL` field that should be reused for the invitation link. New config fields would control the threshold and feature toggle, defaulting to disabled for backwards compatibility.

Existing test infrastructure (`fakegithub.FakeClient`, table-driven tests in `pull-request_test.go`) provides clear patterns to follow for testing the new behavior with mocked search results.

/area plugins
```

### Rationale

**What's being added**:
- Implementation location (`welcomeMsg()` in `pull-request.go`) — not mentioned in the original issue
- That a REST alternative to the proposed GraphQL exists and is already available in the codebase
- Reference to existing config field (`JoinOrgURL`) that should be reused
- Test infrastructure guidance for contributors

**Why these labels**:
- `/area plugins`: The trigger plugin lives under `pkg/plugins/trigger/`; `area/plugins` is the most specific available label
- No `/kind feature`: Already applied
- No `/help-wanted`: Already applied
- No `/priority`: Already has `priority/backlog`, which is appropriate for a nice-to-have enhancement

**What's NOT included**:
- `/kind feature` and `/help-wanted` — already on the issue
- Priority change — `priority/backlog` is correct for this enhancement
- Detailed implementation steps — the issue is well-specified enough; a contributor can figure out the details from the code references provided

## Briefing Completed

Briefed maintainer on: 2026-02-21

Key questions asked:
- None — maintainer acknowledged all slides without additional questions

Maintainer decision:
Proceed with wrapup and posting the augmentation comment.

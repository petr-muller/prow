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

## Next Steps

- Assess effort level
- Propose augmented issue content

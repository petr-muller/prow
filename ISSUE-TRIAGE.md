# Triage for Issue #279

**Status**: In Progress
**Created**: 2026-01-31

## Issue Information

- **Issue Number**: #279
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/279

## Findings

### Initial Validation

**Assessment**: LEGITIMATE

#### Analysis

This issue requests a Prow plugin to send Slack alerts when a PR without a valid CLA is merged. The issue was opened by @pacoxu (MEMBER) and has received substantial discussion from maintainers including @petr-muller and @BenTheElder.

**Issue Category**: Feature Request

**Repository Scope Check**:
- Component mentioned: Prow plugin / slackevents plugin
- Exists in this repo: Yes (`pkg/plugins/slackevents/`)
- Relevant code paths: `pkg/plugins/slackevents/`, `pkg/slack/`

**Information Completeness**:
- Sufficient detail provided: Yes
- Use case: Alert when PRs with `cncf-cla: no` status are merged (rare but important compliance issue)
- Discussion has refined the approach significantly

#### Key Discussion Points from Maintainers

1. **@petr-muller**: Suggested simple single-purpose plugin over generic notification system; identified `slackevents` plugin as the right place to extend
2. **@BenTheElder**: Important clarification - should check CLA *status context*, not the label (status is source of truth)
3. **Merge strategy complication**: For `merge` commits, status is inherited. For `rebase`/`squash`, original status is lost and requires looking up the original PR
4. **Alternative considered**: Prometheus metrics + alertmanager, but dismissed due to cardinality issues for tracking *which* merge was problematic

#### Current State

- Labels: `kind/feature`, `help wanted`, `lifecycle/frozen`
- Last substantive update: @petr-muller (Feb 2025) identified `slackevents` as the target plugin
- Referenced: kubernetes/community#8447 (CLA-related)

#### Recommendation

**Keep open and continue triage.** This is a well-defined feature request with maintainer consensus on approach:
- Extend `slackevents` plugin to check CLA status on push events
- Handle merge commits differently from squash/rebase commits
- Has `help wanted` label indicating it's ready for contribution

**Suggested Action**: Continue to research phase to understand `slackevents` plugin implementation and design the solution

---

### Code Research

#### Current Implementation

**Primary Components**:
- `pkg/plugins/slackevents/slackevents.go` - Main plugin handling push events and comment mentions
- `pkg/slack/client.go` - Slack API client for sending messages
- `pkg/github/client.go` - GitHub API client with commit status methods
- `pkg/plugins/cla/cla.go` - Existing CLA plugin (reference for status checking pattern)

**Architecture Overview**:
The slackevents plugin registers two event handlers:
1. **PushEventHandler** - Detects manual merges and sends Slack alerts
2. **GenericCommentHandler** - Relays SIG mentions to Slack channels

The push event flow:
1. Plugin receives `github.PushEvent` from webhook
2. Handler calls `notifyOnSlackIfManualMerge(client, pushEvent)`
3. Fetches merge warning configuration for the repo
4. Checks if the pusher is exempted (user/branch lists)
5. Sends Slack message via `slackClient.WriteMessage()`

**Key Code Paths**:
1. Push event registration: `pkg/plugins/slackevents/slackevents.go:46` - `plugins.RegisterPushEventHandler`
2. Push handler: `pkg/plugins/slackevents/slackevents.go` - `handlePush`, `notifyOnSlackIfManualMerge`
3. Commit status API: `pkg/github/client.go:2814` - `GetCombinedStatus(org, repo, ref)`
4. CLA status pattern: `pkg/plugins/cla/cla.go` - Shows how to check for "EasyCLA" context

**Data Flow for New Feature**:
1. Push event arrives → `handlePush(pc plugins.Agent, pe github.PushEvent)`
2. Extract commit SHA from `pe.After`
3. Call `pc.GitHubClient.GetCombinedStatus(org, repo, pe.After)` to get statuses
4. Look for CLA status context (e.g., "EasyCLA") in returned statuses
5. If CLA status is failure/error/missing, send Slack alert

#### Related Code

**Plugin Framework** (`pkg/plugins/plugins.go:188`):
```go
type Agent struct {
    GitHubClient PluginGitHubClient  // Has GetCombinedStatus method
    SlackClient  *slack.Client       // Has WriteMessage method
    PluginConfig *Configuration      // Has Slack config
}
```

**Status Data Structures** (`pkg/github/types.go`):
```go
type Status struct {
    State       string  // "success", "failure", "error", "pending"
    Context     string  // e.g., "EasyCLA"
    Description string
    TargetURL   string
}

type CombinedStatus struct {
    SHA      string
    Statuses []Status
    State    string  // Overall state
}
```

**Existing Configuration** (`pkg/plugins/config.go:760-769`):
```go
type MergeWarning struct {
    Repos          []string            // "org/repo" or "org" entries
    Channels       []string            // Slack channels to notify
    ExemptUsers    []string            // Users exempt from warnings
    ExemptBranches map[string][]string // Branch-specific exemptions
}
```

**Similar Functionality**:
- `pkg/plugins/cla/cla.go` - CLA plugin checks for "EasyCLA" status context
- Pattern: `if se.Context != "EasyCLA" { return nil }`

#### Test Coverage

**Existing Tests**:
- `pkg/plugins/slackevents/slackevents_test.go` - Tests push event handling
- `pkg/plugins/cla/cla_test.go` - Tests CLA status checking
- Both use `fakegithub.NewFakeClient()` for mocking

**Test Gaps**:
- No tests for CLA status checking on push events (new functionality)

#### Root Cause Analysis

**What's Missing**:
The slackevents plugin currently only checks:
- If the push was a manual merge (vs. Tide-managed)
- If the user/branch is exempt

It does NOT check:
- Commit status contexts (like CLA status)
- Whether the merged PR had valid CLA

**Contributing Factors**:
1. Original design focused on manual merge detection, not status verification
2. Merge strategy complications (squash/rebase lose original commit status)
3. No existing hook point for "check status on merge" logic

**Merge Strategy Complications**:
| Strategy | Status Inheritance | Solution |
|----------|-------------------|----------|
| `merge` | Merge commit inherits PR status | Check status on `pe.After` SHA |
| `squash` | New commit, status lost | Must look up original PR by commit message |
| `rebase` | New commits, status lost | Must look up original PR by commit message |

#### Proposed Solutions

##### Approach 1: Extend slackevents with CLA Status Check (Recommended)

**Description**: Add CLA status checking to the existing push event handler in slackevents plugin. On each push to protected branches, query commit status and alert if CLA context shows failure.

**Implementation Steps**:
1. Add new config struct `CLAStatusAlert` with repos, channels, context name
2. In push handler, after existing checks, query `GetCombinedStatus()`
3. Search for CLA context in statuses
4. If state != "success", send Slack alert with commit details

**Pros**:
- Follows existing patterns (MergeWarning)
- Reuses existing infrastructure
- Simple, focused change
- Single-purpose as maintainers requested

**Cons**:
- Only works reliably for `merge` strategy repos
- May need separate logic for squash/rebase repos

**Affected Components**:
- `pkg/plugins/slackevents/slackevents.go` - Add new handler logic
- `pkg/plugins/config.go` - Add configuration struct

**Complexity**: Low-Medium

**Backwards Compatibility**: Full (additive change)

##### Approach 2: Use PR Merged Event Instead of Push Event

**Description**: Listen for PR merged events instead of push events. The PR event has direct access to the PR number and can query original commit status.

**Pros**:
- Works consistently across all merge strategies
- Direct access to PR context (author, commits, etc.)
- No need to parse commit messages

**Cons**:
- Requires registering new event handler type
- May not be supported by current plugin infrastructure
- More significant change to plugin structure

**Complexity**: Medium-High

##### Approach 3: Hybrid - Push Event + PR Lookup

**Description**: On push event, use GitHub API to find the associated PR (via commit search), then check status on the original PR commits.

**Pros**:
- Works for all merge strategies
- Still uses push event trigger

**Cons**:
- Additional API calls per push
- More complex logic
- Rate limit considerations

**Complexity**: Medium

##### Recommendation

**Preferred Approach**: Approach 1 (Extend slackevents with CLA Status Check)

This is the simplest solution that addresses the core use case. Per @BenTheElder's comment, the alert is "only a best effort warning in any case" since statuses can change after merge. A simple implementation that works for `merge` strategy repos provides immediate value.

**Key Implementation Considerations**:
1. Check CLA status context specifically (e.g., "EasyCLA")
2. Alert on failure/error/missing states
3. Include PR/commit details in alert message
4. Follow MergeWarning config pattern for consistency
5. Document limitation with squash/rebase strategies

**Testing Requirements**:
- Unit tests with mock GitHub client returning various status states
- Test cases: CLA success (no alert), CLA failure (alert), CLA missing (configurable)
- Test exemption logic if added

---

### Effort Assessment

**Effort Level**: 2 - Moderate (help-needed)

#### Summary

Well-defined feature request with clear solution approach, following existing plugin patterns. Moderate scope (3-4 files, ~150-200 LOC) with some edge cases around merge strategies. Suitable for contributors familiar with Prow plugins.

#### Factor Analysis

##### Scope of Changes
- **Assessment**: Moderate
- **Details**:
  - `pkg/plugins/slackevents/slackevents.go` - Add CLA check logic (~50-80 LOC)
  - `pkg/plugins/config.go` - Add configuration struct (~20-30 LOC)
  - `pkg/plugins/slackevents/slackevents_test.go` - Add test cases (~50-80 LOC)
  - Total: 3-4 files, ~150-200 lines of code
- **Level Indication**: 2

##### Complexity
- **Assessment**: Simple to Moderate
- **Details**:
  - Follows existing MergeWarning pattern closely
  - Simple logic: query status API, check for CLA context, send alert
  - No concurrency concerns
  - Edge case: handling different merge strategies (documented limitation acceptable)
- **Level Indication**: 1-2

##### Required Expertise
- **Assessment**: Moderate
- **Details**:
  - Need to understand Prow plugin framework (can learn from slackevents code)
  - Need to understand GitHub status API (well documented, existing examples)
  - No deep Prow architectural knowledge required
- **Level Indication**: 2

##### Clarity and Certainty
- **Assessment**: Well-defined
- **Details**:
  - Problem clearly stated
  - Solution approach agreed upon by maintainers
  - Implementation pattern exists (MergeWarning)
  - Minor limitation accepted: squash/rebase handling deferred
- **Level Indication**: 1-2

##### Testing Requirements
- **Assessment**: Simple to Moderate
- **Details**:
  - Follow existing test patterns in `slackevents_test.go`
  - Mock GitHub client with `fakegithub.NewFakeClient()`
  - Test cases: CLA success (no alert), CLA failure (alert), CLA missing
- **Level Indication**: 1-2

##### Backwards Compatibility
- **Assessment**: Fully compatible
- **Details**:
  - Additive change only
  - New feature requires explicit configuration to enable
  - No impact on existing deployments without new config
- **Level Indication**: 1

##### Architectural Alignment
- **Assessment**: Perfect fit
- **Details**:
  - Extends existing slackevents plugin (maintainer recommendation)
  - Follows established MergeWarning configuration pattern
  - Uses existing plugin infrastructure and APIs
- **Level Indication**: 1-2

##### External Dependencies
- **Assessment**: Well-supported
- **Details**:
  - GitHub status API is stable and well-documented
  - `GetCombinedStatus()` already used elsewhere in Prow
  - Slack API already integrated
- **Level Indication**: 1-2

#### Recommended Labels

Based on this assessment:
- [x] `help wanted` (already applied): Moderate complexity, suitable for skilled contributors
- [x] `kind/feature` (already applied): New functionality
- [ ] `good-first-issue`: Not appropriate - requires understanding plugin patterns and multiple components

#### Guidance for Contributors

**Prerequisites**:
- Familiarity with Go
- Basic understanding of Prow plugin architecture
- Review `pkg/plugins/slackevents/slackevents.go` for existing patterns

**Suggested Approach**:
1. Study the existing `MergeWarning` implementation in slackevents
2. Review how `GetCombinedStatus()` is used in `pkg/plugins/cla/cla.go`
3. Add new config struct similar to `MergeWarning` in `pkg/plugins/config.go`
4. Add handler logic to check CLA status on push events
5. Write tests following existing patterns

**Key Files to Review**:
- `pkg/plugins/slackevents/slackevents.go` - Main plugin logic
- `pkg/plugins/config.go` - Configuration patterns
- `pkg/plugins/cla/cla.go` - CLA status checking example
- `pkg/github/client.go:2814` - `GetCombinedStatus` API

**Open Questions for Contributor**:
1. Should missing CLA status context trigger an alert? (Likely no - may indicate non-CLA repo)
2. Should specific branches be configurable? (Probably yes, follow MergeWarning pattern)

#### Caveats and Considerations

- **Merge strategy limitation**: The simple approach only works reliably for repositories using the `merge` strategy. For `squash`/`rebase` repos, the original commit status is lost. This is an acceptable limitation per maintainer discussion - "best effort warning" is sufficient.
- **Rate limits**: Each push event triggers a status API call. For high-volume repos, this could add up, but should be negligible for the rare case of CLA failures.

---

### Proposed Issue Augmentation

#### Title Change

- **Current**: "alert prow plugin for no cla PR merges"
- **Proposed**: "slackevents: Add alert for PRs merged without valid CLA status"
- **Rationale**: The current title has awkward phrasing and doesn't specify which plugin. The new title follows Prow conventions (component prefix), is grammatically correct, and clearly describes the feature.

#### Proposed GitHub Comment

```
/retitle slackevents: Add alert for PRs merged without valid CLA status

## Implementation Guide

The existing `MergeWarning` feature in slackevents (`pkg/plugins/slackevents/slackevents.go`) provides a pattern to follow. On push events, it checks repo/branch configuration and sends Slack alerts. A CLA status check would work similarly:

1. Add a new config struct (e.g., `StatusContextAlert`) in `pkg/plugins/config.go` following the `MergeWarning` pattern - with repos, channels, and a configurable context name (defaulting to "EasyCLA")
2. In the push handler, call `GetCombinedStatus(org, repo, pe.After)` to retrieve commit statuses
3. Search for the CLA context in the returned statuses; if state is not "success", send an alert

The `pkg/plugins/cla/cla.go` plugin shows how to check for the "EasyCLA" status context. For repos using the `merge` strategy, the merged commit inherits the PR's status contexts directly. For `squash`/`rebase` repos, statuses are not inherited - this is an acceptable limitation given this is a "best effort" warning.

/area plugins
```

#### Rationale

**What's being added**:
- Specific implementation guidance: which files to modify, which patterns to follow
- API to use: `GetCombinedStatus()`
- Configuration approach: follow MergeWarning pattern
- Acknowledgment of the merge strategy limitation with reasoning

**Why these labels**:
- `/retitle`: Current title is awkward and doesn't specify the plugin; new title follows Prow conventions
- `/area plugins`: This is a plugins area change (slackevents is a plugin)
- No `/kind feature`: Already applied
- No `/help-wanted`: Already applied (appropriate for Level 2)

**What's NOT included**:
- Root cause analysis: Not applicable (this is a feature request, not a bug)
- Detailed code snippets: Too verbose for an issue comment; the patterns are learnable from the codebase
- Workarounds: Not applicable
- Priority label: Not warranted - this is a nice-to-have feature for rare compliance events

---

## Briefing Completed

Briefed maintainer on: 2026-01-31

Key points covered:
- Issue overview and legitimacy (LEGITIMATE feature request)
- Technical context (slackevents plugin, MergeWarning pattern)
- Solution approach (extend push handler with CLA status check)
- Effort assessment (Level 2 - Moderate)
- Recommendations (post comment, apply /area plugins)

Maintainer decision: Proceed to wrapup and post comment

## Next Steps

1. Post the augmentation comment to the issue (via wrapup)
2. Monitor for contributor interest given the `help wanted` label

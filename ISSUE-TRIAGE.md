# Triage for Issue #468

**Status**: In Progress
**Created**: 2026-01-26

## Issue Information

- **Issue Number**: #468
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/468

## Findings

### Initial Validation

**Assessment**: LEGITIMATE

**Issue Category**: Bug

**Repository Scope Check**:
- Component mentioned: cherrypicker (external plugin)
- Exists in this repo: Yes
- Relevant code paths:
  - cmd/external-plugins/cherrypicker/server.go:402 (labels attributed to PR author)
  - cmd/external-plugins/cherrypicker/server.go:422-429 (permission checking)

**Information Completeness**:
- Sufficient detail provided: Yes
- Reproduction cases: Provided (PRs in istio/istio.io repo)
- Root cause analysis: Confirmed by maintainer @smg247
- Code references: Specific line numbers provided

### Analysis

This is a legitimate bug in the cherrypicker external plugin. The issue describes a problematic behavior where:

1. **The Problem**: When an org member adds a cherrypick label to a PR, the cherrypicker plugin may silently fail to act if the PR author is not an org member (even though the label-setter is)

2. **Root Cause**: The code at server.go:402 treats all cherrypick labels as if they were added by the PR author (`pr.User.Login`), not the actual person who set the label. When permission checking occurs at lines 422-429, if the PR author is not an org member, the cherry-pick request is silently removed from the queue with no user feedback.

3. **Technical Details**:
   - GitHub's label API doesn't store information about who added a label
   - The plugin operates after PR merge, making it difficult to capture the label event in real-time
   - The current implementation assumes label = comment from PR author

4. **User Impact**:
   - Intermittent failures that are confusing to users
   - No feedback when action is silently ignored
   - Forces users to leave comments instead of using labels
   - Documented in real-world PRs from Istio project

5. **Maintainer Confirmation**: @smg247 (MEMBER) confirmed the bug, identified the code location, and acknowledged that while fixing the root cause would require architectural changes (webhook handling + state storage), adding a notification comment would be straightforward.

The issue is already correctly labeled with `kind/bug` and `area/plugins`.

### Recommendation

**Suggested Action**: Keep open and continue triage

This is a well-documented, legitimate bug with:
- Clear reproduction steps and real-world examples
- Root cause identified by maintainers
- Specific code locations pinpointed
- Proposed solutions (both ideal and practical)
- Active interest from affected users (Istio project)

The issue should proceed to research and effort assessment phases to determine:
1. Feasibility of the ideal solution (treating label as coming from label-setter)
2. Complexity of the practical workaround (adding notification comment)
3. Appropriate difficulty labeling for potential contributors

### Code Research

**Primary Components**:
- Cherrypicker Server: cmd/external-plugins/cherrypicker/server.go - Handles webhook events, processes cherry-pick requests
- Event Handler: server.go:141-184 - Receives and dispatches GitHub webhook events
- Permission Checker: server.go:415-430 - Validates org membership for requesters

**Architecture Overview**:
The cherrypicker plugin operates as an external webhook server that responds to GitHub pull_request and issue_comment events. When a PR is merged, it examines both comments (with /cherrypick commands) and labels (with cherrypick/ prefix) to determine which branches need cherry-picks. It then creates new PRs targeting those branches.

**Key Code Paths**:
1. Webhook dispatch: server.go:141-184 - Validates HMAC, routes to event handlers
2. Comment processing: server.go:372-385 - Parses /cherrypick commands, attributes to comment author
3. Label processing: server.go:399-405 - Matches cherrypick/ labels, **attributes to PR author**
4. Permission checking: server.go:415-430 - Lists org members, **silently deletes non-members**
5. Notification: server.go:648-660 - Creates GitHub comments for success/failure

**Data Flow**:
1. GitHub sends pull_request webhook (action: labeled, closed, opened)
2. Server validates signature and dispatches to handlePullRequest
3. Handler fetches all comments and labels
4. Comments → parsed for /cherrypick commands → mapped to comment.User.Login
5. Labels → matched against prefix → **mapped to pr.User.Login (BUG)**
6. If !allowAll: Lists org members and filters requesterToComments map
7. For non-members: **Comments get notification, labels get silently dropped**
8. Remaining requests proceed to create cherry-pick PRs

### Related Code

**Critical Finding - Sender Field Available**:
The PullRequestEvent webhook includes a `Sender` field (pkg/github/types.go) that contains the user who performed the action (including adding a label). The current code ignores this field for label-initiated cherry-picks.

**Dependencies**:
- pkg/github package: GitHub API client for membership checks, comments, PR operations
- pkg/plugins: FormatICResponse for comment formatting
- localgit package: Git operations for cherry-picking

**Callers**:
- GitHub webhook infrastructure calls ServeHTTP directly
- No internal callers - standalone external plugin

**Similar Functionality**:
- Hold plugin (pkg/plugins/hold/hold.go): Manages labels but doesn't need user attribution
- Cherrypickapproved plugin: Related but different purpose (auto-approval)

### Test Coverage

**Existing Tests**:
- server_test.go:876-1058: testCherryPickPRWithLabels - Tests label-based cherry-picks
- server_test.go:1154-1232: Tests assignment to requester
- server_test.go:1060-1152: Tests issue creation on conflicts

**Coverage Assessment**: Partial
- Label processing is tested
- Assignment logic is tested
- **Missing**: No test for permission denial on label-initiated cherry-picks
- **Missing**: No test for notification when label requester != PR author
- All existing tests use `isMember: true` - no negative permission tests

**Test Gaps**:
- Scenario: Org member adds label to PR from non-member author
- Scenario: Silent failure when PR author lacks permissions
- Scenario: Notification behavior difference between labels and comments

### Documentation Review

**Code Comments**:
- Line 49: notOrgMemberMessageTemplate - Documents the notification message for denied requests
- Line 402: No comment explaining why labels are attributed to pr.User.Login
- Line 417: TODO comment about caching org members

**Design Documentation**:
- site/content/en/docs/components/external-plugins/cherrypicker.md: Plugin documentation
- No mention of the label attribution behavior in docs

**Known Limitations**:
- Comment at server.go:417 mentions issue discussed by @smg247: "GitHub API doesn't store who added a label"
- However, the webhook **does** include Sender field which is currently unused

### Root Cause Analysis

**Primary Cause**:
The code at server.go:402 hardcodes label-initiated cherry-pick requests as coming from the PR author (`pr.User.Login`), ignoring the available `PullRequestEvent.Sender` field that contains the actual user who added the label. This misattribution causes permission checks to evaluate the wrong user.

**Contributing Factors**:
1. **Incorrect assumption**: Code assumes GitHub doesn't provide who added the label, but `PullRequestEvent.Sender` is available
2. **Silent deletion**: server.go:427 deletes non-member requesters from the map without notification
3. **Inconsistent behavior**: Comments trigger immediate permission checks with notifications (lines 234-243, 273-284), but labels skip this step
4. **Missing test coverage**: No tests for permission denied scenarios on label-initiated picks

**Reproduction Conditions**:
- PR author is not an org member (or has private membership)
- Org member adds a cherrypick/ label to a merged PR
- Plugin configuration has `allowAll: false` (default)
- Expected: Cherry-pick happens (label was added by org member)
- Actual: Silent failure (plugin checks PR author, not label adder)

### Proposed Solutions

#### Approach 1: Use PullRequestEvent.Sender for Label Attribution

**Description**: Modify server.go:399-405 to attribute label-initiated cherry-picks to the user who added the label (`pre.Sender.Login`) instead of the PR author (`pr.User.Login`). This field is already available in the webhook payload.

**Pros**:
- Fixes the root cause directly
- Uses existing webhook data (no API changes needed)
- Consistent with user expectations (person who adds label should be requester)
- No architectural changes required
- Minimal code change (1-2 lines)

**Cons**:
- Behavior change could be unexpected if anyone relies on current behavior
- Need to verify Sender is always populated in labeled events
- May affect cherry-pick PR assignment logic if it depends on requester

**Affected Components**:
- server.go:399-405: Change `pr.User.Login` to `pre.Sender.Login` when processing labels
- Potentially server.go:423-429: Permission check logic (should work as-is)
- Tests: Need new tests for this scenario

**Complexity**: Low

**Backwards Compatibility**:
- Minor behavior change: Cherry-pick PRs initiated by labels will be attributed to label-adder instead of PR author
- If `use-prow-assignments` is enabled, assignments would change
- Most deployments likely want the new behavior (it's more correct)

#### Approach 2: Add Notification for Silent Permission Failures

**Description**: When a label-initiated cherry-pick is filtered out due to permission check (server.go:427), create a notification comment informing that the request was denied. Similar to how comment-based requests notify (lines 241-243, 280-282).

**Pros**:
- Provides user feedback (eliminates silent failures)
- Minimal code change
- Doesn't change attribution behavior
- Low risk

**Cons**:
- Doesn't fix the root cause (PR author still checked instead of label adder)
- Users still can't use labels if PR author isn't an org member
- Adds noise with notification comments
- Notification would be confusing ("PR author can't cherry-pick" when label was added by authorized user)

**Affected Components**:
- server.go:427: Instead of just `delete()`, add notification comment
- Would need to track which label triggered the request for proper notification

**Complexity**: Low

**Backwards Compatibility**: No breaking changes, only adds notifications

#### Approach 3: Hybrid - Fix Attribution + Improve Notifications

**Description**: Combine Approach 1 and Approach 2. Use Sender for attribution AND improve notification messaging to clearly explain who requested the cherry-pick and why it was denied.

**Pros**:
- Best user experience
- Fixes root cause
- Provides clear feedback
- Future-proof

**Cons**:
- Slightly more work than Approach 1 alone
- Need to update notification templates

**Complexity**: Low-Medium

**Backwards Compatibility**: Same as Approach 1

#### Approach 4: Re-architect with Label Webhook Events (Not Recommended)

**Description**: As suggested by @smg247, listen to label webhook events in real-time and store state about who added labels. This would require persistent storage.

**Pros**:
- Could provide audit trail of all label operations
- More information available for other use cases

**Cons**:
- Significant architectural change
- Requires persistent storage (database/cache)
- Much more complex than necessary
- Webhook data already has the needed information (Sender field)
- Overkill for this problem

**Complexity**: High

**Backwards Compatibility**: Would require migration of existing deployments

#### Recommendation

**Preferred Approach**: Approach 3 (Hybrid - Fix Attribution + Improve Notifications)

This provides the best user experience by both fixing the root cause and providing clear feedback. Since we're touching the code anyway, improving notifications is low additional effort.

**Key Implementation Considerations**:
1. Verify `PullRequestEvent.Sender` is always populated for labeled events
2. Update label processing to use `pre.Sender.Login` instead of `pr.User.Login`
3. Add notification when permission check fails, explaining who attempted the cherry-pick
4. Update notification template to mention label-based requests explicitly
5. Ensure assignment logic (if enabled) handles requester correctly

**IMPORTANT LIMITATION**:
This fix only works when labels are added **after** PR merge (which triggers a "labeled" event with `Sender`). If a label is added **before** merge, the plugin processes it during the "closed" event, where only label names are available via `GetIssueLabels()` API - not who added them. The full fix for pre-merge labels would require listening to "labeled" events in real-time and storing state about who added which label. However, the hybrid approach still provides value:
- Fixes the post-merge labeling workflow
- Adds notifications for denied requests in all cases
- Serves as a stepping stone toward full solution with state tracking

**Testing Requirements**:
- Test: Org member adds label to PR from non-member author (should succeed)
- Test: Non-member adds label to any PR (should fail with notification)
- Test: Verify PR assignment uses label-adder when label triggers cherry-pick
- Test: Verify notification messages are clear and helpful

**Migration/Rollout Strategy**:
- Low risk change, can be deployed immediately
- Document behavior change in release notes
- No configuration migration needed
- Existing deployments benefit automatically

### Effort Assessment

**Effort Level**: 2 - Moderate (help-needed)

**Summary**

This is a well-defined bug with a straightforward fix (using `PullRequestEvent.Sender` instead of `pr.User.Login`), but requires understanding webhook event structures, permission checking flow, and careful testing of edge cases. The small scope and clear solution make it accessible, but the backwards compatibility considerations and need for comprehensive testing place it beyond a good-first-issue.

#### Factor Analysis

**Scope of Changes**
- **Assessment**: Small
- **Details**:
  - Approach 1 (minimal): 1 file (server.go), 3-5 lines of code
  - Approach 3 (recommended): 1-2 files, ~20-40 lines of code
  - Single component affected (cherrypicker external plugin)
- **Level Indication**: 1-2

**Complexity**
- **Assessment**: Simple to Moderate
- **Details**:
  - Core fix is simple: change field reference from `pr.User.Login` to `pre.Sender.Login`
  - Adding notification follows existing patterns
  - No concurrency, race conditions, or algorithmic challenges
  - Minor complexity: need to verify `Sender` field is always populated in labeled events
- **Level Indication**: 1-2

**Required Expertise**
- **Assessment**: Moderate
- **Details**:
  - Understanding of GitHub webhook event structures (PullRequestEvent)
  - Familiarity with permission checking flow in the plugin
  - Knowledge of Go testing patterns
  - Can learn from existing code, but requires some time to understand the flow
  - No deep Prow architecture knowledge needed
- **Level Indication**: 2

**Clarity and Certainty**
- **Assessment**: Well-defined
- **Details**:
  - Problem is clearly identified and root cause understood
  - Solution approach is unambiguous (research revealed Sender field is available)
  - One minor verification needed: confirm Sender is always populated
  - Recommended approach (Hybrid) has clear implementation steps
- **Level Indication**: 1-2

**Testing Requirements**
- **Assessment**: Moderate
- **Details**:
  - Need new test cases for permission denied scenarios
  - Test: Org member adds label to PR from non-member author (should succeed)
  - Test: Non-member adds label (should fail with notification)
  - Test: Verify assignment uses correct requester
  - Can follow existing test patterns (testCherryPickPRWithLabels)
  - All unit tests, no complex integration testing required
- **Level Indication**: 2

**Backwards Compatibility**
- **Assessment**: Minor impact, backwards compatible
- **Details**:
  - Behavior change: label-initiated cherry-picks attributed to label-adder instead of PR author
  - Impact on assignments: If `use-prow-assignments` enabled, PRs will be assigned differently
  - Most deployments will benefit from the new behavior (it's more correct)
  - No configuration changes required
  - Should be documented in release notes
  - No breaking changes
- **Level Indication**: 2

**Architectural Alignment**
- **Assessment**: Perfect fit
- **Details**:
  - Uses existing webhook field that was previously overlooked
  - Follows existing patterns for permission checking
  - Notification improvements follow existing createComment patterns
  - Doesn't introduce new architectural patterns
  - Aligns with user expectations (person who adds label should be requester)
- **Level Indication**: 1-2

**External Dependencies**
- **Assessment**: Well-supported
- **Details**:
  - Relies on GitHub webhook's `PullRequestEvent.Sender` field
  - GitHub webhook API is stable and well-documented
  - Need to verify field is always populated (low risk)
  - No changes to external systems required
- **Level Indication**: 1-3

#### Recommended Labels

Based on this assessment, recommend the following labels:

- [x] `help-needed`: Well-defined issue suitable for skilled contributor
- [x] `kind/bug`: Fixing incorrect permission attribution
- [x] `area/plugins`: External plugin (cherrypicker)
- [ ] `good-first-issue`: Requires understanding webhook flow and permission checking - beyond typical first issue

#### Guidance for Contributors

**For Level 2 (Moderate)**:

**Suitable for**: Contributors with Go experience and familiarity with webhooks/REST APIs

**Prerequisites**:
- Understanding of Go testing
- Familiarity with GitHub webhook events
- Ability to read and understand existing code patterns

**Recommended preparation**:
1. Review the webhook event handling flow in server.go:141-184
2. Study how comments are processed vs labels (server.go:372-405)
3. Examine permission checking logic (server.go:415-430)
4. Review existing tests in server_test.go:876-1058
5. Read GitHub's webhook documentation for PullRequestEvent

**Implementation approach** (Approach 3 - Hybrid):
1. Verify `PullRequestEvent.Sender` is always populated for labeled events
2. Modify label processing (server.go:402) to use `pre.Sender.Login` instead of `pr.User.Login`
3. When permission check filters out a requester, add notification comment
4. Update notification templates to clearly indicate label-based vs comment-based requests
5. Add comprehensive tests for new permission scenarios
6. Test with `use-prow-assignments` enabled to verify correct assignment

**Key files to modify**:
- cmd/external-plugins/cherrypicker/server.go (primary changes)
- cmd/external-plugins/cherrypicker/server_test.go (new tests)

**Estimated effort**: 2-4 hours for implementation + 1-2 hours for testing

#### Caveats and Considerations

**Minor uncertainty**: While the research found that `PullRequestEvent` includes a `Sender` field, the implementation should verify this field is always populated for the `labeled` action. GitHub's webhook documentation should confirm this, and a defensive check (fallback to `pr.User.Login` if `Sender` is nil) could be added if needed.

**Testing note**: The existing test suite doesn't have dedicated permission denied tests. The contributor will be establishing a new test pattern for this scenario, which future contributors can follow.

**Backwards compatibility note**: While technically a behavior change, this is a **bug fix** - the current behavior is incorrect. Users experiencing the reported issue will see it fixed; users not hitting this scenario will see no change. The main observable difference is in PR assignments when `use-prow-assignments` is enabled.

### Proposed Issue Augmentation

**Title Change**
- **No change needed**: Current title accurately describes the behavioral difference users observe

**Proposed GitHub Comment**

```markdown
## Root Cause and Solution

The root cause is at cmd/external-plugins/cherrypicker/server.go:402, where label-initiated cherry-picks are attributed to `pr.User.Login` (the PR author) instead of the person who added the label. While @smg247's comment noted that GitHub's Label API doesn't include user information, the `PullRequestEvent` webhook **does** include a `Sender` field containing the user who performed the labeled action. This field is currently unused.

Additionally, when permission checking filters out non-org members, comment-based requests receive a notification (server.go:241-243), but label-based requests are silently deleted from the queue (server.go:427) with no user feedback. This creates confusing intermittent failures.

## Recommended Fix (Partial Solution)

Use `pre.Sender.Login` instead of `pr.User.Login` when processing labels, and add notification comments when label-based requests are denied. This is a small change (~20-40 lines) affecting server.go and server_test.go.

**Important limitation**: This only works when labels are added **after** PR merge. Labels added **before** merge are processed during the "closed" event where `Sender` info isn't available. The full solution would require listening to "labeled" events in real-time and storing state. However, this partial fix still provides value by handling post-merge labeling and adding notifications in all cases.

/help-wanted
```

**Rationale**

**What's being added**:
- **Key discovery**: The Sender field is available in the webhook, contradicting the assumption that the information isn't accessible
- **Second issue**: Silent failures for label-based permission denials (not mentioned in original issue)
- **Solution clarity**: Specific fix approach with file references and scope estimate
- **Contributor guidance**: Points to what needs to be done and tested

**Why these labels**:
- `/area plugins`: Already applied correctly
- `/kind bug`: Already applied correctly
- `/help-wanted`: Level 2 effort assessment - well-defined bug suitable for skilled contributors. Not good-first-issue because it requires understanding webhook event structures and permission checking flow.

**What's NOT included**:
- No /retitle: Current title is clear and accurate
- No priority label: While this causes intermittent failures, it's been present since the plugin's creation and has workarounds (use comments). Let maintainers decide priority.
- Didn't repeat the problem description or examples (already in original issue)
- Didn't include all 4 solution approaches (too verbose; just recommended approach)
- Kept under 3 paragraphs as instructed

## Next Steps

1. ✓ Initial validation complete - issue is LEGITIMATE
2. ✓ Code research complete - Root cause identified, solutions proposed
3. ✓ Effort assessment complete - Level 2 (help-needed)
4. ✓ Augmentation proposed - Ready for maintainer review
5. Final: Use wrapup subcommand to push branches and post comment

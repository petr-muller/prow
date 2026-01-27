# Triage for Issue #194

**Status**: In Progress
**Created**: 2026-01-27

## Issue Information

- **Issue Number**: #194
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/194

## Findings

### Initial Validation

**Assessment**: LEGITIMATE

**Issue Category**: Feature Request

**Repository Scope Check**:
- Component mentioned: Prow integration with GitHub Actions/Workflows
- Exists in this repo: Yes - this is a feature request for Prow itself
- Relevant area: GitHub workflow approval automation

**Information Completeness**:
- Sufficient detail provided: Yes
- Missing information: None critical
- GitHub API endpoint referenced: https://docs.github.com/en/rest/reference/actions#approve-a-workflow-run-for-a-fork-pull-request
- Use case clearly explained: New contributors blocked from running workflows even after `ok-to-test` label added

### Analysis

This is a legitimate feature request for Prow to integrate with GitHub's workflow approval API. The issue asks for Prow to automatically approve GitHub workflow runs when a maintainer adds the `ok-to-test` label to a PR from a new contributor.

**Key Points**:
1. **Valid Use Case**: Multiple Kubernetes ecosystem projects (including Volcano and Kubeflow) have this need
2. **Current Pain Point**: Maintainers must manually approve workflows through GitHub UI even after adding `ok-to-test` label
3. **Workarounds Exist**: Communities are using GitHub Actions to trigger other Actions (see comments), but this is suboptimal
4. **Active Development**: AaruniAggarwal assigned themselves (Oct 2025) and posted an update today (Jan 27, 2026) asking about configuration approach
5. **Community Interest**: Issue kept alive multiple times by removing lifecycle/stale label, indicating sustained interest

**Current Labels**:
- `kind/feature` ✓ (appropriate)
- `help wanted` ✓ (appropriate, though someone is now assigned)
- `sig/contributor-experience` ✓ (appropriate - relates to contributor workflow)

**Lifecycle**:
- Created: June 14, 2024
- Status: Open, actively being worked on
- Has received multiple `/remove-lifecycle stale` commands showing continued relevance

### Recommendation

**Suggested Action**: Keep open and continue triage

This is a well-defined feature request with clear community need. The issue provides sufficient information to implement the feature:
- GitHub API endpoint to call
- Clear trigger (ok-to-test label)
- Valid use case with multiple affected communities

**Next Triage Steps**:
1. Research the code to understand where this integration would fit
2. Assess implementation effort
3. Consider if augmentation is needed (appears well-written already, but could benefit from technical context)

### Code Research

#### Current Implementation

**Primary Components**:
- **Trigger Plugin**: pkg/plugins/trigger/ - Handles `ok-to-test` label and triggers Prow-managed test jobs
  - pkg/plugins/trigger/trigger.go - Main plugin logic
  - pkg/plugins/trigger/pull-request.go:127-151 - PullRequest event handler for `ok-to-test` label
- **GitHub Client**: pkg/github/client.go - GitHub API wrapper with workflow-related methods
  - Lines 2080-2155: Workflow trigger methods (TriggerGitHubWorkflow, TriggerFailedGitHubWorkflow, GetFailedActionRunsByHeadBranch)
- **Hook Server**: pkg/hook/server.go - Webhook receiver and event dispatcher
- **Plugin Framework**: pkg/plugins/plugins.go - Plugin registration system

**Architecture Overview**:

Prow uses a plugin-based architecture where:
1. GitHub webhooks are received by the hook server (pkg/hook/server.go)
2. Events are demuxed to registered plugin handlers based on event type
3. Plugins register handlers for specific events (PullRequestEvent, IssueEvent, etc.)
4. Each handler receives a PluginAgent with access to GitHub API client and configuration
5. Handlers can call GitHub API methods to perform actions (add labels, post comments, etc.)

The `ok-to-test` label is currently processed by the trigger plugin, which:
- Watches for PullRequestActionLabeled events where label.name == "ok-to-test"
- Checks if the label was added by a trusted user (org member, collaborator, or GitHub app)
- Triggers Prow-managed presubmit jobs for the PR
- **Does NOT** interact with GitHub Actions/Workflows at all

**Key Code Paths**:

1. **Label Event Reception**: pkg/hook/server.go:77-183 (demuxEvent)
   - Receives webhook → validates signature → dispatches to event handler

2. **Pull Request Event Handling**: pkg/hook/events.go:164-192 (handlePullRequestEvent)
   - Creates PluginAgent → calls registered PullRequestHandlers

3. **Trigger Plugin Processing**: pkg/plugins/trigger/pull-request.go:127-151
   - Checks for `ok-to-test` label → verifies trusted user → triggers Prow jobs
   - **Gap**: No workflow approval logic

4. **GitHub Workflow API Methods**: pkg/github/client.go
   - GetFailedActionRunsByHeadBranch (lines 2080-2120): Lists workflow runs with failed status
   - TriggerGitHubWorkflow (lines 2122-2137): Re-runs entire workflow
   - TriggerFailedGitHubWorkflow (lines 2139-2155): Re-runs only failed jobs

**Data Flow**:

```
GitHub → Webhook → Hook Server → Event Demux → Pull Request Handler
                                                         ↓
                                               Trigger Plugin (ok-to-test)
                                                         ↓
                                               Trigger Prow Jobs

                                               [MISSING: Approve GitHub Workflows]
```

#### Related Code

**Dependencies**:
- pkg/labels/labels.go:47 - Label constant: `OkToTest = "ok-to-test"`
- pkg/github/types.go:222-236 - PullRequestEvent struct
- pkg/github/types.go:1614-1635 - WorkflowRun struct (contains ID, Status, Conclusion, etc.)
- pkg/plugins/config.go:489-514 - Trigger plugin configuration

**Similar Functionality** (patterns to follow):

1. **Approve Plugin** (pkg/plugins/approve/)
   - Registers PullRequestHandler and GenericCommentHandler
   - Watches for approval commands/labels
   - Calls GitHub API to add/remove labels and post comments
   - **Pattern**: Event-driven API calls based on label changes

2. **LGTM Plugin** (pkg/plugins/lgtm/)
   - Watches for LGTM approvals
   - Adds/removes `lgtm` label via GitHub API
   - **Pattern**: Similar label-based workflow

3. **Label Plugin** (pkg/plugins/label/)
   - Watches for `/label` commands and label events
   - Calls GitHubClient.AddLabel() and RemoveLabel()
   - **Pattern**: Direct GitHub API interaction

**Plugin Registration Pattern**:
```go
func init() {
    plugins.RegisterPullRequestHandler(PluginName, handlePullRequest, helpProvider)
}
```

All plugins are imported in pkg/hook/plugin-imports/plugin-imports.go to ensure init() functions run.

#### Test Coverage

**Existing Tests**:
- pkg/github/client_test.go:384-473 - TestGetFailedActionRunsByHeadBranch
  - Tests filtering workflow runs by status and conclusion
  - Validates API response parsing
- pkg/plugins/trigger/trigger_test.go - Tests for trigger plugin logic
  - Tests `ok-to-test` label handling
  - Tests trust verification
  - Tests Prow job triggering

**Test Gaps**:
- No tests for workflow approval in response to `ok-to-test` label
- No integration tests for trigger → workflow approval flow
- Missing tests for configuration of workflow approval feature

**Test Patterns to Follow**:
- GitHub client tests use mock HTTP servers
- Plugin tests use fake GitHub clients
- Event handler tests create synthetic webhook events

#### Documentation Review

**Code Comments**:
- pkg/plugins/trigger/pull-request.go:129-133 - Comments explain that bot-added labels are skipped to avoid duplicate triggers
- pkg/github/client.go:2082-2084 - Comments explain workflow run filtering logic
- pkg/plugins/config.go:489-514 - Configuration struct has doc comments for each field

**GitHub API Reference**:
- Issue mentions: https://docs.github.com/en/rest/reference/actions#approve-a-workflow-run-for-a-fork-pull-request
- This is the API endpoint needed for approving workflow runs from forks

**Known Limitations**:
- Current trigger plugin only handles Prow-managed jobs, not GitHub Actions
- No existing integration between Prow and GitHub Actions workflow approvals

#### Root Cause Analysis

**Primary Cause**:

This is a **feature gap**, not a bug. The trigger plugin was designed before GitHub Actions became widely used, and it only handles Prow-managed test jobs. When GitHub introduced workflow approval requirements for fork PRs, this created a gap where maintainers must:
1. Add `ok-to-test` label (for Prow jobs) → works automatically
2. Manually approve GitHub workflows via UI → requires manual intervention

**Contributing Factors**:

1. **Architectural Separation**: Prow jobs and GitHub Actions are separate testing systems
2. **GitHub API Evolution**: Workflow approval API was added after Prow's trigger plugin was created
3. **No Configuration Hook**: Trigger plugin doesn't have a mechanism to call additional approval APIs
4. **Plugin Scope**: Each plugin focuses on one responsibility; workflow approval doesn't fit cleanly into existing plugins

**Why Feature Doesn't Exist**:
- Prow predates widespread GitHub Actions usage
- Trigger plugin focused solely on Prow job management
- No plugin has combined Prow label events with GitHub Actions API calls
- Community workarounds emerged before internal Prow support was added

#### Proposed Solutions

#### Approach 1: Extend Trigger Plugin with Workflow Approval

**Description**: Add workflow approval functionality directly to the existing trigger plugin. When `ok-to-test` label is added, the plugin would:
1. Trigger Prow jobs (existing behavior)
2. List pending workflow runs for the PR's head branch/SHA
3. Call GitHub's workflow approval API for each pending run

**Pros**:
- Single source of truth for "ok-to-test" logic
- Minimal architectural changes
- Reuses existing trust verification logic
- No new plugin to maintain

**Cons**:
- Increases trigger plugin complexity
- Couples Prow job triggering with GitHub Actions approval
- May be confusing if users only want one behavior
- Harder to disable workflow approval independently

**Affected Components**:
- pkg/plugins/trigger/pull-request.go - Add workflow approval logic after job triggering
- pkg/plugins/config.go - Add configuration flag to enable/disable workflow approval
- pkg/github/client.go - Potentially add new method for listing pending workflows (vs failed)

**Complexity**: Medium

**Backwards Compatibility**:
- Fully compatible - new behavior only activates when configured
- Requires opt-in configuration flag to enable workflow approval

#### Approach 2: Create New "Approve Workflows" Plugin

**Description**: Create a standalone plugin that focuses solely on GitHub Actions workflow approval. The plugin would:
1. Register a PullRequestHandler for label events
2. Watch for `ok-to-test` label additions
3. List pending workflow runs requiring approval
4. Call GitHub's workflow approval API

**Pros**:
- Separation of concerns - distinct from Prow job triggering
- Can be enabled/disabled independently
- Clearer responsibility boundary
- Could support additional labels beyond `ok-to-test`
- Follows plugin architecture principles

**Cons**:
- Duplicates trust verification logic from trigger plugin
- Another plugin to maintain and document
- Slight increase in webhook processing overhead
- Two plugins responding to same label event

**Affected Components**:
- Create new: pkg/plugins/approve-workflow/ (new plugin directory)
- pkg/plugins/config.go - Add ApproveWorkflow configuration struct
- pkg/hook/plugin-imports/plugin-imports.go - Import new plugin
- pkg/github/client.go - Add ApproveWorkflowRun() method for the actual approval API call

**Complexity**: Medium

**Backwards Compatibility**:
- Fully compatible - opt-in plugin
- No impact on existing functionality

#### Approach 3: Configuration-Based Trigger Extension

**Description**: Keep trigger plugin as-is but make it extensible via configuration. Add a config option like `TriggerGitHubWorkflows: true` (already exists in config!) that enables workflow approval as an additional action.

**Pros**:
- Leverages existing configuration structure
- Single plugin handles related "trigger testing" actions
- Simple on/off switch per repository
- Config field already exists at pkg/plugins/config.go:507

**Cons**:
- Still couples two different systems
- Less flexibility for advanced workflow approval scenarios
- May not handle all workflow approval edge cases

**Affected Components**:
- pkg/plugins/trigger/pull-request.go - Check config.TriggerGitHubWorkflows flag
- Implementation similar to Approach 1 but gated by existing config field

**Complexity**: Low-Medium

**Backwards Compatibility**:
- Fully compatible - config field exists but isn't used for approval
- Defaults to false (no behavior change)

#### Recommendation

**Preferred Approach**: **Approach 2 (Create New Plugin)**

**Rationale**:

1. **Separation of Concerns**: Prow job triggering and GitHub Actions approval are fundamentally different operations on different systems. A dedicated plugin makes this boundary explicit.

2. **Flexibility**: A standalone plugin can evolve independently:
   - Support different labels (not just `ok-to-test`)
   - Implement more sophisticated approval logic
   - Handle workflow-specific configuration
   - Potentially support workflow re-triggering, not just approval

3. **Clarity**: Users can see exactly which plugins are enabled and what each does. Mixing GitHub Actions logic into the Prow trigger plugin could be confusing.

4. **Maintainability**: Smaller, focused plugins are easier to test, debug, and maintain than monolithic plugins.

5. **Current Question in Issue**: The contributor asked about opt-in vs default. A separate plugin makes opt-in natural - repos enable it in their plugin configuration.

**Key Implementation Considerations**:

1. **GitHub API Method Needed**: The current codebase has methods for re-running workflows but NOT for approving pending workflows. Need to add:
   ```go
   func (c *client) ApproveWorkflowRun(org, repo string, runID int) error
   ```
   This should POST to `/repos/{org}/{repo}/actions/runs/{runID}/approve`

2. **Listing Pending Workflows**: Current method `GetFailedActionRunsByHeadBranch` filters for failed runs. Need similar method for pending runs awaiting approval. May need to check workflow run status for `waiting` or `action_required`.

3. **Trust Verification**: Plugin should verify the label was added by a trusted user (same logic as trigger plugin). Can reuse trust checking patterns.

4. **Configuration Structure**:
   ```go
   type ApproveWorkflow struct {
       Repos []string  // org/repos to enable
       // Future: additional config like which labels to watch
   }
   ```

5. **Error Handling**: What if approval API fails? Should plugin:
   - Log error and continue (best effort)
   - Post comment to PR indicating failure
   - Retry logic?

**Testing Requirements**:
- Unit tests for label event handling
- Mock GitHub API client tests for approval calls
- Test trust verification logic
- Test configuration parsing
- Integration test for full label → approval flow

**Documentation Requirements**:
- Plugin help text explaining what it does
- Configuration example in plugin config documentation
- Comment in code explaining GitHub API endpoint used
- Update issue with link to implementation

## Next Steps

1. Run **assess-effort** to determine implementation complexity and appropriate labels
2. Run **augment** to add technical context to the issue for the contributor working on it
3. Run **brief** to walk through findings (optional)
4. Run **wrapup** to post augmentation comment and finalize triage

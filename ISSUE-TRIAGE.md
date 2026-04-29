# Triage for Issue #693

**Status**: In Progress
**Created**: 2026-04-29

## Issue Information

- **Issue Number**: #693
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/693

## Initial Validation

**Assessment**: LEGITIMATE

### Analysis

This is a feature request to extend Prow's `/retest` command to trigger Netlify preview rebuilds for repositories that use Netlify (specifically k/website and k/contributor-site). The issue was originally filed as kubernetes/test-infra#35103 by Tim Bannister (lmktfy, SIG Docs chair) and transferred to the Prow repo by Caesarsage because `/retest` is a Prow plugin.

**Issue Category**: Feature Request

**Repository Scope Check**:
- Component mentioned: `/retest` command in the trigger plugin
- Exists in this repo: Yes — `pkg/plugins/trigger/generic-comment.go`, `pkg/pjutil/filter.go`
- Relevant code paths: `pkg/plugins/trigger/`, `pkg/pjutil/filter.go` (RetestRe, RetestFilter)
- No existing Netlify integration exists in the codebase

**Information Completeness**:
- Sufficient detail provided: Partially
- Missing information:
  - No concrete Netlify API endpoints identified for triggering rebuilds
  - No design proposal for how credentials would be managed
  - BenTheElder raised security concerns about handing Netlify creds to presubmits (arbitrary user code)
  - lmktfy suggested only a "trigger rebuilds" token would be needed

### Key Context from Original Issue (test-infra#35103)

- BenTheElder (Prow maintainer) raised concerns about security: handing Netlify credentials to presubmits that may run arbitrary user code
- BenTheElder noted Netlify exposes deploy triggers but "that's not quite it" — the exact API mechanism is unclear
- lmktfy believes only a rebuild-trigger token would be needed
- The issue has been bouncing between stale/rotten lifecycle stages since July 2025
- Caesarsage self-assigned and opened this tracking issue on 2026-04-28

### Recommendation

Keep open and continue triage. This is a legitimate feature request for the Prow trigger plugin. However, it has significant open questions around:

1. **Feasibility**: Does Netlify's API support triggering deploy preview rebuilds for a specific PR? (Deploy triggers are for production deploys, not PR previews)
2. **Security**: How to safely provide Netlify credentials without exposing them to arbitrary presubmit code
3. **Architecture**: Whether this belongs as an extension to the trigger plugin, a new plugin, or an external webhook

**Suggested Action**:
- Keep open and continue triage
- Research feasibility of Netlify API for PR preview rebuilds
- Assess effort and architectural approach

## Code Research

### Current Implementation

**Primary Components**:
- Trigger plugin: `pkg/plugins/trigger/` — handles `/retest`, `/test`, `/ok-to-test` commands
- Generic comment handler: `pkg/plugins/trigger/generic-comment.go` — entry point for comment-based triggers
- Filter logic: `pkg/pjutil/filter.go` — regex matching and job filtering for `/retest`
- ProwJob creation: `pkg/pjutil/pjutil.go` — creates ProwJob Kubernetes resources

**Architecture Overview**:
When a user posts `/retest`, the trigger plugin's `handleGenericComment()` receives the event, matches it against `RetestRe` regex, identifies failed ProwJobs via GitHub status contexts, and re-creates them as new ProwJob Kubernetes resources.

**Key Code Paths**:
1. Comment handling: `pkg/plugins/trigger/generic-comment.go:38` — `handleGenericComment()` entry point
2. Comment matching: `pkg/plugins/trigger/generic-comment.go:213-231` — `commentMatchesTrigger()` checks `/retest` regex
3. Job filtering: `pkg/plugins/trigger/generic-comment.go:254-274` — `FilterPresubmits()` selects failed jobs
4. Failed context detection: `pkg/plugins/trigger/generic-comment.go:276-288` — `getContexts()` reads GitHub combined status
5. Job creation: `pkg/plugins/trigger/trigger.go:332-356` — `RunRequestedWithLabels()` creates ProwJobs

### Existing External CI Integration Pattern (GitHub Actions)

**Critical finding**: Prow already has a pattern for triggering external CI on `/retest`. The `TriggerGitHubWorkflows` configuration option (added in `pkg/plugins/config.go:510`) enables re-triggering failed GitHub Actions when `/retest` is issued.

**Implementation at `pkg/plugins/trigger/generic-comment.go:168-196`**:
- Checks if `trigger.TriggerGitHubWorkflows` is enabled
- On `/retest` or `/test all`, fetches failed GitHub Action runs via `GetFailedActionRunsByHeadBranch()`
- Triggers each failed run via `TriggerFailedGitHubWorkflow()` in a goroutine
- Runs in parallel with ProwJob re-creation

This is the exact pattern that a Netlify integration would follow — extending the trigger plugin to call an external API alongside ProwJob retesting.

### Plugin Extensibility

**Plugin registration**: `pkg/plugins/plugins.go:176-180` — plugins register handlers via `RegisterGenericCommentHandler()`

**External plugin system**: `pkg/plugins/config.go:139-149` — `ExternalPlugin` struct allows forwarding GitHub events to external HTTP services. However, external plugins receive raw GitHub events, not parsed Prow commands — they'd need their own `/retest` matching logic.

**Agent structure**: `pkg/plugins/plugins.go:188-213` — plugins get access to GitHub client, Kubernetes client, config, etc.

### Netlify API Research

**Critical finding: Netlify does not support triggering deploy preview rebuilds via API.**

| Approach | Triggers build? | Proper deploy preview? | Updates GH PR status? |
|---|---|---|---|
| Build hook + `trigger_branch` | Yes | No (branch deploy only) | No |
| `POST /sites/{site_id}/builds` | Yes | No (production only) | No |
| `POST /sites/{site_id}/deploys` with branch | Partially | No | No |
| Push empty commit to PR branch | Yes | **Yes** | **Yes** |
| Re-deliver GitHub webhook | Yes | **Yes** | **Yes** |

Deploy previews are triggered exclusively by GitHub push webhook events. Netlify build hooks trigger "branch deploys" which are a different concept — they don't create the `deploy-preview-N` URL and don't update the GitHub PR check status.

**Netlify API token limitations**: Personal Access Tokens have no granular scoping — a PAT has full access equivalent to the user. Build hook URLs are safer (scoped to one site, build-only) but can't trigger deploy previews.

### Root Cause Analysis

**Primary Cause**:
This is not a bug — it's a feature gap. The `/retest` command only re-triggers ProwJobs and (optionally) GitHub Actions. There's no mechanism to trigger other external CI systems.

**Contributing Factors**:
1. Netlify's API does not expose a "rebuild deploy preview for PR X" endpoint
2. Deploy previews are tightly coupled to GitHub push webhooks
3. The only reliable way to trigger a deploy preview rebuild is to push a commit (or empty commit) to the PR branch

**Feasibility Assessment**:
The feature as requested (Prow calling Netlify API on `/retest`) is **not directly feasible** with Netlify's current API. The only workarounds involve pushing commits to PR branches, which raises its own concerns (Prow would need push access to contributor forks, it modifies git history, and it's a heavy-handed approach for a simple rebuild).

### Proposed Solutions

#### Approach 1: Push Empty Commit (Workaround)

**Description**: On `/retest`, detect Netlify-managed repos and push an empty commit to the PR branch to trigger a deploy preview rebuild via Netlify's GitHub webhook integration.

**Pros**:
- Only method that produces a true deploy preview with GitHub status updates
- Conceptually simple

**Cons**:
- Requires push access to contributor fork branches — may not be possible for cross-fork PRs
- Pollutes git history with empty commits
- Heavy-handed: triggers ALL CI, not just Netlify
- Security concern: Prow pushing to arbitrary forks
- Poor UX: unexpected commits appearing in PRs

**Complexity**: Medium
**Backwards Compatibility**: No impact on existing behavior (opt-in)

#### Approach 2: Netlify Build Hook Integration

**Description**: On `/retest`, call a configured Netlify build hook URL with `trigger_branch=<pr-branch>` to trigger a branch deploy.

**Pros**:
- Simple API call, no git manipulation
- Build hooks are safe (URL-as-secret, build-only, single-site scoped)
- Follows the `TriggerGitHubWorkflows` pattern in the trigger plugin

**Cons**:
- Triggers a **branch deploy**, NOT a deploy preview — different URL, no GitHub PR status update
- Does not solve the actual problem (PR check status remains failed)
- Contributors would need to know to check a different URL

**Complexity**: Low
**Backwards Compatibility**: No impact

#### Approach 3: External Plugin (Decoupled)

**Description**: Create a standalone external plugin service that receives `/retest` events from Prow via the ExternalPlugin mechanism and handles Netlify-specific logic independently.

**Pros**:
- Keeps Prow codebase clean of Netlify-specific code
- Can implement custom logic (e.g., re-delivering webhooks, or future Netlify API support)
- Decoupled evolution

**Cons**:
- Still faces the fundamental Netlify API limitation
- Additional infrastructure to deploy and maintain
- External plugins receive raw GitHub events, need their own command parsing

**Complexity**: High (new service)

#### Approach 4: Document the Workaround (Non-Code Solution)

**Description**: Instead of code changes, document the available workarounds for triggering Netlify rebuilds: push an empty commit, or have a repo admin re-deliver the webhook from GitHub settings.

**Pros**:
- Zero code changes
- Addresses the immediate user need
- Honest about the limitation

**Cons**:
- Doesn't improve the UX
- Doesn't solve the original problem
- The issue asked specifically for Prow integration

**Complexity**: None

#### Recommendation

**Preferred Approach**: Approach 2 (Build Hook Integration) is the most technically sound option within Prow, BUT it doesn't fully solve the problem because branch deploys don't update GitHub PR status checks. This needs to be communicated clearly in the issue.

**Key Implementation Considerations**:
1. The feature as originally requested is **not fully achievable** due to Netlify API limitations
2. The closest viable Prow change (build hooks) produces branch deploys, not deploy previews
3. The real solution requires changes on Netlify's side (API for deploy preview rebuilds)
4. BenTheElder's security concerns are partially addressed by build hooks (no full-access token needed), but the fundamental limitation remains
5. The `TriggerGitHubWorkflows` pattern at `generic-comment.go:168-196` provides a clean template if/when Netlify adds proper API support

## Next Steps

- Assess effort level given the feasibility constraints
- Augment the issue with technical findings about Netlify API limitations

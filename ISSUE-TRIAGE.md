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

## Effort Assessment

**Effort Level**: 4 - Very Large / Near Impossible (as requested); 2 - Moderate (partial build hook solution)

### Summary

The feature as originally requested — `/retest` triggering Netlify deploy preview rebuilds with proper GitHub PR status updates — is **not achievable** due to Netlify API limitations. Deploy previews can only be triggered by GitHub push webhooks; no external API exists for this. The closest viable Prow change (build hook integration) is moderate effort but produces branch deploys, not deploy previews, and doesn't update GitHub PR status — meaning it doesn't solve the actual problem.

### Factor Analysis

#### Scope of Changes
- **Assessment**: Small (for build hook approach)
- **Details**: ~2-3 files modified (`pkg/plugins/trigger/generic-comment.go`, `pkg/plugins/config.go`, tests), ~50-100 LOC following the existing `TriggerGitHubWorkflows` pattern
- **Level Indication**: 1-2

#### Complexity
- **Assessment**: Simple (for the Prow code change itself)
- **Details**: The code pattern already exists in the GitHub Actions integration. Adding a build hook call is straightforward. The complexity is in the external system limitation.
- **Level Indication**: 1-2

#### Required Expertise
- **Assessment**: Moderate
- **Details**: Need understanding of trigger plugin architecture and Netlify API specifics. Contributor must understand the distinction between deploy previews and branch deploys.
- **Level Indication**: 2-3

#### Clarity and Certainty
- **Assessment**: Significant uncertainty
- **Details**: The fundamental problem (Netlify lacks an API for deploy preview rebuilds) means no Prow code change can fully solve this. The issue as written assumes such an API exists. Open questions about what "partial solution" is acceptable.
- **Level Indication**: 3-4

#### Testing Requirements
- **Assessment**: Moderate
- **Details**: Unit tests following existing trigger plugin test patterns. Integration testing would require a Netlify-connected repo.
- **Level Indication**: 2-3

#### Backwards Compatibility
- **Assessment**: Fully compatible
- **Details**: Any integration would be opt-in via configuration, no impact on existing deployments
- **Level Indication**: 1-2

#### Architectural Alignment
- **Assessment**: Good fit (partial solution follows existing pattern)
- **Details**: The `TriggerGitHubWorkflows` pattern is the exact template. Adding another external CI trigger follows established architecture. However, adding Netlify-specific code to Prow raises questions about vendor-specific integrations.
- **Level Indication**: 2-3

#### External Dependencies
- **Assessment**: Blocking
- **Details**: Netlify's API does not support triggering deploy preview rebuilds. This is the critical blocker. The feature cannot be fully implemented without Netlify adding this capability.
- **Level Indication**: 4

### Recommended Labels

- [x] `kind/feature`: New feature request for external CI integration
- [x] `area/plugins`: Affects the trigger plugin
- [ ] `good-first-issue`: External API limitation makes this deceptively complex
- [ ] `help-wanted`: Cannot be fully solved without Netlify API changes

### Guidance for Contributors

**For Level 4 (as requested — full deploy preview rebuild)**:
- This is blocked by external API limitations, not Prow code complexity
- The Netlify API does not expose an endpoint to trigger deploy preview rebuilds
- A full solution requires Netlify to add this capability
- Alternative: Advocate for Netlify API improvements, then implement when available

**For Level 2 (partial — build hook branch deploys)**:
- Follow the `TriggerGitHubWorkflows` pattern in `pkg/plugins/trigger/generic-comment.go:168-196`
- Add a `TriggerNetlifyBuildHooks` config option to `Trigger` struct
- Call configured build hook URLs with `trigger_branch=<pr-branch>` on `/retest`
- Caveat: This produces branch deploys, not deploy previews

### Caveats and Considerations

1. The issue author (Caesarsage) has self-assigned and seems eager to work on this — the triage should clearly communicate the API limitation to avoid wasted effort
2. The original issue author (lmktfy/Tim Bannister) suggested `netlify deploy --production branch=main --deploy-preview=42 --trigger` — but this CLI command doesn't actually exist in its described form
3. BenTheElder's security concerns are valid but partially addressable with build hooks (no full-access token needed)
4. If Netlify adds a deploy preview rebuild API in the future, the Prow integration would be straightforward following the GitHub Actions pattern

## Proposed Issue Augmentation

### Title Change

- **No change needed**: Current title "Integrate with Netlify for /retest Prow command" is clear and specific enough.

### Proposed GitHub Comment

```
## Netlify API Limitation

After researching this, the main blocker is on Netlify's side rather than Prow's: **Netlify does not expose an API to trigger deploy preview rebuilds for a specific PR.** Deploy previews are triggered exclusively by GitHub push webhook events. The available alternatives each have significant limitations:

- **Build hooks** (`POST https://api.netlify.com/build_hooks/{id}?trigger_branch=<branch>`) trigger *branch deploys*, not deploy previews. These don't create the `deploy-preview-N` URL and don't update GitHub PR status checks — so the failed check that prompted `/retest` would remain failed.
- **Netlify API tokens** (PATs) have no granular scoping — they grant full account access, which makes @BenTheElder's security concern from the [original issue](https://github.com/kubernetes/test-infra/issues/35103) even more relevant.
- **Pushing an empty commit** to the PR branch is the only method that triggers a true deploy preview rebuild with proper GitHub status updates, but this requires push access to contributor forks and pollutes git history.

## Prow Architecture Context

Prow already has a pattern for re-triggering external CI on `/retest`: the `TriggerGitHubWorkflows` option in the trigger plugin config (`pkg/plugins/trigger/generic-comment.go:168-196`) fetches failed GitHub Action runs and re-triggers them via the GitHub API. If Netlify were to add an API for deploy preview rebuilds, the Prow integration would follow this exact pattern and be straightforward to implement.

Until then, the available workaround for contributors is pushing an empty commit (`git commit --allow-empty -m "retrigger" && git push`) or asking a repo admin to re-deliver the GitHub webhook from the repository's webhook settings.

/area plugins
/kind feature
```

### Rationale

**What's being added**:
- Netlify API limitation analysis — the issue assumes an API exists but it doesn't
- Security concern details — PATs lack granular scoping, reinforcing BenTheElder's concern
- Prow architecture context — the `TriggerGitHubWorkflows` pattern shows this would be easy IF the API existed
- Practical workarounds — so users aren't stuck while waiting for Netlify API changes

**Why these labels**:
- `/area plugins`: The trigger plugin in `pkg/plugins/trigger/` is the affected component
- `/kind feature`: This is a new feature request, not a bug

**What's NOT included**:
- No `/good-first-issue` or `/help-wanted`: The feature is blocked by external API limitations (Level 4). Adding difficulty labels would invite contributors to work on something that can't be fully solved.
- No `/retitle`: Current title is adequate
- No `/priority`: This is a nice-to-have, not blocking functionality

## Next Steps

- Brief maintainer on findings
- Wrap up triage

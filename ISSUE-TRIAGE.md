# Triage for Issue #391

**Status**: In Progress
**Created**: 2026-04-04

## Issue Information

- **Issue Number**: #391
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/391

## Initial Validation

**Assessment**: LEGITIMATE

### Analysis

The issue requests a new configuration option for the `assign` plugin that would restrict `/assign` commands to org members only. The motivation is clear: during GSoC periods, non-org participants do "drive-by assigns" on good-first-issues, claiming them without the intent or ability to work on them.

**Issue Category**: Feature Request

**Repository Scope Check**:
- Component mentioned: `assign` plugin (`pkg/plugins/assign/assign.go`)
- Exists in this repo: Yes
- Relevant code paths:
  - `pkg/plugins/assign/assign.go` — the assign handler, specifically the `add` logic around line 148
  - `pkg/plugins/config.go` — plugin configuration definitions

**Information Completeness**:
- Sufficient detail provided: Yes
- The author (Daniel Hiller, KubeVirt contributor) provides:
  - Clear use case with real-world example
  - Specific proposed solution (`onlyOrgMembers` bool config per repo)
  - Links to relevant code locations
  - Context from maintainer discussion (petr-muller) about default behavior

### Recommendation

Keep open and continue triage. This is a well-defined, legitimate feature request for the assign plugin. The issue author is a known contributor and the feature addresses a real operational pain point. A Prow maintainer (petr-muller) has already engaged with the issue, validating its relevance.

**Suggested Action**:
- Keep open and continue triage
- The issue has been reopened by the author after bot auto-close, indicating continued interest
- A maintainer has kept it alive by removing lifecycle/stale labels multiple times

## Code Research

### Current Implementation

**Primary Components**:
- Assign plugin handler: `pkg/plugins/assign/assign.go` — processes `/assign`, `/unassign`, `/cc`, `/uncc` commands
- Plugin configuration: `pkg/plugins/config.go` — holds all plugin-specific config (assign is currently NOT configurable, line 45)

**Architecture Overview**:
The assign plugin registers a `GenericCommentHandler` that triggers on issue/PR comments. When a `/assign @user` comment is created, it:
1. `handleGenericComment()` (line 76) checks for "created" action
2. Constructs a `handler` struct via `newAssignHandler()` (line 190) with `add = gc.AssignIssue`
3. `handle()` (line 113) parses the command regex, splits into toAdd/toRemove lists
4. Calls `h.add()` (line 149) which delegates directly to the GitHub API
5. GitHub API itself enforces permissions — if a user can't be assigned, it returns `MissingUsers` error

**Key Code Paths**:
1. Entry point: `assign.go:76-85` — `handleGenericComment()`
2. Main processing: `assign.go:113-165` — `handle()` function
3. Handler construction: `assign.go:190-206` — `newAssignHandler()`
4. Assignment delegation: `assign.go:147-163` — calls `h.add()` → `gc.AssignIssue()`

**Critical Observation**: The plugin currently does NO access control. It relies entirely on GitHub's own permission model for assignment failures. There is no org membership check.

### Related Code

**Org Membership Checking (existing patterns)**:
- `pkg/plugins/trigger/trigger.go:237-300` — `TrustedUser()` function checks `IsMember(org, user)` and optionally `IsCollaborator(org, repo, user)`
- `pkg/github/client.go` — `OrganizationClient` interface provides `IsMember(org, user string) (bool, error)`
- Used by: trigger plugin (`OnlyOrgMembers` bool flag), welcome plugin (trusted user check)

**Per-Repo Config Patterns (established in codebase)**:
- `Trigger` struct (config.go:486-514): Has `Repos []string`, `OnlyOrgMembers bool` — most similar to what's needed
- `Welcome` struct (config.go:771-781): Has `Repos []string`, `AlwaysPost bool`
- `Approve` struct: Has `Repos []string` with lookup function `ApproveFor()` (config.go:948-978)
- All use `[]string` Repos field supporting `"org"` or `"org/repo"` format
- All have lookup functions: search repo-level first, then org-level, then return defaults

**Similar Functionality**:
- Trigger plugin's `OnlyOrgMembers` is the closest analogue — same boolean concept, same lookup pattern needed

### Test Coverage

**Existing Tests**: `pkg/plugins/assign/assign_test.go`
- `fakeClient` struct simulates GitHub API behavior (lines 28-110)
- `TestAssignAndReview()` has 38 test cases (lines 158-453)
- Covers: basic assign/unassign, cc/uncc, multi-user, invalid users, 10-assignee limit, team assignments, error responses
- Coverage assessment: Good for current functionality, but no config-related tests exist

**Test Gaps**:
- No tests for org membership filtering (doesn't exist yet)
- No config-related test infrastructure for assign plugin

### Root Cause Analysis

**Primary Cause**: The assign plugin was designed without access control — it delegates all permission checking to the GitHub API. GitHub allows anyone who can comment on an issue to use the Prow `/assign` command, even if they are not org members.

**Contributing Factors**:
1. No configuration infrastructure exists for the assign plugin at all
2. No org membership check is performed before calling the GitHub API
3. The plugin was designed for Kubernetes-scale projects where self-assignment was encouraged

### Proposed Solutions

#### Approach 1: Per-Repo Config with OnlyOrgMembers Flag (Recommended)

**Description**: Add an `Assign` config struct with `Repos []string` and `OnlyOrgMembers bool`. Before calling `h.add()`, check each user in `toAdd` against `IsMember(org, user)`. Non-members get a friendly comment explaining they can't assign.

**Pros**:
- Follows established patterns (Trigger, Welcome, Approve plugins)
- Minimal code change — single check before existing `h.add()` call
- Per-repo granularity with org-level fallback
- Backwards compatible — default `false` preserves current behavior

**Cons**:
- Additional API call per assignment to check membership
- Only covers `/assign`, not `/cc` (reviewers) — but this is likely the desired scope

**Affected Components**:
- `pkg/plugins/config.go`: Add `Assign` struct + `AssignFor()` lookup function
- `pkg/plugins/assign/assign.go`: Accept config, check membership before `h.add()`
- `pkg/plugins/assign/assign_test.go`: Add config-aware test cases

**Complexity**: Low

**Backwards Compatibility**: Full — default `OnlyOrgMembers: false` preserves current behavior

#### Approach 2: Global Default with Opt-Out

**Description**: Same as Approach 1 but default `OnlyOrgMembers` to `true`, requiring explicit opt-out. As discussed in the issue by petr-muller.

**Pros**:
- More secure by default
- Prevents drive-by assigns everywhere without configuration

**Cons**:
- Breaking change for all existing deployments
- Requires communication and migration period
- May surprise users who rely on current open-assignment behavior

**Complexity**: Low (same code, different default)

**Backwards Compatibility**: Breaking — all deployments would need to opt out if they want open assigns

#### Recommendation

**Preferred Approach**: Approach 1 (Per-Repo Config, default off)

Start with opt-in behavior to avoid breaking existing deployments. The default can be flipped later after consulting with Kubernetes sig-contribex (as discussed in the issue). This matches how the Trigger plugin handles its `OnlyOrgMembers` flag.

**Key Implementation Considerations**:
1. Follow the Trigger plugin pattern for config struct and lookup
2. Use `IsMember()` from the existing GitHub client interface
3. Generate a user-friendly comment explaining why assignment was rejected
4. Consider whether `/cc` (reviewer requests) should also be gated
5. Config YAML would look like:
   ```yaml
   assign:
     - repos:
         - "kubevirt/kubevirt"
       only_org_members: true
   ```

**Testing Requirements**:
- Test org member can assign when `OnlyOrgMembers: true`
- Test non-org member is rejected when `OnlyOrgMembers: true`
- Test anyone can assign when `OnlyOrgMembers: false` (default)
- Test comment is generated explaining rejection

## Next Steps

(Action items will be added here)

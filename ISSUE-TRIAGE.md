# Triage for Issue #540

**Status**: In Progress
**Created**: 2026-02-20

## Issue Information

- **Issue Number**: #540
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/540

## Initial Validation

**Assessment**: LEGITIMATE

### Analysis

The issue reports that `status-reconciler` catastrophically retired all existing job results on open pull requests when its configuration loading failed. The git-sync sidecar supplying configuration encountered a transient fetch failure, leaving the config directory empty. The status-reconciler then loaded an empty job config and reconciled the world to a state of "no jobs exist", overwriting real CI results with false passing "Context retired without replacement" statuses.

This is a severe bug with significant blast radius: it can silently overwrite legitimate CI results, potentially allowing PRs to merge without proper testing.

**Issue Category**: Bug

**Repository Scope Check**:
- Component mentioned: status-reconciler
- Exists in this repo: Yes
- Relevant code paths:
  - `cmd/status-reconciler/main.go`
  - `pkg/statusreconciler/controller.go`
  - `pkg/statusreconciler/status.go`
  - `pkg/config/agent.go` (config loading)

**Information Completeness**:
- Sufficient detail provided: Yes
- The issue includes: detailed description of the failure mode, relevant log excerpts from git-sync and status-reconciler, root cause analysis, impact assessment, and proposed fix direction (refuse to actuate without provably good config, signal via metrics/liveness probe)

### Recommendation

Keep open and continue triage. This is a well-documented, high-severity bug report for the status-reconciler component. The issue was filed by a project member with deep knowledge of the failure scenario. The proposed fix direction (refuse to reconcile when config is known-bad) is sound and actionable.

**Suggested Action**:
- Keep open and continue triage

## Code Research

### Current Implementation

**Primary Components**:
- **statusController**: `pkg/statusreconciler/status.go` - Manages config agent lifecycle, subscribes to config deltas
- **Controller**: `pkg/statusreconciler/controller.go` - Reconciles GitHub statuses when config changes are detected
- **Config Agent**: `pkg/config/agent.go` - Loads and polls for config changes, broadcasts deltas to subscribers
- **Main entry**: `cmd/status-reconciler/main.go` - Creates controller, starts agent, runs reconciliation loop

**Architecture Overview**:
The status-reconciler watches for Prow configuration changes via the config agent. When the agent detects a config file modification (polling loop), it loads the new config, computes a delta (old vs new), and broadcasts it. The controller receives the delta and reconciles GitHub statuses by: (1) triggering newly added blocking presubmits, (2) retiring removed contexts, and (3) migrating renamed contexts.

**Key Code Paths**:
1. Config initialization: `pkg/statusreconciler/status.go:57-71` - Creates agent, loads saved state, subscribes to deltas
2. Run loop: `pkg/statusreconciler/controller.go:161-183` - Waits for config deltas, calls reconcile
3. Reconcile dispatcher: `pkg/statusreconciler/controller.go:185-209` - Routes changes to retire/trigger/migrate handlers
4. Removed presubmit detection: `pkg/statusreconciler/controller.go:405-434` - Compares old vs new config to find removed jobs
5. Context retirement: `pkg/statusreconciler/controller.go:291-319` - Retires contexts on all open PRs for removed jobs
6. Config agent polling: `pkg/config/agent.go:338-379` - Polls config files, loads changes, broadcasts deltas
7. Delta creation: `pkg/config/agent.go:401-424` - Creates delta from old and new config, broadcasts to subscribers

**Data Flow**:
1. Config agent polls config files for modification time changes (`agent.go:356-366`)
2. On change, calls `Load()` to parse config (`agent.go:368`)
3. On load error, logs and continues (old config preserved) (`agent.go:369-371`)
4. On load success, calls `Set(c)` which creates delta and broadcasts (`agent.go:372-374`, `401-424`)
5. Status controller receives delta via subscription channel (`controller.go:170`)
6. `removedPresubmits()` compares old vs new to find removed jobs (`controller.go:405-434`)
7. `retireRemovedContexts()` sets "Context retired without replacement" on all matching PRs (`controller.go:291-319`)

### Root Cause Analysis

**Primary Cause**:
When git-sync fails and the config directory becomes empty/corrupted, the config agent's `Load()` function can either:
- **Fail and return error**: Agent logs the error and keeps old config (safe path)
- **Succeed with empty/minimal config**: Agent calls `Set()` with the empty config, creating a delta that shows ALL presubmits as "removed" (catastrophic path)

The second scenario is what happened in the reported incident. The `removedPresubmits()` function at `controller.go:405-434` has **no validation** that the new config is non-empty or reasonable. It simply iterates all old presubmits and marks any not found in the new config as "removed". When the new config is empty, **all** presubmits are marked as removed.

**Contributing Factors**:
1. **No config sanity validation**: `removedPresubmits()` does not check if `new` is empty before treating it as authoritative
2. **No circuit breaker**: `retireRemovedContexts()` at `controller.go:291-319` has no protection against retiring "too many" contexts at once
3. **Config agent trusts Load() results unconditionally**: `agent.go:372-374` calls `Set(c)` on any successful load, even if the resulting config has zero presubmits
4. **No distinction between "intentional removal" and "config load failure"**: The delta mechanism cannot distinguish between a legitimate config change that removes all jobs and a corrupted/empty config

**Reproduction Conditions**:
- Git-sync sidecar fails to fetch config repository
- Config directory becomes empty or contains only partial/base config
- Config agent's `Load()` succeeds (returns `*Config` with empty job maps instead of error)
- Delta broadcast triggers mass retirement of all presubmits

### Related Code

**Dependencies**:
- `pkg/config`: Config loading, agent, delta mechanism
- `pkg/statusreconciler/migrator`: Actual GitHub status operations (retire, migrate)
- `pkg/github`: GitHub API client for creating statuses

**Similar Functionality**:
- Other Prow controllers that subscribe to config deltas face the same risk if they act destructively on "removed" items

### Test Coverage

**Existing Tests**:
- `pkg/statusreconciler/status_test.go`: Tests state load/save and initial config loading (lines 46-232)
- `pkg/statusreconciler/controller_test.go`: Tests `addedBlockingPresubmits()` (lines 35-251) and `removedPresubmits()` (lines 253-399)

**Test Gaps**:
- No test for config loading failure scenario
- No test for empty config being loaded after previously having content
- No test for the "retire all contexts when config becomes empty" catastrophic scenario
- No test for git-sync failure while controller is running
- No test validating circuit breaker or sanity checks (because none exist)

### Proposed Solutions

#### Approach 1: Config Sanity Validation in removedPresubmits()

**Description**: Add validation in `removedPresubmits()` and/or `reconcile()` to detect and refuse to act on suspicious deltas where the new config is empty or has drastically fewer jobs than the old config.

**Pros**:
- Directly prevents the catastrophic scenario
- Simple to implement
- Minimal changes to existing architecture

**Cons**:
- Heuristic-based (what threshold is "suspicious"?)
- Could block legitimate mass removal of jobs (rare but possible)
- Only protects status-reconciler, not other delta consumers

**Affected Components**:
- `pkg/statusreconciler/controller.go`: Add validation before retirement

**Complexity**: Low

**Backwards Compatibility**: No impact - adds safety checks only

#### Approach 2: Config Agent Validation (prevent empty config from being Set)

**Description**: Add validation in the config agent's `Set()` or polling loop to refuse to accept a config that has zero presubmits when the previous config had many. This protects all delta consumers, not just status-reconciler.

**Pros**:
- Protects all config delta consumers system-wide
- Prevents the bad delta from ever being created
- Centralized fix

**Cons**:
- May be too broad - some deployments may legitimately have zero presubmits
- Harder to define "valid" config universally
- Config agent is shared infrastructure, higher risk of unintended effects

**Affected Components**:
- `pkg/config/agent.go`: Add validation in Set() or polling loop

**Complexity**: Medium

**Backwards Compatibility**: Could affect deployments with legitimately empty configs

#### Approach 3: Liveness/Readiness Probe + Metrics (issue's suggested approach)

**Description**: When config loading fails, flip the liveness probe to unhealthy and expose metrics indicating the reconciler is not actuating. This causes Kubernetes to restart the pod, preventing stale empty config from causing retirement.

**Pros**:
- Leverages Kubernetes health check infrastructure
- Observable via metrics
- Follows cloud-native patterns

**Cons**:
- Doesn't prevent the issue if `Load()` succeeds with empty config (it's not a load failure per se)
- Only addresses the "load error" scenario, not the "load succeeds with empty config" scenario
- Pod restart loop if config remains unavailable

**Affected Components**:
- `cmd/status-reconciler/main.go`: Health endpoint logic
- `pkg/config/agent.go`: Error signaling

**Complexity**: Medium

**Backwards Compatibility**: Requires deployment config changes for health probes

#### Recommendation

**Preferred Approach**: Combination of Approach 1 and Approach 3

Approach 1 (config sanity validation) should be the primary fix because it directly prevents the catastrophic retirement regardless of how the empty config arrived. The validation should:
- Refuse to retire contexts if the new config has zero presubmits and the old config had any
- Log a clear error when this condition is detected
- Expose a metric for monitoring

Approach 3 (liveness probe) is complementary: it provides observability and auto-recovery when config loading is genuinely failing.

**Key Implementation Considerations**:
1. The validation must distinguish between "config became empty due to corruption" and "legitimate removal of all jobs for a repo"
2. A per-repo check may be better than a global check: if all presubmits for a specific org/repo disappear while others remain, that's suspicious
3. Adding a metric for "contexts retired" count would help with monitoring
4. Tests must cover the empty-config scenario

**Testing Requirements**:
- Test that empty new config does NOT cause retirement when old config had presubmits
- Test that legitimate removal of a single presubmit still works
- Test that metrics/logging fire when suspicious delta is detected

## Effort Assessment

**Effort Level**: 2 - Moderate (help-needed)

### Summary

The fix requires adding config sanity validation in the status-reconciler's reconciliation path and optionally adding health/metrics signaling. The problem is well-understood, the solution is clear, and the scope is contained to a few files, but it requires understanding the config agent delta mechanism and careful consideration of edge cases (legitimate vs corrupted empty config).

### Factor Analysis

#### Scope of Changes
- **Assessment**: Small-Moderate
- **Details**: Primary fix in `pkg/statusreconciler/controller.go` (validation in `removedPresubmits()` or `reconcile()`), plus tests. Optional additions in `cmd/status-reconciler/main.go` for health/metrics. Estimated 2-4 files, ~100-200 lines including tests.
- **Level Indication**: 1-2

#### Complexity
- **Assessment**: Moderate
- **Details**: The fix itself is straightforward (check if new config is empty before retiring), but choosing the right validation heuristic requires thought. Need to distinguish "all jobs removed due to corruption" from "legitimate removal of jobs for one repo". Per-repo vs global check is a design decision. The config agent delta mechanism needs to be understood.
- **Level Indication**: 2-3

#### Required Expertise
- **Assessment**: Moderate
- **Details**: Requires understanding of the config agent subscription/delta mechanism, the status-reconciler's reconciliation loop, and how presubmits are structured in config. Familiarity with Prow config patterns is helpful. Can be learned from reading the relevant code.
- **Level Indication**: 2-3

#### Clarity and Certainty
- **Assessment**: Well-defined
- **Details**: The problem is precisely described with log evidence. The solution direction (refuse to retire when config is empty/corrupted) is clear. The main design question is the exact validation heuristic, which is a bounded decision.
- **Level Indication**: 1-2

#### Testing Requirements
- **Assessment**: Moderate
- **Details**: Need to add test cases for the empty-config scenario in `controller_test.go`, following existing test patterns for `removedPresubmits()`. Existing test infrastructure is sufficient. No integration tests needed.
- **Level Indication**: 2-3

#### Backwards Compatibility
- **Assessment**: Fully compatible
- **Details**: The fix only prevents destructive actions that should never happen. No behavior change for legitimate config changes. No configuration changes required for the primary fix.
- **Level Indication**: 1-2

#### Architectural Alignment
- **Assessment**: Good fit
- **Details**: Adding validation before destructive operations fits naturally with Prow's patterns. The config agent delta mechanism is not being changed, only consumed more carefully. Health probe integration follows existing Prow patterns.
- **Level Indication**: 1-2

#### External Dependencies
- **Assessment**: None
- **Details**: No external API changes needed. All changes are internal to Prow's config and status-reconciler components.
- **Level Indication**: 1-3

### Recommended Labels

- [x] `kind/bug`: This is a bug fix for catastrophic behavior
- [x] `area/status-reconciler`: Already applied
- [x] `help-wanted`: Already applied; appropriate for a skilled contributor
- [ ] `good-first-issue`: Requires understanding of config agent delta mechanism, not ideal for first-timers

### Guidance for Contributors

**For Level 2 (Moderate)**:
- Suitable for contributors familiar with Go and willing to learn the config agent pattern
- Should review:
  - `pkg/statusreconciler/controller.go` (reconciliation logic, `removedPresubmits()`)
  - `pkg/config/agent.go` (how deltas are created and broadcast)
  - `pkg/statusreconciler/controller_test.go` (existing test patterns)
- Recommended approach: Add a validation check in `removedPresubmits()` or `reconcile()` that detects when the new config has zero presubmits but the old config had many, and logs an error + skips retirement in that case
- Consider adding a metric to track when this safety check fires

### Caveats and Considerations

- The issue already has `help-wanted` label, which aligns with the Level 2 assessment
- The optional liveness probe / metrics additions could push this toward Level 3 if pursued as part of the same change, but the core safety fix is Level 2
- A broader fix at the config agent level (Approach 2 from research) would be Level 3 due to the shared infrastructure impact, but is not the recommended approach

## Proposed Issue Augmentation

### Title Change

- **No change needed**: Current title "status-reconciler started retiring whole world when its configuration became corrupted" is clear, specific, mentions the component, and accurately describes the catastrophic behavior.

### Proposed GitHub Comment

```
## Code-Level Root Cause

The vulnerability is in the `removedPresubmits()` function (`pkg/statusreconciler/controller.go:405-434`). When a config delta arrives, this function iterates all presubmits in the **old** config and marks any not found in the **new** config as "removed". There is no validation that the new config is non-empty or reasonable. When `Load()` succeeds with an empty job config (e.g. because git-sync left the config directory empty rather than failing outright), the config agent at `pkg/config/agent.go:372-374` calls `Set()` unconditionally, creating a delta where every presubmit appears "removed". The downstream `retireRemovedContexts()` at `controller.go:291-319` then retires all of them without any circuit breaker.

## Suggested Fix Approach

The primary fix should add config sanity validation before retirement: if the new config has zero presubmits but the old config had many, refuse to retire and log an error. This check belongs in `removedPresubmits()` or `reconcile()`. A per-repo check (detecting when all presubmits for a given org/repo disappear while others remain) would be more precise than a global check, since legitimate removal of all jobs for a single repo is plausible but all jobs for all repos disappearing is not. Adding a metric for when this safety check fires would help with monitoring. The liveness probe approach mentioned in the issue is complementary but doesn't cover the case where `Load()` succeeds with empty config (which is what happened here). Existing test patterns in `controller_test.go` for `removedPresubmits()` can be extended to cover the empty-config scenario.
```

### Rationale

**What's being added**:
- Specific code locations where the vulnerability exists (file:line references to `removedPresubmits()`, `retireRemovedContexts()`, config agent `Set()`)
- The exact mechanism chain: `Load()` succeeds with empty config → `Set()` broadcasts delta unconditionally → `removedPresubmits()` marks everything as removed → `retireRemovedContexts()` retires without circuit breaker
- Concrete fix approach with per-repo vs global check guidance, and pointer to existing test patterns
- Note that the liveness probe approach alone is insufficient (doesn't cover successful-but-empty Load)

**Why these labels**:
- All appropriate labels (`kind/bug`, `area/status-reconciler`, `help-wanted`) are already applied

**What's NOT included**:
- No `/retitle`: current title is already clear and specific
- No label commands: all appropriate labels already applied
- No `/priority`: while this is severe, it's a latent vulnerability rather than an active blocker, and adding priority might imply urgency beyond the existing `help-wanted` invitation
- No `/good-first-issue`: this requires understanding the config agent delta mechanism, making it unsuitable for first-time contributors

## Next Steps

(Action items will be added here)

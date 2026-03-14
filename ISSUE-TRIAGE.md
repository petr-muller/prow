# Triage for Issue #477

**Status**: In Progress
**Created**: 2026-03-14

## Issue Information

- **Issue Number**: #477
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/477
- **Title**: branchprotector: excluded branches retain existing protection instead of being removed
- **State**: CLOSED (auto-closed by lifecycle bot, not resolved)
- **Author**: kaovilai

## Initial Validation

**Assessment**: LEGITIMATE

### Analysis

The issue describes a real bug in `cmd/branchprotector/protect.go`. When branches are added to the `exclude` list in branchprotector configuration, existing GitHub branch protection is not removed from those branches. The tool only stops applying new protection rules.

**Issue Category**: Bug

**Repository Scope Check**:
- Component mentioned: branchprotector (`cmd/branchprotector`)
- Exists in this repo: Yes
- Relevant code paths: `cmd/branchprotector/protect.go` lines 341-343 (exclusion filter), lines 182-188 (removal logic)

**Information Completeness**:
- Sufficient detail provided: Yes
- The issue includes: clear description, impact with error logs, root cause analysis with exact line numbers, workaround, and even a suggested code change in the first comment

### Code Verification

The bug is confirmed by reading the code:

1. In `UpdateRepo()` (line 341-343), excluded branches hit `continue` and are never added to the `branches` map:
   ```go
   } else if !ok && branchExclusions != nil && branchExclusions.MatchString(b.Name) {
       logrus.Infof("%s/%s=%s: excluded", orgName, repoName, b.Name)
       continue  // Branch skipped entirely
   }
   ```

2. The removal mechanism in `configureBranches()` (line 184-188) triggers when `Request` is `nil`, but excluded branches never reach `UpdateBranch()` at all, so no `requirements` with `Request: nil` is ever sent for them.

3. The issue was auto-closed by the k8s lifecycle bot (stale -> rotten -> closed), NOT because it was resolved. The author even tried to keep it alive by removing lifecycle/rotten once.

### Recommendation

Keep open (reopen) and continue triage. This is a valid, well-documented bug with a clear root cause and suggested fix. The auto-closure by the lifecycle bot should not be treated as resolution.

**Suggested Action**:
- Reopen the issue
- Continue triage with research, effort assessment, and augmentation

## Code Research

### Current Implementation

**Primary Components**:
- `cmd/branchprotector/protect.go` — Main branchprotector logic: iterates orgs/repos/branches, builds requirements, sends to update channel
- `cmd/branchprotector/request.go` — Converts `Policy` to `github.BranchProtectionRequest`
- `pkg/config/branch_protection.go` — Config types (`Policy`, `Org`, `Repo`, `Branch`), policy merging (`Apply`), branch resolution (`GetBranch`, `GetPolicy`)

**Architecture Overview**:
The branchprotector has a producer-consumer architecture:
1. **Producer** (`protect()` → `UpdateOrg()` → `UpdateRepo()` → `UpdateBranch()`): Iterates all configured orgs/repos/branches, computes desired state, and sends `requirements` to a channel
2. **Consumer** (`configureBranches()`): Reads from the channel and calls GitHub API — either `UpdateBranchProtection()` (when `Request` is non-nil) or `RemoveBranchProtection()` (when `Request` is nil)

**Key Code Paths**:
1. Branch filtering: `protect.go:328-346` — Fetches all branches from GitHub (two passes: all + protected-only), filters by Include/Exclude patterns
2. Branch processing: `protect.go:371-377` — For each branch in the filtered map, calls `UpdateBranch()`
3. Policy resolution: `protect.go:459` → `branch_protection.go:465` — Merges global/org/repo/branch policies to determine desired state
4. Protection decision: `protect.go:466-480` — If `Protect: false`, sets `req = nil`; if `Protect: true`, builds request
5. Comparison: `protect.go:500-508` — Fetches current GitHub state and compares; skips if already matching
6. Removal: `protect.go:184-188` — When `Request` is nil, calls `RemoveBranchProtection()`

**Data Flow for Excluded Branches (the bug)**:
```
GetBranches() → branch matches exclusion pattern → continue (SKIPPED)
                                                    ↓
                                         Branch never enters branches map
                                                    ↓
                                         UpdateBranch() never called
                                                    ↓
                                         No removal request sent
                                                    ↓
                                         GitHub protection stays in place
```

### Related Code

**Three protection-management semantics**:
1. **`Unmanaged: true`**: Skip entirely, don't touch GitHub state (protect.go:456-458)
2. **`Protect: false`**: Actively remove protection — sends `Request: nil` → `RemoveBranchProtection()` (protect.go:466-480)
3. **`Exclude` pattern match**: Currently behaves like `Unmanaged` (skips entirely) but should behave like `Protect: false` for branches that are currently protected

**Include/Exclude are mutually exclusive**: Validated in config parsing (commit `92d1ed377`). Cannot set both on the same policy level.

### Git History

- **Include feature added**: June 2021 by Mohamed chiheb Ben jemaa (commit `512e3c218`)
- **Exclude feature**: Predates Include, was already established
- **Mutual exclusivity validation**: Added 5 days after Include (commit `92d1ed377`)
- **Bug age**: ~5 years — the exclusion filtering was designed to control which branches to manage but never extended to handle cleanup of previously-managed branches

### Test Coverage

**Existing Tests** (`cmd/branchprotector/protect_test.go`, 2709 lines):
- `TestProtect` (line 287): 50+ sub-cases covering the main protect flow
- 3 exclusion tests:
  - "excluded branches are not protected" (lines 1098-1119): Basic exclude with `sk.*` pattern
  - "org and repo level branch exclusions are combined" (lines 1121-1143): Combined patterns
  - "explicitly specified branches are not affected by Exclude" (lines 1145-1167): Explicit branch overrides exclusion
- Removal tests in `TestConfigureBranches` (line 180): Well-tested — `Request: nil` → `RemoveBranchProtection()`

**Test Gaps**:
- No test for excluded branch that is currently protected on GitHub (the exact bug scenario)
- No tests at all for `Include` functionality (0 test cases)
- Test infrastructure (fakeClient) fully supports tracking deletions via `deleted` map

### Root Cause Analysis

**Primary Cause**:
In `UpdateRepo()` (protect.go:341-343), excluded branches hit `continue` and are never added to the `branches` map. This means they never reach `UpdateBranch()`, and no removal request is ever sent to the updates channel. The exclusion filter was designed as a "don't manage" gate but not as a "clean up previous management" gate.

**Contributing Factors**:
1. The branch filtering loop (lines 328-346) operates on the `branches` map which feeds into the processing loop (lines 371-377). Excluded branches are removed from consideration entirely, with no separate path for cleanup.
2. The two-pass branch fetching (lines 329-331, `onlyProtected=false` then `onlyProtected=true`) correctly identifies which branches are protected on GitHub, but this `Protected` field is never checked during the exclusion filter.
3. The `Unmanaged` feature (which intentionally leaves GitHub state alone) and `Exclude` (which should actively clean up) are semantically different but implemented with the same behavior (skip entirely).

**Reproduction Conditions**:
1. A branch must be protected on GitHub (either manually or by a previous branchprotector run without the exclusion)
2. The branch must then be added to the `exclude` list in the branchprotector config
3. Branchprotector runs and logs the branch as "excluded" but does not remove the existing protection

### Proposed Solutions

#### Approach 1: Direct Removal in Exclusion Filter

**Description**: In the exclusion check within `UpdateRepo()`, when a branch matches the exclusion pattern AND is currently protected (`b.Protected == true`), send a `requirements{Request: nil}` directly to the updates channel.

```go
// Conceptual change at protect.go:341-343
} else if !ok && branchExclusions != nil && branchExclusions.MatchString(b.Name) {
    logrus.Infof("%s/%s=%s: excluded", orgName, repoName, b.Name)
    if b.Protected {
        logrus.Infof("%s/%s=%s: excluded but protected, queuing for removal", orgName, repoName, b.Name)
        p.updates <- requirements{Org: orgName, Repo: repoName, Branch: b.Name, Request: nil}
    }
    continue
}
```

**Pros**:
- Minimal code change (3-4 lines added)
- Targeted fix — only affects excluded branches that are currently protected
- Does not disturb the existing processing flow for non-excluded branches
- The `b.Protected` check means we only send removal requests when needed (no unnecessary API calls)

**Cons**:
- Bypasses the `equalBranchProtections` comparison check in `UpdateBranch` (but this is acceptable since `b.Protected == true` already confirms protection exists)
- Sends directly to `p.updates` channel from `UpdateRepo`, which is a pattern not currently used (all other sends go through `UpdateBranch`)

**Affected Components**:
- `cmd/branchprotector/protect.go:UpdateRepo()` — Add removal logic in exclusion filter

**Complexity**: Low

**Backwards Compatibility**: Safe — previously excluded branches were silently ignored; now they'll have protection actively removed, which matches user expectation

#### Approach 2: Track Excluded Branches Separately

**Description**: Collect excluded-but-protected branches in a separate set, then process them for removal after the main branch processing loop.

```go
// Track excluded branches that need cleanup
excludedProtected := sets.New[string]()

// In the filtering loop:
} else if !ok && branchExclusions != nil && branchExclusions.MatchString(b.Name) {
    if b.Protected {
        excludedProtected.Insert(b.Name)
    }
    continue
}

// After the main processing loop (after line 377):
for bn := range excludedProtected {
    p.updates <- requirements{Org: orgName, Repo: repoName, Branch: bn, Request: nil}
}
```

**Pros**:
- Clear separation between filtering and removal
- Easy to understand and review
- Could log a summary of how many excluded branches needed cleanup

**Cons**:
- Slightly more code than Approach 1
- Adds a new processing phase that needs to be understood

**Complexity**: Low

**Backwards Compatibility**: Same as Approach 1 — safe

#### Recommendation

**Preferred Approach**: Approach 1 (Direct Removal in Exclusion Filter)

It's the simplest change with the smallest diff. The `b.Protected` flag from GitHub's API is reliable (it comes from the `onlyProtected=true` pass at line 329), so there's no need for the full comparison check that `UpdateBranch` performs.

**Key Implementation Considerations**:
1. The `Include` path (lines 338-340) has a similar gap: branches that don't match the include pattern but are currently protected won't have protection removed. However, the semantics of `include` ("only manage these branches") arguably mean "leave others alone", unlike `exclude` ("actively exclude these branches"). Fixing `include` is a separate discussion.
2. A new test case should be added to `TestProtect` covering the scenario: excluded branch that is currently protected → expect `Request: nil` in requirements.
3. The existing test infrastructure (`fakeClient.deleted` map) already supports verifying removal calls.

**Testing Requirements**:
- Add test case: "excluded branches that are currently protected should have protection removed"
- Setup: branches `master` (protected) and `skip` (protected), exclude pattern `sk.*`
- Expected: `master` gets protection applied, `skip` gets `Request: nil` (removal)
- Verify: `fakeClient.deleted` contains the excluded branch

## Effort Assessment

**Effort Level**: 1 - Easy (good-first-issue)

### Summary

This is a well-defined bug with a clear root cause, a straightforward fix (~5 lines of production code), and existing test infrastructure that directly supports verifying the fix. A new contributor could complete this with minimal Prow-specific knowledge.

### Factor Analysis

#### Scope of Changes
- **Assessment**: Small
- **Details**: 1 production file (`cmd/branchprotector/protect.go`, ~5 lines added), 1 test file (`cmd/branchprotector/protect_test.go`, ~30 lines for a new test case)
- **Level Indication**: 1

#### Complexity
- **Assessment**: Simple
- **Details**: Add a `b.Protected` check inside the existing exclusion filter and send a removal request. No concurrency concerns, no edge cases beyond the basic scenario. The removal mechanism already exists and is well-tested.
- **Level Indication**: 1

#### Required Expertise
- **Assessment**: Minimal
- **Details**: The fix is localized to one function (`UpdateRepo`). A contributor only needs to understand the branch filtering loop and the `requirements` channel. The existing code comments and log messages make the flow self-explanatory. No Prow-wide architectural knowledge needed.
- **Level Indication**: 1

#### Clarity and Certainty
- **Assessment**: Well-defined
- **Details**: Root cause is identified to exact line numbers. The issue author provided a code change suggestion. The desired behavior is unambiguous: excluded branches that are currently protected should have protection removed. No open design questions.
- **Level Indication**: 1

#### Testing Requirements
- **Assessment**: Simple
- **Details**: Add one test case to the existing `TestProtect` function following the exact pattern of the three existing exclusion tests (lines 1098-1167). The `fakeClient` mock already tracks deletions via its `deleted` map. The `startUnprotected` field controls initial protection state.
- **Level Indication**: 1

#### Backwards Compatibility
- **Assessment**: Minor behavior change (safe)
- **Details**: Previously, excluded branches with existing protection were silently left alone. After the fix, their protection will be actively removed. This matches user expectations — if you exclude a branch, you expect it to not be protected. The change only affects branches explicitly listed in `exclude` patterns, so it's opt-in via configuration.
- **Level Indication**: 1-2

#### Architectural Alignment
- **Assessment**: Perfect fit
- **Details**: Uses the existing `p.updates` channel and `requirements{Request: nil}` removal mechanism. Follows the same pattern used by `UpdateBranch` when `Protect: false`. No new abstractions or patterns introduced.
- **Level Indication**: 1

#### External Dependencies
- **Assessment**: None
- **Details**: Uses existing GitHub API calls (`RemoveBranchProtection`). The `b.Protected` flag comes from `GetBranches(onlyProtected=true)` which is already called. No new API calls needed.
- **Level Indication**: 1

### Recommended Labels

- [x] `good-first-issue`: Clear, well-defined, small scope, existing test patterns to follow
- [x] `kind/bug`: Fixing incorrect behavior in branch exclusion logic
- [x] `area/branchprotector`: Affects the branchprotector component
- [ ] `help-needed`: Too simple for this label — better suited as good-first-issue

### Guidance for Contributors

- Good starting point for new Prow contributors
- Suggested prerequisite knowledge: Basic Go, understanding of channels
- The fix is ~5 lines in `cmd/branchprotector/protect.go:341-343`
- Follow the existing exclusion test pattern in `protect_test.go:1098-1167`
- The `fakeClient` mock (protect_test.go:83-178) tracks `RemoveBranchProtection` calls in its `deleted` map
- The issue author's comment provides a helpful starting point for the code change

### Caveats and Considerations

- The `Include` path (lines 338-340) has a similar gap but with different semantics: "include" means "only manage these", so leaving non-included branches alone is arguably correct. Fixing `include` would be a separate issue if desired.
- The behavior change (actively removing protection from excluded branches) is technically a change in semantics. Deployments that rely on the current behavior (exclude = "don't touch") would be affected. However, this current behavior is clearly a bug, not a feature — the workaround in the issue confirms users expect exclusion to mean "unprotect".

## Next Steps

- [x] Research: Deep-dive into code paths and solution approaches
- [x] Assess effort: Determine complexity level
- [ ] Augment: Improve issue with triage findings
- [ ] Brief: Walk maintainer through findings
- [ ] Wrapup: Push and post comment

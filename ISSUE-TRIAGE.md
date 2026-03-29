# Triage for Issue #662

**Status**: In Progress
**Created**: 2026-03-28

## Issue Information

- **Issue Number**: #662
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/662

## Initial Validation

**Assessment**: LEGITIMATE

### Analysis

The issue requests migrating Prow's logging from `sirupsen/logrus` to Go's built-in `log/slog` package (available since Go 1.21). This is a valid enhancement request.

**Key facts**:
- logrus has been in maintenance mode since ~2020 (6+ years)
- Go's `log/slog` package is the standard structured logging solution since Go 1.21
- logrus is imported in approximately 250 Go files across the codebase
- slog is not used anywhere in the codebase currently
- Prow has a custom `pkg/logrusutil` wrapper package around logrus

**Issue Category**: Feature Request / Enhancement (dependency modernization)

**Repository Scope Check**:
- Component mentioned: Logging infrastructure (logrus usage throughout)
- Exists in this repo: Yes - pervasive across all packages
- Relevant code paths: All `pkg/` and `cmd/` directories, plus `pkg/logrusutil/` custom wrapper

**Information Completeness**:
- Sufficient detail provided: Yes - the request is clear and well-motivated
- Missing information: None critical; the scope is self-evident from the codebase

### Recommendation

Keep open and continue triage. This is a valid modernization request. The logrus library is indeed in maintenance mode, and slog is the Go standard library's recommended structured logging solution. The migration is straightforward in concept but very large in scope (~250 files).

**Suggested Action**:
- Keep open and continue triage

## Code Research

### Current Implementation

**Primary Components**:
- `pkg/logrusutil/logrusutil.go` - Central logging initialization and utility wrappers
- `pkg/logrusutil/logrusutil_test.go` - Tests for the logging utilities
- `sirupsen/logrus` - Used in ~250 Go files across the entire codebase

**Architecture Overview**:

Prow uses logrus pervasively with a centralized initialization pattern. The `pkg/logrusutil` package provides:

1. **`ComponentInit()`** - Called as the first line in 31+ `cmd/*/main.go` entry points. Sets up JSON formatter with line numbers, component name from build version, and GCP-compatible "severity" field.
2. **`DefaultFieldsFormatter`** - Wraps logrus.Formatter to inject default fields (component name) and rename "level" to "severity" for GCP log collection compatibility. Thread-safe via field map copying.
3. **`CensoringFormatter`** - Wraps any formatter to scrub sensitive data (secrets, tokens) from log output. Used in `pkg/config/secret/agent.go` and `pkg/pod-utils/clone/clone.go`.
4. **`ThrottledWarnf()`** - Rate-limits warning messages for deprecation notices. Used in `pkg/plugins/config.go` for 5 different deprecation warnings.
5. **Controller-runtime integration** - Uses `bombsimon/logrusr/v4` to bridge logrus → logr for Kubernetes library compatibility.

**Key Usage Patterns**:

1. **Struct field injection** (dominant pattern): Loggers stored as `*logrus.Entry` fields in structs:
   - `plugins.Agent.Logger *logrus.Entry` - passed to all plugin handlers
   - `syncController.logger *logrus.Entry` in Tide
   - `reconciler.log *logrus.Entry` in Plank
   - `client.logger *logrus.Entry` in GitHub client

2. **WithField/WithFields chaining**: Context-rich structured logging:
   - `log.WithField("org", org).WithField("repo", repo).Info("synced")`
   - `log.WithFields(logrus.Fields{"org": org, "repo": repo, "branch": branch})`
   - Helper methods like `pr.logFields()` and `pjutil.ProwJobFields(pj)` standardize field sets

3. **Global logger** for initialization: `logrus.WithError(err).Fatal("...")` in main.go files

4. **Printf-style messages**: `logrus.Infof("message %s", val)` used widely

**Common log fields**: `org`, `repo`, `branch`, `pr`, `sha`, `base-sha`, `head-sha`, `job`, `controller`, `component`, `plugin`, `duration`

### Public API Surface Exposing logrus Types

**Critical Interfaces**:
1. `crier.ReportClient` interface - `Report()` and `ShouldReport()` both take `*logrus.Entry` parameter. All 5+ reporter implementations must change.
2. `github.Client` interface - `WithFields(logrus.Fields)` method returns new Client
3. `bugzilla.Client` interface - `WithFields(logrus.Fields)` method
4. `jira.Client` interface - `WithFields(logrus.Fields)` method
5. `repoowners.Interface` - `WithFields(logrus.Fields)` method

**Plugin System**:
- All plugin handler types (`IssueHandler`, `PullRequestHandler`, `GenericCommentHandler`, etc.) receive `plugins.Agent` which exposes `Logger *logrus.Entry`
- `plugins.NewAgent()` takes `*logrus.Entry` parameter
- This is the widest API surface - every plugin interacts with logrus through Agent

**Constructor functions**: Many `New*()` functions accept `*logrus.Entry` parameters across `commentpruner`, `githuboauth`, `git/v2`, and others.

### Root Cause Analysis

**Primary Cause**: Not a bug — this is a dependency modernization request. logrus has been in maintenance mode since 2020. Go introduced `log/slog` in 1.21 (2023) as the standard structured logging solution.

**Contributing Factors**:
1. logrus is ~5x slower than slog (~3200ns/op vs ~650ns/op)
2. logrus uses `Fatal`/`Panic` log levels that conflate logging with control flow
3. The Kubernetes ecosystem (klog, logr) now has native slog interop, making the `logrusr` bridge unnecessary
4. logrus will receive no new features or significant bug fixes

### Proposed Solutions

#### Approach 1: Big-Bang Migration

**Description**: Convert all ~250 files from logrus to slog in a single effort (or small number of large PRs).

**Pros**:
- Clean, complete transition
- No bridge/adapter code needed long-term
- Consistent codebase immediately

**Cons**:
- Extremely large change, high risk of merge conflicts
- Difficult to review
- All-or-nothing — hard to land incrementally
- High risk of regressions

**Complexity**: Very High
**Backwards Compatibility**: Breaking for any downstream consumers using Prow as a library

#### Approach 2: Incremental Migration with Bridge (Recommended)

**Description**: Migrate package-by-package using a bridge adapter (e.g., `samber/slog-logrus`) so new slog code and old logrus code produce consistent output during transition.

**Phases**:
1. **Phase 1 - Foundation**: Create a `pkg/slogutil/` package (replacing `pkg/logrusutil/`) with slog equivalents of `ComponentInit()`, `DefaultFieldsFormatter` (as slog Handler), and `CensoringFormatter` (as slog Handler middleware). Set up bridge so slog output flows through logrus during transition.
2. **Phase 2 - Interface migration**: Update key interfaces (`ReportClient`, `Client.WithFields()`, `plugins.Agent.Logger`) to accept `*slog.Logger` instead of `*logrus.Entry`. This is the hardest step as it affects all implementations.
3. **Phase 3 - Package-by-package conversion**: Convert individual packages from logrus to slog API, starting with leaf packages (fewer dependents) and working inward.
4. **Phase 4 - Cleanup**: Remove logrus dependency, `logrusr` bridge, and `pkg/logrusutil/`. Use `logr.FromSlogHandler()` for controller-runtime compatibility instead of `logrusr`.

**Pros**:
- Each PR is reviewable and testable independently
- Can be paused/resumed
- Lower risk of regressions
- Allows learning and course-correction during migration

**Cons**:
- Longer overall timeline
- Temporary bridge code adds complexity
- Inconsistent codebase during transition period

**Complexity**: Medium per-PR, High overall coordination
**Backwards Compatibility**: Can be managed incrementally

#### Approach 3: Abstraction Layer First

**Description**: Introduce a logging abstraction interface (wrapping slog) that all code migrates to, then swap the backend from logrus to slog.

**Pros**:
- Decouples API migration from backend migration
- Future-proof if another logger emerges

**Cons**:
- Over-engineering — slog IS the standard library abstraction
- More code to maintain
- Kubernetes ecosystem uses logr as the abstraction, adding another layer is redundant

**Complexity**: Medium
**Backwards Compatibility**: Manageable

#### Recommendation

**Preferred Approach**: Approach 2 (Incremental Migration with Bridge)

This is the proven strategy — Gravitational Teleport successfully migrated a similarly large codebase from logrus to slog using this exact approach (tracked in their issue #28109, completed via ~20 PRs over several months).

**Key Implementation Considerations**:
1. Handle `logrus.Fatal`/`logrus.Panic` by converting to `slog.Error()` + `os.Exit(1)` or explicit `panic()`
2. Convert `Infof("message %s", val)` printf-style calls to structured `slog.Info("message", "key", val)`
3. Watch for package-level logger initialization ordering — Teleport discovered `var log = slog.Default()` captures the unconfigured default. Use lazy-resolving handler pattern if needed.
4. For Kubernetes interop: use `logr.FromSlogHandler()` to create logr.Logger from slog handler, replacing the current `logrusr` adapter
5. The `CensoringFormatter` must be reimplemented as a slog Handler middleware (wrapping another Handler)
6. GCP "severity" field mapping must be preserved in the new slog Handler

**Testing Requirements**:
- Verify log output format remains GCP-compatible
- Verify secret censoring works with new slog handler
- Verify controller-runtime integration via logr bridge
- Test each package conversion independently

**Migration/Rollout Strategy**:
- Start with leaf packages that have few dependents
- Core interfaces (ReportClient, plugins.Agent) are the critical path — coordinate these changes carefully
- Each package conversion should be a separate PR for reviewability

## Effort Assessment

**Effort Level**: 3 - Large (requires expertise)

### Summary

This is a codebase-wide dependency migration touching ~250 files, multiple public interfaces, and custom logging infrastructure. While the individual file changes are mechanically simple (API translation), the scope, interface-breaking nature, and coordination required across the plugin system, reporter implementations, and Kubernetes ecosystem interop push this firmly to Level 3.

### Factor Analysis

#### Scope of Changes
- **Assessment**: Very Large
- **Details**: ~250 Go files import logrus. 31+ cmd/ entry points use ComponentInit(). 5+ public interfaces expose logrus types. Custom pkg/logrusutil/ package must be rewritten. If done incrementally, each individual PR is moderate (5-20 files), but the total migration is massive.
- **Level Indication**: 3-4

#### Complexity
- **Assessment**: Moderate
- **Details**: Each individual file conversion is mechanically straightforward (logrus.WithField → slog.With, logrus.Info → slog.Info). The complexity lies in: (1) rewriting CensoringFormatter as slog Handler middleware, (2) handling Fatal/Panic level removal, (3) managing package-level logger initialization ordering, (4) maintaining GCP severity field compatibility, (5) coordinating interface changes across the plugin system.
- **Level Indication**: 2-3

#### Required Expertise
- **Assessment**: Moderate-to-Deep
- **Details**: Requires understanding of: slog Handler interface and middleware patterns, logr/slog interop for Kubernetes libraries, Prow's plugin system architecture, and how CensoringFormatter and DefaultFieldsFormatter work. A contributor needs to understand both the old and new logging ecosystems well.
- **Level Indication**: 2-3

#### Clarity and Certainty
- **Assessment**: Well-defined
- **Details**: The goal is clear (replace logrus with slog), the approach is proven (Teleport did it), and the API mapping is well-understood. The main uncertainty is around ordering of interface changes and whether to use a bridge library during transition.
- **Level Indication**: 1-2

#### Testing Requirements
- **Assessment**: Moderate
- **Details**: Existing tests provide good coverage. Most changes are API-level swaps that existing tests validate. New tests needed for: slog Handler equivalents of DefaultFieldsFormatter and CensoringFormatter, logr bridge integration, and GCP severity field output format.
- **Level Indication**: 2-3

#### Backwards Compatibility
- **Assessment**: Breaking changes (if done at interface level)
- **Details**: Any downstream consumers using Prow as a Go library and depending on `*logrus.Entry` in function signatures, struct fields, or interface methods will break. The `plugins.Agent.Logger` field type change affects all plugin implementations. The `ReportClient` interface change affects all reporter implementations. If done incrementally, each breaking change can be managed, but the total breaking surface is significant.
- **Level Indication**: 3-4

#### Architectural Alignment
- **Assessment**: Good fit with pattern extension
- **Details**: slog is a natural evolution from logrus. The existing patterns (struct field injection, WithField chaining, centralized init) all have direct slog equivalents. The CensoringFormatter → slog Handler middleware pattern is actually cleaner architecturally. Removing logrusr in favor of native logr/slog interop simplifies the dependency chain.
- **Level Indication**: 2-3

#### External Dependencies
- **Assessment**: Well-supported
- **Details**: slog is part of the Go standard library. logr has native slog interop. klog supports SetSlogLogger(). Bridge libraries (samber/slog-logrus) are mature and stable. No external blockers.
- **Level Indication**: 1-3

### Recommended Labels

- [x] `kind/cleanup`: Dependency modernization / tech debt reduction
- [x] `area/prow`: Affects all Prow components
- [x] `help-wanted`: Large scope benefits from community contribution, but requires coordination
- [ ] `good-first-issue`: Too large and cross-cutting for a new contributor

### Guidance for Contributors

**For Level 3 (Large)**:
- Requires understanding of Prow's plugin system, reporter framework, and logging initialization
- Should consult with maintainers before starting to agree on migration strategy and PR ordering
- Recommended approach: Incremental migration with bridge (see Proposed Solutions above)
- Reference: Gravitational Teleport's logrus→slog migration (issue #28109) as a model
- Key architectural considerations:
  - Maintain GCP log collection compatibility ("severity" field)
  - Preserve secret censoring capability
  - Coordinate interface changes to minimize churn
  - Handle Fatal/Panic → Error + os.Exit(1) conversion
- Could be split into a tracking issue with sub-issues for each phase

### Caveats and Considerations

- The scope is borderline Level 3/4 due to sheer volume, but the mechanical nature of most changes and the proven migration pattern keep it at Level 3.
- This is excellent work for an experienced contributor looking for a high-impact modernization project.
- The migration could reasonably be spread across 15-25 PRs over several months.
- Each individual PR in the incremental approach would be Level 1-2 in isolation — the challenge is the coordination and total scope.

## Proposed Issue Augmentation

### Title Change

- **No change needed**: Current title "Switch From logrus To slog" is clear, specific, and accurately describes the request.

### Proposed GitHub Comment

```
Logrus is currently imported in approximately 250 Go files across the codebase, with a centralized initialization through `pkg/logrusutil.ComponentInit()` called by 31+ command entry points. Prow also has custom logging infrastructure in `pkg/logrusutil/` including a `DefaultFieldsFormatter` (which renames "level" to "severity" for GCP log collection compatibility and injects component name), a `CensoringFormatter` (which scrubs secrets from log output), and `ThrottledWarnf` (which rate-limits deprecation warnings). For Kubernetes library interop, Prow currently bridges logrus to logr via `bombsimon/logrusr/v4`. All of these would need slog equivalents.

The main challenge is that `*logrus.Entry` and `logrus.Fields` are exposed in several public interfaces: the `crier.ReportClient` interface (implemented by 5+ reporters), the `github.Client`/`bugzilla.Client`/`jira.Client`/`repoowners.Interface` all have `WithFields(logrus.Fields)` methods, and the plugin system's `plugins.Agent` struct exposes `Logger *logrus.Entry` to every plugin handler. This means the migration cannot be purely internal — interface consumers must update too. An incremental migration using a bridge adapter (e.g., `samber/slog-logrus`) would allow package-by-package conversion while keeping the codebase functional throughout. Gravitational Teleport successfully completed a similar-scale logrus-to-slog migration using this approach. For Kubernetes library compatibility, `logr.FromSlogHandler()` can replace the current `logrusr` bridge, simplifying the dependency chain.

This is a large but well-understood migration that could be broken into 15-25 incremental PRs across four phases: (1) create slog-based equivalents of `pkg/logrusutil/` utilities, (2) update core interfaces to accept `*slog.Logger`, (3) convert packages individually starting from leaves, (4) remove logrus dependency and `logrusr` bridge.

/area dependency
/kind cleanup
```

### Rationale

**What's being added**:
- Scope quantification: the issue says "switch" but doesn't quantify; adding that logrus is in ~250 files with custom infrastructure gives contributors a realistic picture
- API surface impact: the public interface exposure is the key technical challenge that isn't mentioned in the issue
- Migration strategy: referencing the incremental approach and Teleport precedent gives contributors a proven playbook
- Phase breakdown: concrete phases help potential contributors understand the work structure

**Why these labels**:
- `/area dependency`: This is a dependency modernization (logrus → slog). No single component label applies since all components are affected.
- `/kind cleanup`: This is tech debt cleanup / dependency modernization, not a new feature or bug fix.
- No difficulty label: Level 3 effort — too complex and cross-cutting for good-first-issue or help-wanted. Experienced contributors will self-select.

**What's NOT included**:
- No `/retitle`: Current title is already clear and concise
- No `/priority`: This is a modernization, not urgent — logrus works fine, it just won't receive new features
- No `/help-wanted`: Despite the large scope, the cross-cutting nature and interface-breaking changes require an experienced contributor who understands the full architecture, not a typical help-wanted candidate
- Performance comparison (slog is ~5x faster): Omitted because Prow is not performance-sensitive in its logging and this would be a misleading motivation

## Comment Posted

Posted augmentation comment on: 2026-03-29
Comment URL: https://github.com/kubernetes-sigs/prow/issues/662#issuecomment-4149115466

## Briefing Completed

Briefed maintainer on: 2026-03-29

Key questions asked:
- None — maintainer acknowledged all slides without questions

Maintainer decision:
Proceed with wrapup and posting.

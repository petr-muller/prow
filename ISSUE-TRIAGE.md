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

## Next Steps

(Action items will be added here)

# Triage for Issue #718

**Status**: In Progress
**Created**: 2026-05-12

## Issue Information

- **Issue Number**: #718
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/718

## Initial Validation

**Assessment**: LEGITIMATE

### Analysis

The issue reports that metrics endpoints for `crier` and `prow-controller-manager` (title says sinker but body says prow-controller-manager) return errors instead of Prometheus metrics. The error is:

> collected metric "go_gc_duration_seconds" { ... } was collected before with the same name and label values

This is a classic Prometheus duplicate metric collector registration error. All 29 duplicated metrics are Go runtime metrics (`go_*` family), indicating the Go runtime collector is being registered twice with the Prometheus default registry.

The reporter identifies PR #713 ("Bump Kubernetes dependencies to v0.33.11") as the likely cause. This PR was merged on 2026-05-11, and the broken image tag `v20260511-5c4ab968b` matches the merge commit `5c4ab968b`.

**Issue Category**: Bug (regression from dependency bump)

**Repository Scope Check**:
- Components mentioned: crier, prow-controller-manager (sinker mentioned in title but body shows prow-controller-manager)
- Exist in this repo: Yes
- Relevant code paths: pkg/crier/, cmd/prow-controller-manager/, and metrics/prometheus registration code
- Suspected cause: PR #713 bumped k8s deps to v0.33.11, which likely brought in a newer prometheus client or controller-runtime version that auto-registers Go collectors, conflicting with explicit registration in Prow code

**Information Completeness**:
- Sufficient detail provided: Yes
- Error output: Complete, showing all 29 duplicate metrics
- Affected version: Clearly identified (v20260511-5c4ab968b)
- Suspected cause: Identified (PR #713)

### Recommendation

This is a clear, well-documented regression report. The metrics endpoints being broken means monitoring/alerting for these Prow components is non-functional, which is a significant operational impact.

**Suggested Action**:
- Keep open and continue triage
- Investigate the dependency bump for prometheus/controller-runtime changes that cause duplicate Go collector registration

## Code Research

### Current Implementation

**Primary Components**:
- `pkg/metrics/metrics.go` — Central metrics serving logic for all Prow components
- `cmd/crier/main.go:321` — Calls `metrics.ExposeMetrics("crier", ...)`
- `cmd/prow-controller-manager/main.go:220` — Calls `metrics.ExposeMetrics("plank", ...)`

**Architecture Overview**:
Prow combines two Prometheus registries when serving metrics:

1. `prometheus.DefaultRegistry` — Where Prow's own metrics are registered via `init()` functions throughout the codebase (pkg/crier/metrics.go, pkg/kube/metrics.go, etc.). This registry also auto-registers Go runtime and process collectors in its own `init()`.
2. `ctrlruntimemetrics.Registry` — Controller-runtime's own registry, where controller-runtime registers its reconcile metrics AND Go/process collectors via `init()` in `pkg/internal/controller/metrics/metrics.go`.

These are merged at serve time via `prometheus.Gatherers{reg, ctrlruntimemetrics.Registry}` (metrics.go:62).

**The Deduplication Mechanism** (metrics.go:53-56):
To avoid duplicate Go runtime metrics from both registries, Prow attempts to unregister the Go and process collectors from controller-runtime's registry:
```go
ctrlruntimemetrics.Registry.Unregister(prometheus.NewGoCollector())
ctrlruntimemetrics.Registry.Unregister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
```

### Root Cause Analysis

**Primary Cause**: Mismatch between the Go collector registered by controller-runtime and the one Prow tries to unregister.

**Dependency changes in PR #713**:
- `prometheus/client_golang`: v1.20.5 → v1.22.0
- `sigs.k8s.io/controller-runtime`: v0.20.1 → v0.21.0

**What changed in controller-runtime v0.21.0**:

The Go collector registration in `pkg/internal/controller/metrics/metrics.go` init changed:
- **v0.20.1**: `collectors.NewGoCollector()` — default options
- **v0.21.0**: `collectors.NewGoCollector(collectors.WithGoCollectorRuntimeMetrics(collectors.MetricsAll))` — with ALL runtime metrics enabled

**Why the unregister fails**:

Prow's unregister call creates `prometheus.NewGoCollector()` (deprecated API, default options). The `Unregister()` method compares collectors by their `Describe()` output (descriptor sets). Since `collectors.MetricsAll` produces a different/larger set of descriptors than the default, the newly created collector doesn't match the one registered by controller-runtime. The unregister silently fails (returns false), leaving the Go collector in controller-runtime's registry.

**Result**: When both registries are gathered, `go_*` metrics appear from both `prometheus.DefaultRegistry` (auto-registered) and `ctrlruntimemetrics.Registry` (failed to unregister), causing the "collected before with the same name and label values" error.

**Contributing Factors**:
1. The unregister approach was always fragile — it relied on creating an equivalent collector that would match by descriptor identity
2. Using the deprecated `prometheus.NewGoCollector()` API instead of the `collectors` package API
3. No error checking on the `Unregister()` return value (it returns bool)

### Affected Components

All Prow components that call `ExposeMetrics` or `ExposeMetricsWithRegistry` AND import controller-runtime (directly or transitively):
- **crier** (confirmed broken by reporter)
- **prow-controller-manager** (confirmed broken by reporter)
- Potentially: sinker, hook, deck, tide, horologium — if they import controller-runtime

Components that use their own registry are NOT affected:
- **exporter** (cmd/exporter/main.go) — creates `prometheus.NewRegistry()` and registers its own collectors

### Proposed Solutions

#### Approach 1: Match controller-runtime's collector options

Update the unregister call to create a collector with the same options controller-runtime uses:
```go
ctrlruntimemetrics.Registry.Unregister(collectors.NewGoCollector(
    collectors.WithGoCollectorRuntimeMetrics(collectors.MetricsAll)))
```

**Pros**: Minimal change, directly addresses the mismatch
**Cons**: Still fragile — will break again if controller-runtime changes its options. Couples Prow to controller-runtime internals.
**Complexity**: Low
**Backwards Compatibility**: None — purely internal change

#### Approach 2: Clear and rebuild controller-runtime registry

Instead of trying to unregister specific collectors, clear everything from `ctrlruntimemetrics.Registry` and re-register only the controller-runtime specific metrics (reconcile counters, etc.):

**Pros**: More robust, doesn't depend on matching collector options
**Cons**: More invasive, needs to know which controller-runtime metrics to keep. The registry doesn't expose a "clear all" operation — would need to iterate.
**Complexity**: Medium
**Backwards Compatibility**: Could lose controller-runtime metrics if not careful

#### Approach 3: Use a single custom registry for everything

Create a fresh `prometheus.NewRegistry()`, register Go/process collectors once, and use it for everything (similar to what exporter does). Don't combine with controller-runtime's registry.

**Pros**: Clean, no duplication possible, follows exporter's proven pattern
**Cons**: Loses controller-runtime metrics (reconcile counters etc.) unless they're re-registered. Larger change.
**Complexity**: Medium-High
**Backwards Compatibility**: May lose controller-runtime metrics

#### Approach 4: Unregister Go collectors from DefaultRegistry instead

Since DefaultRegistry auto-registers Go/process collectors too, unregister them from there and keep controller-runtime's (which has the richer MetricsAll set):
```go
prometheus.Unregister(prometheus.NewGoCollector())
prometheus.Unregister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
```

**Pros**: Keeps the richer MetricsAll runtime metrics from controller-runtime. DefaultRegistry's own Go collector matches `prometheus.NewGoCollector()` exactly (same init code), so unregister will succeed.
**Cons**: Still fragile if prometheus changes its init. Slightly different metric set served (MetricsAll includes more).
**Complexity**: Low
**Backwards Compatibility**: Slightly different metric set (more runtime metrics exposed, which is arguably better)

#### Recommendation

**Preferred Approach**: Approach 1 (match controller-runtime's options) for the immediate fix, as it's the smallest change and directly addresses the regression. However, a follow-up should consider Approach 4 (unregister from DefaultRegistry instead) as a more robust long-term solution, since the default registry's own collectors are guaranteed to match its own `init()`.

**Key Implementation Considerations**:
1. Import `collectors` package instead of using deprecated `prometheus.NewGoCollector()`
2. Check `Unregister()` return value and log a warning if it fails
3. Add a test that verifies metrics endpoint returns valid data when both registries are combined

### Test Coverage

**Existing Tests**:
- No test currently validates that the combined metrics endpoint works without errors
- `pkg/metrics/` has no test file for `metrics.go`

**Test Gaps**:
- Missing: Test that `ExposeMetricsWithRegistry` produces valid gathered metrics
- Missing: Test that controller-runtime Go collectors are successfully unregistered

## Effort Assessment

**Effort Level**: 1 - Easy (good-first-issue)

### Summary

The fix requires changing 2 lines in a single file (`pkg/metrics/metrics.go`) to match the collector options that controller-runtime v0.21.0 now uses. The root cause is fully understood, the solution is clear, and the change is purely internal with no backwards compatibility concerns.

### Factor Analysis

#### Scope of Changes
- **Assessment**: Small
- **Details**: 1 file (`pkg/metrics/metrics.go`), ~5 lines changed (update imports, change 2 `Unregister` calls)
- **Level Indication**: 1-2

#### Complexity
- **Assessment**: Simple
- **Details**: Direct fix — change the collector constructor to match what controller-runtime registers. No logic changes, no new patterns.
- **Level Indication**: 1-2

#### Required Expertise
- **Assessment**: Minimal
- **Details**: Requires understanding of Prometheus collector registration (well-documented). The root cause analysis in this triage provides all necessary context.
- **Level Indication**: 1-2

#### Clarity and Certainty
- **Assessment**: Well-defined
- **Details**: Root cause fully identified. The exact mismatch (default options vs `MetricsAll`) is known. Solution is mechanical.
- **Level Indication**: 1-2

#### Testing Requirements
- **Assessment**: Simple
- **Details**: A unit test calling `prometheus.Gatherers{DefaultGatherer, ctrlruntimemetrics.Registry}.Gather()` and checking for no errors would validate the fix. Follows standard Go test patterns.
- **Level Indication**: 1-2

#### Backwards Compatibility
- **Assessment**: Fully compatible
- **Details**: Purely internal metrics registration. The metrics endpoint will serve the same `go_*` metrics it always did (just without errors). No configuration or API changes.
- **Level Indication**: 1-2

#### Architectural Alignment
- **Assessment**: Perfect fit
- **Details**: Fixing the existing deduplication mechanism to work with updated dependencies. No new patterns.
- **Level Indication**: 1-2

#### External Dependencies
- **Assessment**: None
- **Details**: Internal code change adapting to already-bumped dependencies. No external system changes needed.
- **Level Indication**: 1-3

### Recommended Labels

- [x] `good-first-issue`: Clear, well-defined, small scope, root cause documented
- [x] `kind/bug`: Regression from dependency bump
- [x] `area/prow`: Core Prow infrastructure
- [ ] `help-needed`: Too simple for this label

### Guidance for Contributors

- Read the root cause analysis in this triage document
- The fix is in `pkg/metrics/metrics.go` lines 53-56
- Change `prometheus.NewGoCollector()` to `collectors.NewGoCollector(collectors.WithGoCollectorRuntimeMetrics(collectors.MetricsAll))`
- Change `prometheus.NewProcessCollector(...)` to `collectors.NewProcessCollector(collectors.ProcessCollectorOpts{})`
- Add import for `github.com/prometheus/client_golang/prometheus/collectors`
- Consider adding a test in `pkg/metrics/metrics_test.go` that validates gathering from both registries succeeds

### Caveats and Considerations

- The `Unregister`-by-creating-equivalent-collector approach is inherently fragile. If controller-runtime changes its collector options again, this will break again. A comment noting this coupling would be valuable.
- The `nolint:staticcheck` directives may need updating since `collectors.NewGoCollector()` is the non-deprecated API.

## Proposed Issue Augmentation

### Title Change

- **Current**: Metric endpoints for `crier` and `prow-controller-manager` are broken
- **Proposed**: Metrics endpoints broken for components using controller-runtime after k8s v0.33.11 bump
- **Rationale**: The issue affects more components than just crier and prow-controller-manager. The title also says "sinker" which contradicts the body. The proposed title identifies the actual scope and cause.

### Proposed GitHub Comment

```
/retitle Metrics endpoints broken for components using controller-runtime after k8s v0.33.11 bump

The root cause is a mismatch in `pkg/metrics/metrics.go`. To avoid duplicate `go_*` metrics when combining `prometheus.DefaultRegistry` with controller-runtime's registry, Prow [unregisters](https://github.com/kubernetes-sigs/prow/blob/5c4ab968b/pkg/metrics/metrics.go#L54-L56) Go and process collectors from controller-runtime's registry by creating equivalent collector instances and passing them to `Unregister()`. This works because `Unregister` matches collectors by their descriptor sets.

However, controller-runtime v0.21.0 (pulled in by the k8s v0.33.11 bump) [changed](https://github.com/kubernetes/controller-runtime/compare/v0.20.1...v0.21.0) its Go collector registration from `collectors.NewGoCollector()` to `collectors.NewGoCollector(collectors.WithGoCollectorRuntimeMetrics(collectors.MetricsAll))`. The `MetricsAll` option produces a different descriptor set, so Prow's unregister call (which still uses `prometheus.NewGoCollector()` with default options) no longer matches, fails silently, and both registries end up serving `go_*` metrics — hence the "collected before with the same name and label values" error.

This affects all components that both call `ExposeMetrics` and import controller-runtime: crier, deck, horologium, prow-controller-manager, sinker, and tide. The fix is to update the unregister calls in `pkg/metrics/metrics.go` to use `collectors.NewGoCollector(collectors.WithGoCollectorRuntimeMetrics(collectors.MetricsAll))` to match what controller-runtime now registers.

/area prow
/kind bug
/good-first-issue
```

### Rationale

**What's being added**:
- Root cause explanation with links to the exact code and the controller-runtime change
- Complete list of affected components (reporter only identified 2 of 6)
- Specific fix location and approach for potential contributors

**Why these labels**:
- `/area prow`: This affects core Prow metrics infrastructure, not a specific component
- `/kind bug`: Already applied by reporter, confirming it
- `/good-first-issue`: Level 1 effort — single file, ~5 lines, clear root cause, mechanical fix

**What's NOT included**:
- No priority label: While this breaks monitoring, it doesn't break Prow's core functionality. Let maintainers decide priority.
- No workaround: There isn't a simple config-level workaround; it requires a code fix.

## Briefing Completed

Briefed maintainer on: 2026-05-12

Key questions asked:
- None

Maintainer decision:
- No retitle — original title is fine
- No /area or /good-first-issue — understanding the failure case and validating the fix requires expertise
- Post root cause analysis and /kind bug only

## Comment Posted

Posted augmentation comment on: 2026-05-12
Comment URL: https://github.com/kubernetes-sigs/prow/issues/718#issuecomment-4433963503

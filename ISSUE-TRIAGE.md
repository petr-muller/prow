# Triage for Issue #609

**Status**: In Progress
**Created**: 2026-02-10

## Issue Information

- **Issue Number**: #609
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/609
- **Title**: Dealing with default and named clusters in prow configuration
- **Author**: derryos
- **Labels**: (none)
- **Comments**: 0

## Issue Summary

The reporter uses multiple build clusters (cluster-a/cluster-b/cluster-c plus default). They configured named clusters with the same kubeconfig as the default cluster, intending to transition away from the default post-upgrade. The problem is that Prow's pipeline controller gets confused because the default cluster and a named cluster share the same kubeconfig/context, causing ProwJobs to be erroneously deleted.

The specific code triggering the issue is in `cmd/pipeline/controller.go` around line 463.

## Initial Validation

**Assessment**: LEGITIMATE

### Analysis

The issue describes a real behavioral problem in Prow's pipeline controller when two cluster context names (e.g., `"default"` and `"my-cluster"`) resolve to the same underlying Kubernetes cluster.

**Issue Category**: Bug

**Repository Scope Check**:
- Component mentioned: Pipeline controller (`cmd/pipeline`)
- Exists in this repo: Yes
- Relevant code paths:
  - `cmd/pipeline/controller.go` (lines 181-195, 285-296, 443-556): Event handlers, config lookup, reconciliation
  - `cmd/pipeline/main.go` (lines 127-164): Pipeline config initialization
  - `pkg/kube/config.go` (lines 34-71): Cluster config loading
  - `pkg/pjutil/pjutil.go` (lines 408-414): `ClusterToCtx()` context mapping

**Information Completeness**:
- Sufficient detail provided: Yes
- The reporter identifies the exact code location, describes the scenario clearly, and explains the resulting behavior (ProwJobs being deleted)

### Mechanism

The pipeline controller creates independent event handlers and pipeline clients for each named cluster context. When two contexts point to the same underlying cluster:

1. Both informers watch the same cluster and see the same PipelineRun events
2. When a ProwJob specifies `Cluster: "my-cluster"`, the reconcile is called with `ctx = "my-cluster"`
3. But the `"default"` informer also enqueues the same ProwJob (same cluster), calling reconcile with `ctx = "default"`
4. At line 463, the check `ClusterToCtx(pj.Spec.Cluster) != ctx` evaluates to `"my-cluster" != "default"` → true
5. This sets `wantPipelineRun = false`, leading to PipelineRun deletion

The behavior is destructive (deleting PipelineRuns that should exist) and the use case is reasonable (naming clusters explicitly for migration purposes).

### Recommendation

This is a legitimate issue. While the user can work around it by not creating overlapping cluster configurations, the behavior is surprising and destructive. Prow should either:
- Detect and warn/error when two cluster contexts resolve to the same underlying cluster
- Handle the case gracefully by deduplicating

**Suggested Action**:
- Keep open and continue triage

## Code Research

### Current Implementation

**Primary Components**:
- Pipeline Controller: `cmd/pipeline/controller.go` - Manages Tekton PipelineRun lifecycle across clusters
- Pipeline Main: `cmd/pipeline/main.go` - Initializes per-cluster pipeline configs and informers
- Cluster Config: `pkg/kube/config.go` - Loads kubeconfig contexts into a map
- Context Mapping: `pkg/pjutil/pjutil.go` - Maps ProwJob cluster specs to context names

**Architecture Overview**:
The pipeline controller is the only Prow controller designed for multi-cluster Tekton pipeline management. It loads all kubeconfig contexts (when `--all-contexts=true`), creates separate informers and clients for each, and reconciles ProwJobs against PipelineRuns using context names as the routing key. The controller assumes a 1:1 mapping between context names and underlying clusters.

**Key Code Paths**:
1. Cluster config loading: `cmd/pipeline/main.go:120-164` - Iterates all contexts, creates `pipelineConfig` per context
2. Event handler registration: `cmd/pipeline/controller.go:181-195` - Each cluster context gets its own informer handlers
3. Work queue key format: `cmd/pipeline/controller.go:255-270` - Keys are `context/namespace/name`
4. Context matching: `cmd/pipeline/controller.go:463` - Checks `ClusterToCtx(pj.Spec.Cluster) != ctx`
5. Pipeline config fallback: `cmd/pipeline/controller.go:285-296` - Falls back to `"default"` if context not found
6. Context alias: `pkg/pjutil/pjutil.go:408-414` - Maps empty string to `"default"`

**Data Flow**:
1. `main.go` calls `LoadClusterConfigs()` which returns a map of context names → REST configs
2. For each context, a separate `pipelineConfig` (client + informer) is created
3. Event handlers are registered per-context: ProwJob events are routed using `ClusterToCtx(pj.Spec.Cluster)`, PipelineRun events are routed using the bound `ctx` variable
4. Reconciliation parses the key to extract `ctx`, then looks up both the ProwJob and PipelineRun, deciding whether a PipelineRun should exist (`wantPipelineRun`)
5. If `wantPipelineRun = false` but a PipelineRun exists, the controller deletes it

### Related Code

**Dependencies**:
- `pkg/kube/constants.go:26-30`: Defines `DefaultClusterAlias = "default"` and `InClusterContext = ""`
- `pkg/apis/prowjobs/v1/types.go:1159-1164`: `ProwJob.ClusterAlias()` method (parallel to `ClusterToCtx`)

**Scope of Impact**:
- Pipeline controller is the ONLY controller affected. Other controllers (crier, status-reconciler, jenkins-operator) use single-cluster mode and don't have the multi-context architecture.

**Similar Functionality**:
- `main.go:127-136` already handles one alias case: it deletes the `InClusterContext` ("") entry to avoid duplication with `DefaultClusterAlias` ("default"). This shows awareness of the aliasing problem but only handles the empty-string-to-default case.

### Test Coverage

**Existing Tests**:
- `cmd/pipeline/controller_test.go` (~1489 lines): 23 reconcile test cases
- `cmd/pipeline/main_test.go` (~88 lines): Main setup tests
- `pkg/kube/config_test.go` (~296 lines): Config loading with 6 scenarios

**Cross-cluster test cases** (controller_test.go:467-533):
- "delete prow pipeline runs in the wrong cluster" - Tests when PipelineRun is found in wrong context
- "ignore random pipeline run in the wrong cluster" - Tests ignoring unrelated PipelineRuns

**Config loading tests** (config_test.go:220-223):
- Tests that duplicate context NAMES in different kubeconfig files cause an error

**Test Gaps**:
- No test for two contexts pointing to the same underlying cluster (same server URL, different context names)
- No test for event handler deduplication when contexts resolve to the same cluster
- No test for ProwJob routing correctness when a named cluster alias points to the same backend as "default"

### Documentation Review

**Code Comments**:
- `main.go:134`: "the InClusterContext is always mapped to DefaultClusterAlias in the controller, so there is no need to watch for this config" - Shows awareness of the aliasing problem for the empty-string case
- `controller.go:463`: "Build is in wrong cluster, we do not want this build" - Assumes context mismatch means different cluster

**User Documentation**:
- `site/content/en/docs/build-clusters.md` (194 lines): Documents multi-cluster setup but assumes 1:1 context-to-cluster mapping. Does not address or warn about aliasing scenarios.

**Known Limitations**:
- No documented limitation about cluster context uniqueness requirements

### Root Cause Analysis

**Primary Cause**:
The pipeline controller assumes each kubeconfig context name maps to a physically distinct cluster. When two context names point to the same underlying cluster, both contexts get independent informers that watch the same resources. During reconciliation, the context-name-based routing (`ClusterToCtx(pj.Spec.Cluster) != ctx`) incorrectly identifies PipelineRuns as being in the "wrong cluster" because the context names differ, even though the underlying cluster is the same.

**Contributing Factors**:
1. The `pipelines` map uses context name as the key with no deduplication by cluster identity
2. The `InClusterContext → DefaultClusterAlias` deduplication in `main.go:127-136` handles only one specific alias case, not the general case
3. The `getPipelineConfig` fallback to "default" (controller.go:285-296) masks configuration issues rather than failing fast
4. No validation at startup that all configured contexts point to distinct clusters

**Reproduction Conditions**:
- Two or more kubeconfig contexts sharing the same cluster API server endpoint
- Pipeline controller running with `--all-contexts=true` (or both contexts configured)
- ProwJobs targeting one of the aliased contexts

### Proposed Solutions

#### Approach 1: Startup Validation and Deduplication

**Description**: At startup, detect when multiple kubeconfig contexts resolve to the same cluster API server. Either deduplicate them (keeping only one) or reject the configuration with a clear error.

**Pros**:
- Prevents the problem entirely
- Fast-fail gives operators clear feedback
- Minimal changes to reconciliation logic
- No runtime overhead

**Cons**:
- May break existing configurations that rely on aliases (though such configs are already broken)
- Requires comparing REST config server URLs, which needs access to the resolved config
- Deduplication must decide which context name to keep

**Affected Components**:
- `cmd/pipeline/main.go`: Add validation after loading cluster configs
- `pkg/kube/config.go`: Possibly add a helper to detect duplicate cluster endpoints

**Complexity**: Low

**Backwards Compatibility**: Configurations that currently have overlapping contexts are already broken (experiencing the reported bug), so rejecting them at startup is strictly better behavior.

#### Approach 2: Reconciliation-Level Tolerance

**Description**: Modify the reconciliation logic to handle the case where a PipelineRun is found via one context but belongs to a ProwJob targeting a different context name that resolves to the same cluster. Instead of deleting the PipelineRun, skip reconciliation for the non-owning context.

**Pros**:
- Allows operators to keep alias configurations
- More flexible for migration scenarios
- Fixes the symptom directly

**Cons**:
- Runtime overhead: requires comparing REST configs or cluster endpoints during reconciliation
- More complex logic in the hot path
- May mask other configuration problems
- Still creates redundant informer watches (wasted resources)

**Affected Components**:
- `cmd/pipeline/controller.go`: Modify the `ClusterToCtx(pj.Spec.Cluster) != ctx` check
- Possibly add a cluster identity cache mapping context names to canonical cluster IDs

**Complexity**: Medium

**Backwards Compatibility**: Fully backwards compatible.

#### Approach 3: Combined - Warn at Startup, Skip at Reconciliation

**Description**: At startup, detect overlapping contexts and log a prominent warning. Build a mapping of context names that share the same cluster. During reconciliation, use this mapping to skip processing for non-primary contexts.

**Pros**:
- Best of both approaches: warns operators AND prevents destructive behavior
- No configuration breakage
- Mapping built once at startup, cheap at runtime

**Cons**:
- More code than either approach alone
- Must define "primary" context selection logic

**Affected Components**:
- `cmd/pipeline/main.go`: Build overlap mapping, emit warnings
- `cmd/pipeline/controller.go`: Use mapping to skip non-primary context reconciliation

**Complexity**: Medium

**Backwards Compatibility**: Fully backwards compatible.

#### Recommendation

**Preferred Approach**: Approach 1 (Startup Validation and Deduplication)

This is the simplest solution and addresses the root cause. Configurations with overlapping contexts are already broken, so rejecting them with a clear error message is the right behavior. If the operator needs named clusters, they should ensure each name points to a distinct cluster.

The validation should:
1. After loading all cluster configs, compare their API server URLs
2. If duplicates are found, log an error identifying the conflicting contexts and exit
3. Alternatively, deduplicate by keeping the first context found and logging a warning

**Key Implementation Considerations**:
1. REST config `Host` field can be compared for equality (normalize URLs first)
2. The existing `InClusterContext → DefaultClusterAlias` dedup in `main.go` should be preserved
3. Error messages should explain what's wrong and how to fix it
4. Consider adding documentation about context uniqueness requirements

**Testing Requirements**:
- Test that duplicate contexts cause startup failure with clear error
- Test that non-overlapping contexts continue to work
- Test the existing `InClusterContext → DefaultClusterAlias` dedup still works
- Regression test for the reconciliation scenario described in the issue

**Migration/Rollout Strategy**:
No migration needed. Operators with overlapping contexts will get a clear error on next restart, prompting them to fix their kubeconfig. Their current configuration is already producing incorrect behavior (PipelineRun deletions).

## Effort Assessment

**Effort Level**: 1 - Easy (good-first-issue)

### Summary

The recommended fix (startup validation to detect duplicate cluster endpoints) is small in scope, follows existing patterns in the codebase, and has clear implementation path. The existing `InClusterContext → DefaultClusterAlias` deduplication in `main.go` serves as a direct template.

### Factor Analysis

#### Scope of Changes
- **Assessment**: Small
- **Details**: 1-2 files (`cmd/pipeline/main.go`, possibly `pkg/kube/config.go`), estimated ~30-50 lines of new code plus tests
- **Level Indication**: 1-2

#### Complexity
- **Assessment**: Simple
- **Details**: Compare REST config `Host` fields across loaded contexts to detect duplicates. Straightforward map-based deduplication logic. No concurrency, no complex algorithms.
- **Level Indication**: 1-2

#### Required Expertise
- **Assessment**: Minimal
- **Details**: Basic Go knowledge, understanding of kubeconfig concepts (contexts, clusters). The existing code in `main.go:120-136` provides a clear template. No deep Prow internals knowledge needed.
- **Level Indication**: 1-2

#### Clarity and Certainty
- **Assessment**: Well-defined
- **Details**: Problem is precisely identified (two contexts → same cluster → destructive behavior). Solution approach is clear (detect at startup, error or warn). No open design questions for the recommended approach.
- **Level Indication**: 1-2

#### Testing Requirements
- **Assessment**: Simple
- **Details**: Add test cases to `cmd/pipeline/main_test.go` or `pkg/kube/config_test.go` following existing patterns. Test that duplicate server URLs are detected and rejected.
- **Level Indication**: 1-2

#### Backwards Compatibility
- **Assessment**: Fully compatible
- **Details**: Configurations with overlapping contexts are already broken (PipelineRuns being deleted). Rejecting them at startup with a clear error is strictly better behavior. No working configurations would break.
- **Level Indication**: 1-2

#### Architectural Alignment
- **Assessment**: Perfect fit
- **Details**: The exact same pattern (deduplicating aliased clusters) already exists for the `InClusterContext → DefaultClusterAlias` case in `main.go:127-136`. This extends that approach to the general case.
- **Level Indication**: 1-2

#### External Dependencies
- **Assessment**: None
- **Details**: Pure Go code comparing REST config fields. No external API calls or system dependencies.
- **Level Indication**: 1-3

### Recommended Labels

- [x] `good-first-issue`: Small scope, clear solution, existing patterns to follow
- [x] `kind/bug`: Destructive behavior (PipelineRun deletion) from valid-seeming configuration
- [x] `area/pipeline`: Affects the pipeline (Tekton) controller

### Guidance for Contributors

- Good starting point for new Prow contributors with basic Go and Kubernetes knowledge
- Suggested prerequisite knowledge: Go maps, kubeconfig structure (contexts, clusters), REST config basics
- Key files to review before starting:
  - `cmd/pipeline/main.go:120-164` - Current cluster config loading (the template to follow)
  - `pkg/kube/config.go:52-71` - How configs are merged
  - `cmd/pipeline/controller.go:463` - The code path that causes the bug (for understanding motivation)
- Recommended approach: After loading all cluster configs in `main.go`, iterate the map and build a reverse mapping of `Host → []contextName`. If any Host has multiple context names, log an error identifying the conflicting contexts and exit.
- Tests: Add a test case that provides two REST configs with the same Host but different context names, and verify the startup rejects them.

### Caveats and Considerations

- URL normalization should be considered (trailing slashes, port numbers) when comparing Host fields
- The fix should preserve the existing `InClusterContext → DefaultClusterAlias` dedup behavior
- An alternative approach of deduplicating silently (keeping one context, warning about the others) would also be valid but is less safe

## Proposed Issue Augmentation

### Title Change
- **Current**: Dealing with default and named clusters in prow configuration
- **Proposed**: Pipeline controller deletes PipelineRuns when two cluster contexts share the same cluster
- **Rationale**: Current title is vague ("Dealing with...") and doesn't mention the component or the destructive behavior. The new title is specific about what happens, which component is affected, and what triggers it.

### Proposed GitHub Comment

~~~
/retitle Pipeline controller deletes PipelineRuns when two cluster contexts share the same cluster

The root cause is in how the pipeline controller sets up per-cluster informers. When multiple kubeconfig contexts point to the same underlying cluster, each context gets its own independent informer watching the same resources. When a PipelineRun event arrives via the "wrong" context's informer, the reconciliation check at `cmd/pipeline/controller.go:463` (`ClusterToCtx(pj.Spec.Cluster) != ctx`) evaluates to true, setting `wantPipelineRun = false` and causing the controller to delete the PipelineRun. This only affects the pipeline (Tekton) controller; other Prow controllers operate in single-cluster mode and are not impacted.

The fix would be to add startup validation in `cmd/pipeline/main.go` that detects when multiple kubeconfig contexts resolve to the same cluster API server endpoint. There's already precedent for this kind of deduplication in the same file: the `InClusterContext` → `DefaultClusterAlias` mapping at lines 127-136 handles the specific case of empty-string vs "default" aliasing. The general case (any two contexts sharing a server URL) can follow the same pattern. As a workaround until this is fixed, ensure each kubeconfig context name maps to a distinct cluster — remove the duplicate context or avoid configuring `cluster:` in ProwJobs to use a named alias that shares a cluster with `default`.

/kind bug
/good-first-issue
~~~

### Rationale

**What's being added**:
- Root cause explanation: the original issue correctly identifies the code location but doesn't explain the dual-informer mechanism that causes the problem
- Scope clarification: this is pipeline-controller-specific, not a general Prow problem
- Fix approach: startup validation following existing patterns, giving potential contributors a clear starting point
- Workaround: practical guidance for the reporter until a fix lands

**Why these labels**:
- `/kind bug`: This is destructive behavior (PipelineRun deletion) from a valid-seeming configuration
- `/good-first-issue`: Level 1 effort — small scope (1-2 files, ~30-50 LOC), clear solution, existing pattern to follow
- No `/area` label: There is no `area/pipeline` label in the repository's label taxonomy. The pipeline controller is not represented.

**What's NOT included**:
- Priority label: While destructive, this affects a specific configuration pattern and has a straightforward workaround. Not adding priority — let maintainers decide.
- Detailed solution approaches: The comment focuses on the recommended approach. The full analysis of 3 approaches is in the triage document for maintainer reference.

## Next Steps

(Action items will be added here)

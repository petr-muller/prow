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

## Findings

(Additional findings from triage subcommands will be added here)

## Next Steps

(Action items will be added here)

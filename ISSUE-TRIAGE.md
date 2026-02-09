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

## Findings

(Findings from triage subcommands will be added here)

## Next Steps

(Action items will be added here)

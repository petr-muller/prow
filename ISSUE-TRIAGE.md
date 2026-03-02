# Triage for Issue #634

**Status**: In Progress
**Created**: 2026-03-02

## Issue Information

- **Issue Number**: #634
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/634
- **Title**: release-note plugin: make documentation URL configurable
- **Author**: DanielBlei
- **Created**: 2026-02-27

## Issue Summary

The `release-note` plugin hardcodes the Kubernetes community release note guide URL (`https://git.k8s.io/community/contributors/guide/release-notes.md`) in both bot comments and the `/help` command output. This prevents projects using Prow outside of the Kubernetes ecosystem from pointing contributors to their own release note process documentation.

The proposed solution is to add a configurable `url` field to the `release_note` plugin configuration, defaulting to the existing Kubernetes URL for backwards compatibility.

## Findings

(Findings from triage subcommands will be added here)

## Next Steps

(Action items will be added here)

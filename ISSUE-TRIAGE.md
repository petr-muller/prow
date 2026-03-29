# Triage for Issue #666

**Status**: In Progress
**Created**: 2026-03-29

## Issue Information

- **Issue Number**: #666
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/666
- **Title**: Allow use of Git subtrees with `mergecommitblocker`
- **Author**: RadaBDimitrova (Rada Dimitrova)
- **Labels**: area/plugins, kind/feature

## Issue Summary

The `mergecommitblocker` plugin blocks all merge commits unconditionally, which prevents Git subtree workflows from functioning. Git subtrees inherently depend on merge commits (created by `git subtree pull`) to track history correctly. The author requests flexibility to exempt certain paths/directories from the merge commit check.

## Initial Validation

**Assessment**: LEGITIMATE

### Analysis

The issue describes a feature request for the `mergecommitblocker` plugin, requesting path-based exclusions so that Git subtree merge commits can pass the check. This is a well-scoped enhancement request for a component that lives in this repository.

**Issue Category**: Feature Request

**Repository Scope Check**:
- Component mentioned: `mergecommitblocker` plugin
- Exists in this repo: Yes (`pkg/plugins/mergecommitblocker/mergecommitblocker.go`)
- Relevant code paths: `pkg/plugins/mergecommitblocker/`

**Information Completeness**:
- Sufficient detail provided: Yes
- The issue clearly explains the problem (unconditional merge commit blocking), the use case (Git subtrees), why the current behavior is incompatible, and suggests a concrete solution approach (path-based exclusions via `excludeDir` config option)

### Recommendation

Keep open and continue triage. This is a valid feature request for the `mergecommitblocker` plugin. The use case is real — Git subtrees require merge commits, and the plugin currently has no mechanism to allow them selectively. The labels (`area/plugins`, `kind/feature`) are already correctly applied.

**Suggested Action**:
- Keep open and continue triage

## Findings

(Additional findings from triage subcommands will be added here)

## Next Steps

(Action items will be added here)
